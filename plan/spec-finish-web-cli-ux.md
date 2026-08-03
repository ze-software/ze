# Spec: finish-web-cli-ux

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/1094-followup-web-cli-ux.md` - what the closed umbrella delivered
4. `plan/deferrals.md` - the rows that point here

## Task

`spec-followup-web-cli-ux` shipped and was closed (`plan/learned/1094-followup-web-cli-ux.md`,
retired in `49d1185ed`). Two of its deferrals were never done and were left pointing at the
deleted spec file, so `commit_helper.py` refuses commits with "live deferrals without a
destination spec". This spec is that destination. It exists to hold the residual items, not
to redo the umbrella.

### Work items (re-homed 2026-07-16 from `plan/deferrals.md`)

- **Nushell shell-generator glue (from AC-8/9, 2026-07-09)** - AC-8 scoped completion to
  bash/zsh/fish (all wired and tested). Nushell's single-completer model needs separate
  wiring, plus `ze config show` config-section completion. `plan/learned/1094` does not
  mention nushell at all, so this was never started. Not urgent: three shells work.
- **Control-hiding on purpose-built workbench pages (from AC-1 tail, 2026-07-09)** - the
  bgp peers/groups/policy, system, firewall and interfaces Add buttons built via
  `workbench_table.html` / `WorkbenchTableData`. Those page builders construct table data
  without the `*http.Request`, so `ReadOnly` cannot be threaded without a wider refactor.
  **UI polish only**: enforcement is already complete (route gate 403 + per-mutation authz),
  so a read-only operator cannot mutate anything today; the controls are merely visible.

### Work items (re-homed 2026-08-03 from `plan/deferrals/cli-root-namespace-grammar.md`)

Same species as the two above, and homeless for the same reason: their source spec
closed and its file is gone, leaving them pointing at prose. Both are CLI UX, neither
is a grammar defect -- the grammar half of that shard went to
`plan/spec-cli-root-namespace-grammar-deferred-gate-reach.md`.

- **Two `format` vocabularies (from Known Limitations, 2026-07-17)** - `set cli format`
  accepts text/table/json/yaml/ndjson (`internal/component/cli/model_keys.go`,
  `validCLIFormats`) while the editor's `format` pipe accepts only tree/config
  (`internal/component/cli/model_load.go`). Both are legitimate uses of the word.
  Reconciling them is a **UX decision, and it is the owner's**: an implementer picking
  one vocabulary to make the row close is picking the answer. Ask before designing.
- **`ze pipe` multi-operator (from Known Limitations, 2026-07-17)** - `runPipe`
  (`cmd/ze/ze_core_pipe.go`) joins argv into one pipe expression, so chaining needs a
  shell-quoted pipe. Accepting repeated operators natively is a usability change.

-> Constraint: none of the four items is a correctness gap. The AC-1 tail is explicitly
cosmetic (enforcement lands elsewhere and is done); the nushell item leaves three
working shells; both 2026-08-03 items are vocabulary and ergonomics on surfaces that
work today. Do not let the spec's existence imply urgency it does not have.

## Required Reading

### Architecture Docs
- [ ] `plan/learned/1094-followup-web-cli-ux.md` - what the umbrella delivered and why these were cut
  → Constraint: (fill during research) confirm the AC-1 enforcement claim against the producing authz code before treating this as cosmetic.

**Key insights:** (fill during research)

## Current Behavior (MANDATORY)

**Source files read:** (fill during research)
- [ ] `internal/plugins/completion/nushell.go` - the nushell generator as it stands

**Behavior to preserve:** bash/zsh/fish completion, and the existing route-gate/authz enforcement.

**Behavior to change:** (fill during design)

## Data Flow (MANDATORY)

### Entry Point
(fill during research)

### Transformation Path
1. (fill during research)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| (fill during research) | | [ ] |

### Integration Points
- (fill during research)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The AC-1 tail is cosmetic only: a read-only operator is already blocked by the route gate + per-mutation authz, so visible controls cannot mutate | `plan/deferrals.md` row (2026-07-09) asserts "enforcement is already complete (route gate 403 + per-mutation authz)" | If enforcement is NOT complete, this is a security gap, not UI polish, and the priority changes entirely | Read the producing authz/route-gate code and drive a read-only operator against one mutation endpoint | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Threading `ReadOnly` into `WorkbenchTableData` is the "wider refactor" the original spec avoided; it may grow past the value of hiding a button | The change starts touching page builders unrelated to the ones listed | Stop and re-scope; the enforcement is already correct |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| (fill during design) | → | (fill during design) | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| (fill during design) | | |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill during design) | | | |

### Functional Tests
<!-- Provisional -- confirmed at the DESIGN gate. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cli-completion-nushell` | `test/ui/cli-completion-nushell.ci` | An operator using nushell gets the same completions bash/zsh/fish already give. | planned |

## Files to Modify
- (fill during design)

## Implementation Steps
- (fill during design)

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

- A closed spec's deferrals outlive it. `spec-followup-web-cli-ux` was retired correctly
  (work done, learned summary written, file `git rm`-ed) but its two open deferral rows kept
  naming the deleted file, so `commit_helper.py`'s destination check blocked unrelated
  commits repo-wide until someone re-homed them. Closure should re-point surviving deferrals.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Re-home into a new `spec-finish-*` rather than recreate `spec-followup-web-cli-ux.md` | Recreate the retired filename | The umbrella genuinely closed; recreating it would misrepresent finished work as open and break `git log --follow` on the retired file. `spec-finish-<subsystem>` is the documented convention for "subsystem shipped, residual bits left" (`plan/deferrals.md` header, 2026-07-06 triage). |

## Known Limitations
- (fill during design)

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

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

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only
