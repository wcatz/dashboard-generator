package generator

import (
	"strings"
	"testing"
)

func TestInferUnit(t *testing.T) {
	tests := []struct {
		metric string
		want   string
	}{
		// Bytes
		{"node_memory_MemTotal_bytes", "bytes"},
		{"node_filesystem_avail_bytes", "bytes"},
		{"process_resident_memory_bytes", "bytes"},

		// Byte rates (counter)
		{"node_network_receive_bytes_total", "Bps"},
		{"node_disk_read_bytes_total", "Bps"},

		// Seconds
		{"request_duration_seconds", "s"},
		{"process_cpu_seconds_total", "s"},
		{"http_request_duration_seconds_bucket", "s"},
		{"go_gc_duration_seconds_count", "s"},

		// Ratios and percentages
		{"node_filesystem_avail_ratio", "percentunit"},
		{"cpu_usage_percent", "percent"},

		// Generic counters
		{"http_requests_total", "short"},
		{"node_context_switches_total", "short"},

		// Info metrics
		{"kube_pod_info", "short"},
		{"node_uname_info", "short"},

		// Histogram buckets
		{"http_request_size_bucket", "short"},

		// Bytes created (edge case)
		{"some_bytes_created", "bytes"},

		// Default
		{"prometheus_tsdb_head_series", "short"},
		{"go_goroutines", "short"},
		{"up", "short"},
	}

	for _, tt := range tests {
		t.Run(tt.metric, func(t *testing.T) {
			got := InferUnit(tt.metric)
			if got != tt.want {
				t.Errorf("InferUnit(%q) = %q, want %q", tt.metric, got, tt.want)
			}
		})
	}
}

func TestSuggestTitle(t *testing.T) {
	tests := []struct {
		metric string
		want   string
	}{
		{"node_cpu_seconds_total", "node cpu"},
		{"node_memory_MemTotal_bytes", "node memory MemTotal"},
		{"http_requests_total", "http requests"},
		{"kube_pod_info", "kube pod"},
		{"http_request_duration_seconds_bucket", "http request duration"},
		{"go_goroutines", "go goroutines"},
		{"up", "up"},
		{"node_network_receive_bytes_total", "node network receive"},
		{"process_cpu_seconds_total", "process cpu"},
		{"http_request_size_bytes_created", "http request size"},
	}

	for _, tt := range tests {
		t.Run(tt.metric, func(t *testing.T) {
			got := SuggestTitle(tt.metric)
			if got != tt.want {
				t.Errorf("SuggestTitle(%q) = %q, want %q", tt.metric, got, tt.want)
			}
		})
	}
}

func TestSuggestLegend(t *testing.T) {
	tests := []struct {
		metricType string
		want       string
	}{
		{"counter", "{{instance}}"},
		{"gauge", "{{instance}}"},
		{"histogram", "p95"},
		{"summary", "{{instance}}"},
		{"untyped", "{{instance}}"},
	}

	for _, tt := range tests {
		t.Run(tt.metricType, func(t *testing.T) {
			got := SuggestLegend(tt.metricType)
			if got != tt.want {
				t.Errorf("SuggestLegend(%q) = %q, want %q", tt.metricType, got, tt.want)
			}
		})
	}
}

func TestSuggestThresholds(t *testing.T) {
	available := map[string]bool{
		"percent_usage": true,
		"binary_health": true,
		"latency_ms":    true,
	}

	tests := []struct {
		name   string
		metric string
		unit   string
		avail  map[string]bool
		want   string
	}{
		{"percent metric", "cpu_usage", "percent", available, "$percent_usage"},
		{"percentunit metric", "filesystem_avail", "percentunit", available, "$percent_usage"},
		{"info metric", "kube_pod_info", "short", available, "$binary_health"},
		{"latency metric", "request_duration", "s", available, "$latency_ms"},
		{"no match", "go_goroutines", "short", available, ""},
		{"nil available", "cpu_usage", "percent", nil, ""},
		{"missing threshold", "cpu_usage", "percent", map[string]bool{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SuggestThresholds(tt.metric, tt.unit, tt.avail)
			if got != tt.want {
				t.Errorf("SuggestThresholds(%q, %q) = %q, want %q", tt.metric, tt.unit, got, tt.want)
			}
		})
	}
}

func TestSuggestSize(t *testing.T) {
	tests := []struct {
		name      string
		panelType string
		count     int
		wantW     int
		wantH     int
	}{
		{"single stat", "stat", 1, 24, 4},
		{"4 stats", "stat", 4, 6, 4},
		{"8 stats", "stat", 8, 3, 4},
		{"6 stats", "stat", 6, 4, 4},
		{"single timeseries", "timeseries", 1, 24, 7},
		{"two timeseries", "timeseries", 2, 12, 7},
		{"many timeseries", "timeseries", 5, 12, 7},
		{"single gauge", "gauge", 1, 24, 4},
		{"4 gauges", "gauge", 4, 6, 4},
		{"table default", "table", 1, 0, 0},
		{"heatmap default", "heatmap", 1, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, h := SuggestSize(tt.panelType, tt.count)
			if w != tt.wantW || h != tt.wantH {
				t.Errorf("SuggestSize(%q, %d) = (%d, %d), want (%d, %d)", tt.panelType, tt.count, w, h, tt.wantW, tt.wantH)
			}
		})
	}
}

