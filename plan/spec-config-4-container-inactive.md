# Spec: config-4-container-inactive

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-config-3-deactivate (done) |
| Phase | - |
| Updated | 2026-04-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/` - Tree, parser, serializer
4. `plan/spec-config-3-deactivate.md` - predecessor spec

## Task

Migrate container-level `inactive` handling from the current schema-polluting
approach (an `inactive` leaf injected into the YANG schema per container) to the
same Tree-level scheme used for leaf and leaf-list values in spec-config-3-deactivate.

In the Tree-level scheme, inactive state lives in the Tree node metadata, not in
the schema. This means no `inactive` leaf appears in YANG autocomplete, and
`deactivate`/`activate` CLI commands work uniformly across all node types.

**Origin:** deferred from spec-config-3-deactivate (Design Insights, dated 2026-04-25).
The current container approach works (auto-injection) but pollutes the schema.
Migration is a refactor for consistency, not a bug fix.

## Required Reading

### Architecture Docs
- [ ] `internal/component/config/` - Tree structure, node types, parser, serializer
- [ ] `internal/component/config/parser_inactive_leaf_test.go` - current inactive leaf parsing
- [ ] `internal/component/config/tree_inactive_test.go` - current tree inactive behavior
- [ ] `internal/component/config/prune_inactive_leaf_test.go` - pruning inactive leaves
- [ ] `internal/component/config/serialize_inactive_leaf_test.go` - serialization
- [ ] `internal/component/config/setparser_inactive_test.go` - set-format parsing
- [ ] `internal/component/cli/editor_inactive_leaf_test.go` - CLI editor behavior
- [ ] `plan/spec-config-3-deactivate.md` - predecessor design decisions

**Key insights:**
- Skeleton: to be filled during design phase

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/config/` - Tree, parser, serializer for inactive handling
- [ ] `cmd/ze/config/cmd_deactivate.go` - deactivate CLI command
- [ ] `internal/component/cli/editor_inactive_leaf_test.go` - CLI editor behavior

**Behavior to preserve:**
- `deactivate` / `activate` CLI commands work on containers
- Existing configs with `inactive` leaf parse correctly (backward compat)
- `show | compare` displays inactive state

**Behavior to change:**
- Container inactive state stored in Tree metadata, not as schema leaf
- YANG schema no longer contains injected `inactive` leaf for containers
- Autocomplete no longer suggests `inactive` inside containers

## Data Flow (MANDATORY)

### Entry Point
- CLI `deactivate <container-path>` command
- Config file parsing with `inactive:` annotation on containers

### Transformation Path
1. CLI `deactivate` sets Tree node metadata flag (not a child leaf)
2. Config parser recognizes `inactive:` annotation on container lines
3. Serializer emits `inactive:` annotation on container lines
4. Pruner uses metadata flag instead of looking for child `inactive` leaf
5. YANG schema injection removed for containers

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI ↔ Config tree | deactivate command sets metadata | [ ] |
| Config file ↔ Tree | parser reads annotation | [ ] |
| Tree ↔ Config file | serializer writes annotation | [ ] |

