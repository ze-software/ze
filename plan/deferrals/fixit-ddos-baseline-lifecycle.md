# Deferrals: fixit-ddos-baseline-lifecycle

One issue per row, recorded not fixed (owner instruction, 2026-08-15). The
aggregate live backlog is folded on read from `plan/deferrals/` by `/ze-status`.
Nothing stores it (`ai/rules/planning.md`).

**Issue:** the ddos-detect rolling baseline latches against sustained legitimate
growth, trusts a snapshot of any age on restore, and evicts out of order after one

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-15 | ad-hoc (ddos detect audit) | **The baseline latches on legitimate permanent growth.** `(*baseline).Add` (`internal/plugins/ddos/detect/baseline.go`) returns before appending whenever its `attacking` argument is true, and `applyTick` (`detector.go`) passes `attacking` OR-ed with `above`, where `above` is `ppsAbove` OR-ed with `bpsAbove`. So no sample at or above the current threshold can ever enter the baseline. For a real attack that is exactly right, and it is the guard that stops an attack raising its own threshold. For a genuine step change in offered load (a new customer, a migrated service, a permanent traffic shift) it is a trap: `p99Cache` cannot rise, `(*baseline).Threshold` cannot rise, `above` stays true, and the detector stays in `stateActive` indefinitely against traffic that is not an attack. Nothing in the package breaks the cycle: no timeout, no ceiling on attack duration, no slow-adapt path. | The guard is load-bearing and must not simply be removed: doing so reintroduces threshold creep during a real attack, which is the defect it exists to prevent. The fix is a bounded escape (a maximum plausible attack duration after which the baseline resumes learning, or a slow secondary baseline that ignores the guard and arbitrates), which is a detection-semantics decision rather than a line fix. | `plan/spec-fixit-ddos-baseline-sustained-growth-escape.md` | deferred <!-- doc-links: ignore (the destination spec is written when the deferral is taken up) --> |
| 2026-08-15 | ad-hoc (ddos detect audit) | **A persisted baseline of any age restores as fully mature.** `persistedBaseline` (`internal/plugins/ddos/detect/persist.go`) is `{Version, Pps, Bps}` and `baselineState` (`baseline.go`) is `{Samples, Count, P99Cache}`. No timestamp is stored anywhere, and `statestore.Put` takes bytes with no mtime, so the age is not merely unchecked, it is not recoverable. `loadBaselines` validates presence, JSON and `Version == 1`; `(*baseline).restore` validates sample count and rejects NaN, Inf and negative values. Neither considers age. An appliance powered off for a month restores a month-old traffic profile and treats it as current, so the first tick after boot is judged against a threshold derived from traffic patterns that no longer hold, in either direction: a stale-low p99 false-positives, a stale-high p99 blinds. | Needs a `baselineStateVersion` bump because the blob shape changes, plus a decision on the staleness horizon and on what happens past it (warm fresh, or restore but mark immature). Coupled to the maturity work in `plan/future/spec-ddos-baseline-warmup-maturity.md`, which needs the same field. | `plan/spec-fixit-ddos-baseline-restore-staleness.md` | deferred <!-- doc-links: ignore (the destination spec is written when the deferral is taken up) --> |
| 2026-08-15 | ad-hoc (ddos detect audit) | **Eviction order is wrong for one window after a restore.** `(*baseline).Add` writes the ring at `b.count%b.window` once the window is full, which is correct FIFO while `b.samples` is filled in order. `(*baseline).restore` sets `b.samples` to the most recent `window` samples in chronological order but sets `b.count = max(st.Count, len(b.samples))`, which after a long run is a large total. The next `Add` therefore overwrites index `count%window`, an arbitrary mid-age slot, rather than the oldest sample. For one full window after each restore, samples retire out of order: some live past `window` ticks, others die early. | Low consequence and self-limiting: `(*baseline).recalc` sorts a copy, so p99 does not depend on slice order, and the effect ends once every sample post-dates the restore. It is still a correctness divergence between the two write paths, and it is cheapest to fix in the same edit as the row above, since both touch `restore`. | `plan/spec-fixit-ddos-baseline-restore-staleness.md` | deferred <!-- doc-links: ignore (the destination spec is written when the deferral is taken up) --> |

## Detail

**The latch is the row that matters.** The other two are hygiene; this one can
hold a production detector in a permanent false attack, and it does so on the
most ordinary event in a network, which is a customer growing. The reason it is
easy to miss in review is that the guard reads as obviously correct in
isolation, and it is: an attack must not raise its own threshold. The failure is
that "above threshold" is used as a proxy for "under attack", and those two
diverge exactly when the offered load has legitimately changed.

Note the interaction with the `stateClearing` path. `attacking` covers
`stateActive` and `stateClearing`, so the holddown window also freezes the
baseline. That is correct and should stay.

**Two shapes of escape, and the trade each makes.** A duration cap says no
genuine attack lasts longer than N, so past N the baseline resumes learning even
while `above` holds. It is simple and it is wrong for a sustained multi-hour
attack, where it would learn the attack as normal, which is the original defect
with a delay on it. A second slow baseline that always learns, and is consulted
only to answer "has the floor genuinely moved", costs one more sample buffer and
keeps the fast baseline's guard intact. The second is more machinery, so the
spec has to justify it against `ai/rules/simplicity.md` rather than assume it.

**What must not regress.** The guard is cross-metric by design: one boolean
freezes both the PPS and the BPS baseline, so a BPS-only spike cannot poison the
PPS baseline and vice versa. Any escape has to preserve that pairing, or the
weaker metric becomes the way in.

**Restore validation already present, and worth keeping.** `(*baseline).restore`
rejects a snapshot below `min(minRestoreSamples, window)` samples, rejects NaN,
Inf and negative values in both the samples and the cached p99, and honours a
shrunk `baseline-window` by keeping the most recent samples. It deliberately does
not persist window, multiplier or floor, so a config change applies on restore.
That is the right shape and the staleness field should join it rather than
replace it.

**Owning gate:** `make ze-test-pkg PKG=./internal/plugins/ddos/detect` for the
unit level, then `make ze-plugin-test` for the thirteen `ddos-*.ci` fixtures. A
latch fix needs a new fixture that drives a sustained step change and asserts the
detector returns to idle, which is the assertion that would fail today.
