# Spec: Config lists carry a key and leaf-lists carry unique values and every default

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

- [ ] `internal/component/config/yang_schema.go` -> yangToList tolerates a keyless config list; the leaf-list branch of yangToNode reads no default
- [ ] `internal/component/config/tree.go` -> a leaf-list append never deduplicates
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` -> `list update` is keyless and addressed through `ze:display-key`

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
Module text reaches `Loader.AddModuleFromText`; config data reaches the tree through the parser and `Validator.ValidateTree`.

### Transformation Path
goyang `yang.Entry` -> `yangToList` and the leaf-list branch of `yangToNode` -> schema nodes -> the tree and the validator walk.

### Boundaries Crossed
None: every step is inside `internal/component/config`, except the BGP module text.

### Integration Points
The schema build error accumulator `recordSchemaBuildError`, the tree append path, and the CLI parser of the `update` block.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point -> Feature Code -> Test |
|-------------------------------------|
| To be designed -> to be designed -> to be designed |

## Acceptance Criteria

- AC-1: every requirement id in the Task table carries a positive and a negative tagged test with a discrimination record, and its `{gap}` annotation leaves `rfc/short/rfc7950.md`.

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

Three list rules of RFC 7950 are unmet. A configuration list MUST carry a key: `internal/component/config/yang_schema.go::yangToList` tolerates `entry.Key == ""`, and `internal/component/bgp/yang/ze-bgp-conf.yang` (`list update`, keyless, addressed by `ze:display-key`) relies on it, so the fix touches the module and the CLI path that parses the block. Leaf-list values in configuration MUST be unique: `internal/component/config/tree.go` appends without deduplicating. A leaf-list with several defaults MUST use all of them: the leaf-list branch of `yang_schema.go::yangToNode` builds `ValueOrArray` and reads no default at all, and `yangToLeaf` keeps `entry.Default[0]` only.

Each row below is a MUST of RFC 7950 the summary `rfc/short/rfc7950.md` carries as `{gap}` naming this spec. The ruling of 2026-09-21 (Thomas: "our goal is RFC compliance") makes every one a requirement until the owner declines it in one word; none is declined today.

| Id | RFC text | Producer or absence |
|----|----------|---------------------|
| RFC7950-7.7-1 | In configuration data, the values in a leaf-list MUST be unique. (§7.7) The definitions of the default values MUST NOT be marked with an "if-feature" statement. (§7.7) Conceptually, the values in the data tree MUST be in the canonical form (see Section 9.1). (§7.7) | internal/component/config/tree.go (leaf-list append never deduplicates, comment at :228), S |
| RFC7950-7.7.2-1 | o If no such ancestor exists in the schema tree, the default values MUST be used. (§7.7.2) o Otherwise, if this ancestor is a case node, the default values MUST be used if any node from the case exists in the data tree or the case node is the choice's default case, and if no nodes from any other case exist in the data tree. (§7.7.2) o Otherwise, the default values MUST be used if the ancestor node exists in the data tree. (§7.7.2) When the default values are in use, the server MUST operationally behave as if the leaf-list was present in the data tree with the default values as its values. (§7.7.2) | internal/component/config/yang_schema.go::yangToLeaf (keeps entry.Default[0] only) and schema_defaults.go::applyChildDefault, S |
| RFC7950-7.8.2-1 | The "key" statement, which MUST be present if the list represents configuration and MAY be present otherwise, takes as an argument a string that specifies a space-separated list of one or more leaf identifiers of this list. (§7.8.2) A leaf identifier MUST NOT appear more than once in the key. (§7.8.2) Each such leaf identifier MUST refer to a child leaf of the list. (§7.8.2) All key leafs MUST be given values when a list entry is created. (§7.8.2) All key leafs in a list MUST have the same value for their "config" as the list itself. (§7.8.2) | internal/component/config/yang_schema.go::yangToList (tolerates entry.Key == "" and defaults the key type to string), S |
