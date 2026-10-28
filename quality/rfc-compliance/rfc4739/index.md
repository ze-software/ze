# RFC 4739 - Multiple Authentication Exchanges in the Internet Key Exchange (IKEv2) Protocol

Future. Every requirement this repository extracted from RFC 4739, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

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
| MUSTs declared | 2 | of 2 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (out-of-scope), so every share below reads what the summary records rather than what the gate enforces |
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
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
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
| Public status | Future |
| Enrolment | Not enrolled (out-of-scope) |
| Requirements | 2 |
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
| Summary | `rfc/short/rfc4739.md` |
| Requirement shard | `rfc/requirements/rfc4739.md` |
| RFC text | `rfc/full/rfc4739.txt` |

## Enrolment

Not enrolled (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OUT OF SCOPE as a document by owner decision, 2026-10-26: the owner rule committed in 1e7a9dbc34 (ai/META-DECISIONS.md, Goal and scope) counts an RFC a profile lists as optional only once Thomas brings it into scope, and this summary's `Optional use cases` row lists this RFC in the optional tier of the vpn-concentrator profile. The extraction is COMPLETE: the source text is at rfc/full/rfc4739.txt and the extraction sign-off is rfc/extraction/rfc4739.json. Ze implements none of it, and each MUST-level row keeps its `{gap}` note naming the absent producer: both MUST-level requirements (the Protocol ID and SPI Size of the two notifications) are classified `{gap}`, because Ze's IKE engine implements neither MULTIPLE_AUTH_SUPPORTED (16404) nor ANOTHER_AUTH_FOLLOWS (16405). `internal/component/ike/wire/payload_notify.go` declares no such Notify Message Type and no engine path sends or reads one.

## What the public ledger says

**Status:** Future

**What the ledger says is covered:**

None: each IKE_AUTH authenticates one identity per peer, by PSK, a certificate signature or one EAP method (`internal/component/ike/engine/auth.go::verifyRemoteAuth`).

**What the ledger says remains**

MULTIPLE_AUTH_SUPPORTED and ANOTHER_AUTH_FOLLOWS are neither sent nor recognized, and no second IDi or IDr authentication round follows a successful AUTH. Both MUST-level requirements are annotated `{gap}` in [`rfc/short/rfc4739.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc4739.md). Out of scope by owner decision, 2026-10-26.

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
| `RFC4739-3.1-1` | The Protocol ID and SPI Size fields MUST be set to zero, and there is no data associated with this Notify type. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze never builds or parses MULTIPLE_AUTH_SUPPORTED: internal/component/ike/wire/payload_notify.go declares no Notify Message Type 16404 and no engine path emits it; internal/component/ike/wire/payload_notify.go |
| `RFC4739-3.2-1` | The Protocol ID and SPI Size fields MUST be set to zero, and there is no data associated with this Notify type. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze never builds or parses ANOTHER_AUTH_FOLLOWS: internal/component/ike/wire/payload_notify.go declares no Notify Message Type 16405 and no engine path emits it; internal/component/ike/wire/payload_notify.go |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4739-3.1-1`](#rfc4739-3.1-1) The Protocol ID and SPI Size fields MUST be set to zero, and there is no data associated with this Notify type. (§3.1) | {gap}, no test | Ze never builds or parses MULTIPLE_AUTH_SUPPORTED: internal/component/ike/wire/payload_notify.go declares no Notify Message Type 16404 and no engine path emits it; internal/component/ike/wire/payload_notify.go |
| [`RFC4739-3.2-1`](#rfc4739-3.2-1) The Protocol ID and SPI Size fields MUST be set to zero, and there is no data associated with this Notify type. (§3.2) | {gap}, no test | Ze never builds or parses ANOTHER_AUTH_FOLLOWS: internal/component/ike/wire/payload_notify.go declares no Notify Message Type 16405 and no engine path emits it; internal/component/ike/wire/payload_notify.go |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4739-3.1-1`](#rfc4739-3.1-1)

The Protocol ID and SPI Size fields MUST be set to zero, and there is no data associated with this Notify type. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4739-3.1-1, so no unit is bound to it.

### [`RFC4739-3.2-1`](#rfc4739-3.2-1)

The Protocol ID and SPI Size fields MUST be set to zero, and there is no data associated with this Notify type. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4739-3.2-1, so no unit is bound to it.

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
