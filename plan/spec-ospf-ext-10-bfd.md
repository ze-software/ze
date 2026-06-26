# Spec: ospf-ext-10 -- BFD for OSPF (RFC 5880, RFC 5881)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospf-ext-0-umbrella.md |
| Phase | - |
| Updated | 2026-06-24 |

> **Scope.** ONE feature spec covering BFD for BOTH OSPF address families on the
> single unified `ospf` engine (`internal/plugins/ospf/`). OSPF in Ze is one engine
> spanning address families exactly as `bgp` is one engine spanning AFI/SAFI: there
> is NO separate `ospfv3` plugin. The IPv4 family is OSPFv2 (RFC 2328); the IPv6
> family is OSPFv3 (RFC 5340) running as a second instance of the same engine over
> the v6 codec (`codec.IsV6()` true). The FSM, flooding, DR election, SPF, LSDB
> sequencing, the neighbour table, and the BFD client LIFECYCLE are AF-neutral and
> SHARED; only the per-family BFD request builder (IPv4 interface address vs IPv6
> link-local source) and the family gate differ. See `plan/learned/972-ospf-af-unify.md`.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/learned/972-ospf-af-unify.md` -- OSPF is ONE engine; IPv4/IPv6 are address families, not separate plugins; the AF-specific code lives in `afstrategy_v6.go`, `codec_v6.go`, `encoder_v6.go`, `origination_v6*.go`, and `v3/{types,packet,transport}`
4. `rfc/short/rfc5881.md` -- single-hop encapsulation: UDP 3784, both ends Active (§3), TTL/Hop-Limit = 255 GTSM (§5), one session per data protocol so IPv4 (OSPFv2) and IPv6 (OSPFv3) are SEPARATE sessions on the same link (§2), Your-Discriminator demux (§6), §6 multi-access on-subnet src/dst -- IPv4 = on-subnet pair, IPv6 = link-local pair
5. `rfc/short/rfc5880.md` -- base BFD: session states Down/Init/Up/AdminDown, Diag codes (1 = detect-expired, 3 = neighbor-signaled-down), slow-start floor (§6.8.3), the failure-detector contract; ze already implements the full engine (`internal/component/bfd`)
6. `internal/component/bgp/reactor/peer_bfd.go` -- the EXEMPLAR client: `bfdClient{mu,svc,handle,sub,stop,done}`, `startBFDClient`/`stopBFDClient`/`runBFDSubscriber`/`bfdRequestFor`; this spec mirrors it for OSPF (both families, one engine)
7. `internal/component/bfd/api/service.go` + `events.go` + `registry.go` -- `GetService()`, `Service.EnsureSession(SessionRequest)`, `SessionHandle.Subscribe()/Unsubscribe()`, `Service.ReleaseSession`; `SessionRequest{Peer,Local netip.Addr, Interface, VRF, Mode, DesiredMinTxInterval, RequiredMinRxInterval, DetectMult, ...}`; `StateChange{Key,State,Diag,When}`; `StateDown`/`StateAdminDown`; `SingleHop`; `Key{Peer,Local,Interface,VRF,Mode}` refcount tuple
8. `internal/plugins/ospf/neighbor/table.go` -- `Table.NeighborDown(interfaceName string, id types.RouterID)` is the AF-neutral NSM down-event injection seam (records `kill-nbr`, `setStateLocked(n, stateDown)`, emits the `down` `eventEmission`); `Neighbor.Address netip.Addr` is the neighbour's reachable source (IPv4 for v2, IPv6 link-local for v3); `AddressOf(id)` and `Lookup(iface,id)` return it
9. `internal/plugins/ospf/instance.go` -- the engine is UNIFIED v2/v3; `e.dispatch.codec.IsV6()` selects the IPv6 (OSPFv3) family; `neighborEventSink.NeighborUp/NeighborDown` is the Full<->non-Full chokepoint; `neighborInterfaceConfig` maps `iface.Config` -> `neighbor.InterfaceConfig`; `e.transport` is the v3 raw IPv6 transport when the engine is v6
10. `internal/plugins/ospf/v3/transport/backend_linux.go` -- `LinkLocalSource() netip.Addr` is the interface's IPv6 link-local source; this is the BFD session `Local` for the v6 family (`iface.Config.InterfaceAddress [4]byte` is IPv4-only and must NOT be used for the v6 `Local`)
11. `internal/plugins/ospf/config.go` -- `interfaceConfig` struct (config.go:141), `parseInterface` (config.go:633); the `ospfConfig` carries the `address-family ipv6` sub-config; `parseInterface` is shared by the IPv4 and IPv6-family interface lists, so the per-interface `bfd` block parses for BOTH families from one code path
12. `internal/plugins/ospf/register.go` -- `registerOSPF()` registers namespace, doctor, diagnostic codes, completions, config sections, the `ze_ospf_*` metric set; the new `ze_ospf_bfd_*` metrics register here ONCE (shared by both families, get-or-create)
13. `plan/learned/560-bfd-3-bgp-client.md` -- the BGP-BFD client design this lifecycle copies (atomic-pointer Service lookup, graceful degradation, per-session subscriber goroutine, stop+done handshake)
14. `plan/learned/564-bfd-2b-ipv6-transport.md` -- the BFD engine's IPv6 single-hop transport (`IPV6_UNICAST_HOPS=255`, `IPV6_RECVHOPLIMIT` cmsg GTSM check); the v6 socket path is already delivered, so an IPv6 single-hop request is carried end-to-end by the engine
15. `plan/learned/970-ospfv3-3-ipv6-transport.md` -- user directive: "OSPFv3 is our OSPF; REUSE the `ze_ospf_*` metric series, do NOT fork a `ze_ospfv3_*` namespace"; base OSPFv3 multicast uses Hop-Limit 1 (NOT GTSM 255) -- BFD's GTSM-255 single-hop session is a SEPARATE unicast session and does not conflict

## Task

Integrate OSPF neighbour liveness, on BOTH address families of the single unified
`ospf` engine, with Ze's existing, delivered BFD engine (`internal/component/bfd`).
When an OSPF adjacency reaches a usable state (Full, the point at which the
neighbour is actually carrying topology/flooding), the engine registers a
**single-hop asynchronous** BFD session for that neighbour (RFC 5881: UDP 3784,
both ends Active, TTL/Hop-Limit = 255 GTSM). The session request is built by an
AF-specific builder gated on `codec.IsV6()`:

| Address family | Engine gate | BFD `Peer` | BFD `Local` | RFC |
|----------------|-------------|------------|-------------|-----|
| IPv4 (OSPFv2) | `!IsV6()` | neighbour IPv4 (`Neighbor.Address`) | interface IPv4 address | RFC 5881 IPv4 single-hop |
| IPv6 (OSPFv3) | `IsV6()` | neighbour IPv6 link-local (`Neighbor.Address`) | interface IPv6 link-local (`v3 transport LinkLocalSource()`) | RFC 5881 IPv6 single-hop, learned 564 |

When BFD reports the session **Down** (or AdminDown), the engine injects the
AF-neutral NSM "neighbour down" event immediately
(`neighbor.Table.NeighborDown(interface, routerID)`), declaring the neighbour dead
far faster than the RouterDeadInterval timer (typically 40 s) would. The session is
released when the adjacency drops for any other reason, when BFD is disabled, or
when the interface goes down.

The work mirrors the BGP-BFD client (`peer_bfd.go`, learned 560) almost one-for-one,
substituting OSPF entry points for BGP's: the FSM-callback hook becomes the NSM
Up/Down transition, `Peer.Teardown(...)` becomes `Table.NeighborDown(iface, routerID)`,
and `PeerSettings.BFD` becomes a per-interface OSPF `bfd` config block. The SHARED
client (one `bfdClient` lifecycle plus a map keyed `(interface, RouterID)` per
engine-instance) serves both families; the family gate (`codec.IsV6()`) only selects
which request builder runs. BFD is strictly **additive**: if the BFD plugin is not
loaded (`api.GetService()` returns nil), or BFD is not enabled on the interface, OSPF
runs exactly as today on the Hello/Dead timers alone, on both families.

### In scope (this spec)

| Item | Detail | Address family |
|------|--------|----------------|
| BFD session lifecycle tied to NSM | Open a single-hop session when a neighbour reaches Full; release it when the neighbour leaves Full / interface goes down / config disables BFD. Hook off `neighborEventSink.NeighborUp` / `NeighborDown` in `instance.go` | shared (both) |
| Per-family session keying | The `Key` tuple (peer, local, interface, vrf, single-hop) differs by family because the address pair differs, so a v2 (IPv4) and a v3 (link-local) session on one physical link are distinct | IPv4: on-subnet pair / IPv6: link-local pair |
| AF-specific request builder | `bfdRequestForNeighbor` selects the v4 or v6 builder by `codec.IsV6()`; v4 uses the interface IPv4 address, v6 uses the interface IPv6 link-local source from the v3 transport | per-AF |
| Down-event injection path | A BFD `StateDown`/`StateAdminDown` change drives `Table.NeighborDown(interfaceName, routerID)` -- the SAME seam `nsmAdapter.NeighborDown` already uses -- so the neighbour drops to Down through the existing AF-neutral NSM, not a new code path | shared |
| Per-interface config surface | A `bfd` container on the OSPF `interface` list under BOTH the IPv4 tree and the `address-family ipv6` tree: `enabled`, `min-tx`, `min-rx`, `multiplier` leaves; resolved into `interfaceConfig` -> `iface.Config` -> `neighbor.InterfaceConfig` | shared parse path, both trees |
| In-process Service lookup | Reuse `api.GetService()` (already published by the BFD plugin's `OnStarted`); nil-safe graceful degradation identical to BGP | shared |
| Per-session subscriber goroutine | One long-lived worker per BFD-protected neighbour draining `<-chan StateChange`; translates Down -> `NeighborDown`; goroutine-lifecycle compliant, mirrors `runBFDSubscriber` (stop+done handshake) | shared |
| Metrics | `ze_ospf_bfd_sessions` gauge, `ze_ospf_bfd_session_down_total` counter, `ze_ospf_bfd_register_failures_total` counter (unified `ze_ospf_*` namespace per learned 970; the `interface` label distinguishes v2 from v3) | shared series |
| CLI surface | BFD enable + session state via the existing `show ospf neighbor`/`interface` (IPv4) and `show ospf ipv6 neighbor`/`interface` (IPv6) outputs (a `bfd` column / flag), no new top-level command | per-AF show verbs |

### Out of scope (noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| BFD echo function (RFC 5880 §6.4) | BFD engine concern; `spec-bfd-6-echo-mode` (delivered). OSPF does not request echo (`DesiredMinEchoTxInterval` left zero) |
| BFD multi-hop (RFC 5883) | engine concern; OSPF adjacencies are single-hop by definition (directly-connected neighbours). The request is always `Mode: SingleHop`. Virtual links / sham links (multi-hop) are not BFD-protected here |
| BFD authentication wiring | engine concern; `spec-bfd-5-authentication` (delivered). OSPF sessions inherit no auth in this spec (`SessionRequest.Auth` left nil) |
| BFD timer negotiation, Poll/Final, slow-start | all inside `internal/component/bfd` (delivered); OSPF only supplies desired min-tx / min-rx / multiplier and never touches the wire |
| BFD on an OSPF interface that never reaches Full (DR/BDR election only, 2-Way DROther pairs on a broadcast LAN) | by design only Full adjacencies (the ones carrying flooding) get a session; 2-Way-only neighbours do not -- same rule on both families |
| GTSM-255 vs base-OSPFv3 Hop-Limit-1 (IPv6 family) | none: base OSPFv3 multicast Hellos use Hop-Limit 1 (learned 970); BFD single-hop is a SEPARATE unicast session at Hop-Limit 255; the two coexist on the same link without interaction (engine owns the BFD wire) |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
- [ ] `plan/learned/972-ospf-af-unify.md` -- OSPF is ONE engine; IPv4 and IPv6 are address families
  -> Decision: implement BFD ONCE, on the shared engine; the lifecycle, config plumbing, client map, and metrics are AF-neutral; only the request builder forks by `codec.IsV6()`
  -> Constraint: there is NO separate OSPFv3 plugin directory; the v6 family lives entirely under `internal/plugins/ospf/` in `afstrategy_v6.go`, `codec_v6.go`, `encoder_v6.go`, `origination_v6*.go`, and `internal/plugins/ospf/v3/{types,packet,transport}`
- [ ] `docs/research/ospf-implementation-guide.md` lines 1542-1545 ("BFD Integration (RFC 5880, RFC 5881)") -- "Implemented as a thin wrapper around zebra's BFD subsystem. BFD failure triggers an NSM event that declares the neighbour down immediately."
  -> Decision: model BFD-for-OSPF as a thin client over the EXISTING `internal/component/bfd` engine, exactly as FRR's `ospfd`/`ospf6d` wrap zebra's BFD subsystem; no new BFD logic, only the NSM <-> session glue
  -> Constraint: the failure action is "an NSM event that declares the neighbour down immediately" -- drive `Table.NeighborDown`, never invent a parallel teardown
- [ ] `plan/learned/560-bfd-3-bgp-client.md` -- the BGP-BFD client this lifecycle mirrors
  -> Decision: hook the session lifecycle at the same architectural layer BGP used (the FSM/NSM transition callback), not a lower packet layer; `startBFDSession` after the state is set Full, `stopBFDSession` on leaving Full
  -> Constraint: graceful degradation is mandatory -- a missing BFD plugin (`GetService()==nil`) MUST NOT block OSPF; the adjacency runs on Hello/Dead timers alone and logs a warning once
  -> Constraint: the subscriber is a long-lived per-session worker (one per BFD-protected neighbour), not per-event; `stopBFDSession` closes a stop chan and waits a done chan so the goroutine has exited before the handle is released
- [ ] `plan/learned/564-bfd-2b-ipv6-transport.md` -- the BFD IPv6 single-hop transport (v6 family only)
  -> Constraint: the BFD engine already carries an IPv6 single-hop session end-to-end (`IPV6_UNICAST_HOPS=255` TX, `IPV6_RECVHOPLIMIT` cmsg GTSM RX). The v6 family supplies `netip.Addr` link-local peer/local; the engine selects the v6 socket by address family. OSPF adds NO transport code
- [ ] `plan/learned/970-ospfv3-3-ipv6-transport.md` -- OSPFv3 transport + the metric-naming directive
  -> Constraint: use the UNIFIED `ze_ospf_bfd_*` metric namespace (NOT `ze_ospfv3_bfd_*`); the registry is get-or-create by name, so both families share one series and the `interface` label distinguishes them
  -> Constraint: base OSPFv3 multicast is Hop-Limit 1, NOT GTSM 255; that base-transport rule does not apply to the BFD session, which is single-hop unicast at Hop-Limit 255 enforced by the BFD engine
- [ ] `ai/rules/plugin-self-containment.md` -- removing the OSPF plugin removes all its BFD wiring
  -> Constraint: all BFD-for-OSPF code lives under `internal/plugins/ospf`; the only outside dependency is `internal/component/bfd/api` (a leaf package). No OSPF spelling appears in `internal/component/bfd`
- [ ] `ai/rules/no-sprintf-alloc.md` -- no `fmt`/`+` on any hot path
  -> Constraint: the per-packet BFD path is entirely inside the BFD engine; OSPF's only frequency is per-NSM-transition (rare). Session-state log lines use structured slog fields, never `fmt.Sprintf` string building

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5881.md` -- BFD for IPv4/IPv6 single hop
  -> Constraint: §3 both ends MUST take the Active role; OSPF supplies `Passive: false` (the api default) so the session comes up symmetrically with FRR
  -> Constraint: §2 a separate BFD session MUST exist per data protocol over a link; the IPv4 family opens an IPv4 single-hop session, the IPv6 family opens an IPv6 single-hop session, and on a dual-stack link they are independent (distinct `Key` tuples)
  -> Constraint: §5 single-hop sessions transmit TTL/Hop-Limit = 255 and discard on receive != 255 (GTSM). Enforced inside `internal/component/bfd` (SingleHop mode, learned 564); OSPF only selects `Mode: SingleHop`
  -> Constraint: §6 multi-access addressing uses on-subnet src/dst. IPv4 family: the on-subnet IPv4 pair. IPv6 family: the link-local pair (`Peer` = neighbour link-local, `Local` = interface link-local). The session is bound to ONE egress interface (`SessionRequest.Interface`), matching the interface the Hello arrived on
