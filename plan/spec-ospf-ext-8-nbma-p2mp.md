# Spec: ospf-ext-8 -- OSPFv2 NBMA + Point-to-Multipoint Network Types (RFC 2328)

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
3. `rfc/short/rfc2328.md` -- §9.1-9.3 Interface State Machine (NBMA/PtMP transitions), §9.4 DR/BDR election (NBMA Start-to-priority-0 step), §9.5 sending Hellos (NBMA/PtMP unicast), §10.1-10.5 Neighbor State Machine (Attempt state, Start event, `should_adj`), §13.3 flooding (non-broadcast unicast), §16.1 next-hop (PtMP host-route next-hop), App A.4.2 Router-LSA link Type 1/Type 3 (PtMP p2p link + host-route stub link), App C.5/C.6 NBMA/PtMP configurables (PollInterval, static neighbour list)
4. `plan/spec-ospf-0-umbrella.md` -- "Shared Contracts" (the `network-type` enum currently `broadcast`/`point-to-point`; this spec ADDS `nbma` + `point-to-multipoint`), and the umbrella "Out of scope" row "NBMA and point-to-multipoint network types" + the §354 decision row that this spec closes
5. `internal/plugins/ospf/iface/iface.go` -- the per-interface runtime: `Start()` network-type switch (the chokepoint for ISM init), `runElectionLocked()` (today gated `NetworkType != NetworkBroadcast`), `SendHello()` (today only multicast), `buildHelloPacket()`, `validateHelloLocked()`
6. `internal/plugins/ospf/iface/ism.go` -- the `State` enum + the `NetworkBroadcast`/`NetworkPointToPoint`/`NetworkLoopback` string constants this spec extends
7. `internal/plugins/ospf/iface/election.go` -- `electDRBDR`/`chooseBDR`/`chooseDeclaredDR` (reused verbatim for NBMA; only the election GATE in `iface.go` changes)
8. `internal/plugins/ospf/neighbor/nsm.go` -- `shouldAdj` (today `point-to-point` always, `broadcast` DR/BDR-only, default false -- the default must become true for PtMP and DR-gated for NBMA), `startExchange`
9. `internal/plugins/ospf/neighbor/table.go` -- `hello()` (NSM driver), `sendInitialDDLocked` (already unicast to `n.Address`), `FloodNeighbors`/`AcceptsFlooding`
10. `internal/plugins/ospf/lsdb/flooding.go` -- `floodDestination` (multicast group; PtMP/NBMA must send per-neighbour unicast), `floodExcept`, `eligibleInterface`, `neighborAddr` (already unicast for retransmit)
11. `internal/plugins/ospf/lsdb/origination.go` -- `routerLinks` (per-interface link records: PtMP must emit one Type-1 p2p link per Full neighbour + a host-route Type-3 stub link, NOT a transit/network stub), `OriginateNetwork` (NBMA originates a Network-LSA when DR; PtMP NEVER does)
12. `internal/plugins/ospf/config.go` -- `parseInterface` network-type switch (rejects anything outside `broadcast|point-to-point|loopback` today), `interfaceConfig` struct
13. `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the `interfaces/interface/network-type` enum (lines ~178-186) + where the new `nbma-neighbor` list and `poll-interval` leaf attach

## Task

Add the two remaining RFC 2328 interface **network types** to the native OSPFv2
plugin at `internal/plugins/ospf/`: **NBMA** (non-broadcast multi-access) and
**point-to-multipoint (PtMP)**. The umbrella (`plan/spec-ospf-0-umbrella.md`)
shipped OSPFv2 with broadcast (DR/BDR + Network-LSA) and point-to-point only, and
explicitly listed NBMA + PtMP as future (umbrella "Out of scope" and the §354
decision row). The codec, the ISM, the NSM, DR/BDR election, flooding, Router-LSA
origination, and the per-interface config plumbing all already exist for the two
shipped types; what is missing is the two new behavioural variants and the config
surface that selects them.

**NBMA** (RFC 2328 §9.5, §10, App C.5) treats one multi-access link with no
multicast capability: there is no AllSPFRouters group to reach, so the router must
be told its peers by a **manually configured neighbour list**. A **DR/BDR is still
elected** among the configured, eligible (priority > 0) routers exactly as on a
broadcast link (the §9.4 election is reused verbatim), and the DR still originates
the Type-2 Network-LSA. Hellos are sent **unicast to each configured neighbour**
(not multicast), at the normal HelloInterval to neighbours we have heard from and
at the slower **PollInterval** to neighbours that are currently silent (the §10.1
Attempt state). A configured neighbour with priority 0 is **ineligible** for the
election but is still polled; per §9.4 step 6, when this router becomes DR or BDR
it sends a Hello (Start) to those priority-0 neighbours so they begin the
adjacency. Adjacency formation follows the broadcast rule (`should_adj`: only with
the DR/BDR).

**Point-to-multipoint** (RFC 2328 §9.5, §10.4, §12.4.1.4, §16.1) treats the same
medium as a **collection of point-to-point links**: there is **no DR, no BDR, no
Network-LSA**; every router forms a full adjacency with **every** other reachable
router (`should_adj` always true). The interface advertises itself in the
Router-LSA as one **Type-1 (point-to-point) link per Full neighbour** (LinkID =
the neighbour's Router ID) plus a **host-route stub link** (Type-3, mask
255.255.255.255) for the interface's own IP address, so that other routers can
reach the PtMP interface address; it does NOT advertise the subnet as a transit or
network-prefix stub. SPF resolves the next-hop to each neighbour directly from
that neighbour's interface address in its Router-LSA p2p link (§16.1). Hellos are
sent to **AllSPFRouters** on the broadcast variant (when the medium supports
multicast) or **unicast to a configured neighbour list** on the non-broadcast
variant; this spec implements the broadcast-variant (multicast Hello) as the
default and supports the same `nbma-neighbor` list for the non-broadcast variant.

Both variants reuse the existing unicast adjacency-packet path (DD / LS Request /
LS Update retransmission already go to `n.Address`, per `neighbor/dd.go` and
`flooding.go neighborAddr`); the only flooding delta is the **initial flood
destination** (`floodDestination`), which on a non-broadcast interface must be a
per-neighbour unicast fan-out instead of a multicast group.

This is an additive, self-contained extension inside the existing OSPF edge
plugin. A broadcast or point-to-point interface behaves exactly as today; the new
behaviour is reachable only when an interface is configured `network-type nbma`
or `network-type point-to-multipoint`.

### In scope (this spec)

| Item | Detail |
|------|--------|
| Network-type config extension | Add `nbma` + `point-to-multipoint` to the YANG `interfaces/interface/network-type` enum and to the `parseInterface` accept-list + the `networkType` string constants; both flow through the existing `Config.NetworkType`/`InterfaceInfo.NetworkType` string threads |
| NBMA neighbour config | A per-interface `nbma-neighbor` list (key `address`, leaf `priority` default 0) and a per-interface `poll-interval` leaf (default 120 s, App C.5); resolved into the interface runtime |
| NBMA ISM + election | On `InterfaceUp` an eligible (priority > 0) NBMA interface enters Waiting and runs the §9.4 election over the configured neighbours; priority 0 goes straight to DROther; the existing `electDRBDR` is reused unchanged, only the election GATE in `iface.go` widens to include NBMA |
| NBMA unicast Hello + poll | Hellos sent unicast to each configured neighbour; HelloInterval to heard neighbours, PollInterval to silent ones (Attempt-state poll); §9.4-step-6 Start Hello to priority-0 neighbours when this router becomes DR/BDR |
| NBMA Network-LSA | The DR of an NBMA segment still originates the Type-2 Network-LSA (the existing `OriginateNetwork` path, gated on `DR == self` and a non-PtMP network type) |
| PtMP adjacency | `should_adj` returns true for PtMP (every neighbour adjacent); no DR/BDR election; no Network-LSA; Hellos to AllSPFRouters (broadcast variant) or the configured neighbour list (non-broadcast variant) |
| PtMP host-route origination | `routerLinks` emits, for a PtMP interface, one Type-1 p2p link (LinkID = neighbour Router ID, LinkData = our interface address, metric = interface cost) per Full neighbour, plus a single host-route Type-3 stub link (LinkID = our interface address, mask 255.255.255.255, metric 0); it does NOT emit the transit/subnet stub link that broadcast/p2p use |
| PtMP next-hop | SPF resolves the next-hop to each PtMP neighbour from the neighbour's interface address in its advertised p2p link (§16.1); reuse the existing `spf/` next-hop machinery (no new next-hop code if the p2p-link next-hop already works for point-to-point) |
| Non-broadcast flood fan-out | `floodDestination` (and the initial flood path) on an NBMA / non-broadcast-PtMP interface unicasts to each Flood-eligible neighbour instead of returning a multicast group |
| `show ip ospf interface` surface | The interface snapshot already carries `network_type`; ensure NBMA/PtMP render and that NBMA shows its configured-neighbour/poll state |

### Out of scope (noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| Virtual links (synthetic p2p across a transit area, RFC 2328 §15) | spec-ospf-ext-7 (explicitly excluded by this task) |
| OSPFv3 NBMA/PtMP (RFC 5340 has no NBMA; PtMP differs) | not in this spec; the v6 YANG enum stays `broadcast`/`point-to-point` |
| RFC 6845 hybrid broadcast-and-PtMP interface type | future (guide ref #28); not the RFC 2328 PtMP this spec adds |
| RFC 5613 LLS / RFC 2328 demand circuits (PtMP-over-demand) | out of scope (umbrella future list) |
| Two-part metric (RFC 8042) on PtMP | out of scope |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -->
- [ ] `docs/research/ospf-implementation-guide.md` §7 "Network Types and Interface Model" (~470-504, the network-type table + per-type prose) -- the authoritative behavioural contrast for all four wire types
  -> Decision: PtMP is "a collection of point-to-point links, no DR" with host-route origination; NBMA is "explicit static neighbour list, DR elected, unicast per neighbour" -- this spec implements exactly that split and reuses the broadcast election for NBMA and the p2p link model for PtMP
  -> Constraint: default Hello interval is 30 s on NBMA/PtMP vs 10 s on broadcast/P2P (guide ~499); the per-interface default stays the YANG `hello-interval` default 10 unless the operator overrides, but the spec documents the 30 s recommendation and the PollInterval default 120 s
- [ ] `docs/research/ospf-implementation-guide.md` §5 ISM/NSM prose (~255-321: Waiting/DROther/Attempt states, the `should_adj` predicate)
  -> Constraint: `should_adj` is "point-to-point, point-to-multipoint, and virtual links: always yes; broadcast or NBMA: only if local or neighbour is DR/BDR" (guide ~321); the NSM `shouldAdj` default-branch must split into PtMP (true) and NBMA (DR-gated), not stay `false`
  -> Constraint: NBMA Attempt state (guide ~298) -- "poll interval is running; we expect to hear from this configured neighbour eventually"; a configured-but-silent NBMA neighbour is polled, not dropped
- [ ] `docs/research/ospf-implementation-guide.md` §8 flooding addressing (~355) -- "Point-to-point, point-to-multipoint, and NBMA use unicast (or multi-unicast) per RFC 2328 §13.3 Table 19"
  -> Constraint: the initial flood on a non-broadcast interface is a per-neighbour unicast fan-out; the existing retransmit path is already unicast, so only `floodDestination`/the initial-flood send changes
- [ ] `plan/spec-ospf-0-umbrella.md` "Shared Contracts" + "Out of scope" + §354 decision row -- the network-type contract this spec extends
  -> Constraint: the umbrella declared the `network-type` enum as `broadcast`/`point-to-point` and NBMA/PtMP as future; this spec adds the two enum values and closes that future item; it must NOT redefine the interface config model, only extend the enum and add the NBMA-only `nbma-neighbor`/`poll-interval` leaves
  -> Decision: keep per-interface `area` binding, costs, timers, priority, passive, auth exactly as the umbrella defines; NBMA/PtMP are new values of an existing leaf, not a new config subsystem
- [ ] `ai/rules/buffer-first.md`, `ai/rules/no-sprintf-alloc.md` -- Hello/flood encode and the Router-LSA host-route link are buffer-first
  -> Constraint: PtMP host-route + per-neighbour p2p links are appended into the existing `routerLinks` slice and encoded via the existing buffer-first `RouterLSA.WriteTo`; the unicast Hello fan-out reuses the existing per-packet `buildHelloPacket` buffer, sent once per neighbour, with no `fmt`/`+` string building

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2328.md` -- OSPF Version 2
  -> Constraint: §9.1/§9.3 -- an eligible interface on broadcast OR NBMA enters Waiting on InterfaceUp and delays the (B)DR election until WaitTimer/BackupSeen; a priority-0 interface goes DROther; PtMP (like p2p) goes straight to the point-to-point ISM state with no election
  -> Constraint: §9.4 step 6 -- "On NBMA networks, if Router X became DR or BDR, it must start sending Hello packets (Start event) to those neighbors that are not eligible to become DR (priority 0)"
  -> Constraint: §9.5 -- "On NBMA networks Hello packets are sent ... directly (as unicasts) to those neighbors eligible to become DR" at HelloInterval, and to ineligible neighbours at the slower PollInterval; on PtMP networks Hellos are sent (multicast or unicast) to all attached neighbours
  -> Constraint: §10.1 -- the **Attempt** state exists ONLY on NBMA: "no recent information ... but Hello packets are still sent at PollInterval"; §10.3 Start event moves Down->Attempt
  -> Constraint: §10.4 `should_adj` -- always adjacent on point-to-point, point-to-multipoint, and virtual links; on broadcast/NBMA only with the DR/BDR
  -> Constraint: App A.4.2 -- a PtMP interface contributes one Type-1 (point-to-point) Router-LSA link per fully adjacent neighbour (LinkID = neighbour Router ID, LinkData = own interface IP) and a host route (Type-3 stub, LinkID = own interface IP, mask 0xffffffff, metric 0); §12.4.1.4 "a point-to-multipoint interface ... adds a host route to its own interface address"
  -> Constraint: §16.1 next-hop -- for PtMP the next-hop to a destination is the neighbour's interface address taken from the destination router-LSA's matching p2p link, exactly as for point-to-point with a known neighbour address
  -> Constraint: App C.5 -- PollInterval is the reduced Hello rate to inactive NBMA neighbours (sample 2 minutes / 120 s)

