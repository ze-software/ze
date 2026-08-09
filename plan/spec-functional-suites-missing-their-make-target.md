# Spec: functional-suites-missing-their-make-target

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/functional-suites-missing-their-make-target.md` |
| Updated | 2026-08-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`ze-functional-test` in `mk/test-functional.mk` runs 24 suites and, when one
fails, prints `make ze-<suite>-test` for each failed name. Three of those
targets do not exist: `ze-ldp-test`, `ze-rsvpte-test` and `ze-install-test`.
The operator is handed a command that cannot run, and those three suites have
no make entry point at all.

Measured on 2026-08-09 by testing each name in the `all_suites` line against
`mk/*.mk` and `Makefile`: 21 of 24 resolve, and `ldp`, `rsvpte` and `install`
do not.

The cost is not only the dead hint. Running one of those suites means
reproducing what `ZE_ALT_BUILD` does by hand, because the binaries the runner
needs carry different build tags from `make ze`. That was paid in this session:
verifying two `.ci` changes in the install suite needed a hand-built pair of
binaries before the suite would run at all.

The fix has two halves. Add the three missing targets, and add a check that
refuses a suite name in `all_suites` with no matching target, so the next suite
added cannot repeat this.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/commands.md` - prefer make over a bare runner invocation
  → Constraint: a suite with no target forces the thing this rule forbids
- [ ] `ai/rules/repo-maintenance.md` - a new feature updates the checks that keep it discoverable
  → Decision: the check is the durable half; the three targets alone would drift again
- [ ] `mk/inventory.mk` - where existing inventory checks live
  → Constraint: the new check belongs beside them, not in a new gate family

**Key insights:**
- The `all_suites` line is already the single source of truth for the suite set.
  The check reads it rather than carrying a second list.

## Current Behavior (MANDATORY)

Source files read:
- [ ] `mk/test-functional.mk` - the `all_suites` line sets the suite set and the
  progress denominator, `run_suite` records each failure by name, and the
  failure block prints `make ze-%s-test` for every failed name. The individual
  suite targets sit below that block, one per suite, each depending on
  `$(ZE_TEST_DEPS)` and running `$(ZE_ALT_BUILD)` before `$(ZE_TEST_RUN)`.

Behavior to preserve: the three properties below, unchanged by the fix.
- The `all_suites` line stays the one place the suite set is written.
- Each suite target keeps the `SUITE_RUN` wall-clock cap and the `ZE_ALT_BUILD`
  isolated-binary path the existing targets use.
- Suites that intentionally have a runner but no `all_suites` entry, named in
  the comment above that line, stay out of the gating run.

## Data Flow (MANDATORY)

### Entry Point
`make ze-functional-test`, its printed retry hint, and `make ze-<suite>-test`
for one suite.

### Transformation Path
1. `ze-functional-test` iterates the `all_suites` line to size the run.
2. `run_suite` runs each suite and records the failed names.
3. The failure block prints one `make ze-<suite>-test` line per failed name.
4. The operator runs that command, which either resolves to a target or fails.

### Boundaries Crossed

| From | To | Carried |
|------|----|---------|
| `all_suites` line | run loop | the suite set |
| Failure block | operator | a make command name |
| Suite target | test runner | the isolated binary pair and the suite name |

### Integration Points
- `$(ZE_ALT_BUILD)` and `$(ZE_ALT_TRAP)` build and clean the isolated binaries.
- `$(SUITE_RUN)` applies the per-suite wall-clock cap.
- `mk/inventory.mk` holds the checks that keep generated and listed sets honest.

### Architectural Verification
- The check derives its expectation from the `all_suites` line. A second list of
  suite names anywhere in the tree would be the defect this spec is fixing.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | Validation |
|----|------------|-------|-----------|
| A-1 | The three suites run correctly once given a target | they already run inside `ze-functional-test` | run each new target and compare against the combined run |
| A-2 | `ldp` and `rsvpte` need the same deps as their neighbours | they are plain `.ci` suites | read the runner invocation in the combined target |

### Risks

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | The check fires on a suite that deliberately has no target | the comment above `all_suites` already separates those; the check reads the same line the run loop reads |

## Blast Radius

`mk/test-functional.mk` and the checks that run in verification. No product code.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `make ze-install-test` | -> | the install suite runner invocation | the new target runs the suite green |
| A suite name in `all_suites` with no target | -> | the new inventory check | a case in the check's sibling python test |

## Acceptance Criteria

| AC | Input / Condition | Expected Behavior |
|----|-------------------|-------------------|
| AC-1 | `make ze-ldp-test`, `make ze-rsvpte-test`, `make ze-install-test` | each runs its suite, with the same caps and isolated binaries as its neighbours |
| AC-2 | A suite name added to `all_suites` with no matching target | the check fails and names the missing target |
| AC-3 | The repository as it stands after AC-1 | the check passes |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| missing target is named | the check's sibling python test | AC-2 |
| the current suite set passes | the check's sibling python test | AC-3 |

### Functional Tests

| Test | File | Validates |
|------|------|-----------|
| `make ze-install-test` reaches the install suite | `test/install/appliance-kernel-registry.ci` | AC-1 for install |
| `make ze-ldp-test` reaches the ldp suite | an existing `.ci` under `test/ldp/` | AC-1 for ldp |
| `make ze-rsvpte-test` reaches the rsvpte suite | an existing `.ci` under `test/rsvpte/` | AC-1 for rsvpte |

## Files to Modify

- `mk/test-functional.mk` - add `ze-ldp-test`, `ze-rsvpte-test`, `ze-install-test`
- `mk/inventory.mk` - register the new check
- `ai/INDEX.md` - name the three targets where the other suite targets are listed

## Files to Create

- the check script, beside the other inventory checks, with its sibling python test

### Integration Checklist
- [ ] The new check runs inside the verification target that owns inventory checks
- [ ] Each of the three new targets has been run once and reported green

## Implementation Steps

1. Add the three targets, copying the shape of their neighbours.
2. Write the check that compares the `all_suites` names against the defined targets.
3. Register the check and run it.

### Critical Review Checklist
- [ ] No second list of suite names is introduced
- [ ] The three new targets carry the same caps and deps as their neighbours
- [ ] The check names the missing target rather than only failing

## Known Limitations

The check sees the `all_suites` line only. A suite invoked from somewhere else
is outside its reach, and that is the whole point: the line is the source of
truth this repository already chose.

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
