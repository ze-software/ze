# RFC 792 - Internet Control Message Protocol

No row in the public ledger. Every requirement this repository extracted from RFC 792, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 70.0% | 7 of 10 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 30.0% | 3 of 10 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 10 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 10 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 10 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 63.6% | 14 of 22 tagged units, 7 escaped and 0 lapsed | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 10 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 10 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 10 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 10 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 10 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 10 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

The 9 shares marked as a part above are the whole of the 10 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

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
| Test tags | 22 |
| Tagged units | 22 |
| Recorded audit verdicts | 8 |
| Discrimination records | 21 |
| Summary | `rfc/short/rfc792.md` |
| Requirement shard | `rfc/requirements/rfc792.md` |
| RFC text | `rfc/full/rfc792.txt` |

## Enrolment

Enrolled: ICMP is part of Ze's whole Linux stack: Go constructs diagnostic Echo Requests, while Linux responds to local Echo Requests, emits ICMP errors, and enforces IPv4 gateway discard requirements. The gateway and unused-field requirements are exercised at Ze's netlink/packet boundary in internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 792.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the gated population |
| Annotated (including scoped evidence) | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **10** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (7):** [`RFC792-Echo-1`](#rfc792-echo-1), [`RFC792-Echo-3`](#rfc792-echo-3), [`RFC792-Echo-5`](#rfc792-echo-5), [`RFC792-Echo-6`](#rfc792-echo-6), [`RFC792-Unreachable-1`](#rfc792-unreachable-1), [`RFC792-TimeExceeded-1`](#rfc792-timeexceeded-1), [`RFC792-ParamProblem-1`](#rfc792-paramproblem-1)

**Annotated (including scoped evidence) (3):** [`RFC792-Echo-2`](#rfc792-echo-2), [`RFC792-Echo-4`](#rfc792-echo-4), [`RFC792-Format-1`](#rfc792-format-1)

## Attainable ceiling

Mandatory ceiling: 10 of 10 (100.0%). Optional ceiling: 4 of 4 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc792 10 of 10 (100.0%). Lowest optional ceiling: rfc792 4 of 4 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC792-Echo-1` | Type 8 for echo message; 0 for echo reply message. (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L402). **positive:** `unit/verify` [`TestRFC792EchoRequestType`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L198). **negative:** `unit/verify` [`TestRFC792EchoRequestType`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L199) |
| `RFC792-Echo-2` | Type 8 for echo message; 0 for echo reply message. Code 0 (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L403). **positive:** `unit/verify` [`TestRFC792EchoRequestCode`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L226). **negative:** no negative test. **{single-polarity}:** ze emits Code 0 and never varies it, so there is no non-zero-code echo it produces to assert against |
| `RFC792-Echo-3` | The checksum is the 16-bit ones's complement of the one's complement sum of the ICMP message starting with the ICMP Type. (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L404). **positive:** `unit/verify` [`TestRFC792ChecksumValid`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L233). **negative:** `unit/verify` [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L405) |
| `RFC792-Echo-4` | If the total length is odd, the received data is padded with one octet of zeros for computing the checksum. (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestRFC792ChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L252). **negative:** no negative test. **{single-polarity}:** the zero pad is an internal step of a correct computation and ze rejects nothing on this basis, so only the positive direction is assertable |
| `RFC792-Echo-5` | The data received in the echo message must be returned in the echo reply message. (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L406). **negative:** `unit/verify` [`TestGatewayICMPEchoReplyToTheQueriedAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L483) |
| `RFC792-Echo-6` | To form an echo reply message, the source and destination addresses are simply reversed, the type code changed to 0, and the checksum recomputed. (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L407). **negative:** `unit/verify` [`TestGatewayICMPEchoReplyToTheQueriedAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L484) |
| `RFC792-Format-1` | Any field labeled "unused" is reserved for later extensions and must be zero when sent (§Format) | MUST | Format | **positive:** `unit/verify` [`TestGatewayICMPDFDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L321). **positive:** `unit/verify` [`TestGatewayICMPHeaderDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L375). **positive:** `unit/verify` [`TestGatewayICMPTTLDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L348). **negative:** no negative test. **{single-polarity}:** Linux builds every ICMP error the gateway emits and ze writes no octet of it, so no input to ze yields an error with a nonzero unused field; the positive tags already contrast the zero unused octets with the nonzero assigned fields (pointer, next-hop MTU, RFC 4884 length) |
| `RFC792-Unreachable-1` | Another case is when a datagram must be fragmented to be forwarded by a gateway yet the Don't Fragment flag is on. In this case the gateway must discard the datagram and may return a destination unreachable message. (§Unreachable) | MUST | Unreachable | **positive:** `unit/verify` [`TestGatewayICMPDFDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L319). **negative:** `unit/verify` [`TestGatewayICMPDFDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L320) |
| `RFC792-TimeExceeded-1` | If the gateway processing a datagram finds the time to live field is zero it must discard the datagram. (§TimeExceeded) | MUST | TimeExceeded | **positive:** `unit/verify` [`TestGatewayICMPTTLDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L346). **negative:** `unit/verify` [`TestGatewayICMPTTLDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L347) |
| `RFC792-ParamProblem-1` | If the gateway or host processing a datagram finds a problem with the header parameters such that it cannot complete processing the datagram it must discard the datagram (§ParamProblem) | MUST | ParamProblem | **positive:** `unit/verify` [`TestGatewayICMPHeaderDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L373). **negative:** `unit/verify` [`TestGatewayICMPHeaderDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L374) |
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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor) after the user-namespace conversion (d8427e4380): the TestGatewayICMPEchoReply unit gained only `if !userns.Enter(t) { return }` as its first statement and its build tag moved from integration&&linux to linux. userns.Enter (internal/test/userns/userns_linux.go) re-runs the one named test in a fresh user+net+mount namespace and returns false to the parent only when the child reported PASS or SKIP; a failing child, a non-zero exit, or a child that ran nothing calls t.Fatalf, so every assertion below the guard still fails the test when the behaviour breaks. Body otherwise byte-identical. RFC 792 Echo: "0 for echo reply message." Fresh judgement: the stack reply to Ze's installed gateway address must carry Type 0 (answer[0] != 0 fails); TestRFC792EchoRequestType (unchanged) pins Type 8/0 for Ze's own probes and its negative refuses a Type 8 as reply. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC792EchoRequestType`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L199) | unit/verify | revert, verified |
| positive | [`TestRFC792EchoRequestType`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L198) | unit/verify | revert, verified |
| positive | [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L402) | unit/verify | revert, verified |

### [`RFC792-Echo-2`](#rfc792-echo-2)

Type 8 for echo message; 0 for echo reply message. Code 0 (§Echo)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor) after the user-namespace conversion (d8427e4380): the TestGatewayICMPEchoReply unit gained only `if !userns.Enter(t) { return }` as its first statement and its build tag moved from integration&&linux to linux. userns.Enter (internal/test/userns/userns_linux.go) re-runs the one named test in a fresh user+net+mount namespace and returns false to the parent only when the child reported PASS or SKIP; a failing child, a non-zero exit, or a child that ran nothing calls t.Fatalf, so every assertion below the guard still fails the test when the behaviour breaks. Body otherwise byte-identical. RFC 792 Echo: Code "0". Fresh judgement: the reply's Code is asserted 0 and TestRFC792EchoRequestCode pins the request byte; single-polarity per the row. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC792EchoRequestCode`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L226) | unit/verify | revert, verified |
| positive | [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L403) | unit/verify | revert, verified |

