package generator

import (
	"fmt"

	"github.com/wcatz/dashboard-generator/internal/config"
)

// DefaultSizes maps panel type to (width, height) defaults.
var DefaultSizes = map[string][2]int{
	"stat":           {3, 4},
	"gauge":          {3, 4},
	"timeseries":     {12, 7},
	"bargauge":       {6, 5},
	"heatmap":        {12, 8},
	"histogram":      {12, 7},
	"table":          {24, 8},
	"piechart":       {6, 6},
	"state-timeline": {12, 5},
	"status-history": {12, 5},
	"text":           {24, 3},
	"logs":           {24, 8},
	"barchart":       {8, 6},
	"row":            {24, 1},
	"comparison":     {12, 8},
	"alertlist":      {12, 5},
	"dashlist":       {12, 5},
	"trend":          {12, 7},
	"candlestick":    {12, 7},
	"news":           {12, 6},
	"xychart":        {12, 7},
	"geomap":         {12, 10},
	"nodeGraph":      {24, 10},
}

// PanelFactory creates Grafana panel JSON dicts.
type PanelFactory struct {
	Config *config.Config
	IDGen  *IDGenerator
}

// NewPanelFactory creates a new panel factory.
func NewPanelFactory(cfg *config.Config, idGen *IDGenerator) *PanelFactory {
	return &PanelFactory{Config: cfg, IDGen: idGen}
}

// FromConfig creates a panel from a config dict.
func (pf *PanelFactory) FromConfig(cfg map[string]interface{}, x, y int) (map[string]interface{}, error) {
	ptype := getString(cfg, "type", "")
	switch ptype {
	case "stat":
		return pf.Stat(cfg, x, y), nil
	case "gauge":
		return pf.Gauge(cfg, x, y), nil
	case "timeseries":
		return pf.Timeseries(cfg, x, y), nil
	case "bargauge":
		return pf.Bargauge(cfg, x, y), nil
	case "heatmap":
		return pf.Heatmap(cfg, x, y), nil
	case "histogram":
		return pf.Histogram(cfg, x, y), nil
	case "table":
		return pf.Table(cfg, x, y), nil
	case "piechart":
		return pf.Piechart(cfg, x, y), nil
	case "state-timeline":
		return pf.StateTimeline(cfg, x, y), nil
	case "status-history":
		return pf.StatusHistory(cfg, x, y), nil
	case "text":
		return pf.Text(cfg, x, y), nil
	case "logs":
		return pf.Logs(cfg, x, y), nil
	case "comparison":
		return pf.Comparison(cfg, x, y)
	case "alertlist":
		return pf.Alertlist(cfg, x, y), nil
	case "barchart":
		return pf.Barchart(cfg, x, y), nil
	case "dashlist":
		return pf.Dashlist(cfg, x, y), nil
	case "trend":
		return pf.Trend(cfg, x, y), nil
	case "candlestick":
		return pf.Candlestick(cfg, x, y), nil
	case "news":
		return pf.News(cfg, x, y), nil
	case "xychart":
		return pf.XYChart(cfg, x, y), nil
	case "geomap":
		return pf.Geomap(cfg, x, y), nil
	case "nodeGraph", "node-graph":
		return pf.NodeGraph(cfg, x, y), nil
	default:
		return nil, fmt.Errorf("unknown panel type: %s", ptype)
	}
}

func (pf *PanelFactory) ds(cfg map[string]interface{}) map[string]interface{} {
	dsName := getString(cfg, "datasource", "")
	if dsName != "" {
		ref, err := pf.Config.GetDatasource(dsName)
		if err == nil {
			return map[string]interface{}{"type": ref.Type, "uid": ref.UID}
		}
	}
	def := pf.Config.GetDefaultDatasource()
	return map[string]interface{}{"type": def.Type, "uid": def.UID}
}

func (pf *PanelFactory) target(expr, legend, refID string, datasource map[string]interface{}) map[string]interface{} {
	if datasource == nil {
		def := pf.Config.GetDefaultDatasource()
		datasource = map[string]interface{}{"type": def.Type, "uid": def.UID}
	}
	return map[string]interface{}{
		"datasource":   datasource,
		"editorMode":   "code",
		"expr":         pf.Config.ResolveRef(expr),
		"legendFormat": legend,
		"range":        true,
		"refId":        refID,
	}
}

func (pf *PanelFactory) buildTargets(cfg map[string]interface{}, datasource map[string]interface{}) []interface{} {
	var targets []interface{}
	if datasource == nil {
		datasource = pf.ds(cfg)
	}

	if query, ok := cfg["query"].(string); ok {
		legend := getString(cfg, "legend", "{{instance}}")
		targets = append(targets, pf.target(query, legend, "A", datasource))
	}

	if targetList, ok := cfg["targets"].([]interface{}); ok {
		for i, item := range targetList {
			t, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			tDS := datasource
			if dsName, ok := t["datasource"].(string); ok {
				ref, err := pf.Config.GetDatasource(dsName)
				if err == nil {
					tDS = map[string]interface{}{"type": ref.Type, "uid": ref.UID}
				}
			}
			legend := getString(t, "legend", "{{instance}}")
			refID := string(rune('A' + i))
			expr := getString(t, "expr", "")
			targets = append(targets, pf.target(expr, legend, refID, tDS))
		}
	}

	return targets
}

func (pf *PanelFactory) thresholds(cfg map[string]interface{}, defaultColor string) []interface{} {
	if t, ok := cfg["thresholds"]; ok {
		resolved := pf.Config.ResolveThresholds(t)
		if len(resolved) > 0 {
			steps := make([]interface{}, len(resolved))
			for i, s := range resolved {
				steps[i] = map[string]interface{}{"color": s.Color, "value": s.Value}
			}
			return steps
		}
	}
	color := defaultColor
	if color == "" {
		color = "#73BF69"
	}
	if c, ok := cfg["color"].(string); ok && c != "" {
		color = pf.Config.ResolveColor(c)
	}
	return []interface{}{map[string]interface{}{"color": color, "value": nil}}
}

