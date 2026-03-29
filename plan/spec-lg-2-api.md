# Spec: lg-2 -- Birdwatcher REST API

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-lg-1-core |
| Phase | - |
| Updated | 2026-03-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-lg-0-umbrella.md` - umbrella spec with design decisions
4. `internal/component/lg/server.go` - LG server (from lg-1-core)
5. `internal/component/bgp/plugins/rib/schema/ze-rib-api.yang` - RIB query RPCs
6. `internal/component/bgp/plugins/bgp/schema/ze-bgp-api.yang` - peer RPCs

## Task

Implement a birdwatcher-compatible REST API on the looking glass HTTP server. The API exposes
BGP session state and route information as JSON, using birdwatcher field naming conventions
so that existing looking glass frontends (Alice-LG, etc.) can consume it directly.

The API transforms Ze's internal JSON format (kebab-case) to birdwatcher format (snake_case)
and restructures the data to match the birdwatcher response schema that Alice-LG expects.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - RPC dispatch, command structure
  -> Decision:
  -> Constraint:
- [ ] `docs/architecture/api/commands.md` - existing RPC commands
  -> Decision:
  -> Constraint:

### RFC Summaries (MUST for protocol work)
- N/A

**Key insights:**
- To be filled during implementation

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/lg/server.go` - LG server mux (from lg-1-core)
- [ ] `internal/component/bgp/plugins/rib/rib.go` - RIB plugin, command handlers
- [ ] `internal/component/bgp/plugins/rib/schema/ze-rib-api.yang` - rib show, rib best, rib count
- [ ] `internal/component/bgp/plugins/bgp/schema/ze-bgp-api.yang` - bgp peer show, peer list

**Behavior to preserve:**
- Ze's internal JSON format (kebab-case) unchanged
- RPC command syntax unchanged
- LG server routing from lg-1-core

**Behavior to change:**
- LG server's `/api/looking-glass/` prefix gains handler registrations
- New transform layer from Ze JSON to birdwatcher JSON

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- HTTP GET to `/api/looking-glass/<endpoint>`
- Request from Alice-LG frontend or API client

### Transformation Path
1. HTTP handler parses endpoint path and query parameters
2. Handler maps birdwatcher endpoint to Ze RPC command
3. Handler calls `CommandDispatcher` with RPC command string
4. Engine returns Ze JSON response
5. Handler transforms Ze JSON to birdwatcher field names and structure
6. Handler writes JSON response with `Content-Type: application/json`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| API client -> LG server | HTTP GET with path/query params | [ ] |
| Handler -> Engine | CommandDispatcher with RPC string | [ ] |
| Ze JSON -> Birdwatcher JSON | Transform functions (field rename + restructure) | [ ] |

### Integration Points
- LG server mux from lg-1-core (register handlers under `/api/looking-glass/`)
- CommandDispatcher from hub (query engine for BGP data)
- Transform layer (Ze JSON to birdwatcher format)

### Architectural Verification
- [ ] No bypassed layers (all queries go through CommandDispatcher)
- [ ] No unintended coupling (transform layer has no imports from RIB/peer plugins)
- [ ] No duplicated functionality (reuses existing RPC commands)
- [ ] Zero-copy preserved where applicable (N/A for JSON)

## Birdwatcher API Endpoints

| Endpoint | Ze RPC | Description |
|----------|--------|-------------|
| `GET /api/looking-glass/status` | `bgp status` | Router ID, version, uptime, peer summary |
| `GET /api/looking-glass/protocols/bgp` | `bgp peer list` | All peers with state, route counts |
| `GET /api/looking-glass/routes/protocol/{name}` | `rib show peer {name}` | Routes received from named peer |
| `GET /api/looking-glass/routes/table/{family}` | `rib best {family}` | Best routes by address family |
| `GET /api/looking-glass/routes/filtered/{name}` | `rib show peer {name} filtered` | Filtered routes per peer |
| `GET /api/looking-glass/routes/search?prefix={prefix}` | `rib show prefix {prefix}` | Prefix lookup across all peers |

## Birdwatcher Field Mapping

### Status Response

