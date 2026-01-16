# Spec: Plugin RIB Pool Storage

## Task

Add `raw-attributes` and `raw-nlri` to engine JSON events, then migrate plugin RIB to pool-based storage for memory efficiency.

## Related Specs

| Spec | Relationship |
|------|--------------|
| `spec-unified-handle-nlri.md` | **Foundation** - Phases 1-2 (Handle encoding) done. Phases 3-6 **superseded** by this spec |
| `spec-context-full-integration.md` | **Complementary** - Provides source-ctx-id for zero-copy forwarding |
| `docs/architecture/plugin/rib-storage-design.md` | **Reference** - Design patterns for NLRISet, FamilyRIB |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - Overall architecture
- [ ] `docs/architecture/pool-architecture.md` - Pool design
- [ ] `docs/architecture/plugin/rib-storage-design.md` - NLRISet, FamilyRIB patterns
- [ ] `docs/architecture/rib-transition.md` - RIB in API programs

### RFC Summaries
- N/A - Pool storage is not protocol-specific

**Key insights:**
- Plugin receives JSON events, needs raw wire bytes for efficient pooling
- DirectNLRISet for IPv4 (1-5 bytes < 4 byte handle overhead)
- PooledNLRISet for IPv6+, VPN, EVPN (benefit from deduplication)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtractRawAttributes` | `pkg/plugin/wire_extract_test.go` | Extract attrs from UPDATE | |
| `TestExtractRawNLRI` | `pkg/plugin/wire_extract_test.go` | Extract NLRI by family | |
| `TestFormatMessage_RawFields` | `pkg/plugin/text_test.go` | JSON includes raw fields | |
| `TestDirectNLRISet_AddRemove` | `pkg/plugin/rib/storage/nlriset_test.go` | Direct set operations | |
| `TestPooledNLRISet_AddRemove` | `pkg/plugin/rib/storage/nlriset_test.go` | Pooled set operations | |
| `TestFamilyRIB_Insert` | `pkg/plugin/rib/storage/familyrib_test.go` | Basic insert | |
| `TestFamilyRIB_ImplicitWithdraw` | `pkg/plugin/rib/storage/familyrib_test.go` | Same prefix, new attrs | |
| `TestRIBManager_PoolStorage` | `pkg/plugin/rib/rib_test.go` | Routes stored with handles | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Pool idx | 0-62 | 62 | N/A | 63 (reserved) |
| Slot | 0-0xFFFFFE | 0xFFFFFE | N/A | 0xFFFFFF (reserved) |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| Pool storage integration | `test/data/plugin/` | Plugin stores routes with handles | |

## Files to Modify

- `pkg/plugin/text.go` - Add raw-attributes, raw-nlri to JSON output
- `pkg/plugin/rib/event.go` - Decode base64 raw fields
- `pkg/plugin/rib/rib.go` - Replace Route struct with pool storage

## Files to Create

- `pkg/plugin/wire_extract.go` - Extract raw bytes from WireUpdate
- `pkg/plugin/wire_extract_test.go` - Tests
- `pkg/plugin/rib/storage/nlriset.go` - NLRISet interface + implementations
- `pkg/plugin/rib/storage/nlriset_test.go` - Tests
- `pkg/plugin/rib/storage/familyrib.go` - FamilyRIB (attr handle → NLRISet)
- `pkg/plugin/rib/storage/familyrib_test.go` - Tests
- `pkg/plugin/rib/storage/peerrib.go` - PeerRIB wrapper
- `pkg/plugin/rib/storage/peerrib_test.go` - Tests

## Implementation Steps

1. **Phase 1: Engine raw bytes**
   - Write tests for wire_extract.go
   - Run tests - verify FAIL
   - Implement ExtractRawAttributes, ExtractRawNLRI
   - Run tests - verify PASS
   - Update text.go to include raw fields in JSON

2. **Phase 2: Storage package**
   - Write NLRISet tests
   - Run tests - verify FAIL
   - Implement DirectNLRISet, PooledNLRISet
   - Run tests - verify PASS
   - Write FamilyRIB tests
   - Implement FamilyRIB with reverse index

3. **Phase 3: Migrate RIBManager**
   - Update event.go to decode raw fields
   - Replace Route storage with PeerRIB
   - Update handleReceived to use pool storage

4. **Phase 4: Route replay**
   - Update sendRoutes to reconstruct from handles
   - Verify functional tests pass

5. **Verify all** - `make lint && make test && make functional`

## Implementation Summary

### What Was Implemented

**Phase 1: Engine raw bytes** ✅
- `pkg/plugin/wire_extract.go` - ExtractRawAttributes, ExtractRawNLRI, ExtractRawWithdrawn, ExtractRawComponents
- `pkg/plugin/text.go` - Added raw-attributes, raw-nlri, raw-withdrawn to JSON (format=full)
- `pkg/plugin/mpwire.go` - Added NLRIBytes(), WithdrawnBytes() methods

**Phase 2: Storage package** ✅
- `pkg/plugin/rib/storage/nlriset.go` - NLRISet interface + DirectNLRISet + PooledNLRISet
- `pkg/plugin/rib/storage/familyrib.go` - FamilyRIB with forward/reverse index
- `pkg/plugin/rib/storage/peerrib.go` - PeerRIB wrapper with thread-safe access

**Phase 3: Event parsing** ✅
- `pkg/plugin/rib/event.go` - Added RawAttributes, RawNLRI, RawWithdrawn fields and helper methods

**Phases 3-4: RIBManager migration** ✅
- Added `ribInPool map[string]*storage.PeerRIB` to RIBManager
- `handleReceived()` uses pool storage when `raw-attributes` present
- `handleState()` clears pool storage on peer down
- `statusJSON()` counts routes from both legacy and pool storage
- Fallback to legacy Route storage when raw fields absent (format=short)

### Bugs Found/Fixed

- **DirectNLRISet.nlriLen() buffer overflow** - Fixed by adding validation that computed length fits in buffer (hook auto-fixed, boundary tests added)
- **ADD-PATH heuristic bug** - Fixed `addPath := pathID != 0` → `addPath := false` (proper negotiation tracking deferred to ADD-PATH implementation)
- **Cross-storage failure handling** - Changed from silent `slog.Debug` to `slog.Warn` for visibility

### Design Insights

- DirectNLRISet stores wire bytes directly (IPv4: 1-5 bytes < 4-byte handle overhead)
- PooledNLRISet uses local index map for O(1) lookup instead of pool.Lookup()
- FamilyRIB maintains single pool ref per unique attr handle (refcount invariant)
- Cross-storage sync added by hooks: `removeFromLegacy()`, `removeFromPool()`, `prefixToWire()`, `wireToPrefix()`

### Deviations from Plan

- Added PeerRIB wrapper not in original spec - provides thread-safe multi-family access
- Cross-storage sync adds ~100 lines - more complex than original plan

### Known Limitations

1. **ADD-PATH hardcoded to false** - Pool storage assumes ADD-PATH is not negotiated.
   When ADD-PATH is implemented, need per-peer/family negotiation state tracking.
   - Mitigation: ADD-PATH peers will use format=short (legacy storage) until fixed

2. **Non-unicast families skipped** - Pool storage only processes IPv4/IPv6 unicast/multicast.
   EVPN, VPN, FlowSpec families are skipped because `splitNLRIs()` only understands
   simple `[prefix-len][prefix-bytes]` format.
   - These families continue to work with legacy Route storage (format=short)

3. **IPv6 show output non-canonical** - Manual formatting produces `2001:db8:0:0:...`
   instead of `2001:db8::`. Visual inconsistency only.

4. **next-hop missing from pool show output** - Decoding from raw attributes requires
   significant complexity. Deferred.

5. **No RawNLRI/FamilyOps alignment validation** - Assumes they match.
   Malformed events could cause silent data loss.

## Implementation Complete

### Phase 3-4: RIBManager Pool Storage

**Implemented:**
1. Added `ribInPool map[string]*storage.PeerRIB` to RIBManager
2. Updated `handleReceived()`:
   - Checks for raw fields (raw-attributes, raw-nlri, raw-withdrawn)
   - If present: uses pool storage via `handleReceivedPool()`
   - If not: falls back to legacy Route storage via `handleReceivedLegacy()`
3. Added helper functions:
   - `parseFamily()` - converts "ipv4/unicast" to nlri.Family
   - `splitNLRIs()` - splits concatenated NLRI wire bytes
4. Updated `handleState()` to clear pool storage on peer down
5. Updated `statusJSON()` to count routes from pool storage

**Design Decision: ribOut stays with Route storage**
- Pool storage only for ribIn (Adj-RIB-In)
- ribOut keeps Route storage (simpler replay with text commands)
- This is intentional - pool storage benefits memory efficiency for received routes

**Known Limitations (future work):**
- **ADD-PATH detection** - Currently `addPath := false` hardcoded. JSON events don't include negotiated capability state. `splitNLRIs()` and `formatNLRIAsPrefix()` assume no path-id prefix. Fix requires ADD-PATH state from engine.
- **Non-unicast NLRI parsing** - `formatNLRIAsPrefix()` only handles IPv4/IPv6 unicast wire format. VPN (RD+label+prefix), EVPN, FlowSpec, labeled unicast return hex fallback. Pool storage works but show output is hex.
- **Show output missing next-hop** - Pool routes in `rib adjacent inbound show` lack `next-hop` field (stored in attrs, not decoded). Legacy routes include it.

### Tests Added

| Test | File | Validates |
|------|------|-----------|
| `TestHandleReceived_PoolStorage` | `pkg/plugin/rib/rib_test.go` | Routes stored with handles |
| `TestHandleReceived_FallbackToRoute` | `pkg/plugin/rib/rib_test.go` | Works without raw fields |
| `TestHandleReceived_PoolStorage_MultipleNLRIs` | `pkg/plugin/rib/rib_test.go` | Concatenated NLRIs split |
| `TestHandleReceived_PoolStorage_Withdraw` | `pkg/plugin/rib/rib_test.go` | Withdrawal removes from pool |
| `TestHandleState_PeerDown_ClearsPoolStorage` | `pkg/plugin/rib/rib_test.go` | Pool cleared on down |
| `TestStatusJSON_WithPoolStorage` | `pkg/plugin/rib/rib_test.go` | Status counts pool routes |
| `TestPrefixToWire` | `pkg/plugin/rib/rib_test.go` | Text→wire conversion |
| `TestWireToPrefix` | `pkg/plugin/rib/rib_test.go` | Wire→text conversion |
| `TestCrossStorage_PoolToLegacy` | `pkg/plugin/rib/rib_test.go` | Pool announce clears legacy |
| `TestCrossStorage_LegacyToPool` | `pkg/plugin/rib/rib_test.go` | Legacy announce clears pool |
| `TestCrossStorage_WithdrawFromPool` | `pkg/plugin/rib/rib_test.go` | Pool withdraw clears legacy |
| `TestCrossStorage_WithdrawFromLegacy` | `pkg/plugin/rib/rib_test.go` | Legacy withdraw clears pool |
| `TestHandleInboundShow_PoolStorage` | `pkg/plugin/rib/rib_test.go` | Show reads from pool |
| `TestHandleInboundEmpty_PoolStorage` | `pkg/plugin/rib/rib_test.go` | Empty clears pool |

## Checklist

### 🧪 TDD (Phases 1-2)
- [x] Tests written
- [x] Tests FAIL then PASS
- [x] Implementation complete
- [x] Boundary tests cover numeric inputs

### 🧪 TDD (Phases 3-4)
- [x] Tests written
- [x] Tests FAIL (ribInPool undefined)
- [x] Implementation complete
- [x] Tests PASS (6 pool storage tests pass)

### Verification
- [x] `make lint` passes (phases 1-2)
- [x] `make test` passes (phases 1-2)
- [x] `make functional` passes (phases 1-2)
- [x] All tests pass after phases 3-4

### Documentation
- [x] Required docs read
- [x] Architecture docs updated with learnings

### Completion
- [x] Spec updated with final Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

---

## Phase 6: Per-Attribute Deduplication (TODO)

> **Note:** Phase 5 in `rib-storage-design.md` is Compaction. This is Phase 6.

### Problem Statement

**Current implementation** stores entire attribute blob as one unit:
```go
h := pool.Attributes.Intern(attrBytes)  // Whole blob = one pool entry
```

**TARGET design** (core-design.md §4) uses per-attribute-type pools:
```go
originPool      *Pool[Origin]      // 3 values max (IGP, EGP, INCOMPLETE)
asPathPool      *Pool[ASPath]      // Many unique, but shared across routes
localPrefPool   *Pool[uint32]      // Few unique values (100, 200, etc.)
medPool         *Pool[uint32]      // Variable
communityPool   *Pool[Communities] // Moderate sharing
// ... etc
```

### Memory Impact

| Scenario | Current (blob) | Target (per-attr) |
|----------|----------------|-------------------|
| 1M routes, same ORIGIN/LP | 1M × ~50B = 50MB | 3 ORIGIN + few LP refs ≈ 1MB |
| Routes differ only in MED | Full blob duplicated | Only MED differs, rest shared |
| Route reflector (same attrs) | Good dedup | Same (blob identical) |

**Worst case:** Routes with identical ORIGIN, AS_PATH, LOCAL_PREF but different MED get zero sharing.

### Design Decision

**Approach: Parse in plugin** (not engine)

Rationale:
- No protocol/engine changes required
- Plugin already has raw bytes
- Parse cost acceptable (once per route insert)
- Keeps engine simple

### Existing Infrastructure (REUSE - DO NOT DUPLICATE)

| Component | Location | Use For |
|-----------|----------|---------|
| `AttrIterator` | `pkg/bgp/attribute/iterator.go` | Iterate raw attr bytes |
| `AttributeCode` | `pkg/bgp/attribute/attribute.go` | Type codes (ORIGIN, AS_PATH, etc.) |
| `WriteAttributesOrdered` | `pkg/bgp/attribute/origin.go` | Wire reconstruction |
| `Pool` | `pkg/pool/pool.go` | Pool infrastructure |

### Required Changes

#### 1. Per-Attribute Pools (NEW - add to existing file)

```go
// pkg/pool/attributes.go - add typed pools alongside existing Attributes/NLRI
var (
    Origin           = NewPool(PoolConfig{ExpectedEntries: 3})      // IGP/EGP/INCOMPLETE
    ASPath           = NewPool(PoolConfig{ExpectedEntries: 10000})
    LocalPref        = NewPool(PoolConfig{ExpectedEntries: 100})    // Few unique values
    MED              = NewPool(PoolConfig{ExpectedEntries: 1000})
    NextHop          = NewPool(PoolConfig{ExpectedEntries: 1000})
    Communities      = NewPool(PoolConfig{ExpectedEntries: 5000})
    LargeCommunities = NewPool(PoolConfig{ExpectedEntries: 1000})
    ExtCommunities   = NewPool(PoolConfig{ExpectedEntries: 1000})
    ClusterList      = NewPool(PoolConfig{ExpectedEntries: 100})
    OriginatorID     = NewPool(PoolConfig{ExpectedEntries: 100})
)
```

#### 2. RouteEntry with Per-Attribute Handles (NEW)

```go
// pkg/plugin/rib/storage/routeentry.go
type RouteEntry struct {
    // Per-attribute handles (InvalidHandle if not present)
    Origin           pool.Handle
    ASPath           pool.Handle
    LocalPref        pool.Handle
    MED              pool.Handle
    NextHop          pool.Handle
    Communities      pool.Handle
    LargeCommunities pool.Handle
    ExtCommunities   pool.Handle
    ClusterList      pool.Handle
    OriginatorID     pool.Handle

    // Unknown/other attributes stored as blob
    OtherAttrs       pool.Handle
}
```

#### 3. Attribute Parser (NEW - uses existing AttrIterator)

```go
// pkg/plugin/rib/storage/attrparse.go
func ParseAttributes(raw []byte) (*RouteEntry, error) {
    entry := &RouteEntry{}
    iter := attribute.NewAttrIterator(raw)  // REUSE existing iterator

    for typeCode, _, value, ok := iter.Next(); ok; typeCode, _, value, ok = iter.Next() {
        switch typeCode {
        case attribute.AttrOrigin:
            entry.Origin = pool.Origin.Intern(value)
        case attribute.AttrASPath:
            entry.ASPath = pool.ASPath.Intern(value)
        case attribute.AttrLocalPref:
            entry.LocalPref = pool.LocalPref.Intern(value)
        case attribute.AttrMED:
            entry.MED = pool.MED.Intern(value)
        // ... etc
        default:
            // Accumulate unknown attrs into OtherAttrs
        }
    }
    return entry, nil
}
```

#### 4. FamilyRIB Changes (MODIFY existing)

```go
// Change from: map[pool.Handle]NLRISet  (blob handle → NLRIs)
// Change to:   map[string]*RouteEntry   (nlriKey → entry with per-attr handles)

