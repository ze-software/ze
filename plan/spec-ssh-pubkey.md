# Spec: ssh-pubkey

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/4 |
| Updated | 2026-04-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/ssh/schema/ze-ssh-conf.yang` - current user schema
4. `internal/component/aaa/types.go` - UserCredential struct
5. `internal/component/authz/auth.go` - LocalAuthenticator
6. `internal/component/ssh/ssh.go` - SSH server, wish.WithPasswordAuth wiring
7. `internal/component/bgp/config/loader.go` - ExtractSSHConfig
8. `internal/component/bgp/config/infra_hook.go` - SSHExtractedConfig
9. `vendor/charm.land/wish/v2/options.go` - WithPublicKeyAuth
10. `vendor/github.com/charmbracelet/ssh/wrap.go` - ParseAuthorizedKey, KeysEqual

## Task

Add per-user SSH public key authentication to ze, following VyOS's config model. Users defined under `system.authentication.user` can have zero or more named SSH public keys. The wish SSH server accepts public key authentication alongside existing password authentication. Password authentication is preserved (users may have both password and keys, either works independently). The web UI continues to use passwords only; public keys are SSH-specific.

Scope boundary: YANG-configured users only. The zefs super-admin stays password-only (no blob store changes).

## Required Reading

### Architecture Docs
- [ ] `docs/guide/authentication.md` - current auth guide
  -> Decision: two user sources (zefs + YANG), merged at config load. Super-admin always password.
  -> Constraint: password leaf has ze:bcrypt extension, plaintext-password is ephemeral.
- [ ] `ai/patterns/config-option.md` - YANG leaf + registration pipeline
  -> Constraint: YANG types from ze-types.yang, env vars for environment/ leaves only. This feature is under system/, not environment/, so no env var needed.
- [ ] `ai/rules/config-design.md` - YANG structure rules
  -> Constraint: grouping+uses for shared structure within component, augment only cross-component.

### RFC Summaries (MUST for protocol work)
N/A - not protocol work.

**Key insights:**
- wish provides `WithPublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool)` alongside existing `WithPasswordAuth`
- `ssh.KeysEqual(ak, bk)` compares two parsed public keys
- `ssh.ParseAuthorizedKey(line)` parses a full authorized_keys-format line
- `golang.org/x/crypto/ssh.ParsePublicKey(wire)` parses wire-format key bytes
- The base64 data in an SSH public key decodes to wire format that `ParsePublicKey` accepts
- `UserCredential` in `aaa/types.go` currently has Name, Hash, Profiles
- `SSHExtractedConfig` in `infra_hook.go` carries `[]authz.UserConfig` as plain data
- `ExtractSSHConfig` in `loader.go` reads `system.authentication.user` entries from the config tree
- `infraSetup` in `infra_setup.go` merges zefs + config users, builds AAA bundle, wires both into `ssh.Config`
- The SSH server already stores users in `Config.Users` and passes them through the `Authenticator` interface
- Public key auth and password auth are independent wish options; both can be registered on the same server

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ssh/schema/ze-ssh-conf.yang` - defines `system.authentication.user` with `name`, `password`, `plaintext-password`, `profile` leaves. No public key leaves.
- [ ] `internal/component/aaa/types.go` - `UserCredential` struct: Name string, Hash string, Profiles []string. `BuildParams.LocalUsers` is `[]UserCredential`.
- [ ] `internal/component/authz/auth.go` - `LocalAuthenticator` wraps `[]UserConfig`, `Authenticate` checks username+password via bcrypt. `CheckPassword` supports hash-as-token and plaintext modes.
- [ ] `internal/component/ssh/ssh.go` - `Config` struct holds `Users []authz.UserConfig` and `Authenticator authz.Authenticator`. Server start registers `wish.WithPasswordAuth(...)` only. No public key handler.
- [ ] `internal/component/bgp/config/infra_hook.go` - `SSHExtractedConfig` carries Users, Listen, HostKeyPath etc. as plain data.
- [ ] `internal/component/bgp/config/loader.go` - `ExtractSSHConfig` iterates `system.authentication.user` list entries, reads `password` and `profile` leaves into `UserConfig`.
- [ ] `cmd/ze/hub/infra_setup.go` - `infraSetup` merges zefs users + config users, builds AAA bundle, creates `ssh.Config` with `Authenticator: bundle.Authenticator`, starts server.
- [ ] `vendor/charm.land/wish/v2/options.go` - `WithPublicKeyAuth(h ssh.PublicKeyHandler) ssh.Option` wraps `ssh.PublicKeyAuth(h)`.
- [ ] `vendor/github.com/charmbracelet/ssh/wrap.go` - `ParseAuthorizedKey`, `KeysEqual`, `ParsePublicKey` available.

**Behavior to preserve:**
- Password authentication works exactly as today for both zefs and YANG users
- Super-admin from zefs is password-only
- Hash-as-token mode for CLI tools unchanged
- Timing-safe authentication (bcrypt runs even for unknown users)
- Profile-based RBAC applies identically regardless of auth method (password or key)
- Web UI uses password authentication only (public keys are SSH-transport-specific)
- The `Authenticator` interface and chain remain unchanged; public key auth is a parallel path in the SSH server, not a new AAA backend

