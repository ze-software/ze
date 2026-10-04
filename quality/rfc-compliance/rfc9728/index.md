# RFC 9728 - OAuth 2.0 Protected Resource Metadata

Partial. Every requirement this repository extracted from RFC 9728, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 46.2% | 12 of 26 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 3.8% | 1 of 26 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 26 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 26 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 26 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 78.4% | 29 of 37 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 26 | of 32 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 13 | of 26 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 26.9% | 7 of 26 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 26 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 23.1% | 6 of 26 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 11 | of 26 gated MUSTs judged | 3 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 26 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 32 |
| Gated MUST-level | 26 |
| Not applicable, so out of scope | 7 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 37 |
| Tagged units | 37 |
| Recorded audit verdicts | 11 |
| Discrimination records | 29 |
| Summary | `rfc/short/rfc9728.md` |
| Requirement shard | `rfc/requirements/rfc9728.md` |
| RFC text | `rfc/full/rfc9728.txt` |

## Enrolment

Enrolled: Ze publishes protected-resource metadata for its MCP resource server. Thomas selected "Keep present OAuth roles" on 2026-09-21: MCP resource server and external-AS metadata consumer, with no authorization server, protected-resource discovery client, response signing or signed metadata.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Ze publishes plain JSON protected-resource metadata for its MCP resource server at the `oauth-protected-resource` well-known URL, answers GET and refuses other methods, keeps the escaped resource path and query in the resource identifier, and sends bearer challenges.

**What the ledger says remains**

