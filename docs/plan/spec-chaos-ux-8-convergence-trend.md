# Spec: Convergence Trend Rolling Percentile Chart

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-chaos-ux-0-umbrella.md` - shared research and constraints
4. `cmd/ze-chaos/web/state.go` - ConvergenceHistogram, DashboardState, RingBuffer
5. `cmd/ze-chaos/web/dashboard.go` - ProcessEvent, broadcastDirty, renderConvergence
6. `cmd/ze-chaos/web/viz.go` - writeConvergenceHistogram pattern

## Task

Add a rolling percentile chart showing convergence time trends over the last N convergence events. The chart displays p50, p90, and p99 convergence times as CSS-only horizontal bars or sparkline-style columns (no JS charting library). Updated via a new `convergence-trend` SSE event type or by extending the existing `convergence` event. This tracks **trend over time** -- how convergence latency changes as the run progresses -- whereas the existing ConvergenceHistogram tracks **distribution** (how many routes fall in each latency bucket).

New state: a rolling window buffer of raw convergence latency measurements in DashboardState. New viz panel rendered in a separate file (viz.go is already over 1000 lines). CSS-only chart using bars with inline style widths/heights scaled to the maximum percentile value.

**Parent spec:** `docs/plan/spec-chaos-ux-0-umbrella.md`

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `docs/plan/spec-chaos-ux-0-umbrella.md` - shared research, constraints, source file survey
  → Constraint: All rendering is server-side Go HTML; SSE broadcast every 200ms; ProcessEvent() must be fast
  → Constraint: HTMX + SSE architecture, no JS framework
  → Constraint: viz.go is 1349 lines -- new viz features MUST go in separate files
  → Decision: Dark theme with CSS custom properties (--green, --yellow, --red, --accent)
- [ ] `docs/architecture/chaos-web-dashboard.md` - dashboard architecture
  → Constraint: Convergence histogram broadcasts every ~2s (10 ticks at 200ms)
  → Decision: SSE events use HTML fragments with sse-swap attributes for HTMX outerHTML swap

### RFC Summaries (MUST for protocol work)
- N/A (no protocol work)

**Key insights:**
- ConvergenceHistogram in state.go tracks distribution (latency buckets with counts), not raw values
- A new rolling buffer of raw time.Duration latencies is needed for percentile computation
- RingBuffer generic type already exists in state.go -- instantiate as RingBuffer[time.Duration]
- Percentile computation: sort a copy of the buffer contents, index at the desired percentile rank
- Raw latency push into rolling buffer happens in ProcessEvent (already fast -- single Push call)
- Percentile computation (sort + index) happens at render time, not in the hot path
- Broadcast can share the convergence interval (~2s) by extending broadcastDirty
- CSS-only chart: horizontal bars where width is proportional to latency value, colored by percentile tier

## Current Behavior (MANDATORY)

**Source files read:** (see umbrella for full survey)
- [ ] `cmd/ze-chaos/web/state.go` (594L) - ConvergenceHistogram tracks bucket distribution; Record() adds latency to appropriate bucket; RingBuffer[T] is a generic fixed-size circular buffer with Push, All, Len, Cap methods
  → Constraint: All state behind DashboardState.mu RWMutex
  → Decision: ConvergenceHistogram uses fixed 13 buckets, tracks Total, Sum, Min, Max, SlowCount
- [ ] `cmd/ze-chaos/web/dashboard.go` (695L) - ProcessEvent calls Convergence.Record(latency) when RouteMatrix.RecordReceived returns positive latency; broadcastDirty broadcasts convergence HTML every 10 ticks (~2s); renderConvergence calls writeConvergenceHistogram
  → Constraint: ProcessEvent is synchronous on main event loop, must stay fast (~1us)
  → Constraint: broadcastDirty convergence interval is 10 ticks (convergenceInterval local var)
- [ ] `cmd/ze-chaos/web/viz.go` (1349L) - writeConvergenceHistogram renders histogram bars with inline style heights; uses htmlWriter pattern; outer div has id="viz-convergence" sse-swap="convergence" hx-swap="outerHTML"
  → Constraint: Already over 1000-line threshold -- new viz features MUST go in separate files
  → Decision: Each viz panel renders a full div with HTMX polling attributes for updates

**Behavior to preserve:**
- Existing convergence histogram (distribution view) unchanged
- ProcessEvent latency recording path (Convergence.Record) unchanged
- Convergence SSE broadcast interval (~2s) unchanged
- All existing SSE event types and swap targets unchanged

**Behavior to change:**
- Add new rolling window RingBuffer[time.Duration] to DashboardState for raw convergence latency values
- Push raw latency values in ProcessEvent alongside existing histogram bucket recording
- Add new `convergence-trend` SSE event broadcast alongside existing `convergence` event at same interval
- Add new viz panel for trend chart rendered in a new file (viz_convergence_trend.go)

## Data Flow (MANDATORY - see rules/data-flow-tracing.md)

### Entry Point
- Convergence latency values enter via ProcessEvent when RouteMatrix.RecordReceived returns a positive latency
- The raw latency is already computed -- this feature stores it in a rolling buffer in addition to the histogram

### Transformation Path
1. ProcessEvent receives event with route received, calls RouteMatrix.RecordReceived to get latency
2. Existing: latency recorded in ConvergenceHistogram.Record() (bucket distribution)
3. New: latency also pushed to RingBuffer[time.Duration] in DashboardState (single Push call, fast)
4. broadcastDirty, on convergence tick (~2s), renders the trend chart from the rolling buffer
5. Trend rendering: copy buffer contents via All(), sort the copy, compute p50/p90/p99 by index
6. Render as CSS-only horizontal bars with inline style widths proportional to latency
7. SSE pushes `convergence-trend` event with rendered HTML fragment
8. HTMX swaps fragment into the trend panel div via sse-swap="convergence-trend"

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Event to State | Write lock in ProcessEvent, push latency to rolling buffer | [ ] |
| State to Render | Read lock on DashboardState, copy + sort buffer for percentiles | [ ] |
| Go to Browser | SSE event `convergence-trend` with HTML fragment | [ ] |

### Integration Points
- `ProcessEvent()` in dashboard.go - add rolling buffer push alongside Convergence.Record()
- `broadcastDirty()` in dashboard.go - add convergence-trend rendering on convergence tick
- `registerRoutes()` in handlers.go - add GET /viz/convergence-trend endpoint for HTMX polling fallback
- `DashboardState` in state.go - add ConvergenceTrend RingBuffer[time.Duration] field
- `NewDashboardState()` in state.go - initialize the rolling buffer with capacity

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing convergence recording, does not recreate)
- [ ] Zero-copy preserved where applicable (buffer copy only at render time under read lock)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| ProcessEvent with route-received latency | → | Rolling buffer receives latency value | TestConvergenceTrendBufferRecording |
| broadcastDirty on convergence tick | → | convergence-trend SSE event rendered and broadcast | TestBroadcastDirtyConvergenceTrend |
| GET /viz/convergence-trend | → | handleVizConvergenceTrend returns trend chart HTML | TestHandleVizConvergenceTrend |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Multiple convergence events recorded | Rolling buffer contains raw latency values (oldest evicted when full) |
| AC-2 | GET /viz/convergence-trend with data | Response contains p50, p90, p99 values displayed as labeled horizontal bars |
| AC-3 | GET /viz/convergence-trend with no data | Response contains empty state message (no bars, "awaiting data" text) |
| AC-4 | Rolling buffer at capacity | Oldest values evicted; percentiles reflect only the most recent N values |
| AC-5 | SSE convergence-trend event pushed | HTML fragment contains outer div with sse-swap="convergence-trend" and hx-swap="outerHTML" |
| AC-6 | Trend chart bars rendered | Each percentile bar width is proportional to its value relative to the maximum percentile |
| AC-7 | Trend chart labels rendered | Each bar labeled with percentile name (p50, p90, p99) and formatted duration value |
| AC-8 | Trend chart colors | p50 uses green (--green), p90 uses yellow (--yellow), p99 uses red (--red) |
| AC-9 | Chart is CSS-only | No JS charting library; bars use inline style widths and CSS classes only |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConvergenceTrendRecord` | `cmd/ze-chaos/web/state_test.go` | Push latencies into rolling buffer, verify contents and capacity | |
| `TestConvergenceTrendPercentiles` | `cmd/ze-chaos/web/state_test.go` | Compute p50, p90, p99 from known data set, verify correct values | |
| `TestConvergenceTrendPercentilesEmpty` | `cmd/ze-chaos/web/state_test.go` | Percentiles from empty buffer return zero durations | |
| `TestConvergenceTrendPercentilesOne` | `cmd/ze-chaos/web/state_test.go` | Single value: p50=p90=p99=that value | |
| `TestConvergenceTrendEviction` | `cmd/ze-chaos/web/state_test.go` | Buffer at capacity evicts oldest, percentiles reflect only recent data | |
| `TestWriteConvergenceTrend` | `cmd/ze-chaos/web/viz_convergence_trend_test.go` | Render function produces HTML with three percentile bars and labels | |
| `TestWriteConvergenceTrendEmpty` | `cmd/ze-chaos/web/viz_convergence_trend_test.go` | Render function produces empty state message when no data | |
| `TestWriteConvergenceTrendSSEAttributes` | `cmd/ze-chaos/web/viz_convergence_trend_test.go` | Output contains sse-swap="convergence-trend" and correct div id | |
| `TestWriteConvergenceTrendColors` | `cmd/ze-chaos/web/viz_convergence_trend_test.go` | p50 bar has green class, p90 yellow class, p99 red class | |
| `TestHandleVizConvergenceTrend` | `cmd/ze-chaos/web/handlers_test.go` | HTTP handler returns HTML with correct content type | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Rolling buffer capacity | 1-10000 | 10000 | N/A (minimum 1 enforced by RingBuffer) | N/A (memory-bounded) |
| Buffer with 0 items | empty state | 0 | N/A | N/A |
| Buffer with 1 item | p50=p90=p99=that value | 1 | 0 (empty state) | N/A |
| Buffer with 2 items | p50=item1, p99=item2 | 2 | N/A | N/A |
| Percentile bar width | 0-100% | 100% | 0% (zero latency, min visible) | N/A (capped at 100%) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-convergence-trend` | `test/chaos/convergence-trend.ci` | Dashboard shows convergence trend panel with percentile bars after convergence events | |

### Future (if deferring any tests)
- Time-series sparkline with multiple columns showing trend over sliding time windows -- deferred, initial version shows current percentiles from rolling buffer

## Files to Modify
- `cmd/ze-chaos/web/state.go` - add ConvergenceTrend RingBuffer[time.Duration] field to DashboardState; add ConvergencePercentiles function to compute p50/p90/p99 from sorted buffer copy; initialize in NewDashboardState
- `cmd/ze-chaos/web/dashboard.go` - push raw latency to rolling buffer in ProcessEvent alongside Convergence.Record(); render and broadcast convergence-trend in broadcastDirty at convergence interval
- `cmd/ze-chaos/web/handlers.go` - add GET /viz/convergence-trend route registration and handler function
- `cmd/ze-chaos/web/assets/style.css` - trend bar styles (horizontal bars), percentile label styles, color classes for p50/p90/p99

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
- `cmd/ze-chaos/web/viz_convergence_trend.go` - writeConvergenceTrend render function (new file; viz.go exceeds 1000-line threshold)
- `cmd/ze-chaos/web/viz_convergence_trend_test.go` - unit tests for trend rendering
- `test/chaos/convergence-trend.ci` - functional test for convergence trend panel

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests for rolling buffer recording and percentile computation** - Review: covers empty, single, full, eviction cases? Boundary tests for 0/1/2 items?
2. **Run tests** - Verify FAIL (paste output). Fail for RIGHT reason (missing function/field)?
3. **Implement ConvergenceTrend buffer and ConvergencePercentiles function in state.go** - Add RingBuffer[time.Duration] field to DashboardState, initialize in NewDashboardState with capacity (e.g., 1000). Implement percentiles: copy buffer contents to slice, sort, index at p*len/100.
4. **Run tests** - Verify PASS (paste output). All pass? Any flaky?
5. **Write unit tests for writeConvergenceTrend render function** - Review: checks HTML structure, sse-swap attribute, bar widths proportional to values, labels, empty state, colors?
6. **Run tests** - Verify FAIL.
7. **Create viz_convergence_trend.go with writeConvergenceTrend** - Render three horizontal bars for p50/p90/p99. Each bar: inline style width proportional to value vs max. Labels show percentile name + formatted duration. Outer div has sse-swap="convergence-trend", id, and hx-swap="outerHTML". Include Design comment and Related comment to viz.go.
8. **Run tests** - Verify PASS.
9. **Write unit test for HTTP handler** - Review: correct content type? Correct render function called?
10. **Run tests** - Verify FAIL.
11. **Wire into dashboard** - Add handleVizConvergenceTrend handler in handlers.go. Register GET /viz/convergence-trend route. In ProcessEvent, push latency to rolling buffer alongside Convergence.Record(). In broadcastDirty, render and broadcast convergence-trend SSE event at convergence interval.
12. **Add CSS styles** - Trend bar container, bar height/width, percentile colors using CSS custom properties (--green for p50, --yellow for p90, --red for p99), label font styling.
13. **Write functional test** - Create test/chaos/convergence-trend.ci
14. **Verify all** - make ze-lint and make ze-chaos-test
15. **Critical Review** - All 6 checks from rules/quality.md must pass.
16. **Complete spec** - Fill audit tables, move to done/.

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 or 7 (fix syntax/types) |
| Test fails wrong reason | Step 1 or 5 (fix test) |
| Test fails behavior mismatch | Re-read source from Current Behavior, RESEARCH if misunderstood |
| Percentile computation incorrect | Step 3 (verify sort + index math against known values) |
| Lint failure | Fix inline; if architectural then DESIGN phase |
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
