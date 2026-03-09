package server

import (
	"net/http"

	"github.com/wcatz/dashboard-generator/internal/config"
)

func (s *Server) handlePalettes(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()
	resolvedThresholds := make(map[string][]config.ThresholdStep)
	for name := range cfg.Thresholds {
		resolvedThresholds[name] = cfg.GetThresholds(name)
	}
	s.renderPage(w, "palettes.html", map[string]interface{}{
		"Title":         "palettes",
		"Active":        "palettes",
		"ConfigPath":    s.ConfigPath(),
		"GrafanaURL":    s.GrafanaURL(),
		"Palettes":      cfg.Palettes,
		"ActivePalette": cfg.ActivePalette,
		"Thresholds":    resolvedThresholds,
	})
}

// ── Palette CRUD handlers ──

func (s *Server) handlePaletteColorSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	palette := r.FormValue("palette")
	color := r.FormValue("color")
	hex := r.FormValue("hex")
	if palette == "" || color == "" || hex == "" {
		s.renderPartial(w, "palette-result.html", map[string]interface{}{"Error": "missing palette, color, or hex"})
		return
	}

	editor := config.NewYAMLEditor(s.ConfigPath())
	if err := editor.SetPaletteColor(palette, color, hex); err != nil {
		s.renderPartial(w, "palette-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}
	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "palette-result.html", map[string]interface{}{"Error": "saved but reload failed: " + err.Error()})
		return
	}
	s.renderPaletteCards(w)
}

func (s *Server) handlePaletteColorDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	palette := r.FormValue("palette")
	color := r.FormValue("color")
	if palette == "" || color == "" {
		s.renderPartial(w, "palette-result.html", map[string]interface{}{"Error": "missing palette or color"})
		return
	}

	editor := config.NewYAMLEditor(s.ConfigPath())
	if err := editor.DeletePaletteColor(palette, color); err != nil {
		s.renderPartial(w, "palette-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}
	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "palette-result.html", map[string]interface{}{"Error": "saved but reload failed: " + err.Error()})
		return
	}
	s.renderPaletteCards(w)
}

func (s *Server) handlePaletteColorRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	palette := r.FormValue("palette")
	oldName := r.FormValue("color")
	newName := r.FormValue("new_name")
	if palette == "" || oldName == "" || newName == "" {
		s.renderPartial(w, "palette-result.html", map[string]interface{}{"Error": "missing palette, color, or new_name"})
		return
	}

	editor := config.NewYAMLEditor(s.ConfigPath())
	if err := editor.RenamePaletteColor(palette, oldName, newName); err != nil {
		s.renderPartial(w, "palette-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}
	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "palette-result.html", map[string]interface{}{"Error": "saved but reload failed: " + err.Error()})
		return
	}
	s.renderPaletteCards(w)
}

func (s *Server) handlePaletteCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		s.renderPartial(w, "palette-result.html", map[string]interface{}{"Error": "missing palette name"})
		return
	}

	editor := config.NewYAMLEditor(s.ConfigPath())
	if err := editor.AddPalette(name); err != nil {
		s.renderPartial(w, "palette-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}
	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "palette-result.html", map[string]interface{}{"Error": "saved but reload failed: " + err.Error()})
		return
	}
	s.renderPaletteCards(w)
}

func (s *Server) handlePaletteDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		s.renderPartial(w, "palette-result.html", map[string]interface{}{"Error": "missing palette name"})
		return
	}

	editor := config.NewYAMLEditor(s.ConfigPath())
	if err := editor.DeletePalette(name); err != nil {
		s.renderPartial(w, "palette-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}
	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "palette-result.html", map[string]interface{}{"Error": "saved but reload failed: " + err.Error()})
		return
	}
	s.renderPaletteCards(w)
}

func (s *Server) handlePaletteActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		s.renderPartial(w, "palette-result.html", map[string]interface{}{"Error": "missing palette name"})
		return
	}

	editor := config.NewYAMLEditor(s.ConfigPath())
	if err := editor.SetActivePalette(name); err != nil {
		s.renderPartial(w, "palette-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}
	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "palette-result.html", map[string]interface{}{"Error": "saved but reload failed: " + err.Error()})
		return
	}
	s.renderPaletteCards(w)
}

func (s *Server) renderPaletteCards(w http.ResponseWriter) {
	cfg := s.Config()
	resolvedThresholds := make(map[string][]config.ThresholdStep)
	for name := range cfg.Thresholds {
		resolvedThresholds[name] = cfg.GetThresholds(name)
	}
	s.renderPartial(w, "palette-result.html", map[string]interface{}{
		"Palettes":      cfg.Palettes,
		"ActivePalette": cfg.ActivePalette,
		"Thresholds":    resolvedThresholds,
	})
}
