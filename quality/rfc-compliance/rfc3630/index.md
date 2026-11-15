# RFC 3630 - Traffic Engineering (TE) Extensions to OSPF Version 2

OSPFv2 Traffic Engineering LSA. Every requirement listed for RFC 3630, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 1 | of 5 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 5 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 5 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 5 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 2 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 4 | of 5 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 5 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 5 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 5 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 5 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 5 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 5 | of 10 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 5 mandatory requirements: they add up to 5. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | OSPFv2 Traffic Engineering LSA |
| Automatic checks | Included |
| Requirements | 10 |
| Mandatory requirements (MUST level) | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tests linked to requirements | 2 |
| Recorded audit verdicts | 1 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc3630.md` |
| Requirement table | `rfc/requirements/rfc3630.md` |
| RFC text | `rfc/full/rfc3630.txt` |

## Automatic checks

Included in automatic checks: OSPF Traffic Engineering (TE) LSA -- the Type 10 area-local Opaque LSA (Opaque type 1). Five MUST-level requirements gated: one tested, four gaps. RFC3630-1-1 (non-TE-capable nodes flood TE LSAs as ordinary Type-10 area-local Opaque LSAs) is covered by TestRFC3630NonTECapableFloodsTELSAByScope in internal/plugins/ospf/lsdb/rfc3630_te_test.go: positive, the consumer-agnostic LSDB carrier floods a Type-10 opaque type-1 (packet.TEOpaqueType) LSA to a Flood-eligible same-area neighbor purely by area scope with no TE code in the path (producer eligibleInterface default iface.AreaID==area at flooding.go:401, floodExcept at flooding.go:318 whose only opaque gate is the RFC 5250 O-bit at flooding.go:369, never a TE-type check); negative, the same area-local LSA is NOT flooded out an interface in a different area, bounding it exactly like any other Type-10 opaque LSA. RFC3630-6-1..6-4 (top-level and sub-TLV Type ranges 32768-32777 experimental / 32778-65535 reserved pending a Standards Track RFC) are gaps: they bind IANA and RFC authors, not an implementation, but their passive subjects fit neither not-applicable shape; ze neither allocates nor documents these type ranges. The 2.4.1/2.5.7/3/2.5.4 SHOULDs and MAYs are not gated.

## Published support status

**Area:** OSPFv2 Traffic Engineering LSA

**Proof:** 0 of 5 MUST proven

**Evidence:** -

**Unmet:** RFC3630-6-1, RFC3630-6-2, RFC3630-6-3, RFC3630-6-4


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 4 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **5** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (1):** [`RFC3630-1-1`](#rfc3630-1-1)

**Annotated (including scoped evidence) (4):** [`RFC3630-6-1`](#rfc3630-6-1), [`RFC3630-6-2`](#rfc3630-6-2), [`RFC3630-6-3`](#rfc3630-6-3), [`RFC3630-6-4`](#rfc3630-6-4)

## Figures

- **MUST bar**: Proof pending 1, Not built 4
- **Overview bar**: Proof pending 1, Not assessed 5, Not built 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 5 |
| MUST rows kept (the MUST bar) | 5 |
| Ze proven | 0 |
| OS verified | 0 |
| Proof pending | 1 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 4 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 5 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 5 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc3630.json](../../../data/rfc/rfc3630.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3630-1-1` | if non-TE capable nodes exist in the topology, they MUST flood TE LSAs as any other type 10 (area-local scope) Opaque LSAs (see [3]). (§1) | MUST | 1 | **positive:** `unit/verify` [`TestRFC3630NonTECapableFloodsTELSAByScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3630_te_test.go#L23). **negative:** `unit/verify` [`TestRFC3630NonTECapableFloodsTELSAByScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3630_te_test.go#L28) |
| `RFC3630-6-1` | o Types in the range 32768-32777 are for experimental use; these will not be registered with IANA, and MUST NOT be mentioned by RFCs. (§6) | MUST NOT | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the sentence binds the authors of RFCs, a role ze never fills, and its passive subject fits neither not-applicable shape; ze mentions no experimental top-level Type and has no behaviour to test |
| `RFC3630-6-2` | Here are the guidelines (using terms defined in [10]) for the assignment of top level Types in TE LSAs: o Types in the range 3-32767 are to be assigned via Standards Action. o Types in the range 32768-32777 are for experimental use; these will not be registered with IANA, and MUST NOT be mentioned by RFCs. o Types in the range 32778-65535 are not to be assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC that specifies IANA Considerations that covers the range being assigned. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the sentences bind IANA and the Standards Track documents that assign top-level TE LSA Types, roles ze never fills, and their passive subjects fit neither not-applicable shape; ze assigns no Type and has no behaviour to test |
| `RFC3630-6-3` | o Types in the range 32768-32777 are for experimental use; these will not be registered with IANA, and MUST NOT be mentioned by RFCs. (§6) | MUST NOT | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the sentence binds the authors of RFCs, a role ze never fills, and its passive subject fits neither not-applicable shape; ze mentions no experimental sub-TLV Type and has no behaviour to test |
| `RFC3630-6-4` | The guidelines for the assignment of types for sub-TLVs in a TE LSA are as follows: o Types in the range 10-32767 are to be assigned via Standards Action. o Types in the range 32768-32777 are for experimental use; these will not be registered with IANA, and MUST NOT be mentioned by RFCs. o Types in the range 32778-65535 are not to be assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC that specifies IANA Considerations that covers the range being assigned. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the sentences bind IANA and the Standards Track documents that assign TE LSA sub-TLV Types, roles ze never fills, and their passive subjects fit neither not-applicable shape; ze assigns no Type and has no behaviour to test |
| `RFC3630-2.4.1-1` | If a router advertises BGP routes with the BGP next hop attribute set to the BGP router ID, then the Router Address SHOULD be the same as the BGP router ID. (§2.4.1) | SHOULD | 2.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3630-2.5.7-1` | This SHOULD be user-configurable; the default value should be the Maximum Bandwidth. (§2.5.7) -- Ze: `max-reservable-bandwidth` leaf, defaulting to `max-bandwidth`, in applyTELinkAttributes | SHOULD | 2.5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC3630-3-1` | In any case, the origination of Traffic Engineering LSAs SHOULD be rate-limited to at most one every MinLSInterval [1]. (§3) -- Ze reuses the carrier's MinLSInterval rate-limit in OriginateSelf; a pull-model unchanged body floods nothing | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3630-2.5.4-1` | If the Link Type of the link is Multi-access, the Remote Interface IP Address is set to 0.0.0.0; alternatively, an implementation MAY choose not to send this sub-TLV. (§2.5.4) -- Ze omits sub-TLV 4 on multi-access links | MAY | 2.5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3630-3-2` | an implementation MAY set thresholds (for example, a bandwidth change threshold) that trigger immediate flooding (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3630-6-1`](#rfc3630-6-1) o Types in the range 32768-32777 are for experimental use; these will not be registered with IANA, and MUST NOT be mentioned by RFCs. (§6) | {gap}, no test | the sentence binds the authors of RFCs, a role ze never fills, and its passive subject fits neither not-applicable shape; ze mentions no experimental top-level Type and has no behaviour to test |
| [`RFC3630-6-2`](#rfc3630-6-2) Here are the guidelines (using terms defined in [10]) for the assignment of top level Types in TE LSAs: o Types in the range 3-32767 are to be assigned via Standards Action. o Types in the range 32768-32777 are for experimental use; these will not be registered with IANA, and MUST NOT be mentioned by RFCs. o Types in the range 32778-65535 are not to be assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC that specifies IANA Considerations that covers the range being assigned. (§6) | {gap}, no test | the sentences bind IANA and the Standards Track documents that assign top-level TE LSA Types, roles ze never fills, and their passive subjects fit neither not-applicable shape; ze assigns no Type and has no behaviour to test |
| [`RFC3630-6-3`](#rfc3630-6-3) o Types in the range 32768-32777 are for experimental use; these will not be registered with IANA, and MUST NOT be mentioned by RFCs. (§6) | {gap}, no test | the sentence binds the authors of RFCs, a role ze never fills, and its passive subject fits neither not-applicable shape; ze mentions no experimental sub-TLV Type and has no behaviour to test |
| [`RFC3630-6-4`](#rfc3630-6-4) The guidelines for the assignment of types for sub-TLVs in a TE LSA are as follows: o Types in the range 10-32767 are to be assigned via Standards Action. o Types in the range 32768-32777 are for experimental use; these will not be registered with IANA, and MUST NOT be mentioned by RFCs. o Types in the range 32778-65535 are not to be assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC that specifies IANA Considerations that covers the range being assigned. (§6) | {gap}, no test | the sentences bind IANA and the Standards Track documents that assign TE LSA sub-TLV Types, roles ze never fills, and their passive subjects fit neither not-applicable shape; ze assigns no Type and has no behaviour to test |

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC3630-1-1`](#rfc3630-1-1)

if non-TE capable nodes exist in the topology, they MUST flood TE LSAs as any other type 10 (area-local scope) Opaque LSAs (see [3]). (§1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-10-21 over current tags, units and records. Row text correction only (trailing implementation note dropped, rfc/corrections/rfc3630.md); the quoted sentence 'they MUST flood TE LSAs as any other type 10 (area-local scope) Opaque LSAs' is unchanged and the units are unchanged: the TE Type-10 LSA reaches the same-area retransmit list and is not flooded outside the area.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3630NonTECapableFloodsTELSAByScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3630_te_test.go#L28) | unit/verify | unproven |
| positive | [`TestRFC3630NonTECapableFloodsTELSAByScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3630_te_test.go#L23) | unit/verify | unproven |

### [`RFC3630-6-1`](#rfc3630-6-1)

o Types in the range 32768-32777 are for experimental use; these will not be registered with IANA, and MUST NOT be mentioned by RFCs. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC3630-6-1, so there is no linked test.

### [`RFC3630-6-2`](#rfc3630-6-2)

Here are the guidelines (using terms defined in [10]) for the assignment of top level Types in TE LSAs: o Types in the range 3-32767 are to be assigned via Standards Action. o Types in the range 32768-32777 are for experimental use; these will not be registered with IANA, and MUST NOT be mentioned by RFCs. o Types in the range 32778-65535 are not to be assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC that specifies IANA Considerations that covers the range being assigned. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC3630-6-2, so there is no linked test.

### [`RFC3630-6-3`](#rfc3630-6-3)

o Types in the range 32768-32777 are for experimental use; these will not be registered with IANA, and MUST NOT be mentioned by RFCs. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC3630-6-3, so there is no linked test.

### [`RFC3630-6-4`](#rfc3630-6-4)

The guidelines for the assignment of types for sub-TLVs in a TE LSA are as follows: o Types in the range 10-32767 are to be assigned via Standards Action. o Types in the range 32768-32777 are for experimental use; these will not be registered with IANA, and MUST NOT be mentioned by RFCs. o Types in the range 32778-65535 are not to be assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC that specifies IANA Considerations that covers the range being assigned. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC3630-6-4, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc3630.txt |
| Source fingerprint | 8ac1c1508da20146 |
| Record | rfc/extraction/rfc3630.json |
| Mapped sentences | 5 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 1 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `1.2` | not stated | 0 | reviewed | not stated |
| `1.3` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `2.2` | not stated | 0 | reviewed | not stated |
| `2.3` | not stated | 0 | reviewed | not stated |
| `2.3.1` | not stated | 0 | reviewed | not stated |
| `2.3.2` | not stated | 0 | reviewed | not stated |
| `2.4` | not stated | 0 | reviewed | not stated |
| `2.4.1` | not stated | 0 | reviewed | not stated |
| `2.4.2` | not stated | 0 | reviewed | not stated |
| `2.5` | not stated | 0 | reviewed | not stated |
| `2.5.1` | not stated | 0 | reviewed | not stated |
| `2.5.2` | not stated | 0 | reviewed | not stated |
| `2.5.3` | not stated | 0 | reviewed | not stated |
| `2.5.4` | not stated | 0 | reviewed | not stated |
| `2.5.5` | not stated | 0 | reviewed | not stated |
| `2.5.6` | not stated | 0 | reviewed | not stated |
| `2.5.7` | not stated | 0 | reviewed | not stated |
| `2.5.8` | not stated | 0 | reviewed | not stated |
| `2.5.9` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 4 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `8.1` | not stated | 0 | reviewed | not stated |
| `8.2` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 3630 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 3630, so its obligations are stated where they were written.
