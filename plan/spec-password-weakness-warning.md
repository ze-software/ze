# Spec: password-weakness-warning

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
3. `internal/component/config/password_hash.go` - commit-time password hashing
4. `internal/plugins/passwd/main.go` - the `ze passwd` hashing helper
5. `ai/rules/config-surface.md` - warning vs rejection semantics

## Task

When an operator sets an account password, Ze bcrypt-hashes whatever plaintext it
is given and rejects only two cases: an empty password and a password over
bcrypt's 72-byte limit. It never warns that a password is weak (too short) or is a
well-known common password. Operators can silently configure trivially guessable
credentials.

Add a non-blocking weakness warning at password-set time, driven by a concrete,
embedded policy: a minimum length and a small embedded common-password denylist.
The warning is advisory (the commit still succeeds) so it never breaks existing
configs or automation, but it makes a weak choice visible.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/config-surface.md` - warning vs error semantics on commit.
  → Constraint: this is a warning, not a rejection; the password is still set.
- [ ] `ai/rules/plugin-self-containment.md` - the check must live with the password logic.
  → Constraint: one shared strength-check helper used by both the config-commit path and the `ze passwd` helper.

**Key insights:**
- The plaintext is in hand at both set sites *before* bcrypt hashing, so the check runs on the plaintext and never persists it.
- Warning-only keeps it safe to ship: no existing config becomes invalid.
- The policy is intentionally minimal and self-contained (length + a small embedded denylist), not a configurable policy engine.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/password_hash.go` - `hashPlaintextSibling` (password_hash.go:153-170) reads the plaintext sibling (:155-158) and bcrypt-hashes it (:159), rejecting only empty (:156-158 returns nil no-op) and too-long (:161-164). No strength check. `ApplyPasswordHashing` (:88) is the commit entry point.
- [ ] `internal/plugins/passwd/main.go` - `runImpl` (main.go:65-87) reads plaintext (:66), rejects empty (:71-74) and too-long (:77-79), then hashes (:75). No strength check.

**Behavior to preserve:**
- Every password that is accepted today is still accepted (warning-only, never a new rejection).
- Empty and >72-byte rejections stay as-is.
- The plaintext is never logged or persisted; only its hash is stored.
- Idempotent re-hashing of an already-hashed leaf is unchanged.

**Behavior to change:**
- Setting a plaintext password that is shorter than the minimum length, or that matches the embedded common-password denylist, emits a warning through the existing warning/error channel.

## Data Flow (MANDATORY)

### Entry Point
- Config commit: a `plaintext-<name>` sibling under a `ze:bcrypt` leaf (the canonical account-password path).
- CLI helper: plaintext read by `ze passwd` (`runImpl`).

### Transformation Path
1. Before bcrypt hashing, the plaintext is passed to a shared strength-check helper.
2. The helper returns a weakness reason if the plaintext is shorter than the minimum length or matches the embedded common-password denylist (case-insensitive, exact match).
3. If weak, a warning is emitted through the caller's existing channel (commit warnings for the config path; `errOut` for the `ze passwd` helper).
4. Hashing proceeds unchanged; the password is set regardless of the warning.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plaintext ↔ strength check | shared helper returns a weakness reason | [ ] |
| Check ↔ commit warnings | config path surfaces the warning on commit | [ ] |
| Check ↔ CLI helper | `ze passwd` surfaces the warning on stderr | [ ] |

### Integration Points
- New shared helper (e.g. under the config or a small auth util package) taking plaintext, returning an optional weakness reason.
- `internal/component/config/password_hash.go` - call the helper in `hashPlaintextSibling` before hashing; route the reason to commit warnings.
- `internal/plugins/passwd/main.go` - call the helper in `runImpl` before hashing; write the reason to `errOut`.

