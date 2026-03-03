# Encoding Context System

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Purpose** | Capability-dependent encoding, zero-copy when contexts match |
| **Key Types** | `EncodingContext`, `NegotiatedCapabilities`, `ContextID` (uint16) |
| **Key Functions** | `FromNegotiatedRecv/Send()`, `Registry.Register()`, `nc.Has()`, `nc.Families()` |
| **Zero-Copy Rule** | If `sourceCtxID == destCtxID`, return cached wire bytes directly |
| **Files** | `internal/plugins/bgp/context/`, `internal/plugins/bgp/reactor/peer.go`, `internal/plugin/wire_update.go` |

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
internal/plugins/bgp/context/
├── context.go      # EncodingContext struct, Hash(), ToPackContext()
├── registry.go     # ContextRegistry, ContextID, global Registry
└── negotiated.go   # FromNegotiatedRecv/Send() helpers

internal/plugins/bgp/reactor/
└── negotiated.go   # NegotiatedCapabilities struct
```

## Family Type

All AFI/SAFI types are consolidated in `nlri.Family`. Other packages use type aliases:

```go
// internal/plugins/bgp/nlri/nlri.go - canonical definition
type Family struct { AFI AFI; SAFI SAFI }

// internal/plugins/bgp/capability/capability.go - alias for backward compat
type Family = nlri.Family

