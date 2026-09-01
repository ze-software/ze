# RFC 8707 - Resource Indicators for OAuth 2.0

No row in the public ledger. Every requirement this repository extracted from RFC 8707, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 3 of 3 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 3 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 3 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 3 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 7 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 7 | of 9 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 4 | of 7 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 3 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 9 |
| Gated MUST-level | 7 |
| Obligations that bind Ze | 3 |
| Not applicable, so out of scope | 4 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 7 |
| Tagged units | 7 |
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
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **7** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC8707-5-1`](#rfc8707-5-1), [`RFC8707-5-2`](#rfc8707-5-2), [`RFC8707-2-3`](#rfc8707-2-3)

**Annotated instead of tested (4):** [`RFC8707-3-1`](#rfc8707-3-1), [`RFC8707-3-2`](#rfc8707-3-2), [`RFC8707-2-1`](#rfc8707-2-1), [`RFC8707-2-2`](#rfc8707-2-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8707-3-1` | Access token's `aud` claim MUST be the canonical resource URL(s) (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** setting the token audience is an authorization-server obligation performed when minting a token; ze is an OAuth resource server (internal/component/mcp) and never issues or signs tokens, so it has no audience-setting code path |
| `RFC8707-3-2` | AS MUST respond with `invalid_target` when it will not issue a token for the requested resource (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the invalid_target error is an authorization-server token-endpoint behavior; ze has no token or authorization endpoint (grep for invalid_target in internal/component/mcp finds none) |
| `RFC8707-5-1` | Resource server MUST reject any token whose `aud` does not match the resource's own canonical URL (§5) | MUST | 5 | **positive:** `unit/verify` [`TestNewStreamable_OAuth_AcceptsSlashDivergentAudience`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L410). **negative:** `unit/verify` [`TestVerifyJWT_RejectAudienceMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_test.go#L268) |
| `RFC8707-5-2` | If `aud` is a JSON array, at least one entry MUST match; array-shape decoding is required (§5) | MUST | 5 | **positive:** `unit/verify` [`TestVerifyJWT_AudienceArrayForm`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_test.go#L278). **negative:** `unit/verify` [`TestAudClaim_Matches`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_test.go#L482) |
| `RFC8707-2-1` | Fragment MUST be absent from the canonical resource URL (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this governs the canonical resource URI a client forms and sends as the resource request parameter; ze is a resource server that never emits or consumes that parameter (internal/component/mcp validates the token audience only) |
| `RFC8707-2-2` | Query MUST be absent from the canonical resource URL form (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this governs the client-formed canonical resource URI; ze (resource server) never emits or consumes the resource request parameter, validating only the token audience |
| `RFC8707-2-3` | Trailing slash usage MUST be consistent on both sides (§2) | MUST | 2 | **positive:** `unit/verify` [`TestAudClaim_MatchesCanonicalVariants`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L356). **positive:** `unit/verify` [`TestNewStreamable_OAuth_AcceptsSlashDivergentAudience`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L411). **negative:** `unit/verify` [`TestAudClaim_MatchesCanonicalVariants`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L370) |
| `RFC8707-2-4` | `resource` parameter MAY appear on the token endpoint (§2, §4) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8707-2-5` | Multiple `resource` values MAY appear; AS policy decides (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8707-3-1`](#rfc8707-3-1) Access token's `aud` claim MUST be the canonical resource URL(s) (§3) | no test | no test carries this requirement id; annotated {not-applicable}: setting the token audience is an authorization-server obligation performed when minting a token; ze is an OAuth resource server (internal/component/mcp) and never issues or signs tokens, so it has no audience-setting code path |
| [`RFC8707-3-2`](#rfc8707-3-2) AS MUST respond with `invalid_target` when it will not issue a token for the requested resource (§3) | no test | no test carries this requirement id; annotated {not-applicable}: the invalid_target error is an authorization-server token-endpoint behavior; ze has no token or authorization endpoint (grep for invalid_target in internal/component/mcp finds none) |
| [`RFC8707-2-1`](#rfc8707-2-1) Fragment MUST be absent from the canonical resource URL (§2) | no test | no test carries this requirement id; annotated {not-applicable}: this governs the canonical resource URI a client forms and sends as the resource request parameter; ze is a resource server that never emits or consumes that parameter (internal/component/mcp validates the token audience only) |
| [`RFC8707-2-2`](#rfc8707-2-2) Query MUST be absent from the canonical resource URL form (§2) | no test | no test carries this requirement id; annotated {not-applicable}: this governs the client-formed canonical resource URI; ze (resource server) never emits or consumes the resource request parameter, validating only the token audience |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8707-3-1`](#rfc8707-3-1)

Access token's `aud` claim MUST be the canonical resource URL(s) (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8707-3-1, so no unit is bound to it.

### [`RFC8707-3-2`](#rfc8707-3-2)

AS MUST respond with `invalid_target` when it will not issue a token for the requested resource (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8707-3-2, so no unit is bound to it.

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

### [`RFC8707-2-2`](#rfc8707-2-2)

Query MUST be absent from the canonical resource URL form (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8707-2-2, so no unit is bound to it.

### [`RFC8707-2-3`](#rfc8707-2-3)

Trailing slash usage MUST be consistent on both sides (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAudClaim_MatchesCanonicalVariants`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L370) | unit/verify | unproven |
| positive | [`TestAudClaim_MatchesCanonicalVariants`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L356) | unit/verify | unproven |
| positive | [`TestNewStreamable_OAuth_AcceptsSlashDivergentAudience`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L411) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 8707, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 8707, so its obligations are stated where they were written.
