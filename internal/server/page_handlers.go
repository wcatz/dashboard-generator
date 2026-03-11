package server

import (
	"fmt"
	"net/http"
	"sort"
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
		Title       string
		UID         string
		Filename    string
		Sections    []sectionInfo
		Variables   []string
		Tags        []string
		PanelCount  int
		TypeCounts  map[string]int
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
		"ActiveConfig":    s.ActiveConfigName(),
		"ConfigDir":       s.ConfigDir(),
		"Dashboards":      dashList,
		"DashboardCount":  len(dashboards),
		"DatasourceCount": len(cfg.Datasources),
		"VariableCount":   len(cfg.Variables),
		"ProfileCount":    len(cfg.Profiles),
		"ConstantCount":   len(cfg.Constants),
		"SelectorCount":   len(cfg.Selectors),
		"PanelCount":      totalPanels,
		"GrafanaURL":      s.GrafanaURL(),
		"Warnings":        cfg.Warnings,
	})
}

func (s *Server) handleEditor(w http.ResponseWriter, r *http.Request) {
	content, err := s.ReadConfigContent()
	if err != nil {
		content = fmt.Sprintf("# error reading config: %v", err)
	}
	cfg := s.Config()
	s.renderPage(w, "editor.html", map[string]interface{}{
		"Title":        "editor",
		"Active":       "editor",
		"ConfigPath":   s.ConfigPath(),
		"ActiveConfig": s.ActiveConfigName(),
		"ConfigDir":    s.ConfigDir(),
		"GrafanaURL":   s.GrafanaURL(),
		"Content":      content,
		"Warnings":     cfg.Warnings,
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
		"Title":        "preview",
		"Active":       "preview",
		"FullWidth":    true,
		"ConfigPath":   s.ConfigPath(),
		"ActiveConfig": s.ActiveConfigName(),
		"ConfigDir":    s.ConfigDir(),
		"GrafanaURL":   s.GrafanaURL(),
		"Dashboards":   opts,
		"SelectedUID":  selectedUID,
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
		"Title":        "references",
		"Active":       "references",
		"ConfigPath":   s.ConfigPath(),
		"ActiveConfig": s.ActiveConfigName(),
		"ConfigDir":    s.ConfigDir(),
		"GrafanaURL":   s.GrafanaURL(),
		"Constants":    constants,
		"Selectors":    selectors,
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
		"Title":        "profiles",
		"Active":       "profiles",
		"ConfigPath":   s.ConfigPath(),
		"ActiveConfig": s.ActiveConfigName(),
		"ConfigDir":    s.ConfigDir(),
		"GrafanaURL":   s.GrafanaURL(),
		"Profiles":     profiles,
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
		"ActiveConfig":     s.ActiveConfigName(),
		"ConfigDir":        s.ConfigDir(),
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
