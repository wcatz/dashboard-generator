package server

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/wcatz/dashboard-generator/internal/config"
	"github.com/wcatz/dashboard-generator/internal/generator"
)

func (s *Server) handleDatasources(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()
	dsWithURL := 0
	for _, ds := range cfg.Datasources {
		if ds.URL != "" {
			dsWithURL++
		}
	}
	s.renderPage(w, "datasources.html", map[string]interface{}{
		"Title":        "datasources",
		"Active":       "datasources",
		"ConfigPath":   s.ConfigPath(),
		"ActiveConfig": s.ActiveConfigName(),
		"ConfigDir":    s.ConfigDir(),
		"GrafanaURL":   s.GrafanaURL(),
		"Datasources":  cfg.Datasources,
		"DsWithURL":    dsWithURL,
	})
}

func (s *Server) handleDatasourceTest(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		s.renderPartial(w, "ds-test-result.html", map[string]interface{}{"Error": "no datasource name"})
		return
	}

	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)
	metrics, err := disc.FetchMetrics(name)
	if err != nil {
		s.renderPartial(w, "ds-test-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	s.renderPartial(w, "ds-test-result.html", map[string]interface{}{
		"MetricCount": len(metrics),
	})
}

func (s *Server) handleDatasourceURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	name := r.FormValue("name")
	dsURL := r.FormValue("url")

	if name == "" || dsURL == "" {
		s.renderPartial(w, "ds-url-result.html", map[string]interface{}{"Error": "name and URL required"})
		return
	}

	editor := config.NewYAMLEditor(s.cfgPath)
	if err := editor.UpdateDatasourceURL(name, dsURL); err != nil {
		s.renderPartial(w, "ds-url-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}
	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "ds-url-result.html", map[string]interface{}{"Error": "saved but reload failed: " + err.Error()})
		return
	}

	s.renderPartial(w, "ds-url-result.html", map[string]interface{}{"Name": name})
}

func (s *Server) handleDatasourcesCompareLabels(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()

	var dsNames []string
	for name, ds := range cfg.Datasources {
		if ds.URL != "" {
			dsNames = append(dsNames, name)
		}
	}
	sort.Strings(dsNames)

	if len(dsNames) < 2 {
		s.renderPartial(w, "ds-compare-labels.html", map[string]interface{}{
			"Error": "need at least 2 datasources with URLs configured",
		})
		return
	}

	disc := generator.NewMetricDiscovery(cfg)

	// Fetch labels for each datasource
	allLabels := make(map[string]map[string]bool)
	for _, ds := range dsNames {
		labels, err := disc.FetchLabels(ds)
		if err != nil {
			s.renderPartial(w, "ds-compare-labels.html", map[string]interface{}{
				"Error": fmt.Sprintf("fetching labels from %s: %v", ds, err),
			})
			return
		}
		labelSet := make(map[string]bool)
		for _, l := range labels {
			if l != "__name__" {
				labelSet[l] = true
			}
		}
		allLabels[ds] = labelSet
	}

	// Shared = intersection of all label sets
	var shared []string
	for label := range allLabels[dsNames[0]] {
		onAll := true
		for _, ds := range dsNames[1:] {
			if !allLabels[ds][label] {
				onAll = false
				break
			}
		}
		if onAll {
			shared = append(shared, label)
		}
	}
	sort.Strings(shared)

	// Exclusive = labels unique to each DS
	sharedSet := make(map[string]bool)
	for _, l := range shared {
		sharedSet[l] = true
	}
	exclusive := make(map[string][]string)
	for _, ds := range dsNames {
		var unique []string
		for label := range allLabels[ds] {
			if sharedSet[label] {
				continue
			}
			onOther := false
			for _, other := range dsNames {
				if other == ds {
					continue
				}
				if allLabels[other][label] {
					onOther = true
					break
				}
			}
			if !onOther {
				unique = append(unique, label)
			}
		}
		sort.Strings(unique)
		exclusive[ds] = unique
	}

	s.renderPartial(w, "ds-compare-labels.html", map[string]interface{}{
		"Datasources": dsNames,
		"Shared":      shared,
		"Exclusive":   exclusive,
		"SharedCount": len(shared),
	})
}

