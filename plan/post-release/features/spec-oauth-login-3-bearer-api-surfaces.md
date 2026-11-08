# Spec: oauth-login-3-bearer-api-surfaces

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Post-release by Thomas's decision (umbrella Task). A further auth mode on surfaces that already authenticate; no shipped surface is fail-open without it |
| RFC gaps | - |
| Depends | `plan/post-release/features/spec-oauth-login-1-token-verifier.md`, `plan/post-release/features/spec-oauth-login-2-identity-mapping.md`, `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: plan/post-release/features/spec-oauth-login-0-umbrella.md (child 3).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

REST, gRPC, gNMI and the looking glass accept an OAuth access token as a
further auth mode, beside the shared-token and per-user modes they have today.
MCP, which already accepts a token, moves onto child 2's mapping so a token
there also runs as a Ze user. Each surface reads the token only from its own
transport (`Authorization` header, or gRPC `authorization` call metadata),
verifies it with child 1's verifier and resolves it with child 2's mapping.

Surfaces covered: REST, gRPC, gNMI, MCP, looking glass. They form one child
because they share the bearer pattern and the listener auth reload; the web
UI's browser redirect flow (child 4) and SSH (child 5) are different
mechanisms.

VyOS defect this child carries (umbrella, "The VyOS reference"): `fa46c294c`,
a client-supplied `X-Client-Verify: SUCCESS` header passed authentication when
mTLS was off (GHSA-4w7q-wwx4-6973): AC-3.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/api-surfaces.md`, `ai/doors/hub-engine.md`, `ai/doors/access-security.md`
  → Constraint: the surfaces are thin adapters; always-on code never imports a gated surface; `cmd/ze/hub` reaches each through a registration seam
  → Constraint: "Each surface carries its own authentication and TLS. A listener that serves without it is a security defect, not a default" (`api-surfaces.md`)
- [ ] `docs/architecture/api/architecture.md`, "Identity and Authorization" - REST and gRPC auth precedence, reload staging
  → Constraint: identity is injected by the transport and is never accepted from the client payload
  → Constraint: REST constructors refuse every non-loopback address (`NewRESTServer`, `RESTServer.Reconfigure`)
- [ ] `docs/architecture/mcp/overview.md` - the MCP auth modes, the RFC 9728 metadata route

### RFC texts
- [ ] `rfc/full/rfc9728.txt`, `rfc/full/rfc8707.txt` - in the `oauth-mgmt` profile; summaries exist. Rows the new surfaces implement are updated (Documentation row 9)
- [ ] `rfc/full/rfc6750.txt` - text present; no profile lists it, so no `rfc/short/` summary is written. Enrolling it in `oauth-mgmt` is Thomas's decision

