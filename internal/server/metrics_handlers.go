package server

import (
	"net/http"
	"sort"
	"strings"

	"github.com/wcatz/dashboard-generator/internal/generator"
)

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()
	hasDatasources := false
	for _, ds := range cfg.Datasources {
		if ds.URL != "" {
			hasDatasources = true
			break
		}
	}
	s.renderPage(w, "metrics.html", map[string]interface{}{
		"Title":          "metrics",
		"Active":         "metrics",
		"ConfigPath":     s.ConfigPath(),
		"GrafanaURL":     s.GrafanaURL(),
		"Datasources":    cfg.Datasources,
		"HasDatasources": hasDatasources,
		"Filter":         "",
		"AIEnabled":      generator.IsAIAvailable(cfg),
	})
}

func (s *Server) handleMetricsBrowse(w http.ResponseWriter, r *http.Request) {
	dsName := r.URL.Query().Get("datasource")
	filter := r.URL.Query().Get("filter")
	metricType := r.URL.Query().Get("type")
	job := r.URL.Query().Get("job")

	if dsName == "" {
		s.renderPartial(w, "metrics-result.html", map[string]interface{}{"Error": "select a datasource"})
		return
	}

	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)

	metrics, err := disc.FetchMetrics(dsName)
	if err != nil {
		s.renderPartial(w, "metrics-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	// Filter by job label if specified
	if job != "" {
		jobMetrics, err := disc.FetchSeriesMetrics(dsName, "job", job)
		if err == nil && len(jobMetrics) > 0 {
			for m := range metrics {
				if !jobMetrics[m] {
					delete(metrics, m)
				}
			}
		}
	}

	// Apply glob filter (supports comma-separated patterns)
	if filter != "" {
		patterns := strings.Split(filter, ",")
		for i := range patterns {
			patterns[i] = strings.TrimSpace(patterns[i])
		}
		metrics = generator.FilterMetrics(metrics, patterns, nil)
	}

	// Get metadata
	meta, _ := disc.FetchMetadata(dsName)

	var rows []metricRow
	names := make([]string, 0, len(metrics))
	for m := range metrics {
		names = append(names, m)
	}
	sort.Strings(names)

	for _, m := range names {
		info, ok := meta[m]
		mType := "untyped"
		help := ""
		if ok {
			mType = info.Type
			help = info.Help
		}
		if metricType != "" && mType != metricType {
			continue
		}
		rows = append(rows, metricRow{Name: m, Type: mType, Help: help})
	}

	s.renderPartial(w, "metrics-result.html", map[string]interface{}{
		"Metrics":    rows,
		"Total":      len(rows),
		"Datasource": dsName,
		"Job":        job,
		"AIEnabled":  generator.IsAIAvailable(cfg),
	})
}

// handleMetricsJobs returns job label values for a datasource (tab rendering).
func (s *Server) handleMetricsJobs(w http.ResponseWriter, r *http.Request) {
	dsName := r.URL.Query().Get("datasource")
	if dsName == "" {
		s.renderPartial(w, "job-tabs.html", map[string]interface{}{"Error": "select a datasource"})
		return
	}

	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)
	jobs, err := disc.FetchLabelValues(dsName, "job")
	if err != nil {
		s.renderPartial(w, "job-tabs.html", map[string]interface{}{"Error": err.Error()})
		return
	}
	sort.Strings(jobs)

	s.renderPartial(w, "job-tabs.html", map[string]interface{}{
		"Jobs":       jobs,
		"Datasource": dsName,
	})
}

