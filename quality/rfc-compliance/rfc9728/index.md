# RFC 9728 - OAuth 2.0 Protected Resource Metadata

OAuth 2.0 Protected Resource Metadata (MCP resource server). Every requirement listed for RFC 9728, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 12 | of 26 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 26 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 26 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 26 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 29 | 29 of 37 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 6 | of 26 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Audit verdicts | 11 | of 26 mandatory requirements reviewed | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 26 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 7 | of 26 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 26 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 26 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 26 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 26 | of 32 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 26 mandatory requirements: they add up to 26. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | OAuth 2.0 Protected Resource Metadata (MCP resource server) |
| Automatic checks | Included |
| Requirements | 32 |
| Mandatory requirements (MUST level) | 26 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 6 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 37 |
| Tests linked to requirements | 37 |
| Recorded audit verdicts | 11 |
| Fault test records | 29 |
| Summary | `rfc/short/rfc9728.md` |
| Requirement table | `rfc/requirements/rfc9728.md` |
| RFC text | `rfc/full/rfc9728.txt` |

## Automatic checks

Included in automatic checks: Ze publishes protected-resource metadata for its MCP resource server. Thomas selected "Keep present OAuth roles" on 2026-09-21: MCP resource server and external-AS metadata consumer, with no authorization server, protected-resource discovery client, response signing or signed metadata.

## Published support status

**Area:** OAuth 2.0 Protected Resource Metadata (MCP resource server)

**Test verification:** 9 of 26 MUST proven

**Evidence:** unit 9

