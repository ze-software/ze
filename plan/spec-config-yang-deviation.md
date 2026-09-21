# Spec: YANG deviation statement

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

- [ ] `internal/component/config/yang/loader.go` -> goyang applies a deviation at Resolve; no Ze module carries one and Ze reads no deviated node
- [ ] `internal/component/config/yang_schema.go` -> yangToNode converts the resolved entry tree

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A module carrying the statement reaches `Loader.AddModuleFromText`, then `Loader.Resolve`.

### Transformation Path
goyang `yang.Entry` with the deviation applied -> Ze schema node -> validator walk.

### Boundaries Crossed
None: every step is inside `internal/component/config`.

### Integration Points
The schema node types and the `validateYangType` walk.

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

No Ze module carries a `deviation` statement. goyang holds deviation code ("cannot find target node to deviate"), and Ze's schema builder reads no deviated node. This is a feature Ze does not offer today; the owner can decline it in one word, and until then its MUSTs are requirements.

Each row below is a MUST of RFC 7950 the summary `rfc/short/rfc7950.md` carries as `{gap}` naming this spec. The ruling of 2026-09-21 (Thomas: "our goal is RFC compliance") makes every one a requirement until the owner declines it in one word; none is declined today.

| Id | RFC text | Producer or absence |
|----|----------|---------------------|
| RFC7950-7.20.3-1 | This means that deviations MUST never be part of a published standard, since they are the mechanism for learning how implementations vary from the standards. (§7.20.3) Server deviations are strongly discouraged and MUST only be used as a last resort. (§7.20.3) After applying all deviations announced by a server, in any order, the resulting data model MUST still be valid. (§7.20.3) | no Ze module carries a deviation statement (the 3 hits are prose); goyang holds deviation code ("cannot find target node to deviate") |
| RFC7950-7.20.3.2-1 | If a property can only appear once, the property MUST NOT exist in the target node. (§7.20.3.2) The properties to replace MUST exist in the target node. (§7.20.3.2) The substatement's keyword MUST match a corresponding keyword in the target node, and the argument's string MUST be equal to the corresponding keyword's argument string in the target node. +--------------+--------------+-------------+ \ | substatement \ |
