# Spec: fixit-recent-cache-buffer-reclaim

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

Re-read after compaction:
1. This spec file.
2. `.claude/rules/planning.md` - workflow rules.
3. `internal/component/bgp/reactor/recent_cache.go` - the cache, valve, soft limit.
4. `internal/component/bgp/reactor/bufmux.go` + `session.go` - shared read pool and its utilization signal.

## Task

**[MEDIUM]** A slow or stuck (not crashed) cache-consumer plugin can pin the shared
read-buffer pool for minutes, starving inbound-UPDATE processing for ALL peers. Each
`RecentUpdateCache` entry pins a `ReceivedUpdate` holding a pooled read-buffer handle
(`received_update.go:58-60`, `poolBuf BufHandle`), released only on consumer `Ack` or by
the gap-based safety valve. `Add` warns but never REJECTS at the soft limit
(`recent_cache.go:285-291`); there is no TTL (`:50`); the valve defaults to 5 minutes
(`:30`) and is scanned every 30 seconds (`:34`). Those read buffers come from the
byte-budgeted `bufMuxStd`/`bufMuxExt` pool (`session.go:56,61`) that also feeds the live
session read path, so sustained pinning starves buffer acquisition for every peer's
incoming UPDATEs long before the valve fires. This is legitimate retention with
coarse-grained reclamation (distinct from the separately-specced forward-path buffer
LEAK). Make reclamation load-aware while preserving correct delivery for well-behaved
consumers.

## Required Reading

- [ ] `internal/component/bgp/reactor/recent_cache.go` - cache, soft limit, gap valve
  → Constraint: `Add` (`:278`) never rejects; soft limit only warns (`:285-291`). No TTL (`:50`).
  → Constraint: `isGapEvictable` (`:135-149`) fires ONLY when `highestFullyAcked > entryID` (a later entry acked) AND `retainedAt` age > valve. Frontier entries (nothing later acked) are never timed out - do not break this for slow-but-correct consumers.
  → Decision: the valve duration is already config-registered (`SetSafetyValveDuration`, `:264`; env `ze.cache.safety.valve`, `reactor.go:526`). Any new threshold registers the same way, not a hardcoded magic constant.
- [ ] `internal/component/bgp/reactor/session.go` - shared read pool + pressure signal
  → Constraint: `CombinedBufMuxUsedRatio()` (`:95-96`) already exposes 0.0..1.0 pool utilization across both muxes; reuse it as the high-water signal, do not add a parallel one.
  → Constraint: `ReturnReadBuffer` (`:119`) is the cache's only buffer-return path; eviction correctness flows through it.
- [ ] `internal/component/bgp/reactor/bufmux.go` - budgeted block pool
  → Constraint: `Get()` returns a zero handle (`Buf == nil`) when the budget denies growth; that starvation is exactly what pinned cache entries cause.
- [ ] `ai/rules/config-surface.md`, `ai/rules/config-naming.md` - env-var vs YANG for any new threshold.

