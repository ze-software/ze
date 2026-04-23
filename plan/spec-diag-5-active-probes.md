# Spec: diag-5-active-probes -- Active Network Probes

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/2 |
| Updated | 2026-04-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `internal/component/cmd/show/ip.go` -- existing `show ip route` implementation
3. `internal/component/iface/` -- ListKernelRoutes, backend pattern
4. Parent: `plan/spec-diag-0-umbrella.md`

## Task

Add active network probes so Claude can validate forwarding paths from the
router's perspective. Two capabilities:

1. **Ping** -- ICMP echo-request/echo-reply from the router, reporting RTT and loss
2. **Route lookup** -- longest-prefix-match query: given a destination IP, which route matches?

Deferred to future specs:
- Traceroute (complex: TTL decrement + ICMP Time Exceeded parsing)
- L2TP echo probe (requires kernel PPP LCP echo interaction)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- registration pattern
  → Constraint: new commands register via YANG RPC + handler in init()
- [ ] `ai/patterns/cli-command.md` -- CLI command pattern
  → Constraint: YANG RPC + ze:command augment

### RFC Summaries
- [ ] Not protocol work (ICMP is OS-level, not BGP/L2TP protocol).

**Key insights:**
- `golang.org/x/net` v0.52.0 already in go.mod; `x/net/icmp` package available
- No raw socket or ICMP code exists anywhere in the codebase
- `show ip route` exists (ip.go:120) with kernel FIB dump via `iface.ListKernelRoutes(filter, limit)`, but no LPM lookup
- Ze runs as root on gokrazy (CAP_NET_RAW available for ICMP sockets)
- Platform backends split via `_linux.go` / `_other.go` build tags

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cmd/show/ip.go` (180 LOC) -- `handleShowIPRoute` dumps kernel routes filtered by exact prefix. Uses `iface.ListKernelRoutes()`.
  → Constraint: existing `show ip route <cidr>` is exact-prefix filter, not LPM. Route lookup is a new command.
- [ ] `internal/component/iface/` -- `ListKernelRoutes(filter, limit)` returns `[]KernelRoute`. Backend dispatches to netlink on linux.
  → Constraint: no `RouteGet(destination)` method exists. Need netlink `RouteGet` for LPM.

**Behavior to preserve:**
- Existing `show ip route` unchanged
- No new dependencies beyond `x/net/icmp` (already in go.mod transitively)

**Behavior to change:**
- Add `ping <destination>` command
- Add `show ip route lookup <destination>` command

## Data Flow (MANDATORY)

### Entry Point (Ping)

1. Operator types `ping <destination> [count N] [timeout Ns]` or Claude sends MCP tool
2. Handler opens privileged ICMP socket via `icmp.ListenPacket`
3. Sends ICMP Echo Request, waits for Echo Reply with timeout
4. Reports RTT per packet and summary (sent/received/loss%)

### Entry Point (Route Lookup)

1. Operator types `show ip route lookup <destination>`
2. Handler calls netlink `RouteGet(destination)` on linux
3. Returns matching route: prefix, next-hop, interface, protocol, metric

### Transformation Path

1. Command dispatch resolves YANG RPC
2. Handler validates destination (must be valid IP address)
3. Platform-specific operation (ICMP socket / netlink RouteGet)
4. Result wrapped in `plugin.Response` JSON

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| CLI/MCP ↔ Command dispatch | YANG RPC | [ ] (existing) |
| Handler ↔ OS kernel | ICMP raw socket / netlink | [ ] |

### Integration Points

- `internal/component/cmd/show/ip.go` -- add route lookup handler
- New `internal/component/cmd/show/ping.go` -- ping handler
- `internal/component/iface/` -- add RouteGet backend method (linux only)

### Architectural Verification

- [ ] No bypassed layers (commands go through dispatch)
- [ ] No unintended coupling (ping is self-contained, route lookup uses iface backend)
- [ ] Platform-safe (non-linux returns "not available")

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ping <dest>` | → | ICMP echo handler | `TestPingLoopback` |
| `show ip route lookup <dest>` | → | netlink RouteGet | `TestRouteLookupLoopback` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ping 127.0.0.1` | JSON: sent, received, loss-percent, min/avg/max RTT |
| AC-2 | `ping 127.0.0.1 count 3` | Sends exactly 3 packets |
| AC-3 | `ping 192.0.2.1` (unreachable) | JSON: sent=N, received=0, loss-percent=100 |
| AC-4 | `ping invalid` | Error: invalid destination address |
| AC-5 | `show ip route lookup 8.8.8.8` on linux | JSON: matching prefix, next-hop, interface, protocol |
| AC-6 | `show ip route lookup 8.8.8.8` on darwin | Error: "route lookup not available on this platform" |
| AC-7 | `show ip route lookup invalid` | Error: invalid destination address |
| AC-8 | Both commands visible in MCP tools/list | Auto-generated from YANG RPCs |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPingParseArgs` | `internal/component/cmd/show/ping_test.go` | AC-2, AC-4: arg parsing |  |
| `TestPingLoopback` | `internal/component/cmd/show/ping_test.go` | AC-1: loopback ping works |  |
| `TestRouteLookupParseArgs` | `internal/component/cmd/show/ip_test.go` | AC-7: validates destination |  |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ping count | 1 - 100 | 100 | 0 | 101 |
| ping timeout | 1s - 30s | 30s | 0s | 31s |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-ping-loopback` | `test/plugin/ping-loopback.ci` | Operator pings 127.0.0.1 and gets RTT | |

### Future
- Traceroute (deferred)
- L2TP echo probe (deferred)
- Route lookup functional test (requires linux with routes installed)

## Files to Modify

- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` -- add ping and route lookup containers
- `internal/component/cmd/show/schema/ze-cli-show-api.yang` -- add ping and route-lookup RPCs
- `internal/component/cmd/show/show.go` -- register new handlers

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 3 | CLI command added? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added? | Yes | `docs/architecture/api/commands.md` |

