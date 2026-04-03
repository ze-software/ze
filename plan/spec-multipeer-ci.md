# Spec: multipeer-ci

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/testing/ci-format.md` - .ci test format
4. `internal/test/runner/runner_exec.go` - process orchestration, port injection

## Task

Add multi-peer support to the `.ci` functional test framework so tests can run ze with 2+ BGP peers, each connecting to a separate ze-peer instance on a different port. This unblocks LLGR AC-9 (non-LLGR peer suppression), egress filter tests, route reflection tests, and any future test needing differentiated peer behavior.

### Scope

**In Scope:**

| Area | Description |
|------|-------------|
| Port allocation | Increase from 2 to 4 ports per test |
| Port substitution | Add `$PORT3` and `$PORT4` in runner |
| Port override suppression | New option to prevent `ze_bgp_tcp_port` injection on ze process |
| Documentation | Update `docs/architecture/testing/ci-format.md` with multi-peer pattern |
| Proof-of-concept test | LLGR AC-9 test: non-LLGR peer does not receive LLGR_STALE routes |

**Out of Scope:**

| Area | Reason |
|------|--------|
| Dynamic port count (N ports) | 4 ports covers 3 peers + 1 auxiliary service; extend later if needed |
| Per-peer capability override at runner level | Already possible via per-peer config + `add-capability`/`drop-capability` per ze-peer stdin |
| Multi-peer encode/decode tests | Only plugin tests need multi-peer |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - .ci format, options, port substitution
  -> Constraint: `$PORT` and `$PORT2` already supported. Port allocation is 2 per test.
- [ ] `docs/architecture/core-design.md` - plugin architecture, peer model
  -> Constraint: Each peer identified by remote IP. All test peers use 127.0.0.1 so port differentiates them.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9494.md` - LLGR readvertisement rules (Section 4.5)
  -> Constraint: LLGR_STALE routes SHOULD NOT be advertised to non-LLGR peers

**Key insights:**
- Per-peer `remote > port` already exists in YANG schema (`ze-bgp-conf.yang:215`)
- `ze_bgp_tcp_port` env var overrides ALL peer ports globally -- must be suppressed for multi-peer
- `option=env:var=...` entries appended after auto-injection but OS may use first duplicate
- Runner allocates 2 ports (`rec.Port`, `rec.Port+1`), substitutes as `$PORT` and `$PORT2`
- Multiple `cmd=background` processes already supported concurrently
- Each ze-peer instance reads its own stdin block with its own expectations

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/runner/runner_exec.go` - injects `ze_bgp_tcp_port=$PORT` for ze and ze-peer binaries; `$PORT`/`$PORT2` substitution; background process orchestration
- [ ] `internal/test/runner/record_parse.go` - port allocation: `et.port += 2` per test
- [ ] `internal/test/peer/peer.go` - ze-peer binds to single `--port`, accepts N sequential connections
- [ ] `internal/component/bgp/config/peers.go:435` - `applyPortOverride` reads `ze.bgp.tcp.port`, overrides all peers if set
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang:209-218` - per-peer `remote > port` leaf exists

**Behavior to preserve:**
- All existing single-peer tests continue working unchanged
- `$PORT` and `$PORT2` semantics unchanged
- Default port injection (`ze_bgp_tcp_port`) remains for single-peer tests
- `option=tcp_connections:value=N` still handles sequential reconnections to one peer

**Behavior to change:**
- Port allocation: 2 -> 4 ports per test
- Add `$PORT3` and `$PORT4` substitution
- New option `option=peers:value=multi` suppresses `ze_bgp_tcp_port` injection on the ze process

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- `.ci` file with `option=peers:value=multi` and multiple `cmd=background` ze-peer processes

### Transformation Path
1. Parser reads `.ci` file, allocates 4 ports instead of 2
2. `option=peers:value=multi` sets flag on test record
3. Runner starts multiple ze-peer instances on `$PORT`, `$PORT2`, etc.
4. Runner starts ze WITHOUT `ze_bgp_tcp_port` injection (multi flag set)
5. Ze reads per-peer `remote > port` from config, connects to correct ze-peer instances
6. Each ze-peer validates its own expectations independently

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| .ci file -> Runner | Parser reads option, allocates ports | [ ] |
| Runner -> ze process | Env var injection conditional on multi flag | [ ] |
| Runner -> ze-peer processes | Each started with `--port $PORTn` | [ ] |
| ze config -> peer connections | Per-peer `remote > port` in YANG config | [ ] |

