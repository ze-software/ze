# 418 -- rib-family-ribout

## Context

`RIBManager.ribOut` was a flat map (peer -> routeKey -> Route) where the route key embedded the family prefix (`family:prefix:pathID`). Per-family operations (route refresh, LLGR readvertisement) required linear scans filtering by `rt.Family == family`. Additionally, four RIB command handlers (`rib clear out`, `rib clear in`, `rib retain-routes`, `rib release-routes`) had a pre-existing bug: they used the `peer` parameter from the execute-command RPC as the selector, but for plugin-dispatched commands this parameter is always `*`. The actual selector (e.g., `!192.168.1.1` from the GR plugin) landed in `args` and was silently discarded.

## Decisions

- Chose three-level map (peer -> family -> prefixKey -> Route) over tagged-route flat map, because it enables O(1) family lookup for route refresh and future LLGR readvertisement.
- Chose string keys for family (not `nlri.Family`) because ribOut operates on event data arriving as strings. No parsing overhead.
- Chose `outRouteKey` (prefix-only) over modifying shared `bgp.RouteKey`, because RouteKey is used by other code and persist has its own key format.
- Chose to fix the selector bug by extracting from args in the RIB plugin handlers, over modifying the engine's `extractPeerSelector` dispatcher. The fix is local to the RIB plugin.
- Made selector mandatory for all four affected commands (use `*` for all peers) rather than defaulting to `*` on empty args, because the silent-default was the root cause of the original bug.
- Persist plugin follows matching (not identical) pattern: same three-level map, but its `StoredRoute` has no PathID field (pre-existing gap with ADD-PATH, not introduced here).

## Consequences

- Per-family route refresh is now O(1) lookup instead of O(n) scan.
- LLGR spec-llgr-4-readvertisement can use `rib clear out !<peer> <family>` for per-family readvertisement.
- GR plugin's `rib clear out !<peer>` now correctly filters by peer (was silently resending to all peers before).
- All callers of `rib clear out`, `rib clear in`, `rib retain-routes`, `rib release-routes` must now pass a selector as the first arg. Existing GR plugin callers already did this (selector was in args, just ignored).
- The exabgp compat test and api-rib-clear-out.ci functional test were updated to pass `*` as mandatory selector.

## Gotchas

- The command dispatch model splits `rib clear out !192.168.1.1 ipv4/unicast` into command=`rib clear out`, peer=`*`, args=`[!192.168.1.1, ipv4/unicast]`. The peer param is only populated when the command string contains the literal `peer <addr>` keyword. This is not obvious and was the root cause of the selector bug.
- When changing a map type that's constructed in 20+ test locations, all tests break at once. Must update all test files in the same phase as the type change.
- Empty-map cleanup on withdrawal must handle multi-family events and multi-NLRI ops correctly. Go's nil-map semantics for `delete` and `len` make redundant cleanup operations safe (no-ops on nil maps).
- `outboundResendJSON` with a family filter can match a peer that has routes in other families but not the filtered one. Without the guard (`len(routesCopy) > 0`), the JSON response incorrectly reported the peer count.

## Files

- `internal/component/bgp/plugins/rib/rib.go` -- ribOut type, handleSent, handleRefresh, handleState, updateMetrics
- `internal/component/bgp/plugins/rib/rib_commands.go` -- selector fix, family filter, statusJSON
- `internal/component/bgp/plugins/rib/rib_pipeline.go` -- outboundSource nested iteration
- `internal/component/bgp/plugins/rib/event.go` -- outRouteKey helper
- `internal/component/bgp/plugins/persist/server.go` -- matching restructuring
- `test/plugin/rib-clear-out-family.ci` -- new functional test
- `docs/guide/command-reference.md`, `docs/architecture/api/commands.md` -- updated syntax
