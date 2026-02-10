package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

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

	type panelBrief struct {
		Title string
		Type  string
	}

	type sectionInfo struct {
		Title     string
		Collapsed bool
		Repeat    string
		Panels    []panelBrief
	}

	type dashInfo struct {
		Title      string
		UID        string
		Filename   string
		Sections   []sectionInfo
		Variables  []string
		Tags       []string
		PanelCount int
		TypeCounts map[string]int
		Description string
	}

	var dashList []dashInfo
	totalPanels := 0
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

		panelCount := 0
		typeCounts := make(map[string]int)
		var sections []sectionInfo
		for _, sec := range db.Sections {
			var panels []panelBrief
			for _, p := range sec.Panels {
				pType, _ := p["type"].(string)
				pTitle, _ := p["title"].(string)
				if pType == "" {
					pType = "unknown"
				}
				panels = append(panels, panelBrief{Title: pTitle, Type: pType})
				typeCounts[pType]++
				panelCount++
			}
			sections = append(sections, sectionInfo{
				Title:     sec.Title,
				Collapsed: sec.Collapsed,
				Repeat:    sec.Repeat,
				Panels:    panels,
			})
		}
		totalPanels += panelCount

		dashList = append(dashList, dashInfo{
			Title:       db.Title,
			UID:         db.UID,
			Filename:    filename,
			Sections:    sections,
			Variables:   db.Variables,
			Tags:        db.Tags,
			PanelCount:  panelCount,
			TypeCounts:  typeCounts,
			Description: db.Description,
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
		"ConstantCount":   len(cfg.Constants),
		"SelectorCount":   len(cfg.Selectors),
		"PanelCount":      totalPanels,
		"GrafanaURL":      s.GrafanaURL(),
	})
}

func (s *Server) handleDatasources(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()
	dsWithURL := 0
	for _, ds := range cfg.Datasources {
		if ds.URL != "" {
			dsWithURL++
		}
	}
	s.renderPage(w, "datasources.html", map[string]interface{}{
		"Title":       "datasources",
		"Active":      "datasources",
		"ConfigPath":  s.ConfigPath(),
		"GrafanaURL":  s.GrafanaURL(),
		"Datasources": cfg.Datasources,
		"DsWithURL":   dsWithURL,
	})
}

