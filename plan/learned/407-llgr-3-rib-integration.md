# 407 -- LLGR RIB Integration

## Objective

Implement RIB-side LLGR support: LLGR_STALE community attachment, NO_LLGR route deletion, stale-level depreference in best-path selection, and generic RIB commands for community manipulation.

## Decisions

- `StaleLevel uint8` on RouteEntry instead of `LLGRStale bool` -- more general: 0=fresh, 1=GR-stale, 2+=LLGR-stale. `DepreferenceThreshold = 2` is the cutoff
- No LLGR-specific RIB commands -- used generic composable commands instead:
  - `rib attach-community <peer> <family> <community-hex>` -- attaches any community to stale routes
  - `rib delete-with-community <peer> <family> <community-hex>` -- deletes routes carrying a specific community
  - `rib mark-stale <peer> <restart-time> [level]` -- optional level parameter for LLGR (level=2)
- Best-path depreference is Step 0 (before RFC 4271 steps): routes at or above threshold lose to any route below it. Between two deprioritized, lower level wins. This preserves standard tiebreaking for non-stale routes

## Patterns

- Stale level as a graduated scale allows future extensions (level 3, 4, etc.) without protocol changes
- Generic community commands mean the GR plugin composes LLGR behavior from building blocks rather than the RIB knowing about LLGR specifically -- clean separation of concerns
- `ComparePair()` Step 0 ordering: GR-stale (level 1) beats LLGR-stale (level 2+), both lose to fresh (level 0)

## Gotchas

- Spec designed `rib enter-llgr` and `rib depreference-stale` as dedicated commands, but implementation used generic `attach-community` + `delete-with-community` + `mark-stale [level]` instead -- better design, more reusable
- Spec designed `LLGRStale bool` on RouteEntry but implementation uses `StaleLevel uint8` -- strictly more capable
- `Candidate.StaleLevel` field must be populated by `extractCandidate` for best-path to work
- GR-stale (level 1) is NOT deprioritized -- only level >= 2 (LLGR-stale) triggers depreference

## Files

- `internal/component/bgp/plugins/rib/storage/routeentry.go` -- StaleLevel field, DepreferenceThreshold constant
- `internal/component/bgp/plugins/rib/bestpath.go` -- Step 0 stale-level depreference in ComparePair/SelectBest
- `internal/component/bgp/plugins/rib/rib_commands.go` -- attach-community, delete-with-community, mark-stale [level]
- `internal/component/bgp/plugins/rib/bestpath_test.go` -- LLGR depreference tests
- `internal/component/bgp/plugins/rib/rib_gr_test.go` -- mark-stale with level tests
