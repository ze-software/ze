# RFC 9552 - Distribution of Link-State and Traffic Engineering Information Using BGP

Partial. Every requirement this repository extracted from RFC 9552, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 81.5% | 22 of 27 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 14.8% | 4 of 27 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 27 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 61 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 48 | of 77 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 21 | of 48 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 3.7% | 1 of 27 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 27 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 77 |
| Gated MUST-level | 48 |
| Obligations that bind Ze | 27 |
| Not applicable, so out of scope | 21 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 61 |
| Tagged units | 61 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9552.md` |
| Requirement shard | `rfc/requirements/rfc9552.md` |
| RFC text | `rfc/full/rfc9552.txt` |

## Enrolment

Enrolled: Distribution of Link-State and TE Information Using BGP

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Same wire format as RFC 7752 and the same role: ze is a BGP-LS Consumer-side decoder and Propagator, never a Producer. Node/Link/Prefix NLRI and node, link and prefix attribute TLV decode (`internal/component/bgp/plugins/nlri/ls`), (AFI 16388, SAFI 71/72) family registration and Multiprotocol capability negotiation, unknown NLRI types framed by Total NLRI Length alone and propagated byte-identically under both SAFI 71 and SAFI 72 (`GetNLRISizeFunc`, [`internal/component/bgp/message/chunk_mp_nlri.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/chunk_mp_nlri.go)), unknown and unexpected attribute TLVs preserved, unordered BGP-LS Attribute TLVs accepted as RFC 9552 now requires, no semantic validation on the propagation path, RFC 9552 §8.2.2 syntactic validation of the BGP-LS Attribute on the receive path (`validateBGPLSAttr`, [`internal/component/bgp/message/rfc7606_bgpls.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_bgpls.go)) with 'Attribute Discard' handling for a malformed one, RFC 9552 §8.2.2 syntactic validation of the Link-State NLRI on the receive path (`validateBGPLSNLRISyntax` and `RetainWellFormedNLRI`, [`internal/component/bgp/message/rfc7606_bgpls_nlri.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_bgpls_nlri.go)) with 'NLRI discard' for a skipable error and session reset for a length error that leaves the UPDATE unprocessable, every descriptor's sub-TLVs emitted in the canonical order Section 5.1 defines -- ascending by TLV type across node, link and prefix descriptors, and, among repeated sub-TLVs of one type, ascending by Length then by Value (`addressTLVs` and `srv6SIDsOrdered`, [`internal/component/bgp/plugins/nlri/ls/types_descriptor.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/types_descriptor.go)) -- so one node never encodes to two keys, RFC 4760 next-hop encoding, zero-padded TE Default Metric and a 1-octet IS-IS small metric whose two high bits are always zero. Requirements bound per line in [`rfc/short/rfc9552.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9552.md).

**What the ledger says remains:**

