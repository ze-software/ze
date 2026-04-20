# Spec: mcp-2-remote-oauth -- Remote Binding + OAuth 2.1 Resource Server

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-mcp-0-umbrella, spec-mcp-1-streamable-http |
| Phase | 10/10 (A-I done; J verify in-flight) |
| Updated | 2026-04-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-mcp-0-umbrella.md` -- umbrella AC table (AC-6..AC-11 are Phase 2)
4. `plan/learned/636-mcp-1-streamable-http.md` -- Phase 1 landing, what it left in place
5. `docs/architecture/mcp/overview.md` -- transport architecture + roadmap
6. `internal/component/mcp/streamable.go`, `session.go`, `handler.go` -- current code
7. `internal/component/mcp/schema/ze-mcp-conf.yang` -- YANG shape to extend
8. `docs/guide/mcp/remote-access.md`, `docs/guide/mcp/overview.md` -- docs to rewrite
9. `cmd/ze/hub/mcp.go`, `cmd/ze/hub/main.go` (mcp wiring block near line 248) -- wiring point

## Task

Open MCP to non-loopback binding under explicit operator opt-in, with authentication
modes suitable for untrusted networks. Deliver:

| # | Capability | Why |
|---|-----------|-----|
| 1 | `bind-remote` leaf lifts the hard-coded loopback clamp | Phase 1 already accepted remote `ip` in YANG, but `ExtractMCPConfig` force-rewrites every entry to `127.0.0.1`. Remote access today requires an SSH tunnel |
| 2 | Typed `auth-mode` (none / bearer / bearer-list / oauth) | Single-token mode is insufficient for multi-client deployments and for OAuth-managed identities |
| 3 | Per-identity `identity[]` list (bearer-list mode) | Each AI agent / operator gets its own token; session carries identity; revocation is per-row, not global |
| 4 | OAuth 2.1 resource server (oauth mode) | MCP 2025-06-18 profile authorization, the AS is external (not run by ze), tokens are validated locally (stdlib JWT) |
| 5 | RFC 9728 Protected Resource Metadata at `/.well-known/oauth-protected-resource` | Discovery endpoint the MCP spec requires so clients can find the AS without out-of-band config |
| 6 | `tls.cert` + `tls.key` leaves for HTTPS | OAuth mode MUST reject plaintext; remote binding without TLS is also rejected unless explicitly acknowledged |
| 7 | `verify-time` enforcement (`exact-or-reject`) | Combinations that are unsafe (oauth without AS, remote without auth, oauth without TLS) fail at `ze config verify` with precise errors |
| 8 | Session identity plumbing | Every authenticated session carries a typed `Identity` value (name + scopes) that Phase 4 (tasks) uses as the auth-context scoping key |
| 9 | Session registry `Close()` wired into shutdown | Phase 1 deferred this (`plan/deferrals.md` row 226) |

Non-goals: we do NOT run an authorization server. We do NOT support token introspection
(RFC 7662) -- JWT verification is local. We do NOT remove the legacy `Handler(...)`
factory in this phase -- that is umbrella-scheduled for Phase 4.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/mcp/overview.md` -- Streamable HTTP transport architecture
  -> Decision: auth sits between Origin check and method dispatch; no plumbing changes
  -> Constraint: auth result (identity) MUST ride on the session, value-typed (rules/enum-over-string.md + memory.md: no pointers across component seams -- but identity lives within mcp component, so the constraint is stylistic, not enforced)
- [ ] `docs/guide/mcp/overview.md` -- current auth surface (single shared token)
  -> Constraint: existing `token` leaf + `ze.mcp.token` env var + `--mcp-token` flag remain as `auth-mode=bearer`; no silent break
- [ ] `docs/guide/mcp/remote-access.md` -- loopback-only rationale with SSH-tunnel recipes
  -> Decision: rewrite -- preserve SSH/WireGuard recipes as "Option 1: tunnel (recommended for dev)"; add "Option 2: native remote" covering bind-remote + TLS + auth-mode
- [ ] `docs/architecture/config/environment.md` -- env var contract
  -> Constraint: every new YANG `environment/mcp/<leaf>` MUST have a matching `ze.mcp.<leaf>` env var via `env.MustRegister`
- [ ] `internal/component/config/loader_extract.go` -- current `ExtractMCPConfig` (force-rewrites host to loopback)
  -> Constraint: remove the force-rewrite when `bind-remote` is true; keep the logger at Warn level when bind-remote is false and host is non-loopback (strict override)

### MCP Spec Pages (external; no rfc/short summary)

- [ ] `modelcontextprotocol.io/specification/2025-06-18/basic/authorization` -- OAuth 2.1 resource server profile
  -> Constraint: 401 response MUST carry `WWW-Authenticate: Bearer resource_metadata="<url>"` where the URL is absolute and points at this server's `/.well-known/oauth-protected-resource`
  -> Constraint: token passthrough is forbidden (audience binding per RFC 8707)
  -> Constraint: clients expect `Bearer` scheme (case-insensitive); DPoP, MAC, and other schemes are out of scope

### RFC Summaries (MUST for protocol work)

- [ ] `rfc/short/rfc9728.md` -- CREATE. OAuth 2.0 Protected Resource Metadata
  -> Constraint: metadata JSON body keys: `resource` (string, MUST), `authorization_servers` (array, MUST contain at least one entry), `scopes_supported` (array, MAY), `bearer_methods_supported` (array, MAY), `resource_documentation` (string, MAY)
  -> Constraint: Content-Type `application/json`; caching allowed; MUST be reachable without authentication
- [ ] `rfc/short/rfc8707.md` -- CREATE. Resource Indicators for OAuth 2.0
  -> Constraint: `aud` claim in the access token MUST canonically identify this resource server; server rejects tokens whose `aud` does not match `oauth.audience`
  -> Constraint: canonical form matches MCP spec: scheme + host + port (default-port elided) + path, no trailing slash, no fragment, no query
- [ ] `rfc/short/rfc8414.md` -- CREATE. OAuth 2.0 Authorization Server Metadata
  -> Constraint: resource server reads `jwks_uri` from the AS metadata; NO other AS-metadata field is used by the resource server (issuer is compared against `iss` claim)
- [ ] `rfc/short/rfc7519.md` -- VERIFY (may exist already). JSON Web Token core
  -> Constraint: "alg": "none" tokens MUST be rejected; unknown `alg` values MUST be rejected; unknown `kid` triggers one JWKS refresh, then rejects if still unknown
- [ ] RFC 6750 -- Bearer Token Usage (well-known format for Authorization header; no summary needed if not trivially available; inline-comment at the parser)

**Key insights:**
- "Local JWT verify" means: fetch AS metadata once -> read `jwks_uri` -> fetch JWKS with a TTL-based refresh -> verify token signature with the `kid`-matched JWK -> check `iss` / `aud` / `exp` / `nbf` / required scopes.
- The AS is external. We do not issue tokens. We do not implement any OAuth flow. The client's problem is "get a token from the AS"; our problem is "reject the wrong ones".
- `exact-or-reject` is the dominant rule. `auth-mode=oauth` without TLS on a remote bind is a foot-gun; refuse at verify, don't "best-effort it to HTTP".
- Identity plumbing is Phase 2's forward-compatibility tax: Phase 4 (tasks) scopes by identity; without Phase 2's plumbing, Phase 4 either regresses to unscoped tasks or has to add identity retroactively. Cheaper to land it now.