func (pf *PanelFactory) overrides(cfg map[string]interface{}) []interface{} {
	if o, ok := cfg["overrides"].([]interface{}); ok {
		return o
	}
	return []interface{}{}
}

func (pf *PanelFactory) valueMappings(cfg map[string]interface{}) []interface{} {
	if m, ok := cfg["value_mappings"].([]interface{}); ok {
		return m
	}
	return []interface{}{}
}

func (pf *PanelFactory) dataLinks(cfg map[string]interface{}) []interface{} {
	if l, ok := cfg["data_links"].([]interface{}); ok {
		return l
	}
	return []interface{}{}
}

// applyTransformations adds Grafana transformations to a panel if configured.
func (pf *PanelFactory) applyTransformations(panel map[string]interface{}, cfg map[string]interface{}) {
	if t, ok := cfg["transformations"].([]interface{}); ok {
		panel["transformations"] = t
	}
}

// applyRepeat adds panel-level repeat config for multi-value variables.
func (pf *PanelFactory) applyRepeat(panel map[string]interface{}, cfg map[string]interface{}) {
	if r := getString(cfg, "repeat", ""); r != "" {
		panel["repeat"] = r
		panel["repeatDirection"] = getString(cfg, "repeat_direction", "h")
		if mr := getInt(cfg, "max_per_row", 0); mr > 0 {
			panel["maxPerRow"] = mr
		}
	}
}

// Row creates a row panel.
func (pf *PanelFactory) Row(title string, y int, collapsed bool, panels []interface{}, repeat string) map[string]interface{} {
	if panels == nil {
		panels = []interface{}{}
	}
	r := map[string]interface{}{
		"collapsed": collapsed,
		"gridPos":   map[string]interface{}{"h": 1, "w": 24, "x": 0, "y": y},
		"id":        pf.IDGen.Next(),
		"panels":    panels,
		"title":     title,
		"type":      "row",
	}
	if repeat != "" {
		r["repeat"] = repeat
		r["repeatDirection"] = "h"
	}
	return r
}

// Stat creates a stat panel.
func (pf *PanelFactory) Stat(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["stat"][0], DefaultSizes["stat"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	steps := pf.thresholds(cfg, "")
	color := pf.Config.ResolveColor(getString(cfg, "color", ""))
	if color != "" && len(steps) == 1 {
		steps = []interface{}{map[string]interface{}{"color": color, "value": nil}}
	}
	panel := map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color":      map[string]interface{}{"mode": "thresholds"},
				"mappings":   pf.valueMappings(cfg),
				"thresholds": map[string]interface{}{"mode": "absolute", "steps": steps},
				"unit":       getString(cfg, "unit", "none"),
				"links":      pf.dataLinks(cfg),
			},
			"overrides": pf.overrides(cfg),
		},
		"gridPos": map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":      pf.IDGen.Next(),
		"options": map[string]interface{}{
			"colorMode":   getString(cfg, "color_mode", "background"),
			"graphMode":   getString(cfg, "graph_mode", "none"),
			"justifyMode": "center",
			"orientation": "auto",
			"reduceOptions": map[string]interface{}{
				"calcs":  getStringSlice(cfg, "calcs", []string{"lastNotNull"}),
				"fields": "",
				"values": false,
			},
			"showPercentChange": false,
			"textMode":          getString(cfg, "text_mode", "value_and_name"),
			"wideLayout":        true,
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "stat",
	}
	pf.applyTransformations(panel, cfg)
	pf.applyRepeat(panel, cfg)
	return panel
}

// Gauge creates a gauge panel.
func (pf *PanelFactory) Gauge(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["gauge"][0], DefaultSizes["gauge"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	panel := map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color":      map[string]interface{}{"mode": "thresholds"},
				"mappings":   pf.valueMappings(cfg),
				"max":        getNumber(cfg, "max", 100),
				"min":        getNumber(cfg, "min", 0),
				"thresholds": map[string]interface{}{"mode": "absolute", "steps": pf.thresholds(cfg, "")},
				"unit":       getString(cfg, "unit", "percent"),
				"links":      pf.dataLinks(cfg),
			},
			"overrides": pf.overrides(cfg),
		},
		"gridPos": map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":      pf.IDGen.Next(),
		"options": map[string]interface{}{
			"minVizHeight": 75,
			"minVizWidth":  75,
			"orientation":  "auto",
			"reduceOptions": map[string]interface{}{
				"calcs":  getStringSlice(cfg, "calcs", []string{"lastNotNull"}),
				"fields": "",
				"values": false,
			},
			"showThresholdLabels":  getBool(cfg, "show_threshold_labels", false),
			"showThresholdMarkers": getBool(cfg, "show_threshold_markers", true),
			"sizing":               "auto",
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "gauge",
	}
	pf.applyTransformations(panel, cfg)
	pf.applyRepeat(panel, cfg)
	return panel
}

