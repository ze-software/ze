# Pool Implementation Completion

**Status:** 🔄 Active
**Priority:** 🔴 High (blocks wire format work)
**Estimated Effort:** 8-10 hours

**TDD REQUIREMENT:** Tests MUST be written and fail BEFORE implementation.

---

## Overview

Complete the pool architecture implementation before proceeding to wire format work. The pool provides zero-copy byte deduplication for attributes and NLRI, critical for memory efficiency with large RIBs.

**Design docs:**
- `docs/architecture/POOL_ARCHITECTURE.md` - Full design specification
- `docs/architecture/POOL_ARCHITECTURE_REVIEW.md` - Issues tracker

---

## TDD Workflow Reminder

```
┌─────────────────────────────────────────────────────────────────┐
│  FOR EACH FEATURE:                                              │
│  1. Write test (with VALIDATES/PREVENTS docs) → 2. Run → FAIL  │
│  3. Write implementation → 4. Run → PASS → 5. Next feature     │
└─────────────────────────────────────────────────────────────────┘
```

---

## Phase 1: Handle Type

### 1.1 Test Specification (WRITE FIRST)

**File:** `internal/pool/handle_test.go`

```go
// TestHandleValid verifies that Valid() correctly identifies valid handles.
//
// VALIDATES: Handle validity check works correctly.
//
// PREVENTS: Invalid handles being used in pool operations, causing
// out-of-bounds access or data corruption.
func TestHandleValid(t *testing.T) {
    tests := []struct {
        name  string
        h     Handle
        valid bool
    }{
        {"zero is valid", Handle(0), true},
        {"positive is valid", Handle(100), true},
        {"max-1 is valid", Handle(0xFFFFFFFE), true},
        {"InvalidHandle is not valid", InvalidHandle, false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            assert.Equal(t, tt.valid, tt.h.Valid())
        })
    }
}

// TestInvalidHandleConstant verifies InvalidHandle has expected value.
//
// VALIDATES: Sentinel value is correct.
//
// PREVENTS: Accidental collision with valid handle values.
func TestInvalidHandleConstant(t *testing.T) {
    assert.Equal(t, Handle(0xFFFFFFFF), InvalidHandle)
}
```

### 1.2 Implementation (WRITE AFTER TESTS FAIL)

**File:** `internal/pool/handle.go`

```go
package pool

type Handle uint32

const InvalidHandle Handle = 0xFFFFFFFF

func (h Handle) Valid() bool {
    return h != InvalidHandle
}
```

---

## Phase 2: Core Pool - Intern/Get/Release

### 2.1 Test Specification (WRITE FIRST)

**File:** `internal/pool/pool_test.go`

