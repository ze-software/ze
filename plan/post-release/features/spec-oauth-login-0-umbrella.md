# Spec: oauth-login-0-umbrella

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Thomas, 2026-11-08: "we should allow oauth for login, but after release". A new way to log in to management surfaces that already authenticate; no shipped surface is fail-open without it, so it is post-release feature work |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md`, `plan/release/fixes/spec-login-service-authorisation.md` |
| Phase | umbrella: six children, all skeletons |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas, 2026-11-08: "we should allow oauth for login, but after release", and
"make an umbrella spec for it". This umbrella replaces
`spec-mgmt-oauth-bearer-and-web-sso`, written
earlier the same day from VyOS's OAuth and web SSO feature. Every acceptance
criterion, assumption, risk and reference of that spec now lives in a child
below; the table "Where the old spec went" maps each one.

An operator who runs an identity provider (Keycloak, Entra ID, Okta, Dex)
wants one login for every management surface. Today only MCP accepts an
OAuth 2.1 access token. REST, gRPC and gNMI accept a shared token or a local
user, the looking glass accepts an optional shared token, the web UI accepts a
local user through its login form or HTTP Basic, and SSH accepts a password or
a public key.

The work makes Ze's existing OAuth resource server, today private to MCP, the
one verifier every surface uses; maps the identity a token carries to a Ze user
and its authorization profiles; accepts bearer tokens on the API surfaces; adds
OpenID Connect sign-in to the web UI and to SSH; and, once mutual TLS exists,
binds tokens to client certificates. A valid token never grants more than the
mapped user is allowed.

Every operator command runs through the daemon on every surface
(`ai/META-DECISIONS.md`, "Goal and scope"), so each surface is in view. The
children cover them as follows:

| Child | Spec | Work it owns | Surfaces | Why it is a child of its own | Status |
|-------|------|--------------|----------|------------------------------|--------|
| 1 | `plan/post-release/features/spec-oauth-login-1-token-verifier.md` | Move the MCP verifier (`verifyJWT`, JWKS cache, RFC 8414 metadata fetch) to one core leaf; make it fail closed on an empty issuer or audience; one provider definition in config; bounded fetches; doctor check, status command and metrics | MCP (the existing consumer, moved). The status command answers on SSH, REST, gRPC, MCP and web | Every other child consumes it, and it lands with no new login behaviour: MCP stays green unchanged, so it is a move plus the fail-closed fix and lands first on its own | skeleton |
| 2 | `plan/post-release/features/spec-oauth-login-2-identity-mapping.md` | Resolve a verified token's subject or claims to a Ze user and its authz profiles in the accepted identity generation, producing the same `aaa.AuthResult` a TACACS+ or RADIUS login produces; refuse an unmapped subject; apply the login-service gate; audit; revocation | All surfaces: it is the one mapping every surface calls | Mapping is a separate trust decision from signature checking (VyOS's full-privilege default was a mapping defect, not a verifier defect), and it extends the AAA generation rather than any surface | skeleton |
| 3 | `plan/post-release/features/spec-oauth-login-3-bearer-api-surfaces.md` | An OAuth auth mode on REST, gRPC, gNMI and the looking glass, beside the shared-token and per-user modes; MCP moves onto the mapping; no identity from any forwarded header | REST, gRPC, gNMI, MCP, looking glass | These surfaces share the bearer pattern (header or call metadata) and the listener auth reload; a browser redirect flow is a different mechanism | skeleton |
| 4 | `plan/post-release/features/spec-oauth-login-4-web-sign-in.md` | OpenID Connect sign-in to the web UI as a relying party: authorization code with PKCE, `state`, `nonce`, ID token through the shared verifier, the same `ze-session` cookie | Web UI, and every page it serves (the web CLI, the workbench) | Ze acts as an OAuth client here, not a resource server: it owns a client registration, a redirect route and a session, none of which the bearer surfaces have | skeleton |
| 5 | `plan/post-release/features/spec-oauth-login-5-ssh-device-login.md` | SSH login through the identity provider with the OAuth device authorization grant (RFC 8628) over SSH keyboard-interactive; a remote `ze cli` over SSH inherits it | SSH, and the CLI over SSH | SSH has no bearer header and no browser; the device grant is the flow that fits a terminal. It reuses child 4's client registration, so it follows child 4 | skeleton |
| 6 | `plan/post-release/features/spec-oauth-login-6-certificate-bound-tokens.md` | Certificate-bound access tokens (RFC 8705 Section 3): a token carrying a `cnf` `x5t#S256` thumbprint is accepted only over a TLS session whose client certificate matches | gRPC and gNMI, the bearer surfaces on which the mTLS spec puts a client certificate (REST stays loopback-only; MCP and the looking glass only if a later spec gives them client certificates) | The old spec put it out of scope until mutual TLS exists; it needs `spec-mgmt-mtls-client-certificate.md` first, so it is its own spec rather than a line in Known Limitations | skeleton |

