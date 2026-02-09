# Grafana Dashboard Generator — Context

## What This Is

A fully generic, config-driven Python Grafana dashboard generator. Reads a YAML config, outputs interlinked Grafana-compatible JSON dashboards. Zero hardcoded domains, prefixes, or assumptions about what is being monitored. Works with any Prometheus-monitored infrastructure.

## Files

| File | Purpose |
|------|---------|
| `grafana-dashboard-generator.py` | Main generator (~1600 lines) |
| `example-config.yaml` | Reference config with 5 generic dashboards |

## Dependencies

- **Python 3.8+ stdlib**: `json`, `os`, `argparse`, `urllib.request`, `re`, `fnmatch`, `base64`, `collections.defaultdict`
- **PyYAML**: Only external dependency. Graceful error if missing.

---

## Architecture

### Classes

| Class | Line | Purpose |
|-------|------|---------|
| `IdGenerator` | ~39 | Auto-incrementing panel ID counter, reset per dashboard |
| `Config` | ~55 | YAML loading, `$ref` resolution (colors, thresholds, selectors, constants) |
| `LayoutEngine` | ~183 | Auto-positions panels: flow left-to-right, wrap at 24 grid units |
| `PanelFactory` | ~226 | Creates panel JSON dicts for all panel types |
| `MetricDiscovery` | ~895 | Queries Prometheus API for metrics, metadata, labels; two-datasource comparison |
| `DashboardBuilder` | ~1200 | Assembles complete dashboard JSON with nav links, variables, sections |

### Standalone Functions

| Function | Purpose |
|----------|---------|
| `push_to_grafana()` | POST dashboard JSON to Grafana API (basic auth or bearer token) |
| `write_dashboard()` | Write JSON to file, print stats, warn if >750KB |
| `parse_args()` | CLI argument parsing via `argparse` |
| `main()` | Orchestration: load config → discovery → build dashboards → write/push |

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

## CLI Flags

| Flag | Default | Purpose |
|------|---------|---------|
| `--config` | (required) | Path to YAML config |
| `--profile` | all dashboards | Generate only dashboards in named profile |
| `--output-dir` | config `generator.output_dir` or `.` | Override output directory |
| `--prometheus-url` | none | Prometheus URL for discovery (overrides first datasource URL) |
| `--grafana-url` | none | Grafana URL for `--push` |
| `--grafana-user` | none | Basic auth user for `--push` |
| `--grafana-pass` | none | Basic auth password for `--push` |
| `--grafana-token` | none | Bearer token for `--push` |
| `--discover-print` | false | Query Prometheus, print YAML snippets |
| `--dry-run` | false | Generate to memory only, print sizes |
| `--verbose` | false | Print panel type and title for each panel |
| `--push` | false | Push dashboards to Grafana API |

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

## Extension Points

### Adding a New Panel Type

1. Add default size to `DEFAULT_SIZES` dict
2. Add method to `PanelFactory` following the pattern: `def new_type(self, cfg, x, y) -> dict`
3. Add dispatch entry in `PanelFactory.from_config()` dispatch dict
4. Document type-specific config keys

### Adding a New Variable Type

1. Add handling in `DashboardBuilder.build_variable()` (check `vtype` cases at ~line 1290)

### Adding a New Config Section

1. Add getter to `Config` class
2. Consume in `main()` or `DashboardBuilder`

### Adding a New Datasource Type

1. Works automatically — `datasources` config just needs `type` and `uid`
2. Discovery only works with Prometheus API endpoints
3. For non-Prometheus discovery, extend `MetricDiscovery` with type-specific fetch methods

---

## Testing

```bash
# dry-run (no files written)
./grafana-dashboard-generator.py --config example-config.yaml --dry-run

# generate to specific directory
./grafana-dashboard-generator.py --config example-config.yaml --output-dir /tmp/dashboards

# verbose (shows every panel)
./grafana-dashboard-generator.py --config example-config.yaml --verbose --dry-run

# profile filtering
./grafana-dashboard-generator.py --config example-config.yaml --profile infra --dry-run

# metric discovery
./grafana-dashboard-generator.py --config example-config.yaml --discover-print --prometheus-url http://localhost:9090

# push to grafana
./grafana-dashboard-generator.py --config example-config.yaml --push --grafana-url http://localhost:3000 --grafana-user admin --grafana-pass secret

# validate output JSON
python3 -c "import json; json.load(open('output.json'))"
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
