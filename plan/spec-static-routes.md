# Spec: static-routes -- Static route plugin with ECMP and weighted next-hops

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 8/11 |
| Updated | 2026-04-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugins/fib/kernel/backend_linux.go` -- netlink route programming (multipath reference)
4. `internal/plugins/fib/kernel/register.go` -- plugin registration pattern with SDK 5-stage protocol
5. `internal/plugins/bfd/api/service.go` -- BFD client Service interface (EnsureSession, ReleaseSession)
6. `internal/core/redistevents/registry.go` -- RegisterProtocol for redistribute visibility

## Task

Add a static route plugin that lets operators declare static routes in ze config. The plugin
programs routes directly to the kernel (via netlink) and/or VPP (via GoVPP), supporting:

- **ECMP**: multiple next-hops installed simultaneously as a multipath route
- **Weighted ECMP**: per-next-hop weight controls traffic distribution
- **BFD failover**: next-hops tracked by BFD are removed from the ECMP group on session DOWN
  and re-added on UP; the multipath route is reprogrammed with remaining next-hops
- **Blackhole/reject**: silent discard or ICMP unreachable
- **IPv4 and IPv6**
- **Config reload**: full reconciliation (add missing, remove orphaned, replace changed)

### Why direct FIB programming, not LocRIB?

LocRIB selects a single best path per prefix (selectBest in entry.go). It has no concept of
ECMP groups or per-next-hop weights. Extending LocRIB for these features would change the
sysrib/FIB pipeline contract for all protocols. Since static routes are config-authoritative
(the operator declares exactly what they want), admin-distance arbitration adds little value:
static (distance 10) already beats eBGP (20) and iBGP (200) by default.

The plugin registers via `redistevents.RegisterProtocol("static")` so redistribute consumers
can discover and import static routes.

### Design influences

Studied Junos (qualified-next-hop with per-NH preference/metric), Nokia SR OS (separate entries
per NH, bfd-enable, weighted-ecmp for tunnels), and VyOS (multiple next-hop lines = ECMP,
distance, bfd profile). Ze's syntax follows VyOS simplicity with per-next-hop weight from
OpenConfig's wecmp-weight.

## Required Reading

### Architecture Docs
- [ ] `internal/plugins/fib/kernel/backend_linux.go` -- netlink route programming
  --> Constraint: buildRoute creates netlink.Route with Dst, Gw, Protocol=RTPROT_ZE(250)
  --> Constraint: does NOT set Route.Table (always main table); does NOT set MultiPath
  --> Decision: Static plugin uses its own netlink handle; sets MultiPath for ECMP routes
- [ ] `internal/plugins/fib/kernel/fibkernel.go` -- FIB kernel plugin lifecycle
  --> Constraint: subscribes to (system-rib, best-change); owns RTPROT_ZE routes from sysrib
  --> Decision: Static plugin uses same RTPROT_ZE=250 but manages different prefixes (config-driven vs protocol-driven)
- [ ] `internal/plugins/fib/kernel/register.go` -- plugin registration pattern
  --> Constraint: registry.Registration{Name, YANG, ConfigRoots, RunEngine}
  --> Constraint: SDK 5-stage: OnConfigVerify, OnConfigure, OnConfigApply, OnConfigRollback, OnStarted
  --> Decision: Static plugin follows identical pattern
- [ ] `internal/plugins/fib/vpp/fibvpp.go` -- VPP FIB programming
  --> Constraint: uses ip_route_add_del with FibPath entries; multiple FibPaths = ECMP
  --> Constraint: FibPath has Weight field for weighted ECMP
  --> Decision: Static plugin calls VPP API directly for VPP-mode routes
- [ ] `internal/core/redistevents/registry.go` -- ProtocolID registration
  --> Constraint: RegisterProtocol(name) allocates ProtocolID at init time; idempotent on name
  --> Constraint: RegisterProducer(id) marks protocol as having a producer
  --> Decision: Static plugin calls RegisterProtocol("static") + RegisterProducer in init()
- [ ] `internal/core/redistevents/events.go` -- RouteChangeBatch payload
  --> Constraint: RouteChangeEntry{Action, Prefix, NextHop, Metric} -- value types only
  --> Decision: Static plugin emits route-change events for redistribute consumers
- [ ] `internal/plugins/bfd/api/service.go` -- BFD client contract
  --> Constraint: GetService() returns nil before BFD plugin starts; callers must handle nil
  --> Constraint: EnsureSession(SessionRequest) returns SessionHandle; caller MUST ReleaseSession when done
  --> Constraint: SessionHandle.Subscribe() returns chan StateChange; caller MUST Unsubscribe
  --> Decision: Static plugin calls GetService() in OnStarted; skips BFD if nil
- [ ] `internal/plugins/bfd/api/events.go` -- SessionRequest, Key, StateChange
  --> Constraint: SessionRequest.Profile is optional name; SessionRequest.Peer is next-hop addr
  --> Constraint: StateChange.State is packet.State; check for packet.StateUp/StateDown
  --> Decision: Each next-hop with bfd-profile gets its own EnsureSession call
- [ ] `vendor/github.com/vishvananda/netlink/route.go` -- netlink.Route struct
  --> Constraint: Route.MultiPath is []*NexthopInfo; each has Gw, LinkIndex, Hops (weight=Hops+1)
  --> Constraint: Route.Type controls RTN_UNICAST/RTN_BLACKHOLE/RTN_UNREACHABLE/RTN_PROHIBIT
  --> Decision: ECMP uses MultiPath; single next-hop uses Gw directly; weight maps to Hops field

### RFC Summaries (MUST for protocol work)

Not protocol work. Static routes are a local configuration feature, not an RFC protocol.

**Key insights:**
- Linux multipath: netlink.Route.MultiPath = []*NexthopInfo, each with Gw and Hops (weight = Hops+1)
- VPP multipath: ip_route_add_del with multiple FibPath entries, each with Weight field
- BFD client API (GetService/EnsureSession/Subscribe) for fast failover within ECMP group
- RTPROT_ZE=250 identifies ze-installed routes in kernel; shared with fib-kernel (no collision since different prefixes)
- redistribute RegisterProtocol("static") + RegisterProducer provides redistribute visibility without LocRIB
- Blackhole/reject: kernel RTN_BLACKHOLE/RTN_UNREACHABLE; VPP FIB_API_PATH_TYPE_DROP/ICMP_UNREACH
- Config reconciliation: list existing RTPROT_ZE routes for static-owned prefixes, diff against desired, add/remove/replace

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/fib/kernel/backend_linux.go` -- programs single-next-hop routes via netlink; uses buildRoute with Dst+Gw+Protocol; no MultiPath support today
  --> Constraint: routeBackend interface: addRoute(prefix, nextHop string), delRoute(prefix), replaceRoute(prefix, nextHop), listZeRoutes(), close()
