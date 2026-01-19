# Spec: RFC 7606 Validation Cache

## MANDATORY READING (BEFORE IMPLEMENTATION)

```
┌─────────────────────────────────────────────────────────────────┐
│  STOP. Read these files FIRST before ANY implementation:        │
│                                                                 │
│  1. .claude/ESSENTIAL_PROTOCOLS.md - Session rules, TDD         │
│  2. .claude/INDEX.md - Find what docs to load                   │
│  3. THIS SPEC FILE - Design requirements                        │
│  4. internal/bgp/attribute/validate.go - Current implementation      │
│                                                                 │
│  DO NOT PROCEED until all are read and understood.              │
└─────────────────────────────────────────────────────────────────┘
```

## Task

Implement optional LRU cache for RFC 7606 validation results to optimize repeated validation of identical path attributes.

## Problem

`ValidateUpdateRFC7606()` is called for every UPDATE message. When multiple UPDATEs share identical path attributes (common in route reflector scenarios), we re-validate the same bytes repeatedly.

**Priority:** Low (optimization, not required for correctness)

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- **FIRST:** Read `.claude/ESSENTIAL_PROTOCOLS.md` for session rules
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- Tests passing is NOT permission to commit - wait for user

### From ESSENTIAL_PROTOCOLS.md
- TDD is BLOCKING: Tests must exist and fail before implementation
- Measure FIRST: Do not optimize without proof of benefit
- Quality over speed: Take time to do it RIGHT

### From RFC 7606
- Validation result depends ONLY on path attribute bytes + session context
- Pure function: Same input = same output (cacheable)

## Codebase Context

### Files to Create/Modify

| File | Action |
|------|--------|
| `internal/bgp/message/validation_cache.go` | Create |
| `internal/bgp/message/validation_cache_test.go` | Create |
| `internal/bgp/message/rfc7606_bench_test.go` | Create (measurement) |
| `internal/reactor/session.go` | Modify (integration) |
| `internal/config/config.go` | Modify (config option) |

### Dependencies

- `github.com/hashicorp/golang-lru/v2` - Battle-tested LRU implementation

## Implementation Steps

### Phase 0: Measurement (REQUIRED FIRST - BLOCKING)

**Before ANY code changes, measure baseline:**

1. Write `rfc7606_bench_test.go` with benchmarks
2. Run: `go test -bench=BenchmarkValidateUpdateRFC7606 -benchmem ./internal/bgp/message/`
3. Write cache potential measurement test
4. Run with simulated traffic patterns
5. **Success criteria to proceed:**
   - [ ] Potential cache hit rate > 30%
   - [ ] Validation > 5% of CPU time in profiles

**If criteria NOT met:** STOP. Do not implement caching.

### Phase 1: Cache Implementation (TDD)

1. Write tests FIRST:
   - `TestValidationCacheHit`
   - `TestValidationCacheMiss`
   - `TestValidationCacheKeyFlags`
   - `TestValidationCacheConcurrent`
   - `TestValidationCacheEviction`
   - `TestValidationCacheMetrics`
2. Run tests - MUST FAIL
3. Implement `ValidationCache` with LRU
4. Run tests - MUST PASS

### Phase 2: Integration

1. Add `validationCache *message.ValidationCache` field to Session
2. Wire cache in `validateUpdateRFC7606()`
3. Add config option (default: disabled)

### Phase 3: Metrics & Observability

1. Export cache metrics for monitoring
2. Add `ValidationCacheMetrics` struct (Hits, Misses, HitRate, Size, Evictions)

## Verification Checklist

- [ ] Phase 0 measurements show > 30% cache potential
- [ ] TDD followed: Tests shown to FAIL first
- [ ] Benchmarks show measurable improvement
- [ ] No behavioral changes (validation results identical)
- [ ] Memory usage within bounds (~2.5 MB for 10k entries)
- [ ] `make test` passes
- [ ] `make lint` passes

## Configuration

```yaml
validation:
  cache:
    enabled: false      # Default OFF - enable only if measured benefit
    size: 10000         # Max cached entries
```

## Memory Analysis

| Cache Size | Memory |
|------------|--------|
| 1,000 | ~250 KB |
| 10,000 | ~2.5 MB |
| 100,000 | ~25 MB |

## When to Proceed

**Only implement if Phase 0 measurement shows:**
1. Cache hit rate > 30% in expected traffic patterns
2. Validation is measurable portion of CPU time

Otherwise, this optimization is not worth the complexity.
