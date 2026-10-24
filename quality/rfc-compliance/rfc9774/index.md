# RFC 9774 - Deprecation of AS_SET and AS_CONFED_SET in BGP

Supported. Every requirement this repository extracted from RFC 9774, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 2 of 2 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 2 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 2 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 2 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 2 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 66.7% | 4 of 6 tagged units, 0 escaped and 2 lapsed | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 2 | of 2 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 2 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 2 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 2 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 2 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 2 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 2 |
| Gated MUST-level | 2 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 6 |
| Tagged units | 6 |
| Recorded audit verdicts | 0 |
| Discrimination records | 6 |
| Summary | `rfc/short/rfc9774.md` |
| Requirement shard | `rfc/requirements/rfc9774.md` |
| RFC text | `rfc/full/rfc9774.txt` |

## Enrolment

Enrolled: Owner decision 2026-10-18, "Implement RFC 9774", lifting the 2026-10-08 enrolment freeze for this RFC only. Two MUST-level requirements, both in Section 3, both met and proven in both polarities. RFC9774-3-2 (treat-as-withdraw on receipt of an AS_SET or AS_CONFED_SET in AS_PATH or AS4_PATH) is enforced by validateASPath (internal/component/bgp/message/rfc7606.go) and validateAS4PathAttr (internal/component/bgp/message/rfc7607.go), which share ASPathSetSegment (internal/component/bgp/message/rfc9774.go). RFC9774-3-1 (never advertise an UPDATE containing either set) is enforced by Session.transmittedPathHoldsSet and Session.writeUpdateAsWithdrawal (internal/component/bgp/reactor/session_rfc9774.go), called from Session.writeUpdateBody, the one UPDATE writer. The two MUST-level sites of Sections 5.2 and 6.1 bind route aggregation, which Ze does not implement, and are excluded as feature-out-of-scope in rfc/extraction/rfc9774.json. Enrolled 2026-10-19.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

