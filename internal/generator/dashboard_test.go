package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wcatz/dashboard-generator/internal/config"
)

func loadFullTestConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := `
generator:
  schema_version: 39
  refresh: "30s"
  time_range:
    from: "now-30m"
    to: "now"
  editable: true
  graph_tooltip: 1
  live_now: true
datasources:
  primary:
    type: prometheus
    uid: prometheus
    is_default: true
palettes:
  grafana:
    green: "#73BF69"
    red: "#F2495C"
    blue: "#5794F2"
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
  namespace:
    type: query
    datasource: primary
    query: 'label_values(kube_pod_info, namespace)'
    multi: true
    include_all: true
    refresh: 2
    sort: 1
    label: namespace
  instance:
    type: query
    datasource: primary
    query: 'label_values(up, instance)'
    multi: true
    include_all: true
  interval:
    type: interval
    values: "1m,5m,15m,30m,1h"
    auto: true
    auto_count: 10
    auto_min: "10s"
    label: interval
dashboards:
  overview:
    uid: gen-overview
    title: overview
    filename: gen-overview.json
    tags: [overview, generated]
    icon: apps
    description: "cluster health"
    variables: [namespace, instance]
    sections:
      - title: cluster health
        panels:
          - type: stat
            title: targets up
            query: 'count(up == 1)'
            color: "$green"
          - type: stat
            title: targets down
            query: 'count(up == 0)'
            color: "$red"
      - title: details
        collapsed: true
        panels:
          - type: timeseries
            title: cpu usage
            query: 'rate(cpu[${rate_interval}])'
            unit: percent
  compute:
    uid: gen-compute
    title: compute
    filename: gen-compute.json
    tags: [compute]
    icon: bolt
    sections:
      - title: cpu
        panels:
          - type: stat
            title: load
            query: 'node_load1'
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestBuildNavigationLinks(t *testing.T) {
	cfg := loadFullTestConfig(t)
	idGen := NewIDGenerator()
	pf := NewPanelFactory(cfg, idGen)
	le := NewLayoutEngine()
	builder := NewDashboardBuilder(cfg, pf, le)

	dbs, _ := cfg.GetDashboards("")
	order := []string{"overview", "compute"}
	links := builder.BuildNavigationLinks(dbs, order)

	if len(links) != 2 {
		t.Fatalf("link count = %d, want 2", len(links))
	}
	l0 := links[0].(map[string]interface{})
	if l0["title"] != "overview" {
		t.Errorf("link[0] title = %v, want overview", l0["title"])
	}
	if l0["url"] != "/d/gen-overview" {
		t.Errorf("link[0] url = %v, want /d/gen-overview", l0["url"])
	}
	if l0["keepTime"] != true {
		t.Error("link keepTime should be true")
	}
	if l0["includeVars"] != true {
		t.Error("link includeVars should be true")
	}
}

func TestBuildVariable(t *testing.T) {
	cfg := loadFullTestConfig(t)
	idGen := NewIDGenerator()
	pf := NewPanelFactory(cfg, idGen)
	le := NewLayoutEngine()
	builder := NewDashboardBuilder(cfg, pf, le)

	// Test query variable
	v, err := builder.BuildVariable("namespace")
	if err != nil {
		t.Fatalf("BuildVariable error: %v", err)
	}
	if v["type"] != "query" {
		t.Errorf("type = %v, want query", v["type"])
	}
	if v["name"] != "namespace" {
		t.Errorf("name = %v, want namespace", v["name"])
	}
	if v["multi"] != true {
		t.Error("multi should be true")
	}
	if v["includeAll"] != true {
		t.Error("includeAll should be true")
	}

	// Test interval variable
	iv, err := builder.BuildVariable("interval")
	if err != nil {
		t.Fatalf("BuildVariable(interval) error: %v", err)
	}
	if iv["type"] != "interval" {
		t.Errorf("type = %v, want interval", iv["type"])
	}
	if iv["auto"] != true {
		t.Error("auto should be true")
	}
	// interval shouldn't have datasource
	if _, ok := iv["datasource"]; ok {
		t.Error("interval variable should not have datasource")
	}

	// Test missing variable
	_, err = builder.BuildVariable("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent variable")
	}
}

func TestBuildDashboard(t *testing.T) {
	cfg := loadFullTestConfig(t)
	idGen := NewIDGenerator()
	pf := NewPanelFactory(cfg, idGen)
	le := NewLayoutEngine()
	builder := NewDashboardBuilder(cfg, pf, le)

	dbs, _ := cfg.GetDashboards("")
	dbCfg := dbs["overview"]

	navLinks := []interface{}{
		map[string]interface{}{"title": "overview", "url": "/d/gen-overview"},
	}

	dashboard, err := builder.Build(dbCfg, navLinks, nil)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}

	// Check top-level fields
	if dashboard["uid"] != "gen-overview" {
		t.Errorf("uid = %v, want gen-overview", dashboard["uid"])
	}
	if dashboard["title"] != "overview" {
		t.Errorf("title = %v, want overview", dashboard["title"])
	}
	if dashboard["schemaVersion"] != 39 {
		t.Errorf("schemaVersion = %v, want 39", dashboard["schemaVersion"])
	}
	if dashboard["refresh"] != "30s" {
		t.Errorf("refresh = %v, want 30s", dashboard["refresh"])
	}

	// Check panels
	panels := dashboard["panels"].([]interface{})
	// 2 sections: open section (row + 2 stat) + collapsed section (row with inner panels)
	// open: 1 row + 2 stat = 3 panels
	// collapsed: 1 row (with 1 inner timeseries) = 1 panel
	// total = 4 top-level panels
	if len(panels) != 4 {
		t.Errorf("panel count = %d, want 4", len(panels))
	}

	// First panel should be a row
	p0 := panels[0].(map[string]interface{})
	if p0["type"] != "row" {
		t.Errorf("panels[0] type = %v, want row", p0["type"])
	}

	// Second panel should be stat
	p1 := panels[1].(map[string]interface{})
	if p1["type"] != "stat" {
		t.Errorf("panels[1] type = %v, want stat", p1["type"])
	}

	// Last panel should be collapsed row with inner panels
	p3 := panels[3].(map[string]interface{})
	if p3["type"] != "row" {
		t.Errorf("panels[3] type = %v, want row", p3["type"])
	}
	if p3["collapsed"] != true {
		t.Error("last panel should be collapsed")
	}
	innerPanels := p3["panels"].([]interface{})
	if len(innerPanels) != 1 {
		t.Errorf("inner panels = %d, want 1", len(innerPanels))
	}

	// Check variables
	templating := dashboard["templating"].(map[string]interface{})
	varList := templating["list"].([]interface{})
	if len(varList) != 2 {
		t.Errorf("variable count = %d, want 2", len(varList))
	}

	// Check tags
	tags := dashboard["tags"].([]interface{})
	if len(tags) != 2 {
		t.Errorf("tag count = %d, want 2", len(tags))
	}
}
