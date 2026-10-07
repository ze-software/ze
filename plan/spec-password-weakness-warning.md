# Spec: password-weakness-warning

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/6 |
| Updated | 2026-10-07 |

Anchor refresh (2026-07-22 plan review, design HOLDS against the landed bcrypt
work, learned 1181): the R-4 risk materialized benignly -- 1181 touched the
same three commit sites and wired `RejectMaskedBcryptLeaves` there, shifting
anchors ~6-9 lines (citations below updated in-body): `editor_commit.go`
152 -> 158 and 312 -> 321; `MigrationWarning` build 190 -> 196 and
330 -> 339. Rebase the helper wiring onto
the current commit-site lines per R-4's stated mitigation.

**Superseded 2026-08-14 by spec-netlab-integration.** Two premises stated here are
now false. `ApplyPasswordHashing` is no longer error-only: it returns
`([]string, error)`, the dot-paths it hashed, which is the warning channel this
spec's Key Design Decisions section says does not exist. And the empty case is no
longer a no-op: it hashes nothing, as before, and now DELETES the ephemeral leaf.
AC-5 still holds, for the corrected reason recorded in its own row.

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
- [ ] `docs/architecture/config/syntax.md` - the JUNOS-like config syntax: blocks, terminators, comments and inheritance
- [ ] `docs/architecture/system-architecture.md` - the legacy note on hub/orchestrator mode with separate plugin processes
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
- [ ] `internal/component/config/password_hash.go` - `hashPlaintextSibling` computes `PasswordWeakness` before bcrypt hashing and carries the weakness with the hashing result. The warning remains advisory. `ApplyPasswordHashing` is the entry point.
  → Constraint (corrected 2026-08-14 by spec-netlab-integration): the empty case is NO LONGER a no-op. It hashes nothing, as before, and it now DELETES the ephemeral `plaintext-` leaf, so the leaf reaches neither the running tree nor a serialized file. `ApplyPasswordHashing` also has a second entry point now: `LoadConfig` calls it, so a config FILE reaches this code and not only an editor commit.
- [ ] `internal/plugins/passwd/main.go` - `runImpl` refuses an empty input, writes the shared weakness warning to stderr, and still hashes and prints an accepted password with exit 0. Bcrypt's too-long input remains an error.

### Post-wave corrections (2026-07-10)

All refs re-verified against current code: NO drift. `ApplyPasswordHashing`
(password_hash.go), `hashPlaintextSibling` and `runImpl` (main.go, empty
:71-74, hash :75, too-long :77-79) all match the citations above exactly.

