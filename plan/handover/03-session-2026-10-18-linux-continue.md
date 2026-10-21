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

### ExaBGP 6 spec, Phase 2 first package (2026-10-20, Linux): continuation owed

- **Done (X-1 to X-8, AC-5..AC-9, bridge half of AC-10):** per-script API version detection and hold,
  `api version` command, `exabgp.api.version` env leaf and migration, per-version formats (v4 text,
  v4 JSON `4.0.1`, v6 JSON), YANG `encoder` default text. Details and evidence: the spec's "Phase 2 state
  (2026-10-20, first package)" line. Cases `api-version-detect-v4`, `api-version-detect-v6`,
  `api-version-command` green against ExaBGP main (`PYTHONPATH= uv run --no-project --python 3.12 --with
  paramiko --with 'exabgp @ git+https://github.com/Exa-Networks/exabgp@685f772e4ede5c818d2ff5c84f4ae040afd37be6'
  ./le --name exa6e test exabgp api --dut exabgp <cases>`) and against ze; red against ze with detection
  disabled.
- **Next package (Phase 2 rest):** X-9/X-10 (OPEN capabilities: v4 JSON keyed by code without `code`, v6
  keyed by name with `code`; `ReadEvent` carries no capabilities today), X-11 (5.x shapes: label as value,
  flow `string` ending with next-hop, one-cluster list in brackets), X-12 (v4 NOTIFICATION data: Shutdown
  Communication decoded, else the wire hexed once). `ExabgpJSON(format)` already takes the format, so each
  shape branches there. Then `api-version-env.ci` (AC-10 functional; ExaBGP reads `exabgp.api.version` from
  its env, the case needs an env line in proof mode) and AC-6's "one warning names the process" proof.
- Open point: the standalone runner `Bridge` (`internal/exabgp/bridge/bridge.go`, `translateAndForward`, the
  MuxConn path) has no version state and writes 6.0 JSON; the spec puts version state in `bridgerun`, so
  either that runner adopts `bridge.Format` too or it is recorded as v6-only. Not decided here.
- Gates owed: `./le go lint run` on `internal/exabgp/bridge`, `internal/plugins/exabgp/...`,
  `internal/exabgp/migration`, `internal/component/config`; the exabgp stage (api both pins, encoding,
  old-4.2/old-5.0), since the encoder default and the v4 JSON envelope changed what scripts read; the
  config YANG tests for the new `version` enum leaf.

### ExaBGP 6 spec, Phase 2 second package (exa6e, 2026-10-20, Linux): continuation owed

- Done: 6bc5593f8f X-9/X-10/X-12 (OPEN capabilities by code for v4, by name with `code` for v6;
  NOTIFICATION `data`/`message` per version, text line too); fbec2a2061 X-11 (v4 label `[[100]]`, flow
  `string` + next-hop, v4 text labeled/VPN route). Evidence and the AC-13 wording conflict: the spec's
  "Phase 2 state (second package)" line. Unit tests red then green (pasted in both commit bodies).
- Api population against ze after both commits (`./le --name exa6e test exabgp api --all`): 40/47, reds
  api-disable, api-eor, api-health, api-on-exit, api-rib, api-rr-rib (the six known reds of ae033ddb68)
  and api-healthcheck-module, NEW against ae033ddb68's 41/47 and red again when run alone (frames of
  sequence 1 never arrive; the case is `encoder text`). Not attributed: commits from other sessions landed
  in between (04e4661dc0 and earlier) and a foreign hunk sits in `test/exabgp-compat/etc/run/api-sr-policy.run`.
  Next: run it at ae033ddb68 vs fbec2a2061 to tell whether my text-encoder change (OPEN hold_time,
  notification data, labeled routes) reaches it.
- NOT run (owed): `./le test exabgp encoding`, `old-4.2`, `old-5.0` populations; lint on
  `internal/exabgp/bridge`.
- NOT done: `api-version-env.ci` (AC-10 functional); AC-6 "one warning names the process" proof.
- Standalone runner decision: the spec does not make `ze exabgp plugin` standalone mode
  (`internal/exabgp/bridge/bridge.go`, `translateAndForward`, `dispatchBatch`) v6-only, so under no-layering
  it adopts the same path: move the per-script version state now in `bridgerun/version.go`
  (`detectAPIVersion`, `adopt`, `resetAPIVersion`) into package `bridge` as one type both runners hold, have
  `dispatchBatch` detect and refuse cross-version lines, and render with `ReadEvent(...).ExabgpJSON(FormatFor(...))`
  / `AppendText` instead of `zebgpToExabgpJSON` (which then has no shipped caller and goes). The standalone
  runner has no `encoder` flag: state in the change whether it gains one or stays JSON (ExaBGP's default is
  text, X-5).

### ExaBGP 6 spec, AC-4 eleventh package (exa6c, 2026-10-20, Linux): continuation owed

- Done: 76b51c8946 api-sr-policy fixture corrected. The real ExaBGP 6.0 pin 685f772e4
  (`uv run --no-project --with "exabgp @ git+...@685f772e4..." ./le test exabgp api --dut exabgp api-sr-policy`)
  sends the sr-policy withdrawal as MP_UNREACH + ORIGIN/AS_PATH/LOCAL_PREF (`collection.py` `_attribute_sets`:
  bare only when every MP withdrawal is unicast/multicast). Ze's producer was right; no Go change. 6.0 api
  api-sr-policy PASS. Under `--dut exabgp` the pin never sent the sr-policy ANNOUNCE (first frame received was
  the withdrawal), so the fixture's announce frame is unverified against ExaBGP.
- Done: 9dea34f1b9 `test/plugin/update-text-sr-policy-attributes.ci`
  (`./le test bgp plugin --pattern update-text-sr-policy-attributes`): PASS; red with
  `NLRISectionCarriesAttributes=false` in `srpolicy/register.go` (restored). Ze's update-text rail writes
  MP_REACH first, then ORIGIN/AS_PATH/LOCAL_PREF/attr 23.
- In progress, uncommitted: `test/interop/scenarios/bgp-exabgp-bridge-sr-policy-gobgp/{ze.conf,gobgp.toml}`,
  `internal/le/interoplab/bgp/register_exabgp_bridge_sr_policy.go` (vet ok). Unknowns: whether ze's config
  parser keeps `\040` in the printf `run` string (the bridge splits `run` on whitespace); GoBGP's text for the
  Preference sub-TLV (guessed "Preference: 300"); the comment "TunnelEncapSubTLV-free NLRI text" in the
  register file is wrong. Then prove red (break af18a709d8's hand-off, rebuild the image), add the inventory row
  to `docs/architecture/testing/interop.md` (foreign uncommitted hunks there: check), commit.
- Not started: conf-watchdog (helper `le test interop-bgp exabgp-api watchdog`, config `api-watchdog.conf`,
  sequence 2 = withdraw 77.77.77.77/32 `0005204D4D4D4D0000`); api-health `status` design.

### ExaBGP 6 spec, AC-4 twelfth package (2026-10-20, Linux): continuation owed, nothing committed

All three changes below are UNCOMMITTED and none is green yet.

- **sr-policy GoBGP scenario** (`test/interop/scenarios/bgp-exabgp-bridge-sr-policy-gobgp/{ze.conf,gobgp.toml}`,
  `internal/le/interoplab/bgp/register_exabgp_bridge_sr_policy.go`). Fixed: ze's config tokenizer
  (`internal/component/config/tokenizer.go`, quoted-string escapes) turns `\n` into a newline and drops the
  backslash of an unknown escape, so `\040` reached printf as `040`; the run string now writes `\\040`/`\\n`
  (comment in ze.conf says why). GoBGP v3.31.0 prints Preference as `{Flags: 0x%02x, Preference: %d}`
  (`pkg/packet/bgp/sr_policy.go`), so the checker matches `Preference: 300}`; the bad NLRI comment is fixed.
  The shared `bin/le` does not see the untracked register file ("has no Go checker"): run with
  `INTEROP_SCENARIO=bgp-exabgp-bridge-sr-policy-gobgp ./le --name exa6c test integration interop`.
  Current RED: assertion 2, `gobgp global rib -a ipv4-srpolicy` exits 1 on every poll (session up). The
  script's line reached the bridge (WARN "uses the deprecated ExaBGP API 4" for announce-sr-policy, from the
  Phase 2 detection: a bare `announce` is v4). Next: read GoBGP's exit-1 text (empty table, or refused family),
  then whether the bridge's dispatch of the v4-held line succeeded (run ze with `ze.log=debug`). Red proof and
  the inventory row still owed. `docs/architecture/testing/interop.md` holds a FOREIGN RADIUS hunk (FreeRADIUS
  accounting port, radclient `-i`) from the RADIUS spec session: the row was NOT added; add it once that hunk lands.
- **conf-watchdog (6.0 encoding)**. Diagnosed two causes, neither a ze daemon defect yet proven:
  1. Run by hand, `run "le test interop-bgp exabgp-api watchdog"` finds no `le` on PATH (`exec: "le":
     executable file not found`, logged only at DEBUG as "post-startup callback failed": that DEBUG level
     for a script that never starts is itself a user-facing-error defect, owed a journal row). The functional
     stage puts the binaries dir on the child PATH (`internal/le/test/functional/binaries.go`); by hand use
     `PATH=$PWD/bin/le-exa6c:$PATH ./le --name exa6c test exabgp encoding conf-watchdog`.
  2. The helper profile sent ONE announce/withdraw pair and exited after 1s, while the 6.0 fixture expects
     three cycles (sequences 2..6) from upstream `watchdog.run`, which loops until its parent dies. Uncommitted
     fix in `internal/le/interoplab/bgp/exabgp_helpers.go`: `exaProfile.repeat`, `repeatExaCommands`,
     `exaInputEnded`; the watchdog profile repeats. With `le` on PATH it is still RED: the bridge logs
     "exabgp-bridge script restarted too often; it stays stopped" (5 starts in 1m4s) and sequence 2 never
     arrives. Next: find why the looping helper exits (stdin closed by the bridge? a write error?), and why
     ze announced 77.77.77.77 at peer up ("watchdog resent on reconnect count=1") although the config says
     `watchdog dnsr withdraw` (initially withdrawn): check the migration of `withdraw`. No unit test written
     yet for `repeatExaCommands` (owed, red first).
- **api-sr-policy announce frame vs the pin**. Cause found by reading the script: `api-sr-policy.run` sends the
  withdrawal right after the announce's ack, before the session is up; ExaBGP holds API routes until
  established and the withdrawal cancels the pending announce, so the pin never sent the announce frame.
  Uncommitted fix: `time.sleep(2)` after each command in `test/exabgp-compat/etc/run/api-sr-policy.run`.
  Not yet run. Next: run it under `--dut exabgp` with the pin (`PYTHONPATH= uv run --no-project --python 3.12
  --with paramiko --with 'exabgp @ git+https://github.com/Exa-Networks/exabgp@685f772e4ede5c818d2ff5c84f4ae040afd37be6'
  ./le --name exa6c test exabgp api --dut exabgp api-sr-policy`) and against ze; if the announce frame
  differs, take ExaBGP's bytes into the `.ci`.
- **api-health `status` design (not built).** No new engine contract is needed for the plugin runner:
  `bridgerun.Dispatcher.DispatchCommand` already returns `(status, json.RawMessage, error)`, and the
  registered `ze-bgp:peer-list` (`internal/component/bgp/plugins/cmd/peer/peer.go`, `handleBgpPeerList`)
  answers address -> {`state`, `name`, `remote-as`, `uptime`}. The runner can dispatch it on `status` and
  write ExaBGP's text/JSON (reactor.py `status`: per peer `name: peer.fsm.name()`, the name being ExaBGP's
  neighbor key and the state upper-case, e.g. `ESTABLISHED`; version/uuid/pid/uptime are the bridge's own).
  Open: the standalone MuxConn runner (`internal/exabgp/bridge`, `pendingResponses`) carries ok/error only,
  so `status` there needs the payload path or is recorded plugin-runner-only; and ze's state names must be
  mapped to ExaBGP's FSM names. Both are design choices for the main thread.

### ExaBGP 6 spec, AC-4 thirteenth package (2026-10-20, Linux): continuation owed

- api-sr-policy: committed ee760dff0a. The pinned ExaBGP refuses an SR Policy announce with neither
  NO_ADVERTISE nor an IPv4 Route Target (RFC 9830 Section 4.2.1), so the fixture now carries
  `community [ no-advertise ]`; the bridge now moves community, large and extended community ahead of
  the `mp` section. Green under `--dut exabgp` and against ze.
- conf-watchdog: the repeat helper ended when stdin closed, while upstream `watchdog.run` loops for the
  parent's life; now only `shutdown` or a failed write ends it (`exaPauseInput`, exabgp_helpers.go).
  Green. 77.77.77.77 at peer up matches the 6.0 fixture; ze still differs from ExaBGP when no script
  ever announces the route (not recorded as a defect). Journal row for the DEBUG-only post-startup
  failure: plan/journal/silent-fall-through.md.
- sr-policy interop (`bgp-exabgp-bridge-sr-policy-gobgp`): still red. Docker exec errors now carry stdout
  when stderr is empty (interoplab docker.go `failureStdout`, docker_stdout_test.go). ze.conf adds
  `extended-community [ target:172.30.0.5:0 ]`. Last run used images built before ee760dff0a. Next:
  rerun `INTEROP_SCENARIO=bgp-exabgp-bridge-sr-policy-gobgp ./le --name exa6c test integration interop`,
  prove red by breaking the bridge hand-off in the image, add the interop.md row, commit.
- api-health `status`: not started. Pinned reactor.py text form: `ExaBGP Daemon Status`, `=====...`,
  `Version:`, `UUID:`, `PID:`, `Uptime: {h}h {m}m {s}s`, `Peers: N`, `  - {key}: {STATE}`, `done`. JSON keys
  version, uuid, pid, uptime, start_time, peers. Check whether the peers key is `neighbor.index()`
  (bytes) before fixing the bytes.

### ExaBGP 6 spec, AC-4 fourteenth package (2026-10-20, Linux): continuation owed

- Committed 64484bfd45: a failed docker exec names stdout when stderr is empty.
- sr-policy interop: the rerun died on a full cache disk (docker run timed out); caches since cleaned.
  Next: `./le test integration cleanup` if anything is left, rerun the scenario, red proof, interop.md row, commit.
- api-health `status`: built, uncommitted. Peer key is ExaBGP `Neighbor.name()` (a str, not index()
  bytes); state is FSMState.name, equal to ze's fsm.State.String(); JSON always under API 6, else only with
  a trailing `json`. Ze exposes the real FSM state through a new `fsm-state` field in `show bgp peer * detail`
  (PeerInfo.FSMState). Files: bridge_status.go/_test.go, bridge_command.go, bridge.go (standalone runner
  refuses `status`: plugin-runner-only, record in the spec), bridge_ack.go (one token), bridgerun/script.go,
  plugin/types_bgp.go, reactor_api.go, cmd/peer peer.go fields.go fsm_state_test.go ze-peer-cmd.yang,
  docs/architecture/exabgp-bridge.md. Bridge Status/Ping tests pass. Owed: peer package run with the
  feature tags (an untagged run showed unrelated reds), bridgerun tests, a fleet test, the api-health case.

### ExaBGP 6 spec, AC-4 fifteenth package (2026-10-20, Linux): continuation owed

- Committed 01fc0f642b: `show bgp peer * detail` carries `fsm-state` (PeerInfo.FSMState).
- Committed 8dec40ea85: the bridge answers `status` / `daemon status` from ze's peer list. This is
  plugin-runner-only: the standalone `ze exabgp plugin` refuses it by name. Recorded in the spec. Proof: cmd/peer,
  bridge and bridgerun are green with every `ze_` gate tag (feature-gates.txt). `TestFleetAnswersStatusFromZesPeerList`
  passes. 6.0 api-health PASSES under the stage's uv line and FAILS (five "status missing UUID" respawns) with the
  bridgerun status answer disabled.
- sr-policy interop `bgp-exabgp-bridge-sr-policy-gobgp` is still uncommitted and NOT proven, for an environmental
  reason. Two runs at load average 50-73 failed at assertion 2: `docker exec ... gobgp global rib -a ipv4-srpolicy`
  did not finish within 30s. The control scenario `bgp-sr-policy-gobgp` failed in the same hour
  (`docker run -d did not finish within 1m0s`, and image removals timed out), so the Docker daemon was too slow to
  judge either one. A third attempt could not build le: another session's uncommitted edit to
  `radiussubscriber/checkers.go` (`lab.capturedRequest` undefined) broke the build. Lab leftovers were removed
  with `./le test integration cleanup` (12 image tags, 1 container).
- Next steps, on a quiet machine with le building:
  1. Rerun `INTEROP_SCENARIO=bgp-exabgp-bridge-sr-policy-gobgp ./le test integration interop` in the background
     (image prep takes 2-4 min), then the control.
  2. Red proof without a tree edit: the lab build inherits os.Environ (gotoolchain Environment), so set
     `GOFLAGS=-overlay=<json>` with an overlay that replaces `internal/component/bgp/plugins/nlri/srpolicy/register.go`
     by a copy setting `NLRISectionCarriesAttributes: false`.
  3. Add interop.md row 49 after row 48, then commit. interop.md holds the RADIUS session's hunk (the
     radius-subscriber section), which cites `replayScript`, not in HEAD. Do not carry the file until that hunk lands.
- The ExaBGP line matches the api-sr-policy fixture except for the community, so ze sends the fixture's bytes (no
  BSID, two segment lists). I read GoBGP v3.31.0's segment-list decoder and apiutil: neither hangs on that shape.

### ExaBGP 6 spec, Phase 2 fourth package (2026-10-20, Linux): continuation owed

