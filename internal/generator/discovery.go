package generator

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/wcatz/dashboard-generator/internal/config"
)

// MetricDiscovery queries Prometheus API for available metrics.
type MetricDiscovery struct {
	Config *config.Config
	cache  map[string]interface{}
}

// NewMetricDiscovery creates a new discovery instance.
func NewMetricDiscovery(cfg *config.Config) *MetricDiscovery {
	return &MetricDiscovery{Config: cfg, cache: make(map[string]interface{})}
}

// MetricInfo holds type and help text for a discovered metric.
type MetricInfo struct {
	Type string
	Help string
}

// TargetInfo holds information about a single Prometheus scrape target.
type TargetInfo struct {
	ScrapePool         string
	Instance           string
	Health             string // "up" or "down"
	Labels             map[string]string
	ScrapeURL          string
	LastScrape         string
	LastScrapeDuration float64
	LastError          string
	ScrapeInterval     string
}

// JobSummary groups targets by scrape pool (job name).
type JobSummary struct {
	Name        string
	TargetCount int
	UpCount     int
	DownCount   int
	Targets     []TargetInfo
}

func (md *MetricDiscovery) get(baseURL, path string) (interface{}, error) {
	url := strings.TrimRight(baseURL, "/") + path
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  error querying %s: %v\n", url, err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if status, ok := result["status"].(string); ok && status != "success" {
		fmt.Fprintf(os.Stderr, "  warning: non-success response from %s\n", url)
	}
	return result["data"], nil
}

// FetchMetrics retrieves all metric names from a datasource.
func (md *MetricDiscovery) FetchMetrics(dsName string) (map[string]bool, error) {
	url := md.Config.GetDatasourceURL(dsName)
	if url == "" {
		return nil, fmt.Errorf("no URL configured for datasource '%s'", dsName)
	}
	key := "metrics:" + dsName
	if cached, ok := md.cache[key]; ok {
		return cached.(map[string]bool), nil
	}
	data, err := md.get(url, "/api/v1/label/__name__/values")
	if err != nil {
		return nil, err
	}
	metrics := make(map[string]bool)
	if list, ok := data.([]interface{}); ok {
		for _, item := range list {
			if s, ok := item.(string); ok {
				metrics[s] = true
			}
		}
	}
	md.cache[key] = metrics
	return metrics, nil
}

// FetchMetadata retrieves metric metadata from a datasource.
func (md *MetricDiscovery) FetchMetadata(dsName string) (map[string]MetricInfo, error) {
	url := md.Config.GetDatasourceURL(dsName)
	if url == "" {
		return map[string]MetricInfo{}, nil
	}
	key := "metadata:" + dsName
	if cached, ok := md.cache[key]; ok {
		return cached.(map[string]MetricInfo), nil
	}
	data, err := md.get(url, "/api/v1/metadata")
	if err != nil {
		return nil, err
	}
	meta := make(map[string]MetricInfo)
	if m, ok := data.(map[string]interface{}); ok {
		for metric, infoList := range m {
			if list, ok := infoList.([]interface{}); ok && len(list) > 0 {
				if info, ok := list[0].(map[string]interface{}); ok {
					mi := MetricInfo{Type: "untyped"}
					if t, ok := info["type"].(string); ok {
						mi.Type = t
					}
					if h, ok := info["help"].(string); ok {
						mi.Help = h
					}
					meta[metric] = mi
				}
			}
		}
	}
	md.cache[key] = meta
	return meta, nil
}

// FetchLabels retrieves all label names from a datasource.
func (md *MetricDiscovery) FetchLabels(dsName string) ([]string, error) {
	url := md.Config.GetDatasourceURL(dsName)
	if url == "" {
		return nil, nil
	}
	data, err := md.get(url, "/api/v1/labels")
	if err != nil {
		return nil, err
	}
	var labels []string
	if list, ok := data.([]interface{}); ok {
		for _, item := range list {
			if s, ok := item.(string); ok {
				labels = append(labels, s)
			}
		}
	}
	return labels, nil
}

