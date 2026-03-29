# Spec: lg-0 -- Looking Glass (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-dns, spec-decorator |
| Phase | - |
| Updated | 2026-03-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/web/server.go` - existing component HTTP server pattern
4. `internal/component/web/handler.go` - existing URL routing pattern
5. `cmd/ze/hub/main.go` - hub startup integration
6. Child specs: `spec-lg-1-core.md` through `spec-lg-4-graph.md`

## Task

Add a looking glass to Ze that exposes BGP session state and route information via both
an HTMX web UI and a birdwatcher-compatible REST API. The looking glass is a component
(`internal/component/lg/`), not a plugin, following the same pattern as the web UI and SSH
components. It serves two audiences: human operators browsing peers and routes through a
browser, and external tools (Alice-LG, monitoring scripts) consuming JSON.

**Supersedes:** `plan/spec-looking-glass.md` (skeleton, plugin-based design). The looking glass
is now a component, not a plugin.

### Vision

The looking glass provides a public, read-only view of Ze's BGP state:

- **Peer dashboard** showing all peers with live state, ASN with organization name, route counts
- **Route lookup** by prefix (exact and IP containment), AS path regex, community filter
- **Per-peer route views** showing received and best routes with full attributes
- **AS path topology graph** rendered as server-side SVG showing the network path to a prefix
- **Birdwatcher REST API** for Alice-LG and external tool integration
- **ASN name resolution** via YANG decorator framework and Team Cymru DNS

### Design Decisions (agreed with user)

#### D-1: Component, Not Plugin

| Aspect | Decision |
|--------|----------|
| Location | `internal/component/lg/` |
| Startup | Hub `main.go` (same as web/ssh) |
| Rationale | LG is infrastructure, not BGP behavior extension. The "delete the folder" test: deleting `bgp/plugins/` should not remove the looking glass |

#### D-2: Access Model

| Aspect | Decision |
|--------|----------|
| Auth | None (public, read-only) |
| Binding | No restrictions on listen address (open to the world) |
| CLI overrides | None (settings via env config only, like web) |
| Rationale | Looking glasses at IXPs are public services. No login required |

#### D-3: Separate HTTP Server

| Aspect | Decision |
|--------|----------|
| Port | Own port (default 8444), separate from web UI (8443) |
| TLS | Optional (LG often behind reverse proxy) |
| Rationale | Different access model (public vs authenticated). Operators may expose LG port to internet while keeping web UI port firewalled |

#### D-4: Data Access

| Aspect | Decision |
|--------|----------|
| Mechanism | `CommandDispatcher` (same as web UI admin commands) |
| Rationale | Follows existing pattern, no direct RIB access, plugin isolation preserved |

#### D-5: Visualization

| Aspect | Decision |
|--------|----------|
| Technology | Server-side SVG via Go `html/template` |
| Layout | Layered layout algorithm in Go for AS path DAGs |
| Rationale | Zero dependencies (no GraphViz binary, no WASM, no JS graph library). Perfect HTMX fit. Sufficient for AS path graphs (typically 3-10 nodes) |

#### D-6: ASN Name Resolution

| Aspect | Decision |
|--------|----------|
| Mechanism | YANG `ze:decorate "asn-name"` extension (spec-decorator) |
| Backend | Team Cymru DNS via DNS resolver component (spec-dns) |
| Scope | All ASN displays: peer tables, route details, graph nodes |
| Rationale | General-purpose decorator framework, not LG-specific. Reusable across web UI |

#### D-7: Birdwatcher API Compatibility

| Aspect | Decision |
|--------|----------|
| Field names | Birdwatcher convention (snake_case) in API JSON |
| Rationale | De facto standard for looking glass APIs. Alice-LG compatibility |

#### D-8: URL Structure

| Path prefix | Handler | Format |
|-------------|---------|--------|
| `/lg/` | HTMX web UI pages | HTML fragments |
| `/lg/graph/` | SVG graph responses | SVG in HTML |
| `/api/looking-glass/` | Birdwatcher REST API | JSON |
| `/lg/assets/` | Static assets (CSS) | Static files |

#### D-9: YANG Configuration

| Path | Type | Default | Description |
|------|------|---------|-------------|
| `environment/looking-glass/host` | string | `0.0.0.0` | Listen address |
| `environment/looking-glass/port` | uint16 | `8444` | Listen port |
| `environment/looking-glass/tls` | boolean | `false` | Enable TLS |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component architecture, startup lifecycle
  -> Decision:
  -> Constraint:
- [ ] `.claude/rules/plugin-design.md` - proximity principle, component vs plugin
  -> Decision:
  -> Constraint:
- [ ] `docs/architecture/api/architecture.md` - RPC dispatch, command structure
  -> Decision:
  -> Constraint:

### RFC Summaries (MUST for protocol work)
- N/A (operational tooling, not protocol work)

**Key insights:**
- To be filled during implementation of child specs

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/server.go` - HTTP server pattern (TLS, bind, lifecycle)
- [ ] `internal/component/web/handler.go` - URL routing, ParseURL, content negotiation
- [ ] `internal/component/web/render.go` - template rendering, fieldFor dispatch
- [ ] `internal/component/web/sse.go` - SSE EventBroker pattern
- [ ] `cmd/ze/hub/main.go` - startWebServer(), component startup from hub
- [ ] `internal/component/web/schema/ze-web-conf.yang` - env config YANG pattern

