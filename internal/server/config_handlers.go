package server

import (
	"log"
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

	// Backup before overwriting
	if _, err := s.BackupConfig(); err != nil {
		log.Printf("backup warning: %v", err)
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

func (s *Server) handleInsertSection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	dashboard := r.FormValue("dashboard")
	snippet := r.FormValue("yaml_snippet")

	if dashboard == "" {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "select a dashboard"})
		return
	}
	if snippet == "" {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "no YAML snippet provided"})
		return
	}

	// Validate the snippet parses as valid YAML
	sectionYAML := []byte(snippet)

	editor := config.NewYAMLEditor(s.ConfigPath())
	if err := editor.AppendSection(dashboard, sectionYAML); err != nil {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "insert failed: " + err.Error()})
		return
	}

	// Reload config to pick up the new section
	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "inserted but reload failed: " + err.Error()})
		return
	}

	s.renderPartial(w, "config-status.html", map[string]interface{}{
		"Message": "section inserted into '" + dashboard + "'",
	})
}