- [ ] `rfc/short/rfc5880.md` -- base BFD protocol
  -> Constraint: §6.8.1 the session declares Down with Diag 1 (Control Detection Time Expired) on timer miss and Diag 3 (Neighbor Signaled Session Down) when the peer reports Down; OSPF treats BOTH `StateDown` and `StateAdminDown` as "neighbour down" regardless of Diag, on both families, matching the BGP client
  -> Constraint: §6.8.3 slow-start floors DesiredMinTxInterval at 1 s until the session reaches Up; OSPF's configured `min-tx` only takes effect after Up. A freshly-formed adjacency therefore detects in up to `multiplier * 1 s` until fast rates negotiate -- this is the engine's contract, not an OSPF bug
  -> Constraint: BFD is a "failure detector, not a session driver" (RFC 5882 client model): OSPF acts ONLY on Down. A BFD `Up`/`Init` transition is logged at debug and does NOT itself bring the OSPF adjacency up (the NSM owns that)

**Key insights:** (minimal context to resume after compaction)
- BFD-for-OSPF is glue on a unified v2/v3 engine, not a new protocol: the engine (incl. the IPv6 single-hop transport) is delivered; this spec wires NSM Full -> EnsureSession and BFD Down -> `Table.NeighborDown`.
- The down-injection seam already exists and is AF-neutral: `Table.NeighborDown(interface, id)` is called today by `nsmAdapter.NeighborDown`; the BFD subscriber calls the same `Table` method on both families.
- The exemplar is `peer_bfd.go`; copy its structure (start/stop/subscriber/request-builder, nil-safe degradation, mutex-guarded per-session state, stop+done handshake). ONE shared lifecycle; the ONLY per-AF divergence is the request builder (IPv4 interface address vs IPv6 link-local) selected by `codec.IsV6()`.
- Only Full adjacencies get a session; only Down/AdminDown drives a teardown. Use `ze_ospf_bfd_*` metrics on both families, never `ze_ospfv3_bfd_*`.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/plugins/ospf/neighbor/table.go` -- `Table` owns the per-interface neighbour map keyed by `(iface, RouterID)`; `NeighborDown(interfaceName, id)` looks up the neighbour, records `kill-nbr`, and `setStateLocked(n, stateDown)`; `setStateLocked` returns a `down` `eventEmission` when a Full neighbour drops, which `emit` forwards to `EventSink.NeighborDown`. `Neighbor.Address netip.Addr` is the reachable source (IPv4 for v2, IPv6 link-local for v3); `AddressOf(id)` and `Lookup(iface, id)` return it. `Snapshot.Address` is the STRING form
  -> Constraint: `NeighborDown` is the EXACT injection point on BOTH families; the BFD subscriber calls it with the neighbour's interface + Router ID. No new NSM event or state is needed. It is idempotent (no-op if absent or already Down)
  -> Constraint: the engine needs the raw `netip.Addr` (`Neighbor.Address`) for the BFD `Peer`, not the `Snapshot.Address` string; prefer a typed lifecycle callback carrying the raw address (or `Table.Lookup`/`AddressOf`) so an IPv6 link-local zone is never lost to a string round-trip (R-2)
- [ ] `internal/plugins/ospf/neighbor/neighbor.go` -- `Snapshot{Interface, Area, RouterID, State, Address(string), ...}`; `Neighbor.Address netip.Addr` (IPv4 for v2, IPv6 link-local for v3); `EventSink{NeighborUp(Snapshot), NeighborDown(Snapshot)}`; the `state` enum (`stateFull` is the usable state); `InterfaceConfig` (no BFD fields yet)
  -> Constraint: `EventSink` carries only a `Snapshot` (string address). To build a `SessionRequest` with a raw `netip.Addr` the lifecycle must obtain `Neighbor.Address` -- widen the event path with a typed callback, or look up via `Table.Lookup`/`AddressOf`. The spec prefers a typed lifecycle callback to avoid the string round-trip
  -> Constraint: `InterfaceConfig` gains a BFD sub-struct (enabled + timers) so the engine can decide whether to open a session for a neighbour on that interface; the field is AF-neutral
- [ ] `internal/plugins/ospf/instance.go` -- the engine is UNIFIED v2/v3 (`newEngineWithCodec`); `e.dispatch.codec.IsV6()` is true for the IPv6 (OSPFv3) family; `e.neighbors.SetEventSink(neighborEventSink{sink: e.sink, onChange: e.originateSelfLSAs})`; `neighborEventSink.NeighborUp/NeighborDown` is the single Full<->non-Full chokepoint; `neighborInterfaceConfig(cfg)` maps `iface.Config` -> `neighbor.InterfaceConfig`; `e.transport` is the v3 transport when the engine is v6
  -> Constraint: `neighborEventSink` is the AF-neutral lifecycle hook (open on up, close on down); it must learn the engine's family (`codec.IsV6()`), the neighbour's raw `netip.Addr`, the interface, and the per-interface BFD config to open/close sessions
  -> Constraint: the per-neighbour `bfdClient` map and the start/stop helpers are shared; only `bfdRequestForNeighbor` forks by family. The v4 path uses the interface IPv4 address; the v6 path uses the interface IPv6 link-local source from the v3 transport
- [ ] `internal/plugins/ospf/iface/iface.go` -- `Config` struct (iface.go:70: Name, AreaID, NetworkType, InterfaceAddress[4]byte, HelloInterval, DeadInterval, IsV6, InterfaceID, ...) is the per-interface runtime config the FSM/Hello layer consumes; `IsV6` marks an OSPFv3 interface
  -> Constraint: the BFD enable + timers must ride through `iface.Config` so the engine sees them when it opens a neighbour session; add the fields and map them in `interfaceRuntimeConfigLocked` / `neighborInterfaceConfig`. `InterfaceAddress` ([4]byte) is the IPv4 `Local` source; the IPv6 link-local `Local` is NOT stored here -- it comes from the live v3 transport (which tracks DAD)
- [ ] `internal/plugins/ospf/v3/transport/backend_linux.go` -- `interfaceLinkLocal(name)` resolves the interface's IPv6 link-local source (DAD-complete preferred, else `ErrNoLinkLocal`); `linuxInterface.LinkLocalSource() netip.Addr` (backend_linux.go:178) exposes it; the RX/TX path binds `ControlMessage.Src` to this link-local
  -> Constraint: `LinkLocalSource()` is the BFD session `Local` for the v6 family. `iface.Config.InterfaceAddress` is IPv4-only and MUST NOT be used for the v6 `Local`. The open path tolerates a zero `Local` (kernel source selection) as a fallback
  -> Constraint: the link-local source can lag link-up (DAD ~1 s); opening at Full (Hellos have flowed) means the local link-local exists by then
- [ ] `internal/plugins/ospf/config.go` -- `interfaceConfig` struct (config.go:141: Name, AreaID, Enabled, NetworkType, Cost, HelloInterval, DeadInterval, ...); `parseInterface(entry)` (config.go:633) reads `hello-interval`/`dead-interval`/`retransmit-interval` via `configNumber`; `ospfConfig` carries the `address-family ipv6` sub-config; `DefaultDeadInterval = 40`
  -> Constraint: add a `BFD bfdInterfaceConfig` field to `interfaceConfig` and parse a nested `bfd` map in `parseInterface`; default disabled (opt-in). `parseInterface` is shared by the IPv4 list and the `address-family ipv6` interface list, so the BFD block parses for BOTH families from one code path
- [ ] `internal/component/bfd/api/service.go`, `events.go`, `registry.go` -- `GetService() Service`; `Service.EnsureSession(SessionRequest) (SessionHandle, error)`; `Service.ReleaseSession(SessionHandle) error`; `SessionHandle.Subscribe() <-chan StateChange` / `Unsubscribe`; `SessionRequest{Peer netip.Addr, Local netip.Addr, Interface, VRF, Mode, DesiredMinTxInterval, RequiredMinRxInterval, DetectMult, ...}`; `StateChange{Key, State, Diag, When}`; `StateDown`/`StateAdminDown`; `SingleHop`; `Key{Peer, Local, Interface, VRF, Mode}`
  -> Constraint: this is the FROZEN client contract BGP already uses (learned 560 "Service interface surface is frozen for this path"); OSPF reuses it verbatim on both families, adding NO methods to `api`. `Peer`/`Local` are `netip.Addr`, so IPv4 addresses and IPv6 link-locals pass through natively
  -> Constraint: `EnsureSession` refcounts on the `Key` tuple; two OSPF neighbours, or a v4 and a v6 session on the same interface, produce distinct keys (different address pair), so each adjacency gets its own session
- [ ] `internal/component/bgp/reactor/peer_bfd.go` -- the EXEMPLAR: `bfdClient{mu, svc, handle, sub, stop, done}`; `startBFDClient` (nil-safe GetService, EnsureSession, Subscribe, spawn `runBFDSubscriber`); `runBFDSubscriber` (drain sub; Down/AdminDown -> `Teardown`); `stopBFDClient` (close stop, Unsubscribe, ReleaseSession, wait done); `bfdRequestFor(settings)` builds the request
  -> Constraint: copy this structure verbatim, substituting OSPF types: per-neighbour `bfdClient` keyed by (interface, Router ID); `Teardown(...)` -> `table.NeighborDown(iface, id)`; `bfdRequestFor(settings)` -> the AF-dispatched `bfdRequestForNeighbor(...)`
- [ ] `internal/plugins/ospf/register.go` -- `registerOSPF()` registers the namespace, doctor, diagnostic codes, completions, config sections, the existing `ze_ospf_*` metric set; both engine instances (v4 + v6) register here
  -> Constraint: the new `ze_ospf_bfd_*` metrics register here ONCE (shared by both families via get-or-create); the OSPF doctor gains an informational check that BFD is configured-but-plugin-absent

**Behavior to preserve:**
- OSPF on Hello/Dead timers alone when BFD is not enabled or the BFD plugin is absent (additive-only contract), on BOTH families. Every existing OSPF/OSPFv3 functional/interop test must stay green with no config change.
- The NSM state machine and `Table.NeighborDown` semantics (a BFD-driven down is indistinguishable from an inactivity-timer-driven down to the rest of OSPF -- same `kill-nbr` event, same LSA re-origination, same SPF re-run), on both families.
- `neighborEventSink.NeighborUp/NeighborDown` continuing to re-originate self-LSAs and emit report-bus events; the BFD lifecycle is layered ON TOP, not in place of, these.
- Base OSPFv3 multicast transport at Hop-Limit 1 (learned 970); BFD's separate Hop-Limit-255 single-hop session does not change the base transport.
- The frozen `internal/component/bfd/api` surface (no new methods).

**Behavior to change:** (only what the task requires)
- `interfaceConfig` / `iface.Config` / `neighbor.InterfaceConfig` gain BFD fields (enabled + timers); the parse path serves both the IPv4 and `address-family ipv6` interface lists.
- `neighborEventSink` (or an equivalent lifecycle observer) opens a single-hop session on Full (AF-dispatched by `codec.IsV6()`) and releases it on leaving Full.
- A BFD Down transition now drives `Table.NeighborDown` (a new, faster trigger for an existing transition), on both families.
- `show ospf neighbor`/`interface` (IPv4) and `show ospf ipv6 neighbor`/`interface` (IPv6) surface BFD session state (additive columns).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Config (IPv4):** `ospf { interface eth0 { bfd { enabled true; min-tx 50; min-rx 50; multiplier 3 } } }` enters via the OSPF config sections -> `parseInterface` -> `interfaceConfig.BFD` on the IPv4 sub-config.
- **Config (IPv6):** `ospf { address-family ipv6 { interface eth0 { bfd { enabled true; ... } } } }` enters via the same `parseInterface` -> `interfaceConfig.BFD` on the v6 sub-config.
- **Adjacency up:** a neighbour reaches Full -> `Table.setStateLocked` emits `up` -> `emit` -> `neighborEventSink.NeighborUp(snap)` -> the BFD lifecycle opens a session (request built by the AF-dispatched builder).
- **BFD down:** the BFD engine declares the session Down -> `StateChange{State: StateDown}` on the subscription channel -> subscriber -> `Table.NeighborDown(interface, routerID)`.
- **Adjacency down (any other cause):** `neighborEventSink.NeighborDown(snap)` -> BFD lifecycle releases the session.

### Transformation Path
1. **Config parse:** `parseInterface` reads the nested `bfd` map (`enabled`, `min-tx`, `min-rx`, `multiplier`) into `interfaceConfig.BFD` (defaults: disabled; min-tx/min-rx 50 000 us; multiplier 3, matching common practice and the BGP defaults). Shared by the IPv4 and IPv6-family interface lists.
2. **Config flow:** `interfaceConfig.BFD` -> `iface.Config.BFD` (via `interfaceRuntimeConfigLocked`) -> `neighbor.InterfaceConfig.BFD` (via `neighborInterfaceConfig`). The engine retains a per-interface BFD config map.
3. **Session open (Full):** on `NeighborUp(snap)`, if `snap` is on a BFD-enabled interface AND `api.GetService() != nil`, build the request via the AF dispatch on `codec.IsV6()`:
   - IPv4 family: `SessionRequest{Peer: neighbour IPv4, Local: interface IPv4, Interface: ifname, Mode: SingleHop, DesiredMinTxInterval: minTx, RequiredMinRxInterval: minRx, DetectMult: multiplier}`.
   - IPv6 family: `SessionRequest{Peer: neighbour link-local, Local: interface link-local (from the v3 transport), Interface: ifname, Mode: SingleHop, ...timers}`.
   Then `EnsureSession`, `Subscribe`, spawn `runBFDSubscriber`. Store the per-neighbour `bfdClient` keyed by (interface, Router ID).
4. **Subscriber loop:** drains `<-chan StateChange`. On `StateDown`/`StateAdminDown`: log a warning, increment `ze_ospf_bfd_session_down_total`, call `Table.NeighborDown(interface, routerID)`. On Up/Init: log at debug. Exits on stop-chan or channel close.
5. **NSM down:** `Table.NeighborDown` runs the existing AF-neutral path: `kill-nbr` event, neighbour -> Down, `down` emission -> `neighborEventSink.NeighborDown` (which ALSO releases the BFD session in step 6, idempotently) -> self-LSA re-origination -> SPF re-run -> route withdrawal (IPv4 or IPv6 by family).
6. **Session release:** on `NeighborDown(snap)` (whether BFD-driven or timer-driven), the lifecycle looks up the per-neighbour `bfdClient`, closes its stop chan, `Unsubscribe`s, `ReleaseSession`s, waits the done chan, and forgets it. Idempotent: a no-op if no session was open.
7. **Interface down / config disable:** `InterfaceDown` / a reload that clears `bfd.enabled` releases every session on that interface through the same release path.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| OSPF config <-> interface config | nested `bfd` map parsed in `parseInterface` into `interfaceConfig.BFD`; flows to `iface.Config` + `neighbor.InterfaceConfig` for both families | [ ] |
| NSM Full transition <-> BFD lifecycle | `neighborEventSink.NeighborUp/NeighborDown` open/close sessions; needs the raw neighbour `netip.Addr` + interface; the request builder dispatches on `codec.IsV6()` | [ ] |
| OSPF engine <-> BFD engine | `api.GetService().EnsureSession/ReleaseSession`; `SessionHandle.Subscribe/Unsubscribe`; value-typed `SessionRequest`/`StateChange` with `netip.Addr` (no cross-boundary pointers) | [ ] |
| BFD Down <-> NSM down | subscriber calls `Table.NeighborDown(interface, routerID)` -- the existing injection seam (AF-neutral) | [ ] |
| v3 transport <-> BFD request (IPv6 only) | the interface's IPv6 link-local source (`LinkLocalSource()`) becomes the BFD `Local` for the v6 family | [ ] |
| Engine <-> Service availability | `GetService()` nil-check; OSPF degrades to timer-only and logs once | [ ] |

### Integration Points
- `internal/plugins/ospf/config.go` -- `interfaceConfig.BFD`, `parseInterface` (consumes the YANG `bfd` container on both interface lists).
- `internal/plugins/ospf/iface/iface.go` -- `Config.BFD` (carries enable + timers to the runtime).
- `internal/plugins/ospf/neighbor` -- `InterfaceConfig.BFD`; the up/down lifecycle needs the neighbour's raw `netip.Addr` + interface; `NeighborDown` is the down seam (consumed, not changed).
- `internal/plugins/ospf/instance.go` -- the BFD lifecycle observer (open on Full, close on down); reads the per-interface BFD config + (for v6) the v3 transport link-local source; owns the per-neighbour `bfdClient` map; AF-dispatches the request builder.
- `internal/plugins/ospf/v3/transport/` -- the `LinkLocalSource()` accessor for the interface's IPv6 link-local (consumed by the v6 builder; expose a typed engine-facing accessor if not already reachable).
- `internal/component/bfd/api` -- `GetService`, `Service`, `SessionHandle`, `SessionRequest`, `StateChange` (consumed verbatim; frozen surface).
- `internal/plugins/ospf/register.go` -- the three `ze_ospf_bfd_*` metrics (shared series); doctor informational check.

### Architectural Verification
- [ ] No bypassed layers (BFD down flows through `Table.NeighborDown` and the existing NSM, not a side path)
- [ ] No unintended coupling (OSPF imports only `bfd/api`; `internal/component/bfd` names no OSPF symbol)
- [ ] No duplicated functionality (reuses the delivered BFD engine + IPv6 transport, the `api.Service` lookup, the existing NSM down seam; ONE shared client lifecycle for both families; only the request builder forks)
- [ ] Zero-copy preserved where applicable (value-typed `SessionRequest`/`StateChange` with `netip.Addr`; no per-packet OSPF involvement)

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `api.GetService()` + `Service.EnsureSession`/`ReleaseSession`/`SessionHandle.Subscribe` is a frozen, in-process client contract OSPF can reuse exactly as BGP does, on both families | `internal/component/bfd/api/service.go`, `events.go`, `registry.go`; learned 560 "Service interface surface is frozen" | OSPF needs new api methods, widening the BFD surface | `TestOSPFBFDSessionOpenedOnFull` (v4) + `TestOSPFv3BFDSessionOpenedOnFull` (v6) against a fake `api.Service` | unvalidated |
| A-2 | `Table.NeighborDown(interfaceName, id)` is a safe, idempotent NSM down injection producing the same downstream effects (kill-nbr, LSA re-origination, SPF) as an inactivity-timer expiry, on both families | `neighbor/table.go` `neighborDown`; `nsmAdapter.NeighborDown` already calls it on the same unified table | a BFD-driven down behaves differently from a timer down | `TestOSPFBFDDownDrivesNeighborDown` + `TestOSPFv3BFDDownDrivesNeighborDown` | unvalidated |
| A-3 | The neighbour's reachable source (`Neighbor.Address`) is the correct single-hop BFD `Peer`, and the interface's own address is the `Local`: IPv4 family uses the interface IPv4 address; IPv6 family uses the interface IPv6 link-local (`v3 transport LinkLocalSource()`), never the `[4]byte` IPv4 `InterfaceAddress` | `neighbor.go` `Neighbor.Address`; `v3/transport/backend_linux.go:178`; RFC 5881 §6 on-subnet src/dst | the session binds to the wrong address pair and never comes Up against FRR | `TestBFDRequestForNeighbor` (v4 pair) + `TestBFDRequestForNeighborV6` (link-local pair); `ospf-bfd-frr` + `ospfv3-bfd-frr` interop | unvalidated |
| A-4 | Only Full adjacencies need a BFD session; opening at Full (not at 2-Way / Exchange) is correct and matches FRR `ospfd`/`ospf6d`, on both families | RFC 2328 / RFC 5340 (Full = adjacency carrying flooding); guide 1542-1545; `setStateLocked` emits up only on `next == stateFull` | sessions churn during ExStart/Exchange, or a half-formed adjacency is unprotected | `TestOSPFBFDOnlyAtFull` + `TestOSPFv3BFDOnlyAtFull`; both interop scenarios | unvalidated |
| A-5 | A single-hop session (`Mode: SingleHop`, TTL/Hop-Limit 255) is always correct for OSPF neighbours (directly connected) on both families | RFC 5881 §1 single-hop = directly-connected; OSPF adjacencies are link-local | a virtual-link / sham-link neighbour (multi-hop) is mis-protected | OSPF here forms only normal/stub/NSSA adjacencies (no virtual links in scope); documented in Known Limitations | unvalidated |
| A-6 | The lifecycle hook can read the engine's address family (`e.dispatch.codec.IsV6()`) so the v6 path builds an IPv6 link-local request and the v4 path builds an IPv4 request, without cross-firing | `instance.go` `e.dispatch.codec.IsV6()` already used to install the v6 encoder | a v6 engine opens an IPv4 session (or vice versa); the address pair is wrong | `TestOSPFv3BFDUsesLinkLocalPair` (v6) + the v4 path's `IsV6()==false` assertion in `TestBFDRequestForNeighbor` | unvalidated |
| A-7 | The BFD engine's IPv6 single-hop transport (learned 564) carries a link-local peer/local request end-to-end (v6 socket, Hop-Limit 255, GTSM RX) with NO OSPF transport code | `plan/learned/564-bfd-2b-ipv6-transport.md`; `internal/component/bfd/transport/udp_linux.go` | the engine cannot open a v6 session; OSPF would need to touch the BFD transport | `ospfv3-bfd-frr` interop brings a v6 session Up; the engine's existing v6 transport tests cover the socket path | unvalidated |
| A-8 | The interface's IPv6 link-local source exists by the time a v6 neighbour reaches Full (Hellos have flowed, so DAD completed); a zero `Local` is tolerated by the engine (kernel source selection) as a fallback | `v3/transport/backend_linux.go` DAD handling; learned 970 | the session opens with a zero/wrong `Local` and fails verification at the peer | `TestBFDRequestForNeighborV6` pins the resolved link-local; `ospfv3-bfd-frr` confirms Up | unvalidated |
| A-9 | `EnsureSession` refcounting on the `Key` tuple keeps a v4 (IPv4) and a v6 (link-local) session on the SAME physical link independent (RFC 5881 §2 one-per-protocol), and keeps two distinct neighbours' sessions independent | `api/events.go` `Key{Peer,Local,Interface,VRF,Mode}` | a v4 and a v6 session collide / one release tears down another | `TestOSPFBFDDistinctKeysPerNeighbor` + `TestOSPFv3BFDDistinctFromV2OnSameLink`; the engine's refcount tests | unvalidated |
| A-10 | A reload that flips `bfd.enabled` on an interface can open/close sessions for already-Full neighbours without re-forming the adjacency, on both families | the reload path in `instance.go`; BGP did the equivalent on its FSM | enabling BFD mid-adjacency requires a neighbour bounce (operator-visible churn) | `TestOSPFBFDReloadEnablesSessionForFullNeighbor` + `TestOSPFv3BFDReloadEnablesSessionForFullNeighbor` | unvalidated |
| A-11 | The shared `bfdClient` map does not cross-fire between the v4 and v6 engine instances (two engine instances, each owns its own map) | each `engine` owns its own per-neighbour map; `(iface, RouterID)` keys are scoped to one instance | a v6 down tears a v4 neighbour; map key collision | `TestOSPFBFDClientMapIsolatedPerEngine` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A BFD Down races a concurrent NSM timer-down for the same neighbour, double-firing `NeighborDown` | duplicate `kill-nbr` metric increments; a panic on a freed neighbour | `Table.NeighborDown` is idempotent (looks up; no-op if absent / already Down); the subscriber and release path tolerate "already gone"; `TestOSPFBFDDownIdempotentWithTimerDown` + v6 variant |
| R-2 | The session-open path needs the neighbour's raw `netip.Addr`, but `Snapshot.Address` is a string -- a parse round-trip could lose an IPv6 zone or fail | a malformed `Peer` address; session never comes Up | carry the raw `netip.Addr` in a typed lifecycle callback, not via the string Snapshot; `TestBFDRequestForNeighbor` + `TestBFDRequestForNeighborV6` pin the exact `netip.Addr` (incl. zone) |
| R-3 | A subscriber goroutine leaks if `stopBFDSession` does not wait the done chan, or if `ReleaseSession` closes the channel without the subscriber noticing | goroutine count grows per adjacency flap; `go test -race` / leak check fails | copy the `peer_bfd.go` stop+done handshake verbatim; `TestOSPFBFDSubscriberExitsOnRelease` + v6 variant; run the OSPF suite under `-race` |
| R-4 | The shared `bfdClient` map cross-fires between the v4 and v6 engine instances | a v6 down tears a v4 neighbour; map key collision | the per-neighbour map is per-engine-instance (each `engine` owns its own); `TestOSPFBFDClientMapIsolatedPerEngine` |
| R-5 | OSPF (a family) and BGP both open a single-hop session to the same neighbour with different timers; the engine picks the more aggressive value, surprising one client | a BFD session runs faster/slower than one client configured | documented engine behaviour (api doc: "engine picks the more aggressive value"); `ze_ospf_bfd_sessions` + the BFD `show` reflect the negotiated rate; acceptable per RFC 5882 refcounting. (A v4 and a v6 OSPF session on one link have distinct keys, so they do NOT coalesce) |
| R-6 | A BFD plugin shutdown (Service set to nil) while OSPF sessions are live leaves dangling handles | `ReleaseSession` after plugin teardown; subscriber sees a closed channel | `SetService(nil)` runs before `stopAll` (learned 560 gotcha); the subscriber exits on channel close; `ReleaseSession` on a torn-down loop is a documented no-op |
| R-7 | The `bfd` config container under the IPv4 `interface` and the `address-family ipv6 interface` collides with / shadows the top-level `bfd { }` plugin config or BGP's `bfd` container | parse error or wrong handler claims the section | the OSPF `bfd` leaf lives strictly under `ospf [address-family ipv6] interface`; YANG namespacing keeps it distinct; `ospf-bfd-config.ci` + `ospfv3-bfd-config.ci` prove coexistence |
| R-8 | Min-tx/min-rx of 0 (or absurdly small) is accepted and produces an unusable session | session never stabilises; CPU spin | YANG `range` validation on the timer leaves (10..255000 us echoing BFD's bounds); `parseInterface` rejects 0; boundary tests |
| R-9 | The interface's IPv6 link-local source is not yet available (DAD) when a v6 open is attempted, producing a zero `Local` that never verifies at the peer | session stuck in Down; FRR `bfdd` drops on source mismatch | open at Full (DAD long complete by then); tolerate zero `Local` (kernel selects) as a fallback; `ospfv3-bfd-frr` retries until Up; documented in Known Limitations |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ospf interface eth0 { bfd { enabled true } }` (IPv4) | -> | `parseInterface` sets `interfaceConfig.BFD.Enabled` on the IPv4 sub-config; flows to `neighbor.InterfaceConfig` | `TestParseInterfaceBFD` (unit) + `test/ospf/ospf-bfd-config.ci` |
| `ospf address-family ipv6 { interface eth0 { bfd { enabled true } } }` (IPv6) | -> | same `parseInterface` sets `interfaceConfig.BFD.Enabled` on the v6 sub-config | `TestParseInterfaceBFDv6` (unit) + `test/ospfv3/ospfv3-bfd-config.ci` |
| An IPv4 neighbour reaches Full on a BFD-enabled interface (`!IsV6()`) | -> | `neighborEventSink.NeighborUp` -> BFD lifecycle -> `api.Service.EnsureSession` (IPv4 pair, SingleHop) + `Subscribe` + subscriber spawned | `TestOSPFBFDSessionOpenedOnFull` (unit, fake Service) + `test/ospf/ospf-bfd-session.ci` |
| An IPv6 neighbour reaches Full on a BFD-enabled interface (`IsV6()`) | -> | `neighborEventSink.NeighborUp` -> BFD lifecycle -> `api.Service.EnsureSession` (link-local pair, SingleHop) + `Subscribe` + subscriber spawned | `TestOSPFv3BFDSessionOpenedOnFull` (unit, fake Service) + `test/ospfv3/ospfv3-bfd-session.ci` |
| BFD reports `StateDown` for a protected neighbour (either family) | -> | subscriber -> `Table.NeighborDown(interface, routerID)` -> neighbour to Down | `TestOSPFBFDDownDrivesNeighborDown` + `TestOSPFv3BFDDownDrivesNeighborDown` + both interop |
| The neighbour leaves Full (timer, interface down, reset) | -> | `neighborEventSink.NeighborDown` -> BFD lifecycle release (`Unsubscribe` + `ReleaseSession`) | `TestOSPFBFDSessionReleasedOnDown` + v6 variant |
| BFD plugin not loaded (`GetService()==nil`) on a BFD-enabled interface (either family) | -> | lifecycle logs a warning once and runs without BFD | `TestOSPFBFDGracefulWhenPluginAbsent` + `TestOSPFv3BFDGracefulWhenPluginAbsent` |

