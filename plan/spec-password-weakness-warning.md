# Spec: password-weakness-warning

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

Anchor refresh (2026-07-22 plan review, design HOLDS against the landed bcrypt
work, learned 1181): the R-4 risk materialized benignly -- 1181 touched the
same three commit sites and wired `RejectMaskedBcryptLeaves` there, shifting
anchors ~6-9 lines (citations below updated in-body): `editor_commit.go`
152 -> 158 and 312 -> 321; `MigrationWarning` build 190 -> 196 and
330 -> 339. `ApplyPasswordHashing` is
still error-only (`password_hash.go`) and `hashPlaintextSibling`'s empty
no-op is preserved, so AC-5 holds. Rebase the helper wiring onto
the current commit-site lines per R-4's stated mitigation.

**Notes:** Promoted to ready per user instruction 2026-07-10 (followup-wave impact review session) authorizing conversion to ready.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/config/password_hash.go` - commit-time password hashing
4. `internal/plugins/passwd/main.go` - the `ze passwd` hashing helper
5. `ai/rules/config.md` - warning vs rejection semantics

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

### Proposed Policy Defaults (owner-adjustable, NOT final)

Both values below are proposals the owner may change before or during
implementation; they exist so boundary tests have concrete numbers, not a
placeholder "N". Neither is a final decision.

| Policy | Proposed default | Adjustable? |
|--------|------------------|-------------|
| Minimum length | 8 characters (a password of length < 8 warns) | yes -- owner may raise/lower |
| Embedded denylist | a small fixed list of the most common weak passwords, e.g. `password`, `123456`, `12345678`, `qwerty`, `admin`, `letmein`, `root`, `changeme` (case-insensitive exact match) | yes -- owner may edit the entries; stays a short embedded list, never a dictionary (R-2) |

## Required Reading

### Architecture Docs
- [ ] `ai/rules/config.md` - warning vs error semantics on commit.
  → Constraint: this is a warning, not a rejection; the password is still set.
- [ ] `ai/rules/plugins.md` - the check must live with the password logic.
  → Constraint: one shared strength-check helper used by both the config-commit path and the `ze passwd` helper.

**Key insights:**
- The plaintext is in hand at both set sites *before* bcrypt hashing, so the check runs on the plaintext and never persists it.
- Warning-only keeps it safe to ship: no existing config becomes invalid.
- The policy is intentionally minimal and self-contained (length + a small embedded denylist), not a configurable policy engine.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/password_hash.go` - `hashPlaintextSibling` (password_hash.go) reads the plaintext sibling (:155-158) and bcrypt-hashes it (:159), rejecting only empty (:156-158 returns nil no-op) and too-long (:161-164). No strength check. `ApplyPasswordHashing` (:88) is the commit entry point.
- [ ] `internal/plugins/passwd/main.go` - `runImpl` (main.go) reads plaintext (:66), rejects empty (:71-74) and too-long (:77-79), then hashes (:75). No strength check.

### Post-wave corrections (2026-07-10)

All refs re-verified against current code: NO drift. `ApplyPasswordHashing`
(password_hash.go), `hashPlaintextSibling` (:153-170 with empty no-op
:156-158, hash :159, too-long :161-164) and `runImpl` (main.go, empty
:71-74, hash :75, too-long :77-79) all match the citations above exactly.

Additional evidence strengthening A-1 (warning channel): the SAME file already
produces advisory warnings on this exact surface -- `CheckBcryptLeaves`
(password_hash.go) returns warning strings for a non-bcrypt canonical
leaf value, and the functional test `test/parse/user-plaintext-warning.ci`
proves those warnings surface through `ze config validate`. The weakness
warning rides an existing, tested channel. A-1 keeps its validation method
(trace the exact routing during the implement audit) but its basis is now
grounded in a producer citation.

Functional test location corrected everywhere in this spec: `test/ci/` does
not exist. Password-hashing functional tests live in `test/parse/`
(user-plaintext-warning.ci, user-plaintext-password.ci, passwd-helper.ci), so
the new test is `test/parse/password-weakness-warning.ci`.

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
3. If weak, a warning is surfaced through the caller's existing warning surface. Concretely (see Key Design Decisions): the config path threads the reason out of `ApplyPasswordHashing`/`hashPlaintextSibling` into the commit result's warning field (`CommitResult.MigrationWarning`, `internal/component/cli/contract/contract.go`) at the two `internal/component/cli/editor_commit.go` sites (:158/:196, :321/:339) and the `commitContent()` site (`internal/component/cli/editor_commands.go`); the `ze passwd` helper writes the reason to `errOut`.
4. Hashing proceeds unchanged; the password is set regardless of the warning.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plaintext ↔ strength check | shared helper returns a weakness reason | [ ] |
| Check ↔ commit warnings | config path surfaces the warning on commit | [ ] |
| Check ↔ CLI helper | `ze passwd` surfaces the warning on stderr | [ ] |

