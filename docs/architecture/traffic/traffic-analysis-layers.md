# Traffic Analysis: facts, judgment, response

The traffic-analysis layer was one component that both measured traffic and
emitted a severity verdict. The DDoS plugins and the security `anomaly` family
both need the measurement, and neither wants the other's verdict. The layer was
split on one dividing principle.

**Analysis computes neutral FACTS. Detection plugins apply JUDGMENT and own the
RESPONSE.**

## The three parts

| Part | Owns |
|------|------|
| `internal/core/stats` | the math primitives: `Window`, `Mean`, `StdDev`, `Quantile`, `Entropy`, `EWMA`, `IntervalRegularity`, the `Hysteresis` confirm-and-clear state machine, and the `LevelHistory` of a sampled level with its growth pattern |
| `internal/component/trafficstat` | per-key rolling aggregation built on `stats.Window` |
| `internal/component/trafficfeature` | neutral features on three entity axes -- the sending address, the receiving address, and the destination service port: fan-out or fan-in, out/in ratio, destination-port or per-source entropy, new-peer, rare port and protocol, coarse beaconing, plus the entity's origin AS when a publisher supplies it |

<!-- source: internal/core/stats/window.go -- Window, the canonical rolling-rate primitive -->
<!-- source: internal/component/trafficstat/window.go -- per-key aggregation on stats.Window -->
<!-- source: internal/component/trafficfeature/feature.go -- neutral feature aggregation on the source, dest and port axes -->

## The decisions

**Deep re-layering, not an additive library.** The per-key rolling window, rate,
history and eviction machinery was extracted out of `trafficstat`'s `entry` into
`stats.Window`, and `trafficstat`'s aggregator was rebuilt on it. Leaving
`window.go` intact and adding a library beside it was rejected: it buys no
reuse. The public `Snapshot` and `SubscribeRates` API stayed byte-identical, and
the existing `window_test.go` ran unchanged as a characterization harness before
and after.

**Flat names.** `internal/component/traffic` and the `ze-traffic:show-traffic` wire
method already belong to QoS and traffic control, so a `traffic/stat` child
would have nested the stat layer under QoS. `trafficstat` kept its name and
`trafficfeature` was added beside it.

**Severity is display-only, computed in the CLI.** The old `computeSeverity`
verdict had exactly one consumer, `trafficstat`'s own command handler, and no
detection plugin ever read it. It left the neutral aggregator. The CLI derives
the same display severity from `Snapshot.History` through `stats.Mean`.
<!-- source: internal/component/trafficfeature/cmd/traffic_feature.go -- show traffic feature handler -->

**`trafficfeature` subscribes to `observation.Feed` directly.** It is a second
consumer of the raw 5-tuple stream, not a consumer of `trafficstat.Snapshot`.
Fan-out and destination-port entropy need every tuple, and the lossy top-N
snapshot cannot supply them.
<!-- source: internal/component/trafficfeature/service.go -- observation.Feed subscription -->

**Origin AS rides the feed; the feature layer never resolves it.** A
`FeatureEntry` carries `SrcAS`, the origin autonomous system of its entity, and
`trafficfeature` takes it from `Observation.SrcAS` in the source-role branch of
`ingest` alone: in the destination-role branch that field describes the other
endpoint. It is an entity property rather than a window measurement, so it lives
in the persistent part of `addrState`, survives the per-window reset, and is
overwritten only by a non-zero value, which keeps a later unattributed flow from
erasing a known AS. `SrcAS == 0` means "not attributed" (AS 0 is reserved,
RFC 7607) and a consumer grouping by AS falls back to the address or its prefix.
The resolver stays in the publishing plugin: this layer copies a label, so
neither it nor a consumer of `Snapshot` imports the producer.
<!-- source: internal/component/trafficfeature/service.go -- FeatureEntry.SrcAS -->
<!-- source: internal/component/trafficfeature/feature.go -- source-role-only AS stamp in ingest -->

