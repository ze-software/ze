# 1046 - Traffic-Analysis Restructure (core/stats + trafficstat + trafficfeature)

**Spec:** spec-traffic-analysis-0-restructure (closed) | **Date:** 2026-07-02

## Context

The traffic-analysis layer was a single component (`trafficstat`, learned 1019) that
both measured traffic AND emitted a severity verdict. To let BOTH the DDoS plugins and a
NEW security `anomaly` family (Specs 2a/2b) share a domain-neutral "facts about traffic"
layer, it was decomposed into three parts under the dividing principle: **analysis
computes neutral FACTS; detection plugins apply JUDGMENT + own RESPONSE.**

## Key Decisions

- **Approach B (deep re-layering) over additive.** Extracted the per-key rolling
  window/rate/history/eviction machinery from `trafficstat/window.go`'s `entry` into a
  shared `internal/core/stats.Window` primitive and rebuilt `trafficstat`'s aggregator on
  it, rather than leaving `window.go` intact and only adding a lib. Chosen by the user for
  real reuse. R-1 (churning fresh 1019 code) was accepted; mitigated by keeping the public
  `Snapshot`/`SubscribeRates` API byte-identical and using the existing `window_test.go` as
  a behavior-identical CHARACTERIZATION harness (green, unchanged, before and after).
- **Flat names.** `internal/component/traffic/` and the `ze-show:traffic` wire method are
  already owned by QoS/traffic-control, so `traffic/stat` would nest under QoS. Kept
  `trafficstat` as the stat layer and added a sibling `trafficfeature`; regrouped nothing.
- **Severity is display-only in the CLI.** `computeSeverity`/`Severity` (a >2x/>5x verdict)
  was consumed ONLY by trafficstat's own `cmd`, never by a detection plugin. Removed it from
  the neutral aggregator; the CLI now computes an identical display severity from the neutral
  `Snapshot.History` facts via `stats.Mean`.
- **`trafficfeature` subscribes to `observation.Feed` directly** (a second consumer), NOT to
  `trafficstat.Snapshot`: fan-out and dest-port entropy need the raw 5-tuple stream, which
  the lossy top-N Snapshot cannot provide.
- **Built all 7 `core/stats` primitives** (Window, Mean, StdDev, Quantile, Entropy, EWMA,
  IntervalRegularity) even though Spec 1 only wires Window/Entropy/IntervalRegularity(+Mean/
  StdDev). EWMA+Quantile ship as foundational-library API for the Spec-2a anomaly detector's
  per-entity baseline. (Explicit user decision at the wiring-completeness fork.)

## Consequences

- `stats.Window` is now the canonical rolling-rate primitive; `trafficstat` and
  `trafficfeature` both build on it, and detectors (ddos, anomaly) can reuse the math.
- Neutral per-source features (fan-out, out/in ratio = exfil, dest-port entropy, new-peer,
  rare-port/proto, coarse beaconing) are surfaced by `show traffic-feature` and are the input
  the Spec-2a `anomaly/detect` judgment layer will consume.
- Coarse beaconing is bounded to periods of a few seconds by the 1s feed tick (Nyquist);
  `IntervalRegularity` returns 0 below a 2s floor. Finer detection needs a sub-second collector
  (out of scope).

## Gotchas

- **Concurrent sessions on main leave known-reds.** The `plugin/all` snapshot test and
  `ze-doc-test` were red from concurrent OSPF/ISIS work (unregistered-in-golden ospf wire
  methods; interop-scenario and release-gate-suite count drift), NOT from this change. Verify
  by scoping: with the full build tags only `ze-*ospf*` methods are "unexpected"; the doc
  source-anchor check ("all references valid") passes. Do not "fix" another session's golden.
- **The wire-methods snapshot needs a manual sorted insert.** `make generate` regenerates
  `all.go` blank imports but the `testdata/wire-methods.snapshot` golden is updated by inserting
  the new method in sorted order (regenerating under default tags would drop tag-gated entries).
- **Hook cadence during a leaf+component build:** every non-test `.go` needs a `// Design:`
  header; test-first is enforced (write `_test.go` before impl); test files need
  `VALIDATES:/PREVENTS:` headers; `goconst`/`intrange`/`unparam` fire on the first write.
  Pair each test with its impl file so the package compiles between steps.

## Files

None recorded.
