# Spec: bng-1 -- RADIUS Attribute Consumption

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-l2tp-8b-radius (done) |
| Phase | 1/9 |
| Updated | 2026-05-07 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/plugins/l2tpauthradius/handler.go` -- unsupportedAccessAcceptAttrs map (lines 18-25)
4. `internal/component/ppp/ip_events.go` -- IPResponseArgs struct
5. `internal/plugins/l2tppool/pool.go` -- pool allocation interface
6. `internal/component/l2tp/session.go` -- L2TPSession struct

## Task

Wire consumption of RADIUS Access-Accept attributes that are currently
explicitly rejected. The handler at `handler.go:18-25` maintains a
`unsupportedAccessAcceptAttrs` map that rejects sessions when the RADIUS
server returns Framed-IP-Address, Framed-IP-Netmask, Framed-Pool,
Session-Timeout, Idle-Timeout, Filter-Id, or Acct-Interim-Interval.

For a production BNG, these attributes carry the subscriber profile:
- **Framed-IP-Address / Framed-IP-Netmask**: RADIUS-assigned IP, bypasses pool
- **Framed-Pool**: selects a named pool for IP allocation
- **Session-Timeout / Idle-Timeout**: session lifetime enforcement
- **Acct-Interim-Interval**: overrides default accounting interval per-session
- **Filter-Id**: rate plan identifier (already consumed by CoA, but not at auth time)

This spec removes the rejection logic and wires each attribute to its
consumer: RADIUS-assigned IPs go to the PPP IP response path (bypassing
pool), pool name routes to a named pool, timeouts drive session teardown
timers, and Filter-Id sets the initial shaping rate.

## Required Reading

### Architecture Docs
- [ ] `docs/research/l2tpv2-ze-integration.md` -- section 6: plugin design
- [ ] `internal/plugins/l2tpauthradius/handler.go` -- current rejection logic
- [ ] `internal/component/ppp/ip_events.go` -- IPResponseArgs, EventIPRequest
- [ ] `internal/component/ppp/session_run.go` -- NCP phase, IP request/response flow
- [ ] `internal/plugins/l2tppool/pool.go` -- pool allocation interface
- [ ] `internal/plugins/l2tpshaper/shaper.go` -- SessionUp rate application
- [ ] `internal/component/l2tp/session.go` -- L2TPSession struct fields

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2865.md` -- RADIUS Authentication: Framed-IP-Address (8), Framed-IP-Netmask (9), Filter-Id (11), Session-Timeout (27), Idle-Timeout (28), Framed-Pool (88)
- [ ] `rfc/short/rfc2866.md` -- RADIUS Accounting: Acct-Interim-Interval (85)

