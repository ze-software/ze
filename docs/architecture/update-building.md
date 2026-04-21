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
- `internal/component/bgp/wireu/wire_update.go` - `WireUpdate` struct with derived accessors
- `internal/component/bgp/reactor/reactor.go` - `notifyMessageReceiver()` takes buf ownership when caching
- `internal/component/bgp/reactor/recent_cache.go` - Returns buf to pool on eviction
<!-- source: internal/component/bgp/reactor/session.go -- Session, getReadBuffer, ReturnReadBuffer -->
<!-- source: internal/component/bgp/wireu/wire_update.go -- WireUpdate struct -->
<!-- source: internal/component/bgp/reactor/recent_cache.go -- RecentUpdateCache -->

**Key types:**
```go
// internal/component/bgp/wireu/wire_update.go
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
<!-- source: internal/component/bgp/reactor/received_update.go -- ReceivedUpdate struct -->

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
- `internal/component/bgp/reactor/peersettings.go` - Domain objects (FlowSpecRoute, StaticRoute, etc.)
- `internal/component/bgp/reactor/peer.go` - Conversion functions (toFlowSpecParams, etc.)
- `internal/component/bgp/message/update_build.go` - UpdateBuilder, *Params structs, Build*() methods
<!-- source: internal/component/bgp/reactor/peersettings.go -- FlowSpecRoute, StaticRoute -->
<!-- source: internal/component/bgp/reactor/peer.go -- toFlowSpecParams, toStaticRouteUnicastParams -->

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
<!-- source: internal/component/bgp/rib/route.go -- Route struct, CanForwardDirect -->
<!-- source: internal/component/bgp/context/registry.go -- ContextID, Registry -->

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

## Scratch Contract (Builder and Splitter)

The `UpdateBuilder` and `Splitter` own a **reusable scratch buffer** that backs every
variable-size `[]byte` they emit. Understanding this contract is mandatory before
modifying any `Build*` method or consuming a returned `*Update`.

<!-- source: internal/component/bgp/message/update_build.go -- UpdateBuilder, scratch, alloc -->
<!-- source: internal/component/bgp/message/update_split.go -- Splitter, Split -->

### Why scratch, not `make`

Every `Update.PathAttributes` and `Update.NLRI` is a wire-facing variable-size byte
slice. Per `ai/rules/design-principles.md` ("No `make` where pools exist"), such
allocations must come from a bounded pool. The builder's `scratch` IS that pool: one
buffer per builder, sized to `wire.StandardMaxSize` (4096) on first use, grown on
demand for Extended Message peers.

### Lifetime invariant (MUST understand before using)

| Object | Valid from | Invalidated by |
|--------|-----------|----------------|
| `update.PathAttributes` | return of `Build*` | next `Build*` call on the same builder |
| `update.NLRI` | return of `Build*` | next `Build*` call on the same builder |
| Splitter chunk's `PathAttributes` | fires via `emit(chunk)` | return of `emit` callback (next chunk's build reuses scratch) |

**Consequence:** callers MUST consume the Update (WriteTo, copy out, or hand to
SendUpdate which copies internally) before the next `Build*` on the same builder,
and splitter callers MUST complete `emit(chunk)` before the callback returns.

### Grow semantics (stranded backings are safe)

`alloc(n)` on overflow does `make(newSize) + copy + swap`. Sub-slices returned before
the grow still reference the OLD backing; the new backing holds everything allocated
after. The old backing stays alive (GC-pinned by those sub-slices) until all emitted
Updates are discarded. A single `Update` returned from one `Build*` call may therefore
have `PathAttributes` and `NLRI` pointing to two different arrays -- this is memory-safe
and byte-correct. Treat slices as opaque; never assume they share backing.

### Callback-builder offset protocol

`BuildGroupedUnicast` and `BuildGroupedMVPN` (and `Splitter.Split`) emit multiple
Updates per outer call, all sharing a subset of scratch. To keep this safe without
re-building shared attributes per chunk:

| Region | Offset | Lifetime |
|--------|-------|----------|
| attrBytes (shared across all chunks in batch) | `scratch[0:A)` | Full outer-call duration |
| per-Update NLRI / chunk PathAttributes | `scratch[A:)` | Until the callback returns |

After each callback returns, the builder resets `off` to **A** (the end of the shared
attribute region), NOT to 0. The next chunk's bytes overwrite the previous chunk's
NLRI region but leave the shared attribute region untouched, so the next emitted
Update still has a valid `PathAttributes` pointing at `scratch[0:A)`.

### Who owns a builder/splitter

| Role | Owner | Scope |
|------|-------|-------|
| UpdateBuilder | Short-lived, one per build-group | Created, used for a batch of builds, discarded |
| Splitter | Long-lived, one per peer (or forward worker) | Created at peer up, retained across sessions |

Builders are cheap to allocate; splitters amortise scratch across millions of
split operations and should NOT be created per-call.

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

## Cross-Peer Update Groups

Update groups are an orthogonal optimization that sits above route grouping. Where route grouping packs multiple NLRIs into a single UPDATE for one peer, update groups share a single UPDATE build across multiple peers.

### GroupKey

Each established peer is assigned a GroupKey combining two fields:

| Field | Source | Purpose |
|-------|--------|---------|
| `CtxID` | `peer.sendCtxID` (ContextID) | Encodes all encoding-relevant capability differences: ASN4, ADD-PATH mode, Extended Message, Extended Next Hop, iBGP/eBGP, and ASN values |
| `PolicyKey` | Currently `0` for all peers | Reserved for future per-peer outbound policy differentiation |

Peers with the same GroupKey produce bit-identical UPDATE wire bytes for the same route set and can share a single build.
<!-- source: internal/component/bgp/reactor/update_group.go -- GroupKey struct, CtxID and PolicyKey fields -->

### Group Lifecycle

The reactor maintains an `UpdateGroupIndex` that maps GroupKey to a set of member peers:

| Event | Action |
|-------|--------|
| Peer session established | Reactor calls `updateGroups.Add(peer)` using the peer's `sendCtxID` |
| Peer session closed | Reactor calls `updateGroups.Remove(peer)` before clearing encoding contexts |
| Group becomes empty | Group entry is deleted from the index |

The index is a simple map with no goroutines or channels. It is accessed only from the reactor event loop.
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- Add on established, Remove on closed (before clearEncodingContexts) -->
<!-- source: internal/component/bgp/reactor/update_group.go -- UpdateGroupIndex, Add, Remove -->

### Group-Aware Build Path (AnnounceNLRIBatch / WithdrawNLRIBatch)

When update groups are enabled, the batch API groups peers by build-equivalent parameters (encoding context, next-hop resolution, AS_PATH form) and builds the UPDATE once per parameter set. All peers sharing those parameters receive the same pre-built wire bytes.

When disabled or when each peer has a unique context, the code falls back to per-peer building with no behavior change.
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- groupsEnabled check, announceBuildKey, withdrawBuildKey -->

### Group-Aware Forward Path (ForwardUpdate)

When forwarding a received UPDATE to multiple peers, the forward path caches the per-context body computation. For peers sharing the same destination context, the context compatibility check and any re-encoding happen once. The cached result is reused for all group members.
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- fwdBodyCache, fwdBodyCacheKey -->

### Env Var Gating

Update groups are controlled by `ze.bgp.reactor.update-groups` (boolean, default `true`). The reactor reads this at startup via `NewUpdateGroupIndexFromEnv()`. When false, the `UpdateGroupIndex` reports disabled and all group-aware code paths fall back to per-peer behavior.

ExaBGP migrated configs inject `update-groups false` in the environment reactor block to preserve ExaBGP's per-peer UPDATE semantics.
<!-- source: internal/component/bgp/reactor/update_group.go -- NewUpdateGroupIndexFromEnv, Enabled -->
<!-- source: internal/exabgp/migration/migrate.go -- injectUpdateGroupsDisabled -->

### Relationship to Route Grouping

| Concept | Scope | Config | Purpose |
|---------|-------|--------|---------|
| Route grouping | Within one UPDATE for one peer | `group-updates` per peer | Pack multiple NLRIs with identical attributes into one UPDATE |
| Update groups | Across peers | `ze.bgp.reactor.update-groups` global | Build UPDATE once, send to all peers with same encoding context |

Both optimizations can be active simultaneously and are independent.

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
<!-- source: internal/component/bgp/message/update_split.go -- SplitUpdate -->
<!-- source: internal/component/bgp/message/chunk_mp_nlri.go -- ChunkMPNLRI -->

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