func (s *Server) handlePalettes(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()
	s.renderPage(w, "palettes.html", map[string]interface{}{
		"Title":         "palettes",
		"Active":        "palettes",
		"ConfigPath":    s.ConfigPath(),
		"GrafanaURL":    s.GrafanaURL(),
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
		"ConfigPath":     s.ConfigPath(),
		"GrafanaURL":     s.GrafanaURL(),
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
		"GrafanaURL": s.GrafanaURL(),
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

	s.renderPage(w, "preview.html", map[string]interface{}{
		"Title":       "preview",
		"Active":      "preview",
		"ConfigPath":  s.ConfigPath(),
		"GrafanaURL":  s.GrafanaURL(),
		"Dashboards":  opts,
		"SelectedUID": selectedUID,
	})
}

func (s *Server) handleVariables(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()

	type varInfo struct {
		Name       string
		Type       string
		Datasource string
		Query      string
		Multi      bool
		IncludeAll bool
		Values     string
		DsType     string
		ChainsFrom []string
	}

	type varUsage struct {
		Name       string
		Dashboards []string
	}

	// Collect variables in sorted order
	names := make([]string, 0, len(cfg.Variables))
	for name := range cfg.Variables {
		names = append(names, name)
	}
	sort.Strings(names)

	var vars []varInfo
	for _, name := range names {
		v := cfg.Variables[name]
		vars = append(vars, varInfo{
			Name:       name,
			Type:       v.Type,
			Datasource: v.Datasource,
			Query:      v.Query,
			Multi:      v.Multi,
			IncludeAll: v.IncludeAll,
			Values:     v.Values,
			DsType:     v.DsType,
			ChainsFrom: v.ChainsFrom,
		})
	}

	// Build variable usage map (which dashboards use each variable)
	dashboards, _ := cfg.GetDashboards("")
	usageMap := make(map[string][]string)
	for dName, db := range dashboards {
		for _, vName := range db.Variables {
			usageMap[vName] = append(usageMap[vName], dName)
		}
	}
	var usedBy []varUsage
	for _, name := range names {
		if dashes, ok := usageMap[name]; ok {
			sort.Strings(dashes)
			usedBy = append(usedBy, varUsage{Name: name, Dashboards: dashes})
		}
	}

	s.renderPage(w, "variables.html", map[string]interface{}{
		"Title":      "variables",
		"Active":     "variables",
		"ConfigPath": s.ConfigPath(),
		"GrafanaURL": s.GrafanaURL(),
		"Variables":  vars,
		"UsedBy":     usedBy,
	})
}

func (s *Server) handleReferences(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()

	type refItem struct {
		Name  string
		Value string
		Usage string
	}

	// Constants
	constNames := make([]string, 0, len(cfg.Constants))
	for name := range cfg.Constants {
		constNames = append(constNames, name)
	}
	sort.Strings(constNames)
	var constants []refItem
	for _, name := range constNames {
		constants = append(constants, refItem{
			Name:  name,
			Value: cfg.Constants[name],
			Usage: "${" + name + "}",
		})
	}

	// Selectors
	selNames := make([]string, 0, len(cfg.Selectors))
	for name := range cfg.Selectors {
		selNames = append(selNames, name)
	}
	sort.Strings(selNames)
	var selectors []refItem
	for _, name := range selNames {
		selectors = append(selectors, refItem{
			Name:  name,
			Value: cfg.Selectors[name],
			Usage: "${" + name + "}",
		})
	}

	s.renderPage(w, "references.html", map[string]interface{}{
		"Title":      "references",
		"Active":     "references",
		"ConfigPath": s.ConfigPath(),
		"GrafanaURL": s.GrafanaURL(),
		"Constants":  constants,
		"Selectors":  selectors,
	})
}

func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()

	type profileInfo struct {
		Name       string
		Dashboards []string
	}

	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)

	var profiles []profileInfo
	for _, name := range names {
		profiles = append(profiles, profileInfo{
			Name:       name,
			Dashboards: cfg.Profiles[name].Dashboards,
		})
	}

	s.renderPage(w, "profiles.html", map[string]interface{}{
		"Title":      "profiles",
		"Active":     "profiles",
		"ConfigPath": s.ConfigPath(),
		"GrafanaURL": s.GrafanaURL(),
		"Profiles":   profiles,
	})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()
	gen := cfg.GetGenerator()
	disc := cfg.GetDiscovery()

	editable := true
	if gen.Editable != nil {
		editable = *gen.Editable
	}
	liveNow := false
	if gen.LiveNow != nil {
		liveNow = *gen.LiveNow
	}

	timeFrom := ""
	timeTo := ""
	if gen.TimeRange != nil {
		timeFrom = gen.TimeRange["from"]
		timeTo = gen.TimeRange["to"]
	}

	s.renderPage(w, "settings.html", map[string]interface{}{
		"Title":            "settings",
		"Active":           "settings",
		"ConfigPath":       s.ConfigPath(),
		"GrafanaURL":       s.GrafanaURL(),
		"SchemaVersion":    gen.SchemaVersion,
		"OutputDir":        gen.OutputDir,
		"Refresh":          gen.Refresh,
		"TimeFrom":         timeFrom,
		"TimeTo":           timeTo,
		"Editable":         editable,
		"GraphTooltip":     gen.GraphTooltip,
		"LiveNow":          liveNow,
		"Timezone":         gen.Timezone,
		"DiscoveryEnabled": disc.Enabled,
		"DiscoverySources": disc.Sources,
		"IncludePatterns":  disc.IncludePatterns,
		"ExcludePatterns":  disc.ExcludePatterns,
	})
}

