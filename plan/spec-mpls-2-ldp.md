# Spec: mpls-2-ldp

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-mpls-1-kernel |
| Phase | 2 of 3 (MPLS) |
| Updated | 2026-05-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugins/fib/kernel/fibkernel.go` - kernel FIB backend (MPLS from phase 1)
4. `internal/component/bgp/reactor/` - BGP FSM as reference for protocol engine pattern

## Task

Implement the Label Distribution Protocol (LDP) as a Ze component. LDP
distributes MPLS labels between label-switching routers for FEC-to-label
bindings. This makes Ze a full MPLS router that can participate in an MPLS
network without relying on BGP labeled unicast for label distribution.

LDP has two phases: discovery (UDP multicast/unicast hello) and session
(TCP, label mapping/request/release/withdraw). Ze implements downstream
unsolicited mode (the common deployment model).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component registration pattern
- [ ] `internal/component/bgp/reactor/` - reference FSM for protocol engines

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5036.md` - LDP Specification (MUST CREATE)
  → Constraint: discovery hello interval, session keepalive, FEC-label binding semantics
  → Constraint: downstream unsolicited vs on-demand, liberal vs conservative retention
- [ ] `rfc/short/rfc5561.md` - LDP Capabilities (MUST CREATE)
  → Constraint: capability negotiation TLV format
- [ ] `rfc/short/rfc3031.md` - MPLS Architecture (from phase 1)
  → Constraint: LSR model, FEC types (prefix, host)
- [ ] `rfc/short/rfc7552.md` - Updates to LDP for IPv6 (SHOULD CREATE)
  → Constraint: dual-stack LDP, IPv6 FEC elements

**Key insights:**
- LDP discovery: UDP port 646, multicast 224.0.0.2 (all routers), hello interval 5s, hold time 15s
- LDP session: TCP port 646, keepalive, ordered label distribution
- FEC types: prefix (most common), host address
- Label space: per-platform (one label space for entire LSR) is the common model
- Loop detection: optional, via path vector or hop count

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] Ze has no LDP implementation today
- [ ] MPLS labels currently come only from BGP labeled unicast
- [ ] Kernel FIB backend (after phase 1) can program MPLS routes

**Behavior to preserve:**
- BGP labeled unicast path remains independent and functional
- Kernel MPLS FIB programming from phase 1 unchanged

**Behavior to change:**
- No LDP support exists; this is entirely new
- Sysrib gains a second label source (LDP alongside BGP)

## Data Flow (MANDATORY)

### Entry Point
- UDP: LDP Hello messages from neighbors (discovery)
- TCP: LDP session establishment, label mapping messages

### Transformation Path
1. **Discovery:** UDP listener receives Hello, builds neighbor adjacency table
2. **Session:** TCP connection to neighbor, exchange Initialization messages, negotiate capabilities
3. **Label mapping:** receive Label Mapping for FEC (prefix), store in LDP label database (LIB)
4. **FIB programming:** LDP publishes label bindings to sysrib; sysrib emits best-change with labels
5. **Kernel FIB:** fibkernel programs MPLS swap/push/pop via netlink (from phase 1)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> LDP engine | UDP/TCP listeners | [ ] |
| LDP engine -> sysrib | EventBus: label binding events | [ ] |
| sysrib -> fibkernel | Existing BestChange with Labels | [ ] |

### Integration Points
- New component: `internal/component/ldp/` (FSM, wire, discovery, session)
- Config: YANG schema under `protocol { ldp { ... } }`
- Sysrib: new label source type alongside BGP
- CLI: `show ldp neighbor`, `show ldp binding`, `show ldp interface`
- Web: LDP neighbor and binding views
- Metrics: Prometheus counters for sessions, bindings, messages

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| config `protocol { ldp { ... } }` | -> | LDP component startup | `TestLDPComponentStart` |
| UDP Hello received | -> | adjacency creation | `TestLDPDiscovery` |
| TCP session established | -> | label mapping exchange | `TestLDPSession` |
| label binding received | -> | sysrib best-change with labels | `TestLDPToSysrib` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | LDP configured on interface | UDP Hello sent on 224.0.0.2:646, adjacency formed |
| AC-2 | Hello received from neighbor | TCP session initiated to neighbor, Initialization exchanged |
| AC-3 | Session established | Label Mapping messages sent for local FECs (connected prefixes) |
| AC-4 | Label Mapping received | Label stored in LIB, forwarding entry programmed in kernel |
| AC-5 | Neighbor goes down (hold timer) | Session torn down, labels withdrawn, MPLS routes removed |
| AC-6 | Keepalive timeout | Session reset, label bindings cleared |
| AC-7 | `show ldp neighbor` | Display LDP neighbors with session state, transport address |
| AC-8 | `show ldp binding` | Display FEC-label bindings (local and remote) |
| AC-9 | Config reload adds/removes LDP interface | Discovery starts/stops on interface, sessions adjusted |
| AC-10 | Interop with FRR ldpd | Session established, labels exchanged, traffic forwarded |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLDPHelloEncode` | `internal/component/ldp/wire_test.go` | Hello message encoding | |
| `TestLDPHelloDecode` | `internal/component/ldp/wire_test.go` | Hello message decoding | |
| `TestLDPInitEncode` | `internal/component/ldp/wire_test.go` | Initialization message encoding | |
| `TestLDPLabelMapEncode` | `internal/component/ldp/wire_test.go` | Label Mapping message encoding | |
| `TestLDPSessionFSM` | `internal/component/ldp/session_test.go` | State transitions | |
| `TestLDPDiscoveryHoldTimer` | `internal/component/ldp/discovery_test.go` | Adjacency timeout | |
| `TestLDPLIBInsert` | `internal/component/ldp/lib_test.go` | Label Information Base operations | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Label | 0-1048575 | 1048575 | N/A | 1048576 |
| Hello hold time | 1-65535 | 65535 | 0 | 65536 |
| Keepalive time | 1-65535 | 65535 | 0 | 65536 |
| LDP identifier | 6 bytes | N/A | < 6 bytes | > 6 bytes |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ldp-session` | `test/ldp/ldp-session.ci` | LDP session establishment and label exchange | |
| `ldp-convergence` | `test/ldp/ldp-convergence.ci` | Label withdrawal on neighbor loss | |
| `ldp-reload` | `test/ldp/ldp-reload.ci` | Config reload adds/removes LDP interface | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ldp-session-frr` | `test/interop/scenarios/` | FRR ldpd | Session establishment, label exchange | |
| `ldp-convergence-frr` | `test/interop/scenarios/` | FRR ldpd | Label withdrawal on link down | |

### Future (if deferring any tests)
- None planned

## Files to Modify
- `internal/plugins/sysrib/sysrib.go` - accept label bindings from LDP source
- `internal/plugins/sysrib/events/events.go` - add Protocol field if needed for LDP source
- `cmd/ze/hub/main.go` - wire LDP component startup

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/yang/modules/ze-ldp.yang` |
| CLI commands/flags | [x] | `show ldp neighbor`, `show ldp binding`, `show ldp interface` |
| CLI grammar (action before identifier) | [x] | `.claude/rules/cli-grammar.md` |
| Editor autocomplete | [x] | YANG-driven |
| Functional test for new RPC/API | [x] | `test/ldp/*.ci` |
| Doctor check for runtime dependencies | [x] | port 646 UDP/TCP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | |
| 6 | Has a user guide page? | [x] | `docs/guide/ldp.md` |
| 7 | Wire format changed? | [ ] | |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc5036.md` |
| 10 | Test infrastructure changed? | [ ] | |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` (new component) |

