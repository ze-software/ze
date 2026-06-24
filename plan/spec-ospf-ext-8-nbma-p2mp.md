# Spec: OSPF NBMA + Point-to-Multipoint Network Types (both address families)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospf-ext-0-umbrella.md |
| Phase | - |
| Updated | 2026-06-24 |

> **One engine, two address families.** Ze implements OSPF as a single unified
> engine (`internal/plugins/ospf/`), exactly as `bgp` is one engine spanning
> address families. There is no separate OSPFv3 plugin. The IPv4 family is
> OSPFv2 (RFC 2328); the IPv6 family is OSPFv3 (RFC 5340), which runs as a second
> instance of the same engine over the `_v6` codec. The FSM, flooding, DR
> election, SPF, and LSDB sequencing are address-family-neutral and SHARED; the
> AF-specific wire/LSA/prefix code lives in the `_v6` strategy files and the
> `internal/plugins/ospf/v3/{types,packet,transport}` leaf packages. This spec
> adds the same two network types to BOTH families: the shared engine behaviour
> is stated once; the per-family wire/LSA differences are labelled with an
> "Address family" column or explicit IPv4/IPv6 sub-rows.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/learned/972-ospf-af-unify.md` -- the AF-unification decision: IPv4/IPv6 are two address families of ONE OSPF engine; the shared ISM/NSM/election/SPF is AF-neutral and string-keyed on the network type; the `_v6` files own the IPv6 wire/LSA/prefix code; there is no separate `ospfv3` plugin
4. `rfc/short/rfc2328.md` (IPv4 family) -- §9.1-9.3 ISM (NBMA/PtMP transitions), §9.4 DR/BDR election (NBMA Start-to-priority-0 step), §9.5 sending Hellos (NBMA/PtMP unicast), §10.1-10.5 NSM (Attempt state, Start event, `should_adj`), §12.4.1.4 PtMP host-route, §13.3 flooding (non-broadcast unicast), §16.1 next-hop, App A.4.2/A.4.3 Router-LSA links, App C.5/C.6 NBMA/PtMP configurables (PollInterval, static neighbour list)
5. `rfc/short/rfc5340.md` (IPv6 family) -- §2.1 link (not subnet) model, §2.9 transport (`ff02::5`/`ff02::6`, link-local source, unicast retransmit), §3.8.1 next-hop from adjacency link-local, §A.3.2 Hello (Interface ID, no network mask), §A.4.3 Router-LSA (address-free p2p/transit links keyed by Router ID + Interface ID), §A.4.4 Network-LSA (DR-originated on broadcast/NBMA), §A.4.10 Intra-Area-Prefix-LSA (where IPv6 prefixes including the PtMP /128 host route live), §A.4.1 prefix encoding (LA-bit, `((PrefixLength+31)/32)` words)
6. `plan/spec-ospf-ext-0-umbrella.md` -- the extension umbrella: the shared `network-type` contract (currently `broadcast`/`point-to-point`; this spec ADDS `nbma` + `point-to-multipoint` per family) and the deferred row this spec closes
7. `internal/plugins/ospf/iface/iface.go` -- the AF-shared per-interface runtime: `Start()` network-type switch (the ISM-init chokepoint), `runElectionLocked()` (today gated `NetworkType != NetworkBroadcast`), `SendHello()` (today multicast only; v6 sends to `allSPFRoutersV6`), `buildHelloPacket()`, `validateHelloLocked()`, the `Config` struct (carries `NetworkType`/`IsV6`/`Priority`), `Sender.SendPacket(name, dst, payload)`
8. `internal/plugins/ospf/iface/ism.go` -- the `State` enum + the `NetworkBroadcast`/`NetworkPointToPoint`/`NetworkLoopback` string constants this spec extends (shared, AF-neutral)
9. `internal/plugins/ospf/iface/election.go` -- `electDRBDR`/`chooseBDR`/`chooseDeclaredDR`/`betterCandidate` (reused verbatim for NBMA in BOTH families; only the election GATE in `iface.go` changes)
10. `internal/plugins/ospf/neighbor/nsm.go` -- `shouldAdj` (today `point-to-point` true, `broadcast`/`""` DR/BDR-only, default false -- the default must split: PtMP true, NBMA DR-gated, shared by both families), `startExchange`
11. `internal/plugins/ospf/neighbor/table.go` -- `hello()` (NSM driver), `sendInitialDDLocked`/`resendLastDDLocked` (already unicast to `n.Address`), `FloodNeighbors`/`AcceptsFlooding`
12. `internal/plugins/ospf/lsdb/flooding.go` -- `floodDestination` (multicast group: AllSPFRouters/AllDRouters for IPv4, `ff02::5`/`ff02::6` for IPv6; non-broadcast must fan out unicast), `floodExcept`, `eligibleInterface`, `neighborAddr` (already unicast for retransmit); `InterfaceInfo`/`NeighborInfo` carry `IsV6`, `NetworkType`, `IPv6LinkLocal`, `InterfaceID`
13. `internal/plugins/ospf/lsdb/origination.go` (IPv4 family) -- `routerLinks` (per-interface link records), `OriginateNetwork`/`OriginateFromTopology` (Network-LSA when DR)
14. `internal/plugins/ospf/origination_v6.go` (IPv6 family) -- `v6RouterLSABody` (switches `NetworkPointToPoint`/`NetworkBroadcast` only), `v6OriginateNetwork` (DR `0x2002` Network-LSA), `v6OriginateSelf` (self-LSA sweep + the Network-Intra-Area-Prefix gate currently `NetworkType != NetworkBroadcast`)
15. `internal/plugins/ospf/origination_v6_link.go` (IPv6 family) -- `v6ShouldOriginateLinkLSA` (gated `NetworkBroadcast || NetworkPointToPoint`), the Intra-Area-Prefix host-route path
16. `internal/plugins/ospf/afstrategy_v6.go` (IPv6 family) -- `v6NextHop.P2PNextHop`/`TransitNextHop` (next-hop = neighbour link-local via `neighbors.AddressOf`), `BuildGraph`/`v6RouterLinks`
17. `internal/plugins/ospf/v3/transport/transport.go` (IPv6 family) -- `SendPacket(name, dst netip.Addr, payload)` (already takes an arbitrary `dst`, so unicast to a neighbour link-local works through the existing send path)
18. `internal/plugins/ospf/config.go` -- `parseInterface` network-type accept-list (per family) + `interfaceConfig` struct
19. `internal/plugins/ospf/instance.go` -- `ifaceConfig`/`neighborInterfaceConfig` thread `NetworkType`/`IsV6` into the shared runtime
20. `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the SINGLE OSPF schema; the IPv4 interface `network-type` enum (line ~178, with `loopback`) and the separate `address-family/ipv6/.../network-type` enum (line ~304, no `loopback`); there is no `ze-ospfv3-conf.yang`

## Task

Add the two remaining OSPF interface **network types** -- **NBMA** (non-broadcast
multi-access) and **point-to-multipoint (PtMP)** -- to the unified OSPF engine at
`internal/plugins/ospf/`, for **both address families**: IPv4 (OSPFv2, RFC 2328)
and IPv6 (OSPFv3, RFC 5340). The umbrella (`plan/spec-ospf-ext-0-umbrella.md`)
shipped OSPF with broadcast (DR/BDR + Network-LSA) and point-to-point only, for
both families, and listed NBMA + PtMP as deferred. The codec, the ISM, the NSM,
DR/BDR election, flooding, Router-LSA origination, and the per-interface config
plumbing all already exist for the two shipped types in both families; what is
missing is the two new behavioural variants and the config surface that selects
them, per family.

**NBMA** (RFC 2328 §9.5/§10/App C.5; RFC 5340 reuses the same election + Hello/poll
model and originates the v6 Network-LSA per §A.4.4) treats one multi-access link
with no all-routers multicast reach for discovery: the router is told its peers by
a **manually configured neighbour list**. A **DR/BDR is still elected** among the
configured, eligible (priority > 0) routers (the §9.4 election is reused verbatim,
shared across both families), and the DR still originates the Network-LSA. Hellos
are sent **unicast to each configured neighbour** at the normal HelloInterval to
neighbours we have heard from and at the slower **PollInterval** to neighbours that
are currently silent (the §10.1 Attempt state). A configured neighbour with
priority 0 is **ineligible** for the election but is still polled; per §9.4 step 6,
when this router becomes DR or BDR it sends a Hello (Start) to those priority-0
neighbours so they begin the adjacency. Adjacency formation follows the broadcast
rule (`should_adj`: only with the DR/BDR).

**Point-to-multipoint** (RFC 2328 §9.5/§10.4/§12.4.1.4/§16.1; RFC 5340 follows the
same semantics over the address-free v6 LSA model) treats the same medium as a
**collection of point-to-point links**: there is **no DR, no BDR, no Network-LSA**;
every router forms a full adjacency with **every** other reachable router
(`should_adj` always true). The interface advertises itself in the Router-LSA as
one **point-to-point (Type-1) link per Full neighbour** plus a **host route** for
the interface's own reachable address, so other routers can reach the PtMP
interface; it does NOT advertise the link's subnet as a transit/network prefix.
SPF resolves the next-hop to each neighbour directly (RFC 2328 §16.1 / RFC 5340
§3.8.1). Hellos are sent to the all-routers group on the broadcast variant (when
the medium supports multicast) or **unicast to a configured neighbour list** on the
non-broadcast variant; this spec implements the broadcast-variant (multicast Hello)
as the default and supports the same `nbma-neighbor` list for the non-broadcast
variant.

The shared ISM/NSM/election/flood **mechanism** is AF-neutral by construction (it
branches on `NetworkType`, and only on `IsV6` for the multicast group / the unicast
destination form). The per-family work differs in the **wire/LSA/prefix
encoding**, which is the crux of the address-family split:

| Aspect | IPv4 family (OSPFv2, RFC 2328) | IPv6 family (OSPFv3, RFC 5340) |
|--------|-------------------------------|-------------------------------|
| PtMP per-neighbour link | Type-1 Router-LSA link: LinkID = neighbour Router ID, LinkData = our interface address, metric = cost (`lsdb/origination.go routerLinks`) | Type-1 address-free Router-LSA link: NeighborRouterID + NeighborInterfaceID + our Interface ID, metric = cost (`origination_v6.go v6RouterLSABody`) |
| PtMP host route | Type-3 stub link in the Router-LSA: LinkID = our interface address, mask 255.255.255.255, metric 0 (Router-LSAs carry addresses) | /128 prefix with the LA-bit (`OptPrefixLA`) in the Intra-Area-Prefix-LSA (Router-LSAs are address-free; prefixes live in the IAP-LSA) |
| NBMA Network-LSA | Type-2 Network-LSA originated by the DR (`OriginateNetwork`) | `0x2002` Network-LSA + DR Network-referencing Intra-Area-Prefix-LSA (`v6OriginateNetwork`) |
| Link-LSA | n/a (no per-link LSA in OSPFv2) | every NBMA + PtMP interface originates its `0x0008` Link-LSA (`v6ShouldOriginateLinkLSA` widened) |
| Hello multicast group / unicast `dst` | AllSPFRouters/AllDRouters; unicast `dst` = neighbour interface IPv4 address | `ff02::5`/`ff02::6`; unicast `dst` = neighbour IPv6 link-local |
| SPF next-hop | neighbour's advertised interface address from its p2p link (§16.1) | neighbour link-local from the adjacency (`v6NextHop.P2PNextHop`, §3.8.1) |
| Config key | `nbma-neighbor` keyed by `address` (IPv4) + `priority` | `nbma-neighbor` keyed by `router-id` (+ optional `link-local`) + `priority` |
| Hello network-mask check | mask must match on broadcast/NBMA/PtMP (§10.5) | no mask in the v6 Hello (Interface ID instead, §A.3.2); `validateHelloLocked` already skips the mask check when `IsV6` |

Both families reuse the existing unicast adjacency-packet path (DD / LS Request /
LS Update retransmission already go to the neighbour address/link-local); the only
flooding delta is the **initial flood / Ack destination** (`floodDestination`),
which on a non-broadcast interface must be a per-neighbour unicast fan-out instead
of a multicast group.

This is an additive, self-contained extension inside the existing OSPF plugin. A
broadcast or point-to-point interface (either family) behaves exactly as today; the
new behaviour is reachable only when an interface is configured `network-type nbma`
or `network-type point-to-multipoint` under its family.

### In scope (this spec)

