# RFC 9728 - OAuth 2.0 Protected Resource Metadata

No row in the public ledger. Every requirement this repository extracted from RFC 9728, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 26 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 3.8% | 1 of 26 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 26 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 6 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 26 | of 32 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 26 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 3.8% | 1 of 26 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 26 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 26 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 92.3% | 24 of 26 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 26 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 32 |
| Gated MUST-level | 26 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 0 |
| Gated with no test | 24 |
| Nightly-only evidence | 0 |
| Test tags | 6 |
| Tagged units | 6 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9728.md` |
| Requirement shard | `rfc/requirements/rfc9728.md` |
| RFC text | `rfc/full/rfc9728.txt` |

## Enrolment

Enrolled: OAuth 2.0 Protected Resource Metadata (ze publishes the protected-resource metadata as an OAuth resource server). The checklist was walked against the RFC's own text on 2026-09-21 and rewritten: it now carries 24 MUST-level rows, of which 2-1 (the metadata contains `resource`) is {single-polarity: positive} and 3.2-5 (unknown parameters are ignored, the row RFC9728-2-3 carried until its section citation was corrected to §3.2) is {not-applicable}. The other 22 were added from sentences the checklist had not carried and none of them has a test: §2 JWK Set and signing-algorithm rules, §2.1 language tags, §2.2 signed metadata, §3 and §3.1 document location and request, §3.2 response shape, §3.3 validation, §6 string comparison, §7.1 TLS and §7.3 certificate checking. Three rows were deleted because no sentence in RFC 9728 states them at any level: 3.1-1 (the metadata endpoint is reachable without authentication), 5.1-2 (the resource_metadata URL is absolute) and 3-1 (metadata TLS matches the resource, plaintext only on loopback). internal/component/mcp/oauth_e2e_test.go and oauth_test.go still carry `RFC requirement:` tags naming 3.1-1 and 5.1-2.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 9728.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 24 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **26** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (2):** [`RFC9728-2-1`](#rfc9728-2-1), [`RFC9728-3.2-5`](#rfc9728-3.2-5)

**No test and no annotation (24):** [`RFC9728-2-6`](#rfc9728-2-6), [`RFC9728-2-7`](#rfc9728-2-7), [`RFC9728-2-8`](#rfc9728-2-8), [`RFC9728-2.1-1`](#rfc9728-2.1-1), [`RFC9728-2.1-2`](#rfc9728-2.1-2), [`RFC9728-2.2-1`](#rfc9728-2.2-1), [`RFC9728-2.2-2`](#rfc9728-2.2-2), [`RFC9728-3-2`](#rfc9728-3-2), [`RFC9728-3-3`](#rfc9728-3-3), [`RFC9728-3-4`](#rfc9728-3-4), [`RFC9728-3.1-2`](#rfc9728-3.1-2), [`RFC9728-3.1-3`](#rfc9728-3.1-3), [`RFC9728-3.2-3`](#rfc9728-3.2-3), [`RFC9728-3.2-4`](#rfc9728-3.2-4), [`RFC9728-3.3-1`](#rfc9728-3.3-1), [`RFC9728-3.3-2`](#rfc9728-3.3-2), [`RFC9728-3.3-3`](#rfc9728-3.3-3), [`RFC9728-3.3-4`](#rfc9728-3.3-4), [`RFC9728-6-1`](#rfc9728-6-1), [`RFC9728-6-2`](#rfc9728-6-2), [`RFC9728-7.1-1`](#rfc9728-7.1-1), [`RFC9728-7.1-2`](#rfc9728-7.1-2), [`RFC9728-7.3-1`](#rfc9728-7.3-1), [`RFC9728-7.3-2`](#rfc9728-7.3-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9728-2-1` | The `resource` field MUST be present in the metadata document and contain the canonical URL identifying the resource (§2) | MUST | 2 | **positive:** `unit/verify` [`TestNewStreamable_OAuth_MetadataEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L290). **positive:** `unit/verify` [`TestResourceMetadata_Document`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_test.go#L212). **negative:** no negative test. **{single-polarity}:** the resource field is emitted unconditionally by writeResourceMetadata (internal/component/mcp/oauth.go:170-171,184-192), so no input can make it absent and there is no negative to assert |
| `RFC9728-2-6` | The `jwks_uri` URL MUST use the https scheme (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-2-7` | When both signing and encryption keys are made available in the referenced JWK Set, a `use` (public key use) parameter value is REQUIRED for all keys to indicate each key's intended usage (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-2-8` | The value `none` MUST NOT be used in `resource_signing_alg_values_supported` (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-2.1-1` | If any human-readable field is sent without a language tag, parties using it MUST NOT make any assumptions about the language, character set, or script of the string value (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-2.1-2` | A human-readable string value sent without a language tag MUST be used as is wherever it is presented in a user interface (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-2.2-1` | Signed metadata MUST be digitally signed or MACed using a JWS and MUST contain an `iss` (issuer) claim denoting the party attesting to the claims in the signed metadata (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-2.2-2` | If the consumer of the metadata supports signed metadata, metadata values conveyed in the signed metadata MUST take precedence over the corresponding values conveyed using plain JSON elements (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-3-2` | Protected resources supporting metadata MUST make a JSON document containing the Section 2 metadata available at a URL formed by inserting a well-known URI string into the resource identifier between the host component and the path and/or query components (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-3-3` | The well-known URI path suffix used MUST be registered in the "Well-Known URIs" registry (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-3-4` | An OAuth 2.0 application using this specification MUST specify what well-known URI suffix it will use for this purpose (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-3.1-2` | A protected resource metadata document MUST be queried using an HTTP GET request at the specified URL (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-3.1-3` | If the resource identifier value contains a path or query component, any terminating slash following the host component MUST be removed before inserting /.well-known/ and the well-known URI path suffix (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-3.2-3` | A successful response MUST use the 200 OK HTTP status code and return a JSON object using the application/json content type whose members are a subset of the metadata parameters defined in Section 2 (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-3.2-4` | Parameters with zero values MUST be omitted from the response (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-3.3-1` | The `resource` value returned MUST be identical to the protected resource's resource identifier value into which the well-known URI path suffix was inserted to create the URL used to retrieve the metadata (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-3.3-2` | If the `resource` value and the resource identifier are not identical, the data contained in the response MUST NOT be used (§3.3) | MUST NOT | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-3.3-3` | If the metadata was retrieved from a URL returned by the protected resource via the WWW-Authenticate `resource_metadata` parameter, the `resource` value returned MUST be identical to the URL that the client used to make the request to the resource server (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-3.3-4` | The recipient MUST validate that any signed metadata was signed by a key belonging to the issuer and that the signature is valid (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-6-1` | Unicode Normalization MUST NOT be applied at any point to either the JSON string or the string it is to be compared against (§6) | MUST NOT | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-6-2` | Comparisons between two strings MUST be performed as a Unicode code-point-to-code-point equality comparison (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-7.1-1` | Implementations MUST support TLS (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-7.1-2` | Implementations MUST follow the guidance in BCP 195 (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-7.3-1` | TLS certificate checking MUST be performed by the client as described in RFC 9525 when making a protected resource metadata request (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-7.3-2` | The client MUST ensure that the resource identifier URL it is using as the prefix for the metadata request exactly matches the value of the `resource` metadata parameter in the protected resource metadata document received (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-2-2` | The `authorization_servers` field MAY be included as a JSON array of AS issuer URLs (§2; the RFC marks the parameter OPTIONAL: "authorization_servers OPTIONAL. JSON array containing a list of OAuth authorization server issuer identifiers") | MAY | 2 | **positive:** `unit/verify` [`TestNewStreamable_OAuth_MetadataEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L294). **positive:** `unit/verify` [`TestResourceMetadata_Document`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_test.go#L216). **negative:** no negative test |
| `RFC9728-3.2-5` | Metadata parameters that are not understood MUST be ignored by the consumer of the metadata response (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this obligation binds an OAuth client consuming the metadata to ignore unrecognized members; ze is the protected-resource-metadata publisher (internal/component/mcp/oauth.go:169-179 emits only defined fields) and does not consume the document |
| `RFC9728-5.1-1` | The `resource_metadata` parameter of the WWW-Authenticate response header field carries the URL of the protected resource metadata, and MAY also be used in WWW-Authenticate responses using authorization schemes other than "Bearer" (§5.1) | MAY | 5.1 | **positive:** `unit/verify` [`TestNewStreamable_OAuth_RejectsMissingBearer`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L216). **positive:** `unit/verify` [`TestOAuth_Authenticate_MissingHeader`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_test.go#L48). **negative:** no negative test |
| `RFC9728-7.10-1` | Implementations should utilize HTTP caching directives such as Cache-Control with max-age to enable caching of retrieved metadata for appropriate time periods (§7.10; lowercase "should" in the RFC, which names no value) | SHOULD | 7.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-2-4` | The `scopes_supported` field MAY be included as a JSON array of scope names (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-2-5` | The `bearer_methods_supported` field MAY be included as an array of delivery methods (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-7.10-2` | Normal HTTP caching behaviors apply, so a metadata GET request may retrieve a cached copy rather than the latest copy (§7.10) | MAY | 7.10 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9728-2-6`](#rfc9728-2-6) The `jwks_uri` URL MUST use the https scheme (§2) | no test | no test carries this requirement id |
| [`RFC9728-2-7`](#rfc9728-2-7) When both signing and encryption keys are made available in the referenced JWK Set, a `use` (public key use) parameter value is REQUIRED for all keys to indicate each key's intended usage (§2) | no test | no test carries this requirement id |
| [`RFC9728-2-8`](#rfc9728-2-8) The value `none` MUST NOT be used in `resource_signing_alg_values_supported` (§2) | no test | no test carries this requirement id |
| [`RFC9728-2.1-1`](#rfc9728-2.1-1) If any human-readable field is sent without a language tag, parties using it MUST NOT make any assumptions about the language, character set, or script of the string value (§2.1) | no test | no test carries this requirement id |
| [`RFC9728-2.1-2`](#rfc9728-2.1-2) A human-readable string value sent without a language tag MUST be used as is wherever it is presented in a user interface (§2.1) | no test | no test carries this requirement id |
| [`RFC9728-2.2-1`](#rfc9728-2.2-1) Signed metadata MUST be digitally signed or MACed using a JWS and MUST contain an `iss` (issuer) claim denoting the party attesting to the claims in the signed metadata (§2.2) | no test | no test carries this requirement id |
| [`RFC9728-2.2-2`](#rfc9728-2.2-2) If the consumer of the metadata supports signed metadata, metadata values conveyed in the signed metadata MUST take precedence over the corresponding values conveyed using plain JSON elements (§2.2) | no test | no test carries this requirement id |
| [`RFC9728-3-2`](#rfc9728-3-2) Protected resources supporting metadata MUST make a JSON document containing the Section 2 metadata available at a URL formed by inserting a well-known URI string into the resource identifier between the host component and the path and/or query components (§3) | no test | no test carries this requirement id |
| [`RFC9728-3-3`](#rfc9728-3-3) The well-known URI path suffix used MUST be registered in the "Well-Known URIs" registry (§3) | no test | no test carries this requirement id |
| [`RFC9728-3-4`](#rfc9728-3-4) An OAuth 2.0 application using this specification MUST specify what well-known URI suffix it will use for this purpose (§3) | no test | no test carries this requirement id |
| [`RFC9728-3.1-2`](#rfc9728-3.1-2) A protected resource metadata document MUST be queried using an HTTP GET request at the specified URL (§3.1) | no test | no test carries this requirement id |
| [`RFC9728-3.1-3`](#rfc9728-3.1-3) If the resource identifier value contains a path or query component, any terminating slash following the host component MUST be removed before inserting /.well-known/ and the well-known URI path suffix (§3.1) | no test | no test carries this requirement id |
| [`RFC9728-3.2-3`](#rfc9728-3.2-3) A successful response MUST use the 200 OK HTTP status code and return a JSON object using the application/json content type whose members are a subset of the metadata parameters defined in Section 2 (§3.2) | no test | no test carries this requirement id |
| [`RFC9728-3.2-4`](#rfc9728-3.2-4) Parameters with zero values MUST be omitted from the response (§3.2) | no test | no test carries this requirement id |
| [`RFC9728-3.3-1`](#rfc9728-3.3-1) The `resource` value returned MUST be identical to the protected resource's resource identifier value into which the well-known URI path suffix was inserted to create the URL used to retrieve the metadata (§3.3) | no test | no test carries this requirement id |
| [`RFC9728-3.3-2`](#rfc9728-3.3-2) If the `resource` value and the resource identifier are not identical, the data contained in the response MUST NOT be used (§3.3) | no test | no test carries this requirement id |
| [`RFC9728-3.3-3`](#rfc9728-3.3-3) If the metadata was retrieved from a URL returned by the protected resource via the WWW-Authenticate `resource_metadata` parameter, the `resource` value returned MUST be identical to the URL that the client used to make the request to the resource server (§3.3) | no test | no test carries this requirement id |
| [`RFC9728-3.3-4`](#rfc9728-3.3-4) The recipient MUST validate that any signed metadata was signed by a key belonging to the issuer and that the signature is valid (§3.3) | no test | no test carries this requirement id |
| [`RFC9728-6-1`](#rfc9728-6-1) Unicode Normalization MUST NOT be applied at any point to either the JSON string or the string it is to be compared against (§6) | no test | no test carries this requirement id |
| [`RFC9728-6-2`](#rfc9728-6-2) Comparisons between two strings MUST be performed as a Unicode code-point-to-code-point equality comparison (§6) | no test | no test carries this requirement id |
| [`RFC9728-7.1-1`](#rfc9728-7.1-1) Implementations MUST support TLS (§7.1) | no test | no test carries this requirement id |
| [`RFC9728-7.1-2`](#rfc9728-7.1-2) Implementations MUST follow the guidance in BCP 195 (§7.1) | no test | no test carries this requirement id |
| [`RFC9728-7.3-1`](#rfc9728-7.3-1) TLS certificate checking MUST be performed by the client as described in RFC 9525 when making a protected resource metadata request (§7.3) | no test | no test carries this requirement id |
| [`RFC9728-7.3-2`](#rfc9728-7.3-2) The client MUST ensure that the resource identifier URL it is using as the prefix for the metadata request exactly matches the value of the `resource` metadata parameter in the protected resource metadata document received (§7.3) | no test | no test carries this requirement id |
| [`RFC9728-3.2-5`](#rfc9728-3.2-5) Metadata parameters that are not understood MUST be ignored by the consumer of the metadata response (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: this obligation binds an OAuth client consuming the metadata to ignore unrecognized members; ze is the protected-resource-metadata publisher (internal/component/mcp/oauth.go:169-179 emits only defined fields) and does not consume the document |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9728-2-1`](#rfc9728-2-1)

The `resource` field MUST be present in the metadata document and contain the canonical URL identifying the resource (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestNewStreamable_OAuth_MetadataEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L290) | unit/verify | unproven |
| positive | [`TestResourceMetadata_Document`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_test.go#L212) | unit/verify | unproven |

### [`RFC9728-2-6`](#rfc9728-2-6)

The `jwks_uri` URL MUST use the https scheme (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-2-6, so no unit is bound to it.

### [`RFC9728-2-7`](#rfc9728-2-7)

When both signing and encryption keys are made available in the referenced JWK Set, a `use` (public key use) parameter value is REQUIRED for all keys to indicate each key's intended usage (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-2-7, so no unit is bound to it.

### [`RFC9728-2-8`](#rfc9728-2-8)

The value `none` MUST NOT be used in `resource_signing_alg_values_supported` (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-2-8, so no unit is bound to it.

### [`RFC9728-2.1-1`](#rfc9728-2.1-1)

If any human-readable field is sent without a language tag, parties using it MUST NOT make any assumptions about the language, character set, or script of the string value (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-2.1-1, so no unit is bound to it.

### [`RFC9728-2.1-2`](#rfc9728-2.1-2)

A human-readable string value sent without a language tag MUST be used as is wherever it is presented in a user interface (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-2.1-2, so no unit is bound to it.

### [`RFC9728-2.2-1`](#rfc9728-2.2-1)

Signed metadata MUST be digitally signed or MACed using a JWS and MUST contain an `iss` (issuer) claim denoting the party attesting to the claims in the signed metadata (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-2.2-1, so no unit is bound to it.

### [`RFC9728-2.2-2`](#rfc9728-2.2-2)

If the consumer of the metadata supports signed metadata, metadata values conveyed in the signed metadata MUST take precedence over the corresponding values conveyed using plain JSON elements (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-2.2-2, so no unit is bound to it.

### [`RFC9728-3-2`](#rfc9728-3-2)

Protected resources supporting metadata MUST make a JSON document containing the Section 2 metadata available at a URL formed by inserting a well-known URI string into the resource identifier between the host component and the path and/or query components (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-3-2, so no unit is bound to it.

### [`RFC9728-3-3`](#rfc9728-3-3)

The well-known URI path suffix used MUST be registered in the "Well-Known URIs" registry (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-3-3, so no unit is bound to it.

### [`RFC9728-3-4`](#rfc9728-3-4)

An OAuth 2.0 application using this specification MUST specify what well-known URI suffix it will use for this purpose (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-3-4, so no unit is bound to it.

### [`RFC9728-3.1-2`](#rfc9728-3.1-2)

A protected resource metadata document MUST be queried using an HTTP GET request at the specified URL (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-3.1-2, so no unit is bound to it.

### [`RFC9728-3.1-3`](#rfc9728-3.1-3)

If the resource identifier value contains a path or query component, any terminating slash following the host component MUST be removed before inserting /.well-known/ and the well-known URI path suffix (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-3.1-3, so no unit is bound to it.

### [`RFC9728-3.2-3`](#rfc9728-3.2-3)

A successful response MUST use the 200 OK HTTP status code and return a JSON object using the application/json content type whose members are a subset of the metadata parameters defined in Section 2 (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-3.2-3, so no unit is bound to it.

### [`RFC9728-3.2-4`](#rfc9728-3.2-4)

Parameters with zero values MUST be omitted from the response (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-3.2-4, so no unit is bound to it.

### [`RFC9728-3.3-1`](#rfc9728-3.3-1)

The `resource` value returned MUST be identical to the protected resource's resource identifier value into which the well-known URI path suffix was inserted to create the URL used to retrieve the metadata (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-3.3-1, so no unit is bound to it.

### [`RFC9728-3.3-2`](#rfc9728-3.3-2)

If the `resource` value and the resource identifier are not identical, the data contained in the response MUST NOT be used (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-3.3-2, so no unit is bound to it.

### [`RFC9728-3.3-3`](#rfc9728-3.3-3)

If the metadata was retrieved from a URL returned by the protected resource via the WWW-Authenticate `resource_metadata` parameter, the `resource` value returned MUST be identical to the URL that the client used to make the request to the resource server (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-3.3-3, so no unit is bound to it.

### [`RFC9728-3.3-4`](#rfc9728-3.3-4)

The recipient MUST validate that any signed metadata was signed by a key belonging to the issuer and that the signature is valid (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-3.3-4, so no unit is bound to it.

### [`RFC9728-6-1`](#rfc9728-6-1)

Unicode Normalization MUST NOT be applied at any point to either the JSON string or the string it is to be compared against (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-6-1, so no unit is bound to it.

### [`RFC9728-6-2`](#rfc9728-6-2)

Comparisons between two strings MUST be performed as a Unicode code-point-to-code-point equality comparison (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-6-2, so no unit is bound to it.

### [`RFC9728-7.1-1`](#rfc9728-7.1-1)

Implementations MUST support TLS (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-7.1-1, so no unit is bound to it.

### [`RFC9728-7.1-2`](#rfc9728-7.1-2)

Implementations MUST follow the guidance in BCP 195 (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-7.1-2, so no unit is bound to it.

### [`RFC9728-7.3-1`](#rfc9728-7.3-1)

TLS certificate checking MUST be performed by the client as described in RFC 9525 when making a protected resource metadata request (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-7.3-1, so no unit is bound to it.

### [`RFC9728-7.3-2`](#rfc9728-7.3-2)

The client MUST ensure that the resource identifier URL it is using as the prefix for the metadata request exactly matches the value of the `resource` metadata parameter in the protected resource metadata document received (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-7.3-2, so no unit is bound to it.

### [`RFC9728-2-2`](#rfc9728-2-2)

The `authorization_servers` field MAY be included as a JSON array of AS issuer URLs (§2; the RFC marks the parameter OPTIONAL: "authorization_servers OPTIONAL. JSON array containing a list of OAuth authorization server issuer identifiers")

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestNewStreamable_OAuth_MetadataEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L294) | unit/verify | unproven |
| positive | [`TestResourceMetadata_Document`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_test.go#L216) | unit/verify | unproven |

### [`RFC9728-3.2-5`](#rfc9728-3.2-5)

Metadata parameters that are not understood MUST be ignored by the consumer of the metadata response (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9728-3.2-5, so no unit is bound to it.

### [`RFC9728-5.1-1`](#rfc9728-5.1-1)

The `resource_metadata` parameter of the WWW-Authenticate response header field carries the URL of the protected resource metadata, and MAY also be used in WWW-Authenticate responses using authorization schemes other than "Bearer" (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestNewStreamable_OAuth_RejectsMissingBearer`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L216) | unit/verify | unproven |
| positive | [`TestOAuth_Authenticate_MissingHeader`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_test.go#L48) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
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
| `2` | not stated | 4 | walked | not stated |
| `2.1` | not stated | 1 | walked | not stated |
| `2.2` | not stated | 2 | walked | not stated |
| `3` | not stated | 3 | walked | not stated |
| `3.1` | not stated | 2 | walked | not stated |
| `3.2` | not stated | 3 | walked | not stated |
| `3.3` | not stated | 5 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
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
| `7.10` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
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
| `3.3:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.3 states the same consequence twice, once for each identity check; the first is mapped at site 3.3:2. | If these values are not identical, the data contained in the response MUST NOT be used. |
| `6:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Chapeau introducing the two comparison rules that follow it, mapped at sites 6:2 and 6:3: 'comparisons between JSON strings and other Unicode strings MUST be performed as specified below:'. | Therefore, comparisons between JSON strings and other Unicode strings MUST be performed as specified below: |

## Superseded

No document obsoletes RFC 9728, so its obligations are stated where they were written.
