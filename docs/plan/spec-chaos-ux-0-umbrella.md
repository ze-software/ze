# Spec: Chaos Dashboard UX Overhaul (Umbrella)

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/chaos-web-dashboard.md` - dashboard architecture
4. Child specs: `docs/plan/spec-chaos-ux-{1..10}-*.md`

## Task

Overhaul the ze-chaos web dashboard UX with 10 prioritized improvements. This umbrella spec captures shared research, architectural constraints, and the dependency graph. Each improvement has its own child spec with specific ACs, tests, and implementation steps.

### Child Specs

| Spec | Priority | Effort | Impact | Depends On |
|------|----------|--------|--------|------------|
| `spec-chaos-ux-1-peer-grid.md` — Peer Grid toggle view | 1 | Medium | High | — |
| `spec-chaos-ux-2-health-donut.md` — Health ring in sidebar | 2 | Low | High | — |
| `spec-chaos-ux-3-event-toasts.md` — Chaos event toasts | 3 | Low | Medium | — |
| `spec-chaos-ux-4-chaos-pulse.md` — Peer cell pulse animation | 4 | Low | Medium | 1 (needs grid) |
| `spec-chaos-ux-5-control-strip.md` — Horizontal control strip | 5 | Medium | Medium | — |
| `spec-chaos-ux-6-multi-panel.md` — Multi-panel viz layout | 6 | Medium | High | — |
| `spec-chaos-ux-7-peer-filter.md` — Peer text search/filter | 7 | Low | Medium | — |
| `spec-chaos-ux-8-convergence-trend.md` — Rolling percentile chart | 8 | Medium | Medium | — |
| `spec-chaos-ux-9-control-feedback.md` — Rate/speed feedback | 9 | Low | Low | — |
| `spec-chaos-ux-10-trigger-buttons.md` — Trigger icon buttons | 10 | Low | Low | — |

### Suggested Implementation Order

Phase A (independent, high impact): 2, 3, 7, 9, 10
Phase B (layout changes): 1, 5, 6
Phase C (depends on grid): 4
Phase D (new state): 8

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/chaos-web-dashboard.md` — dashboard architecture
  → Constraint: All rendering is server-side Go HTML fragments pushed via SSE or pulled via HTMX
  → Constraint: SSE broadcast loop runs every 200ms; animations must be CSS-only or minimal vanilla JS
  → Constraint: ProcessEvent() must be fast (~1us), runs on main event loop under write lock
  → Decision: Dark theme with CSS custom properties (--bg-primary, --green, --red, --accent, etc.)

