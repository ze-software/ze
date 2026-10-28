# RFC 8212 - Default External BGP (EBGP) Route Propagation Behavior without Policies

Unsupported. Every requirement this repository extracted from RFC 8212, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 2 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 2 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 2 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 2 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 2 | of 3 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 2 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 2 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 2 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 2 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 2 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 2 of 2 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 shares marked as a part above are the whole of the 2 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Unsupported |
| Enrolment | Enrolled |
| Requirements | 3 |
| Gated MUST-level | 2 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 2 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8212.md` |
| Requirement shard | `rfc/requirements/rfc8212.md` |
| RFC text | `rfc/full/rfc8212.txt` |

## Enrolment

Enrolled: Two MUST-level requirements, both in Section 3 and both at SHALL NOT: an EBGP peer's received routes are not eligible for the Decision Process without an explicit Import Policy (RFC8212-3-1), and nothing is added to an EBGP peer's Adj-RIB-Out without an explicit Export Policy (RFC8212-3-2). Both are recorded as `{gap}`: Ze treats an absent operator policy as accept-at-exhaustion for every peer, internal or external (internal/component/bgp/filterapi/policy.go::Policy.AcceptsAtEnd, read by internal/component/bgp/reactor/filter_ordered.go::runIngressPolicyChain and runEgressPolicyChain). The one MAY (RFC8212-3-3, a configuration option to deviate) is not gated. The walk is recorded in rfc/extraction/rfc8212.json at register rfc2119, both sites mapped. Enrolled 2026-10-26.

## What the public ledger says

**Status:** Unsupported

**What the ledger says is covered**

Ze requires every configured import or export policy to state `final accept` or `final reject`, so an operator can write default reject explicitly per peer, per group or globally. Ze does not apply RFC 8212's default: an EBGP session with no import policy and no export policy accepts and advertises every route.

**What the ledger says remains**

[`RFC8212-3-1`](#rfc8212-3-1) and [`RFC8212-3-2`](#rfc8212-3-2) are gaps. An absent policy accepts at exhaustion for EBGP peers (`Policy.AcceptsAtEnd`, [`internal/component/bgp/filterapi/policy.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/filterapi/policy.go)). The default-reject behaviour, and the optional switch to deviate from it, are owed.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **2** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (2):** [`RFC8212-3-1`](#rfc8212-3-1), [`RFC8212-3-2`](#rfc8212-3-2)

## Attainable ceiling

Mandatory ceiling: 2 of 2 (100.0%). Optional ceiling: 1 of 1 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc8212 2 of 2 (100.0%). Lowest optional ceiling: rfc8212 1 of 1 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8212-3-1` | Routes contained in an Adj-RIB-In associated with an EBGP peer SHALL NOT be considered eligible in the Decision Process if no explicit Import Policy has been applied. (§3) | SHALL NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze treats an absent import policy as accept for every peer, EBGP included -- internal/component/bgp/filterapi/policy.go::Policy.AcceptsAtEnd returns true for the zero Policy, and internal/component/bgp/reactor/filter_ordered.go::runIngressPolicyChain accepts on it |
| `RFC8212-3-2` | Routes SHALL NOT be added to an Adj-RIB-Out associated with an EBGP peer if no explicit Export Policy has been applied. (§3) | SHALL NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze treats an absent export policy as accept for every peer, EBGP included -- internal/component/bgp/filterapi/policy.go::Policy.AcceptsAtEnd returns true for the zero Policy, and internal/component/bgp/reactor/filter_ordered.go::runEgressPolicyChain accepts on it |
| `RFC8212-3-3` | A BGP speaker MAY provide a configuration option to deviate from the following updated behaviors. (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8212-3-1`](#rfc8212-3-1) Routes contained in an Adj-RIB-In associated with an EBGP peer SHALL NOT be considered eligible in the Decision Process if no explicit Import Policy has been applied. (§3) | {gap}, no test | Ze treats an absent import policy as accept for every peer, EBGP included -- internal/component/bgp/filterapi/policy.go::Policy.AcceptsAtEnd returns true for the zero Policy, and internal/component/bgp/reactor/filter_ordered.go::runIngressPolicyChain accepts on it |
| [`RFC8212-3-2`](#rfc8212-3-2) Routes SHALL NOT be added to an Adj-RIB-Out associated with an EBGP peer if no explicit Export Policy has been applied. (§3) | {gap}, no test | Ze treats an absent export policy as accept for every peer, EBGP included -- internal/component/bgp/filterapi/policy.go::Policy.AcceptsAtEnd returns true for the zero Policy, and internal/component/bgp/reactor/filter_ordered.go::runEgressPolicyChain accepts on it |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8212-3-1`](#rfc8212-3-1)

Routes contained in an Adj-RIB-In associated with an EBGP peer SHALL NOT be considered eligible in the Decision Process if no explicit Import Policy has been applied. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8212-3-1, so no unit is bound to it.

### [`RFC8212-3-2`](#rfc8212-3-2)

Routes SHALL NOT be added to an Adj-RIB-Out associated with an EBGP peer if no explicit Export Policy has been applied. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8212-3-2, so no unit is bound to it.

## Extraction sign-off

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
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `3` | not stated | 2 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | No IANA actions | 0 | skipped (iana) | No IANA actions |
| `6` | References | 0 | skipped (references) | References |
| `6.1` | Normative references | 0 | skipped (references) | Normative references |
| `6.2` | Informative references | 0 | skipped (references) | Informative references |
| `A` | Appendix A lists implementations, no obligation | 0 | skipped (appendix-non-normative) | Appendix A lists implementations, no obligation |
| `A.1` | Implementation list, no obligation | 0 | skipped (appendix-non-normative) | Implementation list, no obligation |

### Excluded sentences

The walk over RFC 8212 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 8212, so its obligations are stated where they were written.
