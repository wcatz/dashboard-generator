package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test-config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfig(t *testing.T) {
	cfg := `
generator:
  schema_version: 39
  refresh: "30s"
datasources:
  primary:
    type: prometheus
    uid: prometheus
    is_default: true
palettes:
  grafana:
    green: "#73BF69"
    red: "#F2495C"
active_palette: grafana
thresholds:
  health:
    - { color: "$red", value: null }
    - { color: "$green", value: 1 }
selectors:
  host: '{instance=~"$instance"}'
constants:
  rate_interval: "5m"
variables:
  instance:
    type: query
    datasource: primary
    query: 'label_values(up, instance)'
    multi: true
    include_all: true
dashboards:
  overview:
    uid: gen-overview
    title: overview
    filename: gen-overview.json
    tags: [overview]
    icon: apps
    sections:
      - title: health
        panels:
          - type: stat
            title: targets up
            query: 'count(up == 1)'
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Test generator settings
	gen := c.GetGenerator()
	if gen.SchemaVersion != 39 {
		t.Errorf("schema_version = %d, want 39", gen.SchemaVersion)
	}
	if gen.Refresh != "30s" {
		t.Errorf("refresh = %s, want 30s", gen.Refresh)
	}

	// Test datasource
	ds, err := c.GetDatasource("primary")
	if err != nil {
		t.Fatalf("GetDatasource error: %v", err)
	}
	if ds.Type != "prometheus" {
		t.Errorf("ds type = %s, want prometheus", ds.Type)
	}
	if ds.UID != "prometheus" {
		t.Errorf("ds uid = %s, want prometheus", ds.UID)
	}

	// Test default datasource
	def := c.GetDefaultDatasource()
	if def.UID != "prometheus" {
		t.Errorf("default ds uid = %s, want prometheus", def.UID)
	}

	// Test missing datasource
	_, err = c.GetDatasource("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent datasource")
	}

	// Test dashboards
	dbs, err := c.GetDashboards("")
	if err != nil {
		t.Fatalf("GetDashboards error: %v", err)
	}
	if len(dbs) != 1 {
		t.Errorf("dashboard count = %d, want 1", len(dbs))
	}
	db, ok := dbs["overview"]
	if !ok {
		t.Fatal("dashboard 'overview' not found")
	}
	if db.UID != "gen-overview" {
		t.Errorf("uid = %s, want gen-overview", db.UID)
	}
	if len(db.Sections) != 1 {
		t.Errorf("sections = %d, want 1", len(db.Sections))
	}
	if len(db.Sections[0].Panels) != 1 {
		t.Errorf("panels = %d, want 1", len(db.Sections[0].Panels))
	}
}

func TestResolveRef(t *testing.T) {
	cfg := `
constants:
  rate_interval: "5m"
selectors:
  host: '{instance=~"$instance"}'
datasources:
  primary:
    type: prometheus
    uid: prometheus
dashboards: {}
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	tests := []struct {
		input, want string
	}{
		{"rate(cpu[${rate_interval}])", "rate(cpu[5m])"},
		{"up${host}", `up{instance=~"$instance"}`},
		{"no refs here", "no refs here"},
		{"${unknown}", "${unknown}"},
	}
	for _, tt := range tests {
		got := c.ResolveRef(tt.input)
		if got != tt.want {
			t.Errorf("ResolveRef(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolveColor(t *testing.T) {
	cfg := `
palettes:
  grafana:
    green: "#73BF69"
    red: "#F2495C"
active_palette: grafana
datasources:
  primary:
    type: prometheus
    uid: prometheus
dashboards: {}
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	tests := []struct {
		input, want string
	}{
		{"$green", "#73BF69"},
		{"$red", "#F2495C"},
		{"$unknown", "unknown"},
		{"#FFFFFF", "#FFFFFF"},
	}
	for _, tt := range tests {
		got := c.ResolveColor(tt.input)
		if got != tt.want {
			t.Errorf("ResolveColor(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolveThresholds(t *testing.T) {
	cfg := `
palettes:
  grafana:
    green: "#73BF69"
    red: "#F2495C"
active_palette: grafana
thresholds:
  health:
    - { color: "$red", value: null }
    - { color: "$green", value: 1 }
datasources:
  primary:
    type: prometheus
    uid: prometheus
dashboards: {}
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	// Test named threshold
	steps := c.ResolveThresholds("$health")
	if len(steps) != 2 {
		t.Fatalf("threshold steps = %d, want 2", len(steps))
	}
	if steps[0].Color != "#F2495C" {
		t.Errorf("step[0].color = %s, want #F2495C", steps[0].Color)
	}
	if steps[1].Color != "#73BF69" {
		t.Errorf("step[1].color = %s, want #73BF69", steps[1].Color)
	}

	// Test inline thresholds
	inline := []interface{}{
		map[string]interface{}{"color": "$green", "value": nil},
		map[string]interface{}{"color": "#FF0000", "value": 50},
	}
	inlineSteps := c.ResolveThresholds(inline)
	if len(inlineSteps) != 2 {
		t.Fatalf("inline steps = %d, want 2", len(inlineSteps))
	}
	if inlineSteps[0].Color != "#73BF69" {
		t.Errorf("inline[0].color = %s, want #73BF69", inlineSteps[0].Color)
	}
	if inlineSteps[1].Color != "#FF0000" {
		t.Errorf("inline[1].color = %s, want #FF0000", inlineSteps[1].Color)
	}
}

func TestGetDashboardsWithProfile(t *testing.T) {
	cfg := `
profiles:
  infra:
    dashboards: [overview, compute]
datasources:
  primary:
    type: prometheus
    uid: prometheus
dashboards:
  overview:
    uid: gen-overview
    title: overview
    sections: []
  compute:
    uid: gen-compute
    title: compute
    sections: []
  services:
    uid: gen-services
    title: services
    sections: []
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	// Test with profile
	dbs, err := c.GetDashboards("infra")
	if err != nil {
		t.Fatalf("GetDashboards error: %v", err)
	}
	if len(dbs) != 2 {
		t.Errorf("profile dashboard count = %d, want 2", len(dbs))
	}
	if _, ok := dbs["services"]; ok {
		t.Error("services should not be in infra profile")
	}

	// Test invalid profile
	_, err = c.GetDashboards("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
}

func TestDashboardOrder(t *testing.T) {
	cfg := `
datasources:
  primary:
    type: prometheus
    uid: prometheus
dashboards:
  overview:
    uid: gen-overview
    title: overview
    sections: []
  compute:
    uid: gen-compute
    title: compute
    sections: []
  memory:
    uid: gen-memory
    title: memory
    sections: []
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	order, err := c.GetDashboardOrder("")
	if err != nil {
		t.Fatalf("GetDashboardOrder error: %v", err)
	}
	want := []string{"overview", "compute", "memory"}
	if len(order) != len(want) {
		t.Fatalf("order length = %d, want %d", len(order), len(want))
	}
	for i, name := range want {
		if order[i] != name {
			t.Errorf("order[%d] = %s, want %s", i, order[i], name)
		}
	}
}
