# Design Document References Map

Authoritative mapping from source directory to the `// Design:` annotation
that files in that directory should carry. Enforced by
`ai/rules/go-standards.md` (BLOCKING for all `.go` source files
that are not test, generated, register, embed, or doc files).

This file replaces the `dirMapping` table that lived in the one-shot
`scripts/add-design-refs.go` script. The script has been deleted; this
document is now the source of truth.

Multi-line entries (Design + RFC) appear when a plugin implements a specific
RFC. Both lines belong at the top of every non-exempt file in the directory.

## How to use

When adding a `.go` file in any of these directories, copy the matching
annotation block to the top of the file (after the package clause and before
the imports), then write the file's specific topic in the annotation.

When adding a new directory of source files, add an entry here first, then
add the annotation to the new files.

## Mapping

### `cmd/`

| Directory | Annotation |
|-----------|------------|
| `cmd/ze` | `// Design: docs/architecture/system-architecture.md - ze main entry point` |
| `cmd/ze-installer` | `// Design: docs/architecture/appliance/installer-initrd.md - installer initrd PID 1` |
| `cmd/ze-serial-shell` | `// Design: docs/architecture/appliance-serial-login.md - appliance serial console` |
| `cmd/ze/bgp` | `// Design: docs/architecture/core-design.md - BGP CLI commands` |
| `cmd/ze/cli` | `// Design: docs/architecture/core-design.md - interactive CLI` |
| `cmd/ze/config` | `// Design: docs/architecture/config/syntax.md - config CLI commands` |
| `cmd/ze/doctor` | `// Design: docs/architecture/system-architecture.md - readiness checks` |
| `cmd/ze/exabgp` | `// Design: docs/architecture/core-design.md - external format bridge CLI` |
| `cmd/ze/hub` | `// Design: docs/architecture/hub-architecture.md - hub startup wiring` |
| `cmd/ze/install` | `// Design: docs/architecture/system-architecture.md - installation and appliance tooling` |
| `cmd/ze/plugin` | `// Design: docs/architecture/api/process-protocol.md - plugin CLI dispatch` |
| `cmd/ze/schema` | `// Design: docs/architecture/config/yang-config-design.md - schema CLI` |
| `cmd/ze/service` | `// Design: docs/architecture/system-architecture.md - service management` |
| `cmd/ze/signal` | `// Design: docs/architecture/behavior/signals.md - signal handling CLI` |
| `cmd/ze/support` | `// Design: docs/architecture/system-architecture.md - support bundle generation` |
| `cmd/ze/ze_chaos_*.go` | `// Design: docs/architecture/chaos-web-dashboard.md - chaos test orchestrator` |
| `internal/test/cli/*.go` | `// Design: docs/architecture/testing/ci-format.md - test runner CLI` |

### `internal/component/`

