# Spec: ddos-baseline-warmup-maturity

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-08-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The problem.** ze's ddos-detect threshold multiplier is constant from the
instant the baseline exists. `(*baseline).Threshold`
(`internal/plugins/ddos/detect/baseline.go`) returns
`max(p99Cache*multiplier, floor)`, and `multiplier` is written exactly once, in
`newBaseline`. No other writer exists in the package. So a baseline built from a
just-filled window is trusted exactly as much as one built from a week of
traffic.

**Why that matters.** `(*baseline).Ready` is `len(b.samples) >= b.window`, a
count and nothing else. A window filled during an unrepresentative period (a
nightly batch, a backup run, a burst that happened to coincide with startup)
yields a p99 that is skewed, and the multiplier applied to it is the same 3x
applied to a mature baseline. The result is false positives on a young baseline
and, in the opposite skew, a threshold high enough to blind the detector. ze
partly compensates with `StartupGrace` in `applyTick` (`detector.go`), but that
is a blind window that discards ticks outright rather than a graduated
confidence, and it ends abruptly.

**The goal.** Make the threshold a function of baseline maturity, not only of
p99. Concretely, three things this spec should decide and then build:

1. **A graduated multiplier.** A larger multiplier while the baseline is young,
   decaying to the configured value as the sample count grows past the window.
   The absolute floor must stay live throughout, so a graduated multiplier can
   never lift the threshold past the point where an unambiguous attack is
   missed. `applyTick` already has exactly this backstop shape in its
   `AbsoluteFloor*5` grace escape, and the same discipline applies here.
2. **A maturity signal on the type.** `(*baseline).Ready` answers a boolean
   where the caller often wants a degree. A maturity accessor (sample count, or
   a graded indicator) lets the responders, the show surface and any diagnostic
   distinguish "warming", "young", and "mature" instead of inferring it.
3. **Maturity that survives a restart.** `baselineState`
   (`internal/plugins/ddos/detect/baseline.go`) already persists `Count`, and
   `(*baseline).restore` already restores it as `max(st.Count, len(b.samples))`.
   Nothing reads it for maturity today, so the field is present and unused. That
   is the cheap half of this work.

**Not in scope, but adjacent.** A restored snapshot of any age currently counts
as mature. That is a defect and is tracked separately in
the retired deferral shard "fixit-ddos-baseline-lifecycle". It needs the same persisted
blob change (a timestamp, behind a `baselineStateVersion` bump), so the two
should probably land together even though only one of them is a defect.

**Constraint carried from the audit that produced this spec.** The poisoning
guard must not be weakened to make room for this. `(*baseline).Add` refuses any
sample while `applyTick` passes `attacking || above`, and that guard is
cross-metric by design: one boolean freezes both the PPS and the BPS baseline.
A graduated multiplier changes what counts as above, so it changes what the
baseline learns. That interaction is the main design risk here and the spec
must state what it does about it.

**Owning gate.** `go test -race ./internal/plugins/ddos/detect`, then
`./le functional plugin` for the thirteen `ddos-*.ci` fixtures.
