# Spec: Unified Negotiated Packing Pattern

## Task
Refactor wire format encoding to use consistent `Pack(negotiated)` pattern across NLRI, attributes, and messages.

## Current State (Research Completed 2025-12-28)

### Two Negotiated Structs Exist

| Location | Purpose | Key Fields |
|----------|---------|------------|
| `pkg/bgp/message/message.go:14` | Wire encoding | ASN4, AddPath map[Family]bool, ExtendedMessage |
| `pkg/bgp/capability/negotiated.go:43` | Session negotiation | Full state + families, addPath map, extNH |

### PackContext Already Exists

```go
// pkg/bgp/nlri/pack.go
type PackContext struct {
    AddPath bool  // Only field currently
}
```

### NLRI Pack() Already Implemented

All NLRI types have `Pack(ctx *PackContext)`:
- ✅ INET (`inet.go:228`)
- ✅ IPVPN (`ipvpn.go:394`)
- ✅ EVPN all types (`evpn.go:928-933`)
- ✅ FlowSpec/FlowSpecVPN (`flowspec.go:945-946`)
- ✅ BGPLS all types (`bgpls.go:818-821`)
- ✅ MVPN, VPLS, RTC, MUP (`other.go:875-878`)

### Call Sites Analysis

**✅ Already uses Pack(ctx):**

| File:Line | Context |
|-----------|---------|
| `rib/commit.go:141` | `route.NLRI().Pack(ctx)` in grouped updates |
| `rib/commit.go:168` | `route.NLRI().Pack(ctx)` in single updates |
| `reactor/peer.go:1354` | IPv4 unicast static routes |
| `reactor/peer.go:1599` | Grouped static routes |

**❌ Still uses Bytes() - NEEDS MIGRATION:**

| File:Line | Context | Priority |
|-----------|---------|----------|
| `reactor/peer.go:1057` | `buildRouteNLRIUpdate` - RIB routes | HIGH |
| `reactor/peer.go:1065` | `buildRouteNLRIUpdate` - MP_REACH | HIGH |
| `reactor/peer.go:1105` | `buildWithdrawNLRI` - IPv4 withdraw | HIGH |
| `reactor/peer.go:1113` | `buildWithdrawNLRI` - MP_UNREACH | HIGH |
| `reactor/peer.go:1132` | `buildStaticRouteWithdraw` - IPv4 | HIGH |
| `reactor/peer.go:1140` | `buildStaticRouteWithdraw` - IPv6 | HIGH |
| `reactor/reactor.go:404` | `buildAnnounceUpdate` - API IPv4 | HIGH |
| `reactor/reactor.go:413` | `buildAnnounceUpdate` - API MP_REACH | HIGH |
| `reactor/reactor.go:431` | `buildWithdrawUpdate` - API IPv4 | HIGH |
| `reactor/reactor.go:440` | `buildWithdrawUpdate` - API IPv6 | HIGH |
| `reactor/reactor.go:1161` | `sendWithdrawals` - IPv4 loop | HIGH |
| `reactor/reactor.go:1170` | `sendWithdrawals` - MP_UNREACH loop | HIGH |
| `rib/update.go:70,76` | `buildNLRIBytes` - grouped updates | MEDIUM |

**OK to keep Bytes() - Internal use:**

| File:Line | Context | Reason |
|-----------|---------|--------|
| `rib/outgoing.go:145` | `buildNLRIIndex` | Indexing, not wire |
| `rib/store.go:80` | `hashableNLRI.Bytes()` | Dedup hash |
| `api/commit_manager.go:86` | `nlriIndex` | Indexing |

## Problem Analysis

Current code has inconsistent patterns for capability-dependent encoding:

```go
// Pattern 1: Ignores negotiation
nlri.Bytes()  // No ADD-PATH awareness

// Pattern 2: Explicit parameter
attribute.PackASPathAttribute(asPath, asn4 bool)  // ASN4 as param

// Pattern 3: Negotiated struct
message.Pack(neg *Negotiated)  // Full negotiated state
```

This causes:
1. ADD-PATH bug in RIB route sending (test R passes for static, fails for API)
2. Potential ASN4 issues if we receive 4-byte and send to 2-byte peer
3. Inconsistent API making code harder to maintain

## Proposed Pattern

### Current PackContext (Keep As-Is)

```go
// pkg/bgp/nlri/pack.go - Already exists, minimal and focused
type PackContext struct {
    AddPath bool  // RFC 7911
    // Future: ASN4 bool, ExtendedNextHop AFI
}
```