**Key insights:**
- NBMA = broadcast election + manual neighbour list + unicast/poll Hellos. The §9.4 election (`electDRBDR`) and the Network-LSA origination are reused verbatim; the only new behaviour is the configured-neighbour source, the unicast/poll Hello send, the Attempt state, and the §9.4-step-6 Start Hello.
- PtMP = point-to-point semantics on a multi-access medium. No DR, no Network-LSA, `should_adj` always true, one p2p Router-LSA link per neighbour, plus a host-route stub link. The point-to-point ISM/NSM/next-hop machinery is reused; the only new behaviour is per-neighbour p2p-link origination and the host-route stub link.
- The two new types share one config delta (the enum + the optional `nbma-neighbor`/`poll-interval` leaves) and one flooding delta (`floodDestination` unicast fan-out on non-broadcast interfaces). Everything else is per-type branching at existing chokepoints (`iface.Start`, the election gate, `shouldAdj`, `routerLinks`).

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/ospf/iface/ism.go` -- defines `State` (Down/Loopback/Waiting/PointToPoint/DROther/Backup/DR) and the network-type string constants `NetworkBroadcast`/`NetworkPointToPoint`/`NetworkLoopback`; there is NO `nbma`/`point-to-multipoint` constant
  -> Constraint: add `NetworkNBMA = "nbma"` and `NetworkPointToMultipoint = "point-to-multipoint"` constants here (the single source of the iface-package string), mirrored by the `lsdb` package's own copies (`flooding.go` `NetworkBroadcast`/`NetworkPointToPoint`) and the `neighbor` package's `NetworkPointToPoint`/`NetworkBroadcast`
- [ ] `internal/plugins/ospf/iface/iface.go` -- `Start()` switches on `NetworkType`: `loopback` -> Loopback (no timers); `point-to-point` -> PointToPoint + timers; `default` -> Waiting (priority > 0) or DROther (priority 0) + timers, and arms the WaitTimer ONLY when `NetworkType == NetworkBroadcast`. `SendHello()` sends to `allSPFRouters` (or v6 group) only. `runElectionLocked()` returns early unless `NetworkType == NetworkBroadcast`. `buildHelloPacket()` builds one Hello for all neighbours
  -> Constraint: PtMP must take the `point-to-point` ISM branch (PointToPoint state, no election, no WaitTimer); NBMA must take the broadcast-like branch (Waiting/DROther + WaitTimer) but with unicast/poll Hellos. The election gate `NetworkType != NetworkBroadcast` must widen to `!= NetworkBroadcast && != NetworkNBMA`. `SendHello`/`buildHelloPacket` gain a per-neighbour unicast fan-out for non-multicast interfaces
- [ ] `internal/plugins/ospf/iface/election.go` -- `electDRBDR`, `chooseBDR`, `chooseDeclaredDR`, `betterCandidate` -- pure RFC §9.4 election over a candidate slice
  -> Constraint: reuse verbatim for NBMA; the candidates for an NBMA election are the configured neighbours we have heard from plus self; no change to this file
- [ ] `internal/plugins/ospf/neighbor/nsm.go` -- `shouldAdj`: `point-to-point` -> true; `broadcast`/`""` -> DR/BDR-only; `default` -> false. `startExchange` resets the DD/summary state
  -> Constraint: split the `default` branch: `point-to-multipoint` -> true (every neighbour adjacent); `nbma` -> the DR/BDR-only rule (same as broadcast). A bare `default false` would leave PtMP/NBMA neighbours stuck at 2-Way
- [ ] `internal/plugins/ospf/neighbor/table.go` -- `hello()` drives the NSM; `sendInitialDDLocked`/`resendLastDDLocked` already unicast to `n.Address`; `FloodNeighbors`/`AcceptsFlooding` gate flooding on neighbour state; there is no NBMA Attempt state and no poll timer here
  -> Constraint: the adjacency-bringup unicast path already works for any neighbour with a known `Address`; NBMA/PtMP need that address populated from the configured list (NBMA) or learned Hello source (both). The NBMA Attempt/poll lives in the `iface` Hello loop, not in `table.go`
- [ ] `internal/plugins/ospf/lsdb/flooding.go` -- `floodDestination(iface)` returns a multicast group (AllDRouters for a broadcast DROther, else AllSPFRouters); `floodExcept` runs the §13.3 eligible-interface + DR-relay rules; `neighborAddr`/the retransmit `sends` loop already unicast to `nbr.Address`; `eligibleInterface` is area/type scope only
  -> Constraint: the retransmit path is already per-neighbour unicast; the gap is the INITIAL flood and the Ack send, which use `floodDestination`'s multicast group. On NBMA / non-broadcast PtMP, the initial flood/ack must fan out unicast to each Flood-eligible neighbour. PtMP has no DR, so the §13.3 DR-relay suppression (`State != DR ... sender == BDR`) does not apply -- a PtMP/NBMA-without-DR-relay must flood to all adjacent neighbours
- [ ] `internal/plugins/ospf/lsdb/origination.go` -- `routerLinks(in)`: for `point-to-point` emits one Type-1 p2p link per Full neighbour (LinkID = neighbour Router ID, LinkData = our address); for `broadcast` with a DR emits a Type-2 transit link; ALWAYS appends a subnet stub link (Type-3, LinkID = network address, mask = NetworkMask) when address+mask are set. `OriginateNetwork` builds the Type-2 Network-LSA for the DR; `OriginateFromTopology` originates a Network-LSA only when `iface.DR == router`
  -> Constraint: PtMP must emit the per-neighbour Type-1 p2p links (reuse the point-to-point branch) BUT replace the subnet stub link with a host-route stub link (LinkID = our interface address, mask 255.255.255.255, metric 0); PtMP must NOT originate a Network-LSA. NBMA behaves like broadcast for origination (transit link when DR, subnet stub) and DOES originate a Network-LSA when DR
- [ ] `internal/plugins/ospf/config.go` -- `parseInterface` defaults `NetworkType = networkBroadcast` and accepts only `broadcast|point-to-point|loopback`, returning an error otherwise; `interfaceConfig` has no neighbour-list or poll-interval field
  -> Constraint: extend the accept-list with `nbma`/`point-to-multipoint`; add `NBMANeighbors []nbmaNeighborConfig` (address + priority) and `PollInterval uint16` to `interfaceConfig` and parse them; thread `PollInterval` and the neighbour addresses into the iface `Config`
- [ ] `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the v4 `interfaces/interface/network-type` enum is `broadcast`/`point-to-point`/`loopback` (default broadcast); no `nbma-neighbor` list, no `poll-interval` leaf; the v6 `address-family/ipv6/.../network-type` enum is `broadcast`/`point-to-point`
  -> Constraint: add `enum nbma;` + `enum point-to-multipoint;` to the v4 enum ONLY; add a `nbma-neighbor` list (key `address`, leaf `priority` default 0) and a `poll-interval` leaf (uint16, default 120, units seconds) under the v4 interface; do NOT touch the v6 enum
