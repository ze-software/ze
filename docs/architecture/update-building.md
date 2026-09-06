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
     │              │              └── Typed struct (UnicastParams, PluginParams, etc.)
     │              └── PluginRoute, StaticRoute, etc.
     └── YAML config, CLI commands
```

The MP_REACH exotic families (MUP, VPLS, MVPN, FlowSpec, SR-Policy) all share one
generic path: the family plugin's config-route parser pre-builds the NLRI and the
family-specific path attributes, which flow through `reactor.PluginRoute` →
`message.PluginParams` → `UpdateBuilder.BuildPlugin()`. There is no per-family
`Build*()` in the message package for these families (see plugin-self-containment).

**Files involved:**
- `internal/component/bgp/reactor/peer_settings.go` - Domain objects (PluginRoute, StaticRoute, etc.)
- `internal/component/bgp/reactor/peer_static_routes.go` - Conversion functions (toPluginParams, etc.)
- `internal/component/bgp/message/update_build.go` - UpdateBuilder, *Params structs, Build*() methods
- `internal/component/bgp/message/update_build_plugin.go` - BuildPlugin + PluginParams (generic exotic-family path)
<!-- source: internal/component/bgp/reactor/peer_settings.go -- PluginRoute, StaticRoute -->
<!-- source: internal/component/bgp/reactor/peer_static_routes.go -- toPluginParams -->
<!-- source: internal/component/bgp/message/update_build_plugin.go -- BuildPlugin, PluginParams -->

**Flow example (FlowSpec via the generic plugin path):**
```go
// 1. The flowspec plugin's config parser pre-builds the NLRI + action attrs.
pr, _ := parser(registry.ConfigRouteRequest{Content: tokens, ExtCommunity: ec})

// 2. At send time, convert to params (NLRI + RawAttrs pre-built by the plugin).
params := message.PluginParams{AFI: 1, SAFI: 133, NLRI: pr.NLRI, RawAttrs: rawAttrs}

// 3. Build UPDATE (BuildPlugin owns only ORIGIN/AS_PATH/LOCAL_PREF/MP_REACH).
update := ub.BuildPlugin(params)  // Returns Update{PathAttributes: []byte}

// 4. Send
peer.SendUpdate(update)
```

### Path 2: Forward Path (Route Reflection)

For routes received from peers and forwarded, the wire bytes never enter a route
struct. The received `WireUpdate` carries them, and it carries the ContextID of
the session that produced them:

```
Receive UPDATE → WireUpdate{payload, sourceCtxID} → Forward
                        │                              │
                        │                              └── One build shared by every
                        │                                  destination on that ContextID
                        └── The read buffer, not a copy
```

`forwardUpdateCore` resolves the source context once for the whole fan-out, then
builds one body per `(destCtxID, wire, extended)` key and reuses it for every
destination that lands on the same key. Peers with identical negotiated
capabilities share one ContextID, so route reflection between same-capability
clients builds once and sends the result N times.

**Files involved:**
- `internal/component/bgp/reactor/reactor_api_forward.go` - ForwardUpdate, forwardUpdateCore, the per-context body cache
- `internal/component/bgp/reactor/update_group.go` - GroupKey, the ContextID that decides which peers share a build
- `internal/core/bgp/context/` - EncodingContext, ContextID, Registry
- `docs/architecture/encoding-context.md` - Detailed context system docs
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- forwardUpdateCore -->
<!-- source: internal/component/bgp/reactor/update_group.go -- GroupKey, UpdateGroupIndex -->
<!-- source: internal/core/bgp/context/registry.go -- ContextID, Registry -->

`rib.Route` takes no part in this path. It is the value a named commit and the
route API hand to `CommitService`, and it holds NLRI, next hop, attributes and
AS-PATH only.
<!-- source: internal/component/bgp/rib/route.go -- Route struct -->
<!-- source: internal/component/bgp/rib/commit.go -- CommitService, NewCommitService -->

**Flow example (named commit):**
```go
// 1. Build the route from what the operator asked for.
route := rib.NewRouteWithASPath(nlri, nextHop, attrs, asPath)

// 2. Group the routes and build one UPDATE per attribute group.
cs := rib.NewCommitService(peer, encodingContext, grouped)
stats, err := cs.Commit([]*rib.Route{route}, rib.CommitOptions{SendEOR: false})
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
| Domain Objects | Store route config | `PluginRoute`, `StaticRoute` |
| *Params | Build UPDATE message | `PluginParams`, `UnicastParams` |
| Update | Wire format container | `Update{PathAttributes []byte}` |

