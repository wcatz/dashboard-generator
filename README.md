# dashboard-generator

Config-driven Grafana dashboard generator. Write YAML, get interlinked JSON dashboards.

## Why

Hand-editing Grafana JSON is painful. Clicking through the UI doesn't scale. This tool lets you define dashboards declaratively in YAML and generates production-ready Grafana JSON with automatic navigation links, layout, and consistent styling.

**One config file. Any number of dashboards. Zero clicking.**

## Features

- **16 panel types**: stat, gauge, timeseries, bargauge, heatmap, histogram, table, piechart, state-timeline, status-history, text, logs, row, comparison, alertlist, dashlist
- **Auto-layout engine**: panels flow left-to-right across a 24-unit grid, wrapping automatically
- **Navigation links**: every dashboard links to every other dashboard in the set
- **Reference system**: reusable colors (`$green`), thresholds (`$percent_usage`), selectors (`${by_ns}`), and constants (`${rate_interval}`)
- **Template variables**: query, custom, datasource, and interval types with chaining support
- **Metric discovery**: query Prometheus (direct or via Grafana proxy) with Bearer token and Basic auth support
- **Intelligent panel suggestions**: heuristic engine infers panel type, unit, thresholds, query transforms, and sizing from metric name and Prometheus type
- **Optional AI suggestions**: Claude API integration for production-quality panel generation (requires API key)
- **Multi-datasource comparison**: compare metrics and labels across Prometheus instances
- **Profiles**: generate subsets of dashboards (e.g. `--profile executive`)
- **Push to Grafana**: deploy dashboards directly via the Grafana API
- **Web UI**: browse metrics, visual dashboard preview with panel detail drawer, interactive palette editor, config editor, generate and push from a browser

## Web UI

Single-binary web server with embedded assets. No Node.js, no build step.

| Page | Description |
|------|-------------|
| **Overview** | Dashboard list with stats, generate/push buttons, preview links |
| **Datasources** | Add/remove datasources, test connections, compare metrics and labels across all datasources |
| **Metrics** | Browse and filter metrics by datasource, job, type. Generate YAML snippets or AI-powered suggestions |
| **Preview** | Interactive visual preview with per-section layout, panel detail drawer, search, type filters, zoom, compact mode |
| **Editor** | Edit YAML config with syntax highlighting, save/reload |
| **Palettes** | CRUD palette colors, activate palettes, threshold presets |
| **Variables** | View template variable definitions |
| **References** | View selectors and constants |
| **Profiles** | View named dashboard subsets |
| **Settings** | View generator and runtime settings |

## Quick Start

### Binary

```bash
# download from GitHub releases
# https://github.com/wcatz/dashboard-generator/releases

# generate dashboard JSON
./dashboard-generator generate --config config.yaml --dry-run --verbose

# start web UI
./dashboard-generator serve --config config.yaml --port 8080

# push to Grafana
./dashboard-generator push --config config.yaml \
  --grafana-url http://localhost:3000 --grafana-token $TOKEN
```

### Docker

```bash
docker run --rm -p 8080:8080 \
  -v $(pwd)/config.yaml:/data/config.yaml:ro \
  -v $(pwd)/output:/data/output \
  wcatz/dashboard-generator:latest

# open http://localhost:8080
```

### Build from Source

```bash
make build
make test
make docker-build
make docker-run    # serves on localhost:8080, outputs to ./output
```

## Datasource Authentication

Supports Bearer token and Basic auth for Prometheus endpoints, including Grafana Cloud proxy:

```yaml
datasources:
  my_prom:
    type: prometheus
    uid: my-prom-uid
    url: https://my-grafana.grafana.net/api/datasources/proxy/uid/my-prom-uid
    is_default: true
    token: "$GRAFANA_SA_TOKEN"     # env var reference, resolved at runtime

  another_prom:
    type: prometheus
    uid: another-uid
    url: https://prom.example.com
    basic_user: "admin"
    basic_pass: "$PROM_PASSWORD"   # env var reference
```

Environment variable references (`$VAR_NAME`) in `token` and `basic_pass` are resolved at runtime, so secrets never need to be in the YAML file.

## CLI Reference

| Command | Purpose |
|---------|---------|
| `generate` | Generate dashboard JSON from YAML config |
| `discover` | Query Prometheus and print suggested YAML snippets |
| `push` | Generate and push dashboards to Grafana API |
| `serve` | Start the web UI server |

| Flag | Commands | Purpose |
|------|----------|---------|
| `--config` | all | Path to YAML config (required) |
| `--profile` | generate, push | Named profile filter |
| `--output-dir` | generate, push | Override output directory |
| `--dry-run` | generate | Generate to memory only |
| `--verbose` | generate, push | Print panel details |
| `--prometheus-url` | discover | Prometheus URL for metric discovery |
| `--prometheus-token` | discover | Bearer token for Prometheus auth |
| `--prometheus-user` | discover | Basic auth user |
| `--prometheus-pass` | discover | Basic auth password |
| `--grafana-url` | push, serve | Grafana URL for push |
| `--grafana-user` | push | Basic auth user |
| `--grafana-pass` | push | Basic auth password |
| `--grafana-token` | push | Bearer token for Grafana API |
| `--port` | serve | HTTP port (default 8080) |

## Config Structure

```yaml
generator:          # global settings (refresh, time range, output dir)
datasources:        # named datasources with optional auth
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

See `example-config.yaml` for a complete working example.

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

## Intelligent Panel Generation

When using metric discovery or the web UI snippet generator, the heuristic engine automatically:

- **Infers panel type** from Prometheus metric type and name suffix (`_total` -> timeseries with `rate()`, `_ratio` -> gauge, `_info` -> stat with value mapping, `_bucket` -> histogram_quantile)
- **Infers units** from metric name (`_bytes` -> bytes, `_seconds` -> s, `_percent` -> percent, `_bytes_total` -> Bps)
- **Generates proper PromQL** including `rate()` for counters, `histogram_quantile()` for histograms, with configurable rate interval
- **Suggests thresholds** based on metric semantics (percent metrics get percentage thresholds, info metrics get binary health)
- **Cleans titles** by stripping type suffixes and replacing underscores

Optional Claude API integration generates even richer panel configs with full context awareness of your selectors, constants, and available thresholds.

## Deployment

### Helm Chart

Published to `oci://ghcr.io/wcatz/helm-charts/dashboard-generator` on every push to master and version tag.

```bash
helm install dashboard-generator \
  oci://ghcr.io/wcatz/helm-charts/dashboard-generator --version 0.1.0
```

### Docker Image

Published to `wcatz/dashboard-generator` on Docker Hub.

| Tag Pattern | Trigger |
|-------------|---------|
| `latest` | push to master or version tag |
| `master` | push to master |
| `0.1.0` | git tag `v0.1.0` |
| `0.1` | git tag `v0.1.x` |
| `0` | git tag `v0.x.x` |

## Releasing

```bash
# 1. bump helm-chart/Chart.yaml version + appVersion
# 2. commit and push
git tag v0.2.0
git push origin v0.2.0
# CI builds: Docker image (Docker Hub), Helm chart (ghcr.io), GitHub release (goreleaser)
```

## Dependencies

- `gopkg.in/yaml.v3` — YAML parsing
- `github.com/spf13/cobra` — CLI framework
- Go stdlib: `encoding/json`, `net/http`, `html/template`, `embed`

## License

MIT