**Superseded 2026-08-14 by spec-netlab-integration.** The line citations into
`password_hash.go` are stale, and the "empty no-op" reading is now false: the
empty branch deletes the ephemeral leaf. Re-read the file before implementing.

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
- `internal/component/config/password_hash.go` - call the helper in `hashPlaintextSibling` before hashing. **Corrected 2026-08-14:** `ApplyPasswordHashing` now returns `([]string, error)` and `hashPlaintextSibling` returns `(bool, error)`, so the warning-carrying return this bullet asked for already exists in the shape it proposed. Thread the reason through those, and note there are now FOUR call sites, not three: `LoadConfig` (`internal/component/config/loader.go`) is a config-file entry point this spec's Data Flow does not list. `CheckBcryptLeaves` is NOT the route -- its only non-test caller besides `ze config validate` (`internal/component/cli/validator.go`) never sets a password.
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
and `hashPlaintextSibling` return error-only (**corrected 2026-08-14: they no longer
do. `ApplyPasswordHashing` returns `([]string, error)` and `hashPlaintextSibling`
returns `(bool, error)`, so the channel exists and `LoadConfig` already uses it to
warn. Re-derive this decision before implementing**), and `CommitResult` carries a
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
| A-1 | The config-commit path can surface a non-fatal warning once the reason is threaded out of `ApplyPasswordHashing` | `CommitResult.MigrationWarning` (`contract.go`) already carries one advisory string to the operator; `commitContent()` (`editor_commands.go`) has none yet and must gain one | warning is swallowed at a site with no surface | trace all three commit call sites during audit (see Key Design Decisions) | confirmed -- `CommitResult.Warnings` (renamed from `MigrationWarning`, `contract.go`) reaches `appendCommitWarnings` (`model_commands_commit.go`); `commitContent` gained `(string, []string, error)` and `Save`/`StageCandidate` carry it to `cmd_set.go`, `cmd_deactivate.go`, `model_load.go` and `ConfigSessionManager.Commit` |
| A-2 | Both password-set sites hold plaintext before hashing | password_hash.go, main.go | a set path bypasses the check | grep all bcrypt set sites | confirmed -- `hashPlaintextSibling` (`password_hash.go`) is the only place a ze:bcrypt leaf is written from plaintext, and every config route reaches it through `ApplyPasswordHashing` (4 call sites); `runImpl` (`internal/plugins/passwd/main.go`) is the second and last plaintext site |

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
| AC-5 | empty password | unchanged, and no weakness warning: the config path hashes nothing, so there is no password to judge. It is not a rejection. Corrected 2026-08-14: the leaf is no longer left untouched, it is DELETED, so a weakness check placed after the hash never sees it. `ze passwd` still rejects an empty value (`internal/plugins/passwd/main.go`) |
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
| `TestPasswordStrengthShort` | `internal/component/config/password_strength_test.go` | short password returns a reason, and 8 characters does not | PASS |
| `TestPasswordStrengthDenylist` | `internal/component/config/password_strength_test.go` | denylisted value (any case) returns a reason; a substring does not | PASS |
| `TestPasswordStrengthStrongNoReason` | `internal/component/config/password_strength_test.go` | strong password returns no reason | PASS |
| `TestPasswordWeaknessNeverEchoesPlaintext` | `internal/component/config/password_strength_test.go` | the reason never carries the password (R-3) | PASS |
| `TestHashPlaintextWeakStillSets` | `internal/component/config/password_hash_test.go` | weak password warns but is still hashed/set | PASS |
| `TestCmdSetWeakPasswordWarnsAndSets` | `internal/component/config/cli/cmd_set_test.go` | `ze config set` prints the warning on stderr and exits 0 | PASS under the daemon feature tags (`ze_core ze_distro` + every `feature-gates.txt` tag); without `ze_bgp` the package resolves no `ze-bgp-conf` and every schema test fails, recorded in `plan/journal/silent-fall-through.md` 2026-09-04 |
| `TestCommitPathsWarnWeakPasswordAndSetIt` | `internal/component/cli/editor_commit_test.go` | `CommitSession` and `CommitSessionCandidate` return the warning in `CommitResult.Warnings` and commit the hash | PASS (added at closure review) |
| `TestLoadConfigWarnsWeakPasswordAndSetsIt` | `internal/component/config/loader_test.go` | `LoadConfig` logs one WARN line for the weak password and sets it | PASS (added at closure review) |
| `TestRunImplWeakPlaintextWarnsAndHashes` | `internal/plugins/passwd/main_test.go` | `ze passwd` warns on stderr and still prints the hash | PASS |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| min length | length 8 (proposed default, owner-adjustable) | 8 (no warning) | 7 (warning) | - |
| bcrypt length | 1..72 bytes | 72 | 0 (rejected) | 73 (rejected) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `password-weakness-warning` | `test/parse/password-weakness-warning.ci` | weak password warns yet sets; strong password is silent | PASS 2026-10-07 (`./le test bgp parse --pattern password-weakness-warning`, 2.6s); also recorded in `features/runs/password-weakness-warning.json` |

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
5. **Full verification** → `./le verify current mode full`
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
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The 2026-09-06 implementation reported `TestCmdSetWeakPasswordWarnsAndSets` blocked by a broken package | `internal/component/config/cli` resolves `ze-bgp-conf` only when built with `ze_bgp`; under the daemon feature tags the package is green | closure re-ran the package with every `feature-gates.txt` tag | recorded the tag set in the TDD table; the untagged trap is already `plan/journal/silent-fall-through.md` 2026-09-04 |
| approach | Three of the six warning surfaces (`CommitSession`, `CommitSessionCandidate`, `LoadConfig`) shipped with no test asserting the warning | the `.ci` drives only `ze config set` and `ze passwd` | closure review, wiring step | added two tests, each red with its surfacing line removed |

