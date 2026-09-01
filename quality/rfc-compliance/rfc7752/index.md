# RFC 7752 - North-Bound Distribution of Link-State and Traffic Engineering (TE) Information Using BGP

Partial. Every requirement this repository extracted from RFC 7752, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 53.3% | 8 of 15 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 20.0% | 3 of 15 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 15 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 19 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 26 | of 51 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 11 | of 26 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 26.7% | 4 of 15 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 15 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 51 |
| Gated MUST-level | 26 |
| Obligations that bind Ze | 15 |
| Not applicable, so out of scope | 11 |
| Declared gaps | 4 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 19 |
| Tagged units | 19 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7752.md` |
| Requirement shard | `rfc/requirements/rfc7752.md` |
| RFC text | `rfc/full/rfc7752.txt` |

## Enrolment

Enrolled: North-Bound Distribution of Link-State and TE Information Using BGP

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Ze is a BGP-LS consumer and transit speaker: Node/Link/Prefix NLRI and node, link and prefix attribute TLV decode (`internal/component/bgp/plugins/nlri/ls`), (AFI 16388, SAFI 71/72) family registration and Multiprotocol capability negotiation, opaque-key RIB storage that keeps routing universes apart by Identifier, RFC 4760 next-hop encoding, unknown-TLV preservation with byte-identical re-advertisement, and Section 6.2.2 TLV syntactic checks. Requirements bound per line in [`rfc/short/rfc7752.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc7752.md).

**What the ledger says remains**