// API handlers

func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	grafanaURL := s.GrafanaURL()
	if grafanaURL == "" {
		s.renderPartial(w, "push-result.html", map[string]interface{}{
			"Error": "no Grafana URL configured (set --grafana-url or GRAFANA_URL)",
		})
		return
	}

	cfg := s.Config()
	dashboardUID := r.URL.Query().Get("dashboard")

	dashboards, err := cfg.GetDashboards("")
	if err != nil {
		s.renderPartial(w, "push-result.html", map[string]interface{}{"Error": err.Error()})
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

	type pushResult struct {
		Title  string
		UID    string
		Status string
	}
	var results []pushResult
	var errors []string

	for _, name := range order {
		dbCfg, ok := dashboards[name]
		if !ok {
			continue
		}
		dashboard, err := builder.Build(dbCfg, navLinks, nil)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", name, err))
			continue
		}

		if err := generator.PushToGrafana(dashboard, grafanaURL, "", "", ""); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", dbCfg.Title, err))
			continue
		}

		results = append(results, pushResult{
			Title:  dbCfg.Title,
			UID:    dbCfg.UID,
			Status: "success",
		})
	}

	s.renderPartial(w, "push-result.html", map[string]interface{}{
		"Count":   len(results),
		"Results": results,
		"Errors":  errors,
	})
}

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
		if err := validateFilename(filename); err != nil {
			s.renderPartial(w, "generate-result.html", map[string]interface{}{
				"Error": fmt.Sprintf("invalid filename '%s': %v", filename, err),
			})
			return
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

// PanelInfo holds layout and detail info for a single panel, used by the visual preview.
type PanelInfo struct {
	ID          int
	Title       string
	Type        string
	X, Y, W, H int
	Section     string
	Datasource  string
	Unit        string
	Description string
	Queries     []QueryInfo
	Thresholds  []ThresholdStep
}

// QueryInfo holds a single target's expression and legend.
type QueryInfo struct {
	Expr       string
	Legend     string
	Datasource string
	RefID      string
}

// ThresholdStep holds a single threshold step for display.
type ThresholdStep struct {
	Color string
	Value string
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

	for _, rp := range rawPanels {
		p, ok := rp.(map[string]interface{})
		if !ok {
			continue
		}

		pType, _ := p["type"].(string)
		if pType == "row" {
			title, _ := p["title"].(string)
			currentSection = title
		}

		info := parsePanelJSON(p, currentSection)
		infos = append(infos, info)

		// Recurse into collapsed row panels that nest their children.
		if pType == "row" {
			if nested, ok := p["panels"].([]interface{}); ok {
				for _, nr := range nested {
					np, ok := nr.(map[string]interface{})
					if !ok {
						continue
					}
					infos = append(infos, parsePanelJSON(np, currentSection))
				}
			}
		}
	}
	return infos
}

// parsePanelJSON extracts PanelInfo from a single panel JSON map.
func parsePanelJSON(p map[string]interface{}, section string) PanelInfo {
	pType, _ := p["type"].(string)
	title, _ := p["title"].(string)
	id, _ := p["id"].(float64)
	desc, _ := p["description"].(string)

	info := PanelInfo{
		ID:          int(id),
		Title:       title,
		Type:        pType,
		Section:     section,
		Description: desc,
	}

	// Grid position
	if gp, ok := p["gridPos"].(map[string]interface{}); ok {
		x, _ := gp["x"].(float64)
		y, _ := gp["y"].(float64)
		w, _ := gp["w"].(float64)
		h, _ := gp["h"].(float64)
		info.X = int(x)
		info.Y = int(y)
		info.W = int(w)
		info.H = int(h)
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

// stringFromMap extracts a string from a map, returning "" if not found.
func stringFromMap(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

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
	dsURL := r.FormValue("url")

	if name == "" || dsURL == "" {
		s.renderPartial(w, "ds-url-result.html", map[string]interface{}{"Error": "name and URL required"})
		return
	}

	editor := config.NewYAMLEditor(s.cfgPath)
	if err := editor.UpdateDatasourceURL(name, dsURL); err != nil {
		s.renderPartial(w, "ds-url-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}
	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "ds-url-result.html", map[string]interface{}{"Error": "saved but reload failed: " + err.Error()})
		return
	}

	s.renderPartial(w, "ds-url-result.html", map[string]interface{}{"Name": name})
}

func (s *Server) handleMetricsBrowse(w http.ResponseWriter, r *http.Request) {
	dsName := r.URL.Query().Get("datasource")
	filter := r.URL.Query().Get("filter")
	metricType := r.URL.Query().Get("type")
	job := r.URL.Query().Get("job")

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

	// Filter by job label if specified
	if job != "" {
		jobMetrics, err := disc.FetchSeriesMetrics(dsName, "job", job)
		if err == nil && len(jobMetrics) > 0 {
			for m := range metrics {
				if !jobMetrics[m] {
					delete(metrics, m)
				}
			}
		}
	}

	// Apply glob filter
	if filter != "" {
		metrics = generator.FilterMetrics(metrics, []string{filter}, nil)
	}

	// Get metadata
	meta, _ := disc.FetchMetadata(dsName)

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
		"Job":        job,
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
		data := map[string]interface{}{"Error": "invalid YAML: " + err.Error()}
		// Extract line number from yaml.v3 errors (e.g. "yaml: line 42: ...")
		if m := regexp.MustCompile(`line (\d+)`).FindStringSubmatch(err.Error()); m != nil {
			data["ErrorLine"] = m[1]
		}
		s.renderPartial(w, "config-status.html", data)
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


func (s *Server) handlePreviewAPI(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("uid")
	if uid == "" {
		s.renderPartial(w, "preview-result.html", map[string]interface{}{"Error": "select a dashboard"})
		return
	}

	jsonStr, title, size, panels, panelInfos, err := s.generatePreview(uid)
	if err != nil {
		s.renderPartial(w, "preview-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	s.renderPartial(w, "preview-result.html", map[string]interface{}{
		"UID":        uid,
		"Title":      title,
		"Size":       size,
		"Panels":     panels,
		"JSON":       jsonStr,
		"PanelInfos": panelInfos,
	})
}

func (s *Server) handlePanelDetail(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("uid")
	id := r.URL.Query().Get("id")
	if uid == "" || id == "" {
		http.Error(w, "missing uid or id", 400)
		return
	}

	_, _, _, _, panelInfos, err := s.generatePreview(uid)
	if err != nil {
		s.renderPartial(w, "panel-detail.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	// Find the panel by ID
	var idNum int
	fmt.Sscanf(id, "%d", &idNum)
	var panel *PanelInfo
	for i := range panelInfos {
		if panelInfos[i].ID == idNum {
			panel = &panelInfos[i]
			break
		}
	}
	if panel == nil {
		s.renderPartial(w, "panel-detail.html", map[string]interface{}{"Error": "panel not found"})
		return
	}

	s.renderPartial(w, "panel-detail.html", map[string]interface{}{"Panel": panel, "UID": uid})
}

func (s *Server) generatePreview(uid string) (jsonStr string, title string, size int, panels int, panelInfos []PanelInfo, err error) {
	cfg := s.Config()
	dashboards, err := cfg.GetDashboards("")
	if err != nil {
		return "", "", 0, 0, nil, err
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
		return "", "", 0, 0, nil, fmt.Errorf("dashboard with uid '%s' not found", uid)
	}

	idGen := generator.NewIDGenerator()
	panelFactory := generator.NewPanelFactory(cfg, idGen)
	layoutEngine := generator.NewLayoutEngine()
	builder := generator.NewDashboardBuilder(cfg, panelFactory, layoutEngine)
	navLinks := builder.BuildNavigationLinks(dashboards, order)

	dashboard, err := builder.Build(dbCfg, navLinks, nil)
	if err != nil {
		return "", "", 0, 0, nil, err
	}

	data, err := json.MarshalIndent(dashboard, "", "  ")
	if err != nil {
		return "", "", 0, 0, nil, err
	}

	panelList, _ := dashboard["panels"].([]interface{})
	pInfos := extractPanelInfo(dashboard)
	return string(data), dbCfg.Title, len(data), len(panelList), pInfos, nil
}

// handleMetricsJobs returns job label values for a datasource (tab rendering).
func (s *Server) handleMetricsJobs(w http.ResponseWriter, r *http.Request) {
	dsName := r.URL.Query().Get("datasource")
	if dsName == "" {
		s.renderPartial(w, "job-tabs.html", map[string]interface{}{"Error": "select a datasource"})
		return
	}

	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)
	jobs, err := disc.FetchLabelValues(dsName, "job")
	if err != nil {
		s.renderPartial(w, "job-tabs.html", map[string]interface{}{"Error": err.Error()})
		return
	}
	sort.Strings(jobs)

	s.renderPartial(w, "job-tabs.html", map[string]interface{}{
		"Jobs":       jobs,
		"Datasource": dsName,
	})
}

// handleMetricsCompare compares metrics between two datasources.
func (s *Server) handleMetricsCompare(w http.ResponseWriter, r *http.Request) {
	dsA := r.URL.Query().Get("datasource")
	dsB := r.URL.Query().Get("datasource_b")
	filter := r.URL.Query().Get("filter")
	metricType := r.URL.Query().Get("type")

	if dsA == "" || dsB == "" {
		s.renderPartial(w, "compare-result.html", map[string]interface{}{"Error": "select two datasources"})
		return
	}
	if dsA == dsB {
		s.renderPartial(w, "compare-result.html", map[string]interface{}{"Error": "datasources must be different"})
		return
	}

	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)
	cats, err := disc.Categorize(dsA, dsB)
	if err != nil {
		s.renderPartial(w, "compare-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	// Apply glob filter
	if filter != "" {
		for _, cat := range []string{"shared", "only_a", "only_b"} {
			cats[cat] = filterMetricInfoMap(cats[cat], filter)
		}
	}

	// Apply type filter
	if metricType != "" {
		for _, cat := range []string{"shared", "only_a", "only_b"} {
			cats[cat] = filterByType(cats[cat], metricType)
		}
	}

	s.renderPartial(w, "compare-result.html", map[string]interface{}{
		"DatasourceA": dsA,
		"DatasourceB": dsB,
		"Shared":      metricInfoToSlice(cats["shared"]),
		"OnlyA":       metricInfoToSlice(cats["only_a"]),
		"OnlyB":       metricInfoToSlice(cats["only_b"]),
		"SharedCount": len(cats["shared"]),
		"OnlyACount":  len(cats["only_a"]),
		"OnlyBCount":  len(cats["only_b"]),
	})
}

// handleMetricsSnippet generates a YAML config snippet from selected metrics.
func (s *Server) handleMetricsSnippet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	r.ParseForm()
	dsName := r.FormValue("datasource")
	selected := r.Form["metrics"]

	if len(selected) == 0 {
		s.renderPartial(w, "snippet-result.html", map[string]interface{}{"Error": "select at least one metric"})
		return
	}

	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)
	meta, _ := disc.FetchMetadata(dsName)

	var lines []string
	lines = append(lines, "      - title: \"discovered metrics\"")
	lines = append(lines, "        panels:")
	for _, m := range selected {
		info, ok := meta[m]
		if !ok {
			info = generator.MetricInfo{Type: "untyped"}
		}
		panelType := generator.SuggestPanelType(info.Type)
		query := generator.SuggestQuery(m, info.Type)
		lines = append(lines, fmt.Sprintf("          - type: %s", panelType))
		lines = append(lines, fmt.Sprintf("            title: \"%s\"", m))
		lines = append(lines, fmt.Sprintf("            query: '%s'", query))
		if dsName != "" {
			lines = append(lines, fmt.Sprintf("            datasource: %s", dsName))
		}
	}

	snippet := strings.Join(lines, "\n")
	s.renderPartial(w, "snippet-result.html", map[string]interface{}{
		"Snippet": snippet,
		"Count":   len(selected),
	})
}

// handleComparisonSnippet generates a YAML snippet for comparison panels from selected shared metrics.
// Accepts either datasource_a+datasource_b (2 DS) or datasources[] (N DS).
func (s *Server) handleComparisonSnippet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	r.ParseForm()
	selected := r.Form["metrics"]

	if len(selected) == 0 {
		s.renderPartial(w, "snippet-result.html", map[string]interface{}{"Error": "select at least one metric"})
		return
	}

	// Support both datasources[] array and datasource_a/datasource_b pair
	dsList := r.Form["datasources"]
	if len(dsList) == 0 {
		dsA := r.FormValue("datasource_a")
		dsB := r.FormValue("datasource_b")
		if dsA != "" && dsB != "" {
			dsList = []string{dsA, dsB}
		}
	}
	if len(dsList) < 2 {
		s.renderPartial(w, "snippet-result.html", map[string]interface{}{"Error": "need at least 2 datasources"})
		return
	}

	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)
	// Fetch metadata from first datasource for type info
	meta, _ := disc.FetchMetadata(dsList[0])

	dsListStr := strings.Join(dsList, ", ")
	var lines []string
	lines = append(lines, "      - title: \"shared metrics comparison\"")
	lines = append(lines, "        panels:")
	for _, m := range selected {
		info, ok := meta[m]
		if !ok {
			info = generator.MetricInfo{Type: "untyped"}
		}
		lines = append(lines, "          - type: comparison")
		lines = append(lines, fmt.Sprintf("            title: \"%s\"", m))
		lines = append(lines, fmt.Sprintf("            metric: \"%s\"", m))
		lines = append(lines, fmt.Sprintf("            metric_type: \"%s\"", info.Type))
		lines = append(lines, fmt.Sprintf("            datasources: [%s]", dsListStr))
	}

	snippet := strings.Join(lines, "\n")
	s.renderPartial(w, "snippet-result.html", map[string]interface{}{
		"Snippet": snippet,
		"Count":   len(selected),
	})
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

func filterMetricInfoMap(m map[string]generator.MetricInfo, pattern string) map[string]generator.MetricInfo {
	keys := make(map[string]bool)
	for k := range m {
		keys[k] = true
	}
	filtered := generator.FilterMetrics(keys, []string{pattern}, nil)
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

type metricRow struct {
	Name string
	Type string
	Help string
}

type labelSummary struct {
	Name       string
	Values     []string
	Constant   bool // same value on all targets
	AllTargets bool // present on every target
}

type enrichedJob struct {
	generator.JobSummary
	Labels       []labelSummary
	LabelCount   int
	ConstCount   int
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

func (s *Server) handleVariableSnippet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	r.ParseForm()
	dsName := r.FormValue("datasource")
	selected := r.Form["labels"]

	if len(selected) == 0 {
		s.renderPartial(w, "snippet-result.html", map[string]interface{}{"Error": "select at least one label"})
		return
	}

	var lines []string
	lines = append(lines, "variables:")
	for _, label := range selected {
		lines = append(lines, fmt.Sprintf("  %s:", label))
		lines = append(lines, "    type: query")
		if dsName != "" {
			lines = append(lines, fmt.Sprintf("    datasource: %s", dsName))
		}
		lines = append(lines, fmt.Sprintf("    query: 'label_values(%s)'", label))
		lines = append(lines, "    multi: true")
		lines = append(lines, "    include_all: true")
		lines = append(lines, "    refresh: 2")
		lines = append(lines, "    sort: 1")
	}

	s.renderPartial(w, "snippet-result.html", map[string]interface{}{
		"Snippet": strings.Join(lines, "\n"),
		"Count":   len(selected),
	})
}

func (s *Server) handleDatasourcesCompareLabels(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()

	var dsNames []string
	for name, ds := range cfg.Datasources {
		if ds.URL != "" {
			dsNames = append(dsNames, name)
		}
	}
	sort.Strings(dsNames)

	if len(dsNames) < 2 {
		s.renderPartial(w, "ds-compare-labels.html", map[string]interface{}{
			"Error": "need at least 2 datasources with URLs configured",
		})
		return
	}

	disc := generator.NewMetricDiscovery(cfg)

	// Fetch labels for each datasource
	allLabels := make(map[string]map[string]bool)
	for _, ds := range dsNames {
		labels, err := disc.FetchLabels(ds)
		if err != nil {
			s.renderPartial(w, "ds-compare-labels.html", map[string]interface{}{
				"Error": fmt.Sprintf("fetching labels from %s: %v", ds, err),
			})
			return
		}
		labelSet := make(map[string]bool)
		for _, l := range labels {
			if l != "__name__" {
				labelSet[l] = true
			}
		}
		allLabels[ds] = labelSet
	}

	// Shared = intersection of all label sets
	var shared []string
	for label := range allLabels[dsNames[0]] {
		onAll := true
		for _, ds := range dsNames[1:] {
			if !allLabels[ds][label] {
				onAll = false
				break
			}
		}
		if onAll {
			shared = append(shared, label)
		}
	}
	sort.Strings(shared)

	// Exclusive = labels unique to each DS
	sharedSet := make(map[string]bool)
	for _, l := range shared {
		sharedSet[l] = true
	}
	exclusive := make(map[string][]string)
	for _, ds := range dsNames {
		var unique []string
		for label := range allLabels[ds] {
			if sharedSet[label] {
				continue
			}
			onOther := false
			for _, other := range dsNames {
				if other == ds {
					continue
				}
				if allLabels[other][label] {
					onOther = true
					break
				}
			}
			if !onOther {
				unique = append(unique, label)
			}
		}
		sort.Strings(unique)
		exclusive[ds] = unique
	}

	s.renderPartial(w, "ds-compare-labels.html", map[string]interface{}{
		"Datasources": dsNames,
		"Shared":      shared,
		"Exclusive":   exclusive,
		"SharedCount": len(shared),
	})
}

func (s *Server) handleDatasourcesCompareAll(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()

	var dsNames []string
	for name, ds := range cfg.Datasources {
		if ds.URL != "" {
			dsNames = append(dsNames, name)
		}
	}
	sort.Strings(dsNames)

	if len(dsNames) < 2 {
		s.renderPartial(w, "ds-compare-all.html", map[string]interface{}{
			"Error": "need at least 2 datasources with URLs configured",
		})
		return
	}

	disc := generator.NewMetricDiscovery(cfg)
	shared, exclusive, err := disc.CompareAll(dsNames)
	if err != nil {
		s.renderPartial(w, "ds-compare-all.html", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	exclusiveRows := make(map[string][]metricRow)
	for ds, metrics := range exclusive {
		exclusiveRows[ds] = metricInfoToSlice(metrics)
	}

	s.renderPartial(w, "ds-compare-all.html", map[string]interface{}{
		"Datasources": dsNames,
		"Shared":      metricInfoToSlice(shared),
		"Exclusive":   exclusiveRows,
		"SharedCount": len(shared),
	})
}

func (s *Server) handleDatasourceAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	r.ParseForm()
	name := r.FormValue("name")
	dsURL := r.FormValue("url")

	if name == "" {
		s.renderPartial(w, "ds-add-result.html", map[string]interface{}{"Error": "datasource name is required"})
		return
	}
	if dsURL == "" {
		s.renderPartial(w, "ds-add-result.html", map[string]interface{}{"Error": "URL is required"})
		return
	}

	// Sanitize name: lowercase, replace spaces with hyphens
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, " ", "-")

	// Generate UID from name (replace hyphens with underscores for Grafana compatibility)
	uid := strings.ReplaceAll(name, "-", "_")

	ds := config.DatasourceDef{
		Type: "prometheus",
		UID:  uid,
		URL:  dsURL,
	}

	editor := config.NewYAMLEditor(s.cfgPath)
	if err := editor.AddDatasource(name, ds); err != nil {
		s.renderPartial(w, "ds-add-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}
	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "ds-add-result.html", map[string]interface{}{"Error": "saved but reload failed: " + err.Error()})
		return
	}

	w.Header().Set("HX-Refresh", "true")
	s.renderPartial(w, "ds-add-result.html", map[string]interface{}{"Name": name})
}

func (s *Server) handleDatasourceDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	r.ParseForm()
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name required", 400)
		return
	}

	editor := config.NewYAMLEditor(s.cfgPath)
	if err := editor.DeleteDatasource(name); err != nil {
		s.renderPartial(w, "ds-add-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}
	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "ds-add-result.html", map[string]interface{}{"Error": "deleted but reload failed: " + err.Error()})
		return
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(200)
}

func (s *Server) handleDatasourceTargets(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		s.renderPartial(w, "ds-targets.html", map[string]interface{}{"Error": "no datasource name"})
		return
	}

	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)

	targets, err := disc.FetchTargets(name)
	if err != nil {
		s.renderPartial(w, "ds-targets.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	jobs := generator.GroupTargetsByJob(targets)

	// Enrich jobs with label analysis
	enriched := make([]enrichedJob, len(jobs))
	for i, job := range jobs {
		labels := buildJobLabels(job)
		constCount := 0
		for _, l := range labels {
			if l.Constant {
				constCount++
			}
		}
		enriched[i] = enrichedJob{
			JobSummary: job,
			Labels:     labels,
			LabelCount: len(labels),
			ConstCount: constCount,
		}
	}

	s.renderPartial(w, "ds-targets.html", map[string]interface{}{
		"Datasource":  name,
		"Jobs":        enriched,
		"TargetCount": len(targets),
	})
}

func (s *Server) handleDatasourceTargetMetrics(w http.ResponseWriter, r *http.Request) {
	dsName := r.URL.Query().Get("name")
	job := r.URL.Query().Get("job")
	if dsName == "" || job == "" {
		s.renderPartial(w, "ds-target-metrics.html", map[string]interface{}{"Error": "datasource and job required"})
		return
	}

	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)

	// Get metrics for this job
	allMetrics, err := disc.FetchMetrics(dsName)
	if err != nil {
		s.renderPartial(w, "ds-target-metrics.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	jobMetrics, err := disc.FetchSeriesMetrics(dsName, "job", job)
	if err != nil {
		s.renderPartial(w, "ds-target-metrics.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	// Intersect: only metrics that exist in both sets
	for m := range allMetrics {
		if !jobMetrics[m] {
			delete(allMetrics, m)
		}
	}

	meta, _ := disc.FetchMetadata(dsName)

	names := make([]string, 0, len(allMetrics))
	for m := range allMetrics {
		names = append(names, m)
	}
	sort.Strings(names)

	var rows []metricRow
	for _, m := range names {
		info, ok := meta[m]
		mType := "untyped"
		help := ""
		if ok {
			mType = info.Type
			help = info.Help
		}
		rows = append(rows, metricRow{Name: m, Type: mType, Help: help})
	}

	s.renderPartial(w, "ds-target-metrics.html", map[string]interface{}{
		"Datasource": dsName,
		"Job":        job,
		"Metrics":    rows,
		"Total":      len(rows),
	})
}

// validateFilename checks for path traversal in dashboard filenames.
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