type FamilyRIB struct {
    routes map[string]*RouteEntry  // nlriKey → per-attr handles
}
```

#### 5. Wire Reconstruction for Resend (NEW - uses existing WriteAttributesOrdered pattern)

```go
func (e *RouteEntry) ToWireBytes() []byte {
    // Use existing attribute.AttributesSize() + WriteAttributesOrdered() pattern
    var buf bytes.Buffer
    if e.Origin != pool.InvalidHandle {
        writeAttrWithHeader(&buf, attribute.AttrOrigin, pool.Origin.Get(e.Origin))
    }
    if e.ASPath != pool.InvalidHandle {
        writeAttrWithHeader(&buf, attribute.AttrASPath, pool.ASPath.Get(e.ASPath))
    }
    // ... etc (order per RFC 4271 Appendix F.3)
    return buf.Bytes()
}
```

### Implementation Steps

1. **Remove legacy `ribIn`** first (simplifies FamilyRIB refactor)
2. **Add per-attribute pools** to `pkg/pool/attributes.go` (extend existing file)
3. **Create RouteEntry struct** in `pkg/plugin/rib/storage/routeentry.go`
4. **Create attribute parser** using existing `AttrIterator` (DO NOT rewrite iterator)
5. **Update FamilyRIB** to use RouteEntry instead of blob handle
6. **Add wire reconstruction** using existing write patterns

### Tests Required

| Test | Validates |
|------|-----------|
| `TestParseAttributes_AllTypes` | Parses all attribute types |
| `TestRouteEntry_SharedOrigin` | Two routes share ORIGIN handle |
| `TestRouteEntry_DifferentMED` | Same LP/ORIGIN, different MED = partial sharing |
| `TestRouteEntry_ToWireBytes` | Reconstructs valid wire format |
| `TestFamilyRIB_PerAttrDedup` | Memory savings with per-attr pools |

### Dependencies

- Phase 4 complete (remove `ribIn` legacy storage)
- Attribute iterator available (check `pkg/bgp/attribute/`)

### Estimated Complexity

| Component | Effort |
|-----------|--------|
| Per-attr pools | Low (copy existing pattern) |
| Attribute parser | Medium (iterate + switch) |
| RouteEntry struct | Low |
| FamilyRIB refactor | Medium |
| Wire reconstruction | Medium |
| Tests | Medium |
| **Total** | ~2-3 days |
