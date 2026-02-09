package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var bracedRefRe = regexp.MustCompile(`\$\{(\w+)\}`)

// DatasourceDef is a datasource definition from config YAML.
type DatasourceDef struct {
	Type      string `yaml:"type"`
	UID       string `yaml:"uid"`
	URL       string `yaml:"url"`
	IsDefault bool   `yaml:"is_default"`
}

// DatasourceRef is a Grafana datasource reference used in panels.
type DatasourceRef struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

// ThresholdStep is a single step in a threshold definition.
type ThresholdStep struct {
	Color string      `yaml:"color" json:"color"`
	Value interface{} `yaml:"value" json:"value"`
}

// VariableDef is a template variable definition.
type VariableDef struct {
	Type       string   `yaml:"type"`
	Datasource string   `yaml:"datasource"`
	Query      string   `yaml:"query"`
	Multi      bool     `yaml:"multi"`
	IncludeAll bool     `yaml:"include_all"`
	Refresh    int      `yaml:"refresh"`
	Sort       int      `yaml:"sort"`
	Label      string   `yaml:"label"`
	Hide       int      `yaml:"hide"`
	Regex      string   `yaml:"regex"`
	AllValue   string   `yaml:"all_value"`
	ChainsFrom []string `yaml:"chains_from"`
	Values     string   `yaml:"values"`
	DsType     string   `yaml:"ds_type"`
	Auto       bool     `yaml:"auto"`
	AutoCount  int      `yaml:"auto_count"`
	AutoMin    string   `yaml:"auto_min"`
	Default    struct {
		Text  string `yaml:"text"`
		Value string `yaml:"value"`
	} `yaml:"default"`
}

// GeneratorSettings holds global generator config.
type GeneratorSettings struct {
	SchemaVersion int               `yaml:"schema_version"`
	OutputDir     string            `yaml:"output_dir"`
	Refresh       string            `yaml:"refresh"`
	TimeRange     map[string]string `yaml:"time_range"`
	Editable      *bool             `yaml:"editable"`
	GraphTooltip  int               `yaml:"graph_tooltip"`
	LiveNow       *bool             `yaml:"live_now"`
	Timezone      string            `yaml:"timezone"`
}

// DiscoveryConfig holds metric discovery settings.
type DiscoveryConfig struct {
	Enabled         bool     `yaml:"enabled"`
	Sources         []string `yaml:"sources"`
	IncludePatterns []string `yaml:"include_patterns"`
	ExcludePatterns []string `yaml:"exclude_patterns"`
	AutoPanels      map[string]string `yaml:"auto_panels"`
}

// ProfileDef is a named dashboard subset.
type ProfileDef struct {
	Dashboards []string `yaml:"dashboards"`
}

// SectionConfig is a dashboard section with panels.
type SectionConfig struct {
	Title     string                   `yaml:"title"`
	Collapsed bool                     `yaml:"collapsed"`
	Repeat    string                   `yaml:"repeat"`
	Panels    []map[string]interface{} `yaml:"panels"`
}

// DashboardConfig is a single dashboard definition.
type DashboardConfig struct {
	UID         string          `yaml:"uid"`
	Title       string          `yaml:"title"`
	Filename    string          `yaml:"filename"`
	Tags        []string        `yaml:"tags"`
	Icon        string          `yaml:"icon"`
	Description string          `yaml:"description"`
	Variables   []string        `yaml:"variables"`
	Sections    []SectionConfig `yaml:"sections"`
}

