package generator

import (
	"fmt"
	"strings"
)

// PanelSuggestion holds a complete suggestion for a single metric.
type PanelSuggestion struct {
	Type        string            // panel type: stat, gauge, timeseries, heatmap, etc.
	Title       string            // cleaned human-readable title
	Query       string            // PromQL expression
	Unit        string            // Grafana unit string
	Description string            // from Prometheus HELP text
	Legend      string            // legend format template
	Thresholds  string            // threshold ref like "$percent_usage" or empty
	Width       int               // suggested width (0 = use default)
	Height      int               // suggested height (0 = use default)
	Extra       map[string]string // type-specific extras (color_mode, min, max, etc.)
}

// SuggestOptions provides context for generating better suggestions.
type SuggestOptions struct {
	// RateInterval is the rate window constant name (e.g., "rate_interval").
	// If set, queries use ${RateInterval} instead of hardcoded "5m".
	RateInterval string

	// AutoPanels maps Prometheus metric types to override panel types.
	// Keys: "counter", "gauge", "histogram", "summary".
	AutoPanels map[string]string

	// AvailableThresholds is the set of threshold names defined in config.
	AvailableThresholds map[string]bool
}

// InferUnit returns a Grafana unit string based on the metric name suffix.
// Checks suffixes in descending specificity order so _bytes_total matches
// before _bytes or _total individually.
func InferUnit(metricName string) string {
	// Most specific first
	switch {
	case strings.HasSuffix(metricName, "_requests_total"):
		return "reqps"
	case strings.HasSuffix(metricName, "_bytes_total"):
		return "Bps"
	case strings.HasSuffix(metricName, "_bytes_created"):
		return "bytes"
	case strings.HasSuffix(metricName, "_bytes"):
		return "bytes"
	case strings.HasSuffix(metricName, "_seconds_total"):
		return "s"
	case strings.HasSuffix(metricName, "_seconds_bucket"):
		return "s"
	case strings.HasSuffix(metricName, "_seconds_count"):
		return "s"
	case strings.HasSuffix(metricName, "_seconds"):
		return "s"
	case strings.HasSuffix(metricName, "_ratio"):
		return "percentunit"
	case strings.HasSuffix(metricName, "_percent"):
		return "percent"
	case strings.HasSuffix(metricName, "_temperature_celsius"):
		return "celsius"
	case strings.HasSuffix(metricName, "_temperature_fahrenheit"):
		return "fahrenheit"
	case strings.HasSuffix(metricName, "_volts"):
		return "volt"
	case strings.HasSuffix(metricName, "_watts"):
		return "watt"
	case strings.HasSuffix(metricName, "_hertz"):
		return "hertz"
	case strings.HasSuffix(metricName, "_amperes"):
		return "amp"
	case strings.HasSuffix(metricName, "_connections"):
		return "short"
	case strings.HasSuffix(metricName, "_total"):
		return "short"
	case strings.HasSuffix(metricName, "_info"):
		return "short"
	case strings.HasSuffix(metricName, "_bucket"):
		return "short"
	default:
		return "short"
	}
}

// SuggestTitle generates a human-readable title from a metric name.
// Strips common suffixes and replaces underscores with spaces.
func SuggestTitle(metricName string) string {
	title := metricName

	// Strip suffixes in order of specificity (longest first)
	suffixes := []string{
		"_seconds_total",
		"_bytes_total",
		"_seconds_bucket",
		"_seconds_count",
		"_bytes_created",
		"_total",
		"_bucket",
		"_count",
		"_sum",
		"_info",
		"_bytes",
		"_seconds",
	}
	for _, s := range suffixes {
		if strings.HasSuffix(title, s) {
			title = strings.TrimSuffix(title, s)
			break
		}
	}

	// Replace underscores with spaces
	title = strings.ReplaceAll(title, "_", " ")

	return title
}

// SuggestLegend returns a legend format template for the metric type.
func SuggestLegend(metricType string) string {
	switch metricType {
	case "histogram":
		return "p95"
	default:
		return "{{instance}}"
	}
}

// SuggestThresholds returns a threshold reference (e.g., "$percent_usage")
// if an appropriate threshold exists in the config. Returns empty string if
// no matching threshold is available.
func SuggestThresholds(metricName, unit string, available map[string]bool) string {
	if available == nil {
		return ""
	}

	switch {
	case (unit == "percent" || unit == "percentunit") && available["percent_usage"]:
		return "$percent_usage"
	case strings.HasSuffix(metricName, "_info") && available["binary_health"]:
		return "$binary_health"
	case unit == "s" && available["latency_ms"]:
		return "$latency_ms"
	default:
		return ""
	}
}

