package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/wcatz/dashboard-generator/internal/config"
	"github.com/wcatz/dashboard-generator/internal/generator"
)

// Page handlers

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	cfg := s.Config()
	dashboards, _ := cfg.GetDashboards("")
	order, _ := cfg.GetDashboardOrder("")

	type dashInfo struct {
		Title     string
		UID       string
		Filename  string
		Sections  []config.SectionConfig
		Variables []string
	}

	var dashList []dashInfo
	seen := make(map[string]bool)
	for _, name := range order {
		if seen[name] {
			continue
		}
		seen[name] = true
		db, ok := dashboards[name]
		if !ok {
			continue
		}
		filename := db.Filename
		if filename == "" {
			filename = name + ".json"
		}
		dashList = append(dashList, dashInfo{
			Title:     db.Title,
			UID:       db.UID,
			Filename:  filename,
			Sections:  db.Sections,
			Variables: db.Variables,
		})
	}

	s.renderPage(w, "index.html", map[string]interface{}{
		"Title":           "dashboards",
		"Active":          "index",
		"ConfigPath":      s.ConfigPath(),
		"Dashboards":      dashList,
		"DashboardCount":  len(dashboards),
		"DatasourceCount": len(cfg.Datasources),
		"VariableCount":   len(cfg.Variables),
		"ProfileCount":    len(cfg.Profiles),
	})
}

func (s *Server) handleDatasources(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()
	s.renderPage(w, "datasources.html", map[string]interface{}{
		"Title":       "datasources",
		"Active":      "datasources",
		"Datasources": cfg.Datasources,
	})
}