- [ ] `internal/plugins/ospf/instance.go` -- `ifaceConfig(ic)` maps `interfaceConfig` to `ospfiface.Config` (threads `NetworkType` as a string); `neighborInterfaceConfig` threads `NetworkType` into the NSM `InterfaceConfig`; `OriginateFromTopology` is driven from here on topology change
  -> Constraint: thread the new `PollInterval` + the configured NBMA neighbour list (address+priority) through `ospfiface.Config` so the iface Hello loop can unicast/poll; the network-type string already flows end to end (no new plumbing for the type itself, only for the NBMA extras)

**Behavior to preserve:**
- Broadcast (Waiting/DROther/Backup/DR + Network-LSA + multicast Hello/flood) and point-to-point (PointToPoint state + per-neighbour p2p link + subnet stub) behave EXACTLY as today; the new branches are reachable only for the two new enum values.
- The `electDRBDR` election, the `OriginateNetwork` Network-LSA, the `sendInitialDDLocked` unicast DD path, the `neighborAddr` retransmit unicast, and the §13.3 receive procedure are reused unchanged.
- All existing OSPF unit/functional/interop tests (broadcast + p2p) stay green; the YANG default `network-type broadcast` is unchanged.
- The v6 (OSPFv3) interface enum stays `broadcast`/`point-to-point` (no NBMA/PtMP for v6 in this spec).

**Behavior to change:** (all RFC-2328-required for the two new types, none discretionary)
- `parseInterface` accepts `nbma` + `point-to-multipoint`; the YANG enum gains both; the new `nbma-neighbor`/`poll-interval` leaves parse and thread through.
- `iface.Start` routes PtMP into the point-to-point ISM branch and NBMA into the broadcast-like (Waiting/DROther + WaitTimer + election) branch.
- The election gate widens to include NBMA; PtMP never elects.
- `shouldAdj` returns true for PtMP and DR-gated for NBMA.
- `SendHello`/the Hello loop unicast/poll on non-multicast interfaces; the §9.4-step-6 Start Hello to priority-0 NBMA neighbours.
- `routerLinks` emits PtMP per-neighbour p2p links + a host-route stub link (not a subnet stub) and no Network-LSA; NBMA originates a Network-LSA when DR.
- `floodDestination`/the initial flood fan out unicast on NBMA / non-broadcast PtMP.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Config:** an operator sets `interfaces/interface/network-type nbma` (with a `nbma-neighbor` list + optional `poll-interval`) or `point-to-multipoint` -> YANG validate -> `parseInterface` -> `interfaceConfig` -> `instance.ifaceConfig` -> `ospfiface.Config`.
- **ISM:** `iface.Start()` selects the per-type initial state and timers.
- **Hello send (clock tick):** `helloLoop` -> `SendHello` -> multicast (broadcast/PtMP-broadcast-variant) or per-neighbour unicast/poll (NBMA / PtMP-non-broadcast).
- **Hello receive:** an incoming Hello -> `receiveHello` -> NSM `hello()` -> election (NBMA) / direct adjacency (PtMP).
- **Origination:** topology change -> `OriginateFromTopology` -> `routerLinks` per-interface (PtMP host-route + p2p links; NBMA transit/stub + Network-LSA when DR).
- **Flood:** an LSA to flood -> `floodExcept` -> per-neighbour unicast fan-out on non-broadcast interfaces.

