# RFC 9085 - Border Gateway Protocol - Link State (BGP-LS) Extensions for Segment Routing

BGP-LS Segment Routing extensions. Every requirement listed for RFC 9085, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 9 | of 9 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 9 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 9 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 9 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 9 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 56 | 56 of 57 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |
| Audit verdicts | 12 | of 9 mandatory requirements reviewed | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 9 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 9 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 9 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 9 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 9 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 9 | of 14 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 9 mandatory requirements: they add up to 9. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | BGP-LS Segment Routing extensions |
| Automatic checks | Included |
| Requirements | 14 |
| Mandatory requirements (MUST level) | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 57 |
| Tests linked to requirements | 57 |
| Recorded audit verdicts | 12 |
| Fault test records | 56 |
| Summary | `rfc/short/rfc9085.md` |
| Requirement table | `rfc/requirements/rfc9085.md` |
| RFC text | `rfc/full/rfc9085.txt` |

## Automatic checks

Included in automatic checks: BGP-LS Segment Routing extensions: Ze decodes the SR TLVs and originates them from its IS-IS and OSPF link-state databases. Correction 2026-09-30: the eight transmit rows were recorded as gaps on the belief that no path originated them; the native IS-IS and OSPF BGP-LS exporters do, so those rows now carry origination tests. The 2026-09-21 extraction walk lowered RFC9085-2.1-1 to SHOULD, the level the document's own sentence uses, and on 2026-09-30 the duplicate rows RFC9085-2.1.2-3 and 2.1.2-4 were merged into RFC9085-2.1.2-2 and 2.1.2-1, so the MUST count is 9 rather than 12.

## Published support status

**Area:** BGP-LS Segment Routing extensions

**Test verification:** 9 of 9 MUST proven