func (s *Server) handlePalettes(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()
	s.renderPage(w, "palettes.html", map[string]interface{}{
		"Title":         "palettes",
		"Active":        "palettes",
		"Palettes":      cfg.Palettes,
		"ActivePalette": cfg.ActivePalette,
		"Thresholds":    cfg.Thresholds,
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()
	hasDatasources := false
	for _, ds := range cfg.Datasources {
		if ds.URL != "" {
			hasDatasources = true
			break
		}
	}
	s.renderPage(w, "metrics.html", map[string]interface{}{
		"Title":          "metrics",
		"Active":         "metrics",
		"Datasources":    cfg.Datasources,
		"HasDatasources": hasDatasources,
		"Filter":         "",
	})
}

func (s *Server) handleEditor(w http.ResponseWriter, r *http.Request) {
	content, err := s.ReadConfigContent()
	if err != nil {
		content = fmt.Sprintf("# error reading config: %v", err)
	}
	s.renderPage(w, "editor.html", map[string]interface{}{
		"Title":      "editor",
		"Active":     "editor",
		"ConfigPath": s.ConfigPath(),
		"Content":    content,
	})
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()
	dashboards, _ := cfg.GetDashboards("")
	order, _ := cfg.GetDashboardOrder("")

	type dashOption struct {
		Title string
		UID   string
	}
	var opts []dashOption
	for _, name := range order {
		db, ok := dashboards[name]
		if !ok {
			continue
		}
		opts = append(opts, dashOption{Title: db.Title, UID: db.UID})
	}

	selectedUID := r.URL.Query().Get("uid")

	data := map[string]interface{}{
		"Title":       "preview",
		"Active":      "preview",
		"Dashboards":  opts,
		"SelectedUID": selectedUID,
		"JSON":        "",
	}

	// If a UID was requested via query param, generate the preview
	if selectedUID != "" {
		jsonStr, title, size, panels, err := s.generatePreview(selectedUID)
		if err != nil {
			data["JSON"] = ""
		} else {
			data["JSON"] = jsonStr
			data["PreviewTitle"] = title
			data["PreviewSize"] = size
			data["PreviewPanels"] = panels
		}
	}

	s.renderPage(w, "preview.html", data)
}

// API handlers

func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	cfg := s.Config()
	dashboardUID := r.URL.Query().Get("dashboard")

	gen := cfg.GetGenerator()
	outDir := gen.OutputDir
	if outDir == "" {
		outDir = "."
	}
	if !filepath.IsAbs(outDir) {
		configDir := filepath.Dir(s.cfgPath)
		absConfig, _ := filepath.Abs(configDir)
		outDir = filepath.Join(absConfig, outDir)
	}

	dashboards, err := cfg.GetDashboards("")
	if err != nil {
		s.renderPartial(w, "generate-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	order, _ := cfg.GetDashboardOrder("")

	// Filter to single dashboard if requested
	if dashboardUID != "" {
		filtered := make(map[string]DashboardConfig)
		for name, db := range dashboards {
			if db.UID == dashboardUID {
				filtered[name] = db
			}
		}
		dashboards = filtered
	}

	idGen := generator.NewIDGenerator()
	panelFactory := generator.NewPanelFactory(cfg, idGen)
	layoutEngine := generator.NewLayoutEngine()
	builder := generator.NewDashboardBuilder(cfg, panelFactory, layoutEngine)
	navLinks := builder.BuildNavigationLinks(dashboards, order)

	type genResult struct {
		Filename string
		Panels   int
		Size     int
	}
	var results []genResult

	for _, name := range order {
		dbCfg, ok := dashboards[name]
		if !ok {
			continue
		}
		dashboard, err := builder.Build(dbCfg, navLinks, nil)
		if err != nil {
			s.renderPartial(w, "generate-result.html", map[string]interface{}{
				"Error": fmt.Sprintf("building %s: %v", name, err),
			})
			return
		}

		filename := dbCfg.Filename
		if filename == "" {
			filename = name + ".json"
		}
		fpath := filepath.Join(outDir, filename)

		size, err := generator.WriteDashboard(dashboard, fpath, false)
		if err != nil {
			s.renderPartial(w, "generate-result.html", map[string]interface{}{
				"Error": fmt.Sprintf("writing %s: %v", filename, err),
			})
			return
		}

		panels, _ := dashboard["panels"].([]interface{})
		results = append(results, genResult{
			Filename: filename,
			Panels:   len(panels),
			Size:     size,
		})
	}

	s.renderPartial(w, "generate-result.html", map[string]interface{}{
		"Count":   len(results),
		"Results": results,
	})
}

// DashboardConfig is a type alias for use in handler scope.
type DashboardConfig = config.DashboardConfig

func (s *Server) handleDatasourceTest(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		s.renderPartial(w, "ds-test-result.html", map[string]interface{}{"Error": "no datasource name"})
		return
	}

	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)
	metrics, err := disc.FetchMetrics(name)
	if err != nil {
		s.renderPartial(w, "ds-test-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	s.renderPartial(w, "ds-test-result.html", map[string]interface{}{
		"MetricCount": len(metrics),
	})
}

func (s *Server) handleDatasourceURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	r.ParseForm()
	name := r.FormValue("name")
	url := r.FormValue("url")

	if name == "" || url == "" {
		s.renderPartial(w, "ds-url-result.html", map[string]interface{}{"Error": "name and URL required"})
		return
	}

	// Read current config, update the datasource URL, write back
	content, err := s.ReadConfigContent()
	if err != nil {
		s.renderPartial(w, "ds-url-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	// Simple approach: reload config, update in memory, but we need to persist
	// For now, just inform the user to add it to the YAML manually
	_ = content
	s.renderPartial(w, "ds-url-result.html", map[string]interface{}{
		"Error": "URL editing not yet implemented — edit the YAML config directly",
	})
}

func (s *Server) handleMetricsBrowse(w http.ResponseWriter, r *http.Request) {
	dsName := r.URL.Query().Get("datasource")
	filter := r.URL.Query().Get("filter")
	metricType := r.URL.Query().Get("type")

	if dsName == "" {
		s.renderPartial(w, "metrics-result.html", map[string]interface{}{"Error": "select a datasource"})
		return
	}

	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)

	metrics, err := disc.FetchMetrics(dsName)
	if err != nil {
		s.renderPartial(w, "metrics-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	// Apply glob filter
	if filter != "" {
		metrics = generator.FilterMetrics(metrics, []string{filter}, nil)
	}

	// Get metadata
	meta, _ := disc.FetchMetadata(dsName)

	type metricRow struct {
		Name string
		Type string
		Help string
	}
	var rows []metricRow
	names := make([]string, 0, len(metrics))
	for m := range metrics {
		names = append(names, m)
	}
	sort.Strings(names)

	for _, m := range names {
		info, ok := meta[m]
		mType := "untyped"
		help := ""
		if ok {
			mType = info.Type
			help = info.Help
		}
		if metricType != "" && mType != metricType {
			continue
		}
		rows = append(rows, metricRow{Name: m, Type: mType, Help: help})
	}

	s.renderPartial(w, "metrics-result.html", map[string]interface{}{
		"Metrics":    rows,
		"Total":      len(rows),
		"Datasource": dsName,
	})
}

func (s *Server) handleConfigReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": err.Error()})
		return
	}
	s.renderPartial(w, "config-status.html", map[string]interface{}{"Message": "config reloaded"})
}

func (s *Server) handleConfigSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	r.ParseForm()
	content := r.FormValue("content")
	if content == "" {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "empty content"})
		return
	}

	// Validate first
	if _, err := config.LoadFromBytes([]byte(content)); err != nil {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "invalid YAML: " + err.Error()})
		return
	}

	if err := s.WriteConfigContent(content); err != nil {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	// Reload after saving
	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "saved but reload failed: " + err.Error()})
		return
	}

	s.renderPartial(w, "config-status.html", map[string]interface{}{"Message": "config saved and reloaded"})
}

