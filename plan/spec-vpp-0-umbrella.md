# Spec: vpp-0-umbrella — VPP Integration Architecture

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-04-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugins/fib/kernel/` — existing FIB backend pattern
4. `internal/plugins/fib/p4/` — noop FIB backend template
5. `internal/component/iface/backend.go` — Backend interface pattern
6. Child specs: `spec-vpp-1-*` through `spec-vpp-6-*`

## Task

Integrate VPP (Vector Packet Processing) as ze's high-performance data plane. Ze programs
VPP's FIB directly from its BGP RIB via GoVPP binary API, manages VPP lifecycle, and manages
VPP interfaces. No kernel intermediary for route programming. LCP (Linux Control Plane) still
provides control-plane connectivity so BGP TCP sessions use Linux TAPs.

This is "Strategy 3" from the original plan: RIB to GoVPP to VPP FIB, bypassing the kernel
netlink path. Convergence drops from ~6s (netlink batched) to sub-second (~250K API calls/sec).

### Why VPP Direct (Strategy 3)

| Approach | Route path | Latency | Bottleneck |
|----------|-----------|---------|-----------|
| IPng/VyOS (Strategy 1) | RIB to kernel netlink to VPP netlink plugin | ~6s for full table | Netlink batching (40ms), kernel route table overhead |
| Ze Strategy 3 | RIB to GoVPP binary API to VPP FIB | Sub-second for full table | GoVPP throughput (~250K API calls/sec) |

Strategy 3 also enables features impossible through the kernel intermediary: MPLS label
push/swap/pop from BGP, SRv6 steering, per-prefix VPP counters, multipath with unequal cost.

### What ze uniquely offers vs alternatives

| Capability | IPng (vppcfg + Bird2) | VyOS | Ze with VPP |
|-----------|----------------------|------|-------------|
| BGP implementation | Bird2 (external) | FRR (external) | Ze (internal, zero-copy) |
| Route to FIB path | Bird to kernel to netlink to VPP | FRR to kernel to netlink to VPP | Ze RIB to GoVPP to VPP FIB (direct) |
| MPLS from BGP | Separate LDP daemon | Not exposed | BGP label to VPP label (integrated) |
| Config model | vppcfg YAML (separate tool) | VyOS CLI (XML/Python) | YANG-native, single CLI/web/API |
| Convergence | ~6s full table (netlink batched) | Similar | Sub-second (binary API, 250K/sec) |
| Per-prefix stats | Not available | Not available | VPP stats segment via GoVPP |
| Plugin extensibility | None | Monolithic | Plugin registry, EventBus |

### Design Decisions

| Decision | Detail |
|----------|--------|
| GoVPP binary API, no CGo | GoVPP's socket client is pure Go. No CGo dependency. |
| Component + plugin split | `internal/component/vpp/` for lifecycle (component). `internal/plugins/fib/vpp/` for FIB (standalone plugin, same pattern as fibkernel/fibp4). `internal/plugins/iface/vpp/` for interfaces (backend plugin for iface component). |
| Ze owns VPP process lifecycle | Ze execs VPP, supervises (restart with backoff), and shuts down cleanly. No systemd dependency. Required for gokrazy appliance (no init system). |
| Connection sharing: bus + direct import | EventBus `("vpp", "connected/disconnected/reconnected")` for lifecycle notifications. Direct import `vpp.Channel()` for GoVPP API calls. `Dependencies: ["rib", "vpp"]` for startup ordering. |
| fib-kernel and fib-vpp coexist | fib-kernel programs kernel routes for local services (SSH, web UI). fib-vpp programs VPP's FIB for transit traffic. |
| fibvpp owns its own metrics | `ConfigureMetrics` callback registers `ze_fibvpp_routes_installed` etc., same pattern as fibkernel. No reverse dependency from telemetry. |
| VPP FIB is ephemeral | Lost on VPP restart. Recovery via replay-request from sysRIB. Simpler than fib-kernel's crash recovery. |
| LCP for control plane | BGP reactor binds TCP sockets on Linux interfaces. LCP ensures those interfaces exist as TAPs. |
| Pin to VPP 25.02 LTS | GoVPP bindings must match VPP release. Regenerate per release. |
| YANG ownership | vpp-1 owns `ze-vpp-conf.yang`. vpp-2 augments `/fib:fib` (cross-component, like fibp4). vpp-5/vpp-6 add leaves directly to `ze-vpp-conf.yang` (same component). |
| ACL owned by spec-fw-6 | VPP ACLs are a firewall backend, not a standalone VPP feature. Dropped from vpp-5. |
| vpp-3 deferred | sysRIB event payload has no labels field today. MPLS label extension needs separate design before vpp-3 can proceed. |

### Architecture

```
                    ┌─────────────────────────────────────────┐
                    │                   ze                     │
                    │                                         │
                    │  BGP reactor ──> protocol RIB            │
                    │                      │                   │
                    │                  sysRIB                  │
                    │                  /     \                 │
                    │           fib-kernel   fib-vpp           │
                    │              │            │              │
                    │           netlink      GoVPP             │
                    └──────────────┼────────────┼──────────────┘
                                  │            │
                    ┌─────────────┼────────────┼──────────────┐
                    │  Linux      │            │    VPP       │
                    │  kernel     │     ┌──────┘              │
                    │  route      │     │  VPP FIB            │
                    │  table      │     │  (direct)           │
                    │             │     │                     │
                    │        LCP TAPs   DPDK NICs             │
                    └─────────────────────────────────────────┘
