# ZeBGP Design Transition: Pool + Wire Lazy Parsing

**Status:** Active Design Target
**Date:** 2026-01-03
**Affects:** All storage, forwarding, and RIB-related specs

---

## Executive Summary

ZeBGP is transitioning from **parsed attribute storage** to **wire-canonical storage with lazy parsing and memory deduplication**. This document defines the target architecture and how existing specs relate to it.

---

## Current State vs. Target State

### Current: Parsed Attribute Storage

```
Receive UPDATE → Parse all attributes → Store []Attribute → Re-pack on forward
```

| Component | Storage | Forward Path |
|-----------|---------|--------------|
| Route | `[]attribute.Attribute` (parsed) | Iterate → Pack each |
| NLRI | Typed struct (40+ bytes) | Re-encode |
| Memory | O(routes × attrSize) | No sharing |

### Target: Wire-Canonical with Pool Deduplication

```
Receive UPDATE → Intern wire bytes → Store Handle → Zero-copy forward
```

| Component | Storage | Forward Path |
|-----------|---------|--------------|
| Route | `pool.Handle` (4 bytes) | `pool.Get(handle)` → zero-copy |
| NLRI | `pool.Handle` (4 bytes) | `pool.Get(handle)` → zero-copy |
| Memory | O(uniqueAttrs × attrSize) | Deduplicated |
| Parsing | On-demand via `AttributesWire.Get()` | Only when needed |

---

## Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────────┐
│                          RECEIVE PATH                                 │
│                                                                      │
│  Wire bytes from peer                                                │
│       ↓                                                              │
│  Validate (RFC 7606)                                                 │
│       ↓                                                              │
│  pool.Intern(attrBytes) → Handle  ←── Deduplication here            │
│       ↓                                                              │
│  Route { attrHandle, nlriHandle, sourceCtxID }                       │
│       ↓                                                              │
│  Store in RIB (4 bytes per route + handles)                         │
└──────────────────────────────────────────────────────────────────────┘
                              ↓
┌──────────────────────────────────────────────────────────────────────┐
│                          STORAGE (RIB)                               │
│                                                                      │
│  Route struct (minimal):                                             │
│    attrHandle:  pool.Handle     (4 bytes → shared wire bytes)       │
│    nlriHandle:  pool.Handle     (4 bytes → shared wire bytes)       │
│    sourceCtxID: ContextID       (2 bytes)                           │
│    tag:         RouteTag        (role, source info)                 │
│                                                                      │
│  Memory: 1M routes with 100K unique attrs = 100K × avgSize          │
│          (vs. current: 1M × avgSize)                                │
└──────────────────────────────────────────────────────────────────────┘
                              ↓
┌──────────────────────────────────────────────────────────────────────┐
│                          FORWARD PATH                                │
│                                                                      │
│  Forward to peer with destCtxID:                                    │
│                                                                      │
│  Case 1: sourceCtxID == destCtxID (COMMON - 90%+ for route reflector)│
│    pool.Get(route.attrHandle) → []byte (zero-copy, no parse)        │
│    Write directly to peer socket                                     │
│                                                                      │
│  Case 2: sourceCtxID != destCtxID (re-encoding needed)              │
│    attrs := NewAttributesWire(pool.Get(handle), sourceCtxID)        │
│    attrs.PackFor(destCtxID) → re-encoded bytes                      │
│                                                                      │
│  buildRIBRouteUpdate → OBSOLETE (replaced by pool forwarding)       │
└──────────────────────────────────────────────────────────────────────┘
                              ↓
