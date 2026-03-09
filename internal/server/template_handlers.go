package server

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/wcatz/dashboard-generator/internal/config"
)

// handleTemplatesPage renders the config templates page
func (s *Server) handleTemplatesPage(w http.ResponseWriter, r *http.Request) {
	templates := GetConfigTemplates()
	
	// Group templates by category
	categories := make(map[string][]ConfigTemplate)
	for _, t := range templates {
		categories[t.Category] = append(categories[t.Category], t)
	}
	
	data := map[string]interface{}{
		"Title":      "templates",
		"Active":     "templates",
		"Templates":  templates,
		"Categories": categories,
		"ConfigPath": s.ConfigPath(),
	}
	
	
	s.renderPage(w, "templates.html", data)
}

// handleTemplatePreview returns the content of a template
func (s *Server) handleTemplatePreview(w http.ResponseWriter, r *http.Request) {
	templateName := r.URL.Query().Get("name")
	if templateName == "" {
		http.Error(w, "template name required", http.StatusBadRequest)
		return
	}
	
	template, err := GetTemplateByName(templateName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	// Return as YAML with syntax highlighting
	data := map[string]interface{}{
		"Template": template,
	}
	
	
	s.renderPartial(w, "template-preview.html", data)
}

// handleTemplateCreate creates a new config file from a template
func (s *Server) handleTemplateCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	templateName := r.FormValue("template")
	outputPath := r.FormValue("path")
	overwrite := r.FormValue("overwrite") == "true"
	
	if templateName == "" {
		http.Error(w, "template name required", http.StatusBadRequest)
		return
	}
	
	if outputPath == "" {
		http.Error(w, "output path required", http.StatusBadRequest)
		return
	}

	// Reject path traversal attempts
	clean := filepath.Clean(outputPath)
	if strings.Contains(clean, "..") {
		http.Error(w, "invalid output path", http.StatusBadRequest)
		return
	}
	
	// Get template
	template, err := GetTemplateByName(templateName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	// Check if file exists
	if _, err := os.Stat(outputPath); err == nil && !overwrite {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusConflict)
		fmt.Fprintf(w, `<div class="alert alert-warning">
			<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current shrink-0 w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path></svg>
			<span>File already exists: %s. Use overwrite option to replace.</span>
		</div>`, html.EscapeString(outputPath))
		return
	}
	
	// Create directory if needed
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, fmt.Sprintf("failed to create directory: %v", err), http.StatusInternalServerError)
		return
	}
	
	// Write template content
	if err := os.WriteFile(outputPath, []byte(template.Content), 0644); err != nil {
		http.Error(w, fmt.Sprintf("failed to write file: %v", err), http.StatusInternalServerError)
		return
	}
	
	// Success response
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div class="alert alert-success">
		<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current shrink-0 w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
		<div>
			<div class="font-bold">Config created successfully!</div>
			<div class="text-sm">Saved to: %s</div>
			<div class="mt-2">
				<a href="/editor?config=%s" class="btn btn-sm btn-primary">Edit Config</a>
				<button onclick="location.reload()" class="btn btn-sm btn-ghost">Create Another</button>
			</div>
		</div>
	</div>`, html.EscapeString(outputPath), url.QueryEscape(outputPath))
}

// handleTemplateLoad loads a template as the active config
func (s *Server) handleTemplateLoad(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	templateName := r.FormValue("template")
	if templateName == "" {
		http.Error(w, "template name required", http.StatusBadRequest)
		return
	}
	
	template, err := GetTemplateByName(templateName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	// Write to current config path
	// Validate the template can be parsed BEFORE writing to disk,
	// to avoid corrupting the active config file on a bad template.
	if _, err := config.LoadFromBytes([]byte(template.Content)); err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w,
			`<div class="alert alert-error"><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current shrink-0 w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg><span>template validation failed: %s</span></div>`,
			html.EscapeString(err.Error()),
		)
		return
	}

	if err := os.WriteFile(s.ConfigPath(), []byte(template.Content), 0644); err != nil {
		http.Error(w, fmt.Sprintf("failed to write config: %v", err), http.StatusInternalServerError)
		return
	}
	
	// Reload config
	if err := s.ReloadConfig(); err != nil {
		http.Error(w, fmt.Sprintf("failed to reload config: %v", err), http.StatusInternalServerError)
		return
	}
	
	// Success response with redirect
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("HX-Redirect", "/")
	fmt.Fprintf(w, `<div class="alert alert-success">
		<span>Config loaded from template: %s. Redirecting...</span>
	</div>`, template.Name)
}
