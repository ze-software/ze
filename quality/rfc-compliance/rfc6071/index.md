# RFC 6071 - IP Security (IPsec) and Internet Key Exchange (IKE) Document Roadmap

No row in the public ledger. Every requirement this repository extracted from RFC 6071, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 0 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 0 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 0 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 0 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 8 | of 20 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 8 | of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

No card above is a share of a population, so there is nothing to add up.

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
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 20 |
| Gated MUST-level | 8 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 8 |
| Declared gaps | 0 |
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

Enrolled: IP Security (IPsec) and Internet Key Exchange (IKE) Document Roadmap: eight MUST-level requirements, all {not-applicable}. RFC 6071 is an informational roadmap that catalogs the IPsec/IKE specifications and defines no independent protocol behavior; each gated MUST restates an algorithm-implementation requirement owned by RFC 4835 (ESP/AH algorithms), RFC 4307 (IKEv2 algorithms), or RFC 4109 (IKEv1, which ze does not implement). The concrete algorithm behavior lives in ze's IKEv2 transform negotiation (internal/component/ike/crypto/transform.go) and ESP dataplane (internal/component/ike/dataplane/xfrm_linux.go), governed by those owning RFCs rather than by the roadmap.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 6071.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 8 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (8):** [`RFC6071-5.1-1`](#rfc6071-5.1-1), [`RFC6071-5.1-2`](#rfc6071-5.1-2), [`RFC6071-5.3-1`](#rfc6071-5.3-1), [`RFC6071-5.4-1`](#rfc6071-5.4-1), [`RFC6071-5.5-1`](#rfc6071-5.5-1), [`RFC6071-5.1-3`](#rfc6071-5.1-3), [`RFC6071-5.5-2`](#rfc6071-5.5-2), [`RFC6071-5.1-4`](#rfc6071-5.1-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6071-5.1-1` | ESP: NULL encryption must be implemented (§5.1, IPsec-v3) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an ESP algorithm-implementation requirement owned by RFC 4835 (NULL encryption per RFC 2410), which governs ze's ESP dataplane (internal/component/ike/dataplane/xfrm_linux.go) |
| `RFC6071-5.1-2` | ESP: AES-CBC-128 encryption must be implemented (§5.1, IPsec-v3) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an ESP algorithm-implementation requirement owned by RFC 4835 (AES-CBC-128 per RFC 3602), which governs ze's ESP dataplane |
| `RFC6071-5.3-1` | ESP/AH: HMAC-SHA-1-96 integrity must be implemented (§5.3, IPsec-v3 and IKEv2) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an ESP/AH and IKEv2 integrity requirement owned by RFC 4835 and RFC 4307 (HMAC-SHA-1-96 per RFC 2404), which governs ze's ESP and IKE code |
| `RFC6071-5.4-1` | IKEv2: PRF-HMAC-SHA-1 must be implemented (§5.4, IKEv2) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an IKEv2 PRF requirement owned by RFC 4307, which governs ze's IKEv2 transform negotiation (internal/component/ike/crypto/transform.go) |
| `RFC6071-5.5-1` | IKEv1: MODP group 2 (1024-bit) must be supported (§5.5, IKEv1, deprecated) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an IKEv1 algorithm requirement owned by RFC 4109, and ze implements IKEv2 only (no IKEv1 code path) |
| `RFC6071-5.1-3` | IKEv2: 3DES-CBC must be supported (§5.1, MUST- / deprecated but mandatory) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an IKEv2 3DES-CBC requirement owned by RFC 4307, which governs ze's IKEv2 transform negotiation |
| `RFC6071-5.5-2` | IKEv2: MODP group 2 (1024-bit) must be supported (§5.5, MUST- / deprecated but mandatory) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an IKEv2 MODP-1024 group requirement owned by RFC 4307, which governs ze's IKEv2 group negotiation |
| `RFC6071-5.1-4` | ESP: 3DES-CBC must be supported (§5.1, MUST- / deprecated but mandatory) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an ESP 3DES-CBC requirement owned by RFC 4835, which governs ze's ESP dataplane |
| `RFC6071-5.1-5` | ESP: AES-CTR encryption should be implemented (§5.1, IPsec-v3) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.3-2` | ESP/AH: AES-XCBC-MAC-96 integrity should be implemented (§5.3, SHOULD+ for IPsec-v3) | SHOULD | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.1-6` | IKEv2: AES-CBC-128 encryption should be implemented (§5.1, SHOULD+ for IKEv2) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.4-2` | IKEv2: AES-XCBC-PRF-128 should be implemented (§5.4, SHOULD+ for IKEv2) | SHOULD | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.5-3` | IKEv2: MODP group 14 (2048-bit) should be supported (§5.5, SHOULD+ for IKEv2) | SHOULD | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.5-4` | IKEv1: MODP group 14 (2048-bit) should be supported (§5.5, SHOULD for IKEv1) | SHOULD | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-Key-1` | IPsec-v3: AH is optional to implement (§Key Architecture Changes, IPsec-v3) | MAY | Key | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.3-3` | ESP/AH: HMAC-MD5-96 may be implemented (§5.3, MAY for IPsec-v3) | MAY | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.2-1` | ESP: AES-GCM combined-mode may be implemented (§5.2) | MAY | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.2-2` | ESP: AES-CCM combined-mode may be implemented (§5.2) | MAY | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.5-5` | IKEv2: ECP groups 19-21 are optional (§5.5) | MAY | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6071-5.3-4` | IKEv2: HMAC-SHA-256/384/512 are optional (§5.3, §5.4) | MAY | 5.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC6071-5.1-1`](#rfc6071-5.1-1) ESP: NULL encryption must be implemented (§5.1, IPsec-v3) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an ESP algorithm-implementation requirement owned by RFC 4835 (NULL encryption per RFC 2410), which governs ze's ESP dataplane (internal/component/ike/dataplane/xfrm_linux.go) |
| [`RFC6071-5.1-2`](#rfc6071-5.1-2) ESP: AES-CBC-128 encryption must be implemented (§5.1, IPsec-v3) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an ESP algorithm-implementation requirement owned by RFC 4835 (AES-CBC-128 per RFC 3602), which governs ze's ESP dataplane |
| [`RFC6071-5.3-1`](#rfc6071-5.3-1) ESP/AH: HMAC-SHA-1-96 integrity must be implemented (§5.3, IPsec-v3 and IKEv2) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an ESP/AH and IKEv2 integrity requirement owned by RFC 4835 and RFC 4307 (HMAC-SHA-1-96 per RFC 2404), which governs ze's ESP and IKE code |
| [`RFC6071-5.4-1`](#rfc6071-5.4-1) IKEv2: PRF-HMAC-SHA-1 must be implemented (§5.4, IKEv2) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an IKEv2 PRF requirement owned by RFC 4307, which governs ze's IKEv2 transform negotiation (internal/component/ike/crypto/transform.go) |
| [`RFC6071-5.5-1`](#rfc6071-5.5-1) IKEv1: MODP group 2 (1024-bit) must be supported (§5.5, IKEv1, deprecated) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an IKEv1 algorithm requirement owned by RFC 4109, and ze implements IKEv2 only (no IKEv1 code path) |
| [`RFC6071-5.1-3`](#rfc6071-5.1-3) IKEv2: 3DES-CBC must be supported (§5.1, MUST- / deprecated but mandatory) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an IKEv2 3DES-CBC requirement owned by RFC 4307, which governs ze's IKEv2 transform negotiation |
| [`RFC6071-5.5-2`](#rfc6071-5.5-2) IKEv2: MODP group 2 (1024-bit) must be supported (§5.5, MUST- / deprecated but mandatory) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an IKEv2 MODP-1024 group requirement owned by RFC 4307, which governs ze's IKEv2 group negotiation |
| [`RFC6071-5.1-4`](#rfc6071-5.1-4) ESP: 3DES-CBC must be supported (§5.1, MUST- / deprecated but mandatory) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 6071 is an informational IPsec/IKE document roadmap that catalogs other specifications and defines no independent protocol behavior; this restates an ESP 3DES-CBC requirement owned by RFC 4835, which governs ze's ESP dataplane |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6071-5.1-1`](#rfc6071-5.1-1)

ESP: NULL encryption must be implemented (§5.1, IPsec-v3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6071-5.1-1, so no unit is bound to it.

### [`RFC6071-5.1-2`](#rfc6071-5.1-2)

ESP: AES-CBC-128 encryption must be implemented (§5.1, IPsec-v3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6071-5.1-2, so no unit is bound to it.

### [`RFC6071-5.3-1`](#rfc6071-5.3-1)

ESP/AH: HMAC-SHA-1-96 integrity must be implemented (§5.3, IPsec-v3 and IKEv2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6071-5.3-1, so no unit is bound to it.

### [`RFC6071-5.4-1`](#rfc6071-5.4-1)

IKEv2: PRF-HMAC-SHA-1 must be implemented (§5.4, IKEv2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6071-5.4-1, so no unit is bound to it.

### [`RFC6071-5.5-1`](#rfc6071-5.5-1)

IKEv1: MODP group 2 (1024-bit) must be supported (§5.5, IKEv1, deprecated)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6071-5.5-1, so no unit is bound to it.

### [`RFC6071-5.1-3`](#rfc6071-5.1-3)

IKEv2: 3DES-CBC must be supported (§5.1, MUST- / deprecated but mandatory)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6071-5.1-3, so no unit is bound to it.

### [`RFC6071-5.5-2`](#rfc6071-5.5-2)

IKEv2: MODP group 2 (1024-bit) must be supported (§5.5, MUST- / deprecated but mandatory)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6071-5.5-2, so no unit is bound to it.

### [`RFC6071-5.1-4`](#rfc6071-5.1-4)

ESP: 3DES-CBC must be supported (§5.1, MUST- / deprecated but mandatory)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6071-5.1-4, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 6071, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 6071, so its obligations are stated where they were written.
