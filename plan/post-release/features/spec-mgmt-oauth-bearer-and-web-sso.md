# Spec: mgmt-oauth-bearer-and-web-sso

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | A new way to log in to management surfaces that already authenticate; no shipped surface is fail-open without it, so it is post-release feature work |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md`, `plan/release/fixes/spec-login-service-authorisation.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator who runs an identity provider (Keycloak, Entra ID, Okta, Dex)
wants one login for every management surface. Today only MCP accepts an
OAuth 2.1 access token. REST, gRPC and gNMI accept a shared token or a local
user, and the web UI accepts a local user through its login form or HTTP Basic.

This spec makes Ze's existing OAuth resource server, today private to MCP,
the one verifier that REST, gRPC and gNMI also use, and adds OpenID Connect
single sign-on to the web UI as a relying party. The identity a token carries
maps to a Ze user and its authorization profiles. A valid token never grants
more than that user is allowed.

VyOS reference: T8989 (`~/Code/github.com/vyos/vyos-1x`, `git log --grep=T8989`)
added OIDC to its HTTP API and then fixed three security defects. Each one is an
acceptance criterion here, because each one is a trap a new verifier falls into:

| VyOS commit | Defect | Ze criterion |
|-------------|--------|--------------|
| `fa46c294c` | A client-supplied `X-Client-Verify: SUCCESS` header passed authentication when mTLS was off (GHSA-4w7q-wwx4-6973) | AC-3 |
| `f75c7b74d` | OIDC without an audience accepted any token the issuer minted for any application, with full privileges | AC-4, AC-5, AC-7 |
| `d8da36885` | The OIDC discovery request had no timeout | AC-6 |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/mcp/overview.md` - the MCP auth modes, the RFC 9728 metadata route
  → Constraint: the MCP `oauth` mode is the existing resource server; this spec moves it, never copies it
- [ ] `docs/architecture/api/architecture.md`, "Identity and Authorization" - REST and gRPC auth precedence, reload staging
  → Constraint: identity is injected by the transport and is never accepted from the client payload
  → Constraint: REST constructors refuse every non-loopback address (`NewRESTServer`, `RESTServer.Reconfigure`)
- [ ] `docs/architecture/web-interface.md` - web routes, the login form, the session cookie
- [ ] `ai/digests/aaa-auth.md` - how a login reaches the chain and RBAC

### RFC texts
- [ ] `rfc/full/rfc9728.txt`, `rfc/full/rfc8414.txt`, `rfc/full/rfc8707.txt` - in the `oauth-mgmt` profile; summaries exist
- [ ] `rfc/full/rfc6750.txt`, `rfc/full/rfc7519.txt`, `rfc/full/rfc6749.txt`, `rfc/full/rfc7636.txt` - texts present; no profile lists them, so no `rfc/short/` summary is written (`ai/META-DECISIONS.md`, "RFC validation scope does not grow"). Enrolling them in `oauth-mgmt` is Thomas's decision
  → Constraint: RFC 7519 Section 4.1.3: "If the principal processing the claim does not identify itself with a value in the "aud" claim when this claim is present, then the JWT MUST be rejected."

**Key insights:**
- MCP already fails closed on an empty audience (`audClaim.Matches`), but `verifyJWT` skips the issuer check when the expected issuer is empty; only `buildOAuthAuthenticator` refuses that. A shared verifier must refuse it itself.
- Every surface is behind its own feature gate (`ze_mcp`, `ze_rest`, `ze_grpc`, `ze_gnmi`, `ze_web`), so the shared verifier cannot live in any one of them.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/mcp/oauth.go` - `oauthAuthenticator.Authenticate` extracts the Bearer token, runs `verifyJWT` with issuer, audience and required scopes, maps each failure to an RFC 6750 challenge from a fixed vocabulary; `buildOAuthAuthenticator` refuses an empty audience or authorization server
- [ ] `internal/component/mcp/jwt.go` - `verifyJWT`: alg allow-list, signature, `exp` required, `nbf`, issuer (skipped when expected is empty), audience (`audClaim.Matches` refuses an empty expectation), `sub` required and free of control characters, scopes
- [ ] `internal/component/mcp/jwks.go`, `as_metadata.go` - JWKS cache and RFC 8414 metadata fetch, both through `oauthHTTPClient` with a 5 second timeout (`defaultJWKSFetchTimeout`, `defaultASMetadataTimeout`)
- [ ] `internal/component/mcp/yang/ze-mcp-conf.yang` - `auth-mode` enum `none`, `bearer`, `bearer-list`, `oauth`; `oauth { authorization-server; audience; required-scopes; }`, both HTTPS-patterned
- [ ] `internal/component/api/rest/auth.go` - `RESTServer.withAuth` reads only the `Authorization` header; `callerIdentity` publishes the trusted identity
- [ ] `internal/component/api/grpc/server.go` - `NewGRPCServer` installs the auth interceptors and a server-only TLS config
- [ ] `internal/component/gnmi/server.go` - `Server.checkAuth`: one shared token from call metadata, no username; no token means no check
- [ ] `internal/component/web/auth.go` - `AuthMiddlewareWithAudit`: `ze-session` cookie, else HTTP Basic against the AAA chain
- [ ] `cmd/ze/hub/aaa_lifecycle.go` - `newAcceptedLocalIdentity`, `liveAcceptedAPIAuthentication`, `stageAPIAuthentication` (fail closed while listeners migrate)
- [ ] `cmd/ze/hub/mgmt_auth_reload.go` - `apiAuthReloader` classifies a candidate exposure as authenticated when users or a token exist