```go
// TestInternDeduplication verifies that interning identical data returns
// the same handle and increments reference count.
//
// VALIDATES: Memory efficiency through deduplication.
//
// PREVENTS: Memory bloat - without deduplication, 1M routes sharing the
// same AS_PATH would store 1M copies instead of 1 with refCount=1M.
func TestInternDeduplication(t *testing.T) {
    p := New(1024)

    h1 := p.Intern([]byte("hello"))
    h2 := p.Intern([]byte("hello"))

    require.Equal(t, h1, h2, "identical data must return same handle")
    require.True(t, h1.Valid(), "handle must be valid")
}

// TestInternUnique verifies that different data gets different handles.
//
// VALIDATES: Correct storage of distinct entries.
//
// PREVENTS: Data corruption where different data incorrectly shares
// the same handle, returning wrong data on Get().
func TestInternUnique(t *testing.T) {
    p := New(1024)

    h1 := p.Intern([]byte("hello"))
    h2 := p.Intern([]byte("world"))

    require.NotEqual(t, h1, h2, "different data must return different handles")
}

// TestGetReturnsCorrectData verifies Get() returns the interned data.
//
// VALIDATES: Data integrity through intern/get cycle.
//
// PREVENTS: Data corruption or loss during storage.
func TestGetReturnsCorrectData(t *testing.T) {
    p := New(1024)
    data := []byte("test data 12345")

    h := p.Intern(data)
    got := p.Get(h)

    require.Equal(t, data, got, "Get must return original data")
}

// TestReleaseDecrementsRefCount verifies Release() decrements reference count.
//
// VALIDATES: Reference counting correctness.
//
// PREVENTS: Memory leaks from entries never being freed, or
// use-after-free from premature deletion.
func TestReleaseDecrementsRefCount(t *testing.T) {
    p := New(1024)

    // Intern twice (refCount = 2)
    h := p.Intern([]byte("data"))
    _ = p.Intern([]byte("data"))

    // Release once (refCount = 1)
    p.Release(h)

    // Data should still be accessible
    got := p.Get(h)
    require.Equal(t, []byte("data"), got, "data must survive partial release")
}

// TestReleaseToZeroMarksDead verifies that releasing to refCount=0 marks dead.
//
// VALIDATES: Entry lifecycle management.
//
// PREVENTS: Dead entries remaining live (memory leak) or live entries
// being marked dead (use-after-free).
func TestReleaseToZeroMarksDead(t *testing.T) {
    p := New(1024)

    h := p.Intern([]byte("data"))
    p.Release(h)

    // After release to zero, entry should be dead
    // New intern of same data should get new handle (or reuse slot)
    h2 := p.Intern([]byte("data"))
    // Either same slot reused or new slot - both are valid
    require.True(t, h2.Valid())
}

// TestInternEmpty verifies empty byte slice handling.
//
// VALIDATES: Edge case - empty data is valid input.
//
// PREVENTS: Panic or corruption on empty input.
func TestInternEmpty(t *testing.T) {
    p := New(1024)

    h := p.Intern([]byte{})
    require.True(t, h.Valid())

    got := p.Get(h)
    require.Equal(t, []byte{}, got)
}

// TestConcurrentIntern verifies thread-safety of concurrent Intern calls.
//
// VALIDATES: Thread-safety under concurrent access.
//
// PREVENTS: Data races, corruption, panics under load from multiple
// BGP peers interning routes simultaneously.
func TestConcurrentIntern(t *testing.T) {
    p := New(1024 * 1024)
    var wg sync.WaitGroup

    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            for j := 0; j < 1000; j++ {
                data := []byte(fmt.Sprintf("data-%d-%d", id, j))
                h := p.Intern(data)
                got := p.Get(h)
                require.Equal(t, data, got)
            }
        }(i)
    }

    wg.Wait()
}
```

### 2.2 Implementation (WRITE AFTER TESTS FAIL)

**File:** `internal/pool/pool.go`

See `docs/architecture/POOL_ARCHITECTURE.md` for full implementation details.

---

## Phase 3: Compaction

### 3.1 Test Specification (WRITE FIRST)

**File:** `internal/pool/compaction_test.go`

