# Spec: fib-0 -- FIB Pipeline (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `pkg/ze/bus.go` -- Bus interface
4. `internal/component/bgp/plugins/rib/bestpath.go` -- existing best-path algorithm
5. `internal/component/bgp/plugins/rib/rib.go` -- RIB plugin (currently on-demand best-path)
6. Child specs: `spec-fib-1-rib-events.md` through `spec-fib-4-p4.md`

## Task

Build the route installation pipeline from protocol RIBs through a System RIB to FIB backends. Three new components, all standalone plugins communicating exclusively via Bus events:

1. **Protocol RIB events** -- BGP RIB (and future protocol RIBs) publish best-route changes to the Bus
2. **System RIB** -- aggregates best routes from all protocols, selects by administrative distance (lower wins), publishes system-wide best routes
3. **FIB backends** -- separate plugins per backend type that subscribe to System RIB events and program the forwarding plane (kernel routing table, P4 switch, etc.)

### Architecture

```
Protocol RIBs (per-protocol best path selection)
  BGP RIB    --> rib/best-change/bgp    (eBGP=20, iBGP=200)
  Static     --> rib/best-change/static  (priority 10)
  OSPF       --> rib/best-change/ospf    (priority 110, future)
                    |
            Bus: rib/best-change/ (prefix subscription)
                    |
              System RIB plugin
              (selects by admin distance, lower wins)
                    |
            Bus: sysrib/best-change
                    |
        +-----------+-----------+
   fib-kernel    fib-p4      fib-xxx
   (build-tag)  (cross-OS)   (future)
```

### Design Decisions (agreed with user)

| Decision | Detail |
|----------|--------|
| FIB is a standalone plugin | Not under BGP, not a subsystem. Reacts to Bus events, no external I/O ownership |
| System RIB is a standalone plugin | `internal/plugins/sysrib/`. Subscribes to Bus, publishes to Bus. No external I/O |
| Protocol RIBs push to System RIB | Protocol RIBs publish `rib/best-change/<protocol>` events. System RIB subscribes to `rib/best-change/` prefix |
| Admin distance: lower wins | Standard convention. Configurable per protocol |
| Separate FIB plugin per backend | `fib-kernel` (OS build tags), `fib-p4` (cross-OS), etc. Each subscribes to `sysrib/best-change` independently |
| OS backends use build tags | `backend_linux.go`, `backend_darwin.go` -- compile-time selection, matches iface plugin pattern |
| P4 and similar are cross-OS | Network protocol based, not OS-specific. Generic Go code |
| VRF follows existing design | Per-VRF hub instances. FIB plugins connect to whichever hub they are given. No FIB-specific VRF logic. Each VRF has its own Bus instance; topic names are identical across VRFs but events never cross VRF boundaries |
| Topics: `rib/best-change/<protocol>` | System RIB subscribes to `rib/best-change/` prefix, catches all protocols without knowing which exist |
| eBGP/iBGP split admin distance | BGP RIB sets priority per route: eBGP=20, iBGP=200. Uses PeerMeta (PeerASN vs LocalASN) to distinguish. Configurable via YANG |
| Batch event format | One Bus event carries an array of prefix changes. Avoids 900K individual publishes on full-table peer down. Collected under lock, published after lock release |
| Crash recovery: stale-mark-then-sweep | fib-kernel uses custom Linux `rtm_protocol` ID for all ze routes. On startup, marks existing ze routes as stale (short timer). As BGP reconverges, matching routes are refreshed. Stale routes removed after timer expires. Preserves forwarding during restart |
| Phase 1 includes Bus wiring | Phase 1 adds Bus reference to BGP RIB (via `ConfigureBus` registration callback) AND best-path tracking + publish. No external dependency |
| CLI naming | `ze show bgp rib` (BGP protocol RIB), `ze show rib` (system RIB). .ci tests verify via CLI output |
| System RIB key | `(family, prefix)` -- each protocol publishes at most one route per (family, prefix) pair |
| ADD-PATH is internal | BGP RIB runs SelectBest across all ADD-PATH candidates internally. Published event always contains the single best route per prefix. ADD-PATH path-IDs never leak to System RIB |

### Scope

**In Scope:**

