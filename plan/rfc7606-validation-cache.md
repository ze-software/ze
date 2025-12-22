# RFC 7606 Validation Cache Plan

**Status:** 📋 Proposed
**Priority:** 🟢 Low (optimization, not required for correctness)
**Created:** 2025-12-22

---

## Problem Statement

`ValidateUpdateRFC7606()` is called for every UPDATE message received. When multiple UPDATEs share identical path attributes (common in route reflector scenarios or full table dumps), we re-validate the same bytes repeatedly.

**Current call site:** `pkg/reactor/session.go:534`

```go
result := message.ValidateUpdateRFC7606(pathAttrs, hasNLRI, isIBGP, asn4)
```

---

## RFC 7606 Context

Per RFC 7606, validation determines one of four actions:
- `session-reset` - Fatal, terminate session
- `AFI/SAFI disable` - Disable address family
- `treat-as-withdraw` - Treat UPDATE as withdrawal
- `attribute-discard` - Discard malformed attribute, continue

**Key insight:** The validation result depends ONLY on:
1. Path attribute bytes (`pathAttrs`)
2. Session parameters (`hasNLRI`, `isIBGP`, `asn4`)

If two UPDATEs have identical inputs, they produce identical results.

---

## Proposed Solution

Cache validation results keyed by path attributes + session context.

### Cache Key Design

```go
// validationCacheKey uniquely identifies a validation context.
// Uses string (not []byte) because Go maps require comparable keys.
type validationCacheKey struct {
    pathAttrs string // string(pathAttrs) - path attribute bytes
    flags     uint8  // packed: hasNLRI(bit0) | isIBGP(bit1) | asn4(bit2)
}

func makeValidationCacheKey(pathAttrs []byte, hasNLRI, isIBGP, asn4 bool) validationCacheKey {
    var flags uint8
    if hasNLRI {
        flags |= 1
    }
    if isIBGP {
        flags |= 2
    }
    if asn4 {
        flags |= 4
    }
    return validationCacheKey{
        pathAttrs: string(pathAttrs), // safe: creates copy
        flags:     flags,
    }
}
```

### Cache Structure

```go
// ValidationCache caches RFC 7606 validation results.
// Thread-safe for concurrent access from multiple sessions.
type ValidationCache struct {
    cache *lru.Cache[validationCacheKey, *RFC7606ValidationResult]

    // Metrics (atomic for lock-free reads)
    hits   atomic.Uint64
    misses atomic.Uint64
}

func NewValidationCache(size int) (*ValidationCache, error) {
    cache, err := lru.New[validationCacheKey, *RFC7606ValidationResult](size)
    if err != nil {
        return nil, fmt.Errorf("create LRU cache: %w", err)
    }
    return &ValidationCache{cache: cache}, nil
}
```

### Integration

```go
// pkg/reactor/session.go

func (s *Session) validateUpdateRFC7606(body []byte) error {
    // ... parse pathAttrs, hasNLRI ...

    // Check cache first (if enabled)
    if s.validationCache != nil {
        key := makeValidationCacheKey(pathAttrs, hasNLRI, isIBGP, asn4)
        if result, ok := s.validationCache.Get(key); ok {
            return s.handleValidationResult(result)
        }
    }

    // Cache miss - validate
    result := message.ValidateUpdateRFC7606(pathAttrs, hasNLRI, isIBGP, asn4)

    // Store in cache (if enabled)
    if s.validationCache != nil {
        key := makeValidationCacheKey(pathAttrs, hasNLRI, isIBGP, asn4)
        s.validationCache.Add(key, result)
    }

    return s.handleValidationResult(result)
}
```

---

## Implementation Phases

### Phase 0: Measurement (REQUIRED FIRST)

**Before ANY code changes, measure baseline.**

#### Step 1: Add Benchmark

**File:** `pkg/bgp/message/rfc7606_bench_test.go`

```go
package message

import (
    "testing"
)

// BenchmarkValidateUpdateRFC7606 measures validation performance.
func BenchmarkValidateUpdateRFC7606(b *testing.B) {
    // Typical UPDATE with ORIGIN, AS_PATH, NEXT_HOP
    pathAttrs := []byte{
        0x40, 0x01, 0x01, 0x00,                         // ORIGIN IGP
        0x40, 0x02, 0x06, 0x02, 0x02, 0x00, 0x01, 0x00, 0x02, // AS_PATH: 1 2
        0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,       // NEXT_HOP 192.0.2.1
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = ValidateUpdateRFC7606(pathAttrs, true, false, false)
    }
}

// BenchmarkValidateUpdateRFC7606Large measures with many attributes.
func BenchmarkValidateUpdateRFC7606Large(b *testing.B) {
    // Larger UPDATE with communities
    pathAttrs := []byte{
        0x40, 0x01, 0x01, 0x00,                         // ORIGIN
        0x40, 0x02, 0x0a, 0x02, 0x04, 0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00, 0x04, // AS_PATH
        0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,       // NEXT_HOP
        0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64,       // MED
        0xc0, 0x08, 0x08, 0x00, 0x01, 0x00, 0x01, 0x00, 0x02, 0x00, 0x02, // Communities
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = ValidateUpdateRFC7606(pathAttrs, true, false, true)
    }
}
```

