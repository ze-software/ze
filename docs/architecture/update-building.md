# UPDATE Message Building Architecture

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Three Paths** | Receive (zero-copy ingest) → Forward (reflection) → Build (origination) |
| **Receive Path** | conn.Read → WireUpdate(owns buffer) → API/Cache (zero-copy) |
| **Build Path** | Config → *Params → UpdateBuilder.Build*() → Update{[]byte} |
| **Forward Path** | Route{wireBytes} → zero-copy if contexts match |
| **Key Insight** | Zero-copy from read buffer to API; high-volume forwarding uses wire cache |

**When to read full doc:** Understanding message building, *Params design, FlowSpec differences.

---

## Three Paths for UPDATE Messages

### Path 0: Receive Path (Zero-Copy Ingest)

For incoming messages from peers (applies to ALL message types):

```
buf := s.getReadBuffer()  ← Get from appropriate pool (4K/64K)
       ↓
conn.Read(buf)            ← Read directly into pool buffer
       ↓
err, kept := processMessage(buf)  ← Callback returns kept=true if caching
       ↓
if !kept: s.returnReadBuffer(buf) ← Return only if not cached
```

**Buffer pools (size-appropriate):**
```go
// internal/component/bgp/reactor/session.go
var readBufPool4K = sync.Pool{...}   // 4096 bytes (before Extended Message)
var readBufPool64K = sync.Pool{...}  // 65535 bytes (after Extended Message)

func ReturnReadBuffer(buf []byte)    // Exported for cache eviction
```

**Files involved:**
- `internal/component/bgp/reactor/session.go` - `getReadBuffer()`, `returnReadBuffer()`, `ReturnReadBuffer()`, `readAndProcessMessage()`, `processMessage()`
- `internal/component/plugin/wire_update.go` - `WireUpdate` struct with derived accessors
- `internal/component/bgp/reactor/reactor.go` - `notifyMessageReceiver()` takes buf ownership when caching
- `internal/component/bgp/reactor/recent_cache.go` - Returns buf to pool on eviction

**Key types:**
```go
// internal/component/plugin/wire_update.go
type WireUpdate struct {
    payload     []byte           // UPDATE body (slice into pool buffer)
    sourceCtxID bgpctx.ContextID
}

// internal/component/bgp/reactor/received_update.go
type ReceivedUpdate struct {
    WireUpdate   *api.WireUpdate  // Slices into poolBuf
    poolBuf      []byte           // Returned to pool on eviction
    SourcePeerIP netip.Addr       // Peer that sent this UPDATE
    ReceivedAt   time.Time        // When received
}

// Derived accessors (zero-copy slices)
// Return (nil, nil) for valid empty, (nil, error) for malformed
func (u *WireUpdate) Withdrawn() ([]byte, error)
func (u *WireUpdate) Attrs() (*AttributesWire, error)
func (u *WireUpdate) NLRI() ([]byte, error)
func (u *WireUpdate) MPReach() (MPReachWire, error)   // nil,nil if attr not present
func (u *WireUpdate) MPUnreach() (MPUnreachWire, error) // nil,nil if attr not present
```

**Buffer lifecycle (ownership transfer):**
1. Session gets buffer from appropriate pool (`getReadBuffer()`)
2. Session reads message into buffer
3. For UPDATE: creates `WireUpdate` from slice (no copy)
4. **Callback executes FIRST** (buffer always valid during callback)
5. Then cache `Add()` - returns `kept=true` if caching
6. If cached: cache owns buf
7. If not cached: session returns buffer to pool immediately

**Cache API:**
- `Add(update)` - cache takes ownership, returns buf to pool if full/rejected
- `Take(id)` - removes entry, transfers ownership to caller
- `Contains(id)` - check existence without taking ownership
- `Delete(id)` - remove and return buffer to pool
- `ReceivedUpdate.Release()` - caller returns buffer after `Take()`

**Critical ordering:** Callback before cache ensures buffer is valid during callback.
Cache `Take()` prevents use-after-free by transferring ownership.

### Path 1: Build Path (Local Origination)

For routes originating from config or API:

```
Config/API → Domain Object → *Params → UpdateBuilder.Build*() → Update
     │              │              │               │              │
     │              │              │               │              └── Contains raw []byte
     │              │              │               └── Packs to wire format
     │              │              └── Typed struct (UnicastParams, etc.)
     │              └── FlowSpecRoute, StaticRoute, etc.
     └── YAML config, CLI commands
```

**Files involved:**
- `internal/component/config/loader.go` - Config parsing, creates domain objects
- `internal/component/bgp/reactor/peersettings.go` - Domain objects (FlowSpecRoute, StaticRoute, etc.)
- `internal/component/bgp/reactor/peer.go` - Conversion functions (toFlowSpecParams, etc.)
- `internal/component/bgp/message/update_build.go` - UpdateBuilder, *Params structs, Build*() methods

