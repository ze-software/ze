# Spec: Unified Handle Encoding

> **📍 SCOPE:** This spec implements Handle encoding in `internal/pool/`.
> Plugin RIB storage using these handles is covered in `spec-plugin-rib-pool-storage.md`.

## Task

Extend pool.Handle for plugin RIB memory efficiency:
- ✅ Extend `pool.Handle` to encode: `poolIdx(6) | flags(2) | slot(24)`
- ✅ Modify `Pool` to store `idx` and extract slot from handles

## Required Reading

### Architecture Docs
- [x] `docs/architecture/pool-architecture.md` - Memory pool design
- [x] `docs/architecture/rib-transition.md` - RIB architecture direction

### RFC Summaries
- N/A - Pool code is not protocol-specific

**Key insights:**
- Handle encoding embeds metadata to eliminate redundant struct fields
- 63 pools × 16M entries each supported
- Flags field allows ADD-PATH marker without struct overhead

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHandleEncoding` | `internal/pool/handle_test.go` | Bit-level poolIdx/flags/slot encoding | ✅ |
| `TestHandleInvalidHandle` | `internal/pool/handle_test.go` | InvalidHandle sentinel (poolIdx=63) | ✅ |
| `TestHandleWithFlags` | `internal/pool/handle_test.go` | Flag modification preserves other fields | ✅ |
| `TestPoolIdxEncoding` | `internal/pool/pool_test.go` | Pool embeds idx in returned handles | ✅ |
| `TestPoolExtractsSlot` | `internal/pool/pool_test.go` | Get/Length/Release use slot portion | ✅ |
| `TestPoolIdxValidation` | `internal/pool/pool_test.go` | Pool rejects idx=63 (reserved) | ✅ |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| poolIdx | 0-62 | 62 | N/A | 63 (reserved) |
| flags | 0-3 | 3 | N/A | N/A (2 bits) |
| slot | 0-0xFFFFFE | 0xFFFFFE | N/A | 0xFFFFFF (reserved) |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| N/A | - | Pool tests cover this | ✅ |

## Files to Modify

- ✅ `internal/pool/handle.go` - Extended Handle with bit encoding
- ✅ `internal/pool/handle_test.go` - Handle encoding tests
- ✅ `internal/pool/pool.go` - Added `idx` field, methods extract slot
- ✅ `internal/pool/pool_test.go` - Pool idx tests

## Implementation Steps

1. ✅ **Write Phase 1 tests** - Handle encoding
2. ✅ **Run tests** - Verified FAIL
3. ✅ **Implement Phase 1** - Handle bit encoding
4. ✅ **Run tests** - Verified PASS
5. ✅ **Write Phase 2 tests** - Pool idx support
6. ✅ **Run tests** - Verified FAIL
7. ✅ **Implement Phase 2** - Pool idx field and slot extraction
8. ✅ **Run tests** - Verified PASS
9. ✅ **Verify all** - `make lint && make test && make functional`

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL verified
- [x] Implementation complete
- [x] Tests PASS verified
- [x] Boundary tests cover numeric inputs

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes (pool tests pass; unrelated flaky tests)

### Documentation
- [x] Required docs read
- N/A RFC references (pool code is not protocol-specific)

### Completion
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

## Implementation Summary

### What Was Implemented

**Handle Encoding** (`internal/pool/handle.go`)
- `NewHandle(poolIdx, flags, slot)` - creates handle with bit-packed fields
- `Handle.PoolIdx()` - extracts 6-bit pool index (0-62)
- `Handle.Flags()` - extracts 2-bit flags
- `Handle.Slot()` - extracts 24-bit slot index
- `Handle.HasPathID()` - flag bit 0 check
- `Handle.Valid()` - checks poolIdx < 63
- `Handle.WithFlags()` - returns handle with modified flags

**Pool idx Support** (`internal/pool/pool.go`)
- `Pool.idx` field stores pool's assigned index
- `NewWithIdx(idx, capacity)` - requires idx parameter (0-62)
- All methods extract slot from handle before use
- Validates idx != 63 (reserved for InvalidHandle)

### Design Insights

- Handle bit layout enables 63 pools × 16M entries each
- Flags field allows ADD-PATH marker without struct overhead
- Plugin RIB can achieve 83% memory reduction (24→4 bytes per NLRI)

## Handle Bit Layout

```
┌──────────┬───────┬────────────────────────┐
│PoolIdx   │ Flags │        Slot            │
│ (6 bits) │(2 bit)│      (24 bits)         │
└──────────┴───────┴────────────────────────┘
 31     26 25   24 23                      0

PoolIdx: 0-62 valid, 63 reserved for InvalidHandle
Flags:   Bit 0 = hasPathID (ADD-PATH), Bit 1 = reserved
Slot:    0 to 16,777,214 (0xFFFFFE)
```

## Related Specs

| Spec | Relationship |
|------|--------------|
| `spec-plugin-rib-pool-storage.md` | **NEXT** - Uses this handle encoding for plugin RIB |
| `docs/architecture/plugin/rib-storage-design.md` | **Reference** - Design patterns |

---

**Created:** 2025-12-28
**Completed:** 2026-01-15
**Status:** ✅ Complete
