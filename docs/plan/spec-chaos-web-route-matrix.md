# Spec: chaos-web-route-matrix

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/architecture/chaos-web-dashboard.md` - "Route Flow Matrix" in Visualization Tabs
3. `cmd/ze-bgp-chaos/web/state.go` - existing state types
4. `cmd/ze-bgp-chaos/web/handlers.go` - existing handler patterns

## Task

Add the **Route Flow Matrix** visualization tab to the chaos web dashboard: a peer-to-peer heatmap showing route propagation counts and latency between peers via the Ze route reflector.

This is the most complex visualization because a 200×200 matrix is 40,000 cells. The design uses **top-N filtering** (default: top 20 by route count) with dropdowns for peer selection, family filtering, and a count/latency toggle.

**Parent spec:** `docs/plan/spec-chaos-web-dashboard.md`
**Design doc:** `docs/architecture/chaos-web-dashboard.md` (section "Tab 5: Route Flow Matrix")
**Depends on:** `spec-chaos-web-foundation` (web package, state, handlers)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/chaos-web-dashboard.md` - Route Flow Matrix design
  -> Decision: CSS grid with inline background-color opacity for heatmap
  -> Decision: Top 20 peers by route count as default view
  -> Decision: Count view (default) vs latency view toggle
  -> Constraint: Source peer inferred from model's announced routes
- [ ] `cmd/ze-bgp-chaos/web/state.go` - Existing state types
  -> Decision: Route matrix = N×N slice of route counts
- [ ] `cmd/ze-bgp-chaos/validation/model.go` - Model tracks which peer announced which route
  -> Constraint: When peer P receives route R, source = the peer that announced R

**Key insights:**
- Route source tracking requires cross-referencing received routes against the model's announced routes
- For 200+ peers, only the top 20 most active peers are shown by default — full matrix is too large
- CSS grid renders the heatmap — each cell is a div with opacity proportional to value
- Click a cell to see detail: which routes, latency breakdown

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze-bgp-chaos/web/state.go` - Per-peer state from foundation
- [ ] `cmd/ze-bgp-chaos/web/handlers.go` - Handler patterns from foundation
- [ ] `cmd/ze-bgp-chaos/validation/model.go` - Model.Announce() tracks route origins

**Behavior to preserve:**
- Foundation dashboard, peer table, other tabs unchanged
- Model tracking unchanged (read-only from web dashboard perspective)

**Behavior to change:**
- WebDashboard state extended with N×N route matrix
- ProcessEvent(RouteReceived) updates matrix by inferring source peer from model

## Data Flow (MANDATORY)

### Entry Point
- Events from peer.Event channel -> Reporter.Process() -> WebDashboard.ProcessEvent()
- Specifically EventRouteReceived triggers matrix updates

### Transformation Path
1. ProcessEvent(EventRouteReceived) fires with peerIndex=B, prefix=R
2. Dashboard looks up "who announced R?" in the validation model -> peer A
3. matrix[A][B]++ (source=A, destination=B)
4. Per-family matrix variant updated if family filter is active
5. GET /viz/route-matrix?top=20 reads matrix state (read lock)
6. Handler sorts peers by total route count, takes top N
7. Renders N×N CSS grid with color-coded cells as HTML fragment

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Main loop -> WebDashboard | ProcessEvent() call (synchronous) | [ ] |
| WebDashboard -> validation model | Read-only lookup of route origins | [ ] |
| Browser -> HTTP server | HTMX GET for /viz/route-matrix | [ ] |

### Integration Points
- `web/state.go` — N×N route count matrix, per-family matrix variant
- `web/dashboard.go` ProcessEvent() — Matrix update on EventRouteReceived
- `validation/model.go` Model.Announce() — Read-only source peer inference
- `web/handlers.go` — /viz/route-matrix handler with top/family/view params

### Architectural Verification
- [ ] No bypassed layers (events flow through Reporter)
- [ ] No unintended coupling (web reads model read-only, no writes)
- [ ] No duplicated functionality (extends foundation state, doesn't recreate)

### Route Source Inference
1. Peer A announces route R -> model records "peer A announced R"
2. Ze reflects route R to peer B
3. Peer B receives R -> EventRouteReceived(peerIndex=B, prefix=R)
4. ProcessEvent looks up "who announced R?" in the model -> peer A
5. matrix[A][B]++ (source A, destination B)

### Rendering
1. GET /viz/route-matrix?top=20 -> handler reads matrix state
2. Sort peers by total route count, take top 20
3. Render 20×20 CSS grid with color-coded cells
4. Return HTML fragment

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Route matrix tab selected | Heatmap renders with top 20 peers |
| AC-2 | Cell shows route count | Color intensity proportional to count |
| AC-3 | Cell clicked | Detail popup shows route list and latency stats |
| AC-4 | "Latency" toggle selected | Cells show avg latency instead of count, color = latency |
| AC-5 | Family filter set to "ipv4/unicast" | Matrix shows only IPv4 unicast route counts |
| AC-6 | Custom peer selection | Dropdown to select specific peers instead of top-N |
| AC-7 | 200+ peers | Top 20 shown by default, no layout issues |
| AC-8 | Peer with zero routes in matrix | Cell empty (no color), not rendered as zero |
| AC-9 | Matrix updates during run | Refreshed on tab activation (not continuous SSE — too heavy) |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestRouteMatrixUpdate | `web/state_test.go` | ProcessEvent(RouteReceived) increments matrix[source][dest] | |
| TestRouteMatrixSourceInference | `web/state_test.go` | Source peer correctly inferred from model | |
| TestRouteMatrixTopN | `web/handlers_test.go` | /viz/route-matrix?top=20 returns 20×20 grid | |
| TestRouteMatrixFamilyFilter | `web/handlers_test.go` | ?family=ipv4/unicast filters by family | |
| TestRouteMatrixLatencyToggle | `web/handlers_test.go` | ?view=latency returns avg latency per cell | |
| TestRouteMatrixEmptyCells | `web/state_test.go` | Zero-count cells not rendered | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| test-web-route-matrix | `test/chaos/web-route-matrix.ci` | GET /viz/route-matrix returns HTML with grid | |

## Files to Modify

- `cmd/ze-bgp-chaos/web/state.go` - Add route matrix (N×N slice), per-family matrix variant
- `cmd/ze-bgp-chaos/web/dashboard.go` - ProcessEvent updates matrix on RouteReceived
- `cmd/ze-bgp-chaos/web/handlers.go` - Add /viz/route-matrix handler

## Files to Create

- `cmd/ze-bgp-chaos/web/templates/route_matrix.html` - Heatmap CSS grid template
- `test/chaos/web-route-matrix.ci` - Functional test

## Implementation Steps

1. **Add matrix state (TDD)** - N×N route count matrix, source inference from model
2. **ProcessEvent extension (TDD)** - RouteReceived updates matrix
3. **Top-N sorting** - Sort peers by total count, return top 20
4. **Heatmap handler (TDD)** - /viz/route-matrix with top/family/view params
5. **CSS grid template** - Cells with opacity-based coloring
6. **Cell detail** - Click handler for route list popup
7. **Family filter and latency toggle** - Dropdown controls
8. **Functional test**

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Route flow heatmap | | | |
| Source peer inference | | | |
| Top-N filtering (200+) | | | |
| Family filter | | | |
| Count/latency toggle | | | |
| Cell detail popup | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | | | |
| AC-2 | | | |
| AC-3 | | | |
| AC-4 | | | |
| AC-5 | | | |
| AC-6 | | | |
| AC-7 | | | |
| AC-8 | | | |
| AC-9 | | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**

## Checklist

### Goal Gates
- [ ] AC-1..AC-9 demonstrated
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] `make ze-lint` passes
- [ ] Heatmap renders correctly in browser for 20+ peers

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
