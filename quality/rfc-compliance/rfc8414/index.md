# RFC 8414 - OAuth 2.0 Authorization Server Metadata

Partial. Every requirement this repository extracted from RFC 8414, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 50.0% | 15 of 30 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 30 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 30 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 30 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 37.8% | 14 of 37 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 30 | of 32 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 15 | of 30 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 40.0% | 12 of 30 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 30 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 10.0% | 3 of 30 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 7 shares marked as a part above are the whole of the 30 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 32 |
| Gated MUST-level | 30 |
| Not applicable, so out of scope | 12 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 37 |
| Tagged units | 37 |
| Recorded audit verdicts | 0 |
| Discrimination records | 14 |
| Summary | `rfc/short/rfc8414.md` |
| Requirement shard | `rfc/requirements/rfc8414.md` |
| RFC text | `rfc/full/rfc8414.txt` |

## Enrolment

Enrolled: Ze consumes authorization-server metadata for its MCP resource server. Thomas selected "Keep present OAuth roles" on 2026-09-21: MCP resource server and external-AS metadata consumer, with no authorization server, protected-resource discovery client, response signing or signed metadata.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

`fetchASMetadata` requires HTTPS, an application/json object, an exact issuer match and an HTTPS `jwks_uri`. `oauthHTTPClient` prevents redirect downgrades. `parseJWKSDocument` rejects ambiguous key usage when encryption keys are present. The metadata URL uses the registered oauth-authorization-server suffix before the issuer path.

**What the ledger says remains**

