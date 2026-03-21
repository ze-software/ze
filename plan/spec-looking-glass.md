# Spec: Looking Glass REST API Plugin

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-03-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/plugin-design.md` - plugin architecture and registration
4. `internal/core/metrics/server.go` - existing HTTP server pattern
5. `internal/chaos/web/handlers.go` - existing web dashboard pattern (SSE, mux)
6. `internal/component/bgp/plugins/rib/schema/ze-rib-api.yang` - RIB query RPCs

## Task

Implement a Looking Glass REST API as a Ze plugin. The API exposes BGP session state and
route information over HTTP/JSON, compatible with the **birdwatcher** field naming convention
used by existing looking glass frontends (Alice-LG, etc.).

IXP operators deploy looking glass services so members can inspect route propagation. The
standard integration path is: BGP daemon -> birdwatcher-compatible REST API -> Alice-LG
frontend. Ze currently has no HTTP API for external route queries.

The plugin:
- Registers as an internal plugin (`ze.looking-glass`)
- Binds an HTTP server on a configurable address/port
- Queries the engine via RIB and peer RPCs (using `sdk.DispatchCommand`)
- Returns JSON responses with birdwatcher-compatible field names
- Supports route filtering by prefix, peer, and family

### Inspiration

rustbgpd exposes a birdwatcher-compatible REST API (`/status`, `/protocols/bgp`,
`/routes/protocol/{id}`) for Alice-LG integration. This is standard IXP infrastructure.
Ze has the RIB query RPCs and HTTP server patterns to support this natively as a plugin.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - API command structure, RPC dispatch
  -> Decision:
  -> Constraint:
- [ ] `docs/architecture/core-design.md` - plugin architecture, event dispatch
  -> Decision:
  -> Constraint:
- [ ] `.claude/rules/plugin-design.md` - plugin registration, YANG requirements, proximity
  -> Decision:
  -> Constraint:

### RFC Summaries (MUST for protocol work)
- N/A (not protocol work -- operational tooling)

### Reference: birdwatcher API
- Alice-LG expects specific JSON field names and endpoint structure
- Key endpoints: `/status`, `/protocols/bgp`, `/routes/protocol/{name}`,
  `/routes/filtered/{name}`, `/routes/table/{name}`
- Field names: `router_id`, `server_time`, `status`, `protocols`, `routes`, `network`, `gateway`

**Key insights:**
- To be filled during design phase

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/metrics/server.go` - HTTP server with configurable addr/port/path, ServeMux
- [ ] `internal/chaos/web/handlers.go` - web dashboard: SSE, REST endpoints, shared mux pattern
- [ ] `internal/component/bgp/plugins/rib/schema/ze-rib-api.yang` - RIB RPCs: show, best, status, count
- [ ] `internal/component/bgp/plugins/rib/rib.go` - RIB plugin entry point, command handlers
- [ ] `internal/component/bgp/plugins/bgp/schema/ze-bgp-api.yang` - peer RPCs: peer show, peer list
- [ ] `internal/plugin/registry/registry.go` - plugin registration interface
- [ ] `pkg/plugin/sdk/` - SDK for plugin-to-engine communication

**Behavior to preserve:**
- Existing metrics HTTP endpoint (separate from looking glass)
- Existing RIB query RPC interface and output format
- Plugin isolation (looking glass queries engine via RPC, not direct access)

**Behavior to change:**
- No HTTP API for external BGP queries currently exists

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- HTTP GET request to looking glass endpoint (e.g., `GET /protocols/bgp`)
- Looking glass plugin receives HTTP request

### Transformation Path
1. HTTP handler parses request path and query parameters
2. Handler maps birdwatcher endpoint to Ze RPC command
3. Handler calls `sdk.DispatchCommand("rib show ...")` or `sdk.DispatchCommand("bgp peer ...")`
4. Engine processes RPC, returns JSON result
5. Handler transforms Ze JSON to birdwatcher-compatible field names
6. Handler writes HTTP JSON response

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| HTTP client -> plugin | HTTP/JSON on configured port | [ ] |
| Plugin -> engine | `sdk.DispatchCommand` over plugin socket | [ ] |
| Engine -> RIB plugin | Internal RPC dispatch | [ ] |

