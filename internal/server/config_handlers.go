package server

import (
	"net/http"
	"regexp"

	"github.com/wcatz/dashboard-generator/internal/config"
)

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
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
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
