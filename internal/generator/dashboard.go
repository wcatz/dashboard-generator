package generator

import (
	"fmt"

	"github.com/wcatz/dashboard-generator/internal/config"
)

// DashboardBuilder assembles complete Grafana dashboard JSON.
type DashboardBuilder struct {
	Config  *config.Config
	Factory *PanelFactory
	Layout  *LayoutEngine
}

// NewDashboardBuilder creates a new dashboard builder.
func NewDashboardBuilder(cfg *config.Config, factory *PanelFactory, layout *LayoutEngine) *DashboardBuilder {
	return &DashboardBuilder{Config: cfg, Factory: factory, Layout: layout}
}

// BuildNavigationLinks creates nav link objects for all dashboards.
func (db *DashboardBuilder) BuildNavigationLinks(dashboards map[string]config.DashboardConfig, order []string) []interface{} {
	var links []interface{}
	for _, name := range order {
		dbCfg, ok := dashboards[name]
		if !ok {
			continue
		}
		links = append(links, map[string]interface{}{
			"title":       dbCfg.Title,
			"type":        "link",
			"url":         fmt.Sprintf("/d/%s", dbCfg.UID),
			"icon":        defaultStr(dbCfg.Icon, "apps"),
			"targetBlank": false,
			"keepTime":    true,
			"includeVars": true,
			"tooltip":     dbCfg.Description,
		})
	}
	return links
}

// BuildVariable creates a Grafana variable dict.
func (db *DashboardBuilder) BuildVariable(name string) (map[string]interface{}, error) {
	v, ok := db.Config.GetVariableDef(name)
	if !ok {
		return nil, fmt.Errorf("variable '%s' not defined in config", name)
	}

	vtype := v.Type
	if vtype == "" {
		vtype = "query"
	}
	query := db.Config.ResolveRef(v.Query)
	label := v.Label
	if label == "" {
		label = name
	}
	hide := v.Hide
	refresh := v.Refresh
	if refresh == 0 {
		refresh = 2
	}
	sort := v.Sort
	if sort == 0 {
		sort = 1
	}

	ds := db.Config.GetDefaultDatasource()
	if v.Datasource != "" {
		ref, err := db.Config.GetDatasource(v.Datasource)
		if err == nil {
			ds = ref
		}
	}

	current := map[string]interface{}{"selected": true, "text": "All", "value": "$__all"}
	if !v.IncludeAll {
		current = map[string]interface{}{
			"selected": true,
			"text":     v.Default.Text,
			"value":    v.Default.Value,
		}
	}

	varDef := map[string]interface{}{
		"current":    current,
		"datasource": map[string]interface{}{"type": ds.Type, "uid": ds.UID},
		"definition": query,
		"hide":       hide,
		"includeAll": v.IncludeAll,
		"label":      label,
		"multi":      v.Multi,
		"name":       name,
		"options":    []interface{}{},
		"query":      map[string]interface{}{"query": query, "refId": "StandardVariableQuery"},
		"refresh":    refresh,
		"regex":      v.Regex,
		"skipUrlSync": false,
		"sort":       sort,
		"type":       vtype,
	}

	if v.AllValue != "" {
		varDef["allValue"] = v.AllValue
	}

	switch vtype {
	case "custom":
		varDef["query"] = v.Values
		delete(varDef, "datasource")
		delete(varDef, "definition")
	case "datasource":
		dsType := v.DsType
		if dsType == "" {
			dsType = "prometheus"
		}
		varDef["query"] = dsType
		delete(varDef, "definition")
		delete(varDef, "datasource")
	case "interval":
		vals := v.Values
		if vals == "" {
			vals = "1m,5m,15m,30m,1h,6h,12h,1d"
		}
		varDef["query"] = vals
		varDef["auto"] = v.Auto
		autoCount := v.AutoCount
		if autoCount == 0 {
			autoCount = 10
		}
		varDef["auto_count"] = autoCount
		autoMin := v.AutoMin
		if autoMin == "" {
			autoMin = "10s"
		}
		varDef["auto_min"] = autoMin
		delete(varDef, "datasource")
		delete(varDef, "definition")
	}

	return varDef, nil
}

// BuildVariables creates variable dicts for a list of names.
func (db *DashboardBuilder) BuildVariables(varNames []string) ([]interface{}, error) {
	var vars []interface{}
	for _, name := range varNames {
		v, err := db.BuildVariable(name)
		if err != nil {
			return nil, err
		}
		vars = append(vars, v)
	}
	if vars == nil {
		vars = []interface{}{}
	}
	return vars, nil
}

