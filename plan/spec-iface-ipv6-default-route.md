# Spec: iface-ipv6-default-route -- IPv6 Default Route from RA with Configurable Metric

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-iface-route-priority |
| Phase | 7/7 |
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
| Default route installation | `AddRoute(ifaceName, "::/0", "fe80::router", metric)` with configured route-priority |
| Router lifetime tracking | Remove default route when router disappears (NTF_ROUTER cleared, NUD_FAILED, or neighbor deleted) |
| Stale route cleanup | After setting `accept_ra_defrtr=0`, remove any pre-existing kernel `::/0` routes on the interface |
| Sysctl restore | Restore `accept_ra_defrtr=1` on clean shutdown and when `route-priority` is removed from config |
| Link failover for IPv6 | Extend `handleLinkDown`/`handleLinkUp` to handle `::/0` routes |
| Multiple routers | Support multiple RA sources on the same link (multiple default routes with same metric) |

**Out of scope:**

| Area | Reason |
|------|--------|
| RA packet parsing | Kernel handles RA processing; ze only reacts to neighbor table changes |
| SLAAC address management | Kernel handles autoconfiguration independently of `accept_ra_defrtr` |
| RDNSS from RA | Separate concern, kernel/systemd-resolved handles it |
| DHCPv6 default routes | Standard IPv6 default routing uses RA, not DHCPv6 |
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
- AddRoute already receives ifaceName as first parameter and sets route.LinkIndex from it, so bare link-local IPs (without zone ID) are sufficient
- `net.ParseIP` does not parse zone IDs (`fe80::1%eth0` returns nil), so gateway must be passed as bare IP

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
3. After sysctl suppression: scan and remove any existing kernel `::/0` routes on eth0 (startup race cleanup)
4. Netlink monitor receives `NeighUpdate` with `Flags & NTF_ROUTER != 0` on eth0
5. Monitor emits `EventInterfaceRouterDiscovered` on event bus with router IP and interface name
6. Iface plugin handler stores router in `activeRouters[routerKey{ifaceName, routerIP}]` with configured metric
7. Handler calls `AddRoute("eth0", "::/0", "fe80::1", 5)` (bare IP, LinkIndex from ifaceName)
8. On link-down: `RemoveRoute("eth0", "::/0", "fe80::1", 5)`, `AddRoute("eth0", "::/0", "fe80::1", 1029)`
9. On link-up: reverse of step 8
10. On NTF_ROUTER cleared or neighbor deleted: monitor emits `EventInterfaceRouterLost`, handler removes route and deletes `activeRouters` entry
11. On config reload removing route-priority: restore `accept_ra_defrtr=1`, remove ze-managed `::/0` routes
12. On clean shutdown (OnStopping): restore `accept_ra_defrtr=1` on all suppressed interfaces

### Router Tracking

IPv6 routers are tracked separately from IPv4 DHCP gateways. The iface plugin maintains:

| State | Type | Key | Value | Purpose |
|-------|------|-----|-------|---------|
| `activeRouters` | map | `routerKey` (ifaceName + routerIP) | `routerEntry` (metric at install time) | Track installed IPv6 default routes for failover and cleanup |
| `suppressedRA` | map | ifaceName (string) | bool | Track which interfaces have `accept_ra_defrtr=0` for restore on shutdown/config change |

Both maps are protected by the existing `dhcpMu` lock (short critical sections, no benefit from a separate lock).

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
- `iface/register.go` - track IPv6 routers via `activeRouters` map (separate from IPv4 DHCP `activeDHCP`)
- `iface/register.go` - track suppressed interfaces via `suppressedRA` map for sysctl restore
- sysctl write for `accept_ra_defrtr`

### Architectural Verification
- [ ] No bypassed layers (netlink -> monitor -> event bus -> handler)
- [ ] No unintended coupling (monitor emits events, handler reacts)
- [ ] No duplicated functionality (extends existing monitor and failover)
- [ ] Zero-copy preserved where applicable (N/A)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with route-priority + DHCPv6 | -> | accept_ra_defrtr=0 set | `test/parse/ipv6-route-priority.ci` |
| Router event on monitored interface | -> | IPv6 default route installed with metric | TestNeighRouterDetected |
| Link down with IPv6 default route | -> | IPv6 route deprioritized | TestLinkDownIPv6 |
| Config reload removes route-priority | -> | accept_ra_defrtr restored to 1 | TestAcceptRaDefrtrRestore |

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
| AC-8 | Reload changes route-priority from 5 to 10 | IPv6 default route metric updated (old metric removed, new metric installed) |
| AC-9 | `route-priority 0` explicitly configured | Same as AC-3: kernel handles everything |
| AC-10 | Config reload removes `route-priority` (was > 0) | `accept_ra_defrtr` restored to 1 on that interface, ze-managed `::/0` routes removed |
| AC-11 | Clean daemon shutdown while `accept_ra_defrtr=0` on eth0 | `accept_ra_defrtr` restored to 1 on eth0 |
| AC-12 | RA installed kernel `::/0` route before ze set `accept_ra_defrtr=0` | Stale kernel route removed after sysctl suppression |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNeighRouterDetected` | `internal/component/iface/config_test.go` | Router event triggers route install | |
| `TestNeighRouterRemoved` | `internal/component/iface/config_test.go` | Router removal triggers route delete | |
| `TestLinkDownIPv6` | `internal/component/iface/config_test.go` | IPv6 route deprioritized on link-down | |
| `TestLinkUpIPv6` | `internal/component/iface/config_test.go` | IPv6 route restored on link-up | |
| `TestAcceptRaDefrtrSet` | `internal/component/iface/config_test.go` | Sysctl set when route-priority > 0 | |
| `TestAcceptRaDefrtrRestore` | `internal/component/iface/config_test.go` | Sysctl restored to 1 when route-priority removed | |
| `TestAcceptRaDefrtrRestoreOnStop` | `internal/component/iface/config_test.go` | Sysctl restored to 1 on clean shutdown | |
| `TestStaleKernelRouteCleanup` | `internal/component/iface/config_test.go` | Pre-existing kernel `::/0` removed after sysctl suppression | |
| `TestReloadMetricChange` | `internal/component/iface/config_test.go` | Metric change removes old route, installs new metric | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| route-priority | 0-4294966271 | 4294966271 | N/A | 4294966272 |

Note: same range as IPv4 (already tested in spec-iface-route-priority).

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipv6-route-priority` | `test/parse/ipv6-route-priority.ci` | Config with route-priority + DHCPv6 parses, sysctl suppression verified | |

