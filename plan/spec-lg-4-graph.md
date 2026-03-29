# Spec: lg-4 -- AS Path Topology Graph

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-lg-3-ui, spec-decorator |
| Phase | - |
| Updated | 2026-03-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-lg-0-umbrella.md` - umbrella spec with design decisions
4. `internal/component/lg/handler_ui.go` - UI handlers (from lg-3-ui)
5. `internal/component/lg/templates/` - existing LG templates (from lg-3-ui)
6. `internal/component/web/decorator.go` - decorator framework (from spec-decorator)

## Task

Implement AS path topology graph visualization for the looking glass. When a user looks up
a prefix, the results include an SVG graph showing the AS path topology: nodes represent
autonomous systems (labeled with ASN and organization name), edges represent peering
relationships.

The graph is rendered server-side as SVG using Go `html/template`. A layered layout
algorithm assigns each AS to a column based on hop depth, producing a left-to-right
directed acyclic graph. The SVG is returned as an HTMX fragment and embedded inline in
the route lookup results page.

This approach requires zero external dependencies (no GraphViz binary, no WASM, no JS
graph library). The layout algorithm is purpose-built for AS path graphs, which are
constrained DAGs typically containing 3-10 nodes.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component architecture
  -> Decision:
  -> Constraint:

### RFC Summaries (MUST for protocol work)
- N/A

**Key insights:**
- To be filled during implementation

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/lg/handler_ui.go` - UI handlers (from lg-3-ui)
- [ ] `internal/component/lg/render.go` - LG template renderer (from lg-3-ui)
- [ ] `internal/component/lg/templates/page/lookup.html` - route lookup page (from lg-3-ui)
- [ ] `internal/component/web/decorator.go` - decorator framework (from spec-decorator)

**Behavior to preserve:**
- Route lookup page from lg-3-ui
- Decorator framework from spec-decorator
- HTMX partial update patterns

**Behavior to change:**
- Route lookup results gain a "Show topology" link/button
- New graph endpoint returns SVG fragment
- New graph data model and layout algorithm

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- User clicks "Show topology" on a route lookup result
- HTMX sends `hx-get="/lg/graph?prefix=10.0.0.0/24"` to LG server

### Transformation Path
1. Graph handler receives prefix from query parameter
2. Handler calls CommandDispatcher to get all routes for prefix
3. Handler extracts AS paths from route results
4. Handler builds graph data model: nodes (unique ASes) + edges (AS-to-AS links)
5. Handler calls decorator to resolve ASN names for all nodes
6. Layout algorithm assigns x,y coordinates to each node (layered, left-to-right)
7. SVG template renders nodes as rectangles with labels, edges as lines with arrows
8. SVG fragment returned to browser, HTMX swaps into graph container

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Browser -> LG server | HTMX hx-get request | [ ] |
| Handler -> Engine | CommandDispatcher for route data | [ ] |
| Handler -> Decorator | ASN name resolution for node labels | [ ] |

### Integration Points
- LG server mux (register graph handler)
- CommandDispatcher for route data
- Decorator framework for ASN names
- LG template renderer for SVG output
- Route lookup page template (add graph container + trigger link)

### Architectural Verification
- [ ] No bypassed layers (route data via CommandDispatcher)
- [ ] No unintended coupling (graph code has no external graph library imports)
- [ ] No duplicated functionality (reuses existing route query, decorator)
- [ ] Zero-copy preserved where applicable (N/A)

## Graph Data Model

### Nodes

| Field | Type | Source |
|-------|------|--------|
| ASN | uint32 | From AS path |
| Name | string | From decorator (ASN name) |
| Layer | int | Computed: hop depth from origin |
| Y position | int | Computed: vertical position within layer |

### Edges

| Field | Type | Source |
|-------|------|--------|
| From ASN | uint32 | AS path element N |
| To ASN | uint32 | AS path element N+1 |

### Graph Construction

Given multiple AS paths to the same prefix from different peers:

