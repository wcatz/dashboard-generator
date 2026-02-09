# dashboard-generator

Config-driven Grafana dashboard generator. Write YAML, get interlinked JSON dashboards.

## Why

Hand-editing Grafana JSON is painful. Clicking through the UI doesn't scale. This tool lets you define dashboards declaratively in YAML and generates production-ready Grafana JSON with automatic navigation links, layout, and consistent styling.

**One config file. Any number of dashboards. Zero clicking.**

## Features

- **14 panel types**: stat, gauge, timeseries, bargauge, heatmap, histogram, table, piechart, state-timeline, status-history, text, logs, row, comparison
- **Auto-layout engine**: panels flow left-to-right across a 24-unit grid, wrapping automatically
- **Navigation links**: every dashboard links to every other dashboard in the set
- **Reference system**: reusable colors (`$green`), thresholds (`$percent_usage`), selectors (`${by_ns}`), and constants (`${rate_interval}`)
- **Template variables**: query, custom, datasource, and interval types with chaining support
- **Metric discovery**: query Prometheus and get suggested YAML config snippets
- **Two-datasource comparison**: compare metrics across Prometheus instances side-by-side
- **Profiles**: generate subsets of dashboards (e.g. `--profile infra` for infra-only)
- **Push to Grafana**: deploy dashboards directly via the Grafana API
- **Zero dependencies beyond PyYAML**: stdlib Python 3.8+

## Quick Start

```bash
pip install pyyaml

# dry run — see what would be generated
./grafana-dashboard-generator.py --config example-config.yaml --dry-run

# generate dashboard JSON files
./grafana-dashboard-generator.py --config example-config.yaml --output-dir ./output

# discover metrics from a live Prometheus
./grafana-dashboard-generator.py --config example-config.yaml --discover-print --prometheus-url http://localhost:9090

# push directly to Grafana
./grafana-dashboard-generator.py --config example-config.yaml --push --grafana-url http://localhost:3000 --grafana-token <token>
```

## Example

The included `example-config.yaml` generates 5 interlinked dashboards from standard Prometheus metrics:

| Dashboard | Panels | Focus |
|-----------|--------|-------|
| overview | 14 | cluster health, resource gauges, target status |
| compute | 18 | cpu, load, disk i/o, filesystem |
| memory | 12 | memory breakdown, swap, page i/o, oom |
| network | 14 | bandwidth, packets, errors, tcp, sockets |
| services | 14 | http metrics, latency percentiles, pod restarts |

```bash
./grafana-dashboard-generator.py --config example-config.yaml --dry-run --verbose
```

## Config Structure

```yaml
generator:          # global settings (refresh, time range, output dir)
datasources:        # named Prometheus/other datasources
palettes:           # named color palettes (hex colors)
active_palette:     # which palette to use
thresholds:         # reusable threshold definitions
selectors:          # reusable PromQL label selectors
variables:          # template variable definitions
constants:          # string constants for DRY queries
discovery:          # metric auto-discovery settings
profiles:           # named dashboard subsets
dashboards:         # dashboard definitions with sections and panels
```

See `example-config.yaml` for a complete working example and `CLAUDE.md` for full schema documentation.

## CLI Reference

| Flag | Purpose |
|------|---------|
| `--config` | Path to YAML config (required) |
| `--profile` | Generate only dashboards in named profile |
| `--output-dir` | Override output directory |
| `--prometheus-url` | Prometheus URL for discovery |
| `--grafana-url` | Grafana URL for push mode |
| `--grafana-user` | Basic auth user |
| `--grafana-pass` | Basic auth password |
| `--grafana-token` | Bearer token for Grafana API |
| `--discover-print` | Query Prometheus, print YAML snippets |
| `--dry-run` | Generate to memory only, print sizes |
| `--verbose` | Print panel details |
| `--push` | Push dashboards to Grafana API |

## Panel Types

| Type | Default Size | Best For |
|------|-------------|----------|
| `stat` | 3x4 | single value with threshold coloring |
| `gauge` | 3x4 | percentage/bounded values |
| `timeseries` | 12x7 | time-series line/bar/area charts |
| `bargauge` | 6x5 | horizontal/vertical bar comparisons |
| `heatmap` | 12x8 | distribution over time |
| `histogram` | 12x7 | value distribution |
| `table` | 24x8 | tabular data with filtering |
| `piechart` | 6x6 | proportional breakdowns |
| `state-timeline` | 12x5 | state changes over time |
| `status-history` | 12x5 | status changes grid |
| `text` | 24x3 | markdown/html content |
| `logs` | 24x8 | log viewer |
| `comparison` | 12x8 | multi-datasource metric comparison |

## Requirements

- Python 3.8+
- PyYAML (`pip install pyyaml`)

## License

MIT
