# 1108 -- ddos-detect Enhancements (bandwidth trigger, baseline persistence, incident confidence)

## Context

The Flowtriq ftagent DDoS agent (our copy at `~/Code/github.com/Flowtriq/ftagent`,
v1.9.29-1.9.31) gained three detection improvements. ze already has a more mature
two-stage DDoS detector (`internal/plugins/ddos/detect`), but three of ftagent's ideas
mapped onto real gaps: (1) the rate trigger was PPS-only, so low-PPS/high-bandwidth
amplification (NTP/memcached/CLDAP) was missed even though `maxBps` was already carried
to the trigger; (2) the baseline was in-memory only, so every restart re-entered the
`StartupGrace` blind window and re-warmed over `BaselineWindow`; (3) the detector emitted
only a `Severity` grade, no composite confidence for observability/responders. The other
ftagent items (payload-byte signatures, HyperLogLog, velocity trigger, classification
vote-locking, NetFlow sampling-rate, Agones) do not fit ze's conntrack-flow + bounded-ring
architecture and were user-rejected.

## Decisions

- **BPS trigger as a SECOND `baseline` instance**, not a generalized metric-agnostic type
  or new fields on the existing struct. `d.baselineBps` reuses the whole tested `baseline`
  (poisoning guard, p99 recalc, `Ready()`) unchanged, so `baseline.go` needed zero trigger-
  path change. Trigger: `bpsAbove = bps-trigger-enable && baselineBps.Ready() && maxBps >
  baselineBps.Threshold()`, OR'd with the PPS path; both baselines fed with the same
  `attacking||above` guard.
- **`bps-floor` leaf in bits/s, converted internally.** ze's `iface` `RxBps` is bytes/s
  despite the name (`iface/rate.go` `rateDelta(RxBytes,...)`); operators think in Mbps. The
  leaf is bits/s (default 50 Mbps) and `newBaseline` divides by 8 to a bytes/s floor. Chose
  this over a bytes/s leaf (unconventional, error-prone) at the cost of one divide.
- **BPS trigger behind `bps-trigger-enable` (default true)** so amplification is caught out
  of the box but an FP-prone site can disable just the bandwidth path.
- **Persistence copies the traffic tc-snapshot idiom**: versioned JSON at
  `<config-dir>/state/ddos-detect-baseline.json`, atomic temp+rename; `baseline.restore`
  guards on version + `min(50,window)` samples + no NaN/Inf/negative. `newDetector` is kept
  I/O-free; `register.go` sets `statePath` and calls `restore()`, `Stop()` saves (fires on
  both reconfigure and shutdown) plus a periodic save every 300 ticks.
- **Confidence as an additive `AttackCharacterized.Confidence` (0-100)** via a pure
  `GradeConfidence` helper beside `GradeSeverity`, from signals already at the emit site
  (peak/threshold ratio, family specificity, source entropy) -- no new query, no duration
  (ze grades at attack start, not end like ftagent). Wired to all three consumers
  (user-approved): `observe` incident + `show ddos incidents`, `flowtriq` report,
  `local`/`flowspec` `confidence-min` gate (default 0 = unchanged behavior).

## Consequences

- `observe` and `flowtriq` now each subscribe to `Characterized` (they previously ignored
  it) solely to carry confidence; they still report the coarse `Family` from `AttackDetected`
  (strict scope -- the pre-existing coarse-Family gap is untouched).
- `local` `confidence-min` gates only the in-place narrowing: the coarse drop is installed on
  `AttackDetected`, which has no confidence, so confidence cannot suppress the initial on-host
  drop. `flowspec` `confidence-min` genuinely gates the upstream announce (its precise path is
  `onCharacterized`); the blackhole-fallback fast path stays ungated by design.
- Confidence attaches to an incident only when the victim prefix matches between Detected and
  Characterized; the flow-derived-victim case (trafficusage absent) leaves confidence 0.
- BPS peak tracking now updates `peakRxBps` independently of the PPS peak, so a low-PPS/high-
  BPS attack reports its true peak bandwidth (previously bps was sampled at the peak-pps tick).

## Gotchas

- `env.Get` caches `os.Environ` at first call, so `t.Setenv("ZE_CONFIG_DIR",...)` does not
  reach `baselineStatePath()` mid-run -- persistence tests inject `d.statePath` directly.
- Review caught two real issues, both fixed: (1) `saveBaselines` used a shared `path+".tmp"`,
  so a periodic save could collide with the on-stop save under a slow disk -- serialized with
  a per-detector `saveMu`; (2) `newDetector` setting `statePath` to the real config dir made
  unit tests that call `Stop()` write outside their tempdir -- `newDetector` is now I/O-free.
- Adding a YANG leaf to an existing module needs no codegen glue (embedded via `go:embed`);
  only new modules/plugins touch the composition root.
- Pre-existing, unrelated local-branch failures were confirmed NOT caused by this work:
  `config/cli` listener-conflict tests and `plugin/all` composition-root tests (missing
  isis/ldp/ospf/rsvp-te -- stale generated `all.go`, needs `make ze-regen`).

## Files

- `internal/core/ddosevent/event.go` -- `AttackCharacterized.Confidence` + `GradeConfidence`
- `internal/plugins/ddos/detect/`: `baseline.go` (snapshot/restore), `persist.go` (new),
  `detector.go` (BPS trigger, persistence lifecycle, `saveMu`), `config.go` + `yang/` (BPS
  leaves), `characterize.go` (compute confidence), `metrics.go` (`bps_trigger_total`),
  `register.go` (restore wiring)
- `internal/plugins/ddos/observe/` -- Characterized subscription + incident confidence
- `internal/plugins/ddos/flowtriq/` -- Characterized subscription + confidence in report
- `internal/plugins/ddos/{local,flowspec}/` -- `confidence-min` gate + YANG leaf
- `docs/guide/ddos-mitigation.md` -- config + behavior docs