**Evidence:** unit 9

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 9 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **9** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (9):** [`RFC9085-2.1.2-1`](#rfc9085-2.1.2-1), [`RFC9085-2.1.2-2`](#rfc9085-2.1.2-2), [`RFC9085-2.1.4-1`](#rfc9085-2.1.4-1), [`RFC9085-2.1.4-2`](#rfc9085-2.1.4-2), [`RFC9085-2.2.1-1`](#rfc9085-2.2.1-1), [`RFC9085-2.2.2-1`](#rfc9085-2.2.2-1), [`RFC9085-2.3.1-1`](#rfc9085-2.3.1-1), [`RFC9085-2.3.5-1`](#rfc9085-2.3.5-1), [`RFC9085-2.1.1-1`](#rfc9085-2.1.1-1)

## Figures

- **MUST bar**: Ze verified 9
- **Overview bar**: Ze MUST verified 9, SHOULD verified 3, Not assessed 2

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 9 |
| MUST rows kept (the MUST bar) | 9 |
| Ze verified | 9 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 5 |
| SHOULD verified | 3 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 2 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc9085.json](../../../data/rfc/rfc9085.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9085-2.1.2-1` | The flags are not currently defined for OSPFv2 and OSPFv3 and MUST be set to 0 and ignored on receipt. (§2.1.2) | MUST | 2.1.2 | **positive:** `unit/verify` [`TestRFC9085CapabilitiesReceiptIgnoresFlagsAndReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9085_capabilities_receipt_test.go#L42). **positive:** `unit/verify` [`TestRFC9085OSPFOriginatedCapabilitiesFlagsReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L78). **positive:** `unit/verify` [`TestRFC9085SRCapabilitiesIgnoresReservedAndUndefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L933). **negative:** `unit/verify` [`TestRFC9085OSPFSourceReservedNeverOriginated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L96) |
| `RFC9085-2.1.2-2` | Reserved: 1 octet that MUST be set to 0 and ignored on receipt. (§2.1.2) | MUST | 2.1.2 | **positive:** `unit/verify` [`TestRFC9085CapabilitiesReceiptIgnoresFlagsAndReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9085_capabilities_receipt_test.go#L43). **positive:** `unit/verify` [`TestRFC9085ISISOriginatedCapabilitiesReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_test.go#L78). **positive:** `unit/verify` [`TestRFC9085OSPFOriginatedCapabilitiesFlagsReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L79). **positive:** `unit/verify` [`TestRFC9085SRCapabilitiesIgnoresReservedAndUndefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L943). **negative:** `unit/verify` [`TestRFC9085ISISAllOnesSourceNeverReachesReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_test.go#L96). **negative:** `unit/verify` [`TestRFC9085OSPFSourceReservedNeverOriginated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L97) |
| `RFC9085-2.1.4-1` | The flags are not currently defined for OSPFv2 and OSPFv3 and MUST be set to 0 and ignored on receipt. (§2.1.4) | MUST | 2.1.4 | **positive:** `unit/verify` [`TestRFC9085CapabilitiesReceiptIgnoresFlagsAndReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9085_capabilities_receipt_test.go#L44). **positive:** `unit/verify` [`TestRFC9085OSPFOriginatedCapabilitiesFlagsReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L80). **negative:** `unit/verify` [`TestRFC9085OSPFSourceReservedNeverOriginated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L98) |
| `RFC9085-2.1.4-2` | Reserved: 1 octet that MUST be set to 0 and ignored on receipt. (§2.1.4) | MUST | 2.1.4 | **positive:** `unit/verify` [`TestRFC9085CapabilitiesReceiptIgnoresFlagsAndReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9085_capabilities_receipt_test.go#L45). **positive:** `unit/verify` [`TestRFC9085ISISOriginatedCapabilitiesReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_test.go#L79). **positive:** `unit/verify` [`TestRFC9085OSPFOriginatedCapabilitiesFlagsReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L81). **negative:** `unit/verify` [`TestRFC9085ISISAllOnesSourceNeverReachesReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_test.go#L97). **negative:** `unit/verify` [`TestRFC9085OSPFSourceReservedNeverOriginated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L99) |
| `RFC9085-2.2.1-1` | Reserved: 2 octets that MUST be set to 0 and ignored on receipt. (§2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestRFC9085ISISOriginatedSIDReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_sid_test.go#L97). **positive:** `unit/verify` [`TestRFC9085OSPFOriginatedSIDReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L245). **positive:** `unit/verify` [`TestRFC9085SIDReceiptIgnoresReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9085_capabilities_receipt_test.go#L72). **negative:** `unit/verify` [`TestRFC9085ISISAllOnesSourceNeverReachesSIDReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_sid_test.go#L119). **negative:** `unit/verify` [`TestRFC9085OSPFSourceReservedNeverReachesSIDReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L259) |
| `RFC9085-2.2.2-1` | Reserved: 2 octets that MUST be set to 0 and ignored on receipt. (§2.2.2) | MUST | 2.2.2 | **positive:** `unit/verify` [`TestRFC9085ISISOriginatedSIDReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_sid_test.go#L98). **positive:** `unit/verify` [`TestRFC9085OSPFOriginatedSIDReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L246). **positive:** `unit/verify` [`TestRFC9085UndecodedSIDReceiptKeepsReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9085_capabilities_receipt_test.go#L108). **negative:** `unit/verify` [`TestRFC9085ISISAllOnesSourceNeverReachesSIDReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_sid_test.go#L120). **negative:** `unit/verify` [`TestRFC9085OSPFSourceReservedNeverReachesSIDReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L260) |
| `RFC9085-2.3.1-1` | Reserved: 2 octets that MUST be set to 0 and ignored on receipt. (§2.3.1) | MUST | 2.3.1 | **positive:** `unit/verify` [`TestRFC9085ISISOriginatedSIDReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_sid_test.go#L99). **positive:** `unit/verify` [`TestRFC9085OSPFOriginatedSIDReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L247). **positive:** `unit/verify` [`TestRFC9085SIDReceiptIgnoresReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9085_capabilities_receipt_test.go#L73). **negative:** `unit/verify` [`TestRFC9085ISISAllOnesSourceNeverReachesSIDReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_sid_test.go#L121). **negative:** `unit/verify` [`TestRFC9085OSPFSourceReservedNeverReachesSIDReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L261) |
| `RFC9085-2.3.5-1` | Reserved: 1 octet that MUST be set to 0 and ignored on receipt. (§2.3.5) | MUST | 2.3.5 | **positive:** `unit/verify` [`TestRFC9085ISISOriginatedSIDReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_sid_test.go#L100). **positive:** `unit/verify` [`TestRFC9085OSPFOriginatedSIDReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L248). **positive:** `unit/verify` [`TestRFC9085UndecodedSIDReceiptKeepsReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9085_capabilities_receipt_test.go#L109). **negative:** `unit/verify` [`TestRFC9085ISISAllOnesSourceNeverReachesSIDReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_sid_test.go#L122). **negative:** `unit/verify` [`TestRFC9085OSPFSourceReservedNeverReachesSIDReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L262) |
| `RFC9085-2.1.1-1` | SID/Label: If the length is set to 3, then the 20 rightmost bits represent a label (the total TLV size is 7), and the 4 leftmost bits are set to 0. (§2.1.1) | MUST | 2.1.1 | **positive:** `unit/verify` [`TestRFC9085OriginatedSRGBLabelTwentyBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_test.go#L112). **positive:** `unit/verify` [`TestRFC9085SIDLabelMasksLeftmostFourBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L891). **negative:** `unit/verify` [`TestRFC9085OriginatedSRGBLabelHighBitsCleared`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_test.go#L125) |
| `RFC9085-2.1-1` | These TLVs should only be added to the BGP-LS Attribute associated with the Node NLRI that describes the IGP node that is originating the corresponding IGP TLV/sub-TLV described below. (§2.1) | SHOULD | 2.1 | **positive:** `unit/verify` [`TestRFC9085ISISCapabilitiesOnOriginatorNode`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_placement_test.go#L64). **positive:** `unit/verify` [`TestRFC9085OSPFCapabilitiesOnOriginatorNode`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L271). **negative:** `unit/verify` [`TestRFC9085ISISCapabilitiesOnNoOtherNLRI`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_placement_test.go#L89). **negative:** `unit/verify` [`TestRFC9085OSPFCapabilitiesOnNoOtherNLRI`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L286) |
| `RFC9085-2.2-1` | These TLVs should only be added to the BGP-LS Attribute associated with the Link NLRI that describes the link of the IGP node that is originating the corresponding IGP TLV/sub-TLV described below. (§2.2) | SHOULD | 2.2 | **positive:** `unit/verify` [`TestRFC9085ISISLinkPlacement`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_object_placement_test.go#L22). **positive:** `unit/verify` [`TestRFC9085NativeExportAttributePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9085_export_placement_test.go#L26). **positive:** `unit/verify` [`TestRFC9085OSPFLinkPlacement`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_object_placement_test.go#L23). **negative:** `unit/verify` [`TestRFC9085ISISLinkPlacement`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_object_placement_test.go#L23). **negative:** `unit/verify` [`TestRFC9085NativeExportAttributePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9085_export_placement_test.go#L27). **negative:** `unit/verify` [`TestRFC9085OSPFLinkPlacement`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_object_placement_test.go#L24) |
| `RFC9085-2.3-1` | These TLVs should only be added to the BGP-LS Attribute associated with the Prefix NLRI that describes the prefix of the IGP node that is originating the corresponding IGP TLV/sub-TLV described below. (§2.3) | SHOULD | 2.3 | **positive:** `unit/verify` [`TestRFC9085ISISPrefixPlacement`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_object_placement_test.go#L101). **positive:** `unit/verify` [`TestRFC9085NativeExportAttributePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9085_export_placement_test.go#L28). **positive:** `unit/verify` [`TestRFC9085OSPFPrefixPlacement`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_object_placement_test.go#L102). **negative:** `unit/verify` [`TestRFC9085ISISPrefixPlacement`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_object_placement_test.go#L102). **negative:** `unit/verify` [`TestRFC9085NativeExportAttributePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9085_export_placement_test.go#L29). **negative:** `unit/verify` [`TestRFC9085OSPFPrefixPlacement`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_object_placement_test.go#L103) |
| `RFC9085-2.2.3-1` | The TLV MAY include sub-TLVs that describe attributes associated with the bundle member. (§2.2.3) | MAY | 2.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9085-2.2.3-2` | Multiple L2 Bundle Member Attributes TLVs MAY be associated with a Link NLRI (S2.2.3) | MAY | 2.2.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 9085 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC9085-2.1.2-1`](#rfc9085-2.1.2-1)

The flags are not currently defined for OSPFv2 and OSPFv3 and MUST be set to 0 and ignored on receipt. (§2.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. D2 rejudge 2026-10-16 (independent judge, BGP-LS codec moved to internal/core/bgp/nlri/bgpls). Tagged units changed only by the package move: identifiers re-qualified with bgpls., comments re-pointed; producer bodies byte-identical modulo the writeTLV->WriteTLV and field-export renames (TLV code constants verified unchanged). The receipt units now reach bgpls.DecodeSRCapabilities through the TLV 1034 decoder the plugin registers (lookupLsAttrTLVDecoder), so the adapter is exercised too; DecodeSRCapabilities is byte-identical to the old decodeSRCapabilities (len<2 refused, Flags stored, ranges from data[2:]). RFC 9085 Section 2.1.2: "The flags are not currently defined for OSPFv2 and OSPFv3 and MUST be set to 0 and ignored on receipt." Set-0: ospf TestRFC9085OSPFOriginatedCapabilitiesFlagsReservedZero / TestRFC9085OSPFSourceReservedNeverOriginated (unchanged). Ignored: Flags ff and 0x82 accepted, range 1000@16000 intact, I flag intact; single-polarity by nature on receipt.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9085OSPFSourceReservedNeverOriginated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L96) | unit/verify | revert, verified |
| positive | [`TestRFC9085SRCapabilitiesIgnoresReservedAndUndefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L933) | unit/verify | revert, verified |
| positive | [`TestRFC9085CapabilitiesReceiptIgnoresFlagsAndReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9085_capabilities_receipt_test.go#L42) | unit/verify | revert, verified |
| positive | [`TestRFC9085OSPFOriginatedCapabilitiesFlagsReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L78) | unit/verify | revert, verified |

### [`RFC9085-2.1.2-2`](#rfc9085-2.1.2-2)

Reserved: 1 octet that MUST be set to 0 and ignored on receipt. (§2.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. D2 rejudge 2026-10-16 (independent judge, BGP-LS codec moved to internal/core/bgp/nlri/bgpls). Tagged units changed only by the package move: identifiers re-qualified with bgpls., comments re-pointed; producer bodies byte-identical modulo the writeTLV->WriteTLV and field-export renames (TLV code constants verified unchanged). The receipt units now reach bgpls.DecodeSRCapabilities through the TLV 1034 decoder the plugin registers (lookupLsAttrTLVDecoder), so the adapter is exercised too; DecodeSRCapabilities is byte-identical to the old decodeSRCapabilities (len<2 refused, Flags stored, ranges from data[2:]). RFC 9085 Section 2.1.2: "Reserved: 1 octet that MUST be set to 0 and ignored on receipt." Set-0: ospf and isis originated-zero / poisoned-source pairs (unchanged). Ignored: Reserved ff skipped, not read as range data (Range 1000, FirstSID 16000 asserted exactly).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9085ISISAllOnesSourceNeverReachesReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_test.go#L96) | unit/verify | revert, verified |
| negative | [`TestRFC9085OSPFSourceReservedNeverOriginated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L97) | unit/verify | revert, verified |
| positive | [`TestRFC9085SRCapabilitiesIgnoresReservedAndUndefinedFlags`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L943) | unit/verify | revert, verified |
| positive | [`TestRFC9085CapabilitiesReceiptIgnoresFlagsAndReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9085_capabilities_receipt_test.go#L43) | unit/verify | revert, verified |
| positive | [`TestRFC9085ISISOriginatedCapabilitiesReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_test.go#L78) | unit/verify | revert, verified |
| positive | [`TestRFC9085OSPFOriginatedCapabilitiesFlagsReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L79) | unit/verify | revert, verified |

### [`RFC9085-2.1.4-1`](#rfc9085-2.1.4-1)

The flags are not currently defined for OSPFv2 and OSPFv3 and MUST be set to 0 and ignored on receipt. (§2.1.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. D2 rejudge 2026-10-16 (independent judge, BGP-LS codec moved to internal/core/bgp/nlri/bgpls). Tagged units changed only by the package move: identifiers re-qualified with bgpls., comments re-pointed; producer bodies byte-identical modulo the writeTLV->WriteTLV and field-export renames (TLV code constants verified unchanged). The receipt units now reach bgpls.DecodeSRCapabilities through the TLV 1034 decoder the plugin registers (lookupLsAttrTLVDecoder), so the adapter is exercised too; DecodeSRCapabilities is byte-identical to the old decodeSRCapabilities (len<2 refused, Flags stored, ranges from data[2:]). RFC 9085 Section 2.1.4: "The flags are not currently defined for OSPFv2 and OSPFv3 and MUST be set to 0 and ignored on receipt." Set-0: ospf SRLB TLV 1036 pair (unchanged). Receipt: decodeSRLocalBlock (unchanged body) decodes Flags ff to the intact range.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9085OSPFSourceReservedNeverOriginated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L98) | unit/verify | revert, verified |
| positive | [`TestRFC9085CapabilitiesReceiptIgnoresFlagsAndReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9085_capabilities_receipt_test.go#L44) | unit/verify | revert, verified |
| positive | [`TestRFC9085OSPFOriginatedCapabilitiesFlagsReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L80) | unit/verify | revert, verified |

### [`RFC9085-2.1.4-2`](#rfc9085-2.1.4-2)

Reserved: 1 octet that MUST be set to 0 and ignored on receipt. (§2.1.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. D2 rejudge 2026-10-16 (independent judge, BGP-LS codec moved to internal/core/bgp/nlri/bgpls). Tagged units changed only by the package move: identifiers re-qualified with bgpls., comments re-pointed; producer bodies byte-identical modulo the writeTLV->WriteTLV and field-export renames (TLV code constants verified unchanged). The receipt units now reach bgpls.DecodeSRCapabilities through the TLV 1034 decoder the plugin registers (lookupLsAttrTLVDecoder), so the adapter is exercised too; DecodeSRCapabilities is byte-identical to the old decodeSRCapabilities (len<2 refused, Flags stored, ranges from data[2:]). RFC 9085 Section 2.1.4: "Reserved: 1 octet that MUST be set to 0 and ignored on receipt." Set-0: isis and ospf TLV 1036 pairs (unchanged). Receipt: Reserved ff on TLV 1036 decodes to the intact range.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9085ISISAllOnesSourceNeverReachesReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_test.go#L97) | unit/verify | revert, verified |
| negative | [`TestRFC9085OSPFSourceReservedNeverOriginated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L99) | unit/verify | revert, verified |
| positive | [`TestRFC9085CapabilitiesReceiptIgnoresFlagsAndReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9085_capabilities_receipt_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestRFC9085ISISOriginatedCapabilitiesReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_test.go#L79) | unit/verify | revert, verified |
| positive | [`TestRFC9085OSPFOriginatedCapabilitiesFlagsReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L81) | unit/verify | revert, verified |

### [`RFC9085-2.2.1-1`](#rfc9085-2.2.1-1)

Reserved: 2 octets that MUST be set to 0 and ignored on receipt. (§2.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rejudge 2026-09-30 (independent): owed OSPF half delivered. Set-0: isis TestRFC9085ISISOriginatedSIDReservedZero (+) / TestRFC9085ISISAllOnesSourceNeverReachesSIDReserved (-); ospf TestRFC9085OSPFOriginatedSIDReservedZero (+, exact 1099 = 60 05 0000 005dc1) / TestRFC9085OSPFSourceReservedNeverReachesSIDReserved (-, native Adj-SID Reserved ff -> 0000, byte-exact); producer bgplsAdjAttribute zero-fills value[2:4]. Receipt: nlri/ls TestRFC9085SIDReceiptIgnoresReserved (Reserved ffff decodes). Records observed red for every unit.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9085ISISAllOnesSourceNeverReachesSIDReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_sid_test.go#L119) | unit/verify | revert, verified |
| negative | [`TestRFC9085OSPFSourceReservedNeverReachesSIDReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L259) | unit/verify | revert, verified |
| positive | [`TestRFC9085SIDReceiptIgnoresReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9085_capabilities_receipt_test.go#L72) | unit/verify | revert, verified |
| positive | [`TestRFC9085ISISOriginatedSIDReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_sid_test.go#L97) | unit/verify | revert, verified |
| positive | [`TestRFC9085OSPFOriginatedSIDReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L245) | unit/verify | revert, verified |

### [`RFC9085-2.2.2-1`](#rfc9085-2.2.2-1)

Reserved: 2 octets that MUST be set to 0 and ignored on receipt. (§2.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rejudge 2026-09-30 (independent): both owed items delivered. Set-0: isis SID units (+/-); ospf TestRFC9085OSPFOriginatedSIDReservedZero (+, exact 1100 = 60 06 0000 04040404 005dc2) / TestRFC9085OSPFSourceReservedNeverReachesSIDReserved (-, native LAN Adj-SID Reserved ff -> 0000). Receipt: nlri/ls TestRFC9085UndecodedSIDReceiptKeepsReserved: a 1100 with Reserved ffff is kept whole as generic-lsid-1100 and the Node Name after it still decodes; this decoder carries 1100 undecoded, so ignoring = not refusing and not interpreting, single-polarity by nature. Records observed red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9085ISISAllOnesSourceNeverReachesSIDReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_sid_test.go#L120) | unit/verify | revert, verified |
| negative | [`TestRFC9085OSPFSourceReservedNeverReachesSIDReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L260) | unit/verify | revert, verified |
| positive | [`TestRFC9085UndecodedSIDReceiptKeepsReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9085_capabilities_receipt_test.go#L108) | unit/verify | revert, verified |
| positive | [`TestRFC9085ISISOriginatedSIDReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_sid_test.go#L98) | unit/verify | revert, verified |
| positive | [`TestRFC9085OSPFOriginatedSIDReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L246) | unit/verify | revert, verified |

### [`RFC9085-2.3.1-1`](#rfc9085-2.3.1-1)

Reserved: 2 octets that MUST be set to 0 and ignored on receipt. (§2.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rejudge 2026-09-30 (independent): owed OSPF half delivered. Set-0: isis SID units (+/-); ospf TestRFC9085OSPFOriginatedSIDReservedZero (+, exact 1158 = 40 01 0000 0000004d) / TestRFC9085OSPFSourceReservedNeverReachesSIDReserved (-, native Prefix-SID Reserved ff -> 0000); the 1158 nested in the OSPF 1159 is also asserted byte-exact. Receipt: nlri/ls TestRFC9085SIDReceiptIgnoresReserved (decodePrefixSID, Reserved ffff). Records observed red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9085ISISAllOnesSourceNeverReachesSIDReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_sid_test.go#L121) | unit/verify | revert, verified |
| negative | [`TestRFC9085OSPFSourceReservedNeverReachesSIDReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L261) | unit/verify | revert, verified |
| positive | [`TestRFC9085SIDReceiptIgnoresReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9085_capabilities_receipt_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestRFC9085ISISOriginatedSIDReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_sid_test.go#L99) | unit/verify | revert, verified |
| positive | [`TestRFC9085OSPFOriginatedSIDReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L247) | unit/verify | revert, verified |

### [`RFC9085-2.3.5-1`](#rfc9085-2.3.5-1)

Reserved: 1 octet that MUST be set to 0 and ignored on receipt. (§2.3.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rejudge 2026-09-30 (independent): both owed items delivered. Set-0: isis SID units (+/-); ospf TestRFC9085OSPFOriginatedSIDReservedZero (+, exact 1159 = 80 00 0010 0486 0008 40000000 00000064) / TestRFC9085OSPFSourceReservedNeverReachesSIDReserved (-, native Range Reserved ff ff ff and inner Prefix-SID Reserved ff); judge targeted break (1159 Reserved <- native r.Value[5] in extendedV2Prefix) turned the negative red while the positive stayed green. Receipt: nlri/ls TestRFC9085UndecodedSIDReceiptKeepsReserved (1159 with Reserved ff kept whole as generic-lsid-1159, attribute not refused), single-polarity by nature. Records observed red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9085ISISAllOnesSourceNeverReachesSIDReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_sid_test.go#L122) | unit/verify | revert, verified |
| negative | [`TestRFC9085OSPFSourceReservedNeverReachesSIDReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L262) | unit/verify | revert, verified |
| positive | [`TestRFC9085UndecodedSIDReceiptKeepsReserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9085_capabilities_receipt_test.go#L109) | unit/verify | revert, verified |
| positive | [`TestRFC9085ISISOriginatedSIDReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_sid_test.go#L100) | unit/verify | revert, verified |
| positive | [`TestRFC9085OSPFOriginatedSIDReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L248) | unit/verify | revert, verified |

### [`RFC9085-2.1.1-1`](#rfc9085-2.1.1-1)

SID/Label: If the length is set to 3, then the 20 rightmost bits represent a label (the total TLV size is 7), and the 4 leftmost bits are set to 0. (§2.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rejudge 2026-09-30: defect fixed (D-8): isis srBlock copied the 3-octet SID/Label verbatim into sub-TLV 1161; it now clears the 4 leftmost bits. isis TestRFC9085OriginatedSRGBLabelTwentyBits (+: 003e80 -> 003e80) and TestRFC9085OriginatedSRGBLabelHighBitsCleared (-: f03e80 -> 003e80) assert the exact TLV 1034; judge break (mask removed) turns the negative red. Receipt: nlri/ls TestRFC9085SIDLabelMasksLeftmostFourBits (0xF12345 -> 0x12345). Marker removed.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9085OriginatedSRGBLabelHighBitsCleared`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_test.go#L125) | unit/verify | revert, verified |
| positive | [`TestRFC9085SIDLabelMasksLeftmostFourBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L891) | unit/verify | not verified |
| positive | [`TestRFC9085OriginatedSRGBLabelTwentyBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_test.go#L112) | unit/verify | revert, verified |

### [`RFC9085-2.1-1`](#rfc9085-2.1-1)

These TLVs should only be added to the BGP-LS Attribute associated with the Node NLRI that describes the IGP node that is originating the corresponding IGP TLV/sub-TLV described below. (§2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (independent judge, ospf c28). OSPF exporter as judged 2026-09-30 (+ TestRFC9085OSPFCapabilitiesOnOriginatorNode, - TestRFC9085OSPFCapabilitiesOnNoOtherNLRI, targeted breaks red). IS-IS exporter now driven (new isis/bgpls_export_rfc9085_placement_test.go): two L1 LSPs through the LSDB and bgplsBuilder.build; + TestRFC9085ISISCapabilitiesOnOriginatorNode: 0000.0000.0002 (TLV 242 with sub-TLVs 2/19/22) gets exactly one 1034/1035/1036 with exact bytes; - TestRFC9085ISISCapabilitiesOnNoOtherNLRI: 0000.0000.0003 Node NLRI, both Link and both Prefix NLRIs (presence asserted) carry none. Judge overlay: TLV 242 deferred and applied to the last node built (0000.0000.0003) -> + red (0 TLV 1034) and - red ('node 0000.0000.0003 carries TLV 1034'); author overlays (copy to other node / to every Link and Prefix NLRI) -> - red only. Revert records on capabilities observed. Support remaining sentence 'tested on the OSPF exporter only' removed as now false.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9085ISISCapabilitiesOnNoOtherNLRI`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_placement_test.go#L89) | unit/verify | revert, verified |
| negative | [`TestRFC9085OSPFCapabilitiesOnNoOtherNLRI`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L286) | unit/verify | revert, verified |
| positive | [`TestRFC9085ISISCapabilitiesOnOriginatorNode`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_export_placement_test.go#L64) | unit/verify | revert, verified |
| positive | [`TestRFC9085OSPFCapabilitiesOnOriginatorNode`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_export_test.go#L271) | unit/verify | revert, verified |

### [`RFC9085-2.2-1`](#rfc9085-2.2-1)

These TLVs should only be added to the BGP-LS Attribute associated with the Link NLRI that describes the link of the IGP node that is originating the corresponding IGP TLV/sub-TLV described below. (§2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-12 (independent auditor). STALE from 96ec49327b (owner-approved lint only): TestRFC9085NativeExportAttributePlacement replaces counted byte loops with range byte(3) (same 0,1,2), an if/else-if chain on kind with a tagless switch carrying the same arms and default Fatalf, and a dead route := 0 initialisation with a declaration where it is assigned. No input or assertion changed. Verdict stands. RFC 9085 §2.2 says, “These TLVs should only be added to the BGP-LS Attribute associated with the Link NLRI that describes the link of the IGP node that is originating the corresponding IGP TLV/sub-TLV described below.” Native IS-IS LSP and OSPFv2/v3 LSDB tests establish association of emitted 1099/1100 with the advertising router and correct link, including parallel links and tested topology identities, while populated Node/Prefix controls exclude link attributes. Separate replace/reconcile/export tests preserve that association into Link NLRIs, assert complete expected interface-ID/address descriptors, exact SID values and multiplicity, and reject cross-object placement. This proves the conditional placement duty for the current native producers, not implementation of all §2.2 features: native IS-IS 1172 remains absent, and RFC 9085 defines no corresponding OSPF bundle feature. It is composed native-derivation and exporter-association evidence, not a single integrated IGP-daemon-to-peer run. Independent source judgment: BGPLSPlacementWholeReview. Current five-root race-enabled observation: job-bgpls-complete-object-identity-after-9a6e8f72.log.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9085NativeExportAttributePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9085_export_placement_test.go#L27) | unit/verify | revert, verified |
| negative | [`TestRFC9085ISISLinkPlacement`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_object_placement_test.go#L23) | unit/verify | revert, verified |
| negative | [`TestRFC9085OSPFLinkPlacement`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_object_placement_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestRFC9085NativeExportAttributePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9085_export_placement_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestRFC9085ISISLinkPlacement`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_object_placement_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestRFC9085OSPFLinkPlacement`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_object_placement_test.go#L23) | unit/verify | revert, verified |

### [`RFC9085-2.3-1`](#rfc9085-2.3-1)

These TLVs should only be added to the BGP-LS Attribute associated with the Prefix NLRI that describes the prefix of the IGP node that is originating the corresponding IGP TLV/sub-TLV described below. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-12 (independent auditor). STALE from 96ec49327b (owner-approved lint only): TestRFC9085NativeExportAttributePlacement replaces counted byte loops with range byte(3) (same 0,1,2), an if/else-if chain on kind with a tagless switch carrying the same arms and default Fatalf, and a dead route := 0 initialisation with a declaration where it is assigned. No input or assertion changed. Verdict stands. RFC 9085 §2.3 says, “These TLVs should only be added to the BGP-LS Attribute associated with the Prefix NLRI that describes the prefix of the IGP node that is originating the corresponding IGP TLV/sub-TLV described below.” Native IS-IS tests establish placement of 1158/1159/1170/1171 from IPv4/IPv6 reachability, bindings and narrow-external input; native OSPFv2/v3 tests establish placement of 1158/1159/1170, including extended prefix classes and base-v3 flags. Exact values, object counts and populated Node/Link exclusions discriminate incorrect association. Separate replace/reconcile/export tests preserve origin, prefix address/length, IPv4/IPv6 NLRI type and tested topology identities; competing same-prefix OSPF classes 1/2/3/5 now retain exact descriptor 264 and their distinct SID values. This proves the conditional placement duty for current native producers, not absent OSPF 1171/1174 support; 1174 is OSPF-only, not an IS-IS gap. No integrated daemon run or received-attribute semantic validation is claimed. Independent source judgment: BGPLSPlacementWholeReview. Current five-root race-enabled observation: job-bgpls-complete-object-identity-after-9a6e8f72.log.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9085NativeExportAttributePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9085_export_placement_test.go#L29) | unit/verify | revert, verified |
| negative | [`TestRFC9085ISISPrefixPlacement`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_object_placement_test.go#L102) | unit/verify | revert, verified |
| negative | [`TestRFC9085OSPFPrefixPlacement`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_object_placement_test.go#L103) | unit/verify | revert, verified |
| positive | [`TestRFC9085NativeExportAttributePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9085_export_placement_test.go#L28) | unit/verify | revert, verified |
| positive | [`TestRFC9085ISISPrefixPlacement`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9085_bgpls_object_placement_test.go#L101) | unit/verify | revert, verified |
| positive | [`TestRFC9085OSPFPrefixPlacement`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc9085_bgpls_object_placement_test.go#L102) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc9085.txt |
| Source fingerprint | fdec3413ebb35a14 |
| Record | rfc/extraction/rfc9085.json |
| Mapped sentences | 8 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `2.1.1` | not stated | 0 | reviewed | not stated |
| `2.1.2` | not stated | 2 | reviewed | not stated |
| `2.1.3` | not stated | 0 | reviewed | not stated |
| `2.1.4` | not stated | 3 | reviewed | not stated |
| `2.1.5` | not stated | 0 | reviewed | not stated |
| `2.2` | not stated | 0 | reviewed | The lowercase placement recommendation covers all Link Attribute TLVs in Table 2: 1099, 1100 and 1172, associated with the originating node's specific Link NLRI. Recording it does not establish exporter proof. |
| `2.2.1` | not stated | 1 | reviewed | not stated |
| `2.2.2` | not stated | 1 | reviewed | not stated |
| `2.2.3` | not stated | 0 | reviewed | not stated |
| `2.3` | not stated | 0 | reviewed | The lowercase placement recommendation covers all Prefix Attribute TLVs in Table 4: 1158, 1159, 1170, 1171 and 1174, associated with the originating node's specific Prefix NLRI. Recording it does not establish exporter proof. |
| `2.3.1` | not stated | 1 | reviewed | not stated |
| `2.3.2` | not stated | 0 | reviewed | not stated |
| `2.3.3` | not stated | 0 | reviewed | not stated |
| `2.3.4` | not stated | 0 | reviewed | not stated |
| `2.3.5` | not stated | 1 | reviewed | not stated |
| `2.4` | not stated | 0 | reviewed | not stated |
| `2.5` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 1 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 0 | reviewed | not stated |
| `6.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate from the Copyright Notice: it states the licence terms under which code components extracted from the document are provided ('must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions'). It binds a republisher of the text, not a BGP-LS implementation, and names no message, field or procedure. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `2.1.4:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation to advertise the SRLB belongs to the IGP documents the same section cites three lines later: 'This information is derived from the protocol-specific advertisements. * IS-IS, as defined by the SRLB Sub-TLV in Section 3.3 of [RFC8667]. * OSPFv2/OSPFv3, as defined by the SR Local Block TLV in Section 3.3 of [RFC8665] and [RFC8666]'. RFC 9085 defines only how BGP-LS carries the range once the IGP has advertised it; the sentence explains why the IGP advertisement exists. | Therefore, in order for such applications or controllers to know the range of local SIDs available, the node is required to advertise its SRLB. |
| `5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A Security Considerations ASSUMPTION rather than an obligation: 'The IGP instances originating these TLVs are assumed to support all the required security and authentication mechanisms (as described in [RFC8665], [RFC8666], and [RFC8667])'. It states what this document takes for granted about the IGP that feeds BGP-LS, and the mechanisms it names are required by those three documents, not by this one. | The IGP instances originating these TLVs are assumed to support all the required security and authentication mechanisms (as described in [RFC8665], [RFC8666], and [RFC8667]) in order to prevent any security issue when propagating the TLVs into BGP-LS. |

## Superseded

No document obsoletes RFC 9085, so its obligations are stated where they were written.
