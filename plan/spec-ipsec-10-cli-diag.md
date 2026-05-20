# Spec: ipsec-10 -- IPsec CLI and Diagnostics

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | ipsec-8 |
| Phase | - |
| Updated | 2026-05-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `spec-ipsec-0-umbrella.md` -- umbrella design decisions
4. `spec-ipsec-7-ikev2-engine.md` -- IKE engine SA table and bus events (this spec consumes them)
5. `spec-ipsec-8-ikev2-child-xfrm.md` -- Child SA state and dataplane (this spec queries it)
5. `internal/component/cmd/show/` -- existing show command patterns
6. `internal/component/cmd/clear/` -- existing clear command patterns
7. `internal/component/cli/model_ping.go` -- monitor model pattern (live streaming via bus)
8. `internal/component/web/page_system.go` -- web page pattern (HTMX + SSE)
9. `internal/component/telemetry/collector/` -- Prometheus collector pattern
10. `internal/core/health/` -- health check registration

## Task

Add CLI commands, web UI, health monitoring, and Prometheus metrics for IPsec tunnels.
This is the presentation and observability layer for the IPsec subsystem. All data
comes from the IKE engine's in-memory SA table (spec ipsec-7/8) and bus events;
this spec creates no new kernel or network interactions.

Six command groups:
- `show vpn ipsec sa` -- list all Security Associations
- `show vpn ipsec status` -- overall IPsec subsystem status
- `show vpn ipsec peer <name>` -- per-peer detail
- `clear vpn ipsec sa` -- terminate all SAs (IKE engine re-establishes per close-action)
- `clear vpn ipsec sa peer <name>` -- terminate specific peer
- `monitor vpn ipsec` -- live SA state change stream

All show commands produce JSON and flow through `ApplyPipes` for full pipe operator
support. The monitor command follows the `model_ping.go` / `model_traceroute.go`
pattern: a long-lived CLI model that subscribes to bus events and renders updates.

A web page at `/vpn/ipsec` provides a live SA table with SSE updates, matching
the existing HTMX pattern used by system and BGP pages.

Health checks register the IPsec component and drive status from tunnel state
(all up = healthy, some down = degraded, engine stopped = down).

Prometheus metrics expose tunnel counts, per-peer up/down gauges, byte counters,
and rekey counts.

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` -- interface management patterns, bus topics
  -> Constraint: IPsec bus events follow existing topic naming convention (e.g., `vpn/ipsec/sa/up`)
- [ ] `internal/component/cmd/show/doc.go` -- show command registration pattern
  -> Constraint: show handlers return JSON string, ApplyPipes handles formatting
- [ ] `internal/component/cmd/clear/doc.go` -- clear command pattern
  -> Constraint: clear handlers return success/error JSON, same pipe flow
- [ ] `internal/component/cli/model_ping.go` -- monitor model pattern for live CLI streaming
  -> Decision: monitor vpn ipsec uses the same bus-subscription model as monitor ping
- [ ] `internal/component/web/page_system.go` -- web page with SSE pattern
  -> Decision: IPsec web page follows same HTMX + SSE live update approach
- [ ] `internal/core/health/registry.go` -- health check registration
  -> Constraint: register IPsec health check in component init, not at first use
- [ ] `internal/component/telemetry/collector/` -- Prometheus collector pattern
  -> Decision: IPsec collector follows same Collect/Describe interface

**Key insights:**
- Show commands return JSON strings; the pipe layer handles json/table/text/yaml/match/count/resolve/origin
- Monitor models subscribe to bus events and render each event as it arrives
- Web SSE streams the same bus events to the browser for live updates
- Health checks are stateless functions called on demand by the health registry
- Prometheus collectors scrape on demand; IPsec collector queries IKE engine for current SA state

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cmd/show/firewall.go` -- show command pattern: dispatch handler receives context, queries state, returns JSON string via ApplyPipes
  -> Constraint: IPsec show handlers follow same signature and JSON return pattern
- [ ] `internal/component/cmd/clear/dns.go` -- clear command pattern: dispatch handler calls action, returns success JSON
  -> Constraint: clear ipsec sa follows same pattern, calls IKE engine to terminate SAs
- [ ] `internal/component/cli/model_ping.go` -- monitor ping: NewPingModel subscribes to bus, renders hop stats in real-time
  -> Decision: NewIPsecMonitorModel subscribes to vpn/ipsec/* bus topics, renders SA state changes
- [ ] `internal/component/web/page_system.go` -- web page: register route, render template, SSE endpoint for live data
  -> Decision: `/vpn/ipsec` page registers same way, SSE streams SA events
- [ ] `internal/core/health/registry.go` -- health.Registry.Register(name, checkFn) at component startup
  -> Constraint: IPsec registers check that queries IKE engine for SA health state
- [ ] `internal/component/telemetry/collector/` -- Prometheus Collector interface: Describe sends metric descs, Collect sends current values
  -> Decision: IPsec collector queries IKE engine ListSAs on each scrape

**Behavior to preserve:**
- Existing show/clear command dispatch unchanged
- Existing monitor model framework unchanged
- Existing web page routing unchanged
- Existing health registry API unchanged
- Existing Prometheus collector registration unchanged
- All pipe operators work on IPsec output (json, table, text, yaml, match, count, resolve, origin)

**Behavior to change:**
- New `show vpn ipsec sa`, `show vpn ipsec status`, `show vpn ipsec peer <name>` commands
- New `clear vpn ipsec sa`, `clear vpn ipsec sa peer <name>` commands
- New `monitor vpn ipsec` CLI model
- New `/vpn/ipsec` web page with SSE live updates
- New IPsec health check registration
- New IPsec Prometheus collector

## Data Flow (MANDATORY)

### Entry Point
- CLI: user types `show vpn ipsec sa` in SSH CLI session
- CLI: user types `monitor vpn ipsec` for live streaming
- Web: browser requests `/vpn/ipsec` page or subscribes to SSE endpoint
- Prometheus: scrape hits `/metrics` endpoint, collector queries IKE engine
- Health: health registry calls IPsec check function on demand

### Transformation Path
1. CLI dispatch matches `show vpn ipsec sa` dispatch key, calls show handler
2. Show handler calls IPsec component's IKE engine `ListSAs()` method (from ipsec-4)
3. IKE engine returns `[]SAInfo` Go structs directly (in-process call)
4. Handler marshals structs to JSON string
5. JSON string passed through `ApplyPipes` which formats per active pipe operator
6. Formatted output returned to CLI session

For monitor:
1. CLI dispatches `monitor vpn ipsec`, creates IPsecMonitorModel
2. Model subscribes to `vpn/ipsec/sa/*` bus topics
3. Each bus event (sa-up, sa-down, sa-rekeyed) rendered as a status line
4. Model continues until user exits (Ctrl-C or `q`)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI dispatch to show handler | YANG-registered dispatch key to Go handler function | [ ] |
| Show handler to IKE engine | Go method call on IPsec component's IKE engine | [ ] |
| Show handler to IKE engine SA table | In-process Go method call (ListSAs, GetPeer) | [ ] |
| Show handler to pipe layer | JSON string passed to ApplyPipes | [ ] |
| Bus events to monitor model | EventBus subscription with topic filter | [ ] |
| Bus events to web SSE | EventBus subscription in SSE handler goroutine | [ ] |
| Prometheus scrape to collector | Collector.Collect() called by Prometheus registry | [ ] |

### Integration Points
- `internal/component/ike/` -- IKE engine SA table for queries (from ipsec-7/8)
- `internal/component/command/pipe.go` -- ApplyPipes for pipe operator support
- `internal/core/events/EventBus` -- bus subscription for monitor and web SSE
- `internal/core/health/Registry` -- health check registration
- `internal/component/telemetry/` -- Prometheus collector registration

### Architectural Verification
- [ ] No bypassed layers (show commands go through dispatch + pipes, not direct engine access)
- [ ] No unintended coupling (CLI layer depends on IPsec component interface, not engine internals)
- [ ] No duplicated functionality (reuses existing pipe, bus, health, telemetry infrastructure)
- [ ] Zero-copy preserved where applicable (JSON strings, not re-serialized)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `show vpn ipsec sa` dispatch key | -> | IPsec SA show handler queries IKE engine, returns JSON | `test/ipsec/ipsec-show-sa.ci` |
| `show vpn ipsec status` dispatch key | -> | IPsec status handler returns engine + connection state | `test/ipsec/ipsec-show-status.ci` |
| `show vpn ipsec peer <name>` dispatch key | -> | Per-peer detail handler returns IKE SA + child SAs | `test/ipsec/ipsec-show-peer.ci` |
| `clear vpn ipsec sa` dispatch key | -> | Clear handler calls IKE engine to terminate SAs | `test/ipsec/ipsec-clear-sa.ci` |
| `monitor vpn ipsec` dispatch key | -> | IPsecMonitorModel subscribes to bus, renders events | `test/ipsec/ipsec-monitor.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show vpn ipsec sa` with active tunnels | Lists all SAs with peer name, algorithms, bytes in/out, rekey time, uptime |
| AC-2 | `show vpn ipsec sa \| json` | Produces valid JSON array of SA objects |
| AC-3 | `show vpn ipsec status` | Shows IKE engine state, count of configured connections, count of active IKE SAs |
| AC-4 | `show vpn ipsec peer management-bridge` | Shows detailed peer info: IKE SA state, child SAs with traffic selectors, byte counts |
| AC-5 | `clear vpn ipsec sa` | Terminates all IKE SAs via IKE engine; peers re-establish per close-action |
| AC-6 | `clear vpn ipsec sa peer management-bridge` | Terminates only the named peer's IKE SA |
| AC-7 | `monitor vpn ipsec` with tunnel flap | Streams SA state changes (up, down, rekeyed) as they happen |
| AC-8 | Browser at `/vpn/ipsec` | Shows SA table with live SSE updates when SA state changes |
| AC-9 | IPsec component running | Health check registered; all tunnels up = healthy, some down = degraded, engine stopped = down |
| AC-10 | Prometheus scrape at `/metrics` | Exposes ipsec_sa_count, ipsec_tunnel_up (gauge per peer), ipsec_bytes_in_total, ipsec_bytes_out_total, ipsec_rekey_total |
| AC-11 | `show vpn ipsec sa \| table`, `\| text`, `\| yaml`, `\| match`, `\| count` | All pipe operators produce correct output |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestShowIPsecSA` | `internal/component/cmd/show/ipsec_test.go` | SA query returns JSON with peer, algo, bytes, rekey, uptime fields | |
| `TestShowIPsecSAEmpty` | `internal/component/cmd/show/ipsec_test.go` | No active SAs returns empty JSON array | |
| `TestShowIPsecStatus` | `internal/component/cmd/show/ipsec_test.go` | Status includes engine state, configured count, active count | |
| `TestShowIPsecPeer` | `internal/component/cmd/show/ipsec_test.go` | Per-peer detail includes IKE SA + child SAs | |
| `TestShowIPsecPeerNotFound` | `internal/component/cmd/show/ipsec_test.go` | Unknown peer returns error JSON | |
| `TestClearIPsecSA` | `internal/component/cmd/clear/ipsec_test.go` | Clear calls IKE engine to terminate all IKE SAs | |
| `TestClearIPsecSAPeer` | `internal/component/cmd/clear/ipsec_test.go` | Clear with peer name terminates only that peer | |
| `TestIPsecMonitorModel` | `internal/component/cli/model_ipsec_test.go` | Model receives bus events and renders state changes | |
| `TestIPsecHealthCheck` | `internal/component/ipsec/health_test.go` | Health function returns healthy/degraded/down based on SA state | |
| `TestIPsecCollector` | `internal/component/telemetry/collector/ipsec_test.go` | Collector emits correct metric names, labels, values | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Peer name length | 1-255 | 255 chars | empty string | 256 chars |
| SA bytes counter | 0-uint64 max | uint64 max | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-show-sa` | `test/ipsec/ipsec-show-sa.ci` | User runs `show vpn ipsec sa` and sees SA table | |
| `ipsec-show-status` | `test/ipsec/ipsec-show-status.ci` | User runs `show vpn ipsec status` and sees subsystem state | |
| `ipsec-show-peer` | `test/ipsec/ipsec-show-peer.ci` | User runs `show vpn ipsec peer <name>` and sees peer detail | |
| `ipsec-clear-sa` | `test/ipsec/ipsec-clear-sa.ci` | User runs `clear vpn ipsec sa` and SAs are terminated | |
| `ipsec-monitor` | `test/ipsec/ipsec-monitor.ci` | User runs `monitor vpn ipsec` and sees live state changes | |

## Files to Modify
- `internal/component/ipsec/register.go` -- register health check during component init
- `internal/component/cmd/show/schema/` -- YANG schema entries for show vpn ipsec dispatch keys
- `internal/component/cmd/clear/schema/` -- YANG schema entries for clear vpn ipsec dispatch keys
- `internal/component/web/register.go` -- register `/vpn/ipsec` web page route
- `internal/component/telemetry/register.go` -- register IPsec Prometheus collector

## Files to Create
- `internal/component/cmd/show/ipsec.go` -- show vpn ipsec sa, show vpn ipsec status, show vpn ipsec peer handlers
- `internal/component/cmd/show/ipsec_test.go` -- unit tests for show handlers
- `internal/component/cmd/clear/ipsec.go` -- clear vpn ipsec sa handlers
- `internal/component/cmd/clear/ipsec_test.go` -- unit tests for clear handlers
- `internal/component/cli/model_ipsec.go` -- monitor vpn ipsec CLI model (bus-subscribed live stream)
- `internal/component/cli/model_ipsec_test.go` -- unit tests for monitor model
- `internal/component/web/page_ipsec.go` -- web status page with HTMX + SSE
- `internal/component/telemetry/collector/ipsec.go` -- Prometheus collector (sa count, tunnel up gauge, bytes, rekeys)
- `internal/component/telemetry/collector/ipsec_test.go` -- unit tests for collector
- `internal/component/ipsec/health.go` -- health check function
- `internal/component/ipsec/health_test.go` -- unit tests for health check
- `test/ipsec/ipsec-show-sa.ci` -- functional test for show vpn ipsec sa
- `test/ipsec/ipsec-show-status.ci` -- functional test for show vpn ipsec status
- `test/ipsec/ipsec-show-peer.ci` -- functional test for show vpn ipsec peer
- `test/ipsec/ipsec-clear-sa.ci` -- functional test for clear vpn ipsec sa
- `test/ipsec/ipsec-monitor.ci` -- functional test for monitor vpn ipsec

## Implementation Steps

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** -- register dispatch keys, write failing wiring tests
   - Tests: all wiring test names from table above
   - Files: YANG schema entries for dispatch keys, stub handlers in show/clear/model files
   - Verify: dispatch keys resolve to handler stubs; wiring tests fail because stubs return empty

2. **Phase: Show commands** -- implement SA query, status, per-peer detail
   - Tests: `TestShowIPsecSA`, `TestShowIPsecSAEmpty`, `TestShowIPsecStatus`, `TestShowIPsecPeer`, `TestShowIPsecPeerNotFound`
   - Files: `internal/component/cmd/show/ipsec.go`
   - Verify: show handlers query IKE engine and return correct JSON; pipe operators work

3. **Phase: Clear commands** -- implement SA termination
   - Tests: `TestClearIPsecSA`, `TestClearIPsecSAPeer`
   - Files: `internal/component/cmd/clear/ipsec.go`
   - Verify: clear calls IKE engine to terminate SAs, returns success/error JSON

4. **Phase: Monitor model** -- live SA state stream via bus events
   - Tests: `TestIPsecMonitorModel`
   - Files: `internal/component/cli/model_ipsec.go`
   - Verify: model renders bus events as they arrive, exits cleanly

5. **Phase: Web page** -- SA table with SSE live updates
   - Files: `internal/component/web/page_ipsec.go`
   - Verify: page renders SA table, SSE stream delivers live updates

6. **Phase: Health and metrics** -- health check registration, Prometheus collector
   - Tests: `TestIPsecHealthCheck`, `TestIPsecCollector`
   - Files: `internal/component/ipsec/health.go`, `internal/component/telemetry/collector/ipsec.go`
   - Verify: health check drives healthy/degraded/down, collector emits correct metrics

7. **Functional tests** -- create after feature works, cover user-visible behavior
8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | JSON output matches documented field names; pipe operators produce valid output |
| Naming | YANG dispatch keys use kebab-case; JSON keys use kebab-case; Prometheus metric names use snake_case with `ipsec_` prefix |
| Data flow | Show handlers query IKE engine via component interface, not directly |
| Rule: pipe-completeness | All show commands route through ApplyPipes; monitor path honors resolve/origin for IPs |
| Rule: derive-not-hardcode | SA algorithm names derived from IKE engine SA state, not hardcoded lists |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `show vpn ipsec sa` dispatch key resolves | `grep -rn 'show.*vpn.*ipsec.*sa' internal/component/cmd/show/schema/` |
| `clear vpn ipsec sa` dispatch key resolves | `grep -rn 'clear.*vpn.*ipsec.*sa' internal/component/cmd/clear/schema/` |
| Monitor model exists | `ls internal/component/cli/model_ipsec.go` |
| Web page registered | `grep -rn 'vpn/ipsec' internal/component/web/` |
| Health check registered | `grep -rn 'health.*ipsec\|ipsec.*health' internal/component/ipsec/` |
| Prometheus collector registered | `grep -rn 'ipsec' internal/component/telemetry/collector/` |
| Functional tests exist | `ls test/ipsec/ipsec-show-sa.ci test/ipsec/ipsec-clear-sa.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Peer name parameter validated (length, characters) before passing to IKE engine |
| Information leakage | Show output does not expose private keys or PSK material |
| Resource exhaustion | Monitor model limits event buffer; Prometheus collector has scrape timeout |
| Injection | Peer name not interpolated into shell commands (IKE engine is in-process Go) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior; RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

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
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (to be filled)

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
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

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ipsec-10-cli-diag.md`
- [ ] Summary included in commit
