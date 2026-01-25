# Spec: hub-phase1-foundation

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/hub-separation-phases.md` - phase overview
4. `internal/plugin/subsystem.go` - existing 5-stage protocol

## Task

Create the `internal/hub/` package with basic process forking and pipe communication. This is the foundation for the hub/orchestrator architecture.

**Scope:** Only fork processes and establish pipes. No config parsing, no schema routing yet.

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
| Deferred to Phase 5 - need full routing first | | | |

## Files to Modify

- `cmd/ze/main.go` - Add hub mode detection (if config file given)

## Files to Create

- `internal/hub/hub.go` - Hub struct, New(), Start(), Shutdown()
- `internal/hub/process.go` - Process management (wraps plugin.Process)
- `internal/hub/hub_test.go` - Unit tests

## Implementation Steps

1. **Write unit tests** - Create tests for Hub struct
   - Test creation, fork, communication, shutdown

   → **Review:** Do tests cover error cases?

2. **Run tests** - Verify FAIL (paste output)

   → **Review:** Tests fail for right reason?

3. **Create hub.go** - Hub struct wrapping existing infrastructure
   ```
   type Hub struct {
       processes map[string]*Process
       mu        sync.RWMutex
   }

   func New() *Hub
   func (h *Hub) Fork(name, binary string, args ...string) (*Process, error)
   func (h *Hub) Shutdown(ctx context.Context) error
   ```

   → **Review:** Reuses existing code? No duplication?

4. **Create process.go** - Thin wrapper around plugin.Process

   → **Review:** Why wrap instead of using directly?

5. **Run tests** - Verify PASS (paste output)

6. **Verify** - `make lint && make test && make functional`

## Design Decisions

### Why create internal/hub/ instead of extending internal/plugin/?

| Option | Pros | Cons |
|--------|------|------|
| Extend plugin/ | Less new code | Mixes hub + plugin concerns |
| New hub/ | Clear separation | More packages |

**Decision:** New package. Hub is the orchestrator, plugins are children. Different concerns.

### What does Hub wrap vs create new?

| Component | Approach |
|-----------|----------|
| Process forking | Wrap `plugin.Process` |
| Pipe I/O | Use `plugin.Process` directly |
| 5-stage protocol | Use `plugin.SubsystemHandler` |
| Process registry | New code in Hub |

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
