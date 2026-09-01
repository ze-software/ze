# RFC 6286 - Autonomous-System-Wide Unique BGP Identifier for BGP-4

Supported. Every requirement this repository extracted from RFC 6286, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 75.0% | 3 of 4 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 25.0% | 1 of 4 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 4 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 12 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 5 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
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
| Requirements | 5 |
| Gated MUST-level | 4 |
| Obligations that bind Ze | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 12 |
| Tagged units | 12 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc6286.md` |
| Requirement shard | `rfc/requirements/rfc6286.md` |
| RFC text | `rfc/full/rfc6286.txt` |

## Enrolment

Enrolled: Autonomous-System-Wide Unique BGP Identifier for BGP-4: three short protocol revisions to RFC 4271 that ze now implements end to end -- the Section 2.2 reception check on both OPEN rails, the Section 2.1 AS-wide identifier claim (and its operator opt-out), and the Section 2.3 collision tie-break.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

- All three protocol revisions. Section 2.1: the local identifier is rejected when zero, on the global leaf and on a per-peer override (`parseRouterID`, [`internal/component/bgp/reactor/config.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config.go)), and AS-wide uniqueness is enforced by default through a claim taken during OPEN validation and released on session teardown ([`internal/component/bgp/reactor/routerid_unique.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/routerid_unique.go)), with `bgp/session/allow-shared-router-id` as the operator opt-out the lowercase "should" permits. Section 2.2: a received OPEN whose BGP Identifier is zero, or equals ze's own identifier and comes from an internal peer, is answered with OPEN Message Error / Bad BGP Identifier on BOTH receive rails -- normal receive and the connection that won collision resolution ([`internal/component/bgp/reactor/session_open_validation.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_validation.go), called from session_handlers.go and session_connection.go)
- the same identifier from an external peer is accepted, as Section 2.2 requires. The LOSING connection of a collision is closed before Section 2.2 runs, so it receives Cease / Connection Collision (6/7) rather than Bad BGP Identifier even when its identifier is zero (`rejectConnectionCollision`, [`internal/component/bgp/reactor/reactor_connection.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_connection.go)). The connection closes either way and RFC 4271 Section 6.8 does not fix the ordering, but the subcode differs and this row does not claim otherwise. Section 2.3: identical identifiers in a connection collision preserve the connection initiated by the larger AS number ([`internal/component/bgp/reactor/session.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session.go) `DetectCollision`). Requirements bound per line in [`rfc/short/rfc6286.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc6286.md).


**What the ledger says remains:**

No tracked gap in current source anchors. Section 2.3 applies only where RFC 4271 Section 6.8 collision detection already applies (OpenConfirm): ze does not implement the Section 6.8 OpenSent MAY, tracked under the RFC 4271 row.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC6286-2.2-1`](#rfc6286-2.2-1), [`RFC6286-2.2-2`](#rfc6286-2.2-2), [`RFC6286-2.3-1`](#rfc6286-2.3-1)

**Annotated instead of tested (1):** [`RFC6286-2.1-1`](#rfc6286-2.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6286-2.1-1` | The BGP Identifier is a 4-octet, unsigned, NON-ZERO integer whose value is determined on startup and is the same for every local interface and every BGP peer (Section 2.1) | MUST | 2.1 - Definition of the BGP Identifier | **positive:** `unit/verify` [`TestParsePeerFromTreeInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L126). **negative:** no negative test. **{single-polarity}:** the definition's other two properties are structural and have no failure mode a negative test could exercise -- the wire field is a uint32 (internal/component/bgp/message/open.go:55), and the value is read once at config load into reactor.Config.RouterID and used for every peer and every OPEN (internal/component/bgp/reactor/session_negotiate.go:160). The non-zero half IS enforced and tested: parseRouterID (internal/component/bgp/reactor/config.go) rejects 0.0.0.0 for both the global leaf and a per-peer override |
| `RFC6286-2.1-2` | The BGP Identifier should be unique within an AS (Section 2.1) | SHOULD | 2.1 - Definition of the BGP Identifier | **positive:** `unit/verify` [`TestRouterIDClaimConcurrentOnlyOneWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/routerid_unique_test.go#L453). **negative:** no negative test. **{single-polarity}:** the negative case -- ze accepting a duplicate -- is the operator-selected `bgp/session/allow-shared-router-id true` path, which is conformant precisely because the requirement is a lowercase "should", so there is no violation for a negative test to catch. Enforcement is proven by TestRouterIDClaimConcurrentOnlyOneWins and the TestRouterIDConflict* family; the opt-out by TestValidateOpenAllowSharedRouterID |
| `RFC6286-2.2-1` | An OPEN whose BGP Identifier field is zero is rejected with Error Subcode "Bad BGP Identifier" (Section 2.2) | MUST | 2.2 - Open Message Error Handling | **positive:** `unit/verify` [`TestHandleOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_validation_test.go#L95). **positive:** `unit/verify` [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L445). **positive:** `unit/verify` [`TestProcessOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_validation_test.go#L153). **negative:** `unit/verify` [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L447) |
| `RFC6286-2.2-2` | An OPEN whose BGP Identifier equals the local BGP Identifier AND comes from an internal peer is rejected with Error Subcode "Bad BGP Identifier"; the same identifier from an EXTERNAL peer is not rejected (Section 2.2) | MUST | 2.2 - Open Message Error Handling | **positive:** `unit/verify` [`TestHandleOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_validation_test.go#L97). **positive:** `unit/verify` [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L449). **negative:** `unit/verify` [`TestHandleOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_validation_test.go#L99). **negative:** `unit/verify` [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L451) |
| `RFC6286-2.3-1` | When the BGP Identifiers involved in a connection collision are identical, the connection initiated by the BGP speaker with the larger AS number is preserved (Section 2.3) | MUST | 2.3 - Connection Collision Resolution | **positive:** `unit/verify` [`TestDetectCollisionEqualIdentifierPrefersLargerAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L607). **negative:** `unit/verify` [`TestDetectCollisionEqualIdentifierPrefersLargerAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L611) |

## Gaps and untested MUSTs

RFC 6286 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6286-2.1-1`](#rfc6286-2.1-1)

The BGP Identifier is a 4-octet, unsigned, NON-ZERO integer whose value is determined on startup and is the same for every local interface and every BGP peer (Section 2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestParsePeerFromTreeInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L126) | unit/verify | unproven |

### [`RFC6286-2.1-2`](#rfc6286-2.1-2)

The BGP Identifier should be unique within an AS (Section 2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRouterIDClaimConcurrentOnlyOneWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/routerid_unique_test.go#L453) | unit/verify | unproven |

### [`RFC6286-2.2-1`](#rfc6286-2.2-1)

An OPEN whose BGP Identifier field is zero is rejected with Error Subcode "Bad BGP Identifier" (Section 2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L447) | unit/verify | unproven |
| positive | [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L445) | unit/verify | unproven |
| positive | [`TestHandleOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_validation_test.go#L95) | unit/verify | unproven |
| positive | [`TestProcessOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_validation_test.go#L153) | unit/verify | unproven |

### [`RFC6286-2.2-2`](#rfc6286-2.2-2)

An OPEN whose BGP Identifier equals the local BGP Identifier AND comes from an internal peer is rejected with Error Subcode "Bad BGP Identifier"; the same identifier from an EXTERNAL peer is not rejected (Section 2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L451) | unit/verify | unproven |
| negative | [`TestHandleOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_validation_test.go#L99) | unit/verify | unproven |
| positive | [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L449) | unit/verify | unproven |
| positive | [`TestHandleOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_validation_test.go#L97) | unit/verify | unproven |

### [`RFC6286-2.3-1`](#rfc6286-2.3-1)

When the BGP Identifiers involved in a connection collision are identical, the connection initiated by the BGP speaker with the larger AS number is preserved (Section 2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDetectCollisionEqualIdentifierPrefersLargerAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L611) | unit/verify | unproven |
| positive | [`TestDetectCollisionEqualIdentifierPrefersLargerAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L607) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, rfc6286 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc6286.txt |
| Source fingerprint | c09b7508ad62e214 |
| Record | rfc/extraction/rfc6286.json |
| Mapped sentences | 0 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 2 | walked | Title block, Abstract, Status of This Memo and Copyright Notice. Walked rather than skipped because the site scan attributes two sites here, both excluded below: the Abstract's restatement of what the document does to RFC 4271, and the IETF Trust Legal Provisions boilerplate. Nothing before section 1 binds a BGP speaker. |
| `1` | Introduction | 1 | walked | Introduction. Reports that RFC 4271 specifies the BGP Identifier as a valid IPv4 host address assigned to the speaker, that deployed code requires two speakers to carry distinct Identifiers, and that this document relaxes both so an IPv6-only network can comply. Its one site, 1:1, is that report and is excluded below. The relaxed definition itself is stated in section 2.1. |
| `2` | Protocol Revisions, the parent heading | 0 | walked | Protocol Revisions, the parent heading. One sentence naming what the revisions cover: the definition of the BGP Identifier and the procedures for a speaker that supports the AS-wide Unique BGP Identifier. It directs nobody; 2.1, 2.2 and 2.3 carry the obligations. |
| `2.1` | Definition of the BGP Identifier | 0 | walked | Definition of the BGP Identifier. The obligation is the indented replacement text: 'The BGP Identifier is a 4-octet, unsigned, non-zero integer that should be unique within an AS. The value of the BGP Identifier for a BGP speaker is determined on startup and is the same for every local interface and every BGP peer.' It is written in the indicative with a lowercase 'should', so neither the capitalised scan nor the modal scan attributes a site to this section, and both ids read from it are declared unsourced here. RFC6286-2.1-1 is the 4-octet, unsigned, NON-ZERO definition, whose non-zero half ze enforces on reception through Open.ValidateBGPIdentifier and refuses to originate through parseRouterID (internal/component/bgp/reactor/config.go). RFC6286-2.1-2 is the lowercase 'should' of AS-wide uniqueness, enforced by default through the claim registry in internal/component/bgp/reactor/routerid_unique.go, which keys routerIDKey on (peer AS, identifier), with bgp/session/allow-shared-router-id as the operator opt-out the advisory level permits. |
| `2.2` | Open Message Error Handling | 0 | walked | Open Message Error Handling. The obligation is the indented replacement text for RFC 4271 section 6.2: 'If the BGP Identifier field of the OPEN message is zero, or if it is the same as the BGP Identifier of the local BGP speaker and the message is from an internal peer, then the Error Subcode is set to "Bad BGP Identifier".' Written in the indicative with no modal at all, so no scan attributes a site to this section. It states two rejection conditions and the second is gated on the peer being internal, which is why the summary declares two ids and both are declared unsourced here. Session.validateOpenIdentifier (internal/component/bgp/reactor/session_open_validation.go) is the producer for both, on the handleOpen and processOpen rails. |
| `2.3` | Connection Collision Resolution | 0 | walked | Connection Collision Resolution. The obligation is the indented extension to RFC 4271 section 6.8: 'If the BGP Identifiers of the peers involved in the connection collision are identical, then the connection initiated by the BGP speaker with the larger AS number is preserved.' Indicative, no modal, no site. Declared unsourced here as RFC6286-2.3-1. The lead-in scopes it to an external peer, because an internal peer presenting this speaker's identifier is already rejected by 2.2, and the closing sentence says the extension covers 4-octet AS numbers [RFC4893]. Both facts qualify the same single obligation and neither adds one. |
| `3` | Remarks | 0 | walked | Remarks. Five indicative observations, none of which directs a speaker: an Identifier allocated per RFC 4271 already fits the revised definition; a confederation counts as one AS for the purpose of AS-wide uniqueness; a speaker supporting this cannot share an Identifier with an external neighbor until that neighbor is upgraded, which is a consequence of the un-upgraded neighbor still applying RFC 4271's stricter test; and the Identifier is also used in AGGREGATOR, in the route-reflection attributes and in route selection. The section's own closing sentence states its purpose: to conclude that the revisions introduce no backward compatibility issue. The confederation observation scopes the lowercase 'should' of 2.1 and adds no obligation of its own, so it earns no id; rfc/short/rfc6286.md carries all five observations under Remarks. |
| `4` | Security Considerations | 0 | walked | Security Considerations. Two sentences: this extension introduces no new security considerations, and BGP's are discussed in RFC 4271. No countermeasure is directed at a speaker. |
| `5` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments. |
| `6` | Normative References: RFC 4271 and RFC 4893 | 0 | skipped (references) | Normative References: RFC 4271 and RFC 4893. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The Abstract, non-normative by RFC convention and word-for-word the second paragraph of section 1. Its lowercase 'required' is inside a report of the document's own editorial effect on RFC 4271 -- it 'relaxes the "uniqueness" requirement so that only Autonomous-System-wide (AS-wide) uniqueness of the BGP Identifiers is required' -- which states what scope the revised requirement has, not an obligation on a speaker. The obligation itself is the indented replacement text of section 2.1, invisible to both scans and declared as unsourced RFC6286-2.1-1 and RFC6286-2.1-2 there. | To accommodate situations where the current requirements for the BGP Identifier are not met, this document relaxes the definition of the BGP Identifier to be a 4-octet, unsigned, non-zero integer and relaxes the "uniqueness" requirement so that only Autonomous-System- wide (AS-wide) uniqueness of the BGP Identifiers is required. |
| `front:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate the extractor did not strip: the IETF Trust Legal Provisions paragraph of the Copyright Notice. Its lowercase 'must' binds a party extracting Code Components from the document text, and directs no protocol behavior at a BGP speaker. RFC 6286 contains no code component: it revises three prose paragraphs of RFC 4271 and defines no syntax. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The Introduction's second paragraph, the same report as front:1 with the IPv6-only network named as the motivating case. It says what this document does to RFC 4271's text, so its lowercase 'required' describes the new scope of the uniqueness requirement rather than imposing one. Classified not-a-requirement rather than duplicate-of because a duplicate must name an id another site MAPS, and no site of this document maps anything: every obligation is stated in the indicative and is declared unsourced on 2.1, 2.2 and 2.3. | To accommodate situations where the current requirements for the BGP Identifier are not met (such as in the case of an IPv6-only network), this document relaxes the definition of the BGP Identifier to be a 4-octet, unsigned, non-zero integer and relaxes the "uniqueness" requirement so that only AS-wide uniqueness of the BGP Identifiers is required. |

## Superseded

No document obsoletes RFC 6286, so its obligations are stated where they were written.
