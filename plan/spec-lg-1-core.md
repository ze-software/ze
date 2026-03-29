# Spec: lg-1 -- Looking Glass Core

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-lg-0-umbrella.md` - umbrella spec with design decisions
4. `internal/component/web/server.go` - HTTP server pattern to follow
5. `internal/component/web/schema/ze-web-conf.yang` - YANG env config pattern
6. `cmd/ze/hub/main.go` - hub startup integration point

## Task

Create the looking glass component skeleton: directory structure, YANG configuration schema,
HTTP server with TLS support, handler routing (mux), and hub startup integration. This is the
foundation that lg-2 (API), lg-3 (UI), and lg-4 (graph) build upon.

The component follows the same pattern as `internal/component/web/`: YANG-configured, started
from hub, serves HTTP on its own port. Unlike the web UI, the looking glass is public (no auth)
and TLS is optional (LG is often behind a reverse proxy).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component architecture
  -> Decision:
  -> Constraint:
- [ ] `.claude/rules/plugin-design.md` - component vs plugin, proximity
  -> Decision:
  -> Constraint:

### RFC Summaries (MUST for protocol work)
- N/A

**Key insights:**
- To be filled during implementation

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/server.go` - WebServer struct, TLS setup, ListenAndServe, WaitReady
- [ ] `internal/component/web/handler.go` - ParseURL, mux registration, content negotiation
- [ ] `internal/component/web/schema/ze-web-conf.yang` - env container YANG pattern
- [ ] `internal/component/web/schema/embed.go` - YANG embedding pattern
- [ ] `internal/component/web/schema/register.go` - YANG registration pattern
- [ ] `cmd/ze/hub/main.go` - startWebServer(), component wiring

**Behavior to preserve:**
- Web UI server unchanged (separate port, separate server)
- Hub startup sequence
- YANG schema registration pattern
- TLS certificate infrastructure (CertStore in zefs)

**Behavior to change:**
- Hub gains a second HTTP server startup for looking glass
- New YANG module `ze-lg-conf` under `environment/looking-glass`

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- HTTP request arrives at LG server on configured port
- Hub provides `CommandDispatcher` function to LG server at startup

### Transformation Path
1. HTTP request received by LG server
2. Mux routes to appropriate handler based on path prefix
3. Handler (in lg-2/lg-3/lg-4) processes the request
4. Handler calls `CommandDispatcher` for BGP data
5. Handler renders response (JSON or HTML)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| HTTP client -> LG server | HTTP/HTTPS on configured port | [ ] |
| LG handler -> Engine | CommandDispatcher (Go function call) | [ ] |

### Integration Points
- `yang.RegisterModule()` in `init()` for YANG schema
- Hub `main.go` creates and starts LG server
- `CommandDispatcher` passed from hub to LG server

### Architectural Verification
- [ ] No bypassed layers (LG is a separate HTTP server, not extending web UI)
- [ ] No unintended coupling (LG package has no imports from web package)
- [ ] No duplicated functionality (follows same pattern as web, does not copy code)
- [ ] Zero-copy preserved where applicable (N/A)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG config with looking-glass section | -> | LG server binds on configured port | `test/parse/lg-config.ci` |
| YANG config without looking-glass section | -> | No LG server started | `test/parse/lg-disabled.ci` |
| HTTP GET to LG server root | -> | Returns 200 (or redirect) | TBD in lg-2/lg-3 |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | YANG config specifies `environment/looking-glass` with host and port | LG HTTP server binds on specified address:port |
| AC-2 | No `looking-glass` config section | No HTTP server started, no resource usage |
| AC-3 | Config specifies `tls true` | Server uses TLS (self-signed or provided cert) |
| AC-4 | Config specifies `tls false` (default) | Server uses plain HTTP |
| AC-5 | HTTP request to `/lg/` prefix | Routed to UI handler (returns 404 until lg-3 implemented) |
| AC-6 | HTTP request to `/api/looking-glass/` prefix | Routed to API handler (returns 404 until lg-2 implemented) |
| AC-7 | Server shutdown signal | Graceful shutdown (in-flight requests complete) |
| AC-8 | Config specifies custom port | Server uses configured port, not default |
| AC-9 | Invalid port in config | Config validation rejects it |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNewLGServer` | `internal/component/lg/server_test.go` | AC-1: server created with config | |
| `TestLGServerRouting` | `internal/component/lg/server_test.go` | AC-5, AC-6: mux routes correctly | |
| `TestLGServerDisabled` | `internal/component/lg/server_test.go` | AC-2: nil config means no server | |
| `TestLGServerTLS` | `internal/component/lg/server_test.go` | AC-3: TLS mode | |
| `TestLGServerPlainHTTP` | `internal/component/lg/server_test.go` | AC-4: plain HTTP mode | |
| `TestLGServerGracefulShutdown` | `internal/component/lg/server_test.go` | AC-7: graceful shutdown | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| port | 1-65535 | 65535 | 0 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-lg-config` | `test/parse/lg-config.ci` | Config with LG section parses without error | |
| `test-lg-disabled` | `test/parse/lg-disabled.ci` | Config without LG section: no listener | |

