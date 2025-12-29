# Spec: Full Context Integration

## Overview

Complete integration of EncodingContext throughout the codebase:
1. **Peer Integration** - recv/send contexts per peer
2. **Route Storage** - SourceCtxID for wire cache optimization
3. **Zero-Copy Forwarding** - skip re-encoding when contexts match
4. **PackWithContext** - context-aware attribute encoding

## Current State (verified)

```
🔍 Functional tests: 24 passed, 13 failed
📋 Last commit: 3a8ef7b
```

## Prerequisites

**MUST complete first:** `spec-encoding-context-impl.md`
- `pkg/bgp/context/` package with EncodingContext, ContextID, ContextRegistry

---

## Phase 1: Peer Integration

### Goal

Each Peer holds recv and send contexts, created at session establishment.

### Current State

```go
// pkg/reactor/peer.go (current)
type Peer struct {
    families atomic.Pointer[NegotiatedFamilies]  // Pre-computed flags
    // ... no context fields
}
```

### Target State

```go
// pkg/reactor/peer.go (new)
type Peer struct {
    families atomic.Pointer[NegotiatedFamilies]  // Keep for backward compat

    // Encoding contexts (created at session establishment)
    recvCtx   *bgpctx.EncodingContext  // For parsing routes FROM peer
    recvCtxID bgpctx.ContextID
    sendCtx   *bgpctx.EncodingContext  // For encoding routes TO peer
    sendCtxID bgpctx.ContextID
}
```

### Implementation Steps

**Step 1.1: Add context fields to Peer**

```go
import bgpctx "github.com/exa-networks/zebgp/pkg/bgp/context"

type Peer struct {
    // ... existing fields ...

    // Encoding contexts for this peer session.
    // Created at session establishment, cleared on teardown.
    recvCtx   *bgpctx.EncodingContext
    recvCtxID bgpctx.ContextID
    sendCtx   *bgpctx.EncodingContext
    sendCtxID bgpctx.ContextID
}
```

**Step 1.2: Create contexts at session establishment**

In `handleEstablished()` or equivalent:

```go
func (p *Peer) onSessionEstablished(neg *capability.Negotiated) {
    // Existing: compute NegotiatedFamilies
    nf := computeNegotiatedFamilies(neg)
    p.families.Store(nf)

    // NEW: Create encoding contexts
    p.recvCtx = bgpctx.FromNegotiatedRecv(neg, p.settings.LocalAS)
    p.recvCtxID = bgpctx.Registry.Register(p.recvCtx)

    p.sendCtx = bgpctx.FromNegotiatedSend(neg, p.settings.LocalAS)
    p.sendCtxID = bgpctx.Registry.Register(p.sendCtx)
}
```

**Step 1.3: Add FromNegotiatedRecv/Send to context package**

```go
// pkg/bgp/context/negotiated.go

// FromNegotiatedRecv creates receive context (what peer sends us).
// Used when storing routes received from this peer.
func FromNegotiatedRecv(neg *capability.Negotiated, localAS uint32) *EncodingContext {
    ctx := &EncodingContext{
        ASN4:            neg.ASN4,
        AddPath:         make(map[Family]bool),
        ExtendedNextHop: make(map[Family]bool),
        IsIBGP:          neg.LocalASN == neg.PeerASN,
        LocalAS:         localAS,
        PeerAS:          neg.PeerASN,
    }

    // ADD-PATH receive: can peer send us path IDs?
    for _, f := range neg.Families() {
        mode := neg.AddPathMode(capability.Family(f))
        // We receive if peer can send (Send or Both)
        canRecv := mode == capability.AddPathSend || mode == capability.AddPathBoth
        if canRecv {
            ctx.AddPath[f] = true
        }
    }

    // Extended next-hop
    for _, f := range neg.Families() {
        if neg.ExtendedNextHopAFI(capability.Family(f)) != 0 {
            ctx.ExtendedNextHop[f] = true
        }
    }

    return ctx
}

// FromNegotiatedSend creates send context (what we send to peer).
// Used when encoding routes to send to this peer.
func FromNegotiatedSend(neg *capability.Negotiated, localAS uint32) *EncodingContext {
    ctx := &EncodingContext{
        ASN4:            neg.ASN4,
        AddPath:         make(map[Family]bool),
        ExtendedNextHop: make(map[Family]bool),
        IsIBGP:          neg.LocalASN == neg.PeerASN,
        LocalAS:         localAS,
        PeerAS:          neg.PeerASN,
    }

    // ADD-PATH send: can we send path IDs to peer?
    for _, f := range neg.Families() {
        mode := neg.AddPathMode(capability.Family(f))
        // We send if we can send AND peer can receive
        canSend := mode == capability.AddPathReceive || mode == capability.AddPathBoth
        if canSend {
            ctx.AddPath[f] = true
        }
    }

    // Extended next-hop (symmetric)
    for _, f := range neg.Families() {
        if neg.ExtendedNextHopAFI(capability.Family(f)) != 0 {
            ctx.ExtendedNextHop[f] = true
        }
    }

    return ctx
}
```