// FetchLabelValues retrieves values for a specific label.
func (md *MetricDiscovery) FetchLabelValues(dsName, label string) ([]string, error) {
	url := md.Config.GetDatasourceURL(dsName)
	if url == "" {
		return nil, nil
	}
	data, err := md.get(url, fmt.Sprintf("/api/v1/label/%s/values", label))
	if err != nil {
		return nil, err
	}
	var values []string
	if list, ok := data.([]interface{}); ok {
		for _, item := range list {
			if s, ok := item.(string); ok {
				values = append(values, s)
			}
		}
	}
	return values, nil
}

// FetchSeriesMetrics returns metric names that have a specific label=value pair.
// Uses /api/v1/series?match[]={label="value"} to find matching series.
func (md *MetricDiscovery) FetchSeriesMetrics(dsName, label, value string) (map[string]bool, error) {
	baseURL := md.Config.GetDatasourceURL(dsName)
	if baseURL == "" {
		return nil, fmt.Errorf("no URL configured for datasource '%s'", dsName)
	}
	path := fmt.Sprintf("/api/v1/series?match[]={%s=%q}", label, value)
	data, err := md.get(baseURL, path)
	if err != nil {
		return nil, err
	}
	metrics := make(map[string]bool)
	if list, ok := data.([]interface{}); ok {
		for _, item := range list {
			if series, ok := item.(map[string]interface{}); ok {
				if name, ok := series["__name__"].(string); ok {
					metrics[name] = true
				}
			}
		}
	}
	return metrics, nil
}

// FetchTargets retrieves active scrape targets from a Prometheus datasource.
func (md *MetricDiscovery) FetchTargets(dsName string) ([]TargetInfo, error) {
	baseURL := md.Config.GetDatasourceURL(dsName)
	if baseURL == "" {
		return nil, fmt.Errorf("no URL configured for datasource '%s'", dsName)
	}
	data, err := md.get(baseURL, "/api/v1/targets?state=active")
	if err != nil {
		return nil, err
	}

	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected targets response format")
	}

	activeList, ok := dataMap["activeTargets"].([]interface{})
	if !ok {
		return nil, nil
	}

	var targets []TargetInfo
	for _, item := range activeList {
		t, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		ti := TargetInfo{
			Health: "unknown",
		}
		if v, ok := t["scrapePool"].(string); ok {
			ti.ScrapePool = v
		}
		if v, ok := t["health"].(string); ok {
			ti.Health = v
		}
		if v, ok := t["scrapeUrl"].(string); ok {
			ti.ScrapeURL = v
		}
		if v, ok := t["lastScrape"].(string); ok {
			ti.LastScrape = v
		}
		if v, ok := t["lastScrapeDuration"].(float64); ok {
			ti.LastScrapeDuration = v
		}
		if v, ok := t["lastError"].(string); ok {
			ti.LastError = v
		}
		if v, ok := t["scrapeInterval"].(string); ok {
			ti.ScrapeInterval = v
		}

		ti.Labels = make(map[string]string)
		if labels, ok := t["labels"].(map[string]interface{}); ok {
			for k, v := range labels {
				if s, ok := v.(string); ok {
					ti.Labels[k] = s
				}
			}
		}
		if inst, ok := ti.Labels["instance"]; ok {
			ti.Instance = inst
		}

		targets = append(targets, ti)
	}

	return targets, nil
}

// GroupTargetsByJob organizes targets into job-based summaries.
func GroupTargetsByJob(targets []TargetInfo) []JobSummary {
	groups := make(map[string]*JobSummary)
	var order []string

	for _, t := range targets {
		job := t.ScrapePool
		if job == "" {
			job = t.Labels["job"]
		}
		if job == "" {
			job = "unknown"
		}

		g, ok := groups[job]
		if !ok {
			g = &JobSummary{Name: job}
			groups[job] = g
			order = append(order, job)
		}
		g.TargetCount++
		if t.Health == "up" {
			g.UpCount++
		} else {
			g.DownCount++
		}
		g.Targets = append(g.Targets, t)
	}

	sort.Strings(order)
	result := make([]JobSummary, 0, len(order))
	for _, name := range order {
		result = append(result, *groups[name])
	}
	return result
}