### Integration Points
- New shared helper (e.g. under the config or a small auth util package) taking plaintext, returning an optional weakness reason.
- `internal/component/config/password_hash.go` - call the helper in `hashPlaintextSibling` before hashing. Today `ApplyPasswordHashing` and `hashPlaintextSibling` return error-only, and there is no warning-carrying return: the reason must be threaded out of `ApplyPasswordHashing` (new out-param or a `([]string, error)` / result-struct return) so the three commit call sites can surface it. `CheckBcryptLeaves` is NOT the route -- its only non-test caller besides `ze config validate` (`internal/component/cli/validator.go`) never sets a password.
- `internal/component/cli/editor_commit.go,321` - map the returned reason into `CommitResult.MigrationWarning` (built at `:196`, `:339`).
- `internal/component/cli/editor_commands.go` - `commitContent()` returns only `(string, error)` today, so this site must also gain a warning surface (extend its signature or route to the editor's status/warning path) for AC-1/AC-2 to hold here.
- `internal/plugins/passwd/main.go` - call the helper in `runImpl` before hashing; write the reason to `errOut`.

### Architectural Verification
- [ ] No bypassed layers (both set paths call the one helper)
- [ ] No unintended coupling (helper takes a string, returns a reason; no global state)
- [ ] No duplicated functionality (single denylist + length rule shared by both callers)
- [ ] Registration over hardcoding - the check is a shared helper invoked at the two password-set sites; no per-caller policy is duplicated into a core/shared package.

## Key Design Decisions

**The warning-channel route (load-bearing).** No path today both emits a warning
AND sets the password: `ApplyPasswordHashing` (`internal/component/config/password_hash.go`)
and `hashPlaintextSibling` return error-only, and `CommitResult` carries a
single-purpose `MigrationWarning` string (`internal/component/cli/contract/contract.go,63`)
built only at `internal/component/cli/editor_commit.go,339`. The
`CheckBcryptLeaves` -> `validator.go` warning walk is the `ze config validate`
path and never sets a password, so it cannot carry this warning.

Decision: thread the weakness reason out of the commit hashing path itself --
`ApplyPasswordHashing`/`hashPlaintextSibling` gain a warning-carrying return
(out-param, `([]string, error)`, or a small result struct) -- and surface it at
the three commit call sites:
- `internal/component/cli/editor_commit.go` and `:321` map the reason into `CommitResult.MigrationWarning`.
- `internal/component/cli/editor_commands.go` (`commitContent()`, currently `(string, error)`) gains a warning surface too.
- `internal/plugins/passwd/main.go` (`runImpl`) writes the reason to `errOut`.

This gives AC-1/AC-2 ("warning emitted AND password set") a real route rather than
riding the validate-only `CheckBcryptLeaves` channel, which cannot set passwords.

**Policy values are proposals, not final.** Minimum length 8 and the embedded
denylist (see Proposed Policy Defaults) are owner-adjustable defaults chosen so
boundary tests have concrete numbers; the owner may change either without
re-approving the design.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The config-commit path can surface a non-fatal warning once the reason is threaded out of `ApplyPasswordHashing` | `CommitResult.MigrationWarning` (`contract.go`) already carries one advisory string to the operator; `commitContent()` (`editor_commands.go`) has none yet and must gain one | warning is swallowed at a site with no surface | trace all three commit call sites during audit (see Key Design Decisions) | unvalidated |
| A-2 | Both password-set sites hold plaintext before hashing | password_hash.go, main.go | a set path bypasses the check | grep all bcrypt set sites | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Warning treated as an error and blocks commit | commit fails on a weak password | keep the helper's return advisory; never convert to error |
| R-2 | Denylist bloat / maintenance | list grows unbounded | keep a small fixed embedded list (top common passwords) + length rule; not a dictionary |
| R-3 | Plaintext leaking into logs | plaintext in a warning string | warn with a generic reason, never echo the password |
| R-4 | Textual merge friction with `plan/spec-fixit-bcrypt-hash-credential.md` (semantically independent, but edits the same `internal/component/config/password_hash.go` and the same three commit call sites `editor_commit.go,321`, `editor_commands.go`) | both specs touch the same lines | SEQUENCE the two specs; land whichever the owner picks first. If the bcrypt spec changes `ApplyPasswordHashing`'s signature, adopt the chosen-first signature and rebase this spec's helper wiring onto it |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| set account password to a short/common value | → | strength helper returns a reason; commit warns | `test/parse/password-weakness-warning.ci` |
| set account password to a strong value | → | no warning; commit clean | `test/parse/password-weakness-warning.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | password shorter than the minimum length | warning emitted; password still set |
| AC-2 | password matching the embedded denylist | warning emitted; password still set |
| AC-3 | password matching denylist (different case) | warning emitted (case-insensitive) |
| AC-4 | strong password (long, not in list) | no warning |
| AC-5 | empty password | unchanged, and no weakness warning: config path is a NO-OP (`password_hash.go` returns nil, leaf untouched -- not a rejection); `ze passwd` still rejects it (`internal/plugins/passwd/main.go`) |
| AC-6 | password over 72 bytes | still rejected (unchanged) |
| AC-7 | `ze passwd` with a weak value | warning on stderr; hash still printed |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | sets a trivially weak account password and sees a warning while the commit still succeeds | plaintext → strength helper → commit warning | `test/parse/password-weakness-warning.ci` |

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
| min length | length 8 (proposed default, owner-adjustable) | 8 (no warning) | 7 (warning) | - |
| bcrypt length | 1..72 bytes | 72 | 0 (rejected) | 73 (rejected) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `password-weakness-warning` | `test/parse/password-weakness-warning.ci` | weak password warns yet sets; strong password is silent | |

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
| Functional test | [ ] yes | `test/parse/password-weakness-warning.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] no | - (no new config; advisory warning only) |

## Files to Create
- `internal/component/config/password_strength.go` - shared length + denylist helper
- `test/parse/password-weakness-warning.ci` - functional test
- (unit tests in new/existing `_test.go`)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - add the strength helper (returns a reason, unused) and a failing `test/parse/password-weakness-warning.ci`.
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
| functional | `test/parse/password-weakness-warning.ci` |

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
