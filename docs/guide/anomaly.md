# Behavioural Anomaly Detection

> **Pre-Alpha.** This page describes behaviour that may change.

Ze includes a report-only behavioural anomaly detector. It builds a per-source behavioural baseline, scores how far each source deviates from its own history and from its peers, and correlates weak signals into a single incident. It is separate from the volumetric DDoS subsystem, which reacts to raw packet and bandwidth rates. DDoS detects interface floods. Anomaly detection detects sources behaving unlike themselves and unlike their neighbours.

For the volumetric subsystem, see [DDoS detection and mitigation](ddos-mitigation.md).

## Traffic-feature facts layer

Both detectors read from a neutral facts layer. `trafficfeature` is a second, independent consumer of the observation feed that derives domain-neutral per-source feature signals. These are measurable facts rather than verdicts. Detection plugins apply judgement on top, and this layer never decides that traffic is an attack.

| Feature | Meaning |
|---------|---------|
| fan-out | Count of distinct destinations a source talked to in the window, used as a scanning or spread signal |
| out-in-ratio | Outbound bytes divided by inbound bytes for the source, used as an exfiltration signal; a pure sender with no inbound bytes is surfaced as `inf` |
| port-entropy | Shannon entropy, in bits, of the source's destination-port byte distribution; near 0 for a single port, higher for a scan |
| new-peer | The source was first observed within the recent new-peer window |
| rare-port | The source's dominant destination port is outside the well-known allowlist |
| beaconing | Interval-regularity score in [0,1] over the source's flow arrival times; higher is more clock-like and can indicate a C2 beacon |

The service is lazy and consumer-refcounted, so it costs nothing until a consumer attaches. It aggregates on a one-second tick and bounds memory with per-source cardinality and idle-eviction caps. Operators can read it directly:

```text
show traffic feature
show traffic feature 203.0.113.7   // one source
```

The output lists the top source IPs by activity with their feature vector and a `degraded` flag when no data has arrived yet.

## How detection scores a source

Each tick, the detector folds one traffic-feature snapshot into per-source, per-feature baselines and scores every source in two ways:

- **Self-deviation.** A z-score of the current value above the source's own running mean, in units of standard deviation, from an EWMA baseline. Below-baseline values score zero because only excess is anomalous. Each feature has a standard-deviation floor so a near-constant feature does not produce a divide-by-tiny-variance spike.
- **Cohort rarity.** How far the value sits above the source's cohort, other sources in the same source-prefix bucket. It is computed leave-one-out so an outlier is never compared against a baseline it dominates. Rarity is scored only once the cohort has at least `min-cohort-size` other members.

A continuous feature fires at the larger of its self-deviation and cohort-rarity score when that reaches `deviation-threshold`. The binary features, new-peer and rare-port, each contribute exactly one unit of evidence, enough to satisfy the correlation gate without dominating a continuous signal.

### Correlation into one incident

A source is anomalous for a tick only when at least `min-features-to-correlate` features fire together. The fired z-scores are fused into one incident score: the strongest at full weight plus a discounted sum of the rest through `corroboration-weight`. The score is capped so a benign multi-feature move cannot saturate into a critical incident. The incident carries a graded severity derived from the score.

### Confirm and clear lifecycle

An incident is confirmed only after `confirm-duration` consecutive anomalous ticks and cleared only after `clear-consecutive` consecutive normal ticks. This drives an `AnomalyDetected`, `AnomalyOngoing`, and `AnomalyCleared` event stream plus a bounded recent-incident ring.

## Poisoning and false-positive guards

**Freeze-learn.** The per-source baseline is updated with the current tick only when the source is not anomalous, or while it is still warming up. A sustained attack cannot drift the source's baseline upward until it looks normal again.

**No first-sight false positive.** A newly seen source has no baseline to deviate from. Self-deviation is suppressed until the baseline has accumulated a short warmup of samples. Early values seed the baseline before the detector can flag the source.

## Viewing anomaly incidents

```text
show anomaly detect
```

The command reports whether the detector is enabled and lists the recent-incident ring. Each incident includes the entity, cohort, fused score and severity, timestamp, and fired features with their z-scores.

