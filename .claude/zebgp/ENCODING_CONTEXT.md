# Encoding Context System

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Purpose** | Capability-dependent encoding, zero-copy when contexts match |
| **Key Types** | `EncodingContext`, `NegotiatedCapabilities`, `ContextID` (uint16) |
| **Key Functions** | `FromNegotiatedRecv/Send()`, `Registry.Register()`, `nc.Has()`, `nc.Families()` |
| **Zero-Copy Rule** | If `sourceCtxID == destCtxID`, return cached wire bytes directly |
| **Files** | `pkg/bgp/context/`, `pkg/rib/route.go`, `pkg/reactor/peer.go` |

**When to read full doc:** Route forwarding, peer session, encoding mismatches, new capabilities.

---

## Overview

The encoding context system enables capability-dependent message encoding and
zero-copy route forwarding. It consists of four layers:

1. **NegotiatedCapabilities** - Tracks "what was negotiated" (which families)
2. **EncodingContext** - Tracks "how to encode" (ASN4, ADD-PATH, ExtNH per family)
3. **ContextRegistry** - Deduplicates contexts, assigns compact IDs
4. **Route Wire Cache** - Stores original wire bytes for zero-copy forwarding

**Separation of Concerns:** NegotiatedCapabilities answers "is this family enabled?"
while EncodingContext answers "how do we encode for this peer?"

## Package Structure

```
pkg/bgp/context/
├── context.go      # EncodingContext struct, Hash(), ToPackContext()
├── registry.go     # ContextRegistry, ContextID, global Registry
└── negotiated.go   # FromNegotiatedRecv/Send() helpers

pkg/reactor/
└── negotiated.go   # NegotiatedCapabilities struct
```

## Family Type

All AFI/SAFI types are consolidated in `nlri.Family`. Other packages use type aliases:

```go
// pkg/bgp/nlri/nlri.go - canonical definition
type Family struct { AFI AFI; SAFI SAFI }

// pkg/bgp/capability/capability.go - alias for backward compat
type Family = nlri.Family

// pkg/bgp/context/context.go - alias
type Family = nlri.Family
```

## NegotiatedCapabilities

Tracks "what was negotiated" - which families are enabled. Lives in `pkg/reactor/`.

```go
type NegotiatedCapabilities struct {
    families        map[nlri.Family]bool  // private, O(1) lookup
    ExtendedMessage bool                  // RFC 8654
}
```

### Key Methods

```go
// Has returns whether the family was negotiated
func (nc *NegotiatedCapabilities) Has(f nlri.Family) bool

// Families returns all negotiated families in deterministic order (sorted by AFI, SAFI)
// Used for EOR sending where order should be reproducible for testing
func (nc *NegotiatedCapabilities) Families() []nlri.Family
```

### Usage

```go
nc := p.negotiated.Load()
if nc.Has(nlri.IPv4Unicast) {
    // family is negotiated, send routes
}

// Send EOR for all families in deterministic order
for _, family := range nc.Families() {
    p.SendUpdate(message.BuildEOR(family))
}
```

## EncodingContext

Captures all capability flags that affect wire encoding. Lives in `pkg/bgp/context/`.

```go
type EncodingContext struct {
    ASN4            bool                    // RFC 6793: 4-byte ASN support
    AddPath         map[nlri.Family]bool    // RFC 7911: per-family ADD-PATH
    ExtendedNextHop map[nlri.Family]nlri.AFI // RFC 8950: next-hop AFI per family
    IsIBGP          bool                    // iBGP vs eBGP session
    LocalAS         uint32                  // Local AS number
    PeerAS          uint32                  // Peer AS number
}
```

**ExtendedNextHop:** Stores the next-hop AFI (not just bool). For example,
`ExtendedNextHop[IPv4Unicast] = AFIIPv6` means IPv4 unicast can use IPv6 next-hop.

### Key Methods

- `Hash() uint64` - FNV-64 hash for deduplication
- `ToPackContext(family) *nlri.PackContext` - Convert to NLRI pack context
- `Equal(other) bool` - Deep equality check

## ContextRegistry

Thread-safe registry that deduplicates contexts and assigns compact IDs:

```go
type ContextID uint16  // Compact ID for fast comparison

type ContextRegistry struct {
    contexts map[ContextID]*EncodingContext
    byHash   map[uint64]ContextID
    nextID   ContextID
}

var Registry = NewRegistry()  // Global instance
```

### Usage Pattern

```go
// At session establishment
ctx := context.FromNegotiatedRecv(negotiated, localAS)
ctxID := context.Registry.Register(ctx)

// For route forwarding (fast path)
if route.CanForwardDirect(destCtxID) {
    return route.WireBytes()  // Zero-copy
}

// Slow path: re-encode
destCtx := context.Registry.Get(destCtxID)
```

## Route Wire Cache

Routes store original wire bytes for zero-copy forwarding:

```go
type Route struct {
    // ... other fields ...

    wireBytes     []byte           // Cached packed attributes
    nlriWireBytes []byte           // Cached packed NLRI
    sourceCtxID   ContextID        // Context used for encoding
}
```

### Constructors