// Timeseries creates a timeseries panel.
func (pf *PanelFactory) Timeseries(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["timeseries"][0], DefaultSizes["timeseries"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	fill := getInt(cfg, "fill_opacity", 8)
	line := getInt(cfg, "line_width", 1)
	stack := getString(cfg, "stack", "none")
	draw := getString(cfg, "draw_style", "line")
	interpolation := getString(cfg, "line_interpolation", "smooth")
	panel := map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color": map[string]interface{}{"mode": getString(cfg, "color_mode", "palette-classic-by-name")},
				"custom": map[string]interface{}{
					"axisBorderShow":    false,
					"axisCenteredZero":  false,
					"axisColorMode":     "text",
					"axisLabel":         getString(cfg, "axis_label", ""),
					"axisPlacement":     "auto",
					"barAlignment":      0,
					"barWidthFactor":    0.6,
					"drawStyle":         draw,
					"fillOpacity":       fill,
					"gradientMode":      "scheme",
					"hideFrom":          map[string]interface{}{"legend": false, "tooltip": false, "viz": false},
					"insertNulls":       false,
					"lineInterpolation": interpolation,
					"lineWidth":         line,
					"pointSize":         5,
					"scaleDistribution": map[string]interface{}{"type": "linear"},
					"showPoints":        "never",
					"spanNulls":         false,
					"stacking":          map[string]interface{}{"group": "A", "mode": stack},
					"thresholdsStyle":   map[string]interface{}{"mode": "off"},
				},
				"mappings":   pf.valueMappings(cfg),
				"thresholds": map[string]interface{}{"mode": "absolute", "steps": pf.thresholds(cfg, "")},
				"unit":       getString(cfg, "unit", "short"),
				"links":      pf.dataLinks(cfg),
			},
			"overrides": pf.overrides(cfg),
		},
		"gridPos": map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":      pf.IDGen.Next(),
		"options": map[string]interface{}{
			"legend": map[string]interface{}{
				"calcs":       getStringSlice(cfg, "legend_calcs", []string{}),
				"displayMode": getString(cfg, "legend_mode", "list"),
				"placement":   getString(cfg, "legend_placement", "bottom"),
				"showLegend":  getBool(cfg, "show_legend", true),
			},
			"tooltip": map[string]interface{}{"mode": "multi", "sort": "desc"},
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "timeseries",
	}
	pf.applyTransformations(panel, cfg)
	pf.applyRepeat(panel, cfg)
	return panel
}

// Bargauge creates a bar gauge panel.
func (pf *PanelFactory) Bargauge(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["bargauge"][0], DefaultSizes["bargauge"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	panel := map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color":      map[string]interface{}{"mode": "thresholds"},
				"mappings":   pf.valueMappings(cfg),
				"max":        getNumber(cfg, "max", 100),
				"min":        getNumber(cfg, "min", 0),
				"thresholds": map[string]interface{}{"mode": "absolute", "steps": pf.thresholds(cfg, "")},
				"unit":       getString(cfg, "unit", "percent"),
				"links":      pf.dataLinks(cfg),
			},
			"overrides": pf.overrides(cfg),
		},
		"gridPos": map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":      pf.IDGen.Next(),
		"options": map[string]interface{}{
			"displayMode":   getString(cfg, "display_mode", "gradient"),
			"maxVizHeight":  300,
			"minVizHeight":  16,
			"minVizWidth":   8,
			"namePlacement": "auto",
			"orientation":   getString(cfg, "orientation", "horizontal"),
			"reduceOptions": map[string]interface{}{
				"calcs":  getStringSlice(cfg, "calcs", []string{"lastNotNull"}),
				"fields": "",
				"values": false,
			},
			"showUnfilled": true,
			"sizing":       "auto",
			"valueMode":    "color",
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "bargauge",
	}
	pf.applyTransformations(panel, cfg)
	pf.applyRepeat(panel, cfg)
	return panel
}

// Barchart creates a bar chart panel.
// Supports x_field, color_by_field, stacking, bar_width, bar_radius, group_width,
// axis_soft_max, and per-target overrides for units, axis placement, and thresholds.
func (pf *PanelFactory) Barchart(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["barchart"][0], DefaultSizes["barchart"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)

	custom := map[string]interface{}{
		"lineWidth":         getInt(cfg, "line_width", 0),
		"fillOpacity":       getInt(cfg, "fill_opacity", 99),
		"gradientMode":      getString(cfg, "gradient_mode", "none"),
		"axisPlacement":     getString(cfg, "axis_placement", "hidden"),
		"axisLabel":         getString(cfg, "axis_label", ""),
		"axisColorMode":     getString(cfg, "axis_color_mode", "series"),
		"axisBorderShow":    getBool(cfg, "axis_border_show", true),
		"axisCenteredZero":  false,
		"scaleDistribution": map[string]interface{}{"type": "linear"},
		"hideFrom":          map[string]interface{}{"tooltip": false, "viz": false, "legend": false},
		"thresholdsStyle":   map[string]interface{}{"mode": "off"},
		"axisGridShow":      getBool(cfg, "axis_grid_show", true),
	}

	if v, ok := cfg["axis_soft_max"]; ok {
		custom["axisSoftMax"] = v
	}

	opts := map[string]interface{}{
		"orientation":        getString(cfg, "orientation", "auto"),
		"xTickLabelRotation": getInt(cfg, "x_tick_rotation", 0),
		"xTickLabelSpacing":  getInt(cfg, "x_tick_spacing", 200),
		"showValue":          getString(cfg, "show_value", "auto"),
		"stacking":           getString(cfg, "stacking", "normal"),
		"groupWidth":         getFloat(cfg, "group_width", 0),
		"barWidth":           getFloat(cfg, "bar_width", 0.83),
		"barRadius":          getFloat(cfg, "bar_radius", 0),
		"fullHighlight":      false,
		"tooltip":            map[string]interface{}{"mode": "multi", "sort": "none", "hideZeros": false},
		"legend": map[string]interface{}{
			"showLegend":  getBool(cfg, "show_legend", false),
			"displayMode": getString(cfg, "legend_mode", "list"),
			"placement":   getString(cfg, "legend_placement", "bottom"),
			"calcs":       getStringSlice(cfg, "legend_calcs", []string{"lastNotNull"}),
		},
		"text":                map[string]interface{}{"valueSize": 1},
		"xTickLabelMaxLength": 0,
	}

	if xf := getString(cfg, "x_field", ""); xf != "" {
		opts["xField"] = xf
	}
	if cbf := getString(cfg, "color_by_field", ""); cbf != "" {
		opts["colorByField"] = cbf
	}

	panel := map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color":      map[string]interface{}{"mode": "thresholds", "fixedColor": pf.Config.ResolveColor(getString(cfg, "color", "$blue"))},
				"mappings":   pf.valueMappings(cfg),
				"thresholds": map[string]interface{}{"mode": "absolute", "steps": pf.thresholds(cfg, "")},
				"unit":       getString(cfg, "unit", "none"),
				"custom":     custom,
			},
			"overrides": pf.overrides(cfg),
		},
		"gridPos":       map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":            pf.IDGen.Next(),
		"options":       opts,
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "barchart",
	}
	pf.applyTransformations(panel, cfg)
	pf.applyRepeat(panel, cfg)
	return panel
}