## Acceptance Criteria

| AC ID | Address family | Input / Condition | Expected Behavior |
|-------|----------------|-------------------|-------------------|
| AC-1 | IPv4 | `ospf interface eth0 { bfd { enabled true; min-tx 50; min-rx 50; multiplier 3 } }` | parsed into `interfaceConfig.BFD{Enabled:true, MinTxUs:50000, MinRxUs:50000, Multiplier:3}` on the IPv4 sub-config; surfaced in `show ospf interface` as BFD enabled |
| AC-1b | IPv6 | `ospf address-family ipv6 { interface eth0 { bfd { enabled true; min-tx 50; min-rx 50; multiplier 3 } } }` | parsed into the same struct on the v6 sub-config; surfaced in `show ospf ipv6 interface` as BFD enabled |
| AC-2 | IPv4 | An IPv4 neighbour on a BFD-enabled interface transitions to Full, BFD plugin loaded | exactly one IPv4 single-hop `api.SessionRequest` (Mode SingleHop, Peer = neighbour IPv4, Local = interface IPv4, Interface = ifname, timers from config) sent to `EnsureSession`; a subscriber goroutine running |
| AC-2b | IPv6 | An IPv6 neighbour on a BFD-enabled interface transitions to Full (engine `IsV6()`), BFD plugin loaded | exactly one IPv6 single-hop `api.SessionRequest` (Mode SingleHop, Peer = neighbour link-local, Local = interface link-local, Interface = ifname, timers from config) sent to `EnsureSession`; a subscriber goroutine running |
| AC-3 | both | A neighbour on an interface WITHOUT BFD enabled reaches Full | no `EnsureSession` call; OSPF runs on timers alone |
| AC-4 | both | BFD plugin not loaded (`GetService()==nil`), interface BFD-enabled, neighbour reaches Full | no session opened; a single warning logged; `ze_ospf_bfd_register_failures_total` incremented; OSPF unaffected |
| AC-5 | both | A protected session reports `StateDown` (Diag 1, detect-expired) | `Table.NeighborDown(interface, routerID)` invoked; the neighbour drops to Down; `ze_ospf_bfd_session_down_total` incremented; self-LSAs re-originated and SPF re-runs (route withdrawn, IPv4 or IPv6 by family) |
| AC-6 | both | A protected session reports `StateAdminDown` | treated identically to Down (neighbour declared down) |
| AC-7 | both | A protected session reports `StateUp` / `StateInit` | logged at debug; OSPF adjacency state unchanged (BFD is a failure detector, not a session driver) |
| AC-8 | both | A protected neighbour leaves Full for any reason (inactivity timer, interface down, `clear [ipv6] ospf neighbor`) | the BFD session is released (`Unsubscribe` + `ReleaseSession`); the subscriber goroutine exits; `ze_ospf_bfd_sessions` decrements |
| AC-9 | both | A BFD Down and an inactivity-timer Down race for the same neighbour | the neighbour drops exactly once; no panic; idempotent (R-1) |
| AC-10 | both | A config reload sets `bfd.enabled false` on an interface with Full neighbours | every BFD session on that interface is released; the adjacencies stay Full (BFD removal does not bounce the neighbour) |
| AC-11 | both | A config reload sets `bfd.enabled true` on an interface with already-Full neighbours | a session is opened for each already-Full neighbour without re-forming the adjacency |
| AC-12 | dual-stack | A dual-stack link runs both OSPFv2 and OSPFv3 with BFD on each | two distinct BFD sessions (distinct `Key`: IPv4 pair vs link-local pair); releasing one does not affect the other |
| AC-13 | per-AF | The request `Peer`/`Local` types | the v4 family request carries IPv4 `netip.Addr`; the v6 family request carries IPv6 link-local `netip.Addr` (never an IPv4 address); `Mode: SingleHop` in both |
| AC-14 | both | `min-tx 0` or `multiplier 0` in config | rejected at parse/validation time with a clear error (YANG `range` + `parseInterface` guard) |
| AC-15 | both | `show ospf neighbor` (IPv4) / `show ospf ipv6 neighbor` (IPv6) for a BFD-protected, Up neighbour | shows the BFD session state (Up); a down/absent session is distinguishable |
| AC-16 | both | The OSPF plugin is removed from the build | no `ze_ospf_bfd_*` metrics, no OSPF BFD code; the BFD engine and BGP-BFD client are unaffected (self-containment) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables BFD on an OSPFv2 interface and forms an adjacency with FRR `ospfd` | config -> `parseInterface` -> Full -> `EnsureSession` (IPv4 single-hop) -> BFD handshake with FRR's `bfdd` -> session Up | `test/ospf/ospf-bfd-session.ci` + `ospf-bfd-frr` interop |
| 2 | Enables BFD on an OSPFv3 interface and forms an adjacency with FRR `ospf6d` | config -> `parseInterface` (v6 sub-config) -> Full -> `EnsureSession` (IPv6 single-hop, link-local pair) -> BFD handshake with FRR's `bfdd` -> session Up | `test/ospfv3/ospfv3-bfd-session.ci` + `ospfv3-bfd-frr` interop |
| 3 | Pulls the link / kills FRR; OSPF detects the loss in the BFD detection window, not after RouterDeadInterval | BFD detect-timer expiry -> `StateDown` -> subscriber -> `Table.NeighborDown` -> neighbour Down -> SPF re-run -> route withdrawal, all well under 40 s (both families) | `ospf-bfd-frr` + `ospfv3-bfd-frr` measure detect time < RouterDeadInterval |
| 4 | Runs `show ospf neighbor` / `show ospf ipv6 neighbor` and sees which adjacencies are BFD-protected and the session state | snapshot -> neighbour rows annotated with BFD state from the session map | `test/ospf/ospf-bfd-show.ci` + `test/ospfv3/ospfv3-bfd-show.ci` |
| 5 | Runs BFD on a dual-stack link for both OSPFv2 and OSPFv3 simultaneously | two distinct sessions (IPv4 pair + link-local pair); each family's down is independent | `TestOSPFv3BFDDistinctFromV2OnSameLink` + `ospfv3-bfd-frr` dual-stack variant |
| 6 | Disables BFD on a live link without dropping OSPF | reload `bfd { enabled false }` -> sessions released -> adjacencies stay Full | `test/ospf/ospf-bfd-disable.ci` + `test/ospfv3/ospfv3-bfd-disable.ci` |
| 7 | Runs OSPF on a box where the BFD plugin was never loaded | `GetService()==nil` -> warning -> OSPF on timers; no crash, no blocked adjacency | `TestOSPFBFDGracefulWhenPluginAbsent` + `TestOSPFv3BFDGracefulWhenPluginAbsent` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Address family | Validates | Status |
|------|------|----------------|-----------|--------|
| `TestParseInterfaceBFD` | `internal/plugins/ospf/config_test.go` | IPv4 | AC-1, AC-14: parse `bfd` container on the IPv4 interface list; defaults; reject 0 timers/multiplier | |
| `TestParseInterfaceBFDv6` | `internal/plugins/ospf/config_test.go` | IPv6 | AC-1b, AC-14: parse `bfd` container under `address-family ipv6 interface` | |
| `TestBFDRequestForNeighbor` | `internal/plugins/ospf/bfd_client_test.go` | IPv4 | AC-2, AC-13, A-3, R-2: SessionRequest IPv4 address pair + Mode SingleHop + timers from config | |
| `TestBFDRequestForNeighborV6` | `internal/plugins/ospf/bfd_client_v6_test.go` | IPv6 | AC-2b, AC-13, A-3, A-8, R-2: SessionRequest link-local pair (Peer = neighbour LL, Local = interface LL incl. zone) + Mode SingleHop | |
| `TestOSPFBFDSessionOpenedOnFull` | `internal/plugins/ospf/bfd_client_test.go` | IPv4 | AC-2, A-1: one EnsureSession + subscriber on Full (fake `api.Service`) | |
| `TestOSPFv3BFDSessionOpenedOnFull` | `internal/plugins/ospf/bfd_client_v6_test.go` | IPv6 | AC-2b, A-1: one EnsureSession + subscriber on Full for a v6 engine | |
| `TestOSPFBFDNotOpenedWhenDisabled` | `internal/plugins/ospf/bfd_client_test.go` | both | AC-3: no EnsureSession when interface BFD disabled | |
| `TestOSPFBFDOnlyAtFull` | `internal/plugins/ospf/bfd_client_test.go` | IPv4 | AC-2, A-4: no session before Full (Init/Exchange) | |
| `TestOSPFv3BFDOnlyAtFull` | `internal/plugins/ospf/bfd_client_v6_test.go` | IPv6 | AC-2b, A-4: no session before Full | |
| `TestOSPFv3BFDUsesLinkLocalPair` | `internal/plugins/ospf/bfd_client_v6_test.go` | IPv6 | AC-13, A-6: a v6 engine produces an IPv6 link-local pair, never an IPv4 address | |
| `TestOSPFBFDGracefulWhenPluginAbsent` | `internal/plugins/ospf/bfd_client_test.go` | IPv4 | AC-4: nil Service -> warning + failure metric, no session | |
| `TestOSPFv3BFDGracefulWhenPluginAbsent` | `internal/plugins/ospf/bfd_client_v6_test.go` | IPv6 | AC-4: nil Service -> warning + failure metric, no session | |
| `TestOSPFBFDDownDrivesNeighborDown` | `internal/plugins/ospf/bfd_client_test.go` | IPv4 | AC-5, A-2: StateDown -> `Table.NeighborDown` -> Down + LSA re-origination | |
| `TestOSPFv3BFDDownDrivesNeighborDown` | `internal/plugins/ospf/bfd_client_v6_test.go` | IPv6 | AC-5, A-2: StateDown -> `Table.NeighborDown` -> Down + LSA re-origination | |
| `TestOSPFBFDAdminDownTreatedAsDown` | `internal/plugins/ospf/bfd_client_test.go` | both | AC-6: StateAdminDown -> neighbour Down | |
| `TestOSPFBFDUpInitNoTeardown` | `internal/plugins/ospf/bfd_client_test.go` | both | AC-7: Up/Init logged, no NSM change | |
| `TestOSPFBFDSessionReleasedOnDown` | `internal/plugins/ospf/bfd_client_test.go` | both | AC-8: leaving Full -> Unsubscribe + ReleaseSession; subscriber exits | |
| `TestOSPFBFDSubscriberExitsOnRelease` | `internal/plugins/ospf/bfd_client_test.go` | both | R-3: subscriber goroutine exits on stop and on channel close (no leak) | |
| `TestOSPFBFDDownIdempotentWithTimerDown` | `internal/plugins/ospf/bfd_client_test.go` | both | AC-9, R-1: BFD down + timer down race drops the neighbour once, no panic | |
| `TestOSPFBFDDistinctKeysPerNeighbor` | `internal/plugins/ospf/bfd_client_test.go` | IPv4 | AC-12 (partial), A-9: two neighbours -> two distinct session Keys; independent release | |
| `TestOSPFv3BFDDistinctFromV2OnSameLink` | `internal/plugins/ospf/bfd_client_v6_test.go` | dual-stack | AC-12, A-9: a v3 link-local key and a v2 IPv4 key on one interface are distinct; independent release | |
| `TestOSPFBFDClientMapIsolatedPerEngine` | `internal/plugins/ospf/bfd_client_v6_test.go` | both | R-4, A-11: the per-neighbour map is per-engine-instance; a v6 down does not touch a v4 neighbour | |
| `TestOSPFBFDReloadDisableKeepsAdjacency` | `internal/plugins/ospf/bfd_client_test.go` | both | AC-10: reload disable releases sessions, adjacency stays Full | |
| `TestOSPFBFDReloadEnablesSessionForFullNeighbor` | `internal/plugins/ospf/bfd_client_test.go` | IPv4 | AC-11, A-10: reload enable opens sessions for already-Full neighbours | |
| `TestOSPFv3BFDReloadEnablesSessionForFullNeighbor` | `internal/plugins/ospf/bfd_client_v6_test.go` | IPv6 | AC-11, A-10: reload enable opens sessions for already-Full v6 neighbours | |
| `TestOSPFBFDMetrics` | `internal/plugins/ospf/metrics_test.go` | both | sessions gauge / down counter / register-failure counter move correctly under the `interface` label | |

