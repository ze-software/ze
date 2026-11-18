# RFC 7752 - North-Bound Distribution of Link-State and Traffic Engineering (TE) Information Using BGP

BGP-LS. Every requirement listed for RFC 7752, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 9 | of 26 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 26 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 26 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Tests shown to detect a fault | 17 | 17 of 36 tests linked to requirements, 0 without a fault test and 8 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Partial test coverage | 1 | of 26 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 15 | of 26 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Audit verdicts | 12 | of 26 mandatory requirements reviewed | 2 weak, wrong or unimplemented, 2 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 26 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 26 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 26 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 26 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 26 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 26 | of 52 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 26 mandatory requirements: they add up to 26. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | bad | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | BGP-LS |
| Automatic checks | Included |
| Requirements | 52 |
| Mandatory requirements (MUST level) | 26 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 16 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 36 |
| Tests linked to requirements | 36 |
| Recorded audit verdicts | 12 |
| Fault test records | 25 |
| Summary | `rfc/short/rfc7752.md` |
| Requirement table | `rfc/requirements/rfc7752.md` |
| RFC text | `rfc/full/rfc7752.txt` |

## Automatic checks

Included in automatic checks: North-Bound Distribution of Link-State and TE Information Using BGP

## Published support status

**Area:** BGP-LS

**Test verification:** 6 of 26 MUST proven

**Evidence:** unit 6

