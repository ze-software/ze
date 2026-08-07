---
kind: directive
level:
stage:
---
**Core-level seam (telemetry).** When more than one start site in *different components* must reach a gated feature, the seam var cannot live in the hub: put it in the always-on leaf both sites already import. The Prometheus exporter (`ze_telemetry`) is started from the hub standalone path *and* the bgp reactor path (`internal/component/bgp/config`), so its hook `metrics.StartExporter` lives in `internal/core/metrics`; the hub's gated `register_telemetry.go` wires the gated `internal/component/telemetry/exporter` (and its `collector` sidecar) into it. The metric COLLECTION API (registry + the `NopRegistry` dummy) stays in that same always-on leaf so dependents keep working when the exporter is gated: gate only the part nothing always-on imports (the HTTP exporter), never the collection API. A core leaf may hold a nil-able hook var set by a gated component init; `make ze-tier-check` stays green (a value, not an import).