## Files to Create
- `internal/component/ldp/` - entire LDP component (wire, discovery, session, lib, engine)
- `internal/yang/modules/ze-ldp.yang` - LDP config schema
- `test/ldp/*.ci` - functional tests
- `rfc/short/rfc5036.md` - LDP Specification summary
- `rfc/short/rfc5561.md` - LDP Capabilities summary

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register LDP component, write failing wiring tests
   - Tests: `TestLDPComponentStart`
   - Files: `internal/component/ldp/register.go`
   - Verify: component registers but discovery/session are stubs
2. **Phase: Wire codec** -- LDP message types
   - Tests: `TestLDPHelloEncode`, `TestLDPHelloDecode`, `TestLDPInitEncode`, `TestLDPLabelMapEncode`
   - Files: `internal/component/ldp/wire.go`
   - Verify: round-trip encode/decode for all message types
3. **Phase: Discovery** -- UDP listener, Hello send/receive, adjacency table
   - Tests: `TestLDPDiscoveryHoldTimer`, `TestLDPDiscovery`
   - Files: `internal/component/ldp/discovery.go`
   - Verify: adjacency formed and expired on timeout
4. **Phase: Session FSM** -- TCP connection, Initialization, keepalive
   - Tests: `TestLDPSessionFSM`, `TestLDPSession`
   - Files: `internal/component/ldp/session.go`
   - Verify: session state transitions match RFC 5036
5. **Phase: Label database** -- LIB, downstream unsolicited distribution
   - Tests: `TestLDPLIBInsert`, `TestLDPToSysrib`
   - Files: `internal/component/ldp/lib.go`
   - Verify: label bindings stored and published to sysrib
6. **Phase: Config** -- YANG schema, config parsing
   - Files: `ze-ldp.yang`, config parser
   - Verify: config accepted and drives discovery
7. **Phase: CLI/Web** -- show commands, web views
   - Files: CLI handlers, web page
   - Verify: `show ldp neighbor` displays neighbors
8. **Phase: Metrics** -- Prometheus counters
9. **Phase: Interop** -- FRR ldpd test scenarios
10. **Full verification** -- `make ze-verify`
11. **Complete spec** -- learned summary, delete spec

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Wire format matches RFC 5036 exactly |
| Naming | YANG uses kebab-case, CLI uses `show ldp <noun>` |
| Data flow | Labels flow LDP -> sysrib -> fibkernel, no bypass |
| CLI grammar | All commands follow action-before-identifier |
| Doctor checks | Port 646 availability check registered |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| LDP component directory | `ls internal/component/ldp/` |
| YANG module | `ls internal/yang/modules/ze-ldp.yang` |
| Functional tests | `ls test/ldp/*.ci` |
| RFC summaries | `ls rfc/short/rfc5036.md rfc/short/rfc5561.md` |
| Interop test | `ls test/interop/scenarios/ldp-session-frr/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | All TLV lengths validated before parsing |
| Authentication | MD5 TCP authentication (RFC 5036 Section 2.9) |
| Resource exhaustion | Maximum neighbor/session limits |
| Privilege | UDP/TCP port 646 requires CAP_NET_BIND_SERVICE or root |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
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

Add `// RFC 5036 Section X.Y: "<quoted requirement>"` above enforcing code.

## Implementation Summary

### What Was Implemented
- [To be filled]

### Bugs Found/Fixed
- [To be filled]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

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

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| LDP session with neighbor | functional test | `ldp-session.ci` |
| Label distribution | functional test | `ldp-convergence.ci` |
| FRR interop | interop test | `ldp-session-frr` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [To be filled]

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
- [ ] AC-1..AC-10 all demonstrated
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
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-mpls-ldp.md`
- [ ] Summary included in commit