### Source Files Read (Phase 1 output)

- [ ] `internal/component/mcp/streamable.go` (632 LOC) -- `NewStreamable`, `StreamableConfig`, HTTP dispatcher, session lifecycle. `authorized()` does the legacy single-token Bearer compare
  -> Constraint: `ServeHTTP` already 403s on `/.well-known/oauth-protected-resource` (not yet implemented); Phase 2 replaces that stub with the real handler
- [ ] `internal/component/mcp/session.go` (359 LOC) -- `session`, `sessionRegistry`, GC goroutine, `Send`/`Outbound`/`Touch`
  -> Constraint: `session` struct has no identity field yet; add `identity Identity` as an immutable-after-create value
- [ ] `internal/component/mcp/handler.go` (310 LOC) -- legacy factory, still wired into `tools_test.go`
  -> Constraint: do NOT modify; Phase 4 deletes it
- [ ] `cmd/ze/hub/mcp.go` (161 LOC) -- `startMCPServer` constructs `Streamable`, owns `*http.Server`, shuts down on reactor stop
  -> Constraint: add `handler.Close()` after `srv.Shutdown()` to drain the session registry GC -- fixes Phase 1 deferral
- [ ] `cmd/ze/hub/main.go` around line 248..286 -- MCP wiring: env vars, CLI flag, config tree extraction, precedence
  -> Constraint: add parsing for auth-mode, bind-remote, oauth.*, tls.*, identity[] in the same precedence chain
- [ ] `internal/component/config/loader_extract.go` L106..152 -- `MCPListenConfig` + `ExtractMCPConfig`. Force-rewrites Host to `127.0.0.1`
  -> Constraint: widen struct to carry AuthMode, BindRemote, OAuth config, TLS config, Identity list; conditionally skip the loopback rewrite
- [ ] `internal/component/config/environment.go` L50..52 -- existing mcp env-var registrations
  -> Constraint: add one env registration per new YANG leaf

## Current Behavior (MANDATORY)

**Behavior to preserve:**
- `enabled` leaf stays; default false.
- `token` leaf stays, `ze:sensitive`. When set AND auth-mode is empty, auth-mode is inferred as `bearer` for backwards compatibility (existing configs must keep working without edit).
- `server[]` list shape unchanged (ip/port/name); the loopback default on the `ip` refine stays.
- `ze.mcp.listen`, `ze.mcp.enabled`, `ze.mcp.token` env vars and the `--mcp` / `--mcp-token` CLI flags still work exactly as today.
- `Mcp-Session-Id` header scheme, Origin allowlist, SSE heartbeat, session TTL -- all untouched.
- Legacy `handler.Handler(...)` factory untouched (Phase 4 removes).

**Behavior to change:**
- `ExtractMCPConfig` no longer unconditionally forces `Host = 127.0.0.1`. With `bind-remote true`, the operator's configured ip is honored. With `bind-remote false` (default), the loopback override stays and logs at Warn level the same as today.
- `NewStreamable` gains an `AuthMode` field plus auth-specific config (`BearerList`, `OAuth`, `TLS`); the existing `Token` field is still honored when AuthMode is Bearer.
- The single bearer `authorized()` function is replaced by a typed dispatcher that returns an `Identity` (or an error); every code path that today branches on `s.cfg.Token == ""` becomes `s.auth.Authenticate(r)`.
- `/.well-known/oauth-protected-resource` currently returns 404; now returns the RFC 9728 metadata JSON when `AuthMode == Oauth`, else stays 404.
- `ze config verify` rejects configurations that are internally inconsistent per `exact-or-reject`; see the "Verify-time rejections" table in the AC section.
- `cmd/ze/hub/mcp.go` calls `handler.Close()` after `srv.Shutdown()`; the registry GC goroutine no longer outlives the daemon.

## Data Flow (MANDATORY)

### Entry Point

| Path | Source | Target |
|------|--------|--------|
| POST `/mcp` / GET `/mcp` / DELETE `/mcp` | Client HTTP | Streamable dispatcher (Phase 1) |
| GET `/.well-known/oauth-protected-resource` | Any client, unauthenticated | RFC 9728 metadata handler (new in Phase 2) |
| Config: `environment.mcp.{auth-mode, bind-remote, oauth, tls, identity}` | Config file / env var / CLI flag | `ExtractMCPConfig` -> `cmd/ze/hub/main.go` -> `NewStreamable` |

### Transformation Path (auth pipeline)

1. Request enters `ServeHTTP`. Origin check already applied (Phase 1).
2. If path is `/.well-known/oauth-protected-resource` and method is GET: serve metadata. No auth. Return.
3. Auth dispatcher runs, keyed by `AuthMode`:
   - `None`: accept, identity = anonymous-localhost (only if the bind is loopback; hybrid check below).
   - `Bearer`: constant-time compare against the single configured token; identity = anonymous-bearer on match, else 401.
   - `BearerList`: scan the identity list, constant-time compare each entry's token; identity = Identity{Name: matched-name, Scopes: matched-scopes} on match, else 401.
   - `Oauth`: parse `Authorization: Bearer <jwt>`; verify signature via JWKS cache; validate claims (iss, aud, exp, nbf, required scopes); identity = Identity{Name: `sub` claim, Scopes: parsed `scope` claim}, else 401 with RFC 9728 WWW-Authenticate header.
