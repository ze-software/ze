# Spec: zebgp api rr / zebgp api persist

> **Architecture:** API programs own ALL RIB data and logic.
> See `docs/architecture/rib-transition.md` for the overall architecture.

## Overview

Two separate plugins sharing common code:

| Plugin | Use Case | RIB | Events |
|--------|----------|-----|--------|
| `zebgp api rr` | Route Server (multi-peer) | ribIn | `update`, `state` |
| `zebgp api persist` | State persistence (single-peer) | ribOut | `sent`, `state` |

**Key features:**
- Pool-based attribute deduplication
- Wire bytes storage for zero-copy replay
- msg-id cache control (retain/release/expire)
- Polyglot-friendly design (Go, Python, Rust)

## Invocation

```
zebgp api rr        # Route Server mode
zebgp api persist   # Persistence mode (for Test C)
```

Runs as API process, communicates via stdin/stdout with ZeBGP engine.

---

## Design: Forward-All with Last-Wins

**Model:** RS forwards all UPDATEs immediately. No best-path selection. Peers receive all routes and use the last one received.

**Rationale:**
- Zero-copy forwarding (performance)
- Simple implementation (no RIB)
- Peers maintain Adj-RIB-In and can compare routes
- Acceptable for IX route server use case

**Trade-off:** If peer A announces, then peer B announces a worse route, peers keep B's route. This is acceptable because:
1. In practice, routes converge quickly
2. Peers with multiple sessions can prefer other sources
3. Simplicity outweighs optimal selection

---

## Architecture

```
Peer A ──UPDATE──▶ ZeBGP ──JSON──▶ zebgp api rr
                  (msg-id 123)           │
                                         │ Store in Adj-RIB-In
                                         │ (for replay on peer up)
                                         ▼
                   ZeBGP ◀─────────── zebgp api rr
                     │       peer !A forward update-id 123
                     ▼
              Peers B, C, D (zero-copy forward)
```

**Key:** RS stores routes in Adj-RIB-In for replay. No best-path selection.

---

## Wire Bytes and Pool

Engine sends base64-encoded wire bytes to API:

```json
{
  "type": "update",
  "msg-id": 123,
  "source-ctx-id": 42,
  "raw-attributes": "AQEBAQECAQID...",
  "raw-nlri": {
    "ipv4/unicast": "GApAAA==",
    "ipv6/unicast": "QCABDbgAAAAA..."
  },
  "parsed": { ... }
}
```

**Note:** `raw-nlri` is keyed by family since UPDATE can contain multiple families.

API stores wire bytes in pool for deduplication:

```go
// On UPDATE received
attrBytes, _ := base64.StdEncoding.DecodeString(event.RawAttributes)
attrHandle := pool.Intern(attrBytes)

for family, nlriB64 := range event.RawNLRI {
    nlriBytes, _ := base64.StdEncoding.DecodeString(nlriB64)
    nlriHandle := pool.Intern(nlriBytes)

    route := &Route{
        AttrHandle:  attrHandle,
        NLRIHandle:  nlriHandle,
        Family:      family,
        MsgID:       event.MsgID,
        SourceCtxID: event.SourceCtxID,
    }
    rib.Insert(event.Peer, route)
}

// Retain msg-id for replay
send("msg-id %d retain", event.MsgID)
```

---

## msg-id Cache Control

API controls engine's msg-id cache lifetime:

| Command | Description |
|---------|-------------|
| `msg-id <id> retain` | Keep until released |
| `msg-id <id> release` | Allow eviction (60s default) |
| `msg-id <id> expire` | Remove immediately |
| `msg-id list` | List cached msg-ids |

**Lifecycle:**
1. Engine receives UPDATE, assigns msg-id, caches wire bytes
2. Engine sends event to API
3. API stores route, sends `msg-id N retain`
4. On peer reconnect: API replays via `peer X forward update-id N`
5. On route withdrawal: API sends `msg-id N release`

---

## State

