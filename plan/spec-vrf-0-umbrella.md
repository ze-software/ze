# Spec: vrf-0 -- VRF as first-class organising principle (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/component/bgp/reactor/reactor.go` -- Reactor struct and constructor
4. `internal/component/bgp/plugins/rib/rib.go` -- RIB plugin
5. `internal/component/plugin/server/` -- plugin server and hub
6. `internal/component/plugin/inprocess.go` -- plugin runner (per-VRF spawning)
7. `internal/component/iface/register.go` -- component registration pattern
8. `internal/plugins/fibkernel/register.go` -- FIB plugin pattern
9. Child specs: `spec-vrf-1-*` through `spec-vrf-N-*`

## Task

Make VRF the organising principle of ze. Every component (BGP, interfaces, firewall, static
routes, SSH, NTP, etc.) runs inside a VRF. The default VRF (table 254) is implicit: top-level
config elements belong to it without a wrapper. Additional VRFs are declared explicitly. Each
VRF spawns its own set of component instances as separate plugin processes.

This is the architectural foundation for replacing VyOS on the Exa LNS, where the Surfprotect
content filter runs in a separate routing domain (table 100) with its own tunnel interface
and default route, while BGP peering and subscriber routing happen in the default VRF.

### Design Decisions (agreed with user)

| Decision | Detail |
|----------|--------|
| VRF is a full stack | Each VRF gets its own BGP, interfaces, firewall, RIB, fibkernel, services. Not just a RIB namespace |
| Universal abstraction | The current global stack is the implicit default VRF (table 254). Named VRFs are additional instances. Ze can run VRF-only with no non-VRF path |
| Process model | One plugin process per component per VRF. Each gets its own RunEngine call with VRF-scoped config via existing SDK. Easier to understand and debug than shared-process multi-VRF state |
| Implicit default VRF | Top-level config elements (no vrf wrapper) belong to vrf default (table 254). Backwards compatible. Engine wraps top-level config in synthetic vrf default internally |
| Plugin naming | Colon-separated: `bgp:vrf-red`, `fib-kernel:vrf-blue` |
| Lifecycle | Config at startup + dynamic create/delete at runtime |
| CLI surface | `show vrf <name> <command>` -- VRF intercepts, extracts name, forwards to the correct instance |
| YANG wrapping | VRF plugin takes the YANG schemas of its child modules and wraps them in `vrf <name> { ... }`. Default VRF keeps unwrapped YANG |
| Hub instantiation | Each VRF gets its own hub instance. VRF plugin is a hub-of-hubs |
| RIB independence | RIB code works unchanged -- it connects to whichever hub it is given. With VRF plugin: per-VRF hub. Without: global hub directly |
| Kernel VRF devices | Non-default VRFs get `ip link add <name> type vrf table <N>`. Default VRF has no device (table 254 is always available) |
| Service socket binding | Services use SO_BINDTODEVICE to bind listeners/dialers to VRF device. Default VRF: no binding needed |
| Policy routing | Cross-VRF mechanism. Lives outside VRF hierarchy. nftables marks + ip rules steer flows between VRFs |
| Route redistribution | Cross-VRF EventBus subscriptions. Redistribution plugin bridges source VRF topics to destination VRF sysRIB |
| EventBus scoping | Topics gain VRF name segment: `rib/best-change/<vrf-name>/bgp`. Prefix matching still works. Cross-VRF subscription natural |
| VRF context delivery | Via config fields (vrf.name, vrf.table, vrf.device), not SDK API changes. Components read from config. Components that don't care ignore the fields |
| Firewall table prefix | Non-default VRFs use `ze_<vrf>_` prefix. Default VRF keeps `ze_` for backwards compatibility |

### Scope

**In Scope:**

| Area | Description |
|------|-------------|
| Reactor multi-instance | Instantiate reactor N times in the same process (analysis shows all global state is safe to share) |
| Hub multi-instance | Hub and ProcessManager support multiple independent instances with derived plugin names |
| VRF orchestrator | Component that creates/destroys per-VRF stacks (all components spawned per VRF) |
| VRF config | YANG schema for VRF definition. Implicit default VRF for top-level config |
| VRF kernel devices | `ip link add type vrf table N`, interface master binding, sysctl |
| VRF CLI | `show vrf <name> <command>` routing, VRF-aware command dispatch |
| Dynamic lifecycle | Runtime VRF create/delete with graceful drain and cleanup |
| Metrics scoping | Per-VRF metric labels (vrf="default", vrf="surfprotect") |
| Service socket binding | SO_BINDTODEVICE for SSH, HTTPS API, NTP, syslog per VRF |
| Cross-VRF route leaking | Route redistribution between VRF sysRIB instances via EventBus |
| Policy routing integration | `then { vrf <name> }` resolves to VRF table, fwmark + ip rule |

