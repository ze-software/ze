# RFC 8707 - Resource Indicators for OAuth 2.0

Resource Indicators for OAuth 2.0 (MCP resource server). Every requirement listed for RFC 8707, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 2 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 2 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 2 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 2 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 2 | of 2 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 2 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 2 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 2 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 2 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 2 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 2 | of 7 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 2 mandatory requirements: they add up to 2. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | Resource Indicators for OAuth 2.0 (MCP resource server) |
| Automatic checks | Included |
| Requirements | 7 |
| Mandatory requirements (MUST level) | 2 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 3 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc8707.md` |
| Requirement table | `rfc/requirements/rfc8707.md` |
| RFC text | `rfc/full/rfc8707.txt` |

## Automatic checks

Included in automatic checks: Thomas selected "Keep present OAuth roles" on 2026-09-21: MCP resource server and external-AS metadata consumer, without an AS, protected-resource discovery client, response signing or signed metadata. JWT audience comparison is enforced from RFC 7519 §§2 and 4.1.3. The historical RFC8707-5-1 and RFC8707-5-2 identifiers retain their tests but explicitly cite that actual source; RFC 8707 §5 is IANA considerations and states neither obligation.

## Published support status

**Area:** Resource Indicators for OAuth 2.0 (MCP resource server)

**Test verification:** 0 of 2 MUST proven

**Evidence:** -

**Unmet:** RFC8707-2-1, RFC8707-2-2, RFC8707-2-6


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 2 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **2** | every mandatory requirement appears in exactly one group above |

**Annotated (including scoped evidence) (2):** [`RFC8707-2-1`](#rfc8707-2-1), [`RFC8707-2-6`](#rfc8707-2-6)

## Figures

- **MUST bar**: Not built 2
- **Overview bar**: Exclusion unchecked 2, Not assessed 2, Not built 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 2 |
| MUST rows kept (the MUST bar) | 2 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 2 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 5 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 2 |
| SHOULD not assessed | 2 |
| SHOULD not built | 1 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc8707.json](../../../data/rfc/rfc8707.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8707-3-1` | The authorization server SHOULD audience-restrict issued access tokens to the resource(s) indicated by the "resource" parameter. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §2 explicitly addresses "The authorization server". Thomas excluded the AS role on 2026-09-21. internal/component/mcp/streamable_auth.go::buildAuthForMode constructs a verifier for external tokens; internal/component/mcp/streamable.go::ServeHTTP has no token-issuance endpoint |
| `RFC8707-3-2` | If the authorization server fails to parse the provided value(s) or does not consider the resource(s) acceptable, it should reject the request with an error response using the error code "invalid_target" as the value of the "error" parameter (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §2.1 addresses the authorization server's response to the resource request parameter. Thomas excluded the AS role on 2026-09-21. internal/component/mcp/streamable.go::ServeHTTP publishes no authorization or token endpoint; internal/component/mcp/streamable_auth.go::buildAuthForMode verifies external tokens without processing an OAuth resource request parameter |
| `RFC8707-2-1` | The URI MUST NOT include a fragment component. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends no resource parameter: internal/component/mcp/streamable_auth.go::buildAuthForMode verifies external tokens, and ze has no authorization or token request producer; owed after the first release (owner decision 2026-09-21, "Keep present OAuth roles", recorded in the Enrolment reason, which left it out of the current work) |
| `RFC8707-2-2` | It SHOULD NOT include a query component, but it is recognized that there are cases that make a query component a useful and necessary part of the resource parameter (§2) | SHOULD NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends no resource parameter: internal/component/mcp/streamable_auth.go::buildAuthForMode verifies external tokens, and ze has no authorization or token request producer; owed after the first release (owner decision 2026-09-21, "Keep present OAuth roles", recorded in the Enrolment reason, which left it out of the current work) |
| `RFC8707-2-4` | In requests to the authorization server, a client MAY indicate the protected resource (a.k.a. resource server, application, API, etc.) to which it is requesting access by including the following parameter in the request. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8707-2-5` | Multiple "resource" parameters MAY be used to indicate that the requested token is intended to be used at multiple resources. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8707-2-6` | Its value MUST be an absolute URI, as specified by Section 4.3 of [RFC3986]. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends no resource parameter: internal/component/mcp/streamable_auth.go::buildAuthForMode verifies external tokens, and ze has no authorization or token request producer; owed after the first release (owner decision 2026-09-21, "Keep present OAuth roles", recorded in the Enrolment reason, which left it out of the current work) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8707-2-1`](#rfc8707-2-1) The URI MUST NOT include a fragment component. (§2) | {gap}, no test | ze sends no resource parameter: internal/component/mcp/streamable_auth.go::buildAuthForMode verifies external tokens, and ze has no authorization or token request producer; owed after the first release (owner decision 2026-09-21, "Keep present OAuth roles", recorded in the Enrolment reason, which left it out of the current work) |
| [`RFC8707-2-2`](#rfc8707-2-2) It SHOULD NOT include a query component, but it is recognized that there are cases that make a query component a useful and necessary part of the resource parameter (§2) | {gap} | ze sends no resource parameter: internal/component/mcp/streamable_auth.go::buildAuthForMode verifies external tokens, and ze has no authorization or token request producer; owed after the first release (owner decision 2026-09-21, "Keep present OAuth roles", recorded in the Enrolment reason, which left it out of the current work) |
| [`RFC8707-2-6`](#rfc8707-2-6) Its value MUST be an absolute URI, as specified by Section 4.3 of [RFC3986]. (§2) | {gap}, no test | ze sends no resource parameter: internal/component/mcp/streamable_auth.go::buildAuthForMode verifies external tokens, and ze has no authorization or token request producer; owed after the first release (owner decision 2026-09-21, "Keep present OAuth roles", recorded in the Enrolment reason, which left it out of the current work) |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC8707-2-1`](#rfc8707-2-1)

The URI MUST NOT include a fragment component. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8707-2-1, so there is no linked test.

### [`RFC8707-2-6`](#rfc8707-2-6)

Its value MUST be an absolute URI, as specified by Section 4.3 of [RFC3986]. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8707-2-6, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | Main integration; independent source-mapping review: OAuthMappingReview |
| Signed off | 2026-09-22 |
| Register | rfc2119 |
| Source | rfc/full/rfc8707.txt |
| Source fingerprint | 3b04fcfbb0d22e5a |
| Record | rfc/extraction/rfc8707.json |
| Mapped sentences | 2 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `1.2` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 2 | reviewed | Section 2 states the SHOULD audience-restriction rule under historical RFC8707-3-1, the SHOULD NOT query-component rule under RFC8707-2-2, and the MAY multiple-resource rule under RFC8707-2-5. The MUST-level modal inventory does not derive separate sites for these advisory clauses. |
| `2.1` | not stated | 0 | reviewed | Section 2.1 states that an authorization server unable to parse or accept the supplied resources should reject the request with invalid_target. The historical section-3 ID records this lowercase recommendation, whose actual citation remains Section 2.1. |
| `2.2` | Section 2 permits including the resource parameter with MAY | 0 | reviewed | Section 2 permits including the resource parameter with MAY. Section 2.2 states: When the resource parameter is used on an access token request made to the token endpoint, for all grant types, it indicates the target service or protected resource where the client intends to use the requested access token. The retained permission row now cites Sections 2 and 2.2 rather than Section 4, which concerns privacy. |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | IANA Considerations | 0 | reviewed | IANA Considerations. No site: the section registers parameters and binds no implementer. The two JWT audience rows once filed here state RFC 7519 Section 4.1.3, and were retired under owner decision D-10 (rfc/corrections/rfc8707.md). |
| `5.1` | not stated | 0 | reviewed | not stated |
| `5.2` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 0 | reviewed | not stated |
| `6.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 8707 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 8707, so its obligations are stated where they were written.
