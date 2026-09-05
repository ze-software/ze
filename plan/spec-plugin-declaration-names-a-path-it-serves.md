# Spec: a plugin declares only on a command path it serves

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | spec-plugin-declares-answer-shape |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A plugin can declare an answer shape, or an alias, on a command path it does not
serve. Nothing checks ownership. Both channels take a path from the same Stage 1
message, and both accept it.

Found in Phase 2 of `spec-plugin-declares-answer-shape` on 2026-08-24. The work,
in the row's own words: an ownership check on a declaration, so a plugin declares
only on a command path it actually serves. It covers the alias channel and the
shape channel together, because both take a path from the same Stage 1 message.

The row's own analysis of the seam, kept whole because the shard is being
deleted:

A plugin declares a command, `PluginRegistry.Register` lets Stage 1 through, and
the dispatcher rejects the command later as a builtin conflict while the plugin
keeps running (`docs/architecture/api/commands.md`). The declaration has landed by
then. The conflict rule in `declareFor` closes most of it: a path whose builtin
ALREADY declares a non-empty value refuses the plugin, which after
`spec-cli-show-bgp-answer-shapes` covers every in-tree `show bgp` path. What stays
open is a builtin that declares NOTHING, and most of the tree does: a plugin could
land a shape on `show interface` and change which operators that command publishes
and refuses. The blast radius is rendering and refusal, never what a caller may
run, so the spec's Security Review row stays true and this is not an
authorization hole. The alias channel has the same seam and is weaker in one way
and stronger in another: an alias can only ADD a name to a path, where a shape
REPLACES what the path holds. Fixing one channel and not the other would leave the
seam open and look closed.

Verified at the producer on 2026-09-05: `declareFor`
(`internal/component/command/column_order.go`) takes an owner, a command path and
a value. It refuses an empty path, treats an empty value as a floor, accepts a
value over a floor, accepts a restatement, and refuses a value that disagrees with
a non-empty held value. Ownership is recorded, in `r.byOwner`, so a declaration
can be removed when its owner leaves. It is never CHECKED against what the owner
serves.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - Stage 1 declarations, the builtin conflict, and when the dispatcher rejects a command
  → Constraint: the declaration lands before the dispatcher's rejection
- [ ] `ai/rules/plugins.md` - what a plugin may declare and where
  → Constraint: [fill at design time]

