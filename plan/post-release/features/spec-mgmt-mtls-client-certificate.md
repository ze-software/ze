# Spec: mgmt-mtls-client-certificate

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | A new credential type for management surfaces that already authenticate; nothing shipped is fail-open without it |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator with a PKI wants automation (gnmic, Ansible, a controller) to log
in to gNMI, gRPC and the web UI with a client certificate instead of a shared
token. Today no management listener asks for a client certificate: gNMI and
gRPC build a server-only `tls.Config`, and the web UI serves the PKI leaf with
no client verification.

This spec adds mutual TLS to those listeners. The operator names one `pki ca`
entry per listener. Only a chain that ends at that exact CA certificate
authenticates, and the certificate's subject maps to a Ze user, whose profiles
then authorize each command.

VyOS reference: T8989 (`~/Code/github.com/vyos/vyos-1x`, `git log --grep=T8989`).
`0cdf7e696` added mTLS to the HTTPS API; two later fixes are the traps this
spec makes into criteria:

| VyOS commit | Defect | Ze criterion |
|-------------|--------|--------------|
| `71ea2f3d9` | The trust bundle held the configured CA's whole chain up to the root, so a sibling intermediate under the same root authenticated | AC-2 |
| `fa46c294c` | A client-supplied `X-Client-Verify: SUCCESS` header authenticated when mTLS was off | AC-4 |
| `0cdf7e696` itself | Every verified certificate became one identity, `mtls-client`, with full API rights | AC-5 |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/pki/tls-listeners.md` - how a listener serves a PKI leaf and chain, rotation, doctor
  → Constraint: a named certificate fails closed; the same rule holds for a named CA
- [ ] `docs/architecture/pki/pki-store.md` - the configured store versus the local CA
- [ ] `docs/architecture/api/architecture.md`, "Identity and Authorization"
  → Constraint: REST refuses every non-loopback address, so REST is out of scope; gRPC permits non-loopback only with auth and TLS
- [ ] `docs/architecture/fleet-config.md` - the managed client trusts one named CA

### RFC texts
- [ ] `rfc/full/rfc8705.txt` - Section 2.1, PKI mutual-TLS: "It relies on a validated certificate chain [RFC5280] and a single subject distinguished name (DN) or a single subject alternative name (SAN) to authenticate the client." No profile lists RFC 8705, so no `rfc/short/` summary is written; it guides the subject mapping only

**Key insights:**
- `managed.clientTLSConfig` already builds a pool from exactly one `pki ca` certificate (`pki.GetCA`), never the system pool and never the issuer chain, and refuses a name that resolves to nothing. That is the trust rule this spec needs, on the server side.
- `CACertEntry.CRLPEM` already carries the CA's revocation lists.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/managed/tls.go` - `clientTLSConfig`: one-CA `RootCAs` pool, `errCANotConfigured` on a missing name, TLS 1.3 minimum
- [ ] `internal/component/pki/store.go` - `GetCA` returns the named entry or nil; `CACertEntry.CRLPEM`
- [ ] `internal/component/gnmi/server.go` - `Server.Serve` builds `tls.Config{Certificates, MinVersion: TLS12}`, no `ClientAuth`; `checkAuth` checks one shared token
- [ ] `internal/component/api/grpc/server.go` - `NewGRPCServer` builds the same server-only config
- [ ] `internal/component/web/auth.go` - `AuthMiddlewareWithAudit`: session cookie or HTTP Basic; no certificate path
- [ ] `internal/component/api/rest/auth.go` - `withAuth` reads only `Authorization`

**Behavior to preserve:**
- Token and per-user logins keep working on a listener without mTLS
- A listener without a client CA never asks for a certificate