**Key insights:**
- handler.go rejects sessions on ANY unsupported attribute present (lines 122-130)
- The pool plugin allocates addresses independently via IPEventsOut channel
- RADIUS-assigned IP must bypass the pool (direct IP response from auth handler)
- Session/Idle timeout need a per-session timer in the reactor or session goroutine
- Acct-Interim-Interval overrides the plugin-global interval for one session

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/l2tpauthradius/handler.go` -- rejects session if any attr in unsupportedAccessAcceptAttrs map is present in Access-Accept
- [ ] `internal/plugins/l2tpauthradius/acct.go` -- uses plugin-global interval for all sessions
- [ ] `internal/component/ppp/ip_events.go` -- IPResponseArgs: Accept, Family, Local, Peer, DNS
- [ ] `internal/component/l2tp/session.go` -- L2TPSession struct, no timeout fields currently
- [ ] `internal/plugins/l2tppool/pool.go` -- single unnamed pool, allocates from bitmap

**Behavior to preserve:**
- Sessions without RADIUS attributes continue to work through pool
- Pool-based allocation remains the default when no Framed-IP-Address
- Existing CoA Filter-Id rate change continues to work
- Accounting interim interval remains configurable at plugin level (default 300s)

**Behavior to change:**
- Remove session rejection on unsupported attributes
- Extract and act on Framed-IP-Address/Netmask (direct IP assignment)
- Extract and route Framed-Pool to named pool selection
- Extract and enforce Session-Timeout/Idle-Timeout
- Extract and apply Filter-Id at session establishment (initial rate)
- Extract and override Acct-Interim-Interval per session

## Data Flow (MANDATORY)

### Entry Point
- RADIUS Access-Accept packet arrives with subscriber profile attributes
- Parsed in `doRADIUS()` goroutine in handler.go

### Transformation Path
1. Access-Accept received, attributes parsed
2. Framed-IP-Address extracted -> stored in auth result metadata
3. Auth handler returns accept with metadata (IP, pool name, timeouts, rate)
4. If Framed-IP-Address present: auth handler pre-populates IP response, pool drain skips allocation
5. If Framed-Pool present: pool drain routes to named pool instead of default
6. Session-Timeout/Idle-Timeout: stored in L2TPSession, timer started in reactor
7. Filter-Id: emitted as initial rate event on session-up (shaper plugin consumes)
8. Acct-Interim-Interval: stored per-session, overrides plugin default in interimLoop

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RADIUS handler -> PPP session | AuthResult metadata -> IPResponse bypass | [ ] |
| RADIUS handler -> pool plugin | Framed-Pool name in session attributes | [ ] |
| RADIUS handler -> reactor | Session-Timeout/Idle-Timeout stored in session | [ ] |
| RADIUS handler -> shaper plugin | SessionUp event carries initial rate from Filter-Id | [ ] |
| RADIUS handler -> acct plugin | Per-session Acct-Interim-Interval override | [ ] |

### Integration Points
- `AuthResult` struct -- needs metadata fields for RADIUS attributes
- `IPResponseArgs` -- may need RADIUS-assigned mode flag
- `L2TPSession` -- needs timeout fields
- `acctSession` -- needs per-session interval override
- Pool plugin -- needs named pool support
- Shaper plugin -- needs initial rate from auth (not just CoA)

### Architectural Verification
- [ ] No bypassed layers (attributes flow through existing handler/drain/event patterns)
- [ ] No unintended coupling (handler stores attributes in session metadata, consumers read via events)
- [ ] No duplicated functionality (reuses existing pool, shaper, acct infrastructure)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| RADIUS Access-Accept with Framed-IP-Address | -> | PPP session gets RADIUS-assigned IP, pool not queried | `TestRADIUSFramedIPBypassesPool` |
| RADIUS Access-Accept with Session-Timeout=300 | -> | Session torn down after 300s | `TestRADIUSSessionTimeout` |
| RADIUS Access-Accept with Filter-Id="10M" | -> | Shaper applies 10M rate at session-up | `TestRADIUSFilterIdInitialRate` |
| RADIUS Access-Accept with Framed-Pool="premium" | -> | Named pool "premium" used for allocation | `TestRADIUSFramedPoolSelection` |
| RADIUS Access-Accept with Acct-Interim-Interval=60 | -> | Interim updates sent every 60s for this session | `TestRADIUSAcctInterimOverride` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Access-Accept with Framed-IP-Address=10.0.0.5 | Session gets 10.0.0.5 assigned; pool not queried for this session |
| AC-2 | Access-Accept with Framed-IP-Address + Framed-IP-Netmask | Netmask applied to pppN interface |
| AC-3 | Access-Accept with Framed-Pool="gold" | Pool plugin uses "gold" pool for IP allocation |
| AC-4 | Access-Accept with Framed-Pool for nonexistent pool | Session rejected with clear error logged |
| AC-5 | Access-Accept with Session-Timeout=600 | Session forcibly disconnected after 600 seconds |
| AC-6 | Access-Accept with Idle-Timeout=120 | Session disconnected after 120s of no traffic |
| AC-7 | Access-Accept with Filter-Id="rate:20M/5M" | Shaper applies 20M down / 5M up at session establishment |
| AC-8 | Access-Accept with Acct-Interim-Interval=60 | Accounting interim updates sent every 60s (not global 300s) |
| AC-9 | Access-Accept with no profile attributes | Behavior unchanged: pool allocates, no timeout, no rate |
| AC-10 | Access-Accept with multiple attributes combined | All attributes processed; session gets IP + timeout + rate |
| AC-11 | Session-Timeout fires | CDN sent, session torn down cleanly, accounting-stop sent |
| AC-12 | Idle-Timeout with active traffic | Timer resets on traffic; session stays up |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtractFramedIP` | `internal/plugins/l2tpauthradius/handler_test.go` | Framed-IP-Address extraction from Access-Accept | |
| `TestExtractFramedPool` | `internal/plugins/l2tpauthradius/handler_test.go` | Framed-Pool string extraction | |
| `TestExtractSessionTimeout` | `internal/plugins/l2tpauthradius/handler_test.go` | Session-Timeout uint32 extraction | |
| `TestExtractIdleTimeout` | `internal/plugins/l2tpauthradius/handler_test.go` | Idle-Timeout uint32 extraction | |
| `TestExtractFilterId` | `internal/plugins/l2tpauthradius/handler_test.go` | Filter-Id string extraction | |
| `TestExtractAcctInterimInterval` | `internal/plugins/l2tpauthradius/handler_test.go` | Acct-Interim-Interval uint32 extraction | |
| `TestRADIUSFramedIPBypassesPool` | `internal/plugins/l2tpauthradius/handler_test.go` | Direct IP assignment skips pool handler | |
| `TestRADIUSSessionTimeout` | `internal/component/l2tp/reactor_test.go` | Session timeout timer fires, triggers CDN | |
| `TestRADIUSIdleTimeoutReset` | `internal/component/l2tp/reactor_test.go` | Idle timer resets on traffic | |
| `TestNamedPoolSelection` | `internal/plugins/l2tppool/pool_test.go` | Named pool routing | |
| `TestAcctInterimOverride` | `internal/plugins/l2tpauthradius/acct_test.go` | Per-session interval override | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Session-Timeout | 0-4294967295 | 4294967295 | N/A (0 = no timeout) | N/A (uint32) |
| Idle-Timeout | 0-4294967295 | 4294967295 | N/A (0 = no timeout) | N/A (uint32) |
| Acct-Interim-Interval | 60-3600 | 3600 | 59 (clamped to 60) | 3601 (clamped to 3600) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `radius-framed-ip` | `test/l2tp/radius-framed-ip.ci` | RADIUS assigns specific IP to subscriber | |
| `radius-session-timeout` | `test/l2tp/radius-session-timeout.ci` | Session auto-disconnects after timeout | |
| `radius-filter-rate` | `test/l2tp/radius-filter-rate.ci` | Initial rate applied from RADIUS | |