### Integration Points
- CLI editor - deactivate/activate commands
- Config parser - annotation handling
- Config serializer - annotation output
- YANG schema - remove container inactive leaf injection

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| CLI `deactivate bgp { peer ... }` | → | Tree metadata set | `test/config/config-container-inactive.ci` |
| Config file with `inactive:` container | → | Parser reads annotation | `test/config/config-container-inactive-parse.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `deactivate` on a container | Tree annotation set, no `inactive` leaf created |
| AC-2 | `activate` on a deactivated container | Tree annotation removed |
| AC-3 | `show` on deactivated container | `inactive:` prefix displayed |
| AC-4 | `show \| compare` with inactive container | Diff shows inactive state change |
| AC-5 | Old config with `inactive` leaf in container | Parses correctly, migrated to Tree annotation |
| AC-6 | YANG autocomplete inside container | `inactive` not suggested as a valid child |
| AC-7 | Config serialize after migration | No `inactive` leaf emitted, `inactive:` annotation used |
| AC-8 | All existing inactive tests pass | No behavioral regression |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestContainerInactiveMetadata` | `internal/component/config/tree_inactive_test.go` | AC-1, AC-2 | skeleton |
| `TestContainerInactiveShow` | `internal/component/config/tree_inactive_test.go` | AC-3 | skeleton |
| `TestContainerInactiveSerialize` | `internal/component/config/serialize_inactive_leaf_test.go` | AC-7 | skeleton |
| `TestContainerInactiveParse` | `internal/component/config/parser_inactive_leaf_test.go` | AC-5 | skeleton |
| `TestContainerInactiveCompare` | `internal/component/config/tree_inactive_test.go` | AC-4 | skeleton |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `config-container-inactive` | `test/config/config-container-inactive.ci` | deactivate/activate container round-trip | skeleton |
| `config-container-inactive-parse` | `test/config/config-container-inactive-parse.ci` | Load config with inactive container annotation | skeleton |

### Future (if deferring any tests)
- None planned

## Files to Modify
- `internal/component/config/` - Tree node metadata, parser, serializer, pruner
- `cmd/ze/config/cmd_deactivate.go` - adapt to Tree annotation path
- `internal/component/cli/` - editor inactive handling
- Existing test files - update expectations

## Files to Create
- `test/config/config-container-inactive.ci` - functional test
- `test/config/config-container-inactive-parse.ci` - functional test

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No (removes injection) | YANG container schema |
| CLI commands/flags | No (existing deactivate/activate) | - |
| Editor autocomplete | Yes (remove `inactive` suggestion) | YANG-driven |
| Functional test for new RPC/API | Yes | `test/config/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No (behavior unchanged) | - |
| 2 | Config syntax changed? | No (annotation syntax unchanged) | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/` (inactive handling) |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4-13. | Standard flow |

### Implementation Phases

1. **Phase: Tree Metadata** -- add inactive flag to Tree node metadata
   - Tests: `TestContainerInactiveMetadata`
   - Files: Tree node struct, metadata accessors
   - Verify: tests fail → implement → tests pass

2. **Phase: Parser** -- recognize `inactive:` annotation on containers
   - Tests: `TestContainerInactiveParse`
   - Files: config parser
   - Verify: old configs with `inactive` leaf still parse

3. **Phase: Serializer** -- emit `inactive:` annotation
   - Tests: `TestContainerInactiveSerialize`
   - Files: config serializer
   - Verify: round-trip: parse → serialize → parse

4. **Phase: CLI** -- deactivate/activate use metadata
   - Tests: functional tests
   - Files: `cmd_deactivate.go`, editor
   - Verify: end-to-end CLI workflow

5. **Phase: Remove YANG injection** -- clean up schema pollution
   - Tests: `TestContainerInactiveShow` (no `inactive` in autocomplete)
   - Files: YANG schema injection code
   - Verify: AC-6

6. **Full verification** → `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Round-trip: deactivate → serialize → parse → activate |
| Backward compat | Old configs with `inactive` leaf still parse |
| Data flow | CLI → Tree metadata → serializer → parser |
| Rule: no-layering | Old YANG injection code fully removed |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Tree metadata supports inactive | grep for metadata accessor |
| YANG injection removed | grep confirms no container inactive leaf injection |
| All existing inactive tests pass | `go test ./internal/component/config/...` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Container paths validated by existing CLI path resolution |
| Config integrity | Inactive state cannot be corrupted by partial writes |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Existing test fails | Backward compat issue; investigate before changing test |
| 3 fix attempts fail | STOP. Ask user. |

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

## RFC Documentation

N/A

## Implementation Summary

### What Was Implemented
- Skeleton

### Bugs Found/Fixed
- None

### Documentation Updates
- None

### Deviations from Plan
- None

## Implementation Audit

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
- **Total items:** -
- **Done:** -
- **Partial:** -
- **Skipped:** -
- **Changed:** -

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- Skeleton

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit
