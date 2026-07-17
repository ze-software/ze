# Spec: fixit-recent-cache-buffer-reclaim

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-17 |

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
- ~~The soft limit is a warning today; a hard cap that sheds the OLDEST stalled entry is the smallest behavior change that bounds pinning.~~
  → SUPERSEDED (2026-07-17, valve-only decision): the soft limit STAYS warn-only (no hard cap, no shedding at `Add`). The bounded-pinning guarantee comes instead from a load-aware shortened safety valve: under pool pressure, passed-over (gap-evictable) entries are reclaimed on a shorter valve than 5 min, so buffers return sooner without ever dropping frontier/live-consumer data.

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
- ~~Make reclamation load-aware: shorten the scan cadence and/or force early eviction of the oldest stalled entry when `CombinedBufMuxUsedRatio()` crosses a high-water mark; optionally enforce a hard cap (shed oldest) instead of warn-only; optionally a TTL.~~
  → RESOLVED (2026-07-17, valve-only decision): Make reclamation load-aware by applying a SHORTER effective safety-valve duration to already-passed-over (gap-evictable) entries when `CombinedBufMuxUsedRatio()` ≥ the configured high-water mark. The eviction *criteria* stay identical to today (`isGapEvictable` unchanged — frontier entries protected); only the valve *duration* shrinks under pressure. NO hard cap (soft limit remains warn-only) and NO TTL. Shortening the 30s scan cadence under pressure is OPTIONAL and out of scope for the minimal fix (a 30s worst-case reclaim latency is already far below the 5-min default); the shortened valve is the load-aware lever.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Inbound UPDATE bytes land in a pooled read buffer, wrapped as a `ReceivedUpdate`, and `Add`ed to the cache. A stalled consumer never `Ack`s, so `evictLocked` (the only `ReturnReadBuffer` caller for cache entries) is never reached until the valve fires.

### Transformation Path
1. Session read acquires a buffer from `bufMuxStd`/`bufMuxExt` (`getReadBuf`, `received_update.go:93`).
2. `Add` stores the entry (soft limit only warns; `:285-291`).
3. Consumer stalls: no `Ack`, no immediate eviction; buffer stays out.
4. Background `runGapScan` (every 30s) evicts only entries past the frontier older than the 5-min valve; the pool can be starved well before.
5. ~~Fix: on high pool utilization, tighten the scan cadence or force-shed the oldest stalled entry so `ReturnReadBuffer` runs sooner.~~
   → RESOLVED (2026-07-17, valve-only): Fix — when `CombinedBufMuxUsedRatio()` ≥ high-water, `runGapScan` applies the shortened `ze.cache.pressure.valve` duration in place of the 5-min `safetyValve` when calling `isGapEvictable`, so passed-over stalled entries are reclaimed (via `evictLocked` → `ReturnReadBuffer`) on the very next scan tick. Frontier entries stay protected (criteria unchanged); nothing is force-shed unconditionally.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session read pool ↔ cache | `getReadBuf` out, `ReturnReadBuffer` back on evict | [ ] |
| Pool pressure ↔ valve | `CombinedBufMuxUsedRatio()` read by the reclaim path (same `reactor` package — cache calls it directly; injected as a stubbable `func() float64` for tests) | [ ] |
| Config ↔ threshold | `ze.cache.pressure.highwater` + `ze.cache.pressure.valve` registered as env vars (`environment.go`), read in `reactor.go`, wired into the cache like `ze.cache.safety.valve` — no YANG leaf | [ ] |

### Integration Points
- `recent_cache.go` reclaim path, `session.go` utilization signal, the config surface for any new threshold (registration over hardcoding: no magic constant baked into the cache).

## Risks & Assumptions

| ID | Assumption / Risk | Basis | If wrong |
|----|-------------------|-------|----------|
| A-1 | `CombinedBufMuxUsedRatio()` is a sufficient, cheap pressure signal | `session.go:95-96` reads both muxes under lock | Need a lighter-weight probe; add one behind the same accessor |
| A-2 | ~~Shedding the OLDEST stalled entry cannot break a well-behaved consumer~~ → (valve-only 2026-07-17) Accelerating the valve for gap-evictable entries cannot break a well-behaved consumer | `isGapEvictable` gap condition is unchanged, so only already-passed-over (stalled) entries are affected; frontier entries are never reached | Keep the pressure path routed through `isGapEvictable` — never bypass the gap condition |
| A-3 | A new threshold belongs on the config surface like `ze.cache.safety.valve` | `reactor.go:526`, `config-surface.md` | → RESOLVED (2026-07-17): env var chosen (internal safety cap, not operator capacity knob). If wrong, promote to a YANG leaf per `config-surface.md` "When Both Exist" precedence |
| R-1 | Load-aware eviction races the ack path and double-returns a buffer | `evictLocked` + `Ack` both under `c.mu` | Keep all eviction under `c.mu`; never return outside the lock |
| R-2 | Early eviction drops an entry a slow consumer still needed | frontier protection in `isGapEvictable` | Gate shedding on gap + pressure, never pressure alone at the frontier |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| pool utilization crosses high-water with stalled entries | -> | reclaim path evicts passed-over (gap-evictable) entries on the shortened pressure valve | `TestCacheReclaimsUnderPoolPressure` |
| well-behaved slow consumer at the frontier under pressure | -> | entry retained, no premature evict | `TestCacheFrontierRetainedUnderPressure` |
| new threshold configured | -> | reclaim path reads the registered env-var value | `TestCacheHighWaterConfigured` |

