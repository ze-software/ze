# Spec: rfc4301-architecture-gaps

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P0 |
| Release reason | Retain remaining IPsec architecture obligations and fragment investigation without claiming an unproven vulnerability. |
| Scope | protocol |
| Depends | `plan/release/fixes/spec-ipsec-lifetime-volume.md` (owns the Section 4.4.2.1 byte-count SAD lifetime); `plan/release/validation/spec-rfcgate-6-supported-extraction-signoff.md` (owns the `rfc4301` extraction sign-off, `rfc/extraction/rfc4301.json`, and the ledger scope set; this spec supplies the implemented obligations and the 12 held sites' evidence the walk classifies). No dependency on `plan/post-release/features/spec-ipsec-vpp-policy-interface.md`: by Thomas's ruling of 2026-11-20 ("plan/release/fixes/spec-ipsec-vpp-policy-interface.md will be done after the release") that spec owns the VPP half of decision 5a and of AC-1 (operator SPD entries on the VPP backend) after the release, and no operator can reach the VPP backend before `plan/post-release/features/spec-ipsec-dataplane-selector.md` lands |
| Phase | 2c -- D-1 to D-6 landed; AC-4 remainder, AC-8 red proofs, `.ci` promotion and interop proof in progress |
| Handoff | - |
| Updated | 2026-11-20 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Current continuation checkpoint, 2026-11-20

Every commit named here is on local `main` (read with `git log` on
2026-11-20); none is pushed. The 2026-11-19 evening checkpoint below is history.

### Thomas's decisions of 2026-11-20

- Decision 1 (c) and decision 6 (a) were already recorded (see "Owner decisions
  recorded through 2026-11-19", the 2026-11-20 paragraphs).
- New: open decision 2 is answered "refusal until done". A port-scoped operator
  BYPASS is refused at commit, not reported by a notice, until the forward-path
  stateful fragment check is done. Recorded in the Open decisions table below and
  in "Owner decisions recorded through 2026-11-19".

### Landed on 2026-11-20

| Commit | What it holds | Still owed |
|---|---|---|
| `df3221e7b8` | A MOBIKE SA survives an interface-address move | the `mobike-initiator` and `mobike-responder` interop runs |
| `40cf3cfaec`, `78d79ad79a` | K1 bit 6: the kernel maps a forwarded ICMP error past a discard; the stock-kernel return route (OUT allows at `dataplane.PriorityICMPErrorReturn`, nftables table `ze_ipsec_icmp_error_return`, doctor check `ipsec-icmp-error-return`) retired | none for bit 6 itself |
| `3aef55597f` | Bit-6 guest proof on arm64 `qemu-c860ed8b`, as root: `TestRFC4301ForwardedICMPErrorsMapUnderDiscard`, `TestRFC4301ForwardedICMPErrorUnderOperatorDiscardMapsToReturnSA`, `TestRFC4301OperatorEntryDecidesTransitTraffic`, `TestRFC4301TransitFromIKEPortIsNotBypassed` PASS. Two fixtures corrected against the product (judge-approved). Known red there, owned by bit 7: `TestRFC4301ForwardedICMPUsesPayloadPolicy` at if_id 0 | none |
| `df31eb1c63` | `test/parse/ipsec-spd-order-total.ci` and `test/parse/ipsec-spd-discard-beside-default-peer.ci` promoted after 40 of 40 stress invocations | a red run of each, as its commit body records |
| `a29420771d` | Decision 2, the refusal: a commit refuses a port-scoped `vpn ipsec policy` BYPASS on an XFRM dataplane unless its local prefix is one address this host holds. At start the entry is installed and logged at Error (decision 3), and doctor check `ipsec-spd-forward-bypass` reports it with the fix. Tests: `TestRFC4301PortScopedForwardBypassRefusedAtCommit`, `TestPortScopedForwardBypassRefusalCoversBothPortsAndUnreadableAddresses`, `TestPortScopedForwardBypassAtStartIsLoggedAndReportedByDoctor` | none: the refusal path is proven by these unit tests. Its old guest draft, `ipsec-spd-forward-bypass-refused.ci`, was deleted by owner decision (C) of 2026-11-20; the guest test is now `test/ipsec/ipsec-spd-forward-bypass-projected.ci` (Guest runs below) |
| `4663a5e15e` | K1 bit 7, a forwarded ICMP error past a transit gate. `XFRM_POLICY_ICMP` is set on the FWD gates only, not the OUT gates; by owner decision B of 2026-11-20 an in or both BYPASS's FWD decision carries it too | guest proof in Guest runs below |
| `7234fdfecc` | AC-13: the four records for `RFC7296-3.13.1-1` and `-2` | a freshness check only |
| `fbca47214a` | this checkpoint, decision 2 recorded | none |
| `1dc1323b86` | The "done" half of decision 2: `dataplane.ForwardFragmentGatherer` answers from K3's capability bit 3 (`xfrmBackend.GathersForwardFragments`); with bit 3 a port-scoped BYPASS is projected onto FWD (`spdEntryHasTransit`, `spdForwardGathers`) and the refusal of `a29420771d` does not apply. Tests: `TestPortScopedBypassForwardPathFollowsFragmentGathering` (`RFC4301-7.4-2` positive), a new phase in `TestRFC4301ForwardedFragmentsRespectOperatorDisposition`, a strengthened `dataplane.TestXFRMPortScopedBypassIsNeverStoredOnTheForwardPath` | none |
| `42edf271d9` | Ruling B: the FWD decision of an in or both operator bypass carries `XFRM_POLICY_ICMP`, so a forwarded ICMP error is judged by payload | none |
| `c90d6ddb33`, `cbd60473c5` | spec records of Thomas's bit-7 rulings and of decisions 3, 4 and 5 | none |
| `51799ad534` | Decision 5c: a catch-all refused at start is installed as far as it goes (`installUnmatchedAtStart`) and reported, not fatal; doctor findings for the interface catch-all and the exemption | none |
| `baed2a8b3d` | Decision 3: 17 Mobility Header mutants over 10 producer guards, each killed by a named test, with the control green | none |
| `8a991db40e` | Decision 4: `TestRFC4301ForgedFragmentsTakeTheReassembledDatagramsDisposition`, a netns packet test on Ze's kernel (not a strongSwan scenario: the forgery is cleartext Ze forwards) | none |
| `06bd727362` | Lint fixes in this spec's tests; the staled `RFC4301-4.4.1-4` and `RFC4301-4.4.1-7` records re-taken | none |
| `47f7ca7b55`, `88d7159e0e` | Re-judge of the five summary rows: `RFC4301-4.4.1.1-3`, `RFC4301-7.3-4`, `RFC4301-7.3-5` gain `{partial}`; `RFC4301-4.4.1-4` and `RFC4301-7.4-2` stay bare; the `RFC4301-7.4-2` tag names the forward fragment test file | none |
| `2c1ea96ed7` | Docs: the promoted SPD tests named; the fragment limit's forward route | none |
| `f804b97b80` | 19 IPsec verdicts the day's changes staled, re-judged | none |
| `d173045116` | Forwarded IPv6 fragments follow a port-scoped bypass, tagged `RFC4301-7.4-2` | none |
| `5666563229` | AC-4: refusal A extended to the OUT transit gate of an in bypass; a forwarded parameter-problem test | none |
| `adb172d967`, `0b920456f2` | Tags for the tests that prove RFC 4301 local fragments and the ICMP transit gates; the local fragment test file named | none |

### Guest runs, 2026-11-20

| Test | Where | Result |
|---|---|---|
| `test/draft/ipsec/ipsec-spd-discard-entry.ci` | `qemu-c860ed8b` | PASS |
| `test/draft/ipsec/ipsec-spd-order-readback.ci` | the guest, Ze's kernel | PASS later on 2026-11-20 (the first attempt waited on a guest build another session's L2TP edit broke). Stress blocked: "Owed next" item 16 |
| `test/ipsec/ipsec-spd-forward-bypass-projected.ci` (promoted; replaces the deleted draft `ipsec-spd-forward-bypass-refused.ci`) | the guest, Ze's kernel | PASS, 40 of 40 under load. It asserts that a port-scoped bypass commits and that the kernel holds its FWD decision and FWD gate with port 22. Red at LIVE-STAGES line 4 with `spdForwardGathers` forced false |
| `TestRFC4301ForwardedICMPPastBypassDecisionUsesPayloadPolicy` | `docker-53c57c37` | PASS; red with the flag removed from the bypass's FWD decision |

### Interop, 2026-11-20, on the bit-7 docker-host image `c88a4387bff259f1`

| Run | Result |
|---|---|
| Baselines `tunnel-df-treatment`, `spd-discard-entry`, `pad-subtree-identity`, `mobike-initiator`, `mobike-responder` | PASS |
| Red walks `break-df-clear`, `break-spd-out`, `break-spd-in`, `break-pad-true`, `break-pad-false` | red at the predicted assertions: df-clear 1 and 3; spd-out 4; spd-in 5; pad-true 7; pad-false 1 |
| Red walk `break-df-copy` | DONE later on 2026-11-20 with the outbound-only break: red at assertions 4 and 6, as predicted. History: the first break was vacuous: it also set the flag on the inbound state, and the kernel refused the SA. The outbound-only break is `break-df-copy-out.patch` in `tmp/session/2026-11-20-035940/scratch`, not yet run: the docker host's containerd store went read-only when the disk filled. The fix is `colima delete --profile ze --force` then `./le setup docker host start`, which is Thomas's call. Later on 2026-11-20 the walk was blocked only on another session's command-pipe change that does not compile (`plugin/server` `hub.go`, `plugins/mrt`) |

### Open question on `516671704b`

This spec records `TestRFC4301ForwardedICMPUsesPayloadPolicy` passing at
`516671704b`. Code reading contradicts that pass: the FWD gate existed there
(`072ef5bf1f`) and matches the error, and the FWD check is identical between the
two kernels. The untested hypothesis is `DST_NOPOLICY`. The bit-7 guest run, which
is owed, supersedes the question.

### Owner questions of 2026-11-20

| # | Question | Evidence | Answer |
|---|---|---|---|
| A | Bit 7's flag is set on the FWD gates only, not the OUT gates | an OUT gate carrying the flag would let stock `icmp_route_lookup` source Ze's own error from the far end | DECIDED 2026-11-20, confirmed: `XFRM_POLICY_ICMP` on FWD policies only, never on an OUT gate or OUT half |
| B | An `in` BYPASS's FWD decision (a template-free allow at the operator's order) can match a forwarded ICMP error by its own headers too, which the bit-7 ruling does not cover | RFC 4301 Section 5.2 step 4 requires the payload match | DECIDED 2026-11-20, "flag it too": the FWD decision of an in or both BYPASS carries the flag as well; a DISCARD's FWD decision does not. Proven by `TestRFC4301ForwardedICMPPastBypassDecisionUsesPayloadPolicy` |
| C | The old draft `ipsec-spd-forward-bypass-refused.ci` | the refusal path is proven by the `a29420771d` unit tests | DECIDED 2026-11-20: delete it (done); the guest test is `test/ipsec/ipsec-spd-forward-bypass-projected.ci` |
| 3, 4, 5 | As in the Open decisions table below | Thomas's "ok" to the recommendations, 2026-11-20 | DECIDED 2026-11-20; work in "Owed next" items 11 to 14 |
| 7 | How to deliver 5a, VPP operator entries | Ze's VPP SPD binds to no interface on any path today, so an operator entry bound to "every interface Ze's SPD is attached to" has nothing to bind to | DECIDED 2026-11-20, option (c), "Pull the post-release spec forward": `plan/release/fixes/spec-ipsec-vpp-policy-interface.md` moves into release scope, and operator entries on VPP are installed, not refused. On its open question Thomas ruled "reuse the existing IPsec WAN interface leaf": on VPP the one Ze SPD is bound to the `vpn ipsec interface` interface, resolved with `iface.ResolveVPPIndex`. Still open: whether `spec-ipsec-vpp-dual-form-proof` and `spec-ipsec-dataplane-selector` also come forward. **The pull-forward was reversed the same day** by Thomas's ruling "plan/release/fixes/spec-ipsec-vpp-policy-interface.md will be done after the release": the spec is `plan/post-release/features/spec-ipsec-vpp-policy-interface.md` again, the ESP dual-form proof and the dataplane selector returned there with it, and VPP IPsec is not a first-release feature. The design half of decision 7 stands (operator entries on VPP are installed, not refused, in the SPD bound to the WAN leaf), owned by that post-release spec. This spec no longer waits on it |

### Owed next, in order

Items marked DONE were reported done by the main thread on 2026-11-20.

1. DONE: the arm64 bit-7 kernel build.
2. DONE: the `ike/engine`, `ike/dataplane`, `cmd` and `ospf` unit tests and lint
   (`06bd727362` holds the lint fixes).
3. DONE: guest runs of the bit-7 ICMP tests, the scope tests, the fragment test
   and the draft `ipsec-spd-order-readback.ci`, on Ze's kernel `docker-53c57c37`
   (`ipsec-spd-forward-bypass-projected.ci` passed and is promoted).
4. DONE: the docker-host image rebuilt from the bit-7 kernel (`c88a4387bff259f1`).
5. DONE: six interop red walks at their predicted assertions (Interop table above).
6. DONE: the `mobike-initiator` and `mobike-responder` interop runs, PASS.
7. DONE: the `./le rfc record` lines for `RFC4301-7.4-2`.
8. DONE: the AC-13 freshness check.
9. DONE: the five rows of "Summary rows that disagree with this spec", re-judged
   in `47f7ca7b55`.
10. The independent spec review is DONE; its findings are recorded here (Goal
    Validation, AC count, checkpoint). Still owed: closure.
11. DONE (`baed2a8b3d`). Decision 3: rebuild the 13 Mobility Header mutation walks as one
    surviving-mutant check per producer guard, from the 15 untagged candidates.
12. DONE (`8a991db40e`, a netns packet test, not a peer scenario). Decision 4: retarget `bypass-fragment-forgery` to an operator port-scoped
    BYPASS on Ze's kernel in a K1/K3 lab, proving K3's reassembly defeats forged
    fragments on the forward path (`1dc1323b86` projects such a bypass).
