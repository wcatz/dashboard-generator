package generator

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wcatz/dashboard-generator/internal/config"
)

func TestFilterMetrics(t *testing.T) {
	metrics := map[string]bool{
		"node_cpu_seconds_total":     true,
		"node_memory_MemTotal_bytes": true,
		"kube_pod_info":              true,
		"ALERTS":                     true,
		"scrape_duration_seconds":    true,
		"node_disk_io_bucket":        true,
	}

	filtered := FilterMetrics(metrics,
		[]string{"node_*", "kube_*"},
		[]string{"*_bucket"},
	)

	if len(filtered) != 3 {
		t.Errorf("filtered count = %d, want 3", len(filtered))
	}
	if !filtered["node_cpu_seconds_total"] {
		t.Error("should include node_cpu_seconds_total")
	}
	if !filtered["node_memory_MemTotal_bytes"] {
		t.Error("should include node_memory_MemTotal_bytes")
	}
	if !filtered["kube_pod_info"] {
		t.Error("should include kube_pod_info")
	}
	if filtered["ALERTS"] {
		t.Error("should not include ALERTS")
	}
	if filtered["node_disk_io_bucket"] {
		t.Error("should not include node_disk_io_bucket (excluded by *_bucket)")
	}
}

func TestFilterMetricsDefaultInclude(t *testing.T) {
	metrics := map[string]bool{
		"metric_a": true,
		"metric_b": true,
	}
	filtered := FilterMetrics(metrics, nil, nil)
	if len(filtered) != 2 {
		t.Errorf("default filter count = %d, want 2", len(filtered))
	}
}

func TestGlobMatch(t *testing.T) {
	tests := []struct {
		pattern, name string
		want          bool
	}{
		{"node_*", "node_cpu_seconds_total", true},
		{"node_*", "kube_pod_info", false},
		{"*_bucket", "node_disk_io_bucket", true},
		{"*_bucket", "node_disk_io", false},
		{"ALERTS*", "ALERTS", true},
		{"ALERTS*", "ALERTS_for_state", true},
		{"up", "up", true},
		{"up", "uptime", false},
		{"?oo", "foo", true},
		{"?oo", "fo", false},
	}
	for _, tt := range tests {
		got := globMatch(tt.pattern, tt.name)
		if got != tt.want {
			t.Errorf("globMatch(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
		}
	}
}

func TestGroupByPrefix(t *testing.T) {
	metrics := map[string]MetricInfo{
		"node_cpu_seconds_total":     {Type: "counter"},
		"node_cpu_guest_seconds":     {Type: "counter"},
		"node_memory_MemTotal_bytes": {Type: "gauge"},
		"up":                         {Type: "gauge"},
	}

	groups := GroupByPrefix(metrics)
	if len(groups) != 3 {
		t.Errorf("group count = %d, want 3", len(groups))
	}
	if len(groups["node_cpu"]) != 2 {
		t.Errorf("node_cpu count = %d, want 2", len(groups["node_cpu"]))
	}
	if len(groups["node_memory"]) != 1 {
		t.Errorf("node_memory count = %d, want 1", len(groups["node_memory"]))
	}
}

func TestSuggestPanelType(t *testing.T) {
	tests := []struct {
		metricType, want string
	}{
		{"counter", "timeseries"},
		{"gauge", "stat"},
		{"histogram", "heatmap"},
		{"summary", "timeseries"},
		{"untyped", "timeseries"},
		{"unknown", "timeseries"},
	}
	for _, tt := range tests {
		got := SuggestPanelType(tt.metricType)
		if got != tt.want {
			t.Errorf("SuggestPanelType(%q) = %q, want %q", tt.metricType, got, tt.want)
		}
	}
}

func TestSuggestQuery(t *testing.T) {
	tests := []struct {
		name, metricType, want string
	}{
		{"http_requests_total", "counter", "rate(http_requests_total[5m])"},
		{"node_memory_MemTotal_bytes", "gauge", "node_memory_MemTotal_bytes"},
		{"request_duration_bucket", "histogram", "histogram_quantile(0.95, sum(rate(request_duration_bucket[5m])) by (le))"},
	}
	for _, tt := range tests {
		got := SuggestQuery(tt.name, tt.metricType)
		if got != tt.want {
			t.Errorf("SuggestQuery(%q, %q) = %q, want %q", tt.name, tt.metricType, got, tt.want)
		}
	}
}

func TestDiscoveryAuthBearer(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data":   []string{"up", "node_cpu_seconds_total"},
		})
	}))
	defer server.Close()

	cfg := &config.Config{
		Datasources: map[string]config.DatasourceDef{
			"test": {Type: "prometheus", UID: "test", URL: server.URL, Token: "my-token"},
		},
	}
	disc := NewMetricDiscovery(cfg)
	_, err := disc.FetchMetrics("test")
	if err != nil {
		t.Fatalf("FetchMetrics error: %v", err)
	}
	if gotAuth != "Bearer my-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer my-token")
	}
}

