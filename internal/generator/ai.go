package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/wcatz/dashboard-generator/internal/config"
)

const (
	anthropicAPIURL     = "https://api.anthropic.com/v1/messages"
	anthropicAPIVersion = "2023-06-01"
	defaultModel        = "claude-haiku-4-5-20251001"
	maxTokens           = 4096
)

// AIClient calls the Anthropic Messages API to generate panel suggestions.
type AIClient struct {
	APIKey     string
	Model      string
	BaseURL    string // configurable for testing; defaults to anthropicAPIURL
	httpClient *http.Client
}

// IsAIAvailable reports whether an Anthropic API key is configured,
// without allocating an http.Client.
func IsAIAvailable(cfg *config.Config) bool {
	return cfg.Generator.AnthropicAPIKey != "" || os.Getenv("ANTHROPIC_API_KEY") != ""
}

// NewAIClient creates a new AI client. Falls back to ANTHROPIC_API_KEY env var.
func NewAIClient(cfg *config.Config) *AIClient {
	apiKey := cfg.Generator.ResolvedAnthropicAPIKey()
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	model := cfg.Generator.AnthropicModel
	if model == "" {
		model = defaultModel
	}

	return &AIClient{
		APIKey:     apiKey,
		Model:      model,
		BaseURL:    anthropicAPIURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Available returns true if an API key is configured.
func (c *AIClient) Available() bool {
	return c.APIKey != ""
}

// MetricContext holds metadata for a single metric to send to the AI.
type MetricContext struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Help   string   `json:"help"`
	Labels []string `json:"labels,omitempty"`
}

// ConfigContext holds config context to inform AI suggestions.
type ConfigContext struct {
	Selectors  map[string]string `json:"selectors"`
	Constants  map[string]string `json:"constants"`
	Thresholds []string          `json:"thresholds"`
	Palettes   []string          `json:"palettes"`
	Variables  []string          `json:"variables"`
}

// AISuggestionResponse holds the AI's response.
type AISuggestionResponse struct {
	YAML  string   // complete YAML section(s)
	Notes []string // explanatory notes about choices
}

// Suggest calls the Anthropic API to generate panel YAML for the given metrics.
func (c *AIClient) Suggest(metrics []MetricContext, configCtx ConfigContext) (*AISuggestionResponse, error) {
	if !c.Available() {
		return nil, fmt.Errorf("anthropic API key not configured")
	}

	systemPrompt := buildSystemPrompt(configCtx)
	userPrompt := buildUserPrompt(metrics)

	body := map[string]interface{}{
		"model":      c.Model,
		"max_tokens": maxTokens,
		"system":     systemPrompt,
		"messages": []map[string]interface{}{
			{"role": "user", "content": userPrompt},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = anthropicAPIURL
	}
	req, err := http.NewRequest(http.MethodPost, baseURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	return parseAIResponse(respBody)
}

// BuildConfigContext creates a ConfigContext from a Config.
func BuildConfigContext(cfg *config.Config) ConfigContext {
	ctx := ConfigContext{
		Selectors: cfg.Selectors,
		Constants: cfg.Constants,
	}

	for name := range cfg.Thresholds {
		ctx.Thresholds = append(ctx.Thresholds, name)
	}

	if palette := cfg.Palettes[cfg.ActivePalette]; palette != nil {
		for name := range palette {
			ctx.Palettes = append(ctx.Palettes, name)
		}
	}

	for name := range cfg.Variables {
		ctx.Variables = append(ctx.Variables, name)
	}

	return ctx
}

func buildSystemPrompt(ctx ConfigContext) string {
	var sb strings.Builder

	sb.WriteString(`You are a Grafana dashboard expert. Generate YAML panel configurations for a config-driven dashboard generator.

## Output Rules
- Return ONLY valid YAML for a dashboard section, starting with "      - title:"
- No markdown fences, no explanations outside the YAML
- After the YAML, you may add a line starting with "# Notes:" followed by brief notes about your choices

## Panel Types Available
| Type | Default Size | Use For |
|------|-------------|---------|
| stat | 3x4 | Single values, counts, current state |
| gauge | 3x4 | Percentages with min/max (0-100) |
| timeseries | 12x7 | Time-based trends, rates, comparisons |
| bargauge | 6x5 | Ranked values, horizontal bars |
| heatmap | 12x8 | Distribution over time |
| histogram | 12x7 | Value distribution |
| table | 24x8 | Detailed multi-column data |
| piechart | 6x6 | Proportional breakdowns |
| state-timeline | 12x5 | State changes over time |
| status-history | 12x5 | Multi-target status history |
| text | 24x3 | Markdown documentation |
| logs | 24x8 | Log streams |
| row | 24x1 | Section headers / grouping |
| comparison | 12x8 | Cross-datasource metric overlay |

## Panel Config Keys
type, title, query, targets (list of {expr, legend, datasource}), width, height,
datasource, unit, description, color, thresholds, transparent, overrides,
value_mappings, data_links, repeat, calcs

### Type-Specific
- stat: color_mode (background/value), graph_mode (none/area), text_mode
- gauge: min, max, show_threshold_labels, show_threshold_markers
- timeseries: fill_opacity, line_width, stack (none/normal), draw_style, line_interpolation, legend_calcs, legend_mode, legend_placement, color_mode
- bargauge: display_mode (gradient/lcd/basic), orientation
- heatmap: color_scheme, color_scale, calculate
- table: filterable, sort_by, transformations
- piechart: pie_type (donut/pie), display_labels

## Style Conventions
- Lowercase titles (no Title Case)
- transparent: true on all panels
- smooth line interpolation on timeseries
- palette-classic-by-name color mode on timeseries
- background color_mode on stat panels
- Use descriptive panel descriptions

## Grafana Units
bytes, Bps, s, ms, percent, percentunit, short, none, reqps, ops, pps, iops

## Query Patterns
- Counter: rate(metric[${rate_interval}]) or rate(metric[5m])
- Histogram: histogram_quantile(0.95, sum(rate(metric_bucket[${rate_interval}])) by (le))
- Percentage: (used / total) * 100
- Top-K: topk(10, metric)
- Aggregation: sum by (label)(metric), avg by (label)(metric)
`)

	// Add config context
	if len(ctx.Selectors) > 0 {
		sb.WriteString("\n## Available Selectors (use as ${name} in queries)\n")
		for name, val := range ctx.Selectors {
			sb.WriteString(fmt.Sprintf("- ${%s} = %s\n", name, val))
		}
	}

	if len(ctx.Constants) > 0 {
		sb.WriteString("\n## Available Constants (use as ${name} in queries)\n")
		for name, val := range ctx.Constants {
			sb.WriteString(fmt.Sprintf("- ${%s} = %s\n", name, val))
		}
	}

	if len(ctx.Thresholds) > 0 {
		sb.WriteString("\n## Available Thresholds (use as $name)\n")
		for _, name := range ctx.Thresholds {
			sb.WriteString(fmt.Sprintf("- $%s\n", name))
		}
	}

	if len(ctx.Palettes) > 0 {
		sb.WriteString("\n## Available Palette Colors (use as $name)\n")
		for _, name := range ctx.Palettes {
			sb.WriteString(fmt.Sprintf("- $%s\n", name))
		}
	}

	if len(ctx.Variables) > 0 {
		sb.WriteString("\n## Available Template Variables (use as $variable in queries)\n")
		for _, name := range ctx.Variables {
			sb.WriteString(fmt.Sprintf("- $%s\n", name))
		}
	}

	return sb.String()
}

func buildUserPrompt(metrics []MetricContext) string {
	var sb strings.Builder

	sb.WriteString("Generate a YAML section config with production-ready panels for these Prometheus metrics:\n\n")

	for _, m := range metrics {
		sb.WriteString(fmt.Sprintf("- **%s** (type: %s)", m.Name, m.Type))
		if m.Help != "" {
			sb.WriteString(fmt.Sprintf(" — %s", m.Help))
		}
		if len(m.Labels) > 0 {
			sb.WriteString(fmt.Sprintf(" [labels: %s]", strings.Join(m.Labels, ", ")))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nGroup related metrics into logical sections. Use appropriate panel types, units, thresholds, and queries. Include descriptions. Make the panels production-ready.")

	return sb.String()
}

func parseAIResponse(body []byte) (*AISuggestionResponse, error) {
	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("API error: %s: %s", resp.Error.Type, resp.Error.Message)
	}

	if len(resp.Content) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	// Concatenate all text content blocks
	var textParts []string
	for _, block := range resp.Content {
		if block.Type == "text" {
			textParts = append(textParts, block.Text)
		}
	}
	if len(textParts) == 0 {
		return nil, fmt.Errorf("no text content in API response")
	}
	text := strings.Join(textParts, "\n")

	// Extract YAML and notes
	yaml, notes := extractYAMLAndNotes(text)

	return &AISuggestionResponse{
		YAML:  yaml,
		Notes: notes,
	}, nil
}

// extractYAMLAndNotes separates the YAML content from any notes.
func extractYAMLAndNotes(text string) (string, []string) {
	// Strip markdown YAML fences if present
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```yaml") {
		text = strings.TrimPrefix(text, "```yaml")
		if idx := strings.LastIndex(text, "```"); idx >= 0 {
			text = text[:idx]
		}
		text = strings.TrimSpace(text)
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		if idx := strings.LastIndex(text, "```"); idx >= 0 {
			text = text[:idx]
		}
		text = strings.TrimSpace(text)
	}

	// Split YAML from notes
	var yamlLines []string
	var notes []string
	inNotes := false

	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "# Notes:") || strings.HasPrefix(line, "# notes:") {
			inNotes = true
			continue
		}
		if inNotes {
			note := strings.TrimPrefix(line, "# ")
			note = strings.TrimPrefix(note, "- ")
			note = strings.TrimSpace(note)
			if note != "" {
				notes = append(notes, note)
			}
		} else {
			yamlLines = append(yamlLines, line)
		}
	}

	return strings.TrimSpace(strings.Join(yamlLines, "\n")), notes
}