// Categorize compares metrics between two datasources.
func (md *MetricDiscovery) Categorize(dsA, dsB string) (map[string]map[string]MetricInfo, error) {
	metricsA, err := md.FetchMetrics(dsA)
	if err != nil {
		return nil, err
	}
	metricsB, err := md.FetchMetrics(dsB)
	if err != nil {
		return nil, err
	}
	metaA, err := md.FetchMetadata(dsA)
	if err != nil {
		return nil, err
	}
	metaB, err := md.FetchMetadata(dsB)
	if err != nil {
		return nil, err
	}

	shared := make(map[string]MetricInfo)
	onlyA := make(map[string]MetricInfo)
	onlyB := make(map[string]MetricInfo)

	for m := range metricsA {
		info := lookupMeta(m, metaA, metaB)
		if metricsB[m] {
			shared[m] = info
		} else {
			onlyA[m] = info
		}
	}
	for m := range metricsB {
		if !metricsA[m] {
			onlyB[m] = lookupMeta(m, metaB, metaA)
		}
	}

	return map[string]map[string]MetricInfo{
		"shared": shared,
		"only_a": onlyA,
		"only_b": onlyB,
	}, nil
}

// CompareAll compares metrics across N datasources, returning metrics shared
// by all and metrics exclusive to each individual datasource.
func (md *MetricDiscovery) CompareAll(dsNames []string) (shared map[string]MetricInfo, exclusive map[string]map[string]MetricInfo, err error) {
	if len(dsNames) < 2 {
		return nil, nil, fmt.Errorf("need at least 2 datasources")
	}

	allMetrics := make(map[string]map[string]bool)
	allMeta := make(map[string]map[string]MetricInfo)

	for _, ds := range dsNames {
		metrics, ferr := md.FetchMetrics(ds)
		if ferr != nil {
			return nil, nil, fmt.Errorf("fetching metrics from %s: %v", ds, ferr)
		}
		allMetrics[ds] = metrics

		meta, _ := md.FetchMetadata(ds)
		if meta == nil {
			meta = make(map[string]MetricInfo)
		}
		allMeta[ds] = meta
	}

	// Shared = intersection of all metric sets
	shared = make(map[string]MetricInfo)
	for m := range allMetrics[dsNames[0]] {
		onAll := true
		for _, ds := range dsNames[1:] {
			if !allMetrics[ds][m] {
				onAll = false
				break
			}
		}
		if onAll {
			// Use metadata from first DS that has it
			info := MetricInfo{Type: "untyped"}
			for _, ds := range dsNames {
				if i, ok := allMeta[ds][m]; ok && i.Type != "" {
					info = i
					break
				}
			}
			shared[m] = info
		}
	}

	// Exclusive = metrics unique to each DS (not on any other)
	exclusive = make(map[string]map[string]MetricInfo)
	for _, ds := range dsNames {
		unique := make(map[string]MetricInfo)
		for m := range allMetrics[ds] {
			if _, isShared := shared[m]; isShared {
				continue
			}
			onOther := false
			for _, other := range dsNames {
				if other == ds {
					continue
				}
				if allMetrics[other][m] {
					onOther = true
					break
				}
			}
			if !onOther {
				info := MetricInfo{Type: "untyped"}
				if i, ok := allMeta[ds][m]; ok && i.Type != "" {
					info = i
				}
				unique[m] = info
			}
		}
		exclusive[ds] = unique
	}

	return shared, exclusive, nil
}

func lookupMeta(name string, primary, fallback map[string]MetricInfo) MetricInfo {
	if info, ok := primary[name]; ok {
		return info
	}
	if info, ok := fallback[name]; ok {
		return info
	}
	return MetricInfo{Type: "untyped"}
}

// FilterMetrics filters a metric set by include/exclude glob patterns.
func FilterMetrics(metrics map[string]bool, include, exclude []string) map[string]bool {
	if len(include) == 0 {
		include = []string{"*"}
	}
	filtered := make(map[string]bool)
	for m := range metrics {
		included := false
		for _, p := range include {
			if globMatch(p, m) {
				included = true
				break
			}
		}
		excluded := false
		for _, p := range exclude {
			if globMatch(p, m) {
				excluded = true
				break
			}
		}
		if included && !excluded {
			filtered[m] = true
		}
	}
	return filtered
}

