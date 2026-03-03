# Pool Architecture Critical Review

Issues identified during design review. Track resolution status here.

---

## Critical Bugs (Must Fix Before Implementation)

### 1. Hash Collision Handling

**Status:** ✅ Fixed

**Problem:** Current design uses hash as map key without verifying actual data equality. Two different byte sequences with the same 64-bit hash would be incorrectly deduplicated, causing data corruption.

**Current (broken):**
```go
index map[uint64]Handle  // hash → handle

if h, ok := p.index[hash]; ok {
    slot.refCount++  // BUG: assumes hash match = data match
    return h
}
```

**Fix required:**
```go
func (p *Pool) Intern(data []byte) Handle {
    hash := hashBytes(data)

    if h, ok := p.index[hash]; ok {
        slot := &p.slots[h]
        if !slot.dead && slot.refCount > 0 {
            existing := p.getSlotData(slot)
            if bytes.Equal(existing, data) {  // VERIFY equality
                slot.refCount++
                return h
            }
            // Hash collision - fall through to create new entry
            // Need collision handling strategy (see below)
        }
    }
    // ... create new entry
}
```

**Collision handling options:**
- **(a)** Linear probing: `index[hash] → index[hash+1] → ...`
- **(b)** Chaining: `map[uint64][]Handle`
- **(c)** Larger hash (128-bit) to reduce collision probability

**Recommendation:** Option (b) chaining is simplest. Collisions are rare with 64-bit hash, so chain length will be ~1 in practice.

**Resolution:** Changed to `map[string]Handle` with `unsafe.String` pointing directly into buffer memory. Zero-copy keys, no hash collisions. Index is rebuilt after buffer reallocation to ensure keys point to current memory (see "Buffer Growth and Index Rebuild" in POOL_ARCHITECTURE.md).

**Severity:** CRITICAL - Data corruption

---

### 2. Buffer Growth During Compaction

**Status:** ✅ Fixed

**Problem:** If `ensureCapacity()` reallocates buffer during compaction, already-migrated entries have offsets pointing to the old (now freed) new-buffer.

**Resolution:** `ensureCapacity()` copies existing data to new buffer, preserving all offsets.

**Scenario:**
```
1. Start compaction: oldBuffer = current, buffer = new (1MB)
2. Migrate entry A to buffer at offset 0
3. Intern() large data, buffer needs 2MB
4. ensureCapacity() does: buffer = make([]byte, 2MB)
5. Entry A's offset 0 now points into old 1MB buffer (freed)
6. Get(entryA) returns garbage
```

**Fix required:**
```go
func (p *Pool) ensureCapacity(needed int) {
    required := p.bufferPos + needed
    if required <= cap(p.buffer) {
        // Have capacity, just extend length if needed
        if required > len(p.buffer) {
            p.buffer = p.buffer[:required]
        }
        return
    }

    // Need to grow - allocate new and COPY existing data
    newCap := cap(p.buffer) * 2
    if newCap < required {
        newCap = required
    }

    newBuf := make([]byte, newCap)
    copy(newBuf, p.buffer[:p.bufferPos])  // Preserve existing data
    p.buffer = newBuf

    // Offsets remain valid because data is at same positions in new buffer
}
```

**Key insight:** Copy existing data to new buffer so offsets remain valid.

**Severity:** CRITICAL - Data corruption

---

## Medium Priority Issues

### 3. Scheduler Starvation

**Status:** ✅ Fixed

**Problem:** Scheduler iterates pools in fixed order. If pool[0] always needs compaction, pools[1..N] never get compacted.

**Current (unfair):**
```go
for _, p := range s.pools {
    if p.shouldCompact() {
        s.activePool = p
        return
    }
}
```

**Fix required:**
```go
type CompactionScheduler struct {
    pools     []*Pool
    lastIndex int  // round-robin cursor
    // ...
}

func (s *CompactionScheduler) findPoolToCompact() *Pool {
    n := len(s.pools)
    for i := 0; i < n; i++ {
        idx := (s.lastIndex + 1 + i) % n
        if s.pools[idx].shouldCompact() {
            s.lastIndex = idx
            return s.pools[idx]
        }
    }
    return nil
}
```

