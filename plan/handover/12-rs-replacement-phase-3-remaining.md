# spec-bgp-rs-replacement-on-withdrawal: what remains after Phase 3 package d

Spec: `plan/immediate/spec-bgp-rs-replacement-on-withdrawal.md` (in-progress,
Handoff `verify`). Earlier state: `plan/handover/03-session-2026-10-18-linux-continue.md`,
sections "Item 8 continuation" and "RS replacement Phase 3 package (b)/(c)".

## STATE 2026-10-21 late night: Phase 3b code landed, round 9 owed

- `c972e77416` (production): Phase 3b, the labelled withdrawal framing, AC-27.
  One step `nlrisplit.writeWithdrawalNLRI` behind `GetWithdrawalWriter` and
  `WithdrawalNLRIFromKey`; `buildWithdrawalPayload(payload, buf, source)`;
  `recoverySentNLRI`, `adjOutWithdrawalNLRI` and the RFC 7606
  `mpReachToUnreach` reach the same step; `nlrisplit.LabeledWithdrawal`
  deleted. Reds, benchmarks, `-gcflags=-m` and the rfc7311 audit re-judgement:
  spec Design Insights, "Phase 3b" entry.
- `a48a721a4c` (production, deletion): `RecoveryRequest.PrefixOnly` had no
  producer; field, IPC argument and bgp-rib branch deleted.
- `69a6d31a08`: interop `bgp-vpn-withheld-withdraw-frr` (checker
  `check_vpn_withheld_withdraw.go`). Green at `a48a721a4c`; red over a HEAD
  export with the writer bypassed (FRR NOTIFICATION 3/9, session reset).
  Phase 3b is complete.
