# Deferrals: fixit-ddos-detection-coverage

One issue per row, recorded not fixed (owner instruction, 2026-08-15). The
aggregate live backlog is folded on read from `plan/deferrals/` by `/ze-status`.
Nothing stores it (`ai/rules/planning.md`).

**Issue:** ddos-detect is blind to bandwidth attacks during startup grace, and
one declared attack family has no producer

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-15 | ad-hoc (ddos detect audit) | **Startup grace is escaped by packet rate only, so a bandwidth attack in that window is invisible even when the bandwidth baseline is warm.** `applyTick` (`internal/plugins/ddos/detect/detector.go`) opens with `if d.tickNum <= d.cfg.StartupGrace { if maxPps < d.cfg.AbsoluteFloor*5 { return d.drainPending() } }`. The bandwidth trigger is computed after that block, so during grace `bpsAbove` is never evaluated at all. The escape hatch tests packet rate alone. Amplification is precisely the class that is low PPS and high bandwidth, which is why the BPS trigger exists, so the one attack shape the grace window cannot see is the one the BPS path was added to catch. It bites hardest after a restart, because `(*baseline).restore` can make `baselineBps.Ready()` true at tick 1 while grace still discards the next 90 ticks (`start-up-grace` default). A warm, trustworthy bandwidth baseline is present and ignored. | The grace window itself is correct and deliberate: `applyTick`'s own comment records that the BPS trigger is gated on a ready BPS baseline "so it never fires during warm-up (bandwidth is more FP-prone than packet rate)". The defect is narrower than that intent, and the fix must not widen it: adding a bandwidth term to the grace escape is only safe when the BPS baseline is genuinely `Ready()`, which after a cold start it is not. Deciding the exact condition is a detection-semantics call. | `plan/spec-fixit-ddos-startup-grace-bps-escape.md` | deferred <!-- doc-links: ignore (the destination spec is written when the deferral is taken up) --> |
| 2026-08-15 | ad-hoc (ddos detect audit) | **`FamilyFragFlood` is declared and never produced.** `internal/core/ddosevent/event.go` declares `FamilyFragFlood AttackFamily = "fragment-flood"` alongside the five families that do have producers. No non-test site anywhere in the tree assigns it. `classifyFlows` (`internal/plugins/ddos/detect/characterize.go`) cannot produce it: its own doc records that defragmentation runs before conntrack, so a fragment flood arrives already reassembled and falls to `FamilyGenericFlood`. The constant is a promise the code does not keep. Anything reading the family set (an operator, a dashboard, a responder policy keyed on family) is told ze classifies fragment floods, and it does not. | Either direction is defensible and the choice is the owner's: delete the constant and stop claiming the capability, or implement the classifier against pre-defrag counters. It is not a silent-wrong-answer defect, because generic-flood is an honest classification of what conntrack sees. Deleting is a one-line change plus whatever reads the family list; implementing needs a counter source upstream of defrag, which the current `flowRecord` does not carry. | `plan/spec-fixit-ddos-frag-flood-family.md` | deferred <!-- doc-links: ignore (the destination spec is written when the deferral is taken up) --> |

## Detail

**The grace blind spot, stated precisely.** Three conditions have to coincide,
and after a restart all three routinely do. `d.tickNum <= d.cfg.StartupGrace`
holds for the first 90 ticks by default. `maxPps < d.cfg.AbsoluteFloor*5` holds
for any amplification flood, because amplification is low packet rate by
definition. And `baselineBps.Ready()` can be true from tick 1 because
`(*baseline).restore` rehydrates the bandwidth baseline from the persisted blob.
So the detector holds a warm bandwidth baseline, the traffic is above its
bandwidth threshold, and the tick returns before the comparison is ever made.

The window is 90 seconds at the default `check-interval`. An amplification flood
that starts during a reboot, or that triggers the reboot, gets that long
unopposed.

**Why the obvious fix is wrong.** Adding `|| maxBps >= d.cfg.BpsFloor*5` to the
grace escape reintroduces the false positives the grace window exists to
suppress, because on a cold start the BPS baseline is empty and the floor is the
only term. The condition has to carry `d.baselineBps.Ready()`, which is exactly
the gate the main path already uses. The narrow reading of the fix is: during
grace, evaluate the bandwidth path if and only if the bandwidth baseline is
ready, and let it escape grace on the same terms the packet path does.

**A question the spec must answer.** `AbsoluteFloor*5` is the packet-path escape
multiplier. Whether the bandwidth escape should use the same 5x against
`bps-floor`, or the ordinary `Ready()` threshold, is not obvious. 5x is a
deliberate "unambiguously real" bar for an unwarmed detector; a ready baseline
arguably does not need it.

**On the dead family.** Worth checking against the same question for the other
five before deciding: `FamilyUDPFlood`, `FamilySYNFlood`, `FamilyICMPFlood` and
`FamilyReflection` all have producers in `classifyFlows`, and
`FamilyGenericFlood` is the fallback. Fragment flood is the only orphan, so this
is a single gap rather than a pattern.

**Owning gate:** `make ze-functional-plugin-test` covers both, with
`test/plugin/ddos-bps-amplification.ci` the closest existing fixture to the grace
row. It does not currently exercise the grace window, so the fix needs either a
new fixture or an extension that starts bandwidth-only traffic inside grace with
a restored baseline, which is the assertion that would fail today.