**Behavior to preserve:**
- MCP `oauth` mode, its RFC 9728 metadata document and its challenge strings
- Shared-token and per-user modes on REST, gRPC and gNMI; the web login form
- REST stays loopback-only

**Behavior to change:**
- REST, gRPC and gNMI accept an OAuth access token as a further auth mode
- The web UI offers an OIDC login beside the local login form

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| MCP OAuth resource server | `internal/component/mcp` `oauthAuthenticator`, `verifyJWT`, `jwksCache`, `fetchASMetadata` | Verifies MCP access tokens | Replaces its location: moved to one core leaf package all surfaces import, deleted from `mcp` in the same change (`ai/rules/no-layering.md`); MCP then imports it |
| Accepted local identity generation | `cmd/ze/hub/aaa_lifecycle.go` `newAcceptedLocalIdentity`; `internal/component/aaa/login_profiles.go` `WithProfileAuthorizer` | One generation of users and profiles for REST and gRPC, swapped atomically on reload | Extends: a token subject resolves to a user of the same generation |
| Login-service authorisation | `plan/release/fixes/spec-login-service-authorisation.md` | A named set of doors an account may use | Extends: an OAuth login is a door; a user may be allowed it or not |
| Per-user gNMI login | `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md` | Gives gNMI a username | Depends: OAuth on gNMI needs that identity path |
| Listener auth reload classification | `cmd/ze/hub/mgmt_auth_reload.go` `apiAuthReloader` | Decides whether a candidate exposure is authenticated | Extends: an OAuth mode counts as authenticated |
| Web session store | `internal/component/web/auth.go` `SessionStore` | Cookie sessions after a local login | Extends: an OIDC login creates the same session |
| Doctor checks | `internal/component/mcp/doctor.go` | MCP readiness | Extends: the shared verifier owns a doctor check for discovery and JWKS reachability |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Bearer token: `Authorization: Bearer <jwt>` on REST and MCP, `authorization` call metadata on gRPC and gNMI
- Web SSO: the browser returns from the identity provider to a Ze callback route with an authorization code and state

### Transformation Path
1. The surface extracts the token from its own transport, never from any other header
2. The shared verifier checks signature, issuer, audience, expiry, subject and scopes against the configured provider
3. The subject (or the configured claim) resolves to a Ze user in the accepted identity generation; an unmapped subject is refused
4. The surface attaches that user and its authorizer to the request context, as per-user mode does today

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to identity provider | HTTPS, RFC 8414 metadata and JWKS fetch, bounded timeout | No |
| Surface to dispatcher | Caller identity in context, unchanged | No |

### Changed-Wire Census
**Search evidence:** to be written at design. Wires: the `Authorization` header and gRPC metadata (readers: `withAuth`, `checkAuth` on gRPC and gNMI, `oauthAuthenticator.Authenticate`); the RFC 9728 document (writer: `writeResourceMetadata`).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [to fill at design] | | | | |