**Conversion functions in `internal/component/bgp/reactor/peer_static_routes.go`:**
```go
func toPluginParams(r PluginRoute, fam family.Family) message.PluginParams
func toStaticRouteUnicastParams(r StaticRoute, nf bool) message.UnicastParams
func toVPNParams(r VPNRoute) message.VPNParams
```
<!-- source: internal/component/bgp/reactor/peer_static_routes.go -- toPluginParams, toStaticRouteUnicastParams -->

The exotic MP_REACH families (MUP/VPLS/MVPN/FlowSpec/SR-Policy) share `PluginRoute`
/ `PluginParams`; their NLRI and family-specific attributes are pre-built by the
family plugin, so the message package carries no per-family domain object or builder
for them. (`message.FlowSpecParams` / `BuildFlowSpec` survive only for the ze-chaos
load generator, not the config/API route path.)

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
slice. Per `ai/rules/architecture.md` ("No `make` where pools exist"), such
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

`BuildGroupedUnicast` (and `Splitter.Split`) emit multiple Updates per outer call,
all sharing a subset of scratch. To keep this safe without re-building shared
attributes per chunk:

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

The leaf governs every rail that sends several NLRIs to one peer, not the
adj-rib-out alone. `nlriUnitLen` turns it into framing, and the batch API rails
(`AnnounceNLRIBatch`, `WithdrawNLRIBatch`) and the LLGR readvertise rail each
read it there: with `group-updates false` a batch of N prefixes leaves as N
UPDATE messages carrying one prefix each.
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- nlriUnitLen, announceBatchToPeers, withdrawBatchFromPeers -->
<!-- source: internal/component/bgp/reactor/peer_initial_sync.go -- the config-driven sync reads the same leaf -->

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