**Behavior to change:**
- YANG users can optionally have SSH public keys configured
- The wish SSH server accepts public key authentication when the connecting user has keys configured

## Data Flow (MANDATORY)

### Entry Point
- Config file: `system.authentication.user <name> { public-keys <id> { type ssh-ed25519; key AAAA...; } }`
- SSH connection: client offers public key during SSH handshake

### Transformation Path
1. Config parse: `ExtractSSHConfig` reads `public-keys` list entries per user, stores in `SSHExtractedConfig.Users`
2. Hub merge: `infraSetup` merges zefs users (no keys) + config users (with keys) into combined `[]UserCredential`
3. Server create: `ssh.Config.Users` carries the full user list including keys
4. Server start: registers `wish.WithPublicKeyAuth(...)` handler alongside existing password handler
5. SSH handshake: wish calls public key handler with `ctx.User()` and `ssh.PublicKey`
6. Key lookup: handler finds user by name, iterates their configured keys
7. Key compare: for each configured key, decode base64, parse, compare with `ssh.KeysEqual`
8. Auth result: if match found, log success with profiles, return true. Otherwise return false (wish falls through to password auth).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file -> SSHExtractedConfig | `ExtractSSHConfig` reads `public-keys` children | [ ] |
| SSHExtractedConfig -> ssh.Config.Users | `infraSetup` passes users through | [ ] |
| ssh.Config.Users -> wish handler closure | Handler closure captures user list or authenticator | [ ] |

### Integration Points
- `ExtractSSHConfig` in `loader.go` - extend to read `public-keys` children per user
- `UserCredential` in `aaa/types.go` - add PublicKeys field
- `ssh.Server.Start` in `ssh.go` - add `wish.WithPublicKeyAuth` option
- Profiles from the matched user flow through to RBAC exactly as with password auth

### Architectural Verification
- [ ] No bypassed layers (public key auth flows through same user lookup, same profile assignment)
- [ ] No unintended coupling (SSH server reads keys from user list, no new cross-component dependencies)
- [ ] No duplicated functionality (extends existing user model, does not create parallel user store)
- [ ] Zero-copy preserved where applicable (N/A, no wire encoding changes)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `public-keys` block | -> | `ExtractSSHConfig` populates `UserCredential.PublicKeys` | `TestExtractSSHConfigPublicKeys` |
| SSH client with key auth | -> | wish public key handler matches configured key | `TestSSHPublicKeyAuth` (unit) + `test-ssh-pubkey-auth.ci` (functional) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `system.authentication.user alice { public-keys laptop { type ssh-ed25519; key <base64>; } }` | `ExtractSSHConfig` returns `UserCredential` for alice with one PublicKey entry containing the type and key data |
| AC-2 | SSH client connects with a key matching alice's configured key | Public key handler returns true, SSH session established, user has alice's profiles |
| AC-3 | SSH client connects with a key NOT matching any configured key | Public key handler returns false, wish falls through to password auth |
| AC-4 | User has both password and public keys configured | Both authentication methods work independently |
| AC-5 | User has public keys but no password | Public key auth works, password auth fails |
| AC-6 | Zefs super-admin (no public keys in blob store) | Password auth works as before, public key auth has no keys to match |
| AC-7 | Config with user having multiple named public keys | All configured keys are accepted |
| AC-8 | YANG schema validates key type as enumeration | Only valid SSH key types accepted (ssh-rsa, ssh-ed25519, ecdsa-sha2-nistp256, ecdsa-sha2-nistp384, ecdsa-sha2-nistp521) |
| AC-9 | Auth success via public key | Log line includes username, remote addr, source, profiles (same format as password auth) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtractSSHConfigPublicKeys` | `internal/component/bgp/config/loader_test.go` | AC-1: config tree with public-keys parsed correctly into UserCredential.PublicKeys | |
| `TestExtractSSHConfigPublicKeysMultiple` | `internal/component/bgp/config/loader_test.go` | AC-7: user with multiple named keys | |
| `TestExtractSSHConfigPublicKeysEmpty` | `internal/component/bgp/config/loader_test.go` | User with no public-keys block has empty PublicKeys slice | |
| `TestPublicKeyMatch` | `internal/component/ssh/pubkey_test.go` | AC-2, AC-3: matching and non-matching key comparison | |
| `TestPublicKeyLookupMultipleKeys` | `internal/component/ssh/pubkey_test.go` | AC-7: multiple keys, any one matches | |
| `TestPublicKeyLookupUnknownUser` | `internal/component/ssh/pubkey_test.go` | AC-6: unknown user returns no match | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no numeric inputs in this feature.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-ssh-pubkey-auth` | `test/ssh/test-ssh-pubkey-auth.ci` | SSH client authenticates with configured public key | |

### Future (if deferring any tests)
- Zefs super-admin public key support (requires blob store schema change, out of scope)

## Files to Modify

