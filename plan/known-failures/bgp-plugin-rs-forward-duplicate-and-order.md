### `ze-test bgp plugin` route-server forwarding -- intermittent duplicate/ordering, NOT the module rename

Observed 2026-07-25 during the `codeberg.org/thomas-mangin/ze` ->
`github.com/ze-software/ze` module rename. Two DIFFERENT tests in the `plugin`
suite failed on two different full runs, both in the route-server forwarding
path, neither reproducibly:

| Run | Test | Symptom |
|-----|------|---------|
| loaded machine | 380 `rfc7606-relay-one-field` | receiver got the split ANNOUNCE and never the withdraw that must precede it |
| idle machine | 254 `llgr-readvertise-multipeer` | observer received the same UPDATE TWICE (identical bytes, back to back) |

NOT caused by the module rename. The rename rewrote import paths only; no `.ci`
file under `test/plugin/` changed (`git diff --stat -- test/plugin/` was empty at
the time of both failures), and no daemon logic changed. 380 passes 3/3 in
isolation and reproduces on invocation 23 of 60 under
`scripts/dev/stress-repro.py "bgp plugin" --test 380 --any-failure`; captured at
`tmp/stress-repro/bgp-plugin-380-20260725-150311.log` (scratch, not durable).

Pre-existing by the tests' own record: the header of
`test/plugin/rfc7606-relay-one-field.ci` documents the sibling
`bgp-rs-reactor-fastpath.ci` failing 1 in 6 **on an unmodified tree** with the
same EOR-versus-forward signature.

## Keeping these two together IS correct (2026-07-25 verification)

A triage pass was going to split this entry, on the grounds that 254 "loads only
`bgp-gr` + `bgp-rib`, so the RS/replay rails cannot apply to it". **That was
wrong, and it was wrong for an instructive reason:** it read the plugin set off
the `plugin { internal ... }` stanzas in the `.ci` config instead of off the
daemon. A `bgp { ... }` config path **auto-loads ~20 BGP plugins**, `bgp-rs` and
`bgp-adj-rib-in` among them, in a startup phase that runs before the
explicitly-configured ones. Verified from the daemon's own log:

- 254: `plugin startup tiers computed ... [[bgp bgp-adj-rib-in bgp-bmp]
  [bgp-filter-* bgp-hostname bgp-llnh bgp-role bgp-route-refresh bgp-rpki bgp-rs
  bgp-softver bgp-watchdog] [bgp-healthcheck]]`, then a second phase
  `[[bgp-rib gr observer]]`. Both RS rails are live, and `bgp.rs` is the subsystem
  that logs `sent EOR` in that run.
- 398 `role-otc-export-unknown`, whose command line names only
  `--plugin ze.bgp-role --plugin ze.bgp-adj-rib-in`, likewise auto-loads `bgp-rs`.

Reproduce the plugin set with
`ze_log_plugin_server=debug ZE_TEST_NO_BUILD=1 ZE_BIN=... ZE_TEST_BIN=... ze-test bgp plugin --save <dir> --pattern <name>`
then grep `<dir>/*/client-stderr.log` for `startup tiers computed`. **Never infer
the loaded plugin set from the `.ci` file.**

## Confirmed mechanism (producers read, margin measured)

Both symptoms fall out of ONE unordered boundary: `bgp-rs` claims peer-up replay
ownership from a callback that is not ordered against peer startup.

`internal/component/plugin/server/startup.go:236-258` `sendPostStartupToAll` fans
the post-startup callback out on **detached goroutines** and returns without
waiting (its own doc comment says so, and says that waiting deadlocks);
`startup.go:210` then calls `SignalPluginStartupComplete` -> StartPeers. So
`rs/server_handlers.go:169-182` `claimReplayOwnership` -> `request bgp adj-rib-in
claim-replay` -> `adj_rib_in/rib_commands.go:332-338` (`replayOwned.Swap(true)`)
races the first session establishment.

**Measured margin on an idle darwin host: 1-2 ms.**

| Test | `ownership claimed` | first `session established` |
|------|---------------------|-----------------------------|
| 380 (6/6 runs) | 18:39:00.411 | 18:39:00.412 |
| 254 | 19:42:30.888 | 19:42:30.889 |

Under 20-way suite load that ordering inverts. When it does:

- **254's duplicate.** `adj_rib_in/rib.go:579` (`if isUp && !r.replayOwned.Load()`)
  self-replays on peer-up *in addition to* `rs/server_handlers.go:214`
  `replayForPeer`. Both funnel into the same `RelayStoredRoute` ->
  `buildRelayUpdate` -> `forwardUpdateCore`, so the two copies are byte-identical
  and back to back -- exactly the recorded symptom.
- **380's missing withdraw.** `rs/server_forward.go:106-128`
  `selectForwardTargets` includes a peer iff `peer.Up`, and the only producer of
  `Up` is `rs/server_handlers.go:70`, which runs when the RS plugin's event loop
  processes the state event. That event is enqueued by the *receiver's* session
  read goroutine (`fsm/fsm.go:220` -> `reactor/peer_run.go:387` ->
  `reactor_api.go:59` -> `bgp/server/events.go:575`), while the source's UPDATE is
  enqueued by the *source's* read goroutine (`events.go:345`). Two producers, one
  queue: their order is scheduling-decided, not wire-decided, so the `.ci`
  handshaking both peers before the send does not close it. A receiver that loses
  that race is not a forward target at all, and `adj_rib_in/rib.go:767-795`
  `buildReplayRoutes` emits stored announcements only -- **the replay rail
  structurally cannot carry a withdraw** -- so the receiver gets the announce via
  replay and never the withdraw.

## Correction: `Replaying` does NOT gate the forward

`plan/learned/1271-fixit-bgp-egress-rail-divergence.md` (lines 40-41) states that
"bgp-rs already marks a peer `Replaying` and withholds it from forward targets",
and three in-tree comments said the same. **All four were wrong.**
`selectForwardTargets` (`rs/server_forward.go:106-128`) never reads `Replaying`.
Including a replaying peer is deliberate, recorded in
`plan/learned/630-rs-fastpath-3-passthrough.md:58-60` and pinned by
`TestReplayingPeerIncludedInForwardTargets`: BGP UPDATE duplicates are idempotent
at the receiver, and excluding replaying peers loses routes when peers connect
together. The comments were corrected 2026-07-25 (`rs/peer.go`,
`rs/server_handlers.go`, `adj_rib_in/rib_commands.go`); learned 1271 is history
and was left as written. **Do not re-derive the false claim from it.**

Consequence: the claim landing before the first establishment is the ONLY thing
preventing a doubled replay. There is no second line of defence.

## Status

Root cause identified, NOT fixed. The fix is a startup-ordering change -- make the
ownership decision reach adj-rib-in through the ordered startup stages instead of
a callback racing them -- in `internal/component/plugin/server/`, and it is
already owned by `plan/spec-fixit-stored-route-relay-hardening.md`, which names
exactly that declarative route. This entry exists so the next occurrence is
recognised rather than re-triaged from zero.

Capture on the next occurrence: the full `tmp/stress-repro/` log, plus the
daemon's `ownership claimed` and `session established` timestamps. If the claim is
the later of the two, this is the double replay and needs no further triage.
