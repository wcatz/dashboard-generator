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

func TestGroupTargetsByJob(t *testing.T) {
	targets := []TargetInfo{
		{ScrapePool: "node-exporter", Health: "up", Labels: map[string]string{"job": "node-exporter", "instance": "host1:9100"}},
		{ScrapePool: "node-exporter", Health: "down", Labels: map[string]string{"job": "node-exporter", "instance": "host2:9100"}},
		{ScrapePool: "prometheus", Health: "up", Labels: map[string]string{"job": "prometheus", "instance": "localhost:9090"}},
		{ScrapePool: "prometheus", Health: "up", Labels: map[string]string{"job": "prometheus", "instance": "localhost:9091"}},
	}

	result := GroupTargetsByJob(targets)
	if len(result) != 2 {
		t.Fatalf("group count = %d, want 2", len(result))
	}
	// sorted by name: node-exporter < prometheus
	if result[0].Name != "node-exporter" {
		t.Errorf("result[0].Name = %q, want node-exporter", result[0].Name)
	}
	if result[0].TargetCount != 2 {
		t.Errorf("node-exporter TargetCount = %d, want 2", result[0].TargetCount)
	}
	if result[0].UpCount != 1 {
		t.Errorf("node-exporter UpCount = %d, want 1", result[0].UpCount)
	}
	if result[0].DownCount != 1 {
		t.Errorf("node-exporter DownCount = %d, want 1", result[0].DownCount)
	}
	if len(result[0].Targets) != 2 {
		t.Errorf("node-exporter Targets count = %d, want 2", len(result[0].Targets))
	}
	if result[1].Name != "prometheus" {
		t.Errorf("result[1].Name = %q, want prometheus", result[1].Name)
	}
	if result[1].TargetCount != 2 {
		t.Errorf("prometheus TargetCount = %d, want 2", result[1].TargetCount)
	}
	if result[1].UpCount != 2 {
		t.Errorf("prometheus UpCount = %d, want 2", result[1].UpCount)
	}
	if result[1].DownCount != 0 {
		t.Errorf("prometheus DownCount = %d, want 0", result[1].DownCount)
	}
}

func TestGroupTargetsByJobEmpty(t *testing.T) {
	result := GroupTargetsByJob(nil)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d entries", len(result))
	}
}

func TestGroupTargetsByJobFallbackLabel(t *testing.T) {
	targets := []TargetInfo{
		{ScrapePool: "", Health: "up", Labels: map[string]string{"job": "custom-job", "instance": "host1:9100"}},
		{ScrapePool: "", Health: "up", Labels: map[string]string{"job": "custom-job", "instance": "host2:9100"}},
		{ScrapePool: "", Health: "down", Labels: map[string]string{"job": "other-job", "instance": "host3:8080"}},
	}

	result := GroupTargetsByJob(targets)
	if len(result) != 2 {
		t.Fatalf("group count = %d, want 2", len(result))
	}
	if result[0].Name != "custom-job" {
		t.Errorf("result[0].Name = %q, want custom-job", result[0].Name)
	}
	if result[0].TargetCount != 2 {
		t.Errorf("custom-job TargetCount = %d, want 2", result[0].TargetCount)
	}
	if result[0].UpCount != 2 {
		t.Errorf("custom-job UpCount = %d, want 2", result[0].UpCount)
	}
	if result[1].Name != "other-job" {
		t.Errorf("result[1].Name = %q, want other-job", result[1].Name)
	}
	if result[1].DownCount != 1 {
		t.Errorf("other-job DownCount = %d, want 1", result[1].DownCount)
	}
}

func TestCategorize(t *testing.T) {
	serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/label/__name__/values":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"data":   []string{"up", "node_cpu", "node_mem"},
			})
		case "/api/v1/metadata":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"data": map[string]interface{}{
					"up":       []map[string]string{{"type": "gauge", "help": "Target up"}},
					"node_cpu": []map[string]string{{"type": "counter", "help": "CPU usage"}},
					"node_mem": []map[string]string{{"type": "gauge", "help": "Memory usage"}},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer serverA.Close()

	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/label/__name__/values":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"data":   []string{"up", "node_mem", "go_gc"},
			})
		case "/api/v1/metadata":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"data": map[string]interface{}{
					"up":       []map[string]string{{"type": "gauge", "help": "Target up"}},
					"node_mem": []map[string]string{{"type": "gauge", "help": "Memory usage"}},
					"go_gc":    []map[string]string{{"type": "counter", "help": "GC cycles"}},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer serverB.Close()

	cfg := &config.Config{
		Datasources: map[string]config.DatasourceDef{
			"ds_a": {Type: "prometheus", UID: "a", URL: serverA.URL},
			"ds_b": {Type: "prometheus", UID: "b", URL: serverB.URL},
		},
	}
	disc := NewMetricDiscovery(cfg)
	result, err := disc.Categorize("ds_a", "ds_b")
	if err != nil {
		t.Fatalf("Categorize error: %v", err)
	}

	// shared: up, node_mem
	if len(result["shared"]) != 2 {
		t.Errorf("shared count = %d, want 2", len(result["shared"]))
	}
	if _, ok := result["shared"]["up"]; !ok {
		t.Error("expected 'up' in shared")
	}
	if _, ok := result["shared"]["node_mem"]; !ok {
		t.Error("expected 'node_mem' in shared")
	}

	// only_a: node_cpu
	if len(result["only_a"]) != 1 {
		t.Errorf("only_a count = %d, want 1", len(result["only_a"]))
	}
	if _, ok := result["only_a"]["node_cpu"]; !ok {
		t.Error("expected 'node_cpu' in only_a")
	}
	if result["only_a"]["node_cpu"].Type != "counter" {
		t.Errorf("node_cpu type = %q, want counter", result["only_a"]["node_cpu"].Type)
	}

	// only_b: go_gc
	if len(result["only_b"]) != 1 {
		t.Errorf("only_b count = %d, want 1", len(result["only_b"]))
	}
	if _, ok := result["only_b"]["go_gc"]; !ok {
		t.Error("expected 'go_gc' in only_b")
	}
}

