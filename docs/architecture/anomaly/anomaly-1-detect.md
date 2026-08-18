# Anomaly Detection: Behavioral, Report-Only

The judgment layer of the behavioral security chain: facts from `trafficfeature`,
judgment here, response in [anomaly-shape](anomaly-2-shape.md). The chain is
proven end to end by [the integration harness](anomaly-4-interop-harness.md).
The fact layer below it is
[traffic analysis layers](../traffic/traffic-analysis-layers.md).

This is a SEPARATE domain from volumetric DDoS. It shares no event contract, no
detector and no responder with `ddos`. It is report-only: it emits incidents, and
`anomaly/shape` acts on them.

## The event contract names one of three entity kinds

<!-- source: internal/core/anomalyevent/event.go -- event structs, events.Register -->

`anomalyevent` mirrors `ddosevent`'s `events.Register[T](ns, et)` pattern under
namespace `anomaly-detect`. `events.Register` self-registers the namespace, so
package-level `var` declarations are the whole contract.

An event carries an `EntityKind` naming what it is about: a SOURCE (an anomalous
sender), a DEST (an anomalous receiver, a distributed sink or a probed host), or
a PORT. Source and dest are identified by `Entity`, a `netip.Prefix`. A port has
no address, so it is identified by `Port` and `Proto` and its `Entity` is zero.
The DDoS side stays keyed on a destination `VectorTuple`, which is the contrast
this section originally drew.

`EntityKindSource` is the ZERO value, deliberately: an event a producer built
before the other two kinds existed reads as a source event, which is what it
was. So a reader tests against `EntityKindSource` rather than against the other
kinds, and a kind added later does not silently join the source population.

## Baselines are two EWMAs per entity and feature

<!-- source: internal/plugins/anomaly/detect/detector.go -- featBaseline, onTick, scoreEntity -->

The detector reads `trafficfeature.Snapshot()` on its own one-second ticker,
because `trafficfeature` exposes a snapshot and not a subscribe callback.

Each (entity, feature) baseline is two `stats.EWMA` instances: one over the value
and one over the value squared. That gives a running mean and standard deviation
with no sample buffer.

## The scoring rule

<!-- source: internal/plugins/anomaly/detect/score.go -- zScore, cohortStats.rarity, combineScore -->

The rule is pinned and pure:

- `zScore` measures positive deviation only. The standard deviation has a
  per-feature floor, and the score is clamped at `zMax` = 10.
- `cohortStats.rarity` measures the same z against the source-prefix cohort, with
  a fallback for a tiny cohort.
- `combineScore` is `Zstrong + corroborationWeight * sum(others)`, capped at
  `scoreMax` = 30. It is not a naive sum. A continuous feature contributes
  `max(self, cohort)`. A binary feature (new peer, rare port) contributes exactly
  one threshold unit.

### Cohort rarity leaves the entity out

Mean and standard deviation are fragile to an outlier that sits inside its own
cohort. The first version scored an entity against a cohort that INCLUDED it, and
fed `finiteRatio(+Inf) = 1e6` into the cohort ratio distribution. An exfiltrating
host then dominated its own /24 baseline, masked its peers, and damped its own
rarity.

`rarity` is therefore leave-one-out, and a member with an infinite ratio is
excluded from the ratio cohort.

Cohort rarity is sparse in practice: few sources in the same /24 are active at
once.

## Freeze-learn: score first, fold later

<!-- source: internal/plugins/anomaly/detect/detector.go -- baselineUpdate, warmupTicks, onTick -->

`scoreEntity` scores WITHOUT mutating the baseline and returns the pending
`baselineUpdate` values. `onTick` folds them only when the entity is not anomalous
on this tick, or while the entity is still warming up (`warmupTicks` = 3).

Two failures this prevents:

- A sustained anomaly drifts the entity's own baseline upward until the anomaly
  looks normal. The first version updated before scoring and self-damped this way.
- A never-seen entity produces a self-deviation on first sight. With warmup, it
  has none until its baseline warms.

## The report surface

<!-- source: internal/plugins/anomaly/detect/show.go -- ze-show:anomaly handler -->

Plugins run in-process, so the detector sets a package global with
`setGlobalDetector`, and the `ze-show:anomaly` handler, registered with
`pluginserver.RegisterRPCs` in the same package, reads it. No cross-process
plumbing exists on this path.

The plugin emits `anomaly-detect` events and keeps a bounded recent-incident ring
that `show anomaly detect` surfaces. Metrics are
`ze_anomaly_incidents_total`, `ze_anomaly_active` and
`ze_anomaly_tracked_entities`.

`ze_anomaly_tracked_entities` carries a `dimension` label (`source`, `dest`,
`port`), one series per tracked map. It was a bare gauge until 2026-08-18, so a
query written against the unlabelled series returns nothing now and has to sum
over the label or select one dimension. That is a BREAKING change for an
existing dashboard, and it is stated here because nothing else would tell the
operator: the series does not disappear, it stops matching.

The doctor check
`anomaly-detect-feature-source` warns when the plugin is enabled and no flow
source feeds `trafficfeature`.

## Test reach

<!-- source: internal/plugins/anomaly/detect/detector.go -- confirm, clear, emit lifecycle -->

The emit path is not testable from a `.ci` without a traffic generator that can
synthesize feature deviations. The confirm, clear, emit and ring path is covered
by the unit lifecycle test, and `anomaly-show.ci` proves the wiring chain from
config through `RunEngine` and `setGlobalDetector` to show. The whole chain is
driven with real data by the in-process integration test described in
[the harness page](anomaly-4-interop-harness.md).