### Known Limitations (documented, not deferred)
- **Router lifetime expiry on idle links:** If a router stops sending RAs and lets its advertised lifetime expire, but the neighbor entry stays in STALE state (no traffic to trigger NUD), ze keeps the stale default route until NUD fires. On active links, NUD detects unreachable routers within ~30 seconds. On idle links the stale route has no operational impact because no traffic uses it.
- **Crash recovery:** If ze is killed (SIGKILL) or crashes, `accept_ra_defrtr` stays at 0. On restart with the same config, ze re-sets to 0 (no harm). If config changed while ze was dead, the user must manually run `sysctl -w net.ipv6.conf.<iface>.accept_ra_defrtr=1` or reboot.

### Future (if deferring any tests)
- Integration test with real RA on Linux (requires router or radvd)
- Multi-router scenario test

## Files to Modify

- `internal/plugins/ifacenetlink/monitor_linux.go` - add NeighSubscribe channel to event loop, emit router discovery/loss events
- `internal/component/iface/register.go` - IPv6 router tracking (`activeRouters`, `suppressedRA` maps), extend handleLinkDown/handleLinkUp for `::/0`, subscribe to router events, sysctl suppression and restore logic, stale kernel route cleanup, OnStopping restore
- `internal/component/plugin/events.go` - add EventInterfaceRouterDiscovered / EventInterfaceRouterLost constants and valid event map entries

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
   - Files: monitor_linux.go, events.go
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Sysctl suppression and stale cleanup** -- set accept_ra_defrtr=0 when route-priority configured, scan and remove pre-existing kernel `::/0` routes
   - Tests: TestAcceptRaDefrtrSet, TestStaleKernelRouteCleanup
   - Files: register.go
   - Verify: tests fail -> implement -> tests pass

3. **Phase: IPv6 route management** -- install/remove IPv6 default route from router events, track routers in activeRouters map
   - Tests: TestNeighRouterDetected (route install), TestNeighRouterRemoved (route removal), TestReloadMetricChange
   - Files: register.go
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Link failover** -- extend handleLinkDown/handleLinkUp for `::/0` using activeRouters
   - Tests: TestLinkDownIPv6, TestLinkUpIPv6
   - Files: register.go
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Sysctl restore** -- restore accept_ra_defrtr=1 on config removal and clean shutdown
   - Tests: TestAcceptRaDefrtrRestore, TestAcceptRaDefrtrRestoreOnStop
   - Files: register.go
   - Verify: tests fail -> implement -> tests pass

6. **Functional tests** -- config parse + sysctl verification test
7. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-12 implemented |
| Correctness | accept_ra_defrtr=0 only set when route-priority > 0, not always |
| Correctness | Link-local next-hop passed as bare IP (no zone ID), LinkIndex from ifaceName |
| Correctness | Router removal handled via NTF_ROUTER cleared, NUD_FAILED, neighbor deleted |
| Correctness | accept_ra_defrtr restored to 1 on config removal and clean shutdown |
| Correctness | Stale kernel `::/0` routes cleaned up after sysctl suppression |
| Naming | Event types follow existing pattern (EventInterfaceRouter*) |
| Data flow | Netlink -> monitor -> event bus -> handler (no shortcuts) |
| Rule: no-layering | No duplicate route management path |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| NeighSubscribe in monitor | grep NeighSubscribe monitor_linux.go |
| Router event types | grep EventInterfaceRouter events.go |
| accept_ra_defrtr management | grep accept_ra_defrtr register.go |
| accept_ra_defrtr restore | grep suppressedRA register.go |
| Stale kernel route cleanup | grep "stale\|cleanup\|existing.*::/0" register.go |
| activeRouters map | grep activeRouters register.go |
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

## Design Decisions

| # | Decision | Resolved | Rationale |
|---|----------|----------|-----------|
| 1 | Router discovery transport | Event bus | Consistent with IPv4 DHCP pattern; package boundary enforces it (ifacenetlink -> bus -> iface) |
| 2 | Link-local gateway scoping | Pass bare IP, rely on existing LinkIndex | AddRoute already receives ifaceName and sets route.LinkIndex; net.ParseIP rejects zone IDs |
| 3 | Router tracking data structure | Separate routerKey/routerEntry map, same dhcpMu | Routers are not DHCP clients; separate map avoids conflating discovery mechanisms |
| 4 | Router lifetime gap | Document, no timer | NUD covers active links; idle links are harmless; YAGNI |
| 5 | Startup race | Scan and remove stale kernel `::/0` routes after sysctl suppression | Fail-mode awareness; tunnel spec reconciliation precedent |
| 6 | Sysctl restore | Restore on shutdown + config change; track suppressed interfaces; document crash | Ze owns what it touches; crash case is inherently unrecoverable |

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
- [ ] AC-1..AC-12 all demonstrated
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