func TestSuggestPanel_Counter(t *testing.T) {
	info := MetricInfo{Type: "counter", Help: "Total HTTP requests."}
	opts := &SuggestOptions{
		RateInterval: "rate_interval",
		AvailableThresholds: map[string]bool{
			"percent_usage": true,
		},
	}

	s := SuggestPanel("http_requests_total", info, opts)

	if s.Type != "timeseries" {
		t.Errorf("Type = %q, want timeseries", s.Type)
	}
	if s.Query != "rate(http_requests_total[${rate_interval}])" {
		t.Errorf("Query = %q, want rate with rate_interval", s.Query)
	}
	if s.Unit != "short" {
		t.Errorf("Unit = %q, want short", s.Unit)
	}
	if s.Title != "http requests" {
		t.Errorf("Title = %q, want 'http requests'", s.Title)
	}
	if s.Description != "Total HTTP requests." {
		t.Errorf("Description = %q, want help text", s.Description)
	}
}

func TestSuggestPanel_CounterBytesTotal(t *testing.T) {
	info := MetricInfo{Type: "counter", Help: "Network bytes received."}
	opts := &SuggestOptions{RateInterval: "rate_interval"}

	s := SuggestPanel("node_network_receive_bytes_total", info, opts)

	if s.Type != "timeseries" {
		t.Errorf("Type = %q, want timeseries", s.Type)
	}
	if s.Unit != "Bps" {
		t.Errorf("Unit = %q, want Bps", s.Unit)
	}
	if !strings.Contains(s.Query, "rate(") {
		t.Errorf("Query = %q, want rate()", s.Query)
	}
}

func TestSuggestPanel_GaugePercent(t *testing.T) {
	info := MetricInfo{Type: "gauge", Help: "Filesystem available ratio."}
	opts := &SuggestOptions{
		AvailableThresholds: map[string]bool{"percent_usage": true},
	}

	s := SuggestPanel("node_filesystem_avail_ratio", info, opts)

	if s.Type != "gauge" {
		t.Errorf("Type = %q, want gauge", s.Type)
	}
	if s.Unit != "percentunit" {
		t.Errorf("Unit = %q, want percentunit", s.Unit)
	}
	if s.Thresholds != "$percent_usage" {
		t.Errorf("Thresholds = %q, want $percent_usage", s.Thresholds)
	}
}

func TestSuggestPanel_GaugeInfo(t *testing.T) {
	info := MetricInfo{Type: "gauge", Help: "Pod information."}
	opts := &SuggestOptions{
		AvailableThresholds: map[string]bool{"binary_health": true},
	}

	s := SuggestPanel("kube_pod_info", info, opts)

	if s.Type != "stat" {
		t.Errorf("Type = %q, want stat", s.Type)
	}
	if s.Thresholds != "$binary_health" {
		t.Errorf("Thresholds = %q, want $binary_health", s.Thresholds)
	}
}

func TestSuggestPanel_GaugeBytes(t *testing.T) {
	info := MetricInfo{Type: "gauge", Help: "Total memory in bytes."}

	s := SuggestPanel("node_memory_MemTotal_bytes", info, nil)

	if s.Type != "stat" {
		t.Errorf("Type = %q, want stat", s.Type)
	}
	if s.Unit != "bytes" {
		t.Errorf("Unit = %q, want bytes", s.Unit)
	}
	if s.Query != "node_memory_MemTotal_bytes" {
		t.Errorf("Query = %q, want bare metric", s.Query)
	}
}

func TestSuggestPanel_HistogramBucket(t *testing.T) {
	info := MetricInfo{Type: "histogram", Help: "Request duration histogram."}
	opts := &SuggestOptions{RateInterval: "rate_interval"}

	s := SuggestPanel("http_request_duration_seconds_bucket", info, opts)

	if s.Type != "timeseries" {
		t.Errorf("Type = %q, want timeseries", s.Type)
	}
	if !strings.Contains(s.Query, "histogram_quantile") {
		t.Errorf("Query = %q, want histogram_quantile", s.Query)
	}
	if !strings.Contains(s.Query, "by (le)") {
		t.Errorf("Query = %q, want by (le)", s.Query)
	}
	if s.Unit != "s" {
		t.Errorf("Unit = %q, want s", s.Unit)
	}
	if s.Legend != "p95" {
		t.Errorf("Legend = %q, want p95", s.Legend)
	}
}

func TestSuggestPanel_HistogramNonBucket(t *testing.T) {
	info := MetricInfo{Type: "histogram"}

	s := SuggestPanel("some_histogram", info, nil)

	if s.Type != "heatmap" {
		t.Errorf("Type = %q, want heatmap", s.Type)
	}
}