| Directory | Annotation |
|-----------|------------|
| `internal/component/api` | `// Design: docs/architecture/api/commands.md - REST and gRPC command API` |
| `internal/component/bgp` | `// Design: docs/architecture/core-design.md - BGP subsystem` |
| `internal/core/bgp/attribute` | `// Design: docs/architecture/wire/attributes.md - path attribute encoding` |
| `internal/core/bgp/capability` | `// Design: docs/architecture/wire/capabilities.md - capability negotiation` |
| `internal/core/bgp/context` | `// Design: docs/architecture/encoding-context.md - encoding context` |
| `internal/component/bgp/fsm` | `// Design: docs/architecture/behavior/peer-lifecycle.md - BGP finite state machine` |
| `internal/component/bgp/message` | `// Design: docs/architecture/wire/messages.md - BGP message types` |
| `internal/component/bgp/plugins` | `// Design: docs/plugin-overview.md - BGP plugin implementations` |
| `internal/component/bgp/reactor` | `// Design: docs/architecture/core-design.md - BGP reactor event loop` |
| `internal/component/bgp/wireu` | `// Design: docs/architecture/wire/messages.md - wire UPDATE lazy parsing` |
| `internal/component/cli` | `// Design: docs/architecture/config/yang-config-design.md - unified CLI model` |
| `internal/component/cmd` | `// Design: docs/architecture/api/commands.md - online command handlers` |
| `internal/component/command` | `// Design: docs/architecture/api/commands.md - command dispatch and pipes` |
| `internal/component/config` | `// Design: docs/architecture/config/syntax.md - config parsing and loading` |
| `internal/component/config/yang` | `// Design: docs/architecture/config/yang-config-design.md - YANG schema handling` |
| `internal/component/engine` | `// Design: docs/architecture/system-architecture.md - engine supervisor` |
| `internal/component/firewall` | `// Design: docs/guide/firewall.md - firewall component` |
| `internal/plugins/flowexport` | `// Design: docs/architecture/flowexport/flow-export-0-umbrella.md - flow export component` |
| `internal/component/gnmi` | `// Design: docs/architecture/api/architecture.md - gNMI service` |
| `internal/component/host` | `// Design: docs/architecture/host/inventory.md - host inventory and hardware facts` |
| `internal/component/hub` | `// Design: docs/architecture/hub-architecture.md - hub coordination` |
| `internal/component/iface` | `// Design: docs/features/interfaces.md - interface component` |
| `internal/component/iface/cli` | `// Design: docs/architecture/iface/management.md - interface config and CLI` |
| `internal/component/iface/cmd` | `// Design: docs/architecture/iface/netlink-monitor.md - interface show commands and netlink monitor` |
| `internal/component/ike` | `// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md - IKEv2 engine` |
| `internal/component/ike/cmd` | `// Design: docs/architecture/ike/ipsec-10-cli-diag.md - IKE CLI and diagnostics` |
| `internal/component/ike/crypto` | `// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md - IKEv2 cryptography` |
| `internal/component/ike/dataplane` | `// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md - Child SA and XFRM dataplane` |
| `internal/component/ike/eap` | `// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md - EAP authentication` |
| `internal/component/ike/engine` | `// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md - IKEv2 exchange engine` |
| `internal/component/ike/ipsec` | `// Design: docs/architecture/ike/ipsec-3-data-model.md - IPsec data model` |
| `internal/component/ike/transport` | `// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md - IKE transport and NAT traversal` |
| `internal/component/ike/wire` | `// Design: docs/architecture/wire/buffer-writer.md - IKEv2 payload encoding` |
| `internal/component/l2tp` | `// Design: docs/guide/l2tp.md - L2TP subsystem` |
| `internal/component/l2tp/pppoe` | `// Design: docs/architecture/l2tp/bng-5-pppoe.md - PPPoE server` |
| `internal/component/l2tp/pppoeclient` | `// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md - PPPoE client` |
| `internal/component/l2tp/subscriber` | `// Design: docs/architecture/l2tp/subscriber-session-model.md - subscriber session model` |
| `internal/plugins/ldp` | `// Design: docs/architecture/ldp/mpls-ldp.md - LDP component` |
| `internal/component/mcp` | `// Design: docs/architecture/mcp/overview.md - MCP server` |
| `internal/component/mpls` | `// Design: docs/architecture/mpls/mpls-kernel.md - MPLS label operations` |
| `internal/component/pki` | `// Design: docs/architecture/pki/pki-store.md - certificate store` |
| `internal/component/plugin` | `// Design: docs/architecture/api/process-protocol.md - plugin infrastructure` |
| `internal/component/plugin/all` | `// Design: docs/architecture/api/architecture.md - plugin auto-registration` |
| `internal/component/plugin/registry` | `// Design: docs/architecture/api/architecture.md - plugin registry` |
| `internal/plugins/rsvpte` | `// Design: docs/architecture/rsvpte/mpls-rsvp-te.md - RSVP-TE component` |
| `internal/component/storage` | `// Design: docs/architecture/storage/smart-health.md - disk health monitoring` |
| `internal/component/telemetry` | `// Design: docs/guide/monitoring.md - telemetry service` |
| `internal/component/traffic` | `// Design: docs/guide/traffic-control.md - traffic control component` |
| `internal/component/trafficfeature` | `// Design: docs/architecture/traffic/traffic-analysis-layers.md - traffic feature extraction` |
| `internal/component/trafficstat` | `// Design: docs/architecture/traffic/traffic-analysis-layers.md - traffic statistics` |
| `internal/component/vpp` | `// Design: docs/guide/vpp.md - VPP lifecycle` |
| `internal/component/web` | `// Design: docs/architecture/web-interface.md - web interface` |

### `internal/plugins/`

| Directory | Annotation |
|-----------|------------|
| `internal/plugins/anomaly/detect` | `// Design: docs/architecture/anomaly/anomaly-1-detect.md - traffic anomaly detection` |
| `internal/plugins/anomaly/shape` | `// Design: docs/architecture/anomaly/anomaly-2-shape.md - anomaly response shaping` |
| `internal/plugins/as112` | `// Design: docs/architecture/dns/as112.md - AS112 DNS service` |
| `internal/component/bfd` | `// Design: docs/architecture/bfd.md - BFD plugin` |
| `internal/plugins/connected` | `// Design: docs/guide/redistribution.md - connected route redistribution` |
| `internal/plugins/copp` | `// Design: docs/architecture/traffic/cp-survival-2-copp-port179.md - control plane policing` |
| `internal/plugins/crashes` | `// Design: docs/architecture/diagnostics/crash-capture.md - crash report plugin` |
| `internal/plugins/ddos` | `// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md - DDoS detection and response` |
| `internal/plugins/debug` | `// Design: docs/architecture/diagnostics/debug-filtering.md - debug filter plugin` |
| `internal/plugins/dhcpserver` | `// Design: docs/architecture/provisioning/dhcp-server.md - DHCP server` |
| `internal/plugins/diag/cmd` | `// Design: docs/architecture/diagnostics/packet-capture.md - packet capture commands` |
| `internal/plugins/fib` | `// Design: docs/architecture/core-design.md - FIB plugins` |
| `internal/plugins/firewall` | `// Design: docs/guide/firewall.md - firewall backends` |
| `internal/plugins/firewall/vpp` | `// Design: docs/architecture/firewall/fw-6-firewall-vpp.md - VPP firewall backend` |
| `internal/component/firewall/plugins/irr` | `// Design: docs/architecture/firewall/firewall-irr.md - IRR firewall backend` |
| `internal/plugins/geodns` | `// Design: docs/architecture/dns/geodns.md - geographic DNS service` |
| `internal/plugins/host` | `// Design: docs/architecture/host/inventory.md - host inventory plugin` |
| `internal/plugins/host-cmd` | `// Design: docs/architecture/host/inventory.md - host inventory commands` |
| `internal/plugins/iface` | `// Design: docs/features/interfaces.md - interface backends` |
| `internal/plugins/imageserver` | `// Design: docs/architecture/provisioning/image-server.md - image server` |
| `internal/plugins/isis` | `// Design: docs/architecture/isis/isis-4-component-config.md - IS-IS component` |
| `internal/plugins/isis/adjacency` | `// Design: docs/architecture/isis/isis-5-adjacency.md - IS-IS adjacency FSM` |
| `internal/plugins/isis/circuit` | `// Design: docs/architecture/isis/isis-5-adjacency.md - IS-IS circuit runtime` |
| `internal/plugins/isis/lsdb` | `// Design: docs/architecture/isis/isis-6-lsdb.md - IS-IS link-state database` |
| `internal/plugins/isis/packet` | `// Design: docs/architecture/wire/isis.md - IS-IS PDU and TLV codec` |
| `internal/plugins/isis/redistribute` | `// Design: docs/architecture/isis/isis-11-redistribution.md - IS-IS redistribution` |
| `internal/plugins/isis/spf` | `// Design: docs/architecture/isis/isis-9-spf-rib.md - IS-IS SPF and route install` |
| `internal/plugins/isis/transport` | `// Design: docs/architecture/isis/isis-3-l2-transport.md - IS-IS Layer 2 transport` |
| `internal/plugins/isis/types` | `// Design: docs/architecture/isis/isis-1-types.md - IS-IS domain types` |
| `internal/plugins/kernel` | `// Design: docs/guide/redistribution.md - kernel route redistribution` |
| `internal/plugins/ospf` | `// Design: docs/architecture/ospf/ospf-4-component-config.md - OSPF component` |
| `internal/plugins/ospf/iface` | `// Design: docs/architecture/ospf/ospf-5-interface-ism.md - OSPF interface state machine` |
| `internal/plugins/ospf/lsdb` | `// Design: docs/architecture/ospf/ospf-7-lsdb-flooding.md - OSPF link-state database` |
| `internal/plugins/ospf/neighbor` | `// Design: docs/architecture/ospf/ospf-6-neighbor-nsm.md - OSPF neighbor state machine` |
| `internal/plugins/ospf/packet` | `// Design: docs/architecture/ospf/ospf-2-wire.md - OSPFv2 packet codec` |
| `internal/plugins/ospf/redistribute` | `// Design: docs/architecture/ospf/ospf-10-as-external-asbr.md - OSPF redistribution` |
| `internal/plugins/ospf/spf` | `// Design: docs/architecture/ospf/ospf-8-spf-rib.md - OSPF SPF and route install` |
| `internal/plugins/ospf/sr` | `// Design: docs/architecture/wire/ospf.md - OSPF segment routing TLVs` |
| `internal/plugins/ospf/transport` | `// Design: docs/architecture/ospf/ospf-3-ip-transport.md - OSPF IP transport` |
| `internal/plugins/ospf/types` | `// Design: docs/architecture/ospf/ospf-1-types.md - OSPF domain types` |
| `internal/plugins/ospf/v3/packet` | `// Design: docs/architecture/ospf/ospfv3-2-wire.md - OSPFv3 packet codec` |
| `internal/plugins/ospf/v3/transport` | `// Design: docs/architecture/ospf/ospfv3-3-ipv6-transport.md - OSPFv3 IPv6 transport` |
| `internal/plugins/ospf/v3/types` | `// Design: docs/architecture/ospf/ospfv3-1-types.md - OSPFv3 domain types` |
| `internal/plugins/policyroute` | `// Design: docs/architecture/policyroute/policy-routing.md - policy routing` |
| `internal/plugins/routingtable` | `// Design: docs/architecture/static-routes.md - named routing tables` |
| `internal/plugins/static` | `// Design: docs/architecture/static-routes.md - static routes` |
| `internal/component/sysctl` | `// Design: docs/guide/environment-variables.md - kernel tunables` |
| `internal/component/sysrib` | `// Design: docs/architecture/core-design.md - system RIB` |
| `internal/plugins/tftpserver` | `// Design: docs/architecture/provisioning/tftp-server.md - TFTP server` |
| `internal/plugins/traffic` | `// Design: docs/guide/traffic-control.md - traffic backends` |
| `internal/plugins/traffic/vpp` | `// Design: docs/architecture/traffic/fw-7-traffic-vpp.md - VPP traffic backend` |
| `internal/plugins/trafficusage` | `// Design: docs/architecture/traffic/traffic-usage.md - traffic usage accounting` |
| `internal/plugins/update-cmd` | `// Design: docs/architecture/appliance/self-update.md - appliance update commands` |
| `internal/plugins/vrrp` | `// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md - VRRP first-hop redundancy` |

### `internal/core/`

| Directory | Annotation |
|-----------|------------|
| `internal/core/bgp/nlri/nlrisplit` | `// Design: docs/architecture/rib/unified-locrib.md - NLRI split for the unified Loc-RIB` |
| `internal/core/crashlog` | `// Design: docs/architecture/diagnostics/crash-capture.md - panic capture` |
| `internal/core/dnsserver` | `// Design: docs/architecture/dns/server-harness.md - DNS server harness` |
| `internal/core/env` | `// Design: docs/architecture/config/environment.md - environment registry` |
| `internal/core/family` | `// Design: docs/architecture/core-design.md - address family registry` |
| `internal/core/health` | `// Design: docs/guide/health-checks.md - health registry` |
| `internal/core/ipc` | `// Design: docs/architecture/api/ipc_protocol.md - IPC framing and dispatch` |
| `internal/core/metrics` | `// Design: docs/plugin-development/metrics.md - metrics registry` |
| `internal/core/procfs` | `// Design: docs/architecture/diagnostics/procfs-diagnostics.md - procfs readers` |
| `internal/core/report` | `// Design: docs/guide/operational-reports.md - report bus` |
| `internal/core/rib` | `// Design: docs/architecture/route-selection.md - shared RIB structures` |
| `internal/core/rib/locrib` | `// Design: docs/architecture/rib/unified-locrib.md - unified Loc-RIB` |
| `internal/core/routewatch` | `// Design: docs/guide/redistribution.md - kernel route watcher` |
| `internal/core/slogutil` | `// Design: docs/architecture/config/environment.md - structured logging utilities` |
| `internal/core/smart` | `// Design: docs/architecture/storage/smart-health.md - SMART attribute reader` |
| `internal/core/source` | `// Design: docs/architecture/core-design.md - source registry` |
| `internal/core/stats` | `// Design: docs/architecture/traffic/traffic-analysis-layers.md - traffic counters` |

### Other

| Directory | Annotation |
|-----------|------------|
| `internal/appliance` | `// Design: docs/architecture/appliance/builder.md - appliance image build` |
| `internal/chaos` | `// Design: docs/architecture/chaos-web-dashboard.md - chaos framework` |
| `internal/exabgp` | `// Design: docs/exabgp/exabgp-migration.md - ExaBGP migration and bridge` |
| `internal/install/disk` | `// Design: docs/architecture/appliance/installer-initrd.md - disk install` |
| `internal/test` | `// Design: docs/architecture/testing/ci-format.md - test infrastructure` |
| `pkg/plugin/rpc` | `// Design: docs/architecture/api/ipc_protocol.md - plugin RPC types` |
| `pkg/plugin/sdk` | `// Design: docs/architecture/api/process-protocol.md - plugin SDK` |
| `pkg/ze` | `// Design: docs/architecture/plugin/component-boundaries.md - component interfaces` |
| `pkg/zefs` | `// Design: docs/architecture/zefs-format.md - ZeFS storage` |
| `scripts` | `// Design: (none - build tool)` |

## Notes

- The mapping uses longest-prefix match: a file under `internal/component/bgp/plugins/rib/` picks the BGP plugin entry, not the parent BGP subsystem entry.
- Exempt files (per `ai/rules/go-standards.md`): `*_test.go`, `*_gen.go`, `register.go`, `embed.go`, `doc.go`.
- When a directory is added to the source tree, add it here in the same commit, then add the annotation to the new files in the same commit.
- The `// Design: (none - ...)` form is used for directories where no architecture document applies; the parenthesised reason is required.
