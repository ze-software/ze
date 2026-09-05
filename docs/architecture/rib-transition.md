# Ze Design Transition: RIB in API Program

**Status:** Active Design Target
**Date:** 2026-01-04
**Affects:** All storage, forwarding, and RIB-related specs

---

## Executive Summary

Ze is transitioning to an architecture where **all RIB data and logic lives in API programs**, not the core engine. The engine remains a minimal BGP speaker focused on protocol handling ("parse on demand"), while API programs own route storage, policy decisions, and features like graceful-restart.

**Key principles:**
1. **Engine = Protocol** - FSM, parsing, wire I/O, capability negotiation
2. **API = Policy** - RIB storage, best-path selection, route refresh, GR state
3. **Polyglot** - API programs can be Go, Python, Rust, etc.
4. **Wire bytes** - Engine sends cached message IDs and raw UPDATE sections when requested by the binding

---

## Current State vs. Target State

### Current: Engine Has No RIB

The reactor no longer holds `ribIn` or `ribStore` fields. Route storage is
fully owned by plugins (`bgp-rib`, `bgp-adj-rib-in`). The engine retains only:
- **msg-id cache** (`recentUpdates`) -- wire bytes for zero-copy forwarding
<!-- source: internal/component/bgp/reactor/reactor.go -- Reactor struct (no RIB fields) -->
<!-- source: internal/component/bgp/reactor/recent_cache.go -- RecentUpdateCache -->

```
Engine receives UPDATE → Cache wire bytes (msg-id) → Send event to plugins → Plugins store/forward
```

| Component | Location | Status |
|-----------|----------|--------|
| Route storage | Plugins (bgp-rib, bgp-adj-rib-in) | Done |
| Best-path | Plugin (bgp-rs) | Done |
| GR state | Plugin (bgp-gr) | Done |
| Policy | Plugins | Done |
| Watchdog | Plugin (bgp-watchdog) | Done |

### Target: API Program Owns RIB

```
Engine receives UPDATE → Send event metadata + cached wire message ID → API stores route state → API decides forwarding
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
│                     Ze ENGINE (Minimal)                               │
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
                    │ YANG RPC events + cached wire message IDs
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
│  │  RIB (internal/component/bgp/rib/ as reference implementation)                      │   │
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

### 1. Ze Engine (Minimal)

What the engine does:
- **FSM**: Connect, OpenSent, OpenConfirm, Established states
- **Parsing**: Parse on demand (only when needed for API output)
- **Wire I/O**: Read/write BGP messages
- **Capabilities**: Negotiate with peers
- **Plugin Server**: newline-framed YANG RPC over `net.Pipe` or TLS connect-back, with DirectBridge for internal hot paths
- **msg-id Cache**: Store wire bytes, lifetime controlled by API
<!-- source: internal/component/bgp/reactor/reactor.go -- Reactor (engine core) -->
<!-- source: internal/component/bgp/fsm/ -- FSM states -->
<!-- source: internal/component/bgp/reactor/session.go -- Session wire I/O -->

What the engine does NOT do:
- ❌ Route storage (no RIB)
- ❌ Best-path selection
- ❌ Policy decisions
- ❌ Graceful restart state

### 2. API Program (Full RIB Owner)

The API program owns all routing logic:
- **Pool System**: Attribute/NLRI deduplication (see `POOL_ARCHITECTURE.md`)
- **RIB**: Route storage with pool handles (use `internal/component/bgp/rib/` as reference)
- **Policy**: Import/export filters, best-path selection
- **GR/RR**: Graceful restart state, route refresh handling
- **msg-id Control**: Tell engine which msg-ids to retain/expire

### 3. Wire Bytes Transfer

Engine sends formatted events to API programs. Message metadata includes the cache `id`; raw sections are present only when the binding format requests them:
```json
{
  "type": "bgp",
  "bgp": {
    "message": {"type": "update", "id": 123, "direction": "received"},
    "peer": {"address": "10.0.0.1", "local": {"address": "10.0.0.2", "as": 65000}, "remote": {"address": "10.0.0.1", "as": 65001}},
    "update": {"attr": {"origin": "igp"}, "nlri": {}}
  }
}
```

Internal plugins can receive `StructuredEvent.RawMessage`, which carries `AttrsWire` and `WireUpdate` for zero-copy section access. External plugins receive formatted text or JSON according to the process binding.

API stores wire bytes in pool for deduplication and zero-copy replay.

### 4. msg-id Cache Control

API controls msg-id lifetime via `bgp cache` commands:
```
# Keep msg-id until API releases it
bgp cache retain 123

# Release msg-id (can be evicted)
bgp cache release 123

# List all cached msg-ids
bgp cache list