```go
// TestCompactionReclaimsDeadSpace verifies compaction frees dead entry space.
//
// VALIDATES: Memory reclamation through compaction.
//
// PREVENTS: Unbounded memory growth as routes are withdrawn and
// re-announced, leaving dead entries consuming space forever.
func TestCompactionReclaimsDeadSpace(t *testing.T) {
    p := New(1024)

    // Create entries
    h1 := p.Intern([]byte("AAAA"))
    h2 := p.Intern([]byte("BBBB"))
    h3 := p.Intern([]byte("CCCC"))

    // Release middle entry (creates dead space)
    p.Release(h2)

    // Record size before compaction
    before := p.Metrics()

    // Force compaction
    p.Compact()

    // Record size after
    after := p.Metrics()

    require.Less(t, after.DeadBytes, before.DeadBytes,
        "compaction must reduce dead bytes")
}

// TestCompactionPreservesLiveData verifies handles remain valid after compaction.
//
// VALIDATES: Handle stability guarantee across compaction.
//
// PREVENTS: Data corruption where compaction moves data but handles
// still point to old offsets, causing Get() to return garbage.
func TestCompactionPreservesLiveData(t *testing.T) {
    p := New(1024)

    h1 := p.Intern([]byte("AAAA"))
    h2 := p.Intern([]byte("BBBB"))
    h3 := p.Intern([]byte("CCCC"))

    // Release middle entry
    p.Release(h2)

    // Compact
    p.Compact()

    // Remaining handles must still work
    require.Equal(t, []byte("AAAA"), p.Get(h1), "h1 must survive compaction")
    require.Equal(t, []byte("CCCC"), p.Get(h3), "h3 must survive compaction")
}

// TestConcurrentAccessDuringCompaction verifies operations work during compaction.
//
// VALIDATES: Availability during maintenance operations.
//
// PREVENTS: BGP session stalls during compaction, causing holdtime
// expiry and session drops.
func TestConcurrentAccessDuringCompaction(t *testing.T) {
    p := New(1024 * 1024)

    // Pre-populate
    handles := make([]Handle, 1000)
    for i := range handles {
        handles[i] = p.Intern([]byte(fmt.Sprintf("data-%d", i)))
    }

    // Release half to create dead space
    for i := 0; i < len(handles); i += 2 {
        p.Release(handles[i])
    }

    var wg sync.WaitGroup

    // Start compaction in background
    wg.Add(1)
    go func() {
        defer wg.Done()
        p.Compact()
    }()

    // Concurrent reads during compaction
    for i := 1; i < len(handles); i += 2 {
        wg.Add(1)
        go func(h Handle, expected string) {
            defer wg.Done()
            got := p.Get(h)
            require.Equal(t, []byte(expected), got)
        }(handles[i], fmt.Sprintf("data-%d", i))
    }

    wg.Wait()
}
```

### 3.2 Implementation (WRITE AFTER TESTS FAIL)

**File:** `internal/pool/compaction.go`

---

## Phase 4: Scheduler

### 4.1 Test Specification (WRITE FIRST)

**File:** `internal/pool/scheduler_test.go`

```go
// TestSchedulerRoundRobin verifies fair pool selection for compaction.
//
// VALIDATES: Fairness in pool compaction scheduling.
//
// PREVENTS: Pool starvation where one busy pool always gets compacted
// while others grow unbounded.
func TestSchedulerRoundRobin(t *testing.T) {
    pools := make([]*Pool, 3)
    for i := range pools {
        pools[i] = New(1024)
    }

    s := NewScheduler(pools, 0)

    // Mark all as needing compaction
    for _, p := range pools {
        // ... make each pool need compaction
    }

    // Should cycle through pools fairly
    selected := make(map[*Pool]int)
    for i := 0; i < 9; i++ {
        p := s.findPoolToCompact()
        if p != nil {
            selected[p]++
        }
    }

    // Each pool should be selected roughly equally
    for _, p := range pools {
        require.GreaterOrEqual(t, selected[p], 2)
    }
}

// TestSchedulerRespectsQuietPeriod verifies compaction waits for inactivity.
//
// VALIDATES: Compaction doesn't interfere with active operations.
//
// PREVENTS: Compaction running during high activity, causing lock
// contention and increased latency.
func TestSchedulerRespectsQuietPeriod(t *testing.T) {
    p := New(1024)
    s := NewScheduler([]*Pool{p}, 100*time.Millisecond)

    // Make pool need compaction but recently active
    p.Intern([]byte("data"))

    // Should not compact immediately after activity
    require.Nil(t, s.findPoolToCompact())

    // Wait for quiet period
    time.Sleep(150 * time.Millisecond)

    // Now should be eligible
    // (only if it needs compaction)
}
```

### 4.2 Implementation (WRITE AFTER TESTS FAIL)

**File:** `internal/pool/scheduler.go`

---

## Phase 5: Debug Validation (Issue #4)

### 5.1 Test Specification (WRITE FIRST)

**File:** `internal/pool/debug_test.go`