// globMatch implements simple glob matching (*, ?).
func globMatch(pattern, name string) bool {
	return matchGlob(pattern, name)
}

func matchGlob(pattern, s string) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			// try matching rest of pattern at each position
			for i := 0; i <= len(s); i++ {
				if matchGlob(pattern[1:], s[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(s) == 0 {
				return false
			}
			pattern = pattern[1:]
			s = s[1:]
		default:
			if len(s) == 0 || pattern[0] != s[0] {
				return false
			}
			pattern = pattern[1:]
			s = s[1:]
		}
	}
	return len(s) == 0
}

// GroupByPrefix groups metrics by first two underscore-delimited segments.
func GroupByPrefix(metrics map[string]MetricInfo) map[string]map[string]MetricInfo {
	groups := make(map[string]map[string]MetricInfo)
	for metric, info := range metrics {
		parts := strings.SplitN(metric, "_", 3)
		var prefix string
		if len(parts) >= 2 {
			prefix = parts[0] + "_" + parts[1]
		} else {
			prefix = parts[0]
		}
		if groups[prefix] == nil {
			groups[prefix] = make(map[string]MetricInfo)
		}
		groups[prefix][metric] = info
	}
	return groups
}

// SuggestPanelType returns a suggested panel type for a metric type.
func SuggestPanelType(metricType string) string {
	switch metricType {
	case "counter":
		return "timeseries"
	case "gauge":
		return "stat"
	case "histogram":
		return "heatmap"
	case "summary":
		return "timeseries"
	default:
		return "timeseries"
	}
}

// SuggestQuery returns a suggested PromQL query for a metric.
func SuggestQuery(metricName, metricType string) string {
	if metricType == "counter" {
		return fmt.Sprintf("rate(%s[5m])", metricName)
	}
	return metricName
}

// PrintDiscovery queries Prometheus and prints suggested YAML config.
func (md *MetricDiscovery) PrintDiscovery(sources, includePatterns, excludePatterns []string) error {
	if len(sources) == 1 {
		return md.printSingleDiscovery(sources[0], includePatterns, excludePatterns)
	}
	if len(sources) == 2 {
		return md.printComparisonDiscovery(sources, includePatterns, excludePatterns)
	}
	return fmt.Errorf("discovery supports 1 or 2 datasources, got %d", len(sources))
}

func (md *MetricDiscovery) printSingleDiscovery(dsName string, include, exclude []string) error {
	metrics, err := md.FetchMetrics(dsName)
	if err != nil {
		return err
	}
	metrics = FilterMetrics(metrics, include, exclude)
	meta, err := md.FetchMetadata(dsName)
	if err != nil {
		return err
	}

	enriched := make(map[string]MetricInfo)
	for m := range metrics {
		if info, ok := meta[m]; ok {
			enriched[m] = info
		} else {
			enriched[m] = MetricInfo{Type: "untyped"}
		}
	}

	fmt.Printf("\n=== Metrics from %s: %d total ===\n\n", dsName, len(metrics))
	grouped := GroupByPrefix(enriched)
	prefixes := sortedKeys(grouped)
	for _, prefix := range prefixes {
		items := grouped[prefix]
		fmt.Printf("# %s_* (%d metrics)\n", prefix, len(items))
		for _, m := range sortedMetricKeys(items) {
			info := items[m]
			panel := SuggestPanelType(info.Type)
			fmt.Printf("  %-60s (%-10s) -> %s\n", m, info.Type, panel)
		}
		fmt.Println()
	}

	md.printYAMLSnippet(grouped, dsName)
	return nil
}

