# Spec: decouple-1-cli-contract

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-decouple-0-umbrella |
| Phase | - |
| Updated | 2026-04-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `.claude/patterns/registration.md`
4. `plan/spec-decouple-0-umbrella.md` (parent spec, must be done first)

## Task

Introduce `internal/component/cli/contract/` to decouple ssh and web from cli's concrete types. Both ssh and web are transport layers that host the cli editor -- the dependency direction is correct but the coupling is deep and concrete. A contract package provides interfaces that ssh and web depend on, while cli provides the implementation.

This is Phase 2 of the decoupling effort. Phase 1 (spec-decouple-0-umbrella) must be complete first because it moves auth out of ssh and server creation out of the loader, simplifying what remains.

## Context

After Phase 1, the remaining cross-component coupling between presentation layers is:

| Source | Imports | Concrete types used |
|--------|---------|---------------------|
| ssh/session.go | cli | `cli.Model`, `cli.NewEditorWithStorage`, `cli.NewEditSession`, `cli.NewModel`, `cli.NewCommandModel`, `cli.NewCommandCompleter`, `cli.ValidateUser` |
| ssh/ssh.go | cli | `cli.MonitorFactory`, `cli.Editor` types in server struct fields |
| ssh/warnings.go | cli | `cli.LoginWarning` |
| web/cli.go | cli | `cli.NewCommandModel`, cli types for web CLI interface |
| web/editor.go | cli | `cli.NewEditorWithStorage`, `cli.NewEditSession`, `cli.NewModel`, editor types |

## Design Direction

The `cli/contract/` package will contain:

| Type | Kind | Purpose |
|------|------|---------|
| `Editor` | Interface | Config editing operations (load, save, validate, diff) |
| `Model` | Interface | TUI model (Update, View, Init) for both edit and command modes |
| `EditSession` | Interface or struct | Session identity (username, origin) |
| `LoginWarning` | Struct (plain data) | Warning message with peer/detail fields |
| `MonitorFactory` | Func type | Creates monitor sessions |
| `DashboardFactory` | Func type | Creates dashboard views |
| `CommandCompleter` | Interface | Command completion provider |

**Constraint:** `cli/contract/` must have ZERO imports from `cli/` or any other component. It contains only interfaces, func types, and plain value structs.

**Implementation pattern:** cli implements these interfaces on its existing types. ssh and web import `cli/contract/` instead of `cli/`. Hub wires concrete cli types into ssh and web at startup (hub already imports cli).

## Required Reading

- [ ] `internal/component/ssh/session.go` -- understand full surface area of cli usage
- [ ] `internal/component/ssh/ssh.go` -- server struct fields using cli types
- [ ] `internal/component/web/editor.go` -- web editor's cli usage
- [ ] `internal/component/web/cli.go` -- web CLI interface
- [ ] `internal/component/cli/editor.go` -- understand what Editor exposes
- [ ] `internal/component/cli/model*.go` -- understand Model interface surface
  -> Constraint: interface must be narrow enough that ssh/web only depend on methods they use (interface segregation)

**Key insights:** (to be filled during design phase)

## Current Behavior (MANDATORY)

**Source files read:** (to be filled during design phase)
- [ ] `internal/component/ssh/session.go` -- cli type usage in SSH sessions
- [ ] `internal/component/ssh/ssh.go` -- cli types in server struct
- [ ] `internal/component/web/editor.go` -- cli types in web editor
- [ ] `internal/component/web/cli.go` -- cli types in web CLI
- [ ] `internal/component/cli/editor.go` -- Editor type surface area
- [ ] `internal/component/cli/model.go` -- Model type surface area

**Behavior to preserve:**
- SSH editor sessions work identically
- Web editor sessions work identically
- Command mode completion works
- Monitor/dashboard factory wiring works
- Login warnings display on connect

**Behavior to change:**
- ssh and web import `cli/contract/` instead of `cli/`
- Hub injects concrete cli implementations into ssh and web at startup

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- SSH connection or HTTP request arrives at transport layer (ssh or web)
- Transport needs a cli editor/model instance to serve the session

### Transformation Path
1. Hub creates concrete cli types at startup
2. Hub injects them into ssh/web via setter or constructor
3. ssh/web use contract interfaces to interact with cli
4. cli implements the interfaces on its existing types

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| hub -> ssh/web | Concrete cli types passed as contract interfaces | [ ] |
| ssh/web -> cli/contract | Interface method calls only | [ ] |

### Integration Points
- `cmd/ze/hub/main.go` -- wires concrete types into ssh and web
- `internal/component/cli/` -- implements contract interfaces

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | Arrow | Feature Code | Test |
|-------------|---|--------------|------|
| SSH login -> editor session | -> | cli/contract.Editor via ssh | Existing SSH functional tests |
| Web login -> editor session | -> | cli/contract.Editor via web | Existing web functional tests |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | grep for `component/cli"` in ssh/*.go (non-test, production code) | Zero matches (only `cli/contract` imports) |
| AC-2 | grep for `component/cli"` in web/*.go (non-test, production code) | Zero matches (only `cli/contract` imports) |
| AC-3 | `cli/contract/*.go` has zero imports of other components | Verified by grep |
| AC-4 | SSH editor session works | Existing ssh tests pass |
| AC-5 | Web editor session works | Existing web tests pass |
| AC-6 | `make ze-verify` passes | All tests green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestContractInterfaceSatisfied` | `internal/component/cli/contract_verify_test.go` | cli types implement contract interfaces | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A -- no numeric inputs | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing SSH tests | `test/` | SSH editor sessions work after contract introduction | |
| Existing web tests | `test/` | Web editor sessions work after contract introduction | |

### Future
- None -- all tests covered

## Files to Modify

- `internal/component/cli/contract/` -- New: interface package
- `internal/component/ssh/session.go` -- Change imports to cli/contract
- `internal/component/ssh/ssh.go` -- Change imports to cli/contract
- `internal/component/ssh/warnings.go` -- Change imports to cli/contract
- `internal/component/web/cli.go` -- Change imports to cli/contract
- `internal/component/web/editor.go` -- Change imports to cli/contract
- `cmd/ze/hub/main.go` -- Wire concrete cli types into ssh and web

## Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` -- note cli/contract pattern |

## Implementation Steps

(To be detailed when spec moves to design/ready status)

1. **Phase: Define contract interfaces** -- Read all cli usage from ssh and web, extract minimal interface set
2. **Phase: Implement in cli** -- Verify cli types satisfy contract interfaces (compile-time check)
3. **Phase: Update ssh** -- Replace cli imports with cli/contract, accept interfaces via constructor/setter
4. **Phase: Update web** -- Same as ssh
5. **Phase: Wire in hub** -- Hub passes concrete cli implementations to ssh and web
6. **Full verification** -- `make ze-verify`

### Critical Review Checklist
| Check | What to verify |
|-------|----------------|
| Interface segregation | ssh and web depend only on methods they use |
| No identity wrapper | contract types add value (decouple), not just delegate |
| Minimal surface | Contract has the minimum types needed |
| No cli import in contract | contract/ has zero component imports |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero cli imports in ssh (non-test) | grep |
| Zero cli imports in web (non-test) | grep |
| Zero component imports in cli/contract | grep |
| `make ze-verify` passes | Run and paste output |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Session identity | EditSession still carries username/origin through the contract interface |

## Files to Create
- `internal/component/cli/contract/contract.go` -- interface definitions
- `internal/component/cli/contract_verify_test.go` -- compile-time interface check

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Implementation Summary

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/540-decouple-1-cli-contract.md`
- [ ] Summary included in commit
