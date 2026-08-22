# API Architecture

> **ARCHITECTURE:** API programs own ALL RIB data and logic.
> The Ze engine is a minimal BGP speaker - no RIB, no best-path, no policy.
> See `docs/architecture/rib-transition.md` for the full architecture.

## Implementation Status

| Feature | Status | Code Location |
|---------|--------|---------------|
| Process management | ✅ Done | `internal/component/plugin/process/process.go` |
| Backpressure (1000/100) | ✅ Done | `internal/component/plugin/process/process.go` |
| Respawn limits (5/60s) | ✅ Done | `internal/component/plugin/process/process.go` |
| Command dispatch | ✅ Done | `internal/component/plugin/server/command.go`, `internal/component/plugin/server/dispatch.go` |
| YANG API schema | ✅ Done | `internal/component/bgp/yang/`, `internal/core/ipc/yang/`, `internal/component/bgp/plugins/rib/yang/` |
| Plugin commands | ✅ Done | `internal/component/plugin/registration.go`, `internal/component/plugin/server/server.go` |
| Route injection | ✅ Done | `internal/component/bgp/plugins/rib/` |
| BGP cache commands | ✅ Done | `internal/component/plugin/server/subsystem.go` |
| Session sync | ✅ Done | `internal/component/plugin/server/session.go` |
| JSON/text encoding | ✅ Done | `internal/component/bgp/format/json.go` |
| RR plugin | ✅ Done | `internal/component/bgp/plugins/rs/` |
| RIB plugin | ✅ Done | `internal/component/bgp/plugins/rib/` |
| Adj-RIB-In plugin | ✅ Done | `internal/component/bgp/plugins/adj_rib_in/` |
| Shared BGP types | ✅ Done | `internal/component/bgp/` |
| borr/eorr markers | ✅ Done | RFC 7313 full support |
| Peer-to-process delivery index | ✅ Done | `internal/component/plugin/server/delivery_graph.go` |
| Send permission per peer | ✅ Done | `internal/component/bgp/reactor/send_permission.go` |
<!-- source: internal/component/plugin/server/server.go -- Server -->
<!-- source: internal/component/plugin/process/process.go -- Process -->

---

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Engine Role** | FSM, parsing, wire I/O, BGP cache |
| **API Role** | RIB storage, policy, best-path, GR state |
| **Communication** | YANG RPC wire format (`#<len>:<id> <verb> [json]`) with JSON events and DirectBridge for in-process hot paths |
| **Key Types** | `Server`, `Client`, `Process`, `Dispatcher` |
| **RIB** | Owned by API program (use `internal/component/bgp/rib/` as reference) |
| **Polyglot** | API programs can be Go, Python, Rust, etc. |
| **Cache Control** | API controls cache via `bgp cache` commands |
| **Who gets what** | The peer's `attach process` block. A peer that attaches no block for a program feeds it nothing and refuses its sends |

**When to read full doc:** Writing API programs, understanding engine/API split.

---

## RIB Ownership

**API programs own all RIB data and logic.** The engine is a minimal BGP speaker.

### Engine Responsibilities

| Component | Description |
|-----------|-------------|
| FSM | Per-peer state machine (Connect, OpenSent, etc.) |
| Parsing | Parse on demand (for API output) |
| Wire I/O | Read/write BGP messages |
| Capabilities | Negotiate with peers |
| BGP Cache | Store wire bytes, lifetime controlled by API via `bgp cache` commands |

### API Program Responsibilities

| Component | Description |
|-----------|-------------|
| RIB | Route storage (use `internal/component/bgp/rib/` as reference) |
| Pool | Attribute deduplication (see `POOL_ARCHITECTURE.md`) |
| Policy | Import/export filters, route manipulation |
| Best-path | Selection algorithm (if needed) |
| GR/RR | Graceful restart, route refresh handling |
| Cache Control | Retain/release/expire via `bgp cache retain/release/expire <id>` commands |

### Wire Bytes in Events

Engine sends wire bytes to API in IPC Protocol format (when `format full` is configured):

```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "peer": {"address": "10.0.0.1", "local": {"address": "10.0.0.2", "as": 65000}, "remote": {"address": "10.0.0.1", "as": 65001}},
    "update": {
      "message": {"id": 123, "direction": "received"},
      "attr": {"origin": "igp", "as-path": [65001]},
      "nlri": {"ipv4/unicast": [{"action": "add", "next-hop": "10.0.0.1", "nlri": ["10.0.0.0/24"]}]},
      "raw": {
        "attr": "40010100400200040001fde8",
        "nlri": {"ipv4/unicast": "180a0000"},
        "withdrawn": {}
      }
    }
  }
}
```

API decodes and stores in pool for deduplication.
<!-- source: internal/component/bgp/format/text_human.go -- appendFilterResultText -->

### BGP Cache Control (✅ IMPLEMENTED)

API controls cache lifetime via `bgp cache` commands:

```
bgp cache retain 123    # Keep until released
bgp cache release 123   # Allow eviction
bgp cache expire 123    # Remove immediately
bgp cache list          # List cached msg-ids
bgp cache forward 123 !10.0.0.1  # Forward to all except source
```

---

## Attaching a Process to a Peer

### The model

```
Process    = one program, declared once under plugin { }, started once
Attachment = one peer's relationship with that program, stated per peer
```

One process serves many peers, and each peer states its own relationship with
it. An attachment carries two independent directions:

| Direction | Leaf | Meaning |
|-----------|------|---------|
| Inbound | `receive [ ... ]` | the event types ze hands the program for this peer |
| Outbound | `send [ ... ]` | the message types the program may originate toward this peer |

A peer that attaches no block for a program is fed to it never, and refuses
every message the program aims at that peer. Silence is the default, and the
config is what breaks it. This holds for ze's own plugins as it holds for an
operator's program: a config that loads `bgp-rib` and attaches it to no peer
stores no route.

### Configuration Syntax

```
# The program, declared once. One process runs, whatever number of peers
# attach it.
plugin {
    external looking-glass {
        run ./looking-glass.py;
        encoder json;
    }
}

# The relationship, stated per peer.
peer 198.51.100.1 {
    attach process looking-glass {
        receive [ update-received state ]   # what the program is fed
        send    [ update ]                  # what it may originate here
    }
}
```

`receive` names an event type and the direction it is fed in. A plain type
means both directions, `update-received` names the UPDATEs the peer sends to
ze, and `update-sent` names the ones ze sends to the peer. `*` names every
registered type. `send` carries no direction, because every send type is sent.

### Groups and Dynamic Peers

A group's `attach process` block reaches every peer that group produces,
because the delivery index is built from RESOLVED peer settings and not from
the config document. Three cases and one rule:

| The peer | What it gets |
|----------|--------------|
| A member that states no block of its own | its group's list |
| A member that restates the block | its own list, which replaces the group's for that member alone |
| A peer created by a dynamic group | its group's list, under the address its connection arrived from |

A dynamic member is named by no config document: ze generates the name
`dyn-<address>` when it accepts the connection. The index needs no such name,
because it is keyed on the peer address and the member carries its group's
bindings by inheritance.
<!-- source: internal/component/bgp/reactor/reactor_dynamic.go -- buildDynamicPeerSettings -->
<!-- source: internal/component/bgp/config/resolve.go -- ResolveBGPTree -->

### Key Differences from ExaBGP

| Aspect | ExaBGP | Ze |
|--------|--------|-------|
| Keyword | `neighbor {` | `peer {` |
| Binding | `api { processes [foo]; }` | `attach process foo { ... }` in the peer |
| Direction | `receive { parsed; packets; }` | part of the type name: `update-received` |
| Output syntax | `neighbor X announce route ...` | `update text nlri <family> ...` |

### Data Flow: Config to Delivery

```
the config document
        │
        ▼ ResolveBGPTree: the group's block merged into each member
PeerSettings.ProcessBindings
        │
        ▼ DeliveryPeersFromSettings
Server.UpdateDeliveryGraph -> DeliveryGraph, swapped under one atomic pointer
        │
        ▼ Server.PeerScopedProcs
the seven peer-scoped delivery sites in bgp/server/events.go
```