**Step 1.4: Clear contexts on teardown**

```go
func (p *Peer) onSessionTeardown() {
    p.families.Store(nil)
    p.recvCtx = nil
    p.recvCtxID = 0
    p.sendCtx = nil
    p.sendCtxID = 0
}
```

### Tests

```go
// TestPeerContextsCreatedOnEstablish verifies contexts are set.
//
// VALIDATES: recvCtx/sendCtx populated from Negotiated.
//
// PREVENTS: Nil context panic when encoding/decoding.
func TestPeerContextsCreatedOnEstablish(t *testing.T)

// TestPeerContextsAsymmetricAddPath verifies recv != send for ADD-PATH.
//
// VALIDATES: Asymmetric ADD-PATH creates different contexts.
//
// PREVENTS: Wrong path ID inclusion/exclusion.
func TestPeerContextsAsymmetricAddPath(t *testing.T)

// TestPeerContextsClearedOnTeardown verifies cleanup.
//
// VALIDATES: Contexts nil after teardown.
//
// PREVENTS: Stale context use after session end.
func TestPeerContextsClearedOnTeardown(t *testing.T)
```

---

## Phase 2: Route Storage with SourceCtxID

### Goal

Routes store `SourceCtxID` to enable zero-copy forwarding.

### Current State

```go
// pkg/rib/route.go (current)
type Route struct {
    nlri       nlri.NLRI
    nextHop    netip.Addr
    attributes []attribute.Attribute
    asPath     *attribute.ASPath
    refCount   atomic.Int32
    indexCache []byte
}
```

### Target State

```go
// pkg/rib/route.go (new)
type Route struct {
    nlri       nlri.NLRI
    nextHop    netip.Addr
    attributes []attribute.Attribute
    asPath     *attribute.ASPath
    refCount   atomic.Int32
    indexCache []byte

    // Wire cache for zero-copy forwarding
    wireBytes   []byte             // Original packed attributes
    sourceCtxID bgpctx.ContextID   // Context used to encode wireBytes
}
```

### Implementation Steps

**Step 2.1: Add wire cache fields to Route**

```go
import bgpctx "github.com/exa-networks/zebgp/pkg/bgp/context"

type Route struct {
    // ... existing fields ...

    // Wire cache: enables zero-copy forwarding when contexts match.
    // wireBytes contains the original packed path attributes.
    // sourceCtxID identifies the encoding context (for compatibility check).
    wireBytes   []byte
    sourceCtxID bgpctx.ContextID
}
```

**Step 2.2: Add constructor with wire cache**

```go
// NewRouteWithWireCache creates a route with cached wire bytes.
// Used when receiving routes - store original bytes for potential forwarding.
func NewRouteWithWireCache(
    n nlri.NLRI,
    nextHop netip.Addr,
    attrs []attribute.Attribute,
    asPath *attribute.ASPath,
    wireBytes []byte,
    sourceCtxID bgpctx.ContextID,
) *Route {
    r := &Route{
        nlri:        n,
        nextHop:     nextHop,
        attributes:  attrs,
        asPath:      asPath,
        wireBytes:   wireBytes,
        sourceCtxID: sourceCtxID,
    }
    r.refCount.Store(1)
    return r
}
```