- [ ] `internal/plugins/fib/kernel/fibkernel.go` -- subscribes to sysrib best-change events; crash recovery via stale-mark-sweep of RTPROT_ZE routes
  --> Constraint: manages routes learned from sysrib; static plugin manages config-driven routes separately
- [ ] `internal/plugins/fib/vpp/fibvpp.go` -- programs routes via GoVPP ip_route_add_del; subscribes to sysrib events
  --> Constraint: VPP route programming uses FibPath with Nh.Address, SwIfIndex, Weight
- [ ] `internal/plugins/bfd/api/registry.go` -- SetService/GetService atomic pointer pattern
  --> Constraint: GetService() returns nil when BFD not running; callers must handle gracefully
- [ ] `internal/component/config/redistribution/schema/ze-redistribute-conf.yang` -- lists "static" as valid source
  --> Constraint: redistribute already expects "static" as a protocol name
- [ ] `internal/plugins/sysrib/schema/ze-rib-conf.yang` -- admin-distance block with static default 10
  --> Constraint: admin-distance "static" remains for future LocRIB integration; not used by direct-FIB approach

**Behavior to preserve:**
- fib-kernel continues to program sysRIB routes into the main table unchanged
- RTPROT_ZE=250 shared by both fib-kernel and static routes (all ze routes identifiable)
- No collision: fib-kernel and static routes manage different prefixes
- redistribute "static" source listing unchanged
- BFD plugin's Service interface unchanged

