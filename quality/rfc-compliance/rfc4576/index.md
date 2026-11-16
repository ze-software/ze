# RFC 4576 - Using a Link State Advertisement (LSA) Options Bit to Prevent Looping in BGP/MPLS IP Virtual Private Networks (VPNs)

OSPF DN bit for BGP/MPLS IP VPNs. Every requirement listed for RFC 4576, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 4 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 4 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 4 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 4 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 4 | of 4 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 4 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 4 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 4 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 4 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 4 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 4 | of 4 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 4 mandatory requirements: they add up to 4. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | OSPF DN bit for BGP/MPLS IP VPNs |
| Automatic checks | Included |
| Requirements | 4 |
| Mandatory requirements (MUST level) | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc4576.md` |
| Requirement table | `rfc/requirements/rfc4576.md` |
| RFC text | `rfc/full/rfc4576.txt` |

## Automatic checks

Included in automatic checks: Using an LSA Options Bit (DN bit) to Prevent Looping in BGP/MPLS IP VPNs: four MUST-level requirements, all gaps: two owed with OSPF PE-CE support (4-1, 4-4) and two met because ze never sets or reads the DN bit, untested (4-2, 4-3). RFC 4576 governs OSPF running as a BGP/MPLS-VPN PE-CE protocol (the DN bit prevents PE-CE loops). Ze does NOT run OSPF as an MPLS-VPN PE-CE protocol -- it has no L3VPN OSPF code path (no VPN OSPF route redistribution, no sham links, no PE-CE OSPF; grep across internal/plugins/ospf/ for sham-link/PE-CE/vpn-ospf/domain-id finds nothing), running OSPF only as a plain IGP. RFC4576-4-1 (PE sets DN bit on type 3/5/7 LSA to CE) and RFC4576-4-2 (DN bit clear on all other LSA types) have no PE origination code path; RFC4576-4-3 (ignore DN bit on other LSA types) -- Ze decodes the DN bit as a field (internal/plugins/ospf/v3/types/prefix.go:68) but performs no VPN loop-prevention acting on it; RFC4576-4-4 (PE MUST NOT use a CE DN-bit-set LSA in route calc) has no PE route-calc code path. No SHOULD/MAY requirements are gated.

## Published support status

**Area:** OSPF DN bit for BGP/MPLS IP VPNs

**Test verification:** 0 of 4 MUST proven

**Evidence:** -

**Unmet:** RFC4576-4-1, RFC4576-4-2, RFC4576-4-3, RFC4576-4-4


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 4 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **4** | every mandatory requirement appears in exactly one group above |

**Annotated (including scoped evidence) (4):** [`RFC4576-4-1`](#rfc4576-4-1), [`RFC4576-4-2`](#rfc4576-4-2), [`RFC4576-4-3`](#rfc4576-4-3), [`RFC4576-4-4`](#rfc4576-4-4)

## Figures

- **MUST bar**: Not built 4
- **Overview bar**: Not built 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 4 |
| MUST rows kept (the MUST bar) | 4 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 4 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 0 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 0 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc4576.json](../../../data/rfc/rfc4576.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4576-4-1` | When a type 3, 5, or 7 LSA is sent from a PE to a CE, the DN bit MUST be set. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze does not run OSPF as a BGP/MPLS VPN PE-CE protocol: internal/plugins/ospf has no VPN route redistribution, sham link or PE-CE code path, so it sends no type 3, 5 or 7 LSA from a PE to a CE with the DN bit set; owed with OSPF PE-CE support |
| `RFC4576-4-2` | The DN bit MUST be clear in all other LSA types. +-------------------------------------+ \| DN \| * \| DC \| EA \| N/P \| MC \| E \| * \| +-------------------------------------+ (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: ze meets it by never setting the DN bit: types.OptionDN (internal/plugins/ospf/types/options.go) and OptPrefixDN (internal/plugins/ospf/v3/types/prefix.go) are named outside their packages only by internal/plugins/ospf/neighbor_detail.go, for display, and no LSA producer sets either; no test asserts it |
| `RFC4576-4-3` | The DN bit MUST be ignored in all other LSA types. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: ze meets it by acting on the DN bit in no LSA type: no route calculation reads types.OptionDN or PrefixOptions.Down (internal/plugins/ospf/v3/types/prefix.go), which only internal/plugins/ospf/neighbor_detail.go names, for display; no test asserts it |
| `RFC4576-4-4` | When the PE receives, from a CE router, a type 3, 5, or 7 LSA with the DN bit set, the information from that LSA MUST NOT be used during the OSPF route calculation. (§4) | MUST NOT | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs no PE OSPF route calculation for a VPN, so it does not exclude a CE's DN-bit-set type 3, 5 or 7 LSA from one; owed with OSPF PE-CE support |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4576-4-1`](#rfc4576-4-1) When a type 3, 5, or 7 LSA is sent from a PE to a CE, the DN bit MUST be set. (§4) | {gap}, no test | ze does not run OSPF as a BGP/MPLS VPN PE-CE protocol: internal/plugins/ospf has no VPN route redistribution, sham link or PE-CE code path, so it sends no type 3, 5 or 7 LSA from a PE to a CE with the DN bit set; owed with OSPF PE-CE support |
| [`RFC4576-4-2`](#rfc4576-4-2) The DN bit MUST be clear in all other LSA types. +-------------------------------------+ \| DN \| * \| DC \| EA \| N/P \| MC \| E \| * \| +-------------------------------------+ (§4) | {gap}, no test | gap in proof: ze meets it by never setting the DN bit: types.OptionDN (internal/plugins/ospf/types/options.go) and OptPrefixDN (internal/plugins/ospf/v3/types/prefix.go) are named outside their packages only by internal/plugins/ospf/neighbor_detail.go, for display, and no LSA producer sets either; no test asserts it |
| [`RFC4576-4-3`](#rfc4576-4-3) The DN bit MUST be ignored in all other LSA types. (§4) | {gap}, no test | gap in proof: ze meets it by acting on the DN bit in no LSA type: no route calculation reads types.OptionDN or PrefixOptions.Down (internal/plugins/ospf/v3/types/prefix.go), which only internal/plugins/ospf/neighbor_detail.go names, for display; no test asserts it |
| [`RFC4576-4-4`](#rfc4576-4-4) When the PE receives, from a CE router, a type 3, 5, or 7 LSA with the DN bit set, the information from that LSA MUST NOT be used during the OSPF route calculation. (§4) | {gap}, no test | ze runs no PE OSPF route calculation for a VPN, so it does not exclude a CE's DN-bit-set type 3, 5 or 7 LSA from one; owed with OSPF PE-CE support |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC4576-4-1`](#rfc4576-4-1)

When a type 3, 5, or 7 LSA is sent from a PE to a CE, the DN bit MUST be set. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4576-4-1, so there is no linked test.

### [`RFC4576-4-2`](#rfc4576-4-2)

The DN bit MUST be clear in all other LSA types. +-------------------------------------+ | DN | * | DC | EA | N/P | MC | E | * | +-------------------------------------+ (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4576-4-2, so there is no linked test.

### [`RFC4576-4-3`](#rfc4576-4-3)

The DN bit MUST be ignored in all other LSA types. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4576-4-3, so there is no linked test.

### [`RFC4576-4-4`](#rfc4576-4-4)

When the PE receives, from a CE router, a type 3, 5, or 7 LSA with the DN bit set, the information from that LSA MUST NOT be used during the OSPF route calculation. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4576-4-4, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc4576.txt |
| Source fingerprint | b8082c52301f1b60 |
| Record | rfc/extraction/rfc4576.json |
| Mapped sentences | 4 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 4 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 4576 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 4576, so its obligations are stated where they were written.