### Boundary Tests (MANDATORY for numeric inputs)
<!-- Same leaves on both interface lists; one boundary table covers both families. -->
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `min-tx` (us) | 10..255000 | 255000 | 0 (rejected) | 255001 (rejected) |
| `min-rx` (us) | 10..255000 | 255000 | 0 (rejected) | 255001 (rejected) |
| `multiplier` | 1..255 | 255 | 0 (rejected) | 256 (rejected, uint8 range) |

### Functional Tests
| Test | Location | Address family | End-User Scenario | Status |
|------|----------|----------------|-------------------|--------|
| `ospf-bfd-config` | `test/ospf/ospf-bfd-config.ci` | IPv4 | `bfd { enabled true; min-tx ...; multiplier ... }` parses; coexists with the top-level `bfd` and BGP `bfd` | |
| `ospf-bfd-session` | `test/ospf/ospf-bfd-session.ci` | IPv4 | a Full adjacency opens a BFD session; `show ospf neighbor` shows it protected | |
| `ospf-bfd-show` | `test/ospf/ospf-bfd-show.ci` | IPv4 | `show ospf interface`/`neighbor` render BFD enabled + session state | |
| `ospf-bfd-disable` | `test/ospf/ospf-bfd-disable.ci` | IPv4 | reload disabling BFD releases sessions without dropping the adjacency | |
| `ospfv3-bfd-config` | `test/ospfv3/ospfv3-bfd-config.ci` | IPv6 | `bfd { ... }` parses under `address-family ipv6 interface`; coexists with the v2 `bfd`, top-level `bfd`, and BGP `bfd` | |
| `ospfv3-bfd-session` | `test/ospfv3/ospfv3-bfd-session.ci` | IPv6 | a Full v3 adjacency opens a BFD session; `show ospf ipv6 neighbor` shows it protected | |
| `ospfv3-bfd-show` | `test/ospfv3/ospfv3-bfd-show.ci` | IPv6 | `show ospf ipv6 interface`/`neighbor` render BFD enabled + session state | |
| `ospfv3-bfd-disable` | `test/ospfv3/ospfv3-bfd-disable.ci` | IPv6 | reload disabling BFD releases sessions without dropping the v3 adjacency | |

