package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/wcatz/dashboard-generator/internal/generator"
)

// parseOutputFormats converts a comma-separated format string to OutputFormat slice
func parseOutputFormats(formatStr string) []generator.OutputFormat {
	if formatStr == "" {
		formatStr = "json"
	}
	parts := strings.Split(formatStr, ",")
	formats := make([]generator.OutputFormat, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		switch p {
		case "json":
			formats = append(formats, generator.FormatJSON)
		case "grafana-yaml":
			formats = append(formats, generator.FormatGrafanaYAML)
		case "configmap":
			formats = append(formats, generator.FormatConfigMap)
		}
	}
	if len(formats) == 0 {
		formats = []generator.OutputFormat{generator.FormatJSON}
	}
	return formats
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

		if err := generator.PushToGrafana(dashboard, grafanaURL, "", "", s.GrafanaToken()); err != nil {
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
	formatStr := r.FormValue("output-format")
	if formatStr == "" {
		formatStr = "json" // default
	}
	formats := parseOutputFormats(formatStr)

	// Get K8s settings from config
	gen := cfg.GetGenerator()
	k8sNamespace := "monitoring"
	grafanaFolder := ""
	if gen.Kubernetes.Namespace != "" {
		k8sNamespace = gen.Kubernetes.Namespace
	}
	if gen.Kubernetes.GrafanaFolder != "" {
		grafanaFolder = gen.Kubernetes.GrafanaFolder
	}

	// Use gen.OutputDir from config
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
		Name    string
		Panels  int
		Size    int
		Formats []string
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

		// Determine base path (without extension)
		basePath := filepath.Join(outDir, name)
		if dbCfg.Filename != "" {
			filename := filepath.Base(dbCfg.Filename)
			if filename != "" && filename != "." {
				// Remove extension if present
				ext := filepath.Ext(filename)
				if ext != "" {
					filename = filename[:len(filename)-len(ext)]
				}
				basePath = filepath.Join(outDir, filename)
			}
		}

		// Determine namespace and folder for this dashboard
		namespace := k8sNamespace
		folder := grafanaFolder
		if dbCfg.K8sNamespace != "" {
			namespace = dbCfg.K8sNamespace
		}
		if dbCfg.GrafanaFolder != "" {
			folder = dbCfg.GrafanaFolder
		} else if folder == "" && len(dbCfg.Tags) > 0 {
			folder = dbCfg.Tags[0]
		}

		// Write dashboard in requested formats
		var size int
		if len(formats) == 1 && formats[0] == generator.FormatJSON {
			// Backward compat: single JSON output
			size, err = generator.WriteDashboard(dashboard, basePath+".json", false)
		} else {
			// Multi-format output
			err = generator.WriteDashboardMultiFormat(dashboard, basePath, formats, namespace, folder, false)
			size = 0 // size not tracked for multi-format
		}
		if err != nil {
			s.renderPartial(w, "generate-result.html", map[string]interface{}{
				"Error": fmt.Sprintf("writing %s: %v", name, err),
			})
			return
		}

		panels, _ := dashboard["panels"].([]interface{})
		// Build format list for display
		formatNames := make([]string, len(formats))
		for i, f := range formats {
			formatNames[i] = string(f)
		}
		results = append(results, genResult{
			Name:    name,
			Panels:  len(panels),
			Size:    size,
			Formats: formatNames,
		})
	}

	s.renderPartial(w, "generate-result.html", map[string]interface{}{
		"Count":   len(results),
		"Results": results,
	})
}