**Behavior to change:**
- Add new static route plugin that programs routes directly to kernel/VPP
- Add YANG config schema for static routes (ze-static-conf.yang)
- Add `static show` CLI command
- Emit redistevents for static route changes (for redistribute consumers)
- Update docs/guide/command-catalogue.md with static route commands

## Data Flow (MANDATORY)

### Entry Point
- YANG config file parsed at startup or reload by config component
- Config tree contains `static { ... }` section
- Plugin receives config JSON via SDK OnConfigure/OnConfigApply callbacks

### Transformation Path
1. Config file: `static { route 0.0.0.0/0 { next-hop 10.0.0.1 { weight 3 } next-hop 10.0.0.2 { weight 1 } } }`
2. YANG validation (prefix format, next-hop address format, weight range, bfd-profile reference)
3. Plugin OnConfigure: parse config JSON into []staticRoute structs
4. Plugin OnStarted: for each forwarding route, resolve next-hops and program multipath route to kernel/VPP
5. For blackhole/reject routes: program with appropriate route type (RTN_BLACKHOLE/RTN_UNREACHABLE)
6. For next-hops with bfd-profile: plugin calls bfd.GetService().EnsureSession(), subscribes to StateChange
7. On BFD DOWN: remove that next-hop from the ECMP group, reprogram the multipath route with remaining active next-hops
8. On BFD UP: re-add the next-hop to the ECMP group, reprogram the multipath route
9. If all next-hops go DOWN: withdraw the route entirely
10. Emit redistevents RouteChangeBatch for redistribute consumers
11. On config reload: diff old vs new route set, apply changes (add/remove/replace)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config --> Plugin | YANG tree JSON via SDK OnConfigure | [ ] |
| Plugin --> Kernel | netlink.RouteAdd/RouteReplace/RouteDel with MultiPath | [ ] |
| Plugin --> VPP | ip_route_add_del with multiple FibPath entries | [ ] |
| Plugin --> BFD | api.GetService().EnsureSession(req) | [ ] |
| BFD --> Plugin | chan StateChange subscription | [ ] |
| Plugin --> Redistribute | redistevents RouteChangeBatch emission | [ ] |

### Integration Points
- `vendor/github.com/vishvananda/netlink` -- Route programming (already vendored)
- `go.fd.io/govpp` -- VPP binary API (already vendored)
- `internal/core/redistevents/` -- RegisterProtocol("static"), RouteChangeBatch emission
- `internal/plugins/bfd/api/` -- GetService/EnsureSession for BFD client
- `internal/component/plugin/registry/` -- Register() for plugin lifecycle
- `pkg/plugin/sdk/` -- SDK 5-stage protocol for config management

### Architectural Verification
- [ ] No bypassed layers (config --> plugin --> kernel/VPP; direct path is intentional for ECMP)
- [ ] No unintended coupling (static plugin independent of fib-kernel/fib-vpp/sysrib)
- [ ] No duplicated functionality (fib-kernel manages sysrib routes; static manages config routes; same RTPROT, different prefixes)
- [ ] Zero-copy not applicable (config-driven, small data, no wire encoding)

## Data Model

### staticRoute (internal representation)

| Field | Type | Description |
|-------|------|-------------|
| Prefix | netip.Prefix | Destination prefix in CIDR notation |
| Description | string | Optional operator note |
| Metric | uint32 | Route metric (default 0); used as kernel route priority |
| Tag | uint32 | Opaque tag for route policy matching (carried in redistribute) |
| Action | actionType | forward, blackhole, or reject |
| NextHops | []nextHop | Gateway list (only for forward action); all active NHs installed as ECMP |

### nextHop