┌──────────────────────────────────────────────────────────────────────┐
│                          API PATH                                    │
│                                                                      │
│  When API needs attribute values:                                    │
│                                                                      │
│  attrs := NewAttributesWire(pool.Get(handle), ctxID)                │
│  asPath, _ := attrs.Get(AttrASPath)     // Lazy parse just AS_PATH  │
│  origin, _ := attrs.Get(AttrOrigin)     // Parse just ORIGIN        │
│                                                                      │
│  Only parse what's requested - not full attribute set               │
└──────────────────────────────────────────────────────────────────────┘
```

---

## Key Components

### 1. Pool System (`POOL_ARCHITECTURE.md`)

- Double-buffer with MSB handles for compaction
- `pool.Intern(data)` → deduplicated Handle
- `pool.Get(handle)` → []byte (shared reference)
- Reference counting with `AddRef()`/`Release()`
- Incremental compaction (non-blocking)

### 2. AttributesWire (`spec-attributes-wire.md`) ✅ DONE

- Wire bytes as canonical storage
- Lazy parsing via `Get(AttrCode)`
- `PackFor(destCtxID)` for zero-copy or re-encode
- Index caching for O(1) attribute lookup

### 3. Pool Handle Migration (`spec-pool-handle-migration.md`)

- Route stores `attrHandle` + `nlriHandle` instead of parsed data
- `NewAttributesWire(pool.Get(handle), ctxID)` wraps pool data
- `Release()` required on route removal

### 4. Unified Handle + NLRI (`spec-unified-handle-nlri.md`)

- NLRI types wrap Handle (4 bytes each)
- Family derived from `handle.PoolIdx()`
- `HandleToNLRI(handle)` reconstructs typed NLRI

---

## What This Obsoletes

### Functions to Delete (Not Refactor)

| Function | Reason |
|----------|--------|
| `buildRIBRouteUpdate` | Replaced by `pool.Get()` + zero-copy forward |
| `buildGroupedUpdate` | Already deleted, UpdateBuilder used |
| Re-packing loops in forwarding | Zero-copy eliminates need |

### Patterns to Avoid

| Anti-Pattern | Why |
|--------------|-----|
| Converting to UpdateBuilder for RIB routes | Pool forwarding is simpler |
| Storing parsed `[]Attribute` in Route | Store Handle instead |
| Iterating attributes to re-pack | Zero-copy forward |

---

## Spec Alignment

### Specs That Enable This Design

| Spec | Role | Status |
|------|------|--------|
| `spec-attributes-wire.md` | AttributesWire with lazy parsing | ✅ Done |
| `spec-pool-handle-migration.md` | Route uses Handle, integrates pool | Ready |
| `spec-unified-handle-nlri.md` | NLRI as 4-byte Handle | Ready |
| `spec-encoding-context-impl.md` | ContextID for zero-copy check | ✅ Done |
| `spec-context-full-integration.md` | Peer contexts, PackFor | In Progress |

### Specs Affected by This Design

| Spec | Update Needed |
|------|---------------|
| `spec-static-route-updatebuilder.md` | Mark RIB section obsolete |
| `spec-pool-integration.md` | Clarify: attrs via pool, not factory |
| `spec-adjribout-memory-profiling.md` | Update Route memory model |
| `plugin-system-mvp.md` | Plugins use RawMessage + AttrsWire |

### Specs Unaffected

| Spec | Reason |
|------|--------|
| `spec-rfc9234-role.md` | Independent capability |
| `spec-rfc7606-validation-cache.md` | Operates on wire bytes (compatible) |
| `phase0-peer-callbacks.md` | Orthogonal to storage |

---

## Implementation Order

```
1. spec-attributes-wire.md          ✅ DONE
        ↓
2. spec-encoding-context-impl.md    ✅ DONE
        ↓
3. spec-pool-handle-migration.md    ← Pool core + AttributesWire integration
        ↓
4. spec-unified-handle-nlri.md      ← NLRI as Handle
        ↓
5. spec-context-full-integration.md ← Zero-copy forwarding path
        ↓
6. Delete buildRIBRouteUpdate       ← Replaced by pool forwarding
```

---

## Memory Model Comparison

### Current (1M routes, 10 peers, route reflector)

```
Per route:
  NLRI struct:     40 bytes
  []Attribute:     24 + attrs (~100 bytes)
  Wire cache:      ~150 bytes
  Total:           ~314 bytes

1M routes:         ~300 MB
10 peers adj-RIB:  ~3 GB (if copied)
```

### Target (Pool + Wire)

```
Per route:
  attrHandle:      4 bytes
  nlriHandle:      4 bytes
  sourceCtxID:     2 bytes
  tag:             ~8 bytes
  Total:           ~18 bytes

1M routes:         ~18 MB
Unique attrs:      ~100K × 150 bytes = 15 MB (shared)
Total:             ~33 MB

Savings:           90%+ for route reflector scenarios
```

---

## Migration Notes

### For Existing Code Using Route.Attributes()

```go
// OLD: Iterate parsed attributes
for _, attr := range route.Attributes() {
    // ... use attr
}

// NEW: Get specific attribute via lazy parsing
attrs := route.AttributesWire(pool)
asPath, _ := attrs.Get(attribute.AttrASPath)
```

### For Existing Code Building UPDATEs

```go
// OLD: buildRIBRouteUpdate (re-packs everything)
update := buildRIBRouteUpdate(route, localAS, isIBGP, ctx)

// NEW: Zero-copy forward
if route.SourceCtxID() == peer.SendCtxID() {
    // Same context - zero copy
    attrBytes := pool.Get(route.AttrHandle())
    nlriBytes := pool.Get(route.NLRIHandle())
    // Write directly
} else {
    // Different context - re-encode
    attrs := NewAttributesWire(pool.Get(route.AttrHandle()), route.SourceCtxID())
    attrBytes, _ := attrs.PackFor(peer.SendCtxID())
}
```

---

## References

- `POOL_ARCHITECTURE.md` - Pool design details
- `spec-attributes-wire.md` - AttributesWire API
- `spec-pool-handle-migration.md` - Migration phases
- `spec-encoding-context-impl.md` - Context system

---

**Last Updated:** 2026-01-03