```go
// Shared types in pkg/api/apiutil/
type Route struct {
    AttrHandle  pool.Handle  // Interned attributes
    NLRIHandle  pool.Handle  // Interned NLRI
    Family      string       // e.g., "ipv4/unicast"
    MsgID       uint64       // For forward update-id
    SourceCtxID uint16       // For context matching
}

type RIB struct {
    mu     sync.RWMutex
    routes map[string]map[string]*Route  // peer → routeKey → route
}

type PeerState struct {
    Address      string
    ASN          uint32
    Up           bool
    Capabilities map[string]bool
    Families     map[string]bool
}

// zebgp api rr
type RouteServer struct {
    pool   *pool.Pool
    peers  map[string]*PeerState
    ribIn  *RIB  // Routes FROM peers (for forwarding to others)
}

// zebgp api persist
type PersistServer struct {
    pool   *pool.Pool
    peers  map[string]*PeerState
    ribOut *RIB  // Routes SENT TO peers (for replay on reconnect)
}
```

**Key difference:**
- `ribIn`: Routes received FROM peers → forward to other peers
- `ribOut`: Routes sent TO peers → replay on peer reconnect

---

## Event Handling

### UPDATE Received

```json
{
  "type": "update",
  "msg-id": 123,
  "source-ctx-id": 42,
  "raw-attributes": "AQEBAQECAQID...",
  "raw-nlri": {"ipv4/unicast": "GApAAA=="},
  "peer": {"address": {"peer": "10.0.0.1"}},
  "parsed": {...}
}
```

**Action:**
```go
// Decode and store in pool
attrBytes, _ := base64.StdEncoding.DecodeString(event.RawAttributes)
attrHandle := pool.Intern(attrBytes)

for family, nlriB64 := range event.RawNLRI {
    nlriBytes, _ := base64.StdEncoding.DecodeString(nlriB64)
    nlriHandle := pool.Intern(nlriBytes)

    route := &Route{AttrHandle: attrHandle, NLRIHandle: nlriHandle, Family: family, MsgID: 123}
    ribIn.Insert("10.0.0.1", route)
}

// Retain msg-id and forward
send("msg-id 123 retain")
send("peer !10.0.0.1 forward update-id 123")
```

### Withdraw Received

```json
{
  "type": "update",
  "msg-id": 124,
  "peer": {"address": {"peer": "10.0.0.1"}},
  "parsed": {"withdraw": {"ipv4/unicast": ["10.0.0.0/24"]}}
}
```

**Action:**
```go
// Remove from RIB
oldRoute := ribIn.Remove("10.0.0.1", "ipv4/unicast", "10.0.0.0/24")

// Release old msg-id, retain new
if oldRoute != nil {
    send("msg-id %d release", oldRoute.MsgID)
}
send("msg-id 124 retain")
send("peer !10.0.0.1 forward update-id 124")
```

### Peer Down

```json
{"type":"state","peer":{"address":{"peer":"10.0.0.1"},"asn":{"peer":65001}},"state":"down"}
```

**Action (zebgp api rr):**
```go
// Get all routes from downed peer
routes := ribIn.GetPeerRoutes("10.0.0.1")

// Withdraw all routes from downed peer to other peers
for _, route := range routes {
    send("peer !10.0.0.1 withdraw route %s %s", route.Prefix, route.Family)
    // Release msg-id - route no longer valid
    send("msg-id %d release", route.MsgID)
}

// Clear ribIn - routes FROM this peer are gone
ribIn.ClearPeer("10.0.0.1")
```

**Action (zebgp api persist):**
```go
// DO NOT clear ribOut - keep routes for replay on reconnect
// ribOut stores routes SENT TO this peer, not FROM
// msg-ids stay retained for replay
peer.Up = false
```

**Critical:** `ribOut` is KEPT on peer down. Routes were sent before; replay on reconnect. msg-ids remain retained.

### Peer Up

```json
{"type":"state","peer":{"address":{"peer":"10.0.0.1"},"asn":{"peer":65001}},"state":"up"}
```

**Action (zebgp api rr):**
```go
// Replay all routes from other peers to new peer
for peerID, routes := range ribIn.GetAllPeers() {
    if peerID == "10.0.0.1" {
        continue  // Don't send peer's own routes back
    }
    for _, route := range routes {
        send("peer 10.0.0.1 forward update-id %d", route.MsgID)
    }
}
// Send EOR for each family after replay
for family := range peer.Families {
    send("peer 10.0.0.1 eor %s", family)
}
```

**Action (zebgp api persist):**
```go
// Replay all routes that were sent to this peer before
for _, route := range ribOut.GetPeerRoutes("10.0.0.1") {
    send("peer 10.0.0.1 forward update-id %d", route.MsgID)
}
// Send EOR for each family after replay
for family := range peer.Families {
    send("peer 10.0.0.1 eor %s", family)
}
peer.Up = true
```

