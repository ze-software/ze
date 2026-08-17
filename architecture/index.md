# Ze Architecture

A one-page guide to how Ze is structured. For the full design document with
rationale, wire format details, and performance analysis, see
[DESIGN.md](https://github.com/ze-software/ze/blob/main/docs/DESIGN.md). For the canonical architecture reference, see
[architecture/core-design.md](https://github.com/ze-software/ze/blob/main/docs/architecture/core-design.md).

## What Ze Is

Ze is an open-source configuration and protocol engine. The network operating
system built on it speaks BGP, manages Linux network interfaces, programs the
FIB, and serves configuration over SSH and the web UI.

None of that is in the core. The core is a supervisor which holds a typed
message bus, a config provider, and a plugin manager. BGP, interface management
and the rest register as subsystems and plugins. Each one brings its own YANG
and augments the configuration tree where it belongs.
<!-- source: internal/component/engine/engine.go -- Engine supervisor -->
<!-- source: cmd/ze/hub/main.go -- primary YANG runtime composition -->

## Component Map

<figure class="doc-diagram">
  <a href="https://ze-software.net/assets/ze-components.svg">
    <img
      src="/assets/ze-components.svg"
      alt="Ze component architecture. Operator surfaces share one command and configuration contract. The protocol-agnostic Engine composes a ConfigProvider, Plugin Server and EventBus, and PluginManager. Registry-loaded routing and system plugins feed route selection and the FIB. An internal Dataplane group contains Netfilter, VPP, ARP, ND, and DHCP; Netfilter and VPP point to external Linux and VPP targets."
      width="1440"
      height="1000"
      loading="lazy"
      decoding="async"
    />
  </a>
  <figcaption>Ze components and their runtime relationships. Dashed outlines mark configured, build-dependent, or optional paths. Open the diagram for a full-size view; the text map below preserves the same structure for text readers.</figcaption>
</figure>

```
ze daemon (registry-driven YANG runtime)
  |
  +-- Engine (protocol-agnostic lifecycle supervisor)
  |     +-- ConfigProvider (YANG tree and transactions)
  |     +-- PluginManager / ProcessManager
  |     +-- Configured subsystems (PPPoE and L2TP)
  |
  +-- Plugin Server (shared runtime backbone)
  |     +-- Typed EventBus, subscriptions, and event history
  |     +-- YANG command / RPC dispatcher
  |     +-- Registry discovery and five-stage plugin startup
  |     +-- BGP reactor and feature plugins (RIB, RS/RR, policy, RPKI, ASPA, BMP)
  |     +-- OSPF
  |     +-- IS-IS
  |     +-- BFD
  |     +-- Loc-RIB -> sysrib -> FIB
  |     +-- Dataplane: Netfilter -> Linux; VPP -> VPP; ARP; ND; DHCP
  |     +-- Interface, firewall, traffic, and other ConfigRoot plugins
  |     +-- Internal plugins (goroutines, DirectBridge runtime path)
  |     +-- External plugins (managed processes, five-stage control over authenticated TLS)
  |
  +-- Northbound listeners
        +-- SSH CLI/editor, Web UI, and Looking Glass
        +-- REST, gRPC, gNMI, and MCP
        +-- Prometheus metrics and BMP routing telemetry
```
<!-- source: cmd/ze/hub/main.go -- hub wiring, component startup order -->

## Key Design Choices

| Principle | What it means |
|-----------|---------------|
| Wire-first | BGP messages are byte buffers, not parsed structs. Parsing is lazy via offset iterators. |
| Buffer-first encoding | All wire writes go into pooled, bounded buffers via `WriteTo(buf, off) int`. No `append()` in encoding. |
| Zero-copy forwarding | When source and destination peers share the same `ContextID` (negotiated capabilities hash), UPDATE bytes are forwarded unchanged. |
| Pool-based dedup | Per-attribute-type memory pools with refcounted handles. ORIGIN has 3 values; dedup saves memory at scale. |
| Registration pattern | Components and plugins register at startup via `init()`. Core discovers through registries, never imports directly. |
| YANG-modeled everything | Config schemas, CLI dispatch, plugin registration, and RPC discovery all flow from YANG modules. |
<!-- source: internal/component/bgp/wireu/wire_update.go -- WireUpdate lazy-parsed byte buffer -->
<!-- source: internal/component/bgp/attrpool/pool.go -- per-attribute-type dedup pools -->
<!-- source: internal/core/bgp/context/registry.go -- ContextID for zero-copy decisions -->
<!-- source: internal/component/config/yang/loader.go -- YANG-based config schema loading -->

## Key Wire Abstractions

| Abstraction | Purpose | Location |
|-------------|---------|----------|
| `WireUpdate` | Lazy-parsed BGP UPDATE: iterators over wire bytes, no intermediate structs | `internal/component/bgp/wireu/` |
| `EncodingContext` | Negotiated capabilities that determine encoding (ASN4, ADD-PATH, extended next hop) | `internal/core/bgp/context/` |
| `ContextID` | uint16 hash of capabilities. Same ID = forward wire bytes unchanged. | `internal/core/bgp/context/` |
| `Pool` / `Handle` | Per-attribute-type pools with refcounted handles and incremental compaction | `internal/component/bgp/attrpool/` |
| `DirectBridge` | Bypasses IPC serialization for internal plugins (direct function calls) | `pkg/plugin/rpc/` |
<!-- source: internal/component/bgp/wireu/wire_update.go -- WireUpdate -->
<!-- source: internal/core/bgp/context/context.go -- EncodingContext -->
<!-- source: internal/component/bgp/attrpool/handle.go -- Handle -->
<!-- source: pkg/plugin/rpc/bridge.go -- DirectBridge -->

## Plugin Server / EventBus

The Plugin Server is Ze's central runtime junction. It implements `ze.EventBus`
for typed publishers and subscribers, owns the YANG-derived command and RPC
dispatchers, coordinates plugin startup, and maintains subscription and event
history state. In-process subscribers receive typed values directly; plugin
process subscribers receive lazily encoded messages through the same fan-out.
There is no separate standalone bus in the primary YANG runtime.
<!-- source: internal/component/plugin/server/server.go -- shared command, subscription, and startup server -->
<!-- source: internal/component/plugin/server/engine_event.go -- typed EventBus fan-out -->
<!-- source: cmd/ze/hub/main.go -- Plugin Server supplied to Engine as EventBus -->

Components avoid importing each other's implementation packages. Internal
plugins bootstrap through `net.Pipe` and use `DirectBridge` for runtime calls;
external plugins connect back over authenticated TLS. Route-producing plugins
write selected paths into the shared Loc-RIB. `sysrib` performs system-wide
arbitration, then publishes typed best-change events to FIB consumers.
<!-- source: internal/component/plugin/process/process.go -- internal and external plugin transports -->
<!-- source: internal/component/sysrib/sysrib.go -- Loc-RIB arbitration and best-change publication -->
<!-- source: internal/plugins/fib/kernel/fibkernel.go -- FIB event consumer -->

## BGP Subsystem

The BGP subsystem owns the protocol: TCP connections, FSM state machines, wire
parsing, capability negotiation, and the reactor event loop. It produces
structured events and consumes commands. It never imports plugin code.

For a walk-through of the BGP state machine, see [bgp-fsm.md](https://github.com/ze-software/ze/blob/main/docs/bgp-fsm.md).
<!-- source: internal/component/bgp/reactor/reactor.go -- Reactor event loop -->
<!-- source: internal/component/bgp/fsm/fsm.go -- FSM state machine -->

## Configuration

JUNOS-like hierarchical syntax: `{}` blocks, `;` terminators, `#` comments.
YANG-driven parsing. Three-level inheritance: BGP globals, group defaults, peer
overrides. Configuration is stored in ZeFS (a blob store with commit/rollback)
and managed through an interactive editor accessible over SSH.

For the full config syntax reference, see [config-reference.md](https://github.com/ze-software/ze/blob/main/docs/config-reference.md).
<!-- source: internal/component/config/tokenizer.go -- tokenizer for JUNOS-like syntax -->
<!-- source: pkg/zefs/store.go -- ZeFS blob store -->

## Plugin Architecture

Most protocol and system features are registry-loaded plugins: RIB storage,
route reflection, graceful restart, RPKI validation, NLRI encoding, FIB
programming, firewall, traffic control, and more. PPPoE and L2TP are
Engine-managed subsystems. Internal plugins start as goroutines, bootstrap over
`net.Pipe`, and then use `DirectBridge` for structured runtime calls without
serialization. External plugins are managed child processes controlled through
the same five-stage protocol and runtime RPC over authenticated TLS.

For the plugin architecture overview, see [plugin-overview.md](https://github.com/ze-software/ze/blob/main/docs/plugin-overview.md).
For writing plugins, see [plugin-development/](https://github.com/ze-software/ze/tree/main/docs/plugin-development).
<!-- source: internal/component/plugin/registry/registry.go -- plugin registry -->
<!-- source: internal/component/plugin/process/process.go -- plugin execution modes -->
<!-- source: cmd/ze/hub/main.go -- configured subsystem registration -->

## Data Flow

```
Receive:  Network -> WireUpdate -> Reactor -> EventDispatcher -> Plugins
Announce: Text command -> ParseUpdate() -> WireUpdate -> Peer
Forward:  Cache lookup -> Egress filters -> Wire (zero-copy when ContextID matches)
```
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- receive path -->
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- forward path -->

The reactor fires a single `StructuredEvent` per received UPDATE. Forwarders
(route server, route reflector) and state trackers (RIB plugin) both subscribe
to the same dispatch but consume it differently: forwarders make per-peer
forwarding decisions, state trackers update best-path state.
<!-- source: internal/component/bgp/reactor/forward_pool.go -- per-destination forward workers -->

## FIB Pipeline

Selected routes move through a shared route-decision pipeline:

1. **Protocol route producers** insert BGP best paths and OSPF or IS-IS SPF paths into the shared Loc-RIB
2. **System RIB** observes Loc-RIB changes and selects system-wide routes by administrative distance and recursive next-hop resolution
3. **FIB consumers** receive typed `system-rib/best-change` events from the Plugin Server/EventBus
4. **Kernel FIB** programs OS routes through netlink or route sockets; optional VPP FIB uses the GoVPP API
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- BGP paths into Loc-RIB -->
<!-- source: internal/plugins/ospf/spf/install.go -- OSPF paths into Loc-RIB -->
<!-- source: internal/plugins/isis/spf/install.go -- IS-IS paths into Loc-RIB -->
<!-- source: internal/component/sysrib/sysrib.go -- system route arbitration -->
<!-- source: internal/plugins/fib/kernel/fibkernel.go -- kernel FIB consumer -->
<!-- source: internal/plugins/fib/vpp/fibvpp.go -- optional VPP FIB consumer -->

## Programs

| Binary | Purpose |
|--------|---------|
| `ze` | Network OS: BGP, CLI, config, hub, interface, ExaBGP migration, plugin, schema, signal, completion |
| `ze-chaos` | Chaos testing orchestrator: fault injection, scheduling |
| `ze-perf` | Performance benchmarking: UPDATE throughput tracking |
| `ze-analyze` | MRT/RIB analysis: attributes, communities, density, dump |
| `ze-test` | Functional test runner: BGP, editor, peer, MCP, web, RPKI, managed |
<!-- source: cmd/ze/main.go -- ze binary entry point -->

## Source Layout

| Area | Location |
|------|----------|
| Components | `internal/component/` (api, bgp, cli, config, firewall, gnmi, iface, ike, l2tp, lg, mcp, pki, resolve, ssh, storage, telemetry, traffic, vpp, web, ...) |
| BGP engine | `internal/component/bgp/` (reactor, FSM, wire, message, capability) |
| Plugin implementations | `internal/plugins/` and `internal/component/bgp/plugins/` |
| Plugin infrastructure | `internal/component/plugin/` (registry, process, hub, SDK) |
| Programs | `cmd/ze/` (build tags: `ze_core`, `ze_test`, `ze_chaos`, `ze_perf`, `ze_analyze`) |
| Public SDKs | `pkg/plugin/sdk/`, `pkg/plugin/rpc/`, `pkg/zefs/` |
| Tests | `test/` (.ci files), `*_test.go` |
<!-- source: cmd/ -- program binaries -->

## Deeper Reading

| Topic | Document |
|-------|----------|
| Full design document | [DESIGN.md](https://github.com/ze-software/ze/blob/main/docs/DESIGN.md) |
| Canonical architecture reference | [architecture/core-design.md](https://github.com/ze-software/ze/blob/main/docs/architecture/core-design.md) |
| System architecture (hub mode) | [architecture/system-architecture.md](system-architecture/index.md) |
| Buffer-first architecture | [architecture/buffer-architecture.md](https://github.com/ze-software/ze/blob/main/docs/architecture/buffer-architecture.md) |
| Pool architecture | [architecture/pool-architecture.md](https://github.com/ze-software/ze/blob/main/docs/architecture/pool-architecture.md) |
| Wire format details | [architecture/wire/](https://github.com/ze-software/ze/tree/main/docs/architecture/wire) |
| Hub architecture | [architecture/hub-architecture.md](https://github.com/ze-software/ze/blob/main/docs/architecture/hub-architecture.md) |
| Config syntax | [config-reference.md](https://github.com/ze-software/ze/blob/main/docs/config-reference.md) |
| BGP FSM | [bgp-fsm.md](https://github.com/ze-software/ze/blob/main/docs/bgp-fsm.md) |
| Plugin architecture | [plugin-overview.md](https://github.com/ze-software/ze/blob/main/docs/plugin-overview.md) |
| Feature inventory | [features.md](../reference/feature-status/index.md) |
