# RFC 9072 - Extended Optional Parameters Length for BGP OPEN Message

Partial. Every requirement this repository extracted from RFC 9072, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 22.2% | 2 of 9 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 33.3% | 3 of 9 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 7 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 12 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 44.4% | 4 of 9 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 9 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 12 |
| Gated MUST-level | 9 |
| Obligations that bind Ze | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tagged units | 7 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9072.md` |
| Requirement shard | `rfc/requirements/rfc9072.md` |
| RFC text | `rfc/full/rfc9072.txt` |

## Enrolment

Enrolled: Extended Optional Parameters Length for BGP OPEN: nine MUST-level requirements. Five are met: 2-2 (a receiver parses the extended form) carries positive+negative tags; 2-1 (use the extended encoding when Optional Parameters exceed 255 octets), 2-3 (the Non-Ext OP marker is non-zero), 2-4 (the Non-Ext OP Type is 255), and 3-2 (the classic form emits only optional-parameter type 2) are {single-polarity: positive}. Four are {gap}: 2-5, 2-6, and 3-1 -- ze's OPEN decoder detects the extended form only when the Non-Ext OP Len octet equals 255, rather than treating any non-zero Non-Ext OP Len with a following type octet of 255 as extended and ignoring the length value; and 3-3 -- an unrecognized OPEN optional-parameter type is silently skipped rather than triggering the RFC 4271 Section 6.2 NOTIFICATION. Disclosed in the docs/features/rfc-status.md RFC 9072 row.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Extended OPEN encoding when Optional Parameters exceed 255 octets (Non-Ext OP Len/Type 0xFF markers plus 2-octet Extended Opt. Parm. Length), extended-form decode, and classic-form encode/decode
- tests bound per requirement in [`rfc/requirements/rfc9072.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc9072.md).


**What the ledger says remains**