// handleMetricsCompare compares metrics between two datasources.
func (s *Server) handleMetricsCompare(w http.ResponseWriter, r *http.Request) {
	dsA := r.URL.Query().Get("datasource")
	dsB := r.URL.Query().Get("datasource_b")
	filter := r.URL.Query().Get("filter")
	metricType := r.URL.Query().Get("type")

	if dsA == "" || dsB == "" {
		s.renderPartial(w, "compare-result.html", map[string]interface{}{"Error": "select two datasources"})
		return
	}
	if dsA == dsB {
		s.renderPartial(w, "compare-result.html", map[string]interface{}{"Error": "datasources must be different"})
		return
	}

	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)
	cats, err := disc.Categorize(dsA, dsB)
	if err != nil {
		s.renderPartial(w, "compare-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	// Apply glob filter (supports comma-separated patterns)
	if filter != "" {
		patterns := strings.Split(filter, ",")
		for i := range patterns {
			patterns[i] = strings.TrimSpace(patterns[i])
		}
		for _, cat := range []string{"shared", "only_a", "only_b"} {
			cats[cat] = filterMetricInfoMap(cats[cat], patterns)
		}
	}

	// Apply type filter
	if metricType != "" {
		for _, cat := range []string{"shared", "only_a", "only_b"} {
			cats[cat] = filterByType(cats[cat], metricType)
		}
	}

	s.renderPartial(w, "compare-result.html", map[string]interface{}{
		"DatasourceA": dsA,
		"DatasourceB": dsB,
		"Shared":      metricInfoToSlice(cats["shared"]),
		"OnlyA":       metricInfoToSlice(cats["only_a"]),
		"OnlyB":       metricInfoToSlice(cats["only_b"]),
		"SharedCount": len(cats["shared"]),
		"OnlyACount":  len(cats["only_a"]),
		"OnlyBCount":  len(cats["only_b"]),
	})
}

// handleMetricsSnippet generates a YAML config snippet from selected metrics.
func (s *Server) handleMetricsSnippet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	dsName := r.FormValue("datasource")
	selected := r.Form["metrics"]

	if len(selected) == 0 {
		s.renderPartial(w, "snippet-result.html", map[string]interface{}{"Error": "select at least one metric"})
		return
	}

	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)
	meta, _ := disc.FetchMetadata(dsName)
	opts := generator.BuildSuggestOptions(cfg)

	suggestions := make([]generator.PanelSuggestion, 0, len(selected))
	for _, m := range selected {
		info, ok := meta[m]
		if !ok {
			info = generator.MetricInfo{Type: "untyped"}
		}
		panel := generator.SuggestPanel(m, info, opts)
		pw, ph := generator.SuggestSize(panel.Type, len(selected))
		panel.Width = pw
		panel.Height = ph
		suggestions = append(suggestions, panel)
	}

	snippet, hints := generator.FormatSnippetYAML(suggestions, "discovered metrics", dsName)
	dashboards, _ := cfg.GetDashboardOrder("")
	s.renderPartial(w, "snippet-result.html", map[string]interface{}{
		"Snippet":    snippet,
		"Count":      len(selected),
		"Hints":      hints,
		"Dashboards": dashboards,
	})
}

// handleComparisonSnippet generates a YAML snippet for comparison panels from selected shared metrics.
// Accepts either datasource_a+datasource_b (2 DS) or datasources[] (N DS).
func (s *Server) handleComparisonSnippet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	selected := r.Form["metrics"]

	if len(selected) == 0 {
		s.renderPartial(w, "snippet-result.html", map[string]interface{}{"Error": "select at least one metric"})
		return
	}

	// Support both datasources[] array and datasource_a/datasource_b pair
	dsList := r.Form["datasources"]
	if len(dsList) == 0 {
		dsA := r.FormValue("datasource_a")
		dsB := r.FormValue("datasource_b")
		if dsA != "" && dsB != "" {
			dsList = []string{dsA, dsB}
		}
	}
	if len(dsList) < 2 {
		s.renderPartial(w, "snippet-result.html", map[string]interface{}{"Error": "need at least 2 datasources"})
		return
	}

	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)
	// Fetch metadata from first datasource for type info
	meta, _ := disc.FetchMetadata(dsList[0])
	opts := generator.BuildSuggestOptions(cfg)

	metricInfos := make(map[string]generator.MetricInfo)
	for _, m := range selected {
		info, ok := meta[m]
		if !ok {
			info = generator.MetricInfo{Type: "untyped"}
		}
		metricInfos[m] = info
	}

	snippet, hints := generator.FormatComparisonSnippetYAML(selected, metricInfos, dsList, opts)
	dashboards, _ := cfg.GetDashboardOrder("")
	s.renderPartial(w, "snippet-result.html", map[string]interface{}{
		"Snippet":    snippet,
		"Count":      len(selected),
		"Hints":      hints,
		"Dashboards": dashboards,
	})
}
