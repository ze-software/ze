# ZeBGP Design Transition: RIB in API Program

**Status:** Active Design Target
**Date:** 2026-01-04
**Affects:** All storage, forwarding, and RIB-related specs

---

## Executive Summary

ZeBGP is transitioning to an architecture where **all RIB data and logic lives in API programs**, not the core engine. The engine remains a minimal BGP speaker focused on protocol handling ("parse on demand"), while API programs own route storage, policy decisions, and features like graceful-restart.

**Key principles:**
1. **Engine = Protocol** - FSM, parsing, wire I/O, capability negotiation
2. **API = Policy** - RIB storage, best-path selection, route refresh, GR state
3. **Polyglot** - API programs can be Go, Python, Rust, etc.
4. **Wire bytes** - Engine sends base64-encoded wire bytes for pool storage in API

---

## Current State vs. Target State

### Current: Engine Owns RIB

```
Engine receives UPDATE → Parse → Store in pkg/rib/ → Forward
```

| Component | Location | Issue |
|-----------|----------|-------|
| Route storage | Engine (pkg/rib/) | Tightly coupled |
| Best-path | Engine | Fixed logic |
| GR state | Engine | Cannot customize |
| Policy | Limited | Hard to extend |

### Target: API Program Owns RIB

```
Engine receives UPDATE → Send JSON+wire bytes → API stores in pool → API decides forwarding
```

| Component | Location | Benefit |
|-----------|----------|---------|
| Route storage | API program | Flexible, polyglot |
| Best-path | API program | Custom algorithms |
| GR state | API program | Full control |
| Policy | API program | Unlimited flexibility |
| Pool dedup | API program | Memory efficiency |

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     ZeBGP ENGINE (Minimal)                               │
│                                                                         │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐     │
│  │  FSM            │    │  Parser         │    │  Wire I/O       │     │
│  │  (per peer)     │    │  (on demand)    │    │  (reader/writer)│     │
│  └─────────────────┘    └─────────────────┘    └─────────────────┘     │
│                                                                         │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐     │
│  │  Capability     │    │  API Socket     │    │  msg-id Cache   │     │
│  │  Negotiation    │    │  Server         │    │  (API-controlled)│    │
│  └─────────────────┘    └─────────────────┘    └─────────────────┘     │
│                                                                         │
│  NO RIB  │  NO Route Storage  │  NO Best-Path  │  NO Policy           │
└─────────────────────────────────────────────────────────────────────────┘
                    │ JSON events + base64 wire bytes
                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                     API PROGRAM (Full RIB Owner)                         │
│                     (Go, Python, Rust, etc.)                            │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  Pool System (POOL_ARCHITECTURE.md)                              │   │
│  │  • Attribute deduplication                                       │   │
│  │  • Wire-canonical storage                                        │   │
│  │  • Double-buffer compaction                                      │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  RIB (pkg/rib/ as reference implementation)                      │   │
│  │  • Routes with pool handles                                      │   │
│  │  • IncomingRIB per peer                                          │   │
│  │  • OutgoingRIB for replay                                        │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  Policy Engine                                                   │   │
│  │  • Best-path selection                                           │   │
│  │  • Import/export filters                                         │   │
│  │  • Route manipulation                                            │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  Graceful Restart / Route Refresh                                │   │
│  │  • State preservation across peer restarts                       │   │
│  │  • msg-id lifetime control                                       │   │
│  │  • EOR management                                                │   │
│  └─────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
                    │ Commands (forward, announce, withdraw)
                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                     PEER SESSIONS                                        │
│                                                                         │
│  Peer A ◄──────── Engine ────────► Peer B                              │
│  Peer C ◄──────────┘  └──────────► Peer D                              │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Key Components

### 1. ZeBGP Engine (Minimal)

What the engine does:
- **FSM**: Connect, OpenSent, OpenConfirm, Established states
- **Parsing**: Parse on demand (only when needed for API output)
- **Wire I/O**: Read/write BGP messages
- **Capabilities**: Negotiate with peers
- **API Server**: Unix socket, JSON protocol
- **msg-id Cache**: Store wire bytes, lifetime controlled by API

What the engine does NOT do:
- ❌ Route storage (no RIB)
- ❌ Best-path selection
- ❌ Policy decisions
- ❌ Graceful restart state

### 2. API Program (Full RIB Owner)