Run:
```bash
go test -bench=BenchmarkValidateUpdateRFC7606 -benchmem ./pkg/bgp/message/
```

#### Step 2: Add Cache Potential Measurement

**File:** `pkg/bgp/message/validation_cache_potential_test.go`

```go
//go:build measurement
// +build measurement

package message

import (
    "crypto/sha256"
    "fmt"
    "testing"
)

// TestCachePotential measures duplicate rate in sample traffic.
// Run with: go test -tags=measurement -v -run TestCachePotential ./pkg/bgp/message/
//
// This test simulates different traffic patterns to estimate cache hit rate.
// Adjust the test data to match your expected traffic profile.
func TestCachePotential(t *testing.T) {
    scenarios := []struct {
        name        string
        generator   func(i int) ([]byte, bool, bool, bool)
        count       int
        description string
    }{
        {
            name:        "RouteReflector",
            description: "Same routes to many clients (high duplicate rate)",
            count:       10000,
            generator: func(i int) ([]byte, bool, bool, bool) {
                // 100 unique path attrs, repeated
                pathID := i % 100
                pathAttrs := makePathAttrs(pathID)
                return pathAttrs, true, true, true // IBGP, ASN4
            },
        },
        {
            name:        "FullTable",
            description: "Full table with some shared AS_PATHs",
            count:       10000,
            generator: func(i int) ([]byte, bool, bool, bool) {
                // 5000 unique path attrs (50% duplicate rate)
                pathID := i % 5000
                pathAttrs := makePathAttrs(pathID)
                return pathAttrs, true, false, true // EBGP, ASN4
            },
        },
        {
            name:        "DiverseRoutes",
            description: "All unique paths (worst case)",
            count:       10000,
            generator: func(i int) ([]byte, bool, bool, bool) {
                // All unique
                pathAttrs := makePathAttrs(i)
                return pathAttrs, true, false, true
            },
        },
    }

    for _, sc := range scenarios {
        t.Run(sc.name, func(t *testing.T) {
            seen := make(map[string]bool)
            duplicates := 0

            for i := 0; i < sc.count; i++ {
                pathAttrs, hasNLRI, isIBGP, asn4 := sc.generator(i)
                key := makeCacheKey(pathAttrs, hasNLRI, isIBGP, asn4)

                if seen[key] {
                    duplicates++
                } else {
                    seen[key] = true
                }
            }

            hitRate := float64(duplicates) / float64(sc.count) * 100
            t.Logf("%s: %d/%d duplicates (%.1f%% cache hit potential)",
                sc.description, duplicates, sc.count, hitRate)

            // Report recommendation
            if hitRate > 30 {
                t.Logf("RECOMMENDATION: Caching LIKELY beneficial")
            } else {
                t.Logf("RECOMMENDATION: Caching probably NOT beneficial")
            }
        })
    }
}

func makePathAttrs(id int) []byte {
    // Generate deterministic path attrs based on ID
    asn := uint16(id%65000 + 1)
    return []byte{
        0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
        0x40, 0x02, 0x04, 0x02, 0x01, byte(asn >> 8), byte(asn), // AS_PATH
        0x40, 0x03, 0x04, 192, 0, 2, byte(id%254 + 1), // NEXT_HOP
    }
}

func makeCacheKey(pathAttrs []byte, hasNLRI, isIBGP, asn4 bool) string {
    h := sha256.Sum256(pathAttrs)
    var flags byte
    if hasNLRI {
        flags |= 1
    }
    if isIBGP {
        flags |= 2
    }
    if asn4 {
        flags |= 4
    }
    return fmt.Sprintf("%x-%d", h[:8], flags)
}
```

Run:
```bash
go test -tags=measurement -v -run TestCachePotential ./pkg/bgp/message/
```

#### Step 3: Profile Real Traffic (Optional)

Add temporary instrumentation to `session.go`:

