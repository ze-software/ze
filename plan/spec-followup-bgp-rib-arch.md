# Spec: followup-bgp-rib-arch

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

Deep BGP engine / RIB architecture follow-ups. Distinct but low-priority; grouped so the intent survives. protorib-0 (L221) in particular is a design decision, not a mechanical task.

This is a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Each item below was confirmed still-open against the codebase with a producing `file:line`. Split into phases when picked up; the sections after Task are lightweight scaffolding to be filled at design time.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **protorib-0 central RIB store (L221)** - DESIGN QUESTION: engine-owned per-protocol store vs the shipped event-bus delta model. Move when a second consumer beyond bgp-redistribute makes it painful (learned/634).
- **plugin-ipc-raw-bytes (L184)** - length-prefixed raw-bytes filter IPC instead of the JSON string `FilterUpdateInput.Update` in `pkg/plugin/rpc/types.go`; breaks external SDK contract.
- **rib-inject RFC 5549 (L58)** - extended next-hop for injected routes; `PackContext.ExtendedNextHop` does not exist.
- **fib-ecmp realtime best-change (L122)** - atomic N-nexthop event delivery; today emits single-best (`SelectMultipath` is show-path only).
- **bmp-6 Loc-RIB monitoring (L231)** - RFC 9069 PeerType=3 from BestChangeBatch; needs UPDATE wire bytes from structured data.
- **rs-fastpath state-tracker consumer (L228)** - first production `locrib.OnChange` subscriber that reads `Change.Forward` bytes (`forward_observer.go` is a nil-check logger today).
- **llgr-readvertisement multi-peer `.ci` (L68)** - single-peer partial exists; multi-peer partial-deployment fixture missing.
- **NLRI structural rewrite via ModAccumulator (L60)** - announce->withdraw conversion exists; general NLRI-byte rewrite field not added.

## Required Reading

### Source files / docs

- [ ] `internal/component/bgp/reactor/`, `internal/component/bgp/plugins/rib/`
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `internal/component/bgp/plugins/rib/forward_handle.go`
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `pkg/plugin/rpc/types.go` (`FilterUpdateInput.Update` string)
  -> Constraint: verify current behaviour against this source before designing.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; line numbers are pre-triage references)

- [ ] `internal/component/bgp/plugins/rib/forward_handle.go`
- [ ] `pkg/plugin/rpc/types.go`
- [ ] `internal/component/bgp/plugins/bmp/`

**Behavior to preserve:**
- All existing behaviour of the listed files; this backlog work only adds the missing pieces named in the Task work items.

**Behavior to change:**
- Only the specific gaps enumerated in the Task work items.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Received UPDATE / best-path change events; filter IPC; RIB inject API

### Transformation Path
1. A route change or injected route enters the RIB
2. Best-path / forward / redistribute machinery processes it
3. Consumers (FIB, BMP, filters) receive the result

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| engine -> plugin | filter IPC (JSON today, raw-bytes proposed) | [ ] |
| RIB -> consumers | event-bus delta vs proposed central store | [ ] |

### Integration Points
- `internal/component/bgp/plugins/rib/`
- `internal/component/bgp/plugins/bmp/`
- `pkg/plugin/rpc/`

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
| Inject a route needing extended next-hop | → | RFC 5549 next-hop encoded | (fill during design) |
| Multipath best-path change | → | N nexthops delivered atomically | (fill during design) |

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
| rib-inject-rfc5549, llgr-multipeer, bmp-locrib (new) (`.ci`) | test/plugin | engine/RIB behaviour end-to-end | |

## Files to Modify

- `internal/component/bgp/plugins/rib/forward_handle.go` - see Task work items
- `pkg/plugin/rpc/types.go` - see Task work items
- `internal/component/bgp/plugins/bmp/` - see Task work items

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
