# Spec: resolve and origin over rows keyed by identity

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Depends | spec-cli-show-bgp-answer-shapes |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Four `show bgp` commands answer rows keyed by IDENTITY, and neither `resolve`
nor `origin` can decorate the address those rows are keyed by. The address is
the MAP KEY, and both operators act on a FIELD.

The four commands are `show bgp peer list`, `show bgp peer detail`, `show bgp rib
status` and `show bgp adj-rib-in`. Each answers a map whose key is the peer
address. An operator who types `show bgp peer list | resolve` gets no reverse DNS
name for any peer, because no field of the row holds the address: the row is
UNDER the address.

`spec-cli-show-bgp-answer-shapes` found this on 2026-08-24 and recorded it. The
row states what is already built and what is missing: `rowsInKeyed`
(`internal/component/command/answer_shape.go`) already answers the identity keys,
so the mechanism exists on the row side. What is missing is a DECISION about what
the decorated key looks like in the rebuilt answer, which is a change to the
operators rather than a declaration a command makes.

That decision is the substance of this spec. A field gains a sibling `<key>-name`
beside it. A map key has no sibling position: decorating it means either
rewriting the key itself, which breaks a caller that indexes the answer by peer
address, or moving the decoration inside the row, which puts a field on a row
that did not declare one. The spec must pick one and say why, for `resolve` and
`origin` together, since both walk the same JSON.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - the answer shapes and which operators each publishes
  → Constraint: [fill at design time]
- [ ] `ai/rules/cli.md` - every command supports all pipe operators
  → Constraint: [fill at design time]

**Key insights:** (minimal context to resume after compaction)
- The blocker is a rendering decision, not a missing mechanism: `rowsInKeyed` already yields the keys
- `resolve` and `origin` share one JSON walk, so a fix to one is a fix to both

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/command/pipe_resolve.go` - `resolveJSON` walks the answer with an explicit stack. For a `map[string]any` it iterates `addressObjectFields(val)` and reads `val[key]`. It decorates only when that VALUE is a string, is not `*`, parses as an address, and the key is a declared address field. The decoration is a sibling `<key>-name` holding `ReverseLookup(s)`. The map KEY is never examined, so an address in key position is never seen. `applyOrigin` runs the same walk
- [ ] `internal/component/command/pipe.go` - the `pipeResolve` and `pipeOrigin` arms refuse the operator with `addressOperatorRefusal` when the command declares no address field. The refusal reads "no field of this command's answer is declared to hold an IP address", which is true and is exactly the state these four commands are in
- [ ] `internal/component/command/answer_shape.go` - `rowsInKeyed` returns the rows, the keys, and the name of the key field for a map-shaped answer, so the identity keys are already reachable from the operator side

**Behavior to preserve:**
- The sibling `<key>-name` shape for a field that already works
- The refusal text for a command that genuinely holds no address anywhere

**Behavior to change:**
- The four identity-keyed answers must resolve the address they are keyed by

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator types `show bgp peer list | resolve` (or `| origin`) at the CLI, or the same command over the API.
- Format at entry: the dispatcher's JSON answer, a map whose keys are peer addresses and whose values are row objects.

### Transformation Path
1. `ProcessPipes` (`pipe.go`) dispatches the `pipeResolve` arm
2. `applyResolve` parses the JSON and hands it to `resolveJSON`
3. `resolveJSON` walks maps and slices, decorating declared address fields
4. The result is re-encoded and rendered by the format operator that follows

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Command dispatcher ↔ pipe operators | JSON answer text | No |

### Integration Points
- `rowsInKeyed` (`answer_shape.go`) - already answers the identity keys the operators need
- `declaredAddressField` (`pipe_resolve.go`) - the declaration test the new path must agree with

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
| A-1 | A caller indexes these answers by peer address, so rewriting the key breaks it | The key IS the identity | The simplest decoration is available | Grep the web and API readers of these four answers | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Decorating inside the row collides with a field the row already holds | A row gains two fields with one name | Refuse the collision, or name the derived field after the key |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The JSON shape of four `show bgp` answers changes under every reader |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `plan/immediate/spec-cli-show-bgp-answer-shapes.md`, `plan/spec-plugin-declares-answer-shape.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show bgp peer list \| resolve` | → | the identity-key arm of `resolveJSON` | [test name, fill at design time] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show bgp peer list \| resolve` over an answer keyed by peer address | Each row carries the reverse DNS name of the address it is keyed by |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Reads the peer list with names beside the addresses | CLI → dispatcher → `ProcessPipes` → `resolveJSON` → table | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [fill at design time] | `internal/component/command/render_records_test.go` | [description] | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | N-A | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [fill at design time] | `test/decode/*.ci` | An operator resolves the peer list | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | Scope is cli, no wire-visible change | |

## Files to Modify
- `internal/component/command/pipe_resolve.go` - the identity-key arm of the walk
- `internal/component/command/pipe.go` - the refusal, which must stop firing for these four answers

## Files to Create
- [fill at design time]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | |
| YANG validation constraints | | |
| YANG custom validators | | |
| CLI commands/flags | | |
| CLI grammar (keyword before value) | | `ai/rules/cli.md` |
| Editor autocomplete | | |
| Functional test for new RPC/API | | `test/decode/*.ci` |
| Pipe completeness | | `ai/rules/cli.md` |
| Env var registration | | |
| Doctor check for runtime dependencies | | |
| Prometheus counters/metrics | | |
| BGP family surface (new SAFI / capability / attribute) | | N-A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | `docs/features.md` |
| 2 | Config syntax changed? | | |
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | | |
| 6 | Has a user guide page? | | |
| 7 | Wire format changed? | | |
| 8 | Plugin SDK/protocol changed? | | |
| 9 | RFC behavior implemented, changed, or newly proven? | | |
| 10 | Test infrastructure changed? | | |
| 11 | Affects daemon comparison? | | |
| 12 | Internal architecture changed? | | |
| 13 | Route metadata keys added/changed? | | |
| 14 | Prometheus counters added/changed? | | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | |
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED: `./le spec citation anchors spec plan/immediate/spec-show-bgp-operators-over-identity-keyed-rows.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- reach the identity keys from the operator, write failing wiring tests
   - Tests: [wiring test names]
   - Files: `pipe_resolve.go`, `pipe.go`
   - Verify: the operator no longer refuses these four answers, and the wiring test fails because nothing is decorated yet
2. **Phase: [name]** -- [fill at design time]

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Data flow | One walk serves `resolve` and `origin`; neither gains its own copy |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| [fill at design time] | [command] |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A key that is not an address must not be decorated or rewritten |

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
