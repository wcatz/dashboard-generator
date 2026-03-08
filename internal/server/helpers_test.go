package server

import (
	"testing"

	"github.com/wcatz/dashboard-generator/internal/generator"
)

func TestFilterMetricInfoMap(t *testing.T) {
	m := map[string]generator.MetricInfo{
		"node_cpu_seconds_total":     {Type: "counter", Help: "CPU seconds"},
		"node_memory_MemTotal_bytes": {Type: "gauge", Help: "Total memory"},
		"node_disk_read_bytes_total": {Type: "counter", Help: "Disk reads"},
		"process_cpu_seconds_total":  {Type: "counter", Help: "Process CPU"},
		"go_goroutines":              {Type: "gauge", Help: "Goroutines"},
	}

	t.Run("include node_* only", func(t *testing.T) {
		result := filterMetricInfoMap(m, []string{"node_*"})
		if len(result) != 3 {
			t.Fatalf("expected 3 metrics, got %d", len(result))
		}
		for name := range result {
			if name != "node_cpu_seconds_total" && name != "node_memory_MemTotal_bytes" && name != "node_disk_read_bytes_total" {
				t.Errorf("unexpected metric %q", name)
			}
		}
	})

	t.Run("empty patterns includes all", func(t *testing.T) {
		result := filterMetricInfoMap(m, nil)
		if len(result) != 5 {
			t.Fatalf("expected 5 metrics, got %d", len(result))
		}
	})
}

func TestFilterByType(t *testing.T) {
	m := map[string]generator.MetricInfo{
		"gauge_metric":     {Type: "gauge", Help: "a gauge"},
		"counter_metric":   {Type: "counter", Help: "a counter"},
		"histogram_metric": {Type: "histogram", Help: "a histogram"},
		"gauge_metric2":    {Type: "gauge", Help: "another gauge"},
	}

	result := filterByType(m, "gauge")
	if len(result) != 2 {
		t.Fatalf("expected 2 gauge metrics, got %d", len(result))
	}
	for name, info := range result {
		if info.Type != "gauge" {
			t.Errorf("metric %q has type %q, want gauge", name, info.Type)
		}
	}
}

func TestBuildJobLabels(t *testing.T) {
	job := generator.JobSummary{
		Name:        "node",
		TargetCount: 3,
		Targets: []generator.TargetInfo{
			{Labels: map[string]string{"__name__": "up", "instance": "host1:9090", "env": "prod", "job": "node"}},
			{Labels: map[string]string{"__name__": "up", "instance": "host2:9090", "env": "prod", "job": "node"}},
			{Labels: map[string]string{"__name__": "up", "instance": "host3:9090", "env": "prod", "job": "node"}},
		},
	}

	labels := buildJobLabels(job)

	// __name__ should be excluded
	for _, ls := range labels {
		if ls.Name == "__name__" {
			t.Error("__name__ should be excluded")
		}
	}

	// Should be sorted by name
	for i := 1; i < len(labels); i++ {
		if labels[i].Name < labels[i-1].Name {
			t.Errorf("labels not sorted: %q before %q", labels[i-1].Name, labels[i].Name)
		}
	}

	// Find specific labels and check flags
	labelMap := make(map[string]labelSummary)
	for _, ls := range labels {
		labelMap[ls.Name] = ls
	}

	// "env" is on all 3 targets with single value "prod" → Constant=true, AllTargets=true
	if env, ok := labelMap["env"]; !ok {
		t.Error("expected 'env' label")
	} else {
		if !env.Constant {
			t.Error("env should be Constant (same value on all targets)")
		}
		if !env.AllTargets {
			t.Error("env should be AllTargets")
		}
	}

	// "instance" is on all targets but with different values → Constant=false, AllTargets=true
	if inst, ok := labelMap["instance"]; !ok {
		t.Error("expected 'instance' label")
	} else {
		if inst.Constant {
			t.Error("instance should not be Constant (different values)")
		}
		if !inst.AllTargets {
			t.Error("instance should be AllTargets")
		}
		if len(inst.Values) != 3 {
			t.Errorf("instance should have 3 values, got %d", len(inst.Values))
		}
	}
}

func TestMetricInfoToSlice(t *testing.T) {
	m := map[string]generator.MetricInfo{
		"zeta_metric":  {Type: "gauge", Help: "zeta help"},
		"alpha_metric": {Type: "counter", Help: "alpha help"},
		"mid_metric":   {Type: "histogram", Help: "mid help"},
	}

	result := metricInfoToSlice(m)
	if len(result) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(result))
	}

	// Verify sorted order
	want := []string{"alpha_metric", "mid_metric", "zeta_metric"}
	for i, name := range want {
		if result[i].Name != name {
			t.Errorf("result[%d].Name = %q, want %q", i, result[i].Name, name)
		}
	}

	// Verify fields preserved
	if result[0].Type != "counter" || result[0].Help != "alpha help" {
		t.Errorf("result[0] type/help mismatch: got %q/%q", result[0].Type, result[0].Help)
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want int
	}{
		{"int value", 42, 42},
		{"float64 value", float64(7), 7},
		{"string value returns 0", "5", 0},
		{"nil returns 0", nil, 0},
		{"bool returns 0", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toInt(tt.in)
			if got != tt.want {
				t.Errorf("toInt(%v) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidateFilename(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid name", "dashboard.json", false},
		{"forward slash", "path/file.json", true},
		{"backslash", "path\\file.json", true},
		{"dot dot", "..", true},
		{"dot dot prefix", "../etc/passwd", true},
		{"null byte", "file\x00.json", true},
		{"single dot", ".", true},
		{"simple name", "my-dashboard", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFilename(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFilename(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
