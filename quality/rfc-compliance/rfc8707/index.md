# RFC 8707 - Resource Indicators for OAuth 2.0

No row in the public ledger. Every requirement this repository extracted from RFC 8707, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 50.0% | 2 of 4 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 4 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 5 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 9 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 4 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 4 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 50.0% | 2 of 4 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 7 shares marked as a part above are the whole of the 4 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
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
| Enrolment | Enrolled |
| Requirements | 9 |
| Gated MUST-level | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 5 |
| Tagged units | 5 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8707.md` |
| Requirement shard | `rfc/requirements/rfc8707.md` |
| RFC text | `rfc/full/rfc8707.txt` |

## Enrolment

Enrolled: Thomas selected "Keep present OAuth roles" on 2026-09-21: MCP resource server and external-AS metadata consumer, without an AS, protected-resource discovery client, response signing or signed metadata. JWT audience comparison is enforced from RFC 7519 §§2 and 4.1.3. The historical RFC8707-5-1 and RFC8707-5-2 identifiers retain their tests but explicitly cite that actual source; RFC 8707 §5 is IANA considerations and states neither obligation.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 8707.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC8707-5-1`](#rfc8707-5-1), [`RFC8707-5-2`](#rfc8707-5-2)

**Annotated instead of tested (2):** [`RFC8707-2-1`](#rfc8707-2-1), [`RFC8707-2-6`](#rfc8707-2-6)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8707-3-1` | The authorization server SHOULD audience-restrict issued access tokens to the resource(s) indicated by the resource parameter (RFC 8707 §2; historical section-3 ID) | SHOULD | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §2 explicitly addresses "The authorization server". Thomas excluded the AS role on 2026-09-21. internal/component/mcp/streamable_auth.go::buildAuthForMode constructs a verifier for external tokens; internal/component/mcp/streamable.go::ServeHTTP has no token-issuance endpoint |
| `RFC8707-3-2` | If the authorization server cannot parse the provided resource value(s) or does not consider them acceptable, it should reject the request with the error code invalid_target (RFC 8707 §2.1; historical section-3 ID; lowercase should in the source) | SHOULD | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §2.1 addresses the authorization server's response to the resource request parameter. Thomas excluded the AS role on 2026-09-21. internal/component/mcp/streamable.go::ServeHTTP publishes no authorization or token endpoint; internal/component/mcp/streamable_auth.go::buildAuthForMode verifies external tokens without processing an OAuth resource request parameter |
| `RFC8707-5-1` | A principal processing a JWT whose `aud` is present MUST reject the JWT if no audience value identifies that principal (actual source: RFC 7519 §4.1.3, with exact StringOrURI comparison from §2; historical ID retained, not an RFC 8707 §5 requirement) | MUST | 2 | **positive:** `unit/verify` [`TestOAuthAudienceIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L359). **negative:** `unit/verify` [`TestOAuthAudienceIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L360). **negative:** `unit/verify` [`TestVerifyJWT_RejectAudienceMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_test.go#L294) |
| `RFC8707-5-2` | Each principal intended to process the JWT MUST identify itself with a value in `aud`; the general form is an array of case-sensitive StringOrURI values, and the one-audience form MAY be a string (actual source: RFC 7519 §4.1.3; historical ID retained, not an RFC 8707 §5 requirement) | MUST | x | **positive:** `unit/verify` [`TestVerifyJWT_AudienceArrayForm`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_test.go#L304). **negative:** `unit/verify` [`TestAudClaim_Matches`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_test.go#L508) |
| `RFC8707-2-1` | The `resource` request parameter URI MUST NOT include a fragment component (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "In requests to the authorization server, a client MAY indicate the protected resource (a.k.a. resource server, application, API, etc.) to which it is requesting access by including the following parameter in the request."; Thomas's 2026-09-21 selection retains the resource-server and AS-metadata-consumer roles only. internal/component/mcp/streamable_auth.go::buildAuthForMode verifies external tokens and emits no authorization/token request or resource request parameter |
| `RFC8707-2-2` | The `resource` request parameter URI SHOULD NOT include a query component, though a query component can be a useful and necessary part of the parameter (§2) | SHOULD NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "In requests to the authorization server, a client MAY indicate the protected resource (a.k.a. resource server, application, API, etc.) to which it is requesting access by including the following parameter in the request."; Thomas's 2026-09-21 selection retains resource-server and AS-metadata-consumer roles only. internal/component/mcp/streamable_auth.go::buildAuthForMode verifies tokens without sending this request parameter; audience values are not automatically canonical resource URLs |
| `RFC8707-2-4` | The `resource` parameter MAY be included in an access token request to the token endpoint (§2, §2.2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8707-2-5` | Multiple `resource` values MAY appear; AS policy decides (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8707-2-6` | Its value MUST be an absolute URI, as specified by Section 4.3 of [RFC3986]. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "In requests to the authorization server, a client MAY indicate the protected resource (a.k.a. resource server, application, API, etc.) to which it is requesting access by including the following parameter in the request."; the 2026-09-21 scope retains the resource-server and AS-metadata-consumer roles. internal/component/mcp/streamable_auth.go::buildAuthForMode verifies external tokens and emits no authorization/token request or resource request parameter |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8707-2-1`](#rfc8707-2-1) The `resource` request parameter URI MUST NOT include a fragment component (§2) | no test | no test carries this requirement id; annotated {feature-declined}: "In requests to the authorization server, a client MAY indicate the protected resource (a.k.a. resource server, application, API, etc.) to which it is requesting access by including the following parameter in the request."; Thomas's 2026-09-21 selection retains the resource-server and AS-metadata-consumer roles only. internal/component/mcp/streamable_auth.go::buildAuthForMode verifies external tokens and emits no authorization/token request or resource request parameter |
| [`RFC8707-2-6`](#rfc8707-2-6) Its value MUST be an absolute URI, as specified by Section 4.3 of [RFC3986]. (§2) | no test | no test carries this requirement id; annotated {feature-declined}: "In requests to the authorization server, a client MAY indicate the protected resource (a.k.a. resource server, application, API, etc.) to which it is requesting access by including the following parameter in the request."; the 2026-09-21 scope retains the resource-server and AS-metadata-consumer roles. internal/component/mcp/streamable_auth.go::buildAuthForMode verifies external tokens and emits no authorization/token request or resource request parameter |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8707-5-1`](#rfc8707-5-1)

A principal processing a JWT whose `aud` is present MUST reject the JWT if no audience value identifies that principal (actual source: RFC 7519 §4.1.3, with exact StringOrURI comparison from §2; historical ID retained, not an RFC 8707 §5 requirement)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVerifyJWT_RejectAudienceMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_test.go#L294) | unit/verify | unproven |
| negative | [`TestOAuthAudienceIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L360) | unit/verify | unproven |
| positive | [`TestOAuthAudienceIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L359) | unit/verify | unproven |

### [`RFC8707-5-2`](#rfc8707-5-2)

Each principal intended to process the JWT MUST identify itself with a value in `aud`; the general form is an array of case-sensitive StringOrURI values, and the one-audience form MAY be a string (actual source: RFC 7519 §4.1.3; historical ID retained, not an RFC 8707 §5 requirement)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAudClaim_Matches`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_test.go#L508) | unit/verify | unproven |
| positive | [`TestVerifyJWT_AudienceArrayForm`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_test.go#L304) | unit/verify | unproven |

### [`RFC8707-2-1`](#rfc8707-2-1)

The `resource` request parameter URI MUST NOT include a fragment component (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8707-2-1, so no unit is bound to it.

### [`RFC8707-2-6`](#rfc8707-2-6)

Its value MUST be an absolute URI, as specified by Section 4.3 of [RFC3986]. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8707-2-6, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | Main integration; independent source-mapping review: OAuthMappingReview |
| Signed off | 2026-09-22 |
| Register | prose |
| Source | rfc/full/rfc8707.txt |
| Source fingerprint | 3b04fcfbb0d22e5a |
| Record | rfc/extraction/rfc8707.json |
| Mapped sentences | 2 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 2 | walked | Section 2 states the SHOULD audience-restriction rule under historical RFC8707-3-1, the SHOULD NOT query-component rule under RFC8707-2-2, and the MAY multiple-resource rule under RFC8707-2-5. The MUST-level modal inventory does not derive separate sites for these advisory clauses. |
| `2.1` | not stated | 0 | walked | Section 2.1 states that an authorization server unable to parse or accept the supplied resources should reject the request with invalid_target. The historical section-3 ID records this lowercase recommendation, whose actual citation remains Section 2.1. |
| `2.2` | Section 2 permits including the resource parameter with MAY | 1 | walked | Section 2 permits including the resource parameter with MAY. Section 2.2 states: When the resource parameter is used on an access token request made to the token endpoint, for all grant types, it indicates the target service or protected resource where the client intends to use the requested access token. The retained permission row now cites Sections 2 and 2.2 rather than Section 4, which concerns privacy. |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | Provenance exception, not an obligation read from Section 5: RFC 8707 Section 5 is IANA Considerations and sources neither allocated JWT audience ID. Their actual primary source is RFC 7519 Section 4.1.3 (https://www.rfc-editor.org/rfc/rfc7519#section-4.1.3): Each principal intended to process the JWT MUST identify itself with a value in the audience claim. It also requires rejection when the processing principal is not identified and defines string/array audience forms. Exact StringOrURI comparison comes from RFC 7519 Section 2. rfc/corrections/rfc8707.md records that correction. The schema has no separate cross-document requirement-provenance field, so this historical Section 5 entry retains the IDs with their actual source stated explicitly; it must not be represented as normative RFC 8707 text. |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The sentence about Code Components including Simplified BSD License text is IETF Trust licensing boilerplate governing reuse of document material; it states no OAuth wire or processing requirement. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The introductory sentence describes the security assumption motivating resource indicators: an access token must only be valid for use at a specific protected resource and scope. Section 1.1 reserves BCP 14 force for uppercase keywords. This does not remove JWT recipient audience checks: retained RFC8707-5-1 and RFC8707-5-2 identify their actual RFC 7519 Sections 2 and 4.1.3 source in the checklist and Section 5 provenance exception. | To prevent misuse, several important security assumptions must hold, one of which is that an access token must only be valid for use at a specific protected resource and for a specific scope of access. |
| `2.2:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The sentence explicitly begins As specified in Section 5.1 of [RFC6749] and repeats that RFC's requirement to report the token's effective scope when it differs from the requested scope. This cross-document attribution does not create an RFC 8707 token-issuing role for Ze. | As specified in Section 5.1 of [RFC6749], the authorization server must indicate the access token's effective scope to the client in the "scope" response parameter value when it differs from the scope requested by the client. |

## Superseded

No document obsoletes RFC 8707, so its obligations are stated where they were written.
