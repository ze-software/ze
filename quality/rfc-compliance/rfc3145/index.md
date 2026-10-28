# RFC 3145 - L2TP Disconnect Cause Information

Future. Every requirement this repository extracted from RFC 3145, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 8 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 8 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 8 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 8 | of 18 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (out-of-scope), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 8 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 8 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 8 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 8 of 8 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 8 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Future |
| Enrolment | Not enrolled (out-of-scope) |
| Requirements | 18 |
| Gated MUST-level | 8 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 8 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc3145.md` |
| Requirement shard | `rfc/requirements/rfc3145.md` |
| RFC text | `rfc/full/rfc3145.txt` |

## Enrolment

Not enrolled (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OUT OF SCOPE as a document by owner decision, 2026-10-26: the owner rule committed in 1e7a9dbc34 (ai/META-DECISIONS.md, Goal and scope) counts an RFC a profile lists as optional only once Thomas brings it into scope, and this summary's `Optional use cases` row lists this RFC in the optional tier of the lns profile. The extraction is COMPLETE: the source text is at rfc/full/rfc3145.txt, every capitalised MUST-level sentence is a row of this summary or an exclusion in rfc/extraction/rfc3145.json (walked 2026-10-26), and none is gated. Ze's L2TP stack defines no AVP 46 (internal/component/l2tp/avp.go) and sends none in a Call-Disconnect-Notify.

## What the public ledger says

**Status:** Future

**What the ledger says is covered:**

None. A received PPP Disconnect Cause Code AVP carries no M bit, so it is skipped as an unknown AVP under RFC 2661 Section 4.1.

**What the ledger says remains:**

Sending AVP 46 with the PPP disconnect cause in a Call-Disconnect-Notify, and logging a received one. Out of scope by owner decision, 2026-10-26.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 8 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**No test and no annotation (8):** [`RFC3145-2-1`](#rfc3145-2-1), [`RFC3145-2-2`](#rfc3145-2-2), [`RFC3145-2-3`](#rfc3145-2-3), [`RFC3145-2-4`](#rfc3145-2-4), [`RFC3145-2-5`](#rfc3145-2-5), [`RFC3145-3.1-1`](#rfc3145-3.1-1), [`RFC3145-3.2-1`](#rfc3145-3.2-1), [`RFC3145-3.3-1`](#rfc3145-3.3-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3145-2-1` | The AVP is valid in the L2TP Call-Disconnect-Notify (CDN) message only, and it MUST NOT be marked Mandatory. (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3145-2-2` | The length of the Value field MUST be at least 11 octets. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3145-2-3` | If the length is more than 11 octets, the additional octets MUST contain a descriptive text in UTF-8 [3] format that can be displayed to the user or in a log file. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3145-2-4` | Mandatory (M) bit: MUST be 0. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3145-2-5` | The length MUST be at least 11. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3145-3.1-1` | The Control Protocol Number for these errors thus MUST be set to 0. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3145-3.2-1` | The Control Protocol Number for these errors MUST be set to C021 hexadecimal (LCP). (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3145-3.3-1` | The Control Protocol Number for such errors MUST correspond to the PPP Control Protocol number for the authentication protocol in use. (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3145-2-7` | The valid values of this field are: 0: global error 1: at peer 2: at local 3-255: Reserved This field SHOULD be 0 unless documented otherwise in the description of the specific Disconnect Code. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3145-3-1` | Unless documented otherwise for a specific Disconnect Code, the Direction value SHOULD be 0. (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3145-3.4-1` | The Control Protocol Number for such errors SHOULD correspond to the PPP Network Control Protocol number in use. (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3145-3.4-3` | Otherwise, if only one copy of the AVP is given, the Control Protocol Number SHOULD correspond to the last (most recent) failing NCP. 17 FSM Timeout error. 18 No NCPs available (all disabled or rejected); no NCPs went to Opened state. (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3145-4-2` | Implementations MAY accept AVPs with these values as equivalent to the message described in this document, but SHOULD NOT transmit an AVP using these values. (§4) | SHOULD NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3145-1-1` | This document provides an additional Attribute Value Pair (AVP), called PPP Disconnect Cause Code, that MAY be used by an L2TP host to provide PPP-specific disconnect information to its peer. (§1) | MAY | 1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3145-2-6` | Hidden (H) bit: MAY be 1 if the attribute is hidden. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3145-3.4-2` | Where multiple network protocols are in use, multiple copies of this AVP MAY be given to indicate failure reasons for each NCP. (§3.4) | MAY | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3145-4-1` | This AVP MAY may be sent by either the LNS or LAC. (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3145-5-1` | It is intended for logging and diagnostic purposes in the event of PPP link failure and should not pose a threat to system security, but the AVP MAY be hidden as described in section 4.3 of RFC 2661. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3145-2-1`](#rfc3145-2-1) The AVP is valid in the L2TP Call-Disconnect-Notify (CDN) message only, and it MUST NOT be marked Mandatory. (§2) | no test | no test carries this requirement id |
| [`RFC3145-2-2`](#rfc3145-2-2) The length of the Value field MUST be at least 11 octets. (§2) | no test | no test carries this requirement id |
| [`RFC3145-2-3`](#rfc3145-2-3) If the length is more than 11 octets, the additional octets MUST contain a descriptive text in UTF-8 [3] format that can be displayed to the user or in a log file. (§2) | no test | no test carries this requirement id |
| [`RFC3145-2-4`](#rfc3145-2-4) Mandatory (M) bit: MUST be 0. (§2) | no test | no test carries this requirement id |
| [`RFC3145-2-5`](#rfc3145-2-5) The length MUST be at least 11. (§2) | no test | no test carries this requirement id |
| [`RFC3145-3.1-1`](#rfc3145-3.1-1) The Control Protocol Number for these errors thus MUST be set to 0. (§3.1) | no test | no test carries this requirement id |
| [`RFC3145-3.2-1`](#rfc3145-3.2-1) The Control Protocol Number for these errors MUST be set to C021 hexadecimal (LCP). (§3.2) | no test | no test carries this requirement id |
| [`RFC3145-3.3-1`](#rfc3145-3.3-1) The Control Protocol Number for such errors MUST correspond to the PPP Control Protocol number for the authentication protocol in use. (§3.3) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3145-2-1`](#rfc3145-2-1)

The AVP is valid in the L2TP Call-Disconnect-Notify (CDN) message only, and it MUST NOT be marked Mandatory. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3145-2-1, so no unit is bound to it.

### [`RFC3145-2-2`](#rfc3145-2-2)

The length of the Value field MUST be at least 11 octets. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3145-2-2, so no unit is bound to it.

### [`RFC3145-2-3`](#rfc3145-2-3)

If the length is more than 11 octets, the additional octets MUST contain a descriptive text in UTF-8 [3] format that can be displayed to the user or in a log file. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3145-2-3, so no unit is bound to it.

### [`RFC3145-2-4`](#rfc3145-2-4)

Mandatory (M) bit: MUST be 0. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3145-2-4, so no unit is bound to it.

### [`RFC3145-2-5`](#rfc3145-2-5)

The length MUST be at least 11. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3145-2-5, so no unit is bound to it.

### [`RFC3145-3.1-1`](#rfc3145-3.1-1)

The Control Protocol Number for these errors thus MUST be set to 0. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3145-3.1-1, so no unit is bound to it.

### [`RFC3145-3.2-1`](#rfc3145-3.2-1)

The Control Protocol Number for these errors MUST be set to C021 hexadecimal (LCP). (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3145-3.2-1, so no unit is bound to it.

### [`RFC3145-3.3-1`](#rfc3145-3.3-1)

The Control Protocol Number for such errors MUST correspond to the PPP Control Protocol number for the authentication protocol in use. (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3145-3.3-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc3145.txt |
| Source fingerprint | cc88e1c3c0d6320f |
| Record | rfc/extraction/rfc3145.json |
| Mapped sentences | 8 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2` | not stated | 5 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.1` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.2` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.3` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.4` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `5` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `8` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `9` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `9.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `9.2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `10` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `10.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |

### Excluded sentences

The walk over RFC 3145 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 3145, so its obligations are stated where they were written.