// Config holds the entire YAML configuration.
type Config struct {
	Generator   GeneratorSettings          `yaml:"generator"`
	Datasources map[string]DatasourceDef   `yaml:"datasources"`
	Palettes    map[string]map[string]string `yaml:"palettes"`
	ActivePalette string                   `yaml:"active_palette"`
	Thresholds  map[string][]ThresholdStep `yaml:"thresholds"`
	Selectors   map[string]string          `yaml:"selectors"`
	Variables   map[string]VariableDef     `yaml:"variables"`
	Constants   map[string]string          `yaml:"constants"`
	Discovery   DiscoveryConfig            `yaml:"discovery"`
	Profiles    map[string]ProfileDef      `yaml:"profiles"`
	Dashboards  map[string]DashboardConfig `yaml:"dashboards"`

	palette        map[string]string
	cliArgs        map[string]string
	dashboardOrder []string
}

// Load reads and parses a YAML config file.
func Load(path string, cliArgs map[string]string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	c, err := loadFromData(data, cliArgs)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// LoadFromBytes parses a YAML config from raw bytes (for validation).
func LoadFromBytes(data []byte) (*Config, error) {
	return loadFromData(data, nil)
}

func loadFromData(data []byte, cliArgs map[string]string) (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	c.dashboardOrder = parseDashboardKeyOrder(data)

	c.cliArgs = cliArgs
	if c.cliArgs == nil {
		c.cliArgs = make(map[string]string)
	}
	c.palette = c.resolvePalette()

	return &c, nil
}

func (c *Config) resolvePalette() map[string]string {
	if c.Palettes == nil {
		return map[string]string{}
	}
	p, ok := c.Palettes[c.ActivePalette]
	if !ok {
		return map[string]string{}
	}
	return p
}

// GetGenerator returns generator settings.
func (c *Config) GetGenerator() GeneratorSettings {
	return c.Generator
}

// GetDatasource returns a DatasourceRef for a named datasource.
func (c *Config) GetDatasource(name string) (DatasourceRef, error) {
	ds, ok := c.Datasources[name]
	if !ok {
		return DatasourceRef{}, fmt.Errorf("datasource '%s' not defined in config", name)
	}
	return DatasourceRef{Type: ds.Type, UID: ds.UID}, nil
}

// GetDatasourceURL returns the URL for a named datasource.
func (c *Config) GetDatasourceURL(name string) string {
	ds, ok := c.Datasources[name]
	if !ok {
		return ""
	}
	url := ds.URL
	if promURL, ok := c.cliArgs["prometheus_url"]; ok && name == c.firstDSName() {
		url = promURL
	}
	return url
}

func (c *Config) firstDSName() string {
	// Go maps don't preserve order, but for the CLI override use case
	// we iterate to find the first key. For determinism we check is_default first.
	for name, ds := range c.Datasources {
		if ds.IsDefault {
			return name
		}
	}
	for name := range c.Datasources {
		return name
	}
	return ""
}

// GetDefaultDatasource returns the default DatasourceRef.
func (c *Config) GetDefaultDatasource() DatasourceRef {
	for _, ds := range c.Datasources {
		if ds.IsDefault {
			return DatasourceRef{Type: ds.Type, UID: ds.UID}
		}
	}
	// fallback to first
	for _, ds := range c.Datasources {
		return DatasourceRef{Type: ds.Type, UID: ds.UID}
	}
	return DatasourceRef{Type: "prometheus", UID: "prometheus"}
}

// GetThresholds returns resolved threshold steps for a named threshold.
func (c *Config) GetThresholds(name string) []ThresholdStep {
	t, ok := c.Thresholds[name]
	if !ok {
		return nil
	}
	resolved := make([]ThresholdStep, len(t))
	for i, step := range t {
		resolved[i] = step
		if strings.HasPrefix(step.Color, "$") {
			resolved[i].Color = c.resolveColorName(step.Color[1:])
		}
	}
	return resolved
}

// GetSelector returns a named selector string.
func (c *Config) GetSelector(name string) string {
	return c.Selectors[name]
}

// GetConstant returns a named constant string.
func (c *Config) GetConstant(name string) string {
	return c.Constants[name]
}

// GetVariableDef returns a variable definition by name.
func (c *Config) GetVariableDef(name string) (VariableDef, bool) {
	v, ok := c.Variables[name]
	return v, ok
}

// GetDashboards returns dashboards, optionally filtered by profile.
func (c *Config) GetDashboards(profile string) (map[string]DashboardConfig, error) {
	if profile == "" {
		return c.Dashboards, nil
	}
	p, ok := c.Profiles[profile]
	if !ok {
		return nil, fmt.Errorf("profile '%s' not defined in config", profile)
	}
	filtered := make(map[string]DashboardConfig)
	nameSet := make(map[string]bool)
	for _, n := range p.Dashboards {
		nameSet[n] = true
	}
	for k, v := range c.Dashboards {
		if nameSet[k] {
			filtered[k] = v
		}
	}
	return filtered, nil
}

// GetDashboardOrder returns dashboard names in the order they appear in a profile,
// or all dashboard names if no profile is specified.
func (c *Config) GetDashboardOrder(profile string) ([]string, error) {
	if profile != "" {
		p, ok := c.Profiles[profile]
		if !ok {
			return nil, fmt.Errorf("profile '%s' not defined in config", profile)
		}
		return p.Dashboards, nil
	}
	// Without a profile, we need to preserve YAML order.
	// Since Go maps don't preserve order, we re-parse to get ordered keys.
	return c.parseDashboardOrder()
}

func (c *Config) parseDashboardOrder() ([]string, error) {
	if len(c.dashboardOrder) > 0 {
		return c.dashboardOrder, nil
	}
	// Fallback to sorted keys
	keys := make([]string, 0, len(c.Dashboards))
	for k := range c.Dashboards {
		keys = append(keys, k)
	}
	return keys, nil
}

// parseDashboardKeyOrder extracts dashboard key ordering from raw YAML.
func parseDashboardKeyOrder(data []byte) []string {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil
	}
	if node.Kind != yaml.DocumentNode || len(node.Content) == 0 {
		return nil
	}
	root := node.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	// Find "dashboards" key
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == "dashboards" {
			dbNode := root.Content[i+1]
			if dbNode.Kind != yaml.MappingNode {
				return nil
			}
			var order []string
			for j := 0; j < len(dbNode.Content)-1; j += 2 {
				order = append(order, dbNode.Content[j].Value)
			}
			return order
		}
	}
	return nil
}

