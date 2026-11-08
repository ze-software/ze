# Spec: oauth-login-4-web-sign-in

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Post-release by Thomas's decision (umbrella Task). A further login on a web UI that already authenticates; nothing shipped is fail-open without it |
| RFC gaps | - |
| Depends | `plan/post-release/features/spec-oauth-login-1-token-verifier.md`, `plan/post-release/features/spec-oauth-login-2-identity-mapping.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: plan/post-release/features/spec-oauth-login-0-umbrella.md (child 4).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The web UI offers an OpenID Connect login beside the local login form. Ze
acts as a relying party: the browser goes to the identity provider, comes back
to a Ze callback route with an authorization code and `state`, Ze exchanges the
code (with its PKCE verifier) at the token endpoint, verifies the ID token with
child 1's verifier (issuer, audience equal to the client id, `nonce`), maps it
with child 2, and issues the same `ze-session` cookie a local login gets.

Surfaces covered: the web UI and every page it serves, the web CLI and the
workbench included, since they all ride the `ze-session` cookie.

Why a child of its own: here Ze is an OAuth client, not a resource server. It
owns a client registration (client id, and a secret or none), a redirect
route, and a browser session, none of which the bearer surfaces (child 3) have.
Child 5 reuses the client registration this child introduces.

VyOS defect this child carries (umbrella, "The VyOS reference"): `fa46c294c`,
identity from a forwarded header: AC-3 here, for the web.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/api-surfaces.md`, `ai/doors/access-security.md`
  → Constraint: markup is `.templ`, with no inline script or style (`api-surfaces.md`, Boundaries)
  → Constraint: always-on code never imports the gated `web` surface
- [ ] `docs/architecture/web-interface.md` - web routes, the login form, the session cookie
- [ ] `ai/patterns/web-endpoint.md` - the files a new page or endpoint needs
- [ ] `ai/digests/aaa-auth.md` - how a login reaches the chain and RBAC

### RFC texts
- [ ] `rfc/full/rfc6749.txt` (Section 4.1, authorization code grant; Section 10.12, CSRF and `state`), `rfc/full/rfc7636.txt` (PKCE) - texts present; no profile lists them, so no `rfc/short/` summary is written (`ai/META-DECISIONS.md`, "RFC validation scope does not grow"). Enrolling them in `oauth-mgmt` is Thomas's decision
- OpenID Connect Core 1.0 (ID token validation, `nonce`) is not an RFC; cite it by section from the OpenID Foundation text

