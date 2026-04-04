# Spec: iface-backend-split

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/6 |
| Updated | 2026-04-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/iface/iface.go` - shared types, topics
4. `internal/component/iface/register.go` - current plugin registration
5. `internal/component/iface/manage_linux.go` - netlink management (moves to backend)
6. `internal/plugins/fibkernel/backend.go` - FIB backend pattern reference

## Task

Split the interface plugin into a configuration/orchestration layer and pluggable backends.
Today the iface component is a monolith: YANG schema, types, validation, discovery, and all
OS operations (netlink management, monitoring, bridge, sysctl, mirror) live together in
`internal/component/iface/`. The Linux-specific code is isolated only by build tags.

The goal is to follow the FIB pattern (fibkernel/fibp4): the iface component owns the config
model and orchestration, while backends (starting with netlink) are separate plugins that
implement a backend interface. This enables future backends (systemd-networkd, FreeBSD, container
runtimes) without restructuring.

DHCP becomes a separate plugin (`iface-dhcp`) since it is a distinct protocol client, not a
backend concern.

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` - interface feature overview
  -> Constraint: JunOS two-layer model (physical + unit) must be preserved
- [ ] `docs/architecture/core-design.md` - component/plugin boundary
  -> Decision: components at `internal/component/`, plugins at `internal/plugins/`
  -> Constraint: plugins register via `registry.Register()` in `init()`

### Existing Learned Summaries
- [ ] `plan/learned/489-iface-0-umbrella.md` - umbrella decisions
  -> Decision: Bus-mediated communication, never direct imports
  -> Decision: `vishvananda/netlink` for all interface operations
  -> Constraint: migration depends on `bgp/listener/ready` Bus event
- [ ] `plan/learned/491-iface-2-manage.md` - management + YANG + CLI
  -> Decision: YANG with `interface-physical` + `interface-unit` groupings
  -> Decision: sysctl with testable `sysctlRoot` override
  -> Constraint: VLAN composite names can exceed IFNAMSIZ

