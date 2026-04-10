# Plugin Metrics

Plugins that have observable runtime state register their own Prometheus metrics
via the existing `ConfigureMetrics` callback in the `Registration` struct.

<!-- source: internal/component/plugin/registry/registry.go -- Registration.ConfigureMetrics -->

## Implementation Pattern

Each plugin follows the same three-step pattern used by bgp-rib and bgp-gr:

1. **Registration:** Set `ConfigureMetrics` in the `Registration` struct inside
   `init()`. The callback type-asserts the `any` parameter to `metrics.Registry`.

2. **Storage:** Declare a package-level `atomic.Pointer` holding a metrics struct.
   The struct contains the counters and gauges created from the registry.

3. **Usage:** At metric update points, load the pointer. If nil (metrics disabled),
   skip. Otherwise update the metric.

<!-- source: internal/component/bgp/plugins/rib/register.go -- ConfigureMetrics callback -->
<!-- source: internal/component/bgp/plugins/rib/rib.go -- ribMetrics struct, metricsPtr, SetMetricsRegistry -->

## Naming Policy

Every metric name is built from four parts in fixed order:

```
ze_{scope}_{subject}_{detail}
```

| Part | Rule | Examples |
|------|------|---------|
| `ze` | Always. Global prefix for all ze metrics. | |
| scope | Plugin name, hyphens stripped, lowercase. Reactor uses functional area. | `rib`, `gr`, `fibkernel`, `sysrib`, `rpki`, `persist`, `watchdog` |
| subject | Singular noun: the type of thing being measured. | `route`, `peer`, `timer`, `session`, `vrp` |
| detail | What aspect. Depends on metric type (see below). | `inserts`, `active`, `expired` |

<!-- source: internal/component/bgp/plugins/rib/rib.go -- ze_rib_* metric names -->
<!-- source: internal/component/bgp/plugins/gr/gr.go -- ze_gr_* metric names -->
<!-- source: internal/component/bgp/reactor/reactor_metrics.go -- ze_peer_* metric names -->

### Type-Specific Rules

| Type | Pattern | Suffix | Examples |
|------|---------|--------|---------|
| Counter | `ze_{scope}_{subject}_{event}_total` | Always `_total` | `ze_rib_route_inserts_total`, `ze_gr_timer_expired_total` |
| Gauge | `ze_{scope}_{subject}_{qualifier}` | Never `_total` | `ze_gr_active_peers`, `ze_fibkernel_routes_installed` |
| Histogram | `ze_{scope}_{subject}_{action}_seconds` | Always unit suffix | `ze_peer_dial_seconds` |
| CounterVec | Same as Counter, labels for dimensions | `_total` | `ze_rpki_validation_outcomes_total` with `{result}` label |
| GaugeVec | Same as Gauge, labels for dimensions | No `_total` | `ze_rib_routes_in` with `{peer}` label |

### Subject Rules

The subject is always a **singular noun** describing the category of thing measured.
When the metric counts a quantity of that noun, the grammatical plural appears
naturally in the detail.

| Subject (singular) | Gauge (current count) | Counter (cumulative events) |
|--------------------|-----------------------|-----------------------------|
| route | `routes_installed` | `route_installs_total` |
| peer | `peers_up` | `peer_flaps_total` |
| vrp | `vrps_cached` | |
| session | `sessions_active` | `session_teardowns_total` |

### Detail Rules

For **counters**, the detail is the event that happened, as a noun or past participle:
`inserts`, `withdrawals`, `expired`, `installs`, `removals`, `errors`.

For **gauges**, the detail is a qualifier distinguishing this gauge from others
in the same scope: `installed`, `active`, `up`, `cached`, `in`, `out`.

### Label Rules

Use labels for runtime dimensions. Never encode variable data in metric names.

| Label | When to use | Example |
|-------|-------------|---------|
| `peer` | Per-peer breakdown | `ze_rib_routes_in{peer="10.0.0.1"}` |
| `family` | Per-address-family breakdown | `ze_rib_route_inserts_total{family="ipv4/unicast"}` |
| `operation` | Error categorization | `ze_fibkernel_errors_total{operation="add"}` |
| `result` | Outcome categorization | `ze_rpki_validation_outcomes_total{result="valid"}` |

