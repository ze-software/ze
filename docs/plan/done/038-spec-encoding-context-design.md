# Spec: Encoding Context Design

## MANDATORY READING (BEFORE IMPLEMENTATION)

```
┌─────────────────────────────────────────────────────────────────┐
│  STOP. Read these files FIRST before ANY implementation:        │
│                                                                 │
│  1. .claude/ESSENTIAL_PROTOCOLS.md - Session rules, TDD         │
│  2. .claude/INDEX.md - Find what docs to load                   │
│  3. docs/plan/CLAUDE_CONTINUATION.md - Current state                 │
│  4. THIS SPEC FILE - Design requirements                        │
│  5. pkg/bgp/context/*.go - Current implementation               │
│                                                                 │
│  DO NOT PROCEED until all are read and understood.              │
└─────────────────────────────────────────────────────────────────┘
```

## Core Insight

**Source context and destination context are the SAME structure.**

- Source: "How this route was encoded when received from peer"
- Dest: "How this route should be encoded when sent to peer"

Each peer has a context. Routes store a reference to their source peer's context.

---

## Design Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                     EncodingContext                             │
│  - ASN4: bool              (RFC 6793)                           │
│  - AddPath: map[Family]bool (RFC 7911)                          │
│  - ExtendedNextHop: map[Family]bool (RFC 8950)                  │
│  - IsIBGP: bool                                                 │
│  - LocalAS, PeerAS: uint32                                      │
└─────────────────────────────────────────────────────────────────┘
                              ▲
                              │ Same structure, different use
          ┌───────────────────┴───────────────────┐
          │                                       │
    Source Context                          Dest Context
    (peer who sent route)                   (peer receiving route)
          │                                       │
          ▼                                       ▼
┌─────────────────┐                     ┌─────────────────┐
│   Peer A        │                     │   Peer B        │
│   sendCtx: *Ctx │                     │   sendCtx: *Ctx │
│   sendCtxID: 1  │                     │   sendCtxID: 2  │
└─────────────────┘                     └─────────────────┘
          │                                       │
          │ receive route                         │ forward route
          ▼                                       ▼
┌─────────────────────────────────────────────────────────────────┐
│   StoredRoute                                                   │
│   - Prefix: netip.Prefix                                        │
│   - Attributes: []Attribute  (semantic, always available)       │
│   - WireBytes: []byte        (cached wire format)               │
│   - SourceCtxID: ContextID   (2 bytes!)                         │
└─────────────────────────────────────────────────────────────────┘
          │
          │ SourceCtxID == peerB.sendCtxID?
          │   YES → zero-copy forward WireBytes
          │   NO  → re-encode from Attributes
          ▼
```

---

## ContextID Optimization

Instead of storing a pointer (8 bytes) or full context (~100 bytes), store a uint16 ID:

```go
// ContextID is a compact identifier for an EncodingContext.
// Enables fast compatibility checks via integer comparison.
type ContextID uint16

// Global registry deduplicates identical contexts.
// 100 RR clients with same caps → 1 context, 1 ID
var globalRegistry = &ContextRegistry{}

type ContextRegistry struct {
    mu       sync.RWMutex
    contexts map[ContextID]*EncodingContext
    byHash   map[uint64]ContextID  // Hash → ID for deduplication
    nextID   ContextID
}

// Register returns ID for context, deduplicating identical ones.
func (r *ContextRegistry) Register(ctx *EncodingContext) ContextID {
    hash := ctx.Hash()

    r.mu.Lock()
    defer r.mu.Unlock()

    // Return existing ID if context already registered
    if id, ok := r.byHash[hash]; ok {
        return id
    }

    // Assign new ID
    id := r.nextID
    r.nextID++
    r.contexts[id] = ctx
    r.byHash[hash] = id
    return id
}

