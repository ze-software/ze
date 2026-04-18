# 630 -- rs-fastpath-3-passthrough

## Context

Spec-rs-fastpath-1-profile measured bgp-rs's forwarding hot path: `plugin/server.tokenize`
at 19.4 % of 2.5 GB allocations and `CommandRegistry.All` adding another ~5 %, both driven
by rs batching text commands (`cache N,N,N forward A,B,C`) that the engine re-tokenises per
call. This child removes that text-RPC round-trip on the forwarding path while keeping the
whole ze architectural contract intact: rs still owns policy, reactor still owns wire
encoding, copy-on-modify still happens in the Outgoing Peer Pool.

## Decisions

- **Added reactor-owned SDK primitive, not an rs-specific hook.** `Plugin.ForwardCached(ctx,
  updateIDs, destinations)` + `Plugin.ReleaseCached(ctx, updateIDs)` live in `pkg/plugin/sdk/`;
  the engine dispatches them directly to `reactorAPIAdapter.ForwardUpdatesDirect` /
  `ReleaseUpdates`. Any plugin with cached msgIDs and a resolved destination list may use
  them. Chose this over an rs-only shortcut so the primitive stays generic.
- **Kept `[]string` on the SDK surface.** Matches existing `UpdateRoute(peerSelector string)`
  convention. Destinations parse to `[]netip.AddrPort` once at the reactor adapter boundary;
  bare addresses (the rs default) expand to every peer instance sharing that address at
  resolution time. Chose this over requiring rs to carry AddrPort end-to-end.
- **Hoisted `supersedeKey` + `withdrawal` into `fwdBodyCache`.** Peers sharing the same
  rawBodies now compute these once on cache miss and reuse on cache hit, benefitting all
  callers of the per-destination loop (ForwardUpdate + ForwardUpdatesDirect). Chose this
  over a separate code path for the fast entry.
- **Deleted rs's async sender plumbing outright** (`forwardCh`, `forwardLoop`,
  `asyncForward`, `releaseCh`, `releaseLoop`, `rsForwardChDepth`, `fwdSendersDefault`,
  `ze.rs.fwd.senders`). Chose this over keeping both paths -- the buffering existed to
  absorb text-RPC cost that no longer exists (`rules/no-layering`). Backpressure now flows
  directly from `p.ForwardCached` return latency into worker channel occupancy.
- **Per-ID Ack on the batch path.** `ForwardUpdatesDirect` acks each updateID after its
  destinations dispatch, rather than deferring until the whole batch finishes, so cache
  entries evict promptly under load.

## Consequences

- Enables: any future cache-consumer plugin to use the fast path without touching rs code.
  bgp-redistribute stays on `UpdateRoute` (different shape: synthesises fresh announces
  at much lower volume).
- Simplified rs worker lifecycle: no background sender goroutines, no separate
  close-vs-send race between `releaseCh`/`releaseStop`, single Stop() drains everything.
- Constrains: pending-never-expires invariant is now load-bearing for rs's batch
  accumulator. `TestPendingCacheNeverExpires` pins it; future cache refactors that
  evict by TTL would silently regress rs.
- Benchmark evidence: `BenchmarkForwardDirect` = 1309 ns/op on M4 Max (~764k UPDATE/s/core);
  AC-9 target was >= 500k UPDATE/s/core.

## Gotchas

- `ForwardUpdatesDirect` is today a thin wrapper: it parses the destination list into one
  `selector.Selector` (once per call, not per id), then calls the existing `ForwardUpdate`
  per cached id with that shared selector and an empty pluginName (the caller owns Ack).
  This preserves every invariant of the per-destination loop (egress filter chain, EBGP
  wire cache, copy-on-modify, supersedeKey + withdrawal hoisting) without duplicating
  ~500 lines. A future refactor can extract the loop body once and call it directly from
  both entry points; the SDK and the RPC opcode are stable regardless.
