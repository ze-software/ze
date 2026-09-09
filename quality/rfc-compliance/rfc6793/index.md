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
| Proven by a recorded break | 6.0% | 4 of 67 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

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

The 7 shares marked as a part above are the whole of the 30 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 36 |
| Gated MUST-level | 30 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 67 |
| Tagged units | 67 |
| Recorded audit verdicts | 0 |
| Discrimination records | 4 |
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
| Annotated instead of tested | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **30** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (28):** [`RFC6793-4.1-1`](#rfc6793-4.1-1), [`RFC6793-4.1-2`](#rfc6793-4.1-2), [`RFC6793-4.1-4`](#rfc6793-4.1-4), [`RFC6793-4.1-5`](#rfc6793-4.1-5), [`RFC6793-4.1-6`](#rfc6793-4.1-6), [`RFC6793-4.1-7`](#rfc6793-4.1-7), [`RFC6793-4.2.1-1`](#rfc6793-4.2.1-1), [`RFC6793-4.2.2-1`](#rfc6793-4.2.2-1), [`RFC6793-4.2.2-2`](#rfc6793-4.2.2-2), [`RFC6793-4.2.2-3`](#rfc6793-4.2.2-3), [`RFC6793-4.2.2-4`](#rfc6793-4.2.2-4), [`RFC6793-3-1`](#rfc6793-3-1), [`RFC6793-4.2.2-5`](#rfc6793-4.2.2-5), [`RFC6793-4.2.2-6`](#rfc6793-4.2.2-6), [`RFC6793-4.2.3-1`](#rfc6793-4.2.3-1), [`RFC6793-4.2.3-3`](#rfc6793-4.2.3-3), [`RFC6793-4.2.3-4`](#rfc6793-4.2.3-4), [`RFC6793-4.2.3-5`](#rfc6793-4.2.3-5), [`RFC6793-4.2.3-6`](#rfc6793-4.2.3-6), [`RFC6793-4.2.3-7`](#rfc6793-4.2.3-7), [`RFC6793-4.2.3-8`](#rfc6793-4.2.3-8), [`RFC6793-4.2.3-9`](#rfc6793-4.2.3-9), [`RFC6793-4.2.3-10`](#rfc6793-4.2.3-10), [`RFC6793-6-1`](#rfc6793-6-1), [`RFC6793-6-2`](#rfc6793-6-2), [`RFC6793-6-3`](#rfc6793-6-3), [`RFC6793-6-4`](#rfc6793-6-4), [`RFC6793-6-5`](#rfc6793-6-5)

**Annotated instead of tested (2):** [`RFC6793-4.1-3`](#rfc6793-4.1-3), [`RFC6793-4.2.3-2`](#rfc6793-4.2.3-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6793-4.1-1` | A BGP speaker that supports four-octet AS numbers SHALL advertise this to its peers using BGP Capabilities Advertisements (Section 4.1) | SHALL | 4.1 | **positive:** `unit/verify` [`TestRFC6793OpenAdvertisesFourOctetASCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L60). **negative:** `unit/verify` [`TestRFC6793OpenOmitsCapabilityWhenDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L75) |
| `RFC6793-4.1-2` | The AS number of the BGP speaker MUST be carried in the Capability Value field of the "support for four-octet AS number capability" (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC6793ASN4CapabilityCarriesSpeakerAS`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc6793_asn4_test.go#L18). **negative:** `unit/verify` [`TestRFC6793ASN4CapabilityValueMustBeFourOctets`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc6793_asn4_test.go#L46) |
| `RFC6793-4.1-3` | When processing an OPEN from another NEW speaker, MUST use the AS number from the Capability Value field in lieu of the "My Autonomous System" field (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** UnpackOpen never populates Open.ASN4 from the code-65 capability -- it sets only Version, MyAS, HoldTime and BGPIdentifier (internal/component/bgp/message/open.go:171-180) -- so the reactor's only OPEN-derived peer AS falls back to the two-octet header field: resolveDynamicPeerSettings reads the always-zero open.ASN4 and keeps uint32(open.MyAS), i.e. AS_TRANS for a non-mappable peer (internal/component/bgp/reactor/reactor_dynamic.go:311-316), and negotiateWith passes that same zero as the peer ASN into Negotiate (internal/component/bgp/reactor/session_negotiate.go:27-32). The route-server plugin does read the capability value for its event view (internal/component/bgp/plugins/rs/server.go:647-649), but that is a reporting path, not the session's peer AS |
| `RFC6793-4.1-4` | When both peers support four-octet AS, MUST encode AS numbers as four-octet entities in both AS_PATH and AGGREGATOR attributes (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC6793EncodeFourOctetToNewSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L33). **negative:** `unit/verify` [`TestRFC6793EncodeTwoOctetToOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L65) |
| `RFC6793-4.1-5` | When both peers support four-octet AS, MUST assume received AS_PATH and AGGREGATOR encode AS numbers as four-octet entities (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC6793DecodeFourOctetWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L113). **negative:** `unit/verify` [`TestRFC6793DecodeTwoOctetWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L137) |
| `RFC6793-4.1-6` | AS4_PATH and AS4_AGGREGATOR MUST NOT be carried in an UPDATE between NEW BGP speakers (Section 4.1, Section 6) | MUST NOT | 4.1 | **positive:** `functional/verify` [`rfc6793-no-as4path-from-new-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-no-as4path-from-new-speaker.ci#L29). **negative:** `functional/verify` [`rfc6793-narrow-to-old-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-narrow-to-old-speaker.ci#L25) |
| `RFC6793-4.1-7` | A NEW speaker receiving AS4_PATH or AS4_AGGREGATOR from another NEW speaker MUST discard the path attribute and continue processing the UPDATE (Section 4.1, Section 6) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC6793NewSpeakerAS4AttributesAreDiscarded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L531). **negative:** `unit/verify` [`TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L205). **positive:** `functional/verify` [`rfc6793-no-as4path-from-new-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-no-as4path-from-new-speaker.ci#L34) |
| `RFC6793-4.2.1-1` | AS_TRANS MUST be used in the OPEN "My Autonomous System" field when the NEW speaker does not have a two-octet AS number (Section 4.2.1) | MUST | 4.2.1 | **positive:** `unit/verify` [`TestRFC6793OpenMyASIsASTransWithoutTwoOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L88). **negative:** `unit/verify` [`TestRFC6793OpenMyASIsRealASWhenMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L104) |
| `RFC6793-4.2.2-1` | When sending to an OLD speaker, MUST send AS path information in AS_PATH encoded with two-octet AS numbers (Section 4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestRFC6793EncodeTwoOctetToOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L68). **negative:** `unit/verify` [`TestRFC6793TwoOctetASPathKeepsMappableASNs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L93) |
| `RFC6793-4.2.2-2` | When sending to an OLD speaker with non-mappable ASes, MUST also send AS4_PATH encoded with four-octet AS numbers (Section 4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestRFC6793TranscodeEmitsAS4PathForNonMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L49). **negative:** `unit/verify` [`TestRFC6793TranscodeOmitsAS4PathWhenAllMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L84) |
| `RFC6793-4.2.2-3` | When sending to an OLD speaker and all ASes are mappable, MUST NOT send AS4_PATH (Section 4.2.2) | MUST NOT | 4.2.2 | **positive:** `unit/verify` [`TestRFC6793TranscodeOmitsAS4PathWhenAllMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L82). **negative:** `unit/verify` [`TestRFC6793TranscodeEmitsAS4PathForNonMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L52) |
| `RFC6793-4.2.2-4` | When constructing AS4_PATH, MUST exclude AS_CONFED_SEQUENCE and AS_CONFED_SET path segments (Section 4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestRFC6793ConstructedAS4PathExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L222). **negative:** `unit/verify` [`TestRFC6793ConstructedAS4PathExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L224) |
| `RFC6793-3-1` | AS_CONFED_SEQUENCE and AS_CONFED_SET MUST NOT be carried in the AS4_PATH attribute of an UPDATE message (Section 3, Section 6) | MUST NOT | 3 | **positive:** `unit/verify` [`TestRFC6793AS4PathWireExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L248). **negative:** `unit/verify` [`TestRFC6793AS4PathWireExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L251) |
| `RFC6793-4.2.2-5` | When aggregator AS is non-mappable, MUST use AS4_AGGREGATOR and set AGGREGATOR AS field to AS_TRANS (Section 4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestForwardedAggregatorIsDowngradedWithItsCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L70). **positive:** `unit/verify` [`TestRFC6793AS4AggregatorForNonMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L154). **negative:** `unit/verify` [`TestAMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L107). **negative:** `unit/verify` [`TestRFC6793NoAS4AggregatorForMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L191) |
| `RFC6793-4.2.2-6` | If aggregator AS is mappable, AS4_AGGREGATOR MUST NOT be sent (Section 4.2.2) | MUST NOT | 4.2.2 | **positive:** `unit/verify` [`TestAMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L105). **positive:** `unit/verify` [`TestRFC6793NoAS4AggregatorForMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L189). **negative:** `unit/verify` [`TestForwardedAggregatorIsDowngradedWithItsCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L73). **negative:** `unit/verify` [`TestRFC6793AS4AggregatorForNonMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L157) |
| `RFC6793-4.2.3-1` | When receiving from an OLD speaker, MUST be prepared to receive AS4_PATH along with AS_PATH (Section 4.2.3) | MUST | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L202). **negative:** `unit/verify` [`TestRFC6793ASPathAloneNotInventedIntoFourOctet`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L222) |
| `RFC6793-4.2.3-2` | MUST be prepared to receive AS4_AGGREGATOR along with AGGREGATOR from an OLD speaker (Section 4.2.3) | MUST | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793ReceivedAS4AggregatorAccepted`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L237). **negative:** no negative test. **{single-polarity}:** the obligation is to accept the pair, and ParseAttributes has no rejection path to drive negatively: AGGREGATOR is interned and AS4_AGGREGATOR falls through the default branch into OtherAttrs, so every AGGREGATOR plus AS4_AGGREGATOR combination is accepted (internal/component/bgp/plugins/rib/storage/attrparse.go:96-102, :138-140). What ze does with the pair afterwards is governed by RFC6793-4.2.3-3 through -7, which are recorded as gaps |
| `RFC6793-4.2.3-3` | When both AGGREGATOR and AS4_AGGREGATOR are received and AGGREGATOR.AS != AS_TRANS, AS4_AGGREGATOR and AS4_PATH SHALL be ignored (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L337). **negative:** `unit/verify` [`TestRFC6793AggregatorOfTheWrongWidthIsNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L483). **negative:** `unit/verify` [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L383) |
| `RFC6793-4.2.3-4` | When both AGGREGATOR and AS4_AGGREGATOR are received and AGGREGATOR.AS != AS_TRANS, AGGREGATOR SHALL be taken as the aggregator info (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L341). **negative:** `unit/verify` [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L387) |
| `RFC6793-4.2.3-5` | When both AGGREGATOR and AS4_AGGREGATOR are received and AGGREGATOR.AS != AS_TRANS, AS_PATH SHALL be taken as the AS path info (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L345). **negative:** `unit/verify` [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L390) |
| `RFC6793-4.2.3-6` | When AGGREGATOR.AS == AS_TRANS, AGGREGATOR SHALL be ignored (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L376). **negative:** `unit/verify` [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L349) |
| `RFC6793-4.2.3-7` | When AGGREGATOR.AS == AS_TRANS, AS4_AGGREGATOR SHALL be taken as the aggregator info (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L379). **negative:** `unit/verify` [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L352). **negative:** `unit/verify` [`TestRFC6793LoneAS4AggregatorIsDropped`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L511) |
| `RFC6793-4.2.3-8` | If AS_PATH AS count < AS4_PATH AS count, AS4_PATH SHALL be ignored and AS_PATH SHALL be taken as AS path info (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793LongerAS4PathIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L277). **negative:** `unit/verify` [`TestRFC6793LongerASPathPrependsLeadingHops`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L260) |
| `RFC6793-4.2.3-9` | If AS_PATH AS count >= AS4_PATH AS count, AS path info SHALL be constructed by prepending leading AS_PATH entries to AS4_PATH (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793EqualCountsPrependNothing`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L418). **positive:** `unit/verify` [`TestRFC6793LeadingASSetIsTakenWhole`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L438). **positive:** `unit/verify` [`TestRFC6793LongerASPathPrependsLeadingHops`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L255). **negative:** `unit/verify` [`TestRFC6793LongerAS4PathIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L282) |
| `RFC6793-4.2.3-10` | A valid AS_CONFED_SEQUENCE or AS_CONFED_SET path segment SHALL be prepended if it is the leading segment or adjacent to a prepended segment (Section 4.2.3) | SHALL | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793LeadingConfedSegmentIsPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L299). **negative:** `unit/verify` [`TestRFC6793UnadjacentConfedSegmentIsNotPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L319) |
| `RFC6793-6-1` | AS4_PATH in an UPDATE SHALL be considered malformed if attribute length is not a multiple of two, is too small, segment length is zero or inconsistent, or segment type is undefined (Section 6) | SHALL | 6 | **positive:** `unit/verify` [`TestRFC6793AS4PathWellFormedAccepted`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L164). **positive:** `unit/verify` [`TestRFC6793MalformedAS4PathIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L459). **negative:** `unit/verify` [`TestRFC6793AS4PathMalformedRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L188) |
| `RFC6793-6-2` | AS4_AGGREGATOR in an UPDATE SHALL be considered malformed if the attribute length is not 8 (Section 6) | SHALL | 6 | **positive:** `unit/verify` [`TestRFC6793AS4AggregatorLengthEight`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L217). **negative:** `unit/verify` [`TestRFC6793AS4AggregatorLengthEight`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L219) |
| `RFC6793-6-3` | On receiving AS_CONFED_* segments in AS4_PATH from an OLD speaker, MUST discard those segments, adjust fields, and continue processing (Section 6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC6793ReceivedConfedInAS4PathDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L328). **negative:** `unit/verify` [`TestRFC6793ReceivedConfedInAS4PathDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L331) |
| `RFC6793-6-4` | On receiving malformed AS4_PATH from an OLD speaker, MUST discard the attribute and continue processing the UPDATE (Section 6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC6793MalformedAS4PathDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L262). **negative:** `unit/verify` [`TestRFC6793WellFormedAS4PathNotDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L297) |
| `RFC6793-6-5` | On receiving malformed AS4_AGGREGATOR from an OLD speaker, MUST discard the attribute and continue processing the UPDATE (Section 6) | MUST | 6 | **positive:** `unit/verify` [`TestCollapseAS4DiscardsMalformedAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/aspath_collapse_test.go#L482). **negative:** `unit/verify` [`TestCollapseAS4SelectsAggregatorPerSection423`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/aspath_collapse_test.go#L350) |
| `RFC6793-6-6` | When AS4_PATH or AS4_AGGREGATOR is received from a NEW speaker, SHOULD log locally for analysis (Section 6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC6793-6-7` | When AS_CONFED_* segments are found in AS4_PATH, SHOULD log locally for analysis (Section 6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC6793-6-8` | When malformed AS4_PATH is received, the error SHOULD be logged locally for analysis (Section 6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC6793-6-9` | When malformed AS4_AGGREGATOR is received, the error SHOULD be logged locally for analysis (Section 6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC6793-5-1` | NEW speakers with non-mappable AS SHOULD use four-octet AS specific extended communities instead of standard communities (Section 5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6793-7-1` | BGP speakers within an AS MAY be upgraded to support four-octet AS extensions on a piecemeal basis (Section 7) | MAY | 7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC6793-4.1-3`](#rfc6793-4.1-3) When processing an OPEN from another NEW speaker, MUST use the AS number from the Capability Value field in lieu of the "My Autonomous System" field (Section 4.1) | {gap}, no test | UnpackOpen never populates Open.ASN4 from the code-65 capability -- it sets only Version, MyAS, HoldTime and BGPIdentifier (internal/component/bgp/message/open.go:171-180) -- so the reactor's only OPEN-derived peer AS falls back to the two-octet header field: resolveDynamicPeerSettings reads the always-zero open.ASN4 and keeps uint32(open.MyAS), i.e. AS_TRANS for a non-mappable peer (internal/component/bgp/reactor/reactor_dynamic.go:311-316), and negotiateWith passes that same zero as the peer ASN into Negotiate (internal/component/bgp/reactor/session_negotiate.go:27-32). The route-server plugin does read the capability value for its event view (internal/component/bgp/plugins/rs/server.go:647-649), but that is a reporting path, not the session's peer AS |

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

When processing an OPEN from another NEW speaker, MUST use the AS number from the Capability Value field in lieu of the "My Autonomous System" field (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6793-4.1-3, so no unit is bound to it.

### [`RFC6793-4.1-4`](#rfc6793-4.1-4)

When both peers support four-octet AS, MUST encode AS numbers as four-octet entities in both AS_PATH and AGGREGATOR attributes (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793EncodeTwoOctetToOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L65) | unit/verify | unproven |
| positive | [`TestRFC6793EncodeFourOctetToNewSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L33) | unit/verify | unproven |

### [`RFC6793-4.1-5`](#rfc6793-4.1-5)

When both peers support four-octet AS, MUST assume received AS_PATH and AGGREGATOR encode AS numbers as four-octet entities (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793DecodeTwoOctetWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L137) | unit/verify | unproven |
| positive | [`TestRFC6793DecodeFourOctetWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L113) | unit/verify | unproven |

### [`RFC6793-4.1-6`](#rfc6793-4.1-6)

AS4_PATH and AS4_AGGREGATOR MUST NOT be carried in an UPDATE between NEW BGP speakers (Section 4.1, Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`rfc6793-narrow-to-old-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-narrow-to-old-speaker.ci#L25) | functional/verify | revert, verified |
| positive | [`rfc6793-no-as4path-from-new-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-no-as4path-from-new-speaker.ci#L29) | functional/verify | revert, verified |

### [`RFC6793-4.1-7`](#rfc6793-4.1-7)

A NEW speaker receiving AS4_PATH or AS4_AGGREGATOR from another NEW speaker MUST discard the path attribute and continue processing the UPDATE (Section 4.1, Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L205) | unit/verify | unproven |
| positive | [`TestRFC6793NewSpeakerAS4AttributesAreDiscarded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L531) | unit/verify | unproven |
| positive | [`rfc6793-no-as4path-from-new-speaker.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc6793-no-as4path-from-new-speaker.ci#L34) | functional/verify | revert, verified |

### [`RFC6793-4.2.1-1`](#rfc6793-4.2.1-1)

AS_TRANS MUST be used in the OPEN "My Autonomous System" field when the NEW speaker does not have a two-octet AS number (Section 4.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793OpenMyASIsRealASWhenMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L104) | unit/verify | unproven |
| positive | [`TestRFC6793OpenMyASIsASTransWithoutTwoOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L88) | unit/verify | unproven |

### [`RFC6793-4.2.2-1`](#rfc6793-4.2.2-1)

When sending to an OLD speaker, MUST send AS path information in AS_PATH encoded with two-octet AS numbers (Section 4.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793TwoOctetASPathKeepsMappableASNs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L93) | unit/verify | unproven |
| positive | [`TestRFC6793EncodeTwoOctetToOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L68) | unit/verify | unproven |

### [`RFC6793-4.2.2-2`](#rfc6793-4.2.2-2)

When sending to an OLD speaker with non-mappable ASes, MUST also send AS4_PATH encoded with four-octet AS numbers (Section 4.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793TranscodeOmitsAS4PathWhenAllMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L84) | unit/verify | unproven |
| positive | [`TestRFC6793TranscodeEmitsAS4PathForNonMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L49) | unit/verify | unproven |

### [`RFC6793-4.2.2-3`](#rfc6793-4.2.2-3)

When sending to an OLD speaker and all ASes are mappable, MUST NOT send AS4_PATH (Section 4.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793TranscodeEmitsAS4PathForNonMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L52) | unit/verify | unproven |
| positive | [`TestRFC6793TranscodeOmitsAS4PathWhenAllMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L82) | unit/verify | unproven |

### [`RFC6793-4.2.2-4`](#rfc6793-4.2.2-4)

When constructing AS4_PATH, MUST exclude AS_CONFED_SEQUENCE and AS_CONFED_SET path segments (Section 4.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793ConstructedAS4PathExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L224) | unit/verify | unproven |
| positive | [`TestRFC6793ConstructedAS4PathExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L222) | unit/verify | unproven |

### [`RFC6793-3-1`](#rfc6793-3-1)

AS_CONFED_SEQUENCE and AS_CONFED_SET MUST NOT be carried in the AS4_PATH attribute of an UPDATE message (Section 3, Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793AS4PathWireExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L251) | unit/verify | unproven |
| positive | [`TestRFC6793AS4PathWireExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L248) | unit/verify | unproven |

### [`RFC6793-4.2.2-5`](#rfc6793-4.2.2-5)

When aggregator AS is non-mappable, MUST use AS4_AGGREGATOR and set AGGREGATOR AS field to AS_TRANS (Section 4.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L107) | unit/verify | unproven |
| negative | [`TestRFC6793NoAS4AggregatorForMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L191) | unit/verify | unproven |
| positive | [`TestForwardedAggregatorIsDowngradedWithItsCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L70) | unit/verify | unproven |
| positive | [`TestRFC6793AS4AggregatorForNonMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L154) | unit/verify | unproven |

### [`RFC6793-4.2.2-6`](#rfc6793-4.2.2-6)

If aggregator AS is mappable, AS4_AGGREGATOR MUST NOT be sent (Section 4.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestForwardedAggregatorIsDowngradedWithItsCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L73) | unit/verify | unproven |
| negative | [`TestRFC6793AS4AggregatorForNonMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L157) | unit/verify | unproven |
| positive | [`TestAMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L105) | unit/verify | unproven |
| positive | [`TestRFC6793NoAS4AggregatorForMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L189) | unit/verify | unproven |

### [`RFC6793-4.2.3-1`](#rfc6793-4.2.3-1)

When receiving from an OLD speaker, MUST be prepared to receive AS4_PATH along with AS_PATH (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793ASPathAloneNotInventedIntoFourOctet`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L222) | unit/verify | unproven |
| positive | [`TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L202) | unit/verify | unproven |

### [`RFC6793-4.2.3-2`](#rfc6793-4.2.3-2)

MUST be prepared to receive AS4_AGGREGATOR along with AGGREGATOR from an OLD speaker (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC6793ReceivedAS4AggregatorAccepted`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L237) | unit/verify | unproven |

### [`RFC6793-4.2.3-3`](#rfc6793-4.2.3-3)

When both AGGREGATOR and AS4_AGGREGATOR are received and AGGREGATOR.AS != AS_TRANS, AS4_AGGREGATOR and AS4_PATH SHALL be ignored (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793AggregatorOfTheWrongWidthIsNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L483) | unit/verify | unproven |
| negative | [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L383) | unit/verify | unproven |
| positive | [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L337) | unit/verify | unproven |

### [`RFC6793-4.2.3-4`](#rfc6793-4.2.3-4)

When both AGGREGATOR and AS4_AGGREGATOR are received and AGGREGATOR.AS != AS_TRANS, AGGREGATOR SHALL be taken as the aggregator info (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L387) | unit/verify | unproven |
| positive | [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L341) | unit/verify | unproven |

### [`RFC6793-4.2.3-5`](#rfc6793-4.2.3-5)

When both AGGREGATOR and AS4_AGGREGATOR are received and AGGREGATOR.AS != AS_TRANS, AS_PATH SHALL be taken as the AS path info (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L390) | unit/verify | unproven |
| positive | [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L345) | unit/verify | unproven |

### [`RFC6793-4.2.3-6`](#rfc6793-4.2.3-6)

When AGGREGATOR.AS == AS_TRANS, AGGREGATOR SHALL be ignored (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L349) | unit/verify | unproven |
| positive | [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L376) | unit/verify | unproven |

### [`RFC6793-4.2.3-7`](#rfc6793-4.2.3-7)

When AGGREGATOR.AS == AS_TRANS, AS4_AGGREGATOR SHALL be taken as the aggregator info (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793AggregatorWithRealASIgnoresAS4Attributes`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L352) | unit/verify | unproven |
| negative | [`TestRFC6793LoneAS4AggregatorIsDropped`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L511) | unit/verify | unproven |
| positive | [`TestRFC6793AggregatorWithASTransPromotesAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L379) | unit/verify | unproven |

### [`RFC6793-4.2.3-8`](#rfc6793-4.2.3-8)

If AS_PATH AS count < AS4_PATH AS count, AS4_PATH SHALL be ignored and AS_PATH SHALL be taken as AS path info (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793LongerASPathPrependsLeadingHops`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L260) | unit/verify | unproven |
| positive | [`TestRFC6793LongerAS4PathIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L277) | unit/verify | unproven |

### [`RFC6793-4.2.3-9`](#rfc6793-4.2.3-9)

If AS_PATH AS count >= AS4_PATH AS count, AS path info SHALL be constructed by prepending leading AS_PATH entries to AS4_PATH (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793LongerAS4PathIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L282) | unit/verify | unproven |
| positive | [`TestRFC6793EqualCountsPrependNothing`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L418) | unit/verify | unproven |
| positive | [`TestRFC6793LeadingASSetIsTakenWhole`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L438) | unit/verify | unproven |
| positive | [`TestRFC6793LongerASPathPrependsLeadingHops`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L255) | unit/verify | unproven |

### [`RFC6793-4.2.3-10`](#rfc6793-4.2.3-10)

A valid AS_CONFED_SEQUENCE or AS_CONFED_SET path segment SHALL be prepended if it is the leading segment or adjacent to a prepended segment (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793UnadjacentConfedSegmentIsNotPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L319) | unit/verify | unproven |
| positive | [`TestRFC6793LeadingConfedSegmentIsPrepended`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L299) | unit/verify | unproven |

### [`RFC6793-6-1`](#rfc6793-6-1)

AS4_PATH in an UPDATE SHALL be considered malformed if attribute length is not a multiple of two, is too small, segment length is zero or inconsistent, or segment type is undefined (Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793AS4PathMalformedRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L188) | unit/verify | unproven |
| positive | [`TestRFC6793AS4PathWellFormedAccepted`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L164) | unit/verify | unproven |
| positive | [`TestRFC6793MalformedAS4PathIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L459) | unit/verify | unproven |

### [`RFC6793-6-2`](#rfc6793-6-2)

AS4_AGGREGATOR in an UPDATE SHALL be considered malformed if the attribute length is not 8 (Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793AS4AggregatorLengthEight`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L219) | unit/verify | unproven |
| positive | [`TestRFC6793AS4AggregatorLengthEight`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L217) | unit/verify | unproven |

### [`RFC6793-6-3`](#rfc6793-6-3)

On receiving AS_CONFED_* segments in AS4_PATH from an OLD speaker, MUST discard those segments, adjust fields, and continue processing (Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793ReceivedConfedInAS4PathDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L331) | unit/verify | unproven |
| positive | [`TestRFC6793ReceivedConfedInAS4PathDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L328) | unit/verify | unproven |

### [`RFC6793-6-4`](#rfc6793-6-4)

On receiving malformed AS4_PATH from an OLD speaker, MUST discard the attribute and continue processing the UPDATE (Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793WellFormedAS4PathNotDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L297) | unit/verify | unproven |
| positive | [`TestRFC6793MalformedAS4PathDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L262) | unit/verify | unproven |

### [`RFC6793-6-5`](#rfc6793-6-5)

On receiving malformed AS4_AGGREGATOR from an OLD speaker, MUST discard the attribute and continue processing the UPDATE (Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCollapseAS4SelectsAggregatorPerSection423`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/aspath_collapse_test.go#L350) | unit/verify | unproven |
| positive | [`TestCollapseAS4DiscardsMalformedAS4Aggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/aspath_collapse_test.go#L482) | unit/verify | revert, verified |

## Extraction sign-off

No extraction sign-off exists for RFC 6793, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 6793, so its obligations are stated where they were written.
