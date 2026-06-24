# Spec: ospf-ext-10 -- BFD for OSPFv2 (RFC 5880, RFC 5881)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospf-0-umbrella.md (delivered) |
| Phase | - |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `rfc/short/rfc5881.md` -- single-hop encapsulation: UDP 3784, both ends Active (§3), TTL/Hop-Limit = 255 GTSM (§5), one session per data protocol (§2), Your-Discriminator demux (§6)
4. `rfc/short/rfc5880.md` -- base BFD: session states Down/Init/Up/AdminDown, Diag codes (1 = detect-expired, 3 = neighbor-signaled-down), the failure detector contract; ze already implements the full engine (`internal/component/bfd`)
5. `internal/component/bgp/reactor/peer_bfd.go` -- the EXEMPLAR client: `startBFDClient`/`stopBFDClient`/`runBFDSubscriber`/`bfdRequestFor`; this spec mirrors it for OSPF
6. `internal/component/bfd/api/service.go` + `registry.go` -- `GetService()`, `Service.EnsureSession(SessionRequest)`, `SessionHandle.Subscribe()/Unsubscribe()`, `Service.ReleaseSession`; `StateChange{State, Diag}`
7. `internal/plugins/ospf/neighbor/table.go` -- `Table.NeighborDown(interfaceName, id types.RouterID)` is the NSM down-event injection seam (emits `kill-nbr`, drops the neighbor to Down)
8. `internal/plugins/ospf/instance.go` -- `neighborEventSink` (NeighborUp/NeighborDown), `nsmAdapter`, `neighborInterfaceConfig`, `e.neighbors` Table wiring; this is where BFD session lifecycle hangs off NSM transitions
9. `internal/plugins/ospf/config.go` -- `interfaceConfig` struct (line ~141), `parseInterface` (line ~633); the per-interface BFD enable + timer leaves are added here
10. `plan/learned/560-bfd-3-bgp-client.md` -- the design record for the BGP-BFD client this spec copies (atomic-pointer Service lookup, graceful degradation, per-session subscriber goroutine, FSM-callback hook)

## Task

Integrate OSPFv2 neighbor liveness with Ze's existing, delivered BFD engine
(`internal/component/bfd`). When an OSPFv2 adjacency reaches a usable state
(Full, the point at which the neighbour is actually carrying topology), the OSPF
engine registers a **single-hop asynchronous** BFD session for that neighbour
(RFC 5881: UDP 3784, both ends Active, TTL = 255). When BFD reports the session
**Down** (or AdminDown), OSPF injects the NSM "neighbour down" event immediately
(`neighbor.Table.NeighborDown`), declaring the neighbour dead far faster than
the RouterDeadInterval timer (typically 40 s) would. The session is released
when the adjacency drops for any other reason, when BFD is disabled, or when the
interface goes down.

The work mirrors the BGP-BFD client (`peer_bfd.go`, learned 560) almost
one-for-one, substituting OSPF entry points for BGP's: the FSM-callback hook
becomes the NSM Up/Down transition, `Peer.Teardown(NotifyCeaseBFDDown,...)`
becomes `Table.NeighborDown(iface, routerID)`, and `PeerSettings.BFD` becomes a
per-interface OSPF `bfd` config block. BFD is strictly **additive**: if the BFD
plugin is not loaded (`api.GetService()` returns nil), or BFD is not enabled on
the interface, OSPF runs exactly as today on the Hello/Dead timers alone.

### In scope (this spec)

| Item | Detail |
|------|--------|
| BFD session lifecycle tied to NSM | Open a single-hop session when a neighbour reaches Full; release it when the neighbour leaves Full / interface goes down / config disables BFD. Hook off `neighborEventSink.NeighborUp` / `NeighborDown` in `instance.go` |
| Down-event injection path | A BFD `StateDown`/`StateAdminDown` change drives `Table.NeighborDown(interfaceName, neighborRouterID)` -- the SAME seam `nsmAdapter.NeighborDown` already uses -- so the neighbour drops to Down through the existing NSM, not a new code path |
| Per-interface config surface | A `bfd` container on the OSPF `interface` list: `enabled`, `min-tx`, `min-rx`, `multiplier` leaves; resolved into `interfaceConfig` -> `iface.Config` -> `neighbor.InterfaceConfig` and consumed by the engine when opening sessions |
| Single-hop session request | `api.SessionRequest{Peer: neighbour address, Local: interface address, Interface: ifname, Mode: SingleHop, DesiredMinTxInterval, RequiredMinRxInterval, DetectMult}` built from the neighbour Snapshot + the interface BFD config |
| In-process Service lookup | Reuse `api.GetService()` (already published by `RunBFDPlugin.OnStarted`); nil-safe graceful degradation identical to BGP |
| Per-session subscriber goroutine | One long-lived worker per BFD-protected neighbour draining `<-chan StateChange`; translates Down -> `NeighborDown`; goroutine-lifecycle compliant, mirrors `runBFDSubscriber` |
| Metrics | `ze_ospf_bfd_sessions` gauge, `ze_ospf_bfd_session_down_total` counter, `ze_ospf_bfd_register_failures_total` counter |
| CLI surface | BFD enable + session state visible via the existing `show ip ospf interface` / `show ip ospf neighbor` outputs (a `bfd` column / flag), no new top-level command |

### Out of scope (noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| BFD echo function (RFC 5880 §6.4) | BFD engine concern; `spec-bfd-6-echo-mode` (delivered). OSPF does not request echo |
| BFD multi-hop (RFC 5883) | engine concern; OSPF adjacencies are single-hop by definition (directly-connected neighbours). The request is always `Mode: SingleHop` |
| BFD authentication wiring | engine concern; `spec-bfd-5-authentication` (delivered). OSPF sessions inherit no auth in this spec |
| BFD timer negotiation, Poll/Final, slow-start | all inside `internal/component/bfd` (delivered); OSPF only supplies desired min-tx / min-rx / multiplier and never touches the wire |
| OSPFv3 BFD (the v6 family) | a sibling spec can mirror this against `internal/plugins/ospf/v3` / `afstrategy_v6.go`; this spec is OSPFv2 only. The config plumbing is laid so the v6 family can reuse it, but no v6 session is opened here |
| BFD on an OSPF interface that never reaches Full (DR/BDR election only, 2-Way DROther pairs) | by design only Full adjacencies (the ones carrying flooding) get a BFD session; 2-Way-only neighbours on a broadcast LAN do not |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
- [ ] `docs/research/ospf-implementation-guide.md` lines 1542-1545 ("BFD Integration (RFC 5880, RFC 5881)") -- "Implemented as a thin wrapper around zebra's BFD subsystem. BFD failure triggers an NSM event that declares the neighbour down immediately."
  -> Decision: model BFD-for-OSPF as a thin client over the EXISTING `internal/component/bfd` engine, exactly as FRR wraps zebra's BFD subsystem; no new BFD logic, only the NSM <-> session glue
  -> Constraint: the failure action is "an NSM event that declares the neighbour down immediately" -- i.e. drive `Table.NeighborDown`, never invent a parallel teardown
