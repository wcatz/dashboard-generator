# Grafana Dashboard Generator — Context

## What This Is

A config-driven Grafana dashboard generator. Reads a YAML config, outputs interlinked Grafana-compatible JSON dashboards. Zero hardcoded domains or assumptions about what is being monitored. Works with any Prometheus-monitored infrastructure.

**Two implementations** — Python original and Go rewrite (feature parity, structurally identical output).

## Files

| Path | Purpose |
|------|---------|
| `grafana-dashboard-generator.py` | Python generator (~1600 lines, original) |
| `example-config.yaml` | Reference config with 5 generic dashboards |
| `cmd/dashboard-generator/main.go` | Go CLI entry point (cobra) |
| `internal/config/config.go` | Go config loading, $ref resolution, YAML key ordering |
| `internal/generator/panel.go` | Go panel factory (14 types) |
| `internal/generator/layout.go` | Go layout engine (24-unit grid) |
| `internal/generator/dashboard.go` | Go dashboard builder (variables, sections, nav links) |
| `internal/generator/discovery.go` | Go metric discovery (Prometheus API) |
| `internal/generator/writer.go` | Go JSON output + Grafana API push |
| `internal/generator/helpers.go` | Go type extraction helpers |
| `internal/generator/idgen.go` | Go panel ID generator |
| `internal/server/server.go` | HTTP server with embedded FS, template rendering |
| `internal/server/routes.go` | Route registration (pages + API endpoints) |
| `internal/server/handlers.go` | Page and API handlers (generate, preview, metrics, etc.) |
| `web/embed.go` | `//go:embed` directive for templates + static assets |
| `web/templates/layout.html` | Base layout (sidebar nav, dark theme) |
| `web/templates/*.html` | Page templates (index, datasources, palettes, metrics, editor, preview) |
| `web/templates/partials/*.html` | HTMX partial response templates |
| `web/static/` | Pico CSS, HTMX, custom CSS (all embedded in binary) |
| `Makefile` | Build, test, lint targets |
| `.goreleaser.yaml` | Cross-platform release config |

## Go Dependencies

- `gopkg.in/yaml.v3` — YAML parsing
- `github.com/spf13/cobra` — CLI framework
- stdlib: `encoding/json`, `net/http`, `html/template`, `embed`

## Go CLI

```bash
# Generate dashboards from YAML config
./dashboard-generator generate --config example-config.yaml --dry-run --verbose

# Discover metrics from Prometheus
./dashboard-generator discover --config example-config.yaml --prometheus-url http://localhost:9090

# Generate and push to Grafana
./dashboard-generator push --config example-config.yaml --grafana-url http://localhost:3000 --grafana-token $TOKEN

# Start web UI
./dashboard-generator serve --config example-config.yaml --port 8080

# Build
make build

# Test
make test

# Lint
make lint
```

## Web UI

Single-binary web UI using Go templates + HTMX + Pico CSS. All assets embedded via `embed.FS`. No JavaScript framework, no Node.js, no build step.

### Pages

| Route | Page | Description |
|-------|------|-------------|
| `/` | Dashboard list | Stats overview, generate buttons, preview links |
| `/datasources` | Datasource manager | View datasources, test Prometheus connections |
| `/palettes` | Color palettes | View palette colors and threshold presets |
| `/metrics` | Metric browser | Browse/filter metrics from connected Prometheus |
| `/editor` | Config editor | Edit YAML config with save/reload/validate |
| `/preview` | JSON preview | Generate and view dashboard JSON |

### API Endpoints (HTMX)

| Route | Method | Description |
|-------|--------|-------------|
| `/api/generate` | POST | Generate dashboards to disk (optional `?dashboard=uid`) |
| `/api/datasource/test` | GET | Test Prometheus connection (`?name=ds_name`) |
| `/api/metrics/browse` | GET | Browse metrics (`?datasource=&filter=&type=`) |
| `/api/config/save` | POST | Save YAML config to disk |
| `/api/config/reload` | POST | Reload config from disk |
| `/api/config/validate` | POST | Validate YAML syntax |
| `/api/preview` | GET | Generate preview JSON (`?uid=dashboard_uid`) |

### Stack

