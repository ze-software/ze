# Spec: YANG 1.1 yang-version declaration and submodules

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-21 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Required Reading

- `docs/architecture/config/yang-config-design.md`
- `rfc/short/rfc7950.md`, and `rfc/full/rfc7950.txt` at each section the Task table cites

## Current Behavior (MANDATORY)

- [ ] `internal/component/config/yang/loader.go` -> goyang parses yang-version and include statements; no Ze module declares a version and none is a submodule
- [ ] `internal/component/config/yang/modules/ze-types.yang` -> the embedded bootstrap module, declaring no yang-version

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
Module text reaches `Loader.AddModuleFromText`, then `Loader.Resolve`.

### Transformation Path
Module text -> goyang AST with its version -> `yang.Entry` tree -> Ze schema nodes.

### Boundaries Crossed
None: every step is inside `internal/component/config`.

### Integration Points
Every embedded and registered module's header.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point -> Feature Code -> Test |
|-------------------------------------|
| To be designed -> to be designed -> to be designed |

## Acceptance Criteria

- AC-1: every requirement id in the Task table carries a positive and a negative tagged test with a discrimination record, and its `{gap}` annotation leaves `rfc/short/rfc7950.md`, or the owner declines the feature and the rows carry `{feature-declined}` with his words.

## Risks & Assumptions

To be written at design.

## 🧪 TDD Test Plan

### Unit Tests

| Test | Asserts |
|------|---------|
| To be designed | To be designed |

## Files to Modify

To be decided at design.

## Implementation Steps

To be decided at design.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] `./le verify worktree`

## Task

No Ze module declares `yang-version` (each defaults to YANG 1) and none is a submodule, so the YANG 1 and 1.1 coexistence rules of Section 12 are exercised by nothing. Declaring `yang-version 1.1` on the modules that use 1.1 constructs (`action`, `notification` under a list) and refusing a YANG 1 module that imports a 1.1 module is the work. This is a feature Ze does not offer today; the owner can decline it in one word, and until then its MUST is a requirement.

Each row below is a MUST of RFC 7950 the summary `rfc/short/rfc7950.md` carries as `{gap}` naming this spec. The ruling of 2026-09-21 (Thomas: "our goal is RFC compliance") makes every one a requirement until the owner declines it in one word; none is declined today.

| Id | RFC text | Producer or absence |
|----|----------|---------------------|
| RFC7950-12-1 | A YANG version 1.1 module MUST NOT include a YANG version 1 submodule, and a YANG version 1 module MUST NOT include a YANG version 1.1 submodule. (§12) A YANG version 1 module or submodule MUST NOT import a YANG version 1.1 module by revision. (§12) | no Ze module declares yang-version (defaults to YANG 1) and none is a submodule (grep: 0) |
