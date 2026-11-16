# RFC 9072 - Extended Optional Parameters Length for BGP OPEN Message

Extended Optional Parameters Length for BGP OPEN. Every requirement listed for RFC 9072, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 3 | of 9 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 2 | of 9 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 9 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 9 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 12 | 12 of 14 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 4 | of 9 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 9 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 9 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 9 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 9 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 9 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 9 | of 12 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 9 mandatory requirements: they add up to 9. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | Extended Optional Parameters Length for BGP OPEN |
| Automatic checks | Included |
| Requirements | 12 |
| Mandatory requirements (MUST level) | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 15 |
| Tests linked to requirements | 14 |
| Recorded audit verdicts | 5 |
| Fault test records | 12 |
| Summary | `rfc/short/rfc9072.md` |
| Requirement table | `rfc/requirements/rfc9072.md` |
| RFC text | `rfc/full/rfc9072.txt` |

## Automatic checks

Included in automatic checks: Extended Optional Parameters Length for BGP OPEN: nine MUST-level requirements. Five are met: 2-2 (a receiver parses the extended form) carries positive+negative tags; 2-1 (use the extended encoding when Optional Parameters exceed 255 octets), 2-3 (the Non-Ext OP marker is non-zero), 2-4 (the Non-Ext OP Type is 255), and 3-2 (the classic form emits only optional-parameter type 2) are {single-polarity: positive}. Four are {gap}: 2-5, 2-6, and 3-1 -- ze's OPEN decoder detects the extended form only when the Non-Ext OP Len octet equals 255, rather than treating any non-zero Non-Ext OP Len with a following type octet of 255 as extended and ignoring the length value; and 3-3 -- an unrecognized OPEN optional-parameter type is silently skipped rather than triggering the RFC 4271 Section 6.2 NOTIFICATION. Disclosed in the docs/features/rfc-status.md RFC 9072 row.

## Published support status

**Area:** Extended Optional Parameters Length for BGP OPEN

**Test verification:** 5 of 9 MUST proven

**Evidence:** unit 5

