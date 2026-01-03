# API Architecture

> **⚠️ PLANNED CHANGE:** Adj-RIB-Out will be removed from router core.
> See `plan/spec-remove-adjrib-integration.md` and `.claude/zebgp/api/CAPABILITY_CONTRACT.md`.
> Route refresh will be delegated to external API programs.

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Purpose** | External route injection via Unix socket and subprocess |
| **Flow** | `acceptLoop()` → `handleClient()` → `processCommand()` → Reactor |
| **Key Types** | `Server`, `Client`, `Process`, `Dispatcher`, `RouteSpec` |
| **Transactions** | `begin`/`commit`/`rollback` with Adj-RIB-Out queuing |
| **Process binding** | Per-peer API binding with encoding/format config |
| **Output syntax** | `announce nlri <family> <nlris>` (differs from ExaBGP) |
| **Encoding** | Per-peer (correct: iBGP/eBGP differ, next-hop-self, ADD-PATH) |

**When to read full doc:** API commands, route injection, subprocess management.

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

| Aspect | ExaBGP | ZeBGP |
|--------|--------|-------|
| Keyword | `neighbor {` | `peer {` |
| API binding | `api { processes [foo]; }` | `api foo { ... }` in peer |
| Format location | `receive { parsed; packets; }` | `content { format ...; }` per binding |
| Output syntax | `neighbor X announce route ...` | `announce nlri <family> <nlris>` |

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

The ZeBGP API system enables external route injection and daemon control via:
- Unix socket connections (CLI tools)
- Subprocess management (external route generators)

## Package Structure

```
pkg/api/
├── server.go         # Server, Client, socket listener, plugin response handling
├── process.go        # Process, subprocess management
├── command.go        # Dispatcher, CommandContext, plugin routing
├── registry.go       # CommandRegistry for plugin commands
├── pending.go        # PendingRequests tracker (timeout, streaming)
├── plugin.go         # Parse register/unregister/response
├── route.go          # Route handlers (announce, withdraw)
├── commit.go         # Transaction handlers
├── commit_manager.go # Transaction management
├── types.go          # ReactorInterface, RouteSpec, Response
├── session.go        # Session commands (ack, sync)
├── handler.go        # System command handlers (help, version, command)
└── text.go           # ExaBGP-style text encoding
```

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

## Route Injection Flow

### Unicast Routes

```
API Client
    │ "announce route 10.0.0.0/24 next-hop 1.2.3.4"
    ▼
Dispatcher.handleAnnounceRoute()
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
    │ "announce ipv4 nlri-mpls 10.0.0.0/24 label 100 next-hop 1.2.3.4 path-id 42"
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
    Prefix      netip.Prefix
    NextHop     netip.Addr
    NextHopSelf bool
    PathAttributes
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

## Output Format: announce nlri

### JSON Format

```json
{
  "type": "update",
  "direction": "received",
  "msg-id": 1,
  "peer": {
    "address": "10.0.0.1",
    "asn": 65001
  },
  "announce": {
    "nlri": {
      "ipv4 unicast": {
        "192.168.1.0/24": {
          "next-hop": "10.0.0.1",
          "origin": "igp",
          "as-path": [65001]
        }
      }
    }
  }
}
```

### Text Format

```
peer 10.0.0.1 received update 1 announce origin igp as-path 65001 ipv4 unicast next-hop 10.0.0.1 nlri 192.168.1.0/24
```

### Withdrawals

JSON:
```json
{
  "type": "update",
  "peer": { "address": "10.0.0.1" },
  "withdraw": {
    "nlri": {
      "ipv4 unicast": ["192.168.1.0/24"]
    }
  }
}
```

Text:
```
peer 10.0.0.1 received update 1 withdraw ipv4 unicast nlri 192.168.1.0/24
```

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
| `session sync enable` | Wait for wire transmission |
| `session sync disable` | Return immediately |
| `session reset` | Reset session state |
| `session ping` | Health check |
| `session bye` | Client disconnect |

### Command Serial (ACK Control)

ACK is controlled by serial prefix, not session commands:

```
# No serial = fire-and-forget (no response)
announce route 10.0.0.0/24 next-hop 1.2.3.4

# With serial = get response
#1 announce route 10.0.0.0/24 next-hop 1.2.3.4
→ {"serial":"1","status":"done"}