// Get retrieves context by ID (for actual encoding).
func (r *ContextRegistry) Get(id ContextID) *EncodingContext {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return r.contexts[id]
}
```

### Memory Savings

| Approach | Per-route overhead |
|----------|-------------------|
| Full context copy | ~100+ bytes |
| Pointer (64-bit) | 8 bytes |
| **ContextID (uint16)** | **2 bytes** |

With 1M routes: 6 bytes saved × 1M = **6 MB saved**

---

## EncodingContext Structure

```go
// EncodingContext holds capability-dependent encoding parameters.
// Same structure for source (receive) and destination (send).
//
// Created once per peer at session establishment.
// Registered in global registry for ID assignment.
type EncodingContext struct {
    // RFC 6793: Use 4-byte AS numbers
    ASN4 bool

    // RFC 7911: ADD-PATH enabled per family
    // Key: Family{AFI, SAFI}, Value: true if path ID included
    AddPath map[Family]bool

    // RFC 8950: Extended next-hop per family
    // Key: Family{AFI, SAFI}, Value: true if IPv6 NH for IPv4 NLRI
    ExtendedNextHop map[Family]bool

    // Session context
    IsIBGP  bool
    LocalAS uint32
    PeerAS  uint32
}

// Hash returns a deterministic hash for deduplication.
func (ctx *EncodingContext) Hash() uint64 {
    // Combine all fields into hash
    // Identical contexts → identical hash → same ContextID
}

// AddPathFor returns whether ADD-PATH is enabled for a family.
func (ctx *EncodingContext) AddPathFor(f Family) bool {
    return ctx.AddPath[f]
}
```

---

## Per-Peer Context

Each peer has **two contexts** to handle asymmetric capabilities (ADD-PATH):

```go
type Peer struct {
    // ... other fields ...

    // Receive context: used when storing routes FROM this peer
    // "How does this peer encode routes it sends to us?"
    recvCtx   *EncodingContext
    recvCtxID ContextID

    // Send context: used when encoding routes TO this peer
    // "How should we encode routes we send to this peer?"
    sendCtx   *EncodingContext
    sendCtxID ContextID
}

// At session establishment
func (p *Peer) onEstablished(neg *capability.Negotiated) {
    // Build receive context (what peer sends us)
    p.recvCtx = buildRecvContext(neg, p.localAS, p.peerAS)
    p.recvCtxID = globalRegistry.Register(p.recvCtx)

    // Build send context (what we send peer)
    p.sendCtx = buildSendContext(neg, p.localAS, p.peerAS)
    p.sendCtxID = globalRegistry.Register(p.sendCtx)
}
```

### Symmetric vs Asymmetric Capabilities

| Capability | Symmetric? | recvCtx == sendCtx? |
|------------|------------|---------------------|
| ASN4 | Yes | Same |
| Extended Next-Hop | Yes | Same |
| ADD-PATH | **No** | May differ |

For ADD-PATH (RFC 7911):
- Peer might send us path IDs (we receive ADD-PATH)
- We might not send path IDs to peer (we don't send ADD-PATH)
- `recvCtx.AddPath[family]` may differ from `sendCtx.AddPath[family]`

---

## Route Storage

```go
// StoredRoute holds a route with optional wire cache.
type StoredRoute struct {
    // Always present: semantic representation
    Prefix     netip.Prefix
    Attributes []Attribute

    // Optional: cached wire format for zero-copy forwarding
    WireBytes   []byte
    SourceCtxID ContextID  // Context used to encode WireBytes
}