**Resolution:** Scheduler now uses round-robin with `lastIndex` cursor.

**Severity:** MEDIUM - Fairness/correctness

---

### 4. Debug Validation for Handles

**Status:** ❌ Not fixed

**Problem:** Invalid handles (out of bounds, dead slots) cause panic or silent corruption.

**Fix required:**
```go
// validateHandle panics on invalid handle (debug builds only)
func (p *Pool) validateHandle(h Handle, op string) {
    if h == InvalidHandle {
        panic(fmt.Sprintf("pool.%s: InvalidHandle", op))
    }
    if int(h) >= len(p.slots) {
        panic(fmt.Sprintf("pool.%s: handle %d out of range (len=%d)", op, h, len(p.slots)))
    }
}

// For release builds, use build tags to make this a no-op
// +build debug
func (p *Pool) validateHandle(h Handle, op string) { ... }

// +build !debug
func (p *Pool) validateHandle(h Handle, op string) {}
```

**Usage:**
```go
func (p *Pool) Get(h Handle) []byte {
    p.validateHandle(h, "Get")
    // ...
}

func (p *Pool) Release(h Handle) {
    p.validateHandle(h, "Release")
    // ...
}
```

**Severity:** MEDIUM - Debugging/robustness

---

### 5. Metrics and Observability

**Status:** ❌ Not implemented

**Problem:** No way to monitor pool health in production.

**Required metrics:**
```go
type PoolMetrics struct {
    // Size metrics
    BufferBytes     int64  // current buffer size
    LiveBytes       int64  // bytes in live entries
    DeadBytes       int64  // bytes in dead entries (reclaimable)

    // Count metrics
    TotalSlots      int32  // total slots allocated
    LiveSlots       int32  // slots with refCount > 0
    DeadSlots       int32  // slots marked dead
    FreeSlots       int32  // slots in freelist

    // Operation metrics
    InternCount     int64  // total Intern() calls
    InternHitCount  int64  // Intern() that found existing (dedup success)
    GetCount        int64  // total Get() calls
    ReleaseCount    int64  // total Release() calls

    // Compaction metrics
    CompactionCount       int64         // completed compactions
    CompactionBytesFreed  int64         // total bytes reclaimed
    LastCompactionTime    time.Time     // when last compaction finished
    LastCompactionDuration time.Duration // how long it took

    // Collision metrics (if using chaining)
    MaxChainLength  int32  // longest collision chain
}

func (p *Pool) Metrics() PoolMetrics {
    p.mu.RLock()
    defer p.mu.RUnlock()
    return PoolMetrics{
        // ... populate from pool state
    }
}
```

**Severity:** MEDIUM - Operational readiness

---

### 6. Graceful Shutdown

**Status:** ❌ Not implemented

**Problem:** No clean way to stop pool operations and release resources.

**Required:**
```go
func (p *Pool) Shutdown() {
    p.mu.Lock()
    defer p.mu.Unlock()

    // Mark as shutting down (reject new operations)
    p.shuttingDown = true

    // Abort any in-progress compaction
    if p.state == PoolCompacting {
        p.oldBuffer = nil
        p.state = PoolNormal
    }

    // Clear all data
    p.buffer = nil
    p.slots = nil
    p.index = nil
    p.freeList = nil
}

func (p *Pool) checkShutdown() error {
    if p.shuttingDown {
        return ErrPoolShutdown
    }
    return nil
}
```

**Severity:** MEDIUM - Correctness

---

## Low Priority Issues (Document or Defer)

### 7. Slice Lifetime and GC

**Status:** ⚠️ Document only

**Concern:** Callers holding Get() slices for long periods prevent old buffer from being garbage collected.

**Analysis:** This is safe (no use-after-free) due to Go's GC. The slice keeps the underlying array alive. But it can cause memory to be retained longer than expected.

**Action:** Document in API:
```go
// Get returns the data for handle h.
//
// The returned slice is valid until the next pool modification.
// Callers should not hold the slice across operations.
// For long-term storage, copy the data:
//
//     data := pool.Get(h)
//     stored := make([]byte, len(data))
//     copy(stored, data)
//
func (p *Pool) Get(h Handle) []byte
```

**Severity:** LOW - Documentation