**Behavior to change:**
- gNMI, gRPC and web accept a client certificate verified against one named CA

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| One-CA trust pool | `internal/component/managed/tls.go` `clientTLSConfig` | Client side: trusts exactly one `pki ca` certificate | Extends: the same one-CA rule becomes a shared helper for `ClientCAs` on the server side; the managed client calls the helper too, so one rule has one spelling |
| PKI CA store | `internal/component/pki/store.go` `GetCA`, `CACertEntry.CRLPEM` | Named CA certificates and their CRLs | Extends: the listener names a CA; CRLs refuse a revoked client |
| Listener TLS material and rotation | `docs/architecture/pki/tls-listeners.md`; `cmd/ze/hub/listener_migrate.go` `updateWebCertificate` | Serves and rotates the server leaf | Extends: a changed client CA rotates the same way, with no rebind |
| EAP-TLS client chain verification | `internal/core/eap` `newTLSMethod`, `checkChainRevocation` | Verifies a client chain and its revocation for EAP-TLS | Extends where it fits: revocation checking is reused, not rewritten |
| Accepted local identity generation | `cmd/ze/hub/aaa_lifecycle.go` `newAcceptedLocalIdentity` | Users and profiles per generation | Extends: a certificate subject resolves to a user here |
| OAuth login specs | `plan/post-release/features/spec-oauth-login-0-umbrella.md`; bearer tokens in `spec-oauth-login-3-bearer-api-surfaces.md`, certificate-bound tokens in `spec-oauth-login-6-certificate-bound-tokens.md` | Bearer tokens on the same surfaces | Does not apply: separate credential; this spec's AC-4 and child 3's AC-3 state the same header rule. Child 6 depends on this spec |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The TLS handshake on the gNMI, gRPC or web listener

### Transformation Path
1. The listener's `tls.Config` sets `ClientCAs` to a pool holding only the named CA certificate, and `ClientAuth` to verify-if-given or require, by config
2. Go's TLS stack verifies the chain against that pool; Ze then checks the chain against the CA's CRLs
3. The leaf's subject (the configured choice of one SAN or the subject DN, RFC 8705 Section 2.1) resolves to a Ze user in the accepted generation
4. The surface attaches that user and its authorizer to the request context

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Client to listener | TLS 1.3 handshake with a client certificate | No |
| Surface to dispatcher | Caller identity in context, unchanged | No |

### Changed-Wire Census
**Search evidence:** to write at design. The TLS handshake gains a CertificateRequest; no application wire changes.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [to fill at design] | | | | |

