# 388 — CLI Metrics Commands

## Objective

Add `bgp metrics show` and `bgp metrics list` CLI commands to expose Prometheus metrics via the dispatch-command interface, without needing HTTP access.

## Decisions

- Handlers retrieve the PrometheusRegistry via `registry.GetMetricsRegistry()` and type-assert to `*metrics.PrometheusRegistry` — avoids new coupling
- Prometheus text output captured via `httptest.NewRecorder()` with synthetic GET request to the registry's HTTP handler
- `extractMetricNames()` parses Prometheus text format lines (skips comments/blanks, extracts name before `{` or space)
- Both commands registered as `ReadOnly: true` for `ze show` access

## Patterns

- Command plugin file structure: `doc.go` (blank import schema) → `schema/embed.go` → `schema/register.go` → `handler.go` with `init()` → `pluginserver.RegisterRPCs()`
- YANG module per command group (`ze-bgp-cmd-metrics-api`) with 1:1 RPC-to-handler mapping
- Blank import in `reactor.go` is required for `init()` to fire in the built binary — without it, unit tests pass but functional tests fail with "unknown command"
- Functional tests use Python plugins with `ze_api.API` dispatching commands via `ze-plugin-engine:dispatch-command`

## Gotchas

- **Missing blank import**: Unit tests pass because dispatch tests import the package directly, but the full binary needs a blank import in `reactor.go` for `init()` registration to fire. All 4 functional tests failed with "unknown command" until the import was added.
- **Handler error contract**: `dispatch-command` RPC sends JSON-RPC errors (not results) when handlers return a Go error. Business logic errors (e.g., "metrics not available") must return `StatusError` Response with `nil` Go error, so the dispatch code takes the success path and wraps the Response as a result.
- **Port 0 rejected by config**: `ExtractTelemetryConfig` validates `n >= 1 && n <= 65535`, so `port 0` falls to default 9273. Functional tests must use a real ephemeral port (e.g., 19273) to avoid bind conflicts.
- **nilerr lint**: Can't use `if err != nil { return ..., nil }` pattern. Extract the error-returning call into a helper function that returns a Response (not error).

## Files

- `internal/component/bgp/plugins/cmd/metrics/` — handler package (metrics.go, doc.go, schema/)
- `internal/component/bgp/reactor/reactor.go` — blank import added
- `test/plugin/cli-metrics-show.ci`, `test/plugin/cli-metrics-list.ci` — functional tests