**Step 2.3: Add compatibility check method**

```go
// CanForwardDirect returns true if wireBytes can be used directly.
// This is the fast path for route reflection.
func (r *Route) CanForwardDirect(destCtxID bgpctx.ContextID) bool {
    return len(r.wireBytes) > 0 && r.sourceCtxID == destCtxID
}

// WireBytes returns the cached wire bytes (may be nil).
func (r *Route) WireBytes() []byte {
    return r.wireBytes
}

// SourceCtxID returns the source context ID.
func (r *Route) SourceCtxID() bgpctx.ContextID {
    return r.sourceCtxID
}
```

### Tests

```go
// TestRouteWireCacheStored verifies wire bytes are stored.
//
// VALIDATES: wireBytes accessible after construction.
//
// PREVENTS: Lost optimization opportunity.
func TestRouteWireCacheStored(t *testing.T)

// TestRouteCanForwardDirect_Match verifies true when IDs match.
//
// VALIDATES: Returns true when contexts match.
//
// PREVENTS: Unnecessary re-encoding.
func TestRouteCanForwardDirect_Match(t *testing.T)

// TestRouteCanForwardDirect_Mismatch verifies false when IDs differ.
//
// VALIDATES: Returns false when contexts differ.
//
// PREVENTS: Sending wrongly encoded data.
func TestRouteCanForwardDirect_Mismatch(t *testing.T)

// TestRouteCanForwardDirect_NoCache verifies false when no cache.
//
// VALIDATES: Returns false when wireBytes is nil.
//
// PREVENTS: Nil dereference.
func TestRouteCanForwardDirect_NoCache(t *testing.T)
```

---

## Phase 3: Zero-Copy Forwarding

### Goal

When forwarding routes, use cached wire bytes if contexts match.

### Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    Route Forwarding                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  route.CanForwardDirect(peer.sendCtxID)?                   │
│           │                                                 │
│     ┌─────┴─────┐                                          │
│     │           │                                          │
│    YES          NO                                         │
│     │           │                                          │
│     ▼           ▼                                          │
│  Zero-copy   Re-encode                                     │
│  forward     with peer's                                   │
│  wireBytes   sendCtx                                       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Implementation

**Step 3.1: Add forwarding method to Route**

```go
// pkg/rib/route.go

// PackAttributesFor returns packed attributes for the destination context.
// Uses cached wire bytes if contexts match (zero-copy), otherwise re-encodes.
func (r *Route) PackAttributesFor(destCtx *bgpctx.EncodingContext, destCtxID bgpctx.ContextID) []byte {
    // Fast path: use cached bytes if compatible
    if r.CanForwardDirect(destCtxID) {
        return r.wireBytes
    }

    // Slow path: re-encode with destination context
    return packAttributesWithContext(r.attributes, r.asPath, destCtx)
}
```

**Step 3.2: Implement packAttributesWithContext**

```go
// pkg/bgp/attribute/pack.go (new file or extend existing)

// PackAttributesWithContext packs attributes using the given context.
// Handles ASN4-dependent encoding for AS_PATH and AGGREGATOR.
func PackAttributesWithContext(
    attrs []attribute.Attribute,
    asPath *ASPath,
    ctx *bgpctx.EncodingContext,
) []byte {
    var result []byte

    // Pack each attribute with context
    for _, attr := range attrs {
        packed := attr.PackWithContext(ctx)
        result = append(result, PackHeader(attr.Flags(), attr.Code(), uint16(len(packed)))...)
        result = append(result, packed...)
    }

    // Pack AS_PATH separately (it's stored outside attrs in Route)
    if asPath != nil {
        packed := asPath.PackWithContext(ctx)
        result = append(result, PackHeader(asPath.Flags(), asPath.Code(), uint16(len(packed)))...)
        result = append(result, packed...)
    }

    return result
}
```