1. Collect all unique ASes across all paths (nodes)
2. Collect all unique AS-to-AS links across all paths (edges)
3. Assign layers: origin AS (rightmost in path) at layer 0, each hop adds a layer leftward
4. If an AS appears at different depths in different paths, use the maximum depth
5. Within each layer, sort nodes by ASN for deterministic layout

## Layout Algorithm

Layered layout for small DAGs (Sugiyama-inspired, simplified for AS paths):

### Step 1: Layer Assignment
- Origin AS (last in path) at layer 0 (rightmost column)
- Each preceding AS gets layer = max(layer of successor) + 1
- Source ASes (peers) at the leftmost layer

### Step 2: Node Positioning
- Each layer is a vertical column
- Nodes within a column spaced evenly
- Column width based on widest label (ASN + name)
- Inter-column spacing fixed

### Step 3: Edge Routing
- Straight lines between connected nodes
- Arrow heads at destination end
- For multi-hop gaps (AS prepending removed), direct connection

### SVG Dimensions

| Parameter | Value | Notes |
|-----------|-------|-------|
| Node width | Computed from label text | Min 80px, max 200px |
| Node height | 40px | Fixed |
| Horizontal spacing | 120px | Between layers |
| Vertical spacing | 60px | Between nodes in same layer |
| Padding | 20px | Around entire graph |
| Font | 12px monospace | ASN + name label |

## SVG Template Structure

The SVG is rendered via Go `html/template` with these elements:

| SVG element | Purpose |
|------------|---------|
| `<svg>` | Root with computed width/height based on graph dimensions |
| `<rect>` per node | AS node rectangle with rounded corners |
| `<text>` per node | ASN number (bold) + organization name |
| `<line>` per edge | Connection between AS nodes |
| `<polygon>` per edge | Arrow head at destination |
| `<title>` per node | Tooltip with full AS details (hover) |

CSS classes on SVG elements enable hover effects (highlight node, dim others).

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| HTTP GET `/lg/graph?prefix=10.0.0.0/24` | -> | Returns SVG fragment | `test/plugin/looking-glass/graph.ci` |
| Route lookup page "Show topology" link | -> | HTMX loads graph inline | `test/plugin/looking-glass/ui-lookup.ci` (extended) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | GET `/lg/graph?prefix=X` with routes from multiple peers | Returns SVG with all unique ASes as nodes and links as edges |
| AC-2 | Graph nodes | Each node shows ASN number and organization name |
| AC-3 | Graph layout | Nodes arranged left-to-right by hop depth (peers left, origin right) |
| AC-4 | Graph edges | Directed arrows from source to destination |
| AC-5 | Single AS path (linear) | Graph is a simple left-to-right chain |
| AC-6 | Multiple AS paths (branching) | Graph shows divergence where paths differ |
| AC-7 | AS prepending (repeated ASN in path) | Single node, not duplicated |
| AC-8 | Prefix with no routes | Returns empty SVG with "No routes found" text |
| AC-9 | SVG response | Content-Type includes SVG, valid SVG document |
| AC-10 | HTMX integration | Graph container in lookup page, loaded via hx-get |
| AC-11 | Graph node hover | Tooltip shows full AS details |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildGraph` | `internal/component/lg/graph_test.go` | AC-1: graph construction from AS paths | |
| `TestBuildGraphSinglePath` | `internal/component/lg/graph_test.go` | AC-5: linear graph | |
| `TestBuildGraphMultiplePaths` | `internal/component/lg/graph_test.go` | AC-6: branching graph | |
| `TestBuildGraphPrepending` | `internal/component/lg/graph_test.go` | AC-7: AS prepending dedup | |
| `TestBuildGraphEmpty` | `internal/component/lg/graph_test.go` | AC-8: no routes | |
| `TestLayoutLayers` | `internal/component/lg/layout_test.go` | AC-3: layer assignment | |
| `TestLayoutPositions` | `internal/component/lg/layout_test.go` | AC-3: node positioning | |
| `TestRenderSVG` | `internal/component/lg/graph_test.go` | AC-9: valid SVG output | |
| `TestRenderSVGWithNames` | `internal/component/lg/graph_test.go` | AC-2: ASN names in labels | |
| `TestGraphHandler` | `internal/component/lg/handler_graph_test.go` | AC-1, AC-10: handler returns SVG | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Max nodes in graph | 1-100 | 100 | N/A | 101 (capped, show warning) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-lg-graph` | `test/plugin/looking-glass/graph.ci` | GET /lg/graph returns SVG | |

