# Deferrals: fixit-installed-count-memory-magnitude-undisclosed

One issue, recorded not fixed (owner instruction, 2026-08-08). The aggregate
live backlog is folded on read from `plan/deferrals/` by `/ze-status`. Nothing
stores it (`ai/rules/planning.md`).

**Issue:** the guide states the memory bound of `count installed` without its magnitude

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-08 | independent review of commit `2eb6a3dda` (the prefix-set counter), carried into the follow-up fix session | `docs/guide/configuration.md` says `installed` "keeps one entry per prefix for that family, so it costs memory in proportion to what the peer sends, bounded by `maximum`", and its own worked example is `maximum 1000000` with `count installed`. The set is `prefixCounts.sets` (`internal/component/bgp/reactor/session_prefix.go`), a `map[string]struct{}` keyed on each NLRI's wire encoding, so that example is roughly 60 to 80 MB per family per peer at steady state. An operator who copies the example onto four peers with two families each is committing to a number the page never names. The bound is disclosed; the magnitude is not | It is a documentation gap over correct code, not a defect: the bound holds, `applyInstalledPrefixSection` stops inserting at `maximum+1`, and the goal of the fix in hand is unaffected. Fixing it is separable and needs a MEASUREMENT rather than the arithmetic above: the per-entry cost depends on the Go map's bucket layout and on the NLRI key length, which differs by family, so the honest page states a measured figure for one named family and says how it scales. **Triaged 2026-08-30 as an improvement, not a release defect:** the code is correct and the bound is disclosed, so this is documentation alone | `plan/future/` (needs its own spec) | deferred |
