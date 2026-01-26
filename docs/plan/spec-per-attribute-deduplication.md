# Spec: Per-Attribute Deduplication

## Task

Enhance plugin RIB pool storage to deduplicate at per-attribute-type level instead of whole-blob level, improving memory efficiency when routes share common attributes but differ in others (e.g., same ORIGIN/AS_PATH but different MED).

## Background

**Extracted from:** `spec-plugin-rib-pool-storage.md` Phase 6

**Prerequisite:** Phases 1-4 of pool storage complete (see `done/NNN-plugin-rib-pool-storage.md`)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - Section 4: per-attribute-type pools
- [ ] `docs/architecture/pool-architecture.md` - Pool design patterns

### Source Files
- [ ] `internal/plugin/rib/storage/familyrib.go` - Current blob-based storage
- [ ] `internal/bgp/attribute/iterator.go` - AttrIterator for parsing
- [ ] `internal/pool/pool.go` - Pool infrastructure

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

- `internal/plugin/rib/storage/familyrib.go` - Use RouteEntry instead of blob handle

## Files to Create

- `internal/pool/attributes.go` - Per-attribute-type pool instances
- `internal/plugin/rib/storage/routeentry.go` - RouteEntry struct
- `internal/plugin/rib/storage/attrparse.go` - Attribute parser using AttrIterator
- `internal/plugin/rib/storage/attrparse_test.go` - Parser tests
- `internal/plugin/rib/storage/routeentry_test.go` - RouteEntry tests

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
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] Architecture docs updated

### Completion
- [ ] Spec moved to `docs/plan/done/`
