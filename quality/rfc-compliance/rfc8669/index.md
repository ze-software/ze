# RFC 8669 - Segment Routing Prefix Segment Identifier Extensions for BGP

Partial. Every requirement this repository extracted from RFC 8669, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 40.0% | 10 of 25 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 20.0% | 5 of 25 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 25 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 25 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 61.1% | 33 of 54 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 25 | of 46 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 25 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 25 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 25 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 25 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 40.0% | 10 of 25 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 14 | of 25 gated MUSTs judged | 4 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 25 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 46 |
| Gated MUST-level | 25 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 11 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 54 |
| Tagged units | 54 |
| Recorded audit verdicts | 14 |
| Discrimination records | 33 |
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

Unproven duplicate-attribute wire handling remains recorded by [`RFC8669-6-2`](#rfc8669-6-2): the keep-first strip in enforceRFC7606 removes repeated attributes, but code-40 forwarding proof is still owed. Missing SR-MPLS semantics remain explicit in [`RFC8669-3.1-1`](#rfc8669-3.1-1), [`RFC8669-4.1-1`](#rfc8669-4.1-1)/4.1-2 (required Label-Index and invalid-state detection), [`RFC8669-4.1-3`](#rfc8669-4.1-3) (conflict detection), [`RFC8669-4.1-4`](#rfc8669-4.1-4)/4.1-6 (invalid/conflicting ignore and discard triggers), [`RFC8669-4.1-5`](#rfc8669-4.1-5) (dynamic local label allocation), [`RFC8669-4.1-7`](#rfc8669-4.1-7) (implicit-NULL pop) and [`RFC8669-5.1-1`](#rfc8669-5.1-1) (advertised local/incoming label programming). [`RFC8669-4.1-15`](#rfc8669-4.1-15) records the absent invalid-state logging trigger separately. The conditional [`RFC8669-6-4`](#rfc8669-6-4) diagnostic duty now has observed malformed-discard and clean-control proof in [`internal/component/bgp/reactor/rfc8669_discard_diagnostics_test.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_discard_diagnostics_test.go), with selective no-log and spurious-log failures. That proof does not establish missing invalid-state detection or discard initiation and prescribes no slog severity enum.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 10 | one part of the gated population |
| Annotated (including scoped evidence) | 15 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **25** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (10):** [`RFC8669-3-1`](#rfc8669-3-1), [`RFC8669-3.1-3`](#rfc8669-3.1-3), [`RFC8669-3.1-4`](#rfc8669-3.1-4), [`RFC8669-3.1-5`](#rfc8669-3.1-5), [`RFC8669-3.1-6`](#rfc8669-3.1-6), [`RFC8669-3.2-2`](#rfc8669-3.2-2), [`RFC8669-4-1`](#rfc8669-4-1), [`RFC8669-8-1`](#rfc8669-8-1), [`RFC8669-6-1`](#rfc8669-6-1), [`RFC8669-6-3`](#rfc8669-6-3)

**Annotated (including scoped evidence) (15):** [`RFC8669-3.1-1`](#rfc8669-3.1-1), [`RFC8669-3.1-2`](#rfc8669-3.1-2), [`RFC8669-3.2-1`](#rfc8669-3.2-1), [`RFC8669-3.2-3`](#rfc8669-3.2-3), [`RFC8669-3.2-4`](#rfc8669-3.2-4), [`RFC8669-4.1-1`](#rfc8669-4.1-1), [`RFC8669-4.1-2`](#rfc8669-4.1-2), [`RFC8669-4.1-3`](#rfc8669-4.1-3), [`RFC8669-4.1-4`](#rfc8669-4.1-4), [`RFC8669-4.1-5`](#rfc8669-4.1-5), [`RFC8669-4.1-6`](#rfc8669-4.1-6), [`RFC8669-4.1-7`](#rfc8669-4.1-7), [`RFC8669-4.1-8`](#rfc8669-4.1-8), [`RFC8669-5.1-1`](#rfc8669-5.1-1), [`RFC8669-6-2`](#rfc8669-6-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8669-3-1` | For future extensibility, unknown TLVs MUST be ignored and propagated unmodified. (§3, §6) | MUST | 3 | **positive:** `unit/verify` [`TestRFC8669UnknownTLVIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L80). **positive:** `unit/verify` [`TestRFC8669UnknownTLVPropagatedAcrossANextHopChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_nexthop_change_defect_test.go#L133). **negative:** `unit/verify` [`TestRFC8669UnknownTLVPropagatedAcrossANextHopChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_nexthop_change_defect_test.go#L134) |
| `RFC8669-3.1-1` | Label-Index TLV MUST be present in the BGP Prefix-SID attribute attached to IPv4/IPv6 Labeled Unicast prefixes (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** neither the sender nor the receiver requires TLV type 1 for labeled unicast -- validatePrefixSIDAttr accepts a Prefix-SID with no Label-Index TLV (internal/component/bgp/message/rfc7606.go:837) and BuildLabeledUnicast attaches whatever bytes the route configuration produced, including an SRv6-only attribute (internal/component/bgp/message/update_build_labeled.go:189) |
| `RFC8669-3.1-2` | It MUST be ignored when received for other BGP AFI/SAFI combinations. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8669LabelIndexIgnoredOnNonLabeledUnicastFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L98). **negative:** no negative test. **{single-polarity}:** ze has no receive-side Label-Index consumer for any family -- ExtractSRv6SIDFull steps over TLV type 1 by length (internal/component/bgp/plugins/rib/pool/srv6sid.go:47) and no other reader of TLV type 1 exists in internal/component/bgp or internal/core/bgp -- so the TLV is ignored on every AFI/SAFI and there is no label-index-driven behavior to drive negatively |
| `RFC8669-3.1-3` | RESERVED: 8-bit field. It MUST be clear on transmission (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8669LabelIndexTLVReservedAndFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L30). **positive:** `unit/verify` [`TestRFC8669LabelIndexTransmissionClearsReservedAndFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_label_index_transmit_test.go#L18). **positive:** `unit/verify` [`TestRFC8669SRGBFormLabelIndexReservedAndFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L89). **negative:** `unit/verify` [`TestRFC8669LabelIndexTransmissionClearsReservedAndFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_label_index_transmit_test.go#L19) |
| `RFC8669-3.1-4` | RESERVED: 8-bit field. It MUST be clear on transmission and MUST be ignored on reception. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8669LabelIndexReservedAndFlagsZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L96). **positive:** `unit/verify` [`TestRFC8669ReservedAndFlagsClearOnTransmissionIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_reserved_flags_test.go#L42). **negative:** `unit/verify` [`TestRFC8669LabelIndexNonZeroReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L114). **negative:** `unit/verify` [`TestRFC8669ReservedAndFlagsClearOnTransmissionIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_reserved_flags_test.go#L44) |
| `RFC8669-3.1-5` | Flags: 16 bits of flags. None are defined by this document. The Flags field MUST be clear on transmission (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8669LabelIndexTLVReservedAndFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L31). **positive:** `unit/verify` [`TestRFC8669LabelIndexTransmissionClearsReservedAndFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_label_index_transmit_test.go#L20). **positive:** `unit/verify` [`TestRFC8669SRGBFormLabelIndexReservedAndFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L90). **negative:** `unit/verify` [`TestRFC8669LabelIndexTransmissionClearsReservedAndFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_label_index_transmit_test.go#L21) |
| `RFC8669-3.1-6` | Flags: 16 bits of flags. None are defined by this document. The Flags field MUST be clear on transmission and MUST be ignored on reception. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8669LabelIndexReservedAndFlagsZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L97). **positive:** `unit/verify` [`TestRFC8669ReservedAndFlagsClearOnTransmissionIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_reserved_flags_test.go#L47). **negative:** `unit/verify` [`TestRFC8669LabelIndexNonZeroFlagsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L127). **negative:** `unit/verify` [`TestRFC8669ReservedAndFlagsClearOnTransmissionIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_reserved_flags_test.go#L49) |
| `RFC8669-3.2-1` | Flags: 16 bits of flags. None are defined in this document. The Flags field MUST be clear on transmission (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8669OriginatorSRGBTLVFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L55). **negative:** no negative test. **{single-polarity}:** parsePrefixSIDWithSRGB hardcodes both Flags octets to 0 on encode with no non-zero path to reject (internal/core/bgp/attribute/prefixsid.go) |
| `RFC8669-3.2-2` | Flags: 16 bits of flags. None are defined in this document. The Flags field MUST be clear on transmission and MUST be ignored on reception. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8669ReservedAndFlagsClearOnTransmissionIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_reserved_flags_test.go#L52). **positive:** `unit/verify` [`TestRFC8669SRGBFlagsZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L140). **negative:** `unit/verify` [`TestRFC8669ReservedAndFlagsClearOnTransmissionIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_reserved_flags_test.go#L54). **negative:** `unit/verify` [`TestRFC8669SRGBNonZeroFlagsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L151) |
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
| `RFC8669-8-1` | The propagation to other ASes MUST be explicitly configured (§8) | MUST | 8 | **positive:** `unit/verify` [`TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_forward_prefix_sid_announce_rail_test.go#L49). **positive:** `unit/verify` [`TestPrefixSIDEgressBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_forward_prefix_sid_test.go#L155). **positive:** `unit/verify` [`TestPrefixSIDOriginationBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_forward_prefix_sid_test.go#L288). **positive:** `unit/verify` [`TestStaleReadvertiseKeepsPrefixSIDInsideTheSRDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_forward_prefix_sid_readvertise_rail_test.go#L141). **negative:** `unit/verify` [`TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_forward_prefix_sid_announce_rail_test.go#L46). **negative:** `unit/verify` [`TestPrefixSIDEgressBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_forward_prefix_sid_test.go#L156). **negative:** `unit/verify` [`TestPrefixSIDOriginationBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_forward_prefix_sid_test.go#L289). **negative:** `unit/verify` [`TestStaleReadvertiseKeepsPrefixSIDInsideTheSRDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_forward_prefix_sid_readvertise_rail_test.go#L138). **positive:** `functional/verify` [`prefixsid-announce-rail-boundary.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-announce-rail-boundary.ci#L7). **positive:** `functional/verify` [`prefixsid-ebgp-egress-boundary.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-ebgp-egress-boundary.ci#L7). **negative:** `functional/verify` [`prefixsid-announce-rail-boundary.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-announce-rail-boundary.ci#L4). **negative:** `functional/verify` [`prefixsid-ebgp-egress-boundary.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-ebgp-egress-boundary.ci#L4) |
| `RFC8669-6-1` | When a BGP speaker receives a BGP UPDATE message containing a malformed or invalid BGP Prefix-SID attribute attached to an IPv4/ IPv6 Labeled Unicast prefix ([RFC8277]), it MUST ignore the received BGP Prefix-SID attribute and not advertise it to other BGP peers. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8669WellFormedAttributeAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L187). **negative:** `unit/verify` [`TestRFC8669MalformedAttributeDiscardedAndNotAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L208). **negative:** `unit/verify` [`TestRFC8669TrailingBytesDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L238) |
| `RFC8669-6-2` | As per [RFC7606], if the BGP Prefix-SID attribute appears more than once in an UPDATE message, all the occurrences of the attribute other than the first one SHALL be discarded and the UPDATE message will continue to be processed. (§6) | SHALL | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unproven for code 40, no longer unimplemented. The processing half holds -- an already-seen non-MP attribute code is skipped without validation (internal/component/bgp/message/rfc7606.go, ValidateUpdateRFC7606) and recorded in DuplicateRanges, on the attribute-discard result as well as the clean one. The wire half is the RFC 7606 Section 3.g keep-first strip in enforceRFC7606 (internal/component/bgp/reactor/session_validation.go), which removes every recorded later copy through StripAttrRanges before the in-place attribute discard runs, so a malformed first occurrence is tombstoned and its duplicate is gone too. What is missing is the proof: no test tagged to this row drives a repeated code 40 through enforceRFC7606 and reads the bytes forwarded on |
| `RFC8669-6-3` | Similarly, if a recognized TLV appears more than once in a BGP Prefix-SID attribute while the specification only allows for a single occurrence, then all the occurrences of the TLV other than the first one SHALL be discarded and the Prefix-SID attribute will continue to be processed. (§6) | SHALL | 6 | **positive:** `unit/verify` [`TestRFC8669DuplicateRecognizedTLVFirstWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L149). **positive:** `unit/verify` [`TestRFC8669DuplicateServiceTLVDiscardedOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_duplicate_tlv_test.go#L132). **negative:** `unit/verify` [`TestRFC8669DuplicateRecognizedTLVCannotOverrideFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L165). **negative:** `unit/verify` [`TestRFC8669SingleOccurrencePrefixSIDKeptWhole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_duplicate_tlv_test.go#L197). **positive:** `functional/verify` [`prefixsid-duplicate-tlv-relay.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-duplicate-tlv-relay.ci#L4) |
| `RFC8669-4-2` | A BGP speaker SHOULD log an error for further analysis when discarding an attribute. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-6-4` | When discarding an attribute, a BGP speaker SHOULD log an error for further analysis. (§6) | SHOULD | 6 | **positive:** `unit/verify` [`TestRFC8669MalformedPrefixSIDDiscardDiagnostic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_discard_diagnostics_test.go#L28). **negative:** `unit/verify` [`TestRFC8669CleanPrefixSIDNoDiscardDiagnostic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_discard_diagnostics_test.go#L94) |
| `RFC8669-4.1-9` | If multiple valid paths for the same prefix are received from multiple BGP speakers or, in the case of [RFC7911], from the same BGP speaker, and the BGP Prefix-SID attributes do not contain the same label index, then the label index from the best path BGP Prefix-SID attribute SHOULD be chosen with a notable exception being when [RFC5004] is being used to dampen route changes. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-4.1-10` | When a BGP speaker receives a path from a neighbor with an "acceptable" BGP Prefix-SID attribute and that path is selected as the best path, it SHOULD program the derived label as the label for the prefix in its local MPLS data plane. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-4.1-11` | In the case of a "conflicting" BGP Prefix-SID attribute, a BGP speaker SHOULD NOT treat it as an error (§4.1) | SHOULD NOT | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-4.1-12` | In the case of a "conflicting" BGP Prefix-SID attribute, a BGP speaker SHOULD NOT treat it as an error and SHOULD propagate the attribute unchanged. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-4.1-13` | A BGP speaker SHOULD log a warning for further analysis, i.e., in the case the conflict is not due to a label-index transition. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-4.1-14` | When a BGP Prefix-SID attribute changes and transitions from "conflicting" to "acceptable", the BGP Prefix-SID attributes for other prefixes may also transition to "acceptable" as well. Implementations SHOULD ensure all impacted prefixes revert to using the label indices corresponding to these newly "acceptable" BGP Prefix-SID attributes. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8669-4.1-15` | A BGP speaker SHOULD log an error for further analysis. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** invalid Prefix-SID state and its logging trigger are not computed by validatePrefixSIDAttr in internal/component/bgp/message/rfc7606.go; recording this requirement does not commission missing SR-MPLS semantics. |
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
| [`RFC8669-6-2`](#rfc8669-6-2) As per [RFC7606], if the BGP Prefix-SID attribute appears more than once in an UPDATE message, all the occurrences of the attribute other than the first one SHALL be discarded and the UPDATE message will continue to be processed. (§6) | {gap}, no test | unproven for code 40, no longer unimplemented. The processing half holds -- an already-seen non-MP attribute code is skipped without validation (internal/component/bgp/message/rfc7606.go, ValidateUpdateRFC7606) and recorded in DuplicateRanges, on the attribute-discard result as well as the clean one. The wire half is the RFC 7606 Section 3.g keep-first strip in enforceRFC7606 (internal/component/bgp/reactor/session_validation.go), which removes every recorded later copy through StripAttrRanges before the in-place attribute discard runs, so a malformed first occurrence is tombstoned and its duplicate is gone too. What is missing is the proof: no test tagged to this row drives a repeated code 40 through enforceRFC7606 and reads the bytes forwarded on |
| [`RFC8669-4.1-15`](#rfc8669-4.1-15) A BGP speaker SHOULD log an error for further analysis. (§4.1) | {gap} | invalid Prefix-SID state and its logging trigger are not computed by validatePrefixSIDAttr in internal/component/bgp/message/rfc7606.go; recording this requirement does not commission missing SR-MPLS semantics. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8669-3-1`](#rfc8669-3-1)

For future extensibility, unknown TLVs MUST be ignored and propagated unmodified. (§3, §6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged after the D-8 fix (applyFactsNextHop records Remove{5,6} on code 40; prefixSIDNextHopHandler rewrites the attribute per route). Clause 'ignored': message/rfc8669_test.go::TestRFC8669UnknownTLVIsIgnored pins RFC7606ActionNone and intact bytes for an unallocated TLV (no discrimination record; supplementary). Clause 'propagated unmodified': reactor TestRFC8669UnknownTLVPropagatedAcrossANextHopChange reads attribute 40 on the wire over both relay rails (forwardUpdateCore, reactorForwardRS): positive, next hop unchanged, [LabelIndex, L3 Svc, type 200, L2 Svc, SRGB] byte-identical; negative (R1b: the input that used to violate), next-hop-self with NEXT_HOP really 10.0.0.254, attribute == LabelIndex+type200+SRGB byte-identical in received order. Observed red: positive with applyFactsNextHop broken, negative with prefixSIDNextHopHandler broken. Buffers isolated (well-formed TLVs, only type 200 unknown).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669UnknownTLVPropagatedAcrossANextHopChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_nexthop_change_defect_test.go#L134) | unit/verify | revert, verified |
| positive | [`TestRFC8669UnknownTLVIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L80) | unit/verify | unproven |
| positive | [`TestRFC8669UnknownTLVPropagatedAcrossANextHopChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_nexthop_change_defect_test.go#L133) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 8669 §3.1 whole field sentence: "RESERVED: 8-bit field. It MUST be clear on transmission and MUST be ignored on reception." This row owns the transmission clause; reception remains RFC8669-3.1-4. Both internal/component/bgp/config/rfc8669_test.go tagged units (TestRFC8669LabelIndexTLVReservedAndFlagsClearOnTransmission and TestRFC8669SRGBFormLabelIndexReservedAndFlagsClearOnTransmission) pin the reserved octet and independent bare-index/SRGB encoders internal/core/bgp/attribute/prefixsid.go::EncodePrefixSID/parsePrefixSIDWithSRGB. internal/component/bgp/reactor/rfc8669_label_index_transmit_test.go::TestRFC8669LabelIndexTransmissionClearsReservedAndFlags separately supplies clean and dirty Label-Index values to raw, whole-packet, parsed, filter-override, general and route-server paths, then checks exact attribute bytes on the real session writer. clearTransmittedLabelIndex in session_prefix_sid.go clears only the bounded type-1 fields in an owned outgoing buffer; session_write.go invokes it after policy. Whole-value equality also requires unchanged unknown TLV, Originator SRGB, source buffer and NLRI. These distinct input cases fail if a dirty Reserved octet escapes or normalization corrupts adjacent content. This repairs the earlier origination-only proof limitation. No SRv6 SID allocation, transposition, VPP or RFC9252 feature claim follows; no runtime result asserted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669LabelIndexTransmissionClearsReservedAndFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_label_index_transmit_test.go#L19) | unit/verify | revert, verified |
| positive | [`TestRFC8669LabelIndexTLVReservedAndFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L30) | unit/verify | revert, verified |
| positive | [`TestRFC8669SRGBFormLabelIndexReservedAndFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L89) | unit/verify | revert, verified |
| positive | [`TestRFC8669LabelIndexTransmissionClearsReservedAndFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_label_index_transmit_test.go#L18) | unit/verify | revert, verified |

### [`RFC8669-3.1-4`](#rfc8669-3.1-4)

RESERVED: 8-bit field. It MUST be clear on transmission and MUST be ignored on reception. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Transmission: every EncodePrefixSID shape (bare, all-ones index, SRGB-leading) has Label-Index Reserved 0. Reception: Reserved 0xFF gives Action None, no discard, bytes unchanged, ParsePrefixSID ok. Judge break (Reserved 1 in the SRGB-path encoder): red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669ReservedAndFlagsClearOnTransmissionIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_reserved_flags_test.go#L44) | unit/verify | revert, verified |
| negative | [`TestRFC8669LabelIndexNonZeroReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L114) | unit/verify | unproven |
| positive | [`TestRFC8669ReservedAndFlagsClearOnTransmissionIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_reserved_flags_test.go#L42) | unit/verify | revert, verified |
| positive | [`TestRFC8669LabelIndexReservedAndFlagsZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L96) | unit/verify | unproven |

### [`RFC8669-3.1-5`](#rfc8669-3.1-5)

Flags: 16 bits of flags. None are defined by this document. The Flags field MUST be clear on transmission (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 8669 §3.1 whole field sentence: "Flags: 16 bits of flags. None are defined by this document. The Flags field MUST be clear on transmission and MUST be ignored on reception." This row owns transmission; reception remains RFC8669-3.1-6. internal/component/bgp/config/rfc8669_test.go::TestRFC8669LabelIndexTLVReservedAndFlagsClearOnTransmission and TestRFC8669SRGBFormLabelIndexReservedAndFlagsClearOnTransmission assert both Flags octets zero for internal/core/bgp/attribute/prefixsid.go::EncodePrefixSID and parsePrefixSIDWithSRGB. The clean/dirty subcases of internal/component/bgp/reactor/rfc8669_label_index_transmit_test.go::TestRFC8669LabelIndexTransmissionClearsReservedAndFlags drive six final writer/relay paths, including filter overrides, and demand the full expected Prefix-SID with Flags 0000. session_prefix_sid.go::clearTransmittedLabelIndex and its session_write.go callers operate only on owned outgoing bytes after policy. Exact source, unknown-TLV, SRGB and NLRI preservation isolates the type-1 normalization from unrelated dropping or rewriting. Genuine clean and nonzero cases distinguish both polarities; this is no longer only a configuration-encoder assertion. RFC8669 explicitly defers SRv6 data-plane behavior, so this verdict grants no SRv6/VPP implementation scope. Runtime proof remains the parent recorder responsibility.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669LabelIndexTransmissionClearsReservedAndFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_label_index_transmit_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestRFC8669LabelIndexTLVReservedAndFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestRFC8669SRGBFormLabelIndexReservedAndFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestRFC8669LabelIndexTransmissionClearsReservedAndFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_label_index_transmit_test.go#L20) | unit/verify | revert, verified |

### [`RFC8669-3.1-6`](#rfc8669-3.1-6)

Flags: 16 bits of flags. None are defined by this document. The Flags field MUST be clear on transmission and MUST be ignored on reception. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Same unit: Label-Index Flags 0 on every encoded shape; received Flags 0xFFFF ignored with bytes unchanged and ParsePrefixSID ok.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669ReservedAndFlagsClearOnTransmissionIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_reserved_flags_test.go#L49) | unit/verify | revert, verified |
| negative | [`TestRFC8669LabelIndexNonZeroFlagsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L127) | unit/verify | unproven |
| positive | [`TestRFC8669ReservedAndFlagsClearOnTransmissionIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_reserved_flags_test.go#L47) | unit/verify | revert, verified |
| positive | [`TestRFC8669LabelIndexReservedAndFlagsZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L97) | unit/verify | unproven |

### [`RFC8669-3.2-1`](#rfc8669-3.2-1)

Flags: 16 bits of flags. None are defined in this document. The Flags field MUST be clear on transmission (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Non-compliant behaviour: non-zero Originator SRGB Flags emitted. config/rfc8669_test.go::TestRFC8669OriginatorSRGBTLVFlagsClearOnTransmission asserts require.Equal([]byte{0,0}, srgb[3:5]) on EncodePrefixSID output. Single-polarity marker present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8669OriginatorSRGBTLVFlagsClearOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8669_test.go#L55) | unit/verify | unproven |

### [`RFC8669-3.2-2`](#rfc8669-3.2-2)

Flags: 16 bits of flags. None are defined in this document. The Flags field MUST be clear on transmission and MUST be ignored on reception. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Same unit: SRGB Flags 0 on send including all-ones base/range; received SRGB Flags 0xFFFF ignored, bytes unchanged (the assertion the earlier note missed). Judge break (SRGB Flags 1): red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669ReservedAndFlagsClearOnTransmissionIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_reserved_flags_test.go#L54) | unit/verify | revert, verified |
| negative | [`TestRFC8669SRGBNonZeroFlagsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_test.go#L151) | unit/verify | unproven |
| positive | [`TestRFC8669ReservedAndFlagsClearOnTransmissionIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8669_reserved_flags_test.go#L52) | unit/verify | revert, verified |
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
| negative | [`TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_forward_prefix_sid_announce_rail_test.go#L46) | unit/verify | revert, verified |
| negative | [`TestStaleReadvertiseKeepsPrefixSIDInsideTheSRDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_forward_prefix_sid_readvertise_rail_test.go#L138) | unit/verify | revert, verified |
| negative | [`TestPrefixSIDEgressBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_forward_prefix_sid_test.go#L156) | unit/verify | revert, verified |
| negative | [`TestPrefixSIDOriginationBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_forward_prefix_sid_test.go#L289) | unit/verify | revert, verified |
| negative | [`prefixsid-announce-rail-boundary.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-announce-rail-boundary.ci#L4) | functional/verify | revert, verified |
| negative | [`prefixsid-ebgp-egress-boundary.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-ebgp-egress-boundary.ci#L4) | functional/verify | revert, verified |
| positive | [`TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_forward_prefix_sid_announce_rail_test.go#L49) | unit/verify | revert, verified |
| positive | [`TestStaleReadvertiseKeepsPrefixSIDInsideTheSRDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_forward_prefix_sid_readvertise_rail_test.go#L141) | unit/verify | revert, verified |
| positive | [`TestPrefixSIDEgressBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_forward_prefix_sid_test.go#L155) | unit/verify | revert, verified |
| positive | [`TestPrefixSIDOriginationBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_forward_prefix_sid_test.go#L288) | unit/verify | revert, verified |
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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged after 465af38f1d against rfc/full/rfc8669.txt Sections 3, 3.1, 3.2 and 6 and rfc/full/rfc9252.txt Section 7. Positive TestRFC8669DuplicateServiceTLVDiscardedOnReceive drives the real receive entry (enforceRFC7606 to publishBase) on an internal and a route-server-client session and asserts no error, RFC7606ActionNone, exactly one attribute 40 equal to the received TLVs less the repeat in received order, the peer's header width, the received attributes after code 40 octet for octet, and the same TLVs on the forward path; its cases are repeated SRv6 L3 and L2 Service TLVs on labelled unicast, Label-Index twice with an unknown TLV between, an unknown TLV after the repeat, a COMMUNITIES attribute after code 40, an Extended Length header, and L3 Service twice on IPv4 unicast (RFC 9252 Section 7 names no family). Negative TestRFC8669SingleOccurrencePrefixSIDKeptWhole pins the 'only allows for a single occurrence' qualifier and the Section 6 MUST 'unknown TLVs MUST be ignored and propagated unmodified': each type once, an unknown TLV twice, an Originator SRGB TLV twice (Section 3.2 'MUST NOT be changed during the propagation', so keeping both copies is the conforming answer) and a Label-Index TLV twice on IPv4 unicast must publish the received body octet-equal and forward every TLV. test/plugin/prefixsid-duplicate-tlv-relay.ci proves the session-in to session-out path with two byte-identical controls (unknown repeat; Label-Index repeat off labelled unicast) and an L3 Service repeat trimmed with the trailing unknown TLV and COMMUNITIES kept. The judge ran five -overlay mutants on HEAD, none recorded: Label-Index single on every family, stop copying after the first discard, family always read as IPv4 unicast, Extended Length header read as 3 octets, and the attributes after code 40 dropped; each turned a tagged reactor test red. Recorded discrimination records cover all three reactor/.ci tags (producer disabled). Readings, not findings: RFC 8669 sets no explicit occurrence limit for Label-Index; Ze treats it as single on IPv4/IPv6 labelled unicast from 'the label index for a given prefix', and off labelled unicast, where Section 3.1 says it 'MUST be ignored when received for other BGP AFI/SAFI combinations', as unrecognized and propagated unmodified (owner decision 2026-10-03). This is a defensible reading of an RFC that is silent, not RFC text. An UPDATE carrying both RFC 4271 NLRI and an MP_REACH_NLRI is judged by the MP_REACH family (prefixSIDRouteFamily), and no test covers it. The pool pair TestRFC8669DuplicateRecognizedTLVFirstWins / CannotOverrideFirst is one read-side first-wins assertion wearing two hats (the SIDs are swapped), so the polarity pair rests on the reactor and .ci tests, not on these two. A malformed repeat stays treat-as-withdraw first (untagged TestRFC8669DuplicateMalformedServiceTLVStillWithdrawn), which is RFC 9252 Section 7 and outside this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669DuplicateRecognizedTLVCannotOverrideFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L165) | unit/verify | unproven |
| negative | [`TestRFC8669SingleOccurrencePrefixSIDKeptWhole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_duplicate_tlv_test.go#L197) | unit/verify | revert, verified |
| positive | [`TestRFC8669DuplicateRecognizedTLVFirstWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc8669_test.go#L149) | unit/verify | unproven |
| positive | [`TestRFC8669DuplicateServiceTLVDiscardedOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_duplicate_tlv_test.go#L132) | unit/verify | revert, verified |
| positive | [`prefixsid-duplicate-tlv-relay.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-duplicate-tlv-relay.ci#L4) | functional/verify | revert, verified |

### [`RFC8669-6-4`](#rfc8669-6-4)

When discarding an attribute, a BGP speaker SHOULD log an error for further analysis. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 8669 Section 6 states: "When discarding an attribute, a BGP speaker SHOULD log an error for further analysis." TestRFC8669MalformedPrefixSIDDiscardDiagnostic exercises isolated TLV-overrun and trailing-byte inputs through actual Session receive enforcement. Both produce AttributeDiscard, remove code 40, preserve the announced route and unrelated attributes without withdrawal, and emit an attribute-discard diagnostic identifying attribute 40 and the exact original UPDATE. TestRFC8669CleanPrefixSIDNoDiscardDiagnostic proves ActionNone, byte-identical publication with one valid Label-Index attribute, and no matching discard diagnostic. The current pair passes twenty race iterations. Selectively suppressing both code-40 diagnostic calls makes the malformed cases fail while the clean control passes; selectively adding a discard diagnostic on ActionNone makes the clean control fail while the malformed cases pass. The semantic cuts and restored race pass are recorded in job-bgp-final-new-claim-discrimination-2bf9854a.log. Both tagged units also have native revert bindings to session_validation.go::applyRFC7606; these producer-halting failures are not substituted for the selective semantic failures. Independent review accepted this conditional logging proof. The requirement prescribes neither exact human-readable wording nor a slog severity enum. This verdict establishes logging when the exercised discard occurs, not complete malformed/invalid-state detection, missing discard initiation, downstream advertisement, or SR-MPLS functionality. Those missing triggers remain separately disclosed by RFC8669-6-1 and RFC8669-4.1-15.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8669CleanPrefixSIDNoDiscardDiagnostic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_discard_diagnostics_test.go#L94) | unit/verify | revert, verified |
| positive | [`TestRFC8669MalformedPrefixSIDDiscardDiagnostic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_discard_diagnostics_test.go#L28) | unit/verify | revert, verified |

### [`RFC8669-4.1-15`](#rfc8669-4.1-15)

A BGP speaker SHOULD log an error for further analysis. (§4.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. RFC 8669 Section 4.1 states: "In the case of an "invalid" BGP Prefix-SID attribute, a BGP speaker MUST follow the error-handling rules specified in Section 6. A BGP speaker SHOULD log an error for further analysis." The same section requires an attribute received without a Label-Index TLV to be considered invalid. validatePrefixSIDAttr checks TLV framing and SRv6 Service TLVs, but does not require Label-Index on labeled-unicast routes or compute that invalid state. Its registered receive path is reachable; the generic discard logger does not supply the absent trigger. This is the disclosed SR-MPLS invalid-state gap, distinct from conflicting-attribute warnings, EBGP boundary logging and conditional logging upon an actual discard. No missing SR-MPLS implementation is commissioned.

No test carries RFC8669-4.1-15, so no unit is bound to it.

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
| `6` | not stated | 4 | walked | The separate SHOULD logging sentence applies when discarding malformed or invalid labeled-unicast Prefix-SID attributes; RFC8669-6-4 records it without asserting a particular logging enum or proving the invalid-state trigger. Site 6:4 remains the unknown-TLV propagation occurrence mapped to RFC8669-3-1. |
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