**Out of Scope:**

| Area | Reason |
|------|--------|
| Route-target based VRF assignment | Phase 2 -- config-based peer-to-VRF mapping first |
| MPLS/VPNv4 label distribution | Requires MPLS infrastructure |
| Per-subscriber VRFs (BNG) | Future, requires L2TP completion |

### Child Specs

| Phase | Spec | Scope | Depends |
|-------|------|-------|---------|
| 1 | `spec-vrf-1-hub-multi.md` | Hub and ProcessManager support multiple instances. Colon-separated naming (`bgp:vrf-red`). Plugin startup per hub | - |
| 2 | `spec-vrf-2-device.md` | VRF device management in iface component. `ip link add type vrf table N`, interface master binding, per-VRF sysctl | vrf-1 |
| 3 | `spec-vrf-3-orchestrator.md` | VRF orchestrator: config parsing, implicit default VRF, per-VRF component spawning via existing plugin machinery (inprocess.go), lifecycle management | vrf-2 |
| 4 | `spec-vrf-4-yang-cli.md` | YANG wrapping (`vrf <name> { ... }`), CLI command routing (`show vrf <name> ...`), VRF-aware dispatch | vrf-3 |
| 5 | `spec-vrf-5-services.md` | VRF-aware service sockets. SO_BINDTODEVICE helper in internal/core/vrfnet/. SSH, HTTPS, NTP, syslog VRF binding | vrf-3 |
| 6 | `spec-vrf-6-dynamic.md` | Runtime VRF create/delete. Graceful drain. Hot reconfiguration | vrf-4 |
| 7 | `spec-vrf-7-redistribution.md` | Route redistribution between VRFs. Import/export policy. Cross-VRF EventBus subscriptions | vrf-3 |

Note: Reactor multi-instance was analyzed and requires no code changes -- all global state is safe to share across VRF instances. The reactor is already instantiable N times.

Phases are strictly ordered within dependencies. Independent specs (5, 7) can proceed in parallel after their dependencies.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- overall architecture, reactor, plugin model
  --> Constraint: reactor is the central event loop; plugins connect via hub
- [ ] `.claude/rules/plugin-design.md` -- plugin registration, 5-stage protocol, proximity principle
  --> Constraint: registration via `init()` in `register.go`, blank import is only coupling
- [ ] `.claude/rules/goroutine-lifecycle.md` -- goroutine patterns
  --> Constraint: long-lived workers only, no per-event goroutines in hot paths
- [ ] `internal/component/plugin/inprocess.go` -- plugin runner
  --> Constraint: creates net.Conn pair, calls RunEngine(conn). VRF orchestrator reuses this
- [ ] `internal/core/events/bus.go` -- EventBus
  --> Constraint: topic-based pub/sub with prefix matching
- [ ] `internal/plugins/fibkernel/backend_linux.go` -- route programming
  --> Constraint: netlink.Route.Table field exists but unused (defaults to main table)
- [ ] `vendor/github.com/vishvananda/netlink/link.go` -- VRF device creation
  --> Constraint: netlink.Vrf{Table: N} for VRF device type
- [ ] `vendor/github.com/vishvananda/netlink/rule.go` -- ip rule management
  --> Constraint: Rule struct with Priority, Table, Mark, IifName fields
