# RFC 8362 - OSPFv3 Link State Advertisement (LSA) Extensibility

OSPFv3 Extended LSAs. Every requirement listed for RFC 8362, the tests linked to it, and their recorded reviews. This summary is excluded from automatic compliance checks.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 3 | of 38 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 38 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 38 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 38 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 9 | 9 of 9 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 34 | of 38 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 38 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 38 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 38 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 38 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 38 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level), not checked | 38 | of 50 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements listed in this summary. This RFC is excluded from automatic checks (out-of-scope), so these requirements are not checked. The counts above show what the summary records. |

The 9 counts marked as a part above are the whole of the 38 mandatory requirements: they add up to 38. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level), not checked | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | OSPFv3 Extended LSAs |
| Automatic checks | Excluded (out-of-scope) |
| Requirements | 50 |
| Mandatory requirements (MUST level), not checked | 38 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 34 |
| Nightly-only evidence | 0 |
| Test tags | 9 |
| Tests linked to requirements | 9 |
| Recorded audit verdicts | 0 |
| Fault test records | 9 |
| Summary | `rfc/short/rfc8362.md` |
| Requirement table | `rfc/requirements/rfc8362.md` |
| RFC text | `rfc/full/rfc8362.txt` |

## Automatic checks

Excluded from automatic checks (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OSPFv3 Link State Advertisement (LSA) Extensibility. OUT OF SCOPE as a document by owner decision, 2026-09-01, marked for future development. The extraction is COMPLETE: the source text is at rfc/full/rfc8362.txt and this summary declares all 50 requirements, 38 of them MUST-level. What Ze has is a by-product of RFC 8666 Segment Routing rather than the framework this document defines: three of the seven Extended LSA types are originated, and only when segment routing is enabled (v6OriginateSR, internal/plugins/ospf/sr_origination_v6.go); receipt decoding is reached from one place and reads Prefix-SIDs alone (v6ReceivedPrefixSIDs, sr_reception_v6.go); no SPF calculation reads an Extended LSA, because the base Router-LSA remains the sole SPF vertex; and there is no RFC 8362 configuration surface and none of its Appendix A or B migration machinery. Nine MUST-level requirements are genuinely unmet and 20 more are met only because Ze performs no action on their subject. RFC8362-2-1 was the most serious until a9d0b54387 (2026-09-20) set the U-bit on the Extended LS type constants (internal/plugins/ospf/v3/types/lsa.go, 0xA021 E-Router-LSA), as Section 2 requires; TestRFC8362ExtendedLSAsSetUBitOnTheWire reads it on the wire for all three Extended LSA types Ze originates (E-Router, E-Intra-Area-Prefix and, from 2026-10-21, E-Inter-Area-Prefix).

## Published support status

**Area:** OSPFv3 Extended LSAs

**Test verification:** not walked

**Evidence:** -

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 1 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 34 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level), not checked** | **38** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (3):** [`RFC8362-3.2-1`](#rfc8362-3.2-1), [`RFC8362-3.3-1`](#rfc8362-3.3-1), [`RFC8362-6.3-1`](#rfc8362-6.3-1)

**Annotated (including scoped evidence) (1):** [`RFC8362-2-1`](#rfc8362-2-1)

**No test and no annotation (34):** [`RFC8362-3.1.1-1`](#rfc8362-3.1.1-1), [`RFC8362-3.4-1`](#rfc8362-3.4-1), [`RFC8362-3.5-1`](#rfc8362-3.5-1), [`RFC8362-3.6-1`](#rfc8362-3.6-1), [`RFC8362-3.7-1`](#rfc8362-3.7-1), [`RFC8362-3.8-1`](#rfc8362-3.8-1), [`RFC8362-3.9-1`](#rfc8362-3.9-1), [`RFC8362-3.10-1`](#rfc8362-3.10-1), [`RFC8362-3.10-2`](#rfc8362-3.10-2), [`RFC8362-3.11-1`](#rfc8362-3.11-1), [`RFC8362-3.11-2`](#rfc8362-3.11-2), [`RFC8362-3.12-1`](#rfc8362-3.12-1), [`RFC8362-4.2-1`](#rfc8362-4.2-1), [`RFC8362-4.3-1`](#rfc8362-4.3-1), [`RFC8362-4.3-2`](#rfc8362-4.3-2), [`RFC8362-4.4-1`](#rfc8362-4.4-1), [`RFC8362-4.4-2`](#rfc8362-4.4-2), [`RFC8362-4.5-1`](#rfc8362-4.5-1), [`RFC8362-4.5-2`](#rfc8362-4.5-2), [`RFC8362-4.7-1`](#rfc8362-4.7-1), [`RFC8362-4.7-2`](#rfc8362-4.7-2), [`RFC8362-4.7-3`](#rfc8362-4.7-3), [`RFC8362-4.8-1`](#rfc8362-4.8-1), [`RFC8362-5-1`](#rfc8362-5-1), [`RFC8362-5-2`](#rfc8362-5-2), [`RFC8362-6.2-1`](#rfc8362-6.2-1), [`RFC8362-6.3-2`](#rfc8362-6.3-2), [`RFC8362-6.3-3`](#rfc8362-6.3-3), [`RFC8362-6.3-4`](#rfc8362-6.3-4), [`RFC8362-6.3-5`](#rfc8362-6.3-5), [`RFC8362-8.1-1`](#rfc8362-8.1-1), [`RFC8362-8.1-2`](#rfc8362-8.1-2), [`RFC8362-8.2-1`](#rfc8362-8.2-1), [`RFC8362-8.2-2`](#rfc8362-8.2-2)

## Figures

- **MUST bar**: requirements not reviewed
- **Overview bar**: requirements not reviewed

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 1 |
| Mandatory requirements complete | not complete |
| MUST rows | requirements not reviewed |
| MUST rows kept (the MUST bar) | requirements not reviewed |
| Ze verified | requirements not reviewed |
| OS verified | requirements not reviewed |
| Tests awaiting verification | requirements not reviewed |
| OS testing gap | requirements not reviewed |
| Exclusion unchecked | requirements not reviewed |
| Not assessed | requirements not reviewed |
| Not built | requirements not reviewed |
| MUST rows not owed | requirements not reviewed |
| MUST not owed: feature-declined | requirements not reviewed |
| MUST not owed: linux-limit | requirements not reviewed |
| MUST not owed: not-applicable | requirements not reviewed |
| SHOULD / MAY rows | requirements not reviewed |
| SHOULD verified | requirements not reviewed |
| SHOULD OS verified | requirements not reviewed |
| SHOULD OS testing gap | requirements not reviewed |
| SHOULD not owed | requirements not reviewed |
| SHOULD not owed: feature-declined | requirements not reviewed |
| SHOULD not owed: linux-limit | requirements not reviewed |
| SHOULD not owed: not-applicable | requirements not reviewed |
| SHOULD tests awaiting verification | requirements not reviewed |
| SHOULD exclusion unchecked | requirements not reviewed |
| SHOULD not assessed | requirements not reviewed |
| SHOULD not built | requirements not reviewed |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc8362.json](../../../data/rfc/rfc8362.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8362-2-1` | For backward compatibility, the U-bit MUST be set in the LS Type so that the LSAs will be flooded by OSPFv3 routers that do not understand them. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC8362ExtendedLSAsSetUBitOnTheWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8362_test.go#L66). **negative:** no negative test. **{single-polarity}:** the U-bit is part of each Extended LS Type constant (internal/plugins/ospf/v3/types/lsa.go, LSTypeERouter = lsTypeUBit \| ...), so no Extended LSA can be built with U clear and no violating input exists to reject; the clear-U base types are RFC 5340 Appendix A.4.2.1, a different rule |
| `RFC8362-3.1.1-1` | If the N-bit is set and the PrefixLength is NOT 128 for the IPv6 Address Family or 32 for the IPv4 Address Family [OSPFV3-AF], the N-bit MUST be ignored. (§3.1.1) | MUST | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.2-1` | The Router-Link TLV is only applicable to the E-Router-LSA (Section 4.1). Inclusion in other Extended LSAs MUST be ignored. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8362RouterLinkTLVReadFromERouter`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8362_tlv_applicability_test.go#L90). **negative:** `unit/verify` [`TestRFC8362RouterLinkTLVIgnoredOutsideERouter`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8362_tlv_applicability_test.go#L118) |
| `RFC8362-3.3-1` | The Attached-Routers TLV is only applicable to the E-Network-LSA (Section 4.2). Inclusion in other Extended LSAs MUST be ignored. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC8362AttachedRoutersTLVReadFromENetwork`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8362_tlv_applicability_test.go#L143). **negative:** `unit/verify` [`TestRFC8362AttachedRoutersTLVIgnoredOutsideENetwork`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8362_tlv_applicability_test.go#L176) |
| `RFC8362-3.4-1` | The Inter-Area-Prefix TLV is only applicable to the E-Inter-Area-Prefix-LSA (Section 4.3). Inclusion in other Extended LSAs MUST be ignored. (§3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.5-1` | The Inter-Area-Router TLV is only applicable to the E-Inter-Area-Router-LSA (Section 4.4). Inclusion in other Extended LSAs MUST be ignored. (§3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.6-1` | The External-Prefix TLV is only applicable to the E-AS-External-LSA (Section 4.5) and the E-NSSA-LSA (Section 4.6). Additionally, the PrefixOptions are extended as described in Section 3.1. Inclusion in other Extended LSAs MUST be ignored. (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.7-1` | The Intra-Area-Prefix TLV is only applicable to the E-Link-LSA (Section 4.7) and the E-Intra-Area-Prefix-LSA (Section 4.8). Additionally, the PrefixOptions are extended as described in Section 3.1. Inclusion in other Extended LSAs MUST be ignored. (§3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.8-1` | The IPv6 Link-Local Address TLV is only applicable to the E-Link-LSA (Section 4.7). Inclusion in other Extended LSAs MUST be ignored. (§3.8) | MUST | 3.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.9-1` | The IPv4 Link-Local Address TLV is only applicable to the E-Link-LSA (Section 4.7). Inclusion in other Extended LSAs MUST be ignored. (§3.9) | MUST | 3.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.10-1` | The sub-TLV is optional and the first specified instance is used as the forwarding address as defined in [OSPFV3]. Instances subsequent to the first MUST be ignored. (§3.10) | MUST | 3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.10-2` | The IPv6-Forwarding-Address TLV is to be used with IPv6 address families as defined in [OSPFV3-AF]. It MUST be ignored for other address families. (§3.10) | MUST | 3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.11-1` | The sub-TLV is optional, and the first specified instance is used as the forwarding address as defined in [OSPFV3]. Instances subsequent to the first MUST be ignored. (§3.11) | MUST | 3.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.11-2` | The IPv4-Forwarding-Address TLV is to be used with IPv4 address families as defined in [OSPFV3-AF]. It MUST be ignored for other address families. (§3.11) | MUST | 3.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.12-1` | The sub-TLV is optional, and the first specified instance is used as the Route Tag as defined in [OSPFV3]. Instances subsequent to the first MUST be ignored. (§3.12) | MUST | 3.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.2-1` | Instances of the Attached-Router TLV subsequent to the first MUST be ignored. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.3-1` | In order to retain compatibility and semantics with the current OSPFv3 specification, each Inter-Area-Prefix LSA MUST contain a single Inter-Area-Prefix TLV. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.3-2` | Instances of the Inter-Area-Prefix TLV subsequent to the first MUST be ignored. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.4-1` | In order to retain compatibility and semantics with the current OSPFv3 specification, each Inter-Area-Router-LSA MUST contain a single Inter-Area-Router TLV. (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.4-2` | Instances of the Inter-Area-Router TLV subsequent to the first MUST be ignored. (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.5-1` | In order to retain compatibility and semantics with the current OSPFv3 specification, each LSA MUST contain a single External-Prefix TLV. (§4.5) | MUST | 4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.5-2` | Instances of the External-Prefix TLV subsequent to the first MUST be ignored. (§4.5) | MUST | 4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.7-1` | A single instance of the IPv6 Link-Local Address TLV (Section 3.8) SHOULD be included in the E-Link-LSA. Instances following the first MUST be ignored. (§4.7) | MUST | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.7-2` | A single instance of the IPv6 Link-Local Address TLV (Section 3.8) SHOULD be included in the E-Link-LSA. Instances following the first MUST be ignored. For IPv4 address families as defined in [OSPFV3-AF], this TLV MUST be ignored. (§4.7) | MUST | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.7-3` | Similarly, only a single instance of the IPv4 Link-Local Address TLV (Section 3.9) SHOULD be included in the E-Link-LSA. Instances following the first MUST be ignored. (§4.7) | MUST | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.8-1` | The Referenced LS Type MUST be either an E-Router-LSA (0xA021) or an E-Network-LSA (0xA022). (§4.8) | MUST | 4.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-5-1` | Extended LSAs that have inconsistent length or other encoding errors, as described herein, MUST NOT be installed in the Link State Database, acknowledged, or flooded. (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-5-2` | Additionally, an LSA MUST be considered malformed if it does not include all of the required TLVs and sub-TLVs. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-6.2-1` | However, if a top-level TLV is advertised, it MUST include required sub-TLVs, or it will be considered malformed as described in Section 5. (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-6.3-1` | To ensure compatibility of future TLV-based LSA extensions, all implementations MUST adhere to these rules: 1. Unrecognized TLVs and sub-TLVs are ignored when parsing or processing Extended LSAs. (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC8362UnknownTLVKeepsLSAForBGPLS`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8362_unknown_tlv_test.go#L67). **positive:** `unit/verify` [`TestRFC8362UnknownTLVKeepsLSAForPrefixSIDs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8362_unknown_tlv_test.go#L109). **negative:** `unit/verify` [`TestRFC8362UnknownTLVIgnoredByBGPLS`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8362_unknown_tlv_test.go#L88). **negative:** `unit/verify` [`TestRFC8362UnknownTLVIgnoredByPrefixSIDs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8362_unknown_tlv_test.go#L129) |
| `RFC8362-6.3-2` | Whether or not partial deployment of a given TLV is supported MUST be specified. (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-6.3-3` | If partial deployment is not supported, mechanisms to ensure the corresponding feature is not deployed MUST be specified in the document defining the new TLV or sub-TLV. (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-6.3-4` | If partial deployment is supported, backward compatibility and partial deployment MUST be specified in the document defining the new TLV or sub-TLV. (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-6.3-5` | Documents specifying future TLVs or Sub-TLVs MUST specify the requirements for usage of those TLVs or sub-TLVs. (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-8.1-1` | Types in the range 32768-33023 are Reserved for Experimental Use; these will not be registered with IANA and MUST NOT be mentioned by RFCs. (§8.1) | MUST NOT | 8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-8.1-2` | Before any assignments can be made in the 33024-65535 range, there MUST be an IETF specification that specifies IANA Considerations that cover the range being assigned. (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-8.2-1` | Types in the range 32768-33023 are Reserved for Experimental Use; these will not be registered with IANA and MUST NOT be mentioned by RFCs. (§8.2) | MUST NOT | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-8.2-2` | Before any assignments can be made in the 33024-65535 range, there MUST be an IETF specification that specifies IANA Considerations that cover the range being assigned. (§8.2) | MUST | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.1-1` | The applicability of the LA-bit is expanded, and it SHOULD be set in Inter-Area-Prefix TLVs and MAY be set in External-Prefix TLVs when the advertised host IPv6 address, i.e., PrefixLength = 128 for the IPv6 Address Family or PrefixLength = 32 for the IPv4 Address Family [OSPFV3-AF], is an interface address. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.7-4` | A single instance of the IPv6 Link-Local Address TLV (Section 3.8) SHOULD be included in the E-Link-LSA. (§4.7) | SHOULD | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.7-5` | Similarly, only a single instance of the IPv4 Link-Local Address TLV (Section 3.9) SHOULD be included in the E-Link-LSA. (§4.7) | SHOULD | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.7-6` | Similarly, only a single instance of the IPv4 Link-Local Address TLV (Section 3.9) SHOULD be included in the E-Link-LSA. Instances following the first MUST be ignored. For OSPFv3 IPv6 address families as defined in [OSPFV3-AF], this TLV SHOULD be ignored. (§4.7) | SHOULD | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-5-3` | Reception of malformed LSAs SHOULD be counted and/or logged for examination by the administrator of the OSPFv3 routing domain. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-6-1` | Both full and, if applicable, partial deployment SHOULD be specified for future TLV- based OSPFv3 LSA extensions. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-6.3-6` | If a TLV or sub-TLV is recognized but the length is less than the minimum, then the LSA should be considered malformed, and it SHOULD NOT be acknowledged. (§6.3) | SHOULD NOT | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-6.3-7` | Additionally, the occurrence SHOULD be logged with enough information to identify the LSA by type, Link State ID, originator, and sequence number and identify the TLV or sub-TLV in error. (§6.3) | SHOULD | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-B-1` | For regular areas, i.e., areas where AS-scoped LSAs are flooded, disabling AreaExtendedLSASupport for a regular OSPFv3 area (not a Stub or NSSA area) when ExtendedLSASupport is enabled is contradictory and SHOULD be prohibited by implementations. (§B) | SHOULD | B | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3-1` | In general, TLVs and sub-TLVs MAY occur in any order, and the specification should define whether the TLV or sub-TLV is required and the behavior when there are multiple occurrences of the TLV or sub-TLV. (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.1-2` | The applicability of the LA-bit is expanded, and it SHOULD be set in Inter-Area-Prefix TLVs and MAY be set in External-Prefix TLVs when the advertised host IPv6 address, i.e., PrefixLength = 128 for the IPv6 Address Family or PrefixLength = 32 for the IPv4 Address Family [OSPFV3-AF], is an interface address. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.1.1-2` | The advertising router MAY choose NOT to set the N-bit even when the above conditions are met. (§3.1.1) | MAY | 3.1.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8362-3.1.1-1`](#rfc8362-3.1.1-1) If the N-bit is set and the PrefixLength is NOT 128 for the IPv6 Address Family or 32 for the IPv4 Address Family [OSPFV3-AF], the N-bit MUST be ignored. (§3.1.1) | no test | no test carries this requirement id |
| [`RFC8362-3.4-1`](#rfc8362-3.4-1) The Inter-Area-Prefix TLV is only applicable to the E-Inter-Area-Prefix-LSA (Section 4.3). Inclusion in other Extended LSAs MUST be ignored. (§3.4) | no test | no test carries this requirement id |
| [`RFC8362-3.5-1`](#rfc8362-3.5-1) The Inter-Area-Router TLV is only applicable to the E-Inter-Area-Router-LSA (Section 4.4). Inclusion in other Extended LSAs MUST be ignored. (§3.5) | no test | no test carries this requirement id |
| [`RFC8362-3.6-1`](#rfc8362-3.6-1) The External-Prefix TLV is only applicable to the E-AS-External-LSA (Section 4.5) and the E-NSSA-LSA (Section 4.6). Additionally, the PrefixOptions are extended as described in Section 3.1. Inclusion in other Extended LSAs MUST be ignored. (§3.6) | no test | no test carries this requirement id |
| [`RFC8362-3.7-1`](#rfc8362-3.7-1) The Intra-Area-Prefix TLV is only applicable to the E-Link-LSA (Section 4.7) and the E-Intra-Area-Prefix-LSA (Section 4.8). Additionally, the PrefixOptions are extended as described in Section 3.1. Inclusion in other Extended LSAs MUST be ignored. (§3.7) | no test | no test carries this requirement id |
| [`RFC8362-3.8-1`](#rfc8362-3.8-1) The IPv6 Link-Local Address TLV is only applicable to the E-Link-LSA (Section 4.7). Inclusion in other Extended LSAs MUST be ignored. (§3.8) | no test | no test carries this requirement id |
| [`RFC8362-3.9-1`](#rfc8362-3.9-1) The IPv4 Link-Local Address TLV is only applicable to the E-Link-LSA (Section 4.7). Inclusion in other Extended LSAs MUST be ignored. (§3.9) | no test | no test carries this requirement id |
| [`RFC8362-3.10-1`](#rfc8362-3.10-1) The sub-TLV is optional and the first specified instance is used as the forwarding address as defined in [OSPFV3]. Instances subsequent to the first MUST be ignored. (§3.10) | no test | no test carries this requirement id |
| [`RFC8362-3.10-2`](#rfc8362-3.10-2) The IPv6-Forwarding-Address TLV is to be used with IPv6 address families as defined in [OSPFV3-AF]. It MUST be ignored for other address families. (§3.10) | no test | no test carries this requirement id |
| [`RFC8362-3.11-1`](#rfc8362-3.11-1) The sub-TLV is optional, and the first specified instance is used as the forwarding address as defined in [OSPFV3]. Instances subsequent to the first MUST be ignored. (§3.11) | no test | no test carries this requirement id |
| [`RFC8362-3.11-2`](#rfc8362-3.11-2) The IPv4-Forwarding-Address TLV is to be used with IPv4 address families as defined in [OSPFV3-AF]. It MUST be ignored for other address families. (§3.11) | no test | no test carries this requirement id |
| [`RFC8362-3.12-1`](#rfc8362-3.12-1) The sub-TLV is optional, and the first specified instance is used as the Route Tag as defined in [OSPFV3]. Instances subsequent to the first MUST be ignored. (§3.12) | no test | no test carries this requirement id |
| [`RFC8362-4.2-1`](#rfc8362-4.2-1) Instances of the Attached-Router TLV subsequent to the first MUST be ignored. (§4.2) | no test | no test carries this requirement id |
| [`RFC8362-4.3-1`](#rfc8362-4.3-1) In order to retain compatibility and semantics with the current OSPFv3 specification, each Inter-Area-Prefix LSA MUST contain a single Inter-Area-Prefix TLV. (§4.3) | no test | no test carries this requirement id |
| [`RFC8362-4.3-2`](#rfc8362-4.3-2) Instances of the Inter-Area-Prefix TLV subsequent to the first MUST be ignored. (§4.3) | no test | no test carries this requirement id |
| [`RFC8362-4.4-1`](#rfc8362-4.4-1) In order to retain compatibility and semantics with the current OSPFv3 specification, each Inter-Area-Router-LSA MUST contain a single Inter-Area-Router TLV. (§4.4) | no test | no test carries this requirement id |
| [`RFC8362-4.4-2`](#rfc8362-4.4-2) Instances of the Inter-Area-Router TLV subsequent to the first MUST be ignored. (§4.4) | no test | no test carries this requirement id |
| [`RFC8362-4.5-1`](#rfc8362-4.5-1) In order to retain compatibility and semantics with the current OSPFv3 specification, each LSA MUST contain a single External-Prefix TLV. (§4.5) | no test | no test carries this requirement id |
| [`RFC8362-4.5-2`](#rfc8362-4.5-2) Instances of the External-Prefix TLV subsequent to the first MUST be ignored. (§4.5) | no test | no test carries this requirement id |
| [`RFC8362-4.7-1`](#rfc8362-4.7-1) A single instance of the IPv6 Link-Local Address TLV (Section 3.8) SHOULD be included in the E-Link-LSA. Instances following the first MUST be ignored. (§4.7) | no test | no test carries this requirement id |
| [`RFC8362-4.7-2`](#rfc8362-4.7-2) A single instance of the IPv6 Link-Local Address TLV (Section 3.8) SHOULD be included in the E-Link-LSA. Instances following the first MUST be ignored. For IPv4 address families as defined in [OSPFV3-AF], this TLV MUST be ignored. (§4.7) | no test | no test carries this requirement id |
| [`RFC8362-4.7-3`](#rfc8362-4.7-3) Similarly, only a single instance of the IPv4 Link-Local Address TLV (Section 3.9) SHOULD be included in the E-Link-LSA. Instances following the first MUST be ignored. (§4.7) | no test | no test carries this requirement id |
| [`RFC8362-4.8-1`](#rfc8362-4.8-1) The Referenced LS Type MUST be either an E-Router-LSA (0xA021) or an E-Network-LSA (0xA022). (§4.8) | no test | no test carries this requirement id |
| [`RFC8362-5-1`](#rfc8362-5-1) Extended LSAs that have inconsistent length or other encoding errors, as described herein, MUST NOT be installed in the Link State Database, acknowledged, or flooded. (§5) | no test | no test carries this requirement id |
| [`RFC8362-5-2`](#rfc8362-5-2) Additionally, an LSA MUST be considered malformed if it does not include all of the required TLVs and sub-TLVs. (§5) | no test | no test carries this requirement id |
| [`RFC8362-6.2-1`](#rfc8362-6.2-1) However, if a top-level TLV is advertised, it MUST include required sub-TLVs, or it will be considered malformed as described in Section 5. (§6.2) | no test | no test carries this requirement id |
| [`RFC8362-6.3-2`](#rfc8362-6.3-2) Whether or not partial deployment of a given TLV is supported MUST be specified. (§6.3) | no test | no test carries this requirement id |
| [`RFC8362-6.3-3`](#rfc8362-6.3-3) If partial deployment is not supported, mechanisms to ensure the corresponding feature is not deployed MUST be specified in the document defining the new TLV or sub-TLV. (§6.3) | no test | no test carries this requirement id |
| [`RFC8362-6.3-4`](#rfc8362-6.3-4) If partial deployment is supported, backward compatibility and partial deployment MUST be specified in the document defining the new TLV or sub-TLV. (§6.3) | no test | no test carries this requirement id |
| [`RFC8362-6.3-5`](#rfc8362-6.3-5) Documents specifying future TLVs or Sub-TLVs MUST specify the requirements for usage of those TLVs or sub-TLVs. (§6.3) | no test | no test carries this requirement id |
| [`RFC8362-8.1-1`](#rfc8362-8.1-1) Types in the range 32768-33023 are Reserved for Experimental Use; these will not be registered with IANA and MUST NOT be mentioned by RFCs. (§8.1) | no test | no test carries this requirement id |
| [`RFC8362-8.1-2`](#rfc8362-8.1-2) Before any assignments can be made in the 33024-65535 range, there MUST be an IETF specification that specifies IANA Considerations that cover the range being assigned. (§8.1) | no test | no test carries this requirement id |
| [`RFC8362-8.2-1`](#rfc8362-8.2-1) Types in the range 32768-33023 are Reserved for Experimental Use; these will not be registered with IANA and MUST NOT be mentioned by RFCs. (§8.2) | no test | no test carries this requirement id |
| [`RFC8362-8.2-2`](#rfc8362-8.2-2) Before any assignments can be made in the 33024-65535 range, there MUST be an IETF specification that specifies IANA Considerations that cover the range being assigned. (§8.2) | no test | no test carries this requirement id |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC8362-2-1`](#rfc8362-2-1)

For backward compatibility, the U-bit MUST be set in the LS Type so that the LSAs will be flooded by OSPFv3 routers that do not understand them. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC8362ExtendedLSAsSetUBitOnTheWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8362_test.go#L66) | unit/verify | revert, verified |

### [`RFC8362-3.1.1-1`](#rfc8362-3.1.1-1)

If the N-bit is set and the PrefixLength is NOT 128 for the IPv6 Address Family or 32 for the IPv4 Address Family [OSPFV3-AF], the N-bit MUST be ignored. (§3.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-3.1.1-1, so there is no linked test.

### [`RFC8362-3.2-1`](#rfc8362-3.2-1)

The Router-Link TLV is only applicable to the E-Router-LSA (Section 4.1). Inclusion in other Extended LSAs MUST be ignored. (§3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8362RouterLinkTLVIgnoredOutsideERouter`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8362_tlv_applicability_test.go#L118) | unit/verify | mutant, verified |
| positive | [`TestRFC8362RouterLinkTLVReadFromERouter`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8362_tlv_applicability_test.go#L90) | unit/verify | mutant, verified |

### [`RFC8362-3.3-1`](#rfc8362-3.3-1)

The Attached-Routers TLV is only applicable to the E-Network-LSA (Section 4.2). Inclusion in other Extended LSAs MUST be ignored. (§3.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8362AttachedRoutersTLVIgnoredOutsideENetwork`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8362_tlv_applicability_test.go#L176) | unit/verify | mutant, verified |
| positive | [`TestRFC8362AttachedRoutersTLVReadFromENetwork`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8362_tlv_applicability_test.go#L143) | unit/verify | mutant, verified |

### [`RFC8362-3.4-1`](#rfc8362-3.4-1)

The Inter-Area-Prefix TLV is only applicable to the E-Inter-Area-Prefix-LSA (Section 4.3). Inclusion in other Extended LSAs MUST be ignored. (§3.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-3.4-1, so there is no linked test.

### [`RFC8362-3.5-1`](#rfc8362-3.5-1)

The Inter-Area-Router TLV is only applicable to the E-Inter-Area-Router-LSA (Section 4.4). Inclusion in other Extended LSAs MUST be ignored. (§3.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-3.5-1, so there is no linked test.

### [`RFC8362-3.6-1`](#rfc8362-3.6-1)

The External-Prefix TLV is only applicable to the E-AS-External-LSA (Section 4.5) and the E-NSSA-LSA (Section 4.6). Additionally, the PrefixOptions are extended as described in Section 3.1. Inclusion in other Extended LSAs MUST be ignored. (§3.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-3.6-1, so there is no linked test.

### [`RFC8362-3.7-1`](#rfc8362-3.7-1)

The Intra-Area-Prefix TLV is only applicable to the E-Link-LSA (Section 4.7) and the E-Intra-Area-Prefix-LSA (Section 4.8). Additionally, the PrefixOptions are extended as described in Section 3.1. Inclusion in other Extended LSAs MUST be ignored. (§3.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-3.7-1, so there is no linked test.

### [`RFC8362-3.8-1`](#rfc8362-3.8-1)

The IPv6 Link-Local Address TLV is only applicable to the E-Link-LSA (Section 4.7). Inclusion in other Extended LSAs MUST be ignored. (§3.8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-3.8-1, so there is no linked test.

### [`RFC8362-3.9-1`](#rfc8362-3.9-1)

The IPv4 Link-Local Address TLV is only applicable to the E-Link-LSA (Section 4.7). Inclusion in other Extended LSAs MUST be ignored. (§3.9)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-3.9-1, so there is no linked test.

### [`RFC8362-3.10-1`](#rfc8362-3.10-1)

The sub-TLV is optional and the first specified instance is used as the forwarding address as defined in [OSPFV3]. Instances subsequent to the first MUST be ignored. (§3.10)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-3.10-1, so there is no linked test.

### [`RFC8362-3.10-2`](#rfc8362-3.10-2)

The IPv6-Forwarding-Address TLV is to be used with IPv6 address families as defined in [OSPFV3-AF]. It MUST be ignored for other address families. (§3.10)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-3.10-2, so there is no linked test.

### [`RFC8362-3.11-1`](#rfc8362-3.11-1)

The sub-TLV is optional, and the first specified instance is used as the forwarding address as defined in [OSPFV3]. Instances subsequent to the first MUST be ignored. (§3.11)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-3.11-1, so there is no linked test.

### [`RFC8362-3.11-2`](#rfc8362-3.11-2)

The IPv4-Forwarding-Address TLV is to be used with IPv4 address families as defined in [OSPFV3-AF]. It MUST be ignored for other address families. (§3.11)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-3.11-2, so there is no linked test.

### [`RFC8362-3.12-1`](#rfc8362-3.12-1)

The sub-TLV is optional, and the first specified instance is used as the Route Tag as defined in [OSPFV3]. Instances subsequent to the first MUST be ignored. (§3.12)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-3.12-1, so there is no linked test.

### [`RFC8362-4.2-1`](#rfc8362-4.2-1)

Instances of the Attached-Router TLV subsequent to the first MUST be ignored. (§4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-4.2-1, so there is no linked test.

### [`RFC8362-4.3-1`](#rfc8362-4.3-1)

In order to retain compatibility and semantics with the current OSPFv3 specification, each Inter-Area-Prefix LSA MUST contain a single Inter-Area-Prefix TLV. (§4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-4.3-1, so there is no linked test.

### [`RFC8362-4.3-2`](#rfc8362-4.3-2)

Instances of the Inter-Area-Prefix TLV subsequent to the first MUST be ignored. (§4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-4.3-2, so there is no linked test.

### [`RFC8362-4.4-1`](#rfc8362-4.4-1)

In order to retain compatibility and semantics with the current OSPFv3 specification, each Inter-Area-Router-LSA MUST contain a single Inter-Area-Router TLV. (§4.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-4.4-1, so there is no linked test.

### [`RFC8362-4.4-2`](#rfc8362-4.4-2)

Instances of the Inter-Area-Router TLV subsequent to the first MUST be ignored. (§4.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-4.4-2, so there is no linked test.

### [`RFC8362-4.5-1`](#rfc8362-4.5-1)

In order to retain compatibility and semantics with the current OSPFv3 specification, each LSA MUST contain a single External-Prefix TLV. (§4.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-4.5-1, so there is no linked test.

### [`RFC8362-4.5-2`](#rfc8362-4.5-2)

Instances of the External-Prefix TLV subsequent to the first MUST be ignored. (§4.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-4.5-2, so there is no linked test.

### [`RFC8362-4.7-1`](#rfc8362-4.7-1)

A single instance of the IPv6 Link-Local Address TLV (Section 3.8) SHOULD be included in the E-Link-LSA. Instances following the first MUST be ignored. (§4.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-4.7-1, so there is no linked test.

### [`RFC8362-4.7-2`](#rfc8362-4.7-2)

A single instance of the IPv6 Link-Local Address TLV (Section 3.8) SHOULD be included in the E-Link-LSA. Instances following the first MUST be ignored. For IPv4 address families as defined in [OSPFV3-AF], this TLV MUST be ignored. (§4.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-4.7-2, so there is no linked test.

### [`RFC8362-4.7-3`](#rfc8362-4.7-3)

Similarly, only a single instance of the IPv4 Link-Local Address TLV (Section 3.9) SHOULD be included in the E-Link-LSA. Instances following the first MUST be ignored. (§4.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-4.7-3, so there is no linked test.

### [`RFC8362-4.8-1`](#rfc8362-4.8-1)

The Referenced LS Type MUST be either an E-Router-LSA (0xA021) or an E-Network-LSA (0xA022). (§4.8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-4.8-1, so there is no linked test.

### [`RFC8362-5-1`](#rfc8362-5-1)

Extended LSAs that have inconsistent length or other encoding errors, as described herein, MUST NOT be installed in the Link State Database, acknowledged, or flooded. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-5-1, so there is no linked test.

### [`RFC8362-5-2`](#rfc8362-5-2)

Additionally, an LSA MUST be considered malformed if it does not include all of the required TLVs and sub-TLVs. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-5-2, so there is no linked test.

### [`RFC8362-6.2-1`](#rfc8362-6.2-1)

However, if a top-level TLV is advertised, it MUST include required sub-TLVs, or it will be considered malformed as described in Section 5. (§6.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-6.2-1, so there is no linked test.

### [`RFC8362-6.3-1`](#rfc8362-6.3-1)

To ensure compatibility of future TLV-based LSA extensions, all implementations MUST adhere to these rules: 1. Unrecognized TLVs and sub-TLVs are ignored when parsing or processing Extended LSAs. (§6.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8362UnknownTLVIgnoredByBGPLS`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8362_unknown_tlv_test.go#L88) | unit/verify | mutant, verified |
| negative | [`TestRFC8362UnknownTLVIgnoredByPrefixSIDs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8362_unknown_tlv_test.go#L129) | unit/verify | mutant, verified |
| positive | [`TestRFC8362UnknownTLVKeepsLSAForBGPLS`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8362_unknown_tlv_test.go#L67) | unit/verify | mutant, verified |
| positive | [`TestRFC8362UnknownTLVKeepsLSAForPrefixSIDs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc8362_unknown_tlv_test.go#L109) | unit/verify | mutant, verified |

### [`RFC8362-6.3-2`](#rfc8362-6.3-2)

Whether or not partial deployment of a given TLV is supported MUST be specified. (§6.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-6.3-2, so there is no linked test.

### [`RFC8362-6.3-3`](#rfc8362-6.3-3)

If partial deployment is not supported, mechanisms to ensure the corresponding feature is not deployed MUST be specified in the document defining the new TLV or sub-TLV. (§6.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-6.3-3, so there is no linked test.

### [`RFC8362-6.3-4`](#rfc8362-6.3-4)

If partial deployment is supported, backward compatibility and partial deployment MUST be specified in the document defining the new TLV or sub-TLV. (§6.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-6.3-4, so there is no linked test.

### [`RFC8362-6.3-5`](#rfc8362-6.3-5)

Documents specifying future TLVs or Sub-TLVs MUST specify the requirements for usage of those TLVs or sub-TLVs. (§6.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-6.3-5, so there is no linked test.

### [`RFC8362-8.1-1`](#rfc8362-8.1-1)

Types in the range 32768-33023 are Reserved for Experimental Use; these will not be registered with IANA and MUST NOT be mentioned by RFCs. (§8.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-8.1-1, so there is no linked test.

### [`RFC8362-8.1-2`](#rfc8362-8.1-2)

Before any assignments can be made in the 33024-65535 range, there MUST be an IETF specification that specifies IANA Considerations that cover the range being assigned. (§8.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-8.1-2, so there is no linked test.

### [`RFC8362-8.2-1`](#rfc8362-8.2-1)

Types in the range 32768-33023 are Reserved for Experimental Use; these will not be registered with IANA and MUST NOT be mentioned by RFCs. (§8.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-8.2-1, so there is no linked test.

### [`RFC8362-8.2-2`](#rfc8362-8.2-2)

Before any assignments can be made in the 33024-65535 range, there MUST be an IETF specification that specifies IANA Considerations that cover the range being assigned. (§8.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8362-8.2-2, so there is no linked test.

## Requirement list review

No requirement list review is recorded for RFC 8362, so there is no recorded check of its requirements against the original RFC, sentence by sentence.

## Superseded

No document obsoletes RFC 8362, so its obligations are stated where they were written.