When update groups are enabled, the batch API groups peers by build-equivalent parameters (encoding context, next-hop resolution, AS_PATH form, and the peer's `group-updates` leaf) and builds the UPDATE once per parameter set. All peers sharing those parameters receive the same pre-built wire bytes.

The parameter set is `announceFacts`, and both rails use it. It is the argument set of the build AND the map key that forms the group, so a per-peer fact the build reads is a fact the key holds. A peer carrying `group-updates false` therefore cannot share a build with a peer that packs: the two receive a different number of messages from one batch.

The withdraw rail had a `withdrawFacts` of its own, carrying three fields, while a withdrawal was a bare MP_UNREACH_NLRI and nothing but the framing could tell two peers apart. A withdrawal now carries attributes (below), so every per-peer decision that shapes an announce's attribute block shapes a withdrawal's too. `withdrawFactsFor` leaves the attribute-shaping fields zero for a unicast withdrawal, which carries no attributes whatever the peer answers, so those peers still share one build.

When disabled or when each peer has a unique context, the code falls back to per-peer building with no behavior change.
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- groupsEnabled check, announceFacts, withdrawFactsFor -->

### The Adj-RIB-Out on the API Rails

RFC 4271 Section 9.2: "A BGP speaker SHOULD NOT advertise a given feasible BGP route from its Adj-RIB-Out if it would produce an UPDATE message containing the same BGP route as was previously advertised."

Each peer keeps a table of what the API rails have sent it. A second announce of a route it already holds, with the same bytes, puts nothing on the wire. Until 2026-09-06 there was no such table, so `announce route X` twice sent two identical UPDATEs, and an operator script that re-announces its set on a timer re-flooded every peer on every tick.

| Question | Answer |
|----------|--------|
| What is the key | The NLRI exactly as written to the wire, so RFC 7911 ADD-PATH keys on the path identifier too |
| What is compared | The attribute block the builder emitted for THIS peer: after next-hop resolution, the AS_PATH prepend, the LOCAL_PREF decision and every other `announceFacts` edit. The MP_REACH_NLRI payload and its length octets are cut, so one route's signature does not change with the size of the batch it travelled in |
| What empties it | A withdrawal removes its route. A session teardown drops the whole table, because the peer reached over the next connection holds nothing (RFC 4271 Section 6.3) |
| What is never suppressed | A withdrawal, and a batch carrying `NLRIBatch.Replay` |
| What an operator sees | A debug line on `subsystem=bgp.routes` naming the peer, the family and the count, and a per-peer counter beside it |

`Replay` is what keeps `clear bgp rib out` and the RFC 2918 route refresh behind it reaching the wire. That rail resends routes the peer already holds, over a session that is still up, so without the marker it would answer a request to re-send with silence. The RIB plugin's `resendRoutesWithCursor` sets it; the peer-up replay does not need it, because the teardown already emptied the table.

Two origination paths do NOT record: the config-driven initial sync (`peer_initial_sync.go`) and the `SendRoutes` transaction rail. Neither can cause a wrong suppression, because a route that was never recorded is always sent; a route one of them sent and the API rail then announces is sent twice, exactly as before.
<!-- source: internal/component/bgp/reactor/adj_rib_out.go -- adjRIBOut, announceSignature, announceUnit -->
<!-- source: internal/component/bgp/plugins/rib/rib_replay.go -- resendRoutesWithCursor -->

### A Withdrawal Names a Route This Connection Advertised

RFC 4271 Section 4.3 identifies a withdrawn route by its destination, "which unambiguously identifies the route in the context of the BGP speaker - BGP speaker connection to which it has been previously advertised."

A connection that has advertised nothing has no route for any withdrawal to name, so the API rail names no route to such a peer. The condition is about the CONNECTION, not about the route: once the connection has carried one UPDATE that makes any destination reachable, every later withdrawal is written, whether or not the peer holds the route named. A key-based rule would be a different rule and a wrong one, because an operator withdrawing a route the peer never received is telling a peer it must not hold it, and RFC 4271 Section 9.1.2 has the receiver ignore a withdrawal for a route it does not have.

What the peer receives is the withdrawal with its routes removed: the path attributes alone, with no MP_UNREACH_NLRI, no Withdrawn Routes and no NLRI. RFC 4271 Section 6.3: "An UPDATE message that contains correct path attributes, but no NLRI, SHALL be treated as a valid UPDATE message." No RFC asks a speaker to SEND it. It is the ExaBGP compatibility contract, and upstream reaches it the same way: `include_withdraw` drops each withdrawn NLRI while it is False and `UpdateCollection.messages` yields the packed attributes regardless. `api-flow` is the recording, and its second frame is that message.

A family whose withdrawal carries no attributes of its own is written NOTHING, because removing the routes leaves an empty UPDATE rather than an attributes-only one. IPv4 unicast and the multiprotocol unicast families are that case, and upstream sends nothing for them too, which `api-fast` records.

The message names no route, so it does not arm the connection either: a second withdrawal is withheld exactly as the first was.

| Question | Answer |
|----------|--------|
| Where the state lives | `Session.advertised`, so a new connection starts unset and there is nothing to clear at teardown |
| What arms it | Any UPDATE carrying NLRI or MP_REACH_NLRI, written at any of the three points a message reaches the socket. A route the RIB forwarded arms it exactly as an API announce does |
| What does not arm it | An End-of-RIB (RFC 4724 Section 2) and a pure withdrawal: neither makes a destination reachable |
| What an operator sees | A warning on `subsystem=bgp.routes` naming the peer, the family, the count and the RFC section; a per-peer counter beside the suppressed count; and `route.ErrWithdrawWithheld` on the command's answer, naming every peer it was withheld from |

The answer is a WARNING and never a failure. The command did what it asked for, so `send bgp <selector> update text ... nlri <family> del <nlri>` still answers `done` with the reason in its warnings. A zero UPDATE count with a bare `done` and no reason is the silent no-op this rail exists to avoid.

ExaBGP has the same asymmetry, reached another way: `include_withdraw` starts False for each session and becomes True only when the first update pass exhausts, and its packing layer drops every withdrawn NLRI while it is False. `api-fast` is the recording: the withdrawal its first burst writes never reaches the wire, and the one its second burst writes does.
<!-- source: internal/component/bgp/reactor/session_write.go -- Session.advertised, noteAdvertised, updateIsReachable -->
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- withdrawBatchFromPeers, buildWithheldWithdrawUpdate, logWithdrawWithheld -->

### What a Withdrawal Carries

RFC 4760 Section 4: "An UPDATE message that contains the MP_UNREACH_NLRI is not required to carry any other path attributes." Both shapes are conformant, so the family decides which one Ze sends.

| Family | Withdrawal shape |
|--------|------------------|
| IPv4 unicast | Withdrawn Routes field (RFC 4271 Section 4.3), no path attributes |
| IPv6 unicast | Bare MP_UNREACH_NLRI, no other path attributes |
| Every other family | MP_UNREACH_NLRI plus the block `planBatchAttrs` plans for an announce: the caller's own attributes, the well-known mandatory ORIGIN and AS_PATH, the RFC 4271 Section 5.1.5 LOCAL_PREF toward an internal peer, and the legacy NEXT_HOP where the family carries one |

Unicast is bare because the Withdrawn Routes field carries no attributes and the IPv6 unicast withdrawal is the same withdrawal in the RFC 4760 encoding: the AFI must not decide what `withdraw <prefix> next-hop X local-preference 200` means. ExaBGP splits them at the same seam.

Until 2026-09-06 every withdrawal took the bare shape, so `send bgp <selector> update text <attributes> nlri <family> del <nlri>` was acknowledged and the attributes never reached the wire.
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- buildBatchWithdrawUpdate, planBatchAttrs -->

### The Legacy NEXT_HOP Beside MP_REACH_NLRI

RFC 4760 Section 3: "An UPDATE message that carries no NLRI, other than the one encoded in the MP_REACH_NLRI attribute, SHOULD NOT carry the NEXT_HOP attribute." It is a SHOULD NOT, so carrying it is conformant, and `family.Family.LegacyNextHop` is the single declaration of which families Ze carries it for: unicast, multicast, labeled unicast, MCAST-VPN, MUP and MPLS-VPN. FlowSpec, VPLS, EVPN, SR Policy, RTC and BGP-LS carry none.

The address must also be an IPv4 address that RFC 4271 Section 6.3 calls syntactically correct, so an IPv6 next hop and `0.0.0.0` each contribute nothing (`legacyNextHopApplies`).

The config rail already sent it -- `message.(*UpdateBuilder).BuildVPN`, and the `nlri/mvpn` and `nlri/mup` config parsers -- while the API rail sent MP_REACH_NLRI alone, so one route reached the wire as two different byte strings depending on whether an operator configured it or announced it.
<!-- source: internal/core/family/family.go -- Family.LegacyNextHop -->
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- legacyNextHopApplies -->

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
│  Config → PluginRoute → PluginParams → BuildPlugin()           │
│                ↓              ↓               ↓                 │
│        plugin-built NLRI  (pass-through)  Update{[]byte}        │
│                                                                 │
│  Volume: Low (tens of rules)                                    │
│  Optimization: Plugin pre-builds NLRI + attrs at config time    │
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
> The send path uses `Splitter.Split` for oversized UPDATEs. Same-context
> forwarding uses `wireu.SplitWireUpdate`. Cross-context forwarding re-encodes
> the UPDATE and uses `Splitter.SplitCompliant` to separate mixed NLRI-bearing
> fields. See `plan/learned/DESIGN-HISTORY.md`, "BGP engine: wire encoding and
> RIB" (retired summary 078).

**Files involved:**
- `internal/component/bgp/message/update_split.go` - `Splitter.Split()` and `Splitter.SplitCompliant()`
- `internal/component/bgp/wireu/split.go` - `SplitWireUpdate()` for same-context forwarding
- `internal/component/bgp/message/chunk_mp_nlri.go` - `ChunkMPNLRI()` for family-aware NLRI parsing
- `internal/component/bgp/reactor/peer_send.go` - `sendUpdateWithSplit()` integration through `Splitter.Split`
- `internal/component/bgp/reactor/forward_body.go` - `buildFwdBody()` selects the forwarding split path and `fwdSplitParsedUpdate()` applies `Splitter.SplitCompliant`
<!-- source: internal/component/bgp/message/update_split.go -- Splitter.Split, Splitter.SplitCompliant -->
<!-- source: internal/component/bgp/wireu/split.go -- SplitWireUpdate -->
<!-- source: internal/component/bgp/reactor/peer_send.go -- sendUpdateWithSplit -->
<!-- source: internal/component/bgp/reactor/forward_body.go -- buildFwdBody, fwdSplitParsedUpdate -->
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

Summaries 057, 059, 078 and 343 were retired on 2026-08-01. Their surviving
knowledge is in `plan/learned/DESIGN-HISTORY.md`, "BGP engine: wire encoding
and RIB".

- DESIGN-HISTORY > Load-bearing invariants - lazy-parsed wire attribute
  storage on the forward path: `AttributesWire` does not own its `packed
  []byte` (057)
- DESIGN-HISTORY > Abandoned approaches - pool handle integration was designed
  and abandoned, not shipped (059)
- DESIGN-HISTORY > Evolution and Load-bearing invariants - buffer pool
  get/return lifecycle, keyed on `cap(buf)` (343)
- DESIGN-HISTORY > Evolution and Load-bearing invariants - wire-level UPDATE
  splitting, its "accept invalid, emit valid" posture and its progress guard
  (078)

---

**Created:** 2026-01-01
**Last Updated: 2026-01-30
