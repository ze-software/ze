# RFC 792 - Internet Control Message Protocol

No row in the public ledger. Every requirement this repository extracted from RFC 792, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 25.0% | 1 of 4 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 75.0% | 3 of 4 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 4 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 5 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 10 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 4 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 10 |
| Gated MUST-level | 6 |
| Obligations that bind Ze | 4 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 5 |
| Tagged units | 5 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc792.md` |
| Requirement shard | `rfc/requirements/rfc792.md` |
| RFC text | `rfc/full/rfc792.txt` |

## Enrolment

Enrolled: ICMP Echo/Echo Reply, the only part of ICMP ze exercises (ping, show ping, ping-monitor). Gated obligations are the echo request's Type/Code/Checksum, which internal/core/probe BuildICMPEcho emits and the probe tests pin; the responder-only obligations (data returned unchanged, reply formation) are {not-applicable} because ze is the echo requester, never the echoer.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 792.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated instead of tested | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC792-Echo-3`](#rfc792-echo-3)

**Annotated instead of tested (5):** [`RFC792-Echo-1`](#rfc792-echo-1), [`RFC792-Echo-2`](#rfc792-echo-2), [`RFC792-Echo-4`](#rfc792-echo-4), [`RFC792-Echo-5`](#rfc792-echo-5), [`RFC792-Echo-6`](#rfc792-echo-6)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC792-Echo-1` | An ICMP echo request carries Type 8 and an echo reply carries Type 0 (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestRFC792EchoRequestType`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L71). **negative:** no negative test. **{single-polarity}:** ze emits only echo requests and never an ICMP message of another type, so there is no ze-produced echo of a different type to assert against |
| `RFC792-Echo-2` | An echo request carries Code 0 (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestRFC792EchoRequestCode`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L78). **negative:** no negative test. **{single-polarity}:** ze emits Code 0 and never varies it, so there is no non-zero-code echo it produces to assert against |
| `RFC792-Echo-3` | The Checksum is the 16-bit one's complement of the one's-complement sum of the ICMP message starting with the ICMP Type field (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestRFC792ChecksumValid`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L85). **negative:** `unit/verify` [`TestRFC792ChecksumRejectsCorruption`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L95) |
| `RFC792-Echo-4` | If the total length is odd, the data is padded with one octet of zeros for computing the checksum (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestRFC792ChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L106). **negative:** no negative test. **{single-polarity}:** the zero pad is an internal step of a correct computation and ze rejects nothing on this basis, so only the positive direction is assertable |
| `RFC792-Echo-5` | The data received in the echo request is returned unchanged in the echo reply (§Echo) | MUST | Echo | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze issues echo requests and consumes replies; it is not an ICMP echo responder, so returning the request data unchanged is the remote host's obligation, which ze relies on but does not implement |
| `RFC792-Echo-6` | To form an echo reply the source and destination addresses are reversed, the Type is changed to 0, and the checksum is recomputed (§Echo) | MUST | Echo | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** reply formation is the responder's role; ze does not answer inbound echo requests |
| `RFC792-Echo-7` | The checksum field is set to zero while the checksum is being computed (§Echo) | SHOULD | Echo | **positive:** no positive test. **negative:** no negative test |
| `RFC792-Echo-8` | The Identifier may be zero, and the echo sender may use it to match replies to requests (§Echo) | MAY | Echo | **positive:** no positive test. **negative:** no negative test |
| `RFC792-Echo-9` | The Sequence Number may be zero, and the echo sender may use it to match replies to requests (§Echo) | MAY | Echo | **positive:** no positive test. **negative:** no negative test |
| `RFC792-Echo-10` | A Code 0 (echo reply) may be received from a gateway or a host (§Echo) | MAY | Echo | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC792-Echo-5`](#rfc792-echo-5) The data received in the echo request is returned unchanged in the echo reply (§Echo) | no test | no test carries this requirement id; annotated {not-applicable}: ze issues echo requests and consumes replies; it is not an ICMP echo responder, so returning the request data unchanged is the remote host's obligation, which ze relies on but does not implement |
| [`RFC792-Echo-6`](#rfc792-echo-6) To form an echo reply the source and destination addresses are reversed, the Type is changed to 0, and the checksum is recomputed (§Echo) | no test | no test carries this requirement id; annotated {not-applicable}: reply formation is the responder's role; ze does not answer inbound echo requests |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC792-Echo-1`](#rfc792-echo-1)

An ICMP echo request carries Type 8 and an echo reply carries Type 0 (§Echo)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC792EchoRequestType`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L71) | unit/verify | unproven |

### [`RFC792-Echo-2`](#rfc792-echo-2)

An echo request carries Code 0 (§Echo)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC792EchoRequestCode`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L78) | unit/verify | unproven |

### [`RFC792-Echo-3`](#rfc792-echo-3)

The Checksum is the 16-bit one's complement of the one's-complement sum of the ICMP message starting with the ICMP Type field (§Echo)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC792ChecksumRejectsCorruption`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L95) | unit/verify | unproven |
| positive | [`TestRFC792ChecksumValid`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L85) | unit/verify | unproven |

### [`RFC792-Echo-4`](#rfc792-echo-4)

If the total length is odd, the data is padded with one octet of zeros for computing the checksum (§Echo)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC792ChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L106) | unit/verify | unproven |

### [`RFC792-Echo-5`](#rfc792-echo-5)

The data received in the echo request is returned unchanged in the echo reply (§Echo)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC792-Echo-5, so no unit is bound to it.

### [`RFC792-Echo-6`](#rfc792-echo-6)

To form an echo reply the source and destination addresses are reversed, the Type is changed to 0, and the checksum is recomputed (§Echo)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC792-Echo-6, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 792, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 792, so its obligations are stated where they were written.
