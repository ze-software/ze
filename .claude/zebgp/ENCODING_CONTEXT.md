# Encoding Context System

## Overview

The encoding context system enables capability-dependent message encoding and
zero-copy route forwarding. It consists of three layers:

1. **EncodingContext** - Captures peer capabilities (ASN4, ADD-PATH, etc.)
2. **ContextRegistry** - Deduplicates contexts, assigns compact IDs
3. **Route Wire Cache** - Stores original wire bytes for zero-copy forwarding

## Package Structure

```
pkg/bgp/context/
├── context.go      # EncodingContext struct, Hash(), ToPackContext()
├── registry.go     # ContextRegistry, ContextID, global Registry
└── negotiated.go   # FromNegotiatedRecv/Send() helpers
```

## EncodingContext

Captures all capability flags that affect wire encoding:

```go
type EncodingContext struct {
    ASN4            bool              // RFC 6793: 4-byte ASN support
    AddPath         map[Family]bool   // RFC 7911: per-family ADD-PATH
    ExtendedNextHop map[Family]bool   // RFC 8950: IPv6 NH for IPv4
    IsIBGP          bool              // iBGP vs eBGP session
    LocalAS         uint32            // Local AS number
    PeerAS          uint32            // Peer AS number
}
```

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

Each Peer holds recv and send contexts:

```go
type Peer struct {
    recvCtx   *EncodingContext  // For parsing routes FROM peer
    recvCtxID ContextID
    sendCtx   *EncodingContext  // For encoding routes TO peer
    sendCtxID ContextID
}
```

Created at session establishment via:
- `FromNegotiatedRecv()` - What peer sends us (their send capabilities)
- `FromNegotiatedSend()` - What we send to peer (their receive capabilities)

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
| `pkg/bgp/context/context.go` | EncodingContext struct |
| `pkg/bgp/context/registry.go` | ContextRegistry, global Registry |
| `pkg/bgp/context/negotiated.go` | FromNegotiatedRecv/Send helpers |
| `pkg/rib/route.go` | Wire cache fields, Pack*For methods |
| `pkg/reactor/peer.go` | Peer context fields |

## Related Specs

- `plan/spec-encoding-context-impl.md` - Original design
- `plan/spec-context-full-integration.md` - Full integration plan
