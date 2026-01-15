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

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover numeric inputs

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] Architecture docs updated with learnings

### Completion
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