The API program owns all routing logic:
- **Pool System**: Attribute/NLRI deduplication (see `POOL_ARCHITECTURE.md`)
- **RIB**: Route storage with pool handles (use `pkg/rib/` as reference)
- **Policy**: Import/export filters, best-path selection
- **GR/RR**: Graceful restart state, route refresh handling
- **msg-id Control**: Tell engine which msg-ids to retain/expire

### 3. Wire Bytes Transfer

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

**Note:** `raw-nlri` is a map keyed by family since UPDATE can contain multiple address families.

API stores wire bytes in pool for deduplication and zero-copy replay.

### 4. msg-id Cache Control

API controls msg-id lifetime via commands:
```
# Keep msg-id until API releases it
msg-id 123 retain

# Release msg-id (can be evicted)
msg-id 123 release

# List all cached msg-ids
msg-id list

# Expire specific msg-id immediately
msg-id 123 expire
```

See [msg-id Cache Control](#msg-id-cache-control) for details.

---

## What This Obsoletes

### Specs Now Obsolete

| Spec | Reason |
|------|--------|
| `spec-pool-handle-migration.md` | Pool in API, not engine |
| `spec-unified-handle-nlri.md` | NLRI handles in API |
| `spec-context-full-integration.md` | Context in API program |

### Engine Code to Simplify

| Component | Change |
|-----------|--------|
| `pkg/rib/` | Keep as library for API programs |
| Route storage in reactor | Remove (API owns) |
| Best-path selection | Remove (API owns) |
| `buildRIBRouteUpdate` | Keep for API "announce raw" |

### Patterns for API Programs

| Pattern | Description |
|---------|-------------|
| Store wire bytes | `pool.Intern(base64Decode(event.RawAttributes))` |
| Forward by msg-id | `peer X forward update-id Y` (zero-copy) |
| Announce raw | `peer X announce raw <attrs> nlri <family> <nlri>` |
| Control msg-id | `msg-id N retain/release/expire` |

---

## Spec Alignment

### Active Specs

| Spec | Role | Status |
|------|------|--------|
| `spec-api-rr.md` | Route Server with RIB | **PRIMARY** |
| `POOL_ARCHITECTURE.md` | Pool design for API | Reference |
| `spec-rfc9234-role.md` | Role capability | Independent |
| `phase0-peer-callbacks.md` | Peer lifecycle | Independent |

### Location Changed (Engine → API)

| Spec | Status |
|------|--------|
| `spec-pool-handle-migration.md` | Design valid, implement in API program |
| `spec-unified-handle-nlri.md` | Design valid, implement in API program |
| `spec-context-full-integration.md` | Context tracking in API program |
| `spec-attributes-wire.md` | Wire bytes via base64 in events |
| `spec-encoding-context-impl.md` | Engine uses for negotiation |

**Note:** These specs describe valid designs - only the *location* changed from engine to API.

### Reference Implementations

| Component | Location | Purpose |
|-----------|----------|---------|
| Pool | `pkg/rib/pool/` | Go pool for API programs |
| RIB | `pkg/rib/` | Route storage patterns |
| Route | `pkg/rib/route.go` | Route with handles |

---

## Implementation Order

```
1. Engine: Add raw-attributes/raw-nlri to UPDATE events
        ↓
2. Engine: Add msg-id control commands (retain/release/expire/list)
        ↓
3. Engine: Add "peer X announce raw <attrs> nlri <nlri>" command
        ↓
4. API: Update zebgp api rr to use wire bytes + pool
        ↓
5. API: Update zebgp api persist with msg-id control
        ↓
6. Engine: Remove RIB storage from reactor (API owns)
        ↓
7. Docs: Update all specs to reflect new architecture
```

---

## Memory Model

### Engine (Minimal Footprint)

```
Per peer:
  FSM state:       ~100 bytes
  Buffers:         ~8 KB (read/write)
  Capabilities:    ~200 bytes
  Total:           ~8.5 KB per peer

msg-id cache:
  Per entry:       ~200 bytes (wire bytes + metadata)
  Typical:         1000 entries × 200 = 200 KB
  Max (retained):  Controlled by API
```

### API Program (Full RIB)

```
Per route (with pool):
  attrHandle:      4 bytes
  nlriHandle:      4 bytes
  sourceCtxID:     2 bytes
  msgID:           8 bytes
  Total:           ~18 bytes

1M routes:         ~18 MB
Unique attrs:      ~100K × 150 bytes = 15 MB (shared in pool)
Total:             ~33 MB

Savings:           90%+ vs storing full attributes per route
```

### Polyglot Considerations

Python/Rust API programs won't use Go pool, but can implement equivalent:
- Python: `dict` with wire bytes keys
- Rust: `HashMap<Vec<u8>, Handle>`
- Simple: No dedup, store wire bytes per route (~300 MB for 1M routes)

---

## msg-id Cache Control

The engine maintains a cache of UPDATE wire bytes indexed by msg-id. API programs control cache lifetime.

### Commands

| Command | Description |
|---------|-------------|
| `msg-id <id> retain` | Keep msg-id until explicitly released |
| `msg-id <id> release` | Allow msg-id to be evicted |
| `msg-id <id> expire` | Remove msg-id immediately |
| `msg-id list` | List all cached msg-ids with status |

### Lifecycle

```
1. Engine receives UPDATE, assigns msg-id, caches wire bytes
2. Engine sends event to API with msg-id
3. API stores route in RIB with msg-id reference
4. API sends: msg-id 123 retain
5. ... peer goes down, comes back up ...
6. API replays: peer X forward update-id 123
7. When route withdrawn: msg-id 123 release
```

### List Output

```json
{
  "msg-ids": [
    {"id": 123, "retained": true, "size": 156, "age": "5m32s"},
    {"id": 124, "retained": false, "size": 89, "age": "2s"},
    {"id": 125, "retained": true, "size": 234, "age": "1h15m"}
  ]
}
```

### Default Behavior

- msg-ids NOT retained are evicted after 60 seconds of no use
- Each `forward update-id` resets the 60s timer
- Retained msg-ids never evicted until `release` or `expire`

---

## API Program Examples

### Go (using pkg/rib/)

```go
// Handle UPDATE event
func (s *Server) handleUpdate(event *Event) {
    // Decode wire bytes
    attrBytes, _ := base64.StdEncoding.DecodeString(event.RawAttributes)
    nlriBytes, _ := base64.StdEncoding.DecodeString(event.RawNLRI)

    // Store in pool
    attrHandle := s.pool.Intern(attrBytes)
    nlriHandle := s.pool.Intern(nlriBytes)

    // Create route
    route := &Route{
        AttrHandle:  attrHandle,
        NLRIHandle:  nlriHandle,
        MsgID:       event.MsgID,
        SourceCtxID: event.SourceCtxID,
    }
    s.rib.Insert(event.Peer, route)

    // Retain msg-id for replay
    s.send("msg-id %d retain", event.MsgID)

    // Forward to other peers
    s.send("peer !%s forward update-id %d", event.Peer, event.MsgID)
}
```

### Python (simple, no pool)

```python
def handle_update(event):
    # Decode wire bytes
    attr_bytes = base64.b64decode(event['raw-attributes'])
    nlri_bytes = base64.b64decode(event['raw-nlri'])

    # Store in dict (no dedup)
    route = {
        'attrs': attr_bytes,
        'nlri': nlri_bytes,
        'msg_id': event['msg-id'],
        'source_ctx_id': event['source-ctx-id'],
    }
    rib[event['peer']][route_key(event)] = route

    # Retain msg-id
    send(f"msg-id {event['msg-id']} retain")

    # Forward
    send(f"peer !{event['peer']} forward update-id {event['msg-id']}")
```

---

## Announce Raw Command

When msg-id cache is unavailable (long outage, cache evicted), API can announce raw wire bytes:

```
peer 10.0.0.1 announce raw <base64-attrs> nlri ipv4/unicast <base64-nlri>
```

Family is required for proper UPDATE construction. This allows API to rebuild UPDATEs from its pool without needing engine cache.

---

## Benefits of This Architecture

| Benefit | Description |
|---------|-------------|
| **Separation** | Engine = protocol, API = policy |
| **Polyglot** | API in any language (Go, Python, Rust) |
| **Flexibility** | Custom best-path, filters, GR handling |
| **Testability** | RIB logic tested independently |
| **Scalability** | API can run on separate process/machine |
| **Simplicity** | Engine stays minimal and stable |

---

## References

- `POOL_ARCHITECTURE.md` - Pool design for API programs
- `spec-api-rr.md` - Route Server implementation
- `CAPABILITY_CONTRACT.md` - GR/RR capability handling
- `pkg/rib/` - Reference Go implementation

---

**Last Updated:** 2026-01-04