- **Go `html/template`** — server-side rendering
- **HTMX** v2.0.4 (~50KB) — dynamic interactions
- **Pico CSS** v2 (~83KB) — classless dark theme
- **Custom CSS** — sidebar nav, stat cards, metric list, badges

---

## Architecture (Go)

| Package | File | Purpose |
|---------|------|---------|
| `config` | `config.go` | YAML loading, `$ref` resolution, palette, thresholds, datasources |
| `generator` | `idgen.go` | Auto-incrementing panel ID counter |
| `generator` | `layout.go` | 24-unit grid flow layout engine |
| `generator` | `panel.go` | Panel factory — 14 types, target building, threshold resolution |
| `generator` | `helpers.go` | Type-safe extraction from `map[string]interface{}` |
| `generator` | `dashboard.go` | Dashboard builder — variables, sections, nav links, full assembly |
| `generator` | `discovery.go` | Prometheus API queries, filtering, comparison, YAML snippets |
| `generator` | `writer.go` | JSON file output, Grafana API push |
| `server` | `server.go` | HTTP server, template rendering, config management |
| `server` | `routes.go` | Route registration (6 pages + 8 API endpoints) |
| `server` | `handlers.go` | Page handlers + HTMX API handlers |

### Python Classes → Go Equivalents

| Python | Go |
|--------|-----|
| `IdGenerator` | `generator.IDGenerator` |
| `Config` | `config.Config` |
| `LayoutEngine` | `generator.LayoutEngine` |
| `PanelFactory` | `generator.PanelFactory` |
| `MetricDiscovery` | `generator.MetricDiscovery` |
| `DashboardBuilder` | `generator.DashboardBuilder` |
| `push_to_grafana()` | `generator.PushToGrafana()` |
| `write_dashboard()` | `generator.WriteDashboard()` |

---

## Panel Types (14 total)

| Type Key | Grafana Type | Default Size | Factory Method |
|----------|-------------|-------------|----------------|
| `stat` | stat | 3×4 | `PanelFactory.stat()` |
| `gauge` | gauge | 3×4 | `PanelFactory.gauge()` |
| `timeseries` | timeseries | 12×7 | `PanelFactory.timeseries()` |
| `bargauge` | bargauge | 6×5 | `PanelFactory.bargauge()` |
| `heatmap` | heatmap | 12×8 | `PanelFactory.heatmap()` |
| `histogram` | histogram | 12×7 | `PanelFactory.histogram()` |
| `table` | table | 24×8 | `PanelFactory.table()` |
| `piechart` | piechart | 6×6 | `PanelFactory.piechart()` |
| `state-timeline` | state-timeline | 12×5 | `PanelFactory.state_timeline()` |
| `status-history` | status-history | 12×5 | `PanelFactory.status_history()` |
| `text` | text | 24×3 | `PanelFactory.text()` |
| `logs` | logs | 24×8 | `PanelFactory.logs()` |
| `row` | row | 24×1 | `PanelFactory.row()` |
| `comparison` | timeseries (mixed DS) | 12×8 | `PanelFactory.comparison()` |

Default sizes are in `DEFAULT_SIZES` dict (~line 218). Every panel method accepts `(cfg, x, y)` where cfg is the panel's YAML config dict and x/y come from the layout engine.

### Panel Config Keys (common to all types)

```yaml
type: timeseries          # required — dispatched by PanelFactory.from_config()
title: "my panel"         # panel title (lowercase convention)
query: 'promql_here'      # single-query shorthand
targets:                  # multi-query (overrides query)
  - expr: 'promql_1'
    legend: "{{label}}"
  - expr: 'promql_2'
    legend: "{{label}}"
    datasource: secondary # per-target datasource override
width: 12                 # grid width (default per type)
height: 7                 # grid height (default per type)
x: 0                      # explicit x position (bypasses auto-layout)
y: 5                      # explicit y position (bypasses auto-layout)
datasource: primary       # datasource name from config (default: first/default DS)
unit: bytes               # Grafana unit string
description: "help text"  # panel description
color: "$blue"            # color ref for stat/gauge base color
thresholds: $percent_usage  # threshold ref or inline list
transparent: true         # default true for all panels
overrides: []             # Grafana field overrides (passthrough)
value_mappings: []        # Grafana value mappings (passthrough)
data_links: []            # Grafana data links (passthrough)
repeat: "variable_name"   # panel repetition variable
calcs: ["lastNotNull"]    # reduce calculations
```