- [ ] `plan/spec-static-routes.md` -- static routes with table ID (forward-compatible)
- [ ] `plan/spec-policy-routing.md` -- cross-VRF flow steering (fwmark + ip rule)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4364.md` -- BGP/MPLS IP VPNs (L3VPN)
  --> Constraint: VRF is per-PE, route distinguisher makes routes unique across VRFs
- [ ] `rfc/short/rfc4271.md` -- BGP-4: session management, UPDATE processing
  --> Constraint: each BGP session belongs to exactly one routing context

**Key insights:**
- VRF is a universal abstraction -- the current ze without VRF is equivalent to a single unnamed VRF (table 254)
- Each VRF spawns separate plugin instances via the existing SDK/inprocess machinery
- The VRF orchestrator is a hub-of-hubs, not a data-plane component
- Cross-VRF route leaking uses EventBus (all VRFs share the same process and Bus)
- Linux VRF: `ip link add type vrf table N`, `ip link set iface master vrfdev`, `SO_BINDTODEVICE`
- Default VRF (table 254) needs no VRF device -- sockets and routes use main table naturally
- Policy routing is inherently cross-VRF: nftables marks packets, ip rules steer to VRF tables

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor.go` -- Reactor struct, `New()` constructor, event loop
- [ ] `internal/component/bgp/reactor/session.go` -- global `bufMuxStd`, `bufMuxExt` buffer pools
- [ ] `internal/component/bgp/reactor/forward_pool.go` -- global `fwdWriteDeadlineNs` atomic
- [ ] `internal/component/bgp/reactor/forward_build.go` -- global `modBufPool` sync.Pool
- [ ] `internal/component/bgp/reactor/received_update.go` -- global `msgIDCounter` atomic
- [ ] `internal/component/bgp/plugins/rib/rib.go` -- RIBManager, RunRIBPlugin entry point
- [ ] `internal/component/bgp/plugins/rib/register.go` -- RIB plugin registration
- [ ] `internal/component/plugin/server/hub.go` -- Hub, SchemaRegistry, command routing
- [ ] `internal/component/plugin/process/manager.go` -- ProcessManager, `map[string]*Process`
- [ ] `internal/component/hub/hub.go` -- Orchestrator, `NewOrchestrator()`

**Behavior to preserve:**
- Single-instance Ze works exactly as today (unnamed default VRF)
- RIB plugin code unchanged -- connects to whichever hub it receives
- Plugin 5-stage startup protocol unchanged per hub instance
- DirectBridge zero-copy event delivery unchanged
- All existing config, CLI, YANG unchanged for non-VRF deployments

**Behavior to change:**
- Reactor global state (buffer pools, counters, deadlines) moved to per-instance
- ProcessManager supports derived plugin names (`bgp-rib:vrf-red`)
- Hub instantiable as independent instances with separate plugin sets
- New VRF plugin orchestrates per-VRF stack lifecycle
- CLI gains `vrf <name>` prefix for VRF-scoped commands
- YANG schemas wrapped in `vrf <name> {}` container for named VRFs

## Data Flow (MANDATORY)

### Entry Points

| Entry | Format | VRF impact |
|-------|--------|------------|
| BGP wire (TCP) | Binary BGP messages | Per-VRF reactor has own TCP listeners, own peer set |
| CLI command | Text (`show vrf red rib status`) | VRF plugin extracts name, routes to correct instance |
| Config file | YANG-modeled tree | VRF block contains nested peer/RIB/interface config |
| Plugin events | JSON/structured events over DirectBridge | Per-VRF hub delivers events to per-VRF plugins only |
| Runtime API | `vrf add red` / `vrf delete red` | VRF plugin creates/destroys stacks dynamically |

### Transformation Path

1. Config parse: `vrf <name> { bgp { peer { ... } } }` extracted per VRF
2. VRF plugin receives per-VRF config blocks
3. VRF plugin creates per-VRF hub + reactor + RIB plugin instances
4. Each reactor starts its own TCP listeners, manages its own peers
5. Each reactor's events flow to its own hub, delivered to its own RIB instance
6. CLI commands with `vrf <name>` prefix routed to the correct VRF's command handlers

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config -> VRF plugin | VRF config block extracted by config parser | [ ] |
| VRF plugin -> per-VRF reactor | VRF plugin calls reactor constructor with per-VRF config | [ ] |
| VRF plugin -> per-VRF hub | VRF plugin creates hub, passes to reactor/plugins | [ ] |
| CLI -> VRF plugin -> per-VRF handler | Command dispatch strips VRF prefix, forwards remainder | [ ] |
| Cross-VRF (future) | Goroutine-safe channel or function call between RIB instances | [ ] |

### Integration Points
- `Reactor.New()` -- must accept hub reference instead of assuming global
- `ProcessManager` -- must support derived names and multiple instances
- CLI dispatch -- must recognize `vrf <name>` prefix and route accordingly
- Config parser -- must support `vrf { ... }` blocks containing nested config
- YANG schema registry -- must support per-VRF schema wrapping

### Architectural Verification
- [ ] No bypassed layers (VRF routes through hub, not direct cross-VRF calls)
- [ ] No unintended coupling (VRF instances share only the process, not state)
- [ ] No duplicated functionality (VRF reuses existing reactor, RIB, hub code)
- [ ] Zero-copy preserved (per-VRF DirectBridge, per-VRF buffer pools)

