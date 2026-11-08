# Spec: oauth-login-6-certificate-bound-tokens

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Post-release by Thomas's decision (umbrella Task). Hardens bearer tokens on surfaces that only gain them in child 3; nothing shipped is fail-open without it |
| RFC gaps | - |
| Depends | `plan/post-release/features/spec-oauth-login-3-bearer-api-surfaces.md`, `plan/post-release/features/spec-mgmt-mtls-client-certificate.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: plan/post-release/features/spec-oauth-login-0-umbrella.md (child 6).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A bearer token is usable by anyone who holds it. RFC 8705 Section 3 binds an
access token to the client certificate it was issued for: the token carries a
`cnf` claim with the certificate's SHA-256 thumbprint (`x5t#S256`), and the
resource server accepts it only over a TLS session whose client certificate
has that thumbprint.

The old spec listed this as a known limitation: "Certificate-bound access
tokens (RFC 8705 Section 3) need that spec first, and are not in this one",
where "that spec" is `plan/post-release/features/spec-mgmt-mtls-client-certificate.md`.
An in-scope item a spec does not do becomes its own spec (`ai/rules/planning.md`),
so it is this child.

Surfaces covered: the bearer surfaces on which the mTLS spec puts a client
certificate, which are gRPC and gNMI. REST stays loopback-only and the mTLS
spec leaves it out; MCP and the looking glass gain binding only if a later
spec gives them client certificates. The web UI uses a session cookie, not a
bearer token, so binding does not apply there.

Why a child of its own: it depends on two specs no other child needs (the
mTLS spec and child 3), and it changes the verifier's acceptance rule only for
tokens that carry `cnf`.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/api-surfaces.md`, `ai/doors/access-security.md`
  → Constraint: identity comes only from the transport and the TLS session, never from a header (child 3, AC-3)
- [ ] `docs/architecture/pki/tls-listeners.md` - listener TLS material
- [ ] `plan/post-release/features/spec-mgmt-mtls-client-certificate.md` - the client certificate the binding is checked against

### RFC texts
- [ ] `rfc/full/rfc8705.txt` - Section 3, mutual-TLS client certificate-bound access tokens; Section 3.1, the `x5t#S256` confirmation method. Text present; no profile lists it, so no `rfc/short/` summary is written (`ai/META-DECISIONS.md`, "RFC validation scope does not grow"). Enrolling it in `oauth-mgmt` is Thomas's decision
- [ ] `rfc/full/rfc8414.txt` - in the `oauth-mgmt` profile; the `tls_client_certificate_bound_access_tokens` metadata parameter RFC 8705 registers

**Key insights:**
- Child 1's verifier answers the claim set; the binding check needs the claim set and the TLS session's leaf certificate together, so it runs where the surface holds both.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/api/grpc/server.go` - `GRPCServer.checkAuth` reads the token from call metadata; the TLS peer is reachable from the same context
- [ ] `internal/component/gnmi/server.go` - `Server.checkAuth` reads the token from call metadata

**Behavior to preserve:**
- a token without `cnf` behaves exactly as child 3 accepts it

**Behavior to change:**
- a token with `cnf` `x5t#S256` is accepted only over a TLS session whose client certificate matches

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Bearer auth on gRPC and gNMI | `plan/post-release/features/spec-oauth-login-3-bearer-api-surfaces.md` | Verifies and maps a token from call metadata | Extends: adds the binding check before the mapping |
| Client certificates on listeners | `plan/post-release/features/spec-mgmt-mtls-client-certificate.md` | Verifies a client chain against one named CA | Depends: the certificate whose thumbprint the token must carry |
| Shared verifier | `plan/post-release/features/spec-oauth-login-1-token-verifier.md` | Answers a verified claim set | Extends: exposes `cnf` to the caller |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A gRPC or gNMI call over mutual TLS carrying a token in the `authorization` metadata

### Transformation Path
1. Child 1 verifies the token
2. If the token carries `cnf` `x5t#S256`, the surface compares it with the SHA-256 thumbprint of the session's client leaf certificate; no certificate or a mismatch refuses
3. Child 2 maps the identity

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Client to Ze | mutual TLS, then the token | No |

### Changed-Wire Census
**Search evidence:** to be written at design.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [to fill at design] | | | | |

### Integration Points
- child 3's `checkAuth` paths on gRPC and gNMI

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | to fill at design |
| No unintended coupling | No | to fill at design |
| No duplicated functionality | No | one thumbprint check shared by both surfaces |
| Zero-copy preserved where applicable | No | N-A, not a hot path |
| Registration over hardcoding, outbound | No | to fill at design |
| Registration over hardcoding, inbound | No | to fill at design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Keycloak issues certificate-bound tokens | vendor documentation | the interop scenario uses a stub issuer and the gap is named | design read of Keycloak's source | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A bound token is accepted over a plain TLS session because the check runs only when a certificate is present | AC-2 red | the check keys on the token's `cnf`, never on the session |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A stolen bound token works without its certificate |
| How is it reverted? | Single commit revert |
| Who else touches this path? | the mTLS spec, child 3 |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| gNMI call with a bound token over mTLS | → | binding check from `Server.checkAuth` | `gnmi-oauth-bound-token.ci` (planned, `test/plugin/`) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Token with `cnf` `x5t#S256` matching the session's client certificate | Accepted and mapped as child 3 does | None |
| AC-2 | Same token over a session with no client certificate, or with a different one | Refused (gRPC `Unauthenticated`) naming the binding failure from the fixed vocabulary | None |
| AC-3 | Token without `cnf` | Behaves exactly as in child 3 | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBoundTokenRefusedWithoutCertificate`, `TestBoundTokenRefusedWithOtherCertificate` | the binding check's package | AC-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `gnmi-oauth-bound-token`, `grpc-oauth-bound-token` | `test/plugin/` | AC-1, AC-2, AC-3 against a stub issuer | |

## Files to Modify
- `internal/component/api/grpc/server.go`, `internal/component/gnmi/server.go` - the binding check
- `docs/guide/authentication.md`, `docs/architecture/api/architecture.md`

## Files to Create
- the tests above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | binding follows the token's own claim; no config planned |
| YANG validation constraints | N-A | none |
| YANG custom validators | N-A | none |
| CLI commands/flags | N-A | none |
| CLI grammar (keyword before value) | N-A | none |
| Editor autocomplete | N-A | none |
| Functional test for new RPC/API | Yes | above |
| Pipe completeness | N-A | none |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | none |
| Prometheus counters/metrics | Yes | binding refusals counted under child 1's refuse reasons |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 6 | User guide | Yes | `docs/guide/authentication.md` |
| 12 | Architecture | Yes | `docs/architecture/api/architecture.md`, "Identity and Authorization" |

## Implementation Steps
1. **Phase: Binding check** - AC-1 to AC-3

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Binding | A `cnf` token is never accepted without its certificate (AC-2, R-1) |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| - | - | none |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Key the check on the token's `cnf` | A per-listener "require bound tokens" switch | RFC 8705 Section 3 makes the token say it is bound; a switch can be added at design if an operator needs to refuse unbound tokens |

## Known Limitations

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le test functional plugin`, and the scoped package tests of `api/grpc` and `gnmi`