```go
// Without cache (locally originated routes)
NewRoute(nlri, nextHop, attrs)
NewRouteWithASPath(nlri, nextHop, attrs, asPath)

// With attribute cache
NewRouteWithWireCache(nlri, nextHop, attrs, asPath, wireBytes, ctxID)

// With full cache (attributes + NLRI)
NewRouteWithWireCacheFull(nlri, nextHop, attrs, asPath, wireBytes, nlriWireBytes, ctxID)
```

### Zero-Copy Forwarding

```go
// PackAttributesFor - zero-copy when contexts match
func (r *Route) PackAttributesFor(destCtxID ContextID) []byte {
    if r.CanForwardDirect(destCtxID) {
        return r.wireBytes  // Fast path: zero-copy
    }
    destCtx := Registry.Get(destCtxID)
    return packAttributesWithContext(r.attributes, r.asPath, destCtx)
}

// PackNLRIFor - same pattern for NLRI
func (r *Route) PackNLRIFor(destCtxID ContextID) []byte {
    if len(r.nlriWireBytes) > 0 && r.sourceCtxID == destCtxID {
        return r.nlriWireBytes  // Fast path
    }
    // Slow path: re-encode
}
```

## Peer Integration

Each Peer holds negotiated capabilities and encoding contexts:

```go
type Peer struct {
    // What was negotiated (which families enabled)
    negotiated atomic.Pointer[NegotiatedCapabilities]

    // How to encode/decode (capabilities that affect wire format)
    recvCtx   *EncodingContext  // For parsing routes FROM peer
    recvCtxID ContextID
    sendCtx   *EncodingContext  // For encoding routes TO peer
    sendCtxID ContextID
}
```

Created at session establishment:
- `NewNegotiatedCapabilities(neg)` - Which families are enabled
- `FromNegotiatedRecv(neg)` - How peer sends to us (their send capabilities)
- `FromNegotiatedSend(neg)` - How we send to peer (their receive capabilities)

### Usage Pattern

```go
// Check if family is negotiated
nc := p.negotiated.Load()
if !nc.Has(nlri.IPv4Unicast) {
    return // family not negotiated, skip
}

// Get encoding context for building UPDATE
ctx := p.packContext(family)  // uses sendCtx internally
ub := message.NewUpdateBuilder(localAS, isIBGP, ctx)
```

## Performance Characteristics

### Memory

| Component | Size |
|-----------|------|
| Per Peer | +20 bytes (2 pointers + 2 IDs) |
| Per Route (with cache) | +10 bytes + wire size |
| Registry | ~100 bytes per unique context |

### CPU

| Operation | Cost |
|-----------|------|
| CanForwardDirect | O(1) - uint16 compare |
| Zero-copy forward | O(1) - slice reference |
| Re-encode | O(n) - n = attribute count |

### Route Reflection Optimization

With same-capability clients, route reflection is O(1):
- Compare context IDs (uint16)
- Return cached wire bytes directly
- No re-encoding needed

## Context-Dependent Encoding

### ASN4 (RFC 6793)

AS_PATH and AGGREGATOR encode differently based on ASN4:
- ASN4=true: 4-byte AS numbers
- ASN4=false: 2-byte AS numbers, use AS_TRANS for >65535

```go
func (p *ASPath) PackWithContext(srcCtx, dstCtx *EncodingContext) []byte {
    if dstCtx == nil || dstCtx.ASN4 {
        return p.PackWithASN4(true)   // 4-byte
    }
    return p.PackWithASN4(false)      // 2-byte with AS_TRANS
}
```

### ADD-PATH (RFC 7911)

NLRI encoding includes path-id when ADD-PATH negotiated:
- Without: length + prefix
- With: path-id(4) + length + prefix

```go
packCtx := destCtx.ToPackContext(family)
// packCtx.AddPath determines if path-id is included
nlri.Pack(packCtx)
```

## API Summary

### ID-Only Pattern

All forwarding methods take only `ContextID`, not context pointer:

```go
attrBytes := route.PackAttributesFor(peer.sendCtxID)
nlriBytes := route.PackNLRIFor(peer.sendCtxID)
```

Benefits:
- Fast path only needs ID comparison
- Slow path does single registry lookup
- Caller doesn't manage context lifecycle

### Callers Must Register

Context IDs must be registered via `Registry.Register()`:
- Unregistered ID (0) may cause incorrect zero-copy decisions
- `Registry.Get(0)` returns nil (defaults to ASN4=true behavior)

## Files

| File | Purpose |
|------|---------|
| `pkg/bgp/nlri/nlri.go` | Canonical `Family` type, `FamilyLess()` |
| `pkg/bgp/context/context.go` | EncodingContext struct |
| `pkg/bgp/context/registry.go` | ContextRegistry, global Registry |
| `pkg/bgp/context/negotiated.go` | FromNegotiatedRecv/Send helpers |
| `pkg/reactor/negotiated.go` | NegotiatedCapabilities struct |
| `pkg/rib/route.go` | Wire cache fields, Pack*For methods |
| `pkg/reactor/peer.go` | Peer.negotiated, recvCtx, sendCtx fields |

## Related Specs

- `plan/spec-encoding-context-impl.md` - Original design
- `plan/spec-context-full-integration.md` - Full integration plan
- `plan/spec-afi-safi-map-refactor.md` - NegotiatedCapabilities, Family consolidation
- `plan/spec-attributes-wire.md` - Lazy-parsed wire attribute storage
- `plan/spec-pool-handle-migration.md` - Future migration to pool handles