- [ ] `plan/learned/560-bfd-3-bgp-client.md` -- the BGP-BFD client this spec mirrors
  -> Decision: hook the session lifecycle at the same architectural layer BGP used (the FSM/NSM transition callback), not at a lower packet layer; `startBFD` after the state is set, `stopBFD` on leaving the usable state
  -> Constraint: graceful degradation is mandatory -- a missing BFD plugin (`GetService()==nil`) MUST NOT block OSPF; the adjacency runs on Hello/Dead timers alone and logs a warning once
  -> Constraint: the subscriber is a long-lived per-session worker (one per BFD-protected neighbour), not per-event; `stopBFD` closes a stop chan and waits on a done chan so the goroutine has exited before the handle is released
- [ ] `ai/rules/plugin-self-containment.md` -- removing the OSPF plugin removes all its BFD wiring
  -> Constraint: all BFD-for-OSPF code lives under `internal/plugins/ospf`; the only outside dependency is `internal/component/bfd/api` (a leaf package). No OSPF spelling appears in `internal/component/bfd`
- [ ] `ai/rules/no-sprintf-alloc.md` -- no `fmt`/`+` on any hot path
  -> Constraint: the per-packet BFD path is entirely inside the BFD engine; OSPF's only frequency is per-NSM-transition (rare). Session-state log lines use structured slog fields, never `fmt.Sprintf` string building

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5881.md` -- BFD for IPv4/IPv6 single hop
  -> Constraint: §3 both ends MUST take the Active role; OSPF supplies `Passive: false` (the api default) so the session comes up symmetrically with FRR
  -> Constraint: §2 a separate BFD session MUST exist per data protocol over a link; OSPFv2 (IPv4) opens an IPv4 single-hop session keyed on (peer addr, local addr, interface, vrf, single-hop). OSPFv3 (IPv6) is a separate session in a sibling spec
  -> Constraint: §5 single-hop sessions transmit TTL/Hop-Limit = 255 and discard packets with TTL != 255 (GTSM). This is enforced inside `internal/component/bfd` (SingleHop mode); OSPF only selects `Mode: SingleHop`
  -> Constraint: §4 the session is bound to ONE egress interface; OSPF passes the adjacency's interface name as `SessionRequest.Interface`, matching the interface the Hello arrived on
- [ ] `rfc/short/rfc5880.md` -- base BFD protocol
  -> Constraint: §6.8.1 the session declares Down with Diag 1 (Control Detection Time Expired) on timer miss and Diag 3 (Neighbor Signaled Session Down) when the peer reports Down; OSPF treats BOTH `StateDown` and `StateAdminDown` as "neighbour down" regardless of Diag, matching the BGP client
  -> Constraint: §6.8.3 slow-start floors DesiredMinTxInterval at 1 s until the session reaches Up; OSPF's configured `min-tx` only takes effect after Up. A freshly-formed adjacency therefore detects in up to `multiplier * 1 s` until fast rates negotiate -- this is the engine's contract, not an OSPF bug
  -> Constraint: BFD is a "failure detector, not a session driver" (RFC 5882 client model): OSPF acts ONLY on Down. A BFD `Up`/`Init` transition is logged at debug and does NOT itself bring the OSPF adjacency up (the NSM owns that)

**Key insights:** (minimal context to resume after compaction)
- BFD-for-OSPF is glue, not protocol: the engine is delivered; this spec wires NSM Full -> EnsureSession and BFD Down -> `Table.NeighborDown`.
- The exact down-injection seam already exists: `Table.NeighborDown(interfaceName, id)` is called today by `nsmAdapter.NeighborDown`; the BFD subscriber calls the same `Table` method.
- The exemplar is `peer_bfd.go`; copy its structure (start/stop/subscriber/request-builder, nil-safe degradation, mutex-guarded per-session state, stop+done channel handshake).
- Only Full adjacencies get a session; only Down/AdminDown drives a teardown.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/plugins/ospf/neighbor/table.go` -- `Table` owns the per-interface neighbour map; `NeighborDown(interfaceName, id types.RouterID)` looks up the neighbour, records the `kill-nbr` NSM event, and `setStateLocked(n, stateDown)`; `setStateLocked` emits a `down` `eventEmission` when a Full neighbour drops, which `emit` forwards to `EventSink.NeighborDown`. `Snapshot()` returns `[]Snapshot` with `Interface`, `RouterID` (string), `Address` (string), `State`
  -> Constraint: `NeighborDown` is the EXACT injection point; the BFD subscriber calls it with the neighbour's interface + Router ID. No new NSM event or state is needed
  -> Constraint: the neighbour's reachable `Address` (its source IP, a `netip.Addr`) is on the internal `Neighbor` struct and surfaced in `Snapshot.Address`; this is the BFD session `Peer` address
- [ ] `internal/plugins/ospf/neighbor/neighbor.go` -- `Snapshot{Interface, Area, RouterID, State, Address, ...}`; `Neighbor.Address netip.Addr`; `FloodNeighbor{RouterID, Address, State, InterfaceID}`; the `state` enum (`stateFull` is the usable state); `Table.AddressOf(id)` returns the neighbour's reachable address
  -> Constraint: the `Snapshot.Address` field is a string; the engine needs the raw `netip.Addr` to build the `SessionRequest`. Either widen the up/down event payload to carry `netip.Addr` + interface + Router ID, or look it up via `Table.Lookup`/a new typed accessor. Prefer a typed lifecycle callback carrying the raw address (see Data Flow)
- [ ] `internal/plugins/ospf/instance.go` -- `e.neighbors = ospfneighbor.NewTable(...)`; `e.neighbors.SetEventSink(neighborEventSink{sink: e.sink, onChange: e.originateSelfLSAs})`; `neighborEventSink.NeighborUp/NeighborDown` forward to the report-bus sink and trigger LSA re-origination; `nsmAdapter` adapts iface events into `Table` calls; `neighborInterfaceConfig(cfg)` maps `iface.Config` -> `neighbor.InterfaceConfig`
  -> Constraint: `neighborEventSink` is the single chokepoint that observes EVERY Full<->non-Full transition; the BFD lifecycle (start on up, stop on down) hangs here. The sink must learn the neighbour's raw address + interface to open/close sessions
  -> Constraint: `neighborInterfaceConfig` is where per-interface BFD config (enabled, timers) flows from `iface.Config` into the neighbour layer; the engine reads it to decide whether to open a session
