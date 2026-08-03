# Spec: ssh-fido2-keys

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/ssh/pubkey.go` - key parse + match
4. `internal/component/ssh/ssh.go` - the wish public-key auth callback
5. `internal/component/ssh/yang/ze-ssh-conf.yang` - the key-type enum

## Task

Ze's SSH server is a native Go server (charmbracelet/wish over
`golang.org/x/crypto/ssh`), not an OpenSSH wrapper. Its configured public-key
type is restricted by a YANG enum to five classic algorithms (`ssh-rsa`,
`ssh-ed25519`, `ecdsa-sha2-nistp256/384/521`). Hardware-backed FIDO2 /
security-key public keys, whose OpenSSH type strings are
`sk-ssh-ed25519@openssh.com` and `sk-ecdsa-sha2-nistp256@openssh.com`, are
rejected at config-validation time because the enum has no value for them. An
operator therefore cannot register a security-key credential for SSH login.

Add security-key public-key authentication:
- Accept the two `sk-*` key types in the public-key config.
- Optionally enforce per-key policy: `touch-required` (user presence) and
  `verify-required` (user verification / PIN), the equivalent of OpenSSH's
  authorized_keys options for security keys.

The underlying `x/crypto/ssh` already parses `sk-*` keys and enforces the
user-presence (touch) flag during signature verification, so the missing pieces
are the config surface and the per-key policy applied at the auth callback.

## Required Reading

### Architecture Docs
- [ ] `internal/component/ssh/pubkey.go` - `parseConfiguredKey` reconstructs an `authorized_keys` line and calls `ssh.ParseAuthorizedKey`; `matchPublicKey` compares with `ssh.KeysEqual`.
  -> Constraint: `ParseAuthorizedKey` already accepts `sk-*` lines; the block is purely the YANG enum, so do not fork the parser.
- [ ] `internal/component/ssh/ssh.go` - the `wish.WithPublicKeyAuth` callback that returns a bool.
  -> Decision: per-key touch/verify policy is enforced here (or in `matchPublicKey`), the single authorization seam.
- [ ] `ai/rules/config.md`, `ai/rules/config.md` - YANG leaf naming.
  -> Constraint: new leaves are kebab-case under the existing `public-keys` list.
- [ ] `plan/learned/648-ssh-pubkey.md` - the original pubkey design; note its forward remark that FIDO keys would need no Go change was only half right (enum + policy are needed).

**Key insights:**
- The value of the feature is letting an operator log in with a hardware security key; touch enforcement comes from the crypto library for free once the key type is allowed.
- `verify-required` (PIN / user verification) enforcement depends on what the wish / `x/crypto` callback exposes about the presented signature's authenticator flags; this must be validated during design (see A-2).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ssh/pubkey.go` - `matchPublicKey` (pubkey.go) loops configured keys, calls `parseConfiguredKey` (pubkey.go) which builds `keyType + " " + keyData` and calls `ssh.ParseAuthorizedKey` (pubkey.go), then accepts on `ssh.KeysEqual` (pubkey.go). No key-type gating beyond what the enum already permits; no touch/verify policy.
- [ ] `internal/component/ssh/ssh.go` - `wish.WithPublicKeyAuth` (ssh.go) returns `true` when `matchPublicKey` returns non-nil profiles (ssh.go); no per-key policy is consulted.
- [ ] `internal/component/ssh/yang/ze-ssh-conf.yang` - the `type` leaf enum (yang lines 61-70) lists only `ssh-rsa`, `ssh-ed25519`, `ecdsa-sha2-nistp256`, `ecdsa-sha2-nistp384`, `ecdsa-sha2-nistp521`. No `sk-*` value.

**Behavior to preserve:**
- Every classic key that authenticates today still authenticates (the five enum values are unchanged).
- Password auth and the auth-failure recording path are unchanged.
- A parse error on any one configured key is logged and skipped, not fatal (pubkey.go).

**Behavior to change:**
- The key-type enum accepts the two `sk-*` types, so a security-key credential can be configured and matched.
- A configured key may carry `touch-required` / `verify-required` policy that the auth callback enforces on the presented security-key signature.

## Data Flow (MANDATORY)

### Entry Point
- Config: a `public-keys` entry whose `type` is `sk-ssh-ed25519@openssh.com` or `sk-ecdsa-sha2-nistp256@openssh.com`, plus optional boolean `touch-required` / `verify-required` leaves.
- Runtime: a client presents a security-key public key during the SSH public-key auth exchange.

### Transformation Path
1. Config parse loads the key entry (type + base64 data + policy flags) into the user's key list.
2. `parseConfiguredKey` reconstructs the authorized-keys line and `ssh.ParseAuthorizedKey` yields an `sk-*` `ssh.PublicKey`.
3. On a login attempt, `matchPublicKey` compares the presented key with the configured one via `ssh.KeysEqual`.
4. If matched, the auth callback applies the per-key policy: reject unless the presented signature satisfies the required user-presence / user-verification flags.
5. The callback returns accept/reject; the existing success-log / failure-record paths run unchanged.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree <-> key list | `sk-*` type + policy flags parsed into the user's public keys | [ ] |
| Key data <-> ssh.PublicKey | `ssh.ParseAuthorizedKey` on the `sk-*` line | [ ] |
| Presented signature <-> policy | auth callback checks touch/verify flags | [ ] |

### Integration Points
- `internal/component/ssh/yang/ze-ssh-conf.yang` - enum values + policy leaves.
- `internal/component/authz` - the `UserConfig` public-key struct gains policy fields (parsed from config).
- `internal/component/ssh/pubkey.go` / `ssh.go` - policy enforcement at the single auth seam.