Implementation features not offered under the 2026-09-21 scope decision: authorization/token/revocation/introspection endpoints and their client-authentication metadata, signed AS metadata, and an AS metadata publisher/TLS listener. These are absent features, not unimplemented obligations of the selected resource-server and plain-metadata-consumer roles. Applicable verification remains subject to the recorded tests and discrimination results.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 15 | one part of the gated population |
| Annotated instead of tested | 15 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **30** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (15):** [`RFC8414-2-1`](#rfc8414-2-1), [`RFC8414-3.3-1`](#rfc8414-3.3-1), [`RFC8414-3.3-2`](#rfc8414-3.3-2), [`RFC8414-2-7`](#rfc8414-2-7), [`RFC8414-2-8`](#rfc8414-2-8), [`RFC8414-3-2`](#rfc8414-3-2), [`RFC8414-3-3`](#rfc8414-3-3), [`RFC8414-3.1-1`](#rfc8414-3.1-1), [`RFC8414-3.1-2`](#rfc8414-3.1-2), [`RFC8414-3.2-1`](#rfc8414-3.2-1), [`RFC8414-4-1`](#rfc8414-4-1), [`RFC8414-4-2`](#rfc8414-4-2), [`RFC8414-4-3`](#rfc8414-4-3), [`RFC8414-6.1-1`](#rfc8414-6.1-1), [`RFC8414-6.1-3`](#rfc8414-6.1-3)

**Annotated instead of tested (15):** [`RFC8414-3-1`](#rfc8414-3-1), [`RFC8414-2-2`](#rfc8414-2-2), [`RFC8414-2-3`](#rfc8414-2-3), [`RFC8414-2-4`](#rfc8414-2-4), [`RFC8414-2-9`](#rfc8414-2-9), [`RFC8414-2-10`](#rfc8414-2-10), [`RFC8414-2-11`](#rfc8414-2-11), [`RFC8414-2-12`](#rfc8414-2-12), [`RFC8414-2-13`](#rfc8414-2-13), [`RFC8414-2-14`](#rfc8414-2-14), [`RFC8414-2-15`](#rfc8414-2-15), [`RFC8414-2.1-1`](#rfc8414-2.1-1), [`RFC8414-2.1-2`](#rfc8414-2.1-2), [`RFC8414-3.2-2`](#rfc8414-3.2-2), [`RFC8414-6.1-2`](#rfc8414-6.1-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8414-2-1` | `issuer` is REQUIRED in the metadata document (§2) | MUST | 2 | **positive:** `unit/verify` [`TestFetchASMetadata_Success`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_test.go#L67). **negative:** `unit/verify` [`TestFetchASMetadata_MissingIssuer`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_test.go#L87) |
| `RFC8414-3-1` | Authorization servers supporting metadata MUST make a JSON document containing metadata as specified in Section 2 available at a path formed by inserting a well-known URI string into the authorization server's issuer identifier between the host component and the path component, if any. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §3 addresses "Authorization servers supporting metadata"; Thomas's 2026-09-21 "Keep present OAuth roles" decision excludes that role. internal/component/mcp/streamable.go::ServeHTTP registers MCP and protected-resource metadata only; internal/component/mcp/as_metadata.go::fetchASMetadata is an outbound consumer, not an AS metadata response producer |
| `RFC8414-3.3-1` | The metadata path MUST use the "https" scheme, and confidentiality protection MUST be applied using TLS with a ciphersuite that provides confidentiality and integrity protection (§3, §6.1; historical section-3.3 ID retained) | MUST | 3 | **positive:** `unit/verify` [`TestRFC8414MetadataRequiresHTTPS`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L270). **negative:** `unit/verify` [`TestRFC8414MetadataRequiresHTTPS`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L271) |
| `RFC8414-3.3-2` | The `issuer` value returned MUST be identical to the issuer identifier value into which the well-known URI string was inserted to create the URL used to retrieve the metadata, and if they are not identical the data in the response MUST NOT be used (§3.3, §6.2) | MUST | 3.3 | **positive:** `unit/verify` [`TestNewStreamable_OAuth_AcceptsValidToken`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L172). **positive:** `unit/verify` [`TestRFC8414MetadataIssuerIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L373). **negative:** `unit/verify` [`TestNewStreamable_OAuth_RejectsIssuerMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L161). **negative:** `unit/verify` [`TestOAuthRejectsIssuerAliases`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L897). **negative:** `unit/verify` [`TestRFC8414MetadataIssuerIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L374) |
| `RFC8414-2-2` | This is REQUIRED unless no grant types are supported that use the authorization endpoint. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §2 says "This is REQUIRED unless no grant types are supported that use the authorization endpoint." This is the AS's endpoint advertisement; Thomas excluded the AS role on 2026-09-21. internal/component/mcp/streamable.go::ServeHTTP has no authorization endpoint, and internal/component/mcp/as_metadata.go::fetchASMetadata consumes issuer and jwks_uri without initiating an authorization grant |
| `RFC8414-2-3` | This is REQUIRED unless only the implicit grant type is supported. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §2 says "This is REQUIRED unless only the implicit grant type is supported." This describes the AS's token endpoint; Thomas excluded the AS role on 2026-09-21. internal/component/mcp/streamable.go::ServeHTTP publishes no token endpoint, and internal/component/mcp/streamable_auth.go::buildAuthForMode creates a verifier for externally issued tokens, not a token issuer |
| `RFC8414-2-4` | `response_types_supported` is REQUIRED (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §2 defines this as the "JSON array containing a list of the OAuth 2.0 \\"response_type\\" values that this authorization server supports." Thomas excluded the AS role on 2026-09-21. internal/component/mcp/streamable.go::ServeHTTP provides no authorization grant endpoint; internal/component/mcp/as_metadata.go::fetchASMetadata consumes issuer and jwks_uri, not grant-response capabilities |
| `RFC8414-2-5` | `jwks_uri` is OPTIONAL; the referenced document contains the signing key(s) the client uses to validate signatures from the authorization server (§2) | OPTIONAL | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-2-6` | `scopes_supported` is RECOMMENDED (§2) | RECOMMENDED | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8414-2-7` | This URL MUST use the "https" scheme. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC8414JWKSMetadataSecurity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L935). **negative:** `unit/verify` [`TestRFC8414JWKSMetadataSecurity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L936) |
| `RFC8414-2-8` | When both signing and encryption keys are made available, a "use" (public key use) parameter value is REQUIRED for all keys in the referenced JWK Set to indicate each key's intended usage (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC8414JWKSMetadataSecurity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L937). **negative:** `unit/verify` [`TestRFC8414JWKSMetadataSecurity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L938) |
| `RFC8414-2-9` | `token_endpoint_auth_signing_alg_values_supported` MUST be present if "private_key_jwt" or "client_secret_jwt" is specified in `token_endpoint_auth_methods_supported` (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §2 conditions the entry on "either of these authentication methods" in token_endpoint_auth_methods_supported, describing client authentication at the AS token endpoint. Thomas excluded the AS and token-client roles on 2026-09-21. internal/component/mcp/streamable.go::ServeHTTP exposes no token endpoint; internal/component/mcp/as_metadata.go::fetchASMetadata reads issuer and jwks_uri and selects no token-endpoint authentication method |
| `RFC8414-2-10` | The value "none" MUST NOT be used in `token_endpoint_auth_signing_alg_values_supported` (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §2 defines the field as algorithms "used to authenticate the client at the token endpoint"; it does not describe resource-server token verification. Thomas's 2026-09-21 decision excludes the AS/token-client roles. internal/component/mcp/streamable.go::ServeHTTP has no token endpoint, and internal/component/mcp/as_metadata.go::fetchASMetadata neither advertises nor selects these client-authentication algorithms |
| `RFC8414-2-11` | `revocation_endpoint_auth_signing_alg_values_supported` MUST be present if "private_key_jwt" or "client_secret_jwt" is specified in `revocation_endpoint_auth_methods_supported` (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §2 conditions this entry on methods "specified in the \\"revocation_endpoint_auth_methods_supported\\" entry." Thomas's 2026-09-21 selection excludes the AS and revocation-client roles. internal/component/mcp/streamable.go::ServeHTTP publishes no revocation endpoint; internal/component/mcp/as_metadata.go::fetchASMetadata consumes only issuer and jwks_uri and makes no revocation request |
| `RFC8414-2-12` | The value "none" MUST NOT be used in `revocation_endpoint_auth_signing_alg_values_supported` (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §2 describes algorithms "used to authenticate the client at the revocation endpoint", not token verification by a resource server. Thomas's 2026-09-21 selection excludes the AS and revocation-client roles. internal/component/mcp/streamable.go::ServeHTTP has no revocation endpoint; internal/component/mcp/as_metadata.go::fetchASMetadata neither publishes nor selects revocation authentication algorithms |
| `RFC8414-2-13` | (These values are and will remain distinct, due to Section 7.2.) If omitted, the set of supported authentication methods MUST be determined by other means. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "OPTIONAL. URL of the authorization server's OAuth 2.0 introspection endpoint [RFC7662]."; §2 makes introspection metadata optional. Thomas chose the present OAuth roles on 2026-09-21, retaining local token verification rather than introspection. internal/component/mcp/streamable_auth.go::buildAuthForMode constructs a JWKS-backed verifier and makes no introspection request that would need an authentication method |
| `RFC8414-2-14` | `introspection_endpoint_auth_signing_alg_values_supported` MUST be present if "private_key_jwt" or "client_secret_jwt" is specified in `introspection_endpoint_auth_methods_supported` (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §2 conditions this entry on methods "specified in the \\"introspection_endpoint_auth_methods_supported\\" entry." Thomas's 2026-09-21 selection excludes the AS and introspection-client roles. internal/component/mcp/streamable.go::ServeHTTP has no introspection endpoint; internal/component/mcp/streamable_auth.go::buildAuthForMode constructs a local JWKS-backed verifier rather than an introspection client |
| `RFC8414-2-15` | The value "none" MUST NOT be used in `introspection_endpoint_auth_signing_alg_values_supported` (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §2 describes algorithms "used to authenticate the client at the introspection endpoint", not the signatures on access tokens. Thomas's 2026-09-21 selection excludes the AS/introspection-client roles. internal/component/mcp/as_metadata.go::fetchASMetadata consumes issuer and jwks_uri only; internal/component/mcp/streamable_auth.go::buildAuthForMode verifies tokens locally and selects no introspection authentication algorithm |
| `RFC8414-2.1-1` | The signed metadata MUST be digitally signed or MACed using JSON Web Signature (JWS) [JWS] and MUST contain an "iss" (issuer) claim denoting the party attesting to the claims in the signed metadata. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "Consumers of the metadata MAY ignore the signed metadata if they do not support this feature."; Thomas excluded signed metadata on 2026-09-21. internal/component/mcp/as_metadata.go::fetchASMetadata consumes only plain JSON issuer and jwks_uri; internal/component/mcp/streamable.go::ServeHTTP publishes no AS metadata, signed or otherwise |
| `RFC8414-2.1-2` | If the consumer of the metadata supports signed metadata, metadata values conveyed in the signed metadata MUST take precedence over the corresponding values conveyed using plain JSON elements (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "Consumers of the metadata MAY ignore the signed metadata if they do not support this feature."; Thomas excluded signed metadata on 2026-09-21. internal/component/mcp/as_metadata.go::fetchASMetadata reads the plain JSON issuer and jwks_uri only, so no supported signed-metadata values compete with those fields |
| `RFC8414-3-2` | The well-known URI suffix used MUST be registered in the IANA "Well-Known URIs" registry (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC8414MetadataRequestIsGETAtWellKnownPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L77). **negative:** `unit/verify` [`TestRFC8414MetadataPathNeverAppendedToIssuerPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L154) |
| `RFC8414-3-3` | An OAuth 2.0 application using this specification MUST specify what well-known URI suffix it will use for this purpose (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC8414MetadataRequestIsGETAtWellKnownPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L78). **negative:** `unit/verify` [`TestRFC8414MetadataPathNeverAppendedToIssuerPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L155) |
| `RFC8414-3.1-1` | An authorization server metadata document MUST be queried using an HTTP "GET" request at the previously specified path (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8414MetadataRequestIsGETAtWellKnownPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L73). **negative:** `unit/verify` [`TestRFC8414MetadataRequestUsesNoOtherVerbOrPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L101) |
| `RFC8414-3.1-2` | If the issuer identifier value contains a path component, any terminating "/" MUST be removed before inserting "/.well-known/" and the well-known URI suffix between the host component and the path component (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8414MetadataPathInsertedBeforeIssuerPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L127). **negative:** `unit/verify` [`TestRFC8414MetadataPathNeverAppendedToIssuerPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L150) |
| `RFC8414-3.2-1` | A successful response MUST use the 200 OK HTTP status code and return a JSON object using the "application/json" content type that contains a set of claims as its members that are a subset of the metadata values defined in Section 2. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8414MetadataHTTPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L324). **negative:** `unit/verify` [`TestRFC8414MetadataHTTPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L325) |
| `RFC8414-3.2-2` | Claims with zero elements MUST be omitted from the response (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §3.2 says "The response is a set of claims about the authorization server's configuration." Thomas excluded the AS publisher role on 2026-09-21. internal/component/mcp/as_metadata.go::fetchASMetadata consumes that response; internal/component/mcp/streamable.go::ServeHTTP exposes only MCP and protected-resource metadata, whose empty-field omission is separately governed by RFC 9728 |
| `RFC8414-4-1` | Comparisons between JSON strings and other Unicode strings MUST be performed as specified in Section 4 (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC8414IssuerCompareUnescapesJSON`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_rfc8414_test.go#L75). **negative:** `unit/verify` [`TestRFC8414IssuerCompareNotOnJSONText`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_rfc8414_test.go#L99) |
| `RFC8414-4-2` | Unicode Normalization [USA15] MUST NOT be applied at any point to either the JSON string or the string it is to be compared against. (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestRFC8414IssuerCompareNoNormalizationAccepts`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_rfc8414_test.go#L120). **positive:** `unit/verify` [`TestRFC8414MetadataIssuerIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L375). **negative:** `unit/verify` [`TestRFC8414IssuerCompareNoNormalizationRejects`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_rfc8414_test.go#L130). **negative:** `unit/verify` [`TestRFC8414MetadataIssuerIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L376) |
| `RFC8414-4-3` | Comparisons between the two strings MUST be performed as a Unicode code-point-to-code-point equality comparison (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC8414IssuerCompareCodePointsEqual`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_rfc8414_test.go#L145). **positive:** `unit/verify` [`TestRFC8414MetadataIssuerIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L377). **negative:** `unit/verify` [`TestRFC8414IssuerCompareCodePointsDiffer`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_rfc8414_test.go#L154). **negative:** `unit/verify` [`TestRFC8414MetadataIssuerIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L378) |
| `RFC8414-6.1-1` | Implementations MUST support TLS (§6.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestRFC8414MetadataFetchOverTLS`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L179). **negative:** `unit/verify` [`TestRFC8414MetadataFetchNeverDowngradesToCleartext`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L203) |
| `RFC8414-6.1-2` | The authorization server MUST support TLS version 1.2 (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §6.1 explicitly says "The authorization server MUST support TLS version 1.2 [RFC5246]". Thomas excluded the AS role on 2026-09-21. internal/component/mcp/as_metadata.go::fetchASMetadata is the external-AS HTTPS client; cmd/ze/hub/service_mcp.go::loadMCPTLSConfig configures Ze's resource-server listener, not an AS listener. Its real-listener proof belongs to RFC9728-7.1-1 and RFC9728-7.1-2 |
| `RFC8414-6.1-3` | When using TLS, the client MUST perform a TLS/SSL server certificate check, per RFC 6125 (§6.1, §6.2) | MUST | 6.1 | **positive:** `unit/verify` [`TestRFC8414MetadataFetchAcceptsTrustedCertificate`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L225). **negative:** `unit/verify` [`TestRFC8414MetadataFetchRefusesUntrustedCertificate`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L243) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8414-3-1`](#rfc8414-3-1) Authorization servers supporting metadata MUST make a JSON document containing metadata as specified in Section 2 available at a path formed by inserting a well-known URI string into the authorization server's issuer identifier between the host component and the path component, if any. (§3) | no test | no test carries this requirement id; annotated {not-applicable}: §3 addresses "Authorization servers supporting metadata"; Thomas's 2026-09-21 "Keep present OAuth roles" decision excludes that role. internal/component/mcp/streamable.go::ServeHTTP registers MCP and protected-resource metadata only; internal/component/mcp/as_metadata.go::fetchASMetadata is an outbound consumer, not an AS metadata response producer |
| [`RFC8414-2-2`](#rfc8414-2-2) This is REQUIRED unless no grant types are supported that use the authorization endpoint. (§2) | no test | no test carries this requirement id; annotated {not-applicable}: §2 says "This is REQUIRED unless no grant types are supported that use the authorization endpoint." This is the AS's endpoint advertisement; Thomas excluded the AS role on 2026-09-21. internal/component/mcp/streamable.go::ServeHTTP has no authorization endpoint, and internal/component/mcp/as_metadata.go::fetchASMetadata consumes issuer and jwks_uri without initiating an authorization grant |
| [`RFC8414-2-3`](#rfc8414-2-3) This is REQUIRED unless only the implicit grant type is supported. (§2) | no test | no test carries this requirement id; annotated {not-applicable}: §2 says "This is REQUIRED unless only the implicit grant type is supported." This describes the AS's token endpoint; Thomas excluded the AS role on 2026-09-21. internal/component/mcp/streamable.go::ServeHTTP publishes no token endpoint, and internal/component/mcp/streamable_auth.go::buildAuthForMode creates a verifier for externally issued tokens, not a token issuer |
| [`RFC8414-2-4`](#rfc8414-2-4) `response_types_supported` is REQUIRED (§2) | no test | no test carries this requirement id; annotated {not-applicable}: §2 defines this as the "JSON array containing a list of the OAuth 2.0 \\"response_type\\" values that this authorization server supports." Thomas excluded the AS role on 2026-09-21. internal/component/mcp/streamable.go::ServeHTTP provides no authorization grant endpoint; internal/component/mcp/as_metadata.go::fetchASMetadata consumes issuer and jwks_uri, not grant-response capabilities |
| [`RFC8414-2-9`](#rfc8414-2-9) `token_endpoint_auth_signing_alg_values_supported` MUST be present if "private_key_jwt" or "client_secret_jwt" is specified in `token_endpoint_auth_methods_supported` (§2) | no test | no test carries this requirement id; annotated {not-applicable}: §2 conditions the entry on "either of these authentication methods" in token_endpoint_auth_methods_supported, describing client authentication at the AS token endpoint. Thomas excluded the AS and token-client roles on 2026-09-21. internal/component/mcp/streamable.go::ServeHTTP exposes no token endpoint; internal/component/mcp/as_metadata.go::fetchASMetadata reads issuer and jwks_uri and selects no token-endpoint authentication method |
| [`RFC8414-2-10`](#rfc8414-2-10) The value "none" MUST NOT be used in `token_endpoint_auth_signing_alg_values_supported` (§2) | no test | no test carries this requirement id; annotated {not-applicable}: §2 defines the field as algorithms "used to authenticate the client at the token endpoint"; it does not describe resource-server token verification. Thomas's 2026-09-21 decision excludes the AS/token-client roles. internal/component/mcp/streamable.go::ServeHTTP has no token endpoint, and internal/component/mcp/as_metadata.go::fetchASMetadata neither advertises nor selects these client-authentication algorithms |
| [`RFC8414-2-11`](#rfc8414-2-11) `revocation_endpoint_auth_signing_alg_values_supported` MUST be present if "private_key_jwt" or "client_secret_jwt" is specified in `revocation_endpoint_auth_methods_supported` (§2) | no test | no test carries this requirement id; annotated {not-applicable}: §2 conditions this entry on methods "specified in the \\"revocation_endpoint_auth_methods_supported\\" entry." Thomas's 2026-09-21 selection excludes the AS and revocation-client roles. internal/component/mcp/streamable.go::ServeHTTP publishes no revocation endpoint; internal/component/mcp/as_metadata.go::fetchASMetadata consumes only issuer and jwks_uri and makes no revocation request |
| [`RFC8414-2-12`](#rfc8414-2-12) The value "none" MUST NOT be used in `revocation_endpoint_auth_signing_alg_values_supported` (§2) | no test | no test carries this requirement id; annotated {not-applicable}: §2 describes algorithms "used to authenticate the client at the revocation endpoint", not token verification by a resource server. Thomas's 2026-09-21 selection excludes the AS and revocation-client roles. internal/component/mcp/streamable.go::ServeHTTP has no revocation endpoint; internal/component/mcp/as_metadata.go::fetchASMetadata neither publishes nor selects revocation authentication algorithms |
| [`RFC8414-2-13`](#rfc8414-2-13) (These values are and will remain distinct, due to Section 7.2.) If omitted, the set of supported authentication methods MUST be determined by other means. (§2) | no test | no test carries this requirement id; annotated {feature-declined}: "OPTIONAL. URL of the authorization server's OAuth 2.0 introspection endpoint [RFC7662]."; §2 makes introspection metadata optional. Thomas chose the present OAuth roles on 2026-09-21, retaining local token verification rather than introspection. internal/component/mcp/streamable_auth.go::buildAuthForMode constructs a JWKS-backed verifier and makes no introspection request that would need an authentication method |
| [`RFC8414-2-14`](#rfc8414-2-14) `introspection_endpoint_auth_signing_alg_values_supported` MUST be present if "private_key_jwt" or "client_secret_jwt" is specified in `introspection_endpoint_auth_methods_supported` (§2) | no test | no test carries this requirement id; annotated {not-applicable}: §2 conditions this entry on methods "specified in the \\"introspection_endpoint_auth_methods_supported\\" entry." Thomas's 2026-09-21 selection excludes the AS and introspection-client roles. internal/component/mcp/streamable.go::ServeHTTP has no introspection endpoint; internal/component/mcp/streamable_auth.go::buildAuthForMode constructs a local JWKS-backed verifier rather than an introspection client |
| [`RFC8414-2-15`](#rfc8414-2-15) The value "none" MUST NOT be used in `introspection_endpoint_auth_signing_alg_values_supported` (§2) | no test | no test carries this requirement id; annotated {not-applicable}: §2 describes algorithms "used to authenticate the client at the introspection endpoint", not the signatures on access tokens. Thomas's 2026-09-21 selection excludes the AS/introspection-client roles. internal/component/mcp/as_metadata.go::fetchASMetadata consumes issuer and jwks_uri only; internal/component/mcp/streamable_auth.go::buildAuthForMode verifies tokens locally and selects no introspection authentication algorithm |
| [`RFC8414-2.1-1`](#rfc8414-2.1-1) The signed metadata MUST be digitally signed or MACed using JSON Web Signature (JWS) [JWS] and MUST contain an "iss" (issuer) claim denoting the party attesting to the claims in the signed metadata. (§2.1) | no test | no test carries this requirement id; annotated {feature-declined}: "Consumers of the metadata MAY ignore the signed metadata if they do not support this feature."; Thomas excluded signed metadata on 2026-09-21. internal/component/mcp/as_metadata.go::fetchASMetadata consumes only plain JSON issuer and jwks_uri; internal/component/mcp/streamable.go::ServeHTTP publishes no AS metadata, signed or otherwise |
| [`RFC8414-2.1-2`](#rfc8414-2.1-2) If the consumer of the metadata supports signed metadata, metadata values conveyed in the signed metadata MUST take precedence over the corresponding values conveyed using plain JSON elements (§2.1) | no test | no test carries this requirement id; annotated {feature-declined}: "Consumers of the metadata MAY ignore the signed metadata if they do not support this feature."; Thomas excluded signed metadata on 2026-09-21. internal/component/mcp/as_metadata.go::fetchASMetadata reads the plain JSON issuer and jwks_uri only, so no supported signed-metadata values compete with those fields |
| [`RFC8414-3.2-2`](#rfc8414-3.2-2) Claims with zero elements MUST be omitted from the response (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: §3.2 says "The response is a set of claims about the authorization server's configuration." Thomas excluded the AS publisher role on 2026-09-21. internal/component/mcp/as_metadata.go::fetchASMetadata consumes that response; internal/component/mcp/streamable.go::ServeHTTP exposes only MCP and protected-resource metadata, whose empty-field omission is separately governed by RFC 9728 |
| [`RFC8414-6.1-2`](#rfc8414-6.1-2) The authorization server MUST support TLS version 1.2 (§6.1) | no test | no test carries this requirement id; annotated {not-applicable}: §6.1 explicitly says "The authorization server MUST support TLS version 1.2 [RFC5246]". Thomas excluded the AS role on 2026-09-21. internal/component/mcp/as_metadata.go::fetchASMetadata is the external-AS HTTPS client; cmd/ze/hub/service_mcp.go::loadMCPTLSConfig configures Ze's resource-server listener, not an AS listener. Its real-listener proof belongs to RFC9728-7.1-1 and RFC9728-7.1-2 |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8414-2-1`](#rfc8414-2-1)

`issuer` is REQUIRED in the metadata document (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFetchASMetadata_MissingIssuer`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_test.go#L87) | unit/verify | unproven |
| positive | [`TestFetchASMetadata_Success`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_test.go#L67) | unit/verify | unproven |

### [`RFC8414-3-1`](#rfc8414-3-1)

Authorization servers supporting metadata MUST make a JSON document containing metadata as specified in Section 2 available at a path formed by inserting a well-known URI string into the authorization server's issuer identifier between the host component and the path component, if any. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-3-1, so no unit is bound to it.

### [`RFC8414-3.3-1`](#rfc8414-3.3-1)

The metadata path MUST use the "https" scheme, and confidentiality protection MUST be applied using TLS with a ciphersuite that provides confidentiality and integrity protection (§3, §6.1; historical section-3.3 ID retained)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8414MetadataRequiresHTTPS`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L271) | unit/verify | unproven |
| positive | [`TestRFC8414MetadataRequiresHTTPS`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L270) | unit/verify | unproven |

### [`RFC8414-3.3-2`](#rfc8414-3.3-2)

The `issuer` value returned MUST be identical to the issuer identifier value into which the well-known URI string was inserted to create the URL used to retrieve the metadata, and if they are not identical the data in the response MUST NOT be used (§3.3, §6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8414MetadataIssuerIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L374) | unit/verify | unproven |
| negative | [`TestNewStreamable_OAuth_RejectsIssuerMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L161) | unit/verify | unproven |
| negative | [`TestOAuthRejectsIssuerAliases`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L897) | unit/verify | unproven |
| positive | [`TestRFC8414MetadataIssuerIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L373) | unit/verify | unproven |
| positive | [`TestNewStreamable_OAuth_AcceptsValidToken`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L172) | unit/verify | unproven |

### [`RFC8414-2-2`](#rfc8414-2-2)

This is REQUIRED unless no grant types are supported that use the authorization endpoint. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2-2, so no unit is bound to it.

### [`RFC8414-2-3`](#rfc8414-2-3)

This is REQUIRED unless only the implicit grant type is supported. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2-3, so no unit is bound to it.

### [`RFC8414-2-4`](#rfc8414-2-4)

`response_types_supported` is REQUIRED (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2-4, so no unit is bound to it.

### [`RFC8414-2-7`](#rfc8414-2-7)

This URL MUST use the "https" scheme. (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8414JWKSMetadataSecurity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L936) | unit/verify | unproven |
| positive | [`TestRFC8414JWKSMetadataSecurity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L935) | unit/verify | unproven |

### [`RFC8414-2-8`](#rfc8414-2-8)

When both signing and encryption keys are made available, a "use" (public key use) parameter value is REQUIRED for all keys in the referenced JWK Set to indicate each key's intended usage (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8414JWKSMetadataSecurity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L938) | unit/verify | unproven |
| positive | [`TestRFC8414JWKSMetadataSecurity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L937) | unit/verify | unproven |

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

(These values are and will remain distinct, due to Section 7.2.) If omitted, the set of supported authentication methods MUST be determined by other means. (§2)

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

The signed metadata MUST be digitally signed or MACed using JSON Web Signature (JWS) [JWS] and MUST contain an "iss" (issuer) claim denoting the party attesting to the claims in the signed metadata. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2.1-1, so no unit is bound to it.

### [`RFC8414-2.1-2`](#rfc8414-2.1-2)

If the consumer of the metadata supports signed metadata, metadata values conveyed in the signed metadata MUST take precedence over the corresponding values conveyed using plain JSON elements (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-2.1-2, so no unit is bound to it.

### [`RFC8414-3-2`](#rfc8414-3-2)

The well-known URI suffix used MUST be registered in the IANA "Well-Known URIs" registry (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8414MetadataPathNeverAppendedToIssuerPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L154) | unit/verify | unproven |
| positive | [`TestRFC8414MetadataRequestIsGETAtWellKnownPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L77) | unit/verify | unproven |

### [`RFC8414-3-3`](#rfc8414-3-3)

An OAuth 2.0 application using this specification MUST specify what well-known URI suffix it will use for this purpose (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8414MetadataPathNeverAppendedToIssuerPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L155) | unit/verify | unproven |
| positive | [`TestRFC8414MetadataRequestIsGETAtWellKnownPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L78) | unit/verify | unproven |

### [`RFC8414-3.1-1`](#rfc8414-3.1-1)

An authorization server metadata document MUST be queried using an HTTP "GET" request at the previously specified path (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8414MetadataRequestUsesNoOtherVerbOrPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L101) | unit/verify | revert, verified |
| positive | [`TestRFC8414MetadataRequestIsGETAtWellKnownPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L73) | unit/verify | revert, verified |

### [`RFC8414-3.1-2`](#rfc8414-3.1-2)

If the issuer identifier value contains a path component, any terminating "/" MUST be removed before inserting "/.well-known/" and the well-known URI suffix between the host component and the path component (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8414MetadataPathNeverAppendedToIssuerPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L150) | unit/verify | revert, verified |
| positive | [`TestRFC8414MetadataPathInsertedBeforeIssuerPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L127) | unit/verify | revert, verified |

### [`RFC8414-3.2-1`](#rfc8414-3.2-1)

A successful response MUST use the 200 OK HTTP status code and return a JSON object using the "application/json" content type that contains a set of claims as its members that are a subset of the metadata values defined in Section 2. (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8414MetadataHTTPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L325) | unit/verify | unproven |
| positive | [`TestRFC8414MetadataHTTPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L324) | unit/verify | unproven |

### [`RFC8414-3.2-2`](#rfc8414-3.2-2)

Claims with zero elements MUST be omitted from the response (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-3.2-2, so no unit is bound to it.

### [`RFC8414-4-1`](#rfc8414-4-1)

Comparisons between JSON strings and other Unicode strings MUST be performed as specified in Section 4 (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8414IssuerCompareNotOnJSONText`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_rfc8414_test.go#L99) | unit/verify | revert, verified |
| positive | [`TestRFC8414IssuerCompareUnescapesJSON`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_rfc8414_test.go#L75) | unit/verify | revert, verified |

### [`RFC8414-4-2`](#rfc8414-4-2)

Unicode Normalization [USA15] MUST NOT be applied at any point to either the JSON string or the string it is to be compared against. (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8414MetadataIssuerIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L376) | unit/verify | unproven |
| negative | [`TestRFC8414IssuerCompareNoNormalizationRejects`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_rfc8414_test.go#L130) | unit/verify | revert, verified |
| positive | [`TestRFC8414MetadataIssuerIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L375) | unit/verify | unproven |
| positive | [`TestRFC8414IssuerCompareNoNormalizationAccepts`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_rfc8414_test.go#L120) | unit/verify | revert, verified |

### [`RFC8414-4-3`](#rfc8414-4-3)

Comparisons between the two strings MUST be performed as a Unicode code-point-to-code-point equality comparison (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8414MetadataIssuerIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L378) | unit/verify | unproven |
| negative | [`TestRFC8414IssuerCompareCodePointsDiffer`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_rfc8414_test.go#L154) | unit/verify | revert, verified |
| positive | [`TestRFC8414MetadataIssuerIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L377) | unit/verify | unproven |
| positive | [`TestRFC8414IssuerCompareCodePointsEqual`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/jwt_rfc8414_test.go#L145) | unit/verify | revert, verified |

### [`RFC8414-6.1-1`](#rfc8414-6.1-1)

Implementations MUST support TLS (§6.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8414MetadataFetchNeverDowngradesToCleartext`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L203) | unit/verify | revert, verified |
| positive | [`TestRFC8414MetadataFetchOverTLS`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L179) | unit/verify | revert, verified |

### [`RFC8414-6.1-2`](#rfc8414-6.1-2)

The authorization server MUST support TLS version 1.2 (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8414-6.1-2, so no unit is bound to it.

### [`RFC8414-6.1-3`](#rfc8414-6.1-3)

When using TLS, the client MUST perform a TLS/SSL server certificate check, per RFC 6125 (§6.1, §6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8414MetadataFetchRefusesUntrustedCertificate`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L243) | unit/verify | revert, verified |
| positive | [`TestRFC8414MetadataFetchAcceptsTrustedCertificate`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/as_metadata_rfc8414_test.go#L225) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | Main integration; independent source-mapping review: OAuthMappingReview |
| Signed off | 2026-09-22 |
| Register | rfc2119 |
| Source | rfc/full/rfc8414.txt |
| Source fingerprint | 9ab822f4868cfc68 |
| Record | rfc/extraction/rfc8414.json |
| Mapped sentences | 32 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 13 | walked | Section 2 describes jwks_uri as OPTIONAL and scopes_supported as RECOMMENDED. These advisory entries have no separate MUST-level source site. Their IDs remain allocated; the MUST clauses governing the referenced key set map separately at 2:4 and 2:5. |
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
| `7` | Registration procedure addresses IANA and Designated Experts | 0 | walked | Registration procedure addresses IANA and Designated Experts. Its lowercase registry instructions do not create an additional MUST-level site in this document's selected inventory. |
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
| `6.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6.2 expressly restates Section 6.1: TLS certificate checking MUST be performed by the client, as described in Section 6.1, when making an authorization server metadata request. Site 6.1:3 directly maps RFC8414-6.1-3. | TLS certificate checking MUST be performed by the client, as described in Section 6.1, when making an authorization server metadata request. |
| `6.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6.2 repeats the Section 3.3 exact issuer-match rule as protection against impersonation. Sites 3.3:1 and 3.3:2 directly map the equality and rejection clauses of RFC8414-3.3-2. | To prevent this, the client MUST ensure that the issuer identifier URL it is using as the prefix for the metadata request exactly matches the value of the "issuer" metadata value in the authorization server metadata document received by the client. |

## Superseded

No document obsoletes RFC 8414, so its obligations are stated where they were written.