- The held R-12 paragraph of `docs/architecture/plugin/rib-storage-design.md`
  is in HEAD: it rode along with `c379d157b1` (the RR session's commit), so
  nothing is held any more.
- REVIEW ROUND 9 IS OWED over `c972e77416`, `a48a721a4c` and `69a6d31a08`,
  and needs Thomas's authorisation. Phase 3a calls
  `WithdrawalNLRIFromKey` (Data Flow 3a), so 3a relies on this code.
- Verification debt for these commits: `plan/verification-debt/6d6a3fdc.md`
  (full verify not FRESH; owner approval owed for the nil-argument change to
  the two RFC 7311 tagged units, recorded with `rfc-change-ok`).
- NEXT after round 9: Phase 3a (AC-6, AC-18), then 4, 5, 6. Phase 2b second
  half still waits on D1 (no `rpc.ReasonNotification` in the tree on
  2026-10-21).

## STATE 2026-10-21 night, later: AC-9 landed, AC-10 found done, 3b next

- Coordinator ruling: round 8 (0 BLOCKER, 0 ISSUE) satisfies the gate; the
  note fixes in `d53835791a` owe no round 9. Round 9 is owed only for new
  production code.
- AC-9 landed in `4a8862c7c2` (test only):
  `TestRSOwnerWithdrawalAddPathSourceKeepsOtherPath`
  (`reactor/rs_withdrawal_addpath_test.go`, fixture `addPathLiveRouter`),
  both rails, reds recorded in the spec Design Insights.
- AC-10 is already DONE, not owed: `e30abc5e48` (2026-10-18) changed
  `TestRFC8654FatalLengthReelectsAlternateBest` to AC-10 under the owner's
  `RFC-approved:` trailer and re-took its RFC8654-5-4 record (unit-sha
  `3f035f768aec519e`); `./le rfc check` on 2026-10-21 does not list it stale.
  The rows below that still call it owed are stale.
- NEXT: Phase 3b (labelled withdrawal framing, `RecoveryRequest.PrefixOnly`
  has no producer, the `buildWithdrawalPayload` source-context change and the
  re-records of `TestAIGPForwardedMetricsReachFinalWire` and
  `TestRFC7311DirectLinkCostNeverWrapsOrDisappears`), then 3a (AC-6, AC-18),
  4, 5, 6. 3b is production code: commit it, then stop for round 9.
- `.git/index.lock` (empty, from 20:23) is still held; commit scripts land but
  cannot repoint the shared index. The coordinator is raising it with Thomas;
  leave it alone.

## STATE 2026-10-21 night: round 8 ran (4 NOTE, fixed), round 9 owed

- Round 8 (Thomas: "ok round 8") over `9e2b655e4c`: 0 BLOCKER, 0 ISSUE,
  4 NOTE, recorded (rounds 8); no product defect. Notes fixed in `d53835791a`
  (comments; FlowSpec extended-length and FlowSpec VPN key-shape cases; JSON
  labeled message-ID test), each new case red with its fix broken.
- STOPPED HERE: round 9 needs Thomas. AC-9, the AC-10 re-record, 3b, 3a, 4, 5,
  6 not started.
- Shared index: `d53835791a`'s script could not repoint the shared index
  (`.git/index.lock` held, empty, from 20:23). Until someone runs the repair
  the script printed (`git ls-tree HEAD -- <its five paths> | git
  update-index --index-info`), `git diff --cached` shows those paths as
  reverse-staged. Commits through `./le commit create` are unaffected.

## STATE 2026-10-21 late: round 7 ran and its findings are fixed, round 8 owed

- Round 7 (Thomas: "ok round 7") over 562a215cb8, 2624ccf8bf and the held page
  paragraph: 0 BLOCKER, 1 ISSUE, 4 NOTE, recorded (rounds 7). All fixed in
  `9e2b655e4c`, including a real FlowSpec key mismatch in `sentSourcePaths`
  (now `sentReceivedIdentity`). Spec Design Insights, last entry, has the reds.
- STOPPED HERE: `9e2b655e4c` is production code; round 8 needs Thomas. Nothing
  after it started (AC-9, AC-10 re-record, 3b, 3a, 4, 5, 6 as below).
- AC-9 groundwork read, not written: `lowLiveRouter`
  (`rfc7705_live_behavior_test.go`) hardcodes the remote OPEN's optional
  parameters (ASN4 + MP ipv4/unicast), so an ADD-PATH source needs a variant
  that also puts `capability.AddPath` (`AddPathSend` from A) in A's OPEN and in
  Ze's settings for A.
- The page paragraph is still held behind the RR hunk (below); it now also
  carries the round-7 wording fixes.

## STATE 2026-10-21 evening: AC-19 purge defect fixed, review round 7 owed

- `2624ccf8bf` fixes the bgp-rib defect behind the AC-19 flake (journal row
  in `negotiated-family-lost-at-retention-boundary.md`, now `fixed:`). Cause,
  read at the producer: bgp-rib runs commands and events on different
  goroutines, so A's forward to C can sit in the event channel while
  purge-stale or release-routes runs; the removal finds no A entry at C and the
  late projection keeps A there. Fix: `sourceApplied` fence per source, and
  `projectForwardedWriteLocked` for a forwarded write whose received message a
  lifecycle removal followed. Spec Design Insights, last entry, has the reds.
- STOPPED HERE: this is production code and needs independent review round 7,
  which is Thomas's decision (six rounds used). Nothing after it was started:
  AC-9 (ADD-PATH live fixture), the AC-10 change and discrimination re-record
  of `TestRFC8654FatalLengthReelectsAlternateBest`, then 3b, 3a (AC-6, AC-18),
  4, 5, 6 all remain as listed below.
- HELD, uncommitted in the working tree: the R-12 paragraph of
  `docs/architecture/plugin/rib-storage-design.md` that describes the
  forwarded-write check. The same file carries another session's RR hunk
  (spec-bgp-update-propagation-rfc-defects) whose `source:` names
  `rr.go` `sendPeerDown, recoverRouteSel`, absent from HEAD, so the file cannot
  be committed until that RR work lands. Commit the page right after it does.
- Live reproduction note: the flake showed 2 of 12 only while another lab job
  loaded the Mac, and 0 of 56 with stderr probes; the deterministic red is the
  unit test.

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
- DONE in `2624ccf8bf` (see the STATE section at the top): the AC-19 flake's
  producer defect. `TestOwedKeyGRPurgeRecoversSurvivor` is green
  `-race -count=20`; its review is round 7.
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
