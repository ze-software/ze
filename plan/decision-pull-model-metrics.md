# Decision: pull-model metrics Collector

| Field | Value |
|-------|-------|
| Status | decided — defer |
| Updated | 2026-04-11 |
| Scope | Decision doc only. Not a spec. |

## The question

Should ze switch from its current push-model metrics to a pull-model
`prometheus.Collector` pattern?

- **Push model (today):** each plugin receives a `metrics.Registry` at
  startup via `ConfigureMetrics`, and increments counters/gauges inline
  with business logic. Metric names live in each plugin next to the
  increments.
- **Pull model (proposed):** each plugin exposes a `Snapshot() *Metrics`
  method returning a plain Go struct. A separate adapter package imports
  the Prometheus client library and converts snapshots to Prometheus
  metrics on scrape. Metric names live in the adapter. The core plugin
  code has no metrics dependency.

Three sub-questions, asked by the user before any refactor:

1. **Is it worth doing now, or wait until the metrics surface is larger?**
2. **What would the cost be?**
3. **How does external-plugin metrics work in the pull model, since external
   plugins run over RPC?**

## Relevant ze state (verified against code, not memory)

- `internal/core/metrics/metrics.go` defines an **abstract `Registry`
  interface** with `Counter`, `Gauge`, `CounterVec`, `GaugeVec`,
  `Histogram`, `HistogramVec`. The interface names and method shapes
  deliberately match Prometheus client_golang types so that the Prometheus
  backend can forward without wrapping, but the interface itself does not
  import Prometheus.
- `internal/core/metrics/prometheus.go` and `internal/core/metrics/nop.go`
  are the two backends. Plugins import only the abstract `metrics.Registry`.
  <!-- source: internal/core/metrics/metrics.go -->
- **Zero direct Prometheus imports in `internal/component/bgp/`.** Verified
  by `grep "github.com/prometheus/"` — matches are in vendor, go.sum,
  `internal/core/metrics/prometheus.go`, and `internal/chaos/report/metrics.go`.
  No BGP plugin touches Prometheus types directly.
- `ConfigureMetrics func(reg any)` lives on `plugin.Registration` in
  `internal/component/plugin/registry/registry.go:75`. It is called before
  `RunEngine` with the registry as `any`; each plugin type-asserts to
  `metrics.Registry` and calls `SetMetricsRegistry(r)`.
  <!-- source: internal/component/plugin/registry/registry.go — ConfigureMetrics -->
- Plugins using the push model today: **six**. `rib`, `watchdog`, `rpki`,
  `persist`, `gr`, plus `reactor/reactor_metrics.go` (reactor itself).
- **External plugins (`pkg/plugin/`) have no metrics hook at all.** The
  plugin SDK exposes no metrics interface, no metrics RPC, no metrics
  namespace. External subprocess plugins cannot publish metrics today.
  <!-- source: pkg/plugin/ — absence of any metrics file -->
- Example push-model code: `internal/component/bgp/plugins/rib/rib.go:113`
  registers six gauges + three vectors via the abstract registry. Metric
  names like `"ze_rib_routes_in_total"` are string literals in the plugin
  file.
  <!-- source: internal/component/bgp/plugins/rib/rib.go — SetMetricsRegistry -->

## What the pull model would buy, given ze's current state

The headline benefits usually cited for the pull-model pattern are:

1. **Prometheus-type isolation** — the core code has no Prometheus
   dependency, transports can be swapped for OpenTelemetry or JSON.
2. **Metric-name centralization** — one file lists all metric names.
3. **Snapshot-on-scrape economics** — no cost on the hot path; the
   structure is walked only when Prometheus scrapes.
4. **Testability** — tests read a struct field directly, no registry
   setup.

**Benefit 1 does not apply to ze.** Ze already has the abstract
`metrics.Registry` interface, so the Prometheus decoupling is already in
place at the type level. Plugins import the abstract interface, not
Prometheus.

**Benefits 2, 3, and 4 do apply.** Metric names still live in each plugin
file today (`"ze_rib_routes_in_total"` is a string literal in `rib.go`).
Live gauge updates happen per-message in the UPDATE hot path. Tests that
want to inspect a metric have to walk the registry rather than read a
struct.

## Answers

### Q1: Is it worth doing now, or wait?

**Wait.**

Reasons:
- Push-model code is correct, decoupled at the type level, and tested.
  Six plugins is small enough that a later refactor is mechanical.
- The usual "Prometheus leaking everywhere" motivator does not apply to
  ze because the abstract registry already handles that decoupling.
- There is a bigger architectural decision pending: **external-plugin
  metrics** (see Q3). Refactoring internal plugins now, then doing a second
  refactor once external-plugin metrics are designed, is worse than waiting
  until both problems are solved together.
- Doing the refactor now locks in a shape that may not fit the
  process-plugin case. The pull model assumes synchronous local method
  calls (`plugin.Snapshot()`), which do not exist for an RPC plugin.

**Revisit when any of these is true:**
- The internal-plugin metrics count roughly doubles (12+ plugins), making
  a name-centralization refactor valuable independent of the external case.
- A second export format is wanted (OpenTelemetry, JSON over HTTP, a
  CLI dump). At that point the adapter split pays for itself because
  each format is one adapter, not six more plugin integrations.