### Interop Tests (MANDATORY for protocol features)
<!-- REQUIRED when the spec adds/changes wire protocol behavior. -->
| Scenario | Directory | Address family | Peer Daemon | What It Proves | Status |
|----------|-----------|----------------|-------------|----------------|--------|
| `ospf-bfd-frr` | `test/interop/scenarios/ospf-bfd-frr/` | IPv4 | FRR `ospfd` + `bfdd` (`ip ospf bfd`) | Ze and FRR form a Full OSPFv2 adjacency AND an IPv4 single-hop BFD Up session; pulling the link drives an OSPF neighbour-down in the BFD detection window (well under RouterDeadInterval); re-adding re-forms both | |
| `ospfv3-bfd-frr` | `test/interop/scenarios/ospfv3-bfd-frr/` | IPv6 (+ dual-stack variant) | FRR `ospf6d` + `bfdd` (`ipv6 ospf6 bfd`) | Ze and FRR form a Full OSPFv3 adjacency AND an IPv6 single-hop BFD Up session (link-local pair, Hop-Limit 255); pulling the link drives a v3 neighbour-down in the BFD detection window; the dual-stack variant proves the v2 and v3 sessions are independent (RFC 5881 §2, AC-12) | |

> Interop is required: this exercises real single-hop BFD wire behaviour (UDP 3784,
> TTL/Hop-Limit 255 GTSM, three-way handshake) against an independent
> implementation, on both families. The single-hop raw path is Linux-only and runs
> as a QEMU integration test (`ai/rules/qemu-testing.md`), consistent with the BFD
> and OSPF interop sets. Both scenarios are true failover tests, not wiring smoke
> tests.

