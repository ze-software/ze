# Spec: ospfv3-ext-7 -- OSPFv3 NBMA + Point-to-Multipoint Network Types (RFC 5340)

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
3. `rfc/short/rfc5340.md` -- OSPFv3: §2.1 link (not subnet) model, §2.9 transport (`ff02::5`/`ff02::6`, link-local source, unicast retransmit), §A.3.2 Hello (Interface ID, no network mask), §A.4.2 LSA scope bits, §A.4.3 Router-LSA (address-free p2p/transit links keyed by Router ID + Interface ID), §A.4.4 Network-LSA (DR-originated on broadcast/NBMA), §A.4.10 Intra-Area-Prefix-LSA (where IPv6 prefixes including the P2MP /128 host route live), §A.4.1 prefix encoding (LA-bit local-address, `((PrefixLength+31)/32)` words)
4. `plan/spec-ospfv3-0-umbrella.md` -- delivered OSPFv3 umbrella: "In Scope" shipped point-to-point + broadcast (DR/BDR + Network-LSA); NBMA/PtMP were NOT listed and are added here; the AC-3 DR/BDR + Network-LSA contract this spec reuses for NBMA
5. `plan/spec-ospf-ext-8-nbma-p2mp.md` -- the OSPFv2 SIBLING adding the SAME two network types on the SAME shared `iface`/`neighbor`/`lsdb` packages (it owns the shared ISM/NSM/election/flood mechanism this spec branches into via the `IsV6` flag); this spec owns the OSPFv3-specific origination, the v6 YANG enum, and the v3 next-hop
6. `internal/plugins/ospf/iface/iface.go` -- the AF-shared per-interface runtime: `Start()` ISM switch (line ~264), `SendHello()` (line ~613, v6 sends to `allSPFRoutersV6`), `runElectionLocked()` (line ~640, gated `NetworkType != NetworkBroadcast`), `buildHelloPacket()`, the `Config` struct (line ~14, carries `NetworkType`/`IsV6`/`Priority`), `Sender.SendPacket(name, dst, payload)`
7. `internal/plugins/ospf/iface/ism.go` -- the `State` enum + `NetworkBroadcast`/`NetworkPointToPoint`/`NetworkLoopback` string constants this spec extends
8. `internal/plugins/ospf/neighbor/nsm.go` -- `shouldAdj` (line 14: `point-to-point` true, `broadcast`/`""` DR/BDR-gated, `default` false -- the default must split: PtMP true, NBMA DR-gated)
9. `internal/plugins/ospf/origination_v6.go` -- `v6RouterLSABody` (line 232: switches `NetworkPointToPoint`/`NetworkBroadcast` only; PtMP needs per-neighbour p2p links, NBMA needs the transit link), `v6OriginateNetwork` (line 204, DR Network-LSA), `v6OriginateSelf` (line 70, the self-LSA sweep + the Network-Intra-Area-Prefix gate at line ~134 currently `NetworkType != NetworkBroadcast`)
10. `internal/plugins/ospf/origination_v6_link.go` -- `v6ShouldOriginateLinkLSA` (line 22: gated `NetworkBroadcast || NetworkPointToPoint`; NBMA + PtMP must also originate a Link-LSA), the Intra-Area-Prefix host-route path
11. `internal/plugins/ospf/afstrategy_v6.go` -- `v6NextHop.P2PNextHop`/`TransitNextHop` (lines 398/405: next-hop = neighbour link-local via `neighbors.AddressOf`; PtMP reuses `P2PNextHop` unchanged), `BuildGraph`/`v6RouterLinks` (line 127, translates p2p/transit links into the shared SPF graph)
12. `internal/plugins/ospf/lsdb/flooding.go` -- `floodDestination` (line 347: v6 returns `allSPFRoutersV6`; non-broadcast must unicast fan-out), `floodExcept` (line 258), `neighborAddr` (line 516, already unicast), `InterfaceInfo`/`NeighborInfo` (carry `IsV6`, `NetworkType`, `IPv6LinkLocal`, `InterfaceID`)
13. `internal/plugins/ospf/v3/transport/transport.go` -- `SendPacket(name, dst netip.Addr, payload)` (line 474: already takes an arbitrary `dst`, so unicast to a neighbour link-local works through the existing send path; no new transport plumbing)
14. `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the v6 `address-family/ipv6/.../network-type` enum (lines 304-307, `broadcast`/`point-to-point` ONLY) + where the new v6 `nbma-neighbor` list and `poll-interval` leaf attach
15. `internal/plugins/ospf/config.go` -- `parseInterface` network-type accept-list + `interfaceConfig` struct; `internal/plugins/ospf/instance.go` -- `ifaceConfig`/`neighborInterfaceConfig` thread `NetworkType`/`IsV6` into the shared runtime

## Task

Add the two remaining OSPFv3 interface **network types** -- **NBMA** (non-broadcast
multi-access) and **point-to-multipoint (PtMP)** -- to the native OSPFv3 ("v6")
implementation, which lives in the SAME plugin tree as OSPFv2
(`internal/plugins/ospf/`) but with a separate v3 wire codec (`v3/packet`), v3
types (`v3/types`), v3 transport (`v3/transport`), and v3 address-family strategy
(`afstrategy_v6.go`). The OSPFv3 umbrella (`plan/spec-ospfv3-0-umbrella.md`)
shipped OSPFv3 with **point-to-point** and **broadcast** (DR/BDR + Network-LSA)
only; NBMA and PtMP were not in its scope. This spec closes that gap on the v3
interface/neighbour model with **link-local addressing**.

The shapes mirror the OSPFv2 sibling (`plan/spec-ospf-ext-8-nbma-p2mp.md`), but
the data model differs because OSPFv3 runs **per link, not per IP subnet**
(RFC 5340 §2.1): Hellos carry an **Interface ID** (no network mask, §A.3.2),
topology LSAs are **address-free** (Router-LSA links are keyed by Router ID +
Interface ID, §A.4.3), and **all IPv6 prefixes -- including the PtMP host route --
live in the Intra-Area-Prefix-LSA** (§A.4.10), not in the Router-LSA. Next-hops
resolve to the neighbour's **IPv6 link-local** from the adjacency table (§3.8.1),
which the v3 strategy already does (`afstrategy_v6.go` `v6NextHop`).

**NBMA** (RFC 5340 reuses the RFC 2328 §9.4 election and §9.5/§10 Hello/poll
model; the v6 Network-LSA is §A.4.4) treats one multi-access link with no
all-routers multicast reach available for discovery: the router is told its peers
by a **manually configured neighbour list** (configured by Router ID with the
neighbour's link-local learned from its Hellos, or by link-local address). A
**DR/BDR is elected** among the configured, eligible (priority > 0) routers
(the existing election is reused verbatim), and the DR still originates the v6
**Network-LSA** (`0x2002`) plus the DR's Network-referencing
Intra-Area-Prefix-LSA. Hellos are sent **unicast to each configured neighbour**
at HelloInterval to neighbours we have heard from and at the slower
**PollInterval** to silent ones. A priority-0 neighbour is **ineligible** for the
election but is still polled, and when this router becomes DR/BDR it sends a Start
Hello to those priority-0 neighbours (RFC 2328 §9.4 step 6, applied to v3).
Adjacency follows the broadcast rule (`should_adj`: only with the DR/BDR).

**Point-to-multipoint** (RFC 5340 follows the RFC 2328 §9.5/§10.4/§12.4.1.4
semantics over the v3 LSA model) treats the medium as a **collection of
point-to-point links**: **no DR, no BDR, no Network-LSA**; every router forms a
full adjacency with every other reachable router (`should_adj` always true). The
interface contributes one **Type-1 (point-to-point) Router-LSA link per Full
neighbour** (address-free: NeighborRouterID + NeighborInterfaceID, §A.4.3), and
the interface's own reachable address is advertised as a **/128 host route with
the LA-bit set in the Intra-Area-Prefix-LSA** (§A.4.1 LA-bit, §A.4.10) so other
routers can reach the PtMP interface; it does NOT advertise the link's subnet
prefix as a transit/network prefix. SPF resolves the next-hop to each PtMP
neighbour from the adjacency's link-local exactly as point-to-point
(`v6NextHop.P2PNextHop`, unchanged).

Both variants reuse the existing unicast adjacency-packet path (DD / LS Request /
LS Update retransmission already go to the neighbour link-local via the v3
transport, which accepts an arbitrary `dst`); the only flooding delta is the
**initial flood / Ack destination** (`floodDestination`), which on a v6
non-broadcast interface must be a per-neighbour unicast fan-out instead of the
`ff02::5`/`ff02::6` group.

This is an additive, self-contained extension. A v6 broadcast or point-to-point
interface behaves exactly as today; the new behaviour is reachable only when a v6
interface is configured `network-type nbma` or `network-type point-to-multipoint`.

**Relationship to the v2 sibling (`spec-ospf-ext-8`).** The `iface`, `neighbor`,
and `lsdb` packages are address-family-shared (they carry an `IsV6` flag and a
`NetworkType` string). The shared ISM/NSM/election/flood *mechanism* for the two
new network types (the `iface.Start` ISM branches, the election gate, the
`shouldAdj` split, the `floodDestination` unicast fan-out, the SendHello
unicast/poll loop, the iface `Config` NBMA fields) is owned by the v2 sibling and
is AF-neutral by construction (it branches on `NetworkType`, never on AF, except
where it already branches on `IsV6` for the multicast group). This spec **depends
on that mechanism existing** and adds ONLY the OSPFv3-specific pieces: the v6 YANG
enum + config resolution, the v6 origination (`v6RouterLSABody` PtMP/NBMA
branches, the v6 PtMP /128 host route in the Intra-Area-Prefix-LSA, the NBMA v6
Network-LSA + Link-LSA gating), the v6 next-hop confirmation, and FRR `ospf6d`
interop. No v3 wire struct, LSA codec, or v3 strategy is shared with v2.

### In scope (this spec)

| Item | Detail |
|------|--------|
| v6 network-type config extension | Add `nbma` + `point-to-multipoint` to the **v6** (`address-family/ipv6`) `network-type` enum (lines 304-307) and to the `parseInterface` accept-list for the v6 interface; the network-type string already threads through `ospfiface.Config`/`InterfaceInfo` |
| v6 NBMA neighbour config | A per-v6-interface `nbma-neighbor` list (key `router-id` with an optional `link-local` address, leaf `priority` default 0) and a per-interface `poll-interval` leaf (default 120 s); resolved into the v6 interface runtime |
| NBMA ISM + election (v6) | On InterfaceUp an eligible (priority > 0) v6 NBMA interface enters Waiting + WaitTimer and runs the election over the configured neighbours; priority 0 goes to DROther; reuses the shared `iface` election unchanged (mechanism owned by the v2 sibling; this spec validates it on a v6 interface) |
| NBMA unicast Hello + poll (v6) | Hellos sent unicast to each configured neighbour's link-local; HelloInterval to heard neighbours, PollInterval to silent ones (Attempt); the Start Hello to priority-0 neighbours when this router becomes DR/BDR |
| NBMA v6 Network-LSA + Link-LSA | The DR of a v6 NBMA segment originates the `0x2002` Network-LSA (the existing `v6OriginateNetwork` path) and the DR Network-referencing Intra-Area-Prefix-LSA; every NBMA interface originates its `0x0008` Link-LSA (extend `v6ShouldOriginateLinkLSA`) |
| PtMP adjacency (v6) | `should_adj` true for v6 PtMP; no DR/BDR; no Network-LSA; the v6 unicast DD/LSReq path reused |
| PtMP v6 origination | `v6RouterLSABody` emits one Type-1 p2p Router-LSA link per Full neighbour (NeighborRouterID + NeighborInterfaceID, metric = cost); a /128 host route for the interface's own global address with the LA-bit set in the Intra-Area-Prefix-LSA; NO transit link, NO Network-LSA, NO subnet prefix for the PtMP link |
| PtMP next-hop (v6) | SPF resolves the next-hop to each PtMP neighbour from the adjacency link-local (`v6NextHop.P2PNextHop`), reused unchanged; confirm no broadcast-only assumption in `v6RouterLinks`/`BuildGraph` |
| v6 non-broadcast flood fan-out | `floodDestination` (and the initial-flood/Ack send) on a v6 NBMA / non-broadcast-PtMP interface unicasts to each Flood-eligible neighbour link-local instead of `ff02::5`/`ff02::6`; PtMP has no DR-relay suppression |
| `show ipv6 ospf interface` surface | The v6 interface snapshot carries `network_type`; ensure NBMA/PtMP render and that NBMA shows its configured-neighbour/poll state |

### Out of scope (noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| OSPFv3 virtual links (synthetic p2p across a transit area) | spec-ospfv3-ext-3 (explicitly excluded by this task) |
| OSPFv2 NBMA/PtMP | `spec-ospf-ext-8` (the v2 sibling; this spec adds only the v6 enum + v6 origination) |
| RFC 6845 OSPFv3 hybrid broadcast-and-PtMP interface type | future (guide ref #28); not the RFC 5340 PtMP this spec adds |
| RFC 5838 multiple address families on the NBMA/PtMP interface | umbrella out-of-scope; Instance ID stays explicit but no multi-AF behaviour |
| RFC 7166 auth interaction beyond "unchanged" | the auth trailer is per-packet and AF-independent; NBMA/PtMP reuse it as-is, no new auth surface |
| Two-part metric (RFC 8042) on PtMP | out of scope |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
- [ ] `docs/research/ospf-implementation-guide.md` §7 "Network Types and Interface Model" (~470-514, the four-type table + per-type prose) -- the authoritative behavioural contrast
  -> Decision: PtMP is "a collection of point-to-point links, no DR" with host-route origination; NBMA is "explicit static neighbour list, DR elected, unicast per neighbour"; this spec implements that split on the v3 LSA model -- reuse the election for NBMA and the p2p link model for PtMP, but the v3 host route is a /128 LA-bit prefix in the Intra-Area-Prefix-LSA, NOT a Router-LSA stub link (v3 Router-LSAs are address-free)
  -> Constraint: default Hello interval is 30 s on NBMA/PtMP vs 10 s on broadcast/P2P (guide ~499); the per-interface YANG `hello-interval` default is unchanged unless the operator overrides, but the spec documents the 30 s recommendation and the PollInterval default 120 s
- [ ] `docs/research/ospf-implementation-guide.md` §5 ISM/NSM prose (~255-321: Waiting/DROther/Attempt states, the `should_adj` predicate)
  -> Constraint: `should_adj` is "point-to-point, point-to-multipoint, and virtual links: always yes; broadcast or NBMA: only if local or neighbour is DR/BDR" (guide ~321); the shared `nsm.go shouldAdj` default-branch splits into PtMP (true) and NBMA (DR-gated), applied to both v4 and v6 by the v2 sibling -- this spec asserts the split holds for a v6 interface
  -> Constraint: NBMA Attempt state (guide ~298) -- a configured-but-silent NBMA neighbour is polled at PollInterval, not dropped
- [ ] `docs/research/ospf-implementation-guide.md` §8 flooding addressing (~355) -- "Point-to-point, point-to-multipoint, and NBMA use unicast (or multi-unicast)"
  -> Constraint: the v6 initial flood on a non-broadcast interface is a per-neighbour unicast fan-out to the neighbour link-local; the existing v6 retransmit path is already unicast (`neighborAddr`), so only `floodDestination`/the initial-flood send changes
- [ ] `docs/research/ospf-implementation-guide.md` §15 (OSPFv3 separation) -- the do-not-unify recommendation
  -> Constraint: keep the v3-specific origination, codec, and next-hop in the v6 files (`origination_v6*.go`, `afstrategy_v6.go`, `v3/`); do not push v3 prefix/Interface-ID semantics into the shared `iface`/`neighbor` packages, which stay AF-neutral string-keyed
- [ ] `plan/spec-ospfv3-0-umbrella.md` "In Scope" (interface model, DR/BDR, Network-LSA) + AC-3 -- the v6 broadcast contract this spec extends
  -> Constraint: the umbrella shipped v6 broadcast (DR/BDR + Network-LSA) and point-to-point; this spec adds the two new enum values and the per-type v6 origination; it must NOT redefine the v6 interface config model, only extend the enum and add the NBMA-only `nbma-neighbor`/`poll-interval` leaves under the v6 interface
  -> Decision: keep the per-interface `area` binding, costs, timers, priority, passive, Instance ID, and Interface ID exactly as the umbrella defines; NBMA/PtMP are new values of an existing leaf, not a new config subsystem
- [ ] `plan/spec-ospf-ext-8-nbma-p2mp.md` (the v2 sibling) -- the shared ISM/NSM/election/flood mechanism owner
  -> Decision: this spec DEPENDS on the v2 sibling's shared-package changes (the `iface.Start` ISM branches for the two new types, the election gate widening to include NBMA, the `shouldAdj` PtMP/NBMA split, the `floodDestination` non-broadcast unicast fan-out, the SendHello unicast/poll loop + Start Hello, the iface `Config` `PollInterval`/`NBMANeighbors` fields). It re-uses them via the `IsV6` flag and adds only the v6-specific origination/config/enum/next-hop
  -> Constraint: do NOT re-implement the shared mechanism in v6 files; if a shared chokepoint is missing the new branch (because the v2 sibling has not landed it), add the AF-neutral branch there once (keyed on `NetworkType`, never on `IsV6` except the multicast group), shared by both versions
- [ ] `ai/rules/buffer-first.md`, `ai/rules/no-sprintf-alloc.md` -- the v6 Hello/flood encode and the Intra-Area-Prefix host route are buffer-first
  -> Constraint: the PtMP /128 host route + per-neighbour p2p links append into the existing `v6RouterLSABody.Links` / Intra-Area-Prefix `Prefixes` slices and encode via the existing buffer-first `RouterLSA.WriteTo` / `IntraAreaPrefixLSA.WriteTo`; the unicast Hello fan-out reuses the existing per-packet `buildHelloPacket` buffer, sent once per neighbour, with no `fmt`/`+` string building

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5340.md` -- OSPF for IPv6
  -> Constraint: §2.1 -- OSPFv3 runs per link, not per subnet; an interface is identified by a 32-bit Interface ID, and the Router-LSA/Network-LSA describe the graph only (no addresses); so the PtMP host route and the link's prefixes live in the Intra-Area-Prefix-LSA, never in the Router-LSA
  -> Constraint: §A.3.2 -- the v6 Hello has an Interface ID and NO network mask; the v2 §10.5 "network mask must match on broadcast/PtMP/NBMA" Hello check does NOT apply to v6 (the existing `validateHelloLocked` already skips the mask check when `IsV6`)
  -> Constraint: §A.4.3 -- a Router-LSA point-to-point link carries Type 1, the neighbour's Router ID, the neighbour's Interface ID, and this router's Interface ID (address-free); a PtMP interface contributes one such link per Full neighbour, exactly like an existing v6 point-to-point interface
  -> Constraint: §A.4.4 -- the Network-LSA is DR-originated on a broadcast OR NBMA link and lists the attached Router IDs; a PtMP interface (no DR) originates NONE
  -> Constraint: §A.4.10 / §A.4.1 -- IPv6 prefixes attach via the Intra-Area-Prefix-LSA; the LA-bit (`OptPrefixLA`, 0x02) marks a prefix as an actual local interface address; the PtMP host route is the interface's own global address as a /128 with the LA-bit set; padding is `((PrefixLength+31)/32)` 32-bit words (a /128 = 4 words)
  -> Constraint: §2.9 -- v6 multicast is `ff02::5` / `ff02::6`; unicast retransmission and (for NBMA/non-broadcast-PtMP) the initial flood and Hello use the neighbour's link-local source address learned from its Hello; the raw IPv6 socket binds the interface link-local as source
  -> Constraint: §3.8.1 -- the v6 next-hop is the neighbour's link-local from the adjacency, not from any LSA (LSAs are address-free); `afstrategy_v6.go v6NextHop` already does this, so PtMP next-hop needs no new code

