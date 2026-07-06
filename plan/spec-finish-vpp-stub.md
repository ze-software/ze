# Spec: finish-vpp-stub

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `git log -p plan/deferrals.md` (pre-2026-07-06) - original deferral rows + evidence

## Task

Extend `test/scripts/vpp_stub.py` (Python VPP binary-API emulator) and add the `test/vpp/*.ci` tests that depend on the richer stub. Unblocks AC-2..16 iface/telemetry coverage that is currently unit-only.

This is a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Each item below was confirmed still-open against the codebase with a producing `file:line`. Split into phases when picked up; the sections after Task are lightweight scaffolding to be filled at design time.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **Iface binary-API handlers (L192)** - stub lacks create_loopback / sw_interface_set_flags / sw_interface_add_del_address / sw_interface_dump / sw_interface_event; AC-2..16 stay unit-only until added.
- **Stats-segment emulation (L178,L181)** - no shared-memory stat-counter emulation (L178); blocks `test/vpp/012-telemetry.ci` (L181).
- **Fault injection (L179)** - `--inject ip_route_add_del:retval=-1:after=3` style error-path testing.
- **Route dump (L180)** - `ip_route_dump` streaming handler returning installed routes (state across calls).
- **fib-withdraw + restart `.ci` (L175)** - `003-fib-withdraw.ci` + `004-vpp-restart.ci` still absent (coexist already landed renamed).

## Required Reading

### Source files / docs

- [ ] `test/scripts/vpp_stub.py:517` (HANDLERS)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `test/vpp/*.ci` (functional VPP tests)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `internal/plugins/iface/vpp/ifacevpp.go`, `internal/plugins/traffic/vpp/` (code the stub exercises)
  -> Constraint: verify current behaviour against this source before designing.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; line numbers are pre-triage references)

- [ ] `internal/plugins/iface/vpp/ifacevpp.go`
- [ ] `internal/plugins/traffic/vpp/backend_linux.go`

**Behavior to preserve:**
- All existing behaviour of the listed files; this backlog work only adds the missing pieces named in the Task work items.

**Behavior to change:**
- Only the specific gaps enumerated in the Task work items.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- `test/vpp/*.ci` running `ze` against the Python `vpp_stub.py` binary-API emulator

### Transformation Path
1. `vpp_stub.py` gains a handler / stats-segment / fault-injection / dump capability
2. A `test/vpp/*.ci` drives `ze` against the enriched stub
3. The test asserts the VPP-facing behaviour end-to-end

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `ze` -> stub | GoVPP binary API over the stub socket | [ ] |
| stub -> test | stats segment / route dump responses | [ ] |

### Integration Points
- `test/scripts/vpp_stub.py`
- `internal/plugins/iface/vpp/`
- `internal/plugins/traffic/vpp/`

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The verified `file:line` evidence in the Task items still holds at design time | 2026-07-06 backlog triage | Re-scope the item | grep/LSP at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scope drift when the umbrella is split into per-item specs | Item needs its own design doc | Split into a dedicated spec and re-point |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `.ci` creates a VPP loopback via `ze` | → | stub `create_loopback` handler responds | (fill during design) |
| `.ci` scrapes VPP stats | → | stub stats-segment emulation returns counters | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | (define per work item when this skeleton moves to `design`) | (define at design time) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (define at design time) | (define at design time) | per Task work item | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| 012-telemetry, 003-fib-withdraw, 004-vpp-restart (new) (`.ci`) | test/vpp | VPP behaviour via the enriched stub | |

## Files to Modify

- `internal/plugins/iface/vpp/ifacevpp.go` - see Task work items
- `internal/plugins/traffic/vpp/backend_linux.go` - see Task work items

## Implementation Steps

1. **Phase: split** - if the umbrella covers unrelated items, split into per-item specs first.
2. **Phase: design** - for the chosen item, re-verify the `file:line` evidence and fill the Data Flow / Wiring / AC sections above.
3. **Phase: wiring** - register entry points, write the failing wiring test.
4. **Phase: implement (TDD)** - write test, fail, implement, pass, per work item.
5. **Full verification** - `make ze-verify`.
6. **Complete spec** - fill audit tables, write `plan/learned/NNN-<name>.md`, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [ ] Every chosen work item has feature code + test
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Skeleton = captured intent, not a designed spec (see `ai/rules/deferral-tracking.md`). Moves to `design` when someone picks it up.