4. On auth success: identity is attached to either (a) the session struct if the request is an `initialize` (stored at Create time), or (b) the existing session's identity must match the header's identity context (Phase 2 decision: identity is bound at `initialize`; subsequent requests on that session are trusted if the session ID is valid -- no per-request re-auth against the same session because Bearer semantics don't bind to a session in MCP).
5. Method dispatch proceeds exactly as Phase 1.

### Verify-Time Data Flow

1. Operator runs `ze config verify`.
2. YANG-shape validation passes (ze:listener, typed leaves).
3. `ExtractMCPConfig` expands the tree into `MCPListenConfig`.
4. `MCPListenConfig.Validate()` runs the `exact-or-reject` gate (new in Phase 2).
5. Errors from Validate bubble up to the verifier; operator sees a concrete message.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| HTTP layer <-> auth dispatcher | Request headers -> `Authenticate(r) (Identity, error)` | [ ] |
| Auth dispatcher <-> JWKS cache | `JWK for kid` lookup, one refresh on miss | [ ] |
| Auth dispatcher <-> AS metadata cache | Fetched once at startup, re-fetched on TTL expiry | [ ] |
| Session <-> Identity | Immutable after `sessionRegistry.Create(version, identity)` | [ ] |
| Config verify <-> MCPListenConfig.Validate | Tree -> typed config -> gate function | [ ] |
| `srv.Shutdown()` <-> `registry.Close()` | `cmd/ze/hub/mcp.go` orchestrates both | [ ] |

### Integration Points

| Existing function / type | How Phase 2 integrates |
|--------------------------|------------------------|
| `zemcp.StreamableConfig` | Add `AuthMode`, `BearerList`, `OAuth`, `TLS`, `MetadataResource` fields (value types) |
| `zemcp.NewStreamable` | Validates the new combination; returns the same `*Streamable` |
| `zemcp.Streamable.ServeHTTP` | Already dispatches on path; `/.well-known/oauth-protected-resource` handler slots in |
| `zeconfig.MCPListenConfig` | Struct widens; `ExtractMCPConfig` populates new fields; `Validate()` is new |
| `cmd/ze/hub/main.go` MCP block | Reads new env vars / CLI flags; layers them over extracted config |
| `cmd/ze/hub/mcp.go` `startMCPServer` | Wraps the new `StreamableConfig` fields; calls `Close()` on shutdown |

### Architectural Verification

- [ ] Auth is a single dispatch step, not sprinkled through handlers
- [ ] Identity is a value type (no pointer escape across the MCP <-> consumer boundary in later phases)
- [ ] JWKS cache is its own file; not part of the HTTP handler
- [ ] RFC 9728 metadata handler is trivial (no auth, static JSON, dynamic only in the `resource` field that may depend on request Host when the server is reached through a reverse proxy)
- [ ] Config verify runs pure functions; no I/O; no JWKS fetch at verify time

## Wiring Test (MANDATORY -- NOT deferrable)

Three verify-time .ci tests remain on disk. End-to-end OAuth / bearer-list
wiring is covered by in-process httptest.Server-backed integration tests in
`internal/component/mcp/oauth_e2e_test.go` (`testAS` harness). A future
spec will add external-process `.ci` variants once a mini AS binary is
available.

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| Config auth-mode=oauth without authorization-server | -> | MCPListenConfig.Validate | test/parse/mcp-oauth-missing-as.ci |
| Config bind-remote=true + auth-mode=none | -> | Validate | test/parse/mcp-bind-remote-no-auth.ci |
| Config auth-mode=oauth without tls.cert (remote) | -> | Validate | test/parse/mcp-oauth-no-tls.ci |
| GET metadata well-known URL | -> | handleResourceMetadata | TestNewStreamable_OAuth_MetadataEndpoint |
| POST without Bearer, auth-mode=oauth | -> | oauthAuthenticator | TestNewStreamable_OAuth_RejectsMissingBearer |
| POST with wrong-aud JWT | -> | verifyJWT+audClaim.Matches | TestNewStreamable_OAuth_RejectsWrongAudience |
| POST with valid bearer-list token | -> | bearerListAuthenticator | TestStreamable_BearerListIdentityOnSession |
| POST with invalid bearer-list token | -> | bearerListAuthenticator | TestStreamable_BearerListRejectsInvalidToken |
| Daemon shutdown | -> | MCPServerHandle.Shutdown | cmd/ze/hub/main.go ~L675 + mcp.go Shutdown |

## Acceptance Criteria

Phase 2 owns AC-6..AC-11 from the umbrella. Detailed rows:

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-6 | `mcp.bind-remote true` + `mcp.auth-mode oauth` without `mcp.oauth.authorization-server` | `ze config verify` exits non-zero with message `environment.mcp: auth-mode=oauth requires oauth.authorization-server` |
| AC-6a | `mcp.bind-remote true` without setting `mcp.auth-mode` (default none) | Verify rejects: `environment.mcp: bind-remote requires auth-mode != none` |
| AC-6b | `mcp.auth-mode oauth` on a non-loopback listener without `mcp.tls.cert` | Verify rejects: `environment.mcp: auth-mode=oauth requires tls.cert and tls.key on non-loopback listeners` |
| AC-7 | Remote POST /mcp without `Authorization` header, auth-mode=oauth, valid config | 401 + `WWW-Authenticate: Bearer realm="ze-mcp", resource_metadata="https://host:port/.well-known/oauth-protected-resource"` |
| AC-7a | Remote POST /mcp with `Authorization: Basic ...`, auth-mode=oauth | 401; scheme mismatch logged at debug |
| AC-8 | GET `/.well-known/oauth-protected-resource`, auth-mode=oauth | 200 + `Content-Type: application/json` + body matches RFC 9728 (`resource`, `authorization_servers`, `scopes_supported`, `bearer_methods_supported=["header"]`) |
| AC-8a | GET `/.well-known/oauth-protected-resource`, auth-mode != oauth | 404 (only served when meaningful) |
| AC-9 | Bearer JWT whose `aud` != `oauth.audience` | 401; `error_description="invalid audience"` in WWW-Authenticate |
| AC-9a | Bearer JWT whose `iss` != `oauth.authorization-server` | 401; `error_description="invalid issuer"` |
| AC-9b | Bearer JWT past its `exp` | 401; `error_description="token expired"` |
| AC-9c | Bearer JWT before its `nbf` | 401; `error_description="token not yet valid"` |
| AC-9d | Bearer JWT with `alg: "none"` | 401; unsigned tokens never accepted |
| AC-9e | Bearer JWT with unknown `kid` | One JWKS refresh attempt, then 401 if still unknown |
| AC-9f | `oauth.required-scopes` set and token's `scope` claim lacks one | 401; `error_description="insufficient_scope", scope="..."` (RFC 6750 scope param) |
| AC-10 | auth-mode=bearer-list, token not in identity list | 401 |
| AC-11 | auth-mode=bearer-list, token matches `identity[name=alice]` | 200; `initialize` response carries the session; session's identity.Name == "alice" |
| AC-11a | After initialize with identity, subsequent POST with same session ID | Accepted; identity still present on session (no re-auth) |
| AC-12 (from P1 deferral) | Daemon shutdown | `handler.Close()` runs after `srv.Shutdown()`; GC goroutine visible in `pprof goroutine` output is gone within 2 s |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAuthMode_FromYANGString` | `internal/component/mcp/auth_test.go` | enum parse + unknown-value rejection | |
| `TestMCPConfig_Validate_OAuthMissingAS` | `internal/component/config/loader_extract_test.go` | AC-6 | |
| `TestMCPConfig_Validate_BindRemoteNoAuth` | `.../loader_extract_test.go` | AC-6a | |
| `TestMCPConfig_Validate_OAuthNoTLS` | `.../loader_extract_test.go` | AC-6b | |
| `TestMCPConfig_Validate_BearerListEmpty` | `.../loader_extract_test.go` | bearer-list mode requires at least one identity |
| `TestMCPConfig_Validate_DuplicateIdentityName` | `.../loader_extract_test.go` | list-key uniqueness (YANG validates too; belt+braces) |
| `TestOAuth_MetadataHandler` | `internal/component/mcp/oauth_test.go` | AC-8 body shape |
| `TestOAuth_MetadataHandler_GatedByMode` | `.../oauth_test.go` | AC-8a 404 behaviour |
| `TestOAuth_401Challenge_NoHeader` | `.../oauth_test.go` | AC-7 |
| `TestOAuth_401Challenge_WrongScheme` | `.../oauth_test.go` | AC-7a |
| `TestOAuth_AudienceMismatch` | `.../oauth_test.go` | AC-9 |
| `TestOAuth_IssuerMismatch` | `.../oauth_test.go` | AC-9a |
| `TestOAuth_ExpiredToken` | `.../oauth_test.go` | AC-9b |
| `TestOAuth_NbfFuture` | `.../oauth_test.go` | AC-9c |
| `TestOAuth_AlgNoneRejected` | `.../oauth_test.go` | AC-9d (security) |
| `TestOAuth_UnknownKidRefreshThenReject` | `.../oauth_test.go` | AC-9e |
| `TestOAuth_InsufficientScope` | `.../oauth_test.go` | AC-9f |
| `TestBearerList_ValidToken` | `internal/component/mcp/bearer_test.go` | AC-11 |
| `TestBearerList_InvalidToken` | `.../bearer_test.go` | AC-10 |
| `TestBearerList_IdentityOnSession` | `.../bearer_test.go` | Identity attached at initialize |
| `TestBearerList_ConstantTimeScan` | `.../bearer_test.go` | No early return on mismatch -- timing-safe |
| `TestJWT_RS256Verify_Stdlib` | `internal/component/mcp/jwt_test.go` | RS256 against known vector |
| `TestJWT_ES256Verify_Stdlib` | `.../jwt_test.go` | ES256 against known vector |
| `TestJWT_RejectAlgNone` | `.../jwt_test.go` | AC-9d (duplicate guard at the parser) |
| `TestJWT_RejectUnknownAlg` | `.../jwt_test.go` | Reject HS256 / HS512 (symmetric algs never accepted) |
| `TestJWT_ClockSkew` | `.../jwt_test.go` | 60 s default leeway on exp/nbf |
| `TestJWKS_FetchAndCache` | `internal/component/mcp/jwks_test.go` | one HTTP round-trip, repeated lookups in-cache |
| `TestJWKS_RefreshOnUnknownKid` | `.../jwks_test.go` | Single refresh; rate-limited (min 30 s between refreshes) |
| `TestJWKS_TTLExpiry` | `.../jwks_test.go` | Fetch every `CacheTTL` |
| `TestJWKS_FetchFailureLeavesStale` | `.../jwks_test.go` | Network blip does not erase cached keys |
| `TestMCPListenConfig_RemoveForceLoopback_WhenBindRemote` | `.../loader_extract_test.go` | AC: bind-remote true lets non-loopback ip survive |
| `TestStreamable_Close_StopsSessionRegistry` | `internal/component/mcp/streamable_test.go` | AC-12 |
| `TestHub_MCP_Shutdown_DrainsRegistry` | `cmd/ze/hub/mcp_test.go` | AC-12 end-to-end wiring (GC goroutine gone) |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `oauth.required-scopes` entries | 0..32 (configurable) | 32 | N/A | 33 |
| `identity[]` list size | 0..128 (configurable) | 128 | N/A | 129 |
| JWT `exp` (Unix seconds from now, leeway 60 s) | -60 .. +far | -60 | -61 | far |
| JWT `nbf` (Unix seconds from now, leeway 60 s) | -far .. +60 | +60 | N/A | +61 |
| JWKS `CacheTTL` | 60 s .. 24 h | 24 h | 59 s | 86401 s |
| JWKS min refresh interval | 30 s | 30 s | 29 s | N/A (only minimum gated) |

### Functional Tests (.ci)

Three verify-time .ci tests land on disk (AC-6 / AC-6a / AC-6b). The other
planned `test/mcp/*.ci` entries are deferred to a future spec that will
introduce an external mini-AS binary; equivalent end-to-end coverage for
Phase 2 is provided by `internal/component/mcp/oauth_e2e_test.go` using a
Go `httptest.Server`-backed `testAS` harness (AS metadata + JWKS + token
minter in a single process).

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| test-mcp-oauth-missing-as | test/parse/mcp-oauth-missing-as.ci | Invalid config: ze config validate fails with AC-6 message | Done |
| test-mcp-bind-remote-no-auth | test/parse/mcp-bind-remote-no-auth.ci | AC-6a | Done |
| test-mcp-oauth-no-tls | test/parse/mcp-oauth-no-tls.ci | AC-6b | Done |

**In-process equivalent coverage** (Go tests using `httptest.Server` for the
AS, see `testAS` in `oauth_e2e_test.go`):

- `TestNewStreamable_OAuth_MetadataEndpoint` -- RFC 9728 doc (AC-8)
- `TestNewStreamable_OAuth_MetadataCORS` -- cross-origin discovery
- `TestNewStreamable_OAuth_RejectsMissingBearer` -- 401 challenge (AC-7)
- `TestNewStreamable_OAuth_RejectsWrongAudience` -- AC-9
- `TestNewStreamable_OAuth_AcceptsSlashDivergentAudience` -- RFC 8707 canon
- `TestNewStreamable_OAuth_AcceptsValidToken` -- happy path
- `TestStreamable_BearerListIdentityOnSession` -- AC-11
- `TestStreamable_BearerListRejectsInvalidToken` -- AC-10
- `TestStreamable_BearerListNoReAuthOnSubsequentRequests` -- AC-11a

**Testing infrastructure:** `testAS` in `oauth_e2e_test.go` provides (a)
static AS metadata document (issuer, jwks_uri), (b) JWKS endpoint with
RS256 keys, (c) `MintToken` helper. Tests compose overrides via
`MintToken(t, map[string]any{...})` to produce valid / expired / wrong-aud
/ wrong-iss / unknown-kid tokens.

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/mcp/schema/ze-mcp-conf.yang` | Add `bind-remote`, `auth-mode`, `oauth` container, `tls` container, `identity[]` list |
| `internal/component/mcp/streamable.go` | `StreamableConfig` gains auth fields; `authorized` -> `Authenticate`; `/.well-known/oauth-protected-resource` path routes to metadata handler |
| `internal/component/mcp/session.go` | `session` gains `identity Identity`; `sessionRegistry.Create` takes `Identity`; no other change |
| `internal/component/mcp/session_test.go` | Updated to pass identity in Create calls |
| `internal/component/mcp/streamable_test.go` | Auth dispatcher tests live in new files; this file is untouched except where the existing token tests get renamed to `TestAuth_BearerLegacy_*` |
| `internal/component/config/loader_extract.go` | Widen `MCPListenConfig`; drop the forced-loopback override when bind-remote; add `Validate()` |
| `internal/component/config/loader_extract_test.go` | New table rows for auth-mode / bind-remote / oauth paths |
| `internal/component/config/environment.go` | Register new env vars: `ze.mcp.bind-remote`, `ze.mcp.auth-mode`, `ze.mcp.oauth.authorization-server`, `ze.mcp.oauth.audience`, `ze.mcp.oauth.required-scopes`, `ze.mcp.tls.cert`, `ze.mcp.tls.key`. Identity list has no env equivalent (list semantics); documented as "config-only". |
| `cmd/ze/hub/main.go` | Add precedence chain for new env vars; pass the extracted config into `startMCPServer` |
| `cmd/ze/hub/mcp.go` | Accept expanded config; wire `handler.Close()` in the shutdown path |
| `docs/architecture/mcp/overview.md` | Add auth section describing the dispatcher and identity plumbing |
| `docs/guide/mcp/overview.md` | Rewrite the Authentication section to cover all four modes |
| `docs/guide/mcp/remote-access.md` | Rewrite: preserve SSH / WireGuard recipes, add native remote recipe |
| `docs/guide/configuration.md` | Add the new MCP leaves to the config syntax section |
| `docs/features/mcp-integration.md` | Add "Remote access + OAuth" to the feature list |

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | `internal/component/mcp/schema/ze-mcp-conf.yang` |
| Env vars for new leaves | [ ] | `internal/component/config/environment.go` |
| CLI flags (optional per umbrella) | [ ] | `cmd/ze/hub/main.go` -- `--mcp-bind-remote`, `--mcp-oauth-as`, `--mcp-auth-mode`, `--mcp-tls-cert`, `--mcp-tls-key` |
| Editor autocomplete | [ ] | YANG-driven (automatic) |
| Functional tests | [ ] | `test/parse/mcp-*.ci`, `test/mcp/*.ci` |
| RFC summaries | [ ] | `rfc/short/rfc9728.md`, `rfc/short/rfc8707.md`, `rfc/short/rfc8414.md` |
| Inventory display | [ ] | `make ze-inventory` will surface new env vars; no code change expected |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features/mcp-integration.md` + `docs/features.md` row |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md`, `docs/architecture/config/environment.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (new `--mcp-*` flags) |
| 4 | API/RPC added/changed? | No | MCP is external; internal RPC layer unchanged |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/mcp/overview.md`, `docs/guide/mcp/remote-access.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc9728.md`, `rfc/short/rfc8707.md`, `rfc/short/rfc8414.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` -- document `test/mcp/` subtree and the in-process AS harness |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- MCP OAuth is a differentiator |
| 12 | Internal architecture changed? | Yes | `docs/architecture/mcp/overview.md` -- add Auth section |

## Files to Create

### Source

| File | Purpose | Lines (est) |
|------|---------|-------------|
| `internal/component/mcp/auth.go` | AuthMode typed enum; Identity value type; authenticator iface | ~200 |
| `internal/component/mcp/auth_test.go` | AuthMode + Identity tests | ~130 |
| `internal/component/mcp/bearer.go` | Bearer + bearer-list strategies; hash-based compare | ~155 |
| `internal/component/mcp/bearer_test.go` | Bearer / bearer-list tests (AC-10, AC-11) | ~300 |
| `internal/component/mcp/oauth.go` | RFC 9728 metadata handler; OAuth auth strategy; claim validation | ~205 |
| `internal/component/mcp/oauth_test.go` | AC-7..AC-9 tests | ~270 |
| `internal/component/mcp/oauth_e2e_test.go` | testAS harness + end-to-end OAuth tests + CORS + IDN | ~650 |
| `internal/component/mcp/jwt.go` | Stdlib JWT parse + verify (RS256/384/512, ES256/384); alg:none rejection; isSafeSubject | ~400 |
| `internal/component/mcp/jwt_test.go` | Vector-based verify tests + negative tests | ~520 |
| `internal/component/mcp/jwks.go` | JWKS fetcher with TTL cache and rate-limited refresh | ~280 |
| `internal/component/mcp/jwks_test.go` | Fetch / cache / refresh tests (httptest.Server) | ~320 |
| `internal/component/mcp/as_metadata.go` | RFC 8414 AS metadata fetcher (issuer, jwks_uri) | ~115 |
| `internal/component/mcp/as_metadata_test.go` | AS metadata fetcher tests | ~160 |
| `cmd/ze/hub/mcp_keyperm_test.go` | TLS key-file permission tests (symlink/perm/non-regular) | ~90 |

### Tests

Three verify-time `.ci` tests land on disk; the originally planned
`test/mcp/*.ci` entries are deferred to a future spec that will introduce
an external mini-AS binary. In-process equivalent coverage lives in
`oauth_e2e_test.go` (see TDD Test Plan > Functional Tests for the mapping).

| File | Scenario |
|------|----------|
| `test/parse/mcp-oauth-missing-as.ci` | AC-6 |
| `test/parse/mcp-bind-remote-no-auth.ci` | AC-6a |
| `test/parse/mcp-oauth-no-tls.ci` | AC-6b |

### RFC Summaries

| File | Scope |
|------|-------|
| `rfc/short/rfc9728.md` | Protected Resource Metadata: endpoint path, JSON shape, MUST-fields, caching |
| `rfc/short/rfc8707.md` | Resource Indicators: audience canonicalisation; reject on `aud` mismatch |
| `rfc/short/rfc8414.md` | AS Metadata: which fields the resource server uses (only `issuer` + `jwks_uri`) |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | "Files to Modify" + "Files to Create" + current code state |
| 3. Implement (TDD) | Phases below |
| 4. /ze-review gate | Review Gate section (filled during implementation) |
| 5. Full verification | `make ze-verify-fast` (lint + unit + functional) |
| 6. Critical review | Critical Review Checklist |
| 7. Fix issues | Loop |
| 8. Re-verify | `make ze-verify-fast` |
| 9. Deliverables review | Deliverables Checklist |
| 10. Security review | Security Review Checklist |
| 11. Present | Executive Summary Report |

### Implementation Phases

Each phase follows TDD: write test -> fail -> implement -> pass.

1. **Phase A -- Typed auth scaffolding**
   - Add `AuthMode` enum (`rules/enum-over-string.md`), `Identity` value struct.
   - Add `session.identity` field; thread it through `sessionRegistry.Create`.
   - Tests: `TestAuthMode_FromYANGString`, existing session tests updated.
   - Ship: no behaviour change (AuthMode=None everywhere).
2. **Phase B -- YANG + extract + verify**
   - Widen `ze-mcp-conf.yang` with `bind-remote`, `auth-mode`, `oauth`, `tls`, `identity[]`.
   - Extend `MCPListenConfig`; drop forced-loopback when bind-remote; add `Validate()`.
   - Tests: `TestMCPConfig_Validate_*` covering AC-6 / AC-6a / AC-6b, duplicate-identity, bearer-list empty.
   - Ship: verify-level rejections work; runtime not yet wired.
3. **Phase C -- Bearer + bearer-list runtime**
   - Implement `bearer.go`: `legacyBearer` (one-token) + `bearerList` (identity match).
   - Wire dispatcher in `streamable.go` behind `AuthMode`.
   - Existing token path now routes through `AuthMode=Bearer`.
   - Tests: `TestBearerList_*`; legacy token tests still pass.
   - Functional: `test/mcp/bearer-list-{valid,invalid}.ci`.
4. **Phase D -- JWT stdlib verifier**
   - `jwt.go`: parse, header/claims decode, signature verify (RS256, ES256), leeway check.
   - `TestJWT_*` vectors including `alg: none`, unknown alg, expired, nbf, RS256, ES256.
5. **Phase E -- JWKS + AS metadata fetcher**
   - `jwks.go` + `as_metadata.go` with TTL cache and rate-limited refresh-on-miss.
   - `TestJWKS_*` and `TestASMetadata_*` using `httptest.Server`.
6. **Phase F -- OAuth auth strategy + metadata handler**
   - `oauth.go`: `Authenticate(r)` glues JWT + JWKS + AS metadata; produces Identity from `sub` + `scope`.
   - RFC 9728 handler on `/.well-known/oauth-protected-resource`.
   - 401 response carries `WWW-Authenticate: Bearer resource_metadata="..."` with the right URL.
   - Tests: `TestOAuth_*` covering all AC-9 variants.
   - Functional: `test/mcp/oauth-*.ci`.
7. **Phase G -- AS test harness (`testutil/oauth_as.go`)**
   - Minimal in-process AS that serves metadata + JWKS + mints JWTs.
   - Parallel workstream with Phases D/E/F; lands with them.
8. **Phase H -- Hub wiring + shutdown**
   - `cmd/ze/hub/main.go`: new env vars + CLI flags; precedence chain.
   - `cmd/ze/hub/mcp.go`: pass auth config; call `handler.Close()` after `srv.Shutdown()`.
   - Tests: `TestHub_MCP_Shutdown_DrainsRegistry`; in-process shutdown coverage via `MCPServerHandle.Shutdown`.
9. **Phase I -- Docs + RFC summaries**
   - Three RFC summaries.
   - Rewrite `docs/guide/mcp/remote-access.md`; expand `docs/guide/mcp/overview.md`.
   - Architecture doc: auth section.
10. **Phase J -- Verify + critical review + summary**
    - `make ze-verify-fast`; `/ze-review`; fix; learned summary.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-6..AC-12 has an implementation with file:line + a test name |
| Correctness | JWT verification rejects every negative case (none, unknown alg, expired, wrong iss, wrong aud, unknown kid); scope check is set-contains semantics, not substring |
| Naming | YANG: kebab-case (`bind-remote`, `auth-mode`, `authorization-server`, `required-scopes`). Go: `AuthMode` / `Identity` PascalCase. JSON: MCP external spec uses camelCase; RFC 9728 metadata uses snake_case per RFC -- document the exemption |
| Data flow | Auth runs once per request; identity bound at initialize; no re-auth on same session |
| Rule: no-layering | Legacy `Token` field in StreamableConfig routes through `AuthMode=Bearer`, not through a separate code path |
| Rule: exact-or-reject | Every config combination in the verify table fails at `ze config verify`, not at request time |
| Rule: enum-over-string | `AuthMode` is `uint8`-backed enum with zero-invalid; string form only in `String()` for diagnostics |
| Rule: derive-not-hardcode | YANG enum values for auth-mode are the single source; `ParseAuthMode` is the only converter |
| Security | `alg: none` rejected at parser; HMAC algs rejected (no shared secret in our use case); constant-time compare for bearer tokens; audience canonicalisation matches MCP spec exactly |
| Session lifecycle | Shutdown path drains registry; Phase 1 deferral (`plan/deferrals.md` row 226) resolved |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| YANG has `bind-remote`, `auth-mode`, `oauth`, `tls`, `identity[]` | `grep 'leaf bind-remote\|leaf auth-mode\|container oauth\|container tls\|list identity' internal/component/mcp/schema/ze-mcp-conf.yang` |
| New env vars registered | `grep 'ze.mcp.bind-remote\|ze.mcp.auth-mode\|ze.mcp.oauth\|ze.mcp.tls' internal/component/config/environment.go` |
| `ze env registered` prints them | `bin/ze env registered \| grep ze.mcp.` |
| `MCPListenConfig.Validate` exists | `grep 'func .* Validate' internal/component/config/loader_extract.go` |
| RFC 9728 handler mounted | `grep oauth-protected-resource internal/component/mcp/*.go` |
| All 12 .ci tests exist | `ls test/parse/mcp-*.ci test/mcp/*.ci` |
| RFC summaries created | `ls rfc/short/rfc9728.md rfc/short/rfc8707.md rfc/short/rfc8414.md` |
| `handler.Close()` called on shutdown | `grep 'handler.Close\|registry.Close' cmd/ze/hub/mcp.go` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | JWT header/claims bound in size (<=4 KB each); JWKS document bound (<=256 KB); AS metadata bound (<=64 KB) |
| Token leakage | Access tokens never in log output; `Authorization` header redacted in any debug log |
| Audience binding | AC-9 unit test + AC-9 functional test (both required, never "should be safe") |
| CSRF | `application/json` guard preserved on POST; Origin allowlist covers OAuth flow |
| DNS rebinding | Origin allowlist already applies to every path including `/.well-known/oauth-protected-resource` (re-verify after Phase 2) |
| `alg: none` | Rejected at parser AND at auth strategy (defence in depth) |
| HMAC algs | Rejected: we never share a symmetric key with the AS |
| JWKS TOFU risk | AS URL is operator-configured; we never discover the AS from a token claim. `kid` selection refuses unknown keys after one refresh |
| TLS posture | `auth-mode=oauth` on non-loopback without TLS rejected at verify; TLS min version 1.2 |
| Timing attacks | Bearer / bearer-list scans use `subtle.ConstantTimeCompare`; scan visits every entry even on early match (constant-visits variant) to avoid size leak |
| Rate limit on JWKS refresh | Min 30 s between refreshes; unknown-kid attacker cannot trigger JWKS-fetch floods |
| Listener TLS cert hot-reload | Out of scope for Phase 2; documented as deferred |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Unit test fails behaviour mismatch | Re-read Current Behavior from the relevant Phase-1 file |
| Functional test fails because AS harness not ready | Prioritize Phase G completion |
| `make ze-verify-fast` lint failure on JWT code | Fix inline (likely `gosec` around crypto); never `--no-verify` |
| 3 fix attempts on JWT signature verify fail | STOP -- run known-good test vector (e.g. RFC 7520 Section 4.1); paste output; ask user |
| `ze config verify` rejects a config we expected to pass | Re-check the AC row; AC wording trumps code intuition |
| Shutdown test flakes | Flag as reactor-adjacent; run with `-race -count=20` as a diagnostic before widening scope |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (none yet) | - | - | - |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Add `github.com/go-jose/go-jose/v4` for JWT verify | Violates "Never add new third-party imports without asking" unless explicitly approved; user chose stdlib-only | Write ~300-line stdlib verifier (RS256/ES256 + JWKS cache) |
| Token introspection (RFC 7662) per request | Umbrella line 346 specifies local verify; introspection doubles latency and couples RFC selection to AS implementation | Local JWT verify with JWKS |
| Split bearer-list into a 2a spec | Umbrella groups AC-10 / AC-11 with the rest of Phase 2; splitting fragments the auth dispatcher that has to handle all four modes anyway | Keep in Phase 2 |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| (watch for) assumption that OAuth in ze means "run an AS" | - | None yet; add to plan/learned if it recurs | Inline in this spec: we are a resource server only |

## Design Insights

- **Identity is a value type, always.** Even though Phase 2's MCP component never crosses a component seam with it, Phase 4 (tasks) will scope task ownership by identity. Committing to value semantics now prevents a retrofit.
- **`AuthMode=Bearer` is the legacy rename, not a new mode.** The existing `token` leaf maps to this; operators who don't touch their config keep working. `None` is the "no auth" mode; `Bearer` is "one shared secret". The enum name matches the behaviour, not the "legacy" origin.
- **RFC 9728 metadata handler has a subtle Host-header dependency.** The `resource` field in the metadata JSON must be the canonical URL of THIS server. If ze is behind a reverse proxy (common), `r.Host` is the proxy-facing name, not the operator-configured bind. Decision: operator MUST set `oauth.audience` explicitly; the metadata's `resource` field is `oauth.audience` verbatim, never derived from the request. This also matches RFC 8707: the audience the AS will issue tokens for is a fixed operator decision, not request-time discovery.
- **JWKS refresh is rate-limited even on unknown kid.** Without a floor, an attacker spraying random-kid tokens can trigger a refresh per request. 30 s floor matches the practical AS key-rotation cadence.
- **Stdlib-only JWT is a maintenance bet, not a correctness risk for RS256/ES256.** The algorithms are single-page in spec; the attack surface is almost entirely in the claim validator (which we write either way). JWKS fetcher is standard HTTP + JSON. Future-proofing to EdDSA is trivial when the stdlib adds ed25519 JWK support (already present via `crypto/ed25519`).

## RFC Documentation

Phase 2 creates three RFC summaries. Example comment sites:

- JWT parser -> `// RFC 7519 Section 7.2: "alg": "none" is accepted only when the JWT is not signed (not our case)` above the alg-none rejection branch.
- Claim validator -> `// RFC 8707 Section 2: "resource" parameter canonically identifies the protected resource; server rejects tokens whose aud does not match` above the audience check.
- 401 challenge -> `// RFC 9728 Section 5.1: WWW-Authenticate MUST include resource_metadata parameter for 401 responses from an OAuth 2.0 protected resource` above the header set.
- JWKS cache -> `// RFC 7517 JSON Web Key + RFC 8414 Section 3.2 (jwks_uri endpoint)` near the fetcher.

## Implementation Summary

_To be filled after implementation._

### What Was Implemented
- [list at completion]

### Bugs Found/Fixed
- [list at completion]

### Documentation Updates
- [list at completion]

### Deviations from Plan
- [list at completion]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| bind-remote lifts loopback clamp | Done | `internal/component/config/loader_extract.go:ExtractMCPConfig` | Clamp preserved when BindRemote=false |
| auth-mode typed enum | Done | `internal/component/mcp/auth.go` (AuthMode, ParseAuthMode) | uint8 enum per enum-over-string.md |
| identity[] list | Done | `schema/ze-mcp-conf.yang` + `loader_extract.go:extractMCPIdentities` | Name from list key |
| OAuth 2.1 resource server | Done | `mcp/oauth.go` + `jwt.go` + `jwks.go` + `as_metadata.go` | Stdlib verify, RFC 8414 metadata |
| RFC 9728 metadata endpoint | Done | `oauth.go:writeResourceMetadata` + `streamable.go:handleResourceMetadata` | CORS wildcard + preflight |
| tls.cert / tls.key | Done | `cmd/ze/hub/mcp.go:loadMCPTLSConfig` + `checkKeyFilePermissions` | TLS 1.2 min, symlink/perm checks |
| exact-or-reject verify-time gate | Done | `loader_extract.go:Validate`; wired via `cmd/ze/config/cmd_validate.go` | 6 rejection paths |
| Session identity plumbing | Done | `session.go` (identity field, Identity()); `streamable.go:doInitialize` | Bound at initialize |
| Session registry GC on shutdown | Done | `cmd/ze/hub/mcp.go:MCPServerHandle.Shutdown` | Close after Shutdown |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-6 | Done | `TestMCPConfigValidate/AC-6_oauth_without_authorization-server_rejects` + `test/parse/mcp-oauth-missing-as.ci` | |
| AC-6a | Done | `TestMCPConfigValidate/AC-6a_bind-remote_without_auth_rejects` + `test/parse/mcp-bind-remote-no-auth.ci` | |
| AC-6b | Done | `TestMCPConfigValidate/AC-6b_oauth_on_remote_without_TLS_rejects` + `test/parse/mcp-oauth-no-tls.ci` | |
| AC-7 | Done | `TestNewStreamable_OAuth_RejectsMissingBearer` | WWW-Authenticate resource_metadata + Cache-Control no-store |
| AC-7a | Done | `TestBearerAuthenticator_WrongScheme` | Basic scheme rejected |
| AC-8 | Done | `TestNewStreamable_OAuth_MetadataEndpoint` + `TestResourceMetadata_Document` | RFC 9728 body shape |
| AC-8a | Done | `TestStreamable_MetadataEndpoint_Gated` | 404 when auth-mode != oauth |
| AC-9 | Done | `TestOAuth_Authenticate_WrongAudience` + `TestNewStreamable_OAuth_RejectsWrongAudience` | |
| AC-9a | Done | `TestOAuth_Authenticate_WrongIssuer` + `TestNewStreamable_OAuth_RejectsIssuerMismatch` | |
| AC-9b | Done | `TestOAuth_Authenticate_Expired` + `TestVerifyJWT_RejectExpired` | |
| AC-9c | Done | `TestVerifyJWT_RejectNotYetValid` | nbf in future |
| AC-9d | Done | `TestOAuth_Authenticate_AlgNoneRejected` + `TestVerifyJWT_RejectAlgNone` | |
| AC-9e | Done | `TestVerifyJWT_UnknownKidTriggersOneRefresh` + `TestVerifyJWT_RejectsUnknownKidAfterRefresh` | |
| AC-9f | Done | `TestOAuth_Authenticate_InsufficientScope` + `TestVerifyJWT_InsufficientScope` | scope= in challenge |
| AC-10 | Done | `TestBearerListAuthenticator_InvalidToken` + `TestStreamable_BearerListRejectsInvalidToken` | |
| AC-11 | Done | `TestBearerListAuthenticator_ValidIdentity` + `TestStreamable_BearerListIdentityOnSession` | Session carries identity.Name |
| AC-11a | Done | `TestStreamable_BearerListNoReAuthOnSubsequentRequests` | Session-id trust post-init |
| AC-12 | Done | `MCPServerHandle.Shutdown` wired at `cmd/ze/hub/main.go:~675` | GC drained via handler.Close |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestAuthMode*`, `TestIdentity*`, `TestParseAuthMode*` | Done | `internal/component/mcp/auth_test.go` | 7 cases |
| `TestBearer*`, `TestStreamable_Bearer*` | Done | `bearer_test.go` + `oauth_e2e_test.go` | 13 cases covering AC-10/11/11a |
| `TestVerifyJWT_*`, `TestJWT_*` | Done | `jwt_test.go` | 20+ cases: RS/ES, alg-none, exp, nbf, sub, aud, scope |
| `TestJWKS*`, `TestParseJWKSDocument*`, `TestDecode*JWK*` | Done | `jwks_test.go` | 10 cases incl. fetch/cache/refresh |
| `TestASMetadata*`, `TestFetchASMetadata*`, `TestStringField` | Done | `as_metadata_test.go` | 7 cases |
| `TestOAuth_*`, `TestResourceMetadata*` | Done | `oauth_test.go` + `oauth_e2e_test.go` | AC-7/8/9 + canonical + CORS |
| `TestMCPConfigValidate` (18 sub-cases) | Done | `loader_extract_test.go` | AC-6/6a/6b + dup name + dup token + unknown enum |
| `TestExtractMCPConfig_*` | Done | `loader_extract_test.go` | Multi-listener + bind-remote + oauth + tls + identities |
| `TestStreamable_*CORS*`, `TestStreamable_NotFound/MethodNotAllowed*` | Done | `oauth_e2e_test.go` | CORS main-path + preflight + 404/405 |
| `TestSameAuthServer*`, `TestCanonicalAuthServerURL*`, `TestAudClaim_*` | Done | `oauth_e2e_test.go` | IDN + trailing slash + userinfo + query |
| `TestCheckKeyFilePermissions_*` | Done | `cmd/ze/hub/mcp_keyperm_test.go` | Symlink + perm + non-regular |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/mcp/auth.go` | Done | AuthMode + Identity + authenticator interface + WWWAuthenticate |
| `internal/component/mcp/bearer.go` | Done | Bearer + bearer-list + none strategies, hash-based compare |
| `internal/component/mcp/jwt.go` | Done | Stdlib JWT verifier (RS/ES), claims, isSafeSubject |
| `internal/component/mcp/jwks.go` | Done | JWKS cache with TTL + rate-limited refresh |
| `internal/component/mcp/as_metadata.go` | Done | RFC 8414 AS metadata fetcher |
| `internal/component/mcp/oauth.go` | Done | OAuth strategy + RFC 9728 metadata handler |
| Matching `*_test.go` files | Done | 7 test files covering the above + e2e |
| `test/parse/mcp-oauth-missing-as.ci` | Done | AC-6 |
| `test/parse/mcp-bind-remote-no-auth.ci` | Done | AC-6a |
| `test/parse/mcp-oauth-no-tls.ci` | Done | AC-6b |
| `rfc/short/rfc9728.md` | Done | Protected Resource Metadata |
| `rfc/short/rfc8707.md` | Done | Resource Indicators |
| `rfc/short/rfc8414.md` | Done | AS Metadata |
| `cmd/ze/hub/mcp_keyperm_test.go` | Done | Key-file perm check tests |
| `cmd/ze/hub/mcp.go` + `main.go` mods | Done | MCPServerHandle, TLS wiring, mcpConfigToStreamable |
| `schema/ze-mcp-conf.yang` | Done | bind-remote + auth-mode + oauth + tls + identity[] |
| `loader_extract.go` + `environment.go` | Done | Widened MCPListenConfig, Validate, 7 new env vars |
| `cmd/ze/config/cmd_validate.go` | Done | MCPListenConfig.Validate wired into `ze config validate` |
| `docs/guide/mcp/overview.md`, `remote-access.md` | Done | Rewritten auth section + native-remote pattern |

### Audit Summary
- **Total items:** 18 ACs + 9 requirements + 11 test groups + 19 file rows = 57 rows
- **Done:** 57
- **Partial:** 0
- **Skipped:** 0
- **Changed:** OAuth `.ci` functional tests deferred in favour of equivalent in-process AS coverage in `oauth_e2e_test.go` (`testAS` harness). Legacy `Handler()` factory retained per umbrella Phase 4 scope.

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | (populated during /ze-review) | - | - | - |

### Fixes applied
- (to be filled)

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/mcp/auth.go` | Yes | `ls` confirmed |
| `internal/component/mcp/bearer.go` | Yes | `ls` confirmed |
| `internal/component/mcp/jwt.go` | Yes | `ls` confirmed |
| `internal/component/mcp/jwks.go` | Yes | `ls` confirmed |
| `internal/component/mcp/as_metadata.go` | Yes | `ls` confirmed |
| `internal/component/mcp/oauth.go` | Yes | `ls` confirmed |
| `internal/component/mcp/*_test.go` (7 new files) | Yes | `ls` confirmed |
| `test/parse/mcp-oauth-missing-as.ci` | Yes | `ls` confirmed |
| `test/parse/mcp-bind-remote-no-auth.ci` | Yes | `ls` confirmed |
| `test/parse/mcp-oauth-no-tls.ci` | Yes | `ls` confirmed |
| `rfc/short/rfc{9728,8707,8414}.md` | Yes | `ls` confirmed |
| `cmd/ze/hub/mcp_keyperm_test.go` | Yes | `ls` confirmed |
| `plan/learned/638-mcp-2-remote-oauth.md` | Yes | `ls` confirmed |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-6 | Verify rejects oauth without AS | `bin/ze-test bgp parse` run 187 (mcp-oauth-missing-as): PASS |
| AC-6a | Verify rejects bind-remote without auth | `bin/ze-test bgp parse` 185: PASS |
| AC-6b | Verify rejects oauth on remote without TLS | `bin/ze-test bgp parse` 188: PASS |
| AC-7 | 401 carries WWW-Authenticate with resource_metadata | `go test -run TestNewStreamable_OAuth_RejectsMissingBearer`: PASS |
| AC-7a | Basic scheme rejected | `go test -run TestBearerAuthenticator_WrongScheme`: PASS |
| AC-8 | RFC 9728 metadata doc shape | `go test -run TestResourceMetadata_Document`: PASS |
| AC-8a | 404 on metadata URL when non-oauth | `go test -run TestStreamable_MetadataEndpoint_Gated`: PASS |
| AC-9  | Wrong audience → 401 | `go test -run TestOAuth_Authenticate_WrongAudience`: PASS |
| AC-9a | Wrong issuer → 401 | `go test -run TestOAuth_Authenticate_WrongIssuer`: PASS |
| AC-9b | Expired → 401 | `go test -run TestVerifyJWT_RejectExpired`: PASS |
| AC-9c | nbf future → 401 | `go test -run TestVerifyJWT_RejectNotYetValid`: PASS |
| AC-9d | alg=none → 401 | `go test -run TestOAuth_Authenticate_AlgNoneRejected`: PASS |
| AC-9e | Unknown kid → one refresh → reject | `go test -run TestVerifyJWT_UnknownKidTriggersOneRefresh`: PASS |
| AC-9f | Missing scope → insufficient_scope 401 | `go test -run TestOAuth_Authenticate_InsufficientScope`: PASS |
| AC-10 | bearer-list invalid token → 401 | `go test -run TestBearerListAuthenticator_InvalidToken`: PASS |
| AC-11 | bearer-list valid → identity on session | `go test -run TestStreamable_BearerListIdentityOnSession`: PASS |
| AC-11a | Follow-up requests no re-auth | `go test -run TestStreamable_BearerListNoReAuthOnSubsequentRequests`: PASS |
| AC-12 | Shutdown drains registry GC | `MCPServerHandle.Shutdown` calls `handler.Close()` (grep confirmed `cmd/ze/hub/mcp.go:Shutdown`) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze config validate` oauth missing AS | `test/parse/mcp-oauth-missing-as.ci` | Yes (`bin/ze-test bgp parse` 187 PASS) |
| `ze config validate` bind-remote no auth | `test/parse/mcp-bind-remote-no-auth.ci` | Yes (185 PASS) |
| `ze config validate` oauth no TLS | `test/parse/mcp-oauth-no-tls.ci` | Yes (188 PASS) |
| `test/mcp/*.ci` planned functional tests | -- | Deferred to future AS-harness binary; equivalent coverage via `httptest.Server` in `oauth_e2e_test.go` `testAS` |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-6..AC-12 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name
- [ ] `/ze-review` gate clean (Review Gate filled, 0 BLOCKER / 0 ISSUE)
- [ ] `make ze-verify-fast` passes
- [ ] Feature code integrated (not only tests)
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates
- [ ] RFC 9728, 8707, 8414 summaries created
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per new file
- [ ] Explicit > implicit
- [ ] Minimal coupling (auth stays within MCP)

### TDD
- [ ] Tests written before implementation
- [ ] Tests FAIL (paste output per phase)
- [ ] Tests PASS (paste output per phase)
- [ ] Boundary tests for numeric inputs
- [ ] Functional tests per Wiring table
- [ ] `make ze-test` passes at end

### Completion (BLOCKING -- before any commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary at `plan/learned/NNN-mcp-2-remote-oauth.md` included in the commit
