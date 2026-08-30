---
kind: directive
level: MUST
stage:
---
- **When start sites in DIFFERENT components reach one gated feature, the seam var MUST live in the always-on leaf both sites already import, never in the hub.** Only the part nothing always-on imports MUST be gated: the telemetry HTTP exporter is gated while the metric collection API stays in `internal/core/metrics`, so dependents keep working. A core leaf holding a nil-able hook var that a gated `init()` sets keeps `./le tier check` green, because it is a value rather than an import.