**Key insights:**
- NBMA (v6) = the v6 broadcast model (DR election + Network-LSA + Link-LSA + Network-Intra-Area-Prefix-LSA) over a static neighbour list with unicast/poll Hellos. The shared election and the v6 origination are reused; the new behaviour is the configured-neighbour source, the unicast/poll Hello send (shared mechanism), the v6 Network-LSA/Link-LSA gating widened to include NBMA, and the Start Hello.
- PtMP (v6) = v6 point-to-point semantics on a multi-access medium. No DR, no Network-LSA, `should_adj` always true, one address-free p2p Router-LSA link per neighbour, plus a /128 LA-bit host route in the Intra-Area-Prefix-LSA. The v6 point-to-point ISM/NSM/next-hop is reused; the only new origination is the per-neighbour p2p link branch (currently only `NetworkPointToPoint`/`NetworkBroadcast` exist in `v6RouterLSABody`) and the host-route prefix.
- The two new types share one v6 config delta (the v6 enum + the optional `nbma-neighbor`/`poll-interval` leaves) and one flood delta (`floodDestination` unicast fan-out on non-broadcast interfaces, AF-neutral). Everything else is per-type branching at existing v6 chokepoints (`v6RouterLSABody`, `v6ShouldOriginateLinkLSA`, the `v6OriginateSelf` Network-Intra-Area-Prefix gate) plus the shared mechanism the v2 sibling owns.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/ospf/iface/ism.go` -- the `State` enum and the AF-shared network-type string constants `NetworkBroadcast`/`NetworkPointToPoint`/`NetworkLoopback`; there is NO `nbma`/`point-to-multipoint` constant
  -> Constraint: the `NetworkNBMA`/`NetworkPointToMultipoint` constants are added here by the v2 sibling (shared); this spec consumes them on a v6 interface, it does not redefine them
- [ ] `internal/plugins/ospf/iface/iface.go` -- the AF-shared `Config` carries `NetworkType`/`IsV6`/`Priority`; `Start()` switches on `NetworkType` (PtMP must reach `StatePointToPoint`, NBMA the Waiting/DROther branch); `SendHello()` (line ~613) sends to `allSPFRoutersV6` for a v6 interface; `runElectionLocked()` (line ~640) returns early unless `NetworkType == NetworkBroadcast`; `Sender.SendPacket(name, dst, payload)` already takes an arbitrary `dst`
  -> Constraint: the ISM branch, election-gate widening, and unicast/poll Hello loop are AF-neutral (they read `NetworkType` and, for the multicast group, `IsV6`); they are the v2 sibling's mechanism; a v6 NBMA/PtMP interface must flow through them unchanged. The unicast fan-out reuses `SendPacket` with the neighbour link-local as `dst`
- [ ] `internal/plugins/ospf/neighbor/nsm.go` -- `shouldAdj` (line 14): `point-to-point` true; `broadcast`/`""` DR/BDR-gated; `default` false
  -> Constraint: the v2 sibling splits the `default`: `point-to-multipoint` -> true, `nbma` -> DR/BDR-gated; this is AF-neutral (it reads `cfg.NetworkType`), so a v6 PtMP/NBMA neighbour inherits it. A bare `default false` would leave v6 PtMP/NBMA neighbours stuck at 2-Way -- this spec asserts the split is exercised on v6
- [ ] `internal/plugins/ospf/origination_v6.go` -- `v6RouterLSABody` (line 232) switches `NetworkPointToPoint` (per-neighbour p2p links) and `NetworkBroadcast` (transit link) ONLY; `v6OriginateNetwork` (line 204) builds the DR `0x2002` Network-LSA; `v6OriginateSelf` (line 70) sweeps Router / DR-Network / Intra-Area-Prefix self-LSAs and gates the Network-Intra-Area-Prefix-LSA on `NetworkType != NetworkBroadcast` (line ~134)
  -> Constraint: add a `NetworkPointToMultipoint` case to `v6RouterLSABody` (per-neighbour p2p links, identical to the point-to-point case) and a `NetworkNBMA` case (the transit link, identical to broadcast); add the PtMP /128 host route to the self Intra-Area-Prefix-LSA; widen the DR Network / Network-Intra-Area-Prefix gate to include NBMA; PtMP originates NO Network-LSA (no DR)
- [ ] `internal/plugins/ospf/origination_v6_link.go` -- `v6ShouldOriginateLinkLSA` (line 22) returns true only for `NetworkBroadcast || NetworkPointToPoint`; `v6OriginateLinkLSA` builds the `0x0008` Link-LSA (link-local address + prefixes)
  -> Constraint: widen `v6ShouldOriginateLinkLSA` to also return true for `NetworkNBMA` and `NetworkPointToMultipoint` (every v6 interface needs a Link-LSA to advertise its link-local + prefixes); the host-route prefix is added via the Link-LSA prefix list and the self Intra-Area-Prefix-LSA, NOT the Router-LSA
- [ ] `internal/plugins/ospf/afstrategy_v6.go` -- `BuildGraph`/`v6RouterLinks` (line 127) translate Router-LSA p2p and transit links into the shared SPF graph; `v6NextHop.P2PNextHop` (line 398) and `TransitNextHop` (line 405) resolve the next-hop to the neighbour link-local via `neighbors.AddressOf`
  -> Constraint: PtMP emits Type-1 p2p links, which `v6RouterLinks` already translates (the `RouterLinkTypeP2P` case), and the next-hop already resolves via `P2PNextHop` -- so SPF + next-hop need NO change for PtMP; CONFIRM with a test, do not add code. NBMA emits a transit link, already handled by the `RouterLinkTypeTransit` case
- [ ] `internal/plugins/ospf/lsdb/flooding.go` -- `floodDestination` (line 347) returns `allSPFRoutersV6`/`allDRoutersV6` for a v6 interface based on broadcast DROther state; `floodExcept` (line 258) runs the eligible-interface + DR-relay rules; `neighborAddr` (line 516) is already unicast; `InterfaceInfo`/`NeighborInfo` carry `IsV6`, `NetworkType`, `IPv6LinkLocal`, `InterfaceID`
  -> Constraint: the unicast fan-out for a non-broadcast interface (the v2 sibling's `floodDestination`/initial-flood change) is AF-neutral except the per-neighbour destination -- for v6 the destination is the neighbour's link-local (`NeighborInfo`/`AddressOf`). PtMP/NBMA-without-DR-relay floods to all adjacent neighbours; PtMP has no DR so the §13.3 DR-relay suppression is skipped
- [ ] `internal/plugins/ospf/v3/transport/transport.go` -- `SendPacket(name, dst netip.Addr, payload)` (line 474) sends to any IPv6 `dst` with the bound interface link-local as source; it rejects a non-IPv6 `dst`
  -> Constraint: the unicast Hello/flood to a neighbour link-local uses this existing send path unchanged; no new transport API; the neighbour link-local must be a valid IPv6 unicast `dst`
- [ ] `internal/plugins/ospf/config.go` -- `parseInterface` resolves the v6 interface `NetworkType` from the v6 `network-type` enum (today `broadcast`/`point-to-point`); `interfaceConfig` has no v6 neighbour-list or poll-interval field
  -> Constraint: extend the v6 accept-list with `nbma`/`point-to-multipoint`; add the v6 `NBMANeighbors` (Router ID + optional link-local + priority) and `PollInterval` to the v6 interface config and parse them; thread them into `ospfiface.Config` (the iface `Config` NBMA fields are the v2 sibling's; this spec populates them from the v6 config)
- [ ] `internal/plugins/ospf/instance.go` -- `ifaceConfig`/`neighborInterfaceConfig` map `interfaceConfig` to `ospfiface.Config`/`ospfneighbor.InterfaceConfig`, threading `NetworkType` (string) and `IsV6`
  -> Constraint: thread the v6 `PollInterval` + the configured v6 NBMA neighbour list into `ospfiface.Config`; the network-type string already flows end to end for v6 (no new plumbing for the type itself, only the NBMA extras)
- [ ] `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the v6 `address-family/ipv6/.../network-type` enum (lines 304-307) is `broadcast`/`point-to-point`; the v4 enum (lines 178-182) is a separate leaf (the v2 sibling adds the new values there)
  -> Constraint: add `enum nbma;` + `enum point-to-multipoint;` to the v6 enum ONLY (lines 304-307); add a v6 `nbma-neighbor` list (key `router-id`, optional `link-local`, leaf `priority` default 0) + a v6 `poll-interval` leaf (uint16, default 120, units seconds); do NOT touch the v4 enum (owned by the v2 sibling)