**Key insights:**
- A load-aware signal already exists (`CombinedBufMuxUsedRatio`); the fix is granularity, not new plumbing.
- The soft limit is a warning today; a hard cap that sheds the OLDEST stalled entry is the smallest behavior change that bounds pinning.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/recent_cache.go` - `Add` (`:278`), soft-limit warn (`:285-291`), `runGapScan` (`:228`), `isGapEvictable` (`:135`), `SetSafetyValveDuration` (`:264`), `evictLocked` (`:459`).
- [ ] `internal/component/bgp/reactor/received_update.go` - `ReceivedUpdate.poolBuf` (`:58-60`); EBGP variant buffers also pinned until eviction (`ebgpSlotASN4/ASN2`).
- [ ] `internal/component/bgp/reactor/session.go` - `bufMuxStd`/`bufMuxExt` (`:56,61`), `CombinedBufMuxUsedRatio` (`:95`), `ReturnReadBuffer` (`:119`).
- [ ] `internal/component/bgp/reactor/bufmux.go` - `BufMux.Get`/`Return`, `combinedBudget.tryReserve` (`:141`).

**Behavior to preserve:**
- Correct delivery for well-behaved consumers: an entry a live consumer will still `Ack` must NOT be evicted prematurely; frontier (slowest-but-progressing) entries stay retained.
- FIFO / unordered ack semantics, immediate O(1) eviction on full ack, and the existing `ze.cache.safety.valve` override.

**Behavior to change:**
- Make reclamation load-aware: shorten the scan cadence and/or force early eviction of the oldest stalled entry when `CombinedBufMuxUsedRatio()` crosses a high-water mark; optionally enforce a hard cap (shed oldest) instead of warn-only; optionally a TTL.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Inbound UPDATE bytes land in a pooled read buffer, wrapped as a `ReceivedUpdate`, and `Add`ed to the cache. A stalled consumer never `Ack`s, so `evictLocked` (the only `ReturnReadBuffer` caller for cache entries) is never reached until the valve fires.

### Transformation Path
1. Session read acquires a buffer from `bufMuxStd`/`bufMuxExt` (`getReadBuf`, `received_update.go:93`).
2. `Add` stores the entry (soft limit only warns; `:285-291`).
3. Consumer stalls: no `Ack`, no immediate eviction; buffer stays out.
4. Background `runGapScan` (every 30s) evicts only entries past the frontier older than the 5-min valve; the pool can be starved well before.
5. Fix: on high pool utilization, tighten the scan cadence or force-shed the oldest stalled entry so `ReturnReadBuffer` runs sooner.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session read pool ↔ cache | `getReadBuf` out, `ReturnReadBuffer` back on evict | [ ] |
| Pool pressure ↔ valve | `CombinedBufMuxUsedRatio()` read by the reclaim path | [ ] |
| Config ↔ threshold | new high-water / cap registered as env var (or YANG) | [ ] |

### Integration Points
- `recent_cache.go` reclaim path, `session.go` utilization signal, the config surface for any new threshold (registration over hardcoding: no magic constant baked into the cache).

## Risks & Assumptions

| ID | Assumption / Risk | Basis | If wrong |
|----|-------------------|-------|----------|
| A-1 | `CombinedBufMuxUsedRatio()` is a sufficient, cheap pressure signal | `session.go:95-96` reads both muxes under lock | Need a lighter-weight probe; add one behind the same accessor |
| A-2 | Shedding the OLDEST stalled entry cannot break a well-behaved consumer | oldest passed-over entry is the most likely crashed/stuck; frontier is protected by `isGapEvictable` | Restrict shedding to entries already past `highestFullyAcked` |
| A-3 | A new threshold belongs on the config surface like `ze.cache.safety.valve` | `reactor.go:526`, `config-surface.md` | If YANG-modeled, add a leaf instead of an env var |
| R-1 | Load-aware eviction races the ack path and double-returns a buffer | `evictLocked` + `Ack` both under `c.mu` | Keep all eviction under `c.mu`; never return outside the lock |
| R-2 | Early eviction drops an entry a slow consumer still needed | frontier protection in `isGapEvictable` | Gate shedding on gap + pressure, never pressure alone at the frontier |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| pool utilization crosses high-water with stalled entries | -> | reclaim path force-evicts oldest passed-over entry | `TestCacheReclaimsUnderPoolPressure` |
| well-behaved slow consumer at the frontier under pressure | -> | entry retained, no premature evict | `TestCacheFrontierRetainedUnderPressure` |
| new threshold configured | -> | reclaim path reads the registered value | `TestCacheHighWaterConfigured` |

Concrete test: `TestCacheReclaimsUnderPoolPressure` builds a cache with a fake clock and a
stubbed pressure source reporting > high-water, adds N entries with one non-acking
(stalled) consumer past the frontier, drives one scan tick, and asserts the oldest stalled
entry is evicted and its `poolBuf` returned (`ReturnReadBuffer` observed) while frontier
entries survive.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Pool utilization ≥ high-water, stalled entry past the frontier | oldest passed-over entry force-evicted this scan; buffer returned to the pool |
| AC-2 | Slow-but-progressing consumer at the frontier under the same pressure | entry retained; no premature eviction (regression on `isGapEvictable`) |
| AC-3 | Soft limit exceeded with a hard cap configured | oldest stalled entry shed (not merely warned); `Len()` bounded |
| AC-4 | High-water / cap / TTL threshold | registered on the config surface (env var or YANG), not a hardcoded constant; default preserves current behavior |
| AC-5 | Normal load below high-water | behavior unchanged: 30s scan, 5-min valve, immediate ack eviction |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCacheReclaimsUnderPoolPressure` | `internal/component/bgp/reactor/recent_cache_test.go` | AC-1 | |
| `TestCacheFrontierRetainedUnderPressure` | `internal/component/bgp/reactor/recent_cache_test.go` | AC-2 | |
| `TestCacheHardCapShedsOldest` | `internal/component/bgp/reactor/recent_cache_test.go` | AC-3 | |
| `TestCacheHighWaterConfigured` | `internal/component/bgp/reactor/recent_cache_test.go` | AC-4 | |
| `TestCacheNormalLoadUnchanged` | `internal/component/bgp/reactor/recent_cache_test.go` | AC-5 | |

