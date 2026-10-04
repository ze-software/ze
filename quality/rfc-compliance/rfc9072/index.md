# RFC 9072 - Extended Optional Parameters Length for BGP OPEN Message

Partial. Every requirement this repository extracted from RFC 9072, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 33.3% | 3 of 9 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 22.2% | 2 of 9 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 9 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 72.7% | 8 of 11 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 12 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 9 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 9 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 9 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 44.4% | 4 of 9 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 5 | of 9 gated MUSTs judged | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 9 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 12 |
| Gated MUST-level | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 12 |
| Tagged units | 11 |
| Recorded audit verdicts | 5 |
| Discrimination records | 8 |
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
| Positive and negative tests | 3 | one part of the gated population |
| Annotated (including scoped evidence) | 6 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC9072-2-1`](#rfc9072-2-1), [`RFC9072-2-2`](#rfc9072-2-2), [`RFC9072-3-2`](#rfc9072-3-2)

**Annotated (including scoped evidence) (6):** [`RFC9072-2-3`](#rfc9072-2-3), [`RFC9072-2-4`](#rfc9072-2-4), [`RFC9072-2-5`](#rfc9072-2-5), [`RFC9072-2-6`](#rfc9072-2-6), [`RFC9072-3-1`](#rfc9072-3-1), [`RFC9072-3-3`](#rfc9072-3-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9072-2-1` | However, if the length of the Optional Parameters in the BGP OPEN message does exceed 255, the OPEN message MUST be encoded according to the procedure below. (S2) | MUST | 2 | **positive:** `unit/verify` [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L419). **negative:** `unit/verify` [`TestTheExtendedEnvelopeAndItsParametersAgree`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9072_extended_open_test.go#L21) |
| `RFC9072-2-2` | (In any case, an implementation MUST accept an OPEN message that uses the encoding of this specification even if the length of the Optional Parameters is 255 or less.) (S2) | MUST | 2 | **positive:** `unit/verify` [`TestOpenUnpackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L208). **positive:** `unit/verify` [`TestRFC9072ShortExtendedOpenIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9072_short_extended_open_test.go#L32). **negative:** `unit/verify` [`TestOpenUnpackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L195). **negative:** `unit/verify` [`TestRFC9072ShortExtendedOpenIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9072_short_extended_open_test.go#L34) |
| `RFC9072-2-3` | The Non-Extended Optional Parameters Length field (Non-Ext OP Len.) SHOULD be set to 255 on transmission and, in any event, MUST NOT be set to 0 (S2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L420). **negative:** no negative test. **{single-polarity}:** writeToExtended sets the Non-Ext OP Len to the constant 0xFF marker (open.go:130), so it is structurally never 0 and no code path can produce the negative case |
| `RFC9072-2-4` | The subsequent one-octet field (which would be the first Optional Parameter Type field in the non-extended format and is called "Non- Ext OP Type" in the figure above) MUST be set to 255 on transmission. (S2) | MUST | 2 | **positive:** `unit/verify` [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L422). **negative:** no negative test. **{single-polarity}:** writeToExtended sets the Non-Ext OP Type to the constant 0xFF (open.go:131), so no code path emits any other value and there is no negative case |
| `RFC9072-2-5` | The Non-Extended Optional Parameters Length field (Non-Ext OP Len.) SHOULD be set to 255 on transmission and, in any event, MUST NOT be set to 0; it MUST be ignored on receipt once the use of the extended format is determined positively by inspection of the Non-Extended Optional Parameters Type (Non-Ext OP Type) field. (S2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's OPEN decoder (internal/component/bgp/message/open.go:190) requires the Non-Ext OP Len octet to equal 255 to select the extended form, so it does not ignore that octet once the extended format would be determined; the octet stays load-bearing and a conformant sender using a non-255 Non-Ext OP Len is mis-parsed as a classic OPEN |
| `RFC9072-2-6` | In parsing an OPEN message, if the one-octet Optional Parameters Length field (labeled "Non-Ext OP Len." in Figure 1) is non-zero, a BGP speaker MUST use the value of the octet following the one-octet Optional Parameters Length field (labeled "Non-Ext OP Type" in Figure 1) to determine both the encoding of the Optional Parameters length and the size of the Parameter Length field of individual Optional Parameters. (S2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's OPEN decoder (internal/component/bgp/message/open.go:190) inspects the octet following Non-Ext OP Len only when Non-Ext OP Len equals 255, so for any other non-zero Non-Ext OP Len it never uses the following octet to determine the encoding and always decodes the classic form |
| `RFC9072-3-1` | It is not considered a fatal error to receive an OPEN message whose (non-extended) Optional Parameters Length value is not 255 and whose first Optional Parameter type code is 255 -- in this case, the encoding of this specification MUST be used for decoding the message. (S3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's OPEN decoder (internal/component/bgp/message/open.go:190) selects the extended form only when Non-Ext OP Len equals 255, not whenever the first type code is 255; the open.go:186-189 comment and the TestOpenUnpackExtendedParams standard-format-first-param-byte-0xFF case cement this, so a first type code of 255 with Non-Ext OP Len != 255 is decoded as a classic OPEN instead of extended |
| `RFC9072-3-2` | Although the Optional Parameter type code 255 is used in this specification as the indication that the extended encoding is in use, it is not a bona fide Optional Parameter type code in the usual sense and MUST NOT be used other than as described above. (S3) | MUST NOT | 3 | **positive:** `unit/verify` [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L423). **positive:** `unit/verify` [`TestRFC9072ParameterType255OnlyAsTheExtendedMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9072_param_type_test.go#L57). **negative:** `unit/verify` [`TestRFC9072ParameterType255OnlyAsTheExtendedMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9072_param_type_test.go#L59) |
| `RFC9072-3-3` | Although the Optional Parameter type code 255 is used in this specification as the indication that the extended encoding is in use, it is not a bona fide Optional Parameter type code in the usual sense and MUST NOT be used other than as described above. If encountered other than as the Non-Ext OP Type, it MUST be treated as an unrecognized Optional Parameter and handled according to [RFC4271], Section 6.2. (S3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze silently ignores an unrecognized BGP OPEN optional-parameter type; ParseFromOptionalParams (internal/core/bgp/capability/capability.go:867-874) skips any parameter whose type is not 2 instead of emitting the RFC 4271 Section 6.2 OPEN Message Error (Unsupported Optional Parameter) NOTIFICATION |
| `RFC9072-2-7` | In the event that the length of the Optional Parameters in the BGP OPEN message does not exceed 255, the encodings of the base BGP specification [RFC4271] SHOULD be used without alteration. (S2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9072-2-8` | The Non-Extended Optional Parameters Length field (Non-Ext OP Len.) SHOULD be set to 255 on transmission (S2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9072-2-9` | Configuration MAY override this to force the extended format to be used in all cases (S2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9072-2-5`](#rfc9072-2-5) The Non-Extended Optional Parameters Length field (Non-Ext OP Len.) SHOULD be set to 255 on transmission and, in any event, MUST NOT be set to 0; it MUST be ignored on receipt once the use of the extended format is determined positively by inspection of the Non-Extended Optional Parameters Type (Non-Ext OP Type) field. (S2) | {gap}, no test | ze's OPEN decoder (internal/component/bgp/message/open.go:190) requires the Non-Ext OP Len octet to equal 255 to select the extended form, so it does not ignore that octet once the extended format would be determined; the octet stays load-bearing and a conformant sender using a non-255 Non-Ext OP Len is mis-parsed as a classic OPEN |
| [`RFC9072-2-6`](#rfc9072-2-6) In parsing an OPEN message, if the one-octet Optional Parameters Length field (labeled "Non-Ext OP Len." in Figure 1) is non-zero, a BGP speaker MUST use the value of the octet following the one-octet Optional Parameters Length field (labeled "Non-Ext OP Type" in Figure 1) to determine both the encoding of the Optional Parameters length and the size of the Parameter Length field of individual Optional Parameters. (S2) | {gap}, no test | ze's OPEN decoder (internal/component/bgp/message/open.go:190) inspects the octet following Non-Ext OP Len only when Non-Ext OP Len equals 255, so for any other non-zero Non-Ext OP Len it never uses the following octet to determine the encoding and always decodes the classic form |
| [`RFC9072-3-1`](#rfc9072-3-1) It is not considered a fatal error to receive an OPEN message whose (non-extended) Optional Parameters Length value is not 255 and whose first Optional Parameter type code is 255 -- in this case, the encoding of this specification MUST be used for decoding the message. (S3) | {gap}, no test | ze's OPEN decoder (internal/component/bgp/message/open.go:190) selects the extended form only when Non-Ext OP Len equals 255, not whenever the first type code is 255; the open.go:186-189 comment and the TestOpenUnpackExtendedParams standard-format-first-param-byte-0xFF case cement this, so a first type code of 255 with Non-Ext OP Len != 255 is decoded as a classic OPEN instead of extended |
| [`RFC9072-3-3`](#rfc9072-3-3) Although the Optional Parameter type code 255 is used in this specification as the indication that the extended encoding is in use, it is not a bona fide Optional Parameter type code in the usual sense and MUST NOT be used other than as described above. If encountered other than as the Non-Ext OP Type, it MUST be treated as an unrecognized Optional Parameter and handled according to [RFC4271], Section 6.2. (S3) | {gap}, no test | ze silently ignores an unrecognized BGP OPEN optional-parameter type; ParseFromOptionalParams (internal/core/bgp/capability/capability.go:867-874) skips any parameter whose type is not 2 instead of emitting the RFC 4271 Section 6.2 OPEN Message Error (Unsupported Optional Parameter) NOTIFICATION |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9072-2-1`](#rfc9072-2-1)

However, if the length of the Optional Parameters in the BGP OPEN message does exceed 255, the OPEN message MUST be encoded according to the procedure below. (S2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Unit changed only by D-15: TestOpenPackExtendedParams now asserts the exact extended body (13+params, the off-by-one D-8 fix) plus the markers and 2-octet length. Still weak: the procedure's two-octet Parameter Length per parameter is not asserted in this row's units (the reactor TestRFC9072ParameterType255OnlyAsTheExtendedMarker parses it but is tagged to 3-2 only). Row is in the child spec's Blocked-by list.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTheExtendedEnvelopeAndItsParametersAgree`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9072_extended_open_test.go#L21) | unit/verify | unproven |
| positive | [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L419) | unit/verify | revert, verified |

### [`RFC9072-2-2`](#rfc9072-2-2)

(In any case, an implementation MUST accept an OPEN message that uses the encoding of this specification even if the length of the Optional Parameters is 255 or less.) (S2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC9072ShortExtendedOpenIsAccepted: an extended OPEN with 9 parameter octets decodes with no error, ExtendedParams true, parameters exact, ASN4 4200000001 parsed; a classic reading of the same octets does not recover it; the classic OPEN of the same capability decodes ExtendedParams false to the same capability.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpenUnpackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L195) | unit/verify | unproven |
| negative | [`TestRFC9072ShortExtendedOpenIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9072_short_extended_open_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestOpenUnpackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L208) | unit/verify | unproven |
| positive | [`TestRFC9072ShortExtendedOpenIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9072_short_extended_open_test.go#L32) | unit/verify | revert, verified |

### [`RFC9072-2-3`](#rfc9072-2-3)

The Non-Extended Optional Parameters Length field (Non-Ext OP Len.) SHOULD be set to 255 on transmission and, in any event, MUST NOT be set to 0 (S2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. {single-polarity: positive} on the row. TestOpenPackExtendedParams requires body[9]==0xFF and now the exact body length; writeToExtended writes the constant.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L420) | unit/verify | revert, verified |

### [`RFC9072-2-4`](#rfc9072-2-4)

The subsequent one-octet field (which would be the first Optional Parameter Type field in the non-extended format and is called "Non- Ext OP Type" in the figure above) MUST be set to 255 on transmission. (S2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. {single-polarity: positive} on the row. TestOpenPackExtendedParams requires body[10]==0xFF; writeToExtended writes the constant.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L422) | unit/verify | revert, verified |

### [`RFC9072-2-5`](#rfc9072-2-5)

The Non-Extended Optional Parameters Length field (Non-Ext OP Len.) SHOULD be set to 255 on transmission and, in any event, MUST NOT be set to 0; it MUST be ignored on receipt once the use of the extended format is determined positively by inspection of the Non-Extended Optional Parameters Type (Non-Ext OP Type) field. (S2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9072-2-5, so no unit is bound to it.

### [`RFC9072-2-6`](#rfc9072-2-6)

In parsing an OPEN message, if the one-octet Optional Parameters Length field (labeled "Non-Ext OP Len." in Figure 1) is non-zero, a BGP speaker MUST use the value of the octet following the one-octet Optional Parameters Length field (labeled "Non-Ext OP Type" in Figure 1) to determine both the encoding of the Optional Parameters length and the size of the Parameter Length field of individual Optional Parameters. (S2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9072-2-6, so no unit is bound to it.

### [`RFC9072-3-1`](#rfc9072-3-1)

It is not considered a fatal error to receive an OPEN message whose (non-extended) Optional Parameters Length value is not 255 and whose first Optional Parameter type code is 255 -- in this case, the encoding of this specification MUST be used for decoding the message. (S3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9072-3-1, so no unit is bound to it.

### [`RFC9072-3-2`](#rfc9072-3-2)

Although the Optional Parameter type code 255 is used in this specification as the indication that the extended encoding is in use, it is not a bona fide Optional Parameter type code in the usual sense and MUST NOT be used other than as described above. (S3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Marker removed; genuine negative (R1b): TestRFC9072ParameterType255OnlyAsTheExtendedMarker builds capabilities full of 0xFF (AS 0xFFFFFFFF, AFI 0xFFFF, SAFI 0xFF) through buildOptionalParams and requires no parameter type 255 and framing ending exactly at the end, classic below 256 octets, extended above; positive body[10]==0xFF on TestOpenPackExtendedParams.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9072ParameterType255OnlyAsTheExtendedMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9072_param_type_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L423) | unit/verify | revert, verified |
| positive | [`TestRFC9072ParameterType255OnlyAsTheExtendedMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9072_param_type_test.go#L57) | unit/verify | revert, verified |

### [`RFC9072-3-3`](#rfc9072-3-3)

Although the Optional Parameter type code 255 is used in this specification as the indication that the extended encoding is in use, it is not a bona fide Optional Parameter type code in the usual sense and MUST NOT be used other than as described above. If encountered other than as the Non-Ext OP Type, it MUST be treated as an unrecognized Optional Parameter and handled according to [RFC4271], Section 6.2. (S3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9072-3-3, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc9072.txt |
| Source fingerprint | c40d3ab7b84b736b |
| Record | rfc/extraction/rfc9072.json |
| Mapped sentences | 8 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 5 | walked | not stated |
| `3` | not stated | 3 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust copyright boilerplate in the Status of This Memo section: it binds the extraction of code components from the document, not any BGP speaker behavior, and the keyword is lowercase 'must'. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |

## Superseded

No document obsoletes RFC 9072, so its obligations are stated where they were written.
