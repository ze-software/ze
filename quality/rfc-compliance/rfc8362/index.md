# RFC 8362 - OSPFv3 Link State Advertisement (LSA) Extensibility

Future. Every requirement this repository extracted from RFC 8362, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 38 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 38 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 38 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 38 | of 50 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (out-of-scope), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 38 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 38 of 38 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 38 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
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
| Public status | Future |
| Enrolment | Not enrolled (out-of-scope) |
| Requirements | 50 |
| Gated MUST-level | 38 |
| Obligations that bind Ze | 38 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 38 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8362.md` |
| Requirement shard | `rfc/requirements/rfc8362.md` |
| RFC text | `rfc/full/rfc8362.txt` |

## Enrolment

Not enrolled (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OSPFv3 Link State Advertisement (LSA) Extensibility. OUT OF SCOPE as a document by owner decision, 2026-09-01, marked for future development. The extraction is COMPLETE: the source text is at rfc/full/rfc8362.txt and this summary declares all 50 requirements, 38 of them MUST-level. What Ze has is a by-product of RFC 8666 Segment Routing rather than the framework this document defines: three of the seven Extended LSA types are originated, and only when segment routing is enabled (v6OriginateSR, internal/plugins/ospf/sr_origination_v6.go); receipt decoding is reached from one place and reads Prefix-SIDs alone (v6ReceivedPrefixSIDs, sr_reception_v6.go); no SPF calculation reads an Extended LSA, because the base Router-LSA remains the sole SPF vertex; and there is no RFC 8362 configuration surface and none of its Appendix A or B migration machinery. Nine MUST-level requirements are genuinely unmet and 20 more are met only because Ze performs no action on their subject. The most serious is RFC8362-2-1: the seven LS type constants carry the U-bit clear where Section 2 requires it set, which is recorded as a defect in plan/journal/declared-format-contradicts-payload.md and is a fix rather than a scope question. No {gap} annotation is written here, because the scope decision covers the document and the U-bit defect is tracked where a fix is owed.

## What the public ledger says

**Status:** Future

**What the ledger says is covered**

Three of the seven Extended LSA types are framed and originated, and only under `segment-routing`: E-Router (0x2021), E-Intra-Area-Prefix (0x2029) and E-Inter-Area-Prefix (0x2023), through `EncodeExtendedLSABody` ([`internal/plugins/ospf/v3/packet/lsa_extended.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/packet/lsa_extended.go)). Receipt decoding reads Prefix-SIDs and nothing else. No SPF calculation reads an Extended LSA.

**What the ledger says remains**

