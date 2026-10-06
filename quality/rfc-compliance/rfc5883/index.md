# RFC 5883 - Bidirectional Forwarding Detection (BFD) for Multihop Paths

Partial. Every requirement this repository extracted from RFC 5883, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 6 of 6 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 6 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 6 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 6 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 42.9% | 6 of 14 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 7 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 6 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 6 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 6 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 6 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 7 |
| Gated MUST-level | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 14 |
| Tagged units | 14 |
| Recorded audit verdicts | 5 |
| Discrimination records | 6 |
| Summary | `rfc/short/rfc5883.md` |
| Requirement shard | `rfc/requirements/rfc5883.md` |
| RFC text | `rfc/full/rfc5883.txt` |

## Enrolment

Enrolled: BFD for Multihop Paths: nine MUST-level requirements after the 2026-09-21 extraction walk, which added RFC5883-4.1-1 (two sessions between the same pair of systems have at least one endpoint address distinct from one another, §4.1) with no test and no annotation. Six of the other eight are met with new positive+negative tests in internal/component/bfd asserting producer decisions: 5-1 (multihop uses UDP destination port 4784), 5-2 (single-hop 3784 and multihop 4784 use separate ports), 4.3-1 (a default session takes the Active role and arms its transmit timer), 4.3-2 (a Passive session stays silent until it receives a packet), and 3-1 / x-1 (BFD Echo is rejected on a multihop path). 7-1 and 7-2 (congestion detection and congestion-triggered transmit-rate reduction) were retired on 2026-09-27: the obligation is RFC 5880 Section 7's, carried with its gap as RFC5880-7-1 and RFC5880-7-2, and ze has no BFD congestion-control code path (only slow-start and jitter). Disclosed in the docs/features/rfc-status.md RFC 5883 row.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