**Key insights:**
- REST and gRPC already authenticate through one seam, `api.Authentication.Authenticate(authHeader)`, which the hub publishes per generation (`liveAcceptedAPIAuthentication`). The OAuth mode fills that seam; neither server gains its own verifier.
- gNMI and the looking glass compare one shared token today and learn no username; `spec-login-identity-for-looking-glass-and-gnmi.md` gives them a per-user path first.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/api/rest/auth.go` - `RESTServer.withAuth` reads only the `Authorization` header, calls `authentication.Authenticate`, and publishes the trusted identity; `callerIdentity` reads it back
- [ ] `internal/component/api/grpc/server.go` - `GRPCServer.checkAuth` reads only the `authorization` call metadata and calls the same seam; `NewGRPCServer` installs the auth interceptors and a server-only TLS config
- [ ] `internal/component/gnmi/server.go` - `Server.checkAuth`: one shared token from call metadata, no username; no token means no check
- [ ] `internal/component/lg/auth.go` - `bearerAuth`: optional shared token, constant-time compare; no token means a public surface
- [ ] `internal/component/mcp/oauth.go` - `oauthAuthenticator.Authenticate` answers the token subject as the MCP identity, with no Ze user behind it
- [ ] `cmd/ze/hub/mgmt_auth_reload.go` - `apiAuthReloader` classifies a candidate exposure as authenticated when users or a token exist

**Behavior to preserve:**
- Shared-token and per-user modes on REST, gRPC and gNMI; the looking glass's opt-in token
- MCP `oauth` mode, its RFC 9728 metadata document and its challenge strings
- REST stays loopback-only

**Behavior to change:**
- REST, gRPC, gNMI and the looking glass accept an OAuth access token as a further auth mode
- MCP's token identity resolves to a Ze user through child 2

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| API authentication seam | `internal/component/api/types.go` `Authentication`, `AuthenticationProvider`; `cmd/ze/hub/aaa_lifecycle.go` `liveAcceptedAPIAuthentication` | One per-generation authenticate function REST and gRPC call | Extends: the OAuth mode is one more answer from the same function, from the same generation |
| Listener auth reload classification | `cmd/ze/hub/mgmt_auth_reload.go` `apiAuthReloader` | Decides whether a candidate exposure is authenticated | Extends: an OAuth mode counts as authenticated |
| Per-user gNMI and looking-glass login | `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md` | Gives gNMI and the looking glass a username | Depends: OAuth on gNMI and the looking glass needs that identity path |
| Shared verifier | `plan/post-release/features/spec-oauth-login-1-token-verifier.md` | One verifier in a core leaf | Extends: every surface here calls it |
| Identity mapping | `plan/post-release/features/spec-oauth-login-2-identity-mapping.md` | Token to Ze user and profiles | Extends: every surface here calls it, MCP included |
| MCP challenge and RFC 9728 route | `internal/component/mcp/oauth.go` `challengeError`, `writeResourceMetadata` | RFC 6750 challenge, protected-resource metadata | Does not apply as a change: preserved; whether other surfaces publish RFC 9728 metadata is decided at design against the `oauth-mgmt` summaries |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `Authorization: Bearer <jwt>` on REST, MCP and the looking glass; `authorization` call metadata on gRPC and gNMI

### Transformation Path
1. The surface extracts the token from its own transport, never from any other header
2. The shared verifier checks signature, issuer, audience, expiry, subject and scopes against the configured provider (child 1)
3. The subject (or the configured claim) resolves to a Ze user in the accepted identity generation; an unmapped subject is refused (child 2)
4. The surface attaches that user and its authorizer to the request context, as per-user mode does today

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to identity provider | HTTPS, RFC 8414 metadata and JWKS fetch, bounded timeout | No |
| Surface to dispatcher | Caller identity in context, unchanged | No |

### Changed-Wire Census
**Search evidence:** to be written at design. Wires: the `Authorization` header and gRPC metadata (readers: `withAuth`, `checkAuth` on gRPC and gNMI, `bearerAuth`, `oauthAuthenticator.Authenticate`); the RFC 9728 document (writer: `writeResourceMetadata`).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [to fill at design] | | | | |

### Integration Points
- `liveAcceptedAPIAuthentication` - the OAuth mode in the published generation
- `serverDispatcher` - unchanged

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | to fill at design |
| No unintended coupling | No | the verifier is a core leaf; no surface imports another |
| No duplicated functionality | No | one verifier, one mapping |
| Zero-copy preserved where applicable | No | N-A, not a hot path |
| Registration over hardcoding, outbound | No | to fill at design |
| Registration over hardcoding, inbound | No | to fill at design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `api.Authentication` seam can carry an OAuth answer without a new field | `api/types.go`: it takes the raw header value and answers a `CallerIdentity` | the seam grows one field, used by REST and gRPC alike | design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A reverse proxy in front of a surface injects identity headers an operator expects Ze to trust | Operator report | AC-3: Ze reads no identity header, and the guide says so |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A management surface admits a caller it should refuse |
| How is it reverted? | Single commit revert; config selecting the OAuth mode would then be refused |
| Who else touches this path? | The login-identity spec; `plan/release/fixes/spec-mcp-protected-resource-metadata-path.md`; `plan/post-release/features/spec-mgmt-mtls-client-certificate.md` (shares AC-3's header rule) |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| REST request with a valid token | → | shared verifier from `withAuth` | `rest-oauth-bearer.ci` (planned, `test/plugin/`) |
| gNMI Get with a valid token | → | shared verifier from `Server.checkAuth` | `gnmi-oauth-bearer.ci` (planned, `test/plugin/`) |
| gRPC call with a valid token | → | shared verifier from `GRPCServer.checkAuth` | `grpc-oauth-bearer.ci` (planned, `test/plugin/`) |
| Looking-glass request with a valid token | → | shared verifier from `bearerAuth` | `lg-oauth-bearer.ci` (planned, `test/plugin/`) |
| MCP request with a valid token | → | child 2's mapping | `mcp-oauth-user.ci` (planned, `test/plugin/`) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | REST, gRPC, gNMI, MCP or looking-glass request with a token the configured provider signed, correct issuer and audience, subject mapped to user `alice` | Command runs as `alice` with `alice`'s profiles; audit records `alice` and the OAuth door | None |
| AC-2 | Same request with an expired token, a wrong signature, `alg=none`, or an unknown `kid` | 401 (gRPC `Unauthenticated`) naming the failed check from the fixed vocabulary; no infrastructure detail in the reply | None |
| AC-3 | Request with no credential and any of `X-Client-Verify: SUCCESS`, `X-Remote-User`, `X-Forwarded-User`, `Remote-User`, `X-Client-DN` | 401 on every surface; only the transport's own `Authorization` header (or gRPC `authorization` metadata) and the TLS session ever set identity or a "verified" state | None |
| AC-4 | A surface configured for the OAuth mode, then a reload | `apiAuthReloader` classifies the exposure as authenticated; a non-loopback listener with only the OAuth mode is not refused as unauthenticated | None |
| AC-5 | The provider's discovery or JWKS endpoint hangs (child 1, AC-6) | The surface keeps serving its other auth modes; token requests are refused with a generic message | None |

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Calls REST with a Keycloak token | header → verifier → user → dispatcher | `rest-oauth-bearer.ci` |
| 2 | Runs gnmic with a token | metadata → verifier → user → config session | `gnmi-oauth-bearer.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNoIdentityFromForwardedHeaders` | each surface's auth test (REST, gRPC, gNMI, MCP, looking glass) | AC-3 | |
| `TestOAuthModeCountsAsAuthenticated` | `cmd/ze/hub` | AC-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rest-oauth-bearer` | `test/plugin/` | AC-1, AC-2, AC-3 against a stub issuer | |
| `gnmi-oauth-bearer` | `test/plugin/` | AC-1 on gNMI | |
| `grpc-oauth-bearer`, `lg-oauth-bearer`, `mcp-oauth-user` | `test/plugin/` | AC-1 on gRPC, the looking glass and MCP | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `oauth-keycloak` | `test/interop/scenarios/` | Keycloak, latest stable | A real IdP's tokens and discovery document are accepted; a token minted for another client is refused | |