# Expire specific msg-id immediately
bgp cache expire 123
```

See [msg-id Cache Control](#msg-id-cache-control) for details.

---

## What This Obsoletes

### Specs Now Architecture Docs

| Spec | New Location | Reason |
|------|--------------|--------|
| `spec-pool-handle-migration.md` | `plugin/rib-storage-design.md` | Pool belongs in API, not engine |
| `spec-unified-handle-nlri.md` | TBD | NLRI handles belong in API |
| `spec-context-full-integration.md` | TBD | Context tracking in API program |

### Engine Code to Simplify

| Component | Change |
|-----------|--------|
| `internal/component/bgp/rib/` | Keep as library for API programs |
| Route storage in reactor | Remove (API owns) |
| Best-path selection | Remove (API owns) |
| `buildRIBRouteUpdate` | Keep for route construction helpers |

### Patterns for API Programs

| Pattern | Description |
|---------|-------------|
| Store wire bytes | `pool.Intern(raw UPDATE sections from StructuredEvent or raw event fields)` |
| Forward by msg-id | `send bgp <selector> cached Y` (zero-copy where contexts match) |
| Announce raw | `send bgp X raw hex <update-payload-hex> type update` |
| Control msg-id | `bgp cache retain/release/expire N` |

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
| `plugin/rib-storage-design.md` | **MOVED** - Design reference for API programs |
| `spec-unified-handle-nlri.md` | Design valid, implement in API program |
| `spec-context-full-integration.md` | Context tracking in API program |
| `spec-attributes-wire.md` | Raw UPDATE sections when requested by binding format |
| `spec-encoding-context-impl.md` | Engine uses for negotiation |

**Note:** These specs describe valid designs - only the *location* changed from engine to API.

### Reference Implementations

| Component | Location | Purpose |
|-----------|----------|---------|
| Pool | `internal/component/bgp/attrpool/` | Go pool for API programs |
| RIB | `internal/component/bgp/rib/` | Route storage patterns |
| Route | `internal/component/bgp/rib/route.go` | Route with handles |
<!-- source: internal/component/bgp/attrpool/pool.go -- Pool implementation -->
<!-- source: internal/component/bgp/attrpool/handle.go -- Handle type -->

---

## Implementation Order

```
1. ✅ Engine: Add cached message IDs and optional raw UPDATE sections to events
        ↓
2. ✅ Engine: Add msg-id control commands (retain/release/expire/list)
        ↓
3. ✅ Engine: Add `send bgp X raw hex <payload> type update` command
        ↓
4. ✅ API: Update `bgp-rs` to use cached message IDs
        ↓
5. ✅ API: Update `bgp-rib` with msg-id control
        ↓
6. ⚠️  Engine: Remove RIB storage from reactor (API owns)
        — ribIn and ribStore removed. Watchdog extracted to bgp-watchdog plugin.
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
| `bgp cache retain <id>` | Keep msg-id until explicitly released |
| `bgp cache release <id>` | Allow msg-id to be evicted |
| `bgp cache expire <id>` | Remove msg-id immediately |
| `bgp cache list` | List all cached msg-ids with status |

### Lifecycle

```
1. Engine receives UPDATE, assigns msg-id, caches wire bytes
2. Engine sends event to API with msg-id
3. API stores route in RIB with msg-id reference
4. API sends: bgp cache retain 123
5. ... peer goes down, comes back up ...
6. API replays: send bgp X cached 123
7. When route withdrawn: bgp cache release 123
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
- Each `send bgp <selector> cached <id>` resets the 60s timer
- Retained msg-ids never evicted until `release` or `expire`

---

## API Program Examples

### Go (using internal/component/bgp/rib/)

```go
// Handle UPDATE event
func (s *Server) handleUpdate(event *Event) {
    // Read raw UPDATE sections from StructuredEvent.RawMessage or from
    // configured raw event fields on external process bindings.
    attrBytes := event.RawAttributes
    nlriBytes := event.RawNLRI

    // Store in pool
    attrHandle := s.pool.Intern(attrBytes)
    nlriHandle := s.pool.Intern(nlriBytes)

    // Create route
    route := &Route{
        AttrHandle:  attrHandle,
        NLRIHandle:  nlriHandle,
        MsgID:       event.MsgID,
    }
    s.rib.Insert(event.Peer, route)

    // Retain msg-id for replay
    s.send("bgp cache retain %d", event.MsgID)

    // Forward to other peers
    s.send("send bgp !%s cached %d", event.Peer, event.MsgID)
}
```

### Python (simple, no pool)

```python
def handle_update(event):
    # Read raw fields when the binding format includes them.
    bgp = event['bgp']
    msg_id = bgp['message']['id']
    peer = bgp['peer']['address']
    attr_bytes = event.get('raw-attributes', b'')
    nlri_bytes = event.get('raw-nlri', b'')

    # Store in dict (no dedup)
    route = {
        'attrs': attr_bytes,
        'nlri': nlri_bytes,
        'msg_id': msg_id,
    }
    rib[peer][route_key(event)] = route

    # Retain msg-id
    send(f"bgp cache retain {msg_id}")

    # Forward
    send(f"send bgp !{peer} cached {msg_id}")
```

---

## Raw Message Command

When msg-id cache is unavailable (long outage, cache evicted), API can send a preserved raw UPDATE payload:

```
send bgp 192.0.2.1 raw hex <update-payload-hex> type update
```

This bypasses normal UPDATE construction and validation. Use it only when the API has preserved the exact UPDATE payload and the engine cache is unavailable.

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

## Stability Guarantees

### Stable (API Contract)

| Component | Guarantee |
|-----------|-----------|
| Text command protocol | Stable - backwards compatible changes only |
| JSON event format | Stable - additive changes only |
| Plugin lifecycle protocol | Stable - 5-stage registration |

### Unstable (Internal)

| Component | Status |
|-----------|--------|
| Go package structure | May change without notice |
| Go types and interfaces | May change without notice |
| Internal wire representations | May change without notice |

**Implication:** External plugins should communicate through newline-framed YANG RPC over the process connection, not by importing Ze Go packages. This enables polyglot plugins and avoids coupling to internal structure.

---

## References

- `POOL_ARCHITECTURE.md` - Pool design for API programs
- `spec-api-rr.md` - Route Server implementation
- `CAPABILITY_CONTRACT.md` - GR/RR capability handling
- `internal/component/bgp/rib/` - Reference Go implementation

---

**Last Updated: 2026-03-01
