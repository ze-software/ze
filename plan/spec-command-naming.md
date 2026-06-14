# Spec: command-naming

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-06-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `ai/rules/cli-grammar.md` - verb-noun-action grammar
3. `ai/rules/naming.md` - naming conventions

## Task

Fix command names that violate the CLI grammar or naming conventions. Audit
found 21 commands across 6 plugins with issues in two categories.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/cli-grammar.md` - verb-noun-action grammar, typed selectors
- [ ] `ai/rules/naming.md` - naming conventions
- [ ] `ai/rules/plugin-self-containment.md` - plugin owns its command surface

### RFC Summaries (MUST for protocol work)
N/A -- not protocol work.

**Key insights:**
- BGP plugins must namespace under `bgp` for consistency with rib and filter_irr
- Hyphens join multi-word tokens, not component+selector

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` - 8 commands without `bgp` prefix
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` - 6 commands without `bgp` prefix
- [ ] `internal/component/bgp/plugins/healthcheck/healthcheck.go` - 2 commands without `bgp` prefix
- [ ] `internal/component/bgp/plugins/watchdog/watchdog.go` - 2 commands without `bgp` prefix
- [ ] `internal/component/bgp/plugins/rs/server.go` - 2 commands without `bgp` prefix
- [ ] `internal/plugins/fib/p4/register.go` - fused fib-p4 token
- [ ] `internal/plugins/fib/vpp/register.go` - fused fib-vpp token
- [ ] `internal/plugins/fib/kernel/register.go` - fused fib-kernel token

**Behavior to preserve:**
- All commands continue to execute correctly
- Cross-plugin dispatches continue to work (after updating dispatch strings)

**Behavior to change:**
- BGP plugin commands gain `bgp` prefix
- fib commands split into two tokens

## Data Flow (MANDATORY)

### Entry Point
- CLI: user types command string
- Cross-plugin: plugin dispatches command via dispatch-command RPC

### Transformation Path
1. User types command (e.g., `show bgp adj-rib-in status`)
2. Dispatcher tokenizes and matches against builtins then plugin registry
3. Plugin registry lookup uses lowercased command name as key
4. Matched command routes to owning plugin process

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI → Dispatcher | tokenized command string | [ ] |
| Dispatcher → Plugin | ExecuteCommand RPC with command name | [ ] |
| Plugin → Plugin (cross-dispatch) | dispatch-command RPC with command string | [ ] |

### Integration Points
- CommandRegistry lookup keys change with renamed commands
- Cross-plugin dispatch strings must match new names

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Issues Found

### 1. Missing `bgp` prefix (5 BGP plugins, 18 commands)

These live under `internal/component/bgp/plugins/` but register without `bgp` namespace:

| Plugin | Current | Proposed |
|--------|---------|----------|
| adj_rib_in (8 cmds) | `show adj-rib-in ...`, `request adj-rib-in ...` | `show bgp adj-rib-in ...`, `request bgp adj-rib-in ...` |
| rpki (6 cmds) | `show rpki ...`, `request rpki ...` | `show bgp rpki ...`, `request bgp rpki ...` |
| healthcheck (2 cmds) | `show healthcheck`, `clear healthcheck` | `show bgp healthcheck`, `clear bgp healthcheck` |
| watchdog (2 cmds) | `request watchdog ...` | `request bgp watchdog ...` |
| rs (2 cmds) | `show rs ...` | `show bgp rs ...` |

### 2. Hyphen misuse (fib, 3 commands)

| Current | Problem | Proposed |
|---------|---------|----------|
| `show fib-p4` | Fuses component+backend into one token | `show fib p4` |
| `show fib-vpp` | Same | `show fib vpp` |
| `show fib-kernel` | Same | `show fib kernel` |

## Cross-Plugin Dispatch Dependencies

Renaming requires updating string-based cross-plugin dispatches:

| Caller | Dispatches | To |
|--------|------------|-----|
| rpki plugin (`rpki.go:218`) | `request adj-rib-in enable-validation` | adj_rib_in |
| rs plugin (`server_handlers.go:32`) | `request adj-rib-in replay` | adj_rib_in |
| healthcheck plugin (`healthcheck.go:283-297`) | `request watchdog announce`, `request watchdog withdraw` | watchdog |

## Backward Compatibility

Per `ai/rules/cli-grammar.md` "Backward Compatibility": Ze has never been released.
No users. Replace outright, no deprecation branches.

However, `CommandDecl` supports `DeprecatedNames` for transition. Whether to use it
is a user decision.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Cross-plugin dispatches use string matching only | Code read | Need to find additional coupling | grep for old command names | unvalidated |
| A-2 | No external tools or scripts reference old command names | Ze unreleased | Functional tests may use old names | grep test/ for old names | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Missed cross-plugin dispatch string | Command fails at runtime | Functional test catches it |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| User types renamed command | → | Plugin receives correct command | TestRenamedCommandsDispatch |
| Cross-plugin dispatch uses new name | → | Target plugin executes | TestCrossPluginDispatchRenamed |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | All BGP plugin commands | Use `bgp` prefix in command name |
| AC-2 | fib commands | Two tokens (`show fib p4`) not one (`show fib-p4`) |
| AC-3 | Cross-plugin dispatches | Updated to use new names |
| AC-4 | Functional tests | Updated to use new command names |
| AC-5 | All renamed commands | Still execute correctly end-to-end |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRenamedCommandsDispatch` | `internal/component/plugin/server/command_registry_test.go` | AC-1, AC-2: new names register and dispatch | |
| `TestCrossPluginDispatchRenamed` | `internal/component/plugin/server/command_registry_test.go` | AC-3: cross-plugin strings updated | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A -- no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing plugin functional tests | `test/plugin/*.ci` | Commands with new names execute correctly | |