## Shadow-first responder

The `anomaly-shape` responder can act on confirmed incidents. It defaults to shadow mode, where it logs the action it would take and installs nothing. Set it explicitly to `armed` before it can alter traffic.

```text
anomaly {
    shape {
        mode shadow;        // default; armed installs live per-source rules
        action limit;       // rate-limit (default) or drop
        limit-rate 1000;
        limit-unit second;
        auto-revert-ttl 300;
        blast-radius-cap 16;
        allowlist [ 192.0.2.0/24 ];
    }
}
```

When armed, it installs a per-source firewall rule, a rate limit by default or a drop, for the anomalous source. Its safety properties are:

- **Per-entity arming.** Each source is armed independently as a whole owner set. Re-arming an already armed source refreshes its timer without reinstalling or double-counting.
- **Timed auto-revert.** Every armed rule carries an `auto-revert-ttl`. The rule is withdrawn when the TTL elapses regardless of any clear event, and an ongoing incident extends the deadline. A cleared incident withdraws the rule early.
- **Blast-radius cap.** At most `blast-radius-cap` sources are armed at once. Beyond the cap, arming is refused and logged.
- **Kill switch.** Engaging the kill switch reverts every armed rule and forces the responder back to shadow.
- **Allowlist.** Sources in the allowlist are never armed.

Inspect the responder state with:

```text
show anomaly shape
```

The output reports the mode, configured action, kill-switch state, armed count, and armed sources.

## Configuration reference

### anomaly detect

| Parameter | Default | Range | Description |
|-----------|---------|-------|-------------|
| `enabled` | `false` | bool | Enable the detector |
| `deviation-threshold` | `3.0` | 1.0-100.0 | Sigma at or above which a feature fires |
| `min-features-to-correlate` | `2` | 1-6 | Features that must fire together to form an incident |
| `min-cohort-size` | `4` | 2-1024 | Minimum other cohort members before rarity is scored |
| `corroboration-weight` | `0.5` | 0.0-1.0 | Discount applied to corroborating features in the fused score |
| `confirm-duration` | `3` | 1-3600 | Consecutive anomalous ticks required to confirm an incident |
| `clear-consecutive` | `10` | 1-100 | Consecutive normal ticks required to clear an incident |
| `baseline-window` | `300` | 10-86400 | Baseline horizon in ticks, used to derive the EWMA alpha |
| `cohort-prefix-len-v4` | `24` | 8-32 | Source-prefix bucket length for IPv4 cohorts |
| `cohort-prefix-len-v6` | `48` | 16-64 | Source-prefix bucket length for IPv6 cohorts |

### anomaly shape

| Parameter | Default | Range | Description |
|-----------|---------|-------|-------------|
| `mode` | `shadow` | shadow, armed | `shadow` logs the would-be action; `armed` installs live rules |
| `action` | `limit` | limit, drop | `limit` rate-limits the armed source; `drop` drops it |
| `limit-rate` | `1000` | 1+ | Rate for the limit action |
| `limit-unit` | `second` | second, minute, hour, day | Rate unit |
| `limit-burst` | `0` | uint | Burst allowance |
| `auto-revert-ttl` | `300` | 5-3600 s | Safety ceiling; a rule is reverted this long after the last signal |
| `blast-radius-cap` | `16` | 1-1024 | Maximum number of concurrent live actions |
| `kill-switch` | `false` | bool | Revert all armed rules and force shadow mode |
| `allowlist` | none | list of prefixes | Protected sources that are never armed |

## See also

- [DDoS detection and mitigation](ddos-mitigation.md) covers the volumetric subsystem that shares the traffic-feature facts layer.
- [Firewall](firewall.md) covers the nftables backend used by the responder.
- [Monitoring](monitoring.md) covers Ze health, metrics, and operational visibility.

<!-- source: internal/component/trafficfeature/feature.go -- per-source facts and bounded snapshots -->
<!-- source: internal/plugins/anomaly/detect/detector.go -- scoring, confirmation, and freeze-learn lifecycle -->
<!-- source: internal/plugins/anomaly/shape/responder.go -- shadow-first response and safety controls -->