- Committed 3511f9ee1b: `exabgp.api.version` / `exabgp.api.ack` read through `env.Get`, so the underscore
  spelling reaches the bridge; `env.ensureCache` now lets a dotted spelling win over an undotted one for
  every key (ExaBGP's own priority: dot > underscore > INI > default).
- AC-10 uncommitted: `option=env:<key>=<value>` in `parseExaBGPCI` (cmd_exabgp.go, test
  `TestParseExaBGPCIEnvOption`), the mock server accepts it (interoplab/bgp/exabgp_case.go), ci-format.md,
  case `api-version-env` (.ci, .conf, .run), parse case `cli-exabgp-migrate-env-api-version.ci` (green, red
  unproven). Owed: split the compound `!found || key == ""` guard; run the case green on ExaBGP and ze and
  red on ze with detection disabled; commit. The spec's `TestMigrateOnExitAndAPIVersion` does not exist:
  migration of `api.version` is covered by `TestEnvSurvivingKey` (migration/env_test.go) and the parse case.
- Populations not run. Pin: `exabgp @ git+https://github.com/Exa-Networks/exabgp@685f772e4ede5c818d2ff5c84f4ae040afd37be6`
  (`internal/le/test/functional/exabgp.go`); one population at a time.

### ExaBGP 6 spec, Phase 2 fifth package (2026-10-20, Linux)

- Committed a42756b10d: AC-10 functional. `option=env:` guard split in two; `api-version-env` green on the
  pin (`--dut exabgp`) and ze, red on ze with `APIVersionFromEnv` overlaid to ignore the setting; parse case
  red with the `api.version` arm of `migration/env.go` overlaid away. Spec fourth-package line added. The
  test-plan row correction (no `TestMigrateOnExitAndAPIVersion`; AC-10 is `TestEnvSurvivingKey`,
  `TestParseExaBGPCIEnvOption`, the parse case) rode along in the other agent's 8dec40ea85.
- Populations at the pin, each run as the stage runs it (`uv run ... bin/le-exa6f/le test exabgp <p> --all`,
  `bin/le-exa6f` on PATH, no `--name`: under `./le --name` the conf-watchdog helper's `le` refuses the
  inherited build name; journal row in `delegation-hands-the-callee-a-different-argv.md`):
  encoding 41/42 (red conf-flow); old-4.2 25 pass, 8 skip, red api-eor; old-5.0 57 pass, 12 skip, red
  api-eor, api-rib, api-rr-rib; api 43/48, red api-disable, api-eor, api-on-exit, api-rib, api-rr-rib
  (api-health passes). Every red's first error matches its recorded cause: api-eor (scripted EOR dropped,
  `claimInitialSyncEOR`), api-rib/api-rr-rib (`clear`/`flush adj-rib out` mapping), api-disable (migration
  refuses `shutdown`, AC-41), api-on-exit (AC-22..24), conf-flow (6.0 `redirect <ip>` is the IETF form; the
  pin refuses `traffic-class 101`). All wait on owner decisions; conf-flow journal row in
  `conflicting-feature-contracts.md`. No bridge defect found, so no fix commit.
- Gates owed by the main thread: `./le go lint run` on `internal/test/cli`, `internal/le/interoplab/bgp`.

### ExaBGP 6 spec, AC-4 sixteenth package: sr-policy interop (2026-10-20, Linux): continuation owed

- The earlier "environmental" verdict was wrong. On a quiet machine (load 2) the CONTROL
  `bgp-sr-policy-gobgp` fails too: it was committed unrun (65ecdbf014, "NOT RUN"). Two real causes:
  1. No gobgp CLI release names an SR Policy family after `-a` (`checkAddressFamily` in cmd/gobgp/common.go,
     read at v3.31.0, v3.37.0, v4.4.0, v4.10.0), so `gobgp global rib -a ipv4-srpolicy` answers
     "unsupported address family". Fix in the tree, UNCOMMITTED: `test/interop/Dockerfile.gobgp` builds
     `gobgp-paths <afi> <safi>`, a gRPC ListPath reader (heredoc, GoBGP v3.31.0) printing NLRI + each
     attribute's String(); `names.go` drops `gobgpFamilyIPv[46]SRPolicy` for `cmdGoBGPPaths`,
     `gobgpAFIIPv4/6`, `gobgpSAFISRPolicy`; `check_extras.go` (control and malformed-receive) and the new
     `register_exabgp_bridge_sr_policy.go` use it; tomls/ze.conf comments and interop.md rows 45 and 49
     (49 is new) say so. The reader skips MP_REACH: GoBGP v3.31.0's apiutil panics converting an SR Policy
     MP_REACH back from API form (`SRPolicyNLRI.Serialize`, "slice bounds out of range [:5] with
     capacity 2"), reproduced offline. `go vet` of the reader clean; interoplab/bgp unit tests pass.
  2. gobgpd itself appears to hang once it holds ze's SR Policy: in both scenarios `docker exec` of
     the CLI and of `gobgp-paths` (own 10s deadline) ran past 30s, the gobgp log section came back empty,
     and load rose to ~72 during the run and fell after. Offline, GoBGP v3.31.0 `ParseBGPMessage` decodes
     the api-sr-policy fixture UPDATE cleanly and prints every needle (Preference 300, labels 16001/16002).
     So the hang is server side, after parse (RIB insert, best path, or ListPath's ToApi). NOT yet observed
     directly. Next: run the scenario and, while assertion 2 waits, `docker top`/`docker stats` the
     `ze-iop-gobgp-*` container and `docker logs` it to a scratch file; if gobgpd spins, find the loop
     (candidate: the server's own MP_REACH round trip through apiutil, the panic above, if a recover
     loops on it) and either work around it in the reader or judge with gobgpd at `-l debug` (it logs
     received UPDATEs through String()).
- How to run: `bin/le` does NOT pick up uncommitted le sources; use `./le --name srp ...`. Another
  session's untracked `internal/test/fixture/register_stop_boot_record.go` (undefined
  `stopDuringBootRecord`) broke the le-linux cross-compile at 18:43; I hid it with
  `GOFLAGS=-overlay=<parent scratch>/srp/hide.json` (never edited it). The red overlay (hide + srpolicy
  register.go `NLRISectionCarriesAttributes: false`) is `<parent scratch>/srp/red.json`. The lab build
  inherits GOFLAGS through gotoolchain `Environment` (os.Environ); the `./le --name` self-build too.
- interop.md: the RADIUS hunk is in HEAD now (`replayScript` in radiussubscriber/checkers.go), so the file
  carries only this session's rows. Nothing of this package is committed; owed: green, red (red.json),
  commit of Dockerfile.gobgp + names.go + check_extras.go + the scenario + interop.md, with the control
  run green too (it shares the reader).
- RESOLVED 2026-10-20 (continuation): gobgpd never hung. Captured mid-wait: gobgpd 0.04% CPU, 12.7 MiB,
  session Established with 3 received, `gobgp-paths` answering in 0.1s. The control failed on the reader:
  GoBGP v3.31.0 apiutil turns the raw 4-octet Binding SID into a label a second time (`NewBSID` shifts 12
  bits), so BSID 1048575 printed 1044480, and the NLRI Length comes back in bits ("Length: 1"). The reader
  now asks ListPath for `EnableOnlyBinary` and decodes with GoBGP's packet decoder (journal row in
  `plan/journal/field-carries-two-meanings.md`). Green: `bgp-sr-policy-gobgp`,
  `bgp-exabgp-bridge-sr-policy-gobgp`, `bgp-sr-policy-malformed-receive-gobgp`. Red: the bridge scenario
  with the srpolicy registration's `NLRISectionCarriesAttributes: false` fails assertion 2 (GoBGP holds
  nothing; ze logs `sr-policy takes only distinguisher, color, endpoint: "preference" is not one of them`),
  so row 49's red claim stands. The hide overlay is no longer needed (the fixture landed in 7d666660d4).
  Committed in 65651c27e0. `go vet` and `go test` of internal/le/interoplab/bgp green under `./le job run`.
  Owed by the main thread: `./le go lint run` (not run here: a subagent must not hold a >5 min command).

### ExaBGP 6 spec, Phase 3 (2026-10-20, Linux, exa7): done

- Committed 1e30e6aa8d: X-13, X-14, AC-14 to AC-16. Details: the spec's "Phase 3 state" line.
- api population on ze at the pin: 45/50, reds api-disable, api-eor, api-on-exit, api-rib, api-rr-rib
  (unchanged known reds, each waiting on its owner decision or later phase).
- Owed by the main thread: `./le go lint run` on `internal/exabgp/bridge`, `internal/plugins/exabgp/bridgerun`;
  encoding, old-4.2, old-5.0 populations (a translator-refused line now answers `error: <why>` then
  `error`, where it was silent; the old corpora are where a script might count answers).
- Next: Phase 4 (X-15 to X-22).

### ExaBGP 6 spec, Phase 4 first package (2026-10-20, Linux, exa8): continuation owed

- Populations before any edit (pin 685f772e4, `bin/le-exa8` on PATH as the stage runs it): encoding
  41/42 (conf-flow; conf-watchdog is red only without `bin/le-exa8` on PATH, the journaled helper-`le`
  cause); old-4.2 25 pass 8 skip, red api-eor; old-5.0 57 pass 12 skip, red api-eor, api-rib,
  api-rr-rib (a first run also failed api-attributes-path and unknown-message on EOR timing; both
  green alone). No Phase 3 regression.
- e9ac5ea736: X-15 (AC-17). The on-exit commit (X-21, X-22, AC-22, AC-24, AC-25) carries this entry;
  details in the spec's "Phase 4 state" line. `api-on-exit` green on the pin and on ze.
- Next agent: X-16 to X-20 group (AC-18 to AC-21: a `group start/end` buffer per script, the inline
  `group a ; b` split by ExaBGP's lexer rule, `attributes` sharing, answers per `command/group.py`;
  cases `api-group`, `api-group-inline`); the `api-on-exit-v4-keep` case (AC-23); A-8; the api
  population rerun.
- Red overlays: `<parent scratch>/p4/red/v6.json` (v4ControlSpelling identity), `red/exit.json`
  (`script.exited` returns at once). Gates owed by the main thread: `./le go lint run` on
  internal/exabgp and internal/plugins/exabgp; `./le test unit all`; the api population.

### ExaBGP 6 spec, Phase 4 second package (2026-10-20, Linux, exa9): continuation owed

- 636de46430: X-16 to X-20 (AC-18 to AC-21). `bridge.Group` (`internal/exabgp/bridge/bridge_group.go`)
  and `bridge.ScriptLines.Batch` (`bridge_lines.go`), now the ONE batch path: `script.batch` calls it
  through `scriptRunner`, `Bridge.dispatchBatch` through `muxRunner`. A closed group's route commands
  differing only in their one-word NLRI are dispatched as one command, so one UPDATE. Cases
  `api-group`, `api-group-inline` green on the pin and ze; `api-group` red with `Group.read` overlaid
  to pass (`<parent scratch>/p4/red/group.json`) and with the merge off (`red/merge.json`).
- df5f1872f5: AC-23 functional, case `api-on-exit-v4-keep` (one-shot API 4 script plus a marker
  sibling). Green on the pin and ze; red with `exitPolicy` forced to withdraw (`red/keep.json`).
- Left, in order: A-8 `TestExitWithdrawRestoresConfiguredRoute` (engine level: does ze re-announce a
  configured static route of the same prefix after the exit withdrawal; not investigated); the
  standalone runner's exit policy: `muxRunner.Dispatched` records nothing, and `ownerLedger`,
  `exitPolicy`, `OnExit` live in `bridgerun`, so sharing them means moving them into
  `internal/exabgp/bridge` and having `Bridge` run the exit settlement where its child exits (it has
  no process block, so `on-exit` is unset there and the version decides); the api population rerun
  with the count.
- Gates owed by the main thread: `./le go lint run` on internal/exabgp and internal/plugins/exabgp
  (the edit-time lint was clean on the new files); `./le test unit all`; the api population. `./le doc
  wiring` is red on files outside this work (hub-architecture.md and a doc-drift list).

### ExaBGP 6 spec, Phase 4 third package (2026-10-20, Linux, exa10): continuation owed

- 41fd5791d8: A-8 was broken. New case `api-on-exit-configured` (static 1.1.1.1/32 nh 10.10.10.10;
  an API 6 script announces it with nh 11.11.11.11 and exits) green on the pin (`--dut exabgp`),
  red on ze before the fix (the mock received the withdrawal). Fix: update-text keyword
  `restore-configured` (before the first section) -> `NLRIGroup`/`NLRIBatch.RestoreConfigured` ->
  `WithdrawNLRIBatch` asks `Peer.restoreConfigured` (`reactor/peer_static_restore.go`), which drops
  each withdrawn NLRI the peer's static config holds and re-sends the configured route via
  `sendStaticRoutes`. The bridge ledger stores each exit withdrawal with the keyword
  (`restoringWithdrawal`, `bridgerun/owner.go`). After it: api-on-exit, api-on-exit-v4-keep,
  api-on-exit-configured green on ze. Docs: update-syntax.md, exabgp-bridge.md.
- Left, in order: (1) A-8 for static routes with an RD or a label stack: `staticRouteNamesNLRI`
  matches prefix routes only, so a VPN or labeled configured route is still withdrawn on exit;
  matching needs the label stack (or the RFC 8277 compatibility field) and RD read from the NLRI
  wire. (2) A parser test for `restore-configured` (accepted before a section, refused after it with
  `errRestoreConfiguredAfterNLRI`) in `cmd/update`; none written. (3) The standalone runner's exit
  policy: move `ownerLedger`, `exitPolicy`, `OnExit`, `restoringWithdrawal` from bridgerun into
  `internal/exabgp/bridge` (delete the bridgerun copies), make `muxRunner.Dispatched` record, settle
  the exit where the standalone `Bridge`'s child exits (no process block: the version decides);
  unit test red first. (4) The api population rerun once with `bin/le-exa10` on PATH; report count
  and every red. (5) Spec Phase 4 state line.
- Gates owed by the main thread: `./le go lint run` on internal/component/bgp/reactor,
  internal/component/bgp/plugins/cmd/update, internal/plugins/exabgp; `./le test unit all` (the
  reactor change touches `WithdrawNLRIBatch`'s queue path: it now ranges `peerBatch.NLRIs`, which
  equals `batch.NLRIs` unless restore-configured removed some); `./le test functional exabgp-test`;
  `./le rfc check`. Untouched foreign files: `internal/plugins/exabgp/bridgerun/version.go`
  (untracked, one line), `rfc/discrimination/.rfc1997.json.tmp-*`.

### ExaBGP 6 spec, Phase 4 fourth package (2026-10-20, Linux, exa10): Phase 4 rows met

- d54ccff4ad: parser tests for `restore-configured` (before a section marks every group; after
  one refused, red with the refusal overlaid away). The refusal text named the refused
  `nlri <family>` form; it now names `nlri del <prefix>` and `mp <family> del <prefix>`.
- 159d3765a2: A-8 for routes with an RD or a label stack (`staticRouteNamesNLRI` compares route
  keys, `nlrisplit.GetPrefixKey` as a withdrawal vs `staticRouteKey`; RFC 4364 4.1 and RFC 8277
  2.4 quoted). Test red against the old matcher; whole reactor package green (`-tags ze_bgp`).
- 8f3048d439: the owner ledger and exit policy live once in `internal/exabgp/bridge/bridge_owner.go`
  (`OwnerLedger.Exited`), bridgerun's copy deleted; the standalone `Bridge` records ownership and
  settles its child's exit in `Run`. `TestStandaloneExitWithdrawsOwnedRoutes` red first.
  `test/weakened/9586869b.md` records the three tests that moved packages.
- api population at the pin, once, `bin/le-exa10` rebuilt: 51/55. Reds: api-disable (AC-41 owner
  question, migration refuses `shutdown`), api-eor (EOR at sequence 3, `claimInitialSyncEOR`),
  api-rib, api-rr-rib (`clear`/`flush adj-rib out`): the known four, first errors match causes.
- Phase 4 rows X-15..X-22, AC-17..AC-25 met. Still open, in no AC: the ledger keys on the selector
  text where ExaBGP's owner is per peer.
- Gates owed by the main thread: `./le go lint run` on internal/exabgp/bridge,
  internal/plugins/exabgp, internal/component/bgp/reactor, internal/component/bgp/plugins/cmd/update;
  `./le test unit all`; `./le test functional exabgp-test`; `./le rfc check`.

### ExaBGP 6 spec, Phase 5 first package (exa11, 2026-10-20): continuation owed

- Audit at ba4388fe47: AC-28 met (af18a709d8, 9dea34f1b9, 76b51c8946, interop 65651c27e0); A-9 adopted.
  Not re-verified: AC-28 "a word after the sub-TLVs is refused". AC-26 (rtc) and AC-27 (mpls-vpn) absent.
- Built, uncommitted at hand-off: RTC encoder (`nlri/rtc/encode.go`, RFC 4684 Section 4), update-text
  rtc section fix (`endsRegistrySection` ended it at `origin-as`), bridge rows `mpls-vpn` and `rtc`,
  cases `api-rtc` (green on pin and ze) and `api-mpls-vpn`, docs incl. `rfc/short/rfc4684.md`.
- `api-mpls-vpn` pins ze's NEXT_HOP beside MP_REACH (`family.LegacyNextHop`, as `api-vpnv4`); the pin
  sends none (RFC 4760 Section 3 "SHOULD NOT carry the NEXT_HOP"), so it is red under `--dut exabgp`.
  Owner decision owed.
- Next: commit the rest; AC-28 refusal; interop `bgp-exabgp-bridge-rtc-gobgp`,
  `bgp-exabgp-bridge-vpnv4-frr` (proven red); spec "Phase 5 state" line.

### ExaBGP 6 spec, Phase 5 second package (exa12, 2026-10-20): Phase 5 rows met

- AC-28 refusal re-verified, no code change: `TestAPISRPolicyAttributesReachParseConfigRoute` green,
  red with srpolicy `parseConfigRoute`'s default-arm refusal overlaid to skip the word.
- 7cec4fa82c: interop `bgp-exabgp-bridge-rtc-gobgp` (row 50) and `bgp-exabgp-bridge-vpnv4-frr`
  (row 51), green, each red at assertion 2 with its `bridgeSAFI` row overlaid away (ze logs the
  refusal; `GOFLAGS=-overlay=<json>` with `./le --name <n> test integration interop` reaches the lab
  build; plain `./le` runs a cached bin/le that lacks new scenario registrations).
- GoBGP v3.31.0 panics (nil Route Target, `BgpServer.propagateUpdate`) on a 0-bit default RTC
  membership after another membership; the rtc scenario sends only `65001:100`. Not a ze defect.
- NEXT_HOP question: FRR accepts ze's UPDATE with NEXT_HOP beside MP_REACH, installs the route
  with its RD and label, session stays Established. Which next hop FRR uses is not observable here:
  the bridge writes the same address into both. `api-mpls-vpn` files stay uncommitted pending owner.
- `docs/architecture/wire/nlri.md`: the "SR-Policy in `update text`" section was false since
  af18a709d8 (said `add` went to `EncodeNLRIHex` refusing all but three words); rewritten to the
  producer, and an "RTC in `update text`" section added (the spec hook required naming the page).
  `./le doc check verify` red only on a foreign anchor (hub-architecture.md `caseInEffect`).
- Spec "Phase 5 state" line added. Phase 5 rows AC-26..AC-28 met. Gates owed by the main thread: `./le go lint run` on
  internal/le/interoplab/bgp; `./le test unit all` (`internal/le/interoplab/pppoe`
  `TestPPPoELabPeersRunWithoutPrivilege` is red on a foreign scenario `ipv6cp-missing-option`).

### Owner decisions, 2026-10-20 (Linux to Mac move)

