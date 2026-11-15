# RFC 5635 - Remote Triggered Black Hole Filtering with Unicast Reverse Path Forwarding (uRPF)

RTBH filtering with uRPF. Every requirement listed for RFC 5635, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 0 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 0 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 0 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 0 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 0 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |
| Audit verdicts | 0 | of 0 mandatory requirements reviewed | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 0 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 0 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 0 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 0 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 0 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 0 | of 2 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

No card above is a part of a population, so there is nothing to add up.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | RTBH filtering with uRPF |
| Automatic checks | Included |
| Requirements | 2 |
| Mandatory requirements (MUST level) | 0 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc5635.md` |
| Requirement table | `rfc/requirements/rfc5635.md` |
| RFC text | `rfc/full/rfc5635.txt` |

## Automatic checks

Included in automatic checks: One MUST-level sentence (§3.2, customer-triggered RTBH): an operator who lets customers trigger blackholing MUST only accept announcements for prefixes the customer is authorized to advertise. RFC5635-3.2-1 restates the first condition RFC 7999 Section 3.3 states for the same customer-triggered blackhole, and Ze answers both through one producer (internal/component/bgp/plugins/rib/rib_blackhole.go::blackholeRouteType, which honours the blackhole only when coveredByAuthorized finds the prefix inside the peer's `blackhole prefixes`). The row is therefore a `{rollup}` onto RFC7999-3.3-1 and gates nothing of its own. The one SHOULD NOT (§4.1, source-based RTBH from peers or customers) is not gated. The walk is recorded in rfc/extraction/rfc5635.json at register rfc2119, its one site mapped. Enrolled 2026-10-26.

## Published support status

**Area:** RTBH filtering with uRPF

**Proof:** no MUST rows

**Evidence:** -

**Unmet:** -


## Coverage

RFC 5635 lists no mandatory requirements for these checks, so nothing is counted here.

## Figures

- **MUST bar**: no rows
- **Overview bar**: Not assessed 1

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 0 |
| MUST rows kept (the MUST bar) | 0 |
| Ze proven | 0 |
| OS verified | 0 |
| Proof pending | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 1 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 1 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc5635.json](../../../data/rfc/rfc5635.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5635-3.2-1` | If this is configured, an operator MUST only accept announcements from the customer for prefixes that the customer is authorized to advertise, in order to prevent the customer from accidentally (or intentionally) black- holing space that they are not allowed to advertise. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** RFC7999-3.3-1; RFC 7999 Section 3.3 states the same condition for the same customer-triggered blackhole, that the announced prefix is covered by one the neighbour is authorized to advertise, and Ze enforces both in internal/component/bgp/plugins/rib/rib_blackhole.go::blackholeRouteType, so this sentence asserts nothing that row does not carry. **derived:** met |
| `RFC5635-4.1-1` | As a matter of policy, operators SHOULD NOT accept source-based RTBH announcements from their peers or customers, they should only be installed by local or attack management systems within their administrative domain. (§4.1) | SHOULD NOT | 4.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5635-3.2-1`](#rfc5635-3.2-1) If this is configured, an operator MUST only accept announcements from the customer for prefixes that the customer is authorized to advertise, in order to prevent the customer from accidentally (or intentionally) black- holing space that they are not allowed to advertise. (§3.2) | no test | no test carries this requirement id; annotated {rollup}: RFC7999-3.3-1; RFC 7999 Section 3.3 states the same condition for the same customer-triggered blackhole, that the announced prefix is covered by one the neighbour is authorized to advertise, and Ze enforces both in internal/component/bgp/plugins/rib/rib_blackhole.go::blackholeRouteType, so this sentence asserts nothing that row does not carry |

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC5635-3.2-1`](#rfc5635-3.2-1)

If this is configured, an operator MUST only accept announcements from the customer for prefixes that the customer is authorized to advertise, in order to prevent the customer from accidentally (or intentionally) black- holing space that they are not allowed to advertise. (§3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5635-3.2-1, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude (spec-rfc-profile-declared-membership phase 1) |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc5635.txt |
| Source fingerprint | d5c6de8647891bec |
| Record | rfc/extraction/rfc5635.json |
| Mapped sentences | 1 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | Title page, status, copyright and contents | 0 | skipped (front-matter) | Title page, status, copyright and contents |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 0 | reviewed | not stated |
| `3.2` | not stated | 1 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments |
| `7` | References | 0 | skipped (references) | References |
| `7.1` | Normative references | 0 | skipped (references) | Normative references |
| `7.2` | Informative references | 0 | skipped (references) | Informative references |
| `A` | not stated | 0 | reviewed | not stated |
| `B` | not stated | 0 | reviewed | not stated |
| `C` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 5635 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 5635, so its obligations are stated where they were written.
