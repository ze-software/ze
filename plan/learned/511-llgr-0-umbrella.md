# 511 -- LLGR umbrella

## Context

Ze had RFC 4724 Graceful Restart but no Long-Lived Graceful Restart (RFC 9494). When GR restart-time expired, all stale routes were purged unconditionally. Operators needing extended stale retention (hours/days) had no option. LLGR adds a second phase after GR: routes are retained with LLGR_STALE community and deprioritized in best-path, giving the restarting speaker much more time to recover.

## Decisions

- Extended bgp-gr plugin (not a new plugin), over separate llgr plugin -- LLGR is a single lifecycle with GR. "Delete the folder" test: if bgp-gr disappears, LLGR should too.
- Generic composable RIB commands (`attach-community`, `delete-with-community`, `mark-stale [level]`), over LLGR-specific commands (`rib enter-llgr`) -- RIB has no LLGR knowledge, cleaner separation of concerns.
- `StaleLevel uint8` with `DepreferenceThreshold = 2`, over `LLGRStale bool` -- more general (0=fresh, 1=GR-stale, 2+=LLGR-stale), supports future extensions.
- Static egress filter registration at FilterStageAnnotation with atomic fast-path, over dynamic registration at LLGR entry -- zero overhead when no peers are in LLGR (common case).
- EBGP non-LLGR: SetWithdraw (convert announce to withdrawal), over ModAccumulator withdrawal -- ModAccumulator lacks withdrawal mechanism. Acceptable for RFC 9494 "SHOULD NOT advertise."
- IBGP partial deployment via ModAccumulator ops (NO_EXPORT + LOCAL_PREF=0), matching OTC egress filter pattern.

## Consequences

- bgp-gr now registers both capability codes (64, 71) and references both RFCs (4724, 9494).
- Egress filter chain has two filters at FilterStageAnnotation: OTC and LLGR.
- Route.StaleLevel field added to bgp.Route -- serialized as `stale-level` in JSON, propagated to ribOut.
- `onSessionReestablished` returns `([]string, bool)` -- bool indicates LLGR state for active count.
- Generic RIB commands are reusable by future plugins needing community manipulation or stale-level control.
- AC-9 (non-LLGR peer suppression) only unit-tested -- multi-peer .ci infrastructure needed for full functional test.

## Gotchas

- `EgressFilterFunc` is registered at init() but grPlugin state doesn't exist until RunGRPlugin. Solved with `atomic.Pointer` to package-level state.
- `llgrActiveCount` must be decremented in three places: normal expiry, reconnect during LLGR (two call sites for structured/JSON event paths).
- Consecutive restart guard is critical: if a peer bounces again during LLGR, old LLST timer callbacks must be invalidated via owner check.
- LLGR capability has no global header unlike GR -- tuple count is `len(value) / 7`, not parsed from a header field.
- restart-time=0 with LLST>0 skips GR entirely -- state machine must handle this edge case without going through GR period.

## Files

- `internal/component/bgp/plugins/gr/gr_llgr.go` -- LLGR capability decode/encode/config/CLI
- `internal/component/bgp/plugins/gr/gr_egress.go` -- LLGR egress filter
- `internal/component/bgp/plugins/gr/gr_state.go` -- LLGR state machine extensions
- `internal/component/bgp/plugins/gr/gr.go` -- LLGR callbacks, cap parsing, egress state wiring
- `internal/component/bgp/plugins/gr/register.go` -- cap codes [64, 71], egress filter, community names
- `internal/component/bgp/plugins/rib/rib_commands.go` -- attach-community, delete-with-community, mark-stale [level]
- `internal/component/bgp/plugins/rib/storage/routeentry.go` -- StaleLevel, DepreferenceThreshold
- `internal/component/bgp/plugins/rib/bestpath.go` -- Step 0 stale-level depreference
- `internal/component/bgp/attribute/community.go` -- CommunityLLGRStale, CommunityNoLLGR
- `internal/component/bgp/route.go` -- StaleLevel field on Route
