# Spec: Repository checks over the embedded YANG modules: namespace and revision rules

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

- [ ] `internal/component/config/yang/loader.go` -> ModuleNames lists every embedded module once; nothing reads a namespace or a revision
- [ ] `internal/le/register.go` -> the native tooling composition root a new `./le` check registers into

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A `./le rfc` or `./le repository` action run by a developer or CI.

### Transformation Path
Embedded module text -> goyang AST -> namespace and revision statements -> comparison against HEAD -> report.

### Boundaries Crossed
`internal/le` reads `internal/component/config/yang` modules.

### Integration Points
The `./le` action registry and the verify gates.

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

Two MUSTs of RFC 7950 bind the author of a module rather than its loader. A namespace URI MUST be chosen so it cannot collide (Section 5.3), and a published change MUST add a revision statement in front, keep the module name and keep the namespace (Section 11). No Ze code checks either. Both are a `./le` check under `internal/le` over the embedded modules: the namespace check reads every module the loader knows and refuses one whose URI does not carry the organization; the revision check compares each module against HEAD. Ze has never been released, so no published module exists yet and the Section 11 check has nothing to compare until the first release.

Each row below is a MUST of RFC 7950 the summary `rfc/short/rfc7950.md` carries as `{gap}` naming this spec. The ruling of 2026-09-21 (Thomas: "our goal is RFC compliance") makes every one a requirement until the owner declines it in one word; none is declined today.

| Id | RFC text | Producer or absence |
|----|----------|---------------------|
| RFC7950-5.3-1 | Namespace URIs MUST be chosen so they cannot collide with standard or other enterprise namespaces -- for example, by using the enterprise or organization name in the namespace. (§5.3) | absent; would live in internal/component/config/yang (loader_test over Loader.ModuleNames), S |
| RFC7950-11-1 | For any published change, a new "revision" statement (Section 7.1.9) MUST be included in front of the existing "revision" statements. (§11) If there are no existing "revision" statements, then one MUST be added to identify the new revision. (§11) Furthermore, any necessary changes MUST be applied to any metadata statements, including the "organization" and "contact" statements (Sections 7.1.7 and 7.1.8). (§11) Thus, a module name MUST NOT be changed. (§11) Furthermore, the "namespace" statement MUST NOT be changed, since all XML elements are qualified by the namespace. (§11) Obsolete definitions MUST NOT be removed from published modules, since their identifiers may still be referenced by other modules. (§11) Otherwise, if the semantics of any previous definition are changed (i.e., if a non-editorial change is made to any definition other than those specifically allowed above), then this MUST be achieved by a new definition with a new identifier. (§11) In statements that have any data definition statements as substatements, those data definition substatements MUST NOT be reordered. (§11) | absent; a repository check over the embedded modules (revision order, name and namespace stability against HEAD) would live under internal/le, S |
