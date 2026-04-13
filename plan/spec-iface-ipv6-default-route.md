# Spec: iface-ipv6-default-route -- IPv6 Default Route from RA with Configurable Metric

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-iface-route-priority |
| Phase | - |
| Updated | 2026-04-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/ifacenetlink/monitor_linux.go` - netlink monitor event loop
4. `internal/component/iface/register.go` - link failover handlers
5. `internal/plugins/sysctl/backend_linux.go` - per-interface sysctl writes
6. `vendor/github.com/vishvananda/netlink/neigh_linux.go` - NeighSubscribe, NTF_ROUTER

## Task

IPv6 default routes come from Router Advertisements (RA), which the Linux
kernel installs automatically via `accept_ra_defrtr`. Ze has no control
over the metric of these kernel-installed routes, so `route-priority`
(spec-iface-route-priority) only works for IPv4 DHCP routes today.

This spec adds IPv6 default route management so `route-priority` works
for both address families. The approach: suppress the kernel's automatic
default route installation via `accept_ra_defrtr=0`, detect routers via
netlink neighbor events (`NTF_ROUTER` flag), and install the default
route with ze's configured metric.

### Context

- IPv4: DHCP provides the gateway, ze calls `AddRoute("0.0.0.0/0", gw, metric)`. Done.
- IPv6: The gateway comes from Router Advertisements (ICMPv6 type 134,
  RFC 4861). The kernel processes RAs and installs the default route
  with metric 0. Ze never sees it.
- The sysctl `accept_ra_defrtr` (per-interface) controls whether the
  kernel installs default routes from RAs. Setting it to 0 suppresses
  the route without disabling RA processing (SLAAC, prefix info, RDNSS
  all continue to work).
- vishvananda/netlink provides `NeighSubscribe` for NDP neighbor events
  and `NTF_ROUTER = 0x80` flag to identify routers.
- The existing netlink monitor (`ifacenetlink/monitor_linux.go`) already
  subscribes to `LinkUpdate` and `AddrUpdate`. Adding `NeighSubscribe`
  extends the same pattern.

### Scope

**In scope:**

| Area | Description |
|------|-------------|
| Sysctl management | Set `accept_ra_defrtr=0` when `route-priority` is configured on a unit with IPv6 |
| Netlink neighbor monitoring | Subscribe to neighbor events, filter for `NTF_ROUTER` flag |
| Default route installation | `AddRoute("::/0", "fe80::router%iface", metric)` with configured route-priority |
| Router lifetime tracking | Remove default route when router disappears (NUD_FAILED/deleted) |
| Link failover for IPv6 | Extend `handleLinkDown`/`handleLinkUp` to handle `::/0` routes |
| Multiple routers | Support multiple RA sources on the same link (multiple default routes with same metric) |

**Out of scope:**

| Area | Reason |
|------|--------|
| RA packet parsing | Kernel handles RA processing; ze only reacts to neighbor table changes |
| SLAAC address management | Kernel handles autoconfiguration independently of `accept_ra_defrtr` |
| RDNSS from RA | Separate concern, kernel/systemd-resolved handles it |
| DHCPv6 default routes | DHCPv6 protocol does not provide default routes (by design) |
| Router preference (RFC 4191) | RA carries router preference (high/medium/low); could be a future enhancement |

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` - DHCP and monitoring capability tables
  -> Constraint: route-priority already documented for IPv4; IPv6 extends the same leaf
- [ ] `plan/learned/582-iface-route-priority.md` - route-priority decisions
  -> Constraint: metric plumbing already exists, this spec adds IPv6 entry point

### RFC Summaries (MUST for protocol work)
- [ ] RFC 4861 - Neighbor Discovery for IPv6
  -> Constraint: Router Advertisement format, router lifetime, NUD states
- [ ] RFC 4862 - IPv6 Stateless Address Autoconfiguration
  -> Constraint: SLAAC must continue working when accept_ra_defrtr=0

