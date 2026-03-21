# 382 — sendCtx Race Condition Fix

## Objective

Fix a data race on `Peer.sendCtx` between the FSM teardown goroutine (writer)
and plugin dispatch goroutines (readers via DirectBridge/route-server).

## Decisions

- Changed `sendCtx` from `*bgpctx.EncodingContext` to `atomic.Pointer[bgpctx.EncodingContext]`
- Matches the existing `negotiated atomic.Pointer` pattern on the same struct
- Chose atomic over RLock because `peer_initial_sync.go` calls `addPathFor()`/`asn4()` while already holding `p.mu.Lock()` — adding RLock inside would deadlock (Go RWMutex is non-reentrant)
- Removed RLock from `SendContext()` since atomic Load is sufficient
- Changed `sendUpdateWithSplit` signature from `(update, maxSize, family)` to `(update, maxSize, addPath)` — every caller already had addPath; re-computing it inside created a TOCTOU with the build step

## Patterns

- Fields read from multiple goroutines without the struct mutex must use `atomic.Pointer` — plain pointer + external lock only works when ALL readers hold the lock
- The `negotiated` field already demonstrated this pattern; `sendCtx` had the same access pattern but was missed
- `recvCtx` is safe: only accessed via locked exported methods, never from plugin dispatch

## Gotchas

- The race was intermittent — only triggered when a plugin (route-server) sends withdrawals via DirectBridge concurrently with a peer receiving a NOTIFICATION
- `peer_initial_sync.go` holds `p.mu.Lock()` while calling `addPathFor()`/`asn4()`, making RLock-based fixes impossible without restructuring all callers
- Functions receiving `sendCtx` as parameter (`toStaticRouteUnicastParams`, `buildStaticRouteUpdateNew`) already guarded `if sendCtx != nil` — no nil-safety issue from concurrent Store(nil)

## Files

- `internal/component/bgp/reactor/peer.go` — struct field type, accessor methods
- `internal/component/bgp/reactor/peer_send.go` — `sendUpdateWithSplit` takes `addPath bool` instead of `family`
- `internal/component/bgp/reactor/peer_initial_sync.go` — `.Load()` for parameter passing, `addPath` for split
- `internal/component/bgp/reactor/reactor_api_batch.go` — `addPath` for split call sites
- `internal/component/bgp/reactor/peer_test.go` — `.Store()`/`.Load()` in tests
- `internal/component/bgp/reactor/forward_update_test.go` — `.Store()` in tests