**Behavior to preserve:**
- Existing web UI unchanged (LG is a separate server on a different port)
- Existing RPC commands and their output format
- Plugin isolation (LG queries via CommandDispatcher, no direct RIB access)
- YANG schema registration pattern

**Behavior to change:**
- No looking glass currently exists
- Hub gains a second HTTP server startup (LG alongside web)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- HTTP request to LG server (browser or API client)
- Request specifies: peer lookup, route query, prefix search, or status

### Transformation Path
1. HTTP handler parses request (URL path, query parameters)
2. Handler maps request to RPC command (e.g., `rib show peer 10.0.0.1`)
3. Handler calls `CommandDispatcher` with RPC command string
4. Engine processes RPC, returns JSON result
5. For API: handler transforms Ze JSON to birdwatcher field names, writes JSON response
6. For UI: handler passes data to Go template, renders HTML fragment
7. For graph: handler builds graph data from AS paths, renders SVG via template

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| HTTP client -> LG server | HTTP/HTTPS on configured port | [ ] |
| LG handler -> Engine | `CommandDispatcher` (Go function call) | [ ] |
| Engine -> RIB/Peer plugins | Internal RPC dispatch | [ ] |
| LG handler -> Decorator | Go function call for ASN annotation | [ ] |
| Decorator -> DNS resolver | Go function call (ResolveTXT) | [ ] |

### Integration Points
- Hub startup: creates LG server, passes CommandDispatcher
- CommandDispatcher: same mechanism web UI uses for admin commands
- YANG schema: `ze-lg-conf.yang` under `environment/looking-glass`
- Decorator framework: YANG `ze:decorate` for ASN name resolution
- DNS resolver: Team Cymru queries for ASN names
- SSE: EventBroker pattern for live peer state (reuse pattern from web)

### Architectural Verification
- [ ] No bypassed layers (LG queries engine via CommandDispatcher only)
- [ ] No unintended coupling (LG component isolated in its own package)
- [ ] No duplicated functionality (reuses existing RPC commands, decorator, DNS)
- [ ] Zero-copy preserved where applicable (N/A for HTTP responses)

## Child Specs

| Spec | Scope | Depends on |
|------|-------|-----------|
| `spec-lg-1-core.md` | Component skeleton, YANG, HTTP server, routing | Nothing |
| `spec-lg-2-api.md` | Birdwatcher-compatible REST API (JSON) | lg-1-core |
| `spec-lg-3-ui.md` | HTMX looking glass pages | lg-1-core, spec-decorator |
| `spec-lg-4-graph.md` | AS path topology visualization (server SVG) | lg-3-ui, spec-decorator |