### Future (if deferring any tests)
- None. Every AC is covered by a unit, functional, or interop test above.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `internal/plugins/ospf/config.go` -- `interfaceConfig.BFD bfdInterfaceConfig` field; `bfdInterfaceConfig` struct (Enabled, MinTxUs, MinRxUs, Multiplier); parse the nested `bfd` map in `parseInterface` (shared by the IPv4 and `address-family ipv6` interface lists); validate timers/multiplier
- `internal/plugins/ospf/iface/iface.go` -- `Config.BFD` fields (carry enable + timers to the runtime layer; AF-neutral)
- `internal/plugins/ospf/neighbor/neighbor.go` / `table.go` -- carry the per-interface BFD config into `InterfaceConfig`; ensure the up/down lifecycle can obtain the neighbour's raw `netip.Addr` + interface (typed lifecycle callback or `Lookup`/`AddressOf` accessor); `NeighborDown` consumed unchanged
- `internal/plugins/ospf/instance.go` -- the AF-neutral BFD lifecycle observer: open on Full, release on down; the per-neighbour `bfdClient` map lives on the engine; `bfdRequestForNeighbor` dispatches on `e.dispatch.codec.IsV6()`; `neighborInterfaceConfig`/`interfaceRuntimeConfigLocked` carry the BFD fields; for the v6 family read the interface IPv6 link-local source from the v3 transport
- `internal/plugins/ospf/v3/transport/transport.go` / `backend_linux.go` -- expose a typed engine-facing accessor for the interface's IPv6 link-local source (`LinkLocalSource()` exists on the backend; surface it through the orchestrator if not already reachable from the engine)
- `internal/plugins/ospf/register.go` -- register `ze_ospf_bfd_sessions`, `ze_ospf_bfd_session_down_total`, `ze_ospf_bfd_register_failures_total` (shared series for both families; get-or-create); doctor informational check (BFD configured but plugin absent)
- `internal/plugins/ospf/cmd_show.go` / `show_summary.go` -- annotate `show ospf neighbor`/`interface` (IPv4) and `show ospf ipv6 neighbor`/`interface` (IPv6) rows with BFD enabled + session state
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- a `container bfd` under the `list interface` in BOTH the IPv4 tree and the `address-family ipv6` tree, with `enabled` (boolean), `min-tx`/`min-rx` (uint32 us, range), `multiplier` (uint8, range)
- `internal/plugins/ospf/doctor.go` -- informational doctor check: BFD enabled on an interface but `api.GetService()` nil
- `internal/core/diagnostic/codes.go` -- the doctor diagnostic code for the BFD-configured-but-absent check (per `ai/rules/doctor-checks.md`)
- `plan/spec-ospf-ext-0-umbrella.md` -- (note, not code) add the `ze_ospf_bfd_*` rows to the umbrella "Metrics" table; mark the Engine<->BFD boundary row covered for both families

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `bfd` container on `interface` (both trees); read `ai/rules/config-surface.md` (operational config, not env var) + `ai/rules/config-naming.md` (kebab-case leaves) |
| YANG validation constraints | [ ] yes | `enabled` boolean; `min-tx`/`min-rx` `uint32 { range "10..255000"; }`; `multiplier` `uint8 { range "1..255"; }`; `units microseconds` on the timers |
| YANG custom validators | [ ] no | native `range` + `boolean` suffice |
| CLI commands/flags | [ ] yes | annotate `show ospf interface`/`neighbor` and `show ospf ipv6 interface`/`neighbor` with a BFD column; no new top-level verb |
| CLI grammar (action before identifier) | [ ] n/a | no new verb added |
| Editor autocomplete | [ ] yes | automatic for the YANG boolean/uint leaves under `bfd` |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-bfd-*.ci` + `test/ospfv3/ospfv3-bfd-*.ci` |
| Pipe completeness | [ ] yes | the annotated show output already routes through `ApplyPipes` like the rest of OSPF show |
| Env var registration | [ ] no | per-interface operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] yes | informational only: BFD enabled but plugin absent (no new socket/port/binary -- the BFD engine owns those); register a diagnostic code in `internal/core/diagnostic/codes.go` per `ai/rules/doctor-checks.md` |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (shared series, unified `ze_ospf_*` namespace per learned 970)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospf_bfd_sessions` | gauge | `interface`, `area` |
| `ze_ospf_bfd_session_down_total` | counter | `interface` |
| `ze_ospf_bfd_register_failures_total` | counter | `interface`, `reason` (plugin-absent / ensure-error) |

