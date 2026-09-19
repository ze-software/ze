# Spec: declared commands take values their command nodes do not declare

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The 2026-08-08 inventory found 30 declared command paths with a `<value>`
placeholder and no command leaf behind it. That count is historical. The
generated-usage migration has since added argument declarations and inheritance,
so this spec owns the residual per-command modelling work, not a second copy
of those completed migrations.

`extractArgDefs` (`internal/component/config/yang/command.go`) is the producer.
It walks the entry's directory and builds a `command.ArgDef` from a leaf, and
only from a leaf: it asks `declaredLeafNames` for the entry's `leaf` and
`leaf-list` sub-statements, calls `argDefFor` on each, and appends the ones that
answer true. Nothing in it reads a description. A command with no leaf therefore
produces no `ArgDef` at all, and `ArgDefs` is what completion and validation
both read.

Two corrections the shard recorded, and both matter to whoever picks this up:

**Trailing-value resolution was fixed; the missing model still has consumers.**
The August 30 triage called this an improvement because typed values could
reach the daemon through `endsDeclaredCommand` and `cli.AbsoluteVerbPath`.
That observation concerns direct CLI dispatch. It does not establish that
completion, argument validation or a generated web form can supply the value.
`buildAdminFragmentData` and `commandArguments`
(`internal/component/web/handler_admin.go`) both read `node.ArgDefs`, so a
command with no effective argument definitions offers no field and submits no
argument.

**Positional order CAN be expressed, so the stated reason nobody picked this up
was false.** The shard originally said `extractArgDefs` sorts by name and cannot
express positional order, so a two-value command could not be declared
correctly. Corrected at the producer on 2026-08-30: `declaredLeafNames` returns
the leaves in module declaration order, and `extractArgDefs` consumes that order
first, falling back to a sorted name order only for a leaf that reaches the
entry from a grouping or an augment.

Declaring the residual leaves is per-command design work. Each needs a name,
a type and native validation that agrees with the handler. The current source
still gives `show command help` and `show command complete` no leaves in
`internal/plugins/meta/yang/ze-command-meta-cmd.yang`, while
`handleBgpCommandHelp` and `handleBgpCommandComplete` refuse an empty argument
list. These are first-release missing answers on the web admin surface, which
justifies retaining the repair in `immediate/`.

The September 15 journal row in
`plan/journal/command-takes-an-untyped-positional-value.md` supplies a residual
candidate list: `request commit`, `request peer borr`, `request peer eorr`,
`request peer clear soft`, `request subscribe`, `system dispatch`,
`show command help`, `show command complete`, `plugin ack`, `plugin encoding`,
`plugin format`, `resolve irr expand` and `resolve irr prefix`. It is a dated
list to reconcile against the merged tree, not a claim that all thirteen still
lack every argument. In particular, the refresh module says its selector is
inherited from `request peer`; a missing local leaf alone proves no defect.

Before implementation, derive the effective definitions after inheritance and
subtract paths already repaired by
`plan/immediate/spec-generated-command-usage.md`. Keep completion and
type-check improvements for otherwise reachable commands in this spec, but
separate their release cost from confirmed missing-input defects. No current
total is asserted until that merged-tree inventory is recorded.

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/config-option.md` - the leaf template and its native validation
  → Constraint: <to be filled>
- [ ] `ai/rules/cli.md` - keyword before value, and what a command owes its operator
  → Decision: <to be filled>

**Key insights:** (minimal context to resume after compaction)
- <to be filled>

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/config/yang/command.go` - `extractArgDefs` builds `ArgDef` values from `leaf` and `leaf-list` sub-statements only, in the order `declaredLeafNames` returns them, and reads no description; `argDefFor` refuses a leaf with no type
- [ ] `cmd/ze/internal/cmdutil/cmdutil.go` - `endsDeclaredCommand` asks `cli.AbsoluteVerbPath`, so a trailing value reaches the daemon whether or not a leaf declares it

**Behavior to preserve:** (unless the user explicitly said to change it)
- trailing-value resolution for accepted direct CLI invocations, including the original inventory

**Behavior to change:** (only what the user asked for)
- <to be filled>

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- a YANG `ze:command` entry, read at schema load
- an operator typing a command with a trailing value, in the CLI or through `ze <verb>`

### Transformation Path
1. <to be filled>

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG schema ↔ command registry | `command.ArgDef` | No |

### Integration Points
- the completion surface, which reads `ArgDefs`
- the daemon-side argument validator, which reads the same

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Registration over hardcoding | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | the residual population is captured by the historical inventories | counts from 2026-08-08 and 2026-09-15 precede later model changes | paths already repaired or still missing are miscounted | derive effective arguments from the current merged command tree and compare each residual handler's grammar | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | a new leaf tightens validation and refuses a value an operator types today | a `.ci` fixture goes red | <to be filled> |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a command an operator types is refused by its own schema |
| How is it reverted? | <to be filled> |
| Who else touches this path? | `spec-generated-command-usage`, `spec-cli-root-namespace-grammar-deferred-gate-reach` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| tab completion on a declared command's value | → | <to be filled> | <to be filled> |
| an operator fills and submits a residual command's web admin form | → | `buildAdminFragmentData`, `commandArguments`, then the existing handler | a functional web case covering a value the bare command cannot supply |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | an operator asks for completion at a declared command's value position | the completion offers the leaf's type or its enumeration |
| AC-2 | an operator types a value the leaf's type refuses | the daemon answers a validation error naming the offending value |
| AC-3 | a two-value command | the values are read in module declaration order |
| AC-4 | an operator opens and submits the web admin form for an affected command | The form supplies every required argument and the handler receives an accepted invocation; a missing or invalid required value is refused rather than silently omitted |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | completes the value of a declared command | <to be filled> | <to be filled> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| <to be filled> | `internal/component/config/yang/` | <to be filled> | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| <to be filled> | `test/` | <to be filled> | |

## Files to Modify
- `internal/component/config/yang/command.go` - <to be filled>
- the residual command modules identified from the merged-tree inventory, with missing leaves added without duplicating inherited arguments

## Files to Create
- <to be filled>

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | the owning plugin's `yang/` |
| YANG validation constraints | | every new leaf takes its native validation |
| Editor autocomplete | | automatic for enum and typed leaves |
| CLI grammar (keyword before value) | | `ai/rules/cli.md` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- re-measure the set and list every path
2. **Phase: <to be filled>**

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every path in the re-measured list has a leaf, or a recorded reason it needs none |
| Naming | the leaf name matches the placeholder the description used |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| no declared command carries a description placeholder with no leaf | a check over the loaded schema |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior |

## Known Limitations
- <to be filled>

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes
