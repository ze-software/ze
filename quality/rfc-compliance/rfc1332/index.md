# RFC 1332 - The PPP Internet Protocol Control Protocol (IPCP)

Partial. Every requirement this repository extracted from RFC 1332, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 75.0% | 6 of 8 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 8 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 8 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 100.0% | 15 of 15 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 8 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 8 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 12.5% | 1 of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 8 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 8 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 12.5% | 1 of 8 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 8 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 14 |
| Gated MUST-level | 8 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 15 |
| Tagged units | 15 |
| Recorded audit verdicts | 6 |
| Discrimination records | 15 |
| Summary | `rfc/short/rfc1332.md` |
| Requirement shard | `rfc/requirements/rfc1332.md` |
| RFC text | `rfc/full/rfc1332.txt` |

## Enrolment

Enrolled: PPP Internet Protocol Control Protocol under L2TP and PPPoE. Ze implements the IPCP option codec, address negotiation and interface programming. Tagged tests cover packet framing, negotiation ordering, address suggestions and the MTU passed to the backend. The checklist retains the unsupported IPCP-code gap and distinguishes Linux IP fragmentation from Ze's MTU setup.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

IPv4 address negotiation and pool integration, IPCP option codec (IP-Address type 3, RFC 1877 DNS 129/131), FSM negotiation to Opened, pppN address/route programming.

**What the ledger says remains**