- A hot-path UPDATE profile shows the current `gauge.Inc()` / `Add()`
  calls as significant. Pull-model snapshot-on-scrape would reduce that
  cost to zero on the UPDATE path.
- The external-plugin metrics gap (Q3) forces a protocol decision that
  incidentally standardizes the internal shape too.

### Q2: What would the cost be?

Roughly six pages of plugin-internal restructuring, plus a new adapter
package, plus a protocol decision for external plugins. Breakdown:

| Work | Scope |
|------|-------|
| Define `Snapshot()` method per plugin + the plain Go struct it returns | Six plugins (`rib`, `watchdog`, `rpki`, `persist`, `gr`) + reactor |
| Adapter package `internal/core/metrics/adapter/prom/` that imports `prometheus` and calls each plugin's snapshot | New package |
| Delete the six `SetMetricsRegistry` functions and the abstract `Registry` interface (or keep the interface for the nop backend path) | Simplification or deletion |
| Delete `ConfigureMetrics` from `plugin.Registration` (or keep as a marker) | Deletion |
| Redefine the metric name catalog as constants in one file | New file |
| Update scrape path: HTTP `/metrics` handler calls the adapter, not the registry | `internal/core/metrics/server.go` |
| External-plugin metrics: design a `Snapshot` RPC on the plugin protocol | Non-trivial: the plugin protocol gains a new method, external plugin authors have to implement it |
| Tests: swap "registry recorded this counter" for "snapshot contains this field" | Per-plugin test rewrite |

Net: manageable for internal plugins. The external-plugin piece is the
hard half; see Q3.

**Time estimate:** deliberately omitted per `CLAUDE.md`. The number of
touch points is what matters for the decision.

### Q3: How does external-plugin metrics work in the pull model?

**Today: not at all.** External plugins (anything running as a subprocess
over RPC under `pkg/plugin/`) have zero metrics hook. Nothing in the SDK
takes a registry, exposes a snapshot method, or carries metric data in the
RPC protocol. This is a gap, not a design — process plugins simply cannot
publish metrics.

**Pull-model options if the refactor is adopted:**

1. **New plugin RPC: `Snapshot() SnapshotResponse`.** The adapter calls
   every registered plugin (in-process and subprocess alike) via the same
   dispatcher. Pros: uniform treatment, the external case gets first-class
   metrics for free. Cons: the RPC has to serialize a structured metric
   snapshot (protobuf or JSON), each external plugin implements a new
   mandatory method, and the scrape path becomes "fan out over RPC to all
   plugins and aggregate".

2. **Streaming metrics.** Plugins push a snapshot at their own cadence (e.g.
   every 10s) over an event stream. Adapter caches the last value per
   plugin and serves it on scrape. Pros: no per-scrape fan-out. Cons:
   staleness bounded by cadence, scrape time no longer reflects reality.

3. **In-process only.** Pull model for in-process plugins, nothing for
   external. Accept the asymmetry. Pros: simplest. Cons: external plugins
   remain invisible in metrics (same as today — no regression, no gain).

None of these is obviously right. Option 1 is the cleanest form of the
pattern but adds a mandatory protocol method for every external plugin.
Option 2 trades a simpler API for staleness. Option 3 solves the internal
case cleanly and leaves the external case for a later design.

**This is the piece that makes me want to defer.** The push model has the
same problem (no external plugin metrics) and therefore does not force a
decision. The pull model forces the decision because the adapter needs to
know where snapshots come from. The design is not mature enough to pick
between options 1/2/3 today.

## Recommendation

**Defer the pull-model refactor until one of the revisit triggers in Q1
fires.** When it does, resolve the external-plugin metrics question first
(Q3) — the answer to Q3 dictates the shape of the pull-model adapter and
therefore the shape of the refactor.

In the interim:

- **Keep the push model as-is.** It works, it is decoupled at the type
  level, and the cost of owning it is one `ConfigureMetrics` callback plus
  a few `Gauge.Set` / `Counter.Inc` calls per plugin.
- **If a new plugin is added**, copy the existing push-model shape. Do not
  invent a pull-model half-implementation alongside the push model.
- **When naming new metrics**, prefer consistency with the existing
  `ze_<subsystem>_<thing>_<unit>` convention (`ze_rib_routes_in_total`,
  `ze_rib_route_inserts_total`). The centralized-name-catalog benefit that
  the pull model would provide is partially recovered by discipline.
- **If the external-plugin metrics gap becomes user-visible**, treat that
  as the forcing function: the design for external-plugin metrics is what
  this decision is waiting on.

This decision does not block adopting the pull model later. None of the
current push-model code survives in the pull-model end state, so there is
no "combine them" trap. When the refactor happens, it replaces the whole
push model in one sitting (no-layering rule, `.claude/rules/no-layering.md`).

## What this decision is not

- Not a rejection of the pull-model pattern on its merits. The long-term
  benefits (metric-name centralization, snapshot-on-scrape economics,
  direct-struct testability) are real.
- Not a rejection of OpenTelemetry. If OpenTelemetry lands as a second
  export format, that is a revisit trigger.
- Not a defense of the push model. The push model is defensible today but
  will get worse as the plugin count grows.