### NLRI Interface (Already Done)

```go
type NLRI interface {
    Bytes() []byte           // For internal use, RIB keys, etc.
    Pack(ctx *PackContext) []byte  // For wire encoding
}
```

### Future: Attribute Changes

```go
// Current
func (p *ASPath) Pack() []byte           // Always 4-byte
func (p *ASPath) PackWithASN4(bool) []byte  // Explicit param

// Future - Unified pattern
func (a *ASPath) Pack(ctx *PackContext) []byte {
    if ctx == nil || ctx.ASN4 {
        return a.pack4Byte()
    }
    return a.pack2ByteWithAS4Path()
}
```

## Capabilities Affected

### 1. ADD-PATH (RFC 7911) - NLRI

| Scenario | Action |
|----------|--------|
| Send=true, has path ID | Include 4-byte path ID |
| Send=true, no path ID | Prepend NOPATH (4 zeros) |
| Send=false | Omit path ID |

### 2. ASN4 (RFC 6793) - AS_PATH

| Scenario | Action |
|----------|--------|
| ASN4=true | 4-byte AS numbers |
| ASN4=false, all ASNs ≤65535 | 2-byte AS numbers |
| ASN4=false, any ASN >65535 | AS_PATH with AS_TRANS (23456), plus AS4_PATH |

**Note:** AS4_PATH (type 17) and AS4_AGGREGATOR (type 18) are transitive optional attributes that carry the real 4-byte AS path when communicating with 2-byte peers.

### 3. Extended Message (RFC 8654) - Message Header

| Scenario | Action |
|----------|--------|
| ExtendedMessage=true | Allow messages up to 65535 bytes |
| ExtendedMessage=false | Max 4096 bytes, split if needed |

### 4. Extended Next Hop (RFC 8950) - MP_REACH_NLRI

| Scenario | Action |
|----------|--------|
| ExtNH negotiated for family | Can use IPv6 next-hop for IPv4 NLRI |
| Not negotiated | Must use matching AFI for next-hop |

## Implementation Steps

### Phase 1: Migrate Remaining Bytes() Calls (Current Priority)

**Already done:**
- ✅ NLRI interface has `Pack(ctx *PackContext)`
- ✅ All NLRI types implement Pack
- ✅ CommitService uses Pack

**Remaining work - reactor/peer.go:**

```go
// 1. buildRouteNLRIUpdate (line ~1040) - needs PackContext from peer
func buildRouteNLRIUpdate(route *rib.Route, peer *Peer) *message.Update {
    ctx := peer.packContext(route.NLRI().Family())  // NEW
    // Change: nlriBytes = routeNLRI.Bytes()
    // To:     nlriBytes = routeNLRI.Pack(ctx)
}

// 2. buildWithdrawNLRI (line ~1097) - needs PackContext
func buildWithdrawNLRI(n nlri.NLRI, ctx *nlri.PackContext) *message.Update {
    // Change: n.Bytes()
    // To:     n.Pack(ctx)
}

// 3. buildStaticRouteWithdraw (line ~1121) - needs PackContext
func buildStaticRouteWithdraw(route StaticRoute, ctx *nlri.PackContext) *message.Update {
    // Change: inet.Bytes()
    // To:     inet.Pack(ctx)
}
```

**Remaining work - reactor/reactor.go:**

```go
// 4. buildAnnounceUpdate (line ~390) - API routes need peer's PackContext
// Currently creates INET then calls .Bytes()
// Need to pass negotiated state from calling peer

// 5. buildWithdrawUpdate (line ~426) - same issue

// 6. sendWithdrawals (line ~1150) - loop calling .Bytes()
// Need PackContext per family from peer's negotiated state
```

**Remaining work - rib/update.go:**

```go
// 7. BuildGroupedUpdate / buildNLRIBytes (line ~62)
// Add ctx parameter: func BuildGroupedUpdate(group *RouteGroup, ctx *nlri.PackContext)
// Change: route.NLRI().Bytes()
// To:     route.NLRI().Pack(ctx)
```

### Phase 2: AS_PATH (Future)

1. Add `ASN4 bool` to PackContext
2. Change `ASPath.Pack()` to `ASPath.Pack(ctx *PackContext)`
3. Handle AS4_PATH generation for 2-byte peers
4. Update attribute packing to use `Pack(ctx)`

