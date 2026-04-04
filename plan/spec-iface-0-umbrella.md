# Spec: iface-0 — Interface Management (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` — workflow rules
3. `plan/spec-arch-0-system-boundaries.md` — umbrella arch spec
4. `pkg/ze/bus.go` — Bus interface
5. Child specs: `spec-iface-1-monitor.md` through `spec-iface-4-advanced.md`

## Task

Add interface lifecycle management to Ze via the Bus. An **interface plugin** (one per OS) monitors and manages OS network interfaces, publishing events to hierarchical Bus topics. BGP and other subsystems subscribe to these events to react to address availability changes.

The primary use case is **make-before-break interface migration**: create a new interface, add an IP, wait for BGP to bind, remove the IP from the old interface, then remove the old interface — ensuring the IP is always reachable.

### Design Decision: JunOS-Style Logical Units

All interface types use a **two-layer model** inspired by JunOS IFD/IFL:
- **Physical layer** (the interface itself): MTU, description, disable, hardware properties
- **Logical layer** (units): IP addresses, VLAN ID, VRF binding, per-unit sysctl, firewall filters

Every interface has at least `unit 0`. Addresses are always configured on units, never directly on interfaces. VLAN units (with `vlan-id`) create Linux VLAN subinterfaces via netlink. Non-VLAN units share the parent OS interface (Linux supports multiple IPs natively).

| Ze concept | Linux mapping |
|-----------|---------------|
| `ethernet eth0 unit 0` (no vlan-id) | Addresses on `eth0` directly |
| `ethernet eth0 unit 100` (vlan-id 100) | `ip link add link eth0 name eth0.100 type vlan id 100` |
| `dummy lo0 unit 0` | Addresses on `lo0` directly |
| `dummy lo0 unit 5` (no vlan-id) | Addresses on `lo0` (alias grouping, no OS subinterface) |
| `loopback unit 0` | Addresses on `lo` directly |

### Design Decision: Plugin per OS

Interface management is implemented as a **plugin** (not a subsystem). Implementation is **Linux-only** for now, using `_linux.go` file suffixes so macOS/BSD support can be added later without restructuring.

| OS | Plugin | Mechanism | Status |
|----|--------|-----------|--------|
| Linux | `iface` (`_linux.go`) | Netlink (`vishvananda/netlink`) — multicast monitoring | This spec set |
| macOS | `iface` (`_darwin.go`) | Route sockets (`syscall.AF_ROUTE`) | Future |
| BSD | `iface` (`_bsd.go`) | Route sockets (similar to macOS) | Future |

### Scope

The plugin both **monitors all OS interfaces** (reacting to external changes) and **manages Ze-created interfaces** (creating/deleting on command). BGP reacts to any IP appearing or disappearing, regardless of who created it.

## Child Specs

| Phase | Spec | Scope | Depends |
|-------|------|-------|---------|
| 1 | `spec-iface-1-monitor.md` | Plugin registration + Linux netlink monitor + Bus publishing (read-only) | iface-0 |
| 2 | `spec-iface-2-manage.md` | Interface create/delete/addr + YANG config + CLI | iface-1 |
| 3 | `spec-iface-3-bgp-react.md` | BGP reacts to addr events (listener start/stop, drain, `local-address` by name) | iface-1 |
| 4 | `spec-iface-4-advanced.md` | DHCP, make-before-break migration, traffic mirroring, SLAAC | iface-2, iface-3 |
| 5 | `spec-iface-5-vm-tests.md` | Linux integration tests (netlink, tc, sysctl, DHCP) in network namespaces | iface-4 |

