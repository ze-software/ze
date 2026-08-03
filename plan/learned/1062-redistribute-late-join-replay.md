# 1062 -- redistribute-late-join-replay

## Context

Redistribute injection is a point-in-time fan-out: `AnnounceNLRIBatch` only reaches
peers in the reactor's peer map at emit time (established peers sent immediately,
configured-but-unestablished peers get `QueueAnnounce`). A peer that FIRST enters the
map after injection (a dynamic/passive inbound peer, or a peer added by a later config
apply) never received the route -- unlike OSPF/IS-IS redistribute consumers whose
routes live in the flooded/synchronized link-state DB and reach a new adjacency via
database exchange. The goal: give the BGP redistribute path an equivalent late-join
delivery without regressing already-up-peer behavior or the value-typed/pool hot-path
contract. This gap already affected l2tp/connected and blocked `spec-as112-bgp-redistribute`.

## Decisions

- Chose **re-emit-on-request** (`redistevents.ReplayRequest` + echoed opaque `ReplayID`
  token) over (a) a redistribute-owned per-prefix store, (b) a generator-callback
  registry, and (c) routing redistribute through the best-path/Loc-RIB. (c) is
  INCOHERENT: Ze's Loc-RIB does not advertise to peers (change-detection + FIB mirror
  only) and redistribute routes are not in it. Re-emit reuses the existing
  `RouteChangeBatch` pipeline (snapshot and incremental are one code path) with no
  synced store to keep consistent. Mirrors `ribevents.ReplayRequest`.
- Chose a **payload-carrying** `events.Register[*ReplayRequest]` (carries `ReplayID`)
  over `ribevents`' payload-less `RegisterSignal`: `ribevents` broadcasts an untargeted
  full-table replay, whereas we must correlate a returning batch to the ONE new peer by
  token. The producer echoes the token blind, so the returning `RouteChangeBatch` stays
  peer-agnostic; the orchestrator alone holds `ReplayID -> peer`.
- Chose a SINGLE `ReplayID uint64` field on `RouteChangeBatch` (0 = incremental) + an
  `IsReplay()` helper over a separate `Replay bool` (derivable; a second field is a
  consistency footgun). Cleared in both `AcquireBatch` and `ReleaseBatch`.
- Orchestrator subscribes to peer `state` (watchdog precedent) and fires on the
  down->up edge; `ReplayID` per peer-up comes from a monotonic gen counter (rs
  `ReplayGen` precedent), so concurrent replays never cross-deliver.
- Held the `ReplayID -> peer` map for a TTL (bounded + hard-capped) rather than dropping
  it right after `Emit`: engine subscribers deliver synchronously but plugin-process
  producers deliver ASYNC, so a fixed drop would discard an out-of-process producer's
  late re-emit.
- Tightened the destination-agnostic evaluator: `ImportRule` records `Destination`;
  `Accept` requires the importing protocol to match (empty Destination stays agnostic
  for back-compat). Fixes the R-3 defect where an `import` under `destination bgp` also
  satisfied `Accept(_, "ospf")`.
- Targeting lives in the orchestrator (peer-aware); producers stay peer-agnostic. Peer
  selector threaded via `redistribute.RouteEntry.Peer` (empty = "*"), consumed only by
  the BGP consumer.

## Consequences

- The redistribute pipeline is now the one code path for snapshot + incremental +
  replay; a new producer that subscribes to `ReplayRequest` and re-emits its current
  set is discovered the same way (registry + `RegisterSource`). The as112 producer
  (spec-as112-bgp-redistribute) plugs in by adding the same subscription.
- `EventBus` request/re-emit correlation must NOT assume subscribers answered by the
  time `Emit` returns -- only true when every subscriber is in-process. Documented in
  `ai/rules/plugins.md` (EventBus Typed Payloads).
- Destination scoping is a behavior change: an import under one destination no longer
  leaks into another protocol. Existing configs that relied on the leak would change;
  none in the test suite did (18 redistribute `.ci` unchanged).
- `ze_bgp_redistribute_replay_total{source}` makes late-join delivery observable.

## Gotchas

- **The reactor SUPPRESSES a duplicate announce to a session that already holds the
  route.** So replay-to-`"*"` vs a targeted `UpdateRoute(<peer>)` is WIRE-INDISTINGUISHABLE
  for an already-up peer. Mutation test (force `""` in `handleReplayBatch`) did NOT fail
  the targeting `.ci`. Consequence: per-peer targeting (R-1) is NOT functionally
  observable and must be guarded by a UNIT test that inspects `RouteEntry.Peer`
  (`TestOrchestratorTargetsNewPeerOnly`), not a `.ci`. Reassuring side effect: even a
  `*` fan-out cannot spam an already-up peer with duplicates.