- [ ] `internal/plugins/ospf/config.go` -- `interfaceConfig` struct (line ~141: Name, AreaID, HelloInterval, DeadInterval, RetransmitInterval, ...); `parseInterface(entry)` (line ~633) reads `hello-interval`/`dead-interval`/`retransmit-interval` via `configNumber`; `DefaultDeadInterval = 40`
  -> Constraint: add a `BFD bfdInterfaceConfig` field to `interfaceConfig` and parse a nested `bfd` map in `parseInterface`; default disabled (BFD is opt-in, matching BGP's presence-container model)
- [ ] `internal/plugins/ospf/iface/iface.go` -- `Config` struct (line ~70: Name, AreaID, DeadInterval, RetransmitInterval, ...) is the per-interface runtime config the FSM/Hello layer consumes
  -> Constraint: the BFD enable + timers must ride through `iface.Config` so the engine sees them when it opens a neighbour session; add the fields and map them in `interfaceRuntimeConfigLocked` / `neighborInterfaceConfig`
- [ ] `internal/component/bfd/api/service.go`, `registry.go`, `events.go` -- `GetService() Service`; `Service.EnsureSession(SessionRequest) (SessionHandle, error)`; `Service.ReleaseSession(SessionHandle) error`; `SessionHandle.Subscribe() <-chan StateChange` / `Unsubscribe`; `SessionRequest{Peer, Local, Interface, VRF, Mode, DesiredMinTxInterval, RequiredMinRxInterval, DetectMult, ...}`; `StateChange{Key, State, Diag, When}`; `StateDown`/`StateAdminDown`; `SingleHop`
  -> Constraint: this is the FROZEN client contract BGP already uses (learned 560 "Service interface surface is frozen for this path"); OSPF reuses it verbatim, adding NO methods to `api`
  -> Constraint: `EnsureSession` refcounts on the `Key` tuple; two OSPF neighbours on different addresses produce distinct keys, so each adjacency gets its own session
- [ ] `internal/component/bgp/reactor/peer_bfd.go` -- the EXEMPLAR: `bfdClient{mu, svc, handle, sub, stop, done}`; `startBFDClient` (nil-safe GetService, EnsureSession, Subscribe, spawn `runBFDSubscriber`); `runBFDSubscriber` (drain sub; Down/AdminDown -> `Teardown`); `stopBFDClient` (close stop, Unsubscribe, ReleaseSession, wait done); `bfdRequestFor(settings)` builds the request
  -> Constraint: copy this structure verbatim, substituting OSPF types: per-neighbour `bfdClient` keyed by (interface, Router ID); `Teardown(...)` -> `table.NeighborDown(iface, id)`; `bfdRequestFor(settings)` -> `bfdRequestForNeighbor(snap, ifcfg)`
- [ ] `internal/plugins/ospf/register.go` -- `registerOSPF()` registers the namespace, doctor, diagnostic codes, completions, config sections; `runOSPFEngine(conn)` is the SDK entry point; metrics registry is set via `setMetricsRegistry`
  -> Constraint: the new `ze_ospf_bfd_*` metrics register here alongside the existing OSPF metric set; the OSPF doctor gains an informational check that BFD is configured-but-plugin-absent

**Behavior to preserve:**
- OSPF on Hello/Dead timers alone when BFD is not enabled or the BFD plugin is absent (additive-only contract). Every existing OSPF functional/interop test must stay green with no config change.
- The NSM state machine and `Table.NeighborDown` semantics (a BFD-driven down is indistinguishable from an inactivity-timer-driven down to the rest of OSPF -- same `kill-nbr` event, same LSA re-origination, same SPF re-run).
- `neighborEventSink.NeighborUp/NeighborDown` continuing to re-originate self-LSAs and emit report-bus events; the BFD lifecycle is layered ON TOP, not in place of, these.
- The frozen `internal/component/bfd/api` surface (no new methods).

**Behavior to change:** (only what the task requires)
- `interfaceConfig` / `iface.Config` / `neighbor.InterfaceConfig` gain BFD fields (enabled + timers).
- `neighborEventSink` (or an equivalent lifecycle observer) opens a BFD session on Full and releases it on leaving Full.
- A BFD Down transition now drives `Table.NeighborDown` (a new, faster trigger for an existing transition).
- `show ip ospf neighbor` / `show ip ospf interface` surface BFD session state (additive columns).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Config:** `ospf { interface eth0 { bfd { enabled true; min-tx 50; min-rx 50; multiplier 3 } } }` enters via the OSPF SDK config sections -> `parseInterface` -> `interfaceConfig.BFD`.
- **Adjacency up:** a neighbour reaches Full -> `Table.setStateLocked` emits `up` -> `emit` -> `neighborEventSink.NeighborUp(snap)` -> BFD lifecycle opens a session.
- **BFD down:** the BFD engine declares the session Down -> `StateChange{State: StateDown}` on the subscription channel -> subscriber -> `Table.NeighborDown(interface, routerID)`.
- **Adjacency down (any other cause):** `neighborEventSink.NeighborDown(snap)` -> BFD lifecycle releases the session.

### Transformation Path
1. **Config parse:** `parseInterface` reads the nested `bfd` map (`enabled`, `min-tx`, `min-rx`, `multiplier`) into `interfaceConfig.BFD` (defaults: disabled; min-tx/min-rx 50 000 us; multiplier 3, matching common practice and the BGP defaults).
2. **Config flow:** `interfaceConfig.BFD` -> `iface.Config.BFD` (via `interfaceRuntimeConfigLocked`) -> `neighbor.InterfaceConfig.BFD` (via `neighborInterfaceConfig`). The engine retains a per-interface BFD config map.
3. **Session open (Full):** on `NeighborUp(snap)`, if `snap` is on a BFD-enabled interface AND `api.GetService() != nil`, build `bfdRequestForNeighbor(snap, ifcfg)` = `SessionRequest{Peer: neighbourAddr, Local: interfaceAddr, Interface: ifname, Mode: SingleHop, DesiredMinTxInterval: minTx, RequiredMinRxInterval: minRx, DetectMult: multiplier}`, call `EnsureSession`, `Subscribe`, spawn `runBFDSubscriber`. Store the per-neighbour `bfdClient` keyed by (interface, Router ID).
4. **Subscriber loop:** drains `<-chan StateChange`. On `StateDown`/`StateAdminDown`: log a warning, increment `ze_ospf_bfd_session_down_total`, call `Table.NeighborDown(interface, routerID)`. On Up/Init: log at debug. Exits on stop-chan or channel close.
5. **NSM down:** `Table.NeighborDown` runs the existing path: `kill-nbr` event, neighbour -> Down, `down` emission -> `neighborEventSink.NeighborDown` (which ALSO releases the BFD session in step 6, idempotently) -> self-LSA re-origination -> SPF re-run -> route withdrawal.
6. **Session release:** on `NeighborDown(snap)` (whether BFD-driven or timer-driven), the lifecycle looks up the per-neighbour `bfdClient`, closes its stop chan, `Unsubscribe`s, `ReleaseSession`s, waits the done chan, and forgets it. Idempotent: a no-op if no session was open.
7. **Interface down / config disable:** `InterfaceDown` / a reload that clears `bfd.enabled` releases every session on that interface through the same release path.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| OSPF config <-> interface config | nested `bfd` map parsed in `parseInterface` into `interfaceConfig.BFD`; flows to `iface.Config` + `neighbor.InterfaceConfig` | [ ] |
| NSM Full transition <-> BFD lifecycle | `neighborEventSink.NeighborUp/NeighborDown` open/close sessions; needs the raw neighbour `netip.Addr` + interface | [ ] |
| OSPF engine <-> BFD engine | `api.GetService().EnsureSession/ReleaseSession`; `SessionHandle.Subscribe/Unsubscribe`; value-typed `SessionRequest`/`StateChange` (no cross-boundary pointers) | [ ] |
| BFD Down <-> NSM down | subscriber calls `Table.NeighborDown(interface, routerID)` -- the existing injection seam | [ ] |
| Engine <-> Service availability | `GetService()` nil-check; OSPF degrades to timer-only and logs once | [ ] |

### Integration Points
- `internal/plugins/ospf/config.go` -- `interfaceConfig.BFD`, `parseInterface` (consumes the YANG `bfd` container).
- `internal/plugins/ospf/iface/iface.go` -- `Config.BFD` (carries enable + timers to the runtime).
- `internal/plugins/ospf/neighbor` -- `InterfaceConfig.BFD`; the up/down lifecycle needs the neighbour's raw `netip.Addr` (extend the event payload or add a typed accessor); `NeighborDown` is the down seam (consumed, not changed).
- `internal/plugins/ospf/instance.go` -- the BFD lifecycle observer (open on Full, close on down); reads the per-interface BFD config; owns the per-neighbour `bfdClient` map.
- `internal/component/bfd/api` -- `GetService`, `Service`, `SessionHandle`, `SessionRequest`, `StateChange` (consumed verbatim; frozen surface).
- `internal/plugins/ospf/register.go` -- the three `ze_ospf_bfd_*` metrics; doctor informational check.

### Architectural Verification
- [ ] No bypassed layers (BFD down flows through `Table.NeighborDown` and the existing NSM, not a side path)
- [ ] No unintended coupling (OSPF imports only `bfd/api`; `internal/component/bfd` names no OSPF symbol)
- [ ] No duplicated functionality (reuses the delivered BFD engine, the `api.Service` lookup, the existing NSM down seam; the client is a near-copy of `peer_bfd.go`)
- [ ] Zero-copy preserved where applicable (value-typed `SessionRequest`/`StateChange`; no per-packet OSPF involvement)

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `api.GetService()` + `Service.EnsureSession`/`ReleaseSession`/`SessionHandle.Subscribe` is a frozen, in-process client contract OSPF can reuse exactly as BGP does | `internal/component/bfd/api/service.go`, `registry.go`; learned 560 "Service interface surface is frozen for this path" | OSPF needs new api methods, widening the BFD surface | `TestOSPFBFDSessionOpenedOnFull` against a fake `api.Service` (mirrors `peer_bfd_test.go`) | unvalidated |
| A-2 | `Table.NeighborDown(interfaceName, id)` is a safe, idempotent NSM down injection that produces the same downstream effects as an inactivity-timer expiry | `neighbor/table.go` `neighborDown` (kill-nbr, setStateLocked Down); `nsmAdapter.NeighborDown` already calls it | a BFD-driven down behaves differently from a timer down (e.g. skips LSA re-origination) | `TestOSPFBFDDownDriversNeighborDown` asserts the neighbour goes Down and an LSA re-origination fires | unvalidated |
| A-3 | The neighbour's reachable source address (its `Neighbor.Address netip.Addr`) is the correct single-hop BFD `Peer` address, and the interface's own address is the `Local` address | `neighbor.go` `Neighbor.Address`; `Snapshot.Address`; RFC 5881 §6 multi-access addressing uses on-subnet src/dst | the session binds to the wrong address pair and never comes Up against FRR | `ospf-bfd-frr` interop forms a BFD Up session; `TestBFDRequestForNeighbor` pins the address pair | unvalidated |
| A-4 | Only Full adjacencies need a BFD session; opening at Full (not at 2-Way / Exchange) is correct and matches FRR | RFC 2328 (Full = adjacency carrying flooding); guide 1542-1545; `setStateLocked` emits up only on `next == stateFull` | sessions churn during ExStart/Exchange, or a half-formed adjacency is not protected | `TestOSPFBFDOnlyAtFull` asserts no session before Full; `ospf-bfd-frr` interop | unvalidated |
| A-5 | A single-hop session (`Mode: SingleHop`, TTL 255) is always correct for OSPFv2 neighbours (they are directly connected) | RFC 5881 §1 single-hop = directly-connected; OSPF adjacencies are link-local | a virtual-link or sham-link neighbour (multi-hop) is mis-protected | OSPF here forms only normal/stub/NSSA adjacencies (no virtual links in scope); documented in Known Limitations | unvalidated |
| A-6 | The BFD engine's slow-start floor (DesiredMinTx >= 1 s until Up) means a fresh adjacency may take up to `multiplier * 1 s` to first detect, and this is acceptable / expected | RFC 5880 §6.8.3; `rfc/short/rfc5880.md` "Slow-start floor" pitfall | operators expect sub-second detection on a brand-new adjacency and file a bug | documented in Known Limitations + RFC comment; `ospf-bfd-frr` measures detect time AFTER the session is Up | unvalidated |
| A-7 | A reload that flips `bfd.enabled` on an interface can open/close sessions for already-Full neighbours without re-forming the adjacency | `interfaceParamsEqual` / reload path in `instance.go`; BGP did the equivalent on its FSM | enabling BFD mid-adjacency requires a neighbour bounce (operator-visible churn) | `TestOSPFBFDReloadEnablesSessionForFullNeighbor` | unvalidated |
| A-8 | `EnsureSession` refcounting on the `Key` tuple means two OSPF neighbours, or an OSPF + BGP session to the same peer, coexist correctly | `api/events.go` `Key`; `bfd.go` `pluginService.EnsureSession` refcount | one neighbour's `ReleaseSession` tears down another's session | `TestOSPFBFDDistinctKeysPerNeighbor`; coexistence covered by the engine's existing refcount tests | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A BFD Down races a concurrent NSM timer-down for the same neighbour, double-firing `NeighborDown` | duplicate `kill-nbr` metric increments; a panic on a freed neighbour | `Table.NeighborDown` is idempotent (looks up; no-op if absent / already Down); the subscriber and the release path both tolerate "already gone"; `TestOSPFBFDDownIdempotentWithTimerDown` |
| R-2 | The session-open path needs the neighbour's raw `netip.Addr`, but `Snapshot.Address` is a string -- a parse round-trip could lose IPv6 zone / fail | a malformed `Peer` address; session never comes Up | carry the raw `netip.Addr` in a typed lifecycle callback (not via the string Snapshot); `TestBFDRequestForNeighbor` pins the exact `netip.Addr` |
| R-3 | A subscriber goroutine leaks if `stopBFD` does not wait the done chan, or if `ReleaseSession` closes the channel without the subscriber noticing | goroutine count grows per adjacency flap; `go test -race` / leak check fails | copy the `peer_bfd.go` stop+done handshake verbatim; `TestOSPFBFDSubscriberExitsOnRelease`; run the OSPF suite under `-race` |
| R-4 | Enabling BFD mid-flight floods the BFD engine with sessions on a large broadcast LAN (one per Full neighbour), exhausting discriminators / sockets | session-count gauge spikes; engine logs allocation pressure | one session per Full adjacency is inherent and bounded by neighbour count; the engine shares one UDP loop per (vrf, single-hop); `ze_ospf_bfd_sessions` gauge makes the count observable |
| R-5 | OSPF and BGP both open a single-hop session to the same neighbour with different timers; the engine picks the more aggressive value, surprising one client | a BFD session runs faster/slower than one client configured | documented engine behaviour (api doc: "engine picks the more aggressive value"); `ze_ospf_bfd_sessions` + the BFD `show` reflect the actual negotiated rate; acceptable per RFC 5882 refcounting |
| R-6 | A BFD plugin shutdown (Service set to nil) while OSPF sessions are live leaves dangling handles | `ReleaseSession` after plugin teardown; subscriber sees a closed channel | `SetService(nil)` runs before `stopAll` (learned 560 gotcha); the subscriber exits on channel close; `ReleaseSession` on a torn-down loop is a documented no-op |
| R-7 | The `bfd` config container on the OSPF interface collides with / shadows the top-level `bfd { }` plugin config or BGP's `bfd` container | parse error or wrong handler claims the section | the OSPF `bfd` leaf lives strictly under `ospf interface`; YANG namespacing keeps it distinct; functional test `ospf-bfd-config.ci` proves both coexist |
| R-8 | Min-tx/min-rx of 0 (or absurdly small) is accepted and produces an unusable session | session never stabilises; CPU spin | YANG `range` validation on the timer leaves (e.g. 10..255000 us echoing BFD's own bounds); `parseInterface` rejects 0; boundary tests |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ospf interface eth0 { bfd { enabled true } }` config | -> | `parseInterface` sets `interfaceConfig.BFD.Enabled`; flows to `neighbor.InterfaceConfig` | `TestParseInterfaceBFD` (unit) + `test/ospf/ospf-bfd-config.ci` |
| A neighbour reaches Full on a BFD-enabled interface | -> | `neighborEventSink.NeighborUp` -> BFD lifecycle -> `api.Service.EnsureSession` + `Subscribe` + subscriber spawned | `TestOSPFBFDSessionOpenedOnFull` (unit, fake Service) + `test/ospf/ospf-bfd-session.ci` |
| BFD reports `StateDown` for a protected neighbour | -> | subscriber -> `Table.NeighborDown(interface, routerID)` -> neighbour to Down | `TestOSPFBFDDownDriversNeighborDown` (unit) + `ospf-bfd-frr` interop |
| The neighbour leaves Full (timer, interface down, reset) | -> | `neighborEventSink.NeighborDown` -> BFD lifecycle release (`Unsubscribe` + `ReleaseSession`) | `TestOSPFBFDSessionReleasedOnDown` (unit) |
| BFD plugin not loaded (`GetService()==nil`) on a BFD-enabled interface | -> | lifecycle logs a warning once and runs without BFD | `TestOSPFBFDGracefulWhenPluginAbsent` (unit) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ospf interface eth0 { bfd { enabled true; min-tx 50; min-rx 50; multiplier 3 } }` | parsed into `interfaceConfig.BFD{Enabled:true, MinTxUs:50000, MinRxUs:50000, Multiplier:3}`; surfaced in `show ip ospf interface` as BFD enabled |
| AC-2 | A neighbour on a BFD-enabled interface transitions to Full, BFD plugin loaded | exactly one single-hop `api.SessionRequest` (Mode SingleHop, Peer = neighbour addr, Local = interface addr, Interface = ifname, timers from config) is sent to `EnsureSession`; a subscriber goroutine is running |
| AC-3 | A neighbour on an interface WITHOUT BFD enabled reaches Full | no `EnsureSession` call; OSPF runs on timers alone |
| AC-4 | BFD plugin not loaded (`GetService()==nil`), interface BFD-enabled, neighbour reaches Full | no session opened; a single warning logged; `ze_ospf_bfd_register_failures_total` incremented; OSPF unaffected |
| AC-5 | A protected session reports `StateDown` (Diag 1, detect-expired) | `Table.NeighborDown(interface, routerID)` is invoked; the neighbour drops to Down; `ze_ospf_bfd_session_down_total` incremented; self-LSAs re-originated and SPF re-runs |
| AC-6 | A protected session reports `StateAdminDown` | treated identically to Down (neighbour declared down) |
| AC-7 | A protected session reports `StateUp` / `StateInit` | logged at debug; OSPF adjacency state unchanged (BFD is a failure detector, not a session driver) |
| AC-8 | A protected neighbour leaves Full for any reason (inactivity timer, interface down, `clear ip ospf neighbor`) | the BFD session is released (`Unsubscribe` + `ReleaseSession`); the subscriber goroutine exits; `ze_ospf_bfd_sessions` decrements |
| AC-9 | A BFD Down and an inactivity-timer Down race for the same neighbour | the neighbour drops exactly once; no panic; idempotent (R-1) |
| AC-10 | A config reload sets `bfd.enabled false` on an interface with Full neighbours | every BFD session on that interface is released; the adjacencies stay Full (BFD removal does not bounce the neighbour) |
| AC-11 | A config reload sets `bfd.enabled true` on an interface with already-Full neighbours | a session is opened for each already-Full neighbour without re-forming the adjacency |
| AC-12 | Two distinct neighbours on the same BFD-enabled interface both reach Full | two distinct sessions (distinct `Key` tuples); releasing one does not affect the other |
| AC-13 | `min-tx 0` or `multiplier 0` in config | rejected at parse/validation time with a clear error (YANG `range` + `parseInterface` guard) |
| AC-14 | `show ip ospf neighbor` for a BFD-protected, Up neighbour | shows the BFD session state (Up); a down/absent session is distinguishable |
| AC-15 | The OSPF plugin is removed from the build | no `ze_ospf_bfd_*` metrics, no OSPF BFD code; the BFD engine and BGP-BFD client are unaffected (self-containment) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables BFD on an OSPF interface and forms an adjacency with FRR | config -> `parseInterface` -> Full -> `EnsureSession` (single-hop) -> BFD handshake with FRR's ospfd -> session Up | `test/ospf/ospf-bfd-session.ci` + `ospf-bfd-frr` interop |
| 2 | Pulls the link / kills FRR; OSPF detects the loss in the BFD detection window, not after RouterDeadInterval | BFD detect-timer expiry -> `StateDown` -> subscriber -> `Table.NeighborDown` -> neighbour Down -> SPF re-run -> route withdrawal, all well under 40 s | `ospf-bfd-frr` interop measures detect time < RouterDeadInterval |
| 3 | Runs `show ip ospf neighbor` and sees which adjacencies are BFD-protected and the session state | snapshot -> neighbour rows annotated with BFD state from the session map | `test/ospf/ospf-bfd-show.ci` |
| 4 | Disables BFD on a live link without dropping OSPF | reload `bfd { enabled false }` -> sessions released -> adjacencies stay Full | `test/ospf/ospf-bfd-disable.ci` + `TestOSPFBFDReloadDisableKeepsAdjacency` |
| 5 | Runs OSPF on a box where the BFD plugin was never loaded | `GetService()==nil` -> warning -> OSPF on timers; no crash, no blocked adjacency | `TestOSPFBFDGracefulWhenPluginAbsent` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseInterfaceBFD` | `internal/plugins/ospf/config_test.go` | AC-1, AC-13: parse `bfd` container; defaults; reject 0 timers/multiplier | |
| `TestBFDRequestForNeighbor` | `internal/plugins/ospf/bfd_client_test.go` | AC-2, A-3, R-2: SessionRequest address pair + Mode SingleHop + timers from config | |
| `TestOSPFBFDSessionOpenedOnFull` | `internal/plugins/ospf/bfd_client_test.go` | AC-2, A-1: one EnsureSession + subscriber on Full (fake `api.Service`) | |
| `TestOSPFBFDNotOpenedWhenDisabled` | `internal/plugins/ospf/bfd_client_test.go` | AC-3: no EnsureSession when interface BFD disabled | |
| `TestOSPFBFDOnlyAtFull` | `internal/plugins/ospf/bfd_client_test.go` | AC-2, A-4: no session before Full (Init/Exchange) | |
| `TestOSPFBFDGracefulWhenPluginAbsent` | `internal/plugins/ospf/bfd_client_test.go` | AC-4: nil Service -> warning + failure metric, no session | |
| `TestOSPFBFDDownDriversNeighborDown` | `internal/plugins/ospf/bfd_client_test.go` | AC-5, A-2: StateDown -> `Table.NeighborDown` -> neighbour Down + LSA re-origination | |
| `TestOSPFBFDAdminDownTreatedAsDown` | `internal/plugins/ospf/bfd_client_test.go` | AC-6: StateAdminDown -> neighbour Down | |
| `TestOSPFBFDUpInitNoTeardown` | `internal/plugins/ospf/bfd_client_test.go` | AC-7: Up/Init logged, no NSM change | |
| `TestOSPFBFDSessionReleasedOnDown` | `internal/plugins/ospf/bfd_client_test.go` | AC-8: leaving Full -> Unsubscribe + ReleaseSession; subscriber exits | |
| `TestOSPFBFDSubscriberExitsOnRelease` | `internal/plugins/ospf/bfd_client_test.go` | R-3: subscriber goroutine exits on stop and on channel close (no leak) | |
| `TestOSPFBFDDownIdempotentWithTimerDown` | `internal/plugins/ospf/bfd_client_test.go` | AC-9, R-1: BFD down + timer down race drops the neighbour once, no panic | |
| `TestOSPFBFDReloadDisableKeepsAdjacency` | `internal/plugins/ospf/bfd_client_test.go` | AC-10: reload disable releases sessions, adjacency stays Full | |
| `TestOSPFBFDReloadEnablesSessionForFullNeighbor` | `internal/plugins/ospf/bfd_client_test.go` | AC-11, A-7: reload enable opens sessions for already-Full neighbours | |
| `TestOSPFBFDDistinctKeysPerNeighbor` | `internal/plugins/ospf/bfd_client_test.go` | AC-12, A-8: two neighbours -> two distinct session Keys; independent release | |
| `TestOSPFBFDMetrics` | `internal/plugins/ospf/metrics_test.go` | sessions gauge / down counter / register-failure counter move correctly | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `min-tx` (us) | 10..255000 | 255000 | 0 (rejected) | 255001 (rejected) |
| `min-rx` (us) | 10..255000 | 255000 | 0 (rejected) | 255001 (rejected) |
| `multiplier` | 1..255 | 255 | 0 (rejected) | 256 (rejected, uint8 range) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-bfd-config` | `test/ospf/ospf-bfd-config.ci` | `bfd { enabled true; min-tx ...; multiplier ... }` parses; coexists with top-level `bfd` and BGP `bfd` | |
| `ospf-bfd-session` | `test/ospf/ospf-bfd-session.ci` | a Full adjacency opens a BFD session; `show ip ospf neighbor` shows it protected | |
| `ospf-bfd-show` | `test/ospf/ospf-bfd-show.ci` | `show ip ospf interface` / `neighbor` render BFD enabled + session state | |
| `ospf-bfd-disable` | `test/ospf/ospf-bfd-disable.ci` | reload disabling BFD releases sessions without dropping the adjacency | |

### Interop Tests (MANDATORY for protocol features)
<!-- REQUIRED when the spec adds/changes wire protocol behavior. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-bfd-frr` | `test/interop/scenarios/ospf-bfd-frr/` | FRR `ospfd` + FRR `bfdd` (`ip ospf bfd` on the link) | Ze and FRR form a Full OSPF adjacency AND a BFD Up session; pulling the link drives an OSPF neighbour-down in the BFD detection window (well under RouterDeadInterval); re-adding the link re-forms both | |

> Interop is required: this exercises real BFD wire behaviour (UDP 3784, TTL 255,
> three-way handshake) against an independent implementation. The single-hop raw
> path is Linux-only and runs as a QEMU integration test
> (`ai/rules/qemu-testing.md`), consistent with the BFD and OSPF interop sets.
> This scenario closes the `spec-bfd-3b-frr-interop` style gap for OSPF: it is a
> true failover test, not a wiring smoke test.

### Future (if deferring any tests)
- None. Every AC is covered by a unit, functional, or interop test above.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `internal/plugins/ospf/config.go` -- `interfaceConfig.BFD bfdInterfaceConfig` field; `bfdInterfaceConfig` struct (Enabled, MinTxUs, MinRxUs, Multiplier); parse the nested `bfd` map in `parseInterface`; validate timers/multiplier
- `internal/plugins/ospf/iface/iface.go` -- `Config.BFD` fields (carry enable + timers to the runtime layer)
- `internal/plugins/ospf/neighbor/neighbor.go` / `table.go` -- carry the per-interface BFD config into `InterfaceConfig`; ensure the up/down lifecycle can obtain the neighbour's raw `netip.Addr` + interface (typed lifecycle callback or accessor); `NeighborDown` consumed unchanged
- `internal/plugins/ospf/instance.go` -- the BFD lifecycle observer: open on Full, release on down; `neighborInterfaceConfig` maps the BFD fields; `interfaceRuntimeConfigLocked` carries them; the per-neighbour `bfdClient` map lives on the engine
- `internal/plugins/ospf/register.go` -- register `ze_ospf_bfd_sessions`, `ze_ospf_bfd_session_down_total`, `ze_ospf_bfd_register_failures_total`; doctor informational check (BFD configured but plugin absent)
- `internal/plugins/ospf/cmd_show.go` / `show_summary.go` -- annotate `show ip ospf neighbor` / `show ip ospf interface` rows with BFD enabled + session state
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- a `container bfd` under BOTH `list interface` instances (line ~166 IPv4, line ~292 the v6 address-family) with `enabled` (boolean), `min-tx`/`min-rx` (uint32 us, range), `multiplier` (uint8, range)
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- (only if a new show sub-leaf is needed; prefer annotating existing neighbor/interface output)
- `internal/plugins/ospf/doctor.go` -- informational doctor check: BFD enabled on an interface but `api.GetService()` nil
- `internal/plugins/ospf/v3/*` / `afstrategy_v6.go` -- (NO behaviour change this spec; the config plumbing is laid so a sibling OSPFv3-BFD spec can reuse it -- note only, do not open v6 sessions)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `bfd` container on `interface`; read `ai/rules/config-surface.md` (operational config, not env var) + `ai/rules/config-naming.md` (kebab-case leaves) |
| YANG validation constraints | [ ] yes | `enabled` boolean; `min-tx`/`min-rx` `uint32 { range "10..255000"; }`; `multiplier` `uint8 { range "1..255"; }`; `units microseconds` on the timers |
| YANG custom validators | [ ] no | native `range` + `boolean` suffice |
| CLI commands/flags | [ ] yes | annotate `show ip ospf interface` / `show ip ospf neighbor` with a BFD column; no new top-level verb |
| CLI grammar (action before identifier) | [ ] n/a | no new verb added |
| Editor autocomplete | [ ] yes | automatic for the YANG boolean/uint leaves under `bfd` |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-bfd-*.ci` |
| Pipe completeness | [ ] yes | the annotated show output already routes through `ApplyPipes` like the rest of OSPF show |
| Env var registration | [ ] no | per-interface operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] yes | informational only: BFD enabled but plugin absent (no new socket/port/binary -- the BFD engine owns those); register a diagnostic code in `internal/core/diagnostic/codes.go` per `ai/rules/doctor-checks.md` |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospf_bfd_sessions` | gauge | `interface`, `area` |
| `ze_ospf_bfd_session_down_total` | counter | `interface` |
| `ze_ospf_bfd_register_failures_total` | counter | `interface`, `reason` (plugin-absent / ensure-error) |

> These extend the umbrella's canonical OSPF metric set; they use the
> `ze_ospf_bfd_*` prefix and are registered by this spec's owner code. The
> umbrella "Metrics" table must gain these rows when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- BFD for OSPFv2 |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` + the OSPF guide -- the per-interface `bfd` block |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- BFD column in `show ip ospf neighbor`/`interface` |
| 4 | API/RPC added/changed? | [ ] no | reuses the frozen `internal/component/bfd/api` surface; no new RPC |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains a BFD client |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` + `docs/guide/bfd.md` -- OSPF opt-in section |
| 7 | Wire format changed? | [ ] no | OSPF wire unchanged; BFD wire is the delivered engine's |
| 8 | Plugin SDK/protocol changed? | [ ] no | uses the existing `api.Service` lookup |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc5880.md` + `rfc/short/rfc5881.md` -- flip the OSPF-client-relevant checklist context (client model, single-hop, GTSM consumed) |
| 10 | Test infrastructure changed? | [ ] yes (interop scenario added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPF BFD parity with FRR |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- NSM <-> BFD lifecycle |
| 13 | Route metadata keys added/changed? | [ ] no | BFD does not install routes |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the three `ze_ospf_bfd_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` + the umbrella metrics table |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed OSPF files |
| 17 | Existing docs show examples for this area? | [ ] check | verify OSPF interface config examples against the new `bfd` block; verify `docs/guide/bfd.md` OSPF section |

## Files to Create
- `internal/plugins/ospf/bfd_client.go` -- the OSPF BFD client: `bfdClient` struct, `startBFDSession`/`stopBFDSession`/`runBFDSubscriber`/`bfdRequestForNeighbor`, the per-neighbour session map and its mutex (near-copy of `peer_bfd.go`, OSPF-typed)
- `internal/plugins/ospf/bfd_client_test.go` -- the unit suite above, driven by a `fakeBFDService` fake (mirrors `peer_bfd_test.go`)
- `test/ospf/ospf-bfd-config.ci`, `test/ospf/ospf-bfd-session.ci`, `test/ospf/ospf-bfd-show.ci`, `test/ospf/ospf-bfd-disable.ci`
- `test/interop/scenarios/ospf-bfd-frr/` -- `ze.conf`, `frr.conf` (ospfd + bfdd, `ip ospf bfd`), `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm `api.Service`, `Table.NeighborDown`, `neighborEventSink` exist as described |
| 3. Wiring phase | Wiring Test table -- the BFD client skeleton + failing wiring tests |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist |
| 8. Fix issues | from critical review |
| 9. Re-verify | re-run stage 6 |
| 10. Repeat 7-9 | until clean |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13. Re-verify | re-run stage 6 |
| 14. Present summary | Executive Summary per `ai/rules/planning.md` |

### Implementation Phases

<!-- Phase 1 is ALWAYS wiring: create the entry point and a failing wiring test. -->

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- the BFD client skeleton hooked into the NSM lifecycle
   - Tests: `TestOSPFBFDSessionOpenedOnFull` (fake Service), `TestOSPFBFDGracefulWhenPluginAbsent`, `test/ospf/ospf-bfd-session.ci`
   - Files: `bfd_client.go` (`startBFDSession`/`stopBFDSession` stubs, per-neighbour map), `instance.go` (call them from `neighborEventSink.NeighborUp/NeighborDown`), a `fakeBFDService` test fake
   - Verify: a Full transition reaches `EnsureSession` (or degrades gracefully on nil); deeper behaviour still stubbed so down/release tests fail
2. **Phase: Config surface** -- the per-interface `bfd` block
   - Tests: `TestParseInterfaceBFD`, boundary tests, `test/ospf/ospf-bfd-config.ci`
   - Files: `yang/ze-ospf-conf.yang` (the `bfd` container on both interface lists), `config.go` (`bfdInterfaceConfig` + parse + validation), `iface/iface.go` (`Config.BFD`), `instance.go` (`neighborInterfaceConfig` / `interfaceRuntimeConfigLocked` carry the fields), `neighbor` (`InterfaceConfig.BFD`)
   - Verify: config parses; 0 timers/multiplier rejected; the engine sees per-interface BFD config
3. **Phase: Session request + open on Full** -- build the single-hop request from the neighbour + config
   - Tests: `TestBFDRequestForNeighbor`, `TestOSPFBFDNotOpenedWhenDisabled`, `TestOSPFBFDOnlyAtFull`, `TestOSPFBFDDistinctKeysPerNeighbor`
   - Files: `bfd_client.go` (`bfdRequestForNeighbor`, the open path, the typed lifecycle callback carrying `netip.Addr`)
   - Verify: correct address pair + Mode SingleHop + timers; only Full, only when enabled; distinct keys per neighbour
4. **Phase: Down injection + subscriber** -- BFD Down -> NSM down
   - Tests: `TestOSPFBFDDownDriversNeighborDown`, `TestOSPFBFDAdminDownTreatedAsDown`, `TestOSPFBFDUpInitNoTeardown`, `TestOSPFBFDSubscriberExitsOnRelease`, `TestOSPFBFDDownIdempotentWithTimerDown`, `ospf-bfd-frr` interop
   - Files: `bfd_client.go` (`runBFDSubscriber` -> `Table.NeighborDown`), the stop+done handshake
   - Verify: Down/AdminDown drop the neighbour through the existing NSM; Up/Init are inert; the subscriber never leaks; race with timer-down is idempotent
5. **Phase: Release lifecycle + reload** -- release on leaving Full, on interface down, on config disable/enable
   - Tests: `TestOSPFBFDSessionReleasedOnDown`, `TestOSPFBFDReloadDisableKeepsAdjacency`, `TestOSPFBFDReloadEnablesSessionForFullNeighbor`, `ospf-bfd-disable.ci`
   - Files: `bfd_client.go` (`stopBFDSession`), `instance.go` (release on `NeighborDown`/`InterfaceDown`/reload diff)
   - Verify: sessions release cleanly; disabling BFD does not bounce the adjacency; enabling opens sessions for already-Full neighbours
6. **Phase: CLI + metrics + doctor** -- operator surface
   - Tests: `TestOSPFBFDMetrics`, `ospf-bfd-show.ci`, `TestOSPFBFDShowNeighbor`
   - Files: `register.go` (metrics), `cmd_show.go`/`show_summary.go` (BFD annotation), `doctor.go` + `internal/core/diagnostic/codes.go` (informational check)
   - Verify: `show ip ospf neighbor`/`interface` show BFD state; the three metric series move; the doctor check fires when BFD is configured-but-absent
7. **Functional tests** -> the four `.ci` cover the user-visible behaviour
8. **RFC refs** -> add `// RFC 5881 Section X` / `// RFC 5880 Section X` comments on the single-hop request, the GTSM rationale, the Down-handling, and the slow-start note
9. **Interop** -> `ospf-bfd-frr` QEMU scenario (true failover test)
10. **Full verification** -> `make ze-verify`
11. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; parity with FRR's `ip ospf bfd` (session on adjacency, immediate down on BFD loss) |
| Correctness | session opened only at Full; only Down/AdminDown drives `NeighborDown`; address pair correct (peer = neighbour addr, local = interface addr); Mode SingleHop always; idempotent down |
| Naming | `ze_ospf_bfd_*` metrics; YANG `bfd`/`min-tx`/`min-rx`/`multiplier` kebab-case; `startBFDSession`/`stopBFDSession` |
| Data flow | BFD down flows through `Table.NeighborDown` + existing NSM; OSPF imports only `bfd/api`; no OSPF symbol in `internal/component/bfd` |
| CLI grammar | no new verb; show annotation only |
| Doctor checks | informational BFD-configured-but-absent check registered per `ai/rules/doctor-checks.md` |
| YANG validation | `bfd` leaves have `range`/`boolean`; 0 timers/multiplier rejected; bare `type string` absent |
| Prometheus counters | the three series defined, registered, listed; umbrella table updated |
| Rule: plugin-self-containment | removing OSPF removes all its BFD wiring; BFD engine + BGP client untouched |
| Rule: goroutine-lifecycle | the subscriber is one per-session worker with stop+done handshake; no leak under `-race` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| OSPF opens a single-hop BFD session on Full | `go test ./internal/plugins/ospf -run TestOSPFBFDSessionOpenedOnFull` |
| BFD Down drives `Table.NeighborDown` | `go test ./internal/plugins/ospf -run TestOSPFBFDDownDriversNeighborDown` |
| Per-interface `bfd` config parses + validates | `go test ./internal/plugins/ospf -run TestParseInterfaceBFD` |
| Graceful degradation when plugin absent | `go test ./internal/plugins/ospf -run TestOSPFBFDGracefulWhenPluginAbsent` |
| Three metric series registered | `grep -rn 'ze_ospf_bfd_' internal/plugins/ospf` |
| Interop scenario present | `ls test/interop/scenarios/ospf-bfd-frr/` |
| Functional tests present | `ls test/ospf/ospf-bfd-*.ci` |
| Only `bfd/api` imported from OSPF | `grep -rn 'internal/component/bfd' internal/plugins/ospf` shows only `/api` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | `bfd` timer/multiplier leaves range-checked; the neighbour address fed to `SessionRequest` is a validated `netip.Addr`, never an unparsed string |
| Resource exhaustion | one session per Full adjacency (bounded by neighbour count); the engine shares one UDP loop per (vrf, single-hop); `ze_ospf_bfd_sessions` gauge observable |
| Subscriber isolation | a panicking subscriber cannot wedge the NSM lock; `Table.NeighborDown` takes its own lock and the subscriber calls it outside any OSPF lock |
| Trust boundary | BFD single-hop relies on GTSM (TTL 255) enforced by the engine; OSPF adds no new listening port or socket -- the BFD engine owns the wire |
| Error leakage | `EnsureSession`/`ReleaseSession` errors are logged + counted, never surfaced to a peer or the wire |
| DoS via flap | a flapping adjacency opens/closes sessions; the release path is idempotent and bounded; no unbounded goroutine growth (R-3) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to the relevant phase and implement |
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
<!-- LIVE -- write IMMEDIATELY when you learn something -->

## Core Insight
BFD-for-OSPF is a wiring problem, not a protocol problem: the BFD engine and its
in-process `api.Service` client contract are delivered, and the down-injection
seam (`Table.NeighborDown`) already exists. The spec connects two ends that are
both already in tree -- NSM Full -> `EnsureSession`, BFD Down -> `NeighborDown` --
and the BGP-BFD client (`peer_bfd.go`) is a complete template for the lifecycle,
nil-safety, and subscriber discipline. The only genuinely new surface is the
per-interface `bfd` config block.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Drive `Table.NeighborDown` on BFD Down | a dedicated BFD-down NSM event; a direct state poke | the existing seam already produces every correct downstream effect (kill-nbr, LSA re-origination, SPF); reuse keeps a BFD-down indistinguishable from a timer-down |
| Open the session only at Full | open at 2-Way / Exchange | Full is the adjacency actually carrying flooding; matches FRR; avoids churn during ExStart/Exchange |
| Single-hop only (`Mode: SingleHop`) | configurable hop mode | OSPFv2 neighbours are directly connected by definition; multi-hop is for virtual/sham links (out of scope) |
| Per-interface `bfd` config block | a global OSPF BFD toggle; per-area | BFD is a link property; FRR's `ip ospf bfd` is per-interface; matches the OSPF config grain |
| Reuse the frozen `api.Service` lookup | a new OSPF-specific BFD API | learned 560 froze this surface for exactly this case; adding methods would widen the BFD boundary unnecessarily |
| Copy `peer_bfd.go` structure verbatim | a fresh design | proven nil-safe, leak-free, mutex-guarded lifecycle; divergence would re-introduce solved bugs |

## Known Limitations
- Virtual links / sham links (multi-hop OSPF adjacencies) are not BFD-protected by this spec (single-hop only).
- OSPFv3 (the IPv6 family) is not wired here; the config plumbing is laid so a sibling spec can mirror it against `v3/`/`afstrategy_v6.go`.
- A freshly-formed adjacency detects in up to `multiplier * 1 s` until the session reaches Up and fast rates negotiate (RFC 5880 §6.8.3 slow-start); configured `min-tx`/`min-rx` apply only after Up.
- BFD authentication and echo are engine concerns; OSPF sessions run unauthenticated, async, no-echo in this spec.

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code:
- RFC 5881 §3 both ends Active -> `bfdRequestForNeighbor` leaves `Passive` false
- RFC 5881 §4 / §5 single-hop bound to one interface, TTL 255 GTSM -> `Mode: SingleHop` + the interface name in the request (engine enforces TTL)
- RFC 5880 §6.8.1 Down with Diag 1/3 -> the subscriber's `StateDown`/`StateAdminDown` handling driving `NeighborDown`
- RFC 5880 §6.8.3 slow-start floor -> the comment on the timer fields explaining why fresh detection is up to `multiplier * 1 s`
- RFC 5882 client model "failure detector, not a session driver" -> the comment on the Up/Init no-op branch

## Implementation Summary

### What Was Implemented
- [filled at implementation time]

### Bugs Found/Fixed
- [filled at implementation time]

### Documentation Updates
- [filled at implementation time]

### Deviations from Plan
- [filled at implementation time]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. -->

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

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| BFD session opened on a usable OSPF adjacency | unit + interop | `TestOSPFBFDSessionOpenedOnFull`, `ospf-bfd-frr` |
| BFD Down declares the neighbour down faster than RouterDeadInterval | interop | `ospf-bfd-frr` measures detect time < RouterDeadInterval |
| Per-interface enable + BFD timer config | unit + functional | `TestParseInterfaceBFD`, `ospf-bfd-config.ci` |
| Down-event path through the existing NSM | unit | `TestOSPFBFDDownDriversNeighborDown` |
| Graceful degradation without the BFD plugin | unit | `TestOSPFBFDGracefulWhenPluginAbsent` |
| Mirrors the BGP-BFD client wiring | review | structural diff against `peer_bfd.go`; self-containment grep |

## Review Gate

<!-- BLOCKING: Run /ze-review BEFORE the final testing/verify step. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE |  | file:line |  |

### Fixes applied
-

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ospf/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass)
- [ ] RFC 5880 / RFC 5881 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (the client is a concrete copy of `peer_bfd.go`, justified by the proven pattern)
- [ ] No speculative features (no echo, no multi-hop, no OSPFv3 session)
- [ ] Single responsibility per component (the BFD client only bridges NSM <-> session)
- [ ] Explicit > implicit behavior (opt-in per interface; nil-safe degradation)
- [ ] Minimal coupling (OSPF imports only `bfd/api`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-bfd-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-10-bfd.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-10-bfd.md`