Four MUST gaps annotated in [`rfc/short/rfc7752.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc7752.md): [`RFC7752-3.3-1`](#rfc7752-3.3-1) -- the UPDATE decoder reads a type-29 attribute for every address family instead of ignoring it outside link-state; [`RFC7752-6.2.2-1`](#rfc7752-6.2.2-1) -- no RFC 7606 validator is registered for attribute code 29, so a malformed BGP-LS attribute is not attribute-discarded; [`RFC7752-6.2.6-1`](#rfc7752-6.2.6-1) -- the per-family prefix maximum applies to bgp-ls but counts with a CIDR byte-walk that does not parse RFC 7752 TLV NLRI; and [`RFC7752-8-1`](#rfc7752-8-1) -- ze models no BGP-LS consumer peer, so any peer negotiating bgp-ls has its UPDATEs accepted. All origination obligations (Protocol-ID selection, Identifier stamping, auxiliary Router-IDs, node keying, ASN/BGP-LS-ID uniqueness, OSPF MT-ID bits, MPLS Protocol Mask restriction, prefix attribute reflection) are `{not-applicable}`: ze derives no link-state from its IS-IS or OSPF and has no BGP-LS config route surface.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 8 | one part of the gated population |
| Annotated instead of tested | 18 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **26** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (8):** [`RFC7752-3.1-1`](#rfc7752-3.1-1), [`RFC7752-3.1-2`](#rfc7752-3.1-2), [`RFC7752-3.1-3`](#rfc7752-3.1-3), [`RFC7752-3.2-1`](#rfc7752-3.2-1), [`RFC7752-3.2-4`](#rfc7752-3.2-4), [`RFC7752-6.2.2-2`](#rfc7752-6.2.2-2), [`RFC7752-3.2-5`](#rfc7752-3.2-5), [`RFC7752-3.2-6`](#rfc7752-3.2-6)

**Annotated instead of tested (18):** [`RFC7752-3.2-2`](#rfc7752-3.2-2), [`RFC7752-3.2-3`](#rfc7752-3.2-3), [`RFC7752-3.2.1-1`](#rfc7752-3.2.1-1), [`RFC7752-3.2.1.1-1`](#rfc7752-3.2.1.1-1), [`RFC7752-3.2.1.1-2`](#rfc7752-3.2.1.1-2), [`RFC7752-3.2.1.4-1`](#rfc7752-3.2.1.4-1), [`RFC7752-3.2.1.4-2`](#rfc7752-3.2.1.4-2), [`RFC7752-3.2.1.4-3`](#rfc7752-3.2.1.4-3), [`RFC7752-3.2.1.5-1`](#rfc7752-3.2.1.5-1), [`RFC7752-3.3-1`](#rfc7752-3.3-1), [`RFC7752-3.3.2.1-1`](#rfc7752-3.3.2.1-1), [`RFC7752-3.3.2.2-1`](#rfc7752-3.3.2.2-1), [`RFC7752-3.3.2.3-1`](#rfc7752-3.3.2.3-1), [`RFC7752-3.3.3-1`](#rfc7752-3.3.3-1), [`RFC7752-3.4-1`](#rfc7752-3.4-1), [`RFC7752-6.2.2-1`](#rfc7752-6.2.2-1), [`RFC7752-6.2.6-1`](#rfc7752-6.2.6-1), [`RFC7752-8-1`](#rfc7752-8-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7752-3.1-1` | Unrecognized TLV types must be preserved and propagated (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC7752UnknownTLVPreservedAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L64). **negative:** `unit/verify` [`TestRFC7752MalformedTLVNotPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L101) |
| `RFC7752-3.1-2` | All TLVs must be ordered in ascending order by TLV Type (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestLinkDescriptorOrdersMixedFamilyAddressesAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L42). **negative:** `unit/verify` [`TestNoDescriptorEmitsADescendingTLVSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L76) |
| `RFC7752-3.1-3` | Same-type TLVs must be ordered in ascending order of value (leftmost octet comparison) (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestNodeDescriptorOrdersRepeatedSRv6SIDs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L127). **negative:** `unit/verify` [`TestSRv6SIDOrderIsLengthBeforeValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L173) |
| `RFC7752-3.2-1` | Two BGP speakers must use BGP Capabilities Advertisement to ensure both can process Link-State NLRI (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC7752BGPLSCapabilityAdvertisedAndNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7752_bgpls_test.go#L28). **negative:** `unit/verify` [`TestRFC7752BGPLSCapabilityNotNegotiatedWhenPeerSilent`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7752_bgpls_test.go#L62) |
| `RFC7752-3.2-2` | For information derived from other protocols, the corresponding Protocol-ID must be used (Section 3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze selects no Protocol-ID because it derives no link-state from an IGP. ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions; the Protocol-ID on every BGP-LS route ze holds arrives on the wire and is parsed at internal/component/bgp/plugins/nlri/ls/types.go:316 |
| `RFC7752-3.2-3` | NLRIs from the same routing universe must have the same Identifier value (Section 3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze stamps no Identifier because it originates no Link-State NLRI. ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions; the Identifier is read from the wire at internal/component/bgp/plugins/nlri/ls/types.go:317 |
| `RFC7752-3.2-4` | NLRIs with different Identifier values must be considered from different routing universes (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC7752DifferentIdentifierIsDifferentRoutingUniverse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc7752_bgpls_test.go#L45). **negative:** `unit/verify` [`TestRFC7752SameIdentifierIsOneRoutingUniverse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc7752_bgpls_test.go#L78) |
| `RFC7752-3.2.1-1` | Auxiliary Router-IDs must be included in the link attribute (Section 3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze assembles no BGP-LS attribute: every Ls*TLV struct is built only inside its own decode function (internal/component/bgp/plugins/nlri/ls/attr_node.go:179, internal/component/bgp/plugins/nlri/ls/attr_link.go:59) and a grep for composite-literal construction of LsIPv4RouterIDRemote, LsIPv4RouterIDLocal, LsIGPFlags and LsPrefixMetric outside _test.go finds only those decode functions, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC7752-3.2.1.1-1` | The same node must not be represented by two keys (Section 3.2.1.1) | MUST | 3.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze assigns no node keys. It builds no link-state topology from BGP-LS: received routes land in the non-CIDR opaque backend keyed by raw NLRI bytes (internal/component/bgp/plugins/rib/storage/familyrib.go:278) and grep -rn "BGPLSNode" --include=*.go outside internal/component/bgp/plugins/nlri/ls/ returns nothing, so no consumer turns a Node NLRI into a keyed topology object |
| `RFC7752-3.2.1.1-2` | Two different nodes must not be represented by the same key (Section 3.2.1.1) | MUST | 3.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze assigns no node keys. It builds no link-state topology from BGP-LS: received routes land in the non-CIDR opaque backend keyed by raw NLRI bytes (internal/component/bgp/plugins/rib/storage/familyrib.go:278) and grep -rn "BGPLSNode" --include=*.go outside internal/component/bgp/plugins/nlri/ls/ returns nothing, so no consumer turns a Node NLRI into a keyed topology object |
| `RFC7752-3.2.1.4-1` | The combination of ASN and BGP-LS ID must be globally unique (Section 3.2.1.4) | MUST | 3.2.1.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze allocates no (ASN, BGP-LS Identifier) tuple. Both values only ever arrive on the wire and are parsed into NodeDescriptor (internal/component/bgp/plugins/nlri/ls/types.go:411), there is no config surface that sets them (grep -rn "bgp-ls" --include=*.yang returns nothing), and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC7752-3.2.1.4-2` | All BGP-LS speakers within an IGP flooding-set must use the same ASN, BGP-LS ID tuple (Section 3.2.1.4) | MUST | 3.2.1.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze advertises no (ASN, BGP-LS Identifier) tuple into any IGP flooding set. It joins no IGP flooding set as a BGP-LS speaker and holds no such tuple: the pair is decoded from received NLRIs only (internal/component/bgp/plugins/nlri/ls/types.go:411) and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC7752-3.2.1.4-3` | Sub-TLVs within a Node Descriptor must be arranged in ascending order by sub-TLV type (Section 3.2.1.4) | MUST | 3.2.1.4 | **positive:** `unit/verify` [`TestRFC7752NodeDescriptorSubTLVsAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L164). **negative:** no negative test. **{single-polarity}:** NodeDescriptor.WriteTo emits sub-TLVs 512, 513, 514, 515, 516 and 517 in that fixed ascending order (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:98), and the ordering duty falls on the sender: parseNodeDescriptorTLVs (internal/component/bgp/plugins/nlri/ls/types.go:391) accepts sub-TLVs in any order on receipt, so there is no out-of-order input for ze to reject |
| `RFC7752-3.2.1.5-1` | For OSPF Multi-Topology ID, upper 9 bits must be set to 0 (Section 3.2.1.5) | MUST | 3.2.1.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no encoder for this TLV exists anywhere, which is what decides the classification -- the same rule stated at RFC7752-3.1-2 and applied there and at RFC7752-3.1-3: gap versus not-applicable turns on whether encoding code for the obligation EXISTS, never on whether it is reachable from production. ze emits no Multi-Topology Identifier TLV at all: LinkDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:200) and PrefixDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:264) write no TLV 263, and grep -rn "TLVMultiTopologyID" --include=*.go matches only the constant declaration at internal/component/bgp/plugins/nlri/ls/types.go:207, so there is no code that could set the upper 9 bits wrong. Reachability, recorded here only as context and NOT as the reason: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go and the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70) -- that is equally true of the RFC7752-3.1-2 and RFC7752-3.1-3 encoders, which are gaps |
| `RFC7752-3.3-1` | BGP-LS attribute must be ignored for all non-Link-State address families (Section 3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parsePathAttributesZe decodes a type-29 attribute into the "bgp-ls" key for every UPDATE it walks, with no address-family context: the MP_REACH value is handed back separately and its AFI/SAFI is read only after the attribute loop ends, so a type-29 attribute carried on an IPv4 unicast UPDATE is decoded rather than ignored (internal/component/bgp/cli/decode_update.go:199) |
| `RFC7752-3.3.2.1-1` | All auxiliary Router-IDs of both local and remote node must be included in link attribute of each Link NLRI (Section 3.3.2.1) | MUST | 3.3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze assembles no BGP-LS attribute: every Ls*TLV struct is built only inside its own decode function (internal/component/bgp/plugins/nlri/ls/attr_node.go:179, internal/component/bgp/plugins/nlri/ls/attr_link.go:59) and a grep for composite-literal construction of LsIPv4RouterIDRemote, LsIPv4RouterIDLocal, LsIGPFlags and LsPrefixMetric outside _test.go finds only those decode functions, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC7752-3.3.2.2-1` | MPLS Protocol Mask TLV must not be included in NLRIs with Protocol-IDs 1-3, 6 (IS-IS, OSPF) (Section 3.3.2.2) | MUST NOT | 3.3.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no MPLS Protocol Mask TLV. TLV 1094 is neither encoded nor registered for decode: grep -rn "1094" over internal/component/bgp/plugins/nlri/ls/ returns nothing and register_attr.go (internal/component/bgp/plugins/nlri/ls/register_attr.go:5) registers no 1094 decoder, so no NLRI ze emits or reads carries the TLV this clause restricts |
| `RFC7752-3.3.2.3-1` | If source protocol uses metric width < 32 bits, high-order bits must be padded with zero (Section 3.3.2.3) | MUST | 3.3.2.3 | **positive:** `unit/verify` [`TestRFC7752TEDefaultMetricZeroPadded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L203). **negative:** no negative test. **{single-polarity}:** LsTEDefaultMetric.WriteTo always emits a 4-octet value (internal/component/bgp/plugins/nlri/ls/attr_link.go:226), so a metric sourced from a narrower width lands zero-padded in the high-order octets and ze owns no short-form TE metric encoder whose output could be rejected |
| `RFC7752-3.3.3-1` | Prefix IGP attributes (metric, route tags, etc.) must be reflected into the BGP-LS attribute (Section 3.3.3) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze reflects no IGP prefix attributes into a BGP-LS attribute because it runs no BGP-LS origination. ze assembles no BGP-LS attribute: every Ls*TLV struct is built only inside its own decode function (internal/component/bgp/plugins/nlri/ls/attr_node.go:179, internal/component/bgp/plugins/nlri/ls/attr_link.go:59) and a grep for composite-literal construction of LsIPv4RouterIDRemote, LsIPv4RouterIDLocal, LsIGPFlags and LsPrefixMetric outside _test.go finds only those decode functions, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC7752-3.4-1` | The next-hop address must be encoded as described in RFC 4760 (Section 3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestRFC7752BGPLSNextHopFollowsRFC4760`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7752_bgpls_test.go#L24). **negative:** no negative test. **{single-polarity}:** MPReachNLRI.WriteTo is family agnostic and lays out AFI, SAFI, next-hop length, next-hop, the zero reserved octet and then the NLRI for AFI 16388 exactly as RFC 4760 Section 3 specifies (internal/core/bgp/attribute/mpnlri.go:154); ValidNextHopLens returns nil for AFI 16388 (internal/core/bgp/attribute/mpnlri.go:305), so ze runs no BGP-LS next-hop length check and holds no rejection path to drive negatively |
| `RFC7752-6.2.2-1` | Malformed BGP-LS attributes must use the "Attribute Discard" action per RFC 7606, Section 2 (Section 6.2.2) | MUST | 6.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the RFC 7606 validator table registers no entry for attribute code 29 (internal/component/bgp/message/rfc7606.go:414), so validateAttribute returns nil for a malformed BGP-LS attribute (internal/component/bgp/message/rfc7606.go:433) and the attribute rides through the UPDATE path instead of being discarded |
| `RFC7752-6.2.2-2` | Implementation must perform syntactic checks (TLV sum vs attribute length, fixed-length TLV sizes) (Section 6.2.2) | MUST | 6.2.2 | **positive:** `unit/verify` [`TestRFC7752AttrSyntacticChecks`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L134). **negative:** `unit/verify` [`TestRFC7752MalformedTLVNotPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L102) |
| `RFC7752-6.2.6-1` | Implementation must have the means to limit inbound updates (Section 6.2.6) | MUST | 6.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the per-family prefix maximum is mandatory for bgp-ls and enforced per family (internal/component/bgp/reactor/config.go:648, internal/component/bgp/reactor/session_prefix.go:288), but the count it compares comes from countPrefixEntries, a [prefix-length][address] CIDR walk (internal/component/bgp/reactor/session_prefix.go:497) that never parses an RFC 7752 type-length NLRI, so the number checked against the configured bgp-ls maximum bears no relation to the BGP-LS NLRI count in the UPDATE |
| `RFC7752-8-1` | A BGP speaker must not accept updates from a consumer peer (Section 8) | MUST NOT | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze models no BGP-LS consumer peer. The per-family peer knobs are enable, disable, require and ignore (internal/component/bgp/reactor/config.go:596), and a negotiated family is accepted in both directions (internal/core/bgp/capability/negotiated.go:413), so any peer that negotiates bgp-ls has its Link-State UPDATEs accepted whatever role it plays |
| `RFC7752-3.2-5` | All non-VPN link, node, and prefix information shall be encoded using AFI 16388 / SAFI 71 (Section 3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestRFC7752NonVPNFamilyIsAFI16388SAFI71`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L229). **negative:** `unit/verify` [`TestRFC7752NonLinkStateFamilyRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L265) |
| `RFC7752-3.2-6` | VPN link, node, and prefix information shall be encoded using AFI 16388 / SAFI 72 (Section 3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestRFC7752VPNFamilyIsAFI16388SAFI72`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L247). **negative:** `unit/verify` [`TestRFC7752NonLinkStateFamilyRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L266) |
| `RFC7752-3.2-7` | "Direct" and "Static configuration" protocol types should be used when BGP-LS is sourcing local information (Section 3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.2-8` | If a given protocol does not support multiple routing universes, it should set the Identifier field to 0 (Section 3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.2.1.4-4` | BGP-LS speakers within the IGP domain should use the same ASN, BGP-LS ID tuple (Section 3.2.1.4) | SHOULD | 3.2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.2.1.5-2` | Reserved bits in Multi-Topology ID should be set to 0 on origination and ignored on receipt (Section 3.2.1.5) | SHOULD | 3.2.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.2.3.2-1` | A router should advertise an IP Prefix NLRI for each of its BGP next hops (Section 3.2.3.2) | SHOULD | 3.2.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.3-2` | BGP-LS attribute should only be included with Link-State NLRIs (Section 3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.3.1.3-1` | FQDN or subset strongly recommended for Node Name and Link Name (Section 3.3.1.3, 3.3.2.7) | SHOULD | 3.3.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.3.2.2-2` | MPLS Protocol Mask TLV should only be used with Protocol-IDs 4 or 5 (Section 3.3.2.2) | SHOULD | 3.3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.3.3-2` | Prefix Attribute TLVs should be used when advertising NLRI types 3 and 4 only (Section 3.3.3) | SHOULD | 3.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.4-2` | Next hop in MP_REACH_NLRI should match BGP session address family (Section 3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.5-1` | Implementation should provide a means to inject inter-AS links into BGP-LS (Section 3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-6.1.2-1` | Configuration parameters should be initialized to default values (Section 6.1.2) | SHOULD | 6.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-6.1.5-1` | Distribution of Link-State NLRIs should be limited to a single admin domain (Section 6.1.5) | SHOULD | 6.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-6.1.6-1` | Implementation should allow operator to list neighbors exchanging Link-State NLRIs (Section 6.1.6) | SHOULD | 6.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-6.2.3-1` | Implementation should allow operator to specify neighbors, max rate, max RIB entries, abstracted topologies, Instance-ID, ASN/BGP-LS ID (Section 6.2.3) | SHOULD | 6.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-6.2.5-1` | Implementation should provide statistics (total NLRIs sent/received, per-neighbor, errors, locally originated) (Section 6.2.5) | SHOULD | 6.2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-6.2.6-2` | Operator should define an import policy to drop all updates from consumer peers (Section 6.2.6) | SHOULD | 6.2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-8-2` | Operator should employ a mechanism to protect BGP speaker against DDoS attacks from consumers (Section 8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.2-9` | Implementation may make the Identifier configurable for a given protocol (Section 3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.2-10` | OSPF and IS-IS may run multiple routing protocol instances over the same link (Section 3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.2.2-1` | Link local/remote identifiers may be included in the link attribute (Section 3.2.2) | MAY | 3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.2.1.5-3` | The MT-ID TLV may be present in Link/Prefix Descriptor or Node NLRI attribute (Section 3.2.1.5) | MAY | 3.2.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.2.3.1-1` | OSPF Route Type TLV is optional and may be present in Prefix NLRI (Section 3.2.3.1) | MAY | 3.2.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-6.1.5-2` | A network operator may use a dedicated Route-Reflector infrastructure (Section 6.1.5) | MAY | 6.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-6.2.5-2` | Implementation may enhance statistics by recording peak per-second counts (Section 6.2.5) | MAY | 6.2.5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7752-3.2-2`](#rfc7752-3.2-2) For information derived from other protocols, the corresponding Protocol-ID must be used (Section 3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze selects no Protocol-ID because it derives no link-state from an IGP. ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions; the Protocol-ID on every BGP-LS route ze holds arrives on the wire and is parsed at internal/component/bgp/plugins/nlri/ls/types.go:316 |
| [`RFC7752-3.2-3`](#rfc7752-3.2-3) NLRIs from the same routing universe must have the same Identifier value (Section 3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze stamps no Identifier because it originates no Link-State NLRI. ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions; the Identifier is read from the wire at internal/component/bgp/plugins/nlri/ls/types.go:317 |
| [`RFC7752-3.2.1-1`](#rfc7752-3.2.1-1) Auxiliary Router-IDs must be included in the link attribute (Section 3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze assembles no BGP-LS attribute: every Ls*TLV struct is built only inside its own decode function (internal/component/bgp/plugins/nlri/ls/attr_node.go:179, internal/component/bgp/plugins/nlri/ls/attr_link.go:59) and a grep for composite-literal construction of LsIPv4RouterIDRemote, LsIPv4RouterIDLocal, LsIGPFlags and LsPrefixMetric outside _test.go finds only those decode functions, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC7752-3.2.1.1-1`](#rfc7752-3.2.1.1-1) The same node must not be represented by two keys (Section 3.2.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze assigns no node keys. It builds no link-state topology from BGP-LS: received routes land in the non-CIDR opaque backend keyed by raw NLRI bytes (internal/component/bgp/plugins/rib/storage/familyrib.go:278) and grep -rn "BGPLSNode" --include=*.go outside internal/component/bgp/plugins/nlri/ls/ returns nothing, so no consumer turns a Node NLRI into a keyed topology object |
| [`RFC7752-3.2.1.1-2`](#rfc7752-3.2.1.1-2) Two different nodes must not be represented by the same key (Section 3.2.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze assigns no node keys. It builds no link-state topology from BGP-LS: received routes land in the non-CIDR opaque backend keyed by raw NLRI bytes (internal/component/bgp/plugins/rib/storage/familyrib.go:278) and grep -rn "BGPLSNode" --include=*.go outside internal/component/bgp/plugins/nlri/ls/ returns nothing, so no consumer turns a Node NLRI into a keyed topology object |
| [`RFC7752-3.2.1.4-1`](#rfc7752-3.2.1.4-1) The combination of ASN and BGP-LS ID must be globally unique (Section 3.2.1.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze allocates no (ASN, BGP-LS Identifier) tuple. Both values only ever arrive on the wire and are parsed into NodeDescriptor (internal/component/bgp/plugins/nlri/ls/types.go:411), there is no config surface that sets them (grep -rn "bgp-ls" --include=*.yang returns nothing), and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC7752-3.2.1.4-2`](#rfc7752-3.2.1.4-2) All BGP-LS speakers within an IGP flooding-set must use the same ASN, BGP-LS ID tuple (Section 3.2.1.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze advertises no (ASN, BGP-LS Identifier) tuple into any IGP flooding set. It joins no IGP flooding set as a BGP-LS speaker and holds no such tuple: the pair is decoded from received NLRIs only (internal/component/bgp/plugins/nlri/ls/types.go:411) and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC7752-3.2.1.5-1`](#rfc7752-3.2.1.5-1) For OSPF Multi-Topology ID, upper 9 bits must be set to 0 (Section 3.2.1.5) | no test | no test carries this requirement id; annotated {not-applicable}: no encoder for this TLV exists anywhere, which is what decides the classification -- the same rule stated at RFC7752-3.1-2 and applied there and at RFC7752-3.1-3: gap versus not-applicable turns on whether encoding code for the obligation EXISTS, never on whether it is reachable from production. ze emits no Multi-Topology Identifier TLV at all: LinkDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:200) and PrefixDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:264) write no TLV 263, and grep -rn "TLVMultiTopologyID" --include=*.go matches only the constant declaration at internal/component/bgp/plugins/nlri/ls/types.go:207, so there is no code that could set the upper 9 bits wrong. Reachability, recorded here only as context and NOT as the reason: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go and the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70) -- that is equally true of the RFC7752-3.1-2 and RFC7752-3.1-3 encoders, which are gaps |
| [`RFC7752-3.3-1`](#rfc7752-3.3-1) BGP-LS attribute must be ignored for all non-Link-State address families (Section 3.3) | {gap}, no test | parsePathAttributesZe decodes a type-29 attribute into the "bgp-ls" key for every UPDATE it walks, with no address-family context: the MP_REACH value is handed back separately and its AFI/SAFI is read only after the attribute loop ends, so a type-29 attribute carried on an IPv4 unicast UPDATE is decoded rather than ignored (internal/component/bgp/cli/decode_update.go:199) |
| [`RFC7752-3.3.2.1-1`](#rfc7752-3.3.2.1-1) All auxiliary Router-IDs of both local and remote node must be included in link attribute of each Link NLRI (Section 3.3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze assembles no BGP-LS attribute: every Ls*TLV struct is built only inside its own decode function (internal/component/bgp/plugins/nlri/ls/attr_node.go:179, internal/component/bgp/plugins/nlri/ls/attr_link.go:59) and a grep for composite-literal construction of LsIPv4RouterIDRemote, LsIPv4RouterIDLocal, LsIGPFlags and LsPrefixMetric outside _test.go finds only those decode functions, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC7752-3.3.2.2-1`](#rfc7752-3.3.2.2-1) MPLS Protocol Mask TLV must not be included in NLRIs with Protocol-IDs 1-3, 6 (IS-IS, OSPF) (Section 3.3.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no MPLS Protocol Mask TLV. TLV 1094 is neither encoded nor registered for decode: grep -rn "1094" over internal/component/bgp/plugins/nlri/ls/ returns nothing and register_attr.go (internal/component/bgp/plugins/nlri/ls/register_attr.go:5) registers no 1094 decoder, so no NLRI ze emits or reads carries the TLV this clause restricts |
| [`RFC7752-3.3.3-1`](#rfc7752-3.3.3-1) Prefix IGP attributes (metric, route tags, etc.) must be reflected into the BGP-LS attribute (Section 3.3.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze reflects no IGP prefix attributes into a BGP-LS attribute because it runs no BGP-LS origination. ze assembles no BGP-LS attribute: every Ls*TLV struct is built only inside its own decode function (internal/component/bgp/plugins/nlri/ls/attr_node.go:179, internal/component/bgp/plugins/nlri/ls/attr_link.go:59) and a grep for composite-literal construction of LsIPv4RouterIDRemote, LsIPv4RouterIDLocal, LsIGPFlags and LsPrefixMetric outside _test.go finds only those decode functions, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC7752-6.2.2-1`](#rfc7752-6.2.2-1) Malformed BGP-LS attributes must use the "Attribute Discard" action per RFC 7606, Section 2 (Section 6.2.2) | {gap}, no test | the RFC 7606 validator table registers no entry for attribute code 29 (internal/component/bgp/message/rfc7606.go:414), so validateAttribute returns nil for a malformed BGP-LS attribute (internal/component/bgp/message/rfc7606.go:433) and the attribute rides through the UPDATE path instead of being discarded |
| [`RFC7752-6.2.6-1`](#rfc7752-6.2.6-1) Implementation must have the means to limit inbound updates (Section 6.2.6) | {gap}, no test | the per-family prefix maximum is mandatory for bgp-ls and enforced per family (internal/component/bgp/reactor/config.go:648, internal/component/bgp/reactor/session_prefix.go:288), but the count it compares comes from countPrefixEntries, a [prefix-length][address] CIDR walk (internal/component/bgp/reactor/session_prefix.go:497) that never parses an RFC 7752 type-length NLRI, so the number checked against the configured bgp-ls maximum bears no relation to the BGP-LS NLRI count in the UPDATE |
| [`RFC7752-8-1`](#rfc7752-8-1) A BGP speaker must not accept updates from a consumer peer (Section 8) | {gap}, no test | ze models no BGP-LS consumer peer. The per-family peer knobs are enable, disable, require and ignore (internal/component/bgp/reactor/config.go:596), and a negotiated family is accepted in both directions (internal/core/bgp/capability/negotiated.go:413), so any peer that negotiates bgp-ls has its Link-State UPDATEs accepted whatever role it plays |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7752-3.1-1`](#rfc7752-3.1-1)

Unrecognized TLV types must be preserved and propagated (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7752MalformedTLVNotPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L101) | unit/verify | unproven |
| positive | [`TestRFC7752UnknownTLVPreservedAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L64) | unit/verify | unproven |

### [`RFC7752-3.1-2`](#rfc7752-3.1-2)

All TLVs must be ordered in ascending order by TLV Type (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNoDescriptorEmitsADescendingTLVSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L76) | unit/verify | unproven |
| positive | [`TestLinkDescriptorOrdersMixedFamilyAddressesAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L42) | unit/verify | unproven |

### [`RFC7752-3.1-3`](#rfc7752-3.1-3)

Same-type TLVs must be ordered in ascending order of value (leftmost octet comparison) (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSRv6SIDOrderIsLengthBeforeValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L173) | unit/verify | unproven |
| positive | [`TestNodeDescriptorOrdersRepeatedSRv6SIDs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L127) | unit/verify | unproven |

### [`RFC7752-3.2-1`](#rfc7752-3.2-1)

Two BGP speakers must use BGP Capabilities Advertisement to ensure both can process Link-State NLRI (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7752BGPLSCapabilityNotNegotiatedWhenPeerSilent`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7752_bgpls_test.go#L62) | unit/verify | unproven |
| positive | [`TestRFC7752BGPLSCapabilityAdvertisedAndNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7752_bgpls_test.go#L28) | unit/verify | unproven |

### [`RFC7752-3.2-2`](#rfc7752-3.2-2)

For information derived from other protocols, the corresponding Protocol-ID must be used (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.2-2, so no unit is bound to it.

### [`RFC7752-3.2-3`](#rfc7752-3.2-3)

NLRIs from the same routing universe must have the same Identifier value (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.2-3, so no unit is bound to it.

### [`RFC7752-3.2-4`](#rfc7752-3.2-4)

NLRIs with different Identifier values must be considered from different routing universes (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7752SameIdentifierIsOneRoutingUniverse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc7752_bgpls_test.go#L78) | unit/verify | unproven |
| positive | [`TestRFC7752DifferentIdentifierIsDifferentRoutingUniverse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc7752_bgpls_test.go#L45) | unit/verify | unproven |

### [`RFC7752-3.2.1-1`](#rfc7752-3.2.1-1)

Auxiliary Router-IDs must be included in the link attribute (Section 3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.2.1-1, so no unit is bound to it.

### [`RFC7752-3.2.1.1-1`](#rfc7752-3.2.1.1-1)

The same node must not be represented by two keys (Section 3.2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.2.1.1-1, so no unit is bound to it.

### [`RFC7752-3.2.1.1-2`](#rfc7752-3.2.1.1-2)

Two different nodes must not be represented by the same key (Section 3.2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.2.1.1-2, so no unit is bound to it.

### [`RFC7752-3.2.1.4-1`](#rfc7752-3.2.1.4-1)

The combination of ASN and BGP-LS ID must be globally unique (Section 3.2.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.2.1.4-1, so no unit is bound to it.

### [`RFC7752-3.2.1.4-2`](#rfc7752-3.2.1.4-2)

All BGP-LS speakers within an IGP flooding-set must use the same ASN, BGP-LS ID tuple (Section 3.2.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.2.1.4-2, so no unit is bound to it.

### [`RFC7752-3.2.1.4-3`](#rfc7752-3.2.1.4-3)

Sub-TLVs within a Node Descriptor must be arranged in ascending order by sub-TLV type (Section 3.2.1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7752NodeDescriptorSubTLVsAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L164) | unit/verify | unproven |

### [`RFC7752-3.2.1.5-1`](#rfc7752-3.2.1.5-1)

For OSPF Multi-Topology ID, upper 9 bits must be set to 0 (Section 3.2.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.2.1.5-1, so no unit is bound to it.

### [`RFC7752-3.3-1`](#rfc7752-3.3-1)

BGP-LS attribute must be ignored for all non-Link-State address families (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.3-1, so no unit is bound to it.

### [`RFC7752-3.3.2.1-1`](#rfc7752-3.3.2.1-1)

All auxiliary Router-IDs of both local and remote node must be included in link attribute of each Link NLRI (Section 3.3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.3.2.1-1, so no unit is bound to it.

### [`RFC7752-3.3.2.2-1`](#rfc7752-3.3.2.2-1)

MPLS Protocol Mask TLV must not be included in NLRIs with Protocol-IDs 1-3, 6 (IS-IS, OSPF) (Section 3.3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.3.2.2-1, so no unit is bound to it.

### [`RFC7752-3.3.2.3-1`](#rfc7752-3.3.2.3-1)

If source protocol uses metric width < 32 bits, high-order bits must be padded with zero (Section 3.3.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7752TEDefaultMetricZeroPadded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L203) | unit/verify | unproven |

### [`RFC7752-3.3.3-1`](#rfc7752-3.3.3-1)

Prefix IGP attributes (metric, route tags, etc.) must be reflected into the BGP-LS attribute (Section 3.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.3.3-1, so no unit is bound to it.

### [`RFC7752-3.4-1`](#rfc7752-3.4-1)

The next-hop address must be encoded as described in RFC 4760 (Section 3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7752BGPLSNextHopFollowsRFC4760`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7752_bgpls_test.go#L24) | unit/verify | unproven |

### [`RFC7752-6.2.2-1`](#rfc7752-6.2.2-1)

Malformed BGP-LS attributes must use the "Attribute Discard" action per RFC 7606, Section 2 (Section 6.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-6.2.2-1, so no unit is bound to it.

### [`RFC7752-6.2.2-2`](#rfc7752-6.2.2-2)

Implementation must perform syntactic checks (TLV sum vs attribute length, fixed-length TLV sizes) (Section 6.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7752MalformedTLVNotPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L102) | unit/verify | unproven |
| positive | [`TestRFC7752AttrSyntacticChecks`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L134) | unit/verify | unproven |

### [`RFC7752-6.2.6-1`](#rfc7752-6.2.6-1)

Implementation must have the means to limit inbound updates (Section 6.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-6.2.6-1, so no unit is bound to it.

### [`RFC7752-8-1`](#rfc7752-8-1)

A BGP speaker must not accept updates from a consumer peer (Section 8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-8-1, so no unit is bound to it.

### [`RFC7752-3.2-5`](#rfc7752-3.2-5)

All non-VPN link, node, and prefix information shall be encoded using AFI 16388 / SAFI 71 (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7752NonLinkStateFamilyRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L265) | unit/verify | unproven |
| positive | [`TestRFC7752NonVPNFamilyIsAFI16388SAFI71`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L229) | unit/verify | unproven |

### [`RFC7752-3.2-6`](#rfc7752-3.2-6)

VPN link, node, and prefix information shall be encoded using AFI 16388 / SAFI 72 (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7752NonLinkStateFamilyRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L266) | unit/verify | unproven |
| positive | [`TestRFC7752VPNFamilyIsAFI16388SAFI72`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L247) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 7752, so no reviewer has walked its text sentence by sentence.

## Superseded

RFC 7752 is obsoleted by RFC 9552.

| Requirement | Disposition | Now stated at | Reason |
|---|---|---|---|
| [`RFC7752-3.1-1`](#rfc7752-3.1-1) Unrecognized TLV types must be preserved and propagated (Section 3.1) | restated | RFC9552-5.1-3 | RFC 9552 Section 5.1 widens the same obligation, from unrecognized TLVs to unknown and unsupported ones, and states it over both the NLRI and the BGP-LS Attribute |
| [`RFC7752-3.1-2`](#rfc7752-3.1-2) All TLVs must be ordered in ascending order by TLV Type (Section 3.1) | restated | RFC9552-5.1-1 | RFC 9552 Section 5.1 keeps the ascending-Type order as a MUST for TLVs within the NLRI, and states the same order over the BGP-LS Attribute as a SHOULD at RFC9552-5.1-8 |
| [`RFC7752-3.1-3`](#rfc7752-3.1-3) Same-type TLVs must be ordered in ascending order of value (leftmost octet comparison) (Section 3.1) | restated | RFC9552-5.1-2 | RFC 9552 Section 5.1 changes the tie-break rule for same-type TLVs, from leftmost-octet value comparison to ascending Length and then ascending Value |
| [`RFC7752-3.2-1`](#rfc7752-3.2-1) Two BGP speakers must use BGP Capabilities Advertisement to ensure both can process Link-State NLRI (Section 3.2) | restated | RFC9552-5.2-7 | RFC 9552 Section 5.2 keeps the obligation to use BGP Capabilities Advertisement before exchanging Link-State NLRIs |
| [`RFC7752-3.2-2`](#rfc7752-3.2-2) For information derived from other protocols, the corresponding Protocol-ID must be used (Section 3.2) | restated | RFC9552-5.2-3 | RFC 9552 Section 5.2 keeps the sentence unchanged, that information derived from another protocol MUST carry that protocol's Protocol-ID |
| [`RFC7752-3.2-3`](#rfc7752-3.2-3) NLRIs from the same routing universe must have the same Identifier value (Section 3.2) | restated | RFC9552-5.2-4 | RFC 9552 Section 5.2 renames the Identifier field's content to the BGP-LS Instance-ID and puts the obligation on the operator, who MUST assign the same BGP-LS Instance-ID on all BGP-LS Producers within one IGP domain |
| [`RFC7752-3.2-4`](#rfc7752-3.2-4) NLRIs with different Identifier values must be considered from different routing universes (Section 3.2) | restated | RFC9552-5.2-5 | RFC 9552 Section 5.2 restates the same rule from the other side, that unique BGP-LS Instance-IDs MUST be assigned to routing protocol instances in different IGP domains |
| [`RFC7752-3.2.1-1`](#rfc7752-3.2.1-1) Auxiliary Router-IDs must be included in the link attribute (Section 3.2.1) | restated | RFC9552-5.2.1-1 | RFC 9552 Section 5.2.1 moves the obligation from the link attribute to the node attribute. Auxiliary TE Router-IDs (TLVs 1028 and 1029) MUST now be included in the node attribute, and RFC9552-5.2.1-2 lowers the link attribute to a MAY |
| [`RFC7752-3.2.1.1-1`](#rfc7752-3.2.1.1-1) The same node must not be represented by two keys (Section 3.2.1.1) | unextracted | §5.2.1.1 | RFC 9552 Section 5.2.1.1 states requirement (A) in the same words, that the same node MUST NOT be represented by two keys. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-3.2.1.1-2`](#rfc7752-3.2.1.1-2) Two different nodes must not be represented by the same key (Section 3.2.1.1) | unextracted | §5.2.1.1 | RFC 9552 Section 5.2.1.1 states requirement (B) in the same words, that two different nodes MUST NOT be represented by the same key. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-3.2.1.4-1`](#rfc7752-3.2.1.4-1) The combination of ASN and BGP-LS ID must be globally unique (Section 3.2.1.4) | dropped | not stated | RFC 9552 deprecates the BGP-LS Identifier sub-TLV (513). Its Section 5.2.1.4 table marks the code point deprecated and its Appendix A states the (ASN, BGP-LS ID) uniqueness rule in the past tense, because the BGP-LS Instance-ID carried in the Identifier field now provides that function. The obligation is gone, and RFC9552-5.2.1.4-3 keeps only a SHOULD to advertise the sub-TLV for compatibility with RFC 7752 implementations |
| [`RFC7752-3.2.1.4-2`](#rfc7752-3.2.1.4-2) All BGP-LS speakers within an IGP flooding-set must use the same ASN, BGP-LS ID tuple (Section 3.2.1.4) | dropped | not stated | same deprecation as RFC7752-3.2.1.4-1. RFC 9552 Appendix A records that BGP-LS Speakers within an IGP flooding-set had to use the same (ASN, BGP-LS ID) tuple, in the past tense, and states no such obligation of its own |
| [`RFC7752-3.2.1.4-3`](#rfc7752-3.2.1.4-3) Sub-TLVs within a Node Descriptor must be arranged in ascending order by sub-TLV type (Section 3.2.1.4) | restated | RFC9552-5.2.1.4-2 | RFC 9552 Section 5.2.1.4 keeps the ascending sub-TLV-type ordering rule for Node Descriptors |
| [`RFC7752-3.2.1.5-1`](#rfc7752-3.2.1.5-1) For OSPF Multi-Topology ID, upper 9 bits must be set to 0 (Section 3.2.1.5) | restated | RFC9552-5.2.2-6 | RFC 9552 moves the MT-ID TLV out of the Node Descriptor section into Section 5.2.2 and fixes the OSPF encoding, keeping the rule that an OSPF-derived value carries 0 in its upper bits |
| [`RFC7752-3.3-1`](#rfc7752-3.3-1) BGP-LS attribute must be ignored for all non-Link-State address families (Section 3.3) | dropped | not stated | RFC 9552 Section 5.3 replaces the sentence with a scoping statement, that the use of this attribute for other address families is outside the scope of this document. No MUST remains about ignoring the attribute on another address family |
| [`RFC7752-3.3.2.1-1`](#rfc7752-3.3.2.1-1) All auxiliary Router-IDs of both local and remote node must be included in link attribute of each Link NLRI (Section 3.3.2.1) | restated | RFC9552-5.3.2.1-1 | RFC 9552 Section 5.3.2.1 keeps the sentence unchanged, that all auxiliary Router-IDs of the local and the remote node MUST be included in the link attribute of each Link NLRI |
| [`RFC7752-3.3.2.2-1`](#rfc7752-3.3.2.2-1) MPLS Protocol Mask TLV must not be included in NLRIs with Protocol-IDs 1-3, 6 (IS-IS, OSPF) (Section 3.3.2.2) | restated | RFC9552-5.3.2.2-1 | RFC 9552 Section 5.3.2.2 keeps the MPLS Protocol Mask TLV out of NLRIs with Protocol-IDs 1 to 3 and 6 |
| [`RFC7752-3.3.2.3-1`](#rfc7752-3.3.2.3-1) If source protocol uses metric width < 32 bits, high-order bits must be padded with zero (Section 3.3.2.3) | restated | RFC9552-5.3.2.3-1 | RFC 9552 Section 5.3.2.3 keeps the zero-padding rule for a TE Default Metric sourced from a protocol narrower than 32 bits |
| [`RFC7752-3.3.3-1`](#rfc7752-3.3.3-1) Prefix IGP attributes (metric, route tags, etc.) must be reflected into the BGP-LS attribute (Section 3.3.3) | dropped | not stated | RFC 9552 Section 5.3.3 states the same sentence without the keyword. RFC 7752 wrote that the IGP attributes MUST be reflected into the BGP-LS attribute; RFC 9552 writes that they are advertised in the BGP-LS Attribute with Prefix NLRI types 3 and 4. The obligation became a description |
| [`RFC7752-3.4-1`](#rfc7752-3.4-1) The next-hop address must be encoded as described in RFC 4760 (Section 3.4) | restated | RFC9552-5.5-1 | RFC 9552 Section 5.5 keeps the sentence unchanged, that the next-hop address MUST be encoded as RFC 4760 describes |
| [`RFC7752-6.2.2-1`](#rfc7752-6.2.2-1) Malformed BGP-LS attributes must use the "Attribute Discard" action per RFC 7606, Section 2 (Section 6.2.2) | restated | RFC9552-8.2.2-6 | RFC 9552 Section 8.2.2 keeps Attribute Discard for a malformed BGP-LS Attribute the receiver can skip, and adds a session-reset path for the errors it cannot skip |
| [`RFC7752-6.2.2-2`](#rfc7752-6.2.2-2) Implementation must perform syntactic checks (TLV sum vs attribute length, fixed-length TLV sizes) (Section 6.2.2) | unextracted | §8.2.2 | RFC 9552 Section 8.2.2 states the obligation twice and in more detail, as two lists of syntactic validation a BGP-LS Speaker MUST perform, one over the Link-State NLRI and one over the BGP-LS Attribute. rfc/short/rfc9552.md declares no row for either list |
| [`RFC7752-6.2.6-1`](#rfc7752-6.2.6-1) Implementation must have the means to limit inbound updates (Section 6.2.6) | restated | RFC9552-8.2.6-1 | RFC 9552 Section 8.2.6 keeps the sentence unchanged, that an implementation MUST have the means to limit inbound updates |
| [`RFC7752-8-1`](#rfc7752-8-1) A BGP speaker must not accept updates from a consumer peer (Section 8) | unextracted | §8.2.6 | RFC 9552 moves the obligation off the speaker and onto the operator's import policy. Its Section 8.2.6 states that an operator MUST define an import policy that drops all updates from peers which only serve BGP-LS Consumers, and its Section 10 states the same intent without a keyword. rfc/short/rfc9552.md declares no row for the Section 8.2.6 import-policy MUST |
| [`RFC7752-3.2-5`](#rfc7752-3.2-5) All non-VPN link, node, and prefix information shall be encoded using AFI 16388 / SAFI 71 (Section 3.2) | restated | RFC9552-5.2-1 | RFC 9552 Section 5.2 keeps AFI 16388 / SAFI 71 for all non-VPN link, node and prefix information |
| [`RFC7752-3.2-6`](#rfc7752-3.2-6) VPN link, node, and prefix information shall be encoded using AFI 16388 / SAFI 72 (Section 3.2) | restated | RFC9552-5.2-2 | RFC 9552 Section 5.2 keeps AFI 16388 / SAFI 72 for VPN link, node and prefix information |
| [`RFC7752-3.2-7`](#rfc7752-3.2-7) "Direct" and "Static configuration" protocol types should be used when BGP-LS is sourcing local information (Section 3.2) | restated | RFC9552-5.2-9 | RFC 9552 Section 5.2 keeps the Direct and Static configuration protocol types for locally sourced information |
| [`RFC7752-3.2-8`](#rfc7752-3.2-8) If a given protocol does not support multiple routing universes, it should set the Identifier field to 0 (Section 3.2) | restated | RFC9552-5.2-10 | RFC 9552 Section 5.2 restates the default as a RECOMMENDED rather than a SHOULD, and rewrites the condition from a protocol without multiple routing universes to a network with a single protocol instance |
| [`RFC7752-3.2.1.4-4`](#rfc7752-3.2.1.4-4) BGP-LS speakers within the IGP domain should use the same ASN, BGP-LS ID tuple (Section 3.2.1.4) | dropped | not stated | same deprecation as RFC7752-3.2.1.4-1. With the BGP-LS Identifier sub-TLV deprecated, RFC 9552 states no obligation about a shared (ASN, BGP-LS ID) tuple across an IGP domain |
| [`RFC7752-3.2.1.5-2`](#rfc7752-3.2.1.5-2) Reserved bits in Multi-Topology ID should be set to 0 on origination and ignored on receipt (Section 3.2.1.5) | unextracted | §5.2.2.1 | RFC 9552 Section 5.2.2.1 raises the rule from SHOULD to MUST, that the reserved R bits are set to 0 when originated and ignored on receipt, and scopes it to a Link or Prefix Descriptor for IS-IS. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-3.2.3.2-1`](#rfc7752-3.2.3.2-1) A router should advertise an IP Prefix NLRI for each of its BGP next hops (Section 3.2.3.2) | restated | RFC9552-5.2.3.2-1 | RFC 9552 Section 5.2.3.2 keeps the recommendation that a router advertises an IP Prefix NLRI for each of its BGP next hops |
| [`RFC7752-3.3-2`](#rfc7752-3.3-2) BGP-LS attribute should only be included with Link-State NLRIs (Section 3.3) | restated | RFC9552-5.3-3 | RFC 9552 Section 5.3 keeps the sentence unchanged, that the BGP-LS Attribute SHOULD only be included with Link-State NLRIs |
| [`RFC7752-3.3.1.3-1`](#rfc7752-3.3.1.3-1) FQDN or subset strongly recommended for Node Name and Link Name (Section 3.3.1.3, 3.3.2.7) | restated | RFC9552-5.3.1.3-1 | RFC 9552 Sections 5.3.1.3 and 5.3.2.7 keep the FQDN recommendation for the Node Name and Link Name TLVs |
| [`RFC7752-3.3.2.2-2`](#rfc7752-3.3.2.2-2) MPLS Protocol Mask TLV should only be used with Protocol-IDs 4 or 5 (Section 3.3.2.2) | restated | RFC9552-5.3.2.2-2 | RFC 9552 Section 5.3.2.2 keeps the MPLS Protocol Mask TLV scoped to Protocol-IDs 4 and 5 |
| [`RFC7752-3.3.3-2`](#rfc7752-3.3.3-2) Prefix Attribute TLVs should be used when advertising NLRI types 3 and 4 only (Section 3.3.3) | dropped | not stated | RFC 9552 Section 5.3.3 folds the sentence into its opening description and states no SHOULD. RFC 7752 wrote that Prefix Attribute TLVs SHOULD be used when advertising NLRI types 3 and 4 only; RFC 9552 writes that the IGP attributes are advertised with Prefix NLRI types 3 and 4 |
| [`RFC7752-3.4-2`](#rfc7752-3.4-2) Next hop in MP_REACH_NLRI should match BGP session address family (Section 3.4) | restated | RFC9552-5.5-2 | RFC 9552 Section 5.5 keeps the recommendation that the MP_REACH_NLRI next hop matches the address family of the BGP session |
| [`RFC7752-3.5-1`](#rfc7752-3.5-1) Implementation should provide a means to inject inter-AS links into BGP-LS (Section 3.5) | restated | RFC9552-5.6-1 | RFC 9552 Section 5.6 keeps the recommendation that an implementation provides a means to inject inter-AS links into BGP-LS |
| [`RFC7752-6.1.2-1`](#rfc7752-6.1.2-1) Configuration parameters should be initialized to default values (Section 6.1.2) | unextracted | §8.1.2 | RFC 9552 Section 8.1.2 keeps the sentence and its two default values, that the Link-State NLRI capability is off for all neighbors and the advertisement rate is 200 updates per second. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-6.1.5-1`](#rfc7752-6.1.5-1) Distribution of Link-State NLRIs should be limited to a single admin domain (Section 6.1.5) | restated | RFC9552-8.1.5-1 | RFC 9552 Section 8.1.5 keeps the sentence unchanged, that distribution of Link-State NLRIs SHOULD be limited to a single administrative domain |
| [`RFC7752-6.1.6-1`](#rfc7752-6.1.6-1) Implementation should allow operator to list neighbors exchanging Link-State NLRIs (Section 6.1.6) | unextracted | §8.1.6 | RFC 9552 Section 8.1.6 keeps the sentence, that an implementation SHOULD let an operator list the neighbors with which the speaker exchanges Link-State NLRIs. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-6.2.3-1`](#rfc7752-6.2.3-1) Implementation should allow operator to specify neighbors, max rate, max RIB entries, abstracted topologies, Instance-ID, ASN/BGP-LS ID (Section 6.2.3) | unextracted | §8.2.3 | RFC 9552 Section 8.2.3 keeps every knob and adds two, a MUST that the operator can configure the 8-octet BGP-LS Instance-ID and a SHOULD for a 4096-byte UPDATE size limit. rfc/short/rfc9552.md declares no row for any of them |
| [`RFC7752-6.2.5-1`](#rfc7752-6.2.5-1) Implementation should provide statistics (total NLRIs sent/received, per-neighbor, errors, locally originated) (Section 6.2.5) | unextracted | §8.2.5 | RFC 9552 Section 8.2.5 keeps the same four statistics and the rule that they are absolute counts since system or session start. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-6.2.6-2`](#rfc7752-6.2.6-2) Operator should define an import policy to drop all updates from consumer peers (Section 6.2.6) | unextracted | §8.2.6 | RFC 9552 Section 8.2.6 raises the rule from SHOULD to MUST, that an operator defines an import policy which drops all updates from peers only serving BGP-LS Consumers. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-8-2`](#rfc7752-8-2) Operator should employ a mechanism to protect BGP speaker against DDoS attacks from consumers (Section 8) | dropped | not stated | RFC 9552 Section 10 states no obligation about protecting a speaker from denial-of-service by its consumers. RFC 7752 Section 8 stated that an operator SHOULD employ such a mechanism and named rate limits; RFC 9552 replaces that paragraph with one about erroneous and tampered link-state information |
| [`RFC7752-3.2-9`](#rfc7752-3.2-9) Implementation may make the Identifier configurable for a given protocol (Section 3.2) | unextracted | §8.2.3 | RFC 9552 raises the permission to an obligation and moves it. Its Section 8.2.3 states that an implementation MUST allow the operator to configure an 8-octet BGP-LS Instance-ID. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-3.2-10`](#rfc7752-3.2-10) OSPF and IS-IS may run multiple routing protocol instances over the same link (Section 3.2) | unextracted | §5.2 | RFC 9552 Section 5.2 keeps the sentence, that OSPF and IS-IS may run multiple routing protocol instances over the same link, and cites RFC 8202 and RFC 6549 for it. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-3.2.2-1`](#rfc7752-3.2.2-1) Link local/remote identifiers may be included in the link attribute (Section 3.2.2) | restated | RFC9552-5.2.2-4 | RFC 9552 Section 5.2.2 raises the permission to an obligation. The Link Local/Remote Identifiers TLV MUST now be included when only link-local identifiers are available, and RFC9552-5.2.2-2 forbids it when interface addresses are present |
| [`RFC7752-3.2.1.5-3`](#rfc7752-3.2.1.5-3) The MT-ID TLV may be present in Link/Prefix Descriptor or Node NLRI attribute (Section 3.2.1.5) | unextracted | §5.2.2.1 | RFC 9552 Section 5.2.2.1 keeps the same permission, that the MT-ID TLV MAY be included as a Link Descriptor, as a Prefix Descriptor, or in the BGP-LS Attribute of a Node NLRI. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-3.2.3.1-1`](#rfc7752-3.2.3.1-1) OSPF Route Type TLV is optional and may be present in Prefix NLRI (Section 3.2.3.1) | restated | RFC9552-5.2.3.1-1 | RFC 9552 Section 5.2.3.1 keeps the TLV optional and adds a MUST, that it is included when the route type is signaled in the underlying LSA or is determinable from another LSA for the same prefix |
| [`RFC7752-6.1.5-2`](#rfc7752-6.1.5-2) A network operator may use a dedicated Route-Reflector infrastructure (Section 6.1.5) | restated | RFC9552-8.1.1-2 | RFC 9552 raises the permission to a recommendation. Its Section 8.1.1 states that dedicated route reflectors SHOULD handle BGP-LS NLRI distribution, and names separate BGP instances or sessions as the alternative when none are available |
| [`RFC7752-6.2.5-2`](#rfc7752-6.2.5-2) Implementation may enhance statistics by recording peak per-second counts (Section 6.2.5) | unextracted | §8.2.5 | RFC 9552 Section 8.2.5 keeps the sentence, that an implementation MAY enhance the statistics by recording peak per-second counts. rfc/short/rfc9552.md declares no row for it |
