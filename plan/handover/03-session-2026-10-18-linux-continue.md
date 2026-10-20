# Handover 03: continue on the Linux host (2026-10-18)

Written on the Mac at the end of session 4a43e416 for the next session on the
Linux host. The history behind each item is in
`plan/handover/02-session-2026-10-17-tests-green-and-rfc.md`. This file is the
list to work from.

## Owner goal (still active)

"ensure all test pass, once done close the number of RFC test failing or
missing". Standing owner decisions from this session:

- Explicit `all` UI rule: `ai/rules/points/cli/cli-grammar-keywords-before-values/acting-on-everything-needs-an-explicit-all.md`.
- ExaBGP 6 work, including 5.0 and 4.2 backward compatibility, and every known
  defect, current or found later (spec below). OD-1 (send BGP OPERATIONAL) is
  unanswered; the recommendation is no.
- WIP cap raised to 24 for the ExaBGP spec ("24 ok").
- Tagged-test corrections approved ("approve fix"); all five landed.

## State at hand-off

- `./le rfc check`: 2 violations, both items 2 below (RFC2545-3-1 and 3-2
  records, producer `buildRelayUpdate` changed in e62f9e8304).
- Full native verify: last run on the Mac failed early on the missing 127.0.0.6
  loopback (since added there). No full verify covers the commits of 2026-10-18.
  Every `plan/verification-debt/*.md` row written today names it.
- Mac-only leftovers that do not travel: the regenerated `../gh-pages` tree
  (uncommitted, built on darwin and wrong, see item 6), and the working-tree
  copies of the held ExaBGP files (item 5 carries them as a patch).

## Outstanding items