**Step 3.3: Update peer forwarding logic**

```go
// pkg/reactor/peer.go

func (p *Peer) forwardRoute(route *rib.Route) error {
    var attrBytes []byte

    if route.CanForwardDirect(p.sendCtxID) {
        // Zero-copy: use cached wire bytes
        attrBytes = route.WireBytes()
    } else {
        // Re-encode with our send context
        attrBytes = route.PackAttributesFor(p.sendCtx, p.sendCtxID)
    }

    // Build and send UPDATE
    update := &message.Update{
        PathAttributes: attrBytes,
        NLRI:           route.NLRI().Pack(p.sendCtx.ToPackContext(route.NLRI().Family())),
    }

    return p.sendUpdate(update)
}
```

### Tests

```go
// TestForwardRouteZeroCopy verifies zero-copy path.
//
// VALIDATES: WireBytes returned when contexts match.
//
// PREVENTS: CPU waste on unnecessary re-encoding.
func TestForwardRouteZeroCopy(t *testing.T)

// TestForwardRouteReencode verifies re-encoding path.
//
// VALIDATES: PackAttributesWithContext called when contexts differ.
//
// PREVENTS: Protocol errors from mismatched encoding.
func TestForwardRouteReencode(t *testing.T)
```

---

## Phase 4: PackWithContext on Attribute Interface

### Goal

Add `PackWithContext(srcCtx, dstCtx *EncodingContext) []byte` to Attribute interface.

**Why BOTH contexts:**
1. **Transcoding (RFC 6793):** AS_PATH merge/split depends on src.ASN4 → dst.ASN4 transition
2. **Wire cache:** If srcCtx == dstCtx, can skip re-encoding (zero-copy optimization)
3. **IBGP/EBGP handling:** LOCAL_PREF add/remove depends on src.IsIBGP → dst.IsIBGP
4. **Future-proof:** Any asymmetric encoding RFC has the info it needs

### Current State

```go
// pkg/bgp/attribute/attribute.go (current)
type Attribute interface {
    Code() AttributeCode
    Flags() AttributeFlags
    Len() int
    Pack() []byte
}
```

### Target State

```go
// pkg/bgp/attribute/attribute.go (new)
type Attribute interface {
    Code() AttributeCode
    Flags() AttributeFlags
    Len() int
    Pack() []byte  // Deprecated: use PackWithContext

    // PackWithContext serializes attribute value for transmission.
    // srcCtx describes how the attribute was received (for transcoding decisions).
    // dstCtx describes how the attribute should be encoded (destination capabilities).
    //
    // Most attributes ignore srcCtx and only use dstCtx for encoding.
    // AS_PATH/AGGREGATOR use both for RFC 6793 ASN4 transcoding.
    PackWithContext(srcCtx, dstCtx *bgpctx.EncodingContext) []byte
}
```

### Implementation Steps

**Step 4.1: Add interface method**

```go
type Attribute interface {
    // ... existing methods ...

    // PackWithContext serializes the attribute value for transmission.
    // srcCtx: how attribute was received (nil if locally originated)
    // dstCtx: how attribute should be encoded for destination
    PackWithContext(srcCtx, dstCtx *bgpctx.EncodingContext) []byte
}
```

**Step 4.2: Implement for simple attributes (default)**

For attributes that don't need context:

```go
// pkg/bgp/attribute/simple.go

func (o Origin) PackWithContext(_, _ *bgpctx.EncodingContext) []byte {
    return o.Pack() // No context dependency
}

func (m MED) PackWithContext(_, _ *bgpctx.EncodingContext) []byte {
    return m.Pack()
}

func (l LocalPref) PackWithContext(_, _ *bgpctx.EncodingContext) []byte {
    return l.Pack()
}

// ... etc for all simple attributes
```

**Step 4.3: Implement for ASPath (context-dependent)**