Out of scope as a document by owner decision, 2026-09-01, and tracked for future development. Requirements bound per line in [`rfc/short/rfc8362.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8362.md). E-Network, E-Inter-Area-Router, E-AS-External, E-Type-7 and E-Link have no producer, nothing validates an Extended LSA body before it is installed, and there is no migration or compatibility mode. The LS types Ze emits carry the U-bit clear where Section 2 requires it set, so they do not flood past an OSPFv3 router that does not support them; that one is a defect rather than a scope decision.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 38 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **38** | every gated MUST falls in exactly one bucket above |

**No test and no annotation (38):** [`RFC8362-2-1`](#rfc8362-2-1), [`RFC8362-3.1.1-1`](#rfc8362-3.1.1-1), [`RFC8362-3.2-1`](#rfc8362-3.2-1), [`RFC8362-3.3-1`](#rfc8362-3.3-1), [`RFC8362-3.4-1`](#rfc8362-3.4-1), [`RFC8362-3.5-1`](#rfc8362-3.5-1), [`RFC8362-3.6-1`](#rfc8362-3.6-1), [`RFC8362-3.7-1`](#rfc8362-3.7-1), [`RFC8362-3.8-1`](#rfc8362-3.8-1), [`RFC8362-3.9-1`](#rfc8362-3.9-1), [`RFC8362-3.10-1`](#rfc8362-3.10-1), [`RFC8362-3.10-2`](#rfc8362-3.10-2), [`RFC8362-3.11-1`](#rfc8362-3.11-1), [`RFC8362-3.11-2`](#rfc8362-3.11-2), [`RFC8362-3.12-1`](#rfc8362-3.12-1), [`RFC8362-4.2-1`](#rfc8362-4.2-1), [`RFC8362-4.3-1`](#rfc8362-4.3-1), [`RFC8362-4.3-2`](#rfc8362-4.3-2), [`RFC8362-4.4-1`](#rfc8362-4.4-1), [`RFC8362-4.4-2`](#rfc8362-4.4-2), [`RFC8362-4.5-1`](#rfc8362-4.5-1), [`RFC8362-4.5-2`](#rfc8362-4.5-2), [`RFC8362-4.7-1`](#rfc8362-4.7-1), [`RFC8362-4.7-2`](#rfc8362-4.7-2), [`RFC8362-4.7-3`](#rfc8362-4.7-3), [`RFC8362-4.8-1`](#rfc8362-4.8-1), [`RFC8362-5-1`](#rfc8362-5-1), [`RFC8362-5-2`](#rfc8362-5-2), [`RFC8362-6.2-1`](#rfc8362-6.2-1), [`RFC8362-6.3-1`](#rfc8362-6.3-1), [`RFC8362-6.3-2`](#rfc8362-6.3-2), [`RFC8362-6.3-3`](#rfc8362-6.3-3), [`RFC8362-6.3-4`](#rfc8362-6.3-4), [`RFC8362-6.3-5`](#rfc8362-6.3-5), [`RFC8362-8.1-1`](#rfc8362-8.1-1), [`RFC8362-8.1-2`](#rfc8362-8.1-2), [`RFC8362-8.2-1`](#rfc8362-8.2-1), [`RFC8362-8.2-2`](#rfc8362-8.2-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8362-2-1` | "For backward compatibility, the U-bit MUST be set in the LS Type so that the LSAs will be flooded by OSPFv3 routers that do not understand them." (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.1.1-1` | "If the N-bit is set and the PrefixLength is NOT 128 for the IPv6 Address Family or 32 for the IPv4 Address Family [OSPFV3-AF], the N-bit MUST be ignored." (§3.1.1) | MUST | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.2-1` | The Router-Link TLV is only applicable to the E-Router-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.3-1` | The Attached-Routers TLV is only applicable to the E-Network-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.4-1` | The Inter-Area-Prefix TLV is only applicable to the E-Inter-Area-Prefix-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.5-1` | The Inter-Area-Router TLV is only applicable to the E-Inter-Area-Router-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.6-1` | The External-Prefix TLV is only applicable to the E-AS-External-LSA and the E-NSSA-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.7-1` | The Intra-Area-Prefix TLV is only applicable to the E-Link-LSA and the E-Intra-Area-Prefix-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.8-1` | The IPv6 Link-Local Address TLV is only applicable to the E-Link-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.8) | MUST | 3.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.9-1` | The IPv4 Link-Local Address TLV is only applicable to the E-Link-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.9) | MUST | 3.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.10-1` | Of the IPv6-Forwarding-Address sub-TLV, "the first specified instance is used as the forwarding address as defined in [OSPFV3]. Instances subsequent to the first MUST be ignored." (§3.10) | MUST | 3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.10-2` | "The IPv6-Forwarding-Address TLV is to be used with IPv6 address families as defined in [OSPFV3-AF]. It MUST be ignored for other address families." (§3.10) | MUST | 3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.11-1` | Of the IPv4-Forwarding-Address sub-TLV, "the first specified instance is used as the forwarding address as defined in [OSPFV3]. Instances subsequent to the first MUST be ignored." (§3.11) | MUST | 3.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.11-2` | "The IPv4-Forwarding-Address TLV is to be used with IPv4 address families as defined in [OSPFV3-AF]. It MUST be ignored for other address families." (§3.11) | MUST | 3.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.12-1` | Of the Route-Tag sub-TLV, "the first specified instance is used as the Route Tag as defined in [OSPFV3]. Instances subsequent to the first MUST be ignored." (§3.12) | MUST | 3.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.2-1` | In the E-Network-LSA, "Instances of the Attached-Router TLV subsequent to the first MUST be ignored." (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.3-1` | "In order to retain compatibility and semantics with the current OSPFv3 specification, each Inter-Area-Prefix LSA MUST contain a single Inter-Area-Prefix TLV." (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.3-2` | In the E-Inter-Area-Prefix-LSA, "Instances of the Inter-Area-Prefix TLV subsequent to the first MUST be ignored." (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.4-1` | "In order to retain compatibility and semantics with the current OSPFv3 specification, each Inter-Area-Router-LSA MUST contain a single Inter-Area-Router TLV." (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.4-2` | In the E-Inter-Area-Router-LSA, "Instances of the Inter-Area-Router TLV subsequent to the first MUST be ignored." (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.5-1` | For the E-AS-External-LSA, "In order to retain compatibility and semantics with the current OSPFv3 specification, each LSA MUST contain a single External-Prefix TLV." (§4.5) | MUST | 4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.5-2` | In the E-AS-External-LSA, "Instances of the External-Prefix TLV subsequent to the first MUST be ignored." (§4.5) | MUST | 4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.7-1` | Of the IPv6 Link-Local Address TLV in the E-Link-LSA, "Instances following the first MUST be ignored." (§4.7) | MUST | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.7-2` | Of the IPv6 Link-Local Address TLV, "For IPv4 address families as defined in [OSPFV3-AF], this TLV MUST be ignored." (§4.7) | MUST | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.7-3` | Of the IPv4 Link-Local Address TLV in the E-Link-LSA, "Instances following the first MUST be ignored." (§4.7) | MUST | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.8-1` | For the E-Intra-Area-Prefix-LSA, "The Referenced LS Type MUST be either an E-Router-LSA (0xA021) or an E-Network-LSA (0xA022)." (§4.8) | MUST | 4.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-5-1` | "Extended LSAs that have inconsistent length or other encoding errors, as described herein, MUST NOT be installed in the Link State Database, acknowledged, or flooded." (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-5-2` | "Additionally, an LSA MUST be considered malformed if it does not include all of the required TLVs and sub-TLVs." (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-6.2-1` | In sparse-mode, "if a top-level TLV is advertised, it MUST include required sub-TLVs, or it will be considered malformed as described in Section 5." (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-6.3-1` | All implementations MUST adhere to the TLV processing rules, of which rule 1 is: "Unrecognized TLVs and sub-TLVs are ignored when parsing or processing Extended LSAs." (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-6.3-2` | "Whether or not partial deployment of a given TLV is supported MUST be specified." (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-6.3-3` | "If partial deployment is not supported, mechanisms to ensure the corresponding feature is not deployed MUST be specified in the document defining the new TLV or sub-TLV." (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-6.3-4` | "If partial deployment is supported, backward compatibility and partial deployment MUST be specified in the document defining the new TLV or sub-TLV." (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-6.3-5` | "Documents specifying future TLVs or Sub-TLVs MUST specify the requirements for usage of those TLVs or sub-TLVs." (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-8.1-1` | Top-level TLV types 32768-33023 are reserved for experimental use; "these will not be registered with IANA and MUST NOT be mentioned by RFCs." (§8.1) | MUST NOT | 8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-8.1-2` | For top-level TLV types, "Before any assignments can be made in the 33024-65535 range, there MUST be an IETF specification that specifies IANA Considerations that cover the range being assigned." (§8.1) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-8.2-1` | Sub-TLV types 32768-33023 are reserved for experimental use; "these will not be registered with IANA and MUST NOT be mentioned by RFCs." (§8.2) | MUST NOT | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-8.2-2` | For sub-TLV types, "Before any assignments can be made in the 33024-65535 range, there MUST be an IETF specification that specifies IANA Considerations that cover the range being assigned." (§8.2) | MUST | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.1-1` | The applicability of the LA-bit is expanded, and "it SHOULD be set in Inter-Area-Prefix TLVs ... when the advertised host IPv6 address ... is an interface address." (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.7-4` | "A single instance of the IPv6 Link-Local Address TLV (Section 3.8) SHOULD be included in the E-Link-LSA." (§4.7) | SHOULD | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.7-5` | "Similarly, only a single instance of the IPv4 Link-Local Address TLV (Section 3.9) SHOULD be included in the E-Link-LSA." (§4.7) | SHOULD | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-4.7-6` | Of the IPv4 Link-Local Address TLV, "For OSPFv3 IPv6 address families as defined in [OSPFV3-AF], this TLV SHOULD be ignored." (§4.7) | SHOULD | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-5-3` | "Reception of malformed LSAs SHOULD be counted and/or logged for examination by the administrator of the OSPFv3 routing domain." (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-6-1` | For future TLV-based OSPFv3 LSA extensions, "Both full and, if applicable, partial deployment SHOULD be specified". (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-6.3-6` | "If a TLV or sub-TLV is recognized but the length is less than the minimum, then the LSA should be considered malformed, and it SHOULD NOT be acknowledged." (§6.3) | SHOULD NOT | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-6.3-7` | "Additionally, the occurrence SHOULD be logged with enough information to identify the LSA by type, Link State ID, originator, and sequence number and identify the TLV or sub-TLV in error." (§6.3) | SHOULD | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-B-1` | "disabling AreaExtendedLSASupport for a regular OSPFv3 area (not a Stub or NSSA area) when ExtendedLSASupport is enabled is contradictory and SHOULD be prohibited by implementations." (§B) | SHOULD | B | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3-1` | "In general, TLVs and sub-TLVs MAY occur in any order, and the specification should define whether the TLV or sub-TLV is required and the behavior when there are multiple occurrences of the TLV or sub-TLV." (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.1-2` | The LA-bit "MAY be set in External-Prefix TLVs when the advertised host IPv6 address ... is an interface address." (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8362-3.1.1-2` | "The advertising router MAY choose NOT to set the N-bit even when the above conditions are met." (§3.1.1) | MAY | 3.1.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8362-2-1`](#rfc8362-2-1) "For backward compatibility, the U-bit MUST be set in the LS Type so that the LSAs will be flooded by OSPFv3 routers that do not understand them." (§2) | no test | no test carries this requirement id |
| [`RFC8362-3.1.1-1`](#rfc8362-3.1.1-1) "If the N-bit is set and the PrefixLength is NOT 128 for the IPv6 Address Family or 32 for the IPv4 Address Family [OSPFV3-AF], the N-bit MUST be ignored." (§3.1.1) | no test | no test carries this requirement id |
| [`RFC8362-3.2-1`](#rfc8362-3.2-1) The Router-Link TLV is only applicable to the E-Router-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.2) | no test | no test carries this requirement id |
| [`RFC8362-3.3-1`](#rfc8362-3.3-1) The Attached-Routers TLV is only applicable to the E-Network-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.3) | no test | no test carries this requirement id |
| [`RFC8362-3.4-1`](#rfc8362-3.4-1) The Inter-Area-Prefix TLV is only applicable to the E-Inter-Area-Prefix-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.4) | no test | no test carries this requirement id |
| [`RFC8362-3.5-1`](#rfc8362-3.5-1) The Inter-Area-Router TLV is only applicable to the E-Inter-Area-Router-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.5) | no test | no test carries this requirement id |
| [`RFC8362-3.6-1`](#rfc8362-3.6-1) The External-Prefix TLV is only applicable to the E-AS-External-LSA and the E-NSSA-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.6) | no test | no test carries this requirement id |
| [`RFC8362-3.7-1`](#rfc8362-3.7-1) The Intra-Area-Prefix TLV is only applicable to the E-Link-LSA and the E-Intra-Area-Prefix-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.7) | no test | no test carries this requirement id |
| [`RFC8362-3.8-1`](#rfc8362-3.8-1) The IPv6 Link-Local Address TLV is only applicable to the E-Link-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.8) | no test | no test carries this requirement id |
| [`RFC8362-3.9-1`](#rfc8362-3.9-1) The IPv4 Link-Local Address TLV is only applicable to the E-Link-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.9) | no test | no test carries this requirement id |
| [`RFC8362-3.10-1`](#rfc8362-3.10-1) Of the IPv6-Forwarding-Address sub-TLV, "the first specified instance is used as the forwarding address as defined in [OSPFV3]. Instances subsequent to the first MUST be ignored." (§3.10) | no test | no test carries this requirement id |
| [`RFC8362-3.10-2`](#rfc8362-3.10-2) "The IPv6-Forwarding-Address TLV is to be used with IPv6 address families as defined in [OSPFV3-AF]. It MUST be ignored for other address families." (§3.10) | no test | no test carries this requirement id |
| [`RFC8362-3.11-1`](#rfc8362-3.11-1) Of the IPv4-Forwarding-Address sub-TLV, "the first specified instance is used as the forwarding address as defined in [OSPFV3]. Instances subsequent to the first MUST be ignored." (§3.11) | no test | no test carries this requirement id |
| [`RFC8362-3.11-2`](#rfc8362-3.11-2) "The IPv4-Forwarding-Address TLV is to be used with IPv4 address families as defined in [OSPFV3-AF]. It MUST be ignored for other address families." (§3.11) | no test | no test carries this requirement id |
| [`RFC8362-3.12-1`](#rfc8362-3.12-1) Of the Route-Tag sub-TLV, "the first specified instance is used as the Route Tag as defined in [OSPFV3]. Instances subsequent to the first MUST be ignored." (§3.12) | no test | no test carries this requirement id |
| [`RFC8362-4.2-1`](#rfc8362-4.2-1) In the E-Network-LSA, "Instances of the Attached-Router TLV subsequent to the first MUST be ignored." (§4.2) | no test | no test carries this requirement id |
| [`RFC8362-4.3-1`](#rfc8362-4.3-1) "In order to retain compatibility and semantics with the current OSPFv3 specification, each Inter-Area-Prefix LSA MUST contain a single Inter-Area-Prefix TLV." (§4.3) | no test | no test carries this requirement id |
| [`RFC8362-4.3-2`](#rfc8362-4.3-2) In the E-Inter-Area-Prefix-LSA, "Instances of the Inter-Area-Prefix TLV subsequent to the first MUST be ignored." (§4.3) | no test | no test carries this requirement id |
| [`RFC8362-4.4-1`](#rfc8362-4.4-1) "In order to retain compatibility and semantics with the current OSPFv3 specification, each Inter-Area-Router-LSA MUST contain a single Inter-Area-Router TLV." (§4.4) | no test | no test carries this requirement id |
| [`RFC8362-4.4-2`](#rfc8362-4.4-2) In the E-Inter-Area-Router-LSA, "Instances of the Inter-Area-Router TLV subsequent to the first MUST be ignored." (§4.4) | no test | no test carries this requirement id |
| [`RFC8362-4.5-1`](#rfc8362-4.5-1) For the E-AS-External-LSA, "In order to retain compatibility and semantics with the current OSPFv3 specification, each LSA MUST contain a single External-Prefix TLV." (§4.5) | no test | no test carries this requirement id |
| [`RFC8362-4.5-2`](#rfc8362-4.5-2) In the E-AS-External-LSA, "Instances of the External-Prefix TLV subsequent to the first MUST be ignored." (§4.5) | no test | no test carries this requirement id |
| [`RFC8362-4.7-1`](#rfc8362-4.7-1) Of the IPv6 Link-Local Address TLV in the E-Link-LSA, "Instances following the first MUST be ignored." (§4.7) | no test | no test carries this requirement id |
| [`RFC8362-4.7-2`](#rfc8362-4.7-2) Of the IPv6 Link-Local Address TLV, "For IPv4 address families as defined in [OSPFV3-AF], this TLV MUST be ignored." (§4.7) | no test | no test carries this requirement id |
| [`RFC8362-4.7-3`](#rfc8362-4.7-3) Of the IPv4 Link-Local Address TLV in the E-Link-LSA, "Instances following the first MUST be ignored." (§4.7) | no test | no test carries this requirement id |
| [`RFC8362-4.8-1`](#rfc8362-4.8-1) For the E-Intra-Area-Prefix-LSA, "The Referenced LS Type MUST be either an E-Router-LSA (0xA021) or an E-Network-LSA (0xA022)." (§4.8) | no test | no test carries this requirement id |
| [`RFC8362-5-1`](#rfc8362-5-1) "Extended LSAs that have inconsistent length or other encoding errors, as described herein, MUST NOT be installed in the Link State Database, acknowledged, or flooded." (§5) | no test | no test carries this requirement id |
| [`RFC8362-5-2`](#rfc8362-5-2) "Additionally, an LSA MUST be considered malformed if it does not include all of the required TLVs and sub-TLVs." (§5) | no test | no test carries this requirement id |
| [`RFC8362-6.2-1`](#rfc8362-6.2-1) In sparse-mode, "if a top-level TLV is advertised, it MUST include required sub-TLVs, or it will be considered malformed as described in Section 5." (§6.2) | no test | no test carries this requirement id |
| [`RFC8362-6.3-1`](#rfc8362-6.3-1) All implementations MUST adhere to the TLV processing rules, of which rule 1 is: "Unrecognized TLVs and sub-TLVs are ignored when parsing or processing Extended LSAs." (§6.3) | no test | no test carries this requirement id |
| [`RFC8362-6.3-2`](#rfc8362-6.3-2) "Whether or not partial deployment of a given TLV is supported MUST be specified." (§6.3) | no test | no test carries this requirement id |
| [`RFC8362-6.3-3`](#rfc8362-6.3-3) "If partial deployment is not supported, mechanisms to ensure the corresponding feature is not deployed MUST be specified in the document defining the new TLV or sub-TLV." (§6.3) | no test | no test carries this requirement id |
| [`RFC8362-6.3-4`](#rfc8362-6.3-4) "If partial deployment is supported, backward compatibility and partial deployment MUST be specified in the document defining the new TLV or sub-TLV." (§6.3) | no test | no test carries this requirement id |
| [`RFC8362-6.3-5`](#rfc8362-6.3-5) "Documents specifying future TLVs or Sub-TLVs MUST specify the requirements for usage of those TLVs or sub-TLVs." (§6.3) | no test | no test carries this requirement id |
| [`RFC8362-8.1-1`](#rfc8362-8.1-1) Top-level TLV types 32768-33023 are reserved for experimental use; "these will not be registered with IANA and MUST NOT be mentioned by RFCs." (§8.1) | no test | no test carries this requirement id |
| [`RFC8362-8.1-2`](#rfc8362-8.1-2) For top-level TLV types, "Before any assignments can be made in the 33024-65535 range, there MUST be an IETF specification that specifies IANA Considerations that cover the range being assigned." (§8.1) | no test | no test carries this requirement id |
| [`RFC8362-8.2-1`](#rfc8362-8.2-1) Sub-TLV types 32768-33023 are reserved for experimental use; "these will not be registered with IANA and MUST NOT be mentioned by RFCs." (§8.2) | no test | no test carries this requirement id |
| [`RFC8362-8.2-2`](#rfc8362-8.2-2) For sub-TLV types, "Before any assignments can be made in the 33024-65535 range, there MUST be an IETF specification that specifies IANA Considerations that cover the range being assigned." (§8.2) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8362-2-1`](#rfc8362-2-1)

"For backward compatibility, the U-bit MUST be set in the LS Type so that the LSAs will be flooded by OSPFv3 routers that do not understand them." (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-2-1, so no unit is bound to it.

### [`RFC8362-3.1.1-1`](#rfc8362-3.1.1-1)

"If the N-bit is set and the PrefixLength is NOT 128 for the IPv6 Address Family or 32 for the IPv4 Address Family [OSPFV3-AF], the N-bit MUST be ignored." (§3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-3.1.1-1, so no unit is bound to it.

### [`RFC8362-3.2-1`](#rfc8362-3.2-1)

The Router-Link TLV is only applicable to the E-Router-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-3.2-1, so no unit is bound to it.

### [`RFC8362-3.3-1`](#rfc8362-3.3-1)

The Attached-Routers TLV is only applicable to the E-Network-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-3.3-1, so no unit is bound to it.

### [`RFC8362-3.4-1`](#rfc8362-3.4-1)

The Inter-Area-Prefix TLV is only applicable to the E-Inter-Area-Prefix-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-3.4-1, so no unit is bound to it.

### [`RFC8362-3.5-1`](#rfc8362-3.5-1)

The Inter-Area-Router TLV is only applicable to the E-Inter-Area-Router-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-3.5-1, so no unit is bound to it.

### [`RFC8362-3.6-1`](#rfc8362-3.6-1)

The External-Prefix TLV is only applicable to the E-AS-External-LSA and the E-NSSA-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-3.6-1, so no unit is bound to it.

### [`RFC8362-3.7-1`](#rfc8362-3.7-1)

The Intra-Area-Prefix TLV is only applicable to the E-Link-LSA and the E-Intra-Area-Prefix-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-3.7-1, so no unit is bound to it.

### [`RFC8362-3.8-1`](#rfc8362-3.8-1)

The IPv6 Link-Local Address TLV is only applicable to the E-Link-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-3.8-1, so no unit is bound to it.

### [`RFC8362-3.9-1`](#rfc8362-3.9-1)

The IPv4 Link-Local Address TLV is only applicable to the E-Link-LSA: "Inclusion in other Extended LSAs MUST be ignored." (§3.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-3.9-1, so no unit is bound to it.

### [`RFC8362-3.10-1`](#rfc8362-3.10-1)

Of the IPv6-Forwarding-Address sub-TLV, "the first specified instance is used as the forwarding address as defined in [OSPFV3]. Instances subsequent to the first MUST be ignored." (§3.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-3.10-1, so no unit is bound to it.

### [`RFC8362-3.10-2`](#rfc8362-3.10-2)

"The IPv6-Forwarding-Address TLV is to be used with IPv6 address families as defined in [OSPFV3-AF]. It MUST be ignored for other address families." (§3.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-3.10-2, so no unit is bound to it.

### [`RFC8362-3.11-1`](#rfc8362-3.11-1)

Of the IPv4-Forwarding-Address sub-TLV, "the first specified instance is used as the forwarding address as defined in [OSPFV3]. Instances subsequent to the first MUST be ignored." (§3.11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-3.11-1, so no unit is bound to it.

### [`RFC8362-3.11-2`](#rfc8362-3.11-2)

"The IPv4-Forwarding-Address TLV is to be used with IPv4 address families as defined in [OSPFV3-AF]. It MUST be ignored for other address families." (§3.11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-3.11-2, so no unit is bound to it.

### [`RFC8362-3.12-1`](#rfc8362-3.12-1)

Of the Route-Tag sub-TLV, "the first specified instance is used as the Route Tag as defined in [OSPFV3]. Instances subsequent to the first MUST be ignored." (§3.12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-3.12-1, so no unit is bound to it.

### [`RFC8362-4.2-1`](#rfc8362-4.2-1)

In the E-Network-LSA, "Instances of the Attached-Router TLV subsequent to the first MUST be ignored." (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-4.2-1, so no unit is bound to it.

### [`RFC8362-4.3-1`](#rfc8362-4.3-1)

"In order to retain compatibility and semantics with the current OSPFv3 specification, each Inter-Area-Prefix LSA MUST contain a single Inter-Area-Prefix TLV." (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-4.3-1, so no unit is bound to it.

### [`RFC8362-4.3-2`](#rfc8362-4.3-2)

In the E-Inter-Area-Prefix-LSA, "Instances of the Inter-Area-Prefix TLV subsequent to the first MUST be ignored." (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-4.3-2, so no unit is bound to it.

### [`RFC8362-4.4-1`](#rfc8362-4.4-1)

"In order to retain compatibility and semantics with the current OSPFv3 specification, each Inter-Area-Router-LSA MUST contain a single Inter-Area-Router TLV." (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-4.4-1, so no unit is bound to it.

### [`RFC8362-4.4-2`](#rfc8362-4.4-2)

In the E-Inter-Area-Router-LSA, "Instances of the Inter-Area-Router TLV subsequent to the first MUST be ignored." (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-4.4-2, so no unit is bound to it.

### [`RFC8362-4.5-1`](#rfc8362-4.5-1)

For the E-AS-External-LSA, "In order to retain compatibility and semantics with the current OSPFv3 specification, each LSA MUST contain a single External-Prefix TLV." (§4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-4.5-1, so no unit is bound to it.

### [`RFC8362-4.5-2`](#rfc8362-4.5-2)

In the E-AS-External-LSA, "Instances of the External-Prefix TLV subsequent to the first MUST be ignored." (§4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-4.5-2, so no unit is bound to it.

### [`RFC8362-4.7-1`](#rfc8362-4.7-1)

Of the IPv6 Link-Local Address TLV in the E-Link-LSA, "Instances following the first MUST be ignored." (§4.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-4.7-1, so no unit is bound to it.

### [`RFC8362-4.7-2`](#rfc8362-4.7-2)

Of the IPv6 Link-Local Address TLV, "For IPv4 address families as defined in [OSPFV3-AF], this TLV MUST be ignored." (§4.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-4.7-2, so no unit is bound to it.

### [`RFC8362-4.7-3`](#rfc8362-4.7-3)

Of the IPv4 Link-Local Address TLV in the E-Link-LSA, "Instances following the first MUST be ignored." (§4.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-4.7-3, so no unit is bound to it.

### [`RFC8362-4.8-1`](#rfc8362-4.8-1)

For the E-Intra-Area-Prefix-LSA, "The Referenced LS Type MUST be either an E-Router-LSA (0xA021) or an E-Network-LSA (0xA022)." (§4.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-4.8-1, so no unit is bound to it.

### [`RFC8362-5-1`](#rfc8362-5-1)

"Extended LSAs that have inconsistent length or other encoding errors, as described herein, MUST NOT be installed in the Link State Database, acknowledged, or flooded." (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-5-1, so no unit is bound to it.

### [`RFC8362-5-2`](#rfc8362-5-2)

"Additionally, an LSA MUST be considered malformed if it does not include all of the required TLVs and sub-TLVs." (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-5-2, so no unit is bound to it.

### [`RFC8362-6.2-1`](#rfc8362-6.2-1)

In sparse-mode, "if a top-level TLV is advertised, it MUST include required sub-TLVs, or it will be considered malformed as described in Section 5." (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-6.2-1, so no unit is bound to it.

### [`RFC8362-6.3-1`](#rfc8362-6.3-1)

All implementations MUST adhere to the TLV processing rules, of which rule 1 is: "Unrecognized TLVs and sub-TLVs are ignored when parsing or processing Extended LSAs." (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-6.3-1, so no unit is bound to it.

### [`RFC8362-6.3-2`](#rfc8362-6.3-2)

"Whether or not partial deployment of a given TLV is supported MUST be specified." (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-6.3-2, so no unit is bound to it.

### [`RFC8362-6.3-3`](#rfc8362-6.3-3)

"If partial deployment is not supported, mechanisms to ensure the corresponding feature is not deployed MUST be specified in the document defining the new TLV or sub-TLV." (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-6.3-3, so no unit is bound to it.

### [`RFC8362-6.3-4`](#rfc8362-6.3-4)

"If partial deployment is supported, backward compatibility and partial deployment MUST be specified in the document defining the new TLV or sub-TLV." (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-6.3-4, so no unit is bound to it.

### [`RFC8362-6.3-5`](#rfc8362-6.3-5)

"Documents specifying future TLVs or Sub-TLVs MUST specify the requirements for usage of those TLVs or sub-TLVs." (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-6.3-5, so no unit is bound to it.

### [`RFC8362-8.1-1`](#rfc8362-8.1-1)

Top-level TLV types 32768-33023 are reserved for experimental use; "these will not be registered with IANA and MUST NOT be mentioned by RFCs." (§8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-8.1-1, so no unit is bound to it.

### [`RFC8362-8.1-2`](#rfc8362-8.1-2)

For top-level TLV types, "Before any assignments can be made in the 33024-65535 range, there MUST be an IETF specification that specifies IANA Considerations that cover the range being assigned." (§8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-8.1-2, so no unit is bound to it.

### [`RFC8362-8.2-1`](#rfc8362-8.2-1)

Sub-TLV types 32768-33023 are reserved for experimental use; "these will not be registered with IANA and MUST NOT be mentioned by RFCs." (§8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-8.2-1, so no unit is bound to it.

### [`RFC8362-8.2-2`](#rfc8362-8.2-2)

For sub-TLV types, "Before any assignments can be made in the 33024-65535 range, there MUST be an IETF specification that specifies IANA Considerations that cover the range being assigned." (§8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8362-8.2-2, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 8362, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 8362, so its obligations are stated where they were written.
