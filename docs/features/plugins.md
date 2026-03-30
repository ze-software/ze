# Plugins

### Storage & Policy

| Plugin | Description |
|--------|-------------|
| bgp-rib | Route Information Base -- stores received/sent routes |
| bgp-adj-rib-in | Adj-RIB-In -- raw hex replay of received routes |
| bgp-persist | Route persistence across restarts |
| bgp-rs | Route server -- client-to-client route reflection (RFC 7947) |
| bgp-watchdog | Deferred route announcement with named watchdog groups |

### Redistribution Filters (planned)

External plugins can act as route filters on import and export. Filters are
configured per peer/group/globally via `redistribution { import [...] export [...] }`
using `<plugin>:<filter>` references. Multiple filters chain as piped transforms
(each sees previous filter's output). Filters respond accept/reject/modify with
delta-only attribute changes and dirty tracking for efficient re-encoding.

Three filter categories:

| Category | Behavior | Example |
|----------|----------|---------|
| Mandatory | Always on, cannot be overridden | `rfc:otc` (RFC 9234) |
| Default | On by default, can be overridden per-peer | `rfc:no-self-as` (loop prevention) |
| User | Only present when explicitly configured | `rpki:validate`, `community:scrub` |

<!-- source: plan/spec-redistribution-filter.md -- redistribution filter design -->

<!-- source: internal/component/bgp/plugins/rib/register.go -- bgp-rib -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/register.go -- bgp-adj-rib-in -->
<!-- source: internal/component/bgp/plugins/persist/register.go -- bgp-persist -->
<!-- source: internal/component/bgp/plugins/rs/register.go -- bgp-rs -->
<!-- source: internal/component/bgp/plugins/watchdog/register.go -- bgp-watchdog -->

### Protocol

| Plugin | Description |
|--------|-------------|
| bgp-gr | Graceful Restart (RFC 4724) and Long-Lived GR (RFC 9494) state machine |
| bgp-aigp | Accumulated IGP Metric (RFC 7311) |
| bgp-rpki | RPKI origin validation via RTR protocol (RFC 6811, RFC 8210). [Guide](guide/rpki.md) |
| bgp-rpki-decorator | Correlates UPDATE + RPKI events into merged update-rpki events |
| bgp-route-refresh | Route Refresh handling (RFC 2918, RFC 7313) |
| role | BGP Role capability enforcement (RFC 9234) |
| bgp-llnh | Link-local next-hop for IPv6 (RFC 2545) |
| bgp-hostname | FQDN capability for peer identification |
| bgp-softver | Software version capability advertisement |
| filter-community | Community tag/strip filter (standard, large, extended) |
| loop | Route loop detection (RFC 4271 S9, RFC 4456 S8) |

<!-- source: internal/component/bgp/plugins/gr/register.go -- bgp-gr -->
<!-- source: internal/component/bgp/plugins/rpki/register.go -- bgp-rpki -->
<!-- source: internal/component/bgp/plugins/rpki_decorator/register.go -- bgp-rpki-decorator -->
<!-- source: internal/component/bgp/plugins/route_refresh/register.go -- bgp-route-refresh -->
<!-- source: internal/component/bgp/plugins/role/register.go -- role -->
<!-- source: internal/component/bgp/plugins/llnh/register.go -- bgp-llnh -->
<!-- source: internal/component/bgp/plugins/hostname/register.go -- bgp-hostname -->
<!-- source: internal/component/bgp/plugins/softver/register.go -- bgp-softver -->
<!-- source: internal/component/bgp/plugins/aigp/register.go -- bgp-aigp -->
<!-- source: internal/component/bgp/plugins/filter_community/register.go -- filter-community -->
<!-- source: internal/component/bgp/reactor/filter/register.go -- loop -->

### Plugin Health Metrics

Plugin infrastructure exposes per-plugin Prometheus metrics for operational visibility:

| Metric | Type | Description |
|--------|------|-------------|
| `ze_plugin_status{plugin}` | Gauge | Current plugin stage (0=init, 6=running). Absent when disabled. |
| `ze_plugin_restarts_total{plugin}` | Counter | Cumulative restart count. |
| `ze_plugin_events_delivered_total{plugin}` | Counter | Total events enqueued to plugin. |

When a plugin is disabled (respawn limit exceeded), its metrics are deleted rather than showing a stale value.

<!-- source: internal/component/plugin/process/manager.go -- pluginMetrics -->
