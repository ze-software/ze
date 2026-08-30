# Spec: finish-report-bus

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

Finish report-bus Phase 11 functional coverage. The keystone is a real, still-live bug: an empty-bus daemon-shutdown hang in the test harness that was never root-caused.

This is a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Each item below was confirmed still-open against the codebase with a producing `file:line`. Split into phases when picked up; the sections after Task are lightweight scaffolding to be filled at design time.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **Root-cause the empty-bus shutdown hang (L103)** - a `.ci` plugin dispatching `show errors` on an empty bus then `daemon shutdown`+`wait_for_shutdown` hangs to timeout. Product handler is benign (`show.go,118`); fault is in the harness shutdown/IPC path. Blocks L102,L115.
- **Empty-bus tests, blocked by L103 (L102,L115)** - `errors-empty-show.ci` (L102), `warnings-empty-show.ci` (L115). Unit tests already cover the empty case.
- **Distinct-blocker report-bus `.ci` (L116,L117,L113,L104)** - config-rollback (L116, multi-phase toggle plugin), config-save (L117, read-only-fs/write-intercept), warnings-clear (L113, ze-peer announce-over-threshold+withdraw), session-dropped (L104, ze-peer abrupt-close action).

### Post-wave corrections (2026-07-10)

**UNVERIFIED lead on the L103 hang.** The 2026-07 implementation wave added a fail-fast
write watchdog to the plugin RPC connection layer: on transports that do not implement
`SetWriteDeadline` (stdio, io.Pipe, SSH channels), a write stalled past a default 30s window
is logged, counted, and the connection is closed by `fireWatchdog`
(`pkg/plugin/rpc/conn.go`; watchdog fields :91-93; window `defaultWriteDeadline` at
:44; armed on the non-deadline path at :314). If the un-root-caused daemon-shutdown hang
involved a write stalled on such a transport in the harness shutdown/IPC path, the watchdog
would now break the stall after 30s instead of hanging to the test timeout, changing or
masking the symptom. This interaction is a hypothesis, not a finding: nobody has re-run the
repro against the new code. Obligation: re-run the L103 repro (`show errors` on an empty bus
then `daemon shutdown` + `wait_for_shutdown`) BEFORE investing in root-cause work, and check
the log for the "plugin rpc write stalled past watchdog window" warning and the
`ze_plugin_write_watchdog_total` counter to confirm or eliminate this lead.

## Required Reading

### Source files / docs

- [ ] `internal/component/cmd/show/show.go` (empty-bus handlers, proven benign at :101,:118)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `test/scripts/ze_api.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) --> (`wait_for_shutdown` - suspected hang site)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `internal/test/runner/` (shutdown/IPC lifecycle)
  -> Constraint: verify current behaviour against this source before designing.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; line numbers are pre-triage references)

- [ ] `internal/component/cmd/show/show.go`
- [ ] `internal/test/runner/`

**Behavior to preserve:**
- All existing behaviour of the listed files; this backlog work only adds the missing pieces named in the Task work items.

**Behavior to change:**
- Only the specific gaps enumerated in the Task work items.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze show errors` / `ze show warnings` dispatched via a `.ci` test plugin, then `daemon shutdown`

### Transformation Path
1. Test plugin dispatches a report-bus show command
2. Daemon renders (benign on empty bus) and acknowledges
3. `daemon shutdown` + `wait_for_shutdown` completes without hanging (the bug to fix)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| test plugin -> daemon | dispatch + shutdown over IPC | [ ] |
| daemon -> harness | `wait_for_shutdown` handshake | [ ] |

### Integration Points
- `internal/component/cmd/show/` (handlers)
- report bus producers
- `internal/test/runner/`

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
| `.ci` dispatches `show errors` on empty bus then shutdown | → | daemon exits cleanly, no hang | (fill during design) |
| `.ci` populates then clears a warning | → | `show warnings` reflects the clear | (fill during design) |

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
| errors-empty-show, warnings-empty-show, +4 (new) (`.ci`) | test/plugin | report-bus show through the daemon on empty and populated bus | |

## Files to Modify

- `internal/component/cmd/show/show.go` - see Task work items
- `internal/test/runner/` - see Task work items

## Implementation Steps

1. **Phase: split** - if the umbrella covers unrelated items, split into per-item specs first.
2. **Phase: design** - for the chosen item, re-verify the `file:line` evidence and fill the Data Flow / Wiring / AC sections above.
3. **Phase: wiring** - register entry points, write the failing wiring test.
4. **Phase: implement (TDD)** - write test, fail, implement, pass, per work item.
5. **Full verification** - `./le verify current mode full`.
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
- [ ] `./le verify worktree` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Skeleton = captured intent, not a designed spec (see `ai/rules/planning.md`). Moves to `design` when someone picks it up.