func (md *MetricDiscovery) printComparisonDiscovery(sources, include, exclude []string) error {
	cats, err := md.Categorize(sources[0], sources[1])
	if err != nil {
		return err
	}

	// filter each category
	filterMap := func(m map[string]MetricInfo) map[string]MetricInfo {
		keys := make(map[string]bool)
		for k := range m {
			keys[k] = true
		}
		filtered := FilterMetrics(keys, include, exclude)
		result := make(map[string]MetricInfo)
		for k := range filtered {
			result[k] = m[k]
		}
		return result
	}
	cats["shared"] = filterMap(cats["shared"])
	cats["only_a"] = filterMap(cats["only_a"])
	cats["only_b"] = filterMap(cats["only_b"])

	fmt.Printf("\n=== Metric Comparison ===\n")
	fmt.Printf("  %s: %d metrics\n", sources[0], len(cats["only_a"])+len(cats["shared"]))
	fmt.Printf("  %s: %d metrics\n", sources[1], len(cats["only_b"])+len(cats["shared"]))
	fmt.Printf("  shared: %d\n", len(cats["shared"]))
	fmt.Printf("  %s only: %d\n", sources[0], len(cats["only_a"]))
	fmt.Printf("  %s only: %d\n", sources[1], len(cats["only_b"]))

	fmt.Printf("\n--- Shared Metrics (%d) ---\n", len(cats["shared"]))
	for _, m := range sortedMetricKeys(cats["shared"]) {
		info := cats["shared"][m]
		fmt.Printf("  %-60s (%s)\n", m, info.Type)
	}

	fmt.Printf("\n--- %s Only (%d) ---\n", sources[0], len(cats["only_a"]))
	for _, m := range sortedMetricKeys(cats["only_a"]) {
		info := cats["only_a"][m]
		fmt.Printf("  %-60s (%s)\n", m, info.Type)
	}

	fmt.Printf("\n--- %s Only (%d) ---\n", sources[1], len(cats["only_b"]))
	for _, m := range sortedMetricKeys(cats["only_b"]) {
		info := cats["only_b"][m]
		fmt.Printf("  %-60s (%s)\n", m, info.Type)
	}

	md.printComparisonYAML(cats, sources)
	return nil
}

func (md *MetricDiscovery) printYAMLSnippet(grouped map[string]map[string]MetricInfo, dsName string) {
	fmt.Print("\n# --- suggested YAML config snippet ---\n\n")
	fmt.Println("dashboards:")
	fmt.Println("  discovered:")
	fmt.Printf("    uid: discovered-%s\n", dsName)
	fmt.Printf("    title: discovered metrics (%s)\n", dsName)
	fmt.Printf("    filename: discovered-%s.json\n", dsName)
	fmt.Println("    tags: [discovered]")
	fmt.Println("    variables: []")
	fmt.Println("    sections:")
	for _, prefix := range sortedKeys(grouped) {
		items := grouped[prefix]
		fmt.Printf("      - title: \"%s\"\n", prefix)
		fmt.Println("        panels:")
		for _, m := range sortedMetricKeys(items) {
			info := items[m]
			panel := SuggestPanelType(info.Type)
			query := SuggestQuery(m, info.Type)
			fmt.Printf("          - type: %s\n", panel)
			fmt.Printf("            title: \"%s\"\n", m)
			fmt.Printf("            query: '%s'\n", query)
		}
	}
}

func (md *MetricDiscovery) printComparisonYAML(cats map[string]map[string]MetricInfo, sources []string) {
	fmt.Print("\n# --- suggested comparison YAML snippet ---\n\n")
	fmt.Println("dashboards:")
	fmt.Println("  comparison:")
	fmt.Println("    uid: metric-comparison")
	fmt.Println("    title: metric comparison")
	fmt.Println("    filename: metric-comparison.json")
	fmt.Println("    tags: [comparison]")
	fmt.Println("    variables: []")
	fmt.Println("    sections:")

	if len(cats["shared"]) > 0 {
		fmt.Println("      - title: \"shared metrics\"")
		fmt.Println("        panels:")
		for _, m := range sortedMetricKeys(cats["shared"]) {
			info := cats["shared"][m]
			fmt.Println("          - type: comparison")
			fmt.Printf("            title: \"%s\"\n", m)
			fmt.Printf("            metric: \"%s\"\n", m)
			fmt.Printf("            metric_type: \"%s\"\n", info.Type)
			fmt.Printf("            datasources: [%s, %s]\n", sources[0], sources[1])
		}
	}

	if len(cats["only_a"]) > 0 {
		fmt.Printf("      - title: \"%s only\"\n", sources[0])
		fmt.Println("        panels:")
		for _, m := range sortedMetricKeys(cats["only_a"]) {
			info := cats["only_a"][m]
			panel := SuggestPanelType(info.Type)
			query := SuggestQuery(m, info.Type)
			fmt.Printf("          - type: %s\n", panel)
			fmt.Printf("            title: \"%s\"\n", m)
			fmt.Printf("            query: '%s'\n", query)
			fmt.Printf("            datasource: %s\n", sources[0])
		}
	}

	if len(cats["only_b"]) > 0 {
		fmt.Printf("      - title: \"%s only\"\n", sources[1])
		fmt.Println("        panels:")
		for _, m := range sortedMetricKeys(cats["only_b"]) {
			info := cats["only_b"][m]
			panel := SuggestPanelType(info.Type)
			query := SuggestQuery(m, info.Type)
			fmt.Printf("          - type: %s\n", panel)
			fmt.Printf("            title: \"%s\"\n", m)
			fmt.Printf("            query: '%s'\n", query)
			fmt.Printf("            datasource: %s\n", sources[1])
		}
	}
}

