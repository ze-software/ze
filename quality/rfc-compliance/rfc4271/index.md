# RFC 4271 - A Border Gateway Protocol 4 (BGP-4)

BGP-4 base protocol. Every requirement this repository extracted from RFC 4271, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 112 | of 133 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 2 | of 133 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 133 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 133 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 247 | 247 of 454 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 133 | of 167 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 133 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 133 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 133 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 133 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 133 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 19 | of 133 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 95 | of 133 gated MUSTs judged | 4 weak, wrong or unimplemented, 4 no longer current. Each is named below under its own requirement id |

The 9 counts marked as a part above are the whole of the 133 gated MUSTs: they add up to 133. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | BGP-4 base protocol |
| Enrolment | Enrolled |
| Requirements | 167 |
| Gated MUST-level | 133 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 19 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 454 |
| Tagged units | 454 |
| Recorded audit verdicts | 95 |
| Discrimination records | 247 |
| Summary | `rfc/short/rfc4271.md` |
| Requirement shard | `rfc/requirements/rfc4271.md` |
| RFC text | `rfc/full/rfc4271.txt` |

## Enrolment

Enrolled: A Border Gateway Protocol 4 (BGP-4): Section 5.1.4 requires a local-configuration mechanism that removes MULTI_EXIT_DISC from a route. Ze exposes the full obligation as `modify NAME { del { med; } }` on an import chain, before Decision Process phases 1 and 2.

## What the public ledger says

**Area:** BGP-4 base protocol

**Proof:** 85 of 133 MUST proven

**Evidence:** interop 4, functional 6, unit 75