Each child keeps its own design, acceptance criteria and evidence. The
umbrella adds no requirement and no code.

### The VyOS reference

VyOS T8989 (`~/Code/github.com/vyos/vyos-1x`, `git log --grep=T8989`) added
OIDC to its HTTP API and then fixed three security defects. Each one is an
acceptance criterion in a child, because each one is a trap a new verifier
falls into:

| VyOS commit | Defect | Old spec criterion | Now owned by |
|-------------|--------|--------------------|--------------|
| `fa46c294c` | A client-supplied `X-Client-Verify: SUCCESS` header passed authentication when mTLS was off (GHSA-4w7q-wwx4-6973) | AC-3 | child 3 AC-3 (API surfaces), child 4 AC-3 (web) |
| `f75c7b74d` | OIDC without an audience accepted any token the issuer minted for any application, with full privileges | AC-4, AC-5, AC-7 | child 1 AC-4 and AC-5, child 2 AC-1 |
| `d8da36885` | The OIDC discovery request had no timeout | AC-6 | child 1 AC-6 |

### Where the old spec went

| Old spec item | Child |
|---------------|-------|
| AC-1 (token on REST, gRPC or gNMI runs as `alice`, audit records the OAuth door) | 3 AC-1 (the surfaces), 2 AC-4 (the user, profiles and audit fields) |
| AC-2 (expired, wrong signature, `alg=none`, unknown `kid`) | 3 AC-2, with the verifier's fixed vocabulary in 1 AC-1 |
| AC-3 (no identity from forwarded headers) | 3 AC-3 (API surfaces), 4 AC-3 (web) |
| AC-4 (provider with no audience refused at verify and validate) | 1 AC-4 |
| AC-5 (verifier fails closed on empty issuer or audience) | 1 AC-5 |
| AC-6 (bounded discovery and JWKS fetch) | 1 AC-6 (the fetch), 3 AC-5 (other auth modes keep serving) |
| AC-7 (unmapped subject refused, 403) | 2 AC-1 |
| AC-8 (login-service set excludes OAuth) | 2 AC-3 |
| AC-9, AC-10 (web OIDC code flow, state, nonce, reused code) | 4 AC-1, AC-2 |
| AC-11 (reload changes provider, audience or mapping) | 1 AC-7 (provider, audience), 2 AC-5 (mapping) |
| AC-12 (doctor: provider unreachable, JWKS empty, mapped user missing) | 1 AC-8 (provider, JWKS), 2 AC-6 (mapped user) |
| AC-13 (status command on every surface) | 1 AC-9 |
| A-1 (verifier holds no MCP logic beyond its realm) | 1 A-1 |
| A-2 (every surface resolves a subject against the accepted generation) | 2 A-1 |
| R-1 (full privileges by default) | 2 R-1 |
| R-2 (reverse proxy identity headers) | 3 R-1, 4 R-2 |
| Wiring: REST, gNMI | 3 |
| Wiring: web OIDC callback | 4 |
| Wiring: MCP after the move | 1 |
| User stories 1, 2 | 3 |
| User story 3 | 4 |
| `TestVerifierRefusesEmptyIssuer`, `TestVerifierRefusesEmptyAudience`, `TestDiscoveryTimeoutBounded` | 1 |
| `TestUnmappedSubjectRefused` | 2 |
| `TestNoIdentityFromForwardedHeaders` | 3, 4 |
| Functional `rest-oauth-bearer`, `gnmi-oauth-bearer` | 3 |
| Functional `oauth-config-audience-required` | 1 |
| Functional `web-oidc-login` | 4 |
| Interop `oauth-keycloak` | 3 (bearer), extended by 4 and 5 |
| Boundary test: clock leeway | 1 |
| Key decision: move the MCP verifier to a core leaf | 1 |
| Key decision: map subject to a Ze user | 2 |
| Blast radius: the two Depends specs, `spec-mcp-protected-resource-metadata-path.md` | 1, 2, 3 |
| Known limitation: mTLS client certificates | referenced by 3 and 6; absorbed by none |
| Known limitation: certificate-bound tokens (RFC 8705 Section 3) | 6 |
| Required reading, Current Behavior files, Files to Modify, Integration and Documentation checklists, Security Review rows, targeted runs | each row is carried by the child that changes that file or owns that check |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md`, `ai/doors/api-surfaces.md`, `ai/doors/commands-cli.md`, `ai/doors/hub-engine.md`, `ai/doors/core.md`
  → Constraint: every surface is behind its own feature gate (`ze_mcp`, `ze_rest`, `ze_grpc`, `ze_gnmi`, `ze_web`, `ze_lg`, `ze_ssh`), so the shared verifier and the mapping cannot live in any one of them; always-on code never imports a gated surface
  → Constraint: "Login is not an SSH feature. SSH, web, REST and gRPC read one accepted generation of users" (`access-security.md`); the mapping extends that generation
- [ ] `docs/architecture/mcp/overview.md` - the MCP auth modes, the RFC 9728 metadata route
  → Constraint: the MCP `oauth` mode is the existing resource server; child 1 moves it, never copies it
- [ ] `docs/architecture/api/architecture.md`, "Identity and Authorization"
  → Constraint: identity is injected by the transport and is never accepted from the client payload
- [ ] `docs/architecture/web-interface.md` - web routes, the login form, the session cookie
- [ ] `ai/digests/aaa-auth.md` - how a login reaches the chain and RBAC

### RFC texts
- [ ] `rfc/full/rfc9728.txt`, `rfc/full/rfc8414.txt`, `rfc/full/rfc8707.txt` - in the `oauth-mgmt` profile; summaries exist
- [ ] `rfc/full/rfc6749.txt`, `rfc/full/rfc6750.txt`, `rfc/full/rfc7519.txt`, `rfc/full/rfc7636.txt`, `rfc/full/rfc8705.txt` - texts present; no profile lists them, so no `rfc/short/` summary is written (`ai/META-DECISIONS.md`, "RFC validation scope does not grow"). Enrolling them in `oauth-mgmt` is Thomas's decision
- RFC 8628 (device authorization grant, child 5) has no text under `rfc/full/` yet; child 5 adds it, which the same META rule allows, and writes no summary
- OpenID Connect Core 1.0 is not an RFC; children 4 and 5 cite it by section from the OpenID Foundation text

**Key insights:**
- MCP already fails closed on an empty audience (`audClaim.Matches`), but `verifyJWT` skips the issuer check when the expected issuer is empty; only `buildOAuthAuthenticator` (which refuses an empty authorization server) and `fetchASMetadata` (which refuses an empty `issuer`) keep that out of reach today. A shared verifier must refuse it itself (child 1, AC-5).
- `aaa.AuthResult` already carries a backend-produced profile list and a `Source` (TACACS+ and RADIUS fill both), and `SessionStore.createSession` takes an `AuthResult`. A token login that produces an `AuthResult` reaches every existing session and authorization path with no new carrier (child 2).
- REST and gRPC already authenticate through one seam, `api.Authentication.Authenticate(authHeader)`, published per generation by the hub (`liveAcceptedAPIAuthentication`). The OAuth mode fills that seam (child 3).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/mcp/oauth.go`, `jwt.go`, `jwks.go`, `as_metadata.go` - the MCP-private resource server (child 1)
- [ ] `internal/component/aaa/types.go` - `AuthResult`, `AuthRequest`, `ChainAuthenticator` (child 2)
- [ ] `internal/component/api/rest/auth.go`, `internal/component/api/grpc/server.go`, `internal/component/gnmi/server.go`, `internal/component/lg/auth.go` - bearer and shared-token auth (child 3)
- [ ] `internal/component/web/auth.go` - `SessionStore.createSession`, `AuthMiddlewareWithAudit`, `LoginHandlerWithAudit` (child 4)
- [ ] `internal/component/ssh/ssh.go` - `wish.WithPublicKeyAuth`, `wish.WithPasswordAuth` (child 5)
- [ ] `cmd/ze/hub/aaa_lifecycle.go`, `cmd/ze/hub/mgmt_auth_reload.go` - accepted identity generation, listener auth reload (children 2 and 3)

