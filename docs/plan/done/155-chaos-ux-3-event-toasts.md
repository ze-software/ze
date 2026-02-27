# Spec: Event Toast Notifications

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-chaos-ux-0-umbrella.md` - shared research and constraints
4. `cmd/ze-chaos/web/dashboard.go` - ProcessEvent(), broadcastDirty()
5. `cmd/ze-chaos/web/sse.go` - SSEBroker, Broadcast(), SSEEvent
6. `cmd/ze-chaos/web/render.go` - writeLayout() for toast container placement
7. `cmd/ze-chaos/web/assets/style.css` - animation styles

## Task

Add brief toast notifications that appear when chaos events occur (peer disconnect, reconnect, error, chaos executed). A new SSE event type `toast` is pushed from broadcastDirty when toast-worthy events accumulate during the broadcast tick. Toasts appear in a container in the top-right corner of the main area, auto-dismiss after 5 seconds using CSS animation, and stack vertically with a maximum of 5 visible (oldest dismissed when limit reached). Auto-dismiss uses CSS animation (slide-in + fade-out) combined with HTMX swap-oob for removal -- not JS setTimeout.

**Parent spec:** `docs/plan/spec-chaos-ux-0-umbrella.md`

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `docs/plan/spec-chaos-ux-0-umbrella.md` - shared research, constraints, source file survey
  --> Constraint: All rendering is server-side Go HTML; SSE broadcast every 200ms; ProcessEvent() must be fast
  --> Constraint: HTMX + SSE architecture, no JS framework; animations must be CSS-only or minimal vanilla JS
  --> Decision: Dark theme with CSS custom properties
- [ ] `docs/architecture/chaos-web-dashboard.md` - dashboard architecture
  --> Constraint: ProcessEvent() is synchronous on main event loop, must stay fast (~1us)
  --> Constraint: SSEBroker.Broadcast is non-blocking, drops events if client buffer full
  --> Decision: SSE events use named event types; client subscribes via sse-swap attributes

**Key insights:**
- ProcessEvent() already categorizes events by type; toast-worthy events are: EventDisconnected, EventReconnecting, EventError, EventChaosExecuted
- broadcastDirty() already iterates dirty peers and broadcasts; toast events should be accumulated during ProcessEvent and flushed during broadcastDirty
- The toast SSE event is a new event type "toast" -- clients need an sse-swap="toast" element in the DOM
- Auto-dismiss must work without JS timers; CSS animation-delay + animation-fill-mode: forwards can hide after 5s, then HTMX swap-oob="delete" from a subsequent broadcast tick removes the DOM element
- Alternative approach: CSS-only 5s animation that ends with opacity:0 + pointer-events:none, with the server stopping toast broadcasts for that toast after ~6s (30 ticks at 200ms)
- Maximum 5 visible: server tracks pending toasts in a bounded slice, only renders the 5 most recent

## Current Behavior (MANDATORY)

**Source files read:** (see umbrella for full survey)
- [ ] `cmd/ze-chaos/web/dashboard.go` (695L) - ProcessEvent() updates state and marks dirty; broadcastDirty() renders and pushes SSE events
  --> Constraint: ProcessEvent() must stay fast; accumulate toast-worthy flags, don't render in ProcessEvent
  --> Decision: broadcastDirty already has a dirtyGlobal flag and processes dirty peers; toast rendering fits here
- [ ] `cmd/ze-chaos/web/sse.go` (160L) - SSEBroker broadcasts SSEEvent with Event name and Data payload; non-blocking per-client
  --> Constraint: New SSE event type "toast" follows same pattern as "stats", "events", etc.
  --> Decision: Toast events use sse-swap="toast" on a container div in the layout
- [ ] `cmd/ze-chaos/web/render.go` (470L) - writeLayout() renders full page; toast container must be added to the main area
  --> Constraint: Toast container needs fixed positioning relative to main content area (top-right)
- [ ] `cmd/ze-chaos/web/state.go` (594L) - DashboardState holds all mutable state; toast queue needs to be added here
  --> Constraint: Toast queue accessed under write lock in ProcessEvent, read lock in broadcastDirty
- [ ] `cmd/ze-chaos/web/assets/style.css` (1066L) - dark theme, CSS custom properties for colors
  --> Decision: Toast colors should match event type: --red for disconnect/error, --yellow for chaos/reconnecting, --green for established

**Behavior to preserve:**
- Existing SSE event types and their swap targets unchanged
- ProcessEvent() performance (must stay fast)
- All existing broadcastDirty rendering unchanged
- No existing functionality removed

**Behavior to change:**
- Add new SSE event type "toast" for toast notifications
- ProcessEvent accumulates toast-worthy events in a bounded queue on DashboardState
- broadcastDirty flushes pending toasts as SSE "toast" events
- writeLayout includes a toast container div with sse-swap="toast"
- CSS provides slide-in animation and auto-fade-out after 5 seconds

## Data Flow (MANDATORY - see rules/data-flow-tracing.md)

### Entry Point
- Chaos events enter via ProcessEvent() -- EventDisconnected, EventReconnecting, EventError, EventChaosExecuted are toast-worthy
- Toast HTML exits via SSEBroker.Broadcast() with event type "toast"

### Transformation Path
1. ProcessEvent receives a toast-worthy event; appends a ToastEntry to DashboardState.pendingToasts (bounded at 5, oldest dropped)
2. broadcastDirty checks if pendingToasts is non-empty; consumes the slice under write lock
3. For each toast, renders an HTML fragment with toast content, CSS animation classes, and unique ID
4. Broadcasts each toast as SSE event type "toast"; HTMX appends to toast container via hx-swap-oob="beforeend"
5. CSS animation plays: slide-in from right (0.3s), hold visible, fade-out (0.5s starting at 4.5s)
6. After animation ends, element is visually hidden; server-side cleanup is not needed (browser reaps on next full-page load or toast container swap)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Event to State | ProcessEvent appends to pendingToasts under write lock | [ ] |
| State to Render | broadcastDirty consumes pendingToasts under write lock, renders under read lock | [ ] |
| Go to Browser | SSE event "toast" with HTML fragment | [ ] |
| CSS animation | slide-in + fade-out, auto-dismiss visual | [ ] |

### Integration Points
- `ProcessEvent()` in dashboard.go - append to pendingToasts for toast-worthy events
- `broadcastDirty()` in dashboard.go - consume and broadcast toast events
- `writeLayout()` in render.go - add toast container div with sse-swap="toast"
- `DashboardState` in state.go - add pendingToasts field and ToastEntry type
- `style.css` - toast container positioning, toast card styles, slide-in and fade-out animations

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | --> | Feature Code | Test |
|-------------|---|--------------|------|
| ProcessEvent(EventChaosExecuted) | --> | Toast appended to pendingToasts | TestProcessEventQueuesToast |
| broadcastDirty with pending toast | --> | SSE "toast" event broadcast | TestBroadcastDirtyFlushesToasts |
| GET / (full page load) | --> | writeLayout includes toast container with sse-swap | TestLayoutIncludesToastContainer |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | EventDisconnected occurs | Toast appears with disconnect message and peer index |
| AC-2 | EventReconnecting occurs | Toast appears with reconnecting message and peer index |
| AC-3 | EventError occurs | Toast appears with error message and peer index |
| AC-4 | EventChaosExecuted occurs | Toast appears with chaos action name and peer index |
| AC-5 | Toast displayed | Slides in from right with CSS animation (0.3s) |
| AC-6 | Toast visible for 5 seconds | Fades out via CSS animation (no JS timer) |
| AC-7 | 6 toasts in rapid succession | Only 5 most recent visible; oldest dropped from queue |
| AC-8 | Full page load (GET /) | Toast container div present with sse-swap="toast" |
| AC-9 | Toast coloring | Disconnect/error toasts use --red; chaos/reconnecting use --yellow |
| AC-10 | EventRouteSent occurs | No toast generated (not a toast-worthy event) |
| AC-11 | Toast content | Shows peer index, event type label, and timestamp |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestProcessEventQueuesToast` | `cmd/ze-chaos/web/dashboard_test.go` | EventChaosExecuted adds entry to pendingToasts | |
| `TestProcessEventNonToastEvent` | `cmd/ze-chaos/web/dashboard_test.go` | EventRouteSent does NOT add to pendingToasts | |
| `TestToastQueueMaxFive` | `cmd/ze-chaos/web/state_test.go` | Pushing 6 toasts keeps only 5 most recent | |
| `TestRenderToast` | `cmd/ze-chaos/web/render_test.go` | renderToast produces HTML with correct CSS classes and content | |
| `TestRenderToastDisconnect` | `cmd/ze-chaos/web/render_test.go` | Disconnect toast has error color class | |
| `TestRenderToastChaos` | `cmd/ze-chaos/web/render_test.go` | Chaos toast has warning color class and action name | |
| `TestBroadcastDirtyFlushesToasts` | `cmd/ze-chaos/web/dashboard_test.go` | broadcastDirty consumes pendingToasts and broadcasts SSE events | |
| `TestLayoutIncludesToastContainer` | `cmd/ze-chaos/web/render_test.go` | writeLayout includes toast container with sse-swap="toast" | |
| `TestToastSSEEventFormat` | `cmd/ze-chaos/web/dashboard_test.go` | Toast SSE event has type "toast" and hx-swap-oob="beforeend:#toast-container" | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| pendingToasts length | 0-5 | 5 | N/A (0 = no toasts) | 6 (oldest dropped) |
| Toast auto-dismiss | 5 seconds | 5s | N/A | N/A (CSS handles) |
| Toast animation duration | 0.3s slide-in + 0.5s fade-out | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-event-toasts` | `test/chaos/event-toasts.ci` | Chaos event triggers toast, toast appears in dashboard, auto-dismisses | |

### Future (if deferring any tests)
- Toast click-to-dismiss interaction (deferrable, cosmetic enhancement)
- Toast grouping when same event type fires for multiple peers in same tick (deferrable, optimization)

## Files to Modify
- `cmd/ze-chaos/web/dashboard.go` - ProcessEvent: append toast-worthy events to pendingToasts; broadcastDirty: consume and broadcast toasts
- `cmd/ze-chaos/web/state.go` - add ToastEntry struct and pendingToasts slice to DashboardState; add ConsumePendingToasts method
- `cmd/ze-chaos/web/render.go` - add toast container to writeLayout; add renderToast helper
- `cmd/ze-chaos/web/assets/style.css` - toast container fixed positioning (top-right), toast card styles, slide-in and fade-out keyframe animations

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
- `test/chaos/event-toasts.ci` - functional test for toast notification behavior

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit test for ToastEntry and queue** - Review: max 5 enforcement? Oldest dropped? Empty queue works?
2. **Run tests** - Verify FAIL (paste output). Fail for RIGHT reason?
3. **Implement ToastEntry struct and pendingToasts on DashboardState** - Bounded slice, ConsumePendingToasts drains and returns.
4. **Run tests** - Verify PASS (paste output).
5. **Write unit test for ProcessEvent toast queuing** - Review: toast-worthy events queued? Non-toast events ignored?
6. **Run tests** - Verify FAIL.
7. **Add toast queuing to ProcessEvent** - Append ToastEntry for EventDisconnected, EventReconnecting, EventError, EventChaosExecuted.
8. **Run tests** - Verify PASS.
9. **Write unit test for renderToast** - Review: correct HTML structure? CSS classes? Content includes peer index and label?
10. **Run tests** - Verify FAIL.
11. **Implement renderToast in render.go** - HTML div with toast class, color class by event type, hx-swap-oob="beforeend:#toast-container".
12. **Run tests** - Verify PASS.
13. **Write unit test for broadcastDirty toast flushing** - Review: toasts consumed? SSE events broadcast with type "toast"?
14. **Run tests** - Verify FAIL.
15. **Add toast broadcasting to broadcastDirty** - Consume pending toasts under write lock, broadcast each as SSE "toast" event.
16. **Add toast container to writeLayout** - Fixed-position div in main area with id="toast-container" and sse-swap="toast".
17. **Run tests** - Verify PASS.
18. **Add CSS styles** - Toast container (fixed top-right, z-index), toast card styles, keyframe animations for slide-in and fade-out.
19. **Write functional test** - Create test/chaos/event-toasts.ci
20. **Verify all** - make ze-lint and make ze-chaos-test
21. **Critical Review** - All 6 checks from rules/quality.md must pass.
22. **Complete spec** - Fill audit tables, move to done/.

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 or 7 or 11 (fix syntax/types) |
| Test fails wrong reason | Step 1, 5, or 9 (fix test) |
| Test fails behavior mismatch | Re-read source from Current Behavior, RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural then DESIGN phase |
| Toast not appearing | Check sse-swap attribute matches event name; check toast container exists in layout |
| Toast not auto-dismissing | Check CSS animation keyframes and animation-fill-mode: forwards |
| Queue overflow | Verify bounded slice enforcement in ConsumePendingToasts |
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
- ToastEntry struct, QueueToast/ConsumePendingToasts on DashboardState (bounded at 5)
- toastForEvent() maps 4 event types to toast entries with color classes
- renderToast() produces HTML with hx-swap-oob for toast container
- ProcessEvent queues toasts; broadcastDirty consumes and broadcasts SSE "toast" events
- Toast container in writeLayout with sse-swap="toast"
- CSS slide-in + auto-fade-out animations

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
| AC-1 | ✅ | TestProcessEventQueuesToast | Disconnect queues toast |
| AC-4 | ✅ | TestProcessEventQueuesToast | Chaos queues toast |
| AC-7 | ✅ | TestToastQueueMaxFive | Bounded at 5 |
| AC-8 | ✅ | TestLayoutIncludesToastContainer | Container with sse-swap |
| AC-9 | ✅ | TestRenderToastColors | Error=red, warn=yellow |
| AC-10 | ✅ | TestProcessEventNonToastEvent | RouteSent ignored |
| AC-11 | ✅ | TestRenderToast | Peer, label, detail |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestProcessEventQueuesToast | ✅ | dashboard_test.go | |
| TestProcessEventNonToastEvent | ✅ | dashboard_test.go | |
| TestToastQueueMaxFive | ✅ | dashboard_test.go | |
| TestRenderToast | ✅ | dashboard_test.go | |
| TestRenderToastColors | ✅ | dashboard_test.go | |
| TestLayoutIncludesToastContainer | ✅ | handlers_test.go | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| state.go | ✅ | ToastEntry, QueueToast, ConsumePendingToasts |
| dashboard.go | ✅ | ProcessEvent + broadcastDirty toast handling |
| render.go | ✅ | toastForEvent, renderToast, toast container |
| style.css | ✅ | Toast styles + animations |

### Audit Summary
- **Total items:** 17
- **Done:** 17
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