func TestSuggestPanel_Untyped_Total(t *testing.T) {
	info := MetricInfo{Type: "untyped"}
	opts := &SuggestOptions{RateInterval: "rate_interval"}

	s := SuggestPanel("my_events_total", info, opts)

	if s.Type != "timeseries" {
		t.Errorf("Type = %q, want timeseries", s.Type)
	}
	if !strings.Contains(s.Query, "rate(") {
		t.Errorf("Query = %q, want rate()", s.Query)
	}
}

func TestSuggestPanel_Untyped_Info(t *testing.T) {
	info := MetricInfo{Type: "untyped"}
	opts := &SuggestOptions{
		AvailableThresholds: map[string]bool{"binary_health": true},
	}

	s := SuggestPanel("node_uname_info", info, opts)

	if s.Type != "stat" {
		t.Errorf("Type = %q, want stat", s.Type)
	}
}

func TestSuggestPanel_NoRateInterval(t *testing.T) {
	info := MetricInfo{Type: "counter"}

	s := SuggestPanel("http_requests_total", info, nil)

	if s.Query != "rate(http_requests_total[5m])" {
		t.Errorf("Query = %q, want hardcoded 5m fallback", s.Query)
	}
}

func TestSuggestPanel_AutoPanelsOverride(t *testing.T) {
	info := MetricInfo{Type: "gauge"}
	opts := &SuggestOptions{
		AutoPanels: map[string]string{
			"gauge": "gauge", // force gauge panel instead of stat
		},
	}

	s := SuggestPanel("node_memory_MemTotal_bytes", info, opts)

	if s.Type != "gauge" {
		t.Errorf("Type = %q, want gauge (from auto_panels override)", s.Type)
	}
}

func TestSuggestPanel_AutoPanelsPluralKey(t *testing.T) {
	info := MetricInfo{Type: "counter"}
	opts := &SuggestOptions{
		AutoPanels: map[string]string{
			"counters": "bargauge", // plural form from config
		},
	}

	s := SuggestPanel("http_requests_total", info, opts)

	if s.Type != "bargauge" {
		t.Errorf("Type = %q, want bargauge (from auto_panels plural override)", s.Type)
	}
}

func TestSuggestPanel_Summary(t *testing.T) {
	info := MetricInfo{Type: "summary", Help: "Request duration summary."}

	s := SuggestPanel("request_duration_seconds", info, nil)

	if s.Type != "timeseries" {
		t.Errorf("Type = %q, want timeseries", s.Type)
	}
	if s.Unit != "s" {
		t.Errorf("Unit = %q, want s", s.Unit)
	}
}

func TestFormatSnippetYAML(t *testing.T) {
	suggestions := []PanelSuggestion{
		{
			Type:        "timeseries",
			Title:       "http requests",
			Query:       "rate(http_requests_total[${rate_interval}])",
			Unit:        "short",
			Description: "Total requests.",
			Legend:      "{{instance}}",
		},
		{
			Type:        "stat",
			Title:       "node memory MemTotal",
			Query:       "node_memory_MemTotal_bytes",
			Unit:        "bytes",
			Description: "Total memory.",
			Legend:      "{{instance}}",
			Thresholds:  "$percent_usage",
		},
	}

	yaml, hints := FormatSnippetYAML(suggestions, "discovered metrics", "primary")

	if !strings.Contains(yaml, "type: timeseries") {
		t.Error("expected timeseries type in output")
	}
	if !strings.Contains(yaml, "unit: bytes") {
		t.Error("expected unit: bytes in output")
	}
	if !strings.Contains(yaml, "thresholds: $percent_usage") {
		t.Error("expected thresholds in output")
	}
	if !strings.Contains(yaml, "datasource: primary") {
		t.Error("expected datasource in output")
	}
	if !strings.Contains(yaml, "description: \"Total memory.\"") {
		t.Error("expected description in output")
	}
	if !strings.Contains(yaml, "legend: \"{{instance}}\"") {
		t.Error("expected legend in output")
	}
	// "short" unit should not be in output (it's the default)
	lines := strings.Split(yaml, "\n")
	for _, line := range lines {
		if strings.Contains(line, "http requests") {
			// The line after title for the first panel should not have unit: short
			continue
		}
	}

	if len(hints) == 0 {
		t.Error("expected at least one hint")
	}
}

func TestFormatComparisonSnippetYAML(t *testing.T) {
	infos := map[string]MetricInfo{
		"up": {Type: "gauge", Help: "Target health."},
	}

	yaml, _ := FormatComparisonSnippetYAML([]string{"up"}, infos, []string{"primary", "secondary"}, nil)

	if !strings.Contains(yaml, "type: comparison") {
		t.Error("expected comparison type")
	}
	if !strings.Contains(yaml, "datasources: [primary, secondary]") {
		t.Error("expected datasources list")
	}
	if !strings.Contains(yaml, "description: \"Target health.\"") {
		t.Error("expected description from help text")
	}
}
