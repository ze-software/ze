# 1190 - fixit-recent-cache-buffer-reclaim

Spec: `plan/spec-fixit-recent-cache-buffer-reclaim.md` (valve-only decision, 2026-07-17).

NOTE: learned number 1190 was chosen to avoid collision with concurrent sessions
(highest existing was 1186; `.counter` was stale at 1183). Run
`python3 scripts/dev/learned_numbers.py --fix` at drain if a collision surfaces.

## Problem

A slow or stuck (not crashed) cache-consumer plugin can pin the shared read-buffer
pool for minutes. Each `RecentUpdateCache` entry holds a pooled `BufHandle`
(`received_update.go:58` `poolBuf`), released only on consumer `Ack` or by the gap
safety valve (default 5 min, scanned every 30s). Those buffers come from the
byte-budgeted `bufMuxStd`/`bufMuxExt` pool that also feeds every live session's read
path, so sustained pinning starves inbound-UPDATE processing for ALL peers long before
the 5-minute valve fires.

## Decision: load-aware valve-only (no hard cap, no TTL)

Under memory pressure, reclaim already-passed-over (gap-evictable) entries on a SHORTER
valve. The eviction *criteria* (`isGapEvictable`) are unchanged — only the valve
*duration* shrinks. Frontier entries (nothing later fully acked) are never touched, so
no data a slow-but-progressing consumer still needs is ever dropped.

Rejected alternatives: a hard cap that sheds the oldest at `Add` (can drop a live
consumer's entry; `Add` stays warn-only), and a TTL (times out live frontier work).

## Implementation

- `recent_cache.go`: three new fields on `RecentUpdateCache` — `pressureSource
  func() float64` (default `CombinedBufMuxUsedRatio`, stubbable for tests),
  `pressureHighWater float64` (0 = disabled, the default), `pressureValve time.Duration`
  (default 30s, const `defaultPressureValveDuration`). `runGapScan` selects the valve:
  `valve := c.safetyValve; if pressureHighWater > 0 && pressureSource != nil &&
  pressureValve < valve && pressureSource() >= pressureHighWater { valve = pressureValve }`.
  Setters `SetPressureHighWater` (clamp 0..1), `SetPressureValve` (>0 only),
  `SetPressureSource`. `isGapEvictable` already took a `valve` parameter — no signature
  churn; the load-aware change is a one-line valve substitution.
- `config/environment.go`: registered `ze.cache.pressure.highwater` (int percent 0-100,
  default 0) and `ze.cache.pressure.valve` (duration, default 30s) next to
  `ze.cache.safety.valve`. Percent (not float) because `internal/core/env` has no
  `GetFloat` — the smaller change (spec note, option a). Divide by 100 at read.
- `reactor.go`: reads both keys near the existing `ze.cache.safety.valve` read; wires
  via the new setters only when `highwater > 0`.

## Traps / knowledge

- `env.Get`/`GetInt`/`GetDuration` call `mustBeRegistered` which `os.Exit(2)` on an
  unregistered key. Any key read in `reactor.go` MUST be registered in
  `config/environment.go` first. The reactor test binary transitively pulls in the
  `config` package init, so `env.IsRegistered("ze.cache.pressure.highwater")` is true in
  reactor unit tests (used as an AC-4 assertion).
- `pressureSource()` runs under `c.mu` (write lock) in `runGapScan`. Production
  `CombinedBufMuxUsedRatio` takes the two bufmux mutexes — DIFFERENT locks, and no path
  takes `c.mu` while holding a bufmux lock, so no lock-ordering hazard. Called once per
  30s scan, cheap.
- The guard `pressureValve < valve` makes the pressure path strictly shortening — a
  misconfigured `pressureValve` longer than `safetyValve` can never lengthen reclamation.
- Buffer-return observation in tests: attach a real `bufMuxStd.Get()` handle to the
  entry's `poolBuf`, snapshot `bufMuxStd.Stats()` in-use count, run the scan, assert it
  dropped by 1. Faithfully observes `ReturnReadBuffer` via `evictLocked`.

## Tests (recent_cache_test.go)

`TestCacheReclaimsUnderPoolPressure` (AC-1, buffer return observed),
`TestCacheFrontierRetainedUnderPressure` (AC-2), `TestCacheSoftLimitStaysWarnOnlyUnderPressure`
(AC-3), `TestCacheHighWaterConfigured` (AC-4, below/at/above high-water + env registration),
`TestCacheNormalLoadUnchanged` (AC-5, disabled default preserves the 5-min valve),
`TestCachePressureValveBoundary` (strict `>` valve boundary: just_under / exactly_at / just_over),
`TestReactorWiresPressureValveFromEnv` + `TestReactorPressureDisabledByDefault` (production
env→reactor→cache wiring and percent→ratio conversion — added in the review loop to close ISSUE-1;
use `env.Set` then `New(&Config{})`, restore with `os.Unsetenv` + `env.ResetCache` in defer). All
pass under `-tags "ze_core $ZE_FEATURES"`.

## Review

Independent reviewer verdict CLEAN. Raised ISSUE-1 (env-wiring path untested) and NIT-2
(only "just over" aging) — both fixed in the loop with the three tests above. Artifact:
`tmp/review/fixit-recent-cache-buffer-reclaim-<SID>.md`.

## Files

None recorded.