- api-eor: ExaBGP at the pin (`announce_eor`, reactor/api/command/announce.py) sends an EOR every time a
  script asks, to each established peer that negotiated that family (RFC 4760 Section 8), and answers
  `error` when no peer qualifies. Owner asked what ExaBGP does; proposal: do the same instead of
  `claimInitialSyncEOR` dropping repeats. Confirm with the owner before coding.
- api-rib, api-rr-rib: owner: "do the same on adj-rib-out". At the pin `clear adj-rib out` withdraws every
  route in the peer's adj-rib-out (`neighbor_rib_out_withdraw`) and `flush adj-rib out` re-sends it
  (`neighbor_rib_resend`), then answer `done`; no matching peer answers `error`. Map both onto ze's own
  adj-rib-out.
- conf-flow: follow the 6.0 flowspec redirect form.
- RFC8955-4-3: owner approved fixing the violation, i.e. updating the RFC-tagged `test/encode/flow-encode.ci`
  to the redirect next hop ze now sends, then `./le rfc discriminate-record` it.
- mpls-vpn NEXT_HOP: follow the RFC (RFC 4760 Section 3 "SHOULD NOT carry the NEXT_HOP"): drop the NEXT_HOP
  beside MP_REACH for VPN (`family.LegacyNextHop`), update `api-vpnv4`/conf-* fixtures. The uncommitted
  `api-mpls-vpn` case files on the Linux VM pin the old bytes; recreate them on the Mac from the pin.
- `restore-configured`: owner asked what it is; still public update-text keyword pending the answer.
- Next ExaBGP phases: Phase 6 (in flight at the move, see its own entry), Phase 7 (incl. AC-41 api-disable),
  Phase 8, then review and close.

### ExaBGP 6 spec, Phase 6 first package (2026-10-20, Linux, stopped for the Mac move)

Committed with this entry (bridge code and one doc fix; no new unit test, no `.ci`, no red run yet):
- X-24 (AC-30): `Translation.Unmatched` deleted. `Translator.Line` refuses a selector left with no
  peer (`errSelectorMatchesNoPeer`): every address excluded by `family-allowed in-open`, or every
  address outside `Translator.Peers` (new `Translator.servedSelector`; `*` and an empty
  `Translator.Peers` pass whole, and ze answers `route.ErrNoPeersMatch` for an unknown address).
  The `bridge_control_test.go` unmatched cases now expect the refusal (spec: X-24 reverses the old
  `done`). `docs/exabgp/exabgp-differences.md` corrected (it said the line was answered `done`).
- X-26/X-27 keep-first (AC-32): `parseRouteAttributes` keeps the first of a repeated attribute
  (keyed on the ze keyword) and records `routeAttributes.Warnings`; `warnRouteLine` logs them
  (`slog.Warn "exabgp-bridge route line"`) from `buildRouteCommand` and `convertAttributesForm`.
- X-27 short split (AC-33): `splitPrefix` returns `(pieces, whole, err)`; a split shorter than the
  prefix sends the prefix itself and adds a warning. A split longer than the address is refused.
- X-28 (AC-34): a value-less attribute is refused as `... path-information is written with no
  value; write path-information <path identifier, a number or an IPv4 address>`.
- X-29 (AC-35): `defaultPrefixLengths` now returns an error; `refuseMaskNotNumber` refuses
  `10.0.0.0/x` (`errMaskNotNumber`).
- X-30 (AC-36): `bridge_operational.go` `convertOperational`: unknown kind gives `unknown operational
  message: <command>`; a known kind gives `the OPERATIONAL message is not sent by Ze (<kind>)` (OD-1).
- Evidence: `go vet ./internal/exabgp/bridge` clean; `go test -count=1 ./internal/exabgp/bridge/`
  green (job p6bridge). bridgerun not re-run (it never read `Unmatched`).

NOT done, next on the Mac:
1. Unit tests from the spec TDD plan, each red first with a Go overlay: `TestSelectorMatchingNoPeerRefused`
   (bridge_batch_test.go, with Translator.Peers narrowing), `TestRepeatedAttributeKeepsFirstAndWarns`,
   `TestSplitShorterThanPrefixSendsPrefix`, `TestPathInformationWithoutValueRefused` (settles A-6),
   `TestPrefixMaskNotNumberRefused`, `TestOperationalRefusedByKind`.
2. X-23 (AC-29) NOT started. Finding: ze's update-text handler answers status `warning`, not
   error, when no selected peer has the family negotiated (`route.ErrNoPeersAcceptedFamily`,
   `update_text.go` near lines 1256 and 1295), and `bridgerun` `scriptRunner.Dispatch` drops the
   status, so the bridge answers `done`. Fix: a `warning` with no route sent is a refusal carrying
   its message, in both runners (check what the standalone MuxConn answer carries for a warning).
   ze tests the NEGOTIATED family, ExaBGP the CONFIGURED one (`carrying`, announce.py): record it.
   EOR (ExaBGP errors when no established peer carries the family) unchecked. Test
   `TestFamilyNoSelectedPeerCarriesAnswersError` against the real engine (A-4).
3. X-25 (AC-31): already refused (`errFlowRouteNoMatch`); needs a case only.
4. `.ci` cases, green on `--dut exabgp` and ze, red on ze with an overlay: one refusal case (selector
   no peer, flow no match, path-information, mask, operational twice; accept `a[-1] == 'error'`,
   because the pin writes the bare marker for NoMatchingPeers) and one keep-first case (NEXT_HOP
   1.1.1.1 on the wire for `next-hop 1.1.1.1 next-hop 2.2.2.2`; the /24 whole for `split /23`).
   Template: `test/exabgp-compat/api/api-error-answer-v4.*`.
5. Docs: keep-first, short split, mask, operational (OD-1) in `exabgp-bridge.md` and
   `exabgp-differences.md`; spec "Phase 6 state" line; api population rerun.
- Gates owed: `./le go lint run` on internal/exabgp/bridge; `./le test unit all`;
  `./le test functional exabgp-test`.
- Untouched: `internal/plugins/exabgp/bridgerun/version.go`, `rfc/discrimination/.rfc1997.json.tmp-*`,
  the owner-pending `api-mpls-vpn` case files.

### ExaBGP 6 adj-rib-out and EOR (2026-10-20, Mac)

Agent stopped at its 100-call budget. Nothing committed but this entry. Baseline on ze
(`./le --name exarib test exabgp api api-rib api-rr-rib api-eor`): all three FAIL.

**api-eor (item 2): code half-done, UNCOMMITTED in the tree. Do not commit it as it stands.**
- Guard named: `Peer.claimInitialSyncEOR` IS a guard. It stops the route server's
  (`rs/server_handlers.go` ~L557) and the route reflector's (`rr/rr.go` L656, `rrEORCmd`)
  replay-closing `update text ... eor` from repeating the marker the reactor's own initial
  sync sent (RFC 4724 Section 2). `AnnounceEOR` has one caller, `update_text.go` L1156, so a
  script's EOR and an RS/RR replay EOR cannot be told apart today.
- Design: `AnnounceEOR` sends every time (ExaBGP `announce_eor`), skips a peer that did not
  negotiate the family (RFC 4760 Section 8 quoted), and answers `eorNoPeerError` when none
  qualifies. The new `AnnounceReplayEOR` keeps the claim gate. Both live in
  `reactor/reactor_api_forward.go` (`announceEOR(sel, fam, sender, once)`); the interface is in
  `bgp/types/reactor.go`; `AnnounceReplayEOR` is added to 7 test mocks. These edits ARE in the
  tree (`go vet` clean). An explicit EOR still claims the family when unclaimed, and releases
  only a claim it took.
- Owed: (1) `update_text.go` handleUpdateText: pick `AnnounceReplayEOR` when
  `ctx.Meta["replay-eor"] == true` (helper `replayEORFromMeta` beside `replayFromMeta`). The
  edit was refused only because `ai/doors/bgp-plugins.md` had not been read. (2) RS and RR send
  their replay EOR with `UpdateRouteWithMeta(..., map[string]any{"replay-eor": true})` (RS has
  an `updateRouteHook` that tests use). Until (1) and (2) land, the AnnounceEOR change in the
  tree lets an RS/RR replay repeat the EOR, which is a regression. (3) Bridge: a bare
  `announce eor` is IPv4 unicast only (pin `api_eor`, `reactor/api/__init__.py` L307: no
  tokens gives `Family(ipv4, unicast)`). `convertEOR` in `bridge_route_forms.go` expands it to
  every declared family, which breaks api-eor sequence 3. (4) Tests, each red first with an
  overlay: a reactor test that two explicit EORs both reach the wire, that a replay EOR after
  the initial-sync one does not, and that a non-negotiated family answers the error. Review
  `peer_initial_sync_test.go` L406 and `forward_replay_fence_test.go` L242, which call
  AnnounceEOR in a sync window (the shouldQueue / initialSyncEOROwed drop is kept). (5) Docs:
  `docs/architecture/api/update-syntax.md` and `commands.md` (EOR semantics, the `replay-eor`
  meta), `exabgp-bridge.md`, `exabgp-differences.md`. (6) api-eor under api, old-4.2 and
  old-5.0, on ze and on the pin.

**api-rib / api-rr-rib (item 1): investigated, not started. Findings for the design:**
- ze has three outbound stores. The reactor's `adjRIBOut` (`reactor/adj_rib_out.go`) holds
  every route on the session as keys plus local signatures and NO attributes, so it can
  withdraw but cannot resend. bgp-rib's `ribOut` is fed by sent events and is the source of
  the `clear bgp rib out` replay (`outboundResend`). Config routes sit in
  `Peer.staticWire.routes` and are NOT in ribOut (`peer_static_wire.go` L255,
  `sendingConfigStatic`).
- Withdraw-all, engine-side: for each matching peer, snapshot `adjOut.routes` under
  `adjOut.mu`. The wire NLRI is `exactNLRI` when set, else `key.path.nlri[:length]` or
  `overflow`; the path id is `key.path.received` when `key.addPath`. Send through the local
  (not source-bound) API withdraw path (`withdrawBatchFromPeers`, `nlri.NewWireNLRI`), so
  `adjOutWrite.allow` removes the key and the sent event lets bgp-rib drop its ribOut entries.
  A later flush must not resend them, and the case checks that. Empty `staticWire.routes` too.
  Open gap: a forwarded labeled route stores no exactNLRI, so its key lacks the label field.
- api-rib step 10 sends `announce .0` then `clear adj-rib out` in one bridge batch and expects
  NOTHING on the wire for .0, because ExaBGP coalesces in its RIB. ze flushes the forward pool
  once per batch (`bridge_lines.go` L179), so a clear has to drop the same batch's earlier
  route commands for its selector before dispatch. Otherwise .0 goes out, which is today's
  failure.
- Flush has to emit BoRR/EoRR when enhanced route refresh is negotiated: api-rr-rib expects
  `05:00010101` ... `05:00010201` around the resend. bgp-rib sends them only for a received
  refresh (`handleRefreshStructured`, `dispatchPeerAction("borr ...")`). `outboundResend`
  sends no markers, and config routes are not in its replay.
- CLI: the verb table says `clear` means the data becomes empty, which fits withdraw-all (the
  existing `clear bgp rib out` replay is the misfit). Candidate: a sibling YANG container,
  `clear bgp rib advertised <selector> [family <f>]`, with a mandatory selector and `*` as the
  explicit all form (cli-grammar `acting-on-everything-needs-an-explicit-all`). Because the
  store is the engine's, the handler belongs with the reactor's peer commands, not in bgp-rib.
  Decide this before coding.
- The pin source was fetched to `tmp/session/2026-10-20-8385ff13-.../scratch/exa-src`, which
  is local to this machine.
- Foreign red seen: `internal/exabgp/bridge/bridge.go` has undefined `documents`,
  `answerDocuments` and `RefuseUnsentRoute`, from another session's in-flight bridge work. It
  stops `plugin/all` from linking, so reactor tests that import it will not build until that
  work lands.
- Gates owed: scoped `./le go lint run` on reactor, types, cmd/update, rs, rr and bridge;
  `go test -tags ze_bgp` on those packages; the exabgp api population.

### ExaBGP 6 spec, Phase 6 second package (2026-10-20, Mac)