```

### Dependencies

| Dependency | Version | License | Purpose |
|-----------|---------|---------|---------|
| go.fd.io/govpp | v0.13.0 | Apache 2.0 | VPP binary API + stats client |
| govpp/binapi | Generated for VPP 25.02+ | Apache 2.0 | Pre-generated Go bindings |

No CGo. GoVPP's socket client is pure Go.

### Existing ze abstractions to build on

| Abstraction | Location | How it is used |
|------------|----------|--------------|
| `routeBackend` interface | `internal/plugins/fib/kernel/` | 5 methods: add/del/replace/list/close |
| fib-p4 plugin (template) | `internal/plugins/fib/p4/` | Same pattern, noop backend, ready to copy |
| `iface.Backend` interface | `internal/component/iface/backend.go` | 30+ methods, pluggable via RegisterBackend |
| EventBus subscribe | `pkg/ze/eventbus.go` | Subscribe("system-rib", "best-change", handler) |
| Plugin registration | `internal/plugin/registry.go` | init() + registry.Register() |
| YANG augment | `ze-fib-p4-conf.yang` | augment "/fib:fib" for backend-specific config |

## Child Specs

| Spec | Scope | Depends | Est. LOC |
|------|-------|---------|----------|
| `spec-vpp-1-lifecycle.md` | VPP component: startup.conf generation, DPDK NIC binding, GoVPP connection, health monitoring | This umbrella | ~800 |
| `spec-vpp-2-fib.md` | fib-vpp plugin: IPv4/IPv6 route programming via GoVPP, batch optimization, replay recovery | vpp-1 | ~1200 |
| `spec-vpp-3-mpls.md` | MPLS label push/swap/pop from BGP labels, sysRIB label extension. **Deferred**: sysRIB event payload needs labels field first. | vpp-2 + sysRIB label spec | ~600 |
| `spec-vpp-4-iface.md` | VPP interface backend: iface.Backend implementation, LCP pairs, naming | vpp-1 | ~2000 |
| ~~`spec-vpp-5-features.md`~~ | ~~VPP-native features: L2XC, bridge domains, VXLAN, policers, ACLs, SRv6, sFlow~~ Retired 2026-04-17 as design dead-end. VPP is a backend for ze abstractions (iface/firewall/traffic), not a separate config surface. Features re-homed: Bridge → existing `iface { bridge ... }` (ifacevpp backend extension). VXLAN → new `case vxlan` under existing `iface { tunnel }` discriminator. Policer/ACL → already owned by spec-fw-6/spec-fw-7. L2XC/SRv6/sFlow deferred pending per-component design. | - | - |
| `spec-vpp-6-telemetry.md` | VPP telemetry: stats segment polling, per-prefix/interface/node counters, Prometheus | vpp-1 | ~600 |
| `spec-vpp-7-test-harness.md` | Python GoVPP-API stub + `test/vpp/` runner + `vpp.external` config leaf; unlocks the seven umbrella `test/vpp/NNN-*.ci` wiring tests without needing real VPP or DPDK in CI | vpp-1, vpp-2 | ~1000 |

### Execution Order and MVP

Phases vpp-1 + vpp-2 together deliver the headline: ze's BGP decisions programmed directly
into VPP's FIB, no kernel intermediary. That is the minimum viable product.

Phase vpp-3 (MPLS) is what makes it worth doing over Strategy 1. Ze already parses MPLS
labels from BGP; pushing them directly into VPP is the natural next step.

Phase vpp-4 (interface backend) makes ze the single configuration authority. Without it,
VPP interfaces are managed separately (vppcfg or manual vppctl). With it, `ze config edit`
handles everything.

Phase vpp-6 is an incremental value add. vpp-5 was retired (see Child Specs table): VPP
features are exposed through ze's existing component abstractions (iface, firewall,
traffic), not through a parallel VPP-native config surface. Whether a given backend
supports a requested feature is enforced at commit time via `spec-backend-feature-gate`.

Phase vpp-7 (test harness) is the coverage backstop for every VPP phase: the
umbrella wiring-test rows for vpp-1..vpp-6 all resolve to `.ci` files under
`test/vpp/` that only run when the vpp-7 stub + runner exist. Land vpp-7 early
so subsequent VPP work has green wiring evidence, not just unit tests.

### Cross-references

| Spec | Relationship |
|------|-------------|
| `spec-fw-6-firewall-vpp.md` | Depends on vpp-1 (VPP lifecycle). Uses GoVPP ACL API. Owns all VPP ACL work (dropped from vpp-5). |
| `spec-fw-7-traffic-vpp.md` | Depends on vpp-1. Uses GoVPP policer/scheduler API. |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component/plugin architecture
  → Constraint: components under `internal/component/`, plugins under `internal/plugins/`
  → Decision: backend interface pattern (component defines interface, plugin implements)
- [ ] `internal/plugins/fib/kernel/` - existing FIB backend pattern
  → Constraint: routeBackend interface with add/del/replace/list/close
  → Decision: FIB plugins subscribe to (system-rib, best-change) events via EventBus
- [ ] `internal/plugins/fib/p4/` - noop FIB backend template
  → Constraint: same registration + event subscription pattern, ready to copy for fibvpp
- [ ] `internal/component/iface/backend.go` - pluggable Backend interface
  → Constraint: Backend registered via RegisterBackend in init(), 30+ methods
  → Decision: ifacevpp implements same interface, registered as "vpp" backend
- [ ] `ai/patterns/registration.md` - registration pattern
  → Constraint: init() + registry.Register() pattern for plugins
- [ ] `ai/patterns/config-option.md` - config option pattern
  → Constraint: YANG leaf + env var registration for every config option
- [ ] `rules/config-design.md` - config design rules
  → Constraint: fail on unknown keys, no version numbers, listener grouping pattern

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc8277.md` - Using BGP to Bind MPLS Labels to Address Prefixes (Phase vpp-3 MPLS; obsoletes RFC 3107)
  → Constraint: label binding procedures, label stack encoding, labelled unicast NLRI format

