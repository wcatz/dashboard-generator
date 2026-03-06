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
		json.NewEncoder(w).Encode(map[string]interface{}{
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
		json.NewEncoder(w).Encode(map[string]interface{}{
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
		json.NewEncoder(w).Encode(map[string]interface{}{
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
		json.NewEncoder(w).Encode(map[string]interface{}{
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
		json.NewEncoder(w).Encode(map[string]interface{}{
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