// Heatmap creates a heatmap panel.
func (pf *PanelFactory) Heatmap(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["heatmap"][0], DefaultSizes["heatmap"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	scheme := getString(cfg, "color_scheme", "Spectral")
	panel := map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color": map[string]interface{}{"mode": "continuous-GrYlRd"},
				"custom": map[string]interface{}{
					"fillOpacity": 80,
					"hideFrom":    map[string]interface{}{"legend": false, "tooltip": false, "viz": false},
					"lineWidth":   1,
				},
				"mappings":   []interface{}{},
				"thresholds": map[string]interface{}{"mode": "absolute", "steps": []interface{}{map[string]interface{}{"color": "green", "value": nil}}},
				"unit":       getString(cfg, "unit", "short"),
			},
			"overrides": pf.overrides(cfg),
		},
		"gridPos": map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":      pf.IDGen.Next(),
		"options": map[string]interface{}{
			"calculate":  getBool(cfg, "calculate", false),
			"cellGap":    getInt(cfg, "cell_gap", 2),
			"cellValues": map[string]interface{}{"decimals": getInt(cfg, "decimals", 0)},
			"color": map[string]interface{}{
				"exponent": 0.5,
				"fill":     "dark-blue",
				"min":      0,
				"mode":     "scheme",
				"reverse":  false,
				"scale":    getString(cfg, "color_scale", "exponential"),
				"scheme":   scheme,
				"steps":    128,
			},
			"exemplars":    map[string]interface{}{"color": "rgba(153,204,255,0.7)"},
			"filterValues": map[string]interface{}{"le": 1e-9},
			"legend":       map[string]interface{}{"show": true},
			"rowsFrame":    map[string]interface{}{"layout": "auto"},
			"tooltip":      map[string]interface{}{"show": true, "yHistogram": false},
			"yAxis": map[string]interface{}{
				"axisPlacement": "left",
				"reverse":       false,
				"unit":          getString(cfg, "y_unit", "short"),
			},
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "heatmap",
	}
	pf.applyTransformations(panel, cfg)
	pf.applyRepeat(panel, cfg)
	return panel
}

// Histogram creates a histogram panel.
func (pf *PanelFactory) Histogram(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["histogram"][0], DefaultSizes["histogram"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	panel := map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color": map[string]interface{}{"mode": getString(cfg, "color_mode", "palette-classic-by-name")},
				"custom": map[string]interface{}{
					"fillOpacity":  getInt(cfg, "fill_opacity", 80),
					"gradientMode": "none",
					"hideFrom":     map[string]interface{}{"legend": false, "tooltip": false, "viz": false},
					"lineWidth":    1,
				},
				"mappings":   []interface{}{},
				"thresholds": map[string]interface{}{"mode": "absolute", "steps": pf.thresholds(cfg, "")},
				"unit":       getString(cfg, "unit", "short"),
			},
			"overrides": pf.overrides(cfg),
		},
		"gridPos": map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":      pf.IDGen.Next(),
		"options": map[string]interface{}{
			"bucketCount":  getInt(cfg, "bucket_count", 30),
			"combine":      getBool(cfg, "combine", false),
			"fillOpacity":  getInt(cfg, "fill_opacity", 80),
			"gradientMode": "none",
			"legend":       map[string]interface{}{"calcs": []interface{}{}, "displayMode": "list", "placement": "bottom", "showLegend": true},
			"tooltip":      map[string]interface{}{"mode": "multi", "sort": "desc"},
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "histogram",
	}
	pf.applyTransformations(panel, cfg)
	pf.applyRepeat(panel, cfg)
	return panel
}

// Table creates a table panel.
func (pf *PanelFactory) Table(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["table"][0], DefaultSizes["table"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)

	sortBy := []interface{}{}
	if s, ok := cfg["sort_by"].([]interface{}); ok {
		sortBy = s
	}

	panel := map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color": map[string]interface{}{"mode": "thresholds"},
				"custom": map[string]interface{}{
					"align":       "auto",
					"cellOptions": map[string]interface{}{"type": "auto"},
					"filterable":  getBool(cfg, "filterable", true),
					"inspect":     true,
				},
				"mappings":   pf.valueMappings(cfg),
				"thresholds": map[string]interface{}{"mode": "absolute", "steps": pf.thresholds(cfg, "")},
				"unit":       getString(cfg, "unit", "short"),
				"links":      pf.dataLinks(cfg),
			},
			"overrides": pf.overrides(cfg),
		},
		"gridPos": map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":      pf.IDGen.Next(),
		"options": map[string]interface{}{
			"cellHeight": "sm",
			"footer": map[string]interface{}{
				"countRows":        false,
				"enablePagination": getBool(cfg, "pagination", false),
				"fields":           "",
				"reducer":          []interface{}{"sum"},
				"show":             false,
			},
			"showHeader": true,
			"sortBy":     sortBy,
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "table",
	}
	pf.applyTransformations(panel, cfg)
	pf.applyRepeat(panel, cfg)
	return panel
}

