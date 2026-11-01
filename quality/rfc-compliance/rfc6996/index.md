# RFC 6996 - Autonomous System (AS) Reservation for Private Use

No row in the public ledger. Every requirement this repository extracted from RFC 6996, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 1 | of 1 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 1 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 1 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 1 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0 | of 1 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 4 | 4 of 6 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 1 | of 1 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 1 | of 3 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 1 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 1 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 1 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 1 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 1 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

The 9 counts marked as a part above are the whole of the 1 gated MUSTs: they add up to 1. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 3 |
| Gated MUST-level | 1 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 6 |
| Tagged units | 6 |
| Recorded audit verdicts | 1 |
| Discrimination records | 4 |
| Summary | `rfc/short/rfc6996.md` |
| Requirement shard | `rfc/requirements/rfc6996.md` |
| RFC text | `rfc/full/rfc6996.txt` |

## Enrolment

Enrolled: Private Use ASN removal: the sole MUST (RFC6996-4-1) requires Private Use ASNs be stripped from the AS path before advertisement to the global Internet, which the remove-private-as filter enforces (internal/component/bgp/plugins/filter_remove_private_as). The positive/negative pair pins the strip to exactly the 64512-65534 / 4200000000-4294967294 ranges, so a globally-routable ASN is never removed. The 4-2 SHOULD and 4-3 MAY are operator guidance, not gated.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 6996.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **1** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC6996-4-1`](#rfc6996-4-1)

## Figures

- **MUST bar**: Ze proven 1
- **Overview bar**: Ze MUST proven 1, SHOULD not decided 2

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | complete |
| MUST rows | 1 |
| MUST rows kept (the MUST bar) | 1 |
| Ze proven | 1 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 2 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 2 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc6996.json](../../../data/rfc/rfc6996.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6996-4-1` | If Private Use ASNs are used and prefixes originate from these ASNs, Private Use ASNs MUST be removed from AS path attributes (including AS4_PATH if utilizing a four-octet AS number space) before being advertised to the global Internet. (Section 4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC6996RemovesPrivateUseASNsFromBothPathAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/rfc6996_private_as_attributes_test.go#L35). **positive:** `unit/verify` [`TestRFC6996StripRemovesPrivateUseASNsFromAS4Path`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6996_remove_private_test.go#L32). **positive:** `unit/verify` [`TestRFC6996StripsPrivateUseASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/rfc6996_private_as_test.go#L47). **negative:** `unit/verify` [`TestRFC6996KeepsASNsOutsideThePrivateUseRanges`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/rfc6996_private_as_attributes_test.go#L62). **negative:** `unit/verify` [`TestRFC6996KeepsPublicASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/rfc6996_private_as_test.go#L56). **negative:** `unit/verify` [`TestRFC6996StripKeepsAS4PathASNsOutsideThePrivateUseRanges`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6996_remove_private_test.go#L48) |
| `RFC6996-4-2` | Operators SHOULD ensure that all External Border Gateway Protocol (EBGP) speakers support the extensions described in [RFC6793] and that implementation-specific features that recognize Private Use ASNs have been updated to recognize both ranges prior to making use of the newer, numerically higher range of Private Use ASNs in the four-octet AS number space. (Section 4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6996-4-3` | Normal AS path filtering MAY also be used to prevent prefixes originating from Private Use ASNs from being advertised to the global Internet (Section 4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 6996 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6996-4-1`](#rfc6996-4-1)

If Private Use ASNs are used and prefixes originate from these ASNs, Private Use ASNs MUST be removed from AS path attributes (including AS4_PATH if utilizing a four-octet AS number space) before being advertised to the global Internet. (Section 4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both clauses, both polarities, at the range edges. Positive: plugin TestRFC6996RemovesPrivateUseASNsFromBothPathAttributes rewrites AS_PATH [64496 64512 65534 4200000000 4294967294 64497] to [64496 64497] and draws the strip directive for an AS4_PATH holding 4294967294; reactor TestRFC6996StripRemovesPrivateUseASNsFromAS4Path asserts the exact AS4_PATH rewrite to [64496 64497]. Negative: 64511/65535/4199999999/4294967295 kept in AS_PATH and AS4_PATH at both tiers (accept, zero ops), so an off-by-one bound in either range goes red. All four new units carry observed-red revert records (isPrivateASN, isRFC6996PrivateASN). The two older private_as_test.go units are subsets with no record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6996KeepsASNsOutsideThePrivateUseRanges`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/rfc6996_private_as_attributes_test.go#L62) | unit/verify | revert, verified |
| negative | [`TestRFC6996KeepsPublicASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/rfc6996_private_as_test.go#L56) | unit/verify | unproven |
| negative | [`TestRFC6996StripKeepsAS4PathASNsOutsideThePrivateUseRanges`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6996_remove_private_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestRFC6996RemovesPrivateUseASNsFromBothPathAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/rfc6996_private_as_attributes_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestRFC6996StripsPrivateUseASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_remove_private_as/rfc6996_private_as_test.go#L47) | unit/verify | unproven |
| positive | [`TestRFC6996StripRemovesPrivateUseASNsFromAS4Path`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6996_remove_private_test.go#L32) | unit/verify | revert, verified |

## Extraction sign-off

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
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 6996 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 6996, so its obligations are stated where they were written.
