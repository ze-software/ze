# Spec: Adj-RIB-Out Memory Profiling

## Status: Not Started

**See:** `docs/architecture/rib-transition.md` for overall architecture direction.

---

## Design Transition Note

This profiling spec should be run **after** Pool + Wire implementation to measure actual savings:

| Scenario | Before Pool+Wire | After Pool+Wire |
|----------|------------------|-----------------|
| Per-route memory | ~250-400 bytes | ~18 bytes + shared pool |
| 1M routes, 100K unique | ~300 MB | ~33 MB |
| Expected savings | - | 90%+ |

**Recommendation:** Run Phase 2 baseline now with current code, then re-run after `spec-pool-handle-migration.md` to measure actual improvement.

---

## Goal

Profile memory usage of adj-rib-out under high route counts to:
1. Establish baseline memory per route
2. Identify optimization opportunities
3. Set memory limits/warnings for production use

## Scenarios to Profile

| Scenario | Routes | Peers | Expected Memory |
|----------|--------|-------|-----------------|
| Small | 1K | 1 | Baseline |
| Medium | 10K | 1 | 10x baseline |
| Large | 100K | 1 | 100x baseline |
| Multi-peer | 10K | 10 | 10x per peer? |
| Full table | 1M | 1 | Production estimate |

## Memory Components per Route

### Current (Before Pool+Wire)

From `internal/rib/route.go`:
```go
type Route struct {
    nlri          nlri.NLRI           // Interface + prefix (~40 bytes)
    nextHop       netip.Addr          // 24 bytes
    attributes    []attribute.Attribute // Slice header + attrs
    asPath        *attribute.ASPath   // Pointer + segments
    wireBytes     []byte              // Cached wire format
    nlriWireBytes []byte              // Cached NLRI bytes
    sourceCtxID   bgpctx.ContextID    // 8 bytes
    refCount      atomic.Int32        // 4 bytes
}
```

Estimated per-route overhead:
- Base struct: ~120 bytes
- NLRI (IPv4/24): ~20 bytes
- Attributes (minimal): ~50 bytes
- Wire cache: ~50-200 bytes
- **Total: ~250-400 bytes/route**

### Target (After Pool+Wire)

From `spec-pool-handle-migration.md`:
```go
type Route struct {
    attrHandle    pool.Handle      // 4 bytes → shared pool data
    nlriHandle    pool.Handle      // 4 bytes → shared pool data
    sourceCtxID   ContextID        // 2 bytes
    tag           RouteTag         // ~8 bytes
    // No parsed attributes
    // No wire cache (pool owns data)
}
```

Estimated per-route overhead:
- Route struct: ~18 bytes
- Pool overhead: amortized across shared routes
- **Total: ~18 bytes/route + shared pool**

### Deduplication Impact

| Routes | Unique Attrs | Current | Target | Savings |
|--------|--------------|---------|--------|---------|
| 1K | 100 | 400 KB | 20 KB + 15 KB pool | 95% |
| 100K | 10K | 40 MB | 2 MB + 1.5 MB pool | 91% |
| 1M | 100K | 400 MB | 18 MB + 15 MB pool | 92% |

## Profiling Approach

### Phase 1: Micro-benchmark

Create benchmark test in `internal/rib/`:

```go
// BenchmarkOutgoingRIBMemory measures memory per route.
func BenchmarkOutgoingRIBMemory(b *testing.B) {
    b.ReportAllocs()

    rib := NewOutgoingRIB()

    for i := 0; i < b.N; i++ {
        prefix := generatePrefix(i)
        route := NewRoute(prefix, nextHop, attrs)
        rib.MarkSent(route)
    }
}
```

### Phase 2: Heap Profile

Add profiling endpoint or test:

