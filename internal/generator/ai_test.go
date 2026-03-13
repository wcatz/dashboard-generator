package generator

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wcatz/dashboard-generator/internal/config"
)

func TestBuildSystemPrompt(t *testing.T) {
	ctx := ConfigContext{
		Selectors:  map[string]string{"host": `{instance=~"$instance"}`},
		Constants:  map[string]string{"rate_interval": "5m"},
		Thresholds: []string{"percent_usage", "binary_health"},
		Palettes:   []string{"green", "red", "blue"},
		Variables:  []string{"instance", "namespace"},
	}

	prompt := buildSystemPrompt(ctx)

	// Should contain schema requirement
	if !strings.Contains(prompt, "panels:") {
		t.Error("expected panels nesting requirement in system prompt")
	}
	if !strings.Contains(prompt, "CRITICAL") {
		t.Error("expected CRITICAL nesting warning in system prompt")
	}

	// Should contain panel type reference
	if !strings.Contains(prompt, "timeseries") {
		t.Error("expected panel type reference in system prompt")
	}

	// Should contain selectors
	if !strings.Contains(prompt, "${host}") {
		t.Error("expected selectors in system prompt")
	}

	// Should contain constants
	if !strings.Contains(prompt, "${rate_interval}") {
		t.Error("expected constants in system prompt")
	}

	// Should contain thresholds
	if !strings.Contains(prompt, "$percent_usage") {
		t.Error("expected thresholds in system prompt")
	}

	// Should contain palette colors
	if !strings.Contains(prompt, "$green") {
		t.Error("expected palette colors in system prompt")
	}

	// Should contain variables
	if !strings.Contains(prompt, "$instance") {
		t.Error("expected variables in system prompt")
	}
}

func TestBuildUserPrompt(t *testing.T) {
	metrics := []MetricContext{
		{Name: "node_cpu_seconds_total", Type: "counter", Help: "CPU time spent.", Labels: []string{"cpu", "mode"}},
		{Name: "node_memory_MemTotal_bytes", Type: "gauge", Help: "Total memory."},
	}

	prompt := buildUserPrompt(metrics)

	if !strings.Contains(prompt, "node_cpu_seconds_total") {
		t.Error("expected metric name in user prompt")
	}
	if !strings.Contains(prompt, "counter") {
		t.Error("expected metric type in user prompt")
	}
	if !strings.Contains(prompt, "CPU time spent.") {
		t.Error("expected help text in user prompt")
	}
	if !strings.Contains(prompt, "cpu, mode") {
		t.Error("expected labels in user prompt")
	}
}

func TestExtractYAMLAndNotes_Plain(t *testing.T) {
	input := `      - title: "cpu metrics"
        panels:
          - type: timeseries
            title: "cpu usage"
            query: 'rate(node_cpu_seconds_total[5m])'`

	yaml, notes := extractYAMLAndNotes(input)

	if !strings.Contains(yaml, "cpu metrics") {
		t.Error("expected YAML content preserved")
	}
	if len(notes) != 0 {
		t.Errorf("expected no notes, got %v", notes)
	}
}

func TestExtractYAMLAndNotes_Fenced(t *testing.T) {
	input := "```yaml\n      - title: \"test\"\n        panels: []\n```"

	yaml, _ := extractYAMLAndNotes(input)

	if !strings.Contains(yaml, "title: \"test\"") {
		t.Errorf("expected fenced YAML extracted, got: %s", yaml)
	}
	if strings.Contains(yaml, "```") {
		t.Error("expected fences stripped")
	}
}

func TestExtractYAMLAndNotes_WithNotes(t *testing.T) {
	input := `      - title: "test"
        panels: []
# Notes:
# - Used rate() for counters
# - Applied percent_usage thresholds`

	yaml, notes := extractYAMLAndNotes(input)

	if !strings.Contains(yaml, "title: \"test\"") {
		t.Error("expected YAML content")
	}
	if len(notes) != 2 {
		t.Errorf("expected 2 notes, got %d: %v", len(notes), notes)
	}
	if notes[0] != "Used rate() for counters" {
		t.Errorf("unexpected note: %s", notes[0])
	}
}