| Item | Address family | Detail |
|------|----------------|--------|
| Network-type config extension | both | Add `nbma` + `point-to-multipoint` to the YANG `network-type` enum for the IPv4 interface (line ~178) AND the `address-family/ipv6` interface (line ~304), and to the `parseInterface` accept-list per family; the network-type string already threads through `Config.NetworkType`/`InterfaceInfo.NetworkType` |
| NBMA neighbour config | IPv4 | A per-interface `nbma-neighbor` list (key `address`, leaf `priority` default 0) + a `poll-interval` leaf (default 120 s, App C.5) under the IPv4 interface |
| NBMA neighbour config | IPv6 | A per-interface `nbma-neighbor` list (key `router-id`, optional `link-local`, leaf `priority` default 0) + a `poll-interval` leaf (default 120 s) under the `address-family/ipv6` interface |
| NBMA ISM + election | shared | On InterfaceUp an eligible (priority > 0) NBMA interface enters Waiting and runs the §9.4 election over the configured neighbours; priority 0 goes to DROther; the existing `electDRBDR` is reused unchanged, only the election GATE in `iface.go` widens to include NBMA (one AF-neutral change, exercised by both families) |
| NBMA unicast Hello + poll | shared | Hellos sent unicast to each configured neighbour; HelloInterval to heard neighbours, PollInterval to silent ones (Attempt-state poll); §9.4-step-6 Start Hello to priority-0 neighbours when this router becomes DR/BDR; the unicast `dst` is the neighbour IPv4 address (IPv4) or link-local (IPv6) |
| NBMA Network-LSA | IPv4 | The DR of an NBMA segment originates the Type-2 Network-LSA (`OriginateNetwork`, gated on `DR == self`) |
| NBMA Network-LSA + Link-LSA | IPv6 | The DR originates the `0x2002` Network-LSA + the DR Network-Intra-Area-Prefix-LSA (`v6OriginateNetwork`); every NBMA interface originates its `0x0008` Link-LSA (`v6ShouldOriginateLinkLSA` widened) |
| PtMP adjacency | shared | `should_adj` returns true for PtMP (every neighbour adjacent); no DR/BDR; no Network-LSA; Hellos to the all-routers group (broadcast variant) or the configured list (non-broadcast variant) |
| PtMP host-route origination | IPv4 | `routerLinks` emits one Type-1 p2p link (LinkID = neighbour Router ID, LinkData = our address, metric = cost) per Full neighbour + a single host-route Type-3 stub link (mask 255.255.255.255, metric 0); NO subnet stub, NO Network-LSA |
| PtMP host-route origination | IPv6 | `v6RouterLSABody` emits one address-free Type-1 p2p link per Full neighbour; the interface's own global address is a /128 LA-bit prefix in the Intra-Area-Prefix-LSA; the Link-LSA is originated; NO transit link, NO Network-LSA, NO subnet prefix |
| PtMP next-hop | IPv4 | SPF resolves the next-hop from the neighbour's interface address in its advertised p2p link (§16.1); reuse the existing point-to-point next-hop |
| PtMP next-hop | IPv6 | SPF resolves the next-hop from the adjacency link-local (`v6NextHop.P2PNextHop`, §3.8.1); reuse unchanged; confirm no broadcast-only assumption in `v6RouterLinks`/`BuildGraph` |
| Non-broadcast flood fan-out | shared | `floodDestination` (and the initial flood + Ack path) on a non-broadcast interface unicasts to each Flood-eligible neighbour (IPv4 address / IPv6 link-local) instead of a multicast group; PtMP has no DR-relay suppression |
| Interface show surface | both | `show ip ospf interface` (IPv4) / `show ipv6 ospf interface` (IPv6) render NBMA/PtMP; NBMA shows its configured-neighbour/poll state |

