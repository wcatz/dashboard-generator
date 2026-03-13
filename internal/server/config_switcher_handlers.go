package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// validConfigName matches alphanumeric, hyphens, and underscores only.
var validConfigName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// handleConfigList returns the list of available configs (HTMX partial).
func (s *Server) handleConfigList(w http.ResponseWriter, r *http.Request) {
	configs, err := s.ListConfigs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.renderPartial(w, "config-list.html", map[string]interface{}{
		"Configs": configs,
		"Active":  filepath.Base(s.cfgPath),
	})
}

// handleConfigSwitch switches the active config.
func (s *Server) handleConfigSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}

	if err := s.SwitchConfig(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

// handleConfigNew creates a new config from a starter template or blank.
func (s *Server) handleConfigNew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.configDir == "" {
		http.Error(w, "config directory not configured", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	starter := strings.TrimSpace(r.FormValue("starter"))

	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	if !validConfigName.MatchString(name) {
		http.Error(w, "invalid name: use alphanumeric, hyphens, underscores only", http.StatusBadRequest)
		return
	}

	var content string
	if starter != "" {
		tmpl, err := GetTemplateByName(starter)
		if err != nil {
			http.Error(w, "starter template not found: "+err.Error(), http.StatusBadRequest)
			return
		}
		content = tmpl.Content
	} else {
		content = blankTemplate
	}

	// If user provided a datasource URL, replace the placeholder
	if dsURL := strings.TrimSpace(r.FormValue("datasource_url")); dsURL != "" {
		content = strings.Replace(content, "http://prometheus:9090", dsURL, 1)
	}

	target := filepath.Join(s.configDir, name+".yaml")
	if _, err := os.Stat(target); err == nil {
		http.Error(w, fmt.Sprintf("config '%s.yaml' already exists", name), http.StatusConflict)
		return
	}

	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		http.Error(w, "failed to write config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.SwitchConfig(name + ".yaml"); err != nil {
		http.Error(w, "created but failed to switch: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/datasources")
	w.WriteHeader(http.StatusOK)
}

// handleConfigDuplicate duplicates the current config under a new name.
func (s *Server) handleConfigDuplicate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.configDir == "" {
		http.Error(w, "config directory not configured", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	if !validConfigName.MatchString(name) {
		http.Error(w, "invalid name: use alphanumeric, hyphens, underscores only", http.StatusBadRequest)
		return
	}

	target := filepath.Join(s.configDir, name+".yaml")
	if _, err := os.Stat(target); err == nil {
		http.Error(w, fmt.Sprintf("config '%s.yaml' already exists", name), http.StatusConflict)
		return
	}

	data, err := os.ReadFile(s.cfgPath)
	if err != nil {
		http.Error(w, "failed to read current config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(target, data, 0644); err != nil {
		http.Error(w, "failed to write config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.SwitchConfig(name + ".yaml"); err != nil {
		http.Error(w, "duplicated but failed to switch: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

// handleConfigDelete deletes a config file from the config directory.
func (s *Server) handleConfigDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.configDir == "" {
		http.Error(w, "config directory not configured", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}

	// Refuse to delete the active config
	if name == filepath.Base(s.cfgPath) {
		http.Error(w, "cannot delete the active config", http.StatusBadRequest)
		return
	}

	// Validate path is within configDir
	clean := filepath.Base(name)
	if clean != name || strings.Contains(clean, "..") {
		http.Error(w, "invalid config name", http.StatusBadRequest)
		return
	}

	target := filepath.Join(s.configDir, clean)
	absTarget, err := filepath.Abs(target)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	absDir, err := filepath.Abs(s.configDir)
	if err != nil {
		http.Error(w, "invalid config dir", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(absTarget, absDir+string(filepath.Separator)) {
		http.Error(w, "invalid config name", http.StatusBadRequest)
		return
	}

	if err := os.Remove(target); err != nil {
		http.Error(w, "failed to delete: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated config list
	s.handleConfigList(w, r)
}