### [`RFC792-Echo-3`](#rfc792-echo-3)

The checksum is the 16-bit ones's complement of the one's complement sum of the ICMP message starting with the ICMP Type. (§Echo)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor) after the user-namespace conversion (d8427e4380): the TestGatewayICMPEchoReply unit gained only `if !userns.Enter(t) { return }` as its first statement and its build tag moved from integration&&linux to linux. userns.Enter (internal/test/userns/userns_linux.go) re-runs the one named test in a fresh user+net+mount namespace and returns false to the parent only when the child reported PASS or SKIP; a failing child, a non-zero exit, or a child that ran nothing calls t.Fatalf, so every assertion below the guard still fails the test when the behaviour breaks. Body otherwise byte-identical. RFC 792 Echo: "The checksum is the 16-bit ones's complement of the one's complement sum of the ICMP message starting with the ICMP Type." Fresh judgement: the reply's checksum must fold to zero, and a request with a deliberately wrong checksum (sequence 2) must never be reflected, beside the valid control; the rejecting branch is Linux's on the address Ze installs (whole-stack rule). Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L405) | unit/verify | revert, verified |
| positive | [`TestRFC792ChecksumValid`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L233) | unit/verify | revert, verified |
| positive | [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L404) | unit/verify | revert, verified |

### [`RFC792-Echo-4`](#rfc792-echo-4)