// Piechart creates a pie chart panel.
func (pf *PanelFactory) Piechart(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["piechart"][0], DefaultSizes["piechart"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	panel := map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color":      map[string]interface{}{"mode": getString(cfg, "color_mode", "palette-classic-by-name")},
				"mappings":   pf.valueMappings(cfg),
				"thresholds": map[string]interface{}{"mode": "absolute", "steps": pf.thresholds(cfg, "")},
				"unit":       getString(cfg, "unit", "short"),
			},
			"overrides": pf.overrides(cfg),
		},
		"gridPos": map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":      pf.IDGen.Next(),
		"options": map[string]interface{}{
			"displayLabels": getStringSlice(cfg, "display_labels", []string{"percent"}),
			"legend": map[string]interface{}{
				"calcs":       getStringSlice(cfg, "legend_calcs", []string{}),
				"displayMode": getString(cfg, "legend_mode", "list"),
				"placement":   getString(cfg, "legend_placement", "right"),
				"showLegend":  true,
			},
			"pieType": getString(cfg, "pie_type", "donut"),
			"reduceOptions": map[string]interface{}{
				"calcs":  getStringSlice(cfg, "calcs", []string{"lastNotNull"}),
				"fields": "",
				"values": false,
			},
			"tooltip": map[string]interface{}{"mode": "multi", "sort": "desc"},
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "piechart",
	}
	pf.applyTransformations(panel, cfg)
	pf.applyRepeat(panel, cfg)
	return panel
}

// StateTimeline creates a state-timeline panel.
func (pf *PanelFactory) StateTimeline(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["state-timeline"][0], DefaultSizes["state-timeline"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	panel := map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color": map[string]interface{}{"mode": "thresholds"},
				"custom": map[string]interface{}{
					"fillOpacity": getInt(cfg, "fill_opacity", 70),
					"hideFrom":    map[string]interface{}{"legend": false, "tooltip": false, "viz": false},
					"lineWidth":   0,
				},
				"mappings":   pf.valueMappings(cfg),
				"thresholds": map[string]interface{}{"mode": "absolute", "steps": pf.thresholds(cfg, "")},
				"unit":       getString(cfg, "unit", "short"),
			},
			"overrides": pf.overrides(cfg),
		},
		"gridPos": map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":      pf.IDGen.Next(),
		"options": map[string]interface{}{
			"alignValue":  "center",
			"legend":      map[string]interface{}{"displayMode": "list", "placement": "bottom", "showLegend": true},
			"mergeValues": getBool(cfg, "merge_values", true),
			"rowHeight":   getFloat(cfg, "row_height", 0.9),
			"showValue":   getString(cfg, "show_value", "auto"),
			"tooltip":     map[string]interface{}{"mode": "multi", "sort": "desc"},
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "state-timeline",
	}
	pf.applyTransformations(panel, cfg)
	pf.applyRepeat(panel, cfg)
	return panel
}

// StatusHistory creates a status-history panel.
func (pf *PanelFactory) StatusHistory(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["status-history"][0], DefaultSizes["status-history"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	panel := map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color": map[string]interface{}{"mode": "thresholds"},
				"custom": map[string]interface{}{
					"fillOpacity": getInt(cfg, "fill_opacity", 70),
					"hideFrom":    map[string]interface{}{"legend": false, "tooltip": false, "viz": false},
					"lineWidth":   1,
				},
				"mappings":   pf.valueMappings(cfg),
				"thresholds": map[string]interface{}{"mode": "absolute", "steps": pf.thresholds(cfg, "")},
				"unit":       getString(cfg, "unit", "short"),
			},
			"overrides": pf.overrides(cfg),
		},
		"gridPos": map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":      pf.IDGen.Next(),
		"options": map[string]interface{}{
			"colWidth":  0.9,
			"legend":    map[string]interface{}{"displayMode": "list", "placement": "bottom", "showLegend": true},
			"rowHeight": getFloat(cfg, "row_height", 0.9),
			"showValue": getString(cfg, "show_value", "auto"),
			"tooltip":   map[string]interface{}{"mode": "multi", "sort": "desc"},
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "status-history",
	}
	pf.applyTransformations(panel, cfg)
	pf.applyRepeat(panel, cfg)
	return panel
}