### Type-Specific Config Keys

**stat**: `color_mode` (background/value), `graph_mode` (none/area), `text_mode` (value_and_name/value/name)

**gauge**: `min`, `max`, `show_threshold_labels`, `show_threshold_markers`

**timeseries**: `fill_opacity`, `line_width`, `stack` (none/normal), `draw_style` (line/bars/points), `line_interpolation` (smooth/linear/stepBefore/stepAfter), `axis_label`, `legend_calcs`, `legend_mode` (list/table/hidden), `legend_placement` (bottom/right), `show_legend`, `color_mode` (palette-classic-by-name/thresholds/fixed)

**bargauge**: `min`, `max`, `display_mode` (gradient/lcd/basic), `orientation` (horizontal/vertical)

**heatmap**: `color_scheme` (Spectral/Blues/Greens/Turbo/RdYlGn), `color_scale` (exponential/linear), `cell_gap`, `calculate`, `decimals`, `y_unit`

**histogram**: `bucket_count`, `combine`, `fill_opacity`

**table**: `filterable`, `pagination`, `sort_by`, `transformations`

**piechart**: `pie_type` (donut/pie), `display_labels` (percent/name/value), `legend_calcs`, `legend_mode`, `legend_placement`

**state-timeline**: `fill_opacity`, `merge_values`, `row_height`, `show_value` (auto/always/never)

**status-history**: `fill_opacity`, `row_height`, `show_value`

**text**: `content` (markdown string), `mode` (markdown/html/code)

**logs**: `dedup` (none/exact/numbers/signature), `prettify`, `show_common_labels`, `show_labels`, `show_time`, `sort_order`, `wrap`

**comparison**: `datasources` (list of DS names, minimum 2), `metric`, `metric_type` (counter/gauge/histogram/summary), `legend`

---

## YAML Config Schema

### Top-Level Sections

| Section | Purpose |
|---------|---------|
| `generator` | Global: `schema_version`, `refresh`, `time_range`, `output_dir`, `editable`, `graph_tooltip`, `live_now`, `timezone` |
| `datasources` | Named datasources: `type`, `uid`, `url` (url for discovery only), `is_default` |
| `palettes` | Named color palettes (any number of named hex colors) |
| `active_palette` | Which palette `$color` refs resolve against |
| `thresholds` | Named threshold sets (list of `{color, value}`) |
| `selectors` | Named PromQL label selector strings |
| `variables` | Template variable definitions with chaining |
| `constants` | String constants for DRY expressions |
| `discovery` | Metric discovery: `enabled`, `sources`, `include_patterns`, `exclude_patterns`, `auto_panels` |
| `profiles` | Named dashboard subsets for selective generation |
| `dashboards` | Dashboard definitions with uid, title, filename, tags, icon, variables, sections |

### Reference Resolution System

The `Config` class resolves references in this priority order:

1. **CLI args** override config values
2. **`${name}`** — checks constants first, then selectors
3. **`$name`** — checks palette colors (via `resolve_color()`), then thresholds (via `resolve_thresholds()`)

Resolution happens in:
- `Config.resolve_ref(value)` — string interpolation for `${braced}` refs
- `Config.resolve_color(value)` — `$color_name` to hex
- `Config.resolve_thresholds(value)` — `$threshold_name` to list, also resolves colors within threshold steps

### Variable Types

| Type | Config Keys | Grafana Behavior |
|------|------------|-----------------|
| `query` | `datasource`, `query`, `multi`, `include_all`, `refresh`, `sort`, `regex`, `all_value`, `default`, `chains_from` | Prometheus `label_values()` query |
| `custom` | `values` (comma-separated string) | Static value list |
| `datasource` | `ds_type` (e.g., "prometheus") | Datasource picker dropdown |
| `interval` | `values`, `auto`, `auto_count`, `auto_min` | Time interval selector |

### Dashboard Structure