### Integration Points
- `registry.Register()` - plugin registration in `init()`
- `sdk.DispatchCommand()` - querying engine for peer/RIB data
- `net/http.ServeMux` - HTTP handler registration (same pattern as metrics server)
- YANG schema - config for listen address, port, access control

### Architectural Verification
- [ ] No bypassed layers (plugin queries engine via RPC, not direct RIB access)
- [ ] No unintended coupling (plugin in own package, blank import only)
- [ ] No duplicated functionality (reuses existing RIB/peer query RPCs)
- [ ] Zero-copy preserved where applicable (JSON transform only at HTTP boundary)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with looking-glass plugin enabled | -> | Plugin registers and starts HTTP | TBD |
| HTTP GET `/status` | -> | Returns daemon status JSON | TBD |
| HTTP GET `/protocols/bgp` | -> | Returns peer list JSON | TBD |
| HTTP GET `/routes/protocol/{name}` | -> | Returns routes for peer | TBD |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin enabled in config | HTTP server binds on configured address:port |
| AC-2 | GET `/status` | Returns JSON with `router_id`, `server_time`, `version`, `status` fields |
| AC-3 | GET `/protocols/bgp` | Returns JSON with peer list, each having `state`, `neighbor_address`, `neighbor_as`, `routes_received`, `routes_imported` |
| AC-4 | GET `/routes/protocol/{name}` | Returns JSON with routes received from named peer, each with `network`, `gateway`, `metric`, `bgp.as_path`, `bgp.local_pref` |
| AC-5 | GET `/routes/table/{name}` | Returns JSON with best routes (Loc-RIB) filtered by family |
| AC-6 | Plugin disabled or not configured | No HTTP server started, no resource usage |
| AC-7 | Invalid peer name in URL | Returns HTTP 404 with JSON error |
| AC-8 | Engine unreachable (daemon shutting down) | Returns HTTP 503 with JSON error |
| AC-9 | Config specifies custom address and port | HTTP server binds on specified address:port |
| AC-10 | Response Content-Type | All responses have `Content-Type: application/json` |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStatusEndpoint` | `internal/component/bgp/plugins/looking-glass/handler_test.go` | AC-2: status response format | |
| `TestProtocolsEndpoint` | `internal/component/bgp/plugins/looking-glass/handler_test.go` | AC-3: peer list format | |
| `TestRoutesEndpoint` | `internal/component/bgp/plugins/looking-glass/handler_test.go` | AC-4: routes response format | |
| `TestRoutesTableEndpoint` | `internal/component/bgp/plugins/looking-glass/handler_test.go` | AC-5: best routes response | |
| `TestUnknownPeer404` | `internal/component/bgp/plugins/looking-glass/handler_test.go` | AC-7: 404 on bad peer | |
| `TestTransformPeerToProtocol` | `internal/component/bgp/plugins/looking-glass/transform_test.go` | Ze JSON -> birdwatcher field mapping | |
| `TestTransformRouteToEntry` | `internal/component/bgp/plugins/looking-glass/transform_test.go` | Ze route JSON -> birdwatcher route | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| HTTP port | 1-65535 | 65535 | 0 | 65536 |
| Page size (if pagination added) | 1-10000 | 10000 | 0 | 10001 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-lg-status` | `test/plugin/looking-glass/status.ci` | Config with LG enabled -> GET /status returns JSON | |
| `test-lg-protocols` | `test/plugin/looking-glass/protocols.ci` | Config with peer + LG -> GET /protocols/bgp lists peer | |
| `test-lg-routes` | `test/plugin/looking-glass/routes.ci` | Config with peer + route -> GET /routes/protocol/{peer} returns route | |
| `test-lg-disabled` | `test/plugin/looking-glass/disabled.ci` | Config without LG -> no HTTP listener | |

### Future (if deferring any tests)
- Alice-LG end-to-end integration test (requires Alice-LG container)
- Performance test with large RIB (100k+ routes)
- TLS on looking glass HTTP endpoint