| # | Item | Where to start |
|---|------|----------------|
| 1 | Run the full native verify on Linux, build the known-reds list, fix reds with targeted reruns, rerun | `./le verify worktree` |
| 2 | Re-record RFC2545-3-1 and 3-2 positive on `test/plugin/adj-rib-in-replay-rfc2545-next-hop.ci` (needs-linux net-admin; the Mac VM run was killed in its fingerprint step) | `./le rfc discriminate-record id RFC2545-3-1 polarity positive unit test/plugin/adj-rib-in-replay-rfc2545-next-hop.ci route revert producer internal/component/bgp/reactor/reactor_api_relay.go::buildRelayUpdate citation "<the .ci's seq=2 expect line>"`, then the same for 3-2 |
| 3 | RFC9494-4.3-1 stays weak: nothing proves the trigger is the capability Ze advertised, not the one received (ze-peer mirrors Ze's OPEN). Add an asymmetric session (`option=open:value=drop-capability:code=71` on the llgr peer) or a tagged `negotiateWith` unit, then an independent re-judge | audit note in `rfc/audit/rfc9494.json` (6397a049c7) |
| 4 | RFC9494-4.3-3 stays weak: the live cache-forward rail (ReceivedUpdate cache entry `Meta: routeMeta` in `notifyMessageReceiver`, read by `forwardUpdateSection` and `forward_rs.go`) has no tagged unit. Also owed by spec-bgp-graceful-restart-rfc-defects: `test/plugin/llgr-stale-received-egress.ci` (live and relay steps), `llgr-reconnect-fbit-purge.ci` (needs a ze-peer per-family LLGR F-bit OPEN option), AC-15, AC-22..25 | `plan/immediate/spec-bgp-graceful-restart-rfc-defects.md` |
| 5 | ExaBGP 6 spec, claimed by session 4a43e416 (re-claim on Linux). Phase 1 landed (778d3912bf) except AC-4: old-5.0 51/72 and old-4.2 9/34 pass, remaining items (a)-(e) in the spec, the largest the Go mock refusing 4.2 `announce:`/`withdraw:` steps; the old populations are not in the stage yet. Phases 2-8 open. api population 33/43 committed cases: the 9 known bridge reds, api-on-exit (phase 4), api-disable (AC-41, phase 7: `ze exabgp migrate` refuses `shutdown true`). Held work: `git apply plan/handover/03-held-exabgp-bridge-and-sr-policy.patch` (bridge `clear adj-rib out` mapping for phase 4, which is semantically inverted versus ExaBGP's withdraw and must not land as is; api-sr-policy case for phase 5). The stage needs `PYTHONPATH=` and python >= 3.11 | `plan/immediate/spec-exabgp-6-api-compatibility.md` |
| 6 | Republish gh-pages from Linux: a darwin `./le site build` drops the linux-only iface-dhcp, iface-ra and memlock plugins (journal row in `plan/journal/gate-verdict-depends-on-the-machine.md`). The wiki catalog landed (61a5651) | `./le site build`, then `./le doc check verify` |
| 7 | Doc-review rows owed for the two `docs/architecture/testing/ci-format.md` claims about `runOneExaBGPTest`, still true after 778d3912bf | `./le doc wiring` |
| 8 | RFC4271-9.2-7 weak: the best-change publish does not drive peer UPDATEs, so a real proof needs a `.ci` on the reactor forward path | `rfc/audit/rfc4271.json` note |
| 9 | RFC7950-9.1-1 weak: binary, bits, empty, identityref, instance-identifier and leafref get no lexical check (`validateYangType`, validator.go) | `rfc/audit/rfc7950.json` note |
| 10 | Tag prose on `TestOnSessionReestablished_DuringLLGR` and `_NoCaps` (gr_state_test.go) cites line numbers and a `newForwarding` map that no longer exist; rewording needs `./le rfc approve` from the owner | `internal/component/bgp/plugins/gr/gr_state_test.go` |
| 11 | Cleanups the gate reports as information: 6 orphan discrimination records (RFC4271-5.1.2-3 on wireu aspath_slot, five RFC9012/RFC9830 on `rfc9012_tunnel_encap_carry_test.go`), 24 `RFC requirement:` comments in production Go | `./le rfc check` |
| 12 | 57 `unimplemented` RFC verdicts, all `{gap}`: implementation needs owner scope. Smallest: RFC2661-5.8-8 (L2TP max-retransmit: the YANG leaf and plumbing only) | `./le rfc check` audit-findings |
| 13 | Earlier items still open from handover 02: grammar follow-ups (hex/b64, config form, next-hop self refusal, event text), spec-docs-read-before-authoring (design, WIP cap), Linux runs (ospf-show, vrrp-show, ipsec, traffic-vpp-reject, test-show-system-sockets on the memory store, qemu-full/iso), owner OK to delete the old bins and the 10-14 scratch | `plan/handover/02-session-2026-10-17-tests-green-and-rfc.md` |

## Commits of 2026-10-18 (oldest first, for orientation)

Full-verify reds: 4a59a226da, e389e62f2e, 4fb7c7e4c3..f814d5a928. RFC fixes:
4ef90497c6 (close cause to GR and BMP), 99ed958ec5 (RFC 7606 5.2), fc20a13797
(RFC 9252 transposition), 7414ef24cc, 96158b01cc, 98975fb2d8, 9d4a5e79e2 (YANG
integer defaults), 0b323594c4 and 5252a8d5f7 (LLGR reconnect and received
stale), e62f9e8304 and d41fb04dfa (LLGR re-advertise rails), approved test
fixes d8e084c3eb, ecba2d9147, c85342c940, 2effa4167c, dc82b0dd89, 31c1a7eb57.
Re-judges: 097c5a6e0e, 7ca8d5215b, 4be0dfb736, 6397a049c7. Records:
1f2a3f82c5. ExaBGP phase 1: 778d3912bf.

## Item 8 continuation: spec-bgp-rs-replacement-on-withdrawal (2026-10-18)

The spec is claimed, `in-progress`, Phase 2/9. Phase 1 (wiring) is done:
`internal/component/bgp/reactor/rs_withdrawal_replacement_test.go` holds the
recreated Evidence probe and the cached-rail and fast-path carriers of AC-1.
All three are red on purpose: client C's history for 10.9.0.0/24 is B, A, then
a bare withdrawal. The commit that carries them names them. The order A, B,
A-withdraw stays green (`TestRSWithdrawalKeepsCurrentDestinationOwner`).
RFC4271-9.2-7 stays weak until Phases 3 and 5 land.

Still open: Phases 2, 2b, 3, 3b, 3a, 4, 5 and 6. Run one agent for each phase,
because each phase is about one agent's budget. Two facts constrain the order:
- Phase 2 cannot delete `RecoveryRequest.Cut` alone. The cut is the AC-18
  guard in `rib_recovery.go` (`SourceMessageID > request.Cut`), and the guard
  that replaces it is Phase 3's owner-session generation. Either keep the cut
  until Phase 3, or land Phases 2 and 3 in one commit.
- The second half of Phase 2b (the no-retention reason) waits on D1 of
  `spec-bgp-graceful-restart-rfc-defects.md`, which is still at `design`.

Phase 2 core landed (commit "rib: answer recovery per key", 2026-10-18): the
RIB answers one outcome per requested key (WITHDRAW, or SURVIVORS in repeated
`SelectBest` order), each key carries its own excluded-source set, the
destination's own candidates are skipped, and the election needs no sent entry.
The cut was KEPT, folded into the outcome's `Sent` list with HEAD's owner rule,
because the spec moves a guard to its carrier before deleting it; Phase 3
deletes it end to end. Wire behavior is unchanged, so the three Phase 1 probes
stay red. Still owed from Phase 2: R-12 (`TestRIBSentProjectionOfPurgedOwnerRemoves`,
the `recovery-write` check and the bgp-rib lifecycle dispatcher). Next, in
order: Phase 2b (its own commit, the spec puts it before Phase 3), then
Phase 3. `process.TestRecoveryQueuedDeliveryTransport` was red in setup
("invalid value for receive: update", a derived-binding refusal) with another
session's uncommitted `plugin/registry/registry.go` in the tree; not this change.

Update (2026-10-18, later): that process red is a missing build tag, not a
code defect. The test runs green with `ze_bgp` (journal row in
`plan/journal/feature-test-missing-build-tag.md`). R-12's RIB half landed
(commit "rib: remove a recovery write's purged survivor at the sent
projection"): `bgptypes.RecoveryWriteMeta`, the check at all three sent-store
sites, and the lifecycle dispatcher (stop cancels, then joins). No producer
sets the key yet. Phase 3 owes it, together with
`TestOwedKeyPurgeBetweenLookupAndAdmission` and a rewrite of the "Phase 3"
sentence in `rib-storage-design.md`. The owner approved changing
`TestRFC8654FatalLengthReelectsAlternateBest` (RFC8654-5-4) to AC-10: no
re-send to a destination that already holds the survivor. The approval is
recorded under session 3f5fcb0a. Make that change in Phase 3 and re-record
its discrimination. Phase 2b has NOT started, and it is the next package.
Its first half (teardown on publication failure, down event after buffered
deliveries, the FSM entered-Established bit, the `notifyPeerClosed` split,
the `fsm.md` edit) needs nothing from D1. The second half
(`ReasonAdjRIBInInvalid`, `ReasonForbidsRetention`) waits on D1. The
per-spec state file lists the code sites already read.

Update (2026-10-18, Phase 2b first half): the teardown on a failed receive
publication and the down moved after the buffered deliveries landed in one
commit ("bgp: announce a session's down after its buffered UPDATEs").
`failReceivePublication` (new `reactor/peer_session_end.go`) tears the source
down with Cease / Out of Resources on the first failure
(`receivePublication.fail` now reports the first). The FSM gained the sticky
`EnteredEstablished` bit. The leave arm is `leaveSession`: BFD
`releaseBFDClient` with no wait, the closing mark plus `forgetClosedPeer`,
Connecting, the history row recorded, the leave signal closed. `runOnce`'s
deferred `sessionDown` fires Event 18 when still Established, drains, waits on
the signal and the BFD subscriber, then raises session-dropped, announces the
down, logs and appends the history row. Test seams: `Peer.sessionEndHooks`
(atomic: leaveArm, beforeLeaveFire, drained, deliveryReceiver). Docs:
`behavior/fsm.md` "Reactor Notification", `behavior/peer-lifecycle.md`.
Green under -race: `TestSessionDownFollowsBufferedDeliveries` (remote-close
case) and `TestOwedKeyPublicationFailureTearsDownSource` (panicking and nil
receiver).

STILL OWED in the Phase 2b first half, before Phase 3: the
`TestSessionDownFollowsBufferedDeliveries` subcases (1) contended local
teardown with the arm held by `leaveArm` until `drained` fires (trap:
teardown's closeConn lets the read goroutine fire Event 18 first and drain the
arm itself, so hold only off the runOnce goroutine, for example by pausing the
read loop first), (2) hold-timer expiry, (3) Run returning Established
(body-read error, `waitForResume` on cancel), (4) the read-goroutine panic
seam after `reactorForwardRS`, (5) the `beforeLeaveFire` race to Idle, (6) BFD
Down drained on the subscriber, (7) reactor stop with a paused session
(`Reactor.Wait`, then `Peer.Wait`; record the red at HEAD), (8) GR congestion
teardown drained on the forward-pool worker. Also owed: an FSM unit test for
`EnteredEstablished`, and the red-at-HEAD record for the base case (not run
here: reverting needs a hand edit of the down defer, destructive git is
banned).

Update (2026-10-18, Phase 2b first half closed): the owed tests landed in
the commit "bgp: prove the down follows every way a session ends".
`TestSessionDownFollowsBufferedDeliveries` holds the remote-close case and
subcases (1) to (8). `fsm/entered_established_test.go` pins the
entered-Established bit. Case (4) uses a new seam,
`sessionEndHooks.beforeHandOff`, called in `notifyMessageReceiver`. The
subcase-1 trap is closed by `parkReadLoop`: it pauses, then sends one
KEEPALIVE so a loop blocked in its read reads once and stops at the gate.
The red at the parent was recorded with a `-overlay` of the parent's
ordering, and all ten subtests went red. The record is in the spec's Design
Insights. The subtests passed ten times under `-race`. The reactor, fsm, rib
and process packages pass under `-race` with the `ze_*` tags, except the
three Phase 1 probes, which stay red until Phase 3. Scoped lint is clean.
Next: Phase 3, split into three agent packages. (a) `adjOutWrite.allow`
records owed keys and the owner's session generation, and the
per-destination recovery worker starts before Established and is stopped
and joined in runOnce's down. (b) Admission checks the record, the
session, the epoch and the movers, and a departure owes a key only where a
no-longer-current session owns it. (c) The cut is deleted end to end
(`RecoveryRequest.Cut`, the `recovery-cut` meta, `types.RecoveryCut`,
`cmd/update/recovery.go`, the cut read in the rs `server.go`, the IPC
`<cut>` argument). Before that, each guard in the spec's "Guards of
deleted code" moves to its carrier, and the cut's AC-18 guard becomes the
session-generation check. Package (c) also adds the `recovery-write`
producer and rewrites the "Phase 3" sentence in `rib-storage-design.md`.
It changes `TestRFC8654FatalLengthReelectsAlternateBest` to AC-10 and
re-records its discrimination; the owner approved that change. The second
half of Phase 2b still waits on D1.

Update (2026-10-18, Phase 3 package (a)): commit "bgp: owe an owner
withdrawal to the RIB's election". The three Phase 1 probes are GREEN.
`adjOutWrite.owesWithdrawal`/`applyOwes` (`session_ownership.go`) owe a
forwarded owner withdrawal or a lifecycle removal of a source-owned key at a
non-ADD-PATH destination when the session has a worker and
`request bgp rib recovery` is registered; otherwise it is written as before
with one warning. `adjOutPath.generation` records the owner's session
generation. `adjRIBOut.owed` is cleared by `reset`, `retireWrite` and a
`beginAdjOut` session change. The worker is `reactor/recovery_owed.go`
(`Session.startRecovery` in the Established callback before
`setState(Established)`, stop in `Session.seal`, join in runOnce's first
defer); it reuses `lookupRecovery`/`admitRecovery`, split out of
`relayRecovery`, and sets each outcome's `Sent` to the record's NLRI. Tests:
`recovery_owed_worker_test.go` (`TestAdjOutOwnerWithdrawalOwedToRecovery`,
`TestAdjOutOwedAuthorities`, `TestOwedKeyHandOffNeverBlocksWriter`,
`TestOwedKeyWorkerLifecycle`); three live ownership tests now wait through
`ownershipLiveDecided`. Red recorded by overlay (spec Design Insights).
reactor, rib, rs, rr, process green under -race with the ze_* tags, except
one `TestMRTCollisionWinnerReservation/taken` red in the full reactor run
that passed 8 of 8 alone (not investigated further). Docs: update-building.md,
buffer-architecture.md (copy trigger), guide/route-reflection.md.
NOT done here, owed by (b): epoch, movers, excluded sets and merges in the
record (a second owe now replaces it), the per-key admission check, departure
owing only where a no-longer-current session owns, `errRelayNoSource` re-owe,
stopped-source exclusion, owed-refusal counter. Owed by (c): the cut deletion
end to end, the `recovery-write` producer, the rib-storage-design sentence,
and the AC-10 change of `TestRFC8654FatalLengthReelectsAlternateBest`, which
still passes unchanged after (a). Gates owed: `./le verify worktree`,
`./le test unit all`.

## spec-config-yang-reference-types (2026-10-18)

Landed in commit bccd1e7cf5 ("config/yang: check leafref, identityref and
instance-identifier"). It implements the owner decision of 2026-10-18: the
loader refusal (`ErrUncheckedType`, `loader_unchecked.go`) is deleted, and
the loader and validator now check the three reference types.
`ValidateCustomSections` runs the whole-tree require-instance walk
(`Validator.ValidateReferences`). traffic-control `default-class` is a
leafref, proved by `test/parse/yang-leafref-default-class.ci`. The commit
holds 17 tagged RFC 7950 tests, each with a discrimination record.
`test/weakened/3f5fcb0a.md` records the deleted refusal test. Scoped lint is
clean. The yang, config and traffic package tests pass untagged, and the
.ci passes.

Still owed:
- Independent `/ze-review` of the commit. The spec stays open.
- Independent re-judge (`./le rfc audit-stamp ... mode rejudge`) of the STALE
  verdicts RFC7950-9.1-1, 9.1-2, 7.6.5-1, 8.3.3-2 and 7.2.2-1. RFC7950-9.1-2's
  audit record still says "unimplemented", which `./le rfc check` reports
  until the re-judge is done. Then run `./le rfc index-update`.
- Retiring `plan/spec-config-yang-type-{leafref,identityref,instance-identifier}.md`.
  `./le commit create` counts each removal as a spec closure, allows one per
  commit, and wants an independent-review artifact at
  `tmp/review/config-yang-type-<name>-<sid>.md`.
- The tagged unit run of `internal/component/config/yang/...` (`yang/cli`
  needs the `ze_*` feature tags). It did not build while other sessions had
  rib and vrrp edits in progress. Untagged, `yang` passes.

## Known reds of the full verify over fd9d7ed55c (2026-10-18, Linux)

Run `20261018T135940...-fd9d7ed55c16` (logs under `tmp/verify-worktree-logs/`,
machine-local) exited 1. Each red was rerun targeted on HEAD (4d1410b2ea and
later) where the tree built; the ones not rerun are marked.

| Red (stage) | Failure | Cause | State |
|-------------|---------|-------|-------|
| `docs/functional-tests.md`, old-4.2/old-5.0 paragraph (doc check/links) | dead path `etc/exabgp/` | real: the line names a path of the ExaBGP repository | fixed in 3db1ae39a7 (ignore marker with the reason); `./le doc check links` exit 0 |
| `website/data/repo-facts.json` (site facts/check) | detail comments 6422 vs 6544, interop scenarios 188 vs 190 on HEAD | real: counts moved | fixed in e16f17f96a |
| `TestCIAcceptOnlyLint` (unit-cached, race part 3) | 106 old-4.2/old-5.0 `.ci` unparseable | real: 778d3912bf ported a dialect the generic parser cannot read and did not list it | fixed in 8f49d5df57 (excluded prefixes; every case pins its messages) |
| `TestCIAcceptOnlyLint` (new on HEAD) | `test/parse/bfd-auth-secret-hex.ci` unannotated accept-only | real: 97686ea267 | fixed in 370e4e638c (accept-only annotation naming the unit tests that assert the bytes); runner package green |
| `TestExaBGPCaseReadsEveryFixture`, interoplab/bgp (unit-cached, race part 3) | Go mock refuses old-4.2 `N:announce:`/`N:withdraw:` steps, `option=notification:send-notification`, `option=open:send-unknown-message` | real, open AC-4 items (a) and (b) of `spec-exabgp-6-api-compatibility.md` (item 5) | still red by reasoning (fixtures unchanged); not rerun on HEAD: the package does not build with another session's uncommitted `config/yang/validator_instance.go` (`undefined: absentContainerInUse`). The test needs `-tags ze_bgp` |
| `TestSIGHUPQueuedBehindTransactionRunsWhenItEnds`, cmd/ze/hub (unit-cached) | "the queued SIGHUP did not reload when the holder's transaction ended", subtest ran 1107s | likely load or hang under the full run; unconfirmed | not rerun: cmd/ze/hub does not build with the uncommitted config/yang and ike edits |
| `TestCLIRunInProcessExitsOnSIGTERM`, chaos/orchestrator (race part 2) | "no route announced within 30s: the in-process run never reached Established" | likely load-timeout flake (30s deadline under -race in the full run); unconfirmed | not rerun (budget) |
| test functional/exabgp-test | the 9 known api bridge cases (api-eor, api-flow-merge, api-health, api-peer-lifecycle, api-rib, api-rr, api-rr-rib, api-sr-policy, api-vpls) | spec-exabgp-6 open phases; api-rib/api-rr-rib possibly fixed by 0fabd740a7 | not rerun (budget) |
| test functional/gating | not read | - | not read or rerun (budget) |
| `test/decode/bgp-srpolicy-bsid-srv6-decode.ci` (8d18d4c4c3) | unparseable in the decode suite (`expect=exit:`, `expect=stdout:contains=`) | real | rewritten as two decode-suite files (Length 18, and `-long-` for Length 20) with full `expect=json:`; both RED on the NLRI only, the journaled SR-Policy Length-octet defect (`plan/journal/declared-format-contradicts-payload.md`, producer now named). The BSID sub-TLV output matches |
| `test/parse/srpolicy-binding-sid-srv6.ci`, `test/encode/bgp-srpolicy-1-encode.ci` | earlier build failure | another session's uncommitted ike/config edits | not run (budget) |

Still owed: fix `srpolicy.DecodeNLRIHex` (read and check the Length octet,
walk a packed region, RFC 9830 Section 2.1 quote, unit test first; move the
`--nlri` cases `bgp-srpolicy-1-decode.ci` and `-2-` to the wire form) so the
two decode cases go green; then every rerun marked "not rerun" above.

## Review fixes for bccd1e7cf5 landed as 2e4be94ae3 (2026-10-18)

All findings of the review are fixed in 2e4be94ae3 (yang, config, cli, traffic
tests green with the feature tags, scoped lint 0 issues,
`./le test bgp parse yang-leafref-default-class` passes). Owed:
- Discrimination records. A background run of 29 `./le rfc discriminate-record`
  calls was started on Linux: the 21 rfc7950 records `./le rfc check` reported
  "no longer verifies (producer-changed)", each re-taken with its stored route
  and producer, and 8 for new or changed tags, all route revert:
  rfc7950_reference_walk_test.go TestRFC7950LeafrefDefaultInUse (9.9-1 +,
  validator_default.go::canonicalDefaults), TestRFC7950LeafrefDefaultNotInUse
  (9.9-1 -, validator_default.go::caseInEffect),
  TestRFC7950InstanceIdentifierDefaultInUse (9.13-1 +,
  validator_instance.go::absentLeafInUse), TestRFC7950InstanceIdentifierKeyPrefix
  (9.1-2 -, validator_reference.go::checkInstanceKeyPrefix),
  TestRFC7950UnionDefaultLeafrefMember (9.1-1 +,
  module_default.go::canonicalUnionDefault), TestRFC7950LeafrefUnionLoop (9.9-1 -,
  loader_reference.go::checkLeafrefLoop); rfc7950_reference_test.go
  TestRFC7950InstanceIdentifierLexical (9.1-2 +,
  validator_reference.go::validateInstanceIdentifier) and
  TestRFC7950IdentityrefFormat (9.1-2 -, validator_reference.go::validateIdentityref).
  Check `git diff rfc/discrimination/rfc7950.json`, rerun any record missing,
  then commit the json alone.
- Independent re-review of 2e4be94ae3, and the audit re-judge of the rows
  `./le rfc check` marks STALE (8.3.3-2, 9.1-1, 9.1-2, 9.3.1-1, 9.5.1-1).

## RS replacement Phase 3 package (b), part 1 (2026-10-18, Linux): production UNCOMMITTED

The production half of package (b) is in the Linux working tree and is NOT
committed: deleting the destination-wide sequence fence leaves 16 test
references that the spec's "Guards of deleted code and their new carriers"
table migrates, so the reactor package's tests do not compile until they are
migrated. This work exists only on the Linux checkout (patch:
`backups/rs-repl-phase3b-20261018-192817.patch` plus the new test file copy
beside it). Do not move machines until it lands.

What it does (all `internal/component/bgp/reactor/`): one owed record per key
(`adjOutOwed` gains an excluded-source set, an epoch, WITHDRAW movers and a
`done` waiter channel; later owes merge); the writer stages movers and refuses
a bgp-rib replay of an owed key, counted once per write on the new
`Peer.owedRefused`; `sentUpdateSequence` and `noteFilteredOperation` are
deleted; the forward worker admits each key against its record, session,
epoch and movers, and re-owes a key whose survivor's session ended
(`errRelayNoSource`, `errForwardNoSource`, generation fence) with that source
excluded; the departure (`recoverNLRIBatch`) owes only keys its ended session
still owns (ADD-PATH: per prefix, the writer refuses identifiers a current
session owns) and waits on the records; `relayRecovery` and
`recoverySnapshot` are deleted. New tests, written first and not yet run:
`recovery_owed_fence_test.go`.

Owed by the continuation, in order: migrate relay_recovery_test.go (13 units),
`TestRecoveryRefusesFilteredCandidateWithdrawalAfterLookup`,
`TestRecoveryWaitsForFilteredReceivedPublication` and
`TestFinalWriterMixedWithdrawalIntent` per that table (fixtures must start the
destination's recovery worker, or the departure retires it as ownerless); run
the new tests; assign or delete `recoveryKeyNothing`; the ADD-PATH departure
test, stopped-source exclusion, `TestOwedKeyPurgeBetweenLookupAndAdmission`
(needs package (c)'s recovery-write producer); update-building.md and
rib-storage-design.md in the same commit. The MRT collision flake was not
investigated. Full file list: the per-spec state file of session d75c882e.

## RS replacement Phase 3 package (c), first half (2026-10-18, Linux)

Landed in the commit "bgp: delete the departure cut end to end" (this file
rides in it): `RecoveryRequest.Cut`, the IPC `<cut>` argument,
`NLRIBatch.RecoveryCut`, the `recovery-cut` meta and its parser, the rs `cut`
parameter chain, and non-ADD-PATH `Sent` (`recoverySentLocked`). AC-18 is
carried by the writer's generation check alone, proven red by `-overlay`
(spec Design Insights). RIB, rs, update and process guard rows rewritten;
`TestRecoveryAddPathDepartureWithdrawsEverySentIdentifier` exists (renamed).
reactor, rib, rs, rr, update, process, ribevents green under `-race` with the
ze_* tags; the three Phase 1 probes green.

Owed next, in order (same package):
1. R-12 producer. Code is in the spec's Design Insights (package c row). With
   it, `process.TestRecoveryQueuedDeliveryTransport/direct/unrelated-prefix/held-sent`
   fails 3 of 25; without it 0 of 25. Find why the RIB's R-12 check
   (`projectRecoveryWriteLocked`, `recoveryCandidateGone` in
   `plugins/rib/sent_owner.go`) withdraws a live survivor in that schedule,
   fix it, then land the producer, its mark assertion on the three probes,
   and the rib-storage-design sentence (it now says no event carries the key).
2. `TestOwedKeyPurgeBetweenLookupAndAdmission` (spec TDD row, real RIB).
3. Then Phase 3's remaining items, then 3b, 3a, 4, 5, 6 as the spec orders.
Gates owed: scoped lint (`./le go lint run scope ./internal/component/bgp/...`
refused twice: "inputs changed during planning" while another session's
evpn interop ran), `./le test unit all`, `./le verify worktree`.

## VRRP checksum follow-up to a78870fd39 (agent, 2026-10-18): owed

Landed: d2abad70ad (transmit test asserts `params.Checksum == spec.Checksum`
in both modes, new RFC9568-7.2-5 positive tag and record; receive tag moved to
RFC9568-5.2.8-2; 89 records re-taken; reseal), c8648e5a33 (rfc5798.md false
claims), 3cbe8df66f (3 guest records).

Owed: 39 QEMU-guest discrimination records in rfc/discrimination/rfc5798.json,
rfc9568.json and rfc3768.json still fail `./le rfc check`. The guest build
broke on other sessions' in-flight bgp/reactor, then iface work
(`internal/plugins/iface/netlink/backend_linux.go` no longer implements
`iface.Backend`; `internal/component/iface/address_owner.go` lacks
`ownerConflictLocked`). Once main builds, re-take every row `./le rfc check`
lists as "no longer verifies" for these units, with the route and producer
stored in the record:
`./le rfc discriminate-record id <ID> polarity <p> unit <unit> route revert producer <producer> kernel tmp/kernel/build/vmlinuz`.
Units: TestVRRPOwnerAnswersWithVirtualMACOnly, TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly,
TestVRRPRedirectSourceFollowsVirtualMAC, TestVRRPIPv6RedirectFollowsVirtualMAC,
TestTxAdvertV4OnWire, TestRxAdvertScopedToReceivingInterface (RFC9568-7.1-4).
Then `./le rfc reseal` and commit only rfc3768/rfc5798/rfc9568 files.

Verdicts to re-judge (stale, reseal refuses): RFC5798-7.2-1, RFC9568-5.2.8-2,
RFC9568-7.2-1, RFC9568-7.2-5.

## Gap #15 RFC5216-5.3-5 (EAP-TLS configured intermediates): committed f6c2374f02

Code, unit tests, reload .ci (`tx-ipsec-intermediate-certificate-requires-eap-tls`,
green) and scoped lint (0 issues) landed in f6c2374f02. Still owed:
- Run interop scenario `responder-eap-tls13-leaf-only-client`
  (`test/interop-ipsec/scenarios/`, needs Docker; never run yet), and its
  revert-red walk per `ai/rules/interop-and-goal-validation.md`.
- Independent re-audit of RFC5216-5.3-5: `./le rfc check` reports the audit
  verdict "unimplemented" against a row that no longer carries `{gap}`, so the
  verdict is stale and must be re-judged by a reader who did not author it.

## RS replacement Phase 3 package (c), R-12 producer (2026-10-18, Linux)

Landed in the commit "bgp: a live recovery source that advanced is retried,
never excluded" (this section rides in it). The held-sent flake was a real
defect, not the producer: `receivePublicationCut.current` was one boolean, so
a live survivor that accepted any UPDATE after the round's cut was re-owed with
itself excluded and C got a bare withdrawal. Now `receivePublicationCut.state`
splits Advanced (key left owed, no exclusion) from Ended (re-owe, excluded).
Root cause, interleaving and red proofs: spec Design Insights, "R-12 producer"
row. Producer (`recovery-write` in a fresh meta copy, `forwardUpdateSection`),
probe mark assertion (red without it), rib-storage-design and update-building
updated. Evidence: process held-sent subtest 100/100 `-race` (was 3/60 without
the producer, 7/60 with it); three probes 10/10; new fence subtests (k), (l).
Builds and tests ran with a Go overlay mapping every foreign dirty file to HEAD
(other sessions' rfc9774, link-local, MUP, dhcp edits did not compile).

Owed next, in order:
1. `TestOwedKeyPurgeBetweenLookupAndAdmission` (spec TDD row; reactor, real
   RIB, file `rs_withdrawal_convergence_test.go`, does not exist yet).
2. Coordinator's report: `test/plugin/mup-t1st-malformed-treat-as-withdraw.ci`
   fails (`./le test bgp plugin mup-t1st-malformed-treat-as-withdraw`,
   reproduced on the working tree): the receiver gets K and the fence, never
   the MP_UNREACH of K. In that config bgp-rib is loaded but no peer attaches
   it, so a recovery owner IS registered and the treat-as-withdraw is owed
   (`owesWithdrawal` -> `recoveryOwed`). Ruled out: the RIB can key both K and
   the malformed K' (`recoveryIdentity` returns no error for either, probe
   run). Not yet found: whether the worker's round reaches the lookup, what it
   answers, and why no withdrawal is written. Next step: daemon debug logs of
   the recovery worker in that .ci, or a reactor test with the RIB loaded but
   unattached. Also question for the spec: is "owner registered" the right
   test when the RIB does not receive the destination's sources?
3. Phase 3's remaining items, then 3b (incl. `RecoveryRequest.PrefixOnly`,
   no producer since package a), 3a, 4, 5, 6.
Gates owed: scoped lint `./le go lint run scope ./internal/component/bgp/...`,
`./le test unit all`, `./le verify worktree`.

### Update: MUP regression fixed (item 2)

Root cause: no stall. The owed withdrawal was decided WITHDRAW, then the
daemon's shutdown (started when the client saw the source's later fence route)
canceled the admission. bgp-rib was auto-loaded and attached to no peer, so it
was not a recovery owner for that source. Fix: the writer owes only when the
owner process receives the owning source's received UPDATEs
(`Session.recoveryOwedFor` in `reactor/recovery_owed.go`); otherwise the
withdrawal is written in order with one warning. Test
`TestOwnerWithdrawalWrittenWhenOwnerDoesNotReceiveSource` (red at parent,
green after). Both MUP `.ci` and `linklocal-capability-malformed-nexthop-withdraw.ci`
green. reactor (three Phase 1 probes included), rib, rs, rr, cmd/update,
plugin/process and ribevents all green under -race with the ze_* tags
(overlay mapping foreign dirty files to HEAD). Spec Design Insights carry
the answer and the open ordering note.
Remaining owed: item 1, item 3, and the gates above.

### Update: RFC 2545 tests re-scoped for capability 77 on receive (owner-approved)

Done (working tree, NOT committed), all in
`internal/component/bgp/reactor/rfc2545_forward_subnet_test.go`:

| Item | Result |
|------|--------|
| Fixture | `rfc2545ReceiveFixtureCaps(t, sourceCap77, destinationCap77, ...)`; `rfc2545ReceiveFixture` keeps source cap77 on, so its four other callers are unchanged |
| Receive helper | `rfc2545ReceiveForwardPublished(..., withdrawnAtReceipt)` returns recipient and published UPDATEs; `rfc2545AssertWithdrawnAtReceipt` checks the published MP_UNREACH (and legacy withdrawal) and that the recipient gets no announcement |
| `TestRFC2545ReceivedPairSecondAddressValidated` | source negotiates 77 only for `link-local-only-control`; four `cap77-source-*` cases (three malformed seconds withdrawn at receipt, link-local second kept) |
| `TestRFC2545ReceivedPairFirstAddressValidated` | caps are source x destination; loopback, multicast, IPv4-mapped first withdrawn at receipt under source 77; `fe80::`/`::` firsts are well formed per draft Section 6 and still withheld at egress |
| `TestRFC2545EffectivePairPolicyAndMixedSibling` | source never negotiates 77 except new `cap77-source-invalid-second-withdrawn` (mixed layout: legacy sibling withdrawn too, RFC 7606 Section 2) |
| Claims | RFC2545-3-1 claims of the three tests match their new cases; RFC2545-3-2 claims unchanged |
| Discrimination | 8 records re-recorded (`route revert`, producer `forward_next_hop.go::egressNextHopWithheld`) in `rfc/discrimination/rfc2545.json` |
| `TestRFC2545ReceivedPairSubnetConditions` (2026-10-19, after 990744517c) | `keepPair` split into `keepUnicast`/`keepVPN`; `vpn/global-off-link` now keeps the 48-octet pair (RFC 4659 Section 3.2.1.1 names the peer alone, owner decision 2026-10-19). RFC2545-3-3 negative claim narrowed to "on IPv6 unicast either off-link entity, and on VPN-IPv6 an off-link recipient"; the positive claim is unchanged. Approval recorded by `./le rfc approve unit reactor.TestRFC2545ReceivedPairSubnetConditions` (`tmp/commit-rfc-approved-3f5fcb0a.md`; re-run it if that file is gone when #13 commits). Both RFC2545-3-3 records re-recorded (`route revert`, producer `forward_next_hop.go::applyEgressNextHopScope`, overlay break). Reactor package green with the working tree as is (tags `ze_core ze_bgp`, 124s, no overlay needed: the untracked RFC 9774 files compile) |

Verification (overlay mapping foreign dirty `internal/plugins/iface` and
`internal/component/iface` files to HEAD): full reactor `-race` ok (144s),
`-run TestRFC2545 -count=5 -race` ok, full message `-race` ok, `go vet` clean.
Scoped lint (reactor + message): only 2 gocritic `appendAssign` findings, both in
the RFC 9774 session's `TestRFC9774SendAddPathFraming` file
(`reactor/rfc9774_send_test.go`).

Not committed, and why: `message/rfc7606.go` and `rfc7606_test.go` carry the RFC
9774 session's hunks, which need the untracked `message/rfc9774.go` (not in
HEAD). There is no hunk-level commit, and #13's
`reactor/session_validation_receive.go` needs the new
`ValidateUpdateRFC7606AddPath(..., linkLocalNextHop)` signature in that same file.
Checked four times up to HEAD 4af80acf2a: `rfc9774.go` still untracked, and
`04-rfc9774-as-set-deprecation.md` still lists 5b, 5c, items 6-10, A-5/A-6 and
the commit as open.

Next: once RFC 9774 lands, commit #13 (parent scratch `llnh-6-1-files.txt`,
plus `docs/features/bgp-protocol.md` with its MUP hunk for 691bd048e9, name both
in the body) with `reactor/rfc2545_forward_subnet_test.go` and
`rfc/discrimination/rfc2545.json`, via `./le commit create replace session
3f5fcb0a ... structural-red-ok "arch tier/check and iface-resolution reds are
stale; both gates pass when run now"`.

Owed (verdict work, not done here): `./le rfc check` reports RFC2545-3-1 and
3-2 audit verdicts STALE (re-judge with ze-rfc-audit), RFC2545-3-3 SHIFTED
(`./le rfc reseal`), and DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1 still
`unimplemented` in its audit JSON with no `{gap}` on its summary row.
Gates still owed: `./le rfc check` for these stems, `./le verify worktree`.

### Functional reds of the 9fe7a1cfe0 full verify (2026-10-19, gating + exabgp)

Fixed: cef2dda105 (unprivileged apply no longer deletes `ip6 ze_iface_rpf`; 26 of
37 gating reds + exabgp api-multisession), bc8625b45e (`update resolve rir` reads
the boot config via `storage.ReadActiveConfig`; storage-rir-restart.ci).

| Case | Cause | State |
|------|-------|-------|
| plugin ddos-* (4), interface-errors-show, interface-type-show; reload config-apply-ordering-coarse-root/-mixed-rollback, ospf-added-live, rsvpte-reload, traffic-usage-added-live | rpf | pass on rerun |
| plugin storage-ddos-restart; rsvpte 1-5; ospf 30 31 33 35 37 38 39 68; vrrp-show | rpf in logs | expected fixed, rerun unread |
| plugin storage-bfd-restart | first run never persisted a sequence. Not the bc8625b45e boot-record ordering: the fixture (`plugin_fixture_02_bfd.go`) reads `meta/bfd/auth/` keys with `storage.OpenReadOnly`, not the `file/active` mirror, and the daemon logs no error | red on rerun 2026-10-19. Cause found 2026-10-19: dd3eaf7773 runs every DUT on an in-memory store (`ze.test.storage=memory`), the fixture's `ze start` inherits it, and the .ci never declared `option=storage:value=disk`. Not caused by the RS work: journal row in `plan/journal/option-set-for-one-caller-changes-another.md`. With the option added the .ci passed (2.4s); the edit was reverted, the fix is open |
| plugin plugin-announce | quiesce failure | passes on rerun (load) |
| plugin vpn-withdrawal-inventory-rr-ipv4, -rs-ipv4 | an owed VPN withdrawal kept the RFC 8277 2.4 Compatibility field 0x800000 (S bit clear); bgp-rib read it as a label stack that never ends, answered `recovery: invalid NLRI for ipv4/mpls-vpn`, and the worker retired B, which then got the announce again on reconnect | fixed by "bgp: an owed labeled withdrawal asks the RIB in announcement framing" (`oweAnnouncementFraming`; `TestOwedVPNWithdrawalAsksInAnnouncementFraming` red before); both .ci pass |
| reload bgp-attach-grant-change-live | f749025a61 deleted the departure cut: `oweDeparture` owes a key only where `departedOwner` finds the source's ENDED session. bgp-rs's targeted down (grant change on a kept binding) runs on a live session, so nothing is owed and B never gets the withdraw. A generation alone cannot tell this from AC-18's reconnect | fixed by 96a24194eb: per-source revocation epoch `Peer.forwardRevocation`, advanced by `unbindProcesses`, recorded per adj-out owner, read by `adjOutOwnerCurrent` (Design Insights 2026-10-19). `TestAdjOutDepartureOwesOnlyDepartedSession/live-source-revoked-owes` red before; reactor/rib/rs/rr/cmd-update -race green; the .ci passes |
| plugin wellknown-no-export-withdraw-egress | two defects, seen with a debug overlay: (1) the mixed UPDATE's withdrawal of 198.51.100.0/24 is owed (bgp-rib does receive the source here) and decided about 1 ms later, after the same UPDATE's announce half and the fence went out, so both peers see the wrong order; (2) at the external peer the synthesized withdrawal of the NO_EXPORT 10.0.0.0/24 is owed, the RIB elects the source's own denied route, and `forwardUpdateCore` returns `no established peers to forward to`, not `errAllDestinationsSuppressed`, so the worker retires the session | red; (1) NOT edited: the .ci is RFC-tagged (`RFC requirement: RFC1997-Well-1 negative`), so changing its fence expectation to wait for the owed withdrawal needs the owner (`rfc-change-ok`). (2) re-run 2026-10-19 after 96a24194eb: the daemon now logs `recovery could not decide owed routes; resetting the session` peer=127.0.0.3 err=`source-DOWN recovery operation was not written`, i.e. `errRecoveryNotWritten` from `fwdPool.releaseItem` (forward_pool.go): the admission's queue item is released while unsettled. Not `admit` (it settles), not a write or flush failure (no Warn logged), not the bucket merge (it skips recovery items); the remaining candidates are `fwdBatchHandler`'s silent returns (`session == nil`, state not Established, `conn == nil`) and the overflow paths. The `no established peers` reading above predates this run. Next: a unit test over `queueRecovery` with an eBGP destination and a NO_EXPORT survivor, asserting the admission settles. 2026-10-19 later: that test (`TestOwedKeyNoExportSurvivorWithdrawsAtExternalDestination`) was GREEN before any change, so the NO_EXPORT survivor is not the producer: it settles as a withdrawal. The producer is `fwdBatchHandler`'s silent returns; the reset follows the test peer closing on its first mismatch, after which the late recovery admission meets a destination that left Established. Fixed: each early return settles every recovery admission with `errRecoveryDestinationClosed` (`closeRecoveryWrites`), and the worker ends quietly on it instead of resetting (`recoveryWorker.run`); `TestRecoveryAdmissionSettlesWhenDestinationClosed` and `TestRecoveryWorkerStopsQuietlyWhenDestinationClosed` red before, green after. The .ci still fails ONLY on cross-prefix ordering (defect 1), twice, no reset logged: internal `expect=bgp:conn=1:seq=3:withdrawn=18C63364` receives the 10.0.0.0/24 announce; external `expect=bgp:conn=1:seq=3:withdrawn=18C63364` receives `0004180A0000` (the 10.0.0.0/24 withdrawal alone), then the fence. Owner decision owed (`rfc-change-ok`). 2026-10-19, after abc56c7ca2 made the .ci order-independent: the external peer still never got the 198.51.100.0/24 withdrawal. Cause, seen with a debug overlay: the owed record for 198.51.100.0/24 held the bytes of 10.0.0.0/24. `adjOutWrite.owes` aliased the body; the merged withdrawal (P owed, the synthesized Q written) is rematerialized and copied over `s.writeBuf` before `applyStaged`, so the RIB was asked about Q, elected the source's denied route, and the key settled with nothing written. Internal was unaffected: its body was all-owed, applied before any overwrite. Fixed: `adjOutWrite.stageOwe` copies into pooled staging storage. `TestOwedWithdrawalKeepsItsNLRIWhenTheBodyIsRewritten` red before (owed nlri 180A0000), green after; the .ci passes 3/3; bgp-attach-grant-change-live and the fixture package pass |
| ui le-evidence-vpp-answers | `leEvidenceVPPAnswers` hard-coded 4 scenarios and 3 plugins; `vppScenarios` has 5 and the report names 4 plugins (6d1d92b1f3 added uRPF) | fixed 4ac8aea24d: expected set read from `testdeployment.VPPInterfaceFeatures` / `VPPReportedPlugins`, fake vppctl answers uRPF; passes |
| ui le-checks-answers | `le test sensitivity check`: assert-nothing 115 over baseline 114, from a test another session had in flight | sensitivity check now 114/114; .ci passes on rerun (157s) |
| runner le-derived-artifact-lifecycle | session hook built no `ai/CODE-TO-DOCS.md`; not reproduced | passes twice on rerun after `./le --update` (likely a stale bin/le) |
| exabgp api-multisession | the exa2 run stalled all 12 api tests at plugin stage 3 for 180s (host load about 20) | passes alone and among the 12 (4.1s); rpf fix holds |
| web chaos-stream-swaps, appliance crash-capture-panic-harvest-qemu | timeout under load | both pass on rerun (15s, 248s) |
| exabgp api-eor, -flow-merge, -health, -peer-lifecycle, -rib, -rr, -rr-rib, -sr-policy, -vpls, -on-exit, -disable | spec-exabgp-6 phases 4, 5, 7 (api-disable is AC-41) | spec scope, not implemented |

### Gate reds of the 9fe7a1cfe0 full verify: lint, compound-guard, iface-resolution, sensitivity, health (2026-10-19)

Logs: `tmp/verify-worktree-logs/20261018T205642.765815094Z-p84123-r1-9fe7a1cfe04d/full-23745759/`
stages 01, 04, 13, 31, 43. Each gate was rerun on the current HEAD.

| Finding | Cause | State |
|---------|-------|-------|
| lint: `recoveryExcludes` and `recoveryEligibleLocked` slicescontains, `recoveryCommand` ST1005 usage error ending in `...` (rib_recovery.go) | f749025a61 | fixed 95fae5058f |
| lint: `setLinkMTU` errorsastype (vpp/mtu.go) | 29a5fffb76 | fixed 95fae5058f |
| lint: embedlit in iface config_create_leftover_test.go (5), reconcile_learned_integration_linux_test.go (2), netlink rpf_integration_linux_test.go (2) | fb84f9bd7e, 0d6d873f2b, 6d1d92b1f3 | fixed 95fae5058f |
| lint: godot on the `parseAuthSecret` comment (bfd/config.go), whose RFC quote ends in a closing quote | 97686ea267 | fixed 95fae5058f |
| lint: DHCP godot (10), minmax, unparam `newV4Rig`, unused `v6EndUnspecified` and `v6KindUnspecified` | DHCP session | fixed by that session (3f3202c50a, b9405b0b6d); scoped golangci-lint over iface/dhcp: 0 issues |
| compound-guard: 16 `if a \|\| b { leave }` (relay_recovery x3, yang validator_instance x2 and validator_reference x4, ike engine config, dhcpserver options, iface/dhcp v4 x3 and v6, test/sim) | since pushed, so the gate no longer judges them | fixed db457cb3c2 |
| compound-guard: `ASPathSetSegment` in internal/component/bgp/message/rfc9774.go | untracked file of the in-flight RFC 9774 work | still red: another session's uncommitted file |
| lint: gocritic appendAssign x2 in reactor rfc9774_send_test.go (seen in a scoped lint of reactor) | untracked RFC 9774 work | still red: another session's uncommitted file |
| iface-resolution: InterfaceByName in the dhcp_v4 and dhcp_v6 exchange files | DHCP session | fixed by 6b5c69ec11 (allowlisted with reasons); `./le arch iface-resolution` OK |
| sensitivity: assert-nothing 115 > 114, TestSessionDownFollowsBufferedDeliveries | the detector did not follow a subtest passed to `t.Run` by name; 9886507d51 dispatches ten named subtests | fixed b8125bceb6 (named subtests followed one level, 3 selftest cases); check OK 114/114 |
| health: structural facts changed without regeneration | committed test/health/latest.json is stale | green on the working tree only: test/health/latest.json and quality-baseline.json are uncommitted by another session, who owes their commit |
| full `./le go lint run` on HEAD | three attempts refused: "lint inputs changed during planning" while other sessions edited | not re-proven whole-tree; scoped golangci-lint (linux with integration tag, and darwin) over every touched package: 0 issues in the edited files |

`bin/le` was older than the sources during this pass: a gate rerun after an
`internal/le` change needs `./le --update` or `./le --name <x>`.

### Unit and race reds of the 9fe7a1cfe0 full verify (2026-10-19)

| Red (stage) | Cause | State |
|-------------|-------|-------|
| reactor `TestRecoveryWaitsForFilteredReceivedPublication/worker-failure` (49), 2/80 | test defect: a recovery round rerun after B reconnects correctly does not exclude B | fixed 72849759cf, 100/100 |
| web `TestWebHandlerGoldenOutput/nav-show-l2tp` (49, 51) | cb8e90f006 changed a YANG description, golden not refreshed | fixed 3fbe231907 |
| sensitivity `TestTheRealCheckoutPassesTheRatchet` (49, 52) | 115 > 114 after 9886507d51 | fixed cb20f4bddc and b8125bceb6 |
| tftpserver `TestConfigTxReloadCutsTransferInFlight` (51), 3/20 | real: reload left an async socket close, a block from the old root could go out | fixed 0f3ca7969b, 30/30; RFC-tagged call-site edit, owner approval owed |
| interoplab/bgp `TestExaBGPCaseReadsEveryFixture` (49, 52) | spec-exabgp-6 AC-4 (a)(b) | open, spec scope |

### Doc and RFC gate reds of the 9fe7a1cfe0 full verify: stages 03, 23, 24, 37, 44 (2026-10-19)

Fixed: bab8668f8a (five renamed-symbol anchors), 0696cb63b2 (repo facts),
doc-reviewed shard `plan/doc-reviewed/3f5fcb0a.md` (three f171a5d47c claims),
reseal of nine audit files, 43837676a0 (179 discrimination records re-taken),
and the three untagged RFC-named test files renamed. `./le doc index check` and
`./le site facts check` are green on HEAD.

| Red | Cause | State |
|-----|-------|-------|
| doc wiring: claims on `writeUpdateBody` (fsm-established.md), `validateRFC7606Attrs` (core-design.md), `reactorMetrics` (001-pull-model-metrics.md) | other sessions' uncommitted edits to session_write.go, session_validation_receive.go, reactor_metrics.go | theirs |
| doc wiring: `ReplaceIPv6RPF` claim in guide/configuration.md | cef2dda105 (fresh commit by another session) | theirs, unreviewed |
| doc check verify: interop.md anchor `Runner.dockerArgs` | another session's uncommitted evidence.go removes it | theirs |
| doc check verify in place: 229 drift issues, ../gh-pages CLI catalog | gh-pages last published 2026-10-16, before the YANG clear-command changes of 10-17; the worktree verify does not see ../gh-pages | needs `./le site build` and a gh-pages commit |
| rfc: 40 Linux-guest discrimination records (VRRP rfc3768/5798/9568, rfc4659 configured transport) | `discriminate-record` needs `kernel <vmlinuz>` (`ze appliance kernel`) | 28 re-taken in cf6344e953. 12 owed: RFC5798-7.2-2, 7.2-3 (TestTxAdvertV4OnWire), RFC9568-8.1.1-2 +, 8.2.1-2 -, 8.2.2-1 +/- (nd_nonowner and owner_answer), 8.2.2-8 +/- (both units). Each refused "already failing before any break": the guest le build failed on in-flight edits (dhcp_v6_linux.go, then interoplab/pppoe `preflight()` vs 4c41912146). Re-take each with its stored route and producer once `cmd/le` cross-builds |
| rfc: RFC8092-6-3 TestRFC8092DuplicateLargeCommunityIsNotMalformed, RFC4271-8.2.2-11 TestRFC4271OpenConfirmKeepaliveKeepsTheConnection | the unit is red before any break on the working tree | not a defect: both units pass at HEAD (15 runs green); the red came from an in-flight tree. Re-taken in 29a5a504df |
| rfc: RFC4271-8.2.2-10 TestRFC4271OpenSentWellFormedOpenKeepsTheConnection | red under the break without a FAIL line naming the unit | re-take now names the unit (`--- FAIL` at line 216); same in-flight-tree cause. Re-taken in 29a5a504df |
| rfc: RFC2918-4-3 refresh-config-static.ci, RFC4659-3.2.1.1-3 rfc4659-vpnv6-link-local-off-subnet.ci | isolated binaries fail to build: `undefined: testdeployment` in another session's ui_fixture_le_evidence_vpp_answers.go | retake once it lands |
| rfc: 3 RFC8669 mutant records on applyRFC7606 | mutant route: needs the mutant site re-chosen | no gomu report exists; the same hand-made spurious-log mutant re-applied at the moved line (186 to 187) through a one-mutant report. Re-taken in 29a5a504df |
| rfc: SHIFTED rfc7606, rfc1350, rfc2348, rfc7440 | rfc7606.json holds another session's hunks; tftpserver handler_test.go is uncommitted | reseal written, not committed |
| rfc: STALE verdicts and unannotated unimplemented/not-applicable rows | listed below | need an independent judge (ze-rfc-audit) |

STALE, by the commit that staled them: MUP-3.3.6-5 (691bd048e9, plus no {gap});
RFC9830-2.4.2-12 (8d18d4c4c3, plus no annotation); LINKLOCAL-CAPABILITY-6-1
(test in an uncommitted file, plus no {gap}); RFC4659-8-5 (5a0190cb08,
4109986b04); RFC8277-3.2.2-3 and RFC2545-3-3 (4109986b04); RFC4271-5-7
(a9628fd589); RFC5880-6.7.2-2, 6.7.3-14, 6.7.4-8 (95fae5058f); RFC7606-5.1-2,
5.1-3 (839819ea2d); RFC8277-2.4-1 (f749025a61); RFC8654-5-4 (e30abc5e48).
Staled by uncommitted edits: RFC1350-4-3, RFC2545-3-1, 3-2, RFC7606-3.c-1,
5.2-1, 7.2-1, 7.16-1, RFC7950-9.1-1.

### Known reds of the full verify over fc539a33f3 (2026-10-19)

Ten red stages; each rerun targeted on HEAD 38ccef54f5 or later before acting.

| Red | Cause | State |
|-----|-------|-------|
| plugin ddos-flowspec-announce | message mismatch in the loaded full run | passes alone on HEAD (693ms); no change |
| plugin storage-ddos-restart | the fixture's ze inherited the in-memory test store, so the restart found no baselines | fixed: `option=storage:value=disk` (commit "test(plugin): storage-ddos-restart runs on the disk store"); red before, green after |
| plugin wellknown-no-export-withdraw-egress | RFC-tagged .ci | order-independent since abc56c7ca2; product defect (owed NLRI aliased the rewritten body) fixed 2026-10-19, passes |
| ui le-evidence-vpp-answers | "the YAML VPP deployment proof exited 1"; the proof's own output is not in the log. 4ac8aea24d was already in fc539a33f3 | passes alone on HEAD; cause under load unknown |
| ui web-user-removed-by-reload | observer: webuser not refused after reload, in the loaded run | passes alone on HEAD |
| web 81 `test/web/set-value-persists.wb` | `agent-browser open` exited 1 at step 6 under load | passes alone on HEAD (13.7s) |
| ospf TestNSSAImportIgnoresUnrelatedDownAndWithdrawnIntent (race) | SPF timer goroutine read the test's plain-pointer clock through `LSDB.Summary` | fixed 0a197a7b75: helper stops SPF; -race -count=20 green |
| doc wiring: fsm-established.md, peer-lifecycle.md, bgp-fsm.md | 96a24194eb (`Peer.forwardRevocation`), e30abc5e48 (`fwdBatchHandler` per-key) | fixed (commit "docs: Peer forwarding fences and per-key recovery staleness"); that page edit also silenced the fsm-established.md `writeUpdateBody` claim, which belongs to another session's uncommitted session_write.go |
| doc wiring: still drifting, this session's commits | structural-forwarding.md (`adjOutPath`, `recoverNLRIBatch`), core-design.md (`fwdBatchHandler`), forward-congestion-pool.md (`fwdBatchHandler`, twice), rib-storage-design.md (`adjOutPath`, `queueRecoveryOutcome`, `adjOutWrite.allow`, `recoverNLRIBatch`, `fwdBatchHandler`), config-reload.md and plugins.md (`unbindProcesses`), route-reflection.md (`recoverNLRIBatch`) | owed by 3f5fcb0a; not done |
| doc wiring: other sessions | iface/management.md `AddrInfo`, dhcp-server.md `findOption`, resolve.md and command-reference.md `configuredDelegationSources`, configuration.md `ReplaceIPv6RPF`, 001-pull-model-metrics.md `reactorMetrics`, core-design.md `validateRFC7606Attrs`, gaterun `Leader` (wiring) | theirs |
| interoplab TestCheckerPopulationMatchesProducer | 43d84584f8 made bgp-vpn-frr bespoke (`checkVPNFRR`) and left its `scenarioOperations` and `scenarioExtras` entries | fix written and backed out: `checkVPNFRR` is RFC-tagged and `./le commit create` refuses without `./le rfc approve unit bgp.checkVPNFRR`. The fix moves the five generic operations (two FRR session checks around two VPN-IPv4 routes, then the ipv4Vpn capability check) into `checkVPNFRR`, in the same order, and deletes both map entries. Needs the owner's approval |
| interoplab TestAnnouncementPlanOrderAndTimingFixture | 0411b53e91 added the overlay-index update to the evpn plan and did not repin the digest | fixed (commit "interoplab: repin the announcement plan digest for overlay-index") |
| lint orphans.go goconst, gaterun misspell, dockerpeers_signal_test misspell; wiring gaterun `Leader`; TestDockerDeploymentRefusesMissingKernelFeature | lab Docker cleanup, session d8e7dbbf (commits 2fe96cdffe, 9cbce6f200, 4f054354a3; state written 09:28) | active; left |
| compound guard in `zeV6LeaseVerdict`, unparam `keaLog.leased`, site facts interop.targets 10 vs 11 (Dockerfile.kea, 8bd003c1ce) | DHCPv6 lease maintenance, session 20123ea5 (state written 09:20) | active; left |
| labtest `TCPReady` wiring | 7edc3e7e89 | no longer reported by `./le doc wiring` on HEAD |
| TestExaBGPCaseReadsEveryFixture | spec-exabgp-6 scope | left |
| lint: session.go `asSetSendLog` unused, flowexport undefined `startExportProcess`, dhcp `v6Rig.awaitSent` unparam | in-flight work of other sessions | not investigated |

### Known reds of the full verify over 31f8aedb30 (2026-10-19)

Logs: `tmp/verify-worktree-logs/20261019T084431*/full-494414736/` (local only;
the causes are summarized here).

| Red | Cause | State |
|-----|-------|-------|
| unit cmd/ze/hub TestSIGHUPQueuedBehindTransactionRunsWhenItEnds/runs_once (310s, package 1170s) | test sequencing: it released the holder once the worker's first `load()` ran, but the worker loads before `tryAcquire`, so a descheduled worker took the freed lock on its first attempt and loaded once. Not a deadlock; the product reloaded correctly | fixed 5b409e1054: the test waits for the refused attempt to clear its staged candidate before releasing. Red forced with a temporary 300ms sleep after `load()` (exact verify message), green with the fix under the same sleep; -count=20 green |
| plugin as112-probe-anycast-not-loopback | fixture took its SSH port from a closed `127.0.0.1:0` listener; another process bound it first (`bind SSH server: ... 127.0.0.1:35439: address already in use`) | fixed eb1c0368f2: the .ci passes the leased `$PORT2`; passes |
| appliance crash-capture-ota-unaffected-qemu | appliance build: `e2fsck -f -n` on the perm image aborted (exit 12) during the loaded run | passes alone on HEAD (191s); recorded as load |
| appliance vpp-hugepages-qemu | appliance did not answer SSH within the bound (accel=kvm) in the loaded run | passes alone on HEAD (121s); recorded as load |
| plugin wellknown-no-export-withdraw-egress | product defect | fixed d44cc5dc98 |
| 11 ExaBGP api cases, TestExaBGPCaseReadsEveryFixture | spec-exabgp-6 scope | left |
| interoplab lint, guard and site-facts reds | sessions d8e7dbbf, 20123ea5 | theirs |

Open question, not verified: a SIGHUP reload reads the config before it asks
for the transaction lock, so a commit that takes and releases the lock in that
gap is followed by a reload of a tree read before the commit. Whether the
staged-candidate check (`ErrCandidateExists`) already refuses that case was not
checked.

## Gap #26 RFC4303-1-2 (integrity-only ESP): committed d64411a639, 08bfef9ee7 (2026-10-19)

- d64411a639: ENCR_NULL negotiable for Child SAs (the IKE SA still refuses it,
  RFC 7296 Section 5), docs, scenario `esp-integrity-only-null`.
- 08bfef9ee7: `{gap}` removed from RFC4303-1-2 and Support remaining rewritten.
  The ze_vpp RFC4301-4.2-1 test went red under `-tags ze_vpp` after d64411a639,
  because vppCryptoAlg now names the null cipher; it is re-scoped to NULL
  without integrity, and TestRFC4303VPPIntegrityOnlyESPIsInstalled asserts
  NULL with sha256 installed. Discrimination records: RFC4303-1-2 positive
  (engine, VPP) and negative (crypto); RFC4301-4.2-1 VPP negative re-taken on
  vppIntegAlg; four RFC5282-8 records re-taken.
- Interop `IPSEC_INTEROP_SCENARIO=esp-integrity-only-null ./le test integration
  interop-ipsec`: PASS over HEAD. Red walk: acceptESPEncryption's null branch
  forced to ErrNoProposalChosen, the harness rebuilt the image, the run FAILED
  (charon logged `selected proposal: ESP:NULL/HMAC_SHA2_256_128/`, Ze logged
  "IKE_AUTH accepted offer rejected ... NO_PROPOSAL_CHOSEN" and sent
  N(NO_PROP)). Fix restored with no diff against HEAD, rerun PASS.
- Owed: `rfc/audit/rfc4303.json` RFC4303-1-2 verdict is STALE and still says
  unimplemented (`./le rfc check` flags it): a ze-rfc-audit re-read, not a
  stamp. `rfc/audit/rfc4301.json` RFC4301-4.2-1 verdict is STALE (the VPP test
  changed). RFC7296-2.7-1 is SHIFTED (checkers.go moved) and needs
  `./le rfc reseal`, but `rfc/audit/rfc7296.json` holds another session's
  uncommitted hunks. The owner's answer is owed on the rfc-change-ok debt row
  in `plan/verification-debt/3f5fcb0a.md`.

## Gap #55 RFC2328-8.1-1 (OSPF unnumbered p2p): 3c2c96ec01, c7a04ba492 (2026-10-20); continuation owed

Shape: owner decision row "#55" in `plan/rfc-gap-difficulty-2026-10-18.md`.
The `{gap}` on RFC2328-8.1-1 in `rfc/short/rfc2328.md` STAYS until every
owed item below lands.

Done:
- 3c2c96ec01 iface: `ipv4 unnumbered <donor>` and `unnumbered-source`
  (`ze-iface-conf.yang`). `internal/component/iface/unnumbered.go` resolves the
  donor at parse (that is, at verify) and appends `<addr>/32` to
  `unit.Addresses`, so the normal netlink reconcile installs it. Refusals:
  with address, with dhcp, a missing, disabled, unnumbered or address-less
  donor, a source not on the donor, and a source alone. The donor name
  `loopback` means the loopback block. "First address" means unit-name order,
  then leaf-list order. Tests: `unnumbered_test.go`,
  `test/parse/iface-unnumbered-{refused,loopback}.ci`. Doc:
  `docs/features/interfaces.md`, "Unnumbered IPv4 units".
- c7a04ba492 OSPF: unnumbered means point-to-point plus a /32 mask
  (`interface_addr.go::interfaceUnnumbered`). The Hello mask is 0.0.0.0
  (`helloNetworkMask` in `interfaceRuntimeConfigLocked`). The Router-LSA Type 1
  Link Data is the ifIndex, with no stub link (`lsdb/origination.go::p2pLinkData`;
  `InterfaceInfo.Unnumbered` and `IfIndex` are set in `instance.go`). Commit
  refuses OSPF on an unnumbered unit that is not point-to-point
  (`unnumbered.go::checkUnnumberedNetworkTypes`, on both verify paths; OSPF
  ConfigReads now holds `ifcomp.ConfigRoot`). SPF guard:
  `spf/spf.go::p2pNeighborAddress` refuses Link Data in 0.0.0.0/8 (an ifIndex)
  as a next-hop address. Doc: `docs/guide/ospf.md`, "Unnumbered point-to-point
  interfaces", which lists what is missing. Update it as items land.

Owed, in order:
1. SPF next hop through an unnumbered neighbor. `v4NextHop.P2PNextHop` gets
   the root's own link. When that link's Link Data is an ifIndex, the next hop
   is the Full neighbor `neighbor` on that ifIndex's interface, and its address
   is the neighbor's Hello source (its donor /32). The spf package holds no
   adjacency, so either give the v4 strategy an adjacency source like
   `v6Strategy.NextHopSource` and `v6NextHop` (`afstrategy_v6.go`) or resolve
   by RouterID and interface in `ospfNextHopResolver.ResolveInterface`
   (`spf_wiring.go`). `lfa.go` (`neighborLinks`) and `tilfa.go` use the same
   source.
2. FIB: the neighbor address is off-subnet, so the kernel route needs
   `onlink` (the FRR shape `via X dev eth0 onlink`). The owner's text says
   "interface-only next hop". A dev-only route works on a true p2p device but
   needs proxy-ARP on Ethernet, so `onlink` with the neighbor address is the
   form that works on Linux. Ask the owner which form, or implement onlink and
   say so.
3. Donor down or address lost must take the link out of OSPF. Today iface
   installs the /32 at apply, from config, and does not withdraw it when the
   donor goes down. Iface must react to the donor's link and address events
   (remove the /32 from the unnumbered device, re-add it on recovery), and
   OSPF must treat the loss of the interface IPv4 address as InterfaceDown.
   Check first what OSPF does today when an interface address disappears.
4. RFC tags and discrimination. The RFC2328-8.1-1 claim is the IP source, so
   the strongest carrier is a netns or guest test showing that the OSPF socket
   on an unnumbered link sends from the donor address
   (`transport/backend_linux.go::interfaceIPv4` picks the /32). No rows exist
   for the 9.5 mask or the 12.4.1.1 Link Data, and the RFC scope is frozen, so
   add no rows. Record with `./le rfc discriminate-record` (overlay only).
5. Doctor: warn when the donor is not in OSPF (`internal/plugins/ospf/doctor.go`).
6. Interop: an `ospf-p2p-frr` variant with FRR `ip unnumbered lo`
   (`docs/architecture/testing/interop.md`). Prove it red by reverting
   `p2pLinkData` or `helloNetworkMask` and rebuilding.
7. Remove the `{gap}` text on RFC2328-8.1-1 and list the ids for a judge.
   Do not stamp audits.

The main thread owes these gates: `./le go lint run` over
`internal/component/iface` and `internal/plugins/ospf`, then
`./le verify current`. Both commits named structural-red-ok: arch tier/check
and arch iface-resolution were red only in a recorded verify run with no path
attribution, and both passed when run on the tree.

### Gap #55 continuation, 2026-10-20: items 1, 2, 3 landed; 4 to 7 owed

Route form decided by the main thread: `via <neighbor donor address> dev <if>
onlink` (the FRR form). `docs/guide/ospf.md` says the "interface-only next hop"
wording was ours, not the owner's.

- 41e11adf5c items 1 and 2. `spf/spf.go::v4NextHop.unnumberedNextHop`: a root
  P2P link whose Link Data is an ifIndex resolves through `LinkAdjacency`
  (the neighbor table's `NextHopOnLink`, keyed by ifIndex, which is the v2
  `InterfaceID`); `NewV4Strategy(adj)` is wired in `spf_wiring.go`. LFA and
  TI-LFA use the same source. The primary was already installed on-link when
  the next hop names its device (`spf/install.go`). The backup lost its device:
  `BackupInterface`/`BackupOnLink` now ride `locrib.Path`, the route-install
  RPC (`pkg/plugin/rpc/types.go`), `ribevents`, sysrib and `ecmp.go::backupPaths`.
  Tests: `spf/unnumbered_nexthop_test.go`, `sysrib_backup_test.go`.
- a0d9424aa6 item 3. iface `unnumbered_donor.go::reconcileUnnumbered`
  (level-triggered, worker in `register.go` runEngine, fed by link/addr events
  and every apply) withdraws or restores the /32. OSPF
  `transport/transport.go::addrWorker`: an IPv4 address leaving an open
  interface closes and re-opens it; the re-open fails with no IPv4, so the link
  stays out of OSPF until the address returns. The event carries the OS device
  name, so it fires only when the OSPF interface name is the OS name
  (documented). Tests: `unnumbered_donor_test.go`, `transport_addr_test.go`.

Not run, owed by the main thread: the full `go test ./internal/component/iface/
./internal/plugins/ospf/` (it failed to BUILD on a foreign uncommitted change in
`internal/component/config` — `choiceMembers` undefined in schema.go and
yang_schema.go — after the scoped tests had passed), `./le go lint run` over the
touched packages (iface, ospf, sysrib, locrib, routeinstall, plugin/server,
pkg/plugin/rpc), and `./le verify current`. `internal/component/plugin/server`
fails 30 tests on unregistered BGP event types (keepalive, bgp rib cache),
outside the two fields added there; `-run 'Route|Install'` passes.

Still owed, in order: item 4 (netns or guest test of the donor IP source,
RFC2328-8.1-1 tag, `./le rfc discriminate-record`), item 5 (doctor warning when
the donor is not in OSPF), item 6 (`ospf-p2p-frr` variant with FRR `ip
unnumbered`, prove red), item 7 (remove the `{gap}`, list ids for a judge, no
audit stamps). Update the "Not yet complete" paragraph in `docs/guide/ospf.md`
as each lands.

### Gap #55 continuation, 2026-10-20

State at hand-off (agent budget exhausted; only this section is committed):

1. `TestOSPFUnnumberedSourcesFromDonorAddress` guest red, cause FOUND, not a Ze
   transport defect. A diagnostic run in the guest showed: the AllSPFRouters
   membership is on the unnumbered veth (`/proc/net/igmp`), not the dummy; the
   guest root namespace has `all.rp_filter=1`, `default.rp_filter=1` (Alpine)
   and `default via 10.0.2.2 dev eth0`, so strict reverse-path checking drops
   the peer reply (source 10.255.0.2), whose route back leaves by eth0. Writing
   0 to `all` and the device `rp_filter` made the same reply arrive. The peer
   namespace (strict, no routes at all) receives fine. FRR does not touch
   rp_filter either. The fix was REFUSED by the pretool hook because the test
   is RFC-tagged: an edit needs `./le rfc approve unit
   transport.TestOSPFUnnumberedSourcesFromDonorAddress reason "<owner words>"`.
   Proposed edit: after `addDonorDevice`, a helper writes `2` (loose) to
   `/proc/sys/net/ipv4/conf/<nameA>/rp_filter` (the kernel takes max(all,
   dev), so only loose overrides a strict all; the value dies with the device).
   OWNER DECISION NEEDED: approve that edit, or pick another way (for example
   the QEMU harness resetting the guest rp_filter to the kernel default 0, which
   the gokrazy appliance runs).
2. Discrimination records: none written. The positive needs item 1 green
   (clean-before-break). The negative (producer `interfaceIPv4`, the no-IPv4
   `fmt.Errorf` mutated to nil, report `gomu-neg.json` in the parent session
   scratch) was refused only because another session's in-flight IKE edits
   broke the guest le build at that moment; rerun it.
3. Interop scenario `ospf-p2p-unnumbered-frr`, UNCOMMITTED and RED:
   `test/interop/scenarios/ospf-p2p-unnumbered-frr/` (ze.conf, frr.conf),
   entries in `internal/le/interoplab/bgp/checkers.go` (an FRR `opExec`
   flushes eth0's IPv4 and puts 10.255.0.3/32 on lo and eth0) and
   `check_extras.go`, donor constants in `names.go`. Run it with
   `INTEROP_SCENARIO=ospf-p2p-unnumbered-frr ./le --name <n> test integration
   interop` (the shared bin/le does not see new checkers). Measured: iface made
   Ze's eth0 unnumbered (only 10.255.0.2/32; the Docker /24 removed, though
   `default via 172.30.0.1 dev eth0` survives; rp_filter 2 on both sides). FRR
   shows eth0 UNNUMBERED, POINTOPOINT, Neighbor Count 0; Ze shows eth0
   point-to-point with neighbor-count 1. So Ze hears FRR and FRR never accepts
   Ze's Hellos. Cause unknown: next step is FRR `debug ospf packet hello recv
   detail` plus its log, or a capture on FRR's eth0, then Ze's Hello source,
   mask and TTL, and whether FRR's loose rp_filter with no route to 10.255.0.2
   drops it (if so the scenario, not Ze, gives FRR a route or rp_filter 0).
   Also seen: Ze's passive donor `zeloop` (dummy) reports `state down`, which
   would keep the donor out of the Router-LSA; check it after the adjacency.
   The red proof (revert the `multicastDevice` ifindex) is owed after green.
4. Done but uncommitted: the two stale `{gap}` comments in
   `internal/plugins/ospf/transport/rfc2328_backend_linux_test.go`. Not done:
   the `{gap}` removal in `rfc/short/rfc2328.md` (summary row and the 8.1-1
   row), the `docs/guide/ospf.md` "Not yet complete" paragraph (add that strict
   reverse-path filtering with a default route elsewhere drops the neighbor's
   Hellos until its route is learned), the judge listing. `features/ospf.md`
   Interop list untouched (it needs a recorded run).

### Gap #55 status, 2026-10-20 (later)

- Item 3 closed: `ospf-p2p-unnumbered-frr` is green, red with unicast forced,
  green again. Two causes: the lab (FRR lost its only routes and loose
  rp_filter dropped Ze's Hellos; the checker now gives FRR a default route out
  of eth0) and a Ze defect, fixed in 6e810d411b: on point-to-point Ze sent DD,
  LSR, LSU and direct LSAck unicast to the neighbor's donor, where RFC 2328
  Section 8.1 says the destination is always AllSPFRouters.
- `zeloop` reporting down is by design (passive interfaces are Down and still
  in the Router-LSA); the donor route is learned by FRR.
- The comment fixes in `rfc2328_backend_linux_test.go` landed with the
  scenario commit; the guide's reverse-path note landed with the checker fix.
- Still owed: the guest test `TestOSPFUnnumberedSourcesFromDonorAddress` is red
  on the VM's strict rp_filter, waiting for the owner to approve setting
  rp_filter 2 on its test link (the test is RFC-tagged); then its positive
  and negative discrimination records (`gomu-pos.json`, `gomu-neg.json` in
  the session scratch), the `{gap}` removal in `rfc/short/rfc2328.md`, the
  guide's "Not yet complete" line, and a judge for RFC2328-8.1-1.

## ExaBGP 6 spec, Phase 1 AC-4 third package (2026-10-20, Linux): continuation owed

- Committed (see `git log --grep "4.2 route steps"`): the Go mock reads 4.2
  `N:announce:`/`N:withdraw:` steps, the 4.2 `notification:send-notification`
  and 5.0 `open:send-unknown-message` options, and `allowed:`/`allowed-error:`
  (`internal/le/interoplab/bgp/exabgp_old.go`); old-5.0 api-notification
  states `frames:none`; allowances for conf-extended-attributes (SKIP
  observed), conf-hostname and conf-cap-software-version (ze's observed OPEN
  bytes, SKIP not yet observed). old-4.2: 22/34.
- UNCOMMITTED, mine, held on purpose: `internal/le/test/functional/exabgp.go`
  and `exabgp_test.go` (`exaBGPOldSuites`, the old populations in the stage
  once, under the main pin; `TestExaBGPStageRunsTheOldPopulationsOnce`, red by
  mutation then green, and the children count 8 to 10). Land only when both
  populations are green. Also uncommitted: the `unknown-message.ci` allowance
  (expects `...0016030103FF`, RFC 4271 Section 6.1), not yet observed.
- Run the populations with `./le --name exa6 test exabgp old-4.2 --all`: the
  mock is `le` itself and the shared `bin/le` is stale after a mock edit.
- Blocker for item (e): under parallel load ze's startup fails "stage stalled
  ... no plugin progress for 5s while waiting for stage 3" (bgp-rib,
  bgp-adj-rib-in, bgp-bmp at declare-capabilities); 4 of 4 allowance cases
  died of it on 2026-10-20. Diagnose it first (`plan/journal/plugin-startup-barrier-deadlock.md`).
- Remaining reds and their first error are in the spec's Phase 1 "AC-4 state
  (2026-10-20, third package)" line. Phases 2-8 untouched.
- Gates owed: `./le go lint run` on `internal/le/interoplab/bgp`, `./le test unit all`.

### 2026-10-20 RFC 4301 gaps #50/#51 (Q6 B, Q7 A): done in the tree, NOT committed

- Implemented and green, uncommitted (agent ran out of budget at the commit step).
  Files: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md, docs/guide/ipsec.md,
  internal/component/ike/dataplane/{dataplane.go,vpp_policy.go,recording_zetest.go,
  register_recording_zetest.go,recording_zetest_test.go,recording_shipped_test.go},
  internal/component/ike/engine/{apply.go,apply_test.go,bypass.go,child.go,
  reconcile_test.go,register.go,register_test.go,unmatched.go,
  unmatched_failclosed_test.go,rfc4301_spd_lifecycle_test.go},
  rfc/discrimination/rfc4301.json, rfc/short/rfc4301.md, test/weakened/3f5fcb0a.md
  (4 rows appended), the 3 test/ipsec and 5 test/reload `.ci` files that ran
  `unmatched bypass` unprivileged (now `ze_test_ike_dataplane=recording`).
- Evidence: go test engine+dataplane+cmd (tags ze_vpp) ok; dataplane -tags zetest ok;
  `./le test ipsec --pattern ipsec-` and `./le test bgp reload --pattern ipsec` all PASS;
  ipsec-status-show.ci catch-all assertion red with a wrong priority, green restored;
  revert record for TestRFC4301FinalEntryIsUnderEveryZeSPDEntry written.
- To land: clear stale tmp/commit-msg-* (create said "no free message tag"), rerun
  `./le commit create replace session 3f5fcb0a` with those files, structural-red-ok
  (arch tier check and iface-resolution both exit 0 on the tree), and run the printed
  script. tmp/commit-3f5fcb0a-z-cd2c6d.sh / -7c470b.sh exist but which is this one was
  not seen; do not run either blind.
- Judge owed (audits stale, not stamped): RFC4301-5.2-1, RFC4301-5.2-9, RFC4301-4.4.1-3,
  RFC4301-5.1-7. Gates owed: `./le go lint run` on internal/component/ike/..., verify.
- Path (a) VPP stays the only `{gap}` on 5.2-1/5.2-9 (owner checks VPP first).

## ExaBGP 6 spec, Phase 1 AC-4 fourth package (2026-10-20, Linux): continuation owed

- Committed: 5cf57ad1aa (mock reads COMMUNITIES and EXTENDED COMMUNITIES as
  sets; 4.2 conf-new-v4, conf-vpn green) and ae49f963d0 (six allowances, each
  observed SKIP). Counts now: old-4.2 24 pass, 4 skip, 6 red of 34; old-5.0
  52 pass, 7 skip, 13 red of 72 (fresh full run old50-r3 plus the two 5.0
  allowances run alone).
- Still held uncommitted until both populations are green: the stage change
  (`internal/le/test/functional/exabgp.go`, `exabgp_test.go`) and the 5.0
  `unknown-message.ci` allowance (now observed SKIP).
- Two ze defects found, owed fixes (they block AC-4 green): flowspec
  redirect-to-IP drops the MP_REACH next hop (4.2 and 5.0 conf-flow); an API
  flow withdrawal goes out as an attribute-only UPDATE with no MP_UNREACH
  (5.0 api-flow). Spec AC-4 fourth-package line lists every remaining red with
  its first error.
- Gates owed: `./le go lint run` on `internal/le/interoplab/bgp`.

## ExaBGP 6 spec, Phase 1 AC-4 fifth package (2026-10-20, Linux): continuation owed

- Committed 4358372547: FlowSpec redirect-to-IP (0x0800) carries its MP_REACH
  next hop on all three origination rails; old-5.0 conf-flow PASS, old-4.2
  conf-flow SKIP on an observed allowance (RFC 8956 Section 3.1 offset pattern).
- 5.0 api-flow is not a plain ze defect: ze withholds a withdrawal of a flow it
  never announced and sends the attribute-only UPDATE on purpose
  (`buildWithheldWithdrawUpdate`). Next step: verify that function's claim
  about ExaBGP 6.0 `api/api-flow.ci` frame 2 (the frame follows a
  `cmd:attributes` line), then allow or fix. Spec AC-4 fifth-package line.
- Still held uncommitted until both corpora are green: the stage change
  (`internal/le/test/functional/exabgp.go`, `exabgp_test.go`) and the 5.0
  `unknown-message.ci` allowance.
- Remaining reds: as the fourth package lists, less conf-flow (both corpora).
- Gates owed: `./le go lint run` on `internal/component/bgp/reactor`,
  `internal/component/bgp/message`, `internal/core/bgp/attribute`,
  `internal/le/interoplab/bgp`.

## ExaBGP 6 spec, Phase 1 AC-4 sixth package (2026-10-20, Linux): continuation owed

- Committed febfd66217: a withheld withdrawal writes nothing (attributes-only
  UPDATE removed; the ExaBGP 6.0 contract it cited does not exist at the pin
  685f772e4). 5.0 and 6.0 api-flow PASS, 4.2 api-flow SKIP on an allowance.
  Mock unexpected-message error now names the first owed frame.
- Lead: api-flow-merge (6.0 and 5.0) fails on the bridge refusing
  `interface-set` extended communities: absent feature, ask the owner for scope.
- Not yet diagnosed: 4.2 api-teardown; timeouts api-eor, api-nexthop-self,
  api-vpls, conf-ipself4/6, extended-nexthop, api-rib; 5.0 api-multiple-api,
  api-rr, api-rr-rib. Full old-4.2 and old-5.0 runs not done this package.
- Still held uncommitted: stage change (`internal/le/test/functional/exabgp.go`,
  `exabgp_test.go`) and the 5.0 `unknown-message.ci` allowance.
- Gates owed: `./le go lint run` on `internal/component/bgp/reactor`,
  `internal/le/interoplab/bgp`; `./le test unit all`.

### ExaBGP 6 spec, Phase 1 AC-4 seventh package (2026-10-20)

- Committed 5430bd457c: migrate injects bgp-rib for any neighbor or template a
  process feeds (ExaBGP's adj-rib-out defaults to true); old-4.2 api-teardown
  passes. Committed (main thread): mapIPv6RouteIPv4NextHop, an IPv6 route with
  no RD and an IPv4 next hop migrates to `::ffff:v4`, as 6.0 at the pin sends.
- Next: sync `test/exabgp-compat/etc/extended-nexthop.conf` lines 26-30 to the
  pin (`next-hop 170.170.170.170`), run encoding extended-nexthop.
- Allowances to write and observe SKIP (run at the default `-t 180s`): 5.0
  extended-nexthop (4-octet next hop for IPv6, RFC 2545 Section 3), 5.0
  conf-ipself4 (self = the peer's own 127.0.0.1, RFC 4271 Section 5.1.3), 4.2
  and 5.0 conf-ipself6 (self = ::1, the peer's address and not global).
- Open: 4.2 api-vpls (bridge has no `announce vpls endpoint`, scope question),
  4.2 api-eor (EOR missing at seq 3), 4.2 api-nexthop-self (bridge script
  restart loop), api-rib (announce then clear sends the route), 5.0
  api-multiple-api, api-rr (bridge emits `nlri add -refresh`, daemon refuses
  "unexpected flag -refresh"), api-rr-rib; api-flow-merge waits on the owner
  (interface-set).
- Still held uncommitted: the stage change (`internal/le/test/functional/
  exabgp.go`, `exabgp_test.go`) and the old-5.0 `unknown-message.ci` allowance.

### ExaBGP 6 spec, Phase 1 AC-4 seventh package, second agent (2026-10-20)

- Committed 40faec5c95: `mapIPv6RouteIPv4NextHop` also maps a bare IPv6 host
  route (no `/len`, as upstream writes it); `extended-nexthop.conf` synced to
  the pin; 6.0 encoding extended-nexthop PASS.
- Committed 2c46dd325c: allowances, each observed SKIP at -t 180s: old-5.0
  extended-nexthop, conf-ipself4, conf-ipself6; old-4.2 conf-ipself6.
- Committed 7fc1f19f28: bridge JSON in ExaBGP's spacing (`MarshalExabgpJSON`,
  fleet and single-event path). Committed e3876abb19: bridge translates
  `announce route-refresh <afi> <safi>`. api-rr PASS under 6.0 and old-5.0.
- Next, needs a design call: api-rr-rib (6.0, 5.0) and api-rib. ExaBGP
  `clear adj-rib out` withdraws every Adj-RIB-Out route (upstream
  `neighbor_rib_out_withdraw`); the bridge sends ze's `clear bgp rib out`, a
  replay. ExaBGP `flush adj-rib out` is that replay (`neighbor_rib_resend`,
  with BoRR/EoRR under RFC 7313); the bridge sends `request peer flush`.
  ze has no withdraw-all-Adj-RIB-Out command. Options: a ze RIB command, or
  bridge-side tracking (which misses config routes). Journal row updated in
  `plan/journal/delegation-hands-the-callee-a-different-argv.md`.
- Not reached: 4.2 api-eor, 4.2 api-nexthop-self, 5.0 api-multiple-api; full
  old-4.2/old-5.0 runs. Stage change and old-5.0 `unknown-message.ci` still
  held uncommitted (not all non-owner reds are gone).
- Gates owed: `./le go lint run` on `internal/exabgp/bridge`,
  `internal/exabgp/migration`, `internal/plugins/exabgp/bridgerun`;
  `./le test unit all`; the `exabgp-test` stage (api suite under every pin,
  since the JSON spacing reaches every JSON script).

### ExaBGP 6 spec, AC-4 ninth package (2026-10-20, Linux): continuation owed

Three 6.0 api cases were handed over: api-peer-lifecycle, api-disable (AC-41), api-on-exit (AC-22..AC-24).

- **api-peer-lifecycle: done, 8dfa2fcfd5.** `create neighbor ... api <p>` now attaches `exabgp-bridge`
  (the script name was attached before, a process ze does not run). `bridge.PluginName` is the one
  declaration. The `.ci` JSON line carried mock defaults (asn 65533, router-id 10.0.0.2); corrected to the
  create line's values. PASS under the 6.0 pin. Still owed on this item: a script that HAS a feed list
  (some config neighbor names it) does not gain the created peer in its scope (`Translator.Peers` and
  `script.grants` are built once in `bridgerun.New`), so its unaddressed lines skip the created peer.
  ExaBGP adds the peer to `api['processes']` with every grant false (`_build_neighbor`, `flatten_api({})`,
  `reactor/api/command/peer.py` at 685f772e4): commands from the process reach it, no event goes to the
  process. Fix: `Translation` carries the `api` names; on a dispatched create the fleet adds the peer with
  empty events to each named script (lock translator and grants, both read on other goroutines);
  `delete neighbor` removes it.
- **api-disable (AC-41): STOPPED, owner question.** The AC needs ze to hold a peer administratively down
  from config and to enable or disable it at runtime with a Cease message. ze has none of it: no peer leaf
  for "start down" in `ze-bgp-conf.yang`, no peer start/stop command (only `request peer <s> teardown
  <subcode>`, after which the peer reconnects), and the bridge does not translate `neighbor <ip>
  enable|disable "<msg>"`. The migration alone cannot satisfy the AC. Question for Thomas: which ze
  surface carries it (name and place of the config leaf, for example `peer <p> { shutdown true }` or an
  admin-state leaf; and the commands, for example `request peer <s> disable [message]` and `enable`)? It
  is a new ze config leaf and new commands, which the spec's Integration Checklist does not name.
- **api-on-exit (AC-22..AC-24): not started; design gathered.** Prerequisite: per-script API version
  detection (X-1, X-2), which does not exist yet (phase 2 not done). Pinned source
  (`dispatch/version.py`): first word in {daemon, session, system, rib, peer} is 6; `neighbor` or a word
  `translate_v4_to_v6` translates is 4 (`V4_SIMPLE_TRANSLATIONS` in `dispatch/v4.py`: shutdown reload
  restart status enable-ack disable-ack silence-ack enable-sync disable-sync reset ping bye help version
  crash queue-status; `api version`; `show adj-rib in|out`; read the rest of `translate_v4_to_v6` for
  announce and withdraw); anything else undecided. `Processes._on_exit`: the leaf wins (by=on-exit); else
  API 4 keeps, any other withdraws (by=API 6 or API undecided); log `process.exited process=<p>
  routes=<keep|withdraw> by=<...>`. Runs on a child's death only (`_handle_problem`), never at daemon
  shutdown (`script.supervise` already returns on ctx or stopped). Ownership (`withdraw_owner`,
  `rib/outgoing.py`): the last announcer owns a route, any withdraw clears it; on exit each owned route is
  withdrawn, or the configured route it replaced is re-announced. Plan: `bridge.Command` gains its
  withdrawal text, set by each announce builder (`oneCommandPerNLRI` renders the same tokens with
  `nlriDel`; flow in `bridge_flow.go` via `r.key(verb)`; the sr-policy announce/withdraw pair in
  `bridge_command.go`); a fleet ledger keyed by (selector, RouteKey identity) records owner and withdrawal
  after a successful dispatch; `supervise` applies the policy after `reap`, dispatching the withdrawals and
  a per-selector flush before `start`. YANG leaf `on-exit` (enum keep, withdraw; no default) in
  `ze-exabgp-bridge-conf.yang`, `Script.OnExit`, parse in `bridgeplugin/config.go`; the migration carries
  it (`migrate.go` process leaves, `exabgp.yang` process list). Docs: `exabgp-bridge.md` (respawn
  section), `docs/exabgp/exabgp-migration.md`. Tests: `api-on-exit.ci` (v6 script, withdraw), new
  `api-on-exit-v4-keep.ci`, `TestMigrateOnExitAndAPIVersion`. Open design point: re-announcing a replaced
  configured route needs the engine, since the bridge cannot see config routes.
- Gates owed for 8dfa2fcfd5: `./le go lint run` on `internal/exabgp/bridge`, `internal/plugins/exabgp/...`,
  `internal/exabgp/migration`; the `exabgp-test` stage (api under both pins and old-4.2/old-5.0, since
  `api-peer-lifecycle.ci` changed).

## Tenth package (exa6c, 2026-10-20): ping, SR Policy attributes

- Done: 1177ffc136 bridge answers `ping` (ExaBGP 6.0 bytes; `bridge_ping.go`, page `docs/architecture/exabgp-bridge.md`).
  af18a709d8 AC-28 API half: sr-policy `add` sections reach `parseConfigRoute` (page `docs/architecture/api/update-syntax.md`).
- 6.0 api-health: red on `status` (refused). ExaBGP's answer (reactor.py `status`): text lines `ExaBGP Daemon Status`,
  `====================`, `Version:`, `UUID:`, `PID:`, `Uptime: Xh Ym Zs`, `Peers: N`, `  - <name>: <fsm>`; JSON
  `{"version","uuid","pid","uptime","start_time","peers"}`. Needs peer FSM state from ze: the bridge's dispatch
  answer carries ok/error only today (`pendingResponses`), so the payload path must be added first.
- 6.0 api-sr-policy: red on the withdrawal frame: ze sends MP_UNREACH plus ORIGIN/AS_PATH/LOCAL_PREF for
  `update text mp ipv4/sr-policy del ...`; ExaBGP sends MP_UNREACH alone. Start in `reactor_api_batch.go`
  (where a withdraw-only batch gets mandatory attributes). Not diagnosed.
- 6.0 conf-watchdog: not started (sequence 2 frame never arrives).
- AC-28 still owes `test/plugin/update-text-sr-policy-attributes.ci` and `bgp-exabgp-bridge-sr-policy-gobgp`.
- Gates owed for 1177ffc136 and af18a709d8: `./le go lint run` on `internal/exabgp/bridge`,
  `internal/component/bgp/plugins/cmd/update`, `internal/component/bgp/plugins/nlri/srpolicy`,
  `internal/component/plugin/registry`; the exabgp stage (6.0 api) and `./le test functional` plugin suite
  (the sr-policy refusal wording changed in `parseConfigRoute`).
