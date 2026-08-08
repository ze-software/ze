# Deferrals: fixit-prefix-config-lookup-scans-per-update

One issue, recorded not fixed (owner instruction, 2026-08-08). The aggregate
live backlog is folded on read from `plan/deferrals/` by `/ze-status`. Nothing
stores it (`ai/rules/planning.md`).

**Issue:** `prefixConfigLookup` runs twice per UPDATE for each installed announce family

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-08 | independent review of commit `2eb6a3dda` (the prefix-set counter), carried into the follow-up fix session | `prefixConfigLookup` (`internal/component/bgp/reactor/session_prefix.go`) walks `s.settings.PrefixMaximum` and converts each key with `familyKeyString` until it matches. An installed announce section calls it once from `applyInstalledPrefixSection`, to read the maximum before it inserts, and the settle loop then reaches it again through `applyPrefixCheck` or `applyPrefixDelta`. That is two linear scans per UPDATE per installed announce family, on the receive path. The reviewer measured 237 ns/op against the offered mode's 185 | It is a cost, not a defect: the answer is correct, the allocation ceilings in `internal/perf/allocgate.go` still hold at 0 allocs/op for the steady state, and the goal of the fix in hand is unaffected. Removing it is separable work with a design choice attached: the map is keyed by `"afi/safi"` strings and the wire path is keyed by `uint32`, so the fix is to resolve each family's maximum and warning ONCE per session into the `prefixCounts` struct that already carries the per-family `installed` set, which then also removes the scan from the offered path | needs a destination spec | deferred |
