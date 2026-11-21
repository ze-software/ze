# RFC 6996 - Autonomous System (AS) Reservation for Private Use

No entry in the public support list. Every requirement listed for RFC 6996, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 1 | of 1 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 1 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 1 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 1 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 1 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 8 | 8 of 8 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 1 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 1 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 1 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 1 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 1 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 1 | of 3 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 1 mandatory requirements: they add up to 1. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | No entry in the public support list |
| Automatic checks | Included |
| Requirements | 3 |
| Mandatory requirements (MUST level) | 1 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tests linked to requirements | 8 |
| Recorded audit verdicts | 1 |
| Fault test records | 8 |
| Summary | `rfc/short/rfc6996.md` |
| Requirement table | `rfc/requirements/rfc6996.md` |
| RFC text | `rfc/full/rfc6996.txt` |

## Automatic checks

Included in automatic checks: Private Use ASN removal: the sole MUST (RFC6996-4-1) requires Private Use ASNs be stripped from the AS path before advertisement to the global Internet, which the remove-private-as filter enforces (internal/component/bgp/plugins/filter_remove_private_as). The positive/negative pair pins the strip to exactly the 64512-65534 / 4200000000-4294967294 ranges, so a globally-routable ASN is never removed. The 4-2 SHOULD and 4-3 MAY are operator guidance, not gated.

## Published support status

No entry in the public support list, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 6996.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **1** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (1):** [`RFC6996-4-1`](#rfc6996-4-1)

## Figures

- **MUST bar**: Ze verified 1
- **Overview bar**: Ze MUST verified 1, Not assessed 2

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 1 |
| MUST rows kept (the MUST bar) | 1 |
| Ze verified | 1 |
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
| SHOULD / MAY rows | 2 |
| SHOULD verified | 0 |
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

Source: [data/rfc/rfc6996.json](../../../data/rfc/rfc6996.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6996-4-1` | If Private Use ASNs are used and prefixes originate from these ASNs, Private Use ASNs MUST be removed from AS path attributes (including AS4_PATH if utilizing a four-octet AS number space) before being advertised to the global Internet. (Section 4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC6996ExportRemovesPrivateUseASNsFromASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6996_remove_private_test.go#L80). **positive:** `unit/verify` [`TestRFC6996OldSpeakerReceivesStrippedASPathAndAS4Path`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6996_old_speaker_export_test.go#L23). **positive:** `unit/verify` [`TestRFC6996RemovesPrivateUseASNsFromBothPathAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/rfc6996_private_as_attributes_test.go#L40). **positive:** `unit/verify` [`TestRFC6996StripRemovesPrivateUseASNsFromAS4Path`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6996_remove_private_test.go#L40). **positive:** `unit/verify` [`TestRFC6996StripsPrivateUseASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/rfc6996_private_as_test.go#L50). **negative:** `unit/verify` [`TestRFC6996KeepsASNsOutsideThePrivateUseRanges`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/rfc6996_private_as_attributes_test.go#L57). **negative:** `unit/verify` [`TestRFC6996KeepsPublicASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/rfc6996_private_as_test.go#L60). **negative:** `unit/verify` [`TestRFC6996StripKeepsAS4PathASNsOutsideThePrivateUseRanges`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6996_remove_private_test.go#L56) |
| `RFC6996-4-2` | Operators SHOULD ensure that all External Border Gateway Protocol (EBGP) speakers support the extensions described in [RFC6793] and that implementation-specific features that recognize Private Use ASNs have been updated to recognize both ranges prior to making use of the newer, numerically higher range of Private Use ASNs in the four-octet AS number space. (Section 4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6996-4-3` | Normal AS path filtering MAY also be used to prevent prefixes originating from Private Use ASNs from being advertised to the global Internet (Section 4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 6996 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC6996-4-1`](#rfc6996-4-1)

If Private Use ASNs are used and prefixes originate from these ASNs, Private Use ASNs MUST be removed from AS path attributes (including AS4_PATH if utilizing a four-octet AS number space) before being advertised to the global Internet. (Section 4)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/reactor/rfc6996_remove_private_test.go::TestRFC6996ExportRemovesPrivateUseASNsFromASPath moved. Re-judged 2026-11-03 by an independent auditor who authored none of 6b105ebc58 or c782411791. RFC 6996 Section 4 read: "Private Use ASNs MUST be removed from AS path attributes (including AS4_PATH if utilizing a four-octet AS number space) before being advertised to the global Internet." TestRFC6996OldSpeakerReceivesStrippedASPathAndAS4Path changed one line: the literal one-step chain list became buildOrderedEgressSteps(), the production builder returning the same export-chain step; the as-path remove private member and the exact stripped AS_PATH and AS4_PATH octets are untouched. Scoped run green. Its mutant record is unit-changed and owes a re-record. Verdict unchanged. Earlier judgement, still standing for the unmoved units: Re-judged 2026-11-02 by an independent auditor who authored none of the changes named. RFC 6996 Section 4: "Private Use ASNs MUST be removed from AS path attributes (including AS4_PATH if utilizing a four-octet AS number space) before being advertised to the global Internet." TestRFC6996ExportRemovesPrivateUseASNsFromASPath changed in 76c2b9156a (runEgressPolicyChainASN4 now takes the session facts, passed as policyview.Session{EBGP: true}, matching the EBGP destination the unit already names) and 5c1aea079c (comment spelling). The same four-octet AS_PATH [64511 64512 65534 65535 4199999999 4200000000 4294967294 4294967295] must still come out exactly [64511 65535 4199999999 4294967295] with accept and a wire override, so both edges of both ranges and their neighbours stay pinned. Other carriers byte-identical. Verdict unchanged: enforced.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6996KeepsASNsOutsideThePrivateUseRanges`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/rfc6996_private_as_attributes_test.go#L57) | unit/verify | mutant, verified |
| negative | [`TestRFC6996KeepsPublicASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/rfc6996_private_as_test.go#L60) | unit/verify | mutant, verified |
| negative | [`TestRFC6996StripKeepsAS4PathASNsOutsideThePrivateUseRanges`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6996_remove_private_test.go#L56) | unit/verify | revert, verified |
| positive | [`TestRFC6996RemovesPrivateUseASNsFromBothPathAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/rfc6996_private_as_attributes_test.go#L40) | unit/verify | mutant, verified |
| positive | [`TestRFC6996StripsPrivateUseASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/rfc6996_private_as_test.go#L50) | unit/verify | mutant, verified |
| positive | [`TestRFC6996OldSpeakerReceivesStrippedASPathAndAS4Path`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6996_old_speaker_export_test.go#L23) | unit/verify | mutant, verified |
| positive | [`TestRFC6996ExportRemovesPrivateUseASNsFromASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6996_remove_private_test.go#L80) | unit/verify | revert, verified |
| positive | [`TestRFC6996StripRemovesPrivateUseASNsFromAS4Path`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6996_remove_private_test.go#L40) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc6996.txt |
| Source fingerprint | 38ea131bbe5b38d2 |
| Record | rfc/extraction/rfc6996.json |
| Mapped sentences | 1 |
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
| `4` | not stated | 1 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `7.1` | not stated | 0 | reviewed | not stated |
| `7.2` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 6996 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 6996, so its obligations are stated where they were written.
