# RFC 8522 - Looking Glass Command Set

Looking Glass command set API. Every requirement listed for RFC 8522, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 11 | of 18 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 7 | of 18 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 18 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 18 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 18 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 19 | 19 of 38 tests linked to requirements, 0 without a fault test and 19 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 18 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 18 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 18 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 18 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 18 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 18 | of 23 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 18 mandatory requirements: they add up to 18. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
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
| Public row | Looking Glass command set API |
| Automatic checks | Included |
| Requirements | 23 |
| Mandatory requirements (MUST level) | 18 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 38 |
| Tests linked to requirements | 38 |
| Recorded audit verdicts | 23 |
| Fault test records | 38 |
| Summary | `rfc/short/rfc8522.md` |
| Requirement table | `rfc/requirements/rfc8522.md` |
| RFC text | `rfc/full/rfc8522.txt` |

## Automatic checks

Included in automatic checks: The document cites neither RFC 2119 nor BCP 14 and uses no capitalised keyword, but it specifies an HTTP API with obligations in lower-case "must" and "mandatory" sentences, so it is walked at register prose and its server obligations are gated at MUST level. 18 gated rows: 16 sourced from a "must" site and two unsourced ("Implementation of the ping command is mandatory." in Section 3.1.1 and the same for "show route" in Section 3.2.1, which the modal inventory cannot see). All 18 gated rows and 5 advisory rows are implemented in Ze's looking glass HTTP server under /.well-known/looking-glass/v1/ (internal/component/lg/handler_rfc8522.go, server.go, auth.go) with strict JSend envelopes, HTTPS enforcement, per-IP rate limiting, mandatory ping/show-route/organizational commands, and optional traceroute/BGP commands. Site 2:1 binds the API client and is excluded as binds-another-role. The walk is recorded in rfc/extraction/rfc8522.json at register prose. Enrolled 2026-10-26.

## Published support status

**Area:** Looking Glass command set API

**Test verification:** 9 of 18 MUST proven

