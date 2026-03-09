# Dashboard Generator — Config-Driven Grafana Dashboards

## Stack
- Go 1.24 (sole implementation — Python generator removed)
- Web UI: HTMX + Tailwind CSS + DaisyUI v4, dark theme, embedded assets via `embed.FS`
- 16 panel types: stat, gauge, timeseries, bargauge, heatmap, histogram, table, piechart, state-timeline, status-history, text, logs, row, comparison, alertlist, dashlist

## Critical Guardrails
- `GeneratorSettings.String()` had a recursion bug — fixed with `type noStringer` pattern, do NOT regress
- CSRF: POST requests require Origin header matching Host
- golangci-lint required: check for errcheck, gosimple, unused
- `plugin_version` is configurable via `generator.plugin_version` in config (default 11.2.0) — NOT hardcoded

## AI Integration
- `internal/generator/ai.go`: Anthropic Messages API client, default model `claude-haiku-4-5-20251001`
- `internal/generator/suggest.go`: heuristic panel suggestions
- Config keys: `generator.anthropic_api_key` (supports `$ENV_VAR`), `generator.anthropic_model`
- Routes: `/api/metrics/ai-suggest`, `/api/metrics/ai-suggest-bulk`

## Live Preview (Grafana iframe)
- Requires `--grafana-url` + `--grafana-token` (or `GRAFANA_URL`/`GRAFANA_TOKEN` env vars)
- Token: Grafana service account token with Editor role
- Grafana `grafana.ini` must have: `security.allow_embedding=true`, `auth.anonymous.enabled=true` (org_role: Viewer)
- Pushes temporary `{uid}-preview` dashboards, renders in kiosk mode iframe
- Falls back to mock preview when Grafana not configured

## Output
- Grafana Operator v1beta1 Dashboard CRD YAML via `internal/generator/yaml_writer.go`
- Routes: `/templates`, `/api/labels/discover`, `/api/labels/values`, `/api/variable/*`, `/api/template/*`

## Testing
- Coverage: config 83%, generator 72%, server 59%, overall 65%
