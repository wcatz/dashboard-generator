package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

func (s *Server) handleImportPage(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()

	var dsNames []string
	for name := range cfg.Datasources {
		dsNames = append(dsNames, name)
	}
	sort.Strings(dsNames)

	// Build UID-to-name map for auto-mapping
	uidToName := make(map[string]string)
	for name, ds := range cfg.Datasources {
		uidToName[ds.UID] = name
	}

	dashboards, _ := cfg.GetDashboardOrder("")

	s.renderPage(w, "import.html", map[string]interface{}{
		"Title":       "import",
		"Active":      "import",
		"ConfigPath":  s.ConfigPath(),
		"GrafanaURL":  s.GrafanaURL(),
		"Datasources": dsNames,
		"Dashboards":  dashboards,
	})
}

// handleImportParse parses Grafana panel JSON and returns config YAML.
func (s *Server) handleImportParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	panelJSON := r.FormValue("panel_json")
	if panelJSON == "" {
		s.renderPartial(w, "import-result.html", map[string]interface{}{"Error": "paste panel JSON"})
		return
	}

	var panel map[string]interface{}
	if err := json.Unmarshal([]byte(panelJSON), &panel); err != nil {
		s.renderPartial(w, "import-result.html", map[string]interface{}{"Error": "invalid JSON: " + err.Error()})
		return
	}

	// Parse using existing infrastructure
	info := parsePanelJSON(panel, "", 0)
	if info.Type == "" {
		s.renderPartial(w, "import-result.html", map[string]interface{}{"Error": "no panel type found in JSON"})
		return
	}

	// Build datasource UID-to-name auto-mapping
	cfg := s.Config()
	uidToName := make(map[string]string)
	for name, ds := range cfg.Datasources {
		uidToName[ds.UID] = name
	}

	// Generate config YAML from parsed info
	yaml := reversePanelToYAML(info, uidToName)

	s.renderPartial(w, "import-result.html", map[string]interface{}{
		"Panel": info,
		"YAML":  yaml,
	})
}

// reversePanelToYAML converts a PanelInfo back to config YAML string.
func reversePanelToYAML(info PanelInfo, uidToName map[string]string) string {
	var lines []string

	lines = append(lines, fmt.Sprintf("- type: %s", info.Type))
	if info.Title != "" {
		lines = append(lines, fmt.Sprintf("  title: \"%s\"", info.Title))
	}

	// Queries
	if len(info.Queries) == 1 {
		q := info.Queries[0]
		if q.Expr != "" {
			lines = append(lines, fmt.Sprintf("  query: '%s'", q.Expr))
		}
		if q.Legend != "" {
			lines = append(lines, fmt.Sprintf("  legend: \"%s\"", q.Legend))
		}
	} else if len(info.Queries) > 1 {
		lines = append(lines, "  targets:")
		for _, q := range info.Queries {
			lines = append(lines, fmt.Sprintf("    - query: '%s'", q.Expr))
			if q.Legend != "" {
				lines = append(lines, fmt.Sprintf("      legend: \"%s\"", q.Legend))
			}
			ds := resolveDatasource(q.Datasource, uidToName)
			if ds != "" {
				lines = append(lines, fmt.Sprintf("      datasource: %s", ds))
			}
		}
	}

	// Datasource
	ds := resolveDatasource(info.Datasource, uidToName)
	if ds != "" {
		lines = append(lines, fmt.Sprintf("  datasource: %s", ds))
	}

	// Unit
	if info.Unit != "" && info.Unit != "none" {
		lines = append(lines, fmt.Sprintf("  unit: %s", info.Unit))
	}

	// Description
	if info.Description != "" {
		desc := strings.ReplaceAll(info.Description, "\"", "\\\"")
		lines = append(lines, fmt.Sprintf("  description: \"%s\"", desc))
	}

	// Size
	if info.W > 0 {
		lines = append(lines, fmt.Sprintf("  width: %d", info.W))
	}
	if info.H > 0 {
		lines = append(lines, fmt.Sprintf("  height: %d", info.H))
	}

	// Type-specific options
	if info.ColorMode != "" {
		lines = append(lines, fmt.Sprintf("  color_mode: %s", info.ColorMode))
	}
	if info.DrawStyle != "" {
		lines = append(lines, fmt.Sprintf("  draw_style: %s", info.DrawStyle))
	}
	if info.FillOpacity > 0 {
		lines = append(lines, fmt.Sprintf("  fill_opacity: %d", info.FillOpacity))
	}
	if info.StackMode != "" && info.StackMode != "none" {
		lines = append(lines, fmt.Sprintf("  stack: %s", info.StackMode))
	}
	if info.GraphMode != "" {
		lines = append(lines, fmt.Sprintf("  graph_mode: %s", info.GraphMode))
	}
	if info.TextMode != "" {
		lines = append(lines, fmt.Sprintf("  text_mode: %s", info.TextMode))
	}
	if info.PieType != "" {
		lines = append(lines, fmt.Sprintf("  pie_type: %s", info.PieType))
	}
	if info.GaugeMin != 0 {
		lines = append(lines, fmt.Sprintf("  min: %g", info.GaugeMin))
	}
	if info.GaugeMax != 0 {
		lines = append(lines, fmt.Sprintf("  max: %g", info.GaugeMax))
	}
	if info.TextContent != "" {
		lines = append(lines, fmt.Sprintf("  content: |\n    %s", strings.ReplaceAll(info.TextContent, "\n", "\n    ")))
	}

	return strings.Join(lines, "\n")
}

func resolveDatasource(uid string, uidToName map[string]string) string {
	if uid == "" {
		return ""
	}
	if name, ok := uidToName[uid]; ok {
		return name
	}
	return uid
}