[`internal/component/bgp/message/rfc9774.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9774.go) (`ASPathSetSegment`), [`internal/component/bgp/message/rfc7606.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606.go) (`validateASPath`), [`internal/component/bgp/message/rfc7607.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607.go) (`validateAS4PathAttr`), [`internal/component/bgp/reactor/session_rfc9774.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_rfc9774.go) (`transmittedPathHoldsSet`, `writeUpdateAsWithdrawal`), [`internal/component/bgp/reactor/session_write.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_write.go) (`writeUpdateBody`)

**What the ledger says remains**

Both Section 3 MUST-level requirements implemented, each with a positive and a negative test tag.

- **One operator override, on receive only:** the peer leaf `session as-set accept`, set by the operator (and written by `ze exabgp migrate` for an ExaBGP `as-set accept` neighbor), keeps a received set (see "Owner-approved deviation"; not counted as conformant). The `send bgp <peer> raw` test instrument writes operator bytes as given. Route aggregation (Sections 5, 5.2, 6.1, 6.2) is not implemented, so its obligations are an implementation gap outside the conformance count.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **2** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC9774-3-1`](#rfc9774-3-1), [`RFC9774-3-2`](#rfc9774-3-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9774-3-1` | MUST NOT advertise BGP UPDATE messages containing AS_SETs or AS_CONFED_SETs (§3) | MUST NOT | 3 | **positive:** `unit/verify` [`TestRFC9774SendASSetWrittenAsWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_send_test.go#L49). **negative:** `unit/verify` [`TestRFC9774SendASSetWrittenAsWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_send_test.go#L52) |
| `RFC9774-3-2` | MUST use the "treat-as-withdraw" error handling behavior per [RFC7606] upon reception of BGP UPDATE messages containing AS_SETs or AS_CONFED_SETs in the AS_PATH or AS4_PATH [RFC6793] (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC9774ReceiveASSetTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_receive_test.go#L64). **positive:** `unit/verify` [`TestValidateASPathRejectsSetSegments`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9774_test.go#L35). **negative:** `unit/verify` [`TestRFC9774ReceiveASSetTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_receive_test.go#L67). **negative:** `unit/verify` [`TestValidateASPathRejectsSetSegments`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9774_test.go#L38) |

## Gaps and untested MUSTs

RFC 9774 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9774-3-1`](#rfc9774-3-1)

MUST NOT advertise BGP UPDATE messages containing AS_SETs or AS_CONFED_SETs (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9774SendASSetWrittenAsWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_send_test.go#L52) | unit/verify | revert, verified |
| positive | [`TestRFC9774SendASSetWrittenAsWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_send_test.go#L49) | unit/verify | revert, verified |

### [`RFC9774-3-2`](#rfc9774-3-2)

MUST use the "treat-as-withdraw" error handling behavior per [RFC7606] upon reception of BGP UPDATE messages containing AS_SETs or AS_CONFED_SETs in the AS_PATH or AS4_PATH [RFC6793] (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateASPathRejectsSetSegments`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9774_test.go#L38) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| negative | [`TestRFC9774ReceiveASSetTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_receive_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestValidateASPathRejectsSetSegments`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9774_test.go#L35) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestRFC9774ReceiveASSetTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_receive_test.go#L64) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-implement agent, spec-bgp-rfc9774-as-set-deprecation |
| Signed off | 2026-10-19 |
| Register | rfc2119 |
| Source | rfc/full/rfc9774.txt |
| Source fingerprint | 1c7ab2a8ddf3a013 |
| Record | rfc/extraction/rfc9774.json |
| Mapped sentences | 2 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of This Memo, copyright notice, Abstract and Table of Contents. The Abstract states scope and binds nobody. |
| `1` | Introduction | 0 | walked | Introduction. Explains why AS_SET and AS_CONFED_SET complicate BGP security and states the document updates RFC 4271 and RFC 5065 and obsoletes RFC 6472. No RFC 2119 keyword; the obligations are stated in section 3. |
| `2` | Requirements Language boilerplate citing BCP 14 | 0 | walked | Requirements Language boilerplate citing BCP 14. It defines keywords and obliges nobody. |
| `3` | not stated | 2 | walked | The two obligations of the document, both mapped: 3:1 to RFC9774-3-1 (send) and 3:2 to RFC9774-3-2 (receive). The closing paragraph records which sections of RFC 4271 and RFC 5065 are updated and binds nothing further. |
| `4` | not stated | 0 | walked | Describes how RPKI-ROV, BGPsec and ASPA already treat a route with an AS_SET. Descriptive, no RFC 2119 keyword, and it obliges no BGP speaker beyond the documents it cites. |
| `5` | Describes brief aggregation | 0 | walked | Describes brief aggregation. Its one SHOULD is a quotation of RFC 4271 Section 5.1.6 and belongs to RFC 4271; the section adds no obligation of its own. |
| `5.1` | not stated | 0 | walked | Explains why brief aggregation yields an unpredictable origin AS for RPKI-ROV. Descriptive, no RFC 2119 keyword. |
| `5.2` | Consistent brief aggregation | 1 | walked | Consistent brief aggregation. Its MUST (5.2:1) and its SHOULD to attach ATOMIC_AGGREGATE bind an aggregating implementation; Ze implements no aggregation, so 5.2:1 is excluded as feature-out-of-scope. |
| `6` | not stated | 0 | walked | Section preamble: operator advice on deployment and configuration. No keyword. |
| `6.1` | not stated | 1 | walked | Its MUST (6.1:1) binds an operator that aggregates; excluded as feature-out-of-scope with 5.2:1. |
| `6.2` | not stated | 0 | walked | SHOULD NOT and SHOULD on announcing an aggregate to contributing ASes; recommendations conditional on aggregation, which Ze does not implement. No MUST-level site. |
| `6.3` | not stated | 0 | walked | Reminds that RFC 4632 Section 5.1 Rule 2 binds aggregating implementations; it restates another document and adds no keyword. |
| `7` | Security Considerations | 0 | walked | Security Considerations. Notes the deprecation shrinks the attack surface. No keyword. |
| `8` | not stated | 0 | skipped (iana) | IANA Considerations: marks segment types 1 and 4 deprecated in the registry. A registry action, no speaker obligation. |
| `9` | References heading | 0 | skipped (references) | References heading. |
| `9.1` | Normative references | 0 | skipped (references) | Normative references. |
| `9.2` | Informative references | 0 | skipped (references) | Informative references. |
| `A` | not stated | 0 | skipped (appendix-non-normative) | Example filtering policy for not advertising an aggregate to contributing ASes. Illustrative, no keyword. |
| `B` | not stated | 0 | skipped (appendix-non-normative) | Examples of consistent brief aggregation and the ROAs they need. Illustrative, no keyword. |
| `B.1` | Worked example of Appendix B | 0 | skipped (appendix-non-normative) | Worked example of Appendix B. Illustrative. |
| `B.2` | Worked example of Appendix B | 0 | skipped (appendix-non-normative) | Worked example of Appendix B. Illustrative. |
| `B.3` | Worked example of Appendix B | 0 | skipped (appendix-non-normative) | Worked example of Appendix B. Illustrative. |
| `B.4` | Worked example of Appendix B | 0 | skipped (appendix-non-normative) | Worked example of Appendix B. Illustrative. |
| `C` | Discussion of forwarding loops and AS_SETs | 0 | skipped (appendix-non-normative) | Discussion of forwarding loops and AS_SETs. Explanatory, no keyword. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `5.2:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Binds implementations of brief BGP aggregation only. RFC 4271 Section 9.1 makes route aggregation optional: "Route aggregation and information reduction can optionally be performed within this phase." Owner decision 2026-10-18 scoped this work to RFC 9774 and not to an aggregation feature (spec-bgp-rfc9774-as-set-deprecation, Key Design Decisions). Ze has no aggregation code: the AS_PATH parsers (ParseASPathText, internal/core/bgp/attribute/text.go) build only AS_SEQUENCE. | To ensure a consistent BGP origin AS is announced for aggregate BGP routes for implementations of "brief" BGP aggregation, the implementation MUST be configured to truncate the AS_PATH after the right-most instance of the desired origin AS for the aggregate. |
| `6.1:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Binds an operator "When aggregating prefixes", which needs an aggregation feature in the speaker. RFC 4271 Section 9.1 makes route aggregation optional: "Route aggregation and information reduction can optionally be performed within this phase." Owner decision 2026-10-18 scoped this work to RFC 9774 and not to an aggregation feature (spec-bgp-rfc9774-as-set-deprecation, Key Design Decisions). Ze has no aggregation code: the AS_PATH parsers (ParseASPathText, internal/core/bgp/attribute/text.go) build only AS_SEQUENCE. | When aggregating prefixes, network operators MUST use consistent brief aggregation as described in Section 5.2. |

## Superseded

No document obsoletes RFC 9774, so its obligations are stated where they were written.
