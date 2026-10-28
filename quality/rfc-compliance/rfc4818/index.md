# RFC 4818 - RADIUS Delegated-IPv6-Prefix Attribute

Future. Every requirement this repository extracted from RFC 4818, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

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
| MUSTs declared | 3 | of 6 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (out-of-scope), so every share below reads what the summary records rather than what the gate enforces |
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
| Requirements | 6 |
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
| Summary | `rfc/short/rfc4818.md` |
| Requirement shard | `rfc/requirements/rfc4818.md` |
| RFC text | `rfc/full/rfc4818.txt` |

## Enrolment

Not enrolled (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OUT OF SCOPE as a document by owner decision, 2026-10-26: the owner rule committed in 1e7a9dbc34 (ai/META-DECISIONS.md, Goal and scope) counts an RFC a profile lists as optional only once Thomas brings it into scope, and this summary's `Optional use cases` row lists this RFC in the optional tier of the lns profile. The extraction is COMPLETE: the source text is at rfc/full/rfc4818.txt, every capitalised MUST-level sentence is a row of this summary or an exclusion in rfc/extraction/rfc4818.json (walked 2026-10-26), and none is gated. Ze reads no Delegated-IPv6-Prefix from an Access-Accept and writes none (internal/component/l2tp/plugins/authradius/extract.go extractAuthMetadata, handler.go buildAccessRequestAttrs, acct.go buildAcctPacket).

## What the public ledger says

**Status:** Future

**What the ledger says is covered:**

None. An Access-Accept carrying Delegated-IPv6-Prefix (123) is accepted and the attribute is ignored.

**What the ledger says remains:**

Reading Delegated-IPv6-Prefix into the subscriber's DHCPv6-PD delegation, and reporting it in accounting. Out of scope by owner decision, 2026-10-26.

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

**No test and no annotation (3):** [`RFC4818-3-1`](#rfc4818-3-1), [`RFC4818-3-2`](#rfc4818-3-2), [`RFC4818-4-1`](#rfc4818-4-1)

## Attainable ceiling

Mandatory ceiling: 3 of 3 (100.0%). Optional ceiling: 3 of 3 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc4818 3 of 3 (100.0%). Lowest optional ceiling: rfc4818 3 of 3 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4818-3-1` | Any bits in the prefix field that are not part of the prefix MUST be zero. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4818-3-2` | The Delegated-IPv6-Prefix MUST NOT appear in any other RADIUS packets. (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4818-4-1` | The meaning of the above table entries is as follows: 0 This attribute MUST NOT be present. 0+ Zero or more instances of this attribute MAY be present. 0-1 Zero or one instance of this attribute MAY be present. 1 Exactly one instance of this attribute MUST be present. 1+ One or more of these attributes MUST be present. (§4) | MUST NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4818-3-3` | The Delegated-IPv6-Prefix MAY appear in an Access-Accept packet, and can appear multiple times. (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4818-3-4` | It MAY appear in an Access-Request packet as a hint by the NAS to the server that it would prefer these prefix(es), but the server is not required to honor the hint. (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4818-3-5` | The Delegated-IPv6-Prefix attribute MAY appear in an Accounting- Request packet. (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4818-3-1`](#rfc4818-3-1) Any bits in the prefix field that are not part of the prefix MUST be zero. (§3) | no test | no test carries this requirement id |
| [`RFC4818-3-2`](#rfc4818-3-2) The Delegated-IPv6-Prefix MUST NOT appear in any other RADIUS packets. (§3) | no test | no test carries this requirement id |
| [`RFC4818-4-1`](#rfc4818-4-1) The meaning of the above table entries is as follows: 0 This attribute MUST NOT be present. 0+ Zero or more instances of this attribute MAY be present. 0-1 Zero or one instance of this attribute MAY be present. 1 Exactly one instance of this attribute MUST be present. 1+ One or more of these attributes MUST be present. (§4) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4818-3-1`](#rfc4818-3-1)

Any bits in the prefix field that are not part of the prefix MUST be zero. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4818-3-1, so no unit is bound to it.

### [`RFC4818-3-2`](#rfc4818-3-2)

The Delegated-IPv6-Prefix MUST NOT appear in any other RADIUS packets. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4818-3-2, so no unit is bound to it.

### [`RFC4818-4-1`](#rfc4818-4-1)

The meaning of the above table entries is as follows: 0 This attribute MUST NOT be present. 0+ Zero or more instances of this attribute MAY be present. 0-1 Zero or one instance of this attribute MAY be present. 1 Exactly one instance of this attribute MUST be present. 1+ One or more of these attributes MUST be present. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4818-4-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc4818.txt |
| Source fingerprint | a7bff0b57f7a36e1 |
| Record | rfc/extraction/rfc4818.json |
| Mapped sentences | 3 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3` | not stated | 2 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `5` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `8` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `8.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `9.2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `5:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The Diameter AVP flag rules table. Its MUST, MAY and MUST NOT columns are the M, P and V flag rules of the Diameter base protocol, RFC 3588 Section 4.1, which the section cites; a RADIUS attribute carries no AVP flags. | +---------------------+ \| AVP Flag rules \| \|----+-----+----+-----\|----+ AVP \| \| \|SHLD\| MUST\| \| Attribute Name Code Value Type \|MUST\| MAY \| NOT\| NOT\|Encr\| ---------------------------------\|----+-----+----+-----\|----\| Delegated-IPv6- 123 OctetString\| M \| P \| \| V \| Y \| Prefix \| \| \| \| \| \| ---------------------------------\|----+-----+----+-----\|----\| |

## Superseded

No document obsoletes RFC 4818, so its obligations are stated where they were written.