### Architectural Verification
- [ ] No bypassed layers (both set paths call the one helper)
- [ ] No unintended coupling (helper takes a string, returns a reason; no global state)
- [ ] No duplicated functionality (single denylist + length rule shared by both callers)
- [ ] Registration over hardcoding - the check is a shared helper invoked at the two password-set sites; no per-caller policy is duplicated into a core/shared package.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The config-commit path can surface a non-fatal warning | `password_hash.go` returns errors today; warnings channel exists on commit | warning is swallowed | trace the commit warning channel during audit | unvalidated |
| A-2 | Both password-set sites hold plaintext before hashing | password_hash.go:155-159, main.go:66-75 | a set path bypasses the check | grep all bcrypt set sites | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Warning treated as an error and blocks commit | commit fails on a weak password | keep the helper's return advisory; never convert to error |
| R-2 | Denylist bloat / maintenance | list grows unbounded | keep a small fixed embedded list (top common passwords) + length rule; not a dictionary |
| R-3 | Plaintext leaking into logs | plaintext in a warning string | warn with a generic reason, never echo the password |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| set account password to a short/common value | → | strength helper returns a reason; commit warns | `test/ci/password-weakness-warning.ci` |
| set account password to a strong value | → | no warning; commit clean | `test/ci/password-weakness-warning.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | password shorter than the minimum length | warning emitted; password still set |
| AC-2 | password matching the embedded denylist | warning emitted; password still set |
| AC-3 | password matching denylist (different case) | warning emitted (case-insensitive) |
| AC-4 | strong password (long, not in list) | no warning |
| AC-5 | empty password | still rejected (unchanged) |
| AC-6 | password over 72 bytes | still rejected (unchanged) |
| AC-7 | `ze passwd` with a weak value | warning on stderr; hash still printed |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | sets a trivially weak account password and sees a warning while the commit still succeeds | plaintext → strength helper → commit warning | `test/ci/password-weakness-warning.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPasswordStrengthShort` | `internal/component/config/password_strength_test.go` | short password returns a reason | |
| `TestPasswordStrengthDenylist` | `internal/component/config/password_strength_test.go` | denylisted value (any case) returns a reason | |
| `TestPasswordStrengthStrongNoReason` | `internal/component/config/password_strength_test.go` | strong password returns no reason | |
| `TestHashPlaintextWeakStillSets` | `internal/component/config/password_hash_test.go` | weak password warns but is still hashed/set | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| min length | length N | N (no warning) | N-1 (warning) | - |
| bcrypt length | 1..72 bytes | 72 | 0 (rejected) | 73 (rejected) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `password-weakness-warning` | `test/ci/password-weakness-warning.ci` | weak password warns yet sets; strong password is silent | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - local account-password UX; no protocol peer | - | - | not a protocol feature | - |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/config/password_hash.go` - call the strength helper in `hashPlaintextSibling`; route the reason to commit warnings
- `internal/plugins/passwd/main.go` - call the strength helper in `runImpl`; write the reason to `errOut`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Shared strength helper | [ ] yes | new `password_strength.go` (length + embedded denylist) |
| Functional test | [ ] yes | `test/ci/password-weakness-warning.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] no | - (no new config; advisory warning only) |

## Files to Create
- `internal/component/config/password_strength.go` - shared length + denylist helper
- `test/ci/password-weakness-warning.ci` - functional test
- (unit tests in new/existing `_test.go`)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - add the strength helper (returns a reason, unused) and a failing `test/ci/password-weakness-warning.ci`.
2. **Phase: Strength policy** - implement length + embedded denylist (case-insensitive).
   - Tests: `TestPasswordStrengthShort`, `TestPasswordStrengthDenylist`, `TestPasswordStrengthStrongNoReason`
3. **Phase: Wire both set paths** - call the helper in `hashPlaintextSibling` (commit warning) and `runImpl` (stderr); never block.
   - Tests: `TestHashPlaintextWeakStillSets`
4. **Functional test** - weak warns yet sets; strong is silent.
5. **Full verification** → `make ze-verify`
6. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | warning-only; empty/too-long rejections unchanged; plaintext never logged |
| Both paths | config-commit and `ze passwd` both call the one helper |
| Registration over hardcoding | single shared helper; no duplicated policy |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| strength helper | `go test ./internal/component/config -run Strength` |
| both set paths warn | `go test ./internal/component/config -run Weak && go test ./internal/plugins/passwd` |
| functional | `test/ci/password-weakness-warning.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| No plaintext leak | warning message never contains the password |
| No downgrade | never weakens the existing empty/too-long rejections |
| Advisory only | a weak password is never silently blocked or altered |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights
<!-- LIVE -->

## Implementation Summary
### What Was Implemented
- (fill during implementation)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
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
- [ ] Boundary tests for all numeric inputs (min length)
- [ ] Functional tests for end-to-end behavior