**Unmet:** RFC9728-2-6, RFC9728-2-7, RFC9728-2-8, RFC9728-2.2-1, RFC9728-2.2-2, RFC9728-3.3-4


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 12 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 14 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **26** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (12):** [`RFC9728-3-2`](#rfc9728-3-2), [`RFC9728-3-3`](#rfc9728-3-3), [`RFC9728-3-4`](#rfc9728-3-4), [`RFC9728-3.1-3`](#rfc9728-3.1-3), [`RFC9728-3.2-3`](#rfc9728-3.2-3), [`RFC9728-3.2-4`](#rfc9728-3.2-4), [`RFC9728-3.3-1`](#rfc9728-3.3-1), [`RFC9728-6-1`](#rfc9728-6-1), [`RFC9728-6-2`](#rfc9728-6-2), [`RFC9728-6-3`](#rfc9728-6-3), [`RFC9728-7.1-1`](#rfc9728-7.1-1), [`RFC9728-7.1-2`](#rfc9728-7.1-2)

**Annotated (including scoped evidence) (14):** [`RFC9728-2-1`](#rfc9728-2-1), [`RFC9728-2-6`](#rfc9728-2-6), [`RFC9728-2-7`](#rfc9728-2-7), [`RFC9728-2-8`](#rfc9728-2-8), [`RFC9728-2.1-1`](#rfc9728-2.1-1), [`RFC9728-2.1-2`](#rfc9728-2.1-2), [`RFC9728-2.2-1`](#rfc9728-2.2-1), [`RFC9728-2.2-2`](#rfc9728-2.2-2), [`RFC9728-3.3-2`](#rfc9728-3.3-2), [`RFC9728-3.3-3`](#rfc9728-3.3-3), [`RFC9728-3.3-4`](#rfc9728-3.3-4), [`RFC9728-7.3-1`](#rfc9728-7.3-1), [`RFC9728-7.3-2`](#rfc9728-7.3-2), [`RFC9728-3.2-5`](#rfc9728-3.2-5)

## Figures

- **MUST bar**: Ze verified 9, Tests awaiting verification 4, Exclusion unchecked 7, Not built 6
- **Overview bar**: Ze MUST verified 9, SHOULD verified 2, Tests awaiting verification 4, Exclusion unchecked 7, Not assessed 4, Not built 6

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 26 |
| MUST rows kept (the MUST bar) | 26 |
| Ze verified | 9 |
| OS verified | 0 |
| Tests awaiting verification | 4 |
| OS testing gap | 0 |
| Exclusion unchecked | 7 |
| Not assessed | 0 |
| Not built | 6 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 6 |
| SHOULD verified | 2 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 4 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc9728.json](../../../data/rfc/rfc9728.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9728-2-1` | resource REQUIRED. The protected resource's resource identifier, as defined in Section 1.2. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestNewStreamable_OAuth_MetadataEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L321). **positive:** `unit/verify` [`TestResourceMetadata_Document`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_oauth_test.go#L212). **negative:** no negative test. **{single-polarity}:** the resource field is emitted unconditionally by writeResourceMetadata (internal/component/mcp/oauth.go:170-171,184-192), so no input can make it absent and there is no negative to assert |
| `RFC9728-2-6` | This URL MUST use the https scheme. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze publishes no jwks_uri: internal/component/mcp/oauth.go::writeResourceMetadata omits the parameter, and no owner decision declines resource signing keys. Publishing it, over https, is owed |
| `RFC9728-2-7` | When both signing and encryption keys are made available, a use (public key use) parameter value is REQUIRED for all keys in the referenced JWK Set to indicate each key's intended usage. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze makes no resource keys available: internal/component/mcp/oauth.go::writeResourceMetadata publishes no jwks_uri, so no JWK Set carries a use parameter; owed after the first release (owner decision 2026-09-21, "Keep present OAuth roles", recorded in the Enrolment reason, which left it out of the current work) |
| `RFC9728-2-8` | resource_signing_alg_values_supported OPTIONAL. JSON array containing a list of the JWS [JWS] signing algorithms (alg values) [JWA] supported by the protected resource for signing resource responses, for instance, as described in [FAPI.MessageSigning]. No default algorithms are implied if this entry is omitted. The value none MUST NOT be used. (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze offers no resource response signing: internal/component/mcp/oauth.go::writeResourceMetadata emits no resource_signing_alg_values_supported; owed after the first release (owner decision 2026-09-21, "Keep present OAuth roles", recorded in the Enrolment reason, which left it out of the current work) |
| `RFC9728-2.1-1` | If any human-readable field is sent without a language tag, parties using it MUST NOT make any assumptions about the language, character set, or script of the string value (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §2.1 addresses "parties using it" when a human-readable field is received. Thomas excluded the protected-resource discovery-client role on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata emits resource/authorization_servers/scopes_supported/bearer_methods_supported only; internal/component/mcp/as_metadata.go::fetchASMetadata consumes AS issuer and jwks_uri, not human-readable resource metadata or a metadata user interface |
| `RFC9728-2.1-2` | If any human-readable field is sent without a language tag, parties using it MUST NOT make any assumptions about the language, character set, or script of the string value, and the string value MUST be used as is wherever it is presented in a user interface. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §2.1 conditions this on "wherever it is presented in a user interface." Thomas excluded the protected-resource discovery-client role on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata is a JSON publisher, not a metadata-consuming UI; internal/component/mcp/as_metadata.go::fetchASMetadata reads AS issuer and jwks_uri without displaying resource names or documentation |
| `RFC9728-2.2-1` | The signed metadata MUST be digitally signed or MACed (protected with a Message Authentication Code) using a JSON Web Signature (JWS) [JWS] and MUST contain an iss (issuer) claim denoting the party attesting to the claims in the signed metadata. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze publishes no signed metadata: internal/component/mcp/oauth.go::writeResourceMetadata emits plain JSON and no signed_metadata parameter; owed after the first release (owner decision 2026-09-21, "Keep present OAuth roles", recorded in the Enrolment reason, which left it out of the current work) |
| `RFC9728-2.2-2` | If the consumer of the metadata supports signed metadata, metadata values conveyed in the signed metadata MUST take precedence over the corresponding values conveyed using plain JSON elements (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze consumes no protected-resource signed metadata: internal/component/mcp/streamable_auth.go::buildAuthForMode consumes the AS issuer and keys only; owed after the first release (owner decision 2026-09-21, "Keep present OAuth roles", recorded in the Enrolment reason, which left it out of the current work) |
| `RFC9728-3-2` | Protected resources supporting metadata MUST make a JSON document containing metadata as specified in Section 2 available at a URL formed by inserting a well-known URI string into the protected resource's resource identifier between the host component and the path and/or query components, if any. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC9728WellKnownInsertedBeforeQuery`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L325). **positive:** `unit/verify` [`TestRFC9728WellKnownInsertedBetweenHostAndPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L87). **negative:** `unit/verify` [`TestRFC9728WellKnownInsertedBeforeQuery`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L326). **negative:** `unit/verify` [`TestRFC9728WellKnownInsertedBetweenHostAndPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L88) |
| `RFC9728-3-3` | The well-known URI path suffix used MUST be registered in the "Well-Known URIs" registry (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC9728RegisteredSuffix`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L138). **negative:** `unit/verify` [`TestRFC9728RegisteredSuffix`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L139) |
| `RFC9728-3-4` | An OAuth 2.0 application using this specification MUST specify what well-known URI suffix it will use for this purpose (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC9728RegisteredSuffix`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L140). **negative:** `unit/verify` [`TestRFC9728RegisteredSuffix`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L141) |
| `RFC9728-3.1-3` | If the resource identifier value contains a path or query component, any terminating slash (/) following the host component MUST be removed before inserting /.well-known/ and the well-known URI path suffix between the host component and the path and/or query components. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC9728TerminatingSlashRemovedBeforeInsertion`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L111). **negative:** `unit/verify` [`TestRFC9728TerminatingSlashRemovedBeforeInsertion`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L112) |
| `RFC9728-3.2-3` | A successful response MUST use the 200 OK HTTP status code and return a JSON object using the application/json content type that contains a set of metadata parameters as its members that are a subset of the metadata parameters defined in Section 2. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC9728ResponseShape`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L172). **negative:** `unit/verify` [`TestRFC9728ResponseShape`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L173) |
| `RFC9728-3.2-4` | Parameters with zero values MUST be omitted from the response (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC9728ZeroValuedParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L194). **negative:** `unit/verify` [`TestRFC9728ZeroValuedParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L195) |
| `RFC9728-3.3-1` | The resource value returned MUST be identical to the protected resource's resource identifier value into which the well-known URI path suffix was inserted to create the URL used to retrieve the metadata. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC9728ResourceMatchesInsertedIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L210). **negative:** `unit/verify` [`TestRFC9728ResourceMatchesInsertedIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L211) |
| `RFC9728-3.3-2` | The resource value returned MUST be identical to the protected resource's resource identifier value into which the well-known URI path suffix was inserted to create the URL used to retrieve the metadata. If these values are not identical, the data contained in the response MUST NOT be used. If the protected resource metadata was retrieved from a URL returned by the protected resource via the WWW-Authenticate resource_metadata parameter, then the resource value returned MUST be identical to the URL that the client used to make the request to the resource server. If these values are not identical, the data contained in the response MUST NOT be used. (§3.3) | MUST NOT | 3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §3.3 forbids using a mismatched metadata response, a consumer operation; §7.3 makes the subject explicit: "the client MUST ensure" the resource identifiers match. Thomas excluded protected-resource discovery clients on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata publishes the configured identity; internal/component/mcp/streamable_auth.go::buildAuthForMode fetches AS metadata only and consumes no resource metadata response |
| `RFC9728-3.3-3` | If the protected resource metadata was retrieved from a URL returned by the protected resource via the WWW-Authenticate resource_metadata parameter, then the resource value returned MUST be identical to the URL that the client used to make the request to the resource server. (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §3.3 conditions this consumer check on metadata "retrieved from a URL returned by the protected resource via the WWW-Authenticate resource_metadata parameter". Thomas excluded protected-resource discovery clients on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata and internal/component/mcp/streamable.go::ServeHTTP produce the document/challenge, but never follow another resource's challenge or consume its metadata |
| `RFC9728-3.3-4` | The recipient MUST validate that any signed metadata was signed by a key belonging to the issuer and that the signature is valid (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze validates no signed metadata: internal/component/mcp/streamable_auth.go::buildAuthForMode builds a local access-token verifier, and ze has no protected-resource metadata recipient; owed after the first release (owner decision 2026-09-21, "Keep present OAuth roles", recorded in the Enrolment reason, which left it out of the current work) |
| `RFC9728-6-1` | Unicode Normalization [USA15] MUST NOT be applied at any point to either the JSON string or the string it is to be compared against. (§6) | MUST NOT | 6 | **positive:** `unit/verify` [`TestRFC9728ResourceIdentityPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L233). **negative:** `unit/verify` [`TestRFC9728ResourceIdentityPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L234) |
| `RFC9728-6-2` | Comparisons between the two strings MUST be performed as a Unicode code-point-to-code-point equality comparison. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC9728ResourceIdentityPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L235). **negative:** `unit/verify` [`TestRFC9728ResourceIdentityPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L236) |
| `RFC9728-6-3` | Therefore, comparisons between JSON strings and other Unicode strings MUST be performed as specified below: 1. Remove any JSON-applied escaping to produce an array of Unicode code points. (§6, step 1) | MUST | 6 | **positive:** `unit/verify` [`TestRFC9728EscapedAudienceMatchesResource`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_jwt_test.go#L48). **negative:** `unit/verify` [`TestRFC9728EscapeTextIsNotUnescapedTwice`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_jwt_test.go#L69) |
| `RFC9728-7.1-1` | Implementations MUST support TLS (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC9728MCPServingTLSVersions`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_test.go#L124). **negative:** `unit/verify` [`TestRFC9728MCPListenerSupportsTLS`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_test.go#L74) |
| `RFC9728-7.1-2` | They MUST follow the guidance in [BCP195], which provides recommendations and requirements for improving the security of deployed services that use TLS. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC9728MCPListenerNegotiatesOnlyBCP195Suites`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_tls_suites_test.go#L28). **positive:** `unit/verify` [`TestRFC9728MCPServingTLSVersions`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_test.go#L125). **negative:** `unit/verify` [`TestRFC9728MCPListenerNegotiatesOnlyBCP195Suites`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_tls_suites_test.go#L29). **negative:** `unit/verify` [`TestRFC9728MCPServingTLSVersions`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_test.go#L126) |
| `RFC9728-7.3-1` | TLS certificate checking MUST be performed by the client as described in [RFC9525] when making a protected resource metadata request. (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §7.3 specifies "by the client" and "when making a protected resource metadata request." Thomas excluded that client role on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata is the publisher; internal/component/mcp/as_metadata.go::fetchASMetadata requests AS metadata only, whose certificate checks remain required by RFC8414-6.1-3 |
| `RFC9728-7.3-2` | To prevent this, the client MUST ensure that the resource identifier URL it is using as the prefix for the metadata request exactly matches the value of the resource metadata parameter in the protected resource metadata document received by the client, as described in Section 3.3. (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §7.3 explicitly addresses "the client" and its protected-resource "metadata document received". Thomas excluded that client role on 2026-09-21. internal/component/mcp/streamable_auth.go::buildAuthForMode requests AS metadata only; internal/component/mcp/oauth.go::writeResourceMetadata publishes the local resource identity rather than accepting a remote resource identity |
| `RFC9728-2-2` | authorization_servers OPTIONAL. JSON array containing a list of OAuth authorization server issuer identifiers, as defined in [RFC8414], for authorization servers that can be used with this protected resource. (§2; the RFC marks the parameter OPTIONAL: "authorization_servers OPTIONAL. JSON array containing a list of OAuth authorization server issuer identifiers") | MAY | 2 | **positive:** `unit/verify` [`TestNewStreamable_OAuth_MetadataEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L325). **positive:** `unit/verify` [`TestRFC9728AuthorizationServersAreIssuerIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L376). **positive:** `unit/verify` [`TestResourceMetadata_Document`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_oauth_test.go#L216). **negative:** `unit/verify` [`TestRFC9728AuthorizationServersAreIssuerIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L377) |
| `RFC9728-3.2-5` | Additional metadata parameters MAY be defined and used; any metadata parameters that are not understood MUST be ignored. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** §3.2 says "any metadata parameters that are not understood MUST be ignored", an operation on the received response. Thomas excluded protected-resource discovery clients on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata publishes defined fields; internal/component/mcp/as_metadata.go::fetchASMetadata consumes a different document, AS metadata, and no producer decodes a protected-resource metadata response |
| `RFC9728-5.1-1` | This specification introduces a new parameter in the WWW-Authenticate HTTP response header field to indicate the protected resource metadata URL: resource_metadata: The URL of the protected resource metadata. The response below is an example of a WWW-Authenticate header that includes the resource identifier. HTTP/1.1 401 Unauthorized WWW-Authenticate: Bearer resource_metadata= "https://resource.example.com/.well-known/oauth-protected-resource" The HTTP status code in the example response above is defined by [RFC6750]. This parameter MAY also be used in WWW-Authenticate responses using authorization schemes other than "Bearer" [RFC6750], such as the DPoP scheme defined by [RFC9449]. (§5.1) | MAY | 5.1 | **positive:** `unit/verify` [`TestNewStreamable_OAuth_RejectsMissingBearer`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L247). **positive:** `unit/verify` [`TestOAuth_Authenticate_MissingHeader`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_oauth_test.go#L48). **positive:** `unit/verify` [`TestRFC9728ChallengeNamesMetadataURL`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L415). **negative:** no negative test. **{single-polarity}:** the row is a permission, "This parameter MAY also be used in WWW-Authenticate responses", so no challenge Ze could send violates it and there is no refusal to assert; the positive proves the parameter carries the whole metadata URL of the configured resource |
| `RFC9728-7.10-1` | Implementations should utilize HTTP caching directives such as Cache-Control with max-age, as defined in [RFC9111], to enable caching of retrieved metadata for appropriate time periods. (§7.10; lowercase "should" in the RFC, which names no value) | SHOULD | 7.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-2-4` | scopes_supported RECOMMENDED. JSON array containing a list of scope values, as defined in OAuth 2.0 [RFC6749], that are used in authorization requests to request access to this protected resource. (§2) | RECOMMENDED | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-2-5` | bearer_methods_supported OPTIONAL. JSON array containing a list of the supported methods of sending an OAuth 2.0 bearer token [RFC6750] to the protected resource. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9728-7.10-2` | Normal HTTP caching behaviors apply, meaning that the GET request may retrieve a cached copy of the content, rather than the latest copy. (§7.10) | MAY | 7.10 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9728-2-6`](#rfc9728-2-6) This URL MUST use the https scheme. (§2) | {gap}, no test | ze publishes no jwks_uri: internal/component/mcp/oauth.go::writeResourceMetadata omits the parameter, and no owner decision declines resource signing keys. Publishing it, over https, is owed |
| [`RFC9728-2-7`](#rfc9728-2-7) When both signing and encryption keys are made available, a use (public key use) parameter value is REQUIRED for all keys in the referenced JWK Set to indicate each key's intended usage. (§2) | {gap}, no test | ze makes no resource keys available: internal/component/mcp/oauth.go::writeResourceMetadata publishes no jwks_uri, so no JWK Set carries a use parameter; owed after the first release (owner decision 2026-09-21, "Keep present OAuth roles", recorded in the Enrolment reason, which left it out of the current work) |
| [`RFC9728-2-8`](#rfc9728-2-8) resource_signing_alg_values_supported OPTIONAL. JSON array containing a list of the JWS [JWS] signing algorithms (alg values) [JWA] supported by the protected resource for signing resource responses, for instance, as described in [FAPI.MessageSigning]. No default algorithms are implied if this entry is omitted. The value none MUST NOT be used. (§2) | {gap}, no test | ze offers no resource response signing: internal/component/mcp/oauth.go::writeResourceMetadata emits no resource_signing_alg_values_supported; owed after the first release (owner decision 2026-09-21, "Keep present OAuth roles", recorded in the Enrolment reason, which left it out of the current work) |
| [`RFC9728-2.1-1`](#rfc9728-2.1-1) If any human-readable field is sent without a language tag, parties using it MUST NOT make any assumptions about the language, character set, or script of the string value (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: §2.1 addresses "parties using it" when a human-readable field is received. Thomas excluded the protected-resource discovery-client role on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata emits resource/authorization_servers/scopes_supported/bearer_methods_supported only; internal/component/mcp/as_metadata.go::fetchASMetadata consumes AS issuer and jwks_uri, not human-readable resource metadata or a metadata user interface |
| [`RFC9728-2.1-2`](#rfc9728-2.1-2) If any human-readable field is sent without a language tag, parties using it MUST NOT make any assumptions about the language, character set, or script of the string value, and the string value MUST be used as is wherever it is presented in a user interface. (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: §2.1 conditions this on "wherever it is presented in a user interface." Thomas excluded the protected-resource discovery-client role on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata is a JSON publisher, not a metadata-consuming UI; internal/component/mcp/as_metadata.go::fetchASMetadata reads AS issuer and jwks_uri without displaying resource names or documentation |
| [`RFC9728-2.2-1`](#rfc9728-2.2-1) The signed metadata MUST be digitally signed or MACed (protected with a Message Authentication Code) using a JSON Web Signature (JWS) [JWS] and MUST contain an iss (issuer) claim denoting the party attesting to the claims in the signed metadata. (§2.2) | {gap}, no test | ze publishes no signed metadata: internal/component/mcp/oauth.go::writeResourceMetadata emits plain JSON and no signed_metadata parameter; owed after the first release (owner decision 2026-09-21, "Keep present OAuth roles", recorded in the Enrolment reason, which left it out of the current work) |
| [`RFC9728-2.2-2`](#rfc9728-2.2-2) If the consumer of the metadata supports signed metadata, metadata values conveyed in the signed metadata MUST take precedence over the corresponding values conveyed using plain JSON elements (§2.2) | {gap}, no test | ze consumes no protected-resource signed metadata: internal/component/mcp/streamable_auth.go::buildAuthForMode consumes the AS issuer and keys only; owed after the first release (owner decision 2026-09-21, "Keep present OAuth roles", recorded in the Enrolment reason, which left it out of the current work) |
| [`RFC9728-3.3-2`](#rfc9728-3.3-2) The resource value returned MUST be identical to the protected resource's resource identifier value into which the well-known URI path suffix was inserted to create the URL used to retrieve the metadata. If these values are not identical, the data contained in the response MUST NOT be used. If the protected resource metadata was retrieved from a URL returned by the protected resource via the WWW-Authenticate resource_metadata parameter, then the resource value returned MUST be identical to the URL that the client used to make the request to the resource server. If these values are not identical, the data contained in the response MUST NOT be used. (§3.3) | no test | no test carries this requirement id; annotated {not-applicable}: §3.3 forbids using a mismatched metadata response, a consumer operation; §7.3 makes the subject explicit: "the client MUST ensure" the resource identifiers match. Thomas excluded protected-resource discovery clients on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata publishes the configured identity; internal/component/mcp/streamable_auth.go::buildAuthForMode fetches AS metadata only and consumes no resource metadata response |
| [`RFC9728-3.3-3`](#rfc9728-3.3-3) If the protected resource metadata was retrieved from a URL returned by the protected resource via the WWW-Authenticate resource_metadata parameter, then the resource value returned MUST be identical to the URL that the client used to make the request to the resource server. (§3.3) | no test | no test carries this requirement id; annotated {not-applicable}: §3.3 conditions this consumer check on metadata "retrieved from a URL returned by the protected resource via the WWW-Authenticate resource_metadata parameter". Thomas excluded protected-resource discovery clients on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata and internal/component/mcp/streamable.go::ServeHTTP produce the document/challenge, but never follow another resource's challenge or consume its metadata |
| [`RFC9728-3.3-4`](#rfc9728-3.3-4) The recipient MUST validate that any signed metadata was signed by a key belonging to the issuer and that the signature is valid (§3.3) | {gap}, no test | ze validates no signed metadata: internal/component/mcp/streamable_auth.go::buildAuthForMode builds a local access-token verifier, and ze has no protected-resource metadata recipient; owed after the first release (owner decision 2026-09-21, "Keep present OAuth roles", recorded in the Enrolment reason, which left it out of the current work) |
| [`RFC9728-7.3-1`](#rfc9728-7.3-1) TLS certificate checking MUST be performed by the client as described in [RFC9525] when making a protected resource metadata request. (§7.3) | no test | no test carries this requirement id; annotated {not-applicable}: §7.3 specifies "by the client" and "when making a protected resource metadata request." Thomas excluded that client role on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata is the publisher; internal/component/mcp/as_metadata.go::fetchASMetadata requests AS metadata only, whose certificate checks remain required by RFC8414-6.1-3 |
| [`RFC9728-7.3-2`](#rfc9728-7.3-2) To prevent this, the client MUST ensure that the resource identifier URL it is using as the prefix for the metadata request exactly matches the value of the resource metadata parameter in the protected resource metadata document received by the client, as described in Section 3.3. (§7.3) | no test | no test carries this requirement id; annotated {not-applicable}: §7.3 explicitly addresses "the client" and its protected-resource "metadata document received". Thomas excluded that client role on 2026-09-21. internal/component/mcp/streamable_auth.go::buildAuthForMode requests AS metadata only; internal/component/mcp/oauth.go::writeResourceMetadata publishes the local resource identity rather than accepting a remote resource identity |
| [`RFC9728-3.2-5`](#rfc9728-3.2-5) Additional metadata parameters MAY be defined and used; any metadata parameters that are not understood MUST be ignored. (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: §3.2 says "any metadata parameters that are not understood MUST be ignored", an operation on the received response. Thomas excluded protected-resource discovery clients on 2026-09-21. internal/component/mcp/oauth.go::writeResourceMetadata publishes defined fields; internal/component/mcp/as_metadata.go::fetchASMetadata consumes a different document, AS metadata, and no producer decodes a protected-resource metadata response |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC9728-2-1`](#rfc9728-2-1)

resource REQUIRED. The protected resource's resource identifier, as defined in Section 1.2. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a metadata document without resource, or with a value other than the resource identifier. Red: oauth_test.go writeResourceMetadata test fails on got["resource"] != "https://mcp.example/"; oauth_e2e_test.go checks the served document carries resource. Single-polarity marker on the row: the field is emitted unconditionally.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestNewStreamable_OAuth_MetadataEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L321) | unit/verify | not verified |
| positive | [`TestResourceMetadata_Document`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_oauth_test.go#L212) | unit/verify | not verified |

### [`RFC9728-2-6`](#rfc9728-2-6)

This URL MUST use the https scheme. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9728-2-6, so there is no linked test.

### [`RFC9728-2-7`](#rfc9728-2-7)

When both signing and encryption keys are made available, a use (public key use) parameter value is REQUIRED for all keys in the referenced JWK Set to indicate each key's intended usage. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9728-2-7, so there is no linked test.

### [`RFC9728-2-8`](#rfc9728-2-8)

resource_signing_alg_values_supported OPTIONAL. JSON array containing a list of the JWS [JWS] signing algorithms (alg values) [JWA] supported by the protected resource for signing resource responses, for instance, as described in [FAPI.MessageSigning]. No default algorithms are implied if this entry is omitted. The value none MUST NOT be used. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9728-2-8, so there is no linked test.

### [`RFC9728-2.1-1`](#rfc9728-2.1-1)

If any human-readable field is sent without a language tag, parties using it MUST NOT make any assumptions about the language, character set, or script of the string value (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9728-2.1-1, so there is no linked test.

### [`RFC9728-2.1-2`](#rfc9728-2.1-2)

If any human-readable field is sent without a language tag, parties using it MUST NOT make any assumptions about the language, character set, or script of the string value, and the string value MUST be used as is wherever it is presented in a user interface. (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9728-2.1-2, so there is no linked test.

### [`RFC9728-2.2-1`](#rfc9728-2.2-1)

The signed metadata MUST be digitally signed or MACed (protected with a Message Authentication Code) using a JSON Web Signature (JWS) [JWS] and MUST contain an iss (issuer) claim denoting the party attesting to the claims in the signed metadata. (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9728-2.2-1, so there is no linked test.

### [`RFC9728-2.2-2`](#rfc9728-2.2-2)

If the consumer of the metadata supports signed metadata, metadata values conveyed in the signed metadata MUST take precedence over the corresponding values conveyed using plain JSON elements (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9728-2.2-2, so there is no linked test.

### [`RFC9728-3-2`](#rfc9728-3-2)

Protected resources supporting metadata MUST make a JSON document containing metadata as specified in Section 2 available at a URL formed by inserting a well-known URI string into the protected resource's resource identifier between the host component and the path and/or query components, if any. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after d8e084c3eb). Section 3: "Protected resources supporting metadata MUST make a JSON document containing metadata as specified in Section 2 available at a URL formed by inserting a well-known URI string into the protected resource's resource identifier between the host component and the path and/or query components, if any." Path clause: TestRFC9728WellKnownInsertedBetweenHostAndPath serves the document through ServeHTTP at /.well-known/oauth-protected-resource/mcp with resource https://mcp.example/mcp, advertises the same URL, and answers 404 at the suffix after the path and at the bare suffix. Query clause: TestRFC9728WellKnownInsertedBeforeQuery covers /mcp?tenant=a and the query-only identifier, and refuses the dropped query, suffix after path, bare suffix and suffix after query. The weak finding rested on the producer defect for https://mcp.example/mcp/, fixed in resourceOriginAndPath; that input is pinned at resourceMetadataURL and resourceMetadataPath by the RFC9728-3.1-3 units, not by this row's own units.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9728WellKnownInsertedBeforeQuery`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L326) | unit/verify | revert, verified |
| negative | [`TestRFC9728WellKnownInsertedBetweenHostAndPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L88) | unit/verify | revert, verified |
| positive | [`TestRFC9728WellKnownInsertedBeforeQuery`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L325) | unit/verify | revert, verified |
| positive | [`TestRFC9728WellKnownInsertedBetweenHostAndPath`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L87) | unit/verify | revert, verified |

### [`RFC9728-3-3`](#rfc9728-3-3)

The well-known URI path suffix used MUST be registered in the "Well-Known URIs" registry (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9728RegisteredSuffix`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L139) | unit/verify | revert, verified |
| positive | [`TestRFC9728RegisteredSuffix`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L138) | unit/verify | revert, verified |

### [`RFC9728-3-4`](#rfc9728-3-4)

An OAuth 2.0 application using this specification MUST specify what well-known URI suffix it will use for this purpose (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9728RegisteredSuffix`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L141) | unit/verify | revert, verified |
| positive | [`TestRFC9728RegisteredSuffix`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L140) | unit/verify | revert, verified |

### [`RFC9728-3.1-3`](#rfc9728-3.1-3)

If the resource identifier value contains a path or query component, any terminating slash (/) following the host component MUST be removed before inserting /.well-known/ and the well-known URI path suffix between the host component and the path and/or query components. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after d8e084c3eb). Section 3.1: "If the resource identifier value contains a path or query component, any terminating slash (/) following the host component MUST be removed before inserting /.well-known/ and the well-known URI path suffix between the host component and the path and/or query components." The earlier wrong finding is fixed at the producer: resourceOriginAndPath (streamable_auth.go) no longer TrimRights the path; it drops only a path of "/" standing alone after the host, the slash that both follows the host and terminates the identifier's path. Read that way, RFC 8414 Section 3.1's broader "any terminating /" is narrowed by RFC 9728's "following the host component"; the reading was approved by the owner on 2026-10-18 and agrees with the Section 3.1 example. TestRFC9728TerminatingSlashRemovedBeforeInsertion: positive, https://mcp.example/ maps to .../oauth-protected-resource and https://mcp.example/?x=1 to .../oauth-protected-resource?x=1; negative, https://mcp.example/mcp/ keeps its own slash at .../oauth-protected-resource/mcp/. Each case also asserts resourceMetadataPath, the value streamable.go stores as metadataPath and ServeHTTP matches against r.URL.RequestURI(), and that no URL carries //.well-known. Restoring TrimRight reddens the /mcp/ case; losing the lone-slash removal reddens both positives.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9728TerminatingSlashRemovedBeforeInsertion`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L112) | unit/verify | revert, verified |
| positive | [`TestRFC9728TerminatingSlashRemovedBeforeInsertion`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L111) | unit/verify | revert, verified |

### [`RFC9728-3.2-3`](#rfc9728-3.2-3)

A successful response MUST use the 200 OK HTTP status code and return a JSON object using the application/json content type that contains a set of metadata parameters as its members that are a subset of the metadata parameters defined in Section 2. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a success status other than 200, a content type other than application/json, a body that is not a JSON object, a member outside Section 2. Red: TestRFC9728ResponseShape fails on w.Code != 200, on Content-Type != application/json, on json.Unmarshal into a map (arrays/scalars refused), and on any member absent from rfc9728Section2Members.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9728ResponseShape`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L173) | unit/verify | revert, verified |
| positive | [`TestRFC9728ResponseShape`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L172) | unit/verify | revert, verified |

### [`RFC9728-3.2-4`](#rfc9728-3.2-4)

Parameters with zero values MUST be omitted from the response (§3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9728ZeroValuedParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L195) | unit/verify | revert, verified |
| positive | [`TestRFC9728ZeroValuedParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L194) | unit/verify | revert, verified |

### [`RFC9728-3.3-1`](#rfc9728-3.3-1)

The resource value returned MUST be identical to the protected resource's resource identifier value into which the well-known URI path suffix was inserted to create the URL used to retrieve the metadata. (§3.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestRFC9728ResourceMatchesInsertedIdentifier proves one identifier: resource https://mcp.example/api at .../oauth-protected-resource/api, and that the audience override is not returned. It does not cover an identifier with a trailing path slash: for https://mcp.example/mcp/ Ze forms the URL from /mcp (TrimRight in resourceOriginAndPath) and returns resource https://mcp.example/mcp/, so the returned value differs from the identifier the URL was formed from, and no tagged assertion goes red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9728ResourceMatchesInsertedIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L211) | unit/verify | revert, verified |
| positive | [`TestRFC9728ResourceMatchesInsertedIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L210) | unit/verify | revert, verified |

### [`RFC9728-3.3-2`](#rfc9728-3.3-2)

The resource value returned MUST be identical to the protected resource's resource identifier value into which the well-known URI path suffix was inserted to create the URL used to retrieve the metadata. If these values are not identical, the data contained in the response MUST NOT be used. If the protected resource metadata was retrieved from a URL returned by the protected resource via the WWW-Authenticate resource_metadata parameter, then the resource value returned MUST be identical to the URL that the client used to make the request to the resource server. If these values are not identical, the data contained in the response MUST NOT be used. (§3.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9728-3.3-2, so there is no linked test.

### [`RFC9728-3.3-3`](#rfc9728-3.3-3)

If the protected resource metadata was retrieved from a URL returned by the protected resource via the WWW-Authenticate resource_metadata parameter, then the resource value returned MUST be identical to the URL that the client used to make the request to the resource server. (§3.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9728-3.3-3, so there is no linked test.

### [`RFC9728-3.3-4`](#rfc9728-3.3-4)

The recipient MUST validate that any signed metadata was signed by a key belonging to the issuer and that the signature is valid (§3.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9728-3.3-4, so there is no linked test.

### [`RFC9728-6-1`](#rfc9728-6-1)

Unicode Normalization [USA15] MUST NOT be applied at any point to either the JSON string or the string it is to be compared against. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: normalizing either string. Red: TestRFC9728ResourceIdentityPreserved fails if the published resource is not the exact decomposed e+U+0301 identifier (returned != resource), and fails if the composed spelling %C3%A9 retrieves the document (status != 404).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9728ResourceIdentityPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L234) | unit/verify | not verified |
| positive | [`TestRFC9728ResourceIdentityPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L233) | unit/verify | not verified |

### [`RFC9728-6-2`](#rfc9728-6-2)

Comparisons between the two strings MUST be performed as a Unicode code-point-to-code-point equality comparison. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: equality other than code-point to code-point. Red: TestRFC9728ResourceIdentityPreserved serves the exact escaped path+query (decodeMetadata requires 200) and requires 404 for the composed spelling, a different query value and a missing query.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9728ResourceIdentityPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L236) | unit/verify | not verified |
| positive | [`TestRFC9728ResourceIdentityPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L235) | unit/verify | not verified |

### [`RFC9728-6-3`](#rfc9728-6-3)

Therefore, comparisons between JSON strings and other Unicode strings MUST be performed as specified below: 1. Remove any JSON-applied escaping to produce an array of Unicode code points. (§6, step 1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: comparing without removing JSON escaping, or removing it more than once. Red: TestRFC9728EscapedAudienceMatchesResource fails if verifyJWT refuses an aud with \u0026 against the & identifier; TestRFC9728EscapeTextIsNotUnescapedTwice fails unless a literal \u0026 aud is refused with errJWTAudienceMismatch.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9728EscapeTextIsNotUnescapedTwice`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_jwt_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestRFC9728EscapedAudienceMatchesResource`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_jwt_test.go#L48) | unit/verify | revert, verified |

### [`RFC9728-7.1-1`](#rfc9728-7.1-1)

Implementations MUST support TLS (§7.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9728MCPListenerSupportsTLS`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_test.go#L74) | unit/verify | not verified |
| positive | [`TestRFC9728MCPServingTLSVersions`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_test.go#L124) | unit/verify | not verified |

### [`RFC9728-7.1-2`](#rfc9728-7.1-2)

They MUST follow the guidance in [BCP195], which provides recommendations and requirements for improving the security of deployed services that use TLS. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. Beyond the HEAD version tags: TestRFC9728MCPListenerNegotiatesOnlyBCP195Suites (ze_mcp) probes the listener startMCPServer opens with loadMCPTLSConfig, ECDSA and RSA certs, with the same tlsprobe assertions as RFC7858-8-1: recommended ECDHE AES-128-GCM selected with null compression and renegotiation_info, every forbidden suite alone never selected, DEFLATE alone refused. Records: loadMCPTLSConfig revert both polarities observed red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9728MCPServingTLSVersions`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_test.go#L126) | unit/verify | revert, verified |
| negative | [`TestRFC9728MCPListenerNegotiatesOnlyBCP195Suites`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_tls_suites_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestRFC9728MCPServingTLSVersions`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_test.go#L125) | unit/verify | revert, verified |
| positive | [`TestRFC9728MCPListenerNegotiatesOnlyBCP195Suites`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc9728_service_mcp_tls_suites_test.go#L28) | unit/verify | revert, verified |

### [`RFC9728-7.3-1`](#rfc9728-7.3-1)

TLS certificate checking MUST be performed by the client as described in [RFC9525] when making a protected resource metadata request. (§7.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9728-7.3-1, so there is no linked test.

### [`RFC9728-7.3-2`](#rfc9728-7.3-2)

To prevent this, the client MUST ensure that the resource identifier URL it is using as the prefix for the metadata request exactly matches the value of the resource metadata parameter in the protected resource metadata document received by the client, as described in Section 3.3. (§7.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9728-7.3-2, so there is no linked test.

### [`RFC9728-2-2`](#rfc9728-2-2)

authorization_servers OPTIONAL. JSON array containing a list of OAuth authorization server issuer identifiers, as defined in [RFC8414], for authorization servers that can be used with this protected resource. (§2; the RFC marks the parameter OPTIONAL: "authorization_servers OPTIONAL. JSON array containing a list of OAuth authorization server issuer identifiers")

Audit verdict: enforced (the tests do what the requirement demands), fresh. [MAY] parameter definition. TestRFC9728AuthorizationServersAreIssuerIdentifiers + authorization_servers decodes as a JSON array of strings holding exactly the validated issuer, https with no query and no fragment; - http, ?tenant=a and #frag authorization-server values are refused by NewStreamable so no document lists them. The refusals are overdetermined: asMetadataURL/oauthHTTPSURL refuse first, and the fetch's issuer match or transport would also refuse, so the negative proves the published property rather than which guard holds it. The oauth_test.go and oauth_e2e_test.go positives keep their tags.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9728AuthorizationServersAreIssuerIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L377) | unit/verify | revert, verified |
| positive | [`TestNewStreamable_OAuth_MetadataEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L325) | unit/verify | revert, verified |
| positive | [`TestRFC9728AuthorizationServersAreIssuerIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L376) | unit/verify | revert, verified |
| positive | [`TestResourceMetadata_Document`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_oauth_test.go#L216) | unit/verify | revert, verified |

### [`RFC9728-3.2-5`](#rfc9728-3.2-5)

Additional metadata parameters MAY be defined and used; any metadata parameters that are not understood MUST be ignored. (§3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9728-3.2-5, so there is no linked test.

### [`RFC9728-5.1-1`](#rfc9728-5.1-1)

This specification introduces a new parameter in the WWW-Authenticate HTTP response header field to indicate the protected resource metadata URL: resource_metadata: The URL of the protected resource metadata. The response below is an example of a WWW-Authenticate header that includes the resource identifier. HTTP/1.1 401 Unauthorized WWW-Authenticate: Bearer resource_metadata= "https://resource.example.com/.well-known/oauth-protected-resource" The HTTP status code in the example response above is defined by [RFC6750]. This parameter MAY also be used in WWW-Authenticate responses using authorization schemes other than "Bearer" [RFC6750], such as the DPoP scheme defined by [RFC9449]. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. [MAY] permission row with {single-polarity: positive}, which holds: no challenge Ze sends can violate a permission. TestRFC9728ChallengeNamesMetadataURL asserts the 401 WWW-Authenticate is Bearer with resource_metadata EXACTLY https://mcp.example/.well-known/oauth-protected-resource/api for a metadata resource that differs from the audience, and a GET there returns that resource's document, so a prefix, the audience-derived URL, or a missing parameter goes red. The oauth_test.go and oauth_e2e_test.go prefix-only positives keep their tags.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestNewStreamable_OAuth_RejectsMissingBearer`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/oauth_e2e_test.go#L247) | unit/verify | revert, verified |
| positive | [`TestRFC9728ChallengeNamesMetadataURL`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_metadata_test.go#L415) | unit/verify | revert, verified |
| positive | [`TestOAuth_Authenticate_MissingHeader`](https://github.com/ze-software/ze/blob/main/internal/component/mcp/rfc9728_oauth_test.go#L48) | unit/verify | revert, verified |

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `1.2` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 4 | reviewed | Section 2 marks authorization_servers and bearer_methods_supported OPTIONAL, and scopes_supported RECOMMENDED. The retained advisory IDs have no separate MUST-level inventory sites. |
| `2.1` | not stated | 1 | reviewed | The sentence mapped at 2.1:1 contains both RFC9728-2.1-1 and RFC9728-2.1-2: If any human-readable field is sent without a language tag, parties using it MUST NOT make any assumptions about the language, character set, or script of the string value, and the string value MUST be used as is wherever it is presented in a user interface. One site has one mapped-to field, so the second MUST is accounted for here under its existing ID. |
| `2.2` | not stated | 2 | reviewed | not stated |
| `3` | not stated | 3 | reviewed | not stated |
| `3.1` | not stated | 2 | reviewed | not stated |
| `3.2` | not stated | 3 | reviewed | not stated |
| `3.3` | not stated | 5 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `5.1` | not stated | 0 | reviewed | Section 5.1 defines resource_metadata as the URL of the protected resource metadata and permits its use with non-Bearer schemes using MAY. The retained advisory ID has no separate MUST-level inventory site. |
| `5.2` | not stated | 0 | reviewed | not stated |
| `5.3` | not stated | 0 | reviewed | not stated |
| `5.4` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 3 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `7.1` | not stated | 2 | reviewed | not stated |
| `7.2` | not stated | 0 | reviewed | not stated |
| `7.3` | not stated | 2 | reviewed | not stated |
| `7.4` | not stated | 0 | reviewed | not stated |
| `7.5` | not stated | 0 | reviewed | not stated |
| `7.6` | not stated | 0 | reviewed | not stated |
| `7.7` | not stated | 0 | reviewed | not stated |
| `7.8` | not stated | 0 | reviewed | not stated |
| `7.9` | not stated | 0 | reviewed | not stated |
| `7.10` | not stated | 0 | reviewed | Section 7.10 says normal HTTP caching behaviours apply and recommends caching directives with lowercase should. The existing advisory IDs record that text and have no MUST-level inventory sites. |
| `8` | Registration procedure addresses IANA and designated experts | 0 | reviewed | Registration procedure addresses IANA and designated experts. Its lowercase registry instructions do not create an additional MUST-level site in this document's selected inventory. |
| `8.1` | not stated | 0 | reviewed | not stated |
| `8.1.1` | not stated | 0 | reviewed | not stated |
| `8.1.2` | not stated | 0 | reviewed | not stated |
| `8.2` | not stated | 0 | reviewed | not stated |
| `8.2.1` | not stated | 0 | reviewed | not stated |
| `8.3` | not stated | 0 | reviewed | not stated |
| `8.3.1` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 0 | reviewed | not stated |
| `9.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the protected-resource metadata client, the party that queries the document, which Ze does not implement: Thomas's 2026-09-21 scope decision excludes protected-resource discovery clients. Ze publishes the document (internal/component/mcp/oauth.go::writeResourceMetadata, served by internal/component/mcp/streamable.go::ServeHTTP) and never requests another resource's metadata; internal/component/mcp/as_metadata.go::fetchASMetadata queries RFC 8414 authorization server metadata, a different document. No layer acts as this client on Ze's behalf. The row RFC9728-3.1-2 was retired 2026-09-29 (rfc/corrections/rfc9728.md). | A protected resource metadata document MUST be queried using an HTTP GET request at the previously specified URL. |
| `3.3:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.3 repeats: If these values are not identical, the data contained in the response MUST NOT be used. The first occurrence follows the well-known URL identity check and maps directly at 3.3:2; this occurrence follows discovery through WWW-Authenticate. RFC9728-3.3-2 now explicitly includes both contexts, so the duplicate target records the obligation of this sentence as well. | If these values are not identical, the data contained in the response MUST NOT be used. |

## Superseded

No document obsoletes RFC 9728, so its obligations are stated where they were written.