**Evidence:** unit 9

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 11 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 7 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **18** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (11):** [`RFC8522-2-1`](#rfc8522-2-1), [`RFC8522-2-2`](#rfc8522-2-2), [`RFC8522-2-3`](#rfc8522-2-3), [`RFC8522-2.3.1-1`](#rfc8522-2.3.1-1), [`RFC8522-2.3.2-1`](#rfc8522-2.3.2-1), [`RFC8522-2.3.3-1`](#rfc8522-2.3.3-1), [`RFC8522-3.1.1-1`](#rfc8522-3.1.1-1), [`RFC8522-3.2.1-1`](#rfc8522-3.2.1-1), [`RFC8522-3.3-1`](#rfc8522-3.3-1), [`RFC8522-3.3.2-1`](#rfc8522-3.3.2-1), [`RFC8522-6-1`](#rfc8522-6-1)

**Annotated (including scoped evidence) (7):** [`RFC8522-2.2-1`](#rfc8522-2.2-1), [`RFC8522-2.3-1`](#rfc8522-2.3-1), [`RFC8522-2.3-2`](#rfc8522-2.3-2), [`RFC8522-2.3.1-2`](#rfc8522-2.3.1-2), [`RFC8522-2.3.1-3`](#rfc8522-2.3.1-3), [`RFC8522-2.3.1-4`](#rfc8522-2.3.1-4), [`RFC8522-2.3.1-5`](#rfc8522-2.3.1-5)

## Figures

- **MUST bar**: Ze verified 9, Tests awaiting verification 9
- **Overview bar**: Ze MUST verified 9, SHOULD verified 1, Tests awaiting verification 13

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 18 |
| MUST rows kept (the MUST bar) | 18 |
| Ze verified | 9 |
| OS verified | 0 |
| Tests awaiting verification | 9 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 5 |
| SHOULD verified | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 4 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 0 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc8522.json](../../../data/rfc/rfc8522.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8522-2-1` | The path component of the resource URI must use the prefix of ".well- known/looking-glass" (see Section 5.1) to stay namespace neutral. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC8522WellKnownPrefixAndRejections`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L232). **negative:** `unit/verify` [`TestRFC8522WellKnownPrefixAndRejections`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L233) |
| `RFC8522-2-2` | The client must not be able to execute random commands on the targeting router. (§2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestRFC8522WellKnownPrefixAndRejections`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L234). **negative:** `unit/verify` [`TestRFC8522WellKnownPrefixAndRejections`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L235) |
| `RFC8522-2-3` | There must not be any direct communication between the client and the router. (§2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestRFC8522WellKnownPrefixAndRejections`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L236). **negative:** `unit/verify` [`TestRFC8522WellKnownPrefixAndRejections`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L237) |
| `RFC8522-2.2-1` | The server must ignore its value. (§2.2) | MUST | 2.2 | **positive:** `unit/verify` [`TestRFC8522RandomParameterIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L342). **negative:** no negative test. **{single-polarity}:** the server unconditionally ignores the cache-buster random query parameter in parseRFC8522Query (internal/component/lg/handler_rfc8522.go) so no value of random is invalid or rejected; positive verifies empty, alphanumeric, and special-character values dispatch the identical command and return identical data to a baseline request without random |
| `RFC8522-2.3-1` | The response text must comply with the JSON syntax specification JSend, which is briefly explained in Appendix A. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L393). **negative:** no negative test. **{single-polarity}:** this is a server output-format invariant whose only emitters (writeRFC8522Success, writeRFC8522Fail, writeRFC8522Error in internal/component/lg/handler_rfc8522.go and the 401 branch in auth.go) always write JSend JSON with Content-Type application/json; there is no non-JSend branch to trigger negatively |
| `RFC8522-2.3-2` | Consequently, every response must contain a "status" field of either "success", "fail", or "error" as explained in the following sections. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L394). **negative:** no negative test. **{single-polarity}:** all RFC 8522 emitters set status from the closed three-value set ("success", "fail", "error") with no code path that omits or alters status |
| `RFC8522-2.3.1-1` | A successful response must set the "status" field to "success". (§2.3.1) | MUST | 2.3.1 | **positive:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L395). **negative:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L396) |
| `RFC8522-2.3.1-2` | It must also contain a "data" object including the following information: (§2.3.1) | MUST | 2.3.1 | **positive:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L397). **negative:** no negative test. **{single-polarity}:** writeRFC8522Success and rfc8522BaseData (internal/component/lg/handler_rfc8522.go) unconditionally emit a non-null data object on every successful response, leaving no non-conformant input path to reject |
| `RFC8522-2.3.1-3` | Combined date and time in UTC ISO 8601 [iso8601] indicating when the operation finished. This information must be present. (§2.3.1) | MUST | 2.3.1 | **positive:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L398). **negative:** no negative test. **{single-polarity}:** rfc8522BaseData (internal/component/lg/handler_rfc8522.go) always stamps performed_at with time.Now().UTC().Format(time.RFC3339) at command completion, with no input that can omit or non-UTC-format it |
| `RFC8522-2.3.1-4` | Amount of seconds (wallclock) used to run the command. This information must be present. (§2.3.1) | MUST | 2.3.1 | **positive:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L399). **negative:** no negative test. **{single-polarity}:** rfc8522BaseData (internal/component/lg/handler_rfc8522.go) always computes runtime from time.Since(start).Seconds() with no input that omits it; positive verifies real elapsed wallclock duration on a 15ms delayed command |
| `RFC8522-2.3.1-5` | Adding more information to the response is permitted and must be placed inside the "data" object. (§2.3.1) | MUST | 2.3.1 | **positive:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L400). **negative:** no negative test. **{single-polarity}:** writeRFC8522Success and writeRFC8522Fail accept only a data map and emit a fixed two-key top-level object with "status" and "data", so no code path emits extra top-level keys |
| `RFC8522-2.3.2-1` | The response message must set the "status" field to "fail" and must contain the "data" object with command-specific content listed in Section 2.3.1. (§2.3.2) | MUST | 2.3.2 | **positive:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L403). **negative:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L404) |
| `RFC8522-2.3.3-1` | The response message must set the "status" field to "error" and must contain the "message" key, which keeps a meaningful message, explaining what went wrong. (§2.3.3) | MUST | 2.3.3 | **positive:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L405). **negative:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L406) |
| `RFC8522-3.1.1-1` | Implementation of the ping command is mandatory. (§3.1.1) | REQUIRED | 3.1.1 | **positive:** `unit/verify` [`TestRFC8522MandatoryCommands`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L628). **negative:** `unit/verify` [`TestRFC8522MandatoryCommands`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L629) |
| `RFC8522-3.2.1-1` | Implementation of the "show route" command is mandatory. (§3.2.1) | REQUIRED | 3.2.1 | **positive:** `unit/verify` [`TestRFC8522MandatoryCommands`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L630). **negative:** `unit/verify` [`TestRFC8522MandatoryCommands`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L631) |
| `RFC8522-3.3-1` | The following organizational commands must be included in the implementation. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC8522MandatoryCommands`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L632). **negative:** `unit/verify` [`TestRFC8522MandatoryCommands`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L633) |
| `RFC8522-3.3.2-1` | The response must contain the router's hostname and router index. (§3.3.2) | MUST | 3.3.2 | **positive:** `unit/verify` [`TestRFC8522MandatoryCommands`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L634). **negative:** `unit/verify` [`TestRFC8522MandatoryCommands`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L635) |
| `RFC8522-6-1` | The use of HTTPS is required to ensure a high level of security, privacy, and confidentiality during transit. (§6) | REQUIRED | 6 | **positive:** `unit/verify` [`TestRFC8522RequiresHTTPS`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L868). **negative:** `unit/verify` [`TestRFC8522RequiresHTTPS`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L869) |
| `RFC8522-2.3.1-6` | The HTTP status code should be 200. (§2.3.1) | SHOULD | 2.3.1 | **positive:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L401). **negative:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L402) |
| `RFC8522-2.3.3-3` | The HTTP status code should be 400 in case of a client-side error, 500 in case of a server-side error, or 502 for errors occurring on the target router. (§2.3.3) | SHOULD | 2.3.3 | **positive:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L408). **negative:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L409) |
| `RFC8522-2.3.3-4` | Code 504 should be used when a command timed out. (§2.3.3) | SHOULD | 2.3.3 | **positive:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L410). **negative:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L411) |
| `RFC8522-6.1-1` | It is recommended that implementers of the Looking Glass API take steps to mitigate the above described abuse. (§6.1) | RECOMMENDED | 6.1 | **positive:** `unit/verify` [`TestRFC8522RateLimiting`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L921). **negative:** `unit/verify` [`TestRFC8522RateLimiting`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L922) |
| `RFC8522-2.3.3-2` | The response may contain the "data" key with required values listed in Section 2.3.1. (§2.3.3) | MAY | 2.3.3 | **positive:** `unit/verify` [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L407). **negative:** no negative test. **{single-polarity}:** optional permission to include data with performed_at and runtime on error responses; positive verifies router-failure and timeout error responses include valid performed_at and elapsed runtime |

