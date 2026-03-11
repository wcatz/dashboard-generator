package server

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func (s *Server) handleImportPage(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config()

	var dsNames []string
	for name := range cfg.Datasources {
		dsNames = append(dsNames, name)
	}
	sort.Strings(dsNames)

	dashboards, _ := cfg.GetDashboardOrder("")

	s.renderPage(w, "import.html", map[string]interface{}{
		"Title":        "import",
		"Active":       "import",
		"ConfigPath":   s.ConfigPath(),
		"ActiveConfig": s.ActiveConfigName(),
		"ConfigDir":    s.ConfigDir(),
		"GrafanaURL":   s.GrafanaURL(),
		"Datasources":  dsNames,
		"Dashboards":   dashboards,
	})
}

// handleImportParse parses Grafana panel JSON and returns config YAML.
func (s *Server) handleImportParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	panelJSON := r.FormValue("panel_json")
	if panelJSON == "" {
		s.renderPartial(w, "import-result.html", map[string]interface{}{"Error": "paste panel JSON"})
		return
	}

	var panel map[string]interface{}
	if err := json.Unmarshal([]byte(panelJSON), &panel); err != nil {
		s.renderPartial(w, "import-result.html", map[string]interface{}{"Error": "invalid JSON: " + err.Error()})
		return
	}

	// Parse using existing infrastructure
	info := parsePanelJSON(panel, "", 0)
	if info.Type == "" {
		s.renderPartial(w, "import-result.html", map[string]interface{}{"Error": "no panel type found in JSON"})
		return
	}

	// Build datasource UID-to-name auto-mapping
	cfg := s.Config()
	uidToName := make(map[string]string)
	for name, ds := range cfg.Datasources {
		uidToName[ds.UID] = name
	}

	// Generate config YAML from parsed info
	yamlStr := reversePanelToYAML(info, uidToName)

	s.renderPartial(w, "import-result.html", map[string]interface{}{
		"Panel": info,
		"YAML":  yamlStr,
	})
}

// reversePanelToYAML converts a PanelInfo back to config YAML string.
// Uses yaml.Marshal on a structured map to avoid YAML injection from untrusted values.
func reversePanelToYAML(info PanelInfo, uidToName map[string]string) string {
	panel := yaml.Node{Kind: yaml.MappingNode}
	addField := func(key string, value interface{}) {
		panel.Content = append(panel.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			marshalValueNode(value),
		)
	}

	addField("type", info.Type)
	if info.Title != "" {
		addField("title", info.Title)
	}

	// Queries
	if len(info.Queries) == 1 {
		q := info.Queries[0]
		if q.Expr != "" {
			addField("query", q.Expr)
		}
		if q.Legend != "" {
			addField("legend", q.Legend)
		}
	} else if len(info.Queries) > 1 {
		var targets []map[string]string
		for _, q := range info.Queries {
			t := map[string]string{"query": q.Expr}
			if q.Legend != "" {
				t["legend"] = q.Legend
			}
			ds := resolveDatasource(q.Datasource, uidToName)
			if ds != "" {
				t["datasource"] = ds
			}
			targets = append(targets, t)
		}
		addField("targets", targets)
	}

	// Datasource
	ds := resolveDatasource(info.Datasource, uidToName)
	if ds != "" {
		addField("datasource", ds)
	}

	// Unit
	if info.Unit != "" && info.Unit != "none" {
		addField("unit", info.Unit)
	}

	// Description
	if info.Description != "" {
		addField("description", info.Description)
	}

	// Size
	if info.W > 0 {
		addField("width", info.W)
	}
	if info.H > 0 {
		addField("height", info.H)
	}

	// Type-specific options
	if info.ColorMode != "" {
		addField("color_mode", info.ColorMode)
	}
	if info.DrawStyle != "" {
		addField("draw_style", info.DrawStyle)
	}
	if info.FillOpacity > 0 {
		addField("fill_opacity", info.FillOpacity)
	}
	if info.StackMode != "" && info.StackMode != "none" {
		addField("stack", info.StackMode)
	}
	if info.GraphMode != "" {
		addField("graph_mode", info.GraphMode)
	}
	if info.TextMode != "" {
		addField("text_mode", info.TextMode)
	}
	if info.PieType != "" {
		addField("pie_type", info.PieType)
	}
	if info.GaugeMin != 0 {
		addField("min", info.GaugeMin)
	}
	if info.GaugeMax != 0 {
		addField("max", info.GaugeMax)
	}
	if info.TextContent != "" {
		addField("content", info.TextContent)
	}

	// Wrap in a sequence node to produce "- type: ..." format
	seq := yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{&panel}}
	out, err := yaml.Marshal(&seq)
	if err != nil {
		return "# error marshaling panel YAML"
	}
	return strings.TrimRight(string(out), "\n")
}

// marshalValueNode converts a Go value to a yaml.Node.
func marshalValueNode(v interface{}) *yaml.Node {
	var node yaml.Node
	if err := node.Encode(v); err != nil {
		return &yaml.Node{Kind: yaml.ScalarNode, Value: ""}
	}
	return &node
}

func resolveDatasource(uid string, uidToName map[string]string) string {
	if uid == "" {
		return ""
	}
	if name, ok := uidToName[uid]; ok {
		return name
	}
	return uid
}
