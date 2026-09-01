# RFC 8414 - OAuth 2.0 Authorization Server Metadata

Partial. Every requirement this repository extracted from RFC 8414, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 66.7% | 2 of 3 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 3 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 3 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 4 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 7 | of 9 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 4 | of 7 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 33.3% | 1 of 3 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 3 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 9 |
| Gated MUST-level | 7 |
| Obligations that bind Ze | 3 |
| Not applicable, so out of scope | 4 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tagged units | 4 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8414.md` |
| Requirement shard | `rfc/requirements/rfc8414.md` |
| RFC text | `rfc/full/rfc8414.txt` |

## Enrolment

Enrolled: OAuth 2.0 Authorization Server Metadata (ze as an OAuth resource server): seven MUST-level requirements. 2-1 (the metadata carries an issuer) and 3.3-2 (the metadata issuer matches the configured authorization server) are met with positive+negative tags in internal/component/mcp. 3-1 (publish the .well-known endpoint), 2-2, 2-3, 2-4 (authorization_endpoint, token_endpoint, response_types_supported) are {not-applicable}: ze is a resource server that consumes only issuer and jwks_uri, not an authorization-server publisher or a client. 3.3-1 (fetch the metadata over TLS) is {gap}: ze admits an http:// scheme for the fetch URL (internal/component/mcp/streamable_auth.go:122-123). Disclosed in the docs/features/rfc-status.md RFC 8414 row.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

As an OAuth resource server, ze fetches the AS metadata document, requires `issuer` and matches it against the token `iss`, and reads `jwks_uri` for verification keys ([`internal/component/mcp/as_metadata.go`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata.go), `streamable_auth.go`). Client-only fields (authorization_endpoint, token_endpoint, response_types_supported) are not consumed. Tests bound per requirement in [`rfc/requirements/rfc8414.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc8414.md).

**What the ledger says remains:**