```yaml
dashboards:
  my_dashboard:
    uid: unique-id           # Grafana dashboard UID (used in URL /d/uid)
    title: dashboard title   # displayed title
    filename: output.json    # output filename
    tags: [tag1, tag2]       # Grafana tags
    icon: apps               # Grafana icon for nav link (apps/database/bolt/cloud/exchange-alt/gf-grid)
    description: "text"      # tooltip in nav links
    variables: [var1, var2]  # list of variable names from top-level variables section
    sections:                # list of row sections
      - title: section name
        collapsed: false     # collapsed row (panels nested inside)
        repeat: var_name     # repeat row per variable value
        panels:              # list of panel configs
          - type: stat
            title: my stat
            query: 'up'
            # ... panel keys
```

---

## Layout Engine

Flow algorithm in `LayoutEngine`:

- Grid is 24 units wide
- Panels flow left-to-right: `cursor_x += width`
- When `cursor_x + width > 24`: wrap to next line (`cursor_y += row_height`, `cursor_x = 0`)
- Row panels (`add_row()`) always force a new line and take 1 unit of height
- `finish_section()` advances past the tallest panel in the current line
- Explicit `x`, `y` in panel config bypasses auto-placement

Collapsed sections use a separate inner `LayoutEngine` instance — panels are positioned relative to the row, then nested inside it.

---

## Metric Discovery

### Prometheus API Endpoints

| Endpoint | Method | Returns |
|----------|--------|---------|
| `/api/v1/label/__name__/values` | `fetch_metrics()` | Set of all metric names |
| `/api/v1/metadata` | `fetch_metadata()` | Dict: metric → {type, help} |
| `/api/v1/labels` | `fetch_labels()` | List of all label names |
| `/api/v1/label/{name}/values` | `fetch_label_values()` | List of values for a label |

### Two-Datasource Comparison

`MetricDiscovery.categorize(ds_a, ds_b)` returns:
```python
{
    "shared": {metric: {type, help}, ...},   # present on both
    "only_a": {metric: {type, help}, ...},   # only on datasource A
    "only_b": {metric: {type, help}, ...},   # only on datasource B
}
```

### Auto Panel Type Mapping

| Prometheus Type | Suggested Panel | Query Transform |
|----------------|----------------|----------------|
| counter | timeseries | `rate(metric[5m])` |
| gauge | stat | `metric` |
| histogram | heatmap | `metric` |
| summary | timeseries | `metric` |
| untyped | timeseries | `metric` |

### Discovery Modes

1. **`--discover-print`**: Queries Prometheus, groups by prefix, prints YAML snippets to stdout
2. **`discovery.enabled: true`** in config: `generate_discovery_sections()` appends auto-discovered sections to dashboards during generation

### Filtering

`filter_metrics()` uses `fnmatch` glob patterns:
- `include_patterns`: metrics must match at least one (default `["*"]`)
- `exclude_patterns`: metrics matching any pattern are excluded

`group_by_prefix()` splits on `_` and groups by first two segments (e.g., `node_cpu` for `node_cpu_seconds_total`).

---

## Navigation Links

Auto-generated by `DashboardBuilder.build_navigation_links()` from all dashboards in the generation set. Every dashboard gets the full link list in its `links` array.

```json
{
  "title": "from config title",
  "type": "link",
  "url": "/d/{uid}",
  "icon": "from config icon field",
  "targetBlank": false,
  "keepTime": true,
  "includeVars": true,
  "tooltip": "from config description"
}
```

When using `--profile`, only dashboards in the profile get links to each other.

---

## CLI Commands (Go)

| Command | Flags | Purpose |
|---------|-------|---------|
| `generate` | `--config`, `--profile`, `--output-dir`, `--dry-run`, `--verbose` | Generate dashboard JSON |
| `discover` | `--config`, `--prometheus-url` | Query Prometheus, print YAML snippets |
| `push` | `--config`, `--profile`, `--output-dir`, `--grafana-url`, `--grafana-user`, `--grafana-pass`, `--grafana-token`, `--verbose` | Generate and push to Grafana |
| `serve` | `--config`, `--port` (default 8080) | Start web UI server |

### Python CLI Flags (original)

| Flag | Purpose |
|------|---------|
| `--config` | Path to YAML config |
| `--profile` | Named profile filter |
| `--output-dir` | Override output directory |
| `--prometheus-url` | Prometheus URL for discovery |
| `--grafana-url/user/pass/token` | Grafana push auth |
| `--discover-print` | Print metric YAML snippets |
| `--dry-run` | Generate to memory only |
| `--verbose` | Print panel details |
| `--push` | Push to Grafana API |