- `internal/component/ssh/schema/ze-ssh-conf.yang` - add `list public-keys` under `list user`
- `internal/component/aaa/types.go` - add `PublicKey` struct and `PublicKeys` field to `UserCredential`
- `internal/component/bgp/config/loader.go` - extend `ExtractSSHConfig` to read public-keys entries
- `internal/component/bgp/config/infra_hook.go` - no change needed (`SSHExtractedConfig.Users` already carries `[]authz.UserConfig` which is `[]aaa.UserCredential`)
- `internal/component/ssh/ssh.go` - add `wish.WithPublicKeyAuth(...)` handler in server start
- `docs/guide/authentication.md` - document public key configuration

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new leaves) | Yes | `internal/component/ssh/schema/ze-ssh-conf.yang` |
| CLI commands/flags | No | YANG-driven config editor handles it automatically |
| Editor autocomplete | No | YANG-driven (automatic when schema updated) |
| Functional test | Yes | `test/ssh/test-ssh-pubkey-auth.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/authentication.md` - add SSH public key section |
| 2 | Config syntax changed? | Yes | `docs/guide/authentication.md` - show public-keys config example |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/authentication.md` - existing page, extend it |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Files to Create

- `internal/component/ssh/pubkey.go` - public key lookup helper (find user's keys, parse, compare)
- `internal/component/ssh/pubkey_test.go` - unit tests for key lookup
- `test/ssh/test-ssh-pubkey-auth.ci` - functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: YANG schema + data model** - add `list public-keys` to YANG, add `PublicKey` struct to `UserCredential`
   - Tests: `TestExtractSSHConfigPublicKeys`, `TestExtractSSHConfigPublicKeysMultiple`, `TestExtractSSHConfigPublicKeysEmpty`
   - Files: `ze-ssh-conf.yang`, `aaa/types.go`, `bgp/config/loader.go`, `bgp/config/loader_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: SSH public key handler** - add `pubkey.go` with lookup helper, wire `wish.WithPublicKeyAuth` into server start
   - Tests: `TestPublicKeyMatch`, `TestPublicKeyLookupMultipleKeys`, `TestPublicKeyLookupUnknownUser`
   - Files: `ssh/pubkey.go`, `ssh/pubkey_test.go`, `ssh/ssh.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Functional test** - SSH client authenticates with configured key
   - Tests: `test-ssh-pubkey-auth`
   - Files: `test/ssh/test-ssh-pubkey-auth.ci`
   - Verify: functional test passes end-to-end

4. **Phase: Documentation** - extend authentication guide with public key section
   - Files: `docs/guide/authentication.md`

5. **Full verification** -> `make ze-verify`

6. **Complete spec** -> fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Key comparison uses `ssh.KeysEqual`, not byte comparison. Base64 decoding handles padding correctly. |
| Naming | YANG leaves use kebab-case (`public-keys`, `key`, `type`). Go struct fields use PascalCase. |
| Data flow | Keys flow from YANG -> ExtractSSHConfig -> ssh.Config.Users -> wish handler. No new cross-package imports from aaa to ssh. |
| Rule: timing-safety | Public key auth does not introduce timing side channels (wish handles the crypto handshake; our callback just compares parsed keys) |
| Rule: profile-assignment | Profiles from matched user propagate to session context identically to password auth |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| YANG `list public-keys` with `name`, `type`, `key` leaves | `grep 'list public-keys' internal/component/ssh/schema/ze-ssh-conf.yang` |
| `PublicKey` struct in `aaa/types.go` | `grep 'type PublicKey struct' internal/component/aaa/types.go` |
| `PublicKeys` field in `UserCredential` | `grep 'PublicKeys' internal/component/aaa/types.go` |
| `ExtractSSHConfig` reads public-keys | `grep 'public-keys' internal/component/bgp/config/loader.go` |
| `wish.WithPublicKeyAuth` in ssh.go | `grep 'WithPublicKeyAuth' internal/component/ssh/ssh.go` |
| `pubkey.go` with lookup helper | `ls internal/component/ssh/pubkey.go` |
| Unit tests pass | `go test ./internal/component/ssh/ -run TestPublicKey` |
| Functional test exists | `ls test/ssh/test-ssh-pubkey-auth.ci` |
| Auth guide updated | `grep -i 'public.key' docs/guide/authentication.md` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Base64 key data must be validated at parse time (malformed base64 -> reject). YANG enumeration constrains key type. |
| Key parsing errors | `ssh.ParseAuthorizedKey` or `ssh.ParsePublicKey` failure must not leak error details to the SSH client (wish handles this; our callback returns bool only) |
| Timing | No timing side channel: `ssh.KeysEqual` uses `subtle.ConstantTimeCompare` internally via the Go SSH library. Unknown users return false with no observable timing difference from known-user-wrong-key. |
| Profile escalation | Public key auth must return the same profiles as password auth for the same user. No path to gain profiles beyond what the config assigns. |
| Key storage | Keys are stored in the config file (same security posture as passwords). No additional secret management needed (public keys are public). |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

N/A - not protocol work.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [pending]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/648-ssh-pubkey.md`
- [ ] Summary included in commit