### Integration Points
- `pki.GetCA`, the listener TLS builders, `newAcceptedLocalIdentity`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | to fill at design |
| No unintended coupling | No | the helper sits with `pki`, which every surface may import |
| No duplicated functionality | No | one-CA rule shared with the managed client |
| Zero-copy preserved where applicable | No | N-A |
| Registration over hardcoding, outbound | No | to fill at design |
| Registration over hardcoding, inbound | No | to fill at design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Go's verifier, given a pool with only an intermediate CA, accepts a chain that ends at it without its root | `crypto/x509` treats every pool certificate as a trust anchor | Operators must configure the root, widening trust | unit test with a two-level PKI | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A sibling intermediate under the same root authenticates | AC-2 test passes with a sibling | Pool holds the named certificate only, never its issuers |
| R-2 | Every certificate maps to one shared identity | AC-5 test | Subject maps to a user or the handshake's request is refused |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A management listener admits a certificate it should refuse |
| How is it reverted? | Single commit revert |
| Who else touches this path? | The OAuth login specs (`spec-oauth-login-3-bearer-api-surfaces.md`, `spec-oauth-login-6-certificate-bound-tokens.md`), the login-identity spec, PKI listener rotation |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| gnmic Get with a client certificate | → | gNMI `tls.Config` and subject mapping | `gnmi-mtls-login.ci` (planned, `test/plugin/`) |
| Browser with a client certificate | → | web listener | `web-mtls-login.wb` (planned, `test/web/`) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Listener names `pki ca ops`; client presents a leaf `ops` issued with SAN `alice` | Handshake completes; the command runs as `alice` with `alice`'s profiles; audit records the certificate door | None |
| AC-2 | `ops` is an intermediate under root `R`; client presents a leaf from sibling intermediate `other`, also under `R` | Handshake refused; the trust pool holds `ops` alone, never `R` and never the system pool | None |
| AC-3 | Listener names a CA that is not in the PKI store | Refused at verify and by `ze config validate`, naming the CA and the command that adds it; never a fallback to the system pool | None |
| AC-4 | Listener without mTLS; request carries `X-Client-Verify: SUCCESS` or `X-Client-DN` | 401; no header ever marks a client certificate as verified | None |
| AC-5 | Verified certificate whose subject maps to no Ze user | Refused, naming that the subject has no Ze account; never a shared full-rights identity | None |
| AC-6 | Certificate revoked by a CRL the CA entry carries | Refused, naming revocation | None |
| AC-7 | `verify-client` set to optional and no certificate sent | The other configured login modes still apply; with required, the handshake fails | None |
| AC-8 | Reload changes the client CA | New handshakes use the new pool with no rebind, as server-leaf rotation does | None |
| AC-9 | Client CA expires within the doctor window, or holds a CRL past its next-update | `ze doctor` reports it with the command that fixes it | None |
| AC-10 | Status command for the listener (name at design) from every surface | Shows client-auth mode and the named CA, the same everywhere | None |

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | gnmic with a client certificate | handshake → subject → user → config session | `gnmi-mtls-login.ci` |
| 2 | Browser login with a smart-card certificate | handshake → subject → web session | `web-mtls-login.wb` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestClientCAPoolHoldsNamedCAOnly` | the shared helper | AC-2 with a sibling intermediate | |
| `TestClientCAMissingRefused` | the shared helper | AC-3 | |
| `TestNoVerifiedStateFromHeaders` | web and gRPC auth tests | AC-4 | |
| `TestUnmappedCertificateSubjectRefused` | `cmd/ze/hub` | AC-5 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N-A: the feature takes no numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `gnmi-mtls-login` | `test/plugin/` | AC-1, AC-2, AC-5 | |
| `mtls-ca-missing` | `test/parse/` | AC-3 | |
| `web-mtls-login` | `test/web/` | AC-1 on web | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | | | TLS client authentication through Go's stack against gnmic in the functional test; no protocol peer | |

## Files to Modify
- `internal/component/managed/tls.go` - calls the shared one-CA helper
- `internal/component/gnmi/server.go`, `internal/component/api/grpc/server.go` - client CA, subject mapping
- `internal/component/web/auth.go` - certificate login path
- `cmd/ze/hub/aaa_lifecycle.go` - subject resolution
- `docs/architecture/pki/tls-listeners.md`, `docs/guide/authentication.md`

## Files to Create
- the shared one-CA helper beside `internal/component/pki/store.go`
- the functional tests above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | per listener: client CA name and `verify-client` (optional, required) |
| YANG validation constraints | Yes | enum for the mode |
| YANG custom validators | Yes | CA name resolves in the PKI store |
| CLI commands/flags | Yes | status (AC-10) |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md` |
| Editor autocomplete | Yes | CA name completion |
| Functional test for new RPC/API | Yes | above |
| Pipe completeness | Yes | status |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | AC-9 |
| Prometheus counters/metrics | Yes | client certificate refusals by reason |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature | Yes | `features/` entry |
| 2 | Config syntax changed | Yes | `docs/guide/configuration.md` |
| 6 | User guide | Yes | `docs/guide/authentication.md` |
| 12 | Architecture | Yes | `docs/architecture/pki/tls-listeners.md` |

## Implementation Steps
1. **Phase: Wiring** - shared one-CA helper; managed client switched to it
2. **Phase: gNMI and gRPC** - AC-1 to AC-7
3. **Phase: Web** - certificate login
4. **Phase: Reload, doctor, status** - AC-8 to AC-10

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Trust boundary | Pool holds the named CA only (AC-2) |
| Identity source | TLS session only, never a header (AC-4) |
| Privilege | No shared identity (AC-5) |
| Revocation | CRLs consulted (AC-6) |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One named CA certificate per listener, nothing else in the pool | The CA's chain to its root (VyOS before `71ea2f3d9`); the system pool | Either widens trust to every sibling or to the whole web PKI |

## Known Limitations
- REST stays loopback-only (`RESTServer.Reconfigure`), so it gains no mTLS here.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le test functional plugin`, `./le test functional web`, `./le test functional parse`, and the scoped package tests of `pki`, `managed`, `gnmi`, `api/grpc`, `web`
