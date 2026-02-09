package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wcatz/dashboard-generator/internal/config"
)

func loadTestConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := `
datasources:
  primary:
    type: prometheus
    uid: prometheus
    is_default: true
  secondary:
    type: prometheus
    uid: thanos
palettes:
  grafana:
    green: "#73BF69"
    red: "#F2495C"
    blue: "#5794F2"
active_palette: grafana
thresholds:
  percent_usage:
    - { color: "$green", value: null }
    - { color: "$red", value: 90 }
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
dashboards: {}
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

func TestStatPanel(t *testing.T) {
	cfg := loadTestConfig(t)
	idGen := NewIDGenerator()
	pf := NewPanelFactory(cfg, idGen)

	panel := pf.Stat(map[string]interface{}{
		"title": "targets up",
		"query": "count(up == 1)",
		"color": "$green",
		"width": 4,
	}, 0, 0)

	if panel["type"] != "stat" {
		t.Errorf("type = %v, want stat", panel["type"])
	}
	if panel["title"] != "targets up" {
		t.Errorf("title = %v, want targets up", panel["title"])
	}
	gridPos := panel["gridPos"].(map[string]interface{})
	if gridPos["w"] != 4 {
		t.Errorf("width = %v, want 4", gridPos["w"])
	}
	if gridPos["h"] != 4 { // default height
		t.Errorf("height = %v, want 4", gridPos["h"])
	}

	targets := panel["targets"].([]interface{})
	if len(targets) != 1 {
		t.Fatalf("targets count = %d, want 1", len(targets))
	}
	target := targets[0].(map[string]interface{})
	if target["expr"] != "count(up == 1)" {
		t.Errorf("expr = %v, want count(up == 1)", target["expr"])
	}
}

func TestTimeseriesPanel(t *testing.T) {
	cfg := loadTestConfig(t)
	idGen := NewIDGenerator()
	pf := NewPanelFactory(cfg, idGen)

	panel := pf.Timeseries(map[string]interface{}{
		"title":      "cpu usage",
		"query":      "rate(cpu[${rate_interval}])",
		"unit":       "percent",
		"stack":      "normal",
		"fill_opacity": 20,
	}, 0, 0)

	if panel["type"] != "timeseries" {
		t.Errorf("type = %v, want timeseries", panel["type"])
	}
	fc := panel["fieldConfig"].(map[string]interface{})
	defaults := fc["defaults"].(map[string]interface{})
	custom := defaults["custom"].(map[string]interface{})
	if custom["fillOpacity"] != 20 {
		t.Errorf("fillOpacity = %v, want 20", custom["fillOpacity"])
	}
	stacking := custom["stacking"].(map[string]interface{})
	if stacking["mode"] != "normal" {
		t.Errorf("stacking mode = %v, want normal", stacking["mode"])
	}

	// Check ref resolution
	targets := panel["targets"].([]interface{})
	target := targets[0].(map[string]interface{})
	if target["expr"] != "rate(cpu[5m])" {
		t.Errorf("expr = %v, want rate(cpu[5m])", target["expr"])
	}
}

func TestGaugePanel(t *testing.T) {
	cfg := loadTestConfig(t)
	idGen := NewIDGenerator()
	pf := NewPanelFactory(cfg, idGen)

	panel := pf.Gauge(map[string]interface{}{
		"title":      "cpu usage",
		"query":      "avg(cpu)",
		"thresholds": "$percent_usage",
		"unit":       "percent",
		"max":        100,
	}, 0, 0)

	if panel["type"] != "gauge" {
		t.Errorf("type = %v, want gauge", panel["type"])
	}
	fc := panel["fieldConfig"].(map[string]interface{})
	defaults := fc["defaults"].(map[string]interface{})
	thresholds := defaults["thresholds"].(map[string]interface{})
	steps := thresholds["steps"].([]interface{})
	if len(steps) != 2 {
		t.Errorf("threshold steps = %d, want 2", len(steps))
	}
}

func TestComparisonPanel(t *testing.T) {
	cfg := loadTestConfig(t)
	idGen := NewIDGenerator()
	pf := NewPanelFactory(cfg, idGen)

	panel, err := pf.Comparison(map[string]interface{}{
		"title":       "cpu comparison",
		"metric":      "node_cpu_seconds_total",
		"metric_type": "counter",
		"datasources": []interface{}{"primary", "secondary"},
	}, 0, 0)
	if err != nil {
		t.Fatalf("Comparison error: %v", err)
	}

	if panel["type"] != "timeseries" {
		t.Errorf("type = %v, want timeseries", panel["type"])
	}
	ds := panel["datasource"].(map[string]interface{})
	if ds["uid"] != "-- Mixed --" {
		t.Errorf("datasource uid = %v, want -- Mixed --", ds["uid"])
	}
	targets := panel["targets"].([]interface{})
	if len(targets) != 2 {
		t.Errorf("targets = %d, want 2", len(targets))
	}
	// Counter should use rate()
	t0 := targets[0].(map[string]interface{})
	if t0["expr"] != "rate(node_cpu_seconds_total[5m])" {
		t.Errorf("expr = %v, want rate(node_cpu_seconds_total[5m])", t0["expr"])
	}
}

func TestComparisonTooFewDS(t *testing.T) {
	cfg := loadTestConfig(t)
	idGen := NewIDGenerator()
	pf := NewPanelFactory(cfg, idGen)

	_, err := pf.Comparison(map[string]interface{}{
		"datasources": []interface{}{"primary"},
	}, 0, 0)
	if err == nil {
		t.Error("expected error for <2 datasources")
	}
}

func TestMultiTargetPanel(t *testing.T) {
	cfg := loadTestConfig(t)
	idGen := NewIDGenerator()
	pf := NewPanelFactory(cfg, idGen)

	panel := pf.Timeseries(map[string]interface{}{
		"title": "multi target",
		"targets": []interface{}{
			map[string]interface{}{"expr": "metric_a", "legend": "a"},
			map[string]interface{}{"expr": "metric_b", "legend": "b"},
			map[string]interface{}{"expr": "metric_c", "legend": "c", "datasource": "secondary"},
		},
	}, 0, 0)

	targets := panel["targets"].([]interface{})
	if len(targets) != 3 {
		t.Fatalf("targets = %d, want 3", len(targets))
	}
	t0 := targets[0].(map[string]interface{})
	if t0["refId"] != "A" {
		t.Errorf("refId[0] = %v, want A", t0["refId"])
	}
	t2 := targets[2].(map[string]interface{})
	ds := t2["datasource"].(map[string]interface{})
	if ds["uid"] != "thanos" {
		t.Errorf("target[2] datasource = %v, want thanos", ds["uid"])
	}
}

func TestRowPanel(t *testing.T) {
	cfg := loadTestConfig(t)
	idGen := NewIDGenerator()
	pf := NewPanelFactory(cfg, idGen)

	row := pf.Row("test section", 5, false, nil, "")
	if row["type"] != "row" {
		t.Errorf("type = %v, want row", row["type"])
	}
	gridPos := row["gridPos"].(map[string]interface{})
	if gridPos["y"] != 5 {
		t.Errorf("y = %v, want 5", gridPos["y"])
	}
	if row["collapsed"] != false {
		t.Errorf("collapsed = %v, want false", row["collapsed"])
	}
}

func TestRowPanelWithRepeat(t *testing.T) {
	cfg := loadTestConfig(t)
	idGen := NewIDGenerator()
	pf := NewPanelFactory(cfg, idGen)

	row := pf.Row("repeated", 0, false, nil, "instance")
	if row["repeat"] != "instance" {
		t.Errorf("repeat = %v, want instance", row["repeat"])
	}
	if row["repeatDirection"] != "h" {
		t.Errorf("repeatDirection = %v, want h", row["repeatDirection"])
	}
}

func TestPanelIDIncrement(t *testing.T) {
	cfg := loadTestConfig(t)
	idGen := NewIDGenerator()
	pf := NewPanelFactory(cfg, idGen)

	p1 := pf.Stat(map[string]interface{}{"title": "a", "query": "1"}, 0, 0)
	p2 := pf.Stat(map[string]interface{}{"title": "b", "query": "2"}, 0, 0)

	id1 := p1["id"].(int)
	id2 := p2["id"].(int)
	if id2 != id1+1 {
		t.Errorf("id2 = %d, want %d", id2, id1+1)
	}

	idGen.Reset()
	p3 := pf.Stat(map[string]interface{}{"title": "c", "query": "3"}, 0, 0)
	id3 := p3["id"].(int)
	if id3 != 1 {
		t.Errorf("id after reset = %d, want 1", id3)
	}
}

func TestFromConfig(t *testing.T) {
	cfg := loadTestConfig(t)
	idGen := NewIDGenerator()
	pf := NewPanelFactory(cfg, idGen)

	types := []string{
		"stat", "gauge", "timeseries", "bargauge", "heatmap",
		"histogram", "table", "piechart", "state-timeline",
		"status-history", "text", "logs",
	}

	for _, typ := range types {
		pcfg := map[string]interface{}{
			"type":  typ,
			"title": typ + " test",
		}
		if typ != "text" {
			pcfg["query"] = "up"
		}
		if typ == "text" {
			pcfg["content"] = "hello"
		}

		panel, err := pf.FromConfig(pcfg, 0, 0)
		if err != nil {
			t.Errorf("FromConfig(%s) error: %v", typ, err)
			continue
		}
		if panel["type"] != typ {
			t.Errorf("FromConfig(%s) type = %v", typ, panel["type"])
		}
	}

	// Test unknown type
	_, err := pf.FromConfig(map[string]interface{}{"type": "unknown"}, 0, 0)
	if err == nil {
		t.Error("expected error for unknown panel type")
	}
}

func TestDefaultPanelSizes(t *testing.T) {
	for ptype, size := range DefaultSizes {
		if size[0] <= 0 || size[1] <= 0 {
			t.Errorf("DefaultSizes[%s] has invalid size: %v", ptype, size)
		}
		if size[0] > 24 {
			t.Errorf("DefaultSizes[%s] width %d > 24", ptype, size[0])
		}
	}
}
