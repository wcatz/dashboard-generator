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
	"row":            {24, 1},
	"comparison":     {12, 8},
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
		if resolved != nil && len(resolved) > 0 {
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
	return map[string]interface{}{
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
			"orientation":  "auto",
			"reduceOptions": map[string]interface{}{
				"calcs":  getStringSlice(cfg, "calcs", []string{"lastNotNull"}),
				"fields": "",
				"values": false,
			},
			"showPercentChange": false,
			"textMode":          getString(cfg, "text_mode", "value_and_name"),
			"wideLayout":        true,
		},
		"pluginVersion": "11.2.0",
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "stat",
	}
}

// Gauge creates a gauge panel.
func (pf *PanelFactory) Gauge(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["gauge"][0], DefaultSizes["gauge"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	return map[string]interface{}{
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
		"pluginVersion": "11.2.0",
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "gauge",
	}
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
	return map[string]interface{}{
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
		"pluginVersion": "11.2.0",
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "timeseries",
	}
}

// Bargauge creates a bar gauge panel.
func (pf *PanelFactory) Bargauge(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["bargauge"][0], DefaultSizes["bargauge"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	return map[string]interface{}{
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
			"displayMode":  getString(cfg, "display_mode", "gradient"),
			"maxVizHeight": 300,
			"minVizHeight": 16,
			"minVizWidth":  8,
			"namePlacement": "auto",
			"orientation":  getString(cfg, "orientation", "horizontal"),
			"reduceOptions": map[string]interface{}{
				"calcs":  getStringSlice(cfg, "calcs", []string{"lastNotNull"}),
				"fields": "",
				"values": false,
			},
			"showUnfilled": true,
			"sizing":       "auto",
			"valueMode":    "color",
		},
		"pluginVersion": "11.2.0",
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "bargauge",
	}
}

// Heatmap creates a heatmap panel.
func (pf *PanelFactory) Heatmap(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["heatmap"][0], DefaultSizes["heatmap"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	scheme := getString(cfg, "color_scheme", "Spectral")
	return map[string]interface{}{
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
		"pluginVersion": "11.2.0",
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "heatmap",
	}
}

// Histogram creates a histogram panel.
func (pf *PanelFactory) Histogram(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["histogram"][0], DefaultSizes["histogram"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	return map[string]interface{}{
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
		"pluginVersion": "11.2.0",
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "histogram",
	}
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
	transformations := []interface{}{}
	if t, ok := cfg["transformations"].([]interface{}); ok {
		transformations = t
	}

	return map[string]interface{}{
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
		"pluginVersion":   "11.2.0",
		"targets":         pf.buildTargets(cfg, nil),
		"title":           getString(cfg, "title", ""),
		"transformations": transformations,
		"transparent":     getBool(cfg, "transparent", true),
		"type":            "table",
	}
}

// Piechart creates a pie chart panel.
func (pf *PanelFactory) Piechart(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["piechart"][0], DefaultSizes["piechart"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	return map[string]interface{}{
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
		"pluginVersion": "11.2.0",
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "piechart",
	}
}

// StateTimeline creates a state-timeline panel.
func (pf *PanelFactory) StateTimeline(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["state-timeline"][0], DefaultSizes["state-timeline"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	return map[string]interface{}{
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
		"pluginVersion": "11.2.0",
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "state-timeline",
	}
}

// StatusHistory creates a status-history panel.
func (pf *PanelFactory) StatusHistory(cfg map[string]interface{}, x, y int) map[string]interface{} {
	dw, dh := DefaultSizes["status-history"][0], DefaultSizes["status-history"][1]
	w := getInt(cfg, "width", dw)
	h := getInt(cfg, "height", dh)
	return map[string]interface{}{
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
		"pluginVersion": "11.2.0",
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "status-history",
	}
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
		"pluginVersion": "11.2.0",
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
	return map[string]interface{}{
		"datasource":  pf.ds(cfg),
		"description": getString(cfg, "description", ""),
		"gridPos":     map[string]interface{}{"h": h, "w": w, "x": x, "y": y},
		"id":          pf.IDGen.Next(),
		"options": map[string]interface{}{
			"dedupStrategy":     getString(cfg, "dedup", "none"),
			"enableLogDetails":  true,
			"prettifyLogMessage": getBool(cfg, "prettify", false),
			"showCommonLabels":  getBool(cfg, "show_common_labels", false),
			"showLabels":        getBool(cfg, "show_labels", false),
			"showTime":          getBool(cfg, "show_time", true),
			"sortOrder":         getString(cfg, "sort_order", "Descending"),
			"wrapLogMessage":    getBool(cfg, "wrap", true),
		},
		"pluginVersion": "11.2.0",
		"targets":       pf.buildTargets(cfg, nil),
		"title":         getString(cfg, "title", ""),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "logs",
	}
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

	return map[string]interface{}{
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
		"pluginVersion": "11.2.0",
		"targets":       targets,
		"title":         getString(cfg, "title", fmt.Sprintf("%s comparison", metric)),
		"transparent":   getBool(cfg, "transparent", true),
		"type":          "timeseries",
	}, nil
}
