# Plugin Architecture

An overview of Ze's plugin system. For writing your own plugins, see
[plugin-development/](plugin-development/). For the full plugin user guide, see
[guide/plugins.md](guide/plugins.md).

## Concept

Most runtime features beyond the core engine are registered components or plugins.
Plugins handle RIB storage, route reflection, graceful restart, RPKI validation,
NLRI encoding, FIB programming, route redistribution, DHCP/PXE helpers, L2TP
helpers, and backend integrations. Components such as interface, firewall,
traffic, VPP, LDP, RSVP-TE, IS-IS, OSPF, and flow export also register through
the same plugin registry when they need config-driven lifecycle or IPC. The engine discovers them
through registries and never imports implementation packages directly.
<!-- source: internal/component/plugin/registry/registry.go -- plugin registry -->
<!-- source: internal/component/plugin/all/all.go -- plugin blank imports -->

## Registration Pattern

Plugins register at startup via `init()` in a `register.go` file. Each plugin
provides a `Registration` struct declaring its name, dependencies, capability
decoders, config roots, event types, and YANG schema.

The "delete the folder" test: if you delete `internal/component/bgp/plugins/rib/`,
only RIB functionality disappears. The engine, reactor, FSM, and all other plugins
continue to work.
<!-- source: internal/component/bgp/plugins/rib/register.go -- bgp-rib plugin registration -->
<!-- source: internal/component/plugin/registry/registry.go -- Registration struct -->

### Module tiers (where a package lives)

Because components and plugins register the same way, the directory a package lives
in is decided by **dependency direction**, not by the registration mechanism:
`internal/core/` for libraries (not config-driven engines), `internal/component/`
for platform plugins that other plugins depend on (BGP, iface, the RIB),
`internal/plugins/` for edge plugins nothing depends on (NTP, static, IS-IS,
OSPF). A config-driven engine (`sdk.NewWithConn`) in the wrong tier fails the
`./le tier check` gate, as does a new `internal/core/` import of
`internal/component/` or `internal/plugins/` (core is the leaf tier; the
grandfathered pairs live in `internal/le/tier/testdata/core_import_baseline.txt`).
Full rule and the audit tool:
[`ai/rules/architecture.md`](../ai/rules/architecture.md).
<!-- source: ai/rules/architecture.md -- tier taxonomy and the engine-placement gate -->
<!-- source: internal/le/tier/actions.go -- Answer -->
<!-- source: internal/plugins/ospf/register.go -- OSPF edge plugin registration -->

OSPF is also a Loc-RIB source named `ospf`: SPF inserts one path per equal-cost
next-hop for intra-area and inter-area candidates, then leaves sysrib/fibkernel as
the only kernel FIB owner.
<!-- source: internal/plugins/ospf/spf/install.go -- ProtocolID and Installer Apply -->
<!-- source: internal/plugins/ospf/spf/interarea.go -- ComputeInterArea -->

## Invocation Modes

| Mode | Syntax | Transport |
|------|--------|-----------|
| Internal | `internal bgp-rib` or `ze.pluginname` | Goroutine + `net.Pipe()` + DirectBridge |
| Fork (default for external) | `pluginname` | Subprocess, TLS connect-back |
| Direct | `ze-pluginname` | Sync in-process call |
| Path | `/path/to/binary` | External binary, TLS connect-back |

Internal plugins bypass IPC serialization via DirectBridge for hot-path
performance. External plugins connect back to the engine's TLS listener and
authenticate with a token (`ZE_PLUGIN_HUB_TOKEN`).
<!-- source: pkg/plugin/rpc/bridge.go -- DirectBridge implementation -->
<!-- source: pkg/plugin/rpc/mux.go -- MuxConn concurrent RPC multiplexer -->

## 5-Stage Startup Protocol

| Stage | Direction | RPC |
|-------|-----------|-----|
| 1. Declaration | Plugin -> Engine | `ze-plugin-engine:declare-registration` |
| 2. Config | Engine -> Plugin | `ze-plugin-callback:configure` |
| 3. Capability | Plugin -> Engine | `ze-plugin-engine:declare-capabilities` |
| 4. Registry | Engine -> Plugin | `ze-plugin-callback:share-registry` |
| 5. Ready | Plugin -> Engine | `ze-plugin-engine:ready` |

