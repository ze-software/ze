# Spec: config-operation-oldconfig-has-no-reader

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Depends | - |
| Phase | - |
| Updated | 2026-09-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`ConfigOperationParams` (`pkg/plugin/rpc/types.go`) is the parameter block every
config operation carries across the plugin ABI. Eight of its twelve fields have
no non-test reader, and six of those have no writer either.

Measured on 2026-09-12 over `internal`, `pkg` and `cmd`, excluding `_test.go`:

| Field | Writers | Readers |
|-------|---------|---------|
| `Property` | iface | `createInterfaceByType` (`internal/component/iface/operation.go`) |
| `Config` | bgp | `applyConfigOperation` (`internal/component/bgp/reactor/operation.go`) |
| `OldConfig` | bgp, two sites | none |
| `Changed` | bgp, one site | none |
| `Prefix`, `NextHop`, `Metric`, `Value`, `OldValue`, `Spec` | none | none |

The six with neither are the residue of the seven operation kinds
`spec-config-apply-ordering-covers-every-root` deleted: `add-static-route`,
`set-sysctl`, `set-property` and their siblings were written for roots that never
arrived, and their parameter fields outlived them.

`OldConfig` is different and it is the more interesting one. It had a reader
until that same spec's phase 4. The remove-peer inverse rebuilt the peer from the
operation's config subtree, which dropped eleven kinds of state and restored a
session that announced nothing. `runningPeerSettings`
(`internal/component/bgp/reactor/operation.go`) answers from the reactor's own
running peer instead, and its comment states why the subtree cannot serve. So the
BGP decomposer still fills a field whose last reader was deliberately removed.

This is dead weight on an EXTERNAL contract, which is what makes it worth a spec
rather than a comment: `pkg/plugin/sdk` re-exports the type to plugin authors, so
every field is a promise about what an operation carries.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - the `config-operation-*`
  payload shape, which is what the field set publishes.
- [ ] `docs/plugin-development/protocol.md` - the same contract as a plugin
  author reads it.

### RFC Summaries (Scope: protocol)
Not applicable. Scope is `config`. The plugin RPC contract is Ze's own.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/plugin/rpc/types.go` - `ConfigOperationParams` declares the twelve
  fields, and `ConfigOperation` carries it.
- [ ] `pkg/plugin/sdk/sdk_types.go` - re-exports the type unchanged for external
  plugin authors.
- [ ] `internal/component/bgp/plugin/operation.go` - `bgpPeerOperation` and
  `bgpModifyPeerOperation` fill `OldConfig`, and `decomposeBGPOperationInput`
  fills `Changed`.
- [ ] `internal/component/bgp/reactor/operation.go` - `runningPeerSettings`
  states in its own comment why the subtree in `OldConfig` cannot be used.

**Behavior to preserve:**
- `Property` and `Config` are live and stay.
- The JSON keys of every surviving field, and the kebab-case spelling.

**Behavior to change:**
- A field nobody reads is deleted, and the writer that fills it goes with it.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A decomposer builds `ConfigOperation` values, which the executor emits and the
  bridge turns into the `config-operation-*` RPC.
- Format at entry: `ConfigOperationParams` as JSON.

### Transformation Path
1. The owning component's decomposer fills the params.
2. The executor emits the operation on the owner's stream event.
3. The owning component's applier reads the params it needs.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin SDK to external plugin author | `pkg/plugin/sdk` aliases of the rpc types | No |

### Integration Points
- `pkg/plugin/rpc/types.go` and its SDK alias are the only declaration sites.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A grep over `internal`, `pkg` and `cmd` for each surviving field of `ConfigOperationParams` | Every field has at least one non-test reader |
| AC-2 | A BGP reload that removes a peer and rolls back | The peer returns with everything it announced, unchanged by the deletion |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SIGHUP whose apply fails after a peer is removed | → | `runningPeerSettings`, which is what replaced the deleted field | `test/reload/config-apply-ordering-mixed-rollback.ci`, which exists and passes |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApplyConfigOperationRemovePeerRollbackRestoresAnnouncedState` | `internal/component/bgp/reactor/operation_test.go` | The rollback is unaffected by the deletion. It exists and passes today | exists |
| To be named | `internal/component/bgp/plugin/operation_test.go` | Replaces the two `assert.NotEmpty(t, ops[0].Params.OldConfig)` assertions, which are the only thing holding the field | not written |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `config-apply-ordering-mixed-rollback` | `test/reload/config-apply-ordering-mixed-rollback.ci` | A failed apply rolls the peer back with its routes | exists and passes |

## Files to Modify
- `pkg/plugin/rpc/types.go` - delete the fields with no reader.
- `pkg/plugin/sdk/sdk_types.go` - drop whatever the alias exposes of them.
- `internal/component/bgp/plugin/operation.go` - delete the writers.
- `internal/component/bgp/plugin/operation_test.go` - the two assertions on
  `OldConfig` go with the field.
- `docs/architecture/api/process-protocol.md`, `docs/plugin-development/protocol.md`
  - the payload tables.

## Implementation Steps

1. **Re-measure the reader and writer counts**, because another spec may have
   given one of these fields a reader since 2026-09-12. The table above is a
   measurement with a date, not a standing fact.
2. **Delete each field with no reader, and its writers with it.** A field kept
   "for a future reader" is the shape this spec exists to remove.
3. **Update the two payload pages** in the same change.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | No config leaf |
| Plugin SDK | Yes | The type is re-exported, so the field set is an external promise |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 4 | API/RPC changed? | Yes | `docs/architecture/api/process-protocol.md` |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/plugin-development/protocol.md` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No external plugin depends on these fields | Ze is pre-release and no external plugin ships | A plugin built against the old SDK loses a field it read | `ai/rules/pre-release.md` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A field is deleted that a spec in flight was about to read | A merge conflict, or a reader appearing in the same week | Step 1 re-measures before anything is deleted |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior

### Goal Gates (MUST pass)
- [ ] `./le verify worktree` passes
- [ ] Every field of `ConfigOperationParams` has a non-test reader

## Provenance

Homed here at the closure of `spec-config-apply-ordering-covers-every-root`
(2026-09-12), whose Work Not Done table names it. That spec deleted the seven
unused operation kinds and left their parameter fields, and its own phase 4
removed `OldConfig`'s last reader. Thomas has not commissioned this; it is
written down so the gap is countable.