### Future (if deferring any tests)
- TLS certificate rotation
- Connection limit / rate limiting

## Files to Modify
- `cmd/ze/hub/main.go` - add LG server startup function

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
| 1 | New user-facing feature? | [x] | `docs/features.md` - looking glass component |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` - looking-glass env section |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A (API in lg-2) |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | N/A (guide in umbrella) |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A (in umbrella) |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` - LG component |

## Files to Create
- `internal/component/lg/server.go` - LG HTTP server (bind, TLS, mux, lifecycle)
- `internal/component/lg/server_test.go` - server unit tests
- `internal/component/lg/schema/ze-lg-conf.yang` - YANG config schema
- `internal/component/lg/schema/embed.go` - embedded YANG
- `internal/component/lg/schema/register.go` - YANG module registration
- `internal/component/lg/schema/schema_test.go` - schema validation test
- `test/parse/lg-config.ci` - config parsing functional test
- `test/parse/lg-disabled.ci` - disabled config functional test

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

1. **Phase: YANG schema** -- define ze-lg-conf.yang, embed, register
   - Tests: `schema_test.go`
   - Files: `schema/ze-lg-conf.yang`, `schema/embed.go`, `schema/register.go`
   - Verify: module loads, config validates
2. **Phase: HTTP server** -- LGServer struct, bind, TLS, mux, shutdown
   - Tests: `TestNewLGServer`, `TestLGServerTLS`, `TestLGServerPlainHTTP`, `TestLGServerGracefulShutdown`
   - Files: `server.go`, `server_test.go`
   - Verify: tests fail -> implement -> tests pass
3. **Phase: Handler routing** -- mux registration for /lg/ and /api/looking-glass/ prefixes
   - Tests: `TestLGServerRouting`
   - Files: `server.go` (mux setup)
   - Verify: tests fail -> implement -> tests pass
4. **Phase: Hub integration** -- startup function in hub main.go
   - Tests: `TestLGServerDisabled`
   - Files: `cmd/ze/hub/main.go`
   - Verify: tests fail -> implement -> tests pass
5. **Phase: Functional tests** -- config parsing tests
   - Tests: `test/parse/lg-config.ci`, `test/parse/lg-disabled.ci`
   - Verify: `make ze-functional-test` passes
6. **Full verification** -> `make ze-verify`
7. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Server binds on configured port, TLS works when enabled |
| Naming | YANG uses kebab-case, Go types follow existing web pattern |
| Data flow | CommandDispatcher wired from hub, not hardcoded |
| Rule: design-principles | No premature handlers (just mux skeleton until lg-2/lg-3) |
| Rule: no-layering | Not extending web server; separate server instance |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Component directory exists | `ls internal/component/lg/` |
| YANG schema exists | `ls internal/component/lg/schema/ze-lg-conf.yang` |
| Server compiles | `go build ./internal/component/lg/...` |
| Config parses | functional test `lg-config.ci` passes |
| Disabled config works | functional test `lg-disabled.ci` passes |
| Hub integration | grep for LG startup in `cmd/ze/hub/main.go` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| No auth endpoints exposed | LG server does not serve web UI config/admin paths |
| TLS configuration | When TLS enabled, minimum TLS 1.2 enforced |
| Port binding | Server only binds on configured address, not additional interfaces |
| Graceful shutdown | Server drains connections on shutdown, does not hang |

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