> ONE set of series produced by BOTH families; the metrics registry is get-or-create
> by name, so v2 and v3 share one series and the `interface` label distinguishes the
> families. Do NOT introduce a `ze_ospfv3_bfd_*` namespace (user directive, learned
> 970). The umbrella "Metrics" table must gain these rows when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- BFD for OSPF (both families) |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` + `docs/guide/ospf.md` -- the per-interface `bfd` block under both `ospf interface` and `address-family ipv6 interface` |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- BFD column in `show ospf` and `show ospf ipv6` neighbor/interface |
| 4 | API/RPC added/changed? | [ ] no | reuses the frozen `internal/component/bfd/api` surface; no new RPC |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains a BFD client (both families) |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` + `docs/guide/bfd.md` -- OSPF opt-in section (IPv4 and IPv6) |
| 7 | Wire format changed? | [ ] no | OSPF wire unchanged; BFD wire is the delivered engine's single-hop paths |
| 8 | Plugin SDK/protocol changed? | [ ] no | uses the existing `api.Service` lookup |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc5880.md` + `rfc/short/rfc5881.md` -- the OSPF-client-relevant checklist context (client model, single-hop IPv4 + IPv6, GTSM consumed) |
| 10 | Test infrastructure changed? | [ ] yes (interop scenarios added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPF BFD parity with FRR `ospfd`/`ospf6d` |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- NSM <-> BFD lifecycle (AF-neutral, AF-dispatched request) |
| 13 | Route metadata keys added/changed? | [ ] no | BFD does not install routes |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the three `ze_ospf_bfd_*` series (both families produce them) |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` + the umbrella metrics table |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed OSPF files |
| 17 | Existing docs show examples for this area? | [ ] check | verify OSPF interface config examples (both families) against the new `bfd` block; verify `docs/guide/bfd.md` OSPF section |

## Files to Create
- `internal/plugins/ospf/bfd_client.go` -- the AF-neutral OSPF BFD client: `bfdClient` struct, `startBFDSession`/`stopBFDSession`/`runBFDSubscriber`, the per-neighbour session map and its mutex, and the AF-dispatching `bfdRequestForNeighbor` plus the IPv4 builder (near-copy of `peer_bfd.go`, OSPF-typed)
- `internal/plugins/ospf/bfd_client_v6.go` -- the IPv6 request builder `bfdRequestForNeighborV6(neighbourLL, interfaceLL netip.Addr, ifname string, ifcfg bfdInterfaceConfig)` and the v6 link-local source lookup glue; the shared lifecycle stays in `bfd_client.go`
- `internal/plugins/ospf/bfd_client_test.go` -- the IPv4 + shared-lifecycle unit suite, driven by a `fakeBFDService` fake (mirrors `peer_bfd_test.go`)
- `internal/plugins/ospf/bfd_client_v6_test.go` -- the IPv6 unit suite (link-local pair, dual-stack independence, per-engine map isolation)
- `test/ospf/ospf-bfd-config.ci`, `test/ospf/ospf-bfd-session.ci`, `test/ospf/ospf-bfd-show.ci`, `test/ospf/ospf-bfd-disable.ci`
- `test/ospfv3/ospfv3-bfd-config.ci`, `test/ospfv3/ospfv3-bfd-session.ci`, `test/ospfv3/ospfv3-bfd-show.ci`, `test/ospfv3/ospfv3-bfd-disable.ci`
- `test/interop/scenarios/ospf-bfd-frr/` -- `ze.conf`, `frr.conf` (`ospfd` + `bfdd`, `ip ospf bfd`), `check.py`
- `test/interop/scenarios/ospfv3-bfd-frr/` -- `ze.conf`, `frr.conf` (`ospf6d` + `bfdd`, `ipv6 ospf6 bfd`; dual-stack variant), `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm `api.Service`, `Table.NeighborDown`, `neighborEventSink`, `codec.IsV6()`, `LinkLocalSource()` exist as described |
| 3. Wiring phase | Wiring Test table -- the BFD client skeleton + failing wiring tests (both families) |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- the AF-neutral BFD client skeleton hooked into the NSM lifecycle
   - Tests: `TestOSPFBFDSessionOpenedOnFull` (v4, fake Service), `TestOSPFv3BFDSessionOpenedOnFull` (v6), `TestOSPFBFDGracefulWhenPluginAbsent` (+ v6), `test/ospf/ospf-bfd-session.ci`, `test/ospfv3/ospfv3-bfd-session.ci`
   - Files: `bfd_client.go` (`startBFDSession`/`stopBFDSession` stubs, per-neighbour map, AF dispatch stub), `bfd_client_v6.go` (v6 builder stub), `instance.go` (call them from `neighborEventSink.NeighborUp/NeighborDown`), a `fakeBFDService` test fake
   - Verify: a Full transition reaches `EnsureSession` (or degrades gracefully on nil) on the correct family; deeper behaviour still stubbed so down/release tests fail
2. **Phase: Config surface** -- the per-interface `bfd` block on both interface lists
   - Tests: `TestParseInterfaceBFD`, `TestParseInterfaceBFDv6`, boundary tests, `ospf-bfd-config.ci`, `ospfv3-bfd-config.ci`
   - Files: `yang/ze-ospf-conf.yang` (the `bfd` container on both interface lists), `config.go` (`bfdInterfaceConfig` + parse + validation), `iface/iface.go` (`Config.BFD`), `instance.go` (`neighborInterfaceConfig`/`interfaceRuntimeConfigLocked` carry the fields), `neighbor` (`InterfaceConfig.BFD`)
   - Verify: config parses on both families; 0 timers/multiplier rejected; the engine sees per-interface BFD config
3. **Phase: AF request builders + open on Full** -- build the single-hop request from the neighbour + config, dispatched by family
   - Tests: `TestBFDRequestForNeighbor` (v4), `TestBFDRequestForNeighborV6` (v6), `TestOSPFv3BFDUsesLinkLocalPair`, `TestOSPFBFDNotOpenedWhenDisabled`, `TestOSPFBFDOnlyAtFull`/`TestOSPFv3BFDOnlyAtFull`, `TestOSPFBFDDistinctKeysPerNeighbor`, `TestOSPFv3BFDDistinctFromV2OnSameLink`
   - Files: `bfd_client.go` (IPv4 builder + AF dispatch on `codec.IsV6()`, the typed lifecycle callback carrying `netip.Addr`), `bfd_client_v6.go` (`bfdRequestForNeighborV6`), `v3/transport` (surface `LinkLocalSource()` to the engine)
   - Verify: correct address pair per family + Mode SingleHop + timers; only Full, only when enabled; distinct keys per neighbour and across families
4. **Phase: Down injection + subscriber** -- BFD Down -> NSM down (shared)
   - Tests: `TestOSPFBFDDownDrivesNeighborDown` (+ v6), `TestOSPFBFDAdminDownTreatedAsDown`, `TestOSPFBFDUpInitNoTeardown`, `TestOSPFBFDSubscriberExitsOnRelease`, `TestOSPFBFDDownIdempotentWithTimerDown`, `TestOSPFBFDClientMapIsolatedPerEngine`, both interop
   - Files: `bfd_client.go` (`runBFDSubscriber` -> `Table.NeighborDown`), the stop+done handshake
   - Verify: Down/AdminDown drop the neighbour through the existing NSM (both families); Up/Init inert; subscriber never leaks; race with timer-down idempotent; per-engine map isolation
5. **Phase: Release lifecycle + reload** -- release on leaving Full, on interface down, on config disable/enable
   - Tests: `TestOSPFBFDSessionReleasedOnDown`, `TestOSPFBFDReloadDisableKeepsAdjacency`, `TestOSPFBFDReloadEnablesSessionForFullNeighbor` (+ v6), `ospf-bfd-disable.ci`, `ospfv3-bfd-disable.ci`
   - Files: `bfd_client.go` (`stopBFDSession`), `instance.go` (release on `NeighborDown`/`InterfaceDown`/reload diff)
   - Verify: sessions release cleanly; disabling BFD does not bounce the adjacency; enabling opens sessions for already-Full neighbours
6. **Phase: CLI + metrics + doctor** -- operator surface
   - Tests: `TestOSPFBFDMetrics`, `ospf-bfd-show.ci`, `ospfv3-bfd-show.ci`, `TestOSPFBFDShowNeighbor`
   - Files: `register.go` (metrics, shared series), `cmd_show.go`/`show_summary.go` (BFD annotation on both show verb sets), `doctor.go` + `internal/core/diagnostic/codes.go` (informational check)
   - Verify: both show verb sets show BFD state; the three metric series move under the `interface` label; the doctor check fires when BFD is configured-but-absent
7. **Functional tests** -> the eight `.ci` cover the user-visible behaviour on both families
8. **RFC refs** -> add `// RFC 5881 Section X` / `// RFC 5880 Section X` comments on the single-hop request(s), the GTSM rationale (and its independence from base-OSPFv3 Hop-Limit-1), the Down-handling, and the slow-start note
9. **Interop** -> `ospf-bfd-frr` + `ospfv3-bfd-frr` QEMU scenarios (true failover tests; the v6 scenario carries a dual-stack variant for AC-12)
10. **Full verification** -> `make ze-verify`
11. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation (both families) |
| Feature completeness | each user story has a working path; parity with FRR's `ip ospf bfd` and `ipv6 ospf6 bfd` (session on adjacency, immediate down on BFD loss) |
| Correctness | session opened only at Full; the AF dispatch picks the right builder by `codec.IsV6()`; v4 uses the IPv4 pair, v6 uses the link-local pair (never the `[4]byte` IPv4 `InterfaceAddress`); only Down/AdminDown drives `NeighborDown`; Mode SingleHop always; idempotent down |
| Naming | `ze_ospf_bfd_*` metrics (NOT `ze_ospfv3_*`); YANG `bfd`/`min-tx`/`min-rx`/`multiplier` kebab-case; `startBFDSession`/`stopBFDSession`/`bfdRequestForNeighbor`/`bfdRequestForNeighborV6` |
| Data flow | BFD down flows through `Table.NeighborDown` + existing NSM; OSPF imports only `bfd/api`; no OSPF symbol in `internal/component/bfd`; the v6 `Local` comes from the v3 transport |
| CLI grammar | no new verb; show annotation only on both verb sets |
| Doctor checks | informational BFD-configured-but-absent check registered per `ai/rules/doctor-checks.md` |
| YANG validation | `bfd` leaves have `range`/`boolean`; 0 timers/multiplier rejected; bare `type string` absent |
| Prometheus counters | the three shared series produced by both families; umbrella table updated |
| Rule: plugin-self-containment | removing OSPF removes all its BFD wiring; BFD engine + BGP client untouched |
| Rule: goroutine-lifecycle | the subscriber is one per-session worker with stop+done handshake; no leak under `-race`; per-engine map isolation (R-4) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| OSPFv2 opens an IPv4 single-hop BFD session on Full | `go test ./internal/plugins/ospf -run TestOSPFBFDSessionOpenedOnFull` |
| OSPFv3 opens an IPv6 single-hop BFD session on Full | `go test ./internal/plugins/ospf -run TestOSPFv3BFDSessionOpenedOnFull` |
| The v6 request carries the IPv6 link-local pair, not an IPv4 address | `go test ./internal/plugins/ospf -run TestBFDRequestForNeighborV6` |
| BFD Down drives `Table.NeighborDown` (both families) | `go test ./internal/plugins/ospf -run 'TestOSPF.*BFDDownDrivesNeighborDown'` |
| Per-interface `bfd` config parses + validates (both families) | `go test ./internal/plugins/ospf -run 'TestParseInterfaceBFD'` |
| Graceful degradation when plugin absent | `go test ./internal/plugins/ospf -run 'TestOSPF.*BFDGracefulWhenPluginAbsent'` |
| Shared `ze_ospf_bfd_*` series (no `ze_ospfv3_bfd_*`) | `grep -rn 'ze_ospf_bfd_' internal/plugins/ospf` and `grep -rn 'ze_ospfv3_bfd_' internal/plugins/ospf` (latter empty) |
| Interop scenarios present | `ls test/interop/scenarios/ospf-bfd-frr/ test/interop/scenarios/ospfv3-bfd-frr/` |
| Functional tests present | `ls test/ospf/ospf-bfd-*.ci test/ospfv3/ospfv3-bfd-*.ci` |
| Only `bfd/api` imported from OSPF | `grep -rn 'internal/component/bfd' internal/plugins/ospf` shows only `/api` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | `bfd` timer/multiplier leaves range-checked; the neighbour address fed to `SessionRequest` is a validated `netip.Addr` (with zone for v6), never an unparsed string |
| Resource exhaustion | one session per Full adjacency (bounded by neighbour count, per family); the engine shares one UDP loop per (vrf, single-hop); `ze_ospf_bfd_sessions` gauge observable |
| Subscriber isolation | a panicking subscriber cannot wedge the NSM lock; `Table.NeighborDown` takes its own lock and the subscriber calls it outside any OSPF lock |
| Trust boundary | BFD single-hop relies on GTSM (TTL/Hop-Limit 255) enforced by the engine; OSPF adds no new listening port or socket -- the BFD engine owns the wire; base OSPFv3 Hop-Limit-1 is a separate transport unaffected |
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
BFD-for-OSPF is a wiring problem on a unified v2/v3 engine, not a protocol problem.
The BFD engine (including the delivered IPv4 and IPv6 single-hop transports), its
in-process `api.Service` client contract, and the AF-neutral down-injection seam
(`Table.NeighborDown`) all already exist, and OSPF is one engine distinguished by
`codec.IsV6()`. The spec connects two ends that are both in tree -- NSM Full ->
`EnsureSession`, BFD Down -> `NeighborDown` -- with ONE shared client lifecycle
(`peer_bfd.go` is the template). The ONLY per-address-family divergence is the
request builder: IPv4 uses the interface IPv4 address; IPv6 uses the interface
link-local source from the v3 transport. Everything else (config block, client map,
subscriber discipline, metrics, doctor) is shared.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One shared BFD client lifecycle on the unified engine, AF-dispatched only at the request builder | a separate OSPFv3 BFD client package; a second engine field | OSPF is one engine v2/v3; one lifecycle + one per-neighbour map keyed `(iface, RouterID)` per engine-instance avoids duplicate code; the `codec.IsV6()` gate keeps the v4/v6 request builders apart |
| `Peer`/`Local` per family: IPv4 pair for v2, link-local pair for v3 (interface LL from the v3 transport) | reuse `iface.Config.InterfaceAddress` ([4]byte) for both | `InterfaceAddress` is IPv4-only; OSPFv3 adjacencies and BFD single-hop are link-local; RFC 5881 §6 on-subnet src/dst maps to the per-family pair |
| Distinct BFD session per family on a dual-stack link | one shared session for both families | RFC 5881 §2 mandates one session per data protocol; the differing address pair gives distinct `Key` tuples and independent refcounts automatically |
| Reuse the `ze_ospf_*` metric namespace (no `ze_ospfv3_*`) | a parallel `ze_ospfv3_bfd_*` series | user directive (learned 970): "OSPFv3 is our OSPF"; the `interface` label distinguishes families on one series |
| Drive `Table.NeighborDown` on BFD Down | a dedicated BFD-down NSM event; a direct state poke | the existing seam already produces every correct downstream effect (kill-nbr, LSA re-origination, SPF); a BFD-down stays indistinguishable from a timer-down |
| Open the session only at Full | open at 2-Way / Exchange | Full is the adjacency actually carrying flooding; matches FRR; avoids churn during ExStart/Exchange |
| Single-hop only (`Mode: SingleHop`) | configurable hop mode | OSPF neighbours are directly connected by definition; multi-hop is for virtual/sham links (out of scope) |
| Per-interface `bfd` config block on both interface lists | a global OSPF BFD toggle; per-area | BFD is a link property; FRR's `ip ospf bfd` / `ipv6 ospf6 bfd` are per-interface; matches the OSPF config grain |
| Reuse the frozen `api.Service` lookup | a new OSPF-specific BFD API | learned 560 froze this surface for exactly this case; adding methods would widen the BFD boundary |
| Copy `peer_bfd.go` structure verbatim | a fresh design | proven nil-safe, leak-free, mutex-guarded lifecycle; divergence would re-introduce solved bugs |

