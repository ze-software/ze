# Spec: plugin-declaration-fields-on-registration

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Updated | 2026-09-09 |

## Task

`show plugin declarations` answers COMMANDS and PIPES for a plugin the ze binary
carries, and nothing else. Those are the two Stage 1 fields that have a
compiled-in twin: `registry.Registration` holds `Commands []rpc.CommandDecl` and
`Pipes []rpc.PipeDecl` and no other declaration field.

Every other field of `rpc.DeclareRegistrationInput` reaches the engine only when
a plugin runs: config operations, filters, doctor checks, show enrichers, schema,
budgets and the failure policy. An operator asking what a plugin declares gets
two of them.

**Provenance:** `spec-plugin-query-mode`, closed 2026-09-09. Its Key Design
Decisions row "Commands and pipes only" named this work, and its Known
Limitations section states the gap in the shipped documentation. That spec is
closed and removed from disk, so this file is the only home for the item.
`plan/learned/013-inertness-is-unreachability.md` carries what it decided.

## Required Reading

<!-- NEVER tick [ ] to [x]. -->

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - the five startup stages and everything the Stage 1 message carries
- [ ] `docs/architecture/plugin/plugin-system.md` - registration and discovery
- [ ] `ai/rules/plugins.md` - what a plugin owns, and the directive that a declaration must be answerable without activating
- [ ] `ai/patterns/plugin.md` - the structural template a runner follows

### RFC Summaries (Scope: protocol)
N-A. No wire protocol and no RFC obligation: the plugin RPC is Ze's own.

**Key insights:** (minimal context to resume after compaction)
- The gap is per FIELD, not per plugin: two fields are answerable with no process and the rest are not.
- Every added field must be a PURE value, or the query answer becomes a second declaration that can disagree with the live one.
- `./le plugin declarations check` already holds the compiled-in commands and pipes against each runner's own literal. A new field owes the same pinning.

## Current Behavior (MANDATORY)

**Source files read:** (2026-09-09, at the producers)
- [ ] `pkg/plugin/rpc/types.go` - `DeclareRegistrationInput` and every field it carries
- [ ] `internal/component/plugin/registry/registry.go` - `Registration` carries `Commands` and `Pipes` and no other declaration field
- [ ] `internal/component/plugin/cli/main.go` - `answerQuery` builds the input from those two fields alone, and says so in a comment naming this spec
- [ ] `internal/component/plugin/declarations.go` - `inTreeDeclarationRow` copies the same two

**Behavior to preserve:**
- One declaration per fact. A field added to `Registration` must be the ONE place the plugin states it, read by the runner's own `p.Run` call as well as by the query.
- A plugin the binary carries is answered with no process started.

**Behavior to change:**
- The fields with no compiled-in twin gain one, and the query answer carries them.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `show plugin declarations`, answered from `registry.Registration` for a plugin the binary carries.

### Transformation Path
(fill during design: which field moves onto `Registration`, and how each runner's `p.Run` call site reads it from there instead of restating it.)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| runner literal -> registration | (fill during design) | [ ] |
| registration -> query answer | `inTreeDeclarationRow` | [ ] |
| registration -> live Stage 1 | (fill during design) | [ ] |

### Integration Points
- `internal/component/plugin/registry/registry.go`
- `internal/le/plugin/declarations` - the check that pins the two declarations against each other

### Architectural Verification
(fill during design)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every remaining Stage 1 field is a pure value at the `p.Run` call site | 122 call sites read 2026-09-08 for `spec-plugin-query-mode`; every field value was a package constant or a pure function except `runSDKMode`'s `Families` | A field cannot be answered without configuration, so it stays out | A re-read of the call sites at design time | unvalidated |
| A-2 | An operator wants the other fields | The owner asked what a plugin declares; the answer today is two fields of nine | The gap is documentation rather than code | Ask Thomas | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A field stated in two places drifts | The query answer and the live Stage 1 disagree | The registration is the ONE declaration and the runner reads it; `./le plugin declarations check` gains the field |
| R-2 | The row grows too wide to read | The table rendering wraps | Shape and column order are declared, so a new field can be off the default column list |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `show plugin declarations` typed at the CLI | -> | the added field on `registry.Registration`, read by `inTreeDeclarationRow` | (fill during design) |
| a plugin's own `p.Run` call | -> | the same field, read from the registration rather than restated | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | (fill during design) | (fill during design) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill during design) | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (fill during design) | `test/plugin/` | | |

### Interop Tests (Scope: protocol)
N-A. No wire-visible change: the fields already travel in the Stage 1 message.

## Files to Modify
- `internal/component/plugin/registry/registry.go` - the fields
- `internal/component/plugin/cli/main.go` - `answerQuery`
- `internal/component/plugin/declarations.go` - `inTreeDeclarationRow`
- `docs/features/introspection.md`, `docs/guide/command-reference.md` - what the answer carries

## Implementation Steps

1. **Phase: Audit.** Re-read the 122 `p.Run` call sites and record, per field, whether its value is pure.
2. (fill during design)

## Known Limitations
- (fill during design)

## Checklist

### Goal Gates (MUST pass)
- [ ] Every AC demonstrated
- [ ] Wiring Test table complete
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior
