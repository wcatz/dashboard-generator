package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/wcatz/dashboard-generator/internal/generator"
)

// handleAISuggest generates a panel YAML suggestion for a single metric using AI.
func (s *Server) handleAISuggest(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()

	// Check if AI is available
	if !generator.IsAIAvailable(cfg) {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{
			"Error": "AI not configured. Set anthropic_api_key in config or ANTHROPIC_API_KEY environment variable.",
		})
		return
	}

	// Extract metric information from form
	metricName := r.FormValue("metric")
	metricType := r.FormValue("type")
	metricHelp := r.FormValue("help")

	if metricName == "" {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{
			"Error": "Metric name is required",
		})
		return
	}

	// Build AI context
	aiClient := generator.NewAIClient(cfg)
	if aiClient == nil {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{
			"Error": "Failed to initialize AI client",
		})
		return
	}

	metrics := []generator.MetricContext{
		{
			Name: metricName,
			Type: metricType,
			Help: metricHelp,
		},
	}

	configCtx := generator.BuildConfigContext(cfg)

	// Call AI to generate suggestion
	suggestion, err := aiClient.Suggest(metrics, configCtx)
	if err != nil {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{
			"Error": fmt.Sprintf("AI suggestion failed: %v", err),
		})
		return
	}

	// Render success response with YAML and notes
	s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{
		"YAML":       suggestion.YAML,
		"Notes":      suggestion.Notes,
		"MetricName": metricName,
	})
}

// handleAISuggestBulk generates a dashboard section YAML from multiple metrics.
func (s *Server) handleAISuggestBulk(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()

	// Check if AI is available
	if !generator.IsAIAvailable(cfg) {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{
			"Error": "AI not configured",
		})
		return
	}

	// Parse metrics from JSON payload
	metricsJSON := r.FormValue("metrics")
	if metricsJSON == "" {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{
			"Error": "No metrics provided",
		})
		return
	}

	var metricData []struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Help string `json:"help"`
	}

	if err := json.Unmarshal([]byte(metricsJSON), &metricData); err != nil {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{
			"Error": fmt.Sprintf("Invalid metrics data: %v", err),
		})
		return
	}

	if len(metricData) == 0 {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{
			"Error": "No metrics selected",
		})
		return
	}

	// Limit to reasonable number of metrics
	if len(metricData) > 20 {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{
			"Error": "Too many metrics selected (max 20)",
		})
		return
	}

	// Convert to MetricContext
	metrics := make([]generator.MetricContext, len(metricData))
	for i, m := range metricData {
		metrics[i] = generator.MetricContext{
			Name: m.Name,
			Type: m.Type,
			Help: m.Help,
		}
	}

	// Build AI context
	aiClient := generator.NewAIClient(cfg)
	if aiClient == nil {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{
			"Error": "Failed to initialize AI client",
		})
		return
	}

	configCtx := generator.BuildConfigContext(cfg)

	// Call AI to generate suggestion
	suggestion, err := aiClient.Suggest(metrics, configCtx)
	if err != nil {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{
			"Error": fmt.Sprintf("AI suggestion failed: %v", err),
		})
		return
	}

	// Build metric names list for display
	var metricNames []string
	for _, m := range metricData {
		metricNames = append(metricNames, m.Name)
	}

	// Render success response with section YAML
	s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{
		"YAML":        suggestion.YAML,
		"Notes":       suggestion.Notes,
		"IsBulk":      true,
		"MetricCount": len(metrics),
		"MetricNames": strings.Join(metricNames, ", "),
	})
}
