# Spec: finish-ci-coverage

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

Write the deferred `.ci` functional tests whose feature code already exists and is unit-tested. No hard infra blocker - this is per-knob/per-command runner plumbing that was batched-deferred.

This is a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Each item below was confirmed still-open against the codebase with a producing `file:line`. Split into phases when picked up; the sections after Task are lightweight scaffolding to be filled at design time.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **Env-knob `.ci` (L215)** - 0 of ~12 exist: openwait, announce-delay, pid-file, pprof, l2tp-log-level, bridge-ack, migration-env. Code reads env directly; unit tests cover plumbing.
- **op-1 Tier-1 command `.ci` (L217)** - ~4 of 10 exist. Missing: system-cpu, system-date, interface-type, interface-errors, generate-wireguard-keypair.
- **cli-dispatch `.ci` (L83)** - validate-config done; missing `set interface create` and `update peeringdb`.
- **no-congestion-initial chaos `.ci` (L118)** - UNBLOCKED - ze-chaos multi-peer orchestration now exists (`mk/test-chaos.mk --peers`); just needs writing.
- **gRPC-over-wire `.ci` (L40)** - engine path covered by `test/plugin/grpc-execute.ci`; a true gRPC-wire test needs grpcio/grpcurl vendored (tooling gate).

## Required Reading

### Source files / docs

- [ ] `internal/test/runner/` (functional runner conventions)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `test/scripts/ze_api.py` (test plugin API)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `test/plugin/grpc-execute.ci` (existing engine-path gRPC coverage)
  -> Constraint: verify current behaviour against this source before designing.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; line numbers are pre-triage references)

- [ ] `internal/test/runner/`
- [ ] `internal/component/cmd/show/show.go`

**Behavior to preserve:**
- All existing behaviour of the listed files; this backlog work only adds the missing pieces named in the Task work items.

**Behavior to change:**
- Only the specific gaps enumerated in the Task work items.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze-test` functional runner executing `.ci` files against a live daemon

### Transformation Path
1. An already-shipped, unit-tested feature is selected
2. A `.ci` test drives it end-to-end through `ze cli` / plugin dispatch
3. The test asserts the observable daemon behaviour

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `.ci` runner -> daemon | plugin dispatch / CLI one-shot | [ ] |
| test plugin -> engine | `ze_api.py` API commands | [ ] |

### Integration Points
- `internal/test/runner/`
- `test/scripts/ze_api.py`
- the feature handlers under test

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugins.md`)

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
| `.ci` drives `show system cpu` | → | op-1 handler returns cpu data | (fill during design) |
| `.ci` sets an env knob | → | feature reads it via `env.Get*` | (fill during design) |

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
| env-*, op1-*, cli-dispatch-* (new) (`.ci`) | test/plugin, test/parse | each shipped knob/command through the daemon | |

## Files to Modify

- `internal/test/runner/` - see Task work items
- `internal/component/cmd/show/show.go` - see Task work items

## Implementation Steps

1. **Phase: split** - if the umbrella covers unrelated items, split into per-item specs first.
2. **Phase: design** - for the chosen item, re-verify the `file:line` evidence and fill the Data Flow / Wiring / AC sections above.
3. **Phase: wiring** - register entry points, write the failing wiring test.
4. **Phase: implement (TDD)** - write test, fail, implement, pass, per work item.
5. **Full verification** - `make ze-verify`.
6. **Complete spec** - fill audit tables, write `plan/learned/NNN-<name>.md`, two-commit closure.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
- Skeleton = captured intent, not a designed spec (see `ai/rules/planning.md`). Moves to `design` when someone picks it up.

### Post-wave corrections (2026-07-10)

- New obligation for the chaos `.ci` work item (no-congestion-initial, L118): the chaos orchestrator now validates listener/port-range conflicts at entry. `ValidateConfigRangeConflicts` (`internal/chaos/orchestrator/conflict.go`) derives the BGP and listen port range bases from the profile list and delegates to `ValidateRangeConflicts` (`conflict.go`), rejecting web/metrics/mcp listener endpoints that fall inside the derived per-peer port ranges; it is invoked at orchestrator entry (`internal/chaos/orchestrator/run.go`). Any new chaos `.ci` config must place its web/metrics/mcp listeners outside the derived BGP/listen ranges or the orchestrator rejects the run before starting.