func TestDiscoveryAuthBasic(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data":   []string{"up"},
		})
	}))
	defer server.Close()

	cfg := &config.Config{
		Datasources: map[string]config.DatasourceDef{
			"cloud": {Type: "prometheus", UID: "cloud", URL: server.URL, BasicUser: "123456", BasicPass: "api-key"},
		},
	}
	disc := NewMetricDiscovery(cfg)
	_, err := disc.FetchMetrics("cloud")
	if err != nil {
		t.Fatalf("FetchMetrics error: %v", err)
	}
	if gotAuth == "" {
		t.Fatal("expected Authorization header")
	}
	if gotAuth[:6] != "Basic " {
		t.Errorf("Authorization should start with 'Basic ', got %q", gotAuth)
	}
}

func TestDiscoveryAuthExplicitOverridesConfig(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data":   []string{"up"},
		})
	}))
	defer server.Close()

	cfg := &config.Config{
		Datasources: map[string]config.DatasourceDef{
			"test": {Type: "prometheus", UID: "test", URL: server.URL, Token: "config-token"},
		},
	}
	disc := NewMetricDiscoveryWithAuth(cfg, "", "", "cli-token")
	_, err := disc.FetchMetrics("test")
	if err != nil {
		t.Fatalf("FetchMetrics error: %v", err)
	}
	if gotAuth != "Bearer cli-token" {
		t.Errorf("Authorization = %q, want %q (explicit should override config)", gotAuth, "Bearer cli-token")
	}
}

func TestDiscoveryNoAuth(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data":   []string{"up"},
		})
	}))
	defer server.Close()

	cfg := &config.Config{
		Datasources: map[string]config.DatasourceDef{
			"local": {Type: "prometheus", UID: "local", URL: server.URL},
		},
	}
	disc := NewMetricDiscovery(cfg)
	_, err := disc.FetchMetrics("local")
	if err != nil {
		t.Fatalf("FetchMetrics error: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("expected no Authorization header, got %q", gotAuth)
	}
}

func TestDiscoveryAuthEnvVar(t *testing.T) {
	t.Setenv("TEST_PROM_TOKEN", "env-resolved-token")

	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data":   []string{"up"},
		})
	}))
	defer server.Close()

	cfg := &config.Config{
		Datasources: map[string]config.DatasourceDef{
			"cloud": {Type: "prometheus", UID: "cloud", URL: server.URL, Token: "$TEST_PROM_TOKEN"},
		},
	}
	disc := NewMetricDiscovery(cfg)
	_, err := disc.FetchMetrics("cloud")
	if err != nil {
		t.Fatalf("FetchMetrics error: %v", err)
	}
	if gotAuth != "Bearer env-resolved-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer env-resolved-token")
	}
}


func TestFetchMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/metadata" {
			t.Errorf("unexpected path %q, want /api/v1/metadata", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data": map[string]interface{}{
				"up": []map[string]string{{"type": "gauge", "help": "Whether target is up", "unit": ""}},
				"node_cpu_seconds_total": []map[string]string{{"type": "counter", "help": "CPU seconds", "unit": ""}},
			},
		})
	}))
	defer server.Close()

	cfg := &config.Config{
		Datasources: map[string]config.DatasourceDef{
			"test": {Type: "prometheus", UID: "test", URL: server.URL},
		},
	}
	disc := NewMetricDiscovery(cfg)
	meta, err := disc.FetchMetadata("test")
	if err != nil {
		t.Fatalf("FetchMetadata error: %v", err)
	}
	if len(meta) != 2 {
		t.Fatalf("metadata count = %d, want 2", len(meta))
	}
	if meta["up"].Type != "gauge" {
		t.Errorf("up type = %q, want gauge", meta["up"].Type)
	}
	if meta["up"].Help != "Whether target is up" {
		t.Errorf("up help = %q, want %q", meta["up"].Help, "Whether target is up")
	}
	if meta["node_cpu_seconds_total"].Type != "counter" {
		t.Errorf("node_cpu type = %q, want counter", meta["node_cpu_seconds_total"].Type)
	}
	if meta["node_cpu_seconds_total"].Help != "CPU seconds" {
		t.Errorf("node_cpu help = %q, want %q", meta["node_cpu_seconds_total"].Help, "CPU seconds")
	}
}

func TestFetchMetadataError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	cfg := &config.Config{
		Datasources: map[string]config.DatasourceDef{
			"test": {Type: "prometheus", UID: "test", URL: server.URL},
		},
	}
	disc := NewMetricDiscovery(cfg)
	_, err := disc.FetchMetadata("test")
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestFetchLabels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/labels" {
			t.Errorf("unexpected path %q, want /api/v1/labels", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data":   []string{"__name__", "instance", "job"},
		})
	}))
	defer server.Close()

	cfg := &config.Config{
		Datasources: map[string]config.DatasourceDef{
			"test": {Type: "prometheus", UID: "test", URL: server.URL},
		},
	}
	disc := NewMetricDiscovery(cfg)
	labels, err := disc.FetchLabels("test")
	if err != nil {
		t.Fatalf("FetchLabels error: %v", err)
	}
	want := []string{"__name__", "instance", "job"}
	if len(labels) != len(want) {
		t.Fatalf("labels count = %d, want %d", len(labels), len(want))
	}
	for i, l := range labels {
		if l != want[i] {
			t.Errorf("labels[%d] = %q, want %q", i, l, want[i])
		}
	}
}