```go
// pkg/bgp/attribute/aspath.go

// PackWithContext serializes AS_PATH with context-dependent ASN size.
//
// RFC 6793 transcoding scenarios:
//   srcCtx.ASN4=true  → dstCtx.ASN4=true:  passthrough 4-byte
//   srcCtx.ASN4=true  → dstCtx.ASN4=false: encode 2-byte + generate AS4_PATH
//   srcCtx.ASN4=false → dstCtx.ASN4=true:  merge with AS4_PATH (done at UPDATE level)
//   srcCtx.ASN4=false → dstCtx.ASN4=false: passthrough 2-byte
//
// Note: AS4_PATH merge/generation is handled at UPDATE processing level.
// This method handles the encoding format based on dstCtx.ASN4.
func (p *ASPath) PackWithContext(srcCtx, dstCtx *bgpctx.EncodingContext) []byte {
    // Use destination context for encoding format
    if dstCtx == nil || dstCtx.ASN4 {
        return p.PackWithASN4(true)  // 4-byte ASNs
    }
    return p.PackWithASN4(false)     // 2-byte ASNs with AS_TRANS
}

// Pack returns 4-byte ASN encoding (backward compat).
// Deprecated: Use PackWithContext for context-aware encoding.
func (p *ASPath) Pack() []byte {
    return p.PackWithASN4(true)
}
```

**Step 4.4: Implement for Aggregator (context-dependent)**

```go
// pkg/bgp/attribute/simple.go

// PackWithContext serializes AGGREGATOR with context-dependent format.
// RFC 6793: 8-byte (4-byte ASN) when dstCtx.ASN4, 6-byte (2-byte ASN) otherwise.
func (a *Aggregator) PackWithContext(srcCtx, dstCtx *bgpctx.EncodingContext) []byte {
    if dstCtx == nil || dstCtx.ASN4 {
        // 8-byte format: 4-byte ASN + 4-byte IP
        buf := make([]byte, 8)
        binary.BigEndian.PutUint32(buf[0:4], a.ASN)
        copy(buf[4:8], a.Address.AsSlice())
        return buf
    }

    // 6-byte format: 2-byte ASN + 4-byte IP
    asn := a.ASN
    if asn > 65535 {
        asn = 23456 // AS_TRANS per RFC 6793
    }
    buf := make([]byte, 6)
    binary.BigEndian.PutUint16(buf[0:2], uint16(asn))
    copy(buf[2:6], a.Address.AsSlice())
    return buf
}
```

### Transcoding Matrix (RFC 6793)

| srcCtx.ASN4 | dstCtx.ASN4 | AS_PATH Action | AS4_PATH Action |
|-------------|-------------|----------------|-----------------|
| true | true | Encode 4-byte | Not needed |
| true | false | Encode 2-byte (AS_TRANS) | Generate from AS_PATH |
| false | true | Merge with AS4_PATH | Remove after merge |
| false | false | Encode 2-byte | Passthrough |

Note: AS4_PATH merge/generation is handled at UPDATE level, not individual attribute.

### Affected Attributes

| Attribute | Uses srcCtx? | Uses dstCtx? | Notes |
|-----------|--------------|--------------|-------|
| ORIGIN | No | No | Simple passthrough |
| AS_PATH | Future | **Yes (ASN4)** | Encoding format |
| NEXT_HOP | No | No | Simple passthrough |
| MED | No | No | Simple passthrough |
| LOCAL_PREF | Future | Future | IBGP/EBGP filtering |
| ATOMIC_AGGREGATE | No | No | Simple passthrough |
| AGGREGATOR | Future | **Yes (ASN4)** | Encoding format |
| COMMUNITIES | No | No | Simple passthrough |
| ORIGINATOR_ID | No | No | Simple passthrough |
| CLUSTER_LIST | No | No | Simple passthrough |
| MP_REACH_NLRI | No | **Yes** | AddPath in NLRI |
| MP_UNREACH_NLRI | No | **Yes** | AddPath in NLRI |
| EXT_COMMUNITIES | No | No | Simple passthrough |
| LARGE_COMMUNITIES | No | No | Simple passthrough |

### Tests

