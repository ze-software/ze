# RFC 6793 - BGP Support for Four-Octet Autonomous System (AS) Number Space

Partial. Every requirement this repository extracted from RFC 6793, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 56.7% | 17 of 30 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 3.3% | 1 of 30 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 30 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 39 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 30 | of 36 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 30 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 40.0% | 12 of 30 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 30 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 36 |
| Gated MUST-level | 30 |
| Obligations that bind Ze | 30 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 12 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 39 |
| Tagged units | 39 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc6793.md` |
| Requirement shard | `rfc/requirements/rfc6793.md` |
| RFC text | `rfc/full/rfc6793.txt` |

## Enrolment

Enrolled: BGP Support for Four-Octet AS Number Space

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- ASN4 capability advertisement and negotiation, 4-octet AS_PATH/AGGREGATOR between NEW speakers, 2-octet AS_PATH with AS_TRANS toward OLD speakers, AS4_PATH and AS4_AGGREGATOR construction (confederation segments excluded), AS4_PATH/AS4_AGGREGATOR malformed-attribute validation, AS_TRANS in the OPEN My AS field
- tests bound per requirement in [`rfc/requirements/rfc6793.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc6793.md).


**What the ledger says remains**

Twelve MUST/SHALL-level gaps, each annotated in [`rfc/short/rfc6793.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc6793.md).

- **Peer identity:** [`RFC6793-4.1-3`](#rfc6793-4.1-3) -- `UnpackOpen` never populates `Open.ASN4` from the code-65 capability ([`internal/component/bgp/message/open.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open.go)), so the session's OPEN-derived peer AS falls back to the two-octet My AS field ([`internal/component/bgp/reactor/reactor_dynamic.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_dynamic.go)). Receive-side reconstruction (RFC 6793 Section 4.2.3): [`RFC6793-4.2.3-3`](#rfc6793-4.2.3-3)/-4/-5/-6/-7 -- nothing compares the AGGREGATOR AS against AS_TRANS to choose between AGGREGATOR and AS4_AGGREGATOR ([`internal/component/bgp/plugins/rib/storage/attrparse.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/attrparse.go), :138-140); [`RFC6793-4.2.3-8`](#rfc6793-4.2.3-8)/-9/-10 -- `canonicalizeASPath` substitutes a received AS4_PATH wholesale instead of running the AS-number-count comparison and leading-segment prepend ([`internal/component/bgp/plugins/rib/storage/attrparse.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/attrparse.go)), and `MergeAS4Path`, which implements that algorithm, has no non-test caller.
- **Error handling:** [`RFC6793-4.1-6`](#rfc6793-4.1-6) and [`RFC6793-4.1-7`](#rfc6793-4.1-7) -- an AS4_PATH or AS4_AGGREGATOR received from a NEW speaker is used and forwarded rather than discarded, so the attribute is carried in an UPDATE between NEW speakers (the NEW-to-NEW fast path copies the attribute section verbatim, [`internal/component/bgp/wireu/aspath_rewrite.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/aspath_rewrite.go),:341,:363, and the full rewrite copies it through at [`internal/component/bgp/wireu/aspath_rewrite.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/aspath_rewrite.go),:521-524); ze itself originates neither attribute toward a NEW peer; [`RFC6793-6-5`](#rfc6793-6-5) -- a malformed AS4_AGGREGATOR is carried through unchecked (no RFC 7606 validator for type code 18).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 17 | one part of the gated population |
| Annotated instead of tested | 13 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **30** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (17):** [`RFC6793-4.1-1`](#rfc6793-4.1-1), [`RFC6793-4.1-2`](#rfc6793-4.1-2), [`RFC6793-4.1-4`](#rfc6793-4.1-4), [`RFC6793-4.1-5`](#rfc6793-4.1-5), [`RFC6793-4.2.1-1`](#rfc6793-4.2.1-1), [`RFC6793-4.2.2-1`](#rfc6793-4.2.2-1), [`RFC6793-4.2.2-2`](#rfc6793-4.2.2-2), [`RFC6793-4.2.2-3`](#rfc6793-4.2.2-3), [`RFC6793-4.2.2-4`](#rfc6793-4.2.2-4), [`RFC6793-3-1`](#rfc6793-3-1), [`RFC6793-4.2.2-5`](#rfc6793-4.2.2-5), [`RFC6793-4.2.2-6`](#rfc6793-4.2.2-6), [`RFC6793-4.2.3-1`](#rfc6793-4.2.3-1), [`RFC6793-6-1`](#rfc6793-6-1), [`RFC6793-6-2`](#rfc6793-6-2), [`RFC6793-6-3`](#rfc6793-6-3), [`RFC6793-6-4`](#rfc6793-6-4)

**Annotated instead of tested (13):** [`RFC6793-4.1-3`](#rfc6793-4.1-3), [`RFC6793-4.1-6`](#rfc6793-4.1-6), [`RFC6793-4.1-7`](#rfc6793-4.1-7), [`RFC6793-4.2.3-2`](#rfc6793-4.2.3-2), [`RFC6793-4.2.3-3`](#rfc6793-4.2.3-3), [`RFC6793-4.2.3-4`](#rfc6793-4.2.3-4), [`RFC6793-4.2.3-5`](#rfc6793-4.2.3-5), [`RFC6793-4.2.3-6`](#rfc6793-4.2.3-6), [`RFC6793-4.2.3-7`](#rfc6793-4.2.3-7), [`RFC6793-4.2.3-8`](#rfc6793-4.2.3-8), [`RFC6793-4.2.3-9`](#rfc6793-4.2.3-9), [`RFC6793-4.2.3-10`](#rfc6793-4.2.3-10), [`RFC6793-6-5`](#rfc6793-6-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6793-4.1-1` | A BGP speaker that supports four-octet AS numbers SHALL advertise this to its peers using BGP Capabilities Advertisements (Section 4.1) | SHALL | 4.1 | **positive:** `unit/verify` [`TestRFC6793OpenAdvertisesFourOctetASCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L60). **negative:** `unit/verify` [`TestRFC6793OpenOmitsCapabilityWhenDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L75) |
| `RFC6793-4.1-2` | The AS number of the BGP speaker MUST be carried in the Capability Value field of the "support for four-octet AS number capability" (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC6793ASN4CapabilityCarriesSpeakerAS`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc6793_asn4_test.go#L18). **negative:** `unit/verify` [`TestRFC6793ASN4CapabilityValueMustBeFourOctets`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc6793_asn4_test.go#L46) |
| `RFC6793-4.1-3` | When processing an OPEN from another NEW speaker, MUST use the AS number from the Capability Value field in lieu of the "My Autonomous System" field (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** UnpackOpen never populates Open.ASN4 from the code-65 capability -- it sets only Version, MyAS, HoldTime and BGPIdentifier (internal/component/bgp/message/open.go:171-180) -- so the reactor's only OPEN-derived peer AS falls back to the two-octet header field: resolveDynamicPeerSettings reads the always-zero open.ASN4 and keeps uint32(open.MyAS), i.e. AS_TRANS for a non-mappable peer (internal/component/bgp/reactor/reactor_dynamic.go:311-316), and negotiateWith passes that same zero as the peer ASN into Negotiate (internal/component/bgp/reactor/session_negotiate.go:27-32). The route-server plugin does read the capability value for its event view (internal/component/bgp/plugins/rs/server.go:647-649), but that is a reporting path, not the session's peer AS |
| `RFC6793-4.1-4` | When both peers support four-octet AS, MUST encode AS numbers as four-octet entities in both AS_PATH and AGGREGATOR attributes (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC6793EncodeFourOctetToNewSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L33). **negative:** `unit/verify` [`TestRFC6793EncodeTwoOctetToOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L65) |
| `RFC6793-4.1-5` | When both peers support four-octet AS, MUST assume received AS_PATH and AGGREGATOR encode AS numbers as four-octet entities (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC6793DecodeFourOctetWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L113). **negative:** `unit/verify` [`TestRFC6793DecodeTwoOctetWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L137) |
| `RFC6793-4.1-6` | AS4_PATH and AS4_AGGREGATOR MUST NOT be carried in an UPDATE between NEW BGP speakers (Section 4.1, Section 6) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze never ORIGINATES either attribute toward a NEW peer, but it forwards a received one verbatim, so an UPDATE between NEW speakers does carry them. tryDirectPrepend is entered exactly when srcASN4 == dstASN4 (internal/component/bgp/wireu/aspath_rewrite.go:292) and copies payload[:aspAttrOff] (internal/component/bgp/wireu/aspath_rewrite.go:341) and payload[aspAttrEnd:] (internal/component/bgp/wireu/aspath_rewrite.go:363) unchanged, so a received AS4_PATH or AS4_AGGREGATOR passes straight through to the NEW peer. The full rewrite does the same: as4PathForRewrite returns nil whenever dstASN4 is true (internal/component/bgp/wireu/aspath_as4.go:81-86) and the AttrAS4Path branch then copies the received attribute through (internal/component/bgp/wireu/aspath_rewrite.go:490-493), while AGGREGATOR transcoding runs only when srcASN4 != dstASN4 (internal/component/bgp/wireu/aspath_rewrite.go:429) so newAggValueLen stays 0 on a NEW-to-NEW session and the AttrAS4Aggregator branch copies through too (internal/component/bgp/wireu/aspath_rewrite.go:521-524). Same code fact as the receive-side half recorded at RFC6793-4.1-7. Disclosed in docs/features/rfc-status.md |
| `RFC6793-4.1-7` | A NEW speaker receiving AS4_PATH or AS4_AGGREGATOR from another NEW speaker MUST discard the path attribute and continue processing the UPDATE (Section 4.1, Section 6) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the discard is never conditioned on the negotiated capability. canonicalizeASPath returns the AS4_PATH value as the route's AS path whenever one is present, ignoring the asn4 flag it is given (internal/component/bgp/plugins/rib/storage/attrparse.go:198-209), so an AS4_PATH from a NEW speaker is used rather than discarded; AS4_AGGREGATOR from a NEW speaker is likewise kept, stored verbatim in OtherAttrs by the default branch (internal/component/bgp/plugins/rib/storage/attrparse.go:138-140). On the forward path a NEW-to-NEW session takes the same-encoding fast path, which copies the whole attribute section verbatim and therefore propagates the attribute too (internal/component/bgp/wireu/aspath_rewrite.go:136-139, tryDirectPrepend at :289-369) |
| `RFC6793-4.2.1-1` | AS_TRANS MUST be used in the OPEN "My Autonomous System" field when the NEW speaker does not have a two-octet AS number (Section 4.2.1) | MUST | 4.2.1 | **positive:** `unit/verify` [`TestRFC6793OpenMyASIsASTransWithoutTwoOctetAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L88). **negative:** `unit/verify` [`TestRFC6793OpenMyASIsRealASWhenMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6793_as4_open_test.go#L104) |
| `RFC6793-4.2.2-1` | When sending to an OLD speaker, MUST send AS path information in AS_PATH encoded with two-octet AS numbers (Section 4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestRFC6793EncodeTwoOctetToOldSpeaker`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L68). **negative:** `unit/verify` [`TestRFC6793TwoOctetASPathKeepsMappableASNs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L93) |
| `RFC6793-4.2.2-2` | When sending to an OLD speaker with non-mappable ASes, MUST also send AS4_PATH encoded with four-octet AS numbers (Section 4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestRFC6793TranscodeEmitsAS4PathForNonMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L49). **negative:** `unit/verify` [`TestRFC6793TranscodeOmitsAS4PathWhenAllMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L84) |
| `RFC6793-4.2.2-3` | When sending to an OLD speaker and all ASes are mappable, MUST NOT send AS4_PATH (Section 4.2.2) | MUST NOT | 4.2.2 | **positive:** `unit/verify` [`TestRFC6793TranscodeOmitsAS4PathWhenAllMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L82). **negative:** `unit/verify` [`TestRFC6793TranscodeEmitsAS4PathForNonMappable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L52) |
| `RFC6793-4.2.2-4` | When constructing AS4_PATH, MUST exclude AS_CONFED_SEQUENCE and AS_CONFED_SET path segments (Section 4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestRFC6793ConstructedAS4PathExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L222). **negative:** `unit/verify` [`TestRFC6793ConstructedAS4PathExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L224) |
| `RFC6793-3-1` | AS_CONFED_SEQUENCE and AS_CONFED_SET MUST NOT be carried in the AS4_PATH attribute of an UPDATE message (Section 3, Section 6) | MUST NOT | 3 | **positive:** `unit/verify` [`TestRFC6793AS4PathWireExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L248). **negative:** `unit/verify` [`TestRFC6793AS4PathWireExcludesConfed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L251) |
| `RFC6793-4.2.2-5` | When aggregator AS is non-mappable, MUST use AS4_AGGREGATOR and set AGGREGATOR AS field to AS_TRANS (Section 4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestForwardedAggregatorIsDowngradedWithItsCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L70). **positive:** `unit/verify` [`TestRFC6793AS4AggregatorForNonMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L154). **negative:** `unit/verify` [`TestAMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L107). **negative:** `unit/verify` [`TestRFC6793NoAS4AggregatorForMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L191) |
| `RFC6793-4.2.2-6` | If aggregator AS is mappable, AS4_AGGREGATOR MUST NOT be sent (Section 4.2.2) | MUST NOT | 4.2.2 | **positive:** `unit/verify` [`TestAMappableAggregatorGetsNoCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L105). **positive:** `unit/verify` [`TestRFC6793NoAS4AggregatorForMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L189). **negative:** `unit/verify` [`TestForwardedAggregatorIsDowngradedWithItsCompanion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc6793_aggregator_test.go#L73). **negative:** `unit/verify` [`TestRFC6793AS4AggregatorForNonMappableAggregator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L157) |
| `RFC6793-4.2.3-1` | When receiving from an OLD speaker, MUST be prepared to receive AS4_PATH along with AS_PATH (Section 4.2.3) | MUST | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc6793_as4_test.go#L50). **negative:** `unit/verify` [`TestRFC6793ASPathAloneNotInventedIntoFourOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc6793_as4_test.go#L75) |
| `RFC6793-4.2.3-2` | MUST be prepared to receive AS4_AGGREGATOR along with AGGREGATOR from an OLD speaker (Section 4.2.3) | MUST | 4.2.3 | **positive:** `unit/verify` [`TestRFC6793ReceivedAS4AggregatorAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc6793_as4_test.go#L97). **negative:** no negative test. **{single-polarity}:** the obligation is to accept the pair, and ParseAttributes has no rejection path to drive negatively: AGGREGATOR is interned and AS4_AGGREGATOR falls through the default branch into OtherAttrs, so every AGGREGATOR plus AS4_AGGREGATOR combination is accepted (internal/component/bgp/plugins/rib/storage/attrparse.go:96-102, :138-140). What ze does with the pair afterwards is governed by RFC6793-4.2.3-3 through -7, which are recorded as gaps |
| `RFC6793-4.2.3-3` | When both AGGREGATOR and AS4_AGGREGATOR are received and AGGREGATOR.AS != AS_TRANS, AS4_AGGREGATOR and AS4_PATH SHALL be ignored (Section 4.2.3) | SHALL | 4.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no producer reads the AGGREGATOR AS field to gate the AS4_* attributes. ParseAttributes interns AGGREGATOR (internal/component/bgp/plugins/rib/storage/attrparse.go:96-102) and routes AS4_AGGREGATOR to OtherAttrs (:138-140) independently, and canonicalizeASPath prefers the AS4_PATH whenever one is present without ever consulting AGGREGATOR (:198-209), so an AGGREGATOR carrying a real AS does not cause the AS4_* attributes to be ignored |
| `RFC6793-4.2.3-4` | When both AGGREGATOR and AS4_AGGREGATOR are received and AGGREGATOR.AS != AS_TRANS, AGGREGATOR SHALL be taken as the aggregator info (Section 4.2.3) | SHALL | 4.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no aggregator-selection step at all. ParseAttributes interns the received AGGREGATOR bytes and keeps AS4_AGGREGATOR beside them in OtherAttrs, and nothing later chooses between the two (internal/component/bgp/plugins/rib/storage/attrparse.go:96-102, :138-140); the AGGREGATOR is used because it is the only attribute anyone reads, not because AGGREGATOR.AS was compared against AS_TRANS |
| `RFC6793-4.2.3-5` | When both AGGREGATOR and AS4_AGGREGATOR are received and AGGREGATOR.AS != AS_TRANS, AS_PATH SHALL be taken as the AS path info (Section 4.2.3) | SHALL | 4.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** canonicalizeASPath takes the AS4_PATH as the AS path information whenever the attribute is present, with no AGGREGATOR check, so a received AGGREGATOR carrying a real AS does not make AS_PATH win (internal/component/bgp/plugins/rib/storage/attrparse.go:198-209) |
| `RFC6793-4.2.3-6` | When AGGREGATOR.AS == AS_TRANS, AGGREGATOR SHALL be ignored (Section 4.2.3) | SHALL | 4.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the AS_TRANS-bearing AGGREGATOR is never ignored: ParseAttributes interns it as the route's aggregator regardless of its AS value (internal/component/bgp/plugins/rib/storage/attrparse.go:96-102), and on the two-octet to four-octet forward path TranscodeASPath re-encodes that AS_TRANS into a four-octet AGGREGATOR while skipping the AS4_AGGREGATOR that held the real AS (internal/component/bgp/wireu/aspath_transcode.go:166-171 and :252-256) |
| `RFC6793-4.2.3-7` | When AGGREGATOR.AS == AS_TRANS, AS4_AGGREGATOR SHALL be taken as the aggregator info (Section 4.2.3) | SHALL | 4.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the received AS4_AGGREGATOR is never promoted to the aggregator information. ParseAttributes stores it verbatim in OtherAttrs and no reader substitutes it for AGGREGATOR (internal/component/bgp/plugins/rib/storage/attrparse.go:138-140); the ToAggregator helper that would perform the substitution has no non-test caller (internal/core/bgp/attribute/as4.go:322-327) |
| `RFC6793-4.2.3-8` | If AS_PATH AS count < AS4_PATH AS count, AS4_PATH SHALL be ignored and AS_PATH SHALL be taken as AS path info (Section 4.2.3) | SHALL | 4.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the ingest path never counts AS numbers -- canonicalizeASPath returns the AS4_PATH value whenever it is non-empty, so a longer AS4_PATH wins instead of being ignored (internal/component/bgp/plugins/rib/storage/attrparse.go:198-209). MergeAS4Path implements exactly this count comparison (internal/core/bgp/attribute/as4.go:361-407) but has no non-test caller |
| `RFC6793-4.2.3-9` | If AS_PATH AS count >= AS4_PATH AS count, AS path info SHALL be constructed by prepending leading AS_PATH entries to AS4_PATH (Section 4.2.3) | SHALL | 4.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no prepend of leading AS_PATH entries happens on ingest: canonicalizeASPath substitutes the AS4_PATH wholesale for the AS_PATH, so a longer AS_PATH loses its leading AS numbers instead of contributing them (internal/component/bgp/plugins/rib/storage/attrparse.go:198-209). MergeAS4Path performs the prepend (internal/core/bgp/attribute/as4.go:381-406) but has no non-test caller |
| `RFC6793-4.2.3-10` | A valid AS_CONFED_SEQUENCE or AS_CONFED_SET path segment SHALL be prepended if it is the leading segment or adjacent to a prepended segment (Section 4.2.3) | SHALL | 4.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** there is no reconstruction step on ingest to prepend into, so no confederation-segment adjacency rule exists: canonicalizeASPath replaces the AS_PATH with the AS4_PATH and drops every AS_PATH segment, leading confederation segments included (internal/component/bgp/plugins/rib/storage/attrparse.go:198-209). MergeAS4Path, the nearest producer, copies leading AS_PATH segments by AS count and applies no adjacency rule of its own (internal/core/bgp/attribute/as4.go:384-401), and it has no non-test caller |
| `RFC6793-6-1` | AS4_PATH in an UPDATE SHALL be considered malformed if attribute length is not a multiple of two, is too small, segment length is zero or inconsistent, or segment type is undefined (Section 6) | SHALL | 6 | **positive:** `unit/verify` [`TestRFC6793AS4PathWellFormedAccepted`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L164). **negative:** `unit/verify` [`TestRFC6793AS4PathMalformedRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L188) |
| `RFC6793-6-2` | AS4_AGGREGATOR in an UPDATE SHALL be considered malformed if the attribute length is not 8 (Section 6) | SHALL | 6 | **positive:** `unit/verify` [`TestRFC6793AS4AggregatorLengthEight`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L217). **negative:** `unit/verify` [`TestRFC6793AS4AggregatorLengthEight`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L219) |
| `RFC6793-6-3` | On receiving AS_CONFED_* segments in AS4_PATH from an OLD speaker, MUST discard those segments, adjust fields, and continue processing (Section 6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC6793ReceivedConfedInAS4PathDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L328). **negative:** `unit/verify` [`TestRFC6793ReceivedConfedInAS4PathDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L331) |
| `RFC6793-6-4` | On receiving malformed AS4_PATH from an OLD speaker, MUST discard the attribute and continue processing the UPDATE (Section 6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC6793MalformedAS4PathDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L262). **negative:** `unit/verify` [`TestRFC6793WellFormedAS4PathNotDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc6793_as4_test.go#L297) |
| `RFC6793-6-5` | On receiving malformed AS4_AGGREGATOR from an OLD speaker, MUST discard the attribute and continue processing the UPDATE (Section 6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nothing on the receive path length-checks AS4_AGGREGATOR and drops it. ParseAttributes copies any AS4_AGGREGATOR into OtherAttrs by length alone (internal/component/bgp/plugins/rib/storage/attrparse.go:138-140, appendOtherAttr at :254-260); the wireu egress paths locate it by offset and copy it through untouched (internal/component/bgp/wireu/aspath_transcode.go:113-117 and :252-256); and RFC 7606 structural validation registers no validator for type code 18 (internal/component/bgp/message/rfc7606.go:415-429). ParseAS4Aggregator does reject a length other than 8 (internal/core/bgp/attribute/as4.go:302-305), but its only production reachability is parseAtLocked, which returns the error to the caller rather than discarding the attribute and continuing (internal/core/bgp/attribute/wire.go:346-349) |
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
| [`RFC6793-4.1-6`](#rfc6793-4.1-6) AS4_PATH and AS4_AGGREGATOR MUST NOT be carried in an UPDATE between NEW BGP speakers (Section 4.1, Section 6) | {gap}, no test | ze never ORIGINATES either attribute toward a NEW peer, but it forwards a received one verbatim, so an UPDATE between NEW speakers does carry them. tryDirectPrepend is entered exactly when srcASN4 == dstASN4 (internal/component/bgp/wireu/aspath_rewrite.go:292) and copies payload[:aspAttrOff] (internal/component/bgp/wireu/aspath_rewrite.go:341) and payload[aspAttrEnd:] (internal/component/bgp/wireu/aspath_rewrite.go:363) unchanged, so a received AS4_PATH or AS4_AGGREGATOR passes straight through to the NEW peer. The full rewrite does the same: as4PathForRewrite returns nil whenever dstASN4 is true (internal/component/bgp/wireu/aspath_as4.go:81-86) and the AttrAS4Path branch then copies the received attribute through (internal/component/bgp/wireu/aspath_rewrite.go:490-493), while AGGREGATOR transcoding runs only when srcASN4 != dstASN4 (internal/component/bgp/wireu/aspath_rewrite.go:429) so newAggValueLen stays 0 on a NEW-to-NEW session and the AttrAS4Aggregator branch copies through too (internal/component/bgp/wireu/aspath_rewrite.go:521-524). Same code fact as the receive-side half recorded at RFC6793-4.1-7. Disclosed in docs/features/rfc-status.md |
| [`RFC6793-4.1-7`](#rfc6793-4.1-7) A NEW speaker receiving AS4_PATH or AS4_AGGREGATOR from another NEW speaker MUST discard the path attribute and continue processing the UPDATE (Section 4.1, Section 6) | {gap}, no test | the discard is never conditioned on the negotiated capability. canonicalizeASPath returns the AS4_PATH value as the route's AS path whenever one is present, ignoring the asn4 flag it is given (internal/component/bgp/plugins/rib/storage/attrparse.go:198-209), so an AS4_PATH from a NEW speaker is used rather than discarded; AS4_AGGREGATOR from a NEW speaker is likewise kept, stored verbatim in OtherAttrs by the default branch (internal/component/bgp/plugins/rib/storage/attrparse.go:138-140). On the forward path a NEW-to-NEW session takes the same-encoding fast path, which copies the whole attribute section verbatim and therefore propagates the attribute too (internal/component/bgp/wireu/aspath_rewrite.go:136-139, tryDirectPrepend at :289-369) |
| [`RFC6793-4.2.3-3`](#rfc6793-4.2.3-3) When both AGGREGATOR and AS4_AGGREGATOR are received and AGGREGATOR.AS != AS_TRANS, AS4_AGGREGATOR and AS4_PATH SHALL be ignored (Section 4.2.3) | {gap}, no test | no producer reads the AGGREGATOR AS field to gate the AS4_* attributes. ParseAttributes interns AGGREGATOR (internal/component/bgp/plugins/rib/storage/attrparse.go:96-102) and routes AS4_AGGREGATOR to OtherAttrs (:138-140) independently, and canonicalizeASPath prefers the AS4_PATH whenever one is present without ever consulting AGGREGATOR (:198-209), so an AGGREGATOR carrying a real AS does not cause the AS4_* attributes to be ignored |
| [`RFC6793-4.2.3-4`](#rfc6793-4.2.3-4) When both AGGREGATOR and AS4_AGGREGATOR are received and AGGREGATOR.AS != AS_TRANS, AGGREGATOR SHALL be taken as the aggregator info (Section 4.2.3) | {gap}, no test | ze has no aggregator-selection step at all. ParseAttributes interns the received AGGREGATOR bytes and keeps AS4_AGGREGATOR beside them in OtherAttrs, and nothing later chooses between the two (internal/component/bgp/plugins/rib/storage/attrparse.go:96-102, :138-140); the AGGREGATOR is used because it is the only attribute anyone reads, not because AGGREGATOR.AS was compared against AS_TRANS |
| [`RFC6793-4.2.3-5`](#rfc6793-4.2.3-5) When both AGGREGATOR and AS4_AGGREGATOR are received and AGGREGATOR.AS != AS_TRANS, AS_PATH SHALL be taken as the AS path info (Section 4.2.3) | {gap}, no test | canonicalizeASPath takes the AS4_PATH as the AS path information whenever the attribute is present, with no AGGREGATOR check, so a received AGGREGATOR carrying a real AS does not make AS_PATH win (internal/component/bgp/plugins/rib/storage/attrparse.go:198-209) |
| [`RFC6793-4.2.3-6`](#rfc6793-4.2.3-6) When AGGREGATOR.AS == AS_TRANS, AGGREGATOR SHALL be ignored (Section 4.2.3) | {gap}, no test | the AS_TRANS-bearing AGGREGATOR is never ignored: ParseAttributes interns it as the route's aggregator regardless of its AS value (internal/component/bgp/plugins/rib/storage/attrparse.go:96-102), and on the two-octet to four-octet forward path TranscodeASPath re-encodes that AS_TRANS into a four-octet AGGREGATOR while skipping the AS4_AGGREGATOR that held the real AS (internal/component/bgp/wireu/aspath_transcode.go:166-171 and :252-256) |
| [`RFC6793-4.2.3-7`](#rfc6793-4.2.3-7) When AGGREGATOR.AS == AS_TRANS, AS4_AGGREGATOR SHALL be taken as the aggregator info (Section 4.2.3) | {gap}, no test | the received AS4_AGGREGATOR is never promoted to the aggregator information. ParseAttributes stores it verbatim in OtherAttrs and no reader substitutes it for AGGREGATOR (internal/component/bgp/plugins/rib/storage/attrparse.go:138-140); the ToAggregator helper that would perform the substitution has no non-test caller (internal/core/bgp/attribute/as4.go:322-327) |
| [`RFC6793-4.2.3-8`](#rfc6793-4.2.3-8) If AS_PATH AS count < AS4_PATH AS count, AS4_PATH SHALL be ignored and AS_PATH SHALL be taken as AS path info (Section 4.2.3) | {gap}, no test | the ingest path never counts AS numbers -- canonicalizeASPath returns the AS4_PATH value whenever it is non-empty, so a longer AS4_PATH wins instead of being ignored (internal/component/bgp/plugins/rib/storage/attrparse.go:198-209). MergeAS4Path implements exactly this count comparison (internal/core/bgp/attribute/as4.go:361-407) but has no non-test caller |
| [`RFC6793-4.2.3-9`](#rfc6793-4.2.3-9) If AS_PATH AS count >= AS4_PATH AS count, AS path info SHALL be constructed by prepending leading AS_PATH entries to AS4_PATH (Section 4.2.3) | {gap}, no test | no prepend of leading AS_PATH entries happens on ingest: canonicalizeASPath substitutes the AS4_PATH wholesale for the AS_PATH, so a longer AS_PATH loses its leading AS numbers instead of contributing them (internal/component/bgp/plugins/rib/storage/attrparse.go:198-209). MergeAS4Path performs the prepend (internal/core/bgp/attribute/as4.go:381-406) but has no non-test caller |
| [`RFC6793-4.2.3-10`](#rfc6793-4.2.3-10) A valid AS_CONFED_SEQUENCE or AS_CONFED_SET path segment SHALL be prepended if it is the leading segment or adjacent to a prepended segment (Section 4.2.3) | {gap}, no test | there is no reconstruction step on ingest to prepend into, so no confederation-segment adjacency rule exists: canonicalizeASPath replaces the AS_PATH with the AS4_PATH and drops every AS_PATH segment, leading confederation segments included (internal/component/bgp/plugins/rib/storage/attrparse.go:198-209). MergeAS4Path, the nearest producer, copies leading AS_PATH segments by AS count and applies no adjacency rule of its own (internal/core/bgp/attribute/as4.go:384-401), and it has no non-test caller |
| [`RFC6793-6-5`](#rfc6793-6-5) On receiving malformed AS4_AGGREGATOR from an OLD speaker, MUST discard the attribute and continue processing the UPDATE (Section 6) | {gap}, no test | nothing on the receive path length-checks AS4_AGGREGATOR and drops it. ParseAttributes copies any AS4_AGGREGATOR into OtherAttrs by length alone (internal/component/bgp/plugins/rib/storage/attrparse.go:138-140, appendOtherAttr at :254-260); the wireu egress paths locate it by offset and copy it through untouched (internal/component/bgp/wireu/aspath_transcode.go:113-117 and :252-256); and RFC 7606 structural validation registers no validator for type code 18 (internal/component/bgp/message/rfc7606.go:415-429). ParseAS4Aggregator does reject a length other than 8 (internal/core/bgp/attribute/as4.go:302-305), but its only production reachability is parseAtLocked, which returns the error to the caller rather than discarding the attribute and continuing (internal/core/bgp/attribute/wire.go:346-349) |

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

No test carries RFC6793-4.1-6, so no unit is bound to it.

### [`RFC6793-4.1-7`](#rfc6793-4.1-7)

A NEW speaker receiving AS4_PATH or AS4_AGGREGATOR from another NEW speaker MUST discard the path attribute and continue processing the UPDATE (Section 4.1, Section 6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6793-4.1-7, so no unit is bound to it.

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
| negative | [`TestRFC6793ASPathAloneNotInventedIntoFourOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc6793_as4_test.go#L75) | unit/verify | unproven |
| positive | [`TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc6793_as4_test.go#L50) | unit/verify | unproven |

### [`RFC6793-4.2.3-2`](#rfc6793-4.2.3-2)

MUST be prepared to receive AS4_AGGREGATOR along with AGGREGATOR from an OLD speaker (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC6793ReceivedAS4AggregatorAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc6793_as4_test.go#L97) | unit/verify | unproven |

### [`RFC6793-4.2.3-3`](#rfc6793-4.2.3-3)

When both AGGREGATOR and AS4_AGGREGATOR are received and AGGREGATOR.AS != AS_TRANS, AS4_AGGREGATOR and AS4_PATH SHALL be ignored (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6793-4.2.3-3, so no unit is bound to it.

### [`RFC6793-4.2.3-4`](#rfc6793-4.2.3-4)

When both AGGREGATOR and AS4_AGGREGATOR are received and AGGREGATOR.AS != AS_TRANS, AGGREGATOR SHALL be taken as the aggregator info (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6793-4.2.3-4, so no unit is bound to it.

### [`RFC6793-4.2.3-5`](#rfc6793-4.2.3-5)

When both AGGREGATOR and AS4_AGGREGATOR are received and AGGREGATOR.AS != AS_TRANS, AS_PATH SHALL be taken as the AS path info (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6793-4.2.3-5, so no unit is bound to it.

### [`RFC6793-4.2.3-6`](#rfc6793-4.2.3-6)

When AGGREGATOR.AS == AS_TRANS, AGGREGATOR SHALL be ignored (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6793-4.2.3-6, so no unit is bound to it.

### [`RFC6793-4.2.3-7`](#rfc6793-4.2.3-7)

When AGGREGATOR.AS == AS_TRANS, AS4_AGGREGATOR SHALL be taken as the aggregator info (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6793-4.2.3-7, so no unit is bound to it.

### [`RFC6793-4.2.3-8`](#rfc6793-4.2.3-8)

If AS_PATH AS count < AS4_PATH AS count, AS4_PATH SHALL be ignored and AS_PATH SHALL be taken as AS path info (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6793-4.2.3-8, so no unit is bound to it.

### [`RFC6793-4.2.3-9`](#rfc6793-4.2.3-9)

If AS_PATH AS count >= AS4_PATH AS count, AS path info SHALL be constructed by prepending leading AS_PATH entries to AS4_PATH (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6793-4.2.3-9, so no unit is bound to it.

### [`RFC6793-4.2.3-10`](#rfc6793-4.2.3-10)

A valid AS_CONFED_SEQUENCE or AS_CONFED_SET path segment SHALL be prepended if it is the leading segment or adjacent to a prepended segment (Section 4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6793-4.2.3-10, so no unit is bound to it.

### [`RFC6793-6-1`](#rfc6793-6-1)

AS4_PATH in an UPDATE SHALL be considered malformed if attribute length is not a multiple of two, is too small, segment length is zero or inconsistent, or segment type is undefined (Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6793AS4PathMalformedRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L188) | unit/verify | unproven |
| positive | [`TestRFC6793AS4PathWellFormedAccepted`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_as4_test.go#L164) | unit/verify | unproven |

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

No test carries RFC6793-6-5, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 6793, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 6793, so its obligations are stated where they were written.