```go
// Temporary: track validation cache potential
var (
    validationTotal   atomic.Uint64
    validationUnique  sync.Map // map[string]struct{}
)

func (s *Session) validateUpdateRFC7606(body []byte) error {
    // ... existing code ...

    // Temporary measurement
    key := fmt.Sprintf("%x-%v-%v-%v", sha256.Sum256(pathAttrs)[:8], hasNLRI, isIBGP, asn4)
    validationTotal.Add(1)
    validationUnique.Store(key, struct{}{})

    // Log periodically
    if total := validationTotal.Load(); total%10000 == 0 {
        var unique int
        validationUnique.Range(func(_, _ any) bool { unique++; return true })
        hitRate := float64(total-uint64(unique)) / float64(total) * 100
        s.log.Info("validation cache potential", "total", total, "unique", unique, "hitRate", hitRate)
    }

    // ... rest of validation ...
}
```

**Success criteria to proceed:**
- [ ] Potential cache hit rate > 30%
- [ ] Validation > 5% of CPU time in profiles

**If criteria not met:** STOP. Do not implement caching. The optimization is not worth the complexity.

---

### Phase 1: Cache Implementation

**File:** `pkg/bgp/message/validation_cache.go`

**TDD Order:**
1. Write `validation_cache_test.go` with tests:
   - `TestValidationCacheHit` - same input returns cached result
   - `TestValidationCacheMiss` - different input calls validator
   - `TestValidationCacheKeyFlags` - flag packing correct
   - `TestValidationCacheConcurrent` - thread-safe
   - `TestValidationCacheEviction` - LRU eviction works
   - `TestValidationCacheMetrics` - hits/misses tracked

2. Run tests - MUST FAIL

3. Implement `ValidationCache`

4. Run tests - MUST PASS

---

### Phase 2: Integration

**File:** `pkg/reactor/session.go`

1. Add `validationCache *message.ValidationCache` field to Session
2. Wire cache in `validateUpdateRFC7606()`
3. Add config option to enable/disable (default: disabled)

---

### Phase 3: Metrics & Observability

Export cache metrics for monitoring:

```go
type ValidationCacheMetrics struct {
    Hits       uint64  // Cache hits
    Misses     uint64  // Cache misses
    HitRate    float64 // Hits / (Hits + Misses)
    Size       int     // Current entries
    Evictions  uint64  // LRU evictions
}
```

---

## Configuration

```yaml
validation:
  cache:
    enabled: false      # Default OFF - enable only if measured benefit
    size: 10000         # Max cached entries
```

**Rationale for default OFF:**
- Cache benefit depends on traffic patterns
- Some deployments may have no duplicates
- Memory overhead (~1MB for 10k entries) not always justified

---

## Memory Analysis

Per cache entry:
- Key: ~100 bytes (typical path attrs) + 1 byte flags
- Value: ~80 bytes (RFC7606ValidationResult with string)
- LRU overhead: ~64 bytes
- **Total: ~250 bytes per entry**

| Cache Size | Memory |
|------------|--------|
| 1,000 | ~250 KB |
| 10,000 | ~2.5 MB |
| 100,000 | ~25 MB |

---

## When Caching Helps

| Scenario | Cache Benefit |
|----------|---------------|
| Route reflector with many clients | High (same routes to all clients) |
| Full table peer (800k routes) | Medium (some shared AS_PATHs) |
| Diverse routes, unique paths | Low (few duplicates) |
| Single peer, few routes | None (no duplicates) |

---

## Risks

| Risk | Mitigation |
|------|------------|
| Memory overhead | Configurable size, default OFF |
| Cache invalidation | None needed - validation is pure function |
| Lock contention | Use hashicorp/golang-lru (minimal locking) |
| Complexity | Simple LRU, ~100 lines of code |

---

## Success Criteria

- [ ] Phase 0 measurements show > 30% cache potential
- [ ] `make test && make lint` pass
- [ ] Benchmarks show measurable improvement
- [ ] No behavioral changes (validation results identical)
- [ ] Memory usage within bounds

---

## Decision Record

**DO implement:**
- Simple LRU cache with configurable size
- Per-session or global cache (TBD based on measurement)
- Metrics for cache effectiveness

**DO NOT implement:**
- Result pooling (Go handles small structs efficiently)
- Complex hash collision handling (use string keys)
- Singleflight (duplicate work acceptable, simpler)

---

## Files to Create/Modify

| File | Action |
|------|--------|
| `pkg/bgp/message/validation_cache.go` | Create |
| `pkg/bgp/message/validation_cache_test.go` | Create |
| `pkg/reactor/session.go` | Modify (integration) |
| `pkg/config/config.go` | Modify (config option) |

---

## Dependencies

- `github.com/hashicorp/golang-lru/v2` - Battle-tested LRU implementation

---

## References

- RFC 7606: Revised Error Handling for BGP UPDATE Messages
- Current implementation: `pkg/bgp/message/rfc7606.go`
- Call site: `pkg/reactor/session.go:534`

---

**Next Step:** Run Phase 0 measurements to determine if caching is worthwhile.
