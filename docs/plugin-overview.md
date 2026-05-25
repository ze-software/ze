# Plugin Architecture

An overview of Ze's plugin system. For writing your own plugins, see
[plugin-development/](plugin-development/). For the full plugin user guide, see
[guide/plugins.md](guide/plugins.md).

## Concept

Everything beyond the core engine is a plugin. Plugins handle RIB storage, route
reflection, graceful restart, RPKI validation, NLRI encoding, FIB programming,
firewall rules, traffic control, and more. The engine discovers plugins through
registries and never imports plugin code directly.
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
works for `interface { }`, `firewall { }`, `traffic-control { }`, and other
top-level sections. Plugins can be added or removed at runtime via config reload.

NLRI family plugins (bgp-nlri-evpn, bgp-nlri-vpn, etc.) are loaded automatically
when you configure the corresponding address family. You do not need to declare them.
<!-- source: internal/component/plugin/server/startup_autoload.go -- getConfigPathPlugins, autoLoadForNewConfigPaths -->
<!-- source: internal/component/bgp/plugins/nlri/ -- NLRI plugin registrations with Families field -->

## Shipped Plugins

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
| `bgp-rpki` | RPKI origin validation (RFC 6811, RFC 8210) |
| `bgp-rpki-decorator` | Merged UPDATE + RPKI events |
| `bgp-aigp` | Accumulated IGP Metric (RFC 7311) |
| `bgp-watchdog` | Deferred route announcement with named groups |
| `bgp-hostname` | FQDN capability (code 73) |
| `bgp-softver` | Software version capability (code 75) |
| `bgp-llnh` | Link-local next-hop for IPv6 |
| `bgp-healthcheck` | Health-dependent route withdrawal |
| `bgp-bmp` | BMP monitoring station (RFC 7854) |
| `bgp-redistribute` | Redistribute learned routes into system RIB |
| `redistribute-orchestrator` | Dispatch redistributed routes to registered protocol consumers |
| `loop` | Route loop detection (RFC 4271 S9, RFC 4456 S8) |
<!-- source: internal/component/bgp/plugins/ -- BGP plugin implementations -->

### Filter Plugins

| Plugin | Purpose |
|--------|---------|
| `bgp-filter-community` | Community tag/strip filter (standard, large, extended) |
| `bgp-filter-aspath` | AS-path filter (regex + exact match) |
| `bgp-filter-prefix` | Prefix-list filter |
| `bgp-filter-modify` | Attribute modification (set LP, prepend, communities) |
| `bgp-filter-community-match` | Community match filter |
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
| `rib` | Shared route information base (system RIB) |
| `static` | Static route management |
| `sysctl` | Kernel sysctl tuning |
| `sysrib` | System RIB (route table management) |
| `fib-kernel` | FIB route installation via netlink |
| `fib-p4` | FIB route installation via P4 backend |
| `fib-vpp` | FIB route installation via VPP binary API |
| `firewall` | Firewall management via nftables |
| `traffic` | Traffic control (TC qdisc/class) |
| `interface` | Interface management via netlink or VPP |
| `iface-dhcp` | DHCP client for interface address assignment |
| `bfd` | BFD session management (RFC 5880, RFC 5881, RFC 5883) |
| `ntp` | NTP time synchronization |
| `vpp` | VPP lifecycle and telemetry management |
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
