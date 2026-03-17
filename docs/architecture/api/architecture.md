# API Architecture

> **ARCHITECTURE:** API programs own ALL RIB data and logic.
> The Ze engine is a minimal BGP speaker - no RIB, no best-path, no policy.
> See `docs/architecture/rib-transition.md` for the full architecture.

## Implementation Status

| Feature | Status | Code Location |
|---------|--------|---------------|
| Process management | ✅ Done | `internal/component/plugin/process.go` |
| Backpressure (1000/100) | ✅ Done | `internal/component/plugin/process.go` |
| Respawn limits (5/60s) | ✅ Done | `internal/component/plugin/process.go` |
| Command dispatch | ✅ Done | `internal/component/plugin/command.go`, `internal/ipc/dispatch.go` |
| YANG API schema | ✅ Done | `internal/component/bgp/schema/`, `internal/ipc/schema/`, `internal/component/plugin/rib/schema/` |
| Plugin commands | ✅ Done | `internal/component/plugin/registry.go`, `internal/component/plugin/plugin.go` |
| Route injection | ✅ Done | `internal/component/plugin/rib/` |
| BGP cache commands | ✅ Done | `internal/component/plugin/cache.go` |
| Session sync | ✅ Done | `internal/component/plugin/session.go` |
| JSON/text encoding | ✅ Done | `internal/component/plugin/json.go` |
| RR plugin | ✅ Done | `internal/component/plugin/rr/` |
| RIB plugin | ✅ Done | `internal/component/bgp/plugins/rib/` |
| Adj-RIB-In plugin | ✅ Done | `internal/component/bgp/plugins/adj_rib_in/` |
| Shared BGP types | ✅ Done | `internal/component/bgp/` |
| borr/eorr markers | ✅ Done | RFC 7313 full support |

---

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Engine Role** | FSM, parsing, wire I/O, BGP cache |
| **API Role** | RIB storage, policy, best-path, GR state |
| **Communication** | JSON events + base64 wire bytes |
| **Key Types** | `Server`, `Client`, `Process`, `Dispatcher` |
| **RIB** | Owned by API program (use `internal/component/bgp/rib/` as reference) |
| **Polyglot** | API programs can be Go, Python, Rust, etc. |
| **Cache Control** | API controls cache via `bgp cache` commands |

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
| Cache Control | Retain/release/expire via `bgp cache <id>` commands |

### Wire Bytes in Events

Engine sends wire bytes to API in IPC Protocol format (when `format full` is configured):

```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "peer": {"address": "10.0.0.1", "asn": 65001},
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

### BGP Cache Control (✅ IMPLEMENTED)

API controls cache lifetime via `bgp cache` commands:

```
bgp cache 123 retain    # Keep until released
bgp cache 123 release   # Allow eviction
bgp cache 123 expire    # Remove immediately
bgp cache list          # List cached msg-ids
bgp cache 123 forward !10.0.0.1  # Forward to all except source
```

---

## Process and Peer API Binding

### Design Principles

```
Process = unique program (runs once, defined globally)
Peer API binding = which process, what messages, what format (per-peer)
```

One process can serve multiple peers. Each peer-binding can have different:
- Message types (update, notification, etc.)
- Format (parsed, raw, full)
- Encoding (json, text)

### Configuration Syntax

```
# Global process definition (program runs once)
process <name> {
    run <command>;
    respawn <bool>;
}