### Interop Tests (MANDATORY for protocol features)
N/A -- not protocol work.

### Future (if deferring any tests)
- None planned

## Files to Modify
- `internal/component/bgp/plugins/adj_rib_in/rib.go` - rename 8 commands
- `internal/component/bgp/plugins/rpki/rpki.go` - rename 6 commands, update dispatch string
- `internal/component/bgp/plugins/healthcheck/healthcheck.go` - rename 2 commands, update dispatch strings
- `internal/component/bgp/plugins/watchdog/watchdog.go` - rename 2 commands
- `internal/component/bgp/plugins/rs/server.go` - rename 2 commands
- `internal/component/bgp/plugins/rs/server_handlers.go` - update dispatch string
- `internal/plugins/fib/p4/register.go` - rename to `show fib p4`
- `internal/plugins/fib/vpp/register.go` - rename to `show fib vpp`
- `internal/plugins/fib/kernel/register.go` - rename to `show fib kernel`
- `test/plugin/*.ci` - update command strings in functional tests

## Files to Create
- None expected (renames only)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify -- verify current names |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Rename commands, update dispatches |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |

### Implementation Phases

1. **Phase: Rename BGP plugin commands** -- add `bgp` prefix to all 18 commands
   - Tests: TestRenamedCommandsDispatch
   - Files: adj_rib_in, rpki, healthcheck, watchdog, rs source files
   - Verify: commands register with new names
2. **Phase: Update cross-plugin dispatches** -- fix dispatch strings
   - Tests: TestCrossPluginDispatchRenamed
   - Files: rpki.go, server_handlers.go, healthcheck.go
   - Verify: cross-plugin commands still execute
3. **Phase: Rename fib commands** -- split fused tokens
   - Tests: TestRenamedCommandsDispatch
   - Files: fib/p4, fib/vpp, fib/kernel register.go
   - Verify: commands register with new names
4. **Phase: Update functional tests** -- fix command strings in .ci files
   - Files: test/plugin/*.ci
   - Verify: all functional tests pass
5. **Full verification** -- `make ze-verify`

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-command-naming.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-command-naming.md`