**Key insights:**
- `SessionStore.createSession` takes a username and an `AuthResult` and nothing else, so an OIDC login reaches the same session, profiles and authorizer as a local login once child 2 produces the result.
- `sanitizeReturnTo` and `isSameOriginPath` already confine the post-login redirect to the same origin; the callback reuses them.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/auth.go` - `AuthMiddlewareWithAudit`: `ze-session` cookie, else HTTP Basic against the AAA chain; `LoginHandlerWithAudit`: the login form; `createSession`: one session per user from an `AuthResult`; `validateToken`: TTL and live-user checks; `sanitizeReturnTo`: same-origin redirect

**Behavior to preserve:**
- the web login form and HTTP Basic
- one session per user, its TTL and its revocation on user removal

**Behavior to change:**
- the web UI offers an OIDC login beside the local login form

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Web session store | `internal/component/web/auth.go` `SessionStore`, `createSession` | Cookie sessions after a local login | Extends: an OIDC login creates the same session |
| Login form and audit | `internal/component/web/auth.go` `LoginHandlerWithAudit`, `recordWebAuthFailure` | Local login and its audit record | Extends: the form offers the OIDC login; a refused callback is audited the same way |
| Same-origin return | `internal/component/web/auth.go` `sanitizeReturnTo`, `isSameOriginPath` | Confines the post-login redirect | Extends: the callback's return path uses it |
| Shared verifier | `plan/post-release/features/spec-oauth-login-1-token-verifier.md` | Verifies a JWT against the provider | Extends: verifies the ID token with audience equal to the client id |
| Identity mapping | `plan/post-release/features/spec-oauth-login-2-identity-mapping.md` | Token to Ze user and profiles | Extends: maps the ID token |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The browser returns from the identity provider to a Ze callback route with an authorization code and `state`

### Transformation Path
1. The operator chooses the OIDC login; Ze stores `state`, `nonce` and the PKCE verifier against a short-lived pre-login cookie and redirects to the provider's authorization endpoint
2. The callback checks `state`, exchanges the code with the PKCE verifier at the token endpoint
3. The shared verifier checks the ID token (issuer, audience equal to the client id, expiry, `nonce`)
4. Child 2 maps it; `createSession` issues the `ze-session` cookie

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Browser to identity provider and back | HTTP redirects | No |
| Ze to identity provider | HTTPS token endpoint, bounded timeout | No |

### Changed-Wire Census
**Search evidence:** to be written at design. Wires: the new callback route, the pre-login cookie, the login page markup.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [to fill at design] | | | | |

### Integration Points
- `createSession` - unchanged, fed by child 2's result

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | to fill at design |
| No unintended coupling | No | to fill at design |
| No duplicated functionality | No | one session store, one verifier |
| Zero-copy preserved where applicable | No | N-A, not a hot path |
| Registration over hardcoding, outbound | No | to fill at design |
| Registration over hardcoding, inbound | No | to fill at design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The provider's RFC 8414 or OIDC discovery document names the authorization and token endpoints child 1 already fetches | `as_metadata.go` reads only `issuer` and `jwks_uri` today | child 1's metadata subset grows two fields | design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Login CSRF or code injection through a forged callback | AC-2 red | `state`, `nonce` and PKCE each checked |
| R-2 | A reverse proxy in front of the web UI injects identity headers an operator expects Ze to trust | Operator report | AC-3: Ze reads no identity header, and the guide says so |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The web UI admits a browser it should refuse |
| How is it reverted? | Single commit revert; config carrying the client registration would then be refused |
| Who else touches this path? | Child 5 reuses the client registration |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Web OIDC callback | → | session creation | `web-oidc-login.wb` (planned, `test/web/`) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Web: operator chooses the OIDC login | Authorization code flow with PKCE (RFC 7636) and `state`; the ID token is verified by the same verifier (issuer, audience equal to the client id, nonce); a `ze-session` cookie is issued for the mapped user | None |
| AC-2 | Web callback with a `state` or `nonce` that does not match, or a reused code | Refused, no session | None |
| AC-3 | Web request with no credential and any of `X-Client-Verify: SUCCESS`, `X-Remote-User`, `X-Forwarded-User`, `Remote-User`, `X-Client-DN` | Refused; only the `ze-session` cookie, HTTP Basic and the TLS session ever set identity | None |
| AC-4 | Valid ID token for a subject child 2 refuses | No session; the login page names that the account has no Ze access, and the refusal is audited | None |

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 3 | Logs in to the web UI through the IdP | redirect → callback → verifier → session | `web-oidc-login.wb` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNoIdentityFromForwardedHeaders` | `internal/component/web` auth test | AC-3 | |
| `TestOIDCCallbackRefusesStateMismatch`, `TestOIDCCallbackRefusesReusedCode` | `internal/component/web` | AC-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| pre-login cookie lifetime | fixed at design | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `web-oidc-login` | `test/web/` | AC-1, AC-2 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `oauth-keycloak` (extended) | `test/interop/scenarios/` | Keycloak, latest stable | A Keycloak code flow ends in a Ze web session for the mapped user | |

## Files to Modify
- `internal/component/web/auth.go` - OIDC login and callback
- `docs/architecture/web-interface.md`, `docs/guide/authentication.md`

## Files to Create
- the login page markup change (`.templ`), the client registration YANG (placement at design, beside child 1's provider)
- the tests above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | client id, client secret (sensitive), redirect URI; shape at design |
| YANG validation constraints | Yes | redirect URI is HTTPS and on the web listener |
| YANG custom validators | Yes | provider reference resolves |
| CLI commands/flags | N-A | none |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md` for the new config |
| Editor autocomplete | Yes | provider name completion |
| Functional test for new RPC/API | Yes | above |
| Pipe completeness | N-A | no command |
| Env var registration | N-A | no env override planned |
| Doctor check for runtime dependencies | Yes | token endpoint reachable, client registration complete |
| Prometheus counters/metrics | N-A | child 1 |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature | Yes | `features/` entry |
| 2 | Config syntax changed | Yes | `docs/guide/configuration.md` |
| 6 | User guide | Yes | `docs/guide/authentication.md` |
| 12 | Architecture | Yes | `docs/architecture/web-interface.md` |

## Implementation Steps
1. **Phase: Client registration** - config, doctor
2. **Phase: Code flow and callback** - AC-1, AC-2, AC-4
3. **Phase: Header hardening and interop** - AC-3, `oauth-keycloak`

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Identity source | Only the cookie, HTTP Basic and the TLS session set identity (AC-3) |
| CSRF and replay | `state`, `nonce`, PKCE, single-use code (AC-2) |
| Open redirect | The return path stays same-origin (`sanitizeReturnTo`) |
| Secret handling | The client secret is `ze:sensitive` and never logged |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| - | - | none |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Issue the existing `ze-session` cookie | Keep the ID token as the session | One session model; revocation and TTL already work |

## Known Limitations
- Mutual TLS client certificates are `plan/post-release/features/spec-mgmt-mtls-client-certificate.md`.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le test functional web`, `./le test integration` for `oauth-keycloak`, and the scoped package test of `web`
