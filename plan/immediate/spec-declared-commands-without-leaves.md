# Spec: 30 declared commands take a value no leaf declares

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

30 unique declared command paths carry a `<value>` placeholder in their YANG
description and no leaf behind it. Counted on 2026-08-08, while trailing-value
resolution in `ze <verb>` was being repaired. An operator typing one of those
commands gets no completion for the value and no daemon-side argument
validation, because a placeholder written in a description reaches neither
surface.

`extractArgDefs` (`internal/component/config/yang/command.go`) is the producer.
It walks the entry's directory and builds a `command.ArgDef` from a leaf, and
only from a leaf: it asks `declaredLeafNames` for the entry's `leaf` and
`leaf-list` sub-statements, calls `argDefFor` on each, and appends the ones that
answer true. Nothing in it reads a description. A command with no leaf therefore
produces no `ArgDef` at all, and `ArgDefs` is what completion and validation
both read.

Two corrections the shard recorded, and both matter to whoever picks this up:

**Resolution is FIXED for all 30, so nothing is broken on the wire.**
`endsDeclaredCommand` (`cmd/ze/internal/cmdutil/cmdutil.go`) keys the trailing
boundary on `cli.AbsoluteVerbPath`, which reads the same two registrations the
daemon's dispatcher is keyed on, so no leaf is needed for a typed value to reach
the daemon. The row was triaged on 2026-08-30 as an improvement rather than a
release defect: what is missing is the operator's completion and the argument
type-check.

**Positional order CAN be expressed, so the stated reason nobody picked this up
was false.** The shard originally said `extractArgDefs` sorts by name and cannot
express positional order, so a two-value command could not be declared
correctly. Corrected at the producer on 2026-08-30: `declaredLeafNames` returns
the leaves in module declaration order, and `extractArgDefs` consumes that order
first, falling back to a sorted name order only for a leaf that reaches the
entry from a grouping or an augment.

Declaring the leaves is per-command DESIGN work rather than a sweep. Each of the
30 needs a name, a type from `ze-types.yang`, and whatever native validation the
value admits. The first step this spec owes is a fresh count and the list, since
the 30 was measured on 2026-08-08 and the command tree has moved since.

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
- trailing-value resolution for all 30 paths, which works today

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
| A-1 | the count is still 30 | measured 2026-08-08 | the list changes | re-measure over the current tree | unvalidated |

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

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | an operator asks for completion at a declared command's value position | the completion offers the leaf's type or its enumeration |
| AC-2 | an operator types a value the leaf's type refuses | the daemon answers a validation error naming the offending value |
| AC-3 | a two-value command | the values are read in module declaration order |

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
- the 30 YANG modules declaring the commands - add the missing leaves

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