One MUST gap annotated in [`rfc/short/rfc9552.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9552.md): [`RFC9552-5.3-2`](#rfc9552-5.3-2) -- an oversized forwarded UPDATE that cannot be split is dropped whole instead of having the BGP-LS Attribute discarded first

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 22 | one part of the gated population |
| Annotated instead of tested | 26 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **48** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (22):** [`RFC9552-5.1-1`](#rfc9552-5.1-1), [`RFC9552-5.1-2`](#rfc9552-5.1-2), [`RFC9552-5.1-3`](#rfc9552-5.1-3), [`RFC9552-5.1-4`](#rfc9552-5.1-4), [`RFC9552-5.1-5`](#rfc9552-5.1-5), [`RFC9552-5.1-6`](#rfc9552-5.1-6), [`RFC9552-5.2-1`](#rfc9552-5.2-1), [`RFC9552-5.2-2`](#rfc9552-5.2-2), [`RFC9552-5.2-7`](#rfc9552-5.2-7), [`RFC9552-5.2-8`](#rfc9552-5.2-8), [`RFC9552-5.2.1.4-1`](#rfc9552-5.2.1.4-1), [`RFC9552-5.3.2.3-2`](#rfc9552-5.3.2.3-2), [`RFC9552-8.2.2-1`](#rfc9552-8.2.2-1), [`RFC9552-8.2.2-2`](#rfc9552-8.2.2-2), [`RFC9552-8.2.2-4`](#rfc9552-8.2.2-4), [`RFC9552-8.2.2-5`](#rfc9552-8.2.2-5), [`RFC9552-8.2.2-6`](#rfc9552-8.2.2-6), [`RFC9552-8.2.6-1`](#rfc9552-8.2.6-1), [`RFC9552-5.2.1.1-1`](#rfc9552-5.2.1.1-1), [`RFC9552-5.2.1.1-2`](#rfc9552-5.2.1.1-2), [`RFC9552-8.2.2-9`](#rfc9552-8.2.2-9), [`RFC9552-8.2.2-10`](#rfc9552-8.2.2-10)

**Annotated instead of tested (26):** [`RFC9552-5.2-3`](#rfc9552-5.2-3), [`RFC9552-5.2-4`](#rfc9552-5.2-4), [`RFC9552-5.2-5`](#rfc9552-5.2-5), [`RFC9552-5.2-6`](#rfc9552-5.2-6), [`RFC9552-5.2.1.4-2`](#rfc9552-5.2.1.4-2), [`RFC9552-5.2.2-1`](#rfc9552-5.2.2-1), [`RFC9552-5.2.2-2`](#rfc9552-5.2.2-2), [`RFC9552-5.2.2-3`](#rfc9552-5.2.2-3), [`RFC9552-5.2.2-4`](#rfc9552-5.2.2-4), [`RFC9552-5.2.2-5`](#rfc9552-5.2.2-5), [`RFC9552-5.2.3.1-1`](#rfc9552-5.2.3.1-1), [`RFC9552-5.2.1-1`](#rfc9552-5.2.1-1), [`RFC9552-5.3.2.1-1`](#rfc9552-5.3.2.1-1), [`RFC9552-5.3.2.2-1`](#rfc9552-5.3.2.2-1), [`RFC9552-5.3.2.3-1`](#rfc9552-5.3.2.3-1), [`RFC9552-5.5-1`](#rfc9552-5.5-1), [`RFC9552-5.3-1`](#rfc9552-5.3-1), [`RFC9552-5.9-1`](#rfc9552-5.9-1), [`RFC9552-5.4-1`](#rfc9552-5.4-1), [`RFC9552-5.2.3-1`](#rfc9552-5.2.3-1), [`RFC9552-5.3-2`](#rfc9552-5.3-2), [`RFC9552-5.2.2-6`](#rfc9552-5.2.2-6), [`RFC9552-5.1-7`](#rfc9552-5.1-7), [`RFC9552-5.2.2.1-1`](#rfc9552-5.2.2.1-1), [`RFC9552-8.2.3-5`](#rfc9552-8.2.3-5), [`RFC9552-8.2.6-2`](#rfc9552-8.2.6-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9552-5.1-1` | All TLVs within the NLRI MUST be ordered in ascending order by TLV Type (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestLinkDescriptorOrdersMixedFamilyAddressesAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L39). **negative:** `unit/verify` [`TestNoDescriptorEmitsADescendingTLVSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L73) |
| `RFC9552-5.1-2` | Same-type TLVs MUST be ordered ascending by Length, then ascending by Value (lexicographic binary) (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestNodeDescriptorOrdersRepeatedSRv6SIDs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L124). **negative:** `unit/verify` [`TestSRv6SIDOrderIsLengthBeforeValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L170) |
| `RFC9552-5.1-3` | Unknown and unsupported TLV types MUST be preserved and propagated within both the NLRI and the BGP-LS Attribute (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC7752UnknownTLVPreservedAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L65). **negative:** `unit/verify` [`TestRFC7752MalformedTLVNotPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L103) |
| `RFC9552-5.1-4` | Presence of unknown or unexpected TLVs MUST NOT result in the NLRI or BGP-LS Attribute being considered malformed (§5.1) | MUST NOT | 5.1 | **positive:** `unit/verify` [`TestRFC9552UnorderedUnexpectedAttributeTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L53). **negative:** `unit/verify` [`TestRFC9552AttributeSyntaxStillRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L94) |
| `RFC9552-5.1-5` | NLRIs having TLVs that do not follow ordering rules MUST be considered malformed by a BGP-LS Propagator (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC9552LinkStateNLRIOutOfOrderTLVsAreDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L165). **negative:** `unit/verify` [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L53) |
| `RFC9552-5.1-6` | BGP-LS Attribute with unordered TLVs MUST NOT be considered malformed (§5.1) | MUST NOT | 5.1 | **positive:** `unit/verify` [`TestRFC9552UnorderedUnexpectedAttributeTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L52). **negative:** `unit/verify` [`TestRFC9552AttributeSyntaxStillRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L93) |
| `RFC9552-5.2-1` | All non-VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 71 (§5.2) | SHALL | 5.2 | **positive:** `unit/verify` [`TestRFC7752NonVPNFamilyIsAFI16388SAFI71`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L230). **negative:** `unit/verify` [`TestRFC7752NonLinkStateFamilyRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L267) |
| `RFC9552-5.2-2` | VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 72 (§5.2) | SHALL | 5.2 | **positive:** `unit/verify` [`TestRFC7752VPNFamilyIsAFI16388SAFI72`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L248). **positive:** `unit/verify` [`TestRFC9552BGPLSVPNNLRIFramedByLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9552_bgpls_test.go#L135). **negative:** `unit/verify` [`TestRFC7752NonLinkStateFamilyRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L268) |
| `RFC9552-5.2-3` | For all information derived from other protocols, the corresponding Protocol-ID MUST be used (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze selects no Protocol-ID because it derives no link-state from an IGP. ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions; the Protocol-ID on every BGP-LS route ze holds arrives on the wire and is parsed at internal/component/bgp/plugins/nlri/ls/types.go:316 |
| `RFC9552-5.2-4` | The network operator MUST assign the same BGP-LS Instance-IDs on all BGP-LS Producers within a given IGP domain (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze assigns no BGP-LS Instance-ID on any Producer because it runs none. The 8-octet Identifier is only ever read from the wire (internal/component/bgp/plugins/nlri/ls/types.go:317), there is no config surface that sets it (grep -rn "bgp-ls" --include=*.yang returns nothing), and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC9552-5.2-5` | Unique BGP-LS Instance-IDs MUST be assigned to routing protocol instances operating in different IGP domains (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze assigns no BGP-LS Instance-ID to any routing protocol instance, so it cannot make two domains collide. The Identifier is only ever read from the wire (internal/component/bgp/plugins/nlri/ls/types.go:317), no config surface sets it (grep -rn "bgp-ls" --include=*.yang returns nothing), and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC9552-5.2-6` | When modifying TLVs in NLRI, Producer MUST withdraw the old NLRI via MP_UNREACH_NLRI first (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze modifies no TLV in any NLRI because it builds none. The NLRI bytes it re-advertises are the received bytes: ParseBGPLS caches the wire slice (internal/component/bgp/plugins/nlri/ls/types.go:333), WriteTo copies it back out (internal/component/bgp/plugins/nlri/ls/types_nlri.go:63) and the transit path forwards the payload verbatim (internal/component/bgp/reactor/forward_body.go:64); ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC9552-5.2-7` | BGP speakers MUST use BGP Capabilities Advertisement to ensure both peers can process Link-State NLRIs (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC7752BGPLSCapabilityAdvertisedAndNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7752_bgpls_test.go#L29). **negative:** `unit/verify` [`TestRFC7752BGPLSCapabilityNotNegotiatedWhenPeerSilent`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7752_bgpls_test.go#L63) |
| `RFC9552-5.2-8` | An implementation MUST handle unknown Link-State NLRI types as opaque objects and MUST preserve and propagate them (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC9552BGPLSVPNNLRIFramedByLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9552_bgpls_test.go#L136). **positive:** `unit/verify` [`TestRFC9552UnknownBGPLSNLRITypeIsOpaque`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9552_bgpls_test.go#L46). **negative:** `unit/verify` [`TestRFC9552MalformedBGPLSNLRIRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9552_bgpls_test.go#L90). **positive:** `functional/verify` [`rfc9552-52-rs-opaque-withdraw-peer-down.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc9552-52-rs-opaque-withdraw-peer-down.ci#L3) |
| `RFC9552-5.2.1.4-1` | At most one instance of each sub-TLV type MUST be present in any Node Descriptor (§5.2.1.4) | MUST | 5.2.1.4 | **positive:** `unit/verify` [`TestRFC9552LinkStateNodeDescriptorDuplicateSubTLVIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L127). **negative:** `unit/verify` [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L54) |
| `RFC9552-5.2.1.4-2` | Sub-TLVs within a Node Descriptor MUST be arranged in ascending order by sub-TLV type (§5.2.1.4) | MUST | 5.2.1.4 | **positive:** `unit/verify` [`TestRFC7752NodeDescriptorSubTLVsAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L165). **negative:** no negative test. **{single-polarity}:** NodeDescriptor.WriteTo emits sub-TLVs 512, 513, 514, 515, 516 and 517 in that fixed ascending order (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:98), and the ordering duty falls on the sender: parseNodeDescriptorTLVs (internal/component/bgp/plugins/nlri/ls/types.go:391) accepts sub-TLVs in any order on receipt, so there is no out-of-order input for ze to reject |
| `RFC9552-5.2.2-1` | When interface/neighbor addresses are present, address TLVs MUST be included in Link Descriptors (§5.2.2) | MUST | 5.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze derives no link from an IGP, so it fills no Link Descriptor: LinkDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:200) runs only from BGPLSLink.WriteTo (internal/component/bgp/plugins/nlri/ls/types_nlri.go:181), and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions, so no interface or neighbor address ever reaches a Link Descriptor ze builds |
| `RFC9552-5.2.2-2` | Link Local/Remote Identifiers TLV MUST NOT be included in Link Descriptor when addresses are present (§5.2.2) | MUST NOT | 5.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze derives no link from an IGP, so it fills no Link Descriptor: LinkDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:200) runs only from BGPLSLink.WriteTo (internal/component/bgp/plugins/nlri/ls/types_nlri.go:181), and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions, so ze never faces the choice between address TLVs and the Link Local/Remote Identifiers TLV |
| `RFC9552-5.2.2-3` | IPv4/IPv6 link-local addresses MUST NOT be carried in TLVs 259/260/261/262 (§5.2.2) | MUST NOT | 5.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze derives no link from an IGP, so it fills no Link Descriptor: LinkDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:200) runs only from BGPLSLink.WriteTo (internal/component/bgp/plugins/nlri/ls/types_nlri.go:181), and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions, so ze places no address, link-local or otherwise, in TLV 259, 260, 261 or 262 |
| `RFC9552-5.2.2-4` | Link Local/Remote Identifiers TLV MUST be included when only link-local identifiers are available (§5.2.2) | MUST | 5.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze derives no link from an IGP, so it fills no Link Descriptor: LinkDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:200) runs only from BGPLSLink.WriteTo (internal/component/bgp/plugins/nlri/ls/types_nlri.go:181), and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions, so ze never has a link with only link-local identifiers to describe |
| `RFC9552-5.2.2-5` | Multi-Topology Identifier TLV MUST be included as Link Descriptor if link is associated with non-default topology (§5.2.2) | MUST | 5.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze emits no Multi-Topology Identifier TLV at all. LinkDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:200) and PrefixDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:264) write no TLV 263, grep -rn "TLVMultiTopologyID" --include=*.go matches only the constant declaration at internal/component/bgp/plugins/nlri/ls/types.go:207, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC9552-5.2.3.1-1` | OSPF Route Type TLV MUST be included when the route type is signaled in the underlying LSA or determinable from another LSA (§5.2.3.1) | MUST | 5.2.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 9552 raises the OSPF Route Type TLV from optional to mandatory, but ze advertises no OSPF prefix through BGP-LS and writes no TLV 264: PrefixDescriptor.WriteTo emits only the IP Reachability Information TLV (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:264) and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC9552-5.2.1-1` | Auxiliary TE Router-IDs (TLVs 1028/1029) MUST be included in the node attribute (§5.2.1) | MUST | 5.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze assembles no BGP-LS Attribute: every Ls*TLV struct is built only inside its own decode function (internal/component/bgp/plugins/nlri/ls/attr_node.go:179, internal/component/bgp/plugins/nlri/ls/attr_link.go:59) and a grep for composite-literal construction of LsIPv4RouterIDRemote, LsIPv4RouterIDLocal, LsIGPFlags and LsPrefixMetric outside _test.go finds only those decode functions, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC9552-5.3.2.1-1` | All auxiliary Router-IDs of both local and remote nodes MUST be included in the link attribute of each Link NLRI (§5.3.2.1) | MUST | 5.3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze assembles no BGP-LS Attribute: every Ls*TLV struct is built only inside its own decode function (internal/component/bgp/plugins/nlri/ls/attr_node.go:179, internal/component/bgp/plugins/nlri/ls/attr_link.go:59) and a grep for composite-literal construction of LsIPv4RouterIDRemote, LsIPv4RouterIDLocal, LsIGPFlags and LsPrefixMetric outside _test.go finds only those decode functions, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC9552-5.3.2.2-1` | MPLS Protocol Mask TLV MUST NOT be included in NLRIs with Protocol-IDs 1-3, 6 (IS-IS, OSPF) (§5.3.2.2) | MUST NOT | 5.3.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no MPLS Protocol Mask TLV. TLV 1094 is neither encoded nor registered for decode: grep -rn "1094" over internal/component/bgp/plugins/nlri/ls/ returns nothing and register_attr.go (internal/component/bgp/plugins/nlri/ls/register_attr.go:8) registers no 1094 decoder, so no NLRI ze emits or reads carries the TLV this clause restricts |
| `RFC9552-5.3.2.3-1` | High-order bits of TE Default Metric MUST be padded with zero if source is less than 32 bits (§5.3.2.3) | MUST | 5.3.2.3 | **positive:** `unit/verify` [`TestRFC7752TEDefaultMetricZeroPadded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L204). **negative:** no negative test. **{single-polarity}:** LsTEDefaultMetric.WriteTo always emits a 4-octet value (internal/component/bgp/plugins/nlri/ls/attr_link.go:226), so a metric sourced from a narrower width lands zero-padded in the high-order octets and ze owns no short-form TE metric encoder whose output could be rejected |
| `RFC9552-5.5-1` | The next-hop address MUST be encoded as described in RFC 4760 (§5.5) | MUST | 5.5 | **positive:** `unit/verify` [`TestRFC7752BGPLSNextHopFollowsRFC4760`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7752_bgpls_test.go#L25). **negative:** no negative test. **{single-polarity}:** MPReachNLRI.WriteTo is family agnostic and lays out AFI, SAFI, next-hop length, next-hop, the zero reserved octet and then the NLRI for AFI 16388 exactly as RFC 4760 Section 3 specifies (internal/core/bgp/attribute/mpnlri.go:154); ValidNextHopLens returns nil for AFI 16388 (internal/core/bgp/attribute/mpnlri.go:305), so ze runs no BGP-LS next-hop length check and holds no rejection path to drive negatively |
| `RFC9552-5.3-1` | BGP-LS Producers MUST ensure TLVs in BGP-LS Attribute do not cause UPDATE to exceed maximum message size (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze puts no TLV in a BGP-LS Attribute, so it cannot push an UPDATE past the maximum message size that way. ze assembles no BGP-LS Attribute: every Ls*TLV struct is built only inside its own decode function (internal/component/bgp/plugins/nlri/ls/attr_node.go:179, internal/component/bgp/plugins/nlri/ls/attr_link.go:59) and a grep for composite-literal construction of LsIPv4RouterIDRemote, LsIPv4RouterIDLocal, LsIGPFlags and LsPrefixMetric outside _test.go finds only those decode functions, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC9552-5.9-1` | Producer MUST re-advertise link-state objects after an unreachable node becomes reachable again (§5.9) | MUST | 5.9 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze tracks no IGP node reachability for BGP-LS and therefore neither withdraws nor re-advertises link-state objects on a reachability change. ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC9552-5.4-1` | Private Use TLV value MUST include 4-octet Enterprise Code as first field (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze emits no Private Use TLV. No encoder writes a type in the 65000-65535 range (register_attr.go registers only assigned code points, internal/component/bgp/plugins/nlri/ls/register_attr.go:8), grep -rni "enterprise" over internal/component/bgp/plugins/nlri/ls/ returns nothing, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC9552-5.2.3-1` | Trailing bits of IP prefix in IP Reachability Information TLV MUST be 0 (§5.2.3) | MUST | 5.2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no IP Reachability Information TLV. PrefixDescriptor.IPReachabilityInfo is copied verbatim from whatever its caller supplied (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:266), no non-test caller supplies one, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions; a received prefix is read as sent (internal/component/bgp/plugins/nlri/ls/plugin.go:593) |
| `RFC9552-5.3.2.3-2` | IS-IS small metric (1-byte IGP Metric): 2 MSBs MUST be set to 0 by originator (§5.3.2.3) | MUST | 5.3.2.3 | **positive:** `unit/verify` [`TestRFC9552ISISSmallMetricTwoMSBsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L218). **negative:** `unit/verify` [`TestRFC9552IGPMetricWidthGrowsInsteadOfTruncating`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L252) |
| `RFC9552-8.2.2-1` | A Link-State NLRI MUST NOT be considered malformed or invalid based on inclusion/exclusion of TLVs or contents of TLV fields (§8.2.2) | MUST NOT | 8.2.2 | **positive:** `unit/verify` [`TestRFC9552NLRIContentsNeverMakeItMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L147). **negative:** `unit/verify` [`TestRFC9552NLRIFramingErrorsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L181) |
| `RFC9552-8.2.2-2` | A BGP-LS Attribute MUST NOT be considered malformed or invalid based on inclusion/exclusion of TLVs or contents of TLV fields (§8.2.2) | MUST NOT | 8.2.2 | **positive:** `unit/verify` [`TestRFC9552UnorderedUnexpectedAttributeTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L54). **negative:** `unit/verify` [`TestRFC9552AttributeSyntaxStillRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L95) |
| `RFC9552-8.2.2-3` | A BGP-LS Propagator should not perform semantic validation of the Link-State NLRI or the BGP-LS Attribute (§8.2.2) | SHOULD NOT | 8.2.2 | **positive:** `unit/verify` [`TestRFC9552NLRIContentsNeverMakeItMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L148). **positive:** `unit/verify` [`TestRFC9552UnorderedUnexpectedAttributeTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L55). **negative:** `unit/verify` [`TestRFC9552AttributeSyntaxStillRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L96). **negative:** `unit/verify` [`TestRFC9552NLRIFramingErrorsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L182) |
| `RFC9552-8.2.2-4` | Skipable malformed NLRIs MUST be handled as "NLRI discard" (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC9552LinkStateNLRITLVOverrunIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L90). **positive:** `unit/verify` [`TestRFC9552LinkStateNodeDescriptorDuplicateSubTLVIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L128). **negative:** `unit/verify` [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L51) |
| `RFC9552-8.2.2-5` | Non-skipable malformed NLRIs MUST cause session reset when session is BGP-LS only or AFI/SAFI disable is not possible (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC9552LinkStateNLRILengthOverrunResetsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L204). **negative:** `unit/verify` [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L52) |
| `RFC9552-8.2.2-6` | Skipable malformed BGP-LS Attribute MUST be handled as "Attribute Discard" (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC9552BGPLSAttributeTLVOverrunDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L111). **positive:** `unit/verify` [`TestRFC9552BGPLSAttributeTrailingOctetsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L152). **negative:** `unit/verify` [`TestRFC9552BGPLSAttributeWellFormedIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L78) |
| `RFC9552-5.3-2` | When BGP-LS Attribute exceeds max message during propagation, MUST apply Attribute Discard and MUST discard BGP-LS Attribute first (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** when a forwarded UPDATE exceeds the destination's maximum message size and the split fails -- which is what a single BGP-LS NLRI larger than the limit produces (internal/component/bgp/message/chunk_mp_nlri.go:133) -- fwdBody logs the failure and drops the whole UPDATE (internal/component/bgp/reactor/forward_body.go:57-60, :95-97). Nothing discards the BGP-LS Attribute first, so the Attribute Discard this clause mandates never happens |
| `RFC9552-8.2.6-1` | An implementation MUST have the means to limit inbound updates (§8.2.6) | MUST | 8.2.6 | **positive:** `unit/verify` [`TestBGPLSPrefixCountCountsNLRIsNotPrefixBytes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_prefix_limit_test.go#L39). **negative:** `unit/verify` [`TestBGPLSPrefixLimitActuallyFires`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_prefix_limit_test.go#L100) |
| `RFC9552-5.2.2-6` | Upper bits of OSPF Multi-Topology ID MUST be 0, values 0-127 only (§5.2.2) | MUST | 5.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze sets no bit of a Multi-Topology ID because it emits no TLV 263. LinkDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:200) and PrefixDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:264) write none, grep -rn "TLVMultiTopologyID" --include=*.go matches only the constant declaration at internal/component/bgp/plugins/nlri/ls/types.go:207, and on receipt the MT-ID is masked to its low 12 bits rather than rejected (internal/component/bgp/plugins/nlri/ls/plugin.go:379); ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC9552-5.1-7` | BGP-LS Consumer MUST NOT send information back to BGP-LS Producers/Propagators (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 9552 Section 5.1 defines the BGP-LS Consumer as an application or process that is not a BGP Speaker; ze is a BGP Speaker and implements no Consumer that could feed link-state information back. It holds no BGP-LS content of its own to send: ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC9552-5.2.1.1-1` | The same node MUST NOT be represented by two keys (§5.2.1.1) | MUST NOT | 5.2.1.1 | **positive:** `unit/verify` [`TestSameNodeHasOneKeyWhateverTheStorageOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/types_descriptor_key_test.go#L186). **negative:** `unit/verify` [`TestNodeDescriptorWriteToMatchesBytes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/types_descriptor_key_test.go#L156) |
| `RFC9552-5.2.1.1-2` | Two different nodes MUST NOT be represented by the same key (§5.2.1.1) | MUST NOT | 5.2.1.1 | **positive:** `unit/verify` [`TestNodeDescriptorEncodesLegalZeroKeyFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/types_descriptor_key_test.go#L20). **negative:** `unit/verify` [`TestNodeDescriptorKeepsBackboneDistinctFromAreaLess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/types_descriptor_key_test.go#L57) |
| `RFC9552-5.2.2.1-1` | When used as a Link or Prefix Descriptor for IS-IS, the Bits R are reserved and MUST be set to 0 when originated and ignored on receipt (§5.2.2.1) | MUST | 5.2.2.1 | **positive:** `unit/verify` [`TestRFC9552ISISMTIDReservedBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L314). **negative:** no negative test. **{single-polarity}:** ze writes no Multi-Topology Identifier TLV, so no originated R bit exists for a negative case -- LinkDescriptor.WriteTo and PrefixDescriptor.WriteTo emit no TLV 263 (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:200, internal/component/bgp/plugins/nlri/ls/types_descriptor.go:264). On receipt the R bits are masked off rather than rejected, in both the prefix and the link descriptor walk (internal/component/bgp/plugins/nlri/ls/plugin.go:405, internal/component/bgp/plugins/nlri/ls/plugin.go:519), so there is no rejection path a negative test could exercise |
| `RFC9552-8.2.2-9` | A BGP-LS Speaker MUST perform the listed syntactic validation of the Link-State NLRI to determine if it is malformed (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC9552LinkStateNLRILengthOverrunResetsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L203). **positive:** `unit/verify` [`TestRFC9552LinkStateNLRIOutOfOrderTLVsAreDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L164). **positive:** `unit/verify` [`TestRFC9552LinkStateNLRITLVOverrunIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L89). **positive:** `unit/verify` [`TestRFC9552LinkStateNodeDescriptorDuplicateSubTLVIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L126). **negative:** `unit/verify` [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L50) |
| `RFC9552-8.2.2-10` | A BGP-LS Speaker MUST perform the listed syntactic validation of the BGP-LS Attribute to determine if it is malformed (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestRFC9552BGPLSAttributeTLVOverrunDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L110). **positive:** `unit/verify` [`TestRFC9552BGPLSAttributeTrailingOctetsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L151). **negative:** `unit/verify` [`TestRFC9552BGPLSAttributeWellFormedIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L77) |
| `RFC9552-8.2.3-5` | An implementation MUST allow the operator to configure an 8-octet BGP-LS Instance-ID (§8.2.3) | MUST | 8.2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the Instance-ID is a BGP-LS Producer's configuration -- it is the value a Producer stamps into the 8-octet Identifier field of the Link-State NLRI, and Section 5.2 assigns it so that each IGP domain a Producer reports is uniquely identified. ze is never a Producer: the four NLRI constructors that take the Identifier, NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:102,:192,:204), have no caller outside their own package, the bgp-ls families register no InProcessRouteEncoder (internal/component/bgp/plugins/nlri/ls/register.go), and ze derives no link-state from its IS-IS or OSPF. Same ground as the other Producer-side obligations of this summary. Disclosed in docs/features/rfc-status.md |
| `RFC9552-8.2.6-2` | An operator MUST define an import policy that drops all updates from peers that are only serving BGP-LS Consumers (§8.2.6) | MUST | 8.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the sentence binds the operator, not the implementation -- Section 8.2.6 reads "An operator MUST define an import policy to limit inbound updates", and assigns the implementation's own share to the next sentence, "An implementation MUST have the means to limit inbound updates", which this summary gates separately as RFC9552-8.2.6-1. ze provides the means this policy needs: a bgp/policy/family-filter instance naming the bgp-ls family with action remove, referenced from a peer's import chain, rejects every BGP-LS UPDATE that peer sends (parseFamilyFilters and handleFilterUpdate, internal/component/bgp/plugins/filter_family/config.go:30, handler.go:49). Which peers only serve BGP-LS Consumers is knowledge ze does not hold and cannot derive. Disclosed in docs/features/rfc-status.md |
| `RFC9552-5.1-8` | TLVs within the BGP-LS Attribute SHOULD be ordered ascending by Type (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.2-9` | "Direct" and "Static configuration" protocol types SHOULD be used when BGP-LS is sourcing local information (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.2.1.4-3` | Implementations SHOULD support advertisement of BGP-LS Identifier sub-TLV (513) for backward compatibility (§5.2.1.4) | SHOULD | 5.2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.3-3` | BGP-LS Attribute SHOULD only be included with Link-State NLRIs (§5.3) | SHOULD | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.2.3.2-1` | A router SHOULD advertise an IP Prefix NLRI for each of its BGP next hops (§5.2.3.2) | SHOULD | 5.2.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.3.1.3-1` | FQDN or subset strongly RECOMMENDED for Node Name and Link Name (§5.3.1.3, §5.3.2.7) | SHOULD | 5.3.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.3.2.2-2` | MPLS Protocol Mask TLV SHOULD only be used with Protocol-IDs 4 or 5 (§5.3.2.2) | SHOULD | 5.3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.5-2` | Next hop in MP_REACH_NLRI SHOULD match BGP session address family (§5.5) | SHOULD | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.6-1` | Implementation SHOULD provide means to inject inter-AS links into BGP-LS (§5.6) | SHOULD | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.1.5-1` | Distribution of Link-State NLRIs SHOULD be limited to a single administrative domain (§8.1.5) | SHOULD | 8.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.9-2` | A BGP-LS Producer SHOULD withdraw all link-state objects when the node is determined unreachable (§5.9) | SHOULD | 5.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.2.2-7` | Preserve and propagate Link-State NLRIs in UPDATE without BGP-LS Attribute (§8.2.2) | SHOULD | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.2.2-8` | Log a message for any errors found during syntax validation (§8.2.2) | SHOULD | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.2.2-11` | A non-skipable malformed NLRI SHOULD be handled as AFI/SAFI disable when another AFI/SAFI is advertised over the same session (§8.2.2) | SHOULD | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.2.3-1` | An implementation SHOULD let the operator name the neighbors that Link-State NLRIs are advertised to and accepted from (§8.2.3) | SHOULD | 8.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.2.3-2` | An implementation SHOULD let the operator set the maximum rate at which Link-State NLRIs are advertised and withdrawn (§8.2.3) | SHOULD | 8.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.2.3-3` | An implementation SHOULD let the operator set the maximum number of Link-State NLRIs stored in the Routing Information Base (§8.2.3) | SHOULD | 8.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.2.3-4` | An implementation SHOULD let the operator create abstracted topologies, with a different abstraction for each neighbor (§8.2.3) | SHOULD | 8.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.2.3-6` | An implementation SHOULD let the operator configure the Autonomous System Number and the BGP-LS identifiers (§8.2.3) | SHOULD | 8.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.2.3-7` | An implementation SHOULD let the operator set a 4096-byte size limit for a BGP-LS UPDATE message, or a larger value when every BGP-LS Speaker supports the extended message size (§8.2.3) | SHOULD | 8.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.2-10` | BGP-LS Instance-ID 0 is RECOMMENDED when there is only a single protocol instance (§5.2) | RECOMMENDED | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.2.1.4-4` | Default value of 0 is RECOMMENDED for BGP-LS Identifier sub-TLV when advertised (§5.2.1.4) | RECOMMENDED | 5.2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.3-4` | Implementations support extended message size for BGP (RFC 8654) (§5.3) | RECOMMENDED | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.1.1-1` | Operators deploying BGP-LS enable two or more BGP-LS Producers in each IGP flooding domain (§8.1.1) | RECOMMENDED | 8.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.1.1-2` | Dedicated route reflectors for BGP-LS distribution (§8.1.1) | RECOMMENDED | 8.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-8.1.1-3` | Separation of BGP instances or separate BGP sessions for Link-State information when no dedicated RRs (§8.1.1) | SHOULD | 8.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.2.2-7` | An implementation MAY suppress advertisement of a Link NLRI unless IGP has verified two-way connectivity (§5.2.2) | MAY | 5.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9552-5.2.1-2` | Auxiliary Router-IDs MAY be included in the link attribute in addition to mandatory node attribute (§5.2.1) | MAY | 5.2.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9552-5.2-3`](#rfc9552-5.2-3) For all information derived from other protocols, the corresponding Protocol-ID MUST be used (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze selects no Protocol-ID because it derives no link-state from an IGP. ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions; the Protocol-ID on every BGP-LS route ze holds arrives on the wire and is parsed at internal/component/bgp/plugins/nlri/ls/types.go:316 |
| [`RFC9552-5.2-4`](#rfc9552-5.2-4) The network operator MUST assign the same BGP-LS Instance-IDs on all BGP-LS Producers within a given IGP domain (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze assigns no BGP-LS Instance-ID on any Producer because it runs none. The 8-octet Identifier is only ever read from the wire (internal/component/bgp/plugins/nlri/ls/types.go:317), there is no config surface that sets it (grep -rn "bgp-ls" --include=*.yang returns nothing), and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC9552-5.2-5`](#rfc9552-5.2-5) Unique BGP-LS Instance-IDs MUST be assigned to routing protocol instances operating in different IGP domains (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze assigns no BGP-LS Instance-ID to any routing protocol instance, so it cannot make two domains collide. The Identifier is only ever read from the wire (internal/component/bgp/plugins/nlri/ls/types.go:317), no config surface sets it (grep -rn "bgp-ls" --include=*.yang returns nothing), and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC9552-5.2-6`](#rfc9552-5.2-6) When modifying TLVs in NLRI, Producer MUST withdraw the old NLRI via MP_UNREACH_NLRI first (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze modifies no TLV in any NLRI because it builds none. The NLRI bytes it re-advertises are the received bytes: ParseBGPLS caches the wire slice (internal/component/bgp/plugins/nlri/ls/types.go:333), WriteTo copies it back out (internal/component/bgp/plugins/nlri/ls/types_nlri.go:63) and the transit path forwards the payload verbatim (internal/component/bgp/reactor/forward_body.go:64); ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC9552-5.2.2-1`](#rfc9552-5.2.2-1) When interface/neighbor addresses are present, address TLVs MUST be included in Link Descriptors (§5.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze derives no link from an IGP, so it fills no Link Descriptor: LinkDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:200) runs only from BGPLSLink.WriteTo (internal/component/bgp/plugins/nlri/ls/types_nlri.go:181), and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions, so no interface or neighbor address ever reaches a Link Descriptor ze builds |
| [`RFC9552-5.2.2-2`](#rfc9552-5.2.2-2) Link Local/Remote Identifiers TLV MUST NOT be included in Link Descriptor when addresses are present (§5.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze derives no link from an IGP, so it fills no Link Descriptor: LinkDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:200) runs only from BGPLSLink.WriteTo (internal/component/bgp/plugins/nlri/ls/types_nlri.go:181), and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions, so ze never faces the choice between address TLVs and the Link Local/Remote Identifiers TLV |
| [`RFC9552-5.2.2-3`](#rfc9552-5.2.2-3) IPv4/IPv6 link-local addresses MUST NOT be carried in TLVs 259/260/261/262 (§5.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze derives no link from an IGP, so it fills no Link Descriptor: LinkDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:200) runs only from BGPLSLink.WriteTo (internal/component/bgp/plugins/nlri/ls/types_nlri.go:181), and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions, so ze places no address, link-local or otherwise, in TLV 259, 260, 261 or 262 |
| [`RFC9552-5.2.2-4`](#rfc9552-5.2.2-4) Link Local/Remote Identifiers TLV MUST be included when only link-local identifiers are available (§5.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze derives no link from an IGP, so it fills no Link Descriptor: LinkDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:200) runs only from BGPLSLink.WriteTo (internal/component/bgp/plugins/nlri/ls/types_nlri.go:181), and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions, so ze never has a link with only link-local identifiers to describe |
| [`RFC9552-5.2.2-5`](#rfc9552-5.2.2-5) Multi-Topology Identifier TLV MUST be included as Link Descriptor if link is associated with non-default topology (§5.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze emits no Multi-Topology Identifier TLV at all. LinkDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:200) and PrefixDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:264) write no TLV 263, grep -rn "TLVMultiTopologyID" --include=*.go matches only the constant declaration at internal/component/bgp/plugins/nlri/ls/types.go:207, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC9552-5.2.3.1-1`](#rfc9552-5.2.3.1-1) OSPF Route Type TLV MUST be included when the route type is signaled in the underlying LSA or determinable from another LSA (§5.2.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 9552 raises the OSPF Route Type TLV from optional to mandatory, but ze advertises no OSPF prefix through BGP-LS and writes no TLV 264: PrefixDescriptor.WriteTo emits only the IP Reachability Information TLV (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:264) and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC9552-5.2.1-1`](#rfc9552-5.2.1-1) Auxiliary TE Router-IDs (TLVs 1028/1029) MUST be included in the node attribute (§5.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze assembles no BGP-LS Attribute: every Ls*TLV struct is built only inside its own decode function (internal/component/bgp/plugins/nlri/ls/attr_node.go:179, internal/component/bgp/plugins/nlri/ls/attr_link.go:59) and a grep for composite-literal construction of LsIPv4RouterIDRemote, LsIPv4RouterIDLocal, LsIGPFlags and LsPrefixMetric outside _test.go finds only those decode functions, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC9552-5.3.2.1-1`](#rfc9552-5.3.2.1-1) All auxiliary Router-IDs of both local and remote nodes MUST be included in the link attribute of each Link NLRI (§5.3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze assembles no BGP-LS Attribute: every Ls*TLV struct is built only inside its own decode function (internal/component/bgp/plugins/nlri/ls/attr_node.go:179, internal/component/bgp/plugins/nlri/ls/attr_link.go:59) and a grep for composite-literal construction of LsIPv4RouterIDRemote, LsIPv4RouterIDLocal, LsIGPFlags and LsPrefixMetric outside _test.go finds only those decode functions, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC9552-5.3.2.2-1`](#rfc9552-5.3.2.2-1) MPLS Protocol Mask TLV MUST NOT be included in NLRIs with Protocol-IDs 1-3, 6 (IS-IS, OSPF) (§5.3.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no MPLS Protocol Mask TLV. TLV 1094 is neither encoded nor registered for decode: grep -rn "1094" over internal/component/bgp/plugins/nlri/ls/ returns nothing and register_attr.go (internal/component/bgp/plugins/nlri/ls/register_attr.go:8) registers no 1094 decoder, so no NLRI ze emits or reads carries the TLV this clause restricts |
| [`RFC9552-5.3-1`](#rfc9552-5.3-1) BGP-LS Producers MUST ensure TLVs in BGP-LS Attribute do not cause UPDATE to exceed maximum message size (§5.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze puts no TLV in a BGP-LS Attribute, so it cannot push an UPDATE past the maximum message size that way. ze assembles no BGP-LS Attribute: every Ls*TLV struct is built only inside its own decode function (internal/component/bgp/plugins/nlri/ls/attr_node.go:179, internal/component/bgp/plugins/nlri/ls/attr_link.go:59) and a grep for composite-literal construction of LsIPv4RouterIDRemote, LsIPv4RouterIDLocal, LsIGPFlags and LsPrefixMetric outside _test.go finds only those decode functions, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC9552-5.9-1`](#rfc9552-5.9-1) Producer MUST re-advertise link-state objects after an unreachable node becomes reachable again (§5.9) | no test | no test carries this requirement id; annotated {not-applicable}: ze tracks no IGP node reachability for BGP-LS and therefore neither withdraws nor re-advertises link-state objects on a reachability change. ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC9552-5.4-1`](#rfc9552-5.4-1) Private Use TLV value MUST include 4-octet Enterprise Code as first field (§5.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze emits no Private Use TLV. No encoder writes a type in the 65000-65535 range (register_attr.go registers only assigned code points, internal/component/bgp/plugins/nlri/ls/register_attr.go:8), grep -rni "enterprise" over internal/component/bgp/plugins/nlri/ls/ returns nothing, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC9552-5.2.3-1`](#rfc9552-5.2.3-1) Trailing bits of IP prefix in IP Reachability Information TLV MUST be 0 (§5.2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no IP Reachability Information TLV. PrefixDescriptor.IPReachabilityInfo is copied verbatim from whatever its caller supplied (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:266), no non-test caller supplies one, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions; a received prefix is read as sent (internal/component/bgp/plugins/nlri/ls/plugin.go:593) |
| [`RFC9552-5.3-2`](#rfc9552-5.3-2) When BGP-LS Attribute exceeds max message during propagation, MUST apply Attribute Discard and MUST discard BGP-LS Attribute first (§5.3) | {gap}, no test | when a forwarded UPDATE exceeds the destination's maximum message size and the split fails -- which is what a single BGP-LS NLRI larger than the limit produces (internal/component/bgp/message/chunk_mp_nlri.go:133) -- fwdBody logs the failure and drops the whole UPDATE (internal/component/bgp/reactor/forward_body.go:57-60, :95-97). Nothing discards the BGP-LS Attribute first, so the Attribute Discard this clause mandates never happens |
| [`RFC9552-5.2.2-6`](#rfc9552-5.2.2-6) Upper bits of OSPF Multi-Topology ID MUST be 0, values 0-127 only (§5.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze sets no bit of a Multi-Topology ID because it emits no TLV 263. LinkDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:200) and PrefixDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:264) write none, grep -rn "TLVMultiTopologyID" --include=*.go matches only the constant declaration at internal/component/bgp/plugins/nlri/ls/types.go:207, and on receipt the MT-ID is masked to its low 12 bits rather than rejected (internal/component/bgp/plugins/nlri/ls/plugin.go:379); ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC9552-5.1-7`](#rfc9552-5.1-7) BGP-LS Consumer MUST NOT send information back to BGP-LS Producers/Propagators (§5.1) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 9552 Section 5.1 defines the BGP-LS Consumer as an application or process that is not a BGP Speaker; ze is a BGP Speaker and implements no Consumer that could feed link-state information back. It holds no BGP-LS content of its own to send: ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC9552-8.2.3-5`](#rfc9552-8.2.3-5) An implementation MUST allow the operator to configure an 8-octet BGP-LS Instance-ID (§8.2.3) | no test | no test carries this requirement id; annotated {not-applicable}: the Instance-ID is a BGP-LS Producer's configuration -- it is the value a Producer stamps into the 8-octet Identifier field of the Link-State NLRI, and Section 5.2 assigns it so that each IGP domain a Producer reports is uniquely identified. ze is never a Producer: the four NLRI constructors that take the Identifier, NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:102,:192,:204), have no caller outside their own package, the bgp-ls families register no InProcessRouteEncoder (internal/component/bgp/plugins/nlri/ls/register.go), and ze derives no link-state from its IS-IS or OSPF. Same ground as the other Producer-side obligations of this summary. Disclosed in docs/features/rfc-status.md |
| [`RFC9552-8.2.6-2`](#rfc9552-8.2.6-2) An operator MUST define an import policy that drops all updates from peers that are only serving BGP-LS Consumers (§8.2.6) | no test | no test carries this requirement id; annotated {not-applicable}: the sentence binds the operator, not the implementation -- Section 8.2.6 reads "An operator MUST define an import policy to limit inbound updates", and assigns the implementation's own share to the next sentence, "An implementation MUST have the means to limit inbound updates", which this summary gates separately as RFC9552-8.2.6-1. ze provides the means this policy needs: a bgp/policy/family-filter instance naming the bgp-ls family with action remove, referenced from a peer's import chain, rejects every BGP-LS UPDATE that peer sends (parseFamilyFilters and handleFilterUpdate, internal/component/bgp/plugins/filter_family/config.go:30, handler.go:49). Which peers only serve BGP-LS Consumers is knowledge ze does not hold and cannot derive. Disclosed in docs/features/rfc-status.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9552-5.1-1`](#rfc9552-5.1-1)

All TLVs within the NLRI MUST be ordered in ascending order by TLV Type (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNoDescriptorEmitsADescendingTLVSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L73) | unit/verify | unproven |
| positive | [`TestLinkDescriptorOrdersMixedFamilyAddressesAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L39) | unit/verify | unproven |

### [`RFC9552-5.1-2`](#rfc9552-5.1-2)

Same-type TLVs MUST be ordered ascending by Length, then ascending by Value (lexicographic binary) (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSRv6SIDOrderIsLengthBeforeValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L170) | unit/verify | unproven |
| positive | [`TestNodeDescriptorOrdersRepeatedSRv6SIDs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L124) | unit/verify | unproven |

### [`RFC9552-5.1-3`](#rfc9552-5.1-3)

Unknown and unsupported TLV types MUST be preserved and propagated within both the NLRI and the BGP-LS Attribute (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7752MalformedTLVNotPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L103) | unit/verify | unproven |
| positive | [`TestRFC7752UnknownTLVPreservedAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L65) | unit/verify | unproven |

### [`RFC9552-5.1-4`](#rfc9552-5.1-4)

Presence of unknown or unexpected TLVs MUST NOT result in the NLRI or BGP-LS Attribute being considered malformed (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552AttributeSyntaxStillRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L94) | unit/verify | unproven |
| positive | [`TestRFC9552UnorderedUnexpectedAttributeTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L53) | unit/verify | unproven |

### [`RFC9552-5.1-5`](#rfc9552-5.1-5)

NLRIs having TLVs that do not follow ordering rules MUST be considered malformed by a BGP-LS Propagator (§5.1)

Audit verdict: not audited: no reader has judged these tests

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

No test carries RFC9552-5.2-3, so no unit is bound to it.

### [`RFC9552-5.2-4`](#rfc9552-5.2-4)

The network operator MUST assign the same BGP-LS Instance-IDs on all BGP-LS Producers within a given IGP domain (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.2-4, so no unit is bound to it.

### [`RFC9552-5.2-5`](#rfc9552-5.2-5)

Unique BGP-LS Instance-IDs MUST be assigned to routing protocol instances operating in different IGP domains (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.2-5, so no unit is bound to it.

### [`RFC9552-5.2-6`](#rfc9552-5.2-6)

When modifying TLVs in NLRI, Producer MUST withdraw the old NLRI via MP_UNREACH_NLRI first (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.2-6, so no unit is bound to it.

### [`RFC9552-5.2-7`](#rfc9552-5.2-7)

BGP speakers MUST use BGP Capabilities Advertisement to ensure both peers can process Link-State NLRIs (§5.2)

Audit verdict: not audited: no reader has judged these tests

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

At most one instance of each sub-TLV type MUST be present in any Node Descriptor (§5.2.1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L54) | unit/verify | unproven |
| positive | [`TestRFC9552LinkStateNodeDescriptorDuplicateSubTLVIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L127) | unit/verify | unproven |

### [`RFC9552-5.2.1.4-2`](#rfc9552-5.2.1.4-2)

Sub-TLVs within a Node Descriptor MUST be arranged in ascending order by sub-TLV type (§5.2.1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7752NodeDescriptorSubTLVsAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L165) | unit/verify | unproven |

### [`RFC9552-5.2.2-1`](#rfc9552-5.2.2-1)

When interface/neighbor addresses are present, address TLVs MUST be included in Link Descriptors (§5.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.2.2-1, so no unit is bound to it.

### [`RFC9552-5.2.2-2`](#rfc9552-5.2.2-2)

Link Local/Remote Identifiers TLV MUST NOT be included in Link Descriptor when addresses are present (§5.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.2.2-2, so no unit is bound to it.

### [`RFC9552-5.2.2-3`](#rfc9552-5.2.2-3)

IPv4/IPv6 link-local addresses MUST NOT be carried in TLVs 259/260/261/262 (§5.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.2.2-3, so no unit is bound to it.

### [`RFC9552-5.2.2-4`](#rfc9552-5.2.2-4)

Link Local/Remote Identifiers TLV MUST be included when only link-local identifiers are available (§5.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.2.2-4, so no unit is bound to it.

### [`RFC9552-5.2.2-5`](#rfc9552-5.2.2-5)

Multi-Topology Identifier TLV MUST be included as Link Descriptor if link is associated with non-default topology (§5.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.2.2-5, so no unit is bound to it.

### [`RFC9552-5.2.3.1-1`](#rfc9552-5.2.3.1-1)

OSPF Route Type TLV MUST be included when the route type is signaled in the underlying LSA or determinable from another LSA (§5.2.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.2.3.1-1, so no unit is bound to it.

### [`RFC9552-5.2.1-1`](#rfc9552-5.2.1-1)

Auxiliary TE Router-IDs (TLVs 1028/1029) MUST be included in the node attribute (§5.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.2.1-1, so no unit is bound to it.

### [`RFC9552-5.3.2.1-1`](#rfc9552-5.3.2.1-1)

All auxiliary Router-IDs of both local and remote nodes MUST be included in the link attribute of each Link NLRI (§5.3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.3.2.1-1, so no unit is bound to it.

### [`RFC9552-5.3.2.2-1`](#rfc9552-5.3.2.2-1)

MPLS Protocol Mask TLV MUST NOT be included in NLRIs with Protocol-IDs 1-3, 6 (IS-IS, OSPF) (§5.3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.3.2.2-1, so no unit is bound to it.

### [`RFC9552-5.3.2.3-1`](#rfc9552-5.3.2.3-1)

High-order bits of TE Default Metric MUST be padded with zero if source is less than 32 bits (§5.3.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7752TEDefaultMetricZeroPadded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L204) | unit/verify | unproven |

### [`RFC9552-5.5-1`](#rfc9552-5.5-1)

The next-hop address MUST be encoded as described in RFC 4760 (§5.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7752BGPLSNextHopFollowsRFC4760`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7752_bgpls_test.go#L25) | unit/verify | unproven |

### [`RFC9552-5.3-1`](#rfc9552-5.3-1)

BGP-LS Producers MUST ensure TLVs in BGP-LS Attribute do not cause UPDATE to exceed maximum message size (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.3-1, so no unit is bound to it.

### [`RFC9552-5.9-1`](#rfc9552-5.9-1)

Producer MUST re-advertise link-state objects after an unreachable node becomes reachable again (§5.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.9-1, so no unit is bound to it.

### [`RFC9552-5.4-1`](#rfc9552-5.4-1)

Private Use TLV value MUST include 4-octet Enterprise Code as first field (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.4-1, so no unit is bound to it.

### [`RFC9552-5.2.3-1`](#rfc9552-5.2.3-1)

Trailing bits of IP prefix in IP Reachability Information TLV MUST be 0 (§5.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.2.3-1, so no unit is bound to it.

### [`RFC9552-5.3.2.3-2`](#rfc9552-5.3.2.3-2)

IS-IS small metric (1-byte IGP Metric): 2 MSBs MUST be set to 0 by originator (§5.3.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552IGPMetricWidthGrowsInsteadOfTruncating`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L252) | unit/verify | unproven |
| positive | [`TestRFC9552ISISSmallMetricTwoMSBsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L218) | unit/verify | unproven |

### [`RFC9552-8.2.2-1`](#rfc9552-8.2.2-1)

A Link-State NLRI MUST NOT be considered malformed or invalid based on inclusion/exclusion of TLVs or contents of TLV fields (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NLRIFramingErrorsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L181) | unit/verify | unproven |
| positive | [`TestRFC9552NLRIContentsNeverMakeItMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L147) | unit/verify | unproven |

### [`RFC9552-8.2.2-2`](#rfc9552-8.2.2-2)

A BGP-LS Attribute MUST NOT be considered malformed or invalid based on inclusion/exclusion of TLVs or contents of TLV fields (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552AttributeSyntaxStillRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L95) | unit/verify | unproven |
| positive | [`TestRFC9552UnorderedUnexpectedAttributeTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L54) | unit/verify | unproven |

### [`RFC9552-8.2.2-3`](#rfc9552-8.2.2-3)

A BGP-LS Propagator should not perform semantic validation of the Link-State NLRI or the BGP-LS Attribute (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552AttributeSyntaxStillRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L96) | unit/verify | unproven |
| negative | [`TestRFC9552NLRIFramingErrorsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L182) | unit/verify | unproven |
| positive | [`TestRFC9552NLRIContentsNeverMakeItMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L148) | unit/verify | unproven |
| positive | [`TestRFC9552UnorderedUnexpectedAttributeTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L55) | unit/verify | unproven |

### [`RFC9552-8.2.2-4`](#rfc9552-8.2.2-4)

Skipable malformed NLRIs MUST be handled as "NLRI discard" (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L51) | unit/verify | unproven |
| positive | [`TestRFC9552LinkStateNLRITLVOverrunIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L90) | unit/verify | unproven |
| positive | [`TestRFC9552LinkStateNodeDescriptorDuplicateSubTLVIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L128) | unit/verify | unproven |

### [`RFC9552-8.2.2-5`](#rfc9552-8.2.2-5)

Non-skipable malformed NLRIs MUST cause session reset when session is BGP-LS only or AFI/SAFI disable is not possible (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L52) | unit/verify | unproven |
| positive | [`TestRFC9552LinkStateNLRILengthOverrunResetsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L204) | unit/verify | unproven |

### [`RFC9552-8.2.2-6`](#rfc9552-8.2.2-6)

Skipable malformed BGP-LS Attribute MUST be handled as "Attribute Discard" (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552BGPLSAttributeWellFormedIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L78) | unit/verify | unproven |
| positive | [`TestRFC9552BGPLSAttributeTLVOverrunDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L111) | unit/verify | unproven |
| positive | [`TestRFC9552BGPLSAttributeTrailingOctetsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L152) | unit/verify | unproven |

### [`RFC9552-5.3-2`](#rfc9552-5.3-2)

When BGP-LS Attribute exceeds max message during propagation, MUST apply Attribute Discard and MUST discard BGP-LS Attribute first (§5.3)

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

Upper bits of OSPF Multi-Topology ID MUST be 0, values 0-127 only (§5.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-5.2.2-6, so no unit is bound to it.

### [`RFC9552-5.1-7`](#rfc9552-5.1-7)

BGP-LS Consumer MUST NOT send information back to BGP-LS Producers/Propagators (§5.1)

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

When used as a Link or Prefix Descriptor for IS-IS, the Bits R are reserved and MUST be set to 0 when originated and ignored on receipt (§5.2.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9552ISISMTIDReservedBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_test.go#L314) | unit/verify | unproven |

### [`RFC9552-8.2.2-9`](#rfc9552-8.2.2-9)

A BGP-LS Speaker MUST perform the listed syntactic validation of the Link-State NLRI to determine if it is malformed (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L50) | unit/verify | unproven |
| positive | [`TestRFC9552LinkStateNLRILengthOverrunResetsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L203) | unit/verify | unproven |
| positive | [`TestRFC9552LinkStateNLRIOutOfOrderTLVsAreDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L164) | unit/verify | unproven |
| positive | [`TestRFC9552LinkStateNLRITLVOverrunIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L89) | unit/verify | unproven |
| positive | [`TestRFC9552LinkStateNodeDescriptorDuplicateSubTLVIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_nlri_test.go#L126) | unit/verify | unproven |

### [`RFC9552-8.2.2-10`](#rfc9552-8.2.2-10)

A BGP-LS Speaker MUST perform the listed syntactic validation of the BGP-LS Attribute to determine if it is malformed (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552BGPLSAttributeWellFormedIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L77) | unit/verify | unproven |
| positive | [`TestRFC9552BGPLSAttributeTLVOverrunDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L110) | unit/verify | unproven |
| positive | [`TestRFC9552BGPLSAttributeTrailingOctetsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_test.go#L151) | unit/verify | unproven |

### [`RFC9552-8.2.3-5`](#rfc9552-8.2.3-5)

An implementation MUST allow the operator to configure an 8-octet BGP-LS Instance-ID (§8.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-8.2.3-5, so no unit is bound to it.

### [`RFC9552-8.2.6-2`](#rfc9552-8.2.6-2)

An operator MUST define an import policy that drops all updates from peers that are only serving BGP-LS Consumers (§8.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9552-8.2.6-2, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 9552, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 9552, so its obligations are stated where they were written.