## Files to Modify

- `internal/plugins/l2tpauthradius/handler.go` -- remove unsupportedAccessAcceptAttrs rejection; extract and store attributes
- `internal/plugins/l2tpauthradius/acct.go` -- per-session Acct-Interim-Interval override
- `internal/component/l2tp/handler.go` -- extend AuthResult with RADIUS attribute metadata
- `internal/component/l2tp/session.go` -- add timeout fields to L2TPSession
- `internal/component/l2tp/reactor.go` -- per-session timeout timers
- `internal/plugins/l2tppool/pool.go` -- named pool support (map of pools)
- `internal/plugins/l2tpshaper/shaper.go` -- initial rate from session attributes (not just CoA)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema update | [x] | `internal/plugins/l2tppool/schema/ze-l2tp-pool-conf.yang` -- named pools |
| CLI commands/flags | [x] | `show l2tp pool` to show named pools |
| Editor autocomplete | [x] | YANG-driven |
| Functional test | [x] | `test/l2tp/radius-framed-ip.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- RADIUS profile attributes |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- named pools |
| 3 | CLI command added/changed? | [ ] | |
| 4 | API/RPC added/changed? | [ ] | |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` -- update l2tp-auth-radius |
| 6 | Has a user guide page? | [ ] | |
| 7 | Wire format changed? | [ ] | |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [x] | RFC 2865 attribute semantics |
| 10 | Test infrastructure changed? | [ ] | |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- RADIUS attribute support |
| 12 | Internal architecture changed? | [ ] | |