Before the scenario expects Keycloak to issue or publish anything, read that
behaviour in Keycloak's source at the version the lab runs and name the
function (`ai/rules/interop-and-goal-validation.md`).

## Files to Modify
- `internal/component/api/rest/auth.go`, `internal/component/api/grpc/server.go`, `internal/component/gnmi/server.go`, `internal/component/lg/auth.go` - OAuth mode
- `internal/component/mcp/oauth.go` - identity through child 2
- `cmd/ze/hub/aaa_lifecycle.go`, `cmd/ze/hub/mgmt_auth_reload.go` - the OAuth mode in the published generation, reload classification
- `docs/architecture/api/architecture.md`, `docs/architecture/mcp/overview.md`, `docs/guide/authentication.md`, `docs/guide/api.md`

## Files to Create
- the functional tests above and the `oauth-keycloak` interop scenario

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | each surface's auth mode references child 1's provider |
| YANG validation constraints | Yes | the OAuth mode requires a provider reference |
| YANG custom validators | Yes | provider reference resolves |
| CLI commands/flags | N-A | child 1 owns the status command |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md` for the new config |
| Editor autocomplete | Yes | provider name completion |
| Functional test for new RPC/API | Yes | above |
| Pipe completeness | N-A | no command |
| Env var registration | N-A | no env override planned |
| Doctor check for runtime dependencies | N-A | child 1 |
| Prometheus counters/metrics | N-A | child 1 counts accept and refuse by reason |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature | Yes | `features/` entry |
| 2 | Config syntax changed | Yes | `docs/guide/configuration.md` |
| 4 | API/RPC added | Yes | `docs/architecture/api/commands.md` |
| 6 | User guide | Yes | `docs/guide/authentication.md`, `docs/guide/api.md` |
| 9 | RFC behavior | Yes | `rfc/short/rfc9728.md`, `rfc8414.md`, `rfc8707.md` rows where the new surfaces implement them |
| 12 | Architecture | Yes | `docs/architecture/api/architecture.md`, "Identity and Authorization" |

## Implementation Steps
1. **Phase: REST and gRPC** - through the shared seam: AC-1 to AC-4
2. **Phase: gNMI and looking glass** - after the login-identity spec: AC-1 to AC-3
3. **Phase: MCP onto the mapping** - AC-1
4. **Phase: Interop** - `oauth-keycloak`, AC-5

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Identity source | Only the verified token and the TLS session set identity (AC-3) |
| Error leakage | Fixed vocabulary in replies, detail only in logs (AC-2) |
| Exposure | A non-loopback listener is never classified unauthenticated while a mode is set (AC-4) |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| - | - | none |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Fill the existing `api.Authentication` seam | A per-server OAuth check | One generation answers every mode; a second check per server would be a second declaration |

## Known Limitations
- Mutual TLS client certificates are `plan/post-release/features/spec-mgmt-mtls-client-certificate.md`.
- Certificate-bound access tokens (RFC 8705 Section 3) are child 6, `plan/post-release/features/spec-oauth-login-6-certificate-bound-tokens.md`.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le test functional plugin`, `./le test integration` for `oauth-keycloak`, and the scoped package tests of `api/rest`, `api/grpc`, `gnmi`, `lg`, `mcp`, `cmd/ze/hub`