### Out of scope (noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| Virtual links (synthetic p2p across a transit area, RFC 2328 §15 / RFC 5340) | the OSPF virtual-links extension spec (explicitly excluded by this task); an NBMA/PtMP interface cannot serve as a virtual-link transit here |
| RFC 6845 hybrid broadcast-and-PtMP interface type | future (guide ref #28); not the RFC 2328/5340 PtMP this spec adds |
| RFC 5613 LLS / RFC 2328 demand circuits (PtMP-over-demand) | out of scope (umbrella future list) |
| RFC 5838 multiple address families on the NBMA/PtMP interface | umbrella out-of-scope; the Instance ID stays explicit, no multi-AF behaviour |
| RFC 7166 auth interaction beyond "unchanged" | the auth path is per-packet and AF-independent; NBMA/PtMP reuse it as-is, no new auth surface |
| Two-part metric (RFC 8042) on PtMP | out of scope |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
- [ ] `plan/learned/972-ospf-af-unify.md` -- the AF-unification decision
  -> Decision: IPv4 and IPv6 are two address families of ONE OSPF engine; the shared `iface`/`neighbor`/`lsdb` packages are AF-neutral and string-keyed on `NetworkType` (branching on `IsV6` only for the multicast group / unicast destination form); the IPv6 wire/LSA/prefix code lives in the `_v6` files and `internal/plugins/ospf/v3/`
  -> Constraint: do NOT push v6 prefix / Interface-ID semantics into the shared packages; add the two new network types as AF-neutral branches at the shared chokepoints, and the per-family wire/LSA encoding in `lsdb/origination.go` (IPv4) and `origination_v6*.go` (IPv6)
- [ ] `docs/research/ospf-implementation-guide.md` §7 "Network Types and Interface Model" (~470-514, the four-type table + per-type prose) -- the authoritative behavioural contrast for all four wire types
  -> Decision: PtMP is "a collection of point-to-point links, no DR" with host-route origination; NBMA is "explicit static neighbour list, DR elected, unicast per neighbour"; this spec implements that split in both families, reusing the broadcast election for NBMA and the p2p link model for PtMP. The IPv4 host route is a Router-LSA stub link (/32); the IPv6 host route is a /128 LA-bit prefix in the Intra-Area-Prefix-LSA (v6 Router-LSAs are address-free)
  -> Constraint: default Hello interval is 30 s on NBMA/PtMP vs 10 s on broadcast/P2P (guide ~499); the per-interface YANG `hello-interval` default stays 10 unless the operator overrides, but the spec documents the 30 s recommendation and the PollInterval default 120 s
- [ ] `docs/research/ospf-implementation-guide.md` §5 ISM/NSM prose (~255-321: Waiting/DROther/Attempt states, the `should_adj` predicate)
  -> Constraint: `should_adj` is "point-to-point, point-to-multipoint, and virtual links: always yes; broadcast or NBMA: only if local or neighbour is DR/BDR" (guide ~321); the shared `nsm.go shouldAdj` default-branch must split into PtMP (true) and NBMA (DR-gated), not stay `false`, for both families
  -> Constraint: NBMA Attempt state (guide ~298) -- a configured-but-silent NBMA neighbour is polled at PollInterval, not dropped
- [ ] `docs/research/ospf-implementation-guide.md` §8 flooding addressing (~355) -- "Point-to-point, point-to-multipoint, and NBMA use unicast (or multi-unicast) per RFC 2328 §13.3 Table 19"
  -> Constraint: the initial flood on a non-broadcast interface is a per-neighbour unicast fan-out (IPv4 address / IPv6 link-local); the existing retransmit path is already unicast, so only `floodDestination`/the initial-flood send changes
- [ ] `docs/research/ospf-implementation-guide.md` §15 (address-family separation) -- the keep-the-v6-wire-code-separate recommendation
  -> Constraint: keep the v6-specific origination, codec, and next-hop in the v6 files (`origination_v6*.go`, `afstrategy_v6.go`, `v3/`); do not push v3 prefix/Interface-ID semantics into the shared `iface`/`neighbor` packages, which stay AF-neutral string-keyed
- [ ] `plan/spec-ospf-ext-0-umbrella.md` -- the extension umbrella that scopes both families and defers NBMA/PtMP
  -> Constraint: the umbrella declared the `network-type` enum as `broadcast`/`point-to-point` per family and NBMA/PtMP as deferred; this spec adds the two enum values to BOTH family leaves and closes that deferred row; it must NOT redefine the interface config model, only extend the enum and add the NBMA-only `nbma-neighbor`/`poll-interval` leaves per family
  -> Decision: keep per-interface `area` binding, costs, timers, priority, passive, auth, Instance ID (IPv6) exactly as the umbrella defines; NBMA/PtMP are new values of an existing leaf, not a new config subsystem
- [ ] `ai/rules/buffer-first.md`, `ai/rules/no-sprintf-alloc.md` -- Hello/flood encode and the host-route LSA/prefix are buffer-first
  -> Constraint: the IPv4 PtMP host route + per-neighbour p2p links append into the existing `routerLinks` slice; the IPv6 /128 host route + p2p links append into the existing `v6RouterLSABody.Links` / Intra-Area-Prefix `Prefixes` slices, encoded via the existing buffer-first `WriteTo`; the unicast Hello fan-out reuses one built `buildHelloPacket` buffer per neighbour, no `fmt`/`+` string building

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2328.md` -- OSPF Version 2 (the IPv4 family)
  -> Constraint: §9.1/§9.3 -- an eligible interface on broadcast OR NBMA enters Waiting on InterfaceUp and delays the (B)DR election until WaitTimer/BackupSeen; priority 0 goes DROther; PtMP (like p2p) goes straight to the point-to-point ISM state with no election
  -> Constraint: §9.4 step 6 -- "On NBMA networks, if Router X became DR or BDR, it must start sending Hello packets (Start event) to those neighbors that are not eligible to become DR (priority 0)"
  -> Constraint: §9.5 -- on NBMA Hellos are sent unicast to neighbours eligible to become DR at HelloInterval, and to ineligible neighbours at PollInterval; on PtMP Hellos are sent (multicast or unicast) to all attached neighbours
  -> Constraint: §10.1/§10.3 -- the **Attempt** state exists ONLY on NBMA; the Start event moves Down->Attempt
  -> Constraint: §10.4 `should_adj` -- always adjacent on point-to-point, point-to-multipoint, and virtual links; on broadcast/NBMA only with the DR/BDR
  -> Constraint: §12.4.1.4 / App A.4.2 -- a PtMP interface contributes one Type-1 (point-to-point) Router-LSA link per fully adjacent neighbour (LinkID = neighbour Router ID, LinkData = own interface IP) and a host route (Type-3 stub, LinkID = own interface IP, mask 0xffffffff, metric 0)
  -> Constraint: §16.1 -- for PtMP the next-hop is the neighbour's interface address from the destination router-LSA's matching p2p link
  -> Constraint: App C.5 -- PollInterval is the reduced Hello rate to inactive NBMA neighbours (sample 120 s)
- [ ] `rfc/short/rfc5340.md` -- OSPF for IPv6 (the IPv6 family)
  -> Constraint: §2.1 -- OSPFv3 runs per link, not per subnet; an interface is identified by a 32-bit Interface ID, and the Router-LSA/Network-LSA describe the graph only (no addresses); so the PtMP host route and the link's prefixes live in the Intra-Area-Prefix-LSA, never in the Router-LSA
  -> Constraint: §A.3.2 -- the v6 Hello has an Interface ID and NO network mask; the §10.5 "mask must match" check does NOT apply to v6 (`validateHelloLocked` already skips it when `IsV6`)
  -> Constraint: §A.4.3 -- a Router-LSA point-to-point link carries Type 1, the neighbour's Router ID, the neighbour's Interface ID, and this router's Interface ID (address-free); a PtMP interface contributes one such link per Full neighbour, exactly like a v6 point-to-point interface
  -> Constraint: §A.4.4 -- the Network-LSA is DR-originated on a broadcast OR NBMA link and lists the attached Router IDs; a PtMP interface (no DR) originates NONE
  -> Constraint: §A.4.10 / §A.4.1 -- IPv6 prefixes attach via the Intra-Area-Prefix-LSA; the LA-bit (`OptPrefixLA`, 0x02) marks a prefix as an actual local interface address; the PtMP host route is the interface's own global address as a /128 with the LA-bit set; padding is `((PrefixLength+31)/32)` 32-bit words (a /128 = 4 words)
  -> Constraint: §2.9 -- v6 multicast is `ff02::5` / `ff02::6`; unicast retransmission and (for NBMA/non-broadcast-PtMP) the initial flood and Hello use the neighbour's link-local; the raw IPv6 socket binds the interface link-local as source
  -> Constraint: §3.8.1 -- the v6 next-hop is the neighbour's link-local from the adjacency, not from any LSA; `afstrategy_v6.go v6NextHop` already does this, so PtMP next-hop needs no new code

**Key insights:**
- NBMA = broadcast election + manual neighbour list + unicast/poll Hellos, in BOTH families. The §9.4 election (`electDRBDR`) and the Network-LSA origination are reused verbatim; the new behaviour is the configured-neighbour source, the unicast/poll Hello send, the Attempt state, and the §9.4-step-6 Start Hello (all AF-neutral, shared). The per-family delta is only the Network-LSA wire shape (Type-2 vs `0x2002` + Link-LSA).
- PtMP = point-to-point semantics on a multi-access medium, in BOTH families. No DR, no Network-LSA, `should_adj` always true, one p2p Router-LSA link per neighbour, plus a host route. The point-to-point ISM/NSM/next-hop machinery is reused. The per-family delta is the host-route encoding: IPv4 = a /32 Router-LSA stub link; IPv6 = a /128 LA-bit prefix in the Intra-Area-Prefix-LSA (v6 Router-LSAs are address-free).
- The two new types share one config delta per family (the enum + the optional `nbma-neighbor`/`poll-interval` leaves) and one shared flooding delta (`floodDestination` unicast fan-out on non-broadcast interfaces). Everything else is per-type branching at existing chokepoints: shared (`iface.Start`, the election gate, `shouldAdj`, `floodDestination`) and per-family (`routerLinks` for IPv4, `v6RouterLSABody`/`v6OriginateSelf`/`v6ShouldOriginateLinkLSA` for IPv6).

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/ospf/iface/ism.go` -- the `State` enum (Down/Loopback/Waiting/PointToPoint/DROther/Backup/DR) and the AF-shared network-type string constants `NetworkBroadcast`/`NetworkPointToPoint`/`NetworkLoopback`; there is NO `nbma`/`point-to-multipoint` constant
  -> Constraint: add `NetworkNBMA = "nbma"` and `NetworkPointToMultipoint = "point-to-multipoint"` constants here (the single source of the iface-package string), mirrored by the `lsdb` and `neighbor` package copies (`NetworkBroadcast`/`NetworkPointToPoint`); shared by both families
- [ ] `internal/plugins/ospf/iface/iface.go` -- `Start()` switches on `NetworkType`: `loopback` -> Loopback (no timers); `point-to-point` -> PointToPoint + timers; `default` -> Waiting (priority > 0) or DROther (priority 0) + timers, arming the WaitTimer ONLY when `NetworkType == NetworkBroadcast`. `SendHello()` sends to `allSPFRouters` (IPv4) / `allSPFRoutersV6` (IPv6) only. `runElectionLocked()` returns early unless `NetworkType == NetworkBroadcast`. `buildHelloPacket()` builds one Hello for all neighbours. `Sender.SendPacket(name, dst, payload)` takes an arbitrary `dst`
  -> Constraint: PtMP must take the `point-to-point` ISM branch (PointToPoint, no election, no WaitTimer); NBMA must take the broadcast-like branch (Waiting/DROther + WaitTimer) with unicast/poll Hellos. The election gate `NetworkType != NetworkBroadcast` widens to `!= NetworkBroadcast && != NetworkNBMA`. `SendHello`/`buildHelloPacket` gain a per-neighbour unicast fan-out for non-multicast interfaces (the `dst` is `IsV6`-branched: IPv4 address vs link-local). All AF-neutral except the destination form
- [ ] `internal/plugins/ospf/iface/election.go` -- `electDRBDR`, `chooseBDR`, `chooseDeclaredDR`, `betterCandidate` -- a pure RFC §9.4 election over a candidate slice
  -> Constraint: reuse verbatim for NBMA in both families; the candidates for an NBMA election are the configured neighbours we have heard from plus self; no change to this file
- [ ] `internal/plugins/ospf/neighbor/nsm.go` -- `shouldAdj`: `point-to-point` -> true; `broadcast`/`""` -> DR/BDR-only; `default` -> false. `startExchange` resets the DD/summary state
  -> Constraint: split the `default` branch: `point-to-multipoint` -> true (every neighbour adjacent); `nbma` -> the DR/BDR-only rule (same as broadcast). A bare `default false` leaves PtMP/NBMA neighbours stuck at 2-Way. This reads `cfg.NetworkType` so it applies to both families
- [ ] `internal/plugins/ospf/neighbor/table.go` -- `hello()` drives the NSM; `sendInitialDDLocked`/`resendLastDDLocked` already unicast to `n.Address`; `FloodNeighbors`/`AcceptsFlooding` gate flooding on neighbour state; there is no NBMA Attempt state or poll timer here
  -> Constraint: the adjacency-bringup unicast path already works for any neighbour with a known `Address`; NBMA/PtMP need that address populated from the configured list (NBMA) or learned Hello source (both). The NBMA Attempt/poll lives in the `iface` Hello loop, not in `table.go`
- [ ] `internal/plugins/ospf/lsdb/flooding.go` -- `floodDestination(iface)` returns a multicast group (AllDRouters for a broadcast DROther, else AllSPFRouters; `ff02::5`/`ff02::6` for IPv6); `floodExcept` runs the §13.3 eligible-interface + DR-relay rules; `neighborAddr`/the retransmit `sends` loop already unicast to `nbr.Address`; `eligibleInterface` is area/type scope only; `InterfaceInfo`/`NeighborInfo` carry `IsV6`, `NetworkType`, `IPv6LinkLocal`, `InterfaceID`
  -> Constraint: the retransmit path is already per-neighbour unicast; the gap is the INITIAL flood and the Ack, which use `floodDestination`'s multicast group. On NBMA / non-broadcast PtMP, the initial flood/ack must fan out unicast to each Flood-eligible neighbour (the destination is the neighbour IPv4 address or, when `IsV6`, the link-local). PtMP has no DR, so the §13.3 DR-relay suppression does not apply -- a PtMP/NBMA-without-DR-relay floods to all adjacent neighbours
- [ ] `internal/plugins/ospf/lsdb/origination.go` (IPv4 family) -- `routerLinks(in)`: for `point-to-point` emits one Type-1 p2p link per Full neighbour (LinkID = neighbour Router ID, LinkData = our address); for `broadcast` with a DR emits a Type-2 transit link; ALWAYS appends a subnet stub link (Type-3, LinkID = network address, mask = NetworkMask) when address+mask are set. `OriginateNetwork` builds the Type-2 Network-LSA for the DR; `OriginateFromTopology` originates a Network-LSA only when `iface.DR == router`
  -> Constraint: PtMP must emit the per-neighbour Type-1 p2p links (reuse the point-to-point branch) BUT replace the subnet stub with a host-route stub link (LinkID = our interface address, mask 255.255.255.255, metric 0); PtMP must NOT originate a Network-LSA. NBMA behaves like broadcast for origination (transit link when DR, subnet stub) and DOES originate a Network-LSA when DR
- [ ] `internal/plugins/ospf/origination_v6.go` (IPv6 family) -- `v6RouterLSABody` switches `NetworkPointToPoint` (per-neighbour address-free p2p links) and `NetworkBroadcast` (transit link) ONLY; `v6OriginateNetwork` builds the DR `0x2002` Network-LSA; `v6OriginateSelf` sweeps Router / DR-Network / Intra-Area-Prefix self-LSAs and gates the Network-Intra-Area-Prefix-LSA on `NetworkType != NetworkBroadcast`
  -> Constraint: add a `NetworkPointToMultipoint` case to `v6RouterLSABody` (per-neighbour p2p links, identical to point-to-point) and a `NetworkNBMA` case (the transit link, identical to broadcast); add the PtMP /128 LA-bit host route to the self Intra-Area-Prefix-LSA; widen the DR Network / Network-Intra-Area-Prefix gate to include NBMA; PtMP originates NO Network-LSA
- [ ] `internal/plugins/ospf/origination_v6_link.go` (IPv6 family) -- `v6ShouldOriginateLinkLSA` returns true only for `NetworkBroadcast || NetworkPointToPoint`; `v6OriginateLinkLSA` builds the `0x0008` Link-LSA (link-local + prefixes)
  -> Constraint: widen `v6ShouldOriginateLinkLSA` to also return true for `NetworkNBMA` and `NetworkPointToMultipoint`; the host-route prefix flows through the Link-LSA prefix list + the self Intra-Area-Prefix-LSA, NOT the Router-LSA
- [ ] `internal/plugins/ospf/afstrategy_v6.go` (IPv6 family) -- `BuildGraph`/`v6RouterLinks` translate Router-LSA p2p and transit links into the shared SPF graph; `v6NextHop.P2PNextHop`/`TransitNextHop` resolve the next-hop to the neighbour link-local via `neighbors.AddressOf`
  -> Constraint: PtMP emits Type-1 p2p links, which `v6RouterLinks` already translates (the `RouterLinkTypeP2P` case), and the next-hop already resolves via `P2PNextHop` -- SPF + next-hop need NO change for PtMP; CONFIRM with a test. NBMA emits a transit link, already handled by `RouterLinkTypeTransit`
- [ ] `internal/plugins/ospf/v3/transport/transport.go` (IPv6 family) -- `SendPacket(name, dst netip.Addr, payload)` sends to any IPv6 `dst` with the bound interface link-local as source; it rejects a non-IPv6 `dst`
  -> Constraint: the unicast Hello/flood to a neighbour link-local uses this existing send path unchanged; no new transport API; the neighbour link-local must be a valid IPv6 unicast `dst`
- [ ] `internal/plugins/ospf/config.go` -- `parseInterface` resolves `NetworkType` per family: the IPv4 interface accepts `broadcast|point-to-point|loopback` (default broadcast), the v6 interface accepts `broadcast|point-to-point`; `interfaceConfig` has no neighbour-list or poll-interval field
  -> Constraint: extend the IPv4 accept-list with `nbma`/`point-to-multipoint` and the v6 accept-list with the same; add `NBMANeighbors` (IPv4: address + priority; IPv6: router-id + optional link-local + priority) and `PollInterval uint16` to `interfaceConfig` and parse them per family; thread `PollInterval` and the neighbour list into the iface `Config`
- [ ] `internal/plugins/ospf/instance.go` -- `ifaceConfig(ic)` maps `interfaceConfig` to `ospfiface.Config` (threads `NetworkType` as a string + `IsV6`); `neighborInterfaceConfig` threads `NetworkType` into the NSM `InterfaceConfig`; `OriginateFromTopology`/`v6OriginateSelf` are driven from here on topology change
  -> Constraint: thread the new `PollInterval` + the configured NBMA neighbour list through `ospfiface.Config` so the iface Hello loop can unicast/poll; the network-type string + `IsV6` already flow end to end (no new plumbing for the type itself, only for the NBMA extras)
- [ ] `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the SINGLE OSPF schema; the IPv4 interface `network-type` enum (line ~178) is `broadcast`/`point-to-point`/`loopback` (default broadcast); the `address-family/ipv6/.../network-type` enum (line ~304) is `broadcast`/`point-to-point`; neither has a `nbma-neighbor` list or a `poll-interval` leaf
  -> Constraint: add `enum nbma;` + `enum point-to-multipoint;` to BOTH the IPv4 and the v6 enum; add a `nbma-neighbor` list + a `poll-interval` leaf under EACH interface (IPv4 keyed by `address`; v6 keyed by `router-id` with optional `link-local`); the IPv4 enum keeps `loopback`, the v6 enum does not gain it

**Behavior to preserve:**
- Broadcast (Waiting/DROther/Backup/DR + Network-LSA + multicast Hello/flood) and point-to-point (PointToPoint state + per-neighbour p2p link + subnet stub / address-free p2p link) behave EXACTLY as today in BOTH families; the new branches are reachable only for the two new enum values.
- The `electDRBDR` election, the `OriginateNetwork`/`v6OriginateNetwork` Network-LSA, the `v6OriginateLinkLSA`, the `sendInitialDDLocked` unicast DD path, the `neighborAddr` retransmit unicast, the `v6NextHop`, and the §13.3 receive procedure are reused unchanged.
- All existing OSPF unit/functional/interop tests (broadcast + p2p, both families, including FRR `ospfd` and `ospf6d`) stay green; the YANG default `network-type broadcast` is unchanged on both leaves.
- The shared `iface`/`neighbor`/`lsdb` packages stay AF-neutral: no v3-specific prefix/Interface-ID logic leaks into them.

**Behavior to change:** (all RFC-required for the two new types, none discretionary)
- `parseInterface` accepts `nbma` + `point-to-multipoint` on both family leaves; both YANG enums gain the two values; the new `nbma-neighbor`/`poll-interval` leaves parse and thread through per family.
- `iface.Start` routes PtMP into the point-to-point ISM branch and NBMA into the broadcast-like (Waiting/DROther + WaitTimer + election) branch (shared).
- The election gate widens to include NBMA; PtMP never elects (shared).
- `shouldAdj` returns true for PtMP and DR-gated for NBMA (shared).
- `SendHello`/the Hello loop unicast/poll on non-multicast interfaces; the §9.4-step-6 Start Hello to priority-0 NBMA neighbours (shared; `dst` is `IsV6`-branched).
- IPv4: `routerLinks` emits PtMP per-neighbour p2p links + a host-route stub (not a subnet stub) and no Network-LSA; NBMA originates a Network-LSA when DR.
- IPv6: `v6RouterLSABody` gains PtMP (p2p links) and NBMA (transit link) cases; the self Intra-Area-Prefix-LSA gains the PtMP /128 LA-bit host route; `v6ShouldOriginateLinkLSA` returns true for NBMA + PtMP; the DR Network/Network-Intra-Area-Prefix gate widens to NBMA.
- `floodDestination`/the initial flood fan out unicast on NBMA / non-broadcast PtMP (shared; `dst` is `IsV6`-branched).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Config:** an operator sets `interfaces/interface/network-type nbma|point-to-multipoint` (IPv4) or `address-family/ipv6/.../network-type nbma|point-to-multipoint` (IPv6), with a `nbma-neighbor` list + optional `poll-interval` -> YANG validate -> `parseInterface` (per family) -> `interfaceConfig` -> `instance.ifaceConfig` -> `ospfiface.Config`.
- **ISM:** `iface.Start()` selects the per-type initial state and timers (shared, AF-neutral).
- **Hello send (clock tick):** `helloLoop` -> `SendHello` -> multicast (broadcast / PtMP-broadcast-variant) or per-neighbour unicast/poll (NBMA / PtMP-non-broadcast); the multicast group / unicast `dst` is `IsV6`-branched.
- **Hello receive:** an incoming Hello -> (IPv6: v3 transport, link-local source, Instance ID check) -> `receiveHello` -> NSM `hello()` -> election (NBMA) / direct adjacency (PtMP).
- **Origination:** topology change -> `OriginateFromTopology` (IPv4) / `v6OriginateSelf` (IPv6) -> per-interface link records.
- **Flood:** an LSA to flood -> `floodExcept` -> per-neighbour unicast fan-out on non-broadcast interfaces.

### Transformation Path
1. **Config resolve:** the YANG enum (per family) accepts the two new values; `parseInterface` resolves `NetworkType`, the `NBMANeighbors` slice (IPv4: address + priority; IPv6: router-id + optional link-local + priority), and `PollInterval`. The network-type string is threaded through `ospfiface.Config`, `ospfneighbor.InterfaceConfig`, and `lsdb.InterfaceInfo` (all already string-typed + `IsV6`).
2. **ISM init (`iface.Start`, shared):** `point-to-multipoint` -> StatePointToPoint, no election, no WaitTimer; `nbma` priority > 0 -> StateWaiting + WaitTimer + election; `nbma` priority 0 -> StateDROther; for NBMA the configured neighbours are seeded in the **Attempt** state (poll-pending) so they are polled before any Hello is heard.
3. **Hello addressing (`SendHello`, shared):** broadcast / PtMP-broadcast-variant -> multicast group (`allSPFRouters` / `ff02::5`); NBMA / PtMP-non-broadcast -> a per-neighbour unicast loop to the neighbour IPv4 address (IPv4) or link-local (IPv6): HelloInterval to neighbours in state >= Init (heard), PollInterval to neighbours still in Attempt (silent). When this NBMA router is DR/BDR, a Start Hello is also sent to priority-0 neighbours (§9.4 step 6).
4. **DR/BDR election (NBMA only, shared):** `runElectionLocked` is entered (gate widened); the candidate set is self + configured neighbours in state >= 2-Way; `electDRBDR` runs unchanged; the elected DR originates the Network-LSA (Type-2 for IPv4; `0x2002` + DR Network-Intra-Area-Prefix-LSA for IPv6). PtMP skips this entirely.
5. **Adjacency (`shouldAdj`, shared):** PtMP -> always adjacent (DD exchange with every neighbour); NBMA -> adjacent only with the DR/BDR. The DD/LSReq/LSUpdate exchange uses the existing unicast path (`n.Address` for IPv4; the neighbour link-local via the v3 transport for IPv6).
6. **Router-LSA origination (per family):**
   - IPv4 (`routerLinks`): PtMP -> one Type-1 p2p link per Full neighbour (LinkID = neighbour Router ID, LinkData = our interface address, metric = cost) + one host-route Type-3 stub link (LinkID = our interface address, mask 255.255.255.255, metric 0); no subnet stub, no Network-LSA. NBMA -> a Type-2 transit link when DR, a subnet Type-3 stub, and a Network-LSA when DR.
   - IPv6 (`v6RouterLSABody`): PtMP -> one Type-1 address-free p2p link per Full neighbour (NeighborRouterID + NeighborInterfaceID + our Interface ID, metric = cost); no transit link. NBMA -> a transit link when DR.
7. **Prefix / host-route origination (per family):**
   - IPv4: the PtMP host route IS the Type-3 stub link emitted in step 6 (Router-LSAs carry addresses).
   - IPv6 (`v6OriginateSelf` / the self Intra-Area-Prefix-LSA): PtMP -> add the interface's own global address as a /128 with the LA-bit (`OptPrefixLA`) set; the link's subnet prefix is NOT advertised for a PtMP interface. The Link-LSA carries the link-local + prefixes for all four types (`v6ShouldOriginateLinkLSA` widened to NBMA + PtMP).
8. **SPF next-hop (per family):** IPv4 -> the PtMP destination's interface address is read from its advertised p2p link (§16.1), giving the direct next-hop, exactly as point-to-point. IPv6 -> the PtMP destination's next-hop is the neighbour's link-local from the adjacency (`v6NextHop.P2PNextHop`, §3.8.1), exactly as point-to-point; the /128 host route inherits that next-hop in `BuildRoutes`. NBMA resolves next-hops via the transit-network vertex like broadcast in both families.
9. **Flood fan-out (`floodDestination` / initial flood, shared):** on a non-broadcast interface, the initial flood + Ack unicast to each Flood-eligible neighbour (IPv4 address / IPv6 link-local); on broadcast / PtMP-broadcast-variant, the existing multicast group is used. PtMP has no DR, so the §13.3 DR-relay suppression is skipped.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config (YANG) <-> engine, IPv4 | IPv4 `network-type` enum + `nbma-neighbor`/`poll-interval` -> `parseInterface` -> `interfaceConfig` -> `ospfiface.Config` | [ ] |
| Config (YANG) <-> engine, IPv6 | v6 `network-type` enum + `nbma-neighbor`/`poll-interval` -> `parseInterface` -> `interfaceConfig` -> `ospfiface.Config` (`IsV6` true) | [ ] |
| Config <-> NSM | `NetworkType` string (+ `IsV6`) threaded into `ospfneighbor.InterfaceConfig` (already string-typed; new values only) | [ ] |
| ISM <-> Hello send | `iface.Start` per-type init (shared); `SendHello` multicast vs per-neighbour unicast/poll (`dst` `IsV6`-branched) | [ ] |
| NSM <-> adjacency | `shouldAdj` PtMP-always / NBMA-DR-gated (shared); existing unicast DD path reused per family | [ ] |
| Topology <-> Router-LSA, IPv4 | `routerLinks` PtMP host-route stub + p2p links; NBMA Network-LSA when DR | [ ] |
| Topology <-> Router-LSA, IPv6 | `v6RouterLSABody` PtMP address-free p2p links / NBMA transit link | [ ] |
| Topology <-> Intra-Area-Prefix-LSA, IPv6 | PtMP /128 LA-bit host route in the self Intra-Area-Prefix-LSA; no subnet prefix for PtMP | [ ] |
| LSDB <-> flooding | `floodDestination` unicast fan-out on non-broadcast interfaces (IPv4 address / IPv6 link-local); PtMP no DR-relay | [ ] |
| SPF <-> next-hop, IPv4 | PtMP next-hop from the neighbour's p2p-link interface address (§16.1, reuse p2p path) | [ ] |
| SPF <-> next-hop, IPv6 | PtMP next-hop from the neighbour link-local (`v6NextHop.P2PNextHop`, §3.8.1, reused) | [ ] |
| Transport <-> unicast, IPv6 | v3 `SendPacket(name, neighbour-link-local, payload)`; reused, no new API | [ ] |

### Integration Points
- `internal/plugins/ospf/iface` (shared) -- ISM init, election gate, Hello send/poll, the network-type constants (the single delta site for ISM + Hello addressing, AF-neutral).
- `internal/plugins/ospf/neighbor` (shared) -- `shouldAdj` per-type branch; the existing unicast DD/LSReq path reused; the configured-neighbour seeding for NBMA Attempt state.
- `internal/plugins/ospf/lsdb/flooding.go` (shared) -- `floodDestination`/initial-flood unicast fan-out; PtMP no DR-relay suppression.
- `internal/plugins/ospf/lsdb/origination.go` (IPv4) -- `routerLinks` PtMP host-route stub / p2p links; `OriginateNetwork` Network-LSA gate (NBMA yes, PtMP no).
- `internal/plugins/ospf/origination_v6.go`, `origination_v6_link.go` (IPv6) -- `v6RouterLSABody` PtMP/NBMA branches; the PtMP /128 host route in the self Intra-Area-Prefix-LSA; the DR Network-LSA / Network-Intra-Area-Prefix gate widened to NBMA; `v6ShouldOriginateLinkLSA` widened to NBMA + PtMP.
- `internal/plugins/ospf/spf` (IPv4, READ-ONLY) / `internal/plugins/ospf/afstrategy_v6.go` (IPv6, READ-ONLY) -- confirm the PtMP next-hop reuses the point-to-point path (neighbour address / link-local).
- `internal/plugins/ospf/v3/transport/transport.go` (IPv6, READ-ONLY) -- the unicast `SendPacket` path reused for Hello/flood to a neighbour link-local.
- `internal/plugins/ospf/config.go` + `yang/ze-ospf-conf.yang` -- the enums + the NBMA-only leaves, per family.
- `internal/plugins/ospf/instance.go` -- thread `PollInterval` + the NBMA neighbour list into `ospfiface.Config`, per family.

### Architectural Verification
- [ ] No bypassed layers (config -> resolve -> iface/neighbor/lsdb runtime + per-family origination, the same spine as broadcast/p2p; no new packet type, no new dispatcher)
- [ ] No unintended coupling (the two new types are additional values of an existing leaf and additional branches at existing chokepoints; the shared packages stay AF-neutral; no v3 prefix logic in `iface`/`neighbor`; no plugin name leaking into generic packages)
- [ ] No duplicated functionality (reuses `electDRBDR`, `OriginateNetwork`/`v6OriginateNetwork`, `v6OriginateLinkLSA`, `sendInitialDDLocked`, `neighborAddr`, the point-to-point ISM/NSM/next-hop, `v6NextHop`, the v3 unicast transport, `RouterLSA.WriteTo`)
- [ ] Zero-copy / buffer-first preserved (Hello + Router-LSA links + the host route encode buffer-first; the unicast Hello fan-out reuses one built buffer per neighbour)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The shared ISM/NSM/election/flood mechanism for the two new network types (ISM branches, election gate, `shouldAdj` split, unicast/poll Hello, `floodDestination` fan-out, iface `Config` NBMA fields) is AF-neutral and serves BOTH families via the `IsV6`+`NetworkType` threads; only the multicast group / unicast `dst` form branches on `IsV6` | `iface.go`/`nsm.go`/`flooding.go` branch on `NetworkType` (and `IsV6` only for the group); `plan/learned/972-ospf-af-unify.md` | a parallel per-family mechanism is needed | package builds; `TestOSPFNBMAElection` + `TestOSPFv3NBMAElection` both pass over the shared path | unvalidated |
| A-2 | `electDRBDR`/`chooseBDR`/`chooseDeclaredDR` is network-type- and AF-agnostic and can be reused for NBMA by only widening the gate in `iface.go` (no change to `election.go`) | `iface/election.go` is a pure candidate-slice election; `iface.go` returns early unless `NetworkBroadcast` | NBMA needs a separate election path | `TestOSPFNBMAElection`, `TestOSPFv3NBMAElection` | unvalidated |
| A-3 | The point-to-point ISM/NSM/next-hop path works unchanged for PtMP once origination emits per-neighbour p2p links + a host route, in both families | `iface.go` `Start` p2p branch; `nsm.go` `shouldAdj` p2p=true; IPv4 §16.1 + `origination.go` p2p branch; IPv6 `v6RouterLinks` `RouterLinkTypeP2P` + `v6NextHop.P2PNextHop` | PtMP needs a distinct ISM/NSM/next-hop path | `TestOSPFPtMPAdjacency`/`TestOSPFv3PtMPAdjacency`, `TestOSPFPtMPNextHop`/`TestOSPFv3PtMPNextHop` | unvalidated |
| A-4 | The DD / LS Request / LS Update retransmit path is ALREADY per-neighbour unicast (`sendInitialDDLocked` -> `n.Address`; v3 `SendPacket(dst)`), so only the INITIAL flood + Ack (`floodDestination` multicast) needs a unicast fan-out for non-broadcast interfaces | `neighbor/dd.go` unicasts to `n.Address`; `flooding.go neighborAddr`; `v3/transport/transport.go` `SendPacket` takes an arbitrary `dst` | a larger unicast plumbing change is needed | `TestOSPFNBMAFloodUnicast`/`TestOSPFv3NBMAFloodUnicast`, `TestOSPFPtMPFloodUnicast`/`TestOSPFv3PtMPFloodUnicast` | unvalidated |
| A-5 | IPv4 PtMP must emit a host-route Type-3 stub link (mask 255.255.255.255) for its OWN interface address + one Type-1 p2p link per Full neighbour, and must NOT emit the subnet stub or a Network-LSA | RFC 2328 §12.4.1.4, App A.4.2; guide §7 | other routers cannot reach the PtMP address / spurious transit vertex | `TestOSPFPtMPHostRoute`, `TestOSPFPtMPNoNetworkLSA` | unvalidated |
| A-6 | IPv6 PtMP must emit the host route as a /128 LA-bit prefix in the Intra-Area-Prefix-LSA (NOT a Router-LSA stub link, because v3 Router-LSAs are address-free) + one address-free Type-1 p2p link per Full neighbour, and must NOT emit the subnet prefix or a Network-LSA | RFC 5340 §A.4.3, §A.4.10/§A.4.1; `v3/types/prefix.go` `OptPrefixLA` | the host route is mis-encoded; remote routers cannot reach the PtMP interface | `TestOSPFv3PtMPHostRoute`, `TestOSPFv3PtMPRouterLSALinks`, `TestOSPFv3PtMPNoNetworkLSA` | unvalidated |
| A-7 | NBMA still elects a DR and the DR still originates the Network-LSA, exactly like broadcast, in both families (the only NBMA difference at origination is the unicast Hello, not the LSA model) | RFC 2328 §9.4/App A.4.3; RFC 5340 §A.4.4; `OriginateNetwork`/`v6OriginateNetwork` | NBMA needs a non-DR origination model | `TestOSPFNBMANetworkLSA`, `TestOSPFv3NBMANetworkLSA` | unvalidated |
| A-8 | Every IPv6 interface (including NBMA + PtMP) must originate a `0x0008` Link-LSA; widening `v6ShouldOriginateLinkLSA` is sufficient | `origination_v6_link.go`; RFC 5340 §4.4.3.8 | v6 PtMP/NBMA neighbours never learn the link-local/prefixes; adjacency or routing breaks | `TestOSPFv3NBMALinkLSA`, `TestOSPFv3PtMPLinkLSA` | unvalidated |
| A-9 | The configured NBMA neighbour list seeds the table in Attempt and drives unicast/poll Hellos; no neighbour is learned by multicast on NBMA. IPv4 keys by address; IPv6 keys by router-id with the link-local configured or learned from the first unicast Hello | RFC 2328 §10.1/§9.5; RFC 5340 reuse; guide §7 | NBMA cannot discover neighbours; adjacency never forms | `TestOSPFNBMAPollAttempt`/`TestOSPFv3NBMAPollAttempt`, `ospf-nbma.ci`/`ospfv3-nbma.ci` | unvalidated |
| A-10 | Adding the two enum values + the `nbma-neighbor`/`poll-interval` leaves to BOTH family leaves of the single YANG file, plus the `parseInterface` accept-list per family, is the entire config surface; the network-type string + `IsV6` already thread end to end | `yang/ze-ospf-conf.yang` lines ~178 (IPv4) + ~304 (v6); `config.go`; `instance.go` | more plumbing or a new typed field is needed | package builds; `TestOSPFParseNBMAInterface`/`TestOSPFv3ParseNBMAInterface`, `TestOSPFParsePtMPInterface`/`TestOSPFv3ParsePtMPInterface` | unvalidated |
| A-11 | PtMP/NBMA do not change the DD MTU check or authentication; the IPv4 mask check applies to NBMA/PtMP, the IPv6 Hello has no mask (`validateHelloLocked` already skips it when `IsV6`) | `iface.go` `validateHelloLocked`; RFC 2328 §10.5; RFC 5340 §A.3.2 | adjacency fails for a new reason / auth regression | existing auth + Hello-validate tests stay green; `TestOSPFNBMAAdjacency`, `TestOSPFv3NBMAAdjacency` form Full | unvalidated |
| A-12 | The IPv4 and IPv6 `network-type` leaves are independent; adding the two values to one leaf does not change the other; a value valid on one is not silently accepted on the other | `yang/ze-ospf-conf.yang` has two separate `network-type` leaves (line ~178 with `loopback`, line ~304 without); `config.go` parses each via its own accept-list | a v6-only value is accepted on the IPv4 leaf or vice versa | `TestOSPFNetworkTypeV4V6Isolation` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | PtMP host route omitted or the subnet stub/prefix still emitted -> remote routers cannot reach the PtMP interface, or a phantom subnet route appears (both families) | a PtMP neighbour's /32 (IPv4) or /128 (IPv6) is unreachable, or a subnet route shows where only a host route should | §12.4.1.4 (IPv4) / §A.4.10 (IPv6) host-route rule; `TestOSPFPtMPHostRoute`/`TestOSPFv3PtMPHostRoute` assert the host route present AND the subnet absent |
| R-2 | IPv6 PtMP host route mis-encoded as a Router-LSA stub link (the IPv4 shape) or without the LA-bit | a v6 PtMP /128 is unreachable, or it appears in the Router-LSA not the Intra-Area-Prefix-LSA | encode the /128 with `OptPrefixLA` in the Intra-Area-Prefix-LSA; `TestOSPFv3PtMPHostRoute` |
| R-3 | PtMP originates a Network-LSA (or runs an election) -> a spurious transit vertex corrupts SPF (either family) | a Type-2 (IPv4) / `0x2002` (IPv6) LSA appears for a PtMP segment; SPF builds a network vertex | gate `OriginateNetwork`/`v6OriginateNetwork`/election on a real DR (PtMP has none); `TestOSPFPtMPNoNetworkLSA`/`TestOSPFv3PtMPNoNetworkLSA`, `TestOSPFPtMPNoElection`/`TestOSPFv3PtMPNoElection` |
| R-4 | `shouldAdj` default stays `false` for PtMP/NBMA -> neighbours stuck at 2-Way (both families share the predicate) | a PtMP/NBMA neighbour never leaves 2-Way; no Router-LSA p2p link | split the `nsm.go` default: PtMP true, NBMA DR-gated; `TestOSPFShouldAdjPtMP`/`TestOSPFv3ShouldAdjPtMP`, `TestOSPFShouldAdjNBMA`/`TestOSPFv3ShouldAdjNBMA` |
| R-5 | NBMA never polls a silent configured neighbour (no Attempt / no PollInterval send), or the unicast `dst` is wrong (a multicast group, an IPv4 group on v6, a global instead of link-local) -> adjacency never starts | a configured NBMA neighbour stays Down; the v3 socket rejects the `dst` | seed configured neighbours in Attempt, poll at PollInterval to the right `dst`; §9.4-step-6 Start Hello; `TestOSPFNBMAPollAttempt`/`TestOSPFv3NBMAPollAttempt`, `TestOSPFNBMAUnicastHello`/`TestOSPFv3NBMAUnicastHello` |
| R-6 | Non-broadcast flood still multicasts (`floodDestination` unchanged) -> LSAs never reach NBMA/PtMP-non-broadcast neighbours | a flooded LSA is acked on a broadcast test but silently lost on NBMA | unicast fan-out in `floodDestination`/the initial flood for non-broadcast; `TestOSPFNBMAFloodUnicast`/`TestOSPFv3NBMAFloodUnicast`, `TestOSPFPtMPFloodUnicast`/`TestOSPFv3PtMPFloodUnicast`; FRR `ospfd`/`ospf6d` interop confirms LSAs cross |
| R-7 | PtMP next-hop wrong (subnet-derived / transit-vertex instead of the neighbour's advertised address or link-local) -> traffic mis-steered (either family) | a PtMP route installs with the wrong next-hop / a directly-connected host route missing | §16.1 (IPv4) / §3.8.1 (IPv6) next-hop from the p2p link / adjacency link-local; `TestOSPFPtMPNextHop`/`TestOSPFv3PtMPNextHop` |
| R-8 | NBMA election widened too far (PtMP accidentally elects) or too little (NBMA never elects) -> wrong LSA model on one type (shared gate affects both families) | a PtMP segment elects a DR, or an NBMA DR is never chosen | the gate is exactly `NetworkBroadcast || NetworkNBMA`; `TestOSPFNBMAElection`/`TestOSPFv3NBMAElection` (elects) + `TestOSPFPtMPNoElection`/`TestOSPFv3PtMPNoElection` (does not) |
| R-9 | The new enum value on one family leaf bleeds into the other, or breaks the config round-trip | an IPv4 interface accepts a v6-only behaviour or vice versa; a config diff churns | the two leaves are independent in the single YANG file; add the values to each explicitly; `TestOSPFNetworkTypeV4V6Isolation` + `config_test.go` round-trip |
| R-10 | IPv6 `v6ShouldOriginateLinkLSA` not widened -> NBMA/PtMP interfaces originate no Link-LSA -> neighbours never learn the link-local, SPF cannot resolve the next-hop | a v6 NBMA/PtMP neighbour reaches Full but no route installs | widen the predicate to NBMA + PtMP; `TestOSPFv3NBMALinkLSA`, `TestOSPFv3PtMPLinkLSA` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| IPv4 config `network-type point-to-multipoint` | -> | `parseInterface` accepts it -> `ospfiface.Config.NetworkType` = PtMP -> `iface.Start` takes the PtMP ISM branch (no election) | `TestOSPFParsePtMPInterface` + `test/ospf/ospf-ptmp.ci` |
| IPv6 config `network-type point-to-multipoint` | -> | `parseInterface` (v6) accepts it -> `Config.NetworkType` = PtMP, `IsV6` true -> `iface.Start` PtMP ISM branch | `TestOSPFv3ParsePtMPInterface` + `test/ospfv3/ospfv3-ptmp.ci` |
| IPv4 config `network-type nbma` + a `nbma-neighbor` list (by address) | -> | `parseInterface` resolves the list + `poll-interval` -> seeded in Attempt -> unicast/poll Hello | `TestOSPFParseNBMAInterface` + `test/ospf/ospf-nbma.ci` |
| IPv6 config `network-type nbma` + a `nbma-neighbor` list (by router-id) | -> | `parseInterface` (v6) resolves the list + `poll-interval` -> seeded in Attempt -> unicast/poll Hello to the link-local | `TestOSPFv3ParseNBMAInterface` + `test/ospfv3/ospfv3-nbma.ci` |
| an IPv4 PtMP interface reaches Full | -> | `routerLinks` emits a Type-1 p2p link + a /32 host-route stub; no Network-LSA | `TestOSPFPtMPHostRoute`, `TestOSPFPtMPNoNetworkLSA` |
| an IPv6 PtMP interface reaches Full | -> | `v6RouterLSABody` emits an address-free Type-1 p2p link; the self Intra-Area-Prefix-LSA emits a /128 LA-bit host route; no Network-LSA | `TestOSPFv3PtMPRouterLSALinks`, `TestOSPFv3PtMPHostRoute`, `TestOSPFv3PtMPNoNetworkLSA` |
| an NBMA interface with eligible neighbours comes up (either family) | -> | the shared election gate admits NBMA -> `electDRBDR` -> a DR is elected -> Network-LSA originated (Type-2 / `0x2002` + Link-LSA) | `TestOSPFNBMAElection`, `TestOSPFNBMANetworkLSA`, `TestOSPFv3NBMAElection`, `TestOSPFv3NBMANetworkLSA`, `TestOSPFv3NBMALinkLSA` |
| an LSA is flooded on an NBMA / non-broadcast interface (either family) | -> | `floodDestination`/the initial flood unicasts to each Flood-eligible neighbour (IPv4 address / IPv6 link-local) | `TestOSPFNBMAFloodUnicast`, `TestOSPFv3NBMAFloodUnicast`, `test/ospf/ospf-nbma.ci`, `test/ospfv3/ospfv3-nbma.ci` |

## Acceptance Criteria

| AC ID | Address family | Input / Condition | Expected Behavior |
|-------|----------------|-------------------|-------------------|
| AC-1 | both | An interface configured `network-type point-to-multipoint` (IPv4 or IPv6 leaf) | accepted by YANG + `parseInterface`; the iface enters the point-to-point ISM state (no Waiting, no WaitTimer); no DR/BDR elected; `show ip ospf interface` / `show ipv6 ospf interface` reports network type `point-to-multipoint` |
| AC-2 | both | A PtMP interface forms an adjacency with a neighbour | `should_adj` is true -> the adjacency proceeds to Full (no DR gating); the existing unicast DD/LSReq/LSUpdate exchange completes |
| AC-3 | IPv4 | A PtMP interface at Full with neighbour N (interface address X) | the Router-LSA contains a Type-1 link with LinkID = N's Router ID, LinkData = our interface address, metric = cost; AND a host-route Type-3 stub link with LinkID = our interface address, mask 255.255.255.255, metric 0 |
| AC-4 | IPv6 | A v6 PtMP interface at Full with neighbour N | the Router-LSA contains a Type-1 address-free link: NeighborRouterID = N's Router ID, NeighborInterfaceID = N's Interface ID, this router's Interface ID set, metric = cost; NO transit link for that interface |
| AC-5 | IPv6 | A v6 PtMP interface with a global address X/p | a /128 host route for X with the LA-bit (`OptPrefixLA`) set is advertised in this router's Intra-Area-Prefix-LSA; NO subnet prefix (X/p) is advertised for that interface |
| AC-6 | both | A PtMP interface | NO subnet (network-prefix) stub/prefix and NO Network-LSA (Type-2 / `0x2002`) is originated for that interface |
| AC-7 | IPv4 | SPF computes the route to a PtMP neighbour's address | the next-hop is the neighbour's interface address taken from its advertised p2p link (§16.1), not a subnet-derived next-hop |
| AC-8 | IPv6 | v6 SPF computes the route to a PtMP neighbour's address (or its advertised /128) | the next-hop is the neighbour's IPv6 link-local from the adjacency (`v6NextHop.P2PNextHop`, §3.8.1), not a transit-vertex next-hop |
| AC-9 | both | An interface configured `network-type nbma` with a `nbma-neighbor` list and `poll-interval` (IPv4: addresses + priorities; IPv6: router-ids + optional link-locals + priorities) | accepted by YANG + `parseInterface`; the configured neighbours are seeded in the Attempt state; an eligible (priority > 0) NBMA interface enters Waiting and arms the WaitTimer |
| AC-10 | both | An NBMA interface sending Hellos | Hellos are sent unicast to each configured neighbour (IPv4 address / IPv6 link-local): at HelloInterval to neighbours heard from (state >= Init), at PollInterval (default 120 s) to silent neighbours (Attempt); no multicast Hello is sent on the NBMA interface |
| AC-11 | both | An NBMA segment with two or more eligible routers | a DR (and BDR) is elected using the §9.4 election (reusing `electDRBDR`); the DR originates the Network-LSA exactly as on a broadcast segment (IPv4: Type-2; IPv6: `0x2002` + DR Network-Intra-Area-Prefix-LSA) |
| AC-12 | both | This NBMA router becomes DR or BDR, and a configured neighbour has priority 0 | a Start (Hello) is sent to that priority-0 neighbour so the adjacency begins (§9.4 step 6); the priority-0 neighbour is ineligible for election but still adjacent to the DR/BDR |
| AC-13 | both | An LSA flooded on an NBMA or non-broadcast PtMP interface | it is sent unicast to each Flood-eligible neighbour (Exchange/Loading/Full; IPv4 address / IPv6 link-local), not to a multicast group; the neighbour acknowledges and the LSA installs |
| AC-14 | both | An NBMA adjacency reaches Full | `should_adj` admits only the DR/BDR; a DROther-to-DROther pair on NBMA stays at 2-Way (no adjacency), exactly as broadcast |
| AC-15 | IPv6 | Every v6 NBMA and PtMP interface | originates its `0x0008` Link-LSA (link-local + prefixes), so neighbours learn the link-local and SPF resolves the next-hop |
| AC-16 | both | A broadcast or point-to-point interface (regression) | behaves byte-for-byte as before: broadcast still multicasts Hellos/floods, elects, and originates a Network-LSA; point-to-point still emits per-neighbour p2p links + (IPv4) a subnet stub / (IPv6) address-free p2p links |
| AC-17 | both | The IPv4 and IPv6 `network-type` leaves | are independent: a value valid on one family leaf is not silently accepted on the other; the two enum changes do not bleed across leaves |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures an IPv4 hub-and-spoke link `network-type point-to-multipoint` and expects each spoke reachable as a /32 host route | config -> PtMP ISM -> Full per neighbour -> `routerLinks` host-route + p2p links -> SPF /32 next-hop -> kernel | `test/ospf/ospf-ptmp.ci` |
| 2 | Configures an IPv6 hub-and-spoke link `network-type point-to-multipoint` and expects each spoke reachable as a /128 host route | v6 config -> PtMP ISM -> Full per neighbour -> `v6RouterLSABody` p2p links + the /128 LA-bit host route in the Intra-Area-Prefix-LSA -> v6 SPF /128 next-hop (link-local) -> IPv6 Loc-RIB | `test/ospfv3/ospfv3-ptmp.ci` |
| 3 | Configures a Frame-Relay-style IPv4 `network-type nbma` with a static neighbour list and expects a DR elected and adjacencies formed without multicast | config -> NBMA Attempt/poll -> unicast Hello -> election -> DR Network-LSA -> Full with DR/BDR | `test/ospf/ospf-nbma.ci` |
| 4 | Configures an IPv6 NBMA segment with a static neighbour list and expects a DR elected and adjacencies formed without all-routers multicast | v6 config -> NBMA Attempt/poll -> unicast Hello to link-local -> election -> DR `0x2002` Network-LSA + Link-LSA -> Full with DR/BDR | `test/ospfv3/ospfv3-nbma.ci` |
| 5 | Adds a priority-0 NBMA neighbour (either family) and expects it adjacent to the DR | config -> election makes this router DR/BDR -> §9.4-step-6 Start Hello to the priority-0 neighbour -> adjacency forms | `test/ospf/ospf-nbma.ci` / `test/ospfv3/ospfv3-nbma.ci` (priority-0 step) |
| 6 | Runs `show ip ospf interface` / `show ipv6 ospf interface` on the NBMA/PtMP interface | CLI -> interface snapshot -> network type + (NBMA) poll/neighbour state rendered | `test/ospf/ospf-nbma.ci` / `test/ospfv3/ospfv3-nbma.ci` (show step) |
| 7 | Peers a PtMP/NBMA Ze interface with FRR `ospfd` (IPv4) or `ospf6d` (IPv6) of the matching type | wire (unicast/multicast Hello + unicast flood) -> Full adjacency -> LSDB sync -> routes both ways | `test/interop/scenarios/ospf-ptmp-frr/`, `ospf-nbma-frr/`, `ospfv3-ptmp-frr/`, `ospfv3-nbma-frr/` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFParsePtMPInterface` | `internal/plugins/ospf/config_test.go` | AC-1, A-10: IPv4 `point-to-multipoint` accepted; `interfaceConfig.NetworkType` set | |
| `TestOSPFv3ParsePtMPInterface` | `internal/plugins/ospf/config_test.go` | AC-1, A-10: v6 `point-to-multipoint` accepted; `NetworkType` set, `IsV6` true | |
| `TestOSPFParseNBMAInterface` | `internal/plugins/ospf/config_test.go` | AC-9, A-10: IPv4 `nbma` + `nbma-neighbor` (by address) + `poll-interval` parsed | |
| `TestOSPFv3ParseNBMAInterface` | `internal/plugins/ospf/config_test.go` | AC-9, A-10: v6 `nbma` + `nbma-neighbor` (router-id + link-local) + `poll-interval` parsed | |
| `TestOSPFNetworkTypeV4V6Isolation` | `internal/plugins/ospf/config_test.go` | AC-17, R-9, A-12: both leaves gain the two values; a value set on one leaf does not bleed into the other | |
| `TestOSPFPtMPISMNoElection` / `TestOSPFv3PtMPISMNoElection` | `internal/plugins/ospf/iface/iface_test.go` | AC-1, R-8: PtMP enters PointToPoint state, no Waiting, no election (one per family `Config`) | |
| `TestOSPFNBMAISMWaiting` / `TestOSPFv3NBMAISMWaiting` | `internal/plugins/ospf/iface/iface_test.go` | AC-9: eligible NBMA enters Waiting + WaitTimer; priority 0 -> DROther | |
| `TestOSPFNBMAElection` / `TestOSPFv3NBMAElection` | `internal/plugins/ospf/iface/iface_test.go` | AC-11, A-1/A-2, R-8: NBMA runs `electDRBDR`; elects the same DR a broadcast set would | |
| `TestOSPFPtMPNoElection` / `TestOSPFv3PtMPNoElection` | `internal/plugins/ospf/iface/iface_test.go` | AC-1/AC-6, R-3/R-8: PtMP never elects, never sets DR/BDR | |
| `TestOSPFNBMAUnicastHello` / `TestOSPFv3NBMAUnicastHello` | `internal/plugins/ospf/iface/iface_test.go` | AC-10, R-5: NBMA Hello sent unicast per configured neighbour (address / link-local); no multicast | |
| `TestOSPFNBMAPollAttempt` / `TestOSPFv3NBMAPollAttempt` | `internal/plugins/ospf/iface/iface_test.go` | AC-10, R-5, A-9: silent neighbour polled at PollInterval (Attempt); heard at HelloInterval | |
| `TestOSPFNBMAStartHelloPriorityZero` / `TestOSPFv3NBMAStartHelloPriorityZero` | `internal/plugins/ospf/iface/iface_test.go` | AC-12: DR/BDR sends a Start Hello to a priority-0 neighbour (§9.4 step 6) | |
| `TestOSPFShouldAdjPtMP` / `TestOSPFv3ShouldAdjPtMP` / `TestOSPFShouldAdjNBMA` / `TestOSPFv3ShouldAdjNBMA` | `internal/plugins/ospf/neighbor/nsm_test.go` | AC-2/AC-14, R-4: PtMP always adjacent; NBMA DR/BDR-gated (shared predicate, asserted per family) | |
| `TestOSPFPtMPAdjacency` / `TestOSPFv3PtMPAdjacency` | `internal/plugins/ospf/neighbor/nsm_test.go` | AC-2: PtMP neighbour reaches Full via the unicast DD path | |
| `TestOSPFNBMAAdjacency` / `TestOSPFv3NBMAAdjacency` | `internal/plugins/ospf/neighbor/nsm_test.go` | AC-14, A-11: NBMA reaches Full only with DR/BDR; a DROther pair stays 2-Way | |
| `TestOSPFPtMPHostRoute` | `internal/plugins/ospf/lsdb/origination_test.go` | AC-3/AC-6, A-5, R-1: IPv4 PtMP emits per-neighbour p2p link + /32 host-route stub; NO subnet stub | |
| `TestOSPFPtMPNoNetworkLSA` | `internal/plugins/ospf/lsdb/origination_test.go` | AC-6, R-3: IPv4 PtMP originates no Type-2 Network-LSA | |
| `TestOSPFNBMANetworkLSA` | `internal/plugins/ospf/lsdb/origination_test.go` | AC-11, A-7: IPv4 NBMA DR originates a Type-2 Network-LSA | |
| `TestOSPFv3PtMPRouterLSALinks` | `internal/plugins/ospf/origination_v6_test.go` | AC-4, A-6: `v6RouterLSABody` emits one address-free Type-1 p2p link per Full neighbour for PtMP; no transit link | |
| `TestOSPFv3PtMPHostRoute` | `internal/plugins/ospf/origination_v6_link_test.go` (or `origination_v6_test.go`) | AC-5, A-6, R-1/R-2: the self Intra-Area-Prefix-LSA carries the /128 with the LA-bit; NO subnet prefix | |
| `TestOSPFv3PtMPNoNetworkLSA` | `internal/plugins/ospf/origination_v6_test.go` | AC-6, R-3: v6 PtMP originates no `0x2002` Network-LSA | |
| `TestOSPFv3NBMANetworkLSA` | `internal/plugins/ospf/origination_v6_test.go` | AC-11, A-7: v6 NBMA DR originates a `0x2002` Network-LSA + the DR Network-Intra-Area-Prefix-LSA | |
| `TestOSPFv3NBMALinkLSA` / `TestOSPFv3PtMPLinkLSA` | `internal/plugins/ospf/origination_v6_link_test.go` | AC-15, A-8, R-10: every v6 NBMA + PtMP interface originates its `0x0008` Link-LSA | |
| `TestOSPFNBMAFloodUnicast` / `TestOSPFv3NBMAFloodUnicast` | `internal/plugins/ospf/lsdb/flooding_test.go` | AC-13, A-4, R-6: NBMA initial flood + Ack unicast to each Flood-eligible neighbour (address / link-local) | |
| `TestOSPFPtMPFloodUnicast` / `TestOSPFv3PtMPFloodUnicast` | `internal/plugins/ospf/lsdb/flooding_test.go` | AC-13, R-6: non-broadcast PtMP floods unicast; no DR-relay suppression | |
| `TestOSPFPtMPNextHop` | `internal/plugins/ospf/spf/spf_test.go` | AC-7, A-3, R-7: IPv4 PtMP next-hop = neighbour's advertised interface address | |
| `TestOSPFv3PtMPNextHop` | `internal/plugins/ospf/afstrategy_v6_test.go` | AC-8, A-3, R-7: v6 PtMP next-hop = neighbour's link-local (`v6NextHop.P2PNextHop`); the p2p link translates into the SPF graph | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `poll-interval` (uint16 seconds, both families) | 1..65535 | 65535 | 0 (rejected; must be > 0) | N/A (16-bit field) |
| `nbma-neighbor` priority (uint8, both families) | 0..255 | 255 | N/A | N/A (1 byte); 0 = ineligible (polled, not elected) |
| IPv4 PtMP host-route mask | 255.255.255.255 only | 255.255.255.255 | N/A | N/A (fixed /32) |
| IPv4 PtMP host-route metric | 0 | 0 | N/A | N/A (RFC 2328 §12.4.1.4 host route cost 0) |
| IPv6 PtMP host-route prefix length | 128 only | 128 | N/A (shorter is a subnet, not a host) | N/A (>128 rejected by prefix decode) |
| IPv6 PtMP host-route LA-bit | `OptPrefixLA` set | set | N/A | N/A (single bit) |
| IPv6 prefix words (host route) | `((PrefixLength+31)/32)` = 4 for /128 | 4 words | too short | non-zero padding rejected |
| IPv4 network-type enum | {broadcast, point-to-point, nbma, point-to-multipoint, loopback} | n/a | unknown string rejected by `parseInterface` | n/a |
| IPv6 network-type enum | {broadcast, point-to-point, nbma, point-to-multipoint} | n/a | unknown string rejected by `parseInterface` | n/a (no loopback enum) |
| NBMA configured-neighbour count (both families) | 0..maxNeighbors (1024) | 1024 | N/A | beyond 1024 hits the existing `maxNeighbors` guard |
| IPv6 NBMA neighbour link-local `dst` | a valid IPv6 link-local unicast | `fe80::/10` unicast | a non-IPv6 or non-link-local addr rejected by v3 `SendPacket` | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-nbma` | `test/ospf/ospf-nbma.ci` | an IPv4 NBMA interface with a static neighbour list elects a DR, polls a silent neighbour, forms Full, originates a Network-LSA, and floods unicast | |
| `ospf-ptmp` | `test/ospf/ospf-ptmp.ci` | an IPv4 PtMP interface forms Full with each neighbour, emits /32 host routes + p2p links, no Network-LSA, no DR | |
| `ospf-nbma-config` | `test/ospf/ospf-nbma-config.ci` | config round-trip of IPv4 `network-type nbma` + `nbma-neighbor` + `poll-interval`; invalid values rejected; `show ip ospf interface` renders | |
| `ospfv3-nbma` | `test/ospfv3/ospfv3-nbma.ci` | a v6 NBMA interface elects a DR, polls a silent neighbour, forms Full, originates a `0x2002` Network-LSA + Link-LSA, and floods unicast | |
| `ospfv3-ptmp` | `test/ospfv3/ospfv3-ptmp.ci` | a v6 PtMP interface forms Full with each neighbour, emits address-free p2p links + /128 LA-bit host routes, no Network-LSA, no DR | |
| `ospfv3-nbma-config` | `test/ospfv3/ospfv3-nbma-config.ci` | config round-trip of v6 `network-type nbma` + `nbma-neighbor` + `poll-interval`; invalid values rejected; the IPv4 leaf untouched; `show ipv6 ospf interface` renders | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-ptmp-frr` | `test/interop/scenarios/ospf-ptmp-frr/` | FRR `ospfd` (`ip ospf network point-to-multipoint`) | Ze and FRR form IPv4 PtMP adjacencies (no DR), exchange host-route LSAs, install each other's /32s; next-hops resolve to the neighbour interface address | |
| `ospf-nbma-frr` | `test/interop/scenarios/ospf-nbma-frr/` | FRR `ospfd` (`ip ospf network non-broadcast` + `neighbor`) | Ze and FRR elect a consistent DR over a static list, exchange unicast Hellos/floods, the DR originates the Network-LSA, routes converge both ways | |
| `ospfv3-ptmp-frr` | `test/interop/scenarios/ospfv3-ptmp-frr/` | FRR `ospf6d` (`ipv6 ospf6 network point-to-multipoint`) | Ze and FRR form v6 PtMP adjacencies (no DR), exchange address-free p2p Router-LSA links + /128 host-route prefixes, install each other's /128s; next-hops resolve to the neighbour link-local | |
| `ospfv3-nbma-frr` | `test/interop/scenarios/ospfv3-nbma-frr/` | FRR `ospf6d` (non-broadcast network + `neighbor`) | Ze and FRR elect a consistent DR over a static list, exchange unicast Hellos/floods (link-local source), the DR originates the `0x2002` Network-LSA, IPv6 routes converge both ways | |

> Interop is required: this changes wire behaviour in both families (unicast Hello
> addressing, the NBMA election + Network-LSA over a static list, PtMP host-route
> LSAs/prefixes, unicast flooding). The raw-IP / unicast paths are Linux-only and run
> as QEMU integration tests (`ai/rules/qemu-testing.md`), consistent with the rest of
> the OSPF interop set. NOTE: confirm the FRR `ospf6d` build supports a PtMP /
> non-broadcast network type for IPv6; if a given FRR build does not, document it and
> gate that scenario, but the wire behaviour must still be unit-tested.

### Future (if deferring any tests)
- None. Every AC is covered by a unit, functional, or interop test above. RFC 6845 hybrid broadcast-and-PtMP and virtual links are explicitly out of scope (no test owed here).

## Files to Modify
<!-- MUST include feature code, not only test files -->
- `internal/plugins/ospf/iface/ism.go` (shared) -- add `NetworkNBMA`/`NetworkPointToMultipoint` string constants
- `internal/plugins/ospf/iface/iface.go` (shared) -- `Start()` per-type ISM init (PtMP -> point-to-point branch; NBMA -> Waiting/DROther + WaitTimer); widen the election gate in `runElectionLocked` to include NBMA; `SendHello`/`buildHelloPacket`/the Hello loop unicast/poll fan-out + the §9.4-step-6 Start Hello (the unicast `dst` is `IsV6`-branched: IPv4 address vs link-local); seed configured NBMA neighbours in Attempt; the `Config` struct gains `PollInterval uint16` + an `NBMANeighbors` slice (carrying the IPv4 address OR the IPv6 router-id/link-local + priority)
- `internal/plugins/ospf/neighbor/nsm.go` (shared) -- split `shouldAdj` default: `point-to-multipoint` true, `nbma` DR/BDR-gated; add the constant copies
- `internal/plugins/ospf/neighbor/table.go` (shared) -- ensure the configured-neighbour Attempt seeding + poll interact correctly with `hello()` (no NSM rule change beyond `shouldAdj`)
- `internal/plugins/ospf/lsdb/flooding.go` (shared) -- add the constant copies; `floodDestination`/the initial-flood + Ack path unicast fan-out on non-broadcast interfaces (the per-neighbour destination is the IPv4 address or, when `IsV6`, the link-local); PtMP no DR-relay suppression in `floodExcept`
- `internal/plugins/ospf/lsdb/origination.go` (IPv4) -- `routerLinks` PtMP branch (per-neighbour Type-1 p2p link + host-route Type-3 stub, no subnet stub); gate Network-LSA origination off PtMP (PtMP has no DR)
- `internal/plugins/ospf/origination_v6.go` (IPv6) -- `v6RouterLSABody`: add a `NetworkPointToMultipoint` case (address-free p2p links) + a `NetworkNBMA` case (transit link); add the PtMP /128 LA-bit host route to the self Intra-Area-Prefix-LSA and emit no subnet prefix for a PtMP interface; widen the DR Network-LSA / Network-Intra-Area-Prefix gate to include NBMA; PtMP never originates a Network-LSA
- `internal/plugins/ospf/origination_v6_link.go` (IPv6) -- `v6ShouldOriginateLinkLSA` returns true for `NetworkNBMA` and `NetworkPointToMultipoint` as well
- `internal/plugins/ospf/afstrategy_v6.go` (IPv6) -- VERIFY ONLY (adjust only if a broadcast-only assumption surfaces) that `v6RouterLinks` translates the PtMP p2p link and `v6NextHop.P2PNextHop` resolves the next-hop; no change expected
- `internal/plugins/ospf/spf/` (IPv4) -- VERIFY (adjust only if needed) the PtMP next-hop reads the neighbour's interface address from its p2p link (§16.1); reuse the point-to-point next-hop, no new path expected
- `internal/plugins/ospf/config.go` -- extend the `parseInterface` accept-list with `nbma`/`point-to-multipoint` for BOTH families; add `networkNBMA`/`networkPointToMultipoint` constants; add `NBMANeighbors` (IPv4: address + priority; IPv6: router-id + optional link-local + priority) + `PollInterval uint16` to `interfaceConfig` and parse them per family
- `internal/plugins/ospf/instance.go` -- thread `PollInterval` + the NBMA neighbour list into `ospfiface.Config` for both families; the network-type string + `IsV6` already thread through
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- add `enum nbma;` + `enum point-to-multipoint;` to the IPv4 `network-type` enum (line ~178) AND the `address-family/ipv6/.../network-type` enum (line ~304); add a `nbma-neighbor` list + a `poll-interval` leaf under each interface (IPv4 key `address`; IPv6 key `router-id` + optional `link-local`; both: `priority` default 0, `poll-interval` uint16 default 120 units seconds); the IPv4 enum keeps `loopback`, the v6 enum does not gain it
- `internal/plugins/ospf/cmd_show.go` / `show_summary.go` -- render NBMA poll/neighbour state in `show ip ospf interface` / `show ipv6 ospf interface` if the existing snapshot does not already surface it

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `yang/ze-ospf-conf.yang` -- two enum values + `nbma-neighbor` list + `poll-interval` leaf on BOTH family leaves; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | `poll-interval` `range "1..65535"`; `nbma-neighbor` priority `range "0..255"`; IPv4 `address` `ze:validate` an IPv4; IPv6 `router-id` `ze:validate` a router-id, `link-local` `ze:validate` an IPv6 link-local |
| YANG custom validators | [ ] check | reuse the existing IPv4 / IPv6-link-local / router-id validators if present; otherwise add `ValidateFn`/`CompleteFn` for the missing one |
| CLI commands/flags | [ ] no | reuses `show ip ospf interface` / `show ipv6 ospf interface`; no new command (NBMA/PtMP are config, not a new verb) |
| CLI grammar (action before identifier) | [ ] n/a | no new command |
| Editor autocomplete | [ ] yes | automatic for the YANG enums + `poll-interval`; `CompleteFn` for the neighbour key/link-local if added |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-nbma*.ci`, `ospf-ptmp.ci`, `test/ospfv3/ospfv3-nbma*.ci`, `ospfv3-ptmp.ci` |
| Pipe completeness | [ ] yes | the interface-show commands already route through `ApplyPipes`; no new output path |
| Env var registration | [ ] no | operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] no | no new socket/port/binary; reuses the existing OSPF raw IPv4 / raw IPv6 sockets |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospf_nbma_neighbors` | gauge | `interface`, `af`, `state` (attempt/heard) |
| `ze_ospf_nbma_polls_total` | counter | `interface`, `af` |
| `ze_ospf_ptmp_host_routes` | gauge | `interface`, `af` |

> The single OSPF engine owns one `ze_ospf_*` metric set with an `af` label
> (ipv4/ipv6) distinguishing the two families, mirroring how the engine is one
> plugin spanning both. These extend the umbrella's canonical OSPF metric set and
> are registered by this spec's owner code; the umbrella "Metrics" table gains
> these rows when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPF NBMA + point-to-multipoint network types (both families) |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` -- `network-type nbma|point-to-multipoint`, `nbma-neighbor`, `poll-interval` (IPv4 + `address-family/ipv6`) |
| 3 | CLI command added/changed? | [ ] check | `docs/guide/command-reference.md` -- `show ip ospf interface` / `show ipv6 ospf interface` NBMA/PtMP fields if rendered |
| 4 | API/RPC added/changed? | [ ] no | reuses the existing interface-show RPCs |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains NBMA/PtMP for both families |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- network-types section (NBMA + PtMP, both families) |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospf.md` (IPv4) + `docs/architecture/wire/ospfv3.md` (IPv6) -- unicast Hello addressing, PtMP host-route LSA links/prefixes, unicast flooding |
| 8 | Plugin SDK/protocol changed? | [ ] no | no SDK surface change |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc2328.md` + `rfc/short/rfc5340.md` -- tick the NBMA/PtMP-relevant compliance items |
| 10 | Test infrastructure changed? | [ ] yes (interop scenarios added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPF network-type parity with FRR `ospfd`/`ospf6d` (all four wire types, both families) |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- network-type branching at the ISM/NSM/origination/flood chokepoints (shared) + the per-family origination |
| 13 | Route metadata keys added/changed? | [ ] no | PtMP installs host routes through the existing OSPF route path; no new meta key |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the three `ze_ospf_nbma_*`/`ze_ospf_ptmp_*` series with the `af` label |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] check | umbrella metrics table + `docs/plugin-overview.md` if the metric inventory is listed |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed iface/neighbor/lsdb/origination_v6/config/yang files |
| 17 | Existing docs show examples for this area? | [ ] check | verify OSPF interface config examples (both families) against the extended `network-type` enums |

## Files to Create
- `internal/plugins/ospf/iface/nbma.go` (shared) -- the NBMA Hello unicast/poll loop + the Attempt seeding + the §9.4-step-6 Start Hello (kept out of the broadcast-heavy `iface.go` core; the unicast `dst` is `IsV6`-branched)
- `internal/plugins/ospf/iface/nbma_test.go`, additions to `iface_test.go` (both-family `Config` variants)
- `internal/plugins/ospf/neighbor/nsm_test.go` additions (`TestOSPFShouldAdjPtMP`/`TestOSPFv3ShouldAdjPtMP`, `TestOSPFShouldAdjNBMA`/`TestOSPFv3ShouldAdjNBMA`, `TestOSPFPtMPAdjacency`/`TestOSPFv3PtMPAdjacency`, `TestOSPFNBMAAdjacency`/`TestOSPFv3NBMAAdjacency`)
- `internal/plugins/ospf/lsdb/origination_test.go` additions (IPv4: `TestOSPFPtMPHostRoute`, `TestOSPFPtMPNoNetworkLSA`, `TestOSPFNBMANetworkLSA`)
- `internal/plugins/ospf/origination_v6_test.go` additions (IPv6: `TestOSPFv3PtMPRouterLSALinks`, `TestOSPFv3PtMPNoNetworkLSA`, `TestOSPFv3NBMANetworkLSA`)
- `internal/plugins/ospf/origination_v6_link_test.go` additions (IPv6: `TestOSPFv3PtMPHostRoute`, `TestOSPFv3NBMALinkLSA`, `TestOSPFv3PtMPLinkLSA`)
- `internal/plugins/ospf/lsdb/flooding_test.go` additions (`TestOSPFNBMAFloodUnicast`/`TestOSPFv3NBMAFloodUnicast`, `TestOSPFPtMPFloodUnicast`/`TestOSPFv3PtMPFloodUnicast`)
- `internal/plugins/ospf/spf/spf_test.go` addition (IPv4: `TestOSPFPtMPNextHop`)
- `internal/plugins/ospf/afstrategy_v6_test.go` addition (IPv6: `TestOSPFv3PtMPNextHop`)
- `internal/plugins/ospf/config_test.go` additions (`TestOSPFParsePtMPInterface`/`TestOSPFv3ParsePtMPInterface`, `TestOSPFParseNBMAInterface`/`TestOSPFv3ParseNBMAInterface`, `TestOSPFNetworkTypeV4V6Isolation`)
- `test/ospf/ospf-nbma.ci`, `test/ospf/ospf-ptmp.ci`, `test/ospf/ospf-nbma-config.ci`
- `test/ospfv3/ospfv3-nbma.ci`, `test/ospfv3/ospfv3-ptmp.ci`, `test/ospfv3/ospfv3-nbma-config.ci`
- `test/interop/scenarios/ospf-ptmp-frr/`, `ospf-nbma-frr/`, `ospfv3-ptmp-frr/`, `ospfv3-nbma-frr/` -- each with `ze.conf`, `frr.conf`, `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm the broadcast election, p2p ISM/NSM, Network-LSA (Type-2 + `0x2002`), Link-LSA, the v6 next-hop, and the unicast retransmit / v3 transport paths exist |
| 3. Wiring phase | Wiring Test table -- both-family enums + parse + failing wiring tests |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- the both-family config surface + failing wiring tests
   - Tests: `TestOSPFParsePtMPInterface`/`TestOSPFv3ParsePtMPInterface`, `TestOSPFParseNBMAInterface`/`TestOSPFv3ParseNBMAInterface`, `TestOSPFNetworkTypeV4V6Isolation`, `test/ospf/ospf-nbma-config.ci`, `test/ospfv3/ospfv3-nbma-config.ci`
   - Files: `yang/ze-ospf-conf.yang` (both enums + both `nbma-neighbor` + `poll-interval`), `config.go` (both accept-lists + new struct fields + parse), `iface/ism.go` (constants), the `neighbor`/`lsdb` constant copies; thread through `instance.go` and the iface `Config`
   - Verify: the two new network types parse and reach the iface runtime in both families; the behaviour branches are stubs so the deeper tests still fail
2. **Phase: PtMP ISM/NSM + adjacency (shared)** -- treat PtMP as point-to-point
   - Tests: `TestOSPFPtMPISMNoElection`/`TestOSPFv3PtMPISMNoElection`, `TestOSPFPtMPNoElection`/`TestOSPFv3PtMPNoElection`, `TestOSPFShouldAdjPtMP`/`TestOSPFv3ShouldAdjPtMP`, `TestOSPFPtMPAdjacency`/`TestOSPFv3PtMPAdjacency`
   - Files: `iface/iface.go` (`Start` PtMP -> point-to-point branch, no election), `neighbor/nsm.go` (`shouldAdj` PtMP true) -- exercised on both-family `Config`
   - Verify: PtMP forms Full with every neighbour and never elects, in both families
3. **Phase: IPv4 PtMP origination + next-hop** -- host route + p2p links, no Network-LSA
   - Tests: `TestOSPFPtMPHostRoute`, `TestOSPFPtMPNoNetworkLSA`, `TestOSPFPtMPNextHop`
   - Files: `lsdb/origination.go` (`routerLinks` PtMP branch + Network-LSA gate), `spf/` (verify next-hop)
   - Verify: IPv4 PtMP emits per-neighbour p2p links + a /32 host route, no subnet stub, no Network-LSA; next-hop = neighbour interface address
4. **Phase: IPv6 PtMP origination + next-hop** -- address-free p2p links + /128 host route, no Network-LSA
   - Tests: `TestOSPFv3PtMPRouterLSALinks`, `TestOSPFv3PtMPHostRoute`, `TestOSPFv3PtMPNoNetworkLSA`, `TestOSPFv3PtMPLinkLSA`, `TestOSPFv3PtMPNextHop`
   - Files: `origination_v6.go` (`v6RouterLSABody` PtMP case + the /128 LA-bit host route in the self Intra-Area-Prefix-LSA + the Network-LSA gate excluding PtMP), `origination_v6_link.go` (`v6ShouldOriginateLinkLSA` PtMP), `afstrategy_v6.go` (verify next-hop)
   - Verify: IPv6 PtMP emits address-free p2p links + a /128 LA-bit host route, no subnet prefix, no Network-LSA; next-hop = neighbour link-local
5. **Phase: NBMA ISM + election + Network-LSA (both families)** -- broadcast-like over a static list
   - Tests: `TestOSPFNBMAISMWaiting`/`TestOSPFv3NBMAISMWaiting`, `TestOSPFNBMAElection`/`TestOSPFv3NBMAElection`, `TestOSPFShouldAdjNBMA`/`TestOSPFv3ShouldAdjNBMA`, `TestOSPFNBMAAdjacency`/`TestOSPFv3NBMAAdjacency`, `TestOSPFNBMANetworkLSA`, `TestOSPFv3NBMANetworkLSA`, `TestOSPFv3NBMALinkLSA`
   - Files: `iface/iface.go` (election gate widened, Waiting/DROther init, NBMA neighbour seeding), `neighbor/nsm.go` (`shouldAdj` NBMA DR-gated), `lsdb/origination.go` (IPv4 Network-LSA when DR), `origination_v6.go` (IPv6 transit link + DR Network/Network-Intra-Area-Prefix gate widened to NBMA), `origination_v6_link.go` (`v6ShouldOriginateLinkLSA` NBMA)
   - Verify: NBMA elects a DR, forms Full only with DR/BDR, originates a Network-LSA (Type-2 / `0x2002` + Link-LSA), in both families
6. **Phase: NBMA unicast Hello + poll + Start (shared)** -- the Attempt-state Hello model
   - Tests: `TestOSPFNBMAUnicastHello`/`TestOSPFv3NBMAUnicastHello`, `TestOSPFNBMAPollAttempt`/`TestOSPFv3NBMAPollAttempt`, `TestOSPFNBMAStartHelloPriorityZero`/`TestOSPFv3NBMAStartHelloPriorityZero`
   - Files: `iface/nbma.go` (unicast/poll loop, Start Hello), `iface/iface.go` (Hello-send dispatch by type; `dst` `IsV6`-branched)
   - Verify: Hellos unicast per neighbour (address / link-local), polled at PollInterval, Start Hello to priority-0 when DR/BDR; no multicast on the NBMA interface
7. **Phase: Non-broadcast flooding (shared)** -- unicast fan-out
   - Tests: `TestOSPFNBMAFloodUnicast`/`TestOSPFv3NBMAFloodUnicast`, `TestOSPFPtMPFloodUnicast`/`TestOSPFv3PtMPFloodUnicast`
   - Files: `lsdb/flooding.go` (`floodDestination`/initial-flood + Ack unicast fan-out; the per-neighbour destination is the IPv4 address or, when `IsV6`, the link-local; PtMP no DR-relay)
   - Verify: LSAs reach each Flood-eligible neighbour by unicast on non-broadcast interfaces, in both families
8. **Phase: CLI/show + metrics** -- user surface
   - Tests: the `.ci` show steps
   - Files: `cmd_show.go`/`show_summary.go` (NBMA poll/neighbour state, both families), metric registration with the `af` label
   - Verify: `show ip ospf interface` / `show ipv6 ospf interface` render NBMA/PtMP; the three metric series register
9. **Functional tests** -> `ospf-nbma.ci`, `ospf-ptmp.ci`, `ospf-nbma-config.ci`, `ospfv3-nbma.ci`, `ospfv3-ptmp.ci`, `ospfv3-nbma-config.ci`
10. **RFC refs** -> add `// RFC 2328 §9.5/§10.1/§12.4.1.4/§16.1` (IPv4) and `// RFC 5340 §A.4.3/§A.4.4/§A.4.10/§A.4.1/§2.9/§3.8.1` (IPv6) comments on the enforcing code
11. **Interop** -> `ospf-ptmp-frr`, `ospf-nbma-frr`, `ospfv3-ptmp-frr`, `ospfv3-nbma-frr` QEMU scenarios
12. **Full verification** -> `make ze-verify`
13. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has a file:line implementation |
| Feature completeness | each user story has a working path; all four wire network types now exist in BOTH families (parity with FRR `ospfd`/`ospf6d` broadcast/p2p/PtMP/NBMA) |
| Correctness | PtMP: no DR/no Network-LSA; IPv4 host route /32 Router-LSA stub; IPv6 host route /128 LA-bit Intra-Area-Prefix-LSA (NOT a Router-LSA stub); per-neighbour p2p link; neighbour-address/link-local next-hop. NBMA: election reused, unicast/poll Hello, Start Hello to priority-0, DR Network-LSA (Type-2 / `0x2002`+Link-LSA), unicast flood |
| Naming | YANG `nbma`/`point-to-multipoint`/`nbma-neighbor`/`poll-interval` kebab-case on both leaves; `ze_ospf_nbma_*`/`ze_ospf_ptmp_*` metrics with the `af` label; `NetworkNBMA`/`NetworkPointToMultipoint` constants consistent across iface/neighbor/lsdb |
| Data flow | the network-type string + `IsV6` thread unchanged; shared branches live only at `iface.Start`, the election gate, `shouldAdj`, `floodDestination`; per-family origination lives only in `lsdb/origination.go` (IPv4) and `origination_v6*.go` (IPv6); the shared packages stay AF-neutral |
| CLI grammar | no new command (reuses the interface-show commands) |
| Doctor checks | none added (no new runtime dependency) -- confirm |
| YANG validation | the two enum values added to BOTH leaves; `poll-interval` range; `nbma-neighbor` priority range + key validation (IPv4 address / IPv6 router-id + link-local); the IPv4 enum keeps `loopback`, the v6 enum does not |
| Prometheus counters | the three series defined, registered, listed with the `af` label; umbrella table updated |
| Rule: plugin-self-containment | all behaviour stays inside `internal/plugins/ospf`; no NBMA/PtMP spelling in generic packages |
| Rule: buffer-first | Hello + Router-LSA links + the host route + unicast flood encode buffer-first; one built buffer reused per unicast neighbour |
| Rule: ze-divergences | no `fmt`/`+` on the wire path; origination uses the existing `WriteTo` encoders |
| Regression | broadcast + point-to-point behave exactly as before (AC-16) in both families; the two family leaves stay independent (AC-17) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Two new network types parse in both families | `go test ./internal/plugins/ospf -run 'TestOSPF(v3)?Parse(NBMA|PtMP)Interface'` |
| Both enums gain the two values, leaves independent | `grep -nE 'nbma|point-to-multipoint' internal/plugins/ospf/yang/ze-ospf-conf.yang` shows them under both the IPv4 (line ~178) and v6 (line ~304) leaves; `go test ./internal/plugins/ospf -run TestOSPFNetworkTypeV4V6Isolation` |
| IPv4 PtMP host-route + no Network-LSA | `go test ./internal/plugins/ospf/lsdb -run 'TestOSPFPtMP(HostRoute|NoNetworkLSA)'` |
| IPv6 PtMP host-route + p2p links + no Network-LSA | `go test ./internal/plugins/ospf -run 'TestOSPFv3PtMP(HostRoute|RouterLSALinks|NoNetworkLSA)'` |
| NBMA election + Network-LSA (both families) | `go test ./internal/plugins/ospf/... -run 'TestOSPF(v3)?NBMA(Election|NetworkLSA)'` |
| IPv6 NBMA + PtMP Link-LSA | `go test ./internal/plugins/ospf -run 'TestOSPFv3(NBMA|PtMP)LinkLSA'` |
| Unicast Hello + poll (both families) | `go test ./internal/plugins/ospf/iface -run 'TestOSPF(v3)?NBMA(UnicastHello|PollAttempt|StartHelloPriorityZero)'` |
| Non-broadcast unicast flood (both families) | `go test ./internal/plugins/ospf/lsdb -run 'TestOSPF(v3)?(NBMA|PtMP)FloodUnicast'` |
| PtMP next-hop reused (both families) | `go test ./internal/plugins/ospf/... -run 'TestOSPF(v3)?PtMPNextHop'` |
| Three metric series registered | `grep -rn 'ze_ospf_nbma_\|ze_ospf_ptmp_' internal/plugins/ospf` |
| Functional tests present | `ls test/ospf/ospf-nbma.ci test/ospf/ospf-ptmp.ci test/ospf/ospf-nbma-config.ci test/ospfv3/ospfv3-nbma.ci test/ospfv3/ospfv3-ptmp.ci test/ospfv3/ospfv3-nbma-config.ci` |
| Interop scenarios present | `ls test/interop/scenarios/ospf-ptmp-frr/ ospf-nbma-frr/ ospfv3-ptmp-frr/ ospfv3-nbma-frr/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | the IPv4 `nbma-neighbor` address must validate as IPv4; the IPv6 `router-id` as a router-id and `link-local` as a valid IPv6 link-local unicast (rejected at config validation, never passed to `SendPacket`); the configured list is bounded by the existing `maxNeighbors` (1024) guard so a huge list cannot exhaust memory |
| Resource exhaustion | the unicast Hello/flood fan-out is O(neighbours); cap at `maxNeighbors`; the poll timer cannot schedule faster than PollInterval; a flood to N neighbours reuses one buffer |
| Trust boundary | NBMA neighbours are operator-configured (no dynamic discovery), so an off-path host cannot inject itself; received Hellos still pass the existing per-interface authentication (RFC 7166 trailer for IPv6, unchanged) |
| Error leakage | a failed unicast send to one neighbour is logged/counted, not fatal; it does not abort the fan-out to the others; no secret material logged |
| Spoofing | the IPv4 unicast Hello source is verified against the configured neighbour address before accepting on NBMA; the IPv6 unicast `dst`/source uses the link-local learned from the neighbour's authenticated Hello (or the configured link-local); no off-link source is honoured (the v3 transport binds the interface) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to the relevant phase and implement |
| Shared chokepoint missing the new AF-neutral branch | Add it once (keyed on `NetworkType`, never on `IsV6` except the multicast group / unicast destination); do not duplicate it per family |
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
NBMA and point-to-multipoint are not new subsystems and not address-family-specific
in their control logic: NBMA is the existing broadcast election + Network-LSA over a
manually configured neighbour list with unicast/poll Hellos, and PtMP is the
existing point-to-point ISM/NSM/next-hop with per-neighbour p2p Router-LSA links
plus a host route and no DR. Because Ze runs OSPF as ONE engine with AF-neutral
`iface`/`neighbor`/`lsdb` packages string-keyed on the network type, the entire
ISM/NSM/election/flood mechanism is written once and serves both families via the
`IsV6` flag (which only selects the multicast group and the unicast destination
form). The per-family work is purely the wire/LSA encoding: IPv4 carries addresses
in the Router-LSA (the host route is a /32 Type-3 stub link), while IPv6 Router-LSAs
are address-free and the host route is a /128 LA-bit prefix in the
Intra-Area-Prefix-LSA, and the NBMA Network-LSA is a Type-2 (IPv4) versus a `0x2002`
+ Link-LSA (IPv6).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One feature spec covering both address families | two version-split specs | the control logic (ISM/NSM/election/flood) is AF-neutral and written once in the shared packages; splitting by version would duplicate the shared mechanism description and obscure that IPv4/IPv6 are two families of one OSPF engine |
| NBMA reuses `electDRBDR` by only widening the `iface.go` gate | a separate NBMA election | the §9.4 election is identical for broadcast and NBMA and is AF-neutral; only the candidate source (configured list) and Hello transport (unicast) differ |
| PtMP reuses the point-to-point ISM/NSM/next-hop in both families | a dedicated PtMP ISM | RFC 2328/5340 define PtMP precisely as "a collection of point-to-point links"; the only origination delta is the host route |
| IPv4 host route as a /32 Router-LSA stub link; IPv6 host route as a /128 LA-bit Intra-Area-Prefix-LSA prefix | one shared host-route encoding | OSPFv2 Router-LSAs carry addresses (§12.4.1.4); OSPFv3 Router-LSAs are address-free and prefixes live only in the Intra-Area-Prefix-LSA with the LA-bit (§A.4.3/§A.4.10/§A.4.1) |
| Add the two enum values to BOTH family leaves of the single YANG file | a single shared network-type leaf | the two families have independent interface config in the schema; a single shared leaf would let an IPv4 interface accept a v6-only value and vice versa, and break per-family validation (the IPv4 leaf keeps `loopback`, the v6 leaf does not) |
| Unicast flood fan-out only at `floodDestination`/initial flood, AF-neutral with an `IsV6`-branched destination | rewrite the flood path | the retransmit path is already unicast (`neighborAddr`); only the initial flood/ack used a multicast group; the only AF difference is the destination form |
| Keep the NBMA Hello/poll loop in a new `iface/nbma.go` | inline in `iface.go` | keeps the broadcast-heavy core readable and the NBMA-specific Attempt/poll/Start logic self-contained, shared by both families |
| One `ze_ospf_*` metric set with an `af` label | separate `ze_ospf_*`/`ze_ospfv3_*` series | mirrors the one-engine model: a single metric family distinguishes the two address families by label, as the engine distinguishes them by `IsV6` |

## Known Limitations
- Virtual links remain out of scope (the OSPF virtual-links extension spec); an NBMA/PtMP interface cannot serve as a virtual-link transit here.
- The RFC 6845 hybrid broadcast-and-PtMP interface type is not this PtMP; it is future.
- RFC 5838 multi-AF on the NBMA/PtMP interface is out of scope; the IPv6 Instance ID stays explicit.
- The non-broadcast PtMP variant uses the same `nbma-neighbor` list for its explicit neighbour set; the broadcast PtMP variant (multicast Hello) is the default.

## RFC Documentation

Add `// RFC 2328 Section X.Y: "<quoted requirement>"` (IPv4 family) and
`// RFC 5340 Section X.Y: "<quoted requirement>"` (IPv6 family) above the enforcing code:
- RFC 2328 §9.5 / RFC 5340 §2.9 -- unicast Hello on NBMA (HelloInterval to eligible, PollInterval to ineligible), Hello-to-all on PtMP, unicast `dst` = neighbour address (IPv4) / link-local (IPv6)
- RFC 2328 §9.4 step 6 -- Start Hello to priority-0 NBMA neighbours when this router becomes DR/BDR (applied to both families)
- RFC 2328 §10.1 / §10.3 -- the NBMA-only Attempt state and Start event (shared)
- RFC 2328 §10.4 -- `should_adj`: always adjacent on PtMP, DR/BDR-only on NBMA (shared)
- RFC 2328 §12.4.1.4 / App A.4.2 -- the IPv4 PtMP host-route stub link + per-neighbour p2p links; no Network-LSA on PtMP
- RFC 5340 §A.4.3 -- the IPv6 PtMP address-free Type-1 p2p Router-LSA link per Full neighbour
- RFC 5340 §A.4.4 -- the IPv6 NBMA DR-originated `0x2002` Network-LSA
- RFC 5340 §A.4.10 / §A.4.1 -- the IPv6 PtMP /128 LA-bit host route in the Intra-Area-Prefix-LSA; word-padded prefix encoding
- RFC 2328 §16.1 / RFC 5340 §3.8.1 -- the PtMP next-hop from the neighbour's advertised interface address (IPv4) / adjacency link-local (IPv6)
- RFC 2328 §13.3 -- unicast flooding on non-broadcast interfaces (shared)

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
| NBMA (both families): manual neighbour list + DR election + unicast/poll Hello | functional + interop | `ospf-nbma.ci`/`ospf-nbma-frr`, `ospfv3-nbma.ci`/`ospfv3-nbma-frr` |
| NBMA: DR originates the Network-LSA (Type-2 / `0x2002`+Link-LSA) | unit | `TestOSPFNBMANetworkLSA`, `TestOSPFv3NBMANetworkLSA`, `TestOSPFv3NBMALinkLSA` |
| PtMP (both families): per-neighbour adjacency, no DR, host-route origination | functional + interop | `ospf-ptmp.ci`/`ospf-ptmp-frr`, `ospfv3-ptmp.ci`/`ospfv3-ptmp-frr` |
| PtMP IPv4: /32 host route + p2p links, no Network-LSA | unit | `TestOSPFPtMPHostRoute`, `TestOSPFPtMPNoNetworkLSA` |
| PtMP IPv6: /128 LA-bit host route + address-free p2p links, no Network-LSA | unit | `TestOSPFv3PtMPHostRoute`, `TestOSPFv3PtMPRouterLSALinks`, `TestOSPFv3PtMPNoNetworkLSA` |
| Non-broadcast unicast flooding (both families) | unit + interop | `TestOSPFNBMAFloodUnicast`, `TestOSPFv3NBMAFloodUnicast`, `ospf-nbma-frr`, `ospfv3-nbma-frr` |
| Broadcast/p2p unchanged + leaves independent (regression) | unit + existing suite | `TestOSPFNetworkTypeV4V6Isolation`, existing OSPF tests green (both families) |

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
- [ ] AC-1..AC-17 all demonstrated
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
- [ ] RFC 2328 + RFC 5340 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (two new enum values per family + branches; no new subsystem)
- [ ] No speculative features (only RFC 2328/5340 NBMA + PtMP; no RFC 6845 hybrid)
- [ ] Single responsibility per component (shared mechanism vs per-family origination)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (behaviour stays inside the OSPF plugin; shared packages stay AF-neutral)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-ptmp-frr`, `ospf-nbma-frr`, `ospfv3-ptmp-frr`, `ospfv3-nbma-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-8-nbma-p2mp.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-8-nbma-p2mp.md`