## Files to Create

- `internal/component/cmd/show/ping.go` -- ping handler
- `internal/component/cmd/show/ping_test.go` -- ping tests
- `internal/component/cmd/show/route_lookup.go` -- route lookup handler (linux)
- `internal/component/cmd/show/route_lookup_other.go` -- platform stub

## Implementation Steps

### Implementation Phases

1. **Phase: Ping** -- ICMP echo via x/net/icmp
   - Tests: `TestPingParseArgs`, `TestPingLoopback`
   - Files: `ping.go`, YANG schemas
   - Verify: tests pass, `ping 127.0.0.1` works

2. **Phase: Route Lookup** -- netlink RouteGet for LPM
   - Tests: `TestRouteLookupParseArgs`
   - Files: `route_lookup.go`, `route_lookup_other.go`, YANG
   - Verify: `show ip route lookup 127.0.0.1` returns loopback route

### Critical Review Checklist

| Check | What to verify |
|-------|---------------|
| Completeness | Every AC has implementation |
| Correctness | ICMP socket closed after use; timeout enforced; invalid IPs rejected |
| Platform safety | Non-linux returns error, not panic |
| Security | Destination IP validated; no command injection; socket closed on error |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Ping works | `ping 127.0.0.1` returns RTT data |
| Route lookup works on linux | `show ip route lookup 127.0.0.1` returns route |
| Platform stub works | Non-linux returns "not available" |
| MCP auto-generation | Commands appear in tools/list |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | Destination must be valid IP (netip.ParseAddr); no hostnames |
| Resource exhaustion | Ping count capped at 100; timeout capped at 30s |
| Socket lifecycle | ICMP socket opened per-request, closed via defer |
| Privilege | Requires CAP_NET_RAW; fails gracefully if unprivileged |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in phase |
| ICMP permission denied | Return clear error about CAP_NET_RAW |
| 3 fix attempts fail | STOP. Ask user. |

## Design Alternatives

### Ping: x/net/icmp (CHOSEN)
Direct ICMP via Go's `x/net/icmp` package. Opens raw socket, sends Echo Request, reads Echo Reply.
**Gains:** No external dependency. Full control over count/timeout/source. JSON output.
**Costs:** Requires CAP_NET_RAW (ze runs as root on gokrazy).

### Ping: shell out to system ping (REJECTED)
**Rejected:** Command injection risk, output parsing fragility, no structured JSON.

### Route lookup: netlink RouteGet (CHOSEN)
Linux `ip route get <dest>` equivalent via vishvananda/netlink.
**Gains:** Kernel-authoritative answer. Already have netlink dependency.
**Costs:** Linux only (acceptable for production router).

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] `make ze-test` passes
- [ ] Feature code integrated

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Write learned summary to `plan/learned/654-diag-5-active-probes.md`

## Implementation Summary

### What Was Implemented
- [To be filled]

## Implementation Audit

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Audit Summary
- **Total items:**
- **Done:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
