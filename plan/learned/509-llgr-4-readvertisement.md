# 509 -- LLGR readvertisement

## Context

LLGR (RFC 9494) phases 1-3 implemented capability negotiation, state machine transitions, and RIB integration (LLGR_STALE attachment, NO_LLGR deletion, best-path depreference). Missing was the readvertisement behavior: when LLGR begins, stale routes must be re-advertised differently per destination peer depending on their LLGR capability. LLGR-capable peers receive routes unchanged. EBGP non-LLGR peers must not receive LLGR_STALE routes. IBGP non-LLGR peers get routes with NO_EXPORT and LOCAL_PREF=0 (partial deployment).

## Decisions

- Egress filter registered statically in GR plugin at FilterStageAnnotation (same as OTC), over registering dynamically at LLGR entry -- static registration with atomic fast-path bail out has zero overhead when no peers are in LLGR.
- Package-level `atomic.Pointer[egressFilterState]` set by RunGRPlugin, over closure capture -- EgressFilterFunc is a fixed-signature function value set at init() time before the plugin starts.
- `llgrActiveCount atomic.Int32` for fast-path, over checking peerLLGRCaps map -- one atomic load vs map iteration on every UPDATE to every peer.
- EBGP non-LLGR: `return false` (suppress) over ModAccumulator withdrawal -- ModAccumulator lacks a withdrawal mechanism. Explicit withdrawal happens when LLGR expires (purge-stale). Acceptable for RFC 9494 "SHOULD NOT advertise."
- IBGP partial deployment via `ModAccumulator.Op()` with community add (NO_EXPORT) and LOCAL_PREF set (0), matching the OTC egress filter pattern.
- Per-family `rib clear out !<peer> <family>` over all-family -- avoids resending unrelated families. Required changing `onLLGREntryDone` signature to include families list.
- Route.StaleLevel on ribOut propagated by markStaleCommand, over cross-referencing ribIn at resend time -- simpler, avoids binary-to-string key matching between ribIn storage and ribOut map.
- `extractLocalASN` added to GR plugin (same pattern as role plugin) for IBGP detection in egress filter.

## Consequences

- Egress filter chain now has two filters at FilterStageAnnotation: OTC and LLGR. Both use the same pattern (package-level state, closure-free).
- `onSessionReestablished` now returns `([]string, bool)` -- the bool indicates whether the peer was in LLGR, needed to decrement the active count.
- Route.StaleLevel field added to bgp.Route -- serialized as `stale-level` in JSON. All ribOut routes get marked stale during markStaleCommand (conservative: marks routes that may not be from the stale source).
- Functional .ci test not created -- requires multi-peer infrastructure that the test framework doesn't support. Egress filter behavior covered by 8 unit tests.

## Gotchas

- `EgressFilterFunc` is registered at init() but grPlugin doesn't exist until RunGRPlugin. Solved with `atomic.Pointer` to package-level state, same pattern role uses with `filterMu`-protected vars.
- PeerFilterInfo.LocalAS is populated by the reactor but the GR plugin captures its own localAS from OnConfigure, matching the OTC pattern of `getLocalASN()`. No dependency on reactor populating LocalAS.
- `llgrActiveCount` must be decremented in three places: onLLGRComplete (normal expiry), onSessionReestablished (reconnect during LLGR, two call sites for structured/JSON event paths).

## Files

- `internal/component/bgp/plugins/gr/gr_egress.go` -- LLGR egress filter, egressFilterState, staleFromMeta
- `internal/component/bgp/plugins/gr/gr_egress_test.go` -- 8 unit tests
- `internal/component/bgp/plugins/gr/register.go` -- EgressFilter + FilterStage registration
- `internal/component/bgp/plugins/gr/gr.go` -- egressState wiring, per-family onLLGREntryDone, localAS capture
- `internal/component/bgp/plugins/gr/gr_state.go` -- onLLGREntryDone signature with families, onSessionReestablished returns wasInLLGR
- `internal/component/bgp/plugins/gr/gr_llgr.go` -- extractLocalASN
- `internal/component/bgp/plugins/rib/rib.go` -- updateRouteWithMeta
- `internal/component/bgp/plugins/rib/rib_commands.go` -- markStale ribOut propagation, sendRoutes with meta
- `internal/component/bgp/route.go` -- StaleLevel field
