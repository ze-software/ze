---
kind: table
level:
stage:
---
| Command (WireMethod) | What the handler calls | Owner |
|----------------------|------------------------|-------|
| `ze-show:ip-route` / `ze-show:neighbors` / `ze-show:kernel-routes` | `iface.ListKernelRoutes` / `iface.ListNeighbors` (kernel tables via the iface backend) | `internal/component/iface` (NOT central `show`; NOT the BGP RIB) |
| `ze-bgp:pool-stats` | `bgp/plugins/rib/pool` attribute-pool metrics | BGP RIB plugin |
| `ze-bgp:metrics-values` / `ze-bgp:metrics-list` | generic core Prometheus registry (`internal/core/metrics`) | generic, stays central |
| `ze-bgp:subscribe` / `ze-bgp:unsubscribe` | generic `pluginserver` subscription manager | generic, stays central |
| `ze-show:policy-list` | cross-plugin filter-type registry (`registry.FilterTypesMap`) | generic, stays central |
