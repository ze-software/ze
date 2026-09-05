# Spec: ci-peer-harness-contract-backlog

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Updated | 2026-08-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Hold the three deferral rows that named
`spec-fixit-redistribute-establishment-stall` as their Destination because it
owned the `.ci` peer-harness contract. That spec closed on 2026-08-23 having
shipped F1 to F6, and `ai/rules/planning.md` requires every row naming it to be
resolved inside the closing commit. This file is that destination.

**Nothing here is commissioned.** The rows keep Status `deferred` and name this
file so no record dangles. Thomas has not scheduled the work.

**The durable contract those rows point at is now
`docs/architecture/testing/ci-format.md`**, not a spec. Whoever picks a row up
starts there: it carries the peer-block table naming which parser reads which
line, and the Consumers table naming what each directive reaches.

| # | The row | Where it came from | Why it is not applied here |
|---|---------|--------------------|----------------------------|
| A | `test/plugin/eor-per-family.ci` has TWO producers of the same pair of End-of-RIB markers: its own peer-block `cmd=api` lines and its observer, which sends them again after a bare `ready()` with no establishment barrier. Check-mode ze-peer exits the moment its expectations are met, so the loser of the race dispatches into an empty peer set and the test fails with `no established peers to send to`. PASS in four suite runs, FAIL in the fifth | the retired deferral shard "ad-hoc-2026-07-27-dd843d81" | The fix needs a decision this backlog cannot make: which producer is authoritative. Deleting the observer's sends removes `.ci` assertions and the test-weakening hook refuses it, correctly, because it cannot tell a duplicate from a lost assertion. Note the row's premise about `cmd=api` is WRONG and the correction matters here: `cmd=api` executes nothing in any suite, so those peer-block lines drive nothing and the observer is the only producer. Re-measure before choosing |
| B | `internal/test/peer/checker.go` consumes ONE RULE PER MESSAGE, so no `.ci` can assert two facts about a single UPDATE. An honest AS4_PATH assertion needs both AS_TRANS in the 2-octet AS_PATH and the real ASN in AS4_PATH, and AS_TRANS alone is what the BUG produces. `test/plugin/bgp-rs-asn4-transcode.ci` cannot carry its three `contains` rules on one UPDATE for the same reason | the retired deferral shard "fixit-as4path-missing-on-rewrite" | Multi-rule-per-message matching is a matcher change in `internal/test/peer/`, with its own risk, and it unblocks two unrelated tests rather than one. The RFC 6793 behaviour is already pinned at the `wireu` seam with mutation-verified unit tests, so what is missing is the through-the-daemon proof, not the behaviour |
| C | An intermittent `test/plugin/bgp-rs-community-strip-multi-fastpath` <!-- doc-links: ignore (test this open spec plans and has not created yet) --> failure: `update-route failed ... no established peers to send to`, produced by `AnnounceEOR` (`internal/component/bgp/reactor/reactor_api_forward.go`) when `sentCount == 0` because no peer had established at EOR-announce time. Observed once in three full plugin-suite runs; passes in isolation | the retired deferral shard "mcp2026-1-stateless-core" | It was homed at the establishment investigation while that spec's premise was an engine stall. That premise is FALSE and the spec proved it, so the row needs re-reading against the surviving question, which is who is entitled to delay a session's EOR. `plan/immediate/spec-bgp-session-ready-contract.md` asks exactly that and is a skeleton Thomas has not commissioned, so routing the row into it would set that spec's scope for him |

## Provenance

Written 2026-08-23 when `spec-fixit-redistribute-establishment-stall` closed.
Three live rows in three shards named it as Destination and none was an
acceptance criterion of it. Each was parked there because that spec owned the
`.ci` peer-harness contract, and the contract outlived the six defects it fixed.

The pattern follows `plan/spec-harness-fail-open-guard-backlog.md`,
written on 2026-08-14 for the same reason: a closing spec must leave no row
without a home, and a closure must not set a live sibling spec's scope.

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. Each row is outstanding work this spec owns. -->

### From `ad-hoc-2026-07-27-dd843d81.md`, 2026-07-27

Deferred by ad-hoc (fixing `reactor-bus-subscribe.ci`).

`test/plugin/eor-per-family.ci` carries the same latent race that made `reactor-bus-subscribe.ci` red, and is FLAKY on it: PASS in four suite runs, FAIL in the fifth with `RPC error from ze-plugin-engine:update-route: no established peers to send to`. The peer harness's own `cmd=api` lines (`eor.ci`, `:8`) already drive both End-of-RIB markers that the `expect=bgp` lines assert; the observer then sends the SAME two markers itself after a bare `ready()` with no establishment barrier. ze-peer is check-mode: it exits the moment its expectations are met, so whichever of the two producers loses the race dispatches into an empty peer set.

### From `mcp2026-1-stateless-core.md`, 2026-07-30

Deferred by spec-mcp2026-1-stateless-core.

Fix the route-server establishment race behind an intermittent `test/plugin/bgp-rs-community-strip-multi-fastpath` failure. The failure is `update-route failed ... "no established peers to send to"`. `AnnounceEOR` (`internal/component/bgp/reactor/reactor_api_forward.go`) produces it when `sentCount == 0`, because no peer had established yet at EOR-announce time
