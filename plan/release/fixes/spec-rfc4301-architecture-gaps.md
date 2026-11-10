# Spec: rfc4301-architecture-gaps

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P0 |
| Release reason | Retain remaining IPsec architecture obligations and fragment investigation without claiming an unproven vulnerability. |
| Scope | protocol |
| Depends | `plan/release/fixes/spec-ipsec-lifetime-volume.md` (owns the Section 4.4.2.1 byte-count SAD lifetime); `plan/release/validation/spec-rfcgate-6-supported-extraction-signoff.md` (owns the `rfc4301` extraction sign-off and the ledger scope set); `plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md` (the dual-form SPD bypass that turns `icmp-error-payload-check` red; its red proofs (c) and (d) wait on that fix) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Session progress, 2026-09-06

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

## Session progress, 2026-11-10

Phases 3, 4, 7 and 8 are written and reviewed. Their units, functional tests
and the PMTU QEMU unit have run green in the main checkout. Of the two interop
scenarios, `tunnel-df-treatment` passes, and `icmp-error-payload-check` is red
because of a defect outside this spec, the dual-form receive SPD bypass, so the
spec stays in progress, blocked on that defect (see "Evidence, 2026-11-10"
below). The phase table
of 2026-09-06 above is superseded for these four rows.

| Phase | Block | State |
|---|---|---|
| 3 | Section 6 ICMP | units and `.ci` green; interop `icmp-error-payload-check` RED as committed, from the dual-form SPD bypass (not a defect of this spec's code), and green with strongSwan `encap = yes`; BLOCKED on `plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md` |
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
| Interop `icmp-error-payload-check`, as committed (strongSwan sends bare ESP) | RED: "the error embedding 10.2.0.1 -> 10.7.0.5, outside the SA's selectors: ze delivered 3 Time Exceeded (Icmp.InTimeExcds), want the error dropped" (`tmp/session/2026-11-09-222726/scratch/ab-plain.log`). Cause: the dual-form receive path re-presents bare ESP over `lo`, where no inbound policy check runs; not a defect of this spec's code. Owned by `plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md` |
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

**Goal Validation.**

| Goal | Evidence | State |
|---|---|---|
| Section 4.4.1.1: an operator SPD entry selects on ICMP type and code, held exactly by the kernel | `TestSPDICMPTypeControlRejectsOrAcceptsOneType`, `test/ipsec/ipsec-icmp-type-selector.ci` (read back from the real SPD) | met |
| Section 6.2: an ICMP error on a Child SA has its payload checked, or not, per SA | `TestICMPErrorPayloadHeaderCheckedOnLocalDelivery`, `ipsec-sa-options-kernel.ci`; against strongSwan: `icmp-error-payload-check` | unit and `.ci` met; interop RED as committed from the dual-form SPD bypass, PASS with `encap = yes`; blocked on `spec-ipsec-dual-form-receive-spd-bypass` |
| Section 8.1: the outer DF bit is copy or clear per peer, on the wire | `TestXFRMOuterHeaderCarriesTheDFTreatment`, `ipsec-sa-options-kernel.ci`; against strongSwan: `tunnel-df-treatment` | met: interop PASS, red proofs (a) and (b) red at the `show` readback (the wire-counter check not reached by a break) |
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

**Red proofs owed by the two scenarios**, each one line of Ze with the image
rebuilt (`docs/architecture/testing/interop.md`, "The outer DF bit and ICMP
errors on a Child SA (RFC 4301)"):

| Proof | Scenario | Break | Expected red | Observed |
|---|---|---|---|---|
| (a) | `tunnel-df-treatment` | `xfrmDFFlag` answers false for `DFClear` | the clear phase's DF-set burst arrives with DF set | RED at the earlier `show` readback ("ze reports df-bit ... on its outbound SA, want ..."); the wire-counter check was not reached |
| (b) | `tunnel-df-treatment` | `xfrmDFFlag` answers true for `DFCopy` | the copy phase's DF-set burst arrives with DF clear | RED at the same `show` readback; the wire-counter check was not reached |
| (c) | `icmp-error-payload-check` | `xfrmStateFromParams` no longer sets `state.ICMP` | the consistent error is not delivered | NOT RUN: waits on the dual-form SPD bypass fix |
| (d) | `icmp-error-payload-check` | `xfrmPolicyFromParams` widens the inbound protect policy's source to `0.0.0.0/0` | the inconsistent error is delivered | NOT RUN: the scenario already delivers it as committed |

**Decisions.**

| Decision | Who decided | Outcome |
|---|---|---|
| DF `set`, and a fixed DSCP 1 to 63, on Linux | owner | keep the refusal at commit; `RFC4301-8.1-2` and `RFC4301-5.1.2.1-1` stay honest `{gap}` rows |
| VPP and `icmp-error-check` | rule (exact-or-refuse) | the default stays true, which is the RFC processing. VPP, reachable only through the test override `ze.test.ike.dataplane`, refuses at commit and names `icmp-error-check false` as the fix |
| A per-SA option a backend cannot apply | rule (exact-or-refuse) | refused at config verify (`verifyPeerSAOptions`) and at install with `dataplane.ErrSAOptionRefused`, never kept as a degraded SA |
| `RFC4301-6.2-3` condition: an ICMP error a bypass entry would carry in clear (an operator outbound bypass outranking a peer's protect policy, or the catch-all under `unmatched bypass`) is mapped into the return SA by Linux `icmp_route_lookup` and `icmpv6_route_lookup` | PENDING OWNER | (a) refuse the overlap at commit; (b) accept and document, the current state; (c) kernel patch, with `LL-XFRM-ICMP-TRANSIT` |
| An ICMP message arriving on a Child SA with `icmp-error-check true` bypasses an operator inbound discard when its embedded header matches the SA (the kernel's `icmp_rcv` reverse check) | PENDING OWNER | (a) refuse the overlap at commit; (b) accept and document, the current state; (c) kernel patch in `icmp_rcv` and `icmpv6_rcv` |
| Direction shown for Ze's own SAs: Ze sets no `XFRMA_SA_DIR`, so `show` renders the direction unknown and the options on both sides | PENDING OWNER | (a) set the SA direction at install (kernel 6.10 and later; the `verify_newsa_info` constraints are still to be read); (b) show the engine's belief; (c) leave it |

## Task

`docs/features/rfc-status.md` claimed RFC 4301 was `Supported` until 2026-08-30. An
extraction walk of `rfc/full/rfc4301.txt` found MUST-level architecture obligations Ze
does not meet, and none of them was on the checklist in `rfc/short/rfc4301.md`. The owner
decided on 2026-08-30 to correct the public row to `Partial` with the gaps disclosed and
to open this spec for the implementation work, one phase per block, security-bearing
blocks first.

The row remains `Partial`. This spec owns the architecture obligations in its
phase table, with the DISCARD, PAD and SPD-ordering deliveries retained as
completed history. A return to `Supported` requires the lifetime-volume work
and the extraction/sign-off owner's decision and evidence; completing this
spec alone cannot authorise that publication.

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

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - the Child SA to XFRM projection, declared by `dataplane.go`, `bypass.go`, `child.go`
  → Decision: [what the page states about how an SPD entry reaches the kernel]
  → Constraint: [what the page says a backend that cannot express a policy must do]
- [ ] `docs/architecture/ike/ipsec-dataplane-inspection.md` - how installed policy and state are read back
  → Decision: [what the page states about drift detection between Ze and the kernel]
- [ ] `docs/guide/ipsec.md` - the operator-facing IPsec surface every new config leaf lands in
  → Constraint: [the config grammar a new SPD surface must match]
- [ ] `docs/architecture/testing/interop.md` - the strongSwan suite and the vacuity traps
  → Constraint: [the red-phase proof each new scenario owes]

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4301.md` - the current checklist, 18 requirement ids, one `{gap}`
  → Constraint: the summary's prose is wrong about the implementation and this spec repairs it (see Current Behavior)
- [ ] `rfc/short/rfc7296.md` - traffic-selector negotiation and narrowing, which feeds every SPD entry IKE creates
  → Constraint: a selector Ze puts on the wire is the selector Ze programs, so a new selector field is negotiated or refused, never rounded outward
- [ ] `rfc/short/rfc4303.md` - ESP, the transform an SPD PROTECT entry names

**Key insights:** (minimal context to resume after compaction)
- The RFC's own text is the authority. `rfc/short/rfc4301.md` is a derived artifact and disagrees with the code today.
- Every obligation below was read in `rfc/full/rfc4301.txt` and every producer was read in the tree on 2026-08-30.

## Current Behavior (MANDATORY)

**Source files read:** (read on 2026-08-30, before this spec was written)
- [ ] `internal/component/ike/dataplane/dataplane.go` - `SPAction` defines `SPActionProtect` (0) and `SPActionBypass` (1) and nothing else. `SPParams` carries `Src`, `Dst`, `Dir`, `Proto`, `Mode`, `IfID`, `ReqID`, `Action`, `Owner`, `Priority`, `SrcPort`, `DstPort`, `UpperProto`, `IfIndex`, `TunnelSrc`, `TunnelDst`, `SAID`. `PortMatch` holds a port and a mask, and `AnyPortMatch` and `ExactPortMatch` are its two constructors. `PriorityIKEBypass` is 100 and `PriorityChildSA` is 2000, both fixed constants.
- [ ] `internal/component/ike/engine/bypass.go` - `ikeBypassPolicies` builds four BYPASS policies per address family: an outbound pair matching `SrcPort` exactly on UDP 500 and 4500 with any destination port, and an inbound pair with the ports reversed. Source and destination prefixes are the any-address net.
- [ ] `internal/component/ike/engine/child.go` - `createFirstChildSA` sets the Child SA mode to tunnel, and to transport when `sa.UseTransportMode` is set. `childPolicyParams` builds the `SPParams` for a negotiated Child SA.
- [ ] `internal/component/ike/engine/remote_id.go` - `remoteIDMatches` compares an asserted identity with the configured `remote-id` after a class gate. ID_FQDN and ID_RFC822_ADDR compare with `asciiEqualFold`, ID_DER_ASN1_DN renders to RFC 4514 and compares with `asciiEqualFold`, ID_KEY_ID compares byte for byte, and an address parses through `netip.ParseAddr` and compares as one value.
- [ ] `internal/component/ike/engine/rekey.go` - `lifetimeState` holds `softTime`, `hardTime`, `softBytes` and `byteCount`. `newLifetimeState` assigns `softTime` and `hardTime` only. `softExpired` reads `softBytes` in its second arm, and that arm is unreachable in a running daemon because nothing assigns the field outside tests.
- [ ] `internal/component/ike/engine/initiator.go` - `tsToIPNet` is deleted; a comment at the deletion site records that `wireToSelectors` in `ts_narrow.go` replaced it, converts every selector, and keeps the port and the protocol.
- [ ] `internal/plugins/ospf/ipsec_install.go` - the RFC 4552 manual path installs transport-mode ESP or AH through the same dataplane seam.
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - the operator config surface. `vpn ipsec` carries `esp-group`, `ike-group`, `remote-access` and `site-to-site peer`, and a peer carries a `traffic-selector` list with `local` and `remote` prefixes and ports. There is no operator-authored SPD entry, no policy action leaf and no policy order leaf.
- [ ] `docs/features/rfc-status.md` - the RFC 4301 row, now `Partial`, naming this spec.
- [ ] `rfc/short/rfc4301.md` - 18 requirement ids, one `{gap}` on `RFC4301-4.4.1.1-1`, seven `{not-applicable}` annotations, and prose that says Ze delegates IKEv2 negotiation to strongSwan and charon.

**Behavior to preserve:**
- The four IKE bypass policies keep exempting Ze's own IKE sockets. A Child SA policy that captured IKE would prevent its own rekey and teardown.
- `SPActionProtect` stays the zero value of `SPAction`, so a caller that forgets the field gets a policy that protects rather than one that passes traffic in the clear.
- A backend that cannot express a policy refuses the install rather than widening or downgrading it.
- Every existing `test/interop-ipsec/scenarios/` scenario keeps passing against strongSwan.

**Behavior to change:**
- An SPD entry can carry a DISCARD action, an ICMP type and code selector, and a port range.
- A BYPASS entry with a non-trivial port range is served by stateful fragment checking, or its install is refused.
- An administrator can accept or reject unauthenticated ICMP error messages at ICMP-type granularity, on both sides of the IPsec boundary.
- A protected transit ICMP error message can be checked for consistency between its payload header and the traffic selectors of the SA that carried it.
- An administrator can order SPD entries totally through the management interface.
- The DF treatment of a tunnel-mode SA is configurable as set, clear, or copy from the inner header.
- A per-SA PMTU is held and aged.
- The outer tunnel header's DSCP value is mapped for the domain the packet enters.
- PAD identity matching accepts a sub-tree for a DN, a DNS name and an RFC 822 address, and an address range.

## Existing Mechanisms (MANDATORY)

Each row was checked against the source on 2026-11-10.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| XFRM selector port with mask | `internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch` (adds `XfrmPolicy.SrcPortMask` and `DstPortMask`; `selFromPolicy` writes them, `parseXfrmPolicy` reads them back); `dataplane.PortMatch` (`dataplane.go`) | Carries a selector port as a value plus a mask, so a mask-aligned block is programmed exactly and read back exactly | Extends. Phase 4 puts the ICMP type in the source port and the code in the destination port with their masks (`dataplane.IsICMPProto`), and splits a code range into mask-aligned blocks, one policy each (`spdICMPMatches`, engine `spd_policy.go`). No new selector field and no rounding outward |
| Operator SPD entry list | `internal/component/ike/ipsec/spd_policy.go` (`SPDPolicy`, `ICMPSelector`, `parseSPDPolicy`, `parseICMPSelector`); `internal/component/ike/engine/spd_policy.go` (`spdPolicyParams`, `installSPDPolicies`) | `vpn ipsec policy <name>` with action, order, direction, protocol, local and remote prefix and port, and an ICMP type and code; each entry becomes out and in policies, a both-direction entry a mirrored pair | Extends. Phase 4 added the ICMP selector to this list rather than a second one (R-3). It still installs out and in only, never fwd, which is the open `RFC4301-6-1` gap (Work Not Done) |
| Dataplane SA and SP seam with exact-or-refuse | `dataplane.SAParams` (`DF`, `OuterDSCP`, `ICMPErrorCheck`), `dataplane.SPParams` (`ICMPErrorReturn`), `outerHeaderTreatmentError` and `ErrSAOptionRefused` (`dataplane/outer_header.go`); `xfrmNoPMTUDisc`, `xfrmDontEncapDSCP`, `xfrmStateFromParams` (`xfrm_linux.go`); `vppDFError`, `vppICMPErrorCheckError` (`vpp.go`), `vppICMPSelectorError` (`vpp_policy.go`) | Each backend programs an option exactly or refuses the install with `ErrSAOptionRefused`, which does not wrap `ErrNotSupported`, so `isXFRMUnsupported` (`engine/child.go`) refuses the Child SA rather than keeping it degraded | Extends. Phases 3, 7 and 8 added the three per-SA options and the return-SA mapping flag to the existing value types, and every refusal goes through the existing error rather than a new path |
| Commit-time refusal of what the backend cannot enforce | `verifyUnmatchedEnforceable` (`engine/unmatched.go`, through `dataplane.CatchAllInstaller`); `verifyPeerSAOptions` (`engine/sa_options.go`, through `dataplane.SAOptionChecker`) | The catch-all check refuses `unmatched discard` at verify when the backend cannot hold it; the per-peer check refuses a `df-bit`, `dscp-map` or `icmp-error-check` value the loaded backend cannot apply, from the same predicate the install path uses | Extends. `verifyPeerSAOptions` mirrors `verifyUnmatchedEnforceable`: a backend that implements no checker has declared nothing, and anything but the defaults is refused on it |
| Dataplane readback | `internal/component/ike/cmd/show_dataplane.go` (`saInfoToMap`, `saInfoRow`, `handleShowVPNIPsecDataplanePolicy`) | `show vpn ipsec dataplane sa` renders `df-bit`, `dscp-map`, `icmp-error-check` and `options-known`, null when unread, and null on the side an option cannot ride when the SA's direction is known | Extends. The interop checkers read the outbound SA's `df-bit` here. An SA with no recorded direction renders every option as read (PENDING OWNER row in the 2026-11-10 decisions) |
| Kernel route PMTU exceptions | Linux `__ip_rt_update_pmtu` (net/ipv4/route.c), `rt6_do_update_pmtu` (net/ipv6/route.c), aged by `net.ipv4.route.mtu_expires` and `net.ipv6.route.mtu_expires` (registered in `internal/core/sysctl/known_linux.go`) | The kernel learns a path MTU under the XFRM bundle and expires it back to the link MTU after the period | Does not apply as a Ze mechanism to extend: Ze holds no PMTU of its own. Phase 7 relies on it for Section 8.2.2 instead of the planned `engine/pmtu.go` (A-4), with the period set through the existing sysctl surface, and records it as a kernel-producer escape |

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
| Boundary | How | Verified |
|----------|-----|----------|
| Config to engine | YANG tree to ipsec config value types | No |
| Engine to dataplane | `dataplane.SPParams` and `dataplane.SAParams` value types | No |
| Dataplane to kernel | netlink XFRM messages | No |
| Dataplane to VPP | VPP binary API | No |
| Ze to peer | IKEv2 traffic selector payloads | No |

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
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | DISCARD was absent in the 2026-08-30 baseline | original `SPAction` read | phase 2a would be smaller if already implemented | 2026-09-06 delivery record above | confirmed then cleared by phase 2a; retained as history |
| A-2 | Ze holds no IP fragment state anywhere in the IPsec path | a case-insensitive search for `fragment` over `internal/component/ike/` returned only EAP-TLS message fragmentation, 2026-08-30 | phase 2 has an existing mechanism to extend rather than to add | re-run the search at the start of phase 2 and read every hit | confirmed |
| A-3 | The Linux kernel may not discharge Section 7.4 for a template-free XFRM policy on its own | hypothesis from the absence of a fragment selector field; kernel reassembly and policy order remain unproven | the obligation may be kernel-delegated and need evidence rather than new fragment state | read the producing kernel path and run the QEMU non-initial-fragment scenario before selecting enforcement | unvalidated; final 2026-09-06 entry says under investigation |
| A-4 | The Section 8.2.2 PMTU aging obligation is met by the kernel's own PMTU expiry for an XFRM route | the kernel ages a dst entry's PMTU, and Ze holds no PMTU value of its own | the phase must add a per-SA PMTU field, its aging timer and its reset period | read the producing kernel path (`__ip_rt_update_pmtu`, net/ipv4/route.c; `rt6_do_update_pmtu`, net/ipv6/route.c), then the namespace tests `TestXFRMTunnelPMTUAgesBackToTheLinkMTU`, `TestXFRMPolicyPMTUAgesBackToTheLinkMTU` and `TestXFRMTunnelPMTUAgesBackToTheLinkMTUOverIPv6` (`internal/component/ike/dataplane/rfc4301_pmtu_*integration_linux_test.go`), recorded as kernel-producer escapes | validated by the kernel read this session (the path the `RFC4301-8.2.2-1` annotation cites); the three tests are written and pending run by the lead, so not confirmed until they run |
| A-5 | The seven `{not-applicable}` annotations on `rfc/short/rfc4301.md` are void under the owner directive of 2026-07-27 and must be re-answered rather than cited | `ai/rules/rfc-compliance.md`, "Every earlier answer that pointed away from full compliance or full proof is VOID" | the annotations stand and phase 9 shrinks to a prose repair | read each annotation's producing function and record the fresh answer | unvalidated |
| A-6 | The VPP backend can express a DISCARD policy | 2026-09-06 read of `IPSEC_API_SPD_ACTION_DISCARD` in the VPP API binding | a backend without the action must refuse rather than substitute | session evidence above | confirmed in the recorded phase 2a work |
| A-7 | No existing interop scenario would break when `SPAction` gains a member and `SPParams` gains fields | the zero value of every new field means "as before", and `SPActionProtect` stays 0 | a landed scenario reds and the change is wider than one enum member | run the whole `test/interop-ipsec` suite at the end of phase 2 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A phase classifies an obligation `{not-applicable}` because the kernel performs the packet-level half, and the gate goes green over an unmet MUST | a new `{not-applicable}` annotation appears in `rfc/short/rfc4301.md` | `ai/rules/rfc-compliance.md` reserves that call to the owner. The kernel performing a step is evidence to put to him, never a classification a phase makes. Ask which way to fix it |
| R-2 | The fragment work grows into a general connection-tracking subsystem | phase 2 starts naming a new package outside `internal/component/ike/` | the obligation is scoped to BYPASS and DISCARD entries with a non-trivial port range. Reassembly for exactly those entries is the deliverable (`ai/rules/simplicity.md`) |
| R-3 | A new SPD config surface duplicates the `traffic-selector` list a peer already carries | two config paths describe one selector | one declaration, and the peer selector list derives from it or names it. `ai/rules/principles.md` forbids the second copy |
| R-4 | An operator order leaf collides with `PriorityIKEBypass`, and a Child SA policy captures IKE | an interop scenario stops rekeying | the IKE bypass keeps a reserved band no operator order can reach, and a config validator refuses an order inside it |
| R-5 | A completion report claims Supported before the external dependencies and owner sign-off are satisfied | phase 9 treats this spec's tests as sole publication authority | AC-12 remains blocked on lifetime-volume completion and the extraction/sign-off owner's recorded decision and evidence; no unilateral supportClaimingScope change |
| R-6 | The ICMP block is large enough to exceed one agent, and gets trimmed to fit | an agent reports partial coverage of Section 6 | the package boundary is the BLOCK. An agent whose block is too big reports the size to the main thread, which re-cuts by section (`ai/rules/planning.md`) |
| R-7 | A new selector field is programmed but never negotiated, so Ze installs a policy the peer never proposed | an interop scenario shows a selector mismatch | RFC 7296 Section 2.9 narrowing is the existing rule: a selector Ze programs is a selector Ze negotiated, or the install is refused |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Live IPsec sessions. A wrong SPD action black-holes traffic an operator asked to pass, and a wrong bypass passes traffic an operator asked to protect. A wrong DF or PMTU setting silently drops large packets. A wrong PAD match authorizes a peer the operator did not name |
| How is it reverted? | Per phase, by one commit. The enum member and the new selector fields are additive and their zero values are the previous behavior, so a revert of a later phase does not disturb an earlier one. The ledger row in phase 9 is one line |
| Who else touches this path? | `plan/release/fixes/spec-ipsec-lifetime-volume.md` owns `lifetimeState`. `plan/release/validation/spec-rfcgate-6-supported-extraction-signoff.md` owns `rfc/extraction/rfc4301.json` and the `supportClaimingScope` list in `internal/le/rfc/check_test.go`. `plan/release/validation/spec-ipsec-0-umbrella.md` and the other `plan/spec-ipsec-*` specs work the same engine |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set vpn ipsec policy <name> action discard` committed | → | the SPD config to `dataplane.SPParams` path, carrying `SPActionDiscard` | `TestDiscardPolicyReachesTheBackend` |
| A forged non-initial fragment matching a port-scoped BYPASS policy | → | the fragment check on the BYPASS path | `TestNonInitialFragmentIsNotBypassed` |
| `set vpn ipsec icmp unauthenticated <type> reject` committed | → | the ICMP acceptance control | `TestUnauthenticatedICMPTypeRejected` |
| A protected transit ICMP error whose payload header contradicts the SA selectors | → | the transit ICMP consistency check | `TestTransitICMPPayloadHeaderChecked` |
| `set vpn ipsec policy <name> selector icmp-type <n> icmp-code <n>` committed | → | the ICMP selector fields on `SPParams` | `TestICMPSelectorReachesTheBackend` |
| A peer asserting `host.example.com` against `remote-id .example.com` | → | `remoteIDMatches` sub-tree arm | `TestRemoteIDSubTreeMatch` |
| `set vpn ipsec policy <name> order <n>` committed | → | `SPParams.Priority` derived from the operator order | `TestPolicyOrderReachesTheBackend` |
| `set vpn ipsec site-to-site peer <name> df-bit copy` committed | → | the DF treatment on the installed SA | `TestDFBitTreatmentReachesTheBackend` |
| `set vpn ipsec site-to-site peer <name> dscp-map <value>` committed | → | the outer-header DSCP mapping | `TestOuterHeaderDSCPReachesTheBackend` |
| `./le rfc check` over the closing tree | → | `checkSupportedSignoff` and `checkStatusAgreement` (`internal/le/rfc/check_status.go`) | `TestSupportedRowsHaveDerivableScope` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An operator writes an SPD entry whose action is discard | matching traffic is dropped by the installed policy, and a backend that cannot express the action refuses the install with an error naming the backend and the action |
| AC-2 | A forged non-initial fragment arrives whose addresses and protocol match a BYPASS entry that names a non-trivial port range | the fragment is not passed in the clear, and the event is counted and logged |
| AC-3 | An administrator configures unauthenticated ICMP error messages of a given ICMP type to be rejected | an ICMP error of that type arriving on the unprotected side is dropped, and one of a permitted type is processed. The same control applies to the protected side |
| AC-4 | A protected transit ICMP error message arrives whose payload header, with source and destination addresses and ports reversed, does not match the traffic selectors of the SA it arrived on | the message is dropped when the check is configured, and forwarded when an SPD entry explicitly allows carriage of such traffic |
| AC-5 | An SPD entry names an ICMP type and code, or a port range | the selector is programmed exactly, and an entry the backend cannot program exactly is refused rather than widened |
| AC-6 | A peer asserts an identity that falls under a configured sub-tree, and another peer asserts one that does not | the first authenticates and the second is refused. The same holds for a DN sub-tree, a partially qualified RFC 822 address, and an address inside and outside a configured range |
| AC-7 | An administrator gives two overlapping SPD entries an explicit order | the first-ordered entry decides the disposition, the order survives a restart, and an order that would outrank the IKE bypass is refused at commit |
| AC-8 | An SA is configured with each of set, clear and copy for the DF bit | the outer header of an emitted tunnel-mode packet carries the configured DF value, observed on the wire |
| AC-9 | A per-SA PMTU is learned and the aging period elapses | the PMTU returns to the first-hop data-link MTU and is relearned, and the period is configurable |
| AC-10 | A packet enters a domain for which the inner DSCP value is not appropriate and a mapping is configured | the outer tunnel header carries the mapped value, observed on the wire |
| AC-11 | `rfc/short/rfc4301.md` is read at closure | its prose describes the native IKEv2 engine rather than strongSwan and charon; the seven `{not-applicable}` annotations and the `{gap}` carry a fresh answer recorded from the owner; the `{single-polarity}` reason on `RFC4301-4.1-3` no longer says the child-SA path hardcodes tunnel mode |
| AC-12 | `docs/features/rfc-status.md` and `./le rfc check` are read at closure | the RFC 4301 row reads `Supported` with a Remaining cell that claims no gap, `rfc4301` is back in `supportClaimingScope` with its pinning assertion removed, `rfc/extraction/rfc4301.json` is a sign-off the check accepts, and `./le rfc check` names `rfc4301` in no violation |

AC-8 note (2026-11-10): `df-bit set` is refused at commit by owner decision,
because neither dataplane can write a set DF bit on an ESP tunnel header
(`LL-XFRM-OUTER-DF-SET-DSCP`). The `tunnel-df-treatment` scenario therefore
covers `clear` and `copy`, and `RFC4301-8.1-2` keeps its `{gap}` for `set`.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An operator writes an SPD entry that discards traffic to a subnet | config to ipsec types to `SPParams` with `SPActionDiscard` to the backend to the kernel SPD | `TestDiscardPolicyReachesTheBackend` plus the `spd-discard-entry` interop scenario |
| 2 | An attacker forges a non-initial fragment aimed at a bypassed port range | interface to the fragment check to a drop | `TestNonInitialFragmentIsNotBypassed` plus the `bypass-fragment-forgery` interop scenario |
| 3 | An operator rejects unauthenticated ICMP Destination Unreachable | config to the ICMP acceptance control to a drop | `TestUnauthenticatedICMPTypeRejected` plus a `.ci` over the operator path |
| 4 | An operator authorizes every peer under one DNS sub-tree with one PAD entry | config to `remoteIDMatches` to the authentication verdict | `TestRemoteIDSubTreeMatch` plus the `pad-subtree-identity` interop scenario |
| 5 | An operator orders two overlapping SPD entries and reads them back | config to `SPParams.Priority` to the kernel, and back through the inspection path | `TestPolicyOrderReachesTheBackend` plus a `.ci` reading the order back |
| 6 | An operator pins the DF bit of a tunnel to clear so a legacy path stops dropping large packets | config to the installed SA to the emitted outer header | `TestDFBitTreatmentReachesTheBackend` plus the `tunnel-df-treatment` interop scenario |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDiscardPolicyReachesTheBackend` | `internal/component/ike/engine/bypass_test.go` | the discard action survives the config to `SPParams` path | |
| `TestBackendRefusesUnexpressibleAction` | `internal/component/ike/dataplane/vpp_policy_test.go` | a backend that cannot express DISCARD refuses rather than downgrading | |
| `TestNonInitialFragmentIsNotBypassed` | `internal/component/ike/engine/bypass_test.go` | a non-initial fragment matching a port-scoped BYPASS entry is not passed | |
| `TestUnauthenticatedICMPTypeRejected` | `internal/component/ike/engine/icmp_test.go` | the per-type acceptance control decides an unauthenticated ICMP error | |
| `TestTransitICMPPayloadHeaderChecked` | `internal/component/ike/engine/icmp_test.go` | the payload header with addresses and ports reversed is matched against the SA selectors | |
| `TestICMPSelectorReachesTheBackend` | `internal/component/ike/dataplane/dataplane_test.go` | an ICMP type and code selector is programmed exactly | |
| `TestPortRangeSelectorRefusedWhenNotExpressible` | `internal/component/ike/dataplane/dataplane_test.go` | a range the backend cannot program is refused, never widened |  |
| `TestRemoteIDSubTreeMatch` | `internal/component/ike/engine/remote_id_test.go` | a partial DNS name, a partial RFC 822 address and a DN sub-tree each match under and only under their sub-tree | |
| `TestRemoteIDAddressRangeMatch` | `internal/component/ike/engine/remote_id_test.go` | an address identity inside a configured range matches and one outside it does not | |
| `TestPolicyOrderReachesTheBackend` | `internal/component/ike/engine/child_test.go` | an operator order becomes the policy priority and keeps the IKE bypass band reserved | |
| `TestPolicyOrderInsideBypassBandRefused` | `internal/component/ike/ipsec/validate_test.go` | an order that would outrank the IKE bypass is refused at commit | |
| `TestDFBitTreatmentReachesTheBackend` | `internal/component/ike/dataplane/dataplane_test.go` | each of set, clear and copy reaches the installed SA | |
| `TestPMTUAgingPeriodConfigurable` | `internal/component/ike/engine/pmtu_test.go` | the aging period is read from config and bounded | |
| `TestOuterHeaderDSCPReachesTheBackend` | `internal/component/ike/dataplane/dataplane_test.go` | the mapped DSCP value reaches the installed SA | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| SPD entry order | 1 to 65535 | 65535 | 0 | 65536 |
| ICMP type selector | 0 to 255 | 255 | N/A | 256 |
| ICMP code selector | 0 to 255 | 255 | N/A | 256 |
| Port range endpoints | 0 to 65535 | 65535 | N/A | 65536 |
| DSCP value | 0 to 63 | 63 | N/A | 64 |
| PMTU aging period, seconds | 60 to 86400 | 86400 | 59 | 86401 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-spd-discard-entry` | `test/ipsec/ipsec-spd-discard-entry.ci` | an operator writes a discard entry and sees matching traffic dropped | |
| `ipsec-bypass-fragment-check` | `test/ipsec/ipsec-bypass-fragment-check.ci` | a forged non-initial fragment aimed at a bypassed port range is dropped and counted | |
| `ipsec-icmp-unauthenticated-control` | `test/ipsec/ipsec-icmp-unauthenticated-control.ci` | an operator rejects one ICMP type and permits another, and reads the result back | |
| `ipsec-spd-order-readback` | `test/ipsec/ipsec-spd-order-readback.ci` | an operator orders two overlapping entries and reads the order back through the inspection path | |
| `ipsec-df-bit-treatment` | `test/ipsec/ipsec-df-bit-treatment.ci` | an operator sets each DF treatment and the emitted outer header carries it | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `spd-discard-entry` | `test/interop-ipsec/scenarios/` | strongSwan | a discard entry drops traffic strongSwan would otherwise carry, and the tunnel beside it stays up | |
| `bypass-fragment-forgery` | `test/interop-ipsec/scenarios/` | strongSwan | a forged non-initial fragment aimed at the bypassed IKE port range does not reach the protected side | |
| `icmp-error-payload-check` | `test/interop-ipsec/scenarios/` | strongSwan | an ICMP error addressed to Ze (local delivery) arriving on the SA: one whose payload header is consistent with the SA selectors is delivered, an inconsistent one is dropped. A transit (forwarded) error is out of scope here and stays `LL-XFRM-ICMP-TRANSIT` | RED as committed from the dual-form SPD bypass, PASS with `encap = yes`; red proofs wait on that fix |
| `pad-subtree-identity` | `test/interop-ipsec/scenarios/` | strongSwan | a strongSwan peer whose ID_FQDN falls under a configured sub-tree authenticates, and one outside it is refused | |
| `tunnel-df-treatment` | `test/interop-ipsec/scenarios/` | strongSwan | the outer header DF bit carries each configured treatment, read on the wire | PASS; both red proofs red at the `show` readback |

## Files to Modify
- `internal/component/ike/dataplane/dataplane.go` - `SPAction` gains DISCARD; `SPParams` gains ICMP type and code selectors, a port range form, a DF treatment and a DSCP mapping; the two priority constants become a reserved band plus an operator order
- `internal/component/ike/dataplane/xfrm_linux.go` - the Linux projection of every new field, and the refusal for a field it cannot express
- `internal/component/ike/dataplane/vpp_policy.go` - the VPP projection, and the refusal for a field it cannot express
- `internal/component/ike/engine/bypass.go` - the fragment check on the BYPASS path
- `internal/component/ike/engine/child.go` - `childPolicyParams` carries the new selector and priority fields
- `internal/component/ike/engine/remote_id.go` - `remoteIDMatches` gains sub-tree and range matching
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - the SPD entry list, the ICMP controls, the DF treatment, the DSCP mapping and the PMTU aging period
- `internal/component/ike/ipsec/validate.go` - the commit-time validators for the new leaves
- `internal/core/diagnostic/codes.go` - a code per new runtime dependency a doctor check reads
- `internal/component/ike/engine/doctor.go` - the doctor checks for the new dependencies
- `rfc/short/rfc4301.md` - the prose repair, the fresh answers on the seven `{not-applicable}` annotations and the `{gap}`, the corrected `{single-polarity}` reason on `RFC4301-4.1-3`, and one new requirement id per obligation this spec implements
- `docs/features/rfc-status.md` - the RFC 4301 row, raised to `Supported` in the closing phase
- `internal/le/rfc/check_test.go` - `rfc4301` returns to `supportClaimingScope` and its pinning assertion is removed
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - declared by the `// Design:` header of `dataplane.go`, `xfrm_linux.go`, `vpp_policy.go`, `bypass.go` and `child.go`. The SPD action set, the selector set and the policy ordering it describes all change
- `docs/architecture/ike/ipsec-dataplane-inspection.md` - the policy fields the inspection path reads back
- `docs/architecture/ike/ipsec-7-ikev2-engine.md` - declared by the `// Design:` header of `remote_id.go` and `doctor.go`. The PAD matching rules it describes change in phase 5, and it gains the new doctor checks in phase 3
- `docs/architecture/ike/ipsec-3-data-model.md` - declared by the `// Design:` header of `internal/component/ike/ipsec/validate.go`. The cross-reference validation it describes gains the SPD order, ICMP type and code, port range, DSCP and PMTU period rules
- `docs/features/ai-first.md` - declared by the `// Design:` header of `internal/core/diagnostic/codes.go`. Named here as affected only by the new diagnostic codes: the page describes how a code reaches an agent, and adding codes changes the list rather than the mechanism
- `docs/guide/ipsec.md` - the new operator config
- `docs/features.md` - the IPsec feature rows
- `docs/functional-tests.md` - the five new `.ci` names
- `docs/architecture/testing/interop.md` - the five new scenario names

## Files to Create
- `internal/component/ike/engine/icmp.go` - the ICMP acceptance controls and the transit consistency check
- `internal/component/ike/engine/pmtu.go` - the per-SA PMTU value and its aging
- `internal/component/ike/engine/fragment.go` - the fragment state for BYPASS and DISCARD entries with a non-trivial port range
- `test/ipsec/*.ci` - the five functional tests named above
- `test/interop-ipsec/scenarios/<name>/` - the five scenarios named above
- the retired deferral shard "rfc4301-architecture-gaps" - the shard named in the metadata table

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` for the SPD entry list, the ICMP controls, the DF treatment, the DSCP mapping and the PMTU aging period |
| YANG validation constraints | Yes | every new leaf takes `range`, `enumeration` or `pattern`: the order, the ICMP type and code, the port range endpoints, the DSCP value and the aging period all have the bounds in the Boundary Tests table |
| YANG custom validators | Yes | the order leaf needs a validator that refuses the reserved IKE bypass band, which no native constraint can express |
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

The phase table at the top is the remaining-work authority. The original
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
   - Tests: `TestSupportedRowsHaveDerivableScope`, `./le rfc check`
   - Files: `rfc/short/rfc4301.md`, `docs/features/rfc-status.md`, `internal/le/rfc/check_test.go`, `rfc/extraction/rfc4301.json`
   - Verify: every void annotation carries a fresh answer, every new obligation carries a new requirement id and a tagged test, the row reads `Supported`, and `./le rfc check` names `rfc4301` in no violation. The sign-off artifact lands under the rules of `plan/release/validation/spec-rfcgate-6-supported-extraction-signoff.md`

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
| Fragments are checked | `test/ipsec/ipsec-bypass-fragment-check.ci` passes and the `bypass-fragment-forgery` scenario passes |
| ICMP controls exist | `./le config schema` shows the ICMP leaves and `ipsec-icmp-unauthenticated-control.ci` passes |
| Selectors are complete | `RFC4301-4.4.1.1-1` carries no `{gap}` annotation and `rfc/requirements/rfc4301.md` lists a test for it |
| PAD sub-tree matching exists | the `pad-subtree-identity` scenario passes against strongSwan |
| SPD ordering exists | `ipsec-spd-order-readback.ci` passes |
| DF and PMTU are configurable | the `tunnel-df-treatment` scenario passes and `TestPMTUAgingPeriodConfigurable` passes |
| DSCP is mapped | `TestOuterHeaderDSCPReachesTheBackend` passes |
| The summary is repaired | `grep -in strongswan rfc/short/rfc4301.md` returns nothing that describes Ze's own negotiation |
| The public row is raised | the RFC 4301 row reads `Supported` and `./le rfc check` names `rfc4301` in no violation |

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
| `guard-added-to-one-half-of-a-pair.md` | 2026-11-10 | RFC4301-6-1: operator SPD entries install out and in, never fwd |
| `silent-fall-through.md` | 2026-11-10 | forwarded fragments (port 0) miss a port-specific Child SA policy and meet the catch-all, in clear under `unmatched bypass` |
| `field-carries-two-meanings.md` | 2026-11-10 | protocol 135: the IKE MH-type value is installed as a port, Linux compares the bare type |
| `two-owners-share-one-name.md` | 2026-11-10 | reqid 1 and SPI-less templates on every Child SA of a peer; a sibling's any-port fwd policy accepts a fragment of an exact-port SA |
| `guard-enumerates-instead-of-subtracting.md` | 2026-11-10 | RFC7296-3.13.1-1/2 weak: a port under a portless protocol other than 0 is committed and emitted |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Correct the public row first and implement second | Implement inside `plan/release/validation/spec-rfcgate-6-supported-extraction-signoff.md`; leave the row at `Supported` | Owner decision, 2026-08-30. The row is a public promise that is false today, and correcting it costs one line. Leaving it was not on the table, and folding eight blocks of protocol work into a bookkeeping spec would have made that spec unclosable |
| One phase per block | One phase for the whole RFC; one phase per requirement id | A block shares one subject matter and one surface, which is the unit an agent can hold. A requirement id is too small to justify a phase report and the whole RFC is too large for one context (`ai/rules/planning.md`) |
| Fragments before ICMP | Follow the RFC's own section order | The fragment gap is reachable on a running daemon by a forged packet, and the ICMP gap is a missing control. Blast radius orders the phases, not the document |
| The byte-count SAD lifetime stays in its own spec | Implement Section 4.4.2.1 here | `plan/release/fixes/spec-ipsec-lifetime-volume.md` already designs it. A second design of one fact is a future disagreement with nothing to arbitrate it (`ai/rules/principles.md`) |
| A new operator-authored SPD entry list | Extend the peer `traffic-selector` list with an action and an order | A DISCARD entry and a BYPASS entry are not attached to a peer, and Section 4.4.1 asks for a management interface over the SPD itself. Overloading the peer selector list would put two meanings on one leaf |
| The IKE bypass keeps a reserved priority band | Let an operator order every entry freely | A Child SA policy that outranks the IKE bypass prevents its own rekey and teardown. The band is refused at commit, where the operator can read the reason |

## Known Limitations

- **It does not implement the Section 4.4.2.1 byte-count SAD lifetime.** `plan/release/fixes/spec-ipsec-lifetime-volume.md` designs it and this spec depends on it. Closure of this spec needs that work landed, because the ledger row cannot read `Supported` while a MUST of Section 4.4.2.1 is unmet.
- **It does not write `rfc/extraction/rfc4301.json`.** That artifact belongs to `plan/release/validation/spec-rfcgate-6-supported-extraction-signoff.md`. Phase 9 lands it under that spec's rules and its checks.
- **It does not answer the Section 6.1.1 MAY.** ICMP type-and-code granularity is a MAY, and `ai/rules/rfc-compliance.md` reserves a MAY to the owner. Phase 3 puts it to him with the cost of each option.
- **It does not widen the class gate in `remoteIDMatches`.** Sub-tree matching is added inside each existing class. An asserted ID_FQDN still cannot satisfy an address-valued `remote-id`.

## Work Not Done

As of 2026-11-10. Each row stays owed; none is dropped.

| Item | Why not done | Home |
|------|--------------|------|
| `RFC4301-6-1`: project operator SPD entries onto fwd, so a gateway can bypass or discard transit ICMP | needs a decision first on how a non-initial fragment, which carries no ICMP type, is classified (Section 7, phase 2b) | this spec, `plan/release/fixes/spec-rfc4301-architecture-gaps.md`; journal row `guard-added-to-one-half-of-a-pair.md` 2026-11-10, owned above |
| Stale names in the post-release port-mask spec | kept as they are by owner decision | `plan/post-release/features/spec-ipsec-opaque-selector-port-mask.md` |
| `RFC4301-6.2-3`: an ICMP error a bypass entry would carry in clear is mapped into the return SA | PENDING OWNER: refuse the overlap at commit, accept and document, or a kernel patch | this spec, `plan/release/fixes/spec-rfc4301-architecture-gaps.md` (2026-11-10 decisions) |
| An ICMP message on a Child SA with `icmp-error-check true` bypasses an operator inbound discard | PENDING OWNER: refuse the overlap at commit, accept and document, or a kernel patch in `icmp_rcv` and `icmpv6_rcv` | this spec, `plan/release/fixes/spec-rfc4301-architecture-gaps.md` (2026-11-10 decisions) |
| Direction of Ze's own SAs in `show` (no `XFRMA_SA_DIR` set) | PENDING OWNER: set the SA direction at install, show the engine's belief, or leave it | this spec, `plan/release/fixes/spec-rfc4301-architecture-gaps.md` (2026-11-10 decisions) |
| Phase 2b, Section 7.4 stateful fragment checking | unchanged, under investigation (A-3) | this spec, `plan/release/fixes/spec-rfc4301-architecture-gaps.md` |
| Forwarded fragments of a port-specific Child SA meet the catch-all (port 0), crossing in clear under `unmatched bypass` | found 2026-11-10 reading LL-XFRM-FRAGMENT-SELECTOR; the fix belongs with phase 2b | this spec; journal row `silent-fall-through.md` 2026-11-10, owned above |
| Protocol 135 (MH) selector installed with the IKE value (type in the high octet) as the port | found 2026-11-10; Linux `xfrm_flowi_sport` compares the bare type | this spec; journal row `field-carries-two-meanings.md` 2026-11-10, owned above |
| Shared reqid 1 with SPI-less templates across a peer's Child SAs | found 2026-11-10; strains Section 7.3 for fragments of an exact-port SA | this spec; journal row `two-owners-share-one-name.md` 2026-11-10, owned above |
| RFC 7296 Section 3.13.1: a port under a portless protocol other than 0 | found by the 2026-11-10 audit re-judge (RFC7296-3.13.1-1 and -2 now `weak`) | this spec; journal row `guard-enumerates-instead-of-subtracting.md` 2026-11-10, owned above |
| Phase 9, summary, annotations and the public row | unchanged, blocked on both Depends specs | this spec, `plan/release/fixes/spec-rfc4301-architecture-gaps.md` |

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
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

## Progress, 2026-09-06

Committed in `0aa0f63d24`. `RFC4301-7.4-2` is under investigation, not a
settled gap.
