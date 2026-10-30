# RFC 4739 - Multiple Authentication Exchanges in the Internet Key Exchange (IKEv2) Protocol

Partial. Every requirement this repository extracted from RFC 4739, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 2 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 100.0% | 2 of 2 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 2 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 2 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 2 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 100.0% | 4 of 4 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 2 | of 2 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 2 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 2 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 2 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 2 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 2 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

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
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 2 |
| Gated MUST-level | 2 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tagged units | 4 |
| Recorded audit verdicts | 0 |
| Discrimination records | 4 |
| Summary | `rfc/short/rfc4739.md` |
| Requirement shard | `rfc/requirements/rfc4739.md` |
| RFC text | `rfc/full/rfc4739.txt` |

## Enrolment

Enrolled: Brought into scope by owner decision, 2026-10-30, ending the 2026-10-26 out-of-scope decision. Under the owner rule in ai/META-DECISIONS.md (Goal and scope), an optional RFC counts once Thomas brings it into scope, and docs/contributing/rfc-conformance-gates.md says bringing it into scope means moving its profile name from `Optional use cases` to `Use cases`. vpn-concentrator is therefore a required use case. The extraction is complete: the source text is rfc/full/rfc4739.txt and the sign-off is rfc/extraction/rfc4739.json. Both MUST-level requirements (the Protocol ID and SPI Size of the two notifications) are met by the one builder of each notification in `internal/component/ike/engine/multiple_auth.go` (multipleAuthNotify, anotherAuthFollowsNotify), proven by tagged tests in `internal/component/ike/engine/rfc4739_test.go` and `rfc4739_rounds_test.go`.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Site-to-site peers, both roles: MULTIPLE_AUTH_SUPPORTED announced in the IKE_SA_INIT response and first IKE_AUTH request for a peer with `authentication second-round`
- ANOTHER_AUTH_FOLLOWS sent beside AUTH and read
- one extra pre-shared-secret or x509 round per direction, each AUTH over that round's identity
- SA, TSi and TSr in the first request and final response only
- an unconfigured, missing or third round refused with AUTHENTICATION_FAILED ([`internal/component/ike/engine/multiple_auth.go`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/multiple_auth.go)).


**What the ledger says remains:**

EAP authentication rounds (Examples 1 and 2), remote-access peers and more than one extra round per direction, owned by [`plan/post-release/features/spec-ike-multiple-authentication-eap-rounds.md`](https://github.com/ze-software/ze/blob/main/plan/post-release/features/spec-ike-multiple-authentication-eap-rounds.md).

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

**Annotated (including scoped evidence) (2):** [`RFC4739-3.1-1`](#rfc4739-3.1-1), [`RFC4739-3.2-1`](#rfc4739-3.2-1)

## Attainable ceiling

Mandatory ceiling: 2 of 2 (100.0%). Optional ceiling: no rows. Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc4739 2 of 2 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4739-3.1-1` | The Protocol ID and SPI Size fields MUST be set to zero, and there is no data associated with this Notify type. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC4739InitiatorRequestCarriesBothNotifies`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4739_test.go#L161). **positive:** `unit/verify` [`TestRFC4739ResponderAnnouncesMultipleAuth`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4739_test.go#L118). **negative:** no negative test. **{single-polarity}:** generate-side shape -- MULTIPLE_AUTH_SUPPORTED is built only by engine/multiple_auth.go::multipleAuthNotify, which leaves NotificationData empty, and wire.PayloadNotify.WriteTo writes Protocol ID and SPI Size as zero whenever no SPI follows; the rule binds the sender, and Ze reads the notification without judging those fields as strongSwan does, so it has no reject path |
| `RFC4739-3.2-1` | The Protocol ID and SPI Size fields MUST be set to zero, and there is no data associated with this Notify type. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC4739InitiatorRequestCarriesBothNotifies`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4739_test.go#L170). **positive:** `unit/verify` [`TestRFC4739ResponderAnotherAuthFollows`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4739_rounds_test.go#L330). **negative:** no negative test. **{single-polarity}:** generate-side shape -- ANOTHER_AUTH_FOLLOWS is built only by engine/multiple_auth.go::anotherAuthFollowsNotify, which leaves NotificationData empty, and wire.PayloadNotify.WriteTo writes Protocol ID and SPI Size as zero whenever no SPI follows; the rule binds the sender, and Ze reads the notification without judging those fields as strongSwan does, so it has no reject path |

## Gaps and untested MUSTs

RFC 4739 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4739-3.1-1`](#rfc4739-3.1-1)

The Protocol ID and SPI Size fields MUST be set to zero, and there is no data associated with this Notify type. (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4739InitiatorRequestCarriesBothNotifies`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4739_test.go#L161) | unit/verify | mutant, verified |
| positive | [`TestRFC4739ResponderAnnouncesMultipleAuth`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4739_test.go#L118) | unit/verify | mutant, verified |

### [`RFC4739-3.2-1`](#rfc4739-3.2-1)

The Protocol ID and SPI Size fields MUST be set to zero, and there is no data associated with this Notify type. (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4739ResponderAnotherAuthFollows`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4739_rounds_test.go#L330) | unit/verify | mutant, verified |
| positive | [`TestRFC4739InitiatorRequestCarriesBothNotifies`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4739_test.go#L170) | unit/verify | mutant, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc4739.txt |
| Source fingerprint | 797349d9f7bf7b48 |
| Record | rfc/extraction/rfc4739.json |
| Mapped sentences | 2 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 0 | walked | not stated |
| `2.4` | not stated | 0 | walked | not stated |
| `2.5` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 1 | walked | not stated |
| `3.2` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 4739 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 4739, so its obligations are stated where they were written.
