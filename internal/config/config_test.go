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


const editorTestYAML = `
datasources:
  primary:
    type: prometheus
    uid: prom1
    url: http://localhost:9090
  secondary:
    type: prometheus
    uid: prom2
palettes:
  default:
    blue: "#3274d9"
    green: "#73bf69"
    red: "#f2495c"
active_palette: default
variables:
  cluster:
    type: query
    datasource: primary
    query: 'label_values(up, cluster)'
`

func TestYAMLEditorDeleteDatasource(t *testing.T) {
	path := writeTestConfig(t, editorTestYAML)
	editor := NewYAMLEditor(path)
	if err := editor.DeleteDatasource("secondary"); err != nil {
		t.Fatalf("DeleteDatasource error: %v", err)
	}
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if _, ok := c.Datasources["secondary"]; ok {
		t.Error("secondary should be deleted")
	}
	if _, ok := c.Datasources["primary"]; !ok {
		t.Error("primary should still exist")
	}
	ds := c.Datasources["primary"]
	if ds.UID != "prom1" {
		t.Errorf("primary uid = %q, want prom1", ds.UID)
	}
}

func TestYAMLEditorDeleteDatasourceNotFound(t *testing.T) {
	path := writeTestConfig(t, editorTestYAML)
	editor := NewYAMLEditor(path)
	err := editor.DeleteDatasource("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent datasource")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}

func TestYAMLEditorUpdateDatasourceURL(t *testing.T) {
	path := writeTestConfig(t, editorTestYAML)
	editor := NewYAMLEditor(path)
	// Update existing URL
	if err := editor.UpdateDatasourceURL("primary", "http://newhost:9090"); err != nil {
		t.Fatalf("UpdateDatasourceURL error: %v", err)
	}
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if c.Datasources["primary"].URL != "http://newhost:9090" {
		t.Errorf("primary url = %q, want http://newhost:9090", c.Datasources["primary"].URL)
	}
	// Add URL to DS that has none
	if err := editor.UpdateDatasourceURL("secondary", "http://secondary:9090"); err != nil {
		t.Fatalf("UpdateDatasourceURL (add) error: %v", err)
	}
	c, err = Load(path, nil)
	if err != nil {
		t.Fatalf("Reload error: %v", err)
	}
	if c.Datasources["secondary"].URL != "http://secondary:9090" {
		t.Errorf("secondary url = %q, want http://secondary:9090", c.Datasources["secondary"].URL)
	}
}

func TestYAMLEditorSetPaletteColor(t *testing.T) {
	path := writeTestConfig(t, editorTestYAML)
	editor := NewYAMLEditor(path)
	// Update existing color
	if err := editor.SetPaletteColor("default", "blue", "#0000FF"); err != nil {
		t.Fatalf("SetPaletteColor error: %v", err)
	}
	// Add new color
	if err := editor.SetPaletteColor("default", "yellow", "#FFFF00"); err != nil {
		t.Fatalf("SetPaletteColor (add) error: %v", err)
	}
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	pal := c.Palettes["default"]
	if pal["blue"] != "#0000FF" {
		t.Errorf("blue = %q, want #0000FF", pal["blue"])
	}
	if pal["yellow"] != "#FFFF00" {
		t.Errorf("yellow = %q, want #FFFF00", pal["yellow"])
	}
	if pal["green"] != "#73bf69" {
		t.Errorf("green should be unchanged, got %q", pal["green"])
	}
}

func TestYAMLEditorDeletePaletteColor(t *testing.T) {
	path := writeTestConfig(t, editorTestYAML)
	editor := NewYAMLEditor(path)
	if err := editor.DeletePaletteColor("default", "red"); err != nil {
		t.Fatalf("DeletePaletteColor error: %v", err)
	}
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	pal := c.Palettes["default"]
	if _, ok := pal["red"]; ok {
		t.Error("red should be deleted")
	}
	if pal["blue"] != "#3274d9" {
		t.Errorf("blue should remain, got %q", pal["blue"])
	}
	if pal["green"] != "#73bf69" {
		t.Errorf("green should remain, got %q", pal["green"])
	}
}

