# RFC 6793 - BGP Support for Four-Octet Autonomous System (AS) Number Space

4-byte AS numbers. Every requirement listed for RFC 6793, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 29 | of 30 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 30 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 30 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 30 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 30 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 69 | 69 of 89 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 30 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 30 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 30 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 30 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 30 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 30 | of 36 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 30 mandatory requirements: they add up to 30. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | 4-byte AS numbers |
| Automatic checks | Included |
| Requirements | 36 |
| Mandatory requirements (MUST level) | 30 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 89 |
| Tests linked to requirements | 89 |
| Recorded audit verdicts | 28 |
| Fault test records | 69 |
| Summary | `rfc/short/rfc6793.md` |
| Requirement table | `rfc/requirements/rfc6793.md` |
| RFC text | `rfc/full/rfc6793.txt` |

## Automatic checks

Included in automatic checks: BGP Support for Four-Octet AS Number Space

## Published support status

**Area:** 4-byte AS numbers

**Test verification:** 30 of 30 MUST proven

**Evidence:** functional 2, unit 28

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 29 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 1 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **30** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (29):** [`RFC6793-4.1-1`](#rfc6793-4.1-1), [`RFC6793-4.1-2`](#rfc6793-4.1-2), [`RFC6793-4.1-3`](#rfc6793-4.1-3), [`RFC6793-4.1-4`](#rfc6793-4.1-4), [`RFC6793-4.1-5`](#rfc6793-4.1-5), [`RFC6793-4.1-6`](#rfc6793-4.1-6), [`RFC6793-4.1-7`](#rfc6793-4.1-7), [`RFC6793-4.2.1-1`](#rfc6793-4.2.1-1), [`RFC6793-4.2.2-1`](#rfc6793-4.2.2-1), [`RFC6793-4.2.2-2`](#rfc6793-4.2.2-2), [`RFC6793-4.2.2-3`](#rfc6793-4.2.2-3), [`RFC6793-4.2.2-4`](#rfc6793-4.2.2-4), [`RFC6793-3-1`](#rfc6793-3-1), [`RFC6793-4.2.2-5`](#rfc6793-4.2.2-5), [`RFC6793-4.2.2-6`](#rfc6793-4.2.2-6), [`RFC6793-4.2.3-1`](#rfc6793-4.2.3-1), [`RFC6793-4.2.3-3`](#rfc6793-4.2.3-3), [`RFC6793-4.2.3-4`](#rfc6793-4.2.3-4), [`RFC6793-4.2.3-5`](#rfc6793-4.2.3-5), [`RFC6793-4.2.3-6`](#rfc6793-4.2.3-6), [`RFC6793-4.2.3-7`](#rfc6793-4.2.3-7), [`RFC6793-4.2.3-8`](#rfc6793-4.2.3-8), [`RFC6793-4.2.3-9`](#rfc6793-4.2.3-9), [`RFC6793-4.2.3-10`](#rfc6793-4.2.3-10), [`RFC6793-6-1`](#rfc6793-6-1), [`RFC6793-6-2`](#rfc6793-6-2), [`RFC6793-6-3`](#rfc6793-6-3), [`RFC6793-6-4`](#rfc6793-6-4), [`RFC6793-6-5`](#rfc6793-6-5)

**Annotated (including scoped evidence) (1):** [`RFC6793-4.2.3-2`](#rfc6793-4.2.3-2)

## Figures

- **MUST bar**: Ze verified 30
- **Overview bar**: Ze MUST verified 30, Not assessed 6

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 30 |
| MUST rows kept (the MUST bar) | 30 |
| Ze verified | 30 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 6 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 6 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc6793.json](../../../data/rfc/rfc6793.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6793-4.1-1` | A BGP speaker that supports four-octet AS numbers SHALL advertise this to its peers using BGP Capabilities Advertisements (Section 4.1) | SHALL | 4.1 | **positive:** `unit/verify` [`TestRFC6793OpenAdvertisesFourOctetASCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L60). **negative:** `unit/verify` [`TestRFC6793OpenOmitsCapabilityWhenDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L75) |
| `RFC6793-4.1-2` | The AS number of the BGP speaker MUST be carried in the Capability Value field of the "support for four-octet AS number capability" (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC6793ASN4CapabilityCarriesSpeakerAS`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc6793_asn4_test.go#L18). **negative:** `unit/verify` [`TestRFC6793ASN4CapabilityValueMustBeFourOctets`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc6793_asn4_test.go#L46) |
| `RFC6793-4.1-3` | When a NEW BGP speaker processes an OPEN message from another NEW BGP speaker, it MUST use the AS number encoded in the Capability Value field of the "support for four-octet AS number capability" in lieu of the "My Autonomous System" field of the OPEN message. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestOpenAdvertisedASFollowsLocalASN4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_session_open_as_asn4_test.go#L51). **negative:** `unit/verify` [`TestOpenAdvertisedASFollowsLocalASN4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_session_open_as_asn4_test.go#L52) |
| `RFC6793-4.1-4` | A BGP speaker that advertises such a capability to a particular peer, and receives from that peer the advertisement of such a capability, MUST encode AS numbers as four-octet entities in both the AS_PATH attribute and the AGGREGATOR attribute in the updates it sends to the peer (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC6793EncodeFourOctetToNewSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L33). **negative:** `unit/verify` [`TestRFC6793EncodeTwoOctetToOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L65) |
| `RFC6793-4.1-5` | A BGP speaker that advertises such a capability to a particular peer, and receives from that peer the advertisement of such a capability, MUST encode AS numbers as four-octet entities in both the AS_PATH attribute and the AGGREGATOR attribute in the updates it sends to the peer and MUST assume that these attributes in the updates received from the peer encode AS numbers as four-octet entities. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC6793DecodeFourOctetWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L113). **negative:** `unit/verify` [`TestRFC6793DecodeTwoOctetWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L138) |
| `RFC6793-4.1-6` | The new attributes, AS4_PATH and AS4_AGGREGATOR, MUST NOT be carried in an UPDATE message between NEW BGP speakers. (Section 4.1, Section 6) | MUST NOT | 4.1 | **positive:** `unit/verify` [`TestOriginatedAggregatorOmitsAS4AggregatorTowardNewSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4agg_new_test.go#L12). **positive:** `unit/verify` [`TestOriginatedUpdateOmitsAS4PathTowardNewSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L290). **positive:** `functional/verify` [`rfc6793-no-as4aggregator-from-new-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-no-as4aggregator-from-new-speaker.ci#L12). **positive:** `functional/verify` [`rfc6793-no-as4path-from-new-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-no-as4path-from-new-speaker.ci#L29). **negative:** `functional/verify` [`rfc6793-narrow-to-old-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-narrow-to-old-speaker.ci#L25) |
| `RFC6793-4.1-7` | A NEW BGP speaker that receives the AS4_PATH attribute or the AS4_AGGREGATOR attribute in an UPDATE message from another NEW BGP speaker MUST discard the path attribute and continue processing the UPDATE message. (Section 4.1, Section 6) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC6793NewSpeakerAS4AttributesAreDiscarded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L533). **negative:** `unit/verify` [`TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L207). **positive:** `functional/verify` [`rfc6793-no-as4path-from-new-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-no-as4path-from-new-speaker.ci#L34) |
| `RFC6793-4.2.1-1` | However, this document does not assume that an Autonomous System with NEW BGP speakers has to have a globally unique two-octet AS number -- AS_TRANS MUST be used when the NEW BGP speaker does not have a two-octet AS number (even if multiple Autonomous Systems would use it). (Section 4.2.1) | MUST | 4.2.1 | **positive:** `unit/verify` [`TestRFC6793OpenMyASIsASTransWithoutTwoOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L88). **negative:** `unit/verify` [`TestRFC6793OpenMyASIsRealASWhenMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L104) |
| `RFC6793-4.2.2-1` | When communicating with an OLD BGP speaker, a NEW BGP speaker MUST send the AS path information in the AS_PATH attribute encoded with two-octet AS numbers. (Section 4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestRFC6793EncodeTwoOctetToOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L68). **positive:** `unit/verify` [`TestRFC6996OldSpeakerReceivesStrippedASPathAndAS4Path`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6996_old_speaker_export_test.go#L33). **negative:** `unit/verify` [`TestRFC6793TwoOctetASPathKeepsMappableASNs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L93) |
| `RFC6793-4.2.2-2` | The NEW BGP speaker MUST also send the AS path information in the AS4_PATH attribute (encoded with four-octet AS numbers), except for the case where all of the AS path information is composed of mappable four-octet AS numbers only. (Section 4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestOriginatedUpdateCarriesAS4PathForANonMappableLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L243). **positive:** `unit/verify` [`TestOriginatedUpdateCarriesAS4PathTowardOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L205). **positive:** `unit/verify` [`TestRFC6793TranscodeEmitsAS4PathForNonMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L52). **positive:** `unit/verify` [`TestRFC6996OldSpeakerReceivesStrippedASPathAndAS4Path`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6996_old_speaker_export_test.go#L40). **negative:** `unit/verify` [`TestOriginatedUpdateOmitsAS4PathWhenEveryASIsMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L272). **negative:** `unit/verify` [`TestRFC6793TranscodeOmitsAS4PathWhenAllMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L87) |
| `RFC6793-4.2.2-3` | In this case, the NEW BGP speaker MUST NOT send the AS4_PATH attribute. (Section 4.2.2) | MUST NOT | 4.2.2 | **positive:** `unit/verify` [`TestOriginatedUpdateOmitsAS4PathWhenEveryASIsMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L270). **positive:** `unit/verify` [`TestRFC6793TranscodeOmitsAS4PathWhenAllMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L85). **negative:** `unit/verify` [`TestOriginatedUpdateCarriesAS4PathTowardOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L208). **negative:** `unit/verify` [`TestRFC6793TranscodeEmitsAS4PathForNonMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L55) |
| `RFC6793-4.2.2-4` | Whenever the AS path information contains the AS_CONFED_SEQUENCE or AS_CONFED_SET path segment, the NEW BGP speaker MUST exclude such path segments from the AS4_PATH attribute being constructed. (Section 4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestRFC6793ConstructedAS4PathExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L302). **positive:** `unit/verify` [`TestRFC6793ConstructedAS4PathExcludesConfedSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_confed_set_test.go#L38). **negative:** `unit/verify` [`TestRFC6793ConstructedAS4PathExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L304). **negative:** `unit/verify` [`TestRFC6793ConstructedAS4PathKeepsAnOrdinarySet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_confed_set_test.go#L57) |
| `RFC6793-3-1` | To prevent the possible propagation of Confederation-related path segments outside of a Confederation, the path segment types AS_CONFED_SEQUENCE and AS_CONFED_SET [RFC5065] are declared invalid for the AS4_PATH attribute and MUST NOT be included in the AS4_PATH attribute of an UPDATE message. (Section 3, Section 6) | MUST NOT | 3 | **positive:** `unit/verify` [`TestRFC6793AS4PathWireExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L250). **negative:** `unit/verify` [`TestRFC6793AS4PathWireExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L253) |
| `RFC6793-4.2.2-5` | Similarly, if the NEW BGP speaker has to send the AGGREGATOR attribute, and if the aggregating Autonomous System's AS number is a non-mappable four-octet AS number, then the speaker MUST use the AS4_AGGREGATOR attribute and set the AS number field in the existing AGGREGATOR attribute to the reserved AS number, AS_TRANS. (Section 4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestForwardedAggregatorIsDowngradedWithItsCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L70). **positive:** `unit/verify` [`TestOriginatedAggregatorCarriesItsCompanionTowardOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L415). **positive:** `unit/verify` [`TestRFC6793AS4AggregatorForNonMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L234). **negative:** `unit/verify` [`TestAMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L107). **negative:** `unit/verify` [`TestOriginatedMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L445). **negative:** `unit/verify` [`TestRFC6793NoAS4AggregatorForMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L271) |
| `RFC6793-4.2.2-6` | Note that if the AS number is mappable, then the AS4_AGGREGATOR attribute MUST NOT be sent. (Section 4.2.2) | MUST NOT | 4.2.2 | **positive:** `unit/verify` [`TestAMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L105). **positive:** `unit/verify` [`TestOriginatedMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L443). **positive:** `unit/verify` [`TestRFC6793NoAS4AggregatorForMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L269). **negative:** `unit/verify` [`TestForwardedAggregatorIsDowngradedWithItsCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L73). **negative:** `unit/verify` [`TestOriginatedAggregatorCarriesItsCompanionTowardOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L418). **negative:** `unit/verify` [`TestRFC6793AS4AggregatorForNonMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L237) |
| `RFC6793-4.2.3-1` | When a NEW BGP speaker receives an update from an OLD BGP speaker, it MUST be prepared to receive the AS4_PATH attribute along with the existing AS_PATH attribute. (Section 4.2.3) | MUST | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L204). **negative:** `unit/verify` [`TestRFC6793ASPathAloneNotInventedIntoFourOctet`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L224) |
| `RFC6793-4.2.3-2` | A NEW BGP speaker MUST also be prepared to receive the AS4_AGGREGATOR attribute along with the AGGREGATOR attribute from an OLD BGP speaker. (Section 4.2.3) | MUST | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793ReceivedAS4AggregatorAccepted`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L239). **negative:** no negative test. **{single-polarity}:** the obligation is to accept the pair, and ParseAttributes has no rejection path to drive negatively: AGGREGATOR is interned and AS4_AGGREGATOR falls through the default branch into OtherAttrs, so every AGGREGATOR plus AS4_AGGREGATOR combination is accepted (internal/component/bgp/plugins/rib/storage/attrparse.go:96-102, :138-140). What ze does with the pair afterwards is governed by RFC6793-4.2.3-3 through -7, which are recorded as gaps |
| `RFC6793-4.2.3-3` | - the AS4_AGGREGATOR attribute and the AS4_PATH attribute SHALL be ignored, (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L339). **negative:** `unit/verify` [`TestRFC6793AggregatorOfTheWrongWidthIsNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L485). **negative:** `unit/verify` [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L385) |
| `RFC6793-4.2.3-4` | When both of the attributes are received, if the AS number in the AGGREGATOR attribute is not AS_TRANS, then: - the AS4_AGGREGATOR attribute and the AS4_PATH attribute SHALL be ignored, - the AGGREGATOR attribute SHALL be taken as the information about the aggregating node (§4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L343). **negative:** `unit/verify` [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L389) |
| `RFC6793-4.2.3-5` | When both of the attributes are received, if the AS number in the AGGREGATOR attribute is not AS_TRANS, then: - the AS4_AGGREGATOR attribute and the AS4_PATH attribute SHALL be ignored, - the AGGREGATOR attribute SHALL be taken as the information about the aggregating node, and - the AS_PATH attribute SHALL be taken as the AS path information. (§4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L347). **negative:** `unit/verify` [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L392) |
| `RFC6793-4.2.3-6` | - the AGGREGATOR attribute SHALL be ignored, (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L378). **negative:** `unit/verify` [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L351) |
| `RFC6793-4.2.3-7` | - the AS4_AGGREGATOR attribute SHALL be taken as the information about the aggregating node, and (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L381). **negative:** `unit/verify` [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L354). **negative:** `unit/verify` [`TestRFC6793LoneAS4AggregatorIsDropped`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L513) |
| `RFC6793-4.2.3-8` | If the number of AS numbers in the AS_PATH attribute is less than the number of AS numbers in the AS4_PATH attribute, then the AS4_PATH attribute SHALL be ignored, and the AS_PATH attribute SHALL be taken as the AS path information. (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793LongerAS4PathIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L279). **negative:** `unit/verify` [`TestRFC6793LongerASPathPrependsLeadingHops`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L262) |
| `RFC6793-4.2.3-9` | If the number of AS numbers in the AS_PATH attribute is larger than or equal to the number of AS numbers in the AS4_PATH attribute, then the AS path information SHALL be constructed by taking as many AS numbers and path segments as necessary from the leading part of the AS_PATH attribute, and then prepending them to the AS4_PATH attribute so that the AS path information has a number of AS numbers identical to that of the AS_PATH attribute. (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793EqualCountsPrependNothing`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L420). **positive:** `unit/verify` [`TestRFC6793LeadingASSetIsTakenWhole`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L440). **positive:** `unit/verify` [`TestRFC6793LongerASPathPrependsLeadingHops`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L257). **negative:** `unit/verify` [`TestRFC6793LongerAS4PathIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L284) |
| `RFC6793-4.2.3-10` | Note that a valid AS_CONFED_SEQUENCE or AS_CONFED_SET path segment SHALL be prepended if it is either the leading path segment or is adjacent to a path segment that is prepended. (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793ConfedSegmentAdjacentToPrependedIsPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_confed_adjacent_test.go#L25). **positive:** `unit/verify` [`TestRFC6793LeadingConfedSegmentIsPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L301). **negative:** `unit/verify` [`TestRFC6793ConfedSegmentAdjacentToPrependedIsPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_confed_adjacent_test.go#L26). **negative:** `unit/verify` [`TestRFC6793UnadjacentConfedSegmentIsNotPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L321) |
| `RFC6793-6-1` | The AS4_PATH attribute in an UPDATE message SHALL be considered malformed under the following conditions: - the attribute length is not a multiple of two or is too small (i.e., less than 6) for the attribute to carry at least one AS number, or - the path segment length in the attribute is either zero or is inconsistent with the attribute length, or - the path segment type in the attribute is not one of the types defined: AS_SEQUENCE, AS_SET, AS_CONFED_SEQUENCE, and AS_CONFED_SET. (§6) | SHALL | 6 | **positive:** `unit/verify` [`TestRFC6793AS4PathWellFormedAccepted`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L166). **positive:** `unit/verify` [`TestRFC6793MalformedAS4PathIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L461). **negative:** `unit/verify` [`TestRFC6793AS4PathMalformedRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L190) |
| `RFC6793-6-2` | The AS4_AGGREGATOR attribute in an UPDATE message SHALL be considered malformed if the attribute length is not 8. (Section 6) | SHALL | 6 | **positive:** `unit/verify` [`TestRFC6793AS4AggregatorLengthEight`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L219). **negative:** `unit/verify` [`TestRFC6793AS4AggregatorLengthEight`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L221) |
| `RFC6793-6-3` | A NEW BGP speaker that receives these path segment types in the AS4_PATH attribute of an UPDATE message from an OLD BGP speaker MUST discard these path segments, adjust the relevant attribute fields accordingly, and continue processing the UPDATE message. (Section 6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC6793ReceivedAS4PathConfedSegmentsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_received_confed_test.go#L44). **positive:** `unit/verify` [`TestRFC6793ReceivedConfedInAS4PathDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L406). **negative:** `unit/verify` [`TestRFC6793ReceivedAS4PathDiscardTakesOnlyConfedSegments`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_received_confed_test.go#L90). **negative:** `unit/verify` [`TestRFC6793ReceivedConfedInAS4PathDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L409) |
| `RFC6793-6-4` | A NEW BGP speaker that receives a malformed AS4_PATH attribute in an UPDATE message from an OLD BGP speaker MUST discard the attribute and continue processing the UPDATE message. (Section 6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC6793MalformedAS4PathDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L343). **negative:** `unit/verify` [`TestRFC6793WellFormedAS4PathNotDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L377) |
| `RFC6793-6-5` | A NEW BGP speaker that receives a malformed AS4_AGGREGATOR attribute in an UPDATE message from an OLD BGP speaker MUST discard the attribute and continue processing the UPDATE message. (Section 6) | MUST | 6 | **positive:** `unit/verify` [`TestCollapseAS4DiscardsMalformedAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_aspath_collapse_test.go#L482). **negative:** `unit/verify` [`TestCollapseAS4SelectsAggregatorPerSection423`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_aspath_collapse_test.go#L350) |
| `RFC6793-6-6` | A NEW BGP speaker that receives the AS4_PATH attribute or the AS4_AGGREGATOR attribute in an UPDATE message from another NEW BGP speaker MUST discard the path attribute and continue processing the UPDATE message. This case SHOULD be logged locally for analysis. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC6793-6-7` | A NEW BGP speaker that receives these path segment types in the AS4_PATH attribute of an UPDATE message from an OLD BGP speaker MUST discard these path segments, adjust the relevant attribute fields accordingly, and continue processing the UPDATE message. This case SHOULD be logged locally for analysis. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC6793-6-8` | A NEW BGP speaker that receives a malformed AS4_PATH attribute in an UPDATE message from an OLD BGP speaker MUST discard the attribute and continue processing the UPDATE message. The error SHOULD be logged locally for analysis. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC6793-6-9` | A NEW BGP speaker that receives a malformed AS4_AGGREGATOR attribute in an UPDATE message from an OLD BGP speaker MUST discard the attribute and continue processing the UPDATE message. The error SHOULD be logged locally for analysis. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC6793-5-1` | Quite clearly, this would not work for a NEW BGP speaker with a non-mappable four-octet AS number. Such BGP speakers should use four-octet AS specific extended communities [RFC5668] instead. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6793-7-1` | When an Autonomous System is using a two-octet AS number, then the BGP speakers within that Autonomous System MAY be upgraded to support the four-octet AS number extensions on a piecemeal basis. (§7) | MAY | 7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 6793 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC6793-4.1-1`](#rfc6793-4.1-1)

A BGP speaker that supports four-octet AS numbers SHALL advertise this to its peers using BGP Capabilities Advertisements (Section 4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793OpenOmitsCapabilityWhenDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L75) | unit/verify | mutant, verified |
| positive | [`TestRFC6793OpenAdvertisesFourOctetASCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L60) | unit/verify | mutant, verified |

### [`RFC6793-4.1-2`](#rfc6793-4.1-2)

The AS number of the BGP speaker MUST be carried in the Capability Value field of the "support for four-octet AS number capability" (Section 4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793ASN4CapabilityValueMustBeFourOctets`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc6793_asn4_test.go#L46) | unit/verify | mutant, verified |
| positive | [`TestRFC6793ASN4CapabilityCarriesSpeakerAS`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc6793_asn4_test.go#L18) | unit/verify | mutant, verified |

### [`RFC6793-4.1-3`](#rfc6793-4.1-3)

When a NEW BGP speaker processes an OPEN message from another NEW BGP speaker, it MUST use the AS number encoded in the Capability Value field of the "support for four-octet AS number capability" in lieu of the "My Autonomous System" field of the OPEN message. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestOpenAdvertisedASFollowsLocalASN4 decodes a received OPEN with My AS 23456, capability AS 4200000000 and Open.ASN4 unset. It asserts that openAdvertisedAS and sessionPeerAS select the capability value between NEW speakers, select My AS when the local speaker does not advertise ASN4, and select My AS when the peer sends no capability. These are distinct conforming controls for the complete field-selection obligation in RFC 6793 Section 4.1. Both recorded OpenPeerAS mutants fail this tagged test at the relevant assertions: substituting My AS fails the NEW-to-NEW controls, and bypassing the local-advertisement guard fails the OLD-local controls. Source tracing confirms validateOpenPeerAS uses the same OpenPeerAS producer before handleOpen accepts the peer, and negotiateWith publishes the identity through judgedPeerAS and sessionPeerAS. Existing TestOpenPeerASIsMyASWhenZeDoesNotAdvertiseASN4 additionally asserts handleOpen acceptance and OpenConfirm for the capability AS, rejection of AS_TRANS on a NEW session, and the opposite outcomes on an OLD-local session; TestASN4DisabledSessionReadersJudgeMyAS asserts the negotiated identity and reader controls. Those supplemental tests were source-reviewed, not run by this audit. The new tags and recorded breaks prove shared-reader field selection; they do not themselves constitute a peer OPEN sent through the socket read loop or a complete session-establishment integration test.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestOpenAdvertisedASFollowsLocalASN4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_session_open_as_asn4_test.go#L52) | unit/verify | mutant, verified |
| positive | [`TestOpenAdvertisedASFollowsLocalASN4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_session_open_as_asn4_test.go#L51) | unit/verify | mutant, verified |

### [`RFC6793-4.1-4`](#rfc6793-4.1-4)

A BGP speaker that advertises such a capability to a particular peer, and receives from that peer the advertisement of such a capability, MUST encode AS numbers as four-octet entities in both the AS_PATH attribute and the AGGREGATOR attribute in the updates it sends to the peer (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793EncodeFourOctetToNewSpeaker drives ASPath/Aggregator WriteToWithContext with an ASN4 context and asserts 4-byte AS_PATH stride and 8-octet AGGREGATOR; the negative shows the 2-octet form without the capability.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793EncodeTwoOctetToOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L65) | unit/verify | mutant, verified |
| positive | [`TestRFC6793EncodeFourOctetToNewSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L33) | unit/verify | mutant, verified |

### [`RFC6793-4.1-5`](#rfc6793-4.1-5)

A BGP speaker that advertises such a capability to a particular peer, and receives from that peer the advertisement of such a capability, MUST encode AS numbers as four-octet entities in both the AS_PATH attribute and the AGGREGATOR attribute in the updates it sends to the peer and MUST assume that these attributes in the updates received from the peer encode AS numbers as four-octet entities. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793DecodeFourOctetWhenNegotiated calls ParseASPath and ParseAggregator with fourByte=true and recovers AS 4200000001 from four ASN octets and an 8-octet AGGREGATOR. TestRFC6793DecodeTwoOctetWhenNotNegotiated calls ParseASPath with fourByte=false over the same four ASN octets but a segment count of two, asserting AS 0xFA56 and AS 0xEA01; it refuses the 8-octet AGGREGATOR with ErrInvalidLength and accepts the valid 6-octet AS_TRANS form. These are codec-width assertions; they do not themselves exercise capability negotiation through a session.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793DecodeTwoOctetWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L138) | unit/verify | mutant, verified |
| positive | [`TestRFC6793DecodeFourOctetWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L113) | unit/verify | mutant, verified |

### [`RFC6793-4.1-6`](#rfc6793-4.1-6)

The new attributes, AS4_PATH and AS4_AGGREGATOR, MUST NOT be carried in an UPDATE message between NEW BGP speakers. (Section 4.1, Section 6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (rfc6793): "The new attributes, AS4_PATH and AS4_AGGREGATOR, MUST NOT be carried in an UPDATE message between NEW BGP speakers." The only edit to test/plugin/rfc6793-narrow-to-old-speaker.ci, test/plugin/rfc6793-no-as4aggregator-from-new-speaker.ci, test/plugin/rfc6793-no-as4path-from-new-speaker.ci since the last judgement is f8f7628bdf, which adds a bgp-level filter { import { final accept; } export { final accept; } } with no filter refs, because RFC 8212 Section 3 now makes an EBGP direction with no explicit policy reject. No expect, reject, input or requirement-tag line changed. policy.ConcatChains keeps every peer-level ref and takes the final from the most specific configured level, so a peer-level chain still decides where one exists. An accepting final with no refs neither refuses nor rewrites a route: a withheld, stripped or discarded outcome the file asserts can still come only from the producer under test, and a delivered outcome is the eligibility the test was written under before RFC 8212; without the block the positives would fail for the wrong reason. What the unit proves is unchanged. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`rfc6793-narrow-to-old-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-narrow-to-old-speaker.ci#L25) | functional/verify | revert, verified |
| positive | [`TestOriginatedUpdateOmitsAS4PathTowardNewSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L290) | unit/verify | revert, verified |
| positive | [`TestOriginatedAggregatorOmitsAS4AggregatorTowardNewSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4agg_new_test.go#L12) | unit/verify | revert, verified |
| positive | [`rfc6793-no-as4aggregator-from-new-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-no-as4aggregator-from-new-speaker.ci#L12) | functional/verify | revert, verified |
| positive | [`rfc6793-no-as4path-from-new-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-no-as4path-from-new-speaker.ci#L29) | functional/verify | revert, verified |

### [`RFC6793-4.1-7`](#rfc6793-4.1-7)

A NEW BGP speaker that receives the AS4_PATH attribute or the AS4_AGGREGATOR attribute in an UPDATE message from another NEW BGP speaker MUST discard the path attribute and continue processing the UPDATE message. (Section 4.1, Section 6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (rfc6793): "A NEW BGP speaker that receives the AS4_PATH attribute or the AS4_AGGREGATOR attribute in an UPDATE message from another NEW BGP speaker MUST discard the path attribute and continue processing the UPDATE message." The only edit to test/plugin/rfc6793-no-as4path-from-new-speaker.ci since the last judgement is f8f7628bdf, which adds a bgp-level filter { import { final accept; } export { final accept; } } with no filter refs, because RFC 8212 Section 3 now makes an EBGP direction with no explicit policy reject. No expect, reject, input or requirement-tag line changed. policy.ConcatChains keeps every peer-level ref and takes the final from the most specific configured level, so a peer-level chain still decides where one exists. An accepting final with no refs neither refuses nor rewrites a route: a withheld, stripped or discarded outcome the file asserts can still come only from the producer under test, and a delivered outcome is the eligibility the test was written under before RFC 8212; without the block the positives would fail for the wrong reason. What the unit proves is unchanged. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L207) | unit/verify | revert, verified |
| positive | [`TestRFC6793NewSpeakerAS4AttributesAreDiscarded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L533) | unit/verify | revert, verified |
| positive | [`rfc6793-no-as4path-from-new-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-no-as4path-from-new-speaker.ci#L34) | functional/verify | revert, verified |

### [`RFC6793-4.2.1-1`](#rfc6793-4.2.1-1)

However, this document does not assume that an Autonomous System with NEW BGP speakers has to have a globally unique two-octet AS number -- AS_TRANS MUST be used when the NEW BGP speaker does not have a two-octet AS number (even if multiple Autonomous Systems would use it). (Section 4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793OpenMyASIsASTransWithoutTwoOctetAS asserts the sent OPEN My AS is 23456 for AS 4200000001; the negative keeps 65001 as itself.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793OpenMyASIsRealASWhenMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L104) | unit/verify | mutant, verified |
| positive | [`TestRFC6793OpenMyASIsASTransWithoutTwoOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L88) | unit/verify | mutant, verified |

### [`RFC6793-4.2.2-1`](#rfc6793-4.2.2-1)

When communicating with an OLD BGP speaker, a NEW BGP speaker MUST send the AS path information in the AS_PATH attribute encoded with two-octet AS numbers. (Section 4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-03 by an independent auditor who authored none of 6b105ebc58 or c782411791. RFC 6793 Section 4.2.2 read: "When communicating with an OLD BGP speaker, a NEW BGP speaker MUST send the AS path information in the AS_PATH attribute encoded with two-octet AS numbers." TestRFC6996OldSpeakerReceivesStrippedASPathAndAS4Path changed one line: the literal one-step chain list became buildOrderedEgressSteps(), which production also installs (reactor.go) and which returns that single export-chain step; input, export member and the exact AS_PATH octets 0204 fde8 fbf0 5ba0 fbf1 are untouched. Scoped run green. Its mutant record is unit-changed and owes a re-record. Verdict unchanged. Earlier judgement, still standing for the unmoved units: Independent re-judge after 4ceb0f7416. Section 4.2.2 read in rfc/full/rfc6793.txt: "When communicating with an OLD BGP speaker, a NEW BGP speaker MUST send the AS path information in the AS_PATH attribute encoded with two-octet AS numbers." and "non-mappable four-octet AS numbers are represented by the well-known two-octet AS number, AS_TRANS." New positive TestRFC6996OldSpeakerReceivesStrippedASPathAndAS4Path asserts the exact AS_PATH octets 0204 fde8 fbf0 5ba0 fbf1 toward a two-octet EBGP peer on the real forward path: two-octet widths (a four-octet encoding changes every byte) and AS_TRANS 23456 for 65536. Break attribute/aspath.go:184 writeSegmentWithSplit as16=23456 -> uint16(asns[i]) (65536 truncates to 0) is the producing statement and was observed red. Claim states only what the bytes show. Existing units unchanged: TestRFC6793EncodeTwoOctetToOldSpeaker asserts a 2-octet AS_PATH with AS_TRANS toward a non-ASN4 context; the negative TestRFC6793TwoOctetASPathKeepsMappableASNs keeps mappable ASNs unsubstituted, so over-substitution is caught.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793TwoOctetASPathKeepsMappableASNs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L93) | unit/verify | not verified |
| positive | [`TestRFC6996OldSpeakerReceivesStrippedASPathAndAS4Path`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6996_old_speaker_export_test.go#L33) | unit/verify | mutant, verified |
| positive | [`TestRFC6793EncodeTwoOctetToOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L68) | unit/verify | not verified |

### [`RFC6793-4.2.2-2`](#rfc6793-4.2.2-2)

The NEW BGP speaker MUST also send the AS path information in the AS4_PATH attribute (encoded with four-octet AS numbers), except for the case where all of the AS path information is composed of mappable four-octet AS numbers only. (Section 4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-03 by an independent auditor who authored none of 6b105ebc58 or c782411791. RFC 6793 Section 4.2.2 read: "The NEW BGP speaker MUST also send the AS path information in the AS4_PATH attribute (encoded with four-octet AS numbers), except for the case where all of the AS path information is composed of mappable four-octet AS numbers only." TestRFC6996OldSpeakerReceivesStrippedASPathAndAS4Path changed one line: the literal one-step chain list became buildOrderedEgressSteps(), the production builder; the exact AS4_PATH octets 0204 0000fde8 0000fbf0 00010000 0000fbf1 are untouched. Scoped run green. Its mutant record is unit-changed and owes a re-record. Verdict unchanged. Earlier judgement, still standing for the unmoved units: Independent re-judge after 4ceb0f7416. Section 4.2.2 read in rfc/full/rfc6793.txt: "The NEW BGP speaker MUST also send the AS path information in the AS4_PATH attribute (encoded with four-octet AS numbers), except for the case where all of the AS path information is composed of mappable four-octet AS numbers only." New positive TestRFC6996OldSpeakerReceivesStrippedASPathAndAS4Path asserts the exact AS4_PATH value 0204 0000fde8 0000fbf0 00010000 0000fbf1 ([65000 64496 65536 64497]) toward a two-octet EBGP peer on the real forward path: present, four-octet encoded, and carrying the same path information as the AS_PATH with the real AS in place of AS_TRANS. Break attribute/as4.go:434 AS4PathFor returning nil removes the attribute and was observed red. The except clause stays covered by the unchanged negatives TestOriginatedUpdateOmitsAS4PathWhenEveryASIsMappable and TestRFC6793TranscodeOmitsAS4PathWhenAllMappable (no AS4_PATH when every AS is mappable); unchanged positives in message and wireu assert AS4_PATH with the real four-octet AS toward an OLD speaker. Whole sentence covered, both polarities genuine.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestOriginatedUpdateOmitsAS4PathWhenEveryASIsMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L272) | unit/verify | revert, verified |
| negative | [`TestRFC6793TranscodeOmitsAS4PathWhenAllMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L87) | unit/verify | not verified |
| positive | [`TestOriginatedUpdateCarriesAS4PathForANonMappableLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L243) | unit/verify | revert, verified |
| positive | [`TestOriginatedUpdateCarriesAS4PathTowardOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L205) | unit/verify | revert, verified |
| positive | [`TestRFC6996OldSpeakerReceivesStrippedASPathAndAS4Path`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6996_old_speaker_export_test.go#L40) | unit/verify | mutant, verified |
| positive | [`TestRFC6793TranscodeEmitsAS4PathForNonMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L52) | unit/verify | not verified |

### [`RFC6793-4.2.2-3`](#rfc6793-4.2.2-3)

In this case, the NEW BGP speaker MUST NOT send the AS4_PATH attribute. (Section 4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Originate builders and TranscodeASPath omit AS4_PATH when all ASNs are mappable; negatives show it present for a non-mappable AS.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestOriginatedUpdateCarriesAS4PathTowardOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L208) | unit/verify | revert, verified |
| negative | [`TestRFC6793TranscodeEmitsAS4PathForNonMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L55) | unit/verify | not verified |
| positive | [`TestOriginatedUpdateOmitsAS4PathWhenEveryASIsMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L270) | unit/verify | revert, verified |
| positive | [`TestRFC6793TranscodeOmitsAS4PathWhenAllMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L85) | unit/verify | not verified |

### [`RFC6793-4.2.2-4`](#rfc6793-4.2.2-4)

Whenever the AS path information contains the AS_CONFED_SEQUENCE or AS_CONFED_SET path segment, the NEW BGP speaker MUST exclude such path segments from the AS4_PATH attribute being constructed. (Section 4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both confederation types now proven excluded from the constructed AS4_PATH. TestRFC6793ConstructedAS4PathExcludesConfedSet: AS_CONFED_SET {64514,64515}, AS_CONFED_SEQUENCE [64512], AS_SEQUENCE [non-mappable, 65001] through TranscodeASPath 4->2 yields an AS4_PATH of exactly the one AS_SEQUENCE, AS_PATH keeping all three. The AS_CONFED_SEQUENCE half is also held by the HEAD TestRFC6793ConstructedAS4PathExcludesConfed (exact single-segment assertion, no record) and by the AS_CONFED_SEQUENCE inside the new recorded unit. Negative TestRFC6793ConstructedAS4PathKeepsAnOrdinarySet: an ordinary AS_SET in the same place is carried, so the exclusion is scoped to type 3/4. Judge break (overlay, attribute/as4.go AS4Path.Len and WriteTo excluding only AS_CONFED_SEQUENCE) turned the new positive red, the HEAD sequence-only unit and the new negative green. Revert records on TranscodeASPath both polarities.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793ConstructedAS4PathExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L304) | unit/verify | not verified |
| negative | [`TestRFC6793ConstructedAS4PathKeepsAnOrdinarySet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_confed_set_test.go#L57) | unit/verify | revert, verified |
| positive | [`TestRFC6793ConstructedAS4PathExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L302) | unit/verify | not verified |
| positive | [`TestRFC6793ConstructedAS4PathExcludesConfedSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_confed_set_test.go#L38) | unit/verify | revert, verified |

### [`RFC6793-3-1`](#rfc6793-3-1)

To prevent the possible propagation of Confederation-related path segments outside of a Confederation, the path segment types AS_CONFED_SEQUENCE and AS_CONFED_SET [RFC5065] are declared invalid for the AS4_PATH attribute and MUST NOT be included in the AS4_PATH attribute of an UPDATE message. (Section 3, Section 6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793AS4PathWireExcludesConfed: AS4Path.WriteTo/Len skip both confed types and keep AS_SEQUENCE/AS_SET, with Len matching bytes written.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793AS4PathWireExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L253) | unit/verify | mutant, verified |
| positive | [`TestRFC6793AS4PathWireExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L250) | unit/verify | mutant, verified |

### [`RFC6793-4.2.2-5`](#rfc6793-4.2.2-5)

Similarly, if the NEW BGP speaker has to send the AGGREGATOR attribute, and if the aggregating Autonomous System's AS number is a non-mappable four-octet AS number, then the speaker MUST use the AS4_AGGREGATOR attribute and set the AS number field in the existing AGGREGATOR attribute to the reserved AS number, AS_TRANS. (Section 4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Three producers (message originate, wireu TranscodeASPath, rib commit pack) each send AS_TRANS in a 6-octet AGGREGATOR plus AS4_AGGREGATOR with the real AS for a non-mappable aggregator; negatives keep a mappable AS with no companion.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestOriginatedMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L445) | unit/verify | revert, verified |
| negative | [`TestAMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L107) | unit/verify | not verified |
| negative | [`TestRFC6793NoAS4AggregatorForMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L271) | unit/verify | not verified |
| positive | [`TestOriginatedAggregatorCarriesItsCompanionTowardOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L415) | unit/verify | revert, verified |
| positive | [`TestForwardedAggregatorIsDowngradedWithItsCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L70) | unit/verify | not verified |
| positive | [`TestRFC6793AS4AggregatorForNonMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L234) | unit/verify | not verified |

### [`RFC6793-4.2.2-6`](#rfc6793-4.2.2-6)

Note that if the AS number is mappable, then the AS4_AGGREGATOR attribute MUST NOT be sent. (Section 4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Same three producers send no AS4_AGGREGATOR for a mappable aggregating AS; negatives show it present for a non-mappable one.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestOriginatedAggregatorCarriesItsCompanionTowardOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L418) | unit/verify | revert, verified |
| negative | [`TestForwardedAggregatorIsDowngradedWithItsCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L73) | unit/verify | not verified |
| negative | [`TestRFC6793AS4AggregatorForNonMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L237) | unit/verify | not verified |
| positive | [`TestOriginatedMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L443) | unit/verify | revert, verified |
| positive | [`TestAMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L105) | unit/verify | not verified |
| positive | [`TestRFC6793NoAS4AggregatorForMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L269) | unit/verify | not verified |

### [`RFC6793-4.2.3-1`](#rfc6793-4.2.3-1)

When a NEW BGP speaker receives an update from an OLD BGP speaker, it MUST be prepared to receive the AS4_PATH attribute along with the existing AS_PATH attribute. (Section 4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath: ReconcileASPathFamily accepts AS_PATH+AS4_PATH from an OLD speaker and uses the AS4_PATH ASNs; the negative shows no 4-octet AS invented without AS4_PATH.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793ASPathAloneNotInventedIntoFourOctet`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L224) | unit/verify | revert, verified |
| positive | [`TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L204) | unit/verify | revert, verified |

### [`RFC6793-4.2.3-2`](#rfc6793-4.2.3-2)

A NEW BGP speaker MUST also be prepared to receive the AS4_AGGREGATOR attribute along with the AGGREGATOR attribute from an OLD BGP speaker. (Section 4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793ReceivedAS4AggregatorAccepted: the AGGREGATOR+AS4_AGGREGATOR pair from an OLD speaker is accepted and the 4-octet aggregating AS survives ingest (single-polarity marker).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC6793ReceivedAS4AggregatorAccepted`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L239) | unit/verify | revert, verified |

### [`RFC6793-4.2.3-3`](#rfc6793-4.2.3-3)

- the AS4_AGGREGATOR attribute and the AS4_PATH attribute SHALL be ignored, (Section 4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793AggregatorWithRealASIgnoresAS4Attributes: with AGGREGATOR.AS 64500 the AS4_PATH does not reach the path and the AS4_AGGREGATOR is discarded; negatives with AS_TRANS use both.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793AggregatorOfTheWrongWidthIsNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L485) | unit/verify | revert, verified |
| negative | [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L385) | unit/verify | revert, verified |
| positive | [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L339) | unit/verify | revert, verified |

### [`RFC6793-4.2.3-4`](#rfc6793-4.2.3-4)

When both of the attributes are received, if the AS number in the AGGREGATOR attribute is not AS_TRANS, then: - the AS4_AGGREGATOR attribute and the AS4_PATH attribute SHALL be ignored, - the AGGREGATOR attribute SHALL be taken as the information about the aggregating node (§4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Same unit: the AGGREGATOR (64500, 10.0.0.1) is the aggregating node; the AS_TRANS counterpart does not take it.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L389) | unit/verify | revert, verified |
| positive | [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L343) | unit/verify | revert, verified |

### [`RFC6793-4.2.3-5`](#rfc6793-4.2.3-5)

When both of the attributes are received, if the AS number in the AGGREGATOR attribute is not AS_TRANS, then: - the AS4_AGGREGATOR attribute and the AS4_PATH attribute SHALL be ignored, - the AGGREGATOR attribute SHALL be taken as the information about the aggregating node, and - the AS_PATH attribute SHALL be taken as the AS path information. (§4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Same unit: the route path is the AS_PATH widened with AS_TRANS intact; the AS_TRANS counterpart reconstructs instead.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L392) | unit/verify | revert, verified |
| positive | [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L347) | unit/verify | revert, verified |

### [`RFC6793-4.2.3-6`](#rfc6793-4.2.3-6)

- the AGGREGATOR attribute SHALL be ignored, (Section 4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793AggregatorWithASTransPromotesAS4Aggregator: with AGGREGATOR.AS AS_TRANS the stored aggregator is the AS4_AGGREGATOR value, not AS_TRANS; negative keeps a real-AS AGGREGATOR.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L351) | unit/verify | revert, verified |
| positive | [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L378) | unit/verify | revert, verified |

### [`RFC6793-4.2.3-7`](#rfc6793-4.2.3-7)

- the AS4_AGGREGATOR attribute SHALL be taken as the information about the aggregating node, and (Section 4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Same unit: the aggregating node is 4200000001/10.0.0.1 from AS4_AGGREGATOR; negatives: not promoted when AGGREGATOR is real, nor when AS4_AGGREGATOR arrives alone.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L354) | unit/verify | revert, verified |
| negative | [`TestRFC6793LoneAS4AggregatorIsDropped`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L513) | unit/verify | revert, verified |
| positive | [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L381) | unit/verify | revert, verified |

### [`RFC6793-4.2.3-8`](#rfc6793-4.2.3-8)

If the number of AS numbers in the AS_PATH attribute is less than the number of AS numbers in the AS4_PATH attribute, then the AS4_PATH attribute SHALL be ignored, and the AS_PATH attribute SHALL be taken as the AS path information. (Section 4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793LongerAS4PathIsIgnored: AS_PATH 2 ASNs vs AS4_PATH 3 yields the widened AS_PATH only; negative shows AS4_PATH used when AS_PATH is longer.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793LongerASPathPrependsLeadingHops`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L262) | unit/verify | revert, verified |
| positive | [`TestRFC6793LongerAS4PathIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L279) | unit/verify | revert, verified |

### [`RFC6793-4.2.3-9`](#rfc6793-4.2.3-9)

If the number of AS numbers in the AS_PATH attribute is larger than or equal to the number of AS numbers in the AS4_PATH attribute, then the AS path information SHALL be constructed by taking as many AS numbers and path segments as necessary from the leading part of the AS_PATH attribute, and then prepending them to the AS4_PATH attribute so that the AS path information has a number of AS numbers identical to that of the AS_PATH attribute. (Section 4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793LongerASPathPrependsLeadingHops, EqualCountsPrependNothing, LeadingASSetIsTakenWhole assert exact reconstructed bytes; negative: no prepend when AS4_PATH is longer.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793LongerAS4PathIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L284) | unit/verify | revert, verified |
| positive | [`TestRFC6793EqualCountsPrependNothing`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L420) | unit/verify | revert, verified |
| positive | [`TestRFC6793LeadingASSetIsTakenWhole`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L440) | unit/verify | revert, verified |
| positive | [`TestRFC6793LongerASPathPrependsLeadingHops`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L257) | unit/verify | revert, verified |

### [`RFC6793-4.2.3-10`](#rfc6793-4.2.3-10)

Note that a valid AS_CONFED_SEQUENCE or AS_CONFED_SET path segment SHALL be prepended if it is either the leading path segment or is adjacent to a path segment that is prepended. (Section 4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. TestRFC6793ConfedSegmentAdjacentToPrependedIsPrepended drives ReconcileASPathFamily: + AS_SEQ 64500, AS_CONFED_SEQ 65001, AS_SEQ TRANS TRANS beside a two-hop AS4_PATH keeps the confed segment after the prepended 64500; - the same confed segment behind a non-prepended AS_SEQ is dropped. Leading-segment clause is TestRFC6793LeadingConfedSegmentIsPrepended. Judge breaks in appendLeadingSegments: continuing past a non-prepended segment turned the negative red; stopping before a confed segment once the budget is spent turned the positive red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793ConfedSegmentAdjacentToPrependedIsPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_confed_adjacent_test.go#L26) | unit/verify | revert, verified |
| negative | [`TestRFC6793UnadjacentConfedSegmentIsNotPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L321) | unit/verify | revert, verified |
| positive | [`TestRFC6793ConfedSegmentAdjacentToPrependedIsPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_confed_adjacent_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestRFC6793LeadingConfedSegmentIsPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L301) | unit/verify | revert, verified |

### [`RFC6793-6-1`](#rfc6793-6-1)

The AS4_PATH attribute in an UPDATE message SHALL be considered malformed under the following conditions: - the attribute length is not a multiple of two or is too small (i.e., less than 6) for the attribute to carry at least one AS number, or - the path segment length in the attribute is either zero or is inconsistent with the attribute length, or - the path segment type in the attribute is not one of the types defined: AS_SEQUENCE, AS_SET, AS_CONFED_SEQUENCE, and AS_CONFED_SET. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793AS4PathMalformedRejected drives every Section 6 condition (odd length, <6 octets, zero and overrunning segment length, undefined type) through ParseAS4Path; positive accepts a well-formed path. The tag at rfc6793_reconcile_test.go TestRFC6793MalformedAS4PathIsDiscarded describes the RFC6793-6-4 discard obligation rather than this row.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793AS4PathMalformedRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L190) | unit/verify | not verified |
| positive | [`TestRFC6793AS4PathWellFormedAccepted`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L166) | unit/verify | not verified |
| positive | [`TestRFC6793MalformedAS4PathIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L461) | unit/verify | revert, verified |

### [`RFC6793-6-2`](#rfc6793-6-2)

The AS4_AGGREGATOR attribute in an UPDATE message SHALL be considered malformed if the attribute length is not 8. (Section 6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793AS4AggregatorLengthEight: ParseAS4Aggregator accepts 8 octets and refuses 0,6,7,9,12 with ErrInvalidLength.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793AS4AggregatorLengthEight`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L221) | unit/verify | mutant, verified |
| positive | [`TestRFC6793AS4AggregatorLengthEight`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L219) | unit/verify | mutant, verified |

### [`RFC6793-6-3`](#rfc6793-6-3)

A NEW BGP speaker that receives these path segment types in the AS4_PATH attribute of an UPDATE message from an OLD BGP speaker MUST discard these path segments, adjust the relevant attribute fields accordingly, and continue processing the UPDATE message. (Section 6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c34 judge). internal/component/bgp/wireu/rfc6793_received_confed_test.go reads the output of CollapseAS4Family, the single ingest rewrite every downstream consumer sees, so the receive-side discard is measured apart from the AS4_PATH encoder's own confed filter (RFC6793-3-1). Positive: an OLD speaker's AS4_PATH holding an AS_CONFED_SEQUENCE and an AS_CONFED_SET ahead of an AS_SEQUENCE collapses to the exact payload (AS_PATH 40 02 10: [65001] then [4200000001,4200000002], attribute section 0x1E, NLRI kept, no error): both segment types discarded, lengths adjusted, processing continues (judge checked the RFC 6793 Section 4.2.3 arithmetic and both length octets). Negative: an AS_SET in the AS4_PATH and a leading AS_CONFED_SEQUENCE in the AS_PATH both survive (payload literal, 0x1C / 0x2A), so the discard is not blanket. Records: attribute/as4.go::appendAS4Segments revert, both polarities observed red; author overlay probes (kept confed segments -> only the positive red, discard widened to AS_SET -> only the negative red) re-read by the judge in scratch c34p. The HEAD egress units in rfc6793_as4_test.go stay as unrecorded supplementary tags. The Section 6 SHOULD-log clause is not this row.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793ReceivedConfedInAS4PathDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L409) | unit/verify | not verified |
| negative | [`TestRFC6793ReceivedAS4PathDiscardTakesOnlyConfedSegments`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_received_confed_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestRFC6793ReceivedConfedInAS4PathDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L406) | unit/verify | not verified |
| positive | [`TestRFC6793ReceivedAS4PathConfedSegmentsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_received_confed_test.go#L44) | unit/verify | revert, verified |

### [`RFC6793-6-4`](#rfc6793-6-4)

A NEW BGP speaker that receives a malformed AS4_PATH attribute in an UPDATE message from an OLD BGP speaker MUST discard the attribute and continue processing the UPDATE message. (Section 6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793MalformedAS4PathDiscarded: an odd-length AS4_PATH from an OLD speaker contributes nothing and the rewrite proceeds; negative keeps a well-formed one.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6793WellFormedAS4PathNotDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L377) | unit/verify | mutant, verified |
| positive | [`TestRFC6793MalformedAS4PathDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L343) | unit/verify | mutant, verified |

### [`RFC6793-6-5`](#rfc6793-6-5)

A NEW BGP speaker that receives a malformed AS4_AGGREGATOR attribute in an UPDATE message from an OLD BGP speaker MUST discard the attribute and continue processing the UPDATE message. (Section 6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestCollapseAS4DiscardsMalformedAS4Aggregator: a 7-octet AS4_AGGREGATOR is removed from the collapsed payload, not promoted, NLRI kept, drop reported; negative reads an 8-octet one.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestCollapseAS4SelectsAggregatorPerSection423`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_aspath_collapse_test.go#L350) | unit/verify | revert, verified |
| positive | [`TestCollapseAS4DiscardsMalformedAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_aspath_collapse_test.go#L482) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc6793.txt |
| Source fingerprint | c532462ab07361de |
| Record | rfc/extraction/rfc6793.json |
| Mapped sentences | 29 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 1 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 6 | reviewed | not stated |
| `4.2` | not stated | 0 | reviewed | not stated |
| `4.2.1` | not stated | 1 | reviewed | not stated |
| `4.2.2` | not stated | 6 | reviewed | not stated |
| `4.2.3` | not stated | 10 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 8 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |
| `11` | not stated | 0 | reviewed | not stated |
| `12` | not stated | 0 | reviewed | not stated |
| `12.1` | not stated | 0 | reviewed | not stated |
| `12.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `6:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6 restates the Section 4.1 prohibition word for word; site 4.1:5 carries the same sentence and maps RFC6793-4.1-6, whose checklist row cites both sections. | The AS4_PATH attribute and AS4_AGGREGATOR attribute MUST NOT be carried in an UPDATE message between NEW BGP speakers. |
| `6:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6 restates the Section 4.1 discard rule word for word; site 4.1:6 carries the same sentence and maps RFC6793-4.1-7, whose checklist row cites both sections. | A NEW BGP speaker that receives the AS4_PATH attribute or the AS4_AGGREGATOR attribute in an UPDATE message from another NEW BGP speaker MUST discard the path attribute and continue processing the UPDATE message. |
| `6:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6 restates the Section 3 prohibition on AS_CONFED_SEQUENCE and AS_CONFED_SET inside AS4_PATH; site 3:1 maps RFC6793-3-1, whose checklist row cites both sections. | In addition, the path segment types AS_CONFED_SEQUENCE and AS_CONFED_SET [RFC5065] MUST NOT be carried in the AS4_PATH attribute of an UPDATE message. |

## Superseded

No document obsoletes RFC 6793, so its obligations are stated where they were written.