## Known Limitations
- Virtual links / sham links (multi-hop OSPF adjacencies) are not BFD-protected by this spec (single-hop only), on either family.
- A freshly-formed adjacency detects in up to `multiplier * 1 s` until the session reaches Up and fast rates negotiate (RFC 5880 §6.8.3 slow-start); configured `min-tx`/`min-rx` apply only after Up.
- BFD authentication and echo are engine concerns; OSPF sessions run unauthenticated, async, no-echo in this spec.
- (IPv6) If the interface IPv6 link-local source has not resolved (DAD) when a session opens, `Local` may be left zero (kernel source selection); in practice DAD completes long before a neighbour reaches Full (Hellos have already flowed). The `ospfv3-bfd-frr` interop retries until Up.

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code:
- RFC 5881 §3 both ends Active -> the request builders leave `Passive` false (both families)
- RFC 5881 §6 on-subnet src/dst -> the IPv4 pair in `bfdRequestForNeighbor`; the link-local pair in `bfdRequestForNeighborV6`
- RFC 5881 §4 / §5 single-hop bound to one interface, TTL/Hop-Limit 255 GTSM -> `Mode: SingleHop` + the interface name in the request (engine enforces TTL/Hop-Limit); a comment noting the v6 GTSM-255 is independent of base OSPFv3 Hop-Limit-1 (learned 970)
- RFC 5881 §2 one session per data protocol -> the comment explaining a v6 session is distinct from a co-resident v4 session on a dual-stack link
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
| BFD session opened on a usable OSPFv2 adjacency (IPv4 single-hop) | unit + interop | `TestOSPFBFDSessionOpenedOnFull`, `TestBFDRequestForNeighbor`, `ospf-bfd-frr` |
| BFD session opened on a usable OSPFv3 adjacency (IPv6 single-hop, link-local pair) | unit + interop | `TestOSPFv3BFDSessionOpenedOnFull`, `TestBFDRequestForNeighborV6`, `ospfv3-bfd-frr` |
| BFD Down declares the neighbour down faster than RouterDeadInterval (both families) | interop | `ospf-bfd-frr` + `ospfv3-bfd-frr` measure detect time < RouterDeadInterval |
| IPv6 link-local keying distinct from a co-resident OSPFv2 session | unit + interop | `TestOSPFv3BFDDistinctFromV2OnSameLink`, `ospfv3-bfd-frr` dual-stack variant |
| Per-interface enable + BFD timer config (both interface lists) | unit + functional | `TestParseInterfaceBFD`, `TestParseInterfaceBFDv6`, `ospf-bfd-config.ci`, `ospfv3-bfd-config.ci` |
| Down-event path through the existing NSM | unit | `TestOSPFBFDDownDrivesNeighborDown`, `TestOSPFv3BFDDownDrivesNeighborDown` |
| Graceful degradation without the BFD plugin | unit | `TestOSPFBFDGracefulWhenPluginAbsent`, `TestOSPFv3BFDGracefulWhenPluginAbsent` |
| Mirrors the BGP-BFD client wiring on the unified engine | review | structural diff against `peer_bfd.go`; self-containment grep |

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
- [ ] AC-1..AC-16 (incl. AC-1b, AC-2b) all demonstrated on the stated families
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ospf/*`)
- [ ] Integration completeness proven end-to-end (both families)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass)
- [ ] RFC 5880 / RFC 5881 constraint comments added (both families)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (one concrete client copied from `peer_bfd.go`, AF-dispatched only at the request builder)
- [ ] No speculative features (no echo, no multi-hop, no virtual-link BFD)
- [ ] Single responsibility per component (the BFD client only bridges NSM <-> session)
- [ ] Explicit > implicit behavior (opt-in per interface; nil-safe degradation; `IsV6()` dispatch)
- [ ] Minimal coupling (OSPF imports only `bfd/api`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior (both families)
- [ ] Interop tests for protocol features (`ospf-bfd-frr`, `ospfv3-bfd-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-10-bfd.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-10-bfd.md`