**Key insights:** (minimal context to resume after compaction)
- The blast radius is rendering and refusal, never what a caller may run: this is not an authorization hole
- Both channels take the path from the same Stage 1 message, so they are one fix

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/command/column_order.go` - `(*declarationRegistry[T]).declareFor` refuses only a nameless path and a value that disagrees with a non-empty held value. It records the owner in `byOwner` for later removal, and never compares the path against what that owner serves
- [ ] `internal/component/command/answer_shape.go` - `RegisterPluginShapes` calls `declareFor` once for the shape and once for the column order, and unregisters the whole owner on the first error
- [ ] `internal/component/command/alias.go` - `RegisterPluginAliases` takes a command path per alias and merges the names onto that path's alias set
- [ ] `internal/component/plugin/server/startup.go` - `validateShapeDecls` validates the Stage 1 declarations: the shape spelling, a declaration on no command path, fields without a shape, and the 64-column, 16-address-field and 64-byte-name bounds. It validates the SHAPE of the declaration, never the plugin's right to the path

**Behavior to preserve:**
- The floor-and-value rule in `declareFor`, and the per-owner removal that `byOwner` supports
- The existing Stage 1 validation and its bounds

**Behavior to change:**
- A declaration on a path the declaring plugin does not serve must be refused, on both channels

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A plugin sends its Stage 1 registration message, carrying its command declarations with their names, shapes, columns, address fields and aliases.
- Format at entry: `rpc.CommandDecl` values over the plugin process protocol.

### Transformation Path
1. `validateShapeDecls` and `validatePipeDecls` (`startup.go`) validate the declarations before conversion
2. `onRegistration` converts them and calls `RegisterPluginShapes` and `RegisterPluginAliases`
3. `declareFor` (`column_order.go`) files each value under the path and the owner
4. `ProcessPipes` later reads the path's shape to decide which operators the command publishes

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ plugin host | the Stage 1 registration message | No |
| Plugin host ↔ command registry | `RegisterPluginShapes`, `RegisterPluginAliases` | No |

### Integration Points
- `commandPathKey` (`startup.go`) - the path normalization both channels already share
- `byOwner` (`column_order.go`) - the ownership record the check can read

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The set of paths a plugin serves is known at the moment the declaration is validated | Both come from the same Stage 1 message | The check needs a second round trip or a deferred validation | Read the Stage 1 message shape and the registration order | unvalidated |
| A-2 | A plugin legitimately declares a shape on a path a DIFFERENT plugin serves in no in-tree case | [fill at design time] | The check refuses a working configuration | Grep the in-tree plugins' declarations against the paths they serve | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Refusing at Stage 1 kills a plugin that previously started with a bad declaration | A plugin that used to run stops running after upgrade | Refuse the declaration, not the plugin, and log which declaration was dropped |
| R-2 | Fixing the shape channel alone leaves the alias seam open while looking closed | A review that reads only the shape path | One check, reached by both channels |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A plugin that declares legitimately is refused, and its command renders with the wrong operators |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `plan/spec-plugin-declares-answer-shape.md`, `plan/immediate/spec-cli-show-bgp-answer-shapes.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a Stage 1 message declaring a shape on a path the plugin does not serve | → | the ownership check ahead of `declareFor` | [test name, fill at design time] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A plugin declares a shape on a path it does not serve | The declaration is refused and the refusal names the path |
| AC-2 | A plugin declares an alias on a path it does not serve | The declaration is refused, by the same check |
| AC-3 | A plugin declares on a path it does serve | The declaration is accepted, as today |
| AC-4 | The refused declaration is the only bad one in the message | The plugin keeps running and the other declarations land |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Loads a plugin that declares on `show interface`, which it does not serve | Stage 1 → validation → refusal → `show interface` keeps its operators | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [fill at design time] | `internal/component/plugin/server/startup_test.go` | the ownership refusal on both channels | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | N-A | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [fill at design time] | `test/plugin/*.ci` | A misdeclaring plugin does not change another command's operators | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | Scope is plugin, no wire-visible change | |

## Files to Modify
- `internal/component/plugin/server/startup.go` - the Stage 1 validation that gains the ownership check
- `internal/component/command/answer_shape.go` - the shape channel
- `internal/component/command/alias.go` - the alias channel

## Files to Create
- [fill at design time]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | |
| YANG validation constraints | | |
| YANG custom validators | | |
| CLI commands/flags | | |
| CLI grammar (keyword before value) | | |
| Editor autocomplete | | |
| Functional test for new RPC/API | | `test/plugin/*.ci` |
| Pipe completeness | | |
| Env var registration | | |
| Doctor check for runtime dependencies | | |
| Prometheus counters/metrics | | a counter for refused declarations |
| BGP family surface (new SAFI / capability / attribute) | | N-A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | |
| 2 | Config syntax changed? | | |
| 3 | CLI command added/changed? | | |
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | | |
| 7 | Wire format changed? | | |
| 8 | Plugin SDK/protocol changed? | | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | | |
| 10 | Test infrastructure changed? | | |
| 11 | Affects daemon comparison? | | |
| 12 | Internal architecture changed? | | |
| 13 | Route metadata keys added/changed? | | |
| 14 | Prometheus counters added/changed? | | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | `docs/plugin-overview.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED: `./le spec citation anchors spec plan/spec-plugin-declaration-names-a-path-it-serves.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- reach the set of paths a plugin serves from the declaration validator, write failing wiring tests
   - Tests: [wiring test names]
   - Files: `startup.go`
   - Verify: the validator can ask the ownership question; the wiring test fails because it does not refuse yet
2. **Phase: [name]** -- [fill at design time]

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Data flow | One check serves both channels; neither gains its own copy |
| Rule: `ai/rules/principles.md` | The check fails closed: an unknown path is refused, never accepted by default |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Both channels refuse an unowned path | one test per channel |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Authorization that could fail open | The check must not read an empty served-path set as "serves everything" |

### Failure Routing
| Failure | Route To |
|---------|----------|
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- [fill at closure]

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
