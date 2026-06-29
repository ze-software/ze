# Ze Architecture

A one-page guide to how Ze is structured. For the full design document with
rationale, wire format details, and performance analysis, see
[DESIGN.md](DESIGN.md). For the canonical architecture reference, see
[architecture/core-design.md](architecture/core-design.md).

## What Ze Is

Ze is a network operating system built in Go. A small, protocol-agnostic engine
supervises a message bus, a config provider, and a plugin manager. The engine has
no knowledge of BGP or any specific protocol. BGP, interface management, firewall,
traffic control, and everything else register as subsystems and plugins.
<!-- source: internal/component/engine/engine.go -- Engine supervisor -->

## Component Map

```
Engine (supervisor, lifecycle, config)
  |
  +-- Hub / Bus (content-agnostic message routing, []byte on hierarchical topics)
  |     |
  |     +-- BGP Subsystem (TCP, FSM, wire parsing, capability negotiation, reactor)
  |     +-- Interface component (netlink/VPP backend, DHCP, NTP)
  |     +-- Firewall component (nftables/VPP backend)
  |     +-- Traffic component (tc/VPP backend)
  |     +-- Config provider (YANG-parsed tree, transaction protocol)
  |     +-- Plugin infrastructure (registry, process manager, DirectBridge)
  |           |
  |           +-- bgp-rib, bgp-rs, bgp-gr, bgp-role, and many more
  |           +-- fib-kernel, sysrib, static, sysctl, ...
  |           +-- External plugins (any language, TLS connect-back)
  |
  +-- CLI (SSH-accessible editor and command shell)
  +-- Web UI (HTMX-based config editor, admin dashboard, SSE live updates)
  +-- Looking Glass (peer/route viewer, birdwatcher-compatible API)
  +-- Telemetry (Prometheus metrics, optional Basic Auth)
  +-- MCP server (Model Context Protocol for AI tool integration)
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
| `PackContext` | Negotiated capabilities that determine encoding (ASN4, ADD-PATH, ExtNH) | `internal/core/bgp/context/` |
| `ContextID` | uint16 hash of capabilities. Same ID = forward wire bytes unchanged. | `internal/core/bgp/context/` |
| `Pool` / `Handle` | Per-attribute-type pools with refcounted handles and incremental compaction | `internal/component/bgp/attrpool/` |
| `DirectBridge` | Bypasses IPC serialization for internal plugins (direct function calls) | `pkg/plugin/rpc/` |
<!-- source: internal/component/bgp/wireu/wire_update.go -- WireUpdate -->
<!-- source: internal/core/bgp/context/context.go -- PackContext -->
<!-- source: internal/component/bgp/attrpool/handle.go -- Handle -->
<!-- source: pkg/plugin/rpc/bridge.go -- DirectBridge -->

## Hub / Bus

The bus is the central notification router. It routes opaque `[]byte` payloads
on hierarchical topics (e.g., `bgp/update`, `interface/up`) with prefix-based
subscription matching. It is used for broadcast state changes and event fan-out;
request/response calls use the plugin dispatcher, DirectBridge, or typed package
interfaces. The bus never inspects payload content.
<!-- source: internal/component/hub/hub.go -- Hub message routing and pub/sub backbone -->

Components avoid importing each other's implementation packages. They communicate
through registries, bus topics, plugin RPC, DirectBridge, or shared core value
types depending on the direction and latency requirements. BGP subscribes to
interface events to react when addresses appear or disappear. The FIB pipeline
uses bus topics to carry best-path decisions from BGP RIB through system RIB to
kernel route installation.
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- best-path tracking via bus -->
<!-- source: internal/plugins/fib/kernel/fibkernel.go -- FIB route programming via bus -->

## BGP Subsystem

The BGP subsystem owns the protocol: TCP connections, FSM state machines, wire
parsing, capability negotiation, and the reactor event loop. It produces
structured events and consumes commands. It never imports plugin code.

For a walk-through of the BGP state machine, see [bgp-fsm.md](bgp-fsm.md).
<!-- source: internal/component/bgp/reactor/reactor.go -- Reactor event loop -->
<!-- source: internal/component/bgp/fsm/fsm.go -- FSM state machine -->

## Configuration

JUNOS-like hierarchical syntax: `{}` blocks, `;` terminators, `#` comments.
YANG-driven parsing. Three-level inheritance: BGP globals, group defaults, peer
overrides. Configuration is stored in ZeFS (a blob store with commit/rollback)
and managed through an interactive editor accessible over SSH.

For the full config syntax reference, see [config-reference.md](config-reference.md).
<!-- source: internal/component/config/tokenizer.go -- tokenizer for JUNOS-like syntax -->
<!-- source: pkg/zefs/store.go -- ZeFS blob store -->

## Plugin Architecture

All features beyond core engine operation are plugins: RIB storage, route
reflection, graceful restart, RPKI validation, NLRI encoding, FIB programming,
firewall, traffic control, and more. Plugins run in-process (goroutine +
DirectBridge) or as external processes (TLS connect-back in any language).

For the plugin architecture overview, see [plugin-overview.md](plugin-overview.md).
For writing plugins, see [plugin-development/](plugin-development/).
<!-- source: internal/component/plugin/registry/registry.go -- plugin registry -->
<!-- source: internal/component/plugin/all/all.go -- plugin blank imports -->

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

Best-path decisions flow through the bus:

1. **BGP RIB** detects best-path changes, publishes to `bgp-rib/best-change/bgp`
2. **System RIB** selects system-wide best by administrative distance, publishes to `system-rib/best-change`
3. **FIB Kernel** programs OS routes via netlink (`RTPROT_ZE=250`)
<!-- source: internal/component/sysrib/sysrib.go -- system RIB, admin distance selection -->
<!-- source: internal/plugins/fib/kernel/fibkernel.go -- RTPROT_ZE, netlink backend -->

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
| Components | `internal/component/` (api, bgp, cli, config, firewall, flowexport, gnmi, iface, ike, ipsec, l2tp, ldp, lg, mcp, pki, pppoe, resolve, rsvpte, ssh, storage, telemetry, traffic, vpp, web, ...) |
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
| Full design document | [DESIGN.md](DESIGN.md) |
| Canonical architecture reference | [architecture/core-design.md](architecture/core-design.md) |
| System architecture (hub mode) | [architecture/system-architecture.md](architecture/system-architecture.md) |
| Buffer-first architecture | [architecture/buffer-architecture.md](architecture/buffer-architecture.md) |
| Pool architecture | [architecture/pool-architecture.md](architecture/pool-architecture.md) |
| Wire format details | [architecture/wire/](architecture/wire/) |
| Hub architecture | [architecture/hub-architecture.md](architecture/hub-architecture.md) |
| Config syntax | [config-reference.md](config-reference.md) |
| BGP FSM | [bgp-fsm.md](bgp-fsm.md) |
| Plugin architecture | [plugin-overview.md](plugin-overview.md) |
| Feature inventory | [features.md](features.md) |
