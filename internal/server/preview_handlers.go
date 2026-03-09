package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	"github.com/wcatz/dashboard-generator/internal/config"
	"github.com/wcatz/dashboard-generator/internal/generator"
)

func (s *Server) handlePreviewAPI(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("uid")
	if uid == "" {
		s.renderPartial(w, "preview-result.html", map[string]interface{}{"Error": "select a dashboard"})
		return
	}

	cfg := s.Config()

	// "All dashboards" mode
	if uid == "all" {
		dashboards, err := cfg.GetDashboards("")
		if err != nil {
			s.renderPartial(w, "preview-result.html", map[string]interface{}{"Error": err.Error()})
			return
		}
		order, _ := cfg.GetDashboardOrder("")

		var allDashboards []PreviewDashboard
		var previewErrors []string
		totalSize := 0
		totalPanels := 0

		for _, name := range order {
			db, ok := dashboards[name]
			if !ok {
				continue
			}
			jsonStr, _, size, panels, panelInfos, _, err := s.generatePreview(db.UID)
			if err != nil {
				previewErrors = append(previewErrors, fmt.Sprintf("%s: %v", db.Title, err))
				continue
			}
			panelJSON, _ := json.Marshal(panelInfos)
			varInfos := s.buildVariableInfos(cfg, db.Variables)
			allDashboards = append(allDashboards, PreviewDashboard{
				UID:            db.UID,
				Title:          db.Title,
				Size:           size,
				Panels:         panels,
				JSON:           jsonStr,
				PanelInfos:     panelInfos,
				PanelInfosJSON: template.JS(panelJSON),
				Variables:      varInfos,
			})
			totalSize += size
			totalPanels += panels
		}

		s.renderPartial(w, "preview-result.html", map[string]interface{}{
			"UID":           "all",
			"Title":         "all dashboards",
			"Size":          totalSize,
			"Panels":        totalPanels,
			"AllDashboards": allDashboards,
			"IsAll":         true,
			"Errors":        previewErrors,
		})
		return
	}

	// Single dashboard mode
	jsonStr, title, size, panels, panelInfos, previewNavLinks, err := s.generatePreview(uid)
	if err != nil {
		s.renderPartial(w, "preview-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	// Look up variable definitions for this dashboard
	var dbCfg config.DashboardConfig
	dashboards, _ := cfg.GetDashboards("")
	for _, db := range dashboards {
		if db.UID == uid {
			dbCfg = db
			break
		}
	}
	varInfos := s.buildVariableInfos(cfg, dbCfg.Variables)

	// Serialize panel infos as JSON for client-side drawer rendering
	panelJSON, _ := json.Marshal(panelInfos)

	s.renderPartial(w, "preview-result.html", map[string]interface{}{
		"UID":            uid,
		"Title":          title,
		"Size":           size,
		"Panels":         panels,
		"JSON":           jsonStr,
		"PanelInfos":     panelInfos,
		"PanelInfosJSON": template.JS(panelJSON),
		"Variables":      varInfos,
		"NavLinks":       previewNavLinks,
		"GrafanaURL":     s.GrafanaURL(),
	})
}

// buildVariableInfos constructs VariableInfo slices from dashboard variable names.
func (s *Server) buildVariableInfos(cfg *config.Config, varNames []string) []VariableInfo {
	var infos []VariableInfo
	for _, name := range varNames {
		vDef, ok := cfg.GetVariableDef(name)
		if !ok {
			infos = append(infos, VariableInfo{Name: name, Type: "unknown"})
			continue
		}
		vi := VariableInfo{
			Name:       name,
			Label:      vDef.Label,
			Type:       vDef.Type,
			Multi:      vDef.Multi,
			IncludeAll: vDef.IncludeAll,
		}
		switch vDef.Type {
		case "query":
			vi.Query = vDef.Query
			vi.Datasource = vDef.Datasource
		case "custom", "interval":
			vi.Values = vDef.Values
		case "datasource":
			vi.Values = vDef.DsType
		}
		infos = append(infos, vi)
	}

	// Enrich with actual values from Prometheus/config
	return s.enrichVariablesWithValues(infos)
}

func (s *Server) generatePreview(uid string) (jsonStr string, title string, size int, panels int, panelInfos []PanelInfo, navLinks []NavLink, err error) {
	cfg := s.Config()
	dashboards, err := cfg.GetDashboards("")
	if err != nil {
		return "", "", 0, 0, nil, nil, err
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
		return "", "", 0, 0, nil, nil, fmt.Errorf("dashboard with uid '%s' not found", uid)
	}

	idGen := generator.NewIDGenerator()
	panelFactory := generator.NewPanelFactory(cfg, idGen)
	layoutEngine := generator.NewLayoutEngine()
	builder := generator.NewDashboardBuilder(cfg, panelFactory, layoutEngine)
	grafanaNavLinks := builder.BuildNavigationLinks(dashboards, order)

	dashboard, err := builder.Build(dbCfg, grafanaNavLinks, nil)
	if err != nil {
		return "", "", 0, 0, nil, nil, err
	}

	data, err := json.MarshalIndent(dashboard, "", "  ")
	if err != nil {
		return "", "", 0, 0, nil, nil, err
	}

	// Build UID-to-name map from config datasources
	uidToName := make(map[string]string)
	for name, ds := range cfg.Datasources {
		uidToName[ds.UID] = name
	}

	panelList, _ := dashboard["panels"].([]interface{})
	pInfos := extractPanelInfo(dashboard)

	// Resolve datasource UIDs to human-readable names
	for i := range pInfos {
		if name, ok := uidToName[pInfos[i].Datasource]; ok {
			pInfos[i].Datasource = name
		}
		for j := range pInfos[i].Queries {
			if name, ok := uidToName[pInfos[i].Queries[j].Datasource]; ok {
				pInfos[i].Queries[j].Datasource = name
			}
		}
	}

	// Extract navigation links from dashboard JSON
	parsedNavLinks := extractNavLinks(dashboard)

	return string(data), dbCfg.Title, len(data), len(panelList), pInfos, parsedNavLinks, nil
}

// handleLivePreview pushes a dashboard to Grafana as a temporary preview and returns the iframe URL.
func (s *Server) handleLivePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	grafanaURL := s.GrafanaURL()
	if grafanaURL == "" {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div class="alert alert-error text-sm">no Grafana URL configured</div>`)
		return
	}

	uid := r.URL.Query().Get("uid")
	if uid == "" {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div class="alert alert-error text-sm">no dashboard UID provided</div>`)
		return
	}

	// Generate the dashboard JSON
	cfg := s.Config()
	dashboards, err := cfg.GetDashboards("")
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<div class="alert alert-error text-sm">%s</div>`, err.Error())
		return
	}
	order, _ := cfg.GetDashboardOrder("")

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
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<div class="alert alert-error text-sm">dashboard '%s' not found</div>`, uid)
		return
	}

	idGen := generator.NewIDGenerator()
	panelFactory := generator.NewPanelFactory(cfg, idGen)
	layoutEngine := generator.NewLayoutEngine()
	builder := generator.NewDashboardBuilder(cfg, panelFactory, layoutEngine)
	grafanaNavLinks := builder.BuildNavigationLinks(dashboards, order)

	dashboard, err := builder.Build(dbCfg, grafanaNavLinks, nil)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<div class="alert alert-error text-sm">build failed: %s</div>`, err.Error())
		return
	}

	// Override UID for preview
	previewUID := uid + "-preview"
	dashboard["uid"] = previewUID

	// Push to Grafana
	if err := generator.PushToGrafana(dashboard, grafanaURL, "", "", s.GrafanaToken()); err != nil {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<div class="alert alert-error text-sm">push failed: %s</div>`, err.Error())
		return
	}

	// Return iframe HTML
	iframeURL := fmt.Sprintf("%s/d/%s?orgId=1&kiosk", grafanaURL, previewUID)
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div class="space-y-2">
  <div class="alert alert-success text-sm py-2">
    <span>pushed preview dashboard to Grafana</span>
  </div>
  <iframe src="%s" class="w-full rounded-lg border border-base-content/10" style="height: 80vh;" loading="lazy"></iframe>
  <div class="flex gap-2 text-xs">
    <a href="%s/d/%s" target="_blank" class="link link-primary">open in Grafana</a>
  </div>
</div>`, iframeURL, grafanaURL, previewUID)
}
