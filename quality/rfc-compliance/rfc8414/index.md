# RFC 8414 - OAuth 2.0 Authorization Server Metadata

Partial. Every requirement this repository extracted from RFC 8414, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 6.7% | 2 of 30 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 30 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 30 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 4 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 30 | of 32 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 4 | of 30 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 13.3% | 4 of 30 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 30 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 30 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 80.0% | 24 of 30 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 30 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 32 |
| Gated MUST-level | 30 |
| Not applicable, so out of scope | 4 |
| Declared gaps | 1 |
| Gated with no test | 23 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tagged units | 4 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8414.md` |
| Requirement shard | `rfc/requirements/rfc8414.md` |
| RFC text | `rfc/full/rfc8414.txt` |

## Enrolment

Enrolled: OAuth 2.0 Authorization Server Metadata (ze as an OAuth resource server): 30 MUST-level requirements after the 2026-09-21 extraction walk. 2-1 (the metadata carries an issuer) and 3.3-2 (the metadata issuer matches the configured authorization server, and a mismatched response is not used) are met with positive+negative tags in internal/component/mcp. 3-1 (publish the metadata document at the well-known path), 2-2, 2-3, 2-4 (authorization_endpoint, token_endpoint, response_types_supported) are {not-applicable}: ze is a resource server that consumes only issuer and jwks_uri, not an authorization-server publisher or a client. 3.3-1 (https metadata path and TLS confidentiality) is {gap}: ze admits an http:// scheme for the fetch URL (internal/component/mcp/streamable_auth.go:122-123). The walk added 23 further MUST-level rows the summary had never listed (2-7 through 2-15, 2.1-1, 2.1-2, 3-2, 3-3, 3.1-1, 3.1-2, 3.2-1, 3.2-2, 4-1 through 4-3, and 6.1-1 through 6.1-3); none carries a test yet. Disclosed in the docs/features/rfc-status.md RFC 8414 row.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

As an OAuth resource server, ze fetches the AS metadata document, requires `issuer` and matches it against the token `iss`, and reads `jwks_uri` for verification keys ([`internal/component/mcp/as_metadata.go`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata.go), `streamable_auth.go`). Client-only fields (authorization_endpoint, token_endpoint, response_types_supported) are not consumed. Tests bound per requirement in [`rfc/requirements/rfc8414.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc8414.md).

**What the ledger says remains**

One annotated MUST gap: ze does not force the AS metadata fetch URL to https; [`internal/component/mcp/streamable_auth.go`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/streamable_auth.go) admits an http:// scheme, leaving transport security to the operator-configured scheme (3.3-1). In addition, the 2026-09-21 extraction walk put 23 MUST-level rows on the checklist that no test covers. Three of them bind ze in its resource-server role and are the ones to write next: 3.1-1 (query the document with an HTTP GET at the well-known path), 3.1-2 (strip a terminating "/" from an issuer identifier that carries a path component before inserting the suffix), and 4-1 through 4-3 (compare the issuer as Unicode code points, with no normalization applied). The remainder (2-7 through 2-15, 2.1-1, 2.1-2, 3-2, 3-3, 3.2-1, 3.2-2, 6.1-1 through 6.1-3) bind the authorization server as publisher or depend on features ze does not offer; they are unannotated, because an annotation on a row is the owner's to write.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 23 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **30** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC8414-2-1`](#rfc8414-2-1), [`RFC8414-3.3-2`](#rfc8414-3.3-2)