## Reactor Global State Analysis

The following global state in `internal/component/bgp/reactor/` was analyzed for VRF multi-instance support. This is the scope of `spec-vrf-1-reactor-multi.md`.

### All Global State Is Safe to Share

| Global | File | Type | Reason |
|--------|------|------|--------|
| `bufMuxStd` | `session.go:53` | Pool + budget | Same process, same memory, global budget is correct |
| `bufMuxExt` | `session.go:58` | Pool + budget | Same as above |
| `modBufPool` | `forward_build.go:18` | `sync.Pool` | Stateless buffer pool, no contention concern |
| `fwdWriteDeadlineNs` | `forward_pool.go:53` | `atomic.Int64` | Global tuning knob, not per-VRF |
| `msgIDCounter` | `received_update.go:20` | `atomic.Uint64` | Monotonic process-wide sequence number, non-contiguous IDs per VRF is fine |
| Loggers | Various | Lazy loggers | Read-only, safe to share |
| Error sentinels | Various | `var Err*` | Read-only, safe to share |

### Already Per-Instance (no change needed)

| Component | Status |
|-----------|--------|
| `Reactor.peers` map | Per-instance |
| `Reactor.listeners` map | Per-instance |
| `Reactor.fwdPool` | Per-instance |
| `Reactor.recentUpdates` | Per-instance |
| `Reactor.eventDispatcher` | Per-instance |
| `Reactor.config` | Per-instance (injected) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `vrf red { bgp { peer { ... } } }` | -> | VRF plugin creates reactor+RIB for VRF red | `test/plugin/vrf-basic.ci` |
| `show vrf red rib status` CLI command | -> | VRF dispatches to red's RIB handler | `test/plugin/vrf-cli.ci` |
| BGP UPDATE on VRF red's peer | -> | VRF red's reactor processes, delivers to VRF red's RIB | `test/plugin/vrf-update.ci` |
| `vrf add blue` runtime command | -> | VRF plugin creates new stack dynamically | `test/plugin/vrf-dynamic.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Reactor constructed twice in same process | Both instances operate independently, no shared mutable state |
| AC-2 | Config with two named VRFs | Two independent component stacks running (each with own plugins) |
| AC-3 | Config with no VRF block | Ze operates exactly as today (implicit default VRF, table 254) |
| AC-4 | `show vrf surfprotect route` | Command reaches surfprotect's sysRIB, returns surfprotect's routes only |
| AC-5 | `show route` (no VRF prefix) | Command reaches default sysRIB (backwards compatible) |
| AC-6 | BGP UPDATE on default VRF peer | Processed by default reactor, stored in default RIB only |
| AC-7 | Static route in surfprotect VRF | Route programmed in table 100, not table 254 |
| AC-8 | `vrf add green` at runtime | VRF device created, components spawned, ready for config |
| AC-9 | `vrf delete surfprotect` at runtime | Child plugins stopped, routes cleaned, VRF device deleted |
| AC-10 | VRF orchestrator wraps child YANG | `vrf surfprotect { interface { ... } }` appears in schema |
| AC-11 | Two VRFs with same peer IP on different ports | No conflict -- each reactor has own VRF-bound listener |
| AC-12 | Metrics from VRF surfprotect | Labeled with vrf="surfprotect", distinguishable from default |
| AC-13 | SSH configured in default VRF only | SSH listens without SO_BINDTODEVICE, not reachable from other VRFs |
| AC-14 | SSH configured inside vrf management | SSH listener bound to management VRF device |
| AC-15 | Policy routing with `vrf surfprotect` action | Resolves to table 100, fwmark + ip rule created |
| AC-16 | fibkernel in surfprotect VRF | Programs routes with Table=100 in netlink calls |
| AC-17 | Config reload adds a VRF | VRF device created, components spawned |
| AC-18 | Config reload removes a VRF | Components stopped, kernel state cleaned, VRF device deleted |
| AC-19 | Implicit default + explicit VRF coexist | Top-level config in default VRF, explicit VRF runs in parallel |
| AC-20 | VRF surfprotect has no BGP config | No BGP instance spawned for surfprotect (only components with config) |

## Open Questions

| # | Question | Options | Status |
|---|----------|---------|--------|
| Q1 | Peer-to-VRF mapping model | Config-based (one peer = one VRF), route-target (one peer feeds multiple VRFs via RT communities), or both | Config-based first. Route-targets out of scope |
| Q2 | ~~Process model~~ | ~~Goroutine-based shared state vs separate plugin instances~~ | **Resolved:** one process per component per VRF. Easier to understand and debug |
| Q3 | ~~Implicit default~~ | ~~Explicit wrapper required vs top-level = default~~ | **Resolved:** implicit default. Top-level config belongs to vrf default (table 254). Can change later to explicit-only |
| Q4 | ~~Service socket binding~~ | ~~SO_BINDTODEVICE vs tcp_l3mdev_accept vs multiple listeners~~ | **Resolved:** SO_BINDTODEVICE. Per-socket, per-service, no global sysctl |
| Q5 | VRF-specific sysctl | Per-VRF `net.ipv4.conf.<vrfdev>.forwarding` etc. managed by sysctl plugin or VRF orchestrator? | Unresolved. Likely sysctl plugin with VRF context |
| Q6 | EventBus per-VRF vs shared | Separate Bus per VRF (stronger isolation) vs shared Bus with topic scoping (simpler cross-VRF) | Leaning shared Bus with VRF-scoped topics |

## Per-VRF Component Spawning

The VRF orchestrator spawns plugin instances using existing inprocess.go machinery.
Each component gets its own RunEngine call with VRF-scoped config:

```
VRF Orchestrator
│
├── vrf "default" (table 254, no VRF device)
│   ├── spawn("bgp",        config=bgp_subtree,     vrf={name:"default", table:254, device:""})
│   ├── spawn("interface",   config=iface_subtree,   vrf={name:"default", table:254, device:""})
│   ├── spawn("firewall",    config=fw_subtree,      vrf={name:"default", table:254, device:""})
│   ├── spawn("fib-kernel",  config=fib_subtree,     vrf={name:"default", table:254, device:""})
│   ├── spawn("rib",         config=rib_subtree,     vrf={name:"default", table:254, device:""})
│   ├── spawn("ssh",         config=ssh_subtree,     vrf={name:"default", table:254, device:""})
│   └── spawn("ntp",         config=ntp_subtree,     vrf={name:"default", table:254, device:""})
│
├── vrf "surfprotect" (table 100, device "surfprotect")
│   ├── spawn("interface",    config=iface_subtree,  vrf={name:"surfprotect", table:100, device:"surfprotect"})
│   ├── spawn("staticroute",  config=static_subtree, vrf={name:"surfprotect", table:100, device:"surfprotect"})
│   └── spawn("fib-kernel",   config=fib_subtree,    vrf={name:"surfprotect", table:100, device:"surfprotect"})
│
└── cross-VRF (no VRF context, orchestrator-level)
    ├── policy routing (nftables + ip rules, steers flows between VRFs)
    └── route redistribution (EventBus cross-VRF subscriptions)
