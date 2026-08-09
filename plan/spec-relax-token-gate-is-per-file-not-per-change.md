# Spec: relax-token-gate-is-per-file-not-per-change

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/relax-token-gate-is-per-file-not-per-change.md` |
| Updated | 2026-08-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The test-weakening gate can be switched off for a whole file, and the audit that
reports relaxations can print the wrong reason. Both come from one shape: the
`test-relax:` token is matched against a whole file, or by position in a list,
never against the change that needs it.

**The gate opens per file, on the Write path.** `c_test_weakening` in
`.claude/hooks/pretool-writeedit.py` calls `_has_relax_token` and returns early,
before `_test_weakening_errs` runs. On the `Edit` and `MultiEdit` branches the
text it searches is the replacement string, so the token must sit inside the
change, and that is correct. On the `Write` branch the text is the ENTIRE new
file, so one pre-existing token anywhere in that file disables the weakening
check for every later overwrite of it. The hook's own RFC-branch message states
the opposite, that the check reads only the replacement text of this edit. That
sentence is true for Edit and false for Write. `_has_relax_token`'s docstring
counts 315 files carrying the hash-comment form of the token, so the population
that can be overwritten unchecked is large.

This is a guard that fails open and says nothing, which `ai/rules/evidence.md`
refuses.

**The audit names reasons by position.** `run_audit` in
`scripts/dev/audit-test-relaxation.py` counts the old file's tokens, then takes
the tail of the new file's token list from that index. A reason inserted
anywhere but last makes the audit print a different, pre-existing reason. Found
on 2026-08-09 while adding a third token to `test/install/kernel-compose.ci`:
the marker had to be placed where the slice would find it rather than beside the
lines it explains.

Reproduction, gate: overwrite with Write any test file that already carries a
`test-relax:` token, deleting assertions in the overwrite. The check returns
without looking. Reproduction, audit: add a reason above a middle hunk of a file
that already carries one, then read the reason the audit prints.

## Required Reading

### Architecture Docs
- [ ] `.claude/hooks/README.md` - what each hook gates and how it is tested
  → Constraint: the fix must keep the Edit path exactly as it is; only Write is wrong
- [ ] `ai/rules/testing.md` - the standing ban on reaching green by weakening a test
  → Decision: the token is an auditable escape hatch, not a switch, so it must bind to a change
- [ ] `ai/rules/evidence.md` - a guard fails closed or says something
  → Constraint: the silence on the Write path is the defect, not the strictness

**Key insights:**
- The two producers share one detection module: the audit imports the hook's
  detector, so a change to the token model must satisfy both readers.

## Current Behavior (MANDATORY)

Source files read:
- [ ] `.claude/hooks/pretool-writeedit.py` - `c_test_weakening` sets the compared
  text from `old_string` and `new_string` on the Edit branch, joins the edits on
  the MultiEdit branch, and on the Write branch reads the file from disk as the
  old text and takes the tool's whole `content` as the new text. It then calls
  `_has_relax_token` on that new text and returns None on a hit, before
  `_test_weakening_errs` is called. The Write branch also leaves `hunks` empty,
  so `_enclosing_tagged_scope` has nothing to widen on.
- [ ] `scripts/dev/audit-test-relaxation.py` - `run_audit` counts the old file's
  reasons and slices the new file's reason list from that index, so the reported
  reasons are whichever tokens sit at the tail positions.
- [ ] `scripts/dev/audit_relaxation_test.py` - the audit's own tests, built on
  fixture repositories that symlink the real hook.

Behavior to preserve: the three properties below, unchanged by the fix.
- The Edit and MultiEdit paths keep binding the token to the replacement text.
- The RFC-tagged branch keeps running BEFORE the token escape hatch, because
  the token is self-service and RFC approval is not.
- A documented relaxation carrying a genuine reason keeps passing, on every path.

## Data Flow (MANDATORY)

### Entry Point
A `Write`, `Edit`, or `MultiEdit` tool call on a test file, and the
`scripts/dev/audit-test-relaxation.py` run that reports what changed.

### Transformation Path
1. The hook decides the target is a test carrier.
2. It builds the old text and the new text, per tool branch.
3. The RFC-tagged branch runs and can block.
4. `_has_relax_token` searches the new text and returns None on a hit.
5. `_test_weakening_errs` compares counts and shapes, and blocks on a reduction.
6. Separately, the audit imports that detector and reports added reasons by slice.

### Boundaries Crossed

| From | To | Carried |
|------|----|---------|
| Tool call | hook stdin | tool name, file path, replacement text or whole content |
| Hook | audit script | the imported weakening detector |
| Audit | operator output | the reason text attributed to a change |

### Integration Points
- `scripts/dev/hook-fixture-check.py` runs the hook's fixtures.
- `scripts/dev/audit_relaxation_test.py` runs the audit's own tests.
- Any commit path that calls the audit before a commit.

### Architectural Verification
- One detection module stays one module. The fix must not fork the token model
  into a hook copy and an audit copy.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | Validation |
|----|------------|-------|-----------|
| A-1 | A Write on a test file carrying a token is reachable in normal work | agents overwrite whole `.ci` files | write the fixture and watch the gate return None |
| A-2 | Binding the token to the changed region is expressible on the Write path | the branch already holds both whole texts | design phase derives the changed region from a diff of the two |

### Risks

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | A stricter Write path blocks a legitimate whole-file rewrite that carries its reason elsewhere | the block message names the line the token must sit on |
| R-2 | Moving reason attribution off the positional slice changes the audit output for existing files | run the audit over the repository before and after, and compare |

## Blast Radius

Every test file edit in the repository passes through this gate, and every
relaxation report reads that attribution.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| A Write that deletes assertions from a test file already carrying a token | -> | the Write branch of `c_test_weakening` | a hook fixture named for the Write bypass |
| A reason added above a middle hunk | -> | reason attribution in `run_audit` | a case in `scripts/dev/audit_relaxation_test.py` |

## Acceptance Criteria

| AC | Input / Condition | Expected Behavior |
|----|-------------------|-------------------|
| AC-1 | A Write deletes assertions from a test file whose pre-existing token is unrelated | the gate blocks and names what was deleted |
| AC-2 | A Write deletes assertions and carries its own reason on the changed lines | the gate allows it, as Edit does today |
| AC-3 | A reason is added above a middle hunk | the audit prints that reason, not a pre-existing one |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| Write bypass is refused | the hook fixture set | AC-1 |
| Write with an in-change reason passes | the hook fixture set | AC-2 |
| reason attribution is not positional | `scripts/dev/audit_relaxation_test.py` | AC-3 |

### Functional Tests

| Test | File | Validates |
|------|------|-----------|
| none | - | the surface is a hook and a script, with no daemon path |

## Files to Modify

- `.claude/hooks/pretool-writeedit.py` - bind the token to the changed region on the Write path
- `scripts/dev/audit-test-relaxation.py` - attribute an added reason to its hunk, not to a tail slice
- `scripts/dev/audit_relaxation_test.py` - the AC-3 case
- `.claude/hooks/README.md` - correct the statement that the check reads only the replacement text

## Files to Create

None expected; the fixture set already has a home.

### Integration Checklist
- [ ] The hook fixture runner and the audit test both pass
- [ ] The audit output over the whole repository is compared before and after

## Implementation Steps

1. Derive the changed region on the Write path and search only it for the token.
2. Attribute an added reason to the hunk it sits on in the audit.
3. Correct the hook message and the README sentence that contradict the Write path.

### Critical Review Checklist
- [ ] The Edit and MultiEdit paths behave identically after the fix
- [ ] The RFC-tagged branch still runs before the token escape hatch
- [ ] The block message names the line the reason must sit on

## Known Limitations

The token stays self-service by design. This spec makes it bind to a change; it
does not make it an approval.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1, AC-2, AC-3 each proven by a named test
- [ ] Tests written
- [ ] Tests FAIL before the fix
- [ ] Tests PASS after the fix

### Quality Gates
- [ ] `make ze-verify`
- [ ] `make ze-lint-changed`

### Closure
- [ ] Deferral shard row closed