If the total length is odd, the received data is padded with one octet of zeros for computing the checksum. (§Echo)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: computing the checksum of an odd-length message without one trailing zero pad octet. Red on it: TestRFC792ChecksumOddLength builds an 11-octet echo and asserts checksumOnesFold == 0xffff, which fails if icmpChecksum dropped the last octet or padded it into the low byte. The receive side is Linux; ze verifies no received ICMP checksum. Single-polarity marker on the row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC792ChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L252) | unit/verify | unproven |

### [`RFC792-Echo-5`](#rfc792-echo-5)

The data received in the echo message must be returned in the echo reply message. (§Echo)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor) after the user-namespace conversion (d8427e4380): the TestGatewayICMPEchoReply unit gained only `if !userns.Enter(t) { return }` as its first statement and its build tag moved from integration&&linux to linux. userns.Enter (internal/test/userns/userns_linux.go) re-runs the one named test in a fresh user+net+mount namespace and returns false to the parent only when the child reported PASS or SKIP; a failing child, a non-zero exit, or a child that ran nothing calls t.Fatalf, so every assertion below the guard still fails the test when the behaviour breaks. Body otherwise byte-identical. RFC 792 Echo: "The data received in the echo message must be returned in the echo reply message." Fresh judgement: odd-length data must come back byte-equal, and TestGatewayICMPEchoReplyToTheQueriedAddress (same guard-only change) requires 1, 333 and 1400 distinct-octet payloads each returned exactly to its own sequence and every request answered. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGatewayICMPEchoReplyToTheQueriedAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L483) | unit/verify | revert, verified |
| positive | [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L406) | unit/verify | revert, verified |

### [`RFC792-Echo-6`](#rfc792-echo-6)

To form an echo reply message, the source and destination addresses are simply reversed, the type code changed to 0, and the checksum recomputed. (§Echo)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor) after the user-namespace conversion (d8427e4380): the TestGatewayICMPEchoReply unit gained only `if !userns.Enter(t) { return }` as its first statement and its build tag moved from integration&&linux to linux. userns.Enter (internal/test/userns/userns_linux.go) re-runs the one named test in a fresh user+net+mount namespace and returns false to the parent only when the child reported PASS or SKIP; a failing child, a non-zero exit, or a child that ran nothing calls t.Fatalf, so every assertion below the guard still fails the test when the behaviour breaks. Body otherwise byte-identical. RFC 792 Echo: "To form an echo reply message, the source and destination addresses are simply reversed, the type code changed to 0, and the checksum recomputed." Fresh judgement: reversed addresses, Type 0 and a verifying checksum on the reply, and the secondary-address request must be answered from .253 to the requester, never from the primary .254. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGatewayICMPEchoReplyToTheQueriedAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L484) | unit/verify | revert, verified |
| positive | [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L407) | unit/verify | revert, verified |