Phases 2 and 3 are independent and can proceed in parallel.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-arch-0-system-boundaries.md` — Bus, Subsystem, Plugin boundaries
  → Decision: Bus is content-agnostic, payload always `[]byte`, topics hierarchical with `/`
  → Decision: Plugins extend subsystem behavior by reacting to bus events
  → Constraint: Plugin infrastructure MUST NOT import plugin implementations — use registry
- [ ] `docs/architecture/core-design.md` — current engine + plugin architecture
  → Constraint: Bus never type-asserts payloads
- [ ] `.claude/rules/plugin-design.md` — plugin registration, 5-stage protocol, import rules
  → Constraint: registration via `init()` in `register.go`, auto-discovered through registry

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` — BGP-4: TCP connection binding, session establishment
  → Constraint: BGP binds to specific local addresses per peer (Section 8)
- [ ] `rfc/short/rfc4724.md` — Graceful Restart: session preservation across restarts
  → Constraint: GR allows session survival during interface migration if forwarding state preserved

**Key insights:**
- Bus topics are hierarchical strings; prefix subscriptions match all subtopics
- Plugins register via `init()` + `register.go`, discovered through registry
- BGP already has per-peer `LocalAddress` binding — the interface plugin provides the "when is this address available?" signal
- Make-before-break requires ordering guarantees: new IP confirmed usable before old IP removed

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/ze/bus.go` — Bus interface: `CreateTopic`, `Publish`, `Subscribe`, `Unsubscribe`. Event has Topic, Payload (`[]byte`), Metadata (`map[string]string`)
- [ ] `pkg/ze/subsystem.go` — Subsystem interface: `Name`, `Start(ctx, Bus, ConfigProvider)`, `Stop`, `Reload`
- [ ] `internal/component/engine/engine.go` — Engine starts plugins first, then subsystems. Stops in reverse
- [ ] `internal/bus/bus.go` — Bus implementation with hierarchical topics, prefix matching, per-consumer delivery goroutine
- [ ] `internal/component/bgp/reactor/listener.go` — `Listener` wraps `net.ListenConfig`, bound to `"addr:port"` strings
- [ ] `internal/component/bgp/reactor/reactor.go` — `startListenerForAddressPort(addr, port, peerKey)` creates per-address listeners
- [ ] `internal/core/network/network.go` — `RealDialer` with optional `LocalAddr` for outbound connections
- [ ] `internal/component/plugin/registry/` — plugin registration via `init()`, `Register()` function

**Behavior to preserve:**
- BGP per-peer `LocalAddress` binding via `net.ListenConfig`
- Bus content-agnostic — payload is `[]byte`, bus never type-asserts
- Plugin registration pattern via `init()` + `register.go`
- Engine startup order: plugins first, then subsystems

**Behavior to change:**
- BGP currently assumes configured IPs exist — no verification or reactive binding
- No interface lifecycle events exist on the Bus today
- No OS interface management capability exists

## Data Flow (MANDATORY)

### Entry Points

| Source | Entry | Format |
|--------|-------|--------|
| OS kernel | Netlink multicast (Linux) / route socket (macOS) | Kernel netlink messages |
| Config | YANG `ze-iface-conf` | Config tree → `map[string]any` |
| CLI | `ze interface create/delete/migrate` | Command arguments |

### Transformation Path

1. **Kernel event** — netlink multicast delivers `RTM_NEWLINK`, `RTM_DELLINK`, `RTM_NEWADDR`, `RTM_DELADDR`
2. **Event classification** — plugin maps netlink message type to Bus topic
3. **Payload encoding** — JSON `[]byte` (kebab-case per `rules/json-format.md`)
4. **Bus publish** — `bus.Publish(topic, payload, metadata)`
5. **BGP subscription** — reactor's `interface/` consumer receives event, checks peer `LocalAddress`
6. **BGP reaction** — on `addr/added`: start listener. On `addr/removed`: drain sessions

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| OS ↔ Interface Plugin | Netlink socket (Linux) / route socket (macOS) | [ ] |
| Interface Plugin ↔ Bus | `bus.Publish(topic, []byte, metadata)` | [ ] |
| Bus ↔ BGP Subsystem | `consumer.Deliver([]Event)` | [ ] |
| BGP ↔ Peers | `net.ListenConfig` / `net.Dialer` binding | [ ] |

### Integration Points
- `internal/component/plugin/registry/` — interface plugin registers here
- `pkg/ze/bus.go` — `Bus.Publish`, `Bus.Subscribe`
- `internal/component/bgp/reactor/listener.go` — BGP starts/stops listeners
- `internal/component/bgp/reactor/reactor_bus.go` — reactor Bus subscription (from spec-reactor-bus-subscribe)

### Architectural Verification
- [ ] No bypassed layers (interface plugin → Bus → BGP, never direct)
- [ ] No unintended coupling (BGP never imports interface plugin)
- [ ] No duplicated functionality (extends existing Bus, not new IPC)
- [ ] Zero-copy preserved where applicable (Bus payload is `[]byte`)

## Bus Topics (shared reference for all children)

| Topic | Published When | Payload Fields |
|-------|---------------|----------------|
| `interface/created` | Interface appeared | `name`, `type`, `index`, `mtu` |
| `interface/deleted` | Interface removed | `name`, `index` |
| `interface/up` | Link state → up | `name`, `index` |
| `interface/down` | Link state → down | `name`, `index` |
| `interface/addr/added` | IP assigned (DAD complete for IPv6) | `name`, `unit`, `index`, `address`, `prefix-length`, `family` |
| `interface/addr/removed` | IP removed | `name`, `unit`, `index`, `address`, `prefix-length`, `family` |
| `interface/dhcp/lease-acquired` | DHCPv4 lease obtained | `name`, `unit`, `address`, `prefix-length`, `router`, `dns`, `lease-time` |
| `interface/dhcp/lease-renewed` | DHCPv4 lease renewed | Same as above |
| `interface/dhcp/lease-expired` | DHCPv4 lease expired | `name`, `unit`, `address` |
| `interface/dhcpv6/lease-acquired` | DHCPv6 lease/PD obtained | `name`, `unit`, `address` or `prefix`, `prefix-length`, `lease-time` |
| `interface/dhcpv6/lease-expired` | DHCPv6 lease expired | `name`, `unit`, `address` or `prefix` |

### Metadata for Filtering

| Key | Value | Purpose |
|-----|-------|---------|
| `name` | Interface name (e.g., `"eth0"`) | Filter by physical interface |
| `unit` | Unit ID as string (e.g., `"0"`, `"100"`) | Filter by logical unit |
| `address` | IP address string (e.g., `"10.0.0.1"`) | BGP matches against peer `LocalAddress` |
| `family` | `"ipv4"` or `"ipv6"` | Address family filter |

### Payload Format (JSON, kebab-case)

All payloads follow `rules/json-format.md`:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Physical interface name |
| `unit` | integer | Logical unit ID |
| `index` | integer | OS interface index |
| `address` | string | IP address (no prefix) |
| `prefix-length` | integer | CIDR prefix length |
| `family` | string | `"ipv4"` or `"ipv6"` |
| `managed` | boolean | `true` if Ze created this interface |

## Interface Migration Protocol (overview)

Detailed in `spec-iface-4-advanced.md`. Five phases, strict ordering. Migration operates at the **unit level** -- moving an IP from one unit to another (possibly on a different physical interface):

| Phase | Action | Bus Event | BGP Reaction |
|-------|--------|-----------|-------------|
| 1 | Create new interface/unit | `interface/created` + `interface/up` | None |
| 2 | Add IP to new unit | `interface/addr/added` (with new unit) | Start listener, begin connections |
| 3 | Confirm BGP ready | `bgp/listener/ready` | N/A (BGP is publisher) |
| 4 | Remove IP from old unit | `interface/addr/removed` (with old unit) | Drain sessions |
| 5 | Remove old interface/unit | `interface/deleted` | No impact |

Phase 4 MUST NOT start until Phase 3 confirms BGP has established sessions on the new address.

## OS-Level Operations (shared reference)

### Linux (Netlink)

| Operation | Netlink Message | Key Attributes |
|-----------|-----------------|----------------|
| Create interface | `RTM_NEWLINK` + `NLM_F_CREATE` | `IFLA_IFNAME`, `IFLA_LINKINFO` |
| Create VLAN unit | `RTM_NEWLINK` + `NLM_F_CREATE` | `IFLA_LINKINFO` (type vlan), `IFLA_LINK` (parent index), `IFLA_IFNAME` (e.g., `eth0.100`) |
| Set interface up | `RTM_NEWLINK` | `ifi_change = IFF_UP`, `ifi_flags = IFF_UP` |
| Set MTU | `RTM_NEWLINK` | `IFLA_MTU` |
| Add IPv4 address | `RTM_NEWADDR` + `NLM_F_CREATE` | `IFA_LOCAL` + `IFA_ADDRESS` (both required) |
| Add IPv6 address | `RTM_NEWADDR` + `NLM_F_CREATE` | `IFA_ADDRESS` only |
| Remove address | `RTM_DELADDR` | `IFA_LOCAL` + `IFA_ADDRESS` (IPv4) or `IFA_ADDRESS` (IPv6) |
| Delete interface | `RTM_DELLINK` | `IFLA_IFNAME` |
| Delete VLAN unit | `RTM_DELLINK` | `IFLA_IFNAME` (e.g., `eth0.100`) |
| Monitor changes | Multicast groups | `RTMGRP_LINK`, `RTMGRP_IPV4_IFADDR`, `RTMGRP_IPV6_IFADDR` |

### Unit-to-Linux Mapping

| Unit config | Linux result | Notes |
|-------------|-------------|-------|
| `unit 0` (no vlan-id) | Addresses on parent interface | Default untagged unit |
| `unit N` with `vlan-id V` | `ip link add link <parent> name <parent>.V type vlan id V` | Separate OS subinterface |
| `unit N` without `vlan-id` (N>0) | Addresses on parent interface | Logical grouping only (VRF, policy), no OS subinterface |

Dependencies: `github.com/vishvananda/netlink` (3200+ stars), `github.com/insomniacslk/dhcp` (815+ stars, Phase 4 only)

### Key Netlink Details

- IPv4 `RTM_NEWADDR` requires both `IFA_LOCAL` and `IFA_ADDRESS`. IPv6 requires only `IFA_ADDRESS`. The `vishvananda/netlink` library abstracts this via `netlink.AddrAdd()`.
- VLAN subinterfaces: `netlink.LinkAdd(&netlink.Vlan{LinkAttrs: netlink.LinkAttrs{Name: "eth0.100", ParentIndex: parentIdx}, VlanId: 100})`

## Design Insights

### freeRouter Reference

| Pattern | freeRouter | Ze |
|---------|-----------|---|
| Interface creation | `tapInt.c` uses `/dev/net/tun`; `veth.c` uses netlink | Go netlink via `vishvananda/netlink` |
| IP assignment | Shells out `ip addr add` or netlink `RTM_NEWADDR` | Netlink `RTM_NEWADDR` (no shell) |
| State monitoring | Polls `SIOCGIFFLAGS` every 1s | Netlink multicast (async) |
| Interface removal | Process exit (kernel reclaims fd) | Explicit `RTM_DELLINK` |
| IP migration | Not supported | Make-before-break via Bus |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test | Phase |
|-------------|---|--------------|------|-------|
| Netlink event (address added) | → | Monitor publishes to Bus | `TestIfaceMonitorPublishesAddrAdded` | 1 |
| Config with `interface` section | → | Plugin creates OS interface | `TestIfacePluginCreatesInterface` | 2 |
| Bus event `interface/addr/added` | → | BGP starts listener | `TestBGPStartsListenerOnAddrAdded` | 3 |
| Bus event `interface/addr/removed` | → | BGP drains sessions | `TestBGPDrainsOnAddrRemoved` | 3 |
| Config reload with migration | → | Full make-before-break | `TestMakeBeforeBreakMigration` | 4 |

## Acceptance Criteria

| AC ID | Phase | Input / Condition | Expected Behavior |
|-------|-------|-------------------|-------------------|
| AC-1 | 1 | Interface plugin starts on Linux | Opens netlink socket, subscribes to multicast groups, begins monitoring |
| AC-2 | 1 | External IP added to OS interface | Plugin publishes `interface/addr/added` to Bus within 1 second |
| AC-3 | 2 | Config specifies managed interface | Plugin creates interface via netlink, brings up, assigns addresses |
| AC-4 | 3 | `interface/addr/added` for a peer's `LocalAddress` | BGP starts listener and attempts outbound connections |
| AC-5 | 3 | `interface/addr/removed` for active listener | BGP sends NOTIFICATION cease, drains, removes listener |
| AC-6 | 4 | Make-before-break migration | No period where IP is unreachable |
| AC-7 | 4 | DHCP enabled on interface | DHCPv4 client obtains lease, adds address, publishes event |
| AC-8 | 4 | DHCPv6 with PD enabled | DHCPv6 client obtains prefix delegation |
| AC-9 | 4 | DHCP lease expires | Address removed, events published |
| AC-10 | 4 | IPv6 autoconf enabled | sysctl set, SLAAC addresses detected and published |
| AC-11 | 4 | Traffic mirror configured | tc mirred action created |
| AC-12 | 4 | Traffic mirror removed | tc filter and qdisc removed |
| AC-13 | 1 | Interface plugin stops | Closes netlink socket cleanly |
| AC-14 | 3 | Multiple peers share `LocalAddress` | All react to events, shared listener |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Phase | Validates |
|------|------|-------|-----------|
| `TestBusTopicCreation` | `internal/component/iface/iface_test.go` | 1 | Plugin creates correct Bus topics |
| `TestNetlinkEventToTopic` | `internal/component/iface/monitor_linux_test.go` | 1 | Maps netlink types to Bus topics |
| `TestPayloadFormat` | `internal/component/iface/iface_test.go` | 1 | JSON payload matches spec |
| `TestIfaceCreate` | `internal/component/iface/iface_linux_test.go` | 2 | Creates interface via netlink |
| `TestSysctlAutoconf` | `internal/component/iface/sysctl_linux_test.go` | 2 | IPv6 sysctl writes correct values |
| `TestBGPAddrAddedReaction` | `internal/component/bgp/reactor/reactor_iface_test.go` | 3 | Listener started on matching addr event |
| `TestBGPAddrRemovedReaction` | `internal/component/bgp/reactor/reactor_iface_test.go` | 3 | Sessions drained on addr removed |
| `TestDHCPLeaseToEvent` | `internal/component/iface/dhcp_linux_test.go` | 4 | DHCP lease publishes correct events |
| `TestMirrorSetup` | `internal/component/iface/mirror_linux_test.go` | 4 | tc mirred filter created |
| `TestMigrationOrdering` | `internal/component/bgp/reactor/reactor_iface_test.go` | 4 | Old IP not removed until new confirmed |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MTU | 68-65535 | 65535 | 67 | 65536 |
| Prefix length IPv4 | 0-32 | 32 | N/A | 33 |
| Prefix length IPv6 | 0-128 | 128 | N/A | 129 |
| VLAN ID | 1-4094 | 4094 | 0 | 4095 |
| Unit ID | 0-16385 | 16385 | N/A (0 is valid) | 16386 |
| Interface name | 1-15 chars (IFNAMSIZ-1) | 15 chars | empty | 16 chars |

### Functional Tests

| Test | Location | Phase | End-User Scenario |
|------|----------|-------|-------------------|
| `test-iface-monitor` | `test/plugin/iface-monitor.ci` | 1 | External IP change triggers Bus event |
| `test-iface-create` | `test/plugin/iface-create.ci` | 2 | Config creates dummy interface |
| `test-iface-bgp-bind` | `test/plugin/iface-bgp-bind.ci` | 3 | BGP session starts after interface IP added |
| `test-iface-dhcp` | `test/plugin/iface-dhcp.ci` | 4 | DHCP client obtains lease |
| `test-iface-mirror` | `test/plugin/iface-mirror.ci` | 4 | Traffic mirroring configured |
| `test-iface-migrate` | `test/plugin/iface-migrate.ci` | 4 | Full make-before-break migration |

## Files to Modify

- `internal/component/bgp/reactor/reactor.go` — Bus subscription for `interface/` events (Phase 3)
- `internal/component/bgp/reactor/listener.go` — `startListenerForAddress` / `stopListenerForAddress` (Phase 3)
- `internal/component/bgp/schema/ze-bgp-conf.yang` — `local-address` accepts interface names (Phase 3)
- `internal/component/plugin/all/all.go` — blank import for `iface` plugin (Phase 1, auto-generated)
- `go.mod` — add `github.com/vishvananda/netlink` (Phase 1), `github.com/insomniacslk/dhcp` (Phase 4)

## Files to Create

| File | Phase | Purpose |
|------|-------|---------|
| `internal/component/iface/iface.go` | 1 | Shared types, Bus topic constants, payload encoding |
| `internal/component/iface/register.go` | 1 | `init()` → `registry.Register()` |
| `internal/component/iface/monitor_linux.go` | 1 | Netlink multicast monitor goroutine |
| `internal/component/iface/iface_linux.go` | 2 | Interface create/delete/addr management |
| `internal/component/iface/sysctl_linux.go` | 2 | sysctl writes for IPv4/IPv6 options |
| `internal/component/iface/schema/ze-iface-conf.yang` | 2 | YANG config schema |
| `cmd/ze/iface/main.go` | 2 | CLI subcommand dispatch |
| `cmd/ze/iface/show.go` | 2 | `ze interface show` |
| `cmd/ze/iface/create.go` | 2 | `ze interface create` |
| `cmd/ze/iface/addr.go` | 2 | `ze interface addr add/del` |
| `internal/component/iface/dhcp_linux.go` | 4 | DHCPv4/v6 client |
| `internal/component/iface/mirror_linux.go` | 4 | tc mirred setup |
| `cmd/ze/iface/migrate.go` | 4 | `ze interface migrate` |

### Integration Checklist

| Integration Point | Needed? | File | Phase |
|-------------------|---------|------|-------|
| YANG schema (new module) | [x] | `internal/component/iface/schema/ze-iface-conf.yang` | 2 |
| YANG schema (BGP update) | [x] | `internal/component/bgp/schema/ze-bgp-conf.yang` | 3 |
| CLI commands/flags | [x] | `cmd/ze/iface/main.go` | 2 |
| API commands doc | [x] | `docs/architecture/api/commands.md` | 2 |
| Functional tests | [x] | `test/plugin/iface-*.ci` | 1-4 |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — interface management |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` — interface stanzas |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` — `ze interface` |
| 4 | API/RPC added/changed? | No | — |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` — iface plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/interfaces.md` — new |
| 7 | Wire format changed? | No | — |
| 8 | Plugin SDK/protocol changed? | No | — |
| 9 | RFC behavior implemented? | No | — |
| 10 | Test infrastructure changed? | No | — |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` — interface management |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` — interface plugin |

## Implementation Steps

Implementation follows child specs in order. Phases 2 and 3 can proceed in parallel after Phase 1.

1. Phase 1: `spec-iface-1-monitor.md` — plugin + netlink monitor + Bus publishing
2. Phase 2: `spec-iface-2-manage.md` — interface management + YANG + CLI
3. Phase 3: `spec-iface-3-bgp-react.md` — BGP reactions to interface events
4. Phase 4: `spec-iface-4-advanced.md` — DHCP, migration, mirroring, SLAAC

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in phase that introduced it |
| Test fails wrong reason | Fix test assertion |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to IMPLEMENT for that criterion |

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

(Moved to top-level sections: freeRouter Reference, Key Netlink Detail)

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from plan and why]

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
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass — defer with user approval)
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

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-iface-0-umbrella.md`
- [ ] **Summary included in commit**