One row is audited wrong. [`RFC9728-3.1-3`](#rfc9728-3.1-3): `resourceOriginAndPath` trims the resource path's own trailing slash, which the RFC keeps, so an identifier such as `https://host/mcp/` is published at a location formed from `/mcp`; [`RFC9728-3-2`](#rfc9728-3-2) and [`RFC9728-3.3-1`](#rfc9728-3.3-1) fail for the same input. Response signing and signed metadata are features not offered.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 12 | one part of the gated population |
| Annotated (including scoped evidence) | 14 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **26** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (12):** [`RFC9728-3-2`](#rfc9728-3-2), [`RFC9728-3-3`](#rfc9728-3-3), [`RFC9728-3-4`](#rfc9728-3-4), [`RFC9728-3.1-3`](#rfc9728-3.1-3), [`RFC9728-3.2-3`](#rfc9728-3.2-3), [`RFC9728-3.2-4`](#rfc9728-3.2-4), [`RFC9728-3.3-1`](#rfc9728-3.3-1), [`RFC9728-6-1`](#rfc9728-6-1), [`RFC9728-6-2`](#rfc9728-6-2), [`RFC9728-6-3`](#rfc9728-6-3), [`RFC9728-7.1-1`](#rfc9728-7.1-1), [`RFC9728-7.1-2`](#rfc9728-7.1-2)

**Annotated (including scoped evidence) (14):** [`RFC9728-2-1`](#rfc9728-2-1), [`RFC9728-2-6`](#rfc9728-2-6), [`RFC9728-2-7`](#rfc9728-2-7), [`RFC9728-2-8`](#rfc9728-2-8), [`RFC9728-2.1-1`](#rfc9728-2.1-1), [`RFC9728-2.1-2`](#rfc9728-2.1-2), [`RFC9728-2.2-1`](#rfc9728-2.2-1), [`RFC9728-2.2-2`](#rfc9728-2.2-2), [`RFC9728-3.3-2`](#rfc9728-3.3-2), [`RFC9728-3.3-3`](#rfc9728-3.3-3), [`RFC9728-3.3-4`](#rfc9728-3.3-4), [`RFC9728-7.3-1`](#rfc9728-7.3-1), [`RFC9728-7.3-2`](#rfc9728-7.3-2), [`RFC9728-3.2-5`](#rfc9728-3.2-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9728-2-1` | resource REQUIRED. The protected resource's resource identifier, as defined in Section 1.2. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestNewStreamable_OAuth_MetadataEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L321). **positive:** `unit/verify` [`TestResourceMetadata_Document`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_oauth_test.go#L212). **negative:** no negative test. **{single-polarity}:** the resource field is emitted unconditionally by writeResourceMetadata (internal/component/mcp/oauth.go:170-171,184-192), so no input can make it absent and there is no negative to assert |
| `RFC9728-2-6` | This URL MUST use the https scheme. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "OPTIONAL. URL of the protected resource's JSON Web Key (JWK) Set [JWK] document."; §2 describes the resource's own keys, not its external AS's verification keys. Thomas excluded response signing on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata publishes no jwks_uri; internal/component/mcp/streamable.go::ServeHTTP exposes no resource signing-key endpoint |
| `RFC9728-2-7` | When both signing and encryption keys are made available, a use (public key use) parameter value is REQUIRED for all keys in the referenced JWK Set to indicate each key's intended usage. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "OPTIONAL. URL of the protected resource's JSON Web Key (JWK) Set [JWK] document."; Thomas excluded response signing on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata publishes no resource jwks_uri and internal/component/mcp/streamable.go::ServeHTTP has no resource-key response producer. The AS verification-key cache is a different key set governed by RFC8414-2-8 |
| `RFC9728-2-8` | resource_signing_alg_values_supported OPTIONAL. JSON array containing a list of the JWS [JWS] signing algorithms (alg values) [JWA] supported by the protected resource for signing resource responses, for instance, as described in [FAPI.MessageSigning]. No default algorithms are implied if this entry is omitted. The value none MUST NOT be used. (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "OPTIONAL. JSON array containing a list of the JWS [JWS] signing algorithms (alg values) [JWA] supported by the protected resource for signing resource responses, for instance, as described in [FAPI.MessageSigning]."; Thomas excluded response signing on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata emits no resource_signing_alg_values_supported; token-verification algorithms are not resource-response signing capabilities |
| `RFC9728-2.1-1` | If any human-readable field is sent without a language tag, parties using it MUST NOT make any assumptions about the language, character set, or script of the string value (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §2.1 addresses "parties using it" when a human-readable field is received. Thomas excluded the protected-resource discovery-client role on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata emits resource/authorization_servers/scopes_supported/bearer_methods_supported only; internal/component/mcp/as_metadata.go::fetchASMetadata consumes AS issuer and jwks_uri, not human-readable resource metadata or a metadata user interface |
| `RFC9728-2.1-2` | If any human-readable field is sent without a language tag, parties using it MUST NOT make any assumptions about the language, character set, or script of the string value, and the string value MUST be used as is wherever it is presented in a user interface. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §2.1 conditions this on "wherever it is presented in a user interface." Thomas excluded the protected-resource discovery-client role on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata is a JSON publisher, not a metadata-consuming UI; internal/component/mcp/as_metadata.go::fetchASMetadata reads AS issuer and jwks_uri without displaying resource names or documentation |
| `RFC9728-2.2-1` | The signed metadata MUST be digitally signed or MACed (protected with a Message Authentication Code) using a JSON Web Signature (JWS) [JWS] and MUST contain an iss (issuer) claim denoting the party attesting to the claims in the signed metadata. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "In addition to JSON elements, metadata values MAY also be provided as a signed_metadata value, which is a JSON Web Token (JWT) [JWT] that asserts metadata values about the protected resource as a bundle."; Thomas excluded signed metadata on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata emits plain JSON and no signed_metadata parameter |
| `RFC9728-2.2-2` | If the consumer of the metadata supports signed metadata, metadata values conveyed in the signed metadata MUST take precedence over the corresponding values conveyed using plain JSON elements (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "Consumers of the metadata MAY ignore the signed metadata if they do not support this feature."; Thomas excluded signed metadata and resource-discovery clients on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata publishes plain JSON; internal/component/mcp/streamable_auth.go::buildAuthForMode consumes AS issuer/keys, not protected-resource signed metadata |
| `RFC9728-3-2` | Protected resources supporting metadata MUST make a JSON document containing metadata as specified in Section 2 available at a URL formed by inserting a well-known URI string into the protected resource's resource identifier between the host component and the path and/or query components, if any. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC9728WellKnownInsertedBeforeQuery`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L322). **positive:** `unit/verify` [`TestRFC9728WellKnownInsertedBetweenHostAndPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L87). **negative:** `unit/verify` [`TestRFC9728WellKnownInsertedBeforeQuery`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L323). **negative:** `unit/verify` [`TestRFC9728WellKnownInsertedBetweenHostAndPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L88) |
| `RFC9728-3-3` | The well-known URI path suffix used MUST be registered in the "Well-Known URIs" registry (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC9728RegisteredSuffix`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L135). **negative:** `unit/verify` [`TestRFC9728RegisteredSuffix`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L136) |
| `RFC9728-3-4` | An OAuth 2.0 application using this specification MUST specify what well-known URI suffix it will use for this purpose (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC9728RegisteredSuffix`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L137). **negative:** `unit/verify` [`TestRFC9728RegisteredSuffix`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L138) |
| `RFC9728-3.1-3` | If the resource identifier value contains a path or query component, any terminating slash (/) following the host component MUST be removed before inserting /.well-known/ and the well-known URI path suffix between the host component and the path and/or query components. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC9728TerminatingSlashRemovedBeforeInsertion`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L111). **negative:** `unit/verify` [`TestRFC9728TerminatingSlashRemovedBeforeInsertion`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L112) |
| `RFC9728-3.2-3` | A successful response MUST use the 200 OK HTTP status code and return a JSON object using the application/json content type that contains a set of metadata parameters as its members that are a subset of the metadata parameters defined in Section 2. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC9728ResponseShape`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L169). **negative:** `unit/verify` [`TestRFC9728ResponseShape`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L170) |
| `RFC9728-3.2-4` | Parameters with zero values MUST be omitted from the response (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC9728ZeroValuedParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L191). **negative:** `unit/verify` [`TestRFC9728ZeroValuedParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L192) |
| `RFC9728-3.3-1` | The resource value returned MUST be identical to the protected resource's resource identifier value into which the well-known URI path suffix was inserted to create the URL used to retrieve the metadata. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC9728ResourceMatchesInsertedIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L207). **negative:** `unit/verify` [`TestRFC9728ResourceMatchesInsertedIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L208) |
| `RFC9728-3.3-2` | The resource value returned MUST be identical to the protected resource's resource identifier value into which the well-known URI path suffix was inserted to create the URL used to retrieve the metadata. If these values are not identical, the data contained in the response MUST NOT be used. If the protected resource metadata was retrieved from a URL returned by the protected resource via the WWW-Authenticate resource_metadata parameter, then the resource value returned MUST be identical to the URL that the client used to make the request to the resource server. If these values are not identical, the data contained in the response MUST NOT be used. (§3.3) | MUST NOT | 3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §3.3 forbids using a mismatched metadata response, a consumer operation; §7.3 makes the subject explicit: "the client MUST ensure" the resource identifiers match. Thomas excluded protected-resource discovery clients on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata publishes the configured identity; internal/component/mcp/streamable_auth.go::buildAuthForMode fetches AS metadata only and consumes no resource metadata response |
| `RFC9728-3.3-3` | If the protected resource metadata was retrieved from a URL returned by the protected resource via the WWW-Authenticate resource_metadata parameter, then the resource value returned MUST be identical to the URL that the client used to make the request to the resource server. (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §3.3 conditions this consumer check on metadata "retrieved from a URL returned by the protected resource via the WWW-Authenticate resource_metadata parameter". Thomas excluded protected-resource discovery clients on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata and internal/component/mcp/streamable.go::ServeHTTP produce the document/challenge, but never follow another resource's challenge or consume its metadata |
| `RFC9728-3.3-4` | The recipient MUST validate that any signed metadata was signed by a key belonging to the issuer and that the signature is valid (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "Consumers of the metadata MAY ignore the signed metadata if they do not support this feature."; Thomas excluded signed metadata and resource-discovery clients on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata emits only plain JSON; internal/component/mcp/streamable_auth.go::buildAuthForMode constructs a local access-token verifier, not a protected-resource signed-metadata recipient |
| `RFC9728-6-1` | Unicode Normalization [USA15] MUST NOT be applied at any point to either the JSON string or the string it is to be compared against. (§6) | MUST NOT | 6 | **positive:** `unit/verify` [`TestRFC9728ResourceIdentityPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L230). **negative:** `unit/verify` [`TestRFC9728ResourceIdentityPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L231) |
| `RFC9728-6-2` | Comparisons between the two strings MUST be performed as a Unicode code-point-to-code-point equality comparison. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC9728ResourceIdentityPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L232). **negative:** `unit/verify` [`TestRFC9728ResourceIdentityPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L233) |
| `RFC9728-6-3` | Therefore, comparisons between JSON strings and other Unicode strings MUST be performed as specified below: 1. Remove any JSON-applied escaping to produce an array of Unicode code points. (§6, step 1) | MUST | 6 | **positive:** `unit/verify` [`TestRFC9728EscapedAudienceMatchesResource`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_jwt_test.go#L48). **negative:** `unit/verify` [`TestRFC9728EscapeTextIsNotUnescapedTwice`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_jwt_test.go#L69) |
| `RFC9728-7.1-1` | Implementations MUST support TLS (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC9728MCPServingTLSVersions`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_test.go#L124). **negative:** `unit/verify` [`TestRFC9728MCPListenerSupportsTLS`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_test.go#L74) |
| `RFC9728-7.1-2` | They MUST follow the guidance in [BCP195], which provides recommendations and requirements for improving the security of deployed services that use TLS. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC9728MCPListenerNegotiatesOnlyBCP195Suites`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_tls_suites_test.go#L28). **positive:** `unit/verify` [`TestRFC9728MCPServingTLSVersions`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_test.go#L125). **negative:** `unit/verify` [`TestRFC9728MCPListenerNegotiatesOnlyBCP195Suites`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_tls_suites_test.go#L29). **negative:** `unit/verify` [`TestRFC9728MCPServingTLSVersions`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_test.go#L126) |
| `RFC9728-7.3-1` | TLS certificate checking MUST be performed by the client as described in [RFC9525] when making a protected resource metadata request. (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §7.3 specifies "by the client" and "when making a protected resource metadata request." Thomas excluded that client role on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata is the publisher; internal/component/mcp/as_metadata.go::fetchASMetadata requests AS metadata only, whose certificate checks remain required by RFC8414-6.1-3 |
| `RFC9728-7.3-2` | To prevent this, the client MUST ensure that the resource identifier URL it is using as the prefix for the metadata request exactly matches the value of the resource metadata parameter in the protected resource metadata document received by the client, as described in Section 3.3. (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §7.3 explicitly addresses "the client" and its protected-resource "metadata document received". Thomas excluded that client role on 2026-09-21. internal/component/mcp/streamable_auth.go::buildAuthForMode requests AS metadata only; internal/component/mcp/oauth.go::writeResourceMetadata publishes the local resource identity rather than accepting a remote resource identity |
| `RFC9728-2-2` | authorization_servers OPTIONAL. JSON array containing a list of OAuth authorization server issuer identifiers, as defined in [RFC8414], for authorization servers that can be used with this protected resource. (§2; the RFC marks the parameter OPTIONAL: "authorization_servers OPTIONAL. JSON array containing a list of OAuth authorization server issuer identifiers") | MAY | 2 | **positive:** `unit/verify` [`TestNewStreamable_OAuth_MetadataEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L325). **positive:** `unit/verify` [`TestRFC9728AuthorizationServersAreIssuerIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L373). **positive:** `unit/verify` [`TestResourceMetadata_Document`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_oauth_test.go#L216). **negative:** `unit/verify` [`TestRFC9728AuthorizationServersAreIssuerIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L374) |
| `RFC9728-3.2-5` | Additional metadata parameters MAY be defined and used; any metadata parameters that are not understood MUST be ignored. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §3.2 says "any metadata parameters that are not understood MUST be ignored", an operation on the received response. Thomas excluded protected-resource discovery clients on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata publishes defined fields; internal/component/mcp/as_metadata.go::fetchASMetadata consumes a different document, AS metadata, and no producer decodes a protected-resource metadata response |
| `RFC9728-5.1-1` | This specification introduces a new parameter in the WWW-Authenticate HTTP response header field to indicate the protected resource metadata URL: resource_metadata: The URL of the protected resource metadata. The response below is an example of a WWW-Authenticate header that includes the resource identifier. HTTP/1.1 401 Unauthorized WWW-Authenticate: Bearer resource_metadata= "https://resource.example.com/.well-known/oauth-protected-resource" The HTTP status code in the example response above is defined by [RFC6750]. This parameter MAY also be used in WWW-Authenticate responses using authorization schemes other than "Bearer" [RFC6750], such as the DPoP scheme defined by [RFC9449]. (§5.1) | MAY | 5.1 | **positive:** `unit/verify` [`TestNewStreamable_OAuth_RejectsMissingBearer`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L247). **positive:** `unit/verify` [`TestOAuth_Authenticate_MissingHeader`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_oauth_test.go#L48). **positive:** `unit/verify` [`TestRFC9728ChallengeNamesMetadataURL`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L412). **negative:** no negative test. **{single-polarity}:** the row is a permission, "This parameter MAY also be used in WWW-Authenticate responses", so no challenge Ze could send violates it and there is no refusal to assert; the positive proves the parameter carries the whole metadata URL of the configured resource |
| `RFC9728-7.10-1` | Implementations should utilize HTTP caching directives such as Cache-Control with max-age, as defined in [RFC9111], to enable caching of retrieved metadata for appropriate time periods. (§7.10; lowercase "should" in the RFC, which names no value) | SHOULD | 7.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-2-4` | scopes_supported RECOMMENDED. JSON array containing a list of scope values, as defined in OAuth 2.0 [RFC6749], that are used in authorization requests to request access to this protected resource. (§2) | RECOMMENDED | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-2-5` | bearer_methods_supported OPTIONAL. JSON array containing a list of the supported methods of sending an OAuth 2.0 bearer token [RFC6750] to the protected resource. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-7.10-2` | Normal HTTP caching behaviors apply, meaning that the GET request may retrieve a cached copy of the content, rather than the latest copy. (§7.10) | MAY | 7.10 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9728-2-6`](#rfc9728-2-6) This URL MUST use the https scheme. (§2) | no test | no test carries this requirement id; annotated {feature-declined}: "OPTIONAL. URL of the protected resource's JSON Web Key (JWK) Set [JWK] document."; §2 describes the resource's own keys, not its external AS's verification keys. Thomas excluded response signing on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata publishes no jwks_uri; internal/component/mcp/streamable.go::ServeHTTP exposes no resource signing-key endpoint |
| [`RFC9728-2-7`](#rfc9728-2-7) When both signing and encryption keys are made available, a use (public key use) parameter value is REQUIRED for all keys in the referenced JWK Set to indicate each key's intended usage. (§2) | no test | no test carries this requirement id; annotated {feature-declined}: "OPTIONAL. URL of the protected resource's JSON Web Key (JWK) Set [JWK] document."; Thomas excluded response signing on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata publishes no resource jwks_uri and internal/component/mcp/streamable.go::ServeHTTP has no resource-key response producer. The AS verification-key cache is a different key set governed by RFC8414-2-8 |
| [`RFC9728-2-8`](#rfc9728-2-8) resource_signing_alg_values_supported OPTIONAL. JSON array containing a list of the JWS [JWS] signing algorithms (alg values) [JWA] supported by the protected resource for signing resource responses, for instance, as described in [FAPI.MessageSigning]. No default algorithms are implied if this entry is omitted. The value none MUST NOT be used. (§2) | no test | no test carries this requirement id; annotated {feature-declined}: "OPTIONAL. JSON array containing a list of the JWS [JWS] signing algorithms (alg values) [JWA] supported by the protected resource for signing resource responses, for instance, as described in [FAPI.MessageSigning]."; Thomas excluded response signing on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata emits no resource_signing_alg_values_supported; token-verification algorithms are not resource-response signing capabilities |
| [`RFC9728-2.1-1`](#rfc9728-2.1-1) If any human-readable field is sent without a language tag, parties using it MUST NOT make any assumptions about the language, character set, or script of the string value (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: §2.1 addresses "parties using it" when a human-readable field is received. Thomas excluded the protected-resource discovery-client role on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata emits resource/authorization_servers/scopes_supported/bearer_methods_supported only; internal/component/mcp/as_metadata.go::fetchASMetadata consumes AS issuer and jwks_uri, not human-readable resource metadata or a metadata user interface |
| [`RFC9728-2.1-2`](#rfc9728-2.1-2) If any human-readable field is sent without a language tag, parties using it MUST NOT make any assumptions about the language, character set, or script of the string value, and the string value MUST be used as is wherever it is presented in a user interface. (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: §2.1 conditions this on "wherever it is presented in a user interface." Thomas excluded the protected-resource discovery-client role on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata is a JSON publisher, not a metadata-consuming UI; internal/component/mcp/as_metadata.go::fetchASMetadata reads AS issuer and jwks_uri without displaying resource names or documentation |
| [`RFC9728-2.2-1`](#rfc9728-2.2-1) The signed metadata MUST be digitally signed or MACed (protected with a Message Authentication Code) using a JSON Web Signature (JWS) [JWS] and MUST contain an iss (issuer) claim denoting the party attesting to the claims in the signed metadata. (§2.2) | no test | no test carries this requirement id; annotated {feature-declined}: "In addition to JSON elements, metadata values MAY also be provided as a signed_metadata value, which is a JSON Web Token (JWT) [JWT] that asserts metadata values about the protected resource as a bundle."; Thomas excluded signed metadata on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata emits plain JSON and no signed_metadata parameter |
| [`RFC9728-2.2-2`](#rfc9728-2.2-2) If the consumer of the metadata supports signed metadata, metadata values conveyed in the signed metadata MUST take precedence over the corresponding values conveyed using plain JSON elements (§2.2) | no test | no test carries this requirement id; annotated {feature-declined}: "Consumers of the metadata MAY ignore the signed metadata if they do not support this feature."; Thomas excluded signed metadata and resource-discovery clients on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata publishes plain JSON; internal/component/mcp/streamable_auth.go::buildAuthForMode consumes AS issuer/keys, not protected-resource signed metadata |
| [`RFC9728-3.3-2`](#rfc9728-3.3-2) The resource value returned MUST be identical to the protected resource's resource identifier value into which the well-known URI path suffix was inserted to create the URL used to retrieve the metadata. If these values are not identical, the data contained in the response MUST NOT be used. If the protected resource metadata was retrieved from a URL returned by the protected resource via the WWW-Authenticate resource_metadata parameter, then the resource value returned MUST be identical to the URL that the client used to make the request to the resource server. If these values are not identical, the data contained in the response MUST NOT be used. (§3.3) | no test | no test carries this requirement id; annotated {not-applicable}: §3.3 forbids using a mismatched metadata response, a consumer operation; §7.3 makes the subject explicit: "the client MUST ensure" the resource identifiers match. Thomas excluded protected-resource discovery clients on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata publishes the configured identity; internal/component/mcp/streamable_auth.go::buildAuthForMode fetches AS metadata only and consumes no resource metadata response |
| [`RFC9728-3.3-3`](#rfc9728-3.3-3) If the protected resource metadata was retrieved from a URL returned by the protected resource via the WWW-Authenticate resource_metadata parameter, then the resource value returned MUST be identical to the URL that the client used to make the request to the resource server. (§3.3) | no test | no test carries this requirement id; annotated {not-applicable}: §3.3 conditions this consumer check on metadata "retrieved from a URL returned by the protected resource via the WWW-Authenticate resource_metadata parameter". Thomas excluded protected-resource discovery clients on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata and internal/component/mcp/streamable.go::ServeHTTP produce the document/challenge, but never follow another resource's challenge or consume its metadata |
| [`RFC9728-3.3-4`](#rfc9728-3.3-4) The recipient MUST validate that any signed metadata was signed by a key belonging to the issuer and that the signature is valid (§3.3) | no test | no test carries this requirement id; annotated {feature-declined}: "Consumers of the metadata MAY ignore the signed metadata if they do not support this feature."; Thomas excluded signed metadata and resource-discovery clients on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata emits only plain JSON; internal/component/mcp/streamable_auth.go::buildAuthForMode constructs a local access-token verifier, not a protected-resource signed-metadata recipient |
| [`RFC9728-7.3-1`](#rfc9728-7.3-1) TLS certificate checking MUST be performed by the client as described in [RFC9525] when making a protected resource metadata request. (§7.3) | no test | no test carries this requirement id; annotated {not-applicable}: §7.3 specifies "by the client" and "when making a protected resource metadata request." Thomas excluded that client role on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata is the publisher; internal/component/mcp/as_metadata.go::fetchASMetadata requests AS metadata only, whose certificate checks remain required by RFC8414-6.1-3 |
| [`RFC9728-7.3-2`](#rfc9728-7.3-2) To prevent this, the client MUST ensure that the resource identifier URL it is using as the prefix for the metadata request exactly matches the value of the resource metadata parameter in the protected resource metadata document received by the client, as described in Section 3.3. (§7.3) | no test | no test carries this requirement id; annotated {not-applicable}: §7.3 explicitly addresses "the client" and its protected-resource "metadata document received". Thomas excluded that client role on 2026-09-21. internal/component/mcp/streamable_auth.go::buildAuthForMode requests AS metadata only; internal/component/mcp/oauth.go::writeResourceMetadata publishes the local resource identity rather than accepting a remote resource identity |
| [`RFC9728-3.2-5`](#rfc9728-3.2-5) Additional metadata parameters MAY be defined and used; any metadata parameters that are not understood MUST be ignored. (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: §3.2 says "any metadata parameters that are not understood MUST be ignored", an operation on the received response. Thomas excluded protected-resource discovery clients on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata publishes defined fields; internal/component/mcp/as_metadata.go::fetchASMetadata consumes a different document, AS metadata, and no producer decodes a protected-resource metadata response |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9728-2-1`](#rfc9728-2-1)

resource REQUIRED. The protected resource's resource identifier, as defined in Section 1.2. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a metadata document without resource, or with a value other than the resource identifier. Red: oauth_test.go writeResourceMetadata test fails on got["resource"] != "https://mcp.example/"; oauth_e2e_test.go checks the served document carries resource. Single-polarity marker on the row: the field is emitted unconditionally.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestNewStreamable_OAuth_MetadataEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L321) | unit/verify | unproven |
| positive | [`TestResourceMetadata_Document`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_oauth_test.go#L212) | unit/verify | unproven |

### [`RFC9728-2-6`](#rfc9728-2-6)

This URL MUST use the https scheme. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-2-6, so no unit is bound to it.

### [`RFC9728-2-7`](#rfc9728-2-7)

When both signing and encryption keys are made available, a use (public key use) parameter value is REQUIRED for all keys in the referenced JWK Set to indicate each key's intended usage. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-2-7, so no unit is bound to it.

### [`RFC9728-2-8`](#rfc9728-2-8)

resource_signing_alg_values_supported OPTIONAL. JSON array containing a list of the JWS [JWS] signing algorithms (alg values) [JWA] supported by the protected resource for signing resource responses, for instance, as described in [FAPI.MessageSigning]. No default algorithms are implied if this entry is omitted. The value none MUST NOT be used. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-2-8, so no unit is bound to it.

### [`RFC9728-2.1-1`](#rfc9728-2.1-1)

If any human-readable field is sent without a language tag, parties using it MUST NOT make any assumptions about the language, character set, or script of the string value (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-2.1-1, so no unit is bound to it.

### [`RFC9728-2.1-2`](#rfc9728-2.1-2)

If any human-readable field is sent without a language tag, parties using it MUST NOT make any assumptions about the language, character set, or script of the string value, and the string value MUST be used as is wherever it is presented in a user interface. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-2.1-2, so no unit is bound to it.

### [`RFC9728-2.2-1`](#rfc9728-2.2-1)

The signed metadata MUST be digitally signed or MACed (protected with a Message Authentication Code) using a JSON Web Signature (JWS) [JWS] and MUST contain an iss (issuer) claim denoting the party attesting to the claims in the signed metadata. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-2.2-1, so no unit is bound to it.

### [`RFC9728-2.2-2`](#rfc9728-2.2-2)

If the consumer of the metadata supports signed metadata, metadata values conveyed in the signed metadata MUST take precedence over the corresponding values conveyed using plain JSON elements (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-2.2-2, so no unit is bound to it.

### [`RFC9728-3-2`](#rfc9728-3-2)

Protected resources supporting metadata MUST make a JSON document containing metadata as specified in Section 2 available at a URL formed by inserting a well-known URI string into the protected resource's resource identifier between the host component and the path and/or query components, if any. (§3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Path clause: TestRFC9728WellKnownInsertedBetweenHostAndPath. Query clause now proven by TestRFC9728WellKnownInsertedBeforeQuery: + https://mcp.example/mcp?tenant=a at /.well-known/oauth-protected-resource/mcp?tenant=a and query-only https://mcp.example?tenant=a at /.well-known/oauth-protected-resource?tenant=a, advertised by resourceMetadataURL, resource equal to the identifier; - query dropped, suffix after path, bare suffix, suffix after query each serve no document. Still weak: resourceOriginAndPath TrimRights the path's trailing slash, so https://mcp.example/mcp/ is published at .../oauth-protected-resource/mcp, not at the URL formed from the identifier, and no tagged assertion covers that input. Blocked by spec-mcp-protected-resource-metadata-path AC-5 (same defect as RFC9728-3.1-3).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9728WellKnownInsertedBeforeQuery`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L323) | unit/verify | revert, verified |
| negative | [`TestRFC9728WellKnownInsertedBetweenHostAndPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L88) | unit/verify | revert, verified |
| positive | [`TestRFC9728WellKnownInsertedBeforeQuery`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L322) | unit/verify | revert, verified |
| positive | [`TestRFC9728WellKnownInsertedBetweenHostAndPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L87) | unit/verify | revert, verified |

### [`RFC9728-3-3`](#rfc9728-3-3)

The well-known URI path suffix used MUST be registered in the "Well-Known URIs" registry (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9728RegisteredSuffix`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L136) | unit/verify | revert, verified |
| positive | [`TestRFC9728RegisteredSuffix`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L135) | unit/verify | revert, verified |

### [`RFC9728-3-4`](#rfc9728-3-4)

An OAuth 2.0 application using this specification MUST specify what well-known URI suffix it will use for this purpose (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9728RegisteredSuffix`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L138) | unit/verify | revert, verified |
| positive | [`TestRFC9728RegisteredSuffix`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L137) | unit/verify | revert, verified |

### [`RFC9728-3.1-3`](#rfc9728-3.1-3)

If the resource identifier value contains a path or query component, any terminating slash (/) following the host component MUST be removed before inserting /.well-known/ and the well-known URI path suffix between the host component and the path and/or query components. (§3.1)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The sentence removes only the terminating slash following the host component when the identifier has a path or query. TestRFC9728TerminatingSlashRemovedBeforeInsertion asserts https://mcp.example/mcp/ maps to .../oauth-protected-resource/mcp and its negative forbids any trailing slash: that is the path's own slash, which the RFC keeps (the path is inserted unchanged). The test pins resourceOriginAndPath's strings.TrimRight(u.EscapedPath(), "/"), which conflates /mcp/ and /mcp. The query case (e.g. https://mcp.example/?x=1) is not asserted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9728TerminatingSlashRemovedBeforeInsertion`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L112) | unit/verify | revert, verified |
| positive | [`TestRFC9728TerminatingSlashRemovedBeforeInsertion`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L111) | unit/verify | revert, verified |

### [`RFC9728-3.2-3`](#rfc9728-3.2-3)

A successful response MUST use the 200 OK HTTP status code and return a JSON object using the application/json content type that contains a set of metadata parameters as its members that are a subset of the metadata parameters defined in Section 2. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a success status other than 200, a content type other than application/json, a body that is not a JSON object, a member outside Section 2. Red: TestRFC9728ResponseShape fails on w.Code != 200, on Content-Type != application/json, on json.Unmarshal into a map (arrays/scalars refused), and on any member absent from rfc9728Section2Members.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9728ResponseShape`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L170) | unit/verify | revert, verified |
| positive | [`TestRFC9728ResponseShape`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L169) | unit/verify | revert, verified |

### [`RFC9728-3.2-4`](#rfc9728-3.2-4)

Parameters with zero values MUST be omitted from the response (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9728ZeroValuedParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L192) | unit/verify | revert, verified |
| positive | [`TestRFC9728ZeroValuedParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L191) | unit/verify | revert, verified |

### [`RFC9728-3.3-1`](#rfc9728-3.3-1)

The resource value returned MUST be identical to the protected resource's resource identifier value into which the well-known URI path suffix was inserted to create the URL used to retrieve the metadata. (§3.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestRFC9728ResourceMatchesInsertedIdentifier proves one identifier: resource https://mcp.example/api at .../oauth-protected-resource/api, and that the audience override is not returned. It does not cover an identifier with a trailing path slash: for https://mcp.example/mcp/ Ze forms the URL from /mcp (TrimRight in resourceOriginAndPath) and returns resource https://mcp.example/mcp/, so the returned value differs from the identifier the URL was formed from, and no tagged assertion goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9728ResourceMatchesInsertedIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L208) | unit/verify | revert, verified |
| positive | [`TestRFC9728ResourceMatchesInsertedIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L207) | unit/verify | revert, verified |

### [`RFC9728-3.3-2`](#rfc9728-3.3-2)

The resource value returned MUST be identical to the protected resource's resource identifier value into which the well-known URI path suffix was inserted to create the URL used to retrieve the metadata. If these values are not identical, the data contained in the response MUST NOT be used. If the protected resource metadata was retrieved from a URL returned by the protected resource via the WWW-Authenticate resource_metadata parameter, then the resource value returned MUST be identical to the URL that the client used to make the request to the resource server. If these values are not identical, the data contained in the response MUST NOT be used. (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-3.3-2, so no unit is bound to it.

### [`RFC9728-3.3-3`](#rfc9728-3.3-3)

If the protected resource metadata was retrieved from a URL returned by the protected resource via the WWW-Authenticate resource_metadata parameter, then the resource value returned MUST be identical to the URL that the client used to make the request to the resource server. (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-3.3-3, so no unit is bound to it.

### [`RFC9728-3.3-4`](#rfc9728-3.3-4)

The recipient MUST validate that any signed metadata was signed by a key belonging to the issuer and that the signature is valid (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-3.3-4, so no unit is bound to it.

### [`RFC9728-6-1`](#rfc9728-6-1)

Unicode Normalization [USA15] MUST NOT be applied at any point to either the JSON string or the string it is to be compared against. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: normalizing either string. Red: TestRFC9728ResourceIdentityPreserved fails if the published resource is not the exact decomposed e+U+0301 identifier (returned != resource), and fails if the composed spelling %C3%A9 retrieves the document (status != 404).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9728ResourceIdentityPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L231) | unit/verify | unproven |
| positive | [`TestRFC9728ResourceIdentityPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L230) | unit/verify | unproven |

### [`RFC9728-6-2`](#rfc9728-6-2)

Comparisons between the two strings MUST be performed as a Unicode code-point-to-code-point equality comparison. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: equality other than code-point to code-point. Red: TestRFC9728ResourceIdentityPreserved serves the exact escaped path+query (decodeMetadata requires 200) and requires 404 for the composed spelling, a different query value and a missing query.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9728ResourceIdentityPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L233) | unit/verify | unproven |
| positive | [`TestRFC9728ResourceIdentityPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L232) | unit/verify | unproven |

### [`RFC9728-6-3`](#rfc9728-6-3)

Therefore, comparisons between JSON strings and other Unicode strings MUST be performed as specified below: 1. Remove any JSON-applied escaping to produce an array of Unicode code points. (§6, step 1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: comparing without removing JSON escaping, or removing it more than once. Red: TestRFC9728EscapedAudienceMatchesResource fails if verifyJWT refuses an aud with \u0026 against the & identifier; TestRFC9728EscapeTextIsNotUnescapedTwice fails unless a literal \u0026 aud is refused with errJWTAudienceMismatch.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9728EscapeTextIsNotUnescapedTwice`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_jwt_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestRFC9728EscapedAudienceMatchesResource`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_jwt_test.go#L48) | unit/verify | revert, verified |

### [`RFC9728-7.1-1`](#rfc9728-7.1-1)

Implementations MUST support TLS (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9728MCPListenerSupportsTLS`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_test.go#L74) | unit/verify | unproven |
| positive | [`TestRFC9728MCPServingTLSVersions`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_test.go#L124) | unit/verify | unproven |

### [`RFC9728-7.1-2`](#rfc9728-7.1-2)

They MUST follow the guidance in [BCP195], which provides recommendations and requirements for improving the security of deployed services that use TLS. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. Beyond the HEAD version tags: TestRFC9728MCPListenerNegotiatesOnlyBCP195Suites (ze_mcp) probes the listener startMCPServer opens with loadMCPTLSConfig, ECDSA and RSA certs, with the same tlsprobe assertions as RFC7858-8-1: recommended ECDHE AES-128-GCM selected with null compression and renegotiation_info, every forbidden suite alone never selected, DEFLATE alone refused. Records: loadMCPTLSConfig revert both polarities observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9728MCPServingTLSVersions`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_test.go#L126) | unit/verify | revert, verified |
| negative | [`TestRFC9728MCPListenerNegotiatesOnlyBCP195Suites`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_tls_suites_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestRFC9728MCPServingTLSVersions`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_test.go#L125) | unit/verify | revert, verified |
| positive | [`TestRFC9728MCPListenerNegotiatesOnlyBCP195Suites`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_tls_suites_test.go#L28) | unit/verify | revert, verified |

### [`RFC9728-7.3-1`](#rfc9728-7.3-1)

TLS certificate checking MUST be performed by the client as described in [RFC9525] when making a protected resource metadata request. (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-7.3-1, so no unit is bound to it.

### [`RFC9728-7.3-2`](#rfc9728-7.3-2)

To prevent this, the client MUST ensure that the resource identifier URL it is using as the prefix for the metadata request exactly matches the value of the resource metadata parameter in the protected resource metadata document received by the client, as described in Section 3.3. (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-7.3-2, so no unit is bound to it.

### [`RFC9728-2-2`](#rfc9728-2-2)

authorization_servers OPTIONAL. JSON array containing a list of OAuth authorization server issuer identifiers, as defined in [RFC8414], for authorization servers that can be used with this protected resource. (§2; the RFC marks the parameter OPTIONAL: "authorization_servers OPTIONAL. JSON array containing a list of OAuth authorization server issuer identifiers")

Audit verdict: enforced (the tests do what the requirement demands), fresh. [MAY] parameter definition. TestRFC9728AuthorizationServersAreIssuerIdentifiers + authorization_servers decodes as a JSON array of strings holding exactly the validated issuer, https with no query and no fragment; - http, ?tenant=a and #frag authorization-server values are refused by NewStreamable so no document lists them. The refusals are overdetermined: asMetadataURL/oauthHTTPSURL refuse first, and the fetch's issuer match or transport would also refuse, so the negative proves the published property rather than which guard holds it. The oauth_test.go and oauth_e2e_test.go positives keep their tags.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9728AuthorizationServersAreIssuerIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L374) | unit/verify | revert, verified |
| positive | [`TestNewStreamable_OAuth_MetadataEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L325) | unit/verify | revert, verified |
| positive | [`TestRFC9728AuthorizationServersAreIssuerIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L373) | unit/verify | revert, verified |
| positive | [`TestResourceMetadata_Document`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_oauth_test.go#L216) | unit/verify | revert, verified |

### [`RFC9728-3.2-5`](#rfc9728-3.2-5)

Additional metadata parameters MAY be defined and used; any metadata parameters that are not understood MUST be ignored. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-3.2-5, so no unit is bound to it.

### [`RFC9728-5.1-1`](#rfc9728-5.1-1)

This specification introduces a new parameter in the WWW-Authenticate HTTP response header field to indicate the protected resource metadata URL: resource_metadata: The URL of the protected resource metadata. The response below is an example of a WWW-Authenticate header that includes the resource identifier. HTTP/1.1 401 Unauthorized WWW-Authenticate: Bearer resource_metadata= "https://resource.example.com/.well-known/oauth-protected-resource" The HTTP status code in the example response above is defined by [RFC6750]. This parameter MAY also be used in WWW-Authenticate responses using authorization schemes other than "Bearer" [RFC6750], such as the DPoP scheme defined by [RFC9449]. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. [MAY] permission row with {single-polarity: positive}, which holds: no challenge Ze sends can violate a permission. TestRFC9728ChallengeNamesMetadataURL asserts the 401 WWW-Authenticate is Bearer with resource_metadata EXACTLY https://mcp.example/.well-known/oauth-protected-resource/api for a metadata resource that differs from the audience, and a GET there returns that resource's document, so a prefix, the audience-derived URL, or a missing parameter goes red. The oauth_test.go and oauth_e2e_test.go prefix-only positives keep their tags.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestNewStreamable_OAuth_RejectsMissingBearer`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L247) | unit/verify | revert, verified |
| positive | [`TestRFC9728ChallengeNamesMetadataURL`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L412) | unit/verify | revert, verified |
| positive | [`TestOAuth_Authenticate_MissingHeader`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_oauth_test.go#L48) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | Main integration; independent source-mapping review: OAuthMappingReview; 2026-09-29 section 3.1 re-walk: services verdict-fix author (spec-rfc-verdict-fix-services), for independent re-judge |
| Signed off | 2026-09-29 |
| Register | rfc2119 |
| Source | rfc/full/rfc9728.txt |
| Source fingerprint | 77e3256aa4514f31 |
| Record | rfc/extraction/rfc9728.json |
| Mapped sentences | 25 |
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
| `2` | not stated | 4 | walked | Section 2 marks authorization_servers and bearer_methods_supported OPTIONAL, and scopes_supported RECOMMENDED. The retained advisory IDs have no separate MUST-level inventory sites. |
| `2.1` | not stated | 1 | walked | The sentence mapped at 2.1:1 contains both RFC9728-2.1-1 and RFC9728-2.1-2: If any human-readable field is sent without a language tag, parties using it MUST NOT make any assumptions about the language, character set, or script of the string value, and the string value MUST be used as is wherever it is presented in a user interface. One site has one mapped-to field, so the second MUST is accounted for here under its existing ID. |
| `2.2` | not stated | 2 | walked | not stated |
| `3` | not stated | 3 | walked | not stated |
| `3.1` | not stated | 2 | walked | not stated |
| `3.2` | not stated | 3 | walked | not stated |
| `3.3` | not stated | 5 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | Section 5.1 defines resource_metadata as the URL of the protected resource metadata and permits its use with non-Bearer schemes using MAY. The retained advisory ID has no separate MUST-level inventory site. |
| `5.2` | not stated | 0 | walked | not stated |
| `5.3` | not stated | 0 | walked | not stated |
| `5.4` | not stated | 0 | walked | not stated |
| `6` | not stated | 3 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 2 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `7.3` | not stated | 2 | walked | not stated |
| `7.4` | not stated | 0 | walked | not stated |
| `7.5` | not stated | 0 | walked | not stated |
| `7.6` | not stated | 0 | walked | not stated |
| `7.7` | not stated | 0 | walked | not stated |
| `7.8` | not stated | 0 | walked | not stated |
| `7.9` | not stated | 0 | walked | not stated |
| `7.10` | not stated | 0 | walked | Section 7.10 says normal HTTP caching behaviours apply and recommends caching directives with lowercase should. The existing advisory IDs record that text and have no MUST-level inventory sites. |
| `8` | Registration procedure addresses IANA and designated experts | 0 | walked | Registration procedure addresses IANA and designated experts. Its lowercase registry instructions do not create an additional MUST-level site in this document's selected inventory. |
| `8.1` | not stated | 0 | walked | not stated |
| `8.1.1` | not stated | 0 | walked | not stated |
| `8.1.2` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `8.2.1` | not stated | 0 | walked | not stated |
| `8.3` | not stated | 0 | walked | not stated |
| `8.3.1` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the protected-resource metadata client, the party that queries the document, which Ze does not implement: Thomas's 2026-09-21 scope decision excludes protected-resource discovery clients. Ze publishes the document (internal/component/mcp/oauth.go::writeResourceMetadata, served by internal/component/mcp/streamable.go::ServeHTTP) and never requests another resource's metadata; internal/component/mcp/as_metadata.go::fetchASMetadata queries RFC 8414 authorization server metadata, a different document. No layer acts as this client on Ze's behalf. The row RFC9728-3.1-2 was retired 2026-09-29 (rfc/corrections/rfc9728.md). | A protected resource metadata document MUST be queried using an HTTP GET request at the previously specified URL. |
| `3.3:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.3 repeats: If these values are not identical, the data contained in the response MUST NOT be used. The first occurrence follows the well-known URL identity check and maps directly at 3.3:2; this occurrence follows discovery through WWW-Authenticate. RFC9728-3.3-2 now explicitly includes both contexts, so the duplicate target records the obligation of this sentence as well. | If these values are not identical, the data contained in the response MUST NOT be used. |

## Superseded

No document obsoletes RFC 9728, so its obligations are stated where they were written.
