# Architecture Documentation

Architecture documents describe how the current implementation is wired. Prefer source-anchored documents here over historical research notes when checking behavior.

## Core Map

| Topic | Document | Code anchor |
|-------|----------|-------------|
| One-page overview | `../architecture.md` | `internal/component/engine/`, `cmd/ze/hub/` |
| Core design | `core-design.md` | BGP, plugin, config, and engine packages |
| System architecture | `system-architecture.md` | Hub startup and subsystem wiring |
| Plugin manager wiring | `plugin-manager-wiring.md` | `internal/component/plugin/` |
| Subsystem wiring | `subsystem-wiring.md` | Registered components and plugin server wiring |
| Config design | `config/` | `internal/component/config/` and YANG modules |
| API and IPC | `api/` | `pkg/plugin/rpc/`, `pkg/plugin/sdk/`, command schemas |
| BGP wire format | `wire/` | `internal/core/bgp/attribute/`, `message/`, `wireu/` |
| Route and RIB behavior | `route-selection.md`, `route-types.md`, `rib-transition.md` | `internal/core/rib/`, `internal/component/bgp/plugins/rib/` |
| Pools and buffers | `pool-architecture.md`, `buffer-architecture.md` | `internal/component/bgp/attrpool/`, `internal/core/bufpool/` |
| Web and UI | `web-interface.md`, `web-components.md` | `internal/component/web/` |
| Testing architecture | `testing/` | `internal/test/cli/*.go`, `internal/test/`, `test/` |
| Session and signal behavior | `behavior/` | `internal/component/bgp/fsm/`, `internal/plugins/signal/` |
| BGP plugin behavior | `bgp/` | `internal/component/bgp/plugins/`, `internal/component/bgp/reactor/` |
| Local RIB internals | `rib/` | `internal/core/rib/locrib/`, `internal/core/bgp/nlri/nlrisplit/` |
| Protocol edge cases | `edge-cases/` | `internal/component/bgp/` |
| Route metadata keys | `meta/` | `internal/component/bgp/plugins/role/` |
| IGP routing protocols | `isis/`, `ospf/` | `internal/plugins/isis/`, `internal/plugins/ospf/` |
| MPLS label switching | `mpls/`, `ldp/`, `rsvpte/`, `fib/` | `internal/component/mpls/`, `internal/plugins/ldp/`, `internal/plugins/rsvpte/`, `internal/plugins/fib/` |
| First-hop redundancy | `vrrp/` | `internal/plugins/vrrp/` |
| IKE and IPsec | `ike/` | `internal/component/ike/` |
| Subscriber access | `l2tp/` | `internal/component/l2tp/` |
| Interfaces | `iface/` | `internal/component/iface/` |
| Firewall | `firewall/` | `internal/component/firewall/`, `internal/plugins/firewall/` |
| Policy routing | `policyroute/` | `internal/plugins/policyroute/` |
| Traffic control and accounting | `traffic/` | `internal/plugins/traffic/`, `internal/plugins/trafficusage/`, `internal/core/stats/` |
| Flow export | `flowexport/` | `internal/plugins/flowexport/` |
| Anomaly and DDoS response | `anomaly/`, `ddos/` | `internal/plugins/anomaly/`, `internal/plugins/ddos/` |
| DNS services | `dns/` | `internal/core/dnsserver/`, `internal/plugins/as112/`, `internal/plugins/geodns/` |
| Diagnostics and crash capture | `diagnostics/` | `internal/core/crashlog/`, `internal/core/procfs/`, `internal/plugins/diag/` |
| Host hardware | `host/` | `internal/component/host/`, `internal/plugins/host/` |
| Storage health | `storage/` | `internal/component/storage/`, `internal/core/smart/` |
| Appliance, install, and update | `appliance/` | `internal/appliance/`, `internal/install/disk/`, `cmd/ze-installer/` |
| Provisioning services | `provisioning/` | `internal/plugins/dhcpserver/`, `internal/plugins/tftpserver/`, `internal/plugins/imageserver/` |
| Certificate store | `pki/` | `internal/component/pki/` |
| Credential masking | `ssh/` | `internal/component/ssh/`, `internal/core/redact/` |
| MCP server | `mcp/` | `internal/component/mcp/` |
| Plugin boundaries and RIB storage | `plugin/` | `internal/component/plugin/`, `pkg/ze/` |
| Plugin test tools | `debugging/` | `internal/component/plugin/cli/`, `internal/test/plugins/` |
| CLI surface | `cli/` | `cmd/ze/`, `internal/component/command/` |
| Memory lifetime contracts | `memory/` | `internal/core/memguard/` |
| Decisions | `decisions/` | Decision records tied to current implementation |

## Reading Order

1. Start with `../architecture.md` for the current component map.
2. Read `core-design.md` for the detailed implementation model.
3. Move into the topic directory that matches the package you are changing.
4. Use source anchors in the document to verify claims against code.
