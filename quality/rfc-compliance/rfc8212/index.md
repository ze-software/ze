# RFC 8212 - Default External BGP (EBGP) Route Propagation Behavior without Policies

EBGP default reject without policy. Every requirement listed for RFC 8212, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 2 | of 2 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 2 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 2 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 2 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 2 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 7 | 7 of 8 tests linked to requirements, 0 without a fault test and 1 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 2 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 2 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 2 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 2 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 2 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 2 | of 3 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 2 mandatory requirements: they add up to 2. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | EBGP default reject without policy |
| Automatic checks | Included |
| Requirements | 3 |
| Mandatory requirements (MUST level) | 2 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tests linked to requirements | 8 |
| Recorded audit verdicts | 0 |
| Fault test records | 8 |
| Summary | `rfc/short/rfc8212.md` |
| Requirement table | `rfc/requirements/rfc8212.md` |
| RFC text | `rfc/full/rfc8212.txt` |

## Automatic checks

Included in automatic checks: Two MUST-level requirements, both in Section 3 and both at SHALL NOT: an EBGP peer's received routes are not eligible for the Decision Process without an explicit Import Policy (RFC8212-3-1), and nothing is added to an EBGP peer's Adj-RIB-Out without an explicit Export Policy (RFC8212-3-2). Ze meets both: an absent policy rejects at exhaustion for an EBGP session and accepts for an IBGP one (internal/component/bgp/filterapi/policy.go::Policy.AcceptsAtEnd), read by internal/component/bgp/reactor/filter_ordered.go::runIngressPolicyChain and runEgressPolicyChain. The one MAY (RFC8212-3-3, a configuration option to deviate) is not gated, and Ze offers no such option. The walk is recorded in rfc/extraction/rfc8212.json at register rfc2119, both sites mapped. Enrolled 2026-10-26.

## Published support status

**Area:** EBGP default reject without policy

**Test verification:** 2 of 2 MUST proven

**Evidence:** interop 1, functional 1

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **2** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (2):** [`RFC8212-3-1`](#rfc8212-3-1), [`RFC8212-3-2`](#rfc8212-3-2)

## Figures

- **MUST bar**: Ze verified 2
- **Overview bar**: Ze MUST verified 2, Not assessed 1

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 2 |
| MUST rows kept (the MUST bar) | 2 |
| Ze verified | 2 |
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
| SHOULD / MAY rows | 1 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 1 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc8212.json](../../../data/rfc/rfc8212.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8212-3-1` | Routes contained in an Adj-RIB-In associated with an EBGP peer SHALL NOT be considered eligible in the Decision Process if no explicit Import Policy has been applied. (§3) | SHALL NOT | 3 | **negative:** `unit/verify` [`TestRFC8212Section3AbsentImportRejectsOnEBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8212_default_reject_test.go#L133). **positive:** `functional/verify` [`rfc8212-ebgp-no-import-policy.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc8212-ebgp-no-import-policy.ci#L36). **negative:** `functional/verify` [`rfc8212-ebgp-no-import-policy.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc8212-ebgp-no-import-policy.ci#L33). **negative:** `interop/nightly` [`checkRFC8212DefaultRejectFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc8212_default_reject.go#L77) |
| `RFC8212-3-2` | Routes SHALL NOT be added to an Adj-RIB-Out associated with an EBGP peer if no explicit Export Policy has been applied. (§3) | SHALL NOT | 3 | **negative:** `unit/verify` [`TestRFC8212Section3AbsentExportRejectsOnEBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8212_default_reject_test.go#L185). **positive:** `functional/verify` [`rfc8212-ebgp-no-export-policy.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc8212-ebgp-no-export-policy.ci#L39). **positive:** `functional/verify` [`rfc8212-explicit-policy-reload.ci`](https://github.com/ze-software/ze/blob/main/test/reload/rfc8212-explicit-policy-reload.ci#L30). **negative:** `functional/verify` [`rfc8212-ebgp-no-export-policy.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc8212-ebgp-no-export-policy.ci#L35) |
| `RFC8212-3-3` | A BGP speaker MAY provide a configuration option to deviate from the following updated behaviors. (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 8212 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC8212-3-1`](#rfc8212-3-1)

Routes contained in an Adj-RIB-In associated with an EBGP peer SHALL NOT be considered eligible in the Decision Process if no explicit Import Policy has been applied. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8212Section3AbsentImportRejectsOnEBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8212_default_reject_test.go#L133) | unit/verify | mutant, verified |
| negative | [`checkRFC8212DefaultRejectFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc8212_default_reject.go#L77) | interop/nightly | mutant, verified |
| negative | [`rfc8212-ebgp-no-import-policy.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc8212-ebgp-no-import-policy.ci#L33) | functional/verify | mutant, verified |
| positive | [`rfc8212-ebgp-no-import-policy.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc8212-ebgp-no-import-policy.ci#L36) | functional/verify | mutant, verified |

### [`RFC8212-3-2`](#rfc8212-3-2)

Routes SHALL NOT be added to an Adj-RIB-Out associated with an EBGP peer if no explicit Export Policy has been applied. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8212Section3AbsentExportRejectsOnEBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8212_default_reject_test.go#L185) | unit/verify | mutant, unit-changed (the tagged unit's behavior changed since the red was observed) |
| negative | [`rfc8212-ebgp-no-export-policy.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc8212-ebgp-no-export-policy.ci#L35) | functional/verify | mutant, verified |
| positive | [`rfc8212-ebgp-no-export-policy.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc8212-ebgp-no-export-policy.ci#L39) | functional/verify | mutant, verified |
| positive | [`rfc8212-explicit-policy-reload.ci`](https://github.com/ze-software/ze/blob/main/test/reload/rfc8212-explicit-policy-reload.ci#L30) | functional/verify | mutant, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude (spec-rfc-profile-declared-membership phase 1) |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc8212.txt |
| Source fingerprint | 2049ae390841ef7d |
| Record | rfc/extraction/rfc8212.json |
| Mapped sentences | 2 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | Title page, status and copyright | 0 | skipped (front-matter) | Title page, status and copyright |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 2 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | No IANA actions | 0 | skipped (iana) | No IANA actions |
| `6` | References | 0 | skipped (references) | References |
| `6.1` | Normative references | 0 | skipped (references) | Normative references |
| `6.2` | Informative references | 0 | skipped (references) | Informative references |
| `A` | Appendix A lists implementations, no obligation | 0 | skipped (appendix-non-normative) | Appendix A lists implementations, no obligation |
| `A.1` | Implementation list, no obligation | 0 | skipped (appendix-non-normative) | Implementation list, no obligation |

### Excluded sentences

The review of RFC 8212 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 8212, so its obligations are stated where they were written.