**EOR required:** After replaying routes, send End-of-RIB marker per family.

### Fallback: Announce Raw

If msg-id cache is unavailable (shouldn't happen with retain), API can announce from pool:

```go
// Get wire bytes from pool
attrBytes := pool.Get(route.AttrHandle)
nlriBytes := pool.Get(route.NLRIHandle)

// Re-announce with raw wire bytes
send("peer 10.0.0.1 announce raw %s nlri %s %s",
    base64.StdEncoding.EncodeToString(attrBytes),
    route.Family,
    base64.StdEncoding.EncodeToString(nlriBytes))
```

### Route Refresh

```json
{"type":"refresh","peer":{"address":"10.0.0.1"},"afi":"ipv4","safi":"unicast"}
```

**Action:**
```
peer !10.0.0.1 refresh ipv4/unicast
```

Request route refresh from all other peers. Their UPDATEs will be forwarded to requesting peer.

---

## Example Flow

```
1. A announces 10.0.0.0/24 (msg-id=100)
   → peer !A forward update-id 100
   → B, C, D receive route

2. B announces 10.0.0.0/24 (msg-id=200)
   → peer !B forward update-id 200
   → A, C, D receive route (replaces A's in their Adj-RIB-In)

3. B withdraws (msg-id=201)
   → peer !B forward update-id 201
   → A, C, D withdraw the route
   → Note: A's original route is gone (last-wins limitation)

4. A re-announces (msg-id=102)
   → peer !A forward update-id 102
   → B, C, D receive route again

5. C session down/up
   → peer C replay
   → C receives all cached UPDATEs (from A, B, D)
```

---

## Command Registration

### Startup

```
#1 capability route-refresh
#2 register command "rr status" description "Show RS status"
#3 register command "rr peers" description "Show peer states"
```

### Commands

| Command | Action |
|---------|--------|
| `rr status` | Show RS is running |
| `rr peers` | List peers and up/down state |

Note: No `rib show/clear` - RS has no RIB.

---

## Config

```
process rr {
    run "zebgp api rr";
    encoder json;
}

peer * {
    api rr {
        receive { update; state; refresh; }
        send { update; }
    }
    capability {
        route-refresh;
    }
}
```

---

## Implementation

### File Structure

```
pkg/api/apiutil/           # Shared types and utilities
├── rib.go                 # RIB type (Insert, Remove, GetPeerRoutes, etc.)
├── rib_test.go
├── peer.go                # PeerState type
├── route.go               # Route type
├── event.go               # Event parsing from JSON
└── event_test.go

pkg/api/rr/                # Route Server plugin
├── server.go              # RouteServer (uses ribIn)
└── server_test.go

pkg/api/persist/           # Persistence plugin
├── server.go              # PersistServer (uses ribOut)
└── server_test.go

cmd/zebgp/
├── api.go                 # "zebgp api" subcommand group
├── api_rr.go              # "zebgp api rr" entry point
└── api_persist.go         # "zebgp api persist" entry point
```

### Shared Code (pkg/api/apiutil/)

Code shared between `rr` and `persist`:
- `RIB` - route storage with peer isolation
- `PeerState` - peer tracking with capabilities/families
- `Route` - route with MsgID, Family, Prefix
- `Event` parsing - JSON event deserialization

---

## Phases (All)

| Phase | Plugin | Tasks | Effort |
|-------|--------|-------|--------|
| 1 | shared | RIB, PeerState, Event types in `pkg/api/apiutil/` | 0.25 day |
| 2 | rr | Event loop, UPDATE forwarding | 0.5 day |
| 3 | rr | Peer state, peer down withdrawals | 0.5 day |
| 4 | rr | Route refresh, EOR on peer up | 0.25 day |
| 5 | rr | Command registration | 0.25 day |
| 6 | persist | Sent event handling, ribOut | 0.5 day |
| 7 | persist | Peer up replay + EOR | 0.25 day |
| 8 | core | `type: "sent"` event, source filtering | 0.5 day |
| 9 | core | msg-id cache (60s TTL, reset on use) | 0.25 day |
| 10 | core | Capability validation on startup | 0.5 day |
| **Total** | | | **3.75 days** |

---

## Known Limitations

### Last-Wins Semantics

Routes are forwarded in order received. If:
1. A announces route (good)
2. B announces same prefix (worse)
3. Peers have B's route (last received)

**Mitigation:** Acceptable for IX use case. Routes converge quickly.

### No Best-Path Selection

RS does not select best route. All routes forwarded.

**Mitigation:** Peers do their own selection.

### No Per-Peer Filtering

All peers receive same routes (except source exclusion).

**Future:** Could add policy later if needed.

### Session Recovery Not Optimized

On peer up, `replay` sends all cached UPDATEs, not just those relevant to the peer.

**Acceptable:** Simplicity over efficiency. Peers handle duplicates.

---

## Comparison: Forward-All vs Best-Path

| Aspect | Forward-All (this spec) | Best-Path |
|--------|------------------------|-----------|
| Complexity | Simple | Complex |
| Memory | O(prefixes × peers) | O(prefixes × peers) |
| CPU | Minimal | Best-path calculation |
| Zero-copy | Always | Sometimes (staleness) |
| Optimality | Last-wins | Always best |
| Failover | Needs re-announce | Automatic |

**Choice:** Forward-All for simplicity. Same memory (need RIB for replay), but no CPU for best-path.

---

## Future: ADD-PATH Support

With ADD-PATH, RS could forward all routes and peers keep all of them:
- No last-wins problem
- Peers see full route diversity
- Automatic failover

**Requires:** ADD-PATH capability negotiation, path-id assignment.

**Deferred:** Not in initial implementation.

---

## Dependencies

Requires ZeBGP engine support for:

### Commands

| Command | Description |
|---------|-------------|
| `peer <sel> forward update-id <id>` | Zero-copy forward cached UPDATE |
| `peer !<addr>` | Send to all except specified peer |
| `peer <sel> withdraw route <nlri>` | Send withdrawal |
| `peer <addr> eor <family>` | Send End-of-RIB marker |
| `peer <addr> announce raw <attrs> nlri <family> <nlri>` | Announce from wire bytes |
| `msg-id <id> retain` | Keep msg-id in cache |
| `msg-id <id> release` | Allow msg-id eviction |
| `msg-id <id> expire` | Remove msg-id immediately |
| `msg-id list` | List cached msg-ids |

### Events

| Event | Description |
|-------|-------------|
| `type: "update"` | UPDATE received with wire bytes |
| `type: "sent"` | UPDATE sent (for persist mode) |
| `type: "state"` | Peer up/down |
| `raw-attributes` | Base64 wire bytes |
| `raw-nlri` | Base64 wire bytes per family |
| `source-ctx-id` | Encoding context ID |
| `source` | API process name (for sent events) |

### msg-id Cache Policy

ZeBGP engine maintains a cache of UPDATEs for `forward update-id`. **API controls lifetime.**

| Aspect | Policy |
|--------|--------|
| **Default** | 60 seconds from last use |
| **Retained** | Never evicted until `release` or `expire` |
| **Reset on use** | Each `forward update-id` resets timer |
| **Eviction** | Only non-retained entries after 60s |

```go
type MsgCache struct {
    entries map[uint64]*CacheEntry
}

type CacheEntry struct {
    Data     []byte
    Retained bool      // API called "msg-id N retain"
    LastUsed time.Time // Reset on each forward
}

// Evict non-retained entries older than 60s
func (c *MsgCache) Evict() {
    cutoff := time.Now().Add(-60 * time.Second)
    for id, entry := range c.entries {
        if !entry.Retained && entry.LastUsed.Before(cutoff) {
            delete(c.entries, id)
        }
    }
}
```

**API controls:**
- `msg-id N retain` → entry.Retained = true
- `msg-id N release` → entry.Retained = false (can be evicted)
- `msg-id N expire` → delete immediately

### Shared Code Location

```
pkg/api/apiutil/
├── rib.go          # RIB type (shared)
├── rib_test.go
├── peer.go         # PeerState type (shared)
├── route.go        # Route type (shared)
└── event.go        # Event parsing (shared)

pkg/api/rr/
├── server.go       # RouteServer (uses ribIn)
└── server_test.go

pkg/api/persist/
├── server.go       # PersistServer (uses ribOut)
└── server_test.go

cmd/zebgp/
├── api_rr.go       # zebgp api rr entry point
└── api_persist.go  # zebgp api persist entry point
```

---

## Phase 5: Outgoing Route Tracking (Test C Support)

### Problem

Test C (teardown) expects routes announced via API to be replayed on peer reconnect:
```
1. teardown.run sends "announce route 1.1.0.0/16"
2. Peer teardown
3. Peer reconnects → expects route 1.1.0.0/16 replayed
```

Without Adj-RIB-Out in ZeBGP core, this fails. RR plugin only tracks incoming routes from peers.

**Challenge:** RR plugin doesn't see announcements from other processes:
```
teardown.run → ZeBGP → peer
                 ↓
            RR plugin (doesn't see announcement)
```

### Solution: Sent Event Type

ZeBGP notifies API processes when routes are **sent** to peers.

#### New Event: `type: "sent"`

```json
{
  "type": "sent",
  "msg-id": 456,
  "source": "announce-routes",
  "sender": {"address": "10.0.0.1"},
  "peer": {"address": "127.0.0.1"},
  "direction": "sent",
  "message": {
    "update": {
      "announce": {
        "ipv4/unicast": {
          "1.1.1.1": {"1.1.0.0/16": {}}
        }
      }
    }
  }
}
```

**Fields:**
- `source`: Named API process that issued the command (e.g., `"announce-routes"`, `"rr"`)
- `sender`: Original peer context (for forwarded routes from peers)
- `peer`: Destination peer receiving the UPDATE

**Source values:**
| Origin | `source` | `sender` |
|--------|----------|----------|
| API process | process name | null |
| Static route | `"static"` | null |
| Forwarded peer route | forwarding process | original peer |

**Trigger:** After ZeBGP successfully sends UPDATE to peer.

**Filter:** Process only receives `sent` events where `source != processName`.
- `persist` receives `sent` when `source != "persist"`
- Prevents loops

**Config:** `receive { sent; }` subscribes to sent events.

#### PersistServer Structure

```go
// pkg/api/persist/server.go
type PersistServer struct {
    peers  map[string]*apiutil.PeerState
    ribOut *apiutil.RIB  // Routes SENT TO peers (for replay)
}
```

### Event Handling (zebgp api persist)

#### Sent Event

```go
func (ps *PersistServer) handleSent(event *Event) {
    peerAddr := event.Peer.Address.Peer

    // Extract routes from UPDATE and store for replay
    for family, routes := range event.Message.Update.Announce {
        for _, prefix := range extractPrefixes(routes) {
            ps.ribOut.Insert(peerAddr, &apiutil.Route{
                MsgID:  event.MsgID,
                Family: family,
                Prefix: prefix,
            })
        }
    }

    // Handle withdrawals - remove from ribOut
    for family, prefixes := range event.Message.Update.Withdraw {
        for _, prefix := range prefixes {
            ps.ribOut.Remove(peerAddr, family, prefix)
        }
    }
}
```

#### Peer Down (persist)

```go
func (ps *PersistServer) handleStateDown(peerAddr string) {
    // DO NOT clear ribOut - keep for replay on reconnect
    if peer, ok := ps.peers[peerAddr]; ok {
        peer.Up = false
    }
}
```

#### Peer Up (persist)

```go
func (ps *PersistServer) handleStateUp(peerAddr string) {
    // Replay all routes that were sent to this peer before
    for _, route := range ps.ribOut.GetPeerRoutes(peerAddr) {
        ps.send("peer %s forward update-id %d", peerAddr, route.MsgID)
    }

    // Send EOR for each family
    if peer, ok := ps.peers[peerAddr]; ok {
        for family := range peer.Families {
            ps.send("peer %s eor %s", peerAddr, family)
        }
        peer.Up = true
    }
}
```

### Test C Architecture

```
┌─────────────────┐     ┌───────────────────┐
│  teardown.run   │     │  zebgp api persist│
│  (announcer)    │     │  (state keeper)   │
└────────┬────────┘     └────────┬──────────┘
         │                       │
         │ announce route ...    │ receive { sent; state; }
         ▼                       ▼
┌─────────────────────────────────────────────┐
│               ZeBGP                         │
│  1. Receives announce from teardown.run     │
│  2. Sends UPDATE to peer                    │
│  3. Notifies persist: type="sent"           │
│  4. On peer reconnect: persist replays + EOR│
└─────────────────────────────────────────────┘
         │
         ▼
    ┌─────────┐
    │  Peer   │
    └─────────┘
```

`zebgp api persist` runs as **extra backend** alongside the original API process:
- `teardown.run`: Sends announcements (existing behavior)
- `zebgp api persist`: Tracks what was sent, replays on reconnect

### Test C Config

```
process announce-routes {
    run ./teardown.run;
    encoder json;
}

process persist {
    run "zebgp api persist";
    encoder json;
}

peer 127.0.0.1 {
    router-id 1.2.3.4;
    local-address 127.0.0.1;
    local-as 1;
    peer-as 1;

    api announce-routes {
        # teardown.run sends announcements (no receive needed)
    }
    api persist {
        receive { sent; state; }  # persist sees what was sent by others
        send { update; }          # persist can replay routes + EOR
    }
}
```

### ZeBGP Core Changes

| File | Change |
|------|--------|
| `pkg/api/types.go` | Add `MessageTypeSent = "sent"`, `Source`, `Sender` fields |
| `pkg/api/server.go` | `NotifySent(peer, msgID, update, source, sender)` method |
| `pkg/api/server.go` | Filter: skip process if `source == processName` |
| `pkg/api/server.go` | Parse `capability X` from process output |
| `pkg/api/process.go` | `ValidateCapabilities(required)` method |
| `pkg/api/process.go` | msg-id cache with 60s TTL, reset on use |
| `pkg/reactor/session.go` | Call `NotifySent()` after sending UPDATE |
| `pkg/reactor/reactor.go` | Call `ValidateCapabilities()` before starting sessions |
| `pkg/config/schema.go` | Add `sent` to receive block options |
| `pkg/config/bgp.go` | Derive required capabilities from peer config |

### Persist Plugin (pkg/api/persist/)

| Task | Description |
|------|-------------|
| `PersistServer` struct | Uses `ribOut` for sent routes |
| `handleSent()` | Store routes on sent events |
| `handleStateDown()` | Mark peer down, keep ribOut |
| `handleStateUp()` | Replay ribOut routes + send EOR |

---

## Phase 6: API Capability Validation (Bug Fix)

### Problem

**Current bug:** ZeBGP doesn't validate API capabilities. Config with `graceful-restart` starts even if no API process supports route-refresh.

### Solution: Advertise Model (Simple)

API processes advertise their capabilities on startup. ZeBGP validates against config requirements.

#### Startup Protocol

```
1. ZeBGP starts process
2. Process → ZeBGP: capability route-refresh (within 5s)
3. ZeBGP collects all process capabilities
4. ZeBGP validates: config requirements ⊆ process capabilities
5. If mismatch: refuse to start, log error
6. If OK: start peer sessions
```

**No query/response.** Processes just send their capabilities on startup.

#### API Process Startup

```go
// pkg/api/rr/server.go
func (rs *RouteServer) registerCommands() {
    // Advertise capabilities (not responding to query)
    rs.sendCommand("capability route-refresh")
    rs.sendCommand(`register command "rr status" description "Show RS status"`)
    rs.sendCommand(`register command "rr peers" description "Show peer states"`)
}
```

#### ZeBGP Validation

```go
// pkg/api/process.go
func (pm *ProcessManager) ValidateCapabilities(required []string) error {
    // Wait up to 5s for processes to advertise capabilities
    // Collect capabilities from all processes
    // Check required ⊆ advertised
    for _, cap := range required {
        if !pm.hasCapability(cap) {
            return fmt.Errorf("required capability %q not provided by any API process", cap)
        }
    }
    return nil
}
```

#### Config Validation

| Peer Config | Required Capability | Check |
|-------------|---------------------|-------|
| `graceful-restart enable` | `route-refresh` | API with `send { update; }` |
| `route-refresh enable` | `route-refresh` | API with `send { update; }` |
| `enhanced-route-refresh enable` | `enhanced-route-refresh` | API with `send { update; }` |

#### Error Message

```
ERROR: peer 192.168.1.1 has graceful-restart but no API supports route-refresh
  hint: add "api <process> { send { update; } }" with a capable process
        or disable graceful-restart
```

#### Implementation

| File | Change |
|------|--------|
| `pkg/api/process.go` | `ValidateCapabilities()`, capability collection |
| `pkg/api/server.go` | Parse `capability X` from processes |
| `pkg/reactor/reactor.go` | Call `ValidateCapabilities()` before starting sessions |
| `pkg/config/bgp.go` | Derive required capabilities from peer config |

---

## References

- RFC 4271 - BGP-4 (protocol)
- RFC 7947 - Internet Exchange BGP Route Server (semantics)
