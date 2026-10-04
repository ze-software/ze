# RFC 6071 - IP Security (IPsec) and Internet Key Exchange (IKE) Document Roadmap

No row in the public ledger. Every requirement this repository extracted from RFC 6071, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 8 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 8 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 8 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 8 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 8 | of 20 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (foundation), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 8 | of 8 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 100.0% | 8 of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 8 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 8 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

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
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Not enrolled (foundation) |
| Requirements | 20 |
| Gated MUST-level | 8 |
| Not applicable, so out of scope | 8 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc6071.md` |
| Requirement shard | `rfc/requirements/rfc6071.md` |
| RFC text | `rfc/full/rfc6071.txt` |

## Enrolment

Not enrolled (foundation, the document defines, registers or describes, and obliges no implementer, so there is no implementation anywhere for a gate to hold): An informational roadmap over the IPsec and IKE document set. It defines no behaviour of its own.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 6071.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 8 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (8):** [`RFC6071-5.1-1`](#rfc6071-5.1-1), [`RFC6071-5.1-2`](#rfc6071-5.1-2), [`RFC6071-5.3-1`](#rfc6071-5.3-1), [`RFC6071-5.4-1`](#rfc6071-5.4-1), [`RFC6071-5.5-1`](#rfc6071-5.5-1), [`RFC6071-5.1-3`](#rfc6071-5.1-3), [`RFC6071-5.5-2`](#rfc6071-5.5-2), [`RFC6071-5.1-4`](#rfc6071-5.1-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6071-5.1-1` | Requirement levels for ESP-NULL: IKEv1 - N/A IKEv2 - N/A ESP-v2 - MUST [RFC4835] ESP-v3 - MUST [RFC4835] (§5.2.1, IPsec-v3) | MUST | 5.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an ESP algorithm-implementation requirement owned by RFC 4835 (NULL encryption per RFC 2410), which governs ze's ESP dataplane (internal/component/ike/dataplane/xfrm_linux.go) |
| `RFC6071-5.1-2` | Requirement levels for AES-CBC with 128-bit keys: IKEv1 - SHOULD [RFC4109] IKEv2 - SHOULD+ [RFC4307] ESP-v2 - MUST [RFC4835] ESP-v3 - MUST [RFC4835] (§5.2.3, IPsec-v3) | MUST | 5.2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an ESP algorithm-implementation requirement owned by RFC 4835 (AES-CBC-128 per RFC 3602), which governs ze's ESP dataplane |
| `RFC6071-5.3-1` | Requirement levels for HMAC-SHA-1: IKEv1 - MUST [RFC4109] IKEv2 - MUST [RFC4307] IPsec-v2 - MUST [RFC4835] IPsec-v3 - MUST [RFC4835] (§5.3.1, IPsec-v3 and IKEv2) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an ESP/AH and IKEv2 integrity requirement owned by RFC 4835 and RFC 4307 (HMAC-SHA-1-96 per RFC 2404), which governs ze's ESP and IKE code |
| `RFC6071-5.4-1` | Requirement levels for PRF-HMAC-SHA1: IKEv1 - MUST [RFC4109] IKEv2 - MUST [RFC4307] (§5.5, IKEv2) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an IKEv2 PRF requirement owned by RFC 4307, which governs ze's IKEv2 transform negotiation (internal/component/ike/crypto/transform.go) |
| `RFC6071-5.5-1` | Requirement levels for DH MODP group 2: IKEv1 - MUST [RFC4109] IKEv2 - MUST- [RFC4307] (§5.7, IKEv1, deprecated) | MUST | 5.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an IKEv1 algorithm requirement owned by RFC 4109, and ze implements IKEv2 only (no IKEv1 code path) |
| `RFC6071-5.1-3` | Requirement levels for 3DES-CBC: IKEv1 - MUST [RFC4109] IKEv2 - MUST- [RFC4307] ESP-v2 - MUST [RFC4835] ESP-v3 - MUST- [RFC4835] (§5.2.2, MUST- / deprecated but mandatory) | MUST | 5.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an IKEv2 3DES-CBC requirement owned by RFC 4307, which governs ze's IKEv2 transform negotiation |
| `RFC6071-5.5-2` | Requirement levels for DH MODP group 2: IKEv1 - MUST [RFC4109] IKEv2 - MUST- [RFC4307] (§5.7, MUST- / deprecated but mandatory) | MUST | 5.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an IKEv2 MODP-1024 group requirement owned by RFC 4307, which governs ze's IKEv2 group negotiation |
| `RFC6071-5.1-4` | Requirement levels for 3DES-CBC: IKEv1 - MUST [RFC4109] IKEv2 - MUST- [RFC4307] ESP-v2 - MUST [RFC4835] ESP-v3 - MUST- [RFC4835] (§5.2.2, MUST- / deprecated but mandatory) | MUST | 5.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an ESP 3DES-CBC requirement owned by RFC 4835, which governs ze's ESP dataplane |
| `RFC6071-5.1-5` | Requirement levels for AES-CTR: IKEv1 - undefined (no IANA #) IKEv2 - optional [RFC5930] ESP-v2 - SHOULD [RFC4835] ESP-v3 - SHOULD [RFC4835] (§5.2.4, IPsec-v3) | SHOULD | 5.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.3-2` | Requirement levels for AES-XCBC-MAC: IKEv1 - undefined (no RFC) IKEv2 - optional IPsec-v2 - SHOULD+ [RFC4835] IPsec-v3 - SHOULD+ [RFC4835] (§5.3.2, SHOULD+ for IPsec-v3) | SHOULD | 5.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.1-6` | Requirement levels for AES-CBC with 128-bit keys: IKEv1 - SHOULD [RFC4109] IKEv2 - SHOULD+ [RFC4307] ESP-v2 - MUST [RFC4835] ESP-v3 - MUST [RFC4835] (§5.2.3, SHOULD+ for IKEv2) | SHOULD | 5.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.4-2` | Requirement levels for AES-XCBC-PRF: IKEv1 - undefined (no RFC) IKEv2 - SHOULD+ [RFC4307] (§5.5.1, SHOULD+ for IKEv2) | SHOULD | 5.5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.5-3` | Requirement levels for DH MODP group 14: IKEv1 - SHOULD [RFC4109] IKEv2 - SHOULD+ [RFC4307] (§5.7.1, SHOULD+ for IKEv2) | SHOULD | 5.7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.5-4` | Requirement levels for DH MODP group 14: IKEv1 - SHOULD [RFC4109] IKEv2 - SHOULD+ [RFC4307] (§5.7.1, SHOULD for IKEv1) | SHOULD | 5.7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-Key-1` | o AH [RFC4302] is mandatory to implement (MUST) in IPsec-v2, optional (MAY) in IPsec-v3 (§2.2.1, IPsec-v3) | MAY | 2.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.3-3` | Requirement levels for HMAC-MD5: IKEv1 - MAY [RFC4109] IKEv2 - optional [RFC4307] IPsec-v2 - MAY [RFC4835] IPsec-v3 - MAY [RFC4835] (§5.3.4, MAY for IPsec-v3) | MAY | 5.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.2-1` | Requirement levels for AES-GCM: IKEv1 - N/A IKEv2 - optional ESP-v2 - N/A ESP-v3 - optional [RFC4835] (§5.4.2) | MAY | 5.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.2-2` | Requirement levels for AES-CCM: IKEv1 - N/A IKEv2 - optional ESP-v2 - N/A ESP-v3 - optional [RFC4835] (§5.4.1) | MAY | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.5-5` | Requirement levels for DH EC groups 19-21: IKEv1 - optional [RFC4109] IKEv2 - optional (§5.7.2) | MAY | 5.7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.3-4` | Requirement levels for HMAC-SHA-256, HMAC-SHA-384, HMAC-SHA-512: IKEv1 - optional IKEv2 - optional IPsec-v2 - optional IPsec-v3 - optional (§5.3.3) | MAY | 5.3.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC6071-5.1-1`](#rfc6071-5.1-1) Requirement levels for ESP-NULL: IKEv1 - N/A IKEv2 - N/A ESP-v2 - MUST [RFC4835] ESP-v3 - MUST [RFC4835] (§5.2.1, IPsec-v3) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an ESP algorithm-implementation requirement owned by RFC 4835 (NULL encryption per RFC 2410), which governs ze's ESP dataplane (internal/component/ike/dataplane/xfrm_linux.go) |
| [`RFC6071-5.1-2`](#rfc6071-5.1-2) Requirement levels for AES-CBC with 128-bit keys: IKEv1 - SHOULD [RFC4109] IKEv2 - SHOULD+ [RFC4307] ESP-v2 - MUST [RFC4835] ESP-v3 - MUST [RFC4835] (§5.2.3, IPsec-v3) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an ESP algorithm-implementation requirement owned by RFC 4835 (AES-CBC-128 per RFC 3602), which governs ze's ESP dataplane |
| [`RFC6071-5.3-1`](#rfc6071-5.3-1) Requirement levels for HMAC-SHA-1: IKEv1 - MUST [RFC4109] IKEv2 - MUST [RFC4307] IPsec-v2 - MUST [RFC4835] IPsec-v3 - MUST [RFC4835] (§5.3.1, IPsec-v3 and IKEv2) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an ESP/AH and IKEv2 integrity requirement owned by RFC 4835 and RFC 4307 (HMAC-SHA-1-96 per RFC 2404), which governs ze's ESP and IKE code |
| [`RFC6071-5.4-1`](#rfc6071-5.4-1) Requirement levels for PRF-HMAC-SHA1: IKEv1 - MUST [RFC4109] IKEv2 - MUST [RFC4307] (§5.5, IKEv2) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an IKEv2 PRF requirement owned by RFC 4307, which governs ze's IKEv2 transform negotiation (internal/component/ike/crypto/transform.go) |
| [`RFC6071-5.5-1`](#rfc6071-5.5-1) Requirement levels for DH MODP group 2: IKEv1 - MUST [RFC4109] IKEv2 - MUST- [RFC4307] (§5.7, IKEv1, deprecated) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an IKEv1 algorithm requirement owned by RFC 4109, and ze implements IKEv2 only (no IKEv1 code path) |
| [`RFC6071-5.1-3`](#rfc6071-5.1-3) Requirement levels for 3DES-CBC: IKEv1 - MUST [RFC4109] IKEv2 - MUST- [RFC4307] ESP-v2 - MUST [RFC4835] ESP-v3 - MUST- [RFC4835] (§5.2.2, MUST- / deprecated but mandatory) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an IKEv2 3DES-CBC requirement owned by RFC 4307, which governs ze's IKEv2 transform negotiation |
| [`RFC6071-5.5-2`](#rfc6071-5.5-2) Requirement levels for DH MODP group 2: IKEv1 - MUST [RFC4109] IKEv2 - MUST- [RFC4307] (§5.7, MUST- / deprecated but mandatory) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an IKEv2 MODP-1024 group requirement owned by RFC 4307, which governs ze's IKEv2 group negotiation |
| [`RFC6071-5.1-4`](#rfc6071-5.1-4) Requirement levels for 3DES-CBC: IKEv1 - MUST [RFC4109] IKEv2 - MUST- [RFC4307] ESP-v2 - MUST [RFC4835] ESP-v3 - MUST- [RFC4835] (§5.2.2, MUST- / deprecated but mandatory) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an ESP 3DES-CBC requirement owned by RFC 4835, which governs ze's ESP dataplane |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6071-5.1-1`](#rfc6071-5.1-1)

Requirement levels for ESP-NULL: IKEv1 - N/A IKEv2 - N/A ESP-v2 - MUST [RFC4835] ESP-v3 - MUST [RFC4835] (§5.2.1, IPsec-v3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6071-5.1-1, so no unit is bound to it.

### [`RFC6071-5.1-2`](#rfc6071-5.1-2)

Requirement levels for AES-CBC with 128-bit keys: IKEv1 - SHOULD [RFC4109] IKEv2 - SHOULD+ [RFC4307] ESP-v2 - MUST [RFC4835] ESP-v3 - MUST [RFC4835] (§5.2.3, IPsec-v3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6071-5.1-2, so no unit is bound to it.

### [`RFC6071-5.3-1`](#rfc6071-5.3-1)

Requirement levels for HMAC-SHA-1: IKEv1 - MUST [RFC4109] IKEv2 - MUST [RFC4307] IPsec-v2 - MUST [RFC4835] IPsec-v3 - MUST [RFC4835] (§5.3.1, IPsec-v3 and IKEv2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6071-5.3-1, so no unit is bound to it.

### [`RFC6071-5.4-1`](#rfc6071-5.4-1)

Requirement levels for PRF-HMAC-SHA1: IKEv1 - MUST [RFC4109] IKEv2 - MUST [RFC4307] (§5.5, IKEv2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6071-5.4-1, so no unit is bound to it.

### [`RFC6071-5.5-1`](#rfc6071-5.5-1)

Requirement levels for DH MODP group 2: IKEv1 - MUST [RFC4109] IKEv2 - MUST- [RFC4307] (§5.7, IKEv1, deprecated)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6071-5.5-1, so no unit is bound to it.

### [`RFC6071-5.1-3`](#rfc6071-5.1-3)

Requirement levels for 3DES-CBC: IKEv1 - MUST [RFC4109] IKEv2 - MUST- [RFC4307] ESP-v2 - MUST [RFC4835] ESP-v3 - MUST- [RFC4835] (§5.2.2, MUST- / deprecated but mandatory)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6071-5.1-3, so no unit is bound to it.

### [`RFC6071-5.5-2`](#rfc6071-5.5-2)

Requirement levels for DH MODP group 2: IKEv1 - MUST [RFC4109] IKEv2 - MUST- [RFC4307] (§5.7, MUST- / deprecated but mandatory)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6071-5.5-2, so no unit is bound to it.

### [`RFC6071-5.1-4`](#rfc6071-5.1-4)

Requirement levels for 3DES-CBC: IKEv1 - MUST [RFC4109] IKEv2 - MUST- [RFC4307] ESP-v2 - MUST [RFC4835] ESP-v3 - MUST- [RFC4835] (§5.2.2, MUST- / deprecated but mandatory)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6071-5.1-4, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc6071.txt |
| Source fingerprint | a844625658606a3d |
| Record | rfc/extraction/rfc6071.json |
| Mapped sentences | 6 |
| Declined as scope | 13 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.2.1` | not stated | 2 | walked | not stated |
| `2.3` | not stated | 0 | walked | not stated |
| `2.3.1` | not stated | 0 | walked | not stated |
| `2.4` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.1.1` | not stated | 0 | walked | not stated |
| `3.1.1.1` | not stated | 0 | walked | not stated |
| `3.1.1.2` | not stated | 0 | walked | not stated |
| `3.1.1.3` | not stated | 0 | walked | not stated |
| `3.1.2` | not stated | 0 | walked | not stated |
| `3.1.2.1` | not stated | 0 | walked | not stated |
| `3.1.2.2` | not stated | 0 | walked | not stated |
| `3.1.2.3` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.2.1` | not stated | 0 | walked | not stated |
| `3.2.2` | not stated | 0 | walked | not stated |
| `3.2.3` | not stated | 0 | walked | not stated |
| `3.2.4` | not stated | 0 | walked | not stated |
| `3.2.5` | not stated | 0 | walked | not stated |
| `3.2.6` | not stated | 0 | walked | not stated |
| `3.2.7` | not stated | 0 | walked | not stated |
| `3.2.8` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `3.3.1` | not stated | 0 | walked | not stated |
| `3.3.2` | not stated | 0 | walked | not stated |
| `3.3.3` | not stated | 0 | walked | not stated |
| `3.3.4` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `4.1.1` | not stated | 0 | walked | not stated |
| `4.1.1.1` | not stated | 0 | walked | not stated |
| `4.1.1.2` | not stated | 0 | walked | not stated |
| `4.1.1.3` | not stated | 0 | walked | not stated |
| `4.1.1.4` | not stated | 0 | walked | not stated |
| `4.1.2` | not stated | 0 | walked | not stated |
| `4.1.2.1` | not stated | 0 | walked | not stated |
| `4.1.2.2` | not stated | 0 | walked | not stated |
| `4.1.2.3` | not stated | 0 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.2.1` | not stated | 0 | walked | not stated |
| `4.2.1.1` | not stated | 0 | walked | not stated |
| `4.2.1.2` | not stated | 0 | walked | not stated |
| `4.2.1.3` | not stated | 0 | walked | not stated |
| `4.2.1.4` | not stated | 0 | walked | not stated |
| `4.2.2` | not stated | 0 | walked | not stated |
| `4.2.2.1` | not stated | 0 | walked | not stated |
| `4.2.2.2` | not stated | 0 | walked | not stated |
| `4.2.2.3` | not stated | 0 | walked | not stated |
| `4.2.3` | not stated | 0 | walked | not stated |
| `4.2.3.1` | not stated | 0 | walked | not stated |
| `4.2.4` | not stated | 0 | walked | not stated |
| `4.2.4.1` | not stated | 0 | walked | not stated |
| `4.2.4.2` | not stated | 0 | walked | not stated |
| `4.2.4.3` | not stated | 0 | walked | not stated |
| `4.2.4.4` | not stated | 0 | walked | not stated |
| `4.2.4.5` | not stated | 0 | walked | not stated |
| `4.2.4.6` | not stated | 0 | walked | not stated |
| `5` | not stated | 1 | walked | not stated |
| `5.1` | not stated | 1 | walked | not stated |
| `5.1.1` | not stated | 0 | walked | not stated |
| `5.1.2` | not stated | 0 | walked | not stated |
| `5.1.3` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `5.2.1` | not stated | 1 | walked | not stated |
| `5.2.2` | not stated | 1 | walked | not stated |
| `5.2.3` | not stated | 2 | walked | not stated |
| `5.2.4` | not stated | 1 | walked | not stated |
| `5.2.5` | not stated | 0 | walked | not stated |
| `5.2.6` | not stated | 1 | walked | not stated |
| `5.2.7` | not stated | 0 | walked | not stated |
| `5.2.8` | not stated | 0 | walked | not stated |
| `5.3` | not stated | 0 | walked | not stated |
| `5.3.1` | not stated | 1 | walked | not stated |
| `5.3.2` | not stated | 0 | walked | not stated |
| `5.3.3` | not stated | 0 | walked | not stated |
| `5.3.4` | not stated | 0 | walked | not stated |
| `5.3.5` | not stated | 0 | walked | not stated |
| `5.3.6` | not stated | 0 | walked | not stated |
| `5.3.7` | not stated | 0 | walked | not stated |
| `5.4` | not stated | 0 | walked | not stated |
| `5.4.1` | not stated | 2 | walked | not stated |
| `5.4.2` | not stated | 2 | walked | not stated |
| `5.4.3` | not stated | 0 | walked | not stated |
| `5.4.4` | not stated | 0 | walked | not stated |
| `5.5` | not stated | 1 | walked | not stated |
| `5.5.1` | not stated | 0 | walked | not stated |
| `5.5.2` | not stated | 0 | walked | not stated |
| `5.6` | not stated | 0 | walked | not stated |
| `5.6.1` | not stated | 0 | walked | not stated |
| `5.6.2` | not stated | 0 | walked | not stated |
| `5.7` | not stated | 1 | walked | not stated |
| `5.7.1` | not stated | 0 | walked | not stated |
| `5.7.2` | not stated | 0 | walked | not stated |
| `5.7.3` | not stated | 0 | walked | not stated |
| `5.7.4` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `6.3` | not stated | 0 | walked | not stated |
| `6.4` | not stated | 0 | walked | not stated |
| `6.5` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.1.1` | not stated | 0 | walked | not stated |
| `7.1.2` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `7.2.1` | not stated | 0 | walked | not stated |
| `7.3` | not stated | 0 | walked | not stated |
| `7.3.1` | not stated | 0 | walked | not stated |
| `7.4` | not stated | 0 | walked | not stated |
| `7.4.1` | not stated | 0 | walked | not stated |
| `7.4.2` | not stated | 0 | walked | not stated |
| `7.4.3` | not stated | 0 | walked | not stated |
| `7.5` | not stated | 0 | walked | not stated |
| `7.5.1` | not stated | 0 | walked | not stated |
| `7.5.2` | not stated | 0 | walked | not stated |
| `7.6` | not stated | 0 | walked | not stated |
| `7.6.1` | not stated | 0 | walked | not stated |
| `7.6.2` | not stated | 0 | walked | not stated |
| `7.7` | not stated | 0 | walked | not stated |
| `7.7.1` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.1.1` | not stated | 0 | walked | not stated |
| `8.1.2` | not stated | 0 | walked | not stated |
| `8.1.3` | not stated | 0 | walked | not stated |
| `8.1.4` | not stated | 0 | walked | not stated |
| `8.1.5` | not stated | 0 | walked | not stated |
| `8.1.6` | not stated | 0 | walked | not stated |
| `8.1.7` | not stated | 0 | walked | not stated |
| `8.1.8` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `8.2.1` | not stated | 0 | walked | not stated |
| `8.3` | not stated | 0 | walked | not stated |
| `8.3.1` | not stated | 0 | walked | not stated |
| `8.3.2` | not stated | 0 | walked | not stated |
| `8.3.3` | not stated | 0 | walked | not stated |
| `8.3.4` | not stated | 0 | walked | not stated |
| `8.4` | not stated | 0 | walked | not stated |
| `8.4.1` | not stated | 0 | walked | not stated |
| `8.5` | not stated | 0 | walked | not stated |
| `8.5.1` | not stated | 0 | walked | not stated |
| `8.5.2` | not stated | 0 | walked | not stated |
| `8.5.3` | not stated | 0 | walked | not stated |
| `8.5.4` | not stated | 0 | walked | not stated |
| `8.5.5` | not stated | 0 | walked | not stated |
| `8.6` | not stated | 0 | walked | not stated |
| `8.6.1` | not stated | 0 | walked | not stated |
| `8.7` | not stated | 0 | walked | not stated |
| `8.7.1` | not stated | 0 | walked | not stated |
| `8.7.2` | not stated | 0 | walked | not stated |
| `8.8` | not stated | 0 | walked | not stated |
| `8.8.1` | not stated | 0 | walked | not stated |
| `8.9` | not stated | 0 | walked | not stated |
| `8.9.1` | not stated | 0 | walked | not stated |
| `8.10` | not stated | 0 | walked | not stated |
| `8.10.1` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.1.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |
| `9.2.1` | not stated | 0 | walked | not stated |
| `9.3` | not stated | 0 | walked | not stated |
| `9.3.1` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `12.1` | not stated | 0 | walked | not stated |
| `A` | not stated | 2 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2.2.1:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 6071 is a roadmap document. The bullet reports what the IPsec architecture documents require of AH: the "(MUST)" belongs to IPsec-v2 (RFC 2401 with RFC 4302), which this bullet cites, and the "(MAY)" to IPsec-v3 (RFC 4301). RFC 6071 states no obligation of its own here. | o AH [RFC4302] is mandatory to implement (MUST) in IPsec-v2, optional (MAY) in IPsec-v3 |
| `2.2.1:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | A bullet in the IPsec-v2 to IPsec-v3 difference list, restating what RFC 2406 and RFC 4303 require of ESP NULL authentication. Section 1 states the roadmap's own standing: "This document does not define requirement levels; it simply restates those found in the IKE and IPsec RFCs." | o NULL authentication, mandatory (MUST) in ESP-v2, is optional (MAY) in ESP-v3 |
| `5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A description of the vocabulary this roadmap uses to print requirement levels. The keywords name the classification terms, not an obligation on an implementation. | For each RFC that describes a cryptographic algorithm, this roadmap will classify its requirement level for each protocol, as either MUST, SHOULD, or MAY [RFC2119]; SHOULD+, SHOULD-, or MUST- [RFC4835]; optional; undefined; or N/A (not applicable). |
| `5.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A description of another system: it says that RFC 4835 and RFC 4307 each classify algorithms as MUST, SHOULD, MAY and SHOULD NOT. The keywords name those documents' vocabulary. | IPsec-v3 and IKEv2 each have an RFC that specifies their mandatory- to-implement (MUST), recommended (SHOULD), optional (MAY), and deprecated (SHOULD NOT) algorithms. |
| `5.2.3:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The AES-CBC key-size obligation belongs to RFC 3602, the document Section 5.2.3 catalogues. Section 1: "This document does not define requirement levels; it simply restates those found in the IKE and IPsec RFCs." | If AES-CBC is implemented, 128-bit keys are MUST; the other sizes are MAY. |
| `5.2.4:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The AES-CTR key-size obligation belongs to RFC 3686, the document Section 5.2.4 catalogues. Section 1: "This document does not define requirement levels; it simply restates those found in the IKE and IPsec RFCs." | If AES-CTR is implemented, 128-bit keys are MUST; 192- and 256-byte keys are MAY. |
| `5.2.6:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The Camellia-CBC key-size obligation belongs to RFC 4312, the document Section 5.2.6 catalogues. Section 1: "This document does not define requirement levels; it simply restates those found in the IKE and IPsec RFCs." | If Camellia-CBC is implemented, 128-bit keys are MUST; the other sizes are MAY. |
| `5.4.1:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The AES-CCM key-size obligation belongs to RFC 4309, the document Section 5.4.1 catalogues. Section 1: "This document does not define requirement levels; it simply restates those found in the IKE and IPsec RFCs." | If AES-CCM is implemented, 128-bit keys are MUST; the other sizes are MAY. |
| `5.4.1:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The AES-CCM ICV-size obligation belongs to RFC 4309, the document Section 5.4.1 catalogues. Section 1: "This document does not define requirement levels; it simply restates those found in the IKE and IPsec RFCs." | ICV sizes of 64 and 128 bits are MUST; 96 bits is MAY. |
| `5.4.2:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The AES-GCM key-size obligation belongs to RFC 4106, the document Section 5.4.2 catalogues. Section 1: "This document does not define requirement levels; it simply restates those found in the IKE and IPsec RFCs." | If AES-GCM is implemented, 128-bit keys are MUST; the other sizes are MAY. |
| `5.4.2:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The AES-GCM ICV-size obligation belongs to RFC 4106, the document Section 5.4.2 catalogues. Section 1: "This document does not define requirement levels; it simply restates those found in the IKE and IPsec RFCs." | An ICV size of 128 bits is a MUST; 64 and 96 bits are MAY. |
| `A:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Appendix A is a reference table that reprints the requirement levels Section 5 already catalogues. Section 1: "This document does not define requirement levels; it simply restates those found in the IKE and IPsec RFCs." | +--------------------------+----------------------------------------+ \| ALGORITHM \| REQUIREMENT LEVEL \| \| \| IKEv1 IKEv2 IPsec-v2 IPsec-v3 \| +--------------------------+----------------------------------------+ \|Encryption Algorithms: \| \|--------------------- \| \| ESP-NULL \| N/A N/A MUST MUST \| \| \| \| \| 3DES-CBC \| MUST MUST- MUST MUST- \| \| \| \| \| Blowfish/CAST/IDEA/RC5 \| optional optional optional optional \| \| \| \| \| AES-CBC 128-bit key \| SHOULD SHOULD+ MUST MUST \| \| \| \| \| AES-CBC 192/256-bit key \| optional optional optional optional \| \| \| \| \| AES-CTR \| undefined optional SHOULD SHOULD \| \| \| \| \| Camellia-CBC \| optional optional optional optional \| \| \| \| \| Camellia-CTR \| undefined undefined undefined optional \| \| \| \| \| SEED-CBC \| undefined undefined optional undefined\| \| \| \| \|Integrity-Protection Algorithms: \| \|------------------------------ \| \| HMAC-SHA-1 \| MUST MUST MUST MUST \| \| \| \| \| AES-XCBC-MAC \| undefined optional SHOULD+ SHOULD+ \| \| \| \| \| HMAC-SHA-256/384/512 \| optional optional optional optional \| \| \| \| \| AES-GMAC \| N/A N/A undefined optional \| \| \| \| \| HMAC-MD5 \| MAY optional MAY MAY \| \| \| \| \| AES-CMAC \| undefined optional undefined optional \| \| \| \| \| HMAC-RIPEMD \| undefined undefined optional undefined\| +--------------------------+----------------------------------------+ Table 1: Algorithm Requirement Levels (continued) |
| `A:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Appendix A is a reference table that reprints the requirement levels Section 5 already catalogues. Section 1: "This document does not define requirement levels; it simply restates those found in the IKE and IPsec RFCs." | +--------------------------+----------------------------------------+ \| ALGORITHM \| REQUIREMENT LEVEL \| \| \| IKEv1 IKEv2 IPsec-v2 IPsec-v3 \| +--------------------------+----------------------------------------+ \|Combined Mode Algorithms: \| \|------------------------ \| \| AES-CCM \| N/A optional N/A optional \| \| \| \| \| AES-GCM \| N/A optional N/A optional \| \| \| \| \| AES-GMAC \| N/A N/A undefined optional \| \| \| \| \| Camellia-CCM \| N/A undefined N/A optional \| \| \| \| \|Pseudorandom Functions: \| \|----------------------- \| \| PRF-HMAC-SHA1 \| MUST MUST \| \| \| \| \| PRF-HMAC-SHA-256/384/512 \| optional optional \| \| \| \| \| AES-XCBC-PRF \| undefined SHOULD+ \| \| \| \| \| AES-CMAC-PRF \| undefined optional \| \| \| \| \|Diffie-Hellman Algorithms: \| \|------------------------- \| \| DH MODP grp 1 \| MAY optional \| \| \| \| \| DH MODP grp 2 \| MUST MUST- \| \| \| \| \| DH MODP grp 5 \| optional optional \| \| \| \| \| DH MODP grp 14 \| SHOULD SHOULD+ \| \| \| \| \| DH MODP grp 15-18 \| optional optional \| \| \| \| \| DH MODP grp 22-24 \| optional optional \| \| \| \| \| DH EC grp 3-4 \| MAY undefined \| \| \| \| \| DH EC grp 19-21 \| optional optional \| \| \| \| \| DH EC grp 25-26 \| optional optional \| +--------------------------+----------------------------------------+ Authors' Addresses |

## Superseded

No document obsoletes RFC 6071, so its obligations are stated where they were written.
