package server

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/wcatz/dashboard-generator/internal/generator"
)

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
