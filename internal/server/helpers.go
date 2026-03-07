package server

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wcatz/dashboard-generator/internal/generator"
)

// toInt extracts an int from a value that may be int or float64.
func toInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}

// stringFromMap extracts a string from a map, returning "" if not found.
func stringFromMap(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

// extractPanelInfo parses the panels array from a generated dashboard JSON map.
// It recurses into collapsed row panels whose nested panels are stored in the
// row's "panels" field rather than at the top level.
func extractPanelInfo(dashboard map[string]interface{}) []PanelInfo {
	rawPanels, ok := dashboard["panels"].([]interface{})
	if !ok {
		return nil
	}

	var infos []PanelInfo
	currentSection := ""
	currentSectionY := 0

	for _, rp := range rawPanels {
		p, ok := rp.(map[string]interface{})
		if !ok {
			continue
		}

		pType, _ := p["type"].(string)
		if pType == "row" {
			title, _ := p["title"].(string)
			currentSection = title
			if gp, ok := p["gridPos"].(map[string]interface{}); ok {
				currentSectionY = toInt(gp["y"])
			}
		}

		info := parsePanelJSON(p, currentSection, currentSectionY)
		infos = append(infos, info)

		// Recurse into collapsed row panels that nest their children.
		if pType == "row" {
			if nested, ok := p["panels"].([]interface{}); ok {
				for _, nr := range nested {
					np, ok := nr.(map[string]interface{})
					if !ok {
						continue
					}
					infos = append(infos, parsePanelJSON(np, currentSection, currentSectionY))
				}
			}
		}
	}
	return infos
}

func parsePanelJSON(p map[string]interface{}, section string, sectionY int) PanelInfo {
	pType, _ := p["type"].(string)
	title, _ := p["title"].(string)
	desc, _ := p["description"].(string)

	info := PanelInfo{
		ID:          toInt(p["id"]),
		Title:       title,
		Type:        pType,
		Section:     section,
		SectionY:    sectionY,
		Description: desc,
	}

	// Grid position (values may be int or float64 depending on source)
	if gp, ok := p["gridPos"].(map[string]interface{}); ok {
		info.X = toInt(gp["x"])
		info.Y = toInt(gp["y"])
		info.W = toInt(gp["w"])
		info.H = toInt(gp["h"])
	}

	// Datasource
	if ds, ok := p["datasource"].(map[string]interface{}); ok {
		if uid, ok := ds["uid"].(string); ok {
			info.Datasource = uid
		}
	}

	// Unit and thresholds from fieldConfig.defaults
	if fc, ok := p["fieldConfig"].(map[string]interface{}); ok {
		if defaults, ok := fc["defaults"].(map[string]interface{}); ok {
			if unit, ok := defaults["unit"].(string); ok && unit != "none" {
				info.Unit = unit
			}
			if th, ok := defaults["thresholds"].(map[string]interface{}); ok {
				if steps, ok := th["steps"].([]interface{}); ok {
					for _, s := range steps {
						step, ok := s.(map[string]interface{})
						if !ok {
							continue
						}
						color, _ := step["color"].(string)
						val := "base"
						if v, ok := step["value"].(float64); ok {
							val = fmt.Sprintf("%g", v)
						}
						info.Thresholds = append(info.Thresholds, ThresholdStep{Color: color, Value: val})
					}
				}
			}
		}
	}

	// Rendering hints from options
	if opts, ok := p["options"].(map[string]interface{}); ok {
		info.ColorMode = stringFromMap(opts, "colorMode")
		info.GraphMode = stringFromMap(opts, "graphMode")
		info.TextMode = stringFromMap(opts, "textMode")
		if rs, ok := opts["reduceOptions"].(map[string]interface{}); ok {
			_ = rs // available for future use
		}
		// Piechart
		info.PieType = stringFromMap(opts, "pieType")
		// Text panel
		if content, ok := opts["content"].(string); ok {
			info.TextContent = content
		}
		// Tooltip draw style
		info.DrawStyle = stringFromMap(opts, "drawStyle")
	}
	// fieldConfig.defaults.custom for timeseries-specific options
	if fc, ok := p["fieldConfig"].(map[string]interface{}); ok {
		if defaults, ok := fc["defaults"].(map[string]interface{}); ok {
			if custom, ok := defaults["custom"].(map[string]interface{}); ok {
				if ds := stringFromMap(custom, "drawStyle"); ds != "" {
					info.DrawStyle = ds
				}
				if fo, ok := custom["fillOpacity"].(float64); ok {
					info.FillOpacity = int(fo)
				}
				if sm := stringFromMap(custom, "stacking"); sm != "" {
					// stacking is nested: {"mode": "normal", "group": "A"}
					if stacking, ok := custom["stacking"].(map[string]interface{}); ok {
						info.StackMode = stringFromMap(stacking, "mode")
					}
				}
			}
			if min, ok := defaults["min"].(float64); ok {
				info.GaugeMin = min
			}
			if max, ok := defaults["max"].(float64); ok {
				info.GaugeMax = max
			}
		}
	}

	// Targets (queries)
	if targets, ok := p["targets"].([]interface{}); ok {
		for _, t := range targets {
			target, ok := t.(map[string]interface{})
			if !ok {
				continue
			}
			q := QueryInfo{
				Expr:   stringFromMap(target, "expr"),
				Legend: stringFromMap(target, "legendFormat"),
				RefID:  stringFromMap(target, "refId"),
			}
			if ds, ok := target["datasource"].(map[string]interface{}); ok {
				if uid, ok := ds["uid"].(string); ok {
					q.Datasource = uid
				}
			}
			info.Queries = append(info.Queries, q)
		}
	}

	return info
}

// extractNavLinks parses the links array from a generated dashboard JSON map
// and converts Grafana URLs (/d/{uid}) to preview page URLs.
func extractNavLinks(dashboard map[string]interface{}) []NavLink {
	rawLinks, ok := dashboard["links"].([]interface{})
	if !ok {
		return nil
	}

	var links []NavLink
	for _, rl := range rawLinks {
		link, ok := rl.(map[string]interface{})
		if !ok {
			continue
		}
		title, _ := link["title"].(string)
		url, _ := link["url"].(string)
		icon, _ := link["icon"].(string)
		tooltip, _ := link["tooltip"].(string)

		// Extract UID from Grafana URL format /d/{uid}
		uid := ""
		if strings.HasPrefix(url, "/d/") {
			uid = strings.TrimPrefix(url, "/d/")
		}
		if uid == "" {
			continue
		}

		links = append(links, NavLink{
			Title:   title,
			UID:     uid,
			Icon:    icon,
			Tooltip: tooltip,
		})
	}
	return links
}

func lookupMetaInfo(name string, primary, fallback map[string]generator.MetricInfo) generator.MetricInfo {
	if info, ok := primary[name]; ok {
		return info
	}
	if info, ok := fallback[name]; ok {
		return info
	}
	return generator.MetricInfo{Type: "untyped"}
}

func filterMetricInfoMap(m map[string]generator.MetricInfo, patterns []string) map[string]generator.MetricInfo {
	keys := make(map[string]bool)
	for k := range m {
		keys[k] = true
	}
	filtered := generator.FilterMetrics(keys, patterns, nil)
	result := make(map[string]generator.MetricInfo)
	for k := range filtered {
		result[k] = m[k]
	}
	return result
}

func filterByType(m map[string]generator.MetricInfo, mtype string) map[string]generator.MetricInfo {
	result := make(map[string]generator.MetricInfo)
	for name, info := range m {
		if info.Type == mtype {
			result[name] = info
		}
	}
	return result
}

func buildJobLabels(job generator.JobSummary) []labelSummary {
	// Collect all label keys and their values across targets
	labelValues := make(map[string]map[string]bool) // label → set of values
	labelCount := make(map[string]int)               // label → targets that have it
	for _, t := range job.Targets {
		for k, v := range t.Labels {
			if k == "__name__" {
				continue
			}
			if labelValues[k] == nil {
				labelValues[k] = make(map[string]bool)
			}
			labelValues[k][v] = true
			labelCount[k]++
		}
	}

	names := make([]string, 0, len(labelValues))
	for k := range labelValues {
		names = append(names, k)
	}
	sort.Strings(names)

	result := make([]labelSummary, 0, len(names))
	for _, name := range names {
		vals := make([]string, 0, len(labelValues[name]))
		for v := range labelValues[name] {
			vals = append(vals, v)
		}
		sort.Strings(vals)
		allTargets := labelCount[name] == job.TargetCount
		result = append(result, labelSummary{
			Name:       name,
			Values:     vals,
			Constant:   allTargets && len(vals) == 1,
			AllTargets: allTargets,
		})
	}
	return result
}

func metricInfoToSlice(m map[string]generator.MetricInfo) []metricRow {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]metricRow, 0, len(names))
	for _, name := range names {
		info := m[name]
		result = append(result, metricRow{Name: name, Type: info.Type, Help: info.Help})
	}
	return result
}

func validateFilename(filename string) error {
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return fmt.Errorf("filename cannot contain path separators")
	}
	if filename == "." || filename == ".." || strings.HasPrefix(filename, "..") {
		return fmt.Errorf("invalid filename")
	}
	if strings.Contains(filename, "\x00") {
		return fmt.Errorf("filename cannot contain null bytes")
	}
	return nil
}