The SDK uses `MuxConn` for the startup connection and runtime RPCs. After Stage 5,
internal plugins can switch supported hot paths to DirectBridge; external plugins
continue using newline-framed YANG RPC over TLS. The wire format is
`#<id> <verb> [<json>]\n` with newline-delimited, UTF-8 messages and correlation IDs.
<!-- source: internal/component/plugin/server/server.go -- plugin server, handshake -->
<!-- source: pkg/plugin/sdk/sdk.go -- plugin SDK, MuxConn wrapping -->

## IPC Wire Format

```
#42 ze-bgp:peer-list {"selector":"10.0.0.1"}
#42 ok {"peers":[{"address":"10.0.0.1","state":"established"}]}
```

Methods use `module:rpc-name` format derived from YANG module names. All JSON
keys are kebab-case. Address families are `"afi/safi"` strings (e.g.,
`"ipv4/unicast"`, `"l2vpn/evpn"`).
<!-- source: docs/architecture/api/wire-format.md -- IPC wire format specification -->

## Config-Driven Loading

BGP itself is a config-driven plugin. If your config has a `bgp { }` section,
BGP loads automatically. Remove it, and ze starts without BGP. The same mechanism
works for `interface { }`, `firewall { }`, `traffic { control { } }`, and other
top-level sections. Plugins can be added or removed at runtime via config reload.

NLRI family plugins (bgp-nlri-evpn, bgp-nlri-vpn, etc.) are loaded automatically
when you configure the corresponding address family. You do not need to declare them.
<!-- source: internal/component/plugin/server/startup_autoload.go -- getConfigPathPlugins, autoLoadForNewConfigPaths -->
<!-- source: internal/component/bgp/plugins/nlri/ -- NLRI plugin registrations with Families field -->

## Shipped Plugins

`./ze show plugins` is the runtime source of truth for the complete registered
plugin list. The groups below mirror the current registrations in
`internal/component/plugin/all/all.go`.

### BGP Plugins

| Plugin | Purpose |
|--------|---------|
| `bgp-rib` | Route Information Base (received/sent routes, best-path tracking) |
| `bgp-adj-rib-in` | Adj-RIB-In with raw hex replay |
| `bgp-persist` | Route persistence across restarts |
| `bgp-rs` | Route server, client-to-client reflection (RFC 7947) |
| `bgp-rr` | Route reflector (RFC 4456) |
| `bgp-gr` | Graceful Restart (RFC 4724) + Long-Lived GR (RFC 9494) |
| `bgp-route-refresh` | Route Refresh handling (RFC 2918, RFC 7313) |
| `bgp-role` | BGP Role enforcement (RFC 9234) |
| `bgp-rpki` | RPKI origin validation (RFC 6811, RFC 8210). Declares `show bgp rpki` and the `summary` pipe alias over it |
| `bgp-rpki-decorator` | Merged UPDATE + RPKI events |
| `bgp-aigp` | Accumulated IGP Metric (RFC 7311) |
| `bgp-bmp` | BMP receiver and sender (RFC 7854, RFC 8671) |
| `bgp-watchdog` | Deferred route announcement with named groups |
| `bgp-hostname` | FQDN capability (code 73) |
| `bgp-softver` | Software version capability (code 75) |
| `bgp-llnh` | Link-local next-hop for IPv6 |
| `bgp-healthcheck` | Health-dependent route withdrawal |
| `bgp-redistribute` | Redistribute learned routes into system RIB |
| `redistribute-orchestrator` | Dispatch redistributed routes to registered protocol consumers (sources: `bgp`/`ibgp`/`ebgp`, `connected`, `static`, `kernel`, `l2tp`, `isis`, `as112`; consumers: `bgp`, `isis`) <!-- source: internal/plugins/as112/redistribute.go -- registerAS112Sources --> |
| `loop` | Route loop detection (RFC 4271 S9, RFC 4456 S8) |
<!-- source: internal/component/bgp/plugins/ -- BGP plugin implementations -->

### Filter Plugins

| Plugin | Purpose |
|--------|---------|
| `bgp-filter-community` | Community tag/strip filter (standard, large, extended) |
| `bgp-filter-aspath` | AS-path filter (regex + exact match) |
| `bgp-filter-aspath-length` | AS-path length filter |
| `bgp-filter-prefix` | Prefix-list filter |
| `bgp-filter-modify` | Attribute modification (local-pref, MED, origin, next-hop, communities, AIGP) |
| `bgp-filter-community-match` | Community match filter |
| `bgp-filter-remove-private-as` | Remove RFC 6996 private-use ASNs |
<!-- source: internal/component/bgp/plugins/filter_community/register.go -- filter plugins -->

