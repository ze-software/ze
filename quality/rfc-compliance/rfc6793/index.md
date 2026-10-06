# RFC 6793 - BGP Support for Four-Octet Autonomous System (AS) Number Space

Partial. Every requirement this repository extracted from RFC 6793, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 93.3% | 28 of 30 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 3.3% | 1 of 30 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 30 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 30 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 57.6% | 49 of 85 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 30 | of 36 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 30 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 30 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 30 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 30 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 3.3% | 1 of 30 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 30 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 36 |
| Gated MUST-level | 30 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 85 |
| Tagged units | 85 |
| Recorded audit verdicts | 27 |
| Discrimination records | 49 |
| Summary | `rfc/short/rfc6793.md` |
| Requirement shard | `rfc/requirements/rfc6793.md` |
| RFC text | `rfc/full/rfc6793.txt` |

## Enrolment

Enrolled: BGP Support for Four-Octet AS Number Space

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- ASN4 capability advertisement and negotiation, 4-octet AS_PATH/AGGREGATOR between NEW speakers, 2-octet AS_PATH with AS_TRANS toward OLD speakers, AS4_PATH and AS4_AGGREGATOR construction (confederation segments excluded), the whole receive-side procedure of Section 4.2.3 (the AGGREGATOR versus AS4_AGGREGATOR choice, the AS-number-count comparison, the leading-segment prepend and its confederation adjacency rule), the Section 4.1 and Section 6 discard of an AS4_PATH or AS4_AGGREGATOR received from a NEW speaker, AS4_PATH/AS4_AGGREGATOR malformed-attribute validation, AS_TRANS in the OPEN My AS field
- tests bound per requirement in [`rfc/requirements/rfc6793.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc6793.md).


**What the ledger says remains**

One MUST-level gap, annotated in [`rfc/short/rfc6793.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc6793.md).