func TestFetchLabelValues(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data":   []string{"node-exporter", "prometheus"},
		})
	}))
	defer server.Close()

	cfg := &config.Config{
		Datasources: map[string]config.DatasourceDef{
			"test": {Type: "prometheus", UID: "test", URL: server.URL},
		},
	}
	disc := NewMetricDiscovery(cfg)
	values, err := disc.FetchLabelValues("test", "job")
	if err != nil {
		t.Fatalf("FetchLabelValues error: %v", err)
	}
	if gotPath != "/api/v1/label/job/values" {
		t.Errorf("URL path = %q, want /api/v1/label/job/values", gotPath)
	}
	if len(values) != 2 {
		t.Fatalf("values count = %d, want 2", len(values))
	}
	if values[0] != "node-exporter" {
		t.Errorf("values[0] = %q, want node-exporter", values[0])
	}
	if values[1] != "prometheus" {
		t.Errorf("values[1] = %q, want prometheus", values[1])
	}
}

func TestFetchLabelValuesError(t *testing.T) {
	cfg := &config.Config{
		Datasources: map[string]config.DatasourceDef{},
	}
	disc := NewMetricDiscovery(cfg)
	values, err := disc.FetchLabelValues("nonexistent", "job")
	// FetchLabelValues returns nil,nil for unknown datasource (empty URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if values != nil {
		t.Errorf("expected nil values for unknown datasource, got %v", values)
	}
}

func TestFetchSeriesMetrics(t *testing.T) {
	var gotPath, gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data": []map[string]string{
				{"__name__": "up", "job": "node"},
				{"__name__": "node_cpu", "job": "node"},
			},
		})
	}))
	defer server.Close()

	cfg := &config.Config{
		Datasources: map[string]config.DatasourceDef{
			"test": {Type: "prometheus", UID: "test", URL: server.URL},
		},
	}
	disc := NewMetricDiscovery(cfg)
	metrics, err := disc.FetchSeriesMetrics("test", "job", "node")
	if err != nil {
		t.Fatalf("FetchSeriesMetrics error: %v", err)
	}
	if gotPath != "/api/v1/series" {
		t.Errorf("URL path = %q, want /api/v1/series", gotPath)
	}
	if gotQuery == "" {
		t.Error("expected match[] query parameter")
	}
	if len(metrics) != 2 {
		t.Fatalf("metrics count = %d, want 2", len(metrics))
	}
	if !metrics["up"] {
		t.Error("expected 'up' in metrics")
	}
	if !metrics["node_cpu"] {
		t.Error("expected 'node_cpu' in metrics")
	}
}

func TestFetchTargets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/targets" {
			t.Errorf("unexpected path %q, want /api/v1/targets", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data": map[string]interface{}{
				"activeTargets": []map[string]interface{}{
					{
						"labels":     map[string]string{"job": "node-exporter", "instance": "localhost:9100"},
						"scrapePool": "node-exporter",
						"health":     "up",
					},
					{
						"labels":     map[string]string{"job": "prometheus", "instance": "localhost:9090"},
						"scrapePool": "prometheus",
						"health":     "up",
					},
				},
			},
		})
	}))
	defer server.Close()

	cfg := &config.Config{
		Datasources: map[string]config.DatasourceDef{
			"test": {Type: "prometheus", UID: "test", URL: server.URL},
		},
	}
	disc := NewMetricDiscovery(cfg)
	targets, err := disc.FetchTargets("test")
	if err != nil {
		t.Fatalf("FetchTargets error: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("targets count = %d, want 2", len(targets))
	}
	if targets[0].ScrapePool != "node-exporter" {
		t.Errorf("targets[0].ScrapePool = %q, want node-exporter", targets[0].ScrapePool)
	}
	if targets[0].Instance != "localhost:9100" {
		t.Errorf("targets[0].Instance = %q, want localhost:9100", targets[0].Instance)
	}
	if targets[0].Health != "up" {
		t.Errorf("targets[0].Health = %q, want up", targets[0].Health)
	}
	if targets[0].Labels["job"] != "node-exporter" {
		t.Errorf("targets[0].Labels[job] = %q, want node-exporter", targets[0].Labels["job"])
	}
	if targets[1].ScrapePool != "prometheus" {
		t.Errorf("targets[1].ScrapePool = %q, want prometheus", targets[1].ScrapePool)
	}
	if targets[1].Instance != "localhost:9090" {
		t.Errorf("targets[1].Instance = %q, want localhost:9090", targets[1].Instance)
	}
}

func TestFetchTargetsError(t *testing.T) {
	cfg := &config.Config{
		Datasources: map[string]config.DatasourceDef{},
	}
	disc := NewMetricDiscovery(cfg)
	_, err := disc.FetchTargets("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown datasource")
	}
}