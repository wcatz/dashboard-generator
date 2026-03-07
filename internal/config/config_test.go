package config

import (
	"os"
	"path/filepath"
	"strings"
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
  by_ns: '{namespace=~"$namespace"}'
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
		// adjacent selectors are merged into a single block
		{`metric${host}{job="test"}`, `metric{instance=~"$instance", job="test"}`},
		{`metric${host}${by_ns}`, `metric{instance=~"$instance", namespace=~"$namespace"}`},
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

func TestDatasourceAuth(t *testing.T) {
	cfg := `
datasources:
  cloud:
    type: prometheus
    uid: cloud-prom
    url: https://prom.grafana.net/api/prom
    basic_user: "123456"
    basic_pass: "secret-key"
  local:
    type: prometheus
    uid: local-prom
    url: http://localhost:9090
  token_ds:
    type: prometheus
    uid: token-prom
    url: http://prom.internal:9090
    token: "my-bearer-token"
dashboards: {}
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	cloud := c.Datasources["cloud"]
	if cloud.BasicUser != "123456" {
		t.Errorf("basic_user = %q, want 123456", cloud.BasicUser)
	}
	if cloud.BasicPass != "secret-key" {
		t.Errorf("basic_pass = %q, want secret-key", cloud.BasicPass)
	}
	if cloud.ResolvedBasicPass() != "secret-key" {
		t.Errorf("ResolvedBasicPass = %q, want secret-key", cloud.ResolvedBasicPass())
	}

	local := c.Datasources["local"]
	if local.BasicUser != "" || local.Token != "" {
		t.Error("local should have no auth")
	}

	tokenDS := c.Datasources["token_ds"]
	if tokenDS.Token != "my-bearer-token" {
		t.Errorf("token = %q, want my-bearer-token", tokenDS.Token)
	}
	if tokenDS.ResolvedToken() != "my-bearer-token" {
		t.Errorf("ResolvedToken = %q, want my-bearer-token", tokenDS.ResolvedToken())
	}
}

func TestDatasourceAuthEnvVar(t *testing.T) {
	t.Setenv("TEST_DS_TOKEN", "env-token-value")
	t.Setenv("TEST_DS_PASS", "env-pass-value")

	cfg := `
datasources:
  cloud:
    type: prometheus
    uid: cloud-prom
    basic_user: "user"
    basic_pass: "$TEST_DS_PASS"
    token: "$TEST_DS_TOKEN"
dashboards: {}
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	ds := c.Datasources["cloud"]
	if ds.ResolvedBasicPass() != "env-pass-value" {
		t.Errorf("ResolvedBasicPass = %q, want env-pass-value", ds.ResolvedBasicPass())
	}
	if ds.ResolvedToken() != "env-token-value" {
		t.Errorf("ResolvedToken = %q, want env-token-value", ds.ResolvedToken())
	}
}

func TestDatasourceDefString(t *testing.T) {
	ds := DatasourceDef{
		Type:      "prometheus",
		UID:       "prom",
		URL:       "http://localhost:9090",
		BasicUser: "admin",
		BasicPass: "secret",
		Token:     "bearer-token",
	}
	s := ds.String()
	if strings.Contains(s, "secret") {
		t.Error("String() should mask BasicPass")
	}
	if strings.Contains(s, "bearer-token") {
		t.Error("String() should mask Token")
	}
	if !strings.Contains(s, "***") {
		t.Error("String() should contain masked value")
	}
	if !strings.Contains(s, "admin") {
		t.Error("String() should show BasicUser")
	}
}

func TestYAMLEditorDatasourceAuth(t *testing.T) {
	initial := `
datasources:
  existing:
    type: prometheus
    uid: prom
dashboards: {}
`
	path := writeTestConfig(t, initial)

	editor := NewYAMLEditor(path)
	err := editor.AddDatasource("cloud", DatasourceDef{
		Type:      "prometheus",
		UID:       "cloud-prom",
		URL:       "https://prom.grafana.net",
		BasicUser: "123456",
		BasicPass: "$CLOUD_API_KEY",
	})
	if err != nil {
		t.Fatalf("AddDatasource error: %v", err)
	}

	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	ds := c.Datasources["cloud"]
	if ds.BasicUser != "123456" {
		t.Errorf("basic_user = %q, want 123456", ds.BasicUser)
	}
	if ds.BasicPass != "$CLOUD_API_KEY" {
		t.Errorf("basic_pass = %q, want $CLOUD_API_KEY", ds.BasicPass)
	}

	// Test UpdateDatasourceAuth
	err = editor.UpdateDatasourceAuth("cloud", "", "", "new-token")
	if err != nil {
		t.Fatalf("UpdateDatasourceAuth error: %v", err)
	}
	c, err = Load(path, nil)
	if err != nil {
		t.Fatalf("Reload error: %v", err)
	}
	ds = c.Datasources["cloud"]
	if ds.BasicUser != "" {
		t.Errorf("basic_user should be cleared, got %q", ds.BasicUser)
	}
	if ds.Token != "new-token" {
		t.Errorf("token = %q, want new-token", ds.Token)
	}
}