### [`RFC792-Format-1`](#rfc792-format-1)

Any field labeled "unused" is reserved for later extensions and must be zero when sent (§Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestGatewayICMPDFDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L321) | unit/verify | no-break escape (kernel-producer), which is not a proof, verified |
| positive | [`TestGatewayICMPHeaderDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L375) | unit/verify | no-break escape (kernel-producer), which is not a proof, verified |
| positive | [`TestGatewayICMPTTLDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L348) | unit/verify | no-break escape (kernel-producer), which is not a proof, verified |

### [`RFC792-Unreachable-1`](#rfc792-unreachable-1)

Another case is when a datagram must be fragmented to be forwarded by a gateway yet the Don't Fragment flag is on. In this case the gateway must discard the datagram and may return a destination unreachable message. (§Unreachable)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor) after the user-namespace conversion (d8427e4380): the TestGatewayICMPDFDiscard unit gained only `if !userns.Enter(t) { return }` as its first statement and its build tag moved from integration&&linux to linux. userns.Enter (internal/test/userns/userns_linux.go) re-runs the one named test in a fresh user+net+mount namespace and returns false to the parent only when the child reported PASS or SKIP; a failing child, a non-zero exit, or a child that ran nothing calls t.Fatalf, so every assertion below the guard still fails the test when the behaviour breaks. Body otherwise byte-identical. RFC 792 Destination Unreachable: "In this case the gateway must discard the datagram and may return a destination unreachable message." Fresh judgement: a 900-octet DF datagram must not appear on the 576-MTU exit, now configured through Ze's netlinkBackend.SetMTU in routedExit (b8c4909334, helper only), with Fragmentation Needed returned; a DF datagram exactly at MTU and a DF-clear oversize datagram must forward, bounding the rule. newGatewayWire no longer skips on EPERM, which can only make the unit fail more often. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGatewayICMPDFDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L320) | unit/verify | mutant, verified |
| positive | [`TestGatewayICMPDFDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L319) | unit/verify | mutant, verified |

### [`RFC792-TimeExceeded-1`](#rfc792-timeexceeded-1)

If the gateway processing a datagram finds the time to live field is zero it must discard the datagram. (§TimeExceeded)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor) after the user-namespace conversion (d8427e4380): the TestGatewayICMPTTLDiscard unit gained only `if !userns.Enter(t) { return }` as its first statement and its build tag moved from integration&&linux to linux. userns.Enter (internal/test/userns/userns_linux.go) re-runs the one named test in a fresh user+net+mount namespace and returns false to the parent only when the child reported PASS or SKIP; a failing child, a non-zero exit, or a child that ran nothing calls t.Fatalf, so every assertion below the guard still fails the test when the behaviour breaks. Body otherwise byte-identical. RFC 792 Time Exceeded: "If the gateway processing a datagram finds the time to live field is zero it must discard the datagram." Fresh judgement: TTL zero and TTL one must be absent on the routed exit and answered with Time Exceeded, while TTL two must forward on the same route, so a forwarding gateway fails. The discrimination record is a kernel-producer escape (ip_forward decides it from the packet alone): the test enforces the stack behaviour, but the row is published as escaped, not as proven by a break of Ze's code. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGatewayICMPTTLDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L347) | unit/verify | no-break escape (kernel-producer), which is not a proof, verified |
| positive | [`TestGatewayICMPTTLDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L346) | unit/verify | no-break escape (kernel-producer), which is not a proof, verified |

### [`RFC792-ParamProblem-1`](#rfc792-paramproblem-1)

If the gateway or host processing a datagram finds a problem with the header parameters such that it cannot complete processing the datagram it must discard the datagram (§ParamProblem)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGatewayICMPHeaderDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L374) | unit/verify | no-break escape (kernel-producer), which is not a proof, verified |
| positive | [`TestGatewayICMPHeaderDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L373) | unit/verify | no-break escape (kernel-producer), which is not a proof, verified |

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
