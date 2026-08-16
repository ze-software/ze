# Spec: fixit-relax-audit-reports-the-wrong-token

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/fixit-vacuous-eor-family-tests.md` |
| Updated | 2026-08-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`scripts/dev/audit-test-relaxation.py` `run_audit` pairs a file's NEW
`test-relax:` tokens with the wrong text when a token is added ANYWHERE except
after every token that was already there. It prints a reason belonging to an
older, unrelated relaxation, so a reviewer reads a justification that does not
describe the change in front of them.

The producing lines are in `run_audit`. It binds `old_tokens` to the LENGTH of
`relax_reasons(old, new_p)`, binds `new_tokens` to the LIST `relax_reasons(new,
new_p)`, and then takes the added set as `new_tokens[old_tokens:]`.

That slice reads the new list BY POSITION using the COUNT of the
old ones. That is only correct when every added token sorts after every existing
one. Insert a token near the top of a file that already has one further down and
the slice returns the pre-existing token instead of the new one.

**Reproduction.** In this repository at commit `e4cf75070`, add a `test-relax:`
comment near the top of `internal/component/bgp/reactor/peer_test.go`, which
already carries an MVPN relaxation lower down, then run
`python3 scripts/dev/audit-test-relaxation.py`. It prints
`reason: MVPNRoute / mvpnRouteGroupKey / groupMVPNRoutesByKey were removed by`,
which is the OLD token. The new one is never shown.

**How it was found.** An independent reviewer of
`spec-fixit-vacuous-eor-family-tests` noticed the audit quoting a
justification unrelated to that spec's change. Nothing in that spec depends on the
audit's text, so the defect blocks no goal and is written up here rather than
fixed there (`ai/rules/completion.md`, "A problem you FIND gets a SPEC").

**Severity beyond the wrong string.** The audit exists so a human can confirm each
relaxation's reason. A reason belonging to a different change makes that
confirmation worthless in the direction that matters: it reads as already
justified.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/testing.md` -- the test-deletion and weakening rule the audit serves
- [ ] `ai/rules/repo-maintenance.md` -- the hook-to-rule mapping, which names the
      shared detector this audit imports
- [ ] `.claude/skills/ze-review/SKILL.md` -- step 0 runs this audit and tells the
      reviewer to quote each reason

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/audit-test-relaxation.py` -- `run_audit`, and `relax_reasons`
      which it calls twice
- [ ] `scripts/dev/audit_relaxation_test.py` -- whether any case adds a token
      before an existing one
  → Constraint: if no case does, the defect is invisible to the suite, and the
    first deliverable is the failing test.

**Behavior to preserve:**
- Every existing finding class: `[DELETED]`, `[WEAKENED]`, `[RELAXED]`, and the
  RFC-tagged-change branch that reuses the hook's own detector.
- The `[RELAXED]` versus `[WEAKENED]` split, which turns on whether any token was
  added at all.

**Behavior to change:**
- Added tokens are identified by IDENTITY, not by list position.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A reviewer runs `python3 scripts/dev/audit-test-relaxation.py` at step 0 of
  `/ze-review`. That is the ONLY entry point: no `mk/*.mk` target and no stage of
  `make ze-precommit-verify` invokes it, and `scripts/dev/commit_helper.py` does not either.
  So a wrong `reason:` line is seen by a human reviewer or by nobody.

### Transformation Path
1. `resolve_anchor` picks the commit to diff against; `changed_test_files` lists
   the changed test paths.
2. For each path, `run_audit` reads the OLD blob and the worktree file.
3. `relax_reasons` extracts the `test-relax:` token texts from each side.
4. The added set is derived, and its texts are printed as `reason: ...`.
5. Step 4 is the defect: the derivation is a positional slice.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Audit ↔ git | `git show <anchor>:<path>` for the old side | Yes, read in `run_audit` |
| Audit ↔ the write hook | `rfc_detector` is the hook's own `_rfc_tagged_change_err`, imported | Yes, read in `run_audit` |
| Audit ↔ reviewer | the printed `reason:` line is the only text a human confirms | Yes |

### Integration Points
- `.claude/skills/ze-review/SKILL.md` step 0 runs it and quotes each reason.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The fix stays inside `run_audit` |
| No unintended coupling (components stay isolated) | Yes | The shared RFC detector is untouched |
| No duplicated functionality (extends existing, does not recreate) | Yes | `relax_reasons` keeps its single caller pair |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Python tooling, not a hot path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | N-A | No registration surface |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `python3 scripts/dev/audit-test-relaxation.py` over a file whose new token sits above an old one | → | `run_audit` added-token derivation | a new case in `scripts/dev/audit_relaxation_test.py` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A file with one existing `test-relax:` token gains a second one ABOVE it | The audit prints the NEW token's reason, never the old one |
| AC-2 | A file's only token is unchanged, and the file changes some other way | No token is reported as added, so the finding stays `[WEAKENED]` |
| AC-3 | A token's TEXT is edited in place, count unchanged | The audit reports the edited token as added, because its justification is new |
| AC-4 | Every existing case in `scripts/dev/audit_relaxation_test.py` | Still passes |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| a new token added ABOVE an existing one reports its own reason | `scripts/dev/audit_relaxation_test.py` | AC-1 | |
| an unchanged token is not reported as added | `scripts/dev/audit_relaxation_test.py` | AC-2 | |
| a token edited in place is reported as added | `scripts/dev/audit_relaxation_test.py` | AC-3 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | The audit is a dev tool with no daemon surface; its own Python suite is the end-to-end test | |

## Files to Modify

- `scripts/dev/audit-test-relaxation.py` -- `run_audit`
- `scripts/dev/audit_relaxation_test.py` -- the failing case first

## Implementation Steps

1. **Phase: Reproduce** -- add the AC-1 case and watch it report the OLD token
   - Verify: the assertion names the new token and the test is RED
2. **Phase: Fix** -- derive the added set by identity in `run_audit`
   - Verify: AC-1 green, AC-2 and AC-3 added and green
3. **Phase: Regression** -- `make ze-unit-pkg-test PKG=./scripts/dev`
   - Verify: AC-4, every existing case still passes

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] **Commit A:** code + tests + spec
- [ ] **Commit B:** `git rm plan/<spec>` only

## Known Limitations

- Matching by identity cannot tell a MOVED token from an unchanged one when the
  text is byte-identical. That is the correct answer anyway: an unchanged
  justification is not a new one.
