# DDoS Detection: Bandwidth Trigger, Baseline Persistence, Confidence

Three additions to the detector described in
[the umbrella page](cp-survival-5-detect-0-umbrella.md). Each closed a real gap:
a PPS-only trigger missed low-PPS amplification, an in-memory baseline re-entered
the startup blind window on every restart, and a severity grade alone gave
responders no measure of how sure the detector was.

## The bandwidth trigger is a second baseline instance

<!-- source: internal/plugins/ddos/detect/detector.go -- baselineBps, onRate trigger -->
<!-- source: internal/plugins/ddos/detect/baseline.go -- Add, Threshold, Ready -->

`d.baselineBps` is another `baseline`, not a metric-agnostic type and not new
fields on the existing struct. It reuses the tested poisoning guard, the p99
recalculation and `Ready()` unchanged, so `baseline.go` needed no change on the
trigger path.

The trigger is
`bpsAbove = bps-trigger-enable && baselineBps.Ready() && maxBps > baselineBps.Threshold()`,
ORed with the PPS path. Both baselines are fed with the same
`attacking || above` guard.

`bps-trigger-enable` defaults to true, so amplification (NTP, memcached, CLDAP)
is caught out of the box, and a site with false positives can disable the
bandwidth path alone.

`peakRxBps` updates independently of the PPS peak, so a low-PPS high-bandwidth
attack reports its true peak bandwidth. It was previously sampled at the peak-PPS
tick.

### `bps-floor` is in bits per second

<!-- source: internal/component/iface/rate.go -- rateDelta, RxBps -->

The iface `RxBps` field is BYTES per second despite the name. Operators think in
Mbps, so the leaf is bits per second with a 50 Mbps default, and `newBaseline`
divides by 8 to reach a bytes-per-second floor. A bytes-per-second leaf was
rejected as unconventional and error-prone. The cost is one divide.

## Baseline persistence

<!-- source: internal/plugins/ddos/detect/persist.go -- saveBaselines, loadBaselines -->
<!-- source: internal/plugins/ddos/detect/baseline.go -- snapshot, restore -->

The format copies the traffic tc-snapshot idiom: versioned JSON at
`<config-dir>/state/ddos-detect-baseline.json`, written to a temp file and
renamed. `baseline.restore` refuses a blob whose version differs, that holds
fewer than `min(50, window)` samples, or that holds a NaN, an infinity or a
negative.

`newDetector` is I/O-free by design. `register.go` sets `statePath` and calls
`restore()`. `Stop()` saves, which covers both reconfigure and shutdown, and a
periodic save runs every 300 ticks.

`saveMu` serializes the periodic save against the save on stop. Both wrote the
same `path + ".tmp"`, so a slow disk let them collide.

## Confidence

<!-- source: internal/core/ddosevent/event.go -- GradeConfidence, GradeSeverity -->

`AttackCharacterized.Confidence` is 0 to 100, computed by the pure
`GradeConfidence` helper beside `GradeSeverity`, from signals already present at
the emit site: the peak-to-threshold ratio, family specificity, and source
entropy. It adds no query and does not use duration, because ze grades at attack
start and not at attack end.

Three consumers read it: the `observe` incident and `show ddos incidents`, the
`flowtriq` report, and the `confidence-min` gate on `local` and `flowspec`
(default 0, which is the previous behavior).

### What confidence can and cannot gate

- `local`'s `confidence-min` gates only the in-place narrowing. The coarse drop is
  installed on `AttackDetected`, which carries no confidence, so confidence cannot
  suppress the initial on-host drop.
- `flowspec`'s `confidence-min` gates the upstream announce, because its precise
  path is `onCharacterized`. The blackhole fallback stays ungated by design.
- Confidence attaches to an incident only when the victim prefix matches between
  `Detected` and `Characterized`. The flow-derived-victim case, where trafficusage
  is absent, leaves confidence 0.

`observe` and `flowtriq` subscribe to `Characterized` only to carry confidence.
They still report the coarse `Family` from `AttackDetected`.

## Traps

- `env.Get` caches `os.Environ` at the first call, so `t.Setenv("ZE_CONFIG_DIR", ...)`
  does not reach `baselineStatePath()` mid-run. A persistence test injects
  `d.statePath` directly.
- Adding a YANG leaf to an existing module needs no codegen glue, because the
  module is embedded with `go:embed`. Only a new module or a new plugin touches
  the composition root.

## Rejected

The other ftagent ideas do not fit ze's conntrack-flow plus bounded-ring
architecture and were rejected by the owner: payload-byte signatures,
HyperLogLog, the velocity trigger, classification vote-locking, the NetFlow
sampling rate, and Agones.
