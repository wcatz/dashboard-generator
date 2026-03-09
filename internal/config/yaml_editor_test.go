package config

import (
	"os"
	"strings"
	"testing"
)

func TestAppendSection(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		dashboard string
		section   string
		wantErr   string
		wantIn    string // substring expected in output
	}{
		{
			name: "append to existing sections",
			config: `dashboards:
  overview:
    uid: gen-overview
    title: overview
    sections:
      - title: existing
        panels:
          - type: stat
            title: up
            query: up
`,
			dashboard: "overview",
			section: `- title: "new section"
  panels:
    - type: timeseries
      title: cpu
      query: 'rate(cpu_total[5m])'
`,
			wantIn: "new section",
		},
		{
			name: "append to dashboard with no sections",
			config: `dashboards:
  overview:
    uid: gen-overview
    title: overview
`,
			dashboard: "overview",
			section: `- title: "first section"
  panels:
    - type: stat
      title: up
      query: up
`,
			wantIn: "first section",
		},
		{
			name: "error on missing dashboard",
			config: `dashboards:
  overview:
    uid: gen-overview
`,
			dashboard: "nonexistent",
			section:   `- title: test`,
			wantErr:   "not found",
		},
		{
			name:      "error on no dashboards section",
			config:    `generator:\n  schema_version: 39`,
			dashboard: "overview",
			section:   `- title: test`,
			wantErr:   "no dashboards section",
		},
		{
			name: "error on invalid section YAML",
			config: `dashboards:
  overview:
    uid: gen-overview
    sections: []
`,
			dashboard: "overview",
			section:   `[[[invalid`,
			wantErr:   "parsing section YAML",
		},
		{
			name: "error on empty section",
			config: `dashboards:
  overview:
    uid: gen-overview
    sections: []
`,
			dashboard: "overview",
			section:   ``,
			wantErr:   "no sections",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTestConfig(t, tt.config)
			editor := NewYAMLEditor(path)
			err := editor.AppendSection(tt.dashboard, []byte(tt.section))

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify the file was written and contains expected content
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading result: %v", err)
			}
			if !strings.Contains(string(data), tt.wantIn) {
				t.Errorf("expected output to contain %q, got:\n%s", tt.wantIn, string(data))
			}

			// Verify the result is valid config by loading it
			if _, loadErr := LoadFromBytes(data); loadErr != nil {
				t.Errorf("result is not valid config: %v", loadErr)
			}
		})
	}
}

func TestAddDashboard(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		dbName  string
		dbYAML  string
		wantErr string
		wantIn  string
	}{
		{
			name: "add to existing dashboards",
			config: `dashboards:
  overview:
    uid: gen-overview
    title: overview
`,
			dbName: "comparison",
			dbYAML: `  uid: gen-comparison
  title: comparison
  filename: gen-comparison.json
  tags: [comparison]
  sections:
    - title: shared metrics
      panels:
        - type: comparison
          title: cpu
          metric: node_cpu_seconds_total
`,
			wantIn: "gen-comparison",
		},
		{
			name:   "add when no dashboards section",
			config: `generator:\n  schema_version: 39`,
			dbName: "first",
			dbYAML: `  uid: gen-first
  title: first dashboard
`,
			wantIn: "gen-first",
		},
		{
			name: "error on duplicate",
			config: `dashboards:
  overview:
    uid: gen-overview
`,
			dbName:  "overview",
			dbYAML:  `  uid: gen-dupe`,
			wantErr: "already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTestConfig(t, tt.config)
			editor := NewYAMLEditor(path)
			err := editor.AddDashboard(tt.dbName, []byte(tt.dbYAML))

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading result: %v", err)
			}
			if !strings.Contains(string(data), tt.wantIn) {
				t.Errorf("expected output to contain %q, got:\n%s", tt.wantIn, string(data))
			}
		})
	}
}

func TestAppendPanel(t *testing.T) {
	cfg := `dashboards:
  overview:
    uid: gen-overview
    title: overview
    sections:
      - title: health
        panels:
          - type: stat
            title: up
            query: up
`
	panel := `- type: timeseries
  title: cpu
  query: 'rate(cpu_total[5m])'
`
	path := writeTestConfig(t, cfg)
	editor := NewYAMLEditor(path)
	if err := editor.AppendPanel("overview", 0, []byte(panel)); err != nil {
		t.Fatalf("AppendPanel: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "cpu") {
		t.Error("new panel not found")
	}
	if !strings.Contains(content, "up") {
		t.Error("existing panel lost")
	}

	// Test out of range
	err := editor.AppendPanel("overview", 5, []byte(panel))
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Errorf("expected out of range error, got: %v", err)
	}

	// Test missing dashboard
	err = editor.AppendPanel("nonexistent", 0, []byte(panel))
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got: %v", err)
	}
}

func TestAppendSectionPreservesExisting(t *testing.T) {
	config := `dashboards:
  overview:
    uid: gen-overview
    title: overview
    sections:
      - title: existing section
        panels:
          - type: stat
            title: targets up
            query: 'count(up == 1)'
`
	section := `- title: "added section"
  panels:
    - type: timeseries
      title: cpu rate
      query: 'rate(node_cpu_seconds_total[5m])'
`
	path := writeTestConfig(t, config)
	editor := NewYAMLEditor(path)
	if err := editor.AppendSection("overview", []byte(section)); err != nil {
		t.Fatalf("AppendSection: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	// Both old and new sections should be present
	if !strings.Contains(content, "existing section") {
		t.Error("existing section was lost")
	}
	if !strings.Contains(content, "added section") {
		t.Error("new section was not added")
	}
	if !strings.Contains(content, "targets up") {
		t.Error("existing panel was lost")
	}
	if !strings.Contains(content, "cpu rate") {
		t.Error("new panel was not added")
	}
}