**Behavior to preserve:**
- every behaviour each child lists as preserved: MCP `oauth` mode and its RFC 9728 document, the shared-token and per-user modes, the web login form, SSH password and public key, REST loopback-only

**Behavior to change:**
- none at umbrella level; each child owns its own

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Umbrella specs | `plan/post-release/features/spec-vpn-concentrator-0-umbrella.md`, `spec-home-cpe-0-umbrella` (closed) | A `-0-umbrella` spec lists its children, each child carries a Parent line, and the umbrella closes when they close | extends: the same shape |
| MCP OAuth resource server | `internal/component/mcp` `oauthAuthenticator`, `verifyJWT`, `jwksCache`, `fetchASMetadata` | Verifies MCP access tokens | does not apply at umbrella level: child 1 moves it to a core leaf and every other child uses it |
| AAA bundle and accepted identity generation | `internal/component/aaa` `AuthResult`, `WithProfileAuthorizer`; `cmd/ze/hub/aaa_lifecycle.go` `newAcceptedLocalIdentity` | One generation of users and profiles for every credential surface, swapped atomically on reload | does not apply at umbrella level: child 2 extends it with token identity |
| Authorization profiles | `internal/component/authz` | Command authorization by profile | does not apply at umbrella level: child 2 maps claims to these profiles; no new authorization model |
| API authentication seam | `internal/component/api/types.go` `Authentication`; `cmd/ze/hub/aaa_lifecycle.go` `liveAcceptedAPIAuthentication` | One per-generation authenticate function REST and gRPC call with the `Authorization` value | does not apply at umbrella level: child 3 fills it with the OAuth mode |
| Login-service authorisation | `plan/release/fixes/spec-login-service-authorisation.md` | A named set of doors an account may use | Depends: child 2 applies it to every token login |
| Per-user gNMI and looking-glass login | `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md` | Gives gNMI and the looking glass a username | Depends: child 3 needs that identity path |
| Web session store | `internal/component/web/auth.go` `SessionStore.createSession` | Cookie sessions from an `AuthResult` | does not apply at umbrella level: child 4 creates the same session |
| SSH auth callbacks | `internal/component/ssh/ssh.go` `wish.WithPasswordAuth`, `wish.WithPublicKeyAuth` | SSH login through the AAA chain or a configured key | does not apply at umbrella level: child 5 adds keyboard-interactive beside them |
| Mutual TLS client certificates | `plan/post-release/features/spec-mgmt-mtls-client-certificate.md` | Planned: client certificates on the same surfaces | referenced, not absorbed: child 6 depends on it |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- None at umbrella level. Each child's entry point: a bearer token on an API request (children 3, 6), the browser returning to a web callback (child 4), an SSH keyboard-interactive login (child 5).

