package server

import (
	"bytes"
	"fmt"
	"github.com/wcatz/dashboard-generator/internal/config"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var funcMap = template.FuncMap{
	"add":   func(a, b int) int { return a + b },
	"upper": strings.ToUpper,
}

// Server holds the HTTP server state and config.
type Server struct {
	cfg           *config.Config
	cfgPath       string
	grafanaURL    string
	grafanaToken  string
	mu            sync.RWMutex
	webFS         fs.FS
	partials      *template.Template
	staticFS      http.FileSystem
	mux           *http.ServeMux
	variableCache *VariableCache
}

// New creates a new Server with the given embedded filesystem, config path, and optional Grafana URL/token.
func New(webFS fs.FS, cfgPath string, grafanaURL string, grafanaToken string) (*Server, error) {
	cfg, err := config.Load(cfgPath, nil)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	s := &Server{
		cfg:           cfg,
		cfgPath:       cfgPath,
		grafanaURL:    grafanaURL,
		grafanaToken:  grafanaToken,
		webFS:         webFS,
		mux:           http.NewServeMux(),
		variableCache: NewVariableCache(5 * time.Minute),
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

// BackupConfig creates a timestamped backup of the current config file.
// Keeps the last 5 backups and deletes older ones.
// Returns the backup file path on success.
func (s *Server) BackupConfig() (string, error) {
	data, err := os.ReadFile(s.cfgPath)
	if err != nil {
		return "", fmt.Errorf("reading config for backup: %w", err)
	}

	bakPath := fmt.Sprintf("%s.%s.bak", s.cfgPath, time.Now().Format("20060102-150405.000"))
	if err := os.WriteFile(bakPath, data, 0640); err != nil {
		return "", fmt.Errorf("writing backup: %w", err)
	}

	// Clean old backups, keep last 5
	s.pruneBackups(5)

	return bakPath, nil
}

// ListBackups returns available backup file paths, newest first.
func (s *Server) ListBackups() []string {
	pattern := s.cfgPath + ".*.bak"
	matches, _ := filepath.Glob(pattern)
	// Sort descending (newest first) — glob returns sorted ascending, so reverse
	for i, j := 0, len(matches)-1; i < j; i, j = i+1, j-1 {
		matches[i], matches[j] = matches[j], matches[i]
	}
	return matches
}

func (s *Server) pruneBackups(keep int) {
	backups := s.ListBackups()
	if len(backups) <= keep {
		return
	}
	for _, old := range backups[keep:] {
		os.Remove(old)
	}
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
	s.variableCache.Clear() // invalidate stale variable values after config change
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

// GrafanaToken returns the configured Grafana API token (empty if not set).
func (s *Server) GrafanaToken() string {
	return s.grafanaToken
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

// ServeHTTP implements http.Handler with security headers.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	csp := "default-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.tailwindcss.com; style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; connect-src 'self' https://cdn.jsdelivr.net"
	if s.grafanaURL != "" {
		csp += "; frame-src " + s.grafanaURL
	}
	w.Header().Set("Content-Security-Policy", csp)

	// CSRF protection: reject state-changing requests from foreign origins
	if r.Method == http.MethodPost {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = r.Header.Get("Referer")
		}
		host := r.Host
		if origin != "" && !strings.Contains(origin, host) {
			http.Error(w, "forbidden: cross-origin request", http.StatusForbidden)
			return
		}
	}

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
	var buf bytes.Buffer
	if err := s.partials.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("renderPartial %s error: %v", name, err)
		http.Error(w, "render error: "+err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}
