# Spec: lg-3 -- Looking Glass HTMX UI

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-lg-1-core, spec-decorator |
| Phase | - |
| Updated | 2026-03-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-lg-0-umbrella.md` - umbrella spec with design decisions
4. `internal/component/lg/server.go` - LG server (from lg-1-core)
5. `internal/component/web/render.go` - existing template rendering pattern
6. `internal/component/web/templates/` - existing HTMX template patterns
7. `internal/component/web/sse.go` - SSE EventBroker pattern
8. `internal/component/web/decorator.go` - decorator framework (from spec-decorator)

## Task

Implement the HTMX web UI for the looking glass. The UI provides server-rendered HTML pages
for human operators to browse BGP peers, look up routes, search by AS path or community,
and view per-peer route details. All pages use HTMX for partial updates without full page
reloads.

The UI follows the same template and rendering patterns as the existing web UI but serves
read-only, public content on the LG server port. ASN numbers are automatically enriched
with organization names via the decorator framework.

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
- [ ] `internal/component/web/render.go` - Renderer, template dispatch
- [ ] `internal/component/web/fragment.go` - HTMX fragment rendering, OOB swaps
- [ ] `internal/component/web/sse.go` - EventBroker, SSE client management
- [ ] `internal/component/web/templates/page/layout.html` - page shell template
- [ ] `internal/component/web/templates/component/` - component templates
- [ ] `internal/component/web/assets/htmx.min.js` - HTMX library (embedded)
- [ ] `internal/component/web/assets/sse.js` - SSE extension
- [ ] `internal/component/lg/server.go` - LG server mux (from lg-1-core)
- [ ] `internal/component/web/decorator.go` - decorator framework (from spec-decorator)

**Behavior to preserve:**
- Existing web UI unchanged
- LG server routing from lg-1-core
- Decorator framework from spec-decorator
- HTMX patterns (OOB swaps, fragment rendering)

**Behavior to change:**
- LG server gains UI handler registrations under `/lg/`
- New templates for LG-specific pages (peer dashboard, route table, search)
- SSE endpoint for live peer state on LG server

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Browser navigates to LG server (e.g., `https://lg.example.com:8444/lg/`)
- HTMX sends partial requests for fragments

### Transformation Path
1. Browser requests full page or HTMX fragment
2. Handler determines if full page or fragment (HX-Request header)
3. Handler calls CommandDispatcher for BGP data
4. Engine returns JSON response
5. Handler parses JSON, passes data to Go template
6. Template renders HTML with HTMX attributes
7. For decorated fields: decorator enriches ASN values with names
8. Response returned as HTML fragment (HTMX) or full page (initial load)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Browser -> LG server | HTTP/HTTPS | [ ] |
| Handler -> Engine | CommandDispatcher | [ ] |
| Handler -> Decorator | Go function call for ASN annotation | [ ] |
| Handler -> SSE broker | Event subscription for live state | [ ] |

### Integration Points
- LG server mux from lg-1-core
- CommandDispatcher from hub
- Decorator framework from spec-decorator
- SSE EventBroker (new instance for LG, same pattern as web UI)
- HTMX library (embedded asset, same as web UI)
- Go `html/template` for rendering

### Architectural Verification
- [ ] No bypassed layers (queries via CommandDispatcher only)
- [ ] No unintended coupling (LG templates separate from web UI templates)
- [ ] No duplicated functionality (reuses HTMX/SSE patterns, own templates)
- [ ] Zero-copy preserved where applicable (N/A)

## UI Pages

### Page: Peer Dashboard (`/lg/peers`)

The main landing page showing all BGP peers in a table.

| Column | Source | Decorator |
|--------|--------|-----------|
| Peer Address | `peer-address` | None |
| Remote AS | `remote-as` | `asn-name` (shows org name) |
| State | `state` | None (color-coded: green=established, red=down) |
| Uptime | computed from `established-time` | None |
| Routes Received | `routes-received` | None |
| Routes Accepted | `routes-accepted` | None |
| Routes Sent | `routes-sent` | None |
| Description | `description` | None |

- SSE live updates: peer state changes push new row HTML via SSE
- Click peer row: navigates to per-peer route view
- Multi-family: tabs for IPv4, IPv6, all

### Page: Route Lookup (`/lg/lookup`)

Search form + results table.

| Input | Type | Description |
|-------|------|-------------|
| Prefix/IP | text | IP address or CIDR prefix |
| Family | select | IPv4, IPv6, or all |

Results table:

| Column | Source | Decorator |
|--------|--------|-----------|
| Prefix | `prefix` | None |
| Next Hop | `next-hop` | None |
| AS Path | `as-path` | `asn-name` (each AS annotated) |
| Origin | `origin` | None |
| Local Pref | `local-preference` | None |
| MED | `med` | None |
| Communities | `community` | None |
| Peer | `peer-address` | None |
| Peer AS | `peer-as` | `asn-name` |