- **Peer identity:** [`RFC6793-4.1-3`](#rfc6793-4.1-3) -- `UnpackOpen` never populates `Open.ASN4` from the code-65 capability ([`internal/component/bgp/message/open.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open.go)), so the session's OPEN-derived peer AS falls back to the two-octet My AS field ([`internal/component/bgp/reactor/reactor_dynamic.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_dynamic.go)).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 28 | one part of the gated population |
| Annotated (including scoped evidence) | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **30** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (28):** [`RFC6793-4.1-1`](#rfc6793-4.1-1), [`RFC6793-4.1-2`](#rfc6793-4.1-2), [`RFC6793-4.1-4`](#rfc6793-4.1-4), [`RFC6793-4.1-5`](#rfc6793-4.1-5), [`RFC6793-4.1-6`](#rfc6793-4.1-6), [`RFC6793-4.1-7`](#rfc6793-4.1-7), [`RFC6793-4.2.1-1`](#rfc6793-4.2.1-1), [`RFC6793-4.2.2-1`](#rfc6793-4.2.2-1), [`RFC6793-4.2.2-2`](#rfc6793-4.2.2-2), [`RFC6793-4.2.2-3`](#rfc6793-4.2.2-3), [`RFC6793-4.2.2-4`](#rfc6793-4.2.2-4), [`RFC6793-3-1`](#rfc6793-3-1), [`RFC6793-4.2.2-5`](#rfc6793-4.2.2-5), [`RFC6793-4.2.2-6`](#rfc6793-4.2.2-6), [`RFC6793-4.2.3-1`](#rfc6793-4.2.3-1), [`RFC6793-4.2.3-3`](#rfc6793-4.2.3-3), [`RFC6793-4.2.3-4`](#rfc6793-4.2.3-4), [`RFC6793-4.2.3-5`](#rfc6793-4.2.3-5), [`RFC6793-4.2.3-6`](#rfc6793-4.2.3-6), [`RFC6793-4.2.3-7`](#rfc6793-4.2.3-7), [`RFC6793-4.2.3-8`](#rfc6793-4.2.3-8), [`RFC6793-4.2.3-9`](#rfc6793-4.2.3-9), [`RFC6793-4.2.3-10`](#rfc6793-4.2.3-10), [`RFC6793-6-1`](#rfc6793-6-1), [`RFC6793-6-2`](#rfc6793-6-2), [`RFC6793-6-3`](#rfc6793-6-3), [`RFC6793-6-4`](#rfc6793-6-4), [`RFC6793-6-5`](#rfc6793-6-5)

**Annotated (including scoped evidence) (2):** [`RFC6793-4.1-3`](#rfc6793-4.1-3), [`RFC6793-4.2.3-2`](#rfc6793-4.2.3-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6793-4.1-1` | A BGP speaker that supports four-octet AS numbers SHALL advertise this to its peers using BGP Capabilities Advertisements (Section 4.1) | SHALL | 4.1 | **positive:** `unit/verify` [`TestRFC6793OpenAdvertisesFourOctetASCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L60). **negative:** `unit/verify` [`TestRFC6793OpenOmitsCapabilityWhenDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L75) |
| `RFC6793-4.1-2` | The AS number of the BGP speaker MUST be carried in the Capability Value field of the "support for four-octet AS number capability" (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC6793ASN4CapabilityCarriesSpeakerAS`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc6793_asn4_test.go#L18). **negative:** `unit/verify` [`TestRFC6793ASN4CapabilityValueMustBeFourOctets`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc6793_asn4_test.go#L46) |
| `RFC6793-4.1-3` | When a NEW BGP speaker processes an OPEN message from another NEW BGP speaker, it MUST use the AS number encoded in the Capability Value field of the "support for four-octet AS number capability" in lieu of the "My Autonomous System" field of the OPEN message. (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** UnpackOpen never populates Open.ASN4 from the code-65 capability -- it sets only Version, MyAS, HoldTime and BGPIdentifier (internal/component/bgp/message/open.go:171-180) -- so the reactor's only OPEN-derived peer AS falls back to the two-octet header field: resolveDynamicPeerSettings reads the always-zero open.ASN4 and keeps uint32(open.MyAS), i.e. AS_TRANS for a non-mappable peer (internal/component/bgp/reactor/reactor_dynamic.go:311-316), and negotiateWith passes that same zero as the peer ASN into Negotiate (internal/component/bgp/reactor/session_negotiate.go:27-32). The route-server plugin does read the capability value for its event view (internal/component/bgp/plugins/rs/server.go:647-649), but that is a reporting path, not the session's peer AS |
| `RFC6793-4.1-4` | A BGP speaker that advertises such a capability to a particular peer, and receives from that peer the advertisement of such a capability, MUST encode AS numbers as four-octet entities in both the AS_PATH attribute and the AGGREGATOR attribute in the updates it sends to the peer (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC6793EncodeFourOctetToNewSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L33). **negative:** `unit/verify` [`TestRFC6793EncodeTwoOctetToOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L65) |
| `RFC6793-4.1-5` | A BGP speaker that advertises such a capability to a particular peer, and receives from that peer the advertisement of such a capability, MUST encode AS numbers as four-octet entities in both the AS_PATH attribute and the AGGREGATOR attribute in the updates it sends to the peer and MUST assume that these attributes in the updates received from the peer encode AS numbers as four-octet entities. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC6793DecodeFourOctetWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L113). **negative:** `unit/verify` [`TestRFC6793DecodeTwoOctetWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L137) |
| `RFC6793-4.1-6` | The new attributes, AS4_PATH and AS4_AGGREGATOR, MUST NOT be carried in an UPDATE message between NEW BGP speakers. (Section 4.1, Section 6) | MUST NOT | 4.1 | **positive:** `unit/verify` [`TestOriginatedAggregatorOmitsAS4AggregatorTowardNewSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4agg_new_test.go#L12). **positive:** `unit/verify` [`TestOriginatedUpdateOmitsAS4PathTowardNewSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L290). **positive:** `functional/verify` [`rfc6793-no-as4aggregator-from-new-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-no-as4aggregator-from-new-speaker.ci#L12). **positive:** `functional/verify` [`rfc6793-no-as4path-from-new-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-no-as4path-from-new-speaker.ci#L29). **negative:** `functional/verify` [`rfc6793-narrow-to-old-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-narrow-to-old-speaker.ci#L25) |
| `RFC6793-4.1-7` | A NEW BGP speaker that receives the AS4_PATH attribute or the AS4_AGGREGATOR attribute in an UPDATE message from another NEW BGP speaker MUST discard the path attribute and continue processing the UPDATE message. (Section 4.1, Section 6) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC6793NewSpeakerAS4AttributesAreDiscarded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L532). **negative:** `unit/verify` [`TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L206). **positive:** `functional/verify` [`rfc6793-no-as4path-from-new-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-no-as4path-from-new-speaker.ci#L34) |
| `RFC6793-4.2.1-1` | However, this document does not assume that an Autonomous System with NEW BGP speakers has to have a globally unique two-octet AS number -- AS_TRANS MUST be used when the NEW BGP speaker does not have a two-octet AS number (even if multiple Autonomous Systems would use it). (Section 4.2.1) | MUST | 4.2.1 | **positive:** `unit/verify` [`TestRFC6793OpenMyASIsASTransWithoutTwoOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L88). **negative:** `unit/verify` [`TestRFC6793OpenMyASIsRealASWhenMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L104) |
| `RFC6793-4.2.2-1` | When communicating with an OLD BGP speaker, a NEW BGP speaker MUST send the AS path information in the AS_PATH attribute encoded with two-octet AS numbers. (Section 4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestRFC6793EncodeTwoOctetToOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L68). **negative:** `unit/verify` [`TestRFC6793TwoOctetASPathKeepsMappableASNs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L93) |
| `RFC6793-4.2.2-2` | The NEW BGP speaker MUST also send the AS path information in the AS4_PATH attribute (encoded with four-octet AS numbers), except for the case where all of the AS path information is composed of mappable four-octet AS numbers only. (Section 4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestOriginatedUpdateCarriesAS4PathForANonMappableLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L243). **positive:** `unit/verify` [`TestOriginatedUpdateCarriesAS4PathTowardOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L205). **positive:** `unit/verify` [`TestRFC6793TranscodeEmitsAS4PathForNonMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L52). **negative:** `unit/verify` [`TestOriginatedUpdateOmitsAS4PathWhenEveryASIsMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L272). **negative:** `unit/verify` [`TestRFC6793TranscodeOmitsAS4PathWhenAllMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L87) |
| `RFC6793-4.2.2-3` | In this case, the NEW BGP speaker MUST NOT send the AS4_PATH attribute. (Section 4.2.2) | MUST NOT | 4.2.2 | **positive:** `unit/verify` [`TestOriginatedUpdateOmitsAS4PathWhenEveryASIsMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L270). **positive:** `unit/verify` [`TestRFC6793TranscodeOmitsAS4PathWhenAllMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L85). **negative:** `unit/verify` [`TestOriginatedUpdateCarriesAS4PathTowardOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L208). **negative:** `unit/verify` [`TestRFC6793TranscodeEmitsAS4PathForNonMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L55) |
| `RFC6793-4.2.2-4` | Whenever the AS path information contains the AS_CONFED_SEQUENCE or AS_CONFED_SET path segment, the NEW BGP speaker MUST exclude such path segments from the AS4_PATH attribute being constructed. (Section 4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestRFC6793ConstructedAS4PathExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L302). **positive:** `unit/verify` [`TestRFC6793ConstructedAS4PathExcludesConfedSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_confed_set_test.go#L38). **negative:** `unit/verify` [`TestRFC6793ConstructedAS4PathExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L304). **negative:** `unit/verify` [`TestRFC6793ConstructedAS4PathKeepsAnOrdinarySet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_confed_set_test.go#L57) |
| `RFC6793-3-1` | To prevent the possible propagation of Confederation-related path segments outside of a Confederation, the path segment types AS_CONFED_SEQUENCE and AS_CONFED_SET [RFC5065] are declared invalid for the AS4_PATH attribute and MUST NOT be included in the AS4_PATH attribute of an UPDATE message. (Section 3, Section 6) | MUST NOT | 3 | **positive:** `unit/verify` [`TestRFC6793AS4PathWireExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L248). **negative:** `unit/verify` [`TestRFC6793AS4PathWireExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L251) |
| `RFC6793-4.2.2-5` | Similarly, if the NEW BGP speaker has to send the AGGREGATOR attribute, and if the aggregating Autonomous System's AS number is a non-mappable four-octet AS number, then the speaker MUST use the AS4_AGGREGATOR attribute and set the AS number field in the existing AGGREGATOR attribute to the reserved AS number, AS_TRANS. (Section 4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestForwardedAggregatorIsDowngradedWithItsCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L70). **positive:** `unit/verify` [`TestOriginatedAggregatorCarriesItsCompanionTowardOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L415). **positive:** `unit/verify` [`TestRFC6793AS4AggregatorForNonMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L234). **negative:** `unit/verify` [`TestAMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L107). **negative:** `unit/verify` [`TestOriginatedMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L445). **negative:** `unit/verify` [`TestRFC6793NoAS4AggregatorForMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L271) |
| `RFC6793-4.2.2-6` | Note that if the AS number is mappable, then the AS4_AGGREGATOR attribute MUST NOT be sent. (Section 4.2.2) | MUST NOT | 4.2.2 | **positive:** `unit/verify` [`TestAMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L105). **positive:** `unit/verify` [`TestOriginatedMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L443). **positive:** `unit/verify` [`TestRFC6793NoAS4AggregatorForMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L269). **negative:** `unit/verify` [`TestForwardedAggregatorIsDowngradedWithItsCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L73). **negative:** `unit/verify` [`TestOriginatedAggregatorCarriesItsCompanionTowardOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L418). **negative:** `unit/verify` [`TestRFC6793AS4AggregatorForNonMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L237) |
| `RFC6793-4.2.3-1` | When a NEW BGP speaker receives an update from an OLD BGP speaker, it MUST be prepared to receive the AS4_PATH attribute along with the existing AS_PATH attribute. (Section 4.2.3) | MUST | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L203). **negative:** `unit/verify` [`TestRFC6793ASPathAloneNotInventedIntoFourOctet`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L223) |
| `RFC6793-4.2.3-2` | A NEW BGP speaker MUST also be prepared to receive the AS4_AGGREGATOR attribute along with the AGGREGATOR attribute from an OLD BGP speaker. (Section 4.2.3) | MUST | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793ReceivedAS4AggregatorAccepted`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L238). **negative:** no negative test. **{single-polarity}:** the obligation is to accept the pair, and ParseAttributes has no rejection path to drive negatively: AGGREGATOR is interned and AS4_AGGREGATOR falls through the default branch into OtherAttrs, so every AGGREGATOR plus AS4_AGGREGATOR combination is accepted (internal/component/bgp/plugins/rib/storage/attrparse.go:96-102, :138-140). What ze does with the pair afterwards is governed by RFC6793-4.2.3-3 through -7, which are recorded as gaps |
| `RFC6793-4.2.3-3` | - the AS4_AGGREGATOR attribute and the AS4_PATH attribute SHALL be ignored, (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L338). **negative:** `unit/verify` [`TestRFC6793AggregatorOfTheWrongWidthIsNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L484). **negative:** `unit/verify` [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L384) |
| `RFC6793-4.2.3-4` | When both of the attributes are received, if the AS number in the AGGREGATOR attribute is not AS_TRANS, then: - the AS4_AGGREGATOR attribute and the AS4_PATH attribute SHALL be ignored, - the AGGREGATOR attribute SHALL be taken as the information about the aggregating node (§4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L342). **negative:** `unit/verify` [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L388) |
| `RFC6793-4.2.3-5` | When both of the attributes are received, if the AS number in the AGGREGATOR attribute is not AS_TRANS, then: - the AS4_AGGREGATOR attribute and the AS4_PATH attribute SHALL be ignored, - the AGGREGATOR attribute SHALL be taken as the information about the aggregating node, and - the AS_PATH attribute SHALL be taken as the AS path information. (§4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L346). **negative:** `unit/verify` [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L391) |
| `RFC6793-4.2.3-6` | - the AGGREGATOR attribute SHALL be ignored, (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L377). **negative:** `unit/verify` [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L350) |
| `RFC6793-4.2.3-7` | - the AS4_AGGREGATOR attribute SHALL be taken as the information about the aggregating node, and (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L380). **negative:** `unit/verify` [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L353). **negative:** `unit/verify` [`TestRFC6793LoneAS4AggregatorIsDropped`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L512) |
| `RFC6793-4.2.3-8` | If the number of AS numbers in the AS_PATH attribute is less than the number of AS numbers in the AS4_PATH attribute, then the AS4_PATH attribute SHALL be ignored, and the AS_PATH attribute SHALL be taken as the AS path information. (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793LongerAS4PathIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L278). **negative:** `unit/verify` [`TestRFC6793LongerASPathPrependsLeadingHops`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L261) |
| `RFC6793-4.2.3-9` | If the number of AS numbers in the AS_PATH attribute is larger than or equal to the number of AS numbers in the AS4_PATH attribute, then the AS path information SHALL be constructed by taking as many AS numbers and path segments as necessary from the leading part of the AS_PATH attribute, and then prepending them to the AS4_PATH attribute so that the AS path information has a number of AS numbers identical to that of the AS_PATH attribute. (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793EqualCountsPrependNothing`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L419). **positive:** `unit/verify` [`TestRFC6793LeadingASSetIsTakenWhole`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L439). **positive:** `unit/verify` [`TestRFC6793LongerASPathPrependsLeadingHops`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L256). **negative:** `unit/verify` [`TestRFC6793LongerAS4PathIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L283) |
| `RFC6793-4.2.3-10` | Note that a valid AS_CONFED_SEQUENCE or AS_CONFED_SET path segment SHALL be prepended if it is either the leading path segment or is adjacent to a path segment that is prepended. (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793ConfedSegmentAdjacentToPrependedIsPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_confed_adjacent_test.go#L25). **positive:** `unit/verify` [`TestRFC6793LeadingConfedSegmentIsPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L300). **negative:** `unit/verify` [`TestRFC6793ConfedSegmentAdjacentToPrependedIsPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_confed_adjacent_test.go#L26). **negative:** `unit/verify` [`TestRFC6793UnadjacentConfedSegmentIsNotPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L320) |
| `RFC6793-6-1` | The AS4_PATH attribute in an UPDATE message SHALL be considered malformed under the following conditions: - the attribute length is not a multiple of two or is too small (i.e., less than 6) for the attribute to carry at least one AS number, or - the path segment length in the attribute is either zero or is inconsistent with the attribute length, or - the path segment type in the attribute is not one of the types defined: AS_SEQUENCE, AS_SET, AS_CONFED_SEQUENCE, and AS_CONFED_SET. (§6) | SHALL | 6 | **positive:** `unit/verify` [`TestRFC6793AS4PathWellFormedAccepted`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L164). **positive:** `unit/verify` [`TestRFC6793MalformedAS4PathIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L460). **negative:** `unit/verify` [`TestRFC6793AS4PathMalformedRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L188) |
| `RFC6793-6-2` | The AS4_AGGREGATOR attribute in an UPDATE message SHALL be considered malformed if the attribute length is not 8. (Section 6) | SHALL | 6 | **positive:** `unit/verify` [`TestRFC6793AS4AggregatorLengthEight`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L217). **negative:** `unit/verify` [`TestRFC6793AS4AggregatorLengthEight`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L219) |
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

| Requirement | State | Reason |
|---|---|---|
| [`RFC6793-4.1-3`](#rfc6793-4.1-3) When a NEW BGP speaker processes an OPEN message from another NEW BGP speaker, it MUST use the AS number encoded in the Capability Value field of the "support for four-octet AS number capability" in lieu of the "My Autonomous System" field of the OPEN message. (Section 4.1) | {gap}, no test | UnpackOpen never populates Open.ASN4 from the code-65 capability -- it sets only Version, MyAS, HoldTime and BGPIdentifier (internal/component/bgp/message/open.go:171-180) -- so the reactor's only OPEN-derived peer AS falls back to the two-octet header field: resolveDynamicPeerSettings reads the always-zero open.ASN4 and keeps uint32(open.MyAS), i.e. AS_TRANS for a non-mappable peer (internal/component/bgp/reactor/reactor_dynamic.go:311-316), and negotiateWith passes that same zero as the peer ASN into Negotiate (internal/component/bgp/reactor/session_negotiate.go:27-32). The route-server plugin does read the capability value for its event view (internal/component/bgp/plugins/rs/server.go:647-649), but that is a reporting path, not the session's peer AS |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6793-4.1-1`](#rfc6793-4.1-1)

A BGP speaker that supports four-octet AS numbers SHALL advertise this to its peers using BGP Capabilities Advertisements (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793OpenOmitsCapabilityWhenDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L75) | unit/verify | unproven |
| positive | [`TestRFC6793OpenAdvertisesFourOctetASCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L60) | unit/verify | unproven |

### [`RFC6793-4.1-2`](#rfc6793-4.1-2)

The AS number of the BGP speaker MUST be carried in the Capability Value field of the "support for four-octet AS number capability" (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793ASN4CapabilityValueMustBeFourOctets`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc6793_asn4_test.go#L46) | unit/verify | unproven |
| positive | [`TestRFC6793ASN4CapabilityCarriesSpeakerAS`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc6793_asn4_test.go#L18) | unit/verify | unproven |

### [`RFC6793-4.1-3`](#rfc6793-4.1-3)

When a NEW BGP speaker processes an OPEN message from another NEW BGP speaker, it MUST use the AS number encoded in the Capability Value field of the "support for four-octet AS number capability" in lieu of the "My Autonomous System" field of the OPEN message. (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6793-4.1-3, so no unit is bound to it.

### [`RFC6793-4.1-4`](#rfc6793-4.1-4)

A BGP speaker that advertises such a capability to a particular peer, and receives from that peer the advertisement of such a capability, MUST encode AS numbers as four-octet entities in both the AS_PATH attribute and the AGGREGATOR attribute in the updates it sends to the peer (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793EncodeFourOctetToNewSpeaker drives ASPath/Aggregator WriteToWithContext with an ASN4 context and asserts 4-byte AS_PATH stride and 8-octet AGGREGATOR; the negative shows the 2-octet form without the capability.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793EncodeTwoOctetToOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L65) | unit/verify | unproven |
| positive | [`TestRFC6793EncodeFourOctetToNewSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L33) | unit/verify | unproven |

### [`RFC6793-4.1-5`](#rfc6793-4.1-5)

A BGP speaker that advertises such a capability to a particular peer, and receives from that peer the advertisement of such a capability, MUST encode AS numbers as four-octet entities in both the AS_PATH attribute and the AGGREGATOR attribute in the updates it sends to the peer and MUST assume that these attributes in the updates received from the peer encode AS numbers as four-octet entities. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793DecodeFourOctetWhenNegotiated reads ParseASPath/ParseAggregator with fourByte=true and recovers 4200000001; the negative shows the same bytes decode on a 2-octet stride and an 8-octet AGGREGATOR is refused without the capability.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793DecodeTwoOctetWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L137) | unit/verify | unproven |
| positive | [`TestRFC6793DecodeFourOctetWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L113) | unit/verify | unproven |

### [`RFC6793-4.1-6`](#rfc6793-4.1-6)

The new attributes, AS4_PATH and AS4_AGGREGATOR, MUST NOT be carried in an UPDATE message between NEW BGP speakers. (Section 4.1, Section 6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both attributes now covered. AS4_AGGREGATOR: rfc6793-no-as4aggregator-from-new-speaker.ci feeds an AS4_AGGREGATOR (AS 4200000000, differing from AGGREGATOR 4200000123) from a NEW speaker and asserts conn=2 exact hex: AGGREGATOR unchanged, no type 18; revert record on CollapseAS4Family. TestOriginatedAggregatorOmitsAS4AggregatorTowardNewSpeaker: every aggregator builder toward NEW with a non-mappable AS emits no AS4_AGGREGATOR and AGGREGATOR carries the real AS; revert record on AS4AggregatorFor. AS4_PATH half as before (originate builders, no-as4path .ci, records). Negative rfc6793-narrow-to-old-speaker.ci: toward an OLD speaker AS4_PATH is carried; revert record on AS4PathForRewrite.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`rfc6793-narrow-to-old-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-narrow-to-old-speaker.ci#L25) | functional/verify | revert, verified |
| positive | [`TestOriginatedUpdateOmitsAS4PathTowardNewSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L290) | unit/verify | revert, verified |
| positive | [`TestOriginatedAggregatorOmitsAS4AggregatorTowardNewSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4agg_new_test.go#L12) | unit/verify | revert, verified |
| positive | [`rfc6793-no-as4aggregator-from-new-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-no-as4aggregator-from-new-speaker.ci#L12) | functional/verify | revert, verified |
| positive | [`rfc6793-no-as4path-from-new-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-no-as4path-from-new-speaker.ci#L29) | functional/verify | revert, verified |

### [`RFC6793-4.1-7`](#rfc6793-4.1-7)

A NEW BGP speaker that receives the AS4_PATH attribute or the AS4_AGGREGATOR attribute in an UPDATE message from another NEW BGP speaker MUST discard the path attribute and continue processing the UPDATE message. (Section 4.1, Section 6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793NewSpeakerAS4AttributesAreDiscarded (ReconcileASPathFamily, fromNew=true) discards both AS4_PATH and AS4_AGGREGATOR and keeps the UPDATE; the .ci proves it end to end for AS4_PATH; the negative keeps an AS4_PATH from an OLD speaker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L206) | unit/verify | revert, verified |
| positive | [`TestRFC6793NewSpeakerAS4AttributesAreDiscarded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L532) | unit/verify | revert, verified |
| positive | [`rfc6793-no-as4path-from-new-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-no-as4path-from-new-speaker.ci#L34) | functional/verify | revert, verified |

### [`RFC6793-4.2.1-1`](#rfc6793-4.2.1-1)

However, this document does not assume that an Autonomous System with NEW BGP speakers has to have a globally unique two-octet AS number -- AS_TRANS MUST be used when the NEW BGP speaker does not have a two-octet AS number (even if multiple Autonomous Systems would use it). (Section 4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793OpenMyASIsASTransWithoutTwoOctetAS asserts the sent OPEN My AS is 23456 for AS 4200000001; the negative keeps 65001 as itself.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793OpenMyASIsRealASWhenMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L104) | unit/verify | unproven |
| positive | [`TestRFC6793OpenMyASIsASTransWithoutTwoOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L88) | unit/verify | unproven |

### [`RFC6793-4.2.2-1`](#rfc6793-4.2.2-1)

When communicating with an OLD BGP speaker, a NEW BGP speaker MUST send the AS path information in the AS_PATH attribute encoded with two-octet AS numbers. (Section 4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793EncodeTwoOctetToOldSpeaker asserts a 2-octet AS_PATH with AS_TRANS toward a non-ASN4 context; the negative keeps mappable ASNs unsubstituted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793TwoOctetASPathKeepsMappableASNs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L93) | unit/verify | unproven |
| positive | [`TestRFC6793EncodeTwoOctetToOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L68) | unit/verify | unproven |

### [`RFC6793-4.2.2-2`](#rfc6793-4.2.2-2)

The NEW BGP speaker MUST also send the AS path information in the AS4_PATH attribute (encoded with four-octet AS numbers), except for the case where all of the AS path information is composed of mappable four-octet AS numbers only. (Section 4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Originate builders (message) and TranscodeASPath (wireu) both emit AS4_PATH with the real 4-octet AS toward an OLD speaker; negatives show no AS4_PATH when every AS is mappable.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOriginatedUpdateOmitsAS4PathWhenEveryASIsMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L272) | unit/verify | revert, verified |
| negative | [`TestRFC6793TranscodeOmitsAS4PathWhenAllMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L87) | unit/verify | unproven |
| positive | [`TestOriginatedUpdateCarriesAS4PathForANonMappableLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L243) | unit/verify | revert, verified |
| positive | [`TestOriginatedUpdateCarriesAS4PathTowardOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L205) | unit/verify | revert, verified |
| positive | [`TestRFC6793TranscodeEmitsAS4PathForNonMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L52) | unit/verify | unproven |

### [`RFC6793-4.2.2-3`](#rfc6793-4.2.2-3)

In this case, the NEW BGP speaker MUST NOT send the AS4_PATH attribute. (Section 4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Originate builders and TranscodeASPath omit AS4_PATH when all ASNs are mappable; negatives show it present for a non-mappable AS.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOriginatedUpdateCarriesAS4PathTowardOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L208) | unit/verify | revert, verified |
| negative | [`TestRFC6793TranscodeEmitsAS4PathForNonMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L55) | unit/verify | unproven |
| positive | [`TestOriginatedUpdateOmitsAS4PathWhenEveryASIsMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L270) | unit/verify | revert, verified |
| positive | [`TestRFC6793TranscodeOmitsAS4PathWhenAllMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L85) | unit/verify | unproven |

### [`RFC6793-4.2.2-4`](#rfc6793-4.2.2-4)

Whenever the AS path information contains the AS_CONFED_SEQUENCE or AS_CONFED_SET path segment, the NEW BGP speaker MUST exclude such path segments from the AS4_PATH attribute being constructed. (Section 4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both confederation types now proven excluded from the constructed AS4_PATH. TestRFC6793ConstructedAS4PathExcludesConfedSet: AS_CONFED_SET {64514,64515}, AS_CONFED_SEQUENCE [64512], AS_SEQUENCE [non-mappable, 65001] through TranscodeASPath 4->2 yields an AS4_PATH of exactly the one AS_SEQUENCE, AS_PATH keeping all three. The AS_CONFED_SEQUENCE half is also held by the HEAD TestRFC6793ConstructedAS4PathExcludesConfed (exact single-segment assertion, no record) and by the AS_CONFED_SEQUENCE inside the new recorded unit. Negative TestRFC6793ConstructedAS4PathKeepsAnOrdinarySet: an ordinary AS_SET in the same place is carried, so the exclusion is scoped to type 3/4. Judge break (overlay, attribute/as4.go AS4Path.Len and WriteTo excluding only AS_CONFED_SEQUENCE) turned the new positive red, the HEAD sequence-only unit and the new negative green. Revert records on TranscodeASPath both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793ConstructedAS4PathExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L304) | unit/verify | unproven |
| negative | [`TestRFC6793ConstructedAS4PathKeepsAnOrdinarySet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_confed_set_test.go#L57) | unit/verify | revert, verified |
| positive | [`TestRFC6793ConstructedAS4PathExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L302) | unit/verify | unproven |
| positive | [`TestRFC6793ConstructedAS4PathExcludesConfedSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_confed_set_test.go#L38) | unit/verify | revert, verified |

### [`RFC6793-3-1`](#rfc6793-3-1)

To prevent the possible propagation of Confederation-related path segments outside of a Confederation, the path segment types AS_CONFED_SEQUENCE and AS_CONFED_SET [RFC5065] are declared invalid for the AS4_PATH attribute and MUST NOT be included in the AS4_PATH attribute of an UPDATE message. (Section 3, Section 6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793AS4PathWireExcludesConfed: AS4Path.WriteTo/Len skip both confed types and keep AS_SEQUENCE/AS_SET, with Len matching bytes written.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793AS4PathWireExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L251) | unit/verify | unproven |
| positive | [`TestRFC6793AS4PathWireExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L248) | unit/verify | unproven |

### [`RFC6793-4.2.2-5`](#rfc6793-4.2.2-5)

Similarly, if the NEW BGP speaker has to send the AGGREGATOR attribute, and if the aggregating Autonomous System's AS number is a non-mappable four-octet AS number, then the speaker MUST use the AS4_AGGREGATOR attribute and set the AS number field in the existing AGGREGATOR attribute to the reserved AS number, AS_TRANS. (Section 4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Three producers (message originate, wireu TranscodeASPath, rib commit pack) each send AS_TRANS in a 6-octet AGGREGATOR plus AS4_AGGREGATOR with the real AS for a non-mappable aggregator; negatives keep a mappable AS with no companion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOriginatedMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L445) | unit/verify | revert, verified |
| negative | [`TestAMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L107) | unit/verify | unproven |
| negative | [`TestRFC6793NoAS4AggregatorForMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L271) | unit/verify | unproven |
| positive | [`TestOriginatedAggregatorCarriesItsCompanionTowardOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L415) | unit/verify | revert, verified |
| positive | [`TestForwardedAggregatorIsDowngradedWithItsCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L70) | unit/verify | unproven |
| positive | [`TestRFC6793AS4AggregatorForNonMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L234) | unit/verify | unproven |

### [`RFC6793-4.2.2-6`](#rfc6793-4.2.2-6)

Note that if the AS number is mappable, then the AS4_AGGREGATOR attribute MUST NOT be sent. (Section 4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Same three producers send no AS4_AGGREGATOR for a mappable aggregating AS; negatives show it present for a non-mappable one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOriginatedAggregatorCarriesItsCompanionTowardOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L418) | unit/verify | revert, verified |
| negative | [`TestForwardedAggregatorIsDowngradedWithItsCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L73) | unit/verify | unproven |
| negative | [`TestRFC6793AS4AggregatorForNonMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L237) | unit/verify | unproven |
| positive | [`TestOriginatedMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc6793_originate_as4_test.go#L443) | unit/verify | revert, verified |
| positive | [`TestAMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L105) | unit/verify | unproven |
| positive | [`TestRFC6793NoAS4AggregatorForMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L269) | unit/verify | unproven |

### [`RFC6793-4.2.3-1`](#rfc6793-4.2.3-1)

When a NEW BGP speaker receives an update from an OLD BGP speaker, it MUST be prepared to receive the AS4_PATH attribute along with the existing AS_PATH attribute. (Section 4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath: ReconcileASPathFamily accepts AS_PATH+AS4_PATH from an OLD speaker and uses the AS4_PATH ASNs; the negative shows no 4-octet AS invented without AS4_PATH.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793ASPathAloneNotInventedIntoFourOctet`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L223) | unit/verify | revert, verified |
| positive | [`TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L203) | unit/verify | revert, verified |

### [`RFC6793-4.2.3-2`](#rfc6793-4.2.3-2)

A NEW BGP speaker MUST also be prepared to receive the AS4_AGGREGATOR attribute along with the AGGREGATOR attribute from an OLD BGP speaker. (Section 4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793ReceivedAS4AggregatorAccepted: the AGGREGATOR+AS4_AGGREGATOR pair from an OLD speaker is accepted and the 4-octet aggregating AS survives ingest (single-polarity marker).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC6793ReceivedAS4AggregatorAccepted`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L238) | unit/verify | revert, verified |

### [`RFC6793-4.2.3-3`](#rfc6793-4.2.3-3)

- the AS4_AGGREGATOR attribute and the AS4_PATH attribute SHALL be ignored, (Section 4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793AggregatorWithRealASIgnoresAS4Attributes: with AGGREGATOR.AS 64500 the AS4_PATH does not reach the path and the AS4_AGGREGATOR is discarded; negatives with AS_TRANS use both.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793AggregatorOfTheWrongWidthIsNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L484) | unit/verify | revert, verified |
| negative | [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L384) | unit/verify | revert, verified |
| positive | [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L338) | unit/verify | revert, verified |

### [`RFC6793-4.2.3-4`](#rfc6793-4.2.3-4)

When both of the attributes are received, if the AS number in the AGGREGATOR attribute is not AS_TRANS, then: - the AS4_AGGREGATOR attribute and the AS4_PATH attribute SHALL be ignored, - the AGGREGATOR attribute SHALL be taken as the information about the aggregating node (§4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Same unit: the AGGREGATOR (64500, 10.0.0.1) is the aggregating node; the AS_TRANS counterpart does not take it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L388) | unit/verify | revert, verified |
| positive | [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L342) | unit/verify | revert, verified |

### [`RFC6793-4.2.3-5`](#rfc6793-4.2.3-5)

When both of the attributes are received, if the AS number in the AGGREGATOR attribute is not AS_TRANS, then: - the AS4_AGGREGATOR attribute and the AS4_PATH attribute SHALL be ignored, - the AGGREGATOR attribute SHALL be taken as the information about the aggregating node, and - the AS_PATH attribute SHALL be taken as the AS path information. (§4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Same unit: the route path is the AS_PATH widened with AS_TRANS intact; the AS_TRANS counterpart reconstructs instead.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L391) | unit/verify | revert, verified |
| positive | [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L346) | unit/verify | revert, verified |

### [`RFC6793-4.2.3-6`](#rfc6793-4.2.3-6)

- the AGGREGATOR attribute SHALL be ignored, (Section 4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793AggregatorWithASTransPromotesAS4Aggregator: with AGGREGATOR.AS AS_TRANS the stored aggregator is the AS4_AGGREGATOR value, not AS_TRANS; negative keeps a real-AS AGGREGATOR.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L350) | unit/verify | revert, verified |
| positive | [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L377) | unit/verify | revert, verified |

### [`RFC6793-4.2.3-7`](#rfc6793-4.2.3-7)

- the AS4_AGGREGATOR attribute SHALL be taken as the information about the aggregating node, and (Section 4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Same unit: the aggregating node is 4200000001/10.0.0.1 from AS4_AGGREGATOR; negatives: not promoted when AGGREGATOR is real, nor when AS4_AGGREGATOR arrives alone.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L353) | unit/verify | revert, verified |
| negative | [`TestRFC6793LoneAS4AggregatorIsDropped`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L512) | unit/verify | revert, verified |
| positive | [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L380) | unit/verify | revert, verified |

### [`RFC6793-4.2.3-8`](#rfc6793-4.2.3-8)

If the number of AS numbers in the AS_PATH attribute is less than the number of AS numbers in the AS4_PATH attribute, then the AS4_PATH attribute SHALL be ignored, and the AS_PATH attribute SHALL be taken as the AS path information. (Section 4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793LongerAS4PathIsIgnored: AS_PATH 2 ASNs vs AS4_PATH 3 yields the widened AS_PATH only; negative shows AS4_PATH used when AS_PATH is longer.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793LongerASPathPrependsLeadingHops`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L261) | unit/verify | revert, verified |
| positive | [`TestRFC6793LongerAS4PathIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L278) | unit/verify | revert, verified |

### [`RFC6793-4.2.3-9`](#rfc6793-4.2.3-9)

If the number of AS numbers in the AS_PATH attribute is larger than or equal to the number of AS numbers in the AS4_PATH attribute, then the AS path information SHALL be constructed by taking as many AS numbers and path segments as necessary from the leading part of the AS_PATH attribute, and then prepending them to the AS4_PATH attribute so that the AS path information has a number of AS numbers identical to that of the AS_PATH attribute. (Section 4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793LongerASPathPrependsLeadingHops, EqualCountsPrependNothing, LeadingASSetIsTakenWhole assert exact reconstructed bytes; negative: no prepend when AS4_PATH is longer.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793LongerAS4PathIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L283) | unit/verify | revert, verified |
| positive | [`TestRFC6793EqualCountsPrependNothing`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L419) | unit/verify | revert, verified |
| positive | [`TestRFC6793LeadingASSetIsTakenWhole`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L439) | unit/verify | revert, verified |
| positive | [`TestRFC6793LongerASPathPrependsLeadingHops`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L256) | unit/verify | revert, verified |

### [`RFC6793-4.2.3-10`](#rfc6793-4.2.3-10)

Note that a valid AS_CONFED_SEQUENCE or AS_CONFED_SET path segment SHALL be prepended if it is either the leading path segment or is adjacent to a path segment that is prepended. (Section 4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. TestRFC6793ConfedSegmentAdjacentToPrependedIsPrepended drives ReconcileASPathFamily: + AS_SEQ 64500, AS_CONFED_SEQ 65001, AS_SEQ TRANS TRANS beside a two-hop AS4_PATH keeps the confed segment after the prepended 64500; - the same confed segment behind a non-prepended AS_SEQ is dropped. Leading-segment clause is TestRFC6793LeadingConfedSegmentIsPrepended. Judge breaks in appendLeadingSegments: continuing past a non-prepended segment turned the negative red; stopping before a confed segment once the budget is spent turned the positive red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793ConfedSegmentAdjacentToPrependedIsPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_confed_adjacent_test.go#L26) | unit/verify | revert, verified |
| negative | [`TestRFC6793UnadjacentConfedSegmentIsNotPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L320) | unit/verify | revert, verified |
| positive | [`TestRFC6793ConfedSegmentAdjacentToPrependedIsPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_confed_adjacent_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestRFC6793LeadingConfedSegmentIsPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L300) | unit/verify | revert, verified |

### [`RFC6793-6-1`](#rfc6793-6-1)

The AS4_PATH attribute in an UPDATE message SHALL be considered malformed under the following conditions: - the attribute length is not a multiple of two or is too small (i.e., less than 6) for the attribute to carry at least one AS number, or - the path segment length in the attribute is either zero or is inconsistent with the attribute length, or - the path segment type in the attribute is not one of the types defined: AS_SEQUENCE, AS_SET, AS_CONFED_SEQUENCE, and AS_CONFED_SET. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793AS4PathMalformedRejected drives every Section 6 condition (odd length, <6 octets, zero and overrunning segment length, undefined type) through ParseAS4Path; positive accepts a well-formed path. The tag at rfc6793_reconcile_test.go TestRFC6793MalformedAS4PathIsDiscarded describes the RFC6793-6-4 discard obligation rather than this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793AS4PathMalformedRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L188) | unit/verify | unproven |
| positive | [`TestRFC6793AS4PathWellFormedAccepted`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L164) | unit/verify | unproven |
| positive | [`TestRFC6793MalformedAS4PathIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L460) | unit/verify | revert, verified |

### [`RFC6793-6-2`](#rfc6793-6-2)

The AS4_AGGREGATOR attribute in an UPDATE message SHALL be considered malformed if the attribute length is not 8. (Section 6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793AS4AggregatorLengthEight: ParseAS4Aggregator accepts 8 octets and refuses 0,6,7,9,12 with ErrInvalidLength.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793AS4AggregatorLengthEight`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L219) | unit/verify | unproven |
| positive | [`TestRFC6793AS4AggregatorLengthEight`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L217) | unit/verify | unproven |

### [`RFC6793-6-3`](#rfc6793-6-3)

A NEW BGP speaker that receives these path segment types in the AS4_PATH attribute of an UPDATE message from an OLD BGP speaker MUST discard these path segments, adjust the relevant attribute fields accordingly, and continue processing the UPDATE message. (Section 6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c34 judge). internal/component/bgp/wireu/rfc6793_received_confed_test.go reads the output of CollapseAS4Family, the single ingest rewrite every downstream consumer sees, so the receive-side discard is measured apart from the AS4_PATH encoder's own confed filter (RFC6793-3-1). Positive: an OLD speaker's AS4_PATH holding an AS_CONFED_SEQUENCE and an AS_CONFED_SET ahead of an AS_SEQUENCE collapses to the exact payload (AS_PATH 40 02 10: [65001] then [4200000001,4200000002], attribute section 0x1E, NLRI kept, no error): both segment types discarded, lengths adjusted, processing continues (judge checked the RFC 6793 Section 4.2.3 arithmetic and both length octets). Negative: an AS_SET in the AS4_PATH and a leading AS_CONFED_SEQUENCE in the AS_PATH both survive (payload literal, 0x1C / 0x2A), so the discard is not blanket. Records: attribute/as4.go::appendAS4Segments revert, both polarities observed red; author overlay probes (kept confed segments -> only the positive red, discard widened to AS_SET -> only the negative red) re-read by the judge in scratch c34p. The HEAD egress units in rfc6793_as4_test.go stay as unrecorded supplementary tags. The Section 6 SHOULD-log clause is not this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793ReceivedConfedInAS4PathDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L409) | unit/verify | unproven |
| negative | [`TestRFC6793ReceivedAS4PathDiscardTakesOnlyConfedSegments`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_received_confed_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestRFC6793ReceivedConfedInAS4PathDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L406) | unit/verify | unproven |
| positive | [`TestRFC6793ReceivedAS4PathConfedSegmentsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_received_confed_test.go#L44) | unit/verify | revert, verified |

### [`RFC6793-6-4`](#rfc6793-6-4)

A NEW BGP speaker that receives a malformed AS4_PATH attribute in an UPDATE message from an OLD BGP speaker MUST discard the attribute and continue processing the UPDATE message. (Section 6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC6793MalformedAS4PathDiscarded: an odd-length AS4_PATH from an OLD speaker contributes nothing and the rewrite proceeds; negative keeps a well-formed one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793WellFormedAS4PathNotDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L377) | unit/verify | unproven |
| positive | [`TestRFC6793MalformedAS4PathDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L343) | unit/verify | unproven |

### [`RFC6793-6-5`](#rfc6793-6-5)

A NEW BGP speaker that receives a malformed AS4_AGGREGATOR attribute in an UPDATE message from an OLD BGP speaker MUST discard the attribute and continue processing the UPDATE message. (Section 6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestCollapseAS4DiscardsMalformedAS4Aggregator: a 7-octet AS4_AGGREGATOR is removed from the collapsed payload, not promoted, NLRI kept, drop reported; negative reads an 8-octet one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCollapseAS4SelectsAggregatorPerSection423`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_aspath_collapse_test.go#L350) | unit/verify | revert, verified |
| positive | [`TestCollapseAS4DiscardsMalformedAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_aspath_collapse_test.go#L482) | unit/verify | revert, verified |

## Extraction sign-off

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
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 6 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.2.1` | not stated | 1 | walked | not stated |
| `4.2.2` | not stated | 6 | walked | not stated |
| `4.2.3` | not stated | 10 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 8 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `12.1` | not stated | 0 | walked | not stated |
| `12.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `6:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6 restates the Section 4.1 prohibition word for word; site 4.1:5 carries the same sentence and maps RFC6793-4.1-6, whose checklist row cites both sections. | The AS4_PATH attribute and AS4_AGGREGATOR attribute MUST NOT be carried in an UPDATE message between NEW BGP speakers. |
| `6:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6 restates the Section 4.1 discard rule word for word; site 4.1:6 carries the same sentence and maps RFC6793-4.1-7, whose checklist row cites both sections. | A NEW BGP speaker that receives the AS4_PATH attribute or the AS4_AGGREGATOR attribute in an UPDATE message from another NEW BGP speaker MUST discard the path attribute and continue processing the UPDATE message. |
| `6:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6 restates the Section 3 prohibition on AS_CONFED_SEQUENCE and AS_CONFED_SET inside AS4_PATH; site 3:1 maps RFC6793-3-1, whose checklist row cites both sections. | In addition, the path segment types AS_CONFED_SEQUENCE and AS_CONFED_SET [RFC5065] MUST NOT be carried in the AS4_PATH attribute of an UPDATE message. |

## Superseded

No document obsoletes RFC 6793, so its obligations are stated where they were written.
