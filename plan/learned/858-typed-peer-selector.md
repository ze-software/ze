# 858 -- Typed Peer Selector

## Context

The BGPReactor interface passed peer selectors as `string` through 8 methods, forcing a parse-stringify-reparse cycle: plugins constructed typed `*selector.Selector`, stringified it to pass through the interface, and the reactor re-parsed it back. Four independent selector resolvers duplicated parsing logic. `SoftClearPeer` only supported IP/glob patterns, silently ignoring name/ASN selectors. The goal was to push the parse boundary outward to CLI/RPC entry points and pass typed selectors internally.

## Decisions

- Chose changing `BGPReactor` interface signatures (breaking internal) over keeping string params and parsing inside each method, because `ForwardUpdate` already proved the typed pattern works and the interface is internal
- Chose adding `*Sel` variant methods to SDK (`UpdateRouteSel`) over changing existing signatures, to avoid breaking external plugin compatibility (see learned 830). `DispatchCommandArgsSel` was initially added but removed during review: no plugin caller exists (GR passes selector in args, not peer param)
- Chose leaving GR plugin's `dispatchCommand` string-based over adding a typed dispatch variant, because GR passes the selector in `args[0]` (positional), not the `peer` parameter; the target command handler parses it at its own boundary
- Chose `ParseDefault` (fail to PeerName) over `ParseOrAll` (fail to All) for boundary parsing, because `ParseDefault` already existed and fail-closed is safer than fail-open
- Added `addr:port` parsing to `selector.parsePositive` because the reactor receives peer keys in `addr:port` format from some paths; without this, exclusion selectors like `!127.0.0.1:179` would fail to match

## Consequences

- `getMatchingPeers(string)` and `parseReactorSel` are deleted; all reactor paths use `getMatchingPeersSel(*selector.Selector)` directly
- `SoftClearPeer` now supports all 7 selector kinds (was IP/glob only); name and ASN selectors work for soft clear
- RR and RS withdrawal paths use `updateRouteSel` through DirectBridge, avoiding one stringify+reparse per withdrawal batch
- External plugins still use string-based JSON RPC; typed path is DirectBridge-only
- `filterPeersBySelectorValue` (peer CLI commands) now uses `selector.ParseDefault` and handles exclude/multi-addr selectors that it previously ignored
- Four independent selector resolvers consolidated to two patterns: `selector.ParseDefault` at boundaries, `getMatchingPeersSel` in the reactor

## Gotchas

- The fork agent updated `AnnounceNLRIBatch` and `WithdrawNLRIBatch` mock signatures but missed the other 6 methods (`AnnounceEOR`, `SendRoutes`, `SendRefresh`, `SendBoRR`, `SendEoRR`, `SoftClearPeer`), causing "BGP reactor not available" test failures from failed type assertions. Interface changes require checking every mock implementation, not just the primary callers.
- `parseReactorSel` handled `addr:port` format that `selector.Parse` did not. Removing it broke `TestGetMatchingPeersExclusion/exclude_with_port`. The fix was adding `netip.ParseAddrPort` to `selector.parsePositive`, which is a better home for the logic.
- GR plugin's exclude selector cannot use the typed SDK path because it flows through `DispatchCommandArgs` as `args[0]` (positional), not the `peer` parameter. This is intentional: the target handler (`clear bgp rib out`) expects the selector at that position.
- `DispatchCommandArgsSel` was added to the SDK and DirectBridge but had zero plugin callers. Review caught it as dead code. Lesson: verify wiring for new API methods before committing, not just that they compile.

## Files

- `internal/core/selector/selector.go` -- added `addr:port` parsing to `parsePositive`
- `internal/core/selector/selector_test.go` -- added `TestParseExclamationName`, `TestParseDefaultEmpty`, `TestParseDefaultStar`
- `internal/component/bgp/types/reactor.go` -- 8 methods changed from `peerSelector string` to `sel *selector.Selector`
- `internal/component/bgp/reactor/reactor_api.go` -- `SoftClearPeer` uses `getMatchingPeersSel`; removed `getMatchingPeers`, `parseReactorSel`
- `internal/component/bgp/reactor/reactor_api_batch.go` -- `AnnounceNLRIBatch`, `WithdrawNLRIBatch`, `SendRoutes` take typed selector
- `internal/component/bgp/reactor/reactor_api_forward.go` -- `AnnounceEOR`, `SendRefresh`, `SendBoRR`, `SendEoRR`, `sendRouteRefresh` take typed selector
- `internal/component/bgp/plugins/rr/rr.go` -- added `updateRouteSel`, withdrawal path uses it
- `internal/component/bgp/plugins/rs/server.go` -- added `updateRouteSel`
- `internal/component/bgp/plugins/rs/server_handlers.go` -- withdrawal path uses `updateRouteSel`
- `internal/component/bgp/plugins/cmd/peer/peer.go` -- `filterPeersBySelectorValue` uses `selector.ParseDefault`
- `pkg/plugin/sdk/sdk_engine.go` -- added `UpdateRouteSel`, `UpdateRouteSelWithMeta`
- `pkg/plugin/rpc/bridge.go` -- added `UpdateRouteSelHandler`, Set/Has/Call methods
- `internal/component/plugin/server/dispatch.go` -- registered typed selector bridge handlers, added `handleUpdateRouteSelDirect`
- `internal/component/bgp/plugins/cmd/update/update_text.go` -- parses selector at boundary
- `internal/component/bgp/plugins/cmd/commit/commit.go` -- parses selector at boundary
- `internal/component/bgp/plugins/route_refresh/handler/refresh.go` -- parses selector at boundary
- `internal/component/bgp/plugins/route_refresh/handler/clear_soft.go` -- uses `selector.Addr(addr)` directly
- 6 mock_reactor_test.go files updated to match new interface signatures
- `internal/component/bgp/reactor/peer_selector_test.go` -- uses `matchPeers` helper with typed selectors
- `internal/component/bgp/reactor/reactor_test.go` -- uses `matchPeers` helper
- `internal/component/bgp/reactor/reactor_batch_test.go` -- uses typed selectors in test calls
