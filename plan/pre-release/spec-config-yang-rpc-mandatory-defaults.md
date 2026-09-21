# Spec: RPC invocation and reply honor mandatory and default on input and output

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

- [ ] `internal/component/config/yang/rpc.go` -> records Mandatory on an RPC input parameter and nothing reads it on invocation
- [ ] `internal/component/command/grammar/checker.go` -> lints the command tree, not the invocation path

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
An RPC invocation reaches the command dispatcher from the CLI, the web and gNMI.

### Transformation Path
Parsed command -> RPC input map -> handler -> RPC output map -> renderer.

### Boundaries Crossed
`internal/component/config/yang` to `internal/component/command`.

### Integration Points
The RPC descriptor built by `rpc.go` and the dispatcher that fills the input map.

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

Ze fills the server role for its 37 rpcs and the client role through its own CLI and web. `internal/component/config/yang/rpc.go` records Mandatory on an RPC input parameter, and `internal/component/command/grammar/checker.go` lints the command tree, but nothing on the invocation path refuses a missing mandatory input leaf or fills an input default, and nothing on the reply path checks a mandatory output leaf or fills an output default.

Each row below is a MUST of RFC 7950 the summary `rfc/short/rfc7950.md` carries as `{gap}` naming this spec. The ruling of 2026-09-21 (Thomas: "our goal is RFC compliance") makes every one a requirement until the owner declines it in one word; none is declined today.

| Id | RFC text | Producer or absence |
|----|----------|---------------------|
| RFC7950-7.14.2-1 | If a leaf in the input tree has a "mandatory" statement with the value "true", the leaf MUST be present in an RPC invocation. (§7.14.2) If a leaf in the input tree has a default value, the server MUST use this value in the same cases as those described in Section 7.6.1. (§7.14.2) In these cases, the server MUST operationally behave as if the leaf was present in the RPC invocation with the default value as its value. (§7.14.2) If a leaf-list in the input tree has one or more default values, the server MUST use these values in the same cases as those described in Section 7.7.2. (§7.14.2) In these cases, the server MUST operationally behave as if the leaf-list was present in the RPC invocation with the default values as its values. (§7.14.2) If any node has a "when" statement that would evaluate to "false", then this node MUST NOT be present in the input tree. (§7.14.2) | internal/component/config/yang/rpc.go:195 records Mandatory on an RPC input parameter; internal/component/config/yang/cli/format.go:117 renders it; internal/component/command/grammar/checker.go:108 reads Mandatory as a design-lint rule (R6 ordering) and usage.go renders it; no runtime dispatch refusal of a missing mandatory input was found, M |
| RFC7950-7.14.3-1 | If a leaf in the output tree has a "mandatory" statement with the value "true", the leaf MUST be present in an RPC reply. (§7.14.3) If a leaf in the output tree has a default value, the client MUST use this value in the same cases as those described in Section 7.6.1. (§7.14.3) In these cases, the client MUST operationally behave as if the leaf was present in the RPC reply with the default value as its value. (§7.14.3) If a leaf-list in the output tree has one or more default values, the client MUST use these values in the same cases as those described in Section 7.7.2. (§7.14.3) In these cases, the client MUST operationally behave as if the leaf-list was present in the RPC reply with the default values as its values. (§7.14.3) If any node has a "when" statement that would evaluate to "false", then this node MUST NOT be present in the output tree. (§7.14.3) | absent; Ze's CLI and web are the clients of its own RPCs, M |
