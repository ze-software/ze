# 1271 -- fixit-bgp-egress-rail-divergence

## Context

Four multi-peer functional tests failed only under load, with WRONG WIRE BYTES to a
peer that established while an UPDATE was being relayed: 372 saw `AS_PATH
[65002 64496 65002 64497]` where ze's own local AS 65000 should have led, 378 saw a
duplicate announce, 394/395 saw an OTC route that should have been suppressed.

One relayed route could reach a peer by two different rails, and the rails disagreed.
The forward rail (`forwardUpdateCore`, `reactor_api_forward.go:249`) runs the ordered
egress steps -- export policy chain plus the in-process role/OTC/community filters --
on the RECEIVED wire and only THEN prepends the local AS, writing pre-filtered. The
replay rail, taken when Adj-RIB-In replayed stored routes on peer-up, emitted
`update hex ... add` announce commands: `AnnounceNLRIBatch` prepended the local AS at
build time (`reactor_api_batch.go:317-345`) and the session write gate then ran ONLY
`facts.exportFilters` on the already-prepended wire (`egress_inject_filter.go:43-91`).
Wrong order, and an incomplete filter set -- the role/OTC step registers through
`filterapi.Register` (`role/register.go:22-31`), which only the forward rail runs.

So under load a `remove-private-as REPLACE` policy rewrote ze's OWN just-prepended
local AS (65000 is inside RFC 6996 private space) to the peer AS, and OTC suppression
never ran at all.

## Decisions

- **Route the replay through the forward rail** rather than making the two rails
  identical (owner decision). A relayed route now has exactly ONE egress transform.
  `RelayStoredRoute` reconstructs the received-shape UPDATE and calls
  `forwardUpdateCore` with the single destination peer.
- **Own the reconstruction buffer through the existing recent-UPDATE cache**
  (`Add` -> `RetainN(id,1)` -> `Activate(id,0)` -> forward -> `Release`) rather than
  inventing a second refcount. `forwardUpdateCore` hands the wire to per-peer worker
  goroutines that write asynchronously and may adopt further pool handles onto the
  entry (`adoptFwdHandle`); `recent_cache.go evictLocked` is the single place that
  returns all of them exactly once. A private refcount would have duplicated that
  contract and risked returning a buffer an in-flight write still aliased. Chosen over
  parameterising `forwardUpdateCore` with a retain/release pair, which would have added
  a second closure allocation to the hot path for every ordinary forward.
- **Dedupe by having bgp-rs own replay, not by merging two replays.** 378's duplicate
  was never two replays racing: bgp-rs already marks a peer `Replaying` and withholds
  it from forward targets until its replay completes
  (`rs/server_handlers.go:157-162`), so its replay cannot race the live forward.
  Adj-RIB-In's own peer-up self-replay had no such gate. Ownership is claimed
  EXPLICITLY: bgp-rs dispatches a hidden `request bgp adj-rib-in claim-replay`
  (`adj_rib_in/rib.go:214`, `rib_commands.go:332`) from `OnAllPluginsReady`
  (`rs/server.go:105-108`, `rs/server_handlers.go:156-169`), and adj-rib-in then
  stands down (`rib.go:579`, `:744`). An earlier design inferred ownership from the
  first explicit replay instead; it was dropped because the latch only engaged
  AFTER a replay had already happened, so it missed every peer until one arrived.
  Claiming at startup is what covers peers that establish immediately.
- **Deferred the `OTCEgressFilter` `src-role` meta fallback** to
  `spec-fixit-otc-src-role-meta-fallback`. It is a real fail-closed gap but not needed
  by any AC here, and closing it requires editing an RFC-tagged test.

## Consequences

- `plugin.ReactorLifecycle` now composes `ReactorRelayCoordinator`, so a coordinator
  facade missing a delegation is a COMPILE error rather than a runtime one.
- Adj-RIB-In's stored-route replay is no longer expressible as a text command; the
  source peer travels with every replayed route, because the egress transform depends
  on it (AS_PATH prepend decision, RFC 4456 reflection, RFC 9234 role).
- A replay whose source peer is no longer established is now REFUSED rather than sent
  under a zero-valued source. The route is about to be withdrawn anyway, and sending
  it with the wrong transform is worse than not sending it.
- **An ADD-PATH source now has its replay refused outright** (`errRelayAddPath`,
  `reactor_api_relay.go:56-82`). This REMOVES behaviour that worked: the old announce
  rail emitted `nlri <fam> add <hex>` with no `addpath` keyword and
  `parseWireNLRISection` defaults `addPath=false` (`cmd/update/update_wire.go:272-278`),
  so add-path-sourced routes replayed correctly, collapsed to path-id 0, for
  single-path prefixes. The refusal keys on the SOURCE context, so one add-path peer
  stops replay of its routes to EVERY destination. It is an accepted interim, not a
  fix: the stored NLRI framing does not record whether it carries a path-id (the
  structured ingest strips it, the legacy ingest prepends it and only when non-zero),
  and emitting the wrong framing under an add-path context resets the destination
  session. Refusing loses a replay; guessing loses the peer. Homed in
  `spec-fixit-stored-route-relay-hardening`.
- The replay-ownership claim races session establishment. `sendPostStartupToAll`
  deliberately does NOT wait before `StartPeers` (`plugin/server/startup.go:220-258`;
  waiting deadlocks a handler that waits on peer activity), so a peer that establishes
  before the claim lands can still be replayed twice. Narrower than the design it
  replaced, but not deterministic.

## Gotchas

- **The stored attribute block is NOT what its doc comment claimed.** `RawRoute.AttrHex`
  came from `RawMessage.AttrsWire`, which `reactor_notify.go:345` sets to
  `wireUpdate.Attrs()` over the WHOLE path-attribute section
  (`wireu/wire_update.go:106`, returned verbatim by `attribute/wire.go:52`). For an MP
  family it therefore contains MP_REACH_NLRI carrying EVERY NLRI of the originating
  UPDATE, not just that route's. Rebuilding naively would emit two MP_REACH attributes
  and re-announce the whole original prefix set. The reconstruction strips types 14/15
  and synthesizes a single-NLRI MP_REACH. The old comment at `rib.go:67` asserted the
  opposite and was believed during design -- a code comment is its author's belief, not
  a producer fact.
- **Attribute ORDER must be preserved.** The functional tests assert exact hex and a
  real forward relays the source's byte order, so only the stripped MP attributes may
  move. Re-emitting a "clean" canonical attribute order breaks byte-for-byte
  expectations even though it is semantically valid BGP.
- **A facade that compiles is not a facade that works.** Phase 1 added
  `RelayStoredRoute` to the reactor adapter but not to `plugin.Coordinator`, which is
  what the plugin server actually holds (`server.go:65,184`). The runtime type
  assertion failed and every replay degraded to a `relay-stored-route: no reactor
  available` warning -- silently, because the caller only logged. It was caught by
  running the functional test, not by any unit test or by the build. Composing the
  interface into `ReactorLifecycle` immediately surfaced 8 test mocks that were
  silently missing the method.
- **"Nothing was dispatched" is not one outcome, and the code could not tell.**
  `forwardUpdateCore` returns a single error when no destination was dispatched to,
  but that state is reached both by a policy decision (RFC 7947 communities, RFC
  4456 reflection, an egress filter step) and by failures (read-buffer pool
  exhaustion, wire build, a stopped pool). A caller that tolerated it to avoid
  failing replays for legitimately-suppressed routes therefore also swallowed the
  drops. Worse, `accept == false` out of the egress pass is itself overloaded: a
  filter-plugin IPC error under the default fail-closed on-error policy, an
  unparseable response, a missing API server, and a recovered filter panic all
  produce it. The fix threads the REASON from each producer
  (`PolicyResponse.Failed` -> `PolicyChainResult.Failed` -> `egressStepResult.failed`)
  rather than guessing at the consumer. Lesson: when a consumer must distinguish
  two outcomes, check whether the value it reads can even carry the distinction --
  fail-closed and "decided to deny" look identical from the outside.
- **A return type that cannot express failure guarantees the failure will be
  misread.** Four review rounds chased "which rejects were not really decisions"
  and each found one the last had missed, because the answer depended on a fact
  the value did not carry. The policy-chain half was fixable: `PolicyResponse`
  grew a `Failed` field and the producers now set it. The in-process half is not,
  yet: `filterapi.EgressFilterFunc` returns a bare `bool`, so a filter that cannot
  evaluate a route looks exactly like a filter that decided to drop it, and
  `OTCEgressFilter` already reaches that state whenever a destination's role went
  unrecorded (a validate-open timeout, a plugin respawn nulling the map). Before
  adding a consumer that COUNTS outcomes, check whether the producer's type can
  even distinguish them; if it cannot, widen the type or do not make the claim.
- **A performance premise is a claim and needs a producer, like any other.**
  A hand-rolled hex decoder was added here to avoid a per-route allocation in
  `hex.Decode(dst, []byte(s))`. That allocation does not exist: `hex.Decode` never
  leaks `src`, so the compiler elides the conversion (`-gcflags=-m` says
  "zero-copy string->[]byte conversion"; `AllocsPerRun` says 0). It was reverted.
  The test written to pin it could not fail, because the form it guarded against
  allocates nothing either. Measure before optimising, and treat "this allocates"
  as a claim requiring evidence (`ai/rules/no-fabrication.md`) -- the session that
  introduced it had, in the same sitting, flagged exactly this class in another
  session's work.
- **Test 351 was mis-triaged into this cluster.** It loads neither `bgp-adj-rib-in` nor
  `bgp-rs` (`redistribute-l2tp-multi-peer-nexthop.ci:121-131,204`), so no peer-up
  replay exists in it and `RelayStoredRoute` is never reached. Its load-dependent
  failure was the RFC 6286 duplicate BGP Identifier race, fixed independently by
  `e4076920c`. "Same symptom class, same load sensitivity" is not attribution -- check
  whether the failing test even loads the subsystem before adding it to a spec's ACs.

## Files

- `internal/component/bgp/reactor/relay_payload.go` (new) -- received-shape UPDATE builders
- `internal/component/bgp/reactor/reactor_api_relay.go` -- `buildRelayUpdate`, `resolveRelaySource`
- `internal/component/bgp/plugins/adj_rib_in/rib.go`, `rib_commands.go` -- source-carrying replay, `replayOwned`
- `internal/component/plugin/coordinator.go`, `types_bgp.go` -- facade delegation + composed interface