### Architectural Verification
- [ ] No bypassed layers (auth still flows through the one wish callback)
- [ ] No unintended coupling (policy fields live with the key entry, not in a global)
- [ ] No duplicated functionality (reuse `ParseAuthorizedKey`; do not add an sk-specific parser)
- [ ] Registration over hardcoding - key algorithms remain enum-driven config, not a hardcoded switch in the auth path.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `ssh.ParseAuthorizedKey` accepts `sk-*` lines unchanged | pubkey.go; x/crypto sk key types exist | need a custom sk parser | parse a real `sk-*` key in a unit test | unvalidated |
| A-2 | The wish / `x/crypto` public-key callback exposes enough to enforce `verify-required` (UV) and to require touch beyond the library default | ssh.go callback signature | `verify-required` cannot be enforced server-side; scope to touch only | inspect the callback/permissions during design | unvalidated |
| A-3 | Touch (user-presence) is enforced by `x/crypto` during signature verify for `sk-*` keys | x/crypto sk key verify path | touch must be enforced manually | craft a no-touch signature test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `verify-required` is not enforceable through the current callback | design finds no UV flag exposed | ship `sk-*` + touch first; track UV as a follow-up rather than block |
| R-2 | Hardware-token functional testing needs a real authenticator | no CI FIDO2 device | unit-test enum/parse/policy with fixtures; use a virtual authenticator for the functional path; document any manual step |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| configure a `sk-ssh-ed25519@openssh.com` public key | -> | key parses and is matchable | `TestSKKeyConfiguredAndParsed` |
| a security-key login with the matching key | -> | auth callback accepts | `TestSKKeyAuthAccepts` |
| a `touch-required` key presented without user presence | -> | auth callback rejects | `TestSKKeyTouchRequiredRejects` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | configure a `sk-ssh-ed25519@openssh.com` key | config validates and the key is registered |
| AC-2 | configure a `sk-ecdsa-sha2-nistp256@openssh.com` key | config validates and the key is registered |
| AC-3 | login presenting a matching security key with touch | authentication succeeds |
| AC-4 | `touch-required` set, key presented without user presence | authentication fails |
| AC-5 | classic `ssh-ed25519` key (existing) | unchanged behaviour |
| AC-6 | malformed `sk-*` key data | parse error logged, entry skipped, other keys still work |
| AC-7 | `verify-required` set and satisfied | authentication succeeds (subject to A-2 outcome) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | registers a FIDO2 security key and logs in with a touch | config -> key list -> auth callback -> accept | `test/ci/ssh-fido2-keys.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSKKeyConfiguredAndParsed` | `internal/component/ssh/pubkey_test.go` | `sk-*` line parses to an `ssh.PublicKey` | |
| `TestSKKeyAuthAccepts` | `internal/component/ssh/pubkey_test.go` | matching security key authorizes | |
| `TestSKKeyTouchRequiredRejects` | `internal/component/ssh/pubkey_test.go` | no-presence signature rejected when `touch-required` | |
| `TestSSHKeyTypeEnumAcceptsSK` | `internal/component/ssh/config_test.go` | YANG enum accepts the two `sk-*` values | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| key type | enum set | `sk-ecdsa-sha2-nistp256@openssh.com` | n/a (non-enum rejected) | n/a |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ssh-fido2-keys` | `test/ci/ssh-fido2-keys.ci` | a configured security key authenticates; a wrong/absent-touch attempt fails | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| OpenSSH client with an `ed25519-sk` key logs into Ze | `test/interop/scenarios/ssh-fido2-openssh/` | OpenSSH client (`ssh` / `ssh-keygen -t ed25519-sk`) | Ze accepts a standard security-key credential from the reference client | |

### Future (if deferring any tests)
- If A-2 shows `verify-required` is not server-enforceable, phase it out with a documented limitation and keep `sk-*` + touch.

## Files to Modify
- `internal/component/ssh/yang/ze-ssh-conf.yang` - add the two `sk-*` enum values and optional `touch-required` / `verify-required` leaves
- `internal/component/authz` - the user public-key struct gains the policy fields
- `internal/component/ssh/pubkey.go` - carry policy through match; enforce touch/verify
- `internal/component/ssh/ssh.go` - apply per-key policy in the auth callback

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` |

## Files to Create
- `test/ci/ssh-fido2-keys.ci` - functional test
- `test/interop/scenarios/ssh-fido2-openssh/` - OpenSSH-client interop
- (unit tests in existing `_test.go`)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - add the enum values (unused) + a failing `test/ci/ssh-fido2-keys.ci`.
2. **Phase: Config surface** - enum values + policy leaves parsed into the key struct.
   - Tests: `TestSSHKeyTypeEnumAcceptsSK`, `TestSKKeyConfiguredAndParsed`
3. **Phase: Auth + policy** - accept `sk-*` in match; enforce touch/verify at the callback.
   - Tests: `TestSKKeyAuthAccepts`, `TestSKKeyTouchRequiredRejects`
4. **Functional + interop** - CI security-key login; OpenSSH-client interop.
5. **Full verification** -> `make ze-verify`
6. **Complete spec** -> audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | classic keys unchanged; touch enforced; parse errors non-fatal |
| Registration over hardcoding | algorithms stay enum-driven, no hardcoded key switch |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights
<!-- LIVE -->

## Implementation Summary
### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 demonstrated (AC-7 subject to A-2)
- [ ] End-to-End User Stories: working path + passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop test with the OpenSSH client