---

### 8. Activity Detection Semantics

**Status:** ⚠️ Document only

**Observation:** Only Intern() updates activity timestamp. Get() and Release() don't.

**Analysis:** This is correct because:
- Get() is safe during compaction (read-only)
- Release() is safe during compaction (only marks dead)
- Only Intern() conflicts with migration (adds new data)

**Action:** Document the reasoning in code comments:
```go
// touchActivity marks the pool as recently active.
// Only called from Intern() because Get() and Release()
// are safe to run during compaction.
func (p *Pool) touchActivity() {
    p.lastActivityNano.Store(time.Now().UnixNano())
}
```

**Severity:** LOW - Documentation

---

### 9. Cross-Pool Release Coupling

**Status:** ⚠️ Consider refactoring

**Concern:** NLRI.Release() calls AS_PATH.Release() while holding NLRI lock. If AS_PATH pool is contended, NLRI operations are blocked.

**Current:**
```go
func (p *NLRIPool) Release(h Handle) {
    p.mu.Lock()
    defer p.mu.Unlock()

    slot := &p.slots[h]
    slot.refCount--
    if slot.refCount <= 0 {
        slot.dead = true
        p.asPathPool.Release(slot.asPathRef)  // Holds p.mu, acquires asPathPool.mu
    }
}
```

**Alternative (deferred cascade):**
```go
func (p *NLRIPool) Release(h Handle) (cascades []CascadeRelease) {
    p.mu.Lock()
    defer p.mu.Unlock()

    slot := &p.slots[h]
    slot.refCount--
    if slot.refCount <= 0 {
        slot.dead = true
        cascades = append(cascades, CascadeRelease{
            Pool:   p.asPathPool,
            Handle: slot.asPathRef,
        })
    }
    return cascades
}

// Caller processes cascades outside lock:
for _, c := range nlriPool.Release(h) {
    c.Pool.Release(c.Handle)
}
```

**Action:** Implement simple version first. Refactor if contention is observed.

**Severity:** LOW - Performance optimization

---

### 10. Lock Contention Under High Load

**Status:** ⚠️ Monitor, optimize if needed

**Concern:** Single mutex per pool may bottleneck under high peer count.

**Potential optimizations:**
1. **Sharded pools:** 16 sub-pools by hash prefix
2. **Lock-free refCount:** atomic.Int32 for refCount operations
3. **Batch operations:** Intern multiple entries per lock

**Action:** Implement simple version first. Add metrics for lock wait time. Optimize if p99 latency degrades.

**Severity:** LOW - Performance optimization

---

## Resolution Checklist

### Before Implementation (Design Phase)
- [x] Fix hash collision handling (Issue #1) - ✅ Using `unsafe.String` into buffer
- [x] Fix buffer growth during compaction (Issue #2) - ✅ `ensureCapacity` copies data
- [x] Implement round-robin scheduler (Issue #3) - ✅ Added `lastIndex` cursor

### Implementation Phase (Current)
- [ ] Create `internal/component/bgp/attrpool/handle.go` - Handle type (30m)
- [ ] Create `internal/component/bgp/attrpool/pool.go` - Core Pool type (2-3h)
- [ ] Create `internal/component/bgp/attrpool/pool_test.go` - Core tests (1h)
- [ ] Create `internal/component/bgp/attrpool/compaction.go` - Compaction logic (1h)
- [ ] Create `internal/component/bgp/attrpool/scheduler.go` - CompactionScheduler (45m)

### Before Testing
- [ ] Add debug handle validation (Issue #4) - debug.go/debug_release.go (30m)

### Before Production
- [ ] Add metrics (Issue #5) - metrics.go (1h)
- [ ] Implement graceful shutdown (Issue #6) - Shutdown() method (45m)
- [ ] Document slice lifetime (Issue #7) - Get() godoc (10m)
- [ ] Document activity semantics (Issue #8) - touchActivity() comment (5m)

### If Performance Issues Observed
- [ ] Consider cross-pool release refactoring (Issue #9)
- [ ] Consider pool sharding (Issue #10)

**Detailed plan:** `docs/plan/wip-pool-completion.md`

---

**Created:** 2025-12-19
**Last Updated:** 2025-12-19
