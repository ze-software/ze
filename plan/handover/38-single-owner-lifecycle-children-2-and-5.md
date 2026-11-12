# Handover: single-owner-lifecycle, children 2 and 5 (2026-11-12)

Written by the main session that ran `implement single-owner-lifecycle` on
2026-11-12. The umbrella is `plan/release/fixes/spec-single-owner-lifecycle.md`
(never claimed). Everything below was true when this file was committed; judge
the tree before acting on it.

## Where the set stands

| Child | State | What is left |
|-------|-------|--------------|
| 1 plugin process | `ready`, not started | Everything. Blocked: `internal/component/plugin/server` and `process` carried another session's uncommitted work all day (19 files at 03:30) |
| 2 BGP session | `in-progress` | Phases 3 and 4 are committed. Phases 1 and 2 (the initial-sync rewrite) are prepared and not started. One functional test is promoted and uncommitted |
| 3 BGP plugins | `ready`, not started | Everything. Same block as child 1 (it edits the plugin server's peer-state delivery) |
| 4 IKE | closed by its own session (`1d22056eb4`) | Nothing |
| 5 session-end locals | `in-progress` | All three code phases are committed. Five daemon-level proofs are written and not run. One acceptance criterion (AC-16) is not met and needs an owner decision |

The WIP cap: Thomas granted an exception for this set on 2026-11-12 ("cap
exception"). Children 2 and 5 were claimed at 15 and 16 in flight against a cap
of 12. The note is in the bodies of `170200cc60` and `4f1e24bf66`. Children 1
and 3 are covered by the same words but were not claimed.

## Commits of this run

| Commit | Content |
|--------|---------|
| `4f1e24bf66` | child 5 phase 1: LDP session end (code, unit tests, pages, journal row, 8 rfc5036 audit records) |
| `bae3903dbb` | child 5 phase 2: the L2TP order ticket across twelve paths, the contained-panic fix, reader tests, pages |
| `bec9096156` | child 5 phase 3: FlowSpec exit withdrawal, the recorded rule, two doctor checks, the guide |
| `170200cc60` | child 2 phase 3: the reconnect backoff forgets after a stable session |
| `0de757b33c` | child 2 phase 4: a listener for a passive peer added in the start window |
| `5d4d269afa` | not spec work: the LLGR reactor test's harness loads the `policy` plugin `bgp-gr` now needs |
| `30bcf7988d` | five journal rows and three skeleton specs for defects met on the way |

`./le repo compiles check` passed after the first four Go commits. It is owed
again for `bae3903dbb` and `bec9096156`.

## Decisions needed from Thomas

| # | Decision | Evidence | What each answer changes |
|---|----------|----------|--------------------------|
| 1 | Who clears an orphan FlowSpec record (child 5, AC-16) | The doctor report's fix command (`send bgp * update text mp ipv4/flow del ...`) withdraws the rule at the peers; nothing then removes the record `meta/ddos-flowspec/announcement`, and `ze data rm` is refused while a daemon owns the store (`storage/ownership.go`). So the warning stands after the operator did what it said (`doctor.go` `orphanDiagnostic`) | (a) The fix becomes `ze request plugin restart ddos-flowspec`: the start-time withdrawal reruns and removes the record; one command, no new surface, but it only works while the section is configured. (b) A plugin command that withdraws and forgets in one step: new CLI surface. (c) `ze data rm` through the daemon, which the rule "every operator command runs through the daemon" already asks for: then the fix is two commands. The main session's recommendation is (a) for the running plugin and (c) for the removed one |

Nothing else needs him. Three points were settled without asking, each on its
evidence, and are his to overturn:

- Child 5, AC-5: an adjacency expiry now really drops the LDP backoff (before,
  the buggy session end wrote it back). A neighbor that NAKs and is rediscovered
  inside 15 seconds is redialled at once. RFC 5036 Section 2.5.3 does not say
  what becomes of the backoff when the adjacency is lost; FRR 10.7.1 forgets it
  (`ldpd/adjacency.c::adj_del` to `nbr_del`, `nbr_new` is `calloc`).
- Child 5, L2TP: the route observer hears a withdrawal only for a removed
  session that had reached established, on every path (the reap and tie-breaker
  paths told it nothing before); and a SessionUp is no longer published for a
  session already gone from its tunnel.
- Child 5, L2TP: the accounting test in `authradius` is a reader-level proof.
  The spec assumed it could drive the reactor; it cannot (unexported, no seam).
  The reactor's order tests carry the red.

## Written and not run (uncommitted, in the working tree)

Every run below builds the daemon or a guest. On 2026-11-12 the shared tree was
broken three times in ten minutes by other sessions' in-flight edits
(`plugin/server/enricher.go`, `l2tp/plugins/authradius/coa_transaction.go`,
`plugins/cos/register.go`). A build failure there is theirs: find the file,
wait, run again. Red proofs go through a Go build overlay
(`GOFLAGS=-overlay=<json>`), which the functional, QEMU and interop builds
honour, so a mutation never touches the shared tree.

| Package | Files | Run, then |
|---------|-------|-----------|
| 2.3b backoff `.ci` | `test/plugin/reconnect-backoff-stable-reset.ci` (promoted) | Green observed twice (98 s). Red observed with `cycleStayedEstablished` forced false: step 6 fails, steps 3 to 5 pass. Owed: `./le test stress-repro run suite plugin test reconnect-backoff-stable-reset any-failure iterations 80 timeout 300 minutes 40` (about 15 minutes, 16 daemons), then commit. Also re-run `prefix-teardown-reconnect-backoff.ci` |
| 5.1b LDP `.ci` | `internal/test/fixture/ldp_neighbor_linux.go`, `ldp_session_end_linux.go`, `register_ldp_neighbor_linux.go`; `test/draft/ldp/ldp-session-end-clears-backoff.ci`, `ldp-setup-failure-publishes-no-session-down.ci`; one subsection in `docs/functional-tests.md` | `./le --name session test qemu ci test test/draft/ldp/<file>` for each. Red proofs in `register.go`'s session-end callback: `case operational:` to `case false:` (test 1), `if operational {` to `if true {` (test 2). Then stress, move to `test/ldp/`, re-run. Also re-run `test/ospf/ospf-ldp-sync-restore.ci` |
| 5.1c LDP interop | `test/interop/scenarios/ldp-backoff-clear-frr/`, `internal/le/interoplab/bgp/check_ldp_backoff_clear.go` and its test, one row in `check_special.go`, a row in `docs/architecture/testing/interop.md`, two sentences in `docs/architecture/ldp/mpls-ldp.md` | `INTEROP_SCENARIO=ldp-backoff-clear-frr ./le test integration interop`, then the red walk with the overlay the agent left under `tmp/session/2026-11-12-000937/scratch/ldp-backoff-red/` (regenerate it: `register.go` changed since), then green again, then write both image ids and the red text into the interop page's row |
| 5.2b L2TP `.ci` | `internal/test/fixture/tunnel_fixture_l2tp_session_clear.go`, `register_l2tp_session_clear.go`, a constant in `tunnel_fixture_l2tp_radius.go` and `tunnel_fixture_l2tp_ppp.go`; `test/draft/l2tp/session-clear-leaves-no-subscriber.ci` | `./le test qemu ci test test/draft/l2tp/session-clear-leaves-no-subscriber.ci`. Red proof: in `teardown.go` `notifySessionDowns`, `if r.eventBus == nil {` to `!= nil`. No QEMU form of the stress tool was found: decide what the test-writing rule then asks |
| 5.3b FlowSpec `.ci` | `internal/test/fixture/ddos_flowspec_exit_linux.go`, `register_ddos_flowspec_exit_linux.go`; `test/draft/reload/ddos-flowspec-parent-removed-withdraws.ci`, `test/draft/plugin/ddos-flowspec-orphan-doctor.ci` | `./le --name session test qemu ci test <file>` for each. The orphan test runs the fix command `ze doctor` prints: a refusal at its step 9 means the CLI does not take `*` for a flow `del`, which is a defect in the fix text. Red proofs are in the 5.3b handoff |

The fixture package and the interop checker package type-check with all of this
in (`./le test unit pkg path ./internal/test/fixture/ path ./internal/le/interoplab/bgp/ vet`,
exit 0). Other sessions hold uncommitted edits in both packages: commit by
hunks, new files whole.

## Child 2, phases 1 and 2: prepared, not started

Stage A is done (no production edit). Its findings, read at the producers:

- A-1 and A-2 hold: every wait of the sync goroutine after its session ended is
  bounded, and a queued teardown waits on nothing `runOnce` releases after the
  join. A-3 holds: `enterEstablished` is the only production publisher of
  Established. No stop condition.
- One behaviour the join would change, and the accepted answer: once the
  goroutine keeps its own session, a peer-level teardown dequeued after that
  session ended would be spent on the dead session and clear the queue, where
  today it stays for the next session. The drains treat "the session's latch
  reads ended" as "no session", which keeps the spec's behaviour to preserve.
- Phases 1 and 2 land as separate commits: phase 1 alone waits up to 2 seconds
  only when a session ends while its sync sits in an unacknowledged barrier.
- The census: the two flags are read or written at code sites in six files
  (`peer.go`, `peer_initial_sync.go`, `peer_run.go`, `peer_send.go`,
  `reactor_api_forward.go`, `session_ownership.go`) and named in comments in
  three more (`reactor_api.go`, `session.go`, `update_delay.go`). Six helpers of
  the sync that the spec's table does not list must take the session too
  (`drainAndCloseQueueGate`, `runQueuedTeardown`, `queuedTeardownSession`,
  `sendQueuedWithdrawal`, `sendDefaultOriginateRoutes`, `sendUpdateDirect`).
- Test migration: 117 store sites in 42 test files, two helpers proposed
  (`setTestInitialSync`, `runInitialSyncNow`), two file lists of 59 and 58
  references for two parallel agents. 17 files call
  `setState(PeerStateEstablished)`, which will stop closing the gate; two tests
  assert exactly that (`TestSetStateEstablished_ClosesGateBeforePublishing`,
  `TestPeersSynced_NotSyncedForJustEstablishedPeer`), so retargeting them goes
  through the independent test judge, and RFC-tagged units in the lists need
  their approvals before they are edited.
- Two stage A tests are written and have never compiled or run:
  `internal/component/bgp/reactor/peer_initial_sync_join_test.go.pending`
  (renamed so it stays out of every other session's reactor run; rename it back
  to `_test.go`). Both are expected to fail on today's code. Observe that red
  FIRST: `./le test unit pkg path ./internal/component/bgp/reactor/ run '^(TestRunOnceJoinsInitialSyncBeforeReturn|TestSuccessorPendingSyncUntilOwnMarker)$'`.

Start the production edit only when no daemon-level run is queued: deleting the
two Peer fields stops the reactor, and so the daemon, from compiling until
every reader is moved. For the interop scenario of phase 2
(`bgp-rapid-reconnect-initial-sync-frr`): FRR prints End-of-RIB received only
when Ze advertises the Graceful Restart capability
(`<peer>.gracefulRestartInfo.endOfRibRecv.<afiSafi>`,
`bgp_vty.c::bgp_show_peer_gr_extra_info`); the prefix count is
`<peer>.addressFamilyInfo.<afiSafi>.acceptedPrefixCounter`.

The full detail (the census table, both migration lists, the helper
signatures, every agent's four-part handoff) is in the per-spec state files
`tmp/session/2026-11-12-000937/state/session-state-single-owner-lifecycle-2-bgp-session-26c7a22c-d184-4e85-925e-5b5cd5ce75fa.md`
and `...-5-session-end-locals-...md`. Those are session files: read them while
they exist, and do not rely on them surviving.

## Owed bookkeeping, for the closure pass

No phase agent edited either spec beyond `Status` and `Phase`: the spec edit
gate asks for every door and every Current Behavior source. The closure agent,
which holds them, writes from the state files:

- Child 2: A-4, A-5 and A-9 confirmed; one TDD row for the added test
  `TestRunPrefixTeardownCountKeptAfterLongCycleThatNeverEstablished`.
- Child 5: A-1, A-2, A-3, A-5, A-11 confirmed; A-13 broken (the daemon asks only
  the plugins in its process table for their declared doctor checks, so a
  removed plugin's check vanishes) with its consequence built; the accounting
  test's wrong assumption; the L2TP behaviour changes above; AC-16 open.
- Both: the scoped lint of `./internal/component/bgp/reactor/...` never ran (it
  refuses to plan while a package it depends on is being edited). The LDP and
  FlowSpec lints report 0 issues. The L2TP lint reports four issues, all in
  files `857da9e798` added.

## Known reds this work did not cause

- `TestOrdinaryRefreshDoesNotDoublePrepend` (reactor): journal
  `non-idempotent-append-duplicates-the-record.md`, 2026-11-06.
- `./le arch doctor-coverage check`: two warnings with no code in
  `l2tp/reactor_kernel.go`, byte-identical before `bae3903dbb` and only moved.
- `./le arch refuse-by-default check`: `internal/plugins/ldp/register.go`
  `runLDPEngine`, from a commit of 2026-11-11.
- Four fixture tests that need `le` on PATH.

## Found and not yet recorded

Three findings have no journal row. Each needs a grep of `plan/journal/` first,
then a row listed by an owning spec:

- `l2tp/reactor.go` `handleTick` discards the kernel setup events
  `collectKernelEventsLocked` returns, after the scan already cleared
  `kernelSetupNeeded`: a session whose kernel setup waits for a retry and then
  meets a tick never gets it. In the code before this run. Read, not reproduced.
- `session_prefix_test.go` `TestPrefixBackoffReset` sets `prefixTeardownCount`
  to 5 then 0 and asserts 0 without running `run`. Reported as vacuous; not
  verified.
- The deletion guard refused a commit command because the words "ze data rm"
  stood in the commit message text beside test file names. Already journaled by
  another session today as the guard reading prose as delete targets; check
  before adding a row.
