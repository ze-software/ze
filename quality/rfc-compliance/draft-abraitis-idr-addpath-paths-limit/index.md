# DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT - Scalability Considerations for ADD-PATH with PATHS-LIMIT

Supported. Every requirement this repository extracted from DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 75.0% | 3 of 4 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 25.0% | 1 of 4 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 4 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 7 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 7 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 4 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 7 |
| Gated MUST-level | 4 |
| Obligations that bind Ze | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 7 |
| Tagged units | 7 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/draft-abraitis-idr-addpath-paths-limit.md` |
| Requirement shard | `rfc/requirements/draft-abraitis-idr-addpath-paths-limit.md` |
| RFC text | `rfc/drafts/draft-abraitis-idr-addpath-paths-limit.txt` |

## Enrolment

Enrolled: BGP ADD-PATH Paths-Limit capability (code 76): four MUST-level requirements, all met and test-bound. 3-2 (limit ignored when ADD-PATH capability absent), 3-3 (per-family limit ignored when the AFI/SAFI was not in ADD-PATH), 3-4 (duplicate tuple: first considered, others ignored) each carry positive+negative tags on internal/core/bgp/capability negotiation/parse tests; 3-1 (a single PATHS-LIMIT capability instance carries all families) is {single-polarity: positive} bound to a new reactor emit test, since the encoder appends exactly one capability.PathsLimit by construction (internal/component/bgp/reactor/config_capabilities.go:388-391).

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

Per-family path-count limit capability for ADD-PATH.

**What the ledger says remains:**

-

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-2`](#draft-abraitis-idr-addpath-paths-limit-3-2), [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-3`](#draft-abraitis-idr-addpath-paths-limit-3-3), [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-4`](#draft-abraitis-idr-addpath-paths-limit-3-4)

**Annotated instead of tested (1):** [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-1`](#draft-abraitis-idr-addpath-paths-limit-3-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-1` | A BGP speaker wishing to indicate support for multiple AFI/SAFIs "MUST do so by including the information in a single instance of the PATHS-LIMIT capability" (§3) | MUST | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestParsePeerCapabilityPathsLimitSingleInstance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L601). **negative:** no negative test. **{single-polarity}:** the encoder appends exactly one capability.PathsLimit holding all families at internal/component/bgp/reactor/config_capabilities.go:388-391, so no code path can emit a second instance and a two-instance negative case cannot be constructed |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-2` | "The PATHS-LIMIT capability MUST be ignored if the ADD-PATH capability is not present" (§3) | MUST | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestNegotiatePathsLimit`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L589). **negative:** `unit/verify` [`TestNegotiatePathsLimitNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L644) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-3` | "An AFI/SAFI tuple MUST be ignored if the same tuple was not received in the ADD-PATH capability" (§3) | MUST | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestNegotiatePathsLimitPartialAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L670). **negative:** `unit/verify` [`TestNegotiatePathsLimitPartialAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L671) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-4` | When more than one tuple is received for the same AFI/SAFI pair, only the first tuple is considered and "All others MUST be ignored" (§3) | MUST | 3 - PATHS-LIMIT Capability | **positive:** `unit/verify` [`TestParsePathsLimitDuplicateFirstWins`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L866). **negative:** `unit/verify` [`TestParsePathsLimitDuplicateFirstWins`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L867) |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-5` | "If the received Paths Limit is zero (0), the tuple SHOULD be ignored" (§3) | SHOULD | 3 - PATHS-LIMIT Capability | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-6` | "A sender advertising multiple paths for the same prefix SHOULD send only the specified maximum number of paths indicated in the PATHS-LIMIT capability" (§3) | SHOULD | 3 - PATHS-LIMIT Capability | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7` | "An implementation SHOULD provide a configuration knob to specify the maximum number of paths to accept from a sender" (§3) | SHOULD | 3 - PATHS-LIMIT Capability | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-1`](#draft-abraitis-idr-addpath-paths-limit-3-1)

A BGP speaker wishing to indicate support for multiple AFI/SAFIs "MUST do so by including the information in a single instance of the PATHS-LIMIT capability" (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestParsePeerCapabilityPathsLimitSingleInstance`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L601) | unit/verify | unproven |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-2`](#draft-abraitis-idr-addpath-paths-limit-3-2)

"The PATHS-LIMIT capability MUST be ignored if the ADD-PATH capability is not present" (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegotiatePathsLimitNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L644) | unit/verify | unproven |
| positive | [`TestNegotiatePathsLimit`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L589) | unit/verify | unproven |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-3`](#draft-abraitis-idr-addpath-paths-limit-3-3)

"An AFI/SAFI tuple MUST be ignored if the same tuple was not received in the ADD-PATH capability" (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegotiatePathsLimitPartialAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L671) | unit/verify | unproven |
| positive | [`TestNegotiatePathsLimitPartialAddPath`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L670) | unit/verify | unproven |

### [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-4`](#draft-abraitis-idr-addpath-paths-limit-3-4)

When more than one tuple is received for the same AFI/SAFI pair, only the first tuple is considered and "All others MUST be ignored" (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParsePathsLimitDuplicateFirstWins`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L867) | unit/verify | unproven |
| positive | [`TestParsePathsLimitDuplicateFirstWins`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L866) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, draft-abraitis-idr-addpath-paths-limit |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/drafts/draft-abraitis-idr-addpath-paths-limit.txt |
| Source fingerprint | 87330146d8f7b5c0 |
| Record | rfc/extraction/draft-abraitis-idr-addpath-paths-limit.json |
| Mapped sentences | 4 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. The Abstract restates section 1 and directs no speaker. |
| `1` | Introduction | 0 | walked | Introduction. Indicative prose: ADD-PATH lets a speaker advertise several paths for one prefix, a large number of such paths can exhaust the receiver's memory, and this document defines a capability that tells the sender the receiver's ceiling. No sentence directs a speaker. |
| `2` | Specification of Requirements | 0 | walked | Specification of Requirements. The BCP 14 key-words paragraph, which binds no speaker and states how to read the other sections. The derivation excludes it from the site inventory for that reason. |
| `3` | PATHS-LIMIT Capability | 4 | walked | PATHS-LIMIT Capability. The document's only normative section: Capability Code 76, the 5-octet AFI/SAFI/Paths-Limit tuple, and four capitalised MUST-level sites mapped below to DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-1 through -3-4. Its three remaining directives are advisory and carry no site: ignore a tuple whose Paths Limit is zero, send no more than the advertised maximum, and offer a configuration knob. Those are the unsourced ids below. Two indicative sentences state no obligation: the Paths Limit field description, and "If the PATHS-LIMIT capability is empty (i.e. the Capability Length field is set to 0), it means that the sender doesn't have any specific limits to communicate", which defines what an empty capability means rather than directing a speaker. |
| `4` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records capability number 76 in the BGP Capability Codes registry. Binds IANA, not a speaker. |
| `5` | not stated | 0 | walked | Security Considerations, and with it the unnumbered Acknowledgements, References and Authors' Addresses blocks, which carry no section number and so stay in this body under the heading derivation. The security text states that PATHS-LIMIT mitigates some RFC 7911 concerns and that a rogue or misconfigured node can advertise a limit too low for the application. Its one direction, "Users of the PATHS-LIMIT Capability are encouraged to examine the behavior and potential impact", is an encouragement with no RFC 2119 keyword and binds no speaker. |
| `A` | Implementation Report | 0 | skipped (appendix-non-normative) | Implementation Report. An RFC 7942 note naming the FRRouting commit that implements the draft. It reports on another implementation and states no obligation. |

### Excluded sentences

The walk over DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT, so its obligations are stated where they were written.