// GenerateDiscoverySections generates dashboard sections from discovered metrics.
func (md *MetricDiscovery) GenerateDiscoverySections(sources, include, exclude []string) ([]config.SectionConfig, error) {
	var sections []config.SectionConfig

	if len(sources) == 1 {
		dsName := sources[0]
		metrics, err := md.FetchMetrics(dsName)
		if err != nil {
			return nil, err
		}
		metrics = FilterMetrics(metrics, include, exclude)
		meta, err := md.FetchMetadata(dsName)
		if err != nil {
			return nil, err
		}

		enriched := make(map[string]MetricInfo)
		for m := range metrics {
			if info, ok := meta[m]; ok {
				enriched[m] = info
			} else {
				enriched[m] = MetricInfo{Type: "untyped"}
			}
		}

		grouped := GroupByPrefix(enriched)
		for _, prefix := range sortedKeys(grouped) {
			items := grouped[prefix]
			var panels []map[string]interface{}
			for _, m := range sortedMetricKeys(items) {
				info := items[m]
				panels = append(panels, map[string]interface{}{
					"type":       SuggestPanelType(info.Type),
					"title":      m,
					"query":      SuggestQuery(m, info.Type),
					"datasource": dsName,
				})
			}
			sections = append(sections, config.SectionConfig{
				Title:  prefix,
				Panels: panels,
			})
		}
	} else if len(sources) == 2 {
		cats, err := md.Categorize(sources[0], sources[1])
		if err != nil {
			return nil, err
		}

		filterMap := func(m map[string]MetricInfo) map[string]MetricInfo {
			keys := make(map[string]bool)
			for k := range m {
				keys[k] = true
			}
			filtered := FilterMetrics(keys, include, exclude)
			result := make(map[string]MetricInfo)
			for k := range filtered {
				result[k] = m[k]
			}
			return result
		}
		cats["shared"] = filterMap(cats["shared"])
		cats["only_a"] = filterMap(cats["only_a"])
		cats["only_b"] = filterMap(cats["only_b"])

		if len(cats["shared"]) > 0 {
			var panels []map[string]interface{}
			for _, m := range sortedMetricKeys(cats["shared"]) {
				info := cats["shared"][m]
				panels = append(panels, map[string]interface{}{
					"type":        "comparison",
					"title":       m,
					"metric":      m,
					"metric_type": info.Type,
					"datasources": []interface{}{sources[0], sources[1]},
				})
			}
			sections = append(sections, config.SectionConfig{
				Title:  "shared metrics",
				Panels: panels,
			})
		}

		for i, cat := range []string{"only_a", "only_b"} {
			if len(cats[cat]) > 0 {
				var panels []map[string]interface{}
				for _, m := range sortedMetricKeys(cats[cat]) {
					info := cats[cat][m]
					panels = append(panels, map[string]interface{}{
						"type":       SuggestPanelType(info.Type),
						"title":      m,
						"query":      SuggestQuery(m, info.Type),
						"datasource": sources[i],
					})
				}
				sections = append(sections, config.SectionConfig{
					Title:  fmt.Sprintf("%s only", sources[i]),
					Panels: panels,
				})
			}
		}
	}

	return sections, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedMetricKeys(m map[string]MetricInfo) []string {
	return sortedKeys(m)
}

