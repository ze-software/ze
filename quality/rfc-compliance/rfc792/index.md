# RFC 792 - Internet Control Message Protocol

No row in the public ledger. Every requirement this repository extracted from RFC 792, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 60.0% | 6 of 10 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 40.0% | 4 of 10 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 10 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 10 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 18 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 10 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 10 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 10 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 10 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 10 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 8 | of 10 gated MUSTs judged | 5 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 10 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 14 |
| Gated MUST-level | 10 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 18 |
| Tagged units | 18 |
| Recorded audit verdicts | 8 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc792.md` |
| Requirement shard | `rfc/requirements/rfc792.md` |
| RFC text | `rfc/full/rfc792.txt` |

## Enrolment

Enrolled: ICMP is part of Ze's whole Linux stack: Go constructs diagnostic Echo Requests, while Linux responds to local Echo Requests, emits ICMP errors, and enforces IPv4 gateway discard requirements. The gateway and unused-field requirements are exercised at Ze's netlink/packet boundary in internal/plugins/vrrp/gateway_icmp_integration_linux_test.go.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 792.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **10** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC792-Echo-3`](#rfc792-echo-3), [`RFC792-Echo-5`](#rfc792-echo-5), [`RFC792-Echo-6`](#rfc792-echo-6), [`RFC792-Unreachable-1`](#rfc792-unreachable-1), [`RFC792-TimeExceeded-1`](#rfc792-timeexceeded-1), [`RFC792-ParamProblem-1`](#rfc792-paramproblem-1)

**Annotated instead of tested (4):** [`RFC792-Echo-1`](#rfc792-echo-1), [`RFC792-Echo-2`](#rfc792-echo-2), [`RFC792-Echo-4`](#rfc792-echo-4), [`RFC792-Format-1`](#rfc792-format-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC792-Echo-1` | Type 8 for echo message; 0 for echo reply message. (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestRFC792EchoRequestType`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L194). **negative:** no negative test. **{single-polarity}:** the diagnostic request callers choose Type 8 and Linux icmp_echo assigns Type 0 to an Echo Reply; this is a transmitted-field requirement |
| `RFC792-Echo-2` | Type 8 for echo message; 0 for echo reply message. Code 0 (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestRFC792EchoRequestCode`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L201). **negative:** no negative test. **{single-polarity}:** ze emits Code 0 and never varies it, so there is no non-zero-code echo it produces to assert against |
| `RFC792-Echo-3` | The checksum is the 16-bit ones's complement of the one's complement sum of the ICMP message starting with the ICMP Type. (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestRFC792ChecksumValid`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L208). **negative:** `unit/verify` [`TestRFC792ChecksumRejectsCorruption`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L218) |
| `RFC792-Echo-4` | If the total length is odd, the received data is padded with one octet of zeros for computing the checksum. (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestRFC792ChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L229). **negative:** no negative test. **{single-polarity}:** the zero pad is an internal step of a correct computation and ze rejects nothing on this basis, so only the positive direction is assertable |
| `RFC792-Echo-5` | The data received in the echo message must be returned in the echo reply message. (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L392). **negative:** `unit/verify` [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L393) |
| `RFC792-Echo-6` | To form an echo reply message, the source and destination addresses are simply reversed, the type code changed to 0, and the checksum recomputed. (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L394). **negative:** `unit/verify` [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L395) |
| `RFC792-Format-1` | Any field labeled "unused" is reserved for later extensions and must be zero when sent (§Format) | MUST | Format | **positive:** `unit/verify` [`TestGatewayICMPDFDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L320). **positive:** `unit/verify` [`TestGatewayICMPHeaderDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L368). **positive:** `unit/verify` [`TestGatewayICMPTTLDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L344). **negative:** no negative test. **{single-polarity}:** Linux builds every ICMP error the gateway emits and ze writes no octet of it, so no input to ze yields an error with a nonzero unused field; the positive tags already contrast the zero unused octets with the nonzero assigned fields (pointer, next-hop MTU, RFC 4884 length) |
| `RFC792-Unreachable-1` | Another case is when a datagram must be fragmented to be forwarded by a gateway yet the Don't Fragment flag is on. In this case the gateway must discard the datagram and may return a destination unreachable message. (§Unreachable) | MUST | Unreachable | **positive:** `unit/verify` [`TestGatewayICMPDFDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L318). **negative:** `unit/verify` [`TestGatewayICMPDFDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L319) |
| `RFC792-TimeExceeded-1` | If the gateway processing a datagram finds the time to live field is zero it must discard the datagram. (§TimeExceeded) | MUST | TimeExceeded | **positive:** `unit/verify` [`TestGatewayICMPTTLDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L342). **negative:** `unit/verify` [`TestGatewayICMPTTLDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L343) |
| `RFC792-ParamProblem-1` | If the gateway or host processing a datagram finds a problem with the header parameters such that it cannot complete processing the datagram it must discard the datagram (§ParamProblem) | MUST | ParamProblem | **positive:** `unit/verify` [`TestGatewayICMPHeaderDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L366). **negative:** `unit/verify` [`TestGatewayICMPHeaderDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L367) |
| `RFC792-Echo-7` | For computing the checksum , the checksum field should be zero. (§Echo) | SHOULD | Echo | **positive:** no positive test. **negative:** no negative test |
| `RFC792-Echo-8` | If code = 0, an identifier to aid in matching echos and replies, may be zero. (§Echo) | MAY | Echo | **positive:** no positive test. **negative:** no negative test |
| `RFC792-Echo-9` | If code = 0, a sequence number to aid in matching echos and replies, may be zero. (§Echo) | MAY | Echo | **positive:** no positive test. **negative:** no negative test |
| `RFC792-Echo-10` | Code 0 may be received from a gateway or a host. (§Echo) | MAY | Echo | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 792 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC792-Echo-1`](#rfc792-echo-1)

Type 8 for echo message; 0 for echo reply message. (§Echo)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Sentence: Type 8 for echo message; 0 for echo reply. TestRFC792EchoRequestType passes the literal 8 to BuildICMPEcho and asserts byte 0 is 8, so it proves the builder copies its argument, not that ze's callers (icmpEcho constant in ping, traceroute) send 8. The reply clause (Type 0) has no Echo-1-tagged assertion; the marker says Linux builds replies, and TestGatewayICMPEchoReply checks it only under Echo-6.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC792EchoRequestType`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L194) | unit/verify | unproven |

### [`RFC792-Echo-2`](#rfc792-echo-2)

Type 8 for echo message; 0 for echo reply message. Code 0 (§Echo)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Span ends 'Code 0': the code is 0 for echo and echo reply. TestRFC792EchoRequestCode asserts byte 1 of a built request is 0, which goes red if the builder wrote a non-zero code. The reply's Code 0 is not asserted by any Echo-2-tagged unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC792EchoRequestCode`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L201) | unit/verify | unproven |

### [`RFC792-Echo-3`](#rfc792-echo-3)

The checksum is the 16-bit ones's complement of the one's complement sum of the ICMP message starting with the ICMP Type. (§Echo)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Sentence: checksum is the one's complement of the one's complement sum from the Type field. Positive TestRFC792ChecksumValid goes red on a wrong ze checksum (checksumOnesFold != 0xffff). The negative TestRFC792ChecksumRejectsCorruption corrupts a built packet and checks that the TEST helper notices, which exercises no ze code, and the row has no single-polarity marker, so the polarity pair is incomplete.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC792ChecksumRejectsCorruption`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L218) | unit/verify | unproven |
| positive | [`TestRFC792ChecksumValid`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L208) | unit/verify | unproven |

### [`RFC792-Echo-4`](#rfc792-echo-4)

If the total length is odd, the received data is padded with one octet of zeros for computing the checksum. (§Echo)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: computing the checksum of an odd-length message without one trailing zero pad octet. Red on it: TestRFC792ChecksumOddLength builds an 11-octet echo and asserts checksumOnesFold == 0xffff, which fails if icmpChecksum dropped the last octet or padded it into the low byte. The receive side is Linux; ze verifies no received ICMP checksum. Single-polarity marker on the row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC792ChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L229) | unit/verify | unproven |

### [`RFC792-Echo-5`](#rfc792-echo-5)

The data received in the echo message must be returned in the echo reply message. (§Echo)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. First verdict 2026-09-27 (QF-2). Forbidden: an echo reply whose data differs from the request's. TestGatewayICMPEchoReply asserts bytes.Equal(answer[4:], icmp[4:]) on an odd-length request, which goes red on changed data. The negative tag rides the same unit and asserts a corrupt-checksum request is not reflected, the checksum rule (RFC792-Echo-6's negative), a neighbouring rule. Weak: one polarity proves the row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L393) | unit/verify | unproven |
| positive | [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L392) | unit/verify | unproven |

### [`RFC792-Echo-6`](#rfc792-echo-6)

To form an echo reply message, the source and destination addresses are simply reversed, the type code changed to 0, and the checksum recomputed. (§Echo)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Sentence: to form a reply, reverse the addresses, change the type to 0, recompute the checksum. The positive in TestGatewayICMPEchoReply asserts all three (address compare, answer[0]==0, gatewayChecksum(answer)==0). The negative (a corrupt-checksum request draws no reply) proves a neighbouring rule, discarding a bad request, not a malformed reply, and the row has no single-polarity marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L395) | unit/verify | unproven |
| positive | [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L394) | unit/verify | unproven |

### [`RFC792-Format-1`](#rfc792-format-1)

Any field labeled "unused" is reserved for later extensions and must be zero when sent (§Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestGatewayICMPDFDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L320) | unit/verify | unproven |
| positive | [`TestGatewayICMPHeaderDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L368) | unit/verify | unproven |
| positive | [`TestGatewayICMPTTLDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L344) | unit/verify | unproven |

### [`RFC792-Unreachable-1`](#rfc792-unreachable-1)

Another case is when a datagram must be fragmented to be forwarded by a gateway yet the Don't Fragment flag is on. In this case the gateway must discard the datagram and may return a destination unreachable message. (§Unreachable)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a gateway forwarding a datagram that needs fragmentation while DF is set. Red on it: gatewayForwarded(t, captured, out, bad, false) in TestGatewayICMPDFDiscard fails if the 900-octet DF datagram appears on the 576-MTU link; the 'may return' clause is checked by gatewayICMP(..., 3, 4). Negative: the DF datagram at MTU and the DF-clear oversize datagram are required to forward (the rule's own boundary).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGatewayICMPDFDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L319) | unit/verify | unproven |
| positive | [`TestGatewayICMPDFDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L318) | unit/verify | unproven |

### [`RFC792-TimeExceeded-1`](#rfc792-timeexceeded-1)

If the gateway processing a datagram finds the time to live field is zero it must discard the datagram. (§TimeExceeded)

Audit verdict: enforced (the tests do what the requirement demands), fresh. First verdict 2026-09-27 (QF-2). Forbidden: forwarding a datagram whose TTL is zero. TestGatewayICMPTTLDiscard asserts gatewayForwarded(..., zero, false) and (..., one, false), red if either leaves the routed exit; the control asserts TTL two is forwarded unchanged, a distinct assertion. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGatewayICMPTTLDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L343) | unit/verify | unproven |
| positive | [`TestGatewayICMPTTLDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L342) | unit/verify | unproven |

### [`RFC792-ParamProblem-1`](#rfc792-paramproblem-1)

If the gateway or host processing a datagram finds a problem with the header parameters such that it cannot complete processing the datagram it must discard the datagram (§ParamProblem)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGatewayICMPHeaderDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L367) | unit/verify | unproven |
| positive | [`TestGatewayICMPHeaderDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/gateway_icmp_integration_linux_test.go#L366) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-27 |
| Register | prose |
| Source | rfc/full/rfc792.txt |
| Source fingerprint | 1cc33872d3b2a6af |
| Record | rfc/extraction/rfc792.json |
| Mapped sentences | 5 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 8 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The Introduction's placement sentence, which says where ICMP lives rather than adding an obligation of its own: 'ICMP, uses the basic support of IP as if it were a higher level protocol, however, ICMP is actually an integral part of IP, and must be implemented by every IP module.' It is a conformance umbrella over the message obligations this document then specifies, and each of those is decided on its own site. The IP module in ze's stack is the Linux IP stack, which implements ICMP. | ICMP, uses the basic support of IP as if it were a higher level protocol, however, ICMP is actually an integral part of IP, and must be implemented by every IP module. |
| `front:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The Introduction's statement that ICMP is not a reliability mechanism, addressed to protocols above IP rather than to an ICMP implementation. The two sentences before it read 'These ICMP messages ... report problems in the communication environment, not to make IP reliable. There are still no guarantees that a datagram will be delivered'. No ICMP behavior answers this sentence; the reliability it names is TCP's. | The higher level protocols that use IP must implement their own reliability procedures if reliable communication is required. |
| `front:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive, not normative: 'Another case is when a datagram must be fragmented to be forwarded by a gateway yet the Don't Fragment flag is on.' The 'must' says what the forwarding case requires, it obliges no role. The obligation of that case is the next sentence, site front:5. | Another case is when a datagram must be fragmented to be forwarded by a gateway yet the Don't Fragment flag is on. |

## Superseded

No document obsoletes RFC 792, so its obligations are stated where they were written.