The reactor pushes a new index after every peer change: `AddPeer`,
`doRemovePeer`, `createDynamicPeer`, `removeDynamicPeer`, the end of a
journaled config apply, and once inside `StartWithContext` before any peer
starts. A reader takes a pointer snapshot, so no reader sees a half-built index
and no surviving edge misses an event across a reload.

The lookup is one index read and it allocates nothing per event. It replaced a
scan of every process and every subscription on every delivered message.
<!-- source: internal/component/plugin/server/delivery_graph.go -- DeliveryGraph, PeerScopedProcs -->
<!-- source: internal/component/bgp/reactor/delivery_graph.go -- DeliveryPeersFromSettings, publishDeliveryGraphLocked -->

The outbound half is enforced at the command rather than in the index: each peer
a command reached is asked whether it grants this process the message type that
command puts on the wire. A peer the command names and the permission refuses is
dropped and reported. A command whose every peer refuses fails. Both halves read
the same resolved settings, so `show event delivery` cannot disagree with what
the daemon does.

Ten commands reach a peer's wire and every one applies that check, but they do
not all get there the same way, and reading "the selector resolver" as the whole
guard is what left four of them open until the review that found it. Six resolve
their peers through `getMatchingPeersSel`: announce, withdraw, End-of-RIB,
commit, route refresh and soft clear. Four do not. `cache forward` parses a
selector of its own and matches it inside `ForwardUpdate`; the `forward-cached`
and `relay-stored-route` plugin RPCs and `peer <addr> raw` name their
destinations directly, as an address list, one address, or one peer. All ten
share one filter, `filterPermittedPeers`.

Raw is gated on ATTACHMENT rather than on a send type. It carries a whole BGP
message the caller chose, so no `send` word describes it; the peer must attach
the process, and any `attach process <name> { }` block does.
<!-- source: internal/component/bgp/reactor/send_permission.go -- Peer.maySend, sendOrigin, filterPermittedPeers -->
<!-- source: internal/component/bgp/reactor/reactor_api.go -- getMatchingPeersSel, SendRawMessage -->
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- ForwardUpdate -->

### What the Plugin Declares and What the Peer Grants

A plugin declares what it CAN handle, in its ready RPC. The peer's config
decides what it GETS. A peer-scoped event reaches a process when both halves
name it. At plugin ready, and again after every config apply, ze names each
peer, process and event type the two halves disagree about, and says nothing
when they agree.

A process that NO peer attaches gets its own line, because it has no peer to
name: it declared events and no `attach process` block anywhere in the config
mentions it, so no peer-scoped event reaches it. Loading a plugin does not
attach it. A config path that starts one (`rs-client`, `route-reflector-client`,
`watchdog { }`, a custom receive token) creates no delivery edge, so the peers
it must serve MUST also name it.
<!-- source: internal/component/plugin/server/delivery_reconcile.go -- deliveryDisagreements -->

A `request subscribe` typed at a running daemon is a live capability override.
It can add an event type the process did not declare at startup only when the
peer's `attach process` block grants that type. It cannot widen the configured
receive authorization, and the next config apply discards it.
<!-- source: internal/component/plugin/server/delivery_graph.go -- (*Server).PeerScopedProcs, (*Server).DiscardRuntimeSubscriptions -->

### Encoding

Encoding belongs to the PROCESS, not to the attachment. It comes from the
`encoder` leaf in the program's own `plugin { }` block, and the ready RPC can
set it. One process therefore has one encoding for every peer it serves.

The `content { encoding format }` block inside an attachment parses and is not
read. Honoring it per peer needs a per-peer format cache, which is a separate
change with its own design question.
<!-- source: internal/component/config/loader.go -- ExtractPluginsFromTree -->
<!-- source: internal/component/plugin/process/process.go -- Process.SetEncoding -->

---

## Overview

The Ze API system enables external route injection and daemon control via:
- SSH connections (CLI tools)
- Subprocess management (external route generators)

## Package Structure

```
internal/component/plugin/
├── registration.go        # Runtime registration types for the 5-stage protocol
├── resolve.go             # Config-to-plugin resolution helpers
├── inprocess.go           # Internal plugin runner lookup
├── registry/registry.go   # Compile-time plugin registry
├── process/process.go     # Internal and external process lifecycle
├── process/delivery.go    # Event queue and batch delivery
├── server/server.go       # Plugin server lifecycle
├── server/startup.go      # Five startup phases and 5-stage handshake
├── server/dispatch.go     # Plugin-to-engine RPC dispatch
├── server/rpc_register.go # Built-in RPC handler registry
├── server/events.go       # Event and NLRI routing
├── ipc/rpc.go             # Engine-side PluginConn wrapper
├── ipc/tls.go             # External TLS connect-back transport
├── schema/                # ze-plugin-conf.yang
└── all/all.go             # Generated blank imports for internal plugins

pkg/plugin/
├── rpc/conn.go            # Newline-framed RPC connection
├── rpc/mux.go             # Bidirectional request/response multiplexer
├── rpc/bridge.go          # DirectBridge for in-process plugins
└── sdk/                   # Callback-based SDK for plugin authors

internal/core/ipc/
├── dispatch.go            # RPCDispatcher (wire-method exact-match dispatch)
├── message.go             # Request/Response/Error types
└── schema/                # ze-system, ze-plugin, and command YANG API modules

internal/component/config/yang/
├── rpc.go                 # Extract RPCs/notifications from YANG Entry tree
└── loader.go              # YANG module loader
```

### Plugin Auto-Loading

Internal plugins are discovered at compile time through Go's `init()` mechanism, not runtime scanning. The chain has three layers:

```
cmd/ze/main.go
  │  _ "internal/component/plugin/all"     (blank import)
  ▼
internal/component/plugin/all/all.go       (generated)
  │  _ "internal/component/bgp/plugins/rib"
  │  _ "internal/component/bgp/plugins/gr"
  │  _ "internal/plugins/fib/kernel"
  │  _ "internal/component/iface"
  │  _ ...                                  (one blank import per plugin)
  ▼
internal/component/bgp/plugins/<name>/register.go or internal/plugins/<name>/register.go
  │  func init() { registry.Register(Registration{...}) }
  ▼
internal/component/plugin/registry/registry.go
      plugins map[string]*Registration     (global, query at runtime)
```

**Why the blank import lives in `cmd/ze/main.go`:** Placing it deeper (e.g., in `internal/component/plugin/`) would create import cycles, because some plugins depend on packages like `format` that themselves depend on `plugin`.

**Registry is a leaf package:** `registry/` has zero dependencies on plugin implementations. Plugins import the registry to register; consumers import the registry to query. This one-directional flow prevents cycles.
<!-- source: internal/component/plugin/registry/registry.go -- Registration, Lookup, All -->

**Code generation keeps `all.go` in sync:** `scripts/codegen/plugin_imports.go` (invoked via `make generate`) walks plugin implementation roots such as `internal/component/bgp/plugins/`, `internal/component/`, and `internal/plugins/` for `register.go` files that import the plugin registry, and separately discovers infrastructure `schema/register.go` files that import `config/yang`. It writes the sorted blank-import list to `all.go`.

**Adding a new plugin:**

1. Create the plugin near its domain, for example `internal/component/bgp/plugins/<name>/register.go` for BGP plugins or `internal/plugins/<name>/register.go` for generic infrastructure plugins, with an `init()` calling `registry.Register()`
2. Run `make generate` to regenerate `all.go`
3. The plugin is now auto-loaded in every binary that imports `plugin/all`

No other wiring is needed. The engine discovers it through registry queries, the CLI dispatches via `CLIHandler`, YANG schemas are picked up automatically, and dependency resolution handles startup ordering.

**Registration struct fields:** Each plugin provides its name, handlers (`RunEngine`, `CLIHandler`), and optional metadata: address families, capability codes, dependencies, YANG schema, event types, and in-process codec functions. See `registry/registry.go` for the full `Registration` type.

**Key registry queries used at runtime:**

