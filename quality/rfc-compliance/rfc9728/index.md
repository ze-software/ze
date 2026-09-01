# RFC 9728 - OAuth 2.0 Protected Resource Metadata

No row in the public ledger. Every requirement this repository extracted from RFC 9728, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 40.0% | 2 of 5 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 60.0% | 3 of 5 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 5 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 5 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 10 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 7 | of 11 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 7 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 5 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 11 |
| Gated MUST-level | 7 |
| Obligations that bind Ze | 5 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 10 |
| Tagged units | 10 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9728.md` |
| Requirement shard | `rfc/requirements/rfc9728.md` |
| RFC text | `rfc/full/rfc9728.txt` |

## Enrolment

Enrolled: OAuth 2.0 Protected Resource Metadata (ze publishes the protected-resource metadata as an OAuth resource server): seven MUST-level requirements. 3.1-1 (the metadata endpoint is reachable without authentication) and 5.1-2 (the resource_metadata URL is a well-formed absolute URL) carry positive+negative tags in internal/component/mcp. 2-1 (metadata contains resource), 2-2 (metadata contains authorization_servers), and 5.1-1 (a 401 WWW-Authenticate carries resource_metadata) are {single-polarity: positive}: each field is emitted unconditionally. 2-3 (a client ignores unknown fields) and 3-1 (metadata TLS matches the resource) are {not-applicable}: ze is the publisher not a consumer, and the metadata endpoint shares the resource's http.Server so its scheme cannot diverge.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 9728.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **7** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC9728-3.1-1`](#rfc9728-3.1-1), [`RFC9728-5.1-2`](#rfc9728-5.1-2)

**Annotated instead of tested (5):** [`RFC9728-2-1`](#rfc9728-2-1), [`RFC9728-2-2`](#rfc9728-2-2), [`RFC9728-2-3`](#rfc9728-2-3), [`RFC9728-3-1`](#rfc9728-3-1), [`RFC9728-5.1-1`](#rfc9728-5.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9728-2-1` | The `resource` field MUST be present in the metadata document and contain the canonical URL identifying the resource (§2) | MUST | 2 | **positive:** `unit/verify` [`TestNewStreamable_OAuth_MetadataEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L290). **positive:** `unit/verify` [`TestResourceMetadata_Document`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_test.go#L212). **negative:** no negative test. **{single-polarity}:** the resource field is emitted unconditionally by writeResourceMetadata (internal/component/mcp/oauth.go:170-171,184-192), so no input can make it absent and there is no negative to assert |
| `RFC9728-2-2` | The `authorization_servers` field MUST be present as a JSON array of AS issuer URLs (§2) | MUST | 2 | **positive:** `unit/verify` [`TestNewStreamable_OAuth_MetadataEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L294). **positive:** `unit/verify` [`TestResourceMetadata_Document`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_test.go#L216). **negative:** no negative test. **{single-polarity}:** authorization_servers is emitted unconditionally by writeResourceMetadata (internal/component/mcp/oauth.go:172,190), so no input can make it absent and there is no negative to assert |
| `RFC9728-2-3` | Unknown fields in the metadata document MUST be ignored by clients (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this obligation binds an OAuth client consuming the metadata to ignore unrecognized members; ze is the protected-resource-metadata publisher (internal/component/mcp/oauth.go:169-179 emits only defined fields) and does not consume the document |
| `RFC9728-3.1-1` | The well-known endpoint MUST be reachable without authentication (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestNewStreamable_OAuth_MetadataEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L282). **negative:** `unit/verify` [`TestStreamable_MetadataEndpoint_Gated`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_test.go#L278) |
| `RFC9728-3-1` | The metadata document MUST be served over TLS when the resource itself is reached over TLS; plaintext HTTP only acceptable on loopback (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the metadata endpoint is served by the same http.Server as the protected resource (internal/component/mcp/streamable.go), so its transport scheme is identical to the resource's by construction; TLS is operator listener configuration (cmd/ze/hub/service_mcp.go:207-224), not an internal/component/mcp producer |
| `RFC9728-5.1-1` | 401 response MUST include `WWW-Authenticate: Bearer` with `resource_metadata` parameter (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestNewStreamable_OAuth_RejectsMissingBearer`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L216). **positive:** `unit/verify` [`TestOAuth_Authenticate_MissingHeader`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_test.go#L48). **negative:** no negative test. **{single-polarity}:** every challengeError sets ResourceMetadata (internal/component/mcp/oauth.go:122) and WWWAuthenticate always appends it when set (internal/component/mcp/auth.go:177), so the 401 carries resource_metadata by construction and there is no negative to assert |
| `RFC9728-5.1-2` | The `resource_metadata` URL in the WWW-Authenticate header MUST be absolute (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestResourceMetadataURL_RejectsMalformedBase`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L827). **negative:** `unit/verify` [`TestResourceMetadataURL_RejectsMalformedBase`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L818) |
| `RFC9728-3.2-1` | Resource servers SHOULD return a short max-age in Cache-Control (e.g., 300 s) (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-2-4` | The `scopes_supported` field MAY be included as a JSON array of scope names (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-2-5` | The `bearer_methods_supported` field MAY be included as an array of delivery methods (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-3.2-2` | Clients MAY cache the metadata document (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9728-2-3`](#rfc9728-2-3) Unknown fields in the metadata document MUST be ignored by clients (§2) | no test | no test carries this requirement id; annotated {not-applicable}: this obligation binds an OAuth client consuming the metadata to ignore unrecognized members; ze is the protected-resource-metadata publisher (internal/component/mcp/oauth.go:169-179 emits only defined fields) and does not consume the document |
| [`RFC9728-3-1`](#rfc9728-3-1) The metadata document MUST be served over TLS when the resource itself is reached over TLS; plaintext HTTP only acceptable on loopback (§3) | no test | no test carries this requirement id; annotated {not-applicable}: the metadata endpoint is served by the same http.Server as the protected resource (internal/component/mcp/streamable.go), so its transport scheme is identical to the resource's by construction; TLS is operator listener configuration (cmd/ze/hub/service_mcp.go:207-224), not an internal/component/mcp producer |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9728-2-1`](#rfc9728-2-1)

The `resource` field MUST be present in the metadata document and contain the canonical URL identifying the resource (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestNewStreamable_OAuth_MetadataEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L290) | unit/verify | unproven |
| positive | [`TestResourceMetadata_Document`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_test.go#L212) | unit/verify | unproven |

### [`RFC9728-2-2`](#rfc9728-2-2)

The `authorization_servers` field MUST be present as a JSON array of AS issuer URLs (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestNewStreamable_OAuth_MetadataEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L294) | unit/verify | unproven |
| positive | [`TestResourceMetadata_Document`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_test.go#L216) | unit/verify | unproven |

### [`RFC9728-2-3`](#rfc9728-2-3)

Unknown fields in the metadata document MUST be ignored by clients (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-2-3, so no unit is bound to it.

### [`RFC9728-3.1-1`](#rfc9728-3.1-1)

The well-known endpoint MUST be reachable without authentication (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestStreamable_MetadataEndpoint_Gated`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_test.go#L278) | unit/verify | unproven |
| positive | [`TestNewStreamable_OAuth_MetadataEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L282) | unit/verify | unproven |

### [`RFC9728-3-1`](#rfc9728-3-1)

The metadata document MUST be served over TLS when the resource itself is reached over TLS; plaintext HTTP only acceptable on loopback (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-3-1, so no unit is bound to it.

### [`RFC9728-5.1-1`](#rfc9728-5.1-1)

401 response MUST include `WWW-Authenticate: Bearer` with `resource_metadata` parameter (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestNewStreamable_OAuth_RejectsMissingBearer`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L216) | unit/verify | unproven |
| positive | [`TestOAuth_Authenticate_MissingHeader`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_test.go#L48) | unit/verify | unproven |

### [`RFC9728-5.1-2`](#rfc9728-5.1-2)

The `resource_metadata` URL in the WWW-Authenticate header MUST be absolute (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestResourceMetadataURL_RejectsMalformedBase`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L818) | unit/verify | unproven |
| positive | [`TestResourceMetadataURL_RejectsMalformedBase`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L827) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 9728, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 9728, so its obligations are stated where they were written.
