# RFC 9552 - Distribution of Link-State and Traffic Engineering Information Using BGP

Partial. Every requirement this repository extracted from RFC 9552, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 76.3% | 45 of 59 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 11.9% | 7 of 59 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 59 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 44.7% | 55 of 123 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 59 | of 88 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 5 | of 59 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 8.5% | 5 of 59 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 59 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 59 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 3.4% | 2 of 59 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 41 | of 59 gated MUSTs judged | 18 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 59 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 88 |
| Gated MUST-level | 59 |
| Not applicable, so out of scope | 5 |
| Declared gaps | 2 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 123 |
| Tagged units | 123 |
| Recorded audit verdicts | 41 |
| Discrimination records | 55 |
| Summary | `rfc/short/rfc9552.md` |
| Requirement shard | `rfc/requirements/rfc9552.md` |
| RFC text | `rfc/full/rfc9552.txt` |

## Enrolment

Enrolled: Distribution of Link-State and TE Information Using BGP

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Consumer decoding and syntactic propagation remain separate from native origination. `bgp-ls-export` consumes registered IS-IS, OSPF, and EPE snapshots, emits distinct topology identities, enforces opaque LSA provenance, clears originated reserved flags, reconciles withdrawals and replacements, and supports per-domain 64-bit Instance-ID configuration. Native SPF-unreachable origins are withdrawn and re-advertised on native SPF recovery, including unchanged LSDB objects
- reachable-origin half-links remain advertised. Native EPE index advertisements require a matching advertised SRGB.


**What the ledger says remains**