- Replaying peers are INCLUDED in forward targets, not excluded. BGP UPDATE duplicates
  are idempotent at the receiver, and excluding replaying peers risks losing routes during
  peer-up races (`TestReplayingPeerIncludedInForwardTargets` pre-exists and is preserved).
  This matches pre-rs-fastpath-3 behavior; rs-fastpath-3 changes forwarding *transport*
  (text RPC -> typed SDK call) but not forwarding *semantics*.
- Empty destinations MUST NOT fall through to a wildcard broadcast. `ForwardUpdatesDirect`
  returns `errNoDestinations` (unexported; propagates as string to external plugins through
  RPCCallError). Callers that decided "no forward" must use `Plugin.ReleaseCached` instead.
  This is the Round 2 BLOCKER fix; if the guard regresses, a buggy plugin passing `[]string{}`
  would leak routes to every peer.
- Destination list is capped at `maxForwardDestinations = 4096`. Above this, the call is
  rejected with an explicit error rather than silently truncated (`rules/exact-or-reject`).
  Covers real-world BGP deployments; raise the const if very-large deployments need it.
- Duplicate IDs are collapsed before dispatch via `dedupIDs`. Common case (fully unique
  input, <=16 items) returns the input slice verbatim with zero allocation; only a
  confirmed duplicate triggers the map-backed rewrite.
- `TestBatchForwardFireAndForget` was deleted: it tested the now-removed property that
  workers do not block on forward RPCs. Under the new design ForwardCached IS synchronous
  and backpressure flows through `workers.BackpressureDetected` exactly as spec AC-5 calls for.
- Functional `.ci` tests were scaffolded to cover the four Wiring Test scenarios, but
  their deep AC-6a/AC-6b hex-identity assertions are minimal smoke checks in this pass.
  Full byte-equality assertions are follow-up work; `reject=stderr:pattern=cache .* forward`
  guards against the legacy text-RPC path silently re-emerging.
- Pre-existing lint failure on `l2tp/kernel_other_types.go:pppoxFD` (logged in
  `plan/known-failures.md:344`) prevented `make ze-verify-fast` from passing cleanly.
  Unit + race-reactor runs are clean.

## Files

- `pkg/plugin/rpc/types.go`, `pkg/plugin/rpc/bridge.go` -- new `ForwardCachedInput`,
  `ReleaseCachedInput`, typed bridge fast-path handlers.
- `pkg/plugin/sdk/sdk_engine.go` -- `Plugin.ForwardCached`, `Plugin.ReleaseCached`.
- `internal/component/plugin/types_bgp.go` -- `ReactorCacheCoordinator` extended with
  `ForwardUpdatesDirect` + `ReleaseUpdates`.
- `internal/component/plugin/coordinator.go` -- delegating wrappers.
- `internal/component/plugin/server/dispatch.go` -- engine-side RPC handlers + typed bridge
  wiring.
- `internal/component/bgp/reactor/reactor_api_forward.go` -- `ForwardUpdatesDirect`,
  `ReleaseUpdates`, `forwardUpdateCore`, `fwdBodyCacheEntry` extension.
- `internal/component/bgp/reactor/forward_update_test.go` -- 7 new tests covering refcount,
  copy-on-modify, ordering, backpressure, hoisting, missing-id, release batch.
- `internal/component/bgp/reactor/recent_cache_test.go` -- `TestPendingCacheNeverExpires`.
- `internal/component/bgp/reactor/forward_update_bench_test.go` -- `BenchmarkForwardDirect`.
- `internal/component/bgp/plugins/rs/server.go`, `server_forward.go` -- fast-path switch,
  plumbing delete, hooks.
- `internal/component/bgp/plugins/rs/server_test.go`, `propagation_test.go` -- updated
  hooks + new `TestRSFlushCallsForwardCached`, `TestRSForwardPlumbingDeleted`.
- `internal/component/config/environment.go`, `docs/architecture/config/environment.md` --
  removed `ze.rs.fwd.senders` registration + doc entry.
- `test/plugin/bgp-rs-fastpath-ibgp-identity.ci`, `bgp-rs-fastpath-ebgp-shared.ci`,
  `bgp-rs-mod-copy.ci`, `bgp-rs-replaying-gate.ci` -- Wiring Test functional tests.
- Mock reactors in 7 test files updated for the extended interface.
