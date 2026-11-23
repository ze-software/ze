# RFC 9252 - BGP Overlay Services Based on Segment Routing over IPv6 (SRv6)

BGP SRv6 Service TLV (Prefix-SID). Every requirement listed for RFC 9252, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 14 | of 25 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 5 | of 25 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 25 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Tests shown to detect a fault | 48 | 48 of 64 tests linked to requirements, 0 without a fault test and 5 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Partial test coverage | 3 | of 25 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 3 | of 25 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Audit verdicts | 18 | of 25 mandatory requirements reviewed | 3 weak, wrong or unimplemented, 3 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 25 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 25 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 25 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 25 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 25 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 25 | of 30 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 25 mandatory requirements: they add up to 25. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | BGP SRv6 Service TLV (Prefix-SID) |
| Automatic checks | Included |
| Requirements | 30 |
| Mandatory requirements (MUST level) | 25 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 3 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 64 |
| Tests linked to requirements | 64 |
| Recorded audit verdicts | 18 |
| Fault test records | 53 |
| Summary | `rfc/short/rfc9252.md` |
| Requirement table | `rfc/requirements/rfc9252.md` |
| RFC text | `rfc/full/rfc9252.txt` |

## Automatic checks

Included in automatic checks: SRv6 BGP Overlay Services / Prefix-SID Service TLV (RFC 9252): receive-side SRv6 L3/L2 Service TLV codec and forwarding treatment based on the final effective next-hop identity. Unchanged identity preserves received Service TLVs; changed identity removes them while retaining unrelated top-level TLVs. The changed-next-hop obligation remains partial because locally allocated SID information is not produced to rebuild the service information. Remaining obligations are bounded by the requirement rows, not by a codec-wide conformance claim. The 2026-09-21 extraction walk read the document against this checklist and changed two things. It corrected the section reference on 21 rows, which carried the draft's numbering (S3.1, S3.2, S3.3, S3.4, S4.1, S5, S6.1, S6.2) rather than the published one: the Service TLV Reserved field is Section 2, the SID Information Sub-TLV is Section 3.1, the argument-validation obligations are Section 3.2.1, the VPN transposition bound is Sections 5.1 and 5.2, the EVPN bounds are Sections 6.1.1 through 6.5, and error handling is Section 7. And it added 6 MUST rows the checklist did not carry: RFC9252-3.2.1-4 (shifted-out bits zeroed in the SID value), RFC9252-3.2.1-5 (per-neighbor and per-service advertisement control), RFC9252-3.2.1-6 (AL zero where the Argument does not apply), RFC9252-5-3 (ingress PE encapsulates in IPv6 and inserts an SRH when required), and RFC9252-7-1 and -7-2 (a second SRv6 L3 or L2 Service TLV is ignored). None of the 6 carries a tagged test, so each is an open gap.

## Published support status

**Area:** BGP SRv6 Service TLV (Prefix-SID)

**Test verification:** 14 of 25 MUST proven

**Evidence:** functional 4, unit 10