- **A RECONNECT `.ci` does NOT isolate the redistribute replay path -- it is a
  false-pass.** Mutation-verified: disabling `handleReplayBatch` (early `return`) still
  PASSES the reconnect, `-withdrawn`, and `-targeting` `.ci`, so the route reaches the
  peer by a path OTHER than the replay -- they never gated on the feature. The exact
  false-path was not traced, but the reactor does NOT itself persist routes across
  reconnects (`internal/component/bgp/reactor/peer.go` delegates that to external
  programs; `peer_initial_sync.go` clears the unsent `opQueue` on teardown), so a
  reconnecting session is a CONFIGURED peer, not a genuinely-new one. Isolate the replay
  with a CONFIG-ADD instead: start `bgp {}` with ZERO peers, inject the route, then add
  the peer via a config reload (write `config2.conf`, `kill -HUP`). The new peer is
  genuinely absent from the reactor map at inject AND has no `opQueue` (no configured
  peer slot to queue against), so it can receive the route ONLY via replay.
  Mutation-verified: `redistribute-late-join-configadd.ci` FAILS when `handleReplayBatch`
  is disabled.
- **A truly dynamic/inbound peer IS expressible -- via interop, not the check-mode
  harness.** `ze-peer` dials in only under `--mode inject` (no per-message expect), and
  a dynamic-only `group { ip dynamic }` on a custom test port opens a PER-peer listener
  (an anchor peer then claims the inbound). The real proof is the interop scenario
  `redist-late-join-dynamic-frr`: on port 179 (root in Docker) the passive listener is
  SHARED, so FRR dialing in from inside the group's range becomes a `dyn-<addr>` peer
  and receives a route redistributed at Ze startup, before FRR connected -- deliverable
  only by the peer-up replay.
- **A passing `.ci` is not evidence a feature works -- mutation-test it.** Disable the
  producing function and re-run: a test that still passes proves nothing (the observed
  effect came from another path -- here a reconnect delivered the route without the
  replay). This caught three false-pass late-join `.ci` that shipped in the first commit
  before the config-add and interop tests replaced them.
- A configured-but-unestablished peer gets `QueueAnnounce`, so injecting before it
  establishes does NOT isolate the replay path -- the queue delivers on establishment.
  The gap is ONLY for peers absent from the map at injection time.
- Emit the `ReplayRequest` OUTSIDE the coordinator lock: in-process producers re-emit
  synchronously, re-entering `handleReplayBatch -> lookupTarget` which locks the same
  mutex -- holding it across `Emit` would deadlock.
- `lookupTarget` must NOT delete on success: several producers each re-emit for the same
  `ReplayID`; the mapping must survive until the TTL, not until the first returning batch.
- `redistevents` importing `internal/core/events` is fine (no cycle) -- the "true leaf"
  constraint was about not importing `family`, not `events`.

## Files

- `internal/core/redistevents/events.go` -- `ReplayRequest` event + handle, `ReplayID` field, `IsReplay()`
- `internal/core/redistevents/pool.go` -- clear `ReplayID` on acquire/release
- `internal/component/config/redistribute/route.go` -- `ImportRule.Destination`, destination-scoped `Accept`
- `internal/component/config/redistribute/evaluator.go` -- `HasDestination`, `Rules()` deep-copy includes Destination
- `internal/component/config/redistribute/consumer.go` -- `RouteEntry.Peer`
- `internal/component/config/loader_redistribute.go` -- populate `ImportRule.Destination`
- `internal/component/bgp/redistribute/consumer.go` -- `BGPConsumer.InjectRoute` uses `entry.Peer`
- `internal/component/bgp/plugins/redistribute_egress/replay.go` (new) -- `replayCoordinator`, `handleReplayBatch`
- `internal/component/bgp/plugins/redistribute_egress/redistribute.go` -- `IsReplay` branch, peer param, replay metric
- `internal/component/bgp/plugins/redistribute_egress/register.go` -- `state` subscription, `parseStateEvent`
- `internal/plugins/static/inject.go`, `register.go` -- `reemitAll` + `ReplayRequest` subscription
- `internal/plugins/connected/connected.go` -- `reemitAll` + subscription
- `internal/component/l2tp/route_observer.go`, `subsystem.go` -- `emitEntries`/`reemitAll` + subscription + unsub
- `internal/test/plugins/fakeredist/{store.go(new),fakeredist.go,register.go}` -- current-set + replay for `.ci`
- `test/plugin/redistribute-late-join-configadd.ci` (new) -- inject with zero peers, add the
  peer via config reload; only the replay can deliver (mutation-verified). Replaced the three
  false-pass reconnect `.ci` (`redistribute-late-join{,-withdrawn,-targeting}.ci`) from the
  first commit.
- `test/interop/scenarios/redist-late-join-dynamic-frr/{ze.conf,frr.conf,check.py}` (new) --
  FRR as a genuinely-new dynamic/inbound peer over the wire receives a pre-connection route
- docs: `docs/features.md`, `docs/architecture/core-design.md`, `docs/guide/{configuration,plugins}.md`, `docs/functional-tests.md`, `ai/rules/plugins.md`
