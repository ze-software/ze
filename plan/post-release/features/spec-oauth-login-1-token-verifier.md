# Spec: oauth-login-1-token-verifier

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Post-release by Thomas's decision (umbrella Task). Moves a verifier MCP already ships and makes it fail closed by itself; MCP's shipped path is not fail-open today because its callers refuse an empty issuer and audience |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: plan/post-release/features/spec-oauth-login-0-umbrella.md (child 1).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make Ze's OAuth resource server, today private to MCP, one verifier every
surface can use, and make it safe to reuse.

The verifier (`verifyJWT`, the JWKS cache, the RFC 8414 metadata fetch) lives
in `internal/component/mcp`, behind the `ze_mcp` gate, so no other surface can
reach it without importing a gated package. This child moves it to one core
leaf package, deletes it from `mcp` in the same change, and has MCP import it.
It also adds one provider definition in config that every surface references,
a doctor check, a status command and metrics.

Why a child of its own: every other child consumes it, and it changes no login
behaviour. MCP's tests stay green unchanged, so it lands first and alone.

It owns the latent defect the old spec found: `verifyJWT` skips the issuer
check when the expected issuer is empty. Only its callers keep that out of
reach today: `buildOAuthAuthenticator` refuses an empty authorization server,
and `fetchASMetadata` refuses metadata with an empty `issuer` or one that
differs from the configured URL. A shared verifier whose callers multiply must
refuse it itself (AC-5).

VyOS defects this child carries (umbrella, "The VyOS reference"): `f75c7b74d`
(no audience accepted any token: AC-4, AC-5) and `d8da36885` (discovery with no
timeout: AC-6).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/core.md`, `ai/doors/api-surfaces.md`, `ai/doors/access-security.md`, `ai/doors/commands-cli.md`
  → Constraint: a core package has no config lifecycle and no registry side effect, and imports no component (`core.md`). The verifier library fits; the provider definition (config, JWKS refresh lifecycle, doctor check, status command) does not, so it lives in an always-on component
  → Constraint: always-on code never imports a gated surface; every surface is behind its own gate (`ze_mcp`, `ze_rest`, `ze_grpc`, `ze_gnmi`, `ze_web`, `ze_lg`, `ze_ssh`), so the verifier cannot live in any one of them
- [ ] `docs/architecture/mcp/overview.md` - the MCP auth modes, the RFC 9728 metadata route
  → Constraint: the MCP `oauth` mode is the existing resource server; this child moves it, never copies it (`ai/rules/no-layering.md`)
- [ ] `docs/architecture/api/commands.md`, "Which surfaces offer a command" - the status command reaches every surface through the registry

### RFC texts
- [ ] `rfc/full/rfc8414.txt`, `rfc/full/rfc8707.txt`, `rfc/full/rfc9728.txt` - in the `oauth-mgmt` profile; summaries exist
- [ ] `rfc/full/rfc7519.txt`, `rfc/full/rfc6750.txt` - texts present; no profile lists them, so no `rfc/short/` summary is written (`ai/META-DECISIONS.md`, "RFC validation scope does not grow"). Enrolling them in `oauth-mgmt` is Thomas's decision
  → Constraint: RFC 7519 Section 4.1.3: "If the principal processing the claim does not identify itself with a value in the "aud" claim when this claim is present, then the JWT MUST be rejected."

**Key insights:**
- MCP already fails closed on an empty audience (`audClaim.Matches`), but `verifyJWT` skips the issuer check when the expected issuer is empty; only its callers refuse that. A shared verifier must refuse it itself.
- Every surface is behind its own feature gate, so the shared verifier cannot live in any one of them.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/mcp/oauth.go` - `oauthAuthenticator.Authenticate` extracts the Bearer token, runs `verifyJWT` with issuer, audience and required scopes, maps each failure to an RFC 6750 challenge from a fixed vocabulary (`mapVerifyError`); `buildOAuthAuthenticator` refuses an empty audience or authorization server
- [ ] `internal/component/mcp/jwt.go` - `verifyJWT`: alg allow-list and explicit `none` refusal, key lookup with one refresh on an unknown `kid`, signature, `exp` required, `nbf`, 60 second default leeway, issuer (skipped when expected is empty), audience (`audClaim.Matches` refuses an empty expectation), `sub` required and free of control characters, scopes
- [ ] `internal/component/mcp/jwks.go` - `jwksCache`: 15 minute TTL, at most one refresh per 30 seconds, 256 KiB document cap, 5 second fetch timeout (`defaultJWKSFetchTimeout`)
- [ ] `internal/component/mcp/as_metadata.go` - `fetchASMetadata`: 5 second timeout (`defaultASMetadataTimeout`), 64 KiB cap, refuses an empty `issuer`, a missing `jwks_uri`, an issuer that differs from the configured URL, and a non-HTTPS `jwks_uri`
- [ ] `internal/component/mcp/streamable_auth.go` - `buildAuthForMode` fetches metadata, primes the JWKS cache and builds the authenticator once before the listener starts
- [ ] `internal/component/mcp/yang/ze-mcp-conf.yang` - `environment mcp`: `auth-mode` enum `none`, `bearer`, `bearer-list`, `oauth`; `oauth { authorization-server; audience; required-scopes; }`, both HTTPS-patterned; both leaves say a change "requires a restart"
- [ ] `internal/component/mcp/doctor.go` - `mcp-tls-material`, the MCP readiness check, registered by the component that owns it

**Behavior to preserve:**
- MCP `oauth` mode, its RFC 9728 metadata document and its challenge strings
- the existing MCP tests, `internal/component/mcp/oauth_e2e_test.go` above all, unchanged

**Behavior to change:**
- the verifier moves to one core leaf; MCP imports it
- the verifier refuses every token when its expected issuer or audience is empty
- one provider definition in config, referenced by each surface that accepts tokens; the MCP `oauth` block becomes a reference (config converted in the repository directly, no migration: `ai/META-DECISIONS.md`, "Ze is pre-release")
- a provider or audience change takes effect on reload, where MCP needs a restart today (AC-7)

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| MCP OAuth resource server | `internal/component/mcp` `oauthAuthenticator`, `verifyJWT`, `jwksCache`, `fetchASMetadata` | Verifies MCP access tokens | Replaces its location: moved to one core leaf package all surfaces import, deleted from `mcp` in the same change (`ai/rules/no-layering.md`); MCP then imports it and keeps only its challenge realm and RFC 9728 route |
| MCP `oauth` config block | `internal/component/mcp/yang/ze-mcp-conf.yang` | Issuer, audience, scopes for MCP only | Replaces: one provider definition referenced by every surface; the MCP block becomes a reference |
| Core leaf shape | `internal/core/ipsecinventory` (`core.md`, "Copy the core-leaf registry shape") | A standard-library-only leaf a component fills | Extends the pattern: the verifier library imports the standard library and core only (`jwks.go` already uses `internal/core/redact`) |
| Doctor check registry | `internal/component/mcp/doctor.go` `mcpTLSDoctorCheck`; `system.md`, "A readiness check" | Each component registers the check over what it owns | Extends: the provider definition's owner registers a check for discovery and JWKS reachability |
| Command registry | `docs/architecture/api/commands.md`; `command.AnswerSurface` | A command declared once reaches every surface | Extends: the status command (AC-9) registers once, with a YANG node, and reaches SSH, REST, gRPC, MCP and web |
| Metrics registry | `docs/plugin-development/metrics.md` | Named counters | Extends: token accept and refuse counts by reason |
| Reload transaction | `cmd/ze/hub/main_reload.go` `runReloadContext` | A failed reload leaves the accepted state serving | Extends: the provider definition joins the accepted generation (AC-7) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A surface passes a token string and the provider reference to the verifier. This child's only consumer is MCP.

### Transformation Path
1. The verifier checks signature, issuer, audience, expiry, subject and scopes against the configured provider.
2. It answers a verified subject and claim set, or a refusal from a fixed vocabulary.
3. Discovery and JWKS fetches run with a bounded timeout and refresh rate.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to identity provider | HTTPS, RFC 8414 metadata and JWKS fetch, bounded timeout | No |

### Changed-Wire Census
**Search evidence:** to be written at design. Wires: the `Authorization` header read by `oauthAuthenticator.Authenticate`; the RFC 9728 document (writer: `writeResourceMetadata`); the `environment mcp oauth` config block.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [to fill at design] | | | | |

### Integration Points
- `buildAuthForMode` (`streamable_auth.go`) - builds the MCP authenticator from the shared verifier

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
| A-1 | The MCP verifier holds no MCP-specific logic beyond its challenge realm | `oauth.go`, `jwt.go` read; note the `exp`-required comment cites "MCP OAuth 2.1", which every surface wants too | The move needs a seam for the realm | design read of `streamable_auth.go` | unvalidated |
| A-2 | The provider definition fits under an always-on component (candidate: `aaa`, beside `system authentication`) | `access-security.md`: `aaa` and `authz` are always on | it needs an always-on component of its own | design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The move changes an MCP challenge string | `oauth_e2e_test.go` red | the test stays unchanged; a red is a defect of the move |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | MCP admits a token it should refuse, or refuses every token |
| How is it reverted? | Single commit revert; config under the new provider container would then be refused |
| Who else touches this path? | `plan/release/fixes/spec-mcp-protected-resource-metadata-path.md`; children 3 to 6 consume it |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| MCP after the move | → | same verifier | existing `internal/component/mcp/oauth_e2e_test.go`, unchanged |
| Config with a provider and no audience | → | provider validation | `oauth-config-audience-required.ci` (planned, `test/parse/`) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A token with an expired `exp`, a wrong signature, `alg=none`, or an unknown `kid` | The verifier refuses it with a named reason from the fixed vocabulary; no infrastructure detail (URL, timeout, key type) in the reason a surface may show | None |
| AC-2 | The verifier's code after the move | One copy, in the core leaf; none under `internal/component/mcp`; MCP's existing tests pass unchanged | None |
| AC-4 | Config sets an OAuth provider with no audience, on any surface | Refused at verify and by `ze config validate`, naming the leaf and the command that sets it | None |
| AC-5 | The verifier is called with an empty expected audience or an empty expected issuer (config check bypassed) | Every token refused; the verifier fails closed itself and never relies on the config check alone | None |
| AC-6 | Discovery or JWKS endpoint accepts the TCP connection and never answers | Fetch ends within its bounded timeout; requests are refused with a generic message, the log names the endpoint | None |
| AC-7 | Reload changes the provider or audience | New requests use the new generation once the reload is accepted; a failed reload leaves the old one serving, as `runReloadContext` does today | None |
| AC-8 | Provider unreachable, or JWKS empty | `ze doctor` reports each with its cause and the command that fixes it | None |
| AC-9 | Operator runs the OAuth provider status command (name fixed at design) from SSH, REST, gRPC, MCP and web | Same answer on every surface: provider, discovery state, key count, last refresh | None |

AC-3 is unused here: AC-4 to AC-6 keep the old spec's numbers for traceability
(umbrella, "Where the old spec went").

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVerifierRefusesEmptyIssuer`, `TestVerifierRefusesEmptyAudience` | the shared package | AC-5 | |
| `TestDiscoveryTimeoutBounded` | the shared package | AC-6, with a server that accepts and never answers | |
| the moved `jwt_test.go`, `jwks_test.go`, `rfc8414_*_test.go` | the shared package | AC-1, AC-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| clock leeway, if made configurable | fixed at design | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `oauth-config-audience-required` | `test/parse/` | AC-4 | |
| status command on every surface | `test/plugin/` (name at design) | AC-9 | |

## Files to Modify
- `internal/component/mcp/oauth.go`, `jwt.go`, `jwks.go`, `as_metadata.go` - moved out; MCP keeps its challenge realm and RFC 9728 route
- `internal/component/mcp/streamable_auth.go`, `internal/component/mcp/yang/ze-mcp-conf.yang` - reference the shared provider
- `docs/architecture/mcp/overview.md`, `docs/guide/authentication.md`

## Files to Create
- a core leaf package for the verifier (name at design)
- the provider definition's YANG and its owner's doctor check and status command (placement at design, A-2)
- the functional tests above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | one provider definition referenced by each surface; shape at design |
| YANG validation constraints | Yes | audience mandatory when a provider is set |
| YANG custom validators | Yes | provider reference resolves |
| CLI commands/flags | Yes | the status command (AC-9) |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md` |
| Editor autocomplete | Yes | provider name completion |
| Functional test for new RPC/API | Yes | above |
| Pipe completeness | Yes | status command |
| Env var registration | N-A | no env override planned |
| Doctor check for runtime dependencies | Yes | AC-8 |
| Prometheus counters/metrics | Yes | token accept and refuse counts by reason |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed | Yes | `docs/guide/configuration.md` |
| 3 | CLI command added | Yes | `docs/guide/command-reference.md` |
| 6 | User guide | Yes | `docs/guide/authentication.md` |
| 12 | Architecture | Yes | `docs/architecture/mcp/overview.md` |
| 14 | Prometheus counters | Yes | `docs/plugin-development/metrics.md` |

## Implementation Steps
1. **Phase: Wiring** - move the verifier to its core leaf; MCP green unchanged
2. **Phase: Fail closed** - AC-5, AC-6
3. **Phase: Provider definition** - AC-4, AC-7
4. **Phase: Doctor, status command, metrics** - AC-8, AC-9

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail closed | Empty issuer or audience refuses everything (AC-5) |
| Resource exhaustion | Bounded fetch timeouts and refresh rate (AC-6) |
| Error leakage | Fixed vocabulary in replies, detail only in logs (AC-1) |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `zero-value-as-valid-answer.md` | 2026-11-08 | `verifyJWT` skips the issuer check on an empty expected issuer; AC-5 fixes it |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Move the MCP verifier to a core leaf | Copy it into each surface; import `mcp` from REST | Copying is a second declaration; importing one gated surface from another is forbidden |

## Known Limitations
- Certificate-bound tokens are child 6, `plan/post-release/features/spec-oauth-login-6-certificate-bound-tokens.md`.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le test functional parse`, `./le test functional plugin`, and the scoped package tests of the verifier and `mcp`
