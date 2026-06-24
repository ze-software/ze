# Spec: ospfv3-ext-5 -- BFD for OSPFv3 (RFC 5881, IPv6 single-hop)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospfv3-0-umbrella.md (delivered) |
| Phase | - |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `rfc/short/rfc5881.md` -- single-hop encapsulation: UDP 3784, both ends Active (§3), Hop-Limit = 255 GTSM (§5), one session per data protocol so OSPFv3 (IPv6) is a SEPARATE session from OSPFv2 (§2), Your-Discriminator demux (§6), the IPv6 multi-access addressing rule uses on-subnet src/dst -- for OSPFv3 the src/dst are the link-local pair (§6)
4. `rfc/short/rfc5880.md` -- base BFD: session states Down/Init/Up/AdminDown, Diag codes (1 = detect-expired, 3 = neighbor-signaled-down), slow-start floor (§6.8.3), the failure-detector contract; ze already implements the full engine (`internal/component/bfd`)
5. `internal/component/bgp/reactor/peer_bfd.go` -- the EXEMPLAR client: `bfdClient{mu,svc,handle,sub,stop,done}`, `startBFDClient`/`stopBFDClient`/`runBFDSubscriber`/`bfdRequestFor`; this spec mirrors it for OSPFv3, and `spec-ospf-ext-10-bfd.md` mirrors it for OSPFv2 (same engine, same `internal/plugins/ospf`)
6. `internal/component/bfd/api/service.go` + `events.go` + `registry.go` -- `GetService()`, `Service.EnsureSession(SessionRequest)`, `SessionHandle.Subscribe()/Unsubscribe()`, `Service.ReleaseSession`; `SessionRequest{Peer,Local,Interface,VRF,Mode,DesiredMinTxInterval,RequiredMinRxInterval,DetectMult,...}` with `netip.Addr` fields; `StateChange{Key,State,Diag,When}`; `SingleHop`; `Key` refcount tuple
7. `internal/plugins/ospf/neighbor/table.go` -- `Table.NeighborDown(interfaceName string, id types.RouterID)` is the NSM down-event injection seam (records `kill-nbr`, `setStateLocked(n, stateDown)`, emits the `down` `eventEmission`); `Neighbor.Address netip.Addr` is the neighbor's IPv6 link-local for v3; `AddressOf(id)` and `Lookup(iface,id)` return it
8. `internal/plugins/ospf/instance.go` -- the engine is UNIFIED v2/v3; `e.dispatch.codec.IsV6()` (instance.go:407) selects the OSPFv3 family; `neighborEventSink.NeighborUp/NeighborDown` (instance.go:717-738) is the Full<->non-Full chokepoint; `neighborInterfaceConfig` (instance.go:690) maps `iface.Config` -> `neighbor.InterfaceConfig`; `e.cfg.InstanceID` is the OSPFv3 Instance ID; `e.transport` is the v3 raw IPv6 transport for a v6 engine
9. `internal/plugins/ospf/v3/transport/backend_linux.go` -- `LinkLocalSource() netip.Addr` (backend_linux.go:178) is the interface's IPv6 link-local source; this is the BFD session `Local` address for v3 (`iface.Config.InterfaceAddress [4]byte` is IPv4-only and must NOT be used for the v6 `Local`)
10. `internal/plugins/ospf/config.go` -- `interfaceConfig` struct (config.go:141), `parseInterface` (config.go:633), `ospfConfig` with `InstanceID` and the `V6 *ospfConfig` address-family sub-config (config.go:178-201); the per-interface BFD enable + timer leaves are added here and flow to BOTH the v2 and v6 interface lists
11. `plan/learned/560-bfd-3-bgp-client.md` -- the BGP-BFD client design (atomic-pointer Service lookup, graceful degradation, per-session subscriber goroutine, stop+done handshake)
12. `plan/learned/970-ospfv3-3-ipv6-transport.md` -- user directive: "OSPFv3 is our OSPF; REUSE the `ze_ospf_*` metric series, do NOT fork a `ze_ospfv3_*` namespace"; base OSPFv3 multicast uses Hop-Limit 1 (NOT GTSM 255) -- BFD's GTSM-255 is a SEPARATE single-hop session and does not conflict
13. `plan/learned/564-bfd-2b-ipv6-transport.md` -- the BFD engine's IPv6 single-hop transport (`IPV6_UNICAST_HOPS=255`, `IPV6_RECVHOPLIMIT` cmsg GTSM check); the v6 socket path is already delivered, so an OSPFv3 IPv6 single-hop request is carried end-to-end by the engine (note: this learned summary's `internal/plugins/bfd/...` paths predate the move to `internal/component/bfd/...`)

## Task

Integrate OSPFv3 neighbor liveness with Ze's existing, delivered BFD engine
(`internal/component/bfd`) over IPv6 single-hop (RFC 5881). When an OSPFv3
adjacency reaches a usable state (Full, the point at which the neighbour is
actually carrying flooding), the OSPF engine -- on its IPv6 (OSPFv3) family
instance only (`codec.IsV6()`) -- registers a **single-hop asynchronous** BFD
session for that neighbour keyed on (interface, neighbour link-local address):
`Peer` = the neighbour's IPv6 link-local, `Local` = the interface's IPv6
link-local, `Interface` = the adjacency's interface, `Mode: SingleHop`. RFC 5881
single-hop transmits Hop-Limit 255 (GTSM) and discards on receive != 255; this
is enforced inside the delivered BFD IPv6 transport (learned 564), so OSPFv3 only
selects `Mode: SingleHop`. When BFD reports the session **Down** (or AdminDown),
OSPFv3 injects the NSM "neighbour down" event immediately
(`neighbor.Table.NeighborDown(interface, routerID)`), declaring the neighbour
dead far faster than the RouterDeadInterval timer (typically 40 s) would. The
session is released when the adjacency drops for any other reason, when BFD is
disabled, or when the interface goes down.

The work mirrors the OSPFv2 BFD client (`spec-ospf-ext-10-bfd.md`) and the
BGP-BFD client (`peer_bfd.go`, learned 560) almost one-for-one, substituting the
IPv6 family for IPv4: the session is opened only on the v6 engine instance, the
address pair is the link-local pair (not an IPv4 pair), and the session is a
distinct RFC 5881 IPv6 single-hop session from any co-resident OSPFv2 session on
the same physical link (one BFD session per data protocol, RFC 5881 §2). BFD is
strictly **additive**: if the BFD plugin is not loaded (`api.GetService()`
returns nil), or BFD is not enabled on the interface, OSPFv3 runs exactly as
today on the Hello/Dead timers alone.

### In scope (this spec)