### Functional Tests
No user-facing behavior change; internal robustness only. No `.ci` functional test applies -
reclaim under pool pressure is covered by fake-clock/fake-pressure unit tests. Revisit if a
stress harness for consumer stalls is added.

## Files to Modify
- `internal/component/bgp/reactor/recent_cache.go` - load-aware reclaim (scan cadence and/or force-shed oldest), optional hard cap / TTL, threshold read.
- `internal/component/bgp/reactor/session.go` - only if the pressure accessor needs to be exposed to the cache (reuse `CombinedBufMuxUsedRatio`; add no parallel signal).
- config surface (env var or YANG leaf) for the new high-water / cap threshold.

## Implementation Steps

1. **Wiring (FIRST)** - thread a pressure source (`CombinedBufMuxUsedRatio`) into the cache and add the reclaim decision point with a minimal body; write the failing wiring tests.
2. Register the high-water / cap threshold on the config surface (default = current behavior); wire it like `ze.cache.safety.valve` (`reactor.go:526`).
3. Implement load-aware reclamation: tighten scan cadence and/or force-shed the oldest passed-over entry when pressure ≥ high-water, all under `c.mu`, routing through `evictLocked`/`ReturnReadBuffer` (R-1).
4. Preserve frontier protection (AC-2) and add the hard-cap shed path (AC-3); optional TTL.
5. Run + triage: unit tests, `make ze-lint-changed`, `make ze-test`.
6. Complete spec: audit, `plan/learned/NNN-<name>.md`, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete - every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Frontier retention preserved (no premature eviction of live-consumer entries)
- [ ] Registration over hardcoding respected (threshold on the config surface, not a magic constant)

### Quality Gates (SHOULD pass - defer with user approval)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary cases (at/above/below high-water; hard-cap edge) present

## Notes
- Skeleton captured from the 2026-07-16 repository audit. Distinct from the separately-specced forward-path buffer LEAK: this is legitimate retention with coarse reclamation.
- Drift observed (not in scope): `recent_cache.go:29` names the override `ZE_CACHE_SAFETY_VALVE`, but `reactor.go:526` reads `ze.cache.safety.valve`. Confirm the env mapping when adding the new threshold.
- Open question for the user: hard cap (shed oldest) vs load-aware valve-only vs TTL - which combination, and env var vs YANG leaf for the threshold?
