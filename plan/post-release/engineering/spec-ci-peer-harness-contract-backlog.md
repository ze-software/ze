# Spec: ci-peer-harness-contract-backlog

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Uncommissioned harness investigations remain optional; inherited same-UPDATE product proof has a release-validation owner. |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Updated | 2026-08-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Ownership after release split

`plan/release/validation/spec-as4path-same-update-proof.md` solely owns row B's
through-daemon AS4_PATH evidence: AS_TRANS and the real ASN on the same UPDATE.
Its unit evidence does not replace socket proof. This file retains rows A/C
as unconfirmed historical observations and owns only any row B matcher
enhancement whose need is demonstrated against existing exact-frame/ordered
contracts. No matcher change is presumed or commissioned.

The complete rows and provenance below are retained as history; row B's
product-proof wording is a non-owning reference to that child. Both owners
retain the unscheduled/deferred disposition. Release classification is not
authorization to investigate A/C, declare a race, or begin implementation.

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
| A | Re-establish the historical `test/plugin/eor-per-family.ci` failure before scheduling a repair. The July record reported four passes and one `no established peers to send to` failure, but its two-producer explanation was wrong: peer-block `cmd=api` executes nothing (`docs/architecture/testing/ci-format.md`). The current `fixture06EORPerFamily` only waits for two sent EORs through `fixture06WaitEOR` (`internal/test/fixture/plugin_fixture_06.go`); it sends neither marker | the retired deferral shard "ad-hoc-2026-07-27-dd843d81" | There is no current producer-selection decision to make. Retain the wire assertions and capture a failure against the current fixture before attributing a race to the product |
| B | Ordinary `hex`, `prefix` and `contains` checks in `internal/test/peer/checker.go` consume at most one rule per message. `consumeOrdered` can consume several ordered needles, so the older claim that no `.ci` can assert two facts about one UPDATE is too broad. The inherited AS4_PATH proof needs AS_TRANS and the real ASN observed on the same UPDATE | the retired deferral shard "fixit-as4path-missing-on-rewrite" | Retain this through-daemon evidence item. First assess the existing exact-frame and ordered matching contracts; ordered needles can span messages, so their presence alone is not proof that both attributes were in one UPDATE. A matcher change requires a demonstrated gap. The inherited RFC 6793 unit evidence does not replace socket proof |
| C | Re-establish the historical `test/plugin/bgp-rs-community-strip-multi-fastpath.ci` failure against the current fixture. The July observation was one failure in three suite runs. `AnnounceEOR` still returns `no established peers to send to` when no peer was handled and no send error exists, but it now counts initial-sync suppression and an already-claimed family as handled (`internal/component/bgp/reactor/reactor_api_forward.go`) | the retired deferral shard "mcp2026-1-stateless-core" | The session-ready spec closed on 2026-09-05. Current ownership is documented in `docs/architecture/api/architecture.md`; its 2026-09-18 ruling also changed when initial sync emits EOR. `routeServerObserver03` waits for sent EORs and a forwarded update. Keep this row here as an unconfirmed harness observation; neither the old engine-stall diagnosis nor the historical failure establishes a current product defect |

## Provenance

Written 2026-08-23 when `spec-fixit-redistribute-establishment-stall` closed.
Three live rows in three shards named it as Destination and none was an
acceptance criterion of it. Each was parked there because that spec owned the
`.ci` peer-harness contract, and the contract outlived the six defects it fixed.

The pattern follows `plan/post-release/engineering/spec-harness-fail-open-guard-backlog.md`,
written on 2026-08-14 for the same reason: a closing spec must leave no row
without a home, and a closure must not set a live sibling spec's scope.

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. These are dated observations; the table above states
     their current disposition and supersedes their diagnoses. -->

### From `ad-hoc-2026-07-27-dd843d81.md`, 2026-07-27

Deferred by ad-hoc (fixing `reactor-bus-subscribe.ci`).

The two-producer diagnosis below is retained as the July account. It was
superseded by the `cmd=api` correction and the compiled observer described in A.

`test/plugin/eor-per-family.ci` carries the same latent race that made `reactor-bus-subscribe.ci` red, and is FLAKY on it: PASS in four suite runs, FAIL in the fifth with `RPC error from ze-plugin-engine:update-route: no established peers to send to`. The peer harness's own `cmd=api` lines (`eor.ci`, `:8`) already drive both End-of-RIB markers that the `expect=bgp` lines assert; the observer then sends the SAME two markers itself after a bare `ready()` with no establishment barrier. ze-peer is check-mode: it exits the moment its expectations are met, so whichever of the two producers loses the race dispatches into an empty peer set.

### From `mcp2026-1-stateless-core.md`, 2026-07-30

Deferred by spec-mcp2026-1-stateless-core.

This is the July observation, before the September session-ready changes.
Its current investigation boundary is row C.

Fix the route-server establishment race behind an intermittent `test/plugin/bgp-rs-community-strip-multi-fastpath` failure. The failure is `update-route failed ... "no established peers to send to"`. `AnnounceEOR` (`internal/component/bgp/reactor/reactor_api_forward.go`) produces it when `sentCount == 0`, because no peer had established yet at EOR-announce time
