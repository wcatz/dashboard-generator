package server

import "net/http"

func (s *Server) registerRoutes() {
	// Static files
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(s.staticFS)))

	// Pages
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/datasources", s.handleDatasources)
	s.mux.HandleFunc("/variables", s.handleVariables)
	s.mux.HandleFunc("/palettes", s.handlePalettes)
	s.mux.HandleFunc("/references", s.handleReferences)
	s.mux.HandleFunc("/editor", s.handleEditor)
	s.mux.HandleFunc("/metrics", s.handleMetrics)
	s.mux.HandleFunc("/preview", s.handlePreview)
	s.mux.HandleFunc("/profiles", s.handleProfiles)
	s.mux.HandleFunc("/settings", s.handleSettings)

	// API endpoints (HTMX)
	s.mux.HandleFunc("/api/push", s.handlePush)
	s.mux.HandleFunc("/api/generate", s.handleGenerate)
	s.mux.HandleFunc("/api/datasource/test", s.handleDatasourceTest)
	s.mux.HandleFunc("/api/datasource/url", s.handleDatasourceURL)
	s.mux.HandleFunc("/api/datasource/add", s.handleDatasourceAdd)
	s.mux.HandleFunc("/api/datasource/delete", s.handleDatasourceDelete)
	s.mux.HandleFunc("/api/datasource/targets", s.handleDatasourceTargets)
	s.mux.HandleFunc("/api/datasource/targets/metrics", s.handleDatasourceTargetMetrics)
	s.mux.HandleFunc("/api/datasources/compare-all", s.handleDatasourcesCompareAll)
	s.mux.HandleFunc("/api/datasources/compare-labels", s.handleDatasourcesCompareLabels)
	s.mux.HandleFunc("/api/datasources/variable-snippet", s.handleVariableSnippet)
	s.mux.HandleFunc("/api/metrics/browse", s.handleMetricsBrowse)
	s.mux.HandleFunc("/api/metrics/jobs", s.handleMetricsJobs)
	s.mux.HandleFunc("/api/metrics/compare", s.handleMetricsCompare)
	s.mux.HandleFunc("/api/metrics/snippet", s.handleMetricsSnippet)
	s.mux.HandleFunc("/api/metrics/comparison-snippet", s.handleComparisonSnippet)
	s.mux.HandleFunc("/api/config/reload", s.handleConfigReload)
	s.mux.HandleFunc("/api/config/save", s.handleConfigSave)
	s.mux.HandleFunc("/api/preview", s.handlePreviewAPI)
}
