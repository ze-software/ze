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