func TestYAMLEditorDeletePaletteColorNotFound(t *testing.T) {
	path := writeTestConfig(t, editorTestYAML)
	editor := NewYAMLEditor(path)
	err := editor.DeletePaletteColor("default", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent color")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}

func TestYAMLEditorRenamePaletteColor(t *testing.T) {
	path := writeTestConfig(t, editorTestYAML)
	editor := NewYAMLEditor(path)
	if err := editor.RenamePaletteColor("default", "blue", "primary_blue"); err != nil {
		t.Fatalf("RenamePaletteColor error: %v", err)
	}
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	pal := c.Palettes["default"]
	if _, ok := pal["blue"]; ok {
		t.Error("old name 'blue' should not exist")
	}
	if pal["primary_blue"] != "#3274d9" {
		t.Errorf("primary_blue = %q, want #3274d9", pal["primary_blue"])
	}
}

func TestYAMLEditorRenamePaletteColorDuplicate(t *testing.T) {
	path := writeTestConfig(t, editorTestYAML)
	editor := NewYAMLEditor(path)
	err := editor.RenamePaletteColor("default", "blue", "green")
	if err == nil {
		t.Fatal("expected error when renaming to existing color")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists'", err)
	}
}

func TestYAMLEditorAddPalette(t *testing.T) {
	path := writeTestConfig(t, editorTestYAML)
	editor := NewYAMLEditor(path)
	if err := editor.AddPalette("dark"); err != nil {
		t.Fatalf("AddPalette error: %v", err)
	}
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if _, ok := c.Palettes["dark"]; !ok {
		t.Error("palette 'dark' should exist")
	}
	if _, ok := c.Palettes["default"]; !ok {
		t.Error("palette 'default' should still exist")
	}
}

func TestYAMLEditorAddPaletteDuplicate(t *testing.T) {
	path := writeTestConfig(t, editorTestYAML)
	editor := NewYAMLEditor(path)
	err := editor.AddPalette("default")
	if err == nil {
		t.Fatal("expected error for duplicate palette")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists'", err)
	}
}

func TestYAMLEditorDeletePalette(t *testing.T) {
	path := writeTestConfig(t, editorTestYAML)
	editor := NewYAMLEditor(path)
	if err := editor.DeletePalette("default"); err != nil {
		t.Fatalf("DeletePalette error: %v", err)
	}
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if _, ok := c.Palettes["default"]; ok {
		t.Error("palette 'default' should be deleted")
	}
}

func TestYAMLEditorSetActivePalette(t *testing.T) {
	path := writeTestConfig(t, editorTestYAML)
	editor := NewYAMLEditor(path)
	// Add a second palette first
	if err := editor.AddPalette("dark"); err != nil {
		t.Fatalf("AddPalette error: %v", err)
	}
	if err := editor.SetActivePalette("dark"); err != nil {
		t.Fatalf("SetActivePalette error: %v", err)
	}
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if c.ActivePalette != "dark" {
		t.Errorf("active_palette = %q, want dark", c.ActivePalette)
	}
}

func TestYAMLEditorSetActivePaletteNotFound(t *testing.T) {
	path := writeTestConfig(t, editorTestYAML)
	editor := NewYAMLEditor(path)
	err := editor.SetActivePalette("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent palette")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}

func TestYAMLEditorAddVariable(t *testing.T) {
	path := writeTestConfig(t, editorTestYAML)
	editor := NewYAMLEditor(path)
	if err := editor.AddVariable("namespace", VariableDef{
		Type:       "query",
		Datasource: "primary",
		Query:      "label_values(up, namespace)",
		Multi:      true,
		IncludeAll: true,
		Refresh:    2,
		Sort:       1,
		Regex:      ".*prod.*",
		DsType:     "prometheus",
	}); err != nil {
		t.Fatalf("AddVariable error: %v", err)
	}
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	v, ok := c.Variables["namespace"]
	if !ok {
		t.Fatal("variable 'namespace' should exist")
	}
	if v.Type != "query" {
		t.Errorf("type = %q, want query", v.Type)
	}
	if v.Datasource != "primary" {
		t.Errorf("datasource = %q, want primary", v.Datasource)
	}
	if v.Query != "label_values(up, namespace)" {
		t.Errorf("query = %q, want label_values(up, namespace)", v.Query)
	}
	if !v.Multi {
		t.Error("multi should be true")
	}
	if !v.IncludeAll {
		t.Error("include_all should be true")
	}
	if v.Refresh != 2 {
		t.Errorf("refresh = %d, want 2", v.Refresh)
	}
	if v.Sort != 1 {
		t.Errorf("sort = %d, want 1", v.Sort)
	}
	if v.Regex != ".*prod.*" {
		t.Errorf("regex = %q, want .*prod.*", v.Regex)
	}
	if v.DsType != "prometheus" {
		t.Errorf("ds_type = %q, want prometheus", v.DsType)
	}
	// Verify original variable still exists
	if _, ok := c.Variables["cluster"]; !ok {
		t.Error("variable 'cluster' should still exist")
	}
}

func TestYAMLEditorAddVariableDuplicate(t *testing.T) {
	path := writeTestConfig(t, editorTestYAML)
	editor := NewYAMLEditor(path)
	err := editor.AddVariable("cluster", VariableDef{Type: "query"})
	if err == nil {
		t.Fatal("expected error for duplicate variable")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists'", err)
	}
}

func TestYAMLEditorDeleteVariable(t *testing.T) {
	path := writeTestConfig(t, editorTestYAML)
	editor := NewYAMLEditor(path)
	if err := editor.DeleteVariable("cluster"); err != nil {
		t.Fatalf("DeleteVariable error: %v", err)
	}
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if _, ok := c.Variables["cluster"]; ok {
		t.Error("variable 'cluster' should be deleted")
	}
}

func TestYAMLEditorDeleteVariableNotFound(t *testing.T) {
	path := writeTestConfig(t, editorTestYAML)
	editor := NewYAMLEditor(path)
	err := editor.DeleteVariable("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent variable")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}

func TestResolvedAnthropicAPIKey(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		envVar string
		envVal string
		want   string
	}{
		{"literal value", "sk-abc123", "", "", "sk-abc123"},
		{"empty string", "", "", "", ""},
		{"env var reference", "$TEST_ANTHROPIC_KEY", "TEST_ANTHROPIC_KEY", "resolved-key", "resolved-key"},
		{"env var unset", "$UNSET_KEY_XYZ", "", "", "$UNSET_KEY_XYZ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" {
				t.Setenv(tt.envVar, tt.envVal)
			}
			g := GeneratorSettings{AnthropicAPIKey: tt.key}
			got := g.ResolvedAnthropicAPIKey()
			if got != tt.want {
				t.Errorf("ResolvedAnthropicAPIKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGeneratorSettingsString(t *testing.T) {
	g := GeneratorSettings{
		SchemaVersion:   42,
		OutputDir:       "/tmp/out",
		Refresh:         "30s",
		AnthropicAPIKey: "sk-secret-key",
		AnthropicModel:  "claude-3",
	}
	s := g.String()
	if strings.Contains(s, "sk-secret-key") {
		t.Error("String() should mask AnthropicAPIKey")
	}
	if !strings.Contains(s, "***") {
		t.Error("String() should contain *** for masked key")
	}
	if !strings.Contains(s, "42") {
		t.Error("String() should contain SchemaVersion")
	}
	if !strings.Contains(s, "/tmp/out") {
		t.Error("String() should contain OutputDir")
	}
	if !strings.Contains(s, "claude-3") {
		t.Error("String() should contain AnthropicModel")
	}

	// Empty key should not produce ***
	g2 := GeneratorSettings{SchemaVersion: 1}
	s2 := g2.String()
	if strings.Contains(s2, "***") {
		t.Error("String() should not mask empty key")
	}
}

func TestLoadFromBytes(t *testing.T) {
	yamlBytes := []byte(`
datasources:
  primary:
    type: prometheus
    uid: prom1
dashboards:
  overview:
    uid: gen-overview
    title: Overview
    sections: []
  compute:
    uid: gen-compute
    title: Compute
    sections: []
`)
	cfg, err := LoadFromBytes(yamlBytes)
	if err != nil {
		t.Fatalf("LoadFromBytes() error: %v", err)
	}
	if len(cfg.Dashboards) != 2 {
		t.Errorf("dashboard count = %d, want 2", len(cfg.Dashboards))
	}
	if _, ok := cfg.Dashboards["overview"]; !ok {
		t.Error("expected 'overview' dashboard")
	}
	if cfg.Datasources["primary"].UID != "prom1" {
		t.Errorf("datasource uid = %q, want prom1", cfg.Datasources["primary"].UID)
	}
}

func TestLoadFromBytesInvalid(t *testing.T) {
	_, err := LoadFromBytes([]byte(`{{{invalid yaml`))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestGetDatasourceURL(t *testing.T) {
	cfg := `
datasources:
  primary:
    type: prometheus
    uid: prom
    url: http://localhost:9090
    is_default: true
  secondary:
    type: prometheus
    uid: prom2
    url: http://remote:9090
dashboards: {}
`
	// Without CLI override
	path := writeTestConfig(t, cfg)
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if got := c.GetDatasourceURL("primary"); got != "http://localhost:9090" {
		t.Errorf("GetDatasourceURL(primary) = %q, want http://localhost:9090", got)
	}
	if got := c.GetDatasourceURL("secondary"); got != "http://remote:9090" {
		t.Errorf("GetDatasourceURL(secondary) = %q, want http://remote:9090", got)
	}
	if got := c.GetDatasourceURL("nonexistent"); got != "" {
		t.Errorf("GetDatasourceURL(nonexistent) = %q, want empty", got)
	}

	// With CLI prometheus_url override — should only apply to default/first DS
	path2 := writeTestConfig(t, cfg)
	c2, err := Load(path2, map[string]string{"prometheus_url": "http://override:9090"})
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if got := c2.GetDatasourceURL("primary"); got != "http://override:9090" {
		t.Errorf("GetDatasourceURL(primary) with override = %q, want http://override:9090", got)
	}
	// secondary should NOT be overridden
	if got := c2.GetDatasourceURL("secondary"); got != "http://remote:9090" {
		t.Errorf("GetDatasourceURL(secondary) with override = %q, want http://remote:9090", got)
	}
}

func TestGetVariableDef(t *testing.T) {
	cfg := `
datasources:
  primary:
    type: prometheus
    uid: prom
variables:
  instance:
    type: query
    datasource: primary
    query: 'label_values(up, instance)'
dashboards: {}
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	v, ok := c.GetVariableDef("instance")
	if !ok {
		t.Fatal("expected to find variable 'instance'")
	}
	if v.Type != "query" {
		t.Errorf("variable type = %q, want query", v.Type)
	}
	if v.Datasource != "primary" {
		t.Errorf("variable datasource = %q, want primary", v.Datasource)
	}

	_, ok = c.GetVariableDef("nonexistent")
	if ok {
		t.Error("expected false for nonexistent variable")
	}
}

func TestGetDiscovery(t *testing.T) {
	cfg := `
datasources:
  primary:
    type: prometheus
    uid: prom
discovery:
  enabled: true
  sources: [primary]
  include_patterns: ["node_*"]
  exclude_patterns: ["node_scrape_*"]
dashboards: {}
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	d := c.GetDiscovery()
	if !d.Enabled {
		t.Error("expected discovery enabled")
	}
	if len(d.Sources) != 1 || d.Sources[0] != "primary" {
		t.Errorf("discovery sources = %v, want [primary]", d.Sources)
	}
	if len(d.IncludePatterns) != 1 || d.IncludePatterns[0] != "node_*" {
		t.Errorf("include_patterns = %v, want [node_*]", d.IncludePatterns)
	}
	if len(d.ExcludePatterns) != 1 || d.ExcludePatterns[0] != "node_scrape_*" {
		t.Errorf("exclude_patterns = %v, want [node_scrape_*]", d.ExcludePatterns)
	}
}

func TestValidateWarnings(t *testing.T) {
	cfg := `
datasources:
  primary:
    type: prometheus
    uid: prom
variables:
  instance:
    type: query
    datasource: nonexistent_ds
    chains_from: [missing_var]
profiles:
  bad_profile:
    dashboards: [missing_dashboard]
dashboards:
  overview:
    uid: gen-overview
    title: Overview
    variables: [undefined_var]
    sections: []
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(c.Warnings) == 0 {
		t.Fatal("expected warnings, got none")
	}

	wantWarnings := []string{
		"undefined variable 'undefined_var'",
		"undefined datasource 'nonexistent_ds'",
		"undefined variable 'missing_var'",
		"undefined dashboard 'missing_dashboard'",
	}
	for _, want := range wantWarnings {
		found := false
		for _, w := range c.Warnings {
			if strings.Contains(w, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected warning containing %q, got warnings: %v", want, c.Warnings)
		}
	}
}

func TestGetDashboardOrderNoProfile(t *testing.T) {
	yaml := `
dashboards:
  alpha:
    uid: alpha
    title: alpha
    filename: alpha.json
    sections: []
  beta:
    uid: beta
    title: beta
    filename: beta.json
    sections: []
  gamma:
    uid: gamma
    title: gamma
    filename: gamma.json
    sections: []
`
	cfg, err := LoadFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}
	order, err := cfg.GetDashboardOrder("")
	if err != nil {
		t.Fatalf("GetDashboardOrder: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 dashboards, got %d: %v", len(order), order)
	}
	// YAML order should be alpha, beta, gamma
	if order[0] != "alpha" || order[1] != "beta" || order[2] != "gamma" {
		t.Errorf("expected [alpha beta gamma], got %v", order)
	}
}

func TestGetDashboardOrderBadProfile(t *testing.T) {
	cfg, _ := LoadFromBytes([]byte(`dashboards: {}`))
	_, err := cfg.GetDashboardOrder("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestGetDefaultDatasourceNoDefault(t *testing.T) {
	yaml := `
datasources:
  myds:
    type: prometheus
    uid: myds_uid
`
	cfg, err := LoadFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}
	ds := cfg.GetDefaultDatasource()
	if ds.UID != "myds_uid" {
		t.Errorf("expected UID myds_uid, got %s", ds.UID)
	}
}

func TestGetDefaultDatasourceEmpty(t *testing.T) {
	cfg, err := LoadFromBytes([]byte(`{}`))
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}
	ds := cfg.GetDefaultDatasource()
	if ds.Type != "prometheus" || ds.UID != "prometheus" {
		t.Errorf("expected fallback {prometheus, prometheus}, got {%s, %s}", ds.Type, ds.UID)
	}
}