## Gaps and untested MUSTs

RFC 8522 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC8522-2-1`](#rfc8522-2-1)

The path component of the resource URI must use the prefix of ".well- known/looking-glass" (see Section 5.1) to stay namespace neutral. (§2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/lg/handler_rfc8522_test.go::TestRFC8522WellKnownPrefixAndRejections, internal/component/lg/handler_rfc8522_test.go::TestRFC8522WellKnownPrefixAndRejections#2 moved. TestRFC8522WellKnownPrefixAndRejections isolates the well-known prefix in both polarities: positive GET /.well-known/looking-glass/v1/routers over TLS asserts StatusOK and jsendSuccess, and negative GET /looking-glass/v1/routers (recOutside) asserts StatusNotFound.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8522WellKnownPrefixAndRejections`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L233) | unit/verify | revert, verified |
| positive | [`TestRFC8522WellKnownPrefixAndRejections`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L232) | unit/verify | revert, verified |

### [`RFC8522-2-2`](#rfc8522-2-2)

The client must not be able to execute random commands on the targeting router. (§2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/lg/handler_rfc8522_test.go::TestRFC8522WellKnownPrefixAndRejections, internal/component/lg/handler_rfc8522_test.go::TestRFC8522WellKnownPrefixAndRejections#2 moved. TestRFC8522WellKnownPrefixAndRejections proves both polarities: positive recPing verifies allowlisted GET /.well-known/looking-glass/v1/ping/192.0.2.1 returns StatusOK and dispatches exact router command "show ping 192.0.2.1", while negative rejected cases (unknown commands, POST, shell/pipe/space injection) return StatusBadRequest with jsendError and len(calls) == 0.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8522WellKnownPrefixAndRejections`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L235) | unit/verify | revert, verified |
| positive | [`TestRFC8522WellKnownPrefixAndRejections`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L234) | unit/verify | revert, verified |

### [`RFC8522-2-3`](#rfc8522-2-3)

There must not be any direct communication between the client and the router. (§2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/lg/handler_rfc8522_test.go::TestRFC8522WellKnownPrefixAndRejections, internal/component/lg/handler_rfc8522_test.go::TestRFC8522WellKnownPrefixAndRejections#2 moved. TestRFC8522WellKnownPrefixAndRejections proves both polarities: positive recPing dispatches exclusively through CommandDispatcher with no Location redirect and recErr verifies backend errors return StatusBadGateway without leaking errRFC8522TestEngineFailed; negative upRec rejects Upgrade: websocket with StatusBadRequest, jsendError, and len(calls) == 0.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8522WellKnownPrefixAndRejections`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L237) | unit/verify | revert, verified |
| positive | [`TestRFC8522WellKnownPrefixAndRejections`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L236) | unit/verify | revert, verified |

### [`RFC8522-2.2-1`](#rfc8522-2.2-1)

The server must ignore its value. (§2.2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/lg/handler_rfc8522_test.go::TestRFC8522RandomParameterIgnored moved. TestRFC8522RandomParameterIgnored verifies empty, alphanumeric, and special-character random/RANDOM values return StatusOK, jsendSuccess, identical payload fields via reflect.DeepEqual against baseData, and exact "show ping 192.0.2.1" dispatch; {single-polarity: positive} is justified because parseRFC8522Query ignores all values of random unconditionally.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC8522RandomParameterIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L342) | unit/verify | revert, verified |

### [`RFC8522-2.3-1`](#rfc8522-2.3-1)

The response text must comply with the JSON syntax specification JSend, which is briefly explained in Appendix A. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming moved. TestRFC8522JSendEnvelopeAndTiming verifies Content-Type application/json and strict JSend top-level keys via decodeJSend and assertJSendKeys across success, fail, 400, 401 (authSrv/recAuth), 500 (writeRFC8522JSON), 502, and 504; {single-polarity: positive} is justified because all RFC 8522 emitters unconditionally write JSend JSON.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L393) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC8522-2.3-2`](#rfc8522-2.3-2)

Consequently, every response must contain a "status" field of either "success", "fail", or "error" as explained in the following sections. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming moved. TestRFC8522JSendEnvelopeAndTiming verifies status values jsendSuccess, jsendFail, and jsendError across all response categories via assertJSendKeys; {single-polarity: positive} is justified because all RFC 8522 response writers emit status solely from that closed three-value set.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L394) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC8522-2.3.1-1`](#rfc8522-2.3.1-1)

A successful response must set the "status" field to "success". (§2.3.1)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming, internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming#2 moved. TestRFC8522JSendEnvelopeAndTiming proves both polarities: positive recOK (ping) and recRouteOK (show route) assert StatusOK and jsendSuccess via assertJSendKeys, while negative recFail, recMiss (jsendFail) and recClient/recAuth/recServerErr/recRouter/recTimeout (jsendError) confirm non-success outcomes do not set status to success.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L396) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L395) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC8522-2.3.1-2`](#rfc8522-2.3.1-2)

It must also contain a "data" object including the following information: (§2.3.1)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming moved. TestRFC8522JSendEnvelopeAndTiming verifies non-null data object on recOK and recRouteOK via assertJSendKeys, assertRequiredDataFields, router, format, and non-empty rfc8522OutputText; {single-polarity: positive} is justified because writeRFC8522Success and rfc8522BaseData unconditionally emit a data object.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L397) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC8522-2.3.1-3`](#rfc8522-2.3.1-3)

Combined date and time in UTC ISO 8601 [iso8601] indicating when the operation finished. This information must be present. (§2.3.1)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming moved. TestRFC8522JSendEnvelopeAndTiming verifies data.performed_at via assertRequiredDataFields is UTC RFC3339 ending in Z and bounded to the [beforeOK, afterOK] execution window; {single-polarity: positive} is justified because rfc8522BaseData unconditionally stamps performed_at in UTC RFC3339 at completion.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L398) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC8522-2.3.1-4`](#rfc8522-2.3.1-4)

Amount of seconds (wallclock) used to run the command. This information must be present. (§2.3.1)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming moved. TestRFC8522JSendEnvelopeAndTiming verifies real elapsed wallclock runtime >= 0.01s on a 15ms delayed command (slowSrv/recSlow) and >= 0.015s on a 20ms timeout (recTimeout/timeoutData); {single-polarity: positive} is justified because rfc8522BaseData unconditionally computes wallclock seconds from start.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L399) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC8522-2.3.1-5`](#rfc8522-2.3.1-5)

Adding more information to the response is permitted and must be placed inside the "data" object. (§2.3.1)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming moved. TestRFC8522JSendEnvelopeAndTiming verifies extra fields min, avg, max, rate (recOK), routers (recRouters), and id, name (recDetails) are placed inside data and never at top level (assertJSendKeys enforces len(body) == 2); {single-polarity: positive} is justified because writeRFC8522Success and writeRFC8522Fail emit a fixed two-key top-level object.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L400) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC8522-2.3.2-1`](#rfc8522-2.3.2-1)

The response message must set the "status" field to "fail" and must contain the "data" object with command-specific content listed in Section 2.3.1. (§2.3.2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming, internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming#2 moved. TestRFC8522JSendEnvelopeAndTiming proves both polarities: positive recFail (0% ping) and recMiss (missing route) assert StatusOK, jsendFail, bounded performed_at/runtime, format text/plain, and non-empty rfc8522OutputText; negative recOK and recRouteOK confirm succeeding commands return jsendSuccess rather than jsendFail.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L404) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L403) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC8522-2.3.3-1`](#rfc8522-2.3.3-1)

The response message must set the "status" field to "error" and must contain the "message" key, which keeps a meaningful message, explaining what went wrong. (§2.3.3)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming, internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming#2 moved. TestRFC8522JSendEnvelopeAndTiming proves both polarities: positive recClient (host), recAuth (unauthorized), recServerErr (500), recRouter (engine error, redacted), and recTimeout (timed out) assert jsendError with meaningful cause-specific message; negative recOK/recRouteOK/recFail/recMiss verify non-error responses do not set status to error.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L406) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L405) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC8522-3.1.1-1`](#rfc8522-3.1.1-1)

Implementation of the ping command is mandatory. (§3.1.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/lg/handler_rfc8522_test.go::TestRFC8522MandatoryCommands, internal/component/lg/handler_rfc8522_test.go::TestRFC8522MandatoryCommands#2 moved. TestRFC8522MandatoryCommands proves both polarities: positive pingCases executes IPv4, IPv6 (protocol=2,1), and DNS hostname (example.com) asserting StatusOK, jsendSuccess, exact min/avg/max, rate 100, and non-empty rfc8522OutputText; negative badPing rejects protocol mismatch and invalid host with StatusBadRequest and jsendError.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8522MandatoryCommands`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L629) | unit/verify | revert, verified |
| positive | [`TestRFC8522MandatoryCommands`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L628) | unit/verify | revert, verified |

### [`RFC8522-3.2.1-1`](#rfc8522-3.2.1-1)

Implementation of the "show route" command is mandatory. (§3.2.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/lg/handler_rfc8522_test.go::TestRFC8522MandatoryCommands, internal/component/lg/handler_rfc8522_test.go::TestRFC8522MandatoryCommands#2 moved. TestRFC8522MandatoryCommands proves both polarities: positive recRoute4, recRoute6, and recLookup assert StatusOK, jsendSuccess, and route prefix/gateway in rfc8522OutputText, while recMissing and recLookupMissing return jsendFail; negative badRoute rejects malformed address and protocol mismatch with StatusBadRequest and jsendError.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8522MandatoryCommands`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L631) | unit/verify | revert, verified |
| positive | [`TestRFC8522MandatoryCommands`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L630) | unit/verify | revert, verified |

### [`RFC8522-3.3-1`](#rfc8522-3.3-1)

The following organizational commands must be included in the implementation. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/lg/handler_rfc8522_test.go::TestRFC8522MandatoryCommands, internal/component/lg/handler_rfc8522_test.go::TestRFC8522MandatoryCommands#2 moved. TestRFC8522MandatoryCommands proves both polarities: positive recRouters, recDetails (routers/0), and recCmd (asserting command, href, description, arguments on every cmdList entry plus ping and show route) return StatusOK and jsendSuccess; negative badRouter rejects /v1/routers/1, /v1/routers/abc, /v1/routers/0/extra, and /v1/cmd/extra with StatusBadRequest and jsendError.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8522MandatoryCommands`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L633) | unit/verify | revert, verified |
| positive | [`TestRFC8522MandatoryCommands`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L632) | unit/verify | revert, verified |

### [`RFC8522-3.3.2-1`](#rfc8522-3.3.2-1)

The response must contain the router's hostname and router index. (§3.3.2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/lg/handler_rfc8522_test.go::TestRFC8522MandatoryCommands, internal/component/lg/handler_rfc8522_test.go::TestRFC8522MandatoryCommands#2 moved. TestRFC8522MandatoryCommands proves both polarities: positive recDetails (GET /v1/routers/0) asserts StatusOK, jsendSuccess, detailsData["name"] == testRFC8522Hostname, and detailsData["id"] == 0; negative badRouter rejects /v1/routers/1, /v1/routers/abc, and /v1/routers/0/extra with StatusBadRequest and jsendError.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8522MandatoryCommands`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L635) | unit/verify | revert, verified |
| positive | [`TestRFC8522MandatoryCommands`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L634) | unit/verify | revert, verified |

### [`RFC8522-6-1`](#rfc8522-6-1)

The use of HTTPS is required to ensure a high level of security, privacy, and confidentiality during transit. (§6)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/lg/handler_rfc8522_test.go::TestRFC8522RequiresHTTPS, internal/component/lg/handler_rfc8522_test.go::TestRFC8522RequiresHTTPS#2 moved. TestRFC8522RequiresHTTPS exercises real TCP listeners in both polarities on the identical path /.well-known/looking-glass/v1/routers: positive startTLSTestServer over HTTPS asserts StatusOK and jsendSuccess; negative startTestServer over plaintext HTTP asserts StatusBadRequest, jsendError, and an error message naming HTTPS.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8522RequiresHTTPS`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L869) | unit/verify | revert, verified |
| positive | [`TestRFC8522RequiresHTTPS`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L868) | unit/verify | revert, verified |

### [`RFC8522-2.3.1-6`](#rfc8522-2.3.1-6)

The HTTP status code should be 200. (§2.3.1)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming, internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming#2 moved. TestRFC8522JSendEnvelopeAndTiming proves both polarities: positive recOK, recRouteOK, recFail, and recMiss assert StatusOK (200); negative recClient (400), recRouter (502), and recTimeout (504) confirm error responses do not return HTTP 200.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L402) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L401) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC8522-2.3.3-3`](#rfc8522-2.3.3-3)

The HTTP status code should be 400 in case of a client-side error, 500 in case of a server-side error, or 502 for errors occurring on the target router. (§2.3.3)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming, internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming#2 moved. TestRFC8522JSendEnvelopeAndTiming proves both polarities across all three RFC 8522 Section 2.3.3 codes: positive recClient returns StatusBadRequest (400), recServerErr via writeRFC8522JSON returns StatusInternalServerError (500) with jsendError, and recRouter returns StatusBadGateway (502); negative recOK returns StatusOK (200).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L409) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L408) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC8522-2.3.3-4`](#rfc8522-2.3.3-4)

Code 504 should be used when a command timed out. (§2.3.3)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming, internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming#2 moved. TestRFC8522JSendEnvelopeAndTiming proves both polarities: positive recTimeout (runtime=0.02) asserts StatusGatewayTimeout (504) with jsendError and timeout message; negative recRouter confirms non-timeout router dispatch failure returns StatusBadGateway (502) rather than 504.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L411) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L410) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC8522-6.1-1`](#rfc8522-6.1-1)

It is recommended that implementers of the Looking Glass API take steps to mitigate the above described abuse. (§6.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/lg/handler_rfc8522_test.go::TestRFC8522RateLimiting, internal/component/lg/handler_rfc8522_test.go::TestRFC8522RateLimiting#2 moved. TestRFC8522RateLimiting proves both polarities and per-client-IP isolation: positive rfc8522RequestFrom confirms requests within the per-IP limit and from a distinct client IP (recOther) return StatusOK and jsendSuccess; negative rec3 confirms the 3rd request from the throttled IP returns StatusTooManyRequests (429) with jsendError.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8522RateLimiting`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L922) | unit/verify | revert, verified |
| positive | [`TestRFC8522RateLimiting`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L921) | unit/verify | revert, verified |

### [`RFC8522-2.3.3-2`](#rfc8522-2.3.3-2)

The response may contain the "data" key with required values listed in Section 2.3.1. (§2.3.3)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/lg/handler_rfc8522_test.go::TestRFC8522JSendEnvelopeAndTiming moved. TestRFC8522JSendEnvelopeAndTiming verifies optional data on 502 (recRouter) and 504 (recTimeout/timeoutData) via assertRequiredDataFields with bounded performed_at and elapsed runtime >= 0.015s; {single-polarity: positive} is justified for this optional MAY permission.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC8522JSendEnvelopeAndTiming`](https://github.com/ze-software/ze/blob/main/internal/component/lg/handler_rfc8522_test.go#L407) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude (spec-rfc-profile-declared-membership phase 1) |
| Signed off | 2026-10-26 |
| Register | prose |
| Source | rfc/full/rfc8522.txt |
| Source fingerprint | 4dc3cf63cfe82351 |
| Record | rfc/extraction/rfc8522.json |
| Mapped sentences | 17 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | Title page, abstract, status, copyright and contents | 0 | skipped (front-matter) | Title page, abstract, status, copyright and contents |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `1.2` | not stated | 0 | reviewed | not stated |
| `1.3` | not stated | 1 | reviewed | not stated |
| `2` | not stated | 4 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `2.2` | not stated | 1 | reviewed | not stated |
| `2.3` | not stated | 2 | reviewed | not stated |
| `2.3.1` | not stated | 5 | reviewed | not stated |
| `2.3.2` | not stated | 1 | reviewed | not stated |
| `2.3.3` | not stated | 2 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 0 | reviewed | not stated |
| `3.1.1` | not stated | 0 | reviewed | not stated |
| `3.1.2` | not stated | 0 | reviewed | not stated |
| `3.2` | not stated | 0 | reviewed | not stated |
| `3.2.1` | not stated | 0 | reviewed | not stated |
| `3.2.2` | not stated | 0 | reviewed | not stated |
| `3.2.3` | not stated | 0 | reviewed | not stated |
| `3.2.4` | not stated | 0 | reviewed | not stated |
| `3.3` | not stated | 1 | reviewed | not stated |
| `3.3.1` | not stated | 0 | reviewed | not stated |
| `3.3.2` | not stated | 1 | reviewed | not stated |
| `3.3.3` | not stated | 0 | reviewed | not stated |
| `3.4` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | Well-Known URI registration | 0 | skipped (iana) | Well-Known URI registration |
| `5.1` | Well-Known URI registration | 0 | skipped (iana) | Well-Known URI registration |
| `6` | not stated | 1 | reviewed | not stated |
| `6.1` | not stated | 0 | reviewed | not stated |
| `6.2` | not stated | 0 | reviewed | not stated |
| `6.3` | not stated | 0 | reviewed | not stated |
| `7` | References | 0 | skipped (references) | References |
| `7.1` | Normative references | 0 | skipped (references) | Normative references |
| `7.2` | Informative references | 0 | skipped (references) | Informative references |
| `A` | not stated | 3 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive: says the examples in the document omit some required parameters; it states nothing an implementation does | The examples skip some required parameters for reasons of simplicity. |
| `2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the looking-glass API client, the HTTP caller that sends the request ('The client must present all necessary parameters'). Ze is the server; the client is outside Ze, and Ze holds no looking-glass API client code | The client must present all necessary parameters for the server to execute the command on the selected router. |
| `A:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the status-key obligation of Section 2.3 for JSend in general; the 'should contain' part is advisory and keyed to the status value, which Sections 2.3.1 to 2.3.3 state per status | A basic JSend-compliant response must contain a "status" key and should contain "data", "message", and "code" keys dependent on the status value. |
| `A:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive: introduces Table 1; 'required' names a column of the table | The following table lists the required and optional keys. |
| `A:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Table 1 of JSend keys; its Required column restates the per-status obligations of Sections 2.3.1, 2.3.2 and 2.3.3, which are mapped there | +---------+-----------------+---------------+ \| Type \| Required keys \| Optional keys \| +---------+-----------------+---------------+ \| success \| status, data \| \| \| fail \| status, data \| \| \| error \| status, message \| code, data \| +---------+-----------------+---------------+ |

## Superseded

No document obsoletes RFC 8522, so its obligations are stated where they were written.
