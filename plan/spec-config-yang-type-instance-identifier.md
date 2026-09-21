# Spec: YANG built-in type instance-identifier

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

- [ ] `internal/component/config/yang/validator.go` -> validateYangType has no arm for this type, so its default arm accepts any value
- [ ] `internal/component/config/yang_schema.go` -> yangTypeToValueType maps a goyang kind to a Ze value type

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A leaf of this type in a Ze module reaches `Loader.AddModuleFromText`; a value reaches `Validator.ValidateTree`.

### Transformation Path
goyang `YangType` -> Ze value type -> validator arm -> `ValidationError`.

### Boundaries Crossed
None: every step is inside `internal/component/config`.

### Integration Points
The `validateYangType` switch and the schema node types.

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

Ze offers no `instance-identifier` type: no Ze module uses one and no validator arm exists. This is a feature Ze does not offer today; the owner can decline it in one word, and until then its MUST is a requirement.

Each row below is a MUST of RFC 7950 the summary `rfc/short/rfc7950.md` carries as `{gap}` naming this spec. The ruling of 2026-09-21 (Thomas: "our goal is RFC compliance") makes every one a requirement until the owner declines it in one word; none is declined today.

| Id | RFC text | Producer or absence |
|----|----------|---------------------|
| RFC7950-9.13-1 | For identifying list entries with keys, each predicate consists of one equality test per key, and each key MUST have a corresponding predicate. (§9.13) If the leaf with the instance-identifier type represents configuration data and the "require-instance" property (Section 9.9.3) is "true", the node it refers to MUST also represent configuration. (§9.13) All such leaf nodes MUST reference existing nodes or leaf or leaf-list nodes with their default value in use (see Sections 7.6.1 and 7.7.2) for the data to be valid. (§9.13) | no Ze module uses type instance-identifier (grep: 0) |