| Birdwatcher field | Ze source | Type |
|-------------------|-----------|------|
| `router_id` | `router-id` | string |
| `server_time` | current time | ISO 8601 string |
| `last_reboot` | `start-time` | ISO 8601 string |
| `last_reconfig` | `last-config-change` | ISO 8601 string |
| `message` | `"Ze BGP daemon"` | string |
| `version` | Ze version string | string |

### Protocol (Peer) Response

| Birdwatcher field | Ze source | Type |
|-------------------|-----------|------|
| `bird_protocol` | peer config name | string |
| `state` | `state` | string |
| `neighbor_address` | `peer-address` | string |
| `neighbor_as` | `remote-as` | integer |
| `description` | `description` or `""` | string |
| `routes_received` | `routes-received` | integer |
| `routes_imported` | `routes-accepted` | integer |
| `routes_exported` | `routes-sent` | integer |
| `routes_filtered` | `routes-filtered` | integer |
| `uptime` | computed from `established-time` | integer (seconds) |

### Route Response

| Birdwatcher field | Ze source | Type |
|-------------------|-----------|------|
| `network` | `prefix` | string |
| `gateway` | `next-hop` | string |
| `metric` | `med` | integer |
| `interface` | `""` | string (N/A for Ze) |
| `from_protocol` | peer name | string |
| `age` | computed from timestamp | integer (seconds) |
| `bgp.origin` | `origin` | string |
| `bgp.as_path` | `as-path` | array of integers |
| `bgp.next_hop` | `next-hop` | string |
| `bgp.local_pref` | `local-preference` | integer |
| `bgp.med` | `med` | integer |
| `bgp.community` | `community` | array of strings |
| `bgp.large_community` | `large-community` | array of strings |
| `bgp.ext_community` | `extended-community` | array of strings |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| HTTP GET `/api/looking-glass/status` | -> | Returns status JSON | `test/plugin/looking-glass/api-status.ci` |
| HTTP GET `/api/looking-glass/protocols/bgp` | -> | Returns peer list JSON | `test/plugin/looking-glass/api-protocols.ci` |
| HTTP GET `/api/looking-glass/routes/protocol/{name}` | -> | Returns routes JSON | `test/plugin/looking-glass/api-routes.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | GET `/api/looking-glass/status` | Returns JSON with `router_id`, `server_time`, `version` fields |
| AC-2 | GET `/api/looking-glass/protocols/bgp` | Returns JSON with peer list, each having `state`, `neighbor_address`, `neighbor_as`, `routes_received`, `routes_imported` |
| AC-3 | GET `/api/looking-glass/routes/protocol/{name}` with valid peer | Returns JSON with routes from peer, each with `network`, `gateway`, `bgp.as_path`, `bgp.local_pref` |
| AC-4 | GET `/api/looking-glass/routes/protocol/{name}` with invalid peer | Returns HTTP 404 with JSON error |
| AC-5 | GET `/api/looking-glass/routes/table/{family}` | Returns JSON with best routes for family |
| AC-6 | GET `/api/looking-glass/routes/search?prefix=10.0.0.0/24` | Returns JSON with matching routes across all peers |
| AC-7 | All API responses | Content-Type: application/json |
| AC-8 | All API JSON keys | Use birdwatcher convention (snake_case), not Ze convention (kebab-case) |
| AC-9 | Engine unreachable | Returns HTTP 503 with JSON error body |
| AC-10 | GET to unknown API path | Returns HTTP 404 with JSON error body |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTransformStatus` | `internal/component/lg/transform_test.go` | AC-1: status field mapping | |
| `TestTransformProtocol` | `internal/component/lg/transform_test.go` | AC-2: peer field mapping | |
| `TestTransformRoute` | `internal/component/lg/transform_test.go` | AC-3: route field mapping | |
| `TestTransformRouteWithCommunities` | `internal/component/lg/transform_test.go` | AC-3: community fields | |
| `TestAPIStatusHandler` | `internal/component/lg/handler_api_test.go` | AC-1: status endpoint | |
| `TestAPIProtocolsHandler` | `internal/component/lg/handler_api_test.go` | AC-2: protocols endpoint | |
| `TestAPIRoutesHandler` | `internal/component/lg/handler_api_test.go` | AC-3: routes endpoint | |
| `TestAPIUnknownPeer` | `internal/component/lg/handler_api_test.go` | AC-4: 404 on bad peer | |
| `TestAPIUnknownPath` | `internal/component/lg/handler_api_test.go` | AC-10: 404 on bad path | |
| `TestAPIContentType` | `internal/component/lg/handler_api_test.go` | AC-7: Content-Type header | |
| `TestAPIEngineError` | `internal/component/lg/handler_api_test.go` | AC-9: 503 on engine error | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric inputs in API query params) | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-lg-api-status` | `test/plugin/looking-glass/api-status.ci` | GET /status returns router info | |
| `test-lg-api-protocols` | `test/plugin/looking-glass/api-protocols.ci` | GET /protocols/bgp lists peers | |
| `test-lg-api-routes` | `test/plugin/looking-glass/api-routes.ci` | GET /routes/protocol/{peer} returns routes | |

### Future (if deferring any tests)
- Alice-LG end-to-end integration test
- Pagination for large route sets
- Response caching

## Files to Modify
- `internal/component/lg/server.go` - register API handlers on mux

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | N/A (API has no config beyond lg-1-core) |
| CLI commands/flags | [ ] | N/A |
| Editor autocomplete | [ ] | N/A |
| Functional test | [x] | `test/plugin/looking-glass/api-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | N/A (in umbrella) |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` - birdwatcher API endpoints |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/looking-glass.md` - API section |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A (in umbrella) |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create
- `internal/component/lg/handler_api.go` - API HTTP handlers
- `internal/component/lg/handler_api_test.go` - API handler unit tests
- `internal/component/lg/transform.go` - Ze JSON to birdwatcher field mapping
- `internal/component/lg/transform_test.go` - transform unit tests
- `test/plugin/looking-glass/api-status.ci` - functional test: status
- `test/plugin/looking-glass/api-protocols.ci` - functional test: protocols
- `test/plugin/looking-glass/api-routes.ci` - functional test: routes

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

1. **Phase: Transform layer** -- Ze JSON to birdwatcher field mapping functions
   - Tests: `TestTransformStatus`, `TestTransformProtocol`, `TestTransformRoute`, `TestTransformRouteWithCommunities`
   - Files: `transform.go`, `transform_test.go`
   - Verify: tests fail -> implement -> tests pass
2. **Phase: API handlers** -- /status, /protocols/bgp, /routes endpoints
   - Tests: `TestAPIStatusHandler`, `TestAPIProtocolsHandler`, `TestAPIRoutesHandler`, `TestAPIUnknownPeer`, `TestAPIUnknownPath`, `TestAPIContentType`, `TestAPIEngineError`
   - Files: `handler_api.go`, `handler_api_test.go`
   - Verify: tests fail -> implement -> tests pass
3. **Phase: Handler registration** -- wire handlers into LG server mux
   - Files: `server.go` (mux registration)
   - Verify: routing tests pass
4. **Phase: Functional tests** -- .ci tests for end-to-end API
   - Tests: `test/plugin/looking-glass/api-*.ci`
   - Verify: `make ze-functional-test` passes
5. **Full verification** -> `make ze-verify`
6. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Birdwatcher field names match Alice-LG expectations exactly |
| Naming | API JSON keys are snake_case (birdwatcher), not kebab-case (Ze) |
| Data flow | All queries via CommandDispatcher, no direct plugin imports |
| Rule: json-format | API uses birdwatcher convention, not Ze convention (documented exception) |
| Rule: plugin-design | No imports from RIB or peer plugin packages |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Transform layer exists | `ls internal/component/lg/transform.go` |
| API handlers exist | `ls internal/component/lg/handler_api.go` |
| Status endpoint works | functional test `api-status.ci` passes |
| Protocols endpoint works | functional test `api-protocols.ci` passes |
| Routes endpoint works | functional test `api-routes.ci` passes |
| All JSON is snake_case | grep for kebab-case keys in transform.go (should be zero) |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Peer name and prefix in URL path sanitized |
| Injection | No command injection via URL parameters into RPC strings |
| Resource exhaustion | Response size bounded (pagination or limit) |
| Error leakage | Error responses do not expose internal paths or stack traces |

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