## Files to Modify
- `internal/plugin/all/all.go` - blank import for new plugin (auto-generated by `make generate`)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/component/bgp/plugins/looking-glass/schema/ze-looking-glass.yang` |
| RPC count in architecture docs | [x] | `docs/architecture/api/architecture.md` |
| CLI commands/flags | [ ] | N/A (HTTP only, no CLI surface) |
| CLI usage/help text | [ ] | N/A |
| API commands doc | [x] | `docs/architecture/api/commands.md` |
| Plugin SDK docs | [ ] | N/A |
| Editor autocomplete | [ ] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | `test/plugin/looking-glass/*.ci` |

## Files to Create
- `internal/component/bgp/plugins/looking-glass/looking_glass.go` - plugin entry point, HTTP server lifecycle
- `internal/component/bgp/plugins/looking-glass/handler.go` - HTTP handlers for each endpoint
- `internal/component/bgp/plugins/looking-glass/transform.go` - Ze JSON -> birdwatcher field name mapping
- `internal/component/bgp/plugins/looking-glass/register.go` - `init()` -> `registry.Register()`
- `internal/component/bgp/plugins/looking-glass/schema/ze-looking-glass.yang` - config schema (address, port)
- `internal/component/bgp/plugins/looking-glass/handler_test.go` - unit tests
- `internal/component/bgp/plugins/looking-glass/transform_test.go` - transform tests
- `test/plugin/looking-glass/status.ci` - functional test: status endpoint
- `test/plugin/looking-glass/protocols.ci` - functional test: protocols endpoint
- `test/plugin/looking-glass/routes.ci` - functional test: routes endpoint
- `test/plugin/looking-glass/disabled.ci` - functional test: plugin disabled

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `.claude/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfcNNNN.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |

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

1. **Phase: YANG schema and registration** -- define config schema, register plugin
   - Tests: `TestAllPluginsRegistered` count update
   - Files: `register.go`, `schema/ze-looking-glass.yang`
   - Verify: plugin appears in `make ze-inventory`
2. **Phase: Transform layer** -- Ze JSON -> birdwatcher field name mapping functions
   - Tests: `TestTransformPeerToProtocol`, `TestTransformRouteToEntry`
   - Files: `transform.go`, `transform_test.go`
   - Verify: tests fail -> implement -> tests pass
3. **Phase: HTTP handlers** -- implement /status, /protocols/bgp, /routes endpoints
   - Tests: `TestStatusEndpoint`, `TestProtocolsEndpoint`, `TestRoutesEndpoint`, `TestRoutesTableEndpoint`, `TestUnknownPeer404`
   - Files: `handler.go`, `handler_test.go`
   - Verify: tests fail -> implement -> tests pass
4. **Phase: Plugin lifecycle** -- HTTP server start/stop, config parsing, engine query via SDK
   - Tests: unit tests for config parsing, server lifecycle
   - Files: `looking_glass.go`
   - Verify: tests fail -> implement -> tests pass
5. **Phase: Functional tests** -- .ci tests proving end-to-end wiring
   - Tests: `test/plugin/looking-glass/*.ci`
   - Files: .ci test files
   - Verify: `make ze-functional-test` passes
6. **Full verification** -> `make ze-verify` (lint + all ze tests except fuzz)
7. **Complete spec** -> Fill audit tables, write learned summary to `plan/learned/NNN-<name>.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Birdwatcher field names match Alice-LG expectations exactly |
| Naming | JSON keys use birdwatcher convention (snake_case), not Ze convention (kebab-case) |
| Data flow | Plugin queries engine via DispatchCommand only, no direct RIB import |
| Rule: plugin-design | Plugin in `plugins/looking-glass/`, YANG schema present, blank import only |
| Rule: no-layering | No duplicate HTTP server logic -- reuse patterns from metrics/chaos |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Plugin registers in inventory | `make ze-inventory` shows looking-glass |
| YANG schema exists | `ls internal/component/bgp/plugins/looking-glass/schema/` |
| HTTP server starts with config | functional test `status.ci` passes |
| `/status` returns valid JSON | functional test + curl in test |
| `/protocols/bgp` returns peer list | functional test `protocols.ci` passes |
| `/routes/protocol/{name}` returns routes | functional test `routes.ci` passes |
| Plugin disabled = no listener | functional test `disabled.ci` passes |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Peer name in URL path sanitized (no path traversal, no command injection) |
| Access control | Config option for allowed source IPs or listen on localhost only by default |
| Resource exhaustion | Pagination or response size limit on route queries |
| Error leakage | Error responses do not expose internal paths or stack traces |
| Timeout | Engine query timeout prevents hanging HTTP requests (5s default) |

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