Concrete test: `TestCacheReclaimsUnderPoolPressure` builds a cache with a fake clock and a
stubbed pressure source (`func() float64`) reporting ≥ high-water, adds N entries with one
non-acking (stalled) consumer past the frontier, advances the fake clock past the shortened
`ze.cache.pressure.valve` (but well short of the 5-min `safetyValve`), drives one scan tick,
and asserts the passed-over stalled entry is evicted and its `poolBuf` returned
(`ReturnReadBuffer` observed) while frontier entries survive. Eviction stays gated on
gap + (shortened) valve age — never unconditional shedding.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Pool utilization ≥ high-water, stalled entry past the frontier, aged past the shortened pressure valve | passed-over entry evicted this scan (via `evictLocked`); buffer returned to the pool. Eviction stays gap-gated (`isGapEvictable`) — timing accelerates, criteria unchanged |
| AC-2 | Slow-but-progressing consumer at the frontier under the same pressure | entry retained; no premature eviction (regression on `isGapEvictable`) |
| AC-3 | Soft limit exceeded under high-water pressure ~~(orig: "with a hard cap configured")~~ | ~~oldest stalled entry shed (not merely warned); `Len()` bounded~~ → SUPERSEDED 2026-07-17 (valve-only): soft limit stays warn-only — `Add` NEVER rejects or sheds; pinning is bounded instead by the shortened pressure valve reclaiming passed-over entries on the next scan, so `Len()` drains once consumers stall (no hard cap) |
| AC-4 | High-water ratio + pressure-valve duration thresholds | registered as ENV VARS on the config surface (`ze.cache.pressure.highwater`, `ze.cache.pressure.valve`), not hardcoded constants; defaults preserve current behavior (high-water 0 = feature disabled). No YANG leaf |
| AC-5 | Normal load below high-water (or feature disabled) | behavior unchanged: 30s scan, 5-min valve, immediate ack eviction |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCacheReclaimsUnderPoolPressure` | `internal/component/bgp/reactor/recent_cache_test.go` | AC-1 | |
| `TestCacheFrontierRetainedUnderPressure` | `internal/component/bgp/reactor/recent_cache_test.go` | AC-2 | |
| ~~`TestCacheHardCapShedsOldest`~~ → `TestCacheSoftLimitStaysWarnOnlyUnderPressure` (valve-only, 2026-07-17): asserts `Add` past the soft limit never rejects, and that under stubbed high-water pressure `Len()` drains as passed-over entries are reclaimed on the shortened valve | `internal/component/bgp/reactor/recent_cache_test.go` | AC-3 | |
| `TestCacheHighWaterConfigured` | `internal/component/bgp/reactor/recent_cache_test.go` | AC-4 | |
| `TestCacheNormalLoadUnchanged` | `internal/component/bgp/reactor/recent_cache_test.go` | AC-5 | |

### Functional Tests
No user-facing behavior change; internal robustness only. No `.ci` functional test applies -
reclaim under pool pressure is covered by fake-clock/fake-pressure unit tests. Revisit if a
stress harness for consumer stalls is added.

## Files to Modify
- `internal/component/bgp/reactor/recent_cache.go` - load-aware reclaim: add a stubbable `pressureSource func() float64` field (defaulting to `CombinedBufMuxUsedRatio`) and a `pressureValve time.Duration` + `pressureHighWater float64`; in `runGapScan`, when `pressureSource() >= pressureHighWater` (and the feature is enabled), pass the shortened `pressureValve` to `isGapEvictable` instead of `safetyValve`. `Add`/soft-limit stay warn-only; `isGapEvictable` criteria unchanged. ~~scan cadence and/or force-shed oldest, optional hard cap / TTL~~ (superseded 2026-07-17: valve-only).
- `internal/component/bgp/reactor/session.go` - ~~only if the pressure accessor needs to be exposed to the cache~~ → NOT expected to change: `CombinedBufMuxUsedRatio` (`session.go:95`) is package-level in `reactor`, so `recent_cache.go` (same package) calls it directly; the injectable `func() float64` field exists only so tests can stub pressure. Add NO parallel signal.
- `internal/component/config/environment.go` - `env.MustRegister` two new keys next to `ze.cache.safety.valve` (`:118`): `ze.cache.pressure.highwater` (Type `int` percent 0-100, or a new `float` accessor — see note; default disabled) and `ze.cache.pressure.valve` (Type `duration`, default `30s`).
- `internal/component/bgp/reactor/reactor.go` - read the two new env keys near `ze.cache.safety.valve` (`:526`) and wire them into the cache via new setters (e.g. `SetPressureHighWater`/`SetPressureValve`), mirroring `SetSafetyValveDuration`.

Note (env-var value type): `internal/core/env` has `GetInt`/`GetInt64`/`GetBool`/`GetDuration` but NO `GetFloat`. The high-water ratio is 0.0..1.0. Implementer picks ONE: (a) express high-water as an integer PERCENT via `GetInt` (`ze.cache.pressure.highwater` = 0-100, default 0 = disabled) and divide by 100 before comparing to `CombinedBufMuxUsedRatio()`; or (b) add a `GetFloat` accessor to `internal/core/env`. Default (a) — the smaller, self-contained change; avoids touching the env package.

## Implementation Steps

1. **Wiring (FIRST)** - thread a stubbable pressure source (`pressureSource func() float64`, default `CombinedBufMuxUsedRatio`) into the cache and add the reclaim decision point (pressure ≥ high-water → use shortened valve) with a minimal body; write the failing wiring tests.
2. Register `ze.cache.pressure.highwater` and `ze.cache.pressure.valve` as ENV VARS in `environment.go` (defaults = current behavior: high-water disabled); read them in `reactor.go:526` and wire via new setters like `SetSafetyValveDuration`.
3. Implement load-aware reclamation: in `runGapScan`, when `pressureSource() >= highWater` substitute `pressureValve` for `safetyValve` in the `isGapEvictable` call; all eviction stays under `c.mu`, routing through `evictLocked`/`ReturnReadBuffer` (R-1). ~~tighten scan cadence and/or force-shed the oldest passed-over entry~~ (superseded: valve-only).
4. Preserve frontier protection (AC-2 — `isGapEvictable` gap condition untouched). ~~add the hard-cap shed path (AC-3); optional TTL~~ → SUPERSEDED 2026-07-17: no hard cap, no TTL. AC-3 becomes "soft limit stays warn-only; pinning bounded by the shortened pressure valve."
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
- [ ] Boundary cases (at/above/below high-water; feature disabled = high-water 0; entry aged just under vs just over the shortened pressure valve) present

## Notes
- Skeleton captured from the 2026-07-16 repository audit. Distinct from the separately-specced forward-path buffer LEAK: this is legitimate retention with coarse reclamation.
- Drift observed (not in scope): `recent_cache.go:29` names the override `ZE_CACHE_SAFETY_VALVE`, but `reactor.go:526` reads `ze.cache.safety.valve`. Confirm the env mapping when adding the new threshold.
  → RESOLVED (2026-07-17): NOT a real drift. `internal/core/env` matching is case-insensitive and treats dots and underscores as equivalent (`env.go:64-70`, `Get()` normalizes the key), so `ZE_CACHE_SAFETY_VALVE` (the OS-env spelling named in the `recent_cache.go:29` comment) and `ze.cache.safety.valve` (the canonical registry key registered in `internal/component/config/environment.go:118` and read in `reactor.go:526`) are the SAME variable. New thresholds follow the same pattern: register the canonical dot-form key in `environment.go`, read it in `reactor.go`. No code change needed for the "drift" itself.
- Open question for the user: hard cap (shed oldest) vs load-aware valve-only vs TTL - which combination, and env var vs YANG leaf for the threshold?
  → AUTONOMOUS DEFAULT (2026-07-17): load-aware VALVE-ONLY, threshold(s) as ENV VARS. No hard cap (`Add` stays warn-only, never rejects or sheds at insert), no TTL. Under memory pressure (`CombinedBufMuxUsedRatio()` ≥ a configured high-water mark) the existing gap-based safety valve uses a SHORTER effective duration, so entries that have already been passed over (gap-evictable per `isGapEvictable`: not pending, has consumers, a later entry fully acked, aged past the valve) are reclaimed sooner. Frontier entries (nothing later fully acked) are NEVER evicted — the pressure path changes eviction *timing*, never the eviction *criteria*, so no cached data a well-behaved (slow-but-progressing) consumer still needs is ever dropped. This is the least-destructive of the three options: unlike a hard cap it never unconditionally sheds the oldest entry, and unlike a TTL it never times out live frontier work. Config surface = two ENV VARS, both defaulting to CURRENT behavior: `ze.cache.pressure.highwater` (ratio 0.0..1.0, default 0 = feature DISABLED) and `ze.cache.pressure.valve` (duration, default 30s, applied only while the feature is enabled). Env var (not YANG) per `config-surface.md`: this is an internal safety cap / emergency escape hatch that protects against stuck-consumer bugs rather than an operator capacity-planning knob, matching the classification of the existing `ze.cache.safety.valve`. Thomas: override if wrong (e.g. promote to a YANG leaf, or add a real hard cap that sheds the oldest passed-over entry).