func (s *Server) handleDatasourcesCompareAll(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()

	var dsNames []string
	for name, ds := range cfg.Datasources {
		if ds.URL != "" {
			dsNames = append(dsNames, name)
		}
	}
	sort.Strings(dsNames)

	if len(dsNames) < 2 {
		s.renderPartial(w, "ds-compare-all.html", map[string]interface{}{
			"Error": "need at least 2 datasources with URLs configured",
		})
		return
	}

	disc := generator.NewMetricDiscovery(cfg)
	shared, exclusive, err := disc.CompareAll(dsNames)
	if err != nil {
		s.renderPartial(w, "ds-compare-all.html", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	exclusiveRows := make(map[string][]metricRow)
	for ds, metrics := range exclusive {
		exclusiveRows[ds] = metricInfoToSlice(metrics)
	}

	s.renderPartial(w, "ds-compare-all.html", map[string]interface{}{
		"Datasources": dsNames,
		"Shared":      metricInfoToSlice(shared),
		"Exclusive":   exclusiveRows,
		"SharedCount": len(shared),
	})
}

func (s *Server) handleDatasourceAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	name := r.FormValue("name")
	dsURL := r.FormValue("url")

	if name == "" {
		s.renderPartial(w, "ds-add-result.html", map[string]interface{}{"Error": "datasource name is required"})
		return
	}
	if dsURL == "" {
		s.renderPartial(w, "ds-add-result.html", map[string]interface{}{"Error": "URL is required"})
		return
	}

	// Sanitize name: lowercase, replace spaces with hyphens
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, " ", "-")

	// Generate UID from name (replace hyphens with underscores for Grafana compatibility)
	uid := strings.ReplaceAll(name, "-", "_")

	ds := config.DatasourceDef{
		Type:      "prometheus",
		UID:       uid,
		URL:       dsURL,
		BasicUser: strings.TrimSpace(r.FormValue("basic_user")),
		BasicPass: strings.TrimSpace(r.FormValue("basic_pass")),
		Token:     strings.TrimSpace(r.FormValue("token")),
	}

	editor := config.NewYAMLEditor(s.cfgPath)
	if err := editor.AddDatasource(name, ds); err != nil {
		s.renderPartial(w, "ds-add-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}
	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "ds-add-result.html", map[string]interface{}{"Error": "saved but reload failed: " + err.Error()})
		return
	}

	w.Header().Set("HX-Refresh", "true")
	s.renderPartial(w, "ds-add-result.html", map[string]interface{}{"Name": name})
}

func (s *Server) handleDatasourceDelete(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "name required", 400)
		return
	}

	editor := config.NewYAMLEditor(s.cfgPath)
	if err := editor.DeleteDatasource(name); err != nil {
		s.renderPartial(w, "ds-add-result.html", map[string]interface{}{"Error": err.Error()})
		return
	}
	if err := s.ReloadConfig(); err != nil {
		s.renderPartial(w, "ds-add-result.html", map[string]interface{}{"Error": "deleted but reload failed: " + err.Error()})
		return
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(200)
}

