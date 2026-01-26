# Spec: Pool Double-Buffer Non-Blocking Compaction

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/pool-architecture.md` - pool design (target)
4. `internal/pool/*.go` - current implementation

## Task

Add missing features to current pool implementation:
1. **Non-blocking incremental compaction** (double-buffer design from docs)
2. **AddRef()** - share handles between owners
3. **GetBySlot() / ReleaseBySlot()** - normalized slot access
4. **Handle normalization** - dedup across compaction cycles

While preserving current features: scheduler, metrics, shutdown, activity tracking, pool index validation, flags.

## Background

Current pool (restored from delete-pool) has single-buffer stop-the-world compaction. Docs describe double-buffer non-blocking compaction. Need to add missing features:

| Feature | Current | Target (from docs) |
|---------|---------|-------------------|
| Compaction | Stop-the-world `Compact()` | Incremental `MigrateBatch()` |
| Buffer design | Single buffer | Double-buffer alternating |
| AddRef() | Missing | Share handles |
| GetBySlot() | Missing | Normalized access |
| ReleaseBySlot() | Missing | Normalized release |
| Handle validity | Always valid or invalid | Both old/new valid during compaction |
| Buffer freeing | Immediate | When refCount=0 |

**Preserving from current:**
- Scheduler, Metrics, Shutdown, Activity tracking
- Pool index validation (5-bit instead of 6-bit)
- Flags (ADD-PATH support)
- Error types, InternWithError()

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/pool-architecture.md` - Target double-buffer design
- [ ] `docs/architecture/core-design.md` - Section 4: per-attribute-type pools

### Source Files
- [ ] `internal/pool/pool.go` - Current single-buffer implementation
- [ ] `internal/pool/handle.go` - Current handle layout
- [ ] `internal/pool/scheduler.go` - Current scheduler

**Key insights:**
- Docs describe double-buffer with MSB buffer bit
- Current has poolIdx(6) + flags(2) + slot(24)
- Hybrid layout: bufferBit(1) + poolIdx(5) + flags(2) + slot(24)

## Handle Layout Design

**Hybrid layout (32-bit):**
```
┌─────────┬─────────┬───────┬──────────────────────┐
│BufferBit│ PoolIdx │ Flags │        Slot          │
│ (1 bit) │ (5 bits)│(2 bit)│      (24 bits)       │
└─────────┴─────────┴───────┴──────────────────────┘
 31        30    26  25   24  23                  0
```

| Field | Bits | Range | Purpose |
|-------|------|-------|---------|
| BufferBit | 1 | 0-1 | Which buffer contains data |
| PoolIdx | 5 | 0-30 (31 reserved) | Pool validation |
| Flags | 2 | 0-3 | ADD-PATH support |
| Slot | 24 | 0-16M | Entry index |

**Trade-off:** Max pools reduced from 63 to 31. Sufficient for BGP use.

**Capacity requirement:** Must handle 1.2M+ prefixes (IPv4 DFZ).
- 24-bit slot = 16.7M max entries ✅
- Initial buffer sizing: ~6MB for 1.2M × 5 bytes avg NLRI

## API Changes

### New Handle Methods

| Method | Purpose |
|--------|---------|
| `BufferBit() uint32` | Extract buffer bit (0 or 1) |
| `WithBufferBit(bit uint32) Handle` | Create handle with different buffer bit |

### New Pool Methods

| Method | Purpose |
|--------|---------|
| `AddRef(h Handle)` | Increment refcount, share handle between owners |
| `GetBySlot(slot uint32) ([]byte, error)` | Get data by normalized slot (auto-select buffer) |
| `ReleaseBySlot(slot uint32) error` | Release by normalized slot |
| `MigrateBatch(batchSize int) bool` | Migrate batch of slots, return true when done |
| `StartCompaction()` | Begin incremental compaction |

### Modified Pool Structure

| Field | Type | Purpose |
|-------|------|---------|
| `buffers` | `[2]buffer` | Double buffer for non-blocking compaction |
| `currentBit` | `uint32` | Which buffer is current (0 or 1) |
| `state` | `PoolState` | Normal or Compacting |
| `compactCursor` | `uint32` | Migration progress |

### Slot Structure Change

| Field | Type | Purpose |
|-------|------|---------|
| `offsets` | `[2]uint32` | Offset in EACH buffer |
| `length` | `uint16` | Data length |
| `refCount` | `int32` | Reference count |
| `dead` | `bool` | Marked for removal |

### Buffer Structure (new)

| Field | Type | Purpose |
|-------|------|---------|
| `data` | `[]byte` | Buffer data |
| `pos` | `int` | Write cursor |
| `refCount` | `atomic.Int32` | Handles pointing to this buffer |

## 🧪 TDD Test Plan

### Unit Tests - Handle Layout

| Test | File | Validates |
|------|------|-----------|
| `TestHandle_BufferBit` | `handle_test.go` | BufferBit extraction |
| `TestHandle_HybridLayout` | `handle_test.go` | All fields encode/decode correctly |
| `TestHandle_WithBufferBit` | `handle_test.go` | Buffer bit manipulation |
| `TestInvalidHandle_NewLayout` | `handle_test.go` | InvalidHandle detection with 5-bit poolIdx |

### Unit Tests - Pool Methods

| Test | File | Validates |
|------|------|-----------|
| `TestPool_AddRef` | `pool_test.go` | Share handles, refCount increments |
| `TestPool_GetBySlot` | `pool_test.go` | Normalized access works |
| `TestPool_ReleaseBySlot` | `pool_test.go` | Normalized release works |
| `TestPool_DoubleBuffer` | `pool_test.go` | Data in correct buffer |

### Unit Tests - Incremental Compaction

| Test | File | Validates |
|------|------|-----------|
| `TestPool_MigrateBatch` | `compaction_test.go` | Batch migration |
| `TestPool_BothHandlesValid` | `compaction_test.go` | Old and new handles work during compaction |
| `TestPool_OldBufferRelease` | `compaction_test.go` | Old buffer freed when refCount=0 |
| `TestPool_CompactionInterrupt` | `compaction_test.go` | Compaction pauses on activity |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Pool idx | 0-30 | 30 | N/A | 31 (reserved) |
| BufferBit | 0-1 | 1 | N/A | N/A |
| Slot | 0-0xFFFFFF | 16,777,215 | N/A | 0x1000000 |
| Flags | 0-3 | 3 | N/A | 4 |
| Data length | 0-65535 | 65535 | N/A | 65536 |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/pool/handle.go` | New hybrid bit layout with BufferBit |
| `internal/pool/handle_test.go` | Tests for new layout, boundary tests |
| `internal/pool/pool.go` | Double-buffer, AddRef, GetBySlot, ReleaseBySlot, MigrateBatch |
| `internal/pool/pool_test.go` | Tests for new methods |
| `internal/pool/compaction_test.go` | Tests for incremental compaction |
| `internal/pool/scheduler.go` | Call MigrateBatch() instead of Compact() |

## Implementation Steps

### Phase 0: Completed (Previous Session)

1. ✅ **Deleted incomplete `internal/pool/`** - Removed double-buffer design that never implemented compaction

2. ✅ **Restored delete-pool to `internal/pool/`** - 13 files:
   - pool.go, handle.go, scheduler.go
   - validate_debug.go, validate_release.go
   - pool_test.go, handle_test.go, scheduler_test.go
   - compaction_test.go, metrics_test.go, shutdown_test.go
   - benchmark_test.go, debug_test.go

3. ✅ **Created `attributes.go`** - Global pools:
   - `pool.Attributes` with idx=0, 1MB capacity
   - `pool.NLRI` with idx=1, 256KB capacity

4. ✅ **Updated callers for error handling:**
   - `nlriset.go` - Handle Get() errors, ignore Release() errors
   - `familyrib.go` - Handle Get() errors, ignore Release() errors

5. ✅ **Added logging for corruption detection** - slogutil.Logger("storage")

6. ✅ **Renamed `Valid()` → `IsValid()`** - User preference

7. ✅ **Fixed index corruption bug in nlriset.go Remove()** - When Get() fails for last element during swap, iterate index to find and remove stale entry

**Current state:** Single-buffer pool with stop-the-world Compact(). Tests pass.

---

### Phase 1: Handle Layout Change

1. **Write handle layout tests** - Tests for BufferBit, 5-bit poolIdx, hybrid layout
   → **Review:** Boundary tests for poolIdx 30 (valid) and 31 (invalid)?

2. **Run tests** - Verify FAIL

3. **Update handle.go constants:**
   - Change poolIdx mask from 0x3F to 0x1F (5 bits)
   - Change shift from 26 to 26 (same position but 5 bits)
   - Add BufferBit at bit 31
   - Update InvalidHandle for new layout

4. **Run tests** - Verify PASS
   → **Review:** Existing tests still pass with new layout?

### Phase 2: Pool Structure Change

5. **Write double-buffer tests** - Test data written to correct buffer

6. **Run tests** - Verify FAIL

7. **Update pool.go structure:**
   - Change `data []byte` to `buffers [2]buffer`
   - Add `currentBit`, `state`, `compactCursor`
   - Change slot to have `offsets [2]uint32`

8. **Update Intern()** - Write to `buffers[currentBit]`, track buffer refCount

9. **Update Get()** - Read from buffer indicated by handle's bufferBit

10. **Update Release()** - Decrement buffer refCount

11. **Run tests** - Verify PASS

### Phase 3: New Methods

12. **Write AddRef tests**

13. **Implement AddRef()** - Increment slot and buffer refCounts

14. **Write GetBySlot/ReleaseBySlot tests**

15. **Implement GetBySlot()** - Auto-select buffer based on compaction state

16. **Implement ReleaseBySlot()** - Auto-select buffer based on compaction state

17. **Run tests** - Verify PASS

### Phase 4: Incremental Compaction

18. **Write MigrateBatch tests** - Test batch migration, both handles valid

19. **Run tests** - Verify FAIL

20. **Implement startCompaction()** - Flip currentBit, allocate new buffer

21. **Implement MigrateBatch()** - Copy slots in batches, update index

22. **Implement finishCompaction()** - Free old buffer when refCount=0

23. **Update Scheduler** - Call MigrateBatch() instead of Compact()

24. **Run tests** - Verify PASS

### Phase 5: Verify

25. **Run all tests** - `make test`

26. **Run lint** - `make lint`

27. **Run functional** - `make functional`

## Global Pool Configuration

| Pool | idx | Initial Size | Rationale |
|------|-----|--------------|-----------|
| Attributes | 0 | 1MB | ~10K unique attr sets × ~100 bytes |
| NLRI | 1 | 6MB | 1.2M prefixes × ~5 bytes avg |

## Compaction Behavior

| Aspect | Behavior |
|--------|----------|
| Trigger | Scheduler detects high dead ratio + idle pool |
| Migration | MigrateBatch() processes N slots per tick |
| Pause | Activity detected → pause until idle |
| Old buffer | Freed when refCount reaches 0 |
| Handle validity | Both old and new handles valid during compaction |

## Error Handling Strategy

For `Get()` errors (should not happen with valid handles): Log error and skip the entry.

For `Release()` errors (cleanup, best-effort): Ignore error during cleanup paths.

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
- [x] `pool-architecture.md` updated after implementation

### Completion
- [ ] Spec moved to `docs/plan/done/`

## Implementation Summary

### What Was Implemented

**Phase 0 (completed in previous session):**

1. **Deleted incomplete `internal/pool/`** - Removed double-buffer design that never implemented compaction

2. **Restored delete-pool to `internal/pool/`** - 13 files:
   - `pool.go`, `handle.go`, `scheduler.go`
   - `validate_debug.go`, `validate_release.go`
   - `pool_test.go`, `handle_test.go`, `scheduler_test.go`
   - `compaction_test.go`, `metrics_test.go`, `shutdown_test.go`
   - `benchmark_test.go`, `debug_test.go`

3. **Created `attributes.go`** - Global pools:
   - `pool.Attributes` with idx=0, 1MB capacity
   - `pool.NLRI` with idx=1, 256KB capacity

4. **Updated callers for error handling:**
   - `nlriset.go` - Handle Get() errors, ignore Release() errors
   - `familyrib.go` - Handle Get() errors, ignore Release() errors

5. **Added logging for corruption detection** - `slogutil.Logger("storage")`

6. **Renamed `Valid()` → `IsValid()`** - User preference

7. **Fixed index corruption bug in nlriset.go Remove()** - When Get() fails for last element during swap, iterate index to find and remove stale entry

**Handle Layout:**
- Hybrid design: `bufferBit(1) | poolIdx(5) | flags(2) | slot(24)`
- InvalidHandle = 0xFFFFFFFF (poolIdx=31)
- 5-bit poolIdx allows 31 pools (sufficient for BGP)

**New Pool Methods:**
- `AddRef(h Handle) error` - Share handles between owners
- `GetBySlot(slot uint32) ([]byte, error)` - Normalized slot access
- `ReleaseBySlot(slot uint32) error` - Normalized slot release
- `StartCompaction()` - Begin incremental compaction
- `MigrateBatch(batchSize int) bool` - Non-blocking batch migration
- `CheckOldBufferRelease()` - Free old buffer when refCount=0

**Test Coverage:**
- 68 tests covering handle encoding, pool operations, compaction, concurrency
- Race-detector clean
- Boundary tests for all numeric fields

### Bugs Found/Fixed

1. **Index corruption in nlriset.go Remove()** - When Get() fails for last element during swap-remove, the index map has stale entry pointing to wrong slot. Fix: iterate index to find and delete stale key.

2. **Architecture doc mismatch** - `pool-architecture.md` described MSB-only design that was never implemented. Updated doc to match hybrid layout.

### Deviations from Plan

1. **NLRI pool size** - Spec said 6MB, implementation uses 256KB. The 6MB was based on full DFZ storage; 256KB is sufficient for initial allocation with growth.

2. **WriteTo not implemented** - Documented in architecture but not needed. Callers use `Get()` then write. Removed from doc.
