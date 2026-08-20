# Deferrals: fixit-prefix-count-installed-unproven-against-a-real-peer

One issue, recorded not fixed (owner instruction, 2026-08-08). The aggregate
live backlog is folded on read from `plan/deferrals/` by `/ze-status`. Nothing
stores it (`ai/rules/planning.md`).

**Issue:** `prefix { count installed; }` has no interop coverage

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-08 | independent review of commit `2eb6a3dda` (the prefix-set counter), carried into the follow-up fix session | The `count installed` mode is proven only against the `ze-peer` harness. `test/interop/scenarios/bgp-max-prefix-cease-frr` and `bgp-max-prefix-per-family-frr` exercise a real FRR peer against the DEFAULT mode alone, so no scenario configures `count installed` at all. The mode's whole purpose is that a real peer churning attributes does not walk the count into its maximum, and attribute churn is what a live peer produces and a scripted harness does not. `applyInstalledPrefixSections` (`internal/component/bgp/reactor/session_prefix.go`) is the producer, and its immunity to a re-announcement is asserted today only by `TestPrefixCountInstalledIsImmuneToReannounce` and `test/plugin/prefix-count-installed-reannounce.ci`, both of which send the churn ze itself decides to send | `ai/rules/interop-and-goal-validation.md` requires an interop test for protocol behavior, and this is that gap rather than a defect: the mode is correct against every message shape tested, and the goal of the fix in hand (a refused message moves no installed set) holds without a new scenario. Adding one is separable work: it needs an FRR scenario that changes an attribute on a prefix it already advertises, holds the count still across that churn, and discriminates by reverting the family to `count offered` | needs a destination spec | deferred |
