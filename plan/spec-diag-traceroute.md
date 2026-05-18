# Spec: diag-traceroute

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/4 |
| Updated | 2026-05-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/cmd/show/ping.go` - reference raw socket handler pattern (ICMP)
4. `internal/component/cmd/show/show.go` - RPC registration hub

## Task

Add `show traceroute` command that replaces `traceroute` on gokrazy appliances. Pure Go implementation using a third-party library. Returns JSON with per-hop address, RTT, and TTL.

Related specs:
- `spec-diag-core` - core diagnostic commands (root privilege check)
- `spec-diag-netlink-monitor` - streaming netlink monitor
- `spec-diag-capture-interface` - interface packet capture

### Third-Party Library Evaluation

| Library | Stars | Approach | cgo? | IPv6? | Notes |
|---------|-------|----------|------|-------|-------|
| `pixelbender/go-traceroute` | 41 | ICMP + UDP | No | Yes | Best fit: concurrent probes, IPv4+IPv6, only depends on `golang.org/x/net`. Last push 2023-07. Small, stable. |
| `aeden/traceroute` | 193 | UDP (raw) | No | No | Simpler, more popular, but IPv4 only and stale (2021). |
| `vaegt/go-traceroute` | 8 | ICMP + UDP | No | Yes | Wraps `pl0th/go-traceroute`, adds IPv6. Most recent (2024-05). |
| `mgranderath/traceroute` | 15 | UDP + TCP | **Yes** | - | Disqualified: requires gopacket/libpcap. |

**Recommendation:** `pixelbender/go-traceroute`. Evaluate during implementation; if the API is too limiting, vendor-fork the ~200 LOC and adapt. All require `CAP_NET_RAW` (same as existing ping command).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall system architecture
  -> Constraint: Components register via init(), discovered through registries
- [ ] `ai/patterns/cli-command.md` - CLI command registration pattern
  -> Constraint: WireMethod format ze-<verb>:<noun>, YANG tree required
- [ ] `internal/component/cmd/show/ping.go` - ICMP raw socket pattern
  -> Decision: Uses buildICMPEcho/icmpChecksum, raw socket with deadline

### RFC Summaries (MUST for protocol work)
- Not protocol work; traceroute is a diagnostic technique, not a protocol.

**Key insights:**
- Handler pattern: `func(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error)`
- ping.go shows the raw socket pattern (ICMP echo, checksum, deadline handling)
- Traceroute sends probes with incrementing TTL, collects ICMP Time Exceeded replies
- All candidates require CAP_NET_RAW; ze enforces root at startup (diag-core)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cmd/show/show.go` - RPC registration hub
- [ ] `internal/component/cmd/show/ping.go` - ICMP ping handler (reference for raw socket pattern)
- [ ] `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - YANG CLI tree

**Behavior to preserve:**
- All existing show commands unchanged
- JSON output format with `plugin.Response{Status, Data}` pattern
- ping.go raw socket handling (reference, not modified)

**Behavior to change:**
- None. Purely additive: one new command.

## Data Flow (MANDATORY)

### Entry Point
- CLI: user types `show traceroute 8.8.8.8` via SSH
- MCP: AI calls `ze_show_traceroute { target: "8.8.8.8" }` via JSON-RPC
- Web: POST /api/commands with `{"command": "show traceroute 8.8.8.8"}`

### Transformation Path
1. CLI/MCP/Web input -> command dispatcher -> `handleTraceroute`
2. Handler resolves target hostname (if needed) via net.Resolver
3. Handler sends ICMP/UDP probes with incrementing TTL (1..max-hops)
4. Handler collects ICMP Time Exceeded and Destination Unreachable replies
5. Handler builds per-hop result: `[{hop: 1, addr: "...", rtt-ms: N}, ...]`
6. Handler returns `plugin.Response{Status: StatusDone, Data: result}`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> handler | RPC dispatch via WireMethod | [ ] |
| Handler -> network | ICMP/UDP raw sockets (CAP_NET_RAW) | [ ] |
| Handler -> response | plugin.Response JSON | [ ] |

### Integration Points
- `pluginserver.RegisterRPCs()` - command registration (existing)
- `ze-cli-show-cmd.yang` - YANG CLI tree (existing, modified)
- Third-party library or vendor-fork for traceroute logic

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (reuses raw socket patterns from ping.go where applicable)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `show traceroute` | -> | `handleTraceroute` | `TestTraceroute_Wiring` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show traceroute 8.8.8.8` | Returns JSON array with per-hop: hop (int), addr (string), rtt-ms (float), ttl (int) |
| AC-2 | `show traceroute 8.8.8.8 max-hops 10` | Limits probe TTL to 10 |
| AC-3 | `show traceroute 8.8.8.8 timeout 2s` | Per-probe timeout of 2 seconds |
| AC-4 | `show traceroute 8.8.8.8 probes 1` | Sends 1 probe per hop instead of default 3 |
| AC-5 | Hop with no response | Entry shows addr: "*", rtt-ms: null |
| AC-6 | IPv6 target | Works with IPv6 addresses / AAAA-resolved hostnames |
| AC-7 | Hostname target | Resolves hostname to IP before tracing |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTracerouteArgsParser` | `cmd/show/traceroute_test.go` | Argument parsing (target, max-hops, timeout, probes) | |
| `TestTracerouteHopResult` | `cmd/show/traceroute_test.go` | Per-hop result formatting (addr, rtt-ms, timeout star) | |
| `TestTracerouteIPv6` | `cmd/show/traceroute_test.go` | IPv6 address detection and handling | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| max-hops | 1-64 | 64 | 0 | 65 |
| timeout | 1s-30s | 30s | <1s | >30s |
| probes | 1-10 | 10 | 0 | 11 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-traceroute` | `test/plugin/show-traceroute.ci` | CLI traceroute to localhost returns single hop | |

