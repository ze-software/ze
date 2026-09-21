# RFC 1332 - The PPP Internet Protocol Control Protocol (IPCP)

Partial. Every requirement this repository extracted from RFC 1332, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 42.9% | 3 of 7 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 7 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 7 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 6 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 7 | of 13 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 7 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 14.3% | 1 of 7 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 7 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 7 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 42.9% | 3 of 7 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 7 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
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
| Requirements | 13 |
| Gated MUST-level | 7 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 1 |
| Gated with no test | 2 |
| Nightly-only evidence | 0 |
| Test tags | 6 |
| Tagged units | 6 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc1332.md` |
| Requirement shard | `rfc/requirements/rfc1332.md` |
| RFC text | `rfc/full/rfc1332.txt` |

## Enrolment

Enrolled: PPP Internet Protocol Control Protocol (IPCP): seven MUST-level requirements after the 2026-09-21 extraction walk. Three are met with positive+negative tags in internal/component/l2tp/ppp: 2-1 (one IPCP packet per 0x8021 frame), 2.1-1 (IPCP reaches Opened before IP is programmed), 3-1 (options follow RFC 1661 TLV format). 2-2 (codes 8-11 on IPCP MUST be Code-Rejected) is {gap}: ze's shared codeToEvent (session_run.go:688-709) maps codes 8-11 to LCP echo/protocol-reject handling, so only codes >= 12 are Code-Rejected; disclosed in the docs/features/rfc-status.md RFC 1332 row. 4.1-1 (no Van Jacobson Type 2) is {not-applicable}: ze emits only IPCP options 3/129/131 and Configure-Rejects a peer Type-2 option. The walk added two MUST rows with no test: 2.1-3 (fragment an IP datagram larger than the maximum length of the PPP Information field) and 3.3-2 (an IP-Address value appended to a Configure-Nak must be acceptable as the remote address or request that the peer supply it). 3.1-1 left the MUST-level population in the same walk: §3.1 states the IP-Addresses rule as a SHOULD NOT, so the row is advisory now and its {not-applicable} note was removed with the gate it excused.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

IPv4 address negotiation and pool integration, IPCP option codec (IP-Address type 3, RFC 1877 DNS 129/131), FSM negotiation to Opened, pppN address/route programming.

**What the ledger says remains**

One MUST gap gated in [`rfc/short/rfc1332.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc1332.md): codes 8-11 received on IPCP are not Code-Rejected (mapped to LCP echo/protocol-reject handling); codes 12 and above are Code-Rejected. Two MUST rows the 2026-09-21 extraction walk added carry no test: [`RFC1332-2.1-3`](#rfc1332-2.1-3) (fragment an oversized IP datagram, §2.1) and [`RFC1332-3.3-2`](#rfc1332-3.3-2) (the IP-Address value in a Configure-Nak, §3.3).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 2 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **7** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC1332-2-1`](#rfc1332-2-1), [`RFC1332-2.1-1`](#rfc1332-2.1-1), [`RFC1332-3-1`](#rfc1332-3-1)

**Annotated instead of tested (2):** [`RFC1332-2-2`](#rfc1332-2-2), [`RFC1332-4.1-1`](#rfc1332-4.1-1)

**No test and no annotation (2):** [`RFC1332-2.1-3`](#rfc1332-2.1-3), [`RFC1332-3.3-2`](#rfc1332-3.3-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1332-2-1` | Exactly one IPCP packet encapsulated per PPP frame with Protocol field 0x8021 (Section 2) | MUST | 2 | **positive:** `unit/verify` [`TestIPCPFrameCarriesExactlyOnePacket`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/frame_test.go#L145). **negative:** `unit/verify` [`TestIPCPFrameRejectsNonSinglePacket`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/frame_test.go#L188) |
| `RFC1332-2-2` | Only Codes 1-7 accepted; other codes treated as unrecognized and result in Code-Reject (Section 2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's shared codeToEvent (internal/component/l2tp/ppp/session_run.go:688-709) maps PPP control codes 8-11 to LCP echo/protocol-reject events rather than RUC, so ze does not Code-Reject codes 8-11 received on IPCP; only codes 12 and above are Code-Rejected |
| `RFC1332-2.1-1` | IPCP reach Opened state before any IP packets may be communicated (Section 2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestIPResponseConfiguresInterface`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L110). **negative:** `unit/verify` [`TestIPCPNoAddressBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L171) |
| `RFC1332-2.1-3` | An IP datagram larger than the maximum length of the PPP Information field must be fragmented as necessary (Section 2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1332-3-1` | Configuration options follow the format defined in RFC 1661 (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestIPCPParseOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ipcp_test.go#L14). **negative:** `unit/verify` [`TestIPCPParseRejects`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ipcp_test.go#L71) |
| `RFC1332-3.3-2` | "The value of the IP-address given must be acceptable as the remote IP-address, or indicate a request that the peer provide the information" when the IP-Address option is appended to a Configure-Nak (Section 3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC1332-3.1-1` | The IP-Addresses option SHOULD NOT be sent in a Configure-Request if a Configure-Request has been received which includes either an IP-Addresses or IP-Address option (Section 3.1) | SHOULD NOT | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1332-4.1-1` | Comp-Slot-Id=1 must not be enabled on links without link-level error-indication mechanism (Section 4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze does not negotiate the IP-Compression-Protocol (Type 2) Van Jacobson option; isKnownIPCPOption (internal/component/l2tp/ppp/ipcp.go:53-55) recognizes only options 3/129/131 and Configure-Rejects a peer Type-2 option; Comp-Slot-Id is never set |
| `RFC1332-2-3` | Be prepared to wait for Authentication and Link Quality Determination to finish before timing out waiting for Configure-Ack (Section 2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1332-2.1-2` | IP datagrams should use mechanisms (TCP MSS, PMTUD) to avoid fragmentation (Section 2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1332-3.3-1` | If negotiation about remote IP-address is required and peer did not provide the option, append IP-Address option to Configure-Nak (Section 3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC1332-3.1-2` | Send IP-Addresses option if previously received Configure-Reject for IP-Address or Configure-Nak with IP-Addresses (Section 3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1332-3.1-3` | Support for IP-Addresses option may be removed (Section 3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC1332-2-2`](#rfc1332-2-2) Only Codes 1-7 accepted; other codes treated as unrecognized and result in Code-Reject (Section 2) | {gap}, no test | ze's shared codeToEvent (internal/component/l2tp/ppp/session_run.go:688-709) maps PPP control codes 8-11 to LCP echo/protocol-reject events rather than RUC, so ze does not Code-Reject codes 8-11 received on IPCP; only codes 12 and above are Code-Rejected |
| [`RFC1332-2.1-3`](#rfc1332-2.1-3) An IP datagram larger than the maximum length of the PPP Information field must be fragmented as necessary (Section 2.1) | no test | no test carries this requirement id |
| [`RFC1332-3.3-2`](#rfc1332-3.3-2) "The value of the IP-address given must be acceptable as the remote IP-address, or indicate a request that the peer provide the information" when the IP-Address option is appended to a Configure-Nak (Section 3.3) | no test | no test carries this requirement id |
| [`RFC1332-4.1-1`](#rfc1332-4.1-1) Comp-Slot-Id=1 must not be enabled on links without link-level error-indication mechanism (Section 4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze does not negotiate the IP-Compression-Protocol (Type 2) Van Jacobson option; isKnownIPCPOption (internal/component/l2tp/ppp/ipcp.go:53-55) recognizes only options 3/129/131 and Configure-Rejects a peer Type-2 option; Comp-Slot-Id is never set |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1332-2-1`](#rfc1332-2-1)

Exactly one IPCP packet encapsulated per PPP frame with Protocol field 0x8021 (Section 2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPCPFrameRejectsNonSinglePacket`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/frame_test.go#L188) | unit/verify | unproven |
| positive | [`TestIPCPFrameCarriesExactlyOnePacket`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/frame_test.go#L145) | unit/verify | unproven |

### [`RFC1332-2-2`](#rfc1332-2-2)

Only Codes 1-7 accepted; other codes treated as unrecognized and result in Code-Reject (Section 2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1332-2-2, so no unit is bound to it.

### [`RFC1332-2.1-1`](#rfc1332-2.1-1)

IPCP reach Opened state before any IP packets may be communicated (Section 2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPCPNoAddressBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L171) | unit/verify | unproven |
| positive | [`TestIPResponseConfiguresInterface`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L110) | unit/verify | unproven |

### [`RFC1332-2.1-3`](#rfc1332-2.1-3)

An IP datagram larger than the maximum length of the PPP Information field must be fragmented as necessary (Section 2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1332-2.1-3, so no unit is bound to it.

### [`RFC1332-3-1`](#rfc1332-3-1)

Configuration options follow the format defined in RFC 1661 (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPCPParseRejects`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ipcp_test.go#L71) | unit/verify | unproven |
| positive | [`TestIPCPParseOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ipcp_test.go#L14) | unit/verify | unproven |

### [`RFC1332-3.3-2`](#rfc1332-3.3-2)

"The value of the IP-address given must be acceptable as the remote IP-address, or indicate a request that the peer provide the information" when the IP-Address option is appended to a Configure-Nak (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1332-3.3-2, so no unit is bound to it.

### [`RFC1332-4.1-1`](#rfc1332-4.1-1)

Comp-Slot-Id=1 must not be enabled on links without link-level error-indication mechanism (Section 4.1)

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
