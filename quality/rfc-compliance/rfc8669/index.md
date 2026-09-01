# RFC 8669 - Segment Routing Prefix Segment Identifier Extensions for BGP

Partial. Every requirement this repository extracted from RFC 8669, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 28.0% | 7 of 25 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 32.0% | 8 of 25 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 25 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 29 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 25 | of 44 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 25 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 40.0% | 10 of 25 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 25 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 44 |
| Gated MUST-level | 25 |
| Obligations that bind Ze | 25 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 10 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 29 |
| Tagged units | 29 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8669.md` |
| Requirement shard | `rfc/requirements/rfc8669.md` |
| RFC text | `rfc/full/rfc8669.txt` |

## Enrolment

Enrolled: SR Prefix Segment Identifier Extensions for BGP

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Attribute code 40 is registered as optional transitive ([`internal/core/bgp/attribute/attribute.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/attribute.go)) and carried end to end: the Label-Index TLV (type 1) and the Originator SRGB TLV (type 3) are encoded from route configuration with their Reserved and Flags octets cleared ([`internal/component/bgp/config/routeattr_prefixsid.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_prefixsid.go),:129) and attached to labeled-unicast and VPN UPDATEs ([`internal/component/bgp/message/update_build_labeled.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_labeled.go))
- on reception every TLV is bounds-checked with Section 6 attribute-discard for an overrunning TLV length or trailing bytes ([`internal/component/bgp/message/rfc7606.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606.go)), a duplicate attribute is discarded unexamined in favor of the first for PROCESSING purposes ([`internal/component/bgp/message/rfc7606.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606.go)), a duplicate recognized Service TLV never displaces the first ([`internal/component/bgp/plugins/rib/pool/srv6sid.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid.go)), the Section 4 EBGP boundary discards the attribute unless the peer is configured to accept it ([`internal/component/bgp/reactor/session_validation.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation.go)), the Section 8 boundary removes it on egress from every UPDATE sent to an EBGP peer the operator has not configured for propagation, on both forward rails and on the origination rails ([`internal/component/bgp/reactor/forward_prefix_sid.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid.go)), unknown TLVs and the Reserved/Flags fields are ignored and left byte-identical for propagation, and the label carried in a received labeled-unicast NLRI is the outbound label programmed toward the next hop ([`internal/core/bgp/nlri/nlrisplit/labeled.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/labeled.go) to [`internal/plugins/fib/kernel/nexthop_linux.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/nexthop_linux.go)). Requirements bound per line in [`rfc/short/rfc8669.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8669.md).


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
| **Gated MUST-level requirements** | **25** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (7):** [`RFC8669-3.1-4`](#rfc8669-3.1-4), [`RFC8669-3.1-6`](#rfc8669-3.1-6), [`RFC8669-3.2-2`](#rfc8669-3.2-2), [`RFC8669-4-1`](#rfc8669-4-1), [`RFC8669-8-1`](#rfc8669-8-1), [`RFC8669-6-1`](#rfc8669-6-1), [`RFC8669-6-3`](#rfc8669-6-3)

**Annotated instead of tested (18):** [`RFC8669-3-1`](#rfc8669-3-1), [`RFC8669-3.1-1`](#rfc8669-3.1-1), [`RFC8669-3.1-2`](#rfc8669-3.1-2), [`RFC8669-3.1-3`](#rfc8669-3.1-3), [`RFC8669-3.1-5`](#rfc8669-3.1-5), [`RFC8669-3.2-1`](#rfc8669-3.2-1), [`RFC8669-3.2-3`](#rfc8669-3.2-3), [`RFC8669-3.2-4`](#rfc8669-3.2-4), [`RFC8669-4.1-1`](#rfc8669-4.1-1), [`RFC8669-4.1-2`](#rfc8669-4.1-2), [`RFC8669-4.1-3`](#rfc8669-4.1-3), [`RFC8669-4.1-4`](#rfc8669-4.1-4), [`RFC8669-4.1-5`](#rfc8669-4.1-5), [`RFC8669-4.1-6`](#rfc8669-4.1-6), [`RFC8669-4.1-7`](#rfc8669-4.1-7), [`RFC8669-4.1-8`](#rfc8669-4.1-8), [`RFC8669-5.1-1`](#rfc8669-5.1-1), [`RFC8669-6-2`](#rfc8669-6-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8669-3-1` | Unknown TLVs MUST be ignored and propagated unmodified (§3, §6) | MUST | 3 | **positive:** `unit/verify` [`TestRFC8669UnknownTLVIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L80). **negative:** no negative test. **{single-polarity}:** ze has no producer that rewrites a Prefix-SID TLV. validatePrefixSIDAttr walks TLV headers and reads the value of types 5 and 6 only, never writing any of them (internal/component/bgp/message/rfc7606.go:840-856), and no forward path edits the attribute value. Ze's one code-40 modification is coarser than this requirement rather than a counter-example to it: applyFactsNextHop drops the WHOLE attribute on every next-hop-changing readvertisement (internal/component/bgp/reactor/peer_forward_facts.go:241), taking any unknown TLV with it, so on that rail the attribute is not propagated at all. Where the attribute IS propagated its bytes are untouched, and no input can make a TLV come out modified, so there is nothing to drive negatively |
| `RFC8669-3.1-1` | Label-Index TLV MUST be present in the BGP Prefix-SID attribute attached to IPv4/IPv6 Labeled Unicast prefixes (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** neither the sender nor the receiver requires TLV type 1 for labeled unicast -- validatePrefixSIDAttr accepts a Prefix-SID with no Label-Index TLV (internal/component/bgp/message/rfc7606.go:837) and BuildLabeledUnicast attaches whatever bytes the route configuration produced, including an SRv6-only attribute (internal/component/bgp/message/update_build_labeled.go:189) |
| `RFC8669-3.1-2` | Label-Index TLV MUST be ignored when received for other BGP AFI/SAFI combinations (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8669LabelIndexIgnoredOnNonLabeledUnicastFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L98). **negative:** no negative test. **{single-polarity}:** ze has no receive-side Label-Index consumer for any family -- ExtractSRv6SIDFull steps over TLV type 1 by length (internal/component/bgp/plugins/rib/pool/srv6sid.go:47) and no other reader of TLV type 1 exists in internal/component/bgp or internal/core/bgp -- so the TLV is ignored on every AFI/SAFI and there is no label-index-driven behavior to drive negatively |
| `RFC8669-3.1-3` | Label-Index TLV Reserved field MUST be clear on transmission (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8669LabelIndexTLVReservedAndFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L26). **negative:** no negative test. **{single-polarity}:** ParsePrefixSID hardcodes the Reserved octet to 0 on encode and no code path emits a non-zero value, so there is no negative input to reject (internal/component/bgp/config/routeattr_prefixsid.go:68) |
| `RFC8669-3.1-4` | Label-Index TLV Reserved field MUST be ignored on reception (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8669LabelIndexReservedAndFlagsZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L96). **negative:** `unit/verify` [`TestRFC8669LabelIndexNonZeroReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L114) |
| `RFC8669-3.1-5` | Label-Index TLV Flags MUST be clear on transmission (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8669LabelIndexTLVReservedAndFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L27). **negative:** no negative test. **{single-polarity}:** ParsePrefixSID hardcodes the Flags field to 0 on encode with no non-zero path to reject (internal/component/bgp/config/routeattr_prefixsid.go:69) |
| `RFC8669-3.1-6` | Label-Index TLV Flags MUST be ignored on reception (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8669LabelIndexReservedAndFlagsZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L97). **negative:** `unit/verify` [`TestRFC8669LabelIndexNonZeroFlagsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L127) |
| `RFC8669-3.2-1` | Originator SRGB TLV Flags MUST be clear on transmission (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8669OriginatorSRGBTLVFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L51). **negative:** no negative test. **{single-polarity}:** parsePrefixSIDWithSRGB hardcodes both Flags octets to 0 on encode with no non-zero path to reject (internal/component/bgp/config/routeattr_prefixsid.go:133) |
| `RFC8669-3.2-2` | Originator SRGB TLV Flags MUST be ignored on reception (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8669SRGBFlagsZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L140). **negative:** `unit/verify` [`TestRFC8669SRGBNonZeroFlagsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L151) |
| `RFC8669-3.2-3` | Originator SRGB TLV MUST NOT be changed during the propagation of the BGP update (§3.2) | MUST NOT | 3.2 | **positive:** `unit/verify` [`TestRFC8669SRGBUnchangedThroughReceiveValidation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L170). **negative:** no negative test. **{single-polarity}:** no producer writes into an Originator SRGB TLV. The receive validator reads TLV headers and never writes a value (internal/component/bgp/message/rfc7606.go:837-856). The forward path's only code-40 operation is a whole-attribute suppress on a next-hop change (internal/component/bgp/reactor/peer_forward_facts.go:241) -- that removes the Originator SRGB along with everything else in the attribute rather than changing it, so it is not a counter-example to "MUST NOT be changed" but it does mean the TLV survives propagation only on the rails that keep the attribute. On those rails the SRGB octets are byte-identical, and no input produces changed SRGB bytes to drive negatively |
| `RFC8669-3.2-4` | Originator SRGB TLV MUST be ignored when received for non-Labeled Unicast AFI/SAFI combinations (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8669SRGBIgnoredOnNonLabeledUnicastFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L125). **negative:** no negative test. **{single-polarity}:** ze has no receive-side Originator SRGB consumer for any family -- ExtractSRv6SIDFull steps over TLV type 3 by length (internal/component/bgp/plugins/rib/pool/srv6sid.go:47) and the only SRGB code in internal/component/bgp is the config-side encoder -- so the TLV is ignored on every AFI/SAFI and there is no SRGB-driven behavior to drive negatively |
| `RFC8669-4-1` | A BGP speaker receiving a BGP Prefix-SID attribute from an EBGP neighbor outside the SR domain MUST discard the attribute unless configured to accept (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC8669PrefixSIDFromEBGPAcceptedWhenConfigured`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_test.go#L58). **positive:** `unit/verify` [`TestRFC8669PrefixSIDKeptPathsKeepExactlyOneCopy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_multi_test.go#L120). **negative:** `unit/verify` [`TestRFC8669PrefixSIDEveryOccurrenceDiscardedFromEBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_multi_test.go#L67). **negative:** `unit/verify` [`TestRFC8669PrefixSIDFromEBGPDiscardedByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_test.go#L80) |
| `RFC8669-4.1-1` | BGP Prefix-SID attribute attached to Labeled Unicast MUST contain the Label-Index TLV (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the receive validator never scans for TLV type 1, so a labeled-unicast Prefix-SID with no Label-Index TLV is accepted as well formed (internal/component/bgp/message/rfc7606.go:837) |
| `RFC8669-4.1-2` | A BGP Prefix-SID attribute received without a Label-Index TLV MUST be considered "invalid" (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no "invalid" state for the Prefix-SID attribute -- validatePrefixSIDAttr returns nil for any attribute whose TLVs fit the declared bounds, whatever their types (internal/component/bgp/message/rfc7606.go:858) |
| `RFC8669-4.1-3` | If multiple different prefixes are received with the same label index, all MUST have their BGP Prefix-SID attribute considered "conflicting" (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze keeps no label-index-to-prefix reverse index and reads no label index at all -- the only reader of the Prefix-SID TLV list is the SRv6 extractor, which skips TLV type 1 (internal/component/bgp/plugins/rib/pool/srv6sid.go:47), so no conflict can be detected |
| `RFC8669-4.1-4` | When receiving "invalid" or "conflicting" BGP Prefix-SID attribute, speaker MUST ignore the attribute (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** neither the invalid nor the conflicting state is computed, so the ignore action has no trigger -- the receive path's only Prefix-SID discard reasons are malformed TLV bounds (internal/component/bgp/message/rfc7606.go:842) and the EBGP boundary rule (internal/component/bgp/reactor/session_validation.go:107) |
| `RFC8669-4.1-5` | Speaker MUST assign a local (dynamic) label (non-SRGB) for prefixes with invalid/conflicting Prefix-SID (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze allocates no local label for a BGP prefix at all -- the label programmed for a labeled-unicast route is the one carried in the received NLRI (internal/component/bgp/plugins/rib/rib_bestchange.go:900), and the only MPLS ingress allocators are LDP and RSVP-TE (internal/plugins/ldp/fib.go:135, internal/plugins/rsvpte/fib.go:45) |
| `RFC8669-4.1-6` | For "invalid" BGP Prefix-SID attribute, speaker MUST follow the error-handling rules specified in Section 6 (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Section 6 error handling exists and fires for malformed TLV bounds and trailing bytes (internal/component/bgp/message/rfc7606.go:842,:859), but the "invalid" condition that would route a missing-Label-Index attribute into it is never computed (internal/component/bgp/message/rfc7606.go:837) |
| `RFC8669-4.1-7` | Implicit NULL label in NLRI: speaker MUST adhere to standard behavior and program MPLS data plane to pop the top label (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the received NLRI label is programmed as an MPLS encapsulation without any implicit-NULL exception -- validateMPLSLabels accepts label 3 like any other value (internal/plugins/fib/kernel/mpls.go:23) and buildMPLSEncap pushes whatever labels it is given (internal/plugins/fib/kernel/nexthop_linux.go:75), so an implicit-NULL labeled-unicast route is programmed as a push of label 3 rather than a pop |
| `RFC8669-4.1-8` | The label NLRI defines the outbound label that MUST be used by the receiving node (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC8669NLRILabelIsTheOutboundLabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L33). **negative:** no negative test. **{single-polarity}:** the label taken off the received NLRI is carried unchanged to the forwarding plane (internal/core/bgp/nlri/nlrisplit/labeled.go:110 to internal/component/bgp/plugins/rib/rib_bestchange.go:900 to internal/plugins/fib/kernel/nexthop_linux.go:75); the rule states which label to use, not a condition to reject, so there is no non-conforming input whose rejection could be asserted |
| `RFC8669-5.1-1` | Label field of the advertised NLRI MUST be set to the local/incoming label programmed in the MPLS data plane (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** BuildLabeledUnicastNLRIBytes writes the label taken from the route configuration into the NLRI (internal/component/bgp/message/update_build_labeled.go:278, fed by internal/component/bgp/reactor/peer_static_routes.go:86) and nothing programs a matching incoming MPLS entry -- the only emitters of MPLS ingress/transit entries are LDP and RSVP-TE (internal/plugins/ldp/fib.go:135, internal/plugins/rsvpte/fib.go:50) |
| `RFC8669-8-1` | The propagation to other ASes MUST be explicitly configured (§8) | MUST | 8 | **positive:** `unit/verify` [`TestPrefixSIDEgressBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_test.go#L155). **positive:** `unit/verify` [`TestPrefixSIDOriginationBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_test.go#L283). **negative:** `unit/verify` [`TestPrefixSIDEgressBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_test.go#L156). **negative:** `unit/verify` [`TestPrefixSIDOriginationBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_test.go#L284). **positive:** `functional/verify` [`prefixsid-ebgp-egress-boundary.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-ebgp-egress-boundary.ci#L7). **negative:** `functional/verify` [`prefixsid-ebgp-egress-boundary.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-ebgp-egress-boundary.ci#L4) |
| `RFC8669-6-1` | Malformed BGP Prefix-SID attribute: MUST ignore the received attribute and not advertise it to other BGP peers (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8669WellFormedAttributeAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L187). **negative:** `unit/verify` [`TestRFC8669MalformedAttributeDiscardedAndNotAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L208). **negative:** `unit/verify` [`TestRFC8669TrailingBytesDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L238) |
| `RFC8669-6-2` | If the BGP Prefix-SID attribute appears more than once in an UPDATE, all occurrences other than the first SHALL be discarded (§6) | SHALL | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the processing half holds -- an already-seen non-MP attribute code is skipped without validation (internal/component/bgp/message/rfc7606.go:283) -- but nothing removes the duplicate from the bytes forwarded on. A valid first occurrence with a duplicate produces no DiscardEntry, and ApplyAttrDiscard returns the path attributes untouched when the entry list is empty (internal/component/bgp/message/attr_discard.go:73-75), so the second copy is re-advertised. When the FIRST occurrence is the malformed one, applyInPlace tombstones it through AttrFind, which returns only the first match (internal/component/bgp/message/attr_discard.go:111, internal/core/bgp/attribute/iterator.go:155), leaving the untouched duplicate on the wire |
| `RFC8669-6-3` | If a recognized TLV appears more than once, all occurrences other than the first SHALL be discarded (§6) | SHALL | 6 | **positive:** `unit/verify` [`TestRFC8669DuplicateRecognizedTLVFirstWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L149). **negative:** `unit/verify` [`TestRFC8669DuplicateRecognizedTLVCannotOverrideFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L165) |
| `RFC8669-4-2` | A BGP speaker SHOULD log an error when discarding an attribute (§4, §6) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-4.1-9` | The label index from the best path BGP Prefix-SID attribute SHOULD be chosen when multiple paths have different indices (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-4.1-10` | Speaker SHOULD program the derived label as the label for the prefix in its local MPLS data plane when acceptable (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-4.1-11` | Speaker SHOULD NOT treat a "conflicting" BGP Prefix-SID attribute as an error (§4.1) | SHOULD NOT | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-4.1-12` | Speaker SHOULD propagate the attribute unchanged for conflicting cases (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-4.1-13` | Speaker SHOULD log a warning for conflicting BGP Prefix-SID attributes (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-4.1-14` | Implementations SHOULD ensure all impacted prefixes revert to using label indices when transitioning from conflicting to acceptable (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-5-1` | Speaker SHOULD advertise the BGP Prefix-SID received with the path without modification (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-5.1-2` | Implementation SHOULD NOT advertise the BGP Prefix-SID attribute outside an AS unless explicitly configured (§5.1) | SHOULD NOT | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-8-2` | BGP Prefix-SID attribute SHOULD NOT be attached to a prefix and advertised by default (§8) | SHOULD NOT | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-8-3` | BGP Prefix-SID advertisement SHOULD require explicit enablement (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-5-2` | Attribute filtering SHOULD be deployed at the administrative boundary of the SR domain (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-3.2-5` | If SRGB received via BGP-LS differs from Prefix-SID attribute, BGP-LS values SHOULD be preferred (§3.2, §9) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-9-1` | Speaker SHOULD log an error if BGP Prefix-SID SRGB differs from that received via BGP-LS Node NLRI (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-9-2` | Error log message rate limiting and suppression of duplicate error log messages SHOULD be deployed (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-1-1` | A BGP Prefix-SID MAY be attached to a BGP prefix (§1) | MAY | 1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-5.1-3` | Originator may optionally announce the Originator SRGB TLV (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-5-3` | If path lacks Prefix-SID, speaker MAY attach a BGP Prefix-SID if configured (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-3.2-6` | SRGB field MAY appear multiple times (ranges concatenated) (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8669-3.1-1`](#rfc8669-3.1-1) Label-Index TLV MUST be present in the BGP Prefix-SID attribute attached to IPv4/IPv6 Labeled Unicast prefixes (§3.1) | {gap}, no test | neither the sender nor the receiver requires TLV type 1 for labeled unicast -- validatePrefixSIDAttr accepts a Prefix-SID with no Label-Index TLV (internal/component/bgp/message/rfc7606.go:837) and BuildLabeledUnicast attaches whatever bytes the route configuration produced, including an SRv6-only attribute (internal/component/bgp/message/update_build_labeled.go:189) |
| [`RFC8669-4.1-1`](#rfc8669-4.1-1) BGP Prefix-SID attribute attached to Labeled Unicast MUST contain the Label-Index TLV (§4.1) | {gap}, no test | the receive validator never scans for TLV type 1, so a labeled-unicast Prefix-SID with no Label-Index TLV is accepted as well formed (internal/component/bgp/message/rfc7606.go:837) |
| [`RFC8669-4.1-2`](#rfc8669-4.1-2) A BGP Prefix-SID attribute received without a Label-Index TLV MUST be considered "invalid" (§4.1) | {gap}, no test | ze has no "invalid" state for the Prefix-SID attribute -- validatePrefixSIDAttr returns nil for any attribute whose TLVs fit the declared bounds, whatever their types (internal/component/bgp/message/rfc7606.go:858) |
| [`RFC8669-4.1-3`](#rfc8669-4.1-3) If multiple different prefixes are received with the same label index, all MUST have their BGP Prefix-SID attribute considered "conflicting" (§4.1) | {gap}, no test | ze keeps no label-index-to-prefix reverse index and reads no label index at all -- the only reader of the Prefix-SID TLV list is the SRv6 extractor, which skips TLV type 1 (internal/component/bgp/plugins/rib/pool/srv6sid.go:47), so no conflict can be detected |
| [`RFC8669-4.1-4`](#rfc8669-4.1-4) When receiving "invalid" or "conflicting" BGP Prefix-SID attribute, speaker MUST ignore the attribute (§4.1) | {gap}, no test | neither the invalid nor the conflicting state is computed, so the ignore action has no trigger -- the receive path's only Prefix-SID discard reasons are malformed TLV bounds (internal/component/bgp/message/rfc7606.go:842) and the EBGP boundary rule (internal/component/bgp/reactor/session_validation.go:107) |
| [`RFC8669-4.1-5`](#rfc8669-4.1-5) Speaker MUST assign a local (dynamic) label (non-SRGB) for prefixes with invalid/conflicting Prefix-SID (§4.1) | {gap}, no test | ze allocates no local label for a BGP prefix at all -- the label programmed for a labeled-unicast route is the one carried in the received NLRI (internal/component/bgp/plugins/rib/rib_bestchange.go:900), and the only MPLS ingress allocators are LDP and RSVP-TE (internal/plugins/ldp/fib.go:135, internal/plugins/rsvpte/fib.go:45) |
| [`RFC8669-4.1-6`](#rfc8669-4.1-6) For "invalid" BGP Prefix-SID attribute, speaker MUST follow the error-handling rules specified in Section 6 (§4.1) | {gap}, no test | the Section 6 error handling exists and fires for malformed TLV bounds and trailing bytes (internal/component/bgp/message/rfc7606.go:842,:859), but the "invalid" condition that would route a missing-Label-Index attribute into it is never computed (internal/component/bgp/message/rfc7606.go:837) |
| [`RFC8669-4.1-7`](#rfc8669-4.1-7) Implicit NULL label in NLRI: speaker MUST adhere to standard behavior and program MPLS data plane to pop the top label (§4.1) | {gap}, no test | the received NLRI label is programmed as an MPLS encapsulation without any implicit-NULL exception -- validateMPLSLabels accepts label 3 like any other value (internal/plugins/fib/kernel/mpls.go:23) and buildMPLSEncap pushes whatever labels it is given (internal/plugins/fib/kernel/nexthop_linux.go:75), so an implicit-NULL labeled-unicast route is programmed as a push of label 3 rather than a pop |
| [`RFC8669-5.1-1`](#rfc8669-5.1-1) Label field of the advertised NLRI MUST be set to the local/incoming label programmed in the MPLS data plane (§5.1) | {gap}, no test | BuildLabeledUnicastNLRIBytes writes the label taken from the route configuration into the NLRI (internal/component/bgp/message/update_build_labeled.go:278, fed by internal/component/bgp/reactor/peer_static_routes.go:86) and nothing programs a matching incoming MPLS entry -- the only emitters of MPLS ingress/transit entries are LDP and RSVP-TE (internal/plugins/ldp/fib.go:135, internal/plugins/rsvpte/fib.go:50) |
| [`RFC8669-6-2`](#rfc8669-6-2) If the BGP Prefix-SID attribute appears more than once in an UPDATE, all occurrences other than the first SHALL be discarded (§6) | {gap}, no test | the processing half holds -- an already-seen non-MP attribute code is skipped without validation (internal/component/bgp/message/rfc7606.go:283) -- but nothing removes the duplicate from the bytes forwarded on. A valid first occurrence with a duplicate produces no DiscardEntry, and ApplyAttrDiscard returns the path attributes untouched when the entry list is empty (internal/component/bgp/message/attr_discard.go:73-75), so the second copy is re-advertised. When the FIRST occurrence is the malformed one, applyInPlace tombstones it through AttrFind, which returns only the first match (internal/component/bgp/message/attr_discard.go:111, internal/core/bgp/attribute/iterator.go:155), leaving the untouched duplicate on the wire |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8669-3-1`](#rfc8669-3-1)

Unknown TLVs MUST be ignored and propagated unmodified (§3, §6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8669UnknownTLVIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L80) | unit/verify | unproven |

### [`RFC8669-3.1-1`](#rfc8669-3.1-1)

Label-Index TLV MUST be present in the BGP Prefix-SID attribute attached to IPv4/IPv6 Labeled Unicast prefixes (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-3.1-1, so no unit is bound to it.

### [`RFC8669-3.1-2`](#rfc8669-3.1-2)

Label-Index TLV MUST be ignored when received for other BGP AFI/SAFI combinations (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8669LabelIndexIgnoredOnNonLabeledUnicastFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L98) | unit/verify | unproven |

### [`RFC8669-3.1-3`](#rfc8669-3.1-3)

Label-Index TLV Reserved field MUST be clear on transmission (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8669LabelIndexTLVReservedAndFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L26) | unit/verify | unproven |

### [`RFC8669-3.1-4`](#rfc8669-3.1-4)

Label-Index TLV Reserved field MUST be ignored on reception (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669LabelIndexNonZeroReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L114) | unit/verify | unproven |
| positive | [`TestRFC8669LabelIndexReservedAndFlagsZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L96) | unit/verify | unproven |

### [`RFC8669-3.1-5`](#rfc8669-3.1-5)

Label-Index TLV Flags MUST be clear on transmission (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8669LabelIndexTLVReservedAndFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L27) | unit/verify | unproven |

### [`RFC8669-3.1-6`](#rfc8669-3.1-6)

Label-Index TLV Flags MUST be ignored on reception (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669LabelIndexNonZeroFlagsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L127) | unit/verify | unproven |
| positive | [`TestRFC8669LabelIndexReservedAndFlagsZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L97) | unit/verify | unproven |

### [`RFC8669-3.2-1`](#rfc8669-3.2-1)

Originator SRGB TLV Flags MUST be clear on transmission (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8669OriginatorSRGBTLVFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L51) | unit/verify | unproven |

### [`RFC8669-3.2-2`](#rfc8669-3.2-2)

Originator SRGB TLV Flags MUST be ignored on reception (§3.2)

Audit verdict: not audited: no reader has judged these tests

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

Originator SRGB TLV MUST be ignored when received for non-Labeled Unicast AFI/SAFI combinations (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8669SRGBIgnoredOnNonLabeledUnicastFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L125) | unit/verify | unproven |

### [`RFC8669-4-1`](#rfc8669-4-1)

A BGP speaker receiving a BGP Prefix-SID attribute from an EBGP neighbor outside the SR domain MUST discard the attribute unless configured to accept (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669PrefixSIDEveryOccurrenceDiscardedFromEBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_multi_test.go#L67) | unit/verify | unproven |
| negative | [`TestRFC8669PrefixSIDFromEBGPDiscardedByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_test.go#L80) | unit/verify | unproven |
| positive | [`TestRFC8669PrefixSIDKeptPathsKeepExactlyOneCopy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_multi_test.go#L120) | unit/verify | unproven |
| positive | [`TestRFC8669PrefixSIDFromEBGPAcceptedWhenConfigured`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_test.go#L58) | unit/verify | unproven |

### [`RFC8669-4.1-1`](#rfc8669-4.1-1)

BGP Prefix-SID attribute attached to Labeled Unicast MUST contain the Label-Index TLV (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-4.1-1, so no unit is bound to it.

### [`RFC8669-4.1-2`](#rfc8669-4.1-2)

A BGP Prefix-SID attribute received without a Label-Index TLV MUST be considered "invalid" (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-4.1-2, so no unit is bound to it.

### [`RFC8669-4.1-3`](#rfc8669-4.1-3)

If multiple different prefixes are received with the same label index, all MUST have their BGP Prefix-SID attribute considered "conflicting" (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-4.1-3, so no unit is bound to it.

### [`RFC8669-4.1-4`](#rfc8669-4.1-4)

When receiving "invalid" or "conflicting" BGP Prefix-SID attribute, speaker MUST ignore the attribute (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-4.1-4, so no unit is bound to it.

### [`RFC8669-4.1-5`](#rfc8669-4.1-5)

Speaker MUST assign a local (dynamic) label (non-SRGB) for prefixes with invalid/conflicting Prefix-SID (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-4.1-5, so no unit is bound to it.

### [`RFC8669-4.1-6`](#rfc8669-4.1-6)

For "invalid" BGP Prefix-SID attribute, speaker MUST follow the error-handling rules specified in Section 6 (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-4.1-6, so no unit is bound to it.

### [`RFC8669-4.1-7`](#rfc8669-4.1-7)

Implicit NULL label in NLRI: speaker MUST adhere to standard behavior and program MPLS data plane to pop the top label (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-4.1-7, so no unit is bound to it.

### [`RFC8669-4.1-8`](#rfc8669-4.1-8)

The label NLRI defines the outbound label that MUST be used by the receiving node (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8669NLRILabelIsTheOutboundLabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L33) | unit/verify | unproven |

### [`RFC8669-5.1-1`](#rfc8669-5.1-1)

Label field of the advertised NLRI MUST be set to the local/incoming label programmed in the MPLS data plane (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-5.1-1, so no unit is bound to it.

### [`RFC8669-8-1`](#rfc8669-8-1)

The propagation to other ASes MUST be explicitly configured (§8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPrefixSIDEgressBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_test.go#L156) | unit/verify | unproven |
| negative | [`TestPrefixSIDOriginationBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_test.go#L284) | unit/verify | unproven |
| negative | [`prefixsid-ebgp-egress-boundary.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-ebgp-egress-boundary.ci#L4) | functional/verify | unproven |
| positive | [`TestPrefixSIDEgressBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_test.go#L155) | unit/verify | unproven |
| positive | [`TestPrefixSIDOriginationBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid_test.go#L283) | unit/verify | unproven |
| positive | [`prefixsid-ebgp-egress-boundary.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-ebgp-egress-boundary.ci#L7) | functional/verify | unproven |

### [`RFC8669-6-1`](#rfc8669-6-1)

Malformed BGP Prefix-SID attribute: MUST ignore the received attribute and not advertise it to other BGP peers (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669MalformedAttributeDiscardedAndNotAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L208) | unit/verify | unproven |
| negative | [`TestRFC8669TrailingBytesDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L238) | unit/verify | unproven |
| positive | [`TestRFC8669WellFormedAttributeAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L187) | unit/verify | unproven |

### [`RFC8669-6-2`](#rfc8669-6-2)

If the BGP Prefix-SID attribute appears more than once in an UPDATE, all occurrences other than the first SHALL be discarded (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8669-6-2, so no unit is bound to it.

### [`RFC8669-6-3`](#rfc8669-6-3)

If a recognized TLV appears more than once, all occurrences other than the first SHALL be discarded (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669DuplicateRecognizedTLVCannotOverrideFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L165) | unit/verify | unproven |
| positive | [`TestRFC8669DuplicateRecognizedTLVFirstWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L149) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 8669, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 8669, so its obligations are stated where they were written.