### Phase 3: Other Attributes (Future)

1. Extended Communities with ASN fields
2. Aggregator attribute (2 vs 4 byte AS)
3. Consider unifying the two Negotiated structs

## Codebase Context

**Key files:**
- `pkg/bgp/nlri/pack.go` - PackContext struct
- `pkg/bgp/nlri/nlri.go` - NLRI interface with Pack
- `pkg/bgp/nlri/inet.go` - INET.Pack implementation
- `pkg/bgp/message/message.go` - message.Negotiated struct
- `pkg/bgp/capability/negotiated.go` - capability.Negotiated struct
- `pkg/rib/commit.go` - Uses Pack correctly ✅
- `pkg/reactor/peer.go` - Needs migration
- `pkg/reactor/reactor.go` - Needs migration

**Helper needed on Peer:**
```go
// Add to Peer struct or as method
func (p *Peer) packContext(family nlri.Family) *nlri.PackContext {
    if p.negotiated == nil {
        return nil
    }
    msgFamily := message.Family{AFI: uint16(family.AFI), SAFI: uint8(family.SAFI)}
    return &nlri.PackContext{
        AddPath: p.negotiated.AddPath[msgFamily],
    }
}
```

## Verification Checklist

### Phase 1 (This PR)
- [x] NLRI interface has Pack method
- [x] INET.Pack handles ADD-PATH correctly
- [x] Other NLRI types have Pack
- [x] CommitService uses Pack
- [ ] reactor/peer.go migrated (buildRouteNLRIUpdate, buildWithdrawNLRI, buildStaticRouteWithdraw)
- [ ] reactor/reactor.go migrated (buildAnnounceUpdate, buildWithdrawUpdate, sendWithdrawals)
- [ ] rib/update.go migrated (BuildGroupedUpdate)
- [ ] Test R passes for all code paths
- [ ] `make test && make lint` passes

### Phase 2 (Future PR)
- [ ] PackContext has ASN4 field
- [ ] ASPath.Pack(ctx) handles ASN4
- [ ] AS4_PATH generated when needed
- [ ] Tests for 4-to-2 byte AS path conversion

## Migration Strategy

1. ✅ **Pack method exists** alongside Bytes() - DONE
2. **Update remaining callers** to use Pack where negotiation matters
3. **Keep Bytes()** for internal use (RIB keys, dedup hashing)
4. **No breaking changes** - existing code continues to work

## Concrete Migration Steps

### Step 1: Add packContext helper to Peer

```go
// pkg/reactor/peer.go - add method
func (p *Peer) packContext(family nlri.Family) *nlri.PackContext {
    if p.negotiatedFamilies == nil {
        return nil
    }
    // Use existing NegotiatedFamilies which has IPv4UnicastAddPath, IPv6UnicastAddPath
    switch {
    case family.AFI == nlri.AFIIPv4 && family.SAFI == nlri.SAFIUnicast:
        return &nlri.PackContext{AddPath: p.negotiatedFamilies.IPv4UnicastAddPath}
    case family.AFI == nlri.AFIIPv6 && family.SAFI == nlri.SAFIUnicast:
        return &nlri.PackContext{AddPath: p.negotiatedFamilies.IPv6UnicastAddPath}
    default:
        return nil  // Other families don't have ADD-PATH yet
    }
}
```

### Step 2: Update buildRouteNLRIUpdate

Change signature to accept peer or PackContext, use Pack instead of Bytes.

### Step 3: Update buildWithdrawNLRI

Add ctx parameter, use Pack.

### Step 4: Update reactor.go API functions

These need access to peer's negotiated state - may need refactoring.

### Step 5: Update rib/update.go

Add ctx parameter to BuildGroupedUpdate.

## Example Usage

```go
// Before
nlriBytes := route.NLRI().Bytes()

// After
ctx := peer.packContext(route.NLRI().Family())
nlriBytes := route.NLRI().Pack(ctx)
```

## Resolved Questions

1. **Pass full Negotiated or just PackContext?**
   - **Decision:** Use PackContext (minimal, focused)
   - PackContext has only what's needed for NLRI encoding
   - Future: add ASN4 bool when needed for AS_PATH

2. **Should Pack return error?**
   - **Decision:** No error - matches Bytes() pattern
   - Caller ensures valid negotiation state

3. **Rename Bytes()?**
   - **Decision:** Keep Bytes() - clear meaning, used for indexing/hashing
