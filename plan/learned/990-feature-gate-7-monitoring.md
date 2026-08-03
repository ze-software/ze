# 990 -- feature-gate child 7: Prometheus exporter compile-out (ze_telemetry)

## Context

Child 7 of the feature-gate umbrella (`plan/spec-feature-gate-0-umbrella.md`): make
the Prometheus/metrics HTTP exporter compile-out-able via `ze_telemetry`, for a
smaller binary and attack surface. The spec scoped this to the HTTP EXPORTER only
(the `/metrics` + `/health` listener, `TelemetryConfig`, `ExtractTelemetryConfig`,
basic-auth, Netdata OS collectors) -- NOT the metric COLLECTION API
(`PrometheusRegistry`, `Gauge`, `Counter`, `Registry`, `NopRegistry`) in
`internal/core/metrics`, which ~60 packages use and which stays always-on. The
user added a requirement mid-flight: a dummy implementation so dependents keep
working when the exporter is gated out.

## Decisions

- **Core-level seam (a first for Ze).** The exporter is started from TWO sites in
  DIFFERENT components -- the hub standalone path (`cmd/ze/hub/main_system.go`)
  and the bgp reactor path (`internal/component/bgp/config/loader_create.go`). A
  hub-local seam var (ssh/gnmi/api style) cannot serve the reactor path. So the
  hook `metrics.StartExporter func(map[string]any, *slog.Logger) (*PrometheusRegistry, io.Closer)`
  lives in always-on `internal/core/metrics` (the leaf both sites import). Returns
  `(nil, nil)` when telemetry is disabled. `make ze-tier-check` stays green: a
  core leaf MAY hold a nil-able hook var set by a component (a value, not an import).
- **Hub-seam setter, not exporter `init()`.** The spec assumed the gated exporter
  package's `init()` sets the hook, blank-imported via generated `all_ze_telemetry.go`.
  But `plugin_imports.go` only emits DISCOVERED packages (plugin/schema/rpc/namespace);
  the exporter package is none of those. So `cmd/ze/hub/register_telemetry.go`
  (`//go:build ze_telemetry`) sets `metrics.StartExporter = exporter.Start`; the
  exporter exports `Start` with no init side-effect. `ze`/`ze-appliance` always link
  the hub, so the seam is wired in every ze_telemetry binary; ze-chaos (no
  ze_telemetry) simply leaves the hook nil.
- **The dummy is always-on and AVAILABLE, not a forced GetMetricsRegistry default.**
  The user asked for a "dummy default." Forcing `registry.GetMetricsRegistry()` to
  return a `NopRegistry` instead of nil would REGRESS `ike/engine/register.go`,
  which gates a 5s ticker GOROUTINE on `GetMetricsRegistry() != nil` -- a Nop would
  spin that idle ticker (and risk other control-signal consumers) when telemetry is
  off, for zero functional gain (skip vs no-op-record are observably identical). So
  the nil contract is preserved; the `NopRegistry` dummy stays always-on; all 8
  `GetMetricsRegistry` consumers already nil-check; collection-without-exporter is
  proven by `TestRegistryCollectsWithoutExporter` + `TestNopRegistryIsTheDummy`.
- **A gated seam package exports ONLY its seam entry.** `ze-validate` flags exported
  symbols with no cross-package non-test caller. The exporter initially exported
  `Server`/`TelemetryConfig`/`ExtractTelemetryConfig`/etc., but only `Start` is
  reached cross-package (by the hub seam) -- the rest are prod-internal. Fix:
  unexport them all (`server`/`telemetryConfig`/`extractTelemetryConfig`, methods
  `start`/`close`), move the in-package `server_test.go` to the internal
  `package exporter` test, and have the trafficusage integration test scrape via
  `reg.Handler()` + `httptest` instead of reaching into the gated `Server`. The
  integration test then needs no `ze_telemetry` tag (it uses only the always-on
  registry handler). General rule: a seam package's public surface is the seam
  function; everything else stays lowercase.