**Unmet:** RFC9072-2-5, RFC9072-2-6, RFC9072-3-1, RFC9072-3-3


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 6 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **9** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (3):** [`RFC9072-2-1`](#rfc9072-2-1), [`RFC9072-2-2`](#rfc9072-2-2), [`RFC9072-3-2`](#rfc9072-3-2)

**Annotated (including scoped evidence) (6):** [`RFC9072-2-3`](#rfc9072-2-3), [`RFC9072-2-4`](#rfc9072-2-4), [`RFC9072-2-5`](#rfc9072-2-5), [`RFC9072-2-6`](#rfc9072-2-6), [`RFC9072-3-1`](#rfc9072-3-1), [`RFC9072-3-3`](#rfc9072-3-3)

## Figures

- **MUST bar**: Ze verified 5, Not built 4
- **Overview bar**: Ze MUST verified 5, SHOULD verified 1, Not assessed 2, Not built 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 9 |
| MUST rows kept (the MUST bar) | 9 |
| Ze verified | 5 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 4 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 3 |
| SHOULD verified | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 2 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc9072.json](../../../data/rfc/rfc9072.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9072-2-1` | However, if the length of the Optional Parameters in the BGP OPEN message does exceed 255, the OPEN message MUST be encoded according to the procedure below. (S2) | MUST | 2 | **positive:** `unit/verify` [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L419). **positive:** `unit/verify` [`TestRFC9072OptionalParamsBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9072_optional_params_boundary_test.go#L64). **negative:** `unit/verify` [`TestRFC9072OptionalParamsBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9072_optional_params_boundary_test.go#L67). **negative:** `unit/verify` [`TestTheExtendedEnvelopeAndItsParametersAgree`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9072_extended_open_test.go#L40) |
| `RFC9072-2-2` | (In any case, an implementation MUST accept an OPEN message that uses the encoding of this specification even if the length of the Optional Parameters is 255 or less.) (S2) | MUST | 2 | **positive:** `unit/verify` [`TestOpenUnpackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L208). **positive:** `unit/verify` [`TestRFC9072ShortExtendedOpenIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9072_short_extended_open_test.go#L32). **negative:** `unit/verify` [`TestOpenUnpackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L195). **negative:** `unit/verify` [`TestRFC9072ShortExtendedOpenIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9072_short_extended_open_test.go#L34) |
| `RFC9072-2-3` | The Non-Extended Optional Parameters Length field (Non-Ext OP Len.) SHOULD be set to 255 on transmission and, in any event, MUST NOT be set to 0 (S2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L420). **negative:** no negative test. **{single-polarity}:** writeToExtended sets the Non-Ext OP Len to the constant 0xFF marker (open.go:130), so it is structurally never 0 and no code path can produce the negative case |
| `RFC9072-2-4` | The subsequent one-octet field (which would be the first Optional Parameter Type field in the non-extended format and is called "Non- Ext OP Type" in the figure above) MUST be set to 255 on transmission. (S2) | MUST | 2 | **positive:** `unit/verify` [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L422). **negative:** no negative test. **{single-polarity}:** writeToExtended sets the Non-Ext OP Type to the constant 0xFF (open.go:131), so no code path emits any other value and there is no negative case |
| `RFC9072-2-5` | The Non-Extended Optional Parameters Length field (Non-Ext OP Len.) SHOULD be set to 255 on transmission and, in any event, MUST NOT be set to 0; it MUST be ignored on receipt once the use of the extended format is determined positively by inspection of the Non-Extended Optional Parameters Type (Non-Ext OP Type) field. (S2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's OPEN decoder (internal/component/bgp/message/open.go:190) requires the Non-Ext OP Len octet to equal 255 to select the extended form, so it does not ignore that octet once the extended format would be determined; the octet stays load-bearing and a conformant sender using a non-255 Non-Ext OP Len is mis-parsed as a classic OPEN |
| `RFC9072-2-6` | In parsing an OPEN message, if the one-octet Optional Parameters Length field (labeled "Non-Ext OP Len." in Figure 1) is non-zero, a BGP speaker MUST use the value of the octet following the one-octet Optional Parameters Length field (labeled "Non-Ext OP Type" in Figure 1) to determine both the encoding of the Optional Parameters length and the size of the Parameter Length field of individual Optional Parameters. (S2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's OPEN decoder (internal/component/bgp/message/open.go:190) inspects the octet following Non-Ext OP Len only when Non-Ext OP Len equals 255, so for any other non-zero Non-Ext OP Len it never uses the following octet to determine the encoding and always decodes the classic form |
| `RFC9072-3-1` | It is not considered a fatal error to receive an OPEN message whose (non-extended) Optional Parameters Length value is not 255 and whose first Optional Parameter type code is 255 -- in this case, the encoding of this specification MUST be used for decoding the message. (S3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's OPEN decoder (internal/component/bgp/message/open.go:190) selects the extended form only when Non-Ext OP Len equals 255, not whenever the first type code is 255; the open.go:186-189 comment and the TestOpenUnpackExtendedParams standard-format-first-param-byte-0xFF case cement this, so a first type code of 255 with Non-Ext OP Len != 255 is decoded as a classic OPEN instead of extended |
| `RFC9072-3-2` | Although the Optional Parameter type code 255 is used in this specification as the indication that the extended encoding is in use, it is not a bona fide Optional Parameter type code in the usual sense and MUST NOT be used other than as described above. (S3) | MUST NOT | 3 | **positive:** `unit/verify` [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L423). **positive:** `unit/verify` [`TestRFC9072ParameterType255OnlyAsTheExtendedMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9072_param_type_test.go#L57). **negative:** `unit/verify` [`TestRFC9072ParameterType255OnlyAsTheExtendedMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9072_param_type_test.go#L59) |
| `RFC9072-3-3` | Although the Optional Parameter type code 255 is used in this specification as the indication that the extended encoding is in use, it is not a bona fide Optional Parameter type code in the usual sense and MUST NOT be used other than as described above. If encountered other than as the Non-Ext OP Type, it MUST be treated as an unrecognized Optional Parameter and handled according to [RFC4271], Section 6.2. (S3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze silently ignores an unrecognized BGP OPEN optional-parameter type; ParseFromOptionalParams (internal/core/bgp/capability/capability.go:867-874) skips any parameter whose type is not 2 instead of emitting the RFC 4271 Section 6.2 OPEN Message Error (Unsupported Optional Parameter) NOTIFICATION |
| `RFC9072-2-7` | In the event that the length of the Optional Parameters in the BGP OPEN message does not exceed 255, the encodings of the base BGP specification [RFC4271] SHOULD be used without alteration. (S2) | SHOULD | 2 | **positive:** `unit/verify` [`TestRFC9072OptionalParamsBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9072_optional_params_boundary_test.go#L72). **negative:** no negative test |
| `RFC9072-2-8` | The Non-Extended Optional Parameters Length field (Non-Ext OP Len.) SHOULD be set to 255 on transmission (S2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9072-2-9` | Configuration MAY override this to force the extended format to be used in all cases (S2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9072-2-5`](#rfc9072-2-5) The Non-Extended Optional Parameters Length field (Non-Ext OP Len.) SHOULD be set to 255 on transmission and, in any event, MUST NOT be set to 0; it MUST be ignored on receipt once the use of the extended format is determined positively by inspection of the Non-Extended Optional Parameters Type (Non-Ext OP Type) field. (S2) | {gap}, no test | ze's OPEN decoder (internal/component/bgp/message/open.go:190) requires the Non-Ext OP Len octet to equal 255 to select the extended form, so it does not ignore that octet once the extended format would be determined; the octet stays load-bearing and a conformant sender using a non-255 Non-Ext OP Len is mis-parsed as a classic OPEN |
| [`RFC9072-2-6`](#rfc9072-2-6) In parsing an OPEN message, if the one-octet Optional Parameters Length field (labeled "Non-Ext OP Len." in Figure 1) is non-zero, a BGP speaker MUST use the value of the octet following the one-octet Optional Parameters Length field (labeled "Non-Ext OP Type" in Figure 1) to determine both the encoding of the Optional Parameters length and the size of the Parameter Length field of individual Optional Parameters. (S2) | {gap}, no test | ze's OPEN decoder (internal/component/bgp/message/open.go:190) inspects the octet following Non-Ext OP Len only when Non-Ext OP Len equals 255, so for any other non-zero Non-Ext OP Len it never uses the following octet to determine the encoding and always decodes the classic form |
| [`RFC9072-3-1`](#rfc9072-3-1) It is not considered a fatal error to receive an OPEN message whose (non-extended) Optional Parameters Length value is not 255 and whose first Optional Parameter type code is 255 -- in this case, the encoding of this specification MUST be used for decoding the message. (S3) | {gap}, no test | ze's OPEN decoder (internal/component/bgp/message/open.go:190) selects the extended form only when Non-Ext OP Len equals 255, not whenever the first type code is 255; the open.go:186-189 comment and the TestOpenUnpackExtendedParams standard-format-first-param-byte-0xFF case cement this, so a first type code of 255 with Non-Ext OP Len != 255 is decoded as a classic OPEN instead of extended |
| [`RFC9072-3-3`](#rfc9072-3-3) Although the Optional Parameter type code 255 is used in this specification as the indication that the extended encoding is in use, it is not a bona fide Optional Parameter type code in the usual sense and MUST NOT be used other than as described above. If encountered other than as the Non-Ext OP Type, it MUST be treated as an unrecognized Optional Parameter and handled according to [RFC4271], Section 6.2. (S3) | {gap}, no test | ze silently ignores an unrecognized BGP OPEN optional-parameter type; ParseFromOptionalParams (internal/core/bgp/capability/capability.go:867-874) skips any parameter whose type is not 2 instead of emitting the RFC 4271 Section 6.2 OPEN Message Error (Unsupported Optional Parameter) NOTIFICATION |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC9072-2-1`](#rfc9072-2-1)

However, if the length of the Optional Parameters in the BGP OPEN message does exceed 255, the OPEN message MUST be encoded according to the procedure below. (S2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21 by an independent judge after 375ccbf4f6 changed the negative unit, against rfc/full/rfc9072.txt Section 2: 'if the length of the Optional Parameters in the BGP OPEN message does exceed 255, the OPEN message MUST be encoded according to the procedure below' (Non-Ext OP Len 255, Non-Ext OP Type 255, two-octet Extended Opt. Parm. Length, and 'the Parameter Length field is extended to be a two-octet unsigned integer'). Negative message TestTheExtendedEnvelopeAndItsParametersAgree now packs through capability.PackOptionalParams and writes through Open.WriteTo, then reads the written octets as a receiver: at base forms 256, 257, 302 and two parameters 257 it requires the 255/255 envelope, an Extended Opt. Parm. Length equal to the octets that follow, and a two-octet walk that tiles them exactly with each packed type and value; at base form 255 it requires the RFC 4271 envelope. The prior weak finding (input already in two-octet framing, opaque round trip) is resolved: the parameters now come from the packer. The recorded discrimination break (PackOptionalParams body replaced by panic) proves only that the unit reaches the packer, so the judge ran overlay mutants of PackOptionalParams, tracked file untouched, against a green HEAD: M1 extended branch packing one-octet lengths fails 'parameter 0: two-octet Parameter Length 65184, want 254' on all four extended cases; M2 the off-by-two threshold (baseOctets <= 257, the original defect) writes the 255/255 envelope around one-octet parameters at 256 and 257 and fails the same walk; M3 a threshold one low fails '255/255 envelope written = true, want false' at 255. Positive carriers unchanged: reactor TestRFC9072OptionalParamsBoundary drives Session.buildOpen at capability totals 0..300 and asserts every octet of the procedure at 254 and above, message TestOpenPackExtendedParams the 300-octet envelope. Both polarities hold and the negative violates this requirement, not a neighbour.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestTheExtendedEnvelopeAndItsParametersAgree`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9072_extended_open_test.go#L40) | unit/verify | revert, verified |
| negative | [`TestRFC9072OptionalParamsBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9072_optional_params_boundary_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L419) | unit/verify | revert, verified |
| positive | [`TestRFC9072OptionalParamsBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9072_optional_params_boundary_test.go#L64) | unit/verify | revert, verified |

### [`RFC9072-2-2`](#rfc9072-2-2)

(In any case, an implementation MUST accept an OPEN message that uses the encoding of this specification even if the length of the Optional Parameters is 255 or less.) (S2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC9072ShortExtendedOpenIsAccepted: an extended OPEN with 9 parameter octets decodes with no error, ExtendedParams true, parameters exact, ASN4 4200000001 parsed; a classic reading of the same octets does not recover it; the classic OPEN of the same capability decodes ExtendedParams false to the same capability.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestOpenUnpackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L195) | unit/verify | not verified |
| negative | [`TestRFC9072ShortExtendedOpenIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9072_short_extended_open_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestOpenUnpackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L208) | unit/verify | not verified |
| positive | [`TestRFC9072ShortExtendedOpenIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9072_short_extended_open_test.go#L32) | unit/verify | revert, verified |

### [`RFC9072-2-3`](#rfc9072-2-3)

The Non-Extended Optional Parameters Length field (Non-Ext OP Len.) SHOULD be set to 255 on transmission and, in any event, MUST NOT be set to 0 (S2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. {single-polarity: positive} on the row. TestOpenPackExtendedParams requires body[9]==0xFF and now the exact body length; writeToExtended writes the constant.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L420) | unit/verify | revert, verified |

### [`RFC9072-2-4`](#rfc9072-2-4)

The subsequent one-octet field (which would be the first Optional Parameter Type field in the non-extended format and is called "Non- Ext OP Type" in the figure above) MUST be set to 255 on transmission. (S2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. {single-polarity: positive} on the row. TestOpenPackExtendedParams requires body[10]==0xFF; writeToExtended writes the constant.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L422) | unit/verify | revert, verified |

### [`RFC9072-2-5`](#rfc9072-2-5)

The Non-Extended Optional Parameters Length field (Non-Ext OP Len.) SHOULD be set to 255 on transmission and, in any event, MUST NOT be set to 0; it MUST be ignored on receipt once the use of the extended format is determined positively by inspection of the Non-Extended Optional Parameters Type (Non-Ext OP Type) field. (S2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9072-2-5, so there is no linked test.

### [`RFC9072-2-6`](#rfc9072-2-6)

In parsing an OPEN message, if the one-octet Optional Parameters Length field (labeled "Non-Ext OP Len." in Figure 1) is non-zero, a BGP speaker MUST use the value of the octet following the one-octet Optional Parameters Length field (labeled "Non-Ext OP Type" in Figure 1) to determine both the encoding of the Optional Parameters length and the size of the Parameter Length field of individual Optional Parameters. (S2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9072-2-6, so there is no linked test.

### [`RFC9072-3-1`](#rfc9072-3-1)

It is not considered a fatal error to receive an OPEN message whose (non-extended) Optional Parameters Length value is not 255 and whose first Optional Parameter type code is 255 -- in this case, the encoding of this specification MUST be used for decoding the message. (S3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9072-3-1, so there is no linked test.

### [`RFC9072-3-2`](#rfc9072-3-2)

Although the Optional Parameter type code 255 is used in this specification as the indication that the extended encoding is in use, it is not a bona fide Optional Parameter type code in the usual sense and MUST NOT be used other than as described above. (S3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-14 (independent auditor). Unit changed only by the move to the one packer (capability.PackOptionalParams via packPeerOptionalParams). TestRFC9072ParameterType255OnlyAsTheExtendedMarker fills capability values with 0xFF octets, writes the OPEN and walks the framing the OPEN announces: below 256 octets no envelope and types [2]; above, the 255/255 envelope and every parameter type 2, framing ending exactly at the OPEN's end. TestOpenPackExtendedParams carries the positive marker position. A packer emitting 255 as a parameter type, or the marker on a short OPEN, goes red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9072ParameterType255OnlyAsTheExtendedMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9072_param_type_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestOpenPackExtendedParams`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L423) | unit/verify | revert, verified |
| positive | [`TestRFC9072ParameterType255OnlyAsTheExtendedMarker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9072_param_type_test.go#L57) | unit/verify | revert, verified |

### [`RFC9072-3-3`](#rfc9072-3-3)

Although the Optional Parameter type code 255 is used in this specification as the indication that the extended encoding is in use, it is not a bona fide Optional Parameter type code in the usual sense and MUST NOT be used other than as described above. If encountered other than as the Non-Ext OP Type, it MUST be treated as an unrecognized Optional Parameter and handled according to [RFC4271], Section 6.2. (S3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9072-3-3, so there is no linked test.

### [`RFC9072-2-7`](#rfc9072-2-7)

In the event that the length of the Optional Parameters in the BGP OPEN message does not exceed 255, the encodings of the base BGP specification [RFC4271] SHOULD be used without alteration. (S2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC9072OptionalParamsBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9072_optional_params_boundary_test.go#L72) | unit/verify | revert, verified |

## Requirement list review

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
| `front` | not stated | 1 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 5 | reviewed | not stated |
| `3` | not stated | 3 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 0 | reviewed | not stated |
| `6.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust copyright boilerplate in the Status of This Memo section: it binds the extraction of code components from the document, not any BGP speaker behavior, and the keyword is lowercase 'must'. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |

## Superseded

No document obsoletes RFC 9072, so its obligations are stated where they were written.