---

## Grafana Compatibility

- **Schema version**: 39 (configurable via `generator.schema_version`)
- **Plugin version**: `11.2.0` hardcoded in all panels
- **Format**: Classic Grafana JSON with `panels` array (NOT v2beta1 scene format)
- **ConfigMap limit**: Warns at >750KB per dashboard JSON
- **Datasource refs**: `{"type": "...", "uid": "..."}` — no `${DS_PROMETHEUS}` variables
- **Mixed datasource**: `{"type": "datasource", "uid": "-- Mixed --"}` for comparison panels
- **Annotations**: Built-in Grafana annotation list included automatically
- **Variable queries**: `{"query": "...", "refId": "StandardVariableQuery"}` format

---

## Output Pipeline

```
main()
  → Config.load(yaml)
  → [optional] MetricDiscovery.print_discovery() for --discover-print
  → config.get_dashboards(profile)
  → DashboardBuilder.build_navigation_links(all_dashboards)
  → [optional] MetricDiscovery.generate_discovery_sections() if discovery.enabled
  → for each dashboard:
      → IdGenerator.reset()
      → LayoutEngine.reset()
      → DashboardBuilder.build_variables(var_names)
      → for each section:
          → LayoutEngine.add_row()
          → for each panel:
              → PanelFactory.from_config(panel_cfg, x, y) or .comparison()
              → LayoutEngine.place(w, h) for auto-positioning
      → DashboardBuilder.build() assembles full dashboard dict
      → write_dashboard() writes JSON + prints stats
      → [optional] push_to_grafana()
```

---

## Extension Points (Go)

### Adding a New Panel Type

1. Add default size to `DefaultSizes` in `panel.go`
2. Add method to `PanelFactory` following the pattern: `func (pf *PanelFactory) NewType(cfg map[string]interface{}, x, y int) map[string]interface{}`
3. Add case in `PanelFactory.FromConfig()` switch
4. Add test in `panel_test.go`

### Adding a New Variable Type

1. Add case in `DashboardBuilder.BuildVariable()` switch in `dashboard.go`

### Adding a New Config Section

1. Add field to `Config` struct in `config.go`
2. Consume in `cmd/dashboard-generator/main.go` or `DashboardBuilder`

### Adding a New Datasource Type

1. Works automatically — `datasources` config just needs `type` and `uid`
2. Discovery only works with Prometheus API endpoints

---

## Testing

```bash
# Go tests
make test

# Go dry-run
make run-dry

# Go generate
./dashboard-generator generate --config example-config.yaml --dry-run --verbose

# Go discover
./dashboard-generator discover --config example-config.yaml --prometheus-url http://localhost:9090

# Go push
./dashboard-generator push --config example-config.yaml --grafana-url http://localhost:3000 --grafana-token $TOKEN

# Go web UI
./dashboard-generator serve --config example-config.yaml --port 8080

# Python (original)
./grafana-dashboard-generator.py --config example-config.yaml --dry-run
./grafana-dashboard-generator.py --config example-config.yaml --discover-print --prometheus-url http://localhost:9090

# Validate JSON
python3 -c "import json; json.load(open('gen-overview.json'))"
```

---

## Known Limitations

- `pluginVersion` is hardcoded to `"11.2.0"` in all panels — should be configurable or auto-detected
- No Loki-specific query support in discovery (only Prometheus API)
- `comparison` panel type only generates timeseries — could support other visualizations for shared gauges
- No `alertlist` or `dashboardlist` panel types yet
- No annotation query support beyond the built-in Grafana annotation
- Variable `default` values only work for non-`includeAll` variables
- No row-level `repeat` tested with collapsed sections (may need gridPos adjustment)
- `transformations` passthrough on table panels — no config abstraction for common transforms
- Discovery `group_by_prefix` uses first two `_`-delimited segments which may not always produce good groupings

---

## Style Conventions

- Lowercase titles throughout (no Title Case, no ALL CAPS)
- No emojis or decorators in titles
- Short panel titles — detail goes in `description` field
- `transparent: true` on all panels by default
- `smooth` line interpolation on all timeseries by default
- `palette-classic-by-name` color mode on timeseries by default
- `background` color mode on stat panels by default
