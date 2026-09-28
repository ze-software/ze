# RFC 8669 - Segment Routing Prefix Segment Identifier Extensions for BGP

Partial. Every requirement this repository extracted from RFC 8669, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 28.0% | 7 of 25 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 32.0% | 8 of 25 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 25 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 34.3% | 12 of 35 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 25 | of 44 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 25 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 25 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 25 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 25 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 40.0% | 10 of 25 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 12 | of 25 gated MUSTs judged | 10 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 25 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 44 |
| Gated MUST-level | 25 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 10 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 35 |
| Tagged units | 35 |
| Recorded audit verdicts | 12 |
| Discrimination records | 12 |
| Summary | `rfc/short/rfc8669.md` |
| Requirement shard | `rfc/requirements/rfc8669.md` |
| RFC text | `rfc/full/rfc8669.txt` |

## Enrolment

Enrolled: SR Prefix Segment Identifier Extensions for BGP

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Attribute code 40 is registered as optional transitive ([`internal/core/bgp/attribute/attribute.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/attribute.go)) and carried end to end: the Label-Index TLV (type 1) and the Originator SRGB TLV (type 3) are encoded from route configuration with their Reserved and Flags octets cleared ([`internal/core/bgp/attribute/prefixsid.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/prefixsid.go)) and attached to labeled-unicast and VPN UPDATEs ([`internal/component/bgp/message/update_build_labeled.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_labeled.go))
- on reception every TLV is bounds-checked with Section 6 attribute-discard for an overrunning TLV length or trailing bytes ([`internal/component/bgp/message/rfc7606.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606.go)), a duplicate attribute is discarded unexamined in favor of the first for PROCESSING purposes ([`internal/component/bgp/message/rfc7606.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606.go)), a duplicate recognized Service TLV never displaces the first ([`internal/component/bgp/plugins/rib/pool/srv6sid.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid.go)), the Section 4 EBGP boundary discards the attribute unless the peer is configured to accept it ([`internal/component/bgp/reactor/session_validation.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation.go)), the Section 8 boundary removes it on egress toward an EBGP peer the operator has not configured for propagation, on every rail that writes an UPDATE: the two forward rails, the two origination rails and the API/readvertise announce rail all ask prefixSIDAllowedTo ([`internal/component/bgp/reactor/forward_prefix_sid.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid.go), [`internal/component/bgp/reactor/reactor_api_batch.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_batch.go)), unknown TLVs and the Reserved/Flags fields are ignored and left byte-identical for propagation, and the label carried in a received labeled-unicast NLRI is the outbound label programmed toward the next hop ([`internal/core/bgp/nlri/nlrisplit/labeled.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/labeled.go) to [`internal/plugins/fib/kernel/nexthop_linux.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/nexthop_linux.go)). Requirements bound per line in [`rfc/short/rfc8669.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8669.md).


**What the ledger says remains**

Ten MUST gaps annotated in [`rfc/short/rfc8669.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8669.md).

- **Duplicate attribute, wire half:** [`RFC8669-6-2`](#rfc8669-6-2) -- a skipped duplicate Prefix-SID is not removed from the bytes forwarded on. A valid first occurrence records no DiscardEntry and ApplyAttrDiscard returns the attributes untouched ([`internal/component/bgp/message/attr_discard.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/attr_discard.go)); when the first occurrence is the malformed one, applyInPlace tombstones only it, because AttrFind returns the first match ([`internal/component/bgp/message/attr_discard.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/attr_discard.go), [`internal/core/bgp/attribute/iterator.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/iterator.go)). The other nine are in the SR-MPLS label-index semantics ze does not implement: [`RFC8669-3.1-1`](#rfc8669-3.1-1)/4.1-1/4.1-2 -- the Label-Index TLV is never required nor looked for, so a labeled-unicast Prefix-SID without one is accepted instead of being considered "invalid"; [`RFC8669-4.1-3`](#rfc8669-4.1-3) -- no label-index-to-prefix reverse index exists, so the "conflicting" state is never detected; [`RFC8669-4.1-4`](#rfc8669-4.1-4)/4.1-6 -- with neither state computed, the ignore action and the Section 6 routing of an "invalid" attribute have no trigger; [`RFC8669-4.1-5`](#rfc8669-4.1-5) -- ze allocates no local (dynamic) label for a BGP prefix, the programmed label is always the one received in the NLRI; [`RFC8669-4.1-7`](#rfc8669-4.1-7) -- an implicit-NULL (3) label in the NLRI is programmed as an MPLS push rather than a pop ([`internal/plugins/fib/kernel/mpls.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/mpls.go), [`internal/plugins/fib/kernel/nexthop_linux.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/nexthop_linux.go)); [`RFC8669-5.1-1`](#rfc8669-5.1-1) -- the advertised NLRI label comes from route configuration and no matching incoming MPLS entry is programmed, only LDP and RSVP-TE emit MPLS ingress state.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the gated population |
| Annotated instead of tested | 18 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **25** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (7):** [`RFC8669-3.1-4`](#rfc8669-3.1-4), [`RFC8669-3.1-6`](#rfc8669-3.1-6), [`RFC8669-3.2-2`](#rfc8669-3.2-2), [`RFC8669-4-1`](#rfc8669-4-1), [`RFC8669-8-1`](#rfc8669-8-1), [`RFC8669-6-1`](#rfc8669-6-1), [`RFC8669-6-3`](#rfc8669-6-3)

**Annotated instead of tested (18):** [`RFC8669-3-1`](#rfc8669-3-1), [`RFC8669-3.1-1`](#rfc8669-3.1-1), [`RFC8669-3.1-2`](#rfc8669-3.1-2), [`RFC8669-3.1-3`](#rfc8669-3.1-3), [`RFC8669-3.1-5`](#rfc8669-3.1-5), [`RFC8669-3.2-1`](#rfc8669-3.2-1), [`RFC8669-3.2-3`](#rfc8669-3.2-3), [`RFC8669-3.2-4`](#rfc8669-3.2-4), [`RFC8669-4.1-1`](#rfc8669-4.1-1), [`RFC8669-4.1-2`](#rfc8669-4.1-2), [`RFC8669-4.1-3`](#rfc8669-4.1-3), [`RFC8669-4.1-4`](#rfc8669-4.1-4), [`RFC8669-4.1-5`](#rfc8669-4.1-5), [`RFC8669-4.1-6`](#rfc8669-4.1-6), [`RFC8669-4.1-7`](#rfc8669-4.1-7), [`RFC8669-4.1-8`](#rfc8669-4.1-8), [`RFC8669-5.1-1`](#rfc8669-5.1-1), [`RFC8669-6-2`](#rfc8669-6-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8669-3-1` | For future extensibility, unknown TLVs MUST be ignored and propagated unmodified. (§3, §6) | MUST | 3 | **positive:** `unit/verify` [`TestRFC8669UnknownTLVIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L80). **negative:** no negative test. **{single-polarity}:** ze has no producer that rewrites a Prefix-SID TLV. validatePrefixSIDAttr walks TLV headers and reads the value of types 5 and 6 only, never writing any of them (internal/component/bgp/message/rfc7606.go:840-856), and no forward path edits the attribute value. Ze's one code-40 modification is coarser than this requirement rather than a counter-example to it: applyFactsNextHop drops the WHOLE attribute on every next-hop-changing readvertisement (internal/component/bgp/reactor/peer_forward_facts.go:241), taking any unknown TLV with it, so on that rail the attribute is not propagated at all. Where the attribute IS propagated its bytes are untouched, and no input can make a TLV come out modified, so there is nothing to drive negatively |
| `RFC8669-3.1-1` | Label-Index TLV MUST be present in the BGP Prefix-SID attribute attached to IPv4/IPv6 Labeled Unicast prefixes (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** neither the sender nor the receiver requires TLV type 1 for labeled unicast -- validatePrefixSIDAttr accepts a Prefix-SID with no Label-Index TLV (internal/component/bgp/message/rfc7606.go:837) and BuildLabeledUnicast attaches whatever bytes the route configuration produced, including an SRv6-only attribute (internal/component/bgp/message/update_build_labeled.go:189) |
| `RFC8669-3.1-2` | It MUST be ignored when received for other BGP AFI/SAFI combinations. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8669LabelIndexIgnoredOnNonLabeledUnicastFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L98). **negative:** no negative test. **{single-polarity}:** ze has no receive-side Label-Index consumer for any family -- ExtractSRv6SIDFull steps over TLV type 1 by length (internal/component/bgp/plugins/rib/pool/srv6sid.go:47) and no other reader of TLV type 1 exists in internal/component/bgp or internal/core/bgp -- so the TLV is ignored on every AFI/SAFI and there is no label-index-driven behavior to drive negatively |
| `RFC8669-3.1-3` | RESERVED: 8-bit field. It MUST be clear on transmission (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8669LabelIndexTLVReservedAndFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L30). **negative:** no negative test. **{single-polarity}:** ParsePrefixSID hardcodes the Reserved octet to 0 on encode and no code path emits a non-zero value, so there is no negative input to reject (internal/core/bgp/attribute/prefixsid.go) |
| `RFC8669-3.1-4` | RESERVED: 8-bit field. It MUST be clear on transmission and MUST be ignored on reception. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8669LabelIndexReservedAndFlagsZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L96). **negative:** `unit/verify` [`TestRFC8669LabelIndexNonZeroReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L114) |
| `RFC8669-3.1-5` | Flags: 16 bits of flags. None are defined by this document. The Flags field MUST be clear on transmission (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8669LabelIndexTLVReservedAndFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L31). **negative:** no negative test. **{single-polarity}:** ParsePrefixSID hardcodes the Flags field to 0 on encode with no non-zero path to reject (internal/core/bgp/attribute/prefixsid.go) |
| `RFC8669-3.1-6` | Flags: 16 bits of flags. None are defined by this document. The Flags field MUST be clear on transmission and MUST be ignored on reception. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8669LabelIndexReservedAndFlagsZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L97). **negative:** `unit/verify` [`TestRFC8669LabelIndexNonZeroFlagsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L127) |
| `RFC8669-3.2-1` | Flags: 16 bits of flags. None are defined in this document. The Flags field MUST be clear on transmission (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8669OriginatorSRGBTLVFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L55). **negative:** no negative test. **{single-polarity}:** parsePrefixSIDWithSRGB hardcodes both Flags octets to 0 on encode with no non-zero path to reject (internal/core/bgp/attribute/prefixsid.go) |
| `RFC8669-3.2-2` | Flags: 16 bits of flags. None are defined in this document. The Flags field MUST be clear on transmission and MUST be ignored on reception. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8669SRGBFlagsZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L140). **negative:** `unit/verify` [`TestRFC8669SRGBNonZeroFlagsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L151) |
| `RFC8669-3.2-3` | Originator SRGB TLV MUST NOT be changed during the propagation of the BGP update (§3.2) | MUST NOT | 3.2 | **positive:** `unit/verify` [`TestRFC8669SRGBUnchangedThroughReceiveValidation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L170). **negative:** no negative test. **{single-polarity}:** no producer writes into an Originator SRGB TLV. The receive validator reads TLV headers and never writes a value (internal/component/bgp/message/rfc7606.go:837-856). The forward path's only code-40 operation is a whole-attribute suppress on a next-hop change (internal/component/bgp/reactor/peer_forward_facts.go:241) -- that removes the Originator SRGB along with everything else in the attribute rather than changing it, so it is not a counter-example to "MUST NOT be changed" but it does mean the TLV survives propagation only on the rails that keep the attribute. On those rails the SRGB octets are byte-identical, and no input produces changed SRGB bytes to drive negatively |
| `RFC8669-3.2-4` | The Originator SRGB TLV may only appear in a BGP Prefix-SID attribute attached to IPv4/IPv6 Labeled Unicast prefixes ([RFC8277]). It MUST be ignored when received for other BGP AFI/SAFI combinations. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8669SRGBIgnoredOnNonLabeledUnicastFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L125). **negative:** no negative test. **{single-polarity}:** ze has no receive-side Originator SRGB consumer for any family -- ExtractSRv6SIDFull steps over TLV type 3 by length (internal/component/bgp/plugins/rib/pool/srv6sid.go:47) and the only SRGB code in internal/component/bgp is the config-side encoder -- so the TLV is ignored on every AFI/SAFI and there is no SRGB-driven behavior to drive negatively |
| `RFC8669-4-1` | A BGP speaker receiving a BGP Prefix-SID attribute from an External BGP (EBGP) neighbor residing outside the boundaries of the SR domain MUST discard the attribute unless it is configured to accept the attribute from the EBGP neighbor. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC8669PrefixSIDFromEBGPAcceptedWhenConfigured`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_test.go#L58). **positive:** `unit/verify` [`TestRFC8669PrefixSIDKeptPathsKeepExactlyOneCopy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_multi_test.go#L120). **negative:** `unit/verify` [`TestRFC8669PrefixSIDEveryOccurrenceDiscardedFromEBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_multi_test.go#L67). **negative:** `unit/verify` [`TestRFC8669PrefixSIDFromEBGPDiscardedByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_test.go#L80) |
| `RFC8669-4.1-1` | When the BGP Prefix-SID attribute is attached to a BGP Labeled IPv4 or IPv6 Unicast [RFC8277] AFI/SAFI, it MUST contain the Label-Index TLV (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the receive validator never scans for TLV type 1, so a labeled-unicast Prefix-SID with no Label-Index TLV is accepted as well formed (internal/component/bgp/message/rfc7606.go:837) |
| `RFC8669-4.1-2` | A BGP Prefix-SID attribute received without a Label-Index TLV MUST be considered to be "invalid" by the receiving speaker. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no "invalid" state for the Prefix-SID attribute -- validatePrefixSIDAttr returns nil for any attribute whose TLVs fit the declared bounds, whatever their types (internal/component/bgp/message/rfc7606.go:858) |
| `RFC8669-4.1-3` | If multiple different prefixes are received with the same label index, all of the different prefixes MUST have their BGP Prefix-SID attribute considered to be "conflicting". (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze keeps no label-index-to-prefix reverse index and reads no label index at all -- the only reader of the Prefix-SID TLV list is the SRv6 extractor, which skips TLV type 1 (internal/component/bgp/plugins/rib/pool/srv6sid.go:47), so no conflict can be detected |
| `RFC8669-4.1-4` | When a BGP speaker receives a path from a neighbor with an "invalid" or "conflicting" BGP Prefix-SID attribute, or when a BGP speaker receives a path from a neighbor with a BGP Prefix-SID attribute but is unable to process it (e.g., local policy disables the functionality), it MUST ignore the BGP Prefix-SID attribute. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** neither the invalid nor the conflicting state is computed, so the ignore action has no trigger -- the receive path's only Prefix-SID discard reasons are malformed TLV bounds (internal/component/bgp/message/rfc7606.go:842) and the EBGP boundary rule (internal/component/bgp/reactor/session_validation.go:107) |
| `RFC8669-4.1-5` | For the purposes of label allocation, a BGP speaker MUST assign a local (also called dynamic) label (non-SRGB) for such a prefix as per classic Multiprotocol BGP IPv4/IPv6 Labeled Unicast ([RFC8277]) operation. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze allocates no local label for a BGP prefix at all -- the label programmed for a labeled-unicast route is the one carried in the received NLRI (internal/component/bgp/plugins/rib/rib_bestchange.go:900), and the only MPLS ingress allocators are LDP and RSVP-TE (internal/plugins/ldp/fib.go:135, internal/plugins/rsvpte/fib.go:45) |
| `RFC8669-4.1-6` | In the case of an "invalid" BGP Prefix-SID attribute, a BGP speaker MUST follow the error-handling rules specified in Section 6. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Section 6 error handling exists and fires for malformed TLV bounds and trailing bytes (internal/component/bgp/message/rfc7606.go:842,:859), but the "invalid" condition that would route a missing-Label-Index attribute into it is never computed (internal/component/bgp/message/rfc7606.go:837) |
| `RFC8669-4.1-7` | Specifically, a BGP speaker receiving a prefix with a BGP Prefix-SID attribute and a label NLRI field of Implicit NULL [RFC3032] from a neighbor MUST adhere to standard behavior and program its MPLS data plane to pop the top label when forwarding traffic to the prefix. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the received NLRI label is programmed as an MPLS encapsulation without any implicit-NULL exception -- validateMPLSLabels accepts label 3 like any other value (internal/plugins/fib/kernel/mpls.go:23) and buildMPLSEncap pushes whatever labels it is given (internal/plugins/fib/kernel/nexthop_linux.go:75), so an implicit-NULL labeled-unicast route is programmed as a push of label 3 rather than a pop |
| `RFC8669-4.1-8` | The label NLRI defines the outbound label that MUST be used by the receiving node (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC8669NLRILabelIsTheOutboundLabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L33). **negative:** no negative test. **{single-polarity}:** the label taken off the received NLRI is carried unchanged to the forwarding plane (internal/core/bgp/nlri/nlrisplit/labeled.go:110 to internal/component/bgp/plugins/rib/rib_bestchange.go:900 to internal/plugins/fib/kernel/nexthop_linux.go:75); the rule states which label to use, not a condition to reject, so there is no non-conforming input whose rejection could be asserted |
| `RFC8669-5.1-1` | In all cases, the Label field of the advertised NLRI ([RFC8277] [RFC4364]) MUST be set to the local/incoming label programmed in the MPLS data plane for the given advertised prefix. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** BuildLabeledUnicastNLRIBytes writes the label taken from the route configuration into the NLRI (internal/component/bgp/message/update_build_labeled.go:278, fed by internal/component/bgp/reactor/peer_static_routes.go:86) and nothing programs a matching incoming MPLS entry -- the only emitters of MPLS ingress/transit entries are LDP and RSVP-TE (internal/plugins/ldp/fib.go:135, internal/plugins/rsvpte/fib.go:50) |
| `RFC8669-8-1` | The propagation to other ASes MUST be explicitly configured (§8) | MUST | 8 | **positive:** `unit/verify` [`TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_announce_rail_test.go#L49). **positive:** `unit/verify` [`TestPrefixSIDEgressBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_test.go#L155). **positive:** `unit/verify` [`TestPrefixSIDOriginationBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_test.go#L283). **positive:** `unit/verify` [`TestStaleReadvertiseKeepsPrefixSIDInsideTheSRDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_readvertise_rail_test.go#L141). **negative:** `unit/verify` [`TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_announce_rail_test.go#L46). **negative:** `unit/verify` [`TestPrefixSIDEgressBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_test.go#L156). **negative:** `unit/verify` [`TestPrefixSIDOriginationBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_test.go#L284). **negative:** `unit/verify` [`TestStaleReadvertiseKeepsPrefixSIDInsideTheSRDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_readvertise_rail_test.go#L138). **positive:** `functional/verify` [`prefixsid-announce-rail-boundary.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-announce-rail-boundary.ci#L7). **positive:** `functional/verify` [`prefixsid-ebgp-egress-boundary.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-ebgp-egress-boundary.ci#L7). **negative:** `functional/verify` [`prefixsid-announce-rail-boundary.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-announce-rail-boundary.ci#L4). **negative:** `functional/verify` [`prefixsid-ebgp-egress-boundary.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-ebgp-egress-boundary.ci#L4) |
| `RFC8669-6-1` | When a BGP speaker receives a BGP UPDATE message containing a malformed or invalid BGP Prefix-SID attribute attached to an IPv4/ IPv6 Labeled Unicast prefix ([RFC8277]), it MUST ignore the received BGP Prefix-SID attribute and not advertise it to other BGP peers. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8669WellFormedAttributeAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L187). **negative:** `unit/verify` [`TestRFC8669MalformedAttributeDiscardedAndNotAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L208). **negative:** `unit/verify` [`TestRFC8669TrailingBytesDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L238) |
| `RFC8669-6-2` | As per [RFC7606], if the BGP Prefix-SID attribute appears more than once in an UPDATE message, all the occurrences of the attribute other than the first one SHALL be discarded and the UPDATE message will continue to be processed. (§6) | SHALL | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the processing half holds -- an already-seen non-MP attribute code is skipped without validation (internal/component/bgp/message/rfc7606.go:283) -- but nothing removes the duplicate from the bytes forwarded on. A valid first occurrence with a duplicate produces no DiscardEntry, and ApplyAttrDiscard returns the path attributes untouched when the entry list is empty (internal/component/bgp/message/attr_discard.go:73-75), so the second copy is re-advertised. When the FIRST occurrence is the malformed one, applyInPlace tombstones it through AttrFind, which returns only the first match (internal/component/bgp/message/attr_discard.go:111, internal/core/bgp/attribute/iterator.go:155), leaving the untouched duplicate on the wire |
| `RFC8669-6-3` | Similarly, if a recognized TLV appears more than once in a BGP Prefix-SID attribute while the specification only allows for a single occurrence, then all the occurrences of the TLV other than the first one SHALL be discarded and the Prefix-SID attribute will continue to be processed. (§6) | SHALL | 6 | **positive:** `unit/verify` [`TestRFC8669DuplicateRecognizedTLVFirstWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L149). **negative:** `unit/verify` [`TestRFC8669DuplicateRecognizedTLVCannotOverrideFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L165) |
| `RFC8669-4-2` | A BGP speaker SHOULD log an error for further analysis when discarding an attribute. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-4.1-9` | If multiple valid paths for the same prefix are received from multiple BGP speakers or, in the case of [RFC7911], from the same BGP speaker, and the BGP Prefix-SID attributes do not contain the same label index, then the label index from the best path BGP Prefix-SID attribute SHOULD be chosen with a notable exception being when [RFC5004] is being used to dampen route changes. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-4.1-10` | When a BGP speaker receives a path from a neighbor with an "acceptable" BGP Prefix-SID attribute and that path is selected as the best path, it SHOULD program the derived label as the label for the prefix in its local MPLS data plane. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-4.1-11` | In the case of a "conflicting" BGP Prefix-SID attribute, a BGP speaker SHOULD NOT treat it as an error (§4.1) | SHOULD NOT | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-4.1-12` | In the case of a "conflicting" BGP Prefix-SID attribute, a BGP speaker SHOULD NOT treat it as an error and SHOULD propagate the attribute unchanged. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-4.1-13` | A BGP speaker SHOULD log a warning for further analysis, i.e., in the case the conflict is not due to a label-index transition. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-4.1-14` | When a BGP Prefix-SID attribute changes and transitions from "conflicting" to "acceptable", the BGP Prefix-SID attributes for other prefixes may also transition to "acceptable" as well. Implementations SHOULD ensure all impacted prefixes revert to using the label indices corresponding to these newly "acceptable" BGP Prefix-SID attributes. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-5-1` | A BGP speaker that advertises a path received from one of its neighbors SHOULD advertise the BGP Prefix-SID received with the path without modification as long as the BGP Prefix-SID was acceptable. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-5.1-2` | Since the label-index value must be unique within an SR domain, by default an implementation SHOULD NOT advertise the BGP Prefix-SID attribute outside an AS unless it is explicitly configured to do so. (§5.1) | SHOULD NOT | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-8-2` | By default, a BGP Prefix-SID attribute SHOULD NOT be attached to a prefix and advertised. (§8) | SHOULD NOT | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-8-3` | Hence, BGP Prefix-SID Advertisement SHOULD require explicit enablement. (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-5-2` | In order to prevent distribution of the BGP Prefix-SID attribute beyond its intended scope of applicability, attribute filtering SHOULD be deployed to remove the BGP Prefix-SID attribute at the administrative boundary of the SR domain. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-3.2-5` | If a BGP speaker receives a node's SRGB as an attribute of the BGP-LS Node NLRI and the BGP speaker also receives the same node's SRGB in a BGP Prefix-SID attribute, then the received values should be the same. If the values are different, the values advertised in the BGP- LS NLRI SHOULD be preferred, and an error should be logged. (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-9-1` | Since BGP-LS is the preferred method for advertising SRGB information, the BGP speaker SHOULD log an error if a BGP Prefix-SID attribute is received with SRGB information different from that received as an attribute of the same node's BGP-LS Node NLRI. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-9-2` | To prevent a Denial-of-Service (DoS) or Distributed-Denial-of-Service (DDoS) attack due to excessive BGP updates with an invalid or conflicting BGP Prefix-SID attribute, error log message rate limiting as well as suppression of duplicate error log messages SHOULD be deployed. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-1-1` | A BGP Prefix-SID MAY be attached to a BGP prefix (§1) | MAY | 1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-5.1-3` | A BGP speaker that originates a BGP Prefix-SID attribute MAY optionally announce the Originator SRGB TLV along with the mandatory Label-Index TLV. (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-5-3` | If the path did not come with a BGP Prefix-SID attribute, the speaker MAY attach a BGP Prefix-SID to the path if configured to do so. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-3.2-6` | Note that the SRGB field MAY appear multiple times. If the SRGB field appears multiple times, the SRGB consists of multiple ranges that are concatenated. (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8669-3.1-1`](#rfc8669-3.1-1) Label-Index TLV MUST be present in the BGP Prefix-SID attribute attached to IPv4/IPv6 Labeled Unicast prefixes (§3.1) | {gap}, no test | neither the sender nor the receiver requires TLV type 1 for labeled unicast -- validatePrefixSIDAttr accepts a Prefix-SID with no Label-Index TLV (internal/component/bgp/message/rfc7606.go:837) and BuildLabeledUnicast attaches whatever bytes the route configuration produced, including an SRv6-only attribute (internal/component/bgp/message/update_build_labeled.go:189) |
| [`RFC8669-4.1-1`](#rfc8669-4.1-1) When the BGP Prefix-SID attribute is attached to a BGP Labeled IPv4 or IPv6 Unicast [RFC8277] AFI/SAFI, it MUST contain the Label-Index TLV (§4.1) | {gap}, no test | the receive validator never scans for TLV type 1, so a labeled-unicast Prefix-SID with no Label-Index TLV is accepted as well formed (internal/component/bgp/message/rfc7606.go:837) |
| [`RFC8669-4.1-2`](#rfc8669-4.1-2) A BGP Prefix-SID attribute received without a Label-Index TLV MUST be considered to be "invalid" by the receiving speaker. (§4.1) | {gap}, no test | ze has no "invalid" state for the Prefix-SID attribute -- validatePrefixSIDAttr returns nil for any attribute whose TLVs fit the declared bounds, whatever their types (internal/component/bgp/message/rfc7606.go:858) |
| [`RFC8669-4.1-3`](#rfc8669-4.1-3) If multiple different prefixes are received with the same label index, all of the different prefixes MUST have their BGP Prefix-SID attribute considered to be "conflicting". (§4.1) | {gap}, no test | ze keeps no label-index-to-prefix reverse index and reads no label index at all -- the only reader of the Prefix-SID TLV list is the SRv6 extractor, which skips TLV type 1 (internal/component/bgp/plugins/rib/pool/srv6sid.go:47), so no conflict can be detected |
| [`RFC8669-4.1-4`](#rfc8669-4.1-4) When a BGP speaker receives a path from a neighbor with an "invalid" or "conflicting" BGP Prefix-SID attribute, or when a BGP speaker receives a path from a neighbor with a BGP Prefix-SID attribute but is unable to process it (e.g., local policy disables the functionality), it MUST ignore the BGP Prefix-SID attribute. (§4.1) | {gap}, no test | neither the invalid nor the conflicting state is computed, so the ignore action has no trigger -- the receive path's only Prefix-SID discard reasons are malformed TLV bounds (internal/component/bgp/message/rfc7606.go:842) and the EBGP boundary rule (internal/component/bgp/reactor/session_validation.go:107) |
| [`RFC8669-4.1-5`](#rfc8669-4.1-5) For the purposes of label allocation, a BGP speaker MUST assign a local (also called dynamic) label (non-SRGB) for such a prefix as per classic Multiprotocol BGP IPv4/IPv6 Labeled Unicast ([RFC8277]) operation. (§4.1) | {gap}, no test | ze allocates no local label for a BGP prefix at all -- the label programmed for a labeled-unicast route is the one carried in the received NLRI (internal/component/bgp/plugins/rib/rib_bestchange.go:900), and the only MPLS ingress allocators are LDP and RSVP-TE (internal/plugins/ldp/fib.go:135, internal/plugins/rsvpte/fib.go:45) |
| [`RFC8669-4.1-6`](#rfc8669-4.1-6) In the case of an "invalid" BGP Prefix-SID attribute, a BGP speaker MUST follow the error-handling rules specified in Section 6. (§4.1) | {gap}, no test | the Section 6 error handling exists and fires for malformed TLV bounds and trailing bytes (internal/component/bgp/message/rfc7606.go:842,:859), but the "invalid" condition that would route a missing-Label-Index attribute into it is never computed (internal/component/bgp/message/rfc7606.go:837) |
| [`RFC8669-4.1-7`](#rfc8669-4.1-7) Specifically, a BGP speaker receiving a prefix with a BGP Prefix-SID attribute and a label NLRI field of Implicit NULL [RFC3032] from a neighbor MUST adhere to standard behavior and program its MPLS data plane to pop the top label when forwarding traffic to the prefix. (§4.1) | {gap}, no test | the received NLRI label is programmed as an MPLS encapsulation without any implicit-NULL exception -- validateMPLSLabels accepts label 3 like any other value (internal/plugins/fib/kernel/mpls.go:23) and buildMPLSEncap pushes whatever labels it is given (internal/plugins/fib/kernel/nexthop_linux.go:75), so an implicit-NULL labeled-unicast route is programmed as a push of label 3 rather than a pop |
| [`RFC8669-5.1-1`](#rfc8669-5.1-1) In all cases, the Label field of the advertised NLRI ([RFC8277] [RFC4364]) MUST be set to the local/incoming label programmed in the MPLS data plane for the given advertised prefix. (§5.1) | {gap}, no test | BuildLabeledUnicastNLRIBytes writes the label taken from the route configuration into the NLRI (internal/component/bgp/message/update_build_labeled.go:278, fed by internal/component/bgp/reactor/peer_static_routes.go:86) and nothing programs a matching incoming MPLS entry -- the only emitters of MPLS ingress/transit entries are LDP and RSVP-TE (internal/plugins/ldp/fib.go:135, internal/plugins/rsvpte/fib.go:50) |
| [`RFC8669-6-2`](#rfc8669-6-2) As per [RFC7606], if the BGP Prefix-SID attribute appears more than once in an UPDATE message, all the occurrences of the attribute other than the first one SHALL be discarded and the UPDATE message will continue to be processed. (§6) | {gap}, no test | the processing half holds -- an already-seen non-MP attribute code is skipped without validation (internal/component/bgp/message/rfc7606.go:283) -- but nothing removes the duplicate from the bytes forwarded on. A valid first occurrence with a duplicate produces no DiscardEntry, and ApplyAttrDiscard returns the path attributes untouched when the entry list is empty (internal/component/bgp/message/attr_discard.go:73-75), so the second copy is re-advertised. When the FIRST occurrence is the malformed one, applyInPlace tombstones it through AttrFind, which returns only the first match (internal/component/bgp/message/attr_discard.go:111, internal/core/bgp/attribute/iterator.go:155), leaving the untouched duplicate on the wire |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8669-3-1`](#rfc8669-3-1)

For future extensibility, unknown TLVs MUST be ignored and propagated unmodified. (§3, §6)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clause 'ignored': a validator rejecting an unknown TLV type goes red at message/rfc8669_test.go::TestRFC8669UnknownTLVIsIgnored require.Equal(RFC7606ActionNone). Clause 'propagated unmodified': the only byte-equality assertion runs after ValidateUpdateRFC7606, not after any forward or re-advertise path, so a forward rail that strips or rewrites the unknown TLV (or drops the whole attribute, as applyFactsNextHop does on a next-hop change) turns nothing red. The propagation half has no assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8669UnknownTLVIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L80) | unit/verify | unproven |

### [`RFC8669-3.1-1`](#rfc8669-3.1-1)

Label-Index TLV MUST be present in the BGP Prefix-SID attribute attached to IPv4/IPv6 Labeled Unicast prefixes (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-3.1-1, so no unit is bound to it.

### [`RFC8669-3.1-2`](#rfc8669-3.1-2)

It MUST be ignored when received for other BGP AFI/SAFI combinations. (§3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA33 strict re-audit: weak (agrees with blind reader). The unit proves only that the SRv6 extractor steps over TLV type 1. The {single-polarity} premise that no other reader of TLV type 1 exists in internal/core/bgp is false: attribute/prefixsid_wire.go validatePrefixSIDTLVs refuses a Label-Index whose length is not 7, and the JSON formatter emits sr-label-index, both regardless of AFI/SAFI. No tagged unit feeds a Label-Index on a non-labeled-unicast route through validation or decode, so acting on it there (erroring or reporting it) stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8669LabelIndexIgnoredOnNonLabeledUnicastFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L98) | unit/verify | unproven |

### [`RFC8669-3.1-3`](#rfc8669-3.1-3)

RESERVED: 8-bit field. It MUST be clear on transmission (§3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA33 strict re-audit: weak. Forbidden: non-zero Label-Index Reserved emitted. There are two Label-Index encoders: EncodePrefixSID's simple path (asserted sid[3]==0) and parsePrefixSIDWithSRGB (prefixsid.go:120), which builds its own Label-Index TLV. TestRFC8669OriginatorSRGBTLVFlagsClearOnTransmission inspects only sid[10:], so a non-zero Reserved in the SRGB path's Label-Index stays green. The {single-polarity} note calls the encoder ParsePrefixSID and names one encoder.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8669LabelIndexTLVReservedAndFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L30) | unit/verify | unproven |

### [`RFC8669-3.1-4`](#rfc8669-3.1-4)

RESERVED: 8-bit field. It MUST be clear on transmission and MUST be ignored on reception. (§3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA33 strict re-audit: weak. Reception clause proven: a Reserved 0xFF Label-Index yields RFC7606ActionNone with unchanged bytes. The quoted transmission clause ('MUST be clear on transmission') has no assertion in this row's tagged units, and the units tagged RFC8669-3.1-3 leave the SRGB-path Label-Index encoder unasserted (see 3.1-3).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669LabelIndexNonZeroReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L114) | unit/verify | unproven |
| positive | [`TestRFC8669LabelIndexReservedAndFlagsZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L96) | unit/verify | unproven |

### [`RFC8669-3.1-5`](#rfc8669-3.1-5)

Flags: 16 bits of flags. None are defined by this document. The Flags field MUST be clear on transmission (§3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA33 strict re-audit: weak. Forbidden: non-zero Label-Index Flags emitted. sid[4:6]=={0,0} is asserted only for EncodePrefixSID's simple Label-Index path; the Label-Index TLV that parsePrefixSIDWithSRGB (prefixsid.go:120) builds is not inspected by any tagged unit, so non-zero Flags there stay green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8669LabelIndexTLVReservedAndFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L31) | unit/verify | unproven |

### [`RFC8669-3.1-6`](#rfc8669-3.1-6)

Flags: 16 bits of flags. None are defined by this document. The Flags field MUST be clear on transmission and MUST be ignored on reception. (§3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA33 strict re-audit: weak. Reception clause proven: Flags 0xFFFF yields RFC7606ActionNone with unchanged bytes. The quoted transmission clause has no assertion in this row's tagged units, and RFC8669-3.1-5's unit misses the SRGB-path Label-Index encoder.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669LabelIndexNonZeroFlagsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L127) | unit/verify | unproven |
| positive | [`TestRFC8669LabelIndexReservedAndFlagsZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L97) | unit/verify | unproven |

### [`RFC8669-3.2-1`](#rfc8669-3.2-1)

Flags: 16 bits of flags. None are defined in this document. The Flags field MUST be clear on transmission (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Non-compliant behaviour: non-zero Originator SRGB Flags emitted. config/rfc8669_test.go::TestRFC8669OriginatorSRGBTLVFlagsClearOnTransmission asserts require.Equal([]byte{0,0}, srgb[3:5]) on EncodePrefixSID output. Single-polarity marker present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8669OriginatorSRGBTLVFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L55) | unit/verify | unproven |

### [`RFC8669-3.2-2`](#rfc8669-3.2-2)

Flags: 16 bits of flags. None are defined in this document. The Flags field MUST be clear on transmission and MUST be ignored on reception. (§3.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA33 strict re-audit: weak. Reception clause: SRGB Flags 0xFFFF yields RFC7606ActionNone (no byte-unchanged assertion, unlike 3.1-4/3.1-6). The quoted transmission clause ('MUST be clear on transmission') has no assertion in this row's tagged units; it is proven only by the unit tagged RFC8669-3.2-1.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669SRGBNonZeroFlagsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L151) | unit/verify | unproven |
| positive | [`TestRFC8669SRGBFlagsZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L140) | unit/verify | unproven |

### [`RFC8669-3.2-3`](#rfc8669-3.2-3)

Originator SRGB TLV MUST NOT be changed during the propagation of the BGP update (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8669SRGBUnchangedThroughReceiveValidation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L170) | unit/verify | unproven |

### [`RFC8669-3.2-4`](#rfc8669-3.2-4)

The Originator SRGB TLV may only appear in a BGP Prefix-SID attribute attached to IPv4/IPv6 Labeled Unicast prefixes ([RFC8277]). It MUST be ignored when received for other BGP AFI/SAFI combinations. (§3.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA33 strict re-audit: weak. The unit proves only that the SRv6 extractor steps over TLV type 3. The {single-polarity} premise that the only SRGB code is the config-side encoder is false: attribute/prefixsid_wire.go validatePrefixSIDTLVs refuses an Originator SRGB whose length is not 2 + 6N, and the JSON formatter decodes it, regardless of AFI/SAFI. No tagged unit covers an SRGB on a non-labeled-unicast route through those paths.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8669SRGBIgnoredOnNonLabeledUnicastFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L125) | unit/verify | unproven |

### [`RFC8669-4-1`](#rfc8669-4-1)

A BGP speaker receiving a BGP Prefix-SID attribute from an External BGP (EBGP) neighbor residing outside the boundaries of the SR domain MUST discard the attribute unless it is configured to accept the attribute from the EBGP neighbor. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clause 'MUST discard': a Prefix-SID kept from an unconfigured EBGP peer goes red at reactor/rfc8669_test.go::TestRFC8669PrefixSIDFromEBGPDiscardedByDefault assert.False(found) after enforceRFC7606, and for repeated copies at rfc8669_multi_test.go::TestRFC8669PrefixSIDEveryOccurrenceDiscardedFromEBGP. Clause 'unless configured': an over-firing strip goes red at TestRFC8669PrefixSIDFromEBGPAcceptedWhenConfigured assert.True(found); IBGP not affected is pinned in TestRFC8669PrefixSIDKeptPathsKeepExactlyOneCopy.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669PrefixSIDEveryOccurrenceDiscardedFromEBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_multi_test.go#L67) | unit/verify | unproven |
| negative | [`TestRFC8669PrefixSIDFromEBGPDiscardedByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_test.go#L80) | unit/verify | unproven |
| positive | [`TestRFC8669PrefixSIDKeptPathsKeepExactlyOneCopy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_multi_test.go#L120) | unit/verify | unproven |
| positive | [`TestRFC8669PrefixSIDFromEBGPAcceptedWhenConfigured`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_test.go#L58) | unit/verify | unproven |

### [`RFC8669-4.1-1`](#rfc8669-4.1-1)

When the BGP Prefix-SID attribute is attached to a BGP Labeled IPv4 or IPv6 Unicast [RFC8277] AFI/SAFI, it MUST contain the Label-Index TLV (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-4.1-1, so no unit is bound to it.

### [`RFC8669-4.1-2`](#rfc8669-4.1-2)

A BGP Prefix-SID attribute received without a Label-Index TLV MUST be considered to be "invalid" by the receiving speaker. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-4.1-2, so no unit is bound to it.

### [`RFC8669-4.1-3`](#rfc8669-4.1-3)

If multiple different prefixes are received with the same label index, all of the different prefixes MUST have their BGP Prefix-SID attribute considered to be "conflicting". (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-4.1-3, so no unit is bound to it.

### [`RFC8669-4.1-4`](#rfc8669-4.1-4)

When a BGP speaker receives a path from a neighbor with an "invalid" or "conflicting" BGP Prefix-SID attribute, or when a BGP speaker receives a path from a neighbor with a BGP Prefix-SID attribute but is unable to process it (e.g., local policy disables the functionality), it MUST ignore the BGP Prefix-SID attribute. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-4.1-4, so no unit is bound to it.

### [`RFC8669-4.1-5`](#rfc8669-4.1-5)

For the purposes of label allocation, a BGP speaker MUST assign a local (also called dynamic) label (non-SRGB) for such a prefix as per classic Multiprotocol BGP IPv4/IPv6 Labeled Unicast ([RFC8277]) operation. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-4.1-5, so no unit is bound to it.

### [`RFC8669-4.1-6`](#rfc8669-4.1-6)

In the case of an "invalid" BGP Prefix-SID attribute, a BGP speaker MUST follow the error-handling rules specified in Section 6. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-4.1-6, so no unit is bound to it.

### [`RFC8669-4.1-7`](#rfc8669-4.1-7)

Specifically, a BGP speaker receiving a prefix with a BGP Prefix-SID attribute and a label NLRI field of Implicit NULL [RFC3032] from a neighbor MUST adhere to standard behavior and program its MPLS data plane to pop the top label when forwarding traffic to the prefix. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-4.1-7, so no unit is bound to it.

### [`RFC8669-4.1-8`](#rfc8669-4.1-8)

The label NLRI defines the outbound label that MUST be used by the receiving node (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8669NLRILabelIsTheOutboundLabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L33) | unit/verify | unproven |

### [`RFC8669-5.1-1`](#rfc8669-5.1-1)

In all cases, the Label field of the advertised NLRI ([RFC8277] [RFC4364]) MUST be set to the local/incoming label programmed in the MPLS data plane for the given advertised prefix. (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-5.1-1, so no unit is bound to it.

### [`RFC8669-8-1`](#rfc8669-8-1)

The propagation to other ASes MUST be explicitly configured (§8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_announce_rail_test.go#L46) | unit/verify | revert, verified |
| negative | [`TestStaleReadvertiseKeepsPrefixSIDInsideTheSRDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_readvertise_rail_test.go#L138) | unit/verify | revert, verified |
| negative | [`TestPrefixSIDEgressBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_test.go#L156) | unit/verify | revert, verified |
| negative | [`TestPrefixSIDOriginationBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_test.go#L284) | unit/verify | revert, verified |
| negative | [`prefixsid-announce-rail-boundary.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-announce-rail-boundary.ci#L4) | functional/verify | revert, verified |
| negative | [`prefixsid-ebgp-egress-boundary.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-ebgp-egress-boundary.ci#L4) | functional/verify | revert, verified |
| positive | [`TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_announce_rail_test.go#L49) | unit/verify | revert, verified |
| positive | [`TestStaleReadvertiseKeepsPrefixSIDInsideTheSRDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_readvertise_rail_test.go#L141) | unit/verify | revert, verified |
| positive | [`TestPrefixSIDEgressBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_test.go#L155) | unit/verify | revert, verified |
| positive | [`TestPrefixSIDOriginationBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_test.go#L283) | unit/verify | revert, verified |
| positive | [`prefixsid-announce-rail-boundary.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-announce-rail-boundary.ci#L7) | functional/verify | revert, verified |
| positive | [`prefixsid-ebgp-egress-boundary.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-ebgp-egress-boundary.ci#L7) | functional/verify | revert, verified |

### [`RFC8669-6-1`](#rfc8669-6-1)

When a BGP speaker receives a BGP UPDATE message containing a malformed or invalid BGP Prefix-SID attribute attached to an IPv4/ IPv6 Labeled Unicast prefix ([RFC8277]), it MUST ignore the received BGP Prefix-SID attribute and not advertise it to other BGP peers. (§6)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Malformed via TLV overrun: message/rfc8669_test.go::TestRFC8669MalformedAttributeDiscardedAndNotAdvertised asserts discard and absence after ApplyAttrDiscard; trailing bytes: TestRFC8669TrailingBytesDiscarded asserts discard only, not removal. Unasserted clauses: (1) 'invalid' (no Label-Index TLV, per 4.1) is never computed, so an invalid attribute is kept and advertised and no tagged unit goes red; (2) 'a TLV length that doesn't conform to the length constraints for the TLV' is not checked by validatePrefixSIDAttr (a Label-Index TLV of length 6 or an SRGB TLV of length 3 is accepted), and no test drives it; (3) 'not meeting the minimum attribute length' (e.g. a zero-length attribute) is accepted and untested.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669MalformedAttributeDiscardedAndNotAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L208) | unit/verify | unproven |
| negative | [`TestRFC8669TrailingBytesDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L238) | unit/verify | unproven |
| positive | [`TestRFC8669WellFormedAttributeAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L187) | unit/verify | unproven |

### [`RFC8669-6-2`](#rfc8669-6-2)

As per [RFC7606], if the BGP Prefix-SID attribute appears more than once in an UPDATE message, all the occurrences of the attribute other than the first one SHALL be discarded and the UPDATE message will continue to be processed. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-6-2, so no unit is bound to it.

### [`RFC8669-6-3`](#rfc8669-6-3)

Similarly, if a recognized TLV appears more than once in a BGP Prefix-SID attribute while the specification only allows for a single occurrence, then all the occurrences of the TLV other than the first one SHALL be discarded and the Prefix-SID attribute will continue to be processed. (§6)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Tagged units pool/rfc8669_test.go::TestRFC8669DuplicateRecognizedTLVFirstWins and ::TestRFC8669DuplicateRecognizedTLVCannotOverrideFirst prove first-wins reading of duplicate SRv6 L3 Service TLVs in ExtractSRv6SIDFull, which is the RFC 9252 rule 'all but the first instance MUST be ignored'. Unasserted: that the later occurrences are DISCARDED (they stay in the attribute bytes forwarded on, same shape as the RFC8669-6-2 gap), and duplicates of RFC 8669's own single-occurrence TLV (Label-Index) are never driven.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669DuplicateRecognizedTLVCannotOverrideFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L165) | unit/verify | unproven |
| positive | [`TestRFC8669DuplicateRecognizedTLVFirstWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L149) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc8669.txt |
| Source fingerprint | bf04f41e726679f9 |
| Record | rfc/extraction/rfc8669.json |
| Mapped sentences | 23 |
| Declined as scope | 8 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 3 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `3.1` | not stated | 4 | walked | not stated |
| `3.2` | not stated | 4 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `4.1` | not stated | 9 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 2 | walked | not stated |
| `6` | not stated | 4 | walked | not stated |
| `7` | not stated | 1 | walked | not stated |
| `8` | not stated | 1 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `10.1` | not stated | 0 | walked | not stated |
| `10.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust boilerplate on the Simplified BSD License for extracted Code Components; it binds republication of the document, not any protocol behaviour. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Overview prose in Section 2: 'if traffic engineering ... is required' and 'may also be required' are lowercase and describe what an SR domain deployment needs, not an obligation on a BGP speaker. | If traffic engineering within the SR domain is required, each node may also be required to advertise topological information and Peer SIDs for each of its links and peers. |
| `2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Overview prose: 'This information is required to perform the explicit path computation' is lowercase and describes why the information exists. | This information is required to perform the explicit path computation and to express an explicit path as a list of SIDs. |
| `2:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Overview prose: 'knowledge of the prefix originator's SRGB is required in order to compute the local label' is lowercase and motivates the Originator SRGB TLV. | If a prefix segment is to be included in an MPLS label stack, e.g., for traffic-engineering purposes, knowledge of the prefix originator's SRGB is required in order to compute the local label used by the originator. |
| `3.2:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | 'Since the Label-Index TLV is required for IPv4/IPv6 prefix applicability' is lowercase and restates RFC8669-3.1-1; the rest of the sentence states the consequence that Section 6 error handling (RFC8669-6-1) already carries. | Since the Label-Index TLV is required for IPv4/IPv6 prefix applicability, the Originator SRGB TLV will be ignored if it is not specified in a manner consistent with Section 6. |
| `4.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | 'A BGP session supporting the Multiprotocol BGP Labeled IPv4 or IPv6 Unicast AFI/SAFI is required' is lowercase and states the deployment precondition for the section, not a behaviour a speaker performs. | A BGP session supporting the Multiprotocol BGP Labeled IPv4 or IPv6 Unicast ([RFC8277]) AFI/SAFI is required. |
| `6:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6 repeats the Section 3 sentence word for word: 'For future extensibility, unknown TLVs MUST be ignored and propagated unmodified.' Site 3:1 is the source and RFC8669-3-1 already cites both sections. | For future extensibility, unknown TLVs MUST be ignored and propagated unmodified. |
| `7:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IANA Considerations: 'The designated experts must be good and faithful stewards' is lowercase and addresses the registry's designated experts about how they review requests, not a BGP speaker. | The designated experts must be good and faithful stewards of the above registries, ensuring that each request is legitimate and corresponds to a viable use case. |

## Superseded

No document obsoletes RFC 8669, so its obligations are stated where they were written.
