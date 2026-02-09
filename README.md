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
- **Web UI**: edit config, browse metrics, generate and push dashboards from a browser

## Quick Start

### Binary

```bash
# download from GitHub releases
# https://github.com/wcatz/dashboard-generator/releases

# generate dashboard JSON
./dashboard-generator generate --config example-config.yaml --dry-run --verbose

# start web UI
./dashboard-generator serve --config example-config.yaml --port 8080

# push to Grafana
./dashboard-generator push --config example-config.yaml --grafana-url http://localhost:3000 --grafana-token $TOKEN
```

### Docker

```bash
# run web UI on localhost:8080, write generated dashboards to ./output
docker run --rm -p 8080:8080 \
  -v $(pwd)/example-config.yaml:/data/config.yaml:ro \
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
| `--grafana-url` | push, serve | Grafana URL for push |
| `--grafana-user` | push | Basic auth user |
| `--grafana-pass` | push | Basic auth password |
| `--grafana-token` | push | Bearer token for Grafana API |
| `--port` | serve | HTTP port (default 8080) |

## Helm Chart

Published to `oci://ghcr.io/wcatz/helm-charts/dashboard-generator` on every push to master and version tag.

```bash
# install from OCI registry
helm install dashboard-generator oci://ghcr.io/wcatz/helm-charts/dashboard-generator --version 0.1.0

# or use in helmfile
chart: oci://ghcr.io/wcatz/helm-charts/dashboard-generator
version: 0.1.0
```

## Docker Image

Published to `wcatz/dashboard-generator` on Docker Hub.

| Tag Pattern | Trigger |
|-------------|---------|
| `latest` | push to master or version tag |
| `master` | push to master |
| `0.1.0` | git tag `v0.1.0` |
| `0.1` | git tag `v0.1.x` |
| `0` | git tag `v0.x.x` |

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

## Releasing

```bash
# 1. bump helm-chart/Chart.yaml version + appVersion
# 2. commit and push
git tag v0.2.0
git push origin v0.2.0
# CI builds: Docker image (Docker Hub), Helm chart (ghcr.io), GitHub release (goreleaser)
```

## License

MIT