| Field | Type | Description |
|-------|------|-------------|
| Address | netip.Addr | Gateway IP address |
| Interface | string | Outgoing interface (required for link-local IPv6 next-hops) |
| Weight | uint16 | ECMP weight (default 1); higher = more traffic; kernel: Hops=Weight-1 |
| BFDProfile | string | BFD profile name for fast failover (empty = no BFD) |

### actionType enum

| Value | Kernel | VPP | Description |
|-------|--------|-----|-------------|
| actionForward | RTN_UNICAST + MultiPath | ip_route_add_del + []FibPath | Normal forwarding, ECMP |
| actionBlackhole | RTN_BLACKHOLE | FIB_API_PATH_TYPE_DROP | Silently discard |
| actionReject | RTN_UNREACHABLE | FIB_API_PATH_TYPE_ICMP_UNREACH | ICMP unreachable |

### FIB mapping

**Linux kernel (netlink):**

| Ze field | netlink field | Notes |
|----------|---------------|-------|
| Prefix | Route.Dst | net.IPNet from netip.Prefix |
| NextHops (single) | Route.Gw | net.IP; no MultiPath needed |
| NextHops (multiple) | Route.MultiPath | []*NexthopInfo; one per active NH |
| NextHop.Address | NexthopInfo.Gw | net.IP |
| NextHop.Interface | NexthopInfo.LinkIndex | resolved via netlink.LinkByName |
| NextHop.Weight | NexthopInfo.Hops | Hops = Weight - 1 (kernel adds 1 internally) |
| Metric | Route.Priority | lower = preferred |
| blackhole | Route.Type = RTN_BLACKHOLE | no Gw/MultiPath |
| reject | Route.Type = RTN_UNREACHABLE | no Gw/MultiPath |
| (all) | Route.Protocol = RTPROT_ZE(250) | identifies ze-managed routes |

**VPP (GoVPP):**

