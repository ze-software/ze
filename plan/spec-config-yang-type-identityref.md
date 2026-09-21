# Spec: YANG built-in type identityref

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

Ze offers no `identityref` type: no Ze module uses one, and `validateYangType` has no arm for it. This is a feature Ze does not offer today; the owner can decline it in one word, and until then its MUSTs are requirements.

Each row below is a MUST of RFC 7950 the summary `rfc/short/rfc7950.md` carries as `{gap}` naming this spec. The ruling of 2026-09-21 (Thomas: "our goal is RFC compliance") makes every one a requirement until the owner declines it in one word; none is declined today.

| Id | RFC text | Producer or absence |
|----|----------|---------------------|
| RFC7950-9.10.2-1 | The "base" statement, which is a substatement to the "type" statement, MUST be present at least once if the type is "identityref". (§9.10.2) Otherwise, an identity with the matching name MUST be defined in the current module or an included submodule. (§9.10.2) | no Ze module uses type identityref (grep: 0); goyang "an identityref must specify a base" |
| RFC7950-9.10.3-1 | Otherwise, an identity with the matching name MUST be defined in the current module or one of its submodules. (§9.10.3) | same as 9.10.2-1 |