// SuggestSize returns a suggested panel size (width, height) based on the
// panel type and number of metrics being displayed. Returns 0,0 when the
// default size for the panel type should be used.
func SuggestSize(panelType string, metricCount int) (int, int) {
	switch panelType {
	case "stat":
		if metricCount > 0 && metricCount <= 8 {
			w := 24 / metricCount
			if w < 3 {
				w = 3
			}
			return w, 4
		}
		return 6, 4
	case "gauge":
		if metricCount > 0 && metricCount <= 4 {
			return 24 / metricCount, 4
		}
		return 6, 4
	case "timeseries":
		if metricCount == 1 {
			return 24, 7
		}
		return 12, 7
	default:
		return 0, 0
	}
}

// SuggestPanel generates a complete PanelSuggestion for a metric.
// It considers the Prometheus metric type, name suffixes, and available
// config context to produce production-ready panel configs.
func SuggestPanel(metricName string, info MetricInfo, opts *SuggestOptions) PanelSuggestion {
	if opts == nil {
		opts = &SuggestOptions{}
	}

	unit := InferUnit(metricName)
	metricType := info.Type

	// Check auto_panels override first
	panelType := suggestPanelTypeFromRules(metricName, metricType, opts.AutoPanels)

	// Build the query
	query := suggestQueryExpr(metricName, metricType, opts.RateInterval)

	// Build extras for type-specific config
	extra := make(map[string]string)

	// Adjust based on panel type + metric characteristics
	switch panelType {
	case "stat":
		if strings.HasSuffix(metricName, "_info") {
			extra["color_mode"] = "background"
		}
	case "gauge":
		extra["min"] = "0"
		switch unit {
		case "percentunit":
			extra["max"] = "1"
		case "percent":
			extra["max"] = "100"
		}
	}

	return PanelSuggestion{
		Type:        panelType,
		Title:       SuggestTitle(metricName),
		Query:       query,
		Unit:        unit,
		Description: info.Help,
		Legend:      SuggestLegend(metricType),
		Thresholds:  SuggestThresholds(metricName, unit, opts.AvailableThresholds),
		Extra:       extra,
	}
}

// suggestPanelTypeFromRules determines the panel type from Prometheus type,
// metric name, and optional auto_panels overrides.
func suggestPanelTypeFromRules(metricName, metricType string, autoPanels map[string]string) string {
	// Check auto_panels override
	if autoPanels != nil {
		if override, ok := autoPanels[metricType]; ok {
			return override
		}
		// Also check plural forms (counters, gauges, etc.) for config compat
		if override, ok := autoPanels[metricType+"s"]; ok {
			return override
		}
	}

	switch metricType {
	case "counter":
		return "timeseries"
	case "gauge":
		if strings.HasSuffix(metricName, "_ratio") || strings.HasSuffix(metricName, "_percent") {
			return "gauge"
		}
		if strings.HasSuffix(metricName, "_info") {
			return "stat"
		}
		return "stat"
	case "histogram":
		if strings.HasSuffix(metricName, "_bucket") {
			return "timeseries" // histogram_quantile visualization
		}
		return "heatmap"
	case "summary":
		return "timeseries"
	default:
		// Untyped — infer from name
		if strings.HasSuffix(metricName, "_total") || strings.HasSuffix(metricName, "_bytes_total") || strings.HasSuffix(metricName, "_seconds_total") {
			return "timeseries"
		}
		if strings.HasSuffix(metricName, "_info") {
			return "stat"
		}
		return "timeseries"
	}
}

// suggestQueryExpr builds a PromQL expression appropriate for the metric.
func suggestQueryExpr(metricName, metricType, rateInterval string) string {
	interval := "5m"
	if rateInterval != "" {
		interval = fmt.Sprintf("${%s}", rateInterval)
	}

	switch metricType {
	case "counter":
		return fmt.Sprintf("rate(%s[%s])", metricName, interval)
	case "histogram":
		if strings.HasSuffix(metricName, "_bucket") {
			// Generate histogram_quantile for bucket metrics
			return fmt.Sprintf("histogram_quantile(0.95, sum(rate(%s[%s])) by (le))", metricName, interval)
		}
		return metricName
	default:
		// Untyped with counter-like name
		if strings.HasSuffix(metricName, "_total") {
			return fmt.Sprintf("rate(%s[%s])", metricName, interval)
		}
		return metricName
	}
}

