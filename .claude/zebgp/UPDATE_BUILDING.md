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

For incoming UPDATE messages from peers:

```
conn.Read(readBuf) → WireUpdate(owns buffer) → pool.Get() → callback → API
       │                    │                      │            │
       │                    │                      │            └── RawMessage.WireUpdate
       │                    │                      └── Fresh buffer for next read
       │                    └── Takes ownership of slice (no copy!)
       └── Session's read buffer
```

**Files involved:**
- `pkg/reactor/session.go` - Buffer pool (`readBufPool`), `processMessage()`, `handleUpdate()`
- `pkg/api/wire_update.go` - `WireUpdate` struct with derived accessors
- `pkg/reactor/reactor.go` - `notifyMessageReceiver()` uses WireUpdate directly

**Key types:**
```go
// pkg/api/wire_update.go
type WireUpdate struct {
    payload     []byte           // UPDATE body (owns buffer)
    sourceCtxID bgpctx.ContextID
}

// Derived accessors (zero-copy slices)
func (u *WireUpdate) Withdrawn() []byte
func (u *WireUpdate) Attrs() *AttributesWire
func (u *WireUpdate) NLRI() []byte
func (u *WireUpdate) MPReach() MPReachWire
func (u *WireUpdate) MPUnreach() MPUnreachWire
```

**Zero-copy mechanism:**
1. Session reads into `readBuf`
2. Creates `WireUpdate` from slice (no copy)
3. Gets fresh buffer from pool
4. Callback receives `WireUpdate` - buffer ownership transferred
5. `RawMessage.WireUpdate` set directly (no copy in `notifyMessageReceiver`)

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
- `pkg/config/loader.go` - Config parsing, creates domain objects
- `pkg/reactor/peersettings.go` - Domain objects (FlowSpecRoute, StaticRoute, etc.)
- `pkg/reactor/peer.go` - Conversion functions (toFlowSpecParams, etc.)
- `pkg/bgp/message/update_build.go` - UpdateBuilder, *Params structs, Build*() methods

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
- `pkg/rib/route.go` - Route struct with wireBytes cache
- `pkg/bgp/context/` - EncodingContext, ContextID, Registry
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

**Conversion functions in `pkg/reactor/peer.go`:**
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
- `pkg/rib/grouping.go` - `GroupByAttributesTwoLevel()`, `RouteGroup`, `ASPathGroup`
- `pkg/reactor/reactor.go` - `sendRoutesWithLimit()`, `sendGroupedIPv4Unicast()`, `sendGroupedMPFamily()`
- `pkg/bgp/message/update_build.go` - `BuildGroupedUnicastWithLimit()`
- `pkg/bgp/message/chunk_mp_nlri.go` - `ChunkMPNLRI()` for MP family splitting

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

**Files involved:**
- `pkg/bgp/message/update_split.go` - `SplitUpdate()`, `SplitUpdateWithAddPath()`
- `pkg/bgp/message/chunk_mp_nlri.go` - `ChunkMPNLRI()` for family-aware NLRI parsing
- `pkg/reactor/peer.go` - `sendUpdateWithSplit()` integration

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

- `ENCODING_CONTEXT.md` - Context system for capability-dependent encoding
- `POOL_ARCHITECTURE.md` - Attribute/NLRI deduplication pools
- `MESSAGE_BUFFER_DESIGN.md` - Passthrough message handling
- `wire/MESSAGES.md` - Wire format specification
- `wire/MP_NLRI_ORDERING.md` - MP attribute ordering rationale

## Related Specs

- `plan/spec-attributes-wire.md` - Lazy-parsed wire attribute storage (forward path)
- `plan/spec-pool-handle-migration.md` - Future pool handle integration

---

**Created:** 2026-01-01
**Last Updated:** 2026-01-02