```go
// TestASPathPackWithContext_ASN4 verifies 4-byte encoding.
//
// VALIDATES: 4-byte ASNs when ctx.ASN4 = true.
//
// PREVENTS: Parse errors on ASN4 peers.
func TestASPathPackWithContext_ASN4(t *testing.T)

// TestASPathPackWithContext_ASN2 verifies 2-byte encoding.
//
// VALIDATES: 2-byte ASNs with AS_TRANS when ctx.ASN4 = false.
//
// PREVENTS: Parse errors on legacy peers.
func TestASPathPackWithContext_ASN2(t *testing.T)

// TestAggregatorPackWithContext_ASN4 verifies 8-byte format.
//
// VALIDATES: 8-byte format when ctx.ASN4 = true.
func TestAggregatorPackWithContext_ASN4(t *testing.T)

// TestAggregatorPackWithContext_ASN2 verifies 6-byte format.
//
// VALIDATES: 6-byte format with AS_TRANS when ctx.ASN4 = false.
func TestAggregatorPackWithContext_ASN2(t *testing.T)
```

---

## Implementation Order

```
Phase 0: pkg/bgp/context/ (spec-encoding-context-impl.md)
    │
    ├── context.go (EncodingContext, Hash, methods)
    ├── registry.go (ContextRegistry, ContextID)
    └── negotiated.go (FromNegotiatedRecv/Send)
    │
    ▼
Phase 1: Peer Integration
    │
    ├── Add recvCtx/sendCtx fields
    ├── Create contexts in onEstablished
    └── Clear contexts in onTeardown
    │
    ▼
Phase 4: PackWithContext (can parallel with Phase 2)
    │
    ├── Add interface method
    ├── Implement for simple attrs (default)
    ├── Implement for ASPath
    └── Implement for Aggregator
    │
    ▼
Phase 2: Route Storage
    │
    ├── Add wireBytes/sourceCtxID fields
    ├── NewRouteWithWireCache constructor
    └── CanForwardDirect method
    │
    ▼
Phase 3: Zero-Copy Forwarding
    │
    ├── PackAttributesFor on Route
    ├── Update peer forwarding logic
    └── Metrics/logging for cache hits
```

---

## Verification Checklist

### Phase 1: Peer Integration
- [ ] Tests for context creation on establish
- [ ] Tests for asymmetric ADD-PATH
- [ ] Tests for cleanup on teardown
- [ ] `make test && make lint` passes

### Phase 4: PackWithContext
- [ ] Tests for ASPath ASN4=true
- [ ] Tests for ASPath ASN4=false
- [ ] Tests for Aggregator ASN4=true
- [ ] Tests for Aggregator ASN4=false
- [ ] All simple attrs have PackWithContext
- [ ] `make test && make lint` passes

### Phase 2: Route Storage
- [ ] Tests for wire cache storage
- [ ] Tests for CanForwardDirect
- [ ] `make test && make lint` passes

### Phase 3: Zero-Copy Forwarding
- [ ] Tests for zero-copy path
- [ ] Tests for re-encode path
- [ ] Functional tests still pass
- [ ] `make test && make lint` passes

---

## Memory/Performance Impact

### Memory

| Change | Impact |
|--------|--------|
| Peer: 2 context pointers + 2 IDs | +20 bytes per peer |
| Route: wireBytes + ContextID | +10 bytes + wire size per route |
| Registry: context storage | ~100 bytes per unique context |

With 1000 peers (10 unique contexts due to dedup): ~1KB registry
With 1M routes: ~10MB + wire bytes (often shared via dedup)

### CPU

| Path | Before | After |
|------|--------|-------|
| Forward (same caps) | Re-encode | Zero-copy (uint16 compare) |
| Forward (diff caps) | Re-encode | Re-encode (same) |
| Receive | Parse | Parse (same) |

**Benefit:** Route reflection with same-capability clients is O(1) instead of O(n) where n = attribute count.

---

**Created:** 2025-12-29
**Status:** Ready for phased implementation
**Dependencies:** spec-encoding-context-impl.md (Phase 0)