### Transformation Path
1. **Config resolve:** YANG enum accepts the two new values; `parseInterface` resolves `NetworkType`, the `NBMANeighbors` slice (address + priority), and `PollInterval`. The network-type string is threaded through `ospfiface.Config`, `ospfneighbor.InterfaceConfig`, and `lsdb.InterfaceInfo` (all already string-typed).
2. **ISM init (`iface.Start`):** `point-to-multipoint` -> StatePointToPoint, no election, no WaitTimer, timers armed; `nbma` with priority > 0 -> StateWaiting + WaitTimer + election; `nbma` with priority 0 -> StateDROther; for NBMA the configured neighbours are seeded into the neighbour table in the **Attempt** state (poll-pending) so they are polled before any Hello is heard.
3. **Hello addressing (`SendHello`):** broadcast / PtMP-broadcast-variant -> multicast AllSPFRouters; NBMA / PtMP-non-broadcast -> a per-neighbour unicast loop: HelloInterval to neighbours in state >= Init (heard), PollInterval to neighbours still in Attempt (silent). When this NBMA router is DR/BDR, a Start Hello is also sent to priority-0 neighbours (§9.4 step 6).
4. **DR/BDR election (NBMA only):** `runElectionLocked` is entered (gate widened); the candidate set is self + the configured neighbours in state >= 2-Way; `electDRBDR` runs unchanged; the elected DR originates the Network-LSA. PtMP skips this entirely.
5. **Adjacency (`shouldAdj`):** PtMP -> always adjacent (DD exchange with every neighbour); NBMA -> adjacent only with the DR/BDR (same as broadcast). The DD/LSReq/LSUpdate exchange uses the existing unicast `n.Address` path.
6. **Router-LSA origination (`routerLinks`):** PtMP -> one Type-1 p2p link per Full neighbour (LinkID = neighbour Router ID, LinkData = our interface address, metric = cost) + one host-route Type-3 stub link (LinkID = our interface address, mask 255.255.255.255, metric 0); no subnet stub, no Network-LSA. NBMA -> a Type-2 transit link when DR, a subnet Type-3 stub link, and a Network-LSA when DR (the broadcast origination path).
7. **SPF next-hop (§16.1):** the PtMP destination's interface address is read from its advertised p2p link, giving the direct next-hop, exactly as point-to-point. NBMA resolves next-hops via the transit-network vertex like broadcast.
8. **Flood fan-out (`floodDestination` / initial flood):** on NBMA / non-broadcast PtMP, the initial flood + Ack unicast to each Flood-eligible neighbour (`FloodNeighbors`); on broadcast / PtMP-broadcast-variant, the existing multicast group is used. PtMP has no DR, so the §13.3 DR-relay suppression is skipped (a PtMP node floods to all adjacent neighbours, never relying on a DR).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config (YANG) <-> engine | `network-type` enum + `nbma-neighbor`/`poll-interval` leaves -> `parseInterface` -> `interfaceConfig` -> `ospfiface.Config` | [ ] |
| Config <-> NSM | `NetworkType` string threaded into `ospfneighbor.InterfaceConfig` (already string-typed; new values only) | [ ] |
| ISM <-> Hello send | `iface.Start` per-type init; `SendHello` multicast vs per-neighbour unicast/poll | [ ] |
| NSM <-> adjacency | `shouldAdj` PtMP-always / NBMA-DR-gated; existing unicast DD path reused | [ ] |
| Topology <-> Router-LSA | `routerLinks` PtMP host-route + p2p links; NBMA Network-LSA when DR | [ ] |
| LSDB <-> flooding | `floodDestination` unicast fan-out on non-broadcast interfaces; PtMP no DR-relay | [ ] |
| SPF <-> next-hop | PtMP next-hop from the neighbour's p2p-link interface address (§16.1, reuse p2p path) | [ ] |

### Integration Points
- `internal/plugins/ospf/iface` -- ISM init, election gate, Hello send/poll, the network-type constants (the single delta site for ISM + Hello addressing).
- `internal/plugins/ospf/neighbor` -- `shouldAdj` per-type branch; the existing unicast DD/LSReq path reused; the configured-neighbour seeding for NBMA Attempt state.
- `internal/plugins/ospf/lsdb` -- `routerLinks` PtMP host-route/p2p links; `OriginateNetwork`/`OriginateFromTopology` Network-LSA gate (NBMA yes, PtMP no); `floodDestination`/initial-flood unicast fan-out.
- `internal/plugins/ospf/spf` -- READ-ONLY for PtMP next-hop (reuse the point-to-point neighbour-address next-hop; verify no broadcast-only assumption).
- `internal/plugins/ospf/config.go` + `yang/ze-ospf-conf.yang` -- the enum + the NBMA-only leaves.
- `internal/plugins/ospf/instance.go` -- thread `PollInterval` + the NBMA neighbour list into `ospfiface.Config`.