// GetDiscovery returns the discovery config.
func (c *Config) GetDiscovery() DiscoveryConfig {
	return c.Discovery
}

func (c *Config) resolveColorName(name string) string {
	if hex, ok := c.palette[name]; ok {
		return hex
	}
	return name
}

// ResolveRef resolves ${name} references in a string (constants and selectors).
func (c *Config) ResolveRef(value string) string {
	return bracedRefRe.ReplaceAllStringFunc(value, func(match string) string {
		refName := bracedRefRe.FindStringSubmatch(match)[1]
		if v := c.GetConstant(refName); v != "" {
			return v
		}
		if v := c.GetSelector(refName); v != "" {
			return v
		}
		return match
	})
}

// ResolveColor resolves a $color_name reference to a hex color.
func (c *Config) ResolveColor(value string) string {
	if strings.HasPrefix(value, "$") {
		return c.resolveColorName(value[1:])
	}
	return value
}

// ResolveThresholds resolves a $threshold_name or inline threshold list.
func (c *Config) ResolveThresholds(value interface{}) []ThresholdStep {
	switch v := value.(type) {
	case string:
		if strings.HasPrefix(v, "$") {
			return c.GetThresholds(v[1:])
		}
	case []interface{}:
		steps := make([]ThresholdStep, 0, len(v))
		for _, item := range v {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			step := ThresholdStep{}
			if color, ok := m["color"].(string); ok {
				if strings.HasPrefix(color, "$") {
					step.Color = c.resolveColorName(color[1:])
				} else {
					step.Color = color
				}
			}
			step.Value = m["value"]
			steps = append(steps, step)
		}
		return steps
	}
	return nil
}
