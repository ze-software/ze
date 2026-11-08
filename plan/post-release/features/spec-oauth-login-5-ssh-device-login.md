# Spec: oauth-login-5-ssh-device-login

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Post-release by Thomas's decision (umbrella Task). A further login on an SSH server that already authenticates by password and public key; nothing shipped is fail-open without it |
| RFC gaps | - |
| Depends | `plan/post-release/features/spec-oauth-login-1-token-verifier.md`, `plan/post-release/features/spec-oauth-login-2-identity-mapping.md`, `plan/post-release/features/spec-oauth-login-4-web-sign-in.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: plan/post-release/features/spec-oauth-login-0-umbrella.md (child 5).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator logs in to Ze's SSH server through the identity provider. SSH has
no bearer header and no browser, so the flow is the OAuth device authorization
grant (RFC 8628) run over SSH keyboard-interactive authentication:

1. The client offers keyboard-interactive. Ze asks the provider's device
   authorization endpoint for a device code and a user code.
2. Ze shows the verification URI and the user code in the keyboard-interactive
   prompt. The operator opens the URI on any device and signs in.
3. Ze polls the token endpoint at the interval the provider sets, until the
   provider answers with tokens, refuses, or the code expires.
4. The ID token is verified by child 1's verifier (audience equal to the client
   id), mapped by child 2, gated by the login-service set for `ssh`, and the
   session carries the result's authorizer, as a password login does.

Surfaces covered: SSH, and the CLI over SSH. A remote `ze cli` reaches the
daemon over SSH and inherits this login; the local `ze cli` on the box keeps
its loopback credential (`AuthRequest.Local`) and is unchanged.

Why a child of its own: it is a third OAuth flow (beside bearer tokens and the
browser code flow), it needs an SSH auth method Ze does not use today, and it
reuses child 4's client registration, so it follows child 4.

Why not accept a token typed as the SSH password: the password goes through
`ChainAuthenticator`, which hands it to TACACS+ and RADIUS servers. A bearer
token must never leave Ze that way, and an operator would still need another
tool to obtain the token.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/commands-cli.md`, `ai/doors/access-security.md`
  → Constraint: SSH can be compiled out (`ze_ssh`); the device flow lives in the SSH component and reaches the verifier and mapping through always-on seams
  → Constraint: "Login is not an SSH feature" (`access-security.md`): SSH resolves identity from the same accepted generation as every other surface
- [ ] `ai/digests/cli-editor.md` - the traced SSH session flow
- [ ] `ai/digests/aaa-auth.md` - how a login reaches the chain and RBAC
- [ ] `ai/META-DECISIONS.md`, "Code and dependencies"
  → Decision: per-use approval binds only an experimental module that implements cryptography (the Cloudflare `circl` module); "Every other module already in `go.mod`, `vishvananda/netlink` included, may be used in full." `charm.land/wish/v2` is neither experimental nor a cryptography module, so keyboard-interactive (`WithKeyboardInteractiveAuth`, present in `vendor/`) needs no approval

### RFC texts
- RFC 8628 (OAuth 2.0 Device Authorization Grant) has no text under `rfc/full/` yet. The design phase adds `rfc/full/rfc8628.txt` (allowed by `ai/META-DECISIONS.md`, "RFC validation scope does not grow", which permits adding a text) and writes no summary, because no profile lists it
- [ ] `rfc/full/rfc6749.txt` - text present; no profile lists it, so no summary
- OpenID Connect Core 1.0 (ID token validation) is not an RFC; cite it by section from the OpenID Foundation text