13. MOVED OUT, not owed here. Decision 5a on VPP: the VPP backend installs no
    operator SPD entries today, and making it install each one on every interface
    Ze's SPD is attached to is AC-6 of
    `plan/post-release/features/spec-ipsec-vpp-policy-interface.md`, which binds
    the SPD to the `vpn ipsec interface` interface. Decision 7 pulled that spec
    forward; Thomas reversed the pull-forward on 2026-11-20 ("plan/release/fixes/spec-ipsec-vpp-policy-interface.md
    will be done after the release"), so the work is owed after the release, and
    no operator can reach the VPP backend before
    `plan/post-release/features/spec-ipsec-dataplane-selector.md` lands. This
    spec does not wait on it. Decision 5a on XFRM is met here.
14. DONE (`51799ad534`). Decision 5c: a catch-all install failure at boot fails the apply today
    (`installUnmatched`); change it to install what it can, log at Error, and
    report a doctor finding that states the default discard is not in force.
15. DONE: the `break-df-copy-out` walk, the outbound-only break, red at
    assertions 4 and 6.
16. Stress of `test/draft/ipsec/ipsec-spd-discard-entry.ci` and
    `test/draft/ipsec/ipsec-spd-order-readback.ci`, then promotion: blocked by
    another session's daemon start failure ("local command thanks declares
    daemon path show thanks list, which no YANG ze:command node declares").

Decision 5b implies no work here: the RFC 4301 extraction sign-off stays in
`plan/release/validation/spec-rfcgate-6-supported-extraction-signoff.md`, and
this spec supplies the evidence.

## Historical continuation checkpoint, 2026-11-19 (evening)

Every commit below is on local `main` and none is pushed. The sections after
this one are history. Where they name the reserved SPD priority band (orders
100 and below), `dataplane.PriorityIKEBypass`, `engine/bypass.go`,
`ikeBypassPolicies`, `installIKEBypass` or `installSPDPolicies`, they describe
the tree before `50b89fb51b` and `e700fb7392`. The current tree holds no
reserved band (an order and a `policy-priority` run from 1 to 2147483647, and 0
is refused), no node-wide IKE bypass (Ze's IKE exchange is exempt on Ze's own
IKE sockets), and `reconcileSPDPolicies` in place of `installSPDPolicies`.

### Status at the end of the evening session

| Item | State | Evidence | Still owed |
|---|---|---|---|
| D-1, AC-14 (failed install fails the apply; an edited entry keeps dropping) | code and tests landed | `TestRFC4301FailedOperatorEntryInstallRefusesTheApply`; `TestRFC4301DiscardHoldsAcrossAnEditedEntry` passes on amd64. `da9e704088` renamed the D-1 and D-4 packet tests `rfc4301_*` | `test/draft/ipsec/ipsec-spd-discard-entry.ci` not yet run (needs the guest), then stress and promotion |
| D-2, AC-15 (total order) | code and tests landed | `TestSPDPolicyEqualOrderOnOverlapRefused` (engine) is the recorded negative of `RFC4301-4.4.1-4`, `TestSpdOperatorOrdersOverlappingPeers` its recorded positive. `test/draft/parse/ipsec-spd-order-total.ci` passes as a draft | the draft's stress and promotion; `test/draft/ipsec/ipsec-spd-order-readback.ci` not yet run (needs the guest) |
| AC-7 (order range) | changed by owner order | `50b89fb51b`: order and `policy-priority` run 1 to 2147483647 and 0 is refused; `dataplane.PriorityIKEBypass` is deleted; `TestSPDPolicyOrderInsideBypassBandRefused` and `TestSpdOrderCannotCaptureTheIKEControlPlane` are replaced by the untagged range tests `TestSPDPolicyOrderOutsideTheRankRangeRefused` and `TestSPDRankBoundsMatchTheYANGRanges` | none for the range |
| D-3, AC-16 (operator entries decide forwarded traffic) | code and tests landed | `072ef5bf1f`; `TestRFC4301OperatorEntryDecidesTransitTraffic` passes at `0c283fffc6` and goes red when `spdTransitPolicies` is emptied (overlay break). `cb6d05616a` points the `RFC4301-7.4-2` positive test at `spdEntryPolicies`; `TestAnyPortBypassReachesTheForwardPath` is its recorded negative carrier | refusal A extended to the D-3 gates; the port-scoped BYPASS notice (open decision 2) |
| D-4, AC-17 (node-wide IKE bypass) | met | `e700fb7392` removes the node-wide UDP 500 and 4500 SPD bypass and deletes `engine/bypass.go`. Red observed at `69fc643e50` (amd64 guest, K1 kernel `docker-1562c4ae`): transit UDP from source ports 500 and 4500 left in clear under `unmatched bypass`. Green at `e700fb7392`. Passing now: `TestRFC4301TransitFromIKEPortIsNotBypassed` under both `unmatched` values, `TestRFC4301IKESocketsCarryTheirOwnBypass`, and 40 MOBIKE tests | the interop regression set of R-8, run 2026-11-19, result recorded in the commit that follows |
| Defect found today: policy-based gateway forwards nothing under `unmatched discard` | fixed | `ce5c276457`: per tunnel Child SA at if_id 0 on XFRM, a template-free FWD allow in the outbound orientation (strongSwan `install_policies_outbound`, `child_sa.c`) and a delivery OUT allow in the inbound orientation, both at the Child priority plus 1, owned in the ledger, not migrated by MOBIKE. `TestChildOutboundForwardPolicyAdmitsProtectedHostTransit`, `TestChildOutboundForwardPolicyBuiltOnlyForPolicyBasedTunnels`, `TestRFC4301DecryptedTransitReachesProtectedHostUnderDiscard` pass. The same commit fixes the D-4 test's 9-octet IKE reply, which fell below the 28-octet floor of `UDPTransport` | none. Tradeoff recorded in Known Limitations |
| D-5, AC-18 | tests green on amd64 | `TestRFC4301NonErrorICMPOutsideSelectorsIsDiscarded` passes on Ze's 7.3 kernel with the K1 series | none beyond the records it holds |
| D-6, AC-19 | tests green on amd64 | `refuseICMPCheckDiscard` is gone (`engine/icmp_overlap.go`); the packet half passes in `TestRFC4301NonErrorICMPOutsideSelectorsIsDiscarded`; `test/draft/parse/ipsec-spd-discard-beside-default-peer.ci` passes as a draft | the draft's stress and promotion |
| AC-4, forwarded ICMP error under `unmatched discard` (`RFC4301-6.2-3` forwarded case) | forwarded half green | `516671704b`: OUT allows for the mapped error types towards each mapping peer's remote selectors at `dataplane.PriorityICMPErrorReturn`, the nftables table `ze_ipsec_icmp_error_return` dropping untransformed errors in forward and output, doctor check `ipsec-icmp-error-return`. `TestRFC4301ForwardedICMPErrorsMapUnderDiscard` passes; `TestRFC4301ForwardedICMPUsesPayloadPolicy` and its XFRM interface variant pass | by the owner decision of 2026-11-20: the K1 extension (`xfrm_out_fwd_icmp` tried on `-EPERM` in the forwarded path), its feature bit, the engine's requirement of it, removal of these allows and this nft table, and the packet test of an operator out DISCARD covering a mappable forwarded error, with its non-mappable negative; a forwarded parameter-problem; a namespace test of a local error under the mapping; refusal A extended to the D-3 gates |
| AC-11 (summary) | met for the count and the prose | `e626cbdf17`: new row `RFC4301-5.2-11` for Section 5.2 step 5, `{partial}`, five records; the strongSwan prose replaced by the native engine description; `RFC4301-5.2-4` records on `TestRFC4301IKESocketsCarryTheirOwnBypass` and `TestRFC4301TransitFromIKEPortIsNotBypassed`. The Enrolment reason counts 105 rows: 96 MUST, 6 MUST NOT, 2 SHOULD, 1 MAY | the five rows of "Summary rows that disagree with this spec" still carry no annotation (read at the working tree on 2026-11-19) |
| RFC records | current | `69fc643e50` retakes the `RFC4301-4.4.1-10` positive record and reseals 25 shifted verdicts; the `RFC4301-4.4.1-7` records still verify after `installSPDPolicies` became `reconcileSPDPolicies`. `721d67d3c4` re-judges 12 stale rfc4301 verdicts with no level change, gives `RFC4301-5.2-11` its first verdict (partial), and corrects the `RFC4301-5.2-2` and `RFC4301-4.4.2.1-6` annotations. `./le rfc check` reports no rfc4301 finding after `721d67d3c4` | none |
| AC-8 (DF on the wire) | scenario PASS, red proofs owed | `22a4740af7` makes `tunnel-df-treatment` judge the wire counters before the readback, with numbered assertions 1 to 6. PASS on the Mac | break `xfrmDFFlag` for `DFClear` and expect assertions 1 and 3 red; break it for `DFCopy` and expect 4 and 6 red; image rebuilt for each |
| Interop scenarios `spd-discard-entry`, `pad-subtree-identity` | written | `22a4740af7` | run 2026-11-19, result recorded in the commit that follows; their red proofs |
| Lab tooling | fixed | `a5528d0863`: a lab boots either backend's cached runtime kernel of the same inputs | none |

### Evidence, 2026-11-19 (evening)

| Run | Where | Result |
|---|---|---|
| `ike/dataplane` and `ike/engine` at `516671704b`, as root | Ze's 7.3 kernel with the K1 series, amd64 QEMU guest, runtime `docker-1562c4ae` | all pass except `TestRFC4301OperatorEntryDecidesTransitTraffic`, whose capture was bound below `ETH_P_ALL` and could not see Ze's transmitted frames (a test defect, fixed in `0c283fffc6`) |
| `TestRFC4301OperatorEntryDecidesTransitTraffic` at `0c283fffc6` | same guest | PASS; RED with `spdTransitPolicies` emptied (overlay break) |
| `TestRFC4301TransitFromIKEPortIsNotBypassed` (both `unmatched` values), `TestRFC4301IKESocketsCarryTheirOwnBypass`, `TestRFC4301DecryptedTransitReachesProtectedHostUnderDiscard`, `TestRFC4301ForwardedICMPErrorsMapUnderDiscard`, `TestRFC4301ForwardedICMPUsesPayloadPolicy` and its XFRM interface variant, the `TestRFC4301LocalGeneratedICMP...` tests, `TestRFC4301NonErrorICMPOutsideSelectorsIsDiscarded`, the fragment tests, `TestRFC4301DiscardHoldsAcrossAnEditedEntry`, 40 MOBIKE tests | same guest | PASS |
| `./le rfc check` after `721d67d3c4` | main checkout | no rfc4301 finding |
| `tunnel-df-treatment` | Mac, Ze's docker-host image, arm64, QEMU-built K1 kernel `qemu-1562c4ae` | PASS |
| `spd-discard-entry`, `pad-subtree-identity`, `child-rekey`, `responder-ike-rekey`, `natt-tunnel-inner-checksum`, `real-nat-transport-rekey-after-ike-rekey`, `mobike-initiator` | same Mac lab | run 2026-11-19, result recorded in the commit that follows |
| `test/draft/parse/ipsec-spd-order-total.ci`, `test/draft/parse/ipsec-spd-discard-beside-default-peer.ci` | main checkout | pass as drafts; stress and promotion owed |
| `test/draft/ipsec/ipsec-spd-discard-entry.ci`, `test/draft/ipsec/ipsec-spd-order-readback.ci` | not run | they need the guest |

### Open decisions for Thomas

None of these is answered by the RFC text, an agreed design or
`ai/META-DECISIONS.md`.

| # | Decision | Options | Recommendation |
|---|---|---|---|
| 1 | Mapping a forwarded ICMP error that an operator out or both DISCARD covers | DECIDED 2026-11-20, option (c), the kernel route; recorded in "Owner decisions recorded through 2026-11-19" | none left |
| 2 | A commit-time notice for a port-scoped operator BYPASS, which is not projected onto the forward path until K3's capability bit 3 exists | DECIDED 2026-11-20, "refusal until done": the commit refuses it, not a notice, until the forward-path stateful fragment check is done (`a29420771d`); recorded in "Owner decisions recorded through 2026-11-19" | none left |
| 3 | The 13 phase-4 Mobility Header mutation walks: the list was lost | DECIDED 2026-11-20: rebuild them as one surviving-mutant check per producer guard, from the 15 untagged candidate tests | none left |
| 4 | The premise of `bypass-fragment-forgery` after D-4: no node-wide IKE bypass exists to forge fragments against | DECIDED 2026-11-20: retarget it to an operator port-scoped BYPASS on Ze's kernel (K1/K3 lab); it proves K3's reassembly defeats forged fragments on the forward path, now that `1dc1323b86` projects such a bypass | none left |
| 5 | Carried from the handover: the interface binding of VPP SPD entries; the RFC 4301 sign-off scope in `plan/release/validation/spec-rfcgate-6-supported-extraction-signoff.md`; whether decision 3 covers a catch-all install failure at boot | DECIDED 2026-11-20: (5a) an operator SPD entry on VPP binds to every interface Ze's SPD is attached to, the node-wide meaning it has on XFRM; (5b) the sign-off stays in rfcgate-6's scope, this spec supplies the evidence; (5c) decision 3 covers a catch-all install failure at boot, and the doctor finding states that the default discard is not in force | none left |
| 7 | How 5a is delivered: Ze's VPP SPD binds to no interface on any path, so "every interface Ze's SPD is attached to" is empty today | DECIDED 2026-11-20, option (c), "Pull the post-release spec forward": the VPP policy-interface spec owns the delivery (its AC-6); the SPD binds to the `vpn ipsec interface` interface ("reuse the existing IPsec WAN interface leaf"). Options (a), install into an unbound SPD, and (b), refuse operator entries on VPP, are set aside. The pull-forward into release scope was REVERSED the same day by Thomas's ruling "plan/release/fixes/spec-ipsec-vpp-policy-interface.md will be done after the release": the spec is `plan/post-release/features/spec-ipsec-vpp-policy-interface.md`, and this spec no longer depends on it | none left |

### Tooling defects found

- The server worktree path is too long for docker-lab's forwarded socket
  (121 bytes against a 103-byte limit), so a cross-machine worktree cannot
  run a lab.
- A Bash hook refuses every command that names `docs/` (and, met while
  writing this checkpoint, `rfc/`) while another session's RFC summary does
  not parse.

## Historical continuation checkpoint, 2026-11-17

The dual-form product fix is closed. This architecture spec remains open for
broader RFC proof and the forwarded/mapped ICMP-return work. Closing the product
fix does not promote `RFC4301-5.2-2` from `{partial}` or complete this spec's
other requirements and final review.

Commit `6f59a38b3d` preserves the product implementation and acceptance record;
`73a3e251e7` closes `spec-ipsec-dual-form-receive-spd-bypass`. Its AC-4 passed
the required normal Ze-kernel tests under the owner's one-off product acceptance
decision. This spec's AC-4 concerns forwarded ICMP and remains a separate
obligation, including the mapped-return assertion recorded below.

The explicitly authorized fixture correction is committed in `c6eda91b9f`.
The final Claude batch passed all eight top-level tests without FAIL/SKIP,
including sixteen context rows, their following authenticated whole-packet
controls and exact selector-negative counters `(1,0,0)`. Production noninitial
fragment permission remains false. The fixture permission and paired negative
were independently approved with their assertions preserved. The earlier
fixture hold and its owner question are settled.

All 79 legacy renewals and their exact-key freshness checks are complete.
The two existing Go-cut renewals affected by the fixture correction also passed
clean and failed at their intended producer halt, with stopped VMs and native
exit zero. Their four selected records are current, and the independently
refreshed partial `RFC4301-5.2-2` audit has nine current covers. Both selected
freshness reports have zero stale/unproven records. The two producer-halt
renewals retain their bounded Go-dependency proof meaning. Final scoped engine
lint reported zero issues in all three flavors, and the product's independent Review Gate was CLEAN
over 100 code files.

The last accepted normal Claude kernel was `d44390c8`, with matching image
`3a6ccfb1c29c1b23`. These identify accepted test inputs and carry no cache
reservation or claim that the cached files still exist. The full hashes,
receipts and earlier B1 regressions remain in the committed product record.
The durable contract is [the Child SA and XFRM architecture](../../../docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md).

General kernel-proof tooling remains separately owned by
`plan/post-release/kernel/spec-rfc-kernel-patch-discrimination-route.md` and
unapproved. The normal-fixture authorization grants no private fault-kernel
preparation or use. This spec retains its existing lifetime-volume and
extraction/sign-off dependencies, its broader RFC discrimination work and its
mapped ICMP-return obligation. Its remaining ACs and final review need their
own evidence reconciliation against the completed product work.

AC-21 evidence, 2026-11-19. AC-21 carries owner decision 2 (a): Ze sets the SA
direction at install. Its evidence is complete. Thomas closed AC-21 on
2026-11-19. The main session observed all of the following on
2026-11-19.

- Commits. `3977cef525` holds the code, the tests, the docs and the AC-21 row.
  Its RFC-tagged test changes were approved by `ze-test-judge`:
  `TestRFC4301SADItemAntiReplayWindowIsInstalled`,
  `TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp` and
  `TestChildSAReplayWindowDefault`. `c068931843` gives the loopback ICMP test
  state the zero `Dir`, because the kernel had refused to send through a state
  marked inbound; the judge approved
  `TestICMPErrorPayloadHeaderCheckedOnLocalDelivery`. `20b70674db` and
  `4f2e4d3da0` hold 68 discrimination records: 10 for the re-pointed or new
  tagged units, the new `RFC4303-3.4.3-1` and `RFC4303-3.4.3-4` records among
  them, and 58 renewals of records whose producer AC-21 changed, across
  rfc4301, rfc4302, rfc4303, rfc4552, rfc7296 and rfc4106.
- Mac (darwin arm64). The unit tests of `internal/plugins/ospf`, `ike/engine`
  and `ike/cmd` pass. The `ike/dataplane` VPP tests (the `ze_vpp` tag) pass,
  69 of them, `TestVPPSAFlagsAntiReplayPerDirection` included. Scoped lint of
  `ike/dataplane` and `ike/engine`, in the host and the GOOS=linux integration
  flavours, reports zero issues. The Linux discrimination records ran in Ze's
  7.3.0-ze arm64 guest kernel.
- claude server (Linux x86_64), in detached worktrees. On kernel 7.2.9-ze,
  `ike/dataplane` passes in full at `c068931843`, with
  `TestXFRMReadbackReportsTheDirectionZeInstalled`,
  `TestXFRMStateCarriesTheSADirection`,
  `TestXFRMStateReplayWindowOnlyOnTheReceivingSA` and
  `TestXFRMStateRefusesADirectionNoStateCarries` passing by name, and
  `internal/plugins/ospf` passes. On 7.2.9, `ike/engine` had 32 failures, all
  refused by the kernel state-scope receipt both with and without AC-21. After
  the server moved to Ze's 7.3.0-ze kernel, `ike/engine` passes in full. On 7.3,
  in a Ze amd64 QEMU guest at `4f2e4d3da0`,
  `test/ipsec/ipsec-sa-options-kernel.ci` and
  `test/ipsec/ipsec-show-dataplane-kernel.ci` both pass, each 1 of 1 ran and 0
  skipped.
- VPP correction. An earlier draft claimed VPP needs `USE_ANTI_REPLAY` on the
  outbound SA. That was wrong. VPP v26.06 `esp_seq_advance` refuses a cycled
  counter on every outbound SA, so the flag follows the window.

## Historical continuation receipts, 2026-11-17

The dated receipts and status tables below preserve what was known at each
checkpoint. Their unrun populations, open product AC-4, fixture holds and
requests for fixture approval are superseded by the current checkpoint above.
They neither reopen the settled normal-fixture question nor authorize private
fault work. Historical defect findings retain their evidence and ownership;
this status update does not independently close them.

The dual-form spec's **Current continuation checkpoint, 2026-11-17** is a
historical receipt record in Git; its dated pending-work labels do not describe
current product closure. The current durable contract is the architecture page
linked above. Those receipts superseded the unrun populations, source-only
lifetime and
79-unexecuted progress statements in the dated checkpoints below. Normal guard
implementation landed in `bde4a49a63473a875f99beb9754650fcd7521387`. The separate
d87/d2eb kernel and matching 43a1 image run accepted all three raw tests in both
families plus the original eight packet tests: 11 top-level PASS, no FAIL/SKIP, zero
exits and stopped VM. Scoped host/Linux engine lint passed.

The earlier lifetime local-counter failure and missing-field RED in run 21799 on the
preceding normal kernel retain their observer diagnosis/TDD scope; neither establishes a
packet defect or native RFC proof. Their receipts are in the dual-form checkpoint.

Normal counter build 43301 subsequently succeeded with C patch `e3ecc903`; actual B1
kernel SHA256 `f82654a4d1458db3094449274348cf5268e0f09e912776a95b52263f27816536` and
matching image `d9efda5bec4fd7f7` passed capability and counter tests in first boot run
36055. All eight input and Colima holds were released after its VM stopped.
Independently approved helper source `08e8a270`, with negative rejection and unchanged
lifetime assertions, passed formatting 84982 and Linux compilation 96345. Current binary
`b1a1994c` passed exact lifetime run 55413 in 7.08 seconds, covering both families and
arrival orders. Separate run 59439 passed the three raw tests and eight packet tests:
eleven top-level PASS results on the B1/f826 kernel and matching d9 image. Both runs had
no skips, all zero exits, interrupted false and stopped VMs. Scoped engine lint 60545
passed host and Linux integration with zero issues. Owning docs are `3601b075`. The
counter and helper phase landed in `84d13c03ee526447bc06646b4fb652fb35dd9bd1`. The
dual-form checkpoint names the exact logs; these normal regressions do not renew RFC
proofs. Native renewal 19280 recorded only manifest order 58, the negative SPD-order
unit, after its clean PASS and own producer-halt FAIL. It proves function dependency for
a partial claim, not C enforcement or the whole requirement. Runs 94797, 40175 and 17887
also renewed host-positive orders 9, 15 and 44 after exact selected clean PASS and
native installChildSA halt FAIL; only their selected producer seals changed to
`17aa3b0ab62f6479`. These are partial-claim dependency proofs; the dual-form checkpoint
names the logs. The original 83 historical rows remain unchanged; the latest ended
final eight-revert batch leaves all 79 current renewals RECORDED and zero pending.
Eight scoped reports verify all 79 exact roster keys, with zero missing/stale/wrong-seal/no-break matches; this completes the bounded legacy renewal/fingerprint debt, without whole-RFC or C credit. Earlier native selected reports verify all four historical records against
current source/fingerprints, without a new packet observation or whole-ID/full-RFC
credit; the CE ID retains six other unproven tags. Both safety holds, the pending safety-action owner question,
compliant-fragment RED, full AC-4 and separate mapped-return gap remain open. The P3
C-proof proposal has independent design reviews CLEAR, including the observer manifest
in draft `2438aa68`, but remains UNAPPROVED with A-R2 and A-R4 unvalidated; its prose
landed in `467baad97515`. Its separate owner design question about prepared pairs versus
recorder builds is PENDING. Seven later RFC792 rows stay post-release, with no tooling
implementation or proof credit. No push was made by this session.

The historical `spec-ipsec-dual-form-receive-spd-bypass` checkpoint
owns the completed host and eight guest batches' native evidence and proof
limits. It also records report repair `3d6aa6c86f`, AC-30's existing lint
limitation and four current VERIFIED historical records; those bounded
conclusions and pending approvals apply here.

## Historical local ICMP proof receipt, 2026-11-16

The dual-form spec's AC-1 now has all seven native runs of
`icmp-error-payload-check`, with independent final review CLEAR. Proof (c)
ran clean PASS 10s, ICMP-flag omission packet FAIL 8s, then clean PASS 10s.
Proof (d) continued with policy-only PASS 10s, state-only PASS 10s, combined
source-widening packet FAIL 8s, then final clean PASS 10s. The Observed column
below records both intended packet failures. All runs used the current
IPv4 single-pair tunnel, unchanged production XFRM source and reviewed private
inputs; every VM stopped without interruption or leftover resources.

This receipt supersedes the earlier local-ICMP blocked/RED progress statements.
Their dated evidence remains historical. It does not settle forwarded ICMP,
the other phase-3 defects, general per-SA or multiple-pair scope, protected
inner-fragment isolation or AC-4's full requirement.

## Historical full kernel route checkpoint, 2026-11-16

The owner selected completion of all three points: exact per-SA permissions,
fragment-context isolation and fragment port-rule enforcement. Current source
supersedes the dated descriptions below that call K3 unenrolled or the engine
scope writer absent. Those historical paths and observations stay unchanged.

The reviewed transport-boundary correction is activated as
`gokrazy/kernel/patches/0005-xfrm-fragment-guard.patch`, following K1/K2 in
`patches/series`; `runtime.config` and `runtime.require` enable the guard.
The old proposal remains only until its removal through the native migration
commit. It must not be deleted manually. Both core and lifecycle reviews are
CLEAR. The new direct transport packet probe first failed on the old kernel
for direct/Destination Options initial and tail fragments while its whole,
atomic, ciphertext and nested controls passed. That is pre-fix RED evidence.

`engine/child_scope.go` now derives exact immutable inbound/outbound scopes
before state publication, with canonical transport addresses, a 64-record
bound and noninitial permission false. Its tests went RED then GREEN; the
full affected host packages passed before the later doctor addition. AC-26's
fresh runtime scope diagnostic also went RED then GREEN and has independent
CLEAR review. Actual old/new configured-daemon probes subsequently verified the
partial 0x7/0xf finding and its absence on the full profile with zero SAs.
All four doctor CIs passed; the external-removal case completed on the newer
cold runtime profile, as qualified below.
The bounded approved mechanical cleanup later landed in `a62375a383`, with
its scoped host/Linux verification and lint qualification recorded below.

Main's cold `ipsec-k123-kernel-guest` job completed with exit 0, building the
native Linux 7.3 arm64 kernel with modules and matching own-system image
`47a4c8b74c98c558`. Kernel SHA256 is
`9740ef22ce9ad40cde536908394d33a4d71dede15955113c15c839fe11f62cb9`.
The complete 0xf receipt test passed in 0.01s without skips; all exits zero,
VM stopped, interrupted false (`job-ipsec-k123-kernel-guest-f67febab.log`).
The same qualifying-RED transport test binary then passed the four direct
transport fragment cases and whole/atomic/ciphertext controls in 4.92s;
the nested transport/tunnel control passed in 0.00s
(`job-ipsec-transport-boundary-new-kernel-fcb2493d.log`). Both new-kernel tests
had no skips and all zero exits on the matching pair.

The first 9740/image47a4 profile also passed eleven scope-population tests
without skips, including all eight ANY rows and K1 readback/multipair/rekey/
bounds/clear-forward. K1's private port cut independently discriminated four
wrong-SA whole-packet tokens with eight controls; NONINITIAL stays false but
effective fragment permission broadens with the port widening, so this is
not an isolated fragment discriminator. IPv6 raw-port acceptance passed all
forms/phases; its old-kernel RED stops at permitted WHOLE send EPERM. Ten PMTU
tests passed. These observations remain bounded to that pre-style profile.

The private fault kernel is unbuilt/unrun. A foreign runtime-container CFS update changed
the default recipe to profile 88c98f02; the fourth external-removal doctor CI
completed its cold build and passed in 4.0s
(`job-ipsec-doctor-removal-external-k123-7fad9c53.log`). That built kernel SHA256 is
`89a57fd6746aa444a1a453feab7c0eece000075f00276fb545cb62312038beb2`, at
`/Users/thomas/.cache/ze/runtime-kernel/7.3-runtime-arm64-runtime-docker-88c98f02-5d7f6651/vmlinuz`;
matching image is `a3933863bf7976cd`. The direct complete-receipt test passed in
0.01s (`job-ipsec-current-profile-complete-caps-d4265cb4.log`). Both selections
had no skips, all zero exits, interrupted false and stopped VMs. Current
runtime.config SHA256 is
`2cf19c2ccdad059fd27d4e1bb9b8a42725f7c285865de13d43db0a9d032172d2`;
runtime.require is `b56687bcd5990d39d3a23dd86b48a3b52fb5cea7051e74470a80bd0b6d83e136`.
Automatic review rejected the later composite scope-key/transport-output input
preparation for possible cybersecurity risk. Read-only inspection found partial
private 0004 scope-key and 0005 transport-output bypass edits in the existing
`scratch/k2-fragment-proof/inputs/patches/`. They are unbuilt/unexecuted: do not
build, use, retry, rephrase or reassign this rejected preparation, and do not use
the earlier pure K2 input as an alternate retry. Retain only this existing blocked
artifact through a safety decision or approved removal, subject to ownership,
reference and active-process checks. That rejected preparation changed no
production patches; the separate normal repair below is authorized. Approved
lint repairs preserve all assertions/packet bytes/tags/claims. Post-style host
engine/dataplane/diagnostic packages passed in 123.593s/2.521s/1.291s
(`job-ipsec-post-style-host-packages-70092bc3.log`, inner `job-unit-pkg-0d6c7694.log`).
Main observed successful Linux engine/dataplane builds `b93836ee`/`668c4a79`;
their silent logs do not alone prove exit status. The bounded nine-path AC-27
cleanup landed in `a62375a383` (native script session 32163 exit 0), carrying
the ordinary `engine.espPolNoPols` approval; two other RFC approval rows remain
unspent. Main verified the commit and clean targeted status. Scoped lint left
only the separately owned diagnostic formatting finding, whose owner's retry
passed on host and Linux; capability/compile-out stages had no findings. No
wider lint-green or spec closure is claimed. Other outstanding checks, the
79 original renewals and current freshness of the four prior renewals remain
owed after the writer change. Keep
full bits 0–3 admission and AC-4 unchanged; no full three-point completion is claimed.

The post-style Linux dataplane population passed 31 top-level tests without
skips (`job-ipsec-post-style-dataplane-linux-units-3641106f.log`), covering exact
scope capability/parameters/wire, SPD recording, ten forwarding-default cases
and full receipt. Binary `bdaf3541` ran on 89a/a393 with zero check/command/lab
exits, interrupted false and stopped VM. These bounded units do not validate
the new C continuation repair.

The reached helper population had ten PASS and six FAIL across 16 tests
(`job-ipsec-style-reached-linux-population-a644e963.log`). Five cleartext/forwarded
ICMP failures used `sadSelStat`'s wrong process-namespace counter source.
The independently CLEAR observer correction `fe6c7813`, with 16 native tagged
consumer approvals in the existing e729 ledger, is applied and read back as
observer-intermediate source hash `711a26fd`; a later authorized comment-only
header correction yields current `cfeea450`, without function/tag/assertion
changes. Main observed formatter `34367ae3` and full-manifest Linux
integration/zetest compile `61adff27` exit 0, with unchanged source. Fresh
`engine-counter.test` (`880a1f6d`) passed all five former counter failures
(`job-ipsec-thread-counter-five-linux-39fb9b9a.log`): cleartext 5.85s and four
forwarded-ICMP cases 0.18/0.20/0.20/0.24s. The 16 tagged consumers had 14 PASS
and two FAIL (`job-ipsec-thread-counter-tagged-linux-f6906673.log`): known
compliant-fragment delivery failed 3.30s; ordered inbound admit passed all
packet checks before cleanup policy-delete ENOENT at line 53, 1.01s.
Noncompliant fragments passed with exact counter deltas (1,0,0). Both guests
used historical 89a/a393 without skips and stopped; five-test exits all zero,
tagged check/lab zero and command one. Closed native handle 74728 has no recovered
outer exit. The independently approved full-selector cleanup and counter repair
are now committed in `714970f5b0`, exactly two files `cfeea450`/`8b36277e`, both
clean in targeted status. Main's native script exited 0 and carried five
RFC/helper trailers; other approvals remain unconsumed. No push by this session.
External CI-main `job-ci-ordered-cleanup-linux-52ac831c.log` passes both ordered
tests 11.26/0.51s, packet vectors intact, no skips, all guest exits zero and
stopped 89a/a393; it is not our runtime execution. The compliant-fragment contract
remains red/safety-blocked. Binary `880a1f6d` predates cleanup and supplies no
fresh-current-binary claim; now the counter commit has landed, main owes
ownership/reference/process-checked cleanup of superseded artifacts while keeping
minimum unresolved evidence.
The dual-form spec
now owns this small shared instrumentation repair in Task/AC-28, preserving
all counter/delivery assertions, inputs, tags and unrelated contracts.
Main reports the exact peer-form source-classification row added and reviewed,
with existing TUN/reqid rows verified. CI owns coordinated registry publication;
actual commit/receipts remain pending, with no runtime change.

The mapped ICMP return's protected=0/clear=0 is independent of that observer.
The isolated case failed again in 12.05s (`job-ipsec-mapped-return-isolated-e9eb39b7.log`),
on 89a/a393 without skips, command one/check zero/lab zero, no interruption and
stopped VM. This remains the existing phase-3 `LL-XFRM-ICMP-TRANSIT` obligation,
not a newly established C regression from style changes. Keep its exact wire
assertion and phase-3 ownership; local generated-return controls passed.

The seven-test post-style packet matrix on 89a/88c and image a393, binary
`2285ba7c`, had five PASS and two FAIL (`e4293794`). IPv4 ANY, IPv6 raw port
policies, nested transport/tunnel and both transport-output families passed.
IPv6 ANY lost four same-SA Destination Options rows, and transport receive lost
its initial whole control. Isolated receive passed 5.16s (`b6c76ed5`). Isolated
ANY repeated only those four failures in 8.68s (`324011f1`), with eight correct
base rows, authenticated counts and whole controls. The same-process pair had
ANY FAIL 10.19s and receive PASS 2.04s (`a015d255`). All guests stopped without
skips or interruption; check/lab exits were zero and command exits reflected
their assertions. The one batch receive loss remains unlocalized, with no
claimed repair or causal cross-test leak.

The repeatable normal-kernel Options failure was traced to
`xfrm6_fragment_prerouting`'s stale receive cursor after gathering. The normal
repair reconstructs it from the retained first fragment and processed
Hop-by-Hop state, leaving LOCAL_OUT unchanged. Active 0005 SHA256 is now
`80cf2492bac5b88c302a915fc0fa4b07986add231ad777dcc222202827b36a00`;
independent C source review is CLEAR. The additive first-fragment Hop-by-Hop
regression also has independent CLEAR review and passed its baseline in 3.30s
(`job-ipsec-hop-options-old-normal-79f077b6.log`): all six both/initial-only/tail-only
Hop-by-Hop arrival-order cases, whole-chain/following controls and exact SA counts.
No skips, zero check/command/lab exits, interrupted false and stopped VM on
89a/a393, binary `127cef6f`. Main observed formatting `09ed7233` and test build
`ecdb71b4` PASS. This is preservation evidence, not mutation proof or repaired-C
acceptance. Native job 72568 (`job-ipsec-normal-cursor-kernel-6fc0be2f.log`) ended
exit 1: kernel/modules compiled and installed, then host
`enforceKernelRequirements` rejected missing built-in `CONFIG_QFMT_V2` after the
worker's `done: /out/vmlinuz`. Shared base config/require and `kernelreq.go`
changed at 17:26:56 during the 17:04:51 build, in `faa6297d78`. The worker checks
live requirements before make; the host rereads them after build. Base hashes
`2bef4e61`/`57692fbb` became `be2894ce`/`6990de48`, while runtime `2cf19c2c`/
`b56687bc` and patch `80cf2492` stayed unchanged. The quota/QFMT dependency is
valid; no requirement weakening is justified. Native cleanup removed rejected
`tmp/kernel/runtime-build-2183217670`, target cache 265a is absent, and no
recoverable/published repaired kernel or image resulted from that attempt.
The original Options matrix supplies natural production RED on historical
pre-repair 89a/88c.

Two-job retry `9c693ee4`/session 82844 completed with exit 0 and published
cache 6a87, kernel SHA256
`1b3b839f751e6ab9f69a896d15847c07c9f8b7e2a24f07f934cf7fb029076ab3`.
Read-back QUOTA/QFMT_V2/XFRM_FRAGMENT_GUARD are built in, modules `7.3.0-ze`
and kernel.version identifies Linux 7.3 runtime/arm64/Docker. Matching image
`913df5db17ebb20a` first boot (`c6c943c2`) passed complete receipt 0.01s,
original IPv6 ANY 16-row matrix 6.68s (four former Options positives now true,
mixed-SA rows false), and HBH six cases 0.15s with controls/counts. Reached
eight-test matrix (`4e681aeb`) then passed all eight using engine `880a1f6d` and
dataplane `bdaf3541`. Both receipts have no skips, all guest exits zero,
interrupted false and stopped VMs on that pair. This accepts the cursor repair,
not full three-point conformance or AC-4. The original seven-test population
then passed five times in one process: 35 top-level PASS results, no FAIL or
SKIP, on the same accepted pair (`c25ff001`, main session 13208 exit 0).
Count 5, shuffle off, the original eight-minute test timeout and every packet
deadline/assertion were unchanged; check/command/lab exits were zero,
interrupted false and VM stopped. The historical RX loss remains unlocalized;
this stability receipt does not establish its cause or a fix. The 79 renewals,
four records' unknown current freshness, both safety blockers and phase-3
mapped-return gap remain.

The ordinary raw-authority regression is now formatted (session 6402 exit 0,
`e9d970e8`) and built (session 80197 exit 0, silent `af85e020` log, terminal
status supplied by main), with source `71d4` unchanged. Static Linux/arm64
binary SHA256 is `b73f7299f689254045b7fb67fd32519e5bd29c5e9f2efa0ce738e562348a5145`.
Its first guest run, session 85260 exit 1 (`be4fca5b`), failed both IPv4
BLOCK-namespace permitted whole controls with `EPERM`; all three IPv6 subtests
and IPv4 lifetime passed. No SKIP; check 0/command 1/lab 0, interrupted false,
stopped VM on accepted kernel `1b3b839f`/matching image `913df5db17ebb20a`.

Four diagnostic lenses cleared fixture, initialization and encoding. Main's
canonical Linux 7.3 read established that `ip_route_output_key` performs XFRM
lookup through `ip_route_output_flow` with a null socket. Both intended
ordinary IPv4 route stages therefore classified too early. The normal repair
changes only those two calls, in `raw_sendmsg` and `xfrm_fragment_route`, to
`__ip_route_output_key`. Final captured-authority lookup and all guards stay
unchanged. Active 0005 SHA256 is now
`235e022ad1b9b1725db8e62852e955a7a3109dc35c2dc6b3492ef7d2cbf43854`, with
independent review CLEAR and targeted runtime acceptance as recorded below.
Resolver session 72041 passed (`a5417483`), selecting d87a6908 with vmlinuz
absent before launch. All eight roots acknowledged input and Colima compiler
holds before main's normal two-job build 29078 started, label
`ipsec-ipv4-raw-route-kernel`, PID/PGID 23658 (`9c693ee4`). It completed exit 0
and published actual kernel SHA256
`d2eb6a14cd07c913deb1a0e9f7ff667230982b64c597ebf8fbee8bc3f80d8f65`.
Firstboot 3966 exited 0 (`631e2522`), building and booting matching image
`43a1d1b2ecccc59c`, module `7.3.0-ze`. Complete receipt passed 0.01s; unchanged
`b73f` PolicyAuthority/ipv4 passed 1.24s and RouteAuthority/ipv4 passed 0.94s.
No SKIP; check/command/lab 0, interrupted false, stopped VM. This accepts the
two-call C fix for the former whole-output failures. Main released input and
Colima compiler holds to all eight roots after actual readback. Full three raw
tests, existing eight-test matrix and new SA-lifetime regression remain unrun
on this pair. Older eight-test/35-run passes retain their bounded source/kernel
identities; targeted acceptance does not close AC-29 or full AC-4.

The metadata preparer's stop is not a new owner policy: main may separately
run the existing 13 ordinary host reverts and 11 existing host mutants after
its phase boundary. Both actual safety rejections stay in force. The 79
renewals/four unknown freshness receipts remain unchanged. The 55 guest commands
still target actual d87a6908 and remain UNEXECUTED; 6a87
observations keep their historical kernel/source identities.
The owner question about the two rejected tasks remains PENDING, not approval.
Native C-discrimination capability is absent; its existing P3/post-release
tooling design needs REWORK and owner design admission before implementation
or C-proof credit. Neither an escape nor a Go dependency proof substitutes.

The dual-form spec also records the ordinary
`TestFragmentSAReplacementKeepsQueuedIdentity` source in
`engine/fragment_sa_lifetime_integration_linux_test.go`, under existing AC-4
queue lifetime. Source `d030` is independently CLEAR but unformatted/unbuilt/
unrun. Its same-public-identity/key replacement cases retain old totals 2/6,
first request increment +1, then B counts 0/1/2/4 in both families/orders.
These are assertions, not runtime results, and add no RFC claim.

The earlier space blocker was resolved when all roots acknowledged a quiet period before
main's supported coordinated cleaner completed with exit 0, freeing 67.4G of
checkout Go cache and 0.4G of lint cache, with 80.1G free
(`scratch/ipsec-coordinated-cache-clean.log`). Valid patched GOROOTs, the built
pre-repair 88c kernel and a393 image remain preserved. Main released all roots
afterward.
The private fault-preparation safety blocker remains unchanged; no fault kernel
has built or executed. The existing state retains the minimum cleaner receipt
through the related phase commit, then reference/ownership cleanup. Its 80.1G
receipt is historical. Second supported cleanup
(`scratch/ipsec-coordinated-cache-clean-second.log`) reports 55.0G checkout and
0.3G lint freed, with 74.9G free then. Execution 72928 was already closed;
its exit code was not recovered and none is inferred. Main sent RELEASE to all
eight roots. Current-floor host-driver construction passed with code 0 on
Darwin arm64/CGO disabled (`job-ipsec-current-floor-host-driver-ae60c218.log`).
The owner corrected a temporary unrelated BGP compilation blocker; main's fresh
admission report passed with one of four slots occupied, no waiters, 25.6GiB
available and 52.1GiB disk free. Normal kernel session 81417 subsequently failed
exit 1 (`job-ipsec-current-floor-cursor-kernel-799a7dd1.log`): gcc `cc1` was killed
at Intel `ice_common.o`, then make exited 2. Earlier guard/quota object
compilation supplies no completed/published 6a87 kernel or image. Fresh Docker
info reports 2 CPUs and 2,054,840,320 bytes of memory (about 1.9GiB); the host
figures were not that builder's budget. Four jobs were too aggressive. Builder-VM
dmesg confirms OOM killed `cc1` in this build's owned container. The next
normal retry must use two jobs, as the earlier worker compilation did. The old
hold was released; root job 82844/PID 75336 completed exit 0 as native key
`9c693ee4`. Its `job-ipsec-two-job-cursor-kernel-9c693ee4.log` ends with ready
cache 6a87; accepted kernel/image receipts are recorded above. All eight roots
received RELEASE of input and Colima compiler holds. Main's exact-ID
container listing succeeded and was empty. Main verified
`tmp/kernel/runtime-build-2858380835` was empty and removed it with `rmdir`, exit 0.
The accepted 6a87 build's patch/base hashes were
`80cf2492`/`be2894ce`/`6990de48`; unique intervening writer attribution for the
earlier failed attempt remains unconfirmed. The later raw-route source repair
is recorded above and is not yet built.

The existing fragment-context fixture correction remains unresolved: automated
review stopped the later concrete patch-preparation task, no patch or approval
exists, and the test is unchanged. The independent judge assessed design only;
it did not approve a nonexistent diff. Do not retry that rejected task. This
testing blocker does not give runtime credit to the new source or relax the
full three-point goal. The 79 other renewals and unrelated owner decisions remain open.

## Historical correction against the tree, 2026-11-14

Owner instruction: "the code changed, so look at it again and update the spec
to be correct." Every statement in this section was read at its producing
function at `9af281ff33`. The dated sections below keep their history. Where
one of them disagreed with this section, this section held at that checkpoint.
The current continuation checkpoint above now governs product closure and
fixture authorization.

**Kernel statements are recorded claims, not re-read.** The Linux 7.3 tree the
2026-11-12 verdicts cite (`tmp/session/2026-11-09-222726/scratch/kernelsrc73/`)
is gone, and no kernel source is in the checkout (searched `tmp/`, `gokrazy/`
and the user cache for `xfrm_policy.c`). A kernel statement here comes from a
`verified` entry of `docs/architecture/linux-limitations.md` (Checked Linux 7.3,
2026-11-10), from the upstream context lines of a tracked kernel patch, or from
a comment in Ze's code. Each one names the netns packet test that settles it.

### Historical phase status at the 2026-11-14 read

"Code landed" means the producer and its unit tests exist. A phase is complete
only with the functional and interop tests its plan names, and none below is.

| Phase | The tree holds | The tree does not hold | State |
|---|---|---|---|
| 2a, Section 7.4 DISCARD | `dataplane.SPActionDiscard`, `xfrmPolicyAction`, `vppSPDAction`, `parseSPDPolicy`, `installSPDPolicies`. Units: `TestDiscardPolicyReachesTheBackend`, `TestDiscardPolicyReachesTheKernelAsBlock`, `TestXfrmPolicyActionRefusesAnUnknownDisposition`, `TestVPPSPDActionCarriesEveryDisposition`, `TestSPDPolicyCarriesTheDiscardDisposition`. Packet probes written, untagged and not yet observed on Ze's kernel: `TestRFC4301CleartextFragmentsRespectOperatorDisposition`, `TestRFC4301ForwardedFragmentsRespectOperatorDisposition` | interop `spd-discard-entry`; `test/ipsec/ipsec-spd-discard-entry.ci` | code landed; defects D-1, D-3 and D-6 open; named functional and interop tests absent |
| 2b, Section 7.4 stateful fragment checking | kernel patches `0003-xfrm-state-scope.patch` and `0004-ip-fragment-xfrm-context.patch` in `gokrazy/kernel/patches/series`; the proposal `gokrazy/kernel/proposals/xfrm-fragment-reassembly.patch` outside the series; `dataplane.SAStateScope`, `xfrmStateScopeFromParams`, `checkXFRMStateScopeCapabilities`; probes `TestRFC4301FragmentReassemblyKeepsProtectionContextsSeparate`, `TestRFC4301SiblingSAsKeepInboundSelectorsBound`, `TestRFC4301MultipairSAsRejectSiblingScope` | a native kernel build; any packet proof; an engine caller of `dataplane.NewSAStateScope`; capability bit 3; `engine/fragment.go` (never written, and not the design any more) | source only; design below; the enforcement route is an open owner decision |
| 3, Section 6 ICMP | peer leaf `icmp-error-check` (`peerSAOptions`), `verifyICMPOverlap`, the namespace forward default (`xfrmBackend.ApplyForwardDefault`, `installUnmatched`), doctor check `ipsec-forward-default`. Probes: `TestRFC4301ForwardedICMPUsesPayloadPolicy`, `TestRFC4301ForwardedICMPErrorsMapUnderDiscard`, `TestRFC4301LocalGeneratedICMPUsesReturnSA` | a green `icmp-error-payload-check` as committed; its red proofs (c) and (d); `engine/icmp.go`, replaced by the peer leaf and the operator ICMP selector; `test/ipsec/ipsec-icmp-unauthenticated-control.ci`, whose job `test/ipsec/ipsec-icmp-type-selector.ci` does | code landed; defects D-5 and D-6 open; the handover of 2026-11-13 records the forwarded return under `unmatched discard` still red |
| 4, Section 4.4.1.1 selectors | `ICMPSelector`, `spdICMPMatches`, `validateICMP`, the Mobility Header refusal. Units: `TestICMPSelectorReachesTheBackend`, `TestSPDICMPSelectorMatchesTheRFCRangeTest`, `TestKernelICMPSelectorDropsOnlyTheNamedMessages` | the 13 mutation walks, four AC-13 records and the audit reseals; a port range (every range is refused, `parseSPDPort`) | code landed; proof owed |
| 5, Section 4.4.3.1 PAD | `remoteIDMatches` sub-tree and range arms. Units: `TestPadSubtreeAdmitsAPeerBeneathIt`, `TestPadSubtreeRefusesAPeerOutsideIt`, `TestPadAddressRangeAdmitsAnAddressInsideIt`, `TestPadAddressRangeRefusesWhatIsOutsideIt`, `TestPadKeyIDStaysExact`, `TestPadSubtreeStillBindsTheCertificate` | interop `pad-subtree-identity` | code and units landed; the named interop scenario is absent |
| 6, Section 4.4.1 ordering | `SPDPolicy.Order`, `SiteToSitePeer.PolicyPriority`, the reserved band (`ValidatePolicyOrder`, `SPDPolicy.validate`). Units: `TestSpdOperatorOrdersOverlappingPeers`, `TestSpdUnstatedOrderTakesTheDefaultRank`, `TestSpdOrderCannotCaptureTheIKEControlPlane`, `TestSPDPolicyOrderInsideBypassBandRefused` | a total order (D-2); `test/ipsec/ipsec-spd-order-readback.ci`; a green `TestRFC4301OrderedInboundSearchAdmitsOnTheFirstRankedEntry` (owned journal row) | reopened: not complete |
| 7, Section 8 DF and PMTU | `xfrmDFFlag`, `xfrmNoPMTUDisc`, the PMTU integration units, interop `tunnel-df-treatment` | a red that reached the wire-counter check (AC-8) | code landed; wire proof owed |
| 8, Section 5.1.2.1 DSCP | `TestOuterHeaderDSCPReachesTheBackend`, `TestXFRMOuterHeaderCarriesTheOuterDSCP`, `test/ipsec/ipsec-dscp-map-linux-refused.ci` | nothing this spec owes beyond the post-release kernel feature | code landed |
| 9, ledger | nothing | the closing work, plus the summary corrections listed below | blocked |

### Defects confirmed on 2026-11-14

Each is a logic defect of a capability Ze ships, so each is fixed here and none
is a question. The AC and the test are in the Acceptance Criteria and TDD
sections.

| ID | Defect | Producer read | RFC 4301 sentence | AC |
|---|---|---|---|---|
| D-1 | An operator DISCARD that fails to install only logs a warning and the apply succeeds. A changed entry is removed before its new form is installed, so a failed re-install leaves the traffic crossing. The function's own comment says the failure "fails OPEN" | `installSPDPolicies` (`engine/spd_policy.go`), called by `applyConfig` with no error path | Section 4.4.1: "The management interface for the SPD MUST allow creation of entries consistent with the selectors defined in Section 4.4.1.1, and MUST support (total) ordering of these entries, as seen via this interface." An entry the interface accepted and the dataplane does not hold was not created | AC-14 |
| D-2 | The order is not total. Every entry defaults to order 1000 (`parseSPDPolicy`), nothing refuses equal orders on overlapping entries (`SPDPolicy.validate` judges one entry alone), and the install order is map iteration (`installSPDPolicies`), which `dataplane.go` says decides a tie in the kernel. The YANG `name` leaf says "commit refuses the pair" for equal selector, direction and order, and no such check exists | `ValidateSPDPolicies`, `parseSPDPolicies`, `installSPDPolicies` | the same Section 4.4.1 sentence: "MUST support (total) ordering of these entries" | AC-15 |
| D-3 | An operator entry installs out and in policies only. Traffic forwarded to a protected host is checked in the forward direction, where no operator entry exists, so a `direction in` discard does not stop it | `spdPolicyParams`; `unmatchedDirections` names the forward lookup the operator list omits | Section 6: "Disposition of non-error, ICMP messages (that are not addressed to the IPsec implementation itself) MUST be explicitly accounted for using SPD entries." | AC-16 |
| D-4 | The IKE bypass matches any address with UDP source port 500 or 4500 in the out direction, at priority 100, above every Child SA policy (2000) and every operator order (101 and up). `LL-XFRM-FRAGMENT-SELECTOR` quotes `ip_forward` calling `xfrm4_route_forward` after the forward check, and the design page says forwarded traffic is looked up "in fwd and out". If that holds, transit UDP from a protected host with source port 500 or 4500 leaves in clear where the tunnel should protect it. The comment on `ikeBypassPolicies` argues only that no forward entry is installed | `ikeBypassPolicies` (`engine/bypass.go`) | Section 5.2, step 2: "(This implies that IKE traffic MUST have an explicit BYPASS entry in the SPD.)" The entry is owed for IKE traffic, and it matches more | AC-17 |
| D-5 | With `icmp-error-check` true (the default), the kernel's reverse check runs for every ICMP type on a Child SA, so a non-error ICMP message from outside the selectors is delivered when the bytes after its ICMP header match the SA. The upstream context lines of `0003-xfrm-state-scope.patch` (`net/ipv4/icmp.c`) show the second check gated on `XFRM_STATE_ICMP` alone, and the comment on `refuseICMPCheckDiscard` says "whatever the ICMP type, error or not" | `peerSAOptions`, `xfrmStateFromParams` (sets the state flag) | Section 5.2, step 5: "If an IPsec system receives an inbound packet on an SA and the packet's header fields are not consistent with the selectors for the SA, it MUST discard the packet." Section 6 limits the payload check to error messages: "This section applies exclusively to error messages." | AC-18 |
| D-6 | A discard entry with the default direction `both` and the default protocol 0 is refused at commit beside any policy-based peer with the default `icmp-error-check` (`refuseICMPCheckDiscard`, `icmpSpans`). User story 1 therefore fails in the default configuration. The refusal's fix, and the recipe in `implicitDiscardICMPExample`, tell the operator to turn the Section 6.2 check off | `refuseICMPCheckDiscard` (`engine/icmp_overlap.go`), `implicitDiscardICMPExample` (`engine/unmatched.go`) | Section 7.4: "All implementations MUST support DISCARDing of fragments using the normal SPD packet classification mechanisms." A discard the default configuration refuses is not supported in it | AC-19 |

**What Ze's code settles for D-4.** Under `unmatched discard` on XFRM the
forward disposition is the namespace default (`xfrmBackend.UsesForwardDefault`,
`installUnmatched`), and no producer installs a forward entry in the
local-to-remote orientation for a policy-based peer: `appendChildReceivePolicies`
builds remote-to-local forward entries, and `xfrmInterfaceExemptions` covers
only a bound XFRM interface. So a cleartext packet forwarded from a protected
host meets no forward entry of its own. If the kernel applies the forward
default to it, the packet is refused before the out lookup, and D-4 is reachable
only under `unmatched bypass`. The same reading says ordinary transit from a
protected host into a policy-based tunnel is refused under `unmatched discard`,
which no test in the tree observes either way. The packet test of AC-17 carries
that control and settles both.

**Observed 2026-11-19 (Ze's 7.3 kernel, K1 series, QEMU amd64, `69fc643e50`).**
The reading held: under `unmatched discard` the positive control of
`TestRFC4301TransitFromIKEPortIsNotBypassed` left 0 ESP datagrams, and under
`unmatched bypass` the transit left as ESP. Linux v7.3 `__xfrm_policy_check`
answers a FWD lookup that finds no policy, and no `xfrm_in_fwd_icmp` payload
policy, with the namespace default, counted `XfrmInNoPols` when it is block.
A second defect on the same path: `__xfrm_route_forward` runs an OUT lookup on
every forwarded packet, and under `unmatched discard` only the catch-all BLOCK
matches a decrypted packet's remote-to-local orientation, so a policy-based
gateway delivered nothing from the tunnel to the hosts behind it in the default
configuration (`XfrmOutPolBlock`); `runICMPForwarding` worked around it with an
operator out bypass, and `TestChildTunnelForwardsProtectedHostTraffic` runs with no
catch-all. Repair: each tunnel Child SA at if_id 0 on a backend with a separate
forward lookup installs two template-free allows one rank below its own, owned in
the ledger (`ownedEntry`), skipped where they would carry the kernel identity of its
PROTECT policies, not migrated by MOBIKE: strongSwan's outbound FWD entry
(`install_policies_outbound`, `child_sa.c`; the OUT selectors in FWD, RFC 4301
Section 5.1) and a delivery entry (the inbound selectors in OUT, RFC 4301 Section
5.2). A packet this node originates with a source inside the remote selector and a
destination inside the local selector now leaves in clear towards the protected side
rather than meeting the catch-all. Tests:
`TestChildOutboundForwardPolicyAdmitsProtectedHostTransit`,
`TestChildOutboundForwardPolicyBuiltOnlyForPolicyBasedTunnels`,
`TestRFC4301DecryptedTransitReachesProtectedHostUnderDiscard`. Still open: a
forwarded ICMP error from a protected host meets the OUT catch-all DISCARD before
`xfrm_out_fwd_icmp` can map it to the return SA (AC-4,
`TestRFC4301ForwardedICMPErrorsMapUnderDiscard`); no SPD entry can fix that on the
stock kernel. The `unmatched bypass` red at line 184 of the AC-17 test was a test
defect: `ikeOwnExchange` sent a 9-octet reply, and `UDPTransport.Run` drops every
datagram shorter than the 28-octet IKE header before `Recv`.

Later the same evening, `516671704b` closed the "still open" case above for the
catch-all: OUT allows at `dataplane.PriorityICMPErrorReturn` let the error reach the
mapping, and the nftables table `ze_ipsec_icmp_error_return` drops an untransformed
error in forward and output. `TestRFC4301ForwardedICMPErrorsMapUnderDiscard` passes.
An error an operator out or both DISCARD covers is still dropped before the mapping
(on 2026-11-20 Thomas chose the kernel route for it: K1 is extended, and this
stock-kernel route is then removed).

### Phase 2b design: the kernel patches

The fragment route is a Ze kernel feature, written as three staged patches
against Linux 7.3. Two are enrolled in the series and one is a proposal. None
has been compiled natively, and no packet has crossed any of them.

| Patch | Where | What it changes in the kernel | What Ze sends and reads |
|---|---|---|---|
| K1, state scope | `gokrazy/kernel/patches/0003-xfrm-state-scope.patch` | A state can carry an immutable, reference-counted OR-set of up to 64 directional inner selectors with a non-initial-fragment flag (netlink attribute `XFRMA_STATE_SCOPE`). The flow decoder marks which fields a packet really carries, so an unavailable port is never read as an exact 0. The ICMP paths decode the quoted packet through that same decoder. A read-only capability receipt (`XFRMA_ZE_CAPS`: feature bits, selector limit, guard references, ABI number) rides the default-policy answer | `xfrmStateScopeFromParams` writes the attribute from `SAParams.Scope`; `xfrmStateScopeFromAttrs` reads it back; `checkXFRMStateScopeCapabilities` refuses any explicit scope unless the ABI matches and feature bits 0 to 3 are all present |
| K2, fragment context | `gokrazy/kernel/patches/0004-ip-fragment-xfrm-context.patch` | The IPv4, IPv6 and conntrack reassembly queue keys carry the state scopes the fragment arrived under (up to the secpath depth), with their references and memory charge, so fragments that arrived under different protection are never reassembled together. It sets capability bit 2 | nothing new; Ze reads bit 2 in the receipt |
| K3, fragment guard | `gokrazy/kernel/patches/0005-xfrm-fragment-guard.patch`, in the series as 0005 (read 2026-11-20; it was a proposal outside the series when this table was written) | Reassembly before the policy decision for traffic a port-scoped entry could match, raw-socket route deferral and output permission, under a new kernel option. It sets capability bit 3 | Ze reads bit 3 (`xfrmZECapFragmentGuard`) through `xfrmBackend.GathersForwardFragments` (`1dc1323b86`); with it a port-scoped BYPASS is projected onto FWD and the commit refusal of `a29420771d` does not apply |

What Ze installs today (read at the tree on 2026-11-20): on a backend that
declares `dataplane.SAStateScopeInstaller` and answers `SAStateScopeSupported`,
`installChildSAs` (`engine/child.go`) sets a state scope on both Child SA states
through `childStateScope` (`engine/child_scope.go`), which calls
`dataplane.NewSAStateScope` with the SA's full directional selector set and
non-initial-fragment permission false; in transport mode the legacy state
selector is the scope's single record. On any other backend a single-pair tunnel
SA still carries its selector on the inbound state (`tunnelStateSelectorSupported`,
`inboundStateSelector`) and a multi-pair SA carries none. The series now
advertises bits 0, 1, 5, 6 and 7 from K1 (`0003`), bit 2 from K2 (`0004`), and
bit 3 from K3 (`0005`) only when `CONFIG_XFRM_FRAGMENT_GUARD` is built and the
guard is ready. `checkXFRMStateScopeCapabilities` requires bits 0, 1, 2, 3, 5, 6
and 7 and knows bit 4, which no patch sets.

Known defect in the K3 proposal, from its review (handover of 2026-11-13):
authenticated IPv6 transport ESP can carry plaintext Fragment headers and reach
reassembly. No fix exists.

The acceptance criteria are AC-2 and AC-20. The packet tests are the probes
named in the phase table plus `TestRFC4301ForwardedFragmentsRespectOperatorDisposition`,
all run in the QEMU guest on Ze's kernel built with the series. Owner decision 1
(below) chose this kernel route on 2026-11-16, and the 2026-11-19 D-6 decision
extends K1 so the reverse ICMP-error check never overrides an operator DISCARD
entry.

### Summary rows that disagree with this spec

`rfc/short/rfc4301.md` is not edited by this spec's correction. A row with no
annotation and a verified discrimination record on each polarity is counted and
published as proven (`figuresByStem`, `internal/le/rfc/profilecoverage.go`: "
`{gap}` and `{partial}` stay in the denominator and are NEVER proven", and an
unannotated row is proven by its records). Each row below has no annotation and
holds records under `rfc/discrimination/rfc4301/`, so a reader of the public
RFC 4301 page sees it counted in the proven MUST figure and absent from the
unmet list. `./le rfc check` says whether each record still verifies.

| Row | What its tagged tests assert | What this spec's evidence says | Annotation the row should carry |
|---|---|---|---|
| `RFC4301-7.4-2` | `TestPortScopedBypassNeverReachesTheForwardPath` and `TestXFRMPortScopedBypassIsNeverStoredOnTheForwardPath`: no port-scoped bypass is stored in the forward direction. `TestIKEBypassIsPortScopedSoSection74Binds`: the IKE bypass is port-scoped | nothing performs stateful fragment checking or reassembly for a port-scoped BYPASS or DISCARD entry on the forward path; the port-scoped out bypass is consulted for forwarded traffic (D-4); phase 2b is source only | `{gap}`: no stateful fragment checking exists for a port-scoped BYPASS or DISCARD entry; a forwarded fragment is classified with port 0 (`LL-XFRM-FRAGMENT-SELECTOR`); the tests prove only that no port-scoped bypass is stored in the forward direction; this spec |
| `RFC4301-7.3-4` | the inbound entry carries its exact port selector unchanged; a selector every fragment matches is narrowed away | a multi-pair or sibling SA shares reqid 1 and SPI-less templates, so a sibling's any-port forward policy accepts a fragment of an exact-port SA (owned row `two-owners-share-one-name.md`, read and not run). A single-pair tunnel SA now carries its selector on the state, which may have closed that case; `TestRFC4301SiblingSAsKeepInboundSelectorsBound` is the unobserved probe | `{partial}`: tested, a single negotiated pair; gap, a non-initial fragment on a multi-pair or sibling SA, which no packet test observes; this spec |
| `RFC4301-7.3-5` | the inbound entry carries the selector and template; `TestRFC4301FragmentsOfANonCompliantPacketAreDiscarded` | a forwarded fragment of a port-specific Child SA misses its policy and meets the catch-all, in clear under `unmatched bypass` (owned row `silent-fall-through.md`) | `{partial}`: tested, a fragment delivered to this node; gap, a forwarded fragment under `unmatched bypass`; this spec |
| `RFC4301-4.4.1.1-3` | an entry requiring one port carries that exact port | the same forwarded case: the fragment is not discarded, it meets the catch-all | `{partial}`: tested, the selector Ze installs; gap, the forwarded fragment's disposition under `unmatched bypass`; this spec |
| `RFC4301-4.4.1-4` | the operator orders peers; an unstated order is never 0; an order that captures IKE is refused | D-2: equal orders on overlapping entries are accepted and resolved by install order | `{partial}`: tested, distinct orders; gap, equal orders on overlapping entries; this spec |

Added 2026-11-20: the prose under the FWD row of `rfc/short/rfc4301.md`'s
fragment table (around line 394) says "every port-scoped BYPASS Ze can install
is confined to IN and OUT." That is false on Ze's kernel: with K3's capability
bit 3, the "done" half of decision 2 (`1dc1323b86`) projects a port-scoped BYPASS
onto FWD. Without bit 3 the commit refuses it (`a29420771d`), except a bypass
whose local prefix is one address this host holds. The text needs rewriting with
the `RFC4301-7.4-2` records.

On 2026-11-19 the summary's Purpose line was corrected to describe Ze's native
IKEv2 engine, and its Enrolment reason to count the 104 rows the file holds
(AC-11). The five rows above are not re-annotated yet: they need a re-judge after
the D-1 to D-6 fixes land. The summary's body still names strongSwan in its
Architecture Overview, SPD and "SA and Key Management" prose, which describes the
earlier strongSwan design; AC-11 does not name it, and it is listed in Work Not
Done.

### Owner decisions recorded through 2026-11-19

Thomas made three decisions on 2026-11-19. Each is written into the design page
that documents its subject, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`,
and into the AC rows it changes.

- Decision 3 (a), a boot configuration holding a conflict a commit would refuse,
  and an SPD entry that fails to install at boot: keep it. Ze installs what it
  can, logs at Error, and `ze doctor` reports the conflict and its fix. Start-up
  never refuses the IPsec block. AC-14 carries the boot half.
- The D-6 route (AC-19): the kernel honours the discard. K1 is extended so the
  reverse ICMP-error check never overrides an operator DISCARD entry, and the
  commit refusal `refuseICMPCheckDiscard` goes. Decision 4 is superseded: it
  applied only if decision 1 were (b), and decision 1 is (a).
- The D-4 route (AC-17): Ze's IKE bypass becomes per-socket XFRM policies on
  Ze's own IKE sockets, as strongSwan does, and the node-wide any-address port
  500 and 4500 bypass is removed.

On 2026-11-20 Thomas decided how a forwarded ICMP error is mapped into the
return SA when an operator out or both DISCARD, or the catch-all DISCARD, covers
it (AC-4, `RFC4301-6.2-3` forwarded case). He chose the kernel route. The K1 patch
(`gokrazy/kernel/patches/0003-xfrm-state-scope.patch`) is extended so the
forwarded path, `__xfrm_route_forward` included, tries `xfrm_out_fwd_icmp` when the
OUT lookup answers a BLOCK (`-EPERM`), as `icmp_route_lookup` already does for a
locally generated error. A K1 feature bit advertises the new path, and the engine
requires that bit as it requires bit 5. Once the extension lands, the stock-kernel
route of `516671704b` (the OUT allows at `dataplane.PriorityICMPErrorReturn` and
the nftables table `ze_ipsec_icmp_error_return`) is removed, not kept beside it
(`ai/rules/no-layering.md`). Set aside: an allow at the entry's order minus 1 with
order refusals, and a commit refusal. The design page records it in "ICMP error
messages carried by a Child SA".

Also on 2026-11-20 Thomas decided a D-3 defect found that night. The FWD transit
gate of an operator out or both BYPASS (a template-free ALLOW at
`dataplane.PriorityTransitGate`, `spdTransitPolicies`, `072ef5bf1f`) matches a
forwarded ICMP error that arrived on a Child SA by the error's own headers, so
stock `__xfrm_policy_check` never takes `xfrm_in_fwd_icmp`'s payload policy and
rejects the consistent error (`XfrmInTmplMismatch`). It was seen as
`TestRFC4301ForwardedICMPUsesPayloadPolicy` red at if_id 0 on the bit-6 kernel
`qemu-c860ed8b`; the if_id 43 variants pass. RFC 4301 Section 5.2 step 4 requires
the payload match. He chose the kernel route: a new K1 capability bit 7,
"forwarded ICMP error past a gate". In the FWD check, an ICMP error that arrived
with a secpath and whose own-header policy is a template-free ALLOW carrying
`XFRM_POLICY_ICMP` takes `xfrm_in_fwd_icmp`'s payload policy instead. Ze sets
`XFRM_POLICY_ICMP` on every FWD policy an operator bypass projects (its FWD gate
and its FWD decision); never on an OUT gate or half; a discard's FWD decision
does not carry it. Ze requires bit 7 as it requires bits 5
and 6. Set aside: not projecting a BYPASS into a FWD gate where it would cover
Child SA traffic (it narrows AC-16), and a template on the gate (not viable:
`xfrm_state_ok` needs an exact tunnel destination). The design page records it in
"Transit traffic".

Later on 2026-11-20 Thomas answered "ok" to the recommendations for decisions
3, 4 and 5. Decision 3: the 13 lost phase-4 Mobility Header mutation walks are
rebuilt as one surviving-mutant check per producer guard, from the 15 untagged
candidate tests. Decision 4: `bypass-fragment-forgery` is retargeted to an
operator port-scoped BYPASS on Ze's kernel in a K1/K3 lab, and proves K3's
reassembly defeats forged fragments on the forward path now that `1dc1323b86`
projects such a bypass. Decision 5a: an operator SPD entry on VPP binds to every
interface Ze's SPD is attached to, the same node-wide meaning it has on XFRM;
VPP installs no operator entries today, so this is owed work (on VPP, after the
release, by `plan/post-release/features/spec-ipsec-vpp-policy-interface.md`,
Thomas's ruling of the same day). Decision 5b: the
RFC 4301 extraction sign-off stays in the scope of
`plan/release/validation/spec-rfcgate-6-supported-extraction-signoff.md`, and
this spec supplies the evidence. Decision 5c: owner decision 3 (install what it
can, log at Error, `ze doctor` reports, start-up never refuses the IPsec block)
also covers a catch-all install failure at boot, which today fails the apply
(owed code), and the doctor finding states that the default discard is not in
force. The design page records 4, 5a and 5c in "Decisions".

Two corrections were recorded with the bit 7 decision. The `40cf3cfaec` patch is not defective:
the cleartext the operator-discard test saw came from a fixture whose two entries
share a kernel selector, which Ze refuses at commit, and the test now runs
`verifySPDOrder`. And an open question: this spec records
`TestRFC4301ForwardedICMPUsesPayloadPolicy` passing at `516671704b`, which already
held D-3 (`072ef5bf1f`), so that pass is unexplained until it is re-read.

Also on 2026-11-20 Thomas answered open decision 2, the port-scoped operator
BYPASS that the forward path cannot judge on a stock kernel: "refusal until
done". The commit refuses such an entry, never a notice, until the forward-path
stateful fragment check (RFC 4301 Section 7.4) is done. `a29420771d` implements
the refusal: on an XFRM dataplane a `vpn ipsec policy` bypass with a local or
remote port is refused unless its local prefix is one address this host holds;
VPP, a discard with a port and an any-port bypass are not refused; at start the
entry is installed and logged at Error (decision 3) and doctor check
`ipsec-spd-forward-bypass` reports it. The "done" half, written and not yet
committed on 2026-11-20, lifts the refusal and projects the bypass onto FWD when
the kernel advertises K3's capability bit 3 (`dataplane.ForwardFragmentGatherer`).

This table preserves the choices and evidence recorded at that time. Decision 1
subsequently selected the full kernel route, whose product fix is now closed.
Its old alternatives and product AC-4 status are historical. Other architecture
questions retain their own scope and are not answered by the product closure.

On 2026-11-16 the owner selected the full kernel implementation route for
decision 1: "ok do that work" and "complete the three points", after the
explanation of per-tunnel traffic permissions, fragment-context isolation and
port-rule enforcement for fragmented packets. Complete and prove K1, K2 and K3
with Ze's engine wiring. The refusal-only release route is not selected, and
dual-form AC-4 remains unchanged. The other decisions below are unchanged.

Each row is a choice no RFC text, agreed design or rule settles. Everything
else in this section is fixed without asking.

| # | Decision | Evidence | Options | What each changes |
|---|---|---|---|---|
| 1 | The Section 7.4 enforcement route for the release | K1 and K2 are enrolled and unbuilt; K3 is a proposal with a known transport defect; capability bit 3 is absent; the Linux limit `LL-XFRM-FRAGMENT-SELECTOR` is verified at 7.3 | (a) finish K1 to K3 and prove them by packet tests before release; (b) refuse at commit every configuration the stock classifier gets wrong (a port-scoped operator entry and a port-specific traffic selector on a gateway under `unmatched bypass`) and keep the kernel route for after the release; (c) both, with the refusal lifted when the receipt shows bit 3 | (a) holds the release on kernel work nobody can prove today. (b) removes port-specific tunnels and port-scoped entries from gateways under `unmatched bypass`, and `RFC4301-7.4-2` stays a gap. (c) ships (b) now and (a) later with no second config change |
| 2 | Direction shown for Ze's own SAs (carried from 2026-11-12) | DECIDED (a) by Thomas on 2026-11-19: set the direction at install. Implemented as AC-21: `xfrmStateFromParams` maps `SAParams.Dir` to `XFRMA_SA_DIR`, and the outbound Child SA drops its anti-replay window, the one `verify_newsa_info` conflict | (a) set the direction at install; (b) show the engine's belief; (c) leave it | (a) chosen; `verify_newsa_info` read at Linux v7.3 by the main thread on 2026-11-19 |
| 3 | A boot configuration that holds a conflict a commit would refuse | DECIDED (a) by Thomas on 2026-11-19, and it covers an SPD entry that fails to install at boot too: Ze installs what it can, logs at Error, and `ze doctor` reports the conflict and its fix; start-up never refuses the IPsec block. Evidence at the time: the start is never verified. `warnICMPOverlapAtStart` and `warnImplicitDiscardAtStart` install the configuration and log at Error level. No owner ruling covers the start: the 2026-11-12 rulings say "refuse at commit". A per-SA option is different and already safe: the install refuses it (`dataplane.ErrSAOptionRefused`) | (a) keep install and log, with the doctor finding; (b) refuse to apply the IPsec block at start | (a) runs a box with an entry the kernel overrides. (b) leaves a box that boots such a file with no tunnels, which is the failure the door's trap records for an earlier start-time refusal |
| 4 | How D-5 and D-6 are closed if decision 1 is (b) | SUPERSEDED on 2026-11-19: it applied only if decision 1 were (b), and decision 1 is (a). D-6 takes the kernel route above. Evidence at the time: K1 changes the ICMP decode paths; without it the stock kernel gates the reverse check on the state flag alone | (a) make `icmp-error-check` default false on a kernel without the receipt, so discard entries work and Section 6.2 is off by default; (b) keep the default and narrow `refuseICMPCheckDiscard` to entries that name an ICMP error type, accepting that a protocol 0 discard is overridden for ICMP on that SA and saying so at commit | (a) trades the Section 6.2 default for a working discard. (b) keeps Section 6.2 and leaves a disclosed hole in a protocol 0 discard |

### Names in this spec that the tree does not hold

| The plan says | The tree holds |
|---|---|
| `TestBackendRefusesUnexpressibleAction` | `TestXfrmPolicyActionRefusesAnUnknownDisposition` (`dataplane/rfc4301_discard_linux_test.go`); VPP expresses DISCARD, so it has no refusal (`TestVPPSPDActionCarriesEveryDisposition`) |
| `TestNonInitialFragmentIsNotBypassed`, scenario `bypass-fragment-forgery`, `ipsec-bypass-fragment-check.ci`, `engine/fragment.go` | nothing that proves it. The probes are `TestRFC4301CleartextFragmentsRespectOperatorDisposition` and `TestRFC4301ForwardedFragmentsRespectOperatorDisposition`, unobserved |
| `TestUnauthenticatedICMPTypeRejected`, `ipsec-icmp-unauthenticated-control.ci`, `engine/icmp.go` | `TestSPDICMPTypeControlRejectsOrAcceptsOneType`, `TestSPDICMPErrorControlFromTheProtectedSide`, `TestKernelICMPSelectorDropsOnlyTheNamedMessages`, `test/ipsec/ipsec-icmp-type-selector.ci` |
| `TestTransitICMPPayloadHeaderChecked` | `TestICMPErrorPayloadHeaderCheckedOnLocalDelivery` (local delivery) and `TestRFC4301ForwardedICMPUsesPayloadPolicy` (forwarded) |
| `TestPortRangeSelectorRefusedWhenNotExpressible` | `TestSPDPolicyPortNeedsAProtocolThatCarriesPorts`, `TestRFC4301SPDInterfaceRefusesWhatItCannotProgram` |
| `TestRemoteIDSubTreeMatch`, `TestRemoteIDAddressRangeMatch`, scenario `pad-subtree-identity` | the six `TestPad...` units of `engine/rfc4301_pad_subtree_test.go`; no scenario at the 2026-11-14 read; the scenario is written in `22a4740af7` |
| `TestPolicyOrderReachesTheBackend`, `TestPolicyOrderInsideBypassBandRefused`, `ipsec-spd-order-readback.ci` | `TestSpdOperatorOrdersOverlappingPeers`, `TestSPDPolicyOrderInsideBypassBandRefused`, `TestSpdOrderCannotCaptureTheIKEControlPlane`; no `.ci`. Since `50b89fb51b` the two band tests are replaced by `TestSPDPolicyOrderOutsideTheRankRangeRefused` and `TestSPDRankBoundsMatchTheYANGRanges`, and the `.ci` is the draft `test/draft/ipsec/ipsec-spd-order-readback.ci` || `TestPMTUAgingPeriodConfigurable`, `engine/pmtu.go` | the kernel ages the PMTU (A-4); `TestXFRMTunnelPMTUAgesBackToTheLinkMTU`, `TestXFRMTunnelPMTUAgesBackToTheLinkMTUOverIPv6`; the period is the `mtu_expires` sysctl |
| `test/ipsec/ipsec-df-bit-treatment.ci` | `test/parse/ipsec-df-bit-treatment.ci` and `test/ipsec/ipsec-sa-options-kernel.ci` |
| scenario `spd-discard-entry` | nothing at the 2026-11-14 read; written in `22a4740af7` |
| SPD entry order 1 to 65535 | YANG `order`: 101 to 2147483647, default 1000, at the 2026-11-14 read; 1 to 2147483647, default 1000, since `50b89fb51b` |

## Historical checkpoint for continuation, 2026-11-13

The library repair landed in `352f231413`; its ordered series and upstream-style
tests landed in `b43120b371` and `c9ce6ba81d`. The kernel K1/K2 patches and their
series entries landed in `5c063bc8d6`, still without native C compilation or
packet proof. The K3 proposal is preserved in `2ae4f0f3f4` at
`gokrazy/kernel/proposals/xfrm-fragment-reassembly.patch`, outside the active
series. Capability bit 3 remains absent. The proposal still owes the authenticated
transport-fragment correction, native build and packet/lifetime proof. These
commits preserve work; they do not close the fragment or SA-appropriateness gaps.

The Ze callers and regression probes landed separately in `466ab09fd8`. The
library-series session can resume moving the private ABI; coordinate edits
to `dataplane.go`, `state_scope.go`,
`xfrm_state_scope_linux.go`, `xfrm_linux.go` and `xfrm_forward_default_linux.go`.
The original scope/publication-reconciliation edits remain in the working tree
for an explicit closure review, outside this lifecycle checkpoint. See
`plan/handover/2026-11-13-ipsec-checkpoint.md` for the next tests, exact artifact
locations and machine-transfer requirements. This spec remains in progress.

## Historical resumed phase 2b, kernel K1, 2026-11-13

The reviewed state-scope and fragment design is being implemented in stages.
K1 owns the exact per-state OR-set, immutable identity and migration lifetime,
own-state IN and directional OUT matching, availability-aware ICMP decoding,
netlink validation/readback/capabilities, packet-offload refusal, and automatic
guard-reference ownership. The ordinary netlink handlers are the entry points.
K2 owns fragment queue identities/references/accounting; K3 owns hooks, raw
routing deferral and output permission. K1 must not advertise those later
bits before their producers exist. No mutable reassembly switch is introduced.

The pinned Linux 7.3 C source is authored in the session's sparse third-party
patch workspace. Main owns generation of the tracked patch, all compilation,
kernel/image builds and execution. Existing real guest packet reds precede
this source. New ABI and packet tests, native build, lifetime stress and
independent review remain owed; source authoring is not proof. Phase4's
selector mutation/RFC record and regression work also remains owed.

## Historical resumed phase 4, 2026-11-12

Wanted exact MH support remains owned by
`plan/post-release/kernel/spec-xfrm-ipv6-mh-selector.md`. The 2026-11-12
incoming/forward guest probe established that ANY Types5/7 deliver on both
paths, while exact Type5 drops its matching packets on both paths. This
release package must refuse that unsupported constrained selector; a high-octet
projection alone is insufficient. The future kernel skeleton retains the
decoder/extraction work without changing the selector requirement into an
exclusion or a full-support claim.

The current package repairs specific selectors under protocols with undefined
ports, including AC-13 request, answer and encoder boundaries. RFC-valid MH
Type encoding is retained, but every constrained MH Type is refused because
the real kernel drops even matching received/forwarded packets. ANY MH remains
usable. DCCP remains a defined-port protocol whose exact port Linux XFRM cannot
select; UDP-Lite ports are accepted on Linux. VPP refuses constrained MH and
UDP-Lite selectors that its matcher would ignore.

The lead observed the named new tests fail before implementation, followed by
the initial targeted three-package pass (`job-unit-pkg-e5bceb7c.log`). The
subsequent real-MH packet failure forced exact refusal and a second constructor
TDD red (`job-unit-pkg-caad188b.log`). After that repair, all new phase4 names
pass under race/verbose execution (`job-ipsec-selector-default-units-87450c7b.log`),
and the guest preserves all four ANY packet deliveries plus typed exact refusal
(`job-ipsec-engine-boundary-guest-b39a72f7.log`). The 13 native mutation walks,
four AC-13 RFC records, affected audit reseals and broader scoped regression
remain owed. Update 2026-11-20: the four AC-13 records for `RFC7296-3.13.1-1`
and `-2` exist (`7234fdfecc`); only their freshness check is owed. This does not close phase4 or another outstanding phase. The phase
metadata was corrected at this resumption checkpoint; it had remained unset
during the initial edits.

## Historical session progress, 2026-09-06

Phase 2's DISCARD half landed. Phases 5 and 6 were found ALREADY LANDED by an
earlier session and are not re-implemented here. Five blocks stay open and each
is its own package (R-6).

| Phase | Block | State |
|---|---|---|
| 2a | Section 7.4 DISCARD disposition | LANDED. `RFC4301-7.4-1`, 10 tagged units, 10 discrimination records |
| 2b | Section 7.4 stateful fragment checking | UNDER INVESTIGATION per the final 2026-09-06 progress entry. Establish backend behaviour before choosing enforcement; no verified vulnerability is claimed |
| 3 | Section 6 ICMP processing | NOT STARTED |
| 4 | Section 4.4.1.1 ICMP type/code selectors | NOT STARTED. `RFC4301-4.4.1.1-1` still carries its `{gap}` |
| 5 | Section 4.4.3.1 PAD matching | LANDED BEFORE THIS SESSION (`rfc4301_pad_subtree_test.go`) |
| 6 | Section 4.4.1 SPD ordering | LANDED BEFORE THIS SESSION (`rfc4301_spd_order_test.go`) |
| 7 | Section 8 DF bit and PMTU | NOT STARTED |
| 8 | Section 5.1.2.1 outer-header DSCP | NOT STARTED |
| 9 | ledger closure | BLOCKED on both Depends specs, see below |

**What phase 2a landed.** `dataplane.SPActionDiscard`, projected to
`XFRM_POLICY_BLOCK` on Linux and `IPSEC_API_SPD_ACTION_DISCARD` on VPP, with
`xfrmPolicyAction` and `vppSPDAction` refusing a disposition they cannot express
rather than substituting one. `SPAction.isTemplateFree` replaces three
`== SPActionBypass` tests that would each have read a discard as a protect
entry. The readback reports a kernel `block` policy as DISCARD on its own action
rather than on an empty template list. The operator surface is a new
`vpn ipsec policy <name>` list carrying action, order, direction, protocol and a
local/remote prefix and port pair, reconciled into the dataplane by
`installSPDPolicies` and released on every engine exit.

**Phase 2b investigation boundary.** The 2026-09-06 read found no fragment
field on `XfrmSelector`. That describes the policy representation, but does
not establish whether kernel reassembly already discharges Section 7.4.
The final progress entry therefore leaves A-3 unvalidated. Establish the
backend packet path first, then put any required enforcement choice to the
owner: existing reassembly with evidence, a rule Ze installs, or refusal of
an uncovered policy. Refusal must account for the port-scoped IKE bypasses
on UDP 500 and 4500; it cannot silently break IKE to satisfy the check.

**Assumptions settled this session.**

| ID | Verdict | Evidence |
|----|---------|----------|
| A-1 | CONFIRMED then CLEARED | `SPAction` held two members; it now holds three |
| A-2 | CONFIRMED | a case-insensitive `fragment` search over `internal/component/ike/` returns EAP-TLS fragmentation, two IKE notify constants and one ESP header comment. No IP fragment state |
| A-6 | CONFIRMED | `IPSEC_API_SPD_ACTION_DISCARD` exists (`vendor/go.fd.io/govpp/binapi/ipsec_types/ipsec_types.ba.go:242`). VPP expresses DISCARD, so no refusal test is reachable for it |
| A-7 | UNVALIDATED | the interop suite was not run this session (owner deferred heavy testing). Every `./internal/component/ike/...` package test passes |

**The seam against the two Depends specs.** `plan/release/fixes/spec-ipsec-lifetime-volume.md`
owns `lifetimeState.softBytes`, `ESPGroup` volume fields and the `life-bytes` /
`life-packets` leaves; nothing here touches `rekey.go` or the esp-group. Its
status is `design`, so Section 4.4.2.1 stays unmet and AC-12 cannot be reached.
`plan/release/validation/spec-rfcgate-6-supported-extraction-signoff.md` owns
`rfc/extraction/rfc4301.json` and `supportClaimingScope`; nothing here writes
either, and that spec's own 2026-09-05 note records that whether `rfc4301`
re-enters the support-claiming set is an owner call no session may settle by
re-derivation. AC-12 is therefore blocked on both, not on this phase.

## Historical session progress, 2026-11-10

Phases 3, 4, 7 and 8 are written and reviewed. Their units, functional tests
and the PMTU QEMU unit have run green in the main checkout. Of the two interop
scenarios, `tunnel-df-treatment` passes, and `icmp-error-payload-check` is red
because of a defect outside this spec, the dual-form receive SPD bypass, so the
spec stays in progress, blocked on that defect (see "Evidence, 2026-11-10"
below). The phase table
of 2026-09-06 above is superseded for these four rows.

| Phase | Block | State |
|---|---|---|
| 3 | Section 6 ICMP | Historical phase receipt: units and `.ci` green; interop `icmp-error-payload-check` RED as committed from the dual-form SPD bypass, and green with strongSwan `encap = yes`. Later clean and proofs (c)/(d) are recorded in the historical `spec-ipsec-dual-form-receive-spd-bypass` |
| 4 | Section 4.4.1.1 ICMP type and code selectors | units and `.ci` green |
| 7 | Section 8.1 DF copy and clear, Section 8.2 PMTU proof | units, `.ci` and QEMU IPv6 PMTU unit green; interop `tunnel-df-treatment` PASS, both red proofs red |
| 8 | Section 5.1.2.1 outer DSCP | units and `.ci` green |
| 2b, 9 | fragment checking, ledger closure | unchanged from 2026-09-06 |

**Evidence, 2026-11-10 (main thread, main checkout, after the last fixes).**

| Run | Result |
|---|---|
| `go test` with every feature tag: `ike/ipsec`, `ike/dataplane` (namespace units in a user namespace included), `ike/engine`, `ike/cmd` | ok |
| `go test`: `core/sysctl`, `core/diagnostic`, `le/interoplab/ipsec`, `test/interop-ipsec` | ok |
| `./le go lint run scope ./internal/component/ike/...` | 0 findings |
| QEMU, root in the initial user namespace: `TestXFRMTunnelPMTUAgesBackToTheLinkMTUOverIPv6` | PASS |
| Parse `.ci`: `ipsec-df-bit-treatment`, `ipsec-dscp-map`, `ipsec-icmp-type-selector-refused`, `ipsec-icmp-error-check` | PASS |
| QEMU guest `.ci`: `ipsec-sa-options-kernel`, `ipsec-dscp-map-linux-refused`, `ipsec-icmp-type-selector`, `ipsec-show-dataplane-kernel` | PASS |
| Host `.ci`: `ipsec-dataplane-show` | PASS |
| Interop `tunnel-df-treatment`, le built at eec854d922 | PASS |
| `tunnel-df-treatment` red proof (a): `xfrmDFFlag` answers false for `DFClear`, image rebuilt | RED at the first check, the `show` readback: "ze reports df-bit ... on its outbound SA, want ...". The wire-counter check was not reached. File restored |
| `tunnel-df-treatment` red proof (b): `xfrmDFFlag` answers true for `DFCopy`, image rebuilt | RED at the same `show` readback check; the wire-counter check was not reached. File restored |
| Interop `icmp-error-payload-check`, as committed (strongSwan sends bare ESP) | Historical RED: "the error embedding 10.2.0.1 -> 10.7.0.5, outside the SA's selectors: ze delivered 3 Time Exceeded (Icmp.InTimeExcds), want the error dropped" (`tmp/session/2026-11-09-222726/scratch/ab-plain.log`). Cause: loopback re-presentation skipped inbound policy. Later fixed in the historical `spec-ipsec-dual-form-receive-spd-bypass` |
| Interop `icmp-error-payload-check` with strongSwan `encap = yes` | PASS (`tmp/session/2026-11-09-222726/scratch/ab-encap.log`) |
| `icmp-error-payload-check` red proofs (c) and (d) | NOT RUN: they cannot discriminate until the scenario is green as committed, which waits on the bypass fix |
| Re-run after the last fixes: `go test` with every feature tag of `ike/engine`, `ike/ipsec`, `ike/dataplane`, `ike/cmd`; parse `ipsec-icmp-type-selector-refused` | ok; PASS |
| `TestLinuxLimitsCheckedAtShippedVersions` (`internal/appliance`) after `LL-XFRM-OUTER-DF-SET-DSCP` and `LL-XFRM-ICMP-TRANSIT` were re-read at Linux 7.3 | PASS |
| Discrimination records: 156 `rfc4301`, and 3 `rfc7296` (`RFC7296-2.23-12` twice, `RFC7296-2.23-2`) staled by this spec's edits to `xfrmStateFromParams` and `newChildSA` | all taken |
| The `RED PROOF` breaks named in this spec's `.ci` files | NOT RUN |

**Bookkeeping, 2026-11-10.** Both Linux limitation entries this spec wrote were
re-read against the v7.3 sources (`xfrm4_tunnel_encap_add`, the uapi
`XFRM_STATE_*` and `XFRM_SA_XFLAG_*` lists, `__xfrm_policy_check`,
`__xfrm_route_forward`, `xfrm_lookup_with_ifid`, `icmp_rcv`, `icmpv6_rcv`,
`icmp_route_lookup`, `icmpv6_route_lookup`); every quoted statement holds, and
their `Checked` lines now read Linux 7.3. An independent judge re-judged the
four stale audit verdicts: `RFC7296-3.13.1-3` stays enforced,
`RFC4303-3.4.4.1-2` stays unimplemented, and `RFC7296-3.13.1-1` and `-2` drop
to `weak` (a port under a portless protocol other than 0 is committed and
emitted). The four shifted `rfc7296` verdicts were resealed. Four defects
found on the way are journal rows owned below.

Fixes made while running: four test-setup corrections (loopback
`disable_xfrm` and `disable_policy` in the ICMP error integration units, the
IPv6 PMTU units' inner MTU, a direct sysctl exec in `icmperror.go`), the
`ipsec-icmp-error-check` expectation (`ze config fmt` prints `disable`), step 3
of `ipsec-sa-options-kernel` without a pipe, and a lint and arch cleanup
(`unknownName`, side constants, compound-guard splits, `verifyPeerSAOptions`
returning an operator error, range-over-int loops). An independent
`ze-test-judge` re-judged the range-over-int rewrites and the earlier test
changes.

**Historical goal validation, 2026-11-10.**

| Goal | Evidence | State |
|---|---|---|
| Section 4.4.1.1: an operator SPD entry selects on ICMP type and code, held exactly by the kernel | `TestSPDICMPTypeControlRejectsOrAcceptsOneType`, `test/ipsec/ipsec-icmp-type-selector.ci` (read back from the real SPD) | met |
| Section 6.2: an ICMP error on a Child SA has its payload checked, or not, per SA | `TestICMPErrorPayloadHeaderCheckedOnLocalDelivery`, `ipsec-sa-options-kernel.ci`; against strongSwan: `icmp-error-payload-check` | unit and `.ci` met; interop RED as committed from the dual-form SPD bypass, PASS with `encap = yes`; blocked on `spec-ipsec-dual-form-receive-spd-bypass` |
| Section 8.1: the outer DF bit is copy or clear per peer, on the wire | `TestXFRMOuterHeaderCarriesTheDFTreatment`, `ipsec-sa-options-kernel.ci`; against strongSwan: `tunnel-df-treatment` | readback met, wire unproven (corrected 2026-11-14): interop PASS, but red proofs (a) and (b) both went red at the `show` readback, so no break has reached the wire-counter check and the wire assertion has no recorded red. Owed: a break that keeps the readback true and changes the wire (AC-8) |
| Section 8.2: a PMTU is learned and ages back, and a DF-clear packet is fragmented | the PMTU integration units, IPv6 aging under QEMU | met; the fragmentation rows take the kernel-producer escape |
| Section 5.1.2.1: outer DSCP is copy or 0 on Linux, and a fixed value is refused with the fix named | `TestXFRMOuterHeaderCarriesTheOuterDSCP`, `ipsec-dscp-map-linux-refused.ci` | met |
| A per-SA option a backend cannot apply is refused, never degraded | `TestVerifyPeerSAOptionsRefusesWhatTheBackendCannotApply`, `TestChildSAOptionRefusalIsNotKeptDegraded` | met |

**Requirement rows, as `rfc/short/rfc4301.md` reads on 2026-11-10.** "Met"
means the row carries no annotation.

| Requirement | Phase | Row state |
|---|---|---|
| `RFC4301-4.4.1.1-4` | 4 | met |
| `RFC4301-6-1` | 3 | `{gap}`: operator entries install in and out only, so no bypass or discard for transit ICMP can be written on a gateway; the fwd projection must first decide how a non-initial fragment is classified (Section 7) |
| `RFC4301-6.1.1-1` | 3 | met |
| `RFC4301-6.1.1-2` | 3 | met |
| `RFC4301-6.1.2-1` | 3 | met |
| `RFC4301-6.2-1` | 3 | met |
| `RFC4301-6.2-2` | 3 | `{gap}`: holds for an ICMP error addressed to or generated by Ze; absent for a forwarded one (`LL-XFRM-ICMP-TRANSIT`) |
| `RFC4301-6.2-3` | 3 | `{gap}`: no namespace test observes the return-SA mapping; a forwarded error is dropped by the out catch-all under `unmatched discard`; the kernel also maps an error a bypass entry would carry in clear (PENDING OWNER below) |
| `RFC4301-6.2-4` | 3 | `{gap}`: performed for an error addressed to Ze (`TestICMPErrorPayloadHeaderCheckedOnLocalDelivery`); a forwarded one is dropped unread (`LL-XFRM-ICMP-TRANSIT`) |
| `RFC4301-6.2-5` | 3 | `{gap}`, testing only: the forwarded case is dropped by the fwd catch-all with no namespace test observing it |
| `RFC4301-8.1-1` | 7 | met |
| `RFC4301-8.1-2` | 7 | `{gap}`: copy and clear per peer; set refused at commit (`LL-XFRM-OUTER-DF-SET-DSCP`). The row's text still calls the owner's choice pending, which the decision below settles; the summary text is owed that repair |
| `RFC4301-8.2.1-1` | 7 | met |
| `RFC4301-8.2.2-1` | 7 | `{lower-layer}`: Linux route PMTU exceptions aged by `mtu_expires` under the XFRM bundle (A-4) |
| `RFC4301-5.1-3` | 7 | met |
| `RFC4301-5.1.2.1-1` | 8 | `{gap}`: copy and 0 on Linux, any value on VPP; 1 to 63 refused on Linux at commit and install. The row's text still calls the owner's choice pending; same repair owed |

**Tests written this session.** All have run green except
`icmp-error-payload-check`, red from the dual-form SPD bypass (evidence table
above).

| Test | Kind | Proves |
|---|---|---|
| `tunnel-df-treatment` | interop, `test/interop-ipsec/scenarios/` | Section 8.1: outer DF under `clear` then `copy`, read by strongSwan's nftables counters, plus the `df-bit` readback |
| `icmp-error-payload-check` | interop, `test/interop-ipsec/scenarios/` | Section 6.2: an ICMP error addressed to Ze over the SA, consistent delivered, inconsistent dropped |
| `test/ipsec/ipsec-sa-options-kernel.ci` | functional | `df-bit clear`, `dscp-map 0` and `icmp-error-check false` reach the real kernel's Child SA states, read back through `show vpn ipsec dataplane sa` |
| `test/ipsec/ipsec-dscp-map-linux-refused.ci` | functional | a commit of `dscp-map 46` on the Linux XFRM dataplane is refused, naming why and the fix |
| `test/ipsec/ipsec-icmp-type-selector.ci` | functional | a `vpn ipsec policy` rejecting one ICMP type and permitting another, read back from the real kernel SPD |
| `test/parse/ipsec-df-bit-treatment.ci` | parse | `df-bit copy` and `clear` accepted on a tunnel-mode IPv4 peer; every treatment no SA of the peer can apply refused |
| `test/parse/ipsec-dscp-map.ci` | parse | `dscp-map copy`, 0 and 63 accepted; 64 and a value on a transport-mode peer refused |
| `test/parse/ipsec-icmp-error-check.ci` | parse | `icmp-error-check` parses true and false, reads back, and refuses a non-boolean |
| `test/parse/ipsec-icmp-type-selector-refused.ci` | parse | an ICMP type and code-range entry accepted; every ICMP selector the entry cannot carry refused |

**Red proofs owed by the two scenarios**, with the image rebuilt for each private
fault (`docs/architecture/testing/interop.md`, "The outer DF bit and ICMP
errors on a Child SA (RFC 4301)"):

| Proof | Scenario | Break | Expected red | Observed |
|---|---|---|---|---|
| (a) | `tunnel-df-treatment` | `xfrmDFFlag` answers false for `DFClear` | the clear phase's DF-set burst arrives with DF set | RED at the earlier `show` readback ("ze reports df-bit ... on its outbound SA, want ..."); the wire-counter check was not reached |
| (b) | `tunnel-df-treatment` | `xfrmDFFlag` answers true for `DFCopy` | the copy phase's DF-set burst arrives with DF clear | RED at the same `show` readback; the wire-counter check was not reached |
| (c) | `icmp-error-payload-check` | `xfrmStateFromParams` no longer sets `state.ICMP` | the consistent error is not delivered, with strongSwan generation and Ze's inbound ESP carriage observed | OBSERVED 2026-11-16: clean PASS 10s → flag-cut packet FAIL 8s with 336 ESP octets decrypted but no Time Exceeded → clean PASS 10s. Generation/carriage guards passed; exact generation count is not printed. Logs `job-ipsec-icmp-clean-current-98342191.log`, `job-ipsec-icmp-flag-cut-current-98342191.log`, `job-ipsec-icmp-clean-after-flag-cut-98342191.log`. Independently CLEAR for current IPv4 single-pair local delivery |
| (d) | `icmp-error-payload-check` | P widens the inbound IPv4 tunnel protect policy's source in `xfrmPolicyFromParams`; S widens the inbound IPv4 tunnel state selector's source in `xfrmStateFromParams`, each to `0.0.0.0/0` | clean PASS, P alone PASS, S alone PASS, S+P packet-delivery RED, then clean PASS; together the faults deliver the inconsistent error | OBSERVED 2026-11-16: recovery clean PASS 10s → P PASS 10s → S PASS 10s → S+P packet FAIL 8s, delivering three inconsistent Time Exceeded messages → final clean PASS 10s. Consistent control and generation/carriage guards passed. Logs `job-ipsec-icmp-policy-only-cut-98342191.log`, `job-ipsec-icmp-state-only-cut-98342191.log`, `job-ipsec-icmp-combined-selectors-cut-98342191.log`, `job-ipsec-icmp-clean-after-selector-cuts-98342191.log`. Independently CLEAR for current IPv4 single-pair local delivery; no general scope or protected inner-fragment credit |

**Decisions recorded on 2026-11-12.**

| Decision | Who decided | Outcome |
|---|---|---|
| DF `set`, and a fixed DSCP 1 to 63, on Linux | owner (2026-11-12: "refuse add a spec which would write the kernel feature for post release") | keep the refusal at commit and install; the kernel feature is `plan/post-release/kernel/spec-xfrm-outer-df-set-dscp.md`; `RFC4301-8.1-2` and `RFC4301-5.1.2.1-1` stay honest `{partial}` rows naming it |
| VPP and `icmp-error-check` | rule (exact-or-refuse) | the default stays true, which is the RFC processing. VPP, reachable only through the test override `ze.test.ike.dataplane`, refuses at commit and names `icmp-error-check false` as the fix |
| A per-SA option a backend cannot apply | rule (exact-or-refuse) | refused at config verify (`verifyPeerSAOptions`) and at install with `dataplane.ErrSAOptionRefused`, never kept as a degraded SA |
| `RFC4301-6.2-3` condition: an ICMP error a bypass entry would carry in clear (an operator outbound bypass outranking a peer's protect policy, or the catch-all under `unmatched bypass`) is mapped into the return SA by Linux `icmp_route_lookup` and `icmpv6_route_lookup` | owner (2026-11-12: "refuse at commit") | (a): for each peer with `icmp-error-check true` and no `vti bind`, `unmatched bypass` and every out or both bypass entry covering ICMP error types towards the peer's remote selectors are refused by `validateIPsecSections` (commit verify and `ze config validate` alike), and logged at start under `config-plugin-verify` for a configuration no commit judged (`engine/icmp_overlap.go`). A VTI peer is exempt: its policies carry its if_id, and the kernel's reverse lookup runs at if_id 0 |
| An ICMP message arriving on a Child SA with `icmp-error-check true` bypasses an operator inbound discard when its embedded header matches the SA (the kernel's `icmp_rcv` reverse check) | owner (2026-11-12: "refuse at commit") | (a): for each peer with `icmp-error-check true` and no `vti bind`, every in or both discard entry covering ICMP of a family the peer carries is refused by `validateIPsecSections`, and logged at start (`engine/icmp_overlap.go`) |
| Where the two refusals run, and doctor | lead (rule: `ze config validate` refuses what commit refuses, `register.go` contract) | in the offline verifier `validateIPsecSections`, not a separate OnConfigVerify step; no dedicated doctor check, because the config-semantics check already reports a `validateIPsecSections` refusal under `config-plugin-verify` and a second check would report it twice |
| The five rows `./le rfc check` names as linux-limit candidates (`RFC4301-5.1.2.1-1`, `RFC4301-8.1-2`, `RFC4301-6.2-2`, `RFC4301-6.2-3`, `RFC4301-6.2-4`) | owner delegated (2026-11-12: "your opinion if you believe it is right, do it"); decided by the lead, checked by an independent judge | none is a kernel limit; see "Linux-limit verdicts, 2026-11-12" below |

**Linux-limit verdicts, 2026-11-12.** The rule: a requirement is a kernel
limit only when Linux lacks the primitive for the whole requirement, not when
Ze could meet it with state it installs. The lead decided each row; an
independent `ze-read` judge read Linux 7.3 (`tmp/session/2026-11-09-222726/scratch/kernelsrc73/`)
and agreed with all five. Both register entries move to `constrains design`, so
`./le rfc check` proposes none of these rows any more.

| Row | Verdict | Reason (judge's decisive source) | Row annotation |
|---|---|---|---|
| `RFC4301-8.1-2` | not a limit | XFRM cannot write DF set (`xfrm4_tunnel_encap_add`), but the outer ESP packet always reaches LOCAL_OUT (`xfrm_output_resume`: `skb_dst(skb)->ops->local_out`), where an nftables payload write with checksum update (`nft_payload_set_eval`) sets DF per SA; the IPv4 path sets no `ignore_df`, so set stays visible on the wire. Ze refuses by owner choice | `{partial}`, gap "set", naming the post-release kernel spec |
| `RFC4301-5.1.2.1-1` | not a limit | same mechanism, `ip dscp set` / `ip6 dscp set` on the outer packet after `INET_ECN_encapsulate` | `{partial}`, as above; the per-class mapping table stays Ze-owed |
| `RFC4301-6.2-4` | not a limit | the forwarded case fails because Ze's fwd catch-all is a policy: `if (!pol && dir == XFRM_POLICY_FWD) pol = xfrm_in_fwd_icmp(...)` runs before `policy_default[dir]` is read. The node fwd catch-all as the netns default (`XFRM_MSG_SETDEFAULT`) and the fwd interface catch-all removed (a secpath packet no policy matches is still rejected by `secpath_has_nontransport`) let it run | `{partial}`, gap Ze-owed (Work Not Done) |
| `RFC4301-6.2-3` | not a limit | local errors are mapped under discard (`icmp_route_lookup` continues on `-EPERM`); the bypass overlap is refused at commit; forwarded errors under `unmatched discard` have a stock-kernel design (ICMP-error allow entries above the out catch-all and operator discards, plus an nftables `rt ipsec missing` drop in forward and output, which `ip_forward` reaches after `xfrm4_route_forward`); no namespace test yet observes the mapping | `{partial}`: tested the bypass-overlap refusal, gap the mapping |
| `RFC4301-6.2-2` | not a limit | follows from 6.2-3 and 6.2-4 | `{gap}`, naming both as Ze-owed designs |
| Direction shown for Ze's own SAs: Ze set no `XFRMA_SA_DIR`, so `show` rendered the direction unknown and the options on both sides | DECIDED (a), Thomas, 2026-11-19 | (a) set the SA direction at install (kernel 6.10 and later), carried by AC-21; (b) and (c) set aside |

## Task

`docs/features/rfc-status.md` claimed RFC 4301 was `Supported` until 2026-08-30. An
extraction walk of `rfc/full/rfc4301.txt` found MUST-level architecture obligations Ze
does not meet, and none of them was on the checklist in `rfc/short/rfc4301.md`. The owner
decided on 2026-08-30 to correct the public row to `Partial` with the gaps disclosed and
to open this spec for the implementation work, one phase per block, security-bearing
blocks first.

RFC 4301 remains a partial implementation. The current public ledger derives
Proof, Evidence and Unmet from the summary and verified records; it no longer
authors the old literal `Supported` status. This spec owns the architecture
obligations in its phase table, with DISCARD, PAD and SPD-ordering deliveries
retained as completed history. Completing it cannot erase wanted feature gaps
or authorize a full-RFC support claim. The separate lifetime-volume release
fix and extraction/sign-off ownership remain; neither is silently waived by
the phase-9 publication correction below.

Historical preservation checkpoint, before the final product acceptance above:
the dual-form repair also owns ordinary preservation regressions for the
existing captured raw-sender authority contract, under its Task and AC-29.
The new `engine/raw_socket_fragment_policy_integration_linux_test.go` covers
IPv4/IPv6 socket-policy presence, selector misses and absence, marks and
bound-device authority, policy generations and socket lifetime, retaining
whole and same-authority controls in both fragment arrival orders. The file
is authored and independently reviewed CLEAR; formatting and Linux build
passed. Its packet run exposed the legitimate IPv4 whole-output route defect
recorded above. The normal C repair now has targeted acceptance in both failed
IPv4 cases; full populations on the new pair remain owed. This preserves the
existing architecture contract and adds
no RFC conformance claim.

**Fragment investigation.** `ikeBypassPolicies` installs port-scoped UDP
500/4500 BYPASS policies. The original draft inferred an exploitable
non-initial-fragment bypass from the absence of fragment state in Ze. The final
2026-09-06 entry retracts that inference as a settled gap. Establish the
kernel and VPP reassembly and policy-check behaviour before treating it as a
vulnerability or selecting the Section 7.4 enforcement mechanism.

**Requirement ids this spec owes (owner ruling, 2026-09-21).** Every MUST is a
requirement; each row below carries `{gap: ...; plan/release/fixes/spec-rfc4301-architecture-gaps.md}`
in `rfc/short/rfc4301.md` until its phase lands a tagged pair.

| Phase | Requirement ids | Producer or absence |
|---|---|---|
| 3, Section 6 ICMP | RFC4301-6-1, RFC4301-6.1.1-1, RFC4301-6.1.1-2, RFC4301-6.1.2-1, RFC4301-6.2-1, RFC4301-6.2-2, RFC4301-6.2-3, RFC4301-6.2-4, RFC4301-6.2-5 | No ICMP control in `internal/component/ike/ipsec`; the operator list accepts a `protocol 1` or `58` entry with no type or code. Whether the kernel maps an outbound ICMP error to the return SA (6.2-3) is unverified and is this phase's first lab step |
| 4, Section 4.4.1.1 ICMP type/code selectors | RFC4301-4.4.1.1-4 | `dataplane.SPParams` carries no ICMP type or code field |
| 7, Section 8 DF bit and PMTU | RFC4301-8.1-1, RFC4301-8.1-2, RFC4301-8.2.1-1, RFC4301-8.2.2-1, RFC4301-5.1-3 | `dataplane/xfrm_linux.go::xfrmStateFromParams` sets no DF flag; `dataplane.SAParams` holds no PMTU |
| 8, Section 5.1.2.1 outer-header DSCP | RFC4301-5.1.2.1-1 | `xfrmStateFromParams` sets no DSCP mapping |

**Why the block list is not the whole RFC.** Two blocks the walk touched are homed
elsewhere and are NOT re-implemented here. The Section 4.4.2.1 byte-count SAD lifetime is
designed in `plan/release/fixes/spec-ipsec-lifetime-volume.md`, which this spec depends on. The
extraction sign-off artifact `rfc/extraction/rfc4301.json` belongs to
`plan/release/validation/spec-rfcgate-6-supported-extraction-signoff.md`, and this spec's last phase lands
under that spec's rules rather than replacing them.

## Required Reading

The discovery notes below record the earlier source and documentation reads.
Their old absence and summary-status claims are historical. The current
checkpoint identifies the completed product work; remaining architecture work
uses the current owning documents and source.

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - the Child SA to XFRM projection, declared by `dataplane.go`, `child.go` (and `bypass.go` until `e700fb7392` deleted it)
  → Decision: [what the page states about how an SPD entry reaches the kernel]
  → Constraint: [what the page says a backend that cannot express a policy must do]
- [ ] `docs/architecture/ike/ipsec-dataplane-inspection.md` - how installed policy and state are read back
  → Decision: [what the page states about drift detection between Ze and the kernel]
- [ ] `docs/guide/ipsec.md` - the operator-facing IPsec surface every new config leaf lands in
  → Constraint: [the config grammar a new SPD surface must match]
- [ ] `docs/architecture/testing/interop.md` - the strongSwan suite and the vacuity traps
  → Constraint: [the red-phase proof each new scenario owes]

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4301.md` - the current checklist: 104 requirement ids on 2026-11-14, with 25 `{gap}`, 7 `{partial}`, 12 `{single-polarity}` and 4 `{lower-layer}` annotations (18 ids and one `{gap}` when this spec was written)
  → Constraint: the summary's Purpose line is still wrong about the implementation, and five unannotated rows disagree with this spec's evidence ("Summary rows that disagree with this spec")
- [ ] `docs/architecture/linux-limitations.md` - `LL-XFRM-FRAGMENT-SELECTOR`, `LL-XFRM-ICMP-TRANSIT`, `LL-XFRM-STATE-ONE-SELECTOR`, `LL-XFRM-OUTER-DF-SET-DSCP`
  → Constraint: a forwarded fragment is classified with port 0 and ICMP type 0 code 0, before any reassembly, and one XFRM state carries one selector
- [ ] `rfc/short/rfc7296.md` - traffic-selector negotiation and narrowing, which feeds every SPD entry IKE creates
  → Constraint: a selector Ze puts on the wire is the selector Ze programs, so a new selector field is negotiated or refused, never rounded outward
- [ ] `rfc/short/rfc4303.md` - ESP, the transform an SPD PROTECT entry names

**Key insights:** (minimal context to resume after compaction)
- The RFC's own text is the authority. `rfc/short/rfc4301.md` is a derived artifact and disagrees with the code today.
- Every obligation below was read in `rfc/full/rfc4301.txt` and every producer was read in the tree on 2026-08-30.

## Current Behavior (MANDATORY)

This source inventory records the 2026-11-14 snapshot. The later product closure
supersedes its missing scope-writer and fragment-guard status. It supplies no
new verdict on this spec's other defect findings. Read again on 2026-11-19 (evening),
five lines no longer hold: `dataplane.go` has no `PriorityIKEBypass` and adds
`PriorityTransitGate` and `PriorityICMPErrorReturn` above the catch-all;
`engine/bypass.go` is deleted (`e700fb7392`); `engine/spd_policy.go` projects an
entry onto the forward path (`spdTransitPolicies`, `spdEntryPolicies`) and reconciles
through `reconcileSPDPolicies`, which returns its failure; `ipsec/spd_policy.go`
judges an order against the rank range 1 to 2147483647 (`spdRankValid`); and the
YANG `order` and `policy-priority` leaves take 1 to 2147483647.

**Source files read:** (first read on 2026-08-30; each line states what the file holds on 2026-11-14)
- [ ] `internal/component/ike/dataplane/dataplane.go` - `SPAction` holds `SPActionProtect` (0), `SPActionBypass` (1) and `SPActionDiscard` (2), and `isTemplateFree` answers for the last two. `PortMatch` holds a port and a mask. The priorities are `PriorityIKEBypass` 100, `PriorityChildSA` 2000, `PriorityUnmatchedXFRMInterface` and `PriorityUnmatched` (the largest u32); a lower value is searched first, and the kernel resolves equal priorities by insertion order.
- [ ] `internal/component/ike/engine/bypass.go` - `ikeBypassPolicies` builds four BYPASS policies per address family at `PriorityIKEBypass`: an out pair matching the UDP source port 500 or 4500 with any destination port, and an in pair with the ports reversed. Source and destination are the any-address prefix. It installs no forward entry. `installIKEBypass` logs a failed install and continues.
- [ ] `internal/component/ike/engine/spd_policy.go` - `spdPolicyParams` turns one operator entry into out and in policies, never a forward one. `installSPDPolicies` removes what left the configuration, then installs what joined it, in map order, and logs an install failure without returning it (D-1, D-2, D-3).
- [ ] `internal/component/ike/ipsec/spd_policy.go` - `parseSPDPolicy` defaults order 1000 and direction both. `parseSPDPort` accepts ANY or one port. `SPDPolicy.validate` judges each entry alone: both prefixes, one family, an order above 100, a port only under a protocol with ports, and the ICMP selector rules of `validateICMP` and `validateICMPFragments`.
- [ ] `internal/component/ike/engine/unmatched.go` - `installUnmatched` installs the in and out catch-all and, on XFRM, sets the namespace forward default (`ForwardDefaultInstaller`); a failure fails the apply. `refuseImplicitDiscard` refuses a commit that turns the default discard on unsaid.
- [ ] `internal/component/ike/engine/icmp_overlap.go` - `refuseICMPErrorBypass` and `refuseICMPCheckDiscard`, run by `verifyICMPOverlap` as the last step of `validateIPsecSections`; `warnICMPOverlapAtStart` logs the same conflict for a configuration the daemon starts with.
- [ ] `internal/component/ike/engine/sa_options.go` - `peerSAOptions` sets `ICMPErrorCheck` true unless the peer writes `icmp-error-check false`; `verifyPeerSAOptions` refuses an option the loaded backend cannot apply.
- [ ] `internal/component/ike/engine/child.go` - `createFirstChildSA` sets the Child SA mode to tunnel, and to transport when `sa.UseTransportMode` is set. `childPolicyParams` builds the `SPParams` for a negotiated Child SA.
- [ ] `internal/component/ike/engine/remote_id.go` - `remoteIDMatches` compares an asserted identity with the configured `remote-id` after a class gate. ID_FQDN and ID_RFC822_ADDR compare with `asciiEqualFold`, ID_DER_ASN1_DN renders to RFC 4514 and compares with `asciiEqualFold`, ID_KEY_ID compares byte for byte, and an address parses through `netip.ParseAddr` and compares as one value.
- [ ] `internal/component/ike/engine/rekey.go` - `lifetimeState` holds `softTime`, `hardTime`, `softBytes` and `byteCount`. `newLifetimeState` assigns `softTime` and `hardTime` only. `softExpired` reads `softBytes` in its second arm, and that arm is unreachable in a running daemon because nothing assigns the field outside tests.
- [ ] `internal/component/ike/engine/initiator.go` - `tsToIPNet` is deleted; a comment at the deletion site records that `wireToSelectors` in `ts_narrow.go` replaced it, converts every selector, and keeps the port and the protocol.
- [ ] `internal/plugins/ospf/ipsec_install.go` - the RFC 4552 manual path installs transport-mode ESP or AH through the same dataplane seam.
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - the operator config surface. `vpn ipsec` carries `esp-group`, `ike-group`, `remote-access` and `site-to-site peer`, and a peer carries a `traffic-selector` list with `local` and `remote` prefixes and ports, plus `policy-priority`, `df-bit`, `dscp-map` and `icmp-error-check`. The `policy` list is the operator SPD: `action` (bypass or discard, mandatory), `order` (101 to 2147483647, default 1000), `direction` (out, in, both; default both), `protocol`, `icmp-type`, `icmp-code`, and a `local` and `remote` prefix and port. Its `name` leaf says commit refuses two entries whose selector, direction and order agree; no check does (D-2).
- [ ] `docs/features/rfc-status.md` - the RFC 4301 row, now `Partial`, naming this spec.
- [ ] `rfc/short/rfc4301.md` - 104 requirement ids, no `{not-applicable}` annotation any more, and a Purpose line that still says Ze delegates IKEv2 negotiation to strongSwan and charon.

**Behavior to preserve:**
- Ze's own IKE exchange stays exempt from IPsec: a Child SA policy that captured IKE would prevent its own rekey and teardown. What is preserved is that exemption for traffic this node sends and receives. The out entry's reach over forwarded traffic is not preserved: it is defect D-4 (AC-17).
- `SPActionProtect` stays the zero value of `SPAction`, so a caller that forgets the field gets a policy that protects rather than one that passes traffic in the clear.
- A backend that cannot express a policy refuses the install rather than widening or downgrading it.
- Every existing `test/interop-ipsec/scenarios/` scenario keeps passing against strongSwan.

**Behavior to change:**
- An SPD entry can carry a DISCARD action, an ICMP type and code selector, and a port range.
- A BYPASS entry with a non-trivial port range is served by stateful fragment checking, or its install is refused.
- An administrator can accept or reject unauthenticated ICMP error messages at ICMP-type granularity, on both sides of the IPsec boundary.
- A protected transit ICMP error message can be checked for consistency between its payload header and the traffic selectors of the SA that carried it.
- An administrator can order SPD entries totally through the management interface.
- The DF treatment of a tunnel-mode SA is configurable as clear or copy from the inner header; set is refused at commit (owner decision 2026-11-12).
- An operator entry that cannot be installed fails the commit, equal orders on overlapping entries are refused, an operator entry decides forwarded traffic, and the IKE bypass stops matching forwarded traffic (D-1 to D-4).
- A non-error ICMP message on a Child SA is held to the SA's selectors, and a discard entry is accepted beside a peer in the default configuration (D-5, D-6).
- A per-SA PMTU is held and aged.
- The outer tunnel header's DSCP value is mapped for the domain the packet enters.
- PAD identity matching accepts a sub-tree for a DN, a DNS name and an RFC 822 address, and an address range.

## Existing Mechanisms (MANDATORY)

The table preserves the mechanisms and implementation status at the dated
reads below. Its pending kernel-route decision and absent engine scope writer
are historical; the current checkpoint records their completed product work.

The first six rows were checked against the source on 2026-11-10, and the
operator SPD row again on 2026-11-14. The rows after them were added on
2026-11-14 from the producing functions and the design page
`docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| XFRM selector port with mask | `internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch` (adds `XfrmPolicy.SrcPortMask` and `DstPortMask`; `selFromPolicy` writes them, `parseXfrmPolicy` reads them back); `dataplane.PortMatch` (`dataplane.go`) | Carries a selector port as a value plus a mask, so a mask-aligned block is programmed exactly and read back exactly | Extends. Phase 4 puts the ICMP type in the source port and the code in the destination port with their masks (`dataplane.IsICMPProto`), and splits a code range into mask-aligned blocks, one policy each (`spdICMPMatches`, engine `spd_policy.go`). No new selector field and no rounding outward |
| Operator SPD entry list | `internal/component/ike/ipsec/spd_policy.go` (`SPDPolicy`, `ICMPSelector`, `parseSPDPolicy`, `parseICMPSelector`); `internal/component/ike/engine/spd_policy.go` (`spdPolicyParams`, `installSPDPolicies`) | `vpn ipsec policy <name>` with action, order, direction, protocol, local and remote prefix and port, and an ICMP type and code; each entry becomes out and in policies, a both-direction entry a mirrored pair | Extends. Phase 4 added the ICMP selector to this list rather than a second one (R-3). It still installs out and in only, never fwd, which is the open `RFC4301-6-1` gap. AC-14 to AC-16 extend the same list and the same reconciler: an install failure becomes an apply error, the validator gains the equal-order refusal, and `spdPolicyParams` gains the forward projection. No second list and no second installer |
| Dataplane SA and SP seam with exact-or-refuse | `dataplane.SAParams` (`DF`, `OuterDSCP`, `ICMPErrorCheck`), `dataplane.SPParams` (`ICMPErrorReturn`), `outerHeaderTreatmentError` and `ErrSAOptionRefused` (`dataplane/outer_header.go`); `xfrmNoPMTUDisc`, `xfrmDontEncapDSCP`, `xfrmStateFromParams` (`xfrm_linux.go`); `vppDFError`, `vppICMPErrorCheckError` (`vpp.go`), `vppICMPSelectorError` (`vpp_policy.go`) | Each backend programs an option exactly or refuses the install with `ErrSAOptionRefused`, which does not wrap `ErrNotSupported`, so `isXFRMUnsupported` (`engine/child.go`) refuses the Child SA rather than keeping it degraded | Extends. Phases 3, 7 and 8 added the three per-SA options and the return-SA mapping flag to the existing value types, and every refusal goes through the existing error rather than a new path |
| Commit-time refusal of what the backend cannot enforce | `verifyUnmatchedEnforceable` (`engine/unmatched.go`, through `dataplane.CatchAllInstaller`); `verifyPeerSAOptions` (`engine/sa_options.go`, through `dataplane.SAOptionChecker`) | The catch-all check refuses `unmatched discard` at verify when the backend cannot hold it; the per-peer check refuses a `df-bit`, `dscp-map` or `icmp-error-check` value the loaded backend cannot apply, from the same predicate the install path uses | Extends. `verifyPeerSAOptions` mirrors `verifyUnmatchedEnforceable`: a backend that implements no checker has declared nothing, and anything but the defaults is refused on it |
| Dataplane readback | `internal/component/ike/cmd/show_dataplane.go` (`saInfoToMap`, `saInfoRow`, `handleShowVPNIPsecDataplanePolicy`) | `show vpn ipsec dataplane sa` renders `df-bit`, `dscp-map`, `icmp-error-check` and `options-known`, null when unread, and null on the side an option cannot ride when the SA's direction is known | Extends. The interop checkers read the outbound SA's `df-bit` here. An SA with no recorded direction renders every option as read (PENDING OWNER row in the 2026-11-10 decisions) |
| Kernel route PMTU exceptions | Linux `__ip_rt_update_pmtu` (net/ipv4/route.c), `rt6_do_update_pmtu` (net/ipv6/route.c), aged by `net.ipv4.route.mtu_expires` and `net.ipv6.route.mtu_expires` (registered in `internal/core/sysctl/known_linux.go`) | The kernel learns a path MTU under the XFRM bundle and expires it back to the link MTU after the period | Does not apply as a Ze mechanism to extend: Ze holds no PMTU of its own. Phase 7 relies on it for Section 8.2.2 instead of the planned `engine/pmtu.go` (A-4), with the period set through the existing sysctl surface, and records it as a kernel-producer escape |
| Final disposition with a fail-closed apply | `installUnmatched` (`engine/unmatched.go`); `dataplane.ForwardDefaultInstaller`, `xfrmBackend.ApplyForwardDefault` (`dataplane/xfrm_forward_default_linux.go`); doctor check `ipsec-forward-default` | Installs the in and out catch-all and sets the namespace forward default, rolls the default back when a later install fails, and returns an error that fails the apply | Extends. AC-14 gives `installSPDPolicies` the same contract the catch-all already has: an entry the operator committed and the dataplane does not hold is an apply error, not a log line |
| Commit-time refusal of an entry the kernel overrides | `verifyICMPOverlap`, `refuseICMPErrorBypass`, `refuseICMPCheckDiscard` (`engine/icmp_overlap.go`), a step of `validateIPsecSections` | Refuses a bypass that would carry an ICMP error in clear, and an inbound discard matching ICMP, beside a peer with `icmp-error-check` true and no `vti bind` | Replaces in part. AC-18 and AC-19 remove the cause of refusal B (the reverse check running for every ICMP type) under owner decision 4, so the refusal that makes a default discard uncommittable goes with it. Refusal A stays |
| Selector on the inbound state | `tunnelStateSelectorSupported`, `inboundStateSelector` (`engine/child.go`); `dataplane.InboundStateSelectorInstaller` | A single-pair Child SA carries its negotiated selector on the inbound XFRM state, so the kernel refuses a decrypted packet outside it before the SPD | Extends. A multi-pair SA has no single selector (`LL-XFRM-STATE-ONE-SELECTOR`); the state scope below is the same idea for several pairs (AC-20) |
| Exact per-state selector scope | `dataplane.SAStateScope`, `NewSAStateScope` (`dataplane/state_scope.go`); `xfrmStateScopeFromParams`, `checkXFRMStateScopeCapabilities` (`dataplane/xfrm_state_scope_linux.go`); `xfrmStateScopeFromAttrs` (`dataplane/xfrm_state_scope_abi_linux.go`) | Builds, writes and reads back an OR-set of up to 64 directional selectors as a private netlink attribute, and refuses any scope unless the kernel's receipt shows the ABI and feature bits 0 to 3. No engine function sets `SAParams.Scope` | Extends. Phase 2b adds the engine writer (AC-20). It does not add a fragment table in Go: the planned `engine/fragment.go` is dropped, because the fragment state belongs to the kernel's reassembly queues |
| Ze's kernel patch series | `gokrazy/kernel/patches/series`; `gokrazy/kernel/proposals/`; `internal/appliance/kernel.version` | The runtime kernel is Linux 7.3 plus the enrolled patches, built by the appliance kernel builder; a proposal outside the series is not built | Extends. K1 and K2 are entries 0003 and 0004; K3 stays a proposal until owner decision 1 |
| Linux limitation register | `docs/architecture/linux-limitations.md`; `TestLinuxLimitsCheckedAtShippedVersions` | Records each kernel behavior that constrains a design, with the function read and the release it was read at | Extends. AC-17 owes `LL-XFRM-FRAGMENT-SELECTOR` the observed forward and out lookup order, and a lifted entry when the kernel route lands |
| SPD ledger and ordered release | `removeZePolicy` and `releaseSPD` (design page, "SPD dispositions") | Records a removal that left an entry installed and keeps the catch-all beneath it at shutdown | Does not apply to the install side. AC-14 keeps removal as it is and changes only what a failed install returns |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator config under `vpn ipsec` in `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`, parsed into the ipsec config types.
- IKEv2 traffic selectors arriving on the wire, narrowed by `wireToSelectors` and `ts_narrow.go`.
- ICMP error messages arriving on the unprotected side, on the protected side, and inside an SA.
- IP fragments arriving at an interface a BYPASS policy covers.

### Transformation Path
1. Config or negotiation produces a policy description in the ipsec config types.
2. `childPolicyParams` and the OSPFv3 installer build `dataplane.SPParams` and `dataplane.SAParams`.
3. The backend selected at runtime writes them: `xfrm_linux.go` through netlink, `vpp_policy.go` through the VPP API.
4. The kernel or VPP classifies each packet against the installed SPD and SAD.

### Boundaries Crossed

The verification column is the 2026-11-14 snapshot. Later product receipts
supersede its missing scope writer, unobserved private attributes and local
ICMP interop RED. Remaining architecture obligations retain their own proof.

| Boundary | How | Verified |
|----------|-----|----------|
| Config to engine | YANG tree to ipsec config value types | Partly: the parse `.ci` files (`ipsec-icmp-type-selector-refused`, `ipsec-df-bit-treatment`, `ipsec-dscp-map`, `ipsec-icmp-error-check`) prove the leaves; no `.ci` commits a discard entry or two entries of equal order |
| Engine to dataplane | `dataplane.SPParams` and `dataplane.SAParams` value types | Partly: `TestDiscardPolicyReachesTheBackend`, `TestSPDPolicyMirrorsTheInboundSelector`, `TestDFBitTreatmentReachesTheBackend`. Not verified: a failed install reaching the caller (D-1), the forward projection (D-3), `SAParams.Scope` (no writer) |
| Dataplane to kernel | netlink XFRM messages, plus the private attributes `XFRMA_STATE_SCOPE` and `XFRMA_ZE_CAPS` | Partly: `test/ipsec/ipsec-icmp-type-selector.ci`, `ipsec-sa-options-kernel.ci` and `ipsec-show-dataplane-kernel.ci` read the real SPD and SAD back. Not verified: the private attributes on a kernel built with the series |
| Dataplane to VPP | VPP binary API | No: reachable only through the test override `ze.test.ike.dataplane`; units only |
| Ze to peer | IKEv2 traffic selector payloads | Partly: `tunnel-df-treatment` passes; `icmp-error-payload-check` is red as committed |

### Integration Points
- `dataplane.SPAction` - gains a DISCARD member, and every backend must express it or refuse.
- `dataplane.SPParams` - gains ICMP type and code selector fields and a port range form.
- `dataplane.SPParams.Priority` - stops being two fixed constants and carries an operator order.
- `remoteIDMatches` - gains sub-tree and range matching.
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - gains the new leaves.
- `internal/core/diagnostic/codes.go` - gains a code for each new runtime dependency a doctor check reads.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes, with one owner-decided path | At the 2026-11-14 read `installSPDPolicies` dropped an install failure (D-1); since D-1 landed `reconcileSPDPolicies` returns it to `applyConfig`. A configuration the daemon starts with still skips `validateIPsecSections` and is installed with an Error log and a doctor finding, by owner decision 3 (a) (`installSPDPoliciesAtStart`). Everything else goes config, `validateIPsecSections`, `applyConfig`, `dataplane.Dataplane` |
| No unintended coupling (components stay isolated) | Yes | `ike/dataplane` stays the ungated seam OSPF also uses (`ai/doors/access-security.md`); the engine reaches the backend only through `dataplane.Dataplane` and the optional interfaces `CatchAllInstaller`, `SAOptionChecker`, `ForwardDefaultInstaller`, `InboundStateSelectorInstaller`; `iface` is reached through its resolver (design page, "SPD dispositions") |
| No duplicated functionality (extends existing, does not recreate) | Yes, with one planned duplicate dropped | The ICMP selector, the per-SA options and the fixes of AC-14 to AC-16 extend the one operator SPD list (Existing Mechanisms). The planned `engine/icmp.go`, `engine/pmtu.go` and `engine/fragment.go` would each have duplicated a kernel mechanism and are not written |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no wire encoding path is touched; policies and states are small value types built at config or negotiation time, not per packet |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | a backend declares what it can hold through the optional interfaces above, and the engine asks the loaded backend, never its name (`verifyUnmatchedEnforceable`, `verifyPeerSAOptions`, `forwardDefaultInstaller`). The doctor check registers from `register_forward_default_linux.go`. Not checked: whether `show vpn ipsec dataplane` needs an edit for the forward projection of AC-16 |

### Changed-Wire Census

This is the historical census from the source read below. Its unused scope
writer and incomplete capability receipt descriptions predate the product fix.

**Search evidence:** searched `internal/component/ike/` and `gokrazy/kernel/`
for `XFRMA_STATE_SCOPE`, `XFRMA_ZE_CAPS`, `xfrmaStateScope`, `xfrmaZECaps`,
`NewSAStateScope` and `ExtraAttrs` on 2026-11-14. Positive control: the search
finds the attribute numbers in both `0003-xfrm-state-scope.patch` and
`xfrm_state_scope_abi_linux.go`. No IKEv2 payload, ESP framing or plugin RPC
changes; the operator config gains a refusal and changes no leaf.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| XFRM netlink state attribute `XFRMA_STATE_SCOPE` (42) | writer | `xfrmStateScopeFromParams`, through `XfrmState.ExtraAttrs` of the vendored netlink library | the attribute is appended only when `SAParams.Scope` is set, which no engine code does | new; unused until AC-20 |
| XFRM netlink state attribute `XFRMA_STATE_SCOPE` (42) | reader | the kernel's `xfrm_user.c` as patched by `0003-xfrm-state-scope.patch`; `xfrmStateScopeFromAttrs` on a dump | a stock kernel does not know the attribute, so Ze checks the receipt first | new; a kernel without the receipt gets no scope, by refusal |
| XFRM default-policy answer attribute `XFRMA_ZE_CAPS` (43) | writer | the kernel's `xfrm_user.c` as patched by 0003 and 0004 | 0004 sets the fragment-context bit; the handover of 2026-11-13 records bits 0 to 2 advertised | new, read-only |
| XFRM default-policy answer attribute `XFRMA_ZE_CAPS` (43) | reader | `checkXFRMStateScopeCapabilities` (`dataplane/xfrm_state_scope_linux.go`) | an absent receipt is a refusal, never read as no features needed | new |
| Operator config `vpn ipsec policy` | writer | the operator, `ze-ipsec-conf.yang` | the list exists; AC-15 refuses a pair it accepts today | configs in the repository with two overlapping entries of equal order are converted in the same work (`ai/META-DECISIONS.md`, "Ze is pre-release") |
| Operator config `vpn ipsec policy` | reader | `parseSPDPolicy`, `ValidateSPDPolicies` | read on 2026-11-14 | preserved, plus the new refusal |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | DISCARD was absent in the 2026-08-30 baseline | original `SPAction` read | phase 2a would be smaller if already implemented | 2026-09-06 delivery record above | confirmed then cleared by phase 2a; retained as history |
| A-2 | Ze holds no IP fragment state anywhere in the IPsec path | a case-insensitive search for `fragment` over `internal/component/ike/` returned only EAP-TLS message fragmentation, 2026-08-30 | phase 2 has an existing mechanism to extend rather than to add | re-run the search at the start of phase 2 and read every hit | confirmed for the engine; since 2026-11-13 the kernel patch K2 holds fragment context, in the kernel and not in Ze |
| A-3 | The Linux kernel may not discharge Section 7.4 for a template-free XFRM policy on its own | hypothesis from the absence of a fragment selector field; kernel reassembly and policy order remain unproven | the obligation may be kernel-delegated and need evidence rather than new fragment state | read the producing kernel path and run the QEMU non-initial-fragment scenario before selecting enforcement | confirmed by reading, not by a packet: `LL-XFRM-FRAGMENT-SELECTOR` (verified at Linux 7.3, 2026-11-10) records that the forward path classifies a fragment with port 0 before any reassembly, and that traffic for this node is reassembled first. The QEMU fragment probes are written and not yet observed |
| A-4 | The Section 8.2.2 PMTU aging obligation is met by the kernel's own PMTU expiry for an XFRM route | the kernel ages a dst entry's PMTU, and Ze holds no PMTU value of its own | the phase must add a per-SA PMTU field, its aging timer and its reset period | read the producing kernel path (`__ip_rt_update_pmtu`, net/ipv4/route.c; `rt6_do_update_pmtu`, net/ipv6/route.c), then the namespace tests `TestXFRMTunnelPMTUAgesBackToTheLinkMTU`, `TestXFRMPolicyPMTUAgesBackToTheLinkMTU` and `TestXFRMTunnelPMTUAgesBackToTheLinkMTUOverIPv6` (`internal/component/ike/dataplane/rfc4301_pmtu_*integration_linux_test.go`), recorded as kernel-producer escapes | validated by the kernel read; the 2026-11-10 evidence table records the package runs and the QEMU IPv6 aging unit green |
| A-5 | The seven `{not-applicable}` annotations on `rfc/short/rfc4301.md` are void under the owner directive of 2026-07-27 and must be re-answered rather than cited | `ai/rules/rfc-compliance.md`, "Every earlier answer that pointed away from full compliance or full proof is VOID" | the annotations stand and phase 9 shrinks to a prose repair | read each annotation's producing function and record the fresh answer | overtaken: on 2026-11-14 the file holds no `{not-applicable}` annotation. What remains is the five rows of "Summary rows that disagree with this spec" |
| A-6 | The VPP backend can express a DISCARD policy | 2026-09-06 read of `IPSEC_API_SPD_ACTION_DISCARD` in the VPP API binding | a backend without the action must refuse rather than substitute | session evidence above | confirmed: `vppSPDAction` maps it |
| A-7 | No existing interop scenario would break when `SPAction` gains a member and `SPParams` gains fields | the zero value of every new field means "as before", and `SPActionProtect` stays 0 | a landed scenario reds and the change is wider than one enum member | run the whole `test/interop-ipsec` suite at the end of phase 2 | unvalidated |
| A-8 | Linux looks a forwarded packet up in the forward direction and then in the out direction, and applies the namespace forward default to a cleartext packet no forward entry matches | `LL-XFRM-FRAGMENT-SELECTOR` quotes `ip_forward` calling `xfrm4_policy_check` for forward and then `xfrm4_route_forward`; the design page says forwarded traffic is looked up "in fwd and out" | D-4 is not reachable, or ordinary transit under `unmatched discard` is refused for a policy-based peer | `TestRFC4301TransitFromIKEPortIsNotBypassed` with its positive control, in the QEMU guest | unvalidated: the kernel source is not in the checkout |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A phase classifies an obligation `{not-applicable}` because the kernel performs the packet-level half, and the gate goes green over an unmet MUST | a new `{not-applicable}` annotation appears in `rfc/short/rfc4301.md` | `ai/rules/rfc-compliance.md` reserves that call to the owner. The kernel performing a step is evidence to put to him, never a classification a phase makes. Ask which way to fix it |
| R-2 | The fragment work grows into a general connection-tracking subsystem | phase 2 starts naming a new package outside `internal/component/ike/` | the obligation is scoped to BYPASS and DISCARD entries with a non-trivial port range. Reassembly for exactly those entries is the deliverable (`ai/rules/simplicity.md`) |
| R-3 | A new SPD config surface duplicates the `traffic-selector` list a peer already carries | two config paths describe one selector | one declaration, and the peer selector list derives from it or names it. `ai/rules/principles.md` forbids the second copy |
| R-4 | An operator order or a Child SA policy captures Ze's own IKE exchange | an interop scenario stops rekeying | superseded on 2026-11-19 by owner order (`50b89fb51b`): no SPD rank is reserved for IKE. The exchange is exempt on Ze's own IKE sockets, which the kernel consults before any SPD entry whatever its rank (`TestRFC4301IKESocketsCarryTheirOwnBypass`), so an order from 1 to 2147483647 cannot capture it. The rekey scenarios of R-8 are the regression set |
| R-5 | A completion report hides unmet RFC obligations or treats an owner-chosen refusal as conformance | phase 9 tries to clear every gap or restore the retired literal Supported status | AC-12 requires truthful derived disclosure and enforced evidence, preserving named feature debt and the separate lifetime-volume/sign-off owners; no unilateral publication-scope waiver |
| R-6 | The ICMP block is large enough to exceed one agent, and gets trimmed to fit | an agent reports partial coverage of Section 6 | the package boundary is the BLOCK. An agent whose block is too big reports the size to the main thread, which re-cuts by section (`ai/rules/planning.md`) |
| R-7 | A new selector field is programmed but never negotiated, so Ze installs a policy the peer never proposed | an interop scenario shows a selector mismatch | RFC 7296 Section 2.9 narrowing is the existing rule: a selector Ze programs is a selector Ze negotiated, or the install is refused |
| R-8 | The D-4 repair narrows the IKE bypass and breaks a NAT or MOBIKE case the any-address form exists for | a real-NAT or MOBIKE interop scenario stops rekeying | the repair is the owner's 2026-11-19 route: per-socket XFRM policies on Ze's own IKE sockets, which follow the socket through a NAT rebinding or a MOBIKE move because they name no address, and never match a forwarded packet; the rekey half of `TestRFC4301TransitFromIKEPortIsNotBypassed` and the `real-nat-*` and `mobike-*` scenarios are its regression set |
| R-9 | A summary row keeps publishing as proven while this spec says the behavior is absent | `./le rfc check` is green and the public page lists the row as proven | the five rows are named with the annotation each should carry; the main thread routes that edit, and it does not wait for the fix |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Live IPsec sessions. A wrong SPD action black-holes traffic an operator asked to pass, and a wrong bypass passes traffic an operator asked to protect. A wrong DF or PMTU setting silently drops large packets. A wrong PAD match authorizes a peer the operator did not name |
| How is it reverted? | Per phase, by one commit. The enum member and the new selector fields are additive and their zero values are the previous behavior, so a revert of a later phase does not disturb an earlier one. The ledger row in phase 9 is one line |
| Who else touches this path? | `plan/release/fixes/spec-ipsec-lifetime-volume.md` owns `lifetimeState`. `plan/release/validation/spec-rfcgate-6-supported-extraction-signoff.md` owns `rfc/extraction/rfc4301.json` and the `supportClaimingScope` list in `internal/le/rfc/check_test.go`. `plan/release/validation/spec-ipsec-0-umbrella.md` and the other `plan/spec-ipsec-*` specs work the same engine |

## Wiring Test (MANDATORY -- NOT deferrable)

Every test in this table exists in the tree on 2026-11-19 (evening). The last
five were written for D-1 to D-5; the current checkpoint records their runs.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set vpn ipsec policy <name> action discard` committed | → | `parseSPDPolicy`, `spdPolicyParams`, `reconcileSPDPolicies`, carrying `SPActionDiscard` | `TestDiscardPolicyReachesTheBackend` |
| A forwarded non-initial fragment between the prefixes of a port-scoped operator entry | → | the kernel classifier over the entries `reconcileSPDPolicies` installed | `TestRFC4301ForwardedFragmentsRespectOperatorDisposition` |
| `set vpn ipsec policy <name> protocol 1 icmp-type <n> action discard direction in` committed | → | `parseICMPSelector`, `spdICMPMatches` | `TestSPDICMPTypeControlRejectsOrAcceptsOneType` |
| A forwarded ICMP error on a Child SA whose embedded header contradicts the SA selectors | → | `SAParams.ICMPErrorCheck`, `SPParams.ICMPErrorReturn` and the namespace forward default | `TestRFC4301ForwardedICMPUsesPayloadPolicy` |
| `set vpn ipsec policy <name> icmp-type <n> icmp-code <n>` committed | → | the ICMP selector in the port fields of `SPParams` | `TestICMPSelectorReachesTheBackend` |
| A peer asserting an identity beneath a configured sub-tree | → | `remoteIDMatches` sub-tree arm | `TestPadSubtreeAdmitsAPeerBeneathIt` |
| `set vpn ipsec policy <name> order <n>` and `site-to-site peer <name> policy-priority <n>` committed | → | `SPParams.Priority` from the operator order | `TestSpdOperatorOrdersOverlappingPeers` |
| `set vpn ipsec site-to-site peer <name> df-bit copy` committed | → | the DF treatment on the installed SA | `TestDFBitTreatmentReachesTheBackend` |
| `set vpn ipsec site-to-site peer <name> dscp-map <value>` committed | → | the outer-header DSCP mapping | `TestOuterHeaderDSCPReachesTheBackend` |
| `./le rfc check` over the closing tree | → | `checkStatusAgreement` and the derived proof/unmet figures | `TestStemFiguresListUnmetIDs`, `TestOneSummaryRendersExactlyOneStatusRow` |
| A committed operator entry the dataplane refuses | → | `reconcileSPDPolicies` returning the failure to `applyConfig` | `TestRFC4301FailedOperatorEntryInstallRefusesTheApply` |
| Two overlapping entries of equal order committed | → | `verifySPDOrder` (`engine/spd_order.go`) | `TestSPDPolicyEqualOrderOnOverlapRefused` |
| Traffic forwarded between a protected host and the outside, under an operator entry | → | the forward projection `spdTransitPolicies`, through `spdEntryPolicies` | `TestRFC4301OperatorEntryDecidesTransitTraffic` |
| Transit UDP from a protected host with source port 500 | → | the Child SA's OUT PROTECT policy, with the node-wide bypass removed (`e700fb7392`) and Ze's IKE sockets carrying their own bypass | `TestRFC4301TransitFromIKEPortIsNotBypassed`, `TestRFC4301IKESocketsCarryTheirOwnBypass` |
| A non-error ICMP message on a Child SA from outside its selectors | → | `peerSAOptions` and the state flag `xfrmStateFromParams` writes | `TestRFC4301NonErrorICMPOutsideSelectorsIsDiscarded` |

## Acceptance Criteria

AC numbers in this table belong to this architecture spec. In particular,
AC-4 covers forwarded ICMP and is separate from the closed product spec's
fragment-acceptance AC-4. The product's CLEAN review does not close this spec's
remaining requirements or its own final Review Gate.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | An operator writes an SPD entry whose action is discard | traffic this node sends or receives that matches the entry is dropped by the installed policy (`block` on XFRM, `IPSEC_API_SPD_ACTION_DISCARD` on VPP), and a disposition a backend cannot express is refused, never substituted (`xfrmPolicyAction`). Forwarded traffic is AC-16, a failed install is AC-14, and the default-configuration refusal is AC-19. The VPP half (operator SPD entries on the VPP backend) is not owed by this spec: it is owned by `plan/post-release/features/spec-ipsec-vpp-policy-interface.md`, whose AC-6 and AC-8 carry the VPP proof, after the release (Thomas's ruling of 2026-11-20, "plan/release/fixes/spec-ipsec-vpp-policy-interface.md will be done after the release", reversing decision 7's pull-forward). No operator can reach the VPP backend before `plan/post-release/features/spec-ipsec-dataplane-selector.md` lands, so this AC is judged on XFRM | Traffic the entry does not name keeps the disposition of the entry it does match, because a discard is one entry of an ordered database |
| AC-2 | A forwarded non-initial fragment arrives whose addresses and protocol match a port-scoped BYPASS or DISCARD entry, or a port-specific Child SA policy | Under the route owner decision 1 selects: with the kernel route, the fragment takes the disposition of the packet it belongs to, and a fragment that belongs to none is dropped (an orphan fragment expires in reassembly and is counted as `ReasmTimeout` in the kernel's SNMP counters); with the refusal route, the commit that would install the port-scoped entry on a forwarding node is refused, naming the entry and the any-port form that is accepted. Proof: `TestRFC4301ForwardedFragmentsRespectOperatorDisposition` and `TestRFC4301CleartextFragmentsRespectOperatorDisposition`, observed on Ze's kernel. Status 2026-11-20: met for an operator port-scoped BYPASS on Ze's kernel. With K3's capability bit 3 the entry is projected onto FWD (`1dc1323b86`) and the kernel reassembles before the check; the commit refusal of `a29420771d` applies only on a kernel without bit 3 (AC-22). Observed on Ze's kernel: IPv4 forwarded (`TestRFC4301ForwardedFragmentsRespectOperatorDisposition`, its new phase), IPv6 forwarded (`TestRFC4301ForwardedIPv6FragmentsFollowAPortScopedBypass`, `d173045116`), IPv4 local delivery (`TestRFC4301CleartextFragmentsRespectOperatorDisposition`, tagged in `adb172d967`), and forgery (`TestRFC4301ForgedFragmentsTakeTheReassembledDatagramsDisposition`, `8a991db40e`). Port-specific Child SA selectors are not covered by this AC's change | Fragments under an any-port entry or SA pass, because RFC 4301 Section 7.1 lets such an SA carry every fragment; fragments addressed to this node are reassembled before the check and take the whole packet's disposition |
| AC-3 | An administrator writes a discard entry for one ICMP type with `direction in` | an ICMP message of that type arriving in clear for this node is dropped, and one of another type is delivered (`TestKernelICMPSelectorDropsOnlyTheNamedMessages`, `test/ipsec/ipsec-icmp-type-selector.ci`). This covers traffic addressed to this node only. Transit ICMP is AC-16, and ICMP arriving on a Child SA is AC-18 | Every ICMP type the entry does not name is delivered, because the control is per type; type 0 with a code range holding 0 on an out or both entry stays refused (`validateICMPFragments`) |
| AC-4 | A protected transit ICMP error message arrives whose payload header, with source and destination addresses and ports reversed, does not match the traffic selectors of the SA it arrived on | the message is dropped when the check is configured, and forwarded when an SPD entry explicitly allows carriage of such traffic. Proof for the forwarded case: `TestRFC4301ForwardedICMPUsesPayloadPolicy`, `TestRFC4301ForwardedICMPErrorsMapUnderDiscard`; for local delivery: `TestICMPErrorPayloadHeaderCheckedOnLocalDelivery`. Status 2026-11-19 (evening): the forwarded half is green on amd64 (`516671704b`: OUT allows at `dataplane.PriorityICMPErrorReturn`, the nftables table `ze_ipsec_icmp_error_return`, doctor check `ipsec-icmp-error-return`). Still owed, by the owner decision of 2026-11-20 (the kernel route): the K1 extension so the forwarded path tries `xfrm_out_fwd_icmp` on a BLOCK (`-EPERM`); its K1 feature bit; the engine's requirement of that bit, as for bit 5; removal of the stock-kernel OUT allows and the nftables table `ze_ipsec_icmp_error_return`; and a namespace packet test in which an operator out DISCARD covers a forwarded mappable error and the error is mapped into the return SA, with its negative, a non-mappable error under the same discard dropped. Also owed: a forwarded parameter-problem, a namespace test of a local error under the mapping, and refusal A extended to the D-3 gates. Status 2026-11-20: DONE. The kernel route is `40cf3cfaec` (K1 bit 6; the stock-kernel route retired in `78d79ad79a`); the packet tests of the catch-all and of an operator out DISCARD, with the echo and the unmapped error staying dropped, passed in the guest (`3aef55597f`); `5666563229` extends refusal A to the OUT transit gate of an in bypass (`TestICMPOverlapBypassTransitGate`) and adds the forwarded parameter-problem (`TestRFC4301ForwardedICMPParameterProblemMapsUnderDiscard`) | An error whose embedded header matches the SA is delivered or forwarded, which is the Section 6.2 processing; a peer with `icmp-error-check false` skips the check by the operator's choice |
| AC-5 | An SPD entry names an ICMP type and a code or code range, ANY port, or one port | the selector is programmed exactly (a code range as mask-aligned blocks, `spdICMPMatches`), and an entry the backend cannot program exactly is refused rather than widened. A port range is refused today (`parseSPDPort`), which meets "never widened" and does not meet RFC 4301 Section 4.4.1.1: "Each of these selectors has a list of ranges of values." The port range stays a gap on `RFC4301-4.4.1.1-1`; since 2026-11-19 its home is AC-14 of `plan/post-release/features/spec-ipsec-opaque-selector-port-mask.md` | ANY port, one port, ANY type, one type with ANY code or a code range are all programmed; a port range, OPAQUE and an exact Mobility Header type are refused until their owning work lands |
| AC-6 | A peer asserts an identity that falls under a configured sub-tree, and another peer asserts one that does not | the first authenticates and the second is refused. The same holds for a DN sub-tree, a partially qualified RFC 822 address, and an address inside and outside a configured range. Units: the six `TestPad...` tests of `engine/rfc4301_pad_subtree_test.go`; interop `pad-subtree-identity` is owed | An exact identity still matches exactly, and ID_KEY_ID stays byte-exact (`TestPadKeyIDStaysExact`), because a sub-tree has no meaning for an opaque key id |
| AC-7 | An administrator gives two overlapping SPD entries distinct orders | the lower order decides the disposition, the order survives a restart, and an order of 0 is refused at commit. The order, and a peer's `policy-priority`, is a number from 1 to 2147483647, default 1000. No band is reserved for IKE: by owner order of 2026-11-19 (`50b89fb51b`) the band is gone, because Ze's IKE exchange is exempt on its own IKE sockets before any SPD entry (`TestRFC4301IKESocketsCarryTheirOwnBypass`). Tests: `TestSpdOperatorOrdersOverlappingPeers`, `TestSPDPolicyOrderOutsideTheRankRangeRefused`, `TestSPDRankBoundsMatchTheYANGRanges`. Equal orders are AC-15 | An entry with no order takes 1000 and a peer with no `policy-priority` takes 2000, so an unordered entry outranks the tunnels by default, as the YANG says |
| AC-8 | A peer is configured with `df-bit clear`, then `df-bit copy`; and separately with `df-bit set` | for clear and copy, the outer header of an emitted tunnel-mode packet carries the configured DF value, observed on the wire by the peer's counters, and a break of `xfrmDFFlag` turns the wire-counter check red. `df-bit set` is refused at commit with the fix named (owner decision 2026-11-12). Not met yet: both recorded reds stopped at the `show` readback, so no break has reached the wire check. Since `22a4740af7` the scenario judges the wire counters first (assertions 1 to 6) and passes; owed: the `DFClear` break with assertions 1 and 3 red, and the `DFCopy` break with 4 and 6 red. Status 2026-11-20: DONE. On the bit-7 docker-host image `c88a4387bff259f1` the `DFClear` break went red at assertions 1 and 3 and the outbound-only `DFCopy` break at 4 and 6; the six red walks of the checkpoint's Interop table each went red at their predicted assertions | `df-bit set` stays refused on Linux and VPP, because neither can write a set DF bit on an ESP tunnel header (`LL-XFRM-OUTER-DF-SET-DSCP`); the kernel feature is the post-release spec |
| AC-9 | A per-SA PMTU is learned and the aging period elapses | the PMTU returns to the first-hop data-link MTU and is relearned, and the period is configurable through the `net.ipv4.route.mtu_expires` and `net.ipv6.route.mtu_expires` sysctls. Proof: the PMTU integration units, IPv6 aging under QEMU | With `df-bit clear` on an IPv4 tunnel no PMTU is learned, because routers fragment the outer packet and return nothing (`xfrmNoPMTUDisc`); VPP learns none and uses the egress MTU |
| AC-10 | A packet enters a domain for which the inner DSCP value is not appropriate and a mapping is configured | the outer tunnel header carries the mapped value, observed on the wire: copy or 0 on Linux (`TestXFRMOuterHeaderCarriesTheOuterDSCP`), any value on VPP | A fixed value 1 to 63 on Linux is refused at commit and install with the fix named (owner decision 2026-11-12), so `RFC4301-5.1.2.1-1` stays partial |
| AC-11 | `rfc/short/rfc4301.md` is read at closure | its Purpose line describes the native IKEv2 engine rather than strongSwan and charon (corrected 2026-11-19); its Enrolment reason counts the rows the file holds (105 rows on 2026-11-19 after `e626cbdf17` added `RFC4301-5.2-11`: 96 MUST, 6 MUST NOT, 2 SHOULD, 1 MAY); its body describes the native engine (the strongSwan prose replaced in `e626cbdf17`); the five rows of "Summary rows that disagree with this spec" carry the annotation the evidence supports. The seven `{not-applicable}` annotations this row once named are gone from the file. Status 2026-11-20: the five rows are re-judged (`47f7ca7b55`, `88d7159e0e`): `RFC4301-4.4.1.1-3`, `RFC4301-7.3-4` and `RFC4301-7.3-5` carry `{partial}`; `RFC4301-4.4.1-4` (D-2 refuses equal orders on overlapping entries) and `RFC4301-7.4-2` (the forward-path proof on Ze's kernel) stay bare | A `{gap}` or `{partial}` whose obligation is still unmet stays, because disclosure is the point of the row |
| AC-12 | The generated RFC 4301 proof row, summary, records and native RFC check are read at closure | The derived Proof/Evidence/Unmet cells disclose every remaining gap or partial obligation and count only enforced records. Owner-chosen DF-set/fixed-DSCP refusals, OPAQUE feature debt and the absent exact-MH classifier retain their named homes and are never counted conformant. No stale Supported/no-gap criterion or retired supportClaimingScope edit overrides those decisions. This spec's current-capability defects and required proof still have to be completed; lifetime-volume and extraction/sign-off obligations retain their separate owners. | Every gap with a named home stays published as a gap; nothing is removed from the unmet list to reach a figure |
| AC-13 | A configured or peer-proposed selector names a port under a protocol with undefined ports, including 47 and 50 as well as 0 | Refuse or narrow the selector without emitting a specific port; RFC7296-3.13.1-1 and RFC7296-3.13.1-2 have discriminating tests and independent enforced verdicts. Owns the weak verdicts transferred under P-3 from spec-rfc-verdict-fix-ike-eap. Status 2026-11-20: the four records exist (`7234fdfecc`) and their freshness check is done | ANY port under such a protocol is accepted and emitted as 0 to 65535, as RFC 7296 Section 3.13.1 sets it; ANY Mobility Header type stays usable |
| AC-14 | An operator DISCARD or BYPASS entry fails to install, on a first apply or after an in-place edit (D-1) | the apply fails with an operator error naming the entry, the cause and the command that retries, and the commit is refused. For an edited entry the dataplane still holds the previous form, so matching traffic stays dropped throughout: the new form is installed before the old one is removed wherever their selectors differ, and replaced in place where they are equal. Tests: `TestRFC4301FailedOperatorEntryInstallRefusesTheApply` (engine, through `applyConfig`), `TestRFC4301DiscardHoldsAcrossAnEditedEntry` (netns packet test), `test/ipsec/ipsec-spd-discard-entry.ci`. At start-up, by owner decision 3 (a) of 2026-11-19, there is no commit to refuse: Ze installs every entry it can, logs each one that fails at Error with its name, cause and fix, and `ze doctor` reports it with the same fix; the IPsec block is never refused at start. Status 2026-11-19 (evening): both Go tests exist and `TestRFC4301DiscardHoldsAcrossAnEditedEntry` passes on amd64; the `.ci` is the draft `test/draft/ipsec/ipsec-spd-discard-entry.ci`, not yet run (it needs the guest), then stress and promotion. Status 2026-11-20: the draft passes in the guest (`qemu-c860ed8b`); its stress, then promotion, is blocked by another session's daemon start failure ("local command thanks declares daemon path show thanks list, which no YANG ze:command node declares") | A platform with no XFRM at all still runs the control plane (`isXFRMUnsupported`), because no dataplane exists there to hold any entry; a removal that finds nothing stays silent |
| AC-15 | Two operator entries, or an operator entry and a peer, overlap in selector and direction and carry the same order, including two entries left at the default (D-2) | the commit is refused, naming both entries, the shared order and the `set vpn ipsec policy <name> order <n>` command that separates them, which is what the YANG `name` leaf already promises. Entries that do not overlap keep equal orders. The dataplane receives the entries in order, never in map order. Tests: `TestSPDPolicyEqualOrderOnOverlapRefused` (engine, `engine/rfc4301_spd_order_total_test.go`, the recorded negative of `RFC4301-4.4.1-4`), `test/parse/ipsec-spd-order-total.ci`, `test/ipsec/ipsec-spd-order-readback.ci`. Status 2026-11-19 (evening): the Go test exists with its record; `test/draft/parse/ipsec-spd-order-total.ci` passes as a draft, stress and promotion owed; `test/draft/ipsec/ipsec-spd-order-readback.ci` not yet run (it needs the guest). Status 2026-11-20: `test/parse/ipsec-spd-order-total.ci` is promoted (`df31eb1c63`); the draft `ipsec-spd-order-readback.ci` passes in the guest; its stress, then promotion, is blocked by the same daemon start failure as AC-14 | Entries whose selectors or directions do not overlap may share an order, because no packet matches both and so no order between them is observable |
| AC-16 | A gateway operator writes a bypass or a discard entry for traffic forwarded between a protected host and the outside (D-3) | the entry decides that traffic in both forwarding directions, and `show vpn ipsec dataplane policy` lists its forward projection. On XFRM, for an entry with direction in or both, the in half's selectors are installed in FWD at the entry's order: these are the decisions. A BYPASS also gets gates, template-free allows at `dataplane.PriorityTransitGate`, just above the catch-all: its in selectors in OUT and its out selectors in FWD. The gates are not at the entry's order, because at the entry's order the OUT gate of an `in` any-to-any bypass would outrank every Child SA's outbound PROTECT policy. A non-initial fragment is handled as AC-2 says. Test: `TestRFC4301OperatorEntryDecidesTransitTraffic` (netns packet test with a protected host, a positive control that passes and the discarded flow that does not). Status 2026-11-19 (evening): landed in `072ef5bf1f`; the test passes at `0c283fffc6` and goes red with `spdTransitPolicies` emptied; the order checks now judge the forward entries (`TestSPDOrderJudgesTheForwardEntries`) and a peer's P and P+1 entries. Defect found 2026-11-20: a BYPASS's FWD gate matches a forwarded ICMP error that arrived on a Child SA, so the consistent error is rejected (`TestRFC4301ForwardedICMPUsesPayloadPolicy` red at if_id 0 on `qemu-c860ed8b`). Owed, by the owner decision of 2026-11-20: the K1 bit 7 hunk, `XFRM_POLICY_ICMP` on every transit gate, the engine's requirement of bit 7, the scope tests' complete receipt moved to 255 or as the bit list says, and the guest run. Status 2026-11-20: bit 7 landed in `4663a5e15e`, with `XFRM_POLICY_ICMP` on the FWD gates, and `42edf271d9` puts it on the FWD decision of an in or both bypass too (owner rulings A and B); `TestRFC4301ForwardedICMPPastBypassDecisionUsesPayloadPolicy` passes on `docker-53c57c37` and goes red with the decision's flag removed. Open decision 2 was decided "refusal until done": `a29420771d` refuses a port-scoped BYPASS at commit on a kernel without K3's bit 3 (AC-22) | Forwarded traffic no operator entry names keeps the Child SA policy or the final disposition. A port-scoped BYPASS is not projected only on a kernel without K3's capability bit 3 (RFC 4301 Section 7.4, `LL-XFRM-FRAGMENT-SELECTOR`, owner decision 1, the kernel route); there the commit refuses it (AC-22), and one whose local prefix is one address this host holds is accepted and not projected, so forwarded traffic on its ports fails closed under `unmatched discard`. With bit 3 it is projected (`1dc1323b86`); a port-scoped DISCARD is projected either way. An `in` ICMP entry whose selector holds type 0 code 0 is not projected, because every forwarded fragment reads as that type and code. VPP needs no forward entry, because its inbound SPD applies before routing |
| AC-17 | A protected host behind Ze sends UDP with source port 500 or 4500 to a destination a tunnel's selectors cover, under `unmatched bypass` and under `unmatched discard` (D-4) | Route, by owner decision of 2026-11-19: Ze's IKE bypass is per-socket XFRM policies on Ze's own IKE sockets, as strongSwan does, and the node-wide any-address UDP port 500 and 4500 bypass (`ikeBypassPolicies`, `installIKEBypass`) is removed. The packet leaves inside the tunnel under both, never in clear, while Ze's own IKE exchange with the same peer still completes a rekey. The positive control, ordinary transit from the same host, is carried by the tunnel under both values. Test: `TestRFC4301TransitFromIKEPortIsNotBypassed` (netns packet test; the peer's inbound SA counter and a cleartext capture on the outside link are the two observations). MET 2026-11-19: `e700fb7392` removes the node-wide bypass and deletes `engine/bypass.go`; red observed at `69fc643e50` (transit UDP from source ports 500 and 4500 left in clear under `unmatched bypass`, amd64 guest, K1 kernel `docker-1562c4ae`), green at `e700fb7392`; `ce5c276457` fixes the test's 9-octet IKE reply and the forwarding defect the positive control exposed under `unmatched discard`. Passing: `TestRFC4301TransitFromIKEPortIsNotBypassed` under both values, `TestRFC4301IKESocketsCarryTheirOwnBypass`, 40 MOBIKE tests. The R-8 interop set ran 2026-11-19, result recorded in the commit that follows | Ze's own IKE datagrams on UDP 500 and 4500 stay exempt, in and out, because RFC 4301 Section 5.2 gives IKE traffic an explicit BYPASS entry |
| AC-18 | A non-error ICMP message arrives on a Child SA from outside its selectors, with the bytes after its ICMP header forged to match the SA, on a peer with the default `icmp-error-check` (D-5) | the message is dropped and the SA's selector mismatch is counted, while an ICMP error whose embedded header matches is still delivered (the Section 6.2 processing). Test: `TestRFC4301NonErrorICMPOutsideSelectorsIsDiscarded` (netns packet test, with the consistent error as its positive control). Status 2026-11-19 (evening): passes on Ze's 7.3 kernel with the K1 series, amd64 guest | An ICMP error whose embedded header, reversed, matches the SA is delivered, because that is the Section 6.2 processing; non-error ICMP inside the selectors is delivered as any packet is |
| AC-19 | An operator commits a discard entry with the default direction and protocol beside a policy-based peer with the default `icmp-error-check` (D-6) | the commit is accepted and the entry drops the traffic it names, ICMP included. Route, by owner decision of 2026-11-19: the kernel honours the discard. K1 is extended so the reverse ICMP-error check never overrides an operator DISCARD entry, and the commit refusal `refuseICMPCheckDiscard` goes (decision 4 is superseded). No refusal or recipe tells the operator to turn the Section 6.2 check off to get a working discard. Tests: `test/parse/ipsec-spd-discard-beside-default-peer.ci`, and the packet half in `TestRFC4301NonErrorICMPOutsideSelectorsIsDiscarded`. Status 2026-11-19 (evening): `refuseICMPCheckDiscard` is gone; the packet half passes on amd64; `test/draft/parse/ipsec-spd-discard-beside-default-peer.ci` passes as a draft, stress and promotion owed | Refusal A stays: a bypass that would carry an ICMP error in clear beside a peer whose SAs map it (owner ruling 2026-11-12, "refuse at commit") |
| AC-20 | Ze's kernel is built with the patch series and a multi-pair Child SA is installed (phase 2b) | the receipt reports the ABI number and the feature bits the series provides; the engine sets the SA's complete directional selector set as its state scope; a decrypted packet outside every pair of its own SA is dropped although a sibling SA's policy would admit it; `show vpn ipsec dataplane sa` reads the scope back. On a kernel without the receipt the install is refused with the kernel build command named. Tests: `TestRFC4301KernelAdvertisesExactStateScope`, `TestRFC4301ChildStateScopeReadback`, `TestRFC4301MultipairSAsRejectSiblingScope`, `TestRFC4301ParallelMultipairSAsKeepPermittedDelivery`, in the QEMU guest. Owed only under owner decision 1 (a) or (c). Status 2026-11-20: the packet tests are observed on Ze's kernel `docker-53c57c37` | A single-pair SA keeps its legacy state selector, byte-identical to its one scope record; a packet inside any pair of its own SA is delivered (`TestRFC4301ParallelMultipairSAsKeepPermittedDelivery`) |
| AC-21 | A Child SA is installed on Linux 6.10 or later, and `show vpn ipsec dataplane sa` reads it back (owner decision 2 (a), 2026-11-19) | CLOSED 2026-11-19 by the owner; evidence in the AC-21 evidence paragraph of the current checkpoint. the inbound state reads back `in` and the outbound one `out`, each showing only its own options (the ICMP error check on `in`; DF and outer DSCP on `out`); the outbound state carries no anti-replay window and the inbound one keeps the negotiated 64 (RFC 4301 Section 4.4.2.1: the window is "used to determine whether an inbound AH or ESP packet is a replay"); an `SAParams` with `Dir` out and a nonzero window, or with `Dir` fwd, is refused before the kernel as a Ze defect; the OSPFv3 per-neighbor outbound state drops its window too. Tests: `TestXFRMStateCarriesTheSADirection`, `TestXFRMStateReplayWindowOnlyOnTheReceivingSA`, `TestXFRMStateRefusesADirectionNoStateCarries` (dataplane, linux), `TestXFRMReadbackReportsTheDirectionZeInstalled` (kernel readback, QEMU guest), `TestChildSAReplayWindowDefault` (engine), `TestIPsecSAReplayWindowOnlyOnTheReceivingSA` (ospf), `TestVPPSAFlagsAntiReplayPerDirection` (ze_vpp) | A zero-`Dir` state, the RFC 4552 OSPFv3 multicast SA serving both directions, is installed without a direction and keeps its window; a kernel older than 6.10 ignores the attribute and reads back `unknown`; the VPP outbound SA carries no window and so no `USE_ANTI_REPLAY`, and still refuses a cycled counter, because VPP v26.06 `esp_seq_advance` does so on every outbound SA (RFC 4303 Section 3.3.3) |
| AC-22 | An operator commits a `vpn ipsec policy` BYPASS with a local or remote port on an XFRM dataplane whose kernel does not advertise K3's capability bit 3 (owner decision 2, "refusal until done", 2026-11-20) | the commit is refused with the entry named and the fix, unless the entry's local prefix is one address this host holds (traffic to and from Ze is reassembled before the IN check and classified before fragmentation); a host address list that cannot be read refuses rather than guesses. At start, by decision 3 (a), the entry is installed and logged at Error, and doctor check `ipsec-spd-forward-bypass` reports it with the fix. With bit 3 the refusal does not apply and the entry is projected onto FWD (AC-2, AC-16). Landed in `a29420771d`. Tests: `TestRFC4301PortScopedForwardBypassRefusedAtCommit`, `TestPortScopedForwardBypassRefusalCoversBothPortsAndUnreadableAddresses`, `TestPortScopedForwardBypassAtStartIsLoggedAndReportedByDoctor`, which its commit records passing on darwin; these unit tests are the proof of the refusal path. The old draft `ipsec-spd-forward-bypass-refused.ci` was deleted by owner decision (C) of 2026-11-20. Its replacement, the promoted `test/ipsec/ipsec-spd-forward-bypass-projected.ci`, proves the bit-3 side on Ze's kernel. It asserts that the bypass commits and that the kernel holds its FWD decision and gate with port 22; it passed 40 of 40 under load in the guest, and goes red at LIVE-STAGES line 4 with `spdForwardGathers` forced false. The bit-3 positive is `TestPortScopedBypassForwardPathFollowsFragmentGathering` (`1dc1323b86`). Status 2026-11-20: DONE | No dataplane, VPP (its inbound SPD runs before routing), a DISCARD with a port and an any-port BYPASS are not refused, because none of them leaves a forwarded fragment judged on ports the check could not read |

Rows of Work Not Done that had no criterion: the forwarded `RFC4301-6.2-3` and
`RFC4301-6.2-4` cases are AC-4 (its forwarded half, proven by
`TestRFC4301ForwardedICMPUsesPayloadPolicy` and
`TestRFC4301ForwardedICMPErrorsMapUnderDiscard`); the forward projection for
`RFC4301-6-1` is AC-16; the shared reqid is AC-20; the SA direction is AC-21,
under owner decision 2 (a), decided 2026-11-19.

AC-8 note (2026-11-10): `df-bit set` is refused at commit by owner decision,
because neither dataplane can write a set DF bit on an ESP tunnel header
(`LL-XFRM-OUTER-DF-SET-DSCP`). The `tunnel-df-treatment` scenario therefore
covers `clear` and `copy`, and `RFC4301-8.1-2` keeps its `{gap}` for `set`.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An operator writes an SPD entry that discards traffic to a subnet | config to ipsec types to `SPParams` with `SPActionDiscard` to the backend to the kernel SPD | `TestDiscardPolicyReachesTheBackend`, `TestRFC4301FailedOperatorEntryInstallRefusesTheApply`, `TestRFC4301OperatorEntryDecidesTransitTraffic`. The three broken links of the 2026-11-14 read (D-6, D-1, D-3) are fixed. Owed: run and promote the draft `test/draft/ipsec/ipsec-spd-discard-entry.ci`; the `spd-discard-entry` interop scenario, written in `22a4740af7`, ran 2026-11-19, result recorded in the commit that follows |
| 2 | An attacker forges a non-initial fragment aimed at a bypassed port range | interface to the kernel classifier to a drop, under the route of owner decision 1 | `TestRFC4301ForwardedFragmentsRespectOperatorDisposition` and `TestRFC4301CleartextFragmentsRespectOperatorDisposition`, written and not yet observed. Owed: the `bypass-fragment-forgery` interop scenario, which does not exist |
| 3 | An operator rejects unauthenticated ICMP Destination Unreachable addressed to this node | config to an operator discard entry with `icmp-type 3 direction in` to the kernel SPD to a drop | `TestSPDICMPTypeControlRejectsOrAcceptsOneType`, `TestKernelICMPSelectorDropsOnlyTheNamedMessages`, `test/ipsec/ipsec-icmp-type-selector.ci` |
| 4 | An operator authorizes every peer under one DNS sub-tree with one PAD entry | config to `remoteIDMatches` to the authentication verdict | `TestPadSubtreeAdmitsAPeerBeneathIt`, `TestPadSubtreeRefusesAPeerOutsideIt`. The `pad-subtree-identity` interop scenario is written in `22a4740af7`; it ran 2026-11-19, result recorded in the commit that follows |
| 5 | An operator orders two overlapping SPD entries and reads them back | config to `SPParams.Priority` to the kernel, and back through the inspection path | `TestSpdOperatorOrdersOverlappingPeers`, `TestSPDPolicyEqualOrderOnOverlapRefused` (D-2 landed). Owed: run and promote the draft `test/draft/ipsec/ipsec-spd-order-readback.ci` |
| 7 | A protected host behind the gateway sends UDP from port 500 | forward lookup, out lookup, the Child SA's OUT PROTECT policy, the tunnel | `TestRFC4301TransitFromIKEPortIsNotBypassed` (AC-17), passing since `e700fb7392` |
| 6 | An operator pins the DF bit of a tunnel to clear so a legacy path stops dropping large packets | config to the installed SA to the emitted outer header | `TestDFBitTreatmentReachesTheBackend` plus the `tunnel-df-treatment` interop scenario |

## 🧪 TDD Test Plan

The status columns preserve the earlier planning inventory. Where they call a
product fragment or scope test unobserved, the final accepted receipts in the
current checkpoint supersede that label. The remaining architecture proof and
mapped-return work stay owed; this prose update adds no runtime test debt.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDiscardPolicyReachesTheBackend` | `engine/rfc4301_spd_discard_test.go` | the discard action survives the config to `SPParams` path | exists |
| `TestXfrmPolicyActionRefusesAnUnknownDisposition` | `dataplane/rfc4301_discard_linux_test.go` | a disposition the backend cannot express is refused, never substituted | exists |
| `TestRFC4301ForwardedFragmentsRespectOperatorDisposition`, `TestRFC4301CleartextFragmentsRespectOperatorDisposition` | `engine/fragment_forward_integration_linux_test.go`, `engine/fragment_boundary_integration_linux_test.go` | a fragment takes the disposition of the operator entry its packet matches | written, untagged, not observed on Ze's kernel |
| `TestSPDICMPTypeControlRejectsOrAcceptsOneType`, `TestSPDICMPErrorControlFromTheProtectedSide` | `engine/rfc4301_icmp_selector_test.go` | the per-type control decides an ICMP message in clear | exists |
| `TestICMPErrorPayloadHeaderCheckedOnLocalDelivery`, `TestRFC4301ForwardedICMPUsesPayloadPolicy` | `dataplane/rfc4301_icmp_error_integration_linux_test.go`, `engine/rfc4301_forward_icmp_integration_linux_test.go` | the embedded header, reversed, is matched against the SA selectors, for local delivery and for a forwarded error | exists |
| `TestICMPSelectorReachesTheBackend` | `dataplane/rfc4301_icmp_selector_linux_test.go` | an ICMP type and code selector is programmed exactly | exists |
| `TestSPDPolicyPortNeedsAProtocolThatCarriesPorts`, `TestRFC4301SPDInterfaceRefusesWhatItCannotProgram` | `ipsec/rfc4301_spd_discard_test.go`, `ipsec/rfc4301_spd_management_test.go` | a selector the backend cannot program is refused, never widened | exists |
| `TestPadSubtreeAdmitsAPeerBeneathIt`, `TestPadSubtreeRefusesAPeerOutsideIt`, `TestPadSubtreeStillBindsTheCertificate`, `TestPadKeyIDStaysExact` | `engine/rfc4301_pad_subtree_test.go` | a sub-tree matches under and only under itself | exists |
| `TestPadAddressRangeAdmitsAnAddressInsideIt`, `TestPadAddressRangeRefusesWhatIsOutsideIt` | `engine/rfc4301_pad_subtree_test.go` | an address identity inside a configured range matches and one outside it does not | exists |
| `TestSpdOperatorOrdersOverlappingPeers`, `TestSpdUnstatedOrderTakesTheDefaultRank` | `engine/rfc4301_spd_order_test.go` | an operator order becomes the policy priority, and an unstated order takes the default rank | exist; `TestSpdOperatorOrdersOverlappingPeers` is the recorded positive of `RFC4301-4.4.1-4`. `TestSpdOrderCannotCaptureTheIKEControlPlane` was removed with the band in `50b89fb51b` |
| `TestSPDPolicyOrderOutsideTheRankRangeRefused`, `TestSPDRankBoundsMatchTheYANGRanges` | `ipsec/rfc4301_spd_discard_test.go`, `ipsec/policy_priority_default_test.go` | an order or `policy-priority` of 0 is refused, and the Go rank bounds equal the YANG ranges 1 to 2147483647 | untagged; replace `TestSPDPolicyOrderInsideBypassBandRefused` since `50b89fb51b` (owner order) |
| `TestDFBitTreatmentReachesTheBackend`, `TestXFRMStateRefusesDFItCannotExpress` | `engine/rfc4301_df_bit_test.go`, `dataplane/rfc4301_df_linux_test.go` | clear and copy reach the installed SA, and set is refused | exists |
| the PMTU integration units (A-4) | `dataplane/rfc4301_pmtu_integration_linux_test.go`, `dataplane/rfc4301_pmtu_ipv6_integration_linux_test.go` | the kernel ages the path MTU back to the link MTU after `mtu_expires` | exists |
| `TestOuterHeaderDSCPReachesTheBackend` | `engine/rfc4301_dscp_test.go` | the mapped DSCP value reaches the installed SA | exists |
| `TestRFC4301FailedOperatorEntryInstallRefusesTheApply` | `engine/rfc4301_spd_install_failure_test.go` | AC-14: a refused install fails `applyConfig` and names the entry | exists |
| `TestRFC4301DiscardHoldsAcrossAnEditedEntry` | `engine/rfc4301_spd_edit_integration_linux_test.go` | AC-14: matching traffic stays dropped while an entry's order is edited | netns packet test; renamed `rfc4301_*` in `da9e704088`; passes on amd64, 2026-11-19 |
| `TestSPDPolicyEqualOrderOnOverlapRefused` | `engine/rfc4301_spd_order_total_test.go` | AC-15: two overlapping entries of equal order are refused, and two that do not overlap are accepted | exists; the recorded negative of `RFC4301-4.4.1-4` |
| `TestSPDOrderJudgesTheForwardEntries`, `TestAnyPortBypassReachesTheForwardPath` | `engine/rfc4301_spd_transit_test.go` | AC-15 and AC-16: the order check judges the forward entries; an any-port bypass reaches the forward path, which is the recorded negative carrier of `RFC4301-7.4-2` | exist since `072ef5bf1f` |
| `TestRFC4301OperatorEntryDecidesTransitTraffic` | `engine/spd_forward_integration_linux_test.go` | AC-16: an operator bypass and discard decide forwarded traffic in both directions | netns packet test; passes at `0c283fffc6`, red with `spdTransitPolicies` emptied |
| `TestRFC4301TransitFromIKEPortIsNotBypassed`, `TestRFC4301IKESocketsCarryTheirOwnBypass` | `engine/rfc4301_ike_bypass_forward_integration_linux_test.go` | AC-17: transit UDP from port 500 or 4500 leaves in the tunnel, Ze's own rekey still completes, and Ze's IKE sockets carry their own bypass | netns packet tests; red at `69fc643e50`, green at `e700fb7392` under both `unmatched` values; `RFC4301-5.2-4` records on both |
| `TestChildOutboundForwardPolicyAdmitsProtectedHostTransit`, `TestChildOutboundForwardPolicyBuiltOnlyForPolicyBasedTunnels`, `TestRFC4301DecryptedTransitReachesProtectedHostUnderDiscard` | `engine/child_forward_policy_test.go`, `engine/rfc4301_transit_delivery_integration_linux_test.go` | the defect of `ce5c276457`: a policy-based tunnel forwards both ways under `unmatched discard`, and only a policy-based tunnel at if_id 0 gets the two allows | pass on amd64, 2026-11-19 |
| `TestRFC4301ForwardedICMPErrorsMapUnderDiscard` | `engine/rfc4301_icmp_return_integration_linux_test.go` | AC-4: a forwarded ICMP error under `unmatched discard` is mapped into the return SA | passes on amd64 since `516671704b` |
| `TestRFC4301NonErrorICMPOutsideSelectorsIsDiscarded` | `dataplane/rfc4301_icmp_nonerror_integration_linux_test.go` | AC-18 and AC-19: a non-error ICMP message outside the selectors is dropped on the SA, a consistent error is delivered, and a protocol 0 discard drops ICMP | netns packet test; passes on amd64, 2026-11-19 |
| `TestRFC4301KernelAdvertisesExactStateScope`, `TestRFC4301ChildStateScopeReadback`, `TestRFC4301MultipairSAsRejectSiblingScope`, `TestRFC4301ParallelMultipairSAsKeepPermittedDelivery` | `engine/rfc4301_state_scope_readback_integration_linux_test.go`, `engine/rfc4301_state_scope_integration_linux_test.go` | AC-20: the receipt, the scope readback and sibling rejection | written; not observed on a kernel built with the series |
| `TestXFRMStateCarriesTheSADirection`, `TestXFRMStateReplayWindowOnlyOnTheReceivingSA`, `TestXFRMStateRefusesADirectionNoStateCarries` | `dataplane/xfrm_sa_dir_linux_test.go` | AC-21: `SAParams.Dir` maps to `XFRMA_SA_DIR`, zero stays unset, an outbound window and a forward-direction SA are refused as a Ze defect | committed in `3977cef525`; pass by name on the claude server (Linux x86_64, kernel 7.2.9-ze), ike/dataplane in full at `c068931843`, 2026-11-19 |
| `TestXFRMReadbackReportsTheDirectionZeInstalled` | `dataplane/xfrm_readback_integration_linux_test.go` | AC-21: the kernel reads back `in`, `out` and no direction, the outbound window is 0, and each side holds only its own options | committed in `3977cef525`; passes by name on the claude server, kernel 7.2.9-ze, ike/dataplane in full at `c068931843`, 2026-11-19 |
| `TestChildSAReplayWindowDefault` (RFC-tagged, changed) | `engine/child_test.go` | AC-21: the inbound Child SA carries 64 and the outbound one 0 | changed in `3977cef525`, approved by `ze-test-judge`; discrimination record in `20b70674db`/`4f2e4d3da0`; ike/engine passes in full on the claude server, kernel 7.3.0-ze, 2026-11-19 |
| `TestRFC4301SADItemAntiReplayWindowIsInstalled`, `TestRFC4301BoundarySADEntryCarriesWhatTheKernelLooksUp` (RFC-tagged, changed) | `dataplane/rfc4301_sad_items_linux_test.go`, `dataplane/rfc4301_boundary_linux_test.go` | AC-21: the window is asserted on an inbound SA (`boundaryInboundSA`), since an outbound one is now refused | changed in `3977cef525`, approved by `ze-test-judge`; discrimination records in `20b70674db`/`4f2e4d3da0`; ike/dataplane passes in full on kernel 7.2.9-ze at `c068931843`, 2026-11-19 |
| `TestIPsecSAReplayWindowOnlyOnTheReceivingSA` | `internal/plugins/ospf/ipsec_install_test.go` | AC-21: the OSPFv3 per-neighbor outbound state carries no window, the shared and inbound ones the configured one | committed in `3977cef525`; internal/plugins/ospf passes on the Mac (darwin arm64) and on the claude server (kernel 7.2.9-ze), 2026-11-19 |
| `TestVPPSAFlagsAntiReplayPerDirection` | `dataplane/vpp_sa_flags_test.go` | AC-21: `USE_ANTI_REPLAY` follows the window, so the VPP outbound SA carries none | committed in `3977cef525`; passes on the Mac (darwin arm64, `ze_vpp`), 69 of 69 ike/dataplane VPP tests, 2026-11-19 |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| SPD entry order (YANG `order`, and a peer's `policy-priority`); no band reserved since `50b89fb51b` (owner order) | 1 to 2147483647 | 2147483647 | 0 | 2147483648 |
| ICMP type selector | 0 to 255 | 255 | N/A | 256 |
| ICMP code selector | 0 to 255 | 255 | N/A | 256 |
| Port range endpoints | 0 to 65535 | 65535 | N/A | 65536 |
| DSCP value | 0 to 63 | 63 | N/A | 64 |
| PMTU aging period, seconds | 60 to 86400 | 86400 | 59 | 86401 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-spd-discard-entry` | `test/ipsec/ipsec-spd-discard-entry.ci` | an operator commits a default discard entry beside a default peer, reads it back from the kernel SPD as `block`, and a commit whose install the kernel refuses fails (AC-14, AC-19) | draft at `test/draft/ipsec/ipsec-spd-discard-entry.ci`; not yet run (needs the guest) |
| `ipsec-spd-discard-beside-default-peer` | `test/parse/ipsec-spd-discard-beside-default-peer.ci` | the default discard entry beside a default policy-based peer is accepted (AC-19) | draft at `test/draft/parse/`; passes as a draft; stress and promotion owed |
| `ipsec-spd-order-total` | `test/parse/ipsec-spd-order-total.ci` | two overlapping entries of equal order are refused with both names and the fix; two that do not overlap are accepted (AC-15) | draft at `test/draft/parse/`; passes as a draft; stress and promotion owed |
| `ipsec-spd-order-readback` | `test/ipsec/ipsec-spd-order-readback.ci` | an operator orders two overlapping entries and reads the order back through `show vpn ipsec dataplane policy` | draft at `test/draft/ipsec/ipsec-spd-order-readback.ci`; not yet run (needs the guest) |
| `ipsec-icmp-type-selector` | `test/ipsec/ipsec-icmp-type-selector.ci` | an operator rejects one ICMP type and permits another, read back from the real kernel SPD. It does the job the planned `ipsec-icmp-unauthenticated-control` named | exists |
| `ipsec-sa-options-kernel`, `ipsec-df-bit-treatment` | `test/ipsec/ipsec-sa-options-kernel.ci`, `test/parse/ipsec-df-bit-treatment.ci` | `df-bit clear`, `dscp-map 0` and `icmp-error-check false` reach the kernel's Child SA states; every DF treatment no SA can apply is refused | exists |

The planned `ipsec-bypass-fragment-check.ci` is not owed as a `.ci`: a forged
fragment needs a raw sender and a capture, which the netns packet tests carry.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `spd-discard-entry` | `test/interop-ipsec/scenarios/` | strongSwan | a discard entry drops traffic strongSwan would otherwise carry, and the tunnel beside it stays up | written in `22a4740af7`; PASS on 2026-11-20 on the bit-7 docker-host image `c88a4387bff259f1`; red proofs observed: `break-spd-out` at assertion 4, `break-spd-in` at assertion 5 |
| `bypass-fragment-forgery` | none: a netns packet test in Ze's QEMU guest, `TestRFC4301ForgedFragmentsTakeTheReassembledDatagramsDisposition` (`8a991db40e`) | none: the forgery is cleartext Ze forwards, so no peer daemon takes part | forged non-initial fragments aimed at an operator port-scoped BYPASS on Ze's kernel take the disposition of the reassembled datagram on the forward path | retargeted by Thomas's decision 4 of 2026-11-20 from the node-wide IKE bypass (gone since `e700fb7392`); not a strongSwan scenario. Written in `8a991db40e` |
| `icmp-error-payload-check` | `test/interop-ipsec/scenarios/` | strongSwan | an ICMP error addressed to Ze (local delivery) arriving on the SA: one whose payload header is consistent with the SA selectors is delivered, an inconsistent one is dropped. A transit (forwarded) error is out of scope here and stays `LL-XFRM-ICMP-TRANSIT` | current IPv4 single-pair clean PASS and proofs (c)/(d) observed in all seven native runs on 2026-11-16, independently CLEAR; general scope and other phase-3 obligations remain open |
| `pad-subtree-identity` | `test/interop-ipsec/scenarios/` | strongSwan | a strongSwan peer whose ID_FQDN falls under a configured sub-tree authenticates, and one outside it is refused | written in `22a4740af7`; PASS on 2026-11-20 on image `c88a4387bff259f1`; red proofs observed: `break-pad-true` at assertion 7, `break-pad-false` at assertion 1 |
| `tunnel-df-treatment` | `test/interop-ipsec/scenarios/` | strongSwan | the outer header DF bit carries each configured treatment, read on the wire | since `22a4740af7` the wire counters are judged before the readback, numbered assertions 1 to 6; PASS on 2026-11-19 (Mac, arm64, K1 kernel `qemu-1562c4ae`). PASS again on 2026-11-20 on image `c88a4387bff259f1`. Red proofs observed (AC-8): `DFClear` break red at assertions 1 and 3; the outbound-only `DFCopy` break red at 4 and 6 |
| `child-rekey`, `responder-ike-rekey`, `natt-tunnel-inner-checksum`, `real-nat-transport-rekey-after-ike-rekey`, `mobike-initiator` | `test/interop-ipsec/scenarios/` | strongSwan | the R-8 regression set: rekey, NAT and MOBIKE still work with no node-wide IKE bypass and with the new forwarding allows | run 2026-11-19, result recorded in the commit that follows |

## Files to Modify
- `internal/component/ike/dataplane/dataplane.go` - `SPAction` gains DISCARD; `SPParams` gains ICMP type and code selectors, a port range form, a DF treatment and a DSCP mapping; the priority constants carry an operator order. Since `50b89fb51b` no band is reserved (`PriorityIKEBypass` deleted), and `PriorityTransitGate` (`072ef5bf1f`) and `PriorityICMPErrorReturn` (`516671704b`) sit just above the catch-all
- `internal/component/ike/dataplane/xfrm_linux.go` - the Linux projection of every new field, and the refusal for a field it cannot express
- `internal/component/ike/dataplane/vpp_policy.go` - the VPP projection, and the refusal for a field it cannot express
- AC-21: `internal/component/ike/dataplane/xfrm_linux.go` (`xfrmSADirFromParams`, the outbound-window refusal), `dataplane.go` (the `Dir`, `ReplayWin` and `SAInfo.Dir` comments), `vpp.go` (`vppSAFlags` keeps anti-replay on the outbound SA), `engine/child.go` (the window on the inbound SA only), `internal/plugins/ospf/ipsec_install.go` (`ipsecReplayWindowFor`), and the tests that built an outbound SA with a window: `dataplane/rfc4301_boundary_linux_test.go`, `rfc4301_sad_items_linux_test.go`, `rfc4301_outer_header_integration_linux_test.go`, `rfc4301_pmtu_integration_linux_test.go`, `engine/rfc4301_transport_output_fragment_integration_linux_test.go`; pages `ipsec-8-ikev2-child-xfrm.md`, `ipsec-dataplane-inspection.md`, `docs/guide/ipsec.md`, `ospf/ospf-ext-16-ipsec-auth.md`
- `internal/component/ike/engine/bypass.go` - deleted in `e700fb7392` with the node-wide IKE bypass (D-4); Ze's IKE sockets carry their own bypass. The fragment check on a BYPASS path now concerns an operator port-scoped BYPASS (AC-2)
- `internal/component/ike/engine/child.go` - since `ce5c276457`, the outbound forward entries and delivery entries of a policy-based Child SA (`appendChildOutboundForwardPolicies`, `appendChildDeliveryPolicies`)
- `internal/component/ike/engine/icmp_error_return.go` - since `516671704b`, the return allows and the nftables drop table `ze_ipsec_icmp_error_return` of the forwarded ICMP error mapping (AC-4), with the doctor check `ipsec-icmp-error-return`
- `internal/component/ike/engine/child.go` - `childPolicyParams` carries the new selector and priority fields
- `internal/component/ike/engine/remote_id.go` - `remoteIDMatches` gains sub-tree and range matching
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - the SPD entry list, the ICMP controls, the DF treatment, the DSCP mapping and the PMTU aging period
- `internal/component/ike/ipsec/validate.go` - the commit-time validators for the new leaves
- `internal/core/diagnostic/codes.go` - a code per new runtime dependency a doctor check reads
- `internal/component/ike/engine/doctor.go` - the doctor checks for the new dependencies
- `rfc/short/rfc4301.md` - the prose repair, the fresh answers on the seven `{not-applicable}` annotations and the `{gap}`, the corrected `{single-polarity}` reason on `RFC4301-4.1-3`, and one new requirement id per obligation this spec implements
- `docs/features/rfc-status.md` - generated read surface only: inspect derived proof and unmet ids, never hand-edit or commit it
- `internal/le/rfc/check_status.go`, `internal/le/rfc/profilecoverage_test.go` - existing publication/disclosure contracts to read, not a retired supportClaimingScope list to modify
- `docs/architecture/core-design.md` - declared by the `// Design:` header of `internal/le/rfc/check_status.go`. Named as unaffected: this spec reads the RFC tooling and changes none of it
- `internal/component/ike/engine/spd_policy.go`, `internal/component/ike/engine/apply.go` - `installSPDPolicies`, now `reconcileSPDPolicies`, returns its install failure and `applyConfig` refuses the apply (AC-14); the forward projection of an operator entry, `spdTransitPolicies` (AC-16)
- `internal/component/ike/engine/spd_order.go`, `internal/component/ike/ipsec/spd_policy.go` - the equal-order refusal `spdOrderRefusals`, run at commit as `verifySPDOrder` (AC-15), which also judges the forward projection; and the rank range 1 to 2147483647 (`spdRankValid`, AC-7)
- `internal/component/ike/engine/icmp_overlap.go`, `internal/component/ike/engine/unmatched.go` - the D-5 and D-6 repair and the recipe text (AC-18, AC-19)
- `docs/architecture/linux-limitations.md` - `LL-XFRM-FRAGMENT-SELECTOR` gains what the AC-17 packet test observes for a forwarded packet and the out lookup
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - declared by the `// Design:` header of `dataplane.go`, `xfrm_linux.go`, `vpp_policy.go`, `child.go` and, until `e700fb7392`, `bypass.go`. The SPD action set, the selector set and the policy ordering it describes all change
- `docs/architecture/ike/ipsec-dataplane-inspection.md` - the policy fields the inspection path reads back
- `docs/architecture/ike/ipsec-7-ikev2-engine.md` - declared by the `// Design:` header of `remote_id.go` and `doctor.go`. The PAD matching rules it describes change in phase 5, and it gains the new doctor checks in phase 3
- `docs/architecture/ike/ipsec-3-data-model.md` - declared by the `// Design:` header of `internal/component/ike/ipsec/validate.go`. The cross-reference validation it describes gains the SPD order, ICMP type and code, port range, DSCP and PMTU period rules
- `docs/features/ai-first.md` - declared by the `// Design:` header of `internal/core/diagnostic/codes.go`. Named here as affected only by the new diagnostic codes: the page describes how a code reaches an agent, and adding codes changes the list rather than the mechanism
- `docs/guide/ipsec.md` - the new operator config
- `docs/features.md` - the IPsec feature rows
- `docs/functional-tests.md` - the five new `.ci` names
- `docs/architecture/testing/interop.md` - the five new scenario names

## Files to Create
- Not created, by design (2026-11-14): `engine/icmp.go` (the peer leaf `icmp-error-check`, the operator ICMP selector and `engine/icmp_overlap.go` took its place), `engine/pmtu.go` (the kernel ages the PMTU, A-4) and `engine/fragment.go` (fragment state belongs to the kernel's reassembly queues, phase 2b)
- The unit and netns packet test files the TDD table marked "not written": all exist on 2026-11-19 (evening)
- `test/ipsec/ipsec-spd-discard-entry.ci`, `test/ipsec/ipsec-spd-order-readback.ci`, `test/parse/ipsec-spd-discard-beside-default-peer.ci`, `test/parse/ipsec-spd-order-total.ci`: each is a draft under `test/draft/` on 2026-11-19, promoted to its `test/` path after its run and stress
- `test/interop-ipsec/scenarios/spd-discard-entry/` and `test/interop-ipsec/scenarios/pad-subtree-identity/`: created in `22a4740af7`. `test/interop-ipsec/scenarios/bypass-fragment-forgery/`: not created, by Thomas's decision 4 of 2026-11-20. The forgery is cleartext Ze forwards and needs no peer, so it is the netns packet test `TestRFC4301ForgedFragmentsTakeTheReassembledDatagramsDisposition` in Ze's guest (`8a991db40e`). `tunnel-df-treatment` and `icmp-error-payload-check` exist
- The engine writer of `SAParams.Scope` (AC-20), in the file that builds the Child SA's parameters, under owner decision 1 (a) or (c)
- the retired deferral shard "rfc4301-architecture-gaps" - the shard named in the metadata table

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` for the SPD entry list, the ICMP controls, the DF treatment, the DSCP mapping and the PMTU aging period |
| YANG validation constraints | Yes | every new leaf takes `range`, `enumeration` or `pattern`: the order, the ICMP type and code, the port range endpoints, the DSCP value and the aging period all have the bounds in the Boundary Tests table |
| YANG custom validators | Yes | the equal-order refusal on overlapping entries (`verifySPDOrder`), which no native constraint can express. The reserved-band validator is gone since `50b89fb51b`: the YANG range 1 to 2147483647 is the whole bound |
| CLI commands/flags | Yes | the config editor reaches the new leaves; `show vpn ipsec` gains the SPD order and action in its output |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md`: every new path is keyword before value, `policy <name> action discard` rather than `<name> discard` |
| Editor autocomplete | Yes | automatic for the enumerations; the policy name leaf needs a `CompleteFn` over the configured entries |
| Functional test for new RPC/API | Yes | the five `.ci` tests named above |
| Pipe completeness | Yes | the `show vpn ipsec` additions route through the existing pipe path |
| Env var registration | N-A | no new environment leaf |
| Doctor check for runtime dependencies | Yes | the fragment path and the ICMP path each read a kernel facility. Each gets an owning-package check plus a code in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | Yes | a counter per dropped forged fragment, per rejected unauthenticated ICMP error, and per transit ICMP error refused by the payload check |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` and `docs/guide/ipsec.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` for the `show vpn ipsec` additions |
| 4 | API/RPC added/changed? | No | no new RPC handler beyond the config surface |
| 5 | Plugin added/changed? | No | the IKE component is not a plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` |
| 7 | Wire format changed? | No | no new IKEv2 payload or ESP framing. The selector fields travel in existing traffic selector payloads |
| 8 | Plugin SDK/protocol changed? | No | no SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4301.md` and the `docs/features/rfc-status.md` row |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` and `docs/architecture/testing/interop.md` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` compares IPsec features against strongSwan |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` and `docs/architecture/ike/ipsec-dataplane-inspection.md` |
| 13 | Route metadata keys added/changed? | No | no route metadata |
| 14 | Prometheus counters added/changed? | Yes | the subsystem telemetry doc for the three new counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no registry entry changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/release/fixes/spec-rfc4301-architecture-gaps.md` at the start of each phase and name every doc it lists. `docs/features/rfc-status.md` already carries a `<!-- source: internal/component/ike/dataplane/dataplane.go -->` anchor over a file every phase edits |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ipsec.md` shows the `vpn ipsec` grammar; verify every example against the YANG after each phase |

Historical ordering note, corrected 2026-11-14: at that checkpoint the
remaining-work authority was the phase-status table in the correction section.
The current continuation checkpoint supersedes its completed product work.
The steps below are the original
plan and are kept as history: several name a test, a file or a scenario that
was never written under that name, and "Names in this spec that the tree does
not hold" gives what exists. The work the 2026-11-14 correction adds runs in
this order, each step test first:

1. D-1 and D-2 (AC-14, AC-15): the install failure and the total order. Both
   are in the operator SPD list and need no kernel work.
2. D-4 (AC-17): the packet test first, because it also settles assumption A-8
   and with it how far D-3 reaches.
3. D-3 (AC-16): the forward projection, with AC-2's fragment disposition taken
   from owner decision 1.
4. D-5 and D-6 (AC-18, AC-19), on the kernel route of the 2026-11-19 owner
   decision: K1 is extended so the reverse ICMP-error check never overrides an
   operator DISCARD entry, and `refuseICMPCheckDiscard` is removed. Decision 4
   is superseded.
5. Phase 2b (AC-2, AC-20), under owner decision 1.
6. The AC-8 wire break, the three absent interop scenarios and the two absent
   `.ci` files.

The earlier text read: the phase table at the top is the remaining-work authority. The original
implementation steps below retain their obligations and evidence requirements;
phase 2a and phases 5 and 6 are delivered history and are not repeated.


## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register every new entry point and prove each wiring test red
   - Tests: the ten rows of the Wiring Test table
   - Files: `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`, `internal/component/ike/dataplane/dataplane.go`, the skeleton handlers
   - Verify: every config path commits, every new field reaches a stub that returns "not implemented", and every wiring test fails on the stub rather than on a missing path
2. **Phase: BYPASS and DISCARD fragments (Sections 7.4 and D.4)** -- the live attack, first
   - RFC: Section 7.4, "All implementations MUST support DISCARDing of fragments using the normal SPD packet classification mechanisms." and "All implementations MUST support stateful fragment checking to accommodate BYPASS traffic for which a non-trivial port range is specified." Appendix D.4, "implementations MUST support fragment reassembly for BYPASS/DISCARD traffic when port fields are specified" and "An implementation also MUST permit a user or administrator to accept such traffic or reject such traffic using the SPD conventions described in Section 4.4.1."
   - Tests: `TestDiscardPolicyReachesTheBackend`, `TestBackendRefusesUnexpressibleAction`, `TestNonInitialFragmentIsNotBypassed`, `ipsec-spd-discard-entry`, `ipsec-bypass-fragment-check`, the `spd-discard-entry` and `bypass-fragment-forgery` scenarios
   - Files: `dataplane.go`, `xfrm_linux.go`, `vpp_policy.go`, `bypass.go`, `fragment.go`
   - Verify: validate A-3 with the QEMU test BEFORE designing the fix. Run the whole `test/interop-ipsec` suite at the end (A-7)
3. **Phase: ICMP processing (Section 6)** -- the whole section, unimplemented and unextracted
   - RFC: Section 6, "Disposition of non-error, ICMP messages (that are not addressed to the IPsec implementation itself) MUST be explicitly accounted for using SPD entries." Section 6.1.1, "a compliant IPsec implementation MUST permit a local administrator to configure an IPsec implementation to accept or reject unauthenticated ICMP traffic. This control MUST be at the granularity of ICMP type and MAY be at the granularity of ICMP type and code." Section 6.1.2, "implementers MUST provide controls to allow local administrators to constrain the processing of ICMP error messages received on the protected side of the boundary, and directed to the IPsec implementation." Section 6.2, "an IPsec implementation MUST be configurable to check that this payload header information is consistent with the SA via which it arrives" and "IPsec senders and receivers MUST support the following processing for ICMP error messages that are sent and received via SAs."
   - Tests: `TestUnauthenticatedICMPTypeRejected`, `TestTransitICMPPayloadHeaderChecked`, `ipsec-icmp-unauthenticated-control`, the `icmp-error-payload-check` scenario
   - Files: `icmp.go`, `ze-ipsec-conf.yang`, `validate.go`, the doctor check and its diagnostic code
   - Verify: the ICMP type granularity is a MUST and the type-and-code granularity is a MAY, so the MAY goes to the owner as a question (`ai/rules/rfc-compliance.md`)
4. **Phase: SPD selectors (Section 4.4.1.1)** -- clears the one gated requirement
   - RFC: Section 4.4.1.1, "The following selector parameters MUST be supported by all IPsec implementations to facilitate control of SA granularity."
   - Tests: `TestICMPSelectorReachesTheBackend`, `TestPortRangeSelectorRefusedWhenNotExpressible`
   - Files: `dataplane.go`, `xfrm_linux.go`, `vpp_policy.go`, `child.go`, `ze-ipsec-conf.yang`
   - Verify: this phase clears `RFC4301-4.4.1.1-1`. Its `{gap}` annotation is removed only when the selectors are programmed AND a tagged test proves it, and the ledger's gap disclosure is corrected in the same commit
5. **Phase: PAD matching (Section 4.4.3.1)** -- the authorization surface
   - RFC: Section 4.4.3.1, "But, at a minimum, sub-tree matching of the sort described above MUST be supported." and "For IPv4 and IPv6 addresses, the same address range syntax used for SPD entries MUST be supported."
   - Tests: `TestRemoteIDSubTreeMatch`, `TestRemoteIDAddressRangeMatch`, the `pad-subtree-identity` scenario
   - Files: `remote_id.go`, `ze-ipsec-conf.yang`, `validate.go`
   - Verify: the class gate in `remoteIDMatches` stays. A sub-tree pattern that would also match an exact identity of another class is refused at commit, not resolved at runtime
6. **Phase: SPD entry ordering (Section 4.4.1)** -- depends on the SPD entry list phase 2 introduces
   - RFC: Section 4.4.1, "The management interface for the SPD MUST allow creation of entries consistent with the selectors defined in Section 4.4.1.1, and MUST support (total) ordering of these entries, as seen via this interface." and "the system administrator MUST be able to specify whether or not a user or application can override (default) system policies."
   - Tests: `TestPolicyOrderReachesTheBackend`, `TestPolicyOrderInsideBypassBandRefused`, `ipsec-spd-order-readback`
   - Files: `dataplane.go`, `child.go`, `ze-ipsec-conf.yang`, `validate.go`
   - Verify: R-4. The IKE bypass keeps a reserved band, and an interop scenario that rekeys proves the bypass still outranks every Child SA policy
7. **Phase: DF bit and PMTU (Section 8)**
   - RFC: Section 8.1, "All IPsec implementations MUST support the option of copying the DF bit from an outbound packet to the tunnel mode header that it emits, when traffic is carried via a tunnel mode SA. This means that it MUST be possible to configure the implementation's treatment of the DF bit (set, clear, copy from inner header) for each SA." Section 8.2.2, "In all IPsec implementations, the PMTU associated with an SA MUST be 'aged' and some mechanism is required to update the PMTU in a timely manner".
   - Tests: `TestDFBitTreatmentReachesTheBackend`, `TestPMTUAgingPeriodConfigurable`, `ipsec-df-bit-treatment`, the `tunnel-df-treatment` scenario
   - Files: `dataplane.go`, `xfrm_linux.go`, `pmtu.go`, `ze-ipsec-conf.yang`
   - Verify: validate A-4 by reading the producing kernel path and observing a PMTU return to the first-hop MTU in QEMU. Kernel behavior is evidence to put to the owner, never a classification this phase makes (R-1)
8. **Phase: Outer-header DSCP (Section 5.1.2.1)**
   - RFC: Section 5.1.2.1, note 5, "If the packet will immediately enter a domain for which the DSCP value in the outer header is not appropriate, that value MUST be mapped to an appropriate value for the domain".
   - Tests: `TestOuterHeaderDSCPReachesTheBackend`
   - Files: `dataplane.go`, `xfrm_linux.go`, `ze-ipsec-conf.yang`
   - Verify: the mapping is configurable per SA, and the emitted outer header carries the mapped value on the wire rather than only in the installed state
9. **Phase: summary, annotations and the public row** -- the closing phase
   - Tests: `TestStemFiguresListUnmetIDs`, `TestOneSummaryRendersExactlyOneStatusRow`, `./le rfc check`
   - Files: `rfc/short/rfc4301.md`, owned discrimination/audit records, and the existing extraction artifact under its separate owner's rules; read the generated public ledger without authoring it
   - Verify: every changed annotation has source evidence, every new tagged claim has an observed red and an independent verdict, and the derived public row honestly discloses every unmet obligation. Do not remove gap/partial annotations to satisfy the superseded Supported criterion. Current-capability repairs and their proof remain mandatory; the separately owned extraction/sign-off scope is not decided here.

**Phase-9 scope reconciliation, 2026-11-12.** The original AC-12/no-gap
deliverable predates the owner's explicit first-release choice to refuse DF
set and fixed DSCP values (decision table above, quoted 2026-11-12 instruction),
and the standing release-timing rule in `ai/META-DECISIONS.md`. OPAQUE remains
owned by `plan/post-release/features/spec-ipsec-opaque-selector-port-mask.md`;
the wanted exact-MH kernel classifier is now owned by
`plan/post-release/kernel/spec-xfrm-ipv6-mh-selector.md`. Their absent features
remain RFC gaps. DF/fixed-DSCP retain
`plan/post-release/kernel/spec-xfrm-outer-df-set-dscp.md` and their partial rows.
This does not reduce the current defect scope: accepted-but-ineffective
selectors must be refused, ICMP/fragment/SA-binding defects and required
interoperability proofs stay owed, and no missing field becomes conformant
through disclosure. The byte-lifetime release fix and the extraction/sign-off
spec remain named dependencies with their original ownership. The old
`supportClaimingScope`, `checkSupportedSignoff` and
`TestSupportedRowsHaveDerivableScope` names no longer exist in the current RFC
tooling; `checkStatusAgreement` and derived Proof/Unmet are the present
publication contract. No tooling behavior is changed by this reconciliation.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every block in the Task has a phase, and every phase has a landed test naming its RFC section |
| Feature completeness | Every new config leaf is reachable from the editor, appears in `show vpn ipsec`, and is proven by a `.ci`, not only by a unit test |
| Correctness | `SPActionProtect` is still the zero value of `SPAction`, so a caller that forgets the field still gets a protecting policy |
| Correctness | Every backend either programs a new selector field exactly or refuses the install. No field is widened, rounded or silently dropped |
| Correctness | The fragment check covers DISCARD entries as well as BYPASS entries. Section 7.4 names both |
| Naming | Every new field says what the value is rather than its Go type, and every new YANG leaf matches the operator vocabulary already in `docs/guide/ipsec.md` |
| Data flow | An SPD entry is declared once. The peer `traffic-selector` list and the new SPD entry list do not both declare the same selector (R-3) |
| Rule: `ai/rules/rfc-compliance.md` | No `{gap}`, `{not-applicable}` or `partial` is written or kept for an unmet obligation without a recorded answer from the owner. Every MUST implemented here carries a comment above the enforcing code quoting its RFC section |
| Rule: `ai/rules/interop-and-goal-validation.md` | Each new scenario was proven RED by reverting the change and rebuilding the artifact, and the RED result is recorded |
| Rule: `ai/rules/evidence.md` | Every claim about what the kernel does names the producing path and is proven by a QEMU test, never inferred from the caller |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| DISCARD is expressible | `grep -n SPActionDiscard internal/component/ike/dataplane/dataplane.go` returns the constant and every backend returns a call site |
| Fragments are checked | the netns packet tests on Ze's kernel pass: `TestRFC4301ForwardedFragmentsRespectOperatorDisposition`, `TestRFC4301ForwardedIPv6FragmentsFollowAPortScopedBypass`, `TestRFC4301CleartextFragmentsRespectOperatorDisposition` and `TestRFC4301ForgedFragmentsTakeTheReassembledDatagramsDisposition` (which replaces the `bypass-fragment-forgery` scenario, decision 4). `ipsec-bypass-fragment-check.ci` is not owed: a forged fragment needs a raw sender and a capture |
| ICMP controls exist | `./le config schema` shows the ICMP leaves and `ipsec-icmp-unauthenticated-control.ci` passes |
| Selector behavior is exact or refused, with absent features disclosed | Supported selectors have enforced boundary evidence; unsupported constrained selectors refuse before state changes; `RFC4301-4.4.1.1-1` keeps each remaining feature gap and its owner |
| PAD sub-tree matching exists | the `pad-subtree-identity` scenario passes against strongSwan |
| SPD ordering exists | `ipsec-spd-order-readback.ci` passes |
| DF and PMTU are configurable | the `tunnel-df-treatment` scenario passes and `TestPMTUAgingPeriodConfigurable` passes |
| DSCP is mapped | `TestOuterHeaderDSCPReachesTheBackend` passes |
| The summary is repaired | `grep -in strongswan rfc/short/rfc4301.md` returns nothing that describes Ze's own negotiation |
| The public row is truthful | Derived Proof/Evidence/Unmet agree with the source-backed summary and enforced records; no wanted feature gap is hidden or counted conformant |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Guard fails open | A fragment the check cannot classify must be dropped, never passed. A selector a backend cannot program must refuse the install, never widen it. A PAD sub-tree that fails to parse must refuse the peer, never match everything |
| Input validation | Every field this spec reads off the wire or out of a fragment header is attacker-controlled. The fragment path parses a forged header by definition, so it takes the boundary tests and a fuzz target |
| Resource exhaustion | Fragment state is attacker-driven. It carries a bounded entry count and a bounded lifetime, and the bound is stated in the code (`docs/contributing/ze-go-style.md`, "A limit on everything") |
| Downgrade | A peer must not be able to negotiate a selector wider than the configured SPD entry. Narrowing goes inward only |
| Authorization | The PAD sub-tree match widens who can authenticate. A pattern that matches more than the operator intended is the failure mode, so every sub-tree test carries a negative case |
| Error leakage | A refusal message names the offending value and the configured expectation. It does not print a key, a certificate private field or an operator credential |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A backend cannot express a field | The install is refused and the refusal is tested. Widening the selector is banned |
| The kernel appears to discharge an obligation | Evidence to put to the owner with the producing path and a QEMU observation. Never a `{not-applicable}` this spec writes (R-1) |
| A phase is too large for one agent | R-6: report the size to the main thread and let it re-cut by section. Do not trim the block |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The walk that found these blocks read the RFC's own text rather than the summary. The summary declared 18 obligations against a source carrying 98 uppercase MUST-level keyword occurrences, and every block above sat outside the 18. An unbounded checklist is green for what nobody wrote down.
- `rfc/short/rfc4301.md` contradicts itself today. Its prose says Ze delegates IKEv2 negotiation to strongSwan and charon, and its own annotations cite `internal/component/ike/engine`, the native engine. A page that disagrees with the code is a defect the work that meets it repairs (`ai/rules/documentation.md`).
- Two annotations on that page are false at the producer as of 2026-08-30, and both were read rather than inferred. The `{gap}` on `RFC4301-4.4.1.1-1` says `SPParams` has no port or protocol field; `SrcPort`, `DstPort` and `UpperProto` all exist, and only the ICMP type and code are absent. The `{single-polarity}` reason on `RFC4301-4.1-3` says the child-SA path hardcodes tunnel mode so a peer SA can never be transport; `createFirstChildSA` sets transport mode on `sa.UseTransportMode`.
- Seven annotations are void under the owner directive of 2026-07-27 and must be re-answered rather than cited: the `{gap}` on `RFC4301-4.4.1.1-1` and the `{not-applicable}` on `RFC4301-4.1-5`, `RFC4301-4.1-6`, `RFC4301-4.4.1-2`, `RFC4301-5.2-1`, `RFC4301-5.2-2` and `RFC4301-7-1`. Six of the seven rest on "the kernel does the per-packet half", which is an observation about where the work happens and not an answer about whether Ze meets the obligation.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `test-asserts-superseded-behaviour.md` | 2026-11-13 | `TestRFC4301OrderedInboundSearchAdmitsOnTheFirstRankedEntry` wants XfrmInPolBlock for an out-of-selector packet |
| `guard-added-to-one-half-of-a-pair.md` | 2026-11-10 | RFC4301-6-1: operator SPD entries install out and in, never fwd |
| `silent-fall-through.md` | 2026-11-10 | forwarded fragments (port 0) miss a port-specific Child SA policy and meet the catch-all, in clear under `unmatched bypass` |
| `field-carries-two-meanings.md` | 2026-11-10 | protocol 135: the IKE MH-type value is installed as a port, Linux compares the bare type |
| `two-owners-share-one-name.md` | 2026-11-10 | reqid 1 and SPI-less templates on every Child SA of a peer; a sibling's any-port fwd policy accepts a fragment of an exact-port SA |
| `guard-enumerates-instead-of-subtracting.md` | 2026-11-10 | RFC7296-3.13.1-1/2 weak: a port under a portless protocol other than 0 is committed and emitted |
| `address-family-inferred-from-address-bytes.md` | 2026-11-13 | netlink `selFromPolicy` loses mapped IPv6 prefix family and transport scope byte identity |

## Key Design Decisions

These decisions preserve the original rationale. Their product-fragment
priority describes the work before its closure; it does not reopen that work
or change the remaining architecture requirements.

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Correct the public row first and implement second | Implement inside `plan/release/validation/spec-rfcgate-6-supported-extraction-signoff.md`; leave the row at `Supported` | Owner decision, 2026-08-30. The row is a public promise that is false today, and correcting it costs one line. Leaving it was not on the table, and folding eight blocks of protocol work into a bookkeeping spec would have made that spec unclosable |
| One phase per block | One phase for the whole RFC; one phase per requirement id | A block shares one subject matter and one surface, which is the unit an agent can hold. A requirement id is too small to justify a phase report and the whole RFC is too large for one context (`ai/rules/planning.md`) |
| Fragments before ICMP | Follow the RFC's own section order | The fragment gap is reachable on a running daemon by a forged packet, and the ICMP gap is a missing control. Blast radius orders the phases, not the document |
| The byte-count SAD lifetime stays in its own spec | Implement Section 4.4.2.1 here | `plan/release/fixes/spec-ipsec-lifetime-volume.md` already designs it. A second design of one fact is a future disagreement with nothing to arbitrate it (`ai/rules/principles.md`) |
| A new operator-authored SPD entry list | Extend the peer `traffic-selector` list with an action and an order | A DISCARD entry and a BYPASS entry are not attached to a peer, and Section 4.4.1 asks for a management interface over the SPD itself. Overloading the peer selector list would put two meanings on one leaf |
| No SPD rank is reserved for IKE (owner order, 2026-11-19, `50b89fb51b`), replacing the earlier reserved band | Keep a reserved band below 101 | Ze's IKE exchange is exempt on Ze's own IKE sockets, which the kernel consults before any SPD entry, so no order can capture it, and the band only cost the operator ranks |

## Known Limitations

- **It does not implement the Section 4.4.2.1 byte-count SAD lifetime.** `plan/release/fixes/spec-ipsec-lifetime-volume.md` designs it and remains a release-fix dependency. The phase-9 disclosure correction does not complete or demote that work, and the corresponding requirement stays unmet until its owner lands the implementation and proof.
- **It does not independently approve `rfc/extraction/rfc4301.json` or its sign-off population.** That work belongs to `plan/release/validation/spec-rfcgate-6-supported-extraction-signoff.md`; this spec preserves its scope question and cannot settle it by regenerating a ledger.
- **It does not answer the Section 6.1.1 MAY.** ICMP type-and-code granularity is a MAY, and `ai/rules/rfc-compliance.md` reserves a MAY to the owner. Phase 3 puts it to him with the cost of each option.
- **It does not widen the class gate in `remoteIDMatches`.** Sub-tree matching is added inside each existing class. An asserted ID_FQDN still cannot satisfy an address-valued `remote-id`.
- **On a kernel without K3's capability bit 3, a port-scoped operator BYPASS does not decide forwarded traffic.** There `spdEntryHasTransit` gives it no forward projection, because the forward check classifies a non-initial fragment with port 0 before any reassembly (RFC 4301 Section 7.4, `LL-XFRM-FRAGMENT-SELECTOR`). By owner decision 2 ("refusal until done", 2026-11-20) the commit refuses such an entry (`a29420771d`, AC-22), except one whose local prefix is one address this host holds; that one is accepted, and forwarded traffic on its ports takes the next entry that matches it, which under `unmatched discard` fails closed. With bit 3 the entry is projected (`1dc1323b86`). A port-scoped DISCARD is projected either way.
- **An `in` ICMP entry whose selector holds type 0 code 0 is not projected onto the forward path**, because every forwarded fragment reads as that type and code.
- **A packet Ze itself originates with a source inside a policy-based tunnel's remote selector and a destination inside its local selector leaves in clear towards the protected side** rather than meeting the catch-all. This is the tradeoff of the delivery entry `ce5c276457` installs so a policy-based gateway forwards decrypted traffic under `unmatched discard` (`appendChildDeliveryPolicies`).
- **A forwarded ICMP error that an operator out or both DISCARD covers is dropped, not mapped into the return SA**, because the DISCARD outranks the return allow at `dataplane.PriorityICMPErrorReturn`. Thomas chose the repair on 2026-11-20: the K1 extension of the forwarded path, which is owed work (AC-4).

## Work Not Done

Historical inventory from 2026-11-10, with the dated corrections in its rows.
The current checkpoint supersedes completed product-fix status in this table.
Each remaining architecture obligation keeps its named owner. This status-only
update does not independently close an old defect or promote an RFC row.

| Item | Why not done | Home |
|------|--------------|------|
| `RFC4301-6-1`: project operator SPD entries onto fwd, so a gateway can bypass or discard transit ICMP | DONE 2026-11-19 in `072ef5bf1f` (AC-16), except a port-scoped BYPASS on a kernel without K3's bit 3 (refused at commit since `a29420771d`, AC-22) and an `in` ICMP entry holding type 0 code 0, which are not projected (Known Limitations). Still owed: refusal A extended to the D-3 gates | this spec, `plan/release/fixes/spec-rfc4301-architecture-gaps.md`; journal row `guard-added-to-one-half-of-a-pair.md` 2026-11-10, owned above |
| Stale names in the post-release port-mask spec | kept as they are by owner decision | `plan/post-release/features/spec-ipsec-opaque-selector-port-mask.md` |
| `RFC4301-8.1-2` DF set and `RFC4301-5.1.2.1-1` a fixed DSCP 1 to 63 on Linux | owner decision 2026-11-12: refused for the first release, a Ze kernel feature after it | `plan/post-release/kernel/spec-xfrm-outer-df-set-dscp.md` |
| `RFC4301-6.2-3` and the inbound discard override: the two commit refusals | corrected 2026-11-14: the code is in the tree (`refuseICMPErrorBypass`, `refuseICMPCheckDiscard`, `verifyICMPOverlap`, with the `TestICMPOverlap...` units of `engine/rfc4301_icmp_overlap_test.go`). Refusal B is what makes a default discard uncommittable (D-6, AC-19). By owner decision of 2026-11-19, refusal B goes and K1 is extended so the kernel honours the discard; refusal A stays | this spec |
| `RFC4301-6.2-4` forwarded case: move the node-wide fwd catch-all to the netns XFRM default policy and drop the fwd interface catch-all, so `xfrm_in_fwd_icmp` runs | corrected 2026-11-14: the code is in the tree (`xfrmBackend.ApplyForwardDefault`, `installUnmatched`, `TestRFC4301ForwardDefaultFollowsAndReleasesUnmatched`, `TestRFC4301ForwardedICMPUsesPayloadPolicy`). Still owed: the observed green on Ze's kernel and the tagged record; the handover of 2026-11-13 records the forwarded return under `unmatched discard` red (AC-4). Observed green 2026-11-19: `TestRFC4301ForwardedICMPUsesPayloadPolicy` and its XFRM interface variant pass on amd64 | this spec |
| `RFC4301-6.2-3` forwarded case under `unmatched discard`: ICMP-error allow entries above the out catch-all plus an nftables drop of untransformed errors no bypass covers; and a namespace test observing the local mapping | landed 2026-11-19 in `516671704b`; `TestRFC4301ForwardedICMPErrorsMapUnderDiscard` passes. Still owed, by the owner decision of 2026-11-20: extend K1 so the forwarded path, `__xfrm_route_forward` included, tries `xfrm_out_fwd_icmp` on a BLOCK (`-EPERM`); advertise it by a K1 feature bit; make the engine require that bit as it requires bit 5; remove the stock-kernel OUT allows at `dataplane.PriorityICMPErrorReturn` and the nftables table `ze_ipsec_icmp_error_return`; and add a namespace packet test where an operator out DISCARD covers a forwarded mappable error and the error is mapped into the return SA, with its negative (a non-mappable error under the same discard is dropped). Also owed: a forwarded parameter-problem, a namespace test of a local error under the mapping, refusal A extended to the D-3 gates | this spec |
| Direction of Ze's own SAs in `show` (no `XFRMA_SA_DIR` set) | DONE 2026-11-19 (AC-21 closed by the owner) | this spec, `plan/release/fixes/spec-rfc4301-architecture-gaps.md` (AC-21) |
| Phase 2b, Section 7.4 stateful fragment checking | unchanged, under investigation (A-3) | this spec, `plan/release/fixes/spec-rfc4301-architecture-gaps.md` |
| Forwarded fragments of a port-specific Child SA meet the catch-all (port 0), crossing in clear under `unmatched bypass` | found 2026-11-10 reading LL-XFRM-FRAGMENT-SELECTOR; the fix belongs with phase 2b | this spec; journal row `silent-fall-through.md` 2026-11-10, owned above |
| Protocol 135 (MH): refuse exact Type while preserving ANY and RFC-valid encoding | 2026-11-12 runtime evidence: exact Type5 dropped matching IN/FWD packets; the repair's named race/verbose units and guest ANY4+typed exact-refusal controls now pass. The 13 mutation walks, four RFC port-field records and affected audit reseals remain owed. On 2026-11-19 the list of the 13 walks was found lost; 15 untagged candidate tests exist (open decision 3) | release repair here, journal row `field-carries-two-meanings.md` 2026-11-10; wanted classifier feature in `plan/post-release/kernel/spec-xfrm-ipv6-mh-selector.md` |
| Shared reqid 1 with SPI-less templates across a peer's Child SAs | found 2026-11-10; strains Section 7.3 for fragments of an exact-port SA | this spec; journal row `two-owners-share-one-name.md` 2026-11-10, owned above |
| RFC 7296 Section 3.13.1: a port under a portless protocol other than 0 | found by the 2026-11-10 audit re-judge (RFC7296-3.13.1-1 and -2 now `weak`) | this spec; journal row `guard-enumerates-instead-of-subtracting.md` 2026-11-10, owned above |
| Phase 9, summary, annotations and the public row | unchanged, blocked on both Depends specs | this spec, `plan/release/fixes/spec-rfc4301-architecture-gaps.md` |
| D-1 to D-6, found 2026-11-14: operator install failure, total ordering, forward projection, IKE bypass scope, non-error ICMP on an SA, the default discard refusal | logic defects of shipped behavior; AC-14 to AC-19. Code and Go tests of all six landed by 2026-11-19 (current checkpoint). Still owed: the `.ci` drafts run, stressed and promoted | this spec, `plan/release/fixes/spec-rfc4301-architecture-gaps.md` |
| The five summary rows published as proven against this spec's evidence (`RFC4301-7.4-2`, `RFC4301-7.3-4`, `RFC4301-7.3-5`, `RFC4301-4.4.1.1-3`, `RFC4301-4.4.1-4`), the Purpose line and the Enrolment reason count | the Purpose line and the Enrolment reason count were corrected on 2026-11-19, and `e626cbdf17` replaced the summary body's strongSwan prose with the native engine description. The D-1 to D-6 fixes have landed; the five rows still carry no annotation on 2026-11-19 (evening) and owe their re-judge | this spec, phase 9 (AC-11) |
| A port range in an operator entry and in a traffic selector (RFC 4301 Section 4.4.1.1: "Each of these selectors has a list of ranges of values.") | MOVED 2026-11-19 to the post-release port-mask spec, as its "Amended scope, 2026-11-19: port ranges" with its own AC-14. Every range is refused (`parseSPDPort`, `parsePortSelector`), and a missing feature may wait until after the release | `plan/post-release/features/spec-ipsec-opaque-selector-port-mask.md` (AC-14) |
| Interop scenarios `spd-discard-entry`, `pad-subtree-identity`, `bypass-fragment-forgery`; `.ci` files `ipsec-spd-discard-entry`, `ipsec-spd-order-readback` | named by phases 2a, 5, 2b and 6. On 2026-11-19: `spd-discard-entry` and `pad-subtree-identity` written in `22a4740af7`, run 2026-11-19 with the result recorded in the commit that follows, red proofs owed; `bypass-fragment-forgery` not written, its premise is open decision 4; both `.ci` files are drafts under `test/draft/ipsec/`, not yet run (they need the guest) | this spec |
| The parse drafts `ipsec-spd-order-total` and `ipsec-spd-discard-beside-default-peer` | pass as drafts on 2026-11-19; stress and promotion to `test/parse/` owed | this spec |
| AC-8 wire proof: a break of the DF treatment that the `show` readback does not catch | both recorded reds stopped at the readback. Since `22a4740af7` the scenario judges the wire counters first and passes; owed: `xfrmDFFlag` broken for `DFClear` (assertions 1 and 3 red expected) and for `DFCopy` (4 and 6 red expected), image rebuilt for each | this spec |
| A commit-time notice for a port-scoped operator BYPASS until K3's capability bit 3 exists | open decision 2 | this spec |
| D-3 defect found 2026-11-20: a BYPASS's FWD transit gate hides `xfrm_in_fwd_icmp`'s payload policy from a forwarded ICMP error that arrived on a Child SA (`XfrmInTmplMismatch`, RFC 4301 Section 5.2 step 4) | owner decision of 2026-11-20, the kernel route. Owed: the K1 bit 7 hunk ("forwarded ICMP error past a gate"), `XFRM_POLICY_ICMP` set on every transit gate, the engine's requirement of bit 7 as for bits 5 and 6, the scope tests' complete receipt moved to 255 or as the bit list says, and the guest run of `TestRFC4301ForwardedICMPUsesPayloadPolicy` at if_id 0. Also open: why this spec records that test passing at `516671704b`, which already held D-3 | this spec (AC-16) |
| Carried from the handover: the interface binding of VPP SPD entries; the RFC 4301 sign-off scope; whether decision 3 covers a catch-all install failure at boot | decisions 5 and 7, decided 2026-11-20. The VPP interface binding and the installation of operator entries on VPP moved to the VPP policy-interface spec (decision 7, "Pull the post-release spec forward"); the same day Thomas reversed its release placement ("plan/release/fixes/spec-ipsec-vpp-policy-interface.md will be done after the release"), so that work is owed after the release and this spec does not wait on it; the sign-off stays with rfcgate-6 (5b); 5c landed in `51799ad534` | the VPP binding, after the release: `plan/post-release/features/spec-ipsec-vpp-policy-interface.md` (AC-1, AC-6); the sign-off scope: `plan/release/validation/spec-rfcgate-6-supported-extraction-signoff.md` |

## Goal Validation

One row per goal of the Task section. Evidence read on 2026-11-20; the guest
kernel is Ze's 7.3 kernel with the K1 to K3 series, `docker-53c57c37` unless a
row names another, and the interop image is the bit-7 docker-host image
`c88a4387bff259f1`.

| Goal (Task section) | Evidence |
|---|---|
| The public RFC 4301 row stays `Partial`, with every unmet obligation disclosed and no figure counted from an unproven row | `47f7ca7b55` and `88d7159e0e` re-judge the five rows that disagreed: `RFC4301-4.4.1.1-3`, `RFC4301-7.3-4`, `RFC4301-7.3-5` carry `{partial}`, and `RFC4301-4.4.1-4` and `RFC4301-7.4-2` stay bare on their recorded proofs; `f804b97b80` re-judges 19 staled verdicts; `./le rfc check` reported no rfc4301 finding after `721d67d3c4` (AC-11, AC-12) |
| DISCARD entries drop what they name, on the host and in transit, and a failed install never leaves the traffic undropped | `TestRFC4301FailedOperatorEntryInstallRefusesTheApply`, `TestRFC4301DiscardHoldsAcrossAnEditedEntry`, `TestRFC4301OperatorEntryDecidesTransitTraffic` (guest); `test/draft/ipsec/ipsec-spd-discard-entry.ci` PASS in the guest; interop `spd-discard-entry` PASS, red at assertion 4 (`break-spd-out`) and 5 (`break-spd-in`) (AC-1, AC-14, AC-16) |
| Section 7.4: stateful fragment checking for BYPASS traffic with a port, established on the kernel before it is claimed (fragment investigation) | `1dc1323b86` projects a port-scoped bypass where K3 gathers fragments; guest tests `TestRFC4301ForwardedFragmentsRespectOperatorDisposition` (IPv4 forwarded), `TestRFC4301ForwardedIPv6FragmentsFollowAPortScopedBypass` (`d173045116`), `TestRFC4301CleartextFragmentsRespectOperatorDisposition` (local delivery), `TestRFC4301ForgedFragmentsTakeTheReassembledDatagramsDisposition` (`8a991db40e`, forgery); `test/ipsec/ipsec-spd-forward-bypass-projected.ci` 40 of 40, red at LIVE-STAGES line 4 with `spdForwardGathers` forced false; without bit 3 the commit refuses (`a29420771d`, `TestRFC4301PortScopedForwardBypassRefusedAtCommit`) (AC-2, AC-22) |
| Section 6 ICMP processing: errors checked against the SA's selectors, forwarded errors mapped to the return SA, non-error ICMP outside selectors dropped | `TestICMPErrorPayloadHeaderCheckedOnLocalDelivery`; `TestRFC4301ForwardedICMPErrorsMapUnderDiscard` and `TestRFC4301ForwardedICMPErrorUnderOperatorDiscardMapsToReturnSA` PASS on `qemu-c860ed8b` (`3aef55597f`); `TestRFC4301ForwardedICMPParameterProblemMapsUnderDiscard` and `TestICMPOverlapBypassTransitGate` (`5666563229`); `TestRFC4301ForwardedICMPPastBypassDecisionUsesPayloadPolicy` PASS, red with the flag removed (`42edf271d9`); `TestRFC4301NonErrorICMPOutsideSelectorsIsDiscarded`; interop `icmp-error-payload-check` PASS (AC-3, AC-4, AC-18, AC-19) |
| Section 4.4.1.1 ICMP type and code selectors, programmed exactly or refused | `TestKernelICMPSelectorDropsOnlyTheNamedMessages`, `test/ipsec/ipsec-icmp-type-selector.ci`; the port range stays a disclosed gap on `RFC4301-4.4.1.1-1`, homed in `plan/post-release/features/spec-ipsec-opaque-selector-port-mask.md` (AC-3, AC-5) |
| SPD ordering: the operator's order decides, equal orders on overlap are refused | `TestSpdOperatorOrdersOverlappingPeers`, `TestSPDPolicyEqualOrderOnOverlapRefused` with its record (re-taken in `06bd727362`); `test/parse/ipsec-spd-order-total.ci` promoted after 40 of 40 (`df31eb1c63`); `test/draft/ipsec/ipsec-spd-order-readback.ci` PASS in the guest (AC-7, AC-15) |
| PAD sub-tree matching | the six `TestPad...` units of `engine/rfc4301_pad_subtree_test.go`; interop `pad-subtree-identity` PASS, red at assertion 7 (`break-pad-true`) and 1 (`break-pad-false`) (AC-6) |
| Section 8 DF bit and PMTU | interop `tunnel-df-treatment` PASS; the `DFClear` break red at assertions 1 and 3, the outbound-only `DFCopy` break red at 4 and 6; `df-bit set` refused at commit (`LL-XFRM-OUTER-DF-SET-DSCP`); the PMTU integration units with IPv6 aging under QEMU (AC-8, AC-9) |
| Section 5.1.2.1 outer-header DSCP | `TestXFRMOuterHeaderCarriesTheOuterDSCP` on the wire; a fixed value 1 to 63 on Linux refused at commit (AC-10) |
| Interop with another implementation for the changed IPsec behaviour | strongSwan scenarios `tunnel-df-treatment`, `spd-discard-entry`, `pad-subtree-identity`, `mobike-initiator`, `mobike-responder` PASS on `c88a4387bff259f1`, with six red walks at their predicted assertions |

## RFC Documentation (Scope: protocol)

Every MUST this spec implements carries a comment directly above the enforcing code, in
the form `// RFC 4301 Section X.Y: "<quoted requirement>"`, with the sentence quoted from
`rfc/full/rfc4301.txt` and not from `rfc/short/rfc4301.md`. The fragment path, the ICMP
path and the DF path each carry an ASCII diagram of the header fields they read, with byte
offsets, because each one parses a header an attacker controls.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-22 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The scoped evidence passes, run in the main checkout by the main thread: `./le test unit pkg path ./internal/component/ike/ipsec/`, `./le test unit pkg path ./internal/component/ike/engine/`, `./le test unit pkg path ./internal/component/ike/dataplane/`, `./le test functional ipsec`, `./le test functional parse`, the named netns packet tests in the QEMU guest on Ze's kernel, and `./le test integration interop-ipsec` for each named scenario. `./le rfc check` judges the records. After a Go commit, `./le repo compiles check` judges committed dependencies; GitHub CI verifies a pushed commit. No agent runs a whole-tree verification or tests in a worktree
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Historical progress, 2026-09-06

Committed in `0aa0f63d24`. `RFC4301-7.4-2` is under investigation, not a
settled gap.
