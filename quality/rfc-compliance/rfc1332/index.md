# RFC 1332 - The PPP Internet Protocol Control Protocol (IPCP)

Partial. Every requirement this repository extracted from RFC 1332, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 75.0% | 3 of 4 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 6 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 11 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 25.0% | 1 of 4 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 4 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 11 |
| Gated MUST-level | 6 |
| Obligations that bind Ze | 4 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 6 |
| Tagged units | 6 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc1332.md` |
| Requirement shard | `rfc/requirements/rfc1332.md` |
| RFC text | `rfc/full/rfc1332.txt` |

## Enrolment

Enrolled: PPP Internet Protocol Control Protocol (IPCP): six MUST-level requirements. Three are met with positive+negative tags in internal/component/l2tp/ppp: 2-1 (one IPCP packet per 0x8021 frame), 2.1-1 (IPCP reaches Opened before IP is programmed), 3-1 (options follow RFC 1661 TLV format). 2-2 (codes 8-11 on IPCP MUST be Code-Rejected) is {gap}: ze's shared codeToEvent (session_run.go:688-709) maps codes 8-11 to LCP echo/protocol-reject handling, so only codes >= 12 are Code-Rejected; disclosed in the docs/features/rfc-status.md RFC 1332 row. 3.1-1 (no IP-Addresses Type 1) and 4.1-1 (no Van Jacobson Type 2) are {not-applicable}: ze emits only IPCP options 3/129/131 and Configure-Rejects a peer Type-2 option.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

IPv4 address negotiation and pool integration, IPCP option codec (IP-Address type 3, RFC 1877 DNS 129/131), FSM negotiation to Opened, pppN address/route programming.

**What the ledger says remains:**

One MUST gap gated in [`rfc/short/rfc1332.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc1332.md): codes 8-11 received on IPCP are not Code-Rejected (mapped to LCP echo/protocol-reject handling); codes 12 and above are Code-Rejected.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC1332-2-1`](#rfc1332-2-1), [`RFC1332-2.1-1`](#rfc1332-2.1-1), [`RFC1332-3-1`](#rfc1332-3-1)

**Annotated instead of tested (3):** [`RFC1332-2-2`](#rfc1332-2-2), [`RFC1332-3.1-1`](#rfc1332-3.1-1), [`RFC1332-4.1-1`](#rfc1332-4.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1332-2-1` | Exactly one IPCP packet encapsulated per PPP frame with Protocol field 0x8021 (Section 2) | MUST | 2 | **positive:** `unit/verify` [`TestIPCPFrameCarriesExactlyOnePacket`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/frame_test.go#L145). **negative:** `unit/verify` [`TestIPCPFrameRejectsNonSinglePacket`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/frame_test.go#L188) |
| `RFC1332-2-2` | Only Codes 1-7 accepted; other codes treated as unrecognized and result in Code-Reject (Section 2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's shared codeToEvent (internal/component/l2tp/ppp/session_run.go:688-709) maps PPP control codes 8-11 to LCP echo/protocol-reject events rather than RUC, so ze does not Code-Reject codes 8-11 received on IPCP; only codes 12 and above are Code-Rejected |
| `RFC1332-2.1-1` | IPCP reach Opened state before any IP packets may be communicated (Section 2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestIPResponseConfiguresInterface`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L106). **negative:** `unit/verify` [`TestIPCPNoAddressBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L156) |
| `RFC1332-3-1` | Configuration options follow the format defined in RFC 1661 (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestIPCPParseOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ipcp_test.go#L14). **negative:** `unit/verify` [`TestIPCPParseRejects`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ipcp_test.go#L71) |
| `RFC1332-3.1-1` | Implementation sending IP-Addresses option must not send it in Configure-Request if peer sent any Configure-Request containing IP-Addresses or IP-Address option (Section 3.1) | MUST NOT | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no IP-Addresses (Type 1) send code path; internal/component/l2tp/ppp/ipcp.go defines only options 3/129/131 and WriteIPCPOptions emits only those |
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
| [`RFC1332-3.1-1`](#rfc1332-3.1-1) Implementation sending IP-Addresses option must not send it in Configure-Request if peer sent any Configure-Request containing IP-Addresses or IP-Address option (Section 3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no IP-Addresses (Type 1) send code path; internal/component/l2tp/ppp/ipcp.go defines only options 3/129/131 and WriteIPCPOptions emits only those |
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
| negative | [`TestIPCPNoAddressBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L156) | unit/verify | unproven |
| positive | [`TestIPResponseConfiguresInterface`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L106) | unit/verify | unproven |

### [`RFC1332-3-1`](#rfc1332-3-1)

Configuration options follow the format defined in RFC 1661 (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPCPParseRejects`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ipcp_test.go#L71) | unit/verify | unproven |
| positive | [`TestIPCPParseOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ipcp_test.go#L14) | unit/verify | unproven |

### [`RFC1332-3.1-1`](#rfc1332-3.1-1)

Implementation sending IP-Addresses option must not send it in Configure-Request if peer sent any Configure-Request containing IP-Addresses or IP-Address option (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1332-3.1-1, so no unit is bound to it.

### [`RFC1332-4.1-1`](#rfc1332-4.1-1)

Comp-Slot-Id=1 must not be enabled on links without link-level error-indication mechanism (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1332-4.1-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 1332, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 1332, so its obligations are stated where they were written.
