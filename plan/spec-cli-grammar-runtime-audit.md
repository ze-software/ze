# Spec: cli-grammar-runtime-audit

| Field | Value |
|-------|-------|
| Status | design |
| Depends | (parent) cli-grammar-gate -- Feeders 1+2 shipped; see plan/learned/1056-cli-grammar-gate.md |
| Phase | - |
| Updated | 2026-07-03 |

## Post-Compaction Recovery

**Re-read these:**
1. This spec file
2. `plan/learned/1056-cli-grammar-gate.md` -- what the parent shipped
3. `internal/component/command/grammar/checker.go` -- `CheckName`/`ExemptCategory` to reuse
4. `internal/component/plugin/server/system.go:301` -- `handleSystemCommandList` (merged runtime surface)
5. `test/plugin/plugin-command-completion.ci` -- runtime `system command list` dispatch precedent

## Task

Add Feeder 3 to the CLI grammar gate: a runtime audit that boots the daemon, reads the
FULL merged command surface (built-ins + started plugins) via the `ze-system:command-list`
RPC, and validates every command against grammar rules R1-R4 using the existing
`grammar.CheckName` checker; plus a drift guard so the audit config cannot silently miss
a command-registering plugin. The parent shipped Feeders 1 (static) and 2 (registration),
which already enforce grammar on 100% of commands; this feeder is belt-and-suspenders,
validating the actual merged dispatch paths.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/cli-grammar.md` (Mechanical Enforcement) -- the three-feeder model
  → Constraint: reuse `grammar.CheckName`; do not re-implement rules (e.g. in Python).
- [ ] parent `plan/learned/1056-cli-grammar-gate.md`
  → Constraint: additive only; must not reduce parent (Feeder 1/2) coverage.

### RFC Summaries (MUST for protocol work)
N/A -- not protocol work.

**Key insights:**
- `system command list` returns command PATHS only (`Completion{Value,Help,Hidden,Source}`), no `ArgDef` structure -> path-level R1-R4 only; R5-R8 stay with Feeder 1.
- Only STARTED plugins appear; startup is per-plugin config-path driven (no all-plugins config exists = assumption A-3).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/system.go:301` -- `handleSystemCommandList`
  → Constraint: emits `Dispatcher().Commands()` (builtins) + `Registry().All()` (plugins); `source` only with `verbose`.
- [ ] `internal/component/command/grammar/checker.go`
  → Constraint: `CheckName(path)` gives R1-R4 findings; `ExemptCategory(wireMethod)` gives bridge/wire-protocol/editor. The RPC gives no wire method -> exemption by path-prefix heuristic or skip exempt namespaces by first token.

**Behavior to preserve:** the static gate and registration gate remain the primary enforcement.

**Behavior to change:** add a runtime audit + drift guard only.

## Data Flow (MANDATORY)

### Entry Point
- A functional test / audit boots `bin/ze` with the audit config.

### Transformation Path
1. Daemon boots; plugins register commands (Feeder 2 validates each at registration).
2. Test dispatches `system command list --verbose --json`.
3. Test parses the merged command list and runs `grammar.CheckName` on each path.
4. Drift guard asserts every command-registering plugin appears (option a).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Daemon ↔ test | `ze-system:command-list` RPC (path-only) | [ ] |

### Integration Points
- `grammar.CheckName` / `ExemptCategory` (reuse), the audit config, the functional suite.

### Architectural Verification
- [ ] No duplicated rule logic (reuse `grammar`).
- [ ] Additive; parent feeders untouched.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | One config can start every command-registering plugin | spike: none exists; per-plugin config-path start | option (a) infeasible -> use (b) best-effort | build the config; assert dump covers each plugin | broken (spike) |
| A-2 | `system command list --verbose` returns the complete merged surface | `system.go:307-330` | audit incomplete | `.ci`/Go assertion vs floor | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Audit config drifts as plugins are added (option a) | new plugin's commands absent | drift guard AC-2 |
| R-2 | RPC lacks ArgDef depth -> only R1-R4 | - | R5-R8 covered by Feeder 1 (static) |
| R-3 | Daemon boot cost | suite wall-clock | single boot, one dump |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Daemon dump contains a noun-first command | → | audit runs `grammar.CheckName`, fails | `TestRuntimeCommandSurfaceGrammar` (or `.ci`) |
| A command-registering plugin not in the audit config | → | drift guard fails | `TestAuditConfigComplete` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Daemon booted with the audit config | Test dumps `system command list --verbose --json` and runs `grammar.CheckName` on every command path; any R1-R4 violation (non-exempt) fails. |
| AC-2 | A command-registering plugin not started by the audit config (option a) | Drift guard fails, naming the missing plugin. |
| AC-3 | The audit | Reuses `grammar.CheckName`/`ExemptCategory` -- no duplicated rule logic. |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAuditConfigComplete` (drift guard) | TBD | AC-2 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| runtime command-surface grammar audit | `test/<suite>/` | boot -> `system command list` -> validate | |

### Interop Tests (MANDATORY for protocol features)
N/A -- no wire protocol change.

### Future
- Decide option (a) all-plugins config vs (b) best-effort broad config before implementation.

## Files to Modify
- (TBD during design) the functional suite registration if a new suite is used.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI grammar | Yes -- this validates it at runtime | reuse `grammar` |
| Functional test | Yes | `test/<suite>/` |
| Doctor check | No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `ai/rules/cli-grammar.md` (Feeder 3 row) |

## Files to Create
- The audit config (option a or b) under `test/<suite>/`.
- The runtime audit test (Go subprocess or `.ci`).

## Implementation Steps

### Implementation Phases
1. **Decide A-3** (option a vs b) with the user.
2. **Audit config** -- build it.
3. **Runtime audit** -- boot, dump `system command list --verbose --json`, run `grammar.CheckName`.
4. **Drift guard** -- assert coverage (option a).
5. **Docs** -- add the Feeder 3 row to `ai/rules/cli-grammar.md` enforcement table; `docs/functional-tests.md`.

### Critical Review Checklist
| Check | What to verify |
|-------|----------------|
| No duplicated rules | audit calls `grammar.CheckName`, no re-implementation |
| Additive | parent feeders untouched |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Runtime audit runs | test present and green in the functional suite |
| Drift guard | `TestAuditConfigComplete` present |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource use | single daemon boot, bounded by suite timeout |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 demonstrated
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] `make ze-verify` passes
- [ ] Additive: parent Feeder 1/2 coverage unchanged

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Completion (BLOCKING)
- [ ] Write learned summary
- [ ] Commit A (code + spec + learned); Commit B (`git rm` spec)
