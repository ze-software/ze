# Spec: followup-hooks

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `git log -p plan/deferrals.md` (pre-2026-07-06) - original deferral rows + evidence

## Task

Enable or fix three dead/broken agent-guard hooks that currently run on a dead path or no-op.

This is a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Each item below was confirmed still-open against the codebase with a producing `file:line`. Split into phases when picked up; the sections after Task are lightweight scaffolding to be filled at design time.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **Enable BGP format-file append-idiom guard (L232)** - `c_format_alloc` returns `None` early (`pretool-writeedit.py:740`); the real check is unreachable dead code. Deliberate switch-on would start blocking `fmt.Sprintf`/`strings.Builder`/`strings.Join` in the 9 `bgp/format/*.go` files.
- **Fix validate-spec.sh (L233)** - still carries a `set -e` crash + Unicode-arrow vs ASCII `->` mismatch, so it aborts before validating some specs. Fix the arrow match + set -e fragility, then decide block-vs-warn.
- **Fixture tests for commit-gated Bash checks (L234)** - spec-audit/deferral-in-diff/deferral-unassigned/wiring-at-commit/doc-drift only fired on a dead `git commit` path; spec-audit was never ported. Add fixture tests driving each check directly.

## Required Reading

### Source files / docs

- [ ] `.claude/hooks/pretool-writeedit.py` (`c_format_alloc` returns None @ :740)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `.claude/hooks/validate-spec.sh` (set -e crash + arrow mismatch)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `.claude/hooks/pretool-bash.py` (CHECKS tuple; spec-audit not ported)
  -> Constraint: verify current behaviour against this source before designing.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; line numbers are pre-triage references)

- [ ] `.claude/hooks/pretool-writeedit.py`
- [ ] `.claude/hooks/validate-spec.sh`
- [ ] `.claude/hooks/pretool-bash.py`

**Behavior to preserve:**
- All existing behaviour of the listed files; this backlog work only adds the missing pieces named in the Task work items.

**Behavior to change:**
- Only the specific gaps enumerated in the Task work items.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Agent Write/Edit and Bash tool calls intercepted by PreTool/PostTool hooks

### Transformation Path
1. A hook fires on a tool call
2. The (currently dead/broken) check runs
3. It blocks or warns on the offending edit/command

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| tool call -> hook | stdin JSON contract | [ ] |
| hook -> agent | exit code 0/1/2 semantics | [ ] |

### Integration Points
- `.claude/hooks/pretool-writeedit.py`
- `.claude/hooks/validate-spec.sh`
- `.claude/hooks/pretool-bash.py`

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The verified `file:line` evidence in the Task items still holds at design time | 2026-07-06 backlog triage | Re-scope the item | grep/LSP at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scope drift when the umbrella is split into per-item specs | Item needs its own design doc | Split into a dedicated spec and re-point |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Edit a `bgp/format/*.go` with `fmt.Sprintf` | → | format-alloc guard blocks it once enabled | (fill during design) |
| Write a spec with an ASCII-arrow wiring table | → | validate-spec.sh validates without aborting | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | (define per work item when this skeleton moves to `design`) | (define at design time) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (define at design time) | (define at design time) | per Task work item | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A | N/A | internal hook tooling - validated by fixture tests, not `.ci` (no daemon path) | |

## Files to Modify

- `.claude/hooks/pretool-writeedit.py` - see Task work items
- `.claude/hooks/validate-spec.sh` - see Task work items
- `.claude/hooks/pretool-bash.py` - see Task work items

## Implementation Steps

1. **Phase: split** - if the umbrella covers unrelated items, split into per-item specs first.
2. **Phase: design** - for the chosen item, re-verify the `file:line` evidence and fill the Data Flow / Wiring / AC sections above.
3. **Phase: wiring** - register entry points, write the failing wiring test.
4. **Phase: implement (TDD)** - write test, fail, implement, pass, per work item.
5. **Full verification** - `make ze-verify`.
6. **Complete spec** - fill audit tables, write `plan/learned/NNN-<name>.md`, two-commit closure.

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
- [ ] Every chosen work item has feature code + test
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Skeleton = captured intent, not a designed spec (see `ai/rules/deferral-tracking.md`). Moves to `design` when someone picks it up.
