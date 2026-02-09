package server

import "net/http"

func (s *Server) registerRoutes() {
	// Static files
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(s.staticFS)))

	// Pages
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/datasources", s.handleDatasources)
	s.mux.HandleFunc("/palettes", s.handlePalettes)
	s.mux.HandleFunc("/metrics", s.handleMetrics)
	s.mux.HandleFunc("/editor", s.handleEditor)
	s.mux.HandleFunc("/preview", s.handlePreview)

	// API endpoints (HTMX)
	s.mux.HandleFunc("/api/generate", s.handleGenerate)
	s.mux.HandleFunc("/api/datasource/test", s.handleDatasourceTest)
	s.mux.HandleFunc("/api/datasource/url", s.handleDatasourceURL)
	s.mux.HandleFunc("/api/metrics/browse", s.handleMetricsBrowse)
	s.mux.HandleFunc("/api/config/reload", s.handleConfigReload)
	s.mux.HandleFunc("/api/config/save", s.handleConfigSave)
	s.mux.HandleFunc("/api/config/validate", s.handleConfigValidate)
	s.mux.HandleFunc("/api/preview", s.handlePreviewAPI)
}
