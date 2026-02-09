package server

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/wcatz/dashboard-generator/internal/config"
)

var funcMap = template.FuncMap{
	"add": func(a, b int) int { return a + b },
}

// Server holds the HTTP server state and config.
type Server struct {
	cfg        *config.Config
	cfgPath    string
	grafanaURL string
	mu         sync.RWMutex
	webFS      fs.FS
	partials   *template.Template
	staticFS   http.FileSystem
	mux        *http.ServeMux
}

// New creates a new Server with the given embedded filesystem, config path, and optional Grafana URL.
func New(webFS fs.FS, cfgPath string, grafanaURL string) (*Server, error) {
	cfg, err := config.Load(cfgPath, nil)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	s := &Server{
		cfg:        cfg,
		cfgPath:    cfgPath,
		grafanaURL: grafanaURL,
		webFS:      webFS,
		mux:        http.NewServeMux(),
	}

	if err := s.loadTemplates(); err != nil {
		return nil, fmt.Errorf("loading templates: %w", err)
	}

	s.registerRoutes()
	return s, nil
}

func (s *Server) loadTemplates() error {
	// Parse partial templates (these are standalone fragments)
	partials, err := template.New("").Funcs(funcMap).ParseFS(s.webFS,
		"templates/partials/*.html",
	)
	if err != nil {
		return fmt.Errorf("parsing partial templates: %w", err)
	}
	s.partials = partials

	// Static file server
	staticSub, err := fs.Sub(s.webFS, "static")
	if err != nil {
		return fmt.Errorf("creating static FS: %w", err)
	}
	s.staticFS = http.FS(staticSub)

	return nil
}

// pageTemplate creates a fresh template set with layout + a specific page.
// This avoids the problem of multiple {{define "content"}} blocks conflicting.
func (s *Server) pageTemplate(page string) (*template.Template, error) {
	return template.New("").Funcs(funcMap).ParseFS(s.webFS,
		"templates/layout.html",
		"templates/"+page,
	)
}

// ReloadConfig reloads the YAML config from disk.
func (s *Server) ReloadConfig() error {
	cfg, err := config.Load(s.cfgPath, nil)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	return nil
}

// Config returns the current config (read-locked).
func (s *Server) Config() *config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// GrafanaURL returns the configured Grafana URL (empty if not set).
func (s *Server) GrafanaURL() string {
	return s.grafanaURL
}

// ConfigPath returns the absolute path to the config file.
func (s *Server) ConfigPath() string {
	abs, err := filepath.Abs(s.cfgPath)
	if err != nil {
		return s.cfgPath
	}
	return abs
}

// ReadConfigContent reads the raw config file content.
func (s *Server) ReadConfigContent() (string, error) {
	data, err := os.ReadFile(s.cfgPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteConfigContent writes content to the config file.
func (s *Server) WriteConfigContent(content string) error {
	return os.WriteFile(s.cfgPath, []byte(content), 0644)
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe(addr string) error {
	fmt.Printf("dashboard-generator web UI: http://localhost%s\n", addr)
	return http.ListenAndServe(addr, s)
}

// renderPage renders a full page template (layout + page).
func (s *Server) renderPage(w http.ResponseWriter, page string, data map[string]interface{}) {
	tmpl, err := s.pageTemplate(page)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		http.Error(w, "render error: "+err.Error(), 500)
	}
}

// renderPartial renders a partial template (HTMX response).
func (s *Server) renderPartial(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.partials.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "render error: "+err.Error(), 500)
	}
}
