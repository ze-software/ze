# RFC 5883 - Bidirectional Forwarding Detection (BFD) for Multihop Paths

BFD multi-hop. Every requirement listed for RFC 5883, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 6 | of 6 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 6 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 6 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 6 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 6 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 6 | 6 of 14 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 6 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 6 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 6 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 6 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 6 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 6 | of 7 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 6 mandatory requirements: they add up to 6. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | BFD multi-hop |
| Automatic checks | Included |
| Requirements | 7 |
| Mandatory requirements (MUST level) | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 14 |
| Tests linked to requirements | 14 |
| Recorded audit verdicts | 5 |
| Fault test records | 6 |
| Summary | `rfc/short/rfc5883.md` |
| Requirement table | `rfc/requirements/rfc5883.md` |
| RFC text | `rfc/full/rfc5883.txt` |

## Automatic checks

Included in automatic checks: BFD for Multihop Paths: nine MUST-level requirements after the 2026-09-21 extraction walk, which added RFC5883-4.1-1 (two sessions between the same pair of systems have at least one endpoint address distinct from one another, §4.1) with no test and no annotation. Six of the other eight are met with new positive+negative tests in internal/component/bfd asserting producer decisions: 5-1 (multihop uses UDP destination port 4784), 5-2 (single-hop 3784 and multihop 4784 use separate ports), 4.3-1 (a default session takes the Active role and arms its transmit timer), 4.3-2 (a Passive session stays silent until it receives a packet), and 3-1 / x-1 (BFD Echo is rejected on a multihop path). 7-1 and 7-2 (congestion detection and congestion-triggered transmit-rate reduction) were retired on 2026-09-27: the obligation is RFC 5880 Section 7's, carried with its gap as RFC5880-7-1 and RFC5880-7-2, and ze has no BFD congestion-control code path (only slow-start and jitter). Disclosed in the docs/features/rfc-status.md RFC 5883 row.

## Published support status

**Area:** BFD multi-hop

**Test verification:** 3 of 6 MUST proven

**Evidence:** unit 3

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **6** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (6):** [`RFC5883-3-1`](#rfc5883-3-1), [`RFC5883-4.1-1`](#rfc5883-4.1-1), [`RFC5883-4.3-1`](#rfc5883-4.3-1), [`RFC5883-4.3-2`](#rfc5883-4.3-2), [`RFC5883-5-1`](#rfc5883-5-1), [`RFC5883-x-1`](#rfc5883-x-1)

## Figures

- **MUST bar**: Ze verified 3, Tests awaiting verification 3
- **Overview bar**: Ze MUST verified 3, Tests awaiting verification 3, Not assessed 1

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 6 |
| MUST rows kept (the MUST bar) | 6 |
| Ze verified | 3 |
| OS verified | 0 |
| Tests awaiting verification | 3 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 1 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 1 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc5883.json](../../../data/rfc/rfc5883.json).

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

RFC 5883 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC5883-3-1`](#rfc5883-3-1)

Finally, the Echo function MUST NOT be used over multiple hops. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RA-BFD strict re-read 2026-09-27 (stale after DF-BFD-4). The row quotes RFC 5883 Section 3 verbatim: "Finally, the Echo function MUST NOT be used over multiple hops." (rfc5883.txt line 132). Forbidden: a multi-hop session that runs Echo. A session's DesiredMinEchoTxInterval, and the Required Min Echo RX it advertises (session.go Init), come only from a profile through profileConfig.applyTo, and every entry passes profileConfig.permitsMode. Red: TestRFC5883MultiHopEchoRejected Fatalf when pluginConfig.validate accepts an echo profile on a multi-hop pinned session; TestRFC5883ClientMultiHopEchoProfileRefused Fatal when EnsureSession returns a handle for a multi-hop client request naming an echo profile, or lacks the `enables echo` refusal. Positive, same profile on single hop: TestRFC5883SingleHopEchoAccepted (validate) and TestRFC5883ClientSingleHopEchoProfileAccepted (EnsureSession), so a blanket echo refusal fails. The engine holds no second guard (engine.go NewLoopWithEcho comment).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5883ClientMultiHopEchoProfileRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/enabled_test.go#L228) | unit/verify | revert, verified |
| negative | [`TestRFC5883MultiHopEchoRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L94) | unit/verify | not verified |
| positive | [`TestRFC5883ClientSingleHopEchoProfileAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/enabled_test.go#L243) | unit/verify | revert, verified |
| positive | [`TestRFC5883SingleHopEchoAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L108) | unit/verify | not verified |