### Future (if deferring any tests)
- None. All tests must be implemented.

## Files to Modify
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - add traceroute container

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` |
| CLI commands/flags | [x] | New handler file (auto-registered via init()) |
| Editor autocomplete | [x] | YANG-driven (automatic) |
| Functional test for new RPC/API | [x] | `test/plugin/show-traceroute.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - add to diagnostics section |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` - add traceroute |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/production-diagnostics.md` (update relevant categories) |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create
- `internal/component/cmd/show/traceroute.go` - traceroute handler (cross-platform via library)
- `internal/component/cmd/show/traceroute_test.go` - unit tests
- `test/plugin/show-traceroute.ci` - functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring** -- register traceroute handler, write failing wiring test
   - Tests: `TestTraceroute_Wiring`
   - Files: handler skeleton, YANG update
   - Verify: wiring test fails because handler is a stub

2. **Phase: Library evaluation** -- add `pixelbender/go-traceroute` dependency, verify API
   - If API is insufficient, vendor-fork (~200 LOC) and adapt
   - Verify: library compiles, basic API call works

3. **Phase: Handler implementation** -- argument parsing, library integration, result formatting
   - Tests: `TestTracerouteArgsParser`, `TestTracerouteHopResult`, `TestTracerouteIPv6`
   - Files: `traceroute.go`, `traceroute_test.go`
   - Verify: `show traceroute 127.0.0.1` returns single hop

4. **Phase: Functional test**
   - Files: `test/plugin/show-traceroute.ci`
   - Verify: `make ze-functional-test` passes

5. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | JSON output per-hop fields match AC-1 spec |
| Naming | YANG uses kebab-case, WireMethod uses ze-show: prefix |
| Data flow | Handler returns via plugin.Response, no direct stdout |
| Rule: dependency | Third-party library is pure Go, no cgo |
| Rule: timeout | Per-probe timeout and total hop limit enforced |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Traceroute command registered | `grep "traceroute" show.go` or `grep WireMethod traceroute.go` |
| YANG container | `grep "traceroute" ze-cli-show-cmd.yang` |
| Functional test | `ls test/plugin/show-traceroute.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | max-hops capped at 64, timeout capped at 30s, probes capped at 10 |
| Target validation | Only IP addresses and resolvable hostnames accepted |
| Resource exhaustion | Bounded by max-hops * probes * timeout |
| Privilege | Requires CAP_NET_RAW (enforced by root check in diag-core) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Library API insufficient | Vendor-fork and adapt |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## RFC Documentation
Not protocol work. No RFC annotations needed.

## Implementation Summary

### What Was Implemented
- [To be filled during implementation]

### Bugs Found/Fixed
- [To be filled during implementation]

### Documentation Updates
- [To be filled during implementation]

### Deviations from Plan
- [To be filled during implementation]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
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
- [ ] Write learned summary to `plan/learned/NNN-diag-traceroute.md`
- [ ] Summary included in commit
