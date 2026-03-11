package server

import (
	"fmt"
	"html"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/wcatz/dashboard-generator/internal/config"
	"github.com/wcatz/dashboard-generator/internal/generator"
)

func (s *Server) handlePanelBuilderPage(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()

	// Pre-fill from query params if metric provided (linked from metric browser)
	metric := r.URL.Query().Get("metric")
	metricType := r.URL.Query().Get("type")
	metricHelp := r.URL.Query().Get("help")
	dsName := r.URL.Query().Get("datasource")

	var prefill map[string]string
	if metric != "" {
		opts := generator.BuildSuggestOptions(cfg)
		info := generator.MetricInfo{Type: metricType, Help: metricHelp}
		panel := generator.SuggestPanel(metric, info, opts)
		pw, ph := generator.SuggestSize(panel.Type, 1)
		prefill = map[string]string{
			"metric":      metric,
			"type":        panel.Type,
			"title":       panel.Title,
			"query":       panel.Query,
			"legend":      panel.Legend,
			"unit":        panel.Unit,
			"description": panel.Description,
			"thresholds":  panel.Thresholds,
			"width":       strconv.Itoa(pw),
			"height":      strconv.Itoa(ph),
			"datasource":  dsName,
		}
		for k, v := range panel.Extra {
			prefill[k] = v
		}
	}

	// Collect datasource names
	var dsNames []string
	for name := range cfg.Datasources {
		dsNames = append(dsNames, name)
	}
	sort.Strings(dsNames)

	// Collect dashboard list with section info
	order, _ := cfg.GetDashboardOrder("")
	dashboards, _ := cfg.GetDashboards("")

	type dashSection struct {
		Dashboard string
		Index     int
		Title     string
	}
	var sections []dashSection
	for _, dbName := range order {
		db, ok := dashboards[dbName]
		if !ok {
			continue
		}
		for i, sec := range db.Sections {
			sections = append(sections, dashSection{
				Dashboard: dbName,
				Index:     i,
				Title:     sec.Title,
			})
		}
	}

	// Collect threshold names
	var thresholdNames []string
	for name := range cfg.Thresholds {
		thresholdNames = append(thresholdNames, "$"+name)
	}
	sort.Strings(thresholdNames)

	s.renderPage(w, "panel-builder.html", map[string]interface{}{
		"Title":          "panel builder",
		"Active":         "panel-builder",
		"ConfigPath":     s.ConfigPath(),
		"ActiveConfig":   s.ActiveConfigName(),
		"ConfigDir":      s.ConfigDir(),
		"GrafanaURL":     s.GrafanaURL(),
		"Prefill":        prefill,
		"Datasources":    dsNames,
		"Dashboards":     order,
		"Sections":       sections,
		"ThresholdNames": thresholdNames,
	})
}

// handlePanelPreviewYAML generates live YAML preview from form fields.
func (s *Server) handlePanelPreviewYAML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	panelType := r.FormValue("type")
	title := r.FormValue("title")
	query := r.FormValue("query")
	dsName := r.FormValue("datasource")
	unit := r.FormValue("unit")
	legend := r.FormValue("legend")
	description := r.FormValue("description")
	thresholds := r.FormValue("thresholds")
	widthStr := r.FormValue("width")
	heightStr := r.FormValue("height")

	if panelType == "" {
		panelType = "timeseries"
	}
	if title == "" {
		title = "untitled"
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("- type: %s", panelType))
	lines = append(lines, fmt.Sprintf("  title: \"%s\"", title))
	if query != "" {
		lines = append(lines, fmt.Sprintf("  query: '%s'", query))
	}
	if dsName != "" {
		lines = append(lines, fmt.Sprintf("  datasource: %s", dsName))
	}
	if unit != "" && unit != "short" {
		lines = append(lines, fmt.Sprintf("  unit: %s", unit))
	}
	if legend != "" {
		lines = append(lines, fmt.Sprintf("  legend: \"%s\"", legend))
	}
	if description != "" {
		desc := strings.ReplaceAll(description, "\"", "\\\"")
		lines = append(lines, fmt.Sprintf("  description: \"%s\"", desc))
	}
	if thresholds != "" {
		lines = append(lines, fmt.Sprintf("  thresholds: %s", thresholds))
	}

	if w, err := strconv.Atoi(widthStr); err == nil && w > 0 {
		lines = append(lines, fmt.Sprintf("  width: %d", w))
	}
	if h, err := strconv.Atoi(heightStr); err == nil && h > 0 {
		lines = append(lines, fmt.Sprintf("  height: %d", h))
	}

	// Type-specific options
	for _, key := range []string{
		"color_mode", "graph_mode", "text_mode", "draw_style",
		"fill_opacity", "line_interpolation", "stack", "pie_type",
		"display_mode", "orientation", "color", "min", "max",
		"legend_mode", "legend_placement", "show_legend",
		"content",
	} {
		val := r.FormValue(key)
		if val != "" {
			lines = append(lines, fmt.Sprintf("  %s: %s", key, val))
		}
	}

	// Additional queries
	additionalQueries := r.Form["additional_query"]
	additionalLegends := r.Form["additional_legend"]
	additionalDS := r.Form["additional_datasource"]
	if len(additionalQueries) > 0 {
		lines = append(lines, "  targets:")
		// First target is the main query
		if query != "" {
			mainTarget := fmt.Sprintf("    - query: '%s'", query)
			if legend != "" {
				mainTarget += fmt.Sprintf("\n      legend: \"%s\"", legend)
			}
			lines = append(lines, mainTarget)
		}
		for i, aq := range additionalQueries {
			if strings.TrimSpace(aq) == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("    - query: '%s'", aq))
			if i < len(additionalLegends) && additionalLegends[i] != "" {
				lines = append(lines, fmt.Sprintf("      legend: \"%s\"", additionalLegends[i]))
			}
			if i < len(additionalDS) && additionalDS[i] != "" {
				lines = append(lines, fmt.Sprintf("      datasource: %s", additionalDS[i]))
			}
		}
	}

	yamlStr := strings.Join(lines, "\n")
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<pre class="bg-base-200 border border-base-content/10 rounded-md p-4 font-mono text-xs leading-relaxed whitespace-pre overflow-auto max-h-[500px]"><code class="language-yaml" id="panel-yaml-output">%s</code></pre>`, html.EscapeString(yamlStr))
}

// handleInsertPanel inserts a panel into a specific section of a dashboard.
func (s *Server) handleInsertPanel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	dashboard := r.FormValue("dashboard")
	sectionIdxStr := r.FormValue("section_index")
	panelYAML := r.FormValue("panel_yaml")

	if dashboard == "" || sectionIdxStr == "" {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "select a dashboard and section"})
		return
	}
	if panelYAML == "" {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "no panel YAML"})
		return
	}

	sectionIdx, err := strconv.Atoi(sectionIdxStr)
	if err != nil {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "invalid section index"})
		return
	}

	editor := config.NewYAMLEditor(s.ConfigPath())
	if err := editor.AppendPanel(dashboard, sectionIdx, []byte(panelYAML)); err != nil {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "insert failed: " + err.Error()})
		return
	}

	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "inserted but reload failed: " + err.Error()})
		return
	}

	s.renderPartial(w, "config-status.html", map[string]interface{}{
		"Message": "panel inserted into '" + dashboard + "'",
	})
}
