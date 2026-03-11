package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/wcatz/dashboard-generator/internal/config"
	"github.com/wcatz/dashboard-generator/internal/generator"
)

// initAIClient checks AI availability and returns an initialized client and config context.
// Returns nil client and a non-empty errMsg if AI is not available.
func (s *Server) initAIClient(cfg *config.Config) (client *generator.AIClient, configCtx generator.ConfigContext, errMsg string) {
	if !generator.IsAIAvailable(cfg) {
		return nil, generator.ConfigContext{}, "AI not configured. Set anthropic_api_key in config or ANTHROPIC_API_KEY environment variable."
	}
	aiClient := generator.NewAIClient(cfg)
	if aiClient == nil {
		return nil, generator.ConfigContext{}, "Failed to initialize AI client"
	}
	return aiClient, generator.BuildConfigContext(cfg), ""
}

// handleAISuggest generates a panel YAML suggestion for a single metric using AI.
func (s *Server) handleAISuggest(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()
	aiClient, configCtx, errMsg := s.initAIClient(cfg)
	if errMsg != "" {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{"Error": errMsg})
		return
	}

	metricName := strings.TrimSpace(r.FormValue("metric"))
	metricType := r.FormValue("type")
	metricHelp := r.FormValue("help")

	if metricName == "" {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{"Error": "Metric name is required"})
		return
	}

	metrics := []generator.MetricContext{{Name: metricName, Type: metricType, Help: metricHelp}}
	suggestion, err := aiClient.Suggest(metrics, configCtx)
	if err != nil {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{"Error": fmt.Sprintf("AI suggestion failed: %v", err)})
		return
	}
	dashboards, _ := cfg.GetDashboardOrder("")
	s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{
		"YAML":       suggestion.YAML,
		"Notes":      suggestion.Notes,
		"MetricName": metricName,
		"Dashboards": dashboards,
	})
}

// handleAISuggestBulk generates a dashboard section YAML from multiple metrics.
func (s *Server) handleAISuggestBulk(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()
	aiClient, configCtx, errMsg := s.initAIClient(cfg)
	if errMsg != "" {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{"Error": errMsg})
		return
	}

	// Parse metrics from JSON payload
	metricsJSON := r.FormValue("metrics")
	if metricsJSON == "" {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{"Error": "No metrics provided"})
		return
	}

	var metricData []struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Help string `json:"help"`
	}

	if err := json.Unmarshal([]byte(metricsJSON), &metricData); err != nil {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{"Error": fmt.Sprintf("Invalid metrics data: %v", err)})
		return
	}

	if len(metricData) == 0 {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{"Error": "No metrics selected"})
		return
	}

	// Limit to reasonable number of metrics
	if len(metricData) > 20 {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{"Error": "Too many metrics selected (max 20)"})
		return
	}

	// Convert to MetricContext, validating names
	metrics := make([]generator.MetricContext, 0, len(metricData))
	for _, m := range metricData {
		name := strings.TrimSpace(m.Name)
		if name == "" {
			s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{"Error": "Metric name required for all entries"})
			return
		}
		metrics = append(metrics, generator.MetricContext{Name: name, Type: m.Type, Help: m.Help})
	}

	// Call AI to generate suggestion
	suggestion, err := aiClient.Suggest(metrics, configCtx)
	if err != nil {
		s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{"Error": fmt.Sprintf("AI suggestion failed: %v", err)})
		return
	}

	// Build metric names list for display
	var metricNames []string
	for _, m := range metrics {
		metricNames = append(metricNames, m.Name)
	}

	// Render success response with section YAML
	dashboards, _ := cfg.GetDashboardOrder("")
	s.renderPartial(w, "ai-suggestion.html", map[string]interface{}{
		"YAML":        suggestion.YAML,
		"Notes":       suggestion.Notes,
		"IsBulk":      true,
		"MetricCount": len(metrics),
		"MetricNames": strings.Join(metricNames, ", "),
		"Dashboards":  dashboards,
	})
}