func TestCompareAll(t *testing.T) {
	newMockServer := func(metrics []string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v1/label/__name__/values":
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "success",
					"data":   metrics,
				})
			case "/api/v1/metadata":
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "success",
					"data":   map[string]interface{}{},
				})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
	}

	srvA := newMockServer([]string{"m1", "m2", "m3"})
	defer srvA.Close()
	srvB := newMockServer([]string{"m1", "m2", "m4"})
	defer srvB.Close()
	srvC := newMockServer([]string{"m1", "m5"})
	defer srvC.Close()

	cfg := &config.Config{
		Datasources: map[string]config.DatasourceDef{
			"a": {Type: "prometheus", UID: "a", URL: srvA.URL},
			"b": {Type: "prometheus", UID: "b", URL: srvB.URL},
			"c": {Type: "prometheus", UID: "c", URL: srvC.URL},
		},
	}
	disc := NewMetricDiscovery(cfg)
	shared, exclusive, err := disc.CompareAll([]string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("CompareAll error: %v", err)
	}

	// shared: only m1 is on all three
	if len(shared) != 1 {
		t.Errorf("shared count = %d, want 1", len(shared))
	}
	if _, ok := shared["m1"]; !ok {
		t.Error("expected 'm1' in shared")
	}

	// exclusive a: m3 (m2 is also on b, so not exclusive)
	if len(exclusive["a"]) != 1 {
		t.Errorf("exclusive[a] count = %d, want 1", len(exclusive["a"]))
	}
	if _, ok := exclusive["a"]["m3"]; !ok {
		t.Error("expected 'm3' exclusive to a")
	}

	// exclusive b: m4
	if len(exclusive["b"]) != 1 {
		t.Errorf("exclusive[b] count = %d, want 1", len(exclusive["b"]))
	}
	if _, ok := exclusive["b"]["m4"]; !ok {
		t.Error("expected 'm4' exclusive to b")
	}

	// exclusive c: m5
	if len(exclusive["c"]) != 1 {
		t.Errorf("exclusive[c] count = %d, want 1", len(exclusive["c"]))
	}
	if _, ok := exclusive["c"]["m5"]; !ok {
		t.Error("expected 'm5' exclusive to c")
	}
}

func TestCompareAllTooFew(t *testing.T) {
	cfg := &config.Config{
		Datasources: map[string]config.DatasourceDef{
			"a": {Type: "prometheus", UID: "a", URL: "http://localhost:9090"},
		},
	}
	disc := NewMetricDiscovery(cfg)
	_, _, err := disc.CompareAll([]string{"a"})
	if err == nil {
		t.Fatal("expected error for < 2 datasources")
	}
}

func TestLookupMeta(t *testing.T) {
	primary := map[string]MetricInfo{
		"up": {Type: "gauge", Help: "Target up"},
	}
	fallback := map[string]MetricInfo{
		"node_cpu": {Type: "counter", Help: "CPU seconds"},
	}

	// Found in primary
	info := lookupMeta("up", primary, fallback)
	if info.Type != "gauge" {
		t.Errorf("lookupMeta(up) type = %q, want gauge", info.Type)
	}

	// Found in fallback
	info = lookupMeta("node_cpu", primary, fallback)
	if info.Type != "counter" {
		t.Errorf("lookupMeta(node_cpu) type = %q, want counter", info.Type)
	}

	// Not found anywhere
	info = lookupMeta("missing", primary, fallback)
	if info.Type != "untyped" {
		t.Errorf("lookupMeta(missing) type = %q, want untyped", info.Type)
	}
}

func TestSortedKeys(t *testing.T) {
	m := map[string]bool{"zebra": true, "alpha": true, "mango": true}
	got := sortedKeys(m)
	want := []string{"alpha", "mango", "zebra"}
	if len(got) != len(want) {
		t.Fatalf("sortedKeys len = %d, want %d", len(got), len(want))
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("sortedKeys[%d] = %q, want %q", i, got[i], v)
		}
	}
}

func TestSortedMetricKeys(t *testing.T) {
	m := map[string]MetricInfo{
		"node_mem": {Type: "gauge"},
		"cpu":      {Type: "counter"},
		"up":       {Type: "gauge"},
	}
	got := sortedMetricKeys(m)
	want := []string{"cpu", "node_mem", "up"}
	if len(got) != len(want) {
		t.Fatalf("sortedMetricKeys len = %d, want %d", len(got), len(want))
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("sortedMetricKeys[%d] = %q, want %q", i, got[i], v)
		}
	}
}