func TestParseAIResponse(t *testing.T) {
	resp := map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": "      - title: \"test\"\n        panels:\n          - type: stat\n            title: \"up\"\n            query: 'up'\n# Notes:\n# - Simple health check",
			},
		},
	}
	body, _ := json.Marshal(resp)

	result, err := parseAIResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.YAML, "type: stat") {
		t.Error("expected YAML in response")
	}
	if len(result.Notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(result.Notes))
	}
}

func TestParseAIResponse_Error(t *testing.T) {
	resp := map[string]interface{}{
		"error": map[string]interface{}{
			"type":    "invalid_request_error",
			"message": "bad request",
		},
	}
	body, _ := json.Marshal(resp)

	_, err := parseAIResponse(body)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "bad request") {
		t.Errorf("expected error message, got: %v", err)
	}
}

func TestParseAIResponse_Empty(t *testing.T) {
	resp := map[string]interface{}{
		"content": []map[string]interface{}{},
	}
	body, _ := json.Marshal(resp)

	_, err := parseAIResponse(body)
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestParseAIResponse_MultipleTextBlocks(t *testing.T) {
	resp := map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": "      - title: \"part one\"\n        panels:"},
			{"type": "text", "text": "          - type: stat\n            title: \"up\""},
		},
	}
	body, _ := json.Marshal(resp)

	result, err := parseAIResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.YAML, "part one") {
		t.Error("expected first block content")
	}
	if !strings.Contains(result.YAML, "type: stat") {
		t.Error("expected second block content")
	}
}

func TestAIClient_Suggest_MockServer(t *testing.T) {
	// Create a mock Anthropic API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key header, got: %s", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != anthropicAPIVersion {
			t.Errorf("expected anthropic-version header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json")
		}

		resp := map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": "      - title: \"node metrics\"\n        panels:\n          - type: timeseries\n            title: \"cpu usage\"\n            query: 'rate(node_cpu_seconds_total[5m])'\n            unit: s\n# Notes:\n# - Applied rate() for counter type",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &AIClient{
		APIKey:     "test-key",
		Model:      "test-model",
		BaseURL:    server.URL,
		httpClient: server.Client(),
	}

	metrics := []MetricContext{
		{Name: "node_cpu_seconds_total", Type: "counter", Help: "CPU time."},
	}
	ctx := ConfigContext{
		Constants: map[string]string{"rate_interval": "5m"},
	}

	result, err := client.Suggest(metrics, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.YAML, "type: timeseries") {
		t.Error("expected timeseries in YAML response")
	}
	if len(result.Notes) != 1 || result.Notes[0] != "Applied rate() for counter type" {
		t.Errorf("unexpected notes: %v", result.Notes)
	}
}

func TestNewAIClient_FromConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Generator.AnthropicAPIKey = "test-key-123"
	cfg.Generator.AnthropicModel = "claude-sonnet-4-20250514"

	client := NewAIClient(cfg)

	if client.APIKey != "test-key-123" {
		t.Errorf("expected API key from config, got: %s", client.APIKey)
	}
	if client.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected model from config, got: %s", client.Model)
	}
	if !client.Available() {
		t.Error("expected client to be available")
	}
}

func TestNewAIClient_DefaultModel(t *testing.T) {
	cfg := &config.Config{}
	cfg.Generator.AnthropicAPIKey = "key"

	client := NewAIClient(cfg)

	if client.Model != defaultModel {
		t.Errorf("expected default model %s, got: %s", defaultModel, client.Model)
	}
}

func TestNewAIClient_NotAvailable(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "") // ensure env var doesn't leak into test
	cfg := &config.Config{}

	client := NewAIClient(cfg)

	if client.Available() {
		t.Error("expected client to not be available without API key")
	}
}

func TestIsAIAvailable(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "") // ensure env var doesn't leak into test
	cfg := &config.Config{}
	if IsAIAvailable(cfg) {
		t.Error("expected not available without key")
	}

	cfg.Generator.AnthropicAPIKey = "key"
	if !IsAIAvailable(cfg) {
		t.Error("expected available with key in config")
	}
}