### Prerequisites (separate specs, not children)

| Spec | What it provides | Must be done before |
|------|-----------------|---------------------|
| `spec-dns.md` | DNS resolver component (`internal/component/dns/`) | lg-3-ui, lg-4-graph |
| `spec-decorator.md` | YANG `ze:decorate` extension framework | lg-3-ui, lg-4-graph |

### Execution Order

Phase 1 (parallel): `spec-dns` + `spec-lg-1-core`
Phase 2 (parallel): `spec-decorator` + `spec-lg-2-api`
Phase 3: `spec-lg-3-ui`
Phase 4: `spec-lg-4-graph`

## Features by Child Spec

### lg-1-core
- Component directory structure at `internal/component/lg/`
- YANG schema (`ze-lg-conf.yang`) under `environment/looking-glass`
- HTTP server lifecycle (bind, optional TLS, graceful shutdown)
- Handler routing (mux setup for UI, API, assets paths)
- Hub startup integration in `cmd/ze/hub/main.go`
- CommandDispatcher wiring

### lg-2-api
- `/api/looking-glass/status` -- router status (router ID, version, uptime)
- `/api/looking-glass/protocols/bgp` -- peer list with state, route counts
- `/api/looking-glass/routes/protocol/{name}` -- routes from named peer
- `/api/looking-glass/routes/table/{family}` -- best routes by family
- `/api/looking-glass/routes/filtered/{name}` -- filtered routes per peer
- `/api/looking-glass/routes/search?prefix=X` -- prefix lookup
- Ze JSON to birdwatcher field name transform layer
- Alice-LG integration compatibility

### lg-3-ui
- Peer dashboard with live SSE state updates
- Prefix lookup (exact + IP containment)
- AS path regex search
- Community filter
- Per-peer route views (received, best)
- Route detail expansion (click row, HTMX loads full attributes)
- Multi-family tabs (IPv4, IPv6, etc.)
- All ASN numbers decorated with organization names
- Templates following existing web UI patterns

### lg-4-graph
- Graph data model (nodes = ASes with names, edges = peering)
- Layered layout algorithm in Go (Sugiyama-style for small DAGs)
- SVG rendering via Go `html/template`
- Embedded in prefix lookup results
- HTMX integration (graph loads as fragment)
- Node labels show ASN + organization name

## Research Summary

Based on survey of all major BGP looking glasses (Alice-LG, Hyperglass, bird-lg-go, NLNOG,
GIXLG, Cougar LG, Peering Manager LG, and others):

| Gap in ecosystem | Ze fills it |
|-----------------|-------------|
| No daemon-native LG | Ze IS the daemon, direct data access |
| No HTMX-based LG | Server-rendered, no SPA framework |
| No real-time streaming | SSE for peer state changes |
| No built-in visualization | Server-side SVG, no external binary |
| No automatic ASN name resolution | YANG decorator + Team Cymru DNS |

## Non-Goals

| Feature | Why excluded |
|---------|-------------|
| Ping/traceroute from router | Security risk, not BGP looking glass scope |
| Historical route data | Requires persistent storage, separate future spec |
| Multi-router aggregation | Ze is one daemon; Alice-LG handles multi-RS |
| RFC 8522 compliance | Nobody implements it; birdwatcher is the de facto standard |
| Client-side JS graph library | Server SVG sufficient for AS path DAGs |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG config with looking-glass section | -> | LG server starts on configured port | `test/parse/lg-config.ci` |
| HTTP GET `/api/looking-glass/status` | -> | Returns router status JSON | `test/plugin/looking-glass/status.ci` |
| HTTP GET `/lg/peers` | -> | Returns HTMX peer dashboard | `test/plugin/looking-glass/peers.ci` |

## Acceptance Criteria

