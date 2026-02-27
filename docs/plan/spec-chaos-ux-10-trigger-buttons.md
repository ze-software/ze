# Spec: Trigger Icon Buttons

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-chaos-ux-0-umbrella.md` - shared research and constraints
4. `cmd/ze-chaos/web/control.go` - writeTriggerForm, chaosActionTypes, writeControlPanel, chaosActionImpact
5. `cmd/ze-chaos/web/handlers.go` - handleControlTrigger, handleControlTriggerForm, registerRoutes
6. `cmd/ze-chaos/web/assets/style.css` - control panel styles, badge class

## Task

Replace the trigger action dropdown in the control panel with individual icon buttons for each trigger type. Each chaos action type (tcp-disconnect, notification-cease, hold-timer-expiry, disconnect-during-burst, reconnect-storm, connection-collision, malformed-update, config-reload) gets its own button showing a Unicode icon and short label. Clicking a button loads the trigger param form for that action (same as the current dropdown change behavior: hx-get to /control/trigger-form with action param). Selected peer(s) are shown as a badge/label next to the buttons. The peer selection input and action-specific params form remain -- only the dropdown selector is replaced with buttons.

**Parent spec:** `docs/plan/spec-chaos-ux-0-umbrella.md`

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `docs/plan/spec-chaos-ux-0-umbrella.md` - shared research, constraints, source file survey
  --> Constraint: All rendering is server-side Go HTML; SSE broadcast every 200ms; ProcessEvent() must be fast
  --> Constraint: HTMX + SSE architecture, no JS framework
  --> Decision: Dark theme with CSS custom properties
- [ ] `docs/architecture/chaos-web-dashboard.md` - dashboard architecture
  --> Constraint: Control commands sent non-blocking to d.control channel; nil channel = UI hidden
  --> Decision: writeTriggerForm renders action dropdown + peer select + params form

### RFC Summaries (MUST for protocol work)
- N/A (no protocol work)

**Key insights:**
- chaosActionTypes() in control.go returns 8 action types as string slice
- chaosActionImpact() returns human-readable impact description for each action (used as tooltip)
- Current flow: select dropdown with hx-get="/control/trigger-form" hx-trigger="change" fetches param form
- handleControlTriggerForm reads action from URL query param "action", serves writeTriggerForm
- handleControlTrigger reads "action" from POST form values -- this contract is unchanged
- writeTriggerForm renders peer input, optional fraction input (partial-withdraw), and Execute button
- Each button replaces one option element; hx-get uses same endpoint with action query param
- Buttons arranged in a flex-wrap grid for compact layout

## Current Behavior (MANDATORY)

**Source files read:** (see umbrella for full survey)
- [ ] `cmd/ze-chaos/web/control.go` (699L) - writeControlPanel renders a select dropdown with option elements for each action from chaosActionTypes(); dropdown has hx-get="/control/trigger-form" hx-target="#trigger-params" hx-swap="innerHTML" hx-trigger="change" hx-include="this"; writeTriggerForm renders peer input + optional fraction + Execute badge
  --> Constraint: handleControlTrigger reads "action" form value from POST -- this contract must be preserved
  --> Decision: chaosActionTypes returns 8 actions: tcp-disconnect, notification-cease, hold-timer-expiry, disconnect-during-burst, reconnect-storm, connection-collision, malformed-update, config-reload
- [ ] `cmd/ze-chaos/web/handlers.go` (330L) - handleControlTriggerForm reads action from r.URL.Query().Get("action") and calls writeTriggerForm(w, actionType); handleControlTrigger parses action, peers, params from form POST
  --> Constraint: handleControlTriggerForm serves GET /control/trigger-form with action query param
- [ ] `cmd/ze-chaos/web/assets/style.css` (1066L) - .badge class used for control buttons; .control-row for layout rows; existing hover/active styling on badge elements
  --> Decision: Badge styling already exists for interactive button-like elements

**Behavior to preserve:**
- handleControlTrigger POST endpoint and its form parsing (action, peers, params)
- handleControlTriggerForm GET endpoint and its query param interface
- writeTriggerForm peer selection input and action-specific params rendering
- Execute button behavior and trigger-result feedback div
- All 8 chaos action types and their impact descriptions (tooltips)
- Control panel hidden when control channel is nil

**Behavior to change:**
- Replace select dropdown with individual icon buttons, one per action type
- Each button shows a Unicode icon + short label (e.g., lightning bolt + "Disconnect")
- Clicking a button loads the param form for that action (same hx-get as dropdown change)
- Add visual selection state: clicked button gets active/highlighted style
- Buttons arranged in a flex-wrap grid layout

## Data Flow (MANDATORY - see rules/data-flow-tracing.md)

### Entry Point
- User clicks a trigger button in the control panel (replaces dropdown selection)
- HTMX sends GET /control/trigger-form?action=<type> to load params form (same as dropdown)

### Transformation Path
1. User clicks trigger button (e.g., "Disconnect" button)
2. Button has hx-get="/control/trigger-form?action=tcp-disconnect" hx-target="#trigger-params" hx-swap="innerHTML"
3. handleControlTriggerForm called with action query param (existing, unchanged)
4. writeTriggerForm renders peer input, optional params, Execute button (existing, unchanged)
5. User fills peers, clicks Execute
6. POST /control/trigger with action=tcp-disconnect, peers=..., params=... (existing handler)
7. handleControlTrigger sends ControlCommand to d.control channel (existing)
8. Response HTML replaces trigger-result div (existing)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Browser to Go | HTMX GET /control/trigger-form?action=<type> (same as dropdown) | [ ] |
| Browser to Go | HTMX POST /control/trigger with form data (unchanged) | [ ] |
| Go to Browser | HTML fragment with param form (writeTriggerForm, unchanged) | [ ] |
| Go to Browser | HTML fragment with trigger result (handleControlTrigger, unchanged) | [ ] |

### Integration Points
- writeControlPanel() in control.go - replace dropdown rendering with button grid
- handleControlTriggerForm in handlers.go - no change (still receives action as query param)
- handleControlTrigger in handlers.go - no change (still receives action in POST form)
- writeTriggerForm in control.go - no change (still renders param form for given action)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (buttons replace dropdown, same backend flow)
- [ ] Zero-copy preserved where applicable (no new state, pure rendering change)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | --> | Feature Code | Test |
|-------------|---|--------------|------|
| GET / (full page with control panel) | --> | writeControlPanel renders trigger buttons instead of dropdown | TestControlPanelRendersTriggerButtons |
| Click trigger button (GET /control/trigger-form?action=tcp-disconnect) | --> | handleControlTriggerForm returns param form (existing handler) | TestHandleControlTriggerFormViaButton |
| POST /control/trigger via button flow | --> | handleControlTrigger processes trigger (existing handler) | TestHandleControlTriggerFromButtonFlow |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Control panel rendered with control channel enabled | 8 individual trigger buttons visible instead of dropdown |
| AC-2 | Each trigger button | Shows Unicode icon + short label text |
| AC-3 | Hover over trigger button | Tooltip shows full action impact description (from chaosActionImpact) |
| AC-4 | Click trigger button | Param form loads for that action in #trigger-params (peers input, optional params, Execute) |
| AC-5 | Control panel when stopped | Trigger buttons hidden (same as current dropdown behavior when Status=stopped) |
| AC-6 | Control panel when control channel is nil | Trigger section not rendered (same as current behavior) |
| AC-7 | Button layout | Buttons arranged in a flex-wrap grid row, not vertically stacked |
| AC-8 | Active button state | Clicked button has highlighted/active style to show which action is selected |
| AC-9 | Trigger execution via button flow | POST /control/trigger still works with correct action, peers, params |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWriteTriggerButtons` | `cmd/ze-chaos/web/control_test.go` | New function renders 8 buttons with correct action values | |
| `TestWriteTriggerButtonIcons` | `cmd/ze-chaos/web/control_test.go` | Each button contains a Unicode icon character | |
| `TestWriteTriggerButtonLabels` | `cmd/ze-chaos/web/control_test.go` | Each button contains a short label matching the action type | |
| `TestWriteTriggerButtonTooltips` | `cmd/ze-chaos/web/control_test.go` | Each button has title attribute with chaosActionImpact text | |
| `TestWriteTriggerButtonHTMX` | `cmd/ze-chaos/web/control_test.go` | Each button has hx-get with correct action query param and hx-target="#trigger-params" | |
| `TestControlPanelRendersTriggerButtons` | `cmd/ze-chaos/web/control_test.go` | writeControlPanel output contains trigger buttons, not select/option elements | |
| `TestControlPanelNoTriggerWhenStopped` | `cmd/ze-chaos/web/control_test.go` | writeControlPanel with Status=stopped omits trigger buttons | |
| `TestChaosActionIcon` | `cmd/ze-chaos/web/control_test.go` | chaosActionIcon returns non-empty string for all 8 action types | |
| `TestChaosActionLabel` | `cmd/ze-chaos/web/control_test.go` | chaosActionLabel returns short label for all 8 action types | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Number of action types | 8 (fixed list from chaosActionTypes) | 8 buttons rendered | N/A | N/A |
| Action type string | Non-empty valid string | "config-reload" (last in list) | "" (empty, not rendered) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-trigger-buttons` | `test/chaos/trigger-buttons.ci` | Load dashboard with control panel, verify trigger buttons visible, click one to see param form | |

### Future (if deferring any tests)
- Keyboard navigation between trigger buttons: deferred (not critical for initial implementation)

## Files to Modify
- `cmd/ze-chaos/web/control.go` - replace dropdown rendering in writeControlPanel with writeTriggerButtons function call; add chaosActionIcon helper mapping action types to Unicode icons; add chaosActionLabel helper for short button labels; remove old select/option rendering code
- `cmd/ze-chaos/web/assets/style.css` - trigger button grid styles (display flex, flex-wrap, gap), button sizing (compact icon + label), active/selected state (border-color highlight), hover effects

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
- `test/chaos/trigger-buttons.ci` - functional test for trigger icon buttons

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests for chaosActionIcon and chaosActionLabel helpers** - Review: covers all 8 action types? Returns non-empty values for each?
2. **Run tests** - Verify FAIL (paste output). Fail for RIGHT reason (missing functions)?
3. **Implement chaosActionIcon and chaosActionLabel in control.go** - Map each action type to a Unicode icon (e.g., tcp-disconnect to lightning bolt, notification-cease to stop sign, etc.) and a short label (e.g., "Disconnect", "Cease", "Hold Expire", "Burst Drop", "Storm", "Collision", "Malformed", "Reload").
4. **Run tests** - Verify PASS (paste output). All pass?
5. **Write unit tests for writeTriggerButtons** - Review: renders 8 buttons? Each has hx-get, title, icon, label? No dropdown element?
6. **Run tests** - Verify FAIL.
7. **Implement writeTriggerButtons in control.go** - Render a flex-wrap container with one span.badge per action. Each has hx-get="/control/trigger-form?action=<type>" hx-target="#trigger-params" hx-swap="innerHTML". Icon + label inside span. Title from chaosActionImpact.
8. **Run tests** - Verify PASS.
9. **Write unit test for writeControlPanel rendering buttons instead of dropdown** - Review: output contains trigger buttons? Does not contain select/option elements?
10. **Run tests** - Verify FAIL.
11. **Replace dropdown in writeControlPanel with writeTriggerButtons call** - Remove the select element rendering block. Call writeTriggerButtons in its place. Keep trigger-params and trigger-result divs unchanged.
12. **Add CSS styles** - Trigger button grid: display flex, flex-wrap, gap. Button sizing: compact (icon + short label). Active state: border-color using --accent on selected. Hover: background highlight.
13. **Write functional test** - Create test/chaos/trigger-buttons.ci
14. **Verify all** - make ze-lint and make ze-chaos-test
15. **Critical Review** - All 6 checks from rules/quality.md must pass.
16. **Complete spec** - Fill audit tables, move to done/.

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 or 7 (fix syntax/types) |
| Test fails wrong reason | Step 1 or 5 (fix test expectations) |
| handleControlTrigger no longer receives action | Step 11 (verify buttons include action in hx-get query param) |
| Lint failure | Fix inline |
| Functional test fails | Check AC; verify buttons rendered and param form loads on click |
| Audit finds missing AC | Back to IMPLEMENT for that criterion |
| Dropdown remnants in output | Step 11 (verify old select element rendering fully removed) |

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
- (pending)

### Bugs Found/Fixed
- (pending)

### Documentation Updates
- (pending)

### Deviations from Plan
- (pending)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
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