| Area | Description |
|------|-------------|
| BGP RIB best-route events | Wire Bus into BGP RIB (ConfigureBus callback). Change from on-demand best-path to real-time best-path tracking with Bus event publishing |
| System RIB plugin | New plugin: subscribe to all protocol RIB events, select by admin distance, publish system-wide best |
| fib-kernel plugin | New plugin: subscribe to System RIB events, program OS kernel routing table (Linux netlink, Darwin route socket) |
| Admin distance table | Default priorities per protocol, configurable via YANG |
| Bus topic contract | Topic hierarchy, payload format, metadata schema for the full pipeline |

**Out of Scope:**

| Area | Reason |
|------|--------|
| fib-p4 plugin | Defer until P4 support is needed. Spec placeholder only |
| ECMP / multipath next-hops | Requires System RIB to track multiple equal-cost paths and array payload format. Separate spec |
| Recursive next-hop resolution | Requires IGP integration. Future spec |
| Route redistribution | Separate concern (which protocol routes to inject into which other protocol) |
| Policy / route-maps | Filtering between protocol RIB and System RIB. Future spec |
| Static route plugin | Separate spec. Will publish to `rib/best-change/static` using same contract |

## Child Specs

| Phase | Spec | Scope | Depends |
|-------|------|-------|---------|
| 1 | `spec-fib-1-rib-events.md` | Wire Bus into BGP RIB via ConfigureBus callback. Add real-time best-path tracking (previous best per family+prefix). Publish batch events to `rib/best-change/bgp` on changes. Collect under lock, publish after release. eBGP priority=20, iBGP priority=200 | - |
| 2 | `spec-fib-2-sysrib.md` | System RIB plugin: subscribes to `rib/best-change/`, maintains per-prefix best across protocols by admin distance, publishes `sysrib/best-change` | fib-1 |
| 3 | `spec-fib-3-kernel.md` | `fib-kernel` plugin: subscribes to `sysrib/best-change`, programs OS routes via netlink (Linux) or route socket (Darwin). Build-tag OS selection. Custom `rtm_protocol` ID. Startup stale-mark-then-sweep for crash recovery | fib-2 |
| 4 | `spec-fib-4-p4.md` | `fib-p4` plugin: subscribes to `sysrib/best-change`, programs P4 switch via gRPC/P4Runtime. Deferred until needed | fib-3 |

Phases are strictly ordered. Each phase must be complete before the next begins.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- overall architecture, plugin model, Bus integration
  -> Constraint: Bus is content-agnostic, payload always `[]byte`, topics hierarchical with `/`
  -> Decision: plugins communicate via Bus events, never import each other
- [ ] `docs/architecture/plugin/rib-storage-design.md` -- RIB storage patterns, pool architecture
  -> Constraint: RIB stores routes per-peer with per-attribute dedup pools
