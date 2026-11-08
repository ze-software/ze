# Spec: oauth-login-2-identity-mapping

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Post-release by Thomas's decision (umbrella Task). No shipped surface maps a token to a Ze identity today, so nothing shipped is fail-open without it |
| RFC gaps | - |
| Depends | `plan/post-release/features/spec-oauth-login-1-token-verifier.md`, `plan/release/fixes/spec-login-service-authorisation.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: plan/post-release/features/spec-oauth-login-0-umbrella.md (child 2).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A verified token proves who the identity provider says the caller is. It says
nothing about what Ze allows. This child turns a verified subject and claim set
into a Ze identity: a username and its authorization profiles, taken from the
accepted identity generation every credential surface already reads.

Two routes resolve an identity, and nothing else does:

1. The subject (or a configured claim such as `preferred_username`) names a
   local Ze user; the login runs as that user with that user's profiles.
2. A configured claim-to-profile rule (for example, a `groups` claim value
   mapped to an authz profile) grants profiles to a subject with no local
   account, the way a TACACS+ or RADIUS server grants profiles today.

A subject that matches neither is refused. A valid token never maps to a
default or full-privilege identity, which is the VyOS defect `f75c7b74d`
(umbrella, "The VyOS reference").

The result is the same `aaa.AuthResult` every other backend produces, with its
own `Source`, so the web session store, the SSH session and the API caller
carry it without a new carrier. Every surface child (3, 4, 5, 6) calls this one
mapping, which is why it is a child of its own: a mapping written per surface
would be four declarations of one trust decision.

Surfaces covered: all of them (REST, gRPC, gNMI, MCP, looking glass, web, SSH),
through the surface children.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md`, `ai/doors/hub-engine.md`, `ai/doors/api-surfaces.md`
  → Constraint: "Login is not an SSH feature. SSH, web, REST and gRPC read one accepted generation of users, and command authorization reads its policy from the same generation" (`access-security.md`)
  → Constraint: "Removing a user takes effect at once on every credential surface" (`access-security.md`, citing `docs/guide/operator-access-rbac.md`, section 8)
  → Constraint: something built only at startup ignores every reload (`hub-engine.md`, Traps); the mapping is part of the accepted generation, swapped atomically
- [ ] `docs/architecture/api/architecture.md`, "Identity and Authorization" - REST and gRPC auth precedence, reload staging
  → Constraint: identity is injected by the transport and is never accepted from the client payload
- [ ] `ai/digests/aaa-auth.md` - how a login reaches the chain and RBAC
- [ ] `plan/release/fixes/spec-login-service-authorisation.md` - the named set of doors an account may use

