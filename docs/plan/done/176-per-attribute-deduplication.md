# Spec: Per-Attribute Deduplication

## Task

Enhance plugin RIB pool storage to deduplicate at per-attribute-type level instead of whole-blob level, improving memory efficiency when routes share common attributes but differ in others (e.g., same ORIGIN/AS_PATH but different MED).

## Background

**Extracted from:** `spec-plugin-rib-pool-storage.md` Phase 6

**Prerequisite:** Phases 1-4 of pool storage complete (see `done/NNN-plugin-rib-pool-storage.md`)

## Required Reading

### Architecture Docs
- [x] `docs/architecture/core-design.md` - Section 4: per-attribute-type pools
- [x] `docs/architecture/pool-architecture.md` - Pool design patterns

### Source Files
- [x] `internal/plugin/rib/storage/familyrib.go` - Current blob-based storage
- [x] `internal/plugin/bgp/attribute/iterator.go` - AttrIterator for parsing
- [x] `internal/pool/pool.go` - Pool infrastructure

## Problem Statement

**Current implementation** stores entire attribute blob as one unit:

| Route | Attributes | Pool Entry |
|-------|------------|------------|
| 10.0.0.0/24 | ORIGIN=IGP, LP=100, MED=10 | blob1 |
| 10.0.1.0/24 | ORIGIN=IGP, LP=100, MED=20 | blob2 (full duplicate!) |

**Target design** uses per-attribute-type pools:

| Route | ORIGIN | LOCAL_PREF | MED |
|-------|--------|------------|-----|
| 10.0.0.0/24 | handle→IGP | handle→100 | handle→10 |
| 10.0.1.0/24 | handle→IGP (shared!) | handle→100 (shared!) | handle→20 |

## Memory Impact

| Scenario | Current (blob) | Target (per-attr) |
|----------|----------------|-------------------|
| 1M routes, same ORIGIN/LP | 1M × ~50B = 50MB | 3 ORIGIN + few LP refs ≈ 1MB |
| Routes differ only in MED | Full blob duplicated | Only MED differs, rest shared |
| Route reflector (same attrs) | Good dedup | Same (blob identical) |

**Worst case improvement:** Routes with identical ORIGIN, AS_PATH, LOCAL_PREF but different MED currently get zero sharing.

## Design

### Per-Attribute Pools

| Pool | Expected Entries | Rationale |
|------|------------------|-----------|
| Origin | 3 | IGP, EGP, INCOMPLETE only |
| ASPath | 10,000 | Many unique, shared across routes |
| LocalPref | 100 | Few unique values (100, 200, etc.) |
| MED | 1,000 | Variable |
| NextHop | 1,000 | Per-peer typically |
| Communities | 5,000 | Moderate sharing |
| LargeCommunities | 1,000 | Less common |
| ExtCommunities | 1,000 | RT/RD values |
| ClusterList | 100 | Route reflector only |
| OriginatorID | 100 | Route reflector only |

### RouteEntry Structure

| Field | Type | Description |
|-------|------|-------------|
| Origin | pool.Handle | ORIGIN attribute handle |
| ASPath | pool.Handle | AS_PATH handle |
| LocalPref | pool.Handle | LOCAL_PREF handle |
| MED | pool.Handle | MED handle |
| NextHop | pool.Handle | NEXT_HOP handle |
| Communities | pool.Handle | COMMUNITIES handle |
| LargeCommunities | pool.Handle | LARGE_COMMUNITIES handle |
| ExtCommunities | pool.Handle | EXTENDED_COMMUNITIES handle |
| ClusterList | pool.Handle | CLUSTER_LIST handle |
| OriginatorID | pool.Handle | ORIGINATOR_ID handle |
| OtherAttrs | pool.Handle | Unknown/other attrs as blob |

Use `pool.InvalidHandle` for attributes not present in route.

### Attribute Parsing

Use existing `AttrIterator` from `internal/bgp/attribute/iterator.go`:
1. Iterate raw attribute bytes
2. Switch on type code
3. Intern each attribute value in its typed pool
4. Accumulate unknown attributes into OtherAttrs blob

### Wire Reconstruction