// internal/plugins/bgp/context/context.go - alias
type Family = nlri.Family
```

## NegotiatedCapabilities

Tracks "what was negotiated" - which families are enabled. Lives in `internal/plugins/bgp/reactor/`.

```go
type NegotiatedCapabilities struct {
    families             map[nlri.Family]bool  // private, O(1) lookup
    ExtendedMessage      bool                  // RFC 8654
    EnhancedRouteRefresh bool                  // RFC 7313
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

Captures all capability flags that affect wire encoding. Lives in `internal/plugins/bgp/context/`.
References sub-components from `capability.Negotiated` for zero duplication.

```go
type EncodingContext struct {
    // References to sub-components (no copy)
    identity  *capability.PeerIdentity
    encoding  *capability.EncodingCaps

    // Direction-specific derived data
    direction Direction
    addPath   map[nlri.Family]bool  // Derived from encoding.AddPathMode + direction
}

// EncodingCaps in internal/plugins/bgp/capability/encoding.go
type EncodingCaps struct {
    ASN4            bool                      // RFC 6793: 4-byte ASN support
    ExtendedMessage bool                      // RFC 8654: max message 65535 bytes
    Families        []Family                  // Negotiated address families
    AddPathMode     map[Family]AddPathMode    // RFC 7911: per-family ADD-PATH mode
    ExtendedNextHop map[Family]AFI            // RFC 8950: next-hop AFI per family
}
```

**ExtendedMessage:** Determines max message size (4096 standard, 65535 extended).
Previously in SessionCaps, moved to EncodingCaps because it affects wire encoding.

**ExtendedNextHop:** Stores the next-hop AFI (not just bool). For example,
`ExtendedNextHop[IPv4Unicast] = AFIIPv6` means IPv4 unicast can use IPv6 next-hop.

### Key Methods

- `ASN4() bool` - Returns true if 4-byte ASN negotiated
- `ExtendedMessage() bool` - Returns true if extended message negotiated
- `MaxMessageSize() int` - Returns 65535 if extended, 4096 otherwise
- `AddPath(family) bool` - Returns true if ADD-PATH enabled for family in this direction
- `LocalASN() uint32` - Returns local AS number
- `PeerASN() uint32` - Returns peer AS number
- `IsIBGP() bool` - Returns true if iBGP session
- `ToPackContext(family) *nlri.PackContext` - Convert to NLRI pack context
- `Hash() uint64` - FNV-64 hash for deduplication

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

## WireUpdate: Zero-Copy Receive Path

When UPDATE messages are received, the session creates a `WireUpdate` for zero-copy
from read to API notification. The buffer is returned to pool after processing.

### Buffer Pools

Two size-appropriate pools handle standard (4K) and extended (64K) messages:

```go
// internal/plugins/bgp/reactor/session.go
var readBufPool4K = sync.Pool{
    New: func() any { return make([]byte, message.MaxMsgLen) },  // 4096
}
var readBufPool64K = sync.Pool{
    New: func() any { return make([]byte, message.ExtMsgLen) }, // 65535
}

// ReturnReadBuffer returns buffer to appropriate pool (exported for cache)
func ReturnReadBuffer(buf []byte)
```

### Session Flow (Zero-Copy with Ownership Transfer)

```go
// In readAndProcessMessage():

// 1. Get buffer from appropriate pool (4K before OPEN, 64K after Extended Message)
buf := s.getReadBuffer()

// 2. Read message into buffer
conn.Read(buf[:HeaderLen])
conn.Read(buf[HeaderLen:msgLen])

// 3. Process message - callback returns kept=true if it took buffer ownership
err, kept := s.processMessage(&hdr, buf[HeaderLen:msgLen], buf)

// 4. Return buffer to pool only if callback didn't keep it
if !kept {
    s.returnReadBuffer(buf)
}

// In processMessage():
// 5. For UPDATE: create WireUpdate, notify callback with buf
wireUpdate := api.NewWireUpdate(body, ctxID)
kept = s.onMessageReceived(addr, TypeUPDATE, body, wireUpdate, ctxID, "received", buf)
s.handleUpdate(wireUpdate)
return err, kept
```

**Key principle:** Zero-copy ownership transfer. Cache takes buffer ownership for received UPDATEs;
buffer returned to pool when cache entry is evicted or deleted.

### WireUpdate Structure

```go
// internal/plugin/wire_update.go
type WireUpdate struct {
    payload     []byte               // UPDATE body bytes (owned, not copied)
    sourceCtxID bgpctx.ContextID     // Encoding context for zero-copy decisions
    messageID   uint64               // Unique ID for forward-by-id
    sourceID    source.SourceID      // Source that sent/created this message
}

// Derived accessors (all zero-copy slices into payload)
// Return (nil, nil) for valid empty, (nil, error) for malformed
func (u *WireUpdate) Withdrawn() ([]byte, error)           // RFC 4271 withdrawn routes
func (u *WireUpdate) Attrs() (*AttributesWire, error)      // Lazy-parsed attributes
func (u *WireUpdate) NLRI() ([]byte, error)                // RFC 4271 NLRI
func (u *WireUpdate) MPReach() (MPReachWire, error)        // RFC 4760 MP_REACH_NLRI
func (u *WireUpdate) MPUnreach() (MPUnreachWire, error)    // RFC 4760 MP_UNREACH_NLRI
```

### Context Propagation

The session's `recvCtxID` is set by Peer after capability negotiation:

```go
// internal/plugins/bgp/reactor/peer.go - setEncodingContexts()
p.recvCtxID = bgpctx.Registry.Register(recvCtx)
p.session.SetRecvCtxID(p.recvCtxID)  // Propagate to session
```

This ensures WireUpdate carries the correct context for forwarding decisions.

### RawMessage Integration

```go
// internal/plugin/types.go
type RawMessage struct {
    Type       message.MessageType
    RawBytes   []byte              // Zero-copy reference to WireUpdate.Payload()
    WireUpdate *WireUpdate         // Non-nil for UPDATE messages
    AttrsWire  *AttributesWire     // Derived from WireUpdate.Attrs()
    // ...
}
```

### Zero-Copy Flow (Ownership Transfer)

```
┌─────────────────────────────────────────────────────────────────┐
│                    RECEIVE PATH (Zero-Copy)                      │
│                                                                 │
│  buf := s.getReadBuffer()  ← Get from appropriate pool (4K/64K) │
│         ↓                                                       │
│  conn.Read(buf)            ← Read directly into pool buffer     │
│         ↓                                                       │
│  processMessage(buf)       ← Callback + handler                 │
│         ↓                                                       │
│  For UPDATE: WireUpdate(slice) → callback(wireUpdate, buf)      │
│         ↓                                                       │
│  receiver.OnMessageReceived() ← Callback FIRST (buf valid)      │
│         ↓                                                       │
│  Cache.Add(buf)?  ─── YES ──→ Cache takes buf ownership         │
│         │                     kept=true, buf NOT returned       │
│         NO                              ↓                       │
│         ↓                     Take() transfers ownership        │
│  kept=false                             ↓                       │
│         ↓                     caller.Release() → pool           │
│  s.returnReadBuffer(buf)                                        │
└─────────────────────────────────────────────────────────────────┘
```

**Cache contract:**
- Callback executes BEFORE caching (buffer always valid during callback)
- Cache takes ownership via `Add()`, no copy
- `Take(id)` removes entry and transfers ownership to caller
- Caller MUST call `ReceivedUpdate.Release()` to return buffer to pool
- `Contains(id)` checks existence without taking ownership

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

### WireWriter Interface

All wire types (Message, Attribute, NLRI) implement a common interface:

```go
// internal/plugins/bgp/context/context.go
// Note: In context package (not wire) due to import cycle: wire→context→nlri→wire
type WireWriter interface {
    // Len returns wire size in bytes. Pass nil for context-independent types.
    Len(ctx *EncodingContext) int

    // WriteTo writes to buf at offset, returns bytes written.
    // Caller guarantees capacity. Pass nil for context-independent types.
    WriteTo(buf []byte, off int, ctx *EncodingContext) int
}
```

Message and Attribute interfaces embed WireWriter:

```go
// internal/plugins/bgp/message/message.go
type Message interface {
    context.WireWriter
    Type() MessageType
}

// internal/plugins/bgp/attribute/attribute.go (planned)
type Attribute interface {
    context.WireWriter
    Code() AttributeCode
    Flags() AttributeFlags
}
```

### ASN4 (RFC 6793)

AS_PATH and AGGREGATOR encode differently based on ASN4:
- ASN4=true: 4-byte AS numbers
- ASN4=false: 2-byte AS numbers, use AS_TRANS for >65535

```go
func (p *ASPath) Len(ctx *EncodingContext) int {
    if ctx == nil || ctx.ASN4() {
        return p.len4byte()
    }
    return p.len2byte()
}

func (p *ASPath) WriteTo(buf []byte, off int, ctx *EncodingContext) int {
    if ctx == nil || ctx.ASN4() {
        return p.writeTo4byte(buf, off)
    }
    return p.writeTo2byte(buf, off)
}
```

### Transcoding (srcCtx → dstCtx)

For attributes that may need transcoding between different encoding contexts:

```go
// Only AS_PATH and Aggregator implement this
type Transcoder interface {
    WireWriter
    LenTranscode(srcCtx, dstCtx *EncodingContext) int
    WriteToTranscode(buf []byte, off int, srcCtx, dstCtx *EncodingContext) int
}
```

### ADD-PATH (RFC 7911)

NLRI encoding includes path-id when ADD-PATH negotiated:
- Without: `[payload]` (length + prefix)
- With: `[path-id(4)][payload]`

**Canonical encoding method:**

```go
packCtx := destCtx.ToPackContext(family)

// Calculate wire length
size := nlri.LenWithContext(n, packCtx)  // +4 when packCtx.AddPath=true

// Write NLRI with ADD-PATH handling
buf := make([]byte, size)
nlri.WriteNLRI(n, buf, 0, packCtx)  // Prepends path ID when AddPath=true
```

**Note:** NLRI `Len()` and `WriteTo()` return payload-only (no path ID).
Use `LenWithContext()` and `WriteNLRI()` for ADD-PATH aware encoding.

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
| `internal/plugins/bgp/nlri/nlri.go` | Canonical `Family` type, `FamilyLess()` |
| `internal/plugins/bgp/context/context.go` | EncodingContext struct (references sub-components) |
| `internal/plugins/bgp/context/registry.go` | ContextRegistry, global Registry |
| `internal/plugins/bgp/context/negotiated.go` | FromNegotiatedRecv/Send factories |
| `internal/plugins/bgp/capability/identity.go` | PeerIdentity sub-component |
| `internal/plugins/bgp/capability/encoding.go` | EncodingCaps sub-component |
| `internal/plugins/bgp/capability/session.go` | SessionCaps sub-component |
| `internal/plugins/bgp/reactor/negotiated.go` | NegotiatedCapabilities struct |
| `internal/plugins/bgp/rib/route.go` | Wire cache fields, Pack*For methods |
| `internal/plugins/bgp/reactor/peer.go` | Peer.negotiated, recvCtx, sendCtx fields |

## Related Specs

- `docs/learned/039-spec-encoding-context-impl.md` - Original design (completed)
- `docs/plan/spec-context-full-integration.md` - Full integration plan (active)
- `docs/learned/063-spec-afi-safi-map-refactor.md` - NegotiatedCapabilities, Family consolidation (completed)
- `docs/learned/057-spec-attributes-wire.md` - Lazy-parsed wire attribute storage (completed)
- `docs/learned/059-spec-pool-handle-migration.md` - Migration to pool handles (completed)
- `docs/learned/070-spec-wireupdate-buffer-lifecycle.md` - Buffer pool get/return lifecycle (completed)
- `docs/learned/078-wireupdate-split.md` - Wire-level UPDATE splitting (completed)

---

**Last Updated:** 2026-01-30