**Key insights:**
- HTMX + SSE, no JS framework — all new UI elements must be Go-rendered HTML
- SSE events: `stats`, `events`, `convergence`, `peer-update`, `peer-add`, `peer-remove`
- Layout: CSS Grid `header + (300px sidebar + main)`, responsive breakpoint at 900px
- Active set manages visible peers (max 40) with priority-based promotion and adaptive TTL decay
- viz.go is 1349 lines — new viz features should go in separate files
- Convergence histogram broadcasts every ~2s (10 ticks)
- Control commands sent via non-blocking channel sends

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze-chaos/web/dashboard.go` (695L) — Dashboard struct, New(), ProcessEvent(), broadcast loop, renderStats(), renderPeerRow()
  → Constraint: ProcessEvent() is synchronous on main event loop, must stay fast
  → Constraint: broadcastDirty() runs under write lock for ConsumeDirty(), then read lock for rendering
- [ ] `cmd/ze-chaos/web/state.go` (594L) — DashboardState, PeerState, PeerStatus enum, RingBuffer, ConvergenceHistogram, formatting helpers
  → Constraint: All state behind DashboardState.mu RWMutex
  → Decision: PeerStatus is iota enum (Idle=0, Up=1, Down=2, Reconnecting=3, Syncing=4)
- [ ] `cmd/ze-chaos/web/handlers.go` (330L) — Route registration, handlePeers (sort/filter), handlePeerDetail, handlePeerPin, sidebar handlers
  → Constraint: handlePeers supports sort by column + status filter (including "fault" mode)
  → Decision: Filter uses ps.Status.String() == statusFilter for string-based matching
- [ ] `cmd/ze-chaos/web/render.go` (470L) — writeLayout() full-page HTML, writePeerRows(), writePeerDetail(), htmlWriter helper
  → Constraint: writeLayout() is the single entry point for full-page render — all structural HTML lives here
  → Decision: Layout structure: header → content → sidebar + main → tab-bar + viz-content
- [ ] `cmd/ze-chaos/web/viz.go` (1349L) — 7 viz tab handlers + renderers (events, convergence, timeline, chaos, matrix, families, all-peers)
  → Constraint: Already over 1000-line threshold — new viz features MUST go in separate files
  → Decision: Each viz renders a full panel div with HTMX polling attributes for updates
- [ ] `cmd/ze-chaos/web/control.go` (699L) — Control panel handlers, manual trigger form, route dynamics, speed, restart
  → Constraint: Control commands sent non-blocking to d.control channel; nil channel = UI hidden
  → Decision: writeTriggerForm() renders full form with action dropdown + peer select + params
- [ ] `cmd/ze-chaos/web/sse.go` (160L) — SSEBroker with per-client buffered channels (cap 64)
  → Constraint: Broadcast is non-blocking, drops events if client buffer full
- [ ] `cmd/ze-chaos/web/state_activeset.go` (211L) — ActiveSet with promotion priorities, decay, pinning
  → Decision: Priorities: Manual > High (disconnect/error) > Medium (chaos/reconnect) > Low (missing)
- [ ] `cmd/ze-chaos/web/state_routematrix.go` (393L) — RouteMatrix for heatmap, prefix tracking with eviction at 100K
- [ ] `cmd/ze-chaos/web/assets/style.css` (1066L) — Dark theme, CSS Grid layout, all component styles
  → Constraint: CSS custom properties for theming; responsive at 900px breakpoint
  → Decision: Font stacks: --font-mono (SFMono), --font-sans (system); base 14px
- [ ] `cmd/ze-chaos/web/assets/sse.js` (291L) — HTMX SSE extension with exponential backoff retry

**Behavior to preserve:**
- Existing SSE event types and their swap targets
- Active set promotion/decay behavior
- Table sorting and filtering (status, fault mode)
- Control panel functionality (pause/resume/rate/trigger/stop/speed/restart)
- All existing viz tabs and their data sources
- Responsive behavior at 900px breakpoint

**Behavior to change:**
- Add peer grid as alternative view (toggle, not replacement)
- Add health donut to sidebar (replaces flat counter display)
- Add event toast notifications (new SSE event type)
- Add chaos pulse CSS animation (on grid cells)
- Move controls to horizontal strip below header
- Support multi-panel viz layout (alongside existing tabs)
- Add text search/filter for peers
- Add convergence trend chart (rolling percentiles)
- Add rate/speed feedback to control panel
- Replace trigger dropdown with icon buttons

## Data Flow (MANDATORY)

### Entry Point
- **Events** enter via `Dashboard.ProcessEvent(ev)` — called from reporter fan-out on main loop
- **User actions** enter via HTTP handlers (HTMX POST/GET requests)
- **SSE push** exits via `SSEBroker.Broadcast()` with rendered HTML fragments

### Transformation Path
1. Event arrives → ProcessEvent updates DashboardState under write lock → marks dirty flags
2. Broadcast loop (every 200ms) → ConsumeDirty() under write lock → render HTML under read lock
3. SSE broker pushes HTML fragments to all connected clients
4. HTMX client swaps HTML into DOM based on sse-swap attributes
5. CSS applies styling, transitions, and animations

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Go → Browser | SSE event with HTML fragment | [ ] |
| Browser → Go | HTMX HTTP request (hx-get/hx-post) | [ ] |
| State → Render | Read lock on DashboardState | [ ] |
| Event → State | Write lock in ProcessEvent() | [ ] |

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

### Integration Points
- `report.Consumer` interface — ProcessEvent entry point for all event-driven features
- `SSEBroker.Broadcast()` — push channel for all new SSE event types
- `registerRoutes()` in handlers.go — registration point for all new HTTP endpoints
- `writeLayout()` in render.go — structural HTML changes (control strip, multi-panel)
- `broadcastDirty()` in dashboard.go — render loop for new dirty-flag-driven content

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | | Feature Code | Test |
|-------------|---|--------------|------|
| Full page load (GET /) | | writeLayout() renders all new UI elements | TestHandleIndex (existing, extend) |
| SSE connection (/events) | | New event types broadcast correctly | One wiring test per child spec |
| Each child spec defines its own wiring tests | | See child specs | See child specs |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | All 10 child specs written | Each has ACs, wiring tests, TDD plan, implementation steps |
| AC-2 | Dependency graph documented | Children with dependencies reference the prerequisite spec |
| AC-3 | Shared research captured | Umbrella has source file survey, constraints, data flow |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Umbrella has no implementation code | N/A | This is a coordination spec | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| No numeric inputs in umbrella | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Each child spec defines its own functional tests | See child specs | See child specs | |

## Files to Modify
- `docs/plan/spec-chaos-ux-{1..10}-*.md` — child spec files (created by this umbrella)
- `docs/architecture/chaos-web-dashboard.md` — update after all children complete

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
- `docs/plan/spec-chaos-ux-1-peer-grid.md` — Peer Grid toggle view
- `docs/plan/spec-chaos-ux-2-health-donut.md` — Health ring in sidebar
- `docs/plan/spec-chaos-ux-3-event-toasts.md` — Chaos event toasts
- `docs/plan/spec-chaos-ux-4-chaos-pulse.md` — Peer cell pulse animation
- `docs/plan/spec-chaos-ux-5-control-strip.md` — Horizontal control strip
- `docs/plan/spec-chaos-ux-6-multi-panel.md` — Multi-panel viz layout
- `docs/plan/spec-chaos-ux-7-peer-filter.md` — Peer text search/filter
- `docs/plan/spec-chaos-ux-8-convergence-trend.md` — Rolling percentile chart
- `docs/plan/spec-chaos-ux-9-control-feedback.md` — Rate/speed feedback
- `docs/plan/spec-chaos-ux-10-trigger-buttons.md` — Trigger icon buttons

## Implementation Steps

1. **Write all child specs** → Review: each has complete ACs, wiring tests, TDD plan
2. **Review dependency graph** → Verify no circular dependencies, implementation order is sound
3. **Implement children in phase order** → Phase A first (independent), then B (layout), C (grid-dependent), D (new state)
4. **After each child** → Run `make ze-chaos-test`, verify no regressions
5. **After all children** → Update `docs/architecture/chaos-web-dashboard.md`
6. **Move umbrella + all children to done/**

### Failure Routing

| Failure | Route To |
|---------|----------|
| Child spec validation fails | Fix child spec sections |
| Dependency conflict between children | Revisit umbrella dependency graph |
| Layout changes conflict | Coordinate control-strip + multi-panel specs |

## Cross-Cutting Concerns

### SSE Event Types
New features may need new SSE event types:
- Event toasts: new `toast` event
- Convergence trend: new `convergence-trend` event (or extend existing `convergence`)
- Peer grid: reuse existing `peer-update`/`peer-add`/`peer-remove` with grid-aware rendering

### CSS Architecture
All new components must use:
- Existing CSS custom properties for colors
- Existing font stacks (--font-mono, --font-sans)
- Consistent card/panel styling patterns
- Dark theme only (no light mode)

### Testing Pattern
All new handlers/renderers need:
- Unit test: render function produces correct HTML
- Integration test: HTTP handler returns expected fragment
- For new SSE events: test that ProcessEvent + broadcastDirty produces the event

## Checklist

### Goal Gates (MUST pass)
- [ ] All child specs written with ACs, wiring tests, and TDD plans
- [ ] Dependency graph documented
- [ ] No overlapping file modifications without coordination noted
- [ ] `make ze-chaos-test` passes after each child spec implementation

### Quality Gates (SHOULD pass)
- [ ] `make ze-lint` passes
- [ ] Architecture doc updated after all children complete
- [ ] File modularity maintained (no file > 1000 lines without split plan)

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

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec. One commit = code + tests + spec.

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
