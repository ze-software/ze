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

### Watchdog initial announcement state

The watchdog plugin owns every route in an update block with a named watchdog.
Its `withdraw` setting defaults to `false`: omitted and explicit false announce
the route when the session reaches Established; true holds the route until an
explicit announce command. This matches ExaBGP's initial-state behavior.
The owner approved that choice on 2026-11-15 with "ok", in response to
"Omitted or withdraw false: announce" and "withdraw true: hold until explicitly
announced".

The config reader sets the initial pool flag. Existing peer-up and command
handlers send the routes through the SDK; the engine adds no watchdog policy.
Reload keeps an existing route's runtime announcement state. Newly added routes
use the initial state that their configuration selects.
<!-- source: internal/component/bgp/plugins/watchdog/config.go -- collectWatchdogRoutes, parseNLRIEntries -->
<!-- source: internal/component/bgp/plugins/watchdog/server.go -- handleStateUp -->
<!-- source: internal/component/bgp/plugins/watchdog/config_tx.go -- carryEntryState -->
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- update-block watchdog default -->

### Ordinary selected-route advertisement

Attach one `bgp-rib` process to the source and destination peers with the
[grants for ordinary transit](../guide/route-reflection.md#without-route-reflection).
The RIB selects the route for advertisement under RFC 4271 Sections 9.1.3
and 9.2.
<!-- source: internal/component/plugin/server/dissemination.go -- ResolveDisseminationOwner -->

The engine caches wire bytes. When an ordinary destination comes up, its
worker sends the current selection and subsequent changes through the final
engine writer.
Initial readiness follows those final receipts, including for an empty table.
Explicit route-server and reflector destinations retain their role writer.
Ordinary refresh reoffers the current selected snapshot and completes after
final writer receipts, as described in the [plugin process protocol](./api/process-protocol.md).
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- publishBestChanges -->
<!-- source: internal/component/bgp/plugins/rib/rib_ordinary_destination.go -- updateSelectedDestination -->
<!-- source: internal/component/bgp/plugins/rib/rib_selected_refresh.go -- requestRefresh, refreshMarker -->
<!-- source: internal/component/bgp/plugins/rs/server_forward.go -- batchForwardUpdateSkipped -->

### Owner ruling: a learned route needs the recipient's export policy

On 2026-11-16, Thomas chose how a BGP route crosses from one peer to another.
There is no BGP-to-BGP redistribution mechanism. The explicit decision is the
recipient's export policy, and it is required on every peer, internal ones
included: a peer with no configured export policy is sent no route Ze learned
from another peer.

This narrows the ruling above without reversing it. Best routes pass with no
route-server plugin, wherever the recipient's export policy accepts them.
RFC 4271 Section 9.1.3 allows the exclusion, and BIRD has the same default.
FRR advertises to an internal peer with no policy, so the two disagree and Ze
takes the explicit side.

On the bus side, `bgp-rib` receives a peer's routes and may send to a peer by
its registered participant defaults unless a `bus` block or allow-list overrides
it ([API architecture](./api/architecture.md), "Message Bus and Participant
Wiring"). Learned-route forwarding toward any peer still requires an explicit
export policy, and an override that cuts `bgp-rib` when a peer's export policy
or configured feature requires it is reported with the completing bus line.

The requirement binds learned routes only, by the same ruling. A local
announcement (API, configured or redistributed) and a destination served by an
explicit route-server (`rs-client`) or reflector (`route-reflector-client`) role
keep the default they have today, because an announcement, a redistribution rule
and a role configuration are each already an explicit operator decision. An
external session keeps RFC 8212 for every route.

The export final now takes an engine-derived route kind. On an internal
session, absent policy rejects ordinary learned output and permits explicit
local or role output; external sessions reject both. The policy dry run uses
the same rule for its read-only learned or local simulation. Validation, live
session warnings and doctor share the absence report. Learned-only policy
reload retires or restores ordinary learned output while preserving local output.
<!-- source: internal/component/bgp/filterapi/policy.go -- AcceptsExportAtEnd -->
<!-- source: internal/component/bgp/filterapi/export_kind.go -- UnconfiguredExportAccepts -->
<!-- source: internal/component/bgp/reactor/policy_dryrun.go -- PolicyDryRun -->
<!-- source: internal/component/bgp/reactor/rfc8212_warning.go -- RFC8212Absences -->
<!-- source: internal/component/bgp/reactor/policy_absence.go -- PolicyAbsences -->
<!-- source: internal/component/bgp/reactor/policy_selected_retire.go -- retireOrdinaryLearned -->
<!-- source: internal/component/bgp/plugins/rib/rib_selected_policy.go -- reofferLearned -->

### Owner ruling: local and learned route competition

On 2026-11-16, Thomas answered the choice between configured preferences and
explicit-local priority:

> Use your rule, in doubt do what FRR and Bird do if they agree

Eligible local announcements, configured redistribution and learned BGP routes
compete using Ze's configured route preferences. An explicit local announcement
has no unconditional priority until withdrawal.
<!-- source: internal/component/bgp/plugins/rib/rib_origins.go -- electOriginLocked, originPreferenceLocked -->

BGP best-path selection chooses among learned BGP paths before cross-protocol
competition. A route's distance override takes precedence over its protocol's
configured distance. Lower distance wins, then lower metric, then stable
ordering. These election rules do not authorize export of a system-table entry.
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- bgpLocRIBInstance -->
<!-- source: internal/core/rib/locrib/distance.go -- resolvedDistance -->
<!-- source: internal/core/rib/locrib/entry.go -- selectBest -->

Origin admission and destination scope remain required. An API announcement
targeted at one peer does not become eligible for every peer. A system route
needs configured redistribution before it can compete for BGP advertisement.
The selected route then passes through each recipient's export policy. If that
policy excludes the winner, withdraw the previous output. Ordinary transit
does not search for a losing candidate that the recipient would accept.
<!-- source: internal/component/bgp/plugins/rib/rib_origins.go -- originIntake, currentScopeLocked, electOriginLocked -->
<!-- source: internal/component/bgp/redistribute/consumer.go -- InjectRoute -->

Ze's configured protocol distances and per-route overrides feed the shared
comparison used by admitted-origin election.
<!-- source: internal/core/rib/locrib/preference.go -- PreferenceFor, Preference.Before -->
<!-- source: internal/component/sysrib/yang/ze-rib-conf.yang -- distance defaults -->

For ordinary selected output, admitted explicit local BGP origins use the
configured static distance class. Their origin identity, attributes and
recipient scope stay distinct. Admitted redistribution origins retain their
original protocol, metric and distance override; the RIB compares these captured
facts with its learned winner.
<!-- source: internal/component/bgp/plugins/rib/rib_origins.go -- newOriginContribution, originPreferenceLocked, electOriginLocked -->
<!-- source: internal/component/bgp/plugins/rib/rib_origin_preferences.go -- parseOriginPreferences, installOriginPreferences -->

Internal and forked RIB processes request the `bgp` and `rib` configuration
roots. A reload of either root keeps the installed settings from the other.
Removing `rib` restores the default distances through the config transaction.
<!-- source: internal/component/bgp/plugins/rib/rib.go -- runRIBPlugin -->
<!-- source: internal/component/bgp/plugins/rib/config_tx.go -- newConfigTx, parseRIBConfigSections -->

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
│  │  RIB (internal/component/bgp/plugins/rib/ is Ze's own)           │   │
│  │  • Routes with pool handles                                      │   │
│  │  • Adj-RIB-In per peer                                           │   │
│  │  • ribOut entries for replay                                     │   │
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

**Last Updated: 2026-11-16**