| Function | Consumer | Purpose |
|----------|----------|---------|
| `Lookup(name)` | Engine startup | Get a specific plugin's registration |
| `All()` | CLI help, inventory | All registered plugins (sorted) |
| `FamilyMap()` | Config loader | Map address families to plugin names |
| `CapabilityMap()` | Wire decoder | Map capability codes to plugin names |
| `DecodeNLRIByFamily()` | `ze bgp decode` | Fast-path NLRI decoding (no RPC) |
| `YANGSchemas()` | YANG loader | All YANG schemas for CLI generation |
| `ResolveDependencies()` | Engine startup | Expand dependency graph (with cycle detection) |
| `TopologicalTiers()` | Engine startup | Order plugins for startup (Kahn's algorithm) |
<!-- source: internal/component/plugin/registry/registry.go -- FamilyMap, CapabilityMap, YANGSchemas, ResolveDependencies, TopologicalTiers -->

### YANG API Modules

Each YANG module defines RPCs and notifications for a domain. Every RPC maps 1:1 to a handler function via `RPCRegistration`:

| Module | Location | Contains |
|--------|----------|----------|
| `ze-bgp-api` | `internal/component/bgp/yang/` | BGP daemon, peer, monitor, and decode RPCs |
| `ze-bgp-cmd-log-api` | `internal/component/cmd/log/yang/` | Log command RPCs |
| `ze-bgp-cmd-metrics-api` | `internal/component/cmd/metrics/yang/` | Metrics command RPCs |
| `ze-system-api` | `internal/core/ipc/yang/` | System RPCs |
| `ze-plugin-api` | `internal/core/ipc/yang/` | Plugin lifecycle RPCs |
| `ze-rib-api` | `internal/component/bgp/plugins/rib/yang/` | RIB RPCs and notifications |
| `ze-plugin-engine` | `internal/core/ipc/yang/` | Engine RPCs served to plugins |
| `ze-plugin-callback` | `internal/core/ipc/yang/` | Callback RPCs served by plugins |

Wire methods use `module:rpc-name` format with `-api` suffix stripped (e.g., `ze-bgp-api` defines `ze-bgp:peer-list`). This is done by `WireModule()` in `internal/component/config/yang/rpc.go`.
<!-- source: internal/component/config/yang/rpc.go -- WireModule -->

### Handler Registration

Handlers are organized by domain, each file providing a `*RPCs()` function:

| File | Function | Module |
|------|----------|--------|
| `bgp.go` | `PeerOpsRPCs()` + `IntrospectionRPCs()` | ze-bgp |
| `bgp_summary.go` | `SummaryRPCs()` | ze-bgp |
| `system.go` | `systemRPCs()` | ze-system |
| `rib_handler.go` | `ribRPCs()` | ze-rib |
| `session.go` | `sessionRPCs()` | ze-plugin |
| `plugin.go` | `pluginRPCs()` | ze-plugin |

`AllBuiltinRPCs()` in `command.go` aggregates all registered handlers into a flat `[]RPCRegistration` slice.
<!-- source: internal/component/plugin/server/command.go -- AllBuiltinRPCs -->
<!-- source: internal/component/plugin/server/handler.go -- RPCRegistration -->

## Monitor Streaming

The `ze-bgp:monitor` RPC provides live BGP event streaming over SSH sessions. Unlike request/response RPCs, monitor keeps the SSH session open and writes events line-by-line as they occur.

| Component | Location | Purpose |
|-----------|----------|---------|
| `StreamingExecutorFactory` | SSH server | Detects monitor commands and returns a streaming executor instead of a one-shot executor |
| `MonitorManager` | `internal/component/plugin/` (Server) | Manages active monitor clients (add, remove, broadcast) |
| Event delivery | All 6 event functions | After delivering events to plugins, each event function also delivers to active monitor clients |

Monitor supports filtering by peer address, event type, and direction. Pipe operators (`| json`, `| table`, `| match`) are applied server-side before streaming to the client.
<!-- source: internal/component/plugin/server/monitor.go -- MonitorManager -->
<!-- source: internal/component/ssh/ssh.go -- StreamingExecutorFactory -->

## Identity and Authorization

### REST and gRPC authentication flow

At boot, `runYANGConfig` loads the zefs user snapshot once. It creates
`liveLocalUsers` after the config provider contains the running tree.
`liveLocalUsers` merges that snapshot with `system.authentication.user`.
A config user replaces a same-name zefs user. Only surviving zefs users retain
their recovery profile.
<!-- source: cmd/ze/hub/main.go -- runYANGConfig -->
<!-- source: cmd/ze/hub/main_servers.go -- liveLocalUsers, mergeAuthUsers, usersFromZefsDB -->

`runYANGConfig` calls the live source once to produce the shared boot snapshot.
A source error stops startup before AAA installation or management listener
construction.
<!-- source: cmd/ze/hub/main.go -- runYANGConfig boot user resolution -->

The first BGP or no-BGP construction builds and publishes one boot-owned AAA
bundle before command dispatch. A later BGP infrastructure hook reuses that
bundle, so open sessions and in-flight accounting never cross a backend close.
Each accepted API generation binds its local policy store as the bundle's local
fallback. External authorizers keep registry priority, and TACACS+ receives the
same local fallback.
<!-- source: cmd/ze/hub/main.go -- runYANGConfig -->
<!-- source: cmd/ze/hub/infra_setup.go -- boot-owned bundle construction and reuse -->
<!-- source: cmd/ze/hub/aaa_lifecycle.go -- claimAAABundleBoot, acceptedLocalGenerationAuthorizer -->
<!-- source: internal/component/aaa/types.go -- Bundle.AuthorizerWithLocalFallback -->
<!-- source: internal/component/tacacs/authorizer.go -- TacacsAuthorizer.BindLocalFallback -->

`newAcceptedLocalIdentity` calls `buildAPIAuthentication` once for each
candidate generation. The builder wraps that generation's fixed users with
`WithProfileAuthorizer`. A successful result carries the same generation's
authorization view in `AuthResult.Authorizer`, beside the authenticated
username. REST and gRPC obtain the accepted `api.Authentication` snapshot once
for each request.
<!-- source: cmd/ze/hub/aaa_lifecycle.go -- newAcceptedLocalIdentity, liveAcceptedAPIAuthentication -->
<!-- source: cmd/ze/hub/api.go -- buildAPIAuthentication -->
<!-- source: internal/component/api/types.go -- Authentication, AuthenticationProvider -->
<!-- source: internal/component/aaa/login_profiles.go -- WithProfileAuthorizer, AuthorizerForResult -->

REST and gRPC use the same authentication precedence and authorization
identities:

| Mode | Transport producer | Authorization identity | Authority |
|------|--------------------|------------------------|-----------|
| Per-user | REST `withAuth`, gRPC `checkAuth` | Authenticated username and result authorizer | Assigned or authentication-resolved profiles |
| Shared token | REST `withAuth`, gRPC `checkAuth` | `ReservedSharedAPIUsername` | Read and write |
| No auth | REST `withAuth`, gRPC `checkAuth` | `ReservedSharedAPIUsername` | Read-only |

REST `callerIdentity` publishes the trusted request identity and its authorizer.
gRPC returns the same classification from `checkAuth`. Both transports attach
that identity to the request context before command dispatch. Concurrent
requests with the same username cannot replace each other's authorization
view. The read-only check rejects no-auth writes before `Store.Authorize`
permits the shared identity.
<!-- source: internal/component/api/rest/auth.go -- RESTServer.withAuth, RESTServer.callerIdentity -->
<!-- source: internal/component/api/grpc/server.go -- GRPCServer.checkAuth -->
<!-- source: internal/component/plugin/dispatch.go -- WithCallerAuthorizer, CallerAuthorizer -->
<!-- source: internal/component/aaa/reserved.go -- ReservedSharedAPIUsername -->
<!-- source: internal/component/authz/authz.go -- Store.Authorize, Store.AuthorizeWithProfiles -->

On reload, `runReloadContext` applies the candidate tree to the config provider.
It resolves candidate users through the accepted generation's resolver. It then
passes those users, the candidate authorization store, and the API token to
`newAcceptedLocalIdentity`. The new state stays unpublished while remaining
steps can fail.

During listener migration, REST and gRPC use a fail-closed staging view.
`apiAuthReloader` only classifies whether the candidate exposure is
authenticated. It does not build or publish an authenticator. After every reload
step succeeds, `runReloadContext` atomically publishes the complete state. A
failed listener migration restores the prior API generation after rollback. A
failed listener rollback leaves API authentication fail closed.
<!-- source: cmd/ze/hub/main_reload.go -- runReloadContext -->
<!-- source: cmd/ze/hub/aaa_lifecycle.go -- newAcceptedLocalIdentity, stageAPIAuthentication, publishAcceptedLocalIdentity -->
<!-- source: cmd/ze/hub/mgmt_auth_reload.go -- apiAuthReloader -->

REST constructors and reconfiguration reject every non-loopback address. gRPC
permits a non-loopback address only when authentication and TLS are both
configured.
<!-- source: internal/component/api/rest/server.go -- NewRESTServer, RESTServer.Reconfigure -->
<!-- source: internal/component/api/grpc/server.go -- NewGRPCServer, checkGRPCListenAddr -->

Shared local users belong to `acceptedLocalIdentityState`.
`SSHExtractedConfig` contains only SSH listener settings. An `environment.ssh`
block does not produce API users or API AAA.
<!-- source: cmd/ze/hub/aaa_lifecycle.go -- acceptedLocalIdentityState -->
<!-- source: internal/component/config/infra/hook.go -- SSHExtractedConfig -->

### Other command transports

User identity for authorization is injected by the transport layer and is
never accepted from the client payload:

| Transport | Identity source |
|-----------|----------------|
| SSH session | SSH authenticated username |
| Plugin (internal) | Plugin process name (engine-trusted) |
| Plugin (external) | Plugin auth token (TLS handshake) |
| Unix socket | OS peer credentials |

The `RPCParams` struct does not carry a username field. Authorization checks use
the `CommandContext.Username` that the server sets from the transport session.
<!-- source: internal/component/plugin/server/command.go -- CommandContext -->

## Connection Types

### Socket Clients

`Server` itself owns no listener: plugin processes reach it over their own
pipes (see Subprocess below). The socket path in this package is
`ManagedServer`, which serves managed fleet clients over TLS.

```go
type ManagedServer struct {
    svc       *ManagedConfigService
    addrs     []string
    cert      tls.Certificate
    conns     map[string]*rpc.MuxConn // Connected client name -> mux
    listeners []net.Listener
    // ... plus lookup, mu, notifyCh, sem, wg, ctx
}
```

Flow: `acceptLoop()` → `handleConn()` → `serve()` → `handleRequest()`. One
goroutine per connection, bounded by `managedMaxConns`.
<!-- source: internal/component/plugin/server/server.go -- Server, NewServer -->
<!-- source: internal/component/plugin/server/managed_serve.go -- ManagedServer, acceptLoop, handleConn, handleRequest -->

### Subprocess (Process)

```go
type Process struct {
    config ProcessConfig
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout io.ReadCloser

    // Session state
    syncEnabled atomic.Bool    // Wait for wire transmission

    // Backpressure
    writeQueue   chan []byte
    queueDropped atomic.Uint64
}
```

Features:
- Per-process session state (sync mode)
- Write queue with backpressure (high: 1000, low: 100)
- Respawn limits (max 5 per 60 seconds)
- **ACK controlled by serial prefix** (`#N` in command)
<!-- source: internal/component/plugin/process/process.go -- Process -->

### Plugin IPC Protocol (YANG RPC)

The plugin IPC layer replaces stdin/stdout text pipes with YANG RPC calls over a single bidirectional connection per plugin. MuxConn multiplexes plugin-initiated and engine-initiated RPCs by distinguishing responses (verb=ok/error) from requests (verb=method name).

```
pkg/plugin/rpc/
├── conn.go           # rpc.Conn -- newline-framed RPC connection
├── mux.go            # MuxConn -- bidirectional RPC multiplexer
└── types.go          # Canonical wire-format types (DeclareRegistrationInput, etc.)

pkg/plugin/sdk/
└── sdk.go            # Plugin SDK -- callback-based API for plugin authors

internal/component/plugin/ipc/
├── tls.go            # TLS transport, auth, PluginAcceptor (external)
└── rpc.go            # PluginConn (typed stage methods, MuxConn shadowing)

internal/core/ipc/yang/
├── ze-plugin-engine.yang    # RPCs engine serves (startup, routes, dispatch, subscriptions, decode/encode)
└── ze-plugin-callback.yang  # RPCs plugin serves (configure, deliver-event, bye, etc.)
```

**Transport:**

| Plugin Type | Transport | Connection Model |
|-------------|-----------|------------------|
| Internal (goroutine) | net.Pipe + DirectBridge | Single MuxConn (bidirectional) |
| External (subprocess) | TLS connect-back | Single MuxConn (bidirectional) |
<!-- source: pkg/plugin/rpc/mux.go -- MuxConn -->
<!-- source: pkg/plugin/rpc/bridge.go -- DirectBridge -->
<!-- source: internal/component/plugin/ipc/tls.go -- TLS transport -->

**5-stage startup preserved as typed RPCs:**
1. Plugin calls `declare-registration`
2. Engine calls `configure`
3. Plugin calls `declare-capabilities`
4. Engine calls `share-registry`
5. Plugin calls `ready`
<!-- source: pkg/plugin/rpc/types.go -- DeclareRegistrationInput -->
<!-- source: internal/component/plugin/ipc/rpc.go -- PluginConn -->

## Route Injection Flow

### Unicast Routes

```
API Client
    │ "update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24"
    ▼
Dispatcher.handleUpdate()
    │ Parse attributes, validate keywords
    ▼
Reactor.AnnounceRoute(peerSelector, RouteSpec)
    │ getMatchingPeers(), build NLRI
    ▼
Per Peer:
    ├─ InTransaction? → Adj-RIB-Out.QueueAnnounce()
    ├─ Established?   → SendUpdate() + MarkSent()
    └─ Down?          → opQueue (send when up)
```

### Labeled-Unicast Routes (SAFI 4)

```
API Client
    │ "announce ipv4/nlri-mpls 10.0.0.0/24 label 100 next-hop 1.2.3.4 path-id 42"
    ▼
Dispatcher.announceLabeledUnicastImpl()
    │ parseLabeledUnicastAttributes() - validates MPLSKeywords
    ▼
Reactor.AnnounceLabeledUnicast(peerSelector, LabeledUnicastRoute)
    │
    ├─ buildLabeledUnicastRIBRoute()
    │      Creates rib.Route with:
    │      - nlri.LabeledUnicast (prefix + labels + pathID)
    │      - ALL attributes (Origin, MED, LocalPref, Communities, etc.)
    │      - AS_PATH (empty for iBGP, LocalAS prepend for eBGP)
    ▼
Per Peer:
    ├─ InTransaction? → Adj-RIB-Out.QueueAnnounce(ribRoute)
    │                   Queued for commit
    │
    ├─ Established?   → buildLabeledUnicastParams() → BuildLabeledUnicast()
    │                   SendUpdate() + MarkSent(ribRoute)
    │                   Tracks for re-announcement on reconnect
    │
    └─ Down?          → peer.QueueAnnounce(ribRoute)
                        Sent when session establishes
```

### LabeledUnicastRoute Structure

```go
type LabeledUnicastRoute struct {
    Prefix  netip.Prefix  // IP prefix
    NextHop netip.Addr    // Next-hop address
    Labels  []uint32      // MPLS label stack
    PathID  uint32        // ADD-PATH identifier (RFC 7911)
    PathAttributes        // Origin, MED, Communities, etc.
}
```

### Key Differences from UnicastRoute

| Feature | UnicastRoute | LabeledUnicastRoute |
|---------|--------------|---------------------|
| SAFI | 1 (Unicast) | 4 (MPLS Label) |
| NLRI type | `nlri.INET` | `nlri.LabeledUnicast` |
| Labels | None | `[]uint32` (RFC 8277) |
| PathID | Not in API type | `uint32` (RFC 7911) |
| Attribute storage | Only OriginIGP ⚠️ | ALL attributes ✅ |
| Keyword set | UnicastKeywords | MPLSKeywords |

**Note:** The unicast route flow has a bug where only OriginIGP is stored in rib.Route.
Labeled-unicast correctly stores ALL attributes for proper queue replay.
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- handleUpdateText -->

## RouteSpec Structure

```go
type RouteSpec struct {
    Prefix  netip.Prefix
    NextHop RouteNextHop  // Encapsulates next-hop policy (explicit or self)
    PathAttributes
}

// RouteNextHop encapsulates next-hop policy for route origination.
// Resolution happens at peer level where negotiated capabilities are known.
type RouteNextHop struct {
    Policy NextHopPolicy  // NextHopUnset, NextHopExplicit, or NextHopSelf
    Addr   netip.Addr     // Valid only when Policy == NextHopExplicit
}

type PathAttributes struct {
    Origin              *uint8
    LocalPreference     *uint32
    MED                 *uint32
    ASPath              []uint32
    Communities         []uint32
    LargeCommunities    []LargeCommunity
    ExtendedCommunities []attribute.ExtendedCommunity
}
```
<!-- source: internal/component/bgp/types/types.go -- RouteSpec -->
<!-- source: internal/component/bgp/types/nexthop.go -- RouteNextHop -->

### Next-Hop Resolution

`RouteNextHop` is resolved at **peer level** in `internal/component/bgp/reactor/peer.go` via `resolveNextHop()`:
<!-- source: internal/component/bgp/reactor/peer.go -- resolveNextHop, ErrNextHopUnset, ErrNextHopSelfNoLocal, ErrNextHopIncompatible -->

| Policy | Behavior |
|--------|----------|
| `NextHopExplicit` | Returns configured address (no validation) |
| `NextHopSelf` | Returns `peer.settings.LocalAddress`, validates capability |
| `NextHopUnset` | Returns `ErrNextHopUnset` |

**Errors:**
- `ErrNextHopUnset` - zero value `RouteNextHop`
- `ErrNextHopSelfNoLocal` - Self policy but `LocalAddress` not configured
- `ErrNextHopIncompatible` - Self address incompatible with NLRI family (no Extended NH)

**Extended Next Hop (RFC 5549/8950):** Cross-family next-hop (e.g., IPv6 next-hop for IPv4 NLRI) allowed when `peer.sendCtx.ExtendedNextHopFor(family) != 0`.

## Update Text Parser

The `ParseUpdateText` function parses the "update text" command format for batch route operations:
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- ParseUpdateText -->

```
<section>*
<section>     := <scalar-attr> | <list-attr> | <nlri-section> | <wire-attr>

<scalar-attr> := <scalar-name> (set <value> | del [<value>])
<scalar-name> := origin | med | local-preference | nhop | path-information | rd | label

<list-attr>   := <list-name> (set <list> | add <list> | del [<list>])
<list-name>   := as-path | community | large-community | extended-community

<nlri-section> := nlri <family> <nlri-op>+
<nlri-op>      := add <prefix>+ [watchdog set <name>] | del <prefix>+

<wire-attr>    := attr (set <bytes> | del [<bytes>])   # hex/b64 mode only
```

Standalone watchdog commands (separate from update text):
```
request bgp watchdog announce <name>   # send all routes in pool to peers
request bgp watchdog withdraw <name>   # withdraw all routes in pool from peers
```

### Result Types

```go
type UpdateTextResult struct {
    Groups       []NLRIGroup  // Each nlri section produces a group
    WatchdogName string       // Optional watchdog pool name
}

type NLRIGroup struct {
    Family   family.Family    // ipv4/unicast, ipv6/unicast, etc.
    Announce []nlri.NLRI    // Prefixes to announce
    Withdraw []nlri.NLRI    // Prefixes to withdraw
    Attrs    PathAttributes // Snapshot of attributes at this point
    NextHop  RouteNextHop   // Encapsulates next-hop policy (explicit or self)
}
```
<!-- source: internal/component/bgp/types/types.go -- UpdateTextResult, NLRIGroup -->

### YANG-Driven Attribute Validation

Attribute values in the update text parser are validated against the YANG schema (`ze-bgp-conf.yang`), making YANG the single source of truth for data validation. The `ValueValidator` interface provides the validation, set via `SetYANGValidator()`.
<!-- source: internal/component/plugin/validator.go -- ValueValidator, SetYANGValidator -->

| Attribute | YANG Path | YANG Type | Validation |
|-----------|-----------|-----------|------------|
| `origin` | `bgp.peer.update.attribute.origin` | `enumeration {igp, egp, incomplete}` | Enum check |
| `med` | `bgp.peer.update.attribute.med` | `uint32` | Range check |
| `local-preference` | `bgp.peer.update.attribute.local-preference` | `uint32` | Range check |

The CLI decode path (`ze bgp decode --nlri`) also validates family strings against known AFI/SAFI combinations and hex inputs before dispatching to plugin decoders.

### Key Semantics

- **Attribute accumulation:** Attribute sections accumulate; each `nlri` section captures a snapshot
- **Deep copy:** Each group gets independent copies of attributes (slices AND pointers)
- **Supported families:** `ipv4/unicast`, `ipv6/unicast`, `ipv4/multicast`, `ipv6/multicast`
- **Case-sensitive:** Family strings must be lowercase

### Example

```
origin set igp
nhop set 192.0.2.1
community set [65000:100]
nlri ipv4/unicast add 10.0.0.0/24 10.0.1.0/24 del 10.0.2.0/24
community add [65000:200]
nlri ipv6/unicast add 2001:db8::/32
watchdog pool1
```

Produces 2 groups:
1. IPv4 group with community `[65000:100]`, 2 announce, 1 withdraw
2. IPv6 group with communities `[65000:100, 65000:200]`, 1 announce

### FlowSpec NLRI (RFC 8955)

FlowSpec uses a different syntax than prefix-based families. Instead of prefixes,
it uses match components that describe traffic flows.

**Grammar:**
```
<nlri-section>     := nlri <flowspec-family> [rd <value>] <flowspec-op>+
<flowspec-op>      := add <component>+ | del <component>+

<flowspec-family>  := ipv4/flowspec | ipv6/flowspec
                    | ipv4/flowspec-vpn | ipv6/flowspec-vpn

<component>        := destination <prefix>
                    | source <prefix>
                    | protocol <proto>+
                    | port <op><value>+
                    | destination-port <op><value>+
                    | source-port <op><value>+
                    | icmp-type <value>+
                    | icmp-code <value>+
                    | tcp-flags <bitmask-match>+
                    | packet-length <op><value>+
                    | dscp <value>+
                    | fragment <bitmask-match>+

<op>               := = | > | >= | < | <=    # default is =
<proto>            := tcp | udp | icmp | gre | <number>

<bitmask-match>    := [&][!][=]<flag>[&<flag>...]
<flag>             := syn | ack | fin | rst | psh | push | urg | ece | cwr  # tcp-flags
                    | dont-fragment | is-fragment | first-fragment | last-fragment  # fragment
```

**Value Ranges (validated at parse time):**

| Component | Range | Bits |
|-----------|-------|------|
| protocol, icmp-type, icmp-code | 0-255 | 8 |
| port, destination-port, source-port, packet-length | 0-65535 | 16 |
| dscp | 0-63 | 6 |

**Bitmask Operators (RFC 8955 Section 4.2.1.2):**

| Syntax | Meaning | Wire Op |
|--------|---------|---------|
| `flag` | match if ANY of the flags are set | 0x00 (INCLUDE) |
| `=flag` | match if EXACTLY these flags are set | 0x01 (Match) |
| `!flag` | match if flag is NOT set | 0x02 (Not) |
| `!=flag` | match if NOT exactly these flags | 0x03 (Not+Match) |
| `flag1&flag2` | combine flags in same match | combined value |
| `&flag` | AND with previous match (vs OR) | 0x40 (And bit) |

**Examples:**
```
# Basic FlowSpec: match TCP port 80 to destination
nlri ipv4/flowspec add destination 10.0.0.0/24 protocol tcp destination-port =80

# Multiple components (AND logic)
nlri ipv4/flowspec add destination 10.0.0.0/24 source 192.168.0.0/16 protocol tcp

# FlowSpec VPN with RD
nlri ipv4/flowspec-vpn rd 65000:100 add destination 10.0.0.0/24

# Port range (>=1024 AND <=65535)
nlri ipv4/flowspec add destination-port >=1024 <=65535

# TCP flags with operators
nlri ipv4/flowspec add tcp-flags syn          # SYN is set (any)
nlri ipv4/flowspec add tcp-flags =syn         # ONLY SYN is set (exact)
nlri ipv4/flowspec add tcp-flags !rst         # RST is NOT set
nlri ipv4/flowspec add tcp-flags =syn&ack     # exactly SYN+ACK

# Fragment matching
nlri ipv4/flowspec add fragment !is-fragment  # NOT a fragment
nlri ipv4/flowspec add fragment dont-fragment # DF bit set

# Withdraw
nlri ipv4/flowspec del destination 10.0.0.0/24 protocol tcp
```

**FlowSpec Extended Community Actions (RFC 5575 Section 7):**

Actions are specified via extended-community with function syntax:

```
extended-community set traffic-rate <asn> <rate>   # Rate limit (bytes/sec)
extended-community set discard                      # Drop traffic (rate=0)
extended-community set redirect <asn> <target>      # Redirect to VRF
extended-community set traffic-marking <dscp>       # Set DSCP value
```

**Complete FlowSpec Rule Example:**
```
extended-community set traffic-rate 65000 1000000
nlri ipv4/flowspec add destination 10.0.0.0/24 protocol tcp destination-port =80
```

## Transaction Support

```go
// Per-peer transactions via CommitManager
BeginTransaction(peerSelector, label)  // Start batch
CommitTransaction(peerSelector)        // Flush + EOR
RollbackTransaction(peerSelector)      // Discard pending
```

Transaction flow:
1. `begin transaction batch1` - Mark peers in transaction mode
2. Routes → `Adj-RIB-Out.QueueAnnounce()` (queued, not sent)
3. `commit transaction` - Flush all queued, send EOR
<!-- source: internal/component/bgp/transaction/commit_manager.go -- CommitManager, Transaction -->

## The Reactor Interface

The seam between a command handler and the BGP engine is `BGPReactor`. Read the
declaration rather than a copy of it: a copy here goes stale on the next method
added, and this one did.

Most methods that reach a peer take the peer selector the command carried;
`SendRawMessage` takes one peer address instead. Every method that puts a
message on a peer's wire also takes the issuer: the attached process that asked,
or the operator who typed the command at the CLI, SSH or REST surface. The issuer
is what the send permission is checked against. Two further wire-writing rails
are not on this interface and carry the issuer the same way,
`ReactorCacheCoordinator.ForwardUpdatesDirect` and
`ReactorRelayCoordinator.RelayStoredRoute`.

A method that carries a `pluginName` beside the issuer is not naming a second
identity. That string is cache accounting, it names the consumer whose acks are
tracked, and it is empty for a caller that is not one. Nothing checks a
permission against it.
<!-- source: internal/component/bgp/types/reactor.go -- BGPReactor -->
<!-- source: internal/component/plugin/types_bgp.go -- ReactorCacheCoordinator, ReactorRelayCoordinator -->

The issuer is a `plugin.Sender`, and it has a third state. Its zero value means
no dispatch path said who issued the command, and the reactor refuses such a
command instead of reading it as the operator. A new dispatch path therefore
sets `CommandContext.Sender`, or every send it makes is denied and logged.
<!-- source: internal/component/plugin/sender.go -- Sender -->

## Output Format: UPDATE Events

### JSON Format (Command Style)

UPDATE events use the IPC Protocol format with a top-level wrapper and nested structure.
Each address family contains a list of operations grouped by next-hop.

**Announcements:**
```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "peer": {"address": "10.0.0.1", "local": {"address": "10.0.0.2", "as": 65000}, "remote": {"address": "10.0.0.1", "as": 65001}},
    "update": {
      "message": {"id": 1, "direction": "received"},
      "attr": {
        "origin": "igp",
        "as-path": [65001]
      },
      "nlri": {
        "ipv4/unicast": [
          {"action": "add", "next-hop": "10.0.0.1", "nlri": ["192.168.1.0/24", "192.168.2.0/24"]}
        ]
      }
    }
  }
}
```

**Withdrawals:**
```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "peer": {"address": "10.0.0.1", "local": {"address": "10.0.0.2", "as": 65000}, "remote": {"address": "10.0.0.1", "as": 65001}},
    "update": {
      "message": {"id": 2, "direction": "received"},
      "nlri": {
        "ipv4/unicast": [{"action": "del", "nlri": ["192.168.1.0/24"]}]
      }
    }
  }
}
```

**Mixed (announce + withdraw in same UPDATE):**
```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "peer": {"address": "10.0.0.1", "local": {"address": "10.0.0.2", "as": 65000}, "remote": {"address": "10.0.0.1", "as": 65001}},
    "update": {
      "message": {"id": 3, "direction": "received"},
      "attr": {"origin": "igp"},
      "nlri": {
        "ipv4/unicast": [
          {"action": "add", "next-hop": "10.0.0.1", "nlri": ["10.0.0.0/24"]},
          {"action": "del", "nlri": ["172.16.0.0/16"]}
        ]
      }
    }
  }
}
```

### Text Format

```
peer 10.0.0.1 remote as 65001 received update 1 origin igp path 65001 next 10.0.0.1 nlri ipv4/unicast add 192.168.1.0/24
```
<!-- source: internal/component/bgp/format/text_human.go -- appendFilterResultText -->

### NLRI Format by Family

| Family | Simple NLRI | Complex NLRI |
|--------|-------------|--------------|
| ipv4/unicast | `["10.0.0.0/24"]` | `[{"prefix": "10.0.0.0/24", "path-id": 1}]` (ADD-PATH) |
| ipv4/labeled-unicast | - | `[{"prefix": "10.0.0.0/24", "labels": [100]}]` |
| ipv4/mpls-vpn | - | `[{"prefix": "10.0.0.0/24", "rd": "2:65000:1", "labels": [100]}]` |
| l2vpn/evpn | - | `[{"route-type": "mac-ip", "rd": "2:65000:1", "esi": "00:...", ...}]` |
| ipv4/flowspec | - | String representation of FlowSpec rule |

**RD (Route Distinguisher) format:** `<type>:<value>` where:
- Type 0: `0:<asn2>:<assigned>` (e.g., `0:65000:100`)
- Type 1: `1:<ipv4>:<assigned>` (e.g., `1:192.0.2.1:100`)
- Type 2: `2:<asn4>:<assigned>` (e.g., `2:65536:100`)

### Format Options

| Option | Description |
|--------|-------------|
| `format parsed` | Decoded fields only (default) |
| `format raw` | Wire bytes only (hex) |
| `format full` | Both parsed AND raw bytes |

---

## Route Encoding

Routes are encoded using peer's context:

```go
func buildAnnounceUpdate(route RouteSpec, localAS uint32,
                         isIBGP bool, ctx *nlri.PackContext) *message.Update {
    // ctx.ASN4 → 2-byte vs 4-byte AS encoding
    // ctx.AddPath → path ID in NLRI
    // IPv6 → MP_REACH_NLRI (RFC 4760)
}
```

## Session Commands

| Command | Effect |
|---------|--------|
| `plugin session ready` | Signal plugin init complete |
| `plugin session ping` | Health check |
| `plugin session bye` | Disconnect |

### BGP Plugin Configuration

| Command | Effect |
|---------|--------|
| `bgp plugin encoding json` | Set event encoding to JSON (default) |
| `bgp plugin encoding text` | Set event encoding to human-readable text |
| `bgp plugin format hex` | Wire bytes as hex string |
| `bgp plugin format base64` | Wire bytes as base64 |
| `bgp plugin format parsed` | Decoded fields only (default) |
| `bgp plugin format full` | Both parsed AND wire bytes |
| `bgp plugin ack sync` | Wait for wire transmission |
| `bgp plugin ack async` | Return immediately (default) |

### Command Serial (ACK Control)

ACK is controlled by serial prefix, not session commands:

```
# No serial = fire-and-forget (no response)
update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24

# With serial = get response
#1 update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24
→ {"serial":"1","status":"done"}

# Error response
#2 bad command
→ {"serial":"2","status":"error","data":"unknown command"}
```

Response format:
```go
type Response struct {
    Serial  string `json:"serial,omitempty"`  // Correlation ID
    Status  string `json:"status"`            // "done", "error", "warning", or streaming
    Partial bool   `json:"partial,omitempty"` // True for streaming chunks
    Data    any    `json:"data,omitempty"`    // Payload
}
```
<!-- source: internal/component/plugin/types.go -- Response -->

### Response Status Values

| Status | Meaning |
|--------|---------|
| `done` | Command succeeded |
| `error` | Command failed |
| `warning` | Partial success or non-fatal issue (e.g., no peers accepted family) |
| `ack` | Streaming: more data coming |

## Plugin Commands

External processes can register custom commands that extend the API.

### Registration

```
#1 register command "myapp status" description "Show status" args "<component>" completable timeout 60s
```

### Execution Flow

```
CLI/Socket
    │ "myapp status web"
    ▼
Dispatcher.Dispatch()
    │ No builtin match → check registry
    ▼
CommandRegistry.Lookup("myapp status")
    │ Found → route to process
    ▼
routeToProcess()
    │ Add to PendingRequests
    │ Send JSON: {"serial":"a","type":"request","command":"myapp status","args":["web"],"peer":"*"}
    ▼
Process stdout
    │ @a done {"status":"running"}
    ▼
handlePluginResponse()
    │ Complete pending request
    ▼
Response returned to CLI
```

### Key Types

```go
// CommandRegistry manages plugin commands
type CommandRegistry struct {
    commands map[string]*RegisteredCommand  // lowercase name → registration
    builtins map[string]bool                // cannot be shadowed
}

// PendingRequests tracks in-flight requests
type PendingRequests struct {
    requests  map[string]*PendingRequest    // serial → pending
    byProcess map[*Process]map[string]bool  // for cleanup on death
}
```
<!-- source: internal/component/plugin/server/command_registry.go -- CommandRegistry -->
<!-- source: internal/component/plugin/server/pending.go -- PendingRequests -->

### Lifecycle

- **Process death:** `UnregisterAll()` + `CancelAll()` pending
- **Timeout:** 30s default, configurable per-command
- **Streaming:** `@serial+` resets timeout, collected into array

See `process-protocol.md` for full protocol details.

## Adj-RIB-Out (API Owned)

> **Note:** Adj-RIB-Out is now owned by API programs, not the engine.
> The engine has no route storage - it delegates to API.

API programs use `internal/component/bgp/rib/` as reference implementation:

```go
// In API program
type RIB struct {
    mu     sync.RWMutex
    routes map[string]map[string]*Route  // peer → routeKey → route
}

type Route struct {
    AttrHandle  pool.Handle  // Interned attributes
    NLRIHandle  pool.Handle  // Interned NLRI
    MsgID       uint64       // For bgp cache forward
    SourceCtxID uint16       // Encoding context
}
```

Key operations:
- `Insert(peer, route)` - Store route from peer
- `Remove(peer, prefix)` - Remove route
- `GetPeerRoutes(peer)` - Get all routes from peer
- `ClearPeer(peer)` - Remove all routes from peer

## Route Reflection via API (Cache Pattern)

> **Design history:** summary 148 was retired on 2026-08-01 and was not carried
> into `plan/learned/DESIGN-HISTORY.md`. That file's header gives the
> git-recovery route. What survives of the cache pattern is in its "BGP engine:
> wire encoding and RIB" > Load-bearing invariants: cache `Ack` and `Retain`
> are independent refcount axes, and engine cache ack is cumulative.

Ze implements route reflection through the API, not internally. This enables
external policy engines to make routing decisions.

### Architecture

```
Peer A → Receive UPDATE → Store (wire + msg-id) → API output (partial parse)
                                                            ↓
                                                   External process decides
                                                            ↓
                          API command: "bgp cache forward 123 !<ip>"
                                                            ↓
Peer B,C ← Send wire bytes directly ← Lookup cache by ID
```

### Key Concepts

| Concept | Description |
|---------|-------------|
| **Message ID** | Unique identifier per message, stored in `WireUpdate.MessageID()` for UPDATEs |
| **JSON Format** | `{"message":{"type":"update","id":N},...}` - common fields in `message` wrapper |
| **Direction** | `"sent"` or `"received"` indicator at top level for all messages |
| **Time-based cache** | Recent updates cached for fast lookup (TTL configurable) |
| **Partial parsing** | Only parse attributes needed for API output |
| **Forward by ID** | API references updates by ID via `bgp cache forward <id> <sel>` |
| **`!<ip>`** | Negated selector for "all except this peer" |

### Flow Details

1. **Receive:** Assign msg-id, cache UPDATE, store NLRIs in RIB
2. **API output:** Parse only configured attributes, include msg-id
3. **External decision:** Policy engine decides destinations
4. **Forward command:** `bgp cache forward <id> !<source-ip>`
5. **Send:** Lookup cached update, use wire bytes (zero-copy if contexts match)

### API Output with Message ID and Direction

```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "peer": {"address": "10.0.0.1", "local": {"address": "10.0.0.2", "as": 65000}, "remote": {"address": "10.0.0.1", "as": 65001}},
    "update": {
      "message": {"id": 12345, "direction": "received"},
      "attr": {"as-path": [65001, 65002]},
      "nlri": {
        "ipv4/unicast": [
          {"action": "add", "next-hop": "10.0.0.1", "nlri": ["192.168.1.0/24"]}
        ]
      }
    }
  }
}
```

### Cache Forward Command

```
# Forward update to all peers except source
bgp cache forward 12345 !10.0.0.1

# Forward to specific peer
bgp cache forward 12345 10.0.0.2
```

### Attribute Filtering (Partial Parse)

The engine can render an event with a subset of the path attributes parsed,
which costs less CPU, stores fewer parsed attributes, and leaves the wire bytes
whole for forwarding. `ContentConfig.Attributes` carries that selection.

The `content` block inside an attachment states `encoding`, `format` and
`attribute`, and ze reads none of the three per peer. Encoding and format are
process-wide (see "Encoding" above), and `attribute` reaches no field of
`ProcessBinding` at all. Do not write an attachment expecting one peer's events
to be rendered differently from another's.
<!-- source: internal/component/bgp/types/contentconfig.go -- ContentConfig -->
<!-- source: internal/component/bgp/reactor/peer_settings.go -- ProcessBinding -->

### RFC 9234 Role Tagging (Planned)

RFC 9234 (BGP Role) enables route decisions **without parsing attributes**:

```
Peer A (Role: Customer) → Receive → Tag with role → API output (role + update-id)
                                                            ↓
                                      External process decides based on ROLE
                                                            ↓
                             API command: "bgp cache forward 123 !<ip>"
```

Each route carries a `RouteTag`:
- `SourceRole` - RFC 9234 role (Provider/RS/RS-Client/Customer/Peer)
- `SourcePeerIP` - for `!<ip>` selector
- `HasOTC` / `OTCValue` - Only To Customer attribute (RFC 9234 Section 5)

With role tagging, decisions can be made without parsing AS_PATH, communities, etc.

### Wire Cache Value

Unlike locally-originated API routes, **received updates** benefit from wire caching:

| Route Type | Wire Cache | Reason |
|------------|------------|--------|
| API-originated | ❌ | Built from command, per-peer encoding |
| Received | ✅ | Forward by ID uses original wire bytes |

### Zero-Copy Forwarding

When forwarding by update-id:
1. Lookup cached update by ID
2. Check context compatibility (`sourceCtxID == destCtxID`)
3. If compatible: return `wireBytes` directly (zero-copy)
4. If not: re-encode with destination context

---

## Design Note: API Routes and Encoding

API routes are **locally originated** - they have no source wire bytes to cache.
This is correct behavior, not a limitation:

- **Zero-copy** is for route reflection (forwarding received routes)
- **API routes** are created from text commands, then encoded per-peer
- **Per-peer encoding** is required anyway (iBGP vs eBGP AS_PATH, next-hop-self, ADD-PATH)

The current flow builds UPDATEs with each peer's `PackContext`, which is RFC-correct.

## Peer Lifecycle Callbacks

The reactor notifies observers when peers change state via the unexported
`peerLifecycleObserver` interface:

```go
type peerLifecycleObserver interface {
    OnPeerEstablished(peer *Peer)
    OnPeerClosed(peer *Peer, reason string)
}
```
<!-- source: internal/component/bgp/reactor/reactor.go -- peerLifecycleObserver -->

### Registration

An observer outside the reactor package cannot name `*Peer`, so it registers
the any-typed `registry.PeerLifecycleCallback` instead. `callbackAdapter`
wraps it as a `peerLifecycleObserver` and `addPeerObserver` appends it.

```go
reactor.AddPeerLifecycleCallback(callback)
```
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- AddPeerLifecycleCallback, callbackAdapter, addPeerObserver -->
<!-- source: internal/component/plugin/registry/interfaces.go -- PeerLifecycleCallback -->

Observers are called synchronously in registration order. Implementations MUST NOT block.

### API State Observer

The `apiStateObserver` is automatically registered when API server starts. It emits state messages to all configured processes:

**Text format:**
```
peer 192.0.2.1 remote as 65001 state up
peer 192.0.2.1 remote as 65001 state down
```

**JSON format:**
```json
{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"local":{"address":"192.0.2.2","as":65000},"name":"","remote":{"address":"192.0.2.1","as":65001}},"state":"up"}}
{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"local":{"address":"192.0.2.2","as":65000},"name":"","remote":{"address":"192.0.2.1","as":65001}},"state":"down","reason":"hold-timer-expired"}}
```
<!-- source: internal/component/bgp/format/text_human.go -- appendStateChangeText -->
<!-- source: internal/component/bgp/format/text_json.go -- appendStateChangeJSON -->

### Close Reasons

| Reason | Trigger |
|--------|---------|
| `connection lost` | FSM transitions to Idle |
| `session closed` | FSM leaves Established for other state |

### Flow

```
FSM callback (peer.go)
    │ State changes from/to Established
    ▼
Peer.reactor.notifyPeerEstablished/Closed()
    │ Copy observers, iterate
    ▼
apiStateObserver.OnPeerEstablished/Closed()
    │ Build PeerInfo, call Server
    ▼
api.Server.OnPeerStateChange(peer, "up"/"down")
    │ appendStateChangeText or appendStateChangeJSON per process encoding
    ▼
Process stdin
```

---

## RIB Plugin and Route Replay

The RIB plugin (`internal/component/bgp/plugins/rib/`) tracks routes received from peers (Adj-RIB-In) and sent to peers (Adj-RIB-Out), replaying outgoing routes on session re-establishment.

### RIB Plugin Features

| RIB | Purpose | Events |
|-----|---------|--------|
| Adj-RIB-In | Routes received FROM peers | `update` (received) |
| Adj-RIB-Out | Routes sent TO peers | `sent` |

### RIB Flow

```
Session A established
    │ API sends route1
    ▼
Engine sends UPDATE to peer
    │ RIB receives "sent" event with route1
    ▼
RIB stores: ribOut[peerAddr][prefix] = route1
    │
    ▼ Session A teardown
    │
    ▼ Session B establishes
    │
RIB receives "state up"
    │ Looks up ribOut[peerAddr]
    ▼
RIB replays: "peer <addr> update text nhop set <nh> nlri <family> add <prefix>"
    │
    ▼
RIB signals: "peer <addr> plugin session ready"
```

### API Sync Protocol

To ensure routes are replayed before EOR is sent, the engine uses an API sync protocol:

1. **Session establishment:** Engine counts API bindings with `SendUpdate` permission
2. **ResetAPISync(count):** Peer initializes sync state with expected signal count
3. **RIB replays routes:** After "state up", replays stored routes
4. **RIB signals ready:** `"peer <addr> plugin session ready"`
5. **SignalPeerAPIReady:** Engine decrements counter, closes channel when all received
6. **sendInitialRoutes:** Waits up to 500ms for API sync before sending EOR
7. **AnnounceEOR guard:** External `AnnounceEOR` calls (plugin, route-server) skip peers where `ShouldQueue()` is true, preventing a race where a plugin EoR reaches the wire before queued route NLRI. The peer's own `sendInitialRoutes` EoR covers those families.

```go
// In sendInitialRoutes()
p.mu.RLock()
needsAPIWait := p.apiSyncExpected > 0
p.mu.RUnlock()
if needsAPIWait {
    time.Sleep(500 * time.Millisecond)
}
// Then process opQueue and send EOR
```

---

## gNMI Transport

Ze implements a gNMI (gRPC Network Management Interface) server as an independent component (`internal/component/gnmi/`). gNMI provides industry-standard YANG-modeled device management, making Ze addressable by the same tooling used for Cisco, Juniper, and Arista devices (gnmic, Ansible, controllers).

The gNMI transport is default-on but compile-out-able behind `ze_gnmi`. The hub reaches it through `gnmi_infra.go`; with the tag absent, the service hook is nil and the generated plugin imports drop `internal/component/gnmi`, `internal/component/gnmi/yang`, and `internal/plugins/gnmi-cmd/yang`, so `show gnmi` and `environment { gnmi {} }` are absent rather than partially registered.
<!-- source: feature-gates.txt -- ze_gnmi -->
<!-- source: cmd/ze/hub/gnmi_infra.go -- gnmiBuild, gnmiReloadNotify -->
<!-- source: internal/component/plugin/all/all_ze_gnmi.go -- gated gNMI imports -->

The gNMI server runs on a separate port (default 9339) with independent TLS and auth configuration. It supports Capabilities, Get, Set, and Subscribe (ONCE + STREAM) RPCs. Set operations use segment-based config paths through `ConfigSessionManager.SetSegments`/`DeleteSegments` to preserve IP addresses in list keys (Ze's dot-separated `splitPath` would break on IPs).

Configuration is YANG-modeled under `environment { gnmi { enabled; token; tls { cert; key; } server <name> { ip; port; } } }` with env var overrides (`ze.gnmi.enabled`, `ze.gnmi.listen`, `ze.gnmi.token`, `ze.gnmi.tls.cert`, `ze.gnmi.tls.key`).

Subscribe STREAM delivers change notifications from both gNMI Set operations and external config commits (web, CLI, managed). External commits trigger a generic config-reload notification via the `ChangeNotifier`, which is wired into the hub's `reloadAfterCommit` hook.

Prometheus counters: `ze_gnmi_requests_total{rpc}`, `ze_gnmi_subscribe_active`, `ze_gnmi_errors_total{rpc,code}`.

CLI: when `ze_gnmi` is compiled in, `show gnmi` returns server status (enabled, listen address, auth, TLS, active subscribers).
<!-- source: internal/component/gnmi/show.go -- handleShowGNMI -->

## Files

| File | Purpose |
|------|---------|
| `internal/component/plugin/server/server.go` | Plugin server lifecycle and client handling |
| `internal/component/plugin/server/startup.go` | Five startup phases and 5-stage handshake |
| `internal/component/plugin/process/process.go` | Internal and external plugin process lifecycle |
| `internal/component/plugin/process/delivery.go` | Event queue and batched delivery |
| `internal/component/plugin/server/dispatch.go` | Plugin-to-engine RPC dispatch |
| `internal/component/plugin/server/rpc_register.go` | Built-in RPC registration |
| `internal/component/plugin/server/events.go` | Event and NLRI routing |
| `internal/component/plugin/server/session.go` | Session handlers (ready, ping, bye) |
| `internal/component/plugin/server/schema.go` | SchemaRegistry (YANG RPC/notification indexing) |
| `internal/component/plugin/ipc/rpc.go` | Engine-side PluginConn wrapper |
| `internal/component/plugin/ipc/tls.go` | External TLS connect-back transport |
| `internal/component/plugin/registry/registry.go` | Compile-time plugin registry |
| `internal/component/bgp/route/route.go` | Route attribute/NLRI parsing |
| `internal/component/plugin/types.go` | ReactorInterface, RouteSpec |
| `internal/core/ipc/dispatch.go` | RPCDispatcher (wire-method exact-match) |
| `internal/component/config/yang/rpc.go` | YANG RPC/notification extraction |
| `pkg/plugin/rpc/conn.go` | Newline-framed RPC connection |
| `pkg/plugin/rpc/mux.go` | Bidirectional RPC multiplexer |
| `pkg/plugin/rpc/bridge.go` | DirectBridge in-process hot path |
| `pkg/plugin/sdk/` | Callback-based SDK for plugin authors |
| `internal/component/bgp/plugins/rib/rib.go` | RIB plugin (Adj-RIB-In/Out, route replay) |
| `internal/component/bgp/reactor/reactor.go` | AnnounceRoute, PeerLifecycleObserver |
| `internal/component/bgp/reactor/peer.go` | FSM callback, reactor notification, API sync |
| `internal/component/bgp/reactor/session.go` | Session lifecycle, teardown handling |
| `internal/component/bgp/rib/outgoing.go` | Adj-RIB-Out structure |
<!-- source: internal/component/plugin/server/ -- server, command, handler packages -->
<!-- source: internal/core/ipc/dispatch.go -- RPCDispatcher -->