### [`RFC5883-4.1-1`](#rfc5883-4.1-1)

Multiple sessions between the same pair of systems must have at least one endpoint address distinct from one another (§4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5883MultihopSessionsNeedDistinctEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5883_multihop_pair_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestRFC5883MultihopSessionsNeedDistinctEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5883_multihop_pair_test.go#L35) | unit/verify | revert, verified |

### [`RFC5883-4.3-1`](#rfc5883-4.3-1)

In this approach, the Unidirectional Sender MUST operate in the Active role (as defined in the base BFD specification) (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-09-27. Forbidden: the sender taking the Passive role (silent). TestRFC5883DefaultSessionActiveArmsTx fails on m.role != RoleActive or a zero NextTxDeadline; TestRFC5883PassiveSessionDoesNotArmTx shows arming depends on the role.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5883PassiveSessionDoesNotArmTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L53) | unit/verify | not verified |
| positive | [`TestRFC5883DefaultSessionActiveArmsTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L39) | unit/verify | not verified |

### [`RFC5883-4.3-2`](#rfc5883-4.3-2)

the Unidirectional Receiver MUST operate in the Passive role. (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-09-27. Forbidden: a receiver in the Passive role transmitting before it has received a packet. TestRFC5883PassiveSessionSilentUntilRx fails on a non-zero NextTxDeadline before any Receive; TestRFC5883PassiveSessionTransmitsAfterRx shows the silence lifts on the first packet. Ze has no unidirectional-link notion: the operator configures passive, and the units prove the Passive role behaves as defined.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5883PassiveSessionTransmitsAfterRx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L79) | unit/verify | not verified |
| positive | [`TestRFC5883PassiveSessionSilentUntilRx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5883_test.go#L68) | unit/verify | not verified |

### [`RFC5883-5-1`](#rfc5883-5-1)

The encapsulation of BFD Control packets for multihop application in IPv4 and IPv6 is identical to that defined in [BFD-1HOP], except that the UDP destination port MUST have a value of 4784. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, netns+pinning). Units now run in a rootless user+net namespace (userns.Enter), which removes the prior weak reason (fixed host ports colliding with a running ze or a parallel run); the judge ran them: each child ran and passed, none skipped. §5 "...except that the UDP destination port MUST have a value of 4784." + TestRFC5883MultiHopControlSentToPort4784OnTheWire (IPv4 and IPv6), - TestRFC5883SingleHopControlNeverAddressesPort4784 (3784 seen, silence at 4784). Records on destination (+) and newUDPTransport (-) re-recorded and observed red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5883SingleHopControlNeverAddressesPort4784`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_wire_port_linux_test.go#L230) | unit/verify | revert, verified |
| positive | [`TestRFC5883MultiHopControlSentToPort4784OnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_wire_port_linux_test.go#L213) | unit/verify | revert, verified |

### [`RFC5883-x-1`](#rfc5883-x-1)

Finally, the Echo function MUST NOT be used over multiple hops. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-09-27; the quote is the same §3 sentence as RFC5883-3-1 and the same two units carry it. Forbidden: Echo used over multiple hops; TestRFC5883MultiHopEchoRejected fails when validate accepts it, TestRFC5883SingleHopEchoAccepted is the scoping contrast.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5883MultiHopEchoRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L98) | unit/verify | not verified |
| positive | [`TestRFC5883SingleHopEchoAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5883_test.go#L112) | unit/verify | not verified |

## Requirement list review

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
| `front` | not stated | 1 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 1 | reviewed | not stated |
| `3` | not stated | 1 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 1 | reviewed | not stated |
| `4.2` | not stated | 0 | reviewed | not stated |
| `4.3` | not stated | 1 | reviewed | not stated |
| `5` | not stated | 1 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 0 | reviewed | not stated |
| `9.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust copyright boilerplate in the Status of This Memo section: it binds the extraction of code components from the document, not a BFD speaker, and the keyword is lowercase 'must'. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 2 Applicability is descriptive deployment guidance addressed to the operator ('it is required that the operator correctly provision the rates'), with a lowercase keyword and no obligation on a BFD implementation. The implementation-side congestion obligation lives in RFC 5880 Section 7, which rfc/short/rfc5880.md carries as RFC5880-7-1 and RFC5880-7-2. | In these scenarios it is required that the operator correctly provision the rates at which BFD is transmitted to avoid congestion (e.g link, I/O, CPU) and false failure detection. |

## Superseded

No document obsoletes RFC 5883, so its obligations are stated where they were written.
