# Spec: YANG built-in type leafref

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

Ze offers no `leafref` type: no Ze module uses one and `internal/component/config/yang/validator.go::validateYangType` has no `Yleafref` arm, so a reference to a node that does not exist would pass. Ze enforces cross-node references with Go validators (`ze:validate`). This is a feature Ze does not offer today; the owner can decline it in one word, and until then its MUSTs are requirements.

Each row below is a MUST of RFC 7950 the summary `rfc/short/rfc7950.md` carries as `{gap}` naming this spec. The ruling of 2026-09-21 (Thomas: "our goal is RFC compliance") makes every one a requirement until the owner declines it in one word; none is declined today.

| Id | RFC text | Producer or absence |
|----|----------|---------------------|
| RFC7950-9.9-1 | If the "require-instance" property (Section 9.9.3) is "true", there MUST exist a node in the data tree, or a node with a default value in use (see Sections 7.6.1 and 7.7.2), of the referred schema tree leaf or leaf-list node with the same value as the leafref value in a valid data tree. (§9.9) If the referring node represents configuration data and the "require-instance" property (Section 9.9.3) is "true", the referred node MUST also represent configuration. (§9.9) There MUST NOT be any circular chains of leafrefs. (§9.9) If the leaf that the leafref refers to is conditional based on one or more features (see Section 7.20.2), then the leaf with the leafref type MUST also be conditional based on at least the same set of features. (§9.9) | no Ze module uses type leafref (grep: 0); internal/component/config/yang/validator.go::validateYangType has no Yleafref arm |
| RFC7950-9.9.2-1 | The "path" statement, which is a substatement to the "type" statement, MUST be present if the type is "leafref". (§9.9.2) It takes as an argument a string that MUST refer to a leaf or leaf-list node. (§9.9.2) If the "require-instance" property is "true", this node set MUST be non-empty. (§9.9.2) | same as 9.9-1 |
| RFC7950-9.9.3-1 | If "require-instance" is "true", it means that the instance being referred to MUST exist for the data to be valid. (§9.9.3) | same as 9.9-1 |