- Multi-hop UDP 4784 sessions, single-hop/multihop port separation (3784/4784), Active/Passive roles, echo-on-multihop rejection, and Ze's local min-TTL policy
- tests bound per requirement in [`rfc/requirements/rfc5883.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc5883.md).


**What the ledger says remains**

No BFD congestion control or congestion-triggered transmit-rate reduction (RFC 5883 / RFC 5880 Section 7); IPv6 dual-bind and wider deployment proof are tracked with BFD. [`RFC5883-4.1-1`](#rfc5883-4.1-1), the distinct-endpoint rule for two sessions between the same pair of systems, was added by the 2026-09-21 extraction walk and carries no test.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC5883-3-1`](#rfc5883-3-1), [`RFC5883-4.1-1`](#rfc5883-4.1-1), [`RFC5883-4.3-1`](#rfc5883-4.3-1), [`RFC5883-4.3-2`](#rfc5883-4.3-2), [`RFC5883-5-1`](#rfc5883-5-1), [`RFC5883-x-1`](#rfc5883-x-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5883-3-1` | Finally, the Echo function MUST NOT be used over multiple hops. (§3) | MUST NOT | 3 | **positive:** `unit/verify` [`TestRFC5883ClientSingleHopEchoProfileAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/enabled_test.go#L243). **positive:** `unit/verify` [`TestRFC5883SingleHopEchoAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L108). **negative:** `unit/verify` [`TestRFC5883ClientMultiHopEchoProfileRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/enabled_test.go#L228). **negative:** `unit/verify` [`TestRFC5883MultiHopEchoRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L94) |
| `RFC5883-4.1-1` | Multiple sessions between the same pair of systems must have at least one endpoint address distinct from one another (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC5883MultihopSessionsNeedDistinctEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5883_multihop_pair_test.go#L35). **negative:** `unit/verify` [`TestRFC5883MultihopSessionsNeedDistinctEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5883_multihop_pair_test.go#L36) |
| `RFC5883-4.3-1` | In this approach, the Unidirectional Sender MUST operate in the Active role (as defined in the base BFD specification) (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC5883DefaultSessionActiveArmsTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L39). **negative:** `unit/verify` [`TestRFC5883PassiveSessionDoesNotArmTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L53) |
| `RFC5883-4.3-2` | the Unidirectional Receiver MUST operate in the Passive role. (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC5883PassiveSessionSilentUntilRx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L68). **negative:** `unit/verify` [`TestRFC5883PassiveSessionTransmitsAfterRx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L79) |
| `RFC5883-5-1` | The encapsulation of BFD Control packets for multihop application in IPv4 and IPv6 is identical to that defined in [BFD-1HOP], except that the UDP destination port MUST have a value of 4784. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC5883MultiHopControlSentToPort4784OnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_wire_port_linux_test.go#L213). **negative:** `unit/verify` [`TestRFC5883SingleHopControlNeverAddressesPort4784`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_wire_port_linux_test.go#L230) |
| `RFC5883-x-1` | Finally, the Echo function MUST NOT be used over multiple hops. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC5883SingleHopEchoAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L112). **negative:** `unit/verify` [`TestRFC5883MultiHopEchoRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L98) |
| `RFC5883-6-1` | As such, implementations of BFD SHOULD utilize cryptographic authentication over multihop paths to help mitigate denial-of-service attacks. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 5883 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5883-3-1`](#rfc5883-3-1)

Finally, the Echo function MUST NOT be used over multiple hops. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RA-BFD strict re-read 2026-09-27 (stale after DF-BFD-4). The row quotes RFC 5883 Section 3 verbatim: "Finally, the Echo function MUST NOT be used over multiple hops." (rfc5883.txt line 132). Forbidden: a multi-hop session that runs Echo. A session's DesiredMinEchoTxInterval, and the Required Min Echo RX it advertises (session.go Init), come only from a profile through profileConfig.applyTo, and every entry passes profileConfig.permitsMode. Red: TestRFC5883MultiHopEchoRejected Fatalf when pluginConfig.validate accepts an echo profile on a multi-hop pinned session; TestRFC5883ClientMultiHopEchoProfileRefused Fatal when EnsureSession returns a handle for a multi-hop client request naming an echo profile, or lacks the `enables echo` refusal. Positive, same profile on single hop: TestRFC5883SingleHopEchoAccepted (validate) and TestRFC5883ClientSingleHopEchoProfileAccepted (EnsureSession), so a blanket echo refusal fails. The engine holds no second guard (engine.go NewLoopWithEcho comment).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5883ClientMultiHopEchoProfileRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/enabled_test.go#L228) | unit/verify | revert, verified |
| negative | [`TestRFC5883MultiHopEchoRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L94) | unit/verify | unproven |
| positive | [`TestRFC5883ClientSingleHopEchoProfileAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/enabled_test.go#L243) | unit/verify | revert, verified |
| positive | [`TestRFC5883SingleHopEchoAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L108) | unit/verify | unproven |

### [`RFC5883-4.1-1`](#rfc5883-4.1-1)

Multiple sessions between the same pair of systems must have at least one endpoint address distinct from one another (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5883MultihopSessionsNeedDistinctEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5883_multihop_pair_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestRFC5883MultihopSessionsNeedDistinctEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5883_multihop_pair_test.go#L35) | unit/verify | revert, verified |

### [`RFC5883-4.3-1`](#rfc5883-4.3-1)

In this approach, the Unidirectional Sender MUST operate in the Active role (as defined in the base BFD specification) (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-09-27. Forbidden: the sender taking the Passive role (silent). TestRFC5883DefaultSessionActiveArmsTx fails on m.role != RoleActive or a zero NextTxDeadline; TestRFC5883PassiveSessionDoesNotArmTx shows arming depends on the role.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5883PassiveSessionDoesNotArmTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L53) | unit/verify | unproven |
| positive | [`TestRFC5883DefaultSessionActiveArmsTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L39) | unit/verify | unproven |

### [`RFC5883-4.3-2`](#rfc5883-4.3-2)

the Unidirectional Receiver MUST operate in the Passive role. (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-09-27. Forbidden: a receiver in the Passive role transmitting before it has received a packet. TestRFC5883PassiveSessionSilentUntilRx fails on a non-zero NextTxDeadline before any Receive; TestRFC5883PassiveSessionTransmitsAfterRx shows the silence lifts on the first packet. Ze has no unidirectional-link notion: the operator configures passive, and the units prove the Passive role behaves as defined.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5883PassiveSessionTransmitsAfterRx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L79) | unit/verify | unproven |
| positive | [`TestRFC5883PassiveSessionSilentUntilRx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L68) | unit/verify | unproven |

### [`RFC5883-5-1`](#rfc5883-5-1)

The encapsulation of BFD Control packets for multihop application in IPv4 and IPv6 is identical to that defined in [BFD-1HOP], except that the UDP destination port MUST have a value of 4784. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, netns+pinning). Units now run in a rootless user+net namespace (userns.Enter), which removes the prior weak reason (fixed host ports colliding with a running ze or a parallel run); the judge ran them: each child ran and passed, none skipped. §5 "...except that the UDP destination port MUST have a value of 4784." + TestRFC5883MultiHopControlSentToPort4784OnTheWire (IPv4 and IPv6), - TestRFC5883SingleHopControlNeverAddressesPort4784 (3784 seen, silence at 4784). Records on destination (+) and newUDPTransport (-) re-recorded and observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5883SingleHopControlNeverAddressesPort4784`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_wire_port_linux_test.go#L230) | unit/verify | revert, verified |
| positive | [`TestRFC5883MultiHopControlSentToPort4784OnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_wire_port_linux_test.go#L213) | unit/verify | revert, verified |

### [`RFC5883-x-1`](#rfc5883-x-1)

Finally, the Echo function MUST NOT be used over multiple hops. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-09-27; the quote is the same §3 sentence as RFC5883-3-1 and the same two units carry it. Forbidden: Echo used over multiple hops; TestRFC5883MultiHopEchoRejected fails when validate accepts it, TestRFC5883SingleHopEchoAccepted is the scoping contrast.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5883MultiHopEchoRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L98) | unit/verify | unproven |
| positive | [`TestRFC5883SingleHopEchoAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L112) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc5883.txt |
| Source fingerprint | edbd101dc21d89ad |
| Record | rfc/extraction/rfc5883.json |
| Mapped sentences | 4 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 1 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.3` | not stated | 1 | walked | not stated |
| `5` | not stated | 1 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust copyright boilerplate in the Status of This Memo section: it binds the extraction of code components from the document, not a BFD speaker, and the keyword is lowercase 'must'. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 2 Applicability is descriptive deployment guidance addressed to the operator ('it is required that the operator correctly provision the rates'), with a lowercase keyword and no obligation on a BFD implementation. The implementation-side congestion obligation lives in RFC 5880 Section 7, which rfc/short/rfc5880.md carries as RFC5880-7-1 and RFC5880-7-2. | In these scenarios it is required that the operator correctly provision the rates at which BFD is transmitted to avoid congestion (e.g link, I/O, CPU) and false failure detection. |

## Superseded

No document obsoletes RFC 5883, so its obligations are stated where they were written.