### Transformation Path
1. The surface extracts the credential from its own transport (children 3, 4, 5, 6).
2. The shared verifier checks it against the configured provider (child 1).
3. The mapping resolves it to a Ze user and profiles in the accepted generation (child 2).
4. The surface attaches that identity as it does for a local login today.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to identity provider | HTTPS: RFC 8414 metadata and JWKS fetch; the token endpoint (children 4, 5) | by each child's interop scenario |
| Surface to dispatcher | Caller identity in context, unchanged | by each child |

### Changed-Wire Census

**Search evidence:** none at umbrella level; the umbrella changes no code. Each child carries its own census.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| - | - | - | - | - |

### Integration Points
- None at umbrella level.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | N-A | no code |
| No unintended coupling | N-A | no code; child 1 puts the verifier in a core leaf so no gated surface imports another |
| No duplicated functionality | Yes | one verifier (child 1) and one mapping (child 2), used by every surface child |
| Zero-copy preserved where applicable | N-A | no code |
| Registration over hardcoding, outbound | N-A | no code |
| Registration over hardcoding, inbound | N-A | no code |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The six children cover every surface and every item of the old spec | the "Where the old spec went" table; grep of `plan/` for `oauth` and `oidc` on 2026-11-08 | a seventh child gains a Parent line here | the grep re-run when a child closes | validated (grep) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A surface gains a token login that bypasses the mapping | a surface child whose wiring test resolves no user | child 2 is the only producer of a token `AuthResult`; each surface child's AC-1 asserts the mapped user |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | nothing in the product; the umbrella is planning only |
| How is it reverted? | single commit revert |
| Who else touches this path? | the six children; `spec-mgmt-mtls-client-certificate.md`; `plan/release/fixes/spec-mcp-protected-resource-metadata-path.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| each surface's token login | → | child 1 verifier, child 2 mapping | each child's own Wiring Test table |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Children 1 and 2 closed | One verifier in a core leaf, no copy under `internal/component/mcp`; one mapping every surface calls | none |
| AC-2 | Children 3, 4 and 5 closed | REST, gRPC, gNMI, MCP, looking glass, web and SSH each accept an identity-provider login that runs as the mapped user, proven by each child's functional and interop tests | none |
| AC-3 | Child 6 closed | A certificate-bound token is refused over a TLS session without the matching client certificate | none |
| AC-4 | Every child closed | Every item of the "Where the old spec went" table is demonstrated by its child | none |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| each child's unit tests | the children | AC-1 to AC-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| each child's `.ci`, `.wb` and interop scenarios | the children | an operator logs in through the identity provider | |

## Files to Modify
- none at umbrella level; each child lists its own

## Files to Create
- none at umbrella level

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | children 1, 2, 4, 5 |
| YANG validation constraints | N-A | child 1 |
| YANG custom validators | N-A | children 1, 2 |
| CLI commands/flags | N-A | child 1 |
| CLI grammar (keyword before value) | N-A | child 1 |
| Editor autocomplete | N-A | child 1 |
| Functional test for new RPC/API | N-A | children |
| Pipe completeness | N-A | child 1 |
| Env var registration | N-A | no env override planned |
| Doctor check for runtime dependencies | N-A | children 1, 2 |
| Prometheus counters/metrics | N-A | child 1 |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature | No | children |
| 2 | Config syntax changed | No | children |
| 3 | CLI command added | No | child 1 |
| 4 | API/RPC added | No | children |
| 6 | User guide | No | children |
| 9 | RFC behavior | No | children |
| 12 | Architecture | No | children |
| 14 | Prometheus counters | No | child 1 |
| 16 | Any changed source file referenced by existing doc source anchors? | No | no source changes |

## Implementation Steps

1. **Child 1** - the verifier move and fail-closed fix; MCP green unchanged.
2. **Child 2** - the mapping, after `spec-login-service-authorisation.md` lands.
3. **Child 3** - bearer on REST, gRPC, gNMI, MCP and the looking glass, after `spec-login-identity-for-looking-glass-and-gnmi.md` lands.
4. **Child 4** - web sign-in; may run beside child 3.
5. **Child 5** - SSH device login, after child 4 (shared client registration).
6. **Child 6** - certificate-bound tokens, after child 3 and `spec-mgmt-mtls-client-certificate.md`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every old-spec item has a child; every surface has a child |
| Scope | the umbrella adds no requirement and enrols no RFC |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Children closed | the six child spec files are removed by their closure commits |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Identity source | only the verified token and the TLS session set identity (children 3, 4) |
| Fail closed | empty issuer or audience refuses everything (child 1) |
| Privilege | no default identity for an unmapped subject (child 2) |
| Credential spread | a token never enters the password chain toward TACACS+ or RADIUS (child 5) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A child finds a surface no child covers | a new child spec listed in the Task table |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The split follows Ze's existing seams rather than OAuth's grant types: token checking is a library (core leaf), identity is the AAA generation, and each surface is a thin adapter (`api-surfaces.md`, "They are thin adapters"). A surface child therefore adds only extraction and the reply shape.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| - | - | none owned at umbrella level; each child lists its own |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| An umbrella with six children replaces the single spec | keep one spec with four phases | Thomas asked for an umbrella; the pieces have different dependencies (two release specs, the mTLS spec) and land independently |
| `post-release/features`, P3 | `release/` | Thomas: "but after release" |
| SSH login is a child (5) | leave SSH out | every command runs through the daemon on every surface; SSH is where most operators log in, and the device grant fits it without passing a token through the password chain |
| Certificate-bound tokens are a child (6) | a Known Limitations line, as before | an in-scope item a spec does not do becomes its own spec (`ai/rules/planning.md`) |

## Known Limitations
- Mutual TLS client certificates are `plan/post-release/features/spec-mgmt-mtls-client-certificate.md`, referenced by children 3 and 6, absorbed by none.
- Every child is a skeleton: its design is owed before it moves to `ready`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Every child closed
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written (in the children)
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs, as each child names them: `./le test functional plugin`, `./le test functional web`, `./le test functional parse`, `./le test functional ui`, `./le test integration` with each child's interop scenario, and the scoped package tests of the verifier, `aaa`, `mcp`, `api/rest`, `api/grpc`, `gnmi`, `lg`, `web`, `ssh`

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** edited spec
- [ ] **Commit B:** `remove plan/post-release/features/spec-oauth-login-0-umbrella.md` only
