# Spec: YANG feature and if-feature statements

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

- [ ] `internal/component/config/yang/loader.go` -> goyang parses feature and if-feature statements; Ze reads neither
- [ ] `internal/component/config/yang/validator.go` -> walkTree evaluates no if-feature

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A module carrying the statement reaches `Loader.AddModuleFromText`; data under the node reaches `Validator.ValidateTree`.

### Transformation Path
goyang `yang.Entry` with the statement -> Ze schema node -> validator walk.

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

No Ze module declares a `feature` or carries an `if-feature`, and the validator evaluates none, so a node conditional on a feature is neither hidden nor refused. This is a feature Ze does not offer today; the owner can decline it in one word, and until then its MUSTs are requirements.

Each row below is a MUST of RFC 7950 the summary `rfc/short/rfc7950.md` carries as `{gap}` naming this spec. The ruling of 2026-09-21 (Thomas: "our goal is RFC compliance") makes every one a requirement until the owner declines it in one word; none is declined today.

| Id | RFC text | Producer or absence |
|----|----------|---------------------|
| RFC7950-7.20.1-1 | A feature MUST NOT reference itself, neither directly nor indirectly through a chain of other features. (§7.20.1) In order for a server to support a feature that is dependent on any other features (i.e., the feature has one or more "if-feature" substatements), the server MUST also support all the dependent features. (§7.20.1) | no Ze module carries an if-feature statement (grep over internal/**/*.yang: 0) and none declares a feature statement (the 10 hits are prose); {V} has no feature evaluation |
| RFC7950-7.20.2-1 | Otherwise, a feature with the matching name MUST be defined in the current module or an included submodule. (§7.20.2) A leaf that is a list key MUST NOT have any "if-feature" statements. (§7.20.2) | same as 7.20.1-1 |