## Files to Create

- `test/l2tp/radius-framed-ip.ci` -- RADIUS-assigned IP functional test
- `test/l2tp/radius-session-timeout.ci` -- session timeout functional test
- `test/l2tp/radius-filter-rate.ci` -- initial rate functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: AuthResult metadata** -- extend AuthResult with attribute fields (FramedIP, FramedPool, SessionTimeout, IdleTimeout, FilterId, AcctInterimInterval); remove unsupportedAccessAcceptAttrs rejection; extract attributes from Access-Accept
   - Tests: `TestExtractFramedIP`, `TestExtractFramedPool`, `TestExtractSessionTimeout`, etc.
   - Files: `handler.go`, `handler.go` (l2tp), auth result types
   - Verify: tests fail -> implement -> tests pass

2. **Phase: RADIUS-assigned IP** -- when Framed-IP-Address is in auth result, bypass pool; deliver IP directly via modified drain/response path
   - Tests: `TestRADIUSFramedIPBypassesPool`
   - Files: `handler.go`, `drain.go`, `ip_events.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Named pools** -- extend l2tppool to support multiple named pools; Framed-Pool attribute routes to named pool
   - Tests: `TestNamedPoolSelection`
   - Files: `pool.go`, YANG schema
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Session/Idle timeout** -- add timeout fields to L2TPSession; start timer in reactor on session-up; CDN on expiry; idle timeout resets on traffic
   - Tests: `TestRADIUSSessionTimeout`, `TestRADIUSIdleTimeoutReset`
   - Files: `session.go`, `reactor.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Initial rate from Filter-Id** -- parse Filter-Id rate format; apply via shaper at session-up (same as CoA but at establishment time)
   - Tests: `TestRADIUSFilterIdInitialRate`
   - Files: `shaper.go`, event payload
   - Verify: tests fail -> implement -> tests pass

6. **Phase: Per-session Acct-Interim-Interval** -- override interimLoop interval from session attribute
   - Tests: `TestAcctInterimOverride`
   - Files: `acct.go`
   - Verify: tests fail -> implement -> tests pass

7. **Functional tests** -> Create after feature works.
8. **Full verification** -> `make ze-verify`
9. **Complete spec** -> Fill audit tables, write learned summary, delete spec.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-12 has implementation with file:line |
| Correctness | Framed-IP-Address is 4 bytes big-endian (RFC 2865 S5.8); Session-Timeout is seconds |
| Naming | Attribute constants in radius/dict.go match RFC numbering |
| Data flow | RADIUS attrs -> AuthResult metadata -> consumers (pool/reactor/shaper/acct) |
| Rule: no-layering | Old rejection code fully removed |
| Rule: goroutine-lifecycle | Timeout timers cancelled on session teardown |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| unsupportedAccessAcceptAttrs removed | `grep -c unsupportedAccessAcceptAttr handler.go` returns 0 |
| Framed-IP-Address consumed | `go test ./internal/plugins/l2tpauthradius/ -run TestExtractFramedIP` |
| Session timeout enforced | `go test ./internal/component/l2tp/ -run TestRADIUSSessionTimeout` |
| Named pools work | `go test ./internal/plugins/l2tppool/ -run TestNamedPoolSelection` |
| `make ze-verify` passes | Run and check exit code |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Framed-IP-Address must be valid unicast; reject multicast/broadcast/loopback |
| Resource exhaustion | Session-Timeout=0 means no timer (not infinite timer spinning) |
| Trust boundary | RADIUS server is trusted for attribute values; validate format, not semantics |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Existing tests break | Attribute handling changed existing behavior; investigate |
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

Add `// RFC 2865 Section 5.8` above Framed-IP-Address handling.
Add `// RFC 2865 Section 5.27` above Session-Timeout enforcement.
Add `// RFC 2865 Section 5.28` above Idle-Timeout enforcement.

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

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/`
- [ ] Summary included in commit