- [ ] `.claude/rules/plugin-design.md` -- plugin registration, 5-stage protocol, import rules
  -> Constraint: registration via `init()` in `register.go`, blank import is only coupling
  -> Constraint: plugins MUST NOT import sibling plugin packages

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` -- BGP-4: Decision Process (Section 9.1.2)
  -> Constraint: best-path selection criteria and ordering
- [ ] `rfc/short/rfc4364.md` -- BGP/MPLS IP VPNs: VRF concept
  -> Constraint: VRF is per-PE, each VRF has independent forwarding table

### Related Specs
- [ ] `plan/spec-rib-05-best-path.md` -- existing best-path spec (deferred). Phase 1 builds on its algorithm
  -> Decision: best-path was on-demand (YAGNI). Phase 1 changes to real-time tracking
- [ ] `plan/spec-iface-0-umbrella.md` -- interface management (separate concern, no overlap)
  -> Decision: iface = interface lifecycle, FIB = route installation
- [ ] `plan/spec-vrf-0-umbrella.md` -- VRF support
  -> Decision: per-VRF hub instances. FIB plugins connect to their VRF's hub. No FIB-specific VRF logic

**Key insights:**
- BGP RIB has `SelectBest()` and `ComparePair()` as pure functions in `bestpath.go`. Phase 1 wires these into real-time tracking.
- Bus implementation in `internal/component/bus/bus.go`: per-consumer worker goroutines, batch drain, prefix matching. Ready to use.
- RIB currently handles received UPDATEs via `rib_structured.go` (DirectBridge). Best-path is computed only at query time (`rib best`). No event publishing.
- The interface plugin (`spec-iface-0-umbrella`) and FIB are cleanly separate: iface publishes `interface/*` events for link/addr lifecycle. FIB publishes/subscribes `sysrib/*` events for route installation.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/ze/bus.go` -- Bus interface: `CreateTopic`, `Publish`, `Subscribe`, `Unsubscribe`. Event has Topic, Payload (`[]byte`), Metadata (`map[string]string`)
- [ ] `internal/component/bus/bus.go` -- Bus implementation: per-consumer workers, batch drain, prefix matching
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` -- `SelectBest()`, `ComparePair()`, `Candidate` struct. Pure functions, RFC 4271 Phase 2. 7 of 8 decision steps (IGP cost deferred)
- [ ] `internal/component/bgp/plugins/rib/rib.go` -- `RIBManager`, handles events, ribInPool per-peer storage. Best-path on-demand only
- [ ] `internal/component/bgp/plugins/rib/rib_structured.go` -- `dispatchStructured()` routes events by type. `handleReceivedStructured()` inserts into RIB. No outbound event publishing
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` -- `extractCandidate()` builds Candidate from pool handles. `bestPathStatusJSON()` computes best-path status at query time
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib.go` -- `FamilyRIB`: per-family NLRI->RouteEntry storage. `Insert()`, `Remove()`, `LookupEntry()`, `IterateEntry()`

**Behavior to preserve:**
- BGP RIB per-peer storage with per-attribute dedup pools
- Best-path algorithm (RFC 4271 Phase 2) in `bestpath.go` -- pure functions unchanged
- DirectBridge zero-copy structured event delivery to RIB
- Bus content-agnostic -- payload is `[]byte`, bus never type-asserts
- Plugin registration via `init()` + `register.go`
- Existing `rib best` CLI command (on-demand query still works, future rename to `ze show bgp rib best`)

**Behavior to change:**
- BGP RIB currently computes best-path only on `rib best` query. Must track best-path in real-time and publish changes to Bus
- BGP RIB has no Bus reference. Must add ConfigureBus callback to registry.Registration and wire into RIB
- No System RIB exists. Must create as new plugin
- No FIB component exists. Must create as new plugin(s)
- No `rib/best-change/*` Bus topics exist. Must create and publish

## Data Flow (MANDATORY)

### Entry Points

| Source | Entry | Format |
|--------|-------|--------|
| BGP UPDATE | RIB `handleReceivedStructured()` | Structured event with WireUpdate |
| Static route config | (future) Static plugin | Config tree |
| OSPF SPF result | (future) OSPF plugin | Protocol-specific |

### Transformation Path

1. **BGP UPDATE received** -- reactor delivers to RIB plugin via DirectBridge
2. **RIB stores route** -- `FamilyRIB.Insert()` with deduped attribute handles
3. **Best-path recomputed (under lock)** -- RIB runs `SelectBest()` for affected prefix, compares with previous best
4. **Collect changes (under lock)** -- if best changed, append to batch (prefix, action, next-hop, priority)
5. **Release lock**
6. **Publish batch to Bus** -- one `bus.Publish("rib/best-change/bgp", batchPayload, metadata)` per UPDATE
7. **System RIB receives** -- subscribed to `rib/best-change/` prefix, receives batch event
8. **System RIB selects** -- for each change in batch, compares by admin distance (lower wins), tracks per (family, prefix) system best
9. **System best changed?** -- compare new system best with previous
10. **Publish to Bus** -- `bus.Publish("sysrib/best-change", payload, metadata)`
11. **FIB receives** -- subscribed to `sysrib/best-change`, decodes payload
12. **FIB programs kernel** -- netlink `RTM_NEWROUTE` / `RTM_DELROUTE` (Linux) or route socket (Darwin), using ze's custom `rtm_protocol` ID

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| BGP RIB -> Bus | `bus.Publish("rib/best-change/bgp", ...)` | [ ] |
| Bus -> System RIB | `consumer.Deliver([]Event)` via prefix subscription | [ ] |
| System RIB -> Bus | `bus.Publish("sysrib/best-change", ...)` | [ ] |
| Bus -> FIB | `consumer.Deliver([]Event)` via prefix subscription | [ ] |
| FIB -> OS kernel | Netlink RTM_NEWROUTE/RTM_DELROUTE (Linux) | [ ] |
| FIB -> P4 switch | gRPC/P4Runtime (future) | [ ] |

### Integration Points
- `internal/component/plugin/registry/registry.go` -- add `ConfigureBus func(ze.Bus)` callback to `Registration` struct (mirrors `ConfigureMetrics` pattern)
- `internal/component/bgp/plugins/rib/rib.go` -- RIB receives Bus via ConfigureBus, adds best-path tracking state, publishes batch events
- `pkg/ze/bus.go` -- `Bus.Publish`, `Bus.Subscribe` (existing, unchanged)
- `internal/component/plugin/registry/` -- new plugins (sysrib, fib-kernel) register here
- VRF hub instances -- FIB/System RIB plugins connect to per-VRF hub (existing pattern)

### Architectural Verification
- [ ] No bypassed layers (protocol RIB -> Bus -> System RIB -> Bus -> FIB -> kernel)
- [ ] No unintended coupling (plugins communicate only via Bus, never import each other)
- [ ] No duplicated functionality (extends existing Bus and RIB, creates new plugins)
- [ ] Zero-copy preserved where applicable (Bus payload is `[]byte`)

## Bus Topic Contract (shared reference for all children)

### Topics

| Topic | Publisher | Subscriber | When |
|-------|-----------|------------|------|
| `rib/best-change/bgp` | BGP RIB | System RIB | BGP best path for a prefix changed |
| `rib/best-change/static` | Static plugin (future) | System RIB | Static route added/removed |
| `rib/best-change/ospf` | OSPF plugin (future) | System RIB | OSPF best route changed |
| `sysrib/best-change` | System RIB | FIB plugins | System-wide best route changed |

System RIB subscribes to `rib/best-change/` prefix -- catches all protocols without enumeration.

### Metadata for Filtering

| Key | Value | Purpose |
|-----|-------|---------|
| `protocol` | `"bgp"`, `"static"`, `"ospf"` | System RIB knows the source protocol |
| `family` | `"ipv4/unicast"`, `"ipv6/unicast"` | Address family filter |

### Payload Format (JSON, kebab-case per `rules/json-format.md`)

Protocol RIB -> System RIB (`rib/best-change/<protocol>`):

Events use a batch format -- one event carries an array of changes. This avoids 900K individual Bus publishes when a full-table peer goes down.

| Field | Type | Description |
|-------|------|-------------|
| `changes` | array | Array of change objects (see below) |

Each change object:

| Field | Type | Description |
|-------|------|-------------|
| `action` | string | `"add"`, `"update"`, `"withdraw"` |
| `prefix` | string | CIDR prefix (e.g., `"10.0.0.0/24"`, `"2001:db8::/32"`) |
| `next-hop` | string | Next-hop IP address (absent for withdraw) |
| `priority` | integer | Admin distance (20 for eBGP, 200 for iBGP, 10 for static) |
| `metric` | integer | Protocol-specific metric (MED for BGP, cost for OSPF) |

System RIB -> FIB (`sysrib/best-change`):

| Field | Type | Description |
|-------|------|-------------|
| `action` | string | `"add"`, `"update"`, `"withdraw"` |
| `prefix` | string | CIDR prefix |
| `next-hop` | string | Next-hop IP address |
| `protocol` | string | Winning protocol name (e.g., `"bgp"`, `"static"`) |

### Action Semantics

| Action | Meaning |
|--------|---------|
| `"add"` | New prefix with no previous best route |
| `"update"` | Existing prefix, best route changed (different next-hop, different protocol) |
| `"withdraw"` | Last route for prefix removed, no best route remains |

## Administrative Distance Table

| Protocol | Default Priority | Notes |
|----------|-----------------|-------|
| Connected | 0 | Directly connected networks |
| Static | 10 | Administratively configured |
| eBGP | 20 | External BGP |
| OSPF | 110 | Future |
| IS-IS | 115 | Future |
| iBGP | 200 | Internal BGP |

Lower value wins. Configurable per protocol via YANG config. BGP RIB sets priority per route using PeerMeta (PeerASN vs LocalASN) to distinguish eBGP from iBGP.

## FIB Backend Model

### OS Kernel Backends (build-tag selected)

| OS | File | Mechanism |
|----|------|-----------|
| Linux | `backend_linux.go` | Netlink `RTM_NEWROUTE` / `RTM_DELROUTE` via `vishvananda/netlink` |
| Darwin | `backend_darwin.go` | Route socket `RTM_ADD` / `RTM_DELETE` via `syscall` |

One binary per OS. Same pattern as `spec-iface-0-umbrella` interface plugin.

### Cross-OS Backends (generic Go)

| Backend | Plugin | Mechanism |
|---------|--------|-----------|
| P4 | `fib-p4` | gRPC / P4Runtime (future) |

Cross-OS backends are separate plugins, not build variants of fib-kernel.

### Crash Recovery (stale-mark-then-sweep)

On ungraceful termination (SIGKILL, OOM, power loss), routes installed by fib-kernel are orphaned in the kernel. A blind sweep on startup would cause a traffic blackhole until BGP reconverges.

| Step | Action |
|------|--------|
| 1. Install routes | All ze routes use a custom Linux `rtm_protocol` ID (e.g., `RTPROT_ZE=250`) |
| 2. On startup | Scan kernel for routes with ze's protocol ID, mark them stale with a short timer |
| 3. BGP reconverges | System RIB publishes best routes. FIB matches against stale routes -- if they match, clear stale mark (no kernel change needed) |
| 4. Timer expires | Remove routes still marked stale (BGP did not reconfirm them) |

This preserves the forwarding plane during ze restart. Only genuinely stale routes are removed.

For Darwin: no `rtm_protocol` equivalent. Route table ID or tag convention to be defined in `spec-fib-3-kernel.md`.

### VRF Integration

No FIB-specific VRF logic. Per `spec-vrf-0-umbrella`, each VRF gets its own hub instance. The VRF plugin instantiates FIB plugins per VRF. Each FIB instance receives events scoped to its VRF. The kernel backend accepts a VRF name parameter for route installation (`ip route add ... vrf <name>`). Default VRF uses the main routing table.

## Relationship to Other Specs

| Spec | Relationship |
|------|-------------|
| `spec-rib-05-best-path.md` | Phase 1 supersedes the on-demand approach. Best-path algorithm is reused, tracking becomes real-time |
| `spec-iface-0-umbrella.md` | No overlap. iface = interface lifecycle (`interface/*` topics). FIB = route installation (`sysrib/*` topics) |
| `spec-vrf-0-umbrella.md` | VRF provides per-VRF hub instances. FIB plugins connect to their VRF's hub. No FIB-specific logic |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test | Phase |
|-------------|---|--------------|------|-------|
| BGP UPDATE changes best path | -> | RIB tracks best, `ze show bgp rib` shows it | `test/plugin/fib-rib-event.ci` | 1 |
| Protocol RIB publishes change | -> | System RIB selects by priority, `ze show rib` shows winner | `test/plugin/fib-sysrib.ci` | 2 |
| System RIB publishes best | -> | fib-kernel installs route in OS | `test/plugin/fib-kernel.ci` | 3 |

## Acceptance Criteria

| AC ID | Phase | Input / Condition | Expected Behavior |
|-------|-------|-------------------|-------------------|
| AC-1 | 1 | BGP UPDATE makes prefix 10.0.0.0/24 best path change | RIB publishes `rib/best-change/bgp` with action `"add"` or `"update"` and correct next-hop |
| AC-2 | 1 | BGP withdraws last route for prefix | RIB publishes `rib/best-change/bgp` with action `"withdraw"` |
| AC-3 | 1 | BGP UPDATE does not change best path | No event published (same best, no change) |
| AC-4 | 2 | System RIB receives `rib/best-change/bgp` (eBGP priority 20) | Installs as system best if no lower-priority route exists for that prefix |
| AC-5 | 2 | System RIB has static (priority 10) and eBGP (priority 20) for same prefix | Static wins. System best uses static next-hop |
| AC-6 | 2 | Static route withdrawn, BGP route still exists for prefix | BGP becomes system best. System RIB publishes `sysrib/best-change` with action `"update"` |
| AC-7 | 2 | All routes withdrawn for prefix | System RIB publishes `sysrib/best-change` with action `"withdraw"` |
| AC-8 | 3 | `sysrib/best-change` with action `"add"` for 10.0.0.0/24 | fib-kernel installs route via netlink (Linux) or route socket (Darwin) |
| AC-9 | 3 | `sysrib/best-change` with action `"withdraw"` | fib-kernel removes route from OS |
| AC-10 | 3 | `sysrib/best-change` with action `"update"` | fib-kernel replaces route (delete old, add new, or replace) |
| AC-11 | 1 | Peer goes down, all its routes withdrawn | RIB publishes single batch event with withdraws for all affected prefixes where best path changed |
| AC-12 | 2 | Admin distance configurable via YANG | Config `sysrib { protocol ebgp { priority 30; } }` overrides default 20 |
| AC-13 | 3 | fib-kernel plugin starts on Linux | Opens netlink socket for route management |
| AC-14 | 3 | fib-kernel plugin stops gracefully | Configurable: leave routes installed or flush on shutdown |
| AC-15 | 3 | fib-kernel starts after crash (ze routes exist in kernel) | Marks existing ze routes as stale, refreshes matching routes as BGP reconverges, removes stale routes after timer |
| AC-16 | 3 | All ze routes use custom `rtm_protocol` ID | `ip route show proto ze` lists only ze-installed routes |
| AC-17 | 1 | `ze show bgp rib` CLI command | Shows BGP RIB best routes per prefix with next-hop and attributes |
| AC-18 | 2 | `ze show rib` CLI command | Shows system RIB best routes per prefix with winning protocol and priority |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Phase | Validates |
|------|------|-------|-----------|
| `TestRIBBestChangePublish` | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | 1 | RIB publishes event when best path changes |
| `TestRIBBestChangeNoPublishSameBest` | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | 1 | No event when best path unchanged |
| `TestRIBBestChangeWithdraw` | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | 1 | Withdraw event when last route removed |
| `TestSysRIBSelectByPriority` | `internal/plugins/sysrib/sysrib_test.go` | 2 | Lower admin distance wins |
| `TestSysRIBFallback` | `internal/plugins/sysrib/sysrib_test.go` | 2 | Higher-priority route takes over on withdraw |
| `TestSysRIBPublishChange` | `internal/plugins/sysrib/sysrib_test.go` | 2 | Publishes `sysrib/best-change` on system best change |
| `TestRIBBestChangeBatchPeerDown` | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | 1 | Peer down produces single batch with all affected prefix withdrawals |
| `TestRIBBestChangeEBGPPriority` | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | 1 | eBGP routes published with priority 20 |
| `TestRIBBestChangeIBGPPriority` | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | 1 | iBGP routes published with priority 200 |
| `TestSysRIBConfigOverride` | `internal/plugins/sysrib/sysrib_test.go` | 2 | YANG config overrides default admin distance |
| `TestFIBKernelInstall` | `internal/plugins/fib-kernel/fib_test.go` | 3 | Route installed via backend |
| `TestFIBKernelRemove` | `internal/plugins/fib-kernel/fib_test.go` | 3 | Route removed via backend |
| `TestFIBKernelReplace` | `internal/plugins/fib-kernel/fib_test.go` | 3 | Route replaced on update |
| `TestFIBKernelStartupSweep` | `internal/plugins/fib-kernel/fib_test.go` | 3 | On startup, marks existing ze routes stale, refreshes matches, removes expired |
| `TestFIBKernelProtocolID` | `internal/plugins/fib-kernel/fib_test.go` | 3 | All installed routes use ze custom rtm_protocol ID |
| `TestFIBKernelFlushOnStop` | `internal/plugins/fib-kernel/fib_test.go` | 3 | Graceful stop with flush-on-stop=true removes routes |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Admin distance | 0-255 | 255 | N/A (0 is valid = connected) | 256 |
| IPv4 prefix length | 0-32 | 32 | N/A | 33 |
| IPv6 prefix length | 0-128 | 128 | N/A | 129 |

### Functional Tests

| Test | Location | Phase | End-User Scenario |
|------|----------|-------|-------------------|
| `test-fib-rib-event` | `test/plugin/fib-rib-event.ci` | 1 | BGP route learned, `ze show bgp rib` shows best route with correct next-hop |
| `test-fib-sysrib` | `test/plugin/fib-sysrib.ci` | 2 | `ze show rib` shows system best route with winning protocol and priority |
| `test-fib-kernel` | `test/plugin/fib-kernel.ci` | 3 | Route installed in OS routing table (requires CAP_NET_ADMIN or dry-run mode) |
| `test-fib-startup` | `test/plugin/fib-startup.ci` | 3 | After restart, stale routes swept after timer |

## Files to Modify

- `internal/component/plugin/registry/registry.go` -- add `ConfigureBus func(ze.Bus)` to `Registration` struct (Phase 1)
- `internal/component/bgp/plugins/rib/rib.go` -- add Bus reference, best-path tracking state, batch event publishing (Phase 1)
- `internal/component/bgp/plugins/rib/rib_structured.go` -- trigger best-path recomputation after insert/remove, collect changes under lock (Phase 1)
- `internal/component/bgp/plugins/rib/register.go` -- add ConfigureBus callback, EventTypes for `rib/best-change/bgp` (Phase 1)
- `internal/component/plugin/all/all.go` -- blank imports for sysrib, fib-kernel (auto-generated)
- `scripts/gen-plugin-imports.go` -- scan `internal/plugins/` in addition to `internal/component/bgp/plugins/` (Phase 2)

## Files to Create

| File | Phase | Purpose |
|------|-------|---------|
| `internal/component/bgp/plugins/rib/rib_bestchange.go` | 1 | Best-path tracking + Bus event publishing |
| `internal/plugins/sysrib/sysrib.go` | 2 | System RIB plugin: admin distance selection |
| `internal/plugins/sysrib/register.go` | 2 | `init()` registration |
| `internal/plugins/sysrib/schema/ze-sysrib-conf.yang` | 2 | YANG config (admin distance overrides) |
| `internal/plugins/fib-kernel/fib.go` | 3 | FIB kernel plugin: route management dispatch |
| `internal/plugins/fib-kernel/register.go` | 3 | `init()` registration |
| `internal/plugins/fib-kernel/backend_linux.go` | 3 | Netlink route add/del/replace |
| `internal/plugins/fib-kernel/backend_darwin.go` | 3 | Route socket add/del |
| `internal/plugins/fib-kernel/schema/ze-fib-conf.yang` | 3 | YANG config (flush-on-stop, etc.) |

### Integration Checklist

| Integration Point | Needed? | File | Phase |
|-------------------|---------|------|-------|
| YANG schema (sysrib) | [ ] | `internal/plugins/sysrib/schema/ze-sysrib-conf.yang` | 2 |
| YANG schema (fib-kernel) | [ ] | `internal/plugins/fib-kernel/schema/ze-fib-conf.yang` | 3 |
| CLI commands | [ ] | `ze show rib` (system RIB), `ze show bgp rib` (BGP RIB) | 1-2 |
| Functional tests | [ ] | `test/plugin/fib-*.ci` | 1-3 |
| Plugin registration | [ ] | `register.go` files + `all/all.go` | 1-3 |
| ConfigureBus callback | [ ] | `internal/component/plugin/registry/registry.go` | 1 |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- FIB / route installation |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` -- sysrib + fib stanzas |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- `ze show rib`, `ze show bgp rib` |
| 4 | API/RPC added/changed? | No | -- |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` -- sysrib, fib-kernel |
| 6 | Has a user guide page? | Yes | `docs/guide/fib.md` -- new |
| 7 | Wire format changed? | No | -- |
| 8 | Plugin SDK/protocol changed? | No | -- |
| 9 | RFC behavior implemented? | No | -- (best-path RFC already documented) |
| 10 | Test infrastructure changed? | No | -- |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- FIB support |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` -- FIB pipeline |

## Implementation Steps

Implementation follows child specs in order. Each phase must be complete before the next begins.

1. Phase 1: `spec-fib-1-rib-events.md` -- BGP RIB real-time best-path tracking + Bus event publishing
2. Phase 2: `spec-fib-2-sysrib.md` -- System RIB plugin + admin distance selection + YANG config
3. Phase 3: `spec-fib-3-kernel.md` -- fib-kernel plugin + OS route programming
4. Phase 4: `spec-fib-4-p4.md` -- fib-p4 plugin (deferred until needed)

### Critical Review Checklist (see child specs for phase-specific checks)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Bus wiring | ConfigureBus callback exists and RIB receives Bus reference |
| Batch correctness | Changes collected under lock, published after release |
| Admin distance | eBGP=20, iBGP=200 correctly applied per route |
| Crash recovery | Custom rtm_protocol ID used, startup sweep works |
| CLI | `ze show rib` and `ze show bgp rib` return correct data |

### Deliverables Checklist (see child specs for phase-specific checks)

| Deliverable | Verification method |
|-------------|---------------------|
| ConfigureBus callback on Registration | grep ConfigureBus internal/component/plugin/registry/registry.go |
| BGP RIB publishes batch events | TestRIBBestChangePublish passes |
| sysrib plugin registered | grep sysrib internal/component/plugin/all/all.go |
| fib-kernel plugin registered | grep fib-kernel internal/component/plugin/all/all.go |
| `ze show rib` command works | test/plugin/fib-sysrib.ci passes |
| Startup sweep works | TestFIBKernelStartupSweep passes |

### Security Review Checklist (see child specs for phase-specific checks)

| Check | What to look for |
|-------|-----------------|
| Input validation | Malformed JSON in Bus events must not crash sysrib or fib-kernel |
| Privilege escalation | fib-kernel netlink calls require CAP_NET_ADMIN. Validate ze runs with correct capabilities |
| Resource exhaustion | Batch events with unbounded prefix count. Limit batch size or use streaming |
| Route injection | Malicious plugin publishing fake `rib/best-change/*` events. Bus access control |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in phase that introduced it |
| Test fails wrong reason | Fix test assertion |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural -> DESIGN |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to IMPLEMENT for that criterion |

## Design Insights

### Relationship to freeRtr

| Pattern | freeRtr | Ze |
|---------|---------|---|
| Protocol RIB | Per-protocol route tables | Per-protocol RIB plugins publishing to Bus |
| System RIB | `tabRoute` with admin distance | sysrib plugin with priority selection |
| FIB programming | `ifcBridge` / `tapInt` | fib-kernel (netlink/route socket) |
| Multi-backend | `tabRoute` + multiple `fwd*` handlers | Separate FIB plugins, each subscribing independently |
| VRF | Per-VRF `tabRoute` + `tabFib` | Per-VRF hub instances, FIB connects to VRF's hub |

### Best-Path Transition: On-Demand to Real-Time

The existing `bestpath.go` algorithm is a pure function (`SelectBest`, `ComparePair`). Phase 1 does not change the algorithm -- it adds tracking state (previous best per (family, prefix) pair), collects changes under the RIB lock, and publishes a batch Bus event after lock release. The existing `rib best` command continues to work (on-demand query).

### Batch Event Performance Model

One BGP UPDATE may affect multiple prefixes (MP_REACH + MP_UNREACH). One peer-down may affect hundreds of thousands of prefixes. The batch format ensures one Bus channel send per event, regardless of how many prefixes changed. Changes are collected in memory under the RIB lock (fast), marshaled and published after lock release (no contention).

### CLI: `ze show bgp rib` / `ze show rib`

`ze show bgp rib` displays the BGP protocol RIB -- best BGP route per prefix with next-hop, attributes, peer source. `ze show rib` displays the system RIB -- best route per prefix across all protocols, showing the winning protocol and admin distance. Both are observable in .ci functional tests via standard `expect=json:` directives.

### System RIB is Intentionally Simple

The System RIB's only job is selecting between protocols by admin distance. Within a protocol, the protocol's own RIB has already selected the best route. The System RIB receives one route per prefix per protocol -- never multiple candidates from the same protocol.

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
- [ ] AC-1..AC-18 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

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
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-fib-0-umbrella.md`
- [ ] **Summary included in commit**