```go
//go:build debug

// TestDebugValidationCatchesInvalidHandle verifies debug build catches bad handles.
//
// VALIDATES: Debug builds catch programming errors early.
//
// PREVENTS: Silent corruption in production from invalid handle usage.
// Debug builds should panic to catch errors during development.
func TestDebugValidationCatchesInvalidHandle(t *testing.T) {
    p := New(1024)

    require.Panics(t, func() {
        p.Get(InvalidHandle)
    }, "Get(InvalidHandle) must panic in debug build")
}

// TestDebugValidationCatchesOutOfBounds verifies debug build catches OOB.
//
// VALIDATES: Bounds checking in debug mode.
//
// PREVENTS: Buffer overflow exploits from malformed handles.
func TestDebugValidationCatchesOutOfBounds(t *testing.T) {
    p := New(1024)

    require.Panics(t, func() {
        p.Get(Handle(999999))
    }, "Get(OOB handle) must panic in debug build")
}

// TestDebugValidationCatchesDeadSlot verifies debug build catches dead access.
//
// VALIDATES: Use-after-free detection in debug mode.
//
// PREVENTS: Accessing released entries that may have been reused.
func TestDebugValidationCatchesDeadSlot(t *testing.T) {
    p := New(1024)
    h := p.Intern([]byte("data"))
    p.Release(h)

    require.Panics(t, func() {
        p.Get(h)
    }, "Get(released handle) must panic in debug build")
}
```

### 5.2 Implementation (WRITE AFTER TESTS FAIL)

**Files:** `internal/pool/debug.go`, `internal/pool/debug_release.go`

---

## Phase 6: Metrics (Issue #5)

### 6.1 Test Specification (WRITE FIRST)

**File:** `internal/pool/metrics_test.go`

```go
// TestMetricsAccuracy verifies metrics reflect actual pool state.
//
// VALIDATES: Observability correctness.
//
// PREVENTS: Misleading metrics causing incorrect capacity planning
// or missed memory issues in production.
func TestMetricsAccuracy(t *testing.T) {
    p := New(1024)

    h1 := p.Intern([]byte("AAAA"))
    h2 := p.Intern([]byte("BBBB"))
    p.Release(h2)

    m := p.Metrics()

    require.Equal(t, int32(2), m.TotalSlots)
    require.Equal(t, int32(1), m.LiveSlots)
    require.Equal(t, int32(1), m.DeadSlots)
    require.Equal(t, int64(4), m.LiveBytes)
    require.Equal(t, int64(4), m.DeadBytes)
}

// TestMetricsDeduplicationRate verifies dedup rate calculation.
//
// VALIDATES: Deduplication effectiveness metric.
//
// PREVENTS: Incorrect efficiency reporting, missing optimization opportunities.
func TestMetricsDeduplicationRate(t *testing.T) {
    p := New(1024)

    // 3 interns, 2 hits (same data)
    p.Intern([]byte("data"))
    p.Intern([]byte("data"))
    p.Intern([]byte("data"))

    m := p.Metrics()

    require.Equal(t, int64(3), m.InternTotal)
    require.Equal(t, int64(2), m.InternHits)
    require.InDelta(t, 0.666, m.DeduplicationRate, 0.01)
}
```

### 6.2 Implementation (WRITE AFTER TESTS FAIL)

**File:** `internal/pool/metrics.go`

---

## Phase 7: Graceful Shutdown (Issue #6)

### 7.1 Test Specification (WRITE FIRST)

**File:** `internal/pool/shutdown_test.go`

```go
// TestShutdownRejectsNewOperations verifies shutdown blocks new work.
//
// VALIDATES: Clean shutdown semantics.
//
// PREVENTS: Operations starting during shutdown, causing races or panics.
func TestShutdownRejectsNewOperations(t *testing.T) {
    p := New(1024)
    p.Shutdown()

    _, err := p.Intern([]byte("data"))
    require.ErrorIs(t, err, ErrPoolShutdown)
}

// TestShutdownIdempotent verifies multiple shutdown calls are safe.
//
// VALIDATES: Idempotent shutdown.
//
// PREVENTS: Double-free or panic on repeated shutdown calls.
func TestShutdownIdempotent(t *testing.T) {
    p := New(1024)

    require.NoError(t, p.Shutdown())
    require.NoError(t, p.Shutdown())
    require.NoError(t, p.Shutdown())
}

// TestShutdownAbortsCompaction verifies in-flight compaction is aborted.
//
// VALIDATES: Clean abort of background work.
//
// PREVENTS: Compaction continuing after shutdown, accessing freed memory.
func TestShutdownAbortsCompaction(t *testing.T) {
    p := New(1024)

    // Start compaction
    h := p.Intern([]byte("data"))
    p.Release(h)

    // Shutdown should abort any in-progress compaction
    p.Shutdown()

    // Pool should be in clean state
}
```

