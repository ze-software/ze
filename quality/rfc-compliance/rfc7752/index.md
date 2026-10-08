# RFC 7752 - North-Bound Distribution of Link-State and Traffic Engineering (TE) Information Using BGP

Partial. Every requirement this repository extracted from RFC 7752, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 34.6% | 9 of 26 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 7.7% | 2 of 26 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 26 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 26 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 32.4% | 11 of 34 tagged units, 0 escaped and 8 lapsed | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 26 | of 52 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 11 | of 26 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 42.3% | 11 of 26 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 26 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 26 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 15.4% | 4 of 26 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 12 | of 26 gated MUSTs judged | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 26 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 52 |
| Gated MUST-level | 26 |
| Not applicable, so out of scope | 11 |
| Declared gaps | 5 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 34 |
| Tagged units | 34 |
| Recorded audit verdicts | 12 |
| Discrimination records | 19 |
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

Historical requirement lineage is recorded by the per-row supersession annotations; current BGP-LS implementation scope is in [`rfc/short/rfc9552.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9552.md). The native exporter now supplies `originateAttributes`, so the earlier no-native-origination claim no longer describes the code. Optional FQDN-based native Link Name selection is not implemented ([`RFC7752-3.3.2.7-1`](#rfc7752-3.3.2.7-1)); received Link Name TLVs remain decoded and propagated without imposing a naming restriction.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 9 | one part of the gated population |
| Annotated (including scoped evidence) | 17 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **26** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (9):** [`RFC7752-3.1-1`](#rfc7752-3.1-1), [`RFC7752-3.1-2`](#rfc7752-3.1-2), [`RFC7752-3.1-3`](#rfc7752-3.1-3), [`RFC7752-3.2-1`](#rfc7752-3.2-1), [`RFC7752-3.2-4`](#rfc7752-3.2-4), [`RFC7752-3.3.2.3-1`](#rfc7752-3.3.2.3-1), [`RFC7752-6.2.2-2`](#rfc7752-6.2.2-2), [`RFC7752-3.2-5`](#rfc7752-3.2-5), [`RFC7752-3.2-6`](#rfc7752-3.2-6)

**Annotated (including scoped evidence) (17):** [`RFC7752-3.2-2`](#rfc7752-3.2-2), [`RFC7752-3.2-3`](#rfc7752-3.2-3), [`RFC7752-3.2.1-1`](#rfc7752-3.2.1-1), [`RFC7752-3.2.1.1-1`](#rfc7752-3.2.1.1-1), [`RFC7752-3.2.1.1-2`](#rfc7752-3.2.1.1-2), [`RFC7752-3.2.1.4-1`](#rfc7752-3.2.1.4-1), [`RFC7752-3.2.1.4-2`](#rfc7752-3.2.1.4-2), [`RFC7752-3.2.1.4-3`](#rfc7752-3.2.1.4-3), [`RFC7752-3.2.1.5-1`](#rfc7752-3.2.1.5-1), [`RFC7752-3.3-1`](#rfc7752-3.3-1), [`RFC7752-3.3.2.1-1`](#rfc7752-3.3.2.1-1), [`RFC7752-3.3.2.2-1`](#rfc7752-3.3.2.2-1), [`RFC7752-3.3.3-1`](#rfc7752-3.3.3-1), [`RFC7752-3.4-1`](#rfc7752-3.4-1), [`RFC7752-6.2.2-1`](#rfc7752-6.2.2-1), [`RFC7752-6.2.6-1`](#rfc7752-6.2.6-1), [`RFC7752-8-1`](#rfc7752-8-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7752-3.1-1` | Unrecognized types MUST be preserved and propagated. (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC7752UnknownTLVPreservedAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L64). **positive:** `unit/verify` [`TestRFC7752UnknownTLVsReachForwardDestinations`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_unknown_forward_test.go#L11). **negative:** `unit/verify` [`TestRFC7752UnknownTLVsReachForwardDestinations`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_unknown_forward_test.go#L12) |
| `RFC7752-3.1-2` | In order to compare NLRIs with unknown TLVs, all TLVs MUST be ordered in ascending order by TLV Type. (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestLinkDescriptorOrdersMixedFamilyAddressesAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L42). **positive:** `unit/verify` [`TestRFC9552NativeLinkTLVsCanonicalOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_ordering_test.go#L95). **negative:** `unit/verify` [`TestNoDescriptorEmitsADescendingTLVSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L76). **negative:** `unit/verify` [`TestRFC9552NativeLinkTLVsReorderedSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_ordering_test.go#L117) |
| `RFC7752-3.1-3` | If there are more TLVs of the same type, then the TLVs MUST be ordered in ascending order of the TLV value within the TLVs with the same type by treating the entire Value field as an opaque hexadecimal string and comparing leftmost octets first, regardless of the length of the string. (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC9552NativeLinkTLVsCanonicalOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_ordering_test.go#L97). **negative:** `unit/verify` [`TestRFC9552NativeLinkTLVsReorderedSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_ordering_test.go#L119) |
| `RFC7752-3.2-1` | In order for two BGP speakers to exchange Link-State NLRI, they MUST use BGP Capabilities Advertisement to ensure that they are both capable of properly processing such NLRI. (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC7752BGPLSCapabilityAdvertisedAndNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7752_bgpls_test.go#L28). **positive:** `unit/verify` [`TestRFC9552LinkStateSentToCapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_capability_test.go#L55). **negative:** `unit/verify` [`TestRFC7752BGPLSCapabilityNotNegotiatedWhenPeerSilent`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7752_bgpls_test.go#L62). **negative:** `unit/verify` [`TestRFC9552LinkStateNeverSentToIncapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_capability_test.go#L78) |
| `RFC7752-3.2-2` | For all information derived from other protocols, the corresponding Protocol-ID MUST be used. (Section 3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze selects no Protocol-ID because it derives no link-state from an IGP. ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions; the Protocol-ID on every BGP-LS route ze holds arrives on the wire and is parsed at internal/component/bgp/plugins/nlri/ls/types.go:316 |
| `RFC7752-3.2-3` | The NLRIs representing link-state objects (nodes, links, or prefixes) from the same routing universe MUST have the same 'Identifier' value. (Section 3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze stamps no Identifier because it originates no Link-State NLRI. ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions; the Identifier is read from the wire at internal/component/bgp/plugins/nlri/ls/types.go:317 |
| `RFC7752-3.2-4` | NLRIs with different 'Identifier' values MUST be considered to be from different routing universes. (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC7752DifferentIdentifierIsDifferentRoutingUniverse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc7752_bgpls_test.go#L45). **negative:** `unit/verify` [`TestRFC7752SameIdentifierIsOneRoutingUniverse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc7752_bgpls_test.go#L78) |
| `RFC7752-3.2.1-1` | These auxiliary Router-IDs MUST be included in the link attribute described in Section 3.3.2. (Section 3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze assembles no BGP-LS attribute: every Ls*TLV struct is built only inside its own decode function (internal/component/bgp/plugins/nlri/ls/attr_node.go:179, internal/component/bgp/plugins/nlri/ls/attr_link.go:59) and a grep for composite-literal construction of lsIPv4RouterIDRemote, lsIPv4RouterIDLocal, lsIGPFlags and lsPrefixMetric outside _test.go finds only those decode functions, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC7752-3.2.1.1-1` | The same node MUST NOT be represented by two keys (otherwise, one node will look like two nodes). (Section 3.2.1.1) | MUST NOT | 3.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze assigns no node keys. It builds no link-state topology from BGP-LS: received routes land in the non-CIDR opaque backend keyed by raw NLRI bytes (internal/component/bgp/plugins/rib/storage/familyrib.go:278) and grep -rn "BGPLSNode" --include=*.go outside internal/component/bgp/plugins/nlri/ls/ returns nothing, so no consumer turns a Node NLRI into a keyed topology object |
| `RFC7752-3.2.1.1-2` | Two different nodes MUST NOT be represented by the same key (otherwise, two nodes will look like one node). (Section 3.2.1.1) | MUST NOT | 3.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze assigns no node keys. It builds no link-state topology from BGP-LS: received routes land in the non-CIDR opaque backend keyed by raw NLRI bytes (internal/component/bgp/plugins/rib/storage/familyrib.go:278) and grep -rn "BGPLSNode" --include=*.go outside internal/component/bgp/plugins/nlri/ls/ returns nothing, so no consumer turns a Node NLRI into a keyed topology object |
| `RFC7752-3.2.1.4-1` | The combination of ASN and BGP-LS ID MUST be globally unique. (Section 3.2.1.4) | MUST | 3.2.1.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze allocates no (ASN, BGP-LS Identifier) tuple. Both values only ever arrive on the wire and are parsed into NodeDescriptor (internal/component/bgp/plugins/nlri/ls/types.go:411), there is no config surface that sets them (grep -rn "bgp-ls" --include=*.yang returns nothing), and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC7752-3.2.1.4-2` | All BGP-LS speakers within an IGP flooding-set (set of IGP nodes within which an LSP/LSA is flooded) MUST use the same ASN, BGP-LS ID tuple. (Section 3.2.1.4) | MUST | 3.2.1.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze advertises no (ASN, BGP-LS Identifier) tuple into any IGP flooding set. It joins no IGP flooding set as a BGP-LS speaker and holds no such tuple: the pair is decoded from received NLRIs only (internal/component/bgp/plugins/nlri/ls/types.go:411) and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC7752-3.2.1.4-3` | The sub-TLVs within a Node Descriptor MUST be arranged in ascending order by sub-TLV type. (Section 3.2.1.4) | MUST | 3.2.1.4 | **positive:** `unit/verify` [`TestRFC7752NodeDescriptorSubTLVsAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L163). **negative:** no negative test. **{single-polarity}:** NodeDescriptor.WriteTo emits sub-TLVs 512, 513, 514, 515, 516 and 517 in that fixed ascending order (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:98), and the ordering duty falls on the sender: parseNodeDescriptorTLVs (internal/component/bgp/plugins/nlri/ls/types.go:391) accepts sub-TLVs in any order on receipt, so there is no out-of-order input for ze to reject |
| `RFC7752-3.2.1.5-1` | If the value in the MT-ID TLV is derived from OSPF, then the upper 9 bits MUST be set to 0. (Section 3.2.1.5) | MUST | 3.2.1.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no encoder for this TLV exists anywhere, which is what decides the classification -- the same rule stated at RFC7752-3.1-2 and applied there and at RFC7752-3.1-3: gap versus not-applicable turns on whether encoding code for the obligation EXISTS, never on whether it is reachable from production. ze emits no Multi-Topology Identifier TLV at all: LinkDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:200) and PrefixDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:264) write no TLV 263, and grep -rn "TLVMultiTopologyID" --include=*.go matches only the constant declaration at internal/component/bgp/plugins/nlri/ls/types.go:207, so there is no code that could set the upper 9 bits wrong. Reachability, recorded here only as context and NOT as the reason: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go and the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70) -- that is equally true of the RFC7752-3.1-2 and RFC7752-3.1-3 encoders, which are gaps |
| `RFC7752-3.3-1` | This attribute MUST be ignored for all other address families. (Section 3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parsePathAttributesZe decodes a type-29 attribute into the "bgp-ls" key for every UPDATE it walks, with no address-family context: the MP_REACH value is handed back separately and its AFI/SAFI is read only after the attribute loop ends, so a type-29 attribute carried on an IPv4 unicast UPDATE is decoded rather than ignored (internal/component/bgp/cli/decode_update.go:199) |
| `RFC7752-3.3.2.1-1` | All auxiliary Router-IDs of both the local and the remote node MUST be included in the link attribute of each Link NLRI. (Section 3.3.2.1) | MUST | 3.3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze assembles no BGP-LS attribute: every Ls*TLV struct is built only inside its own decode function (internal/component/bgp/plugins/nlri/ls/attr_node.go:179, internal/component/bgp/plugins/nlri/ls/attr_link.go:59) and a grep for composite-literal construction of lsIPv4RouterIDRemote, lsIPv4RouterIDLocal, lsIGPFlags and lsPrefixMetric outside _test.go finds only those decode functions, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC7752-3.3.2.2-1` | The MPLS Protocol Mask TLV MUST NOT be included in NLRIs with the other Protocol-IDs listed in Table 2. (Section 3.3.2.2) | MUST NOT | 3.3.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no MPLS Protocol Mask TLV. TLV 1094 is neither encoded nor registered for decode: grep -rn "1094" over internal/component/bgp/plugins/nlri/ls/ returns nothing and register_attr.go (internal/component/bgp/plugins/nlri/ls/register_attr.go:5) registers no 1094 decoder, so no NLRI ze emits or reads carries the TLV this clause restricts |
| `RFC7752-3.3.2.3-1` | If a source protocol uses a metric width of less than 32 bits, then the high- order bits of this field MUST be padded with zero. (Section 3.3.2.3) | MUST | 3.3.2.3 | **positive:** `unit/verify` [`TestRFC7752TEDefaultMetricWidenedWithZeroHighOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc7752_bgpls_export_te_metric_test.go#L47). **positive:** `unit/verify` [`TestRFC7752TEDefaultMetricZeroPadded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L202). **negative:** `unit/verify` [`TestRFC7752TEDefaultMetricAllOnesNeverReachesHighOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc7752_bgpls_export_te_metric_test.go#L59) |
| `RFC7752-3.3.3-1` | Prefixes are learned from the IGP topology (IS-IS or OSPF) with a set of IGP attributes (such as metric, route tags, etc.) that MUST be reflected into the BGP-LS attribute with a prefix NLRI. (Section 3.3.3) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze reflects no IGP prefix attributes into a BGP-LS attribute because it runs no BGP-LS origination. ze assembles no BGP-LS attribute: every Ls*TLV struct is built only inside its own decode function (internal/component/bgp/plugins/nlri/ls/attr_node.go:179, internal/component/bgp/plugins/nlri/ls/attr_link.go:59) and a grep for composite-literal construction of lsIPv4RouterIDRemote, lsIPv4RouterIDLocal, lsIGPFlags and lsPrefixMetric outside _test.go finds only those decode functions, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| `RFC7752-3.4-1` | The next-hop address MUST be encoded as described in [RFC4760]. (Section 3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestRFC7752BGPLSNextHopFollowsRFC4760`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7752_bgpls_test.go#L24). **negative:** no negative test. **{single-polarity}:** MPReachNLRI.WriteTo is family agnostic and lays out AFI, SAFI, next-hop length, next-hop, the zero reserved octet and then the NLRI for AFI 16388 exactly as RFC 4760 Section 3 specifies (internal/core/bgp/attribute/mpnlri.go:154); ValidNextHopLens returns nil for AFI 16388 (internal/core/bgp/attribute/mpnlri.go:305), so ze runs no BGP-LS next-hop length check and holds no rejection path to drive negatively |
| `RFC7752-6.2.2-1` | If an implementation of BGP-LS detects a malformed attribute, then it MUST use the 'Attribute Discard' action as per [RFC7606], Section 2. (Section 6.2.2) | MUST | 6.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the RFC 7606 validator table registers no entry for attribute code 29 (internal/component/bgp/message/rfc7606.go:414), so validateAttribute returns nil for a malformed BGP-LS attribute (internal/component/bgp/message/rfc7606.go:433) and the attribute rides through the UPDATE path instead of being discarded |
| `RFC7752-6.2.2-2` | An implementation of BGP-LS MUST perform the following syntactic checks for determining if a message is malformed. o Does the sum of all TLVs found in the BGP-LS attribute correspond to the BGP-LS path attribute length? o Does the sum of all TLVs found in the BGP MP_REACH_NLRI attribute correspond to the BGP MP_REACH_NLRI length? o Does the sum of all TLVs found in the BGP MP_UNREACH_NLRI attribute correspond to the BGP MP_UNREACH_NLRI length? o Does the sum of all TLVs found in a Node, Link or Prefix Descriptor NLRI attribute correspond to the Total NLRI Length field of the Node, Link, or Prefix Descriptors? o Does any fixed-length TLV correspond to the TLV Length field in this document? (Section 6.2.2) | MUST | 6.2.2 | **positive:** `unit/verify` [`TestRFC7752AttrSyntacticChecks`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L133). **positive:** `unit/verify` [`TestRFC7752AttributeLengthSumsOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_length_sums_test.go#L96). **positive:** `unit/verify` [`TestRFC7752LinkStateLengthSumsOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_length_sums_test.go#L25). **negative:** `unit/verify` [`TestRFC7752AttributeLengthSumsOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_length_sums_test.go#L97). **negative:** `unit/verify` [`TestRFC7752LinkStateLengthSumsOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_length_sums_test.go#L26). **negative:** `unit/verify` [`TestRFC7752MalformedTLVNotPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L101) |
| `RFC7752-6.2.6-1` | An implementation MUST have the means to limit inbound updates. (Section 6.2.6) | MUST | 6.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the per-family prefix maximum is mandatory for bgp-ls and enforced per family (internal/component/bgp/reactor/config.go:648, internal/component/bgp/reactor/session_prefix.go:288), but the count it compares comes from countPrefixEntries, a [prefix-length][address] CIDR walk (internal/component/bgp/reactor/session_prefix.go:497) that never parses an RFC 7752 type-length NLRI, so the number checked against the configured bgp-ls maximum bears no relation to the BGP-LS NLRI count in the UPDATE |
| `RFC7752-8-1` | In the context of the BGP peerings associated with this document, a BGP speaker MUST NOT accept updates from a consumer peer. (Section 8) | MUST NOT | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze models no BGP-LS consumer peer. The per-family peer knobs are enable, disable, require and ignore (internal/component/bgp/reactor/config.go:596), and a negotiated family is accepted in both directions (internal/core/bgp/capability/negotiated.go:413), so any peer that negotiates bgp-ls has its Link-State UPDATEs accepted whatever role it plays |
| `RFC7752-3.2-5` | All non-VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 71. (Section 3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestRFC7752NativeNLRIAnnouncedAsSAFI71`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc7752_export_family_test.go#L58). **positive:** `unit/verify` [`TestRFC7752NonVPNFamilyIsAFI16388SAFI71`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L228). **negative:** `unit/verify` [`TestRFC7752NativeNLRINeverAnnouncedAsVPN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc7752_export_family_test.go#L71). **negative:** `unit/verify` [`TestRFC7752NonLinkStateFamilyRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L264) |
| `RFC7752-3.2-6` | VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 72. (Section 3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestRFC7752VPNFamilyIsAFI16388SAFI72`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L246). **positive:** `unit/verify` [`TestRFC7752VPNFamilyReachesMPReach`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_vpn_family_test.go#L18). **negative:** `unit/verify` [`TestRFC7752NonLinkStateFamilyRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L265). **negative:** `unit/verify` [`TestRFC7752VPNFamilyReachesMPReach`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_vpn_family_test.go#L19) |
| `RFC7752-3.2-7` | The 'Direct' and 'Static configuration' protocol types SHOULD be used when BGP-LS is sourcing local information. (Section 3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.2-8` | If a given protocol does not support multiple routing universes, then it SHOULD set the Identifier field according to Table 3. (Section 3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.2.1.4-4` | If an IGP domain consists of multiple flooding-sets, then all BGP-LS speakers within the IGP domain SHOULD use the same ASN, BGP-LS ID tuple. (Section 3.2.1.4) | SHOULD | 3.2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.2.1.5-2` | Bits R are reserved and SHOULD be set to 0 when originated and ignored on receipt. (Section 3.2.1.5) | SHOULD | 3.2.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.2.3.2-1` | A router SHOULD advertise an IP Prefix NLRI for each of its BGP next hops. (Section 3.2.3.2) | SHOULD | 3.2.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.3-2` | This attribute SHOULD only be included with Link-State NLRIs. (Section 3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.3.1.3-1` | This symbolic name can be the Fully Qualified Domain Name (FQDN) for the router, it can be a subset of the FQDN (e.g., a hostname), or it can be any string operators want to use for the router. The use of FQDN or a subset of it is strongly RECOMMENDED. (Section 3.3.1.3) | SHOULD | 3.3.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.3.2.7-1` | This symbolic name can be the FQDN for the link, it can be a subset of the FQDN, or it can be any string operators want to use for the link. The use of FQDN or a subset of it is strongly RECOMMENDED. (§3.3.2.7) | SHOULD | 3.3.2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** FQDN-based native Link Name selection is not implemented; decodeLinkName preserves a received string and originateAttributes copies supplied TLV values. The optional TLV is supported as received data, not as a native FQDN naming policy. |
| `RFC7752-3.3.2.2-2` | Generation of the MPLS Protocol Mask TLV is only valid for and SHOULD only be used with originators that have local link insight, for example, the Protocol-IDs 'Static configuration' or 'Direct' as per Table 2. (Section 3.3.2.2) | SHOULD | 3.3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.3.3-2` | Prefix Attribute TLVs SHOULD be used when advertising NLRI types 3 and 4 only. (Section 3.3.3) | SHOULD | 3.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.4-2` | If an IPv4 BGP session is used, then the next hop in the MP_REACH_NLRI SHOULD be an IPv4 address. Similarly, if an IPv6 BGP session is used, then the next hop in the MP_REACH_NLRI SHOULD be an IPv6 address. (Section 3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.5-1` | In other cases, an implementation SHOULD provide a means to inject inter-AS links into BGP-LS. (Section 3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-6.1.2-1` | Configuration parameters defined in Section 6.2.3 SHOULD be initialized to the following default values: o The Link-State NLRI capability is turned off for all neighbors. o The maximum rate at which Link-State NLRIs will be advertised/ withdrawn from neighbors is set to 200 updates per second. (Section 6.1.2) | SHOULD | 6.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-6.1.5-1` | Distribution of Link-State NLRIs SHOULD be limited to a single admin domain, which can consist of multiple areas within an AS or multiple ASes. (Section 6.1.5) | SHOULD | 6.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-6.1.6-1` | In addition, an implementation SHOULD allow an operator to: o List neighbors with whom the speaker is exchanging Link-State NLRIs. (Section 6.1.6) | SHOULD | 6.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-6.2.3-1` | An implementation SHOULD allow the operator to specify neighbors to which Link-State NLRIs will be advertised and from which Link-State NLRIs will be accepted. An implementation SHOULD allow the operator to specify the maximum rate at which Link-State NLRIs will be advertised/withdrawn from neighbors. An implementation SHOULD allow the operator to specify the maximum number of Link-State NLRIs stored in a router's Routing Information Base (RIB). An implementation SHOULD allow the operator to create abstracted topologies that are advertised to neighbors and create different abstractions for different neighbors. An implementation SHOULD allow the operator to configure a 64-bit Instance-ID. An implementation SHOULD allow the operator to configure a pair of ASN and BGP-LS identifiers (Section 3.2.1.4) per flooding set in which the node participates. (Section 6.2.3) | SHOULD | 6.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-6.2.5-1` | An implementation SHOULD provide the following statistics: o Total number of Link-State NLRI updates sent/received o Number of Link-State NLRI updates sent/received, per neighbor o Number of errored received Link-State NLRI updates, per neighbor o Total number of locally originated Link-State NLRIs (Section 6.2.5) | SHOULD | 6.2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-6.2.6-2` | An operator SHOULD define an import policy to limit inbound updates as follows: o Drop all updates from consumer peers. (Section 6.2.6) | SHOULD | 6.2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-8-2` | An operator SHOULD employ a mechanism to protect a BGP speaker against DDoS attacks from consumers. (Section 8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.2-9` | However, an implementation MAY make the 'Identifier' configurable for a given protocol. (Section 3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.2-10` | Both OSPF and IS-IS MAY run multiple routing protocol instances over the same link. (Section 3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.2.2-1` | The link local/remote identifiers MAY be included in the link attribute. (Section 3.2.2) | MAY | 3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.2.1.5-3` | The MT-ID TLV MAY be present in a Link Descriptor, a Prefix Descriptor, or the BGP-LS attribute of a Node NLRI. (Section 3.2.1.5) | MAY | 3.2.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-3.2.3.1-1` | The OSPF Route Type TLV is an optional TLV that MAY be present in Prefix NLRIs. (Section 3.2.3.1) | MAY | 3.2.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-6.1.5-2` | A network operator MAY use a dedicated Route- Reflector infrastructure to distribute Link-State NLRIs. (Section 6.1.5) | MAY | 6.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7752-6.2.5-2` | An implementation MAY also enhance this information by recording peak per-second counts in each case. (Section 6.2.5) | MAY | 6.2.5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7752-3.2-2`](#rfc7752-3.2-2) For all information derived from other protocols, the corresponding Protocol-ID MUST be used. (Section 3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze selects no Protocol-ID because it derives no link-state from an IGP. ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions; the Protocol-ID on every BGP-LS route ze holds arrives on the wire and is parsed at internal/component/bgp/plugins/nlri/ls/types.go:316 |
| [`RFC7752-3.2-3`](#rfc7752-3.2-3) The NLRIs representing link-state objects (nodes, links, or prefixes) from the same routing universe MUST have the same 'Identifier' value. (Section 3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze stamps no Identifier because it originates no Link-State NLRI. ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions; the Identifier is read from the wire at internal/component/bgp/plugins/nlri/ls/types.go:317 |
| [`RFC7752-3.2.1-1`](#rfc7752-3.2.1-1) These auxiliary Router-IDs MUST be included in the link attribute described in Section 3.3.2. (Section 3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze assembles no BGP-LS attribute: every Ls*TLV struct is built only inside its own decode function (internal/component/bgp/plugins/nlri/ls/attr_node.go:179, internal/component/bgp/plugins/nlri/ls/attr_link.go:59) and a grep for composite-literal construction of lsIPv4RouterIDRemote, lsIPv4RouterIDLocal, lsIGPFlags and lsPrefixMetric outside _test.go finds only those decode functions, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC7752-3.2.1.1-1`](#rfc7752-3.2.1.1-1) The same node MUST NOT be represented by two keys (otherwise, one node will look like two nodes). (Section 3.2.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze assigns no node keys. It builds no link-state topology from BGP-LS: received routes land in the non-CIDR opaque backend keyed by raw NLRI bytes (internal/component/bgp/plugins/rib/storage/familyrib.go:278) and grep -rn "BGPLSNode" --include=*.go outside internal/component/bgp/plugins/nlri/ls/ returns nothing, so no consumer turns a Node NLRI into a keyed topology object |
| [`RFC7752-3.2.1.1-2`](#rfc7752-3.2.1.1-2) Two different nodes MUST NOT be represented by the same key (otherwise, two nodes will look like one node). (Section 3.2.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze assigns no node keys. It builds no link-state topology from BGP-LS: received routes land in the non-CIDR opaque backend keyed by raw NLRI bytes (internal/component/bgp/plugins/rib/storage/familyrib.go:278) and grep -rn "BGPLSNode" --include=*.go outside internal/component/bgp/plugins/nlri/ls/ returns nothing, so no consumer turns a Node NLRI into a keyed topology object |
| [`RFC7752-3.2.1.4-1`](#rfc7752-3.2.1.4-1) The combination of ASN and BGP-LS ID MUST be globally unique. (Section 3.2.1.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze allocates no (ASN, BGP-LS Identifier) tuple. Both values only ever arrive on the wire and are parsed into NodeDescriptor (internal/component/bgp/plugins/nlri/ls/types.go:411), there is no config surface that sets them (grep -rn "bgp-ls" --include=*.yang returns nothing), and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC7752-3.2.1.4-2`](#rfc7752-3.2.1.4-2) All BGP-LS speakers within an IGP flooding-set (set of IGP nodes within which an LSP/LSA is flooded) MUST use the same ASN, BGP-LS ID tuple. (Section 3.2.1.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze advertises no (ASN, BGP-LS Identifier) tuple into any IGP flooding set. It joins no IGP flooding set as a BGP-LS speaker and holds no such tuple: the pair is decoded from received NLRIs only (internal/component/bgp/plugins/nlri/ls/types.go:411) and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC7752-3.2.1.5-1`](#rfc7752-3.2.1.5-1) If the value in the MT-ID TLV is derived from OSPF, then the upper 9 bits MUST be set to 0. (Section 3.2.1.5) | no test | no test carries this requirement id; annotated {not-applicable}: no encoder for this TLV exists anywhere, which is what decides the classification -- the same rule stated at RFC7752-3.1-2 and applied there and at RFC7752-3.1-3: gap versus not-applicable turns on whether encoding code for the obligation EXISTS, never on whether it is reachable from production. ze emits no Multi-Topology Identifier TLV at all: LinkDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:200) and PrefixDescriptor.WriteTo (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:264) write no TLV 263, and grep -rn "TLVMultiTopologyID" --include=*.go matches only the constant declaration at internal/component/bgp/plugins/nlri/ls/types.go:207, so there is no code that could set the upper 9 bits wrong. Reachability, recorded here only as context and NOT as the reason: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go and the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70) -- that is equally true of the RFC7752-3.1-2 and RFC7752-3.1-3 encoders, which are gaps |
| [`RFC7752-3.3-1`](#rfc7752-3.3-1) This attribute MUST be ignored for all other address families. (Section 3.3) | {gap}, no test | parsePathAttributesZe decodes a type-29 attribute into the "bgp-ls" key for every UPDATE it walks, with no address-family context: the MP_REACH value is handed back separately and its AFI/SAFI is read only after the attribute loop ends, so a type-29 attribute carried on an IPv4 unicast UPDATE is decoded rather than ignored (internal/component/bgp/cli/decode_update.go:199) |
| [`RFC7752-3.3.2.1-1`](#rfc7752-3.3.2.1-1) All auxiliary Router-IDs of both the local and the remote node MUST be included in the link attribute of each Link NLRI. (Section 3.3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze assembles no BGP-LS attribute: every Ls*TLV struct is built only inside its own decode function (internal/component/bgp/plugins/nlri/ls/attr_node.go:179, internal/component/bgp/plugins/nlri/ls/attr_link.go:59) and a grep for composite-literal construction of lsIPv4RouterIDRemote, lsIPv4RouterIDLocal, lsIGPFlags and lsPrefixMetric outside _test.go finds only those decode functions, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC7752-3.3.2.2-1`](#rfc7752-3.3.2.2-1) The MPLS Protocol Mask TLV MUST NOT be included in NLRIs with the other Protocol-IDs listed in Table 2. (Section 3.3.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no MPLS Protocol Mask TLV. TLV 1094 is neither encoded nor registered for decode: grep -rn "1094" over internal/component/bgp/plugins/nlri/ls/ returns nothing and register_attr.go (internal/component/bgp/plugins/nlri/ls/register_attr.go:5) registers no 1094 decoder, so no NLRI ze emits or reads carries the TLV this clause restricts |
| [`RFC7752-3.3.3-1`](#rfc7752-3.3.3-1) Prefixes are learned from the IGP topology (IS-IS or OSPF) with a set of IGP attributes (such as metric, route tags, etc.) that MUST be reflected into the BGP-LS attribute with a prefix NLRI. (Section 3.3.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze reflects no IGP prefix attributes into a BGP-LS attribute because it runs no BGP-LS origination. ze assembles no BGP-LS attribute: every Ls*TLV struct is built only inside its own decode function (internal/component/bgp/plugins/nlri/ls/attr_node.go:179, internal/component/bgp/plugins/nlri/ls/attr_link.go:59) and a grep for composite-literal construction of lsIPv4RouterIDRemote, lsIPv4RouterIDLocal, lsIGPFlags and lsPrefixMetric outside _test.go finds only those decode functions, and ze originates no BGP-LS: NewBGPLSNode, NewBGPLSLink, NewBGPLSPrefixV4 and NewBGPLSPrefixV6 (internal/component/bgp/plugins/nlri/ls/types_nlri.go:23,:104,:196,:210) have no caller outside _test.go, the plugin registers both families as Mode "decode" only (internal/component/bgp/plugins/nlri/ls/plugin.go:70), and grep -rn "NewBGPLSNode\|NewBGPLSLink\|NewBGPLSPrefixV" --include=*.go outside _test.go returns only those four definitions |
| [`RFC7752-6.2.2-1`](#rfc7752-6.2.2-1) If an implementation of BGP-LS detects a malformed attribute, then it MUST use the 'Attribute Discard' action as per [RFC7606], Section 2. (Section 6.2.2) | {gap}, no test | the RFC 7606 validator table registers no entry for attribute code 29 (internal/component/bgp/message/rfc7606.go:414), so validateAttribute returns nil for a malformed BGP-LS attribute (internal/component/bgp/message/rfc7606.go:433) and the attribute rides through the UPDATE path instead of being discarded |
| [`RFC7752-6.2.6-1`](#rfc7752-6.2.6-1) An implementation MUST have the means to limit inbound updates. (Section 6.2.6) | {gap}, no test | the per-family prefix maximum is mandatory for bgp-ls and enforced per family (internal/component/bgp/reactor/config.go:648, internal/component/bgp/reactor/session_prefix.go:288), but the count it compares comes from countPrefixEntries, a [prefix-length][address] CIDR walk (internal/component/bgp/reactor/session_prefix.go:497) that never parses an RFC 7752 type-length NLRI, so the number checked against the configured bgp-ls maximum bears no relation to the BGP-LS NLRI count in the UPDATE |
| [`RFC7752-8-1`](#rfc7752-8-1) In the context of the BGP peerings associated with this document, a BGP speaker MUST NOT accept updates from a consumer peer. (Section 8) | {gap}, no test | ze models no BGP-LS consumer peer. The per-family peer knobs are enable, disable, require and ignore (internal/component/bgp/reactor/config.go:596), and a negotiated family is accepted in both directions (internal/core/bgp/capability/negotiated.go:413), so any peer that negotiates bgp-ls has its Link-State UPDATEs accepted whatever role it plays |
| [`RFC7752-3.3.2.7-1`](#rfc7752-3.3.2.7-1) This symbolic name can be the FQDN for the link, it can be a subset of the FQDN, or it can be any string operators want to use for the link. The use of FQDN or a subset of it is strongly RECOMMENDED. (§3.3.2.7) | {gap} | FQDN-based native Link Name selection is not implemented; decodeLinkName preserves a received string and originateAttributes copies supplied TLV values. The optional TLV is supported as received data, not as a native FQDN naming policy. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7752-3.1-1`](#rfc7752-3.1-1)

Unrecognized types MUST be preserved and propagated. (Section 3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7752 Section 3.1: "Unrecognized types MUST be preserved and propagated." Current successor RFC 9552 Section 5.1: "Unknown and unsupported types MUST be preserved and propagated within both the NLRI and the BGP-LS Attribute." All current carriers read: nlri/ls/rfc7752_test.go TestRFC7752UnknownTLVPreservedAndPropagated checks opaque attribute bytes and exact Node NLRI re-encoding; reactor/rfc7752_unknown_forward_test.go TestRFC7752UnknownTLVsReachForwardDestinations adds exact forwarded NLRI AND attribute equality toward internal/external peers with mixed recognized/unknown and all-unknown attribute variants. Removing unknowns, dropping an all-unknown attribute or changing either propagated half fails. Producer forward_validation.go forwardUpdateCore forwards through the selected path; the receive syntax boundary validates framing without TLV semantics. The obsolete malformed-TLV negative is no longer a current tag for this row. The test header paraphrases RFC 9552 under an RFC 7752 label, but the binding is substantively the existing restated successor obligation; no new exemption. Independent source rejudgment only; no test, mutation, build or gate was executed by this auditor. Native observed-red renewal is a separate parent step.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7752UnknownTLVsReachForwardDestinations`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_unknown_forward_test.go#L12) | unit/verify | revert, verified |
| positive | [`TestRFC7752UnknownTLVPreservedAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L64) | unit/verify | unproven |
| positive | [`TestRFC7752UnknownTLVsReachForwardDestinations`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_unknown_forward_test.go#L11) | unit/verify | revert, verified |

### [`RFC7752-3.1-2`](#rfc7752-3.1-2)

In order to compare NLRIs with unknown TLVs, all TLVs MUST be ordered in ascending order by TLV Type. (Section 3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c22 judge): stale only because TestNoDescriptorEmitsADescendingTLVSequence lost its SRv6SIDs fixture line with the RFC 9552 D-8 field deletion; revert record on addressTLVs re-observed red. Finding of 2026-09-30 unchanged: the native Link producer encodeNativeLink emits the exact canonical sequence from ascending and interleaved descending sources.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeLinkTLVsReorderedSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_ordering_test.go#L117) | unit/verify | revert, verified |
| negative | [`TestNoDescriptorEmitsADescendingTLVSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeLinkTLVsCanonicalOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_ordering_test.go#L95) | unit/verify | revert, verified |
| positive | [`TestLinkDescriptorOrdersMixedFamilyAddressesAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go#L42) | unit/verify | unproven |

### [`RFC7752-3.1-3`](#rfc7752-3.1-3)

If there are more TLVs of the same type, then the TLVs MUST be ordered in ascending order of the TLV value within the TLVs with the same type by treating the entire Value field as an opaque hexadecimal string and comparing leftmost octets first, regardless of the length of the string. (Section 3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rejudge 2026-09-30 (was wrong: the tagged 518 units asserted the RFC 9552 Length-first rule; tags removed under D-15). Now the ls_export units assert same-type 259/260/261/262 ascending by Value compared leftmost octet first, from ascending and descending sources. Every Type this producer repeats has one fixed Length, so the 'regardless of the length' comparison and RFC 9552's Length-then-Value order give identical output; judge break (Value comparison reversed) turns both red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552NativeLinkTLVsReorderedSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_ordering_test.go#L119) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeLinkTLVsCanonicalOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_ordering_test.go#L97) | unit/verify | revert, verified |

### [`RFC7752-3.2-1`](#rfc7752-3.2-1)

In order for two BGP speakers to exchange Link-State NLRI, they MUST use BGP Capabilities Advertisement to ensure that they are both capable of properly processing such NLRI. (Section 3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c31 judge) after the send path was tagged here. RFC 7752 Section 3.2: "In order for two BGP speakers to exchange Link-State NLRI, they MUST use BGP Capabilities Advertisement to ensure that they are both capable of properly processing such NLRI." reactor rfc9552_capability_test.go::TestRFC9552LinkStateSentToCapablePeer drives AnnounceNLRIBatch to an established peer that negotiated (16388, 71) and asserts one UPDATE whose MP_REACH_NLRI starts 40 04 47; ::TestRFC9552LinkStateNeverSentToIncapablePeer drives the same NLRI to an IPv4-unicast-only peer and asserts no UPDATE written and ErrNoPeersAcceptedFamily. The HEAD capability units keep the advertisement and the Negotiate intersection (unrecorded). Judge overlay: the per-peer gate `nc == nil || !nc.Has(batch.Family)` reduced to `nc == nil` turns the negative red and leaves the positive green; revert records exist on both new units. Row is superseded by RFC9552-5.2-7, whose verdict rests on the same two units. Receive side (an unnegotiated peer sending Link-State NLRI) is not driven, as on RFC9552-5.2-7; the sentence obliges the speaker to gate the exchange on the capability, which the send-path pair proves.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9552LinkStateNeverSentToIncapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_capability_test.go#L78) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC7752BGPLSCapabilityNotNegotiatedWhenPeerSilent`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7752_bgpls_test.go#L62) | unit/verify | unproven |
| positive | [`TestRFC9552LinkStateSentToCapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_capability_test.go#L55) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7752BGPLSCapabilityAdvertisedAndNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7752_bgpls_test.go#L28) | unit/verify | unproven |

### [`RFC7752-3.2-2`](#rfc7752-3.2-2)

For all information derived from other protocols, the corresponding Protocol-ID MUST be used. (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.2-2, so no unit is bound to it.

### [`RFC7752-3.2-3`](#rfc7752-3.2-3)

The NLRIs representing link-state objects (nodes, links, or prefixes) from the same routing universe MUST have the same 'Identifier' value. (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.2-3, so no unit is bound to it.

### [`RFC7752-3.2-4`](#rfc7752-3.2-4)

NLRIs with different 'Identifier' values MUST be considered to be from different routing universes. (Section 3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: merging NLRIs that differ only in Identifier. TestRFC7752DifferentIdentifierIsDifferentRoutingUniverse inserts two Node NLRIs identical except Identifier 0 and 1 and fails unless rib.Len() is 2, both are retrievable, and removing one leaves the other. Negative: TestRFC7752SameIdentifierIsOneRoutingUniverse fails unless a repeat with the same Identifier collapses to one route (Len 1), so the separation turns on the Identifier bytes.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7752SameIdentifierIsOneRoutingUniverse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc7752_bgpls_test.go#L78) | unit/verify | unproven |
| positive | [`TestRFC7752DifferentIdentifierIsDifferentRoutingUniverse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc7752_bgpls_test.go#L45) | unit/verify | unproven |

### [`RFC7752-3.2.1-1`](#rfc7752-3.2.1-1)

These auxiliary Router-IDs MUST be included in the link attribute described in Section 3.3.2. (Section 3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.2.1-1, so no unit is bound to it.

### [`RFC7752-3.2.1.1-1`](#rfc7752-3.2.1.1-1)

The same node MUST NOT be represented by two keys (otherwise, one node will look like two nodes). (Section 3.2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.2.1.1-1, so no unit is bound to it.

### [`RFC7752-3.2.1.1-2`](#rfc7752-3.2.1.1-2)

Two different nodes MUST NOT be represented by the same key (otherwise, two nodes will look like one node). (Section 3.2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.2.1.1-2, so no unit is bound to it.

### [`RFC7752-3.2.1.4-1`](#rfc7752-3.2.1.4-1)

The combination of ASN and BGP-LS ID MUST be globally unique. (Section 3.2.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.2.1.4-1, so no unit is bound to it.

### [`RFC7752-3.2.1.4-2`](#rfc7752-3.2.1.4-2)

All BGP-LS speakers within an IGP flooding-set (set of IGP nodes within which an LSP/LSA is flooded) MUST use the same ASN, BGP-LS ID tuple. (Section 3.2.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.2.1.4-2, so no unit is bound to it.

### [`RFC7752-3.2.1.4-3`](#rfc7752-3.2.1.4-3)

The sub-TLVs within a Node Descriptor MUST be arranged in ascending order by sub-TLV type. (Section 3.2.1.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. {single-polarity: positive} carried on the row: the ordering duty is the sender's. Forbidden: node descriptor sub-TLVs out of ascending type order. TestRFC7752NodeDescriptorSubTLVsAscending fails unless NodeDescriptor.WriteTo emits exactly 512, 513, 514, 515, 516, 517 in that order. The native exporter builds its node descriptors through ls.NodeDescriptor (ls_export exportNodeID), so the asserted encoder is the one production uses.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7752NodeDescriptorSubTLVsAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L163) | unit/verify | unproven |

### [`RFC7752-3.2.1.5-1`](#rfc7752-3.2.1.5-1)

If the value in the MT-ID TLV is derived from OSPF, then the upper 9 bits MUST be set to 0. (Section 3.2.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.2.1.5-1, so no unit is bound to it.

### [`RFC7752-3.3-1`](#rfc7752-3.3-1)

This attribute MUST be ignored for all other address families. (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.3-1, so no unit is bound to it.

### [`RFC7752-3.3.2.1-1`](#rfc7752-3.3.2.1-1)

All auxiliary Router-IDs of both the local and the remote node MUST be included in the link attribute of each Link NLRI. (Section 3.3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.3.2.1-1, so no unit is bound to it.

### [`RFC7752-3.3.2.2-1`](#rfc7752-3.3.2.2-1)

The MPLS Protocol Mask TLV MUST NOT be included in NLRIs with the other Protocol-IDs listed in Table 2. (Section 3.3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.3.2.2-1, so no unit is bound to it.

### [`RFC7752-3.3.2.3-1`](#rfc7752-3.3.2.3-1)

If a source protocol uses a metric width of less than 32 bits, then the high- order bits of this field MUST be padded with zero. (Section 3.3.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rejudge 2026-09-30: new isis units drive the real widening in bgpls_export.go linkAttribute (IS-IS sub-TLV 18, 3 octets, into TLV 1092): 0a0b0c -> 000a0b0c (+) and ffffff -> 00ffffff (-). Judge break (copy into v[0:3]) turns both red; revert records observed red. Marker removed; nlri/ls TestRFC7752TEDefaultMetricZeroPadded keeps its positive tag.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7752TEDefaultMetricAllOnesNeverReachesHighOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc7752_bgpls_export_te_metric_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestRFC7752TEDefaultMetricZeroPadded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L202) | unit/verify | unproven |
| positive | [`TestRFC7752TEDefaultMetricWidenedWithZeroHighOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc7752_bgpls_export_te_metric_test.go#L47) | unit/verify | revert, verified |

### [`RFC7752-3.3.3-1`](#rfc7752-3.3.3-1)

Prefixes are learned from the IGP topology (IS-IS or OSPF) with a set of IGP attributes (such as metric, route tags, etc.) that MUST be reflected into the BGP-LS attribute with a prefix NLRI. (Section 3.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-3.3.3-1, so no unit is bound to it.

### [`RFC7752-3.4-1`](#rfc7752-3.4-1)

The next-hop address MUST be encoded as described in [RFC4760]. (Section 3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. {single-polarity: positive} carried on the row. Forbidden: a BGP-LS MP_REACH_NLRI whose next hop departs from the RFC 4760 layout. TestRFC7752BGPLSNextHopFollowsRFC4760 fails unless, for AFI 16388 SAFI 71, the encoder emits AFI, SAFI, next-hop length 4 or 16, the address, a zero reserved octet and then the NLRI, and the parse round-trips; IPv4 and IPv6 next hops both asserted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7752BGPLSNextHopFollowsRFC4760`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7752_bgpls_test.go#L24) | unit/verify | unproven |

### [`RFC7752-6.2.2-1`](#rfc7752-6.2.2-1)

If an implementation of BGP-LS detects a malformed attribute, then it MUST use the 'Attribute Discard' action as per [RFC7606], Section 2. (Section 6.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-6.2.2-1, so no unit is bound to it.

### [`RFC7752-6.2.2-2`](#rfc7752-6.2.2-2)

An implementation of BGP-LS MUST perform the following syntactic checks for determining if a message is malformed. o Does the sum of all TLVs found in the BGP-LS attribute correspond to the BGP-LS path attribute length? o Does the sum of all TLVs found in the BGP MP_REACH_NLRI attribute correspond to the BGP MP_REACH_NLRI length? o Does the sum of all TLVs found in the BGP MP_UNREACH_NLRI attribute correspond to the BGP MP_UNREACH_NLRI length? o Does the sum of all TLVs found in a Node, Link or Prefix Descriptor NLRI attribute correspond to the Total NLRI Length field of the Node, Link, or Prefix Descriptors? o Does any fixed-length TLV correspond to the TLV Length field in this document? (Section 6.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent semantic rejudgment: RFC7752 Section 6.2.2: "An implementation of BGP-LS MUST perform the following syntactic checks for determining if a message is malformed." The full following list is retained: sums of TLV lengths for BGP-LS attribute, MP_REACH and MP_UNREACH, Node/Link/Prefix Descriptor Total NLRI Length sums, and fixed-TLV valid length. Current codec malformed/valid pairs and reactor TestRFC7752 length-sum units cover attribute and MP carriers with exact preserved bytes or RFC9552-compatible discard/reset behavior. The code29 malformed-length acceptance fault now fails malformed-attribute absence/dispatch handling; the discard-all code29 fault independently fails intact legal-attribute delivery. Both preserve their specified opposite-side controls. These are actual caller effect counterfactuals and do not rely on fabricated fixture differences or claim semantic TLV-content validation prohibited by successor RFC9552. These are inspected scratch semantic executions, not canonical discrimination records. Current native record renewal/stamping remains parent-owned; no pending FRR carrier run, SRPM behavior, dataplane behavior, or full-RFC acceptance is inferred.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7752MalformedTLVNotPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L101) | unit/verify | unproven |
| negative | [`TestRFC7752AttributeLengthSumsOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_length_sums_test.go#L97) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC7752LinkStateLengthSumsOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_length_sums_test.go#L26) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7752AttrSyntacticChecks`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L133) | unit/verify | unproven |
| positive | [`TestRFC7752AttributeLengthSumsOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_length_sums_test.go#L96) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7752LinkStateLengthSumsOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_length_sums_test.go#L25) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC7752-6.2.6-1`](#rfc7752-6.2.6-1)

An implementation MUST have the means to limit inbound updates. (Section 6.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-6.2.6-1, so no unit is bound to it.

### [`RFC7752-8-1`](#rfc7752-8-1)

In the context of the BGP peerings associated with this document, a BGP speaker MUST NOT accept updates from a consumer peer. (Section 8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7752-8-1, so no unit is bound to it.

### [`RFC7752-3.2-5`](#rfc7752-3.2-5)

All non-VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 71. (Section 3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rejudge 2026-09-30: positive TestRFC7752NativeNLRIAnnouncedAsSAFI71 drives the native producer and asserts Node, Link and Prefix NLRI each announced under bgp-ls/bgp-ls, with BGPLSFamily pinned to (16388, 71); family-name to wire AFI/SAFI is the reactor registry's encode. Genuine negative is the receive-side refusal TestRFC7752NonLinkStateFamilyRefused (R1(a): non-(16388,71) carrier refused). The new TestRFC7752NativeNLRINeverAnnouncedAsVPN repeats the positive's assertion on the same input and adds no polarity.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7752NativeNLRINeverAnnouncedAsVPN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc7752_export_family_test.go#L71) | unit/verify | revert, verified |
| negative | [`TestRFC7752NonLinkStateFamilyRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L264) | unit/verify | unproven |
| positive | [`TestRFC7752NativeNLRIAnnouncedAsSAFI71`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc7752_export_family_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestRFC7752NonVPNFamilyIsAFI16388SAFI71`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L228) | unit/verify | unproven |

### [`RFC7752-3.2-6`](#rfc7752-3.2-6)

VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 72. (Section 3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7752 Section 3.2 and RFC 9552 Section 5.2: "VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 72." TestRFC7752VPNFamilyIsAFI16388SAFI72 pins the registered family and TestRFC7752NonLinkStateFamilyRefused confines decoder entrypoints. TestRFC7752VPNFamilyReachesMPReach now creates a real VPN Node NLRI with RD, sends through AnnounceNLRIBatch, and checks exact MP_REACH AFI/SAFI and unchanged NLRI bytes; its non-VPN SAFI 71 arm prevents conflating the families. The producer buildBatchAnnounceUpdate takes AFI/SAFI from batch.Family independently of Node/Link/Prefix contents, so this is wire evidence of the shared family boundary rather than only a constant assertion. Preserve the existing restated RFC9552-5.2-2 marker and historical VPN propagation scope. Independent source rejudgment only; no test, mutation, build or gate was executed by this auditor. Native observed-red renewal is a separate parent step.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7752NonLinkStateFamilyRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L265) | unit/verify | unproven |
| negative | [`TestRFC7752VPNFamilyReachesMPReach`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_vpn_family_test.go#L19) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7752VPNFamilyIsAFI16388SAFI72`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L246) | unit/verify | unproven |
| positive | [`TestRFC7752VPNFamilyReachesMPReach`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_vpn_family_test.go#L18) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC7752-3.3.2.7-1`](#rfc7752-3.3.2.7-1)

This symbolic name can be the FQDN for the link, it can be a subset of the FQDN, or it can be any string operators want to use for the link. The use of FQDN or a subset of it is strongly RECOMMENDED. (§3.3.2.7)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. §3.3.2.7 recommends an FQDN-derived Link Name but permits arbitrary operator strings and makes the TLV optional. The native naming policy is absent: originateAttributes copies supplied values, and decodeLinkName decodes them. TestLinkNameRoundTrip exercises preservation of ge-0/0/1, not native name selection. This is an optional implementation gap, not an enforced recommendation or a receive-side rejection duty.

No test carries RFC7752-3.3.2.7-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc7752.txt |
| Source fingerprint | 6245dec342b27f9d |
| Record | rfc/extraction/rfc7752.json |
| Mapped sentences | 26 |
| Declined as scope | 0 |
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
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 3 | walked | not stated |
| `3.2` | not stated | 6 | walked | not stated |
| `3.2.1` | not stated | 1 | walked | not stated |
| `3.2.1.1` | not stated | 2 | walked | not stated |
| `3.2.1.2` | not stated | 0 | walked | not stated |
| `3.2.1.3` | not stated | 0 | walked | not stated |
| `3.2.1.4` | not stated | 3 | walked | not stated |
| `3.2.1.5` | not stated | 1 | walked | not stated |
| `3.2.2` | not stated | 0 | walked | not stated |
| `3.2.3` | not stated | 0 | walked | not stated |
| `3.2.3.1` | not stated | 0 | walked | not stated |
| `3.2.3.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 1 | walked | not stated |
| `3.3.1` | not stated | 0 | walked | not stated |
| `3.3.1.1` | not stated | 0 | walked | not stated |
| `3.3.1.2` | not stated | 0 | walked | not stated |
| `3.3.1.3` | not stated | 0 | walked | not stated |
| `3.3.1.4` | not stated | 0 | walked | not stated |
| `3.3.1.5` | not stated | 0 | walked | not stated |
| `3.3.2` | not stated | 0 | walked | not stated |
| `3.3.2.1` | not stated | 1 | walked | not stated |
| `3.3.2.2` | not stated | 1 | walked | not stated |
| `3.3.2.3` | not stated | 1 | walked | not stated |
| `3.3.2.4` | not stated | 0 | walked | not stated |
| `3.3.2.5` | not stated | 0 | walked | not stated |
| `3.3.2.6` | not stated | 0 | walked | not stated |
| `3.3.2.7` | not stated | 0 | walked | not stated |
| `3.3.3` | not stated | 1 | walked | not stated |
| `3.3.3.1` | not stated | 0 | walked | not stated |
| `3.3.3.2` | not stated | 0 | walked | not stated |
| `3.3.3.3` | not stated | 0 | walked | not stated |
| `3.3.3.4` | not stated | 0 | walked | not stated |
| `3.3.3.5` | not stated | 0 | walked | not stated |
| `3.3.3.6` | not stated | 0 | walked | not stated |
| `3.4` | not stated | 1 | walked | not stated |
| `3.5` | not stated | 0 | walked | not stated |
| `3.6` | not stated | 0 | walked | not stated |
| `3.7` | not stated | 0 | walked | not stated |
| `3.8` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.3` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.1.1` | not stated | 0 | walked | not stated |
| `6.1.2` | not stated | 0 | walked | not stated |
| `6.1.3` | not stated | 0 | walked | not stated |
| `6.1.4` | not stated | 0 | walked | not stated |
| `6.1.5` | not stated | 0 | walked | not stated |
| `6.1.6` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `6.2.1` | not stated | 0 | walked | not stated |
| `6.2.2` | not stated | 2 | walked | not stated |
| `6.2.3` | not stated | 0 | walked | not stated |
| `6.2.4` | not stated | 0 | walked | not stated |
| `6.2.5` | not stated | 0 | walked | not stated |
| `6.2.6` | not stated | 1 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 1 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 7752 declined no sentence: every site it found is mapped to a requirement.

## Superseded

RFC 7752 is obsoleted by RFC 9552.

| Requirement | Disposition | Now stated at | Reason |
|---|---|---|---|
| [`RFC7752-3.1-1`](#rfc7752-3.1-1) Unrecognized types MUST be preserved and propagated. (Section 3.1) | restated | RFC9552-5.1-3 | RFC 9552 Section 5.1 widens the same obligation, from unrecognized TLVs to unknown and unsupported ones, and states it over both the NLRI and the BGP-LS Attribute |
| [`RFC7752-3.1-2`](#rfc7752-3.1-2) In order to compare NLRIs with unknown TLVs, all TLVs MUST be ordered in ascending order by TLV Type. (Section 3.1) | restated | RFC9552-5.1-1 | RFC 9552 Section 5.1 keeps the ascending-Type order as a MUST for TLVs within the NLRI, and states the same order over the BGP-LS Attribute as a SHOULD at RFC9552-5.1-8 |
| [`RFC7752-3.1-3`](#rfc7752-3.1-3) If there are more TLVs of the same type, then the TLVs MUST be ordered in ascending order of the TLV value within the TLVs with the same type by treating the entire Value field as an opaque hexadecimal string and comparing leftmost octets first, regardless of the length of the string. (Section 3.1) | restated | RFC9552-5.1-2 | RFC 9552 Section 5.1 changes the tie-break rule for same-type TLVs, from leftmost-octet value comparison to ascending Length and then ascending Value |
| [`RFC7752-3.2-1`](#rfc7752-3.2-1) In order for two BGP speakers to exchange Link-State NLRI, they MUST use BGP Capabilities Advertisement to ensure that they are both capable of properly processing such NLRI. (Section 3.2) | restated | RFC9552-5.2-7 | RFC 9552 Section 5.2 keeps the obligation to use BGP Capabilities Advertisement before exchanging Link-State NLRIs |
| [`RFC7752-3.2-2`](#rfc7752-3.2-2) For all information derived from other protocols, the corresponding Protocol-ID MUST be used. (Section 3.2) | restated | RFC9552-5.2-3 | RFC 9552 Section 5.2 keeps the sentence unchanged, that information derived from another protocol MUST carry that protocol's Protocol-ID |
| [`RFC7752-3.2-3`](#rfc7752-3.2-3) The NLRIs representing link-state objects (nodes, links, or prefixes) from the same routing universe MUST have the same 'Identifier' value. (Section 3.2) | restated | RFC9552-5.2-4 | RFC 9552 Section 5.2 renames the Identifier field's content to the BGP-LS Instance-ID and puts the obligation on the operator, who MUST assign the same BGP-LS Instance-ID on all BGP-LS Producers within one IGP domain |
| [`RFC7752-3.2-4`](#rfc7752-3.2-4) NLRIs with different 'Identifier' values MUST be considered to be from different routing universes. (Section 3.2) | restated | RFC9552-5.2-5 | RFC 9552 Section 5.2 restates the same rule from the other side, that unique BGP-LS Instance-IDs MUST be assigned to routing protocol instances in different IGP domains |
| [`RFC7752-3.2.1-1`](#rfc7752-3.2.1-1) These auxiliary Router-IDs MUST be included in the link attribute described in Section 3.3.2. (Section 3.2.1) | restated | RFC9552-5.2.1-1 | RFC 9552 Section 5.2.1 moves the obligation from the link attribute to the node attribute. Auxiliary TE Router-IDs (TLVs 1028 and 1029) MUST now be included in the node attribute, and RFC9552-5.2.1-2 lowers the link attribute to a MAY |
| [`RFC7752-3.2.1.1-1`](#rfc7752-3.2.1.1-1) The same node MUST NOT be represented by two keys (otherwise, one node will look like two nodes). (Section 3.2.1.1) | unextracted | §5.2.1.1 | RFC 9552 Section 5.2.1.1 states requirement (A) in the same words, that the same node MUST NOT be represented by two keys. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-3.2.1.1-2`](#rfc7752-3.2.1.1-2) Two different nodes MUST NOT be represented by the same key (otherwise, two nodes will look like one node). (Section 3.2.1.1) | unextracted | §5.2.1.1 | RFC 9552 Section 5.2.1.1 states requirement (B) in the same words, that two different nodes MUST NOT be represented by the same key. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-3.2.1.4-1`](#rfc7752-3.2.1.4-1) The combination of ASN and BGP-LS ID MUST be globally unique. (Section 3.2.1.4) | dropped | not stated | RFC 9552 deprecates the BGP-LS Identifier sub-TLV (513). Its Section 5.2.1.4 table marks the code point deprecated and its Appendix A states the (ASN, BGP-LS ID) uniqueness rule in the past tense, because the BGP-LS Instance-ID carried in the Identifier field now provides that function. The obligation is gone, and RFC9552-5.2.1.4-3 keeps only a SHOULD to advertise the sub-TLV for compatibility with RFC 7752 implementations |
| [`RFC7752-3.2.1.4-2`](#rfc7752-3.2.1.4-2) All BGP-LS speakers within an IGP flooding-set (set of IGP nodes within which an LSP/LSA is flooded) MUST use the same ASN, BGP-LS ID tuple. (Section 3.2.1.4) | dropped | not stated | same deprecation as RFC7752-3.2.1.4-1. RFC 9552 Appendix A records that BGP-LS Speakers within an IGP flooding-set had to use the same (ASN, BGP-LS ID) tuple, in the past tense, and states no such obligation of its own |
| [`RFC7752-3.2.1.4-3`](#rfc7752-3.2.1.4-3) The sub-TLVs within a Node Descriptor MUST be arranged in ascending order by sub-TLV type. (Section 3.2.1.4) | restated | RFC9552-5.2.1.4-2 | RFC 9552 Section 5.2.1.4 keeps the ascending sub-TLV-type ordering rule for Node Descriptors |
| [`RFC7752-3.2.1.5-1`](#rfc7752-3.2.1.5-1) If the value in the MT-ID TLV is derived from OSPF, then the upper 9 bits MUST be set to 0. (Section 3.2.1.5) | restated | RFC9552-5.2.2-6 | RFC 9552 moves the MT-ID TLV out of the Node Descriptor section into Section 5.2.2 and fixes the OSPF encoding, keeping the rule that an OSPF-derived value carries 0 in its upper bits |
| [`RFC7752-3.3-1`](#rfc7752-3.3-1) This attribute MUST be ignored for all other address families. (Section 3.3) | dropped | not stated | RFC 9552 Section 5.3 replaces the sentence with a scoping statement, that the use of this attribute for other address families is outside the scope of this document. No MUST remains about ignoring the attribute on another address family |
| [`RFC7752-3.3.2.1-1`](#rfc7752-3.3.2.1-1) All auxiliary Router-IDs of both the local and the remote node MUST be included in the link attribute of each Link NLRI. (Section 3.3.2.1) | restated | RFC9552-5.3.2.1-1 | RFC 9552 Section 5.3.2.1 keeps the sentence unchanged, that all auxiliary Router-IDs of the local and the remote node MUST be included in the link attribute of each Link NLRI |
| [`RFC7752-3.3.2.2-1`](#rfc7752-3.3.2.2-1) The MPLS Protocol Mask TLV MUST NOT be included in NLRIs with the other Protocol-IDs listed in Table 2. (Section 3.3.2.2) | restated | RFC9552-5.3.2.2-1 | RFC 9552 Section 5.3.2.2 keeps the MPLS Protocol Mask TLV out of NLRIs with Protocol-IDs 1 to 3 and 6 |
| [`RFC7752-3.3.2.3-1`](#rfc7752-3.3.2.3-1) If a source protocol uses a metric width of less than 32 bits, then the high- order bits of this field MUST be padded with zero. (Section 3.3.2.3) | restated | RFC9552-5.3.2.3-1 | RFC 9552 Section 5.3.2.3 keeps the zero-padding rule for a TE Default Metric sourced from a protocol narrower than 32 bits |
| [`RFC7752-3.3.3-1`](#rfc7752-3.3.3-1) Prefixes are learned from the IGP topology (IS-IS or OSPF) with a set of IGP attributes (such as metric, route tags, etc.) that MUST be reflected into the BGP-LS attribute with a prefix NLRI. (Section 3.3.3) | dropped | not stated | RFC 9552 Section 5.3.3 states the same sentence without the keyword. RFC 7752 wrote that the IGP attributes MUST be reflected into the BGP-LS attribute; RFC 9552 writes that they are advertised in the BGP-LS Attribute with Prefix NLRI types 3 and 4. The obligation became a description |
| [`RFC7752-3.4-1`](#rfc7752-3.4-1) The next-hop address MUST be encoded as described in [RFC4760]. (Section 3.4) | restated | RFC9552-5.5-1 | RFC 9552 Section 5.5 keeps the sentence unchanged, that the next-hop address MUST be encoded as RFC 4760 describes |
| [`RFC7752-6.2.2-1`](#rfc7752-6.2.2-1) If an implementation of BGP-LS detects a malformed attribute, then it MUST use the 'Attribute Discard' action as per [RFC7606], Section 2. (Section 6.2.2) | restated | RFC9552-8.2.2-6 | RFC 9552 Section 8.2.2 keeps Attribute Discard for a malformed BGP-LS Attribute the receiver can skip, and adds a session-reset path for the errors it cannot skip |
| [`RFC7752-6.2.2-2`](#rfc7752-6.2.2-2) An implementation of BGP-LS MUST perform the following syntactic checks for determining if a message is malformed. o Does the sum of all TLVs found in the BGP-LS attribute correspond to the BGP-LS path attribute length? o Does the sum of all TLVs found in the BGP MP_REACH_NLRI attribute correspond to the BGP MP_REACH_NLRI length? o Does the sum of all TLVs found in the BGP MP_UNREACH_NLRI attribute correspond to the BGP MP_UNREACH_NLRI length? o Does the sum of all TLVs found in a Node, Link or Prefix Descriptor NLRI attribute correspond to the Total NLRI Length field of the Node, Link, or Prefix Descriptors? o Does any fixed-length TLV correspond to the TLV Length field in this document? (Section 6.2.2) | unextracted | §8.2.2 | RFC 9552 Section 8.2.2 states the obligation twice and in more detail, as two lists of syntactic validation a BGP-LS Speaker MUST perform, one over the Link-State NLRI and one over the BGP-LS Attribute. rfc/short/rfc9552.md declares no row for either list |
| [`RFC7752-6.2.6-1`](#rfc7752-6.2.6-1) An implementation MUST have the means to limit inbound updates. (Section 6.2.6) | restated | RFC9552-8.2.6-1 | RFC 9552 Section 8.2.6 keeps the sentence unchanged, that an implementation MUST have the means to limit inbound updates |
| [`RFC7752-8-1`](#rfc7752-8-1) In the context of the BGP peerings associated with this document, a BGP speaker MUST NOT accept updates from a consumer peer. (Section 8) | unextracted | §8.2.6 | RFC 9552 moves the obligation off the speaker and onto the operator's import policy. Its Section 8.2.6 states that an operator MUST define an import policy that drops all updates from peers which only serve BGP-LS Consumers, and its Section 10 states the same intent without a keyword. rfc/short/rfc9552.md declares no row for the Section 8.2.6 import-policy MUST |
| [`RFC7752-3.2-5`](#rfc7752-3.2-5) All non-VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 71. (Section 3.2) | restated | RFC9552-5.2-1 | RFC 9552 Section 5.2 keeps AFI 16388 / SAFI 71 for all non-VPN link, node and prefix information |
| [`RFC7752-3.2-6`](#rfc7752-3.2-6) VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 72. (Section 3.2) | restated | RFC9552-5.2-2 | RFC 9552 Section 5.2 keeps AFI 16388 / SAFI 72 for VPN link, node and prefix information |
| [`RFC7752-3.2-7`](#rfc7752-3.2-7) The 'Direct' and 'Static configuration' protocol types SHOULD be used when BGP-LS is sourcing local information. (Section 3.2) | restated | RFC9552-5.2-9 | RFC 9552 Section 5.2 keeps the Direct and Static configuration protocol types for locally sourced information |
| [`RFC7752-3.2-8`](#rfc7752-3.2-8) If a given protocol does not support multiple routing universes, then it SHOULD set the Identifier field according to Table 3. (Section 3.2) | restated | RFC9552-5.2-10 | RFC 9552 Section 5.2 restates the default as a RECOMMENDED rather than a SHOULD, and rewrites the condition from a protocol without multiple routing universes to a network with a single protocol instance |
| [`RFC7752-3.2.1.4-4`](#rfc7752-3.2.1.4-4) If an IGP domain consists of multiple flooding-sets, then all BGP-LS speakers within the IGP domain SHOULD use the same ASN, BGP-LS ID tuple. (Section 3.2.1.4) | dropped | not stated | same deprecation as RFC7752-3.2.1.4-1. With the BGP-LS Identifier sub-TLV deprecated, RFC 9552 states no obligation about a shared (ASN, BGP-LS ID) tuple across an IGP domain |
| [`RFC7752-3.2.1.5-2`](#rfc7752-3.2.1.5-2) Bits R are reserved and SHOULD be set to 0 when originated and ignored on receipt. (Section 3.2.1.5) | unextracted | §5.2.2.1 | RFC 9552 Section 5.2.2.1 raises the rule from SHOULD to MUST, that the reserved R bits are set to 0 when originated and ignored on receipt, and scopes it to a Link or Prefix Descriptor for IS-IS. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-3.2.3.2-1`](#rfc7752-3.2.3.2-1) A router SHOULD advertise an IP Prefix NLRI for each of its BGP next hops. (Section 3.2.3.2) | restated | RFC9552-5.2.3.2-1 | RFC 9552 Section 5.2.3.2 keeps the recommendation that a router advertises an IP Prefix NLRI for each of its BGP next hops |
| [`RFC7752-3.3-2`](#rfc7752-3.3-2) This attribute SHOULD only be included with Link-State NLRIs. (Section 3.3) | restated | RFC9552-5.3-3 | RFC 9552 Section 5.3 keeps the sentence unchanged, that the BGP-LS Attribute SHOULD only be included with Link-State NLRIs |
| [`RFC7752-3.3.1.3-1`](#rfc7752-3.3.1.3-1) This symbolic name can be the Fully Qualified Domain Name (FQDN) for the router, it can be a subset of the FQDN (e.g., a hostname), or it can be any string operators want to use for the router. The use of FQDN or a subset of it is strongly RECOMMENDED. (Section 3.3.1.3) | restated | RFC9552-5.3.1.3-1 | RFC 9552 Section 5.3.1.3 keeps the FQDN recommendation for the Node Name TLV |
| [`RFC7752-3.3.2.7-1`](#rfc7752-3.3.2.7-1) This symbolic name can be the FQDN for the link, it can be a subset of the FQDN, or it can be any string operators want to use for the link. The use of FQDN or a subset of it is strongly RECOMMENDED. (§3.3.2.7) | restated | RFC9552-5.3.2.7-1 | RFC 9552 Section 5.3.2.7 retains the recommendation for the Link Name TLV |
| [`RFC7752-3.3.2.2-2`](#rfc7752-3.3.2.2-2) Generation of the MPLS Protocol Mask TLV is only valid for and SHOULD only be used with originators that have local link insight, for example, the Protocol-IDs 'Static configuration' or 'Direct' as per Table 2. (Section 3.3.2.2) | restated | RFC9552-5.3.2.2-2 | RFC 9552 Section 5.3.2.2 keeps the MPLS Protocol Mask TLV scoped to Protocol-IDs 4 and 5 |
| [`RFC7752-3.3.3-2`](#rfc7752-3.3.3-2) Prefix Attribute TLVs SHOULD be used when advertising NLRI types 3 and 4 only. (Section 3.3.3) | dropped | not stated | RFC 9552 Section 5.3.3 folds the sentence into its opening description and states no SHOULD. RFC 7752 wrote that Prefix Attribute TLVs SHOULD be used when advertising NLRI types 3 and 4 only; RFC 9552 writes that the IGP attributes are advertised with Prefix NLRI types 3 and 4 |
| [`RFC7752-3.4-2`](#rfc7752-3.4-2) If an IPv4 BGP session is used, then the next hop in the MP_REACH_NLRI SHOULD be an IPv4 address. Similarly, if an IPv6 BGP session is used, then the next hop in the MP_REACH_NLRI SHOULD be an IPv6 address. (Section 3.4) | restated | RFC9552-5.5-2 | RFC 9552 Section 5.5 keeps the recommendation that the MP_REACH_NLRI next hop matches the address family of the BGP session |
| [`RFC7752-3.5-1`](#rfc7752-3.5-1) In other cases, an implementation SHOULD provide a means to inject inter-AS links into BGP-LS. (Section 3.5) | restated | RFC9552-5.6-1 | RFC 9552 Section 5.6 keeps the recommendation that an implementation provides a means to inject inter-AS links into BGP-LS |
| [`RFC7752-6.1.2-1`](#rfc7752-6.1.2-1) Configuration parameters defined in Section 6.2.3 SHOULD be initialized to the following default values: o The Link-State NLRI capability is turned off for all neighbors. o The maximum rate at which Link-State NLRIs will be advertised/ withdrawn from neighbors is set to 200 updates per second. (Section 6.1.2) | unextracted | §8.1.2 | RFC 9552 Section 8.1.2 keeps the sentence and its two default values, that the Link-State NLRI capability is off for all neighbors and the advertisement rate is 200 updates per second. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-6.1.5-1`](#rfc7752-6.1.5-1) Distribution of Link-State NLRIs SHOULD be limited to a single admin domain, which can consist of multiple areas within an AS or multiple ASes. (Section 6.1.5) | restated | RFC9552-8.1.5-1 | RFC 9552 Section 8.1.5 keeps the sentence unchanged, that distribution of Link-State NLRIs SHOULD be limited to a single administrative domain |
| [`RFC7752-6.1.6-1`](#rfc7752-6.1.6-1) In addition, an implementation SHOULD allow an operator to: o List neighbors with whom the speaker is exchanging Link-State NLRIs. (Section 6.1.6) | unextracted | §8.1.6 | RFC 9552 Section 8.1.6 keeps the sentence, that an implementation SHOULD let an operator list the neighbors with which the speaker exchanges Link-State NLRIs. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-6.2.3-1`](#rfc7752-6.2.3-1) An implementation SHOULD allow the operator to specify neighbors to which Link-State NLRIs will be advertised and from which Link-State NLRIs will be accepted. An implementation SHOULD allow the operator to specify the maximum rate at which Link-State NLRIs will be advertised/withdrawn from neighbors. An implementation SHOULD allow the operator to specify the maximum number of Link-State NLRIs stored in a router's Routing Information Base (RIB). An implementation SHOULD allow the operator to create abstracted topologies that are advertised to neighbors and create different abstractions for different neighbors. An implementation SHOULD allow the operator to configure a 64-bit Instance-ID. An implementation SHOULD allow the operator to configure a pair of ASN and BGP-LS identifiers (Section 3.2.1.4) per flooding set in which the node participates. (Section 6.2.3) | unextracted | §8.2.3 | RFC 9552 Section 8.2.3 keeps every knob and adds two, a MUST that the operator can configure the 8-octet BGP-LS Instance-ID and a SHOULD for a 4096-byte UPDATE size limit. rfc/short/rfc9552.md declares no row for any of them |
| [`RFC7752-6.2.5-1`](#rfc7752-6.2.5-1) An implementation SHOULD provide the following statistics: o Total number of Link-State NLRI updates sent/received o Number of Link-State NLRI updates sent/received, per neighbor o Number of errored received Link-State NLRI updates, per neighbor o Total number of locally originated Link-State NLRIs (Section 6.2.5) | unextracted | §8.2.5 | RFC 9552 Section 8.2.5 keeps the same four statistics and the rule that they are absolute counts since system or session start. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-6.2.6-2`](#rfc7752-6.2.6-2) An operator SHOULD define an import policy to limit inbound updates as follows: o Drop all updates from consumer peers. (Section 6.2.6) | unextracted | §8.2.6 | RFC 9552 Section 8.2.6 raises the rule from SHOULD to MUST, that an operator defines an import policy which drops all updates from peers only serving BGP-LS Consumers. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-8-2`](#rfc7752-8-2) An operator SHOULD employ a mechanism to protect a BGP speaker against DDoS attacks from consumers. (Section 8) | dropped | not stated | RFC 9552 Section 10 states no obligation about protecting a speaker from denial-of-service by its consumers. RFC 7752 Section 8 stated that an operator SHOULD employ such a mechanism and named rate limits; RFC 9552 replaces that paragraph with one about erroneous and tampered link-state information |
| [`RFC7752-3.2-9`](#rfc7752-3.2-9) However, an implementation MAY make the 'Identifier' configurable for a given protocol. (Section 3.2) | unextracted | §8.2.3 | RFC 9552 raises the permission to an obligation and moves it. Its Section 8.2.3 states that an implementation MUST allow the operator to configure an 8-octet BGP-LS Instance-ID. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-3.2-10`](#rfc7752-3.2-10) Both OSPF and IS-IS MAY run multiple routing protocol instances over the same link. (Section 3.2) | unextracted | §5.2 | RFC 9552 Section 5.2 keeps the sentence, that OSPF and IS-IS may run multiple routing protocol instances over the same link, and cites RFC 8202 and RFC 6549 for it. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-3.2.2-1`](#rfc7752-3.2.2-1) The link local/remote identifiers MAY be included in the link attribute. (Section 3.2.2) | restated | RFC9552-5.2.2-4 | RFC 9552 Section 5.2.2 raises the permission to an obligation. The Link Local/Remote Identifiers TLV MUST now be included when only link-local identifiers are available, and RFC9552-5.2.2-2 forbids it when interface addresses are present |
| [`RFC7752-3.2.1.5-3`](#rfc7752-3.2.1.5-3) The MT-ID TLV MAY be present in a Link Descriptor, a Prefix Descriptor, or the BGP-LS attribute of a Node NLRI. (Section 3.2.1.5) | unextracted | §5.2.2.1 | RFC 9552 Section 5.2.2.1 keeps the same permission, that the MT-ID TLV MAY be included as a Link Descriptor, as a Prefix Descriptor, or in the BGP-LS Attribute of a Node NLRI. rfc/short/rfc9552.md declares no row for it |
| [`RFC7752-3.2.3.1-1`](#rfc7752-3.2.3.1-1) The OSPF Route Type TLV is an optional TLV that MAY be present in Prefix NLRIs. (Section 3.2.3.1) | restated | RFC9552-5.2.3.1-1 | RFC 9552 Section 5.2.3.1 keeps the TLV optional and adds a MUST, that it is included when the route type is signaled in the underlying LSA or is determinable from another LSA for the same prefix |
| [`RFC7752-6.1.5-2`](#rfc7752-6.1.5-2) A network operator MAY use a dedicated Route- Reflector infrastructure to distribute Link-State NLRIs. (Section 6.1.5) | restated | RFC9552-8.1.1-2 | RFC 9552 raises the permission to a recommendation. Its Section 8.1.1 states that dedicated route reflectors SHOULD handle BGP-LS NLRI distribution, and names separate BGP instances or sessions as the alternative when none are available |
| [`RFC7752-6.2.5-2`](#rfc7752-6.2.5-2) An implementation MAY also enhance this information by recording peak per-second counts in each case. (Section 6.2.5) | unextracted | §8.2.5 | RFC 9552 Section 8.2.5 keeps the sentence, that an implementation MAY enhance the statistics by recording peak per-second counts. rfc/short/rfc9552.md declares no row for it |
