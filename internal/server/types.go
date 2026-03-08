package server

import (
	"html/template"

	"github.com/wcatz/dashboard-generator/internal/config"
	"github.com/wcatz/dashboard-generator/internal/generator"
)

// DashboardConfig is a type alias for use in handler scope.
type DashboardConfig = config.DashboardConfig

// PanelInfo holds layout and detail info for a single panel, used by the visual preview.
type PanelInfo struct {
	ID          int
	Title       string
	Type        string
	X, Y, W, H int
	Section     string
	SectionY    int
	Datasource  string
	Unit        string
	Description string
	Queries     []QueryInfo
	Thresholds  []ThresholdStep
	// Rendering hints for visual preview
	ColorMode  string  `json:",omitempty"` // stat: background/value, timeseries: palette-classic-by-name/fixed/thresholds
	GraphMode  string  `json:",omitempty"` // stat: none/area
	TextMode   string  `json:",omitempty"` // stat: value/value_and_name/name
	DrawStyle  string  `json:",omitempty"` // timeseries: line/bars/points
	FillOpacity int    `json:",omitempty"` // timeseries/bargauge fill
	GaugeMin   float64 `json:",omitempty"` // gauge/bargauge min
	GaugeMax   float64 `json:",omitempty"` // gauge/bargauge max
	StackMode  string  `json:",omitempty"` // timeseries: none/normal
	PieType    string  `json:",omitempty"` // piechart: donut/pie
	TextContent string `json:",omitempty"` // text panel content
}

// QueryInfo holds a single target's expression and legend.
type QueryInfo struct {
	Expr       string
	Legend     string
	Datasource string
	RefID      string
}

// VariableInfo holds summary info about a template variable for preview display.
type VariableInfo struct {
	Name         string
	Label        string   // Display label (optional, defaults to Name)
	Type         string
	Query        string
	Values       string
	SampleValues []string // Actual values for dropdowns
	Multi        bool
	IncludeAll   bool
	Datasource   string // datasource name for query variables
}
// PreviewDashboard holds a single dashboard's preview data for multi-dashboard rendering.
type PreviewDashboard struct {
	UID            string
	Title          string
	Size           int
	Panels         int
	JSON           string
	PanelInfos     []PanelInfo
	PanelInfosJSON template.JS
	Variables      []VariableInfo
}

// NavLink holds a parsed dashboard navigation link for the preview page.
type NavLink struct {
	Title   string
	UID     string
	Icon    string
	Tooltip string
}

// ThresholdStep holds a single threshold step for display.
type ThresholdStep struct {
	Color string
	Value string
}

type metricRow struct {
	Name string
	Type string
	Help string
}

type labelSummary struct {
	Name       string
	Values     []string
	Constant   bool // same value on all targets
	AllTargets bool // present on every target
}

type enrichedJob struct {
	generator.JobSummary
	Labels       []labelSummary
	LabelCount   int
	ConstCount   int
}