### Architectural Verification
- [ ] No bypassed layers (config -> resolve -> iface/neighbor/lsdb runtime, the same spine as broadcast/p2p; no new packet type, no new dispatcher)
- [ ] No unintended coupling (the two new types are additional values of an existing leaf and additional branches at existing chokepoints; no new component, no plugin name leaking into generic packages)
- [ ] No duplicated functionality (reuses `electDRBDR`, `OriginateNetwork`, `sendInitialDDLocked`, `neighborAddr`, the point-to-point ISM/NSM/next-hop, `RouterLSA.WriteTo`)
- [ ] Zero-copy / buffer-first preserved (Hello + Router-LSA links encode buffer-first; the unicast Hello fan-out reuses one built buffer per neighbour)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `electDRBDR` / `chooseBDR` / `chooseDeclaredDR` election in `election.go` is network-type-agnostic and can be reused for NBMA by only widening the gate in `iface.go` (no change to `election.go`) | `iface/election.go` is a pure candidate-slice election; `iface.go` `runElectionLocked` returns early unless `NetworkBroadcast` | NBMA needs a separate election path | `TestOSPFNBMAElection` (NBMA elects the same DR a broadcast set would) | unvalidated |
| A-2 | The PtMP point-to-point ISM/NSM path (StatePointToPoint, `shouldAdj` true, p2p Router-LSA link, p2p next-hop) works unchanged for PtMP once `routerLinks` emits per-neighbour p2p links + a host route | `iface.go` `Start` p2p branch; `nsm.go` `shouldAdj` p2p=true; `origination.go` p2p link branch; §16.1 | PtMP needs a distinct ISM/NSM/next-hop path | `TestOSPFPtMPAdjacency`, `TestOSPFPtMPHostRoute`, `TestOSPFPtMPNextHop` | unvalidated |
| A-3 | The DD / LS Request / LS Update retransmit path is ALREADY per-neighbour unicast (`sendInitialDDLocked` -> `n.Address`, `flooding.go neighborAddr`), so only the INITIAL flood + Ack (`floodDestination` multicast) needs a unicast fan-out for non-broadcast interfaces | `neighbor/dd.go` line ~178 unicasts to `n.Address`; `flooding.go` retransmit `sends` loop uses `neighborAddr` | a larger unicast plumbing change is needed | `TestOSPFNBMAFloodUnicast`, `TestOSPFPtMPFloodUnicast` | unvalidated |
| A-4 | A PtMP interface must emit a host-route stub link (mask 255.255.255.255) for its OWN interface address and one Type-1 p2p link per Full neighbour, and must NOT emit the subnet stub link or a Network-LSA | RFC 2328 §12.4.1.4, App A.4.2; guide §7 PtMP prose | other routers cannot reach the PtMP interface address / spurious transit vertex | `TestOSPFPtMPHostRoute` (host route present, subnet stub absent), `TestOSPFPtMPNoNetworkLSA` | unvalidated |
| A-5 | NBMA still elects a DR and the DR still originates the Type-2 Network-LSA, exactly like broadcast (the only NBMA difference at origination is the unicast Hello, not the LSA model) | RFC 2328 §9.4, App A.4.3; guide §7 ("a DR is elected") | NBMA needs a non-DR origination model | `TestOSPFNBMANetworkLSA` (NBMA DR originates a Type-2) | unvalidated |
| A-6 | The configured NBMA neighbour list (address + priority) is sufficient to seed the neighbour table in Attempt state and to drive unicast/poll Hellos; no neighbour is learned by multicast on NBMA | RFC 2328 §10.1 Attempt, §9.5 unicast Hello; guide §7 ("explicit static neighbour list") | NBMA cannot discover neighbours; adjacency never forms | `TestOSPFNBMAPollAttempt`, functional `ospf-nbma.ci` | unvalidated |
| A-7 | Adding `nbma` + `point-to-multipoint` to the v4 YANG enum and the `parseInterface` accept-list, plus the `nbma-neighbor`/`poll-interval` leaves, is the entire config surface; the network-type string already threads end to end | `config.go` `parseInterface`; `instance.go` `ifaceConfig`/`neighborInterfaceConfig`; `flooding.go`/`nsm.go` string constants | more plumbing or a new typed field is needed | package builds; `TestOSPFParseNBMAInterface`, `TestOSPFParsePtMPInterface` | unvalidated |
| A-8 | PtMP/NBMA do not change the Hello option (E/N) match, the DD MTU check, or authentication; the two new types reuse the existing Hello-validate and auth paths | `iface.go` `validateHelloLocked` matches mask only on broadcast; auth is per-interface independent of type | adjacency fails for a new reason / auth regression | existing auth + Hello-validate tests stay green; `TestOSPFNBMAAdjacency` forms Full | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | PtMP host-route omitted or the subnet stub still emitted -> remote routers cannot reach the PtMP interface address, or a phantom subnet route appears | a PtMP neighbour's address is unreachable, or a /<prefix> route shows where only a /32 should | §12.4.1.4 host-route rule in `routerLinks`; `TestOSPFPtMPHostRoute` asserts the /32 present AND the subnet stub absent |
| R-2 | PtMP originates a Network-LSA (or runs an election) -> a spurious transit vertex corrupts SPF | a Type-2 LSA appears for a PtMP segment; SPF builds a network vertex | gate `OriginateNetwork`/election on a real DR (PtMP has none); `TestOSPFPtMPNoNetworkLSA`, `TestOSPFPtMPNoElection` |
| R-3 | `shouldAdj` default stays `false` for PtMP/NBMA -> neighbours stuck at 2-Way, never reach Full | a PtMP/NBMA neighbour never leaves 2-Way; no Router-LSA p2p link | split the `nsm.go` default: PtMP true, NBMA DR-gated; `TestOSPFPtMPAdjacency`, `TestOSPFNBMAAdjacency` |
| R-4 | NBMA never polls a silent configured neighbour (no Attempt state / no PollInterval send) -> adjacency never starts on a quiet link | a configured NBMA neighbour stays Down forever | seed configured neighbours in Attempt and poll at PollInterval; §9.4-step-6 Start Hello to priority-0; `TestOSPFNBMAPollAttempt` |
| R-5 | Non-broadcast flood still multicasts (`floodDestination` unchanged) -> LSAs never reach NBMA/PtMP-non-broadcast neighbours (no multicast on the medium) | a flooded LSA is acked on broadcast tests but silently lost on NBMA | unicast fan-out in `floodDestination`/the initial flood for non-broadcast; `TestOSPFNBMAFloodUnicast`, `TestOSPFPtMPFloodUnicast`; FRR interop confirms LSAs cross |
| R-6 | PtMP next-hop wrong (subnet-derived instead of the neighbour's advertised interface address) -> traffic to a PtMP neighbour mis-steered | a PtMP route installs with the wrong next-hop / a directly-connected /32 missing | §16.1 next-hop from the neighbour p2p link; `TestOSPFPtMPNextHop` asserts the neighbour interface address |
| R-7 | NBMA election widened too far (PtMP accidentally elects) or too little (NBMA never elects) -> wrong LSA model on one type | a PtMP segment elects a DR, or an NBMA DR is never chosen | the gate is exactly `NetworkBroadcast || NetworkNBMA`; `TestOSPFNBMAElection` (elects) + `TestOSPFPtMPNoElection` (does not) |
| R-8 | The new YANG enum value breaks the v6 interface (shared parser) or the config round-trip | a v6 interface accepts `nbma`, or a config diff churns | add the enum ONLY to the v4 leaf; the v6 enum and parser stay `broadcast`/`point-to-point`; `config_test.go` round-trip + a v6 reject test |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| config `network-type point-to-multipoint` on an interface | -> | `parseInterface` accepts it -> `ospfiface.Config.NetworkType` = PtMP -> `iface.Start` takes the PtMP ISM branch (no election) | `TestOSPFParsePtMPInterface` (unit) + `test/ospf/ospf-ptmp.ci` |
| config `network-type nbma` + a `nbma-neighbor` list | -> | `parseInterface` resolves the neighbour list + `poll-interval` -> seeded in Attempt -> unicast/poll Hello | `TestOSPFParseNBMAInterface` (unit) + `test/ospf/ospf-nbma.ci` |
| a PtMP interface reaches Full with a neighbour | -> | `routerLinks` emits a Type-1 p2p link + a host-route stub; no Network-LSA | `TestOSPFPtMPHostRoute`, `TestOSPFPtMPNoNetworkLSA` |
| an NBMA interface with eligible neighbours comes up | -> | the election gate admits NBMA -> `electDRBDR` -> a DR is elected -> Network-LSA originated | `TestOSPFNBMAElection`, `TestOSPFNBMANetworkLSA` |
| an LSA is flooded on an NBMA / non-broadcast interface | -> | `floodDestination`/the initial flood unicasts to each Flood-eligible neighbour | `TestOSPFNBMAFloodUnicast`, `test/ospf/ospf-nbma.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An interface configured `network-type point-to-multipoint` | accepted by YANG + `parseInterface`; the iface enters the point-to-point ISM state (no Waiting, no WaitTimer); no DR/BDR is elected; `show ip ospf interface` reports network type `point-to-multipoint` |
| AC-2 | A PtMP interface forms an adjacency with a neighbour | `should_adj` is true -> the adjacency proceeds to Full (no DR gating); the existing unicast DD/LSReq/LSUpdate exchange completes |
| AC-3 | A PtMP interface at Full with neighbour N (interface address X) | the Router-LSA contains a Type-1 (point-to-point) link with LinkID = N's Router ID, LinkData = our interface address, metric = interface cost; AND a host-route Type-3 stub link with LinkID = our interface address, mask 255.255.255.255, metric 0 |
| AC-4 | A PtMP interface | NO subnet (network-prefix) stub link and NO Type-2 Network-LSA is originated for that interface |
| AC-5 | SPF computes the route to a PtMP neighbour's address | the next-hop is the neighbour's interface address taken from its advertised p2p link (§16.1), not a subnet-derived next-hop |
| AC-6 | An interface configured `network-type nbma` with a `nbma-neighbor` list (addresses + priorities) and `poll-interval` | accepted by YANG + `parseInterface`; the configured neighbours are seeded in the Attempt state; an eligible (priority > 0) NBMA interface enters Waiting and arms the WaitTimer |
| AC-7 | An NBMA interface sending Hellos | Hellos are sent unicast to each configured neighbour: at HelloInterval to neighbours heard from (state >= Init), at PollInterval (default 120 s) to silent neighbours (Attempt); no multicast Hello is sent on the NBMA interface |
| AC-8 | An NBMA segment with two or more eligible routers | a DR (and BDR) is elected using the §9.4 election (reusing `electDRBDR`); the elected DR originates the Type-2 Network-LSA exactly as on a broadcast segment |
| AC-9 | This NBMA router becomes DR or BDR, and a configured neighbour has priority 0 | a Start (Hello) is sent to that priority-0 neighbour so the adjacency begins (§9.4 step 6); the priority-0 neighbour is ineligible for the election but still adjacent to the DR/BDR |
| AC-10 | An LSA flooded on an NBMA or non-broadcast PtMP interface | it is sent unicast to each Flood-eligible neighbour (Exchange/Loading/Full), not to a multicast group; the neighbour acknowledges and the LSA installs |
| AC-11 | An NBMA adjacency reaches Full | `should_adj` admits only the DR/BDR (the broadcast rule); a DROther-to-DROther pair on NBMA stays at 2-Way (no adjacency), exactly as broadcast |
| AC-12 | A broadcast or point-to-point interface (regression) | behaves byte-for-byte as before this spec: broadcast still multicasts Hellos/floods, elects, and originates a Network-LSA; point-to-point still emits per-neighbour p2p links + a subnet stub |
| AC-13 | The v6 (OSPFv3) interface config | the v6 `network-type` enum still rejects `nbma`/`point-to-multipoint` (only `broadcast`/`point-to-point` are valid for v6) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a hub-and-spoke link `network-type point-to-multipoint` and expects each spoke reachable as a /32 host route | config -> PtMP ISM -> Full adjacency per neighbour -> `routerLinks` host-route + p2p links -> SPF /32 next-hop -> kernel | `test/ospf/ospf-ptmp.ci` |
| 2 | Configures a Frame-Relay-style `network-type nbma` with a static neighbour list and expects a DR elected and adjacencies formed without multicast | config -> NBMA Attempt/poll -> unicast Hello -> election -> DR Network-LSA -> Full with DR/BDR | `test/ospf/ospf-nbma.ci` |
| 3 | Adds a priority-0 NBMA neighbour and expects it adjacent to the DR | config -> election makes this router DR/BDR -> §9.4-step-6 Start Hello to the priority-0 neighbour -> adjacency forms | `test/ospf/ospf-nbma.ci` (priority-0 step) |
| 4 | Runs `show ip ospf interface` on the NBMA/PtMP interface | CLI -> interface snapshot -> network type + (NBMA) poll/neighbour state rendered | `test/ospf/ospf-nbma.ci` / `ospf-ptmp.ci` (show step) |
| 5 | Peers a PtMP/NBMA Ze interface with FRR `ospfd` of the matching type | wire (unicast/multicast Hello + unicast flood) -> Full adjacency -> LSDB sync -> routes both ways | `test/interop/scenarios/ospf-ptmp-frr/`, `ospf-nbma-frr/` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFParsePtMPInterface` | `internal/plugins/ospf/config_test.go` | AC-1, A-7: `point-to-multipoint` accepted; `interfaceConfig.NetworkType` set | |
| `TestOSPFParseNBMAInterface` | `internal/plugins/ospf/config_test.go` | AC-6, A-7: `nbma` + `nbma-neighbor` list + `poll-interval` parsed | |
| `TestOSPFRejectV6NBMA` | `internal/plugins/ospf/config_test.go` | AC-13, R-8: v6 interface rejects `nbma`/`point-to-multipoint` | |
| `TestOSPFPtMPISMNoElection` | `internal/plugins/ospf/iface/iface_test.go` | AC-1, R-7: PtMP enters PointToPoint state, no Waiting, no election | |
| `TestOSPFNBMAISMWaiting` | `internal/plugins/ospf/iface/iface_test.go` | AC-6: eligible NBMA enters Waiting + WaitTimer; priority 0 -> DROther | |
| `TestOSPFNBMAElection` | `internal/plugins/ospf/iface/iface_test.go` | AC-8, A-1, R-7: NBMA runs `electDRBDR`; elects the same DR a broadcast set would | |
| `TestOSPFPtMPNoElection` | `internal/plugins/ospf/iface/iface_test.go` | AC-1/AC-4, R-2/R-7: PtMP never elects, never sets DR/BDR | |
| `TestOSPFNBMAUnicastHello` | `internal/plugins/ospf/iface/iface_test.go` | AC-7: NBMA Hello sent unicast per configured neighbour; no multicast | |
| `TestOSPFNBMAPollAttempt` | `internal/plugins/ospf/iface/iface_test.go` | AC-7, R-4, A-6: silent neighbour polled at PollInterval (Attempt); heard at HelloInterval | |
| `TestOSPFNBMAStartHelloPriorityZero` | `internal/plugins/ospf/iface/iface_test.go` | AC-9: DR/BDR sends Start Hello to a priority-0 neighbour (§9.4 step 6) | |
| `TestOSPFShouldAdjPtMP` / `TestOSPFShouldAdjNBMA` | `internal/plugins/ospf/neighbor/nsm_test.go` | AC-2/AC-11, R-3: PtMP always adjacent; NBMA DR/BDR-gated | |
| `TestOSPFPtMPAdjacency` | `internal/plugins/ospf/neighbor/nsm_test.go` | AC-2: PtMP neighbour reaches Full via the unicast DD path | |
| `TestOSPFNBMAAdjacency` | `internal/plugins/ospf/neighbor/nsm_test.go` | AC-11, A-8: NBMA reaches Full only with DR/BDR; DROther pair stays 2-Way | |
| `TestOSPFPtMPHostRoute` | `internal/plugins/ospf/lsdb/origination_test.go` | AC-3/AC-4, A-4, R-1: PtMP emits per-neighbour p2p link + /32 host-route stub; NO subnet stub | |
| `TestOSPFPtMPNoNetworkLSA` | `internal/plugins/ospf/lsdb/origination_test.go` | AC-4, R-2: PtMP originates no Type-2 Network-LSA | |
| `TestOSPFNBMANetworkLSA` | `internal/plugins/ospf/lsdb/origination_test.go` | AC-8, A-5: NBMA DR originates a Type-2 Network-LSA | |
| `TestOSPFNBMAFloodUnicast` | `internal/plugins/ospf/lsdb/flooding_test.go` | AC-10, A-3, R-5: NBMA initial flood + Ack unicast to each Flood-eligible neighbour | |
| `TestOSPFPtMPFloodUnicast` | `internal/plugins/ospf/lsdb/flooding_test.go` | AC-10, R-5: non-broadcast PtMP floods unicast; no DR-relay suppression | |
| `TestOSPFPtMPNextHop` | `internal/plugins/ospf/spf/spf_test.go` | AC-5, A-2, R-6: PtMP next-hop = neighbour's advertised interface address | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `poll-interval` (uint16 seconds) | 1..65535 | 65535 | 0 (rejected; must be > 0) | N/A (16-bit field) |
| `nbma-neighbor` priority (uint8) | 0..255 | 255 | N/A | N/A (1 byte); 0 = ineligible (polled, not elected) |
| PtMP host-route mask | 255.255.255.255 only | 255.255.255.255 | N/A | N/A (fixed /32) |
| PtMP host-route metric | 0 | 0 | N/A | N/A (RFC 2328 §12.4.1.4 host route cost 0) |
| network-type enum | {broadcast, point-to-point, nbma, point-to-multipoint, loopback} | n/a | unknown string rejected by `parseInterface` | n/a |
| NBMA configured-neighbour count | 0..maxNeighbors (1024) | 1024 | N/A | beyond 1024 hits the existing neighbour-limit guard |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-nbma` | `test/ospf/ospf-nbma.ci` | NBMA interface with a static neighbour list elects a DR, polls a silent neighbour, forms Full, originates a Network-LSA, and floods unicast | |
| `ospf-ptmp` | `test/ospf/ospf-ptmp.ci` | PtMP interface forms Full with each neighbour, emits /32 host routes + p2p links, no Network-LSA, no DR | |
| `ospf-nbma-config` | `test/ospf/ospf-nbma-config.ci` | config round-trip of `network-type nbma` + `nbma-neighbor` + `poll-interval`; invalid values rejected; `show ip ospf interface` renders | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-ptmp-frr` | `test/interop/scenarios/ospf-ptmp-frr/` | FRR `ospfd` (`ip ospf network point-to-multipoint`) | Ze and FRR form PtMP adjacencies (no DR), exchange host-route LSAs, and install each other's /32s; next-hops resolve to the neighbour interface address | |
| `ospf-nbma-frr` | `test/interop/scenarios/ospf-nbma-frr/` | FRR `ospfd` (`ip ospf network non-broadcast` + `neighbor` statements) | Ze and FRR elect a consistent DR over a static neighbour list, exchange unicast Hellos/floods, the DR originates the Network-LSA, and routes converge both ways | |

> Interop is required: this changes wire behaviour (unicast Hello addressing, the
> NBMA election + Network-LSA over a static list, PtMP host-route LSAs, unicast
> flooding). The raw-IP / unicast paths are Linux-only and run as QEMU integration
> tests (`ai/rules/qemu-testing.md`), consistent with the rest of the OSPF interop set.

### Future (if deferring any tests)
- None. Every AC is covered by a unit, functional, or interop test above. RFC 6845 hybrid broadcast-and-PtMP and the OSPFv3 PtMP variant are explicitly out of scope (no test owed here).

## Files to Modify
<!-- MUST include feature code, not only test files -->
- `internal/plugins/ospf/iface/ism.go` -- add `NetworkNBMA`/`NetworkPointToMultipoint` string constants
- `internal/plugins/ospf/iface/iface.go` -- `Start()` per-type ISM init (PtMP -> point-to-point branch; NBMA -> Waiting/DROther + WaitTimer); widen the election gate in `runElectionLocked` to include NBMA; `SendHello`/`buildHelloPacket`/the Hello loop unicast/poll fan-out + the §9.4-step-6 Start Hello; seed configured NBMA neighbours in Attempt
- `internal/plugins/ospf/neighbor/nsm.go` -- split `shouldAdj` default: `point-to-multipoint` true, `nbma` DR/BDR-gated; add the `NetworkNBMA`/`NetworkPointToMultipoint` constants (neighbor-package copy)
- `internal/plugins/ospf/neighbor/table.go` -- ensure the configured-neighbour Attempt seeding + poll interact correctly with `hello()` (no NSM rule change beyond `shouldAdj`)
- `internal/plugins/ospf/lsdb/flooding.go` -- add the `NetworkNBMA`/`NetworkPointToMultipoint` constants (lsdb-package copy); `floodDestination`/the initial-flood + Ack path unicast fan-out on non-broadcast interfaces; PtMP no DR-relay suppression in `floodExcept`
- `internal/plugins/ospf/lsdb/origination.go` -- `routerLinks` PtMP branch (per-neighbour Type-1 p2p link + host-route Type-3 stub, no subnet stub); gate Network-LSA origination off PtMP (PtMP has no DR)
- `internal/plugins/ospf/config.go` -- extend the `parseInterface` accept-list with `nbma`/`point-to-multipoint`; add `networkNBMA`/`networkPointToMultipoint` constants; add `NBMANeighbors []nbmaNeighborConfig` + `PollInterval uint16` to `interfaceConfig` and parse them
- `internal/plugins/ospf/instance.go` -- thread `PollInterval` + the NBMA neighbour list (address + priority) into `ospfiface.Config`; thread network type unchanged
- `internal/plugins/ospf/iface/iface.go` `Config` -- add `PollInterval uint16` + `NBMANeighbors []NBMANeighbor{Address [4]byte; Priority uint8}`
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- add `enum nbma;` + `enum point-to-multipoint;` to the v4 `interfaces/interface/network-type`; add a `nbma-neighbor` list (key `address`, leaf `priority` default 0) + a `poll-interval` leaf (uint16, default 120, units seconds)
- `internal/plugins/ospf/spf/` -- verify (and adjust only if needed) the PtMP next-hop reads the neighbour's interface address from its p2p link (§16.1); reuse the point-to-point next-hop, no new path expected
- `internal/plugins/ospf/cmd_show.go` / `show_summary.go` -- render NBMA poll/neighbour state in `show ip ospf interface` if the existing snapshot does not already surface it

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `yang/ze-ospf-conf.yang` -- two enum values + `nbma-neighbor` list + `poll-interval` leaf; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | `poll-interval` `range "1..65535"`; `nbma-neighbor` priority `range "0..255"`; `address` `ze:validate` an IPv4 (reuse the existing IPv4 validator if present) |
| YANG custom validators | [ ] check | if no IPv4-address validator exists, add a `CompleteFn`/`ValidateFn` for `nbma-neighbor/address`; otherwise reuse |
| CLI commands/flags | [ ] no | reuses `show ip ospf interface`; no new command (NBMA/PtMP are config, not a new verb) |
| CLI grammar (action before identifier) | [ ] n/a | no new command |
| Editor autocomplete | [ ] yes | automatic for the YANG enum + `poll-interval`; `CompleteFn` for `nbma-neighbor/address` if added |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-nbma*.ci`, `ospf-ptmp.ci` |
| Pipe completeness | [ ] yes | `show ip ospf interface` already routes through `ApplyPipes`; no new output path |
| Env var registration | [ ] no | operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] no | no new socket/port/binary; reuses the existing OSPF raw socket |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospf_nbma_neighbors` | gauge | `interface`, `state` (attempt/heard) |
| `ze_ospf_nbma_polls_total` | counter | `interface` |
| `ze_ospf_ptmp_host_routes` | gauge | `interface` |

> These extend the umbrella's canonical OSPF metric set, use the `ze_ospf_*`
> prefix, and are registered by this spec's owner code. The umbrella "Metrics"
> table gains these rows when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPF NBMA + point-to-multipoint network types |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` -- `network-type nbma|point-to-multipoint`, `nbma-neighbor`, `poll-interval` |
| 3 | CLI command added/changed? | [ ] check | `docs/guide/command-reference.md` -- `show ip ospf interface` NBMA/PtMP fields if rendered |
| 4 | API/RPC added/changed? | [ ] no | reuses the existing interface-show RPC |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains NBMA/PtMP |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- network-types section (NBMA + PtMP) |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospf.md` (or equivalent) -- unicast Hello addressing, PtMP host-route LSA links, unicast flooding |
| 8 | Plugin SDK/protocol changed? | [ ] no | no SDK surface change |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc2328.md` -- tick the NBMA/PtMP-relevant compliance items |
| 10 | Test infrastructure changed? | [ ] yes (interop scenarios added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPF network-type parity with FRR (all four wire types) |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- network-type branching at the ISM/NSM/origination/flood chokepoints |
| 13 | Route metadata keys added/changed? | [ ] no | PtMP installs host routes through the existing OSPF route path; no new meta key |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the three `ze_ospf_nbma_*`/`ze_ospf_ptmp_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] check | umbrella metrics table + `docs/plugin-overview.md` if the metric inventory is listed |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed OSPF iface/neighbor/lsdb/config files |
| 17 | Existing docs show examples for this area? | [ ] check | verify OSPF interface config examples against the extended `network-type` enum |