## Design Insights
<!-- LIVE -->

## Implementation Summary
### What Was Implemented
- `internal/component/config/password_strength.go` -- `PasswordWeakness(plaintext) string`, `PasswordMinLength = 8`, and `passwordDenylist` (8 entries, whole-value case-insensitive match). The reason names the rule and never the password.
- `internal/component/config/password_hash.go` -- `hashPlaintextSibling` runs the check on the plaintext before hashing and returns the reason beside the hashed bool. `ApplyPasswordHashing` now returns `[]HashedPassword{Path, Weakness}`, and `PasswordWeaknessWarnings` is the single home for the warning wording.
- `internal/component/config/loader.go` -- `warnWeakPassword` logs one WARN line per weak password at load.
- `internal/component/cli/contract/contract.go` -- `CommitResult.MigrationWarning string` became `CommitResult.Warnings []string`, one advisory channel carrying both the migration warning and the password warnings.
- `internal/component/cli/editor_commit.go`, `editor_commands.go` -- both session commit sites and `commitContent`/`Save`/`StageCandidate` carry the warnings out.
- `internal/component/cli/model_commands_commit.go`, `model_load.go` -- `appendCommitWarnings` renders them in the commit status line.
- `internal/component/config/cli/cmd_set.go`, `cmd_deactivate.go`, `main.go` -- `printCommitWarnings` writes them to stderr before the success line.
- `internal/component/api/config_session.go` -- the `ConfigEditor` interface carries the warnings and `ConfigSessionManager.Commit` logs them, because the REST, gRPC and gNMI commit responses have no warning field.
- `internal/plugins/passwd/main.go` -- `runImpl` warns on `errOut` and still prints the hash with exit 0.
- Docs: `docs/guide/authentication.md` (the policy, the surface table, the load-path line), `docs/features.md` (one row), `docs/guide/command-reference.md` (`ze passwd`).

### Bugs Found/Fixed
- None in the product. The closure review found two test and comment gaps, listed under Review Gate.

### Documentation Updates
- None at closure. `docs/guide/authentication.md` ("Weak passwords are named, never refused", the surface table and the load-path line), `docs/features.md` and `docs/guide/command-reference.md` (`ze passwd`) landed in `0cb93dd5b0`; re-read at closure against `PasswordWeakness`, `PasswordWeaknessWarnings`, `printCommitWarnings`, `appendCommitWarnings`, `logCommitWarnings` and `warnWeakPassword`, and every claim holds, including the masked `set` line (`cmd_set.go`, `DisplayValueAtPath`).

### Deviations from Plan
- `CommitResult.MigrationWarning string` became `Warnings []string` rather than gaining a second field: one advisory channel for the migration warning and the password warnings.
- The REST, gRPC and gNMI commit responses carry no warning field, so `ConfigSessionManager.Commit` logs the warning at WARN with the user name instead.
- Step 5 (`./le verify current mode full`) was not run at closure: the owner barred whole-tree gates while another session runs a heavy programme on this machine. Scoped evidence is under Pre-Commit Verification, and the commit records the verification debt.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Warn at password-set time on a short password | Done | `internal/component/config/password_strength.go` `PasswordWeakness` | `PasswordMinLength = 8`, counted in runes |
| Warn on an embedded common-password denylist | Done | `password_strength.go` `passwordDenylist` | 8 entries, `strings.EqualFold` whole-value |
| Advisory only, the commit still succeeds | Done | `password_hash.go` `hashPlaintextSibling` | reason computed before the hash, hash always written |
| One helper for both set paths | Done | `hashPlaintextSibling` and `internal/plugins/passwd/main.go` `runImpl` | both call `PasswordWeakness` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `.ci` seq 1-2, `TestPasswordStrengthShort`, `TestCommitPathsWarnWeakPasswordAndSetIt`, `TestLoadConfigWarnsWeakPasswordAndSetsIt` | |
| AC-2 | Done | `TestPasswordStrengthDenylist`, `TestCommitPathsWarnWeakPasswordAndSetIt`, `.ci` seq 5 | |
| AC-3 | Done | `.ci` seq 3 (`LetMeIn`), `TestPasswordStrengthDenylist` | |
| AC-4 | Done | `.ci` seq 4, `TestPasswordStrengthStrongNoReason` | |
| AC-5 | Done | `TestApplyPasswordHashingEmptyPlaintext`, `TestRunImplEmptyPlaintext` | `PasswordWeakness("")` returns "" |
| AC-6 | Done | `TestRunImplOversizePlaintextRejected`, 73-byte case in `password_hash_test.go` | unchanged |
| AC-7 | Done | `.ci` seq 5, `TestRunImplWeakPlaintextWarnsAndHashes` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestPasswordStrength*`, `TestPasswordWeaknessNeverEchoesPlaintext` | Done | `internal/component/config/password_strength_test.go` | PASS |
| `TestHashPlaintextWeakStillSets` | Done | `internal/component/config/password_hash_test.go` | PASS |
| `TestCmdSetWeakPasswordWarnsAndSets` | Done | `internal/component/config/cli/cmd_set_test.go` | PASS under daemon tags |
| `TestRunImplWeakPlaintextWarnsAndHashes` | Done | `internal/plugins/passwd/main_test.go` | PASS |
| `password-weakness-warning` | Done | `test/parse/password-weakness-warning.ci` | PASS |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/config/password_strength.go` | Done | |
| `internal/component/config/password_hash.go` | Done | |
| `internal/plugins/passwd/main.go` | Done | |
| `test/parse/password-weakness-warning.ci` | Done | |

### Audit Summary
- **Total items:** 19
- **Done:** 19
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A weak password is made visible at set time | functional | `test/parse/password-weakness-warning.ci` PASS 2026-10-07: `ze config set` prints `warning: system.authentication.user.alice.password: weak password (shorter than 8 characters)` and `ze passwd` prints `warning: weak password (one of the most common passwords)` |
| The warning never breaks an existing config | functional | same `.ci`: exit 0 on every weak set, and `cat test.conf` shows a `$2a$10$` hash with no `plaintext-password` leaf |
| The editor commit and the daemon load surface it too | unit, discriminated | `TestCommitPathsWarnWeakPasswordAndSetIt` and `TestLoadConfigWarnsWeakPasswordAndSetsIt` went red with the `PasswordWeaknessWarnings` append in `editor_commit.go` and the `warnWeakPassword` call in `loader.go` removed, and green restored |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every AC is demonstrated | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/password-weakness-warning-450bc92b-6ac1-4190-bd40-b427ecba17bf.md` |
| `./le spec review check` | clean |
| Rounds | 2 |
| Reviewer lenses used | wiring and functional coverage, removed-behavior audit, security (plaintext leak, downgrade), stale comments, Go style; run inline by the closure agent, which did not write the code |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Three warning surfaces had no test asserting the warning: `CommitSession`, `CommitSessionCandidate` and `LoadConfig`. Dropping the `PasswordWeaknessWarnings` append left every test green | `internal/component/cli/editor_commit.go`, `internal/component/config/loader.go` | `TestCommitPathsWarnWeakPasswordAndSetIt` (`editor_commit_test.go`), `TestLoadConfigWarnsWeakPasswordAndSetsIt` (`loader_test.go`); red with the surfacing removed |
| 2 | ISSUE | Stale comment: `ApplyPasswordHashing` said "Two callers read it" while four call sites read it | `internal/component/config/password_hash.go` | now "Every caller reads it" |

NOTEs (no action): `ConfigSessionManager.Commit` logs the warning before the `onCommit` reload hook runs, so a reload that then fails still leaves the advisory line in the log; the line is true of the staged password either way. `appendCommitWarnings` rendering in the TUI status line has no own test; it is the same three-line loop the migration warning used. `./le commit audit` reports 20 deleted/weakened findings, all under `internal/component/bgp/plugins/persist`, `internal/component/bgp/reactor` and `test/plugin`/`test/reload` persist tests, another session's work; none touches this spec's files.

Run 2 over the fixed tree: 0 BLOCKER, 0 ISSUE.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify worktree` passes
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