**Unmet:** RFC4271-3.1-2, RFC4271-5.1.6-1, RFC4271-5.1.7-1, RFC4271-6.2-3, RFC4271-8.2.1-3, RFC4271-8.2.2-13, RFC4271-9.1.2-1, RFC4271-9.1.2-4, RFC4271-9.1.2.1-2, RFC4271-9.1.2.2-2, RFC4271-9.2-2, RFC4271-9.2-3, RFC4271-9.2.1.1-2, RFC4271-9.2.1.1-3, RFC4271-9.2.2.2-1, RFC4271-9.2.2.2-2, RFC4271-9.2.2.2-3, RFC4271-9.2.2.2-4, RFC4271-9.2.2.2-5


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 112 | one part of the gated population |
| Annotated (including scoped evidence) | 21 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **133** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (112):** [`RFC4271-4.1-1`](#rfc4271-4.1-1), [`RFC4271-4.1-2`](#rfc4271-4.1-2), [`RFC4271-4.1-3`](#rfc4271-4.1-3), [`RFC4271-4.3-1`](#rfc4271-4.3-1), [`RFC4271-4.3-2`](#rfc4271-4.3-2), [`RFC4271-4.3-3`](#rfc4271-4.3-3), [`RFC4271-4.3-4`](#rfc4271-4.3-4), [`RFC4271-4.4-1`](#rfc4271-4.4-1), [`RFC4271-4.4-2`](#rfc4271-4.4-2), [`RFC4271-6-1`](#rfc4271-6-1), [`RFC4271-4.2-1`](#rfc4271-4.2-1), [`RFC4271-4.2-2`](#rfc4271-4.2-2), [`RFC4271-6.2-1`](#rfc4271-6.2-1), [`RFC4271-6.2-2`](#rfc4271-6.2-2), [`RFC4271-5-1`](#rfc4271-5-1), [`RFC4271-5-2`](#rfc4271-5-2), [`RFC4271-5-3`](#rfc4271-5-3), [`RFC4271-5-4`](#rfc4271-5-4), [`RFC4271-5-5`](#rfc4271-5-5), [`RFC4271-5-6`](#rfc4271-5-6), [`RFC4271-5.1.3-1`](#rfc4271-5.1.3-1), [`RFC4271-5.1.3-2`](#rfc4271-5.1.3-2), [`RFC4271-5.1.3-3`](#rfc4271-5.1.3-3), [`RFC4271-5.1.4-1`](#rfc4271-5.1.4-1), [`RFC4271-5.1.4-4`](#rfc4271-5.1.4-4), [`RFC4271-5.1.4-2`](#rfc4271-5.1.4-2), [`RFC4271-5.1.5-1`](#rfc4271-5.1.5-1), [`RFC4271-5.1.5-2`](#rfc4271-5.1.5-2), [`RFC4271-5.1.5-3`](#rfc4271-5.1.5-3), [`RFC4271-5.1.5-4`](#rfc4271-5.1.5-4), [`RFC4271-6.1-1`](#rfc4271-6.1-1), [`RFC4271-6.1-2`](#rfc4271-6.1-2), [`RFC4271-6.1-3`](#rfc4271-6.1-3), [`RFC4271-6.1-4`](#rfc4271-6.1-4), [`RFC4271-6.3-1`](#rfc4271-6.3-1), [`RFC4271-6.7-1`](#rfc4271-6.7-1), [`RFC4271-8.2.1-1`](#rfc4271-8.2.1-1), [`RFC4271-8.2.1-2`](#rfc4271-8.2.1-2), [`RFC4271-8.2.2-1`](#rfc4271-8.2.2-1), [`RFC4271-8.2.2-2`](#rfc4271-8.2.2-2), [`RFC4271-8.2.2-3`](#rfc4271-8.2.2-3), [`RFC4271-8.2.2-4`](#rfc4271-8.2.2-4), [`RFC4271-8.2.2-5`](#rfc4271-8.2.2-5), [`RFC4271-8.2.2-7`](#rfc4271-8.2.2-7), [`RFC4271-8.2.2-20`](#rfc4271-8.2.2-20), [`RFC4271-8.2.2-8`](#rfc4271-8.2.2-8), [`RFC4271-8.2.2-9`](#rfc4271-8.2.2-9), [`RFC4271-8.2.2-10`](#rfc4271-8.2.2-10), [`RFC4271-8.2.2-11`](#rfc4271-8.2.2-11), [`RFC4271-8.2.2-12`](#rfc4271-8.2.2-12), [`RFC4271-8.2.2-14`](#rfc4271-8.2.2-14), [`RFC4271-8.2.2-15`](#rfc4271-8.2.2-15), [`RFC4271-8.2.2-21`](#rfc4271-8.2.2-21), [`RFC4271-8.2.2-22`](#rfc4271-8.2.2-22), [`RFC4271-8.2.2-23`](#rfc4271-8.2.2-23), [`RFC4271-8.2.2-24`](#rfc4271-8.2.2-24), [`RFC4271-8.2.2-25`](#rfc4271-8.2.2-25), [`RFC4271-8.2.2-26`](#rfc4271-8.2.2-26), [`RFC4271-8.2.2-27`](#rfc4271-8.2.2-27), [`RFC4271-8.2.2-28`](#rfc4271-8.2.2-28), [`RFC4271-8.2.2-16`](#rfc4271-8.2.2-16), [`RFC4271-8.2.2-17`](#rfc4271-8.2.2-17), [`RFC4271-10-1`](#rfc4271-10-1), [`RFC4271-5.1.2-2`](#rfc4271-5.1.2-2), [`RFC4271-5.1.2-3`](#rfc4271-5.1.2-3), [`RFC4271-5.1.5-5`](#rfc4271-5.1.5-5), [`RFC4271-6.7-4`](#rfc4271-6.7-4), [`RFC4271-6.8-1`](#rfc4271-6.8-1), [`RFC4271-6.8-2`](#rfc4271-6.8-2), [`RFC4271-9-1`](#rfc4271-9-1), [`RFC4271-9-2`](#rfc4271-9-2), [`RFC4271-9-3`](#rfc4271-9-3), [`RFC4271-9.1.1-1`](#rfc4271-9.1.1-1), [`RFC4271-9.1.1-2`](#rfc4271-9.1.1-2), [`RFC4271-9.1.2-2`](#rfc4271-9.1.2-2), [`RFC4271-9.1.2-3`](#rfc4271-9.1.2-3), [`RFC4271-9.1.2.1-1`](#rfc4271-9.1.2.1-1), [`RFC4271-9.1.2.2-1`](#rfc4271-9.1.2.2-1), [`RFC4271-9.1.2.2-3`](#rfc4271-9.1.2.2-3), [`RFC4271-9.1.2.2-4`](#rfc4271-9.1.2.2-4), [`RFC4271-9.2-4`](#rfc4271-9.2-4), [`RFC4271-9.2-5`](#rfc4271-9.2-5), [`RFC4271-Security-1`](#rfc4271-security-1), [`RFC4271-9.2-6`](#rfc4271-9.2-6), [`RFC4271-9.2-7`](#rfc4271-9.2-7), [`RFC4271-9.2-8`](#rfc4271-9.2-8), [`RFC4271-9.2-9`](#rfc4271-9.2-9), [`RFC4271-9.2-10`](#rfc4271-9.2-10), [`RFC4271-5-8`](#rfc4271-5-8), [`RFC4271-6.2-5`](#rfc4271-6.2-5), [`RFC4271-6.2-6`](#rfc4271-6.2-6), [`RFC4271-6.2-7`](#rfc4271-6.2-7), [`RFC4271-6.2-8`](#rfc4271-6.2-8), [`RFC4271-6.2-9`](#rfc4271-6.2-9), [`RFC4271-6.2-10`](#rfc4271-6.2-10), [`RFC4271-6.3-4`](#rfc4271-6.3-4), [`RFC4271-6.3-5`](#rfc4271-6.3-5), [`RFC4271-6.3-6`](#rfc4271-6.3-6), [`RFC4271-6.3-7`](#rfc4271-6.3-7), [`RFC4271-6.3-8`](#rfc4271-6.3-8), [`RFC4271-6.3-9`](#rfc4271-6.3-9), [`RFC4271-6.3-10`](#rfc4271-6.3-10), [`RFC4271-6.3-11`](#rfc4271-6.3-11), [`RFC4271-6.3-12`](#rfc4271-6.3-12), [`RFC4271-6.3-13`](#rfc4271-6.3-13), [`RFC4271-6.3-14`](#rfc4271-6.3-14), [`RFC4271-6.3-15`](#rfc4271-6.3-15), [`RFC4271-6.3-16`](#rfc4271-6.3-16), [`RFC4271-6.3-17`](#rfc4271-6.3-17), [`RFC4271-8.2.2-19`](#rfc4271-8.2.2-19), [`RFC4271-9-4`](#rfc4271-9-4), [`RFC4271-10-4`](#rfc4271-10-4)

**Annotated (including scoped evidence) (21):** [`RFC4271-4.3-5`](#rfc4271-4.3-5), [`RFC4271-5.1.6-1`](#rfc4271-5.1.6-1), [`RFC4271-6.2-3`](#rfc4271-6.2-3), [`RFC4271-8.2.1-3`](#rfc4271-8.2.1-3), [`RFC4271-8.2.2-13`](#rfc4271-8.2.2-13), [`RFC4271-3.1-2`](#rfc4271-3.1-2), [`RFC4271-5.1.4-3`](#rfc4271-5.1.4-3), [`RFC4271-5.1.7-1`](#rfc4271-5.1.7-1), [`RFC4271-9.1.2-1`](#rfc4271-9.1.2-1), [`RFC4271-9.1.2-4`](#rfc4271-9.1.2-4), [`RFC4271-9.1.2.1-2`](#rfc4271-9.1.2.1-2), [`RFC4271-9.1.2.2-2`](#rfc4271-9.1.2.2-2), [`RFC4271-9.2-2`](#rfc4271-9.2-2), [`RFC4271-9.2-3`](#rfc4271-9.2-3), [`RFC4271-9.2.1.1-2`](#rfc4271-9.2.1.1-2), [`RFC4271-9.2.2.2-1`](#rfc4271-9.2.2.2-1), [`RFC4271-9.2.2.2-2`](#rfc4271-9.2.2.2-2), [`RFC4271-9.2.2.2-3`](#rfc4271-9.2.2.2-3), [`RFC4271-9.2.2.2-4`](#rfc4271-9.2.2.2-4), [`RFC4271-9.2.2.2-5`](#rfc4271-9.2.2.2-5), [`RFC4271-9.2.1.1-3`](#rfc4271-9.2.1.1-3)

## Figures

- **MUST bar**: Ze proven 85, Ze correctness gap 48
- **Overview bar**: Ze MUST proven 85, SHOULD proven 3, SHOULD not decided 31, Ze correctness gap 48

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 133 |
| MUST rows kept (the MUST bar) | 133 |
| Ze proven | 85 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 48 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 34 |
| SHOULD proven | 3 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 31 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc4271.json](../../../data/rfc/rfc4271.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4271-4.1-1` | This 16-octet field is included for compatibility; it MUST be set to all ones. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4271EveryMessageSentWithAllOnesMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_header_send_test.go#L54). **positive:** `unit/verify` [`TestRFC4271MarkerAllOnesOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L20). **negative:** `unit/verify` [`TestRFC4271EveryMessageSentWithAllOnesMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_header_send_test.go#L56). **negative:** `unit/verify` [`TestRFC4271MarkerNotAllOnesRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L43) |
| `RFC4271-4.1-2` | Therefore, the Length field MUST have the smallest value required, given the rest of the message. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4271EveryMessageSentWithSmallestLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_header_send_test.go#L128). **positive:** `unit/verify` [`TestRFC4271SmallestLengthOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L63). **negative:** `unit/verify` [`TestRFC4271EveryMessageSentWithSmallestLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_header_send_test.go#L130). **negative:** `unit/verify` [`TestRFC4271NonSmallestLengthRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L96) |
| `RFC4271-4.1-3` | The value of the Length field MUST always be at least 19 and no greater than 4096 (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4271MessageLengthWithinBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L125). **positive:** `unit/verify` [`TestRFC4271UpdatesSentWithinLengthBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_header_send_test.go#L86). **negative:** `unit/verify` [`TestRFC4271MessageLengthOutOfBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L151). **negative:** `unit/verify` [`TestRFC4271UpdatesSentWithinLengthBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_header_send_test.go#L88) |
| `RFC4271-4.3-1` | For well-known attributes, the Transitive bit MUST be set to 1 (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC4271WellKnownAttributesAreTransitive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L31). **negative:** `unit/verify` [`TestRFC4271WellKnownAttributeErrorsAreCaught`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L501) |
| `RFC4271-4.3-2` | For well-known attributes and for optional non-transitive attributes, the Partial bit MUST be set to 0. (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC4271PartialBitClearOnSend`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L55). **positive:** `unit/verify` [`TestRFC4271PartialClearedOnTheRelayedWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_relay_partial_test.go#L48). **positive:** `unit/verify` [`TestRFC4271PartialNotSetOnRecognizedOrNonTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1123). **negative:** `unit/verify` [`TestRFC4271PartialBitClearedOnReadvertisedWellKnown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L47). **negative:** `unit/verify` [`TestRFC4271PartialClearedWhenTheRailReadvertises`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc4271_partial_test.go#L75). **negative:** `unit/verify` [`TestRFC4271PartialNotStampedOnExcludedClasses`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L170) |
| `RFC4271-4.3-3` | The lower-order four bits of the Attribute Flags octet are unused. They MUST be zero when sent (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC4271AttributeFlagsLowNibbleZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L80). **positive:** `unit/verify` [`TestRFC4271RelayedLowNibbleClearedOnBothRails`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_attr_flags_relay_test.go#L27). **positive:** `unit/verify` [`TestRFC4271WriteClearsLowNibbleEveryAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_attr_flags_egress_test.go#L109). **negative:** `unit/verify` [`TestRFC4271WriteLeavesCleanFlagsUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_attr_flags_egress_test.go#L135). **positive:** `functional/verify` [`rfc4271-attr-flags-low-nibble-relay.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc4271-attr-flags-low-nibble-relay.ci#L4) |
| `RFC4271-4.3-4` | The lower-order four bits of the Attribute Flags octet are unused. They MUST be zero when sent and MUST be ignored when received. (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC4271AttrFlagsLowNibbleIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L233). **positive:** `unit/verify` [`TestRFC4271RelayedLowNibbleClearedOnBothRails`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_attr_flags_relay_test.go#L26). **negative:** `unit/verify` [`TestRFC4271AttrFlagsHighBitsNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L260). **positive:** `functional/verify` [`rfc4271-attr-flags-low-nibble-relay.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc4271-attr-flags-low-nibble-relay.ci#L5) |
| `RFC4271-4.4-1` | KEEPALIVE messages MUST NOT be sent more frequently than one per second (§4.4) | MUST NOT | 4.4 | **positive:** `unit/verify` [`TestRFC4271KeepaliveNotFasterThanOnePerSecond`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_test.go#L18). **negative:** `unit/verify` [`TestRFC4271KeepaliveIntervalNeverSubSecond`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_test.go#L50) |
| `RFC4271-4.4-2` | If the negotiated Hold Time interval is zero, then periodic KEEPALIVE messages MUST NOT be sent. (§4.4) | MUST NOT | 4.4 | **positive:** `unit/verify` [`TestRFC4271NonZeroNegotiatedHoldSendsPeriodicKeepalives`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_keepalive_session_test.go#L70). **positive:** `unit/verify` [`TestTimersKeepaliveTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_timer_test.go#L144). **negative:** `unit/verify` [`TestKeepaliveWithZeroHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_timer_test.go#L365). **negative:** `unit/verify` [`TestRFC4271ZeroNegotiatedHoldSendsNoPeriodicKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_keepalive_session_test.go#L92) |
| `RFC4271-6-1` | If no Error Subcode is specified, then a zero MUST be used. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4271NotificationUnspecifiedSubcodeIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L354). **negative:** `unit/verify` [`TestRFC4271NotificationSpecifiedSubcodePreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L377) |
| `RFC4271-4.2-1` | Hold Time MUST be either zero or at least three seconds (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L339). **negative:** `unit/verify` [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L341). **positive:** `functional/verify` [`open-hold-time-peer-lower-wins.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/open-hold-time-peer-lower-wins.ci#L3) |
| `RFC4271-4.2-2` | Upon receipt of an OPEN message, a BGP speaker MUST calculate the value of the Hold Timer by using the smaller of its configured Hold Time and the Hold Time received in the OPEN message. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestNegotiateWith_HoldTimeMinOfBoth`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L45). **negative:** `unit/verify` [`TestNegotiateWith_HoldTimeZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L74) |
| `RFC4271-6.2-1` | An implementation MUST reject Hold Time values of one or two seconds (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L343). **negative:** `unit/verify` [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L345) |
| `RFC4271-6.2-2` | An implementation that accepts a Hold Time MUST use the negotiated value (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC4271NegotiatedHoldTimeDrivesTimers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L169). **negative:** `unit/verify` [`TestRFC4271LocalHoldTimeNotUsedWhenPeerProposesSmaller`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L190) |
| `RFC4271-5-1` | BGP implementations MUST recognize all well-known attributes (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4271WellKnownAttributesAreRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L464). **negative:** `unit/verify` [`TestRFC4271WellKnownAttributeErrorsAreCaught`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L493) |
| `RFC4271-5-2` | Some of these attributes are mandatory and MUST be included in every UPDATE message that contains NLRI. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4271UpdateWithNLRICarriesTheMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_mandatory_send_test.go#L23). **positive:** `unit/verify` [`TestRFC4271WellKnownAttributesAreRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L467). **negative:** `unit/verify` [`TestBuildUnicastRefusesIPv4RouteWithNoUsableNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_ipv4_route_ipv6_nexthop_test.go#L30). **negative:** `unit/verify` [`TestRFC4271UpdateWithNLRICarriesTheMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_mandatory_send_test.go#L26). **negative:** `unit/verify` [`TestRFC4271WellKnownAttributeErrorsAreCaught`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L498) |
| `RFC4271-5-3` | If a path with an unrecognized transitive optional attribute is accepted and passed to other BGP peers, then the unrecognized transitive optional attribute of that path MUST be passed, along with the path, to other BGP peers with the Partial bit in the Attribute Flags octet set to 1. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4271PartialSetOnUnrecognizedTransitiveOptional`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1091). **positive:** `unit/verify` [`TestRFC4271PartialStampedOnUnrecognizedTransitive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L132). **negative:** `unit/verify` [`TestRFC4271PartialNotSetOnRecognizedOrNonTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1120). **negative:** `unit/verify` [`TestRFC4271PartialNotStampedOnExcludedClasses`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L167). **positive:** `functional/verify` [`rfc4271-partial-unknown-transitive.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc4271-partial-unknown-transitive.ci#L32) |
| `RFC4271-5-4` | If a path with a recognized, transitive optional attribute is accepted and passed along to other BGP peers and the Partial bit in the Attribute Flags octet is set to 1 by some previous AS, it MUST NOT be set back to 0 by the current AS. (§5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC4271PartialBitPreservedOnUnknownTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L82). **positive:** `unit/verify` [`TestRFC4271PartialFromPreviousASNeverCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1159). **negative:** `unit/verify` [`TestRFC4271PartialBitSurvivesLengthReframing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L115). **negative:** `unit/verify` [`TestRFC4271PartialFromPreviousASNotCleared`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L197) |
| `RFC4271-5-5` | Unrecognized non-transitive optional attributes MUST be quietly ignored (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4271UnrecognizedNonTransitiveIsNotPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1217). **negative:** `unit/verify` [`TestRFC4271TheNonTransitiveDropSparesEveryOtherClass`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1258) |
| `RFC4271-5-6` | The receiver of an UPDATE message MUST be prepared to handle path attributes within UPDATE messages that are out of order. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4271AttributesOutOfOrderAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L294). **positive:** `unit/verify` [`TestRFC4271OutOfOrderAttributesReachTheReceivePathIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_out_of_order_receive_test.go#L26). **negative:** `unit/verify` [`TestRFC4271OutOfOrderAttributesReachTheReceivePathIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_out_of_order_receive_test.go#L29). **negative:** `unit/verify` [`TestRFC4271OutOfOrderDoesNotMaskMalformation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L324) |
| `RFC4271-4.3-5` | However, a BGP speaker MUST be able to process UPDATE messages in this form. (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRIBInjectSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L119). **positive:** `unit/verify` [`TestRIBPoolPathSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L85). **positive:** `unit/verify` [`TestRIBSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L39). **negative:** no negative test. **{single-polarity}:** the obligation is to ACCEPT a message shape, so there is no non-conformant input to reject -- every UPDATE of this form must be processed, and a negative case would have to assert the absence of an error, which proves nothing (ai/rules/testing.md). The consequence the same paragraph asks for, treating the UPDATE as though WITHDRAWN did not contain the prefix, is RFC4271-4.3-7 and is proven by the same test |
| `RFC4271-5.1.3-1` | A route originated by a BGP speaker SHALL NOT be advertised to a peer using an address of that peer as NEXT_HOP. (§5.1.3) | SHALL NOT | 5.1.3 | **positive:** `unit/verify` [`TestEgressNextHopIsPeerOwnReadsTheRewrittenAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L294). **positive:** `unit/verify` [`TestForwardRSWithholdsRouteWhoseNextHopIsTheClientsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L260). **positive:** `unit/verify` [`TestForwardWithdrawsFromDestinationWhoseNextHopIsItsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L225). **positive:** `unit/verify` [`TestForwardWithholdsRouteWhoseNextHopIsTheDestinationsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L185). **positive:** `unit/verify` [`TestSendAnnounceWithholdsRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L476). **positive:** `unit/verify` [`TestSendUpdateWithholdsOriginatedRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L437). **negative:** `unit/verify` [`TestEgressNextHopIsPeerOwnReadsTheRewrittenAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L298). **negative:** `unit/verify` [`TestForwardRSWithholdsRouteWhoseNextHopIsTheClientsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L263). **negative:** `unit/verify` [`TestForwardWithholdsRouteWhoseNextHopIsTheDestinationsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L190). **negative:** `unit/verify` [`TestSendAnnounceWithholdsRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L478). **negative:** `unit/verify` [`TestSendUpdateWithholdsOriginatedRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L441). **positive:** `functional/verify` [`originated-nexthop-peer-own.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/originated-nexthop-peer-own.ci#L7). **negative:** `functional/verify` [`originated-nexthop-peer-own.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/originated-nexthop-peer-own.ci#L10). **positive:** `interop/nightly` [`checkSelfNextHopWithheld`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1000). **negative:** `interop/nightly` [`checkSelfNextHopWithheld`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1001) |
| `RFC4271-5.1.3-2` | A BGP speaker SHALL NOT install a route with itself as the next hop (§5.1.3) | SHALL NOT | 5.1.3 | **positive:** `unit/verify` [`TestRFC4271SelfNextHopRouteIsNotInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L43). **positive:** `unit/verify` [`TestRFC4271SelfNextHopSetComesFromPeerEvents`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L122). **negative:** `unit/verify` [`TestRFC4271SelfNextHopDoesNotShadowASoundAlternative`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L89). **negative:** `unit/verify` [`TestRFC4271SelfNextHopRouteIsNotInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L47). **negative:** `unit/verify` [`TestRFC4271SelfNextHopSetComesFromPeerEvents`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L126) |
| `RFC4271-5.1.3-3` | A BGP speaker MUST be able to support the disabling advertisement of third party NEXT_HOP attributes in order to handle imperfectly bridged media. (§5.1.3) | MUST | 5.1.3 | **positive:** `unit/verify` [`TestRFC4271NextHopSelfDisablesThirdPartyNextHopOnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_third_party_nexthop_test.go#L41). **positive:** `unit/verify` [`TestRFC4271NextHopSelfWithAutoLocalAddressSendsTheConnectedEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_third_party_nexthop_test.go#L106). **positive:** `unit/verify` [`TestRFC4271ThirdPartyNextHopCanBeDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L339). **negative:** `unit/verify` [`TestRFC4271NextHopSelfWithNoLocalAddressWithholdsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_third_party_nexthop_test.go#L147). **negative:** `unit/verify` [`TestRFC4271ThirdPartyNextHopDisableFailsClosed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L373) |
| `RFC4271-5.1.4-1` | The MULTI_EXIT_DISC attribute received from a neighboring AS MUST NOT be propagated to other neighboring ASes. (§5.1.4) | MUST NOT | 5.1.4 | **positive:** `unit/verify` [`TestForwardSuppressesReceivedMEDToAnotherAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L198). **negative:** `unit/verify` [`TestForwardKeepsFilterSetMED`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L299). **negative:** `unit/verify` [`TestForwardSuppressesReceivedMEDToAnotherAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L203). **negative:** `unit/verify` [`TestForwardWritesLocallySetMED`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L266). **negative:** `unit/verify` [`TestMEDPropagationAllowedTo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L403). **positive:** `functional/verify` [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L4). **negative:** `functional/verify` [`med-locally-set-reaches-peer.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-locally-set-reaches-peer.ci#L4). **negative:** `functional/verify` [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L8). **positive:** `interop/nightly` [`checkMEDAcrossAS`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L197). **negative:** `interop/nightly` [`checkMEDAcrossAS`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L201) |
| `RFC4271-5.1.4-4` | A BGP speaker MUST implement a mechanism (based on local configuration) that allows the MULTI_EXIT_DISC attribute to be removed from a route (§5.1.4) | MUST | 5.1.4 | **positive:** `unit/verify` [`TestParseModifyDefsMEDRemove`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/rfc4271_modify_test.go#L314). **negative:** `unit/verify` [`TestParseModifyDefsMEDRemove`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/rfc4271_modify_test.go#L320). **positive:** `functional/verify` [`med-removal-configured.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-configured.ci#L4). **positive:** `interop/nightly` [`checkMEDRemovalConfiguration`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L319). **negative:** `interop/nightly` [`checkMEDRemovalConfiguration`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L323) |
| `RFC4271-5.1.4-2` | If a BGP speaker is configured to remove the MULTI_EXIT_DISC attribute from a route, then this removal MUST be done prior to determining the degree of preference of the route and prior to performing route selection (Decision Process phases 1 and 2). (§5.1.4) | MUST | 5.1.4 | **positive:** `unit/verify` [`TestHandleFilterUpdateMEDRemoveIsImportOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/rfc4271_modify_test.go#L389). **negative:** `unit/verify` [`TestHandleFilterUpdateMEDRemoveIsImportOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/rfc4271_modify_test.go#L396). **positive:** `functional/verify` [`med-removal-before-decision.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-before-decision.ci#L4). **positive:** `functional/verify` [`med-removal-configured.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-configured.ci#L8). **negative:** `functional/verify` [`med-removal-before-decision.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-before-decision.ci#L10). **negative:** `functional/verify` [`med-removal-export-refused.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-export-refused.ci#L4) |
| `RFC4271-5.1.5-1` | LOCAL_PREF is a well-known attribute that SHALL be included in all UPDATE messages that a given BGP speaker sends to other internal peers. (§5.1.5) | SHALL | 5.1.5 | **positive:** `unit/verify` [`TestRFC4271ForwardAddsLocalPrefTowardInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_a2_forward_test.go#L50). **positive:** `unit/verify` [`TestRFC4271LocalPrefIncludedForInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L81). **negative:** `unit/verify` [`TestAnnounceStripsLocalPrefTowardExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_api_origin_test.go#L307). **negative:** `unit/verify` [`TestForwardLocalPrefStrippedToExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_local_pref_test.go#L45). **negative:** `unit/verify` [`TestRFC4271ForwardAddsLocalPrefTowardInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_a2_forward_test.go#L51). **negative:** `unit/verify` [`TestRFC4271LocalPrefOmittedForExternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L107) |
| `RFC4271-5.1.5-2` | A BGP speaker MUST NOT include this attribute in UPDATE messages it sends to external peers, except in the case of BGP Confederations [RFC3065]. (§5.1.5) | MUST NOT | 5.1.5 | **positive:** `unit/verify` [`TestAnnounceStripsLocalPrefTowardExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_api_origin_test.go#L305). **positive:** `unit/verify` [`TestForwardLocalPrefStripBeatsAFilterSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_local_pref_test.go#L113). **positive:** `unit/verify` [`TestForwardLocalPrefStrippedToExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_local_pref_test.go#L41). **positive:** `unit/verify` [`TestRFC4271LocalPrefOmittedForExternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L104). **negative:** `unit/verify` [`TestLocalPrefAllowedToIsTheOnlyAnswer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_local_pref_test.go#L146). **negative:** `unit/verify` [`TestRFC4271LocalPrefIncludedForInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L83). **positive:** `functional/verify` [`local-pref-strip-ebgp.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/local-pref-strip-ebgp.ci#L14). **negative:** `functional/verify` [`local-pref-strip-ebgp.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/local-pref-strip-ebgp.ci#L19). **positive:** `interop/nightly` [`checkLocalPrefStrip`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L95) |
| `RFC4271-5.1.5-3` | If it is contained in an UPDATE message that is received from an external peer, then this attribute MUST be ignored by the receiving speaker, except in the case of BGP Confederations [RFC3065]. (§5.1.5) | MUST | 5.1.5 | **positive:** `unit/verify` [`TestRFC4271LocalPrefFromExternalPeerNeverReachesTheRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L177). **positive:** `unit/verify` [`TestRFC4271LocalPrefKeptOnInternalSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L567). **negative:** `unit/verify` [`TestRFC4271LocalPrefFromExternalPeerNeverReachesTheRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L179). **negative:** `unit/verify` [`TestRFC4271LocalPrefIgnoredOnExternalSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L590) |
| `RFC4271-5.1.5-4` | The higher degree of preference MUST be preferred. (§5.1.5) | MUST | 5.1.5 | **positive:** `unit/verify` [`TestBestPath_LocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L207). **negative:** `unit/verify` [`TestBestPath_LocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L209) |
| `RFC4271-5.1.6-1` | A BGP speaker that receives a route with the ATOMIC_AGGREGATE attribute MUST NOT make any NLRI of that route more specific (as defined in 9.1.4) when advertising this route to other BGP speakers. (§5.1.6) | MUST NOT | 5.1.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: ze meets it, because readvertisement writes the route's stored NLRI bytes unchanged (internal/component/bgp/reactor/peer_rib_routes.go::buildRIBRouteUpdate) and no code makes a received prefix more specific or splits it; no test asserts it |
| `RFC4271-6.1-1` | All errors detected while processing the Message Header MUST be indicated by sending the NOTIFICATION message with the Error Code Message Header Error. (§6.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestRFC4271MessageHeaderBadLengthIsReported`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L128). **positive:** `unit/verify` [`TestRFC4271MessageHeaderErrorIsReported`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L29). **positive:** `unit/verify` [`TestRFC4271UnknownMessageTypeIsReported`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L158). **negative:** `unit/verify` [`TestRFC4271MessageHeaderAtTheBoundsIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L174). **negative:** `unit/verify` [`TestRFC4271OpenSentWellFormedOpenKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L201) |
| `RFC4271-6.1-2` | If the Marker field of the message header is not as expected, then a synchronization error has occurred and the Error Subcode MUST be set to Connection Not Synchronized. (§6.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestRFC4271MessageHeaderErrorIsReported`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L30). **negative:** `unit/verify` [`TestRFC4271OpenSentWellFormedOpenKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L202) |
| `RFC4271-6.1-3` | If at least one of the following is true: - if the Length field of the message header is less than 19 or greater than 4096, or - if the Length field of an OPEN message is less than the minimum length of the OPEN message, or - if the Length field of an UPDATE message is less than the minimum length of the UPDATE message, or - if the Length field of a KEEPALIVE message is not equal to 19, or - if the Length field of a NOTIFICATION message is less than the minimum length of the NOTIFICATION message, then the Error Subcode MUST be set to Bad Message Length. The Data field MUST contain the erroneous Length field. (§6.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestRFC4271MessageHeaderBadLengthIsReported`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L129). **positive:** `unit/verify` [`TestRFC4271MessageHeaderErrorIsReported`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L31). **negative:** `unit/verify` [`TestRFC4271MessageHeaderAtTheBoundsIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L175). **negative:** `unit/verify` [`TestRFC4271MessageHeaderAtTheUpperBoundsIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_bounds_peer_test.go#L49). **negative:** `unit/verify` [`TestRFC4271OpenSentWellFormedOpenKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L203) |
| `RFC4271-6.1-4` | If the Type field of the message header is not recognized, then the Error Subcode MUST be set to Bad Message Type. The Data field MUST contain the erroneous Type field. (§6.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestRFC4271UnknownMessageTypeIsReported`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L159). **negative:** `unit/verify` [`TestRFC4271MessageHeaderAtTheBoundsIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L176) |
| `RFC4271-6.2-3` | All errors detected while processing the OPEN message MUST be indicated by sending the NOTIFICATION message with the Error Code OPEN Message Error. (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** one class of OPEN error is detected and never reported. UnpackOpen returns the bare sentinel ErrShortRead when the body is under 10 octets or when the Optional Parameters Length (standard or RFC 9072 extended) overruns the body (internal/component/bgp/message/open.go:167-168, :193-194, :199-200, :209-210), and handleOpen turns that into an FSM event and a returned error, writing no NOTIFICATION and not even closing the connection (internal/component/bgp/reactor/session_handlers.go:43-47); session_read.go:264 only propagates it. Every other OPEN error path does send Error Code 2 -- unsupported version (session_handlers.go:54-60), unacceptable Hold Time (:70-77) and a malformed capability (rejectOpenCapabilityError, :185-199) -- so the obligation holds everywhere except the decode failure. Disclosed in docs/features/rfc-status.md RFC 4271 row |
| `RFC4271-6.3-1` | All errors detected while processing the UPDATE message MUST be indicated by sending the NOTIFICATION message with the Error Code UPDATE Message Error. (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC4271EstablishedUpdateErrorReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_established_teardown_peer_test.go#L130). **positive:** `unit/verify` [`TestRFC4271UpdateErrorReportedAsUpdateMessageError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L639). **positive:** `unit/verify` [`TestSessionRFC4271RetainedUpdateNotifications`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L160). **positive:** `unit/verify` [`TestSessionRFC7606DuplicateMPUnreachNotificationOnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_dupmp_unreach_wire_test.go#L32). **negative:** `unit/verify` [`TestRFC4271ConformantUpdateSendsNoUpdateError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L702) |
| `RFC4271-6.7-1` | However, the Cease NOTIFICATION message MUST NOT be used when a fatal error indicated by this section does exist. (§6.7) | MUST NOT | 6.7 | **positive:** `unit/verify` [`TestPrefixExceedTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L110). **positive:** `unit/verify` [`TestRFC4271PrefixLimitTeardownSendsCease`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_prefix_limit_cease_peer_test.go#L90). **negative:** `unit/verify` [`TestRFC4271EstablishedHoldTimerExpiryRunsTheEvent10List`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L239). **negative:** `unit/verify` [`TestRFC4271EstablishedUpdateErrorReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_established_teardown_peer_test.go#L131). **negative:** `unit/verify` [`TestRFC4271MessageHeaderErrorIsReported`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L32). **negative:** `unit/verify` [`TestRFC4271OpenConfirmUpdateIsAnFSMError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_error_peer_test.go#L96). **negative:** `unit/verify` [`TestRFC4271OpenSentErrorReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L126). **negative:** `unit/verify` [`TestRFC4271OpenSentUnexpectedMessageIsAnFSMError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_error_peer_test.go#L59). **negative:** `unit/verify` [`TestRFC4271UpdateErrorReportedAsUpdateMessageError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L642). **negative:** `unit/verify` [`TestSessionBFDStrictSecondOpenIsAnFSMErrorOnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L723) |
| `RFC4271-8.2.1-1` | BGP MUST maintain a separate FSM for each configured peer (§8.2.1) | MUST | 8.2.1 | **positive:** `unit/verify` [`TestRFC4271SeparateFSMPerPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L211). **negative:** `unit/verify` [`TestRFC4271PerPeerFSMDoesNotShareTimers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L235) |
| `RFC4271-8.2.1-2` | A BGP implementation MUST connect to and listen on TCP port 179 (§8.2.1) | MUST | 8.2.1 | **positive:** `unit/verify` [`TestRFC4271DefaultBGPPortIs179`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L304). **negative:** `unit/verify` [`TestRFC4271ExplicitPortOverridesDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L320) |
| `RFC4271-8.2.1-3` | For each incoming connection, a state machine MUST be instantiated (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an incoming connection does not get its own state machine. acceptOrReject hands the accepted connection to the peer's existing session (internal/component/bgp/reactor/reactor_connection.go:117-163), and a connection queued for collision resolution is read raw by handlePendingCollision with no FSM behind it (internal/component/bgp/reactor/reactor_connection.go:196-249). An FSM is created per session, i.e. per connection attempt of a configured peer (internal/component/bgp/reactor/session.go:396), not per inbound connection |
| `RFC4271-8.2.2-1` | If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271HoldTimerExpirySendsNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L813). **negative:** `unit/verify` [`TestRFC4271HoldTimerNotYetExpiredSendsNoNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L842). **positive:** `functional/verify` [`deadpeer-holddown.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/deadpeer-holddown.ci#L3) |
| `RFC4271-8.2.2-2` | If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271EstablishedHoldTimerExpiryRunsTheEvent10List`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L235). **positive:** `unit/verify` [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L935). **negative:** `unit/verify` [`TestRFC4271EstablishedHoldTimerFedByKeepalivesNeverExpires`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L260). **negative:** `unit/verify` [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1007) |
| `RFC4271-8.2.2-3` | If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271EstablishedHoldTimerExpiryRunsTheEvent10List`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L236). **positive:** `unit/verify` [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L938). **negative:** `unit/verify` [`TestRFC4271EstablishedHoldTimerFedByKeepalivesNeverExpires`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L261). **negative:** `unit/verify` [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1009) |
| `RFC4271-8.2.2-4` | If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271EstablishedHoldTimerExpiryRunsTheEvent10List`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L237). **positive:** `unit/verify` [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L941). **negative:** `unit/verify` [`TestRFC4271EstablishedHoldTimerFedByKeepalivesNeverExpires`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L262). **negative:** `unit/verify` [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1012) |
| `RFC4271-8.2.2-5` | If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1, - (optionally) performs peer oscillation damping if the DampPeerOscillations attribute is set to TRUE, and - changes its state to Idle. (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271EstablishedHoldTimerExpiryRunsTheEvent10List`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L238). **positive:** `unit/verify` [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L944). **negative:** `unit/verify` [`TestRFC4271EstablishedHoldTimerFedByKeepalivesNeverExpires`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L263). **negative:** `unit/verify` [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1014) |
| `RFC4271-8.2.2-6` | If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1, - (optionally) performs peer oscillation damping if the DampPeerOscillations attribute is set to TRUE (§8.2.2) | MAY | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-8.2.2-7` | In response to a ManualStart event (Event 1) or an AutomaticStart event (Event 3), the local system: - initializes all BGP resources for the peer connection, - sets ConnectRetryCounter to zero (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterZeroedOnManualStart`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L42). **positive:** `unit/verify` [`TestRFC4271ManualStartZeroesTheCounterAndDials`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L296). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterSurvivesDampedStart`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L75) |
| `RFC4271-8.2.2-20` | or an AutomaticStart event (Event 3), the local system: - initializes all BGP resources for the peer connection, - sets ConnectRetryCounter to zero (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271AutomaticStartEvent3ZeroesCounter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_automatic_start_peer_test.go#L141). **positive:** `unit/verify` [`TestRFC4271AutomaticStartZeroesConnectRetryCounter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L672). **positive:** `unit/verify` [`TestRFC4271HoldExpiryInboundShortenedBackoffZeroesCounter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_automatic_start_peer_test.go#L497). **positive:** `unit/verify` [`TestRFC4271PassiveAutomaticStartReconnectZeroesCounter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_automatic_start_peer_test.go#L518). **negative:** `unit/verify` [`TestRFC4271AutomaticStartZeroesOnlyFromIdle`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L701). **negative:** `unit/verify` [`TestRFC4271FailedCycleAfterEstablishedKeepsCounter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_automatic_start_peer_test.go#L244). **negative:** `unit/verify` [`TestRFC4271FailedCycleInboundShortenedBackoffKeepsCounter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_automatic_start_peer_test.go#L479). **negative:** `unit/verify` [`TestRFC4271FailedCycleRestartKeepsCounter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_automatic_start_peer_test.go#L191) |
| `RFC4271-8.2.2-8` | If a ManualStop event (Event 2) is issued in the OpenSent state, the local system: - sends the NOTIFICATION with a Cease, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - sets the ConnectRetryCounter to zero (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterZeroedOnManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L108). **positive:** `unit/verify` [`TestRFC4271OpenSentManualStopReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L54). **positive:** `unit/verify` [`TestShutdownNotifySendsCeaseFromEveryConnectedState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_shutdown_notify_test.go#L175). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterNotZeroedByIdleManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L130). **negative:** `unit/verify` [`TestRFC4271OpenSentWellFormedOpenKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L198). **positive:** `functional/verify` [`signal-stop-cease.ci`](https://github.com/ze-software/ze/blob/main/test/reload/signal-stop-cease.ci#L3) |
| `RFC4271-8.2.2-9` | If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnHoldTimerExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L151). **positive:** `unit/verify` [`TestRFC4271OpenSentHoldTimerExpiryReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L115). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterQuietOnHealthyEstablishedTraffic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L174). **negative:** `unit/verify` [`TestRFC4271OpenSentWellFormedOpenKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L199) |
| `RFC4271-8.2.2-10` | If the BGP message header checking (Event 21) or OPEN message checking detects an error (Event 22)(see Section 6.2), the local system: - sends a NOTIFICATION message with the appropriate error code, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnHeaderAndOpenErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L210). **positive:** `unit/verify` [`TestRFC4271OpenSentErrorReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L125). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterNotIncrementedByIdleErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L236). **negative:** `unit/verify` [`TestRFC4271OpenSentWellFormedOpenKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L197) |
| `RFC4271-8.2.2-11` | If the local system receives a TcpConnectionFails event (Event 18) from the underlying TCP or a NOTIFICATION message (Event 25), the local system: - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L262). **positive:** `unit/verify` [`TestRFC4271OpenConfirmNotificationOrTCPFailureReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L154). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterStepsByExactlyOnePerNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L293). **negative:** `unit/verify` [`TestRFC4271OpenConfirmKeepaliveKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L195) |
| `RFC4271-8.2.2-12` | If the local system receives a NOTIFICATION message (Event 24 or Event 25) or a TcpConnectionFails (Event 18) from the underlying TCP, the local system: - sets the ConnectRetryTimer to zero, - deletes all routes associated with this connection, - releases all the BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterOnVersionErrorPerState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L318). **positive:** `unit/verify` [`TestRFC4271EstablishedNotificationOrTCPFailureReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_established_teardown_peer_test.go#L85). **positive:** `unit/verify` [`TestRFC4760PeerDownDeletesOnlyThatNeighborsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4760_peer_down_test.go#L51). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterQuietOnVersionErrorInOpenStates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L342). **negative:** `unit/verify` [`TestRFC4271EstablishedGoodMessagesKeepTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_established_teardown_peer_test.go#L157) |
| `RFC4271-8.2.2-13` | If the local system receives a TcpConnectionFails event (Event 18), the local system: - restarts the ConnectRetryTimer (with the initial value), - stops and clears the DelayOpenTimer (sets the value to zero), - releases all BGP resource, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** DelayOpen is implemented and a held connection can fail in Active. internal/component/bgp/fsm/fsm.go::FSM.handleActive starts the initial-value ConnectRetryTimer and increments the counter; FSM.change preserves that retry on Active Event 18's transition to Idle and clears DelayOpen. However, internal/component/bgp/reactor/session_read.go::Session.handleConnectionCloseLocked delivers Event 18 and immediately calls timers.StopAll, cancelling the restarted retry; internal/component/bgp/reactor/peer_run.go::Peer.run substitutes its independent exponential reconnect backoff rather than the required initial-value retry. The live retry clause remains unimplemented; owned by plan/post-release/features/spec-bgp-fsm-delayopen-and-automatic-start.md AC-1e |
| `RFC4271-8.2.2-14` | If the local system receives an UPDATE message, and the UPDATE message error handling procedure (see Section 6.3) detects an error (Event 28), the local system: - sends a NOTIFICATION message with an Update error, - sets the ConnectRetryTimer to zero, - deletes all routes associated with this connection, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnUpdateError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L421). **positive:** `unit/verify` [`TestRFC4271EstablishedUpdateErrorReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_established_teardown_peer_test.go#L129). **positive:** `unit/verify` [`TestRFC4760PeerDownDeletesOnlyThatNeighborsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4760_peer_down_test.go#L55). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterQuietOnGoodUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L441). **negative:** `unit/verify` [`TestRFC4271EstablishedGoodMessagesKeepTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_established_teardown_peer_test.go#L158) |
| `RFC4271-8.2.2-15` | In response to any other events (Events 8, 10-11, 13, 19, 23, 25-28), the local system: - if the ConnectRetryTimer is running, stops and resets the ConnectRetryTimer (sets to zero), - if the DelayOpenTimer is running, stops and resets the DelayOpenTimer (sets to zero), - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ActiveAutomaticStopReleasesThePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_active_any_other_event_peer_test.go#L62). **positive:** `unit/verify` [`TestRFC4271ConnectActiveAnyOtherEventCountsAndDropsToIdle`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_any_other_event_test.go#L44). **positive:** `unit/verify` [`TestRFC4271ConnectAutomaticStopDuringTheDialReleasesThePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_connect_any_other_event_peer_test.go#L126). **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnAnyOtherEvent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L463). **negative:** `unit/verify` [`TestRFC4271ActiveDuplicateStartKeepsThePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_active_any_other_event_peer_test.go#L98). **negative:** `unit/verify` [`TestRFC4271ConnectActiveUnlistedEventKeepsTheState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_any_other_event_test.go#L78). **negative:** `unit/verify` [`TestRFC4271ConnectDuplicateStartDuringTheDialKeepsThePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_connect_any_other_event_peer_test.go#L165). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterIdleDefaultArmCountsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L498) |
| `RFC4271-8.2.2-21` | In response to any other event (Events 8, 10-11, 13, 19, 23, 25-28), the local system: - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by one (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ActiveAutomaticStopReleasesThePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_active_any_other_event_peer_test.go#L63). **negative:** `unit/verify` [`TestRFC4271ActiveDuplicateStartKeepsThePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_active_any_other_event_peer_test.go#L99) |
| `RFC4271-8.2.2-22` | In response to a ManualStop event (Event 2) initiated by the operator, the local system: - sends the NOTIFICATION message with a Cease, - releases all BGP resources, - drops the TCP connection, - sets the ConnectRetryCounter to zero, - sets the ConnectRetryTimer to zero (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271OpenConfirmManualStopReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_manualstop_peer_test.go#L29). **positive:** `unit/verify` [`TestShutdownNotifySendsCeaseFromEveryConnectedState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_shutdown_notify_test.go#L183). **negative:** `unit/verify` [`TestRFC4271OpenConfirmWithoutManualStopKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_manualstop_peer_test.go#L52) |
| `RFC4271-8.2.2-23` | In response to a ManualStop event (initiated by an operator) (Event 2), the local system: - sends the NOTIFICATION message with a Cease, - sets the ConnectRetryTimer to zero, - deletes all routes associated with this connection, - releases BGP resources, - drops the TCP connection, - sets the ConnectRetryCounter to zero (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271EstablishedManualStopReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_manualstop_peer_test.go#L83). **positive:** `unit/verify` [`TestShutdownNotifySendsCeaseFromEveryConnectedState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_shutdown_notify_test.go#L184). **negative:** `unit/verify` [`TestRFC4271EstablishedWithoutManualStopKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_manualstop_peer_test.go#L126). **positive:** `functional/verify` [`signal-stop-cease.ci`](https://github.com/ze-software/ze/blob/main/test/reload/signal-stop-cease.ci#L8) |
| `RFC4271-8.2.2-24` | In response to any other event (Events 9, 11-13, 20, 25-28), the local system: - sends the NOTIFICATION with the Error Code Finite State Machine Error, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271OpenSentUnexpectedMessageIsAnFSMError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_error_peer_test.go#L58). **negative:** `unit/verify` [`TestRFC4271ExpectedMessagesRaiseNoFSMError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_error_peer_test.go#L128). **negative:** `unit/verify` [`TestRFC4271VersionErrorNotificationReleasesQuietly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_version_error_peer_test.go#L57) |
| `RFC4271-8.2.2-25` | In response to any other event (Events 9, 12-13, 20, 27-28), the local system: - sends a NOTIFICATION with a code of Finite State Machine Error, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271OpenConfirmUpdateIsAnFSMError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_error_peer_test.go#L95). **negative:** `unit/verify` [`TestRFC4271ExpectedMessagesRaiseNoFSMError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_error_peer_test.go#L129) |
| `RFC4271-8.2.2-26` | If a NOTIFICATION message is received with a version error (Event 24), the local system: - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, and - changes its state to Idle. (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271VersionErrorNotificationReleasesQuietly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_version_error_peer_test.go#L55). **negative:** `unit/verify` [`TestRFC4271OtherOpenErrorNotificationIsNotAVersionError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_version_error_peer_test.go#L84) |
| `RFC4271-8.2.2-27` | If the local system receives a NOTIFICATION message with a version error (NotifMsgVerErr (Event 24)), the local system: - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, and - changes its state to Idle. (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271VersionErrorNotificationReleasesQuietly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_version_error_peer_test.go#L56). **negative:** `unit/verify` [`TestRFC4271OtherOpenErrorNotificationIsNotAVersionError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_version_error_peer_test.go#L85) |
| `RFC4271-8.2.2-28` | In response to any other event (Events 9, 12-13, 20-22), the local system: - sends a NOTIFICATION message with the Error Code Finite State Machine Error, - deletes all routes associated with this connection, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271EstablishedHeaderErrorReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_established_header_error_peer_test.go#L31). **negative:** `unit/verify` [`TestRFC4271EstablishedValidHeaderKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_established_header_error_peer_test.go#L77) |
| `RFC4271-8.2.2-16` | If an AutomaticStop event (Event 8) is issued in the OpenSent state, the local system: - sends the NOTIFICATION with a Cease, - sets the ConnectRetryTimer to zero, - releases all the BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnAutomaticStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L569). **positive:** `unit/verify` [`TestRFC4271OpenSentAutomaticStopReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L76). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterAutomaticStopIsNotAManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L595). **negative:** `unit/verify` [`TestRFC4271OpenSentWellFormedOpenKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L200) |
| `RFC4271-8.2.2-17` | If a connection in the OpenSent state is determined to be the connection that must be closed, an OpenCollisionDump (Event 23) is signaled to the state machine. If such an event is received in the OpenSent state, the local system: - sends a NOTIFICATION with a Cease, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC4271ConnectRetryCounterIncrementsOnOpenCollisionDump`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L626). **positive:** `unit/verify` [`TestRFC4271OpenSentCollisionDumpReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L96). **negative:** `unit/verify` [`TestRFC4271ConnectRetryCounterCollisionDumpIsQuietInIdle`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L650). **negative:** `unit/verify` [`TestRFC4271OpenSentWellFormedOpenKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L204) |
| `RFC4271-10-1` | An implementation of BGP MUST allow the HoldTimer to be configurable on a per-peer basis (§10) | MUST | 10 | **positive:** `unit/verify` [`TestRFC4271HoldTimeConfigurablePerPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L259). **positive:** `unit/verify` [`TestRFC4271HoldTimeConfiguredPerPeerFromConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_hold_time_config_test.go#L53). **negative:** `unit/verify` [`TestRFC4271HoldTimeUnconfiguredPeerKeepsTheDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_hold_time_config_test.go#L75). **negative:** `unit/verify` [`TestRFC4271PerPeerHoldTimeSurvivesNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L283) |
| `RFC4271-3-1` | To allow local policy changes to have the correct effect without resetting any BGP connections, a BGP speaker SHOULD either (a) retain the current version of the routes advertised to it by all of its peers for the duration of the connection, or (b) make use of the Route Refresh extension [RFC2918]. (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-5-7` | The sender of an UPDATE message SHOULD order path attributes within the UPDATE message in ascending order of attribute type. (§5) Context: RFC 7606 Section 5.1, which updates RFC 4271, says the MP_REACH_NLRI or MP_UNREACH_NLRI attribute SHALL be encoded as the very first path attribute; the ascending order applies to the attributes after it. | SHOULD | 5 | **positive:** `unit/verify` [`TestAnnounceBatchRail_AS4PathOrderedAgainstLargeCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_api_batch_attr_order_test.go#L335). **positive:** `unit/verify` [`TestAnnounceBatchRail_AscendingTypeCodeOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_api_batch_attr_order_test.go#L275). **positive:** `unit/verify` [`TestAnnounceQueuedRail_AscendingTypeCodeOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_api_batch_attr_order_test.go#L296). **positive:** `unit/verify` [`TestRFC4271SentAttributesAscendEvenWithRawConfigAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_attr_order_send_test.go#L24). **positive:** `unit/verify` [`TestSplitMP_PreservesAscendingAttributeOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_update_split_attr_order_test.go#L75). **negative:** `unit/verify` [`TestRFC4271SentAttributesAscendEvenWithRawConfigAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_attr_order_send_test.go#L27) |
| `RFC4271-5.1.6-2` | If an aggregate excludes at least some of the AS numbers present in the AS_PATH of the routes that are aggregated as a result of dropping the AS_SET, the aggregated route, when advertised to the peer, SHOULD include the ATOMIC_AGGREGATE attribute. (§5.1.6) | SHOULD | 5.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-5.1.6-3` | A BGP speaker that receives a route with the ATOMIC_AGGREGATE attribute SHOULD NOT remove the attribute when propagating the route to other speakers. (§5.1.6) | SHOULD NOT | 5.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-6.3-2` | If the NEXT_HOP attribute is semantically incorrect, the error SHOULD be logged, and the route SHOULD be ignored. (§6.3) | SHOULD | 6.3 | **positive:** `unit/verify` [`TestRFC4271SelfNextHopRouteIsNotInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L52). **positive:** `unit/verify` [`TestRFC4271SemanticallyIncorrectNextHopIsLoggedAndIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_nexthop_semantic_test.go#L51). **positive:** `unit/verify` [`TestRFC4271SemanticallyIncorrectNextHopIsLoggedAtTheDefaultLevel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_nexthop_semantic_test.go#L93). **negative:** `unit/verify` [`TestRFC4271SemanticallyIncorrectNextHopIsLoggedAndIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_nexthop_semantic_test.go#L52). **negative:** `unit/verify` [`TestRFC4271SemanticallyIncorrectNextHopIsLoggedAtTheDefaultLevel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_nexthop_semantic_test.go#L94) |
| `RFC4271-6.3-3` | In this case, a NOTIFICATION message SHOULD NOT be sent (§6.3) | SHOULD NOT | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.2-1` | A BGP speaker SHOULD NOT advertise a given feasible BGP route from its Adj-RIB-Out if it would produce an UPDATE message containing the same BGP route as was previously advertised. (§9.2) | SHOULD NOT | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.2.1.1-1` | Since fast convergence is needed within an autonomous system, either (a) the MinRouteAdvertisementIntervalTimer used for internal peers SHOULD be shorter than the MinRouteAdvertisementIntervalTimer used for external peers, or (b) the procedure describe in this section SHOULD NOT apply to routes sent to internal peers. (§9.2.1.1) | SHOULD NOT | 9.2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-10-2` | To minimize the likelihood that the distribution of BGP messages by a given BGP speaker will contain peaks, jitter SHOULD be applied to the timers associated with MinASOriginationIntervalTimer, KeepaliveTimer, MinRouteAdvertisementIntervalTimer, and ConnectRetryTimer. (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-5.1.1-1` | The ORIGIN attribute is generated by the speaker that originates the associated routing information. Its value SHOULD NOT be changed by any other speaker. (§5.1.1) | SHOULD | 5.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-4.2-3` | An implementation MAY reject connections on the basis of the Hold Time. (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-3.1-1` | If a BGP speaker chooses to advertise a previously received route, it MAY add to, or modify, the path attributes of the route before advertising it to a peer. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-5.1.2-1` | Whenever the modification of the AS_PATH attribute calls for including or prepending the AS number of the local system, the local system MAY include/prepend more than one instance of its own AS number in the AS_PATH attribute. (§5.1.2) | MAY | 5.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-6.7-2` | A BGP speaker MAY support the ability to impose a locally-configured, upper bound on the number of address prefixes the speaker is willing to accept from a neighbor. (§6.7) | MAY | 6.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-10-3` | MAY allow the other timers to be configurable. (§10) | MAY | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-6.2-4` | An implementation MAY reject any proposed Hold Time (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-6.7-3` | The speaker MAY also log this locally. (§6.7) | MAY | 6.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-3.1-2` | The next hop for each of these routes MUST be resolvable via the local BGP speaker's Routing Table. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the BGP Loc-RIB install performs no reachability check on the route's next hop. mirrorToLocRIB inserts the winning path with whatever next-hop address the attribute carried (internal/component/bgp/plugins/rib/rib_bestchange.go:797-830), and the candidate gather step filters only on SRv6 ineligibility (internal/component/bgp/plugins/rib/rib_commands.go:1039-1057). Resolvability is enforced downstream at FIB-install time, which removes the route from the routing table but leaves it in the Loc-RIB |
| `RFC4271-5.1.2-2` | a) When a given BGP speaker advertises the route to an internal peer, the advertising speaker SHALL NOT modify the AS_PATH attribute associated with the route. (§5.1.2) | SHALL NOT | 5.1.2 | **positive:** `unit/verify` [`TestEstablishedAnnounce_ExplicitASPath_IBGPVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_batch_test.go#L569). **positive:** `unit/verify` [`TestRFC4271ASPathUnmodifiedTowardInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L129). **positive:** `unit/verify` [`TestRFC4271ForwardASPathUnmodifiedTowardInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_a2_forward_test.go#L93). **negative:** `unit/verify` [`TestRFC4271ASPathPrependedTowardExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L146). **negative:** `unit/verify` [`TestRFC4271ExportASPathEditsFollowTheMigrationAwareInternalVerdict`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_ibgp_aspath_export_test.go#L167). **negative:** `unit/verify` [`TestRFC4271ExportPrependNeverModifiesAnInternalPeersASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_ibgp_aspath_export_test.go#L92). **negative:** `unit/verify` [`TestRFC4271ForwardASPathUnmodifiedTowardInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_a2_forward_test.go#L94). **negative:** `unit/verify` [`TestRFC4271ForwardExportASPathEditsSkipInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_ibgp_aspath_export_test.go#L312). **negative:** `unit/verify` [`TestRFC4271PolicyDryRunAgreesNoASPathEditTowardAnInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_ibgp_aspath_export_test.go#L234). **negative:** `unit/verify` [`TestRFC4271StaleReadvertiseASPathEditsSkipInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_stale_aspath_export_test.go#L77) |
| `RFC4271-5.1.2-3` | When a given BGP speaker advertises the route to an external peer, the advertising speaker updates the AS_PATH attribute as follows: 1) if the first path segment of the AS_PATH is of type AS_SEQUENCE, the local system prepends its own AS number as the last element of the sequence (put it in the leftmost position with respect to the position of octets in the protocol message). If the act of prepending will cause an overflow in the AS_PATH segment (i.e., more than 255 ASes), it SHOULD prepend a new segment of type AS_SEQUENCE and prepend its own AS number to this new segment. 2) if the first path segment of the AS_PATH is of type AS_SET, the local system prepends a new path segment of type AS_SEQUENCE to the AS_PATH, including its own AS number in that segment. 3) if the AS_PATH is empty, the local system creates a path segment of type AS_SEQUENCE, places its own AS into that segment, and places that segment into the AS_PATH. (§5.1.2) | SHALL | 5.1.2 | **positive:** `unit/verify` [`TestASPathSlotPrependOnlyWhenAdvertising`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc4271_advertise_test.go#L97). **positive:** `unit/verify` [`TestASPathSlotPrependsBeforeALeadingASSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc4271_aspath_slot_test.go#L358). **positive:** `unit/verify` [`TestASPathSlotStartsANewSegmentWhenTheLeadingOneIsFull`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc4271_aspath_slot_test.go#L392). **positive:** `unit/verify` [`TestEstablishedAnnounce_ExplicitASPath_PrependsLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_batch_test.go#L543). **positive:** `unit/verify` [`TestForwardPresentEmptyASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_aspath_empty_test.go#L23). **negative:** `unit/verify` [`TestASPathSlotPrependOnlyWhenAdvertising`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc4271_advertise_test.go#L99). **negative:** `unit/verify` [`TestEstablishedAnnounce_ExplicitASPath_IBGPVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_batch_test.go#L567). **negative:** `unit/verify` [`TestForwardWithdrawalDoesNotCreateASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_withdraw_aspath_pair_test.go#L24). **positive:** `interop/nightly` [`checkRelayWithdrawalShape`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L432). **negative:** `interop/nightly` [`checkRelayWithdrawalShape`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L433) |
| `RFC4271-5.1.4-3` | If a BGP speaker is configured to alter the value of the MULTI_EXIT_DISC attribute received over EBGP, then altering the value MUST be done prior to determining the degree of preference of the route and prior to performing route selection (Decision Process phases 1 and 2). (§5.1.4) | MUST | 5.1.4 | **positive:** `unit/verify` [`TestRFC4271MEDAlterationHappensAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L455). **positive:** `unit/verify` [`TestRFC4271ReceivedMEDAlteredBeforeTheDecisionProcess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_med_ingress_test.go#L42). **negative:** no negative test. **{single-polarity}:** the requirement constrains only the ORDER of an alteration that a speaker chooses to make, so there is no non-conformant input a receiver could reject. ze's only place to alter a received MULTI_EXIT_DISC is the ingress filter chain, whose rewritten payload replaces the WireUpdate before the UPDATE is dispatched to the RIB plugin that runs phases 1 and 2 (internal/component/bgp/reactor/reactor_notify.go:427-466) |
| `RFC4271-5.1.5-5` | A BGP speaker SHALL calculate the degree of preference for each external route based on the locally-configured policy, and include the degree of preference when advertising a route to its internal peers. (§5.1.5) | SHALL | 5.1.5 | **positive:** `unit/verify` [`TestRFC4271ExternalRouteDegreeOfPreferenceFromLocalPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L164). **positive:** `unit/verify` [`TestRFC4271ExternalRoutePreferenceFromPolicyReachesRIBAndInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_degree_of_preference_test.go#L94). **negative:** `unit/verify` [`TestRFC4271DegreeOfPreferenceNotAHardcodedConstant`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L191). **negative:** `unit/verify` [`TestRFC4271ExternalRouteWithoutPolicyStillAdvertisesAPreferenceInternally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_degree_of_preference_test.go#L126) |
| `RFC4271-5.1.7-1` | A BGP speaker that performs route aggregation MAY add the AGGREGATOR attribute, which SHALL contain its own AS number and IP address. (§5.1.7) | SHALL | 5.1.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze performs no route aggregation, so it forms no AGGREGATOR of its own. The aggregator leaf of a static route (internal/component/bgp/yang/ze-bgp-conf.yang, read in internal/component/bgp/config/bgp_routes.go) sends any AS:IP the operator writes with no check against the local AS or address, so the knob can emit an AGGREGATOR that is not the speaker's own. No owner decision declines aggregation, so this is owed with it |
| `RFC4271-6.7-4` | If the BGP speaker decides to terminate its BGP connection with a neighbor because the number of address prefixes received from the neighbor exceeds the locally-configured, upper bound, then the speaker MUST send the neighbor a NOTIFICATION message with the Error Code Cease. (§6.7) | MUST | 6.7 | **positive:** `unit/verify` [`TestPrefixExceedTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L107). **positive:** `unit/verify` [`TestRFC4271PrefixLimitTeardownSendsCease`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_prefix_limit_cease_peer_test.go#L89). **negative:** `unit/verify` [`TestPrefixExceedDrop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L144). **negative:** `unit/verify` [`TestRFC4271PrefixLimitWithoutTeardownSendsNoCease`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_prefix_limit_cease_peer_test.go#L114) |
| `RFC4271-6.8-1` | In the event of connection collision, one of the connections MUST be closed (§6.8) | MUST | 6.8 | **positive:** `unit/verify` [`TestRFC4271CollisionClosesExactlyOneConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rfc7611_ingress_test.go#L52). **negative:** `unit/verify` [`TestCollisionOpenSentNoCollision`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L167) |
| `RFC4271-6.8-2` | Upon receipt of an OPEN message, the local system MUST examine all of its connections that are in the OpenConfirm state. (§6.8) | MUST | 6.8 | **positive:** `unit/verify` [`TestCollisionOpenConfirmLocalWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L120). **positive:** `unit/verify` [`TestRFC4271ReceivedOpenExaminesTheOpenConfirmConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_collision_open_receipt_test.go#L66). **negative:** `unit/verify` [`TestCollisionNonCollisionStates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L529). **negative:** `unit/verify` [`TestRFC4271ReceivedOpenWithNoOpenConfirmConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_collision_open_receipt_test.go#L107) |
| `RFC4271-9-1` | If the UPDATE message contains a non-empty WITHDRAWN ROUTES field, the previously advertised routes, whose destinations (expressed as IP prefixes) are contained in this field, SHALL be removed from the Adj-RIB-In. (§9) | SHALL | 9 | **positive:** `unit/verify` [`TestRFC4271ReceivedWithdrawalRemovesTheRouteFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L99). **positive:** `unit/verify` [`TestRFC4271WithdrawRemovesFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L205). **negative:** `unit/verify` [`TestRFC4271ReceivedWithdrawalRemovesOnlyTheNamedRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L125). **negative:** `unit/verify` [`TestRFC4271WithdrawRemovesFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L210) |
| `RFC4271-9-2` | If the UPDATE message contains a feasible route, the Adj-RIB-In will be updated with this route as follows: if the NLRI of the new route is identical to the one the route currently has stored in the Adj- RIB-In, then the new route SHALL replace the older route in the Adj- RIB-In, thus implicitly withdrawing the older route from service. (§9) | SHALL | 9 | **positive:** `unit/verify` [`TestRFC4271ReceivedRouteWithIdenticalNLRIReplacesTheOlder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L151). **positive:** `unit/verify` [`TestRFC4271SamePrefixReplacesRatherThanAccumulates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L179). **negative:** `unit/verify` [`TestRFC4271ReplacementWithdrawsTheOlderRouteEvenWhenItWasBetter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L172). **negative:** `unit/verify` [`TestRFC4271WithdrawRemovesFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L207) |
| `RFC4271-9-3` | Once the BGP speaker updates the Adj-RIB-In, the speaker SHALL run its Decision Process. (§9) | SHALL | 9 | **positive:** `unit/verify` [`TestRFC4271AdjRIBInUpdateRunsTheDecisionProcess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L194). **negative:** `unit/verify` [`TestRFC4271AdjRIBInUpdateThatDisplacesTheBestReselects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L214) |
| `RFC4271-9.1.1-1` | The function that calculates the degree of preference for a given route SHALL NOT use any of the following as its inputs: the existence of other routes, the non-existence of other routes, or the path attributes of other routes. (§9.1) | SHALL NOT | 9.1 | **positive:** `unit/verify` [`TestRFC4271DegreeOfPreferenceIgnoresOtherRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L36). **negative:** `unit/verify` [`TestRFC4271DegreeOfPreferenceFollowsOwnAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L95) |
| `RFC4271-9.1.1-2` | the return value MUST be used as the LOCAL_PREF value in any IBGP readvertisement. (§9.1.1) | MUST | 9.1.1 | **positive:** `unit/verify` [`TestRFC4271ImportPolicyLocalPrefIsReadvertisedToInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_local_pref_readvertise_test.go#L66). **positive:** `unit/verify` [`TestRFC4271LocalPrefIncludedForInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L86). **negative:** `unit/verify` [`TestRFC4271ImportPolicyLocalPrefIsReadvertisedToInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_local_pref_readvertise_test.go#L67) |
| `RFC4271-9.1.2-1` | If the NEXT_HOP attribute of a BGP route depicts an address that is not resolvable, or if it would become unresolvable if the route was installed in the routing table, the BGP route MUST be excluded from the Phase 2 decision function. (§9.1.2) | MUST | 9.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an unresolvable NEXT_HOP does not exclude the route from Phase 2. gatherCandidatesLocked excludes only SRv6-ineligible entries and next hops naming one of ze's own session addresses (internal/component/bgp/plugins/rib/rib_commands.go), and extractCandidate uses the next hop solely to look up an interior cost, giving an unresolvable one the maximum (internal/component/bgp/plugins/rib/rib_commands.go `extractCandidate`). The route therefore still competes: it loses Section 9.1.2.2 step e) to any resolvable path, but wins outright when it is preferred at an earlier step (LOCAL_PREF, AIGP, AS_PATH, ORIGIN, MULTI_EXIT_DISC, EBGP over IBGP) or is the only path |
| `RFC4271-9.1.2-2` | The local speaker SHALL then install that route in the Loc-RIB, replacing any route to the same destination that is currently being held in the Loc-RIB. (§9.1.2) | SHALL | 9.1.2 | **positive:** `unit/verify` [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_bestchange_test.go#L1611). **positive:** `unit/verify` [`TestRFC4271SelectedRouteReplacesTheLocRIBRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L239). **negative:** `unit/verify` [`TestRFC4271LocRIBHoldsOnlyTheReplacingRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L263) |
| `RFC4271-9.1.2-3` | The local speaker MUST determine the immediate next-hop address from the NEXT_HOP attribute (§9.1.2) | MUST | 9.1.2 | **positive:** `unit/verify` [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_bestchange_test.go#L1613). **negative:** `unit/verify` [`TestRFC4271LocRIBNextHopComesFromNextHopAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L121) |
| `RFC4271-9.1.2-4` | If either the immediate next-hop or the IGP cost to the NEXT_HOP (where the NEXT_HOP is resolved through an IGP route) changes, Phase 2 Route Selection MUST be performed again. (§9.1.2) | MUST | 9.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nothing re-runs Phase 2 when the immediate next-hop or the IGP cost to the NEXT_HOP changes. The only entry points to checkBestPathChange are the UPDATE ingest path and the peer-state paths (internal/component/bgp/plugins/rib/rib_structured.go:271-286), and the IGP cost function is a passive lookup registered once with no invalidation callback (internal/component/bgp/plugins/rib/bestpath.go:30-43) |
| `RFC4271-9.1.2.1-1` | Notice that even though BGP routes do not have to be installed in the Routing Table with the immediate next-hop(s), implementations MUST take care that, before any packets are forwarded along a BGP route, its associated NEXT_HOP address is resolved to the immediate (directly connected) next-hop address, and that this address (or multiple addresses) is finally used for actual packet forwarding. (§9.1.2) | MUST | 9.1.2 | **positive:** `unit/verify` [`TestRFC4271BGPNextHopResolvedToTheImmediateNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc4271_nexthop_resolution_test.go#L107). **negative:** `unit/verify` [`TestRFC4271BGPRouteWithAnUnresolvedNextHopLeavesTheFIB`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc4271_nexthop_resolution_test.go#L145) |
| `RFC4271-9.1.2.1-2` | Unresolvable routes SHALL be removed from the Loc-RIB and the routing table. (§9.1.2) | SHALL | 9.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an unresolvable route is not removed from the Loc-RIB. The only Loc-RIB removal in the BGP plugin is the no-candidate-remains branch of checkBestPathChange (internal/component/bgp/plugins/rib/rib_bestchange.go:766-782), which is driven by the Adj-RIB-In losing its last path and never by next-hop resolvability; nothing in the plugin consults a resolver |
| `RFC4271-9.1.2.2-1` | The criteria MUST be applied in the order specified. (§9.1.2.2) | MUST | 9.1.2.2 | **positive:** `unit/verify` [`TestBestPathStepFComparesThePeerBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_test.go#L88). **positive:** `unit/verify` [`TestBestPathStepFComparesThePeerBGPIdentifierOnTheJSONRail`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_json_test.go#L79). **positive:** `unit/verify` [`TestBestPath_FullTiebreak`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L540). **positive:** `unit/verify` [`TestRFC4271AdjacentCriteriaApplyInTheOrderSpecified`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_criteria_order_test.go#L44). **positive:** `unit/verify` [`TestRFC4271WholeSetMEDBeforeLaterCriteria`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_whole_set_med_test.go#L44). **positive:** `unit/verify` [`TestRFC4271WholeSetMEDRIBBestChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_whole_set_med_test.go#L188). **negative:** `unit/verify` [`TestBestPathEqualBGPIdentifiersFallThroughToPeerAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_test.go#L116). **negative:** `unit/verify` [`TestBestPathEqualBGPIdentifiersOnTheJSONRailFallThroughToPeerAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_json_test.go#L107). **negative:** `unit/verify` [`TestBestPath_MED_SameNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L325). **negative:** `unit/verify` [`TestRFC4271WholeSetMEDEarlierCriteria`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_whole_set_med_test.go#L104) |
| `RFC4271-9.1.2.2-2` | If an implementation chooses to remove MULTI_EXIT_DISC, then the optional comparison on MULTI_EXIT_DISC, if performed, MUST be performed only among EBGP-learned routes. (§9.1.2.2) | MUST | 9.1.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the common egress guard is implemented, but normal BGP selected-route readvertisement has no runnable producer and no discriminating proof. `bgp-rib` records selected Loc-RIB state (internal/component/bgp/plugins/rib/rib_bestchange.go:738-790), route-server and route-reflector plugins forward cached UPDATEs instead (internal/component/bgp/plugins/rs/server.go:433-434 and internal/component/bgp/plugins/rr/rr.go:188-195), and BGP-to-BGP redistribution is rejected as same-protocol redistribution (internal/core/redistevents/registry.go:144-146) |
| `RFC4271-9.1.2.2-3` | For IBGP- learned routes, the MULTI_EXIT_DISC MUST be used in route comparisons that reach this step in the Decision Process. (§9.1.2.2) | MUST | 9.1.2.2 | **positive:** `unit/verify` [`TestBestPath_MED_SameNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L329). **positive:** `unit/verify` [`TestRFC4271IBGPAggregatesLedByASSetsCompareMED`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_ibgp_aggregate_med_test.go#L63). **positive:** `unit/verify` [`TestRFC4271IBGPLocallyOriginatedRoutesCompareMED`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_ibgp_med_test.go#L42). **negative:** `unit/verify` [`TestBestPath_MED_SameNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L332). **negative:** `unit/verify` [`TestRFC4271IBGPMEDIsNotSkippedWhenLaterStepsDisagree`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_ibgp_med_test.go#L60). **negative:** `unit/verify` [`TestRFC4271IBGPRouteLedByASSequenceKeepsItsNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_ibgp_aggregate_med_test.go#L83) |
| `RFC4271-9.1.2.2-4` | Routes that do not have the MULTI_EXIT_DISC attribute are considered to have the lowest possible MULTI_EXIT_DISC value (§9.1.2.2) | MUST | 9.1.2.2 | **positive:** `unit/verify` [`TestAbsentMedStillComparesAsZeroInPhaseTwo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L248). **negative:** `unit/verify` [`TestAbsentMedTiesAnExplicitMedOfZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L280) |
| `RFC4271-9.2-2` | A route SHALL NOT be installed in the Adj-Rib-Out unless the destination, and NEXT_HOP described by this route, may be forwarded appropriately by the Routing Table. (§9.1.3) | SHALL NOT | 9.1.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nothing gates Adj-RIB-Out installation on the destination and NEXT_HOP being forwardable. QueueAnnounce records the route unconditionally (internal/component/bgp/rib/outgoing.go:65-101), and the forwarding rails decide only on filters, family negotiation and the route-reflection rules (internal/component/bgp/reactor/forward_rs.go:295-333) |
| `RFC4271-9.2-3` | If a route in Loc-RIB is excluded from a particular Adj-RIB-Out, the previously advertised route in that Adj-RIB-Out MUST be withdrawn from service by means of an UPDATE message (§9.1.3) | MUST | 9.1.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a route excluded from a peer's Adj-RIB-Out by an egress filter is skipped silently, leaving the peer's previous advertisement in place instead of withdrawing it. Both forwarding rails `continue` on suppression with no withdrawal built (internal/component/bgp/reactor/forward_rs.go:320-333 and internal/component/bgp/reactor/reactor_api_forward.go:496-506); the one announce-to-withdraw conversion is LLGR-specific and filter-requested, not exclusion-driven (internal/component/bgp/reactor/reactor_api_forward.go:588-601) |
| `RFC4271-9.2-4` | If a BGP speaker receives overlapping routes, the Decision Process MUST consider both routes based on the configured acceptance policy. (§9.1.4) | MUST | 9.1.4 | **positive:** `unit/verify` [`TestRFC4271OverlappingReceivedRoutesAreBothInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L291). **positive:** `unit/verify` [`TestRFC4271OverlappingRoutesAcceptedByPolicyBothReachTheDecisionProcess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_overlap_acceptance_test.go#L99). **negative:** `unit/verify` [`TestRFC4271OverlappingRouteArrivalDisplacesNeither`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L318). **negative:** `unit/verify` [`TestRFC4271OverlappingRouteRejectedByPolicyIsNotConsidered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_overlap_acceptance_test.go#L122) |
| `RFC4271-9.2-5` | If both a less and a more specific route are accepted, then the Decision Process MUST install, in Loc-RIB, either both the less and the more specific routes or aggregate the two routes and install, in Loc-RIB, the aggregated route, provided that both routes have the same value of the NEXT_HOP attribute. (§9.1.4) | MUST | 9.1.4 | **positive:** `unit/verify` [`TestRFC4271OverlappingReceivedRoutesAreBothInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L294). **negative:** `unit/verify` [`TestRFC4271OverlappingRouteArrivalDisplacesNeither`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L320) |
| `RFC4271-9.2.1.1-2` | Two UPDATE messages sent by a BGP speaker to a peer that advertise feasible routes and/or withdrawal of unfeasible routes to some common set of destinations MUST be separated by at least MinRouteAdvertisementIntervalTimer. (§9.2.1.1) | MUST | 9.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no MinRouteAdvertisementIntervalTimer, so successive UPDATEs to a common set of destinations are not spaced. The timer set implements only ConnectRetry, Hold and Keepalive and records the omission in its own doc comment (internal/component/bgp/fsm/timer.go:34-42, "MinRouteAdvertisementIntervalTimer (Section 9.2.1.1) - not implemented here"); `grep -rniE 'minroute\|mrai' --include=*.go internal/` finds no producer |
| `RFC4271-9.2.2.2-1` | Routes that have different MULTI_EXIT_DISC attributes SHALL NOT be aggregated. (§9.2.2.2) | SHALL NOT | 9.2.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no route aggregation (Section 9.2.2.2): no code forms an aggregate from contributing routes, and internal/component/bgp/message/update_build_grouped.go::BuildGroupedUnicast packing several prefixes into one UPDATE is not aggregation. An operator can configure a covering prefix on a static route with any ORIGIN, NEXT_HOP, ATOMIC_AGGREGATE and AGGREGATOR (internal/component/bgp/config/bgp_routes.go), and nothing relates it to the routes it covers. No owner decision declines aggregation, so this is owed with it |
| `RFC4271-9.2.2.2-2` | ORIGIN attribute: If at least one route among routes that are aggregated has ORIGIN with the value INCOMPLETE, then the aggregated route MUST have the ORIGIN attribute with the value INCOMPLETE. Otherwise, if at least one route among routes that are aggregated has ORIGIN with the value EGP, then the aggregated route MUST have the ORIGIN attribute with the value EGP. (§9.2.2.2) | MUST | 9.2.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no route aggregation (Section 9.2.2.2): no code forms an aggregate from contributing routes, and internal/component/bgp/message/update_build_grouped.go::BuildGroupedUnicast packing several prefixes into one UPDATE is not aggregation. An operator can configure a covering prefix on a static route with any ORIGIN, NEXT_HOP, ATOMIC_AGGREGATE and AGGREGATOR (internal/component/bgp/config/bgp_routes.go), and nothing relates it to the routes it covers. No owner decision declines aggregation, so this is owed with it |
| `RFC4271-9.2.2.2-3` | NEXT_HOP: When aggregating routes that have different NEXT_HOP attributes, the NEXT_HOP attribute of the aggregated route SHALL identify an interface on the BGP speaker that performs the aggregation. (§9.2.2.2) | SHALL | 9.2.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no route aggregation (Section 9.2.2.2): no code forms an aggregate from contributing routes, and internal/component/bgp/message/update_build_grouped.go::BuildGroupedUnicast packing several prefixes into one UPDATE is not aggregation. An operator can configure a covering prefix on a static route with any ORIGIN, NEXT_HOP, ATOMIC_AGGREGATE and AGGREGATOR (internal/component/bgp/config/bgp_routes.go), and nothing relates it to the routes it covers. No owner decision declines aggregation, so this is owed with it |
| `RFC4271-9.2.2.2-4` | ATOMIC_AGGREGATE: If at least one of the routes to be aggregated has ATOMIC_AGGREGATE path attribute, then the aggregated route SHALL have this attribute as well. (§9.2.2.2) | SHALL | 9.2.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no route aggregation (Section 9.2.2.2): no code forms an aggregate from contributing routes, and internal/component/bgp/message/update_build_grouped.go::BuildGroupedUnicast packing several prefixes into one UPDATE is not aggregation. An operator can configure a covering prefix on a static route with any ORIGIN, NEXT_HOP, ATOMIC_AGGREGATE and AGGREGATOR (internal/component/bgp/config/bgp_routes.go), and nothing relates it to the routes it covers. No owner decision declines aggregation, so this is owed with it |
| `RFC4271-9.2.2.2-5` | AGGREGATOR: Any AGGREGATOR attributes from the routes to be aggregated MUST NOT be included in the aggregated route. (§9.2.2.2) | MUST NOT | 9.2.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no route aggregation (Section 9.2.2.2): no code forms an aggregate from contributing routes, and internal/component/bgp/message/update_build_grouped.go::BuildGroupedUnicast packing several prefixes into one UPDATE is not aggregation. An operator can configure a covering prefix on a static route with any ORIGIN, NEXT_HOP, ATOMIC_AGGREGATE and AGGREGATOR (internal/component/bgp/config/bgp_routes.go), and nothing relates it to the routes it covers. No owner decision declines aggregation, so this is owed with it |
| `RFC4271-Security-1` | An implementation MUST support the TCP MD5 option [RFC2385]. (§E) | MUST | E | **positive:** `unit/verify` [`TestMD5PeersForListener`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L2357). **positive:** `unit/verify` [`TestRFC2385ConfiguredKeyReachesBothSockets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2385_test.go#L44). **positive:** `unit/verify` [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L86). **negative:** `unit/verify` [`TestMD5PeersForListener`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L2360). **negative:** `unit/verify` [`TestRFC2385KeyOnOneEndOnlyCarriesNoSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L180) |
| `RFC4271-9.2.1.1-3` | If new routes are selected multiple times while awaiting the expiration of MinRouteAdvertisementIntervalTimer, the last route selected SHALL be advertised at the end of MinRouteAdvertisementIntervalTimer. (§9.2.1.1) | SHALL | 9.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** with no MinRouteAdvertisementIntervalTimer there is no expiry at which a last-selected route could be advertised. The timer is absent by design note (internal/component/bgp/fsm/timer.go:39) and no producer buffers a pending best-route advertisement against such a timer; best-path changes are published as they are computed (internal/component/bgp/plugins/rib/rib_bestchange.go:832-880) |
| `RFC4271-9.2-6` | When a BGP speaker receives an UPDATE message from an internal peer, the receiving BGP speaker SHALL NOT re-distribute the routing information contained in that UPDATE message to other internal peers (unless the speaker acts as a BGP Route Reflector [RFC2796]). (§9.2) | SHALL NOT | 9.2 | **positive:** `unit/verify` [`TestRFC4271NoIBGPToIBGPRedistribution`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L514). **negative:** `unit/verify` [`TestRFC4271ForwardRailNeverRedistributesInternalRoutesToInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_ibgp_split_horizon_test.go#L35). **negative:** `unit/verify` [`TestRFC4271IBGPRedistributionAllowedForReflectorClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L573) |
| `RFC4271-9.2-7` | All newly installed routes and all newly unfeasible routes for which there is no replacement route SHALL be advertised to its peers by means of an UPDATE message. (§9.2) | SHALL | 9.2 | **positive:** `unit/verify` [`TestRFC4271RouteServerLastSourceWithdrawalIsSent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rs_replacement_test.go#L88). **negative:** `unit/verify` [`TestRFC4271RouteServerOwnerWithdrawalSendsReplacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rs_replacement_test.go#L42) |
| `RFC4271-9.2-8` | Any routes in the Loc-RIB marked as unfeasible SHALL be removed (§9.2) | SHALL | 9.2 | **positive:** `unit/verify` [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_bestchange_test.go#L1616). **negative:** `unit/verify` [`TestRFC4271RouteServerStillFeasibleRouteIsNotWithdrawn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rs_replacement_test.go#L180). **negative:** `unit/verify` [`TestRFC4271RouteServerStillFeasibleRouteStaysInLocRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rs_replacement_test.go#L215) |
| `RFC4271-9.2-9` | Changes to the reachable destinations within its own autonomous system SHALL also be advertised in an UPDATE message. (§9.2) | SHALL | 9.2 | **positive:** `unit/verify` [`TestRFC4271OwnASDestinationBecomingReachableIsSentToPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_own_as_change_test.go#L102). **positive:** `unit/verify` [`TestRFC4271OwnASReachabilityChangeAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L401). **negative:** `unit/verify` [`TestRFC4271OwnASDestinationBecomingUnreachableIsSentToPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_own_as_change_test.go#L128). **negative:** `unit/verify` [`TestRFC4271OwnASUnreachabilityChangeAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L429) |
| `RFC4271-9.2-10` | If, due to the limits on the maximum size of an UPDATE message (see Section 4), a single route doesn't fit into the message, the BGP speaker MUST not advertise the route to its peers and MAY choose to log an error locally. (§9.2) | MUST | 9.2 | **positive:** `unit/verify` [`TestRFC4271OversizeSingleRouteNotAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L403). **positive:** `unit/verify` [`TestRFC4271RelayedRouteTooLargeForThePeerIsWithheld`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_oversize_route_test.go#L38). **negative:** `unit/verify` [`TestRFC4271FittingRouteIsAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L432). **negative:** `unit/verify` [`TestRFC4271RelayedRouteTooLargeForThePeerIsWithheld`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_oversize_route_test.go#L40) |
| `RFC4271-Appendix-1` | If a local system TCP user interface supports the TCP PUSH function, then each BGP message SHOULD be transmitted with PUSH flag set. (§E) | SHOULD | E | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-Appendix-2` | If a local system TCP user interface supports setting the DSCP field [RFC2474] for TCP connections, then the TCP connection used by BGP SHOULD be opened with bits 0-2 of the DSCP field set to 110 (binary). (§E) | SHOULD | E | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.3-1` | an AS SHOULD avoid using unstable routes (§9.3) | SHOULD | 9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.3-2` | Thus, an AS SHOULD avoid using unstable routes, and it SHOULD NOT make rapid, spontaneous changes to its choice of route. (§9.3) | SHOULD NOT | 9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.4-1` | The decision of whether to distribute non-BGP acquired routes within an AS via BGP depends on the environment within the AS (e.g., type of IGP) and SHOULD be controlled via configuration. (§9.4) | SHOULD | 9.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.1.2-5` | If the AS_PATH attribute of a BGP route contains an AS loop, the BGP route should be excluded from the Phase 2 decision function (§9.1.2). Detection scans the full AS path and checks that the local autonomous system number does not appear in it. RFC 4271 writes this keyword in lower case, so the level is a recommendation and not a capitalized RFC 2119 SHOULD. The same paragraph places a speaker configured to accept routes with its own autonomous system number in the AS path outside the scope of the document. That out-of-scope case is what the allow-own-as setting selects, so a non-zero allow-own-as is not a deviation from this line. | SHOULD | 9.1.2 | **positive:** `unit/verify` [`TestDetectASLoop_NotPresent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/loopdetect/rfc4271_loop_test.go#L136). **negative:** `unit/verify` [`TestDetectASLoop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/loopdetect/rfc4271_loop_test.go#L114). **negative:** `unit/verify` [`TestDetectASLoop_ASSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/loopdetect/rfc4271_loop_test.go#L125). **negative:** `functional/verify` [`loop-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/loop-as.ci#L7) |
| `RFC4271-9.1.2.1-3` | However, corresponding unresolvable routes SHOULD be kept in the Adj-RIBs-In (in case they become resolvable). (§9.1.2) | SHOULD | 9.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.1.2.1-4` | If multiple matching routes are available, only the longest matching route SHOULD be considered. (§9.1.2.1) | SHOULD | 9.1.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.1.2.1-5` | Whenever a BGP speaker identifies a route that fails the resolvability check because of mutual recursion, an error message SHOULD be logged. (§9.1.2.1) | SHOULD | 9.1.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-8.2.1.4-1` | After the connection collision is resolved (see Section 6.8), the FSM for the connection that is closed SHOULD be disposed. (§8.2.1.2) | SHOULD | 8.2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-4.3-6` | An UPDATE message SHOULD NOT include the same address prefix in the WITHDRAWN ROUTES and Network Layer Reachability Information fields. (§4.3) | SHOULD NOT | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-4.3-7` | A BGP speaker SHOULD treat an UPDATE message of this form as though the WITHDRAWN ROUTES do not contain the address prefix. (§4.3) | SHOULD | 4.3 | **positive:** `unit/verify` [`TestRIBInjectSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L121). **positive:** `unit/verify` [`TestRIBPoolPathSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L87). **positive:** `unit/verify` [`TestRIBSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L42). **negative:** `unit/verify` [`TestRFC4271MixedUpdateStillAppliesItsOtherWithdrawals`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L354) |
| `RFC4271-5.1.7-2` | The IP address SHOULD be the same as the BGP Identifier of the speaker. (§5.1.7) | SHOULD | 5.1.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.2-11` | If a BGP speaker chooses to aggregate, then it SHOULD either include all ASes used to form the aggregate in an AS_SET, or add the ATOMIC_AGGREGATE attribute to the route. (§9.1.4) | SHOULD | 9.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.2-12` | Routes SHOULD NOT be de-aggregated. (§9.1.4) | SHOULD NOT | 9.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-9.2.2.2-6` | If the aggregated route has an AS_SET as the first element in its AS_PATH attribute, then the router that originates the route SHOULD NOT advertise the MULTI_EXIT_DISC attribute with this route. (§9.2.2.2) | SHOULD NOT | 9.2.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4271-5-8` | Once a BGP peer has updated any well-known attributes, it MUST pass these attributes to its peers in any updates it transmits (§5) | MUST | 5 | **positive:** `unit/verify` [`TestForwardTransmitsUpdatedWellKnownAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_section5_test.go#L145). **negative:** `unit/verify` [`TestForwardNeverTransmitsTheSupersededWellKnownAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_section5_test.go#L166) |
| `RFC4271-6.2-5` | If the version number in the Version field of the received OPEN message is not supported, then the Error Subcode MUST be set to Unsupported Version Number (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_open_error_test.go#L64). **negative:** `unit/verify` [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_open_error_test.go#L66) |
| `RFC4271-6.2-6` | If the Autonomous System field of the OPEN message is unacceptable, then the Error Subcode MUST be set to Bad Peer AS (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_open_error_test.go#L67). **negative:** `unit/verify` [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_open_error_test.go#L69) |
| `RFC4271-6.2-7` | If the Hold Time field of the OPEN message is unacceptable, then the Error Subcode MUST be set to Unacceptable Hold Time (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_open_error_test.go#L71). **negative:** `unit/verify` [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_open_error_test.go#L73) |
| `RFC4271-6.2-8` | If the BGP Identifier field of the OPEN message is syntactically incorrect, then the Error Subcode MUST be set to Bad BGP Identifier (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_open_error_test.go#L74). **negative:** `unit/verify` [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_open_error_test.go#L76) |
| `RFC4271-6.2-9` | If one of the Optional Parameters in the OPEN message is not recognized, then the Error Subcode MUST be set to Unsupported Optional Parameters (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestSessionRFC4271OptionalParameterErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L36). **negative:** `unit/verify` [`TestSessionRFC4271OptionalParameterErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L37) |
| `RFC4271-6.2-10` | If one of the Optional Parameters in the OPEN message is recognized, but is malformed, then the Error Subcode MUST be set to 0 (Unspecific) (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestSessionRFC4271OptionalParameterErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L38). **negative:** `unit/verify` [`TestSessionRFC4271OptionalParameterErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L39) |
| `RFC4271-6.3-4` | If the Withdrawn Routes Length or Total Attribute Length is too large (i.e., if Withdrawn Routes Length + Total Attribute Length + 23 exceeds the message Length), then the Error Subcode MUST be set to Malformed Attribute List. (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC4271UpdateMalformedAttributeList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_update_error_test.go#L54). **negative:** `unit/verify` [`TestRFC4271UpdateMalformedAttributeList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_update_error_test.go#L57) |
| `RFC4271-6.3-5` | If any recognized attribute has Attribute Flags that conflict with the Attribute Type Code, then the Error Subcode MUST be set to Attribute Flags Error. The Data field MUST contain the erroneous attribute (type, length, and value). (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC4271RecognizedAttributeErrorsBeyondTheFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_recognized_attribute_errors_test.go#L52). **positive:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L102). **negative:** `unit/verify` [`TestRFC4271RecognizedAttributeErrorsBeyondTheFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_recognized_attribute_errors_test.go#L53). **negative:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L103) |
| `RFC4271-6.3-6` | If any recognized attribute has an Attribute Length that conflicts with the expected length (based on the attribute type code), then the Error Subcode MUST be set to Attribute Length Error. The Data field MUST contain the erroneous attribute (type, length, and value). (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC4271LocalPrefLengthFromInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_a_local_pref_length_test.go#L31). **positive:** `unit/verify` [`TestRFC4271RecognizedAttributeErrorsBeyondTheFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_recognized_attribute_errors_test.go#L54). **positive:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L104). **negative:** `unit/verify` [`TestRFC4271LocalPrefLengthFromInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_a_local_pref_length_test.go#L32). **negative:** `unit/verify` [`TestRFC4271RecognizedAttributeErrorsBeyondTheFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_recognized_attribute_errors_test.go#L55). **negative:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L105) |
| `RFC4271-6.3-7` | If any of the well-known mandatory attributes are not present, then the Error Subcode MUST be set to Missing Well-known Attribute. The Data field MUST contain the Attribute Type Code of the missing, well-known attribute. (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC4271MandatoryAttributesAcrossUpdateForms`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_mandatory_test.go#L10). **positive:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L106). **negative:** `unit/verify` [`TestRFC4271MandatoryAttributesAcrossUpdateForms`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_mandatory_test.go#L11). **negative:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L107) |
| `RFC4271-6.3-8` | If any of the well-known mandatory attributes are not recognized, then the Error Subcode MUST be set to Unrecognized Well-known Attribute. The Data field MUST contain the unrecognized attribute (type, length, and value). (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestSessionRFC4271RetainedUpdateNotifications`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L156). **negative:** `unit/verify` [`TestSessionRFC4271RetainedUpdateNotifications`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L157) |
| `RFC4271-6.3-9` | If the ORIGIN attribute has an undefined value, then the Error Sub- code MUST be set to Invalid Origin Attribute. The Data field MUST contain the unrecognized attribute (type, length, and value). (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L108). **negative:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L109) |
| `RFC4271-6.3-10` | If the NEXT_HOP attribute field is syntactically incorrect, then the Error Subcode MUST be set to Invalid NEXT_HOP Attribute. The Data field MUST contain the incorrect attribute (type, length, and value). (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L110). **negative:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L111) |
| `RFC4271-6.3-11` | The IP address in the NEXT_HOP MUST meet the following criteria to be considered semantically correct: a) It MUST NOT be the IP address of the receiving speaker. b) In the case of an EBGP, where the sender and receiver are one IP hop away from each other, either the IP address in the NEXT_HOP MUST be the sender's IP address that is used to establish the BGP connection, or the interface associated with the NEXT_HOP IP address MUST share a common subnet with the receiving BGP speaker. (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestSessionRFC4271IBGPNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_next_hop_test.go#L62). **positive:** `unit/verify` [`TestSessionRFC4271NextHopMixedUpdateAndAddressChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_next_hop_test.go#L15). **positive:** `unit/verify` [`TestSessionRFC4271NextHopSemantics`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L206). **negative:** `unit/verify` [`TestSessionRFC4271IBGPNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_next_hop_test.go#L63). **negative:** `unit/verify` [`TestSessionRFC4271NextHopMixedUpdateAndAddressChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_next_hop_test.go#L16). **negative:** `unit/verify` [`TestSessionRFC4271NextHopSemantics`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L207) |
| `RFC4271-6.3-12` | If the path is syntactically incorrect, then the Error Subcode MUST be set to Malformed AS_PATH. (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L112). **negative:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L113) |
| `RFC4271-6.3-13` | If the UPDATE message is received from an external peer, the local system MAY check whether the leftmost (with respect to the position of octets in the protocol message) AS in the AS_PATH attribute is equal to the autonomous system number of the peer that sent the message. If the check determines this is not the case, the Error Subcode MUST be set to Malformed AS_PATH. (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC4271LeftmostASMismatchIsMalformedASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rfc7611_ingress_test.go#L35). **negative:** `unit/verify` [`TestRFC4271LeftmostASMismatchIsMalformedASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rfc7611_ingress_test.go#L38) |
| `RFC4271-6.3-14` | If an optional attribute is recognized, then the value of this attribute MUST be checked. If an error is detected, the attribute MUST be discarded, and the Error Subcode MUST be set to Optional Attribute Error. The Data field MUST contain the attribute (type, length, and value). (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC4271RecognizedAttributeErrorsBeyondTheFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_recognized_attribute_errors_test.go#L56). **positive:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L114). **negative:** `unit/verify` [`TestRFC4271RecognizedAttributeErrorsBeyondTheFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_recognized_attribute_errors_test.go#L57). **negative:** `unit/verify` [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L115) |
| `RFC4271-6.3-15` | If any attribute appears more than once in the UPDATE message, then the Error Subcode MUST be set to Malformed Attribute List (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC4271UpdateMalformedAttributeList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_update_error_test.go#L59). **positive:** `unit/verify` [`TestSessionRFC7606DuplicateMPUnreachNotificationOnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_dupmp_unreach_wire_test.go#L31). **negative:** `unit/verify` [`TestRFC4271UpdateMalformedAttributeList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_update_error_test.go#L61) |
| `RFC4271-6.3-16` | If the field is syntactically incorrect, then the Error Subcode MUST be set to Invalid Network Field. (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestSessionRFC4271RetainedUpdateNotifications`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L158). **negative:** `unit/verify` [`TestSessionRFC4271RetainedUpdateNotifications`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L159) |
| `RFC4271-6.3-17` | An UPDATE message that contains correct path attributes, but no NLRI, SHALL be treated as a valid UPDATE message (§6.3) | SHALL | 6.3 | **positive:** `unit/verify` [`TestRFC4271UpdateWithoutNLRIIsValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_update_error_test.go#L125). **negative:** `unit/verify` [`TestRFC4271UpdateWithoutNLRIIsValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_update_error_test.go#L128) |
| `RFC4271-8.2.2-19` | In response to an indication that the TCP connection is successfully established (Event 16 or Event 17), the second connection SHALL be tracked until it sends an OPEN message (§8.2.2) | SHALL | 8.2.2 | **positive:** `unit/verify` [`TestSessionRFC4271EstablishedCollisionWaitsForOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L247). **negative:** `unit/verify` [`TestSessionRFC4271EstablishedCollisionWaitsForOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L248) |
| `RFC4271-9-4` | Otherwise, if the Adj-RIB-In has no route with NLRI identical to the new route, the new route SHALL be placed in the Adj-RIB-In. (§9) | SHALL | 9 | **positive:** `unit/verify` [`TestRFC4271AdjRIBInPlacesNewRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rfc4271_rib_test.go#L41). **negative:** `unit/verify` [`TestRFC4271AdjRIBInPlacesNewRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rfc4271_rib_test.go#L44) |
| `RFC4271-10-4` | The suggested default amount of jitter SHALL be determined by multiplying the base value of the appropriate timer by a random factor, which is uniformly distributed in the range from 0.75 to 1.0 (§10) | SHALL | 10 | **positive:** `unit/verify` [`TestTimersRFC4271Jitter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_timer_jitter_test.go#L11). **negative:** `unit/verify` [`TestTimersRFC4271Jitter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_timer_jitter_test.go#L12) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4271-5.1.6-1`](#rfc4271-5.1.6-1) A BGP speaker that receives a route with the ATOMIC_AGGREGATE attribute MUST NOT make any NLRI of that route more specific (as defined in 9.1.4) when advertising this route to other BGP speakers. (§5.1.6) | {gap}, no test | gap in proof: ze meets it, because readvertisement writes the route's stored NLRI bytes unchanged (internal/component/bgp/reactor/peer_rib_routes.go::buildRIBRouteUpdate) and no code makes a received prefix more specific or splits it; no test asserts it |
| [`RFC4271-6.2-3`](#rfc4271-6.2-3) All errors detected while processing the OPEN message MUST be indicated by sending the NOTIFICATION message with the Error Code OPEN Message Error. (§6.2) | {gap}, no test | one class of OPEN error is detected and never reported. UnpackOpen returns the bare sentinel ErrShortRead when the body is under 10 octets or when the Optional Parameters Length (standard or RFC 9072 extended) overruns the body (internal/component/bgp/message/open.go:167-168, :193-194, :199-200, :209-210), and handleOpen turns that into an FSM event and a returned error, writing no NOTIFICATION and not even closing the connection (internal/component/bgp/reactor/session_handlers.go:43-47); session_read.go:264 only propagates it. Every other OPEN error path does send Error Code 2 -- unsupported version (session_handlers.go:54-60), unacceptable Hold Time (:70-77) and a malformed capability (rejectOpenCapabilityError, :185-199) -- so the obligation holds everywhere except the decode failure. Disclosed in docs/features/rfc-status.md RFC 4271 row |
| [`RFC4271-8.2.1-3`](#rfc4271-8.2.1-3) For each incoming connection, a state machine MUST be instantiated (§8.2.1) | {gap}, no test | an incoming connection does not get its own state machine. acceptOrReject hands the accepted connection to the peer's existing session (internal/component/bgp/reactor/reactor_connection.go:117-163), and a connection queued for collision resolution is read raw by handlePendingCollision with no FSM behind it (internal/component/bgp/reactor/reactor_connection.go:196-249). An FSM is created per session, i.e. per connection attempt of a configured peer (internal/component/bgp/reactor/session.go:396), not per inbound connection |
| [`RFC4271-8.2.2-13`](#rfc4271-8.2.2-13) If the local system receives a TcpConnectionFails event (Event 18), the local system: - restarts the ConnectRetryTimer (with the initial value), - stops and clears the DelayOpenTimer (sets the value to zero), - releases all BGP resource, - increments the ConnectRetryCounter by 1 (§8.2.2) | {gap}, no test | DelayOpen is implemented and a held connection can fail in Active. internal/component/bgp/fsm/fsm.go::FSM.handleActive starts the initial-value ConnectRetryTimer and increments the counter; FSM.change preserves that retry on Active Event 18's transition to Idle and clears DelayOpen. However, internal/component/bgp/reactor/session_read.go::Session.handleConnectionCloseLocked delivers Event 18 and immediately calls timers.StopAll, cancelling the restarted retry; internal/component/bgp/reactor/peer_run.go::Peer.run substitutes its independent exponential reconnect backoff rather than the required initial-value retry. The live retry clause remains unimplemented; owned by plan/post-release/features/spec-bgp-fsm-delayopen-and-automatic-start.md AC-1e |
| [`RFC4271-3.1-2`](#rfc4271-3.1-2) The next hop for each of these routes MUST be resolvable via the local BGP speaker's Routing Table. (§3.2) | {gap}, no test | the BGP Loc-RIB install performs no reachability check on the route's next hop. mirrorToLocRIB inserts the winning path with whatever next-hop address the attribute carried (internal/component/bgp/plugins/rib/rib_bestchange.go:797-830), and the candidate gather step filters only on SRv6 ineligibility (internal/component/bgp/plugins/rib/rib_commands.go:1039-1057). Resolvability is enforced downstream at FIB-install time, which removes the route from the routing table but leaves it in the Loc-RIB |
| [`RFC4271-5.1.7-1`](#rfc4271-5.1.7-1) A BGP speaker that performs route aggregation MAY add the AGGREGATOR attribute, which SHALL contain its own AS number and IP address. (§5.1.7) | {gap}, no test | ze performs no route aggregation, so it forms no AGGREGATOR of its own. The aggregator leaf of a static route (internal/component/bgp/yang/ze-bgp-conf.yang, read in internal/component/bgp/config/bgp_routes.go) sends any AS:IP the operator writes with no check against the local AS or address, so the knob can emit an AGGREGATOR that is not the speaker's own. No owner decision declines aggregation, so this is owed with it |
| [`RFC4271-9.1.2-1`](#rfc4271-9.1.2-1) If the NEXT_HOP attribute of a BGP route depicts an address that is not resolvable, or if it would become unresolvable if the route was installed in the routing table, the BGP route MUST be excluded from the Phase 2 decision function. (§9.1.2) | {gap}, no test | an unresolvable NEXT_HOP does not exclude the route from Phase 2. gatherCandidatesLocked excludes only SRv6-ineligible entries and next hops naming one of ze's own session addresses (internal/component/bgp/plugins/rib/rib_commands.go), and extractCandidate uses the next hop solely to look up an interior cost, giving an unresolvable one the maximum (internal/component/bgp/plugins/rib/rib_commands.go `extractCandidate`). The route therefore still competes: it loses Section 9.1.2.2 step e) to any resolvable path, but wins outright when it is preferred at an earlier step (LOCAL_PREF, AIGP, AS_PATH, ORIGIN, MULTI_EXIT_DISC, EBGP over IBGP) or is the only path |
| [`RFC4271-9.1.2-4`](#rfc4271-9.1.2-4) If either the immediate next-hop or the IGP cost to the NEXT_HOP (where the NEXT_HOP is resolved through an IGP route) changes, Phase 2 Route Selection MUST be performed again. (§9.1.2) | {gap}, no test | nothing re-runs Phase 2 when the immediate next-hop or the IGP cost to the NEXT_HOP changes. The only entry points to checkBestPathChange are the UPDATE ingest path and the peer-state paths (internal/component/bgp/plugins/rib/rib_structured.go:271-286), and the IGP cost function is a passive lookup registered once with no invalidation callback (internal/component/bgp/plugins/rib/bestpath.go:30-43) |
| [`RFC4271-9.1.2.1-2`](#rfc4271-9.1.2.1-2) Unresolvable routes SHALL be removed from the Loc-RIB and the routing table. (§9.1.2) | {gap}, no test | an unresolvable route is not removed from the Loc-RIB. The only Loc-RIB removal in the BGP plugin is the no-candidate-remains branch of checkBestPathChange (internal/component/bgp/plugins/rib/rib_bestchange.go:766-782), which is driven by the Adj-RIB-In losing its last path and never by next-hop resolvability; nothing in the plugin consults a resolver |
| [`RFC4271-9.1.2.2-2`](#rfc4271-9.1.2.2-2) If an implementation chooses to remove MULTI_EXIT_DISC, then the optional comparison on MULTI_EXIT_DISC, if performed, MUST be performed only among EBGP-learned routes. (§9.1.2.2) | {gap}, no test | the common egress guard is implemented, but normal BGP selected-route readvertisement has no runnable producer and no discriminating proof. `bgp-rib` records selected Loc-RIB state (internal/component/bgp/plugins/rib/rib_bestchange.go:738-790), route-server and route-reflector plugins forward cached UPDATEs instead (internal/component/bgp/plugins/rs/server.go:433-434 and internal/component/bgp/plugins/rr/rr.go:188-195), and BGP-to-BGP redistribution is rejected as same-protocol redistribution (internal/core/redistevents/registry.go:144-146) |
| [`RFC4271-9.2-2`](#rfc4271-9.2-2) A route SHALL NOT be installed in the Adj-Rib-Out unless the destination, and NEXT_HOP described by this route, may be forwarded appropriately by the Routing Table. (§9.1.3) | {gap}, no test | nothing gates Adj-RIB-Out installation on the destination and NEXT_HOP being forwardable. QueueAnnounce records the route unconditionally (internal/component/bgp/rib/outgoing.go:65-101), and the forwarding rails decide only on filters, family negotiation and the route-reflection rules (internal/component/bgp/reactor/forward_rs.go:295-333) |
| [`RFC4271-9.2-3`](#rfc4271-9.2-3) If a route in Loc-RIB is excluded from a particular Adj-RIB-Out, the previously advertised route in that Adj-RIB-Out MUST be withdrawn from service by means of an UPDATE message (§9.1.3) | {gap}, no test | a route excluded from a peer's Adj-RIB-Out by an egress filter is skipped silently, leaving the peer's previous advertisement in place instead of withdrawing it. Both forwarding rails `continue` on suppression with no withdrawal built (internal/component/bgp/reactor/forward_rs.go:320-333 and internal/component/bgp/reactor/reactor_api_forward.go:496-506); the one announce-to-withdraw conversion is LLGR-specific and filter-requested, not exclusion-driven (internal/component/bgp/reactor/reactor_api_forward.go:588-601) |
| [`RFC4271-9.2.1.1-2`](#rfc4271-9.2.1.1-2) Two UPDATE messages sent by a BGP speaker to a peer that advertise feasible routes and/or withdrawal of unfeasible routes to some common set of destinations MUST be separated by at least MinRouteAdvertisementIntervalTimer. (§9.2.1.1) | {gap}, no test | ze has no MinRouteAdvertisementIntervalTimer, so successive UPDATEs to a common set of destinations are not spaced. The timer set implements only ConnectRetry, Hold and Keepalive and records the omission in its own doc comment (internal/component/bgp/fsm/timer.go:34-42, "MinRouteAdvertisementIntervalTimer (Section 9.2.1.1) - not implemented here"); `grep -rniE 'minroute\|mrai' --include=*.go internal/` finds no producer |
| [`RFC4271-9.2.2.2-1`](#rfc4271-9.2.2.2-1) Routes that have different MULTI_EXIT_DISC attributes SHALL NOT be aggregated. (§9.2.2.2) | {gap}, no test | ze implements no route aggregation (Section 9.2.2.2): no code forms an aggregate from contributing routes, and internal/component/bgp/message/update_build_grouped.go::BuildGroupedUnicast packing several prefixes into one UPDATE is not aggregation. An operator can configure a covering prefix on a static route with any ORIGIN, NEXT_HOP, ATOMIC_AGGREGATE and AGGREGATOR (internal/component/bgp/config/bgp_routes.go), and nothing relates it to the routes it covers. No owner decision declines aggregation, so this is owed with it |
| [`RFC4271-9.2.2.2-2`](#rfc4271-9.2.2.2-2) ORIGIN attribute: If at least one route among routes that are aggregated has ORIGIN with the value INCOMPLETE, then the aggregated route MUST have the ORIGIN attribute with the value INCOMPLETE. Otherwise, if at least one route among routes that are aggregated has ORIGIN with the value EGP, then the aggregated route MUST have the ORIGIN attribute with the value EGP. (§9.2.2.2) | {gap}, no test | ze implements no route aggregation (Section 9.2.2.2): no code forms an aggregate from contributing routes, and internal/component/bgp/message/update_build_grouped.go::BuildGroupedUnicast packing several prefixes into one UPDATE is not aggregation. An operator can configure a covering prefix on a static route with any ORIGIN, NEXT_HOP, ATOMIC_AGGREGATE and AGGREGATOR (internal/component/bgp/config/bgp_routes.go), and nothing relates it to the routes it covers. No owner decision declines aggregation, so this is owed with it |
| [`RFC4271-9.2.2.2-3`](#rfc4271-9.2.2.2-3) NEXT_HOP: When aggregating routes that have different NEXT_HOP attributes, the NEXT_HOP attribute of the aggregated route SHALL identify an interface on the BGP speaker that performs the aggregation. (§9.2.2.2) | {gap}, no test | ze implements no route aggregation (Section 9.2.2.2): no code forms an aggregate from contributing routes, and internal/component/bgp/message/update_build_grouped.go::BuildGroupedUnicast packing several prefixes into one UPDATE is not aggregation. An operator can configure a covering prefix on a static route with any ORIGIN, NEXT_HOP, ATOMIC_AGGREGATE and AGGREGATOR (internal/component/bgp/config/bgp_routes.go), and nothing relates it to the routes it covers. No owner decision declines aggregation, so this is owed with it |
| [`RFC4271-9.2.2.2-4`](#rfc4271-9.2.2.2-4) ATOMIC_AGGREGATE: If at least one of the routes to be aggregated has ATOMIC_AGGREGATE path attribute, then the aggregated route SHALL have this attribute as well. (§9.2.2.2) | {gap}, no test | ze implements no route aggregation (Section 9.2.2.2): no code forms an aggregate from contributing routes, and internal/component/bgp/message/update_build_grouped.go::BuildGroupedUnicast packing several prefixes into one UPDATE is not aggregation. An operator can configure a covering prefix on a static route with any ORIGIN, NEXT_HOP, ATOMIC_AGGREGATE and AGGREGATOR (internal/component/bgp/config/bgp_routes.go), and nothing relates it to the routes it covers. No owner decision declines aggregation, so this is owed with it |
| [`RFC4271-9.2.2.2-5`](#rfc4271-9.2.2.2-5) AGGREGATOR: Any AGGREGATOR attributes from the routes to be aggregated MUST NOT be included in the aggregated route. (§9.2.2.2) | {gap}, no test | ze implements no route aggregation (Section 9.2.2.2): no code forms an aggregate from contributing routes, and internal/component/bgp/message/update_build_grouped.go::BuildGroupedUnicast packing several prefixes into one UPDATE is not aggregation. An operator can configure a covering prefix on a static route with any ORIGIN, NEXT_HOP, ATOMIC_AGGREGATE and AGGREGATOR (internal/component/bgp/config/bgp_routes.go), and nothing relates it to the routes it covers. No owner decision declines aggregation, so this is owed with it |
| [`RFC4271-9.2.1.1-3`](#rfc4271-9.2.1.1-3) If new routes are selected multiple times while awaiting the expiration of MinRouteAdvertisementIntervalTimer, the last route selected SHALL be advertised at the end of MinRouteAdvertisementIntervalTimer. (§9.2.1.1) | {gap}, no test | with no MinRouteAdvertisementIntervalTimer there is no expiry at which a last-selected route could be advertised. The timer is absent by design note (internal/component/bgp/fsm/timer.go:39) and no producer buffers a pending best-route advertisement against such a timer; best-path changes are published as they are computed (internal/component/bgp/plugins/rib/rib_bestchange.go:832-880) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4271-4.1-1`](#rfc4271-4.1-1)

This 16-octet field is included for compatibility; it MUST be set to all ones. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sender side. TestRFC4271EveryMessageSentWithAllOnesMarker packs OPEN (classic, extended, >255), UPDATE, NOTIFICATION, KEEPALIVE, ROUTE-REFRESH into a fresh buffer and at offset 7 of a zero-filled buffer and asserts the 16 marker octets equal 0xFF each time; red if writeHeader writes any other marker. Receive-side (6.1) tags kept.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271EveryMessageSentWithAllOnesMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_header_send_test.go#L56) | unit/verify | revert, verified |
| negative | [`TestRFC4271MarkerNotAllOnesRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L43) | unit/verify | unproven |
| positive | [`TestRFC4271EveryMessageSentWithAllOnesMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_header_send_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestRFC4271MarkerAllOnesOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L20) | unit/verify | unproven |

### [`RFC4271-4.1-2`](#rfc4271-4.1-2)

Therefore, the Length field MUST have the smallest value required, given the rest of the message. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC4271EveryMessageSentWithSmallestLength asserts, for all five types incl. both RFC 9072 extended OPEN shapes, that the Length field and the octets written equal an independently computed minimum, and that a 0xAA spare tail past it is untouched. Judge break openExtendedFixedLen 13->14: red (the D-8 fix it proves).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271EveryMessageSentWithSmallestLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_header_send_test.go#L130) | unit/verify | revert, verified |
| negative | [`TestRFC4271NonSmallestLengthRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L96) | unit/verify | unproven |
| positive | [`TestRFC4271EveryMessageSentWithSmallestLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_header_send_test.go#L128) | unit/verify | revert, verified |
| positive | [`TestRFC4271SmallestLengthOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L63) | unit/verify | unproven |

### [`RFC4271-4.1-3`](#rfc4271-4.1-3)

The value of the Length field MUST always be at least 19 and no greater than 4096 (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 4271 Section 4.1: “The value of the Length field MUST always be at least 19 and no greater than 4096, and MAY be further constrained, depending on the message type.” The changed sender test supplies an equivalent all-false per-family ADD-PATH selector. It establishes that 1500 prefixes exceed one 4096-byte UPDATE, encodes every emitted chunk, requires declared Length equal to written octets within 19..4096, and requires every prefix in original order. Receive controls accept 19, 1000 and 4096 and reject 18, 4097 and 65535 without Extended Message, with exact header-error classification. Producer tracing confirms size budgets include the BGP header; the final ordinary writer sizes effective post-policy bytes, consumes split chunks synchronously and refuses unencodable output instead of emitting a truncated frame. Negotiated RFC 8654 extension remains a separate qualification. This judgment is based on assertions and current source, not producer-halt proof alone.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271UpdatesSentWithinLengthBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_header_send_test.go#L88) | unit/verify | revert, verified |
| negative | [`TestRFC4271MessageLengthOutOfBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L151) | unit/verify | unproven |
| positive | [`TestRFC4271UpdatesSentWithinLengthBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_header_send_test.go#L86) | unit/verify | revert, verified |
| positive | [`TestRFC4271MessageLengthWithinBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L125) | unit/verify | unproven |

### [`RFC4271-4.3-1`](#rfc4271-4.3-1)

For well-known attributes, the Transitive bit MUST be set to 1 (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271WellKnownAttributeErrorsAreCaught`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L501) | unit/verify | unproven |
| positive | [`TestRFC4271WellKnownAttributesAreTransitive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L31) | unit/verify | unproven |

### [`RFC4271-4.3-2`](#rfc4271-4.3-2)

For well-known attributes and for optional non-transitive attributes, the Partial bit MUST be set to 0. (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. send side (attribute encoders), receive-path clear (enforceRFC7606), readvertise rails (rib CommitService, storage ToWireBytes) and the RS relay each fed 0x60/0xA0 and asserted 0x40/0x80 on the wire, COMMUNITIES 0xE0 kept

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271PartialBitClearedOnReadvertisedWellKnown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L47) | unit/verify | unproven |
| negative | [`TestRFC4271PartialClearedWhenTheRailReadvertises`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc4271_partial_test.go#L75) | unit/verify | revert, verified |
| negative | [`TestRFC4271PartialNotStampedOnExcludedClasses`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L170) | unit/verify | unproven |
| positive | [`TestRFC4271PartialClearedOnTheRelayedWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_relay_partial_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestRFC4271PartialNotSetOnRecognizedOrNonTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1123) | unit/verify | unproven |
| positive | [`TestRFC4271PartialBitClearOnSend`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L55) | unit/verify | unproven |

### [`RFC4271-4.3-3`](#rfc4271-4.3-3)

The lower-order four bits of the Attribute Flags octet are unused. They MUST be zero when sent (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (rfc4271): "The lower-order four bits of the Attribute Flags octet are unused. They MUST be zero when sent" The only edit to test/plugin/rfc4271-attr-flags-low-nibble-relay.ci since the last judgement is f8f7628bdf, which adds a bgp-level filter { import { final accept; } export { final accept; } } with no filter refs, because RFC 8212 Section 3 now makes an EBGP direction with no explicit policy reject. No expect, reject, input or requirement-tag line changed. policy.ConcatChains keeps every peer-level ref and takes the final from the most specific configured level, so a peer-level chain still decides where one exists. An accepting final with no refs neither refuses nor rewrites a route: a withheld, stripped or discarded outcome the file asserts can still come only from the producer under test, and a delivered outcome is the eligibility the test was written under before RFC 8212; without the block the positives would fail for the wrong reason. What the unit proves is unchanged. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271WriteLeavesCleanFlagsUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_attr_flags_egress_test.go#L135) | unit/verify | revert, verified |
| positive | [`TestRFC4271WriteClearsLowNibbleEveryAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_attr_flags_egress_test.go#L109) | unit/verify | revert, verified |
| positive | [`TestRFC4271RelayedLowNibbleClearedOnBothRails`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_attr_flags_relay_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestRFC4271AttributeFlagsLowNibbleZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L80) | unit/verify | unproven |
| positive | [`rfc4271-attr-flags-low-nibble-relay.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc4271-attr-flags-low-nibble-relay.ci#L4) | functional/verify | revert, verified |

### [`RFC4271-4.3-4`](#rfc4271-4.3-4)

The lower-order four bits of the Attribute Flags octet are unused. They MUST be zero when sent and MUST be ignored when received. (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (rfc4271): "They MUST be zero when sent and MUST be ignored when received." The only edit to test/plugin/rfc4271-attr-flags-low-nibble-relay.ci since the last judgement is f8f7628bdf, which adds a bgp-level filter { import { final accept; } export { final accept; } } with no filter refs, because RFC 8212 Section 3 now makes an EBGP direction with no explicit policy reject. No expect, reject, input or requirement-tag line changed. policy.ConcatChains keeps every peer-level ref and takes the final from the most specific configured level, so a peer-level chain still decides where one exists. An accepting final with no refs neither refuses nor rewrites a route: a withheld, stripped or discarded outcome the file asserts can still come only from the producer under test, and a delivered outcome is the eligibility the test was written under before RFC 8212; without the block the positives would fail for the wrong reason. What the unit proves is unchanged. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271AttrFlagsHighBitsNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L260) | unit/verify | unproven |
| positive | [`TestRFC4271AttrFlagsLowNibbleIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L233) | unit/verify | unproven |
| positive | [`TestRFC4271RelayedLowNibbleClearedOnBothRails`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_attr_flags_relay_test.go#L26) | unit/verify | revert, verified |
| positive | [`rfc4271-attr-flags-low-nibble-relay.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc4271-attr-flags-low-nibble-relay.ci#L5) | functional/verify | revert, verified |

### [`RFC4271-4.4-1`](#rfc4271-4.4-1)

KEEPALIVE messages MUST NOT be sent more frequently than one per second (§4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271KeepaliveIntervalNeverSubSecond`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_test.go#L50) | unit/verify | unproven |
| positive | [`TestRFC4271KeepaliveNotFasterThanOnePerSecond`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_test.go#L18) | unit/verify | unproven |

### [`RFC4271-4.4-2`](#rfc4271-4.4-2)

If the negotiated Hold Time interval is zero, then periodic KEEPALIVE messages MUST NOT be sent. (§4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Session level: handleOpen+handleKeepalive with a 1 s configured keepalive. Positive: negotiated hold 9 -> timers hold 9, KeepaliveTimer running, >=1 KEEPALIVE in 1.6 s. Negative: negotiated zero from either side -> timers hold 0, KeepaliveTimer stopped, zero octets in 1.6 s. Judge break: negotiateWith feeding the configured hold into the timers when the peer offers 0 turned the negative red. Old fsm timer_test.go tags are supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestKeepaliveWithZeroHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_timer_test.go#L365) | unit/verify | unproven |
| negative | [`TestRFC4271ZeroNegotiatedHoldSendsNoPeriodicKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_keepalive_session_test.go#L92) | unit/verify | revert, verified |
| positive | [`TestTimersKeepaliveTimer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_timer_test.go#L144) | unit/verify | unproven |
| positive | [`TestRFC4271NonZeroNegotiatedHoldSendsPeriodicKeepalives`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_keepalive_session_test.go#L70) | unit/verify | revert, verified |

### [`RFC4271-6-1`](#rfc4271-6-1)

If no Error Subcode is specified, then a zero MUST be used. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a non-zero subcode when none is specified. TestRFC4271NotificationUnspecifiedSubcodeIsZero asserts data[HeaderLen+1]==0 on the packed NOTIFICATION and 0 after decode. The negative is a same-producer contrast (three specified subcodes packed verbatim and non-zero), which rules out an encoder that writes a constant zero, so the zero is shown to come from absence.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271NotificationSpecifiedSubcodePreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L377) | unit/verify | unproven |
| positive | [`TestRFC4271NotificationUnspecifiedSubcodeIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L354) | unit/verify | unproven |

### [`RFC4271-4.2-1`](#rfc4271-4.2-1)

Hold Time MUST be either zero or at least three seconds (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L341) | unit/verify | unproven |
| positive | [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L339) | unit/verify | unproven |
| positive | [`open-hold-time-peer-lower-wins.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/open-hold-time-peer-lower-wins.ci#L3) | functional/verify | unproven |

### [`RFC4271-4.2-2`](#rfc4271-4.2-2)

Upon receipt of an OPEN message, a BGP speaker MUST calculate the value of the Hold Timer by using the smaller of its configured Hold Time and the Hold Time received in the OPEN message. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive cases local-smaller, peer-smaller and equal would fail on max() or on always-local; the negative (zero from either side) is a mirror of min rather than a violating input

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegotiateWith_HoldTimeZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L74) | unit/verify | unproven |
| positive | [`TestNegotiateWith_HoldTimeMinOfBoth`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L45) | unit/verify | unproven |

### [`RFC4271-6.2-1`](#rfc4271-6.2-1)

An implementation MUST reject Hold Time values of one or two seconds (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L345) | unit/verify | unproven |
| positive | [`TestOpenValidateHoldTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L343) | unit/verify | unproven |

### [`RFC4271-6.2-2`](#rfc4271-6.2-2)

An implementation that accepts a Hold Time MUST use the negotiated value (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271LocalHoldTimeNotUsedWhenPeerProposesSmaller`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L190) | unit/verify | unproven |
| positive | [`TestRFC4271NegotiatedHoldTimeDrivesTimers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L169) | unit/verify | unproven |

### [`RFC4271-5-1`](#rfc4271-5-1)

BGP implementations MUST recognize all well-known attributes (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271WellKnownAttributeErrorsAreCaught`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L493) | unit/verify | unproven |
| positive | [`TestRFC4271WellKnownAttributesAreRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L464) | unit/verify | unproven |

### [`RFC4271-5-2`](#rfc4271-5-2)

Some of these attributes are mandatory and MUST be included in every UPDATE message that contains NLRI. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged after R43 (2026-09-30). Positive: TestRFC4271UpdateWithNLRICarriesTheMandatoryAttributes requires ORIGIN, AS_PATH and NEXT_HOP in every BuildUnicast UPDATE with body NLRI, stated and bare route, eBGP and iBGP (assertions unchanged; only the call takes the new error). Negative: TestBuildUnicastRefusesIPv4RouteWithNoUsableNextHop, an IPv4 unicast route with an IPv6 next hop and no Extended Next Hop, and with no next hop, is refused with ErrUnicastNextHopUnusable and no Update, rather than written as body NLRI with no NEXT_HOP; observed red on a break of checkInlineNextHop. Producer read: checkInlineNextHop runs before any attribute for IPv4 unicast; the NEXT_HOP arm now keeps NEXT_HOP for an IPv4 next hop when Extended Next Hop is set (RFC 8950 moves only an IPv6 next hop into MP_REACH_NLRI), the second shape of the same defect, covered by the untagged TestBuildUnicastIPv4NextHopWithExtendedNextHopKeepsNextHop. Every caller takes the error: grouped build returns it, static routes log Warn and skip, ze bgp encode exits 1 (untagged TestCmdEncodeRefusesIPv4RouteWithIPv6NextHop). Scope: originating builder rail; forwarded UPDATEs keep the received attributes. The rfc4271_test.go receive-side units carry no records and are neighbours, not the proof.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildUnicastRefusesIPv4RouteWithNoUsableNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_ipv4_route_ipv6_nexthop_test.go#L30) | unit/verify | revert, verified |
| negative | [`TestRFC4271UpdateWithNLRICarriesTheMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_mandatory_send_test.go#L26) | unit/verify | revert, verified |
| negative | [`TestRFC4271WellKnownAttributeErrorsAreCaught`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L498) | unit/verify | unproven |
| positive | [`TestRFC4271UpdateWithNLRICarriesTheMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_mandatory_send_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestRFC4271WellKnownAttributesAreRecognized`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L467) | unit/verify | unproven |

### [`RFC4271-5-3`](#rfc4271-5-3)

If a path with an unrecognized transitive optional attribute is accepted and passed to other BGP peers, then the unrecognized transitive optional attribute of that path MUST be passed, along with the path, to other BGP peers with the Partial bit in the Attribute Flags octet set to 1. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (rfc4271): "If a path with an unrecognized transitive optional attribute is accepted and passed to other BGP peers, then the unrecognized transitive optional attribute of that path MUST be passed, along with the path, to other BGP peers with the Partial bit in the Attribute Flags octet set to 1." The only edit to test/plugin/rfc4271-partial-unknown-transitive.ci since the last judgement is f8f7628bdf, which adds a bgp-level filter { import { final accept; } export { final accept; } } with no filter refs, because RFC 8212 Section 3 now makes an EBGP direction with no explicit policy reject. No expect, reject, input or requirement-tag line changed. policy.ConcatChains keeps every peer-level ref and takes the final from the most specific configured level, so a peer-level chain still decides where one exists. An accepting final with no refs neither refuses nor rewrites a route: a withheld, stripped or discarded outcome the file asserts can still come only from the producer under test, and a delivered outcome is the eligibility the test was written under before RFC 8212; without the block the positives would fail for the wrong reason. What the unit proves is unchanged. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271PartialNotSetOnRecognizedOrNonTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1120) | unit/verify | unproven |
| negative | [`TestRFC4271PartialNotStampedOnExcludedClasses`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L167) | unit/verify | unproven |
| positive | [`TestRFC4271PartialSetOnUnrecognizedTransitiveOptional`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1091) | unit/verify | unproven |
| positive | [`TestRFC4271PartialStampedOnUnrecognizedTransitive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L132) | unit/verify | unproven |
| positive | [`rfc4271-partial-unknown-transitive.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc4271-partial-unknown-transitive.ci#L32) | functional/verify | revert, verified |

### [`RFC4271-5-4`](#rfc4271-5-4)

If a path with a recognized, transitive optional attribute is accepted and passed along to other BGP peers and the Partial bit in the Attribute Flags octet is set to 1 by some previous AS, it MUST NOT be set back to 0 by the current AS. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. receive path, attribute walk and storage readvertise (including the extended-length reframing branch) each keep a Partial bit a previous AS set on recognized and unrecognized transitive attributes

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271PartialBitSurvivesLengthReframing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L115) | unit/verify | unproven |
| negative | [`TestRFC4271PartialFromPreviousASNotCleared`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4271_test.go#L197) | unit/verify | unproven |
| positive | [`TestRFC4271PartialBitPreservedOnUnknownTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L82) | unit/verify | unproven |
| positive | [`TestRFC4271PartialFromPreviousASNeverCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1159) | unit/verify | unproven |

### [`RFC4271-5-5`](#rfc4271-5-5)

Unrecognized non-transitive optional attributes MUST be quietly ignored (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271TheNonTransitiveDropSparesEveryOtherClass`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1258) | unit/verify | unproven |
| positive | [`TestRFC4271UnrecognizedNonTransitiveIsNotPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1217) | unit/verify | unproven |

### [`RFC4271-5-6`](#rfc4271-5-6)

The receiver of an UPDATE message MUST be prepared to handle path attributes within UPDATE messages that are out of order. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC4271OutOfOrderAttributesReachTheReceivePathIntact builds interleaved and fully descending UPDATEs, asserts ValidateUpdateRFC7606 None and that every attribute via wireu.WireUpdate.Attrs().Get and the NLRI equal the ascending UPDATE's, value by value. A receiver assuming ascending order goes red; the lazy view the reactor uses is exercised, not only the validator.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OutOfOrderAttributesReachTheReceivePathIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_out_of_order_receive_test.go#L29) | unit/verify | revert, verified |
| negative | [`TestRFC4271OutOfOrderDoesNotMaskMalformation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L324) | unit/verify | unproven |
| positive | [`TestRFC4271OutOfOrderAttributesReachTheReceivePathIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_out_of_order_receive_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestRFC4271AttributesOutOfOrderAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L294) | unit/verify | unproven |

### [`RFC4271-4.3-5`](#rfc4271-4.3-5)

However, a BGP speaker MUST be able to process UPDATE messages in this form. (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity annotated; structured, pool and injection paths each process an UPDATE naming 10.0.0.0/8 in WITHDRAWN and NLRI and assert the prefix installed with no withdrawal

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRIBInjectSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L119) | unit/verify | unproven |
| positive | [`TestRIBPoolPathSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L85) | unit/verify | unproven |
| positive | [`TestRIBSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L39) | unit/verify | unproven |

### [`RFC4271-5.1.3-1`](#rfc4271-5.1.3-1)

A route originated by a BGP speaker SHALL NOT be advertised to a peer using an address of that peer as NEXT_HOP. (§5.1.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/le/interoplab/bgp/check_rfc.go::checkSelfNextHopWithheld, internal/le/interoplab/bgp/check_rfc.go::checkSelfNextHopWithheld#2 moved. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (rfc4271): "A route originated by a BGP speaker SHALL NOT be advertised to a peer using an address of that peer as NEXT_HOP." The only edit to test/plugin/originated-nexthop-peer-own.ci since the last judgement is f8f7628bdf, which adds a bgp-level filter { import { final accept; } export { final accept; } } with no filter refs, because RFC 8212 Section 3 now makes an EBGP direction with no explicit policy reject. No expect, reject, input or requirement-tag line changed. policy.ConcatChains keeps every peer-level ref and takes the final from the most specific configured level, so a peer-level chain still decides where one exists. An accepting final with no refs neither refuses nor rewrites a route: a withheld, stripped or discarded outcome the file asserts can still come only from the producer under test, and a delivered outcome is the eligibility the test was written under before RFC 8212; without the block the positives would fail for the wrong reason. What the unit proves is unchanged. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEgressNextHopIsPeerOwnReadsTheRewrittenAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L298) | unit/verify | unproven |
| negative | [`TestForwardRSWithholdsRouteWhoseNextHopIsTheClientsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L263) | unit/verify | revert, verified |
| negative | [`TestForwardWithholdsRouteWhoseNextHopIsTheDestinationsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L190) | unit/verify | revert, verified |
| negative | [`TestSendAnnounceWithholdsRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L478) | unit/verify | unproven |
| negative | [`TestSendUpdateWithholdsOriginatedRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L441) | unit/verify | unproven |
| negative | [`checkSelfNextHopWithheld`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1001) | interop/nightly | unproven |
| negative | [`originated-nexthop-peer-own.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/originated-nexthop-peer-own.ci#L10) | functional/verify | unproven |
| positive | [`TestEgressNextHopIsPeerOwnReadsTheRewrittenAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L294) | unit/verify | unproven |
| positive | [`TestForwardRSWithholdsRouteWhoseNextHopIsTheClientsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L260) | unit/verify | revert, verified |
| positive | [`TestForwardWithdrawsFromDestinationWhoseNextHopIsItsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L225) | unit/verify | revert, verified |
| positive | [`TestForwardWithholdsRouteWhoseNextHopIsTheDestinationsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L185) | unit/verify | revert, verified |
| positive | [`TestSendAnnounceWithholdsRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L476) | unit/verify | unproven |
| positive | [`TestSendUpdateWithholdsOriginatedRouteWithPeerOwnNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_next_hop_test.go#L437) | unit/verify | unproven |
| positive | [`checkSelfNextHopWithheld`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1000) | interop/nightly | unproven |
| positive | [`originated-nexthop-peer-own.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/originated-nexthop-peer-own.ci#L7) | functional/verify | unproven |

### [`RFC4271-5.1.3-2`](#rfc4271-5.1.3-2)

A BGP speaker SHALL NOT install a route with itself as the next hop (§5.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271SelfNextHopDoesNotShadowASoundAlternative`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L89) | unit/verify | unproven |
| negative | [`TestRFC4271SelfNextHopRouteIsNotInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L47) | unit/verify | unproven |
| negative | [`TestRFC4271SelfNextHopSetComesFromPeerEvents`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L126) | unit/verify | unproven |
| positive | [`TestRFC4271SelfNextHopRouteIsNotInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L43) | unit/verify | unproven |
| positive | [`TestRFC4271SelfNextHopSetComesFromPeerEvents`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L122) | unit/verify | unproven |

### [`RFC4271-5.1.3-3`](#rfc4271-5.1.3-3)

A BGP speaker MUST be able to support the disabling advertisement of third party NEXT_HOP attributes in order to handle imperfectly bridged media. (§5.1.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-01 (BGP c17 judge, residuals of the c16 verdict). Positive: TestRFC4271NextHopSelfWithAutoLocalAddressSendsTheConnectedEndpoint (next-hop self under `local ip auto`, session endpoint 10.0.0.77: on the general AND route-server forward rails the next-hop-self destination is sent NEXT_HOP 10.0.0.77, a default internal destination the third-party 192.0.2.254; resolveNextHop answers the same). HEAD TestRFC4271NextHopSelfDisablesThirdPartyNextHopOnTheWire keeps the configured-address wire case. Negative: TestRFC4271NextHopSelfWithNoLocalAddressWithholdsTheRoute and TestRFC4271ThirdPartyNextHopDisableFailsClosed (withhold with withdrawal still sent, nhSelfWithheld, ErrNextHopSelfNoLocal). Records on connectedLocalAddress (+), precomputeNextHop (- two units) observed red. c16 residuals closed in c17: (1) the WARN line is now asserted by untagged TestRFC4271NextHopSelfWithheldRouteIsLoggedAtWarn (Warn-level handler, both rails, exactly one line per withhold naming peer= and rfc="RFC 4271 Section 5.1.3"); (2) TestRFC4271ThirdPartyNextHopCanBeDisabled now carries a record on precomputeNextHop; (3) a link-local connected endpoint is now gated on both forward rails by the same predicate as the announce rail (Peer.linkLocalOnlyNextHopRefused via egressNextHopLinkLocalOnlyRefused), proven by untagged TestNextHopSelfLinkLocalEndpointNeedsTheCapabilityOnTheForwardRails; the judge's overlay disabling only the route-server gate reddens its rs=true case (j17-ov-rs.log). Quote verbatim (RFC 4271 Section 5.1.3).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ThirdPartyNextHopDisableFailsClosed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L373) | unit/verify | revert, verified |
| negative | [`TestRFC4271NextHopSelfWithNoLocalAddressWithholdsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_third_party_nexthop_test.go#L147) | unit/verify | revert, verified |
| positive | [`TestRFC4271ThirdPartyNextHopCanBeDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L339) | unit/verify | revert, verified |
| positive | [`TestRFC4271NextHopSelfDisablesThirdPartyNextHopOnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_third_party_nexthop_test.go#L41) | unit/verify | revert, verified |
| positive | [`TestRFC4271NextHopSelfWithAutoLocalAddressSendsTheConnectedEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_third_party_nexthop_test.go#L106) | unit/verify | revert, verified |

### [`RFC4271-5.1.4-1`](#rfc4271-5.1.4-1)

The MULTI_EXIT_DISC attribute received from a neighboring AS MUST NOT be propagated to other neighboring ASes. (§5.1.4)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/le/interoplab/bgp/check_rfc.go::checkMEDAcrossAS, internal/le/interoplab/bgp/check_rfc.go::checkMEDAcrossAS#2 moved. Independent source rejudgment of RFC 4271 Section 5.1.4: "The MULTI_EXIT_DISC attribute received from a neighboring AS MUST NOT be propagated to other neighboring ASes." Read all tagged Go units, both functional files, native checker and relevant producer/fixture. applyFactsMED strips a received unchanged metric for ordinary eBGP, but retains iBGP/route-server-client propagation and locally created or policy-replaced metrics. Go positives compare exact rebuilt attributes for metrics zero/100/max; controls retain the distinct allowed provenance. med-not-propagated-across-as compares full frames toward the RS client and ordinary external peer. The moved med-locally-set-reaches-peer still expects an exact MED=100 frame and the external fixture originates it through UpdateRoute; its accepting policy does not manufacture MED. This control's source/header naming should not be inflated to every established-session rail: the prior audit located it on queued initial replay, not writeAnnounceUpdate. checkMEDAcrossAS independently requires genuine source AS/NEXT_HOP blocks plus absent received MED and present locally originated MED=42. Verdict enforced with those precise controls, not a blanket ban on attribute 4. No execution or discrimination replay was performed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestForwardKeepsFilterSetMED`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L299) | unit/verify | unproven |
| negative | [`TestForwardSuppressesReceivedMEDToAnotherAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L203) | unit/verify | unproven |
| negative | [`TestForwardWritesLocallySetMED`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L266) | unit/verify | unproven |
| negative | [`TestMEDPropagationAllowedTo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L403) | unit/verify | unproven |
| negative | [`checkMEDAcrossAS`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L201) | interop/nightly | unproven |
| negative | [`med-locally-set-reaches-peer.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-locally-set-reaches-peer.ci#L4) | functional/verify | unproven |
| negative | [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L8) | functional/verify | unproven |
| positive | [`TestForwardSuppressesReceivedMEDToAnotherAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_med_test.go#L198) | unit/verify | unproven |
| positive | [`checkMEDAcrossAS`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L197) | interop/nightly | unproven |
| positive | [`med-not-propagated-across-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-not-propagated-across-as.ci#L4) | functional/verify | unproven |

### [`RFC4271-5.1.4-4`](#rfc4271-5.1.4-4)

A BGP speaker MUST implement a mechanism (based on local configuration) that allows the MULTI_EXIT_DISC attribute to be removed from a route (§5.1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseModifyDefsMEDRemove`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/rfc4271_modify_test.go#L320) | unit/verify | revert, verified |
| negative | [`checkMEDRemovalConfiguration`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L323) | interop/nightly | unproven |
| positive | [`TestParseModifyDefsMEDRemove`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/rfc4271_modify_test.go#L314) | unit/verify | revert, verified |
| positive | [`checkMEDRemovalConfiguration`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L319) | interop/nightly | mutant, verified |
| positive | [`med-removal-configured.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-configured.ci#L4) | functional/verify | unproven |

### [`RFC4271-5.1.4-2`](#rfc4271-5.1.4-2)

If a BGP speaker is configured to remove the MULTI_EXIT_DISC attribute from a route, then this removal MUST be done prior to determining the degree of preference of the route and prior to performing route selection (Decision Process phases 1 and 2). (§5.1.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-03 by an independent auditor who authored none of ee1025b381. RFC 4271 Section 5.1.4: "If a BGP speaker is configured to remove the MULTI_EXIT_DISC attribute from a route, then this removal MUST be done prior to determining the degree of preference of the route and prior to performing route selection (Decision Process phases 1 and 2)." The only change to test/plugin/med-removal-before-decision.ci is its header comment, which the previous verdict flagged as a nit: it named the deleted TestMEDRemovalMechanismIsConfigurable. It now names four live carriers, each read and found to assert what the header says: filter_modify TestHandleFilterUpdateMEDRemoveIsImportOnly (med remove on import, withheld on export), policychain TestExternalMEDRemove (one AttrModSuppress of code 4 on import, no op without a MED, a failed route on export), reactor TestLibraryMEDRemovalFollowsTheChainOrder (the later import member decides, three subtests) and TestLibraryMEDRemovalLeavesNoZeroLengthAttribute (no attribute 4, no zero-length attribute, ORIGIN survives). No option, input, expect or requirement-tag line moved, so the .ci still requires the Adj-RIB-In stored route to carry no MED with the exact ORIGIN, AS_PATH and COMMUNITIES survivors. The other carriers (TestHandleFilterUpdateMEDRemoveIsImportOnly, med-removal-configured.ci, med-removal-export-refused.ci) are unchanged. Verdict unchanged: enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHandleFilterUpdateMEDRemoveIsImportOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/rfc4271_modify_test.go#L396) | unit/verify | revert, verified |
| negative | [`med-removal-before-decision.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-before-decision.ci#L10) | functional/verify | unproven |
| negative | [`med-removal-export-refused.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-export-refused.ci#L4) | functional/verify | unproven |
| positive | [`TestHandleFilterUpdateMEDRemoveIsImportOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/rfc4271_modify_test.go#L389) | unit/verify | revert, verified |
| positive | [`med-removal-before-decision.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-before-decision.ci#L4) | functional/verify | unproven |
| positive | [`med-removal-configured.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/med-removal-configured.ci#L8) | functional/verify | unproven |

### [`RFC4271-5.1.5-1`](#rfc4271-5.1.5-1)

LOCAL_PREF is a well-known attribute that SHALL be included in all UPDATE messages that a given BGP speaker sends to other internal peers. (§5.1.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-06 by an independent auditor who authored none of the change; no test or record ran for this judgement. RFC 4271 Section 5.1.5: "LOCAL_PREF is a well-known attribute that SHALL be included in all UPDATE messages that a given BGP speaker sends to other internal peers." The staleness is doc comments only: git diff from the verdict's commit to the working tree changes nine comment lines in rfc4271_test.go and no statement. The new citations are true: reactor_wire.go:536-542 is the if localPrefAllowedTo(isIBGP) block in writeAnnounceUpdate (line 356), and :536 its gate; the rename to the unexported writeAnnounceUpdate is correct. Positive asserts present and exactly 00000064 on the announce rail; negative asserts absent toward external with ORIGIN and NEXT_HOP present. TestRFC4271ForwardAddsLocalPrefTowardInternalPeer proves the add on both forward rails with an external control. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestForwardLocalPrefStrippedToExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_local_pref_test.go#L45) | unit/verify | unproven |
| negative | [`TestRFC4271ForwardAddsLocalPrefTowardInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_a2_forward_test.go#L51) | unit/verify | revert, verified |
| negative | [`TestAnnounceStripsLocalPrefTowardExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_api_origin_test.go#L307) | unit/verify | revert, verified |
| negative | [`TestRFC4271LocalPrefOmittedForExternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L107) | unit/verify | unproven |
| positive | [`TestRFC4271ForwardAddsLocalPrefTowardInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_a2_forward_test.go#L50) | unit/verify | revert, verified |
| positive | [`TestRFC4271LocalPrefIncludedForInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L81) | unit/verify | unproven |

### [`RFC4271-5.1.5-2`](#rfc4271-5.1.5-2)

A BGP speaker MUST NOT include this attribute in UPDATE messages it sends to external peers, except in the case of BGP Confederations [RFC3065]. (§5.1.5)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/le/interoplab/bgp/check_rfc.go::checkLocalPrefStrip moved. Re-judged 2026-11-06 by an independent auditor who authored none of the change; no test, record or lab ran for this judgement. RFC 4271 Section 5.1.5: "A BGP speaker MUST NOT include this attribute in UPDATE messages it sends to external peers, except in the case of BGP Confederations [RFC3065]." rfc4271_test.go changed in doc comments only and its citations are true (reactor_wire.go:536-542, the gate through localPrefAllowedTo). checkLocalPrefStrip, uncommitted in the working tree, drops requireGoBGPAttributeAbsent for waitLocalPrefWire, a stronger proof: the speaker relays the original Ze to GoBGP connection and requireLocalPrefInterval reads each frame, so code 5 on any subject UPDATE through the FIFO fence is a semantic failure at assertion 13, with session identity, all three subjects and the reverse KEEPALIVE barrier required. The old assertion could not have gone red: GoBGP v4.11.0 ProcessMessage (read from upstream) holds case *bgp.PathAttributeLocalPref ... if !external, with external true for this non-RS-client peer, so the RIB never showed a leak, as the checker's comment says. The scenario's injector is internal and sends LOCAL_PREF 200, so a strip is owed. TestLocalPrefStripChecksDestinationWire reddens leaked200, leaked0, truncated, wrong connection and missing fence with an unchanged RIB, by its own words checker-boundary evidence and no native run. Bound: no native run of the rewritten checker was seen here and it carries no discrimination record. The unit proofs stand without it: TestRFC4271LocalPrefOmittedForExternalPeers, TestForwardLocalPrefStrippedToExternalPeer, TestForwardLocalPrefStripBeatsAFilterSet. Confederation exception: localPrefAllowedTo holds it constantly false and no YANG module configures a member AS, so it cannot apply. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalPrefAllowedToIsTheOnlyAnswer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_local_pref_test.go#L146) | unit/verify | unproven |
| negative | [`TestRFC4271LocalPrefIncludedForInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L83) | unit/verify | unproven |
| negative | [`local-pref-strip-ebgp.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/local-pref-strip-ebgp.ci#L19) | functional/verify | unproven |
| positive | [`TestForwardLocalPrefStripBeatsAFilterSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_local_pref_test.go#L113) | unit/verify | unproven |
| positive | [`TestForwardLocalPrefStrippedToExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_local_pref_test.go#L41) | unit/verify | unproven |
| positive | [`TestAnnounceStripsLocalPrefTowardExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_api_origin_test.go#L305) | unit/verify | revert, verified |
| positive | [`TestRFC4271LocalPrefOmittedForExternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L104) | unit/verify | unproven |
| positive | [`checkLocalPrefStrip`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L95) | interop/nightly | unproven |
| positive | [`local-pref-strip-ebgp.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/local-pref-strip-ebgp.ci#L14) | functional/verify | unproven |

### [`RFC4271-5.1.5-3`](#rfc4271-5.1.5-3)

If it is contained in an UPDATE message that is received from an external peer, then this attribute MUST be ignored by the receiving speaker, except in the case of BGP Confederations [RFC3065]. (§5.1.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-12 (independent auditor). SHIFTED by 4e731b2d96 (owner-approved lint-only reactor test edits). The tagged unit is byte-identical (gate-verified); the file-level changes are comment punctuation, slices.Concat over the same bytes, joined appends, strings.SplitSeq, newDefaultOriginatePeer taking its fixed addresses from constants with the same values, and registerTypeLenRecognizer losing a lo every caller passed as 1, none of which changes what a helper this unit calls produces. Verdict stands. RFC 4271 §5.1.5, rfc/full/rfc4271.txt:1599-1604: 'If it is contained in an UPDATE message that is received from an external peer, then this attribute MUST be ignored by the receiving speaker, except in the case of BGP Confederations [RFC3065].' The current full short row has this same receive-side sentence and no partial annotation. Read all four covers: internal/component/bgp/message/rfc4271_test.go::TestRFC4271LocalPrefKeptOnInternalSession requires ActionNone and empty DiscardEntries; ::TestRFC4271LocalPrefIgnoredOnExternalSession requires exactly AttributeDiscard, one entry, code 5; both polarities of internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go::TestRFC4271LocalPrefFromExternalPeerNeverReachesTheRIB drive an actual Established session. The internal case requires byte-identical attributes including preference 200 and unchanged NLRI. The external case uses the matching four-octet peer AS, requires one dispatched UPDATE, exact attribute codes [1,2,3,252], absence of code 5, byte-identical retained ORIGIN/AS_PATH/NEXT_HOP, unchanged NLRI, Established state and no NOTIFICATION. The 252 marker is not mistaken for a surviving LOCAL_PREF.
Producing/consuming path read: message/rfc7606.go::validateLocalPrefAttr → reactor/session_validation.go::enforceRFC7606 → ApplyAttrDiscard/rebuild → session_read.go::processMessage → reactor_notify.go::notifyMessageReceiver. The latter derives RawBytes and AttrsWire from the rewritten WireUpdate, not the original raw body. rib_structured.go::handleReceivedStructured consumes that WireUpdate and packed attributes. Q1 yes: exact ignore-versus-retain behavior; Q2 yes by inspection: preserving external preference, dropping the UPDATE or stripping the internal control breaks explicit assertions; Q3 yes: valid established peers and matching external AS isolate preference handling; Q4 yes for the implemented non-confederation receive boundary, not send-side omission or entire RFC support. No confederation feature support or fresh semantic execution is inferred.
Pending note: use the preceding RFC quote, population, assertions and producer chain; retain enforced and state explicitly that this audit ran no tests and independent peer acceptance remains pending.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271LocalPrefIgnoredOnExternalSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L590) | unit/verify | unproven |
| negative | [`TestRFC4271LocalPrefFromExternalPeerNeverReachesTheRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L179) | unit/verify | revert, verified |
| positive | [`TestRFC4271LocalPrefKeptOnInternalSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L567) | unit/verify | unproven |
| positive | [`TestRFC4271LocalPrefFromExternalPeerNeverReachesTheRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L177) | unit/verify | revert, verified |

### [`RFC4271-5.1.5-4`](#rfc4271-5.1.5-4)

The higher degree of preference MUST be preferred. (§5.1.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. SelectBest cases: higher wins, lower loses, equal falls through to AS_PATH; an inverted comparison fails two cases

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBestPath_LocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L209) | unit/verify | unproven |
| positive | [`TestBestPath_LocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L207) | unit/verify | unproven |

### [`RFC4271-5.1.6-1`](#rfc4271-5.1.6-1)

A BGP speaker that receives a route with the ATOMIC_AGGREGATE attribute MUST NOT make any NLRI of that route more specific (as defined in 9.1.4) when advertising this route to other BGP speakers. (§5.1.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-5.1.6-1, so no unit is bound to it.

### [`RFC4271-6.1-1`](#rfc4271-6.1-1)

All errors detected while processing the Message Header MUST be indicated by sending the NOTIFICATION message with the Error Code Message Header Error. (§6.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Every Section 6.1 error class is now driven through a running Peer's real read path in OpenSent and each answer is asserted as exactly one NOTIFICATION with Error Code 1 before EOF: marker all zeros / one 0xFE octet (1/1), Length 18 and 0 (1/2), Length 4097 and the OPEN 28 / UPDATE 22 / KEEPALIVE 20 / NOTIFICATION 20 per-type minima (1/2), Type 99 (1/3). Negatives: a well-formed OPEN in OpenSent, and a KEEPALIVE of 19 plus an UPDATE of 23 in Established, draw no NOTIFICATION and keep the connection. Records observed red: notifyHeaderErr, ValidateLengthWithMax, handleUnknownType (positives); handleOpen, handleUpdate (negatives). Re-read 2026-10-01 (c10 judge): the only change under this verdict is a tag comment line for another id (RFC4271-6.7-1 / RFC4271-Security-1 added, RFC4271-9.1.2.1-1 removed) in the unit's doc comment; every assertion is byte-identical, so the judgement stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271MessageHeaderAtTheBoundsIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L174) | unit/verify | revert, verified |
| negative | [`TestRFC4271OpenSentWellFormedOpenKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L201) | unit/verify | revert, verified |
| positive | [`TestRFC4271MessageHeaderBadLengthIsReported`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L128) | unit/verify | revert, verified |
| positive | [`TestRFC4271MessageHeaderErrorIsReported`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestRFC4271UnknownMessageTypeIsReported`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L158) | unit/verify | revert, verified |

### [`RFC4271-6.1-2`](#rfc4271-6.1-2)

If the Marker field of the message header is not as expected, then a synchronization error has occurred and the Error Subcode MUST be set to Connection Not Synchronized. (§6.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-10-01 (c8 judge): the only change under this verdict is the deleted RFC4271-8.2.2-18 tag line in TestRFC4271OpenSentWellFormedOpenKeepsTheConnection's doc comment (8.2.2-18 retired into 8.2.2-8); the unit's body and assertions are unchanged. All-zero marker and a marker with one 0xFE octet each draw exactly one NOTIFICATION 1/1 with empty Data from a running Peer in OpenSent, then EOF; an all-ones marker (well-formed OPEN) draws no NOTIFICATION. notifyHeaderErr break observed red (positive), handleOpen break observed red (negative). Re-read 2026-10-01 (c10 judge): the only change under this verdict is a tag comment line for another id (RFC4271-6.7-1 / RFC4271-Security-1 added, RFC4271-9.1.2.1-1 removed) in the unit's doc comment; every assertion is byte-identical, so the judgement stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OpenSentWellFormedOpenKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L202) | unit/verify | revert, verified |
| positive | [`TestRFC4271MessageHeaderErrorIsReported`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L30) | unit/verify | revert, verified |

### [`RFC4271-6.1-3`](#rfc4271-6.1-3)

If at least one of the following is true: - if the Length field of the message header is less than 19 or greater than 4096, or - if the Length field of an OPEN message is less than the minimum length of the OPEN message, or - if the Length field of an UPDATE message is less than the minimum length of the UPDATE message, or - if the Length field of a KEEPALIVE message is not equal to 19, or - if the Length field of a NOTIFICATION message is less than the minimum length of the NOTIFICATION message, then the Error Subcode MUST be set to Bad Message Length. The Data field MUST contain the erroneous Length field. (§6.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-01 (BGP c11 judge) after the negative's NOTIFICATION subtest moved from OpenSent to OpenConfirm (OpenSent now answers Event 25 with 5/0 per Section 8.2.2, so 'no NOTIFICATION at all' is only true in OpenConfirm); the 21-octet bound under test is unchanged. Positives prove every Section 6.1 Length condition with exactly one NOTIFICATION 1/2 whose Data is the received Length octets, then EOF: Length 18 and 0, 4097, OPEN 28, UPDATE 22, KEEPALIVE 20, NOTIFICATION 20. Negatives: KEEPALIVE 19 and UPDATE 23 and UPDATE 4096 in Established draw no NOTIFICATION; OPEN 29 in OpenSent draws none; a 21-octet Cease in OpenConfirm draws none and closes. Records observed red: ValidateLengthWithMax (re-recorded for the moved subtest), notifyHeaderErr, handleUpdate, handleOpen.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271MessageHeaderAtTheUpperBoundsIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_bounds_peer_test.go#L49) | unit/verify | revert, verified |
| negative | [`TestRFC4271MessageHeaderAtTheBoundsIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L175) | unit/verify | revert, verified |
| negative | [`TestRFC4271OpenSentWellFormedOpenKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L203) | unit/verify | revert, verified |
| positive | [`TestRFC4271MessageHeaderBadLengthIsReported`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L129) | unit/verify | revert, verified |
| positive | [`TestRFC4271MessageHeaderErrorIsReported`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L31) | unit/verify | revert, verified |

### [`RFC4271-6.1-4`](#rfc4271-6.1-4)

If the Type field of the message header is not recognized, then the Error Subcode MUST be set to Bad Message Type. The Data field MUST contain the erroneous Type field. (§6.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Previously a false {gap} (corrected 2026-10-01). Producer read: processMessage's default arm is the only caller of handleUnknownType, with no state gate before it, and it writes Message Header Error subcode 3 (NotifyHeaderBadType) with Data []byte{type}. Positive: Type 99 through a running Peer's read path in OpenSent draws exactly one NOTIFICATION 1/3 whose Data is 0x63, then EOF (handleUnknownType break observed red). Negative: recognized Types 4 (KEEPALIVE) and 2 (UPDATE) in Established draw no NOTIFICATION within 500 ms and the session stays Established (handleUpdate break observed red).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271MessageHeaderAtTheBoundsIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L176) | unit/verify | revert, verified |
| positive | [`TestRFC4271UnknownMessageTypeIsReported`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L159) | unit/verify | revert, verified |

### [`RFC4271-6.2-3`](#rfc4271-6.2-3)

All errors detected while processing the OPEN message MUST be indicated by sending the NOTIFICATION message with the Error Code OPEN Message Error. (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-6.2-3, so no unit is bound to it.

### [`RFC4271-6.3-1`](#rfc4271-6.3-1)

All errors detected while processing the UPDATE message MUST be indicated by sending the NOTIFICATION message with the Error Code UPDATE Message Error. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. 'All errors' that still produce a NOTIFICATION after RFC 7606 are now proven across the reset classes, each asserting Error Code 3 on the wire: duplicate MP_REACH_NLRI (TestRFC4271UpdateErrorReportedAsUpdateMessageError), duplicate MP_UNREACH_NLRI (TestSessionRFC7606DuplicateMPUnreachNotificationOnTheWire), unrecognized well-known attribute 3/2, impossible NLRI prefix length 3/10, truncated NLRI 3/10, truncated Withdrawn Routes 3/10 (TestSessionRFC4271RetainedUpdateNotifications), malformed MP_REACH_NLRI on a running Established Peer (TestRFC4271EstablishedUpdateErrorReleasesTheConnection). Producer check: every session-reset path funnels Session.rfc7606ResetNotification with NotifyUpdateMessage (rfc7606SessionReset, rfc7606NLRISyntaxAction, the 3(g) Notification in message/rfc7606.go), so the untagged 3(b) section-length and flag-conflict resets reach the same code. rfc7606ResetNotification break observed red for the three new units. Negative: a conformant UPDATE sends nothing (TestRFC4271ConformantUpdateSendsNoUpdateError). Errors RFC 7606 revises to treat-as-withdraw or attribute-discard send no NOTIFICATION by design. Re-read 2026-10-01 (c10 judge): the only change under this verdict is a tag comment line for another id (RFC4271-6.7-1 / RFC4271-Security-1 added, RFC4271-9.1.2.1-1 removed) in the unit's doc comment; every assertion is byte-identical, so the judgement stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConformantUpdateSendsNoUpdateError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L702) | unit/verify | unproven |
| positive | [`TestRFC4271EstablishedUpdateErrorReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_established_teardown_peer_test.go#L130) | unit/verify | revert, verified |
| positive | [`TestSessionRFC4271RetainedUpdateNotifications`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L160) | unit/verify | revert, verified |
| positive | [`TestRFC4271UpdateErrorReportedAsUpdateMessageError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L639) | unit/verify | unproven |
| positive | [`TestSessionRFC7606DuplicateMPUnreachNotificationOnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_dupmp_unreach_wire_test.go#L32) | unit/verify | revert, verified |

### [`RFC4271-6.7-1`](#rfc4271-6.7-1)

However, the Cease NOTIFICATION message MUST NOT be used when a fatal error indicated by this section does exist. (§6.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-01 (BGP c11 judge). All five fatal-error classes of Section 6 are now proven at peer level never to be answered with Cease: Message Header Error (TestRFC4271MessageHeaderErrorIsReported, exactly one code 1; TestRFC4271OpenSentErrorReleasesTheConnection 1/1, 1/2), OPEN Message Error (2/1), UPDATE Message Error (TestRFC4271EstablishedUpdateErrorReleasesTheConnection, exactly one code 3), Hold Timer Expired (exactly one 4/0), and now Finite State Machine Error (Section 6.6): TestRFC4271OpenSentUnexpectedMessageIsAnFSMError and TestRFC4271OpenConfirmUpdateIsAnFSMError assert exactly one 5/0, never Cease, and TestSessionBFDStrictSecondOpenIsAnFSMErrorOnTheWire (newly tagged) asserts the first wire message is code 5. Positive TestRFC4271PrefixLimitTeardownSendsCease shows Cease where no fatal error exists. Each new negative carries an observed-red record (fsmMessageEvent x2, handleOpen). HEAD units rfc4271_test.go:636 and session_prefix_test.go:110 stay supplementary without records.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271EstablishedUpdateErrorReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_established_teardown_peer_test.go#L131) | unit/verify | revert, verified |
| negative | [`TestRFC4271OpenConfirmUpdateIsAnFSMError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_error_peer_test.go#L96) | unit/verify | revert, verified |
| negative | [`TestRFC4271OpenSentUnexpectedMessageIsAnFSMError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_error_peer_test.go#L59) | unit/verify | revert, verified |
| negative | [`TestRFC4271EstablishedHoldTimerExpiryRunsTheEvent10List`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L239) | unit/verify | revert, verified |
| negative | [`TestRFC4271MessageHeaderErrorIsReported`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_header_error_peer_test.go#L32) | unit/verify | revert, verified |
| negative | [`TestRFC4271OpenSentErrorReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L126) | unit/verify | revert, verified |
| negative | [`TestRFC4271UpdateErrorReportedAsUpdateMessageError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L642) | unit/verify | unproven |
| negative | [`TestSessionBFDStrictSecondOpenIsAnFSMErrorOnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L723) | unit/verify | revert, verified |
| positive | [`TestRFC4271PrefixLimitTeardownSendsCease`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_prefix_limit_cease_peer_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestPrefixExceedTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L110) | unit/verify | unproven |

### [`RFC4271-8.2.1-1`](#rfc4271-8.2.1-1)

BGP MUST maintain a separate FSM for each configured peer (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271PerPeerFSMDoesNotShareTimers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L235) | unit/verify | unproven |
| positive | [`TestRFC4271SeparateFSMPerPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L211) | unit/verify | unproven |

### [`RFC4271-8.2.1-2`](#rfc4271-8.2.1-2)

A BGP implementation MUST connect to and listen on TCP port 179 (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ExplicitPortOverridesDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L320) | unit/verify | unproven |
| positive | [`TestRFC4271DefaultBGPPortIs179`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L304) | unit/verify | unproven |

### [`RFC4271-8.2.1-3`](#rfc4271-8.2.1-3)

For each incoming connection, a state machine MUST be instantiated (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-8.2.1-3, so no unit is bound to it.

### [`RFC4271-8.2.2-1`](#rfc4271-8.2.2-1)

If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: hold expiry with no Hold Timer Expired NOTIFICATION. TestRFC4271HoldTimerExpirySendsNotification reads the production hold-expiry write and asserts 21 octets, type NOTIFICATION, code 4, subcode 0; it is red on a bare close or another code. Negative: at 1ms before expiry nothing is written, red on a NOTIFICATION sent early. deadpeer-holddown.ci is byte-exact.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271HoldTimerNotYetExpiredSendsNoNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L842) | unit/verify | unproven |
| positive | [`TestRFC4271HoldTimerExpirySendsNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L813) | unit/verify | unproven |
| positive | [`deadpeer-holddown.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/deadpeer-holddown.ci#L3) | functional/verify | unproven |

### [`RFC4271-8.2.2-2`](#rfc4271-8.2.2-2)

If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 4271 Section 8.2.2 Established Event 10 requires a Hold Timer Expired NOTIFICATION and ConnectRetryTimer zero. Re-read all four tagged units: HoldExpiryRunsTheEvent10ActionList and NoHoldExpiryLeavesTheSessionIntact drive real Session.Run, with event10Established explicitly arming ConnectRetryTimer before the expiry/no-expiry split; the positive now requires the exact ErrHoldTimerExpired sentinel, a stopped ConnectRetryTimer, and code 4 on the captured closing connection, while the negative retains the armed timer and Established session. EstablishedHoldTimerExpiryRunsTheEvent10List additionally drives a running Peer and requireReleased demands exactly one 4/0, EOF and stopped timers; EstablishedHoldTimerFedByKeepalivesNeverExpires supplies KEEPALIVEs for three hold times and demands no NOTIFICATION, no close and the same live session. The peer timer alone starts unarmed, but the explicitly armed Session pair makes deletion of timer stopping observable. Producers are the NewSession hold-expiry callback, Run/retireSessionTimers, and fsm.Timers.StopAll/Retire; the sentinel assertion strengthens causal isolation rather than replacing any action assertion. Source judgment only; no native execution in this audit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271EstablishedHoldTimerFedByKeepalivesNeverExpires`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L260) | unit/verify | revert, verified |
| negative | [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1007) | unit/verify | unproven |
| positive | [`TestRFC4271EstablishedHoldTimerExpiryRunsTheEvent10List`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L235) | unit/verify | revert, verified |
| positive | [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L935) | unit/verify | unproven |

### [`RFC4271-8.2.2-3`](#rfc4271-8.2.2-3)

If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Read the complete Established Event 10 list in RFC 4271 Section 8.2.2 and all four tagged units. HoldExpiryRunsTheEvent10ActionList requires exact ErrHoldTimerExpired, the HoldTimer and KeepaliveTimer stopped, ConnectRetryTimer stopped and the closing connection's code-4 NOTIFICATION; its no-expiry counterpart retains all armed timers and Established state. EstablishedHoldTimerExpiryRunsTheEvent10List and its KEEPALIVE-fed counterpart exercise the running Peer: requireReleased checks exactly one 4/0, release of the session object and all three session timers, with the healthy control retaining the same session, timers and connection over three hold times. These cover release of the per-connection resources held by these sessions, not merely successful Run completion. The RFC separately names route deletion in neighboring Event 18/24/25 lists; no new claim about a populated RIB is inferred from these empty-route fixtures. Run retires timers and closeConn retires transport, while the FSM transition releases the Peer session. The exact-sentinel edit removes no resource assertion. Source judgment only; no native red or green claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271EstablishedHoldTimerFedByKeepalivesNeverExpires`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L261) | unit/verify | revert, verified |
| negative | [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1009) | unit/verify | unproven |
| positive | [`TestRFC4271EstablishedHoldTimerExpiryRunsTheEvent10List`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L236) | unit/verify | revert, verified |
| positive | [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L938) | unit/verify | unproven |

### [`RFC4271-8.2.2-4`](#rfc4271-8.2.2-4)

If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 4271 Section 8.2.2 Event 10 cumulatively requires code-4 NOTIFICATION, ConnectRetryTimer zero, resource release and dropped TCP. Re-read HoldExpiryRunsTheEvent10ActionList, NoHoldExpiryLeavesTheSessionIntact, EstablishedHoldTimerExpiryRunsTheEvent10List and EstablishedHoldTimerFedByKeepalivesNeverExpires. The Session positive now pins ErrHoldTimerExpired, checks all three timer stops and Idle, requires EOF/closed-pipe rather than timeout via requireConnClosed, and reads code 4 from that same connection before closure. The running-Peer positive strengthens notification identity to exactly one 4/0, checks session release and all timers, and observes EOF; the fed negative keeps the socket/session/timers alive with no NOTIFICATION, while the Session negative rejects a premature close and retains the explicitly armed ConnectRetryTimer. NewSession's expiry callback sends the notification; Run's closeConn/retireSessionTimers and the common connection-close path produce retirement. No proof was replaced by joining the error: the exact expiry sentinel is additional isolation. Native commands were not run.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271EstablishedHoldTimerFedByKeepalivesNeverExpires`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L262) | unit/verify | revert, verified |
| negative | [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1012) | unit/verify | unproven |
| positive | [`TestRFC4271EstablishedHoldTimerExpiryRunsTheEvent10List`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L237) | unit/verify | revert, verified |
| positive | [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L941) | unit/verify | unproven |

### [`RFC4271-8.2.2-5`](#rfc4271-8.2.2-5)

If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1, - (optionally) performs peer oscillation damping if the DampPeerOscillations attribute is set to TRUE, and - changes its state to Idle. (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 4271 Section 8.2.2 Established Event 10 requires the complete NOTIFICATION/zero/release/drop/increment/Idle action list; peer-oscillation damping is optional. All four tagged units were re-read. The Session expiry unit now requires exact ErrHoldTimerExpired as well as stopped Hold/Keepalive/ConnectRetry timers, Idle, a closed connection and code-4 NOTIFICATION; its no-expiry unit retains the timers, connection and Established state. The running-Peer expiry unit's requireReleased observes exactly one 4/0 then EOF, release of the Peer session and all three timers, and ConnectRetryCounter exactly 0 to 1, followed by exact Idle. The KEEPALIVE-fed Peer control runs three hold times with no NOTIFICATION or close, the same session and Established state, and counter exactly zero. fsm's Established EventHoldTimerExpires branch increments once and changes to Idle; Run and connection teardown retire the resources. The older Session unit alone does not assert the counter, but the separately tagged Peer pair does. Damping is not claimed. The sentinel edit strengthens the expiry cause without dropping any clause. No native execution claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271EstablishedHoldTimerFedByKeepalivesNeverExpires`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L263) | unit/verify | revert, verified |
| negative | [`TestRFC4271NoHoldExpiryLeavesTheSessionIntact`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L1014) | unit/verify | unproven |
| positive | [`TestRFC4271EstablishedHoldTimerExpiryRunsTheEvent10List`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L238) | unit/verify | revert, verified |
| positive | [`TestRFC4271HoldExpiryRunsTheEvent10ActionList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L944) | unit/verify | unproven |

### [`RFC4271-8.2.2-7`](#rfc4271-8.2.2-7)

In response to a ManualStart event (Event 1) or an AutomaticStart event (Event 3), the local system: - initializes all BGP resources for the peer connection, - sets ConnectRetryCounter to zero (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. After the R45 split the Event 3 clause is RFC4271-8.2.2-20 ({gap}); this verdict judges the ManualStart (Event 1) clause the row keeps. Positive at peer level: Peer.Start on a dial-only Peer at counter 7 builds a session, dials (the far end reads an OPEN: resources initialized) and reads counter 0; StartWithContext break observed red. Negative: TestRFC4271ConnectRetryCounterSurvivesDampedStart, Event 6 from Idle and Events 1/6 in Connect and Active leave counter 7, so the reset is not spread to other start paths; handleIdle break observed red (recorded by this judge). The row text still reads 'or an AutomaticStart event (Event 3)', which no tag claims.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterSurvivesDampedStart`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L75) | unit/verify | revert, verified |
| positive | [`TestRFC4271ConnectRetryCounterZeroedOnManualStart`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L42) | unit/verify | unproven |
| positive | [`TestRFC4271ManualStartZeroesTheCounterAndDials`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L296) | unit/verify | revert, verified |

### [`RFC4271-8.2.2-20`](#rfc4271-8.2.2-20)

or an AutomaticStart event (Event 3), the local system: - initializes all BGP resources for the peer connection, - sets ConnectRetryCounter to zero (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-06 by an independent auditor who authored none of the changes named; no test or record ran for this judgement. RFC 4271 Section 8.2.2 (Idle): "In response to a ManualStart event (Event 1) or an AutomaticStart event (Event 3), the local system: - initializes all BGP resources for the peer connection, - sets ConnectRetryCounter to zero". runOnce changed in 1e65b6d4bb: its only hunk replaces the egressRouteFilter closure with p.egressGate(session), which touches neither cycleReachedEstablished, operatorStarted nor the start selection and moves that selection up two lines (peer_run.go:438 and :441). The tagged test files and fsm.go show no diff since the verdict's commit. Peer units read: exactly 0 once the next connection dials after an Established session, by Cease and by real HoldTimer expiry, and exactly 1 kept after a cycle that never reached Established, including one following an earlier Established cycle, so a start event fired on every cycle, on none, or from an uncleared flag goes red. The record file is retaken: mutants at lines 438 and 441 match the current source, and the runOnce whole-body revert proves reach only. Bound on question 4: the row also holds "initializes all BGP resources", which the RFC does not enumerate; no unit lists resources, and its only observation here is that the next cycle dials or accepts. Sibling RFC4271-8.2.2-7 judges the same clause for Event 1. Verdict unchanged: enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271AutomaticStartZeroesOnlyFromIdle`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L701) | unit/verify | revert, verified |
| negative | [`TestRFC4271FailedCycleAfterEstablishedKeepsCounter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_automatic_start_peer_test.go#L244) | unit/verify | revert, verified |
| negative | [`TestRFC4271FailedCycleInboundShortenedBackoffKeepsCounter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_automatic_start_peer_test.go#L479) | unit/verify | mutant, verified |
| negative | [`TestRFC4271FailedCycleRestartKeepsCounter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_automatic_start_peer_test.go#L191) | unit/verify | revert, verified |
| positive | [`TestRFC4271AutomaticStartZeroesConnectRetryCounter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L672) | unit/verify | revert, verified |
| positive | [`TestRFC4271AutomaticStartEvent3ZeroesCounter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_automatic_start_peer_test.go#L141) | unit/verify | revert, verified |
| positive | [`TestRFC4271HoldExpiryInboundShortenedBackoffZeroesCounter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_automatic_start_peer_test.go#L497) | unit/verify | mutant, verified |
| positive | [`TestRFC4271PassiveAutomaticStartReconnectZeroesCounter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_automatic_start_peer_test.go#L518) | unit/verify | mutant, verified |

### [`RFC4271-8.2.2-8`](#rfc4271-8.2.2-8)

If a ManualStop event (Event 2) is issued in the OpenSent state, the local system: - sends the NOTIFICATION with a Cease, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - sets the ConnectRetryCounter to zero (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Absorbs retired RFC4271-8.2.2-18 (its first four items). Peer level: shutdownNotify on a running Peer in OpenSent at counter 3 writes exactly one 6/2, EOF, HoldTimer/KeepaliveTimer/ConnectRetryTimer stopped, session dropped, counter 0; the shared OpenSent negative (well-formed OPEN, no stop) keeps the connection, session and counter 3. teardown / handleOpenSent breaks observed red. Moved positives: TestShutdownNotifySendsCeaseFromEveryConnectedState drives OpenSent among OpenConfirm and Established and asserts the exact 6/2 bytes (shutdownNotify break observed red); test/reload/signal-stop-cease.ci is Established-only, so it proves the Established ManualStop sentence, now row RFC4271-8.2.2-23, and is supplementary here, not evidence for the OpenSent sentence. The ConnectRetryTimer clause holds by construction: fsm Timers.StartConnectRetryTimer has no non-test caller, so the peer-level 'not running' assertion guards a future arming, and the HEAD rfc4271_test.go unit arms it and proves StopAll stops it. 2026-10-01 re-judge (BGP c9 judge, wrote none of these units): stale only because TestShutdownNotifySendsCeaseFromEveryConnectedState gained two tag comment lines (8.2.2-22/-23) and signal-stop-cease.ci gained an 8.2.2-23 tag paragraph; no executable or expect line moved. Verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterNotZeroedByIdleManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L130) | unit/verify | unproven |
| negative | [`TestRFC4271OpenSentWellFormedOpenKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L198) | unit/verify | revert, verified |
| positive | [`TestRFC4271ConnectRetryCounterZeroedOnManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L108) | unit/verify | unproven |
| positive | [`TestRFC4271OpenSentManualStopReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestShutdownNotifySendsCeaseFromEveryConnectedState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_shutdown_notify_test.go#L175) | unit/verify | revert, verified |
| positive | [`signal-stop-cease.ci`](https://github.com/ze-software/ze/blob/main/test/reload/signal-stop-cease.ci#L3) | functional/verify | revert, verified |

### [`RFC4271-8.2.2-9`](#rfc4271-8.2.2-9)

If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independently rejudged after the enum-owned claim changed 'default arm' to 'named ignore arm'. Read the full Section 8.2.2 in rfc/full/rfc4271.txt, including the whole Established sentence: 'If the HoldTimer_Expires event occurs (Event 10), the local system: - sends a NOTIFICATION message with the Error Code Hold Timer Expired, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1, - (optionally) performs peer oscillation damping if the DampPeerOscillations attribute is set to TRUE, and - changes its state to Idle.' The OpenSent and OpenConfirm paragraphs prescribe the corresponding action list. All four tagged units were read: fsm/rfc4271_connect_retry_test.go::TestRFC4271ConnectRetryCounterIncrementsOnHoldTimerExpiry checks exactly 3 to 4 and Idle in OpenSent, OpenConfirm and Established; TestRFC4271ConnectRetryCounterQuietOnHealthyEstablishedTraffic checks unchanged counter 3 for the enumerated healthy events and Idle hold expiry. reactor/rfc4271_fsm_teardown_peer_test.go::TestRFC4271OpenSentHoldTimerExpiryReleasesTheConnection arms the session HoldTimer to 150 ms, sends no OPEN and, through requireReleased, asserts exactly one NOTIFICATION 4/0, connection closure, session release, stopped timers and counter 0 to 1. reactor/rfc4271_opensent_error_peer_test.go::TestRFC4271OpenSentWellFormedOpenKeepsTheConnection supplies a valid OPEN before expiry and asserts no NOTIFICATION, live connection, same session and counter 3. Read timer.go::fireHold, session.go::NewSession's shared hold callback and Run cleanup, session_read.go::handleConnectionClose, peer_run.go::runOnce's release defer, and all three FSM hold-expiry arms. The common callback sends 4/0, fires EventHoldTimerExpires and signals errChan; Run consumes that signal and releases the connection and timers. handleIdle's explicit ignore branch preserves the previous result and matches Section 8.2.2: 'Any other event (Events 9-12, 15-28) received in the Idle state does not cause change in the state of the local system.' The saved diff changes no assertion. Evidence boundary: wire-level assertions here are OpenSent, with FSM counter/state coverage in all three states, not three independent wire-level runs. ConnectRetryTimer is already unarmed in this production lifecycle; its stopped-state assertion does not independently demonstrate cancellation of an armed timer. Static call-site search found StartConnectRetryTimer only in tests and its declaration. Stored fireHold/handleOpenSent discrimination records were read, not renewed or verified by this audit; no new green or red execution is claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterQuietOnHealthyEstablishedTraffic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L174) | unit/verify | revert, verified |
| negative | [`TestRFC4271OpenSentWellFormedOpenKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L199) | unit/verify | revert, verified |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnHoldTimerExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L151) | unit/verify | unproven |
| positive | [`TestRFC4271OpenSentHoldTimerExpiryReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L115) | unit/verify | revert, verified |

### [`RFC4271-8.2.2-10`](#rfc4271-8.2.2-10)

If the BGP message header checking (Event 21) or OPEN message checking detects an error (Event 22)(see Section 6.2), the local system: - sends a NOTIFICATION message with the appropriate error code, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independently rejudged after two enum-owned claims changed 'default arm' to 'error arm' and 'named ignore arm'. RFC 4271 Section 8.2.2, OpenSent, states the whole action sentence: 'If the BGP message header checking (Event 21) or OPEN message checking detects an error (Event 22)(see Section 6.2), the local system: - sends a NOTIFICATION message with the appropriate error code, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1, - (optionally) performs peer oscillation damping if the DampPeerOscillations attribute is TRUE, and - changes its state to Idle.' All four tagged units were read. fsm/rfc4271_connect_retry_test.go::TestRFC4271ConnectRetryCounterIncrementsOnHeaderAndOpenErrors checks counter 0 to 1 and Idle for Events 21 and 22 in every non-Idle state; TestRFC4271ConnectRetryCounterNotIncrementedByIdleErrors checks unchanged counter 2, Idle and no error for the listed ignored events. reactor/rfc4271_opensent_error_peer_test.go::TestRFC4271OpenSentErrorReleasesTheConnection isolates an all-zero marker, Length 18, and OPEN version 3, respectively expecting exactly one 1/1, 1/2 or 2/1 NOTIFICATION, connection closure, session release, stopped timers and counter 0 to 1. TestRFC4271OpenSentWellFormedOpenKeepsTheConnection supplies a version-4 AS4 OPEN and asserts no NOTIFICATION for 500 ms, live connection, same session and unchanged counter 3. Read session_read.go::readAndProcessMessage and notifyHeaderErr, session_handlers.go::handleOpen, FSM.Event and all five consuming handlers, session.go::Run and peer_run.go::runOnce cleanup. Event 22 still reaches an incrementing ErrFSMError arm in Established; this supplementary counter assertion is not a claim that Established sends OpenSent's notification code. The isolated invalid inputs and conforming OPEN distinguish acceptance from refusal, and no assertion was changed in the saved diff. Evidence is for the named header/version errors, not an exhaustive enumeration of every OPEN validation failure. ConnectRetryTimer is unarmed in the production lifecycle, so stopped-state assertions preserve that invariant rather than prove armed-timer cancellation. Stored notifyHeaderErr/handleOpenSent discrimination records were inspected only; the changed claim paragraphs still owe the parent's native evidence renewal, including where no old FSM-unit record exists. No new test result or current fingerprint verification is claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterNotIncrementedByIdleErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L236) | unit/verify | revert, verified |
| negative | [`TestRFC4271OpenSentWellFormedOpenKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L197) | unit/verify | revert, verified |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnHeaderAndOpenErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L210) | unit/verify | revert, verified |
| positive | [`TestRFC4271OpenSentErrorReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L125) | unit/verify | revert, verified |

### [`RFC4271-8.2.2-11`](#rfc4271-8.2.2-11)

If the local system receives a TcpConnectionFails event (Event 18) from the underlying TCP or a NOTIFICATION message (Event 25), the local system: - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independently rejudged after the enum-owned claim changed 'default (FSM Error) arm' to 'named FSM Error arm'. RFC 4271 Section 8.2.2, OpenConfirm, states the whole sentence: 'If the local system receives a TcpConnectionFails event (Event 18) from the underlying TCP or a NOTIFICATION message (Event 25), the local system: - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1, - (optionally) performs peer oscillation damping if the DampPeerOscillations attribute is set to TRUE, and - changes its state to Idle.' Read all four tagged units. fsm/rfc4271_connect_retry_test.go::TestRFC4271ConnectRetryCounterIncrementsOnNotification checks Event 25 gives counter 1 and Idle in all non-Idle states, with ErrFSMError specifically in OpenSent and no error elsewhere. TestRFC4271ConnectRetryCounterStepsByExactlyOnePerNotification checks cumulative counts 1 through 10 using the same counter across Established FSM instances; this is supplementary exact-step evidence, not the non-triggering input of the semantic pair. reactor/rfc4271_fsm_teardown_peer_test.go::TestRFC4271OpenConfirmNotificationOrTCPFailureReleasesTheConnection runs separate received-Cease and far-end TCP-close cases, asserting session release, stopped timers and counter 0 to 1; the NOTIFICATION case also reads connection closure and no reply NOTIFICATION. TestRFC4271OpenConfirmKeepaliveKeepsTheConnection supplies the genuinely non-triggering KEEPALIVE, asserts Established, no NOTIFICATION for 500 ms, live connection, same session, running HoldTimer and counter 0. Read session_handlers.go::handleNotification and fsmMessageEvent, session_read.go::handleConnectionClose, session.go::Run, peer_run.go::runOnce cleanup, connect_retry_counter.go::Increment, and the FSM dispatch/handlers. Events 18 and 25 reach separate explicit OpenConfirm arms that each increment once and enter Idle; message handling and Run stop timers and close the connection. The new named OpenSent Event 25 arm preserves the prior ErrFSMError result and does not silently drop the event. Assertions are unchanged in the saved diff. Evidence boundary: the TCP-close case closes the far-end socket itself and does not read that socket afterward, so it is not an independent wire observation of 'no reply'; the row itself requires no reply notification. ConnectRetryTimer starts unarmed in production. Stored handleOpenSent/handleOpenConfirm records were inspected, not renewed; no new green, red or freshness result is claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterStepsByExactlyOnePerNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L293) | unit/verify | unproven |
| negative | [`TestRFC4271OpenConfirmKeepaliveKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L195) | unit/verify | revert, verified |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L262) | unit/verify | revert, verified |
| positive | [`TestRFC4271OpenConfirmNotificationOrTCPFailureReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L154) | unit/verify | revert, verified |

### [`RFC4271-8.2.2-12`](#rfc4271-8.2.2-12)

If the local system receives a NOTIFICATION message (Event 24 or Event 25) or a TcpConnectionFails (Event 18) from the underlying TCP, the local system: - sets the ConnectRetryTimer to zero, - deletes all routes associated with this connection, - releases all the BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 (independent auditor). Row: RFC 4271 Section 8.2.2 Established, Event 24/25/18 list. c987777bb8 changed TestRFC4271ConnectRetryCounterOnVersionErrorPerState in its doc comment only (the Event 24 clause in Connect/Active now said to sit in the DelayOpenTimer-is-not-running branch the timer-disabled crcFSM fixture exercises); the body is unchanged and still requires counter exactly 1 and Idle after NotifMsgVerErr in Connect, Active and Established, while the negative requires OpenSent and OpenConfirm to leave the counter at 0. Peer-level units unchanged: NOTIFICATION 2/1 (Event 24), Cease (Event 25) and far-end close (Event 18) in Established each raise peer-down, drop the session, stop the timers and move the counter 0 -> 1; KEEPALIVE plus a valid UPDATE keep the session (negative). Routes clause: TestRFC4760PeerDownDeletesOnlyThatNeighborsRoutes deletes that neighbor's routes and keeps a bystander's; the dispatcher translation from reactor peer-down to the RIB event is not asserted end to end. Correction to the previous note: fsm Timers.StartConnectRetryTimer now has production callers (b6d84e8104: Connect Event 18 with a running DelayOpenTimer, Active Event 18), and FSM.change stops the ConnectRetryTimer on every Idle entry except Active Event 18; the peer-level 'ConnectRetryTimer not running' assertion therefore guards a reachable timer, not only a future one. Tagged units ran green 2026-10-31 under -race. Verdict unchanged: enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterQuietOnVersionErrorInOpenStates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L342) | unit/verify | unproven |
| negative | [`TestRFC4271EstablishedGoodMessagesKeepTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_established_teardown_peer_test.go#L157) | unit/verify | revert, verified |
| positive | [`TestRFC4271ConnectRetryCounterOnVersionErrorPerState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L318) | unit/verify | mutant, verified |
| positive | [`TestRFC4760PeerDownDeletesOnlyThatNeighborsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4760_peer_down_test.go#L51) | unit/verify | revert, verified |
| positive | [`TestRFC4271EstablishedNotificationOrTCPFailureReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_established_teardown_peer_test.go#L85) | unit/verify | revert, verified |

### [`RFC4271-8.2.2-13`](#rfc4271-8.2.2-13)

If the local system receives a TcpConnectionFails event (Event 18), the local system: - restarts the ConnectRetryTimer (with the initial value), - stops and clears the DelayOpenTimer (sets the value to zero), - releases all BGP resource, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Independent source rejudgment against the complete RFC 4271 Section 8.2.2 Active Event 18 list (rfc/full/rfc4271.txt:3355-3376): restart ConnectRetryTimer with its initial value, stop/clear DelayOpenTimer, release resources, increment ConnectRetryCounter, optional damping, and Idle. DelayOpen is implemented: handleActive first consults handleDelayOpen, Event 17 can retain the connection in Active, Event 18 starts retry and increments the counter, and FSM.change preserves that retry at its Idle transition while clearing DelayOpen. The corrected gap annotation names the actual remaining producer defect: Session.handleConnectionCloseLocked delivers Event 18 and then immediately calls timers.StopAll, cancelling the newly restarted ConnectRetryTimer; Peer.run uses its separate exponential reconnect backoff rather than preserving the initial-value retry. No test tags this row. Untagged TestRFC4271ActiveEvent18WithDelayOpen reaches the real passive Peer, checking Idle, counter increment, nil connection and stopped DelayOpen, but never checks live retry; the FSM-only DelayOpenEventsActive test checks the retry before Session teardown. Thus the full producer's restart clause is unmet, not merely an absent tag and not an absent optional feature. Verdict unimplemented. Existing owner is spec-bgp-fsm-delayopen-and-automatic-start AC-1e; the mechanism-specific open finding is recorded in plan/journal/lifetime-ends-before-its-last-reader.md dated 2026-11-04 and its owned journal rows table. This source finding has not been runtime reproduced here.

No test carries RFC4271-8.2.2-13, so no unit is bound to it.

### [`RFC4271-8.2.2-14`](#rfc4271-8.2.2-14)

If the local system receives an UPDATE message, and the UPDATE message error handling procedure (see Section 6.3) detects an error (Event 28), the local system: - sends a NOTIFICATION message with an Update error, - sets the ConnectRetryTimer to zero, - deletes all routes associated with this connection, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Peer level in Established: an UPDATE with a malformed MP_REACH_NLRI (an error RFC 7606 still answers with a session reset, Event 28) draws exactly one NOTIFICATION with Error Code 3, EOF, the peer-down for that neighbor, the session dropped, three timers stopped, counter 0 -> 1; the RIB unit proves the routes deletion on that peer-down. Negative: a well-formed UPDATE is dispatched with no peer-down, same session Established, counter 0. Records observed red: handleEstablished (positive), handleStructuredState (rib positive), handleUpdate (negative). Subcode not asserted (the row asks for 'an Update error', the code). Not asserted end to end: the plugin dispatcher's peer-down translation into the RIB event (same caveat as RFC4760-7-1). The ConnectRetryTimer clause holds by construction: fsm Timers.StartConnectRetryTimer has no non-test caller, so the peer-level 'not running' assertion guards a future arming, and the HEAD rfc4271_test.go unit arms it and proves StopAll stops it. 2026-10-01 re-judge (BGP c9 judge, wrote none of these units): stale only because TestRFC4271EstablishedUpdateErrorReleasesTheConnection gained one RFC4271-6.3-1 tag comment line; no executable line moved. Verdict unchanged. Re-read 2026-10-01 (c10 judge): the only change under this verdict is a tag comment line for another id (RFC4271-6.7-1 / RFC4271-Security-1 added, RFC4271-9.1.2.1-1 removed) in the unit's doc comment; every assertion is byte-identical, so the judgement stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterQuietOnGoodUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L441) | unit/verify | unproven |
| negative | [`TestRFC4271EstablishedGoodMessagesKeepTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_established_teardown_peer_test.go#L158) | unit/verify | revert, verified |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnUpdateError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L421) | unit/verify | unproven |
| positive | [`TestRFC4760PeerDownDeletesOnlyThatNeighborsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4760_peer_down_test.go#L55) | unit/verify | revert, verified |
| positive | [`TestRFC4271EstablishedUpdateErrorReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_established_teardown_peer_test.go#L129) | unit/verify | revert, verified |

### [`RFC4271-8.2.2-15`](#rfc4271-8.2.2-15)

In response to any other events (Events 8, 10-11, 13, 19, 23, 25-28), the local system: - if the ConnectRetryTimer is running, stops and resets the ConnectRetryTimer (sets to zero), - if the DelayOpenTimer is running, stops and resets the DelayOpenTimer (sets to zero), - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Re-judged 2026-10-31 (independent auditor). RFC 4271 Section 8.2.2 (Connect): "In response to any other events (Events 8, 10-11, 13, 19, 23, 25-28), the local system: - if the ConnectRetryTimer is running, stops and resets the ConnectRetryTimer (sets to zero), - if the DelayOpenTimer is running, stops and resets the DelayOpenTimer (sets to zero), - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1, - performs peer oscillation damping if the DampPeerOscillations attribute is set to True, and - changes its state to Idle." (rfc/full/rfc4271.txt lines 3226-3244) Downgraded from enforced to weak because the code moved under the previous judgement, not the tests: the previous verdict rested on 'Ze has no DelayOpenTimer and never starts the ConnectRetryTimer, so both conditional clauses never trigger'. b6d84e8104 implemented configurable DelayOpen: in Connect, Event 17 with DelayOpen set now holds the TCP connection with a running DelayOpenTimer (handleDelayOpen), and Connect Event 18 with a running DelayOpenTimer starts the ConnectRetryTimer. The DelayOpenTimer clause, and dropping the held TCP connection, are now reachable, and no tagged unit runs with a DelayOpenTimer: the FSM units use crcFSM with no timers installed, and the peer units (Connect Event 8 during the dial, Event 1 negative) configure no DelayOpen. FSM.change stops the DelayOpenTimer on every Idle entry, so the code complies today, but removing that stop would leave every tagged unit green. What the tagged units still enforce: counter 3 -> 4 and Idle for Events 8, 10, 11, 19, 23, 25-28 in Connect (and Active, supplementary), Events 1/6/9 keeping state and counter, and at peer level the dial-time Event 8 release (no OPEN, conn nil, timers stopped, counter 4). Fix: tag internal/component/bgp/fsm/delay_open_test.go::TestRFC4271DelayOpenEventsConnect (untagged today; it drives each listed event in Connect with a running DelayOpenTimer and requires Idle, counter 4, DelayOpenTimer and ConnectRetryTimer not running) as a positive of this row with a discrimination record that removes FSM.change's StopDelayOpenTimer, and add a peer-level held-connection case for the TCP-drop clause. Tagged units ran green 2026-10-31 under -race.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectActiveUnlistedEventKeepsTheState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_any_other_event_test.go#L78) | unit/verify | revert, verified |
| negative | [`TestRFC4271ConnectRetryCounterIdleDefaultArmCountsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L498) | unit/verify | unproven |
| negative | [`TestRFC4271ActiveDuplicateStartKeepsThePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_active_any_other_event_peer_test.go#L98) | unit/verify | revert, verified |
| negative | [`TestRFC4271ConnectDuplicateStartDuringTheDialKeepsThePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_connect_any_other_event_peer_test.go#L165) | unit/verify | revert, verified |
| positive | [`TestRFC4271ConnectActiveAnyOtherEventCountsAndDropsToIdle`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_any_other_event_test.go#L44) | unit/verify | mutant, verified |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnAnyOtherEvent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L463) | unit/verify | unproven |
| positive | [`TestRFC4271ActiveAutomaticStopReleasesThePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_active_any_other_event_peer_test.go#L62) | unit/verify | revert, verified |
| positive | [`TestRFC4271ConnectAutomaticStopDuringTheDialReleasesThePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_connect_any_other_event_peer_test.go#L126) | unit/verify | revert, verified |

### [`RFC4271-8.2.2-21`](#rfc4271-8.2.2-21)

In response to any other event (Events 8, 10-11, 13, 19, 23, 25-28), the local system: - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by one (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Active 'any other event' list (verbatim span, R47(b) row). Peer level, running passive Peer in Active at counter 3: teardownAutomatic (Event 8) -> Idle, counter 4, HoldTimer/KeepaliveTimer/ConnectRetryTimer not running, Peer drops the session, a later TCP connection is refused with ErrSessionTearingDown and never held, counter stays 4. Negative: Event 1 (not in the list) leaves Active, counter 3 for 300 ms, same unsealed session. handleActive break observed red both polarities. Clause notes: 'sets the ConnectRetryTimer to zero' holds by construction (Timers.StartConnectRetryTimer has no non-test caller), so the 'not running' assertion guards a future arming. 'drops the TCP connection': Ze's Active holds no TCP connection, so the proof is that a TCP connection offered afterwards is refused and never becomes the session's; the FSM-level unit for 8.2.2-15 (TestRFC4271ConnectActiveAnyOtherEventCountsAndDropsToIdle) also drives Events 10, 11, 19, 23, 25-28 in Active but is not tagged here, so only Event 8 is proven at peer level for this row. Negative uses a start event (pair holds: Event 1 is ignored, not this list).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ActiveDuplicateStartKeepsThePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_active_any_other_event_peer_test.go#L99) | unit/verify | revert, verified |
| positive | [`TestRFC4271ActiveAutomaticStopReleasesThePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_active_any_other_event_peer_test.go#L63) | unit/verify | revert, verified |

### [`RFC4271-8.2.2-22`](#rfc4271-8.2.2-22)

In response to a ManualStop event (Event 2) initiated by the operator, the local system: - sends the NOTIFICATION message with a Cease, - releases all BGP resources, - drops the TCP connection, - sets the ConnectRetryCounter to zero, - sets the ConnectRetryTimer to zero (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. OpenConfirm ManualStop list (verbatim span). Peer level: running Peer in OpenConfirm at counter 3, shutdownNotify -> exactly one NOTIFICATION 6/2 on the wire, EOF, HoldTimer/KeepaliveTimer/ConnectRetryTimer stopped, Peer drops the session, Idle, counter 0 (every clause). Supplementary: TestShutdownNotifySendsCeaseFromEveryConnectedState pins the exact 6/2 octets from OpenConfirm. Negative: same harness at counter 3, a KEEPALIVE and no ManualStop -> no NOTIFICATION in 500 ms, connection and session kept, Established, counter 3. Records observed red: shutdownNotify (both positives), handleOpenConfirm (negative). ConnectRetryTimer clause holds by construction (no non-test caller arms it).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OpenConfirmWithoutManualStopKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_manualstop_peer_test.go#L52) | unit/verify | revert, verified |
| positive | [`TestRFC4271OpenConfirmManualStopReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_manualstop_peer_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestShutdownNotifySendsCeaseFromEveryConnectedState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_shutdown_notify_test.go#L183) | unit/verify | revert, verified |

### [`RFC4271-8.2.2-23`](#rfc4271-8.2.2-23)

In response to a ManualStop event (initiated by an operator) (Event 2), the local system: - sends the NOTIFICATION message with a Cease, - sets the ConnectRetryTimer to zero, - deletes all routes associated with this connection, - releases BGP resources, - drops the TCP connection, - sets the ConnectRetryCounter to zero (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Established ManualStop list (verbatim span). Peer level: startMPLinkNeighbor at counter 3, shutdownNotify -> peer-down for that neighbor (the event on which the RIB deletes the connection's routes; same RIB caveat as 8.2.2-12/-14: the dispatcher translation is not end to end), exactly one 6/2 then EOF, session dropped, three timers stopped, counter 0. Whole daemon: test/reload/signal-stop-cease.ci, SIGTERM on an Established session, seq=3 expect line is the exact Cease 6/2 (functional record with citation observed red on shutdownNotify). Every-state unit pins 6/2 from Established. Negative: KEEPALIVE with no ManualStop at counter 3 -> no peer-down, no EOF in 300 ms, Established, same session, Hold/Keepalive timers running, counter 3 (handleKeepalive break observed red). The negative does not assert the absence of a NOTIFICATION without EOF, a residual the positive's 'exactly one then EOF' bounds.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271EstablishedWithoutManualStopKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_manualstop_peer_test.go#L126) | unit/verify | revert, verified |
| positive | [`TestRFC4271EstablishedManualStopReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_manualstop_peer_test.go#L83) | unit/verify | revert, verified |
| positive | [`TestShutdownNotifySendsCeaseFromEveryConnectedState`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_shutdown_notify_test.go#L184) | unit/verify | revert, verified |
| positive | [`signal-stop-cease.ci`](https://github.com/ze-software/ze/blob/main/test/reload/signal-stop-cease.ci#L8) | functional/verify | revert, verified |

### [`RFC4271-8.2.2-24`](#rfc4271-8.2.2-24)

In response to any other event (Events 9, 11-13, 20, 25-28), the local system: - sends the NOTIFICATION with the Error Code Finite State Machine Error, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-01 (BGP c12 judge). Positive TestRFC4271OpenSentUnexpectedMessageIsAnFSMError proves the whole list at peer level for the four listed events Ze can receive in OpenSent (Cease NOTIFICATION = Event 25, KEEPALIVE 26, empty UPDATE 27, malformed UPDATE 28): exactly one 5/0, EOF, three timers stopped, session released, counter 0->1. Negatives: TestRFC4271ExpectedMessagesRaiseNoFSMError (OPEN, Event 19, draws nothing) and the c11-owed list-boundary negative, now present: TestRFC4271VersionErrorNotificationReleasesQuietly/OPENSENT, a 2/1 NOTIFICATION (Event 24, not on the list) draws no 5/0 and leaves the counter at 3. That negative was red before the D-9 fix (handleNotification filed every NOTIFICATION as Event 25) and is green on notificationEvent; its record is observed red on Session.notificationEvent, the others on fsmMessageEvent. Events 9, 11, 12, 13, 20 do not occur in Ze's OpenSent (corrections). The c11 weak reason is resolved at the producer.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ExpectedMessagesRaiseNoFSMError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_error_peer_test.go#L128) | unit/verify | revert, verified |
| negative | [`TestRFC4271VersionErrorNotificationReleasesQuietly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_version_error_peer_test.go#L57) | unit/verify | revert, verified |
| positive | [`TestRFC4271OpenSentUnexpectedMessageIsAnFSMError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_error_peer_test.go#L58) | unit/verify | revert, verified |

### [`RFC4271-8.2.2-25`](#rfc4271-8.2.2-25)

In response to any other event (Events 9, 12-13, 20, 27-28), the local system: - sends a NOTIFICATION with a code of Finite State Machine Error, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-10-01 (BGP c11 judge). Positive TestRFC4271OpenConfirmUpdateIsAnFSMError: a running Peer in OpenConfirm receiving an empty UPDATE (Event 27) or an UPDATE whose Withdrawn Routes Length overruns the message (Event 28) writes exactly one NOTIFICATION 5/0, drops TCP (EOF), stops ConnectRetry/Hold/Keepalive timers, releases the session, counter 0->1: every listed action. The producer (processMessage -> updateIsUnexpected -> fsmMessageEvent) intercepts the UPDATE before RFC 7606 parsing and plugin delivery. Negative TestRFC4271ExpectedMessagesRaiseNoFSMError: KEEPALIVE in OpenConfirm (Event 26, not listed) reaches Established with nothing written, connection/session kept, counter 0; the same UPDATE in Established draws nothing, so the state decides. Events 9, 12-13, 20 do not occur in Ze's OpenConfirm (no ConnectRetryTimer running, no DelayOpen, no IdleHoldTimer). Records observed red on updateIsUnexpected both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ExpectedMessagesRaiseNoFSMError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_error_peer_test.go#L129) | unit/verify | revert, verified |
| positive | [`TestRFC4271OpenConfirmUpdateIsAnFSMError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_error_peer_test.go#L95) | unit/verify | revert, verified |

### [`RFC4271-8.2.2-26`](#rfc4271-8.2.2-26)

If a NOTIFICATION message is received with a version error (Event 24), the local system: - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, and - changes its state to Idle. (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-10-01 (BGP c12 judge). Row is the OpenSent Event 24 list verbatim (RFC 4271 p.66, 'If a NOTIFICATION message is received with a version error (Event 24), the local system: - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, and - changes its state to Idle.'). Positive TestRFC4271VersionErrorNotificationReleasesQuietly/OPENSENT: a running Peer at ConnectRetryCounter 3 receives 2/1 and writes NO NOTIFICATION (exact empty list), far end reads EOF, ConnectRetry/Hold/Keepalive timers stopped, session released, FSM Idle, counter stays 3 (the list has no counter line; the start value 3 makes an increment or a reset visible). Negative TestRFC4271OtherOpenErrorNotificationIsNotAVersionError/OPENSENT: 2/2 (same code, other subcode, so not the Event 24 trigger) takes Event 25: exactly one 5/0 and counter 3->4, so a classifier keyed on the error code alone goes red. Producer: Session.notificationEvent (Event 24 only for 2/1, the Section 6.2 Unsupported Version Number) and FSM.handleOpenSent's own EventNotifMsgVerErr arm. Records observed red on notificationEvent, both polarities; the author's failing-first run showed 5/0 and 3->4 before the fix (D-9).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OtherOpenErrorNotificationIsNotAVersionError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_version_error_peer_test.go#L84) | unit/verify | revert, verified |
| positive | [`TestRFC4271VersionErrorNotificationReleasesQuietly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_version_error_peer_test.go#L55) | unit/verify | revert, verified |

### [`RFC4271-8.2.2-27`](#rfc4271-8.2.2-27)

If the local system receives a NOTIFICATION message with a version error (NotifMsgVerErr (Event 24)), the local system: - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, and - changes its state to Idle. (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-10-01 (BGP c12 judge). Row is the OpenConfirm Event 24 list verbatim (RFC 4271 Section 8.2.2, 'If the local system receives a NOTIFICATION message with a version error (NotifMsgVerErr (Event 24)), the local system: ... and - changes its state to Idle.'). Positive TestRFC4271VersionErrorNotificationReleasesQuietly/OPENCONFIRM: 2/1 at counter 3 draws no NOTIFICATION, EOF, three timers stopped, session released, Idle, counter stays 3. Negative TestRFC4271OtherOpenErrorNotificationIsNotAVersionError/OPENCONFIRM: 2/2 is Event 25 (the Event 18/25 list): no NOTIFICATION and counter 3->4, so the counter is the discriminating clause between the two lists and both directions are pinned. Records observed red on notificationEvent both polarities; before the fix the positive moved the counter to 4 (D-9). Supplementary: fsm.TestRFC4271ConnectRetryCounterQuietOnVersionErrorInOpenStates proves the same counter half at FSM level but stays tagged RFC4271-8.2.2-12 negative (a valid state-boundary negative there).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OtherOpenErrorNotificationIsNotAVersionError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_version_error_peer_test.go#L85) | unit/verify | revert, verified |
| positive | [`TestRFC4271VersionErrorNotificationReleasesQuietly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_version_error_peer_test.go#L56) | unit/verify | revert, verified |

### [`RFC4271-8.2.2-28`](#rfc4271-8.2.2-28)

In response to any other event (Events 9, 12-13, 20-22), the local system: - sends a NOTIFICATION message with the Error Code Finite State Machine Error, - deletes all routes associated with this connection, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-10-01 (BGP c12 judge) under R49(a) and the dated correction: for Event 21 the Section 6.1 Message Header Error code governs the NOTIFICATION, the Section 8.2.2 Established list governs the teardown. Events 9, 12, 13, 20 cannot occur in Ze's Established, and Event 22 does not either (handleOpen answers any OPEN in Established with a Cease before checking it), so the claim covers Event 21 only and says so. Positive TestRFC4271EstablishedHeaderErrorReleasesTheConnection, subtests Marker (all-zero Marker, isolated: Length 19, Type KEEPALIVE) and Length (all-ones Marker, Length 18): exactly one NOTIFICATION 1/1 resp. 1/2, EOF, peer-down raised to the Reactor observers (the route-deletion trigger), session released, three timers stopped, counter 0->1. Negative TestRFC4271EstablishedValidHeaderKeepsTheConnection: a well-formed KEEPALIVE (Length 19, the Section 4.1 floor) gives no peer-down, same session, Established, counter 0. Records: positive on session_read.go::notifyHeaderErr, negative on message/header.go::ParseHeader, both observed red. The routes clause is proven to the peer-down raise; the RIB's deletion on that peer-down is proven by TestRFC4760PeerDownDeletesOnlyThatNeighborsRoutes (tagged RFC4271-8.2.2-12), the same chain 8.2.2-12 is judged on. 'FSM Error' itself is not asserted, by the R49(a) reading.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271EstablishedValidHeaderKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_established_header_error_peer_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestRFC4271EstablishedHeaderErrorReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_established_header_error_peer_test.go#L31) | unit/verify | revert, verified |

### [`RFC4271-8.2.2-16`](#rfc4271-8.2.2-16)

If an AutomaticStop event (Event 8) is issued in the OpenSent state, the local system: - sends the NOTIFICATION with a Cease, - sets the ConnectRetryTimer to zero, - releases all the BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-10-01 (c8 judge): the only change under this verdict is the deleted RFC4271-8.2.2-18 tag line in TestRFC4271OpenSentWellFormedOpenKeepsTheConnection's doc comment (8.2.2-18 retired into 8.2.2-8); the unit's body and assertions are unchanged. Peer level: teardownAutomatic (Event 8) in OpenSent writes exactly one 6/8, EOF, three timers stopped, session dropped, counter 0 -> 1; the shared negative sends no Cease and keeps counter 3. teardownAutomatic / handleOpenSent breaks observed red. Driven at the producer the BFD-down and forward-pool paths call, not through those triggers. The ConnectRetryTimer clause holds by construction: fsm Timers.StartConnectRetryTimer has no non-test caller, so the peer-level 'not running' assertion guards a future arming, and the HEAD rfc4271_test.go unit arms it and proves StopAll stops it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterAutomaticStopIsNotAManualStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L595) | unit/verify | unproven |
| negative | [`TestRFC4271OpenSentWellFormedOpenKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L200) | unit/verify | revert, verified |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnAutomaticStop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L569) | unit/verify | unproven |
| positive | [`TestRFC4271OpenSentAutomaticStopReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L76) | unit/verify | revert, verified |

### [`RFC4271-8.2.2-17`](#rfc4271-8.2.2-17)

If a connection in the OpenSent state is determined to be the connection that must be closed, an OpenCollisionDump (Event 23) is signaled to the state machine. If such an event is received in the OpenSent state, the local system: - sends a NOTIFICATION with a Cease, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1 (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-10-01 (c8 judge): the only change under this verdict is the deleted RFC4271-8.2.2-18 tag line in TestRFC4271OpenSentWellFormedOpenKeepsTheConnection's doc comment (8.2.2-18 retired into 8.2.2-8); the unit's body and assertions are unchanged. D-8 fixed: CloseWithNotification now sets ErrCollisionDump and signals errChan, so the Run loop no longer outlives the connection. Peer level in OpenSent: exactly one 6/7, EOF, three timers stopped, session dropped, counter exactly 1; shared negative keeps counter 3. CloseWithNotification break re-recorded after the fix, observed red. The ConnectRetryTimer clause holds by construction: fsm Timers.StartConnectRetryTimer has no non-test caller, so the peer-level 'not running' assertion guards a future arming, and the HEAD rfc4271_test.go unit arms it and proves StopAll stops it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ConnectRetryCounterCollisionDumpIsQuietInIdle`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L650) | unit/verify | unproven |
| negative | [`TestRFC4271OpenSentWellFormedOpenKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_opensent_error_peer_test.go#L204) | unit/verify | revert, verified |
| positive | [`TestRFC4271ConnectRetryCounterIncrementsOnOpenCollisionDump`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_connect_retry_test.go#L626) | unit/verify | unproven |
| positive | [`TestRFC4271OpenSentCollisionDumpReleasesTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_fsm_teardown_peer_test.go#L96) | unit/verify | revert, verified |

### [`RFC4271-10-1`](#rfc4271-10-1)

An implementation of BGP MUST allow the HoldTimer to be configurable on a per-peer basis (§10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-01 (c10 judge). The operator entry point is now exercised: TestRFC4271HoldTimeConfiguredPerPeerFromConfig parses two peer trees through parsePeerFromTree with `timer { receive-hold-time }` 30 and 240 and each NewSession's HoldTimer holds its own value; negative TestRFC4271HoldTimeUnconfiguredPeerKeepsTheDefault: a third peer with no timer container keeps DefaultReceiveHoldTime (guarded against equalling a configured value) and `receive-hold-time 2` is refused. Both records observed red on parsePeerFromTree. HEAD PeerSettings-level units stay supplementary (timers + negotiation).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271HoldTimeUnconfiguredPeerKeepsTheDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_hold_time_config_test.go#L75) | unit/verify | revert, verified |
| negative | [`TestRFC4271PerPeerHoldTimeSurvivesNegotiation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L283) | unit/verify | unproven |
| positive | [`TestRFC4271HoldTimeConfiguredPerPeerFromConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_hold_time_config_test.go#L53) | unit/verify | revert, verified |
| positive | [`TestRFC4271HoldTimeConfigurablePerPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L259) | unit/verify | unproven |

### [`RFC4271-5-7`](#rfc4271-5-7)

The sender of an UPDATE message SHOULD order path attributes within the UPDATE message in ascending order of attribute type. (§5) Context: RFC 7606 Section 5.1, which updates RFC 4271, says the MP_REACH_NLRI or MP_UNREACH_NLRI attribute SHALL be encoded as the very first path attribute; the ascending order applies to the attributes after it.

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-20 (independent auditor, authored none of the change). RFC 4271 Section 5: "The sender of an UPDATE message SHOULD order path attributes within the UPDATE message in ascending order of attribute type." RFC 7606 (header: "Updates: 1997, 4271, ...") Section 5.1: "The MP_REACH_NLRI or MP_UNREACH_NLRI attribute (if present) SHALL be encoded as the very first path attribute in an UPDATE message." f29e582ed4 changes the row text only: the verbatim Section 5 quote is unchanged and a Context note after (§5) says RFC 7606 Section 5.1, which updates RFC 4271, puts MP_REACH_NLRI or MP_UNREACH_NLRI first and the ascending order applies after it. Checked against rfc/full/rfc7606.txt: RFC 7606 lists 4271 in Updates, and the note paraphrases the 5.1 bullet faithfully (it omits "(if present)", which does not change what it asserts, and adds no obligation). The note answers the finding of the previous judgement and now matches what the tagged units assert (requireAscendingAttributes skips a leading code 14 then requires strictly ascending codes; TestSplitMP_PreservesAscendingAttributeOrder and the reactor rails assert exact sequences after a leading 14/15). No code, test or claim changed; the claims already said "after any leading MP_REACH_NLRI", so no claim exceeds its assertion. Scope unchanged: originating builders and split preservation. Verdict stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271SentAttributesAscendEvenWithRawConfigAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_attr_order_send_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestRFC4271SentAttributesAscendEvenWithRawConfigAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_attr_order_send_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestSplitMP_PreservesAscendingAttributeOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_update_split_attr_order_test.go#L75) | unit/verify | revert, verified |
| positive | [`TestAnnounceBatchRail_AS4PathOrderedAgainstLargeCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_api_batch_attr_order_test.go#L335) | unit/verify | unproven |
| positive | [`TestAnnounceBatchRail_AscendingTypeCodeOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_api_batch_attr_order_test.go#L275) | unit/verify | unproven |
| positive | [`TestAnnounceQueuedRail_AscendingTypeCodeOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_api_batch_attr_order_test.go#L296) | unit/verify | revert, verified |

### [`RFC4271-6.3-2`](#rfc4271-6.3-2)

If the NEXT_HOP attribute is semantically incorrect, the error SHOULD be logged, and the route SHOULD be ignored. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-01 (BGP c15 judge). Both clauses proven at their producers. 'the route SHOULD be ignored': TestRFC4271SemanticallyIncorrectNextHopIsLoggedAndIgnored and TestRFC4271SemanticallyIncorrectNextHopIsLoggedAtTheDefaultLevel drive a live session read over a one-hop EBGP session; NEXT_HOP = the receiving speaker (criterion a) and NEXT_HOP off the shared subnet (criterion b) leave NLRI empty and move 203.0.113.0/24 to the withdrawn routes, an on-subnet NEXT_HOP is delivered. 'the error SHOULD be logged' (the c14 gap, D-8 fixed): session_next_hop.go::logIgnoredNextHopRoute, called in session_read.go processMessage at the invalid-next-hop branch with the Section 6.3 quote above it, writes 'route ignored: semantically incorrect NEXT_HOP' at Warn, the slogutil default level; the new unit captures the session logger at slog.LevelWarn and asserts level=WARN, peer=192.0.2.1 and next-hop=<the NEXT_HOP> for both invalid cases, and no such record for the valid one. Records: revert on logIgnoredNextHopRoute (+) and invalidReceiveNextHop (-); the author's c15-63red shows the new unit red before the fix. Residual, not a verdict defect: there is NO rate limiting (Ze has none to reuse), so a peer repeating such routes writes one WARN per UPDATE; the RFC asks only that the error be logged. The rib unit TestRFC4271SelfNextHopRouteIsNotInstalled still has no record (unproven).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271SemanticallyIncorrectNextHopIsLoggedAndIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_nexthop_semantic_test.go#L52) | unit/verify | revert, verified |
| negative | [`TestRFC4271SemanticallyIncorrectNextHopIsLoggedAtTheDefaultLevel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_nexthop_semantic_test.go#L94) | unit/verify | revert, verified |
| positive | [`TestRFC4271SelfNextHopRouteIsNotInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_self_nexthop_test.go#L52) | unit/verify | unproven |
| positive | [`TestRFC4271SemanticallyIncorrectNextHopIsLoggedAndIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_nexthop_semantic_test.go#L51) | unit/verify | revert, verified |
| positive | [`TestRFC4271SemanticallyIncorrectNextHopIsLoggedAtTheDefaultLevel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_nexthop_semantic_test.go#L93) | unit/verify | revert, verified |

### [`RFC4271-3.1-2`](#rfc4271-3.1-2)

The next hop for each of these routes MUST be resolvable via the local BGP speaker's Routing Table. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-3.1-2, so no unit is bound to it.

### [`RFC4271-5.1.2-2`](#rfc4271-5.1.2-2)

a) When a given BGP speaker advertises the route to an internal peer, the advertising speaker SHALL NOT modify the AS_PATH attribute associated with the route. (§5.1.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Re-judged 2026-11-06 by an independent auditor who authored none of the changes; no test or record ran for this judgement, statements about what reddens are by reading unless called observed. RFC 4271 Section 5.1.2: "When a given BGP speaker advertises the route to an internal peer, the advertising speaker SHALL NOT modify the AS_PATH attribute associated with the route." The stale rail gap an earlier auditor named is closed: TestRFC4271StaleReadvertiseASPathEditsSkipInternalPeers drives decideStaleReadvertise on the opted-filter and whole-chain runs, requires the filter was asked, staleKeep and the body byte for byte toward an internal and an RFC 7705 migration-internal destination, and staleModify with the exact edited path toward the same peer AS with no migration AS. Its recorded mutant (stale_readvertise_chain.go:159, asPathEditable true) was observed red in all four subtests (scratch rec-new.log). Its bound: the unit builds dest itself, so dropping IsIBGP: peer.IsIBGP() in sendStaleReadvertiseUnit reddens no tagged unit. WEAK on question 4, one rail still short of the whole requirement: the route-server rail. reactorForwardRSSection hands the export chain !facts.isEBGP (forward_rs.go:473, then libraryASPathOps) and gates as-override and the prepend on facts.isEBGP (forward_rs.go:577). Internal destinations reach this rail: its selection holds no internal or RS-client test, and TestRFC4271ForwardAddsLocalPrefTowardInternalPeer delivers to one with rs true. Every forward unit tagged here passes rs false (TestRFC4271ForwardASPathUnmodifiedTowardInternalPeer, TestRFC4271ForwardExportASPathEditsSkipInternalPeers), and the others call the chain, the writers or the dry run directly, so changing either guard reddens no tagged unit. The code is guarded today, so this is a missing proof, not a defect. What holds: both forward units assert exact bytes with an external control in one fan-out; TestRFC4271ExportPrependNeverModifiesAnInternalPeersASPath and TestRFC4271PolicyDryRunAgreesNoASPathEditTowardAnInternalPeer are exact with controls; TestRFC4271ExportASPathEditsFollowTheMigrationAwareInternalVerdict changed in its doc comment only and the new claim is accurate. Records on the other units are whole-body panics, proving reach only, and the batch and rfc4271_test.go tags carry none. The rfc4271_test.go citations are true: reactor_wire.go:513-522 is the AS_PATH switch through writeASPathAttr in writeAnnounceUpdate. Those two units build a route with no path, so they assert the origination sentence of Section 5.1.2 ("the originating speaker includes an empty AS_PATH attribute in all UPDATE messages sent to internal peers"), not non-modification of a propagated path: correct assertions, misattributed. To reach enforced: tag a unit on reactorForwardRS, as-override and as-path-prepend and remove-private toward internal and migration-internal destinations with an external control, exact AS_PATH asserted, and record a mutant on each of forward_rs.go:473 and :577.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ExportASPathEditsFollowTheMigrationAwareInternalVerdict`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_ibgp_aspath_export_test.go#L167) | unit/verify | revert, verified |
| negative | [`TestRFC4271ExportPrependNeverModifiesAnInternalPeersASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_ibgp_aspath_export_test.go#L92) | unit/verify | revert, verified |
| negative | [`TestRFC4271ForwardExportASPathEditsSkipInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_ibgp_aspath_export_test.go#L312) | unit/verify | revert, verified |
| negative | [`TestRFC4271PolicyDryRunAgreesNoASPathEditTowardAnInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_ibgp_aspath_export_test.go#L234) | unit/verify | revert, verified |
| negative | [`TestRFC4271ForwardASPathUnmodifiedTowardInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_a2_forward_test.go#L94) | unit/verify | revert, verified |
| negative | [`TestRFC4271StaleReadvertiseASPathEditsSkipInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_stale_aspath_export_test.go#L77) | unit/verify | mutant, verified |
| negative | [`TestRFC4271ASPathPrependedTowardExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L146) | unit/verify | unproven |
| positive | [`TestRFC4271ForwardASPathUnmodifiedTowardInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_a2_forward_test.go#L93) | unit/verify | revert, verified |
| positive | [`TestEstablishedAnnounce_ExplicitASPath_IBGPVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_batch_test.go#L569) | unit/verify | unproven |
| positive | [`TestRFC4271ASPathUnmodifiedTowardInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L129) | unit/verify | unproven |

### [`RFC4271-5.1.2-3`](#rfc4271-5.1.2-3)

When a given BGP speaker advertises the route to an external peer, the advertising speaker updates the AS_PATH attribute as follows: 1) if the first path segment of the AS_PATH is of type AS_SEQUENCE, the local system prepends its own AS number as the last element of the sequence (put it in the leftmost position with respect to the position of octets in the protocol message). If the act of prepending will cause an overflow in the AS_PATH segment (i.e., more than 255 ASes), it SHOULD prepend a new segment of type AS_SEQUENCE and prepend its own AS number to this new segment. 2) if the first path segment of the AS_PATH is of type AS_SET, the local system prepends a new path segment of type AS_SEQUENCE to the AS_PATH, including its own AS number in that segment. 3) if the AS_PATH is empty, the local system creates a path segment of type AS_SEQUENCE, places its own AS into that segment, and places that segment into the AS_PATH. (§5.1.2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/le/interoplab/bgp/check_rfc.go::checkRelayWithdrawalShape, internal/le/interoplab/bgp/check_rfc.go::checkRelayWithdrawalShape#2 moved. Independent source rejudgment against committed source b4215935acd576856c4ddfb9160d1d67a6860fd8 after concurrent commit cd0b42213c added TestForwardWithdrawalDoesNotCreateASPath and its native discrimination record. No unrelated working-tree checker edits were included. RFC 4271 Section 5.1.2(b): "When a given BGP speaker advertises the route to an external peer, the advertising speaker updates the AS_PATH attribute as follows:" The following three cases require a local-AS prepend to a leading AS_SEQUENCE, a new leading AS_SEQUENCE before AS_SET, or creation of an AS_SEQUENCE for an empty path, with a new segment recommended on overflow. Section 4.3 separately states: "An UPDATE message might advertise only routes that are to be withdrawn from service, in which case the message will not include path attributes or Network Layer Reachability Information." The existing positive tests cover the actual established announce path with an explicit leading sequence; the ASPathEdit leading-AS_SET and full-255-member sequence branches; and present zero-length AS_PATH through cached and route-server forwarding at both negotiated ASN widths. TestForwardPresentEmptyASPath requires exactly one output, unchanged NLRI and the exact one-member local-AS sequence for an ordinary external recipient, with internal/route-server controls and received-input immutability. TestASPathSlotPrependOnlyWhenAdvertising uses the same prepend intent for advertisement and withdrawal, requires the precise prepended AS-number sequence for the former and no AS_PATH operation for attribute-free withdrawal or MP_UNREACH-only input. The established iBGP control keeps the decoded AS-number list unchanged. Existing limits remain: that iBGP control compares decoded numbers rather than exact TLV bytes, and the overflow test checks the new local-AS segment and retained-segment member count rather than every retained member value. The unchanged checkRelayWithdrawalShape adds independent FRR decoded-path, withdrawal-decode, no-attribute-error and live-session observations, with prior route arrival preceding absence; no fresh interop run is claimed here. The added cd0b42213c test strengthens the negative at actual ForwardUpdate dispatch: forwardOneBody captures exactly one rebuilt raw body from one selected destination; the withdrawal subtest requires absent AS_PATH, complete byte equality and unchanged source input. Its paired advertisement control compares the complete expected UPDATE body, including local AS65000 before source AS65001, next-hop-self192.0.2.1 and unchanged NLRI, and independently checks source immutability. A blanket disabled prepend or next-hop rewrite therefore cannot satisfy that control. The test inspects the dispatched body; this judgment does not extend it into an additional socket-delivery claim. Producing paths were read: ASPathEdit.Record sends non-advertising input to recordWithdrawOnly; recordPrepend/tryShift and ASPath.Prepend implement the sequence, set and empty-path rewrites; forward_build.planAttr independently refuses creation of an absent attribute when advertiseGate is false. The new paired mutation names exactly the two guards that would otherwise mask each other, while each standalone guard can protect the withdrawal. cd0b42213c changes no production source or previous assertion: it adds the bounded negative carrier and proof metadata. The row remains enforced with the stated existing limits. This is source rejudgment only; no new test, guest, paired-mutation, solo-mutation or interop observation is claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEstablishedAnnounce_ExplicitASPath_IBGPVerbatim`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_batch_test.go#L567) | unit/verify | unproven |
| negative | [`TestForwardWithdrawalDoesNotCreateASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_withdraw_aspath_pair_test.go#L24) | unit/verify | mutant, verified |
| negative | [`TestASPathSlotPrependOnlyWhenAdvertising`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc4271_advertise_test.go#L99) | unit/verify | unproven |
| negative | [`checkRelayWithdrawalShape`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L433) | interop/nightly | mutant, verified |
| positive | [`TestForwardPresentEmptyASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_aspath_empty_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestEstablishedAnnounce_ExplicitASPath_PrependsLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_batch_test.go#L543) | unit/verify | unproven |
| positive | [`TestASPathSlotPrependOnlyWhenAdvertising`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc4271_advertise_test.go#L97) | unit/verify | unproven |
| positive | [`TestASPathSlotPrependsBeforeALeadingASSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc4271_aspath_slot_test.go#L358) | unit/verify | revert, verified |
| positive | [`TestASPathSlotStartsANewSegmentWhenTheLeadingOneIsFull`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc4271_aspath_slot_test.go#L392) | unit/verify | revert, verified |
| positive | [`checkRelayWithdrawalShape`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L432) | interop/nightly | revert, verified |

### [`RFC4271-5.1.4-3`](#rfc4271-5.1.4-3)

If a BGP speaker is configured to alter the value of the MULTI_EXIT_DISC attribute received over EBGP, then altering the value MUST be done prior to determining the degree of preference of the route and prior to performing route selection (Decision Process phases 1 and 2). (§5.1.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-03 by an independent auditor who authored none of 816ce1ce4c, 2cbd255ba4, 2e5c2d012a, d22b811ce4 or 1a2ccaeef8 (phase 5 of spec-filter-wire-0-umbrella, the per-peer text filter chain deleted). RFC 4271 Section 5.1.4 read: "If a BGP speaker is configured to alter the value of the MULTI_EXIT_DISC attribute received over EBGP, then altering the value MUST be done prior to determining the degree of preference of the route and prior to performing route selection (Decision Process phases 1 and 2)." The moved unit TestRFC4271ReceivedMEDAlteredBeforeTheDecisionProcess replaced the text delta "med 10" answer with the typed set answerSet("med","10") from a filter that declares med read and set: The test filter is now an answerTransport behind filterTransportSeam, compiled by answerPolicy into a policychain library chain, so the verdict travels the real filter-update path (route.Input, rpc.FilterUpdateOutput decoded by the library chain) instead of the deleted policyFilterSeam that handed a PolicyResponse straight to the text chain; that is the production path every plugin filter now takes. It still feeds an EBGP UPDATE with MED 100 through notifyMessageReceiver with the import chain step and requires exactly one dispatch carrying MED 10 in both RawBytes and WireUpdate.Payload, so an alteration applied after dispatch, or not at all, goes red. Single-polarity scope unchanged. Ran the tagged units under ./le job run at HEAD (-tags ze_bgp, -v): PASS (scratch audit-p5-units-v.log). Verdict unchanged: enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4271ReceivedMEDAlteredBeforeTheDecisionProcess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_med_ingress_test.go#L42) | unit/verify | revert, verified |
| positive | [`TestRFC4271MEDAlterationHappensAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L455) | unit/verify | unproven |

### [`RFC4271-5.1.5-5`](#rfc4271-5.1.5-5)

A BGP speaker SHALL calculate the degree of preference for each external route based on the locally-configured policy, and include the degree of preference when advertising a route to its internal peers. (§5.1.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (rfc4271): "A BGP speaker SHALL calculate the degree of preference for each external route based on the locally-configured policy, and include the degree of preference when advertising a route to its internal peers." be95c10053 gave receiveExternalRoute an explicit import policy in both cases (FinalAccept, no refs, plus the set-local-pref ref in the policy case) and reworded the negative tag from 'no import policy' to 'an import policy that sets none'; faf7c5c380 renamed filterapi to routepolicy in the same fixture. Under RFC 8212 a route with no import policy is ineligible and is never advertised, so the old negative input no longer reaches the decision; the reworded input keeps its point (nothing on the route names a degree of preference) and the assertions are unchanged: dispatched to the RIB with no LOCAL_PREF (never 200), advertised internally with the default 100. The tag now states the input the body drives. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271DegreeOfPreferenceNotAHardcodedConstant`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L191) | unit/verify | unproven |
| negative | [`TestRFC4271ExternalRouteWithoutPolicyStillAdvertisesAPreferenceInternally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_degree_of_preference_test.go#L126) | unit/verify | revert, verified |
| positive | [`TestRFC4271ExternalRouteDegreeOfPreferenceFromLocalPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L164) | unit/verify | unproven |
| positive | [`TestRFC4271ExternalRoutePreferenceFromPolicyReachesRIBAndInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_degree_of_preference_test.go#L94) | unit/verify | revert, verified |

### [`RFC4271-5.1.7-1`](#rfc4271-5.1.7-1)

A BGP speaker that performs route aggregation MAY add the AGGREGATOR attribute, which SHALL contain its own AS number and IP address. (§5.1.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-5.1.7-1, so no unit is bound to it.

### [`RFC4271-6.7-4`](#rfc4271-6.7-4)

If the BGP speaker decides to terminate its BGP connection with a neighbor because the number of address prefixes received from the neighbor exceeds the locally-configured, upper bound, then the speaker MUST send the neighbor a NOTIFICATION message with the Error Code Cease. (§6.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Peer level: running Peer Established with ipv4/unicast maximum 2 and teardown default, far end announces 3 prefixes -> exactly one NOTIFICATION 6/1 (Cease, Maximum Number of Prefixes Reached) read off the socket, then EOF (TestRFC4271PrefixLimitTeardownSendsCease). Negative: teardown false, same 3 prefixes -> no NOTIFICATION in 500 ms, connection up, Established, so the Cease is bound to the decision to terminate. checkPrefixLimits break observed red both polarities. Closes 'the send to the neighbor is not exercised'. HEAD units TestPrefixExceedTeardown/TestPrefixExceedDrop stay supplementary (return-value level). Re-read 2026-10-01 (c10 judge): the only change under this verdict is a tag comment line for another id (RFC4271-6.7-1 / RFC4271-Security-1 added, RFC4271-9.1.2.1-1 removed) in the unit's doc comment; every assertion is byte-identical, so the judgement stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271PrefixLimitWithoutTeardownSendsNoCease`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_prefix_limit_cease_peer_test.go#L114) | unit/verify | revert, verified |
| negative | [`TestPrefixExceedDrop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L144) | unit/verify | unproven |
| positive | [`TestRFC4271PrefixLimitTeardownSendsCease`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_prefix_limit_cease_peer_test.go#L89) | unit/verify | revert, verified |
| positive | [`TestPrefixExceedTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L107) | unit/verify | unproven |

### [`RFC4271-6.8-1`](#rfc4271-6.8-1)

In the event of connection collision, one of the connections MUST be closed (§6.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCollisionOpenSentNoCollision`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L167) | unit/verify | unproven |
| positive | [`TestRFC4271CollisionClosesExactlyOneConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rfc7611_ingress_test.go#L52) | unit/verify | revert, verified |

### [`RFC4271-6.8-2`](#rfc4271-6.8-2)

Upon receipt of an OPEN message, the local system MUST examine all of its connections that are in the OpenConfirm state. (§6.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-01 (BGP c12 judge). The trigger (receipt of an OPEN) now reaches the examination in a tagged unit: TestRFC4271ReceivedOpenExaminesTheOpenConfirmConnection hands a second connection to Reactor.acceptOrReject, which tracks it as pending until its OPEN, then writes a complete OPEN on it; handlePendingCollision examines the session held in OpenConfirm. Both outcomes of the examination are asserted on the wire: remote Identifier 10.0.0.3 above local 10.0.0.2 -> Cease 6/7 on the OpenConfirm connection; 10.0.0.1 below -> 6/7 on the new connection, pending cleared, OpenConfirm session kept. Negative TestRFC4271ReceivedOpenWithNoOpenConfirmConnection: with no connection in OpenConfirm (session Established) an OPEN with Identifier 255.255.255.255, which would win an OpenConfirm comparison, does not close the existing session; the new connection gets 6/7 and Established stays. Records on session.go::detectCollision observed red both polarities. HEAD units in collision_test.go (direct detectCollision calls) stay supplementary. Ze holds at most one OpenConfirm connection per peer, so 'all' is that one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCollisionNonCollisionStates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L529) | unit/verify | unproven |
| negative | [`TestRFC4271ReceivedOpenWithNoOpenConfirmConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_collision_open_receipt_test.go#L107) | unit/verify | revert, verified |
| positive | [`TestCollisionOpenConfirmLocalWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L120) | unit/verify | unproven |
| positive | [`TestRFC4271ReceivedOpenExaminesTheOpenConfirmConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_collision_open_receipt_test.go#L66) | unit/verify | revert, verified |

### [`RFC4271-9-1`](#rfc4271-9-1)

If the UPDATE message contains a non-empty WITHDRAWN ROUTES field, the previously advertised routes, whose destinations (expressed as IP prefixes) are contained in this field, SHALL be removed from the Adj-RIB-In. (§9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both new units feed a received UPDATE with a non-empty WITHDRAWN ROUTES field through handleReceivedStructured (not FamilyRIB directly). Positive: the named route leaves the peer's Adj-RIB-In (2->1), leaves the Loc-RIB and a BestChangeWithdraw names it. Negative: 11/8, not in the field, stays in both and gets no withdraw; withdrawing an unannounced prefix removes nothing. Judge mutants 2026-09-30 (go test -overlay, tree untouched): dropping the decision run for legacy withdrawals reddens the positive. Storage-level HEAD tags remain as supplementary proof of FamilyRIB.Remove.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ReceivedWithdrawalRemovesOnlyTheNamedRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L125) | unit/verify | revert, verified |
| negative | [`TestRFC4271WithdrawRemovesFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L210) | unit/verify | unproven |
| positive | [`TestRFC4271ReceivedWithdrawalRemovesTheRouteFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L99) | unit/verify | revert, verified |
| positive | [`TestRFC4271WithdrawRemovesFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L205) | unit/verify | unproven |

### [`RFC4271-9-2`](#rfc4271-9-2)

If the UPDATE message contains a feasible route, the Adj-RIB-In will be updated with this route as follows: if the NLRI of the new route is identical to the one the route currently has stored in the Adj- RIB-In, then the new route SHALL replace the older route in the Adj- RIB-In, thus implicitly withdrawing the older route from service. (§9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: a second received route with identical NLRI from the same peer leaves one Adj-RIB-In entry and the new next hop in service. Negative (inputs biased toward the violation): the newer route has a LONGER AS_PATH, so keep-both or keep-better would leave the older 10.0.0.1 selected; the test asserts one entry and 10.0.0.2 in service. Both run through handleReceivedStructured. The HEAD storage negative (removal keyed on NLRI) is a neighbouring rule kept as supplementary; the new pair carries the row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ReplacementWithdrawsTheOlderRouteEvenWhenItWasBetter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L172) | unit/verify | revert, verified |
| negative | [`TestRFC4271WithdrawRemovesFromAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L207) | unit/verify | unproven |
| positive | [`TestRFC4271ReceivedRouteWithIdenticalNLRIReplacesTheOlder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L151) | unit/verify | revert, verified |
| positive | [`TestRFC4271SamePrefixReplacesRatherThanAccumulates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc4271_test.go#L179) | unit/verify | unproven |

### [`RFC4271-9-3`](#rfc4271-9-3)

Once the BGP speaker updates the Adj-RIB-In, the speaker SHALL run its Decision Process. (§9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 4271 Section 9: "Once the BGP speaker updates the Adj-RIB-In, the speaker SHALL run its Decision Process." Re-judged 2026-10-26 by an independent auditor: the rib_bestchange_test.go units lost their tags, leaving the two plugins/rib/rfc4271_receive_decision_test.go carriers, both byte-identical to the units previously judged. Both feed an UPDATE body to handleReceivedStructured, never calling checkBestPathChange. Positive: one received route reaches the Loc-RIB with next hop 10.0.0.1 and exactly one BestChangeAdd names 10.0.0.0/8. Negative: after peer A withdraws its selected route, the Loc-RIB holds peer B's 10.0.0.2 rather than A's stale route or none, so a skipped run on the withdrawal update fails. The pair is two triggers of one obligation (announce, withdraw) rather than a confining case; no conforming input exists where the process must not run, so this is the strongest available pair. Discrimination records verified: revert of rib_bestchange.go::checkRouteBestChange reddens both units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271AdjRIBInUpdateThatDisplacesTheBestReselects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L214) | unit/verify | revert, verified |
| positive | [`TestRFC4271AdjRIBInUpdateRunsTheDecisionProcess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L194) | unit/verify | revert, verified |

### [`RFC4271-9.1.1-1`](#rfc4271-9.1.1-1)

The function that calculates the degree of preference for a given route SHALL NOT use any of the following as its inputs: the existence of other routes, the non-existence of other routes, or the path attributes of other routes. (§9.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The vacuous withNoise ComparePair assertion is gone. The positive now asserts SelectBest picks b for {a,b}, {b,a} and all 120 orders of {a,b}+3 unrelated routes (one ties b on LOCAL_PREF), and that b beats each member pairwise. Judge mutants 2026-09-30 (go test -overlay, tree untouched): a SelectBest that, with more than two candidates, prefers the longer AS_PATH among LOCAL_PREF ties (preference reading the existence of other routes) reddens the positive; swapping LOCAL_PREF after AS_PATH reddens both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271DegreeOfPreferenceFollowsOwnAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L95) | unit/verify | unproven |
| positive | [`TestRFC4271DegreeOfPreferenceIgnoresOtherRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L36) | unit/verify | revert, verified |

### [`RFC4271-9.1.1-2`](#rfc4271-9.1.1-2)

the return value MUST be used as the LOCAL_PREF value in any IBGP readvertisement. (§9.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-06 by an independent auditor who authored none of the change; no test or record ran for this judgement. RFC 4271 Section 9.1.1: "otherwise, the return value MUST be used as the LOCAL_PREF value in any IBGP readvertisement." The staleness is a doc comment: TestRFC4271LocalPrefIncludedForInternalPeers changed in its comment only, and reactor_wire.go:537-541 is the true citation (lpVal default, stored override, writeLocalPrefAttr). Its claim is wider than its body: it says the value is the stored LOCAL_PREF defaulting to 100, and the route it builds stores none, so it proves the default only; the claim should be narrowed or the tag dropped. The verdict rests on TestRFC4271ImportPolicyLocalPrefIsReadvertisedToInternalPeers, re-read: the source's real import chain computes 200, the internal destination is sent exactly 000000C8 and the external none, and with no policy the internal is sent 00000064, so no constant passes both; both polarities carry observed reach records. Bound: the unit drives the general forward rail. The route-server rail shares applyFactsLocalPref (forward_rs.go:553), and the rib plugin's readvertisement is not driven by it. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ImportPolicyLocalPrefIsReadvertisedToInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_local_pref_readvertise_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestRFC4271ImportPolicyLocalPrefIsReadvertisedToInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_local_pref_readvertise_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestRFC4271LocalPrefIncludedForInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L86) | unit/verify | unproven |

### [`RFC4271-9.1.2-1`](#rfc4271-9.1.2-1)

If the NEXT_HOP attribute of a BGP route depicts an address that is not resolvable, or if it would become unresolvable if the route was installed in the routing table, the BGP route MUST be excluded from the Phase 2 decision function. (§9.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.1.2-1, so no unit is bound to it.

### [`RFC4271-9.1.2-2`](#rfc4271-9.1.2-2)

The local speaker SHALL then install that route in the Loc-RIB, replacing any route to the same destination that is currently being held in the Loc-RIB. (§9.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 4271 Section 9.1.2: "The local speaker SHALL then install that route in the Loc-RIB, replacing any route to the same destination that is currently being held in the Loc-RIB." Re-judged 2026-10-26 by an independent auditor: TestRIBBestChangeWithdraw lost its tag; the remaining carriers are TestRFC4271SelectedRouteReplacesTheLocRIBRoute and TestRFC4271LocRIBHoldsOnlyTheReplacingRoute (receive path through handleReceivedStructured, byte-identical to the units judged before) and TestLocRIBMirror (supplementary positive: checkBestPathChange installs the best in a locrib with eBGP admin distance and NEXT_HOP). Positive: peer B's better route replaces peer A's as the Loc-RIB best (10.0.0.1 -> 10.0.0.2). Negative: the destination's Loc-RIB group holds exactly one BGP path, peer B's, so install-beside fails. Both clauses (install, replacing) are asserted. Discrimination records verified: revert of rib_remote.go::insertLocRIB reddens both receive-path units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271LocRIBHoldsOnlyTheReplacingRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L263) | unit/verify | revert, verified |
| positive | [`TestRFC4271SelectedRouteReplacesTheLocRIBRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L239) | unit/verify | revert, verified |
| positive | [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_bestchange_test.go#L1611) | unit/verify | unproven |

### [`RFC4271-9.1.2-3`](#rfc4271-9.1.2-3)

The local speaker MUST determine the immediate next-hop address from the NEXT_HOP attribute (§9.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271LocRIBNextHopComesFromNextHopAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L121) | unit/verify | unproven |
| positive | [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_bestchange_test.go#L1613) | unit/verify | unproven |

### [`RFC4271-9.1.2-4`](#rfc4271-9.1.2-4)

If either the immediate next-hop or the IGP cost to the NEXT_HOP (where the NEXT_HOP is resolved through an IGP route) changes, Phase 2 Route Selection MUST be performed again. (§9.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.1.2-4, so no unit is bound to it.

### [`RFC4271-9.1.2.1-1`](#rfc4271-9.1.2.1-1)

Notice that even though BGP routes do not have to be installed in the Routing Table with the immediate next-hop(s), implementations MUST take care that, before any packets are forwarded along a BGP route, its associated NEXT_HOP address is resolved to the immediate (directly connected) next-hop address, and that this address (or multiple addresses) is finally used for actual packet forwarding. (§9.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-01 (c10 judge). The RIB-level tags (which proved the Loc-RIB copies NEXT_HOP unresolved, RFC4271-9.1.2-3) are gone; resolution is proven where Ze performs it, in sysrib. Positive TestRFC4271BGPNextHopResolvedToTheImmediateNextHop: BGP /24 NEXT_HOP 172.16.5.5 inside an IGP /16 via 10.0.0.1 inside a connected /24 on eth1; the (system-rib, best-change) Add the FIB writer programs carries 10.0.0.1 + eth1, protocol bgp, and no entry ever carries 172.16.5.5 (resolveMember, record observed red). Negative TestRFC4271BGPRouteWithAnUnresolvedNextHopLeavesTheFIB: the IGP /16 is removed, a Withdraw is published and only removes follow for 200 ms (cascadeRecompute, record observed red). Caveat: a BGP route whose NEXT_HOP was NEVER resolvable in the Loc-RIB is published with its raw gateway (fibEntry fibUnproved); the Linux FIB refuses an off-link gateway, so no packet is forwarded along it, but no unit asserts that kernel refusal.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271BGPRouteWithAnUnresolvedNextHopLeavesTheFIB`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc4271_nexthop_resolution_test.go#L145) | unit/verify | revert, verified |
| positive | [`TestRFC4271BGPNextHopResolvedToTheImmediateNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc4271_nexthop_resolution_test.go#L107) | unit/verify | revert, verified |

### [`RFC4271-9.1.2.1-2`](#rfc4271-9.1.2.1-2)

Unresolvable routes SHALL be removed from the Loc-RIB and the routing table. (§9.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.1.2.1-2, so no unit is bound to it.

### [`RFC4271-9.1.2.2-1`](#rfc4271-9.1.2.2-1)

The criteria MUST be applied in the order specified. (§9.1.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent supplemental source rejudgment 2026-10-06; weak to enforced on changed/new evidence relative to the recorded audit. RFC4271 Section 9.1.2.2: "The tie-breaking algorithm begins by considering all equally preferable routes to the same destination, and then selects routes to be removed from consideration. The algorithm terminates as soon as only one route remains in consideration. The criteria MUST be applied in the order specified." Also: "BGP implementations MAY use any algorithm that produces the same results as those described here." MED: "Remove from consideration routes with less-preferred MULTI_EXIT_DISC attributes. MULTI_EXIT_DISC is only comparable between routes learned from the same neighboring AS (the neighboring AS is determined from the AS_PATH attribute). Routes that do not have the MULTI_EXIT_DISC attribute are considered to have the lowest possible MULTI_EXIT_DISC value." Full criteria (a)-(g) and all ten current functions read, including three whole-set units absent from the old seven-unit map. Old pairwise fold could choose C for A(AS65001,MED0,ID .3), B(AS65002,MED0,ID .2), C(AS65001,MED100,ID .1), although A must remove C before B removes A. Current bestpath_selection.go::selectBestCandidates eliminates compareBeforeMED losers, compareNeighborAS/retainLowestMED eliminate within each known neighboring AS, and compareAfterMED chooses survivors. SelectBest/SelectBestExplain share this; SelectMultipath uses the MED survivors. TestRFC4271WholeSetMEDBeforeLaterCriteria covers six permutations, both APIs, exact winner B and exact elimination witnesses C by A at MED then A by B at Router ID. TestRFC4271WholeSetMEDEarlierCriteria makes A lose at stale, LOCAL_PREF, AIGP, AS_PATH length or ORIGIN; six permutations require C, preventing an already eliminated low-MED candidate from removing C. TestRFC4271WholeSetMEDRIBBestChange drives actual structured received UPDATEs, requires Loc-RIB B, withdraws non-best A and requires C plus exact BestChangeUpdate, restores A and requires B plus exact replacement, and rejects spurious unchanged publication. Adjacent criteria conflict in both orders. Structured/production-JSON remote-ID pairs prove identifier precedence and equal-ID address fall-through. Older FullTiebreak and aligned MED/address examples remain supplementary. Consumer chain handleReceivedStructured/JSON -> metadata/storage -> extractCandidate -> checkRouteBestChange -> SelectMultipath -> Loc-RIB and best-change publication was read. This replacement producer and newly tagged multi-candidate consumer evidence independently close the specific prior finding; no unchanged-unit reinterpretation or panic record justifies the upgrade. All identifier and whole-set units have no current diff HEAD, and the only current selection producer hunk is a comment, so this is preexisting audit debt relative to HEAD, not this mechanical edit. They are nevertheless changed/new against the supplied audit map, hence upgrade_reason is omitted rather than misused. No tests, semantic mutants, builds, stamps or checks executed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBestPath_MED_SameNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L325) | unit/verify | unproven |
| negative | [`TestBestPathEqualBGPIdentifiersOnTheJSONRailFallThroughToPeerAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_json_test.go#L107) | unit/verify | revert, verified |
| negative | [`TestBestPathEqualBGPIdentifiersFallThroughToPeerAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_test.go#L116) | unit/verify | revert, verified |
| negative | [`TestRFC4271WholeSetMEDEarlierCriteria`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_whole_set_med_test.go#L104) | unit/verify | revert, verified |
| positive | [`TestBestPath_FullTiebreak`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L540) | unit/verify | unproven |
| positive | [`TestBestPathStepFComparesThePeerBGPIdentifierOnTheJSONRail`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_json_test.go#L79) | unit/verify | revert, verified |
| positive | [`TestBestPathStepFComparesThePeerBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_bgp_identifier_test.go#L88) | unit/verify | revert, verified |
| positive | [`TestRFC4271AdjacentCriteriaApplyInTheOrderSpecified`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_criteria_order_test.go#L44) | unit/verify | revert, verified |
| positive | [`TestRFC4271WholeSetMEDBeforeLaterCriteria`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_whole_set_med_test.go#L44) | unit/verify | revert, verified |
| positive | [`TestRFC4271WholeSetMEDRIBBestChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_whole_set_med_test.go#L188) | unit/verify | revert, verified |

### [`RFC4271-9.1.2.2-2`](#rfc4271-9.1.2.2-2)

If an implementation chooses to remove MULTI_EXIT_DISC, then the optional comparison on MULTI_EXIT_DISC, if performed, MUST be performed only among EBGP-learned routes. (§9.1.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.1.2.2-2, so no unit is bound to it.

### [`RFC4271-9.1.2.2-3`](#rfc4271-9.1.2.2-3)

For IBGP- learned routes, the MULTI_EXIT_DISC MUST be used in route comparisons that reach this step in the Decision Process. (§9.1.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-01 (BGP c13 judge). The MISSING clause (b) of Section 9.1.2.2 (c) is closed at the producer: firstASInPath answers 0 for a path that begins with an AS_SET (quote above the statement), and neighborAS turns 0 into the local AS for an IBGP route (PeerASN == LocalASN). Positive TestRFC4271IBGPAggregatesLedByASSetsCompareMED: two IBGP aggregates led by different AS_SETs share neighbor AS = local AS, MED 10 beats MED 20 at BestStepMED against IGP cost 50 vs 5, and SelectBest agrees. Negative TestRFC4271IBGPRouteLedByASSequenceKeepsItsNeighborAS: an AS_SEQUENCE-led IBGP route keeps neighbor AS 65010 against an aggregate led by AS_SET {65010, 65020}, MED is skipped and the IGP cost decides, so the local-AS rule is not over-applied. Both build candidates from wire bytes through firstASInPath and asPathLength, the calls extractCandidate (rib_commands.go) makes. Their records break the producer with a panic (reach only); a judge overlay (tree untouched) removing only the AS_SET check reddens both units and TestFirstASInPath, whose wrong expectation (65003, the set's first member) was corrected to 0 under D-15, so the clause is discriminated. HEAD units unchanged: empty-AS_PATH IBGP positive and the later-steps negative (2026-09-30 judge mutants). Side effect read: an EBGP route led by an AS_SET now compares no MED (route-selection.md); such a path breaks Section 5.1.2 b) at its sender, and MED is owed only between routes of one known neighbor AS, so not comparing it stays conformant. Residual outside this row's RFC 4271 reading: the RIB classifies IBGP by PeerASN == LocalASN, not the reactor's RFC 7705-aware rule (journal helper-bypassed-by-an-open-coded-copy.md).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBestPath_MED_SameNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L332) | unit/verify | unproven |
| negative | [`TestRFC4271IBGPRouteLedByASSequenceKeepsItsNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_ibgp_aggregate_med_test.go#L83) | unit/verify | revert, verified |
| negative | [`TestRFC4271IBGPMEDIsNotSkippedWhenLaterStepsDisagree`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_ibgp_med_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestBestPath_MED_SameNeighborAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L329) | unit/verify | unproven |
| positive | [`TestRFC4271IBGPAggregatesLedByASSetsCompareMED`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_ibgp_aggregate_med_test.go#L63) | unit/verify | revert, verified |
| positive | [`TestRFC4271IBGPLocallyOriginatedRoutesCompareMED`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_ibgp_med_test.go#L42) | unit/verify | revert, verified |

### [`RFC4271-9.1.2.2-4`](#rfc4271-9.1.2.2-4)

Routes that do not have the MULTI_EXIT_DISC attribute are considered to have the lowest possible MULTI_EXIT_DISC value (§9.1.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAbsentMedTiesAnExplicitMedOfZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L280) | unit/verify | revert, verified |
| positive | [`TestAbsentMedStillComparesAsZeroInPhaseTwo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_test.go#L248) | unit/verify | revert, verified |

### [`RFC4271-9.2-2`](#rfc4271-9.2-2)

A route SHALL NOT be installed in the Adj-Rib-Out unless the destination, and NEXT_HOP described by this route, may be forwarded appropriately by the Routing Table. (§9.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2-2, so no unit is bound to it.

### [`RFC4271-9.2-3`](#rfc4271-9.2-3)

If a route in Loc-RIB is excluded from a particular Adj-RIB-Out, the previously advertised route in that Adj-RIB-Out MUST be withdrawn from service by means of an UPDATE message (§9.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2-3, so no unit is bound to it.

### [`RFC4271-9.2-4`](#rfc4271-9.2-4)

If a BGP speaker receives overlapping routes, the Decision Process MUST consider both routes based on the configured acceptance policy. (§9.1.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-03 by an independent auditor who authored none of 816ce1ce4c, 2cbd255ba4, 2e5c2d012a, d22b811ce4 or 1a2ccaeef8 (phase 5 of spec-filter-wire-0-umbrella, the per-peer text filter chain deleted). RFC 4271 Section 9.2 read: "If a BGP speaker receives overlapping routes, the Decision Process MUST consider both routes based on the configured acceptance policy." Both tagged units moved through receiveOverlappingRoutes only: The test filter is now an answerTransport behind filterTransportSeam, compiled by answerPolicy into a policychain library chain, so the verdict travels the real filter-update path (route.Input, rpc.FilterUpdateOutput decoded by the library chain) instead of the deleted policyFilterSeam that handed a PolicyResponse straight to the text chain; that is the production path every plugin filter now takes. The filter reads prefix, and the received WireUpdate now carries a registered ASN4 receive context so the prefix read can decode the NLRI, as every received UPDATE does. Positive: the acceptance policy is asked twice and both 10.0.0.0/8 and 10.1.0.0/24 are dispatched in order. Negative: the policy rejects when the fields name 10.1.0.0/24, and only the /8 is dispatched; a fields read that missed the prefix would accept both and fail the exact-equality assertion, so the negative still discriminates on the policy verdict. Assertions unchanged; the residual noted before (verdict supplied by a test filter, not a parsed filter config) stands. Ran the tagged units under ./le job run at HEAD (-tags ze_bgp, -v): PASS (scratch audit-p5-units-v.log). Verdict unchanged: enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OverlappingRouteArrivalDisplacesNeither`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L318) | unit/verify | revert, verified |
| negative | [`TestRFC4271OverlappingRouteRejectedByPolicyIsNotConsidered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_overlap_acceptance_test.go#L122) | unit/verify | revert, verified |
| positive | [`TestRFC4271OverlappingReceivedRoutesAreBothInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L291) | unit/verify | revert, verified |
| positive | [`TestRFC4271OverlappingRoutesAcceptedByPolicyBothReachTheDecisionProcess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_overlap_acceptance_test.go#L99) | unit/verify | revert, verified |

### [`RFC4271-9.2-5`](#rfc4271-9.2-5)

If both a less and a more specific route are accepted, then the Decision Process MUST install, in Loc-RIB, either both the less and the more specific routes or aggregate the two routes and install, in Loc-RIB, the aggregated route, provided that both routes have the same value of the NEXT_HOP attribute. (§9.1.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: a less and a more specific route with the same NEXT_HOP are both installed in the Loc-RIB (bestNextHop for /8 and /24). Negative: installing the second overlapping route, in either order, never uninstalls the first (no withdraw, first still in Loc-RIB, Adj-RIB-In holds 2), which a most-specific-wins or covering-wins process fails. Ze takes the 'install both' branch; aggregation is not used. Old storage NLRI-keying tags removed (D-15).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OverlappingRouteArrivalDisplacesNeither`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L320) | unit/verify | revert, verified |
| positive | [`TestRFC4271OverlappingReceivedRoutesAreBothInstalled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L294) | unit/verify | revert, verified |

### [`RFC4271-9.2.1.1-2`](#rfc4271-9.2.1.1-2)

Two UPDATE messages sent by a BGP speaker to a peer that advertise feasible routes and/or withdrawal of unfeasible routes to some common set of destinations MUST be separated by at least MinRouteAdvertisementIntervalTimer. (§9.2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.1.1-2, so no unit is bound to it.

### [`RFC4271-9.2.2.2-1`](#rfc4271-9.2.2.2-1)

Routes that have different MULTI_EXIT_DISC attributes SHALL NOT be aggregated. (§9.2.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.2.2-1, so no unit is bound to it.

### [`RFC4271-9.2.2.2-2`](#rfc4271-9.2.2.2-2)

ORIGIN attribute: If at least one route among routes that are aggregated has ORIGIN with the value INCOMPLETE, then the aggregated route MUST have the ORIGIN attribute with the value INCOMPLETE. Otherwise, if at least one route among routes that are aggregated has ORIGIN with the value EGP, then the aggregated route MUST have the ORIGIN attribute with the value EGP. (§9.2.2.2)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Re-judged by an independent judge after the row's annotation moved from {not-applicable} to {gap}. RFC 4271 Section 9.2.2.2 reads 'ORIGIN attribute: If at least one route among routes that are aggregated has ORIGIN with the value INCOMPLETE, then the aggregated route MUST have the ORIGIN attribute with the value INCOMPLETE.' and 'Otherwise, if at least one route among routes that are aggregated has ORIGIN with the value EGP, then the aggregated route MUST have the ORIGIN attribute with the value EGP.' Ze is the BGP speaker this binds once it aggregates. Aggregation is optional in Section 9.2.2.1, but no owner decision declines it or places it out of scope, so under the rule that an annotation proving less than a gap is written only for a role or feature the owner named, the absent feature is a conformance gap and the earlier not-applicable verdict no longer stands. Ze forms no aggregate: internal/component/bgp/message/update_build_grouped.go::BuildGroupedUnicast packs independent prefixes that share one attribute set into one UPDATE, and packGroupedAttributes copies the ORIGIN the caller supplied; no code derives an aggregate's ORIGIN from contributing routes. A configured covering static prefix takes whatever ORIGIN the operator writes, related to no contributing route.

No test carries RFC4271-9.2.2.2-2, so no unit is bound to it.

### [`RFC4271-9.2.2.2-3`](#rfc4271-9.2.2.2-3)

NEXT_HOP: When aggregating routes that have different NEXT_HOP attributes, the NEXT_HOP attribute of the aggregated route SHALL identify an interface on the BGP speaker that performs the aggregation. (§9.2.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.2.2-3, so no unit is bound to it.

### [`RFC4271-9.2.2.2-4`](#rfc4271-9.2.2.2-4)

ATOMIC_AGGREGATE: If at least one of the routes to be aggregated has ATOMIC_AGGREGATE path attribute, then the aggregated route SHALL have this attribute as well. (§9.2.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.2.2-4, so no unit is bound to it.

### [`RFC4271-9.2.2.2-5`](#rfc4271-9.2.2.2-5)

AGGREGATOR: Any AGGREGATOR attributes from the routes to be aggregated MUST NOT be included in the aggregated route. (§9.2.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.2.2-5, so no unit is bound to it.

### [`RFC4271-Security-1`](#rfc4271-security-1)

An implementation MUST support the TCP MD5 option [RFC2385]. (§E)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Collateral source re-judgment 2026-10-02 after unrelated MD5 requirement tags were removed from TestRFC2385MatchingKeysCarryASignedSession's enclosing doc comment. RFC4271 AppendixE states: An implementation MUST support the TCP MD5 option [RFC2385]. Its Security Considerations also require per-peer activation support. Every currently tagged unit was reread: TestRFC2385ConfiguredKeyReachesBothSockets drives an operator password through parsePeerFromTree and NewSession and asserts the exact key and peer address in RealDialer and md5PeersForListener; TestMD5PeersForListener and its second polarity assert two keyed default-port peers, MD5IP override, exclusion of an unkeyed peer and differently bound listener, the separate keyed custom-port peer, and an empty unrelated port. TestRFC2385MatchingKeysCarryASignedSession exercises actual RealDialer/RealListenerFactory sockets with matching keys and requires256KiB received; TestRFC2385KeyOnOneEndOnlyCarriesNoSession requires timeout for an unsigned dial into the keyed listener and fails on a successful connection, discriminating a no-op option installer. Source trace reaches reactor.newListenerFactory, which installs the collected keys in RealListenerFactory, and both socket Control callbacks reach md5_linux.go::setTCPMD5Sig before bind/connect with errors propagated. Configuration-only tests are supplementary, not a substitute for the real sockets. This preserves the Linux support verdict and does not claim macOS MD5 support, exact wire digest composition from the old loopback unit, or any new native execution. Unsupported-kernel skips are not positive evidence; the separately reviewed configured runtime and stock kernel emitted configs now establish TCP_MD5SIG availability. Existing executable assertions and this row's tag claims are unchanged; Main must stamp fresh enclosing-unit hashes with mode rejudge, not treat changed doc comments as mere shifted-file reseals.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMD5PeersForListener`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L2360) | unit/verify | unproven |
| negative | [`TestRFC2385KeyOnOneEndOnlyCarriesNoSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L180) | unit/verify | revert, verified |
| positive | [`TestMD5PeersForListener`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_test.go#L2357) | unit/verify | unproven |
| positive | [`TestRFC2385ConfiguredKeyReachesBothSockets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2385_test.go#L44) | unit/verify | revert, verified |
| positive | [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L86) | unit/verify | revert, verified |

### [`RFC4271-9.2.1.1-3`](#rfc4271-9.2.1.1-3)

If new routes are selected multiple times while awaiting the expiration of MinRouteAdvertisementIntervalTimer, the last route selected SHALL be advertised at the end of MinRouteAdvertisementIntervalTimer. (§9.2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4271-9.2.1.1-3, so no unit is bound to it.

### [`RFC4271-9.2-6`](#rfc4271-9.2-6)

When a BGP speaker receives an UPDATE message from an internal peer, the receiving BGP speaker SHALL NOT re-distribute the routing information contained in that UPDATE message to other internal peers (unless the speaker acts as a BGP Route Reflector [RFC2796]). (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-01 (BGP c15 judge). Both c14 gaps on the general rail closed: TestRFC4271ForwardRailNeverRedistributesInternalRoutesToInternalPeers forwards one route from an internal non-client source on forwardUpdateCore -> forwardUpdateSection to an internal non-client (sent nothing), an external peer (sent the route) and an internal route-reflector client DESTINATION (sent the route: the RR exception with a client destination). Absence is judged after the two controls of the same fan-out arrive plus a 500 ms window, not a bare sleep. Revert record on forwardUpdateSection; the author's overlay turning the !srcInfo.isRRClient && !facts.rrClient refusal into if false reddens the non-client assertion (c15-ov-sh). The route-server rail keeps TestRFC4271NoIBGPToIBGPRedistribution (absence after a fixed 50 ms sleep, no in-fan-out control; its partner TestRFC4271IBGPRedistributionAllowedForReflectorClient shows the same dispatch within 2 s) and TestRFC4271IBGPRedistributionAllowedForReflectorClient: a residual timing weakness on that rail only, and neither RS unit has a record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271ForwardRailNeverRedistributesInternalRoutesToInternalPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_ibgp_split_horizon_test.go#L35) | unit/verify | revert, verified |
| negative | [`TestRFC4271IBGPRedistributionAllowedForReflectorClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L573) | unit/verify | unproven |
| positive | [`TestRFC4271NoIBGPToIBGPRedistribution`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L514) | unit/verify | unproven |

### [`RFC4271-9.2-7`](#rfc4271-9.2-7)

All newly installed routes and all newly unfeasible routes for which there is no replacement route SHALL be advertised to its peers by means of an UPDATE message. (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 4271 Section 9.2: "All newly installed routes and all newly unfeasible routes for which there is no replacement route SHALL be advertised to its peers by means of an UPDATE message." Re-judged 2026-10-26 by an independent auditor after the tags moved from plugins/rib/rfc4271_rib_bestchange_test.go (745b0890f8) to reactor/rfc4271_rs_replacement_test.go. Both carriers drive the real bgp-rs, bgp-rib and bgp-adj-rib-in over live TCP sessions and judge the client's replayed TCP history behind a causal fence taken once the client owes nothing. Positive TestRFC4271RouteServerLastSourceWithdrawalIsSent: the only source withdraws P; the client no longer holds P, saw exactly one withdrawal of P and exactly one announcement (no re-announcement), so the no-replacement clause is asserted at the wire, not as an internal event. Negative TestRFC4271RouteServerOwnerWithdrawalSendsReplacement: the owner of the held best withdraws while another source's route is retained; the client ends on the retained route's exact attributes with zero withdrawals of P, which proves both the newly installed replacement is advertised and the bare withdrawal is confined to the no-replacement case; the earlier newly-installed steps (B's route, then A's better route reaching C) are also asserted. Discrimination records verified: revert of relay_recovery.go::queueRecoveryWithdrawal reddens the positive; mutant owesWithdrawal 'entry == nil' -> true (owner withdrawal sent bare) reddens the negative. The earlier weak finding (an internal BestChangeWithdraw event, no UPDATE, no newly-installed carrier) is answered by the new units. Scope read: owesWithdrawal/oweAnnouncement (session_ownership.go) and recoveryOwedFor (recovery_owed.go) do not branch on RSClient, so the RS-client fixture exercises the general non-ADD-PATH path; when the RIB recovery owner is not attached to the source, the bare withdrawal is sent with a warning naming the attach fix (ownerReceives), a misconfiguration path no tag covers.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271RouteServerOwnerWithdrawalSendsReplacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rs_replacement_test.go#L42) | unit/verify | mutant, verified |
| positive | [`TestRFC4271RouteServerLastSourceWithdrawalIsSent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rs_replacement_test.go#L88) | unit/verify | mutant, verified |

### [`RFC4271-9.2-8`](#rfc4271-9.2-8)

Any routes in the Loc-RIB marked as unfeasible SHALL be removed (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 4271 Section 9.2: "Any routes in the Loc-RIB marked as unfeasible SHALL be removed." First judgement 2026-10-26 by an independent auditor. Positive TestLocRIBMirror (plugins/rib/rfc4271_rib_bestchange_test.go): after the only route is removed from the Adj-RIB-In and the decision re-runs, loc.Best reports the prefix absent; it calls checkBestPathChange directly rather than through a received withdrawal (the receive path running the decision is 9-3's carrier) and carries no discrimination record, but its assertion is exact and fails if the no-candidate branch of checkRouteBestChange stopped calling removeLocRIB. Negatives (reactor/rfc4271_rs_replacement_test.go, split in 1d549b72cd, live bgp-rs/bgp-rib/bgp-adj-rib-in sessions): a second source's worse route is announced then withdrawn while the best stays feasible; TestRFC4271RouteServerStillFeasibleRouteStaysInLocRIB reads locrib.Default() behind a causal fence (bgp-rib stored B's marker sent after its withdrawal) and requires P still installed with A's NEXT_HOP, which confines removal to unfeasible routes; TestRFC4271RouteServerStillFeasibleRouteIsNotWithdrawn requires the client wire to keep A's route with one announcement and zero withdrawals, the wire consequence of the feasible route staying. Discrimination records verified for both negatives: mutant checkRouteBestChange 'newBest == nil' -> true reddens the Loc-RIB unit; mutant oweAnnouncement contest test -> true reddens the wire unit. Two negatives for one requirement are distinct observations (Loc-RIB, wire), not one assertion wearing two hats.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271RouteServerStillFeasibleRouteIsNotWithdrawn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rs_replacement_test.go#L180) | unit/verify | mutant, verified |
| negative | [`TestRFC4271RouteServerStillFeasibleRouteStaysInLocRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rs_replacement_test.go#L215) | unit/verify | mutant, verified |
| positive | [`TestLocRIBMirror`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_bestchange_test.go#L1616) | unit/verify | unproven |

### [`RFC4271-9.2-9`](#rfc4271-9.2-9)

Changes to the reachable destinations within its own autonomous system SHALL also be advertised in an UPDATE message. (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-01 (BGP c18 judge). The old objection (units call the encoders only) is closed at the entry point: NEW TestRFC4271OwnASDestinationBecomingReachableIsSentToPeers originates 10.20.0.0/24 through reactorAPIAdapter.AnnounceNLRIBatch (the API announce entry point) toward one internal and one external Established peer and parses each connection's bytes: exactly one UPDATE, Withdrawn Routes empty, attributes present, NLRI = the prefix. NEW TestRFC4271OwnASDestinationBecomingUnreachableIsSentToPeers: after that announcement WithdrawNLRIBatch writes exactly one more UPDATE per peer with the prefix in Withdrawn Routes, no attributes, no NLRI, so a change that removes reachability is advertised, not left standing. Observed-red records: + on reactor_api_batch.go::AnnounceNLRIBatch, - on WithdrawNLRIBatch. The HEAD encoder units (writeAnnounceUpdate/writeWithdrawUpdate) stay as supplementary. Residual, not a clause gap: own-AS reachability changes are driven through the API rail; static-route reload and default-originate rails are not exercised by these units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OwnASDestinationBecomingUnreachableIsSentToPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_own_as_change_test.go#L128) | unit/verify | revert, verified |
| negative | [`TestRFC4271OwnASUnreachabilityChangeAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L429) | unit/verify | unproven |
| positive | [`TestRFC4271OwnASDestinationBecomingReachableIsSentToPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_own_as_change_test.go#L102) | unit/verify | revert, verified |
| positive | [`TestRFC4271OwnASReachabilityChangeAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_test.go#L401) | unit/verify | unproven |

### [`RFC4271-9.2-10`](#rfc4271-9.2-10)

If, due to the limits on the maximum size of an UPDATE message (see Section 4), a single route doesn't fit into the message, the BGP speaker MUST not advertise the route to its peers and MAY choose to log an error locally. (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 4271 Section 9.2: “If, due to the limits on the maximum size of an UPDATE message (see Section 4), a single route doesn't fit into the message, the BGP speaker MUST not advertise the route to its peers and MAY choose to log an error locally.” The changed splitter pair retains equivalent non-ADD-PATH framing. Its oversized opaque-attribute case requires an error and zero callbacks; the fitting control preserves attributes and NLRI in one output. The genuine relay pair supplies protocol-shaped input: a single route with 900 communities fits and is relayed byte-identically; 1100 communities exceed the 4096-byte destination limit and yield ok=false with neither raw nor parsed output. Both forwarding callers explicitly release and skip failed items, so failure is not later dispatched. Final ordinary-write admission also refuses an indivisible effective chunk. The corrected test tag accurately says an error, not an assertion of the exact ErrAttributesTooLarge identity. Logging is optional. Whole-producer halt records are reachability evidence, not selective size-check mutations.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271FittingRouteIsAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L432) | unit/verify | revert, verified |
| negative | [`TestRFC4271RelayedRouteTooLargeForThePeerIsWithheld`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_oversize_route_test.go#L40) | unit/verify | revert, verified |
| positive | [`TestRFC4271OversizeSingleRouteNotAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_test.go#L403) | unit/verify | revert, verified |
| positive | [`TestRFC4271RelayedRouteTooLargeForThePeerIsWithheld`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_oversize_route_test.go#L38) | unit/verify | revert, verified |

### [`RFC4271-9.1.2-5`](#rfc4271-9.1.2-5)

If the AS_PATH attribute of a BGP route contains an AS loop, the BGP route should be excluded from the Phase 2 decision function (§9.1.2). Detection scans the full AS path and checks that the local autonomous system number does not appear in it. RFC 4271 writes this keyword in lower case, so the level is a recommendation and not a capitalized RFC 2119 SHOULD. The same paragraph places a speaker configured to accept routes with its own autonomous system number in the AS path outside the scope of the document. That out-of-scope case is what the allow-own-as setting selects, so a non-zero allow-own-as is not a deviation from this line.

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDetectASLoop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/loopdetect/rfc4271_loop_test.go#L114) | unit/verify | unproven |
| negative | [`TestDetectASLoop_ASSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/loopdetect/rfc4271_loop_test.go#L125) | unit/verify | unproven |
| negative | [`loop-as.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/loop-as.ci#L7) | functional/verify | unproven |
| positive | [`TestDetectASLoop_NotPresent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/loopdetect/rfc4271_loop_test.go#L136) | unit/verify | unproven |

### [`RFC4271-4.3-7`](#rfc4271-4.3-7)

A BGP speaker SHOULD treat an UPDATE message of this form as though the WITHDRAWN ROUTES do not contain the address prefix. (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. HEAD positives (rfc4271_rib_mixed_update_test.go, three paths) prove the prefix named in both WITHDRAWN and NLRI stays installed with no withdrawal published. New negative TestRFC4271MixedUpdateStillAppliesItsOtherWithdrawals drives handleReceivedStructured and asserts the OTHER withdrawn prefix (11/8) still leaves Adj-RIB-In and Loc-RIB, so the treatment is scoped to the duplicated prefix. Judge mutants 2026-09-30 (go test -overlay, tree untouched): skipping the whole legacy WITHDRAWN field when the UPDATE carries NLRI reddens the negative alone; the positives stay green, so the two polarities catch opposite failures.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271MixedUpdateStillAppliesItsOtherWithdrawals`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go#L354) | unit/verify | revert, verified |
| positive | [`TestRIBInjectSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L121) | unit/verify | unproven |
| positive | [`TestRIBPoolPathSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L87) | unit/verify | unproven |
| positive | [`TestRIBSamePrefixInWithdrawnAndNLRIInstallsTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_rib_mixed_update_test.go#L42) | unit/verify | unproven |

### [`RFC4271-5-8`](#rfc4271-5-8)

Once a BGP peer has updated any well-known attributes, it MUST pass these attributes to its peers in any updates it transmits (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestForwardNeverTransmitsTheSupersededWellKnownAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_section5_test.go#L166) | unit/verify | revert, verified |
| positive | [`TestForwardTransmitsUpdatedWellKnownAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_forward_section5_test.go#L145) | unit/verify | revert, verified |

### [`RFC4271-6.2-5`](#rfc4271-6.2-5)

If the version number in the Version field of the received OPEN message is not supported, then the Error Subcode MUST be set to Unsupported Version Number (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_open_error_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_open_error_test.go#L64) | unit/verify | revert, verified |

### [`RFC4271-6.2-6`](#rfc4271-6.2-6)

If the Autonomous System field of the OPEN message is unacceptable, then the Error Subcode MUST be set to Bad Peer AS (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_open_error_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_open_error_test.go#L67) | unit/verify | revert, verified |

### [`RFC4271-6.2-7`](#rfc4271-6.2-7)

If the Hold Time field of the OPEN message is unacceptable, then the Error Subcode MUST be set to Unacceptable Hold Time (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_open_error_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_open_error_test.go#L71) | unit/verify | revert, verified |

### [`RFC4271-6.2-8`](#rfc4271-6.2-8)

If the BGP Identifier field of the OPEN message is syntactically incorrect, then the Error Subcode MUST be set to Bad BGP Identifier (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_open_error_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestRFC4271OpenErrorSubcodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_open_error_test.go#L74) | unit/verify | revert, verified |

### [`RFC4271-6.2-9`](#rfc4271-6.2-9)

If one of the Optional Parameters in the OPEN message is not recognized, then the Error Subcode MUST be set to Unsupported Optional Parameters (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271OptionalParameterErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L37) | unit/verify | unproven |
| positive | [`TestSessionRFC4271OptionalParameterErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L36) | unit/verify | unproven |

### [`RFC4271-6.2-10`](#rfc4271-6.2-10)

If one of the Optional Parameters in the OPEN message is recognized, but is malformed, then the Error Subcode MUST be set to 0 (Unspecific) (§6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271OptionalParameterErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L39) | unit/verify | unproven |
| positive | [`TestSessionRFC4271OptionalParameterErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L38) | unit/verify | unproven |

### [`RFC4271-6.3-4`](#rfc4271-6.3-4)

If the Withdrawn Routes Length or Total Attribute Length is too large (i.e., if Withdrawn Routes Length + Total Attribute Length + 23 exceeds the message Length), then the Error Subcode MUST be set to Malformed Attribute List. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive drives Total Attribute Length and Withdrawn Routes Length overruns through enforceRFC7606 and asserts SessionReset plus NOTIFICATION 3/1 read off the written bytes (RFC 7606 3(b) keeps this subcode); negative: consistent lengths draw no NOTIFICATION

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271UpdateMalformedAttributeList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_update_error_test.go#L57) | unit/verify | revert, verified |
| positive | [`TestRFC4271UpdateMalformedAttributeList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_update_error_test.go#L54) | unit/verify | revert, verified |

### [`RFC4271-6.3-5`](#rfc4271-6.3-5)

If any recognized attribute has Attribute Flags that conflict with the Attribute Type Code, then the Error Subcode MUST be set to Attribute Flags Error. The Data field MUST contain the erroneous attribute (type, length, and value). (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Revised by RFC 7606 3(c) to treat-as-withdraw unless the attribute's specification mandates other flag handling. Flag conflicts are decided by one table (AttributeCode.FlagsConflict via validateAttributeFlags). Tagged units now cover ORIGIN (well-known), MED, AGGREGATOR and ATOMIC_AGGREGATE with conflicting Optional/Transitive bits: exact withdraw-only payload, session Established; the same attributes with specified flags keep the prefix and the value byte for byte. AGGREGATOR flags go to withdraw because RFC 7606 7.7 mandates attribute discard only for its length conditions, not for flags. Observed-red records on validateAttributeFlags both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271RecognizedAttributeErrorsBeyondTheFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_recognized_attribute_errors_test.go#L53) | unit/verify | revert, verified |
| negative | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L103) | unit/verify | unproven |
| positive | [`TestRFC4271RecognizedAttributeErrorsBeyondTheFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_recognized_attribute_errors_test.go#L52) | unit/verify | revert, verified |
| positive | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L102) | unit/verify | unproven |

### [`RFC4271-6.3-6`](#rfc4271-6.3-6)

If any recognized attribute has an Attribute Length that conflicts with the expected length (based on the attribute type code), then the Error Subcode MUST be set to Attribute Length Error. The Data field MUST contain the erroneous attribute (type, length, and value). (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Revised by RFC 7606 to per-attribute outcomes. Every attribute whose length its type code fixes now has a length case against its expected length: ORIGIN (withdraw, 7.1), NEXT_HOP (withdraw, 7.3) and MED in TestSessionRFC4271RevisedAttributeErrors, ATOMIC_AGGREGATE and AGGREGATOR (attribute discard, prefix kept, 7.6/7.7) in TestRFC4271RecognizedAttributeErrorsBeyondTheFirst, and LOCAL_PREF in TestRFC4271LocalPrefLengthFromInternalPeer: an iBGP session over the real receive path, length 3 yields exactly the withdrawal of the prefix with the session still Established (7.5), length 4 before and after keeps the prefix with LOCAL_PREF byte-identical. The LOCAL_PREF buffer is otherwise valid, so only the length rule can fire. Observed-red records on validateLocalPrefAttr, validateAggregatorAttr and validateNextHopAttr, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271LocalPrefLengthFromInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_a_local_pref_length_test.go#L32) | unit/verify | revert, verified |
| negative | [`TestRFC4271RecognizedAttributeErrorsBeyondTheFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_recognized_attribute_errors_test.go#L55) | unit/verify | revert, verified |
| negative | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L105) | unit/verify | revert, verified |
| positive | [`TestRFC4271LocalPrefLengthFromInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_reactor_a_local_pref_length_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestRFC4271RecognizedAttributeErrorsBeyondTheFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_recognized_attribute_errors_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L104) | unit/verify | revert, verified |

### [`RFC4271-6.3-7`](#rfc4271-6.3-7)

If any of the well-known mandatory attributes are not present, then the Error Subcode MUST be set to Missing Well-known Attribute. The Data field MUST contain the Attribute Type Code of the missing, well-known attribute. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7606 Section 3(d) (Updates: 4271) replaces the subcode and Data clauses with treat-as-withdraw, so the forbidden behaviour is a reset or a silent keep. TestSessionRFC4271RevisedAttributeErrors: missing ORIGIN, missing AS_PATH and missing NEXT_HOP each require.Equal the payload to a withdrawal of 203.0.113.0/24 (red on a keep or a drop), and the good UPDATE after it on the same session is received (red on a reset). Negative: all three present, NLRI announced. TestRFC4271MandatoryAttributesAcrossUpdateForms pins the MP_REACH/legacy NEXT_HOP split.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271MandatoryAttributesAcrossUpdateForms`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_mandatory_test.go#L11) | unit/verify | unproven |
| negative | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L107) | unit/verify | unproven |
| positive | [`TestRFC4271MandatoryAttributesAcrossUpdateForms`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4271_mandatory_test.go#L10) | unit/verify | unproven |
| positive | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L106) | unit/verify | unproven |

### [`RFC4271-6.3-8`](#rfc4271-6.3-8)

If any of the well-known mandatory attributes are not recognized, then the Error Subcode MUST be set to Unrecognized Well-known Attribute. The Data field MUST contain the unrecognized attribute (type, length, and value). (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Subcode clause: unknown well-known code 250 (flags 0x50, extended length) asserts the NOTIFICATION body equals 3/2 exactly, red on any other subcode. Data clause: the same Equal pins Data to the whole attribute, header and extended length included, red on a truncated or absent Data field. Negative: the same code with flags 0xd0 (optional transitive) is accepted and its NLRI announced. 2026-10-01 re-judge (BGP c9 judge, wrote none of these units): stale only because TestSessionRFC4271RetainedUpdateNotifications gained one RFC4271-6.3-1 tag comment line; no executable line moved. Verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271RetainedUpdateNotifications`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L157) | unit/verify | unproven |
| positive | [`TestSessionRFC4271RetainedUpdateNotifications`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L156) | unit/verify | unproven |

### [`RFC4271-6.3-9`](#rfc4271-6.3-9)

If the ORIGIN attribute has an undefined value, then the Error Sub- code MUST be set to Invalid Origin Attribute. The Data field MUST contain the unrecognized attribute (type, length, and value). (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7606 Section 7.1 (Updates: 4271) replaces the subcode and Data clauses with treat-as-withdraw for an undefined ORIGIN. ORIGIN 3, the first undefined value, require.Equals the payload to a withdrawal (red on a keep) and the following good UPDATE is received (red on a reset). Negative: ORIGIN IGP announces the prefix.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L109) | unit/verify | unproven |
| positive | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L108) | unit/verify | unproven |

### [`RFC4271-6.3-10`](#rfc4271-6.3-10)

If the NEXT_HOP attribute field is syntactically incorrect, then the Error Subcode MUST be set to Invalid NEXT_HOP Attribute. The Data field MUST contain the incorrect attribute (type, length, and value). (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7606 (Updates: 4271) revises this: the error is handled by treat-as-withdraw, so no NOTIFICATION, subcode or Data field is sent. multicast NEXT_HOP 224.0.0.1 (not a valid host address) withdraws the prefix exactly (7.3); unicast NEXT_HOP announces it

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L111) | unit/verify | unproven |
| positive | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L110) | unit/verify | unproven |

### [`RFC4271-6.3-11`](#rfc4271-6.3-11)

The IP address in the NEXT_HOP MUST meet the following criteria to be considered semantically correct: a) It MUST NOT be the IP address of the receiving speaker. b) In the case of an EBGP, where the sender and receiver are one IP hop away from each other, either the IP address in the NEXT_HOP MUST be the sender's IP address that is used to establish the BGP connection, or the interface associated with the NEXT_HOP IP address MUST share a common subnet with the receiving BGP speaker. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. criterion a) receiver's own address (eBGP and iBGP) and b) off-link next hop on a one-hop eBGP session each withhold the legacy announcement on the live receive path; sender address and on-subnet third party accepted; multihop and iBGP accept off-link; interface snapshot change flips the verdict. The reaction (ignore, no NOTIFICATION) is RFC4271-6.3-2/6.3-3

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271NextHopSemantics`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L207) | unit/verify | unproven |
| negative | [`TestSessionRFC4271IBGPNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_next_hop_test.go#L63) | unit/verify | unproven |
| negative | [`TestSessionRFC4271NextHopMixedUpdateAndAddressChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_next_hop_test.go#L16) | unit/verify | unproven |
| positive | [`TestSessionRFC4271NextHopSemantics`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L206) | unit/verify | unproven |
| positive | [`TestSessionRFC4271IBGPNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_next_hop_test.go#L62) | unit/verify | unproven |
| positive | [`TestSessionRFC4271NextHopMixedUpdateAndAddressChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_next_hop_test.go#L15) | unit/verify | unproven |

### [`RFC4271-6.3-12`](#rfc4271-6.3-12)

If the path is syntactically incorrect, then the Error Subcode MUST be set to Malformed AS_PATH. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7606 (Updates: 4271) revises this: the error is handled by treat-as-withdraw, so no NOTIFICATION, subcode or Data field is sent. unknown segment type, zero segment count, segment overrun and trailing octet each withdraw the prefix exactly (7.2); a well-framed AS_SEQUENCE announces

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L113) | unit/verify | unproven |
| positive | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L112) | unit/verify | unproven |

### [`RFC4271-6.3-13`](#rfc4271-6.3-13)

If the UPDATE message is received from an external peer, the local system MAY check whether the leftmost (with respect to the position of octets in the protocol message) AS in the AS_PATH attribute is equal to the autonomous system number of the peer that sent the message. If the check determines this is not the case, the Error Subcode MUST be set to Malformed AS_PATH. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7606 (Updates: 4271) revises this: the error is handled by treat-as-withdraw, so no NOTIFICATION, subcode or Data field is sent. an external UPDATE whose leftmost AS 65003 is not the peer AS 65002 is delivered as exactly a withdrawal of its prefix (7.2); leftmost AS equal to the peer AS announces unchanged

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271LeftmostASMismatchIsMalformedASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rfc7611_ingress_test.go#L38) | unit/verify | revert, verified |
| positive | [`TestRFC4271LeftmostASMismatchIsMalformedASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rfc7611_ingress_test.go#L35) | unit/verify | revert, verified |

### [`RFC4271-6.3-14`](#rfc4271-6.3-14)

If an optional attribute is recognized, then the value of this attribute MUST be checked. If an error is detected, the attribute MUST be discarded, and the Error Subcode MUST be set to Optional Attribute Error. The Data field MUST contain the attribute (type, length, and value). (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Revised by RFC 7606. 'If an optional attribute is recognized, then the value MUST be checked': RFC 4271's own optional attributes, MED (3 octets -> exact withdrawal, 4 kept) and AGGREGATOR (bad length -> discarded, prefix kept, the 'attribute MUST be discarded' clause), are both covered, plus COMMUNITIES and EXTENDED COMMUNITIES (withdraw per 7606 7.8/7.14) with valid values kept byte for byte. Observed-red records on validateCommunityAttr both polarities. ORIGINATOR_ID and CLUSTER_LIST value checks belong to the RFC 4456 / RFC 7606 per-attribute rows, not this generic RFC 4271 row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271RecognizedAttributeErrorsBeyondTheFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_recognized_attribute_errors_test.go#L57) | unit/verify | revert, verified |
| negative | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L115) | unit/verify | unproven |
| positive | [`TestRFC4271RecognizedAttributeErrorsBeyondTheFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_recognized_attribute_errors_test.go#L56) | unit/verify | revert, verified |
| positive | [`TestSessionRFC4271RevisedAttributeErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L114) | unit/verify | unproven |

### [`RFC4271-6.3-15`](#rfc4271-6.3-15)

If any attribute appears more than once in the UPDATE message, then the Error Subcode MUST be set to Malformed Attribute List (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Revised by RFC 7606 Section 3(g): a duplicate MP_REACH_NLRI or MP_UNREACH_NLRI draws NOTIFICATION Malformed Attribute List; any other duplicate is discarded after the first. TestRFC4271UpdateMalformedAttributeList: duplicate MP_REACH_NLRI -> NOTIFICATION 3/1 read off the socket; duplicate COMMUNITY -> no NOTIFICATION, exactly one COMMUNITY left (negative). 2026-10-01 re-judge (BGP c9 judge, wrote none of these units): the MP_UNREACH clause is now tagged: TestSessionRFC7606DuplicateMPUnreachNotificationOnTheWire reads 3/1 off the wire for a duplicate MP_UNREACH_NLRI, and its fixture (ORIGIN, empty AS_PATH, two well-formed MP_UNREACH, no NLRI) records no other error, so only the duplicate check produces the reset (RFC7606-3.g-1 mutant B evidence). Dropping the mpUnreachCount operand now turns a tagged unit red. ValidateUpdateRFC7606AddPath breaks observed red for all three units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271UpdateMalformedAttributeList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_update_error_test.go#L61) | unit/verify | revert, verified |
| positive | [`TestRFC4271UpdateMalformedAttributeList`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_update_error_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestSessionRFC7606DuplicateMPUnreachNotificationOnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_dupmp_unreach_wire_test.go#L31) | unit/verify | revert, verified |

### [`RFC4271-6.3-16`](#rfc4271-6.3-16)

If the field is syntactically incorrect, then the Error Subcode MUST be set to Invalid Network Field. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. impossible prefix length and truncated NLRI (and truncated withdrawn) reset with NOTIFICATION 3/10 read off the socket after a good UPDATE was accepted on the same session; a valid prefix is announced 2026-10-01 re-judge (BGP c9 judge, wrote none of these units): stale only because TestSessionRFC4271RetainedUpdateNotifications gained one RFC4271-6.3-1 tag comment line; no executable line moved. Verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271RetainedUpdateNotifications`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L159) | unit/verify | unproven |
| positive | [`TestSessionRFC4271RetainedUpdateNotifications`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L158) | unit/verify | unproven |

### [`RFC4271-6.3-17`](#rfc4271-6.3-17)

An UPDATE message that contains correct path attributes, but no NLRI, SHALL be treated as a valid UPDATE message (§6.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271UpdateWithoutNLRIIsValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_update_error_test.go#L128) | unit/verify | revert, verified |
| positive | [`TestRFC4271UpdateWithoutNLRIIsValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_update_error_test.go#L125) | unit/verify | revert, verified |

### [`RFC4271-8.2.2-19`](#rfc4271-8.2.2-19)

In response to an indication that the TCP connection is successfully established (Event 16 or Event 17), the second connection SHALL be tracked until it sends an OPEN message (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC4271EstablishedCollisionWaitsForOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L248) | unit/verify | unproven |
| positive | [`TestSessionRFC4271EstablishedCollisionWaitsForOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_session_core4271_test.go#L247) | unit/verify | unproven |

### [`RFC4271-9-4`](#rfc4271-9-4)

Otherwise, if the Adj-RIB-In has no route with NLRI identical to the new route, the new route SHALL be placed in the Adj-RIB-In. (§9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a new NLRI not placed. handleReceivedStructured (the reactor UPDATE entry) asserts Len 1 then 2 and Get(10.0.0.0/8) with its own NEXT_HOP, red on a route not stored. Negative (the condition false, identical NLRI): Len stays 2 and the slot holds the newer NEXT_HOP, red on a route placed beside the existing one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4271AdjRIBInPlacesNewRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rfc4271_rib_test.go#L44) | unit/verify | revert, verified |
| positive | [`TestRFC4271AdjRIBInPlacesNewRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rfc4271_rib_test.go#L41) | unit/verify | revert, verified |

### [`RFC4271-10-4`](#rfc4271-10-4)

The suggested default amount of jitter SHALL be determined by multiplying the base value of the appropriate timer by a random factor, which is uniformly distributed in the range from 0.75 to 1.0 (§10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTimersRFC4271Jitter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_timer_jitter_test.go#L12) | unit/verify | unproven |
| positive | [`TestTimersRFC4271Jitter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/fsm/rfc4271_timer_jitter_test.go#L11) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-01 |
| Register | rfc2119 |
| Source | rfc/full/rfc4271.txt |
| Source fingerprint | d5034568e80da453 |
| Record | rfc/extraction/rfc4271.json |
| Mapped sentences | 106 |
| Declined as scope | 23 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 3 | walked | not stated |
| `4.2` | not stated | 2 | walked | not stated |
| `4.3` | not stated | 4 | walked | not stated |
| `4.4` | not stated | 2 | walked | not stated |
| `4.5` | not stated | 0 | walked | not stated |
| `5` | not stated | 8 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.1.1` | not stated | 0 | walked | not stated |
| `5.1.2` | not stated | 1 | walked | not stated |
| `5.1.3` | not stated | 3 | walked | not stated |
| `5.1.4` | not stated | 4 | walked | not stated |
| `5.1.5` | not stated | 5 | walked | not stated |
| `5.1.6` | not stated | 1 | walked | not stated |
| `5.1.7` | not stated | 1 | walked | not stated |
| `6` | not stated | 1 | walked | not stated |
| `6.1` | not stated | 6 | walked | not stated |
| `6.2` | not stated | 9 | walked | not stated |
| `6.3` | not stated | 25 | walked | not stated |
| `6.4` | not stated | 0 | walked | not stated |
| `6.5` | not stated | 0 | walked | not stated |
| `6.6` | not stated | 0 | walked | not stated |
| `6.7` | not stated | 2 | walked | not stated |
| `6.8` | not stated | 2 | walked | not stated |
| `7` | not stated | 1 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.1.1` | not stated | 0 | walked | not stated |
| `8.1.2` | not stated | 0 | walked | not stated |
| `8.1.3` | not stated | 0 | walked | not stated |
| `8.1.4` | not stated | 0 | walked | not stated |
| `8.1.5` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `8.2.1` | not stated | 3 | walked | not stated |
| `8.2.1.1` | not stated | 0 | walked | not stated |
| `8.2.1.2` | not stated | 0 | walked | not stated |
| `8.2.1.3` | not stated | 0 | walked | not stated |
| `8.2.1.4` | not stated | 0 | walked | not stated |
| `8.2.1.5` | not stated | 0 | walked | not stated |
| `8.2.2` | not stated | 1 | walked | not stated |
| `9` | not stated | 5 | walked | not stated |
| `9.1` | not stated | 1 | walked | not stated |
| `9.1.1` | not stated | 1 | walked | not stated |
| `9.1.2` | not stated | 6 | walked | not stated |
| `9.1.2.1` | not stated | 0 | walked | not stated |
| `9.1.2.2` | not stated | 3 | walked | not stated |
| `9.1.3` | not stated | 2 | walked | not stated |
| `9.1.4` | not stated | 3 | walked | not stated |
| `9.2` | not stated | 5 | walked | not stated |
| `9.2.1` | not stated | 0 | walked | not stated |
| `9.2.1.1` | not stated | 2 | walked | not stated |
| `9.2.1.2` | not stated | 0 | walked | not stated |
| `9.2.2` | not stated | 0 | walked | not stated |
| `9.2.2.1` | not stated | 0 | walked | not stated |
| `9.2.2.2` | not stated | 11 | walked | not stated |
| `9.3` | not stated | 0 | walked | not stated |
| `9.4` | not stated | 0 | walked | not stated |
| `10` | not stated | 2 | walked | not stated |
| `A` | not stated | 1 | walked | not stated |
| `B` | not stated | 0 | walked | not stated |
| `C` | not stated | 0 | walked | not stated |
| `D` | not stated | 0 | walked | not stated |
| `E` | not stated | 1 | walked | not stated |
| `F` | not stated | 0 | walked | not stated |
| `F.1` | not stated | 0 | walked | not stated |
| `F.2` | not stated | 0 | walked | not stated |
| `F.3` | not stated | 0 | walked | not stated |
| `F.4` | not stated | 0 | walked | not stated |
| `F.5` | not stated | 0 | walked | not stated |
| `F.6` | not stated | 1 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `5:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the sentence defines the mandatory category by restating the obligation site 5:2 already carries, that a mandatory attribute be present when the UPDATE contains NLRI | The mandatory category refers to an attribute that MUST be present in both IBGP and EBGP exchanges if NLRI are contained in the UPDATE message. |
| `6.1:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Data field clause belongs to the Bad Message Length obligation, and the row already states both halves | The Data field MUST contain the erroneous Length field. |
| `6.1:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Data field clause belongs to the Bad Message Type obligation, and the row already states both halves | The Data field MUST contain the erroneous Type field. |
| `6.3:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Data field clause belongs to the Attribute Flags Error obligation, and the row states both halves | The Data field MUST contain the erroneous attribute (type, length, and value). |
| `6.3:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Data field clause belongs to the Attribute Length Error obligation, and the row states both halves | The Data field MUST contain the erroneous attribute (type, length, and value). |
| `6.3:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Data field clause belongs to the Missing Well-known Attribute obligation, and the row states both halves | The Data field MUST contain the Attribute Type Code of the missing, well-known attribute. |
| `6.3:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Data field clause belongs to the Unrecognized Well-known Attribute obligation, and the row states both halves | The Data field MUST contain the unrecognized attribute (type, length, and value). |
| `6.3:12` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Data field clause belongs to the Invalid Origin Attribute obligation, and the row states both halves | The Data field MUST contain the unrecognized attribute (type, length, and value). |
| `6.3:14` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Data field clause belongs to the Invalid NEXT_HOP Attribute obligation, and the row states both halves | The Data field MUST contain the incorrect attribute (type, length, and value). |
| `6.3:16` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | criterion a) is one of the two criteria the row states | a) It MUST NOT be the IP address of the receiving speaker. |
| `6.3:17` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | criterion b) is the other of the two criteria the row states | b) In the case of an EBGP, where the sender and receiver are one IP hop away from each other, either the IP address in the NEXT_HOP MUST be the sender's IP address that is used to establish the BGP connection, or the interface associated with the NEXT_HOP IP address MUST share a common subnet with the receiving BGP speaker. |
| `6.3:21` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the discard and the Optional Attribute Error subcode are the consequence half of the same obligation, and the row states them | If an error is detected, the attribute MUST be discarded, and the Error Subcode MUST be set to Optional Attribute Error. |
| `6.3:22` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Data field clause belongs to the Optional Attribute Error obligation, and the row states it | The Data field MUST contain the attribute (type, length, and value). |
| `7:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the sentence binds the DESIGN of future BGP versions, not the behavior of a BGP-4 implementation: it tells whoever specifies BGP-5 to keep the OPEN and NOTIFICATION formats. No code ze runs can conform to it or violate it, because the obligation is discharged by a future document rather than by a speaker on the wire | In order to support BGP version negotiation, future versions of BGP MUST retain the format of the OPEN and NOTIFICATION messages. |
| `9:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | running the Decision Process is the second half of the withdrawal obligation, and the row states both halves | This BGP speaker SHALL run its Decision Process because the previously advertised route is no longer available for use. |
| `9.1.4:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the next sentence defines the act: 'That is, the NLRI of this route cannot be more specific.' That is the Section 5.1.6 prohibition on making the NLRI of an ATOMIC_AGGREGATE route more specific, restated where overlapping routes are discussed | In particular, a route that carries the ATOMIC_AGGREGATE attribute MUST NOT be de-aggregated. |
| `9.2.2.2:5` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the obligation is conditional on aggregating, and ze performs no route aggregation (the scope decision recorded in the Support remaining row of rfc/short/rfc4271.md). The RFC makes the choice the speaker's: "If a BGP speaker chooses to aggregate, then it SHOULD either include" | If the routes to be aggregated have different AS_PATH attributes, then the aggregated AS_PATH attribute SHALL satisfy all of the following conditions: |
| `9.2.2.2:6` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | an AS_SEQUENCE condition on an aggregated AS_PATH ze never builds, because ze performs no route aggregation. The RFC makes the choice the speaker's: "If a BGP speaker chooses to aggregate, then it SHOULD either include" | - all tuples of type AS_SEQUENCE in the aggregated AS_PATH SHALL appear in all of the AS_PATHs in the initial set of routes to be aggregated. |
| `9.2.2.2:7` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | an AS_SET condition on an aggregated AS_PATH ze never builds, because ze performs no route aggregation. The RFC makes the choice the speaker's: "If a BGP speaker chooses to aggregate, then it SHOULD either include" | - all tuples of type AS_SET in the aggregated AS_PATH SHALL appear in at least one of the AS_PATHs in the initial set (they may appear as either AS_SET or AS_SEQUENCE types). |
| `9.2.2.2:8` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | a duplicate-AS_SET condition on an aggregated AS_PATH ze never builds, because ze performs no route aggregation. The RFC makes the choice the speaker's: "If a BGP speaker chooses to aggregate, then it SHOULD either include" | - No tuple of type AS_SET with the same value SHALL appear more than once in the aggregated AS_PATH. |
| `9.2.2.2:9` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the conformance floor is an aggregation algorithm, and ze performs no route aggregation. The RFC makes the choice the speaker's: "If a BGP speaker chooses to aggregate, then it SHOULD either include" | At a minimum, a conformant implementation SHALL be able to perform the following algorithm that meets all of the above conditions: |
| `E:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix E restates the Security Considerations obligation to support the TCP MD5 option of RFC 2385 | An implementation MUST support the TCP MD5 option [RFC2385]. |
| `F.6:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix F.6 restates the same RFC 2385 authentication obligation as a change from RFC 1771 | A BGP implementation MUST support the authentication mechanism specified in RFC 2385 [RFC2385]. |

## Superseded

No document obsoletes RFC 4271, so its obligations are stated where they were written.