- **Schema relocated to keep one manifest line.** `telemetry/yang` ->
  `telemetry/exporter/yang` so the manifest line `ze_telemetry internal/component/telemetry/exporter`
  gates both the package and its `<pkg>/yang` schema (api/rest/yang precedent). A
  second line `ze_telemetry internal/component/telemetry/collector` gives dep_audit
  coverage of the Netdata sidecar (a transitive dep of the gated exporter, not
  blank-imported by the generator). The `.yang` `register.go`/`embed.go` reference
  only the filename, so moving the whole dir kept them valid.

## Consequences

- nm: ze_core links 0 `telemetry/exporter` + 0 `telemetry/collector` symbols and
  keeps 82 `internal/core/metrics` symbols; ON (`ze_core` + ZE_FEATURES) links 30
  exporter symbols. `ze`/`ze-appliance` serve `/metrics`; `ze-stripped`/`ze_core` do not.
- A reusable pattern: when >1 start site in DIFFERENT components must reach a gated
  feature, put the seam var in the always-on leaf both already import -- not the hub.
- Gate only the part nothing always-on imports (the HTTP exporter); the collection
  API + its no-op dummy stay always-on so dependents degrade gracefully.

## Gotchas

- **`make generate` scans the filesystem**, so a concurrent session's untracked
  `geodns` plugin got pulled into shared `all.go`. Stripped via Bash `grep -v`
  (NOT Edit -- the gofmt PostToolUse hook strips the generator's trailing blank
  line, breaking `plugin_imports.go --check`). `generate --check` stays locally red
  on geodns until that session commits; my `all.go` is byte-canonical minus geodns,
  self-consistent on a clean checkout.
- **Moving an exporter symbol out of `core/metrics` breaks every test that imported
  it.** `server_test.go` moved to the exporter package (`metrics.Server` ->
  `exporter.Server`, `metrics.NewPrometheusRegistry` stays). `trafficusage`'s
  `attach_integration_linux_test.go` gained `&& ze_telemetry` and the exporter import.
- **The gated package's `_test.go` must carry `//go:build ze_telemetry`** or the
  bare-ze_core lint/test set sees an empty package ("no go files to analyze"). The
  `!ze_telemetry` absent test is excluded from lint once `.golangci.yml` build-tags
  include `ze_telemetry` (so its `noctx`/`exec.Command` never fires) -- same as mcp.
- **doc-test catches relocated source anchors:** `telemetry/yang/...` ->
  `telemetry/exporter/yang/...` and `core/metrics/server.go` ->
  `telemetry/exporter/server.go` in monitoring.md + configuration.md.

## Files

- Created: `internal/core/metrics/exporter_hook.go` (StartExporter seam, always-on);
  `internal/component/telemetry/exporter/{server,exporter}.go` + `server_test.go`
  + `exporter_test.go` (gated); `cmd/ze/hub/register_telemetry.go` (gated seam);
  `cmd/ze/hub/build_tag_telemetry_{present,absent}_test.go`;
  `internal/core/metrics/registry_test.go` (dummy/collection proof);
  `internal/component/plugin/all/all_ze_telemetry.go` (generated)
- Moved: `internal/component/telemetry/exporter/yang` -> `internal/component/telemetry/exporter/yang`;
  exporter `Server`+config+extraction out of `internal/core/metrics/server.go` (deleted)
  into `telemetry/exporter/server.go`
- Modified: `cmd/ze/hub/{main_system.go,main.go}` (standalone seam + Close),
  `internal/component/bgp/config/loader_create.go` (reactor seam),
  `internal/plugins/trafficusage/attach_integration_linux_test.go`,
  `internal/component/config/all_schemas_test.go` (relocated import),
  `feature-gates.txt`, `.golangci.yml`, `internal/component/plugin/all/all.go`,
  `docs/features.md`, `docs/guide/{monitoring,configuration}.md`,
  `ai/rules/architecture.md`, `ai/rules/plugins.md`
