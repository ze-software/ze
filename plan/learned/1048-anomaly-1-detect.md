# 1048 - Anomaly Detection (behavioral, report-only): anomalyevent + anomaly/detect

**Spec:** spec-anomaly-1-detect (closed) | **Date:** 2026-07-02 | **Depends:** 1046 (traffic-analysis)

## Context

The Darktrace-style SECURITY anomaly detector: the judgment layer on top of Spec 1's
neutral `trafficfeature` facts. A deliberately SEPARATE domain from volumetric DDoS (no
shared event contract, detector, or responder). Report-only: it emits incidents; the
`anomaly/shape` responder (Spec 2b) acts.

## Key Decisions

- **Separate `anomalyevent` contract, source/entity-oriented.** Mirrors `ddosevent`'s
  `events.Register[T](ns, et)` pattern but with namespace `anomaly-detect` and structs keyed
  on a source `netip.Prefix` (not a destination `VectorTuple`). `events.Register` self-registers
  the namespace (typed.go), so package-init `var` declarations are all the contract needs.
- **Consume trafficfeature, baseline in the detector.** The detector reads
  `trafficfeature.Snapshot()` on its own 1s ticker (trafficfeature has Snapshot, not a
  SubscribeRates callback). Per-(entity,feature) baseline = TWO `stats.EWMA`s (value +
  value-squared) -> running mean and stddev. This WIRES `stats.EWMA`, resolving the EWMA half
  of Spec 1's review ISSUE-1. `stats.Quantile` stays unwired (the pinned rule uses mean/stddev,
  not quantile) -- an accepted carry.
- **Pinned scoring rule is authoritative and pure.** `score.go`: `zScore` (positive deviation,
  stddev floored per-feature, clamped to zMax=10), cohort `rarity` (same z vs source-prefix
  cohort, tiny-cohort fallback, LEAVE-ONE-OUT so an outlier is not measured against a baseline
  it dominates, +Inf-ratio exfil hosts excluded from the cohort baseline so they cannot mask
  peers -- both review-driven, see Gotchas), `combineScore` (Zstrong + corroboration-weight*sum(others),
  capped at scoreMax=30 -- NOT naive sum). Continuous features take max(self, cohort); binary
  features (new-peer, rare-port) contribute exactly one threshold unit.
- **Freeze-learn + warmup, not update-before-score** (the initial cut used update-before-score,
  which self-dampened via slow poisoning). `scoreEntity` now scores WITHOUT mutating the baseline
  and returns the pending `baselineUpdate`s; `onTick` folds them only when the entity is NOT
  anomalous this tick, or while still warming (`warmupTicks=3`). So a SUSTAINED anomaly cannot
  drift the entity's own baseline up until it looks normal, and a never-seen entity has NO
  self-deviation until its baseline warms (no first-sight false positive). Regression:
  `TestFreezeLearnDuringSustainedAnomaly`.
- **Report surface = process-global + pluginserver RPC.** Plugins run in-process, so the detector
  sets a package-global (`setGlobalDetector`) that the `ze-show:anomaly` handler (registered via
  `pluginserver.RegisterRPCs` in the same package) reads. No cross-process plumbing.

## Consequences

- `anomaly-detect` is a report-only plugin: config `anomaly-detect{enabled true}` starts it,
  it attaches to trafficfeature, ticks, scores, and emits `anomaly-detect` events + a bounded
  recent-incident ring. `show anomaly` surfaces the ring; `anomaly/shape` (Spec 2b) will
  subscribe to the events.
- Metrics: `ze_anomaly_incidents_total`, `ze_anomaly_active`, `ze_anomaly_tracked_entities`.
  Doctor: `anomaly-detect-feature-source` (+ code in diagnostic/codes.go) warns when enabled
  without a flow source (trafficfeature has no data).

## Gotchas

- **Concurrent-session known-reds (same as 1046).** `plugin/all` snapshot and `ze-doc-test`
  were red from concurrent OSPF work (ospf wire methods not in golden; interop/suite count
  drift). Scope by verifying MY additions are accepted: `ze-show:anomaly`, `anomaly-detect`
  (plugins + yang-providers snapshots), DESIGN.md Shipped Plugins, and doc anchors all pass;
  the residue is entirely `ze-*ospf*`. Do not touch another session's golden.
- **The .counter races.** Two sessions bumped it mid-work last time (1044 collision). Always
  allocate via `commit_helper.py learned-next <slug>` (atomic), never a hand-picked number.
- **Emit path isn't functionally .ci-testable** without a traffic generator to synthesize
  feature deviations; the confirm/clear/emit/ring path is covered by the unit lifecycle test.
  `anomaly-show.ci` proves the wiring chain (config -> RunEngine -> setGlobalDetector -> show).
- Hook cadence: `wg.Add(1)/go func` trips `modernize` -> use `wg.Go(func(){...})` (Go 1.25).
- **Cohort rarity with mean/stddev is fragile to outliers (review ISSUE 1 + NOTE 2).** The first
  cut fed `finiteRatio(+Inf)=1e6` into the cohort ratio distribution and scored an entity against
  a cohort that INCLUDED itself. Both let an exfiltrator (+Inf ratio) dominate its /24 baseline and
  mask peers, and dampened the outlier's own rarity. Fix: `cohortStats` (running sum/sumSq/count)
  with leave-one-out `rarity`, and exclude +Inf members from the ratio cohort. Regression:
  `TestBuildCohortsExcludesInfiniteRatio`, and `TestCohortRarity` asserts leave-one-out > self-included.
  Cohort rarity is also SPARSE in practice (few same-/24 sources active at once) -- a slice-one limit.

## Files

None recorded.