### Integration Points
- `record_parse.go:74` - change `et.port += 2` to `et.port += 4`
- `runner_exec.go:173` - suppress `ze_bgp_tcp_port` when multi flag set (plugin test peer start)
- `runner_exec.go:230` - suppress `ze_bgp_tcp_port` when multi flag set (plugin test client start)
- `runner_exec.go:525-526` - suppress `ze_bgp_tcp_port` when multi flag set (background process start)
- `runner_exec.go:391-392` - add `$PORT3` and `$PORT4` substitution

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| .ci file with `option=peers:value=multi` | -> | Runner port allocation + suppression | `test/plugin/llgr-egress-suppress.ci` |
| Config with per-peer `remote > port` | -> | ze connects to correct ze-peer | `test/plugin/llgr-egress-suppress.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | .ci file with `option=peers:value=multi` | ze process started WITHOUT `ze_bgp_tcp_port` env var |
| AC-2 | .ci file uses `$PORT3` in background command | `$PORT3` expanded to `rec.Port+2` |
| AC-3 | Multi-peer test with 2 ze-peer instances | Both ze-peer instances start, ze connects to both |
| AC-4 | Existing single-peer test (no multi option) | Unchanged behavior, `ze_bgp_tcp_port` still injected |
| AC-5 | LLGR stale route, destination peer lacks LLGR cap | Route NOT forwarded (EBGP: withdrawn; IBGP: deprioritized) |
| AC-6 | LLGR stale route, destination peer has LLGR cap | Route forwarded normally |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPortSubstitution_PORT3` | `internal/test/runner/runner_exec_test.go` | `$PORT3` expanded correctly | |
| `TestMultiPeerOption_SuppressesPortOverride` | `internal/test/runner/runner_exec_test.go` | Multi flag prevents env injection | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Port count | 4 per test | $PORT4 = base+3 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `llgr-egress-suppress` | `test/plugin/llgr-egress-suppress.ci` | Peer A in LLGR, Peer B (non-LLGR) does not receive stale routes | |

### Future (if deferring any tests)
- Multi-peer route reflection test (needs route reflector config, separate spec)
- Multi-peer best-path test (needs 2 sources for same prefix)

## Files to Modify

- `internal/test/runner/record_parse.go` - port allocation: 2 -> 4
- `internal/test/runner/runner_exec.go` - `$PORT3`/`$PORT4` substitution, multi flag suppresses port override
- `docs/architecture/testing/ci-format.md` - document multi-peer pattern and new option

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | Yes | `test/plugin/llgr-egress-suppress.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | N/A |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/ci-format.md` -- multi-peer section |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | N/A |

## Files to Create

- `test/plugin/llgr-egress-suppress.ci` - multi-peer LLGR egress filter test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Phases below |
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

1. **Phase: Port allocation and substitution** -- extend runner for 4 ports
   - Tests: `TestPortSubstitution_PORT3`
   - Files: `record_parse.go`, `runner_exec.go`
   - Verify: `$PORT3` and `$PORT4` expand to correct values

2. **Phase: Multi-peer option** -- `option=peers:value=multi` suppresses port override
   - Tests: `TestMultiPeerOption_SuppressesPortOverride`
   - Files: `runner_exec.go`, `record_parse.go`
   - Verify: ze started without `ze_bgp_tcp_port` when multi flag set

3. **Phase: LLGR egress suppress test** -- proof-of-concept multi-peer test
   - Tests: `test/plugin/llgr-egress-suppress.ci`
   - Files: new .ci file
   - Verify: test passes, stale routes suppressed to non-LLGR peer

4. **Documentation** -- update ci-format.md
   - Files: `docs/architecture/testing/ci-format.md`

5. **Full verification** -- `make ze-verify`

6. **Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 6 ACs implemented with evidence |
| Correctness | Existing single-peer tests still pass (no regression) |
| Naming | Option name consistent with existing option patterns |
| Data flow | Port substitution covers all cmd types (background, foreground, stdin) |
| Rule: no-layering | No duplicate port allocation paths |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `$PORT3` and `$PORT4` work | grep in runner_exec.go |
| Multi-peer option parsed | grep for `peers` in record_parse.go |
| Port override suppressed | unit test |
| LLGR egress suppress test passes | `ze-test bgp plugin llgr-egress-suppress` |
| Existing tests unchanged | `make ze-functional-test` full pass |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Port allocation | No port conflicts between concurrent tests |
| Process cleanup | All background ze-peer instances killed on test end |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Port conflict in concurrent tests | Increase port stride or use dynamic allocation |
| ze ignores per-peer port | Trace config resolution, check `applyPortOverride` |
| ze-peer expectations mismatch | Check correct stdin block routed to correct instance |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Multi-Peer Test Pattern (reference)

This is the pattern the `.ci` test will follow:

**ze-peer A** (LLGR-capable, port `$PORT`): peer goes down, routes become stale, reconnects.

**ze-peer B** (non-LLGR, port `$PORT3`): second peer that should NOT receive LLGR_STALE routes.

**ze config**: two peers, each with explicit `remote > port`, LLGR on peer A only. `option=peers:value=multi` prevents global port override.

**Flow:**
1. Ze connects to both peers
2. Peer A sends routes to ze, then TCP close
3. GR timer expires, LLGR begins for peer A's routes
4. LLGR_STALE routes should NOT be forwarded to peer B (EBGP non-LLGR)
5. Ze reconnects to peer A, stale routes re-sent (LLGR-capable)

## Cross-References

| Document | Relevance |
|----------|-----------|
| `plan/learned/511-llgr-0-umbrella.md` | AC-9 partial: this spec completes it |
| `plan/learned/509-llgr-4-readvertisement.md` | Noted multi-peer gap |

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Port stride increase breaks concurrent tests | Low | Medium | Port allocation already handles conflicts via `FindFreePortRange` |
| Multi-peer timing sensitivity | Medium | Low | Use connect-retry timers and action=close for determinism |
| Existing tests regressed by port change | Low | High | All existing tests use 2 ports max, stride increase is safe |

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
- (to be filled after implementation)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan

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
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-verify` passes
- [ ] Existing tests not regressed
- [ ] Integration completeness proven end-to-end

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Documentation updated

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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-multipeer-ci.md`
- [ ] Summary included in commit