### Scope-to-Prefix Mapping

| Plugin (Registration Name) | Metric scope | Rationale |
|-----------------------------|-------------|-----------|
| `bgp-rib` | `rib` | Established; `bgp` prefix would be redundant |
| `bgp-gr` | `gr` | Established |
| `fib-kernel` | `fibkernel` | Hyphen stripped |
| `rib` (sysrib) | `systemrib` | Registration name is `rib` but `ze_rib_` is taken by bgp-rib; `systemrib` reads naturally |
| `bgp-watchdog` | `watchdog` | `bgp` prefix redundant |
| `bgp-rpki` | `rpki` | `bgp` prefix redundant |
| `bgp-persist` | `persist` | `bgp` prefix redundant |

### Full Inventory

| Metric Name | Type | Labels | Owner |
|-------------|------|--------|-------|
| `ze_rib_routes_in` | GaugeVec | peer | bgp-rib |
| `ze_rib_routes_out` | GaugeVec | peer | bgp-rib |
| `ze_rib_route_inserts_total` | CounterVec | peer, family | bgp-rib |
| `ze_rib_route_withdrawals_total` | CounterVec | peer, family | bgp-rib |
| `ze_gr_peers_active` | Gauge | | bgp-gr |
| `ze_gr_routes_stale` | GaugeVec | peer | bgp-gr |
| `ze_gr_timer_expired_total` | CounterVec | peer | bgp-gr |
| `ze_fibkernel_routes_installed` | Gauge | | fib-kernel |
| `ze_fibkernel_route_installs_total` | Counter | | fib-kernel |
| `ze_fibkernel_route_updates_total` | Counter | | fib-kernel |
| `ze_fibkernel_route_removals_total` | Counter | | fib-kernel |
| `ze_fibkernel_errors_total` | CounterVec | operation | fib-kernel |
| `ze_systemrib_routes_best` | Gauge | | rib (sysrib) |
| `ze_systemrib_route_changes_total` | CounterVec | action | rib (sysrib) |
| `ze_systemrib_events_received_total` | Counter | | rib (sysrib) |
| `ze_watchdog_peers_up` | Gauge | | bgp-watchdog |
| `ze_watchdog_route_announcements_total` | Counter | | bgp-watchdog |
| `ze_watchdog_route_withdrawals_total` | Counter | | bgp-watchdog |
| `ze_rpki_vrps_cached` | Gauge | | bgp-rpki |
| `ze_rpki_sessions_active` | Gauge | | bgp-rpki |
| `ze_rpki_validation_outcomes_total` | CounterVec | result | bgp-rpki |
| `ze_persist_routes_stored` | Gauge | | bgp-persist |
| `ze_persist_peers_tracked` | Gauge | | bgp-persist |
| `ze_persist_route_replays_total` | Counter | | bgp-persist |

## NopRegistry Fallback

When telemetry is disabled in config, plugins receive a `NopRegistry` that returns
no-op metric implementations. The nil-check on the atomic pointer handles the case
where `ConfigureMetrics` is never called at all (plugin loaded without a metrics
registry).

<!-- source: internal/core/metrics/nop.go -- NopRegistry -->

## Which Plugins Need Metrics

Not every plugin needs metrics. NLRI codec plugins (flowspec, evpn, labeled, etc.)
are stateless encoders/decoders with no runtime state to observe. Only plugins
with state machines, caches, or I/O operations benefit from metrics.

| Has metrics | No metrics needed |
|-------------|-------------------|
| Plugins with route state (rib, sysrib, persist) | NLRI codecs (bgp-nlri-*) |
| Plugins with external I/O (fib-kernel, rpki) | Capability-only plugins (bgp-role) |
| Plugins with timers/state machines (gr, watchdog) | Format/encoding plugins |

## Reference Implementation

The bgp-rib plugin (`internal/component/bgp/plugins/rib/`) is the reference
implementation. It demonstrates all aspects: registration callback, metrics struct,
atomic pointer, periodic update loop, and per-peer label cleanup.
