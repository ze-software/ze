# Spec: YANG built-in type bits

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

Ze offers no `bits` type: no Ze module uses one and `internal/component/config/yang/validator.go::validateYangType` has no `Ybits` arm, so a value of that type would pass unchecked. This is a feature Ze does not offer today; the owner can decline it in one word, and until then its MUSTs are requirements.

Each row below is a MUST of RFC 7950 the summary `rfc/short/rfc7950.md` carries as `{gap}` naming this spec. The ruling of 2026-09-21 (Thomas: "our goal is RFC compliance") makes every one a requirement until the owner declines it in one word; none is declined today.

| Id | RFC text | Producer or absence |
|----|----------|---------------------|
| RFC7950-9.7-1 | When an existing bits type is restricted, the set of assigned names in the new type MUST be a subset of the base type's set of assigned names. (§9.7) The bit position of such an assigned name MUST NOT be changed. (§9.7) | no Ze module uses type bits (grep: 0); internal/component/config/yang/validator.go::validateYangType has no Ybits arm |
| RFC7950-9.7.4-1 | The "bit" statement, which is a substatement to the "type" statement, MUST be present if the type is "bits". (§9.7.4) All assigned names in a bits type MUST be unique. (§9.7.4) | same as 9.7-1 |
| RFC7950-9.7.4.2-1 | The position value MUST be in the range 0 to 4294967295, and it MUST be unique within the bits type. (§9.7.4.2) If the current highest bit position value is equal to 4294967295, then a position value MUST be specified for "bit" substatements following the one with the current highest position value. (§9.7.4.2) When an existing bits type is restricted, the "position" statement MUST either have the same value as in the base type or not be present, in which case the value is the same as in the base type. (§9.7.4.2) | same as 9.7-1 |