**Unmet:** RFC9252-3.2-6, RFC9252-3.3-2, RFC9252-4.1-1, RFC9252-5-2, RFC9252-6.1-1, RFC9252-6.2-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 14 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 11 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 3 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **25** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (14):** [`RFC9252-3.1-2`](#rfc9252-3.1-2), [`RFC9252-3.2.1-1`](#rfc9252-3.2.1-1), [`RFC9252-3.2.1-2`](#rfc9252-3.2.1-2), [`RFC9252-3.2.1-3`](#rfc9252-3.2.1-3), [`RFC9252-3.2-5`](#rfc9252-3.2-5), [`RFC9252-5-1`](#rfc9252-5-1), [`RFC9252-3.3-1`](#rfc9252-3.3-1), [`RFC9252-3.4-1`](#rfc9252-3.4-1), [`RFC9252-3.2.1-4`](#rfc9252-3.2.1-4), [`RFC9252-3.2.1-5`](#rfc9252-3.2.1-5), [`RFC9252-3.2.1-6`](#rfc9252-3.2.1-6), [`RFC9252-5-3`](#rfc9252-5-3), [`RFC9252-7-1`](#rfc9252-7-1), [`RFC9252-7-2`](#rfc9252-7-2)

**Annotated (including scoped evidence) (11):** [`RFC9252-3.1-1`](#rfc9252-3.1-1), [`RFC9252-3.2-1`](#rfc9252-3.2-1), [`RFC9252-3.2-2`](#rfc9252-3.2-2), [`RFC9252-3.2-3`](#rfc9252-3.2-3), [`RFC9252-4.1-1`](#rfc9252-4.1-1), [`RFC9252-6.1-1`](#rfc9252-6.1-1), [`RFC9252-6.2-1`](#rfc9252-6.2-1), [`RFC9252-3.2-4`](#rfc9252-3.2-4), [`RFC9252-3.2-6`](#rfc9252-3.2-6), [`RFC9252-5-2`](#rfc9252-5-2), [`RFC9252-3.3-2`](#rfc9252-3.3-2)

**Partial test coverage (subset of annotated; zero whole-requirement credit) (3):** [`RFC9252-4.1-1`](#rfc9252-4.1-1), [`RFC9252-6.2-1`](#rfc9252-6.2-1), [`RFC9252-3.3-2`](#rfc9252-3.3-2)

## Figures

- **MUST bar**: Ze verified 14, Tests awaiting verification 5, Not built 6
- **Overview bar**: Ze MUST verified 14, Tests awaiting verification 5, Not assessed 5, Not built 6

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 25 |
| MUST rows kept (the MUST bar) | 25 |
| Ze verified | 14 |
| OS verified | 0 |
| Tests awaiting verification | 5 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 6 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 5 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 5 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc9252.json](../../../data/rfc/rfc9252.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9252-3.1-1` | This field is reserved; it MUST be set to 0 by the sender (§2) | MUST | 2 | **positive:** `unit/verify` [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L163). **negative:** no negative test. **{single-polarity}:** ParsePrefixSIDSRv6 hardcodes the Service TLV Reserved octet to 0 on encode and no code path emits a non-zero value, so there is no negative input to reject (internal/core/bgp/attribute/prefixsid.go) |
| `RFC9252-3.1-2` | This field is reserved; it MUST be set to 0 by the sender and ignored by the receiver. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestExtractSRv6SID_ServiceReservedZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L257). **negative:** `unit/verify` [`TestExtractSRv6SID_ServiceReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L271) |
| `RFC9252-3.2-1` | RESERVED1 (1 octet): This field MUST be set to 0 by the sender (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L164). **negative:** no negative test. **{single-polarity}:** ParsePrefixSIDSRv6 hardcodes RESERVED1 to 0 on encode with no non-zero path to reject (internal/core/bgp/attribute/prefixsid.go) |
| `RFC9252-3.2-2` | This field encodes SRv6 Service SID Flags -- none are currently defined. It MUST be set to 0 by the sender (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L165). **negative:** no negative test. **{single-polarity}:** ParsePrefixSIDSRv6 hardcodes the Service SID Flags octet to 0 on encode with no non-zero path to reject (internal/core/bgp/attribute/prefixsid.go) |
| `RFC9252-3.2-3` | RESERVED2 (1 octet): This field MUST be set to 0 by the sender (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L166). **negative:** no negative test. **{single-polarity}:** ParsePrefixSIDSRv6 hardcodes RESERVED2 to 0 on encode with no non-zero path to reject (internal/core/bgp/attribute/prefixsid.go) |
| `RFC9252-3.2.1-1` | In this case, the Transposition Offset MUST be set to 0. (S3.2.1) | MUST | 3.2.1 | **positive:** `unit/verify` [`TestExtractSRv6SIDFull_ZeroLengthNonZeroOffset`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_sid_structure_validity_test.go#L60). **negative:** `unit/verify` [`TestExtractSRv6SIDFull_ZeroLengthNonZeroOffset`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_sid_structure_validity_test.go#L59) |
| `RFC9252-3.2.1-2` | The transposition offset and length MUST be 0 when the Sub-Sub-TLV is advertised along with routes where the Transposition Scheme is not applicable (e.g., for global IPv6 service [RFC2545] where there is no label field). (§7) | MUST | 7 | **positive:** `unit/verify` [`TestCheckSRv6TranspositionForFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9252_prefixsid_origin_test.go#L23). **positive:** `unit/verify` [`TestSRv6TranspositionOnFamilyWithoutLabelIsIneligible`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9252_srv6_admission_test.go#L105). **positive:** `unit/verify` [`TestUpdateTextSRv6TranspositionNeedsALabelField`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc9252_update_text_srv6_transposition_test.go#L26). **negative:** `unit/verify` [`TestCheckSRv6TranspositionForFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9252_prefixsid_origin_test.go#L24). **negative:** `unit/verify` [`TestSRv6TranspositionOnFamilyWithoutLabelIsIneligible`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9252_srv6_admission_test.go#L104). **negative:** `unit/verify` [`TestUpdateTextSRv6TranspositionNeedsALabelField`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc9252_update_text_srv6_transposition_test.go#L27). **negative:** `functional/verify` [`srv6-transposition-refused-unicast.ci`](https://github.com/ze-software/ze/blob/main/test/parse/srv6-transposition-refused-unicast.ci#L12) |
| `RFC9252-3.2.1-3` | As defined in [RFC8986], the sum of the Locator Block Length (LBL), Locator Node Length (LNL), Function Length (FL), and Argument Length (AL) fields MUST be less than or equal to 128 and greater than or equal to the sum of Transposition Offset and Transposition Length. (S3.2.1, errata 7817) | MUST | 3.2.1 | **positive:** `unit/verify` [`TestExtractSRv6SIDFull_Sum128IsValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_sid_structure_validity_test.go#L104). **positive:** `unit/verify` [`TestExtractSRv6SIDFull_WithTransposition`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L139). **negative:** `unit/verify` [`TestExtractSRv6SIDFull_InvalidSIDStructure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L217). **negative:** `unit/verify` [`TestExtractSRv6SIDFull_SIDStructureLengthNotSix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_sid_structure_validity_test.go#L36). **negative:** `unit/verify` [`TestExtractSRv6SIDFull_SumBelowTransposition`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L237) |
| `RFC9252-4.1-1` | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 20 and less than or equal to the FL. (§5.1, §5.2) | MUST | 5.1 | **positive:** `unit/verify` [`TestCheckSRv6TranspositionLabelWidth`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9252_prefixsid_origin_test.go#L78). **negative:** `unit/verify` [`TestCheckSRv6TranspositionLabelWidth`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9252_prefixsid_origin_test.go#L79). **{partial}:** tested "the Transposition Length MUST be less than or equal to 20"; gap "less than or equal to the FL"; the 20-bit half is enforced on receive, where srv6SIDFromResult refuses to reconstruct and isSRv6Ineligible makes the path ineligible when Transposition Length exceeds nlrisplit.TranspositionLabelWidth (internal/component/bgp/plugins/rib/rib_bestchange.go), and on origination, where internal/core/bgp/attribute/prefixsid.go::CheckSRv6TranspositionForFamily refuses it; neither side nor parseSIDStructure bounds it against the Function Length (internal/component/bgp/plugins/rib/pool/srv6sid.go). **Partial test coverage:** Tests and fault test records cover only the part marked Tested; they do not verify the whole requirement. not reviewed: no reviewer has checked whether these tests enforce the requirement |
| `RFC9252-6.1-1` | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the AL. (§6.1.1) | MUST | 6.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the 24-bit half is enforced by nlrisplit.TranspositionLabelWidth through srv6SIDFromResult and isSRv6Ineligible (internal/component/bgp/plugins/rib/rib_bestchange.go), but the Argument Length is never bounded, the ESI Label extended community that carries these bits is never read, and the EVPN encoder carries no SRv6 ESI-label SID (internal/component/bgp/plugins/rib/pool/srv6sid.go) |
| `RFC9252-6.2-1` | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. (§6.2, §6.3, §6.5) | MUST | 6.2 | **positive:** `unit/verify` [`TestCheckSRv6TranspositionLabelWidth`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9252_prefixsid_origin_test.go#L80). **negative:** `unit/verify` [`TestCheckSRv6TranspositionLabelWidth`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9252_prefixsid_origin_test.go#L81). **{partial}:** tested "the Transposition Length MUST be less than or equal to 24"; gap "less than or equal to the FL"; the 24-bit half is enforced by nlrisplit.TranspositionLabelWidth through srv6SIDFromResult and isSRv6Ineligible on receive (internal/component/bgp/plugins/rib/rib_bestchange.go) and through internal/core/bgp/attribute/prefixsid.go::CheckSRv6TranspositionForFamily on origination, but the Function Length is never bounded and no EVPN label field is read at all -- TranspositionLabel answers only the VPN families, so an EVPN transposition yields no SID rather than a reconstructed one (internal/core/bgp/nlri/nlrisplit/transposition.go). **Partial test coverage:** Tests and fault test records cover only the part marked Tested; they do not verify the whole requirement. not reviewed: no reviewer has checked whether these tests enforce the requirement |
| `RFC9252-3.2-4` | An unrecognized SRv6 Endpoint Behavior MUST NOT be considered invalid by the receiver, except for behaviors that involve the use of arguments (§3.1) | MUST NOT | 3.1 | **positive:** `unit/verify` [`TestExtractSRv6SID_UnknownEndpointBehavior`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L287). **negative:** no negative test. **{single-polarity}:** extractSIDFromServiceTLV extracts the SID without inspecting or validating the SRv6 Endpoint Behavior, so an unrecognized behavior is never rejected and there is no behavior-based rejection path to drive negatively (internal/component/bgp/plugins/rib/pool/srv6sid.go:84) |
| `RFC9252-3.2-5` | A receiver is unable to validate the applicability of arguments for SRv6 Endpoint Behaviors that are unknown to it and hence MUST ignore SRv6 SIDs with arguments (indicated by a non-zero AL) with unknown SRv6 Endpoint Behaviors. (S3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestExtractSRv6SIDFull_NonZeroArgumentLengthInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_sid_structure_validity_test.go#L84). **negative:** `unit/verify` [`TestExtractSRv6SIDFull_NonZeroArgumentLengthInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_sid_structure_validity_test.go#L83) |
| `RFC9252-3.2-6` | For SIDs corresponding to an SRv6 Endpoint Behavior that is known, a receiver MUST validate that the consistency of the AL with the specific SRv6 Endpoint Behavior definition. (S3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no SRv6 Endpoint Behavior definitions, so it never validates the Argument Length against a known behavior's expected argument size (internal/component/bgp/plugins/rib/pool/srv6sid.go:84) |
| `RFC9252-5-1` | The path having any such Prefix-SID attribute without any valid SRv6 SID information MUST be considered ineligible during the selection of the best path for the corresponding prefix. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestIsSRv6Ineligible_ValidSID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9252_srv6_ineligible_test.go#L77). **positive:** `unit/verify` [`TestSRv6TranspositionAtLabelWidthIsEligible`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9252_srv6_admission_test.go#L75). **negative:** `unit/verify` [`TestIsSRv6Ineligible_InvalidSIDStructure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9252_srv6_admission_test.go#L44). **negative:** `unit/verify` [`TestSRv6TranspositionWiderThanLabelFieldIsIneligible`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9252_srv6_transposition_test.go#L201). **negative:** `functional/verify` [`srv6-invalid-sid-ineligible.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/srv6-invalid-sid-ineligible.ci#L4) |
| `RFC9252-5-2` | Therefore, the ingress PE MUST perform a resolvability check for the SRv6 Service SID before considering the received prefix for the BGP best path computation. (S5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze acts as an SRv6 ingress PE -- it extracts the received best-path Service SID (internal/component/bgp/plugins/rib/rib_bestchange.go:729,:882) and installs it into the FIB as a kernel SEG6 encap route (internal/plugins/fib/kernel/nexthop_linux.go:78) or a VPP SR steering policy (internal/plugins/fib/vpp/srv6.go:35) -- but performs no RFC 9252 Section 5 resolvability check: isSRv6Ineligible (internal/component/bgp/plugins/rib/rib_bestchange.go:963) gates best-path on SID extraction validity only, never on locator reachability (no resolvability check exists in internal/component/bgp) |
| `RFC9252-3.3-1` | If the BGP next hop is unchanged during the advertisement, the SRv6 Service TLVs, including any unrecognized Types of Sub-TLV and Sub-Sub-TLV, SHOULD be propagated further. In addition, all Reserved fields in the TLV, Sub-TLV, or Sub-Sub-TLV MUST be propagated unchanged. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_peer_forward_facts_test.go#L339). **positive:** `unit/verify` [`TestRFC9252EffectiveNextHopControlsServiceTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_effective_next_hop_test.go#L32). **positive:** `unit/verify` [`TestRFC9252ReservedFieldsPropagatedUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_reserved_propagated_test.go#L55). **negative:** `unit/verify` [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_peer_forward_facts_test.go#L340). **negative:** `unit/verify` [`TestRFC9252EffectiveNextHopControlsServiceTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_effective_next_hop_test.go#L33) |
| `RFC9252-3.3-2` | If the BGP next hop is changed, the TLVs, Sub-TLVs, and Sub-Sub- TLVs SHOULD be updated with the locally allocated SRv6 SID information. Any received Sub-TLVs and Sub-Sub-TLVs that are unrecognized MUST be removed. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_peer_forward_facts_test.go#L341). **positive:** `unit/verify` [`TestRFC9252EffectiveNextHopControlsServiceTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_effective_next_hop_test.go#L34). **positive:** `unit/verify` [`TestRFC9252ServiceOnlyPrefixSIDDroppedOnANextHopChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_nexthop_change_defect_test.go#L176). **negative:** `unit/verify` [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_peer_forward_facts_test.go#L342). **negative:** `unit/verify` [`TestRFC9252EffectiveNextHopControlsServiceTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_effective_next_hop_test.go#L35). **{partial}:** tested "Any received Sub-TLVs and Sub-Sub-TLVs that are unrecognized MUST be removed."; gap "SHOULD be updated with the locally allocated SRv6 SID information."; internal/component/bgp/reactor/forward_prefix_sid.go::applyEgressPrefixSIDNextHop compares the received and final effective next-hop entity and requests removal of the received SRv6 Service TLVs when that entity changes. The registered handler removes their unknown nested fields while retaining unrelated top-level TLVs. Locally allocated SRv6 SID information is not produced to rebuild the service information; only that allocation/rebuilding remains absent and uncommissioned.. **Partial test coverage:** Tests and fault test records cover only the part marked Tested; they do not verify the whole requirement. partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), stale-unit: internal/component/bgp/reactor/forward_prefix_sid.go::applyEgressPrefixSIDNextHop, internal/component/bgp/reactor/forward_prefix_sid.go::prefixSIDNextHopHandler moved. Tested: ‘Any received Sub-TLVs and Sub-Sub-TLVs that are unrecognized MUST be removed.’ Gap: ‘SHOULD be updated with the locally allocated SRv6 SID information.’ Producer: internal/component/bgp/reactor/forward_prefix_sid.go::applyEgressPrefixSIDNextHop. Independent post-fix review confirms that original received identity is compared with final effective carrier identity after policy/configuration/scope on cached and RS paths, including raw-policy fallback and repaired intermediate addressless MP fields. The registered prefixSIDNextHopHandler consumes Remove of types 5 and 6 while retaining unrelated top-level TLVs. Migrated TestPrefixSIDPropagationNextHop and TestRFC9252EffectiveNextHopControlsServiceTLVs inspect actual recipient Session output: effective A-to-B removes Service TLVs and unknown nested fields while the route survives; A-to-A and restoration controls preserve them; mixed MP/legacy cases follow the applicable MP identity; unknown top-level TLVs and SRGB remain, with Label-Index transmit normalization tested separately. The older service-only test supplies complementary removal evidence, and the new actual-writer service-only cases forbid an empty attribute. The policy-only-change and equal-address defects are fixed, not scoped away. Locally allocated SRv6 SID information is still not produced to rebuild the service information, so the whole SHOULD-plus-MUST quotation remains partial. No local SID allocation is commissioned, and pending live FRR/SDK or native-record completion is not claimed by this source judgment. Re-judged 2026-10-14 (independent reader, after 8c1c4013f2 and 7d37dcd562). The only edit to TestRFC9252EffectiveNextHopControlsServiceTLVs changes the expected Label-Index from normalized to as received (unrecognized off labeled unicast, RFC 8669 Section 3.1); the Tested removal assertions are unchanged and the Gap (no locally allocated SRv6 SID information) stands. Verdict unchanged. |
| `RFC9252-3.4-1` | The treat-as-withdraw action [RFC7606] MUST be performed when at least one malformed SRv6 Service TLV is present in the BGP Prefix-SID attribute. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestValidatePrefixSIDAttr_Valid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2411). **negative:** `unit/verify` [`TestRFC7606PrefixSIDFramingErrorAfterServiceTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9252_service_tlv_overrun_test.go#L119). **negative:** `unit/verify` [`TestRFC9252SIDInformationStrayOctetsTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9252_service_tlv_overrun_test.go#L98). **negative:** `unit/verify` [`TestRFC9252ServiceTLVOverrunTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9252_service_tlv_overrun_test.go#L31). **negative:** `unit/verify` [`TestRFC9252ZeroLengthServiceTLVTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9252_service_tlv_overrun_test.go#L60). **negative:** `unit/verify` [`TestValidateSRv6ServiceTLV_SIDInfoTooShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2480). **negative:** `unit/verify` [`TestValidateSRv6ServiceTLV_TrailingBytes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2440). **negative:** `functional/verify` [`prefixsid-malformed-receive.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-malformed-receive.ci#L8) |
| `RFC9252-3.2.1-4` | The bits that have been shifted out MUST be set to 0 in the SID value. (§3.2.1) | MUST | 3.2.1 | **positive:** `unit/verify` [`TestSRv6OriginTranspositionKeepsOnlyUntransposedBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc9252_routeattr_srv6_test.go#L10). **negative:** `unit/verify` [`TestSRv6OriginRefusesNonzeroTransposedBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc9252_routeattr_srv6_test.go#L21) |
| `RFC9252-3.2.1-5` | Implementations supporting this specification MUST provide a mechanism to control the advertisement of SRv6-based BGP service routes on a per-neighbor and per-service basis. (S3.2.1) | MUST | 3.2.1 | **positive:** `functional/verify` [`srv6-service-export-control.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/srv6-service-export-control.ci#L2). **negative:** `functional/verify` [`srv6-service-export-control.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/srv6-service-export-control.ci#L3) |
| `RFC9252-3.2.1-6` | Arguments may be generally applicable for SIDs of only specific SRv6 Endpoint Behaviors (e.g., End.DT2M); therefore, the AL MUST be set to 0 for SIDs where the Argument is not applicable. (S3.2.1) | MUST | 3.2.1 | **positive:** `unit/verify` [`TestSRv6OriginWithoutArguments`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc9252_routeattr_srv6_test.go#L30). **negative:** `unit/verify` [`TestSRv6OriginRefusesArgumentsForDT4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc9252_routeattr_srv6_test.go#L41) |
| `RFC9252-5-3` | it indicates that the egress PE supports SRv6 overlay, and the BGP ingress PE receiving this route MUST perform IPv6 encapsulation and insert an SRH [RFC8754] when required (§5) | MUST | 5 | **positive:** `unit/verify` [`TestSRv6ServiceSIDSelectsIPv6Encapsulation`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc9252_nexthop_srv6_linux_test.go#L15). **negative:** `unit/verify` [`TestRouteWithoutServiceSIDKeepsMPLSEncapsulation`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc9252_nexthop_srv6_linux_test.go#L39) |
| `RFC9252-7-1` | If multiple instances of the SRv6 L3 Service TLV are encountered, all but the first instance MUST be ignored. (S7) | MUST | 7 | **positive:** `unit/verify` [`TestRFC9252FirstServiceTLVWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_first_service_tlv_test.go#L31). **negative:** `unit/verify` [`TestRFC9252FirstServiceTLVWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_first_service_tlv_test.go#L32). **negative:** `unit/verify` [`TestRFC9252SecondL3ServiceTLVIgnoredWhenFirstHasNoSID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_service_tlv_first_instance_test.go#L33) |
| `RFC9252-7-2` | If multiple instances of the SRv6 L2 Service TLV are encountered, all but the first instance MUST be ignored. (S7) | MUST | 7 | **positive:** `unit/verify` [`TestRFC9252FirstServiceTLVWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_first_service_tlv_test.go#L33). **negative:** `unit/verify` [`TestRFC9252FirstServiceTLVWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_first_service_tlv_test.go#L34). **negative:** `unit/verify` [`TestRFC9252SecondL2ServiceTLVIgnoredWhenFirstHasNoSID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_service_tlv_first_instance_test.go#L50) |
| `RFC9252-3.2-7` | When multiple SRv6 SID Information Sub-TLVs are present, the ingress PE SHOULD use the SRv6 SID from the first instance of the Sub-TLV. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9252-3.3-3` | If the BGP next hop is unchanged during the advertisement, the SRv6 Service TLVs, including any unrecognized Types of Sub-TLV and Sub-Sub-TLV, SHOULD be propagated further. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9252-3.3-4` | If the BGP next hop is changed, the TLVs, Sub-TLVs, and Sub-Sub- TLVs SHOULD be updated with the locally allocated SRv6 SID information. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9252-3.2-8` | When multiple SRv6 SID Information Sub-TLVs are present, the ingress PE SHOULD use the SRv6 SID from the first instance of the Sub-TLV. An implementation MAY provide a local policy to override this selection. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9252-3.2-9` | The opaque SRv6 Endpoint Behavior (i.e., value 0xFFFF) MAY be used when the advertising router wishes to abstract the actual behavior of its locally instantiated SRv6 SID. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9252-4.1-1`](#rfc9252-4.1-1) When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 20 and less than or equal to the FL. (§5.1, §5.2) | {partial}, Partial test coverage | tested "the Transposition Length MUST be less than or equal to 20"; gap "less than or equal to the FL"; the 20-bit half is enforced on receive, where srv6SIDFromResult refuses to reconstruct and isSRv6Ineligible makes the path ineligible when Transposition Length exceeds nlrisplit.TranspositionLabelWidth (internal/component/bgp/plugins/rib/rib_bestchange.go), and on origination, where internal/core/bgp/attribute/prefixsid.go::CheckSRv6TranspositionForFamily refuses it; neither side nor parseSIDStructure bounds it against the Function Length (internal/component/bgp/plugins/rib/pool/srv6sid.go) |
| [`RFC9252-6.1-1`](#rfc9252-6.1-1) When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the AL. (§6.1.1) | {gap}, no test | the 24-bit half is enforced by nlrisplit.TranspositionLabelWidth through srv6SIDFromResult and isSRv6Ineligible (internal/component/bgp/plugins/rib/rib_bestchange.go), but the Argument Length is never bounded, the ESI Label extended community that carries these bits is never read, and the EVPN encoder carries no SRv6 ESI-label SID (internal/component/bgp/plugins/rib/pool/srv6sid.go) |
| [`RFC9252-6.2-1`](#rfc9252-6.2-1) When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. (§6.2, §6.3, §6.5) | {partial}, Partial test coverage | tested "the Transposition Length MUST be less than or equal to 24"; gap "less than or equal to the FL"; the 24-bit half is enforced by nlrisplit.TranspositionLabelWidth through srv6SIDFromResult and isSRv6Ineligible on receive (internal/component/bgp/plugins/rib/rib_bestchange.go) and through internal/core/bgp/attribute/prefixsid.go::CheckSRv6TranspositionForFamily on origination, but the Function Length is never bounded and no EVPN label field is read at all -- TranspositionLabel answers only the VPN families, so an EVPN transposition yields no SID rather than a reconstructed one (internal/core/bgp/nlri/nlrisplit/transposition.go) |
| [`RFC9252-3.2-6`](#rfc9252-3.2-6) For SIDs corresponding to an SRv6 Endpoint Behavior that is known, a receiver MUST validate that the consistency of the AL with the specific SRv6 Endpoint Behavior definition. (S3.2) | {gap}, no test | ze has no SRv6 Endpoint Behavior definitions, so it never validates the Argument Length against a known behavior's expected argument size (internal/component/bgp/plugins/rib/pool/srv6sid.go:84) |
| [`RFC9252-5-2`](#rfc9252-5-2) Therefore, the ingress PE MUST perform a resolvability check for the SRv6 Service SID before considering the received prefix for the BGP best path computation. (S5) | {gap}, no test | ze acts as an SRv6 ingress PE -- it extracts the received best-path Service SID (internal/component/bgp/plugins/rib/rib_bestchange.go:729,:882) and installs it into the FIB as a kernel SEG6 encap route (internal/plugins/fib/kernel/nexthop_linux.go:78) or a VPP SR steering policy (internal/plugins/fib/vpp/srv6.go:35) -- but performs no RFC 9252 Section 5 resolvability check: isSRv6Ineligible (internal/component/bgp/plugins/rib/rib_bestchange.go:963) gates best-path on SID extraction validity only, never on locator reachability (no resolvability check exists in internal/component/bgp) |
| [`RFC9252-3.3-2`](#rfc9252-3.3-2) If the BGP next hop is changed, the TLVs, Sub-TLVs, and Sub-Sub- TLVs SHOULD be updated with the locally allocated SRv6 SID information. Any received Sub-TLVs and Sub-Sub-TLVs that are unrecognized MUST be removed. (§2) | {partial}, Partial test coverage | tested "Any received Sub-TLVs and Sub-Sub-TLVs that are unrecognized MUST be removed."; gap "SHOULD be updated with the locally allocated SRv6 SID information."; internal/component/bgp/reactor/forward_prefix_sid.go::applyEgressPrefixSIDNextHop compares the received and final effective next-hop entity and requests removal of the received SRv6 Service TLVs when that entity changes. The registered handler removes their unknown nested fields while retaining unrelated top-level TLVs. Locally allocated SRv6 SID information is not produced to rebuild the service information; only that allocation/rebuilding remains absent and uncommissioned. |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC9252-3.1-1`](#rfc9252-3.1-1)

This field is reserved; it MUST be set to 0 by the sender (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: the sender emits a non-zero Service TLV RESERVED octet. (b) TestEncodePrefixSIDSRv6_ReservedFieldsZero asserts ps[3]==0 on EncodePrefixSIDSRv6 output, with a non-zero behavior code so the zero is not a zero-filled buffer. Single-polarity marker present (no reject path on encode).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L163) | unit/verify | revert, verified |

### [`RFC9252-3.1-2`](#rfc9252-3.1-2)

This field is reserved; it MUST be set to 0 by the sender and ignored by the receiver. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a receiver that rejects or mis-parses a Service TLV whose RESERVED octet is non-zero. (b) TestExtractSRv6SID_ServiceReservedIgnored feeds RESERVED=0xFF and asserts ExtractSRv6SID still returns the SID; TestExtractSRv6SID_ServiceReservedZero is the positive. validateSRv6ServiceTLV (rfc7606.go) never reads the octet, so the UPDATE path does not reject it either.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestExtractSRv6SID_ServiceReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L271) | unit/verify | not verified |
| positive | [`TestExtractSRv6SID_ServiceReservedZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L257) | unit/verify | not verified |

### [`RFC9252-3.2-1`](#rfc9252-3.2-1)

RESERVED1 (1 octet): This field MUST be set to 0 by the sender (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: the sender emits a non-zero SID Information RESERVED1 octet. (b) TestEncodePrefixSIDSRv6_ReservedFieldsZero asserts ps[7]==0. Single-polarity marker present.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L164) | unit/verify | revert, verified |

### [`RFC9252-3.2-2`](#rfc9252-3.2-2)

This field encodes SRv6 Service SID Flags -- none are currently defined. It MUST be set to 0 by the sender (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: the sender emits a non-zero Service SID Flags octet. (b) TestEncodePrefixSIDSRv6_ReservedFieldsZero asserts ps[24]==0. The quote covers the sender half only; the receiver half (unknown flags MUST be ignored) has no row. Single-polarity marker present.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L165) | unit/verify | revert, verified |

### [`RFC9252-3.2-3`](#rfc9252-3.2-3)

RESERVED2 (1 octet): This field MUST be set to 0 by the sender (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: the sender emits a non-zero RESERVED2 octet. (b) TestEncodePrefixSIDSRv6_ReservedFieldsZero asserts ps[27]==0, next to a behavior of 0x003e. Single-polarity marker present.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L166) | unit/verify | revert, verified |

### [`RFC9252-3.2.1-1`](#rfc9252-3.2.1-1)

In this case, the Transposition Offset MUST be set to 0. (S3.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestExtractSRv6SIDFull_ZeroLengthNonZeroOffset`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_sid_structure_validity_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestExtractSRv6SIDFull_ZeroLengthNonZeroOffset`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_sid_structure_validity_test.go#L60) | unit/verify | revert, verified |

### [`RFC9252-3.2.1-2`](#rfc9252-3.2.1-2)

The transposition offset and length MUST be 0 when the Sub-Sub-TLV is advertised along with routes where the Transposition Scheme is not applicable (e.g., for global IPv6 service [RFC2545] where there is no label field). (§7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestUpdateTextSRv6TranspositionNeedsALabelField`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc9252_update_text_srv6_transposition_test.go#L27) | unit/verify | revert, verified |
| negative | [`TestSRv6TranspositionOnFamilyWithoutLabelIsIneligible`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9252_srv6_admission_test.go#L104) | unit/verify | revert, verified |
| negative | [`TestCheckSRv6TranspositionForFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9252_prefixsid_origin_test.go#L24) | unit/verify | revert, verified |
| negative | [`srv6-transposition-refused-unicast.ci`](https://github.com/ze-software/ze/blob/main/test/parse/srv6-transposition-refused-unicast.ci#L12) | functional/verify | revert, verified |
| positive | [`TestUpdateTextSRv6TranspositionNeedsALabelField`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc9252_update_text_srv6_transposition_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestSRv6TranspositionOnFamilyWithoutLabelIsIneligible`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9252_srv6_admission_test.go#L105) | unit/verify | revert, verified |
| positive | [`TestCheckSRv6TranspositionForFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9252_prefixsid_origin_test.go#L23) | unit/verify | revert, verified |

### [`RFC9252-3.2.1-3`](#rfc9252-3.2.1-3)

As defined in [RFC8986], the sum of the Locator Block Length (LBL), Locator Node Length (LNL), Function Length (FL), and Argument Length (AL) fields MUST be less than or equal to 128 and greater than or equal to the sum of Transposition Offset and Transposition Length. (S3.2.1, errata 7817)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-14 (independent reader, after 8c1c4013f2 and 7d37dcd562). RFC 9252 Section 3.2.1 with erratum 7817: the sum LBL+LNL+FL+AL 'MUST be less than or equal to 128 and greater than or equal to the sum of Transposition Offset and Transposition Length'. The earlier weakness (both negatives asserted the SID stayed valid) is gone: TestExtractSRv6SIDFull_InvalidSIDStructure (sum 192, AL 0 so only the sum bound fires) and TestExtractSRv6SIDFull_SumBelowTransposition (64 < 76) now assert no SID is answered, Section 7 making such a SID invalid; TestExtractSRv6SIDFull_Sum128IsValid asserts sum 128 and sum equal to offset plus length keep the SID with its transposition, and sum 129 answers none; TestExtractSRv6SIDFull_WithTransposition stays the positive. The consequence for best path is asserted under RFC9252-5-1. TestExtractSRv6SIDFull_SIDStructureLengthNotSix carries this tag but tests the neighbouring 6-octet length rule of Section 3.2.1, not the sum bound: it adds nothing here.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestExtractSRv6SIDFull_SIDStructureLengthNotSix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_sid_structure_validity_test.go#L36) | unit/verify | revert, verified |
| negative | [`TestExtractSRv6SIDFull_InvalidSIDStructure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L217) | unit/verify | revert, verified |
| negative | [`TestExtractSRv6SIDFull_SumBelowTransposition`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L237) | unit/verify | revert, verified |
| positive | [`TestExtractSRv6SIDFull_Sum128IsValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_sid_structure_validity_test.go#L104) | unit/verify | revert, verified |
| positive | [`TestExtractSRv6SIDFull_WithTransposition`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L139) | unit/verify | not verified |

### [`RFC9252-4.1-1`](#rfc9252-4.1-1)

When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 20 and less than or equal to the FL. (§5.1, §5.2)

Records cover only part of this requirement: tested "the Transposition Length MUST be less than or equal to 20"; gap "less than or equal to the FL"; the 20-bit half is enforced on receive, where srv6SIDFromResult refuses to reconstruct and isSRv6Ineligible makes the path ineligible when Transposition Length exceeds nlrisplit.TranspositionLabelWidth (internal/component/bgp/plugins/rib/rib_bestchange.go), and on origination, where internal/core/bgp/attribute/prefixsid.go::CheckSRv6TranspositionForFamily refuses it; neither side nor parseSIDStructure bounds it against the Function Length (internal/component/bgp/plugins/rib/pool/srv6sid.go); they do not verify the whole requirement.

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestCheckSRv6TranspositionLabelWidth`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9252_prefixsid_origin_test.go#L79) | unit/verify | revert, verified |
| positive | [`TestCheckSRv6TranspositionLabelWidth`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9252_prefixsid_origin_test.go#L78) | unit/verify | revert, verified |

### [`RFC9252-6.1-1`](#rfc9252-6.1-1)

When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the AL. (§6.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9252-6.1-1, so there is no linked test.

### [`RFC9252-6.2-1`](#rfc9252-6.2-1)

When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. (§6.2, §6.3, §6.5)

Records cover only part of this requirement: tested "the Transposition Length MUST be less than or equal to 24"; gap "less than or equal to the FL"; the 24-bit half is enforced by nlrisplit.TranspositionLabelWidth through srv6SIDFromResult and isSRv6Ineligible on receive (internal/component/bgp/plugins/rib/rib_bestchange.go) and through internal/core/bgp/attribute/prefixsid.go::CheckSRv6TranspositionForFamily on origination, but the Function Length is never bounded and no EVPN label field is read at all -- TranspositionLabel answers only the VPN families, so an EVPN transposition yields no SID rather than a reconstructed one (internal/core/bgp/nlri/nlrisplit/transposition.go); they do not verify the whole requirement.

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestCheckSRv6TranspositionLabelWidth`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9252_prefixsid_origin_test.go#L81) | unit/verify | revert, verified |
| positive | [`TestCheckSRv6TranspositionLabelWidth`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9252_prefixsid_origin_test.go#L80) | unit/verify | revert, verified |

### [`RFC9252-3.2-4`](#rfc9252-3.2-4)

An unrecognized SRv6 Endpoint Behavior MUST NOT be considered invalid by the receiver, except for behaviors that involve the use of arguments (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a receiver treating a SID with an unrecognized, argument-free Endpoint Behavior as invalid. (b) TestExtractSRv6SID_UnknownEndpointBehavior feeds behavior 0xFEED with no SID Structure and asserts the SID is returned. validateSRv6ServiceTLV never reads the behavior. The argument exception is the gap row RFC9252-3.2-5. Single-polarity marker present.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestExtractSRv6SID_UnknownEndpointBehavior`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L287) | unit/verify | not verified |

### [`RFC9252-3.2-5`](#rfc9252-3.2-5)

A receiver is unable to validate the applicability of arguments for SRv6 Endpoint Behaviors that are unknown to it and hence MUST ignore SRv6 SIDs with arguments (indicated by a non-zero AL) with unknown SRv6 Endpoint Behaviors. (S3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestExtractSRv6SIDFull_NonZeroArgumentLengthInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_sid_structure_validity_test.go#L83) | unit/verify | revert, verified |
| positive | [`TestExtractSRv6SIDFull_NonZeroArgumentLengthInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_sid_structure_validity_test.go#L84) | unit/verify | revert, verified |

### [`RFC9252-3.2-6`](#rfc9252-3.2-6)

For SIDs corresponding to an SRv6 Endpoint Behavior that is known, a receiver MUST validate that the consistency of the AL with the specific SRv6 Endpoint Behavior definition. (S3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9252-3.2-6, so there is no linked test.

### [`RFC9252-5-1`](#rfc9252-5-1)

The path having any such Prefix-SID attribute without any valid SRv6 SID information MUST be considered ineligible during the selection of the best path for the corresponding prefix. (§7)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/rib/rfc9252_srv6_admission_test.go::TestIsSRv6Ineligible_InvalidSIDStructure, internal/component/bgp/plugins/rib/rfc9252_srv6_admission_test.go::TestSRv6TranspositionAtLabelWidthIsEligible, internal/component/bgp/plugins/rib/rfc9252_srv6_transposition_test.go::TestSRv6TranspositionWiderThanLabelFieldIsIneligible, test/plugin/srv6-invalid-sid-ineligible.ci moved. Re-judged 2026-10-14 (independent reader, after 8c1c4013f2 and 7d37dcd562). RFC 9252 Section 7: 'The path having any such Prefix-SID attribute without any valid SRv6 SID information MUST be considered ineligible during the selection of the best path for the corresponding prefix.' The earlier findings are answered: isSRv6Ineligible now calls ExtractSRv6SIDFull and srv6SIDValidForFamily, so an invalid SID Structure or a transposition wider than the label field excludes the path. Negatives: TestIsSRv6Ineligible_InvalidSIDStructure (sum 200) and TestSRv6TranspositionWiderThanLabelFieldIsIneligible (24 bits into 20) feed a received VPNv4 UPDATE and assert candidateAdmitted, the gate checkRouteBestChange applies, refuses the stored path; srv6-invalid-sid-ineligible.ci asserts through the daemon that only the valid path of three has a best path. Positives: TestSRv6TranspositionAtLabelWidthIsEligible (exactly 20 bits) is admitted with its SID reconstructed, and TestIsSRv6Ineligible_ValidSID. TestIsSRv6Ineligible_InvalidSID is a structurally malformed Sub-TLV (treat-as-withdraw upstream) and is supplementary.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIsSRv6Ineligible_InvalidSIDStructure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9252_srv6_admission_test.go#L44) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| negative | [`TestSRv6TranspositionWiderThanLabelFieldIsIneligible`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9252_srv6_transposition_test.go#L201) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| negative | [`srv6-invalid-sid-ineligible.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/srv6-invalid-sid-ineligible.ci#L4) | functional/verify | revert, verified |
| positive | [`TestSRv6TranspositionAtLabelWidthIsEligible`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9252_srv6_admission_test.go#L75) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestIsSRv6Ineligible_ValidSID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9252_srv6_ineligible_test.go#L77) | unit/verify | revert, verified |

### [`RFC9252-5-2`](#rfc9252-5-2)

Therefore, the ingress PE MUST perform a resolvability check for the SRv6 Service SID before considering the received prefix for the BGP best path computation. (S5)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. {gap} independently re-confirmed against the complete RFC 9252 Section 5 and the current selection and forwarding producers. The clause remains: "Therefore, the ingress PE MUST perform a resolvability check for the SRv6 Service SID before considering the received prefix for the BGP best path computation." isSRv6Ineligible in internal/component/bgp/plugins/rib/rib_bestchange.go still checks Service SID extraction and family validity, not locator reachability before best-path selection. The existing late SID lookup moved unchanged from sysRIB.fibEntry into sysRIB.resolvedFibEntry in internal/component/sysrib/sysrib.go. The selected-route consumer reaches it only after the BGP winner has been mirrored into the Loc-RIB and the authority has selected its system route. An unreachable Service SID therefore still permits the route to win before the FIB write is declined; the next-best BGP route is not chosen by this late refusal. fibEntry now wraps that existing resolution result and classifies only an otherwise valid targetless unicast output as nonprogrammable, using the complete output validator and its private typed missing-target result. That classification changes neither the ordering nor the scope of the Service SID check and is not RFC 9252 Section 5 conformance. The same unimplemented verdict, MUST clause, gap annotation and absent tagged coverage remain; no credit is added. The wrapper and extracted helper are both cited so this judgment remains bound to the actual call and late check, alongside the unchanged BGP eligibility producer. The pre-selection resolvability work remains owed by plan/release/fixes/spec-srv6-bestpath-resolvability.md.

No test carries RFC9252-5-2, so there is no linked test.

### [`RFC9252-3.3-1`](#rfc9252-3.3-1)

If the BGP next hop is unchanged during the advertisement, the SRv6 Service TLVs, including any unrecognized Types of Sub-TLV and Sub-Sub-TLV, SHOULD be propagated further. In addition, all Reserved fields in the TLV, Sub-TLV, or Sub-Sub-TLV MUST be propagated unchanged. (§2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/reactor/rfc9252_effective_next_hop_test.go::TestRFC9252EffectiveNextHopControlsServiceTLVs, internal/component/bgp/reactor/rfc9252_effective_next_hop_test.go::TestRFC9252EffectiveNextHopControlsServiceTLVs#2 moved. Rejudged the whole RFC 9252 Section 2 unchanged-next-hop propagation and Reserved-field obligation after the implemented identity-condition fix. applyFactsNextHop now records only next-hop operations; applyEgressPrefixSIDNextHop compares the original received entity with the final effective carrier after policy, configured rewriting and scope normalization on cached and RS paths. Equal identity records no Service-TLV removal, including explicit A-to-A and configured restoration of the original address. The migrated TestPrefixSIDPropagationNextHop and new TestRFC9252EffectiveNextHopControlsServiceTLVs inspect actual recipient Session output after validated receipt and cache publication, asserting exact Service-TLV Reserved and unknown nested bytes for unchanged identity, with changed-identity counter-controls. The matrix covers native IPv6/VPNv6 carriers, operation and raw-policy changes, RS export fallback, mixed legacy/MP output, pair trimming and repaired intermediate addressless MP fields. MP carrier presence is selected by base.mpFamily rather than address validity, so a legacy sibling or absent companion operation cannot decide the MP service identity. The earlier reserved-field fixture remains complementary materialized-body evidence, not the sole end-to-end proof. Legacy/mapped and extra L2 fixtures are propagation controls, not claims of service origination or EVPN support. The former explicit-equality defect is fixed, not retained as a scope gap. Re-judged 2026-10-14 (independent reader, after 8c1c4013f2 and 7d37dcd562). The only edit to TestRFC9252EffectiveNextHopControlsServiceTLVs changes the expected Label-Index from normalized to as received, since on these carriers (not labeled unicast) the TLV is unrecognized (RFC 8669 Section 3.1); the Service-TLV Reserved and unknown nested-byte assertions are unchanged. Verdict unchanged.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9252EffectiveNextHopControlsServiceTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_effective_next_hop_test.go#L33) | unit/verify | revert, verified |
| negative | [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_peer_forward_facts_test.go#L340) | unit/verify | revert, verified |
| positive | [`TestRFC9252EffectiveNextHopControlsServiceTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_effective_next_hop_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_peer_forward_facts_test.go#L339) | unit/verify | revert, verified |
| positive | [`TestRFC9252ReservedFieldsPropagatedUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_reserved_propagated_test.go#L55) | unit/verify | revert, verified |

### [`RFC9252-3.3-2`](#rfc9252-3.3-2)

If the BGP next hop is changed, the TLVs, Sub-TLVs, and Sub-Sub- TLVs SHOULD be updated with the locally allocated SRv6 SID information. Any received Sub-TLVs and Sub-Sub-TLVs that are unrecognized MUST be removed. (§2)

Records cover only part of this requirement: tested "Any received Sub-TLVs and Sub-Sub-TLVs that are unrecognized MUST be removed."; gap "SHOULD be updated with the locally allocated SRv6 SID information."; internal/component/bgp/reactor/forward_prefix_sid.go::applyEgressPrefixSIDNextHop compares the received and final effective next-hop entity and requests removal of the received SRv6 Service TLVs when that entity changes. The registered handler removes their unknown nested fields while retaining unrelated top-level TLVs. Locally allocated SRv6 SID information is not produced to rebuild the service information; only that allocation/rebuilding remains absent and uncommissioned.; they do not verify the whole requirement.

Audit verdict: partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), stale-unit: internal/component/bgp/reactor/forward_prefix_sid.go::applyEgressPrefixSIDNextHop, internal/component/bgp/reactor/forward_prefix_sid.go::prefixSIDNextHopHandler moved. Tested: ‘Any received Sub-TLVs and Sub-Sub-TLVs that are unrecognized MUST be removed.’ Gap: ‘SHOULD be updated with the locally allocated SRv6 SID information.’ Producer: internal/component/bgp/reactor/forward_prefix_sid.go::applyEgressPrefixSIDNextHop. Independent post-fix review confirms that original received identity is compared with final effective carrier identity after policy/configuration/scope on cached and RS paths, including raw-policy fallback and repaired intermediate addressless MP fields. The registered prefixSIDNextHopHandler consumes Remove of types 5 and 6 while retaining unrelated top-level TLVs. Migrated TestPrefixSIDPropagationNextHop and TestRFC9252EffectiveNextHopControlsServiceTLVs inspect actual recipient Session output: effective A-to-B removes Service TLVs and unknown nested fields while the route survives; A-to-A and restoration controls preserve them; mixed MP/legacy cases follow the applicable MP identity; unknown top-level TLVs and SRGB remain, with Label-Index transmit normalization tested separately. The older service-only test supplies complementary removal evidence, and the new actual-writer service-only cases forbid an empty attribute. The policy-only-change and equal-address defects are fixed, not scoped away. Locally allocated SRv6 SID information is still not produced to rebuild the service information, so the whole SHOULD-plus-MUST quotation remains partial. No local SID allocation is commissioned, and pending live FRR/SDK or native-record completion is not claimed by this source judgment. Re-judged 2026-10-14 (independent reader, after 8c1c4013f2 and 7d37dcd562). The only edit to TestRFC9252EffectiveNextHopControlsServiceTLVs changes the expected Label-Index from normalized to as received (unrecognized off labeled unicast, RFC 8669 Section 3.1); the Tested removal assertions are unchanged and the Gap (no locally allocated SRv6 SID information) stands. Verdict unchanged.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9252EffectiveNextHopControlsServiceTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_effective_next_hop_test.go#L35) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| negative | [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_peer_forward_facts_test.go#L342) | unit/verify | revert, verified |
| positive | [`TestRFC9252ServiceOnlyPrefixSIDDroppedOnANextHopChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_nexthop_change_defect_test.go#L176) | unit/verify | revert, verified |
| positive | [`TestRFC9252EffectiveNextHopControlsServiceTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_effective_next_hop_test.go#L34) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_peer_forward_facts_test.go#L341) | unit/verify | revert, verified |

### [`RFC9252-3.4-1`](#rfc9252-3.4-1)

The treat-as-withdraw action [RFC7606] MUST be performed when at least one malformed SRv6 Service TLV is present in the BGP Prefix-SID attribute. (§7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (rfc9252): "The treat-as-withdraw action [RFC7606] MUST be performed when at least one malformed SRv6 Service TLV is present in the BGP Prefix-SID attribute." The only edit to test/plugin/prefixsid-malformed-receive.ci since the last judgement is f8f7628bdf, which adds a bgp-level filter { import { final accept; } export { final accept; } } with no filter refs, because RFC 8212 Section 3 now makes an EBGP direction with no explicit policy reject. No expect, reject, input or requirement-tag line changed. policy.ConcatChains keeps every peer-level ref and takes the final from the most specific configured level, so a peer-level chain still decides where one exists. An accepting final with no refs neither refuses nor rewrites a route: a withheld, stripped or discarded outcome the file asserts can still come only from the producer under test, and a delivered outcome is the eligibility the test was written under before RFC 8212; without the block the positives would fail for the wrong reason. What the unit proves is unchanged. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestValidateSRv6ServiceTLV_SIDInfoTooShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2480) | unit/verify | revert, verified |
| negative | [`TestValidateSRv6ServiceTLV_TrailingBytes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2440) | unit/verify | revert, verified |
| negative | [`TestRFC7606PrefixSIDFramingErrorAfterServiceTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9252_service_tlv_overrun_test.go#L119) | unit/verify | revert, verified |
| negative | [`TestRFC9252SIDInformationStrayOctetsTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9252_service_tlv_overrun_test.go#L98) | unit/verify | revert, verified |
| negative | [`TestRFC9252ServiceTLVOverrunTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9252_service_tlv_overrun_test.go#L31) | unit/verify | revert, verified |
| negative | [`TestRFC9252ZeroLengthServiceTLVTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9252_service_tlv_overrun_test.go#L60) | unit/verify | revert, verified |
| negative | [`prefixsid-malformed-receive.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefixsid-malformed-receive.ci#L8) | functional/verify | revert, verified |
| positive | [`TestValidatePrefixSIDAttr_Valid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2411) | unit/verify | not verified |

### [`RFC9252-3.2.1-4`](#rfc9252-3.2.1-4)

The bits that have been shifted out MUST be set to 0 in the SID value. (§3.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: advertising a SID with a set bit inside the transposed range. (b) TestSRv6OriginRefusesNonzeroTransposedBits configures bit 79 set within offset 65 length 15 and requires ParseRouteAttributes to fail (validateSRv6OriginStructure); TestSRv6OriginTranspositionKeepsOnlyUntransposedBits is the positive, keeping the adjacent bit 64.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestSRv6OriginRefusesNonzeroTransposedBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc9252_routeattr_srv6_test.go#L21) | unit/verify | not verified |
| positive | [`TestSRv6OriginTranspositionKeepsOnlyUntransposedBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc9252_routeattr_srv6_test.go#L10) | unit/verify | not verified |

### [`RFC9252-3.2.1-5`](#rfc9252-3.2.1-5)

Implementations supporting this specification MUST provide a mechanism to control the advertisement of SRv6-based BGP service routes on a per-neighbor and per-service basis. (S3.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-02 by an independent auditor who authored none of the changes named. RFC 9252 Section 3.2.1: "Implementations supporting this specification MUST provide a mechanism to control the advertisement of SRv6-based BGP service routes on a per-neighbor and per-service basis." fd4c866c74 changed test/plugin/srv6-service-export-control.ci only to reach the community list through a match filter: a new policy match community-SERVICE-20 { community SERVICE-20 } and the filtered peer's export filters [ community-SERVICE-20 ] in place of community-match:SERVICE-20. The list (target:65000:10 reject, target:65000:20 accept), the final accept, both peers, the two services and every expect and reject line are byte-identical, so the filtered peer must still receive only the RT20/SID 2001:db8:20:: route and the unfiltered control both, the second service fencing the first. Verdict unchanged: enforced. Earlier judgements, including the note on the removed ExportFilters record, are in this file's git history.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`srv6-service-export-control.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/srv6-service-export-control.ci#L3) | functional/verify | revert, verified |
| positive | [`srv6-service-export-control.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/srv6-service-export-control.ci#L2) | functional/verify | revert, verified |

### [`RFC9252-3.2.1-6`](#rfc9252-3.2.1-6)

Arguments may be generally applicable for SIDs of only specific SRv6 Endpoint Behaviors (e.g., End.DT2M); therefore, the AL MUST be set to 0 for SIDs where the Argument is not applicable. (S3.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: advertising a non-zero AL for a behavior with no argument. (b) TestSRv6OriginRefusesArgumentsForDT4 configures End.DT4 with AL=16 and requires ParseRouteAttributes to fail; TestSRv6OriginWithoutArguments asserts AL octet 0 for End.DT4. validateSRv6OriginStructure allows AL only for End.DT2M (0x0018).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestSRv6OriginRefusesArgumentsForDT4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc9252_routeattr_srv6_test.go#L41) | unit/verify | not verified |
| positive | [`TestSRv6OriginWithoutArguments`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc9252_routeattr_srv6_test.go#L30) | unit/verify | not verified |

### [`RFC9252-5-3`](#rfc9252-5-3)

it indicates that the egress PE supports SRv6 overlay, and the BGP ingress PE receiving this route MUST perform IPv6 encapsulation and insert an SRH [RFC8754] when required (§5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. (a) forbidden: an ingress PE forwarding an SRv6 service route without IPv6 encapsulation. TestSRv6ServiceSIDSelectsIPv6Encapsulation asserts SEG6 encap mode 1 with the SID for the kernel backend (buildRichRoute). The VPP backend (internal/plugins/fib/vpp/srv6.go) has no test. The negative-tagged TestRouteWithoutServiceSIDKeepsMPLSEncapsulation proves a route without a SID keeps MPLS, which is not a violation of this row.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRouteWithoutServiceSIDKeepsMPLSEncapsulation`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc9252_nexthop_srv6_linux_test.go#L39) | unit/verify | not verified |
| positive | [`TestSRv6ServiceSIDSelectsIPv6Encapsulation`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc9252_nexthop_srv6_linux_test.go#L15) | unit/verify | not verified |

### [`RFC9252-7-1`](#rfc9252-7-1)

If multiple instances of the SRv6 L3 Service TLV are encountered, all but the first instance MUST be ignored. (S7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-14 (independent reader, after 8c1c4013f2 and 7d37dcd562). RFC 9252 Section 7: 'If multiple instances of the SRv6 L3 Service TLV are encountered, all but the first instance MUST be ignored.' The earlier gap (ExtractSRv6SIDFull fell through to the second instance when the first had no valid SID) is fixed and tested: TestRFC9252SecondL3ServiceTLVIgnoredWhenFirstHasNoSID asserts no SID is answered for each first instance without a valid SID; TestRFC9252FirstServiceTLVWins asserts the first instance's SID when both are valid. Revert record over ExtractSRv6SIDFull re-observed 2026-10-14.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9252FirstServiceTLVWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_first_service_tlv_test.go#L32) | unit/verify | revert, verified |
| negative | [`TestRFC9252SecondL3ServiceTLVIgnoredWhenFirstHasNoSID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_service_tlv_first_instance_test.go#L33) | unit/verify | revert, verified |
| positive | [`TestRFC9252FirstServiceTLVWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_first_service_tlv_test.go#L31) | unit/verify | revert, verified |

### [`RFC9252-7-2`](#rfc9252-7-2)

If multiple instances of the SRv6 L2 Service TLV are encountered, all but the first instance MUST be ignored. (S7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-14 (independent reader, after 8c1c4013f2 and 7d37dcd562). RFC 9252 Section 7: 'If multiple instances of the SRv6 L2 Service TLV are encountered, all but the first instance MUST be ignored.' Same fix as RFC9252-7-1 for type 6: TestRFC9252SecondL2ServiceTLVIgnoredWhenFirstHasNoSID asserts no SID for each first instance without a valid SID, and TestRFC9252FirstServiceTLVWins#2 the first instance's SID when both are valid. Revert record over ExtractSRv6SIDFull re-observed 2026-10-14.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9252FirstServiceTLVWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_first_service_tlv_test.go#L34) | unit/verify | revert, verified |
| negative | [`TestRFC9252SecondL2ServiceTLVIgnoredWhenFirstHasNoSID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_service_tlv_first_instance_test.go#L50) | unit/verify | revert, verified |
| positive | [`TestRFC9252FirstServiceTLVWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_first_service_tlv_test.go#L33) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc9252.txt |
| Source fingerprint | dd9dd72a8f831935 |
| Record | rfc/extraction/rfc9252.json |
| Mapped sentences | 24 |
| Declined as scope | 12 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 1 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 3 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 4 | reviewed | not stated |
| `3.2` | not stated | 0 | reviewed | not stated |
| `3.2.1` | not stated | 7 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 2 | reviewed | not stated |
| `5.1` | not stated | 1 | reviewed | not stated |
| `5.2` | not stated | 1 | reviewed | not stated |
| `5.3` | not stated | 0 | reviewed | not stated |
| `5.4` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 2 | reviewed | not stated |
| `6.1` | not stated | 0 | reviewed | not stated |
| `6.1.1` | not stated | 1 | reviewed | not stated |
| `6.1.2` | not stated | 1 | reviewed | not stated |
| `6.2` | not stated | 2 | reviewed | not stated |
| `6.2.1` | not stated | 1 | reviewed | not stated |
| `6.2.2` | not stated | 2 | reviewed | not stated |
| `6.3` | not stated | 1 | reviewed | not stated |
| `6.4` | not stated | 0 | reviewed | not stated |
| `6.5` | not stated | 1 | reviewed | not stated |
| `6.6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 5 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `8.1` | not stated | 0 | reviewed | not stated |
| `8.2` | not stated | 0 | reviewed | not stated |
| `8.3` | not stated | 0 | reviewed | not stated |
| `8.4` | not stated | 0 | reviewed | not stated |
| `8.5` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 0 | reviewed | not stated |
| `9.2` | not stated | 0 | reviewed | not stated |
| `9.3` | not stated | 1 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |
| `10.1` | not stated | 0 | reviewed | not stated |
| `10.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction applicability statement about the deployment: it says which underlay nodes a segment list may name, and binds no behavior of a BGP implementation of this document. The obligation on an SRv6 data-plane node is RFC 8986's, not this document's. | The underlay nodes whose SRv6 SIDs are part of the SRH segment list MUST support the SRv6 data plane. |
| `5.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 5.2 states the IPv6 VPN case in the same words as the Section 5.1 IPv4 VPN case that RFC9252-4.1-1 maps. | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 20 and less than or equal to the FL. |
| `6:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6 repeats for EVPN the ingress-device encapsulation obligation Section 5 states for L3 services and RFC9252-5-3 maps. | Signaling of the SRv6 Service SID(s) serves two purposes -- first, it indicates that the BGP egress device supports SRv6 overlay, and the BGP ingress device receiving this route MUST perform IPv6 encapsulation and insert an SRH [RFC8754] when required; second, it indicates the value of the Service SID(s) to be used in the encapsulation. |
| `6:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6 repeats for EVPN the ingress-PE resolvability check Section 5 states and RFC9252-5-2 maps. | Therefore, the ingress PE MUST perform a resolvability check for the SRv6 Service SID before considering the received prefix for the BGP best path computation. |
| `6.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6.2 repeats the 24-bit and Function Length transposition bound word for word. | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. |
| `6.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second MAC/IP label field repeats the same 24-bit and Function Length bound word for word. | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. |
| `6.2.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6.2.1 repeats the same 24-bit and Function Length bound word for word. | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. |
| `6.2.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6.2.2 repeats the same 24-bit and Function Length bound word for word. | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. |
| `6.2.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second label field of the MAC+IP route repeats the same bound word for word. | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. |
| `6.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6.3 repeats the same 24-bit and Function Length bound word for word. | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. |
| `6.5:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6.5 repeats the same 24-bit and Function Length bound word for word. | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. |
| `9.3:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The sentence points the operator at Section 7 of RFC 8754 and at BCP 38 (RFC 2827) and BCP 84 (RFC 3704); the infrastructure ACL and ingress filtering obligations are stated in those documents, and this sentence adds none of its own. | As such, an operator deploying SRv6 MUST follow the considerations described in Section 7 of [RFC8754] to implement the infrastructure Access Control Lists (ACLs) and the recommendations described in BCP 38 [RFC2827] and BCP 84 [RFC3704]. |

## Superseded

No document obsoletes RFC 9252, so its obligations are stated where they were written.