Two MUST rows carry {gap}. [`RFC9552-5.3-2`](#rfc9552-5.3-2) still requires oversized propagation to discard the BGP-LS Attribute first. The owner selected **Standard origination only**: `encodeTopology` emits standard NLRI types and `originateAttributes` refuses private-use TLV types; no native vendor-private producer is enabled. Section 5.4's Enterprise Code duties are conditional on private origination; received unknown/private NLRIs remain opaque on the propagation path. New native origination proofs require centralized execution and discrimination; no conformance claim follows from the source additions alone. The native exporter does not bound a BGP-LS Attribute by the collector session's maximum UPDATE size ([`RFC9552-5.3-1`](#rfc9552-5.3-1)).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 45 | one part of the gated population |
| Annotated instead of tested | 14 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **59** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (45):** [`RFC9552-5.1-1`](#rfc9552-5.1-1), [`RFC9552-5.1-2`](#rfc9552-5.1-2), [`RFC9552-5.1-3`](#rfc9552-5.1-3), [`RFC9552-5.1-4`](#rfc9552-5.1-4), [`RFC9552-5.1-5`](#rfc9552-5.1-5), [`RFC9552-5.1-6`](#rfc9552-5.1-6), [`RFC9552-5.2-1`](#rfc9552-5.2-1), [`RFC9552-5.2-2`](#rfc9552-5.2-2), [`RFC9552-5.2-3`](#rfc9552-5.2-3), [`RFC9552-5.2-6`](#rfc9552-5.2-6), [`RFC9552-5.2-7`](#rfc9552-5.2-7), [`RFC9552-5.2-8`](#rfc9552-5.2-8), [`RFC9552-5.2.1.4-1`](#rfc9552-5.2.1.4-1), [`RFC9552-5.2.2-1`](#rfc9552-5.2.2-1), [`RFC9552-5.2.2-2`](#rfc9552-5.2.2-2), [`RFC9552-5.2.2-3`](#rfc9552-5.2.2-3), [`RFC9552-5.2.2-5`](#rfc9552-5.2.2-5), [`RFC9552-5.3.2.1-1`](#rfc9552-5.3.2.1-1), [`RFC9552-5.3.2.2-1`](#rfc9552-5.3.2.2-1), [`RFC9552-5.9-1`](#rfc9552-5.9-1), [`RFC9552-5.2.3-1`](#rfc9552-5.2.3-1), [`RFC9552-5.3.2.3-2`](#rfc9552-5.3.2.3-2), [`RFC9552-8.2.2-1`](#rfc9552-8.2.2-1), [`RFC9552-8.2.2-2`](#rfc9552-8.2.2-2), [`RFC9552-8.2.2-4`](#rfc9552-8.2.2-4), [`RFC9552-8.2.2-5`](#rfc9552-8.2.2-5), [`RFC9552-8.2.2-6`](#rfc9552-8.2.2-6), [`RFC9552-8.2.6-1`](#rfc9552-8.2.6-1), [`RFC9552-5.2.2-6`](#rfc9552-5.2.2-6), [`RFC9552-5.2.1.1-1`](#rfc9552-5.2.1.1-1), [`RFC9552-5.2.1.1-2`](#rfc9552-5.2.1.1-2), [`RFC9552-5.2.2.1-1`](#rfc9552-5.2.2.1-1), [`RFC9552-8.2.2-9`](#rfc9552-8.2.2-9), [`RFC9552-8.2.2-10`](#rfc9552-8.2.2-10), [`RFC9552-8.2.3-5`](#rfc9552-8.2.3-5), [`RFC9552-5.2.2.1-2`](#rfc9552-5.2.2.1-2), [`RFC9552-5.2.3-2`](#rfc9552-5.2.3-2), [`RFC9552-5.3.1.1-1`](#rfc9552-5.3.1.1-1), [`RFC9552-5.3.1.5-1`](#rfc9552-5.3.1.5-1), [`RFC9552-5.3.2.2-3`](#rfc9552-5.3.2.2-3), [`RFC9552-5.3.2.6-1`](#rfc9552-5.3.2.6-1), [`RFC9552-5.3.2.6-2`](#rfc9552-5.3.2.6-2), [`RFC9552-5.3.3.1-1`](#rfc9552-5.3.3.1-1), [`RFC9552-5.3.3.6-1`](#rfc9552-5.3.3.6-1), [`RFC9552-5.3.3.6-2`](#rfc9552-5.3.3.6-2)

**Annotated instead of tested (14):** [`RFC9552-5.2-4`](#rfc9552-5.2-4), [`RFC9552-5.2-5`](#rfc9552-5.2-5), [`RFC9552-5.2.1.4-2`](#rfc9552-5.2.1.4-2), [`RFC9552-5.2.2-4`](#rfc9552-5.2.2-4), [`RFC9552-5.2.3.1-1`](#rfc9552-5.2.3.1-1), [`RFC9552-5.2.1-1`](#rfc9552-5.2.1-1), [`RFC9552-5.3.2.3-1`](#rfc9552-5.3.2.3-1), [`RFC9552-5.5-1`](#rfc9552-5.5-1), [`RFC9552-5.3-1`](#rfc9552-5.3-1), [`RFC9552-5.4-1`](#rfc9552-5.4-1), [`RFC9552-5.3-2`](#rfc9552-5.3-2), [`RFC9552-5.1-7`](#rfc9552-5.1-7), [`RFC9552-8.2.6-2`](#rfc9552-8.2.6-2), [`RFC9552-5.4-2`](#rfc9552-5.4-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9552-5.1-1` | To compare NLRIs with unknown TLVs, all TLVs within the NLRI MUST be ordered in ascending order by TLV Type. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestLinkDescriptorOrdersMixedFamilyAddressesAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L39). **negative:** `unit/verify` [`TestNoDescriptorEmitsADescendingTLVSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L73) |
| `RFC9552-5.1-2` | If there are multiple TLVs of the same type within a single NLRI, then the TLVs sharing the same type MUST be first in ascending order based on the Length field followed by ascending order based on the Value field. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestNodeDescriptorOrdersRepeatedSRv6SIDs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L124). **negative:** `unit/verify` [`TestSRv6SIDOrderIsLengthBeforeValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L170) |
| `RFC9552-5.1-3` | Unknown and unsupported types MUST be preserved and propagated within both the NLRI and the BGP-LS Attribute. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC7752UnknownTLVPreservedAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L65). **negative:** `unit/verify` [`TestRFC7752MalformedTLVNotPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L103) |
| `RFC9552-5.1-4` | The presence of unknown or unexpected TLVs MUST NOT result in the NLRI or the BGP-LS Attribute being considered malformed. (§5.1) | MUST NOT | 5.1 | **positive:** `unit/verify` [`TestRFC9552UnorderedUnexpectedAttributeTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L53). **negative:** `unit/verify` [`TestRFC9552AttributeSyntaxStillRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L94) |
| `RFC9552-5.1-5` | NLRIs having TLVs that do not follow the above ordering rules MUST be considered as malformed by a BGP-LS Propagator. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC9552LinkStateNLRIOutOfOrderTLVsAreDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L165). **negative:** `unit/verify` [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L53) |
| `RFC9552-5.1-6` | BGP-LS Attribute with unordered TLVs MUST NOT be considered malformed (§5.1) | MUST NOT | 5.1 | **positive:** `unit/verify` [`TestRFC9552UnorderedUnexpectedAttributeTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L52). **negative:** `unit/verify` [`TestRFC9552AttributeSyntaxStillRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L93) |
| `RFC9552-5.2-1` | All non-VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 71 (§5.2) | SHALL | 5.2 | **positive:** `unit/verify` [`TestRFC7752NonVPNFamilyIsAFI16388SAFI71`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L230). **negative:** `unit/verify` [`TestRFC7752NonLinkStateFamilyRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L267) |
| `RFC9552-5.2-2` | VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 72 (§5.2) | SHALL | 5.2 | **positive:** `unit/verify` [`TestRFC7752VPNFamilyIsAFI16388SAFI72`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L248). **positive:** `unit/verify` [`TestRFC9552BGPLSVPNNLRIFramedByLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9552_bgpls_test.go#L135). **negative:** `unit/verify` [`TestRFC7752NonLinkStateFamilyRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L268) |
| `RFC9552-5.2-3` | For all information derived from other protocols, the corresponding Protocol-ID MUST be used (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC9552NativeProtocolIDFollowsSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L25). **negative:** `unit/verify` [`TestRFC9552NativeProtocolIDFollowsSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L38) |
| `RFC9552-5.2-4` | The network operator MUST assign the same BGP-LS Instance-IDs on all BGP-LS Producers within a given IGP domain (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the sentence binds the network operator, who assigns Instance-IDs across Producers; ze carries the per-domain value the operator configures unchanged into the NLRI (internal/component/bgp/plugins/ls_export/export_config.go::parseExportConfig) and cannot see which IGP domain another Producer reports |
| `RFC9552-5.2-5` | Unique BGP-LS Instance-IDs MUST be assigned to routing protocol instances operating in different IGP domains (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the sentence binds the network operator, who assigns Instance-IDs to IGP domains; ze carries the per-domain value the operator configures unchanged into the NLRI (internal/component/bgp/plugins/ls_export/export_config.go::parseExportConfig) and cannot see the domains of other routers |
| `RFC9552-5.2-6` | When adding, removing, or modifying a TLV/sub-TLV from a Link-State NLRI, the BGP-LS Producer MUST withdraw the old NLRI by including it in the MP_UNREACH_NLRI. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC9552NativeDescriptorChangeWithdrawsFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L60). **negative:** `unit/verify` [`TestRFC9552NativeDescriptorChangeWithdrawsFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L71) |
| `RFC9552-5.2-7` | For two BGP Speakers to exchange Link-State NLRI, they MUST use BGP Capabilities Advertisement to ensure that they are both capable of properly processing such NLRI. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC7752BGPLSCapabilityAdvertisedAndNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7752_bgpls_test.go#L29). **negative:** `unit/verify` [`TestRFC7752BGPLSCapabilityNotNegotiatedWhenPeerSilent`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7752_bgpls_test.go#L63) |
| `RFC9552-5.2-8` | An implementation MUST handle unknown Link-State NLRI types as opaque objects and MUST preserve and propagate them (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC9552BGPLSVPNNLRIFramedByLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9552_bgpls_test.go#L136). **positive:** `unit/verify` [`TestRFC9552UnknownBGPLSNLRITypeIsOpaque`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9552_bgpls_test.go#L46). **negative:** `unit/verify` [`TestRFC9552MalformedBGPLSNLRIRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9552_bgpls_test.go#L90). **positive:** `functional/verify` [`rfc9552-52-rs-opaque-withdraw-peer-down.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc9552-52-rs-opaque-withdraw-peer-down.ci#L3) |
| `RFC9552-5.2.1.4-1` | At most, there MUST be one instance of each sub-TLV type present in any Node Descriptor. (§5.2.1.4) | MUST | 5.2.1.4 | **positive:** `unit/verify` [`TestRFC9552LinkStateNodeDescriptorDuplicateSubTLVIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L127). **negative:** `unit/verify` [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L54) |
| `RFC9552-5.2.1.4-2` | The sub-TLVs within a Node Descriptor MUST be arranged in ascending order by sub-TLV type. (§5.2.1.4) | MUST | 5.2.1.4 | **positive:** `unit/verify` [`TestRFC7752NodeDescriptorSubTLVsAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L165). **negative:** no negative test. **{single-polarity}:** NodeDescriptor.WriteTo emits sub-TLVs 512, 513, 514, 515, 516 and 517 in that fixed ascending order (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:98), and the ordering duty falls on the sender: parseNodeDescriptorTLVs (internal/component/bgp/plugins/nlri/ls/types.go:391) accepts sub-TLVs in any order on receipt, so there is no out-of-order input for ze to reject |
| `RFC9552-5.2.2-1` | If interface and neighbor addresses, either IPv4 or IPv6, are present, then the interface/neighbor address TLVs MUST be included (§5.2.2) | MUST | 5.2.2 | **positive:** `unit/verify` [`TestRFC9552NativeLinkIdentitySelection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L242). **negative:** `unit/verify` [`TestRFC9552NativeLinkDescriptorComplements`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L101) |
| `RFC9552-5.2.2-2` | If interface and neighbor addresses, either IPv4 or IPv6, are present, then the interface/neighbor address TLVs MUST be included, and the Link Local/Remote Identifiers TLV MUST NOT be included in the Link Descriptor. (§5.2.2) | MUST NOT | 5.2.2 | **positive:** `unit/verify` [`TestRFC9552NativeLinkDescriptorComplements`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L97). **negative:** `unit/verify` [`TestRFC9552NativeLinkIdentitySelection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L243) |
| `RFC9552-5.2.2-3` | IPv4/IPv6 link-local addresses MUST NOT be carried in the IPv4/IPv6 interface/neighbor address TLVs (259/260/261/262) as descriptors of a link since they are not considered unique. (§5.2.2) | MUST NOT | 5.2.2 | **positive:** `unit/verify` [`TestRFC9552NativeLinkDescriptorComplements`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L98). **negative:** `unit/verify` [`TestRFC9552NativeLinkIdentitySelection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L244) |
| `RFC9552-5.2.2-4` | If interface and neighbor addresses are not present and the link local/remote identifiers are present, then the Link Local/Remote Identifiers TLV MUST be included in the Link Descriptor. The Link Local/Remote identifiers MUST be included in the Link Descriptor and in the case of links having only IPv6 link-local addressing on them. (§5.2.2) | MUST | 5.2.2 | **positive:** `unit/verify` [`TestRFC9552NativeLinkIdentitySelection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L245). **negative:** no negative test. **{single-polarity}:** the only non-conforming output is a link-local-only link without TLV 258, which is the positive test's own assertion; internal/component/bgp/plugins/ls_export/export_encode.go::encodeNativeLink appends TLV 258 whenever no global address descriptor was produced and the source holds link identifiers, so no input exists for a refusal |
| `RFC9552-5.2.2-5` | The Multi-Topology Identifier TLV MUST be included as a Link Descriptor if the underlying IGP link object is associated with a non-default topology. (§5.2.2) | MUST | 5.2.2 | **positive:** `unit/verify` [`TestRFC9552NativeLinkNonDefaultTopology`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L120). **negative:** `unit/verify` [`TestRFC9552NativeLinkNonDefaultTopology`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L122) |
| `RFC9552-5.2.3.1-1` | The OSPF Route Type TLV MUST be included in the advertisement when the type is either being signaled explicitly in the underlying LSA or can be determined via another LSA for the same prefix when it is not signaled explicitly (e.g., in the case of OSPFv2 Extended Prefix Opaque LSA [RFC7684]). (§5.2.3.1) | MUST | 5.2.3.1 | **positive:** `unit/verify` [`TestRFC9552NativeOSPFRouteType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L134). **negative:** no negative test. **{single-polarity}:** a zero RouteType is the source's statement that no route type is signaled, where the requirement does not bind, and every non-zero type is emitted as TLV 264 (internal/component/bgp/plugins/ls_export/export_encode.go::encodeTopology), so no input exists for a refusal |
| `RFC9552-5.2.1-1` | When configured, these auxiliary TE Router-IDs (TLV 1028/1029) MUST be included in the node attribute described in Section 5.3.1 (§5.2.1) | MUST | 5.2.1 | **positive:** `unit/verify` [`TestRFC9552ISISNodeRouterID`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/bgpls_export_rfc9552_test.go#L49). **negative:** no negative test. **{single-polarity}:** the obligation is to include the Router-IDs the IGP carries, so its only failure is omission, which the positive test asserts per node; no input exists that ze could refuse |
| `RFC9552-5.3.2.1-1` | All auxiliary Router-IDs of both the local and the remote node MUST be included in the link attribute of each Link NLRI. (§5.3.2.1) | MUST | 5.3.2.1 | **positive:** `unit/verify` [`TestRFC9552ISISLinkRouterIDs`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/bgpls_export_rfc9552_test.go#L72). **negative:** `unit/verify` [`TestRFC9552ISISLinkRouterIDs`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/bgpls_export_rfc9552_test.go#L77) |
| `RFC9552-5.3.2.2-1` | The MPLS Protocol Mask TLV MUST NOT be included in NLRIs with the other Protocol-IDs listed in Table 2. (§5.3.2.2) | MUST NOT | 5.3.2.2 | **positive:** `unit/verify` [`TestRFC9552NativeDefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L200). **negative:** `unit/verify` [`TestRFC9552NativeIGPMPLSMaskRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L223) |
| `RFC9552-5.3.2.3-1` | If a source protocol uses a metric width of fewer than 32 bits, then the high- order bits of this field MUST be padded with zero. (§5.3.2.3) | MUST | 5.3.2.3 | **positive:** `unit/verify` [`TestRFC7752TEDefaultMetricZeroPadded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L204). **negative:** no negative test. **{single-polarity}:** lsTEDefaultMetric.WriteTo always emits a 4-octet value (internal/component/bgp/plugins/nlri/ls/attr_link.go:226), so a metric sourced from a narrower width lands zero-padded in the high-order octets and ze owns no short-form TE metric encoder whose output could be rejected |
| `RFC9552-5.5-1` | The next-hop address MUST be encoded as described in [RFC4760]. (§5.5) | MUST | 5.5 | **positive:** `unit/verify` [`TestRFC7752BGPLSNextHopFollowsRFC4760`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7752_bgpls_test.go#L25). **negative:** no negative test. **{single-polarity}:** MPReachNLRI.WriteTo is family agnostic and lays out AFI, SAFI, next-hop length, next-hop, the zero reserved octet and then the NLRI for AFI 16388 exactly as RFC 4760 Section 3 specifies (internal/core/bgp/attribute/mpnlri.go:154); ValidNextHopLens returns nil for AFI 16388 (internal/core/bgp/attribute/mpnlri.go:305), so ze runs no BGP-LS next-hop length check and holds no rejection path to drive negatively |
| `RFC9552-5.3-1` | BGP-LS Producers MUST ensure that the TLVs included in the BGP-LS Attribute does not result in a BGP UPDATE message for a single Link-State NLRI that crosses the maximum limit for a BGP message. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the native exporter bounds a BGP-LS Attribute only at the 65535-octet TLV length and never against the maximum UPDATE size of the collector session; internal/component/bgp/plugins/ls_export/export_encode.go::originateAttributes |
| `RFC9552-5.9-1` | If the BGP-LS Producer does withdraw link-state objects associated with an IGP node based on the failure of reachability check for that node, then it MUST re-advertise those link-state objects after that node becomes reachable again in the IGP domain. (§5.9) | MUST | 5.9 | **positive:** `unit/verify` [`TestBGPLSNativeSPFInterAreaASBROrigin`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/bgpls_export_test.go#L738). **positive:** `unit/verify` [`TestBGPLSNativeSPFPartitionRestoresStaleOrigins`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/bgpls_export_test.go#L631). **positive:** `unit/verify` [`TestBGPLSNativeV3SPFPseudonodeRestoration`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/bgpls_export_test.go#L800). **positive:** `unit/verify` [`TestISISBGPLSNativeSPFReachability`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/bgpls_export_test.go#L442). **positive:** `unit/verify` [`TestRFC9552NativeReachabilityWithdrawalAndRestoration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L322). **negative:** `unit/verify` [`TestBGPLSNativeSPFPartitionRestoresStaleOrigins`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/bgpls_export_test.go#L633). **negative:** `unit/verify` [`TestBGPLSNativeSPFUnknownOriginsWaitForComputation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/bgpls_export_test.go#L862). **negative:** `unit/verify` [`TestISISBGPLSNativeSPFReachability`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/bgpls_export_test.go#L443). **negative:** `unit/verify` [`TestRFC9552NativeReachabilityWithdrawalAndRestoration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L323) |
| `RFC9552-5.4-1` | For such TLV use in the NLRI or BGP-LS Attribute, the format described in Section 5.1 is to be used and a 4-octet field MUST be included as the first field in the value to carry the Enterprise Code. (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9552NativePrivateUseWithoutEnterpriseRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L183). **{single-polarity}:** ze originates no private-use TLV, because internal/component/bgp/plugins/ls_export/export_encode.go::originateAttributes refuses every type from 65000, so no emitted private-use TLV exists for a positive case |
| `RFC9552-5.2.3-1` | The IP Prefix field contains an IP address prefix followed by the minimum number of trailing bits needed to make the end of the field fall on an octet boundary. Any trailing bits MUST be set to 0. (§5.2.3.2) | MUST | 5.2.3.2 | **positive:** `unit/verify` [`TestRFC9552NativeMaskedPrefixUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L148). **negative:** `unit/verify` [`TestRFC9552NativeTopologySeparation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L86) |
| `RFC9552-5.3.2.3-2` | IS-IS small metrics are 6 bits in size but are encoded in a 1-octet field; therefore, the two most significant bits of the field MUST be set to 0 by the originator (§5.3.2.4) | MUST | 5.3.2.4 | **positive:** `unit/verify` [`TestRFC9552ISISSmallMetricTwoMSBsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L218). **negative:** `unit/verify` [`TestRFC9552IGPMetricWidthGrowsInsteadOfTruncating`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L252) |
| `RFC9552-8.2.2-1` | A Link-State NLRI MUST NOT be considered malformed or invalid based on the inclusion/exclusion of TLVs or contents of the TLV fields (i.e., semantic errors), as described in Sections 5.1 and 5.2. (§8.2.2) | MUST NOT | 8.2.2 | **positive:** `unit/verify` [`TestRFC9552NLRIContentsNeverMakeItMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L147). **negative:** `unit/verify` [`TestRFC9552NLRIFramingErrorsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L181) |
| `RFC9552-8.2.2-2` | A BGP-LS Attribute MUST NOT be considered malformed or invalid based on the inclusion/exclusion of TLVs or contents of the TLV fields (i.e., semantic errors), as described in Sections 5.1 and 5.3. (§8.2.2) | MUST NOT | 8.2.2 | **positive:** `unit/verify` [`TestRFC9552UnorderedUnexpectedAttributeTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L54). **negative:** `unit/verify` [`TestRFC9552AttributeSyntaxStillRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L95) |
| `RFC9552-8.2.2-3` | A BGP-LS Propagator, even when it has a coexisting BGP-LS Consumer on the same node, should not perform semantic validation of the Link- State NLRI or the BGP-LS Attribute to determine if it is malformed or invalid. (§8.2.2) | SHOULD NOT | 8.2.2 | **positive:** `unit/verify` [`TestRFC9552NLRIContentsNeverMakeItMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L148). **positive:** `unit/verify` [`TestRFC9552UnorderedUnexpectedAttributeTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L55). **negative:** `unit/verify` [`TestRFC9552AttributeSyntaxStillRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L96). **negative:** `unit/verify` [`TestRFC9552NLRIFramingErrorsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L182) |
| `RFC9552-8.2.2-4` | When the error that is determined allows for the router to skip the malformed NLRI(s) and continue the processing of the rest of the BGP UPDATE message (e.g., when the TLV ordering rule is violated), then it MUST handle such malformed NLRIs as 'NLRI discard' (i.e., processing similar to what is described in Section 5.4 of [RFC7606]). (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC9552LinkStateNLRITLVOverrunIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L90). **positive:** `unit/verify` [`TestRFC9552LinkStateNodeDescriptorDuplicateSubTLVIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L128). **negative:** `unit/verify` [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L51) |
| `RFC9552-8.2.2-5` | Alternately, the router MUST perform a 'session reset' when the session is only being used for BGP-LS or if 'AFI/SAFI disable' action is not possible. (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC9552LinkStateNLRILengthOverrunResetsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L204). **negative:** `unit/verify` [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L52) |
| `RFC9552-8.2.2-6` | When the error that is determined allows for the router to skip the malformed BGP-LS Attribute and continue the processing of the rest of the BGP UPDATE message (e.g., when the BGP-LS Attribute length and the total Path Attribute Length are correct but some TLV/sub-TLV length within the BGP-LS Attribute is invalid), then it MUST handle such malformed BGP-LS Attribute as 'Attribute Discard'. (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC9552BGPLSAttributeTLVOverrunDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L121). **positive:** `unit/verify` [`TestRFC9552BGPLSAttributeTrailingOctetsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L162). **negative:** `unit/verify` [`TestRFC9552BGPLSAttributeWellFormedIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L88) |
| `RFC9552-5.3-2` | When a BGP-LS Propagator finds that it is exceeding the maximum BGP message size due to the addition or update of some other BGP Attribute (e.g., AS_PATH), it MUST consider the BGP-LS Attribute to be malformed, apply the 'Attribute Discard' error-handling approach [RFC7606], and handle the propagation as described in Section 8.2.2. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** when a forwarded UPDATE exceeds the destination's maximum message size and the split fails -- which is what a single BGP-LS NLRI larger than the limit produces (internal/component/bgp/message/chunk_mp_nlri.go:133) -- fwdBody logs the failure and drops the whole UPDATE (internal/component/bgp/reactor/forward_body.go:57-60, :95-97). Nothing discards the BGP-LS Attribute first, so the Attribute Discard this clause mandates never happens |
| `RFC9552-8.2.6-1` | An implementation MUST have the means to limit inbound updates (§8.2.6) | MUST | 8.2.6 | **positive:** `unit/verify` [`TestBGPLSPrefixCountCountsNLRIsNotPrefixBytes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_prefix_limit_test.go#L39). **negative:** `unit/verify` [`TestBGPLSPrefixLimitActuallyFires`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_prefix_limit_test.go#L100) |
| `RFC9552-5.2.2-6` | If the value in the MT-ID TLV is derived from OSPF, then the upper R bits of the MT-ID field MUST be set to 0 and only the values from 0 to 127 are valid for the MT-ID (§5.2.2.1) | MUST | 5.2.2.1 | **positive:** `unit/verify` [`TestRFC9552NativeOSPFTopologyBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L111). **negative:** `unit/verify` [`TestRFC9552NativeOSPFTopologyBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L112) |
| `RFC9552-5.1-7` | a BGP-LS Consumer MUST NOT be able to send information to a BGP Speaker for origination into BGP-LS. (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 9552 Section 5.1 defines the BGP-LS Consumer as an application or process that is not a BGP Speaker. Ze's native topology exporter is a BGP-LS Producer, not a Consumer feeding received BGP-LS information back into Producers or Propagators; the selected implementation does not provide that Consumer role |
| `RFC9552-5.2.1.1-1` | The same node MUST NOT be represented by two keys (§5.2.1.1) | MUST NOT | 5.2.1.1 | **positive:** `unit/verify` [`TestSameNodeHasOneKeyWhateverTheStorageOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/types_descriptor_key_test.go#L186). **negative:** `unit/verify` [`TestNodeDescriptorWriteToMatchesBytes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/types_descriptor_key_test.go#L156) |
| `RFC9552-5.2.1.1-2` | Two different nodes MUST NOT be represented by the same key (§5.2.1.1) | MUST NOT | 5.2.1.1 | **positive:** `unit/verify` [`TestNodeDescriptorEncodesLegalZeroKeyFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/types_descriptor_key_test.go#L20). **negative:** `unit/verify` [`TestNodeDescriptorKeepsBackboneDistinctFromAreaLess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/types_descriptor_key_test.go#L57) |
| `RFC9552-5.2.2.1-1` | When used as a Link or Prefix Descriptor for IS-IS, the Bits R are reserved and MUST be set to 0 (as per Section 7.2 of [RFC5120]) when originated and ignored on receipt. (§5.2.2.1) | MUST | 5.2.2.1 | **positive:** `unit/verify` [`TestRFC9552ISISMTIDReservedBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L314). **negative:** `unit/verify` [`TestRFC9552NativeISISTopologyReservedBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L161) |
| `RFC9552-8.2.2-9` | A BGP-LS Speaker MUST perform the following syntactic validation of the Link-State NLRI to determine if it is malformed. (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC9552LinkStateNLRILengthOverrunResetsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L203). **positive:** `unit/verify` [`TestRFC9552LinkStateNLRIOutOfOrderTLVsAreDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L164). **positive:** `unit/verify` [`TestRFC9552LinkStateNLRITLVOverrunIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L89). **positive:** `unit/verify` [`TestRFC9552LinkStateNodeDescriptorDuplicateSubTLVIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L126). **negative:** `unit/verify` [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L50) |
| `RFC9552-8.2.2-10` | A BGP-LS Speaker MUST perform the following syntactic validation of the BGP-LS Attribute to determine if it is malformed. (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC9552BGPLSAttributeTLVOverrunDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L120). **positive:** `unit/verify` [`TestRFC9552BGPLSAttributeTrailingOctetsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L161). **negative:** `unit/verify` [`TestRFC9552BGPLSAttributeWellFormedIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L87) |
| `RFC9552-8.2.3-5` | An implementation MUST allow the operator to configure an 8-octet BGP-LS Instance-ID (§8.2.3) | MUST | 8.2.3 | **positive:** `unit/verify` [`TestRFC9552NativeInstanceIDFullWidth`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L277). **negative:** `unit/verify` [`TestRFC9552NativeInstanceIDOverflowRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L174) |
| `RFC9552-8.2.6-2` | An operator MUST define an import policy to limit inbound updates as follows: * Drop all updates from peers that are only serving BGP-LS Consumers. (§8.2.6) | MUST | 8.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the sentence binds the operator, not the implementation -- Section 8.2.6 reads "An operator MUST define an import policy to limit inbound updates", and assigns the implementation's own share to the next sentence, "An implementation MUST have the means to limit inbound updates", which this summary gates separately as RFC9552-8.2.6-1. ze provides the means this policy needs: a bgp/policy/family-filter instance naming the bgp-ls family with action remove, referenced from a peer's import chain, rejects every BGP-LS UPDATE that peer sends (parseFamilyFilters and handleFilterUpdate, internal/component/bgp/plugins/filter_family/config.go:30, handler.go:49). Which peers only serve BGP-LS Consumers is knowledge ze does not hold and cannot derive. Disclosed in docs/features/rfc-status.md |
| `RFC9552-5.2.2.1-2` | In case one wants to advertise multiple topologies for a given Link or Prefix Descriptor, multiple NLRIs MUST be generated where each NLRI contains a single unique MT-ID (§5.2.2.1) | MUST | 5.2.2.1 | **positive:** `unit/verify` [`TestRFC9552NativeTopologySeparation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L83). **negative:** `unit/verify` [`TestRFC9552NativeTopologyReplacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L146) |
| `RFC9552-5.2.3-2` | The Multi-Topology Identifier TLV MUST be included in the Prefix Descriptor if the underlying IGP prefix object is associated with a non-default topology (§5.2.3) | MUST | 5.2.3 | **positive:** `unit/verify` [`TestRFC9552NativeTopologySeparation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L84). **negative:** `unit/verify` [`TestRFC9552NativeTopologyReplacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L147) |
| `RFC9552-5.3.1.1-1` | The bits that are not defined MUST be set to 0 by the originator and MUST be ignored by the receiver. (§5.3.1.1) | MUST | 5.3.1.1 | **positive:** `unit/verify` [`TestRFC9552NativeDefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L197). **positive:** `unit/verify` [`TestRFC9552NodeFlagBitsUndefinedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_flags_rfc9552_test.go#L61). **negative:** `unit/verify` [`TestRFC9552NativeReservedFlagsCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L210). **negative:** `unit/verify` [`TestRFC9552UndefinedNodeBitsDoNotSetDefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_flags_rfc9552_test.go#L126) |
| `RFC9552-5.3.1.5-1` | In the case of OSPF, this TLV MUST NOT be used to advertise TLVs other than those in the OSPF Router Information (RI) LSA [RFC7770]. (§5.3.1.5) | MUST NOT | 5.3.1.5 | **positive:** `unit/verify` [`TestRFC9552NativeOpaqueNodeProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L51). **negative:** `unit/verify` [`TestRFC9552NativeOpaqueNodeProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L52) |
| `RFC9552-5.3.2.2-3` | The bits that are not defined MUST be set to 0 by the originator and MUST be ignored by the receiver. (§5.3.2.2) | MUST | 5.3.2.2 | **positive:** `unit/verify` [`TestRFC9552MPLSProtocolMaskDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_flags_rfc9552_test.go#L154). **positive:** `unit/verify` [`TestRFC9552NativeDefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L199). **negative:** `unit/verify` [`TestRFC9552MPLSProtocolMaskReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_flags_rfc9552_test.go#L164). **negative:** `unit/verify` [`TestRFC9552NativeReservedFlagsCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L212) |
| `RFC9552-5.3.2.6-1` | In the case of OSPFv2, this TLV MUST NOT be used to advertise information carried using TLVs other than those in the OSPFv2 Extended Link Opaque LSA [RFC7684]. (§5.3.2.6) | MUST NOT | 5.3.2.6 | **positive:** `unit/verify` [`TestRFC9552NativeOpaqueOSPFv2LinkProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L59). **negative:** `unit/verify` [`TestRFC9552NativeOpaqueOSPFv2LinkProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L60) |
| `RFC9552-5.3.2.6-2` | In the case of OSPFv3, this TLV MUST NOT be used to advertise TLVs other than those in the OSPFv3 E- Router-LSA or E-Link-LSA [RFC8362]. (§5.3.2.6) | MUST NOT | 5.3.2.6 | **positive:** `unit/verify` [`TestRFC9552NativeOpaqueOSPFv3LinkProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L67). **negative:** `unit/verify` [`TestRFC9552NativeOpaqueOSPFv3LinkProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L68) |
| `RFC9552-5.3.3.1-1` | The bits that are not defined MUST be set to 0 by the originator and MUST be ignored by the receiver. (§5.3.3.1) | MUST | 5.3.3.1 | **positive:** `unit/verify` [`TestRFC9552IGPFlagsUndefinedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_flags_rfc9552_test.go#L98). **positive:** `unit/verify` [`TestRFC9552NativeDefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L198). **negative:** `unit/verify` [`TestRFC9552NativeReservedFlagsCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L211). **negative:** `unit/verify` [`TestRFC9552UndefinedIGPBitsDoNotSetDefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_flags_rfc9552_test.go#L140) |
| `RFC9552-5.3.3.6-1` | In the case of OSPFv2, this TLV MUST NOT be used to advertise information carried using TLVs other than those in the OSPFv2 Extended Prefix Opaque LSA [RFC7684]. (§5.3.3.6) | MUST NOT | 5.3.3.6 | **positive:** `unit/verify` [`TestRFC9552NativeOpaqueOSPFv2PrefixProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L75). **negative:** `unit/verify` [`TestRFC9552NativeOpaqueOSPFv2PrefixProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L76) |
| `RFC9552-5.3.3.6-2` | In the case of OSPFv3, this TLV MUST NOT be used to advertise TLVs other than those in the OSPFv3 E-Inter-Area-Prefix-LSA, E-Intra-Area-Prefix-LSA, E-AS-External-LSA, and E-NSSA-LSA [RFC8362]. (§5.3.3.6) | MUST NOT | 5.3.3.6 | **positive:** `unit/verify` [`TestRFC9552NativeOpaqueOSPFv3PrefixProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L83). **negative:** `unit/verify` [`TestRFC9552NativeOpaqueOSPFv3PrefixProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L84) |
| `RFC9552-5.4-2` | For a private use NLRI type, a 4-octet field MUST be included as the first field in the NLRI immediately following the Total NLRI Length field of the Link-State NLRI format as described in Section 5.2 to carry the Enterprise Code [ENTNUM]. (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates no private-use NLRI type: internal/component/bgp/plugins/ls_export/export_encode.go::encodeTopology builds only Node (1), Link (2), IPv4 and IPv6 Prefix (3, 4) and SRv6 SID (6) NLRIs |
| `RFC9552-5.1-8` | The TLVs within the BGP-LS Attribute SHOULD be ordered in ascending order by TLV type. (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.2-9` | The 'Direct' and 'Static configuration' protocol types SHOULD be used when BGP-LS is sourcing local information. (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.2.1.4-3` | The BGP-LS Identifier was introduced by [RFC7752], and its use is being deprecated by this document. Implementations SHOULD support the advertisement of this sub-TLV for backward compatibility in deployments where there are BGP-LS Producer implementations that conform to [RFC7752] to ensure consistency of NLRI encoding for link- state objects. (§5.2.1.4) | SHOULD | 5.2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.3-3` | This attribute SHOULD only be included with Link-State NLRIs. (§5.3) | SHOULD | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.2.3.2-1` | A router SHOULD advertise an IP Prefix NLRI for each of its BGP next hops (§5.2.3.2) | SHOULD | 5.2.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.3.1.3-1` | This symbolic name can be the Fully Qualified Domain Name (FQDN) for the router, a substring of the FQDN (e.g., a hostname), or any string that an operator wants to use for the router. The use of the FQDN or a substring of it is strongly RECOMMENDED. (§5.3.1.3) | RECOMMENDED | 5.3.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.3.2.2-2` | Generation of the MPLS Protocol Mask TLV is only valid for and SHOULD only be used with originators that have local link insight, for example, the Protocol-IDs 'Static configuration' or 'Direct' as per Table 2. (§5.3.2.2) | SHOULD | 5.3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.5-2` | If an IPv4 BGP session is used, then the next hop in the MP_REACH_NLRI SHOULD be an IPv4 address. Similarly, if an IPv6 BGP session is used, then the next hop in the MP_REACH_NLRI SHOULD be an IPv6 address. (§5.5) | SHOULD | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.6-1` | In other cases, an implementation SHOULD provide a means to inject inter-AS links into BGP-LS. (§5.6) | SHOULD | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.1.5-1` | Distribution of Link-State NLRIs SHOULD be limited to a single admin domain, which can consist of multiple areas within an AS or multiple ASes. (§8.1.5) | SHOULD | 8.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.9-2` | A BGP-LS Producer SHOULD withdraw all link-state objects advertised by it in BGP when the node that originated its corresponding LSPs/ LSAs is determined to have become unreachable in the IGP. (§5.9) | SHOULD | 5.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.2.2-7` | When a BGP Speaker receives an UPDATE message with Link-State NLRI(s) in the MP_REACH_NLRI but without the BGP-LS Attribute, it is most likely an indication that a BGP Speaker preceding it has performed the 'Attribute Discard' fault handling. An implementation SHOULD preserve and propagate the Link-State NLRIs, unless denied by local policy, in such an UPDATE message so that the BGP-LS Consumers can detect the loss of link-state information for that object and not assume its deletion/withdrawal. (§8.2.2) | SHOULD | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.2.2-8` | An implementation SHOULD log a message for any errors found during syntax validation for further analysis. (§8.2.2) | SHOULD | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.2.2-11` | In other cases, where the error in the NLRI encoding results in the inability to process the BGP UPDATE message (e.g., length-related encoding errors), then the router SHOULD handle such malformed NLRIs as 'AFI/SAFI disable' when other AFI/SAFI besides BGP-LS are being advertised over the same session. (§8.2.2) | SHOULD | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.2.3-1` | An implementation SHOULD allow the operator to specify neighbors to which Link-State NLRIs will be advertised and from which Link-State NLRIs will be accepted. (§8.2.3) | SHOULD | 8.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.2.3-2` | An implementation SHOULD allow the operator to specify the maximum rate at which Link-State NLRIs will be advertised/withdrawn from neighbors. (§8.2.3) | SHOULD | 8.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.2.3-3` | An implementation SHOULD allow the operator to specify the maximum number of Link-State NLRIs stored in a router's Routing Information Base (RIB). (§8.2.3) | SHOULD | 8.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.2.3-4` | An implementation SHOULD allow the operator to create abstracted topologies that are advertised to neighbors and create different abstractions for different neighbors. (§8.2.3) | SHOULD | 8.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.2.3-6` | An implementation SHOULD allow the operator to configure Autonomous System Number (ASN) and BGP-LS identifiers (refer to Section 5.2.1.4). (§8.2.3) | SHOULD | 8.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.2.3-7` | An implementation SHOULD allow the operator to configure a 4096-byte size limit for a BGP-LS UPDATE message on a BGP-LS Producer or allow larger values when they know that all BGP-LS Speakers support the extended message size [RFC8654]. (§8.2.3) | SHOULD | 8.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.2-10` | The BGP-LS Instance-ID 0 is RECOMMENDED to be used when there is only a single protocol instance in the network where BGP-LS is operational. (§5.2) | RECOMMENDED | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.2.1.4-4` | The default value of 0 is RECOMMENDED to be used when a BGP-LS Producer includes this sub-TLV when originating information into BGP-LS. (§5.2.1.4) | RECOMMENDED | 5.2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.3-4` | It is RECOMMENDED that implementations support the extended message size for BGP [RFC8654] to accommodate a larger size of information within the BGP-LS Attribute. (§5.3) | RECOMMENDED | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.1.1-1` | It is RECOMMENDED that operators deploying BGP-LS enable two or more BGP-LS Producers in each IGP flooding domain to achieve redundancy in the origination of link-state information into BGP-LS. (§8.1.1) | RECOMMENDED | 8.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.1.1-2` | Distribution of the BGP-LS NLRIs SHOULD be handled by dedicated route reflectors in most deployments providing a level of isolation and fault containment between different BGP address families. (§8.1.1) | SHOULD | 8.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.1.1-3` | In the event of dedicated route reflectors not being available, other alternate mechanisms like separation of BGP instances or separate BGP sessions (e.g., using different addresses for peering) for Link-State information distribution SHOULD be used. (§8.1.1) | SHOULD | 8.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.2.2-7` | An implementation MAY suppress the advertisement of a Link NLRI, corresponding to a half-link, from a link-state IGP unless the IGP has verified that the link is being reported in the IS-IS LSP or OSPF Router LSA by both the nodes connected by that link. (§5.2.2) | MAY | 5.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.2.1-2` | When configured, these auxiliary TE Router-IDs (TLV 1028/1029) MUST be included in the node attribute described in Section 5.3.1 and MAY be included in the link attribute described in Section 5.3.2. (§5.2.1) | MAY | 5.2.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9552-5.2-4`](#rfc9552-5.2-4) The network operator MUST assign the same BGP-LS Instance-IDs on all BGP-LS Producers within a given IGP domain (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: the sentence binds the network operator, who assigns Instance-IDs across Producers; ze carries the per-domain value the operator configures unchanged into the NLRI (internal/component/bgp/plugins/ls_export/export_config.go::parseExportConfig) and cannot see which IGP domain another Producer reports |
| [`RFC9552-5.2-5`](#rfc9552-5.2-5) Unique BGP-LS Instance-IDs MUST be assigned to routing protocol instances operating in different IGP domains (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: the sentence binds the network operator, who assigns Instance-IDs to IGP domains; ze carries the per-domain value the operator configures unchanged into the NLRI (internal/component/bgp/plugins/ls_export/export_config.go::parseExportConfig) and cannot see the domains of other routers |
| [`RFC9552-5.3-1`](#rfc9552-5.3-1) BGP-LS Producers MUST ensure that the TLVs included in the BGP-LS Attribute does not result in a BGP UPDATE message for a single Link-State NLRI that crosses the maximum limit for a BGP message. (§5.3) | {gap}, no test | the native exporter bounds a BGP-LS Attribute only at the 65535-octet TLV length and never against the maximum UPDATE size of the collector session; internal/component/bgp/plugins/ls_export/export_encode.go::originateAttributes |
| [`RFC9552-5.3-2`](#rfc9552-5.3-2) When a BGP-LS Propagator finds that it is exceeding the maximum BGP message size due to the addition or update of some other BGP Attribute (e.g., AS_PATH), it MUST consider the BGP-LS Attribute to be malformed, apply the 'Attribute Discard' error-handling approach [RFC7606], and handle the propagation as described in Section 8.2.2. (§5.3) | {gap}, no test | when a forwarded UPDATE exceeds the destination's maximum message size and the split fails -- which is what a single BGP-LS NLRI larger than the limit produces (internal/component/bgp/message/chunk_mp_nlri.go:133) -- fwdBody logs the failure and drops the whole UPDATE (internal/component/bgp/reactor/forward_body.go:57-60, :95-97). Nothing discards the BGP-LS Attribute first, so the Attribute Discard this clause mandates never happens |
| [`RFC9552-5.1-7`](#rfc9552-5.1-7) a BGP-LS Consumer MUST NOT be able to send information to a BGP Speaker for origination into BGP-LS. (§3) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 9552 Section 5.1 defines the BGP-LS Consumer as an application or process that is not a BGP Speaker. Ze's native topology exporter is a BGP-LS Producer, not a Consumer feeding received BGP-LS information back into Producers or Propagators; the selected implementation does not provide that Consumer role |
| [`RFC9552-8.2.6-2`](#rfc9552-8.2.6-2) An operator MUST define an import policy to limit inbound updates as follows: * Drop all updates from peers that are only serving BGP-LS Consumers. (§8.2.6) | no test | no test carries this requirement id; annotated {not-applicable}: the sentence binds the operator, not the implementation -- Section 8.2.6 reads "An operator MUST define an import policy to limit inbound updates", and assigns the implementation's own share to the next sentence, "An implementation MUST have the means to limit inbound updates", which this summary gates separately as RFC9552-8.2.6-1. ze provides the means this policy needs: a bgp/policy/family-filter instance naming the bgp-ls family with action remove, referenced from a peer's import chain, rejects every BGP-LS UPDATE that peer sends (parseFamilyFilters and handleFilterUpdate, internal/component/bgp/plugins/filter_family/config.go:30, handler.go:49). Which peers only serve BGP-LS Consumers is knowledge ze does not hold and cannot derive. Disclosed in docs/features/rfc-status.md |
| [`RFC9552-5.4-2`](#rfc9552-5.4-2) For a private use NLRI type, a 4-octet field MUST be included as the first field in the NLRI immediately following the Total NLRI Length field of the Link-State NLRI format as described in Section 5.2 to carry the Enterprise Code [ENTNUM]. (§5.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates no private-use NLRI type: internal/component/bgp/plugins/ls_export/export_encode.go::encodeTopology builds only Node (1), Link (2), IPv4 and IPv6 Prefix (3, 4) and SRv6 SID (6) NLRIs |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9552-5.1-1`](#rfc9552-5.1-1)

To compare NLRIs with unknown TLVs, all TLVs within the NLRI MUST be ordered in ascending order by TLV Type. (§5.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. re-audit RA12: the units drive only the nlri/ls LinkDescriptor/NodeDescriptor/PrefixDescriptor encoders; ze's native producer builds Link NLRI descriptors itself in ls_export export_encode.go::encodeNativeLink (own slices.SortFunc over 258-263), and no 5.1-1 tag drives that output, so a descending pair from the native Link path stays green

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNoDescriptorEmitsADescendingTLVSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L73) | unit/verify | unproven |
| positive | [`TestLinkDescriptorOrdersMixedFamilyAddressesAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L39) | unit/verify | unproven |

### [`RFC9552-5.1-2`](#rfc9552-5.1-2)

If there are multiple TLVs of the same type within a single NLRI, then the TLVs sharing the same type MUST be first in ascending order based on the Length field followed by ascending order based on the Value field. (§5.1)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. re-audit RA12: both units build a NodeDescriptor carrying three (or two) sub-TLV 518 and assert all are emitted inside the Node Descriptor, which RFC 9552 5.2.1.4 forbids (at most one instance of each sub-TLV type) and RFC 9514 places 518 at NLRI level, not in the descriptor; the one real repeated-type producer, encodeNativeLink emitting several 259/260/261/262 for multi-address links, is driven by no 5.1-2 tag

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSRv6SIDOrderIsLengthBeforeValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L170) | unit/verify | unproven |
| positive | [`TestNodeDescriptorOrdersRepeatedSRv6SIDs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L124) | unit/verify | unproven |

### [`RFC9552-5.1-3`](#rfc9552-5.1-3)

Unknown and unsupported types MUST be preserved and propagated within both the NLRI and the BGP-LS Attribute. (§5.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. attribute half is proven only by AttrTLVsToJSON keeping the bytes, not by the attribute being propagated on a forwarded UPDATE; the NLRI half (byte-identical re-encode) holds; the negative (length overrun refused) exercises 8.2.2 syntax validation, not this preserve-and-propagate rule

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7752MalformedTLVNotPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L103) | unit/verify | unproven |
| positive | [`TestRFC7752UnknownTLVPreservedAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L65) | unit/verify | unproven |

### [`RFC9552-5.1-4`](#rfc9552-5.1-4)

The presence of unknown or unexpected TLVs MUST NOT result in the NLRI or the BGP-LS Attribute being considered malformed. (§5.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. tagged units cover only the BGP-LS Attribute half at decoder level (iterateAttrTLVs/decodeAllAttrTLVs); the NLRI half of the sentence is not under a 5.1-4 tag, although reactor TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated proves it on the receive path and could carry the tag

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552AttributeSyntaxStillRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L94) | unit/verify | unproven |
| positive | [`TestRFC9552UnorderedUnexpectedAttributeTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L53) | unit/verify | unproven |

### [`RFC9552-5.1-5`](#rfc9552-5.1-5)

NLRIs having TLVs that do not follow the above ordering rules MUST be considered as malformed by a BGP-LS Propagator. (§5.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. receive path discards only a top-level descending pair (257 before 256); bgplsTLVOrdered also enforces the same-type Length/Value rule, which no tagged unit drives, so "the above ordering rules" is proven for one of its two clauses

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L53) | unit/verify | unproven |
| positive | [`TestRFC9552LinkStateNLRIOutOfOrderTLVsAreDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L165) | unit/verify | unproven |

### [`RFC9552-5.1-6`](#rfc9552-5.1-6)

BGP-LS Attribute with unordered TLVs MUST NOT be considered malformed (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552AttributeSyntaxStillRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L93) | unit/verify | unproven |
| positive | [`TestRFC9552UnorderedUnexpectedAttributeTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L52) | unit/verify | unproven |

### [`RFC9552-5.2-1`](#rfc9552-5.2-1)

All non-VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 71 (§5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7752NonLinkStateFamilyRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L267) | unit/verify | unproven |
| positive | [`TestRFC7752NonVPNFamilyIsAFI16388SAFI71`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L230) | unit/verify | unproven |

### [`RFC9552-5.2-2`](#rfc9552-5.2-2)

VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 72 (§5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7752NonLinkStateFamilyRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L268) | unit/verify | unproven |
| positive | [`TestRFC9552BGPLSVPNNLRIFramedByLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9552_bgpls_test.go#L135) | unit/verify | unproven |
| positive | [`TestRFC7752VPNFamilyIsAFI16388SAFI72`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L248) | unit/verify | unproven |

### [`RFC9552-5.2-3`](#rfc9552-5.2-3)

For all information derived from other protocols, the corresponding Protocol-ID MUST be used (§5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeProtocolIDFollowsSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L38) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeProtocolIDFollowsSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L25) | unit/verify | revert, verified |

### [`RFC9552-5.2-4`](#rfc9552-5.2-4)

The network operator MUST assign the same BGP-LS Instance-IDs on all BGP-LS Producers within a given IGP domain (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.2-4, so no unit is bound to it.

### [`RFC9552-5.2-5`](#rfc9552-5.2-5)

Unique BGP-LS Instance-IDs MUST be assigned to routing protocol instances operating in different IGP domains (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.2-5, so no unit is bound to it.

### [`RFC9552-5.2-6`](#rfc9552-5.2-6)

When adding, removing, or modifying a TLV/sub-TLV from a Link-State NLRI, the BGP-LS Producer MUST withdraw the old NLRI by including it in the MP_UNREACH_NLRI. (§5.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. R-7: drives only a modified address descriptor; adding or removing a TLV/sub-TLV is never driven, and the assertion is a " del " engine command, not MP_UNREACH_NLRI on the wire; negative (attribute-only change, no withdraw) is sound

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeDescriptorChangeWithdrawsFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeDescriptorChangeWithdrawsFirst`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L60) | unit/verify | revert, verified |

### [`RFC9552-5.2-7`](#rfc9552-5.2-7)

For two BGP Speakers to exchange Link-State NLRI, they MUST use BGP Capabilities Advertisement to ensure that they are both capable of properly processing such NLRI. (§5.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. asserts the Multiprotocol capability encoding and that Negotiate intersects families; nothing drives the UPDATE send or receive path, so ze sending Link-State NLRI to a peer that did not negotiate it would leave both units green; negative tag prose claims "never exchanged", wider than its assertion

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7752BGPLSCapabilityNotNegotiatedWhenPeerSilent`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7752_bgpls_test.go#L63) | unit/verify | unproven |
| positive | [`TestRFC7752BGPLSCapabilityAdvertisedAndNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7752_bgpls_test.go#L29) | unit/verify | unproven |

### [`RFC9552-5.2-8`](#rfc9552-5.2-8)

An implementation MUST handle unknown Link-State NLRI types as opaque objects and MUST preserve and propagate them (§5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552MalformedBGPLSNLRIRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9552_bgpls_test.go#L90) | unit/verify | unproven |
| positive | [`TestRFC9552BGPLSVPNNLRIFramedByLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9552_bgpls_test.go#L136) | unit/verify | unproven |
| positive | [`TestRFC9552UnknownBGPLSNLRITypeIsOpaque`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9552_bgpls_test.go#L46) | unit/verify | unproven |
| positive | [`rfc9552-52-rs-opaque-withdraw-peer-down.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc9552-52-rs-opaque-withdraw-peer-down.ci#L3) | functional/verify | unproven |

### [`RFC9552-5.2.1.4-1`](#rfc9552-5.2.1.4-1)

At most, there MUST be one instance of each sub-TLV type present in any Node Descriptor. (§5.2.1.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. re-audit RA12: receive half proven (duplicate sub-TLV 512 NLRI discarded, sibling survives; distinct types kept); sender half unproven and the code violates it: NodeDescriptor.WriteTo (nlri/ls/types_descriptor.go) emits one sub-TLV 518 per SRv6SIDs entry inside the Node Descriptor with no refusal, and the RFC9552-5.1-2 units assert that repeated emission

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L54) | unit/verify | unproven |
| positive | [`TestRFC9552LinkStateNodeDescriptorDuplicateSubTLVIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L127) | unit/verify | unproven |

### [`RFC9552-5.2.1.4-2`](#rfc9552-5.2.1.4-2)

The sub-TLVs within a Node Descriptor MUST be arranged in ascending order by sub-TLV type. (§5.2.1.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity positive per row annotation; asserts the exact ascending sub-TLV sequence 512..517 from NodeDescriptor.WriteTo

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7752NodeDescriptorSubTLVsAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L165) | unit/verify | unproven |

### [`RFC9552-5.2.2-1`](#rfc9552-5.2.2-1)

If interface and neighbor addresses, either IPv4 or IPv6, are present, then the interface/neighbor address TLVs MUST be included (§5.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. re-audit RA12: positives pin 259/262 and 261/260 exactly for links with global addresses, so omission fails; but the negative tag drives an invalid (zero) address being refused, a neighbouring input-validation rule, not an omitted address TLV, and the row carries no {single-polarity} marker

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeLinkDescriptorComplements`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L101) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeLinkIdentitySelection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L242) | unit/verify | revert, verified |

### [`RFC9552-5.2.2-2`](#rfc9552-5.2.2-2)

If interface and neighbor addresses, either IPv4 or IPv6, are present, then the interface/neighbor address TLVs MUST be included, and the Link Local/Remote Identifiers TLV MUST NOT be included in the Link Descriptor. (§5.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. negative asserts TLV 258 absent from a link carrying global addresses and link IDs; positive asserts the link is still advertised by 261/260

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeLinkIdentitySelection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L243) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeLinkDescriptorComplements`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L97) | unit/verify | revert, verified |

### [`RFC9552-5.2.2-3`](#rfc9552-5.2.2-3)

IPv4/IPv6 link-local addresses MUST NOT be carried in the IPv4/IPv6 interface/neighbor address TLVs (259/260/261/262) as descriptors of a link since they are not considered unique. (§5.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. negative asserts link-local 169.254/fe80 addresses absent from 259-262 (exact single global values), and a link-local-only link carries none of 259-262; positive pins the global addresses once each

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeLinkIdentitySelection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L244) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeLinkDescriptorComplements`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L98) | unit/verify | revert, verified |

### [`RFC9552-5.2.2-4`](#rfc9552-5.2.2-4)

If interface and neighbor addresses are not present and the link local/remote identifiers are present, then the Link Local/Remote Identifiers TLV MUST be included in the Link Descriptor. The Link Local/Remote identifiers MUST be included in the Link Descriptor and in the case of links having only IPv6 link-local addressing on them. (§5.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. re-audit RA12: second sentence proven (link with only link-local 169.254/fe80 addressing emits TLV 258 with ids 0/8 and none of 259-262); first clause, a link with NO interface/neighbor addresses and link ids present, is never driven, so no assertion goes red on that input

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9552NativeLinkIdentitySelection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L245) | unit/verify | revert, verified |

### [`RFC9552-5.2.2-5`](#rfc9552-5.2.2-5)

The Multi-Topology Identifier TLV MUST be included as a Link Descriptor if the underlying IGP link object is associated with a non-default topology. (§5.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. positive asserts TLV 263 MT-ID 2 on a topology-2 link; the negative loops over the same single captured command and re-asserts the same 263 value, so the pair is one assertion wearing two hats and carries no single-polarity annotation

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeLinkNonDefaultTopology`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L122) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeLinkNonDefaultTopology`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L120) | unit/verify | revert, verified |

### [`RFC9552-5.2.3.1-1`](#rfc9552-5.2.3.1-1)

The OSPF Route Type TLV MUST be included in the advertisement when the type is either being signaled explicitly in the underlying LSA or can be determined via another LSA for the same prefix when it is not signaled explicitly (e.g., in the case of OSPFv2 Extended Prefix Opaque LSA [RFC7684]). (§5.2.3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. re-audit RA12: first clause proven only for one explicit type (OSPFv2 route type 1 -> TLV 264 value 1 on the ls_export NLRI); the second clause, a type not signaled explicitly but determinable via another LSA (Extended Prefix route type 0 or AS/NSSA external 1-vs-2, resolved by ospf/bgpls_export.go::v2PrefixRouteTypes), is driven by no tagged unit, and the {single-polarity} argument that zero means unsignaled ignores exactly that clause

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9552NativeOSPFRouteType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L134) | unit/verify | revert, verified |

### [`RFC9552-5.2.1-1`](#rfc9552-5.2.1-1)

When configured, these auxiliary TE Router-IDs (TLV 1028/1029) MUST be included in the node attribute described in Section 5.3.1 (§5.2.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. asserts only IPv4 TE Router-ID (IS-IS TLV 134 -> 1028) on the linkstateevents snapshot; the IPv6 path (TLV 140 -> 1029, isis/bgpls_export.go) and the OSPF producer (ospf/bgpls_export.go:187) are untested, and the unit stops before the BGP-LS attribute wire

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9552ISISNodeRouterID`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/bgpls_export_rfc9552_test.go#L49) | unit/verify | revert, verified |

### [`RFC9552-5.3.2.1-1`](#rfc9552-5.3.2.1-1)

All auxiliary Router-IDs of both the local and the remote node MUST be included in the link attribute of each Link NLRI. (§5.3.2.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. asserts IPv4 local 1028 and remote 1030 on IS-IS links (negative catches a swap); the IPv6 1029/1031 path produced by bgplsBuilder.linkRouterIDs and the OSPF producer are untested, so "all auxiliary Router-IDs" is proven for one type

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552ISISLinkRouterIDs`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/bgpls_export_rfc9552_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestRFC9552ISISLinkRouterIDs`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/bgpls_export_rfc9552_test.go#L72) | unit/verify | revert, verified |

### [`RFC9552-5.3.2.2-1`](#rfc9552-5.3.2.2-1)

The MPLS Protocol Mask TLV MUST NOT be included in NLRIs with the other Protocol-IDs listed in Table 2. (§5.3.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. negative refuses an IS-IS replacement carrying TLV 1094 and keeps the prior state; positive shows a Direct link still carries the mask; OSPF Protocol-IDs 3 and 6 are not driven

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeIGPMPLSMaskRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L223) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeDefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L200) | unit/verify | revert, verified |

### [`RFC9552-5.3.2.3-1`](#rfc9552-5.3.2.3-1)

If a source protocol uses a metric width of fewer than 32 bits, then the high- order bits of this field MUST be padded with zero. (§5.3.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity positive per row annotation; TE Default Metric always 4 octets with zero high-order octets for narrow metrics

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7752TEDefaultMetricZeroPadded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L204) | unit/verify | unproven |

### [`RFC9552-5.5-1`](#rfc9552-5.5-1)

The next-hop address MUST be encoded as described in [RFC4760]. (§5.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity positive per row annotation; exact AFI/SAFI/NH-len/NH/reserved/NLRI layout for IPv4 and IPv6 next hops

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7752BGPLSNextHopFollowsRFC4760`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7752_bgpls_test.go#L25) | unit/verify | unproven |

### [`RFC9552-5.3-1`](#rfc9552-5.3-1)

BGP-LS Producers MUST ensure that the TLVs included in the BGP-LS Attribute does not result in a BGP UPDATE message for a single Link-State NLRI that crosses the maximum limit for a BGP message. (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.3-1, so no unit is bound to it.

### [`RFC9552-5.9-1`](#rfc9552-5.9-1)

If the BGP-LS Producer does withdraw link-state objects associated with an IGP node based on the failure of reachability check for that node, then it MUST re-advertise those link-state objects after that node becomes reachable again in the IGP domain. (§5.9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. ls_export unit withdraws the unreachable origin and re-announces the identical commands once Unreachable clears, and emits nothing while it stays unreachable; IS-IS and OSPF units prove the source restores eligibility only after completed SPF; assertions are engine commands, not wire

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeReachabilityWithdrawalAndRestoration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L323) | unit/verify | revert, verified |
| negative | [`TestISISBGPLSNativeSPFReachability`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/bgpls_export_test.go#L443) | unit/verify | unproven |
| negative | [`TestBGPLSNativeSPFPartitionRestoresStaleOrigins`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/bgpls_export_test.go#L633) | unit/verify | unproven |
| negative | [`TestBGPLSNativeSPFUnknownOriginsWaitForComputation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/bgpls_export_test.go#L862) | unit/verify | unproven |
| positive | [`TestRFC9552NativeReachabilityWithdrawalAndRestoration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L322) | unit/verify | revert, verified |
| positive | [`TestISISBGPLSNativeSPFReachability`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/bgpls_export_test.go#L442) | unit/verify | unproven |
| positive | [`TestBGPLSNativeSPFInterAreaASBROrigin`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/bgpls_export_test.go#L738) | unit/verify | unproven |
| positive | [`TestBGPLSNativeSPFPartitionRestoresStaleOrigins`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/bgpls_export_test.go#L631) | unit/verify | unproven |
| positive | [`TestBGPLSNativeV3SPFPseudonodeRestoration`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/bgpls_export_test.go#L800) | unit/verify | unproven |

### [`RFC9552-5.4-1`](#rfc9552-5.4-1)

For such TLV use in the NLRI or BGP-LS Attribute, the format described in Section 5.1 is to be used and a 4-octet field MUST be included as the first field in the value to carry the Enterprise Code. (§5.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity negative per row annotation; a private-use 65000 node attribute is refused with no UPDATE; the refusal is of every private type, not of a short Enterprise Code

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativePrivateUseWithoutEnterpriseRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L183) | unit/verify | revert, verified |

### [`RFC9552-5.2.3-1`](#rfc9552-5.2.3-1)

The IP Prefix field contains an IP address prefix followed by the minimum number of trailing bits needed to make the end of the field fall on an octet boundary. Any trailing bits MUST be set to 0. (§5.2.3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. negative: an unmasked 198.51.100.129/25 is emitted as 25,198.51.100.128; positive: a masked prefix is unchanged

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeTopologySeparation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L86) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeMaskedPrefixUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L148) | unit/verify | revert, verified |

### [`RFC9552-5.3.2.3-2`](#rfc9552-5.3.2.3-2)

IS-IS small metrics are 6 bits in size but are encoded in a 1-octet field; therefore, the two most significant bits of the field MUST be set to 0 by the originator (§5.3.2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. row now cites 5.3.2.4; positive: 1-octet metrics keep bits 7-6 clear; negative: metrics above 0x3F widen to 2/3 octets, so a 1-octet encoder for 0x40 would fail

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552IGPMetricWidthGrowsInsteadOfTruncating`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L252) | unit/verify | unproven |
| positive | [`TestRFC9552ISISSmallMetricTwoMSBsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L218) | unit/verify | unproven |

### [`RFC9552-8.2.2-1`](#rfc9552-8.2.2-1)

A Link-State NLRI MUST NOT be considered malformed or invalid based on the inclusion/exclusion of TLVs or contents of the TLV fields (i.e., semantic errors), as described in Sections 5.1 and 5.2. (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. re-audit RA12: both units are decoder-level (parseBGPLS accepts private Protocol-ID, missing IGP Router-ID, unknown sub-TLV; framing errors refused); the 8.2.2 fault-management verdict that a received NLRI is malformed is taken on the session path (message/rfc7606_bgpls_nlri.go::bgplsNLRIWellFormed via enforceRFC7606), which no 8.2.2-1 tag drives with semantically odd content

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NLRIFramingErrorsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L181) | unit/verify | unproven |
| positive | [`TestRFC9552NLRIContentsNeverMakeItMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L147) | unit/verify | unproven |

### [`RFC9552-8.2.2-2`](#rfc9552-8.2.2-2)

A BGP-LS Attribute MUST NOT be considered malformed or invalid based on the inclusion/exclusion of TLVs or contents of the TLV fields (i.e., semantic errors), as described in Sections 5.1 and 5.3. (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. re-audit RA12: units are decoder-level (iterateAttrTLVs/decodeAllAttrTLVs accept descending, cross-context, unknown TLVs and reserved flag bits); the session-path attribute judgement (message/rfc7606_bgpls.go::validateBGPLSAttr) is not driven with semantically odd TLVs under an 8.2.2-2 tag; and the negative unit also asserts decodeAttrTLV refusing a wrong-size fixed-length TLV, which 8.2.2 itself lists as semantic validation

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552AttributeSyntaxStillRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L95) | unit/verify | unproven |
| positive | [`TestRFC9552UnorderedUnexpectedAttributeTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L54) | unit/verify | unproven |

### [`RFC9552-8.2.2-3`](#rfc9552-8.2.2-3)

A BGP-LS Propagator, even when it has a coexisting BGP-LS Consumer on the same node, should not perform semantic validation of the Link- State NLRI or the BGP-LS Attribute to determine if it is malformed or invalid. (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. re-audit RA12: every tagged unit is decoder-level; none drives the Propagator receive path (enforceRFC7606/validateBGPLSAttr) to show no semantic check there; the negative unit presents decodeAttrTLV refusing a 3-octet Administrative Group as 'syntax still checked', yet 8.2.2 names 'the length of a fixed-length TLV is correct' as semantic validation not to be performed

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552AttributeSyntaxStillRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L96) | unit/verify | unproven |
| negative | [`TestRFC9552NLRIFramingErrorsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L182) | unit/verify | unproven |
| positive | [`TestRFC9552NLRIContentsNeverMakeItMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L148) | unit/verify | unproven |
| positive | [`TestRFC9552UnorderedUnexpectedAttributeTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L55) | unit/verify | unproven |

### [`RFC9552-8.2.2-4`](#rfc9552-8.2.2-4)

When the error that is determined allows for the router to skip the malformed NLRI(s) and continue the processing of the rest of the BGP UPDATE message (e.g., when the TLV ordering rule is violated), then it MUST handle such malformed NLRIs as 'NLRI discard' (i.e., processing similar to what is described in Section 5.4 of [RFC7606]). (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. receive path: TLV overrun and duplicate sub-TLV NLRIs removed alone, sibling survives, no session reset; a well-formed NLRI survives byte-identically

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L51) | unit/verify | unproven |
| positive | [`TestRFC9552LinkStateNLRITLVOverrunIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L90) | unit/verify | unproven |
| positive | [`TestRFC9552LinkStateNodeDescriptorDuplicateSubTLVIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L128) | unit/verify | unproven |

### [`RFC9552-8.2.2-5`](#rfc9552-8.2.2-5)

Alternately, the router MUST perform a 'session reset' when the session is only being used for BGP-LS or if 'AFI/SAFI disable' action is not possible. (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Total NLRI Length overrunning MP_REACH gives RFC7606ActionSessionReset; a well-formed section leaves the session up

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L52) | unit/verify | unproven |
| positive | [`TestRFC9552LinkStateNLRILengthOverrunResetsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L204) | unit/verify | unproven |

### [`RFC9552-8.2.2-6`](#rfc9552-8.2.2-6)

When the error that is determined allows for the router to skip the malformed BGP-LS Attribute and continue the processing of the rest of the BGP UPDATE message (e.g., when the BGP-LS Attribute length and the total Path Attribute Length are correct but some TLV/sub-TLV length within the BGP-LS Attribute is invalid), then it MUST handle such malformed BGP-LS Attribute as 'Attribute Discard'. (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TLV overrun and trailing-octet attributes give AttributeDiscard with tombstone and MP_REACH kept; a well-formed attribute survives byte-identically

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552BGPLSAttributeWellFormedIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L88) | unit/verify | unproven |
| positive | [`TestRFC9552BGPLSAttributeTLVOverrunDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L121) | unit/verify | unproven |
| positive | [`TestRFC9552BGPLSAttributeTrailingOctetsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L162) | unit/verify | unproven |

### [`RFC9552-5.3-2`](#rfc9552-5.3-2)

When a BGP-LS Propagator finds that it is exceeding the maximum BGP message size due to the addition or update of some other BGP Attribute (e.g., AS_PATH), it MUST consider the BGP-LS Attribute to be malformed, apply the 'Attribute Discard' error-handling approach [RFC7606], and handle the propagation as described in Section 8.2.2. (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.3-2, so no unit is bound to it.

### [`RFC9552-8.2.6-1`](#rfc9552-8.2.6-1)

An implementation MUST have the means to limit inbound updates (§8.2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBGPLSPrefixLimitActuallyFires`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_prefix_limit_test.go#L100) | unit/verify | unproven |
| positive | [`TestBGPLSPrefixCountCountsNLRIsNotPrefixBytes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_prefix_limit_test.go#L39) | unit/verify | unproven |

### [`RFC9552-5.2.2-6`](#rfc9552-5.2.2-6)

If the value in the MT-ID TLV is derived from OSPF, then the upper R bits of the MT-ID field MUST be set to 0 and only the values from 0 to 127 are valid for the MT-ID (§5.2.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeOSPFTopologyBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L112) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeOSPFTopologyBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L111) | unit/verify | revert, verified |

### [`RFC9552-5.1-7`](#rfc9552-5.1-7)

a BGP-LS Consumer MUST NOT be able to send information to a BGP Speaker for origination into BGP-LS. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.1-7, so no unit is bound to it.

### [`RFC9552-5.2.1.1-1`](#rfc9552-5.2.1.1-1)

The same node MUST NOT be represented by two keys (§5.2.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNodeDescriptorWriteToMatchesBytes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/types_descriptor_key_test.go#L156) | unit/verify | unproven |
| positive | [`TestSameNodeHasOneKeyWhateverTheStorageOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/types_descriptor_key_test.go#L186) | unit/verify | unproven |

### [`RFC9552-5.2.1.1-2`](#rfc9552-5.2.1.1-2)

Two different nodes MUST NOT be represented by the same key (§5.2.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNodeDescriptorKeepsBackboneDistinctFromAreaLess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/types_descriptor_key_test.go#L57) | unit/verify | unproven |
| positive | [`TestNodeDescriptorEncodesLegalZeroKeyFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/types_descriptor_key_test.go#L20) | unit/verify | unproven |

### [`RFC9552-5.2.2.1-1`](#rfc9552-5.2.2.1-1)

When used as a Link or Prefix Descriptor for IS-IS, the Bits R are reserved and MUST be set to 0 (as per Section 7.2 of [RFC5120]) when originated and ignored on receipt. (§5.2.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive: R bits set decode to the same MT-ID on Link and Prefix descriptors with the next TLV still read; negative: IS-IS MT-ID 4096 (R bit) refused at origination

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeISISTopologyReservedBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L161) | unit/verify | revert, verified |
| positive | [`TestRFC9552ISISMTIDReservedBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L314) | unit/verify | unproven |

### [`RFC9552-8.2.2-9`](#rfc9552-8.2.2-9)

A BGP-LS Speaker MUST perform the following syntactic validation of the Link-State NLRI to determine if it is malformed. (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. row is the lead-in to seven syntactic checks; tagged units drive the MP_REACH_NLRI length sum (session reset), a Node Descriptor TLV overrun, a duplicate Node Descriptor sub-TLV and descending TLV order through enforceRFC7606, with a well-formed negative; the MP_UNREACH_NLRI length-sum bullet and the invalid-length sub-TLV of a recognized TLV, both implemented in validateBGPLSNLRISyntax/bgplsNLRIWellFormed, are driven by no tagged unit, so a regression there stays green

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L50) | unit/verify | unproven |
| positive | [`TestRFC9552LinkStateNLRILengthOverrunResetsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L203) | unit/verify | unproven |
| positive | [`TestRFC9552LinkStateNLRIOutOfOrderTLVsAreDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L164) | unit/verify | unproven |
| positive | [`TestRFC9552LinkStateNLRITLVOverrunIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L89) | unit/verify | unproven |
| positive | [`TestRFC9552LinkStateNodeDescriptorDuplicateSubTLVIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L126) | unit/verify | unproven |

### [`RFC9552-8.2.2-10`](#rfc9552-8.2.2-10)

A BGP-LS Speaker MUST perform the following syntactic validation of the BGP-LS Attribute to determine if it is malformed. (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive units drive a TLV overrunning the attribute and a trailing half-header (both sum-to-length failures) to Attribute Discard with a tombstone; negative keeps a well-formed attribute byte-identical; the recognized sub-TLV clause is vacuous on the session path because ze recognizes no BGP-LS Attribute TLV there (rfc7606_bgpls.go)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552BGPLSAttributeWellFormedIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L87) | unit/verify | unproven |
| positive | [`TestRFC9552BGPLSAttributeTLVOverrunDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L120) | unit/verify | unproven |
| positive | [`TestRFC9552BGPLSAttributeTrailingOctetsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L161) | unit/verify | unproven |

### [`RFC9552-8.2.3-5`](#rfc9552-8.2.3-5)

An implementation MUST allow the operator to configure an 8-octet BGP-LS Instance-ID (§8.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeInstanceIDOverflowRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L174) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeInstanceIDFullWidth`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L277) | unit/verify | revert, verified |

### [`RFC9552-8.2.6-2`](#rfc9552-8.2.6-2)

An operator MUST define an import policy to limit inbound updates as follows: * Drop all updates from peers that are only serving BGP-LS Consumers. (§8.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-8.2.6-2, so no unit is bound to it.

### [`RFC9552-5.2.2.1-2`](#rfc9552-5.2.2.1-2)

In case one wants to advertise multiple topologies for a given Link or Prefix Descriptor, multiple NLRIs MUST be generated where each NLRI contains a single unique MT-ID (§5.2.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeTopologyReplacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L146) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeTopologySeparation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L83) | unit/verify | revert, verified |

### [`RFC9552-5.2.3-2`](#rfc9552-5.2.3-2)

The Multi-Topology Identifier TLV MUST be included in the Prefix Descriptor if the underlying IGP prefix object is associated with a non-default topology (§5.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeTopologyReplacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L147) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeTopologySeparation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L84) | unit/verify | revert, verified |

### [`RFC9552-5.3.1.1-1`](#rfc9552-5.3.1.1-1)

The bits that are not defined MUST be set to 0 by the originator and MUST be ignored by the receiver. (§5.3.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. originator half: native export clears source bits 6-7 to 0 and keeps the defined bits (0xfc kept, 0x03 emitted as 0); receiver half: decoder yields identical defined bits with undefined bits set or clear, sets no defined flag from undefined-only bits, and keeps walking to the next TLV

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeReservedFlagsCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L210) | unit/verify | revert, verified |
| negative | [`TestRFC9552UndefinedNodeBitsDoNotSetDefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_flags_rfc9552_test.go#L126) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeDefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L197) | unit/verify | revert, verified |
| positive | [`TestRFC9552NodeFlagBitsUndefinedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_flags_rfc9552_test.go#L61) | unit/verify | revert, verified |

### [`RFC9552-5.3.1.5-1`](#rfc9552-5.3.1.5-1)

In the case of OSPF, this TLV MUST NOT be used to advertise TLVs other than those in the OSPF Router Information (RI) LSA [RFC7770]. (§5.3.1.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. OSPFv2 node opaque from RI-LSA provenance reaches TLV 1025; Extended Link LSA provenance is refused by validOpaqueSource and leaves the advertised state untouched; the OSPF node check is shared by OSPFv2 and OSPFv3, only OSPFv2 driven

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeOpaqueNodeProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L52) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeOpaqueNodeProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L51) | unit/verify | revert, verified |

### [`RFC9552-5.3.2.2-3`](#rfc9552-5.3.2.2-3)

The bits that are not defined MUST be set to 0 by the originator and MUST be ignored by the receiver. (§5.3.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. quote is the undefined-bits sentence: native export clears reserved MPLS mask bits (0x3f emitted as 0) and keeps L/R (0xc0); decoder reports L=0 R=0 for reserved-only bits and keeps walking, and decodes L/R when reserved bits are zero

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeReservedFlagsCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L212) | unit/verify | revert, verified |
| negative | [`TestRFC9552MPLSProtocolMaskReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_flags_rfc9552_test.go#L164) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeDefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L199) | unit/verify | revert, verified |
| positive | [`TestRFC9552MPLSProtocolMaskDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_flags_rfc9552_test.go#L154) | unit/verify | revert, verified |

### [`RFC9552-5.3.2.6-1`](#rfc9552-5.3.2.6-1)

In the case of OSPFv2, this TLV MUST NOT be used to advertise information carried using TLVs other than those in the OSPFv2 Extended Link Opaque LSA [RFC7684]. (§5.3.2.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. OSPFv2 link opaque from Extended Link LSA reaches TLV 1097; RI-LSA provenance refused without replacing the valid state; the OSPFv2 link allowlist has one member, so the pair discriminates it

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeOpaqueOSPFv2LinkProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeOpaqueOSPFv2LinkProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L59) | unit/verify | revert, verified |

### [`RFC9552-5.3.2.6-2`](#rfc9552-5.3.2.6-2)

In the case of OSPFv3, this TLV MUST NOT be used to advertise TLVs other than those in the OSPFv3 E- Router-LSA or E-Link-LSA [RFC8362]. (§5.3.2.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. OSPFv3 link opaque from E-Router-LSA reaches TLV 1097; E-Inter-Area-Prefix provenance refused; E-Link-LSA acceptance and other forbidden sources are not driven, one representative of each side

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeOpaqueOSPFv3LinkProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L68) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeOpaqueOSPFv3LinkProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L67) | unit/verify | revert, verified |

### [`RFC9552-5.3.3.1-1`](#rfc9552-5.3.3.1-1)

The bits that are not defined MUST be set to 0 by the originator and MUST be ignored by the receiver. (§5.3.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. originator half: native export clears IGP Flags bits 4-7 (0x0f emitted as 0) and keeps D/N/L/P (0xf0); receiver half: decoder gives identical defined bits with undefined bits set or clear, sets no defined flag from undefined-only bits, and continues the walk

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeReservedFlagsCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L211) | unit/verify | revert, verified |
| negative | [`TestRFC9552UndefinedIGPBitsDoNotSetDefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_flags_rfc9552_test.go#L140) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeDefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_test.go#L198) | unit/verify | revert, verified |
| positive | [`TestRFC9552IGPFlagsUndefinedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_flags_rfc9552_test.go#L98) | unit/verify | revert, verified |

### [`RFC9552-5.3.3.6-1`](#rfc9552-5.3.3.6-1)

In the case of OSPFv2, this TLV MUST NOT be used to advertise information carried using TLVs other than those in the OSPFv2 Extended Prefix Opaque LSA [RFC7684]. (§5.3.3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. OSPFv2 prefix opaque from Extended Prefix LSA reaches TLV 1157; Extended Link LSA provenance refused; single-member allowlist, so the pair discriminates it

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeOpaqueOSPFv2PrefixProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeOpaqueOSPFv2PrefixProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L75) | unit/verify | revert, verified |

### [`RFC9552-5.3.3.6-2`](#rfc9552-5.3.3.6-2)

In the case of OSPFv3, this TLV MUST NOT be used to advertise TLVs other than those in the OSPFv3 E-Inter-Area-Prefix-LSA, E-Intra-Area-Prefix-LSA, E-AS-External-LSA, and E-NSSA-LSA [RFC8362]. (§5.3.3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. OSPFv3 prefix opaque from E-NSSA-LSA reaches TLV 1157; E-Link-LSA provenance refused; the other three permitted LSAs and other forbidden sources are not driven, one representative of each side

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeOpaqueOSPFv3PrefixProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L84) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeOpaqueOSPFv3PrefixProvenance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_provenance_test.go#L83) | unit/verify | revert, verified |

### [`RFC9552-5.4-2`](#rfc9552-5.4-2)

For a private use NLRI type, a 4-octet field MUST be included as the first field in the NLRI immediately following the Total NLRI Length field of the Link-State NLRI format as described in Section 5.2 to carry the Enterprise Code [ENTNUM]. (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.4-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc9552.txt |
| Source fingerprint | ab66a307e33b68f0 |
| Record | rfc/extraction/rfc9552.json |
| Mapped sentences | 58 |
| Declined as scope | 12 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 6 | walked | not stated |
| `5.2` | not stated | 9 | walked | not stated |
| `5.2.1` | not stated | 1 | walked | not stated |
| `5.2.1.1` | not stated | 2 | walked | not stated |
| `5.2.1.2` | not stated | 0 | walked | not stated |
| `5.2.1.3` | not stated | 0 | walked | not stated |
| `5.2.1.4` | not stated | 2 | walked | not stated |
| `5.2.2` | not stated | 5 | walked | not stated |
| `5.2.2.1` | not stated | 3 | walked | not stated |
| `5.2.3` | not stated | 1 | walked | not stated |
| `5.2.3.1` | not stated | 1 | walked | not stated |
| `5.2.3.2` | not stated | 1 | walked | not stated |
| `5.3` | not stated | 3 | walked | not stated |
| `5.3.1` | not stated | 0 | walked | not stated |
| `5.3.1.1` | not stated | 1 | walked | not stated |
| `5.3.1.2` | not stated | 0 | walked | not stated |
| `5.3.1.3` | not stated | 0 | walked | not stated |
| `5.3.1.4` | not stated | 0 | walked | not stated |
| `5.3.1.5` | not stated | 1 | walked | not stated |
| `5.3.2` | not stated | 0 | walked | not stated |
| `5.3.2.1` | not stated | 1 | walked | not stated |
| `5.3.2.2` | not stated | 3 | walked | not stated |
| `5.3.2.3` | not stated | 1 | walked | not stated |
| `5.3.2.4` | not stated | 1 | walked | not stated |
| `5.3.2.5` | not stated | 0 | walked | not stated |
| `5.3.2.6` | not stated | 2 | walked | not stated |
| `5.3.2.7` | not stated | 0 | walked | not stated |
| `5.3.3` | not stated | 0 | walked | not stated |
| `5.3.3.1` | not stated | 1 | walked | not stated |
| `5.3.3.2` | not stated | 0 | walked | not stated |
| `5.3.3.3` | not stated | 0 | walked | not stated |
| `5.3.3.4` | not stated | 0 | walked | not stated |
| `5.3.3.5` | not stated | 0 | walked | not stated |
| `5.3.3.6` | not stated | 2 | walked | not stated |
| `5.4` | not stated | 2 | walked | not stated |
| `5.5` | not stated | 1 | walked | not stated |
| `5.6` | not stated | 0 | walked | not stated |
| `5.7` | not stated | 0 | walked | not stated |
| `5.8` | not stated | 0 | walked | not stated |
| `5.9` | not stated | 1 | walked | not stated |
| `5.10` | not stated | 0 | walked | not stated |
| `5.11` | not stated | 0 | walked | not stated |
| `5.12` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `6.3` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.1.1` | not stated | 0 | walked | not stated |
| `7.1.2` | not stated | 0 | walked | not stated |
| `7.1.3` | not stated | 0 | walked | not stated |
| `7.1.4` | not stated | 0 | walked | not stated |
| `7.1.5` | not stated | 0 | walked | not stated |
| `7.1.6` | not stated | 0 | walked | not stated |
| `7.1.7` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 8 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.1.1` | not stated | 0 | walked | not stated |
| `8.1.2` | not stated | 0 | walked | not stated |
| `8.1.3` | not stated | 0 | walked | not stated |
| `8.1.4` | not stated | 0 | walked | not stated |
| `8.1.5` | not stated | 0 | walked | not stated |
| `8.1.6` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `8.2.1` | not stated | 0 | walked | not stated |
| `8.2.2` | not stated | 7 | walked | not stated |
| `8.2.3` | not stated | 1 | walked | not stated |
| `8.2.4` | not stated | 0 | walked | not stated |
| `8.2.5` | not stated | 0 | walked | not stated |
| `8.2.6` | not stated | 2 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `11.1` | not stated | 0 | walked | not stated |
| `11.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `5.2:8` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Addressed to the authors of future documents that extend BGP-LS, not to an implementation: it says what such a document must contain. No ze code path can satisfy or violate it. | Documents extending BGP-LS specifications with new NLRI Types and/or protocols MUST specify the NLRI descriptors for them. |
| `5.2.2:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Link Local/Remote Identifiers obligation the preceding sentence states: "If interface and neighbor addresses are not present and the link local/remote identifiers are present, then the Link Local/Remote Identifiers TLV MUST be included in the Link Descriptor." | The Link Local/Remote identifiers MUST be included in the Link Descriptor and in the case of links having only IPv6 link-local addressing on them. |
| `5.3:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Attribute Discard ordering the preceding sentence states, that the BGP-LS Attribute is discarded first when the BGP-LS Propagator must reduce the BGP UPDATE message size. | When a BGP-LS Propagator needs to perform 'Attribute Discard' for reducing the BGP UPDATE message size as specified in Section 4 of [RFC8654], it MUST first discard the BGP-LS Attribute to enable the detection and diagnosis of this error condition as discussed in Section 8.2.2. |
| `5.3.2.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the reserved-bit obligation the preceding sentence in the same TLV description states, which RFC9552-5.3.2.2-3 carries in full. | The bits that are not defined MUST be set to 0 by the originator and MUST be ignored by the receiver. |
| `7.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds the IANA designated expert who performs Expert Review for the BGP-LS registries under Section 7.2, a role ze never fills: ze is a BGP-LS Speaker that encodes and decodes code points, and reviews no allocation request. producer: none in ze. The reviewing role is exercised by IANA and the designated experts named in Section 7.2, outside any ze code path; the closest ze producer, internal/component/bgp/plugins/nlri/ls, consumes allocated code points and never allocates one. | Application for a code point allocation may be made to the designated experts at any time and MUST be accompanied by technical documentation explaining the use of the code point. |
| `7.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds the IANA designated expert who performs Expert Review for the BGP-LS registries under Section 7.2, a role ze never fills: ze is a BGP-LS Speaker that encodes and decodes code points, and reviews no allocation request. producer: none in ze. The reviewing role is exercised by IANA and the designated experts named in Section 7.2, outside any ze code path; the closest ze producer, internal/component/bgp/plugins/nlri/ls, consumes allocated code points and never allocates one. | In the case of working group documents, the designated experts MUST check with the working group chairs that there is a consensus within the working group to allocate at this time. |
| `7.2:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds the IANA designated expert who performs Expert Review for the BGP-LS registries under Section 7.2, a role ze never fills: ze is a BGP-LS Speaker that encodes and decodes code points, and reviews no allocation request. producer: none in ze. The reviewing role is exercised by IANA and the designated experts named in Section 7.2, outside any ze code path; the closest ze producer, internal/component/bgp/plugins/nlri/ls, consumes allocated code points and never allocates one. | In the case of AD-Sponsored documents, the designated experts MUST check with the AD for approval to allocate at this time. |
| `7.2:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds the IANA designated expert who performs Expert Review for the BGP-LS registries under Section 7.2, a role ze never fills: ze is a BGP-LS Speaker that encodes and decodes code points, and reviews no allocation request. producer: none in ze. The reviewing role is exercised by IANA and the designated experts named in Section 7.2, outside any ze code path; the closest ze producer, internal/component/bgp/plugins/nlri/ls, consumes allocated code points and never allocates one. | If the document is not adopted by the IDR Working Group (or its successor), the designated expert MUST notify the IDR mailing list (or its successor) of the request and MUST provide access to the document. |
| `7.2:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds the IANA designated expert who performs Expert Review for the BGP-LS registries under Section 7.2, a role ze never fills: ze is a BGP-LS Speaker that encodes and decodes code points, and reviews no allocation request. producer: none in ze. The reviewing role is exercised by IANA and the designated experts named in Section 7.2, outside any ze code path; the closest ze producer, internal/component/bgp/plugins/nlri/ls, consumes allocated code points and never allocates one. | The designated expert MUST allow two weeks for any response. |
| `7.2:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds the IANA designated expert who performs Expert Review for the BGP-LS registries under Section 7.2, a role ze never fills: ze is a BGP-LS Speaker that encodes and decodes code points, and reviews no allocation request. producer: none in ze. The reviewing role is exercised by IANA and the designated experts named in Section 7.2, outside any ze code path; the closest ze producer, internal/component/bgp/plugins/nlri/ls, consumes allocated code points and never allocates one. | Any comments received MUST be considered by the designated expert as part of the subsequent step. |
| `7.2:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds the IANA designated expert who performs Expert Review for the BGP-LS registries under Section 7.2, a role ze never fills: ze is a BGP-LS Speaker that encodes and decodes code points, and reviews no allocation request. producer: none in ze. The reviewing role is exercised by IANA and the designated experts named in Section 7.2, outside any ze code path; the closest ze producer, internal/component/bgp/plugins/nlri/ls, consumes allocated code points and never allocates one. | The designated experts MUST then review the assignment requests on their technical merit. |
| `7.2:8` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds the IANA designated expert who performs Expert Review for the BGP-LS registries under Section 7.2, a role ze never fills: ze is a BGP-LS Speaker that encodes and decodes code points, and reviews no allocation request. producer: none in ze. The reviewing role is exercised by IANA and the designated experts named in Section 7.2, outside any ze code path; the closest ze producer, internal/component/bgp/plugins/nlri/ls, consumes allocated code points and never allocates one. | The designated expert MUST ensure that any request for a code point does not conflict with work that is active or already published within the IETF. |

## Superseded

No document obsoletes RFC 9552, so its obligations are stated where they were written.