### Integration Points
- `newAcceptedLocalIdentity` - token subject lookup
- `serverDispatcher` - unchanged

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | to fill at design |
| No unintended coupling | No | the verifier is a core leaf; no surface imports another |
| No duplicated functionality | No | one verifier, moved, not copied |
| Zero-copy preserved where applicable | No | N-A, not a hot path |
| Registration over hardcoding, outbound | No | to fill at design |
| Registration over hardcoding, inbound | No | to fill at design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The MCP verifier holds no MCP-specific logic beyond its challenge realm | `oauth.go`, `jwt.go` read | The move needs a seam for the realm | design read of `streamable_auth.go` | unvalidated |
| A-2 | Every surface can resolve a subject against the accepted identity generation | `api/architecture.md`, "Identity and Authorization" | gNMI and web need their own lookup | `spec-login-identity-for-looking-glass-and-gnmi.md` lands first | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A token maps to full privileges by default, as VyOS did | A test with an unmapped subject passes | AC-7 refuses unmapped subjects |
| R-2 | A reverse proxy in front of the web UI injects identity headers an operator expects Ze to trust | Operator report | AC-3: Ze reads no identity header, and the guide says so |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A management surface admits a caller it should refuse |
| How is it reverted? | Single commit revert; config under the new provider container would then be refused |
| Who else touches this path? | The two Depends specs; `plan/release/fixes/spec-mcp-protected-resource-metadata-path.md` |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| REST request with a valid token | → | shared verifier from `withAuth` | `rest-oauth-bearer.ci` (planned, `test/plugin/`) |
| gNMI Get with a valid token | → | shared verifier from `Server.checkAuth` | `gnmi-oauth-bearer.ci` (planned, `test/plugin/`) |
| Web OIDC callback | → | session creation | `web-oidc-login.wb` (planned, `test/web/`) |
| MCP after the move | → | same verifier | existing `internal/component/mcp/oauth_e2e_test.go`, unchanged |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | REST, gRPC or gNMI request with a token the configured provider signed, correct issuer and audience, subject mapped to user `alice` | Command runs as `alice` with `alice`'s profiles; audit records `alice` and the OAuth door | None |
| AC-2 | Same request with an expired token, a wrong signature, `alg=none`, or an unknown `kid` | 401 (gRPC `Unauthenticated`) naming the failed check from the fixed vocabulary; no infrastructure detail in the reply | None |
| AC-3 | Request with no credential and any of `X-Client-Verify: SUCCESS`, `X-Remote-User`, `X-Forwarded-User`, `Remote-User`, `X-Client-DN` | 401 on every surface; only the transport's own `Authorization` header (or gRPC `authorization` metadata) and the TLS session ever set identity or a "verified" state | None |
| AC-4 | Config sets an OAuth provider with no audience, on any surface | Refused at verify and by `ze config validate`, naming the leaf and the command that sets it | None |
| AC-5 | The verifier is called with an empty expected audience or an empty expected issuer (config check bypassed) | Every token refused; the verifier fails closed itself and never relies on the config check alone | None |
| AC-6 | Discovery or JWKS endpoint accepts the TCP connection and never answers | Fetch ends within its bounded timeout; requests are refused with a generic message, the log names the endpoint; the surface keeps serving its other auth modes | None |
| AC-7 | Valid token whose subject maps to no Ze user and matches no configured claim-to-profile rule | 403, naming that the subject has no Ze account; never a default or full-privilege identity | None |
| AC-8 | Valid token for a user whose login-service set excludes OAuth | Refused as the login-service spec refuses any other door | None |
| AC-9 | Web: operator chooses the OIDC login | Authorization code flow with PKCE (RFC 7636) and `state`; the ID token is verified by the same verifier (issuer, audience equal to the client id, nonce); a `ze-session` cookie is issued for the mapped user | None |
| AC-10 | Web callback with a `state` or `nonce` that does not match, or a reused code | Refused, no session | None |
| AC-11 | Reload changes the provider, audience or mapping | New requests use the new generation once the reload is accepted; a failed reload leaves the old one serving, as `runReloadContext` does today | None |
| AC-12 | Provider unreachable, JWKS empty, or a token-mapped user missing | `ze doctor` reports each with its cause and the command that fixes it | None |
| AC-13 | Operator runs the OAuth provider status command (name fixed at design) from SSH, REST, gRPC, MCP and web | Same answer on every surface: provider, discovery state, key count, last refresh | None |

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Calls REST with a Keycloak token | header → verifier → user → dispatcher | `rest-oauth-bearer.ci` |
| 2 | Runs gnmic with a token | metadata → verifier → user → config session | `gnmi-oauth-bearer.ci` |
| 3 | Logs in to the web UI through the IdP | redirect → callback → verifier → session | `web-oidc-login.wb` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVerifierRefusesEmptyIssuer`, `TestVerifierRefusesEmptyAudience` | the shared package | AC-5 | |
| `TestNoIdentityFromForwardedHeaders` | each surface's auth test | AC-3 | |
| `TestDiscoveryTimeoutBounded` | the shared package | AC-6, with a server that accepts and never answers | |
| `TestUnmappedSubjectRefused` | `cmd/ze/hub` | AC-7 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| clock leeway, if made configurable | fixed at design | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rest-oauth-bearer` | `test/plugin/` | AC-1, AC-2, AC-3 against a stub issuer | |
| `gnmi-oauth-bearer` | `test/plugin/` | AC-1 on gNMI | |
| `oauth-config-audience-required` | `test/parse/` | AC-4 | |
| `web-oidc-login` | `test/web/` | AC-9, AC-10 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `oauth-keycloak` | `test/interop/scenarios/` | Keycloak, latest stable | A real IdP's tokens and discovery document are accepted; a token minted for another client is refused | |