- HTMX: form submits via `hx-post`, results swap into table div
- Click route row: expands full attribute detail in-place via `hx-get`
- Graph link: "Show topology" link loads AS path graph (lg-4-graph)

### Page: AS Path Search (`/lg/search/aspath`)

| Input | Type | Description |
|-------|------|-------------|
| AS Path Pattern | text | Regex or space-separated AS numbers |
| Family | select | IPv4, IPv6, or all |

Same results table as route lookup. Limited to 1000 results.

### Page: Community Search (`/lg/search/community`)

| Input | Type | Description |
|-------|------|-------------|
| Community | text | Standard (N:N), large (N:N:N), or extended |
| Family | select | IPv4, IPv6, or all |

Same results table as route lookup.

### Page: Per-Peer Routes (`/lg/peer/{address}`)

Routes received from a specific peer. Same table as route lookup but filtered to one peer.
Header shows peer details (address, AS, state, uptime, description).

### Page: Route Detail (fragment, loaded via HTMX)

Expanded view of a single route's full attributes. Loaded inline when clicking a route row.

| Section | Content |
|---------|---------|
| Prefix | Network prefix |
| Next Hop | Next hop address |
| Origin | IGP/EGP/Incomplete |
| AS Path | Full path with AS names |
| Local Pref | Local preference value |
| MED | Multi-exit discriminator |
| Communities | Standard communities list |
| Large Communities | Large communities list |
| Extended Communities | Extended communities list |
| Originator ID | If present |
| Cluster List | If present |
| Peer | Source peer address and AS |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| HTTP GET `/lg/peers` | -> | Returns peer dashboard HTML | `test/plugin/looking-glass/ui-peers.ci` |
| HTTP POST `/lg/lookup` with prefix | -> | Returns route results HTML | `test/plugin/looking-glass/ui-lookup.ci` |
| HTMX request (HX-Request header) | -> | Returns fragment, not full page | `test/plugin/looking-glass/ui-fragment.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | GET `/lg/peers` | Returns HTML page with peer table |
| AC-2 | GET `/lg/peers` with established peers | Each peer row shows AS number with organization name |
| AC-3 | POST `/lg/lookup` with valid prefix | Returns HTML with matching routes |
| AC-4 | POST `/lg/lookup` with IP address (not CIDR) | Returns routes covering that IP |
| AC-5 | POST `/lg/search/aspath` with AS number | Returns routes with matching AS path |
| AC-6 | POST `/lg/search/community` with community | Returns routes with matching community |
| AC-7 | GET `/lg/peer/{address}` with valid peer | Returns routes from that peer |
| AC-8 | GET `/lg/peer/{address}` with invalid peer | Returns 404 page |
| AC-9 | HTMX request (HX-Request header present) | Returns HTML fragment, not full page |
| AC-10 | Non-HTMX request (no HX-Request header) | Returns full page with layout |
| AC-11 | Peer state changes while dashboard open | SSE pushes updated row HTML |
| AC-12 | Click route row in results | HTMX loads full attribute detail inline |
| AC-13 | Route results with AS path | Each AS number in path annotated with name |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerDashboardData` | `internal/component/lg/handler_ui_test.go` | AC-1: peer data preparation | |
| `TestPeerDashboardASNDecoration` | `internal/component/lg/handler_ui_test.go` | AC-2: ASN names in output | |
| `TestLookupHandler` | `internal/component/lg/handler_ui_test.go` | AC-3: route lookup | |
| `TestLookupIPContainment` | `internal/component/lg/handler_ui_test.go` | AC-4: IP vs CIDR handling | |
| `TestASPathSearch` | `internal/component/lg/handler_ui_test.go` | AC-5: AS path search | |
| `TestCommunitySearch` | `internal/component/lg/handler_ui_test.go` | AC-6: community search | |
| `TestPerPeerRoutes` | `internal/component/lg/handler_ui_test.go` | AC-7: per-peer routes | |
| `TestPerPeerNotFound` | `internal/component/lg/handler_ui_test.go` | AC-8: invalid peer 404 | |
| `TestFragmentVsFullPage` | `internal/component/lg/handler_ui_test.go` | AC-9, AC-10: HX-Request detection | |
| `TestRouteDetailFragment` | `internal/component/lg/handler_ui_test.go` | AC-12: detail expansion | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Search result limit | 1-1000 | 1000 | N/A | 1001 (capped) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-lg-ui-peers` | `test/plugin/looking-glass/ui-peers.ci` | GET /lg/peers shows peer table | |
| `test-lg-ui-lookup` | `test/plugin/looking-glass/ui-lookup.ci` | POST /lg/lookup returns routes | |
| `test-lg-ui-fragment` | `test/plugin/looking-glass/ui-fragment.ci` | HTMX request returns fragment | |

### Future (if deferring any tests)
- SSE live update test (requires async test infrastructure)
- Multi-family tab switching test
- Large result set pagination

## Files to Modify
- `internal/component/lg/server.go` - register UI handlers on mux

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | N/A (UI has no extra config beyond lg-1-core) |
| CLI commands/flags | [ ] | N/A |
| Editor autocomplete | [ ] | N/A |
| Functional test | [x] | `test/plugin/looking-glass/ui-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | N/A (in umbrella) |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/looking-glass.md` - UI section |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create
- `internal/component/lg/handler_ui.go` - UI HTTP handlers
- `internal/component/lg/handler_ui_test.go` - UI handler unit tests
- `internal/component/lg/render.go` - LG template renderer
- `internal/component/lg/sse.go` - SSE broker for peer state events
- `internal/component/lg/templates/page/layout.html` - LG page shell
- `internal/component/lg/templates/page/peers.html` - peer dashboard
- `internal/component/lg/templates/page/lookup.html` - route lookup form + results
- `internal/component/lg/templates/page/search.html` - AS path / community search
- `internal/component/lg/templates/page/peer_routes.html` - per-peer route view
- `internal/component/lg/templates/component/route_table.html` - route results table
- `internal/component/lg/templates/component/route_detail.html` - expanded route detail
- `internal/component/lg/templates/component/peer_row.html` - single peer row (SSE update)
- `internal/component/lg/assets/style.css` - LG-specific styles
- `test/plugin/looking-glass/ui-peers.ci` - peer dashboard functional test
- `test/plugin/looking-glass/ui-lookup.ci` - route lookup functional test
- `test/plugin/looking-glass/ui-fragment.ci` - HTMX fragment functional test

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

1. **Phase: Templates and renderer** -- page layout, peer dashboard, route table templates
   - Tests: `TestFragmentVsFullPage`
   - Files: `render.go`, all template files
   - Verify: templates parse without error
2. **Phase: Peer dashboard** -- handler, data fetching, ASN decoration
   - Tests: `TestPeerDashboardData`, `TestPeerDashboardASNDecoration`
   - Files: `handler_ui.go`, `templates/page/peers.html`, `templates/component/peer_row.html`
   - Verify: tests fail -> implement -> tests pass
3. **Phase: Route lookup** -- lookup handler, IP vs CIDR, results table
   - Tests: `TestLookupHandler`, `TestLookupIPContainment`
   - Files: `handler_ui.go`, `templates/page/lookup.html`, `templates/component/route_table.html`
   - Verify: tests fail -> implement -> tests pass
4. **Phase: Search handlers** -- AS path and community search
   - Tests: `TestASPathSearch`, `TestCommunitySearch`
   - Files: `handler_ui.go`, `templates/page/search.html`
   - Verify: tests fail -> implement -> tests pass
5. **Phase: Per-peer routes and detail** -- per-peer view, route detail expansion
   - Tests: `TestPerPeerRoutes`, `TestPerPeerNotFound`, `TestRouteDetailFragment`
   - Files: `handler_ui.go`, `templates/page/peer_routes.html`, `templates/component/route_detail.html`
   - Verify: tests fail -> implement -> tests pass
6. **Phase: SSE for live peer state** -- EventBroker, peer state push
   - Files: `sse.go`
   - Verify: SSE endpoint returns event stream
7. **Phase: Functional tests** -- .ci tests for end-to-end UI
   - Tests: `test/plugin/looking-glass/ui-*.ci`
   - Verify: `make ze-functional-test` passes
8. **Full verification** -> `make ze-verify`
9. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | ASN decoration shows on all ASN fields, not just some |
| Naming | Template files follow existing web UI naming pattern |
| Data flow | All data via CommandDispatcher, decorator for ASN, no direct imports |
| Rule: design-principles | Templates are minimal, single-concern per template |
| Rule: file-modularity | Handler file not over 600 lines; split by page if needed |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Templates exist | `ls internal/component/lg/templates/` |
| Peer dashboard works | functional test `ui-peers.ci` passes |
| Route lookup works | functional test `ui-lookup.ci` passes |
| HTMX fragments work | functional test `ui-fragment.ci` passes |
| Styles exist | `ls internal/component/lg/assets/style.css` |
| SSE endpoint exists | grep for SSE handler in `server.go` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| XSS | All user-provided search inputs HTML-escaped in templates |
| Input validation | Prefix, AS path, community inputs validated before use |
| Resource exhaustion | Search results capped at 1000 |
| Error leakage | Template error pages do not expose internal paths |
| CSRF | Not needed (read-only, no state mutation) |

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