**Behavior to preserve:**
- v6 broadcast (Waiting/DROther/Backup/DR + `0x2002` Network-LSA + `ff02::5`/`ff02::6` Hello/flood + DR Network-Intra-Area-Prefix-LSA) and v6 point-to-point (StatePointToPoint + per-neighbour address-free p2p link) behave EXACTLY as today; the new branches are reachable only for the two new v6 enum values.
- The v6 next-hop (`v6NextHop.P2PNextHop`/`TransitNextHop` via `neighbors.AddressOf`), `v6OriginateNetwork`, `v6OriginateLinkLSA`, `v6OriginateIntraAreaPrefix`, the v3 unicast DD path, and the v6 `floodExcept` receive procedure are reused unchanged.
- All existing OSPFv3 unit/functional/interop tests (v6 broadcast + p2p, including FRR `ospf6d`) stay green; the v6 YANG default `network-type broadcast` is unchanged.
- The v4 (OSPFv2) interface enum and origination (`spec-ospf-ext-8`'s territory) are not touched by this spec; the v2 sibling owns them.
- The shared `iface`/`neighbor`/`lsdb` packages stay AF-neutral: no v3-specific prefix/Interface-ID logic leaks into them.

**Behavior to change:** (all RFC-5340-required for the two new v6 types, none discretionary)
- The v6 `parseInterface` accepts `nbma` + `point-to-multipoint`; the v6 YANG enum gains both; the new v6 `nbma-neighbor`/`poll-interval` leaves parse and thread through.
- `v6RouterLSABody` gains a `NetworkPointToMultipoint` case (per-neighbour p2p links) and a `NetworkNBMA` case (transit link).
- The v6 self Intra-Area-Prefix-LSA gains the PtMP /128 LA-bit host route for the interface's own global address.
- `v6ShouldOriginateLinkLSA` returns true for NBMA + PtMP; the v6 DR Network-LSA / Network-Intra-Area-Prefix gate widens to include NBMA (broadcast OR NBMA originates them when DR; PtMP never).
- The shared `floodDestination`/initial flood fan out unicast on a v6 NBMA / non-broadcast PtMP interface to the neighbour link-local (the v2 sibling's mechanism, with the v6 link-local destination).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Config:** an operator sets `address-family/ipv6/.../network-type nbma` (with a `nbma-neighbor` list + optional `poll-interval`) or `point-to-multipoint` -> YANG validate -> `parseInterface` (v6 path) -> `interfaceConfig` -> `instance.ifaceConfig` -> `ospfiface.Config`.
- **ISM:** `iface.Start()` selects the per-type initial state and timers (shared mechanism, AF-neutral).
- **Hello send (clock tick):** `helloLoop` -> `SendHello` -> `ff02::5` (broadcast / PtMP-broadcast-variant) or per-neighbour unicast/poll to the neighbour link-local (NBMA / PtMP-non-broadcast).
- **Hello receive:** an incoming v6 Hello -> v3 transport (link-local source, Instance ID check) -> `receiveHello` -> NSM `hello()` -> election (NBMA) / direct adjacency (PtMP).
- **Origination:** topology change -> `v6OriginateSelf` -> `v6RouterLSABody` per-interface (PtMP p2p links; NBMA transit link + Network-LSA when DR) + the self Intra-Area-Prefix-LSA (PtMP /128 host route).
- **Flood:** an LSA to flood -> `floodExcept` -> per-neighbour unicast fan-out to the neighbour link-local on a non-broadcast v6 interface.

### Transformation Path
1. **Config resolve:** the v6 YANG enum accepts the two new values; `parseInterface` resolves the v6 `NetworkType`, the `NBMANeighbors` slice (Router ID + optional link-local + priority), and `PollInterval`. The network-type string is threaded through `ospfiface.Config`, `ospfneighbor.InterfaceConfig`, and `lsdb.InterfaceInfo` (all already string-typed + `IsV6`).
2. **ISM init (`iface.Start`, shared):** `point-to-multipoint` -> StatePointToPoint, no election, no WaitTimer; `nbma` with priority > 0 -> StateWaiting + WaitTimer + election; `nbma` with priority 0 -> StateDROther; for NBMA the configured neighbours are seeded in the Attempt state (poll-pending) so they are polled before any Hello is heard.
3. **Hello addressing (`SendHello`, shared):** broadcast / PtMP-broadcast-variant -> `ff02::5`; NBMA / PtMP-non-broadcast -> a per-neighbour unicast loop to the neighbour link-local: HelloInterval to neighbours in state >= Init (heard), PollInterval to neighbours still in Attempt (silent). When this NBMA router is DR/BDR, a Start Hello is also sent to priority-0 neighbours.
4. **DR/BDR election (NBMA only, shared):** `runElectionLocked` is entered (gate widened to include NBMA); the candidate set is self + configured neighbours in state >= 2-Way; the election runs unchanged; the elected DR originates the v6 Network-LSA + the DR Network-Intra-Area-Prefix-LSA. PtMP skips this entirely.
5. **Adjacency (`shouldAdj`, shared):** PtMP -> always adjacent (DD exchange with every neighbour); NBMA -> adjacent only with the DR/BDR. The DD/LSReq/LSUpdate exchange uses the existing v3 unicast path to the neighbour link-local.
6. **Router-LSA origination (`v6RouterLSABody`, v6-specific):** PtMP -> one Type-1 address-free p2p link per Full neighbour (NeighborRouterID + NeighborInterfaceID + this router's Interface ID, metric = cost); no transit link. NBMA -> a Type-2 transit link when DR (the broadcast path). The PtMP /128 host route is NOT here -- it is a prefix in the self Intra-Area-Prefix-LSA.
7. **Prefix origination (`v6OriginateIntraAreaPrefix`, v6-specific):** PtMP -> add the interface's own global address as a /128 with the LA-bit (`OptPrefixLA`) set to the self (Router-referencing) Intra-Area-Prefix-LSA; the link's subnet prefix is NOT advertised for a PtMP interface. The Link-LSA carries the link-local + prefixes for all four types.
8. **SPF next-hop (§3.8.1, v6-specific, reused):** the PtMP destination's next-hop is the neighbour's link-local from the adjacency (`v6NextHop.P2PNextHop`), exactly as point-to-point; the /128 host route inherits that next-hop in `BuildRoutes`. NBMA resolves next-hops via the transit-network vertex like broadcast.
9. **Flood fan-out (`floodDestination` / initial flood, shared):** on a v6 NBMA / non-broadcast PtMP interface, the initial flood + Ack unicast to each Flood-eligible neighbour's link-local; on broadcast / PtMP-broadcast-variant, the existing `ff02::5`/`ff02::6` group is used. PtMP has no DR, so the DR-relay suppression is skipped.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config (v6 YANG) <-> engine | v6 `network-type` enum + `nbma-neighbor`/`poll-interval` leaves -> `parseInterface` (v6) -> `interfaceConfig` -> `ospfiface.Config` | [ ] |
| Config <-> NSM | `NetworkType` string + `IsV6` threaded into `ospfneighbor.InterfaceConfig` (already string-typed; new values only) | [ ] |
| ISM <-> Hello send | `iface.Start` per-type init (shared); `SendHello` `ff02::5` vs per-neighbour unicast/poll to link-local | [ ] |
| NSM <-> adjacency | `shouldAdj` PtMP-always / NBMA-DR-gated (shared); existing v3 unicast DD path reused | [ ] |
| Topology <-> Router-LSA | `v6RouterLSABody` PtMP p2p links / NBMA transit link; address-free | [ ] |
| Topology <-> Intra-Area-Prefix-LSA | PtMP /128 LA-bit host route in the self Intra-Area-Prefix-LSA; no subnet prefix for PtMP | [ ] |
| LSDB <-> flooding | `floodDestination` unicast fan-out to neighbour link-local on non-broadcast v6 interfaces; PtMP no DR-relay | [ ] |
| SPF <-> next-hop | PtMP next-hop from the neighbour link-local (`v6NextHop.P2PNextHop`, reused) | [ ] |
| Transport <-> unicast | v3 `SendPacket(name, neighbour-link-local, payload)`; reused, no new API | [ ] |

### Integration Points
- `internal/plugins/ospf/origination_v6.go` -- `v6RouterLSABody` PtMP/NBMA branches; the PtMP host route in the self Intra-Area-Prefix-LSA; the DR Network-LSA / Network-Intra-Area-Prefix gate widened to NBMA (the single v6-origination delta site).
- `internal/plugins/ospf/origination_v6_link.go` -- `v6ShouldOriginateLinkLSA` widened to NBMA + PtMP.
- `internal/plugins/ospf/afstrategy_v6.go` -- READ-ONLY confirmation that the PtMP p2p link + link-local next-hop already work (`v6RouterLinks`, `v6NextHop`); no change expected.
- `internal/plugins/ospf/iface`, `.../neighbor`, `.../lsdb` -- the shared ISM/NSM/election/flood mechanism (owned by the v2 sibling); this spec exercises it on a v6 interface and ensures the unicast destination is the neighbour link-local.
- `internal/plugins/ospf/config.go` + `yang/ze-ospf-conf.yang` -- the v6 enum + the v6 NBMA-only leaves.
- `internal/plugins/ospf/instance.go` -- thread the v6 `PollInterval` + the NBMA neighbour list into `ospfiface.Config`.
- `internal/plugins/ospf/v3/transport/transport.go` -- READ-ONLY: the unicast `SendPacket` path reused for Hello/flood to a neighbour link-local.

### Architectural Verification
- [ ] No bypassed layers (config -> resolve -> iface/neighbor/lsdb runtime + v6 origination, the same spine as v6 broadcast/p2p; no new packet type, no new dispatcher)
- [ ] No unintended coupling (the two new types are additional v6-enum values + additional branches at existing v6 chokepoints; the shared packages stay AF-neutral; no v3 prefix logic in `iface`/`neighbor`)
- [ ] No duplicated functionality (reuses the shared election/ISM/NSM/flood mechanism, `v6OriginateNetwork`, `v6OriginateLinkLSA`, `v6NextHop`, the v3 unicast transport; adds only the `v6RouterLSABody` branches, the PtMP host route, the NBMA Link/Network gating, and the v6 config/enum)
- [ ] Zero-copy / buffer-first preserved (the host route + p2p links append into existing slices encoded buffer-first; the unicast Hello fan-out reuses one built buffer per neighbour)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The shared `iface`/`neighbor`/`lsdb` mechanism for the two new network types (ISM branches, election gate, `shouldAdj` split, unicast/poll Hello, `floodDestination` non-broadcast fan-out, iface `Config` NBMA fields) is AF-neutral and is delivered by `spec-ospf-ext-8`; a v6 interface inherits it via the `IsV6`+`NetworkType` threads | `iface.go`/`nsm.go`/`flooding.go` branch on `NetworkType` (and `IsV6` only for the group); `spec-ospf-ext-8` "Files to Modify" lists these shared edits | this spec must add the shared mechanism itself (AF-neutral) before the v6 work | package builds; `TestOSPFv3PtMPISMNoElection`, `TestOSPFv3NBMAISMWaiting` pass over a v6 `Config` | unvalidated |
| A-2 | `v6RouterLSABody`'s `NetworkPointToPoint` case (address-free p2p link per Full neighbour) is correct for PtMP verbatim; only a `NetworkPointToMultipoint` case mirroring it is needed | `origination_v6.go` line 255-268 p2p case | PtMP needs a distinct link encoding | `TestOSPFv3PtMPRouterLSALinks` (one p2p link per Full neighbour, address-free) | unvalidated |
| A-3 | The PtMP host route is a /128 LA-bit prefix in the Intra-Area-Prefix-LSA (NOT a Router-LSA stub link), because v3 Router-LSAs are address-free | RFC 5340 §A.4.3 (address-free Router-LSA), §A.4.10/§A.4.1 (prefixes + LA-bit in Intra-Area-Prefix-LSA); `v3/types/prefix.go` `OptPrefixLA` | the host route is mis-encoded; remote routers cannot reach the PtMP interface | `TestOSPFv3PtMPHostRoute` (a /128 with LA-bit present in the Intra-Area-Prefix-LSA; no subnet prefix) | unvalidated |
| A-4 | The v6 next-hop to a PtMP neighbour is the neighbour link-local via `v6NextHop.P2PNextHop`, reused unchanged; `v6RouterLinks` already translates the p2p link into the SPF graph | `afstrategy_v6.go` lines 127-150 (p2p case), 398-403 (`P2PNextHop`) | PtMP needs a distinct next-hop path | `TestOSPFv3PtMPNextHop` (next-hop = neighbour link-local); no new next-hop code | unvalidated |
| A-5 | A v6 NBMA segment elects a DR (shared election) and the DR originates the `0x2002` Network-LSA + the DR Network-Intra-Area-Prefix-LSA exactly like v6 broadcast; only the Hello addressing differs | `v6OriginateNetwork` (line 204), the broadcast case in `v6RouterLSABody`/`v6OriginateSelf`; RFC 5340 §A.4.4 | NBMA needs a non-DR origination model | `TestOSPFv3NBMANetworkLSA` (NBMA DR originates `0x2002`) | unvalidated |
| A-6 | Every v6 interface (including NBMA + PtMP) must originate a `0x0008` Link-LSA; widening `v6ShouldOriginateLinkLSA` is sufficient | `origination_v6_link.go` line 22-27; RFC 5340 §4.4.3.8 | PtMP/NBMA neighbours never learn the link-local/prefixes; adjacency or routing breaks | `TestOSPFv3NBMALinkLSA`, `TestOSPFv3PtMPLinkLSA` (Link-LSA originated for both) | unvalidated |
| A-7 | The configured v6 NBMA neighbour list (Router ID + optional link-local + priority) is sufficient to seed Attempt and drive unicast/poll Hellos to the neighbour link-local; no neighbour is discovered by multicast on v6 NBMA | RFC 5340 reuses RFC 2328 §10.1/§9.5; the configured link-local (or the one learned from the neighbour's first unicast Hello) is the `dst` | NBMA cannot reach neighbours; adjacency never forms | `TestOSPFv3NBMAPollAttempt`, functional `ospfv3-nbma.ci` | unvalidated |
| A-8 | The v3 transport `SendPacket(name, dst, payload)` (arbitrary IPv6 `dst`) is sufficient for the unicast Hello/flood; no new transport API is needed | `v3/transport/transport.go` line 474 | new v3 unicast plumbing is needed | `TestOSPFv3NBMAFloodUnicast`, `TestOSPFv3PtMPFloodUnicast` | unvalidated |
| A-9 | Adding `nbma`+`point-to-multipoint` to the v6 YANG enum + the v6 `parseInterface` accept-list, plus the v6 `nbma-neighbor`/`poll-interval` leaves, is the entire v6 config surface; the network-type string already threads end to end | `yang/ze-ospf-conf.yang` lines 304-307; `config.go` `parseInterface`; `instance.go` `ifaceConfig` | more v6 plumbing or a new typed field is needed | package builds; `TestOSPFv3ParseNBMAInterface`, `TestOSPFv3ParsePtMPInterface` | unvalidated |
| A-10 | The v6 Hello has no network mask (Interface ID instead), so the new types add no Hello-validation change for v6; `validateHelloLocked` already skips the mask check when `IsV6` | `iface.go` line 683 (`!i.cfg.IsV6 && ... NetworkBroadcast` mask check); RFC 5340 §A.3.2 | adjacency fails for a new reason on v6 | existing v6 Hello-validate tests stay green; `TestOSPFv3NBMAAdjacency` forms Full | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | PtMP host route mis-encoded (placed as a Router-LSA stub link like OSPFv2, or without the LA-bit, or the subnet prefix advertised instead) -> remote routers cannot reach the PtMP interface, or a phantom subnet route appears | a PtMP neighbour's /128 is unreachable, or a link-subnet route shows where only a /128 should | encode the /128 with `OptPrefixLA` in the Intra-Area-Prefix-LSA (RFC 5340 §A.4.10/§A.4.1); `TestOSPFv3PtMPHostRoute` asserts the /128+LA present AND no subnet prefix |
| R-2 | PtMP originates a Network-LSA (or runs an election) -> a spurious transit vertex corrupts v6 SPF | a `0x2002` LSA appears for a PtMP segment; `BuildGraph` builds a Network vertex | gate `v6OriginateNetwork`/election on a real DR (PtMP has none); `TestOSPFv3PtMPNoNetworkLSA`, `TestOSPFv3PtMPNoElection` |
| R-3 | `shouldAdj` not split for the v6 path (or the v2 sibling's split not exercised on v6) -> v6 PtMP/NBMA neighbours stuck at 2-Way, never reach Full | a v6 PtMP/NBMA neighbour never leaves 2-Way; no p2p Router-LSA link | the split is AF-neutral (`cfg.NetworkType`); `TestOSPFv3ShouldAdjPtMP`, `TestOSPFv3ShouldAdjNBMA`, `TestOSPFv3PtMPAdjacency` |
| R-4 | NBMA never polls a silent configured neighbour (no Attempt / no PollInterval send), or the unicast `dst` is wrong (an IPv4 group / a global address, not the link-local) -> adjacency never starts | a configured v6 NBMA neighbour stays Down; the v3 socket rejects the `dst`; `SendPacket` errors | seed configured neighbours in Attempt, poll at PollInterval to the link-local `dst`; Start Hello to priority-0; `TestOSPFv3NBMAPollAttempt`, `TestOSPFv3NBMAUnicastHello` |
| R-5 | Non-broadcast flood still uses `ff02::5`/`ff02::6` (`floodDestination` not branched for non-broadcast) -> LSAs never reach NBMA/PtMP-non-broadcast neighbours | a flooded LSA is acked on a v6 broadcast test but silently lost on v6 NBMA | unicast fan-out in `floodDestination`/the initial flood to the neighbour link-local for non-broadcast; `TestOSPFv3NBMAFloodUnicast`, `TestOSPFv3PtMPFloodUnicast`; FRR `ospf6d` interop confirms LSAs cross |
| R-6 | PtMP next-hop wrong (treated as a transit/Network vertex instead of a direct p2p link) -> traffic to a PtMP neighbour mis-steered or the /128 missing | a PtMP route installs with the wrong next-hop / a directly-connected /128 missing | reuse `v6NextHop.P2PNextHop`; the p2p link is translated by `v6RouterLinks`; `TestOSPFv3PtMPNextHop` asserts the neighbour link-local |
| R-7 | The new v6 enum value accidentally touches the v4 leaf (shared YANG file) or breaks the v6 config round-trip | a v4 interface accepts `nbma` via the v6 leaf, or a config diff churns | add the enum ONLY to the v6 leaf (lines 304-307); the v4 leaf is owned by `spec-ospf-ext-8`; `config_test.go` v6 round-trip + a v4-vs-v6 isolation test |
| R-8 | `v6ShouldOriginateLinkLSA` not widened -> NBMA/PtMP interfaces originate no Link-LSA -> neighbours never learn the link-local, SPF cannot resolve the next-hop | a v6 NBMA/PtMP neighbour reaches Full but no route installs (no link-local in the adjacency / no prefixes) | widen the predicate to NBMA + PtMP; `TestOSPFv3NBMALinkLSA`, `TestOSPFv3PtMPLinkLSA` |
| R-9 | The election gate widened for NBMA accidentally also lets PtMP elect (or NBMA never elects) on the v6 path | a v6 PtMP segment elects a DR, or a v6 NBMA DR is never chosen | the gate is exactly `NetworkBroadcast || NetworkNBMA` (shared); `TestOSPFv3NBMAElection` (elects) + `TestOSPFv3PtMPNoElection` (does not) |
| R-10 | The v6 PtMP host route and the v6 broadcast prefix origination collide (the same `v6OriginateSelf` sweep emits both) -> a duplicate or wrong Intra-Area-Prefix-LSA | a PtMP interface emits a subnet prefix as well as the /128, or the self IAP-LSA is wrong | PtMP adds ONLY the /128 LA-bit host route to the self IAP-LSA and emits no subnet prefix for that interface; `TestOSPFv3PtMPHostRoute` asserts exactly the /128 |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| v6 config `network-type point-to-multipoint` on an interface | -> | `parseInterface` (v6) accepts it -> `ospfiface.Config.NetworkType` = PtMP, `IsV6` = true -> `iface.Start` takes the point-to-point ISM branch (no election) | `TestOSPFv3ParsePtMPInterface` (unit) + `test/ospfv3/ospfv3-ptmp.ci` |
| v6 config `network-type nbma` + a `nbma-neighbor` list | -> | `parseInterface` (v6) resolves the neighbour list + `poll-interval` -> seeded in Attempt -> unicast/poll Hello to the neighbour link-local | `TestOSPFv3ParseNBMAInterface` (unit) + `test/ospfv3/ospfv3-nbma.ci` |
| a v6 PtMP interface reaches Full with a neighbour | -> | `v6RouterLSABody` emits a Type-1 address-free p2p link; the self Intra-Area-Prefix-LSA emits a /128 LA-bit host route; no Network-LSA | `TestOSPFv3PtMPRouterLSALinks`, `TestOSPFv3PtMPHostRoute`, `TestOSPFv3PtMPNoNetworkLSA` |
| a v6 NBMA interface with eligible neighbours comes up | -> | the shared election gate admits NBMA -> a DR is elected -> `v6OriginateNetwork` originates the `0x2002` Network-LSA; `v6ShouldOriginateLinkLSA` originates the Link-LSA | `TestOSPFv3NBMAElection`, `TestOSPFv3NBMANetworkLSA`, `TestOSPFv3NBMALinkLSA` |
| an LSA is flooded on a v6 NBMA / non-broadcast interface | -> | `floodDestination`/the initial flood unicasts to each Flood-eligible neighbour's link-local via v3 `SendPacket` | `TestOSPFv3NBMAFloodUnicast`, `test/ospfv3/ospfv3-nbma.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A v6 interface configured `network-type point-to-multipoint` | accepted by the v6 YANG enum + `parseInterface`; the iface enters the point-to-point ISM state (no Waiting, no WaitTimer); no DR/BDR elected; `show ipv6 ospf interface` reports network type `point-to-multipoint` |
| AC-2 | A v6 PtMP interface forms an adjacency with a neighbour | `should_adj` is true -> the adjacency proceeds to Full (no DR gating); the existing v3 unicast DD/LSReq/LSUpdate exchange completes |
| AC-3 | A v6 PtMP interface at Full with neighbour N | the Router-LSA contains a Type-1 (point-to-point) address-free link: NeighborRouterID = N's Router ID, NeighborInterfaceID = N's Interface ID, this router's Interface ID set, metric = interface cost; NO transit link for that interface |
| AC-4 | A v6 PtMP interface with a global address X/p | a /128 host route for X with the LA-bit (`OptPrefixLA`) set is advertised in this router's Intra-Area-Prefix-LSA; NO subnet prefix (X/p) is advertised for that interface; NO `0x2002` Network-LSA is originated |
| AC-5 | v6 SPF computes the route to a PtMP neighbour's address (or the neighbour's advertised /128) | the next-hop is the neighbour's IPv6 link-local from the adjacency (`v6NextHop.P2PNextHop`, §3.8.1), not a transit-vertex next-hop |
| AC-6 | A v6 interface configured `network-type nbma` with a `nbma-neighbor` list (Router IDs + optional link-locals + priorities) and `poll-interval` | accepted by the v6 YANG + `parseInterface`; the configured neighbours are seeded in the Attempt state; an eligible (priority > 0) NBMA interface enters Waiting and arms the WaitTimer |
| AC-7 | A v6 NBMA interface sending Hellos | Hellos are sent unicast to each configured neighbour's link-local: HelloInterval to neighbours heard from (state >= Init), PollInterval (default 120 s) to silent neighbours (Attempt); no `ff02::5` multicast Hello is sent on the NBMA interface |
| AC-8 | A v6 NBMA segment with two or more eligible routers | a DR (and BDR) is elected using the shared election; the elected DR originates the `0x2002` Network-LSA + the DR Network-referencing Intra-Area-Prefix-LSA exactly as on a v6 broadcast segment |
| AC-9 | This v6 NBMA router becomes DR or BDR, and a configured neighbour has priority 0 | a Start (Hello) is sent to that priority-0 neighbour's link-local so the adjacency begins; the priority-0 neighbour is ineligible for the election but still adjacent to the DR/BDR |
| AC-10 | An LSA flooded on a v6 NBMA or non-broadcast PtMP interface | it is sent unicast to each Flood-eligible neighbour's link-local (Exchange/Loading/Full), not to `ff02::5`/`ff02::6`; the neighbour acknowledges and the LSA installs |
| AC-11 | A v6 NBMA adjacency reaches Full | `should_adj` admits only the DR/BDR; a DROther-to-DROther pair on v6 NBMA stays at 2-Way (no adjacency), exactly as v6 broadcast |
| AC-12 | Every v6 NBMA and PtMP interface | originates its `0x0008` Link-LSA (link-local address + prefixes), so neighbours learn the link-local and SPF resolves the next-hop |
| AC-13 | A v6 broadcast or point-to-point interface (regression) | behaves exactly as before this spec: v6 broadcast still multicasts Hellos/floods, elects, and originates a Network-LSA + DR Network-Intra-Area-Prefix-LSA; v6 point-to-point still emits per-neighbour address-free p2p links |
| AC-14 | The v4 (OSPFv2) interface config and the v4 `network-type` enum | are untouched by this spec; the v6 enum changes do not leak into the v4 leaf, and v4 NBMA/PtMP remain owned by `spec-ospf-ext-8` |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures an IPv6 hub-and-spoke link `network-type point-to-multipoint` and expects each spoke reachable as a /128 host route | v6 config -> PtMP ISM -> Full adjacency per neighbour -> `v6RouterLSABody` p2p links + the /128 LA-bit host route in the Intra-Area-Prefix-LSA -> v6 SPF /128 next-hop (link-local) -> IPv6 Loc-RIB | `test/ospfv3/ospfv3-ptmp.ci` |
| 2 | Configures an IPv6 NBMA segment with a static neighbour list and expects a DR elected and adjacencies formed without all-routers multicast | v6 config -> NBMA Attempt/poll -> unicast Hello to link-local -> election -> DR `0x2002` Network-LSA + Link-LSA -> Full with DR/BDR | `test/ospfv3/ospfv3-nbma.ci` |
| 3 | Adds a priority-0 v6 NBMA neighbour and expects it adjacent to the DR | v6 config -> election makes this router DR/BDR -> Start Hello to the priority-0 neighbour link-local -> adjacency forms | `test/ospfv3/ospfv3-nbma.ci` (priority-0 step) |
| 4 | Runs `show ipv6 ospf interface` on the NBMA/PtMP interface | CLI -> v6 interface snapshot -> network type + (NBMA) poll/neighbour state rendered | `test/ospfv3/ospfv3-nbma.ci` / `ospfv3-ptmp.ci` (show step) |
| 5 | Peers a PtMP/NBMA Ze v6 interface with FRR `ospf6d` of the matching type | v6 wire (unicast/multicast Hello + unicast flood, link-local source) -> Full adjacency -> LSDB sync -> IPv6 routes both ways | `test/interop/scenarios/ospfv3-ptmp-frr/`, `ospfv3-nbma-frr/` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFv3ParsePtMPInterface` | `internal/plugins/ospf/config_test.go` | AC-1, A-9: v6 `point-to-multipoint` accepted; `interfaceConfig.NetworkType` set, `IsV6` true | |
| `TestOSPFv3ParseNBMAInterface` | `internal/plugins/ospf/config_test.go` | AC-6, A-9: v6 `nbma` + `nbma-neighbor` list (Router ID + link-local + priority) + `poll-interval` parsed | |
| `TestOSPFv3NetworkTypeV4V6Isolation` | `internal/plugins/ospf/config_test.go` | AC-14, R-7: the v6 enum gains the two values; the v4 leaf is unchanged; a value set on one leaf does not bleed into the other | |
| `TestOSPFv3PtMPISMNoElection` | `internal/plugins/ospf/iface/iface_test.go` | AC-1, R-2: a v6 PtMP `Config` enters StatePointToPoint, no Waiting, no election | |
| `TestOSPFv3NBMAISMWaiting` | `internal/plugins/ospf/iface/iface_test.go` | AC-6: an eligible v6 NBMA `Config` enters Waiting + WaitTimer; priority 0 -> DROther | |
| `TestOSPFv3NBMAElection` | `internal/plugins/ospf/iface/iface_test.go` | AC-8, A-5, R-9: a v6 NBMA `Config` runs the election; elects the same DR a broadcast set would | |
| `TestOSPFv3PtMPNoElection` | `internal/plugins/ospf/iface/iface_test.go` | AC-1/AC-4, R-2/R-9: a v6 PtMP `Config` never elects, never sets DR/BDR | |
| `TestOSPFv3NBMAUnicastHello` | `internal/plugins/ospf/iface/iface_test.go` | AC-7, R-4: a v6 NBMA Hello is sent unicast per configured neighbour link-local; no `ff02::5` | |
| `TestOSPFv3NBMAPollAttempt` | `internal/plugins/ospf/iface/iface_test.go` | AC-7, R-4, A-7: a silent neighbour is polled at PollInterval (Attempt); heard at HelloInterval | |
| `TestOSPFv3NBMAStartHelloPriorityZero` | `internal/plugins/ospf/iface/iface_test.go` | AC-9: DR/BDR sends a Start Hello to a priority-0 neighbour link-local | |
| `TestOSPFv3ShouldAdjPtMP` / `TestOSPFv3ShouldAdjNBMA` | `internal/plugins/ospf/neighbor/nsm_test.go` | AC-2/AC-11, R-3: v6 PtMP always adjacent; v6 NBMA DR/BDR-gated | |
| `TestOSPFv3PtMPAdjacency` | `internal/plugins/ospf/neighbor/nsm_test.go` | AC-2: a v6 PtMP neighbour reaches Full via the v3 unicast DD path | |
| `TestOSPFv3NBMAAdjacency` | `internal/plugins/ospf/neighbor/nsm_test.go` | AC-11, A-10: v6 NBMA reaches Full only with DR/BDR; a DROther pair stays 2-Way | |
| `TestOSPFv3PtMPRouterLSALinks` | `internal/plugins/ospf/origination_v6_test.go` | AC-3, A-2: `v6RouterLSABody` emits one address-free Type-1 p2p link per Full neighbour for PtMP; no transit link | |
| `TestOSPFv3PtMPHostRoute` | `internal/plugins/ospf/origination_v6_link_test.go` (or `origination_v6_test.go`) | AC-4, A-3, R-1/R-10: the self Intra-Area-Prefix-LSA carries the interface's /128 with the LA-bit; NO subnet prefix for the PtMP interface | |
| `TestOSPFv3PtMPNoNetworkLSA` | `internal/plugins/ospf/origination_v6_test.go` | AC-4, R-2: a v6 PtMP interface originates no `0x2002` Network-LSA | |
| `TestOSPFv3NBMANetworkLSA` | `internal/plugins/ospf/origination_v6_test.go` | AC-8, A-5: a v6 NBMA DR originates a `0x2002` Network-LSA + the DR Network-Intra-Area-Prefix-LSA | |
| `TestOSPFv3NBMALinkLSA` / `TestOSPFv3PtMPLinkLSA` | `internal/plugins/ospf/origination_v6_link_test.go` | AC-12, A-6, R-8: every v6 NBMA + PtMP interface originates its `0x0008` Link-LSA | |
| `TestOSPFv3NBMAFloodUnicast` | `internal/plugins/ospf/lsdb/flooding_test.go` | AC-10, A-8, R-5: a v6 NBMA initial flood + Ack unicast to each Flood-eligible neighbour link-local | |
| `TestOSPFv3PtMPFloodUnicast` | `internal/plugins/ospf/lsdb/flooding_test.go` | AC-10, R-5: a v6 non-broadcast PtMP floods unicast; no DR-relay suppression | |
| `TestOSPFv3PtMPNextHop` | `internal/plugins/ospf/afstrategy_v6_test.go` | AC-5, A-4, R-6: v6 PtMP next-hop = the neighbour's link-local (`v6NextHop.P2PNextHop`); the p2p link translates into the SPF graph | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| v6 `poll-interval` (uint16 seconds) | 1..65535 | 65535 | 0 (rejected; must be > 0) | N/A (16-bit field) |
| v6 `nbma-neighbor` priority (uint8) | 0..255 | 255 | N/A | N/A (1 byte); 0 = ineligible (polled, not elected) |
| PtMP host-route prefix length | 128 only | 128 | N/A (a shorter prefix is a subnet, not a host) | N/A (>128 rejected by prefix decode) |
| PtMP host-route LA-bit | `OptPrefixLA` set | set | N/A | N/A (single bit) |
| v6 prefix words (host route) | `((PrefixLength+31)/32)` = 4 for /128 | 4 words | too short | non-zero padding rejected |
| v6 network-type enum | {broadcast, point-to-point, nbma, point-to-multipoint} | n/a | unknown string rejected by `parseInterface` | n/a (v6 has no loopback enum) |
| v6 NBMA configured-neighbour count | 0..maxNeighbors (1024) | 1024 | N/A | beyond 1024 hits the existing `maxNeighbors` guard |
| NBMA neighbour link-local `dst` | a valid IPv6 link-local unicast | `fe80::/10` unicast | a non-IPv6 or non-link-local addr rejected by v3 `SendPacket` | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospfv3-nbma` | `test/ospfv3/ospfv3-nbma.ci` | a v6 NBMA interface with a static neighbour list elects a DR, polls a silent neighbour, forms Full, originates a Network-LSA + Link-LSA, and floods unicast | |
| `ospfv3-ptmp` | `test/ospfv3/ospfv3-ptmp.ci` | a v6 PtMP interface forms Full with each neighbour, emits address-free p2p links + /128 LA-bit host routes, no Network-LSA, no DR | |
| `ospfv3-nbma-config` | `test/ospfv3/ospfv3-nbma-config.ci` | config round-trip of v6 `network-type nbma` + `nbma-neighbor` + `poll-interval`; invalid values rejected; the v4 leaf untouched; `show ipv6 ospf interface` renders | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospfv3-ptmp-frr` | `test/interop/scenarios/ospfv3-ptmp-frr/` | FRR `ospf6d` (`ipv6 ospf6 network point-to-multipoint`) | Ze and FRR form v6 PtMP adjacencies (no DR), exchange address-free p2p Router-LSA links + /128 host-route prefixes, and install each other's /128s; next-hops resolve to the neighbour link-local | |
| `ospfv3-nbma-frr` | `test/interop/scenarios/ospfv3-nbma-frr/` | FRR `ospf6d` (non-broadcast network + `neighbor` statements) | Ze and FRR elect a consistent DR over a static neighbour list, exchange unicast Hellos/floods (link-local source), the DR originates the `0x2002` Network-LSA, and IPv6 routes converge both ways | |

> Interop is required: this changes v6 wire behaviour (unicast Hello addressing,
> the NBMA election + Network-LSA over a static list, PtMP address-free p2p links +
> /128 host-route prefixes, unicast flooding). The raw IPv6 / unicast paths are
> Linux-only and run as QEMU integration tests (`ai/rules/qemu-testing.md`),
> consistent with the rest of the OSPFv3 interop set. NOTE: confirm the FRR
> `ospf6d` version supports a PtMP/non-broadcast network type for v6; if a given
> FRR build does not, document it and gate the scenario, but the wire behaviour
> must still be unit-tested.

### Future (if deferring any tests)
- None. Every AC is covered by a unit, functional, or interop test above. RFC 6845 hybrid broadcast-and-PtMP and OSPFv3 virtual links are explicitly out of scope (no test owed here).

## Files to Modify
<!-- MUST include feature code, not only test files -->
- `internal/plugins/ospf/origination_v6.go` -- `v6RouterLSABody`: add a `NetworkPointToMultipoint` case (per-neighbour address-free p2p links, mirroring the point-to-point case) and a `NetworkNBMA` case (the transit link, mirroring broadcast); the self Intra-Area-Prefix-LSA path (`v6OriginateIntraAreaPrefix`/`v6OriginateSelf`): add the PtMP /128 LA-bit host route and emit no subnet prefix for a PtMP interface; widen the DR Network-LSA / Network-Intra-Area-Prefix gate (line ~134) to include NBMA; PtMP never originates a Network-LSA
- `internal/plugins/ospf/origination_v6_link.go` -- `v6ShouldOriginateLinkLSA`: return true for `NetworkNBMA` and `NetworkPointToMultipoint` as well; (host-route prefixes flow through the existing Link-LSA prefix list + the self Intra-Area-Prefix-LSA)
- `internal/plugins/ospf/config.go` -- extend the **v6** `parseInterface` accept-list with `nbma`/`point-to-multipoint`; add v6 `NBMANeighbors []nbmaNeighborV6{RouterID; LinkLocal; Priority}` + `PollInterval uint16` to the v6 interface config and parse them (the iface `Config` NBMA fields themselves are added by `spec-ospf-ext-8`; this spec populates them for v6)
- `internal/plugins/ospf/instance.go` -- thread the v6 `PollInterval` + the NBMA neighbour list into `ospfiface.Config`; the network-type string + `IsV6` already thread through
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- add `enum nbma;` + `enum point-to-multipoint;` to the **v6** `address-family/ipv6/.../network-type` enum (lines 304-307); add a v6 `nbma-neighbor` list (key `router-id`, optional `link-local`, leaf `priority` default 0) + a v6 `poll-interval` leaf (uint16, default 120, units seconds); do NOT touch the v4 enum
- `internal/plugins/ospf/afstrategy_v6.go` -- VERIFY ONLY (and adjust only if a broadcast-only assumption surfaces) that `v6RouterLinks` translates the PtMP p2p link and `v6NextHop.P2PNextHop` resolves the next-hop; no change expected
- `internal/plugins/ospf/iface/iface.go`, `.../iface/ism.go`, `.../neighbor/nsm.go`, `.../lsdb/flooding.go` -- the shared ISM/NSM/election/flood mechanism is owned by `spec-ospf-ext-8`; this spec MODIFIES them ONLY if that sibling has not yet landed the AF-neutral branch the v6 path needs (then the branch is added once, keyed on `NetworkType`, with the v6 unicast destination = the neighbour link-local)
- `internal/plugins/ospf/cmd_show.go` / `show_summary.go` -- render v6 NBMA poll/neighbour state in `show ipv6 ospf interface` if the existing snapshot does not already surface it

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `yang/ze-ospf-conf.yang` -- two v6 enum values + v6 `nbma-neighbor` list + v6 `poll-interval` leaf; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | v6 `poll-interval` `range "1..65535"`; v6 `nbma-neighbor` priority `range "0..255"`; `router-id` `ze:validate` a router-id; `link-local` `ze:validate` an IPv6 link-local (reuse the existing IPv6 validator if present) |
| YANG custom validators | [ ] check | if no IPv6-link-local validator exists, add a `ValidateFn`/`CompleteFn` for `nbma-neighbor/link-local`; otherwise reuse |
| CLI commands/flags | [ ] no | reuses `show ipv6 ospf interface`; no new command (NBMA/PtMP are config, not a new verb) |
| CLI grammar (action before identifier) | [ ] n/a | no new command |
| Editor autocomplete | [ ] yes | automatic for the v6 YANG enum + `poll-interval`; `CompleteFn` for `nbma-neighbor/link-local` if added |
| Functional test for new RPC/API | [ ] yes | `test/ospfv3/ospfv3-nbma*.ci`, `ospfv3-ptmp.ci` |
| Pipe completeness | [ ] yes | `show ipv6 ospf interface` already routes through `ApplyPipes`; no new output path |
| Env var registration | [ ] no | operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] no | no new socket/port/binary; reuses the existing OSPFv3 raw IPv6 socket |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospfv3_nbma_neighbors` | gauge | `interface`, `state` (attempt/heard) |
| `ze_ospfv3_nbma_polls_total` | counter | `interface` |
| `ze_ospfv3_ptmp_host_routes` | gauge | `interface` |

> These extend the umbrella's canonical OSPFv3 metric set, use the `ze_ospfv3_*`
> prefix, and are registered by this spec's owner code. The umbrella "Metrics"
> row set gains these when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPFv3 NBMA + point-to-multipoint network types |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` -- v6 `network-type nbma|point-to-multipoint`, v6 `nbma-neighbor`, v6 `poll-interval` |
| 3 | CLI command added/changed? | [ ] check | `docs/guide/command-reference.md` -- `show ipv6 ospf interface` NBMA/PtMP fields if rendered |
| 4 | API/RPC added/changed? | [ ] no | reuses the existing v6 interface-show RPC |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPFv3 gains NBMA/PtMP |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospfv3.md` (or the OSPFv3 section of `docs/guide/ospf.md`) -- v6 network-types section (NBMA + PtMP) |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospfv3.md` -- unicast Hello addressing, PtMP address-free p2p links + /128 host-route prefixes, unicast flooding, NBMA Network-LSA over a static list |
| 8 | Plugin SDK/protocol changed? | [ ] no | no SDK surface change |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc5340.md` -- tick the NBMA/PtMP-relevant compliance items |
| 10 | Test infrastructure changed? | [ ] yes (interop scenarios added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPFv3 network-type parity with FRR `ospf6d` |
| 12 | Internal architecture changed? | [ ] yes | the OSPFv3 subsystem doc -- v6 network-type branching at the origination/flood chokepoints + the shared-mechanism dependency on the v2 sibling |
| 13 | Route metadata keys added/changed? | [ ] no | PtMP installs /128 host routes through the existing OSPFv3 route path; no new meta key |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPFv3 telemetry doc -- the three `ze_ospfv3_nbma_*`/`ze_ospfv3_ptmp_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] check | umbrella metrics list + `docs/plugin-overview.md` if the metric inventory is listed |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed `origination_v6*.go`/`afstrategy_v6.go`/`config.go`/`yang` files |
| 17 | Existing docs show examples for this area? | [ ] check | verify OSPFv3 interface config examples against the extended v6 `network-type` enum |

## Files to Create
- `internal/plugins/ospf/origination_v6_test.go` additions (`TestOSPFv3PtMPRouterLSALinks`, `TestOSPFv3PtMPNoNetworkLSA`, `TestOSPFv3NBMANetworkLSA`)
- `internal/plugins/ospf/origination_v6_link_test.go` additions (`TestOSPFv3PtMPHostRoute`, `TestOSPFv3NBMALinkLSA`, `TestOSPFv3PtMPLinkLSA`)
- `internal/plugins/ospf/afstrategy_v6_test.go` addition (`TestOSPFv3PtMPNextHop`)
- `internal/plugins/ospf/config_test.go` additions (`TestOSPFv3ParsePtMPInterface`, `TestOSPFv3ParseNBMAInterface`, `TestOSPFv3NetworkTypeV4V6Isolation`)
- `internal/plugins/ospf/iface/iface_test.go` additions (`TestOSPFv3PtMPISMNoElection`, `TestOSPFv3NBMAISMWaiting`, `TestOSPFv3NBMAElection`, `TestOSPFv3PtMPNoElection`, `TestOSPFv3NBMAUnicastHello`, `TestOSPFv3NBMAPollAttempt`, `TestOSPFv3NBMAStartHelloPriorityZero`) -- v6 `Config` variants
- `internal/plugins/ospf/neighbor/nsm_test.go` additions (`TestOSPFv3ShouldAdjPtMP`, `TestOSPFv3ShouldAdjNBMA`, `TestOSPFv3PtMPAdjacency`, `TestOSPFv3NBMAAdjacency`)
- `internal/plugins/ospf/lsdb/flooding_test.go` additions (`TestOSPFv3NBMAFloodUnicast`, `TestOSPFv3PtMPFloodUnicast`)
- `test/ospfv3/ospfv3-nbma.ci`, `test/ospfv3/ospfv3-ptmp.ci`, `test/ospfv3/ospfv3-nbma-config.ci`
- `test/interop/scenarios/ospfv3-ptmp-frr/` -- `ze.conf`, `frr.conf`, `check.py`
- `test/interop/scenarios/ospfv3-nbma-frr/` -- `ze.conf`, `frr.conf`, `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm the shared election/ISM/NSM/flood mechanism (or its delivery in `spec-ospf-ext-8`), the v6 broadcast origination, the v6 Link-LSA, the v6 next-hop, and the v3 unicast transport exist |
| 3. Wiring phase | Wiring Test table -- v6 enum + parse + failing wiring tests |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- the v6 config surface + failing wiring tests
   - Tests: `TestOSPFv3ParsePtMPInterface`, `TestOSPFv3ParseNBMAInterface`, `TestOSPFv3NetworkTypeV4V6Isolation`, `test/ospfv3/ospfv3-nbma-config.ci`
   - Files: `yang/ze-ospf-conf.yang` (v6 enum + v6 `nbma-neighbor` + v6 `poll-interval`), `config.go` (v6 accept-list + new v6 struct fields + parse), `instance.go` (thread the v6 NBMA extras into `ospfiface.Config`); confirm the shared `iface`/`neighbor`/`lsdb` mechanism exists (from `spec-ospf-ext-8`) or add the AF-neutral branch once
   - Verify: the two new v6 network types parse and reach the v6 iface runtime; the v6 behaviour branches are stubs so the deeper tests still fail
2. **Phase: PtMP ISM/NSM + adjacency (v6)** -- treat v6 PtMP as point-to-point
   - Tests: `TestOSPFv3PtMPISMNoElection`, `TestOSPFv3PtMPNoElection`, `TestOSPFv3ShouldAdjPtMP`, `TestOSPFv3PtMPAdjacency`
   - Files: the shared `iface.go` (`Start` PtMP -> point-to-point branch, no election) + `nsm.go` (`shouldAdj` PtMP true) -- exercised on a v6 `Config`; no v6-specific code if the v2 sibling has landed them
   - Verify: a v6 PtMP interface forms Full with every neighbour and never elects
3. **Phase: PtMP v6 origination + next-hop** -- p2p links + /128 host route, no Network-LSA
   - Tests: `TestOSPFv3PtMPRouterLSALinks`, `TestOSPFv3PtMPHostRoute`, `TestOSPFv3PtMPNoNetworkLSA`, `TestOSPFv3PtMPLinkLSA`, `TestOSPFv3PtMPNextHop`
   - Files: `origination_v6.go` (`v6RouterLSABody` PtMP case + the /128 LA-bit host route in the self Intra-Area-Prefix-LSA + the Network-LSA gate excluding PtMP), `origination_v6_link.go` (`v6ShouldOriginateLinkLSA` PtMP), `afstrategy_v6.go` (verify next-hop)
   - Verify: v6 PtMP emits address-free p2p links + a /128 LA-bit host route, no subnet prefix, no Network-LSA; SPF next-hop = neighbour link-local
4. **Phase: NBMA ISM + election + v6 Network/Link-LSA** -- broadcast-like over a static list
   - Tests: `TestOSPFv3NBMAISMWaiting`, `TestOSPFv3NBMAElection`, `TestOSPFv3ShouldAdjNBMA`, `TestOSPFv3NBMAAdjacency`, `TestOSPFv3NBMANetworkLSA`, `TestOSPFv3NBMALinkLSA`
   - Files: shared `iface.go` (election gate widened, Waiting/DROther init, NBMA neighbour seeding) + `nsm.go` (`shouldAdj` NBMA DR-gated) exercised on v6; `origination_v6.go` (NBMA transit link + DR Network-LSA/Network-Intra-Area-Prefix gate widened to NBMA), `origination_v6_link.go` (`v6ShouldOriginateLinkLSA` NBMA)
   - Verify: a v6 NBMA interface elects a DR, forms Full only with DR/BDR, originates a `0x2002` Network-LSA + `0x0008` Link-LSA
5. **Phase: NBMA unicast/poll Hello + non-broadcast flood (v6)** -- unicast to link-local
   - Tests: `TestOSPFv3NBMAUnicastHello`, `TestOSPFv3NBMAPollAttempt`, `TestOSPFv3NBMAStartHelloPriorityZero`, `TestOSPFv3NBMAFloodUnicast`, `TestOSPFv3PtMPFloodUnicast`
   - Files: shared `iface.go` (SendHello unicast/poll loop + Start Hello) + `lsdb/flooding.go` (`floodDestination`/initial-flood unicast fan-out) exercised on v6 with the neighbour link-local as the unicast destination via v3 `SendPacket`
   - Verify: v6 NBMA Hellos/floods reach each neighbour link-local; silent neighbours polled; priority-0 gets a Start Hello; no `ff02::5` on the non-broadcast interface
6. **Functional tests** -> `ospfv3-nbma.ci`, `ospfv3-ptmp.ci`, `ospfv3-nbma-config.ci` cover the user-visible behaviour
7. **RFC refs** -> add `// RFC 5340 §A.4.3 / §A.4.4 / §A.4.10 / §A.4.1 / §2.9` comments on the PtMP p2p-link, NBMA Network-LSA, host-route prefix, and unicast-flood code
8. **Interop** -> `ospfv3-ptmp-frr`, `ospfv3-nbma-frr` QEMU scenarios against FRR `ospf6d`
9. **Full verification** -> `make ze-verify`
10. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has a file:line implementation |
| Feature completeness | each user story has a working path; v6 NBMA/PtMP parity with FRR `ospf6d` for the two new network types |
| Correctness | PtMP address-free p2p links + /128 LA-bit host route (NOT a Router-LSA stub link); no PtMP Network-LSA/election; NBMA election + `0x2002` Network-LSA + Link-LSA; unicast Hello/flood `dst` = the neighbour link-local; v6-only enum change |
| Naming | `ze_ospfv3_nbma_*`/`ze_ospfv3_ptmp_*` metrics; v6 YANG kebab-case; v6 `NetworkType` strings reuse the shared constants |
| Data flow | the shared `iface`/`neighbor`/`lsdb` stay AF-neutral; the v6 prefix/host-route logic lives only in `origination_v6*.go`; next-hop reuses `v6NextHop` |
| CLI grammar | no new command; `show ipv6 ospf interface` reused |
| Doctor checks | none added (no new runtime dependency) -- confirm |
| YANG validation | the v6 `poll-interval`/`priority` have ranges; `link-local`/`router-id` have validators |
| Prometheus counters | the three v6 series defined, registered, listed; umbrella list updated |
| Rule: plugin-self-containment | the v6 enum/leaves/origination are all within the OSPF plugin; no v3 spelling leaks into generic packages |
| Rule: buffer-first | the p2p links + host route append into existing buffer-first slices; the unicast Hello reuses one built buffer per neighbour |
| Rule: ze-divergences | no `fmt`/`+` on the wire path; the v6 origination uses the existing `WriteTo` encoders |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| v6 enum gains the two values, v4 untouched | `grep -nE 'nbma|point-to-multipoint' internal/plugins/ospf/yang/ze-ospf-conf.yang` shows them under the v6 leaf only |
| PtMP v6 origination | `go test ./internal/plugins/ospf -run 'TestOSPFv3PtMP(RouterLSALinks|HostRoute|NoNetworkLSA)'` |
| NBMA v6 Network/Link-LSA | `go test ./internal/plugins/ospf -run 'TestOSPFv3NBMA(NetworkLSA|LinkLSA)'` |
| v6 next-hop reused | `go test ./internal/plugins/ospf -run TestOSPFv3PtMPNextHop` |
| Unicast Hello/flood to link-local | `go test ./internal/plugins/ospf/... -run 'TestOSPFv3(NBMA|PtMP)(UnicastHello|FloodUnicast)'` |
| Three v6 metric series registered | `grep -rn 'ze_ospfv3_nbma_\|ze_ospfv3_ptmp_' internal/plugins/ospf` |
| Interop scenarios present | `ls test/interop/scenarios/ospfv3-ptmp-frr/ test/interop/scenarios/ospfv3-nbma-frr/` |
| Functional tests present | `ls test/ospfv3/ospfv3-nbma.ci test/ospfv3/ospfv3-ptmp.ci test/ospfv3/ospfv3-nbma-config.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | the configured `nbma-neighbor` link-local must be a valid IPv6 link-local unicast before being used as a `dst`; an invalid or non-link-local address is rejected at config validation, never passed to `SendPacket` |
| Resource exhaustion | the configured-neighbour list is bounded by the existing `maxNeighbors` (1024) guard; the PollInterval poll cannot create unbounded send work (one packet per configured neighbour per poll tick) |
| Trust boundary | a received unicast v6 Hello on an NBMA interface from a non-configured source is treated per the existing NSM rules; the RFC 7166 auth trailer (if enabled) verifies before ISM/NSM, unchanged by this spec |
| Spoofed link-local | the unicast `dst`/source uses the link-local learned from the neighbour's authenticated Hello (or the configured link-local); no off-link source is honoured (the v3 transport binds the interface) |
| Error leakage | a failed unicast send is counted (`ze_ospfv3_nbma_polls_total` / a send-error metric), not surfaced to peers; no secret material logged |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to the relevant phase and implement |
| Shared mechanism missing (v2 sibling not landed) | Add the AF-neutral branch once in the shared file, keyed on `NetworkType`; do not duplicate it in v6 files |
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
OSPFv3 NBMA/PtMP is the same behavioural split as OSPFv2 (NBMA = broadcast
election over a static list with unicast/poll Hellos; PtMP = point-to-point on a
multi-access medium with a host route), but the data model is address-free:
v3 Router-LSAs name neighbours by Router ID + Interface ID, and ALL prefixes --
including the PtMP host route -- live in the Intra-Area-Prefix-LSA as a /128 with
the LA-bit, never as a Router-LSA stub link. Because the `iface`/`neighbor`/`lsdb`
packages are AF-shared and string-keyed on the network type, the entire ISM/NSM/
election/flood mechanism is reused from the v2 sibling via the `IsV6` flag; the
v6-only work is the origination (`v6RouterLSABody` branches + the host-route
prefix + the NBMA Link/Network gating), the v6 enum/config, and confirming the
existing v6 next-hop already resolves to the neighbour link-local.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| PtMP host route as a /128 LA-bit prefix in the Intra-Area-Prefix-LSA | a Router-LSA stub link (the OSPFv2 shape) | v3 Router-LSAs are address-free (RFC 5340 §A.4.3); prefixes live only in the Intra-Area-Prefix-LSA (§A.4.10); the LA-bit (§A.4.1) marks a local interface address |
| Reuse the shared ISM/NSM/election/flood mechanism from the v2 sibling via `IsV6` | a parallel v6-only iface/neighbor mechanism | the packages are AF-neutral string-keyed; duplicating them would diverge the two versions and double the maintenance; the only AF difference is the multicast group and the unicast destination (link-local), both already `IsV6`-branched |
| v6 next-hop reused unchanged for PtMP (`v6NextHop.P2PNextHop`) | a PtMP-specific next-hop path | v3 next-hops always come from the adjacency link-local (§3.8.1); a PtMP neighbour is reached exactly like a point-to-point neighbour |
| NBMA reuses the v6 broadcast origination (Network-LSA + Link-LSA + Network-Intra-Area-Prefix) | a non-DR NBMA origination model | RFC 5340 §A.4.4 originates a Network-LSA on broadcast OR NBMA; only the Hello addressing differs |
| Add the enum to the v6 leaf only; the v4 leaf is the v2 sibling's | a single shared network-type leaf | the two address families have independent interface config in the YANG; mixing the enums would let a v4 interface accept v6-only values and vice versa |

## Known Limitations
- OSPFv3 virtual links are out of scope (`spec-ospfv3-ext-3`); an NBMA/PtMP interface cannot be a virtual-link transit in this spec.
- RFC 6845 hybrid broadcast-and-PtMP is not implemented (the RFC 5340 PtMP semantics only).
- The NBMA non-broadcast-PtMP variant (unicast Hellos to a configured list for PtMP) reuses the same `nbma-neighbor` list; the default PtMP variant uses multicast discovery.
- Multi-AF (RFC 5838) on the NBMA/PtMP interface is out of scope; the Instance ID stays explicit.

## RFC Documentation

Add `// RFC 5340 Section X.Y: "<quoted requirement>"` above the enforcing code:
- §A.4.3 -- the PtMP address-free Type-1 p2p Router-LSA link per Full neighbour
- §A.4.4 -- the NBMA DR-originated `0x2002` Network-LSA
- §A.4.10 / §A.4.1 -- the PtMP /128 LA-bit host route in the Intra-Area-Prefix-LSA; word-padded prefix encoding
- §2.9 -- unicast Hello/flood to the neighbour link-local on a non-broadcast v6 interface; link-local source
- §3.8.1 -- the v6 next-hop from the adjacency link-local

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
| v6 PtMP as a collection of p2p links with /128 host routes, no DR | functional + interop | `test/ospfv3/ospfv3-ptmp.ci`, `ospfv3-ptmp-frr` |
| v6 NBMA election + Network-LSA over a static neighbour list | functional + interop | `test/ospfv3/ospfv3-nbma.ci`, `ospfv3-nbma-frr` |
| Unicast Hello/poll + unicast flood to the neighbour link-local | unit + interop | `TestOSPFv3NBMAUnicastHello`, `TestOSPFv3NBMAFloodUnicast`, `ospfv3-nbma-frr` |
| PtMP address-free p2p links + /128 LA-bit host route (v3 model) | unit | `TestOSPFv3PtMPRouterLSALinks`, `TestOSPFv3PtMPHostRoute` |
| v6 next-hop reused (neighbour link-local) | unit | `TestOSPFv3PtMPNextHop` |
| v4 enum and origination untouched | unit | `TestOSPFv3NetworkTypeV4V6Isolation` + existing v4 suite green |

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
- [ ] AC-1..AC-14 all demonstrated
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
- [ ] RFC 5340 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (two concrete network types, both RFC-required)
- [ ] No speculative features (only the two new v6 types; no RFC 6845 hybrid)
- [ ] Single responsibility per component (v6 origination vs shared mechanism)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (shared packages stay AF-neutral)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospfv3-ptmp-frr`, `ospfv3-nbma-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospfv3-ext-7-nbma-p2mp.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospfv3-ext-7-nbma-p2mp.md`
