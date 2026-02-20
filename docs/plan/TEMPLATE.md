# Spec: <task-name>

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. [List key architecture docs from Required Reading]
4. [List key source files from Files to Modify]

## Task
<description>

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/<doc>.md` - [why relevant]
  → Decision: [specific architectural decision that constrains this spec]
  → Constraint: [specific rule from the doc that applies here]

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfcNNNN.md` - [why relevant]
  → Constraint: [specific RFC rule that applies here]

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- [insight from docs]

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `path/to/file.go` - [what it currently does]

**Behavior to preserve:** (unless user explicitly said to change)
- [output format, e.g., "JSON uses nested [[]] arrays for OR/AND grouping"]
- [function signatures that callers depend on]
- [test expectations from existing .ci files]

**Behavior to change:** (only if user explicitly requested)
- [list changes user asked for, or "None - preserve all existing behavior"]

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- [Where data enters: wire bytes, API command, config, plugin message]
- [Format at entry]

### Transformation Path
1. [Stage 1: e.g., "Wire parsing in internal/bgp/message/"]
2. [Stage 2: e.g., "Attribute extraction via iterators"]
3. [Stage 3: e.g., "Pool storage with dedup"]
4. [Stage N: ...]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | [JSON format, command syntax] | [ ] |
| Wire ↔ Storage | [via iterators/pools] | [ ] |
| [other boundaries] | [mechanism] | [ ] |

### Integration Points
- [Existing function/type this connects to] - [how it integrates]

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
<!-- The Implementation Audit cross-references these criteria. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | [what triggers the behavior] | [observable outcome] |
| AC-2 | [what triggers the behavior] | [observable outcome] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `internal/.../xxx_test.go` | [description] | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [field] | [min-max] | [value] | [value or N/A] | [value or N/A] |

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
<!-- New RPCs/APIs MUST have functional tests — unit tests alone are NOT sufficient -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [what user expects to happen] | |

### Future (if deferring any tests)
- [Tests to add later and why deferred — requires explicit user approval]

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `internal/...` - [feature changes]

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | `internal/yang/modules/*.yang` |
| RPC count in architecture docs | [ ] | `docs/architecture/api/architecture.md` |
| CLI commands/flags | [ ] | `cmd/ze/*/main.go` or subcommand files |
| CLI usage/help text | [ ] | Same as above |
| API commands doc | [ ] | `docs/architecture/api/commands.md` |
| Plugin SDK docs | [ ] | `.claude/rules/plugin-design.md` |
| Editor autocomplete | [ ] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [ ] | `test/plugin/*.ci` or `test/decode/*.ci` |

## Files to Create
- `internal/...` - [new feature file]
- `test/.../*.ci` - [functional test for end-user behavior]

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests** → Review: edge cases? Boundary tests?
2. **Run tests** → Verify FAIL (paste output). Fail for RIGHT reason?
3. **Implement** → Minimal code to pass. Simplest solution? Follows patterns?
4. **Run tests** → Verify PASS (paste output). All pass? Any flaky?
5. **RFC refs** → Add `// RFC NNNN Section X.Y` comments
6. **RFC constraints** → Add quoted requirement comments above enforcing code
7. **Functional tests** → Create after feature works. Cover user-visible behavior?
8. **Verify all** → `make ze-lint && make ze-unit-test && make ze-functional-test`
9. **Final self-review** → Re-read changes, check for bugs, unused code, TODOs

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 (fix syntax/types) |
| Test fails wrong reason | Step 1 (fix test) |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to IMPLEMENT for that criterion |

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
<!-- LIVE — write IMMEDIATELY when you learn something -->
<!-- Route at completion: subsystem → arch doc, process → rules, knowledge → memory.md -->

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer constraints, message ordering, any MUST/MUST NOT.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered — add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] `make ze-lint` passes
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written → FAIL → implement → PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