## Files to Create
- `internal/plugins/ospf/iface/nbma.go` -- the NBMA Hello unicast/poll loop + the Attempt seeding + the §9.4-step-6 Start Hello (kept out of the broadcast-heavy `iface.go` core)
- `internal/plugins/ospf/iface/nbma_test.go`, additions to `iface_test.go`
- `internal/plugins/ospf/neighbor/nsm_test.go` additions (`TestOSPFShouldAdjPtMP`, `TestOSPFShouldAdjNBMA`, `TestOSPFPtMPAdjacency`, `TestOSPFNBMAAdjacency`)
- `internal/plugins/ospf/lsdb/origination_test.go` additions (`TestOSPFPtMPHostRoute`, `TestOSPFPtMPNoNetworkLSA`, `TestOSPFNBMANetworkLSA`)
- `internal/plugins/ospf/lsdb/flooding_test.go` additions (`TestOSPFNBMAFloodUnicast`, `TestOSPFPtMPFloodUnicast`)
- `internal/plugins/ospf/spf/spf_test.go` addition (`TestOSPFPtMPNextHop`)
- `internal/plugins/ospf/config_test.go` additions (`TestOSPFParsePtMPInterface`, `TestOSPFParseNBMAInterface`, `TestOSPFRejectV6NBMA`)
- `test/ospf/ospf-nbma.ci`, `test/ospf/ospf-ptmp.ci`, `test/ospf/ospf-nbma-config.ci`
- `test/interop/scenarios/ospf-ptmp-frr/` -- `ze.conf`, `frr.conf`, `check.py`
- `test/interop/scenarios/ospf-nbma-frr/` -- `ze.conf`, `frr.conf`, `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm the broadcast election, p2p ISM/NSM, Network-LSA, and unicast retransmit paths exist |
| 3. Wiring phase | Wiring Test table -- enum + parse + failing wiring tests |
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

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- config surface + failing wiring tests
   - Tests: `TestOSPFParsePtMPInterface`, `TestOSPFParseNBMAInterface`, `TestOSPFRejectV6NBMA`, `test/ospf/ospf-nbma-config.ci`
   - Files: `yang/ze-ospf-conf.yang` (enum + `nbma-neighbor` + `poll-interval`), `config.go` (accept-list + new struct fields + parse), `iface/ism.go` (constants), the `neighbor`/`lsdb` constant copies; thread through `instance.go`
   - Verify: the two new network types parse and reach the iface runtime; the behaviour branches are stubs so the deeper tests still fail
2. **Phase: PtMP ISM/NSM + adjacency** -- treat PtMP as point-to-point
   - Tests: `TestOSPFPtMPISMNoElection`, `TestOSPFPtMPNoElection`, `TestOSPFShouldAdjPtMP`, `TestOSPFPtMPAdjacency`
   - Files: `iface/iface.go` (`Start` PtMP -> point-to-point branch, no election), `neighbor/nsm.go` (`shouldAdj` PtMP true)
   - Verify: PtMP forms Full with every neighbour and never elects
3. **Phase: PtMP origination + next-hop** -- host route + p2p links, no Network-LSA
   - Tests: `TestOSPFPtMPHostRoute`, `TestOSPFPtMPNoNetworkLSA`, `TestOSPFPtMPNextHop`
   - Files: `lsdb/origination.go` (`routerLinks` PtMP branch + Network-LSA gate), `spf/` (verify next-hop)
   - Verify: PtMP emits per-neighbour p2p links + a /32 host route, no subnet stub, no Network-LSA; SPF next-hop = neighbour interface address
4. **Phase: NBMA ISM + election + Network-LSA** -- broadcast-like over a static list
   - Tests: `TestOSPFNBMAISMWaiting`, `TestOSPFNBMAElection`, `TestOSPFShouldAdjNBMA`, `TestOSPFNBMAAdjacency`, `TestOSPFNBMANetworkLSA`
   - Files: `iface/iface.go` (election gate widened, Waiting/DROther init, NBMA neighbour seeding), `neighbor/nsm.go` (`shouldAdj` NBMA DR-gated), `lsdb/origination.go` (Network-LSA when DR)
   - Verify: NBMA elects a DR, forms Full only with DR/BDR, originates a Network-LSA
5. **Phase: NBMA unicast Hello + poll + Start** -- the Attempt-state Hello model
   - Tests: `TestOSPFNBMAUnicastHello`, `TestOSPFNBMAPollAttempt`, `TestOSPFNBMAStartHelloPriorityZero`
   - Files: `iface/nbma.go` (unicast/poll loop, Start Hello), `iface/iface.go` (Hello-send dispatch by type)
   - Verify: Hellos unicast per neighbour, polled at PollInterval, Start Hello to priority-0 when DR/BDR
6. **Phase: Non-broadcast flooding** -- unicast fan-out
   - Tests: `TestOSPFNBMAFloodUnicast`, `TestOSPFPtMPFloodUnicast`
   - Files: `lsdb/flooding.go` (`floodDestination`/initial-flood + Ack unicast fan-out; PtMP no DR-relay)
   - Verify: LSAs reach each Flood-eligible neighbour by unicast on non-broadcast interfaces
7. **Phase: CLI/show + metrics** -- user surface
   - Tests: the `.ci` show steps
   - Files: `cmd_show.go`/`show_summary.go` (NBMA poll/neighbour state), metric registration
   - Verify: `show ip ospf interface` renders NBMA/PtMP; the three metric series register
8. **Functional tests** -> `ospf-nbma.ci`, `ospf-ptmp.ci`, `ospf-nbma-config.ci`
9. **RFC refs** -> add `// RFC 2328 Section 9.5 / 10.1 / 12.4.1.4 / 16.1` comments on the unicast-Hello, Attempt, host-route, and next-hop code
10. **Interop** -> `ospf-ptmp-frr` + `ospf-nbma-frr` QEMU scenarios
11. **Full verification** -> `make ze-verify`
12. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; all four RFC 2328 wire network types now exist (parity with FRR's broadcast/p2p/PtMP/NBMA) |
| Correctness | PtMP: no DR/no Network-LSA, host-route /32, per-neighbour p2p link, neighbour-address next-hop. NBMA: election reused, unicast/poll Hello, Start Hello to priority-0, DR Network-LSA, unicast flood |
| Naming | YANG `nbma`/`point-to-multipoint`/`nbma-neighbor`/`poll-interval` kebab-case; `ze_ospf_nbma_*`/`ze_ospf_ptmp_*` metrics; `NetworkNBMA`/`NetworkPointToMultipoint` constants consistent across iface/neighbor/lsdb |
| Data flow | the network-type string threads unchanged; the new branches live only at `iface.Start`, the election gate, `shouldAdj`, `routerLinks`, `floodDestination`; no new component |
| CLI grammar | no new command (reuses `show ip ospf interface`) |
| Doctor checks | none added (no new runtime dependency) -- confirm |
| YANG validation | the two enum values added to v4 ONLY; `poll-interval` range; `nbma-neighbor` priority range + address validation; v6 enum unchanged |
| Prometheus counters | the three series defined, registered, listed; umbrella table updated |
| Rule: plugin-self-containment | all behaviour stays inside `internal/plugins/ospf`; no NBMA/PtMP spelling in generic packages |
| Rule: buffer-first | Hello + Router-LSA links + unicast flood encode buffer-first; one built buffer reused per unicast neighbour |
| Regression | broadcast + point-to-point behave exactly as before (AC-12); v6 rejects the new values (AC-13) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Two new network types parse | `go test ./internal/plugins/ospf -run 'TestOSPFParse(NBMA|PtMP)Interface'` |
| PtMP host-route + no Network-LSA | `go test ./internal/plugins/ospf/lsdb -run 'TestOSPFPtMP(HostRoute|NoNetworkLSA)'` |
| NBMA election + Network-LSA | `go test ./internal/plugins/ospf/... -run 'TestOSPFNBMA(Election|NetworkLSA)'` |
| Unicast Hello + poll | `go test ./internal/plugins/ospf/iface -run 'TestOSPFNBMA(UnicastHello|PollAttempt|StartHelloPriorityZero)'` |
| Non-broadcast unicast flood | `go test ./internal/plugins/ospf/lsdb -run 'TestOSPF(NBMA|PtMP)FloodUnicast'` |
| Three metric series registered | `grep -rn 'ze_ospf_nbma_\|ze_ospf_ptmp_' internal/plugins/ospf` |
| Functional tests present | `ls test/ospf/ospf-nbma.ci test/ospf/ospf-ptmp.ci test/ospf/ospf-nbma-config.ci` |
| Interop scenarios present | `ls test/interop/scenarios/ospf-ptmp-frr/ test/interop/scenarios/ospf-nbma-frr/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | the `nbma-neighbor` address must validate as IPv4 (reject malformed); the configured neighbour list is bounded by the existing `maxNeighbors` (1024) guard so a huge list cannot exhaust memory |
| Resource exhaustion | the unicast Hello/flood fan-out is O(neighbours); cap at `maxNeighbors`; the poll timer cannot schedule faster than PollInterval; a flood to N neighbours reuses one buffer |
| Trust boundary | NBMA neighbours are operator-configured (no dynamic discovery), so an off-path host cannot inject itself; received Hellos still pass the existing per-interface authentication |
| Error leakage | a failed unicast send to one neighbour is logged/counted, not fatal; it does not abort the fan-out to the others |
| Spoofing | unicast Hello source is verified against the configured neighbour address before accepting on NBMA (a Hello from an unconfigured source on NBMA is ignored per §9.5) |

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
<!-- LIVE -->

## Core Insight
NBMA and point-to-multipoint are not new subsystems: NBMA is the existing
broadcast election + Network-LSA over a manually configured neighbour list with
unicast/poll Hellos, and PtMP is the existing point-to-point ISM/NSM/next-hop with
per-neighbour p2p Router-LSA links plus a host-route stub and no DR. The work is
two new enum values and per-type branches at five existing chokepoints
(`iface.Start`, the election gate, `shouldAdj`, `routerLinks`,
`floodDestination`), reusing the broadcast election, the Network-LSA origination,
the p2p next-hop, and the already-unicast adjacency-packet path.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| NBMA reuses `electDRBDR` by only widening the `iface.go` gate | a separate NBMA election | the §9.4 election is identical for broadcast and NBMA; only the candidate source (configured list) and Hello transport (unicast) differ |
| PtMP reuses the point-to-point ISM/NSM/next-hop | a dedicated PtMP ISM | RFC 2328 defines PtMP precisely as "a collection of point-to-point links"; the only origination delta is the host-route stub |
| Host-route stub link (mask /32) instead of the subnet stub on PtMP | advertise the subnet | §12.4.1.4: a PtMP interface advertises only a host route to its own address so each spoke is independently reachable (no shared subnet semantics) |
| Add the two enum values to the v4 leaf ONLY | extend the v6 enum too | OSPFv3 has no NBMA and a different PtMP; the umbrella scopes this to v4 |
| Unicast flood fan-out only at `floodDestination`/initial flood | rewrite the flood path | the retransmit path is already unicast (`neighborAddr`); only the initial flood/ack used a multicast group |
| Keep the NBMA Hello/poll loop in a new `iface/nbma.go` | inline in `iface.go` | keeps the broadcast-heavy core readable and the NBMA-specific Attempt/poll/Start logic self-contained |

## Known Limitations
- Virtual links (RFC 2328 §15) remain out of scope (spec-ospf-ext-7); an NBMA/PtMP interface cannot serve as a virtual-link transit here.
- OSPFv3 NBMA/PtMP is not implemented (RFC 5340 has no NBMA; PtMP differs); the v6 enum stays `broadcast`/`point-to-point`.
- The RFC 6845 hybrid broadcast-and-PtMP interface type is not this PtMP; it is future.
- The non-broadcast PtMP variant uses the same `nbma-neighbor` list for its explicit neighbour set; the broadcast PtMP variant (multicast Hello) is the default.

## RFC Documentation

Add `// RFC 2328 Section X.Y: "<quoted requirement>"` above the enforcing code:
- §9.5 unicast Hello on NBMA (HelloInterval to eligible, PollInterval to ineligible) and Hello-to-all on PtMP
- §9.4 step 6 Start Hello to priority-0 NBMA neighbours when this router becomes DR/BDR
- §10.1 / §10.3 the NBMA-only Attempt state and Start event
- §10.4 `should_adj`: always adjacent on PtMP, DR/BDR-only on NBMA
- §12.4.1.4 / App A.4.2 the PtMP host-route stub link + per-neighbour p2p links; no Network-LSA on PtMP
- §16.1 PtMP next-hop from the neighbour's advertised interface address
- §13.3 unicast flooding on non-broadcast interfaces

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

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|-----------|-------|

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
| NBMA: manual neighbour list + DR election + unicast/poll Hello | functional + interop | `ospf-nbma.ci`, `ospf-nbma-frr` |
| NBMA: DR originates the Network-LSA | unit | `TestOSPFNBMANetworkLSA` |
| PtMP: per-neighbour adjacency, no DR, host-route origination | functional + interop | `ospf-ptmp.ci`, `ospf-ptmp-frr` |
| PtMP: /32 host route + p2p links, no Network-LSA | unit | `TestOSPFPtMPHostRoute`, `TestOSPFPtMPNoNetworkLSA` |
| Non-broadcast unicast flooding | unit + interop | `TestOSPFNBMAFloodUnicast`, `ospf-nbma-frr` |
| Broadcast/p2p + v6 unchanged (regression) | unit + existing suite | `TestOSPFRejectV6NBMA`, existing OSPF tests green |

## Review Gate

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
- [ ] AC-1..AC-13 all demonstrated
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
- [ ] RFC 2328 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (two new enum values + branches; no new subsystem)
- [ ] No speculative features (only RFC 2328 NBMA + PtMP; no RFC 6845 hybrid, no v6 PtMP)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (behaviour stays inside the OSPF plugin)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-ptmp-frr`, `ospf-nbma-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-8-nbma-p2mp.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-8-nbma-p2mp.md`