func TestAIClient_BaseURL(t *testing.T) {
	cfg := &config.Config{}
	cfg.Generator.AnthropicAPIKey = "key"
	client := NewAIClient(cfg)

	if client.BaseURL != anthropicAPIURL {
		t.Errorf("expected default BaseURL %s, got: %s", anthropicAPIURL, client.BaseURL)
	}
}

func TestValidateAISectionYAML_Valid(t *testing.T) {
	yaml := `- title: "overview"
  panels:
    - type: stat
      title: "uptime"
      query: 'up'`

	err := ValidateAISectionYAML(yaml)
	if err != nil {
		t.Errorf("expected valid YAML, got error: %v", err)
	}
}

func TestValidateAISectionYAML_FlatPanels(t *testing.T) {
	yaml := `- title: "overview"
  type: row
- title: "uptime"
  type: stat
  query: 'up'`

	err := ValidateAISectionYAML(yaml)
	if err == nil {
		t.Error("expected error for flat panels without nesting")
	}
}

func TestValidateAISectionYAML_Empty(t *testing.T) {
	err := ValidateAISectionYAML("")
	if err == nil {
		t.Error("expected error for empty YAML")
	}
}

func TestRepairFlatSectionYAML_AlreadyValid(t *testing.T) {
	yaml := `- title: "overview"
  panels:
    - type: stat
      title: "uptime"
      query: 'up'`

	result, repaired := RepairFlatSectionYAML(yaml)
	if repaired {
		t.Error("expected no repair for valid YAML")
	}
	if result != yaml {
		t.Error("expected original YAML returned unchanged")
	}
}

func TestRepairFlatSectionYAML_FlatPanels(t *testing.T) {
	yaml := `- title: "overview"
  type: row
- title: "uptime"
  type: stat
  query: 'up'
- title: "memory"
  type: gauge
  query: 'node_memory'`

	result, repaired := RepairFlatSectionYAML(yaml)
	if !repaired {
		t.Fatal("expected repair to be applied")
	}

	// Repaired YAML should now be valid
	if err := ValidateAISectionYAML(result); err != nil {
		t.Errorf("repaired YAML should be valid, got: %v", err)
	}

	// Should contain the panel titles
	if !strings.Contains(result, "uptime") {
		t.Error("expected panel title 'uptime' preserved")
	}
	if !strings.Contains(result, "memory") {
		t.Error("expected panel title 'memory' preserved")
	}
}

func TestRepairFlatSectionYAML_NoPrecedingSection(t *testing.T) {
	yaml := `- title: "cpu usage"
  type: timeseries
  query: 'rate(cpu[5m])'`

	result, repaired := RepairFlatSectionYAML(yaml)
	if !repaired {
		t.Fatal("expected repair to be applied")
	}

	if err := ValidateAISectionYAML(result); err != nil {
		t.Errorf("repaired YAML should be valid, got: %v", err)
	}

	// Should create a default section title
	if !strings.Contains(result, "generated panels") {
		t.Error("expected default section title 'generated panels'")
	}
}

func TestBuildConfigContext(t *testing.T) {
	cfg := &config.Config{
		Selectors: map[string]string{"host": `{instance=~"$instance"}`},
		Constants: map[string]string{"rate_interval": "5m"},
		Thresholds: map[string][]config.ThresholdStep{
			"percent_usage": {{Color: "green"}, {Color: "red", Value: "90"}},
		},
		Palettes: map[string]map[string]string{
			"grafana": {"green": "#73BF69", "red": "#F2495C"},
		},
		ActivePalette: "grafana",
		Variables: map[string]config.VariableDef{
			"instance": {Type: "query"},
		},
	}

	ctx := BuildConfigContext(cfg)

	if ctx.Selectors["host"] == "" {
		t.Error("expected selectors in context")
	}
	if ctx.Constants["rate_interval"] == "" {
		t.Error("expected constants in context")
	}
	if len(ctx.Thresholds) != 1 {
		t.Errorf("expected 1 threshold, got %d", len(ctx.Thresholds))
	}
	if len(ctx.Palettes) != 2 {
		t.Errorf("expected 2 palette colors, got %d", len(ctx.Palettes))
	}
	if len(ctx.Variables) != 1 {
		t.Errorf("expected 1 variable, got %d", len(ctx.Variables))
	}
}