# Per-peer API binding
peer <address> {
    api <process-name> {
        content {
            encoding json;       # json | text
            format parsed;       # parsed | raw | full
        }
        receive {
            update;              # route announcements
            notification;        # errors
            state;               # up/down events
            all;                 # shorthand
        }
        send {
            update;              # can inject routes
        }
    }
}
```

### Key Differences from ExaBGP

| Aspect | ExaBGP | Ze |
|--------|--------|-------|
| Keyword | `neighbor {` | `peer {` |
| API binding | `api { processes [foo]; }` | `api foo { ... }` in peer |
| Format location | `receive { parsed; packets; }` | `content { format ...; }` per binding |
| Output syntax | `neighbor X announce route ...` | `update text nlri <family> ...` |

### Data Flow: Config → Server

```
config.PeerConfig.APIBindings
        │
        ▼ (loader.go)
reactor.PeerSettings.APIBindings
        │
        ▼ (stored in Reactor.peers)
Server.OnMessageReceived()
        │ calls reactor.GetPeerAPIBindings(addr)
        ▼
Per-binding format/encoding applied
```

**Server queries Reactor via ReactorInterface:**
- No data duplication (bindings live in PeerSettings)
- Server doesn't track peer lifecycle
- Encoding inheritance resolved at query time

### Encoding Inheritance

1. Peer binding specifies `content { encoding json; }` → use JSON
2. Peer binding empty → inherit from process `encoder json;`
3. Both empty → default to "text"

---

## Overview

The Ze API system enables external route injection and daemon control via:
- SSH connections (CLI tools)
- Subprocess management (external route generators)

## Package Structure

```
internal/component/plugin/
├── server.go         # Server, Client, plugin response handling
├── process.go        # Process, subprocess management
├── command.go        # Dispatcher, CommandContext, AllBuiltinRPCs()
├── handler.go        # RPCRegistration struct, constants
├── bgp.go            # BGP handlers (daemon, peer, introspection)
├── system.go         # System handlers (help, version, command, complete)
├── rib_handler.go    # RIB handlers (show/clear in/out, introspection)
├── session.go        # Session handlers (ready, ping, bye)
├── plugin.go         # Plugin handlers (register/unregister/response)
├── schema.go         # SchemaRegistry (RPC/notification indexing)
├── registry.go       # CommandRegistry for plugin commands
├── pending.go        # PendingRequests tracker (timeout, streaming)
├── route.go          # Route handlers (announce, withdraw)
├── commit.go         # Transaction handlers
├── commit_manager.go # Transaction management
├── types.go          # ReactorInterface, RouteSpec, Response
├── subscribe.go      # Event subscription handlers
└── text.go           # ExaBGP-style text encoding

internal/ipc/
├── dispatch.go       # RPCDispatcher (wire-method exact-match dispatch)
├── framing.go        # NUL-byte framing (Spec 1)
├── message.go        # Request/Response/Error types (Spec 1)
└── schema/           # ze-system-api.yang, ze-plugin-api.yang

internal/yang/
├── rpc.go            # Extract RPCs/notifications from YANG Entry tree
└── loader.go         # YANG module loader
```

### YANG API Modules

Each YANG module defines RPCs and notifications for a domain. Every RPC maps 1:1 to a handler function via `RPCRegistration`:

| Module | Location | RPCs | Notifications |
|--------|----------|------|---------------|
| `ze-bgp-api` | `internal/component/bgp/schema/` | 25 | 7 |
| `ze-bgp-cmd-log-api` | `internal/component/cmd/log/schema/` | 2 | 0 |
| `ze-bgp-cmd-metrics-api` | `internal/component/cmd/metrics/schema/` | 2 | 0 |
| `ze-system-api` | `internal/ipc/schema/` | 8 | 0 |
| `ze-plugin-api` | `internal/ipc/schema/` | 8 | 0 |
| `ze-rib-api` | `internal/component/plugin/rib/schema/` | 9 | 1 |
| `ze-plugin-engine` | `internal/yang/modules/` | 11 | 0 |
| `ze-plugin-callback` | `internal/yang/modules/` | 8 | 0 |

Wire methods use `module:rpc-name` format with `-api` suffix stripped (e.g., `ze-bgp-api` defines `ze-bgp:peer-list`). This is done by `WireModule()` in `internal/yang/rpc.go`.

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

`AllBuiltinRPCs()` in `command.go` aggregates all 51 handlers into a flat `[]RPCRegistration` slice.

## Monitor Streaming

The `ze-bgp:monitor` RPC provides live BGP event streaming over SSH sessions. Unlike request/response RPCs, monitor keeps the SSH session open and writes events line-by-line as they occur.

| Component | Location | Purpose |
|-----------|----------|---------|
| `StreamingExecutorFactory` | SSH server | Detects monitor commands and returns a streaming executor instead of a one-shot executor |
| `MonitorManager` | `internal/component/plugin/` (Server) | Manages active monitor clients (add, remove, broadcast) |
| Event delivery | All 6 event functions | After delivering events to plugins, each event function also delivers to active monitor clients |

Monitor supports filtering by peer address, event type, and direction. Pipe operators (`| json`, `| table`, `| match`) are applied server-side before streaming to the client.

## Connection Types

### Socket Clients

```go
type Client struct {
    id     string
    conn   net.Conn
    server *Server
    ctx    context.Context
    cancel context.CancelFunc
}
```

Flow: `acceptLoop()` → `handleClient()` → `clientLoop()` → `processCommand()`

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

### Plugin IPC Protocol (YANG RPC)

The plugin IPC layer replaces stdin/stdout text pipes with YANG RPC calls over two Unix socket pairs per plugin. Infrastructure is in place; individual plugin migration is incremental.

```
pkg/plugin/rpc/
├── conn.go           # rpc.Conn — shared NUL-framed JSON RPC connection
└── types.go          # Canonical wire-format types (DeclareRegistrationInput, etc.)

pkg/plugin/sdk/
└── sdk.go            # Plugin SDK — callback-based API for plugin authors

internal/component/plugin/
├── socketpair.go     # DualSocketPair (internal: net.Pipe, external: socketpair)
└── rpc_plugin.go     # PluginConn (embeds *rpc.Conn, typed stage methods)

internal/yang/modules/
├── ze-plugin-engine.yang    # RPCs engine serves (12: startup, routes, dispatch, subscriptions, decode/encode)
└── ze-plugin-callback.yang  # RPCs plugin serves (8: configure, deliver-event, bye, etc.)
```

**Two-socket architecture:**

| Socket | Engine Role | Plugin Role | RPCs |
|--------|-------------|-------------|------|
| A | Server | Client | declare-registration, declare-capabilities, ready, update-route, dispatch-command, subscribe/unsubscribe, decode/encode-nlri, decode-mp-reach/unreach, decode-update |
| B | Client | Server | configure, share-registry, deliver-event, encode/decode-nlri, decode-capability, execute-command, bye |

**5-stage startup preserved as typed RPCs:**
1. Plugin calls `declare-registration` (Socket A)
2. Engine calls `configure` (Socket B)
3. Plugin calls `declare-capabilities` (Socket A)
4. Engine calls `share-registry` (Socket B)
5. Plugin calls `ready` (Socket A)

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

### Next-Hop Resolution

`RouteNextHop` is resolved at **peer level** in `internal/component/bgp/reactor/peer.go` via `resolveNextHop()`:

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
watchdog announce <name>   # send all routes in pool to peers
watchdog withdraw <name>   # withdraw all routes in pool from peers
```

### Result Types

```go
type UpdateTextResult struct {
    Groups       []NLRIGroup  // Each nlri section produces a group
    WatchdogName string       // Optional watchdog pool name
}

type NLRIGroup struct {
    Family   nlri.Family    // ipv4/unicast, ipv6/unicast, etc.
    Announce []nlri.NLRI    // Prefixes to announce
    Withdraw []nlri.NLRI    // Prefixes to withdraw
    Attrs    PathAttributes // Snapshot of attributes at this point
    NextHop  RouteNextHop   // Encapsulates next-hop policy (explicit or self)
}
```

### YANG-Driven Attribute Validation

Attribute values in the update text parser are validated against the YANG schema (`ze-bgp-conf.yang`), making YANG the single source of truth for data validation. The `ValueValidator` interface provides the validation, set via `SetYANGValidator()`.

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

## ReactorInterface

```go
type ReactorInterface interface {
    // Route injection
    AnnounceRoute(peerSelector string, route RouteSpec) error
    WithdrawRoute(peerSelector string, prefix netip.Prefix) error

    // Transactions
    BeginTransaction(peerSelector, label string) error
    CommitTransaction(peerSelector) (TransactionResult, error)
    RollbackTransaction(peerSelector) (TransactionResult, error)

    // RIB access
    RIBInRoutes(peerID string) []RIBRoute
    RIBOutRoutes() []RIBRoute
    RIBStats() RIBStatsInfo

    // Peer management
    GetPeerByIP(ip string) (Peer, bool)
    GetPeers() []Peer

    // API bindings (Phase 1)
    GetPeerAPIBindings(addr netip.Addr) []PeerAPIBinding
    // ... etc
}
```

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
    "peer": {"address": "10.0.0.1", "asn": 65001},
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
    "peer": {"address": "10.0.0.1", "asn": 65001},
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
    "peer": {"address": "10.0.0.1", "asn": 65001},
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
peer 10.0.0.1 received update 1 announce origin igp as-path 65001 nhop set 10.0.0.1 nlri ipv4/unicast add 192.168.1.0/24
```

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

### Lifecycle

- **Process death:** `UnregisterAll()` + `CancelAll()` pending
- **Timeout:** 30s default, configurable per-command
- **Streaming:** `@serial+` resets timeout, collected into array

See `PROCESS_PROTOCOL.md` for full protocol details.

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

> **Implementation spec:** `docs/learned/148-api-command-restructure-step-8.md`

Ze implements route reflection through the API, not internally. This enables
external policy engines to make routing decisions.

### Architecture

```
Peer A → Receive UPDATE → Store (wire + msg-id) → API output (partial parse)
                                                            ↓
                                                   External process decides
                                                            ↓
                          API command: "bgp cache 123 forward !<ip>"
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
| **Forward by ID** | API references updates by ID via `bgp cache <id> forward` |
| **`!<ip>`** | Negated selector for "all except this peer" |

### Flow Details

1. **Receive:** Assign msg-id, cache UPDATE, store NLRIs in RIB
2. **API output:** Parse only configured attributes, include msg-id
3. **External decision:** Policy engine decides destinations
4. **Forward command:** `bgp cache <id> forward !<source-ip>`
5. **Send:** Lookup cached update, use wire bytes (zero-copy if contexts match)

### API Output with Message ID and Direction

```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "peer": {"address": "10.0.0.1", "asn": 65001},
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
bgp cache 12345 forward !10.0.0.1

# Forward to specific peer
bgp cache 12345 forward 10.0.0.2
```

### Attribute Filtering (Partial Parse)

API bindings can limit which attributes are parsed:

```
api foo {
    content {
        attribute as-path community next-hop;  # Only parse these
        nlri ipv4/unicast;                     # Only include IPv4 unicast
        nlri ipv6/unicast;                     # Also include IPv6 unicast
    }
    receive { update; }
}
```

Benefits:
- Reduced CPU (parse only what's needed)
- Reduced memory (don't store parsed attributes long-term)
- Wire bytes preserved for forwarding
- NLRI filtering reduces output to relevant families only

### RFC 9234 Role Tagging (Planned)

RFC 9234 (BGP Role) enables route decisions **without parsing attributes**:

```
Peer A (Role: Customer) → Receive → Tag with role → API output (role + update-id)
                                                            ↓
                                      External process decides based on ROLE
                                                            ↓
                             API command: "peer !<ip> forward update-id 123"
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

The reactor notifies observers when peers change state via the `PeerLifecycleObserver` interface:

```go
type PeerLifecycleObserver interface {
    OnPeerEstablished(peer *Peer)
    OnPeerClosed(peer *Peer, reason string)
}
```

### Registration

```go
reactor.AddPeerObserver(observer)
```

Observers are called synchronously in registration order. Implementations MUST NOT block.

### API State Observer

The `apiStateObserver` is automatically registered when API server starts. It emits state messages to all configured processes:

**Text format:**
```
peer 192.0.2.1 asn 65001 state up
peer 192.0.2.1 asn 65001 state down
```

**JSON format:**
```json
{"type":"bgp","bgp":{"type":"state","peer":{"address":"192.0.2.1","asn":65001},"state":"up"}}
{"type":"bgp","bgp":{"type":"state","peer":{"address":"192.0.2.1","asn":65001},"state":"down","reason":"hold timer expired"}}
```

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
    │ FormatStateChange per process encoding
    ▼
Process stdin
```

---

## RIB Plugin and Route Replay

The RIB plugin (`internal/component/plugin/rib/`) tracks routes received from peers (Adj-RIB-In) and sent to peers (Adj-RIB-Out), replaying outgoing routes on session re-establishment.

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

## Files

| File | Purpose |
|------|---------|
| `internal/component/plugin/server.go` | Server, Client, socket handling |
| `internal/component/plugin/process.go` | Subprocess management |
| `internal/component/plugin/command.go` | Dispatcher, AllBuiltinRPCs() |
| `internal/component/plugin/handler.go` | RPCRegistration struct, constants |
| `internal/component/plugin/bgp.go` | BGP handlers (daemon, peer, introspection) |
| `internal/component/plugin/system.go` | System handlers (help, version, command) |
| `internal/component/plugin/rib_handler.go` | RIB handlers (show/clear, introspection) |
| `internal/component/plugin/session.go` | Session handlers (ready, ping, bye) |
| `internal/component/plugin/schema.go` | SchemaRegistry (YANG RPC/notification indexing) |
| `internal/component/bgp/route/route.go` | Route attribute/NLRI parsing |
| `internal/component/plugin/types.go` | ReactorInterface, RouteSpec |
| `internal/component/plugin/text.go` | Text/JSON formatting including FormatStateChange |
| `internal/component/plugin/commit_manager.go` | Transaction management |
| `internal/ipc/dispatch.go` | RPCDispatcher (wire-method exact-match) |
| `internal/yang/rpc.go` | YANG RPC/notification extraction |
| `internal/component/plugin/rib/rib.go` | RIB plugin (Adj-RIB-In/Out, route replay) |
| `internal/component/bgp/reactor/reactor.go` | AnnounceRoute, PeerLifecycleObserver |
| `internal/component/bgp/reactor/peer.go` | FSM callback, reactor notification, API sync |
| `internal/component/bgp/reactor/session.go` | Session lifecycle, teardown handling |
| `internal/component/bgp/rib/outgoing.go` | Adj-RIB-Out structure |