One MUST gap gated in [`rfc/short/rfc8414.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8414.md): ze does not force the AS metadata fetch URL to https; [`internal/component/mcp/streamable_auth.go`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/streamable_auth.go) admits an http:// scheme, leaving transport security to the operator-configured scheme.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **7** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC8414-2-1`](#rfc8414-2-1), [`RFC8414-3.3-2`](#rfc8414-3.3-2)

**Annotated instead of tested (5):** [`RFC8414-3-1`](#rfc8414-3-1), [`RFC8414-3.3-1`](#rfc8414-3.3-1), [`RFC8414-2-2`](#rfc8414-2-2), [`RFC8414-2-3`](#rfc8414-2-3), [`RFC8414-2-4`](#rfc8414-2-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8414-2-1` | The `issuer` field is required in the metadata document (§2) | MUST | 2 | **positive:** `unit/verify` [`TestFetchASMetadata_Success`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_test.go#L56). **negative:** `unit/verify` [`TestFetchASMetadata_MissingIssuer`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_test.go#L75) |
| `RFC8414-3-1` | The well-known endpoint must be reachable without authentication (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is an authorization-server publishing obligation; ze is an OAuth resource server that only consumes AS metadata (internal/component/mcp/as_metadata.go fetches and reads issuer/jwks_uri) and publishes no RFC 8414 metadata endpoint |
| `RFC8414-3.3-1` | Fetching metadata must use TLS in production (§3.3, §6) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze does not force the RFC 8414 authorization-server metadata fetch URL to https; internal/component/mcp/streamable_auth.go:122-123 admits an http:// scheme, leaving transport security to the operator-configured scheme |
| `RFC8414-3.3-2` | The `issuer` field in the metadata document must match the base URL used to fetch it (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestNewStreamable_OAuth_AcceptsValidToken`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L141). **negative:** `unit/verify` [`TestNewStreamable_OAuth_RejectsIssuerMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L130) |
| `RFC8414-2-2` | `authorization_endpoint` is required for clients (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** authorization_endpoint is consumed by an OAuth client to drive the authorization/token flow; ze is a resource server that reads only issuer and jwks_uri from the AS metadata (internal/component/mcp/as_metadata.go) and does not act as a client |
| `RFC8414-2-3` | `token_endpoint` is required for clients (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** token_endpoint is consumed by an OAuth client to drive the authorization/token flow; ze is a resource server that reads only issuer and jwks_uri from the AS metadata (internal/component/mcp/as_metadata.go) and does not act as a client |
| `RFC8414-2-4` | `response_types_supported` is required (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** response_types_supported is consumed by an OAuth client to drive the authorization/token flow; ze is a resource server that reads only issuer and jwks_uri from the AS metadata (internal/component/mcp/as_metadata.go) and does not act as a client |
| `RFC8414-2-5` | `jwks_uri` should be present in the metadata document (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-2-6` | `scopes_supported` is recommended to be present (§2) | RECOMMENDED | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8414-3-1`](#rfc8414-3-1) The well-known endpoint must be reachable without authentication (§3) | no test | no test carries this requirement id; annotated {not-applicable}: this is an authorization-server publishing obligation; ze is an OAuth resource server that only consumes AS metadata (internal/component/mcp/as_metadata.go fetches and reads issuer/jwks_uri) and publishes no RFC 8414 metadata endpoint |
| [`RFC8414-3.3-1`](#rfc8414-3.3-1) Fetching metadata must use TLS in production (§3.3, §6) | {gap}, no test | ze does not force the RFC 8414 authorization-server metadata fetch URL to https; internal/component/mcp/streamable_auth.go:122-123 admits an http:// scheme, leaving transport security to the operator-configured scheme |
| [`RFC8414-2-2`](#rfc8414-2-2) `authorization_endpoint` is required for clients (§2) | no test | no test carries this requirement id; annotated {not-applicable}: authorization_endpoint is consumed by an OAuth client to drive the authorization/token flow; ze is a resource server that reads only issuer and jwks_uri from the AS metadata (internal/component/mcp/as_metadata.go) and does not act as a client |
| [`RFC8414-2-3`](#rfc8414-2-3) `token_endpoint` is required for clients (§2) | no test | no test carries this requirement id; annotated {not-applicable}: token_endpoint is consumed by an OAuth client to drive the authorization/token flow; ze is a resource server that reads only issuer and jwks_uri from the AS metadata (internal/component/mcp/as_metadata.go) and does not act as a client |
| [`RFC8414-2-4`](#rfc8414-2-4) `response_types_supported` is required (§2) | no test | no test carries this requirement id; annotated {not-applicable}: response_types_supported is consumed by an OAuth client to drive the authorization/token flow; ze is a resource server that reads only issuer and jwks_uri from the AS metadata (internal/component/mcp/as_metadata.go) and does not act as a client |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8414-2-1`](#rfc8414-2-1)

The `issuer` field is required in the metadata document (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFetchASMetadata_MissingIssuer`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_test.go#L75) | unit/verify | unproven |
| positive | [`TestFetchASMetadata_Success`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_test.go#L56) | unit/verify | unproven |

### [`RFC8414-3-1`](#rfc8414-3-1)

The well-known endpoint must be reachable without authentication (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-3-1, so no unit is bound to it.

### [`RFC8414-3.3-1`](#rfc8414-3.3-1)

Fetching metadata must use TLS in production (§3.3, §6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-3.3-1, so no unit is bound to it.

### [`RFC8414-3.3-2`](#rfc8414-3.3-2)

The `issuer` field in the metadata document must match the base URL used to fetch it (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNewStreamable_OAuth_RejectsIssuerMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L130) | unit/verify | unproven |
| positive | [`TestNewStreamable_OAuth_AcceptsValidToken`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L141) | unit/verify | unproven |

### [`RFC8414-2-2`](#rfc8414-2-2)

`authorization_endpoint` is required for clients (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2-2, so no unit is bound to it.

### [`RFC8414-2-3`](#rfc8414-2-3)

`token_endpoint` is required for clients (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2-3, so no unit is bound to it.

### [`RFC8414-2-4`](#rfc8414-2-4)

`response_types_supported` is required (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2-4, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 8414, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 8414, so its obligations are stated where they were written.
