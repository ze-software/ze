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