**Unmet:** RFC7752-3.2-2, RFC7752-3.2-3, RFC7752-3.2.1-1, RFC7752-3.2.1.1-1, RFC7752-3.2.1.1-2, RFC7752-3.2.1.4-1, RFC7752-3.2.1.4-2, RFC7752-3.2.1.5-1, RFC7752-3.3-1, RFC7752-3.3.2.1-1, RFC7752-3.3.2.2-1, RFC7752-3.3.2.7-1, RFC7752-3.3.3-1, RFC7752-3.4-1, RFC7752-6.2.2-1, RFC7752-6.2.6-1, RFC7752-8-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 9 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 17 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 1 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **26** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (9):** [`RFC7752-3.1-1`](#rfc7752-3.1-1), [`RFC7752-3.1-2`](#rfc7752-3.1-2), [`RFC7752-3.1-3`](#rfc7752-3.1-3), [`RFC7752-3.2-1`](#rfc7752-3.2-1), [`RFC7752-3.2-4`](#rfc7752-3.2-4), [`RFC7752-3.3.2.3-1`](#rfc7752-3.3.2.3-1), [`RFC7752-6.2.2-2`](#rfc7752-6.2.2-2), [`RFC7752-3.2-5`](#rfc7752-3.2-5), [`RFC7752-3.2-6`](#rfc7752-3.2-6)

**Annotated (including scoped evidence) (17):** [`RFC7752-3.2-2`](#rfc7752-3.2-2), [`RFC7752-3.2-3`](#rfc7752-3.2-3), [`RFC7752-3.2.1-1`](#rfc7752-3.2.1-1), [`RFC7752-3.2.1.1-1`](#rfc7752-3.2.1.1-1), [`RFC7752-3.2.1.1-2`](#rfc7752-3.2.1.1-2), [`RFC7752-3.2.1.4-1`](#rfc7752-3.2.1.4-1), [`RFC7752-3.2.1.4-2`](#rfc7752-3.2.1.4-2), [`RFC7752-3.2.1.4-3`](#rfc7752-3.2.1.4-3), [`RFC7752-3.2.1.5-1`](#rfc7752-3.2.1.5-1), [`RFC7752-3.3-1`](#rfc7752-3.3-1), [`RFC7752-3.3.2.1-1`](#rfc7752-3.3.2.1-1), [`RFC7752-3.3.2.2-1`](#rfc7752-3.3.2.2-1), [`RFC7752-3.3.3-1`](#rfc7752-3.3.3-1), [`RFC7752-3.4-1`](#rfc7752-3.4-1), [`RFC7752-6.2.2-1`](#rfc7752-6.2.2-1), [`RFC7752-6.2.6-1`](#rfc7752-6.2.6-1), [`RFC7752-8-1`](#rfc7752-8-1)

**Partial test coverage (subset of annotated; zero whole-requirement credit) (1):** [`RFC7752-3.4-1`](#rfc7752-3.4-1)

## Figures

- **MUST bar**: Ze verified 6, Tests awaiting verification 4, Not built 16
- **Overview bar**: Ze MUST verified 6, Tests awaiting verification 4, Not assessed 25, Not built 17

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 26 |
| MUST rows kept (the MUST bar) | 26 |
| Ze verified | 6 |
| OS verified | 0 |
| Tests awaiting verification | 4 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 16 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 26 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 25 |
| SHOULD not built | 1 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc7752.json](../../../data/rfc/rfc7752.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7752-3.1-1` | Unrecognized types MUST be preserved and propagated. (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC7752UnknownTLVPreservedAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L66). **positive:** `unit/verify` [`TestRFC7752UnknownTLVsReachForwardDestinations`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_unknown_forward_test.go#L11). **negative:** `unit/verify` [`TestRFC7752UnknownTLVsReachForwardDestinations`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_unknown_forward_test.go#L12) |
| `RFC7752-3.1-2` | In order to compare NLRIs with unknown TLVs, all TLVs MUST be ordered in ascending order by TLV Type. (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestLinkDescriptorOrdersMixedFamilyAddressesAscending`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/bgpls/rfc9552_ordering_test.go#L42). **positive:** `unit/verify` [`TestRFC9552NativeLinkTLVsCanonicalOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_ordering_test.go#L95). **negative:** `unit/verify` [`TestNoDescriptorEmitsADescendingTLVSequence`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/bgpls/rfc9552_ordering_test.go#L76). **negative:** `unit/verify` [`TestRFC9552NativeLinkTLVsReorderedSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_ordering_test.go#L117) |
| `RFC7752-3.1-3` | If there are more TLVs of the same type, then the TLVs MUST be ordered in ascending order of the TLV value within the TLVs with the same type by treating the entire Value field as an opaque hexadecimal string and comparing leftmost octets first, regardless of the length of the string. (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC9552NativeLinkTLVsCanonicalOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_ordering_test.go#L97). **negative:** `unit/verify` [`TestRFC9552NativeLinkTLVsReorderedSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_ordering_test.go#L119) |
| `RFC7752-3.2-1` | In order for two BGP speakers to exchange Link-State NLRI, they MUST use BGP Capabilities Advertisement to ensure that they are both capable of properly processing such NLRI. (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestMultiprotocolApplicationsAdvertiseLiteralCodeOne`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/multiprotocol_application_code_test.go#L29). **positive:** `unit/verify` [`TestRFC7752BGPLSCapabilityAdvertisedAndNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7752_bgpls_test.go#L28). **positive:** `unit/verify` [`TestRFC9552LinkStateSentToCapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_capability_test.go#L56). **negative:** `unit/verify` [`TestMultiprotocolApplicationsRejectUnrecognizedCapabilityCode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/multiprotocol_application_code_test.go#L108). **negative:** `unit/verify` [`TestRFC7752BGPLSCapabilityNotNegotiatedWhenPeerSilent`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7752_bgpls_test.go#L62). **negative:** `unit/verify` [`TestRFC9552LinkStateNeverSentToIncapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_capability_test.go#L79) |
| `RFC7752-3.2-2` | For all information derived from other protocols, the corresponding Protocol-ID MUST be used. (Section 3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: ze meets it, because the native BGP-LS exporter stamps every NLRI of a snapshot with the Protocol-ID of the link-state source it was derived from (internal/component/bgp/plugins/ls_export/export_encode.go::encodeTopology, from the domain's protocol-id in internal/component/bgp/plugins/ls_export/export_config.go::parseExportConfig); no test asserts it |
| `RFC7752-3.2-3` | The NLRIs representing link-state objects (nodes, links, or prefixes) from the same routing universe MUST have the same 'Identifier' value. (Section 3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: ze meets it, because the exporter maps each configured domain to one instance-id (internal/component/bgp/plugins/ls_export/export_config.go::parseExportConfig) and writes that Identifier on every NLRI of the domain (internal/component/bgp/plugins/ls_export/export_encode.go::encodeTopology); no test asserts it |
| `RFC7752-3.2-4` | NLRIs with different 'Identifier' values MUST be considered to be from different routing universes. (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC7752DifferentIdentifierIsDifferentRoutingUniverse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc7752_bgpls_test.go#L45). **negative:** `unit/verify` [`TestRFC7752SameIdentifierIsOneRoutingUniverse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc7752_bgpls_test.go#L78) |
| `RFC7752-3.2.1-1` | These auxiliary Router-IDs MUST be included in the link attribute described in Section 3.3.2. (Section 3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: ze meets it, because the exporter carries every attribute TLV its link-state source supplies for a link, auxiliary Router-IDs included, into the link attribute (internal/component/bgp/plugins/ls_export/export_encode.go::originateAttributes, called from encodeTopology); no test asserts it |
| `RFC7752-3.2.1.1-1` | The same node MUST NOT be represented by two keys (otherwise, one node will look like two nodes). (Section 3.2.1.1) | MUST NOT | 3.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze originates Node NLRI from its own link-state database through internal/component/bgp/plugins/ls_export/export_encode.go::encodeTopology, keyed by exportNodeID, and RFC 9552 Section 5.2.1.1 keeps this sentence while rfc/short/rfc9552.md declares no row for it, so no test proves the key never represents one node twice |
| `RFC7752-3.2.1.1-2` | Two different nodes MUST NOT be represented by the same key (otherwise, two nodes will look like one node). (Section 3.2.1.1) | MUST NOT | 3.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze originates Node NLRI from its own link-state database through internal/component/bgp/plugins/ls_export/export_encode.go::encodeTopology, keyed by exportNodeID, and RFC 9552 Section 5.2.1.1 keeps this sentence while rfc/short/rfc9552.md declares no row for it, so no test proves the key never represents two nodes once |
| `RFC7752-3.2.1.4-1` | The combination of ASN and BGP-LS ID MUST be globally unique. (Section 3.2.1.4) | MUST | 3.2.1.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no test carries this RFC 7752 id. ze's BGP-LS exporter writes the ASN and BGP-LS Identifier its link-state source supplies (internal/component/bgp/plugins/ls_export/export_encode.go::nativeOrigin) and checks no global uniqueness; RFC 9552 deprecates the BGP-LS Identifier sub-TLV |
| `RFC7752-3.2.1.4-2` | All BGP-LS speakers within an IGP flooding-set (set of IGP nodes within which an LSP/LSA is flooded) MUST use the same ASN, BGP-LS ID tuple. (Section 3.2.1.4) | MUST | 3.2.1.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no test carries this RFC 7752 id. ze's BGP-LS exporter writes the ASN and BGP-LS Identifier its link-state source supplies (internal/component/bgp/plugins/ls_export/export_encode.go::nativeOrigin) and checks no agreement across a flooding set; RFC 9552 deprecates the BGP-LS Identifier sub-TLV |
| `RFC7752-3.2.1.4-3` | The sub-TLVs within a Node Descriptor MUST be arranged in ascending order by sub-TLV type. (Section 3.2.1.4) | MUST | 3.2.1.4 | **positive:** `unit/verify` [`TestRFC7752NodeDescriptorSubTLVsAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L166). **negative:** no negative test. **{single-polarity}:** NodeDescriptor.WriteTo emits sub-TLVs 512, 513, 514, 515, 516 and 517 in that fixed ascending order (internal/core/bgp/nlri/bgpls/types_descriptor.go:98), and the ordering duty falls on the sender: parseNodeDescriptorTLVs (internal/core/bgp/nlri/bgpls/types.go:388) accepts sub-TLVs in any order on receipt, so there is no out-of-order input for ze to reject |
| `RFC7752-3.2.1.5-1` | If the value in the MT-ID TLV is derived from OSPF, then the upper 9 bits MUST be set to 0. (Section 3.2.1.5) | MUST | 3.2.1.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: ze meets it, because the exporter refuses an OSPF-derived topology above 127, so the upper 9 bits of the MT-ID it writes are 0 (internal/component/bgp/plugins/ls_export/export_encode.go::validateExportTopology, called from encodeTopology); no test asserts it |
| `RFC7752-3.3-1` | This attribute MUST be ignored for all other address families. (Section 3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parsePathAttributesZe decodes a type-29 attribute into the "bgp-ls" key for every UPDATE it walks, with no address-family context: the MP_REACH value is handed back separately and its AFI/SAFI is read only after the attribute loop ends, so a type-29 attribute carried on an IPv4 unicast UPDATE is decoded rather than ignored (internal/component/bgp/cli/decode_update.go:199) |
| `RFC7752-3.3.2.1-1` | All auxiliary Router-IDs of both the local and the remote node MUST be included in the link attribute of each Link NLRI. (Section 3.3.2.1) | MUST | 3.3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: ze meets it, because the exporter carries every attribute TLV its link-state source supplies for a link, the local and remote auxiliary Router-IDs included, into the link attribute of each Link NLRI (internal/component/bgp/plugins/ls_export/export_encode.go::originateAttributes, called from encodeTopology); no test asserts it |
| `RFC7752-3.3.2.2-1` | The MPLS Protocol Mask TLV MUST NOT be included in NLRIs with the other Protocol-IDs listed in Table 2. (Section 3.3.2.2) | MUST NOT | 3.3.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: ze meets it, because the exporter refuses MPLS Protocol Mask TLV 1094 on any NLRI whose source is not Direct or Static (internal/component/bgp/plugins/ls_export/export_encode.go::originateAttributes); no test asserts it |
| `RFC7752-3.3.2.3-1` | If a source protocol uses a metric width of less than 32 bits, then the high- order bits of this field MUST be padded with zero. (Section 3.3.2.3) | MUST | 3.3.2.3 | **positive:** `unit/verify` [`TestRFC7752TEDefaultMetricWidenedWithZeroHighOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc7752_bgpls_export_te_metric_test.go#L47). **positive:** `unit/verify` [`TestRFC7752TEDefaultMetricZeroPadded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L205). **negative:** `unit/verify` [`TestRFC7752TEDefaultMetricAllOnesNeverReachesHighOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc7752_bgpls_export_te_metric_test.go#L59) |
| `RFC7752-3.3.3-1` | Prefixes are learned from the IGP topology (IS-IS or OSPF) with a set of IGP attributes (such as metric, route tags, etc.) that MUST be reflected into the BGP-LS attribute with a prefix NLRI. (Section 3.3.3) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: ze meets it, because the exporter writes each prefix's IGP attributes from its own link-state database into the BGP-LS attribute of the Prefix NLRI (internal/component/bgp/plugins/ls_export/export_encode.go::originateAttributes, called from encodeTopology); no test asserts it |
| `RFC7752-3.4-1` | The next-hop address MUST be encoded as described in [RFC4760]. (Section 3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestRFC7752BGPLSNextHopFollowsRFC4760`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7752_bgpls_test.go#L24). **negative:** no negative test. **{partial}:** tested "MUST be encoded"; gap "as described in [RFC4760]"; the split is by family and address form. Existing positive evidence covers AFI 16388 SAFI 71 single-address layout: the IPv4 address and parsed fields, and the IPv6 length, reserved octet and following NLRI. It does not cover the SAFI 71 global-plus-link-local IPv6 pair or any SAFI 72 form. internal/core/bgp/attribute/mpnlri.go::MPReachNLRI.WriteTo now uses the registered shared profile for field sizing and writing. The LS plugin registers SAFI 71 lengths 4/16/32 and SAFI 72 lengths 12/24/48. The shared NextHopProfile.WriteNextHops writer prefixes each SAFI 72 address with a zero RD, DecodeNextHops reads those framed forms, and buildMPReachPlugin uses the same writer for singles. This framing is implemented, but the new literal-wire and forwarding tests are untagged and supply no audited proof for this row. Negative coverage and current producer-break records remain absent. The existing tagged IPv6 case does not assert address bytes or decoding. The selected 12/24/48 per-address RD layout is supported by the pinned Wireshark v4.4.0 decoder. Live routing-peer acceptance of its paired form remains unverified. plan/release/fixes/spec-bgp-next-hop-auto-rewrites-for-ebgp.md AC-12 owns the remaining proof and profile/scope integration. No whole-row enforcement is claimed. **Partial test coverage:** Tests and fault test records cover only the part marked Tested; they do not verify the whole requirement. weak (the tests pass over code that does not enforce the requirement), fresh. Independent rejudgment on 2026-11-16 of only RFC7752-3.4-1, against the applied summary row and current working tree (comparison HEAD d7a7a46affc8c2c272cea0458fc7e525c7e61077). Read both full RFC 7752 and RFC 9552, RFC 4760 in full, the exact applied correction, the sole tagged unit, and the current producing boundaries. RFC 7752 Section 3.4: "The next-hop address MUST be encoded as described in [RFC4760]." RFC 9552 obsoletes RFC 7752; the legacy row retains its restated-successor link, and the sentence is unchanged. RFC 4760 Section 3 defines the length as "A 1-octet field whose value expresses the length of the "Network Address of Next Hop" field, measured in octets." Its MP_REACH envelope puts AFI, SAFI, this length, the address bytes, a zero Reserved octet and then NLRI. The LS section additionally describes native IPv4, global IPv6, a global/link-local pair and the VPN zero-RD prefix. The row remains annotated partial, with Tested "MUST be encoded" and Gap "as described in [RFC4760]". These are valid source selectors; they do not divide this single MUST into independently fulfilled clauses. The Reason correctly bounds its declared Tested evidence by family/address form and distinguishes implemented framing from missing proof. Producer is internal/core/bgp/attribute/mpnlri.go::MPReachNLRI.WriteTo, which exists and was read. Q1: TestRFC7752BGPLSNextHopFollowsRFC4760 makes RFC-derived layout assertions for SAFI 71. Its IPv4 arm independently checks literal AFI 16388, SAFI 71, length 4, address 192.0.2.1, zero Reserved and following NLRI; ParseMPReachNLRI is checked for AFI/SAFI/NLRI, not decoded next-hop identity. The IPv6 arm checks length 16, zero Reserved and following NLRI, but not the IPv6 address bytes, AFI/SAFI or decoded next hop. Its tag prose therefore exceeds what that arm proves about the address. Q2: IPv4 address corruption or misplaced envelope fields would fail its independent literals. Corrupting only the IPv6 address bytes while retaining length/reserved/NLRI can remain green. There is no directly tagged negative input and no discrimination record for either requirement found in the bounded record search; no current producer-break observation is inferred. Q3: the direct codec test isolates the MP_REACH envelope, with real IPv4/global-IPv6 address values and independently located Reserved/NLRI boundaries, rather than passing because another rejection fired. It does not exercise negotiated family admission, the linked LS plugin profile or a daemon receiver. The opaque Node NLRI bytes are compared rather than independently validated as a complete emitted link-state object; this is codec-layout evidence only. Q4: the tagged unit covers only SAFI 71 singles. It proves no SAFI 71 global/link-local pair and no SAFI 72 address/RD form, and has no negative/current-break proof pair. New literal-wire/profile/forwarding tests are untagged and cannot supply audited credit to this row. Live routing-peer acceptance of the LS72 paired form remains unverified. Thus weak remains the honest verdict, not partial/enforced; zero whole-requirement credit. The earlier implementation-absence explanation is superseded by actual current source: the LS plugin registers SAFI 71 lengths 4/16/32 and SAFI 72 lengths 12/24/48 with RDPerAddress. MPReachNLRI.WriteTo selects and sizes through the registered profile and calls NextHopProfile.WriteNextHops; that writer explicitly clears an eight-octet RD before each SAFI 72 address. NextHopProfile.DecodeNextHops and decodeMPNextHopRD read 12/24/48 framing; buildMPReachPlugin selects the same profile and writer for singles. The removed writeNextHops symbol is not cited. These producers match the comparison HEAD; this audit corrects proof context and does not implement or newly test them. The applied private correction b67b8385d7fd4fb2a25291ef8b27fb18e6c9fd2d5770c588e80c76516d43aa67 exactly matches the current row; quote, ID, level, Tested/Gap selectors and RFC7752 successor suffix are preserved. Its Wireshark v4.4.0 layout statement was checked against the primary packet-bgp.c source: decode_mp_next_hop dispatches AFNUM_BGP_LS/SAFNUM_BGP_LS_VPN to VPN decoders, which recognize zero-RD+IPv4, zero-RD+IPv6 and two RD+IPv6 slots. This is decoder-backed layout evidence, not routing-peer acceptance (https://raw.githubusercontent.com/wireshark/wireshark/v4.4.0/epan/dissectors/packet-bgp.c). No summary/test/tag/discrimination/source edit, runtime test, mutant/revert record, interop execution or full-RFC conformance claim is made by this rejudgment. ROOT still owes native rfc check and the declared proof/real-entry-point work. |
| `RFC7752-6.2.2-1` | If an implementation of BGP-LS detects a malformed attribute, then it MUST use the 'Attribute Discard' action as per [RFC7606], Section 2. (Section 6.2.2) | MUST | 6.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the RFC 7606 validator table registers no entry for attribute code 29 (internal/component/bgp/message/rfc7606.go:414), so validateAttribute returns nil for a malformed BGP-LS attribute (internal/component/bgp/message/rfc7606.go:433) and the attribute rides through the UPDATE path instead of being discarded |
| `RFC7752-6.2.2-2` | An implementation of BGP-LS MUST perform the following syntactic checks for determining if a message is malformed. o Does the sum of all TLVs found in the BGP-LS attribute correspond to the BGP-LS path attribute length? o Does the sum of all TLVs found in the BGP MP_REACH_NLRI attribute correspond to the BGP MP_REACH_NLRI length? o Does the sum of all TLVs found in the BGP MP_UNREACH_NLRI attribute correspond to the BGP MP_UNREACH_NLRI length? o Does the sum of all TLVs found in a Node, Link or Prefix Descriptor NLRI attribute correspond to the Total NLRI Length field of the Node, Link, or Prefix Descriptors? o Does any fixed-length TLV correspond to the TLV Length field in this document? (Section 6.2.2) | MUST | 6.2.2 | **positive:** `unit/verify` [`TestRFC7752AttrSyntacticChecks`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L136). **positive:** `unit/verify` [`TestRFC7752AttributeLengthSumsOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_length_sums_test.go#L100). **positive:** `unit/verify` [`TestRFC7752LinkStateLengthSumsOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_length_sums_test.go#L25). **negative:** `unit/verify` [`TestRFC7752AttributeLengthSumsOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_length_sums_test.go#L101). **negative:** `unit/verify` [`TestRFC7752LinkStateLengthSumsOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_length_sums_test.go#L26). **negative:** `unit/verify` [`TestRFC7752MalformedTLVNotPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L104) |
| `RFC7752-6.2.6-1` | An implementation MUST have the means to limit inbound updates. (Section 6.2.6) | MUST | 6.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the per-family prefix maximum is mandatory for bgp-ls and enforced per family (internal/component/bgp/reactor/config.go:648, internal/component/bgp/reactor/session_prefix.go:288), but the count it compares comes from countPrefixEntries, a [prefix-length][address] CIDR walk (internal/component/bgp/reactor/session_prefix.go:497) that never parses an RFC 7752 type-length NLRI, so the number checked against the configured bgp-ls maximum bears no relation to the BGP-LS NLRI count in the UPDATE |
| `RFC7752-8-1` | In the context of the BGP peerings associated with this document, a BGP speaker MUST NOT accept updates from a consumer peer. (Section 8) | MUST NOT | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze models no BGP-LS consumer peer. The per-family peer knobs are enable, disable, require and ignore (internal/component/bgp/reactor/config.go:596), and a negotiated family is accepted in both directions (internal/core/bgp/capability/negotiated.go:413), so any peer that negotiates bgp-ls has its Link-State UPDATEs accepted whatever role it plays |
| `RFC7752-3.2-5` | All non-VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 71. (Section 3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestRFC7752NativeNLRIAnnouncedAsSAFI71`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc7752_export_family_test.go#L58). **positive:** `unit/verify` [`TestRFC7752NonVPNFamilyIsAFI16388SAFI71`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L231). **negative:** `unit/verify` [`TestRFC7752NativeNLRINeverAnnouncedAsVPN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc7752_export_family_test.go#L71). **negative:** `unit/verify` [`TestRFC7752NonLinkStateFamilyRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L267) |
| `RFC7752-3.2-6` | VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 72. (Section 3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestRFC7752VPNFamilyIsAFI16388SAFI72`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L249). **positive:** `unit/verify` [`TestRFC7752VPNFamilyReachesMPReach`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_vpn_family_test.go#L18). **negative:** `unit/verify` [`TestRFC7752NonLinkStateFamilyRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L268). **negative:** `unit/verify` [`TestRFC7752VPNFamilyReachesMPReach`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_vpn_family_test.go#L19) |
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
| [`RFC7752-3.2-2`](#rfc7752-3.2-2) For all information derived from other protocols, the corresponding Protocol-ID MUST be used. (Section 3.2) | {gap}, no test | gap in proof: ze meets it, because the native BGP-LS exporter stamps every NLRI of a snapshot with the Protocol-ID of the link-state source it was derived from (internal/component/bgp/plugins/ls_export/export_encode.go::encodeTopology, from the domain's protocol-id in internal/component/bgp/plugins/ls_export/export_config.go::parseExportConfig); no test asserts it |
| [`RFC7752-3.2-3`](#rfc7752-3.2-3) The NLRIs representing link-state objects (nodes, links, or prefixes) from the same routing universe MUST have the same 'Identifier' value. (Section 3.2) | {gap}, no test | gap in proof: ze meets it, because the exporter maps each configured domain to one instance-id (internal/component/bgp/plugins/ls_export/export_config.go::parseExportConfig) and writes that Identifier on every NLRI of the domain (internal/component/bgp/plugins/ls_export/export_encode.go::encodeTopology); no test asserts it |
| [`RFC7752-3.2.1-1`](#rfc7752-3.2.1-1) These auxiliary Router-IDs MUST be included in the link attribute described in Section 3.3.2. (Section 3.2.1) | {gap}, no test | gap in proof: ze meets it, because the exporter carries every attribute TLV its link-state source supplies for a link, auxiliary Router-IDs included, into the link attribute (internal/component/bgp/plugins/ls_export/export_encode.go::originateAttributes, called from encodeTopology); no test asserts it |
| [`RFC7752-3.2.1.1-1`](#rfc7752-3.2.1.1-1) The same node MUST NOT be represented by two keys (otherwise, one node will look like two nodes). (Section 3.2.1.1) | {gap}, no test | ze originates Node NLRI from its own link-state database through internal/component/bgp/plugins/ls_export/export_encode.go::encodeTopology, keyed by exportNodeID, and RFC 9552 Section 5.2.1.1 keeps this sentence while rfc/short/rfc9552.md declares no row for it, so no test proves the key never represents one node twice |
| [`RFC7752-3.2.1.1-2`](#rfc7752-3.2.1.1-2) Two different nodes MUST NOT be represented by the same key (otherwise, two nodes will look like one node). (Section 3.2.1.1) | {gap}, no test | ze originates Node NLRI from its own link-state database through internal/component/bgp/plugins/ls_export/export_encode.go::encodeTopology, keyed by exportNodeID, and RFC 9552 Section 5.2.1.1 keeps this sentence while rfc/short/rfc9552.md declares no row for it, so no test proves the key never represents two nodes once |
| [`RFC7752-3.2.1.4-1`](#rfc7752-3.2.1.4-1) The combination of ASN and BGP-LS ID MUST be globally unique. (Section 3.2.1.4) | {gap}, no test | no test carries this RFC 7752 id. ze's BGP-LS exporter writes the ASN and BGP-LS Identifier its link-state source supplies (internal/component/bgp/plugins/ls_export/export_encode.go::nativeOrigin) and checks no global uniqueness; RFC 9552 deprecates the BGP-LS Identifier sub-TLV |
| [`RFC7752-3.2.1.4-2`](#rfc7752-3.2.1.4-2) All BGP-LS speakers within an IGP flooding-set (set of IGP nodes within which an LSP/LSA is flooded) MUST use the same ASN, BGP-LS ID tuple. (Section 3.2.1.4) | {gap}, no test | no test carries this RFC 7752 id. ze's BGP-LS exporter writes the ASN and BGP-LS Identifier its link-state source supplies (internal/component/bgp/plugins/ls_export/export_encode.go::nativeOrigin) and checks no agreement across a flooding set; RFC 9552 deprecates the BGP-LS Identifier sub-TLV |
| [`RFC7752-3.2.1.5-1`](#rfc7752-3.2.1.5-1) If the value in the MT-ID TLV is derived from OSPF, then the upper 9 bits MUST be set to 0. (Section 3.2.1.5) | {gap}, no test | gap in proof: ze meets it, because the exporter refuses an OSPF-derived topology above 127, so the upper 9 bits of the MT-ID it writes are 0 (internal/component/bgp/plugins/ls_export/export_encode.go::validateExportTopology, called from encodeTopology); no test asserts it |
| [`RFC7752-3.3-1`](#rfc7752-3.3-1) This attribute MUST be ignored for all other address families. (Section 3.3) | {gap}, no test | parsePathAttributesZe decodes a type-29 attribute into the "bgp-ls" key for every UPDATE it walks, with no address-family context: the MP_REACH value is handed back separately and its AFI/SAFI is read only after the attribute loop ends, so a type-29 attribute carried on an IPv4 unicast UPDATE is decoded rather than ignored (internal/component/bgp/cli/decode_update.go:199) |
| [`RFC7752-3.3.2.1-1`](#rfc7752-3.3.2.1-1) All auxiliary Router-IDs of both the local and the remote node MUST be included in the link attribute of each Link NLRI. (Section 3.3.2.1) | {gap}, no test | gap in proof: ze meets it, because the exporter carries every attribute TLV its link-state source supplies for a link, the local and remote auxiliary Router-IDs included, into the link attribute of each Link NLRI (internal/component/bgp/plugins/ls_export/export_encode.go::originateAttributes, called from encodeTopology); no test asserts it |
| [`RFC7752-3.3.2.2-1`](#rfc7752-3.3.2.2-1) The MPLS Protocol Mask TLV MUST NOT be included in NLRIs with the other Protocol-IDs listed in Table 2. (Section 3.3.2.2) | {gap}, no test | gap in proof: ze meets it, because the exporter refuses MPLS Protocol Mask TLV 1094 on any NLRI whose source is not Direct or Static (internal/component/bgp/plugins/ls_export/export_encode.go::originateAttributes); no test asserts it |
| [`RFC7752-3.3.3-1`](#rfc7752-3.3.3-1) Prefixes are learned from the IGP topology (IS-IS or OSPF) with a set of IGP attributes (such as metric, route tags, etc.) that MUST be reflected into the BGP-LS attribute with a prefix NLRI. (Section 3.3.3) | {gap}, no test | gap in proof: ze meets it, because the exporter writes each prefix's IGP attributes from its own link-state database into the BGP-LS attribute of the Prefix NLRI (internal/component/bgp/plugins/ls_export/export_encode.go::originateAttributes, called from encodeTopology); no test asserts it |
| [`RFC7752-3.4-1`](#rfc7752-3.4-1) The next-hop address MUST be encoded as described in [RFC4760]. (Section 3.4) | {partial}, Partial test coverage | tested "MUST be encoded"; gap "as described in [RFC4760]"; the split is by family and address form. Existing positive evidence covers AFI 16388 SAFI 71 single-address layout: the IPv4 address and parsed fields, and the IPv6 length, reserved octet and following NLRI. It does not cover the SAFI 71 global-plus-link-local IPv6 pair or any SAFI 72 form. internal/core/bgp/attribute/mpnlri.go::MPReachNLRI.WriteTo now uses the registered shared profile for field sizing and writing. The LS plugin registers SAFI 71 lengths 4/16/32 and SAFI 72 lengths 12/24/48. The shared NextHopProfile.WriteNextHops writer prefixes each SAFI 72 address with a zero RD, DecodeNextHops reads those framed forms, and buildMPReachPlugin uses the same writer for singles. This framing is implemented, but the new literal-wire and forwarding tests are untagged and supply no audited proof for this row. Negative coverage and current producer-break records remain absent. The existing tagged IPv6 case does not assert address bytes or decoding. The selected 12/24/48 per-address RD layout is supported by the pinned Wireshark v4.4.0 decoder. Live routing-peer acceptance of its paired form remains unverified. plan/release/fixes/spec-bgp-next-hop-auto-rewrites-for-ebgp.md AC-12 owns the remaining proof and profile/scope integration. No whole-row enforcement is claimed |
| [`RFC7752-6.2.2-1`](#rfc7752-6.2.2-1) If an implementation of BGP-LS detects a malformed attribute, then it MUST use the 'Attribute Discard' action as per [RFC7606], Section 2. (Section 6.2.2) | {gap}, no test | the RFC 7606 validator table registers no entry for attribute code 29 (internal/component/bgp/message/rfc7606.go:414), so validateAttribute returns nil for a malformed BGP-LS attribute (internal/component/bgp/message/rfc7606.go:433) and the attribute rides through the UPDATE path instead of being discarded |
| [`RFC7752-6.2.6-1`](#rfc7752-6.2.6-1) An implementation MUST have the means to limit inbound updates. (Section 6.2.6) | {gap}, no test | the per-family prefix maximum is mandatory for bgp-ls and enforced per family (internal/component/bgp/reactor/config.go:648, internal/component/bgp/reactor/session_prefix.go:288), but the count it compares comes from countPrefixEntries, a [prefix-length][address] CIDR walk (internal/component/bgp/reactor/session_prefix.go:497) that never parses an RFC 7752 type-length NLRI, so the number checked against the configured bgp-ls maximum bears no relation to the BGP-LS NLRI count in the UPDATE |
| [`RFC7752-8-1`](#rfc7752-8-1) In the context of the BGP peerings associated with this document, a BGP speaker MUST NOT accept updates from a consumer peer. (Section 8) | {gap}, no test | ze models no BGP-LS consumer peer. The per-family peer knobs are enable, disable, require and ignore (internal/component/bgp/reactor/config.go:596), and a negotiated family is accepted in both directions (internal/core/bgp/capability/negotiated.go:413), so any peer that negotiates bgp-ls has its Link-State UPDATEs accepted whatever role it plays |
| [`RFC7752-3.3.2.7-1`](#rfc7752-3.3.2.7-1) This symbolic name can be the FQDN for the link, it can be a subset of the FQDN, or it can be any string operators want to use for the link. The use of FQDN or a subset of it is strongly RECOMMENDED. (§3.3.2.7) | {gap} | FQDN-based native Link Name selection is not implemented; decodeLinkName preserves a received string and originateAttributes copies supplied TLV values. The optional TLV is supported as received data, not as a native FQDN naming policy. |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC7752-3.1-1`](#rfc7752-3.1-1)

Unrecognized types MUST be preserved and propagated. (Section 3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. D2 rejudge 2026-10-16 (independent judge, BGP-LS codec moved to internal/core/bgp/nlri/bgpls). Tagged units changed only by the package move: identifiers re-qualified with bgpls., comments re-pointed; producer bodies byte-identical modulo the writeTLV->WriteTLV and field-export renames (TLV code constants verified unchanged). RFC 7752 Section 3.1: "Unrecognized types MUST be preserved and propagated." Proof is carried by reactor TestRFC7752UnknownTLVsReachForwardDestinations (exact forwarded NLRI and attribute bytes, internal and external peers, all-unknown attribute variant). The ls unit now calls bgpls.ParseBGPLSWithRest and additionally requires the one NLRI to consume the whole buffer (a strengthening); it keeps the opaque attribute TLV under a generic key and re-encodes the Node NLRI byte-identically.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7752UnknownTLVsReachForwardDestinations`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_unknown_forward_test.go#L12) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7752UnknownTLVPreservedAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestRFC7752UnknownTLVsReachForwardDestinations`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_unknown_forward_test.go#L11) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC7752-3.1-2`](#rfc7752-3.1-2)

In order to compare NLRIs with unknown TLVs, all TLVs MUST be ordered in ascending order by TLV Type. (Section 3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. D2 rejudge 2026-10-16 (independent judge, BGP-LS codec moved to internal/core/bgp/nlri/bgpls). Tagged units changed only by the package move: identifiers re-qualified with bgpls., comments re-pointed; producer bodies byte-identical modulo the writeTLV->WriteTLV and field-export renames (TLV code constants verified unchanged). RFC 7752 Section 3.1: "In order to compare NLRIs with unknown TLVs, all TLVs MUST be ordered in ascending order by TLV Type." The core units (internal/core/bgp/nlri/bgpls/rfc9552_ordering_test.go) are byte-identical to the deleted plugin copies apart from the package clause: mixed-family Link Descriptor emits 260 before 261, and the sweep over link/node/prefix descriptors finds no descending pair. The native whole-NLRI proof stays with ls_export encodeNativeLink (canonical and reordered-source units). Note the negative is a sweep of the sender, not a rejected input; the ordering duty is the originator's.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9552NativeLinkTLVsReorderedSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_ordering_test.go#L117) | unit/verify | revert, verified |
| negative | [`TestNoDescriptorEmitsADescendingTLVSequence`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/bgpls/rfc9552_ordering_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeLinkTLVsCanonicalOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_ordering_test.go#L95) | unit/verify | revert, verified |
| positive | [`TestLinkDescriptorOrdersMixedFamilyAddressesAscending`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/bgpls/rfc9552_ordering_test.go#L42) | unit/verify | revert, verified |

### [`RFC7752-3.1-3`](#rfc7752-3.1-3)

If there are more TLVs of the same type, then the TLVs MUST be ordered in ascending order of the TLV value within the TLVs with the same type by treating the entire Value field as an opaque hexadecimal string and comparing leftmost octets first, regardless of the length of the string. (Section 3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rejudge 2026-09-30 (was wrong: the tagged 518 units asserted the RFC 9552 Length-first rule; tags removed under D-15). Now the ls_export units assert same-type 259/260/261/262 ascending by Value compared leftmost octet first, from ascending and descending sources. Every Type this producer repeats has one fixed Length, so the 'regardless of the length' comparison and RFC 9552's Length-then-Value order give identical output; judge break (Value comparison reversed) turns both red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9552NativeLinkTLVsReorderedSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_ordering_test.go#L119) | unit/verify | revert, verified |
| positive | [`TestRFC9552NativeLinkTLVsCanonicalOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_ordering_test.go#L97) | unit/verify | revert, verified |

### [`RFC7752-3.2-1`](#rfc7752-3.2-1)

In order for two BGP speakers to exchange Link-State NLRI, they MUST use BGP Capabilities Advertisement to ensure that they are both capable of properly processing such NLRI. (Section 3.2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/reactor/multiprotocol_application_code_test.go::TestMultiprotocolApplicationsAdvertiseLiteralCodeOne, internal/component/bgp/reactor/multiprotocol_application_code_test.go::TestMultiprotocolApplicationsRejectUnrecognizedCapabilityCode, internal/component/bgp/reactor/reactor_api_batch.go::AnnounceNLRIBatch, internal/component/bgp/reactor/reactor_api_batch.go::buildBatchAnnounceUpdate, internal/component/bgp/reactor/rfc9552_capability_test.go::TestRFC9552LinkStateNeverSentToIncapablePeer, internal/component/bgp/reactor/rfc9552_capability_test.go::TestRFC9552LinkStateSentToCapablePeer, internal/component/bgp/reactor/session_negotiate.go::buildOpenFromSnapshot, internal/core/bgp/capability/capability.go::Multiprotocol.WriteTo, internal/core/bgp/capability/capability.go::parseCapability, internal/core/bgp/capability/negotiated.go::Negotiate, internal/core/bgp/capability/rfc7752_bgpls_test.go::TestRFC7752BGPLSCapabilityAdvertisedAndNegotiated, internal/core/bgp/capability/rfc7752_bgpls_test.go::TestRFC7752BGPLSCapabilityNotNegotiatedWhenPeerSilent moved. Re-read RFC 7752 Section 3.2's complete capability paragraph and RFC 9552 Section 5.2's restatement against the current summary/derived row. The superseded/restated RFC9552-5.2-7 annotation remains accurate and unchanged. TestRFC7752BGPLSCapabilityAdvertisedAndNegotiated checks encoded length, literal AFI/reserved/SAFI and both-sided BGP-LS intersection; its code comparison alone shares the implementation constant. TestRFC7752BGPLSCapabilityNotNegotiatedWhenPeerSilent excludes BGP-LS from SupportsFamily and Families while retaining unicast, and distinguishes VPN SAFI 72 from SAFI 71. The reactor positive submits a real Node NLRI, requires one UPDATE and literal MP_REACH prefix 40 04 47; the reactor negative submits that same NLRI against unicast-only negotiated families and requires exact ErrNoPeersAcceptedFamily and no UPDATE bodies. Their shared fixture publishes the same family/encoding choices to Session and preserves all original checks. Together these retain both-sided capability-gated exchange; no receive-side validation or additional NLRI-type support is credited. The configured OPEN fixture's test-only Session.buildOpen delegates directly to the current production buildOpenFromSnapshot with one captured plugin snapshot. In production prepareOpen retains the built message with its settings and declarations in one preparedOpen; sendOpen writes that message, and prepareNegotiation/publishNegotiation retain the advertised local/remote family intersection. The obsolete production buildOpen code key is replaced by buildOpenFromSnapshot; all other existing code anchors and tagged covers remain. The literal positive independently walks parameter and capability TLVs from the configured OPEN, requiring exactly one literal code 1, length 4 and exact AFI/reserved/SAFI payload for each exercised application (1/133, 1/134, 16388/71), then requires application and unicast negotiation against a separately handwritten code-1 peer stream. The negative provides a valid code-1 unicast control and the application payload under code 250, requires successful parsing, first excludes the application, and then requires surviving unicast. Thus a shared codec constant or rejection of every capability cannot satisfy the pair. Existing recorded breaks target Multiprotocol.WriteTo emitting 250 and parseCapability recognizing 250; the latter must fail for acceptance of the bad application, not solely loss of the unicast control. This independent current-source judgment credits only the capability-advertisement obligation. Main's final native inventories at HEAD 606a3ba2 report all existing records current: four for RFC5575-4-1, five for RFC7752-3.2-1, two for RFC8955-4-1 and five for RFC9552-5.2-7, with no stale bindings. This continuation read all sixteen actual record objects and every tagged test and reached producer before stamping. The eight numeric-code records and eight partner records remain intact; four supplementary registration/core-positive tagged covers have no historical record and are judged from their assertions without inventing execution. This verdict covers this capability-advertisement requirement alone, not whole-RFC or additional-NLRI conformance.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMultiprotocolApplicationsRejectUnrecognizedCapabilityCode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/multiprotocol_application_code_test.go#L108) | unit/verify | mutant, verified |
| negative | [`TestRFC9552LinkStateNeverSentToIncapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_capability_test.go#L79) | unit/verify | revert, producer-gone (the producer the break was applied to is no longer in the tree) |
| negative | [`TestRFC7752BGPLSCapabilityNotNegotiatedWhenPeerSilent`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7752_bgpls_test.go#L62) | unit/verify | revert, verified |
| positive | [`TestMultiprotocolApplicationsAdvertiseLiteralCodeOne`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/multiprotocol_application_code_test.go#L29) | unit/verify | mutant, verified |
| positive | [`TestRFC9552LinkStateSentToCapablePeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9552_capability_test.go#L56) | unit/verify | revert, producer-gone (the producer the break was applied to is no longer in the tree) |
| positive | [`TestRFC7752BGPLSCapabilityAdvertisedAndNegotiated`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc7752_bgpls_test.go#L28) | unit/verify | not verified |

### [`RFC7752-3.2-2`](#rfc7752-3.2-2)

For all information derived from other protocols, the corresponding Protocol-ID MUST be used. (Section 3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7752-3.2-2, so there is no linked test.

### [`RFC7752-3.2-3`](#rfc7752-3.2-3)

The NLRIs representing link-state objects (nodes, links, or prefixes) from the same routing universe MUST have the same 'Identifier' value. (Section 3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7752-3.2-3, so there is no linked test.

### [`RFC7752-3.2-4`](#rfc7752-3.2-4)

NLRIs with different 'Identifier' values MUST be considered to be from different routing universes. (Section 3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: merging NLRIs that differ only in Identifier. TestRFC7752DifferentIdentifierIsDifferentRoutingUniverse inserts two Node NLRIs identical except Identifier 0 and 1 and fails unless rib.Len() is 2, both are retrievable, and removing one leaves the other. Negative: TestRFC7752SameIdentifierIsOneRoutingUniverse fails unless a repeat with the same Identifier collapses to one route (Len 1), so the separation turns on the Identifier bytes.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7752SameIdentifierIsOneRoutingUniverse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc7752_bgpls_test.go#L78) | unit/verify | not verified |
| positive | [`TestRFC7752DifferentIdentifierIsDifferentRoutingUniverse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc7752_bgpls_test.go#L45) | unit/verify | not verified |

### [`RFC7752-3.2.1-1`](#rfc7752-3.2.1-1)

These auxiliary Router-IDs MUST be included in the link attribute described in Section 3.3.2. (Section 3.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7752-3.2.1-1, so there is no linked test.

### [`RFC7752-3.2.1.1-1`](#rfc7752-3.2.1.1-1)

The same node MUST NOT be represented by two keys (otherwise, one node will look like two nodes). (Section 3.2.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7752-3.2.1.1-1, so there is no linked test.

### [`RFC7752-3.2.1.1-2`](#rfc7752-3.2.1.1-2)

Two different nodes MUST NOT be represented by the same key (otherwise, two nodes will look like one node). (Section 3.2.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7752-3.2.1.1-2, so there is no linked test.

### [`RFC7752-3.2.1.4-1`](#rfc7752-3.2.1.4-1)

The combination of ASN and BGP-LS ID MUST be globally unique. (Section 3.2.1.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7752-3.2.1.4-1, so there is no linked test.

### [`RFC7752-3.2.1.4-2`](#rfc7752-3.2.1.4-2)

All BGP-LS speakers within an IGP flooding-set (set of IGP nodes within which an LSP/LSA is flooded) MUST use the same ASN, BGP-LS ID tuple. (Section 3.2.1.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7752-3.2.1.4-2, so there is no linked test.

### [`RFC7752-3.2.1.4-3`](#rfc7752-3.2.1.4-3)

The sub-TLVs within a Node Descriptor MUST be arranged in ascending order by sub-TLV type. (Section 3.2.1.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. D2 rejudge 2026-10-16 (independent judge, BGP-LS codec moved to internal/core/bgp/nlri/bgpls). Tagged units changed only by the package move: identifiers re-qualified with bgpls., comments re-pointed; producer bodies byte-identical modulo the writeTLV->WriteTLV and field-export renames (TLV code constants verified unchanged). RFC 7752 Section 3.2.1.4: "The sub-TLVs within a Node Descriptor MUST be arranged in ascending order by sub-TLV type." {single-polarity: positive} holds: TestRFC7752NodeDescriptorSubTLVsAscending fails unless bgpls.NodeDescriptor.WriteTo emits exactly 512..517 in order; that WriteTo is the production encoder (ls_export exportNodeID builds every native Node Descriptor through it, now via bgpls.NewBGPLSNode/Link/Prefix).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC7752NodeDescriptorSubTLVsAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L166) | unit/verify | not verified |

### [`RFC7752-3.2.1.5-1`](#rfc7752-3.2.1.5-1)

If the value in the MT-ID TLV is derived from OSPF, then the upper 9 bits MUST be set to 0. (Section 3.2.1.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7752-3.2.1.5-1, so there is no linked test.

### [`RFC7752-3.3-1`](#rfc7752-3.3-1)

This attribute MUST be ignored for all other address families. (Section 3.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7752-3.3-1, so there is no linked test.

### [`RFC7752-3.3.2.1-1`](#rfc7752-3.3.2.1-1)

All auxiliary Router-IDs of both the local and the remote node MUST be included in the link attribute of each Link NLRI. (Section 3.3.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7752-3.3.2.1-1, so there is no linked test.

### [`RFC7752-3.3.2.2-1`](#rfc7752-3.3.2.2-1)

The MPLS Protocol Mask TLV MUST NOT be included in NLRIs with the other Protocol-IDs listed in Table 2. (Section 3.3.2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7752-3.3.2.2-1, so there is no linked test.

### [`RFC7752-3.3.2.3-1`](#rfc7752-3.3.2.3-1)

If a source protocol uses a metric width of less than 32 bits, then the high- order bits of this field MUST be padded with zero. (Section 3.3.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. D2 rejudge 2026-10-16 (independent judge, BGP-LS codec moved to internal/core/bgp/nlri/bgpls). Tagged units changed only by the package move: identifiers re-qualified with bgpls., comments re-pointed; producer bodies byte-identical modulo the writeTLV->WriteTLV and field-export renames (TLV code constants verified unchanged). RFC 7752 Section 3.3.2.3: "If a source protocol uses a metric width of less than 32 bits, then the high-order bits of this field MUST be padded with zero." Proof is the IS-IS pair (TestRFC7752TEDefaultMetricWidenedWithZeroHighOrder 0a0b0c->000a0b0c, TestRFC7752TEDefaultMetricAllOnesNeverReachesHighOrder ffffff->00ffffff) over the real producer isis bgpls_export. The plugin ls unit TestRFC7752TEDefaultMetricZeroPadded proves nothing about emitted bytes: lsTEDefaultMetric is constructed only by its decoder and its WriteTo has no non-test caller; it is supplementary and earns no credit.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7752TEDefaultMetricAllOnesNeverReachesHighOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc7752_bgpls_export_te_metric_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestRFC7752TEDefaultMetricZeroPadded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L205) | unit/verify | not verified |
| positive | [`TestRFC7752TEDefaultMetricWidenedWithZeroHighOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc7752_bgpls_export_te_metric_test.go#L47) | unit/verify | revert, verified |

### [`RFC7752-3.3.3-1`](#rfc7752-3.3.3-1)

Prefixes are learned from the IGP topology (IS-IS or OSPF) with a set of IGP attributes (such as metric, route tags, etc.) that MUST be reflected into the BGP-LS attribute with a prefix NLRI. (Section 3.3.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7752-3.3.3-1, so there is no linked test.

### [`RFC7752-3.4-1`](#rfc7752-3.4-1)

The next-hop address MUST be encoded as described in [RFC4760]. (Section 3.4)

Records cover only part of this requirement: tested "MUST be encoded"; gap "as described in [RFC4760]"; the split is by family and address form. Existing positive evidence covers AFI 16388 SAFI 71 single-address layout: the IPv4 address and parsed fields, and the IPv6 length, reserved octet and following NLRI. It does not cover the SAFI 71 global-plus-link-local IPv6 pair or any SAFI 72 form. internal/core/bgp/attribute/mpnlri.go::MPReachNLRI.WriteTo now uses the registered shared profile for field sizing and writing. The LS plugin registers SAFI 71 lengths 4/16/32 and SAFI 72 lengths 12/24/48. The shared NextHopProfile.WriteNextHops writer prefixes each SAFI 72 address with a zero RD, DecodeNextHops reads those framed forms, and buildMPReachPlugin uses the same writer for singles. This framing is implemented, but the new literal-wire and forwarding tests are untagged and supply no audited proof for this row. Negative coverage and current producer-break records remain absent. The existing tagged IPv6 case does not assert address bytes or decoding. The selected 12/24/48 per-address RD layout is supported by the pinned Wireshark v4.4.0 decoder. Live routing-peer acceptance of its paired form remains unverified. plan/release/fixes/spec-bgp-next-hop-auto-rewrites-for-ebgp.md AC-12 owns the remaining proof and profile/scope integration. No whole-row enforcement is claimed; they do not verify the whole requirement.

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Independent rejudgment on 2026-11-16 of only RFC7752-3.4-1, against the applied summary row and current working tree (comparison HEAD d7a7a46affc8c2c272cea0458fc7e525c7e61077). Read both full RFC 7752 and RFC 9552, RFC 4760 in full, the exact applied correction, the sole tagged unit, and the current producing boundaries. RFC 7752 Section 3.4: "The next-hop address MUST be encoded as described in [RFC4760]." RFC 9552 obsoletes RFC 7752; the legacy row retains its restated-successor link, and the sentence is unchanged. RFC 4760 Section 3 defines the length as "A 1-octet field whose value expresses the length of the "Network Address of Next Hop" field, measured in octets." Its MP_REACH envelope puts AFI, SAFI, this length, the address bytes, a zero Reserved octet and then NLRI. The LS section additionally describes native IPv4, global IPv6, a global/link-local pair and the VPN zero-RD prefix. The row remains annotated partial, with Tested "MUST be encoded" and Gap "as described in [RFC4760]". These are valid source selectors; they do not divide this single MUST into independently fulfilled clauses. The Reason correctly bounds its declared Tested evidence by family/address form and distinguishes implemented framing from missing proof. Producer is internal/core/bgp/attribute/mpnlri.go::MPReachNLRI.WriteTo, which exists and was read. Q1: TestRFC7752BGPLSNextHopFollowsRFC4760 makes RFC-derived layout assertions for SAFI 71. Its IPv4 arm independently checks literal AFI 16388, SAFI 71, length 4, address 192.0.2.1, zero Reserved and following NLRI; ParseMPReachNLRI is checked for AFI/SAFI/NLRI, not decoded next-hop identity. The IPv6 arm checks length 16, zero Reserved and following NLRI, but not the IPv6 address bytes, AFI/SAFI or decoded next hop. Its tag prose therefore exceeds what that arm proves about the address. Q2: IPv4 address corruption or misplaced envelope fields would fail its independent literals. Corrupting only the IPv6 address bytes while retaining length/reserved/NLRI can remain green. There is no directly tagged negative input and no discrimination record for either requirement found in the bounded record search; no current producer-break observation is inferred. Q3: the direct codec test isolates the MP_REACH envelope, with real IPv4/global-IPv6 address values and independently located Reserved/NLRI boundaries, rather than passing because another rejection fired. It does not exercise negotiated family admission, the linked LS plugin profile or a daemon receiver. The opaque Node NLRI bytes are compared rather than independently validated as a complete emitted link-state object; this is codec-layout evidence only. Q4: the tagged unit covers only SAFI 71 singles. It proves no SAFI 71 global/link-local pair and no SAFI 72 address/RD form, and has no negative/current-break proof pair. New literal-wire/profile/forwarding tests are untagged and cannot supply audited credit to this row. Live routing-peer acceptance of the LS72 paired form remains unverified. Thus weak remains the honest verdict, not partial/enforced; zero whole-requirement credit. The earlier implementation-absence explanation is superseded by actual current source: the LS plugin registers SAFI 71 lengths 4/16/32 and SAFI 72 lengths 12/24/48 with RDPerAddress. MPReachNLRI.WriteTo selects and sizes through the registered profile and calls NextHopProfile.WriteNextHops; that writer explicitly clears an eight-octet RD before each SAFI 72 address. NextHopProfile.DecodeNextHops and decodeMPNextHopRD read 12/24/48 framing; buildMPReachPlugin selects the same profile and writer for singles. The removed writeNextHops symbol is not cited. These producers match the comparison HEAD; this audit corrects proof context and does not implement or newly test them. The applied private correction b67b8385d7fd4fb2a25291ef8b27fb18e6c9fd2d5770c588e80c76516d43aa67 exactly matches the current row; quote, ID, level, Tested/Gap selectors and RFC7752 successor suffix are preserved. Its Wireshark v4.4.0 layout statement was checked against the primary packet-bgp.c source: decode_mp_next_hop dispatches AFNUM_BGP_LS/SAFNUM_BGP_LS_VPN to VPN decoders, which recognize zero-RD+IPv4, zero-RD+IPv6 and two RD+IPv6 slots. This is decoder-backed layout evidence, not routing-peer acceptance (https://raw.githubusercontent.com/wireshark/wireshark/v4.4.0/epan/dissectors/packet-bgp.c). No summary/test/tag/discrimination/source edit, runtime test, mutant/revert record, interop execution or full-RFC conformance claim is made by this rejudgment. ROOT still owes native rfc check and the declared proof/real-entry-point work.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC7752BGPLSNextHopFollowsRFC4760`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7752_bgpls_test.go#L24) | unit/verify | not verified |

### [`RFC7752-6.2.2-1`](#rfc7752-6.2.2-1)

If an implementation of BGP-LS detects a malformed attribute, then it MUST use the 'Attribute Discard' action as per [RFC7606], Section 2. (Section 6.2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7752-6.2.2-1, so there is no linked test.

### [`RFC7752-6.2.2-2`](#rfc7752-6.2.2-2)

An implementation of BGP-LS MUST perform the following syntactic checks for determining if a message is malformed. o Does the sum of all TLVs found in the BGP-LS attribute correspond to the BGP-LS path attribute length? o Does the sum of all TLVs found in the BGP MP_REACH_NLRI attribute correspond to the BGP MP_REACH_NLRI length? o Does the sum of all TLVs found in the BGP MP_UNREACH_NLRI attribute correspond to the BGP MP_UNREACH_NLRI length? o Does the sum of all TLVs found in a Node, Link or Prefix Descriptor NLRI attribute correspond to the Total NLRI Length field of the Node, Link, or Prefix Descriptors? o Does any fixed-length TLV correspond to the TLV Length field in this document? (Section 6.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-23 (independent auditor, after 5d4afb0c4e). RFC 7752 Section 6.2.2: "An implementation of BGP-LS MUST perform the following syntactic checks for determining if a message is malformed." followed by the five length-sum questions (BGP-LS attribute, MP_REACH_NLRI, MP_UNREACH_NLRI, descriptor Total NLRI Length, fixed-length TLVs). TestRFC7752LinkStateLengthSumsOnReceive changed only the NOTIFICATION it requires for the outer NLRI overrun: across NLRI types 1, 2 and 3, in MP_REACH_NLRI and MP_UNREACH_NLRI, an NLRI length one octet too long must give SessionReset with the RFC 9552 Section 8.2.2 length-sum error and the exact frame 3/9 whose Data is the MP attribute alone (mpAttr = attrs[7:] after the 4-octet ORIGIN and 3-octet empty AS_PATH of mpReachAttrs, or the whole of mpUnreachAttrs, checked against both helpers), leaving the session Idle with nothing dispatched; descriptor over and underruns still discard exactly one NLRI with the diagnostic and no NOTIFICATION; exact-sum controls are dispatched unchanged. The assertion is as exact as before; only the expected subcode and Data moved, per RFC 4760 Section 7. TestRFC7752AttributeLengthSumsOnReceive and the ls plugin units are unchanged. Ran green. Verdict stands. Finding recorded outside this verdict: the four discrimination records on these reactor units no longer verify (producer-changed applyRFC7606, and unit-changed for TestRFC7752LinkStateLengthSumsOnReceive).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7752MalformedTLVNotPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L104) | unit/verify | revert, verified |
| negative | [`TestRFC7752AttributeLengthSumsOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_length_sums_test.go#L101) | unit/verify | revert, verified |
| negative | [`TestRFC7752LinkStateLengthSumsOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_length_sums_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestRFC7752AttrSyntacticChecks`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L136) | unit/verify | not verified |
| positive | [`TestRFC7752AttributeLengthSumsOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_length_sums_test.go#L100) | unit/verify | revert, verified |
| positive | [`TestRFC7752LinkStateLengthSumsOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_length_sums_test.go#L25) | unit/verify | revert, verified |

### [`RFC7752-6.2.6-1`](#rfc7752-6.2.6-1)

An implementation MUST have the means to limit inbound updates. (Section 6.2.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7752-6.2.6-1, so there is no linked test.

### [`RFC7752-8-1`](#rfc7752-8-1)

In the context of the BGP peerings associated with this document, a BGP speaker MUST NOT accept updates from a consumer peer. (Section 8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7752-8-1, so there is no linked test.

### [`RFC7752-3.2-5`](#rfc7752-3.2-5)

All non-VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 71. (Section 3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. D2 rejudge 2026-10-16 (independent judge, BGP-LS codec moved to internal/core/bgp/nlri/bgpls). Tagged units changed only by the package move: identifiers re-qualified with bgpls., comments re-pointed; producer bodies byte-identical modulo the writeTLV->WriteTLV and field-export renames (TLV code constants verified unchanged). RFC 7752 Section 3.2: "All non-VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 71." BGPLSFamily now registers in plugin ls register.go from family.AFIBGPLS/SAFIBGPLinkState; the unit still pins (16388, 71), bgp-ls/bgp-ls, and that a bgpls.NewBGPLSNode reports that family. Native positive (ls_export TestRFC7752NativeNLRIAnnouncedAsSAFI71) and the non-(16388,71) refusal negative unchanged.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7752NativeNLRINeverAnnouncedAsVPN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc7752_export_family_test.go#L71) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC7752NonLinkStateFamilyRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L267) | unit/verify | not verified |
| positive | [`TestRFC7752NativeNLRIAnnouncedAsSAFI71`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc7752_export_family_test.go#L58) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7752NonVPNFamilyIsAFI16388SAFI71`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L231) | unit/verify | not verified |

### [`RFC7752-3.2-6`](#rfc7752-3.2-6)

VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 72. (Section 3.2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/reactor/rfc7752_vpn_family_test.go::TestRFC7752VPNFamilyReachesMPReach, internal/component/bgp/reactor/rfc7752_vpn_family_test.go::TestRFC7752VPNFamilyReachesMPReach#2 moved. D2 rejudge 2026-10-16 (independent judge, BGP-LS codec moved to internal/core/bgp/nlri/bgpls). Tagged units changed only by the package move: identifiers re-qualified with bgpls., comments re-pointed; producer bodies byte-identical modulo the writeTLV->WriteTLV and field-export renames (TLV code constants verified unchanged). RFC 7752 Section 3.2: "VPN link, node, and prefix information SHALL be encoded using AFI 16388 / SAFI 72." Wire proof stays reactor TestRFC7752VPNFamilyReachesMPReach (exact MP_REACH AFI/SAFI, SAFI 71 arm not conflated); the ls unit pins BGPLSVPNFamily (16388, 72), now registered in register.go.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7752NonLinkStateFamilyRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L268) | unit/verify | not verified |
| negative | [`TestRFC7752VPNFamilyReachesMPReach`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_vpn_family_test.go#L19) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestRFC7752VPNFamilyIsAFI16388SAFI72`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc7752_test.go#L249) | unit/verify | not verified |
| positive | [`TestRFC7752VPNFamilyReachesMPReach`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7752_vpn_family_test.go#L18) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC7752-3.3.2.7-1`](#rfc7752-3.3.2.7-1)

This symbolic name can be the FQDN for the link, it can be a subset of the FQDN, or it can be any string operators want to use for the link. The use of FQDN or a subset of it is strongly RECOMMENDED. (§3.3.2.7)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. D2 rejudge 2026-10-16 (independent judge, BGP-LS codec moved to internal/core/bgp/nlri/bgpls). Cited producer originateAttributes changed only by ls.WriteTLVBytes -> bgpls.WriteTLVBytes. RFC 7752 Section 3.3.2.7: "The use of FQDN or a subset of it is strongly RECOMMENDED." No native naming policy exists: originateAttributes copies supplied TLV values (sorted, reserved-cleared) and decodeLinkName decodes a received string. Optional implementation gap, not a rejection duty.

No test carries RFC7752-3.3.2.7-1, so there is no linked test.

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `2.2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 3 | reviewed | not stated |
| `3.2` | not stated | 6 | reviewed | not stated |
| `3.2.1` | not stated | 1 | reviewed | not stated |
| `3.2.1.1` | not stated | 2 | reviewed | not stated |
| `3.2.1.2` | not stated | 0 | reviewed | not stated |
| `3.2.1.3` | not stated | 0 | reviewed | not stated |
| `3.2.1.4` | not stated | 3 | reviewed | not stated |
| `3.2.1.5` | not stated | 1 | reviewed | not stated |
| `3.2.2` | not stated | 0 | reviewed | not stated |
| `3.2.3` | not stated | 0 | reviewed | not stated |
| `3.2.3.1` | not stated | 0 | reviewed | not stated |
| `3.2.3.2` | not stated | 0 | reviewed | not stated |
| `3.3` | not stated | 1 | reviewed | not stated |
| `3.3.1` | not stated | 0 | reviewed | not stated |
| `3.3.1.1` | not stated | 0 | reviewed | not stated |
| `3.3.1.2` | not stated | 0 | reviewed | not stated |
| `3.3.1.3` | not stated | 0 | reviewed | not stated |
| `3.3.1.4` | not stated | 0 | reviewed | not stated |
| `3.3.1.5` | not stated | 0 | reviewed | not stated |
| `3.3.2` | not stated | 0 | reviewed | not stated |
| `3.3.2.1` | not stated | 1 | reviewed | not stated |
| `3.3.2.2` | not stated | 1 | reviewed | not stated |
| `3.3.2.3` | not stated | 1 | reviewed | not stated |
| `3.3.2.4` | not stated | 0 | reviewed | not stated |
| `3.3.2.5` | not stated | 0 | reviewed | not stated |
| `3.3.2.6` | not stated | 0 | reviewed | not stated |
| `3.3.2.7` | not stated | 0 | reviewed | not stated |
| `3.3.3` | not stated | 1 | reviewed | not stated |
| `3.3.3.1` | not stated | 0 | reviewed | not stated |
| `3.3.3.2` | not stated | 0 | reviewed | not stated |
| `3.3.3.3` | not stated | 0 | reviewed | not stated |
| `3.3.3.4` | not stated | 0 | reviewed | not stated |
| `3.3.3.5` | not stated | 0 | reviewed | not stated |
| `3.3.3.6` | not stated | 0 | reviewed | not stated |
| `3.4` | not stated | 1 | reviewed | not stated |
| `3.5` | not stated | 0 | reviewed | not stated |
| `3.6` | not stated | 0 | reviewed | not stated |
| `3.7` | not stated | 0 | reviewed | not stated |
| `3.8` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 0 | reviewed | not stated |
| `4.2` | not stated | 0 | reviewed | not stated |
| `4.3` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `5.1` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 0 | reviewed | not stated |
| `6.1.1` | not stated | 0 | reviewed | not stated |
| `6.1.2` | not stated | 0 | reviewed | not stated |
| `6.1.3` | not stated | 0 | reviewed | not stated |
| `6.1.4` | not stated | 0 | reviewed | not stated |
| `6.1.5` | not stated | 0 | reviewed | not stated |
| `6.1.6` | not stated | 0 | reviewed | not stated |
| `6.2` | not stated | 0 | reviewed | not stated |
| `6.2.1` | not stated | 0 | reviewed | not stated |
| `6.2.2` | not stated | 2 | reviewed | not stated |
| `6.2.3` | not stated | 0 | reviewed | not stated |
| `6.2.4` | not stated | 0 | reviewed | not stated |
| `6.2.5` | not stated | 0 | reviewed | not stated |
| `6.2.6` | not stated | 1 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 1 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 0 | reviewed | not stated |
| `9.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 7752 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

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