// BuildSection processes a dashboard section and returns panels.
func (db *DashboardBuilder) BuildSection(section config.SectionConfig) ([]interface{}, error) {
	var panels []interface{}

	if section.Collapsed {
		innerLayout := NewLayoutEngine()
		var innerPanels []interface{}
		for _, pcfg := range section.Panels {
			ptype := getString(pcfg, "type", "")
			ds := DefaultSizes[ptype]
			if ds == [2]int{} {
				ds = [2]int{6, 4}
			}
			w := getInt(pcfg, "width", ds[0])
			h := getInt(pcfg, "height", ds[1])

			var px, py int
			if hasKey(pcfg, "x") && hasKey(pcfg, "y") {
				px = getInt(pcfg, "x", 0)
				py = getInt(pcfg, "y", 0)
			} else {
				px, py = innerLayout.Place(w, h)
			}

			panel, err := db.Factory.FromConfig(pcfg, px, py)
			if err != nil {
				return nil, fmt.Errorf("panel '%s': %w", getString(pcfg, "title", "?"), err)
			}
			innerPanels = append(innerPanels, panel)
		}

		rowY := db.Layout.AddRow()
		innerPanelIfaces := make([]interface{}, len(innerPanels))
		copy(innerPanelIfaces, innerPanels)
		panels = append(panels, db.Factory.Row(section.Title, rowY, true, innerPanelIfaces, section.Repeat))
	} else {
		rowY := db.Layout.AddRow()
		panels = append(panels, db.Factory.Row(section.Title, rowY, false, nil, section.Repeat))

		for _, pcfg := range section.Panels {
			ptype := getString(pcfg, "type", "")
			ds := DefaultSizes[ptype]
			if ds == [2]int{} {
				ds = [2]int{6, 4}
			}
			w := getInt(pcfg, "width", ds[0])
			h := getInt(pcfg, "height", ds[1])

			var px, py int
			if hasKey(pcfg, "x") && hasKey(pcfg, "y") {
				px = getInt(pcfg, "x", 0)
				py = getInt(pcfg, "y", 0)
			} else {
				px, py = db.Layout.Place(w, h)
			}

			panel, err := db.Factory.FromConfig(pcfg, px, py)
			if err != nil {
				return nil, fmt.Errorf("panel '%s': %w", getString(pcfg, "title", "?"), err)
			}
			panels = append(panels, panel)
		}

		db.Layout.FinishSection()
	}

	return panels, nil
}

// Build assembles a complete Grafana dashboard.
func (db *DashboardBuilder) Build(dbCfg config.DashboardConfig, navLinks []interface{}, discoverySections []config.SectionConfig) (map[string]interface{}, error) {
	db.Factory.IDGen.Reset()
	db.Layout.Reset()

	gen := db.Config.GetGenerator()

	variables, err := db.BuildVariables(dbCfg.Variables)
	if err != nil {
		return nil, err
	}

	var allPanels []interface{}
	for _, section := range dbCfg.Sections {
		panels, err := db.BuildSection(section)
		if err != nil {
			return nil, err
		}
		allPanels = append(allPanels, panels...)
	}

	for _, section := range discoverySections {
		panels, err := db.BuildSection(section)
		if err != nil {
			return nil, err
		}
		allPanels = append(allPanels, panels...)
	}

	if allPanels == nil {
		allPanels = []interface{}{}
	}

	editable := true
	if gen.Editable != nil {
		editable = *gen.Editable
	}
	liveNow := true
	if gen.LiveNow != nil {
		liveNow = *gen.LiveNow
	}
	refresh := gen.Refresh
	if refresh == "" {
		refresh = "30s"
	}
	schemaVersion := gen.SchemaVersion
	if schemaVersion == 0 {
		schemaVersion = 39
	}
	timeRange := gen.TimeRange
	if timeRange == nil {
		timeRange = map[string]string{"from": "now-30m", "to": "now"}
	}
	graphTooltip := gen.GraphTooltip
	if graphTooltip == 0 {
		graphTooltip = 1
	}

	if navLinks == nil {
		navLinks = []interface{}{}
	}

	return map[string]interface{}{
		"annotations": map[string]interface{}{
			"list": []interface{}{
				map[string]interface{}{
					"builtIn":    1,
					"datasource": map[string]interface{}{"type": "grafana", "uid": "-- Grafana --"},
					"enable":     true,
					"hide":       true,
					"iconColor":  "rgba(0, 211, 255, 1)",
					"name":       "Annotations & Alerts",
					"type":       "dashboard",
				},
			},
		},
		"description":          dbCfg.Description,
		"editable":             editable,
		"fiscalYearStartMonth": 0,
		"graphTooltip":         graphTooltip,
		"id":                   nil,
		"links":                navLinks,
		"liveNow":              liveNow,
		"panels":               allPanels,
		"refresh":              refresh,
		"schemaVersion":        schemaVersion,
		"tags":                 toInterfaceSlice(dbCfg.Tags),
		"templating":           map[string]interface{}{"list": variables},
		"time":                 timeRange,
		"timepicker": map[string]interface{}{
			"refresh_intervals": []interface{}{"5s", "10s", "30s", "1m", "5m", "15m", "30m"},
		},
		"timezone": gen.Timezone,
		"title":    dbCfg.Title,
		"uid":      dbCfg.UID,
		"version":  1,
	}, nil
}

func defaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func hasKey(m map[string]interface{}, key string) bool {
	_, ok := m[key]
	return ok
}

func toInterfaceSlice(ss []string) []interface{} {
	if ss == nil {
		return []interface{}{}
	}
	result := make([]interface{}, len(ss))
	for i, s := range ss {
		result[i] = s
	}
	return result
}