**Key insights:**
- FIB plugins (fibkernel, fibp4) share identical registration + event subscription patterns; fibvpp copies this
- iface.Backend is a 30+ method interface registered via RegisterBackend; ifacevpp implements it
- EventBus subscription: Subscribe("system-rib", "best-change", handler) in OnStarted callback
- VPP FIB is ephemeral (lost on restart), so recovery is replay-request, not sweep/reconcile
- GoVPP socket client is pure Go, no CGo needed
- LCP provides TAPs for BGP TCP sessions on Linux side

## Reference Documents

| Document | Purpose |
|----------|---------|
| `docs/research/vpp-deployment-reference.md` | Production values, NIC driver matrix, startup.conf reference, LCP details, performance baselines (from IPng.ch 83 articles) |
| `docs/research/ze-vpp-analysis.md` | Three-strategy feasibility analysis (strategy 1 LCP-transparent, strategy 2 VPP-native iface backend, strategy 3 direct FIB). Local copy; originally from `~/Code/site/ipng.ch/ze-vpp-analysis.md` |
| `docs/guide/vpp.md` | Reader-facing user guide. MUST be kept current with every phase that lands (see Documentation Update Checklist). |
| `docs/research/vpp-deployment-notes.md` | Consolidated notes from 83 IPng articles (local copy; originally from `~/Code/site/ipng.ch/vpp-deployment-notes.md`) |
| GoVPP documentation (go.fd.io/govpp) | Binary API client, stats client, binapi code generation |
| VPP 25.02 documentation | API reference for ip, mpls, interface, lcp, acl, policer, srv6, sflow modules |

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/fib/kernel/fibkernel.go` — FIB kernel plugin: subscribes to (system-rib, best-change), programs kernel routes via netlink
  → Constraint: same event subscription pattern for fibvpp
- [ ] `internal/plugins/fib/kernel/register.go` — Registration with Dependencies: ["rib", "sysctl"]
  → Constraint: fibvpp registration with Dependencies: ["rib", "vpp"]
- [ ] `internal/plugins/fib/kernel/backend.go` — routeBackend interface: addRoute, delRoute, replaceRoute, listZeRoutes, close
  → Constraint: fibvpp backend extends with batch operations and MPLS
- [ ] `internal/plugins/fib/p4/fibp4.go` — noop FIB backend, same structure as fibkernel
  → Constraint: copy pattern for fibvpp
- [ ] `internal/component/iface/backend.go` — Backend interface with RegisterBackend/LoadBackend/GetBackend
  → Constraint: same pattern for ifacevpp
- [ ] `internal/plugins/iface/netlink/register.go` — init() calls iface.RegisterBackend("netlink", factory)
  → Constraint: ifacevpp registers as "vpp" backend
- [ ] `pkg/ze/eventbus.go` — EventBus interface: Emit, Subscribe
  → Constraint: fibvpp subscribes to (system-rib, best-change) events

**Behavior to preserve:**
- fib-kernel continues to work for kernel route programming
- ifacenetlink continues to work for Linux interfaces
- EventBus event format unchanged
- Plugin registration patterns unchanged

**Behavior to change:**
- Add new VPP component (`internal/component/vpp/`)
- Add fib-vpp plugin (`internal/plugins/fib/vpp/`)
- Add iface-vpp plugin (`internal/plugins/iface/vpp/`)
- Add GoVPP as new dependency

## Data Flow (MANDATORY)

### Entry Point
- VPP lifecycle: YANG config parsed at startup, VPP component generates startup.conf, binds DPDK NICs, starts VPP, connects via GoVPP
- FIB programming: BGP reactor selects best routes, sysRIB emits (system-rib, best-change) events with JSON payload
- Interface management: YANG config parsed, iface component calls Backend methods via ifacevpp

### Transformation Path
1. BGP reactor selects best routes, stores in protocol RIB
2. sysRIB receives best routes, emits (system-rib, best-change) event with JSON payload
3. fib-vpp plugin receives event in subscribed handler
4. Plugin parses JSON payload into prefix + next-hop (+ optional labels for MPLS)
5. Plugin calls GoVPP binary API: IPRouteAddDel for IPv4/IPv6, MplsRouteAddDel for MPLS
6. VPP FIB updated directly, no kernel intermediary

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| BGP reactor → sysRIB | Protocol RIB best route selection | [ ] |
| sysRIB → fib-vpp | EventBus JSON payload (system-rib, best-change) | [ ] |
| fib-vpp → VPP | GoVPP binary API over Unix socket (/run/vpp/api.sock) | [ ] |
| Config → VPP component | YANG tree parsing, startup.conf generation | [ ] |
| iface component → ifacevpp | Backend method calls (RegisterBackend pattern) | [ ] |
| ifacevpp → VPP | GoVPP binary API (SwInterfaceAddDelAddress, etc.) | [ ] |

### Integration Points
- `internal/plugins/fib/kernel/` — same event subscription pattern, coexists with fibvpp
- `internal/component/iface/backend.go` — Backend interface that ifacevpp implements
- `pkg/ze/eventbus.go` — EventBus for (system-rib, best-change) events
- `internal/plugin/registry.go` — Plugin registration for vpp component, fibvpp, ifacevpp
- `internal/component/config/` — YANG tree parsing for VPP config

### Architectural Verification
- [ ] No bypassed layers (sysRIB events to GoVPP, not direct RIB access)
- [ ] No unintended coupling (fibvpp and ifacevpp share VPP connection from vpp component, nothing else)
- [ ] No duplicated functionality (fibvpp parallels fibkernel for a different dataplane)
- [ ] Zero-copy preserved where applicable (EventBus payload is JSON string, GoVPP handles binary encoding)

## Wiring Test (MANDATORY — NOT deferrable)

Umbrella spec delegates wiring tests to child specs. Each child has its own wiring table.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| VPP YANG config | → | vpp component startup.conf generation + GoVPP connect | spec-vpp-1 wiring tests |
| sysRIB best-change event | → | fibvpp IPRouteAddDel via GoVPP | spec-vpp-2 wiring tests |
| sysRIB best-change with labels | → | fibvpp MplsRouteAddDel via GoVPP | spec-vpp-3 wiring tests |
| iface YANG config with vpp backend | → | ifacevpp Backend methods via GoVPP | spec-vpp-4 wiring tests |
| VPP stats polling interval | → | stats segment read, Prometheus metrics | spec-vpp-6 wiring tests |

## Acceptance Criteria

Umbrella AC; child specs have detailed per-feature AC.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | ze boots with VPP config enabled | VPP started with generated startup.conf, GoVPP connected |
| AC-2 | DPDK interfaces configured | NICs bound to vfio-pci, VPP DPDK interfaces created |
| AC-3 | BGP learns a new prefix | fib-vpp programs route in VPP FIB via IPRouteAddDel |
| AC-4 | BGP withdraws a prefix | fib-vpp removes route from VPP FIB |
| AC-5 | VPP restarts | GoVPP detects disconnect, reconnects, replays full RIB |
| AC-6 | BGP learns labeled unicast prefix | fib-vpp programs MPLS label push in VPP FIB |
| AC-7 | iface Backend "vpp" configured | ifacevpp creates/configures VPP interfaces via GoVPP |
| AC-8 | LCP enabled | VPP interfaces have Linux TAP pairs for control plane |
| AC-9 | VPP stats polling enabled | Per-interface and per-node counters exported as Prometheus metrics |
| AC-10 | fib-kernel and fib-vpp both configured | Both coexist: kernel routes for local, VPP FIB for transit |

## 🧪 TDD Test Plan

### Unit Tests
Delegated to child specs. Each child has its own test plan.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| VPP startup.conf generation | spec-vpp-1 | Config to startup.conf template rendering | |
| DPDK NIC binding | spec-vpp-1 | PCI address parsing, driver save/restore | |
| GoVPP connection lifecycle | spec-vpp-1 | Connect/disconnect/reconnect state machine | |
| fib-vpp event processing | spec-vpp-2 | JSON payload parsing, route add/del/replace | |
| fib-vpp batch optimization | spec-vpp-2 | Batch collection and dispatch | |
| MPLS label operations | spec-vpp-3 | Push/swap/pop via GoVPP MPLS API | |
| ifacevpp Backend methods | spec-vpp-4 | All 30+ Backend interface methods | |
| VPP interface naming | spec-vpp-4 | Ze name to VPP name bidirectional mapping | |
| VPP stats polling | spec-vpp-6 | Stats segment read, metric export | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| PCI address format | 0000:00:00.0 pattern | valid PCI addr | malformed string | malformed string |
| VPP workers | 0-255 | 255 | N/A (0 = auto) | 256 |
| Hugepage size | enum (2M, 1G) | 1G | invalid string | invalid string |
| Buffer count | 1+ | 1 | 0 | uint32 max |
| VRF table ID | 0-4294967295 | 4294967295 | N/A (0 = default) | N/A (uint32) |
| Batch size | 1-65535 | 65535 | 0 | 65536 |
| MPLS label | 0-1048575 (20-bit) | 1048575 | N/A | 1048576 |
| Stats poll interval | 1s+ | 1 | 0 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| VPP lifecycle boot | `test/vpp/001-boot.ci` | VPP config present, ze starts, VPP running, GoVPP connected | |
| FIB route programming | `test/vpp/002-fib-route.ci` | BGP learns prefix, VPP FIB has route | |
| FIB withdrawal | `test/vpp/003-fib-withdraw.ci` | BGP withdraws prefix, VPP FIB route gone | |
| VPP restart recovery | `test/vpp/004-vpp-restart.ci` | VPP restarts, fib-vpp replays full table | |
| MPLS label push | `test/vpp/005-mpls-push.ci` | Labeled unicast prefix, MPLS push in VPP | |
| Interface creation | `test/vpp/006-iface-create.ci` | VPP interface config, interface exists in VPP | |
| Coexistence | `test/vpp/007-coexist.ci` | fib-kernel + fib-vpp both active, both program routes | |

### Future (if deferring any tests)
- VPP-specific feature tests moved to per-component specs: bridge (ifacevpp extension), VXLAN (iface tunnel case), L2XC/SRv6/sFlow (TBD)
- Advanced telemetry tests deferred to spec-vpp-6

## Files to Modify

Umbrella creates no files directly. All files are in child specs.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | `internal/component/vpp/schema/ze-vpp-conf.yang` (spec-vpp-1) |
| YANG schema | Yes | fib-vpp augment (spec-vpp-2) |
| CLI commands/flags | Yes | VPP status/show commands (spec-vpp-1) |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | Yes | `test/vpp/*.ci` (each child spec) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — add VPP integration |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` — add VPP section |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` — add VPP commands |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` — add fib-vpp, iface-vpp |
| 6 | Has a user guide page? | Yes | `docs/guide/vpp.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | Yes (MPLS, Phase vpp-3 deferred) | `rfc/short/rfc8277.md` |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` — add VPP comparison |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` — add VPP component |

## Files to Create

All files delegated to child specs. Summary:

| Child spec | Files |
|------------|-------|
| vpp-1 | `internal/component/vpp/` (vpp.go, config.go, startupconf.go, dpdk.go, conn.go, register.go, schema/) |
| vpp-2 | `internal/plugins/fib/vpp/` (fibvpp.go, backend.go, register.go, schema/) |
| vpp-3 | `internal/plugins/fib/vpp/mpls.go`, `internal/plugins/fib/vpp/mpls_test.go` |
| vpp-4 | `internal/plugins/iface/vpp/` (ifacevpp.go, naming.go, register.go) |
| vpp-5 | Feature-specific files within vpp component or plugins |
| vpp-6 | `internal/plugins/fib/vpp/stats.go`, `internal/component/vpp/telemetry.go` |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + relevant child spec |
| 2. Audit | Child spec Files to Modify/Create |
| 3. Implement (TDD) | Child spec Implementation phases |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Child spec Critical Review Checklist |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Child spec Deliverables Checklist |
| 10. Security review | Child spec Security Review Checklist |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: VPP lifecycle (vpp-1)** — component, startup.conf, DPDK, GoVPP connection
   - Tests: startup.conf generation, DPDK binding, connection lifecycle
   - Files: `internal/component/vpp/` (6-7 files)
   - Verify: tests fail → implement → tests pass

2. **Phase: fib-vpp core (vpp-2)** — route programming
   - Tests: event processing, batch optimization, replay recovery
   - Files: `internal/plugins/fib/vpp/` (4 files)
   - Verify: tests fail → implement → tests pass

3. **Phase: MPLS labels (vpp-3)** — label operations from BGP
   - Tests: push/swap/pop operations, sysRIB label extension
   - Files: `internal/plugins/fib/vpp/mpls.go`
   - Verify: tests fail → implement → tests pass

4. **Phase: VPP interface backend (vpp-4)** — iface.Backend implementation
   - Tests: all Backend interface methods, naming, LCP pairs
   - Files: `internal/plugins/iface/vpp/` (4 files)
   - Verify: tests fail → implement → tests pass

5. **Phase: VPP features (vpp-5)** — L2XC, VXLAN, policers, ACLs, SRv6, sFlow
   - Tests: per-feature tests
   - Files: feature-specific within vpp component/plugins
   - Verify: per-feature TDD cycle

6. **Phase: Telemetry (vpp-6)** — stats segment, Prometheus
   - Tests: stats polling, metric export
   - Files: stats.go, telemetry.go
   - Verify: tests fail → implement → tests pass

7. **Functional tests** → Cover all AC from umbrella and children
8. **Full verification** → `make ze-verify`
9. **Complete spec** → Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N from umbrella and active children demonstrated |
| Correctness | GoVPP API calls match VPP 25.02 bindings, route operations correct |
| Naming | YANG config follows ze conventions, VPP interface naming bidirectional |
| Data flow | sysRIB events to GoVPP, no shortcut past EventBus |
| Rule: no-layering | No kernel intermediary for VPP FIB programming |
| Rule: single-responsibility | VPP component for lifecycle, fibvpp for FIB, ifacevpp for interfaces |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| VPP component exists | `ls internal/component/vpp/` |
| fib-vpp plugin exists | `ls internal/plugins/fib/vpp/` |
| iface-vpp plugin exists | `ls internal/plugins/iface/vpp/` |
| YANG module registers | `grep -r "yang.RegisterModule" internal/component/vpp/` |
| Plugin registration works | `grep -r "registry.Register" internal/plugins/fib/vpp/ internal/plugins/iface/vpp/` |
| Backend registration works | `grep -r "RegisterBackend" internal/plugins/iface/vpp/` |
| VPP starts from config | functional .ci test |
| Routes in VPP FIB | functional .ci test |
| fib-kernel + fib-vpp coexist | functional .ci test |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | YANG config values validated: PCI addresses, worker counts, hugepage sizes |
| Privilege | DPDK NIC binding requires root (vfio-pci). VPP API socket access. |
| Socket path | GoVPP connects to /run/vpp/api.sock. Validate path is not user-controllable injection. |
| PCI address handling | PCI addresses written to sysfs. Validate format strictly to prevent path traversal. |
| Module loading | vfio kernel modules loaded via modprobe. Validate module names are hardcoded, not from config. |
| VPP restart loop | If VPP keeps crashing, backoff on reconnect attempts to avoid busy loop. |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Package Layout

| Package | Purpose | Library |
|---------|---------|---------|
| `internal/component/vpp/` | VPP lifecycle: startup, shutdown, health, config | go.fd.io/govpp |
| `internal/component/vpp/schema/` | `ze-vpp-conf.yang` | None |
| `internal/plugins/fib/vpp/` | FIB backend: route/MPLS programming via GoVPP | go.fd.io/govpp (ip, mpls binapi) |
| `internal/plugins/fib/vpp/schema/` | `ze-fib-vpp-conf.yang` augmenting /fib:fib | None |
| `internal/plugins/iface/vpp/` | Interface backend: iface.Backend via GoVPP | go.fd.io/govpp (interfaces, lcp binapi) |

## YANG Config Shape

VPP component YANG module (`ze-vpp-conf`):

| Container | Leaf | Type | Default | Description |
|-----------|------|------|---------|-------------|
| vpp | enabled | boolean | false | Enable VPP integration |
| vpp | api-socket | string | /run/vpp/api.sock | GoVPP API socket path |
| vpp/cpu | main-core | uint8 | - | CPU core for VPP main thread |
| vpp/cpu | workers | uint8 | - | Number of worker threads |
| vpp/cpu | isolate | leaf-list uint8 | - | CPU cores to isolate |
| vpp/memory | main-heap | string | 1G | Main heap size |
| vpp/memory | hugepage-size | enum (2M, 1G) | 2M | Hugepage size |
| vpp/memory | buffers | uint32 | 128000 | Buffer count |
| vpp/dpdk/interface (list, key: pci-address) | pci-address | string | - | PCI address (0000:03:00.0) |
| vpp/dpdk/interface | name | string | - | Interface name (xe0) |
| vpp/dpdk/interface | rx-queues | uint8 | - | Receive queues |
| vpp/dpdk/interface | tx-queues | uint8 | - | Transmit queues |
| vpp/stats | segment-size | string | 512M | Stats segment size |
| vpp/stats | socket-path | string | /run/vpp/stats.sock | Stats socket path |
| vpp/lcp | enabled | boolean | true | Enable LCP plugin |
| vpp/lcp | sync | boolean | true | Sync LCP state |
| vpp/lcp | auto-subint | boolean | true | Auto-create sub-interfaces |
| vpp/lcp | netns | string | dataplane | Network namespace for LCP |

fib-vpp YANG (augments /fib:fib):

| Container | Leaf | Type | Default | Description |
|-----------|------|------|---------|-------------|
| fib/vpp | enabled | boolean | false | Enable VPP FIB programming |
| fib/vpp | table-id | uint32 | 0 | VRF table ID |
| fib/vpp | batch-size | uint16 | 256 | Batch collection size |
| fib/vpp | batch-interval-ms | uint16 | 10 | Batch dispatch interval |

## VPP startup.conf Generation

The VPP component generates startup.conf from YANG config. Template sections:

| Section | Contents | Source |
|---------|----------|--------|
| unix | nodaemon, cli-listen socket, log path, coredump | Hardcoded defaults |
| cpu | main-core, corelist-workers | vpp/cpu YANG leaves |
| buffers | buffers-per-numa, default-data-size, page-size | vpp/memory YANG leaves |
| dpdk | dev entries with PCI address, name, rx/tx queues | vpp/dpdk/interface list |
| plugins | Enable dpdk, linux_cp, linux_nl; disable others | Hardcoded, LCP toggle from vpp/lcp/enabled |
| linux-cp | lcp-sync, lcp-auto-subint, default netns | vpp/lcp YANG leaves |
| linux-nl | rx-buffer-size | Hardcoded (67108864) |
| statseg | size, page-size | vpp/stats + vpp/memory YANG leaves |

Pattern follows IPng.ch blog and VyOS proven templates.

## DPDK NIC Driver Management

Ported from VyOS `control_host.py` to Go:

**Bind sequence (per configured PCI address):**
1. Read current driver from `/sys/bus/pci/devices/<addr>/driver`
2. Save original driver name to persistent state
3. Load vfio modules: `vfio`, `vfio_pci`, `vfio_iommu_type1`
4. Write PCI address to `/sys/bus/pci/devices/<addr>/driver/unbind`
5. Write vendor:device to `/sys/bus/pci/drivers/vfio-pci/new_id`

**Unbind sequence (teardown, reverse order):**
1. Unbind from vfio-pci
2. Trigger PCI rescan via `/sys/bus/pci/rescan`
3. Rebind to original saved driver

## GoVPP Connection Management

| State | Transition | Action |
|-------|-----------|--------|
| Disconnected | Connect requested | AsyncConnect to api-socket with retry (10 attempts, 1s interval) |
| Connecting | Connected event | Mark ready, emit EventBus ("vpp", "connected"), expose vpp.Channel() |
| Connected | Disconnect event | Mark unavailable, emit ("vpp", "disconnected"), dependents stop operations |
| Connected | Health check timeout | Restart VPP process with backoff, reconnect |
| Reconnecting | Connected event | Emit ("vpp", "reconnected"), fibvpp replays FIB |

Connection provides typed API clients:
- IP client for route operations (IPRouteAddDel)
- MPLS client for label operations (MplsRouteAddDel)
- Interface client for interface management (SwInterfaceSetFlags, etc.)
- Stats client for telemetry (shared memory, separate from binary API)

## Risks

| Risk | Mitigation |
|------|-----------|
| GoVPP version coupling (bindings must match VPP release) | Pin to VPP 25.02 LTS. Regenerate bindings per release. |
| VPP crashes lose FIB (ephemeral state) | GoVPP detects disconnect; ze re-emits replay-request to repopulate. |
| LCP plugin stability (IPng maintains their own fork) | Start with upstream LCP (shipped with VPP 25.02+). Evaluate lcpng if insufficient. |
| DPDK NIC compatibility | Document supported NICs: Intel E810, Mellanox Cx5+, i40e, i350, virtio. |
| sysRIB event format change for MPLS labels | vpp-3 deferred until sysRIB label extension spec is designed and implemented. |
| VPP process management | Ze owns full VPP lifecycle (exec, supervise, restart with backoff, clean shutdown). Required for gokrazy appliance (no systemd). |

## Phase Summary

| Phase | What | Depends on | New dependency |
|-------|------|-----------|----------------|
| vpp-1 | VPP lifecycle (startup.conf, DPDK, connect) | Nothing | go.fd.io/govpp |
| vpp-2 | fib-vpp (IPv4/IPv6 route programming) | vpp-1 | - |
| vpp-3 | MPLS label operations (**deferred**: sysRIB needs labels field) | vpp-2 + sysRIB label spec | - |
| vpp-4 | VPP interface backend (iface.Backend) | vpp-1 | - |
| vpp-5 | VPP-specific features (L2XC, VXLAN, policers...) | vpp-4 | - |
| vpp-6 | Telemetry (stats segment, Prometheus) | vpp-1 | - |

**Total for core (vpp-1 + vpp-2):** ~2000 LOC, one new dependency
**Total with interfaces (add vpp-4):** ~4000 LOC
**Total with everything:** ~6000-8000 LOC depending on vpp-5 scope

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
- [ ] Write learned summary to `plan/learned/NNN-vpp-0-umbrella.md`
- [ ] Summary included in commit