**All seven `stats` primitives shipped together.** `EWMA` and `Quantile` have no
consumer in the stat and feature layers. They are the foundational API for the
anomaly detector's per-entity baseline, and shipping them with the rest avoided
a second pass over the package.

## The shared confirm-and-clear machine

`stats.Hysteresis` is the confirm-and-clear state machine detectors share. At
each evaluation the detector decides whether its condition holds and passes
that answer to `Tick`. The condition is confirmed after `confirmAfter`
consecutive evaluations that hold, and cleared after `clearAfter` consecutive
evaluations that fail. One failing evaluation sends a confirming machine back
to idle, and one holding evaluation sends a clearing machine back to active, so
a single noisy sample neither raises an alarm nor drops one. The states are
idle, confirming, active and clearing. A confirm count of 0 or 1 confirms on
the first evaluation that holds, and a clear count below 1 reads as 1.

The machine counts answers and nothing else. The detector keeps its threshold,
its counts and its response, and the machine holds no clock. It began as
ddos-detect's own state machine and moved here to be shared. ddos-detect drives
it from `applyTick`: `OnConfirmed` starts the attack and `OnCleared` ends it.
<!-- source: internal/core/stats/hysteresis.go -- Hysteresis, NewHysteresis, Tick -->
<!-- source: internal/plugins/ddos/detect/detector.go -- newDetector wires the callbacks, applyTick feeds Tick -->

## The level history and the growth pattern

`stats.LevelHistory` keeps the history of a **level**, how much of a resource
is used, which is not a rate: a level legitimately falls, so it is kept as it
falls where `Window` clamps a decrease. The caller passes each sample with its
time, and the history never reads the clock. Samples are bucketed by period
from the first sample on; a closed bucket holds the minimum and the last sample
of its period, the newest `BucketCount` buckets are kept, and a period with no
sample is recorded as missing. `Add` says when a sample closed a bucket, which
is when the caller judges.

A clock step closes nothing early or twice. A sample dated before the open
bucket's start joins the open bucket. A sample a period or more past it closes
the open bucket and records the skipped periods as missing, at most
`BucketCount` of them however far the clock jumped.

`Growth` judges the pattern of a leak. The newest `GrowthRises`+1 closed
buckets must all be observed, each minimum must exceed the one before, the
minimum must rise by at least the amount the caller passes, and a least-squares
line fitted to those minimums must reach the limit the caller passes within
`GrowthHorizon`. The minimum is the floor of a period: a leak raises it, and a
burst raises only the peak. The fitted line keeps growth that settles from
holding. The minimum rise and the limit are levels passed at each judgment,
because both can move and the package stays domain-neutral.

A missing bucket ends a run, so growth is never bridged across a period nobody
observed. The answer is a `GrowthOutcome`, so "not enough observed history"
(`GrowthWarmingUp`) never reads as "no growth" (`GrowthNotRising`,
`GrowthRiseTooSmall`, `GrowthBeyondHorizon`), and a limit already reached
(`GrowthLimitReached`) never reads as a zero time to exhaustion. `GrowthHolds`
reports the rise, the span and the projected time to exhaustion. Figures the
history cannot work with are refused at `NewLevelHistory`, and a limit that is
not positive, or a minimum rise that is negative, is refused at `Growth`.
`ObservedRun` says how many consecutive observed buckets end at the newest,
which is how long the level has been watched.
<!-- source: internal/core/stats/level.go -- LevelParams, LevelHistory, NewLevelHistory, Add, ObservedRun, Growth, GrowthOutcome -->

## The limit a reader must know

Coarse beaconing is bounded by the 1s feed tick. `IntervalRegularity` returns 0
below a 2s period, which is the Nyquist floor for that tick. Detecting a faster
beacon needs a sub-second collector.
<!-- source: internal/core/stats/beacon.go -- IntervalRegularity and its period floor -->