**Key insights:**
- The SSH server already sets the session authorizer from an `AuthResult` in its password and public-key callbacks (`setSessionAuthorizer`, `aaa.AuthorizerForResult`); a keyboard-interactive callback does the same with child 2's result.
- Each SSH login check runs inside `containAuth`, so a panic refuses the login; the device flow runs inside it too.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ssh/ssh.go` - `wish.WithPublicKeyAuth` and `wish.WithPasswordAuth`, each wrapped in `containAuth`, each setting the session authorizer from the result; no keyboard-interactive handler

**Behavior to preserve:**
- SSH password and public-key login, their audit and their login-service gate
- the local `ze cli` loopback credential

**Behavior to change:**
- SSH offers keyboard-interactive, running the device authorization grant, when an OAuth client registration is configured

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| SSH auth callbacks | `internal/component/ssh/ssh.go` `wish.WithPasswordAuth`, `wish.WithPublicKeyAuth`, `containAuth`, `setSessionAuthorizer` | SSH login, each method contained and bound to its result | Extends: a keyboard-interactive callback beside them, contained the same way |
| Password chain | `internal/component/aaa/types.go` `ChainAuthenticator` | Hands the password to TACACS+, RADIUS, local | Does not apply: the device flow never enters the chain |
| OAuth client registration | `plan/post-release/features/spec-oauth-login-4-web-sign-in.md` | Client id and secret for the web code flow | Extends: the device flow uses the same registration |
| Shared verifier | `plan/post-release/features/spec-oauth-login-1-token-verifier.md` | Verifies a JWT against the provider | Extends: verifies the ID token |
| Identity mapping | `plan/post-release/features/spec-oauth-login-2-identity-mapping.md` | Token to Ze user and profiles | Extends: maps the ID token, door `ssh` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An SSH client offers keyboard-interactive authentication

### Transformation Path
1. Ze requests a device code from the provider's device authorization endpoint
2. Ze prompts the verification URI and user code
3. Ze polls the token endpoint at the provider's interval, bounded by the code's expiry
4. Child 1 verifies the ID token; child 2 maps it; the session carries its authorizer

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| SSH client to Ze | keyboard-interactive prompts | No |
| Ze to identity provider | HTTPS device authorization and token endpoints, bounded timeout and polling | No |

### Changed-Wire Census
**Search evidence:** to be written at design. Wires: the SSH auth methods Ze advertises; the prompt text.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [to fill at design] | | | | |

### Integration Points
- `setSessionAuthorizer` - unchanged, fed by child 2's result

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | to fill at design |
| No unintended coupling | No | to fill at design |
| No duplicated functionality | No | one verifier, one mapping, one client registration |
| Zero-copy preserved where applicable | No | N-A, not a hot path |
| Registration over hardcoding, outbound | No | to fill at design |
| Registration over hardcoding, inbound | No | to fill at design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Common SSH clients (OpenSSH) show keyboard-interactive instructions and wait while the server polls | OpenSSH supports keyboard-interactive | the prompt needs a different shape | interop scenario | unvalidated |
| A-2 | Keycloak, Entra ID and Okta implement RFC 8628 | vendor documentation | the feature is limited to providers that do; doctor says so | design read of Keycloak's source | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An attacker starts many device flows to load the provider | rising poll counts | a bound on concurrent pending device flows, and the existing SSH session limit |
| R-2 | A phishing user code relayed to a victim (the RFC 8628 Section 5.4 remote phishing case) | operator report | the prompt names the Ze host; the design reads RFC 8628 Section 5 in full |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | SSH admits an operator it should refuse |
| How is it reverted? | Single commit revert; keyboard-interactive is no longer offered |
| Who else touches this path? | the login-service spec (door `ssh`) |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SSH keyboard-interactive login against a stub provider | → | device flow, verifier, mapping | `ssh-oauth-device-login.ci` (planned, `test/plugin/`) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | SSH client offers keyboard-interactive; the operator completes the provider login as a subject mapped to `alice` | Session runs as `alice` with `alice`'s profiles; audit records `alice` and the OAuth door | None |
| AC-2 | The provider answers `access_denied`, or the device code expires | Login refused with a message naming the cause; no session | None |
| AC-3 | The provider answers `slow_down` | Ze increases its poll interval as RFC 8628 Section 3.5 requires | None |
| AC-4 | The provider hangs | The poll ends at its bound; login refused; password and public-key logins keep working | None |
| AC-5 | A subject child 2 refuses, or a user whose login-service set excludes `ssh` | Refused as child 2 and the login-service spec refuse any other door | None |
| AC-6 | No OAuth client registration is configured | Keyboard-interactive is not offered; SSH behaves as today | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDeviceFlowSlowDown`, `TestDeviceFlowExpiry`, `TestDeviceFlowDenied` | `internal/component/ssh` | AC-2, AC-3, AC-4 | |
| `TestKeyboardInteractiveNotOfferedWithoutClient` | `internal/component/ssh` | AC-6 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| poll interval | provider-set, 5 seconds default (RFC 8628 Section 3.2) | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ssh-oauth-device-login` | `test/plugin/` | AC-1, AC-5 against a stub provider | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `oauth-keycloak` (extended) | `test/interop/scenarios/` | Keycloak, latest stable; OpenSSH client | A Keycloak device flow ends in a Ze SSH session for the mapped user | |

## Files to Modify
- `internal/component/ssh/ssh.go` - keyboard-interactive callback
- `docs/guide/authentication.md`, `ai/digests/cli-editor.md`

## Files to Create
- `rfc/full/rfc8628.txt` (text only, no summary)
- the device flow code in the SSH component (file at design)
- the tests above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | SSH references child 4's client registration; shape at design |
| YANG validation constraints | Yes | reference resolves |
| YANG custom validators | Yes | provider publishes a device authorization endpoint |
| CLI commands/flags | N-A | none |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md` for the new config |
| Editor autocomplete | Yes | client name completion |
| Functional test for new RPC/API | Yes | above |
| Pipe completeness | N-A | no command |
| Env var registration | N-A | no env override planned |
| Doctor check for runtime dependencies | Yes | device authorization endpoint reachable |
| Prometheus counters/metrics | Yes | device flows started, completed, refused |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature | Yes | `features/` entry |
| 2 | Config syntax changed | Yes | `docs/guide/configuration.md` |
| 6 | User guide | Yes | `docs/guide/authentication.md` |
| 14 | Prometheus counters | Yes | `docs/plugin-development/metrics.md` |

## Implementation Steps
1. **Phase: Design** - add the RFC 8628 text, read Section 5
2. **Phase: Device flow** - AC-2, AC-3, AC-4, AC-6
3. **Phase: Login** - AC-1, AC-5
4. **Phase: Interop** - `oauth-keycloak` extended

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Credential spread | The token never enters `ChainAuthenticator` |
| Resource exhaustion | Bounded polling and pending flows (R-1, AC-4) |
| Phishing | Prompt names the host (R-2) |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| - | - | none |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Device authorization grant over keyboard-interactive | A token typed as the SSH password; SSH certificates issued from the IdP | The password goes to TACACS+ and RADIUS through the chain; SSH certificates need a CA Ze does not have for SSH and a client-side tool |

## Known Limitations

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le test functional plugin`, `./le test integration` for `oauth-keycloak`, and the scoped package test of `ssh`