```

VRF context is delivered as config fields, not SDK API changes:

```json
{
    "vrf": { "name": "surfprotect", "table": 100, "device": "surfprotect" },
    "interface": { "tun100": { ... } },
    "static": { ... }
}
```

Components read vrf.table or vrf.device from config. Components that don't need VRF
awareness ignore these fields. No SDK changes needed.

### Per-VRF component behaviour

| Component | VRF context usage |
|-----------|-------------------|
| **fibkernel** | Route.Table = vrf.table in netlink calls |
| **sysRIB** | Publishes to rib/best-change/<vrf.name>/... |
| **BGP** | Binds TCP sessions via SO_BINDTODEVICE(vrf.device) if non-default |
| **interfaces** | Calls `ip link set <iface> master <vrf.device>` for non-default VRF |
| **firewall** | Table prefix ze_<vrf.name>_ for non-default, ze_ for default |
| **static routes** | Route.Table = vrf.table |
| **SSH** | SO_BINDTODEVICE(vrf.device) on listener |
| **HTTPS API** | SO_BINDTODEVICE(vrf.device) on listener |
| **NTP** | SO_BINDTODEVICE(vrf.device) on dialer |
| **Prometheus** | Localhost (VRF-independent) or per-VRF |

### EventBus topic scoping

| Today | VRF-aware |
|-------|-----------|
| rib/best-change/bgp | rib/best-change/default/bgp |
| rib/replay-request | rib/replay-request/default |
| iface/created | iface/created/default |

VRF name inserted as path segment. Prefix matching still works:
- `rib/best-change/default/` catches all protocols in default VRF
- `rib/best-change/` catches all VRFs (useful for redistribution)

### VRF-aware socket helper

Shared utility in internal/core/vrfnet/:

```
vrfnet.Listen(ctx, vrfDevice, network, address) (net.Listener, error)
vrfnet.Dial(ctx, vrfDevice, network, address) (net.Conn, error)
```

Default VRF (empty device string): normal socket, no binding.
Non-default VRF: applies SO_BINDTODEVICE before connect/listen.

## Config Syntax

### Implicit default VRF (backwards compatible)

Top-level config belongs to vrf default (table 254). No wrapper needed:

```
bgp {
    router-id 82.219.0.154
    local { as 30740 }
    ...
}
interface eth0 { address 82.219.41.88/28; mtu 9000; }
interface lo { address 82.219.0.154/32; }
firewall { table wan { ... } }
ssh { listen-address 82.219.0.154; port 21982; }
ntp { server 82.219.4.30; server 82.219.4.31; }
```

### Explicit additional VRF

```
vrf surfprotect {
    table 100
    interface tun100 {
        encapsulation gre
        remote 82.219.26.116
        source-address 82.219.7.254
        mtu 8000
    }
    static {
        route 0.0.0.0/0 { interface tun100; }
    }
}
```

### Cross-VRF policy routing

Policy routing lives at top level, steering flows between VRFs:

```
policy {
    route surfprotect {
        interface "l2tp*"
        rule surfprotect-tcp {
            from { destination-port 80,443; protocol tcp; }
            then { vrf surfprotect; }  # resolves to table 100
        }
    }
}
```

### Route redistribution (future)

```
redistribute {
    from management to default {
        match { prefix-list mgmt-prefixes; }
    }
}
```

## Forward Compatibility of Phase 1 Specs

The Phase 1 specs (fw-8-lns-gaps, static-routes, policy-routing) use explicit table IDs.
When VRF is implemented, table IDs come from VRF context instead of explicit config:

| Phase 1 spec | VRF transition |
|---|---|
| spec-fw-8-lns-gaps | Firewall reactor receives VRF context in config. ze_ prefix unchanged for default VRF |
| spec-static-routes | table N in config becomes vrf.table from VRF context. Backend unchanged |
| spec-policy-routing | then { table 100 } becomes then { vrf surfprotect }. Same table, same fwmark + ip rule |

No Phase 1 work needs to be redone when VRF is implemented.

## Implementation Phases (full roadmap)

| Phase | Scope | Depends on | Blocks LNS? |
|-------|-------|------------|-------------|
| **Phase 1** (now) | Static routes with table ID, policy routing with fwmark, firewall gaps | Nothing | Yes |
| **Phase 2** | Hub multi-instance (spec-vrf-1) | Phase 1 | No |
| **Phase 3** | VRF device support in iface (spec-vrf-2) | Phase 2 | No |
| **Phase 4** | VRF orchestrator (spec-vrf-3) | Phase 3 | No |
| **Phase 5** | YANG/CLI VRF-aware dispatch (spec-vrf-4) | Phase 4 | No |
| **Phase 6** | VRF-aware services (spec-vrf-5) | Phase 4 | No |
| **Phase 7** | Dynamic VRF create/delete (spec-vrf-6) | Phase 5 | No |
| **Phase 8** | Route redistribution (spec-vrf-7) | Phase 4 | No |
| **Phase 9** | `vrf <name>` sugar in policy routing | Phase 4 | No |

## Design Insights

- All reactor globals are safe to share: pools, budgets, deadlines, message ID counter. Same process, same memory, no isolation needed.
- The reactor is already multi-instance ready. No refactoring needed -- just instantiate it N times.
- One process per component per VRF is simpler to understand and debug than shared-process multi-VRF state. Each plugin instance runs through the standard SDK 5-stage protocol.
- VRF deletion must carefully stop all child plugin instances, clean up kernel state (routes, rules, VRF device), and drain EventBus subscriptions.
- YANG wrapping is the key CLI enabler -- it makes VRF-scoped commands appear naturally in the schema tree without modifying any child module's YANG.
- The implicit default VRF is the key to backwards compatibility. Existing configs are valid VRF configs. Migration cost is zero.
- Policy routing and route redistribution are inherently cross-VRF. They sit outside the VRF hierarchy, operating on the seams between routing domains.
- SO_BINDTODEVICE is the right mechanism for service VRF binding: per-socket, per-service, no global sysctl changes.
- For the LNS, the surfprotect VRF needs no services (no SSH, no BGP). Only interfaces, static routes, and fibkernel. The VRF orchestrator only spawns the components that have config in that VRF block.