**Key insights:**
- `aaa.AuthorizerForResult` already binds a non-local result to its own profile list and strips the reserved recovery profile, and `resultAuthorizingAuthenticator` refuses a reserved username. A token result passes through the same two guards.
- `SessionStore.createSession` decides revocation from `AuthResult.GrantedByLocalBackend()` (`LocalAnchored`). A token mapped to a local user reports a source other than `local`, so the design must state how removing that local user revokes its token sessions (AC-2).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/aaa/types.go` - `AuthResult` (`Profiles`, `Source`, `LocalGeneration`, `Authorizer`), `GrantedByLocalBackend`, `AuthRequest`, `ChainAuthenticator`
- [ ] `internal/component/aaa/login_profiles.go` - `WithProfileAuthorizer` (reserved-name refusal), `AuthorizerForResult` (remote profiles bound to the result, recovery profile stripped)
- [ ] `cmd/ze/hub/aaa_lifecycle.go` - `newAcceptedLocalIdentity`, `liveAcceptedAPIAuthentication`, `stageAPIAuthentication` (fail closed while listeners migrate)
- [ ] `internal/component/web/auth.go` - `createSession` takes the `AuthResult` and records `LocalAnchored`; `validateToken` drops a session whose local user the running config no longer declares

**Behavior to preserve:**
- every existing backend's results and the chain order
- reserved-name refusal and recovery-profile stripping

**Behavior to change:**
- a verified token resolves to an `AuthResult` through a mapping in the accepted generation

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Accepted local identity generation | `cmd/ze/hub/aaa_lifecycle.go` `newAcceptedLocalIdentity`; `internal/component/aaa/login_profiles.go` `WithProfileAuthorizer` | One generation of users and profiles for REST and gRPC, swapped atomically on reload | Extends: a token subject resolves to a user of the same generation, and the claim rules are part of it |
| Backend-granted profiles | `internal/component/aaa/types.go` `AuthResult.Profiles`, `Source`; `tacacs/authenticator.go` | A remote backend grants profiles to a user with no local account | Extends: a claim-to-profile rule grants profiles the same way, with its own `Source` |
| Result-bound authorization | `internal/component/aaa/login_profiles.go` `AuthorizerForResult` | Binds a non-local result's profiles; strips the recovery profile | Extends: a token result uses it unchanged |
| Login-service authorisation | `plan/release/fixes/spec-login-service-authorisation.md` | A named set of doors an account may use | Extends: an OAuth login is a door; a user may be allowed it or not |
| Web session revocation | `internal/component/web/auth.go` `createSession`, `validateToken`, `LocalAnchored` | Removing a local user drops that user's sessions | Extends: a token-mapped local user is revoked the same way (AC-2) |
| Audit | `internal/core/audit` `Recorder` | Every surface records logins and commands | Extends: the audit record names the user and the OAuth door |
| Chain authenticator | `internal/component/aaa/types.go` `ChainAuthenticator` | Password logins walk TACACS+, RADIUS, local | Does not apply: a token is never a password and never enters the chain, so it never reaches a TACACS+ or RADIUS server |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A surface child hands the mapping a verified subject and claim set from child 1's verifier, plus the surface's own door name.

### Transformation Path
1. Read the accepted generation once, for the whole request.
2. Resolve the subject (or the configured claim) to a local user; otherwise apply the claim-to-profile rules; otherwise refuse.
3. Refuse a reserved username; apply the login-service gate for the door.
4. Answer an `AuthResult` with its `Source`, profiles and authorizer; the surface attaches it as per-user mode does today.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Surface to mapping | in-process call through an always-on seam the hub fills; no gated import | No |
| Surface to dispatcher | Caller identity in context, unchanged | No |

### Changed-Wire Census
**Search evidence:** to be written at design. Wires: the mapping config (shape at design); the audit record fields.

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
| No unintended coupling | No | to fill at design |
| No duplicated functionality | No | one mapping for every surface |
| Zero-copy preserved where applicable | No | N-A, not a hot path |
| Registration over hardcoding, outbound | No | to fill at design |
| Registration over hardcoding, inbound | No | to fill at design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every surface can resolve a subject against the accepted identity generation | `api/architecture.md`, "Identity and Authorization" | gNMI and web need their own lookup | `spec-login-identity-for-looking-glass-and-gnmi.md` lands first | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A token maps to full privileges by default, as VyOS did | A test with an unmapped subject passes | AC-1 refuses unmapped subjects |
| R-2 | A removed local user keeps working through an unexpired token | AC-2 red | the mapping reads the live generation per request |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A management surface admits a caller it should refuse, or with more profiles than allowed |
| How is it reverted? | Single commit revert; the mapping config would then be refused |
| Who else touches this path? | The two Depends specs; children 3, 4, 5, 6 |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| REST request with a valid token for an unmapped subject | → | the mapping from child 3's REST path | `rest-oauth-unmapped-refused.ci` (planned, `test/plugin/`) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Valid token whose subject maps to no Ze user and matches no configured claim-to-profile rule | 403, naming that the subject has no Ze account; never a default or full-privilege identity | None |
| AC-2 | Valid, unexpired token mapped to local user `alice`; `alice` is then removed by reload | The next request with that token is refused, on every surface, as a removed local user's password and session are today | None |
| AC-3 | Valid token for a user whose login-service set excludes OAuth | Refused as the login-service spec refuses any other door | None |
| AC-4 | Valid token mapped to `alice` | The result carries `alice`'s profiles and an authorizer bound to them; audit records `alice` and the OAuth door | None |
| AC-5 | Reload changes the mapping | New requests use the new generation once the reload is accepted; a failed reload leaves the old one serving, as `runReloadContext` does today | None |
| AC-6 | A token-mapped user missing from the config | `ze doctor` reports it with its cause and the command that fixes it | None |
| AC-7 | Token whose subject or claim maps to a reserved username, or whose claim rule names the reserved recovery profile | Refused, and the recovery profile is never granted, as `WithProfileAuthorizer` and `AuthorizerForResult` do for every backend | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUnmappedSubjectRefused` | `cmd/ze/hub` | AC-1 | |
| `TestTokenUserRemovedOnReload` | `cmd/ze/hub` | AC-2, AC-5 | |
| `TestTokenReservedNameRefused` | the mapping's package | AC-7 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rest-oauth-unmapped-refused` | `test/plugin/` | AC-1 | |
| `rest-oauth-login-service-refused` | `test/plugin/` | AC-3 | |

## Files to Modify
- `cmd/ze/hub/aaa_lifecycle.go` - subject resolution in the accepted generation
- `internal/component/aaa/types.go`, `internal/component/aaa/login_profiles.go` - the token `Source` and its result binding
- `internal/component/web/auth.go` - revocation of a token-mapped local user (AC-2)
- `docs/architecture/api/architecture.md`, "Identity and Authorization"; `docs/guide/authentication.md`, `docs/guide/authorization.md`

## Files to Create
- the mapping config YANG (placement at design, beside child 1's provider definition)
- the tests above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | subject claim choice and claim-to-profile rules; shape at design |
| YANG validation constraints | Yes | a rule names a declared profile |
| YANG custom validators | Yes | profile reference resolves |
| CLI commands/flags | N-A | child 1 owns the status command |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md` for the new config |
| Editor autocomplete | Yes | profile name completion |
| Functional test for new RPC/API | Yes | above |
| Pipe completeness | N-A | no command |
| Env var registration | N-A | no env override planned |
| Doctor check for runtime dependencies | Yes | AC-6 |
| Prometheus counters/metrics | N-A | child 1 counts accept and refuse |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed | Yes | `docs/guide/configuration.md` |
| 6 | User guide | Yes | `docs/guide/authentication.md`, `docs/guide/authorization.md` |
| 12 | Architecture | Yes | `docs/architecture/api/architecture.md`, "Identity and Authorization" |

## Implementation Steps
1. **Phase: Mapping** - AC-1, AC-4, AC-7
2. **Phase: Login-service gate** - AC-3
3. **Phase: Reload, revocation, doctor** - AC-2, AC-5, AC-6

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Privilege | No default identity for an unmapped subject (AC-1) |
| Revocation | Removing a user ends its token logins at once (AC-2) |
| Reserved identities | No token reaches a reserved name or the recovery profile (AC-7) |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| - | - | none |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Map subject to a Ze user | Grant a fixed identity to any valid token (VyOS) | A valid token proves who, not what Ze allows |
| Produce an `aaa.AuthResult` with its own `Source` | A token-specific identity type per surface | Every session and authorization path already carries an `AuthResult` |
| Never pass a token through `ChainAuthenticator` | Treat the token as a password | The chain sends the password to TACACS+ and RADIUS servers; a bearer token must not leave Ze |

## Known Limitations

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le test functional plugin`, and the scoped package tests of `aaa`, `cmd/ze/hub` and `web`