- Committed: a6e0ac212f (bridgerun test helper counted words, not lines; Mac red 3/3), 05aeb4fd75 (six
  Phase 6 unit tests, each red under an overlay), 19013e6857 (X-23/AC-29: `bridge.RefuseUnsentRoute`
  turns an update-text `warning` or a message with no counts into a refusal, both runners; cases
  `api-refusals-v4` and `api-keep-first-v4` green on the pin and ze, red on ze by overlay; docs and the
  spec's Phase 6 state line). A-4 was wrong (fixed), A-5 and A-6 validated. X-25 covered by the refusal case.
- Shared-file change: the standalone runner releases a dispatch on the terminator, holding the first
  record (`answerDocuments`, bridge.go; `pendingResult.document`).
- Journal row (warning reaches socket callers as done) in `plan/journal/silent-fall-through.md`.
- Left: AC-29's EOR half (with the adj-rib-out/EOR package); scoped lint on internal/exabgp/bridge and
  internal/plugins/exabgp ("inputs changed during planning" twice); the api population rerun (blocked by a
  foreign build break in rs/server_handlers.go); `./le test unit all`; `./le test functional exabgp-test`.

### ExaBGP 6 adj-rib-out and EOR, continuation (2026-10-20, Mac)

Agent stopped near its 100-call budget. **api-eor is DONE and committed: `b0b734ec2e`.** The
regression the previous entry warned about is gone (rs/rr replay EORs carry the `replay-eor` meta).

- What landed: `AnnounceEOR` sends each time (RFC 4760 Section 8 skip, `eorNoPeerError`);
  `AnnounceReplayEOR` keeps the claim gate; `update_text.go` picks it on `bgptypes.ReplayEORMeta`
  (`replayEORFromMeta`); rs (`updateRouteWithMeta`, the unused `updateRoute` deleted) and rr
  (`updateRouteWithMeta`) send their replay EOR with the meta; bridge `convertEOR` reads a bare
  `announce eor` as IPv4 unicast only and refuses a token count other than 0 or 2. Docs:
  update-syntax.md ("Who gets the marker"), commands.md (translation row), exabgp-bridge.md.
  exabgp-differences.md needed nothing: the bridge now matches ExaBGP.
- Evidence: red by `-overlay` over HEAD for the reactor, update, rs, rr tests; bridge test red on
  the unfixed tree; replay-gate test red with the gate disabled by overlay. Green after.
  `api-eor` PASS on ze under api, old-4.2, old-5.0, and PASS on the pin.
- Foreign reds seen in `go test -tags ze_bgp ./internal/component/bgp/reactor/`:
  TestQueuedSelfResolvesReplacementConnection, TestBatchSelfDoesNotCrossReplacementSession,
  TestStaticSelfStaysOnCapturedSession, TestWireFormat_UnicastIPv4. They fail identically with
  this change overlaid back to HEAD (the VPN legacy-next-hop work in reactor_api_batch.go,
  wire_compat_test.go is uncommitted in the tree). Not this package's.
- Owed cleanup (small): `bridge.Translator.Families` and `bridgerun.New`'s `families` parameter
  have no reader now. Remove the field, its writers (`bridge.go` ScriptLines literal, `fleet.go`),
  the parameter and its 3 callers (`bridgeplugin/internal.go` x2, `exabgp/main_sdk.go`), and the
  test literals (`bridge_batch_test.go` L52/L429, `bridge_eor_test.go`). Comments already say so.
- Gates owed for b0b734ec2e: `./le verify worktree` (two debt rows in
  `plan/verification-debt/0c501db4.md`). Scoped lint showed no finding on lines this commit
  touched once `rs.updateRoute` was removed; pre-existing goconst/nilerr/godot findings in the
  bridge and update_text.go remain.

**api-rib / api-rr-rib (item 1): NOT STARTED.** The main thread's design decisions (2026-10-20)
stand and are the brief for the next agent:
- Command `clear bgp rib advertised <selector> [family <f>]`, sibling of `clear bgp rib out`,
  selector mandatory, every-peer spelled `all` (cli rule point
  `acting-on-everything-needs-an-explicit-all`: "an explicit selector word, `all`, or the typed
  wildcard the family already uses"); bare prefix refused naming the forms; `RequiresSelector`
  plus a mandatory YANG leaf. Handler registered with the reactor peer commands, no central switch.
- Engine side: snapshot `adjOut.routes`, rebuild wire NLRI (`exactNLRI`, else
  `key.path.nlri[:length]`/`overflow`, path id when `key.addPath`), withdraw through the local API
  withdraw path (`withdrawBatchFromPeers`), so bgp-rib drops ribOut entries via sent events; empty
  `staticWire.routes`.
- Fix the labeled-route gap: a forwarded labeled route stores no `exactNLRI`, so the key lacks the
  label; the withdrawal must carry the RFC 8277 Section 2.4 / RFC 3107 withdrawal label (quote
  from rfc/full).
- Flush (`flush adj-rib out`) sends BoRR/EoRR when enhanced route refresh is negotiated (RFC 7313,
  quote it) and resends config routes too.
- Bridge: drop earlier same-batch route commands for the clear's selector before dispatch; map
  `clear adj-rib out` to the new withdraw and `flush adj-rib out` to the resend, both runners;
  no matching peer answers `error`.
- Cases api-rib and api-rr-rib green on ze and the pin; docs update-syntax.md, commands.md,
  exabgp-bridge.md, exabgp-differences.md in the same commit.

### ExaBGP 6 owner decisions A-C (2026-10-20, Mac)

- A (VPN NEXT_HOP, RFC 4760 Section 3): done in the tree; commit prepared as
  `tmp/commit-485edca9-b-e1caa8.sh` but not run (the auto-mode classifier refused running it).
  `family.LegacyNextHop` drops SAFI VPN; `BuildVPN` writes no NEXT_HOP. Fixtures: test/encode vpn,
  addpath, addpath-vpn-pathid-zero; 6.0 api-vpnv4, api-mpls-vpn (rebuilt from the pin), api-attributes-vpn,
  conf-vpn, conf-addpath; observed allowances on old-4.2 and old-5.0. The five 6.0 cases pass on the pin and
  ze, red with HEAD producers overlaid. Owed: interop bgp-exabgp-bridge-vpnv4-frr; plugin
  vpn-withdrawal-inventory-* and encode rfc4659 (need the Mac loopback aliases, `./le setup install`).
- MUP, MCAST-VPN and labeled unicast still add NEXT_HOP beside MP_REACH: under M2 they follow RFC 4760
  Section 3 too (journal row in conflicting-feature-contracts.md); 6.0 conf-srv6-mup-v3 red on the pin.
- C (RFC8955-4-3): `test/encode/flow-encode.ci` seqs 2, 8, 9 carry the redirect next hop, claim rewritten
  as the owner-approved deviation. The discrimination record did NOT run: the Mac lacks loopback aliases
  127.0.0.2-6 and fd00::2-3. Retake after `./le setup install`, then commit with
  `rfc/discrimination/rfc8955.json`.
- B (conf-flow redirect) not started. At the pin, `redirect <ip>` is the IETF extended community with no
  next hop (IPv6 in attribute 25); `redirect-simpson <ip>` is next hop plus 0x0800; `copy <ip>` IETF,
  `copy-simpson` old; bare `redirect-to-nexthop` simpson, with an address IETF. Change the `case "redirect"`
  arm of `internal/exabgp/migration/migrate_routes.go`; old-4.2/old-5.0 conf-flow then need allowances.
  The pin refuses `traffic-class 101`, so 6.0 conf-flow needs that fixture line checked first.

### ExaBGP 6 adj-rib-out clear/flush, third agent (2026-10-21, Mac): engine command written, NOT committed

Stopped at the call budget. Nothing committed but this entry. The engine half is in the tree,
`go vet -tags ze_bgp` clean over reactor, types, cmd/..., route_refresh.

- In the tree (mine): `reactor/reactor_api_advertised.go` (`WithdrawAdvertised`, `withdrawPeerAdvertised`,
  `adjRIBOut.withdrawalBatches`, `adjOutWithdrawalNLRI`: RFC 8277 Section 2.4 Compatibility rebuild, plus a
  re-key round trip that refuses a family whose key cannot be written back, e.g. forwarded FlowSpec) and its
  `_test.go`; `cmd/peer/advertised.go` (`handleRibClearAdvertised`, wire `ze-bgp:rib-clear-advertised`,
  registered in `peer.go`) and `advertised_test.go`; `WithdrawAdvertised` on `bgptypes.BGPReactor` with stubs
  in 7 mocks (the peer mock records calls); YANG `clear bgp rib advertised <selector> [family <f>]` in
  `ze-peer-cmd.yang`, rpc `rib-clear-advertised` in `ze-bgp-api.yang`, both with revisions;
  `docs/architecture/api/commands.md` line and paragraph.
- Deviation from the brief: every-peer is `*`, not `all`, to match the sibling `clear bgp rib out` and the
  peer-selector type (the cli point allows "the typed wildcard the family already uses"). Main thread to confirm.
- Evidence: `TestAdjOutWithdrawalNLRILabeled/Unicast/Batches` and `TestRibClearAdvertisedSelector/Refusals`
  green; red under `-overlay` (labeled branch disabled: the labeled test fails; the ErrNoPeersMatch branch
  disabled: the refusal test fails). The reactor overlay had to blank the foreign untracked
  `plugin_route_addpath_test.go` (redeclares `mustHex`, unknown field `PathID`).
- COMMIT BLOCKER: `types/reactor.go` and the 6 mock files also carry another session's `SelectedPeersCarry`
  hunk, implemented in the untracked `reactor/reactor_api_carry.go`, which HEAD lacks. Carrying those files
  alone breaks HEAD. Commit after that session lands its carry work, or together with it.
- Not started: flush (`outboundResend` in `rib_commands.go` sends BoRR/EoRR per family via
  `dispatchPeerAction`, RFC 7313 Section 4 "initiated locally" quote; the reactor already skips peers without
  ERR; config routes are already in ribOut, `ConfigStatic` is skipped only on the peer-up replay); bridge
  (`bridge_command.go convertControl`: `clear adj-rib out` to `clear bgp rib advertised <sel>`,
  `flush adj-rib out` to `clear bgp rib out <sel>`, drop earlier same-batch route commands for the selector,
  both runners); a reactor end-to-end test that the withdrawal reaches the wire and bgp-rib drops ribOut;
  docs update-syntax.md, exabgp-bridge.md, exabgp-differences.md; cases api-rib, api-rr-rib (api, old-5.0)
  on ze and on the pin. `Translator.Families` is left to the other agent.

### ExaBGP 6 family check (X-23) and plugin-socket warning, agent stop (2026-10-21, Mac)

Agent stopped at its 100-call budget. **Fix 1 is written, tested, and UNCOMMITTED.** It shares
`types/reactor.go` and 7 reactor mocks with the stopped adj-rib-out agent's uncommitted
`WithdrawAdvertised` work, so neither lands alone; the joint commit was beyond this budget.

- Fix 1 design: `Translator.Families` is the bridge's declared union, not per peer, so the configured
  check lives in ze (also a ze logic fix: a down session queued, and answered accepted, a route its
  OPEN never offers). `reactor_api_carry.go` `SelectedPeersCarry` (BGPReactor method, afi/safi like
  AnnounceEOR) reads `offeredMultiprotocol`, extracted from `buildOpen` (session_negotiate.go);
  `route.ErrNoPeerConfiguredFamily`; `update_text.go` pre-checks every announce group before any is
  sent. Bridge: `RefuseUnsentRoute`, `answerDocuments`, `pendingResult.document` deleted;
  `terminatorResult` (bridge_dispatch_answer.go); `scriptRunner.Dispatch` returns the dispatch error
  only, so a ze `warning` answers done as ExaBGP does. Docs: exabgp-bridge.md paragraph,
  exabgp-differences.md "Which family counts" bullet removed (two differences remain).
- Files: reactor_api_carry.go(+_test), session_negotiate.go, route/route.go, types/reactor.go,
  update_text.go, update_text_test.go, mocks (cmd/{update,commit,cache,peer,raw}/mock_reactor_test.go,
  announce/{announce_test,rfc7999_blackhole_agreement_test}.go, route_refresh/handler/mock_reactor_test.go,
  reactor/peer_ready_producer_test.go), bridge.go, bridge_dispatch_answer.go(+_test), bridge_muxconn.go,
  bridgerun/script.go, the two docs, case api-family-carry-v4 (.ci, etc/.conf, etc/run/.run). Draft
  commit message: tmp/session/2026-10-20-8385ff13-31c1-470c-822b-d731474836f1/scratch/msg-fam.txt
  (machine-local; its content is this entry).
- Evidence: TestSelectedPeersCarryReadsTheConfiguredFamily red with the peer loop overlaid to
  `return nil`; TestHandleUpdateText_FamilyNoSelectedPeerConfiguredIsRefused red with the pre-check
  overlaid away; both green; vet clean on reactor, cmd/..., route_refresh; bridge, exabgp,
  bridgeplugin tests green. `api-family-carry-v4` PASS on the pin (down peer: configured family done,
  unconfigured refused).
- Owed for Fix 1: run api-family-carry-v4 and api-refusals-v4 on ze (blocked: foreign uncommitted
  `internal/component/bgp/config/bgp_routes.go` breaks the ze build, ConfigRouteRequest arity), then
  red on ze by overlaying `update_text.go` pre-check away; the established-but-not-negotiated
  direction has no case (the exabgp DUT runner refuses `option=open:value=drop-capability`); the
  next-hop refusals of ExaBGP `unsendable` (link-local without capability or multihop; RFC 4271
  5.1.3 next-hop-is-the-peer refused when no carrier is left) are not compared yet (ze withholds
  5.1.3 routes per link_scope.go, answer unverified); joint commit with the adj-rib-out files.
- Foreign red: bridgerun TestFleetExpandsABareEOROverTheDeclaredFamilies expects two markers for a
  bare eor, which b0b734ec2e made IPv4 unicast alone (stale test).
- Fix 2 (warning lost on the plugin socket) NOT STARTED. Finding: the terminator
  (`rpc.AppendAnswerTerminator` id/count/faults/message, `AnswerTail`, `Verdict`) has no field for a
  warning, so the fix is a wire-format change in pkg/plugin/rpc plus WriteAnswer/answerMessage,
  SDK answerValue and every consumer; docs ipc_protocol.md / process-protocol.md.

## spec-plugin-ownership-imports, agent a09707eb109bfcd1b (2026-10-21, Mac)

Claimed under the agent marker (`ZE_AGENT_ID`); the parent's ExaBGP claim untouched.
- Done: `931051fa3b` tier walk skips top-level `cache/` (docker route gate read the
  patched GOROOT; `TestDockerExecGateSkipsTheBuildCache` red via overlay, green;
  `./le arch tier check` exit 0). `75167aea59` route J / AC-17: health rpcs moved to
  `ze-l2tp-api` / `ze-bgp-api`, both AC-17 tests red then green, both health `.ci` green.
  AC-15 ns/op from the Mac pasted in the spec (same commit as this section).
- Red at HEAD, not this spec: `TestRegisteredWireMethods` (`plugin/all`) on
  `ze-bgp:rib-clear-advertised` from `2e32def1a9`; row in `plan/journal/hardcoded-count-in-test.md`.
- Left: implementation `/ze-review` by an independent reader over the whole spec's
  implementation (routes A-J, incl. `75167aea59`, `931051fa3b`), recorded with
  `./le spec review record`; then `/ze-close` (closure template, commit A + B).
  Gates owed: `./le verify worktree` (verification debt `plan/verification-debt/0fa95b48.md`).

### ExaBGP 6 adj-rib-out package, fourth agent (2026-10-21, Mac): items 0-4 committed, cases partly run

Stopped at the call budget. Committed:
- `37d6aff8b1` bridgerun stale test: `TestFleetSendsABareEORForIPv4UnicastOnly` (IPv4 EoR only, no IPv6,
  one ack); row in `test/weakened/f1d4344e.md`. `TestFleetLeavesAOneShotScriptStopped` flaked once under
  load (green x3 alone): a flake, not investigated.
- `7339d8b323` bgp-rib `outboundResend` brackets each family with BoRR/EoRR (`resendFamiliesBracketed`,
  RFC 7313 Section 4 quoted). Config routes confirmed in the resend (`collectGroupedRibOutRoutesFiltered`
  skips `ConfigStatic` only when `peerUp`). Three tests red with HEAD `rib_commands.go` overlaid.
- `d27dcdbdf7` bridge: `clear adj-rib out` -> `clear bgp rib advertised <sel>` (`Command.WithdrawsAdjRIBOut`),
  `flush adj-rib out` -> `clear bgp rib out <sel>`; `Net` cancels earlier same-batch route commands the clear's
  selector covers and nets each run between clears separately. Both runners share `convertControl`/`ScriptLines`.
  No-match: the bridge refuses an unserved selector; ze answers `ErrNoPeersMatch` for advertised. Docs:
  exabgp-bridge.md (new section), update-syntax.md, exabgp-differences.md (route aimed wider than the clear).
- `e21bd09703` `TestWithdrawAdvertisedPutsTheWithdrawalOnTheWire` (reactor, wire bytes; red with the
  `WithdrawNLRIBatch` call disabled) and `TestLocalResendAfterAWithdrawalSendsNothing` (rib; no red phase).

Cases on ze (working tree, which also carries other sessions' uncommitted plugin-socket and NLRI work):
api `api-rib` PASS, `api-rr-rib` PASS, `api-refusals-v4` PASS; `api-family-carry-v4` still running past 34 s
when this was written (likely a timeout: re-run it). NOT DONE: old-5.0 api-rib/api-rr-rib on ze (queued
in the same background run); both cases on the pin
(`PYTHONPATH= uv run --no-project --python 3.12 --with paramiko --with 'exabgp @ git+https://github.com/Exa-Networks/exabgp@685f772e4ede5c818d2ff5c84f4ae040afd37be6' ./le --name exarib3 test exabgp api --dut exabgp api-rib api-rr-rib`);
`api-family-carry-v4` red on ze with the `update_text.go` pre-check overlaid away (note `update_text.go` is
dirty in the tree now, another session's). Gates owed: `./le verify worktree`, scoped lint on the four commits.

## spec-config-yang-reference-types closure (2026-10-21, Mac subagent): round-4 review done, round 5 owed

Claimed by agent `affc7043b343b5acc` of session 8385ff13 (agent marker; the parent's ExaBGP claim untouched).
Round 4 independent review of bccd1e7cf5..32d2ed5b51 (plus later edits to the same files) found 0 BLOCKER,
3 ISSUE, all fixed in 65a25421f0: leafref path prefixes were never resolved (`checkPathPrefix`), a leafref
under an rpc input or notification referring to state was refused (`representsConfig`), and two l2tp
"no leafref" claims were stale. Both new tests observed red with the check disabled; yang package tests and
`./le test bgp parse yang-leafref-default-class` green. Findings and NOTEs are in the spec's `## Review Gate`.

Left, in order:
1. Round 5 by a FRESH reader (not agent affc7043...) over 65a25421f0 at least; loop to 0/0, then
   `./le spec review record spec spec-config-yang-reference-types verdict clean rounds 5 file <each code file>`.
2. Retire the three skeletons `plan/spec-config-yang-type-{leafref,identityref,instance-identifier}.md`, one
   commit each (`remove`); each closure wants `tmp/review/config-yang-type-<name>-<sid>.md`.
3. `/ze-close`: commit A (spec + learned), commit B (`remove plan/spec-config-yang-reference-types.md`).
   The spec still lacks a Goal Validation table; its single goal's evidence is
   `test/parse/yang-leafref-default-class.ci` plus the tagged tests in `rfc7950_reference*_test.go`.
Gates owed: `./le go lint run` scoped to `internal/component/config/yang`, `./le verify worktree`
(debt `plan/verification-debt/c1266198.md`). `go test ./internal/component/config/yang/cli/` is red at HEAD
without these edits (`TestDocCommand`: unknown command: show bgp peer list), from other work.

## Closing spec-rfc-verdict-fix-bgp (2026-10-21, agent a887f9b6, stopped at budget)

Done and committed: 25938997c9 (five waiting specs to `blocked`, phase lines of
daemon-drain and rs-replacement, M1 pointer in docker-hosts), and the commit after it
(reseal of the four SHIFTED verdicts RFC4659-3.2-2, RFC4659-3.2.1.1-1, RFC7999-3.1-2,
RFC8654-4-2, resealed in a `git archive HEAD` export so no foreign working-tree edit
leaked in; MUP-SAFI-3.1-1 added to "Blocked by" and to update-propagation AC-5;
Phase 48/48).

Findings that decide the closure:
- Derived listing (spec "Derived listing" jq) re-run 2026-10-21: 6 weak, 0 wrong, all
  six owned in another spec's AC (PATHS-LIMIT-3-7 add-path AC-21, MUP-SAFI-3.1-1
  update-propagation AC-5, RFC4271-9.2-7 rs-replacement AC-3, RFC7999-3.1-2
  update-propagation AC-5, RFC9252-5-3 vpp-srv6-service-policy AC-7, RFC9494-4.2-6 GR AC-4).
  AC-C1 holds.
- MUP-SAFI-3.1-1 is a test-strength weak, not a producer defect: the fix is asserting
  `r.Discarded == 4` and `r.WithdrawnCount == 0` in
  `TestMUPOtherRouteTypesSilentlyIgnoredAtIngress`. Moved by the coordinator's order; a
  test agent can fix it any time (needs P-1 approve, discrimination record, independent re-judge).
- Parent P-3 (spec-rfc-verdict-test-fix-pass) lets a child close before the parent.
- AC-C5 at HEAD (rfc check over an archive export): after the reseal no stale verdict in
  BGP stems. The working tree shows more (RFC8955-4-3 STALE, revert records for rfc2545,
  rfc2918, rfc7705, bfd-strict-mode, rfc9830 producer-changed). CORRECTED 2026-10-21 (agent
  a3e683aa, round-1 review ISSUE 2): the producer-changed records were NOT working-tree only.
  Their producers changed in COMMITTED code (`2e32def1a9` buildOpen/offeredMultiprotocol,
  `7052b662a4` srpolicy EncodeRoute, and the reactor_api_batch.go planBatchAttrs change), so
  they were stale at HEAD; RFC8955-4-3 alone traces to an uncommitted foreign edit. `rfc7313_local_resend_test.go` naming
  violation is another session's commit.
- AC-C3/C6 spot check: 16 of 17 split rows and all 4 missing rows have their own row in
  `rfc/short/`; RFC9552 "Table 9" reserved-bit sentence maps to RFC9552-5.3.2.2-3 only
  by reading, confirm it. RFC9552-5.1-2 is `enforced`.

Left for closure, in order (the closer must not be this agent's author context):
1. `/ze-close` step 1: append `plan/TEMPLATE-CLOSURE.md`, fill Goal Validation (AC-C1:
   the listing above), Implementation Summary, Deliverables (spec line ~1057) and Security
   (~1063) checklists, Documentation Update Checklist (~607).
2. Gates: `./le verify worktree` (main thread or a fresh agent), then `/ze-review` over
   the child's diff, `./le spec review record spec spec-rfc-verdict-fix-bgp verdict CLEAN ...`.
3. Citers: `git grep -n "plan/pre-release/spec-rfc-verdict-fix-bgp"` (the parent, and the
   blocking specs' AC rows, cite it), repoint or bare-stem them in commit A.
4. Commit A then commit B (`remove plan/pre-release/spec-rfc-verdict-fix-bgp.md`).
The claim is held by agent a887f9b6 under session 8385ff13
(`tmp/session/.session-8385ff13-...+a887f9b6a1c02b23e`); the continuation re-claims.

## ExaBGP 6 case reruns, then conf-flow and MUP T1ST (2026-10-21, Mac subagent exa6x)

Case results (working tree carries other sessions' uncommitted edits):
- ze: `api-family-carry-v4` was not hung: it was RED. The bridge sends `send bgp 127.0.0.1,192.0.2.1 ...`
  and a KindAddrs selector looked only at the port-179 map key, so peers on the harness port matched
  nothing ("no peers match selector"); the script restarted five times and the mock server waited out
  its 60 s frame timeout. Fixed in `matchPositive` (`peerAtAddr`, shared with KindAddr), unit test
  `TestPeerSelectorAddrListOnNonDefaultPort` red before, green after. Now PASS, `api-refusals-v4` PASS.
- ze with HEAD `update_text.go` minus the configured-family pre-check (GOFLAGS overlay): `api-family-carry-v4`
  RED ("announce route 2001:db8:1::/48 ... answered ['done'], want the error marker").
- ze old-5.0 `api-rib` PASS, `api-rr-rib` PASS. Pin (685f772e4) `api-rib` PASS, `api-rr-rib` PASS.
- Found: a multi-peer `request peer <a>,<b> flush` is refused, the bridge only logs it; journal row in
  `plan/journal/silent-fall-through.md`.

Committed: `209a278438` (address-list selector on any port, item 1), `63b2159e48` (config refuses a
DSCP above 63: `parseFlowDSCP` bounds through `componentMaxValue`; `TestConfigDSCPAboveSixBitsIsRefused`
red then green; 6.0 conf-flow fixture moved to traffic-class 46, the pin's own value, 0B8165 -> 0B812E,
also `native/conf-flow.conf`). Agent stopped at its call budget. Left, in order:
1. **Red now, owed:** `test/encode/flow-encode.ci` route 6 carries `traffic-class =101` (main thread holds
   the file: change it to 46 and its bytes), old-5.0 and old-4.2 conf-flow (recorded 0x65: observed allowance).
2. conf-flow redirect, the rest of item B: 6.0 conf-flow on ze is red on the first UPDATE (MP_REACH next hop
   1.2.3.4 + 0x0800, the fixture wants no next hop + 010C01020304 0000). The pin's own fixture is
   `~/.cache/uv/git-v0/checkouts/f954c44d1361f3bc/685f772e4/{etc/exabgp,qa/encoding}/conf-flow.*`: `copy 1.2.3.4`
   is `C01008 010C 01020304 0001`, IETF copy. Ze vocabulary today: `redirect-to-nexthop <ip>` IETF (no copy),
   `redirect-to-nexthop-draft` simpson 0x0800 value 0, bare `copy-to-nexthop` simpson value 1
   (`flowSpecActionKeywords`, attribute/flowspec_action.go; `parseExtCommunity` two-word switch,
   config/routeattr_community.go; also extcomm_decoded.go renderer, flowspec-firewall translate.go reads
   `copy-to-nexthop `). The IETF copy form is absent: needs a keyword decision (e.g. `copy-to-nexthop <ip>`
   with the bare simpson word renamed `copy-to-nexthop-draft`), encoders with copy flag 1 (v4 and v6 attr 25),
   draft-ietf-idr-flowspec-redirect-ip fetched to rfc/drafts/ and quoted. Then migrate_routes.go arms and the
   bridge flow translation, old-5.0/old-4.2 allowances. Forwarding-strips-next-hop journal row not started.
3. MUP T1ST zero source length (journal row 2026-10-21): not started.

## Closing spec-rfc-verdict-fix-bgp, round-1 fixes (2026-10-21, agent a3e683aa, stopped at budget)

Claim: re-claimed under the agent marker `+a3e683aaf956e36c2` of session 8385ff13; a887f9b6's
marker is dead (last transcript write 00:50) and the 2026-10-05 `01a0fdb1` marker has no session dir.

Committed with this handover: re-recorded revert records (producer-changed at HEAD) for
BFD-STRICT-MODE-6-1 positive and negative, RFC2918-2-1 positive, RFC7705-4.2-3 negative
(producer `reactor/session_negotiate.go::buildOpen`), RFC9830-2.1-3 positive and negative
(`reactor/reactor_api_batch.go::planBatchAttrs`). RFC2545-3-1/3-2 were already re-recorded by
`cb7a3ca68f`. RFC7950 records are config/yang (another agent), not touched.

Done in the WORKING TREE, uncommitted (the next agent commits them in commit A):
- BLOCKER 1 route (a): `TestMUPOtherRouteTypesSilentlyIgnoredAtIngress` now asserts discarded
  == 4 and withdrawn == 0 apart (new helper `retainMUPCounts`, `retainMUP` delegates to it);
  P-1 approval in `tmp/commit-rfc-approved-485edca9.md` (agent namespace; quotes parent P-1);
  scoped test PASS; `./le rfc discriminate-record` revert record taken over `rfc7606.go::RecognizeNLRI`
  (in `rfc/discrimination/draft-ietf-bess-mup-safi.json`). HAZARD: the same test file now also
  carries a FOREIGN hunk in `TestMUPDefinedRouteTypesSurviveIngress` (T1ST body gains a trailing
  0) from the MUP-encoder agent, which depends on its uncommitted `types.go` and `encode.go`; the
  file cannot land until that agent commits, or lands with it. Its other MUP tests are red on its WIP.
- Update-propagation AC-5: the 2026-10-21 MUP-SAFI-3.1-1 addition removed.
- Citers reworded (ISSUE 4), all saying "closed 2026-10-21" (true only once commit B lands):
  add-path AC-21; rs-replacement (six sites); update-propagation (three); GR (three);
  open-session (three); tunnel-encap; fsm-delayopen (two); epe (two); the parent (two);
  `features/bgp-protocol.md` Defect review row; rfc-gap-difficulty #12 and #21 (both now
  `enforced`) and #44 (RFC2545-3-6 to unowned-rows row 5). ISD rows MUP-SAFI-3.3.3-1/-3/-4 moved
  to `spec-bgp-unowned-rfc-rows` as rows 13 to 15 with a new AC-9.

Still owed, in order:
1. This spec's own "Blocked by" table: the DRAFT-IETF-BESS-MUP-SAFI-3.1-1 row must leave
   (fixed here, not transferred); it stays `weak` until the independent re-judge.
2. OWED BY THE PARENT (not the test author): `./le rfc audit-stamp stem draft-ietf-bess-mup-safi
   from <pending> mode rejudge` for 3.1-1 by a reader who did not write the test change.
3. ISSUE 3 numbers, measured 2026-10-21 over the 60 owned stems' `rfc/audit` and
   `rfc/discrimination`: 807 `enforced` verdicts, 230 with NO discrimination record at all; 6 weak,
   0 wrong (PATHS-LIMIT-3-7, MUP-SAFI-3.1-1, RFC4271-9.2-7, RFC7999-3.1-2, RFC9252-5-3,
   RFC9494-4.2-6). The reviewer's 335/336 counts only verdicts this child moved; that population
   was not reproduced here. Write the reading taken into Goal Validation with both numbers, and
   home the 230 (an existing spec, or a journal row) explicitly.
4. Closure sections (TEMPLATE-CLOSURE, Goal Validation, Implementation Summary, checklists, Review
   Gate, Updated bump), then commit A; commit B only after a fresh round-2 reviewer records CLEAN.

## 2026-10-21 ExaBGP 6 FlowSpec child (spec-exabgp-6-api-compatibility), budget handoff

Landed:
- 2dfb23ed0b test/encode/flow-encode.ci route 6 traffic-class 46 (0x2E); RFC8955-4-3 positive
  discrimination record retaken; old-5.0 and old-4.2 conf-flow SKIP on an allowance naming the
  refusal of traffic-class 101 (test/weakened/920463e1.md).
- 21a081938b test/plugin flowspec-fw-withdraw, flowspec-fw-add, flowspec-announce: stale
  expectations (copy-to-nexthop 0x0800 now carries `04 01020304`, owner deviation of 2026-10-20,
  same bytes simple-flow.ci already asserted). All three PASS.
- c4c085b727 attribute.FlowSpecRedirectToIPv4/IPv6(addr, copyTraffic); config
  `copy-to-nexthop <address>` = IETF copy (C bit), bare `copy-to-nexthop` = Simpson copy; drafts
  rfc/drafts/draft-ietf-idr-flowspec-redirect-ip-16.txt and draft-simpson-idr-flowspec-redirect-02.txt;
  test/encode/flow-redirect-forms.ci routes 3-4; docs/architecture/config/syntax.md.

Owed, in order:
1. RFC8955-4-3 audit verdict is STALE (`./le rfc check`, flow-encode.ci changed): independent
   rejudgment with ze-rfc-audit by an agent that did not author 2dfb23ed0b. Not reseal.
2. Migration arms in internal/exabgp/migration/migrate_routes.go (the flow `then` action switch),
   per the 6.0 grammar configuration/grammar/types/flow.py: `redirect <ip>` to
   `redirect-to-nexthop <ip>` (IETF); `redirect-simpson <ip>` to next-hop plus
   `redirect-to-nexthop-draft`; `copy <ip>` to `copy-to-nexthop <ip>` (IETF; today it maps to
   next-hop plus the Simpson copy); `copy-simpson <ip>` to next-hop plus `copy-to-nexthop`; bare
   `redirect-to-nexthop` Simpson; `redirect-to-nexthop <ip>` IETF; the `-ietf`/`-simpson`
   explicit spellings. The same keywords in internal/exabgp/bridge/bridge_flow.go
   (flowRouteRedirectNextHop, flowRouteCopyNextHop and the action switch) and the wire-JSON rename
   table in bridge_wire_json.go (`copy-to-nexthop ` to `copy-to-nexthop-ietf `). The bridge files
   are being edited by another session (peer flush) as of 2026-10-21 01:30; coordinate.
3. Then 6.0 encoding conf-flow green on ze and on the pin (command in the parent brief), and
   old-4.2/old-5.0 allowances where they sent the old form.
4. Journal row plan/journal/conflicting-feature-contracts.md 2026-10-20: forwarding strips a
   received 0x0800 redirect next hop. Draft now at rfc/drafts/draft-simpson-idr-flowspec-redirect-02.txt
   (Section 3). Fix with a test, mark the row fixed. Not started.
5. Gates: ./le go lint run on internal/core/bgp/attribute, internal/component/bgp/config.

Note: while another session's internal/component/config/yang and internal/exabgp/bridge edits
do not build, run Go tests with `-overlay` mapping those dirty files to HEAD, and functional
tests with `GOFLAGS=-overlay=<json>` (the harness's go build inherits it).

## 2026-10-21 ExaBGP 6 FlowSpec child, continuation (budget handoff)

Landed: be35caa194 (migration `flowRedirectAction`, every 6.0 keyword, tests red then green, docs
exabgp-syntax.md); 81020ca754 (journal rows: 6.0 conf-flow state, asdot migration test red at HEAD).

Owed, in order:
1. Reseal RFC5575-4-8, RFC8955-6-1, RFC8955-6-3, RFC8956-5-1, RFC9117-4.1-1, RFC9117-4.2-1
   (shifted by c4c085b727 in rib_flowspec_validation_test.go) with `./le rfc reseal`, commit only those
   rfc/audit files; then `./le rfc check` for records in those stems. Not started.
2. 6.0 `encoding/conf-flow.ci` holds 4.x frames. `--dut ze`: conf-flow-redirect PASS, conf-flow FAIL
   on the now-correct IETF frame. `--dut exabgp` (pin) fails first on IPv6 flow-label: pin `A1000007DD`,
   fixture `9107DD`. Regenerate the frames from the pin via the mock recorder (`newFrameRecorder`,
   `saveDir` flag, interoplab/bgp/exabgp_server.go), pick one flow-label width, then old-4.2/old-5.0
   allowances, then mark the journal row fixed.
3. Bridge: `update text` (route/route_community.go `parseExtendedCommunity`) has no IETF
   redirect-to-IP word and no attribute-25 support, so bridge_flow.go `readRedirect`/`readWord` cannot
   send `redirect <ip>`/`copy <ip>` as 6.0 does until update text learns both. bridge_wire_json.go's
   rename table already matches the 6.0 reprs. bridge_flow_test.go asserts the Simpson form today.
4. Forwarding strips a received 0x0800 next hop: forward_next_hop.go `payloadNextHop` sets only
   `mpIgnored` for FlowSpec and `applyNextHopFamily` clears it. Keep the address when
   `attribute.FlowSpecRedirectsToNextHop` holds; Simpson-02 Section 3: "When a flow-spec route with
   this community is advertised to an EBGP peer the next-hop address in the MP_REACH_NLRI SHOULD be
   reset to an address of the advertising router by default, per normal BGP procedures." (no IBGP
   rule, so unchanged toward IBGP). Check `applyFactsNextHop` handles a FlowSpec family. Red first.
5. plan/verification-debt/485edca9.md has an uncommitted count bump, possibly from a refused script
   of this agent. Gate owed: ./le go lint run on internal/exabgp/migration.

## 2026-10-21: closing spec-plugin-ownership-imports (closure agent ran out of budget)

Done and committed: `9997a1b5a8` (numberparse: three AS parses to `asn.Parse`, stale
allowlist line; `./le repo tree-check` green), `9e008044bb` (`TestRibProxyRPCRegistration`
selects proxy RPCs by `PluginCommand`, ElementsMatch, ledger `test/weakened/25bc7201.md`),
`1d4f2ade74` (filterapi hooks called outside `mu.RLock`, red/green test; NOT yet independently
reviewed), and the commit carrying this note: the spec's closure record (AC-11d superseded
by `4e01d13a6e`, Architectural Verification, A-1..A-9, TDD Status, Goal Validation, all
TEMPLATE-CLOSURE sections) plus journal rows for the radius reload test, the l2tp/ppp
link-local tests (darwin red, `installLinkLocal` called only from `ipv6_service_linux.go`)
and the `TestReloadRemovalFailureRecoversCommittedService` flake.

Left, in order, for a fresh closure agent:
1. Clear the citers before commit B (ze-close step 6d), bare stem `spec-plugin-ownership-imports`:
   `plan/immediate/spec-bgp-ls-receiver-accept-and-limit-controls.md` (two lines),
   `plan/spec-yang-feature-opt-ins-owned-by-feature.md` (three lines),
   `plan/handoff-linux-2026-10-12.md`, `plan/handover/02-session-2026-10-17-tests-green-and-rfc.md`.
   Re-run `git grep -n plan/pre-release/spec-plugin-ownership-imports`.
2. An independent review of `1d4f2ade74` (two filterapi files changed after the CLEAN
   artifact `tmp/review/plugin-ownership-imports-8385ff13-31c1-470c-822b-d731474836f1.md`).
   If commit B's `CheckReview` refuses on the stale hashes, re-record the artifact after
   that review.
3. Commit B: `./le commit create replace remove plan/pre-release/spec-plugin-ownership-imports.md`
   (citer edits ride in a commit before it), then `./le spec release` and `./le spec citation`.
4. `le-placement-answers.ci` is red only on another session's uncommitted
   `nlri/mup/types.go` (three exported symbols with no caller); not this spec's.

## Closing spec-rfc-verdict-fix-bgp, commit A (2026-10-21, agent a9299db3)

Claim re-taken under agent marker `+a9299db32d905ab90` (session 8385ff13). Committed:
`13bc1a99af` (citers of the spec, MUP-SAFI-3.1-1 out of update-propagation AC-5, ISD rows to
unowned-rows AC-9; their "closed 2026-10-21" wording is true once commit B lands) and
`1d410c3810` (commit A: closure sections, Blocked-by row removed, AC-C2 counts, round-1
table; D-7 measurement in the evidence-strength umbrella). The MUP test strengthening and its
record had already landed inside `32ff59085b`.

AC-C2 counts (60 stems of the 2026-09-28 population): 336/336 moved verdicts carry a record;
807 enforced in all, 230 with none, none moved by this child. Literal every-unit reading: 47/336
(722 of 1538 unit tags unrecorded). Closure takes the change-scoped reading; owner may rule.

Owed before commit B, in order:
1. Independent re-judge and re-record of draft-ietf-bess-mup-safi after `32ff59085b` changed its
   fixtures: STALE 3.1-1, 3.1.1-1, 3.1.1-2, 3.1.2-1, 3.1.3-1, 3.1.3.1-1/-2/-3/-6/-7/-9/-10,
   3.1.4-1, 3.1.4.1-1; SHIFTED 3.1.4.1-3 (reseal); 33 revert records no longer verify (re-take
   with `./le rfc discriminate-record`). 3.1-1's re-judge (`audit-stamp ... mode rejudge`) must be
   by a reader who did not write `retainMUPCounts`.
2. RFC8955-4-3 STALE from `2dfb23ed0b` (flow-encode.ci); SHIFTED RFC5575-4-8, RFC8955-6-1/-3,
   RFC8956-5-1 once the FlowSpec agent commits `rib_flowspec_validation_test.go` (reseal).
3. Fresh round-2 `/ze-review`, `./le spec review record ... verdict CLEAN rounds 2`, then commit B
   (`remove plan/pre-release/spec-rfc-verdict-fix-bgp.md`) and `./le spec release`.

### ExaBGP 6 FlowSpec part B (2026-10-21, Mac)

Spec `plan/immediate/spec-exabgp-6-api-compatibility.md`. Committed:

- `3cc1e2a56a`: `update text` redirect-to-nexthop and copy-to-nexthop take an address. One
  table, `attribute.ParseFlowSpecRedirectToIP`; an IPv6 target goes in attribute 25 through
  `Builder.AddIPv6ExtendedCommunity`.
- `068f67cee0`: the ExaBGP bridge's flow redirect and copy follow the 6.0 grammar.

Owed:
1. An API compat case under `test/exabgp-compat/api/` for the 6.0 redirect/copy forms, green on
   ze and on the pin, red on ze with the bridge mapping removed.
2. Lint over attribute, route, config, `cmd update` and the bridge.
3. `./le rfc check`.
4. The functional flowspec plugin cases.
5. Config package redistribute/ospf tests fail "unknown top-level keyword: ospf" (likely a
   missing build tag); not yet investigated.
6. Watchdog: `internal/component/bgp/plugins/watchdog/config.go` ignores a parse error on
   extended-community (silent fall-through); fix with a refusal (M4) or journal it.

Later the same day: `048e8f33c7` refuses an `interface-set:` item inside extended-community
(bridge flow, route/attributes and family flow forms; migration flow then, announce, static,
vpls, flex), renames `CheckInterfaceSetAS` to `CheckInterfaceSet` and judges transitivity first
as ExaBGP does, and lists every refusal in exabgp-syntax.md Scope. `c1fdc079f5` adds
`test/exabgp-compat/api/api-flow-redirect.ci`: PASS on the pin and on ze (the reactor's foreign
dirty files overlaid to HEAD), FAIL on ze with the bridge mapping overlaid back to Simpson.
`edac2f8d05` is the watchdog fix (items 1 and 6 above are done). Still owed: items 2 to 5 (lint
over the touched packages, now including bridge, migration and watchdog; `./le rfc check`;
functional flowspec plugin cases; the config ospf build-tag failure).

## rfc-verdict-fix-bgp round-2 findings 1 and 2, 2026-10-21 (agent handoff, budget reached)

Finding 1 (dangling citers, stale claims) is done: `3444b9d8f2`. `./le spec citation` OK.

Finding 2 is measured, records NOT yet taken. The measured numbers and the 13 units are
in the spec's Goal Validation row AC-C2 (`plan/pre-release/spec-rfc-verdict-fix-bgp.md`).
Method, so a successor can repeat it: rebuild the 2026-09-28 population from
`git show cb4801b4cc:rfc/audit/*.json` with the spec's "Derived listing" filter plus the
pre-rename paths `internal/plugins/fib/kernel/nexthop_srv6_linux_test.go` and
`internal/plugins/isis/bgpls_export_rfc9552_test.go` (gives 360); join the `enforced` ones
at HEAD to `rfc/discrimination/<stem>.json` on (rid, unit with `#n` stripped); for each
unrecorded unit compare the function body at `cb4801b4cc` and HEAD with
`git log --follow --name-status` to follow renames, and name each commit whose body change
it is. Result: 478 unrecorded tags over 320 units; 54 tags (32 id-unit pairs, 13 units) on
units this child changed (6 commits naming the child or carrying D-15 trailers, plus
`a936e853c7`, which writes owned-stem audit files).

Still owed before commit B:
1. Take 32 records with `./le rfc discriminate-record id <id> polarity <p> unit <file>::<Func>
   route revert producer <file>::<Func>` for the 13 units in Goal Validation. Read each
   unit's `RFC requirement:` tag lines first: the audit's tests map for RFC4760-3-4 names
   `TestRFC4760IBGPMPReachCarriesLocalPref`, whose tags read RFC4760-7-1, so the recorder may
   refuse that pair; that is a mistagged-unit finding to fix, not to skip. Producers: the
   sibling records of each id name them (e.g. RFC9552-8.2.2-x `rfc7606_bgpls.go::validateBGPLSAttr`,
   RFC4271-9.1.2.2-1 `rib/bestpath.go::comparePair`). The interop carrier
   `interoplab/bgp/check_rfc.go::checkRelayWithdrawalShape` needs the interop route.
2. Write the records taken into Goal Validation (replace "Records taken in this round: none yet")
   and remove the first Work Not Done row.
3. The owner answers strict versus change-scoped; the spec now states both with numbers.
4. Fresh review round 3 over `3444b9d8f2` and the Goal Validation edit, then commit B.

### ExaBGP 6 FlowSpec part A (2026-10-21, Mac)

Landed:

- `3beea91222`: the six SHIFTED verdicts resealed.
- `8916c505b1`: the RFC8955-4-3 summary row is `{partial}`, and the deviation is named in
  Support remaining. An independent re-judge to partial is still owed.
- `fc2b3c83ef`: the Flow Label is written as 4 octets, per RFC 8956 Section 3.7 "SHOULD be
  encoded as 4-octet quantities". The `conf-flow.ci` frames come from the pin, `flow-encode.ci`
  carries seq 6, and conf-flow passes on ze and on the pin.

Open:

1. Forwarding drops the 0x0800 redirect-to-IP next hop (`payloadNextHop` sets only
   mpIgnored for FlowSpec, then `applyNextHopFamily` clears it). Being fixed in the next package.
2. conf-flow-redirect fails on the pin: the expected 00A0 UPDATE carrying community 30740 is
   never sent. Not investigated yet.
3. Lint and the exabgp-test stage are owed.

### ExaBGP 6 FlowSpec part B (2026-10-21, Mac, agent budget reached)

Done: `7b0bf3a723` fixes open item 1. Forwarding keeps the 0x0800 next hop toward IBGP and
route-server clients, and resets it to self toward EBGP under auto. It writes the configured
address under self or explicit, and zero length when the destination is not sent extended
communities. Relay reconstruction (RS, RR, adj-rib-in replay from the selecting RIB) dropped the
field too, because FlowSpec StoredRoutes carry no NextHopHex: `relayFlowSpecNextHop` now takes it
from the stored MP_REACH. The new tests are `TestFlowSpecRedirectForwardNextHop` and
`TestFlowSpecRedirectRouteServerKeepsNextHop`. Each went red under its own mutation: with the
redirect branch removed, and with the RS-client keep removed. The scoped reactor run
`-run 'FlowSpec|Relay|NextHop|Forward'` is green. The journal row is marked fixed. The RFC8955-4-3
negative tags still assert length 0, because their fixtures carry traffic-rate and no 0x0800.

Still owed:

1. conf-flow-redirect fails on the pin, and the cause is not found yet. The pin connects, sends 7
   UPDATEs and closes. The mock waits for connection 1, sequence 1: an `00A0` UPDATE ordered
   ORIGIN, AS_PATH, LOCAL_PREF, COMMUNITY `C008087814000078147814`, MP_REACH 1/133, then
   EXTENDED_COMMUNITIES. No sent frame matches. Next step: capture the 7 frames the pin sends
   (its recorder) and diff them against seq 1 of `test/exabgp-compat/encoding/conf-flow-redirect.ci`.
   If the fixture holds 4.x frames, as conf-flow did, regenerate it from the pin. Repro:
   `PYTHONPATH= uv run --no-project --python 3.12 --with paramiko --with 'exabgp @ git+https://github.com/Exa-Networks/exabgp@685f772e4ede5c818d2ff5c84f4ae040afd37be6' ./le --name exaflow7 test exabgp encoding --dut exabgp conf-flow-redirect`.
2. Gates over `7b0bf3a723`: lint over `internal/component/bgp/reactor`, the exabgp-test
   stage, `./le test functional encode`/`plugin` (flowspec), interop (wire-visible change),
   `go test -race -count=20` on the reactor, and an independent review.

### bgp-open-session D2/D1 proofs (2026-10-21, Mac)

Spec: spec-bgp-open-session-rfc-defects (closed 2026-10-21). Claim taken over by a
continuation agent from the finished agent `a2c395bd09a52530c`.

- D2 red is recorded in the spec (commit `1ee515439a`). Two Go `-overlay` copies of
  `capabilityChanges` in `pkg/plugin/sdk/capability_declaration.go`, the tracked file
  untouched: `acceptall` (comparison disabled) reddens the five refused reload `.ci` and the
  SDK and plugin capability-reload units; `refuseall` (every declared code a change) reddens
  `reload-capability-unchanged-accepted` and `reload-gr-explicit-family-list-accepted` and
  the same units. `TestCapabilitySelectors` goes red under an overlay of
  `configjson.CapabilitySelectors` that drops the remote IP.
- D1 interop `bgp-open-extended-params-boundary-frr` passes at `be35caa194` and fails with
  the `d1rev` overlay of `internal/core/bgp/capability/optional_params.go` (FRR sends OPEN
  Message Error/Unspecific, `0 passed, 1 failed`). A-5 is validated by the green run.
- Runs go through the QEMU docker-lab:
  `./le job run label X command env ZE_QEMU_SSH_PORT=2290 'ze.qemu.ssh.port=2290' bash -c "cd <export> && ./le test qemu docker-lab lab 'test integration interop' env 'INTEROP_SCENARIO=<name> GOFLAGS=-overlay=/workspace/tmp/ov/<overlay>.json'"`.
  The non-default port avoids a race on 2222. The guest has a five-minute compile timeout,
  so the first runs fail until the guest build cache fills; retry. The runs used a
  `git archive` export of HEAD because `le` did not build from the working tree then (a
  foreign ExaBGP bridge change in progress). The overlay JSON and replacement sources live
  under the export's `tmp/ov/`, so the guest sees them at `/workspace/tmp/ov/`.
- A-4 is invalidated: the `le` test peer has no `option=open` refusal value (its value arms
  are in `internal/test/peer/expect_keys.go`), so the fallback was used.
- Owed: D3 `frr-software-version` green at HEAD and red with the `d3rev` overlay (drops the
  `logNotificationReceived` call in `internal/component/bgp/reactor/session_handlers.go`);
  spec A-4/A-5 rows, decision blocks, header; an independent judge for the RFC9072-2-1
  enforced stamp and the RFC6286-2.1-1 re-judge against its deviation record; the Review Gate.
- Update, same day: D3 green and red, the A-4/A-5 rows and the header (Phase 8/8) are recorded
  in the spec (commit `e23b95ddca`). Left: the AC-4 judge (RFC9072-2-1, RFC6286-2.1-1) and the
  Review Gate, both for an independent agent. The scratch export was removed.

### LINUX TASK: spec-rfc-verdict-fix-bgp RFC4271-5.1.2-3 records (2026-10-21)

Needs a Linux host with Docker (Colima on the Mac cannot load the `ze-kernel-probe`
AppArmor profile, and the recorder has no QEMU route for an interop carrier). This is
the only thing `plan/pre-release/spec-rfc-verdict-fix-bgp.md` waits on (metadata
"Blocked by"). The exact commands for both polarities are in that spec's Work Not Done
row 1: positive by revert on `peer_forward_facts.go::prependTo` (citation 3); negative
by a one-mutant report that drops `!mods.IsWithdraw()` from the EBGP AS_PATH guard in
`reactor_api_forward.go::forwardUpdateSection` (citation 6). Confirm the negative's red
is `fail(6)` in the scenario output before trusting the record. Commit the
`rfc/discrimination/rfc4271.json` change.

### rfc-verdict-fix-bgp record re-take, 2026-10-21 (agent ac8fb1ba, budget reached)

Done in commit "rfc: re-take BGP records staled by the FlowSpec next-hop work":
13 revert records re-taken over HEAD producer bodies (RFC4271-5.1.2-2 x3, RFC4271-9.2-6,
RFC5575-4-8 x4, RFC7705-4.2-4, RFC9494-4.3-3 x3), RFC5575-4-8 SHIFTED resealed; spec
metadata, AC-C1 (Done: 5 weak, all Blocked by), Audit Summary, Goal Validation AC-C1,
Work Not Done row 1; umbrella D-7 limitation and 52 + 2 count.

Owed by the continuation:
- `rfc/discrimination/rfc8277.json` (RFC8277-3.2.1-1, producer `mpReachNextHopHandler`)
  was re-taken but NOT committed: the FlowSpec agent edited that producer in the working
  tree around the record's time (03:15-03:17), so the record may hold the in-flight body.
  Re-take it after that edit is committed, then commit.
- RFC9552-5.1-3 (`rfc9552_propagate_test.go::TestRFC9552UnknownTypesPropagateOnAForwardedUpdate`,
  producer `reactor_api_forward.go::forwardUpdateSection`) not re-taken: the run hit a
  build break from the in-flight FlowSpec edit. Re-take after that edit lands.
- RFC2545-3-1 and -3-2 (`test/plugin/adj-rib-in-replay-rfc2545-next-hop.ci`, producer
  `reactor_api_relay.go::buildRelayUpdate`, the stored `expect=bgp:conn=1:seq=2:hex=...`
  citation) run only in the guest: `kernel tmp/kernel/build/vmlinuz`. A guest run was
  started; check `git diff rfc/discrimination/rfc2545.json` and commit it if written.
- The in-flight FlowSpec edit (`forward_next_hop.go`, `forward_rs.go`,
  `reactor_api_forward.go`, `filter_delta_handlers.go`, `peer_forward_facts.go`) will
  stale the records above again when it lands; that commit owes their re-take.
- STALE verdicts for an independent judge (not stamped): RFC4659-8-5 and RFC8277-3.2.2-3,
  both on `filter_delta_handlers.go::mpReachNextHopHandler`. RFC4271-9.2-6's verdict is
  not stale (only its record was).
- Spec edits not yet done: Review Gate rows for rounds 2 (findings in `3444b9d8f2`,
  `35b24fbbc8`) and 3; Pre-Commit AC-C5 row from a fresh `./le rfc check`; the
  "closed 2026-10-21" to "closed" sweep over the citers of spec-rfc-verdict-fix-bgp
  (`grep -rn "closed 2026-10-21" plan features`, only lines naming that spec: 14 lines
  in 11 plan files plus the Defect review row of `features/bgp-protocol.md`).

## 2026-10-21 fsfix agent: review of 7b0bf3a723 (FlowSpec redirect next hop)

Committed as f70588873f (verification debt `plan/verification-debt/29a038dd.md`).
Owed next, not done (budget): `./le rfc check` over f70588873f and a re-take of
every record it reports stale whose producer is forwardUpdateSection,
applyNextHopFamily, payloadNextHop, reactorForwardRSSection or
mpReachNextHopHandler, including RFC8277-3.2.1-1 (rfc/discrimination/rfc8277.json
is dirty from another agent's take over the in-flight body: re-take over the
committed body), RFC9552-5.1-3 (never taken: the build broke on the in-flight
edit), and RFC2545-3-2 (rfc2545.json dirty, re-taken in a QEMU guest over the
in-flight reactor_api_relay.go; that file is unchanged by f70588873f, but re-take
it over the committed tree with `kernel tmp/kernel/build/vmlinuz`). STALE verdicts
for an independent judge, not stamped: RFC4659-8-5 and RFC8277-3.2.2-3 on
mpReachNextHopHandler, plus any new one rfc check names. Gates owed: full
`./le verify worktree`; `go test -count=5 -race` over the whole reactor package
exceeds go test's 10m default timeout (it ran 601s and was killed mid-test, not
hung), so it needs `-timeout 30m`; count=1 -race whole package and count=5 -race
over the FlowSpec tests were green. The TestFlowSpecForwardingOmitsNextHop
(flowspec_forward_wire_test.go) case already proves a FlowSpec route without
0x0800 and with a non-zero received next hop leaves with length 0.

What f70588873f does:
ISSUE 1 (egress strip of 0x0800 -> length 0, `redirectActionLeaves`), ISSUE 2
(`TestRelayFlowSpecRedirectNextHop`), ISSUE 3 (nhSelfWithheld comment), the NOTEs
(handler fails closed; peer-own gate on a kept redirect address,
`redirectNextHopIsPeerOwn`; 0x8008/0x8108/0x8208 void the action in
`FlowSpecRedirectsToNextHop`, so origination follows too; link-local never a reset
address; reset address written only for a rule of its own family, so an IPv6 rule
over an IPv4-only speaker now leaves with length 0; flow-vpn and 12/32-octet cases
tested), rfc/short RFC8955-4-3 annotation, journal row
`plan/journal/test-shares-a-process-global.md`. Every new test was red under its
named mutation (scratch `fsmut-*.log`).

Left open: ISSUE 4, draft-simpson-idr-flowspec-redirect-02 Section 3 "BGP speakers
that support the redirect-to-IP extended community MUST also, by default, enforce
the following check when receiving a flow-spec route from an EBGP peer: If the
flow-spec route has an IP next-hop X and includes a redirect-to-IP extended
community, then the BGP speaker SHOULD discard the redirect-to-ip extended
community (and not propagate it further with the flow-spec route) if the last AS in
the AS_PATH or AS4_PATH attribute of the longest prefix match for X does not match
the AS of the EBGP peer." and "It MUST be possible to disable this additional
validation check on a per-EBGP session basis." Absent today; not started (budget).
No rfc/short summary for the draft exists, so no requirement rows; owes a journal
note that the draft is not enrolled. Design found:
- The unicast domain with each path's neighbor AS is already in the RIB:
  `rib_flowspec_validation.go` (`flowSpecUnicast.firstAS`, `flowSpecAuthorized`'s
  LPM loop). "Last AS" read as the most recently prepended (leftmost) AS, the one
  that equals the EBGP peer's AS for a route learned from it; confirm before coding.
  draft-ietf-idr-flowspec-redirect-ip-16 replaced this check with origin-AS rules;
  the task said follow simpson-02.
- Verdict per FlowSpec path (redirect valid or not), computed in
  `reconcileFlowSpecs` beside `eligible`, published like the eligibility lookup
  (`ribevents.RegisterFlowSpecLookup`) so both forward rails and the relay read it.
- Effect on the forward rails: a Remove of every 0x0800 value on code 16 plus the
  zero length, which `redirectActionLeaves` already turns into length 0; local
  install (flowspec-firewall) must drop the redirect action too.
- Config: a per-EBGP-session leaf to disable the check (YANG, `ai/rules/config.md`),
  plumbed to the RIB; plus a .ci test of both polarities and the off switch.

## 2026-10-21 FlowSpec unmatchable follow-ups to c0c89f09a0 (agent of 8385ff13)

Done: adfa12e587 (bridge refuses an ICMP-and-port flow route, braced or flat,
announce or withdraw, answered `error` as the ExaBGP 6.0 pin does, warning names
the route); 9e465ecb28 (parseComponentText, isComponentKeyword and the bridge
derive from `flowspec.KeywordComponentType`); the RFC commit that follows this
entry (RFC8955-4.2-3 `{partial}`, tags, records).

Receive side landed concurrently in 62a025c7b8 (rib `reconcileFlowSpecs` asks
`flowspec.CheckMatchable`), so the `{partial}` ba12c0b7ac wrote is removed in the
next commit. Owed:
- Tag `TestFlowSpecUnmatchableReceivedRuleIsNotPropagated` (rib) with
  RFC8955-4.2-3 and take its discrimination records, so the receive half is on
  the ledger beside origination.
- Regenerate `rfc/requirements/` and `ai/RFC-REQUIREMENTS.md`
  (`./le rfc index-update`, tree-wide).
- RFC8955-4.2-3 has no audit verdict: an independent `/ze-rfc-audit` judge owes
  one. Note for the judge: the positive record is a revert of `anyPresent`,
  which halts the unit in its negative loop before the positive loop runs; a
  mutant on `CheckMatchable` (always refuse) would prove the positive half
  directly.

## 2026-10-21 fsrecv agent: FlowSpec receive side (RFC 8955 4.2, simpson-02 Section 3)

Item 2, committed (rib: a received FlowSpec rule no packet can match is not
propagated): `reconcileFlowSpecs` makes an unmatchable received rule
ineligible (kept in the peer's RIB). FRR only ignores it at PBR install
(`bgp_pbr.c` `bgp_pbr_validate_policy_route`: "match multiple port/imcp
operations: too complex. ignoring."); BIRD has no such check. The RFC 8955
row's {partial} receive gap can now be closed and re-judged.

Item 1, committed in the same session (FlowSpec redirect-to-IP EBGP check):
FRR (`bgp_ecommunity.c`, `bgp_pbr.c`, `bgp_flowspec*.c`, `bgp_route.c`) and
BIRD (`nest/rt-table.c` `rt_flowspec_check`) have no such check. "Last AS"
is the origin (rightmost): the draft's authors restate it in
draft-ietf-idr-flowspec-redirect-ip-02 Section 3 as "the best matching route
to X is not a BGP route with origin AS matching the peer AS". The RIB
computes the verdict (`rib_flowspec_redirect.go`), strips 0x0800 from the
installed communities, and publishes the refused (peer, msgID) set through
`ribevents.FlowSpecRedirectRefused`; both forward rails (`forwardUpdateSection`,
`reactorForwardRSSection`) remove 0x0800 and send length 0
(`discardRedirectAction`). Off switch: `flowspec-redirect-check { enabled }`
in `ze-rib.yang` at bgp, group and peer levels. Tests: RIB
`TestFlowSpecRedirectToIPEBGPCheck` and `TestFlowSpecRedirectConfigInheritance`,
reactor `TestFlowSpecRedirectRefusedForward` and
`TestFlowSpecRedirectRefusedRouteServer` (live daemon RIB plus RS rail). Each
was red under its named mutation (leftmost-AS, no off switch, no strip, IBGP
checked, no rail branch, no RS ask); scratch `fsr-mut-*.log`.

Left open (budget):
- A `.ci` functional test of the leaf through the real config path (both
  polarities). The parse and the inheritance are unit-tested only.
- Gates owed: `./le verify worktree`; `./le rfc check` (records whose producer
  is `forwardUpdateSection`, `reactorForwardRSSection`, `applyNextHopFamily` or
  `reconcileFlowSpecs` may go stale and need a re-take).
- Journal row `plan/journal/implemented-draft-not-enrolled.md`: the draft has
  no `rfc/short` summary, so these tests carry no requirement tags.
- FRR and BIRD clones are under `tmp/ext-frr-fs`, `tmp/ext-bird-fs`, and the
  successor drafts under `tmp/ext-drafts-fs` (untracked data).

## 2026-10-21 fsr3 agent: review fixes for 15c484a543 and 62a025c7b8 (budget reached)

Landed (each test red first, green after):
- 27edca2c3f BLOCKER 1: both rails asked the refusal by the forward cache id;
  replay (RelayStoredRoute) leaked 0x0800. Now the received generation.
  `TestFlowSpecRedirectRefusedRouteServer` red then green;
  `TestFlowSpecRedirectRefusedReplay` (origin flip twice, no FlowSpec UPDATE)
  and rib `TestFlowSpecRedirectConfigSwitchRejudges` (installConfig off/on) new.
- ccf6880bb7 ISSUE 1: refusal is per rule now.
  `ribevents.FlowSpecRedirectRefused(route ValidationRoute, msgID)`, answered
  from `flowSpecState.paths[key]`; `forwardValidationWire` splits refused rules
  into a third UPDATE in place (three reversals, no allocation) and
  `forwardUpdateValidated` sends it with `forwardSourceInfo.flowSpecRedirectRefused`.
  The RS-rail ask was removed: FlowSpec never takes that rail
  (`sourceUsesCachedForward`). Tests `TestForwardValidationSplitsRedirectRefusedRules`,
  `TestFlowSpecRedirectVPNVerdictPerRule`, both red under their mutations.
- 8f11c35354 ISSUE 2: LocalASN 0 is external (fail closed).
  `TestFlowSpecRedirectUnknownLocalASIsChecked` red then green.
- 072fb3bdf0 item 7: `TestFlowSpecUnmatchableReceivedRuleWithdraw`.
- 2e8121d321 BLOCKER 2a (parse half) and ISSUE 4: `test/parse/flowspec-redirect-check.ci`
  (three levels, both values, non-boolean refused with "expected
  true/false/enable/disable"); guide row in `docs/guide/configuration.md`;
  `ze-rib.yang` description made true (rule's own domain, per rule, no match
  refuses, unknown local AS checked).
- 4cf6dd6152 re-took RFC9494-4.3-3 (both polarities) and RFC9552-5.1-3,
  staled by the rail edits. `./le rfc check` then had no violation over
  this work; the 18 left are other sessions' (rfc7950 config/yang records,
  rfc4659/rfc8277 mpReachNextHopHandler, RFC8955-4-3 stale requirement, two
  file-name rows).

Still owed:
- BLOCKER 2a functional half: a `.ci` through the daemon (two ze-peers, the
  RIB attached) where an EBGP-learned redirect is forwarded without 0x0800
  and with next-hop length 0 when the check refuses, and with both when
  `flowspec-redirect-check { enabled false }`. Model: `test/plugin/bgp-rs-fastpath.ci`.
- BLOCKER 2b interop: FRR scenario via the QEMU docker-lab (recipe in the
  "bgp-open-session D2/D1 proofs" section above, `ZE_QEMU_SSH_PORT=2290`,
  `ze.qemu.ssh.port=2290`): FRR sends a redirect rule whose X resolves to a
  route originated in another AS; a second FRR or GoBGP must receive it
  without 0x0800; red with the check disabled by overlay; inventory row in
  `docs/architecture/testing/interop.md`.
- Owner only: `./le rfc approve` for the [WEAKENED] flag on
  `rib_flowspec_validation_test.go` (15c484a543's refactor). Not approved here.
- Gates: `./le verify worktree` over 27edca2c3f..4cf6dd6152 (debt rows in
  `plan/verification-debt/8675d4bc.md`), and `./le test functional parse`.

## 2026-10-21 fsr4 agent: redirect-check functional proof landed, interop owed (budget)

Landed: da23dcee89, BLOCKER 2a functional half.
- `test/plugin/flowspec-redirect-check-refused.ci`: two ze-peers (conn_map,
  127.0.0.1 AS 65002 source, 127.0.0.2 AS 65003 receiver, both `rs-client`),
  bgp-rib + bgp-rs + bgp-adj-rib-in. Source sends 192.0.2.0/24 AS_PATH
  65002 65009, then the rule (dest 192.0.2.0/24, 0x0800, NH 192.0.2.77). The
  receiver gets the rule exactly `...0032 02 0000001B 800E0B0001850000050118C00002
  40010100 40020602010000FDEA` (NH length 0, no EXT_COMMUNITY). Green 3 of 3.
  Red under the `refusedoff` overlay (`flowSpecRedirectRefused` returns false):
  the receiver got `800E0F00018504C000024D...` with the community.
- `test/plugin/flowspec-redirect-check-disabled.ci`: same with
  `flowspec-redirect-check { enabled false }` on the source. The receiver gets
  NH 192.0.2.77 and C010080800000000000000 (exact hex). Green 2 of 2. Red under
  `switchignored` (`parseFlowSpecRedirectConfig` stores every peer enabled).
- Overlay sources were under the parent session's scratch `ov/` (not
  portable): each is a one-line edit of
  `internal/component/bgp/plugins/rib/rib_flowspec_redirect.go`, the final
  `return` of `flowSpecRedirectRefused` made `false && (...)`, and
  `cfg.peers[key] = enabled` made `enabled || true`. The plugin runner honours
  `GOFLAGS=-overlay=...` (it rebuilt ze, "building... ready").
- Found while writing it, and why seq 3 is loose: the route server sends the
  rule twice, live and again on the RIB's ValidationChange (every new path is
  a change, `emitFlowSpecChanges`). The live copy is a WITHDRAWAL when the
  forward gate asks before the RIB has judged that UPDATE, otherwise a
  duplicate announcement. Seq 3 matches either by NLRI, seq 4 is exact.
- `.ci` mechanics learned: a conn_map peer serves connections IN TURN and only
  moves to the next one after the current connection's last EXPECTATION is
  met, so sends go first on conn 1 and ze's End-of-RIB expect last; seq numbers
  are global across conns; the daemon must be stopped (`action=sigterm`) because
  the batch waits for the daemon to close every connection.

Still owed:
- BLOCKER 2b interop, not started. Finding that changes the brief: neither FRR
  (receive-only FlowSpec) nor GoBGP (its redirect actions are the 0x8008 /
  0x8108 / 0x8208 route-target forms, not draft-simpson 0x0800; confirm in
  GoBGP source before relying on it) can ORIGINATE a 0x0800 redirect-to-IP
  rule. Proposed shape, modelled on `test/interop/scenarios/bgp-flowspec-gobgp`:
  the raw injector (`inject.msg`, `inject-args --asn 65004`) sends the
  other-AS cover and the rule to ze; ze relays to FRR (which decodes 0x0800 as
  redirect IP NH), and the checker in `internal/le/interoplab/bgp` requires the
  rule in `show bgp ipv4 flowspec detail json` with no redirect action. Name it
  e.g. `bgp-flowspec-redirect-check-frr`; register via a
  `register_<name>.go` init into `specialCheckers` or the op table
  (`checkers.go`), never a new central case. Note `helper.go` already holds a
  per-scenario `switch` (pre-existing). Red with the `refusedoff` overlay
  through the QEMU docker-lab recipe above; inventory row in
  `docs/architecture/testing/interop.md`. The owner may prefer an ExaBGP
  sender instead of the injector: ask only if the injector shape is refused.
- Gates for da23dcee89: `./le test functional plugin` (debt row in
  `plan/verification-debt/de91e90f.md`).

### FlowSpec CheckMatchable completed (2026-10-21, Mac)

Spec `plan/immediate/spec-exabgp-6-api-compatibility.md`, RFC8955-4.2-3.
Commit A (recorder tells a function from same-named methods) landed as
78fb6a2196.

State carried from the predecessor agent (uncommitted at handoff):
- `internal/core/bgp/nlri/flowspec/unmatchable.go` rewritten:
  `CheckMatchable(afi, has, matches)` intersects the protocol sets of the
  transport components (ports {6,17}, ICMP {1} v4 / {58} v6, tcp-flags {6}),
  evaluates a protocol operator expression over 0..255 (RFC 8955 4.2.1.1
  AND/OR), and a fragment expression over the transport-bearing states.
  `anyPresent` deleted. The message keeps "ICMP has no ports" (the parse .ci
  asserts it).
- Tests: core `rfc8955_unmatchable_test.go` (TestCheckMatchable), plugin
  `rfc8955_unmatchable_values_test.go`, rib
  `rfc8955_rib_flowspec_unmatchable_values_test.go`, bridge
  `bridge_flow_unmatchable_values_test.go`. Existing tagged tests were not
  edited; new tagged functions were added instead. The core and rib files
  were renamed to the rfc8955_ prefix by `./le rfc rename` after
  `./le rfc check` asked for it.
- Callers: bridge `flowRouteMatchable` (bridge_flow.go, bridge_command.go) and
  migration `flowRouteMatchable` pass the AFI and a nil operator lookup (they
  hold types only); the migration check moved after IPv6 detection.
- Docs: `docs/architecture/wire/nlri-flowspec.md` "Combinations that match no
  packet"; `docs/exabgp/exabgp-differences.md` bullet "More flow routes match
  no packet".

Done by this agent, landed in the commit "flowspec: a flow route no packet
can match is refused by its values too":
- Red: under a Go overlay restoring HEAD's ICMP-and-port-only rule, the new
  tests fail on every new row (core TestCheckMatchable 18 rows, plus
  TestCheckMatchableTypesOnly and TestCheckMatchableNamesTheFix; plugin
  TestFlowSpecUnmatchableByValueRefused; rib ByValue 4 rows; bridge
  TestFlowRouteUnmatchableTypesRefused) and the ICMP-and-port rows pass.
- Records in `rfc/discrimination/rfc8955.json`, all RFC8955-4.2-3: mutants
  on `unmatchable.go::CheckMatchable` from a one-file gomu report
  (`narrowed.empty()` to false for the negatives of TestCheckMatchable and
  TestFlowSpecUnmatchableRefused; the protocol clause to false for the
  ByValue negatives; the protocol clause to true for every positive), and a
  revert of CheckMatchable for `test/parse/flowspec-unmatchable-refused.ci`.
  No record names `anyPresent` any more.
- `bin/le` was older than 78fb6a2196, so the recorder refused the
  same-named methods until `./le --update`.

Owed by the main thread:
`./le go lint run`, `./le test functional parse`, exabgp-test,
`./le doc check verify`, and an independent re-judge of RFC8955-4.2-3.

## 2026-10-21 fsr5 agent: FlowSpec double send fixed, interop not started (budget)

Race (M4): the live forward gate (`flowSpecSection`,
`reactor/forward_validation.go`) asked `RouteEligible(key, msgID)` while the RIB
judges the same generation from its own UPDATE event and then ALWAYS emits a
ValidationChange for it (`emitFlowSpecChanges`: the verdict carries msgID), which
rs `processValidation` / rr replay through `RelayStoredRoute`. So each rule left
twice; when the gate asked first the live copy was a withdrawal (flap).
Fix (in the commit that carries this section): the live forward of a received
UPDATE keeps no FlowSpec rule; only a stored relay (`validationReplay`) does.
Unicast siblings still go live. Page: `structural-forwarding.md`. Journal row:
`new-caller-assumes-the-old-callers-timing.md`. Tests: new
`TestFlowSpecLiveForwardDefersToRIBVerdict` (red on old gate, green now);
`TestFlowSpecForwardingRequiresAuthorizationAcrossRails` and the direct split
test updated; the wire tests that drove FlowSpec through the live forward now
use `forwardViaRIB` (`reactor/flowspec_rib_relay_test.go`), the RIB-driven path.
reactor, rs, rr, rib packages green before the rs edit below. Both
redirect-check `.ci` now expect the rule once (seq 4) and reject a withdrawal of
it; refused is red under the old gate (overlay), disabled red 3 of 5.

Left UNCOMMITTED, decide first: `plugins/rs/server_validation.go` +
`server_handlers.go` drop the peer-up cut skip in `replayFlowSpecs` (a
post-cut rule's ValidationChange is not ordered against rs's peer-up, so with no
live copy it can reach nobody; rr has no cut). Unproven (no failing run traced
to it) and it reds `TestPeerUpReplaysExistingFlowSpecBeforeEOR`, which encodes
the cut. Either prove the hole with a test and update that test, or restore the
cut. `./le test bgp plugin -c 5` on both `.ci` timed out 1-3 of 5 with and
without the fix while another session's gomu-fs ran; sequential runs 4 of 4
green. Investigate a timed-out run's peer log before calling them stable.

Owed: re-take discrimination records staled by the edited tagged tests
(`flowspec_forward_wire_test.go`, `rfc8955_width_wire_test.go`,
`rfc8955_flowspec_forward_mixed_test.go`; `./le rfc check` names them); gates
`./le go lint run`, `./le test functional plugin`, reactor `-race -count=20`.
Interop `bgp-flowspec-redirect-check-frr` (task 2 of the brief) NOT started:
the shape in the fsr4 section stands; first check for an ExaBGP image under
`test/interop/scenarios/bgp-exabgp-*`.

## 2026-10-21 fsr6 agent: single export landed, rs cut decided, one .ci defect open

Landed: `4f0134cacd` (fsr5's fix + the rs edit + shard `test/weakened/fbb21666.md`
+ debt `plan/verification-debt/fbb21666.md`, which owes the owner's approval of
the two tagged tests' drive change), `129396605b` (reseal of RFC8955-4.2.2.x).
rs cut: the gap is real. `TestPeerUpReplaysFlowSpecJudgedBeforeTheTargetWasUp`
(rs/server_validation_test.go) processes the ValidationChange before the target
is up, then peer-up with cut 42 below the rule's msgID 43: red under HEAD's cut
(target gets only EOR), green without it. Cut removal kept;
`rib-storage-design.md` and the `ForwardFrom` comment say so.
`test/weakened/9f4f57f3.md` (fsr5's shard in the wrong session) is superseded
and untracked: delete it.

OPEN, not load: both redirect-check `.ci` hang intermittently (3 of 50 under
`-c 25`, and one in a plain pair run at load 40; never under `ze_log=debug`).
The one timed-out peer log captured: the receiver got the RULE and never the
UNICAST route (seq 3, `02020000FDEA0000FDF1`) in 90 s; no rs WARN (catch-up
budget not hit). The runner's "all expected messages received" line is wrong
there. Not yet root-caused: suspect the unicast's path to the receiver (live
forward held behind the replay fence, or the adj-rib-in replay below the cut),
not FlowSpec. Next: capture a failing run with rs/reactor debug that does not
mask the race (`ze_log=debug` does), per-iteration `-s` dirs
(`./le test bgp plugin --pattern flowspec-redirect-check -s <dir>` in a bounded
loop, break on failure).

Records: `rfc/discrimination/rfc8955.json` holds the three re-taken RFC8955-4-3
records (TestFlowSpecForwardingOmitsNextHop pos/neg, KeepsLegacyNextHop neg)
UNCOMMITTED, because the same file carries another session's CheckMatchable
re-records against its uncommitted `unmatchable.go`: land it with that session.
`rfc/audit/rfc5575.json` holds a reseal of RFC5575-4-1 fingerprinted over that
session's uncommitted `plugin_test.go`: same. Still owed: `/ze-rfc-audit` re-read
of RFC8955-4-3 and RFC5575-4-8 (stale-unit, reseal refuses).
Green: rs, rr, rib packages; reactor `-run FlowSpec|Validation|Forward`;
`./le test bgp plugin --pattern flowspec` 13/13 (5 skipped).

## 2026-10-21 fshang agent: redirect-check hang narrowed, runner hint fixed (budget)

Landed with this section: the runner's timeout hint. `likelyCauseTimeout`
(`internal/test/runner/report.go`) no longer says "All expected messages
received" on a count alone: it quotes the check peer's own "expectations unmet"
line when present, else says the count cannot name the unmet expectation. The
timeout report now prints RAW PEER OUTPUT like the mismatch report
(`printRawPeerOutput`). Red then green:
`TestLikelyCauseTimeoutNeverClaimsEveryExpectationMet`.

Hang, NOT fixed, NOT root-caused. Reproduced at HEAD 3ddee31ae3: `-c 25` gave
2/50 timeouts (both on the -refused id). `-c N` is quiet stress mode and drops
the failure report, so reproduce with a loop of single runs breaking on rc!=0,
over a gitignored copy `test/draft/plugin/fsdbg-refused.ci` (the .ci plus
`option=env:var=ze.log.bgp.rs:value=debug`, same for `bgp.adj.rib.in`,
`bgp.reactor.forward`, `bgp.routes`; timeouts cut to 8s) run with
`./le test bgp plugin -draft --pattern fsdbg`. Those per-subsystem debug keys do
NOT mask the race: failures at runs 7 and 13. The copy is still in place (untracked,
gitignored), so the next agent can reuse it.

What the two captured failures show (ze stderr):
- rs applied BOTH peers up with `cut=0` before any UPDATE, and both rs replays
  finished (EOR sent, `plugin session ready` credited 3/3, fence lowered) before
  the forwards. So the replay owned nothing: the unicast's ONLY path to the
  receiver is the live forward. The replay-fence and below-cut suspects of the
  previous section are ruled out for these runs.
- rs logged no `forward matched no target`, so rs did not drop it.
- One forward logged `ForwardUpdatesDirect: dispatch returned id=20 err="all
  destinations suppressed by egress policy"`. That is most likely the FlowSpec
  UPDATE (forwardUpdateValidated returns that error when the live forward carries
  no rule, by design), not the unicast. Unproven: map msg ids first.
- At shutdown: `forward batch flush failed peer=127.0.0.2 ... broken pipe`, so
  bytes for the receiver were still unwritten when the daemon stopped.

Leading hypothesis (unverified): the unicast is written into the receiver's
bufWriter by the rs fast path (`tryDirectWriteNoFlush`, forward_rs.go) on the
source's read goroutine, and the flush is deferred to `flushFwdDirty`
(session_write.go), which runs only when the SOURCE's reader drains. When
`dst.writeMu.TryLock()` fails there (another writer holds the receiver, e.g. the
RIB's ValidationChange export of the rule or the replay EOR), the receiver is
kept for "the next flush cycle", and that cycle never comes because the source
sends nothing more after its two UPDATEs. If the holder does not flush the
bufWriter itself, the unicast stays unflushed until shutdown. Check first:
whether every writer that takes writeMu flushes the bufWriter on release, and
the `state != Established || conn == nil` arm, which drops a dirty dest
without flushing. A deterministic test would hold dst.writeMu across
flushFwdDirty and assert the parked bytes still reach the conn without
another source read.

The next capture prints RAW PEER OUTPUT (the runner fix), which settles whether
the unicast reached the wire at all, and in which order relative to the rule.

Not done from the brief: the .ci fix with its red-first Go test, the 100/100
under `-c 25`, the journal row for the hang (no row exists yet; grep found
none), and the `rfc/discrimination/rfc8955.json` re-take plus `./le rfc check`
(not started). Disk was full (453M) during this work: `./le scratch
cache-clean` freed 72G.

### Forward flush hand-off and redirect-check hang (2026-10-21, Mac)

Landed: b228f3f8ae `reactor: hand a contended fast-path flush to the forward
worker` (flushFwdDirty hands a contended destination to its forward worker,
`Peer.handOffForwardFlush`; test `TestFlushFwdDirtyContendedReachesWire`, red
before). Committed with `rfc-change-ok`: three RFC9687-4.3-8 tests changed
signature only (`appendFwdDirty(nil, s)`), so they wait in the owner's approval
batch (`plan/verification-debt/8a08314c.md`). It is a real defect fix and NOT the
hang's cause.

Gate results with the fix in the binary:
- `.ci -c 500` over ids 374/375 (redirect-check refused/disabled): 6 timeouts in
  1000 (1 and 5).
- `--pattern flowspec`: 13/13 pass, 5 skipped.
- `--pattern rs` matches 844 tests (substring): 801 pass, 42 fail, 1 timeout
  (374, the hang). The 42 are commit/session-editor/pki tests failing on "store is
  owned by another process" (concurrent runs), not the reactor.
- reactor `-race -count=10`: see the agent report (ran in background).

Hang root cause, evidenced but not yet fixed. Captured with the fresh runner
(`./le --name fshang test bgp plugin -draft --pattern fsdbgx`, 40 gitignored
copies of `test/draft/plugin/fsdbg-refused.ci` run in parallel: 3/40 fail under
that load, so a capture takes one run). RAW PEER OUTPUT of a failure: the
receiver gets the RULE (`0032 02 ... 800E0B0001850000050118C00002`) while the
fixture still waits for the unicast (`contains=02020000FDEA0000FDF1`, seq 3);
the check peer prints "failed: message mismatch" and exits, and ze runs on to
the timeout. So the unicast is not lost: it is ordered after the rule.

Why: the rule reaches the receiver only through rs `processValidation`
(`plugins/rs/server_validation.go`), on the RIB's ValidationChange; the unicast
through the rs worker's live batch (`batchForwardUpdate`, flushed together with
the rule's own suppressed id). `processValidation` flushes the worker's batch
first, which orders it only against UPDATEs the worker already holds. The RIB
and rs receive the source's UPDATEs on independent deliveries, so the RIB can
judge the rule and its ValidationChange can reach the rs worker BEFORE the
unicast's update-received does: the relay of the rule goes out, the unicast
follows. `TestFlowSpecValidation...` in `server_validation_test.go` (around
line 240, "seenMsgID 42: this route server has not yet seen the UPDATE (43) the
RIB has already judged") already exercises that window for the peer-up case.
The ze log of each failure shows one `ForwardUpdatesDirect ... suppressed` (the
rule's live id) and no rs drop. Not yet proven by a log line: add a debug in
`processValidation` naming `path.MsgID` and the highest msg id the worker has
processed for the source.

Fix to build (red first): on the source worker, park a FlowSpec validation
whose `path.MsgID` is above the highest msg id that worker has processed, and
run it right after the worker batches that msg id (flush, then relay). Open
design point before coding: a parked replay must still be released if that
UPDATE never reaches rs (source down: drop it, handleStateDown withdraws;
another reason: name it). Red test: a unit test in `server_validation_test.go`
that calls `processValidation` for a rule with MsgID 43 before `processForward`
of the unicast (42) and asserts the recorded order is unicast forward, then rule
relay. Proof: 0 failures over 1000 parallel-copy runs against the 3/40 baseline.

Still owed: `./le rfc check` after b228f3f8ae named RFC9687-4.3-8 STALE audit
verdict (re-judge with `ze-rfc-audit`, mode rejudge; signature-only change) and
five discrimination records (RFC9687-4.3-8 x3, RFC9494-4.3-3 x2), re-taken by
this agent's `retake.sh`; see its report for the outcome. The runner still
reports a check-peer mismatch as TYPE timeout when ze keeps running: a second
journal-worthy defect in `internal/test/runner`.

### FlowSpec ordering race: rs worker fix landed, two shapes remain (2026-10-21, Mac)

Landed:
- 123c46d4b5 `rs: park a FlowSpec relay until its source worker batches the
  UPDATE`. Confirmed first by a debug overlay in `processValidation`: in the
  failures `path-msg-id=20 worker-processed=0`. The source worker parks a
  FlowSpec relay whose path MessageID is above the highest MessageID it has
  handled (`forwardBatch.processed`), releases it after that UPDATE (flush, then
  relay); pool `onIdle` releases with a warning, `onClose` drops with a warning.
  Three tests in `server_validation_test.go`, all red with parking disabled by
  overlay; rs package green under `-race`.
- 8719b6ea1a `runner: report a check-peer mismatch, not timeout, past the
  deadline` (`failPastDeadline`, `TestPastDeadlineReportsPeerMismatch` red
  first). `bin/le` must be refreshed (`./le --update`) before the runner shows it.
- Reactor `-race -count=5 -timeout 40m -run 'Forward|Flush|Direct|RS'`: ok (42s).
- `./le rfc check` after both: exit 2, every finding in other sessions'
  uncommitted FlowSpec test files or rfc7950; none in rs or the runner.

NOT done: the proof. The 40 parallel copies still fail: 1 in 120, then 2 in 80
(was 3 in 40). A second overlay (`DEBUG-FSVALID` in `relayFlowSpecValidation`,
`DEBUG-FSREPLAY` in `replayFlowSpecs`) shows two further shapes, both with the
receiver getting the rule and then failing on the unicast (13; rule is 14):
- (a) fsdbgx-16: receiver up with `cut=0`; the worker relayed with
  `handled=14`, right after the flush that logged `ForwardUpdatesDirect id=14
  suppressed`, and rs sent the receiver its EOR 1 ms later. So the relay was
  ordered after 13 inside rs, and the overtake is downstream: the receiver was
  inside its peer-up replay, so look at the reactor rails (receive fast path in
  `forward_rs.go` vs the API path in `reactor_api_forward.go`, the replay fence
  `forwardOrderHold`, overflow) for how 13 can land behind a later live relay,
  or be dropped.
- (b) fsdbgx-19: the receiver came up after both UPDATEs (`target-cut=14`), so
  13 is owed by the adj-rib-in replay and the rule by `replayFlowSpecs`
  (`replayForPeer` runs them in that order). adj-rib-in sends its replay from
  its own process; whether its routes reach the engine before rs's
  `RelayStoredRoute` is not ordered. Check whether the replay command answers
  before its routes are dispatched.
Both are rows in `plan/journal/new-caller-assumes-the-old-callers-timing.md`
(open). The 40 gitignored copies `test/draft/plugin/fsdbgx-NN.ci` are KEPT for
the continuation; remove them once 0 failures over 1000 runs is shown
(`tmp/.../scratch/hang/loop.sh N`; pass `GOFLAGS=-overlay=<json>` for debug).
Owed gates for 123c46d4b5 and 8719b6ea1a: `./le go lint run` scoped to rs and
the runner, `./le verify worktree` (debt rows in `plan/verification-debt/21a4961e.md`).
Foreign red seen: `TestCIAcceptOnlyLint` on `test/parse/iface-unnumbered-loopback.ci`
(3c2c96ec01).

### FlowSpec ordering race: both remaining shapes fixed (2026-10-21, Mac)

A file-writing debug overlay (reactor write/dispatch/overflow, adj-rib-in and
rs replay; not committed) showed both causes:
- (a) receiver up with cut 0: `replayFlowSpecs` replayed rule 14 (above the
  cut) with InitialUpdate, which passes the replay fence, while the live
  unicast 13 waited behind it. Fix: the FlowSpec replay carries only rules at
  or below the target's cut, like Adj-RIB-In's unicast replay; the source
  worker now parks a relay even with no target yet, so a target coming up
  meanwhile gets the rule live after the worker handles its UPDATE.
- (b) receiver up after both (cut 14): the live validation relay of rule 14
  (InitialUpdate false) was released when the receiver's initial sync ended,
  before rs's replay sent unicast 13: the replay fence was never raised for
  rs. `registry.FencesLiveForwards` looked up the PROCESS name, and the
  fixtures attach `internal rs { use bgp-rs }`, so "rs" matched no
  registration. Fix: alias resolution through a plugin-server seam
  (`SetRuntimeRegistryNames`, `Server.processRegistryNames`), the resolution
  `declaresSessionReady` already makes.
Red first: `TestPeerUpReplaysExistingFlowSpecBeforeEOR`,
`TestPeerUpReplaysFlowSpecJudgedBeforeTheTargetWasUp` (against HEAD rs code),
`TestFencesLiveForwardsResolvesTheProcessAlias` (resolution disabled by
overlay). Proof: 0 failures in 25 x 40 = 1000 parallel runs. The fsdbgx copies
are deleted. Owed gates (main thread): reactor `-race -count=5 -timeout 40m
-run 'Forward|Flush|Direct|RS|Replay'`, `./le rfc check` and re-takes,
`./le go lint run` scoped to rs, registry, plugin/server, `./le verify
worktree`. Foreign red: `internal/component/plugin/server` has 31 failing
tests at HEAD versions of the touched files too (command registry tests,
e.g. `TestValidBgpEventTypes`), not from this change.

### ExaBGP 6 Phase 7 AC-41 (2026-10-21, Mac)

Spec `plan/immediate/spec-exabgp-6-api-compatibility.md`. The exa7 agent ran
out of budget with AC-41 written and green but NOTHING COMMITTED.

Done, uncommitted at the time of writing, tests green:
- Design (M3): peer presence container `shutdown { message "<text>" }`
  (ze-bgp-conf.yang, peer-fields, revision 2026-10-21), the FRR
  `neighbor X shutdown [message MSG]` shape (bgpd.c
  peer_flag_modify_action: PEER_FLAG_SHUTDOWN sends CEASE_ADMIN_SHUTDOWN with
  tx_shutdown_message) and BIRD proto_cmd_disable/enable (nest/proto.c: a
  runtime disable with message overrides config until reconfigure). Commands
  `request peer <sel> disable [message]` / `enable` (ze-peer-cmd.yang
  containers; ze-bgp-api.yang rpcs peer-disable, peer-enable).
- Reactor: peer_admin.go (AdminShutdown type, Peer.Disable/Enable/AdminDown,
  holdAdminDown parks the run loop as PeerStateIdleHold, an inbound connection
  is answered Cease 6/5 per RFC 4486 S4), peer.go fields adminDown/adminWake
  plus NewPeer seed, peer_run.go (loop-top hold, publish-time check under p.mu,
  backoff wake), config.go parse, peer_settings.go field,
  peer_settings_apply.go hot-swap plus applyAdminShutdownChange (presence
  change only).
- Plugin interface: ReactorPeerController DisablePeer/EnablePeer
  (types_bgp.go), coordinator.go, reactor_api.go adapter, 10 mocks.
- cmd/peer: admin.go handlers, peer.go registrations plus
  errEnableTakesNoArgument, admin_test.go.
- Migration: exabgp.yang leaf shutdown, migrate.go leafFields and
  copySimpleFields to the shutdown container, migrate_serialize.go writes
  `shutdown {}`; TestMigrateShutdownHoldsPeer (red before: "unknown field in
  neighbor: shutdown").
- Bridge: bridge_neighbor.go convertDisable (shlexSplit, ExaBGP neighbor.py
  disable), convertEnable; TestNeighborDisableEnable and its Refusals test.
- Tests: reactor peer_admin_test.go, 4 tests green under -race;
  TestPeerShutdownContainerHoldsUntilEnable went RED with the run-loop checks
  overlaid away. cmd/peer, migration and bridge packages green.
- api-disable: PASS on ze (`./le test exabgp api api-disable`) and PASS on the
  pin (`--dut exabgp`).

Owed next, in order:
1. Compile check every mock package with all feature tags.
2. `./le yang glue check`, `./le doc yang-contract command-contract`, then the
   AC-41 commit.
3. Functional `test/plugin/peer-admin-shutdown.ci` through real config and
   CLI: config shutdown container holds the peer idle, `request peer peer1
   enable` brings it up, `disable "back soon"` sends Cease 6/2 with the RFC
   9003 message (hex), an inbound connection while held gets Cease 6/5. Use
   the engine-steps pattern of bgp-blackhole-honor.ci.
4. Docs: config guide (shutdown container), command reference
   (disable/enable), docs/architecture/exabgp-bridge.md,
   exabgp-migration.md, exabgp-differences.md.
5. Journal row or fix: the ExaBGP pin's teardown takes `[code] [subcode]
   [text]` but convertTeardown refuses more than one word.
6. `./le rfc check` after the commit; the spec's "Phase 7 state" line.
7. Not started: X-35 (TestOldCommands50/42, TestOldResponses50/42), X-37/AC-39
   (IPv4 route with an IPv6 next hop: extended-nexthop entry, capability and
   warning in migrate), AC-37/38, old-4.2/old-5.0 population runs.

Update, exa8 agent (budget reached): items 1 to 4 are done and committed as
72a56ddfbf "bgp: hold a peer administratively down (ExaBGP 6 disable/enable)".
- Vet with every feature tag over cmd/ze, plugin, bgp/plugins, exabgp, reactor:
  clean. `./le yang glue check` and `./le doc yang-contract command-contract`
  green (peer-disable, peer-enable listed) after `./le --update` refreshed a
  stale bin/le.
- The update/ batch mock's two methods live in update/mock_reactor_test.go;
  update_text_test.go keeps only another session's FlowSpec hunks, uncommitted.
- `test/plugin/peer-admin-shutdown.ci` (config hold reads idle-hold, enable
  brings the session up, `disable back soon` sends
  `...001F030602096261636B20736F6F6E`) and
  `test/plugin/peer-admin-shutdown-inbound.ci` (dialed connection answered
  `...0015030605`): both green, both RED with the run-loop hold overlaid away
  (idle-hold never seen; OPEN mismatch). Run one with
  `./le test bgp plugin <name>`.
- ze-peer: under `option=open:value=inspect-open-message` a NOTIFICATION in
  place of the OPEN is now judged as the first expectation
  (`internal/test/peer/peer.go` doOpenHandshake,
  `preopen_notification_test.go`); ci-format.md row updated.
- Docs: configuration.md "Holding a peer administratively down",
  command-reference.md, exabgp-bridge.md, exabgp-migration.md,
  exabgp-differences.md.

Still owed, in order:
1. `./le rfc check` over 72a56ddfbf and re-take any staled records.
2. `./le doc check verify` reports the gh-pages command-equivalents pages for
   request-peer-disable and request-peer-enable missing: the next site build
   publishes them. Its 4 missing-short-help rows are another session's
   flowspec-redirect-check.
3. Journal row or fix: the ExaBGP pin's teardown takes `[code] [subcode]
   [text]` but convertTeardown refuses more than one word (not started).
4. Spec "Phase 7 state" line; then X-35, X-37/AC-39, AC-37/38 and the
   old-4.2/old-5.0 population runs (not started).
5. Gates for the main thread: reactor `-race -count=20` on peer_admin, `./le go
   lint run` scoped to the touched packages, `./le verify worktree`.

### ExaBGP 6 Phase 7 continued (2026-10-21, Mac)

Copied from the exa9 agent's handoff (budget reached at 100 calls).

Committed: 05ccb60217 (rfc records and verdicts re-taken for 72a56ddfbf),
484eb6e6cc (ze-rib.yang ze:help on the flowspec-redirect-check container;
help-shape green), 6fafa673af (bridge teardown takes the pin's [code] [subcode]
[text]; test/weakened/6d35cd0d.md row).

Uncommitted, in progress (AC-39 / X-37):
- internal/exabgp/migration/migrate_extended_nexthop.go (new):
  addExtendedNextHopEntries, requireExtendedNextHop, peerCapability,
  extendedNextHopDisabled; RFC 8950 Section 4 quote verified against
  rfc/full/rfc8950.txt line 352.
- internal/exabgp/migration/migrate.go: calls addExtendedNextHopEntries after
  copyContainers in migrateSingleNeighbor.
- internal/exabgp/migration/migrate_test.go:
  TestMigrateIPv4RouteIPv6NextHopAddsExtendedNextHop plus the
  migratedNexthopEntries helper; red before the code (entries = []).
- Last edit not yet applied: move the `capabilityDisable` const below the
  extendedNextHopFamilies var. Then gofmt, run the migration package test.
- Owed with it: docs/architecture/exabgp-migration.md section "extended next
  hop for an IPv4 route", exabgp-differences.md if any; commit.

Main-thread decision for this run (M3): replace 6fafa673af's refusal of
`teardown <code> <subcode>` with code other than 6 by widening ze so the bridge
sends exactly what the pin sends. Then X-35, AC-37/AC-38, the old-4.2 and
old-5.0 population runs, and the spec "Phase 7 state" line.

## 2026-10-21 FlowSpec redirect-check interop and gates for b68de78958 (agent of 8385ff13)

Gates over b68de78958 (HEAD then), all green:
- reactor `-race -count=5 -timeout 40m -run 'Forward|Flush|Direct|RS|Replay'`: ok (45s).
- rs and plugin/registry `-race`: ok.
- `./le rfc check`: exit 2, 25 findings, none in rs, registry, plugin/server or
  the reactor, and no discrimination record whose producer these commits touch.
  The findings are rfc7950 config/yang records (producer-changed), SHIFTED or
  STALE FlowSpec verdicts over another session's uncommitted
  `nlri/flowspec/types_test.go` and `rfc8955_text_encode_reserved_test.go`, and
  two file-name rows (rfc4760_mp_no_next_hop_send_test.go from e7354f3351,
  rfc7313_local_resend_test.go from e21bd09703). Nothing re-taken: nothing of ours staled.
- `internal/component/plugin/server`: the 31 "failing" tests are a missing build
  tag, not a red. Bare `go test`: FAIL; with the `ze_*` tags of
  `feature-gates.txt`: ok (21s). This is the documented trap
  (`docs/contributing/running-commands.md`, "A bare `go test` is not
  `./le test unit`"). No journal row.

BLOCKER 2b landed: interop `bgp-flowspec-redirect-check-frr` (raw injector, no
lab daemon originates a 0x0800 rule; no ExaBGP sender image exists). Checker
`internal/le/interoplab/bgp/register_flowspec_redirect_check.go`; inventory row 52.
Green at b68de78958 plus the scenario; red under the `refusedoff` overlay
(FRR holds `"ecomlist":"FS:redirect IP 0x0"`, assertion 3 fails).
Traps met: FRR's plain `show bgp ipv4 flowspec json` is the generic route
printer and shows no extended communities (a first red run passed with it, so
the check was vacuous until moved to `detail json`). The docker-lab guest
builds the staged `test/interop/ze-linux` itself, so the guest-side
`env 'GOFLAGS=-overlay=/workspace/...'` of the recipe does apply.
Owed by the main thread: `./le doc check verify` (interop.md row), `./le go lint run`
scoped to `internal/le/interoplab/bgp`.