## Files to Modify
- `internal/component/mcp/oauth.go`, `jwt.go`, `jwks.go`, `as_metadata.go` - moved out; MCP keeps its challenge realm and RFC 9728 route
- `internal/component/api/rest/auth.go`, `internal/component/api/grpc/server.go`, `internal/component/gnmi/server.go` - OAuth mode
- `internal/component/web/auth.go` - OIDC login and callback
- `cmd/ze/hub/aaa_lifecycle.go`, `cmd/ze/hub/mgmt_auth_reload.go` - subject resolution, reload classification
- `docs/architecture/api/architecture.md`, `docs/architecture/mcp/overview.md`, `docs/guide/authentication.md`

## Files to Create
- a core leaf package for the verifier (name at design)
- the functional tests above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | one provider definition referenced by each surface; shape at design |
| YANG validation constraints | Yes | audience mandatory when a provider is set |
| YANG custom validators | Yes | provider reference resolves |
| CLI commands/flags | Yes | the status command (AC-13) |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md` |
| Editor autocomplete | Yes | provider name completion |
| Functional test for new RPC/API | Yes | above |
| Pipe completeness | Yes | status command |
| Env var registration | N-A | no env override planned |
| Doctor check for runtime dependencies | Yes | AC-12 |
| Prometheus counters/metrics | Yes | token accept and refuse counts by reason |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature | Yes | `features/` entry |
| 2 | Config syntax changed | Yes | `docs/guide/configuration.md` |
| 3 | CLI command added | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added | Yes | `docs/architecture/api/commands.md` |
| 6 | User guide | Yes | `docs/guide/authentication.md`, `docs/guide/api.md` |
| 9 | RFC behavior | Yes | `rfc/short/rfc9728.md`, `rfc8414.md`, `rfc8707.md` rows where the new surfaces implement them |
| 12 | Architecture | Yes | `docs/architecture/api/architecture.md`, "Identity and Authorization" |
| 14 | Prometheus counters | Yes | `docs/plugin-development/metrics.md` |

## Implementation Steps
1. **Phase: Wiring** - move the verifier to its core leaf; MCP green unchanged
2. **Phase: Bearer on REST, gRPC, gNMI** - AC-1 to AC-8
3. **Phase: Web SSO** - AC-9, AC-10
4. **Phase: Reload, doctor, status command** - AC-11 to AC-13

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Identity source | Only the verified token and the TLS session set identity (AC-3) |
| Fail closed | Empty issuer or audience refuses everything (AC-5) |
| Privilege | No default identity for an unmapped subject (AC-7) |
| Resource exhaustion | Bounded fetch timeouts and refresh rate (AC-6) |
| Error leakage | Fixed vocabulary in replies, detail only in logs |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Move the MCP verifier to a core leaf | Copy it into each surface; import `mcp` from REST | Copying is a second declaration; importing one gated surface from another is forbidden |
| Map subject to a Ze user | Grant a fixed identity to any valid token (VyOS) | A valid token proves who, not what Ze allows |

## Known Limitations
- Mutual TLS client certificates are `plan/post-release/features/spec-mgmt-mtls-client-certificate.md`.
- Certificate-bound access tokens (RFC 8705 Section 3) need that spec first, and are not in this one.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le test functional plugin`, `./le test functional web`, `./le test functional parse`, and the scoped package tests of the verifier, `mcp`, `api/rest`, `api/grpc`, `gnmi`, `web`