# Error response
#2 bad command
→ {"serial":"2","status":"error","data":"unknown command"}
```

Response format:
```go
type Response struct {
    Serial  string `json:"serial,omitempty"`  // Correlation ID
    Status  string `json:"status"`            // "done", "error", or streaming
    Partial bool   `json:"partial,omitempty"` // True for streaming chunks
    Data    any    `json:"data,omitempty"`    // Payload
}
```

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

## Adj-RIB-Out

```go
type OutgoingRIB struct {
    pending   []*Route           // Queued for announcement
    withdrawn []nlri.NLRI        // Queued for withdrawal
    sent      map[string]*Route  // Already sent (cache)

    inTransaction    bool
    transactionLabel string
}
```

Key methods:
- `QueueAnnounce(route)` - Queue for sending
- `MarkSent(route)` - Move to sent cache
- `BeginTransaction()` - Start batch mode
- `CommitAndClear()` - Flush queued

## Route Reflection via API (Update ID Pattern)

> **Implementation spec:** `plan/spec-route-id-forwarding.md`

ZeBGP implements route reflection through the API, not internally. This enables
external policy engines to make routing decisions.

### Architecture

```
Peer A → Receive UPDATE → Store (wire + update ID) → API output (partial parse)
                                                            ↓
                                                   External process decides
                                                            ↓
                          API command: "peer !<ip> forward update-id 123"
                                                            ↓
Peer B,C ← Send wire bytes directly ← Lookup update by ID
```

### Key Concepts

| Concept | Description |
|---------|-------------|
| **Message ID** | Unique identifier per message (all types: OPEN, UPDATE, KEEPALIVE, NOTIFICATION) |
| **Direction** | `"sent"` or `"received"` indicator for all messages |
| **Time-based cache** | Recent updates cached for fast lookup (TTL configurable) |
| **Partial parsing** | Only parse attributes needed for API output |
| **Forward by ID** | API references updates by ID, ZeBGP forwards wire bytes |
| **`peer !<ip>`** | Negated selector for "all except this peer" |

### Flow Details

1. **Receive:** Assign update-id, cache UPDATE, store NLRIs in RIB
2. **API output:** Parse only configured attributes, include update-id
3. **External decision:** Policy engine decides destinations
4. **Forward command:** `peer !<source-ip> forward update-id <id>`
5. **Send:** Lookup cached update, use wire bytes (zero-copy if contexts match)

### API Output with Message ID and Direction

```json
{
  "type": "update",
  "direction": "received",
  "msg-id": 12345,
  "peer": { "address": "10.0.0.1" },
  "announce": {
    "nlri": { "ipv4 unicast": ["192.168.1.0/24"] },
    "attributes": {
      "as-path": [65001, 65002],
      "next-hop": "10.0.0.1"
    }
  }
}
```

### Forward Command

```
# Forward update to all peers except source
peer !10.0.0.1 forward update-id 12345

# Forward to specific peer
peer 10.0.0.2 forward update-id 12345
```

### Attribute Filtering (Partial Parse)

API bindings can limit which attributes are parsed:

```
api foo {
    content {
        attribute as-path community next-hop;  # Only parse these
        nlri ipv4 unicast;                     # Only include IPv4 unicast
        nlri ipv6 unicast;                     # Also include IPv6 unicast
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
{"type":"state","peer":{"address":"192.0.2.1","asn":65001},"state":"up"}
{"type":"state","peer":{"address":"192.0.2.1","asn":65001},"state":"down"}
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

## Files

| File | Purpose |
|------|---------|
| `pkg/api/server.go` | Server, Client, socket handling |
| `pkg/api/process.go` | Subprocess management |
| `pkg/api/route.go` | Route announce/withdraw handlers |
| `pkg/api/types.go` | ReactorInterface, RouteSpec |
| `pkg/api/text.go` | Text/JSON formatting including FormatStateChange |
| `pkg/api/commit_manager.go` | Transaction management |
| `pkg/reactor/reactor.go` | AnnounceRoute, PeerLifecycleObserver |
| `pkg/reactor/peer.go` | FSM callback, reactor notification |
| `pkg/rib/outgoing.go` | Adj-RIB-Out structure |