```go
func TestAdjRIBOutMemoryProfile(t *testing.T) {
    if testing.Short() {
        t.Skip("memory profiling")
    }

    // Create 100K routes
    rib := NewOutgoingRIB()
    for i := 0; i < 100_000; i++ {
        route := createTestRoute(i)
        rib.MarkSent(route)
    }

    // Force GC and get stats
    runtime.GC()
    var m runtime.MemStats
    runtime.ReadMemStats(&m)

    t.Logf("HeapAlloc: %d MB", m.HeapAlloc/1024/1024)
    t.Logf("HeapObjects: %d", m.HeapObjects)
    t.Logf("Bytes per route: %d", m.HeapAlloc/100_000)

    // Write heap profile
    f, _ := os.Create("adjribout.pprof")
    pprof.WriteHeapProfile(f)
    f.Close()
}
```

### Phase 3: Production Simulation

Test with realistic forwarding scenario:
1. Receive UPDATE with 1000 NLRIs
2. Forward to 10 peers
3. Store in adj-rib-out for each peer
4. Measure: total memory, per-peer overhead

## Optimization Opportunities

### 1. Attribute Deduplication

Routes with same attributes share pointer:
```go
// Current: each route has own attribute slice
route1.attributes = []Attribute{origin, asPath, med}
route2.attributes = []Attribute{origin, asPath, med} // duplicate

// Optimized: shared via intern pool
route1.attributes = attrPool.Intern(attrs)
route2.attributes = attrPool.Intern(attrs) // same pointer
```

**Savings:** For 100K routes with 100 unique attr sets: ~90% reduction in attr memory.

### 2. Wire Cache Control

Option to disable wire cache when memory-constrained:
```go
type OutgoingRIBConfig struct {
    DisableWireCache bool  // Trade CPU for memory
}
```

**Savings:** ~100-200 bytes/route (25-50% reduction).

### 3. Per-Peer Route Limits

Prevent unbounded growth:
```go
type OutgoingRIBConfig struct {
    MaxRoutes int  // 0 = unlimited
}

func (r *OutgoingRIB) MarkSent(route *Route) error {
    if r.maxRoutes > 0 && r.count >= r.maxRoutes {
        return ErrMaxRoutesExceeded
    }
    // ...
}
```

### 4. Lazy Wire Encoding

Don't cache wire bytes until replay needed:
```go
func NewRouteWithoutWireCache(...) *Route {
    // Skip wireBytes and nlriWireBytes
    // Re-encode on replay
}
```

## Implementation Steps

### Step 1: Add Benchmark Test
```bash
# File: internal/rib/outgoing_bench_test.go
go test -bench=BenchmarkOutgoingRIBMemory -benchmem ./internal/rib/...
```

### Step 2: Add Memory Profile Test
```bash
# File: internal/rib/outgoing_profile_test.go
go test -run=TestAdjRIBOutMemoryProfile -v ./internal/rib/...
go tool pprof -http=:8080 adjribout.pprof
```

### Step 3: Document Findings

Update `docs/architecture/POOL_ARCHITECTURE.md` with:
- Measured bytes per route
- Memory formula: `total_mem = routes * peers * bytes_per_route`
- Recommended limits

### Step 4: Implement Optimizations (if needed)

Based on profiling results, prioritize:
1. Attribute deduplication (if attrs dominate)
2. Wire cache control (if cache dominates)
3. Route limits (always useful for safety)

## Success Criteria

- [ ] Benchmark test measures bytes/route accurately
- [ ] Heap profile identifies top memory consumers
- [ ] Memory formula documented
- [ ] Optimization plan prioritized based on data

## Out of Scope

- Full RIB (Loc-RIB) memory profiling
- Incoming RIB (Adj-RIB-In) profiling
- Wire buffer pool profiling (separate spec)

## Commands

```bash
# Run benchmark
go test -bench=Memory -benchmem ./internal/rib/...

# Generate heap profile
go test -run=Profile -memprofile=mem.pprof ./internal/rib/...

# Analyze profile
go tool pprof -top mem.pprof
go tool pprof -http=:8080 mem.pprof
```

---

**Created:** 2026-01-01