For route replay/resend, reconstruct wire bytes:
1. Read each attribute from its pool via handle
2. Write in RFC 4271 Appendix F.3 order
3. Use existing `WriteAttributesOrdered` pattern

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestParseAttributes_AllTypes` | `storage/attrparse_test.go` | Parses all attribute types |
| `TestParseAttributes_Optional` | `storage/attrparse_test.go` | Handles missing optional attrs |
| `TestRouteEntry_SharedOrigin` | `storage/routeentry_test.go` | Two routes share ORIGIN handle |
| `TestRouteEntry_DifferentMED` | `storage/routeentry_test.go` | Same LP/ORIGIN, different MED = partial sharing |
| `TestRouteEntry_ToWireBytes` | `storage/routeentry_test.go` | Reconstructs valid wire format |
| `TestRouteEntry_WireRoundTrip` | `storage/routeentry_test.go` | Parse → store → reconstruct matches |
| `TestFamilyRIB_PerAttrDedup` | `storage/familyrib_test.go` | Memory savings with per-attr pools |

### Boundary Tests

| Field | Range | Last Valid | Invalid |
|-------|-------|------------|---------|
| ORIGIN | 0-2 | 2 (INCOMPLETE) | 3+ |
| LOCAL_PREF | 0-4294967295 | 4294967295 | N/A (u32) |
| MED | 0-4294967295 | 4294967295 | N/A (u32) |

## Files to Modify

- `internal/pool/attributes.go` - Add per-attribute-type pool instances (idx 2-12)

## Files to Create

- `internal/pool/perattr_test.go` - Per-attribute pool tests
- `internal/plugin/rib/storage/routeentry.go` - RouteEntry struct
- `internal/plugin/rib/storage/routeentry_test.go` - RouteEntry tests
- `internal/plugin/rib/storage/attrparse.go` - Attribute parser using AttrIterator
- `internal/plugin/rib/storage/attrparse_test.go` - Parser tests
- `internal/plugin/rib/storage/familyrib_perattr.go` - New FamilyRIB with per-attr storage
- `internal/plugin/rib/storage/familyrib_perattr_test.go` - FamilyRIB per-attr tests

## Implementation Steps

1. **Write parser tests** - Test attribute iteration and pool interning
   → **Review:** All attribute types covered? Edge cases?

2. **Run tests** - Verify FAIL

3. **Add per-attribute pools** - Extend `internal/pool/` with typed pools
   → **Review:** Pool sizes reasonable? Missing any attribute types?

4. **Create RouteEntry struct** - Per-attribute handles
   → **Review:** All BGP attributes covered? InvalidHandle for optional?

5. **Implement attribute parser** - Use existing AttrIterator
   → **Review:** Reusing existing code? No duplication?

6. **Run tests** - Verify PASS

7. **Update FamilyRIB** - Use RouteEntry instead of blob handle
   → **Review:** Backward compatible? Migration path?

8. **Add wire reconstruction** - ToWireBytes method
   → **Review:** Correct attribute order per RFC?

9. **Verify all** - `make lint && make test && make functional`

## Existing Infrastructure (REUSE)

| Component | Location | Use For |
|-----------|----------|---------|
| AttrIterator | `internal/bgp/attribute/iterator.go` | Iterate raw attr bytes |
| AttributeCode | `internal/bgp/attribute/attribute.go` | Type codes |
| Pool | `internal/pool/pool.go` | Pool infrastructure |

**Do NOT duplicate** - extend existing code.

## Estimated Effort

| Component | Effort |
|-----------|--------|
| Per-attr pools | Low |
| Attribute parser | Medium |
| RouteEntry struct | Low |
| FamilyRIB refactor | Medium |
| Wire reconstruction | Medium |
| Tests | Medium |
| **Total** | ~2-3 days |

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] Architecture docs updated

### Completion
- [ ] Spec moved to `docs/plan/done/`

## Implementation Summary

### What Was Implemented

**Phase 1: Per-Attribute Pools** (`internal/pool/attributes.go`)
- Added 11 per-attribute pools with indices 2-12
- Origin (idx=2, 64B), ASPath (idx=3, 256KB), LocalPref (idx=4, 4KB)
- MED (idx=5, 16KB), NextHop (idx=6, 16KB), Communities (idx=7, 64KB)
- LargeCommunities (idx=8, 16KB), ExtCommunities (idx=9, 16KB)
- ClusterList (idx=10, 4KB), OriginatorID (idx=11, 4KB), OtherAttrs (idx=12, 64KB)

**Phase 2: RouteEntry** (`internal/plugin/rib/storage/routeentry.go`)
- RouteEntry struct with 11 per-attribute handles
- `Has*()` methods for attribute presence checks
- `Release()` to decrement all valid handle refcounts
- `AddRef()` for sharing between owners
- `Clone()` creates copy with AddRef

**Phase 3: Attribute Parser** (`internal/plugin/rib/storage/attrparse.go`)
- `ParseAttributes()` uses existing `AttrIterator` (no duplication)
- Routes each known attribute type to its dedicated pool
- Handles all 22 BGP attribute codes (exhaustive switch)
- Unknown + known-but-not-pooled attrs accumulated to OtherAttrs

**Phase 4: FamilyRIBPerAttr** (`internal/plugin/rib/storage/familyrib_perattr.go`)
- New `FamilyRIBPerAttr` type with `map[string]*RouteEntry` storage
- `Insert()` parses attrs, handles implicit withdraw with proper Release
- No-op detection when same NLRI + same attrs
- `LookupEntry()` returns RouteEntry for inspection
- `IterateEntry()` for iteration with RouteEntry access
- `ToWireBytes()` reconstructs wire format from pool handles

### Tests Added

| File | Tests | Coverage |
|------|-------|----------|
| `perattr_test.go` | 5 | Pool existence, indices, intern/get, dedup, cross-pool |
| `routeentry_test.go` | 7 | Empty, Has*, Release, AddRef, Clone, sharing |
| `attrparse_test.go` | 9 | All types, optional, unknown, mixed, empty, dedup, ext-length |
| `familyrib_perattr_test.go` | 7 | Per-attr dedup, insert, implicit withdraw, remove, iterate, no-op, wire roundtrip |

### Design Decisions

| Decision | Rationale |
|----------|-----------|
| New `FamilyRIBPerAttr` type | Keeps existing `FamilyRIB` intact for migration |
| Pool indices 2-12 | idx=0 (Attributes), idx=1 (NLRI) already used |
| OtherAttrs blob | Known-but-not-pooled attrs (MP_REACH, etc.) stored together |
| No-op detection by handle equality | Same handles = same data, skip redundant release/intern |
| Wire reconstruction in RouteEntry | Keeps pool access encapsulated |

### Design Insights

- AttrIterator exists at `internal/plugin/bgp/attribute/` not `internal/bgp/attribute/`
- Attribute codes include 22 types; exhaustive switch required for lint
- Extended length (>255 bytes) needs flag 0x10 in wire reconstruction
- Implicit withdraw must save slot values before Release invalidates handles

### Deviations from Plan

- Created new `FamilyRIBPerAttr` instead of modifying existing `FamilyRIB`
- `ToWireBytes()` on RouteEntry instead of separate reconstruction function
- OtherAttrs includes known-but-not-individually-pooled attrs (MP_REACH, AGGREGATOR, etc.)

### Known Limitations

**Partial flag not preserved for pooled optional-transitive attributes**

RFC 4271 defines the Partial flag (0x20) for optional transitive attributes that were
modified by a router that didn't understand them. For individually-pooled attributes
(COMMUNITIES, LARGE_COMMUNITIES, EXT_COMMUNITIES), we store only the VALUE bytes,
not the flags. When reconstructing wire format, we use hardcoded flags (0xC0).

| Attribute | Stored | Reconstructed Flags | Partial Preserved |
|-----------|--------|---------------------|-------------------|
| COMMUNITIES | Value only | 0xC0 | No |
| LARGE_COMMUNITIES | Value only | 0xC0 | No |
| EXT_COMMUNITIES | Value only | 0xC0 | No |
| OtherAttrs | Full wire | Original | Yes |

**Impact:** Low. The Partial flag is informational and rarely set in practice since all
modern BGP implementations understand these community attributes. Routes will still
function correctly; only the Partial metadata is lost.

**Mitigation:** If exact flag preservation is required, store these attributes in
OtherAttrs instead of individual pools. This trades deduplication efficiency for
byte-exact round-trip.
