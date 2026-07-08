# Spec: followup-vpp-traffic

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

Wire the VPP classify + QoS pipeline for traffic-control. The govpp classify/qos binapi packages are vendored, so these are design-unblocked but code-unwired; the verifier rejects each feature at commit time (exact-or-reject) rather than shipping a silent no-op.

This is a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Each item below was confirmed still-open against the codebase with a producing `file:line`. Split into phases when picked up; the sections after Task are lightweight scaffolding to be filled at design time.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **filter protocol end-to-end (L207,L208)** - classify table create + `ClassifySetInterfaceIPTable` attach + session mgmt + IPv6 next-header (L207). First impl attached to no interface and matched wrong offset.
- **filter dscp 3-step QoS (L209)** - needs `QosRecordEnableDisable` (ingress) + `QosEgressMapUpdate` + `QosMarkEnableDisable`; first impl skipped the record step so mark read an uncaptured DSCP.
- **filter mark (L205)** - VPP classify matches header bytes not SKB metadata; needs a VPP-native metadata primitive.
- **qdisc prio -> QoS egress map (L206)** - needs an explicit operator-facing DSCP->class binding; `egressMapFromPrioClasses` retained as skeleton.
- **Multi-class HTB/TBF shaping (L210)** - per-class shaping needs filter-based steering to distinct policers (depends on the classify pipeline above); single-class works today.

## Required Reading

### Source files / docs

- [ ] `internal/plugins/traffic/vpp/verify.go` (current rejections at :15-17,:142-183)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `internal/plugins/traffic/vpp/translate.go` (retained skeletons)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `internal/plugins/traffic/vpp/backend_linux.go`
  -> Constraint: verify current behaviour against this source before designing.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; line numbers are pre-triage references)

- [ ] `internal/plugins/traffic/vpp/verify.go`
- [ ] `internal/plugins/traffic/vpp/translate.go`
- [ ] `internal/plugins/traffic/vpp/backend_linux.go`

**Behavior to preserve:**
- All existing behaviour of the listed files; this backlog work only adds the missing pieces named in the Task work items.

**Behavior to change:**
- Only the specific gaps enumerated in the Task work items.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- `traffic-control` YANG under the `vpp` backend
- verifier gate at config-apply time

### Transformation Path
1. Config declares filter protocol/dscp/mark or a multi-class qdisc
2. Verifier currently rejects it (exact-or-reject)
3. The classify/QoS pipeline is wired so the verifier can accept and program VPP

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config -> trafficvpp | YANG resolution + verifier | [ ] |
| trafficvpp -> VPP | govpp classify/qos binary API | [ ] |

### Integration Points
- `internal/plugins/traffic/vpp/`
- vendored govpp classify/qos binapi

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
| `filter protocol` under vpp backend | → | classify table attached to the interface | (fill during design) |
| `filter dscp` under vpp backend | → | 3-step QoS record+map+mark programmed | (fill during design) |

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
| traffic-vpp classify/dscp/prio (new) (`.ci`) | test/traffic | classify+QoS through a real VPP daemon | |

## Files to Modify

- `internal/plugins/traffic/vpp/verify.go` - see Task work items
- `internal/plugins/traffic/vpp/translate.go` - see Task work items
- `internal/plugins/traffic/vpp/backend_linux.go` - see Task work items

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
- Skeleton = captured intent, not a designed spec (see `ai/rules/deferral-tracking.md`). Moves to `design` when someone picks it up.