| Item | Detail |
|------|--------|
| v6 BFD session lifecycle tied to NSM | Open an IPv6 single-hop session when a v3 neighbour reaches Full; release it when the neighbour leaves Full / interface goes down / config disables BFD. Hook off `neighborEventSink.NeighborUp` / `NeighborDown` in `instance.go`, gated on `e.dispatch.codec.IsV6()` |
| IPv6 link-local keying | `Peer` = neighbour's IPv6 link-local (`Neighbor.Address`, surfaced in `Snapshot.Address`), `Local` = interface's IPv6 link-local (`v3 transport LinkLocalSource()`); the `Key` tuple (peer, local, interface, vrf, single-hop) is link-local-scoped and distinct from any OSPFv2 IPv4 session on the same link |
| Down-event injection path | A BFD `StateDown`/`StateAdminDown` change drives `Table.NeighborDown(interfaceName, neighborRouterID)` -- the SAME seam `nsmAdapter.NeighborDown` already uses -- so the neighbour drops to Down through the existing NSM, not a new code path |
| Per-interface config surface | A `bfd` container on the OSPF `interface` list under BOTH the IPv4 interface list and the `address-family ipv6` interface list: `enabled`, `min-tx`, `min-rx`, `multiplier` leaves; resolved into `interfaceConfig` -> `iface.Config` -> `neighbor.InterfaceConfig` and consumed by the v6 engine when opening sessions |
| Single-hop session request | `api.SessionRequest{Peer: neighbour link-local, Local: interface link-local, Interface: ifname, Mode: SingleHop, DesiredMinTxInterval, RequiredMinRxInterval, DetectMult}` built from the neighbour Snapshot/record + the interface BFD config |
| In-process Service lookup | Reuse `api.GetService()` (already published by the BFD plugin's `OnStarted`); nil-safe graceful degradation identical to BGP and OSPFv2 |
| Per-session subscriber goroutine | One long-lived worker per BFD-protected neighbour draining `<-chan StateChange`; translates Down -> `NeighborDown`; goroutine-lifecycle compliant, mirrors `runBFDSubscriber` (stop+done handshake) |
| Metrics | `ze_ospf_bfd_sessions` gauge, `ze_ospf_bfd_session_down_total` counter, `ze_ospf_bfd_register_failures_total` counter (the unified `ze_ospf_*` namespace per learned 970; the `interface` label distinguishes v2 from v3) |
| CLI surface | BFD enable + session state visible via the existing `show ipv6 ospf neighbor` / `show ipv6 ospf interface` outputs (a `bfd` column / flag), no new top-level command |

### Out of scope (noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| BFD echo function (RFC 5880 §6.4) | BFD engine concern; `spec-bfd-6-echo-mode` (delivered). OSPFv3 does not request echo (`DesiredMinEchoTxInterval` left zero) |
| BFD multi-hop (RFC 5883) | engine concern; OSPFv3 adjacencies are single-hop by definition (directly-connected link-local neighbours). The request is always `Mode: SingleHop`. Virtual links (ext-3, multi-hop) are not BFD-protected here |
| BFD authentication wiring | engine concern; `spec-bfd-5-authentication` (delivered). OSPFv3 sessions inherit no auth in this spec (`SessionRequest.Auth` left nil) |
| BFD timer negotiation, Poll/Final, slow-start | all inside `internal/component/bfd` (delivered); OSPFv3 only supplies desired min-tx / min-rx / multiplier and never touches the wire |
| OSPFv2 BFD (the IPv4 family) | `spec-ospf-ext-10-bfd.md` (sibling) covers the IPv4 family on the same unified engine; this spec is the IPv6 (OSPFv3) family. The config plumbing and the per-neighbour client map are shared by both families; this spec adds the v6-specific request builder (link-local source) and the `IsV6()` gate |
| OSPFv3 BFD on an adjacency that never reaches Full (2-Way DROther pairs on a broadcast LAN) | by design only Full adjacencies (the ones carrying flooding) get a BFD session; 2-Way-only neighbours do not |
| GTSM-255 vs base-OSPFv3 Hop-Limit-1 conflict | none: base OSPFv3 multicast Hellos use Hop-Limit 1 (learned 970), BFD single-hop is a SEPARATE unicast session that uses Hop-Limit 255; the two coexist on the same link without interaction (engine owns the BFD wire) |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
- [ ] `docs/research/ospf-implementation-guide.md` lines 1542-1545 ("BFD Integration (RFC 5880, RFC 5881)") -- "Implemented as a thin wrapper around zebra's BFD subsystem. BFD failure triggers an NSM event that declares the neighbour down immediately."
  -> Decision: model BFD-for-OSPFv3 as a thin client over the EXISTING `internal/component/bfd` engine, exactly as FRR's `ospf6d` wraps zebra's BFD subsystem; no new BFD logic, only the NSM <-> session glue
  -> Constraint: the failure action is "an NSM event that declares the neighbour down immediately" -- i.e. drive `Table.NeighborDown`, never invent a parallel teardown
- [ ] `plan/spec-ospf-ext-10-bfd.md` -- the OSPFv2 BFD sibling on the SAME unified engine
  -> Decision: share the per-neighbour `bfdClient` map and the start/stop/subscriber/release lifecycle with the OSPFv2 client; the only v3-specific divergence is the request builder (link-local `Local` source from the v3 transport, not the `[4]byte` IPv4 `InterfaceAddress`) and the `IsV6()` gate
  -> Constraint: a v2 and a v3 session on the same physical link are DISTINCT BFD sessions (RFC 5881 §2 one-per-protocol); their `Key` tuples differ because the address pair differs (IPv4 vs link-local), so refcounting keeps them independent
- [ ] `plan/learned/560-bfd-3-bgp-client.md` -- the BGP-BFD client this lifecycle mirrors
  -> Constraint: graceful degradation is mandatory -- a missing BFD plugin (`GetService()==nil`) MUST NOT block OSPFv3; the adjacency runs on Hello/Dead timers alone and logs a warning once
  -> Constraint: the subscriber is a long-lived per-session worker (one per BFD-protected neighbour), not per-event; `stopBFDSession` closes a stop chan and waits on a done chan so the goroutine has exited before the handle is released
- [ ] `plan/learned/970-ospfv3-3-ipv6-transport.md` -- OSPFv3 transport + the metric-naming directive
  -> Constraint: use the UNIFIED `ze_ospf_bfd_*` metric namespace (NOT `ze_ospfv3_bfd_*`); the metrics registry is get-or-create by name, so v2 and v3 share one series and the `interface` label distinguishes them. (The `spec-ospfv3-ext-0-umbrella.md` "Metrics" text still says `ze_ospfv3_<ext>_*`; that predates the directive in learned 970 and the umbrella mapping must be updated to `ze_ospf_bfd_*` when this spec lands.)
  -> Constraint: base OSPFv3 multicast is Hop-Limit 1, NOT GTSM 255; this is a base-transport rule and does not apply to the BFD session, which is single-hop unicast at Hop-Limit 255 enforced by the BFD engine
- [ ] `plan/learned/564-bfd-2b-ipv6-transport.md` -- the BFD IPv6 single-hop transport
  -> Constraint: the BFD engine already carries an IPv6 single-hop session end-to-end (`IPV6_UNICAST_HOPS=255` on TX, `IPV6_RECVHOPLIMIT` cmsg GTSM check on RX). OSPFv3 supplies `netip.Addr` link-local peer/local in the request; the engine selects the v6 socket by address family. OSPFv3 adds NO transport code
- [ ] `ai/rules/plugin-self-containment.md` -- removing the OSPF plugin removes all its BFD wiring
  -> Constraint: all BFD-for-OSPFv3 code lives under `internal/plugins/ospf`; the only outside dependency is `internal/component/bfd/api` (a leaf package). No OSPF spelling appears in `internal/component/bfd`
- [ ] `ai/rules/no-sprintf-alloc.md` -- no `fmt`/`+` on any hot path
  -> Constraint: the per-packet BFD path is entirely inside the BFD engine; OSPFv3's only frequency is per-NSM-transition (rare). Session-state log lines use structured slog fields, never `fmt.Sprintf` string building

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5881.md` -- BFD for IPv4/IPv6 single hop
  -> Constraint: §3 both ends MUST take the Active role; OSPFv3 supplies `Passive: false` (the api default) so the session comes up symmetrically with FRR `ospf6d`/`bfdd`
  -> Constraint: §2 a separate BFD session MUST exist per data protocol over a link; OSPFv3 (IPv6) opens an IPv6 single-hop session keyed on (peer link-local, local link-local, interface, vrf, single-hop); a co-resident OSPFv2 (IPv4) session is independent
  -> Constraint: §5 single-hop sessions transmit Hop-Limit = 255 and discard received packets with Hop-Limit != 255 (GTSM). Enforced inside `internal/component/bfd` (SingleHop mode, learned 564); OSPFv3 only selects `Mode: SingleHop`
  -> Constraint: §6 multi-access addressing uses on-subnet src/dst; for OSPFv3 the on-link addresses are the link-local pair. `Peer` = neighbour link-local, `Local` = interface link-local; the session is bound to ONE egress interface (`SessionRequest.Interface`), matching the interface the Hello arrived on
- [ ] `rfc/short/rfc5880.md` -- base BFD protocol
  -> Constraint: §6.8.1 the session declares Down with Diag 1 (Control Detection Time Expired) on timer miss and Diag 3 (Neighbor Signaled Session Down) when the peer reports Down; OSPFv3 treats BOTH `StateDown` and `StateAdminDown` as "neighbour down" regardless of Diag, matching the BGP and OSPFv2 clients
  -> Constraint: §6.8.3 slow-start floors DesiredMinTxInterval at 1 s until the session reaches Up; OSPFv3's configured `min-tx` only takes effect after Up. A freshly-formed adjacency therefore detects in up to `multiplier * 1 s` until fast rates negotiate -- this is the engine's contract, not an OSPFv3 bug
  -> Constraint: BFD is a "failure detector, not a session driver" (RFC 5882 client model): OSPFv3 acts ONLY on Down. A BFD `Up`/`Init` transition is logged at debug and does NOT itself bring the OSPFv3 adjacency up (the NSM owns that)

**Key insights:** (minimal context to resume after compaction)
- BFD-for-OSPFv3 is glue, not protocol: the engine (incl. the IPv6 single-hop transport) is delivered; this spec wires NSM Full -> EnsureSession and BFD Down -> `Table.NeighborDown`, gated on `codec.IsV6()`.
- The down-injection seam already exists: `Table.NeighborDown(interfaceName, id)` is called today by `nsmAdapter.NeighborDown`; the BFD subscriber calls the same `Table` method.
- The exemplars are `peer_bfd.go` (BGP) and `spec-ospf-ext-10-bfd.md` (OSPFv2 on the same engine); copy the lifecycle (start/stop/subscriber/request-builder, nil-safe degradation, mutex-guarded per-session state, stop+done handshake). The ONLY v3 divergences: the link-local `Local` source and the `IsV6()` gate.
- Only Full adjacencies get a session; only Down/AdminDown drives a teardown. Use `ze_ospf_*` metrics, not `ze_ospfv3_*`.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/plugins/ospf/neighbor/table.go` -- `Table` owns the per-interface neighbour map keyed by `(iface, RouterID)`; `NeighborDown(interfaceName, id)` looks up the neighbour, records `kill-nbr`, and `setStateLocked(n, stateDown)`; `setStateLocked` returns a `down` `eventEmission` when a Full neighbour drops, which `emit` forwards to `EventSink.NeighborDown`. `Neighbor.Address netip.Addr` is the reachable source (IPv6 link-local for v3); `AddressOf(id)` and `Lookup(iface, id)` return it. `Snapshot.Address` is the STRING form
  -> Constraint: `NeighborDown` is the EXACT injection point; the BFD subscriber calls it with the neighbour's interface + Router ID. No new NSM event or state is needed. It is idempotent (no-op if the neighbour is absent or already Down)
  -> Constraint: the engine needs the raw `netip.Addr` (`Neighbor.Address`) for the BFD `Peer`, not the `Snapshot.Address` string; prefer a typed lifecycle callback carrying the raw address, or `Table.Lookup` + a typed accessor, so an IPv6 link-local zone is never lost to a string round-trip (R-2)
- [ ] `internal/plugins/ospf/neighbor/neighbor.go` -- `Snapshot{Interface, Area, RouterID, State, Address(string), ...}`; `Neighbor.Address netip.Addr` (the IPv6 link-local for v3, documented as such); `EventSink{NeighborUp(Snapshot), NeighborDown(Snapshot)}`; the `state` enum (`stateFull` is the usable state); `InterfaceConfig` (no BFD fields yet)
  -> Constraint: `EventSink` carries only a `Snapshot` (string address). To build a `SessionRequest` with a raw `netip.Addr` the lifecycle must obtain `Neighbor.Address` -- widen the event path with a typed callback, or look up via `Table.Lookup`/`AddressOf` and reparse. The spec prefers a typed lifecycle callback to avoid the string round-trip
  -> Constraint: `InterfaceConfig` gains a BFD sub-struct (enabled + timers) so the engine can decide whether to open a session for a neighbour on that interface
- [ ] `internal/plugins/ospf/instance.go` -- the engine is UNIFIED v2/v3 (`newEngineWithCodec`); `e.dispatch.codec.IsV6()` (instance.go:407) is true for the OSPFv3 family; `e.neighbors.SetEventSink(neighborEventSink{sink: e.sink, onChange: e.originateSelfLSAs})` (instance.go:411); `neighborEventSink.NeighborUp/NeighborDown` (instance.go:717-738) is the single Full<->non-Full chokepoint; `neighborInterfaceConfig(cfg)` (instance.go:690) maps `iface.Config` -> `neighbor.InterfaceConfig`; `e.cfg.InstanceID` is the OSPFv3 Instance ID; `e.transport` is the v3 transport for a v6 engine
  -> Constraint: `neighborEventSink` is the lifecycle hook (open on up, close on down); it must learn whether the engine is v6 (`codec.IsV6()`), the neighbour's raw `netip.Addr`, the interface, and the per-interface BFD config to open/close sessions
  -> Constraint: the BFD lifecycle is gated on `codec.IsV6()` for THIS spec; the OSPFv2 sibling (ext-10) gates the same hook on `!IsV6()`. The per-neighbour `bfdClient` map and the start/stop helpers are shared; only `bfdRequestForNeighbor` differs by family
- [ ] `internal/plugins/ospf/v3/transport/backend_linux.go` -- `interfaceLinkLocal(name)` resolves the interface's IPv6 link-local source (DAD-complete preferred, else `ErrNoLinkLocal`); `linuxInterface.LinkLocalSource() netip.Addr` (backend_linux.go:178) exposes it; the RX/TX path binds `ControlMessage.Src` to this link-local
  -> Constraint: `LinkLocalSource()` is the BFD session `Local` address for v3. `iface.Config.InterfaceAddress` is `[4]byte` (IPv4-only) and MUST NOT be used for the v6 `Local`. The engine reads the per-interface link-local source from the v3 transport (a typed accessor on the v3 transport/iface), falling back to leaving `Local` zero if not yet available (the engine then lets the kernel choose, consistent with the slow-start window)
  -> Constraint: the link-local source can lag link-up (DAD ~1 s); opening a BFD session before the source resolves leaves `Local` zero or defers the open until the neighbour reaches Full (by which point the local link-local exists, since Hellos have flowed). The open path tolerates a zero `Local`
- [ ] `internal/plugins/ospf/config.go` -- `interfaceConfig` struct (config.go:141: Name, AreaID, Enabled, NetworkType, Cost, HelloInterval, DeadInterval, ...); `parseInterface(entry)` (config.go:633) reads `hello-interval`/`dead-interval`/`retransmit-interval` via `configNumber`; `ospfConfig` carries `InstanceID` and the `V6 *ospfConfig` address-family sub-config (config.go:178-201) parsed from `address-family ipv6`
  -> Constraint: add a `BFD bfdInterfaceConfig` field to `interfaceConfig` and parse a nested `bfd` map in `parseInterface`; default disabled (opt-in). `parseInterface` is shared by the IPv4 list and the `address-family ipv6` interface list, so the BFD block parses for BOTH families from one code path
- [ ] `internal/plugins/ospf/iface/iface.go` -- `Config` struct (iface.go:70: Name, AreaID, NetworkType, InterfaceAddress[4]byte, HelloInterval, DeadInterval, IsV6, InterfaceID, ...) is the per-interface runtime config the FSM/Hello layer consumes; `IsV6` marks an OSPFv3 interface
  -> Constraint: the BFD enable + timers must ride through `iface.Config` so the engine sees them when it opens a neighbour session; add the fields and map them in `interfaceRuntimeConfigLocked` / `neighborInterfaceConfig`. The link-local `Local` source is NOT stored here (it comes from the live v3 transport, which tracks DAD)
- [ ] `internal/component/bfd/api/service.go`, `events.go`, `registry.go` -- `GetService() Service` (registry.go:62); `Service.EnsureSession(SessionRequest) (SessionHandle, error)`; `Service.ReleaseSession(SessionHandle) error`; `SessionHandle.Subscribe() <-chan StateChange` / `Unsubscribe`; `SessionRequest{Peer netip.Addr, Local netip.Addr, Interface, VRF, Mode, DesiredMinTxInterval, RequiredMinRxInterval, DetectMult, ...}`; `StateChange{Key, State, Diag, When}`; `StateDown`/`StateAdminDown`; `SingleHop`; `Key{Peer, Local, Interface, VRF, Mode}`
  -> Constraint: this is the FROZEN client contract BGP and OSPFv2 already use (learned 560 "Service interface surface is frozen for this path"); OSPFv3 reuses it verbatim, adding NO methods to `api`. `SessionRequest.Peer`/`Local` are `netip.Addr`, so IPv6 link-locals pass through natively
  -> Constraint: `EnsureSession` refcounts on the `Key` tuple; a v3 (link-local) key and a v2 (IPv4) key on the same interface differ, so each adjacency gets its own session even on a dual-stack link
- [ ] `internal/component/bgp/reactor/peer_bfd.go` -- the EXEMPLAR: `bfdClient{mu, svc, handle, sub, stop, done}`; `startBFDClient` (nil-safe GetService, EnsureSession, Subscribe, spawn `runBFDSubscriber`); `runBFDSubscriber` (drain sub; Down/AdminDown -> `Teardown`); `stopBFDClient` (close stop, Unsubscribe, ReleaseSession, wait done); `bfdRequestFor(settings)` builds the request
  -> Constraint: copy this structure verbatim, substituting OSPFv3 types: per-neighbour `bfdClient` keyed by (interface, Router ID); `Teardown(...)` -> `table.NeighborDown(iface, id)`; `bfdRequestFor(settings)` -> `bfdRequestForNeighborV6(addr, localLinkLocal, ifname, ifcfg)`
- [ ] `internal/plugins/ospf/register.go` -- `registerOSPF()` registers the namespace, doctor, diagnostic codes, completions, config sections, the existing `ze_ospf_*` metric set; the v6 engine instance is registered alongside the v4 instance
  -> Constraint: the new `ze_ospf_bfd_*` metrics register here ONCE (shared by v2 and v3 via get-or-create); the OSPF doctor gains an informational check that BFD is configured-but-plugin-absent

**Behavior to preserve:**
- OSPFv3 on Hello/Dead timers alone when BFD is not enabled or the BFD plugin is absent (additive-only contract). Every existing OSPFv3 functional/interop test must stay green with no config change.
- The NSM state machine and `Table.NeighborDown` semantics (a BFD-driven down is indistinguishable from an inactivity-timer-driven down to the rest of OSPFv3 -- same `kill-nbr` event, same LSA re-origination, same SPF re-run).
- `neighborEventSink.NeighborUp/NeighborDown` continuing to re-originate self-LSAs and emit report-bus events; the BFD lifecycle is layered ON TOP, not in place of, these.
- Base OSPFv3 multicast transport at Hop-Limit 1 (learned 970); BFD's separate Hop-Limit-255 single-hop session does not change the base transport.
- The frozen `internal/component/bfd/api` surface (no new methods).
- The OSPFv2 BFD sibling (ext-10): the shared `bfdClient` map and lifecycle helpers must keep working for the IPv4 family.

**Behavior to change:** (only what the task requires)
- `interfaceConfig` / `iface.Config` / `neighbor.InterfaceConfig` gain BFD fields (enabled + timers); the parse path serves both the IPv4 and `address-family ipv6` interface lists.
- `neighborEventSink` (or an equivalent lifecycle observer) opens a v6 BFD session on Full (when `codec.IsV6()`) and releases it on leaving Full.
- A BFD Down transition now drives `Table.NeighborDown` for the v3 neighbour (a new, faster trigger for an existing transition).
- `show ipv6 ospf neighbor` / `show ipv6 ospf interface` surface BFD session state (additive columns).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Config:** `ospf { address-family ipv6 { interface eth0 { bfd { enabled true; min-tx 50; min-rx 50; multiplier 3 } } } }` enters via the OSPF config sections -> `parseInterface` -> `interfaceConfig.BFD` on the v6 sub-config.
- **Adjacency up:** a v3 neighbour reaches Full -> `Table.setStateLocked` emits `up` -> `emit` -> `neighborEventSink.NeighborUp(snap)` -> (engine is v6) BFD lifecycle opens an IPv6 single-hop session.
- **BFD down:** the BFD engine declares the session Down -> `StateChange{State: StateDown}` on the subscription channel -> subscriber -> `Table.NeighborDown(interface, routerID)`.
- **Adjacency down (any other cause):** `neighborEventSink.NeighborDown(snap)` -> BFD lifecycle releases the session.

### Transformation Path
1. **Config parse:** `parseInterface` reads the nested `bfd` map (`enabled`, `min-tx`, `min-rx`, `multiplier`) into `interfaceConfig.BFD` (defaults: disabled; min-tx/min-rx 50 000 us; multiplier 3, matching common practice and the OSPFv2/BGP defaults). Shared by the IPv4 and IPv6-family interface lists.
2. **Config flow:** `interfaceConfig.BFD` -> `iface.Config.BFD` (via `interfaceRuntimeConfigLocked`) -> `neighbor.InterfaceConfig.BFD` (via `neighborInterfaceConfig`). The engine retains a per-interface BFD config map.
3. **Session open (Full, v6):** on `NeighborUp(snap)`, if the engine is v6 (`codec.IsV6()`) AND `snap` is on a BFD-enabled interface AND `api.GetService() != nil`, build `bfdRequestForNeighborV6(neighbourLinkLocal, interfaceLinkLocal, ifname, ifcfg)` = `SessionRequest{Peer: neighbourLinkLocal, Local: interfaceLinkLocal, Interface: ifname, Mode: SingleHop, DesiredMinTxInterval: minTx, RequiredMinRxInterval: minRx, DetectMult: multiplier}`, call `EnsureSession`, `Subscribe`, spawn `runBFDSubscriber`. Store the per-neighbour `bfdClient` keyed by (interface, Router ID).
4. **Subscriber loop:** drains `<-chan StateChange`. On `StateDown`/`StateAdminDown`: log a warning, increment `ze_ospf_bfd_session_down_total`, call `Table.NeighborDown(interface, routerID)`. On Up/Init: log at debug. Exits on stop-chan or channel close.
5. **NSM down:** `Table.NeighborDown` runs the existing path: `kill-nbr` event, neighbour -> Down, `down` emission -> `neighborEventSink.NeighborDown` (which ALSO releases the BFD session in step 6, idempotently) -> self-LSA re-origination -> SPF re-run -> IPv6 route withdrawal via Loc-RIB.
6. **Session release:** on `NeighborDown(snap)` (whether BFD-driven or timer-driven), the lifecycle looks up the per-neighbour `bfdClient`, closes its stop chan, `Unsubscribe`s, `ReleaseSession`s, waits the done chan, and forgets it. Idempotent: a no-op if no session was open.
7. **Interface down / config disable:** `InterfaceDown` / a reload that clears `bfd.enabled` releases every session on that interface through the same release path.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| OSPF config <-> interface config | nested `bfd` map parsed in `parseInterface` into `interfaceConfig.BFD`; flows to `iface.Config` + `neighbor.InterfaceConfig` for both families | [ ] |
| NSM Full transition <-> BFD lifecycle (v6 gate) | `neighborEventSink.NeighborUp/NeighborDown` open/close sessions when `codec.IsV6()`; needs the raw neighbour `netip.Addr` (link-local) + the interface link-local source + interface name | [ ] |
| OSPFv3 engine <-> BFD engine | `api.GetService().EnsureSession/ReleaseSession`; `SessionHandle.Subscribe/Unsubscribe`; value-typed `SessionRequest`/`StateChange` with `netip.Addr` link-locals (no cross-boundary pointers) | [ ] |
| BFD Down <-> NSM down | subscriber calls `Table.NeighborDown(interface, routerID)` -- the existing injection seam | [ ] |
| v3 transport <-> BFD request | the interface's IPv6 link-local source (`LinkLocalSource()`) becomes the BFD `Local`; the engine reads it from the live v3 transport | [ ] |
| Engine <-> Service availability | `GetService()` nil-check; OSPFv3 degrades to timer-only and logs once | [ ] |

### Integration Points
- `internal/plugins/ospf/config.go` -- `interfaceConfig.BFD`, `parseInterface` (consumes the YANG `bfd` container on both interface lists).
- `internal/plugins/ospf/iface/iface.go` -- `Config.BFD` (carries enable + timers to the runtime).
- `internal/plugins/ospf/neighbor` -- `InterfaceConfig.BFD`; the up/down lifecycle needs the neighbour's raw `netip.Addr` (link-local) + interface; `NeighborDown` is the down seam (consumed, not changed).
- `internal/plugins/ospf/instance.go` -- the BFD lifecycle observer (open on Full when `codec.IsV6()`, close on down); reads the per-interface BFD config + the v3 transport link-local source; owns the per-neighbour `bfdClient` map (shared with the OSPFv2 sibling).
- `internal/plugins/ospf/v3/transport/` -- the `LinkLocalSource()` accessor for the interface's IPv6 link-local (consumed, not changed; expose a typed engine-facing accessor if not already reachable).
- `internal/component/bfd/api` -- `GetService`, `Service`, `SessionHandle`, `SessionRequest`, `StateChange` (consumed verbatim; frozen surface).
- `internal/plugins/ospf/register.go` -- the three `ze_ospf_bfd_*` metrics; doctor informational check.

### Architectural Verification
- [ ] No bypassed layers (BFD down flows through `Table.NeighborDown` and the existing NSM, not a side path)
- [ ] No unintended coupling (OSPF imports only `bfd/api`; `internal/component/bfd` names no OSPF symbol)
- [ ] No duplicated functionality (reuses the delivered BFD engine + IPv6 transport, the `api.Service` lookup, the existing NSM down seam, and the OSPFv2 client lifecycle; only the v6 request builder + `IsV6()` gate are new)
- [ ] Zero-copy preserved where applicable (value-typed `SessionRequest`/`StateChange` with `netip.Addr`; no per-packet OSPFv3 involvement)

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `api.GetService()` + `Service.EnsureSession`/`ReleaseSession`/`SessionHandle.Subscribe` is a frozen, in-process client contract OSPFv3 can reuse exactly as BGP and OSPFv2 do | `internal/component/bfd/api/service.go`, `events.go`, `registry.go`; learned 560 "Service interface surface is frozen" | OSPFv3 needs new api methods, widening the BFD surface | `TestOSPFv3BFDSessionOpenedOnFull` against a fake `api.Service` (mirrors `peer_bfd_test.go`) | unvalidated |
| A-2 | `Table.NeighborDown(interfaceName, id)` is a safe, idempotent NSM down injection producing the same downstream effects (kill-nbr, LSA re-origination, SPF) as an inactivity-timer expiry, for the v3 family too | `neighbor/table.go` `neighborDown`; `nsmAdapter.NeighborDown` already calls it on the same unified table | a BFD-driven down behaves differently from a timer down for v3 | `TestOSPFv3BFDDownDrivesNeighborDown` asserts the v3 neighbour goes Down and an LSA re-origination fires | unvalidated |
| A-3 | The neighbour's IPv6 link-local (`Neighbor.Address`) is the correct single-hop BFD `Peer`, and the interface's own IPv6 link-local (`v3 transport LinkLocalSource()`) is the `Local`; `iface.Config.InterfaceAddress [4]byte` is IPv4-only and is NOT used | `neighbor.go` `Neighbor.Address` doc ("IPv6 link-local for OSPFv3"); `v3/transport/backend_linux.go:178`; RFC 5881 §6 on-subnet src/dst | the session binds to the wrong address pair (or an IPv4 address) and never comes Up against FRR `bfdd` | `ospfv3-bfd-frr` interop forms a BFD Up session; `TestBFDRequestForNeighborV6` pins the link-local pair | unvalidated |
| A-4 | Only Full adjacencies need a BFD session; opening at Full (not at 2-Way / Exchange) is correct and matches FRR `ospf6d` | RFC 5340 (Full = adjacency carrying flooding); guide 1542-1545; `setStateLocked` emits up only on `next == stateFull` | sessions churn during ExStart/Exchange, or a half-formed adjacency is unprotected | `TestOSPFv3BFDOnlyAtFull` asserts no session before Full; `ospfv3-bfd-frr` interop | unvalidated |
| A-5 | An IPv6 single-hop session (`Mode: SingleHop`, Hop-Limit 255) is always correct for OSPFv3 neighbours (directly connected, link-local) | RFC 5881 §1 single-hop = directly-connected; OSPFv3 adjacencies are link-local | a virtual-link neighbour (multi-hop, ext-3) is mis-protected | OSPFv3 here forms only normal/stub/NSSA adjacencies (no virtual links in scope); documented in Known Limitations | unvalidated |
| A-6 | The lifecycle hook can read the engine's address family (`e.dispatch.codec.IsV6()`) so the v6 client opens v6 sessions and the v2 client (ext-10) opens v4 sessions, without cross-firing | `instance.go:407` `e.dispatch.codec.IsV6()` already used to install the v6 encoder | a v6 engine opens an IPv4 session (or vice versa); the address pair is wrong | `TestOSPFv3BFDUsesLinkLocalPair` (v6 engine) + the v2 sibling's `IsV6()==false` test | unvalidated |
| A-7 | The BFD engine's IPv6 single-hop transport (learned 564) carries a link-local peer/local request end-to-end (v6 socket, Hop-Limit 255, GTSM RX check) with NO OSPFv3 transport code | `plan/learned/564-bfd-2b-ipv6-transport.md`; `internal/component/bfd/transport/udp_linux.go` (`IPV6_UNICAST_HOPS`, `IPV6_RECVHOPLIMIT`) | the engine cannot open a v6 session; OSPFv3 would need to touch the BFD transport | `ospfv3-bfd-frr` interop brings a v6 session Up; the engine's existing v6 transport tests already cover the socket path | unvalidated |
| A-8 | The interface's IPv6 link-local source exists by the time a neighbour reaches Full (Hellos have flowed, so DAD completed); a zero `Local` is tolerated by the engine (kernel source selection) as a fallback | `v3/transport/backend_linux.go` DAD handling; learned 970 (link-local can lag link-up ~1 s, but adjacency formation outlasts DAD) | the session opens with a zero/wrong `Local` and fails verification at the peer | `TestBFDRequestForNeighborV6` pins the resolved link-local; `ospfv3-bfd-frr` interop confirms Up | unvalidated |
| A-9 | `EnsureSession` refcounting on the `Key` tuple keeps a v3 (link-local) and a v2 (IPv4) session on the SAME physical link independent (RFC 5881 §2 one-per-protocol) | `api/events.go` `Key{Peer,Local,Interface,VRF,Mode}`; the address pair differs by family | a v2 and a v3 session on a dual-stack link collide / one release tears down the other | `TestOSPFv3BFDDistinctFromV2OnSameLink` (distinct keys); the engine's refcount tests | unvalidated |
| A-10 | A reload that flips `bfd.enabled` on a v6 interface can open/close sessions for already-Full neighbours without re-forming the adjacency | the reload path in `instance.go`; OSPFv2 (ext-10) does the equivalent on the same engine | enabling BFD mid-adjacency requires a neighbour bounce (operator-visible churn) | `TestOSPFv3BFDReloadEnablesSessionForFullNeighbor` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A BFD Down races a concurrent NSM timer-down for the same v3 neighbour, double-firing `NeighborDown` | duplicate `kill-nbr` metric increments; a panic on a freed neighbour | `Table.NeighborDown` is idempotent (looks up; no-op if absent / already Down); the subscriber and release path both tolerate "already gone"; `TestOSPFv3BFDDownIdempotentWithTimerDown` |
| R-2 | The session-open path needs the neighbour's raw `netip.Addr` (IPv6 link-local), but `Snapshot.Address` is a string -- a parse round-trip could lose the IPv6 zone or fail | a malformed `Peer` address; session never comes Up | carry the raw `netip.Addr` (`Neighbor.Address`) in a typed lifecycle callback, not via the string Snapshot; `TestBFDRequestForNeighborV6` pins the exact `netip.Addr` including zone |
| R-3 | A subscriber goroutine leaks if `stopBFDSession` does not wait the done chan, or if `ReleaseSession` closes the channel without the subscriber noticing | goroutine count grows per adjacency flap; `go test -race` / leak check fails | copy the `peer_bfd.go` stop+done handshake verbatim; `TestOSPFv3BFDSubscriberExitsOnRelease`; run the OSPF suite under `-race` |
| R-4 | The shared `bfdClient` map cross-fires between the v2 and v3 engine instances (two engines, one map) | a v6 down tears a v4 neighbour; map key collision | the per-neighbour map is per-engine-instance (each `engine` owns its own map); `(iface, RouterID)` keys are scoped to one instance; `TestOSPFv3BFDClientMapIsolatedPerEngine` |
| R-5 | OSPFv3 and OSPFv2 both open a single-hop session on the same link with different timers; the engine picks the more aggressive value, surprising one family | a BFD session runs faster/slower than one family configured | distinct `Key` tuples per family (different address pair) mean NO coalescing -- each family's timers apply to its own session; `ze_ospf_bfd_sessions` (interface label) reflects both; documented |
| R-6 | A BFD plugin shutdown (Service set to nil) while OSPFv3 sessions are live leaves dangling handles | `ReleaseSession` after plugin teardown; subscriber sees a closed channel | the subscriber exits on channel close; `ReleaseSession` on a torn-down loop is a documented no-op (learned 560 gotcha) |
| R-7 | The `bfd` config container under `address-family ipv6 interface` collides with / shadows the IPv4 `bfd` block, the top-level `bfd { }` plugin config, or BGP's `bfd` | parse error or wrong handler claims the section | the OSPF `bfd` leaf lives strictly under `ospf [address-family ipv6] interface`; YANG namespacing keeps it distinct; functional test `ospfv3-bfd-config.ci` proves coexistence with the v2 and top-level blocks |
| R-8 | Min-tx/min-rx of 0 (or absurdly small) is accepted and produces an unusable session | session never stabilises; CPU spin | YANG `range` validation on the timer leaves (10..255000 us echoing BFD's bounds); `parseInterface` rejects 0; boundary tests |
| R-9 | The interface's IPv6 link-local source is not yet available (DAD) when the open is attempted, producing a zero `Local` and a session that never verifies at the peer | session stuck in Down; FRR `bfdd` drops on source mismatch | open at Full (DAD long complete by then); tolerate zero `Local` (kernel selects) as a fallback; `ospfv3-bfd-frr` retries until Up; documented in Known Limitations |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ospf address-family ipv6 { interface eth0 { bfd { enabled true } } }` config | -> | `parseInterface` sets `interfaceConfig.BFD.Enabled`; flows to `neighbor.InterfaceConfig` on the v6 sub-config | `TestParseInterfaceBFDv6` (unit) + `test/ospfv3/ospfv3-bfd-config.ci` |
| A v3 neighbour reaches Full on a BFD-enabled interface (v6 engine) | -> | `neighborEventSink.NeighborUp` (gated `codec.IsV6()`) -> BFD lifecycle -> `api.Service.EnsureSession` (link-local pair, SingleHop) + `Subscribe` + subscriber spawned | `TestOSPFv3BFDSessionOpenedOnFull` (unit, fake Service) + `test/ospfv3/ospfv3-bfd-session.ci` |
| BFD reports `StateDown` for a protected v3 neighbour | -> | subscriber -> `Table.NeighborDown(interface, routerID)` -> v3 neighbour to Down | `TestOSPFv3BFDDownDrivesNeighborDown` (unit) + `ospfv3-bfd-frr` interop |
| The v3 neighbour leaves Full (timer, interface down, reset) | -> | `neighborEventSink.NeighborDown` -> BFD lifecycle release (`Unsubscribe` + `ReleaseSession`) | `TestOSPFv3BFDSessionReleasedOnDown` (unit) |
| BFD plugin not loaded (`GetService()==nil`) on a BFD-enabled v6 interface | -> | lifecycle logs a warning once and runs without BFD | `TestOSPFv3BFDGracefulWhenPluginAbsent` (unit) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ospf address-family ipv6 { interface eth0 { bfd { enabled true; min-tx 50; min-rx 50; multiplier 3 } } }` | parsed into `interfaceConfig.BFD{Enabled:true, MinTxUs:50000, MinRxUs:50000, Multiplier:3}` on the v6 sub-config; surfaced in `show ipv6 ospf interface` as BFD enabled |
| AC-2 | A v3 neighbour on a BFD-enabled interface transitions to Full, BFD plugin loaded, engine is v6 | exactly one IPv6 single-hop `api.SessionRequest` (Mode SingleHop, Peer = neighbour link-local, Local = interface link-local, Interface = ifname, timers from config) is sent to `EnsureSession`; a subscriber goroutine is running |
| AC-3 | A v3 neighbour on an interface WITHOUT BFD enabled reaches Full | no `EnsureSession` call; OSPFv3 runs on timers alone |
| AC-4 | BFD plugin not loaded (`GetService()==nil`), interface BFD-enabled, v3 neighbour reaches Full | no session opened; a single warning logged; `ze_ospf_bfd_register_failures_total` incremented; OSPFv3 unaffected |
| AC-5 | A protected v3 session reports `StateDown` (Diag 1, detect-expired) | `Table.NeighborDown(interface, routerID)` invoked; the neighbour drops to Down; `ze_ospf_bfd_session_down_total` incremented; self-LSAs re-originated and SPF re-runs (IPv6 route withdrawn) |
| AC-6 | A protected v3 session reports `StateAdminDown` | treated identically to Down (neighbour declared down) |
| AC-7 | A protected v3 session reports `StateUp` / `StateInit` | logged at debug; OSPFv3 adjacency state unchanged (BFD is a failure detector, not a session driver) |
| AC-8 | A protected v3 neighbour leaves Full for any reason (inactivity timer, interface down, `clear ipv6 ospf neighbor`) | the BFD session is released (`Unsubscribe` + `ReleaseSession`); the subscriber goroutine exits; `ze_ospf_bfd_sessions` decrements |
| AC-9 | A BFD Down and an inactivity-timer Down race for the same v3 neighbour | the neighbour drops exactly once; no panic; idempotent (R-1) |
| AC-10 | A config reload sets `bfd.enabled false` on a v6 interface with Full neighbours | every BFD session on that interface is released; the adjacencies stay Full (BFD removal does not bounce the neighbour) |
| AC-11 | A config reload sets `bfd.enabled true` on a v6 interface with already-Full neighbours | a session is opened for each already-Full neighbour without re-forming the adjacency |
| AC-12 | A dual-stack link runs both OSPFv2 and OSPFv3 with BFD on each | two distinct BFD sessions (distinct `Key`: IPv4 pair vs link-local pair); releasing the v3 session does not affect the v2 session |
| AC-13 | The request `Peer`/`Local` are link-locals (engine is v6) | the `SessionRequest` carries IPv6 `netip.Addr` link-locals, never an IPv4 address; `Mode: SingleHop`; the v2 sibling (ext-10) on the same link uses the IPv4 pair |
| AC-14 | `min-tx 0` or `multiplier 0` in the v6 interface `bfd` config | rejected at parse/validation time with a clear error (YANG `range` + `parseInterface` guard) |
| AC-15 | `show ipv6 ospf neighbor` for a BFD-protected, Up v3 neighbour | shows the BFD session state (Up); a down/absent session is distinguishable |
| AC-16 | The OSPF plugin is removed from the build | no `ze_ospf_bfd_*` metrics, no OSPF BFD code; the BFD engine and BGP-BFD client are unaffected (self-containment) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables BFD on an OSPFv3 interface and forms an adjacency with FRR `ospf6d` | config -> `parseInterface` (v6 sub-config) -> Full -> `EnsureSession` (IPv6 single-hop, link-local pair) -> BFD handshake with FRR's `bfdd` -> session Up | `test/ospfv3/ospfv3-bfd-session.ci` + `ospfv3-bfd-frr` interop |
| 2 | Pulls the link / kills FRR; OSPFv3 detects the loss in the BFD detection window, not after RouterDeadInterval | BFD detect-timer expiry -> `StateDown` -> subscriber -> `Table.NeighborDown` -> neighbour Down -> SPF re-run -> IPv6 route withdrawal, all well under 40 s | `ospfv3-bfd-frr` interop measures detect time < RouterDeadInterval |
| 3 | Runs `show ipv6 ospf neighbor` and sees which adjacencies are BFD-protected and the session state | snapshot -> neighbour rows annotated with BFD state from the session map | `test/ospfv3/ospfv3-bfd-show.ci` |
| 4 | Runs BFD on a dual-stack link for both OSPFv2 and OSPFv3 simultaneously | two distinct sessions (IPv4 pair + link-local pair); each family's down is independent | `TestOSPFv3BFDDistinctFromV2OnSameLink` + `ospfv3-bfd-frr` (dual-stack variant) |
| 5 | Runs OSPFv3 on a box where the BFD plugin was never loaded | `GetService()==nil` -> warning -> OSPFv3 on timers; no crash, no blocked adjacency | `TestOSPFv3BFDGracefulWhenPluginAbsent` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseInterfaceBFDv6` | `internal/plugins/ospf/config_test.go` | AC-1, AC-14: parse `bfd` container under `address-family ipv6 interface`; defaults; reject 0 timers/multiplier | |
| `TestBFDRequestForNeighborV6` | `internal/plugins/ospf/bfd_client_v6_test.go` | AC-2, AC-13, A-3, A-8, R-2: SessionRequest link-local pair (Peer = neighbour LL, Local = interface LL incl. zone) + Mode SingleHop + timers from config | |
| `TestOSPFv3BFDSessionOpenedOnFull` | `internal/plugins/ospf/bfd_client_v6_test.go` | AC-2, A-1: one EnsureSession + subscriber on Full for a v6 engine (fake `api.Service`) | |
| `TestOSPFv3BFDNotOpenedWhenDisabled` | `internal/plugins/ospf/bfd_client_v6_test.go` | AC-3: no EnsureSession when interface BFD disabled | |
| `TestOSPFv3BFDOnlyAtFull` | `internal/plugins/ospf/bfd_client_v6_test.go` | AC-2, A-4: no session before Full (Init/Exchange) | |
| `TestOSPFv3BFDUsesLinkLocalPair` | `internal/plugins/ospf/bfd_client_v6_test.go` | AC-13, A-6: a v6 engine produces an IPv6 link-local pair, never an IPv4 address | |
| `TestOSPFv3BFDGracefulWhenPluginAbsent` | `internal/plugins/ospf/bfd_client_v6_test.go` | AC-4: nil Service -> warning + failure metric, no session | |
| `TestOSPFv3BFDDownDrivesNeighborDown` | `internal/plugins/ospf/bfd_client_v6_test.go` | AC-5, A-2: StateDown -> `Table.NeighborDown` -> v3 neighbour Down + LSA re-origination | |
| `TestOSPFv3BFDAdminDownTreatedAsDown` | `internal/plugins/ospf/bfd_client_v6_test.go` | AC-6: StateAdminDown -> neighbour Down | |
| `TestOSPFv3BFDUpInitNoTeardown` | `internal/plugins/ospf/bfd_client_v6_test.go` | AC-7: Up/Init logged, no NSM change | |
| `TestOSPFv3BFDSessionReleasedOnDown` | `internal/plugins/ospf/bfd_client_v6_test.go` | AC-8: leaving Full -> Unsubscribe + ReleaseSession; subscriber exits | |
| `TestOSPFv3BFDSubscriberExitsOnRelease` | `internal/plugins/ospf/bfd_client_v6_test.go` | R-3: subscriber goroutine exits on stop and on channel close (no leak) | |
| `TestOSPFv3BFDDownIdempotentWithTimerDown` | `internal/plugins/ospf/bfd_client_v6_test.go` | AC-9, R-1: BFD down + timer down race drops the neighbour once, no panic | |
| `TestOSPFv3BFDDistinctFromV2OnSameLink` | `internal/plugins/ospf/bfd_client_v6_test.go` | AC-12, A-9: a v3 link-local key and a v2 IPv4 key on one interface are distinct; independent release | |
| `TestOSPFv3BFDClientMapIsolatedPerEngine` | `internal/plugins/ospf/bfd_client_v6_test.go` | R-4: the per-neighbour map is per-engine-instance; a v6 down does not touch a v4 neighbour | |
| `TestOSPFv3BFDReloadDisableKeepsAdjacency` | `internal/plugins/ospf/bfd_client_v6_test.go` | AC-10: reload disable releases sessions, adjacency stays Full | |
| `TestOSPFv3BFDReloadEnablesSessionForFullNeighbor` | `internal/plugins/ospf/bfd_client_v6_test.go` | AC-11, A-10: reload enable opens sessions for already-Full neighbours | |
| `TestOSPFv3BFDMetrics` | `internal/plugins/ospf/metrics_test.go` | sessions gauge / down counter / register-failure counter move correctly under the `interface` label | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `min-tx` (us) | 10..255000 | 255000 | 0 (rejected) | 255001 (rejected) |
| `min-rx` (us) | 10..255000 | 255000 | 0 (rejected) | 255001 (rejected) |
| `multiplier` | 1..255 | 255 | 0 (rejected) | 256 (rejected, uint8 range) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospfv3-bfd-config` | `test/ospfv3/ospfv3-bfd-config.ci` | `bfd { enabled true; min-tx ...; multiplier ... }` parses under `address-family ipv6 interface`; coexists with the v2 `bfd`, the top-level `bfd`, and BGP `bfd` | |
| `ospfv3-bfd-session` | `test/ospfv3/ospfv3-bfd-session.ci` | a Full v3 adjacency opens a BFD session; `show ipv6 ospf neighbor` shows it protected | |
| `ospfv3-bfd-show` | `test/ospfv3/ospfv3-bfd-show.ci` | `show ipv6 ospf interface` / `neighbor` render BFD enabled + session state | |
| `ospfv3-bfd-disable` | `test/ospfv3/ospfv3-bfd-disable.ci` | reload disabling BFD releases sessions without dropping the v3 adjacency | |

### Interop Tests (MANDATORY for protocol features)
<!-- REQUIRED when the spec adds/changes wire protocol behavior. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospfv3-bfd-frr` | `test/interop/scenarios/ospfv3-bfd-frr/` | FRR `ospf6d` + FRR `bfdd` (`ipv6 ospf6 bfd` on the link) | Ze and FRR form a Full OSPFv3 adjacency AND an IPv6 single-hop BFD Up session (link-local pair, Hop-Limit 255); pulling the link drives an OSPFv3 neighbour-down in the BFD detection window (well under RouterDeadInterval); re-adding the link re-forms both | |

> Interop is required: this exercises real IPv6 single-hop BFD wire behaviour
> (UDP 3784, Hop-Limit 255 GTSM, three-way handshake) against an independent
> implementation. The single-hop raw path is Linux-only and runs as a QEMU
> integration test (`ai/rules/qemu-testing.md`), consistent with the BFD,
> OSPFv2-BFD, and OSPFv3 transport interop sets. This scenario is a true failover
> test, not a wiring smoke test. A dual-stack variant (v2 + v3 BFD on one link)
> proves the per-protocol session independence (RFC 5881 §2, AC-12).

### Future (if deferring any tests)
- None. Every AC is covered by a unit, functional, or interop test above.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `internal/plugins/ospf/config.go` -- `interfaceConfig.BFD bfdInterfaceConfig` field; `bfdInterfaceConfig` struct (Enabled, MinTxUs, MinRxUs, Multiplier); parse the nested `bfd` map in `parseInterface` (shared by the IPv4 and `address-family ipv6` interface lists); validate timers/multiplier
- `internal/plugins/ospf/iface/iface.go` -- `Config.BFD` fields (carry enable + timers to the runtime layer)
- `internal/plugins/ospf/neighbor/neighbor.go` / `table.go` -- carry the per-interface BFD config into `InterfaceConfig`; ensure the up/down lifecycle can obtain the neighbour's raw `netip.Addr` (link-local) + interface (typed lifecycle callback or `Lookup`/`AddressOf` accessor); `NeighborDown` consumed unchanged
- `internal/plugins/ospf/instance.go` -- the BFD lifecycle observer gated on `e.dispatch.codec.IsV6()`: open on Full, release on down; `neighborInterfaceConfig` maps the BFD fields; `interfaceRuntimeConfigLocked` carries them; the per-neighbour `bfdClient` map lives on the engine (shared lifecycle with the OSPFv2 sibling); read the interface IPv6 link-local source from the v3 transport
- `internal/plugins/ospf/v3/transport/transport.go` / `backend_linux.go` -- expose a typed engine-facing accessor for the interface's IPv6 link-local source (`LinkLocalSource()` exists on the backend; surface it through the orchestrator if not already reachable from the engine)
- `internal/plugins/ospf/register.go` -- register `ze_ospf_bfd_sessions`, `ze_ospf_bfd_session_down_total`, `ze_ospf_bfd_register_failures_total` (shared with the OSPFv2 sibling; get-or-create); doctor informational check (BFD configured but plugin absent)
- `internal/plugins/ospf/cmd_show.go` / `show_summary.go` -- annotate `show ipv6 ospf neighbor` / `show ipv6 ospf interface` rows with BFD enabled + session state
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- a `container bfd` under the `list interface` in BOTH the IPv4 tree and the `address-family ipv6` tree, with `enabled` (boolean), `min-tx`/`min-rx` (uint32 us, range), `multiplier` (uint8, range)
- `internal/plugins/ospf/doctor.go` -- informational doctor check: BFD enabled on an interface but `api.GetService()` nil
- `plan/spec-ospfv3-ext-0-umbrella.md` -- (note, not code) update the umbrella "Metrics" mapping from `ze_ospfv3_<ext>_*` to `ze_ospf_bfd_*` for this child, reconciling with learned 970; mark the Engine<->BFD boundary row done

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `bfd` container on `interface` (both trees); read `ai/rules/config-surface.md` (operational config, not env var) + `ai/rules/config-naming.md` (kebab-case leaves) |
| YANG validation constraints | [ ] yes | `enabled` boolean; `min-tx`/`min-rx` `uint32 { range "10..255000"; }`; `multiplier` `uint8 { range "1..255"; }`; `units microseconds` on the timers |
| YANG custom validators | [ ] no | native `range` + `boolean` suffice |
| CLI commands/flags | [ ] yes | annotate `show ipv6 ospf interface` / `show ipv6 ospf neighbor` with a BFD column; no new top-level verb |
| CLI grammar (action before identifier) | [ ] n/a | no new verb added |
| Editor autocomplete | [ ] yes | automatic for the YANG boolean/uint leaves under `bfd` |
| Functional test for new RPC/API | [ ] yes | `test/ospfv3/ospfv3-bfd-*.ci` |
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

> These are the SAME series the OSPFv2 BFD sibling (ext-10) defines; the metrics
> registry is get-or-create by name, so v2 and v3 share one series and the
> `interface` label distinguishes the families. Do NOT introduce a `ze_ospfv3_bfd_*`
> namespace (user directive, learned 970). The umbrella "Metrics" table must be
> reconciled to `ze_ospf_bfd_*` when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- BFD for OSPFv3 |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` + the OSPF guide -- the per-interface `bfd` block under `address-family ipv6` |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- BFD column in `show ipv6 ospf neighbor`/`interface` |
| 4 | API/RPC added/changed? | [ ] no | reuses the frozen `internal/component/bfd/api` surface; no new RPC |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains an IPv6 BFD client |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` (the OSPFv3 / `show ipv6 ospf` section) + `docs/guide/bfd.md` -- OSPFv3 opt-in section |
| 7 | Wire format changed? | [ ] no | OSPFv3 wire unchanged; BFD wire is the delivered engine's IPv6 single-hop path |
| 8 | Plugin SDK/protocol changed? | [ ] no | uses the existing `api.Service` lookup |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc5880.md` + `rfc/short/rfc5881.md` -- the OSPFv3-client-relevant checklist context (client model, IPv6 single-hop, GTSM consumed) |
| 10 | Test infrastructure changed? | [ ] yes (interop scenario added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPFv3 BFD parity with FRR `ospf6d` |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- NSM <-> BFD lifecycle for the v6 family |
| 13 | Route metadata keys added/changed? | [ ] no | BFD does not install routes |
| 14 | Prometheus counters added/changed? | [ ] yes (shared series gains v3 producer) | the OSPF telemetry doc -- the `ze_ospf_bfd_*` series now produced by the v6 family too |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` + the umbrella metrics table |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed OSPF files |
| 17 | Existing docs show examples for this area? | [ ] check | verify OSPFv3 interface config examples against the new `bfd` block; verify `docs/guide/bfd.md` OSPFv3 section |

## Files to Create
- `internal/plugins/ospf/bfd_client_v6.go` -- the OSPFv3 BFD request builder + the v6 lifecycle gate: `bfdRequestForNeighborV6(neighbourLL, interfaceLL netip.Addr, ifname string, ifcfg bfdInterfaceConfig)`, the `codec.IsV6()`-gated open/close hooks. The shared `bfdClient` struct, `startBFDSession`/`stopBFDSession`/`runBFDSubscriber`, and the per-neighbour map live in a shared `bfd_client.go` introduced by the OSPFv2 sibling (ext-10); if ext-10 is not yet merged, create `bfd_client.go` here and have ext-10 reuse it
- `internal/plugins/ospf/bfd_client_v6_test.go` -- the unit suite above, driven by a `fakeBFDService` fake (mirrors `peer_bfd_test.go`)
- `test/ospfv3/ospfv3-bfd-config.ci`, `test/ospfv3/ospfv3-bfd-session.ci`, `test/ospfv3/ospfv3-bfd-show.ci`, `test/ospfv3/ospfv3-bfd-disable.ci`
- `test/interop/scenarios/ospfv3-bfd-frr/` -- `ze.conf`, `frr.conf` (`ospf6d` + `bfdd`, `ipv6 ospf6 bfd`), `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm `api.Service`, `Table.NeighborDown`, `neighborEventSink`, `codec.IsV6()`, `LinkLocalSource()` exist as described; confirm whether the ext-10 shared `bfd_client.go` is present |
| 3. Wiring phase | Wiring Test table -- the v6 BFD lifecycle gate + failing wiring tests |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- the v6 BFD lifecycle hook gated on `codec.IsV6()`
   - Tests: `TestOSPFv3BFDSessionOpenedOnFull` (fake Service), `TestOSPFv3BFDGracefulWhenPluginAbsent`, `test/ospfv3/ospfv3-bfd-session.ci`
   - Files: `bfd_client_v6.go` (the `IsV6()`-gated open/close hooks + stub request builder), `instance.go` (call them from `neighborEventSink.NeighborUp/NeighborDown`), the shared `bfd_client.go` (`startBFDSession`/`stopBFDSession` stubs + per-neighbour map) if not provided by ext-10, a `fakeBFDService` test fake
   - Verify: a v3 Full transition reaches `EnsureSession` (or degrades gracefully on nil); a v4 engine does NOT open a v6 session; deeper behaviour still stubbed so down/release tests fail
2. **Phase: Config surface** -- the per-interface `bfd` block on both interface lists
   - Tests: `TestParseInterfaceBFDv6`, boundary tests, `test/ospfv3/ospfv3-bfd-config.ci`
   - Files: `yang/ze-ospf-conf.yang` (the `bfd` container on both interface lists), `config.go` (`bfdInterfaceConfig` + parse + validation), `iface/iface.go` (`Config.BFD`), `instance.go` (`neighborInterfaceConfig` / `interfaceRuntimeConfigLocked` carry the fields), `neighbor` (`InterfaceConfig.BFD`)
   - Verify: config parses under `address-family ipv6`; 0 timers/multiplier rejected; the v6 engine sees per-interface BFD config
3. **Phase: v6 session request + open on Full** -- build the IPv6 single-hop request from the neighbour link-local + the interface link-local
   - Tests: `TestBFDRequestForNeighborV6`, `TestOSPFv3BFDUsesLinkLocalPair`, `TestOSPFv3BFDNotOpenedWhenDisabled`, `TestOSPFv3BFDOnlyAtFull`, `TestOSPFv3BFDDistinctFromV2OnSameLink`
   - Files: `bfd_client_v6.go` (`bfdRequestForNeighborV6`, the open path, the typed lifecycle callback carrying the raw `netip.Addr`), `v3/transport` (surface `LinkLocalSource()` to the engine)
   - Verify: correct link-local pair + Mode SingleHop + timers; only Full, only when enabled; distinct key from a co-resident v2 session
4. **Phase: Down injection + subscriber** -- BFD Down -> NSM down
   - Tests: `TestOSPFv3BFDDownDrivesNeighborDown`, `TestOSPFv3BFDAdminDownTreatedAsDown`, `TestOSPFv3BFDUpInitNoTeardown`, `TestOSPFv3BFDSubscriberExitsOnRelease`, `TestOSPFv3BFDDownIdempotentWithTimerDown`, `TestOSPFv3BFDClientMapIsolatedPerEngine`, `ospfv3-bfd-frr` interop
   - Files: `bfd_client.go`/`bfd_client_v6.go` (`runBFDSubscriber` -> `Table.NeighborDown`), the stop+done handshake
   - Verify: Down/AdminDown drop the v3 neighbour through the existing NSM; Up/Init inert; subscriber never leaks; race with timer-down idempotent; per-engine map isolation
5. **Phase: Release lifecycle + reload** -- release on leaving Full, on interface down, on config disable/enable
   - Tests: `TestOSPFv3BFDSessionReleasedOnDown`, `TestOSPFv3BFDReloadDisableKeepsAdjacency`, `TestOSPFv3BFDReloadEnablesSessionForFullNeighbor`, `ospfv3-bfd-disable.ci`
   - Files: `bfd_client.go` (`stopBFDSession`), `instance.go` (release on `NeighborDown`/`InterfaceDown`/reload diff)
   - Verify: sessions release cleanly; disabling BFD does not bounce the v3 adjacency; enabling opens sessions for already-Full neighbours
6. **Phase: CLI + metrics + doctor** -- operator surface
   - Tests: `TestOSPFv3BFDMetrics`, `ospfv3-bfd-show.ci`, `TestOSPFv3BFDShowNeighbor`
   - Files: `register.go` (metrics, shared series), `cmd_show.go`/`show_summary.go` (BFD annotation on `show ipv6 ospf`), `doctor.go` + `internal/core/diagnostic/codes.go` (informational check)
   - Verify: `show ipv6 ospf neighbor`/`interface` show BFD state; the three metric series move under the `interface` label; the doctor check fires when BFD is configured-but-absent
7. **Functional tests** -> the four `.ci` cover the user-visible behaviour
8. **RFC refs** -> add `// RFC 5881 Section X` / `// RFC 5880 Section X` comments on the IPv6 single-hop request, the GTSM-255 rationale (and its independence from base-OSPFv3 Hop-Limit-1), the Down-handling, and the slow-start note
9. **Interop** -> `ospfv3-bfd-frr` QEMU scenario (true failover test, dual-stack variant for AC-12)
10. **Full verification** -> `make ze-verify`
11. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; parity with FRR's `ipv6 ospf6 bfd` (session on adjacency, immediate down on BFD loss) |
| Correctness | session opened only at Full AND only when `codec.IsV6()`; only Down/AdminDown drives `NeighborDown`; address pair is the IPv6 link-local pair (Peer = neighbour LL, Local = interface LL), never an IPv4 address; Mode SingleHop always; idempotent down |
| Naming | `ze_ospf_bfd_*` metrics (NOT `ze_ospfv3_*`); YANG `bfd`/`min-tx`/`min-rx`/`multiplier` kebab-case; `bfdRequestForNeighborV6`/`startBFDSession`/`stopBFDSession` |
| Data flow | BFD down flows through `Table.NeighborDown` + existing NSM; OSPF imports only `bfd/api`; no OSPF symbol in `internal/component/bfd`; the v6 link-local `Local` comes from the v3 transport, not `iface.Config.InterfaceAddress` |
| CLI grammar | no new verb; show annotation only |
| Doctor checks | informational BFD-configured-but-absent check registered per `ai/rules/doctor-checks.md` |
| YANG validation | `bfd` leaves have `range`/`boolean`; 0 timers/multiplier rejected; bare `type string` absent |
| Prometheus counters | the shared series produced by the v6 family; umbrella table reconciled to `ze_ospf_bfd_*` |
| Rule: plugin-self-containment | removing OSPF removes all its BFD wiring; BFD engine + BGP client untouched |
| Rule: goroutine-lifecycle | the subscriber is one per-session worker with stop+done handshake; no leak under `-race`; per-engine map isolation (R-4) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| OSPFv3 opens an IPv6 single-hop BFD session on Full | `go test ./internal/plugins/ospf -run TestOSPFv3BFDSessionOpenedOnFull` |
| The request carries the IPv6 link-local pair, not an IPv4 address | `go test ./internal/plugins/ospf -run TestBFDRequestForNeighborV6` |
| BFD Down drives `Table.NeighborDown` for a v3 neighbour | `go test ./internal/plugins/ospf -run TestOSPFv3BFDDownDrivesNeighborDown` |
| Per-interface `bfd` config parses + validates under v6 | `go test ./internal/plugins/ospf -run TestParseInterfaceBFDv6` |
| Graceful degradation when plugin absent | `go test ./internal/plugins/ospf -run TestOSPFv3BFDGracefulWhenPluginAbsent` |
| Shared `ze_ospf_bfd_*` series (no `ze_ospfv3_bfd_*`) | `grep -rn 'ze_ospf_bfd_' internal/plugins/ospf` and `grep -rn 'ze_ospfv3_bfd_' internal/plugins/ospf` (latter empty) |
| Interop scenario present | `ls test/interop/scenarios/ospfv3-bfd-frr/` |
| Functional tests present | `ls test/ospfv3/ospfv3-bfd-*.ci` |
| Only `bfd/api` imported from OSPF | `grep -rn 'internal/component/bfd' internal/plugins/ospf` shows only `/api` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | `bfd` timer/multiplier leaves range-checked; the neighbour link-local fed to `SessionRequest` is a validated `netip.Addr` (with zone), never an unparsed string |
| Resource exhaustion | one session per Full v3 adjacency (bounded by neighbour count); the engine shares one UDP loop per (vrf, single-hop); `ze_ospf_bfd_sessions` gauge observable |
| Subscriber isolation | a panicking subscriber cannot wedge the NSM lock; `Table.NeighborDown` takes its own lock and the subscriber calls it outside any OSPF lock |
| Trust boundary | BFD IPv6 single-hop relies on GTSM (Hop-Limit 255) enforced by the engine; OSPFv3 adds no new listening port or socket -- the BFD engine owns the wire; base OSPFv3 Hop-Limit-1 is a separate transport unaffected |
| Error leakage | `EnsureSession`/`ReleaseSession` errors are logged + counted, never surfaced to a peer or the wire |
| DoS via flap | a flapping v3 adjacency opens/closes sessions; the release path is idempotent and bounded; no unbounded goroutine growth (R-3) |

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
BFD-for-OSPFv3 is a wiring problem on a unified engine, not a new protocol: the
BFD engine (including the delivered IPv6 single-hop transport, learned 564), its
in-process `api.Service` client contract, and the down-injection seam
(`Table.NeighborDown`) all already exist, and the OSPF engine is one unified v2/v3
implementation distinguished by `codec.IsV6()`. The spec connects two ends that
are both in tree -- NSM Full (on the v6 instance) -> `EnsureSession` with the
link-local pair, BFD Down -> `NeighborDown` -- and the BGP and OSPFv2 BFD clients
are complete templates. The only genuinely v3-specific surface is the request
builder (IPv6 link-local `Local` from the v3 transport, never the IPv4
`InterfaceAddress`) and the `IsV6()` gate; everything else (config block, client
map, lifecycle, metrics) is shared with the OSPFv2 sibling.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Gate the v6 BFD lifecycle on `codec.IsV6()` and share the client map/lifecycle with OSPFv2 | a separate OSPFv3 BFD client package; a second engine field | the engine is unified v2/v3; one lifecycle + one map keyed `(iface, RouterID)` per engine-instance avoids duplicate code, and the family gate keeps the v4/v6 request builders apart |
| `Peer`/`Local` = the IPv6 link-local pair (neighbour LL from `Neighbor.Address`, interface LL from the v3 transport) | reuse `iface.Config.InterfaceAddress` ([4]byte) | `InterfaceAddress` is IPv4-only; OSPFv3 adjacencies and BFD single-hop are link-local; RFC 5881 §6 on-subnet src/dst maps to the link-local pair |
| Distinct BFD session from any co-resident OSPFv2 session on the same link | one shared session for both families | RFC 5881 §2 mandates one session per data protocol; the differing address pair gives distinct `Key` tuples and independent refcounts automatically |
| Reuse the `ze_ospf_*` metric namespace (no `ze_ospfv3_*`) | a parallel `ze_ospfv3_bfd_*` series | user directive (learned 970): "OSPFv3 is our OSPF"; the `interface` label distinguishes families on one series |
| Drive `Table.NeighborDown` on BFD Down | a dedicated BFD-down NSM event; a direct state poke | the existing seam produces every correct downstream effect (kill-nbr, LSA re-origination, SPF); a BFD-down stays indistinguishable from a timer-down |
| Open the session only at Full | open at 2-Way / Exchange | Full is the adjacency actually carrying flooding; matches FRR `ospf6d`; avoids churn during ExStart/Exchange |
| Single-hop only (`Mode: SingleHop`) | configurable hop mode | OSPFv3 neighbours are directly-connected link-local; multi-hop is for virtual links (ext-3, out of scope) |

## Known Limitations
- Virtual links (multi-hop OSPFv3 adjacencies, ext-3) are not BFD-protected by this spec (single-hop only).
- A freshly-formed v3 adjacency detects in up to `multiplier * 1 s` until the session reaches Up and fast rates negotiate (RFC 5880 §6.8.3 slow-start); configured `min-tx`/`min-rx` apply only after Up.
- BFD authentication and echo are engine concerns; OSPFv3 sessions run unauthenticated, async, no-echo in this spec.
- If the interface IPv6 link-local source has not resolved (DAD) when a session opens, `Local` may be left zero (kernel source selection); in practice DAD completes long before a neighbour reaches Full (Hellos have already flowed). The `ospfv3-bfd-frr` interop retries until Up.

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code:
- RFC 5881 §3 both ends Active -> `bfdRequestForNeighborV6` leaves `Passive` false
- RFC 5881 §6 on-subnet src/dst -> the link-local `Peer`/`Local` pair in the request
- RFC 5881 §4 / §5 single-hop bound to one interface, Hop-Limit 255 GTSM -> `Mode: SingleHop` + the interface name in the request (engine enforces Hop-Limit); a comment noting this is independent of base OSPFv3 Hop-Limit-1 (learned 970)
- RFC 5881 §2 one session per data protocol -> the comment explaining a v3 session is distinct from a co-resident v2 session
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
| BFD session opened on a usable OSPFv3 adjacency (IPv6 single-hop, link-local pair) | unit + interop | `TestOSPFv3BFDSessionOpenedOnFull`, `TestBFDRequestForNeighborV6`, `ospfv3-bfd-frr` |
| BFD Down declares the v3 neighbour down faster than RouterDeadInterval | interop | `ospfv3-bfd-frr` measures detect time < RouterDeadInterval |
| IPv6 link-local keying distinct from a co-resident OSPFv2 session | unit + interop | `TestOSPFv3BFDDistinctFromV2OnSameLink`, `ospfv3-bfd-frr` dual-stack variant |
| Per-interface enable + BFD timer config under `address-family ipv6` | unit + functional | `TestParseInterfaceBFDv6`, `ospfv3-bfd-config.ci` |
| Down-event path through the existing NSM | unit | `TestOSPFv3BFDDownDrivesNeighborDown` |
| Graceful degradation without the BFD plugin | unit | `TestOSPFv3BFDGracefulWhenPluginAbsent` |
| Mirrors the BGP/OSPFv2 BFD client wiring on the unified engine | review | structural diff against `peer_bfd.go` + `spec-ospf-ext-10-bfd.md`; self-containment grep |

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
- [ ] AC-1..AC-16 all demonstrated
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
- [ ] No premature abstraction (the client is a concrete copy of `peer_bfd.go` / the OSPFv2 sibling, justified by the proven pattern)
- [ ] No speculative features (no echo, no multi-hop, no virtual-link BFD)
- [ ] Single responsibility per component (the BFD client only bridges NSM <-> session)
- [ ] Explicit > implicit behavior (opt-in per interface; nil-safe degradation; `IsV6()` gate)
- [ ] Minimal coupling (OSPF imports only `bfd/api`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospfv3-bfd-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospfv3-ext-5-bfd.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospfv3-ext-5-bfd.md`
