# Spec: Control Panel Rate and Speed Feedback

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-chaos-ux-0-umbrella.md` - shared research and constraints
4. `cmd/ze-chaos/web/control.go` - writeControlPanel, writeSpeedControl, rate slider
5. `cmd/ze-chaos/web/dashboard.go` - renderStats, broadcastDirty
6. `cmd/ze-chaos/web/state.go` - ControlState, DashboardState

## Task

Show current rate and speed values as live feedback in the control panel. Display the actual chaos event rate (events/sec) and the current speed multiplier as live-updating values. Updates arrive via the existing `stats` SSE event (extend existing renderStats in dashboard.go). Visual indicator: colored number that changes color at thresholds (green for low chaos rate, yellow for moderate, red for high). The control panel already has sliders for rate and speed -- this adds live readback of actual observed values, not just the configured slider position.

**Parent spec:** `docs/plan/spec-chaos-ux-0-umbrella.md`

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `docs/plan/spec-chaos-ux-0-umbrella.md` - shared research, constraints, source file survey
  → Constraint: All rendering is server-side Go HTML; SSE broadcast every 200ms; ProcessEvent() must be fast
  → Constraint: HTMX + SSE architecture, no JS framework
  → Decision: Dark theme with CSS custom properties (--green, --yellow, --red)
- [ ] `docs/architecture/chaos-web-dashboard.md` - dashboard architecture
  → Constraint: Control commands sent non-blocking to d.control channel; nil channel = UI hidden
  → Decision: SSE stats event broadcasts every dirty tick (~200ms when activity occurs)

### RFC Summaries (MUST for protocol work)
- N/A (no protocol work)

**Key insights:**
- renderStats() in dashboard.go produces the stats SSE fragment; right place to add rate/speed feedback
- ControlState already has Rate (float64 0.0-1.0) and SpeedFactor (int 1/10/100/1000)
- Actual chaos event rate can be computed from TotalChaos delta over time (EMA like throughput)
- Current speed factor is already tracked in ControlState.SpeedFactor
- The stats div is re-rendered on every dirty broadcast -- feedback updates in near-real-time
- Color thresholds: green (low activity), yellow (moderate), red (high chaos rate)
- Existing throughput EMA pattern in UpdateThroughput can be reused for chaos rate EMA

## Current Behavior (MANDATORY)

**Source files read:** (see umbrella for full survey)
- [ ] `cmd/ze-chaos/web/control.go` (699L) - writeControlPanel renders rate slider with percentage display and speed buttons with factor labels; handlers update ControlState and re-render panel
  → Constraint: writeControlPanel takes pointer to ControlState; re-rendered on every control action
  → Decision: Rate slider shows configured percentage; speed buttons show discrete factor (1x/10x/100x/1000x)
- [ ] `cmd/ze-chaos/web/dashboard.go` (695L) - renderStats produces stats HTML fragment for SSE; includes peer counts, message counts, byte counts, throughput, chaos count, reconnects; broadcast on every dirty tick
  → Constraint: renderStats runs under read lock on DashboardState
  → Decision: Stats fragment has id="stats" sse-swap="stats" hx-swap="outerHTML"
- [ ] `cmd/ze-chaos/web/state.go` (594L) - ControlState has Rate, SpeedFactor, SpeedAvailable, Paused, Status; DashboardState has TotalChaos (int), StartTime (time.Time), throughputEMAAlpha pattern for EMA computation
  → Constraint: ControlState.Rate is 0.0-1.0; SpeedFactor is 1, 10, 100, or 1000
  → Decision: UpdateThroughput uses EMA with alpha=0.3 on per-peer byte deltas; same pattern works for chaos rate

**Behavior to preserve:**
- Existing stats sidebar content and formatting
- Rate slider and speed button functionality unchanged
- Control panel layout (writeControlPanel, writeSpeedControl)
- Stats SSE event type and swap target unchanged
- Existing throughput EMA computation pattern

**Behavior to change:**
- Add actual chaos event rate (events/sec EMA) tracking to DashboardState
- Add chaos rate and speed readback values to renderStats output
- Color-code the rate readback: green (low), yellow (moderate), red (high)
- Show current speed factor in stats if speed control is available

## Data Flow (MANDATORY - see rules/data-flow-tracing.md)

### Entry Point
- Chaos events enter via ProcessEvent with EventChaosExecuted type, incrementing TotalChaos
- Speed changes enter via handleControlSpeed updating ControlState.SpeedFactor
- Rate changes enter via handleControlRate updating ControlState.Rate

### Transformation Path
1. ProcessEvent increments TotalChaos on EventChaosExecuted (existing, unchanged)
2. broadcastDirty calls UpdateThroughput (existing EMA) -- add chaos rate EMA alongside
3. renderStats reads chaos rate EMA and speed factor from state under read lock
4. renderStats renders colored rate value and speed factor into stats HTML fragment
5. SSE pushes stats event with fragment to browser
6. HTMX swaps into stats div (existing swap target, unchanged)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Event to State | Write lock in ProcessEvent, increment TotalChaos (existing) | [ ] |
| State to EMA | Write lock in broadcastDirty via UpdateThroughput, compute chaos rate EMA | [ ] |
| State to Render | Read lock in renderStats, read chaos rate EMA and speed factor | [ ] |
| Go to Browser | SSE stats event with rate/speed feedback HTML in stats fragment | [ ] |

### Integration Points
- `ProcessEvent()` in dashboard.go - no change (already increments TotalChaos)
- `UpdateThroughput()` in state.go or broadcastDirty in dashboard.go - add chaos event rate EMA
- `renderStats()` in dashboard.go - add rate and speed feedback spans
- `DashboardState` in state.go - add chaos rate EMA fields (prevTotalChaos, chaosRate float64)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing stats rendering, reuses EMA pattern)
- [ ] Zero-copy preserved where applicable (no new allocations in hot path)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| GET / (full page load) | → | renderStats includes chaos rate feedback | TestRenderStatsIncludesChaosRate |
| Stats SSE event after chaos events | → | Stats fragment contains colored rate value | TestBroadcastStatsWithChaosRate |
| GET / with speed control enabled | → | renderStats includes speed factor readback | TestRenderStatsIncludesSpeedFactor |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Chaos events occurring | Stats panel shows actual chaos event rate (events/sec) with EMA smoothing |
| AC-2 | Chaos rate is low (under 1 event/sec) | Rate value displayed in green color |
| AC-3 | Chaos rate is moderate (1-5 events/sec) | Rate value displayed in yellow color |
| AC-4 | Chaos rate is high (over 5 events/sec) | Rate value displayed in red color |
| AC-5 | No chaos events yet | Rate shows "0.0/s" in default color |
| AC-6 | Speed control enabled | Stats panel shows current speed factor (e.g., "100x") |
| AC-7 | Speed control disabled | No speed readback shown in stats |
| AC-8 | User changes rate via slider | Chaos rate EMA reflects the change over subsequent broadcasts |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestChaosRateEMA` | `cmd/ze-chaos/web/state_test.go` | Chaos event rate EMA computes correct events/sec from event deltas | |
| `TestChaosRateEMAZero` | `cmd/ze-chaos/web/state_test.go` | Zero chaos events gives 0.0 rate | |
| `TestChaosRateEMABurst` | `cmd/ze-chaos/web/state_test.go` | Burst of chaos events produces high rate, decays over time | |
| `TestChaosRateColorClass` | `cmd/ze-chaos/web/state_test.go` | Rate below 1/s returns green class, 1-5/s returns yellow, above 5/s returns red | |
| `TestRenderStatsIncludesChaosRate` | `cmd/ze-chaos/web/dashboard_test.go` | renderStats output contains chaos rate span with value | |
| `TestRenderStatsIncludesSpeedFactor` | `cmd/ze-chaos/web/dashboard_test.go` | renderStats output contains speed factor when SpeedAvailable is true | |
| `TestRenderStatsNoSpeedWhenDisabled` | `cmd/ze-chaos/web/dashboard_test.go` | renderStats output omits speed factor when SpeedAvailable is false | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Chaos rate (events/sec) | 0.0+ | N/A (unbounded float) | 0.0 (valid, green) | N/A |
| Color threshold low | 0.0 to 1.0 events/sec | 1.0 (green) | N/A | 1.01 (yellow) |
| Color threshold high | 5.0+ events/sec | 5.0 (yellow) | 4.99 (yellow) | 5.01 (red) |
| Speed factor | 1, 10, 100, 1000 | 1000 | N/A (1 is min) | N/A (1000 is max) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-control-feedback` | `test/chaos/control-feedback.ci` | Load dashboard with chaos running, verify chaos rate value appears in stats panel | |

### Future (if deferring any tests)
- Comparison of displayed rate vs actual measured event frequency: deferred (requires event counting infrastructure)

## Files to Modify
- `cmd/ze-chaos/web/state.go` - add chaos rate EMA fields to DashboardState (prevTotalChaos int, chaosRate float64); add chaosRateColorClass helper function
- `cmd/ze-chaos/web/dashboard.go` - add chaos rate EMA update in broadcastDirty alongside UpdateThroughput; add chaos rate and speed factor spans to renderStats output
- `cmd/ze-chaos/web/assets/style.css` - add styles for rate feedback coloring (if not already covered by existing stat classes)

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
- `test/chaos/control-feedback.ci` - functional test for rate and speed feedback

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests for chaos rate EMA computation** - Review: covers zero events, steady rate, burst then decay? Uses same EMA alpha as throughput?
2. **Run tests** - Verify FAIL (paste output). Fail for RIGHT reason (missing fields/methods)?
3. **Implement chaos rate EMA in state.go** - Add prevTotalChaos and chaosRate fields to DashboardState. Follow existing throughputEMAAlpha pattern. Add update method or integrate into UpdateThroughput.
4. **Run tests** - Verify PASS (paste output). All pass? Any flaky?
5. **Write unit tests for chaosRateColorClass** - Review: tests green/yellow/red thresholds?
6. **Run tests** - Verify FAIL.
7. **Implement chaosRateColorClass helper** - Return CSS class name based on rate thresholds (under 1/s green, 1-5/s yellow, over 5/s red).
8. **Run tests** - Verify PASS.
9. **Write unit tests for renderStats output** - Review: chaos rate span present? Speed factor span present when enabled? Absent when disabled?
10. **Run tests** - Verify FAIL.
11. **Add chaos rate and speed factor to renderStats in dashboard.go** - Append spans to existing stats HTML. Chaos rate span: colored by threshold class, shows "N.N/s". Speed span: shows "Nx" factor when SpeedAvailable is true.
12. **Add CSS styles if needed** - Color classes for rate feedback (may reuse existing status colors).
13. **Write functional test** - Create test/chaos/control-feedback.ci
14. **Verify all** - make ze-lint and make ze-chaos-test
15. **Critical Review** - All 6 checks from rules/quality.md must pass.
16. **Complete spec** - Fill audit tables, move to done/.

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 or 7 (fix syntax/types) |
| Test fails wrong reason | Step 1 or 5 (fix test expectations) |
| EMA values incorrect | Step 3 (verify EMA formula matches throughput pattern in state.go) |
| Color thresholds wrong | Step 7 (adjust threshold constants) |
| Lint failure | Fix inline |
| Functional test fails | Check AC; verify renderStats actually outputs the new spans |
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
- [ ] AC-1..AC-8 all demonstrated
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
