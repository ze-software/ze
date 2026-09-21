# RFC 8707 - Resource Indicators for OAuth 2.0

No row in the public ledger. Every requirement this repository extracted from RFC 8707, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 66.7% | 2 of 3 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 3 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 3 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 3 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 4 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 3 | of 8 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 3 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 33.3% | 1 of 3 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 3 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 3 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 7 shares marked as a part above are the whole of the 3 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 8 |
| Gated MUST-level | 3 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tagged units | 4 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8707.md` |
| Requirement shard | `rfc/requirements/rfc8707.md` |
| RFC text | `rfc/full/rfc8707.txt` |

## Enrolment

Enrolled: Resource Indicators for OAuth 2.0 (ze as an OAuth resource server): seven MUST-level requirements. Three are met with positive+negative tags in internal/component/mcp: 5-1 (reject a token whose audience does not match the resource), 5-2 (accept an array audience when at least one entry matches), and 2-3 (canonicalize the audience so a trailing-slash divergence does not mismatch). 3-1 (set the token audience when minting) and 3-2 (return invalid_target for an unknown resource) are {not-applicable}: those are authorization-server token-endpoint behaviors and ze issues no tokens. 2-1 (no fragment) and 2-2 (no query) are {not-applicable}: they govern the client-formed canonical resource request parameter, which a resource server neither emits nor consumes.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 8707.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **3** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC8707-5-1`](#rfc8707-5-1), [`RFC8707-5-2`](#rfc8707-5-2)

**Annotated instead of tested (1):** [`RFC8707-2-1`](#rfc8707-2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8707-3-1` | The authorization server SHOULD audience-restrict issued access tokens to the resource(s) indicated by the resource parameter (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** setting the token audience is an authorization-server obligation performed when minting a token; ze is an OAuth resource server (internal/component/mcp) and never issues or signs tokens, so it has no audience-setting code path |
| `RFC8707-3-2` | If the authorization server cannot parse the provided resource value(s) or does not consider the resource(s) acceptable, it should reject the request with the error code invalid_target (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the invalid_target error is an authorization-server token-endpoint behavior; ze has no token or authorization endpoint (grep for invalid_target in internal/component/mcp finds none) |
| `RFC8707-5-1` | Resource server MUST reject any token whose `aud` does not match the resource's own canonical URL (§5) | MUST | 5 | **positive:** `unit/verify` [`TestNewStreamable_OAuth_AcceptsSlashDivergentAudience`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L410). **negative:** `unit/verify` [`TestVerifyJWT_RejectAudienceMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_test.go#L268) |
| `RFC8707-5-2` | If `aud` is a JSON array, at least one entry MUST match; array-shape decoding is required (§5) | MUST | 5 | **positive:** `unit/verify` [`TestVerifyJWT_AudienceArrayForm`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_test.go#L278). **negative:** `unit/verify` [`TestAudClaim_Matches`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_test.go#L482) |
| `RFC8707-2-1` | Fragment MUST be absent from the canonical resource URL (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this governs the canonical resource URI a client forms and sends as the resource request parameter; ze is a resource server that never emits or consumes that parameter (internal/component/mcp validates the token audience only) |
| `RFC8707-2-2` | The resource URI SHOULD NOT include a query component, though a query component can be a useful and necessary part of the resource parameter (§2) | SHOULD NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this governs the client-formed canonical resource URI; ze (resource server) never emits or consumes the resource request parameter, validating only the token audience |
| `RFC8707-2-4` | `resource` parameter MAY appear on the token endpoint (§2, §4) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8707-2-5` | Multiple `resource` values MAY appear; AS policy decides (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8707-2-1`](#rfc8707-2-1) Fragment MUST be absent from the canonical resource URL (§2) | no test | no test carries this requirement id; annotated {not-applicable}: this governs the canonical resource URI a client forms and sends as the resource request parameter; ze is a resource server that never emits or consumes that parameter (internal/component/mcp validates the token audience only) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8707-5-1`](#rfc8707-5-1)

Resource server MUST reject any token whose `aud` does not match the resource's own canonical URL (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVerifyJWT_RejectAudienceMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_test.go#L268) | unit/verify | unproven |
| positive | [`TestNewStreamable_OAuth_AcceptsSlashDivergentAudience`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L410) | unit/verify | unproven |

### [`RFC8707-5-2`](#rfc8707-5-2)

If `aud` is a JSON array, at least one entry MUST match; array-shape decoding is required (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAudClaim_Matches`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_test.go#L482) | unit/verify | unproven |
| positive | [`TestVerifyJWT_AudienceArrayForm`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_test.go#L278) | unit/verify | unproven |

### [`RFC8707-2-1`](#rfc8707-2-1)

Fragment MUST be absent from the canonical resource URL (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8707-2-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc8707.txt |
| Source fingerprint | 3b04fcfbb0d22e5a |
| Record | rfc/extraction/rfc8707.json |
| Mapped sentences | 1 |
| Declined as scope | 4 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 2 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 1 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust Legal Provisions boilerplate on the front page: it governs Code Components extracted from the document, not any protocol behavior Ze implements. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction prose describing the security assumption the resource parameter exists to protect, not an obligation placed on an implementation. The resource-server obligation it motivates is declared as RFC8707-5-1 in rfc/short/rfc8707.md and sourced from Section 3. | To prevent misuse, several important security assumptions must hold, one of which is that an access token must only be valid for use at a specific protected resource and for a specific scope of access. |
| `2:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | The obligation shapes the value of the resource request parameter, and sending that parameter is optional: RFC 8707 Section 2 says "In requests to the authorization server, a client MAY indicate the protected resource (a.k.a. resource server, application, API, etc.) to which it is requesting access by including the following parameter in the request." Ze is an OAuth resource server (internal/component/mcp) that issues no authorization or token requests, so it never forms a resource parameter value. The same scope decision gates RFC8707-2-1 and RFC8707-2-2 in rfc/short/rfc8707.md. | Its value MUST be an absolute URI, as specified by Section 4.3 of [RFC3986]. |
| `2.2:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The sentence restates an RFC 6749 obligation it cites by section: "As specified in Section 5.1 of [RFC6749], the authorization server must indicate the access token's effective scope to the client in the scope response parameter value when it differs from the scope requested by the client." The obligation belongs to RFC 6749 Section 5.1. | As specified in Section 5.1 of [RFC6749], the authorization server must indicate the access token's effective scope to the client in the "scope" response parameter value when it differs from the scope requested by the client. |

## Superseded

No document obsoletes RFC 8707, so its obligations are stated where they were written.