## Progress, 2026-09-06

Committed in `0cb93dd5b0`. Its `.ci` never ran.

The implementation remains present: `PasswordWeakness` supplies the shared
policy, `hashPlaintextSibling` records its warning, and `passwd.runImpl` warns
before producing the hash. The recorded commit does not demonstrate the
functional `.ci` or satisfy the unticked review and goal gates. This spec
remains in progress until that evidence is supplied.

## Progress, 2026-10-07

The evidence is supplied: the `.ci` passes, the review gate is clean, and the
spec closes.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/config/password_strength.go` | yes | `git hash-object` resolves; `gopls symbols` lists `PasswordWeakness`, `PasswordMinLength`, `passwordDenylist` |
| `test/parse/password-weakness-warning.ci` | yes | blob `9dc3c9d652654177429546bf8f81098d591d7a97`, the blob `features/runs/password-weakness-warning.json` recorded |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-4, AC-7 | warn, set, exit 0; strong is silent | `./le test bgp parse --pattern password-weakness-warning`: `2.6s 1/1 PASS 235 password-weakness-warning` |
| AC-1, AC-2 (editor, load) | warning carried, hash set | `go test -tags "<daemon tags>" -run 'Weak|Strength|Weakness|TestLoadConfig|TestCommitPathPasswordHashingUnchanged' ./internal/component/cli/ ./internal/component/config/`: `ok` both |
| AC-5, AC-6, AC-7 | empty and 73-byte refusals unchanged in `ze passwd` | `go test -race ./internal/plugins/passwd/`: `ok 14.939s` |
| all | the touched packages | under the daemon tags: `ok internal/component/config/cli 94.959s`, `ok internal/component/cli 346.127s`, `ok internal/component/api 1.224s`; `internal/component/config` fails only `TestRFC7950MandatoryUnderAbsentNonPresenceContainer`, a deliberate untagged red owned by the rfc-verdict-test-fix-pass programme (`validator_mandatory_rfc7950_red_test.go`) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze config set ... plaintext-password` weak and strong | `test/parse/password-weakness-warning.ci` seq 1-4 | read: real `ze` binary, stderr `contains=` the warning, `cat test.conf` shows the hash |
| `ze passwd` weak | same file, seq 5 | read: stdin `qwerty`, stdout hash, stderr warning |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `CommitResult.Warnings` filled at `editor_commit.go` (both commit functions) and `commitContent` (`editor_commands.go`); `TestCommitPathsWarnWeakPasswordAndSetIt` |
| A-2 | confirmed | `git grep "ApplyPasswordHashing(\|PasswordWeakness("`: 4 hashing call sites plus `passwd.runImpl`, all through the one helper |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/authentication.md` surface table and sample lines | `printCommitWarnings` (`config/cli/main.go`), `appendCommitWarnings` (`model_commands_commit.go`), `warnWeakPassword` (`loader.go`), `logCommitWarnings` (`api/config_session.go`), `runImpl` | yes |
| `features/password-weakness-warning.md` Defect review row | repointed to the bare stem at closure | yes |