func (s *Server) handleDatasourceTargets(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		s.renderPartial(w, "ds-targets.html", map[string]interface{}{"Error": "no datasource name"})
		return
	}

	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)

	targets, err := disc.FetchTargets(name)
	if err != nil {
		s.renderPartial(w, "ds-targets.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	jobs := generator.GroupTargetsByJob(targets)

	// Enrich jobs with label analysis
	enriched := make([]enrichedJob, len(jobs))
	for i, job := range jobs {
		labels := buildJobLabels(job)
		constCount := 0
		for _, l := range labels {
			if l.Constant {
				constCount++
			}
		}
		enriched[i] = enrichedJob{
			JobSummary: job,
			Labels:     labels,
			LabelCount: len(labels),
			ConstCount: constCount,
		}
	}

	s.renderPartial(w, "ds-targets.html", map[string]interface{}{
		"Datasource":  name,
		"Jobs":        enriched,
		"TargetCount": len(targets),
	})
}

func (s *Server) handleDatasourceTargetMetrics(w http.ResponseWriter, r *http.Request) {
	dsName := r.URL.Query().Get("name")
	job := r.URL.Query().Get("job")
	if dsName == "" || job == "" {
		s.renderPartial(w, "ds-target-metrics.html", map[string]interface{}{"Error": "datasource and job required"})
		return
	}

	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)

	// Get metrics for this job
	allMetrics, err := disc.FetchMetrics(dsName)
	if err != nil {
		s.renderPartial(w, "ds-target-metrics.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	jobMetrics, err := disc.FetchSeriesMetrics(dsName, "job", job)
	if err != nil {
		s.renderPartial(w, "ds-target-metrics.html", map[string]interface{}{"Error": err.Error()})
		return
	}

	// Intersect: only metrics that exist in both sets
	for m := range allMetrics {
		if !jobMetrics[m] {
			delete(allMetrics, m)
		}
	}

	meta, _ := disc.FetchMetadata(dsName)

	names := make([]string, 0, len(allMetrics))
	for m := range allMetrics {
		names = append(names, m)
	}
	sort.Strings(names)

	var rows []metricRow
	for _, m := range names {
		info, ok := meta[m]
		mType := "untyped"
		help := ""
		if ok {
			mType = info.Type
			help = info.Help
		}
		rows = append(rows, metricRow{Name: m, Type: mType, Help: help})
	}

	s.renderPartial(w, "ds-target-metrics.html", map[string]interface{}{
		"Datasource": dsName,
		"Job":        job,
		"Metrics":    rows,
		"Total":      len(rows),
	})
}

// handleLabelsDiscover fetches label names from all datasources (fast — no value fetching).
func (s *Server) handleLabelsDiscover(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)

	type labelInfo struct {
		Name    string
		Sources []string
	}

	// Collect datasources that have URLs
	var dsNames []string
	for name, ds := range cfg.Datasources {
		if ds.URL != "" {
			dsNames = append(dsNames, name)
		}
	}
	sort.Strings(dsNames)

	// Fetch label names only (one request per datasource)
	labelSources := make(map[string][]string) // label → datasource names
	var errors []string

	for _, dsName := range dsNames {
		labels, err := disc.FetchLabels(dsName)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", dsName, err))
			continue
		}
		for _, label := range labels {
			if label == "__name__" {
				continue
			}
			labelSources[label] = append(labelSources[label], dsName)
		}
	}

	// Build sorted label list
	var allLabels []labelInfo
	for name, sources := range labelSources {
		allLabels = append(allLabels, labelInfo{Name: name, Sources: sources})
	}
	sort.Slice(allLabels, func(i, j int) bool {
		return allLabels[i].Name < allLabels[j].Name
	})

	// Check which labels already have variables
	existingVars := make(map[string]bool)
	for name := range cfg.Variables {
		existingVars[name] = true
	}

	s.renderPartial(w, "labels-discover.html", map[string]interface{}{
		"Labels":       allLabels,
		"Datasources":  dsNames,
		"ExistingVars": existingVars,
		"Errors":       errors,
	})
}

// handleLabelValues fetches values for a single label from a datasource.
func (s *Server) handleLabelValues(w http.ResponseWriter, r *http.Request) {
	label := r.URL.Query().Get("label")
	dsName := r.URL.Query().Get("datasource")
	if label == "" || dsName == "" {
		http.Error(w, "label and datasource required", 400)
		return
	}

	cfg := s.Config()
	disc := generator.NewMetricDiscovery(cfg)
	values, err := disc.FetchLabelValues(dsName, label)
	if err != nil {
		s.renderPartial(w, "label-values.html", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}
	if len(values) > 100 {
		values = values[:100]
	}

	s.renderPartial(w, "label-values.html", map[string]interface{}{
		"Values": values,
		"Total":  len(values),
	})
}