func (s *Server) handleConfigValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	r.ParseForm()
	content := r.FormValue("content")
	if content == "" {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "empty content"})
		return
	}

	cfg, err := config.LoadFromBytes([]byte(content))
	if err != nil {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	dashboards, _ := cfg.GetDashboards("")
	s.renderPartial(w, "config-status.html", map[string]interface{}{
		"Message": fmt.Sprintf("valid — %d datasources, %d dashboards, %d variables",
			len(cfg.Datasources), len(dashboards), len(cfg.Variables)),
	})
}

func (s *Server) handlePreviewAPI(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("uid")
	if uid == "" {
		s.renderPartial(w, "preview-result.html", map[string]interface{}{"Error": "select a dashboard"})
		return
	}

	jsonStr, title, size, panels, err := s.generatePreview(uid)
	if err != nil {
		s.renderPartial(w, "preview-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	s.renderPartial(w, "preview-result.html", map[string]interface{}{
		"Title":  title,
		"Size":   size,
		"Panels": panels,
		"JSON":   jsonStr,
	})
}

func (s *Server) generatePreview(uid string) (jsonStr string, title string, size int, panels int, err error) {
	cfg := s.Config()
	dashboards, err := cfg.GetDashboards("")
	if err != nil {
		return "", "", 0, 0, err
	}
	order, _ := cfg.GetDashboardOrder("")

	// Find dashboard by UID
	var dbCfg config.DashboardConfig
	var found bool
	for _, db := range dashboards {
		if db.UID == uid {
			dbCfg = db
			found = true
			break
		}
	}
	if !found {
		return "", "", 0, 0, fmt.Errorf("dashboard with uid '%s' not found", uid)
	}

	idGen := generator.NewIDGenerator()
	panelFactory := generator.NewPanelFactory(cfg, idGen)
	layoutEngine := generator.NewLayoutEngine()
	builder := generator.NewDashboardBuilder(cfg, panelFactory, layoutEngine)
	navLinks := builder.BuildNavigationLinks(dashboards, order)

	dashboard, err := builder.Build(dbCfg, navLinks, nil)
	if err != nil {
		return "", "", 0, 0, err
	}

	data, err := json.MarshalIndent(dashboard, "", "  ")
	if err != nil {
		return "", "", 0, 0, err
	}

	panelList, _ := dashboard["panels"].([]interface{})
	return string(data), dbCfg.Title, len(data), len(panelList), nil
}