| Ze field | VPP API field | Notes |
|----------|---------------|-------|
| Prefix | Route.Prefix | ip_types.Prefix |
| NextHops | Route.Paths[] | one FibPath per active NH |
| NextHop.Address | FibPath.Nh.Address | ip_types.Address |
| NextHop.Interface | FibPath.SwIfIndex | resolved via sw_interface_dump |
| NextHop.Weight | FibPath.Weight | uint8; direct mapping |
| Metric | FibPath.Preference | uint8; lower = preferred |
| blackhole | FibPath.Type = FIB_API_PATH_TYPE_DROP | |
| reject | FibPath.Type = FIB_API_PATH_TYPE_ICMP_UNREACH | |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | --> | Feature Code | Test |
|-------------|-----|--------------|------|
| Config with `static { route ... next-hop ... }` at boot | --> | registry.Register, OnConfigure, backend.Apply | `test/static/001-boot-apply.ci` |
| Config reload adds a static route | --> | OnConfigApply, backend.Apply reconciliation | `test/static/002-reload-add.ci` |
| Config reload removes a static route | --> | OnConfigApply, backend.Apply reconciliation | `test/static/003-reload-remove.ci` |
| `static show` CLI command | --> | OnExecuteCommand, showRoutes | `test/static/004-show.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `route 10.0.0.0/8 { next-hop 192.168.1.1 { } }` | Single-NH route programmed in kernel with RTPROT_ZE, Gw=192.168.1.1 |
| AC-2 | Config with `route 0.0.0.0/0 { next-hop 10.0.0.1 { } next-hop 10.0.0.2 { } }` | ECMP multipath route with both next-hops active (equal weight) |
| AC-3 | Config with `next-hop 10.0.0.1 { weight 3 } next-hop 10.0.0.2 { weight 1 }` | Weighted ECMP: kernel Hops=2/0 (75%/25% traffic split) |
| AC-4 | One ECMP next-hop's BFD session goes DOWN | Multipath route reprogrammed with remaining active next-hops; weights preserved |
| AC-5 | All BFD-tracked next-hops go DOWN | Route withdrawn from kernel/VPP entirely |
| AC-6 | BFD session comes back UP | Next-hop re-added to ECMP group; multipath route reprogrammed |
| AC-7 | Config with `route 192.0.2.0/24 { blackhole { } }` | Blackhole route (RTN_BLACKHOLE) installed |
| AC-8 | Config with `route 198.51.100.0/24 { reject { } }` | Reject route (RTN_UNREACHABLE) installed |
| AC-9 | Config reload removes a previously configured route | Route deleted from kernel/VPP |
| AC-10 | Config reload adds a new route | Route programmed into kernel/VPP |
| AC-11 | `static show` CLI command | Displays all configured static routes with prefix, next-hop(s), weight, BFD status |
| AC-12 | IPv6 route: `route 2001:db8::/32 { next-hop 2001:db8::1 { } }` | IPv6 route programmed correctly |
| AC-13 | ze boots with static config | All static routes programmed before ze reports "running" |
| AC-14 | BFD not configured (no bfd plugin) | Static routes work; BFD integration gracefully skipped |
| AC-15 | Config reload changes weight on existing next-hop | Multipath route reprogrammed with new weights |
| AC-16 | Redistribute import static | Static route changes emitted as redistevents; BGP redistribute consumer picks them up |
| AC-17 | `docs/guide/command-catalogue.md` updated | Static route show command row added to Section 2 (Routing Table / FIB) |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseStaticConfig` | `internal/plugins/static/config_test.go` | Config JSON to []staticRoute, single next-hop | |
| `TestParseStaticConfigMultiNextHop` | `internal/plugins/static/config_test.go` | Multiple next-hops produce ECMP route | |
| `TestParseStaticConfigWeight` | `internal/plugins/static/config_test.go` | Weight field parsed, default 1 | |
| `TestParseStaticConfigBlackhole` | `internal/plugins/static/config_test.go` | Blackhole route parsing | |
| `TestParseStaticConfigReject` | `internal/plugins/static/config_test.go` | Reject route parsing | |
| `TestParseStaticConfigIPv6` | `internal/plugins/static/config_test.go` | IPv6 prefix and next-hop | |
| `TestParseStaticConfigTag` | `internal/plugins/static/config_test.go` | Tag field parsing | |
| `TestParseStaticConfigDescription` | `internal/plugins/static/config_test.go` | Description field parsing | |
| `TestParseStaticConfigEmpty` | `internal/plugins/static/config_test.go` | Empty config produces no routes | |
| `TestParseStaticConfigInvalidPrefix` | `internal/plugins/static/config_test.go` | Invalid prefix rejected at parse | |
| `TestStaticRouteRegistration` | `internal/plugins/static/register_test.go` | registry.Register succeeds | |
| `TestDiffRoutes` | `internal/plugins/static/diff_test.go` | Diff old vs new route set: additions, removals, changes | |
| `TestDiffRoutesNextHopChange` | `internal/plugins/static/diff_test.go` | Next-hop address change detected | |
| `TestDiffRoutesWeightChange` | `internal/plugins/static/diff_test.go` | Weight change detected | |
| `TestDiffRoutesNHAdded` | `internal/plugins/static/diff_test.go` | Next-hop added to existing route | |
| `TestDiffRoutesNHRemoved` | `internal/plugins/static/diff_test.go` | Next-hop removed from existing route | |
| `TestBuildMultipath` | `internal/plugins/static/backend_test.go` | staticRoute with 3 NHs produces correct []*NexthopInfo with Hops=Weight-1 | |
| `TestBuildMultipathSingleNH` | `internal/plugins/static/backend_test.go` | Single NH uses Gw directly (no MultiPath) | |
| `TestBuildBlackhole` | `internal/plugins/static/backend_test.go` | Blackhole route has Type=RTN_BLACKHOLE, no Gw | |
| `TestBuildReject` | `internal/plugins/static/backend_test.go` | Reject route has Type=RTN_UNREACHABLE, no Gw | |
| `TestActiveNextHops` | `internal/plugins/static/inject_test.go` | BFD DOWN removes NH from active set; BFD UP re-adds | |
| `TestActiveNextHopsAllDown` | `internal/plugins/static/inject_test.go` | All NHs down produces empty active set (route withdrawn) | |
| `TestFormatStaticRoutes` | `internal/plugins/static/cmd_test.go` | CLI show output formatting with weights and BFD status | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Weight | 1-65535 | 65535 | 0 (rejected, min 1) | N/A (uint16) |
| Metric | 0-4294967295 | 4294967295 | N/A (0 is valid default) | N/A (uint32) |
| Tag | 0-4294967295 | 4294967295 | N/A (0 is valid) | N/A (uint32) |
| IPv4 prefix length | 0-32 | 32 | N/A (0 is default route) | 33 (parse error) |
| IPv6 prefix length | 0-128 | 128 | N/A (0 is default route) | 129 (parse error) |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Boot apply | `test/static/001-boot-apply.ci` | Config with static routes, verify in kernel via show command | |
| Reload add | `test/static/002-reload-add.ci` | Add route on reload, verify appears | |
| Reload remove | `test/static/003-reload-remove.ci` | Remove route on reload, verify gone | |
| Show command | `test/static/004-show.ci` | `static show` returns configured routes with weights | |

