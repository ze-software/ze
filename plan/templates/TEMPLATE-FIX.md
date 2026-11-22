# Spec: <fix-name>

<!-- COMPACT FIX SPEC template: for focused defect fixes, small improvements,
     and single-component repairs that do not add new protocol wire formats,
     new subsystems, or architecture departures. For new features, protocol
     additions, or architectural changes, use plan/templates/TEMPLATE.md.
     The closure half lives in plan/templates/TEMPLATE-CLOSURE.md and is
     appended by /ze-close at step 1. -->

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol \| cli \| plugin \| config \| tooling \| docs |
| Priority | P0 \| P1 \| P2 \| P3 |
| Release reason | <why this fix belongs in its release category> |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | YYYY-MM-DD |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

<symptom, root cause at the producing function, and intended observable fix>

## Required Reading

- [ ] `ai/doors/<subsystem>.md` - subsystem boundaries and gates
  → Constraint: [binding constraint for this fix]
- [ ] `docs/architecture/<doc>.md` - [why relevant]
  → Decision: [relevant architectural decision]

## Current Behavior

**Source files read:**
- [ ] `path/to/file.go` - [what the producing function currently does]

**Behavior to preserve:**
- [existing contract callers depend on]

**Behavior to change:**
- [exact defect behavior being fixed]

## Existing Mechanisms

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| [mechanism] | [file + symbol] | [current behavior] | [extends: why] |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | [section] | always | [verbatim quote] | agrees | [how this fix holds it] |

## Data Flow

### Entry Point
- [Where the triggering input enters]

### Transformation Path
1. [Entry -> producing function -> observable output]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| [caller -> producer] | [signature / format] | No |

### Integration Points
- [Existing caller or registry this fix connects to]

## Risks & Assumptions

| ID | Assumption / Risk | Basis / Early signal | Validation / Mitigation | Status |
|----|-------------------|----------------------|-------------------------|--------|
| A-1 | [assumption] | [source file] | [test] | unvalidated |

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [command / config / message] | → | [producing function] | [test name proving reachability] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | [triggering input] | [positive observable outcome after fix] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `internal/.../xxx_test.go` | [reproduces bug red, passes green after fix] | |

### Functional Tests (when operator-facing)
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [or N-A for internal-only fix] | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when Scope is protocol: a fix that changes protocol behavior
     carries an interop test against at least one other implementation
     (ai/rules/interop-and-goal-validation.md), including the vacuity traps:
     prove the test FAILS when the fix is reverted. Delete this section only
     for another Scope. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `feature-peer` | `test/interop/scenarios/` | [FRR/BIRD/GoBGP/strongSwan] | [protocol behavior validated] | |

## Files to Modify
- `internal/...` - [producing fix]

## Files to Create
- [new test file, or None]

### Integration Checklist
- [ ] Applicable surfaces checked (YANG, CLI grammar, doctor check, docs) or N-A: [reason]

### Documentation Update Checklist
- [ ] Updated any doc whose `<!-- source: -->` or `// Design:` claim changed (or verified unaffected via `./le spec citation anchors spec <path>`)

## Design presentation

### What changes
- [one-sentence summary of the behavioral fix]

### Before and after
- **Before:** [failing input and defective output]
- **After:** [same input and corrected output]

### Alternatives set aside
- [simpler or wider alternative considered and why it was set aside]

### Documents departed from
none

### Choices for Thomas
none

## Implementation Steps

1. **Phase: Fix and prove** -- write failing regression test reproducing the defect, fix the producing code, verify scoped tests and lint pass.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## RFC Documentation (Scope: protocol)
<!-- REQUIRED when Scope is protocol (ai/rules/rfc-compliance.md). Delete this
     section only for another Scope. -->

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

Assess the fix on the default configuration path, preserve explicit options
and extensions, name the configuration each claim covers, and test both paths.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| [RFC and section] | [exact configuration and observable behavior] | [option paths and behavior, or none with reason] | [test names and proof] | [test names, including any difference from the RFC] |

## Checklist

- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Scoped verification run by main thread: `./le test unit pkg path <dir>/` and `./le go lint run scope "<dir>"`
- [ ] Closure: append `plan/templates/TEMPLATE-CLOSURE.md`, complete `/ze-review` gate, land Commit A + Commit B (`remove <spec-path>`)