### NLRI Family Plugins

| Plugin | Family |
|--------|--------|
| `bgp-nlri-evpn` | L2VPN EVPN (5 route types) |
| `bgp-nlri-flowspec` | FlowSpec |
| `bgp-nlri-vpn` | VPN |
| `bgp-nlri-vpls` | VPLS |
| `bgp-nlri-ls` | BGP-LS |
| `bgp-nlri-labeled` | MPLS labeled |
| `bgp-nlri-mup` | Mobile User Plane |
| `bgp-nlri-mvpn` | Multicast VPN |
| `bgp-nlri-rtc` | Route Target Constraint |
<!-- source: internal/component/bgp/plugins/nlri/ -- NLRI plugin implementations -->

### Infrastructure Plugins

| Plugin | Purpose |
|--------|---------|
| `bgp` | BGP routing daemon subsystem plugin |
| `rib` | System RIB: selects best route across protocols by admin distance |
| `static` | Static route management |
| `sysctl` | Kernel sysctl tuning |
| `routing-table` | Named routing table registry |
| `connected` | Connected route redistribution |
| `kernel` | Kernel route redistribution |
| `policy-routes` | Policy-based routing with nftables marks and ip rules |
| `fib-kernel` | FIB route installation via netlink |
| `fib-p4` | FIB route installation via P4 backend |
| `fib-vpp` | FIB route installation via VPP binary API |
| `firewall` | Firewall management via nftables |
| `flowspec-firewall` | BGP FlowSpec to nftables bridge |
| `flow-export` | sFlow, NetFlow v9, and IPFIX export |
| `traffic-usage` | eBPF TCX per-port/protocol (and opt-in per-IP) byte accounting |
| `traffic` | Traffic control (TC qdisc/class) |
| `interface` | Interface management via netlink or VPP |
| `iface-dhcp` | DHCP client for interface address assignment |
| `iface-ra` | IPv6 Router Advertisement sender for a LAN interface unit (RFC 4861) |
| `bfd` | BFD session management (RFC 5880, RFC 5881, RFC 5883) |
| `dhcpserver` | DHCP server for LAN/PXE clients |
| `imageserver` | HTTP image server for provisioning |
| `tftpserver` | Read-only TFTP server for PXE (RFC 2347 option negotiation) |
| `geodns` | GeoDNS server: DNS answers selected by client source IP (RFC 7871 client-subnet) |
| `as112` | AS112 anycast DNS node: authoritative sink for misdirected RFC 1918 / link-local reverse-DNS queries (RFC 7534, RFC 7535) |
| `ntp` | NTP time synchronization |
| `vpp` | VPP lifecycle and telemetry management |
| `ike` | IKEv2 engine for native IPsec VPN |
| `ldp` | Label Distribution Protocol |
| `rsvp-te` | RSVP-TE signaling |
| `l2tp-auth-local` | Local L2TP PPP authentication |
| `l2tp-auth-radius` | RADIUS authentication and accounting for L2TP PPP sessions |
| `l2tp-pool` | IPv4 and IPv6 pool allocation for L2TP PPP sessions |
| `l2tp-shaper` | Traffic shaping for L2TP subscriber sessions |
<!-- source: internal/plugins/ -- infrastructure plugin implementations -->

## Which Plugins Do I Need?

| Use case | Plugins |
|----------|---------|
| Announce routes to upstream | `bgp-rib` |
| Route server (IXP) | `bgp-rib` + `bgp-rs` + `bgp-adj-rib-in` |
| With RPKI validation | Add `bgp-rpki` + `bgp-adj-rib-in` |
| With graceful restart | Add `bgp-gr` |
| Service healthcheck | `bgp-healthcheck` + `bgp-watchdog` |
| Monitor only (no RIB) | None (peers connect, events fire, no routes stored) |
| Interface-aware BGP | `iface` + `bgp-rib` |
<!-- source: docs/guide/plugins.md -- Which Plugins Do I Need? section -->

## Further Reading

| Topic | Document |
|-------|----------|
| Writing plugins | [plugin-development/](plugin-development/) |
| Full plugin user guide | [guide/plugins.md](guide/plugins.md) |
| Architecture overview | [architecture.md](architecture.md) |
| IPC wire format | [architecture/api/wire-format.md](architecture/api/wire-format.md) |
| Plugin SDK | [architecture/api/architecture.md](architecture/api/architecture.md) |
| Plugin manager wiring | [architecture/plugin-manager-wiring.md](architecture/plugin-manager-wiring.md) |