// Text creates a text panel.
func (pf *PanelFactory) Text(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["text"][0], DefaultSizes["text"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	return map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"gridPos":     map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":          pf.IDGen.Next(),
		"options": map[string]interface{}{
			"code": map[string]interface{}{
				"language":        "plaintext",
				"showLineNumbers": false,
				"showMiniMap":     false,
			},
			"content": getString(cfg, "content", ""),
			"mode":    getString(cfg, "mode", "markdown"),
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "text",
	}
}

// Logs creates a logs panel.
func (pf *PanelFactory) Logs(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["logs"][0], DefaultSizes["logs"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	panel := map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"gridPos":     map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":          pf.IDGen.Next(),
		"options": map[string]interface{}{
			"dedupStrategy":      getString(cfg, "dedup", "none"),
			"enableLogDetails":   true,
			"prettifyLogMessage": getBool(cfg, "prettify", false),
			"showCommonLabels":   getBool(cfg, "show_common_labels", false),
			"showLabels":         getBool(cfg, "show_labels", false),
			"showTime":           getBool(cfg, "show_time", true),
			"sortOrder":          getString(cfg, "sort_order", "Descending"),
			"wrapLogMessage":     getBool(cfg, "wrap", true),
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "logs",
	}
	pf.applyTransformations(panel, cfg)
	pf.applyRepeat(panel, cfg)
	return panel
}

// Comparison creates a mixed-datasource comparison panel.
func (pf *PanelFactory) Comparison(cfg map[string]interface{}, x, y int) (map[string]interface{}, error) {
	dw, dh := DefaultSizes["comparison"][0], DefaultSizes["comparison"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)

	dsNames := getStringSliceAsStrings(cfg, "datasources")
	if len(dsNames) < 2 {
		return nil, fmt.Errorf("comparison panel requires at least 2 datasources")
	}

	metric := getString(cfg, "metric", "up")
	metricType := getString(cfg, "metric_type", "gauge")
	mixedDS := map[string]interface{}{"type": "datasource", "uid": "-- Mixed --"}

	var targets []interface{}
	for i, dsName := range dsNames {
		ds, err := pf.Config.GetDatasource(dsName)
		if err != nil {
			return nil, err
		}
		var expr string
		if metricType == "counter" {
			expr = fmt.Sprintf("rate(%s[5m])", metric)
		} else {
			expr = metric
		}
		legend := getString(cfg, "legend", fmt.Sprintf("%s: {{instance}}", dsName))
		if !contains(legend, dsName) {
			legend = fmt.Sprintf("%s: %s", dsName, legend)
		}
		targets = append(targets, map[string]interface{}{
			"datasource":   map[string]interface{}{"type": ds.Type, "uid": ds.UID},
			"editorMode":   "code",
			"expr":         pf.Config.ResolveRef(expr),
			"legendFormat": legend,
			"range":        true,
			"refId":        string(rune('A' + i)),
		})
	}

	panel := map[string]interface{}{
		"datasource":  mixedDS,
		"description": getString(cfg, "description", fmt.Sprintf("comparison: %s", metric)),
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color": map[string]interface{}{"mode": "palette-classic-by-name"},
				"custom": map[string]interface{}{
					"axisBorderShow":    false,
					"axisCenteredZero":  false,
					"axisColorMode":     "text",
					"axisLabel":         "",
					"axisPlacement":     "auto",
					"barAlignment":      0,
					"barWidthFactor":    0.6,
					"drawStyle":         "line",
					"fillOpacity":       8,
					"gradientMode":      "scheme",
					"hideFrom":          map[string]interface{}{"legend": false, "tooltip": false, "viz": false},
					"insertNulls":       false,
					"lineInterpolation": "smooth",
					"lineWidth":         1,
					"pointSize":         5,
					"scaleDistribution": map[string]interface{}{"type": "linear"},
					"showPoints":        "never",
					"spanNulls":         false,
					"stacking":          map[string]interface{}{"group": "A", "mode": "none"},
					"thresholdsStyle":   map[string]interface{}{"mode": "off"},
				},
				"mappings":   []interface{}{},
				"thresholds": map[string]interface{}{"mode": "absolute", "steps": []interface{}{map[string]interface{}{"color": "#73BF69", "value": nil}}},
				"unit":       getString(cfg, "unit", "short"),
			},
			"overrides": []interface{}{},
		},
		"gridPos": map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":      pf.IDGen.Next(),
		"options": map[string]interface{}{
			"legend":  map[string]interface{}{"calcs": []interface{}{}, "displayMode": "list", "placement": "bottom", "showLegend": true},
			"tooltip": map[string]interface{}{"mode": "multi", "sort": "desc"},
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"targets":       targets,
		"title":         getString(cfg, "title", fmt.Sprintf("%s comparison", metric)),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "timeseries",
	}
	pf.applyTransformations(panel, cfg)
	pf.applyRepeat(panel, cfg)
	return panel, nil
}

// Alertlist creates an alert list panel.
func (pf *PanelFactory) Alertlist(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["alertlist"][0], DefaultSizes["alertlist"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	return map[string]interface{}{
		"datasource":  map[string]interface{}{"type": "datasource", "uid": "-- Grafana --"},
		"description": getString(cfg, "description", ""),
		"gridPos":     map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":          pf.IDGen.Next(),
		"options": map[string]interface{}{
			"alertInstanceLabelFilter": getString(cfg, "label_filter", ""),
			"alertName":                getString(cfg, "alert_name", ""),
			"dashboardAlerts":          getBool(cfg, "dashboard_alerts", false),
			"groupBy":                  getStringSlice(cfg, "group_by", []string{}),
			"groupMode":                getString(cfg, "group_mode", "default"),
			"maxItems":                 getInt(cfg, "max_items", 20),
			"sortOrder":                getInt(cfg, "sort_order", 1),
			"stateFilter": map[string]interface{}{
				"error":   getBool(cfg, "show_error", true),
				"firing":  getBool(cfg, "show_firing", true),
				"noData":  getBool(cfg, "show_nodata", false),
				"normal":  getBool(cfg, "show_normal", false),
				"pending": getBool(cfg, "show_pending", true),
			},
			"viewMode": getString(cfg, "view_mode", "list"),
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"title":         getString(cfg, "title", "alerts"),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "alertlist",
	}
}

// Dashlist creates a dashboard list panel.
func (pf *PanelFactory) Dashlist(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["dashlist"][0], DefaultSizes["dashlist"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	return map[string]interface{}{
		"datasource":  map[string]interface{}{"type": "datasource", "uid": "-- Grafana --"},
		"description": getString(cfg, "description", ""),
		"gridPos":     map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":          pf.IDGen.Next(),
		"options": map[string]interface{}{
			"folderId":    getInt(cfg, "folder_id", 0),
			"headings":    getBool(cfg, "show_headings", true),
			"includeVars": getBool(cfg, "include_vars", false),
			"keepTime":    getBool(cfg, "keep_time", false),
			"limit":       getInt(cfg, "limit", 10),
			"maxItems":    getInt(cfg, "max_items", 10),
			"query":       getString(cfg, "query", ""),
			"recent":      getBool(cfg, "show_recent", true),
			"search":      getBool(cfg, "show_search", false),
			"starred":     getBool(cfg, "show_starred", true),
			"tags":        getStringSlice(cfg, "tags", []string{}),
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"title":         getString(cfg, "title", "dashboards"),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "dashlist",
	}
}

// Trend creates a trend panel (timeseries with sequential numeric x-axis).
func (pf *PanelFactory) Trend(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["trend"][0], DefaultSizes["trend"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	fill := getInt(cfg, "fill_opacity", 8)
	line := getInt(cfg, "line_width", 1)
	interpolation := getString(cfg, "line_interpolation", "smooth")

	opts := map[string]interface{}{
		"legend": map[string]interface{}{
			"calcs":       getStringSlice(cfg, "legend_calcs", []string{}),
			"displayMode": getString(cfg, "legend_mode", "list"),
			"placement":   getString(cfg, "legend_placement", "bottom"),
			"showLegend":  getBool(cfg, "show_legend", true),
		},
		"tooltip": map[string]interface{}{"mode": "multi", "sort": "desc"},
	}
	if xf := getString(cfg, "x_field", ""); xf != "" {
		opts["xField"] = xf
	}

	panel := map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color": map[string]interface{}{"mode": getString(cfg, "color_mode", "palette-classic-by-name")},
				"custom": map[string]interface{}{
					"axisBorderShow":    false,
					"axisCenteredZero":  false,
					"axisColorMode":     "text",
					"axisLabel":         getString(cfg, "axis_label", ""),
					"axisPlacement":     "auto",
					"drawStyle":         getString(cfg, "draw_style", "line"),
					"fillOpacity":       fill,
					"gradientMode":      "scheme",
					"hideFrom":          map[string]interface{}{"legend": false, "tooltip": false, "viz": false},
					"lineInterpolation": interpolation,
					"lineWidth":         line,
					"pointSize":         5,
					"scaleDistribution": map[string]interface{}{"type": "linear"},
					"showPoints":        "never",
					"spanNulls":         false,
					"thresholdsStyle":   map[string]interface{}{"mode": "off"},
				},
				"mappings":   pf.valueMappings(cfg),
				"thresholds": map[string]interface{}{"mode": "absolute", "steps": pf.thresholds(cfg, "")},
				"unit":       getString(cfg, "unit", "short"),
				"links":      pf.dataLinks(cfg),
			},
			"overrides": pf.overrides(cfg),
		},
		"gridPos":       map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":            pf.IDGen.Next(),
		"options":       opts,
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "trend",
	}
	pf.applyTransformations(panel, cfg)
	pf.applyRepeat(panel, cfg)
	return panel
}

// Candlestick creates an OHLC candlestick panel.
func (pf *PanelFactory) Candlestick(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["candlestick"][0], DefaultSizes["candlestick"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)

	fields := map[string]interface{}{}
	if v := getString(cfg, "open_field", ""); v != "" {
		fields["open"] = v
	}
	if v := getString(cfg, "high_field", ""); v != "" {
		fields["high"] = v
	}
	if v := getString(cfg, "low_field", ""); v != "" {
		fields["low"] = v
	}
	if v := getString(cfg, "close_field", ""); v != "" {
		fields["close"] = v
	}

	panel := map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color": map[string]interface{}{"mode": getString(cfg, "color_mode", "thresholds")},
				"custom": map[string]interface{}{
					"axisPlacement":   "auto",
					"drawStyle":       "default",
					"hideFrom":        map[string]interface{}{"legend": false, "tooltip": false, "viz": false},
					"thresholdsStyle": map[string]interface{}{"mode": "off"},
				},
				"mappings":   pf.valueMappings(cfg),
				"thresholds": map[string]interface{}{"mode": "absolute", "steps": pf.thresholds(cfg, "")},
				"unit":       getString(cfg, "unit", "short"),
			},
			"overrides": pf.overrides(cfg),
		},
		"gridPos": map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":      pf.IDGen.Next(),
		"options": map[string]interface{}{
			"includeAllFields": getBool(cfg, "include_all_fields", false),
			"mode":             getString(cfg, "mode", "candles"),
			"candleStyle":      getString(cfg, "candle_style", "candles"),
			"colorStrategy":    getString(cfg, "color_strategy", "open-close"),
			"fields":           fields,
			"colors": map[string]interface{}{
				"up":   getString(cfg, "up_color", "green"),
				"down": getString(cfg, "down_color", "red"),
				"flat": getString(cfg, "flat_color", "gray"),
			},
			"legend":  map[string]interface{}{"displayMode": "list", "placement": "bottom", "showLegend": true},
			"tooltip": map[string]interface{}{"mode": "multi", "sort": "desc"},
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "candlestick",
	}
	pf.applyTransformations(panel, cfg)
	pf.applyRepeat(panel, cfg)
	return panel
}

// News creates a news/RSS feed panel.
func (pf *PanelFactory) News(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["news"][0], DefaultSizes["news"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	return map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"gridPos":     map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":          pf.IDGen.Next(),
		"options": map[string]interface{}{
			"feedUrl":   getString(cfg, "feed_url", ""),
			"showImage": getBool(cfg, "show_image", true),
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "news",
	}
}

// XYChart creates an XY scatter/correlation chart panel.
func (pf *PanelFactory) XYChart(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["xychart"][0], DefaultSizes["xychart"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	fill := getInt(cfg, "fill_opacity", 8)
	line := getInt(cfg, "line_width", 1)
	pointSize := getInt(cfg, "point_size", 5)

	showMode := getString(cfg, "show", "points")
	drawStyle := "line"
	showPoints := "always"
	switch showMode {
	case "lines":
		drawStyle = "line"
		showPoints = "never"
	case "both":
		drawStyle = "line"
		showPoints = "always"
	default: // "points"
		drawStyle = "points"
		showPoints = "always"
	}

	dims := map[string]interface{}{}
	if xf := getString(cfg, "x_field", ""); xf != "" {
		dims["x"] = xf
	}
	if ex := getStringSlice(cfg, "exclude", nil); len(ex) > 0 {
		dims["exclude"] = ex
	}

	seriesMapping := getString(cfg, "series_mapping", "auto")

	panel := map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color": map[string]interface{}{"mode": getString(cfg, "color_mode", "palette-classic-by-name")},
				"custom": map[string]interface{}{
					"axisBorderShow":    false,
					"axisCenteredZero":  false,
					"axisColorMode":     "text",
					"axisLabel":         getString(cfg, "axis_label", ""),
					"axisPlacement":     "auto",
					"drawStyle":         drawStyle,
					"fillOpacity":       fill,
					"hideFrom":          map[string]interface{}{"legend": false, "tooltip": false, "viz": false},
					"lineWidth":         line,
					"pointSize":         pointSize,
					"scaleDistribution": map[string]interface{}{"type": "linear"},
					"showPoints":        showPoints,
				},
				"mappings":   pf.valueMappings(cfg),
				"thresholds": map[string]interface{}{"mode": "absolute", "steps": pf.thresholds(cfg, "")},
				"unit":       getString(cfg, "unit", "short"),
				"links":      pf.dataLinks(cfg),
			},
			"overrides": pf.overrides(cfg),
		},
		"gridPos": map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":      pf.IDGen.Next(),
		"options": map[string]interface{}{
			"dims":          dims,
			"seriesMapping": seriesMapping,
			"legend":        map[string]interface{}{"calcs": getStringSlice(cfg, "legend_calcs", []string{}), "displayMode": getString(cfg, "legend_mode", "list"), "placement": getString(cfg, "legend_placement", "bottom"), "showLegend": getBool(cfg, "show_legend", true)},
			"tooltip":       map[string]interface{}{"mode": "multi", "sort": "desc"},
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "xychart",
	}
	pf.applyTransformations(panel, cfg)
	pf.applyRepeat(panel, cfg)
	return panel
}

// Geomap creates a geographic map panel with marker layers.
func (pf *PanelFactory) Geomap(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["geomap"][0], DefaultSizes["geomap"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)

	viewCfg := map[string]interface{}{
		"id":      "zero",
		"lat":     getFloat(cfg, "lat", 0),
		"lon":     getFloat(cfg, "lon", 0),
		"zoom":    getInt(cfg, "zoom", 3),
		"padding": 0,
	}

	basemap := getString(cfg, "basemap", "default")

	panel := map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color":      map[string]interface{}{"mode": getString(cfg, "color_mode", "thresholds")},
				"mappings":   pf.valueMappings(cfg),
				"thresholds": map[string]interface{}{"mode": "absolute", "steps": pf.thresholds(cfg, "")},
				"unit":       getString(cfg, "unit", "short"),
			},
			"overrides": pf.overrides(cfg),
		},
		"gridPos": map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":      pf.IDGen.Next(),
		"options": map[string]interface{}{
			"basemap": map[string]interface{}{
				"name": basemap,
				"type": basemap,
			},
			"layers": []interface{}{
				map[string]interface{}{
					"type": "markers",
					"config": map[string]interface{}{
						"showLegend": true,
						"style": map[string]interface{}{
							"color":   map[string]interface{}{"field": getString(cfg, "color_field", ""), "fixed": "dark-green"},
							"opacity": 0.6,
							"size":    map[string]interface{}{"field": getString(cfg, "size_field", ""), "fixed": 5, "max": 15, "min": 2},
							"symbol":  map[string]interface{}{"fixed": "img/icons/marker/circle.svg", "mode": "fixed"},
						},
					},
					"location": map[string]interface{}{
						"mode":      getString(cfg, "location_mode", "coords"),
						"latitude":  getString(cfg, "lat_field", "latitude"),
						"longitude": getString(cfg, "lon_field", "longitude"),
					},
					"name": "markers",
				},
			},
			"tooltip": map[string]interface{}{"mode": "details"},
			"view":    viewCfg,
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "geomap",
	}
	pf.applyTransformations(panel, cfg)
	pf.applyRepeat(panel, cfg)
	return panel
}