**Key insights:**
- FIB pattern: independent plugins (`fibkernel`, `fibp4`), each with own `backend` interface, selected by config presence
- iface today: ~6,150 lines, 40+ files, all in `internal/component/iface/`
- OnConfigure callback currently ignores config sections (no config application yet)
- Monitor, manage, bridge, sysctl, mirror, show are all direct netlink -- no abstraction layer
- DHCP is a protocol client (DHCPv4/v6) that happens to install addresses via netlink

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/iface.go` (135L) - Bus topics, JSON payload types (AddrPayload, LinkPayload, StatePayload, DHCPPayload), InterfaceInfo, AddrInfo, DiscoveredInterface
  -> Constraint: payload types are the Bus event contract -- all subscribers depend on them
- [ ] `internal/component/iface/register.go` (131L) - plugin registration, `runEngine()` with SDK 5-stage, starts Monitor on configure, ignores config sections
  -> Constraint: OnConfigure receives `[]sdk.ConfigSection` but discards them
- [ ] `internal/component/iface/manage_linux.go` (330L) - CreateDummy/Veth/Bridge/VLAN, DeleteInterface, Add/RemoveAddress, Set/GetMTU, SetAdminUp/Down, Set/GetMACAddress, GetStats. All package-level functions calling netlink directly.
- [ ] `internal/component/iface/monitor_linux.go` (296L) - Monitor struct, netlink subscription (RTM_NEWLINK/DELLINK/ADDR_ADD/ADDR_DEL), publishes Bus events. Long-lived goroutine.
- [ ] `internal/component/iface/show_linux.go` (117L) - ListInterfaces(), GetInterface() via netlink
- [ ] `internal/component/iface/bridge_linux.go` (82L) - BridgeAddPort/DelPort/SetSTP via netlink + sysfs
- [ ] `internal/component/iface/sysctl_linux.go` (145L) - per-interface sysctl writes to /proc/sys/net/
- [ ] `internal/component/iface/mirror_linux.go` (217L) - tc qdisc/filter setup for traffic mirroring
- [ ] `internal/component/iface/dhcp_linux.go` (184L) - DHCPClient type, Start/Stop/Deliver lifecycle
- [ ] `internal/component/iface/dhcp_v4_linux.go` (283L) - DHCPv4 DORA worker, lease install via AddAddress
- [ ] `internal/component/iface/dhcp_v6_linux.go` (356L) - DHCPv6 SOLICIT/REQUEST worker, prefix delegation
- [ ] `internal/component/iface/migrate_linux.go` (213L) - make-before-break migration, subscribes to `bgp/listener/ready`
- [ ] `internal/component/iface/discover.go` (79L) - DiscoverInterfaces(), maps netlink types to Ze types
- [ ] `internal/component/iface/validate.go` (36L) - interface name validation
- [ ] `internal/component/iface/manage_other.go` (76L) - non-Linux stubs, all return "not supported"
- [ ] `internal/component/iface/monitor_other.go` (30L) - non-Linux monitor stub
- [ ] `internal/component/iface/show_other.go` (101L) - non-Linux show via stdlib net
- [ ] `internal/component/iface/bridge_other.go` (26L) - non-Linux bridge stub
- [ ] `internal/component/iface/slaac_linux.go` (18L) - wraps SetIPv6Autoconf
- [ ] `internal/component/iface/schema/ze-iface-conf.yang` (330L) - YANG config schema
- [ ] `internal/component/iface/schema/ze-iface-cmd.yang` (18L) - migrate RPC
- [ ] `internal/component/iface/cmd/cmd.go` (136L) - RPC handler registration, migrate command

**FIB pattern reference:**
- [ ] `internal/plugins/fibkernel/backend.go` (51L) - `routeBackend` interface, shared helpers
- [ ] `internal/plugins/fibkernel/backend_linux.go` (119L) - `netlinkBackend` implementing `routeBackend`
- [ ] `internal/plugins/fibkernel/register.go` (102L) - plugin registration with ConfigRoots `["fib.kernel"]`
- [ ] `internal/plugins/fibp4/register.go` (87L) - ConfigRoots `["fib.p4"]`, augments fib YANG

**Behavior to preserve:**
- Bus topic constants and event payloads (AddrPayload, LinkPayload, StatePayload, DHCPPayload)
- JunOS two-layer model (physical + unit)
- YANG schema structure (interface-physical, interface-unit groupings)
- Interface name validation rules
- Make-before-break migration protocol (5-phase with Bus coordination)
- All existing management operations (create/delete/addr/mtu/mac/bridge/sysctl/mirror)
- Monitor event publishing (created, deleted, up, down, addr-added, addr-removed)
- DHCP lease events (acquired, renewed, expired)
- CLI commands (`ze interface`)

**Behavior to change:**
- Monolithic component -> component + backend plugins
- Direct netlink calls in component -> backend interface calls
- DHCP bundled in iface -> separate `iface-dhcp` plugin
- No config application -> config selects and dispatches to backend
- No backend YANG node -> `backend` leaf with default `netlink`

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Config file defines `interface { ... }` block with backend selection
- Engine parses against YANG schema, produces ConfigSection objects
- Plugin receives config via `OnConfigure([]sdk.ConfigSection)`

### Transformation Path (after split)

1. **Config parse** - engine validates against `ze-iface-conf.yang`, produces ConfigSection
2. **Backend selection** - iface component reads `interface { backend netlink; }`, loads backend
3. **Desired state** - component unmarshals config into desired interface state
4. **Current state** - backend queries OS for current interface state (`ListInterfaces`)
5. **Diff** - component computes delta (create, delete, modify)
6. **Apply** - component dispatches operations to backend via interface
7. **Monitor** - backend watches OS events, publishes to Bus via component topics
8. **React** - subscribers (BGP, other plugins) receive Bus events

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine -> iface component | ConfigSection via SDK 5-stage | [ ] |
| iface component -> backend plugin | Backend interface method calls | [ ] |
| backend plugin -> OS | netlink/sysctl/sysfs (Linux), other APIs (future) | [ ] |
| backend -> Bus | JSON events on `interface/*` topics | [ ] |
| iface-dhcp plugin -> Bus | DHCP lease events on `interface/dhcp/*` topics | [ ] |

### Integration Points
- `registry.Register()` - backend plugins register here
- `ze.Bus` - event publishing for monitor and DHCP
- `sdk.ConfigSection` - config delivery from engine
- `bgp/listener/ready` Bus topic - migration coordination (stays in component)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Design

### Three-Way Split

| Package | Location | Owns |
|---------|----------|------|
| **iface component** | `internal/component/iface/` | Types, topics, YANG config schema, validation, discovery, backend interface definition, config application/orchestration, migration |
| **iface-netlink plugin** | `internal/plugins/ifacenetlink/` | Linux netlink management, monitoring, bridge, sysctl, mirror, show. Implements backend interface. Build-tagged `_linux.go`/`_other.go` within plugin. |
| **iface-dhcp plugin** | `internal/plugins/ifacedhcp/` | DHCPv4/v6 client lifecycle. Subscribes to `interface/addr/*` Bus topics for coordination. Publishes `interface/dhcp/*` events. |

### Backend Interface

Defined in `internal/component/iface/backend.go`. The backend interface covers the operations
the component needs to dispatch to OS-specific code.

| Method group | Operations |
|-------------|------------|
| **Lifecycle** | `CreateDummy`, `CreateVeth`, `CreateBridge`, `CreateVLAN`, `DeleteInterface` |
| **Address** | `AddAddress`, `RemoveAddress` |
| **Link state** | `SetAdminUp`, `SetAdminDown` |
| **Properties** | `SetMTU`, `SetMACAddress`, `GetMACAddress` |
| **Query** | `ListInterfaces`, `GetInterface`, `GetStats` |
| **Bridge** | `BridgeAddPort`, `BridgeDelPort`, `BridgeSetSTP` |
| **Sysctl** | `SetSysctl(ifaceName, key, value string)` |
| **Mirror** | `SetupMirror(ifaceName string, ingress, egress string)` |
| **Monitor** | `StartMonitor(bus ze.Bus)`, `StopMonitor()` |
| **Close** | `Close()` |

The component holds the interface definition. The netlink plugin provides the implementation.
Registration follows the FIB pattern: plugin registers with `registry.Register()`, component
discovers it by name via `registry.Lookup()` or by config root.

### Backend Selection via YANG

Add a `backend` leaf to the `interface` container:

```
container interface {
    leaf backend {
        type string;
        default "netlink";
        description "Interface management backend";
    }
    // ... existing lists (ethernet, dummy, veth, bridge, loopback, monitor)
}
```

The component reads this value during OnConfigure and loads the matching backend plugin.
If the backend is not registered (e.g., `networkd` on a system without the plugin), startup fails
with a clear error.

### DHCP as Separate Plugin

| Aspect | Detail |
|--------|--------|
| Package | `internal/plugins/ifacedhcp/` |
| Registration | `registry.Register()` with name `iface-dhcp`, ConfigRoots `["interface"]` (reads DHCP config from unit containers) |
| Dependencies | `["interface"]` (needs iface plugin for address operations) |
| Communication | Subscribes to `interface/addr/*` for coordination. Publishes `interface/dhcp/lease-*` events. Uses backend via iface component's exported functions or dispatches commands. |
| YANG | DHCP/DHCPv6 leaves stay in `ze-iface-conf.yang` (they're part of the interface config model). Plugin reads them from ConfigSection. |

### What Stays in the Component

| File | Stays? | Reason |
|------|--------|--------|
| `iface.go` | Yes | Types, topics -- the Bus contract |
| `register.go` | Yes (modified) | Component registration, backend loading |
| `validate.go` | Yes | Interface name validation is backend-agnostic |
| `discover.go` | Moves to netlink plugin | Discovery uses netlink type mapping |
| `manage_linux.go` | Moves to netlink plugin | All netlink management ops |
| `manage_other.go` | Moves to netlink plugin | Non-Linux stubs |
| `monitor_linux.go` | Moves to netlink plugin | Netlink event subscription |
| `monitor_other.go` | Moves to netlink plugin | Non-Linux monitor stub |
| `show_linux.go` | Moves to netlink plugin | Netlink interface query |
| `show_other.go` | Moves to netlink plugin | Non-Linux show fallback |
| `bridge_linux.go` | Moves to netlink plugin | Netlink bridge ops |
| `bridge_other.go` | Moves to netlink plugin | Non-Linux bridge stub |
| `sysctl_linux.go` | Moves to netlink plugin | Linux sysctl writes |
| `slaac_linux.go` | Moves to netlink plugin | Wraps sysctl |
| `mirror_linux.go` | Moves to netlink plugin | tc qdisc/filter setup |
| `dhcp_linux.go` | Moves to iface-dhcp plugin | DHCP client lifecycle |
| `dhcp_v4_linux.go` | Moves to iface-dhcp plugin | DHCPv4 worker |
| `dhcp_v6_linux.go` | Moves to iface-dhcp plugin | DHCPv6 worker |
| `migrate_linux.go` | Stays (modified) | Migration is orchestration, calls backend interface |
| `schema/*` | Stays | YANG is the config model |
| `cmd/*` | Stays (modified) | CLI dispatches to backend |

### File Movement Summary

| Destination | Files moving in | Approx lines |
|-------------|----------------|--------------|
| `internal/plugins/ifacenetlink/` | manage, monitor, show, bridge, sysctl, slaac, mirror, discover + their `_other.go` stubs | ~1,550 |
| `internal/plugins/ifacedhcp/` | dhcp, dhcp_v4, dhcp_v6 | ~823 |
| Stays in `internal/component/iface/` | iface.go, register.go, validate.go, migrate, schema/, cmd/ | ~750 + new backend.go |

### Registration Pattern (follows FIB exactly)

**iface-netlink plugin (`internal/plugins/ifacenetlink/register.go`):**

| Field | Value |
|-------|-------|
| Name | `iface-netlink` |
| Description | `Interface backend: Linux netlink management and monitoring` |
| Features | `yang` |
| ConfigRoots | `["interface"]` |
| Dependencies | `[]` |
| RunEngine | `runIfaceNetlinkPlugin` |

**iface-dhcp plugin (`internal/plugins/ifacedhcp/register.go`):**

| Field | Value |
|-------|-------|
| Name | `iface-dhcp` |
| Description | `DHCP client: DHCPv4/DHCPv6 lease acquisition and renewal` |
| Features | `yang` |
| ConfigRoots | `["interface"]` |
| Dependencies | `["interface"]` |
| RunEngine | `runIfaceDHCPPlugin` |

### Monitoring Approach

The monitor is inherently backend-specific (netlink subscriptions on Linux, kqueue/routing socket
on BSD, etc.). The backend interface includes `StartMonitor(bus ze.Bus)` and `StopMonitor()`.
The backend implementation owns the monitor goroutine and publishes events using the component's
topic constants.

The component's `runEngine` starts the backend, which starts its own monitor. Events flow:
OS -> backend monitor -> Bus -> subscribers. The component does not intermediate monitor events.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `interface { backend netlink; ethernet eth0 { ... } }` config | -> | iface component loads netlink backend | `test/plugin/iface-backend-select.ci` |
| `ze interface show` CLI | -> | backend `ListInterfaces()` via component dispatch | `test/plugin/iface-show.ci` |
| netlink link event on Linux | -> | backend monitor publishes `interface/created` | `internal/plugins/ifacenetlink/monitor_test.go` |
| `interface { ... unit 0 { dhcp { enabled true } } }` | -> | iface-dhcp plugin starts DHCPv4 client | `test/plugin/iface-dhcp.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `backend netlink` (or default) | iface component loads iface-netlink backend, all management ops work as before |
| AC-2 | Config with `backend nonexistent` | Startup fails with error naming the unknown backend |
| AC-3 | `ze interface show` with netlink backend | Returns same InterfaceInfo output as today |
| AC-4 | Netlink link event (interface created) | Backend monitor publishes `interface/created` on Bus with correct LinkPayload |
| AC-5 | Config with DHCP enabled on a unit | iface-dhcp plugin starts, acquires lease, publishes `interface/dhcp/lease-acquired` |
| AC-6 | Backend interface defined in component | At least `CreateDummy`, `AddAddress`, `SetAdminUp`, `ListInterfaces`, `StartMonitor` methods |
| AC-7 | `make ze-verify` passes | No regressions from the split |
| AC-8 | Migration command (`ze interface migrate`) | Still works, calls backend via interface (not direct netlink) |
| AC-9 | Non-Linux build | Compiles. Backend stubs return "not supported". No panics. |
| AC-10 | Bridge operations (add-port, set-stp) | Work through backend interface, same behavior as today |
| AC-11 | Sysctl operations (IPv4/IPv6 settings) | Work through backend interface, same behavior as today |
| AC-12 | Mirror operations (ingress/egress) | Work through backend interface, same behavior as today |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBackendInterface` | `internal/component/iface/backend_test.go` | Backend interface compile check with mock | |
| `TestBackendSelection` | `internal/component/iface/register_test.go` | Correct backend loaded from config | |
| `TestBackendSelectionUnknown` | `internal/component/iface/register_test.go` | Error on unknown backend name | |
| `TestNetlinkBackendCreateDummy` | `internal/plugins/ifacenetlink/manage_test.go` | CreateDummy via backend in netns | |
| `TestNetlinkBackendAddAddress` | `internal/plugins/ifacenetlink/manage_test.go` | AddAddress via backend in netns | |
| `TestNetlinkBackendMonitor` | `internal/plugins/ifacenetlink/monitor_test.go` | Monitor publishes Bus events | |
| `TestDHCPPluginStartStop` | `internal/plugins/ifacedhcp/dhcp_test.go` | Plugin lifecycle | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MTU | 68-16000 | 16000 | 67 | 16001 |
| VLAN ID | 1-4094 | 4094 | 0 | 4095 |
| Unit ID | 0-16385 | 16385 | N/A | 16386 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-iface-backend-select` | `test/plugin/iface-backend-select.ci` | Config with backend leaf parsed correctly | |
| `test-iface-show` | `test/plugin/iface-show.ci` | `ze interface show` returns interface list | |
| `test-iface-dhcp` | `test/plugin/iface-dhcp.ci` | DHCP enabled, plugin starts | |

### Future (if deferring any tests)
- Integration tests requiring physical hardware or specific OS features (e.g., real DHCP server)
- Tests for future backends (networkd, FreeBSD) -- do not exist yet

## Files to Modify
- `internal/component/iface/register.go` - backend loading, OnConfigure dispatches to backend
- `internal/component/iface/iface.go` - remove DHCP types (move to dhcp plugin)
- `internal/component/iface/migrate_linux.go` - call backend interface instead of direct netlink
- `internal/component/iface/cmd/cmd.go` - dispatch via backend interface
- `internal/component/iface/schema/ze-iface-conf.yang` - add `backend` leaf
- `internal/plugin/all/all.go` - add blank imports for new plugins

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new leaf) | Yes | `internal/component/iface/schema/ze-iface-conf.yang` |
| CLI commands/flags | No | Existing commands unchanged, dispatch target changes |
| Editor autocomplete | Yes | Automatic from YANG update |
| Functional test for backend selection | Yes | `test/plugin/iface-backend-select.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - backend selection |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` - `backend` leaf |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` - iface-netlink, iface-dhcp |
| 6 | Has a user guide page? | Yes | `docs/features/interfaces.md` - backend architecture |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` - iface backend split |

## Files to Create
- `internal/component/iface/backend.go` - backend interface definition
- `internal/plugins/ifacenetlink/ifacenetlink.go` - netlink backend implementation hub
- `internal/plugins/ifacenetlink/register.go` - plugin registration
- `internal/plugins/ifacenetlink/manage_linux.go` - interface management (from component)
- `internal/plugins/ifacenetlink/manage_other.go` - non-Linux stubs (from component)
- `internal/plugins/ifacenetlink/monitor_linux.go` - netlink monitor (from component)
- `internal/plugins/ifacenetlink/monitor_other.go` - non-Linux stub (from component)
- `internal/plugins/ifacenetlink/show_linux.go` - interface query (from component)
- `internal/plugins/ifacenetlink/show_other.go` - non-Linux fallback (from component)
- `internal/plugins/ifacenetlink/bridge_linux.go` - bridge ops (from component)
- `internal/plugins/ifacenetlink/bridge_other.go` - non-Linux stub (from component)
- `internal/plugins/ifacenetlink/sysctl_linux.go` - sysctl writes (from component)
- `internal/plugins/ifacenetlink/mirror_linux.go` - tc mirror setup (from component)
- `internal/plugins/ifacenetlink/discover.go` - interface discovery (from component)
- `internal/plugins/ifacenetlink/schema/ze-iface-netlink-conf.yang` - netlink-specific config (if needed)
- `internal/plugins/ifacenetlink/schema/embed.go` - YANG embedding
- `internal/plugins/ifacenetlink/schema/register.go` - YANG registration
- `internal/plugins/ifacedhcp/ifacedhcp.go` - DHCP plugin hub
- `internal/plugins/ifacedhcp/register.go` - plugin registration
- `internal/plugins/ifacedhcp/dhcp_linux.go` - DHCP client lifecycle (from component)
- `internal/plugins/ifacedhcp/dhcp_v4_linux.go` - DHCPv4 worker (from component)
- `internal/plugins/ifacedhcp/dhcp_v6_linux.go` - DHCPv6 worker (from component)
- `internal/plugins/ifacedhcp/dhcp_other.go` - non-Linux stubs
- `internal/plugins/ifacedhcp/schema/ze-iface-dhcp-conf.yang` - DHCP-specific YANG (if needed)
- `internal/plugins/ifacedhcp/schema/embed.go` - YANG embedding
- `internal/plugins/ifacedhcp/schema/register.go` - YANG registration
- `test/plugin/iface-backend-select.ci` - backend selection functional test
- `test/plugin/iface-dhcp.ci` - DHCP plugin functional test

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

1. **Phase: Backend interface** -- define the backend interface in `internal/component/iface/backend.go`
   - Tests: `TestBackendInterface` (compile check with mock)
   - Files: `internal/component/iface/backend.go`
   - Verify: interface compiles, mock satisfies it

2. **Phase: Extract netlink plugin** -- move all netlink code to `internal/plugins/ifacenetlink/`
   - Tests: existing tests must still pass (just relocated)
   - Files: all `*_linux.go` and `*_other.go` files (manage, monitor, show, bridge, sysctl, mirror, discover)
   - Verify: `make ze-verify` passes, no import cycles

3. **Phase: Wire backend into component** -- component loads backend, dispatches through interface
   - Tests: `TestBackendSelection`, `TestBackendSelectionUnknown`
   - Files: `register.go`, `migrate_linux.go`, `cmd/cmd.go`
   - Verify: existing behavior preserved, backend is actually called

4. **Phase: YANG backend leaf** -- add `backend` leaf to `ze-iface-conf.yang`
   - Tests: `test/plugin/iface-backend-select.ci`
   - Files: `schema/ze-iface-conf.yang`
   - Verify: config with `backend netlink` parses, default works

5. **Phase: Extract DHCP plugin** -- move DHCP code to `internal/plugins/ifacedhcp/`
   - Tests: `TestDHCPPluginStartStop`
   - Files: dhcp_linux.go, dhcp_v4_linux.go, dhcp_v6_linux.go
   - Verify: DHCP events still published on correct Bus topics

6. **Phase: Plugin registration** -- register both new plugins, update `all/all.go`
   - Tests: `TestAllPluginsRegistered` count update
   - Files: `internal/plugin/all/all.go`, both `register.go` files
   - Verify: `make ze-verify`, plugins appear in `make ze-inventory`

7. **Functional tests** -- create after feature works. Cover user-visible behavior.
8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | All existing tests pass after file moves, no broken imports |
| Naming | Plugin names follow convention (`iface-netlink`, `iface-dhcp`), package names are `ifacenetlink`, `ifacedhcp` |
| Data flow | Config -> component -> backend interface -> netlink. No direct netlink calls in component. |
| Rule: no-layering | Old direct netlink code fully deleted from component after move |
| Rule: integration-completeness | Backend selection reachable from config, not just unit tests |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Backend interface in `internal/component/iface/backend.go` | `ls -la internal/component/iface/backend.go` |
| Netlink plugin at `internal/plugins/ifacenetlink/` | `ls internal/plugins/ifacenetlink/` |
| DHCP plugin at `internal/plugins/ifacedhcp/` | `ls internal/plugins/ifacedhcp/` |
| No netlink imports in `internal/component/iface/*.go` (except test) | `grep -r "vishvananda/netlink" internal/component/iface/` returns empty |
| `backend` leaf in YANG | `grep "leaf backend" internal/component/iface/schema/ze-iface-conf.yang` |
| Both plugins in `all/all.go` | `grep "ifacenetlink\|ifacedhcp" internal/plugin/all/all.go` |
| `make ze-verify` passes | Run and capture output |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Backend name from config validated against registered backends (no path traversal) |
| Interface name validation | Preserved from current code (validateIfaceName) |
| sysctl path injection | Interface names in sysctl paths still validated |
| DHCP input | DHCP responses from network still validated before use |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -- RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -- DESIGN phase |
| Functional test fails | Check AC; if AC wrong -- DESIGN; if AC correct -- IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |
| Import cycle after move | Restructure: ensure plugin imports component, never reverse |

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

- The FIB pattern works well for iface because the relationship is identical: shared config model + OS-specific implementation. The main difference is iface has more operation types (manage, monitor, bridge, sysctl, mirror) vs FIB's simpler (add/del/replace route).
- DHCP is cleanly separable because it's a protocol client that happens to use address operations. It communicates via Bus events already. Making it a separate plugin means non-DHCP deployments don't load the DHCP code.
- The `backend` YANG leaf with a default gives a zero-config upgrade path: existing configs work unchanged because `netlink` is the default.
- Migration stays in the component because it's orchestration logic (coordinate with BGP via Bus), not an OS operation. It calls the backend interface for the actual address moves.
- Discovery moves to the netlink plugin because interface type mapping is OS-specific (netlink link types to Ze types).

## RFC Documentation

N/A -- no RFC protocol work in this spec.

## Implementation Summary

### What Was Implemented
- [To be filled after implementation]

### Bugs Found/Fixed
- [To be filled after implementation]

### Documentation Updates
- [To be filled after implementation]

### Deviations from Plan
- [To be filled after implementation]

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
- [ ] AC-1..AC-12 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-iface-backend-split.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