### Future (if deferring any tests)
- Interactive graph (click node to filter routes by AS)
- Graph export as standalone SVG file
- Graph with IXP nodes (intermediate peering points)

## Files to Modify
- `internal/component/lg/server.go` - register graph handler on mux
- `internal/component/lg/templates/page/lookup.html` - add graph container + trigger

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | N/A |
| CLI commands/flags | [ ] | N/A |
| Editor autocomplete | [ ] | N/A |
| Functional test | [x] | `test/plugin/looking-glass/graph.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | N/A (in umbrella) |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/looking-glass.md` - graph section |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create
- `internal/component/lg/graph.go` - graph data model and construction
- `internal/component/lg/graph_test.go` - graph unit tests
- `internal/component/lg/layout.go` - layered layout algorithm
- `internal/component/lg/layout_test.go` - layout unit tests
- `internal/component/lg/handler_graph.go` - graph HTTP handler
- `internal/component/lg/handler_graph_test.go` - graph handler unit tests
- `internal/component/lg/templates/component/graph.svg.html` - SVG template
- `test/plugin/looking-glass/graph.ci` - graph functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Graph data model** -- node/edge types, graph construction from AS paths
   - Tests: `TestBuildGraph`, `TestBuildGraphSinglePath`, `TestBuildGraphMultiplePaths`, `TestBuildGraphPrepending`, `TestBuildGraphEmpty`
   - Files: `graph.go`, `graph_test.go`
   - Verify: tests fail -> implement -> tests pass
2. **Phase: Layout algorithm** -- layer assignment, node positioning
   - Tests: `TestLayoutLayers`, `TestLayoutPositions`
   - Files: `layout.go`, `layout_test.go`
   - Verify: tests fail -> implement -> tests pass
3. **Phase: SVG template** -- SVG rendering with nodes, edges, labels
   - Tests: `TestRenderSVG`, `TestRenderSVGWithNames`
   - Files: `templates/component/graph.svg.html`, `graph.go` (render function)
   - Verify: tests fail -> implement -> tests pass
4. **Phase: Graph handler** -- HTTP handler, wiring to lookup page
   - Tests: `TestGraphHandler`
   - Files: `handler_graph.go`, `handler_graph_test.go`, `server.go`, `templates/page/lookup.html`
   - Verify: tests fail -> implement -> tests pass
5. **Phase: Functional test** -- .ci test for graph endpoint
   - Tests: `test/plugin/looking-glass/graph.ci`
   - Verify: `make ze-functional-test` passes
6. **Full verification** -> `make ze-verify`
7. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Layout produces valid non-overlapping positions for all test cases |
| Naming | Graph types follow Go conventions, SVG template uses semantic class names |
| Data flow | Route data via CommandDispatcher, ASN names via decorator |
| Rule: design-principles | Layout algorithm is minimal (no premature Sugiyama complexity) |
| Rule: file-modularity | Graph, layout, handler in separate files |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Graph model exists | `ls internal/component/lg/graph.go` |
| Layout exists | `ls internal/component/lg/layout.go` |
| SVG template exists | `ls internal/component/lg/templates/component/graph.svg.html` |
| Graph handler exists | `ls internal/component/lg/handler_graph.go` |
| Graph endpoint works | functional test `graph.ci` passes |
| SVG is valid | test parses output as XML |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Prefix parameter validated before use |
| XSS | ASN names HTML-escaped in SVG text elements |
| Resource exhaustion | Graph node count capped (max 100 nodes) |
| SVG injection | No user-controlled content in SVG attributes without escaping |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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

## RFC Documentation

N/A -- not protocol work.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

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

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
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
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