Acceptance criteria are defined in each child spec. The umbrella tracks overall integration:

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | YANG config with `environment/looking-glass` section | LG HTTP server starts on configured host:port |
| AC-2 | LG disabled (no config section) | No HTTP server started, no resource usage |
| AC-3 | API endpoint returns JSON | Content-Type: application/json, birdwatcher field names |
| AC-4 | UI endpoint returns HTML | Content-Type: text/html, HTMX fragments |
| AC-5 | ASN displayed in any LG page | Organization name annotation shown via decorator |
| AC-6 | Prefix lookup with graph | SVG topology graph rendered inline |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Tests defined in child specs | See lg-1 through lg-4 | Per-child-spec | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| LG port | 1-65535 | 65535 | 0 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-lg-config` | `test/parse/lg-config.ci` | Config with LG section parses | |
| Additional tests in child specs | See lg-1 through lg-4 | Per-child-spec | |

### Future (if deferring any tests)
- Alice-LG end-to-end integration test (requires Alice-LG container)
- Performance test with large RIB (100k+ routes)
- Historical route data (separate future spec)

## Files to Modify
- `cmd/ze/hub/main.go` - add LG server startup (parallel to web server startup)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [x] | `internal/component/lg/schema/ze-lg-conf.yang` |
| CLI commands/flags | [ ] | N/A |
| Editor autocomplete | [x] | YANG-driven (automatic) |
| Functional test | [x] | `test/parse/lg-config.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - looking glass |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` - looking-glass env section |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` - birdwatcher API |
| 5 | Plugin added/changed? | [ ] | N/A (component, not plugin) |
| 6 | Has a user guide page? | [x] | `docs/guide/looking-glass.md` - LG user guide |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` - built-in looking glass |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` - LG component |

## Files to Create
- Files defined in child specs. See lg-1 through lg-4.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + all child specs |
| 2. Audit | Child spec Files to Modify/Create |
| 3. Implement (TDD) | Execute child specs in order |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Per child spec |
| 6. Fix issues | Per child spec |
| 7. Re-verify | `make ze-verify` |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | `make ze-verify` |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each child spec is an implementation phase. See child specs for detailed phases.

1. **Phase: Prerequisites** -- implement spec-dns and spec-decorator
   - Parallel with lg-1-core
2. **Phase: lg-1-core** -- component skeleton, YANG, HTTP server
   - See `spec-lg-1-core.md`
3. **Phase: lg-2-api** -- birdwatcher REST API
   - See `spec-lg-2-api.md`
4. **Phase: lg-3-ui** -- HTMX web pages
   - See `spec-lg-3-ui.md`
5. **Phase: lg-4-graph** -- AS path visualization
   - See `spec-lg-4-graph.md`
6. **Full verification** -> `make ze-verify`
7. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All child specs implemented, all ACs demonstrated |
| Correctness | API returns valid birdwatcher JSON; UI renders correctly |
| Naming | YANG kebab-case, API snake_case (birdwatcher), HTML templates consistent |
| Data flow | All data via CommandDispatcher, no direct RIB access |
| Rule: design-principles | Component isolated, no coupling to web UI internals |
| Rule: plugin-design | Component in `internal/component/lg/`, not `plugins/` |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| LG component exists | `ls internal/component/lg/` |
| YANG schema exists | `ls internal/component/lg/schema/ze-lg-conf.yang` |
| API returns JSON | functional test status.ci passes |
| UI renders HTML | functional test peers.ci passes |
| Graph renders SVG | functional test with prefix lookup |
| Config parses | functional test lg-config.ci passes |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Peer names, prefixes, AS path patterns sanitized |
| No auth bypass | LG server cannot reach config/admin endpoints |
| Resource exhaustion | Pagination or size limits on route queries |
| Error leakage | Errors do not expose internal paths or stack traces |
| DoS | Rate limiting or response size bounds for public endpoint |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the child spec that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant child spec and implement |
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

N/A -- operational tooling, not protocol work.

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
