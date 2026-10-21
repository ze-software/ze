# spec-bgp-rs-replacement-on-withdrawal: what remains after Phase 3 package d

Spec: `plan/immediate/spec-bgp-rs-replacement-on-withdrawal.md` (in-progress,
Handoff `verify`). Earlier state: `plan/handover/03-session-2026-10-18-linux-continue.md`,
sections "Item 8 continuation" and "RS replacement Phase 3 package (b)/(c)".

## Landed 2026-10-21 (package d, tests only)

`TestOwedKeyPurgeBetweenLookupAndAdmission` and `TestOwedKeyLookupChunked` in
`internal/component/bgp/reactor/rs_withdrawal_convergence_test.go`;
`TestRSOwnerWithdrawalSkipsDestinationOwnPath` and
`TestRSOwnerWithdrawalLookupFailureRetiresDestination` in
`rs_withdrawal_replacement_test.go`. Each has a recorded red (spec Design
Insights, 2026-10-21). The file holds the reusable seam `publishLookupTap`
(hook after bgp-rib's in-process lookup) and the RIB command helper
`dispatchRIBCommand`, which the remaining live tests can use.

## Landed 2026-10-21 (package e, production change)

`TestOwedKeyLivenessUnderChurn` (AC-15) was red: the per-source
`receiveCutAdvanced` fence retried a key whenever the survivor accepted any
UPDATE after the cut. The fence is deleted; every forwarded write of an owed
key (withdrawal or announcement) is now a mover, which round 1 of the
independent review showed is needed once the fence is gone. A read-buffer
shortage inside an admission no longer retires the destination: keys defer,
the worker backs off (`TestOwedKeyBufferShortageDefersKeys`). Also landed:
`TestOwedKeyNoRecoveryOwnerWritesWithdrawal`, `TestOwedKeyOwnerGoneAfterOweRetires`,
`TestRSDepartureWritesOnlyWhereSourceOwns`, `TestOwedKeyStormKeepsSourceAndForwardWorkerMoving`,
`TestOwedKeyBufferStallsCountOnlyConsecutive`, and AC-5's two tests, which
found that recovery wrote only the first survivor and withdrew a client whose
policy denied it although a permitted survivor existed (fixed: candidates are
stepped in election order). Spec Design Insights,
"Phase 3 package e", has the reds. D1 of
`spec-bgp-graceful-restart-rfc-defects.md` had NOT landed on 2026-10-21 (that
spec is still `design`, no `rpc.ReasonNotification` in the tree), so the Phase
2b second half still waits.

### Update 2026-10-21 afternoon (items 1 and 2 below are DONE)

- Item 1 closed in `6607a280a3` and `c8989803ee`. Causes: a product deadlock
  (`Reactor.cleanup` held `r.mu` while `fwdPool.Stop` waited for a worker
  firing `onResumed` -> `emitCongestionEvent` -> `r.mu.RLock`; fixed, pool
  stops first, `TestReactorStopsWithACongestedForwardWorker`), read-pool
  contamination (an RFC 8654 test's 64K block shrinks the next reactor's
  auto budget to 128 4K buffers and the weight tracker undid the lift;
  `rsLiveRouter` now sets `ze.fwd.pool.maxbytes` before `New`), and two test
  races (sources sent on three sessions with no wait; B's own recovery
  returning buffers into the dry window). Proof: `-race -count=3` wide run,
  363 passes. Details: spec Design Insights, 2026-10-21 entries.
- Item 2 closed in `6f18c55e24`: `TestOwedKeyPartialRoundSkipsBackoff`, red
  656 to 675 ms with the conversion broken, green 13 to 35 ms.
- Item 3: review round 4 ran (0 BLOCKER, 3 ISSUE, 2 NOTE), all fixed in
  `2affaf0809`. Round 5 ran over `2affaf0809` and `a740edfa20` (0 BLOCKER,
  1 ISSUE: an invented RFC 4271 5.1.3 quote in the AC-14 test), fixed in
  `bb50dfc36f` with the AC-12 property test. A round over `bb50dfc36f` is
  owed, and it is the SIXTH implementation round of this spec. Thomas
  authorised it ("round 6", relayed by the main session); it came back
  0 BLOCKER, 0 ISSUE. A seventh round needs Thomas again.
- NEXT (blocks AC-19): `TestOwedKeyGRPurgeRecoversSurvivor` (landed, about
  1 in 16 red in end-of-rib/survivor) hits the open row in
  `plan/journal/negotiated-family-lost-at-retention-boundary.md`: fix the
  producer (R-12's path-presence check extended to forwarded sent events,
  `plugins/rib/`), then `-count=20` green and a discrimination red. Spec
  Design Insights, last entry, has the detail.
- Phase 3 rows landed since: AC-14 `TestRSOwnerSynthesizedWithdrawalRecovers`
  (`a740edfa20`), AC-12 `TestOwedKeyRealRIBConvergence` (`2affaf0809`). Still
  owed in Phase 3 (the property test landed in `bb50dfc36f`): AC-19
  `TestOwedKeyGRPurgeRecoversSurvivor`, AC-9 (needs an ADD-PATH live fixture),
  AC-18 and AC-6 (their expectation depends on Phase 3a, Q-3), the AC-10
  re-record of `TestRFC8654FatalLengthReelectsAlternateBest`, then 3b, 3a, 4,
  5, 6 as below. Phase 2b second half still waits on D1.
- Test recipe that works with the broken working tree: `git archive HEAD`
  into a fresh dir under the session scratch, copy your edited files in, run
  `./le job run label unit-pkg quiet command go -C <dir> test -race -tags
  "<tags>" ...`. `rm -rf` of an export trips the test-deletion hook: make a
  new directory instead.

### OPEN at hand-off (do these first)

Review round 3 (fresh subagent) ran over the round-2 fixes; fixed since:
the stepped-past candidate fence (`recoveryAdmitKey.skipped`,
`TestRSOwnerWithdrawalDeniedCandidateReannounces`, red without it), the
RFC citations in `queueRecoveryOutcome`, the stale `recovery_owed_no_export_test.go`
header, the production journal row (`bound-too-small-for-its-own-burst.md`),
and its BLOCKER: the RFC-tagged `rfc7705_live_behavior_test.go` is back to HEAD
(the budget lift moved to `rsLiveRouter`; the churn test reads sent events,
`observeSentKey`, and the storm test reads Adj-RIB-Out tables, instead of the
64-frame wire capture). Still open:

1. FLAKY, not yet explained: in `-count=2` and in the wide run
   (`-run 'Recovery|Owed|RSOwner|...'`), `TestRSOwnerWithdrawalPolicyDeniedSkipsToNextSurvivor`
   (`next-permitted`: C never shows A's route; `all-denied`: one extra
   withdrawal of the prefix), `TestOwedKeyBufferShortageDefersKeys/stalled`
   and `TestOwedKeyBufferStallsCountOnlyConsecutive` (0 lookups while the pool
   was dry), and `TestOwedKeyLivenessUnderChurn/other-keys` (reactor cleanup
   timeout) fail; each is green alone at `-count=1`. Suspect process-global
   state the second run inherits (the read pools, a `ribevents` recovery
   owner, the policy seam). Find the cause before relying on these reds.
2. Round 3 ISSUE 4: the "later chunk stalls after an earlier chunk decided,
   so partial" conversion in `recoveryWorker.round` has no discriminating
   test (needs more than 500 owed keys with the pool going dry between chunks).
3. Round 4 of the review is owed over everything after round 3.

Gates owed by the main thread for this package: `./le verify current` (the
full verify) and the functional `.ci` suites that load bgp-rs or bgp-rr. The
`TestOwedKeyLookupChunked` red in a wide package run at 64f841ed12 is the
process-global read pool row in `plan/journal/registry-contamination.md`;
with this package it is green in that run.

## Phase 3 TDD rows with no test yet

Checked by `grep "func <name>("` over `internal/` on 2026-10-21. A row may be
covered under another name; read the row, then the named file, before writing.

| Test | Spec AC | Note |
|------|---------|------|
| `TestRSOwnerWithdrawalAddPathSourceKeepsOtherPath` | AC-9 | the live fixture negotiates no ADD-PATH yet |
| `TestRSDepartureAfterReconnectKeepsNewerRoute` | AC-18 | |
| `TestRSOwnerWithdrawalLaterAnnouncementElected` | AC-6 | expectation changes with Phase 3a (Q-3) |
| `TestRSOwnerSynthesizedWithdrawalRecovers` | AC-14 | `policyDenyASNs` (`rs_withdrawal_replacement_test.go`) gives the egress-denial shape; both rails, both synthesized shapes |
| `TestOwedKeyRealRIBConvergence` | AC-12 | real RIB, N=4, K=2 and K=N, every order |
| `TestOwedKeyRealRIBConvergenceProperty` | AC-12 | seeded random N in [3, 12] |
| `TestOwedKeyGRPurgeRecoversSurvivor` | AC-19 | real GR down with retention; `mark-stale`/`purge-stale` through `dispatchRIBCommand` is the lifecycle half |

Also owed by Phase 3: the AC-10 change of `TestRFC8654FatalLengthReelectsAlternateBest`
(owner-approved, re-record its discrimination), and the gates the spec names.

## Then, in the spec's order

Phase 3b (labelled withdrawal framing, its own commit), 3a (announcement
election, Q-3), 4 (bgp-rr), 5 (functional and RFC carriers), 6 (interop
`bgp-route-server-replacement-frr`), and the 2b second half once D1 of
`spec-bgp-graceful-restart-rfc-defects.md` lands. Each is about one agent's
budget.

## Facts found here

- bgp-rs sends a departure in batches of 500 (`withdrawalBatchSize`,
  `internal/component/bgp/plugins/rs/server_handlers.go`), and the reactor waits
  on each batch, so a departure never hands one recovery round more than 500
  keys. Only an owner-withdrawal burst exercises `recoveryLookupKeysMax`.
- With the chunk removed, a 1500-key admission fails `relay-stored-route: read
  buffer pool exhausted` and the worker resets sessions.
- Under `-race`, a departure of 2000 keys over the live fixture takes up to
  20 seconds.
