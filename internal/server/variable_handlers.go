package server

import (
	"fmt"
	"html"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/wcatz/dashboard-generator/internal/config"
)

func (s *Server) handleVariables(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()

	type varInfo struct {
		Name       string
		Type       string
		Datasource string
		Query      string
		Multi      bool
		IncludeAll bool
		Values     string
		DsType     string
		ChainsFrom []string
	}

	type varUsage struct {
		Name       string
		Dashboards []string
	}

	// Collect variables in sorted order
	names := make([]string, 0, len(cfg.Variables))
	for name := range cfg.Variables {
		names = append(names, name)
	}
	sort.Strings(names)

	var vars []varInfo
	for _, name := range names {
		v := cfg.Variables[name]
		vars = append(vars, varInfo{
			Name:       name,
			Type:       v.Type,
			Datasource: v.Datasource,
			Query:      v.Query,
			Multi:      v.Multi,
			IncludeAll: v.IncludeAll,
			Values:     v.Values,
			DsType:     v.DsType,
			ChainsFrom: v.ChainsFrom,
		})
	}

	// Build variable usage map (which dashboards use each variable)
	dashboards, _ := cfg.GetDashboards("")
	usageMap := make(map[string][]string)
	for dName, db := range dashboards {
		for _, vName := range db.Variables {
			usageMap[vName] = append(usageMap[vName], dName)
		}
	}
	var usedBy []varUsage
	for _, name := range names {
		if dashes, ok := usageMap[name]; ok {
			sort.Strings(dashes)
			usedBy = append(usedBy, varUsage{Name: name, Dashboards: dashes})
		}
	}

	// Collect datasource names for the add-variable form
	var dsNames []string
	for name := range cfg.Datasources {
		dsNames = append(dsNames, name)
	}
	sort.Strings(dsNames)

	s.renderPage(w, "variables.html", map[string]interface{}{
		"Title":           "variables",
		"Active":          "variables",
		"ConfigPath":      s.ConfigPath(),
		"ActiveConfig":    s.ActiveConfigName(),
		"ConfigDir":       s.ConfigDir(),
		"GrafanaURL":      s.GrafanaURL(),
		"Variables":       vars,
		"UsedBy":          usedBy,
		"DatasourceNames": dsNames,
	})
}

func (s *Server) handleVariableSnippet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	dsName := r.FormValue("datasource")
	selected := r.Form["labels"]

	if len(selected) == 0 {
		s.renderPartial(w, "snippet-result.html", map[string]interface{}{"Error": "select at least one label"})
		return
	}

	var lines []string
	lines = append(lines, "variables:")
	for _, label := range selected {
		lines = append(lines, fmt.Sprintf("  %s:", label))
		lines = append(lines, "    type: query")
		if dsName != "" {
			lines = append(lines, fmt.Sprintf("    datasource: %s", dsName))
		}
		lines = append(lines, fmt.Sprintf("    query: 'label_values(%s)'", label))
		lines = append(lines, "    multi: true")
		lines = append(lines, "    include_all: true")
		lines = append(lines, "    refresh: 2")
		lines = append(lines, "    sort: 1")
	}

	s.renderPartial(w, "snippet-result.html", map[string]interface{}{
		"Snippet": strings.Join(lines, "\n"),
		"Count":   len(selected),
	})
}

// handleVariableAdd adds a new variable to the config via YAMLEditor.
func (s *Server) handleVariableAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	varType := r.FormValue("type")
	dsName := r.FormValue("datasource")
	query := r.FormValue("query")
	values := r.FormValue("values")
	multi := r.FormValue("multi") == "true"
	includeAll := r.FormValue("include_all") == "true"
	regex := r.FormValue("regex")

	if name == "" {
		s.renderPartial(w, "variable-result.html", map[string]interface{}{"Error": "variable name required"})
		return
	}

	// Sanitize name (only alphanumeric and underscores)
	if matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, name); !matched {
		s.renderPartial(w, "variable-result.html", map[string]interface{}{"Error": "invalid variable name (use letters, numbers, underscores)"})
		return
	}

	v := config.VariableDef{
		Type:       varType,
		Datasource: dsName,
		Query:      query,
		Values:     values,
		Multi:      multi,
		IncludeAll: includeAll,
		Refresh:    2,
		Sort:       1,
		Regex:      regex,
	}

	editor := config.NewYAMLEditor(s.ConfigPath())
	if err := editor.AddVariable(name, v); err != nil {
		s.renderPartial(w, "variable-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "variable-result.html", map[string]interface{}{"Error": "saved but reload failed: " + err.Error()})
		return
	}

	s.renderPartial(w, "variable-result.html", map[string]interface{}{
		"Success": fmt.Sprintf("variable '%s' added", name),
		"Name":    name,
	})
}

// handleBulkVariableAdd adds multiple variables in one request with per-variable feedback.
func (s *Server) handleBulkVariableAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	// Parse parallel arrays from form data
	names := r.Form["name"]
	queries := r.Form["query"]
	datasources := r.Form["datasource"]
	multis := r.Form["multi"]
	includeAlls := r.Form["include_all"]

	if len(names) == 0 {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div class="text-error text-sm">no variables provided</div>`)
		return
	}

	type result struct {
		Name    string
		Success bool
		Error   string
	}

	nameRegex := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	editor := config.NewYAMLEditor(s.ConfigPath())
	var results []result
	anySuccess := false

	for i, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			results = append(results, result{Name: "(empty)", Error: "name required"})
			continue
		}
		if !nameRegex.MatchString(name) {
			results = append(results, result{Name: name, Error: "invalid name"})
			continue
		}

		query := ""
		if i < len(queries) {
			query = queries[i]
		}
		ds := ""
		if i < len(datasources) {
			ds = datasources[i]
		}
		multi := false
		if i < len(multis) {
			multi = multis[i] == "true"
		}
		includeAll := false
		if i < len(includeAlls) {
			includeAll = includeAlls[i] == "true"
		}

		v := config.VariableDef{
			Type:       "query",
			Datasource: ds,
			Query:      query,
			Multi:      multi,
			IncludeAll: includeAll,
			Refresh:    2,
			Sort:       1,
		}

		if err := editor.AddVariable(name, v); err != nil {
			results = append(results, result{Name: name, Error: err.Error()})
		} else {
			results = append(results, result{Name: name, Success: true})
			anySuccess = true
		}
	}

	if anySuccess {
		if err := s.ReloadConfig(); err != nil {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<div class="text-warning text-sm">variables saved but reload failed: %s</div>`, html.EscapeString(err.Error()))
			return
		}
	}

	s.renderPartial(w, "bulk-variable-result.html", map[string]interface{}{
		"Results": results,
	})
}

// handleVariableDelete removes a variable from the config.
func (s *Server) handleVariableDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		s.renderPartial(w, "variable-result.html", map[string]interface{}{"Error": "variable name required"})
		return
	}

	editor := config.NewYAMLEditor(s.ConfigPath())
	if err := editor.DeleteVariable(name); err != nil {
		s.renderPartial(w, "variable-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "variable-result.html", map[string]interface{}{"Error": "deleted but reload failed: " + err.Error()})
		return
	}

	// Get updated count
	cfg := s.Config()
	count := len(cfg.Variables)

	// Return empty main swap (deletes the element) + OOB swap for count
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<span class="text-xs text-base-content/50" id="var-count" hx-swap-oob="true">%d variables</span>`, count)
}