### Future (if deferring any tests)
- BFD failover functional test: requires BFD engine running in test harness (complex setup; unit tests cover the code path)
- VPP backend functional test: requires VPP stub harness (unit tests cover API construction)

## Files to Modify

- `docs/guide/command-catalogue.md` -- add static route show command row to Section 2

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | `internal/plugins/static/schema/ze-static-conf.yang` |
| CLI commands | Yes | `static show` via OnExecuteCommand |
| Editor autocomplete | Yes | YANG-driven (automatic) |
| Functional test | Yes | `test/static/*.ci` |
| Plugin import generation | Yes | `scripts/gen-plugin-imports.go` scans internal/plugins/ |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- static route plugin with ECMP |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` -- static route config section |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- `static show` |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` -- static route plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/static-routes.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- static routes with ECMP |
| 12 | Internal architecture changed? | No | Follows existing patterns |
| 13 | Command catalogue updated? | Yes | `docs/guide/command-catalogue.md` -- static route commands in Section 2 |

## Files to Create

- `internal/plugins/static/model.go` -- staticRoute, nextHop, actionType structs
- `internal/plugins/static/config.go` -- config JSON parsing to []staticRoute
- `internal/plugins/static/diff.go` -- diff old vs new route set for reload
- `internal/plugins/static/inject.go` -- route lifecycle, BFD session management, active NH tracking
- `internal/plugins/static/backend.go` -- Backend interface (Apply, Remove, List, Close)
- `internal/plugins/static/backend_linux.go` -- netlink backend with multipath support
- `internal/plugins/static/backend_other.go` -- noop backend for non-Linux
- `internal/plugins/static/backend_vpp.go` -- VPP backend via GoVPP
- `internal/plugins/static/register.go` -- init(), registry.Register, runStaticPlugin
- `internal/plugins/static/schema/ze-static-conf.yang` -- YANG config schema
- `internal/plugins/static/schema/register.go` -- go:embed + yang.RegisterModule
- `internal/plugins/static/config_test.go` -- config parsing tests
- `internal/plugins/static/diff_test.go` -- diff tests
- `internal/plugins/static/backend_test.go` -- backend construction tests
- `internal/plugins/static/inject_test.go` -- BFD integration / active NH tracking tests
- `internal/plugins/static/register_test.go` -- registration test
- `internal/plugins/static/cmd_test.go` -- CLI output formatting test
- `test/static/001-boot-apply.ci` -- functional test
- `test/static/002-reload-add.ci` -- functional test
- `test/static/003-reload-remove.ci` -- functional test
- `test/static/004-show.ci` -- functional test
- `docs/guide/static-routes.md` -- user guide

## YANG Config Schema