**Key insights:**
- `accept_ra_defrtr=0` only suppresses default route installation, not other RA processing
- NTF_ROUTER flag is set by the kernel when it receives an RA from a neighbor
- Router lifetime in RA determines how long the default route should persist
- Multiple routers can exist on the same link (multiple default routes)
- Link-local addresses (fe80::) are used as next-hop for IPv6 default routes

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ifacenetlink/monitor_linux.go` - subscribes to LinkUpdate and AddrUpdate only
- [ ] `internal/component/iface/register.go` - handleLinkDown/handleLinkUp are IPv4-only ("0.0.0.0/0")
- [ ] `internal/plugins/sysctl/backend_linux.go` - keyToPath supports net.ipv6.conf.<iface>.<leaf>
- [ ] `vendor/github.com/vishvananda/netlink/neigh_linux.go` - NeighSubscribe, NTF_ROUTER=0x80

**Behavior to preserve:**
- IPv4 DHCP route metric via route-priority (spec-iface-route-priority)
- RA processing for SLAAC, prefix info, RDNSS (only suppress default route)
- Existing link failover for IPv4 DHCP routes
- Default behavior when route-priority not configured (kernel handles everything, metric 0)

**Behavior to change:**
- When `route-priority` is configured on a unit, suppress kernel default route from RA
- Monitor netlink neighbor events to detect IPv6 routers
- Install IPv6 default route with configured metric
- Extend link failover to handle IPv6 default routes

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Config file with `interface { ethernet eth0 { unit 0 { route-priority 5 } } }`
- Netlink neighbor event with NTF_ROUTER flag on interface eth0

### Transformation Path
1. Config parsed: `unitEntry.RoutePriority = 5` (already done, spec-iface-route-priority)
2. If RoutePriority > 0 and IPv6 is active: set sysctl `net.ipv6.conf.eth0.accept_ra_defrtr = 0`
3. Netlink monitor receives `NeighUpdate` with `Flags & NTF_ROUTER != 0` on eth0
4. Router's link-local address extracted (e.g., `fe80::1`)
5. `AddRoute("::/0", "fe80::1%eth0", 5)` installs default route with configured metric
6. On link-down: `RemoveRoute("::/0", "fe80::1%eth0", 5)`, `AddRoute("::/0", "fe80::1%eth0", 1029)`
7. On link-up: reverse of step 6
8. On neighbor deletion (router gone): `RemoveRoute("::/0", "fe80::1%eth0", metric)`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> iface plugin | unitEntry.RoutePriority (existing) | [ ] |
| iface plugin -> sysctl | Set accept_ra_defrtr=0 via sysctl plugin or direct write | [ ] |
| netlink -> monitor | NeighSubscribe channel in monitor event loop | [ ] |
| monitor -> event bus | New event type for router discovery | [ ] |
| event bus -> iface plugin | Handler installs/removes IPv6 default route | [ ] |

### Integration Points
- `ifacenetlink/monitor_linux.go` - add NeighSubscribe channel to event loop
- `iface/register.go` - extend handleLinkDown/handleLinkUp for `::/0`
- `iface/register.go` - new handler for router discovery events
- `iface/register.go` - track IPv6 router gateways per interface (like gateway for IPv4)
- sysctl write for `accept_ra_defrtr`

### Architectural Verification
- [ ] No bypassed layers (netlink -> monitor -> event bus -> handler)
- [ ] No unintended coupling (monitor emits events, handler reacts)
- [ ] No duplicated functionality (extends existing monitor and failover)
- [ ] Zero-copy preserved where applicable (N/A)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with route-priority + IPv6 RA | -> | accept_ra_defrtr=0 set, IPv6 default route installed | To be designed |
| Link down with IPv6 default route | -> | IPv6 route deprioritized | To be designed |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `route-priority 5` on unit with IPv6 | `accept_ra_defrtr` set to 0 on that interface |
| AC-2 | Netlink neighbor event with NTF_ROUTER on monitored interface | IPv6 default route installed with metric 5 |
| AC-3 | No route-priority configured (default 0) | `accept_ra_defrtr` left at kernel default (1), no ze intervention |
| AC-4 | Link down on interface with IPv6 default route, route-priority 5 | IPv6 route deprioritized to metric 1029 |
| AC-5 | Link up after AC-4 | IPv6 route restored to metric 5 |
| AC-6 | Router disappears (neighbor deleted or NUD_FAILED) | IPv6 default route removed |
| AC-7 | Multiple routers on same link | Multiple IPv6 default routes, all with configured metric |
| AC-8 | Reload changes route-priority from 5 to 10 | IPv6 default route metric updated |
| AC-9 | `route-priority 0` explicitly configured | Same as AC-3: kernel handles everything |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNeighRouterDetected` | `internal/component/iface/config_test.go` | Router event triggers route install | |
| `TestNeighRouterRemoved` | `internal/component/iface/config_test.go` | Router removal triggers route delete | |
| `TestLinkDownIPv6` | `internal/component/iface/config_test.go` | IPv6 route deprioritized on link-down | |
| `TestLinkUpIPv6` | `internal/component/iface/config_test.go` | IPv6 route restored on link-up | |
| `TestAcceptRaDefrtrSet` | `internal/component/iface/config_test.go` | Sysctl set when route-priority > 0 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| route-priority | 0-4294966271 | 4294966271 | N/A | 4294966272 |

