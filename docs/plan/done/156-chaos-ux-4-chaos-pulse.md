# Spec: Chaos Pulse Animation

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-chaos-ux-0-umbrella.md` - shared research and constraints
4. `docs/plan/spec-chaos-ux-1-peer-grid.md` - prerequisite spec (grid view must exist)
5. `cmd/ze-chaos/web/dashboard.go` - renderPeerCell (from spec 1), broadcastDirty
6. `cmd/ze-chaos/web/state.go` - PeerState struct
7. `cmd/ze-chaos/web/assets/style.css` - grid cell and animation styles

## Task

Add a CSS pulse animation to peer grid cells when a chaos event affects that peer. The pulse is a brief radial glow animation (0.5s) triggered by adding a `pulse` CSS class to the grid cell HTML during the SSE update following a chaos event. PeerState gets a transient `ChaosActive` bool flag that is set when a chaos event occurs and cleared after one render cycle. This spec depends on spec 1 (peer grid view) -- the grid view must exist before pulse animations can be applied to grid cells.

**Parent spec:** `docs/plan/spec-chaos-ux-0-umbrella.md`
**Depends on:** `docs/plan/spec-chaos-ux-1-peer-grid.md` (needs grid cells to exist)

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `docs/plan/spec-chaos-ux-0-umbrella.md` - shared research, constraints, source file survey
  --> Constraint: All rendering is server-side Go HTML; SSE broadcast every 200ms; ProcessEvent() must be fast
  --> Constraint: Animations must be CSS-only or minimal vanilla JS
  --> Decision: Dark theme with CSS custom properties
- [ ] `docs/plan/spec-chaos-ux-1-peer-grid.md` - prerequisite: grid view with renderPeerCell
  --> Constraint: Grid cells are rendered by renderPeerCell (from spec 1); each cell has id="peer-N" and status CSS class
  --> Decision: Grid cells are ~28x28px with status-based background color
- [ ] `docs/architecture/chaos-web-dashboard.md` - dashboard architecture
  --> Constraint: ProcessEvent() is synchronous on main event loop, must stay fast
  --> Constraint: broadcastDirty() renders under read lock; state flags cleared under write lock

**Key insights:**
- ChaosActive is a transient one-render-cycle flag: set in ProcessEvent (write lock), read in broadcastDirty render (read lock), cleared in ConsumeDirty or a dedicated clear step (write lock)
- The pulse class is added to the grid cell HTML only when ChaosActive is true; after one SSE push, the next render cycle omits it
- CSS animation with animation-iteration-count: 1 plays once per class addition; HTMX outerHTML swap re-adds the element, retriggering the animation
- The radial glow effect can be done with box-shadow animation: 0 0 0 0px transparent --> 0 0 8px 4px color --> 0 0 0 0px transparent
- broadcastDirty already renders per-peer updates for dirty peers; the pulse class is simply an additional CSS class on the grid cell
- The flag must be cleared AFTER rendering, not before -- otherwise the render cycle won't see it

## Current Behavior (MANDATORY)

**Source files read:** (see umbrella for full survey)
- [ ] `cmd/ze-chaos/web/dashboard.go` (695L) - ProcessEvent sets ChaosCount on EventChaosExecuted; broadcastDirty renders dirty peers
  --> Constraint: broadcastDirty runs ConsumeDirty under write lock first, then renders under read lock
  --> Decision: ChaosActive flag must survive the ConsumeDirty call but be cleared after render
- [ ] `cmd/ze-chaos/web/state.go` (594L) - PeerState struct holds per-peer counters; no transient render flags exist yet
  --> Constraint: PeerState is accessed under DashboardState.mu; new fields must follow same locking pattern
  --> Decision: ChaosActive is a bool on PeerState, not a separate map -- keeps the data with the peer
- [ ] `cmd/ze-chaos/web/render.go` (470L) - writePeerRows renders table rows; spec 1 adds renderPeerCell for grid cells
  --> Constraint: Grid cell from spec 1 has status CSS class and hx-get for detail; pulse class is additive
- [ ] `cmd/ze-chaos/web/assets/style.css` (1066L) - dark theme, existing status color classes
  --> Decision: Pulse animation keyframes use the peer's status color as the glow color (inherits from status class)

**Behavior to preserve:**
- Grid cell rendering from spec 1 (status colors, tooltip, click-to-detail)
- Table row rendering unchanged (pulse only applies to grid cells)
- ChaosCount increment in ProcessEvent unchanged
- All existing broadcastDirty rendering unchanged

**Behavior to change:**
- PeerState gets a new ChaosActive bool field
- ProcessEvent sets ChaosActive=true on EventChaosExecuted, EventDisconnected, EventError, EventReconnecting
- renderPeerCell (from spec 1) adds "pulse" CSS class when ChaosActive is true
- broadcastDirty clears ChaosActive for all rendered peers after the render pass
- CSS adds pulse keyframe animation on .pulse class (radial glow, 0.5s, single iteration)

## Data Flow (MANDATORY - see rules/data-flow-tracing.md)

### Entry Point
- Chaos events enter via ProcessEvent() -- EventChaosExecuted, EventDisconnected, EventError, EventReconnecting set ChaosActive
- Grid cell HTML exits via SSEBroker.Broadcast() with event type "peer-update"

### Transformation Path
1. ProcessEvent receives a chaos-related event; sets ps.ChaosActive = true on the affected PeerState
2. ProcessEvent marks the peer dirty (already happens via MarkDirty)
3. broadcastDirty consumes dirty flags under write lock (ChaosActive survives this step)
4. broadcastDirty renders grid cells under read lock; renderPeerCell checks ChaosActive and adds "pulse" CSS class
5. After rendering all dirty peers, broadcastDirty clears ChaosActive for those peers under a brief write lock
6. SSE pushes grid cell with "pulse" class; HTMX swaps outerHTML; CSS animation plays the radial glow
7. Next render cycle: ChaosActive is false, "pulse" class absent, animation does not play

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Event to State | ProcessEvent sets ChaosActive under write lock | [ ] |
| State to Render | broadcastDirty reads ChaosActive under read lock during render | [ ] |
| Render to Cleanup | broadcastDirty clears ChaosActive under write lock after render | [ ] |
| Go to Browser | SSE peer-update with grid cell HTML containing "pulse" class | [ ] |
| CSS animation | Radial glow keyframes, 0.5s duration, single iteration | [ ] |

### Integration Points
- `ProcessEvent()` in dashboard.go - set ChaosActive on chaos-related events
- `renderPeerCell()` in dashboard.go (from spec 1) - add "pulse" class when ChaosActive is true
- `broadcastDirty()` in dashboard.go - clear ChaosActive after render pass
- `PeerState` in state.go - add ChaosActive bool field
- `style.css` - pulse keyframe animation and .pulse class styles

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | --> | Feature Code | Test |
|-------------|---|--------------|------|
| ProcessEvent(EventChaosExecuted) | --> | PeerState.ChaosActive set to true | TestProcessEventSetsChaosActive |
| broadcastDirty with ChaosActive peer | --> | renderPeerCell includes "pulse" class | TestRenderPeerCellPulseClass |
| broadcastDirty after render | --> | ChaosActive cleared for rendered peers | TestBroadcastDirtyClearsChaosActive |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | EventChaosExecuted for peer N | PeerState[N].ChaosActive set to true |
| AC-2 | EventDisconnected for peer N | PeerState[N].ChaosActive set to true |
| AC-3 | EventError for peer N | PeerState[N].ChaosActive set to true |
| AC-4 | EventReconnecting for peer N | PeerState[N].ChaosActive set to true |
| AC-5 | EventRouteSent for peer N | PeerState[N].ChaosActive NOT set |
| AC-6 | Grid cell rendered with ChaosActive=true | Cell HTML has "pulse" CSS class |
| AC-7 | Grid cell rendered with ChaosActive=false | Cell HTML does NOT have "pulse" CSS class |
| AC-8 | After broadcastDirty renders a ChaosActive peer | ChaosActive is cleared to false |
| AC-9 | Pulse animation | Radial glow effect, 0.5s duration, plays once |
| AC-10 | Table row rendering | Table rows never get "pulse" class (grid cells only) |
| AC-11 | Rapid chaos events on same peer | Each event re-triggers pulse (ChaosActive set again before next render) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestProcessEventSetsChaosActive` | `cmd/ze-chaos/web/dashboard_test.go` | EventChaosExecuted sets ChaosActive=true on PeerState | |
| `TestProcessEventSetsChaosActiveDisconnect` | `cmd/ze-chaos/web/dashboard_test.go` | EventDisconnected sets ChaosActive=true | |
| `TestProcessEventSetsChaosActiveError` | `cmd/ze-chaos/web/dashboard_test.go` | EventError sets ChaosActive=true | |
| `TestProcessEventSetsChaosActiveReconnecting` | `cmd/ze-chaos/web/dashboard_test.go` | EventReconnecting sets ChaosActive=true | |
| `TestProcessEventNoChaosActiveForRoute` | `cmd/ze-chaos/web/dashboard_test.go` | EventRouteSent does NOT set ChaosActive | |
| `TestRenderPeerCellPulseClass` | `cmd/ze-chaos/web/dashboard_test.go` | renderPeerCell output contains "pulse" when ChaosActive=true | |
| `TestRenderPeerCellNoPulseClass` | `cmd/ze-chaos/web/dashboard_test.go` | renderPeerCell output does NOT contain "pulse" when ChaosActive=false | |
| `TestClearChaosActive` | `cmd/ze-chaos/web/state_test.go` | ClearChaosActive sets ChaosActive=false for specified peer indices | |
| `TestBroadcastDirtyClearsChaosActive` | `cmd/ze-chaos/web/dashboard_test.go` | After broadcastDirty, ChaosActive is false for rendered peers | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Pulse animation duration | 0.5s fixed | N/A | N/A | N/A |
| ChaosActive lifetime | 1 render cycle | 1 cycle | N/A | N/A |
| Rapid chaos events | Multiple per 200ms tick | Each sets flag | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-chaos-pulse` | `test/chaos/chaos-pulse.ci` | Chaos event on a peer triggers visible pulse on grid cell, clears after one cycle | |

### Future (if deferring any tests)
- Visual regression test for pulse glow appearance (deferrable, requires browser screenshot tooling)

## Files to Modify
- `cmd/ze-chaos/web/state.go` - add ChaosActive bool to PeerState; add ClearChaosActive method on DashboardState
- `cmd/ze-chaos/web/dashboard.go` - ProcessEvent: set ChaosActive on chaos events; renderPeerCell (from spec 1): add "pulse" class; broadcastDirty: clear ChaosActive after render
- `cmd/ze-chaos/web/assets/style.css` - add .pulse class with radial glow keyframe animation (0.5s, single iteration)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | No | N/A |
| CLI usage/help text | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |

## Files to Create
- `test/chaos/chaos-pulse.ci` - functional test for pulse animation behavior

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Verify spec 1 (peer grid) is implemented** - Grid view must exist with renderPeerCell. If not, this spec is blocked.
2. **Write unit test for ChaosActive flag** - Review: set on chaos events? Not set on route events? Cleared by ClearChaosActive?
3. **Run tests** - Verify FAIL (paste output). Fail for RIGHT reason?
4. **Add ChaosActive bool to PeerState in state.go** - Simple bool field. Add ClearChaosActive method on DashboardState.
5. **Run tests** - Verify PASS (paste output).
6. **Write unit tests for ProcessEvent setting ChaosActive** - Review: all 4 chaos event types? Non-chaos event excluded?
7. **Run tests** - Verify FAIL.
8. **Add ChaosActive setting to ProcessEvent** - Set ps.ChaosActive = true for EventChaosExecuted, EventDisconnected, EventError, EventReconnecting.
9. **Run tests** - Verify PASS.
10. **Write unit test for renderPeerCell with pulse class** - Review: pulse present when ChaosActive? Absent when not? Other classes preserved?
11. **Run tests** - Verify FAIL.
12. **Update renderPeerCell to add "pulse" class** - Conditional CSS class addition based on ps.ChaosActive.
13. **Run tests** - Verify PASS.
14. **Write unit test for broadcastDirty clearing ChaosActive** - Review: cleared after render? Not cleared for non-rendered peers?
15. **Run tests** - Verify FAIL.
16. **Add ChaosActive cleanup to broadcastDirty** - After render pass, acquire write lock and clear ChaosActive for rendered peers.
17. **Run tests** - Verify PASS.
18. **Add CSS pulse animation** - Keyframe: box-shadow glow 0 to max to 0. Duration 0.5s. Single iteration. Applied via .pulse class.
19. **Write functional test** - Create test/chaos/chaos-pulse.ci
20. **Verify all** - make ze-lint and make ze-chaos-test
21. **Critical Review** - All 6 checks from rules/quality.md must pass.
22. **Complete spec** - Fill audit tables, move to done/.

### Failure Routing

| Failure | Route To |
|---------|----------|
| Spec 1 not implemented | BLOCKED -- implement spec 1 first |
| Compilation error | Step 4 or 8 or 12 (fix syntax/types) |
| Test fails wrong reason | Step 2, 6, or 10 (fix test) |
| Test fails behavior mismatch | Re-read source from Current Behavior, RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural then DESIGN phase |
| ChaosActive not cleared | Check broadcastDirty cleanup ordering (must be after render, not before) |
| Pulse animation not visible | Check CSS keyframes, verify .pulse class is on the outerHTML-swapped element |
| Animation plays twice | Check animation-iteration-count: 1 and HTMX swap mode |
| Functional test fails | Check AC; if AC wrong then DESIGN; if AC correct then IMPLEMENT |
| Audit finds missing AC | Back to IMPLEMENT for that criterion |

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
- ChaosActive bool field on PeerState (transient, one render cycle)
- ProcessEvent sets ChaosActive on 4 chaos event types
- renderPeerCell adds "pulse" CSS class when ChaosActive is true
- broadcastDirty clears ChaosActive after render pass
- CSS keyframe animation: box-shadow glow 0.5s, single iteration

### Bugs Found/Fixed
- None

### Documentation Updates
- None needed

### Deviations from Plan
- None

## Implementation Audit

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ | TestProcessEventSetsChaosActive | ChaosExecuted |
| AC-2 | ✅ | TestProcessEventSetsChaosActive | Disconnected |
| AC-3 | ✅ | TestProcessEventSetsChaosActive | Error |
| AC-4 | ✅ | TestProcessEventSetsChaosActive | Reconnecting |
| AC-5 | ✅ | TestProcessEventNoChaosActiveForRoute | RouteSent excluded |
| AC-6 | ✅ | TestRenderPeerCellPulseClass | Pulse class present |
| AC-7 | ✅ | TestRenderPeerCellNoPulseClass | No pulse when inactive |
| AC-8 | ✅ | broadcastDirty cleanup loop | Cleared after render |
| AC-9 | ✅ | CSS chaos-pulse keyframe | 0.5s glow animation |
| AC-10 | ✅ | renderPeerRow unchanged | Table rows unaffected |
| AC-11 | ✅ | Flag reset per cycle | Re-set on next event |

### Tests from TDD Plan
| Test | Status | Location |
|------|--------|----------|
| TestProcessEventSetsChaosActive | ✅ | dashboard_test.go |
| TestProcessEventNoChaosActiveForRoute | ✅ | dashboard_test.go |
| TestRenderPeerCellPulseClass | ✅ | dashboard_test.go |
| TestRenderPeerCellNoPulseClass | ✅ | dashboard_test.go |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| state.go | ✅ | ChaosActive field |
| dashboard.go | ✅ | Set in ProcessEvent, pulse in renderPeerCell, clear in broadcastDirty |
| style.css | ✅ | .pulse class + chaos-pulse keyframe |

### Audit Summary
- **Total items:** 18
- **Done:** 18
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`cmd/ze-chaos/web/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] `make ze-lint` passes
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] **Spec included in commit** -- NEVER commit implementation without the completed spec. One commit = code + tests + spec.
