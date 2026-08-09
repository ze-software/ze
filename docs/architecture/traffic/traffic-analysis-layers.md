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
| `internal/core/stats` | the math primitives: `Window`, `Mean`, `StdDev`, `Quantile`, `Entropy`, `EWMA`, `IntervalRegularity` |
| `internal/component/trafficstat` | per-key rolling aggregation built on `stats.Window` |
| `internal/component/trafficfeature` | neutral per-source features: fan-out, out/in ratio, destination-port entropy, new-peer, rare port and protocol, coarse beaconing |

<!-- source: internal/core/stats/window.go -- Window, the canonical rolling-rate primitive -->
<!-- source: internal/component/trafficstat/window.go -- per-key aggregation on stats.Window -->
<!-- source: internal/component/trafficfeature/feature.go -- neutral per-source feature aggregation -->

## The decisions

**Deep re-layering, not an additive library.** The per-key rolling window, rate,
history and eviction machinery was extracted out of `trafficstat`'s `entry` into
`stats.Window`, and `trafficstat`'s aggregator was rebuilt on it. Leaving
`window.go` intact and adding a library beside it was rejected: it buys no
reuse. The public `Snapshot` and `SubscribeRates` API stayed byte-identical, and
the existing `window_test.go` ran unchanged as a characterization harness before
and after.

**Flat names.** `internal/component/traffic` and the `ze-show:traffic` wire
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

**All seven `stats` primitives shipped together.** `EWMA` and `Quantile` have no
consumer in the stat and feature layers. They are the foundational API for the
anomaly detector's per-entity baseline, and shipping them with the rest avoided
a second pass over the package.

## The limit a reader must know

Coarse beaconing is bounded by the 1s feed tick. `IntervalRegularity` returns 0
below a 2s period, which is the Nyquist floor for that tick. Detecting a faster
beacon needs a sub-second collector.
<!-- source: internal/core/stats/beacon.go -- IntervalRegularity and its period floor -->