**Flow example (FlowSpec):**
```go
// 1. Config loader pre-packs communities
route.CommunityBytes = buildFlowSpecCommunities(fr.Then)  // []byte

// 2. At send time, convert to params
params := message.FlowSpecParams{
    CommunityBytes: route.CommunityBytes,  // Pass through
    // ...
}

// 3. Build UPDATE
update := ub.BuildFlowSpec(params)  // Returns Update{PathAttributes: []byte}

// 4. Send
peer.SendUpdate(update)
```

### Path 2: Forward Path (Route Reflection)

For routes received from peers and forwarded:

```
Receive UPDATE → Parse → Route{wireBytes, sourceCtxID} → Forward
                   │              │                          │
                   │              │                          └── Zero-copy if contexts match
                   │              └── Cached original wire bytes
                   └── Store in RIB
```

**Files involved:**
- `internal/component/bgp/rib/route.go` - Route struct with wireBytes cache
- `internal/component/bgp/context/` - EncodingContext, ContextID, Registry
- `ENCODING_CONTEXT.md` - Detailed context system docs

**Flow example (route reflection):**
```go
// 1. Receive and store with wire cache
route := rib.NewRouteWithWireCache(nlri, nextHop, attrs, asPath, wireBytes, sourceCtxID)

// 2. Forward to peer - check context compatibility
if route.CanForwardDirect(peer.sendCtxID) {
    // Fast path: zero-copy
    attrBytes := route.WireBytes()
} else {
    // Slow path: re-encode
    attrBytes := route.PackAttributesFor(peer.sendCtxID)
}
```

---

## Why Two Paths?

| Concern | Build Path | Forward Path |
|---------|------------|--------------|
| Volume | Low (config rules) | High (millions of routes) |
| Frequency | Once at session start | Continuous |
| Optimization | Pre-pack at config time | Zero-copy forwarding |
| Key structure | *Params structs | Route.wireBytes cache |

**The forward path is where scale matters.** Route reflection of millions of routes needs zero-copy. The build path handles low-volume local origination.

---

## *Params Struct Design

### Consistent Types (Unicast, VPN, LabeledUnicast)

```go
type UnicastParams struct {
    Communities       []uint32  // Typed - packed at Build time
    ExtCommunityBytes []byte    // Raw - complex encoding
    OriginatorID      uint32    // Typed - simple fixed format
    ClusterList       []uint32  // Typed - simple fixed format
}
```

### FlowSpec Exception

```go
type FlowSpecParams struct {
    CommunityBytes    []byte   // Raw - pre-packed by config loader
    ExtCommunityBytes []byte   // Raw - complex encoding
    OriginatorID      uint32   // Typed - simple fixed format
    ClusterList       []uint32 // Typed - simple fixed format
}
```

**Why FlowSpec uses `CommunityBytes []byte`:**
1. FlowSpec routes are config-originated, low-volume
2. Config loader pre-packs once at load time
3. Build path passes through without repacking
4. Negligible optimization, but intentional design

**This does NOT affect route reflection** - received FlowSpec routes use Route.wireBytes like everything else.

---

## Domain Objects vs Params

| Layer | Purpose | Example |
|-------|---------|---------|
| Domain Objects | Store route config | `FlowSpecRoute`, `StaticRoute` |
| *Params | Build UPDATE message | `FlowSpecParams`, `UnicastParams` |
| Update | Wire format container | `Update{PathAttributes []byte}` |

**Conversion functions in `internal/component/bgp/reactor/peer.go`:**
```go
func toFlowSpecParams(r FlowSpecRoute) message.FlowSpecParams
func toStaticRouteUnicastParams(r StaticRoute, nf bool) message.UnicastParams
func toVPNParams(r VPNRoute) message.VPNParams
```

---

## Update Struct

The `Update` struct holds raw bytes ready for wire:

```go
type Update struct {
    rawData         []byte  // Full message for passthrough
    WithdrawnRoutes []byte  // Withdrawn prefixes
    PathAttributes  []byte  // Packed attributes
    NLRI            []byte  // Announced prefixes
}
```

All `Build*()` methods produce an `Update` with populated `[]byte` fields.

---

## Context-Dependent Encoding

Wire format depends on negotiated capabilities:

| Capability | Effect |
|------------|--------|
| ASN4 | 2-byte vs 4-byte AS numbers in AS_PATH |
| ADD-PATH | Path ID prefix in NLRI |
| Extended Message | >4096 byte messages |

**Build path:** `UpdateBuilder.Ctx` contains pack context
**Forward path:** `Route.sourceCtxID` vs `peer.sendCtxID` determines zero-copy eligibility

---

## Route Grouping (adj-rib-out → Peer)

When sending routes from adj-rib-out, routes with identical attributes are grouped into single UPDATE messages:

```
adj-rib-out Routes → GroupByAttributesTwoLevel() → ASPathGroups → BuildGrouped*()
       ↓                      ↓                         ↓              ↓
  []*rib.Route         []AttributeGroup           Same AS_PATH    Multiple NLRIs
                             ↓                     per UPDATE       per UPDATE
                        []ASPathGroup
```

**Complexity reduction:** O(routes) → O(routes/capacity)