One MUST row carries {gap}. Codes 8-11 received on IPCP are not Code-Rejected: the shared dispatcher maps them to LCP echo/protocol-reject handling. Codes 12 and above are Code-Rejected. [`RFC1332-2.1-3`](#rfc1332-2.1-3) is exercised at the backend MTU boundary: `afterLCPOpen` sets the IP MTU to the peer's negotiated Information-field MRU, without subtracting PPP framing. [`RFC1332-2.1-4`](#rfc1332-2.1-4), the fragmentation of larger datagrams, is performed by the Linux IPv4 output path on that MTU; its tests assert the MTU Ze installs, the size above which a datagram is fragmented. [`RFC1332-3.3-2`](#rfc1332-3.3-2) has tagged tests for the IP-Address value in a Configure-Nak.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated (including scoped evidence) | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC1332-2-1`](#rfc1332-2-1), [`RFC1332-2.1-1`](#rfc1332-2.1-1), [`RFC1332-2.1-3`](#rfc1332-2.1-3), [`RFC1332-2.1-4`](#rfc1332-2.1-4), [`RFC1332-3-1`](#rfc1332-3-1), [`RFC1332-3.3-2`](#rfc1332-3.3-2)

**Annotated (including scoped evidence) (2):** [`RFC1332-2-2`](#rfc1332-2-2), [`RFC1332-4.1-1`](#rfc1332-4.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1332-2-1` | Exactly one IPCP packet is encapsulated in the Information field of PPP Data Link Layer frames where the Protocol field indicates type hex 8021 (IP Control Protocol). (§2) | MUST | 2 | **positive:** `unit/verify` [`TestIPCPFrameCarriesExactlyOnePacket`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/frame_test.go#L145). **positive:** `unit/verify` [`TestIPCPOnePacketPerFrameThroughRun`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L222). **negative:** `unit/verify` [`TestIPCPOnePacketPerFrameThroughRun`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L223) |
| `RFC1332-2-2` | Only Codes 1 through 7 (Configure-Request, Configure-Ack, Configure-Nak, Configure-Reject, Terminate-Request, Terminate-Ack and Code-Reject) are used. Other Codes should be treated as unrecognized and should result in Code-Rejects. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's shared codeToEvent (internal/component/l2tp/ppp/session_run.go:688-709) maps PPP control codes 8-11 to LCP echo/protocol-reject events rather than RUC, so ze does not Code-Reject codes 8-11 received on IPCP; only codes 12 and above are Code-Rejected |
| `RFC1332-2.1-1` | Before any IP packets may be communicated, PPP must reach the Network-Layer Protocol phase, and the IP Control Protocol must reach the Opened state. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestIPResponseConfiguresInterface`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L110). **negative:** `unit/verify` [`TestIPCPNoAddressBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L171). **negative:** `unit/verify` [`TestNCPHeldUntilNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L165) |
| `RFC1332-2.1-3` | The maximum length of an IP packet transmitted over a PPP link is the same as the maximum length of the Information field of a PPP data link layer frame. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestIPMTUInstalledFromNegotiatedMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_mtu_test.go#L18). **negative:** `unit/verify` [`TestIPMTUInstalledFromNegotiatedMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_mtu_test.go#L19) |
| `RFC1332-2.1-4` | Larger IP datagrams must be fragmented as necessary. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestIPMTUInstalledFromNegotiatedMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_mtu_test.go#L20). **negative:** `unit/verify` [`TestIPMTUInstalledFromNegotiatedMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_mtu_test.go#L21) |
| `RFC1332-3-1` | IPCP uses the same Configuration Option format defined for LCP [1], with a separate set of Options. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestIPCPOptionsWrittenInLCPFormat`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_clauses_test.go#L23). **positive:** `unit/verify` [`TestIPCPParseOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_ipcp_test.go#L14). **negative:** `unit/verify` [`TestIPCPParseRejects`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_ipcp_test.go#L71) |
| `RFC1332-3.3-2` | The value of the IP-address given must be acceptable as the remote IP-address, or indicate a request that the peer provide the information. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestIPCPNakCarriesTheAssignedRemoteAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L296). **negative:** `unit/verify` [`TestIPCPNakNeverCarriesAnUnacceptableAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L310) |
| `RFC1332-3.1-1` | This option SHOULD NOT be sent in a Configure-Request if a Configure-Request has been received which includes either an IP- Addresses or IP-Address option. (§3.1) | SHOULD NOT | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1332-4.1-1` | The slot identifier must not be compressed if there is no ability for the PPP link level to indicate an error in reception to the decompression module. (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze does not negotiate the IP-Compression-Protocol (Type 2) Van Jacobson option; isKnownIPCPOption (internal/component/l2tp/ppp/ipcp.go:53-55) recognizes only options 3/129/131 and Configure-Rejects a peer Type-2 option; Comp-Slot-Id is never set |
| `RFC1332-2-3` | An implementation should be prepared to wait for Authentication and Link Quality Determination to finish before timing out waiting for a Configure-Ack or other response. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1332-2.1-2` | If a system wishes to avoid fragmentation and reassembly, it should use the TCP Maximum Segment Size option [4], and MTU discovery [5]. (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1332-3.3-1` | If negotiation about the remote IP-address is required, and the peer did not provide the option in its Configure-Request, the option SHOULD be appended to a Configure-Nak. (Section 3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC1332-3.1-2` | This option MAY be sent if a Configure-Reject is received for the IP-Address option, or a Configure-Nak is received with an IP-Addresses option as an appended option. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1332-3.1-3` | Support for this option MAY be removed after the IPCP protocol status advances to Internet Draft Standard. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC1332-2-2`](#rfc1332-2-2) Only Codes 1 through 7 (Configure-Request, Configure-Ack, Configure-Nak, Configure-Reject, Terminate-Request, Terminate-Ack and Code-Reject) are used. Other Codes should be treated as unrecognized and should result in Code-Rejects. (§2) | {gap}, no test | ze's shared codeToEvent (internal/component/l2tp/ppp/session_run.go:688-709) maps PPP control codes 8-11 to LCP echo/protocol-reject events rather than RUC, so ze does not Code-Reject codes 8-11 received on IPCP; only codes 12 and above are Code-Rejected |
| [`RFC1332-4.1-1`](#rfc1332-4.1-1) The slot identifier must not be compressed if there is no ability for the PPP link level to indicate an error in reception to the decompression module. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze does not negotiate the IP-Compression-Protocol (Type 2) Van Jacobson option; isKnownIPCPOption (internal/component/l2tp/ppp/ipcp.go:53-55) recognizes only options 3/129/131 and Configure-Rejects a peer Type-2 option; Comp-Slot-Id is never set |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1332-2-1`](#rfc1332-2-1)

Exactly one IPCP packet is encapsulated in the Information field of PPP Data Link Layer frames where the Protocol field indicates type hex 8021 (IP Control Protocol). (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge ppp4 2026-09-30. Re-read after the frame_test.go negative moved to RFC1661-5-2 (it proved Length validation, a neighbour). Still proven by TestIPCPOnePacketPerFrameThroughRun: ze's IPCP CONFREQ from run() is 0x8021 with Length == Information length (+); a 0x8021 frame carrying a CONFREQ and a trailing Terminate-Request draws the Configure-Ack alone and no Terminate-Ack in 1 s (-, receiver-side injection).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPCPOnePacketPerFrameThroughRun`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L223) | unit/verify | revert, verified |
| positive | [`TestIPCPFrameCarriesExactlyOnePacket`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/frame_test.go#L145) | unit/verify | revert, verified |
| positive | [`TestIPCPOnePacketPerFrameThroughRun`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L222) | unit/verify | revert, verified |

### [`RFC1332-2-2`](#rfc1332-2-2)

Only Codes 1 through 7 (Configure-Request, Configure-Ack, Configure-Nak, Configure-Reject, Terminate-Request, Terminate-Ack and Code-Reject) are used. Other Codes should be treated as unrecognized and should result in Code-Rejects. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1332-2-2, so no unit is bound to it.

### [`RFC1332-2.1-1`](#rfc1332-2.1-1)

Before any IP packets may be communicated, PPP must reach the Network-Layer Protocol phase, and the IP Control Protocol must reach the Opened state. (§2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudge 2026-10-02 (independent judge). Opened clause: positive TestIPResponseConfiguresInterface (completeIPCP -> AddAddressP2P ppp42 10.0.0.1/32 10.0.0.2/32, EventSessionIPAssigned, SetAdminUp after EventSessionUp; revert sendIPAssigned observed red). Negative TestIPCPNoAddressBeforeOpened holds IPCP at Ack-Rcvd and fails on EventSessionIPAssigned, any AddAddressP2P or any SetAdminUp (SetAdminUp runs only after runNCPPhase returns, session_run.go). Recorded on the mutant route: gomu 0.1.0 report over ncp.go (552 mutants, 315 killed; this mutant TIMED_OUT in the full package run) mutant ncp.go_245, handleNCPPacket line 540 `tr.NewState == LCPStateOpened` -> `!=`; judge re-ran it under a Go overlay: red on the unit's own assertion 'EventSessionIPAssigned emitted before IPCP reached Opened'. Network-phase clause: TestNCPHeldUntilNetworkPhase/ipv4 (runAuthPhase revert observed red). Ruling 2 receive side: a supported IPv4 packet before NCP Opened is discarded, TestRFC1661NetworkLayerPacketDiscardedBeforeNCPOpened under RFC1661-3.6-2 (supportsProtocol revert recorded). Claim nit, not a verdict defect: 'so the kernel has no interface to carry IP on' is a rationale the fake backend does not assert; pppN exists but stays down.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPCPNoAddressBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L171) | unit/verify | revert, verified |
| negative | [`TestNCPHeldUntilNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L165) | unit/verify | revert, verified |
| positive | [`TestIPResponseConfiguresInterface`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L110) | unit/verify | revert, verified |

### [`RFC1332-2.1-3`](#rfc1332-2.1-3)

The maximum length of an IP packet transmitted over a PPP link is the same as the maximum length of the Information field of a PPP data link layer frame. (§2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge ppp5 2026-09-30. Unit gained the RFC1332-2.1-4 tags only; body unchanged. TestIPMTUInstalledFromNegotiatedMRU fails unless exactly one MTU is installed on ppp7 and it equals the peer's negotiated MRU 1400 (the largest IP packet sent = the Information-field maximum); 1400 differs from the 1500 default and MaxMRU, so a producer ignoring the negotiated MRU is red (forced-input negative, R1 b). Recorded afterLCPOpen revert.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPMTUInstalledFromNegotiatedMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_mtu_test.go#L19) | unit/verify | revert, verified |
| positive | [`TestIPMTUInstalledFromNegotiatedMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_mtu_test.go#L18) | unit/verify | revert, verified |

### [`RFC1332-2.1-4`](#rfc1332-2.1-4)

Larger IP datagrams must be fragmented as necessary. (§2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge ppp5 2026-09-30. Row is the verbatim second sentence of RFC 1332 Section 2.1. The Linux IPv4 output path fragments on the pppN MTU; Ze's part is the MTU it installs, which this row's tests assert at the boundary Ze owns (rfc-compliance: a requirement met through a lower layer still carries a test over the value Ze installs). TestIPMTUInstalledFromNegotiatedMRU: exactly one MTU on ppp7, equal to the peer MRU 1400 (+); no MTU above 1400, 1500 included (-). Kernel fragmentation itself is not observed (would need a guest kernel); the claim states only the installed MTU. Records: afterLCPOpen revert, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPMTUInstalledFromNegotiatedMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_mtu_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestIPMTUInstalledFromNegotiatedMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_mtu_test.go#L20) | unit/verify | revert, verified |

### [`RFC1332-3-1`](#rfc1332-3-1)

IPCP uses the same Configuration Option format defined for LCP [1], with a separate set of Options. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Send side now tagged: TestIPCPOptionsWrittenInLCPFormat compares WriteIPCPOptions output byte for byte (Types 3/129/131, Length 6, four octets) and walks it with ParseLCPOptions; any drift of Type, Length or Data is red. Receive side: TestIPCPParseOptions decodes a well-formed list, TestIPCPParseRejects refuses malformed TLVs with the exact errors.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPCPParseRejects`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_ipcp_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestIPCPOptionsWrittenInLCPFormat`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_clauses_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestIPCPParseOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_ipcp_test.go#L14) | unit/verify | revert, verified |

### [`RFC1332-3.3-2`](#rfc1332-3.3-2)

The value of the IP-address given must be acceptable as the remote IP-address, or indicate a request that the peer provide the information. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Configure-Nak IP-Address that is not acceptable as the remote address. TestIPCPNakCarriesTheAssignedRemoteAddress asserts the Nak carries the assigned 10.0.0.2 for a request of 10.9.9.9 and for a request with no address, red if another value is given. TestIPCPNakNeverCarriesAnUnacceptableAddress asserts the requested address is not echoed and that with nothing assigned the option is omitted rather than written invalid. The 'or indicate a request' branch is a permitted alternative Ze does not use.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPCPNakNeverCarriesAnUnacceptableAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L310) | unit/verify | revert, verified |
| positive | [`TestIPCPNakCarriesTheAssignedRemoteAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L296) | unit/verify | revert, verified |

### [`RFC1332-4.1-1`](#rfc1332-4.1-1)

The slot identifier must not be compressed if there is no ability for the PPP link level to indicate an error in reception to the decompression module. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1332-4.1-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc1332.txt |
| Source fingerprint | 05b932877d43bddd |
| Record | rfc/extraction/rfc1332.json |
| Mapped sentences | 5 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 2 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 2 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 2 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `4.1` | not stated | 3 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 is a non-normative description of PPP's three components. The sentence describes the LCP establishment sequence that RFC 1331 defines, not an IPCP obligation this document places on an implementation. | In order to establish communications over a point-to-point link, each end of the PPP link must first send LCP packets to configure and test the data link. |
| `1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Same non-normative introduction: it describes how the NCP family defined in RFC 1331 is used after LCP establishment. IPCP's own obligations start at Section 2. | After the link has been established and optional facilities have been negotiated as needed by the LCP, PPP must send NCP packets to choose and configure one or more network-layer protocols. |
| `4:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Van Jacobson TCP/IP header compression is optional in this document and Ze declined it. Section 3.2 states the default: "By default, compression is not enabled." Ze never requests the IP-Compression-Protocol option and Configure-Rejects a peer's Type-2 option (isKnownIPCPOption, internal/component/l2tp/ppp/ipcp.go), so the obligation to request it separately per direction never binds. | Each end of the link must separately request this option if bi-directional compression is desired. |
| `4.1:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Comp-Slot-Id semantics bind only a speaker that negotiated the optional IP-Compression-Protocol option, and Section 3.2 makes it optional: "By default, compression is not enabled." Ze negotiates only IPCP options 3, 129 and 131 and Configure-Rejects Type 2, so it never carries a Comp-Slot-Id field. | 0 The slot identifier must not be compressed. |
| `4.1:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | The C bit and slot identifier rules bind a Van Jacobson compressor, a feature Section 3.2 makes optional: "By default, compression is not enabled." Ze emits no compressed TCP frames (PPP protocol 002d) because it never negotiates the option. | All compressed TCP packets must set the C bit in every change mask, and must include the slot identifier. |

## Superseded

No document obsoletes RFC 1332, so its obligations are stated where they were written.