Four MUST-level gaps, each annotated in [`rfc/short/rfc9072.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9072.md): [`RFC9072-2-5`](#rfc9072-2-5), [`RFC9072-2-6`](#rfc9072-2-6) and [`RFC9072-3-1`](#rfc9072-3-1) -- the decoder ([`internal/component/bgp/message/open.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open.go)) selects the extended form only when Non-Ext OP Len equals 255, so it does not ignore that octet, does not consult the following octet for other non-zero lengths, and mis-parses a first type code of 255 with Non-Ext OP Len not 255 as a classic OPEN; and [`RFC9072-3-3`](#rfc9072-3-3) -- an unrecognized OPEN optional-parameter type is silently skipped ([`internal/core/bgp/capability/capability.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability.go)) instead of triggering the RFC 4271 Section 6.2 Unsupported Optional Parameter NOTIFICATION.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC9072-2-1`](#rfc9072-2-1), [`RFC9072-2-2`](#rfc9072-2-2)

**Annotated instead of tested (7):** [`RFC9072-2-3`](#rfc9072-2-3), [`RFC9072-2-4`](#rfc9072-2-4), [`RFC9072-2-5`](#rfc9072-2-5), [`RFC9072-2-6`](#rfc9072-2-6), [`RFC9072-3-1`](#rfc9072-3-1), [`RFC9072-3-2`](#rfc9072-3-2), [`RFC9072-3-3`](#rfc9072-3-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9072-2-1` | If Optional Parameters length exceeds 255, the OPEN message MUST be encoded using the extended procedure (S2) | MUST | 2 | **positive:** `unit/verify` [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L419). **negative:** `unit/verify` [`TestTheExtendedEnvelopeAndItsParametersAgree`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9072_extended_open_test.go#L21) |
| `RFC9072-2-2` | An implementation MUST accept an OPEN message using extended encoding even if Optional Parameters length is 255 or less (S2) | MUST | 2 | **positive:** `unit/verify` [`TestOpenUnpackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L208). **negative:** `unit/verify` [`TestOpenUnpackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L195) |
| `RFC9072-2-3` | Non-Ext OP Len MUST NOT be set to 0 when using extended format (S2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L420). **negative:** no negative test. **{single-polarity}:** writeToExtended sets the Non-Ext OP Len to the constant 0xFF marker (open.go:130), so it is structurally never 0 and no code path can produce the negative case |
| `RFC9072-2-4` | Non-Ext OP Type MUST be set to 255 on transmission (S2) | MUST | 2 | **positive:** `unit/verify` [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L422). **negative:** no negative test. **{single-polarity}:** writeToExtended sets the Non-Ext OP Type to the constant 0xFF (open.go:131), so no code path emits any other value and there is no negative case |
| `RFC9072-2-5` | Non-Ext OP Len MUST be ignored on receipt once extended format is determined (S2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's OPEN decoder (internal/component/bgp/message/open.go:190) requires the Non-Ext OP Len octet to equal 255 to select the extended form, so it does not ignore that octet once the extended format would be determined; the octet stays load-bearing and a conformant sender using a non-255 Non-Ext OP Len is mis-parsed as a classic OPEN |
| `RFC9072-2-6` | If Non-Ext OP Len is non-zero, BGP speaker MUST use value of following octet to determine encoding format (S2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's OPEN decoder (internal/component/bgp/message/open.go:190) inspects the octet following Non-Ext OP Len only when Non-Ext OP Len equals 255, so for any other non-zero Non-Ext OP Len it never uses the following octet to determine the encoding and always decodes the classic form |
| `RFC9072-3-1` | If first type code is 255 (even when Non-Ext OP Len != 255), extended encoding MUST be used for decoding (S3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's OPEN decoder (internal/component/bgp/message/open.go:190) selects the extended form only when Non-Ext OP Len equals 255, not whenever the first type code is 255; the open.go:186-189 comment and the TestOpenUnpackExtendedParams standard-format-first-param-byte-0xFF case cement this, so a first type code of 255 with Non-Ext OP Len != 255 is decoded as a classic OPEN instead of extended |
| `RFC9072-3-2` | Type code 255 MUST NOT be used other than as the extended length indicator (S3) | MUST NOT | 3 | **positive:** `unit/verify` [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L423). **negative:** no negative test. **{single-polarity}:** ze's OPEN encoder emits only optional-parameter type 2 (Capabilities) via buildOptionalParams (internal/component/bgp/reactor/session_negotiate.go:193) and uses 255 solely as the extended-length indicator; no code path emits any other classic opt-param type, so there is no negative case |
| `RFC9072-3-3` | Type code 255 in non-indicator position MUST be treated as unrecognized Optional Parameter per RFC 4271 S6.2 (S3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze silently ignores an unrecognized BGP OPEN optional-parameter type; ParseFromOptionalParams (internal/core/bgp/capability/capability.go:867-874) skips any parameter whose type is not 2 instead of emitting the RFC 4271 Section 6.2 OPEN Message Error (Unsupported Optional Parameter) NOTIFICATION |
| `RFC9072-2-7` | When Optional Parameters length does not exceed 255, standard RFC 4271 encoding SHOULD be used (S2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9072-2-8` | Non-Ext OP Len SHOULD be set to 255 on transmission (S2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9072-2-9` | Configuration MAY override to force extended format in all cases (S2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9072-2-5`](#rfc9072-2-5) Non-Ext OP Len MUST be ignored on receipt once extended format is determined (S2) | {gap}, no test | ze's OPEN decoder (internal/component/bgp/message/open.go:190) requires the Non-Ext OP Len octet to equal 255 to select the extended form, so it does not ignore that octet once the extended format would be determined; the octet stays load-bearing and a conformant sender using a non-255 Non-Ext OP Len is mis-parsed as a classic OPEN |
| [`RFC9072-2-6`](#rfc9072-2-6) If Non-Ext OP Len is non-zero, BGP speaker MUST use value of following octet to determine encoding format (S2) | {gap}, no test | ze's OPEN decoder (internal/component/bgp/message/open.go:190) inspects the octet following Non-Ext OP Len only when Non-Ext OP Len equals 255, so for any other non-zero Non-Ext OP Len it never uses the following octet to determine the encoding and always decodes the classic form |
| [`RFC9072-3-1`](#rfc9072-3-1) If first type code is 255 (even when Non-Ext OP Len != 255), extended encoding MUST be used for decoding (S3) | {gap}, no test | ze's OPEN decoder (internal/component/bgp/message/open.go:190) selects the extended form only when Non-Ext OP Len equals 255, not whenever the first type code is 255; the open.go:186-189 comment and the TestOpenUnpackExtendedParams standard-format-first-param-byte-0xFF case cement this, so a first type code of 255 with Non-Ext OP Len != 255 is decoded as a classic OPEN instead of extended |
| [`RFC9072-3-3`](#rfc9072-3-3) Type code 255 in non-indicator position MUST be treated as unrecognized Optional Parameter per RFC 4271 S6.2 (S3) | {gap}, no test | ze silently ignores an unrecognized BGP OPEN optional-parameter type; ParseFromOptionalParams (internal/core/bgp/capability/capability.go:867-874) skips any parameter whose type is not 2 instead of emitting the RFC 4271 Section 6.2 OPEN Message Error (Unsupported Optional Parameter) NOTIFICATION |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9072-2-1`](#rfc9072-2-1)

If Optional Parameters length exceeds 255, the OPEN message MUST be encoded using the extended procedure (S2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTheExtendedEnvelopeAndItsParametersAgree`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9072_extended_open_test.go#L21) | unit/verify | unproven |
| positive | [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L419) | unit/verify | unproven |

### [`RFC9072-2-2`](#rfc9072-2-2)

An implementation MUST accept an OPEN message using extended encoding even if Optional Parameters length is 255 or less (S2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpenUnpackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L195) | unit/verify | unproven |
| positive | [`TestOpenUnpackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L208) | unit/verify | unproven |

### [`RFC9072-2-3`](#rfc9072-2-3)

Non-Ext OP Len MUST NOT be set to 0 when using extended format (S2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L420) | unit/verify | unproven |

### [`RFC9072-2-4`](#rfc9072-2-4)

Non-Ext OP Type MUST be set to 255 on transmission (S2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L422) | unit/verify | unproven |

### [`RFC9072-2-5`](#rfc9072-2-5)

Non-Ext OP Len MUST be ignored on receipt once extended format is determined (S2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9072-2-5, so no unit is bound to it.

### [`RFC9072-2-6`](#rfc9072-2-6)

If Non-Ext OP Len is non-zero, BGP speaker MUST use value of following octet to determine encoding format (S2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9072-2-6, so no unit is bound to it.

### [`RFC9072-3-1`](#rfc9072-3-1)

If first type code is 255 (even when Non-Ext OP Len != 255), extended encoding MUST be used for decoding (S3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9072-3-1, so no unit is bound to it.

### [`RFC9072-3-2`](#rfc9072-3-2)

Type code 255 MUST NOT be used other than as the extended length indicator (S3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L423) | unit/verify | unproven |

### [`RFC9072-3-3`](#rfc9072-3-3)

Type code 255 in non-indicator position MUST be treated as unrecognized Optional Parameter per RFC 4271 S6.2 (S3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9072-3-3, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 9072, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 9072, so its obligations are stated where they were written.