| Family | Builder Method | Notes |
|--------|---------------|-------|
| IPv4 unicast | `BuildGroupedUnicastWithLimit()` | Uses UnicastParams |
| IPv6/VPN | `sendGroupedMPFamily()` | Packs into MP_REACH_NLRI |

**Files involved:**
- `internal/component/bgp/rib/grouping.go` - `GroupByAttributesTwoLevel()`, `RouteGroup`, `ASPathGroup`
- `internal/component/bgp/reactor/reactor.go` - `sendRoutesWithLimit()`, `sendGroupedIPv4Unicast()`, `sendGroupedMPFamily()`
- `internal/component/bgp/message/update_build.go` - `BuildGroupedUnicastWithLimit()`
- `internal/component/bgp/message/chunk_mp_nlri.go` - `ChunkMPNLRI()` for MP family splitting

**Config:** `group-updates true` (default) in peer settings.

---

## Summary

```
┌─────────────────────────────────────────────────────────────────┐
│                    BUILD PATH (Local Origination)               │
│                                                                 │
│  Config → FlowSpecRoute → FlowSpecParams → BuildFlowSpec()      │
│                ↓                ↓                 ↓             │
│         CommunityBytes    (pass-through)    Update{[]byte}      │
│                                                                 │
│  Volume: Low (tens of rules)                                    │
│  Optimization: Pre-pack at config time                          │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    FORWARD PATH (Route Reflection)              │
│                                                                 │
│  Receive → Route{wireBytes, sourceCtxID} → CanForwardDirect()?  │
│                        ↓                          ↓             │
│              Stored in RIB          YES: zero-copy wireBytes    │
│                                     NO:  PackAttributesFor()    │
│                                                                 │
│  Volume: High (millions of routes)                              │
│  Optimization: Zero-copy when contexts match                    │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    GROUPED SEND PATH (adj-rib-out)              │
│                                                                 │
│  Routes → GroupByAttributesTwoLevel() → BuildGrouped*WithLimit()│
│     ↓              ↓                            ↓               │
│  []*Route    []ASPathGroup              Multiple NLRIs/UPDATE   │
│                                                                 │
│  Volume: Medium (adj-rib-out replay, API announces)             │
│  Optimization: O(routes) → O(routes/capacity) UPDATEs           │
└─────────────────────────────────────────────────────────────────┘
```

---

## UPDATE Size Limiting

UPDATEs must respect max message size (4096 standard, 65535 with Extended Message). Two approaches:

### Option A: Proactive (Build Path)
```go
// Size-aware builder splits at build time
updates, err := ub.BuildGroupedUnicastWithLimit(params, maxSize)
for _, update := range updates {
    peer.SendUpdate(update)
}
```

### Option B: Reactive (Forward/Replay Path)
```go
// Split after building if oversized
update := buildRIBRouteUpdate(route, ...)
peer.sendUpdateWithSplit(update, maxSize, family)
```

> **Wire-Level Split (Implemented)**
>
> Forward path uses `SplitUpdate()` for oversized UPDATEs when forwarding
> to non-Extended Message peers. See `plan/learned/078-wireupdate-split.md`.

**Files involved:**
- `internal/component/bgp/message/update_split.go` - `SplitUpdate()`, `SplitUpdateWithAddPath()`
- `internal/component/bgp/message/chunk_mp_nlri.go` - `ChunkMPNLRI()` for family-aware NLRI parsing
- `internal/component/bgp/reactor/peer.go` - `sendUpdateWithSplit()` integration

**NLRI formats handled by ChunkMPNLRI:**
| SAFI | Format |
|------|--------|
| 1 (Unicast) | `[prefix-len][prefix-bytes]` or Add-Path: `[path-id:4][prefix-len][prefix-bytes]` |
| 4 (Labeled) | `[total-bits][labels][prefix-bytes]` |
| 128 (VPN) | `[total-bits][labels][RD:8][prefix-bytes]` |
| 70 (EVPN) | `[route-type][length][payload]` |
| 133 (FlowSpec) | `[length:1-2][components]` |
| 71 (BGP-LS) | `[nlri-type:2][length:2][payload]` |

**MP Attribute Ordering:** See `wire/MP_NLRI_ORDERING.md` - MP_REACH/MP_UNREACH can be placed at end of PathAttributes.

---

## Related Documentation

- `encoding-context.md` - Context system for capability-dependent encoding
- `pool-architecture.md` - Attribute/NLRI deduplication pools
- `message-buffer-design.md` - Passthrough message handling
- `wire/messages.md` - Wire format specification
- `wire/mp-nlri-ordering.md` - MP attribute ordering rationale

## Related Specs

- `plan/learned/057-spec-attributes-wire.md` - Lazy-parsed wire attribute storage (forward path)
- `plan/learned/059-spec-pool-handle-migration.md` - Pool handle integration (completed)
- `plan/learned/343-wireupdate-buffer-lifecycle.md` - Buffer pool get/return lifecycle (completed)
- `plan/learned/078-wireupdate-split.md` - Wire-level UPDATE splitting (completed)

---

**Created:** 2026-01-01
**Last Updated: 2026-01-30