```
static {
    route 10.0.0.0/8 {
        next-hop 192.168.1.1 { }
    }
    route 0.0.0.0/0 {
        description "default route with weighted ECMP and BFD"
        next-hop 10.0.0.1 {
            weight 3
            bfd-profile wan-fast
        }
        next-hop 10.0.0.2 {
            weight 1
            bfd-profile wan-fast
        }
    }
    route 192.0.2.0/24 {
        blackhole { }
    }
    route 198.51.100.0/24 {
        reject { }
    }
    route 2001:db8::/32 {
        next-hop 2001:db8::1 { }
    }
    route 172.16.0.0/12 {
        next-hop 10.0.0.1 { }
        tag 100
    }
}
```

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Create |
| 3. Implement (TDD) | Phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7-12 | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Data model** -- staticRoute, nextHop, actionType structs and validation
   - Tests: TestParseStaticConfigEmpty, TestParseStaticConfigInvalidPrefix
   - Files: model.go
   - Verify: tests fail --> implement --> tests pass

2. **Phase: YANG + config parsing** -- schema and JSON parser
   - Tests: TestParseStaticConfig, TestParseStaticConfigMultiNextHop, TestParseStaticConfigWeight, TestParseStaticConfigBlackhole, TestParseStaticConfigReject, TestParseStaticConfigIPv6, TestParseStaticConfigTag, TestParseStaticConfigDescription
   - Files: ze-static-conf.yang, config.go, config_test.go, schema/*.go
   - Verify: tests fail --> implement --> tests pass

3. **Phase: Diff engine** -- compute additions, removals, changes between two route sets
   - Tests: TestDiffRoutes, TestDiffRoutesNextHopChange, TestDiffRoutesWeightChange, TestDiffRoutesNHAdded, TestDiffRoutesNHRemoved
   - Files: diff.go, diff_test.go
   - Verify: tests fail --> implement --> tests pass

4. **Phase: Backend (Linux)** -- netlink route programming with multipath + weight
   - Tests: TestBuildMultipath, TestBuildMultipathSingleNH, TestBuildBlackhole, TestBuildReject
   - Files: backend.go, backend_linux.go, backend_other.go, backend_test.go
   - Verify: tests fail --> implement --> tests pass

5. **Phase: Backend (VPP)** -- GoVPP route programming with multipath + weight
   - Tests: unit tests with mock VPP
   - Files: backend_vpp.go
   - Verify: tests fail --> implement --> tests pass

6. **Phase: BFD integration + active NH tracking** -- manage ECMP group membership based on BFD state
   - Tests: TestActiveNextHops, TestActiveNextHopsAllDown
   - Files: inject.go, inject_test.go
   - Verify: tests fail --> implement --> tests pass

7. **Phase: Plugin registration** -- init(), RunEngine, SDK 5-stage, CLI show command, redistevents
   - Tests: TestStaticRouteRegistration, TestFormatStaticRoutes
   - Files: register.go, register_test.go, cmd_test.go
   - Verify: tests fail --> implement --> tests pass

8. **Phase: Documentation** -- command catalogue, features, user guide
   - Files: docs/guide/command-catalogue.md, docs/features.md, docs/guide/static-routes.md
   - Verify: content accurate

9. **Functional tests** --> Boot apply, reload, show
10. **Full verification** --> `make ze-verify`
11. **Complete spec** --> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N (AC-1 through AC-17) has implementation with file:line |
| Correctness | ECMP weights map correctly: kernel Hops=Weight-1; VPP Weight=Weight |
| Naming | YANG uses kebab-case; config keys match YANG leaf names |
| Data flow | Config --> staticRoute --> backend.Apply --> netlink/GoVPP; no LocRIB |
| BFD lifecycle | Sessions released on config reload and plugin shutdown; nil GetService handled; ECMP group reprogrammed on state change |
| Reconciliation | Apply lists existing RTPROT_ZE routes for owned prefixes; adds missing, removes orphaned, replaces changed |
| Redistribute | RouteChangeBatch emitted on route add/remove for redistribute consumers |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| Static plugin registered | `grep "registry.Register" internal/plugins/static/register.go` |
| YANG schema exists | `ls internal/plugins/static/schema/ze-static-conf.yang` |
| ProtocolID registered | `grep 'RegisterProtocol.*static' internal/plugins/static/register.go` |
| Multipath support | `grep "MultiPath" internal/plugins/static/backend_linux.go` |
| Weight support | `grep -i "weight\|Hops" internal/plugins/static/backend_linux.go` |
| VPP backend | `ls internal/plugins/static/backend_vpp.go` |
| BFD client integration | `grep "GetService\|EnsureSession" internal/plugins/static/inject.go` |
| CLI show command | `grep "static show" internal/plugins/static/register.go` |
| Command catalogue updated | `grep -i "static.*show\|show.*static" docs/guide/command-catalogue.md` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Prefix validated via netip.ParsePrefix; next-hop via netip.ParseAddr |
| Input validation | Weight validated: range 1-65535; reject 0 |
| Input validation | BFD profile name validated against existing bfd profile list |
| Resource bounds | Number of static routes bounded only by config size (operator-controlled) |
| Privilege | Requires CAP_NET_ADMIN for netlink route operations |
| Route isolation | Only manages RTPROT_ZE routes for owned prefixes; never touches non-ze routes |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior --> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural --> DESIGN phase |
| Functional test fails | Check AC; if AC wrong --> DESIGN; if AC correct --> IMPLEMENT |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | Plugin in internal/plugins/static/ | Static routes are a route source, like sysrib and FIB. Plugins own protocol lifecycle. |
| 2 | Direct FIB programming, not LocRIB | LocRIB selects single best, has no ECMP or weight concept. Extending it for ECMP would change the pipeline contract for all protocols. Direct programming gives native ECMP + weighted ECMP on both kernel and VPP. |
| 3 | ECMP via multipath routes | All active next-hops installed simultaneously. Linux: Route.MultiPath with []*NexthopInfo. VPP: multiple FibPath entries. Matches how VyOS, Junos, and Nokia handle static ECMP. |
| 4 | Per-next-hop weight | Linux: NexthopInfo.Hops = Weight-1. VPP: FibPath.Weight. OpenConfig wecmp-weight is the standard model equivalent. Default weight=1 gives equal ECMP. |
| 5 | BFD modifies ECMP group membership | On BFD DOWN, that NH is removed from the active set and the multipath route is reprogrammed with remaining NHs. Preserves weights of surviving NHs. When all NHs are down, the route is withdrawn. |
| 6 | Same RTPROT_ZE=250 as fib-kernel | All ze-managed routes identifiable by protocol ID. No collision: fib-kernel manages sysrib-derived prefixes, static manages config-driven prefixes. Crash recovery: sweep RTPROT_ZE routes for static-owned prefixes. |
| 7 | RegisterProtocol("static") for redistribute | Even without LocRIB, static routes are visible to redistribute consumers via redistevents. BGP redistribute can import static routes. |
| 8 | Separate backend implementations per OS/dataplane | backend_linux.go (netlink), backend_vpp.go (GoVPP), backend_other.go (noop). Build-tag selection. Same pattern as fib-kernel/fib-vpp. |
| 9 | Config syntax: VyOS simplicity + OpenConfig weight | VyOS-like hierarchy (route/next-hop/blackhole/reject). Per-next-hop weight from OpenConfig wecmp-weight. BFD profile reference from VyOS bfd integration. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| LocRIB injection (first spec draft) | LocRIB has no ECMP or weight support; selectBest picks single winner | Direct FIB programming with native multipath |
| Single-best with failover | User requires ECMP (VyOS parity); weighted ECMP also needed | Full ECMP with per-NH weight |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

Not applicable (static routes are not an RFC protocol).

## Implementation Summary

### What Was Implemented
- (To be filled after implementation)

### Bugs Found/Fixed
- (To be filled)

### Documentation Updates
- (To be filled)

### Deviations from Plan
- (To be filled)

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
- [ ] AC-1..AC-17 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes
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

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-static-routes.md`
- [ ] Summary included in commit
