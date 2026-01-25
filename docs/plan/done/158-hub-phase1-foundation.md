# Spec: hub-phase1-foundation

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/hub-separation-phases.md` - phase overview
4. `internal/plugin/subsystem.go` - existing 5-stage protocol

## Task

Create the `internal/hub/` package as the **hub process entry point** that uses existing `internal/plugin/` infrastructure.

**Key clarification:** This phase does NOT create new forking/pipe code. The existing infrastructure handles that:
- `internal/plugin/subsystem.go` - SubsystemHandler, 5-stage protocol
- `internal/plugin/process.go` - Process struct, pipe management
- `internal/plugin/hub.go` - Hub struct with RouteCommand, ProcessConfig
- `internal/plugin/schema.go` - SchemaRegistry with FindHandler

**Scope:** Create entry point that wires existing components together. No config parsing, no schema routing yet.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/system-architecture.md` - end-user view of target
- [ ] `docs/plan/hub-separation-phases.md` - phase overview

### Source Files
- [ ] `internal/plugin/subsystem.go` - existing SubsystemHandler, 5-stage protocol
- [ ] `internal/plugin/process.go` - existing Process struct for pipes
- [ ] `internal/plugin/hub.go` - existing Hub struct with RouteCommand, ProcessConfig
- [ ] `internal/plugin/schema.go` - existing SchemaRegistry with FindHandler
- [ ] `cmd/ze-subsystem/main.go` - existing subsystem binary

**Key insights:**
- SubsystemHandler already handles forked process management
- Process struct already manages stdin/stdout pipes
- 5-stage protocol already implemented
- Hub struct already exists in `internal/plugin/hub.go` with routing logic
- SchemaRegistry already exists with longest-prefix FindHandler
- `internal/hub/` is for the hub **process** entry point, reusing existing infrastructure

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHubNew` | `internal/hub/hub_test.go` | Hub creates with config | |
| `TestHubForkProcess` | `internal/hub/hub_test.go` | Hub forks child, gets pipes | |
| `TestHubProcessCommunication` | `internal/hub/hub_test.go` | Hub sends/receives via pipes | |
| `TestHubProcessShutdown` | `internal/hub/hub_test.go` | Hub cleanly shuts down children | |
| `TestHubMultipleProcesses` | `internal/hub/hub_test.go` | Hub manages multiple children | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A - no new numeric inputs in this phase | | | | |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `hub-fork-echo` | `test/data/hub/fork-echo.ci` | Hub forks process, sends message, gets response | |

**Smoke test:** Minimal verification that hub entry point can fork and communicate. Full routing tests in Phase 5.

## Files to Modify

- `cmd/ze/main.go` - Add hub mode detection (if config file given)

## Files to Create

- `internal/hub/hub.go` - Entry point that composes existing `plugin.Hub`, `plugin.SubsystemManager`, `plugin.SchemaRegistry`
- `internal/hub/hub_test.go` - Unit tests

**Note:** No `process.go` needed - use `plugin.Process` directly. Avoid wrapping existing code unnecessarily.

## Implementation Steps

1. **Write unit tests** - Create tests for Hub struct
   - Test creation, fork, communication, shutdown

   → **Review:** Do tests cover error cases?

2. **Run tests** - Verify FAIL (paste output)

   → **Review:** Tests fail for right reason?

3. **Create hub.go** - Hub struct composing existing infrastructure

   **Hub struct fields:**
   | Field | Type | Description |
   |-------|------|-------------|
   | subsystems | *plugin.SubsystemManager | Manages forked processes |
   | registry | *plugin.SchemaRegistry | Schema routing |
   | hub | *plugin.Hub | Command routing |

   **Hub methods:**
   | Method | Description |
   |--------|-------------|
   | New | Create hub instance with existing components |
   | Run | Start subsystems, run event loop |
   | Shutdown | Clean shutdown of all children |

   → **Review:** Reuses existing code? No duplication?

4. **Run tests** - Verify PASS (paste output)

5. **Verify** - `make lint && make test && make functional`

## Design Decisions

### Why create internal/hub/ instead of extending internal/plugin/?

| Option | Pros | Cons |
|--------|------|------|
| Extend plugin/ | Less new code | Mixes hub + plugin concerns |
| New hub/ | Clear separation | More packages |

**Decision:** New package. Hub is the orchestrator entry point, `internal/plugin/` provides the components.

### What internal/hub/ does

| Responsibility | Implementation |
|----------------|----------------|
| Entry point | New code - `hub.Run()` |
| Process forking | Use `plugin.SubsystemManager` |
| Pipe I/O | Use `plugin.Process` |
| 5-stage protocol | Use `plugin.SubsystemHandler` |
| Command routing | Use `plugin.Hub` |
| Schema registry | Use `plugin.SchemaRegistry` |

**Key insight:** `internal/hub/` is a thin entry point that composes existing components. Avoid duplication.

## Implementation Summary

<!-- Fill after implementation -->

### What Was Implemented
- [List actual changes]

### Bugs Found/Fixed
- [Any bugs discovered]

### Deviations from Plan
- [Any differences and why]

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read

### Completion (after tests pass)
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