// NodeGraph creates a node graph panel for topology/dependency visualization.
func (pf *PanelFactory) NodeGraph(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["nodeGraph"][0], DefaultSizes["nodeGraph"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)

	var targets []interface{}
	ds := pf.ds(cfg)

	// Node query
	if nq := getString(cfg, "node_query", ""); nq != "" {
		targets = append(targets, pf.target(nq, "", "nodes", ds))
	}
	// Edge query
	if eq := getString(cfg, "edge_query", ""); eq != "" {
		targets = append(targets, pf.target(eq, "", "edges", ds))
	}
	// Fall back to standard targets if no explicit node/edge queries
	if len(targets) == 0 {
		targets = pf.buildTargets(cfg, ds)
	}

	panel := map[string]interface{}{
		"datasource":  ds,
		"description": getString(cfg, "description", ""),
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color":      map[string]interface{}{"mode": getString(cfg, "color_mode", "thresholds")},
				"mappings":   pf.valueMappings(cfg),
				"thresholds": map[string]interface{}{"mode": "absolute", "steps": pf.thresholds(cfg, "")},
			},
			"overrides": pf.overrides(cfg),
		},
		"gridPos": map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":      pf.IDGen.Next(),
		"options": map[string]interface{}{
			"nodes": map[string]interface{}{
				"mainStatUnit":      getString(cfg, "main_stat_unit", ""),
				"secondaryStatUnit": getString(cfg, "secondary_stat_unit", ""),
			},
			"edges": map[string]interface{}{},
		},
		"pluginVersion": pf.Config.GetGenerator().GetPluginVersion(),
		"targets":       targets,
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "nodeGraph",
	}
	pf.applyTransformations(panel, cfg)
	pf.applyRepeat(panel, cfg)
	return panel
}