// FormatSnippetYAML formats a slice of PanelSuggestions as a YAML section
// snippet ready to paste into a config file.
func FormatSnippetYAML(suggestions []PanelSuggestion, sectionTitle, dsName string) (string, []string) {
	var lines []string
	var hints []string

	lines = append(lines, fmt.Sprintf("      - title: \"%s\"", sectionTitle))
	lines = append(lines, "        panels:")

	for _, s := range suggestions {
		lines = append(lines, fmt.Sprintf("          - type: %s", s.Type))
		lines = append(lines, fmt.Sprintf("            title: \"%s\"", s.Title))
		lines = append(lines, fmt.Sprintf("            query: '%s'", s.Query))
		if dsName != "" {
			lines = append(lines, fmt.Sprintf("            datasource: %s", dsName))
		}
		if s.Unit != "" && s.Unit != "short" {
			lines = append(lines, fmt.Sprintf("            unit: %s", s.Unit))
			hints = append(hints, fmt.Sprintf("inferred unit: %s for %s", s.Unit, s.Title))
		}
		if s.Description != "" {
			// Escape quotes in description
			desc := strings.ReplaceAll(s.Description, "\"", "\\\"")
			lines = append(lines, fmt.Sprintf("            description: \"%s\"", desc))
		}
		if s.Legend != "" {
			lines = append(lines, fmt.Sprintf("            legend: \"%s\"", s.Legend))
		}
		if s.Thresholds != "" {
			lines = append(lines, fmt.Sprintf("            thresholds: %s", s.Thresholds))
			hints = append(hints, fmt.Sprintf("suggested thresholds: %s for %s", s.Thresholds, s.Title))
		}
		if s.Width > 0 {
			lines = append(lines, fmt.Sprintf("            width: %d", s.Width))
		}
		if s.Height > 0 {
			lines = append(lines, fmt.Sprintf("            height: %d", s.Height))
		}
		for k, v := range s.Extra {
			lines = append(lines, fmt.Sprintf("            %s: %s", k, v))
		}
	}

	return strings.Join(lines, "\n"), hints
}

// SuggestVariablesFromLabels filters discovered labels to those worth creating as template variables.
// Excludes internal/low-cardinality labels and labels that already exist as variables.
func SuggestVariablesFromLabels(labels []string, existingVars []string) []string {
	existing := make(map[string]bool, len(existingVars))
	for _, v := range existingVars {
		existing[v] = true
	}

	// Labels that are almost never useful as template variables
	skip := map[string]bool{
		"__name__":           true,
		"le":                 true,
		"quantile":           true,
		"alertname":          true,
		"alertstate":         true,
		"prometheus":         true,
		"prometheus_replica": true,
	}

	var suggestions []string
	for _, label := range labels {
		if skip[label] || existing[label] {
			continue
		}
		// Skip internal labels (start with __)
		if strings.HasPrefix(label, "__") {
			continue
		}
		suggestions = append(suggestions, label)
	}
	return suggestions
}

// FormatComparisonSnippetYAML formats comparison panel suggestions as YAML.
func FormatComparisonSnippetYAML(metricNames []string, metricInfos map[string]MetricInfo, dsList []string, opts *SuggestOptions) (string, []string) {
	var lines []string
	var hints []string

	dsListStr := strings.Join(dsList, ", ")
	lines = append(lines, "      - title: \"shared metrics comparison\"")
	lines = append(lines, "        panels:")

	for _, m := range metricNames {
		info := metricInfos[m]
		if info.Type == "" {
			info.Type = "untyped"
		}

		title := SuggestTitle(m)
		unit := InferUnit(m)

		lines = append(lines, "          - type: comparison")
		lines = append(lines, fmt.Sprintf("            title: \"%s\"", title))
		lines = append(lines, fmt.Sprintf("            metric: \"%s\"", m))
		lines = append(lines, fmt.Sprintf("            metric_type: \"%s\"", info.Type))
		lines = append(lines, fmt.Sprintf("            datasources: [%s]", dsListStr))
		if unit != "" && unit != "short" {
			lines = append(lines, fmt.Sprintf("            unit: %s", unit))
			hints = append(hints, fmt.Sprintf("inferred unit: %s for %s", unit, title))
		}
		if info.Help != "" {
			desc := strings.ReplaceAll(info.Help, "\"", "\\\"")
			lines = append(lines, fmt.Sprintf("            description: \"%s\"", desc))
		}
	}

	return strings.Join(lines, "\n"), hints
}
