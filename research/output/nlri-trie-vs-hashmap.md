# NLRI Storage: Trie vs Hash Map Analysis

**Date:** 2024-12-22
**Status:** Research Complete

## Current ZeBGP Approach

Hash map with FNV-1a collision chains:

```
FNV-1a(NLRI bytes) → bucket[] → linear scan for match
```

**Key files:**
- `internal/rib/store.go` - RouteStore with per-family NLRI stores
- `internal/store/nlri.go` - FamilyStore hash bucket implementation
- `internal/bgp/nlri/inet.go` - INET NLRI using `netip.Prefix`

**Index format:** `AFI(2) + SAFI(1) + NLRI_bytes + [AS-PATH_hash(8)]`

---

## Memory Comparison

### Per-Entry Overhead

| Approach | Per-Entry | Notes |
|----------|-----------|-------|
| **Hash map** | ~48 bytes | prefix + map overhead |
| **Binary trie** | ~72 bytes | 2 child ptrs + data ptr |
| **Radix/Patricia** | ~64 bytes | compressed paths |
| **Level-compressed** | ~32 bytes | multi-bit strides |

### Full Internet Table (~1M prefixes)

| Approach | IPv4 | IPv6 | Combined |
|----------|------|------|----------|
| **Hash map** | ~48 MB | ~64 MB | ~112 MB |
| **Binary trie** | ~72 MB | ~2+ GB* | Impractical |
| **Radix/Patricia** | ~64 MB | ~128 MB | ~192 MB |
| **Level-compressed** | ~32 MB | ~64 MB | ~96 MB |

*Binary trie explodes for IPv6 due to /128 path depth

---

## Performance Comparison

### Time Complexity

| Operation | Hash Map | Patricia Trie |
|-----------|----------|---------------|
| **Exact lookup** | O(1) avg | O(k) k=prefix len |
| **Insert** | O(1) avg | O(k) |
| **Delete** | O(1) avg | O(k) |
| **LPM** | O(n) scan | O(k) ✅ |
| **Covered-by query** | O(n) scan | O(k) ✅ |
| **Covering query** | O(n) scan | O(k) ✅ |
| **Range iteration** | O(n) | O(m) m=range size |

### Estimated Benchmarks

```
1M prefixes, exact lookup:

Hash map:  ~50-100 ns/lookup
Patricia:  ~200-400 ns/lookup (IPv4 /24 = 24 bit comparisons)

1M prefixes, LPM lookup:

Hash map:  N/A (requires full scan)
Patricia:  ~200-400 ns/lookup
```

### Cache Behavior

| Aspect | Hash Map | Trie |
|--------|----------|------|
| **Locality** | Poor (random access) | Better (tree walk) |
| **Predictability** | Unpredictable | Deterministic |
| **Prefetch** | Difficult | Possible |

---

## BGP-Specific Considerations

### Feature Support

| Feature | Hash Map | Trie |
|---------|----------|------|
| **Route deduplication** | ✅ Native | ✅ At leaves |
| **ADD-PATH support** | ✅ In index | Needs leaf lists |
| **ORF prefix filters** | O(n×f) | O(k×f) per filter |
| **Aggregation detect** | O(n) scan | O(1) parent check |
| **RPKI validation** | External | Can embed ROA |
| **Flowspec matching** | External | Native prefix match |

### Use Case Fit

| Use Case | Best Structure |
|----------|----------------|
| **Route server** | Hash map ✅ |
| **Looking glass** | Hash map ✅ |
| **Forwarding router** | Trie (LPM required) |
| **Policy engine** | Trie (range queries) |
| **RPKI validator** | Trie (coverage queries) |

---

## Implementation Complexity

### Hash Map (Current)

```go
type FamilyStore[T NLRIHashable] struct {
    entries map[uint64][]familyEntry[T]
    mu      sync.RWMutex
}
```

- **Lines of code:** ~200
- **Concurrency:** Simple RWMutex
- **Testing:** Straightforward

### Patricia Trie

```go
type PatriciaNode struct {
    bit      int           // bit position to test
    prefix   netip.Prefix  // stored prefix (if leaf)
    children [2]*PatriciaNode
    data     *Route
}
```

- **Lines of code:** ~500-800
- **Concurrency:** Complex (path copying or fine-grained locks)
- **Testing:** Edge cases for bit manipulation

---

## Recommendation

### Keep Hash Map For

✅ RIB storage (exact match dominant)
✅ Route deduplication
✅ UPDATE processing
✅ Adj-RIB-In/Out management

### Consider Trie For

🔍 Prefix-list filtering (future)
🔍 RPKI origin validation (future)
🔍 BGP Flowspec matching (future)
🔍 Route analytics/reporting (future)

### Rationale

1. **ZeBGP is not a forwarding router** - No LPM requirement
2. **BGP UPDATE = exact match** - Hash map optimal
3. **Complexity cost** - Trie adds ~400 LoC, testing burden
4. **Memory difference marginal** - ~80 MB for full table
5. **Current design clean** - Worker goroutines, reference counting work well

---

## If We Did Implement Trie

### Recommended Approach

1. **Use `github.com/kentik/patricia`** or similar battle-tested library
2. **Wrap, don't replace** - Trie as optional index alongside hash map
3. **Lazy population** - Build trie only when LPM/range queries needed
4. **IPv4/IPv6 separate** - Two tries, not unified

### Go Libraries

| Library | Notes |
|---------|-------|
| `github.com/kentik/patricia` | Production-proven, IPv4/IPv6 |
| `github.com/yl2chen/cidranger` | Simple API, good performance |
| `github.com/gaissmai/cidrtree` | Modern, generics-based |

---

## Conclusion

**Current hash map approach is correct for ZeBGP's route server use case.**

Trie would add ~70% more code complexity for marginal benefit. Only revisit if:
- Implementing prefix-list ORF
- Adding RPKI validation
- Building route analytics with coverage queries