**Annotated instead of tested (5):** [`RFC8414-3-1`](#rfc8414-3-1), [`RFC8414-3.3-1`](#rfc8414-3.3-1), [`RFC8414-2-2`](#rfc8414-2-2), [`RFC8414-2-3`](#rfc8414-2-3), [`RFC8414-2-4`](#rfc8414-2-4)

**No test and no annotation (23):** [`RFC8414-2-7`](#rfc8414-2-7), [`RFC8414-2-8`](#rfc8414-2-8), [`RFC8414-2-9`](#rfc8414-2-9), [`RFC8414-2-10`](#rfc8414-2-10), [`RFC8414-2-11`](#rfc8414-2-11), [`RFC8414-2-12`](#rfc8414-2-12), [`RFC8414-2-13`](#rfc8414-2-13), [`RFC8414-2-14`](#rfc8414-2-14), [`RFC8414-2-15`](#rfc8414-2-15), [`RFC8414-2.1-1`](#rfc8414-2.1-1), [`RFC8414-2.1-2`](#rfc8414-2.1-2), [`RFC8414-3-2`](#rfc8414-3-2), [`RFC8414-3-3`](#rfc8414-3-3), [`RFC8414-3.1-1`](#rfc8414-3.1-1), [`RFC8414-3.1-2`](#rfc8414-3.1-2), [`RFC8414-3.2-1`](#rfc8414-3.2-1), [`RFC8414-3.2-2`](#rfc8414-3.2-2), [`RFC8414-4-1`](#rfc8414-4-1), [`RFC8414-4-2`](#rfc8414-4-2), [`RFC8414-4-3`](#rfc8414-4-3), [`RFC8414-6.1-1`](#rfc8414-6.1-1), [`RFC8414-6.1-2`](#rfc8414-6.1-2), [`RFC8414-6.1-3`](#rfc8414-6.1-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8414-2-1` | `issuer` is REQUIRED in the metadata document (§2) | MUST | 2 | **positive:** `unit/verify` [`TestFetchASMetadata_Success`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_test.go#L56). **negative:** `unit/verify` [`TestFetchASMetadata_MissingIssuer`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_test.go#L75) |
| `RFC8414-3-1` | Authorization servers supporting metadata MUST make the Section 2 metadata JSON document available at a path formed by inserting a well-known URI string into the issuer identifier between the host component and the path component (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is an authorization-server publishing obligation; ze is an OAuth resource server that only consumes AS metadata (internal/component/mcp/as_metadata.go fetches and reads issuer/jwks_uri) and publishes no RFC 8414 metadata endpoint |
| `RFC8414-3.3-1` | The metadata path MUST use the "https" scheme, which RFC 8414 states in Section 3, and confidentiality protection MUST be applied using TLS with a ciphersuite that provides confidentiality and integrity protection, which it states in Section 6.1. Both are preconditions of the metadata validation this id is anchored to (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze does not force the RFC 8414 authorization-server metadata fetch URL to https; internal/component/mcp/streamable_auth.go:122-123 admits an http:// scheme, leaving transport security to the operator-configured scheme |
| `RFC8414-3.3-2` | The `issuer` value returned MUST be identical to the issuer identifier value into which the well-known URI string was inserted to create the URL used to retrieve the metadata, and if they are not identical the data in the response MUST NOT be used (§3.3, §6.2) | MUST | 3.3 | **positive:** `unit/verify` [`TestNewStreamable_OAuth_AcceptsValidToken`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L141). **negative:** `unit/verify` [`TestNewStreamable_OAuth_RejectsIssuerMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L130) |
| `RFC8414-2-2` | `authorization_endpoint` is REQUIRED unless no grant types are supported that use the authorization endpoint (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is an authorization-server publishing obligation; ze is an OAuth resource server that reads only issuer and jwks_uri from the AS metadata (internal/component/mcp/as_metadata.go), publishes no RFC 8414 metadata endpoint, and does not act as a client |
| `RFC8414-2-3` | `token_endpoint` is REQUIRED unless only the implicit grant type is supported (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is an authorization-server publishing obligation; ze is an OAuth resource server that reads only issuer and jwks_uri from the AS metadata (internal/component/mcp/as_metadata.go), publishes no RFC 8414 metadata endpoint, and does not act as a client |
| `RFC8414-2-4` | `response_types_supported` is REQUIRED (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is an authorization-server publishing obligation; ze is an OAuth resource server that reads only issuer and jwks_uri from the AS metadata (internal/component/mcp/as_metadata.go), publishes no RFC 8414 metadata endpoint, and does not act as a client |
| `RFC8414-2-5` | `jwks_uri` is OPTIONAL; the referenced document contains the signing key(s) the client uses to validate signatures from the authorization server (§2) | OPTIONAL | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-2-6` | `scopes_supported` is RECOMMENDED (§2) | RECOMMENDED | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-2-7` | The `jwks_uri` URL MUST use the "https" scheme (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-2-8` | When both signing and encryption keys are made available, a "use" (public key use) parameter value is REQUIRED for all keys in the referenced JWK Set to indicate each key's intended usage (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-2-9` | `token_endpoint_auth_signing_alg_values_supported` MUST be present if "private_key_jwt" or "client_secret_jwt" is specified in `token_endpoint_auth_methods_supported` (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-2-10` | The value "none" MUST NOT be used in `token_endpoint_auth_signing_alg_values_supported` (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-2-11` | `revocation_endpoint_auth_signing_alg_values_supported` MUST be present if "private_key_jwt" or "client_secret_jwt" is specified in `revocation_endpoint_auth_methods_supported` (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-2-12` | The value "none" MUST NOT be used in `revocation_endpoint_auth_signing_alg_values_supported` (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-2-13` | If `introspection_endpoint_auth_methods_supported` is omitted, the set of supported authentication methods MUST be determined by other means (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-2-14` | `introspection_endpoint_auth_signing_alg_values_supported` MUST be present if "private_key_jwt" or "client_secret_jwt" is specified in `introspection_endpoint_auth_methods_supported` (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-2-15` | The value "none" MUST NOT be used in `introspection_endpoint_auth_signing_alg_values_supported` (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-2.1-1` | Signed metadata MUST be digitally signed or MACed using JSON Web Signature and MUST contain an "iss" (issuer) claim denoting the party attesting to the claims in the signed metadata (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-2.1-2` | If the consumer of the metadata supports signed metadata, metadata values conveyed in the signed metadata MUST take precedence over the corresponding values conveyed using plain JSON elements (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-3-2` | The well-known URI suffix used MUST be registered in the IANA "Well-Known URIs" registry (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-3-3` | An OAuth 2.0 application using this specification MUST specify what well-known URI suffix it will use for this purpose (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-3.1-1` | An authorization server metadata document MUST be queried using an HTTP "GET" request at the previously specified path (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-3.1-2` | If the issuer identifier value contains a path component, any terminating "/" MUST be removed before inserting "/.well-known/" and the well-known URI suffix between the host component and the path component (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-3.2-1` | A successful response MUST use the 200 OK HTTP status code and return a JSON object using the "application/json" content type that contains a set of claims that are a subset of the metadata values defined in Section 2 (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-3.2-2` | Claims with zero elements MUST be omitted from the response (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-4-1` | Comparisons between JSON strings and other Unicode strings MUST be performed as specified in Section 4 (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-4-2` | Unicode Normalization MUST NOT be applied at any point to either the JSON string or the string it is to be compared against (§4) | MUST NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-4-3` | Comparisons between the two strings MUST be performed as a Unicode code-point-to-code-point equality comparison (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-6.1-1` | Implementations MUST support TLS (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-6.1-2` | The authorization server MUST support TLS version 1.2 (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-6.1-3` | When using TLS, the client MUST perform a TLS/SSL server certificate check, per RFC 6125 (§6.1, §6.2) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8414-3-1`](#rfc8414-3-1) Authorization servers supporting metadata MUST make the Section 2 metadata JSON document available at a path formed by inserting a well-known URI string into the issuer identifier between the host component and the path component (§3) | no test | no test carries this requirement id; annotated {not-applicable}: this is an authorization-server publishing obligation; ze is an OAuth resource server that only consumes AS metadata (internal/component/mcp/as_metadata.go fetches and reads issuer/jwks_uri) and publishes no RFC 8414 metadata endpoint |
| [`RFC8414-3.3-1`](#rfc8414-3.3-1) The metadata path MUST use the "https" scheme, which RFC 8414 states in Section 3, and confidentiality protection MUST be applied using TLS with a ciphersuite that provides confidentiality and integrity protection, which it states in Section 6.1. Both are preconditions of the metadata validation this id is anchored to (§3.3) | {gap}, no test | ze does not force the RFC 8414 authorization-server metadata fetch URL to https; internal/component/mcp/streamable_auth.go:122-123 admits an http:// scheme, leaving transport security to the operator-configured scheme |
| [`RFC8414-2-2`](#rfc8414-2-2) `authorization_endpoint` is REQUIRED unless no grant types are supported that use the authorization endpoint (§2) | no test | no test carries this requirement id; annotated {not-applicable}: this is an authorization-server publishing obligation; ze is an OAuth resource server that reads only issuer and jwks_uri from the AS metadata (internal/component/mcp/as_metadata.go), publishes no RFC 8414 metadata endpoint, and does not act as a client |
| [`RFC8414-2-3`](#rfc8414-2-3) `token_endpoint` is REQUIRED unless only the implicit grant type is supported (§2) | no test | no test carries this requirement id; annotated {not-applicable}: this is an authorization-server publishing obligation; ze is an OAuth resource server that reads only issuer and jwks_uri from the AS metadata (internal/component/mcp/as_metadata.go), publishes no RFC 8414 metadata endpoint, and does not act as a client |
| [`RFC8414-2-4`](#rfc8414-2-4) `response_types_supported` is REQUIRED (§2) | no test | no test carries this requirement id; annotated {not-applicable}: this is an authorization-server publishing obligation; ze is an OAuth resource server that reads only issuer and jwks_uri from the AS metadata (internal/component/mcp/as_metadata.go), publishes no RFC 8414 metadata endpoint, and does not act as a client |
| [`RFC8414-2-7`](#rfc8414-2-7) The `jwks_uri` URL MUST use the "https" scheme (§2) | no test | no test carries this requirement id |
| [`RFC8414-2-8`](#rfc8414-2-8) When both signing and encryption keys are made available, a "use" (public key use) parameter value is REQUIRED for all keys in the referenced JWK Set to indicate each key's intended usage (§2) | no test | no test carries this requirement id |
| [`RFC8414-2-9`](#rfc8414-2-9) `token_endpoint_auth_signing_alg_values_supported` MUST be present if "private_key_jwt" or "client_secret_jwt" is specified in `token_endpoint_auth_methods_supported` (§2) | no test | no test carries this requirement id |
| [`RFC8414-2-10`](#rfc8414-2-10) The value "none" MUST NOT be used in `token_endpoint_auth_signing_alg_values_supported` (§2) | no test | no test carries this requirement id |
| [`RFC8414-2-11`](#rfc8414-2-11) `revocation_endpoint_auth_signing_alg_values_supported` MUST be present if "private_key_jwt" or "client_secret_jwt" is specified in `revocation_endpoint_auth_methods_supported` (§2) | no test | no test carries this requirement id |
| [`RFC8414-2-12`](#rfc8414-2-12) The value "none" MUST NOT be used in `revocation_endpoint_auth_signing_alg_values_supported` (§2) | no test | no test carries this requirement id |
| [`RFC8414-2-13`](#rfc8414-2-13) If `introspection_endpoint_auth_methods_supported` is omitted, the set of supported authentication methods MUST be determined by other means (§2) | no test | no test carries this requirement id |
| [`RFC8414-2-14`](#rfc8414-2-14) `introspection_endpoint_auth_signing_alg_values_supported` MUST be present if "private_key_jwt" or "client_secret_jwt" is specified in `introspection_endpoint_auth_methods_supported` (§2) | no test | no test carries this requirement id |
| [`RFC8414-2-15`](#rfc8414-2-15) The value "none" MUST NOT be used in `introspection_endpoint_auth_signing_alg_values_supported` (§2) | no test | no test carries this requirement id |
| [`RFC8414-2.1-1`](#rfc8414-2.1-1) Signed metadata MUST be digitally signed or MACed using JSON Web Signature and MUST contain an "iss" (issuer) claim denoting the party attesting to the claims in the signed metadata (§2.1) | no test | no test carries this requirement id |
| [`RFC8414-2.1-2`](#rfc8414-2.1-2) If the consumer of the metadata supports signed metadata, metadata values conveyed in the signed metadata MUST take precedence over the corresponding values conveyed using plain JSON elements (§2.1) | no test | no test carries this requirement id |
| [`RFC8414-3-2`](#rfc8414-3-2) The well-known URI suffix used MUST be registered in the IANA "Well-Known URIs" registry (§3) | no test | no test carries this requirement id |
| [`RFC8414-3-3`](#rfc8414-3-3) An OAuth 2.0 application using this specification MUST specify what well-known URI suffix it will use for this purpose (§3) | no test | no test carries this requirement id |
| [`RFC8414-3.1-1`](#rfc8414-3.1-1) An authorization server metadata document MUST be queried using an HTTP "GET" request at the previously specified path (§3.1) | no test | no test carries this requirement id |
| [`RFC8414-3.1-2`](#rfc8414-3.1-2) If the issuer identifier value contains a path component, any terminating "/" MUST be removed before inserting "/.well-known/" and the well-known URI suffix between the host component and the path component (§3.1) | no test | no test carries this requirement id |
| [`RFC8414-3.2-1`](#rfc8414-3.2-1) A successful response MUST use the 200 OK HTTP status code and return a JSON object using the "application/json" content type that contains a set of claims that are a subset of the metadata values defined in Section 2 (§3.2) | no test | no test carries this requirement id |
| [`RFC8414-3.2-2`](#rfc8414-3.2-2) Claims with zero elements MUST be omitted from the response (§3.2) | no test | no test carries this requirement id |
| [`RFC8414-4-1`](#rfc8414-4-1) Comparisons between JSON strings and other Unicode strings MUST be performed as specified in Section 4 (§4) | no test | no test carries this requirement id |
| [`RFC8414-4-2`](#rfc8414-4-2) Unicode Normalization MUST NOT be applied at any point to either the JSON string or the string it is to be compared against (§4) | no test | no test carries this requirement id |
| [`RFC8414-4-3`](#rfc8414-4-3) Comparisons between the two strings MUST be performed as a Unicode code-point-to-code-point equality comparison (§4) | no test | no test carries this requirement id |
| [`RFC8414-6.1-1`](#rfc8414-6.1-1) Implementations MUST support TLS (§6.1) | no test | no test carries this requirement id |
| [`RFC8414-6.1-2`](#rfc8414-6.1-2) The authorization server MUST support TLS version 1.2 (§6.1) | no test | no test carries this requirement id |
| [`RFC8414-6.1-3`](#rfc8414-6.1-3) When using TLS, the client MUST perform a TLS/SSL server certificate check, per RFC 6125 (§6.1, §6.2) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8414-2-1`](#rfc8414-2-1)

`issuer` is REQUIRED in the metadata document (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFetchASMetadata_MissingIssuer`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_test.go#L75) | unit/verify | unproven |
| positive | [`TestFetchASMetadata_Success`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_test.go#L56) | unit/verify | unproven |

### [`RFC8414-3-1`](#rfc8414-3-1)

Authorization servers supporting metadata MUST make the Section 2 metadata JSON document available at a path formed by inserting a well-known URI string into the issuer identifier between the host component and the path component (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-3-1, so no unit is bound to it.

### [`RFC8414-3.3-1`](#rfc8414-3.3-1)

The metadata path MUST use the "https" scheme, which RFC 8414 states in Section 3, and confidentiality protection MUST be applied using TLS with a ciphersuite that provides confidentiality and integrity protection, which it states in Section 6.1. Both are preconditions of the metadata validation this id is anchored to (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-3.3-1, so no unit is bound to it.

### [`RFC8414-3.3-2`](#rfc8414-3.3-2)

The `issuer` value returned MUST be identical to the issuer identifier value into which the well-known URI string was inserted to create the URL used to retrieve the metadata, and if they are not identical the data in the response MUST NOT be used (§3.3, §6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNewStreamable_OAuth_RejectsIssuerMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L130) | unit/verify | unproven |
| positive | [`TestNewStreamable_OAuth_AcceptsValidToken`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L141) | unit/verify | unproven |

### [`RFC8414-2-2`](#rfc8414-2-2)

`authorization_endpoint` is REQUIRED unless no grant types are supported that use the authorization endpoint (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2-2, so no unit is bound to it.

### [`RFC8414-2-3`](#rfc8414-2-3)

`token_endpoint` is REQUIRED unless only the implicit grant type is supported (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2-3, so no unit is bound to it.

### [`RFC8414-2-4`](#rfc8414-2-4)

`response_types_supported` is REQUIRED (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2-4, so no unit is bound to it.

### [`RFC8414-2-7`](#rfc8414-2-7)

The `jwks_uri` URL MUST use the "https" scheme (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2-7, so no unit is bound to it.

### [`RFC8414-2-8`](#rfc8414-2-8)

When both signing and encryption keys are made available, a "use" (public key use) parameter value is REQUIRED for all keys in the referenced JWK Set to indicate each key's intended usage (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2-8, so no unit is bound to it.

### [`RFC8414-2-9`](#rfc8414-2-9)

`token_endpoint_auth_signing_alg_values_supported` MUST be present if "private_key_jwt" or "client_secret_jwt" is specified in `token_endpoint_auth_methods_supported` (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2-9, so no unit is bound to it.

### [`RFC8414-2-10`](#rfc8414-2-10)

The value "none" MUST NOT be used in `token_endpoint_auth_signing_alg_values_supported` (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2-10, so no unit is bound to it.

### [`RFC8414-2-11`](#rfc8414-2-11)

`revocation_endpoint_auth_signing_alg_values_supported` MUST be present if "private_key_jwt" or "client_secret_jwt" is specified in `revocation_endpoint_auth_methods_supported` (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2-11, so no unit is bound to it.

### [`RFC8414-2-12`](#rfc8414-2-12)

The value "none" MUST NOT be used in `revocation_endpoint_auth_signing_alg_values_supported` (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2-12, so no unit is bound to it.

### [`RFC8414-2-13`](#rfc8414-2-13)

If `introspection_endpoint_auth_methods_supported` is omitted, the set of supported authentication methods MUST be determined by other means (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2-13, so no unit is bound to it.

### [`RFC8414-2-14`](#rfc8414-2-14)

`introspection_endpoint_auth_signing_alg_values_supported` MUST be present if "private_key_jwt" or "client_secret_jwt" is specified in `introspection_endpoint_auth_methods_supported` (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2-14, so no unit is bound to it.

### [`RFC8414-2-15`](#rfc8414-2-15)

The value "none" MUST NOT be used in `introspection_endpoint_auth_signing_alg_values_supported` (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2-15, so no unit is bound to it.

### [`RFC8414-2.1-1`](#rfc8414-2.1-1)

Signed metadata MUST be digitally signed or MACed using JSON Web Signature and MUST contain an "iss" (issuer) claim denoting the party attesting to the claims in the signed metadata (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2.1-1, so no unit is bound to it.

### [`RFC8414-2.1-2`](#rfc8414-2.1-2)

If the consumer of the metadata supports signed metadata, metadata values conveyed in the signed metadata MUST take precedence over the corresponding values conveyed using plain JSON elements (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2.1-2, so no unit is bound to it.

### [`RFC8414-3-2`](#rfc8414-3-2)

The well-known URI suffix used MUST be registered in the IANA "Well-Known URIs" registry (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-3-2, so no unit is bound to it.

### [`RFC8414-3-3`](#rfc8414-3-3)

An OAuth 2.0 application using this specification MUST specify what well-known URI suffix it will use for this purpose (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-3-3, so no unit is bound to it.

### [`RFC8414-3.1-1`](#rfc8414-3.1-1)

An authorization server metadata document MUST be queried using an HTTP "GET" request at the previously specified path (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-3.1-1, so no unit is bound to it.

### [`RFC8414-3.1-2`](#rfc8414-3.1-2)

If the issuer identifier value contains a path component, any terminating "/" MUST be removed before inserting "/.well-known/" and the well-known URI suffix between the host component and the path component (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-3.1-2, so no unit is bound to it.

### [`RFC8414-3.2-1`](#rfc8414-3.2-1)

A successful response MUST use the 200 OK HTTP status code and return a JSON object using the "application/json" content type that contains a set of claims that are a subset of the metadata values defined in Section 2 (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-3.2-1, so no unit is bound to it.

### [`RFC8414-3.2-2`](#rfc8414-3.2-2)

Claims with zero elements MUST be omitted from the response (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-3.2-2, so no unit is bound to it.

### [`RFC8414-4-1`](#rfc8414-4-1)

Comparisons between JSON strings and other Unicode strings MUST be performed as specified in Section 4 (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-4-1, so no unit is bound to it.

### [`RFC8414-4-2`](#rfc8414-4-2)

Unicode Normalization MUST NOT be applied at any point to either the JSON string or the string it is to be compared against (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-4-2, so no unit is bound to it.

### [`RFC8414-4-3`](#rfc8414-4-3)

Comparisons between the two strings MUST be performed as a Unicode code-point-to-code-point equality comparison (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-4-3, so no unit is bound to it.

### [`RFC8414-6.1-1`](#rfc8414-6.1-1)

Implementations MUST support TLS (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-6.1-1, so no unit is bound to it.

### [`RFC8414-6.1-2`](#rfc8414-6.1-2)

The authorization server MUST support TLS version 1.2 (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-6.1-2, so no unit is bound to it.

### [`RFC8414-6.1-3`](#rfc8414-6.1-3)

When using TLS, the client MUST perform a TLS/SSL server certificate check, per RFC 6125 (§6.1, §6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-6.1-3, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc8414.txt |
| Source fingerprint | 9ab822f4868cfc68 |
| Record | rfc/extraction/rfc8414.json |
| Mapped sentences | 31 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 13 | walked | not stated |
| `2.1` | not stated | 2 | walked | not stated |
| `3` | not stated | 4 | walked | not stated |
| `3.1` | not stated | 2 | walked | not stated |
| `3.2` | not stated | 2 | walked | not stated |
| `3.3` | not stated | 2 | walked | not stated |
| `4` | not stated | 3 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 4 | walked | not stated |
| `6.2` | not stated | 2 | walked | not stated |
| `6.3` | not stated | 0 | walked | not stated |
| `6.4` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.1.1` | not stated | 0 | walked | not stated |
| `7.1.2` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `7.3` | not stated | 0 | walked | not stated |
| `7.3.1` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The reject arm of the same sentence pair in Section 3.3: "If these values are not identical, the data contained in the response MUST NOT be used." Row RFC8414-3.3-2 states both arms and carries the positive and negative tags. | If these values are not identical, the data contained in the response MUST NOT be used. |
| `6.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6.2 restates the Section 6.1 certificate-check obligation and says so in its own words: "TLS certificate checking MUST be performed by the client, as described in Section 6.1, when making an authorization server metadata request." | TLS certificate checking MUST be performed by the client, as described in Section 6.1, when making an authorization server metadata request. |
| `6.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6.2 restates the Section 3.3 issuer-match obligation as an anti-impersonation measure; the client ensuring the issuer identifier URL matches the "issuer" metadata value is the same check row RFC8414-3.3-2 carries. | To prevent this, the client MUST ensure that the issuer identifier URL it is using as the prefix for the metadata request exactly matches the value of the "issuer" metadata value in the authorization server metadata document received by the client. |

## Superseded

No document obsoletes RFC 8414, so its obligations are stated where they were written.