// CanForwardDirect returns true if WireBytes can be sent as-is.
// This is the fast path for route reflection.
func (r *StoredRoute) CanForwardDirect(destCtxID ContextID) bool {
    return len(r.WireBytes) > 0 && r.SourceCtxID == destCtxID
}
```

---

## Route Forwarding Logic

```go
func (p *Peer) forwardRoute(route *StoredRoute) error {
    // Fast path: zero-copy if contexts match
    if route.CanForwardDirect(p.sendCtxID) {
        return p.sendRaw(route.WireBytes)
    }

    // Slow path: re-encode with destination context
    ctx := globalRegistry.Get(p.sendCtxID)
    packed := PackAttributesWithContext(route.Attributes, ctx)
    return p.sendRaw(packed)
}
```

### When Zero-Copy Works

| Scenario | SourceCtxID == DestCtxID? | Result |
|----------|---------------------------|--------|
| Same iBGP cluster, same caps | YES | Zero-copy ✓ |
| Route reflector clients, same caps | YES | Zero-copy ✓ |
| eBGP peers with same ASN4/AddPath | YES (deduplicated) | Zero-copy ✓ |
| Peer A (ASN4) → Peer B (no ASN4) | NO | Re-encode |
| Peer A (AddPath) → Peer B (no AddPath) | NO | Re-encode |

---

## Receiving Routes

When receiving a route from a peer:

```go
func (p *Peer) handleUpdate(update *message.Update) {
    // Parse attributes (always needed for RIB)
    attrs := parseAttributes(update.PathAttributes, p.recvCtx)

    // Store route with source context
    route := &StoredRoute{
        Prefix:      prefix,
        Attributes:  attrs,
        WireBytes:   update.PathAttributes,  // Keep original bytes
        SourceCtxID: p.recvCtxID,            // Tag with source
    }

    p.rib.Store(route)
}
```

---

## Integration with Existing Code

### Relationship to Current Types

```
capability.Negotiated (full session state)
        │
        ▼ computed once at session establishment
NegotiatedFamilies (current: pre-computed bools)
        │
        ▼ REPLACED BY
EncodingContext + ContextID
        │
        ▼ used for encoding
Attribute.PackWithContext(ctx)
NLRI.Pack(ctx.ToPackContext())
```

### Migration Path

1. **Phase 1**: Add `EncodingContext` and `ContextID` types
2. **Phase 2**: Add registry, integrate with Peer
3. **Phase 3**: Add `SourceCtxID` to route storage
4. **Phase 4**: Implement zero-copy forwarding

### Backward Compatibility

- Keep `NegotiatedFamilies` for now (can derive from EncodingContext)
- Keep `nlri.PackContext` (can create from EncodingContext)
- Add `PackWithContext(ctx *EncodingContext)` alongside existing `Pack()`

---

## API Summary

```go
// pkg/bgp/context/context.go

type ContextID uint16

type EncodingContext struct {
    ASN4            bool
    AddPath         map[Family]bool
    ExtendedNextHop map[Family]bool
    IsIBGP          bool
    LocalAS         uint32
    PeerAS          uint32
}

func (ctx *EncodingContext) Hash() uint64
func (ctx *EncodingContext) AddPathFor(f Family) bool
func (ctx *EncodingContext) ToPackContext(f Family) *nlri.PackContext

// pkg/bgp/context/registry.go

type ContextRegistry struct { ... }

func (r *ContextRegistry) Register(ctx *EncodingContext) ContextID
func (r *ContextRegistry) Get(id ContextID) *EncodingContext

// Global instance
var Registry = &ContextRegistry{}

// pkg/bgp/attribute/attribute.go

type Attribute interface {
    Code() AttributeCode
    Flags() AttributeFlags
    Len() int
    Pack() []byte  // Deprecated

    PackWithContext(ctx *EncodingContext) []byte  // New
}
```

---

## Decision Points

1. **Where to put EncodingContext?**
   - Option A: `pkg/bgp/context/` (new package)
   - Option B: `pkg/bgp/capability/` (extend existing)
   - Option C: `pkg/bgp/nlri/` (extend PackContext)

2. **Registry scope?**
   - Global singleton (simple)
   - Per-reactor (isolation)

3. **When to implement?**
   - Phase 1 (PackWithContext) is valuable standalone
   - Phase 3-4 (ContextID, zero-copy) can wait until RIB is implemented

---

**Created:** 2025-12-29
**Status:** Design complete, ready for implementation decision
