# RFC 6911 - RADIUS Attributes for IPv6 Access Networks

Future. Every requirement this repository extracted from RFC 6911, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 3 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 3 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 3 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 3 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 3 | of 12 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (out-of-scope), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 3 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 3 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 3 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 3 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 3 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 3 of 3 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 shares marked as a part above are the whole of the 3 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 12 |
| Gated MUST-level | 3 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 3 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc6911.md` |
| Requirement shard | `rfc/requirements/rfc6911.md` |
| RFC text | `rfc/full/rfc6911.txt` |

## Enrolment

Not enrolled (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OUT OF SCOPE as a document by owner decision, 2026-10-26: the owner rule committed in 1e7a9dbc34 (ai/META-DECISIONS.md, Goal and scope) counts an RFC a profile lists as optional only once Thomas brings it into scope, and this summary's `Optional use cases` row lists this RFC in the optional tier of the lns profile. The extraction is COMPLETE: the source text is at rfc/full/rfc6911.txt, every capitalised MUST-level sentence is a row of this summary or an exclusion in rfc/extraction/rfc6911.json (walked 2026-10-26), and none is gated. Ze reads none of the five attributes (types 168 to 172) from an Access-Accept and writes none (internal/component/l2tp/plugins/authradius/extract.go extractAuthMetadata).

## What the public ledger says

**Status:** Future

**What the ledger says is covered**

- None. Framed-IPv6-Address, DNS-Server-IPv6-Address, Route-IPv6-Information, Delegated-IPv6-Prefix-Pool and Stateful-IPv6-Address-Pool are ignored when received. The radius dictionary ([`internal/component/radius/dict.go`](https://github.com/ze-software/ze/blob/main/internal/component/radius/dict.go)) defines none of these attributes
- its type 99, Framed-IPv6-Route, is RFC 3162's (Section 2.5).


**What the ledger says remains:**

All five attributes. Out of scope by owner decision, 2026-10-26.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 3 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **3** | every gated MUST falls in exactly one bucket above |

**No test and no annotation (3):** [`RFC6911-3.3-1`](#rfc6911-3.3-1), [`RFC6911-3.4-1`](#rfc6911-3.4-1), [`RFC6911-3.5-1`](#rfc6911-3.5-1)

## Attainable ceiling

Mandatory ceiling: 3 of 3 (100.0%). Optional ceiling: 9 of 9 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc6911 3 of 3 (100.0%). Lowest optional ceiling: rfc6911 9 of 9 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6911-3.3-1` | The bits in the prefix after the prefix length, if any, are reserved and MUST be initialized to zero. (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC6911-3.4-1` | If a NAS does not support prefix pools, the NAS MUST ignore this Attribute. (§3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6911-3.5-1` | If a NAS does not support address pools, the NAS MUST ignore this Attribute. (§3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6911-3.2-3` | If more than one DNS-Server-IPv6-Address Attribute is present in the Access- Accept packet, the addresses from the Attributes SHOULD be copied in the same order as received. (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6911-3.4-2` | The Delegated-IPv6-Prefix-Pool Attribute contains the name of an assigned pool that SHOULD be used to select an IPv6 delegated prefix for the user on the NAS. (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6911-3.5-2` | The Stateful-IPv6-Address-Pool Attribute contains the name of an assigned pool that SHOULD be used to select an IPv6 address for the user on the NAS. (§3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6911-3.1-1` | It MAY be used in Access-Accept packets and MAY appear multiple times. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6911-3.1-2` | It MAY be used in an Access-Request packet as a hint by the NAS to the RADIUS server that it would prefer this IPv6 address, but the RADIUS server is not required to honor the hint. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6911-3.2-1` | This Attribute MAY be included multiple times in Access- Accept packets when the intention is for a NAS to announce more than one DNS server address to an RG/host. (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6911-3.2-2` | The Attribute MAY be used in an Access-Request packet as a hint by the NAS to the RADIUS server regarding the DNS IPv6 address, but the RADIUS server is not required to honor the hint. (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6911-3.3-2` | It MAY be used in an Access-Request packet as a hint by the NAS to the RADIUS server, but the RADIUS server is not required to honor the hint. (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC6911-3.4-3` | It MAY be used in an Access-Request packet as a hint by the NAS to the RADIUS server regarding the pool, but the RADIUS server is not required to honor the hint. (§3.4) | MAY | 3.4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC6911-3.3-1`](#rfc6911-3.3-1) The bits in the prefix after the prefix length, if any, are reserved and MUST be initialized to zero. (§3.3) | no test | no test carries this requirement id |
| [`RFC6911-3.4-1`](#rfc6911-3.4-1) If a NAS does not support prefix pools, the NAS MUST ignore this Attribute. (§3.4) | no test | no test carries this requirement id |
| [`RFC6911-3.5-1`](#rfc6911-3.5-1) If a NAS does not support address pools, the NAS MUST ignore this Attribute. (§3.5) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6911-3.3-1`](#rfc6911-3.3-1)

The bits in the prefix after the prefix length, if any, are reserved and MUST be initialized to zero. (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6911-3.3-1, so no unit is bound to it.

### [`RFC6911-3.4-1`](#rfc6911-3.4-1)

If a NAS does not support prefix pools, the NAS MUST ignore this Attribute. (§3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6911-3.4-1, so no unit is bound to it.

### [`RFC6911-3.5-1`](#rfc6911-3.5-1)

If a NAS does not support address pools, the NAS MUST ignore this Attribute. (§3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6911-3.5-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc6911.txt |
| Source fingerprint | b20f9aa7e2880b8f |
| Record | rfc/extraction/rfc6911.json |
| Mapped sentences | 3 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `1.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.3` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.4` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.5` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.3` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.4` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.5` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.6` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `5` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `8` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `8.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `8.2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |

### Excluded sentences

The walk over RFC 6911 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 6911, so its obligations are stated where they were written.
