# API Architecture

## Overview

The ZeBGP API system enables external route injection and daemon control via:
- Unix socket connections (CLI tools)
- Subprocess management (external route generators)

## Package Structure

```
pkg/api/
├── server.go         # Server, Client, socket listener
├── process.go        # Process, subprocess management
├── command.go        # Dispatcher, CommandContext
├── route.go          # Route handlers (announce, withdraw)
├── commit.go         # Transaction handlers
├── commit_manager.go # Transaction management
├── types.go          # ReactorInterface, RouteSpec
├── session.go        # Session commands (ack, sync)
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
    ackEnabled  atomic.Bool    // Send "done" responses
    syncEnabled atomic.Bool    // Wait for wire transmission

    // Backpressure
    writeQueue   chan []byte
    queueDropped atomic.Uint64
}
```

Features:
- Per-process session state (ACK/sync mode)
- Write queue with backpressure (high: 1000, low: 100)
- Respawn limits (max 5 per 60 seconds)

## Route Injection Flow

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
    // ... etc
}
```

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
| `session ack enable` | Send "done" after each command |
| `session ack disable` | No acknowledgments |
| `session sync enable` | Wait for wire transmission |
| `session sync disable` | Return immediately |

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

## Current Limitation: No API Context

**Problem:** API routes don't have an EncodingContext:
- No `sourceCtxID` for zero-copy forwarding
- No wire cache for API-sourced routes
- API doesn't declare capabilities (ASN4, ADD-PATH)

**Solution:** API-as-Peer design (see `plan/spec-api-virtual-peer.md`)

## Files

| File | Purpose |
|------|---------|
| `pkg/api/server.go` | Server, Client, socket handling |
| `pkg/api/process.go` | Subprocess management |
| `pkg/api/route.go` | Route announce/withdraw handlers |
| `pkg/api/types.go` | ReactorInterface, RouteSpec |
| `pkg/api/commit_manager.go` | Transaction management |
| `pkg/reactor/reactor.go` | AnnounceRoute implementation |
| `pkg/rib/outgoing.go` | Adj-RIB-Out structure |
