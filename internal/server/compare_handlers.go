package server

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/wcatz/dashboard-generator/internal/config"
	"github.com/wcatz/dashboard-generator/internal/generator"
	"gopkg.in/yaml.v3"
)

func (s *Server) handleComparePage(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()

	var dsNames []string
	for name, ds := range cfg.Datasources {
		if ds.URL != "" {
			dsNames = append(dsNames, name)
		}
	}
	sort.Strings(dsNames)

	s.renderPage(w, "compare.html", map[string]interface{}{
		"Title":       "compare",
		"Active":      "compare",
		"ConfigPath":  s.ConfigPath(),
		"GrafanaURL":  s.GrafanaURL(),
		"Datasources": dsNames,
	})
}

// handleCompareGenerate creates a new comparison dashboard from selected datasources and metrics.
func (s *Server) handleCompareGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	dsList := r.Form["datasources"]
	metrics := r.Form["metrics"]
	name := strings.TrimSpace(r.FormValue("name"))
	title := strings.TrimSpace(r.FormValue("title"))
	tagsRaw := strings.TrimSpace(r.FormValue("tags"))

	if len(dsList) < 2 {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "select at least 2 datasources"})
		return
	}
	if len(metrics) == 0 {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "select at least one metric"})
		return
	}
	if name == "" {
		name = "comparison"
	}

	if title == "" {
		title = name
	}

	// Build tags
	var tags []string
	if tagsRaw != "" {
		for _, t := range strings.Split(tagsRaw, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}
	tags = append(tags, "comparison", "generated")

	// Get metadata from first datasource
	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)
	meta, err := disc.FetchMetadata(dsList[0])
	if err != nil {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "failed to fetch metadata: " + err.Error()})
		return
	}
	opts := generator.BuildSuggestOptions(cfg)

	metricInfos := make(map[string]generator.MetricInfo)
	for _, m := range metrics {
		info, ok := meta[m]
		if !ok {
			info = generator.MetricInfo{Type: "untyped"}
		}
		metricInfos[m] = info
	}

	sectionYAML, _ := generator.FormatComparisonSnippetYAML(metrics, metricInfos, dsList, opts)

	// Build UID
	uid := "gen-" + strings.ReplaceAll(name, " ", "-")

	// Build the full dashboard YAML using yaml.Marshal for safe value encoding
	dashDoc := map[string]interface{}{
		"uid":      uid,
		"title":    title,
		"filename": uid + ".json",
	}
	if len(tags) > 0 {
		dashDoc["tags"] = tags
	}
	docBytes, err := yaml.Marshal(dashDoc)
	if err != nil {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "failed to marshal dashboard: " + err.Error()})
		return
	}

	// Indent the marshaled YAML by 2 spaces to match editor expectations, then append sections
	var lines []string
	for _, line := range strings.Split(strings.TrimRight(string(docBytes), "\n"), "\n") {
		lines = append(lines, "  "+line)
	}
	lines = append(lines, "  sections:")
	lines = append(lines, sectionYAML)

	dashboardYAML := strings.Join(lines, "\n")

	editor := config.NewYAMLEditor(s.ConfigPath())
	if err := editor.AddDashboard(name, []byte(dashboardYAML)); err != nil {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "failed to add dashboard: " + err.Error()})
		return
	}

	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "config-status.html", map[string]interface{}{"Error": "dashboard added but reload failed: " + err.Error()})
		return
	}

	s.renderPartial(w, "config-status.html", map[string]interface{}{
		"Message": fmt.Sprintf("dashboard '%s' created with %d comparison panels — <a href='/preview?uid=%s' class='link link-primary'>preview</a>", name, len(metrics), uid),
	})
}