### 7.2 Implementation (WRITE AFTER TESTS FAIL)

Add to `internal/pool/pool.go`

---

## Phase 8: Benchmarks

**File:** `internal/pool/benchmark_test.go`

```go
func BenchmarkInternExisting(b *testing.B) {
    p := New(1024 * 1024)
    p.Intern([]byte("benchmark-data"))

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        p.Intern([]byte("benchmark-data"))
    }
}

func BenchmarkInternNew(b *testing.B) {
    p := New(1024 * 1024)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        p.Intern([]byte(fmt.Sprintf("data-%d", i)))
    }
}

func BenchmarkGet(b *testing.B) {
    p := New(1024)
    h := p.Intern([]byte("benchmark-data"))

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = p.Get(h)
    }
}

func BenchmarkRelease(b *testing.B) {
    p := New(1024 * 1024)
    handles := make([]Handle, b.N)
    for i := range handles {
        handles[i] = p.Intern([]byte(fmt.Sprintf("data-%d", i)))
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        p.Release(handles[i])
    }
}
```

**Targets:**
- `BenchmarkInternExisting`: < 100ns
- `BenchmarkInternNew`: < 500ns
- `BenchmarkGet`: < 50ns
- `BenchmarkRelease`: < 100ns

---

## File Checklist (TDD Order)

```
internal/pool/
├── handle_test.go       # [1] Test first
├── handle.go            # [2] Implement
├── pool_test.go         # [3] Test first
├── pool.go              # [4] Implement
├── compaction_test.go   # [5] Test first
├── compaction.go        # [6] Implement
├── scheduler_test.go    # [7] Test first
├── scheduler.go         # [8] Implement
├── debug_test.go        # [9] Test first
├── debug.go             # [10] Implement
├── debug_release.go     # [10] Implement
├── metrics_test.go      # [11] Test first
├── metrics.go           # [12] Implement
├── shutdown_test.go     # [13] Test first (add to pool.go)
└── benchmark_test.go    # [14] After all features
```

---

## TDD Execution Template

For each phase, follow this template:

```
=== PHASE N: [Feature] ===

Step 1: Write test
File: internal/pool/X_test.go

Step 2: Run test (MUST FAIL)
$ go test -race ./internal/pool/... -v -run TestX
OUTPUT:
[paste failure here]

Step 3: Write implementation
File: internal/pool/X.go

Step 4: Run test (MUST PASS)
$ go test -race ./internal/pool/... -v -run TestX
OUTPUT:
[paste pass here]

=== PHASE N COMPLETE ===
```

---

## Success Criteria

- [ ] All tests written with VALIDATES/PREVENTS documentation
- [ ] Each test failed before implementation
- [ ] All tests pass with race detector
- [ ] Debug build catches invalid handles
- [ ] Metrics available via `pool.Metrics()`
- [ ] Clean shutdown releases resources
- [ ] Benchmarks meet targets
- [ ] No race conditions

---

## Resume Point

**Next action:** Create `internal/pool/handle_test.go` with tests

**TDD order:**
1. Write `handle_test.go` → Run → FAIL
2. Write `handle.go` → Run → PASS
3. Write `pool_test.go` → Run → FAIL
4. Write `pool.go` → Run → PASS
5. Continue through phases...

**Blocks:** All wire format work depends on pool completion

---

**Created:** 2025-12-19
**Last Updated:** 2025-12-19