Note: same range as IPv4 (already tested in spec-iface-route-priority).

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipv6-route-priority` | `test/parse/ipv6-route-priority.ci` | Config with route-priority + IPv6 settings parses | |

### Future (if deferring any tests)
- Integration test with real RA on Linux (requires router or radvd)
- Multi-router scenario test

## Files to Modify

- `internal/plugins/ifacenetlink/monitor_linux.go` - add NeighSubscribe channel
- `internal/component/iface/register.go` - IPv6 router tracking, extend handleLinkDown/handleLinkUp
- `internal/component/iface/backend.go` - ensure AddRoute/RemoveRoute handle IPv6 (::/0, fe80:: next-hop)
- `internal/component/iface/iface.go` - new event type for router discovery
- `internal/component/plugin/events.go` - EventInterfaceRouterDiscovered / EventInterfaceRouterLost

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | N/A (route-priority leaf already exists) |
| CLI commands/flags | [ ] | N/A |
| Editor autocomplete | [ ] | N/A |
| Functional test | [x] | `test/parse/ipv6-route-priority.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features/interfaces.md` - IPv6 route-priority row |
| 2 | Config syntax changed? | [ ] | N/A (reuses existing route-priority leaf) |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/configuration.md` - extend Route Priority section |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc4861.md` if not exists |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `test/parse/ipv6-route-priority.ci` - functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Netlink neighbor monitoring** -- add NeighSubscribe to monitor, emit router events
   - Tests: TestNeighRouterDetected, TestNeighRouterRemoved
   - Files: monitor_linux.go, events.go, iface.go
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Sysctl suppression** -- set accept_ra_defrtr=0 when route-priority configured
   - Tests: TestAcceptRaDefrtrSet
   - Files: register.go
   - Verify: tests fail -> implement -> tests pass

3. **Phase: IPv6 route management** -- install/remove IPv6 default route from router events
   - Tests: TestLinkDownIPv6, TestLinkUpIPv6
   - Files: register.go, backend.go
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Link failover** -- extend handleLinkDown/handleLinkUp for ::/0
   - Tests: existing link failover tests extended
   - Files: register.go
   - Verify: tests fail -> implement -> tests pass

5. **Functional tests** -- config parse test
6. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented |
| Correctness | accept_ra_defrtr=0 only set when route-priority > 0, not always |
| Correctness | Link-local next-hop includes interface scope (fe80::1%eth0) |
| Correctness | Router lifetime respected (route removed when router expires) |
| Naming | Event types follow existing pattern (EventInterfaceRouter*) |
| Data flow | Netlink -> monitor -> event bus -> handler (no shortcuts) |
| Rule: no-layering | No duplicate route management path |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| NeighSubscribe in monitor | grep NeighSubscribe monitor_linux.go |
| Router event types | grep EventInterfaceRouter events.go |
| accept_ra_defrtr management | grep accept_ra_defrtr register.go |
| IPv6 route install | grep "::/0" register.go |
| Link failover handles IPv6 | grep "::/0" handleLinkDown/handleLinkUp |
| Functional test | ls test/parse/ipv6-route-priority.ci |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | NTF_ROUTER flag verified before installing route |
| Scope | Only suppress accept_ra_defrtr on interfaces ze manages |
| Restore | accept_ra_defrtr restored to 1 when route-priority removed or ze shuts down |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
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

(to be filled during implementation)

## RFC Documentation

- RFC 4861 Section 6.3.4 -- Router Advertisement processing, router lifetime
- RFC 4861 Section 7.3.1 -- Neighbor Unreachability Detection (NUD states)

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
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
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
- [ ] Write learned summary to `plan/learned/NNN-iface-ipv6-default-route.md`
- [ ] Summary included in commit
