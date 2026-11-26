# Spec: RSVP routing and interface integration (RFC 2205 Sections 2.8, 3.8, 3.9, 3.10, 3.11)

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P1 |
| Release reason | Required correctness of shipped configuration, routing or forwarding behavior; existing partial implementations and remaining proof are retained. |
| Scope | protocol |
| RFC gaps | RFC2205-2-11, RFC2205-2-12, RFC2205-3-6, RFC2205-3-21, RFC2205-3-23, RFC2205-3-33, RFC2205-3-38, RFC2205-3-40 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Own the shipped RSVP routing and interface integration behavior and all required repairs across `RFC2205-2-11`, `RFC2205-2-12`, `RFC2205-3-6`, `RFC2205-3-21`, `RFC2205-3-23`, the unicast portion of `RFC2205-3-33`, and `RFC2205-3-37`/`3-38`/`3-39`/`3-40`/`3-42`. Preserve the already implemented Linux raw protocol-46 transport, Router Alert carriage, netlink route/address queries, and `RSVP_HOP` previous-hop resolution while fixing every unmet MUST defect in implemented RSVP, closing the owned `RFC2205-3-40` weak-audit journal row, and delivering the outstanding native Linux forwarding proofs.

Owner decision (2026-10-08, unfolded into this design): `RFC2205-2-12`, `RFC2205-3-6`, and `RFC2205-3-21` are unmet MUSTs in RSVP that Ze implements, so under `ai/rules/rfc-compliance.md` they are defects this spec fixes, not gaps moved elsewhere. General multicast/interface selection for `RFC2205-3-33` (`Iapp` vs `Isp`, `Local_only` path state) remains owned by `plan/post-release/features/spec-rsvp-multicast-interface-selection.md`. Non-RSVP-hop detection and traffic-control `NonRSVP` reporting (`RFC2205-3-30`) remains owned by `plan/release/fixes/spec-rsvpte-non-rsvp-neighbors.md` (with `RFC2205-3-31` and `RFC3209-4.2.5-1` split to post-release per the 2026-11-24 owner decision there).

| Requirement | RFC text | HEAD status and spec action |
|-------------|----------|-----------------------------|
| `RFC2205-2-11` | "RSVP must therefore provide correct protocol operation even when two RSVP-capable routers are joined by an arbitrary \"cloud\" of non-RSVP routers." (§2.9) | Partial: `engine.go::handlePacket` derives `peer` from `RSVP_HOP`, but `reservation.go::acceptReservation` rejects a returning RESV across a non-RSVP IPv4 gateway because `lsp.NextHop` is the non-RSVP gateway rather than the downstream RSVP peer. Fix RESV admission for implicit/loose non-RSVP-cloud peers (requiring IP TTL decrement below `Send_TTL` and route-via-gateway match while keeping adjacent forwarding gateway) and prove in a 3-namespace Linux lab. |
| `RFC2205-2-12` | "If the destination address does not match any local interface and the message is not a Path or PathTear, the message must be forwarded without further processing by this node." (§2.9) | Defect (owner, 2026-10-08): only `ResvConf` has a non-local relay (`confirmation.go::handleResvConf`); `Resv`, `PathErr`, `ResvErr`, `ResvTear`, and unknown-object non-local messages fall into local decode/processing. Add header-first non-local transparent forwarding in `engine.go::handlePacket` via `Transport.Forward`. |
| `RFC2205-3-6` | "RSVP must not forward (according to the rules of Section 3.9) Path messages that arrive on an incoming interface different from that provided by routing." (§3.1.3) | Defect (owner, 2026-10-08): `Packet.IfIndex` is populated on Linux, but `handlePacket`/`handlePath` never compares it against routing's expected incoming interface. Enforce incoming-interface check before transit PATH forwarding (`ResolveRoute` toward sender for implicit PATH, toward `RSVP_HOP` for explicit ERO PATH). |
| `RFC2205-3-21` | "However, this must not trigger sending a message out the interface through which M arrived (which could happen if the implementation simply triggered an immediate refresh of all state for the session)." (§3.3) | Defect (owner, 2026-10-08): immediate forwarding paths do not exclude the triggering packet's arrival interface or peer. Thread `(pkt.IfIndex, incomingPeer)` exclusion into immediate PATH, PathTear, RESV, ResvTear, PathErr, ResvErr, ResvConf, and non-local forwarding paths while preserving single-LAN explicit-ERO transit between distinct peers (`interop-rsvpte`). |
| `RFC2205-3-23` | "Forwarding of RSVP messages must avoid looping." (§3.4) | Partial: `handlePathTransit` and `handleResvConf` check `Send_TTL <= 1` and `resolveExplicitPath` rejects a local `NextHop`, but `resolveImplicitPath` lacks a local-nexthop check, transit PATH lacks RRO self-address loop detection, and incoming-interface/no-reflection guards are absent. Complete all loop guards and prove in unit and Linux tests. |
| `RFC2205-3-33` (unicast) | "The RSVP process must determine which case holds by examining the path state, to decide which incoming interface to use for sending Resv messages." (§3.9) | Partial: `sendResv` chooses `PrevHop` from path state, but `pathStateBlock` and `mergedPath` do not store `InIfIndex` and `rawTransport.Send` does not pin `OifIndex`. Record `InIfIndex` in path state and pin `OifIndex` via `Transport.SendOnInterface` with fallback to `RouteGet`. Multicast/`Iapp`/`Local_only` stays in `spec-rsvp-multicast-interface-selection.md`. |
| `RFC2205-3-37` | "To forward Path and PathTear messages, an RSVP process must be able to query the routing process(s) for routes." (§3.11.4) | Implemented and tagged (`TestIntegrationRSVPPathCarriage`, `TestIntegrationRSVPPathAvoidsDataFEC`). Preserve implementation and proofs. |
| `RFC2205-3-38` | "RSVP must be able to learn what real and virtual interfaces are active, with their IP addresses." (§3.11.4) | Implemented (`rawTransport.LocalAddresses`), proof open: add tagged Linux integration test covering active veth and virtual (dummy/VLAN) interfaces and link-down distinction, with discrimination record. |
| `RFC2205-3-39` | "Packets received for IP protocol 46 but not addressed to the node must be diverted to the RSVP program for processing, without being forwarded." (§3.11.5) | Implemented and tagged (`TestIntegrationRSVPPathAvoidsDataFEC`). Preserve implementation and extend transit diversion test to confirm no kernel forwarding past the RSVP process without engine relay. |
| `RFC2205-3-40` | "On a router or multi-homed host, the identity of the interface (real or virtual) on which a diverted message is received, as well as the IP source address and IP TTL with which it arrived, must also be available to the RSVP process." (§3.11.5) | Implemented (`receivedIPv4Packet`), audit `weak` (owned journal row 2026-10-30): `TestIntegrationRSVPPathCarriage` has `IP TTL == Send_TTL`. Deliver a diverted PATH whose IP TTL differs from `Send_TTL`, assert `Packet.TTL` equals arrived IP TTL, record break (`data[8]` -> `data[ihl+4]`), and re-judge `enforced`. |
| `RFC2205-3-42` | "RSVP must be able to specify the IP source address and IP TTL to be used when sending Path messages." (§3.11.5) | Implemented and tagged (`TestIntegrationRSVPPathCarriage`). Preserve implementation and proofs. |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform and plugin process-boundary rules
  → Decision: plugins run internal or forked with no plain Go calls across components; wire/peer contracts follow RFC first.
  → Constraint: keep all routing/interface changes inside `internal/plugins/rsvpte/` over its `Transport` seam.
- [ ] `ai/doors/routing-protocols.md` - routing-protocol boundaries, RFC quoting, discrimination records, and interop gates
  → Decision: `rsvpte/transport_linux.go` uses netlink sockets strictly to read routes and links, never to write kernel routes; wire-visible RSVP-TE changes owe named freeRouter scenarios in `test/interop-rsvpte/scenarios/` proved red with the fix reverted and image rebuilt.
  → Constraint: every new/changed RFC-tagged test needs a `./le rfc record` discrimination record and quote above enforcing code; add named freeRouter scenarios (`transit-wrong-incoming-interface-refused` and `ingress-non-rsvp-cloud`) and keep all existing `./le test integration interop-rsvpte` scenarios green.
- [ ] `ai/doors/le-tooling.md` - development launcher and interop-lab test-tree boundaries
  → Decision: interop scenarios live under `test/interop-rsvpte/scenarios/` and run through typed Go checkers in `internal/le/interoplab/rsvpte/`.
  → Constraint: keep scenario directories declarative (config/route files only, no shell scripts) and never import `internal/le` from product code.
- [ ] `docs/architecture/rsvpte/mpls-rsvp-te.md` - RSVP-TE signaling engine, transport carriage, RRO, and dataplane design
  → Decision: `PATH`, `PathTear`, and `ResvConf` use IPv4 Router Alert and an explicitly selected output interface; ordinary replies use the outgoing interface address without Router Alert; transit sends an immediate RESV update when RRO is withdrawn.
  → Constraint: `Transport.Forward` MUST include IPv4 Router Alert (24-byte `0x46` header with `0x94, 4, 0, 0`) whenever forwarding `MsgTypeResvConf` and use a 20-byte plain IPv4 header (`0x45`, no Router Alert) for `Resv`/`PathErr`/`ResvErr`/`ResvTear`; update `Dataplane` in this page.
- [ ] `docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md` - FRR bypass carriage, merge-point merging, and strict `samePeer` identity
  → Decision: bypass `PATH`, `PathTear`, and `ResvConf` select the protected LSP's exact mark-scoped bypass context; alternate merge-point source addresses require live OSPF/IS-IS attribution via `samePeer`.
  → Constraint: preserve `handleResvConf` bypass translation, `mergeBackupPath` branch merging, and strict `samePeer` enforcement for adjacent peers and merge points.
- [ ] `docs/architecture/testing/interop.md` - freeRouter RSVP-TE suite topology, `labLinks` VLAN links, capture assertions, and scoped kernel capabilities
  → Decision: each Docker interop suite and scenario checks only the kernel capabilities it actually requires; assertions read peer `tcpdump` captures and `labLinks` declares multi-link VLAN topologies in Go.
  → Constraint: register both new freeRouter scenarios in `internal/le/interoplab/rsvpte/rsvpte.go` (`labLinks`, `scenarioCheckerMap`) and `checkers.go`, using only existing MPLS/VLAN capabilities, and document them in `docs/architecture/testing/interop.md`.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2205.md` and `rfc/full/rfc2205.txt` Sections 2.9, 3.1.3, 3.3, 3.4, 3.9, 3.11.4, 3.11.5 - normative text for all 11 owned requirements
  → Constraint: non-local non-Path/PathTear messages must be forwarded without further processing (`ResvConf` retains IPv4 Router Alert per §3.3, while `Resv`/`PathErr`/`ResvErr`/`ResvTear` omit it); transit PATH arriving on an unexpected incoming interface must not be forwarded; immediate state forwarding must not send back out the arrival interface; RESV interface selection consults path state; diverted IP TTL must be distinguished from `Send_TTL`.

**Key insights:**
- Nine of ten scenarios in `test/interop-rsvpte/scenarios/` run a Ze transit on a single shared Ethernet segment (`eth0`, `172.29.81.0/24`) forwarding explicit-routed unicast LSPs between distinct peers (`.15` -> `.3` -> `.14`). Unconditionally blocking `outIfIndex == pkt.IfIndex` on explicit-routed unicast hops between distinct peers would break the entire freeRouter interop suite; loop/reflection exclusion must distinguish implicit routes and same-peer bounces from distinct-peer explicit-ERO hops on a shared LAN.
- `TestRSVPNativeProducer` runs three parallel links (`primary`, `bypass-a`, `bypass-b`) between `H` and `M` where `H`'s router-id is routed only over `primary` (or `bypass-b`). Checking incoming interface against `SenderAddr` on explicit-routed PATHs would reject `bypass-a`; explicit-routed PATHs must resolve expected incoming interface against `RSVP_HOP`, while implicit PATHs resolve against `SenderAddr`.
- `rawTransport.Forward` can reuse `t.pathFD` (`IP_HDRINCL`) with `SO_MARK = 0`, writing a 24-byte IPv4 header with Router Alert (`0x46`, `0x94, 4, 0, 0`) when `routerAlert` is true (`MsgTypeResvConf`) and a 20-byte plain IPv4 header (`0x45`, no Router Alert) when `routerAlert` is false (`Resv`, `PathErr`, `ResvErr`, `ResvTear`), while transmitting `pkt.Payload` byte-for-byte unmodified without calling `outgoingMessage`.
- `internal/le/interoplab/rsvpte/rsvpte.go` already supports multi-link VLAN topologies via `labLinks` (`interoplab.Link`), enabling clean freeRouter interop scenarios (`transit-wrong-incoming-interface-refused` and `ingress-non-rsvp-cloud`) with zero changes to `Dockerfile.freertr` or `ze-interop-resv.patch`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/rsvpte/engine.go` - `handlePacket` decodes the whole message via `DecodeMessage` before checking `pkt.Dst`, rejects unknown `0bbbbbbb` objects locally, refreshes local addresses only for `Path`/`Resv`/`ResvConf`, derives `peer` from `RSVP_HOP` on `Path`/`PathTear`, and dispatches `Resv`/`PathErr`/`ResvErr`/`ResvTear` locally regardless of whether `pkt.Dst` is local. `handlePath`/`handlePathTransit` never receives or checks `pkt.IfIndex`, never checks RRO for self-loops, and never checks `samePeer(peer, nextHop)` or `outIfIndex == pkt.IfIndex`. `sendResv` calls `transport.Send(dst, raw)` without an interface index.
- [ ] `internal/plugins/rsvpte/fsm.go` - `pathStateBlock` and `mergedPath` store `Hop` (`rsvpHop`) and `PrevHop` (`netip.Addr`) but hold no `InIfIndex` field.
- [ ] `internal/plugins/rsvpte/routing.go` - `resolveExplicitPath` rejects `e.isLocalAddress(route.NextHop)`, whereas `resolveImplicitPath` returns `ResolveRoute` without checking `isLocalAddress(route.NextHop)`. No incoming-interface resolution helper exists.
- [ ] `internal/plugins/rsvpte/reservation.go` - `acceptReservation` enforces `lsp.NextHop.IsValid() && !e.samePeer(lsp.NextHop, src)`, which drops any RESV arriving across a non-RSVP IPv4 gateway where `lsp.NextHop` is the gateway and `src` is the downstream RSVP router. `acceptReservation`, `handleResvErr`, and `handleResvTear`/`removeReservation` do not receive or exclude triggering `(pkt.IfIndex, pkt.Src)`.
- [ ] `internal/plugins/rsvpte/confirmation.go` - `handleResvConf` already checks `e.isLocalAddress(msg.ResvConfirm)` and relays non-local `ResvConf` via `SendPath` (with FRR bypass translation through `confirmationRoute`), but does not check `samePeer(pkt.Src, route.NextHop)` or `outIfIndex == pkt.IfIndex`.
- [ ] `internal/plugins/rsvpte/frr.go` - `mergeBackupPath` records incoming branches into `lsp.MergedPaths` without recording `InIfIndex`.
- [ ] `internal/plugins/rsvpte/transport.go` - `Transport` exposes `Send`, `SendPath`, `LocalAddresses`, `ResolveRoute`, `Recv`, and `Close`, with no transparent `Forward` method and no interface-pinned `SendOnInterface` method.
- [ ] `internal/plugins/rsvpte/transport_linux.go` - `openSockets` opens wildcard protocol-46 `t.rx` with `IP_ROUTER_ALERT` and `IP_PKTINFO`, `t.sendFD` for replies, `t.pathFD` with `IP_HDRINCL`, and read-only netlink `t.routes`. `Send` queries `RouteGet` without `OifIndex` and rewrites `RSVP_HOP`. `SendPath` writes a 24-byte IPv4+Router-Alert header with `Send_TTL`, rewrites `RSVP_HOP`, and pins `OifIndex`. `LocalAddresses` joins netlink `AddrList` and `LinkList`. `ResolveRoute` queries netlink `RouteGetWithOptions`. `receivedIPv4Packet` extracts `Src`, `Dst`, `TTL` (`data[8]`), and `IfIndex` (`IP_PKTINFO`).
- [ ] `internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go` - `TestIntegrationRSVPPathCarriage` proves `RFC2205-x-1`, `3-37`, `3-40`, `3-41`, `3-42` over a 1-hop veth pair where IP TTL equals `Send_TTL` (leaving `RFC2205-3-40` audit `weak`); `TestIntegrationRSVPPathAvoidsDataFEC` proves `RFC2205-3-37` and `3-39`; no test proves `RFC2205-3-38`, `2-11`, `2-12`, `3-6`, `3-21`, `3-23`, or unicast `3-33`.
- [ ] `internal/le/interoplab/rsvpte/rsvpte.go`, `internal/le/interoplab/rsvpte/checkers.go` - registers 10 freeRouter RSVP-TE scenarios (`scenarioCheckerMap`) and supports multi-link VLAN topologies via `labLinks` (`interoplab.Link`, used today by `plr-backup-path-to-egress-merge-point`); no scenario currently exercises wrong-incoming-interface PATH refusal (`RFC2205-3-6`) or non-RSVP IPv4 cloud signaling (`RFC2205-2-11`).

**Behavior to preserve:**
- Existing `rawTransport` sockets, Router Alert carriage (`SendPath`), `RSVP_HOP` substitution (`outgoingMessage`), bypass table mark isolation (`TestIntegrationRSVPSelectedBypass`), data-FEC avoidance (`TestIntegrationRSVPPathAvoidsDataFEC`), and `receivedIPv4Packet` validation.
- All 10 existing scenarios in `./le test integration interop-rsvpte` (including the 9 single-LAN explicit-ERO transit scenarios where `transit` receives from `.15`/`.12` and forwards to `.14` on `eth0`).
- All multi-link FRR scenarios in `TestRSVPNativeProducer` (`primary`, `bypass-a`, `bypass-b`, `tail`), `handleResvConf` FRR bypass translation, and strict `samePeer` merge-point source verification.
- Existing transit RRO-withdrawal immediate RESV update (`mpls-rsvp-te.md`) and unit tests that omit `Packet.Dst` or `Packet.IfIndex` (`0` / invalid).

**Behavior to change:**
- Add header-first non-local transparent forwarding for non-Path/non-PathTear messages (`RFC2205-2-12`), preserving IPv4 Router Alert on fallback `MsgTypeResvConf` and omitting Router Alert on `Resv`/`PathErr`/`ResvErr`/`ResvTear`.
- Add routing-derived incoming-interface validation before transit PATH forwarding (`RFC2205-3-6`).
- Add triggering arrival-interface and arrival-peer exclusion on immediate forwarding paths (`RFC2205-3-21`) and complete loop-avoidance checks on implicit routes and RROs (`RFC2205-3-23`).
- Record `InIfIndex` in `pathStateBlock`/`mergedPath` and pin `OifIndex` in unicast RESV sends with fallback (`RFC2205-3-33` unicast).
- Allow implicit/loose non-RSVP-cloud RESVs whose IP TTL decremented below `Send_TTL` and whose route traverses `lsp.NextHop` while preserving adjacent forwarding gateway (`RFC2205-2-11`).
- Add Linux integration proofs, discrimination records, and two named freeRouter interop scenarios (`transit-wrong-incoming-interface-refused` and `ingress-non-rsvp-cloud`, each proved red with its fix reverted and image rebuilt) for the wire-visible routing/interface changes and owned `RFC2205-3-40` journal row.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Linux raw protocol-46 RSVP transport | `internal/plugins/rsvpte/transport_linux.go` (`rawTransport.openSockets`, `Send`, `SendPath`, `ResolveRoute`, `LocalAddresses`, `receivedIPv4Packet`) | Diverts wildcard Router Alert protocol-46 packets with `Src`, `Dst`, `TTL`, and `IfIndex`; sends PATH/PathTear/ResvConf with `IP_HDRINCL`+Router Alert and replies via `sendFD`; queries native routes and interface addresses via read-only netlink | Extends: adds `Forward` (reusing `pathFD` with `routerAlert` selecting 24-byte `0x46` Router Alert for `MsgTypeResvConf` vs 20-byte `0x45` plain IPv4 for `Resv`/`PathErr`/`ResvErr`/`ResvTear`, unmodified payload) and `SendOnInterface` (`OifIndex` lookup with `RouteGet` fallback) while preserving every existing send/receive/query path |
| RSVP-TE signaling engine & dispatcher | `internal/plugins/rsvpte/engine.go` (`handlePacket`, `handlePath`, `handlePathTransit`, `sendResv`) | Decodes RSVP messages, resolves `RSVP_HOP` as `peer` on PATH/PathTear, runs egress/transit handlers, and sends RESV upstream | Extends: checks common header and non-local `pkt.Dst` before local processing, checks incoming interface and RRO self-loop before transit PATH relay, excludes arrival `(IfIndex, peer)` on immediate relays, and passes `InIfIndex` to `sendResv` |
| Reservation acceptance, error, and teardown handlers | `internal/plugins/rsvpte/reservation.go` (`acceptReservation`, `handleResvErr`, `handleResvTear`, `removeReservation`) | Validates incoming RESV/ResvErr/ResvTear peers via `samePeer`, installs reservations, and relays immediate upstream/downstream reservation control messages | Extends: adds non-RSVP-cloud RESV admission in `acceptReservation` (gated on `!repaired`, non-strict path, `Packet.TTL < Send_TTL`, and `ResolveRoute(src)` traversing `lsp.NextHop`, preserving adjacent forwarding gateway) and threads triggering `(pkt.IfIndex, pkt.Src)` exclusion into immediate RESV, ResvTear, and ResvErr relays |
| FRR merge-point backup path merging | `internal/plugins/rsvpte/frr.go` (`mergeBackupPath`) | Merges incoming backup PATH branches at a merge point into `lsp.MergedPaths` with per-branch `PrevHop`, `Hop`, and `Adspec` | Extends: records `InIfIndex` from `Packet.IfIndex` on each `mergedPath` entry so unicast RESV interface pinning and arrival-interface exclusion apply per branch |
| Path and reservation state blocks | `internal/plugins/rsvpte/fsm.go` (`pathStateBlock`, `mergedPath`, `resvStateBlock`) | Retains per-LSP `Hop`, `Route`, `PrevHop`, and merged FRR branches | Extends: adds `InIfIndex int` to `pathStateBlock` and `mergedPath` to retain PATH arrival interface for unicast RESV selection and loop exclusion |
| Explicit and implicit route resolver | `internal/plugins/rsvpte/routing.go` (`resolveReceivedPath`, `resolveImplicitPath`, `resolveExplicitPath`) | Resolves ERO strict/loose hops and implicit endpoints via `Transport.ResolveRoute` | Extends: adds local-nexthop loop guard to `resolveImplicitPath` and a helper resolving expected incoming interface for `RFC2205-3-6` |
| Local address and peer identity cache | `internal/plugins/rsvpte/peer_identity.go` (`refreshLocalAddresses`, `isLocalAddress`, `samePeer`, `peerInPrefix`) | Caches assigned namespace IPv4 addresses and OSPF/IS-IS peer aliases | Does not apply (reused unchanged): `peer_identity.go` is not modified; callers invoke existing `isLocalAddress` for non-local forwarding and RRO loop detection and keep existing `samePeer` unchanged for strict adjacent and merge-point peer verification |
| Reservation confirmation relay | `internal/plugins/rsvpte/confirmation.go` (`handleResvConf`, `confirmationRoute`) | Relays non-local `ResvConf` with Router Alert and FRR bypass identity translation | Extends: adds arrival-interface/peer reflection check while keeping FRR bypass confirmation carriage intact |
| Linux namespace wire lab | `internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go` (`newRSVPWireLab`, `TestIntegrationRSVPPathCarriage`, `TestIntegrationRSVPPathAvoidsDataFEC`) | Exercises production `rawTransport` across isolated Linux namespaces with AF_PACKET captures | Extends: adds 3-namespace non-RSVP forwarder, IP-TTL-distinct `RFC2205-3-40` proof, virtual interface `RFC2205-3-38` proof, non-local forwarding (both 20-byte plain IPv4 and 24-byte Router Alert `ResvConf`), and multi-interface incoming/reflection/RESV-pinning proofs |
| FreeRouter RSVP-TE Docker interop lab | `internal/le/interoplab/rsvpte/rsvpte.go` (`labLinks`, `scenarioCheckerMap`), `internal/le/interoplab/rsvpte/checkers.go` | Runs 10 freeRouter/Ze Docker scenarios with optional multi-link VLAN topologies (`interoplab.Link`) and `tcpdump` wire capture assertions | Extends: adds `transit-wrong-incoming-interface-refused` (`RFC2205-3-6`) and `ingress-non-rsvp-cloud` (`RFC2205-2-11`) reusing `labLinks` VLAN links and `tcpdump` checkers |
| MPLS FIB event bus | `internal/plugins/rsvpte/fib.go` (`busFIB`), `internal/core/mplsfib/events.go` | Emits push/swap/pop forwarding entries over `(mpls-fib, entry)` events to `fib-kernel`/`fib-vpp` | Does not apply: this spec changes control-plane routing/interface handling only and keeps `fib-kernel`/`fib-vpp` as sole FIB writers |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC). The SDK hides which. Code that only works internal is broken." | agrees | Every product change stays inside `internal/plugins/rsvpte/` over its `Transport` interface and netlink read handle, introducing no cross-component direct Go calls or in-process-only assumptions. |
| `ai/doors/routing-protocols.md` | Boundaries | the door covering `internal/plugins/rsvpte/engine.go`, `fsm.go`, `routing.go`, `reservation.go`, `confirmation.go`, `frr.go`, `transport.go`, `transport_linux.go` | "No file under `ospf`, `isis` or `ldp` imports the netlink library. `rsvpte/transport_linux.go` uses netlink sockets only to read routes and links." | agrees | `rawTransport.Forward` and `rawTransport.SendOnInterface` use the existing read-only netlink handle strictly for `RouteGetWithOptions`/`AddrList` lookups and never write kernel routes or labels. |
| `ai/doors/routing-protocols.md` | RFC and interop obligations | the door covering `internal/plugins/rsvpte/` | "RSVP-TE uses freeRouter in `test/interop-rsvpte/scenarios/`. Prove each one red with the fix reverted and the image rebuilt (`ai/rules/interop-and-goal-validation.md`)." | agrees | Adds two named freeRouter interop scenarios (`transit-wrong-incoming-interface-refused` and `ingress-non-rsvp-cloud`) under `test/interop-rsvpte/scenarios/` proving the wire-visible routing and non-RSVP-cloud changes against real freeRouter, with each scenario proved red with its fix reverted and image rebuilt. |
| `ai/doors/le-tooling.md` | Where it sits | the door covering `internal/le/interoplab/rsvpte/` and `test/interop-rsvpte/` | "Interop scenarios live under `test/interop*/` and run through `internal/le/interoplab` (`docs/architecture/testing/interop.md`)." | agrees | Places declarative scenario configs under `test/interop-rsvpte/scenarios/` and registers their typed Go checkers and `labLinks` VLAN declarations in `internal/le/interoplab/rsvpte/` without importing `internal/le` from product code. |
| `docs/architecture/rsvpte/mpls-rsvp-te.md` | Dataplane | the `// Design:` page of `engine.go`, `fsm.go`, `routing.go`, `reservation.go`, `transport.go`, `transport_linux.go`; anchored to `engine.go`, `fsm.go`, `reservation.go`, `transport_linux.go`; modified by this spec | "PATH, PathTear and ResvConf use IPv4 Router Alert and an explicitly selected output interface. A PATH's IP source remains the sender, while RSVP_HOP names the preceding node's outgoing interface. Ordinary replies use that interface as their IP source too, so a received RESV supplies an adjacent MPLS next hop rather than a potentially remote router ID. The transport's current assigned IPv4 addresses identify the local node when consuming an ERO or terminating a session; configured link prefixes remain admission metadata, not proof of address ownership." | agrees | Preserves Router Alert carriage on `PATH`, `PathTear`, and `ResvConf` (including fallback non-local `ResvConf` via `Transport.Forward` with `routerAlert = true`, 24-byte `0x46` IPv4 header + `0x94, 4, 0, 0`), keeps 20-byte plain IPv4 (`routerAlert = false`) for non-local `Resv`/`PathErr`/`ResvErr`/`ResvTear`, preserves ordinary reply `RSVP_HOP`/source semantics and assigned-address ownership, and documents the new routing/interface behavior in `Dataplane`. |
| `docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md` | Decision: local repair moves data and control traffic | the `// Design:` page of `frr.go`, `confirmation.go`; anchored to `engine.go`, `fsm.go`, `reservation.go`, `confirmation.go`, `frr.go`, `transport_linux.go` | "PATH, PathTear and ResvConf select the protected LSP's exact bypass context. Their IP destination remains the session endpoint or confirmation receiver; the routing lookup uses the merge point. An absent or unlabeled context is refused, and a terminal reserved-mark rule prevents fallback to the ordinary routing table." | agrees | Preserves `handleResvConf` FRR bypass translation and mark-scoped bypass carriage, retains `mergeBackupPath` branch merging (recording `InIfIndex` per branch), and keeps strict OSPF/IS-IS `samePeer` verification for adjacent and merge-point peers untouched. |
| `docs/architecture/testing/interop.md` | The freeRouter RSVP-TE suite | the `// Design:` page of `internal/le/interoplab/rsvpte/rsvpte.go` and `checkers.go`; anchored to `rsvpte.go` and `checkers.go`; modified by this spec | "Every assertion reads what a peer received. Every container, Ze or freeRouter, runs `tcpdump -vvv` on its `eth0` into `/run/fr/rsvp.txt`, RSVP and every MPLS-labelled frame, because a PATH carried through a bypass LSP is labelled." | agrees | Both new scenarios use `labLinks` VLAN plumbing, declarative peer configs, and `tcpdump` capture assertions in `checkers.go`, and are documented in the suite's scenario table in `docs/architecture/testing/interop.md`. |
| `docs/architecture/testing/interop.md` | Design decisions | decision row on `docs/architecture/testing/interop.md` | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | Both new scenarios run inside the existing `interop-rsvpte` suite using only its existing MPLS/VLAN kernel capabilities and add no unrelated kernel capability check. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Inbound IPv4 protocol-46 datagrams arriving on `rawTransport.rx` (`IP_ROUTER_ALERT` + `IP_PKTINFO`) in `internal/plugins/rsvpte/transport_linux.go`, parsed into `Packet{Src, Dst, IfIndex, TTL, Payload}` by `receivedIPv4Packet` and delivered via `Transport.Recv()` to `engine.handlePacket`.
- Outbound signaling replies and relays from `engine.go`, `reservation.go`, `confirmation.go`, and `reroute.go` calling `Transport.SendOnInterface`, `Transport.SendPath`, and `Transport.Forward`.

### Transformation Path
1. `rawTransport.receive` reads one raw IPv4 protocol-46 datagram and `IP_PKTINFO` ancillary data into `Packet{Src, Dst, IfIndex, TTL, Payload}` via `receivedIPv4Packet` (`TTL` read from IPv4 header byte `data[8]`, `IfIndex` from `IP_PKTINFO`).
2. `engine.handlePacket` acquires `e.control`, decodes only the 8-byte common header via `DecodeHeader(pkt.Payload)`, and branches on destination locality when `pkt.Dst.IsValid()` and `hdr.MsgType` is neither `MsgTypePath` nor `MsgTypePathTear`:
   - Calls `e.refreshLocalAddresses()`.
   - When `!e.isLocalAddress(pkt.Dst)`:
     - If `hdr.MsgType == MsgTypeResvConf`, attempts `DecodeMessage`; when decoded cleanly without an unknown object and `msg.ResvConfirm == pkt.Dst`, delegates to `handleResvConf(pkt, msg)` so FRR bypass translation and Router Alert carriage are preserved; otherwise forwards transparently via `e.forwardNonLocal(pkt, hdr)` with `routerAlert = true`.
     - For `MsgTypeResv`, `MsgTypePathErr`, `MsgTypeResvErr`, `MsgTypeResvTear`, and any other non-Path/non-PathTear type, immediately calls `e.forwardNonLocal(pkt, hdr)` with `routerAlert = false` and returns without calling `DecodeMessage`, `rejectUnknownObject`, or touching LSP/admission/FIB state.
3. `engine.forwardNonLocal(pkt, hdr)` computes outgoing IP TTL (`pkt.TTL - 1` when `pkt.TTL > 0`, else `hdr.TTL - 1`), drops if TTL `<= 1`, resolves the unicast route to `pkt.Dst` via `Transport.ResolveRoute`, drops on error, local `NextHop`, `samePeer(pkt.Src, route.NextHop)`, or `pkt.IfIndex > 0 && route.IfIndex == pkt.IfIndex`, sets `routerAlert := hdr.MsgType == MsgTypeResvConf`, and calls `Transport.Forward(PathRoute{Destination: pkt.Dst, Source: pkt.Src, NextHop: route.NextHop, IfIndex: route.IfIndex}, outTTL, routerAlert, pkt.Payload)`.
4. `rawTransport.Forward` locks `sendMu`, clears `SO_MARK` to `0` on `t.pathFD`, builds the IPv4 header with `TTL = outTTL`, `proto = 46`, and preserved `Src` and `Dst` -- a 24-byte header (`header[0] = 0x46` with Router Alert `0x94, 4, 0, 0` at bytes `20..23`) when `routerAlert` is true (`MsgTypeResvConf`), and a 20-byte plain IPv4 header (`header[0] = 0x45`, no Router Alert) when `routerAlert` is false (`Resv`/`PathErr`/`ResvErr`/`ResvTear`) -- pins `IP_PKTINFO{Ifindex: route.IfIndex}` with zero `Spec_dst`, and transmits `payload` byte-for-byte unmodified to `route.NextHop`.
5. For local messages and all `Path`/`PathTear` messages, `handlePacket` proceeds to `DecodeMessage`, `RSVP_HOP` peer extraction, unknown-object handling, and local address refresh:
   - `handlePath(pkt, peer, msg)` runs policy, route selection (`resolveReceivedPath` with local-nexthop guard in `resolveImplicitPath`), MTU floor check, `mergeBackupPath(peer, pkt.IfIndex, msg, selected.ERO)`, and egress check (`handlePathEgress(peer, pkt.IfIndex, msg)`).
   - Before `handlePathTransit(pkt, peer, msg, selected)` installs or forwards transit PATH state:
     - Incoming-interface check (`RFC2205-3-6`): when `pkt.IfIndex > 0`, resolves expected incoming route via `routing.go::expectedIncomingRoute` (`SenderTemplate.SenderAddr` if `!msg.HasERO`, `peer` if `msg.HasERO`); drops without forwarding if route lookup fails or returns `expected.IfIndex > 0 && expected.IfIndex != pkt.IfIndex`.
     - Loop and reflection checks (`RFC2205-3-21`, `RFC2205-3-23`): rejects transit PATH if `msg.HasRRO` already contains a local address (sending PathErr Routing Problem / dropping without forwarding), if `e.samePeer(peer, selected.Route.NextHop)`, or if `!msg.HasERO && pkt.IfIndex > 0 && selected.Route.IfIndex == pkt.IfIndex`.
   - `handlePathEgress`, `handlePathTransit`, and `mergeBackupPath` store `InIfIndex: pkt.IfIndex` on `pathStateBlock` / `mergedPath`.
6. `handleResv(pkt, msg)` -> `acceptReservation(pkt, msg, descriptor, filter)`:
   - Preserves strict `samePeer(lsp.NextHop, pkt.Src)` for strict-hop and FRR-repaired LSPs.
   - For an implicit or loose-hop LSP (`!repaired` and `!strictHop`) whose `pkt.TTL > 0 && pkt.TTL < msg.Header.TTL` (traversed a non-RSVP cloud), verifies `msg.HasHop && msg.Hop.NextHop == pkt.Src` and `ResolveRoute(pkt.Src/32, pkt.Src, 0)` routes via `lsp.NextHop` (same gateway and interface); accepts the RESV while keeping `lsp.PSB.Route.NextHop` as the adjacent forwarding gateway for FIB programming (`RFC2205-2-11`).
   - Immediate upstream RESV relay (`sendResvImmediate(lsp, pkt.IfIndex, pkt.Src)`) skips any target matching `samePeer(target, pkt.Src)` or any merged branch whose `InIfIndex == pkt.IfIndex` (`RFC2205-3-21`), and calls `Transport.SendOnInterface(dst, inIfIndex, raw)` so RESV exits the PATH's recorded arrival interface (`RFC2205-3-33` unicast).
7. `handlePathTear(pkt, peer, msg)`, `handleResvTear(pkt, msg)`, `handlePathErr(pkt, msg)`, `handleResvErr(pkt, msg)`, and `handleResvConf(pkt, msg)` apply the same triggering `(pkt.IfIndex, incomingPeer)` reflection guard before relaying immediately.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Linux kernel ↔ `rawTransport` | Wildcard protocol-46 raw socket (`IP_ROUTER_ALERT`, `IP_PKTINFO`), `IP_HDRINCL` `pathFD` (24-byte Router Alert for PATH/PathTear/ResvConf including fallback `Forward` with `routerAlert=true`; 20-byte plain IPv4 for `Forward` with `routerAlert=false`), `sendFD` (`SendOnInterface` with `OifIndex`), read-only `NETLINK_ROUTE` | No |
| `Transport` ↔ `engine` | Go `Transport` interface (`Packet{Src, Dst, IfIndex, TTL, Payload}`, `SendOnInterface`, `SendPath`, `Forward`, `ResolveRoute`, `LocalAddresses`) | No |
| `engine` ↔ `fib-kernel` / `fib-vpp` | Unchanged `(mpls-fib, entry)` events via `busFIB` | No |
| `interoplab/rsvpte` ↔ Docker (`ze` / `freeRouter`) | Declarative scenario configs and `labLinks` VLAN interfaces observed via container `tcpdump` captures | No |

### Changed-Wire Census

No wire format changes in this spec.

**Search evidence:** Searched `internal/plugins/rsvpte/`, `internal/le/interoplab/rsvpte/`, `test/interop-rsvpte/`, and `docs/architecture/rsvpte/` for `SendPath`, `Send(`, `receivedIPv4Packet`, `DecodeHeader`, `DecodeMessage`, `encodeMessage`, `rsvpProtocol`, and `events.go` (positive controls: `transport_linux.go::SendPath`, `transport_linux.go::Send`, `transport_linux.go::receivedIPv4Packet`, `build.go::encodeMessage`, `wire.go::DecodeMessage`, `events.go::PathErr`). Every RSVP common header, object class/C-Type encoding, YANG configuration leaf (`internal/plugins/rsvpte/yang/ze-rsvp-te-conf.yang`), CLI/RPC show payload (`show_data.go`), and EventBus message (`events.go`, `fib.go`) is unchanged. `rawTransport.Forward` transmits standard RFC 2205 IPv4 protocol-46 frames carrying unmodified RSVP bytes already read by `rawTransport.receivedIPv4Packet` and external RFC 2205 peers (`freeRouter`), using the standard 24-byte IPv4+Router Alert header (`0x46`, `0x94, 4, 0, 0`) for fallback `MsgTypeResvConf` and the standard 20-byte plain IPv4 header (`0x45`) for `Resv`/`PathErr`/`ResvErr`/`ResvTear`. `rawTransport.SendOnInterface` emits the exact same unicast RSVP reply wire format while pinning `OifIndex` on Linux route lookup.

### Integration Points
- `internal/plugins/rsvpte/transport.go` - extends `Transport` with `Forward` (`routerAlert bool`) and `SendOnInterface`; implemented by `rawTransport` (`transport_linux.go`), `fakeTransport` (`rfc3209_engine_test.go`), and `fabricPort` (`interop_test.go`).
- `internal/plugins/rsvpte/engine.go` - `handlePacket` wires non-local forwarding, incoming-interface validation, RRO loop detection, and triggering-interface exclusion directly on the real packet ingress path.
- `internal/plugins/rsvpte/fsm.go`, `frr.go`, `reservation.go`, `confirmation.go`, `routing.go` - share `InIfIndex` and triggering `(IfIndex, peer)` inside the `rsvpte` package.
- `internal/le/interoplab/rsvpte/rsvpte.go`, `internal/le/interoplab/rsvpte/checkers.go` - registers the two new freeRouter scenarios (`transit-wrong-incoming-interface-refused` and `ingress-non-rsvp-cloud`) and their `labLinks` VLAN topologies in the existing `interop-rsvpte` catalog.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | All packets enter `rawTransport.receive` -> `engine.handlePacket` and leave through `Transport.Forward` / `SendOnInterface` / `SendPath`; FIB programming stays on `busFIB`. |
| No unintended coupling (components stay isolated) | Yes | Product edits stay inside `internal/plugins/rsvpte/` with no imports of `internal/le/` or other components/FIB writers; interop scenario registration stays inside `internal/le/interoplab/rsvpte/`. |
| No duplicated functionality (extends existing, does not recreate) | Yes | `Forward` reuses `rawTransport.pathFD` and `sendMu`; `SendOnInterface` shares `outgoingSource` and `outgoingMessage` with `Send`; `expectedIncomingRoute` reuses `Transport.ResolveRoute`. |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `rawTransport.Forward` sends `header[:]` and caller `payload` via scatter-gather `unix.SendmsgBuffers` without copying `payload`; `SendOnInterface` keeps `outgoingMessage` scatter-gather buffers. |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new commands, views, families, or core switches are added; all changes sit inside the registered `rsvp-te` edge plugin. |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Introduces no new plugin, command, config leaf, or family name; searched `internal/plugins/rsvpte/register.go` and `internal/component/plugin/all/` confirming zero central-list changes. |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Explicit-routed unicast LSPs on a shared Ethernet LAN (`interop-rsvpte`) always have distinct upstream and downstream peer addresses (`!samePeer(incomingPeer, nextHop)`), whereas a loop/bounce returns to `samePeer` or hairpins an implicit route on `pkt.IfIndex`. | `internal/le/interoplab/rsvpte/rsvpte.go` lines 4-73 and `test/interop-rsvpte/scenarios/transit-strict-hop-forwarded/`. | Blocking all `outIfIndex == pkt.IfIndex` would break all 9 single-LAN `interop-rsvpte` scenarios. | `./le test integration interop-rsvpte` plus unit tests for both shared-LAN distinct-peer transit and same-peer/implicit reflection refusal | unvalidated |
| A-2 | In `TestRSVPNativeProducer`, every explicit-routed PATH arriving on `primary`, `bypass-a`, or `bypass-b` carries an `RSVP_HOP` address that Linux `ResolveRoute` reaches via that exact arrival interface. | `internal/plugins/rsvpte/producer_integration_linux_test.go` lines 210-300 and `transport_linux.go::SendPath` (`outgoingSource` writes outgoing link address into `RSVP_HOP`). | Explicit bypass PATHs would fail the incoming-interface check. | `TestRSVPNativeProducer` and multi-link Linux integration test | unvalidated |
| A-3 | Linux `RTM_GETROUTE` with `RouteGetOptions{OifIndex: ifIndex}` selects a route via `ifIndex` when reachable on that link and returns an error when `ifIndex` cannot reach `dst`, allowing clean fallback to `RouteGet`. | `transport_linux.go:266` already uses `RouteGetWithOptions` with `OifIndex`. | Unicast RESV would fail when a link goes down or return path is asymmetric. | Linux multi-homed integration test (`TestIntegrationRSVPResvPinsPathInterface`) | unvalidated |
| A-4 | On Linux, `IP_HDRINCL` on `t.pathFD` with `SO_MARK = 0`, `IP_PKTINFO{Ifindex}` with zero `Spec_dst`, and either a 20-byte header (`header[0] = 0x45`, no Router Alert) or a 24-byte header (`header[0] = 0x46` with Router Alert `0x94, 4, 0, 0` for `ResvConf`) preserves a non-local IPv4 `Src` and transmits unmodified RSVP payload bytes. | `transport_linux.go:309-329` already uses `t.pathFD` with `IP_HDRINCL` and zero `Spec_dst` for non-local sender source on 24-byte headers. | Non-local forwarded messages would have source rewritten, drop Router Alert on `ResvConf`, or be rejected by kernel. | Linux 3-namespace integration test (`TestIntegrationRSVPNonLocalForwarding`) with AF_PACKET capture | unvalidated |
| A-5 | Non-RSVP-cloud RESVs arrive with `Packet.TTL < msg.Header.TTL` because at least one intermediate IPv4 router decrements IP TTL without altering RSVP `Send_TTL`. | RFC 2205 Section 2.9 and Section 3.8; Linux `ip_forward` decrements IPv4 TTL by 1 per hop. | Non-RSVP-cloud RESV admission would not trigger if an intermediate tunnel hid TTL decrement (that IP-tunnel case is `RFC2205-3-31`, split to post-release). | Linux 3-namespace non-RSVP cloud test (`TestIntegrationRSVPNonRSVPCloud`) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Incoming-interface or same-interface exclusion breaks single-LAN `interop-rsvpte` or multi-link `TestRSVPNativeProducer`. | Red in `./le test integration interop-rsvpte` or `TestRSVPNativeProducer`. | Keep explicit-ERO incoming check keyed on `RSVP_HOP`, exempt egress and `mergeBackupPath` (neither forwards PATH), and allow distinct-peer explicit unicast hops on a shared LAN. |
| R-2 | Non-local forwarding branch accidentally intercepts `ResvConf` for an FRR-repaired LSP and bypasses `confirmationRoute` translation. | Red in `TestMergePointRestoresConfirmationIdentity` or `TestRSVPNativeProducer` ResvConf step. | Route cleanly decoded `MsgTypeResvConf` through `handleResvConf` before transparent fallback. |
| R-3 | Non-RSVP-cloud RESV acceptance weakens strict merge-point `samePeer` enforcement. | Red in `resv_control_source_test.go` or FRR alias tests, or forged RESV accepted in negative test. | Gate non-RSVP-cloud RESV admission strictly on `!repaired`, non-strict/implicit path, `pkt.TTL > 0 && pkt.TTL < msg.Header.TTL`, `msg.Hop.NextHop == pkt.Src`, and `ResolveRoute(pkt.Src)` traversing `lsp.NextHop`. |
| R-4 | `RFC2205-3-40` discrimination test fails to discriminate `data[8]` vs `data[ihl+4]` if test TTLs coincide. | `./le rfc record` stays green on mutation. | Use a 3-namespace lab (or distinct IP TTL vs `Send_TTL`, e.g., `Send_TTL = 37`, arrived `IP TTL = 36`) and assert `packet.TTL == 36` and `packet.TTL != payload[4]`. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | RSVP-TE transit PATH forwarding, FRR backup merging, non-local RSVP forwarding, or single-LAN `interop-rsvpte` signaling could drop legitimate messages or loop signaling packets. |
| How is it reverted? | Single commit revert of the `internal/plugins/rsvpte/` and doc changes; no config schema or persistent state migration. |
| Who else touches this path? | `plan/release/fixes/spec-rsvpte-non-rsvp-neighbors.md` (reads `Packet.TTL` vs `Send_TTL` for `RFC2205-3-30`), `plan/release/validation/spec-rfc-weak-verdict-proof-gaps.md` (AC-2 tracks `RFC2205-3-40`, discharged when this spec closes its journal row), and `plan/post-release/features/spec-rsvp-multicast-interface-selection.md` (depends on this spec). |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `engine.handlePacket` receiving non-local `Resv`/`PathErr`/`ResvErr`/`ResvTear` | → | `engine.forwardNonLocal` -> `Transport.Forward` | `TestRFC2205NonLocalMessagesForwardedUnmodified` and `TestIntegrationRSVPNonLocalForwarding` |
| `engine.handlePacket` receiving transit `PATH` with `Packet.IfIndex` | → | `engine.handlePath` -> `routing.expectedIncomingRoute` -> `handlePathTransit` | `TestRFC2205PathWrongIncomingInterfaceNotForwarded` and `TestIntegrationRSVPIncomingInterfaceAndLoopGuards` |
| `engine.handlePacket` receiving state-modifying `PATH`/`RESV`/`ResvTear`/`PathErr`/`ResvErr` on `Packet.IfIndex` | → | `handlePathTransit`, `acceptReservation` -> `sendResvImmediate`, `removeReservation`, `handlePathErr`, `handleResvErr`, `handleResvConf` | `TestRFC2205ImmediateForwardExcludesArrivalInterface` and `TestIntegrationRSVPIncomingInterfaceAndLoopGuards` |
| `engine.handlePacket` receiving cyclic RRO or local-nexthop implicit `PATH` | → | `routing.resolveImplicitPath` and `handlePathTransit` RRO loop check | `TestRFC2205ForwardingAvoidsLoops` |
| `engine.handlePacket` receiving `PATH` -> `engine.sendResv` on multi-homed router | → | `pathStateBlock.InIfIndex` -> `sendResv` -> `Transport.SendOnInterface` | `TestRFC2205UnicastResvUsesPathIncomingInterface` and `TestIntegrationRSVPResvPinsPathInterface` |
| `engine.handlePacket` receiving `PATH` and `RESV` across non-RSVP IPv4 forwarder | → | `handlePacket` (`RSVP_HOP`) -> `acceptReservation` non-RSVP-cloud match | `TestRFC2205NonRSVPCloudSignaling` and `TestIntegrationRSVPNonRSVPCloud` |
| `rawTransport.receive` diverting `PATH` with `IP TTL != Send_TTL` | → | `rawTransport.receivedIPv4Packet` (`TTL: data[8]`) | `TestIntegrationRSVPPathCarriage` (extended with distinct IP TTL) |
| `rawTransport.LocalAddresses` querying active real and virtual links | → | `rawTransport.LocalAddresses` | `TestIntegrationRSVPLocalAddressesRealAndVirtual` |
| `./le test integration interop-rsvpte` running multi-link VLAN freeRouter scenarios | → | `internal/le/interoplab/rsvpte/rsvpte.go` (`labLinks`, `scenarioCheckerMap`) -> `checkers.go` -> Ze transit/ingress `engine.handlePacket` / `acceptReservation` | `transit-wrong-incoming-interface-refused` and `ingress-non-rsvp-cloud` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `RFC2205-2-12`: a `Resv`, `PathErr`, `ResvErr`, or `ResvTear` (plus a non-local `Resv` and non-local fallback `ResvConf` each carrying an unknown `0bbbbbbb` object class) arrives at `handlePacket` with a valid non-local `Packet.Dst` and `TTL > 1`, alongside a local-destination control message | `handlePacket` decodes only the common header, confirms `Packet.Dst` is non-local, and emits the exact unmodified RSVP payload through `Transport.Forward` with preserved `Src`/`Dst`, decremented IP TTL, pinned outgoing interface, a 24-byte IPv4 header with Router Alert (`0x46`, `0x94, 4, 0, 0`) when `hdr.MsgType == MsgTypeResvConf`, and a 20-byte plain IPv4 header (`0x45`, no Router Alert) for `Resv`/`PathErr`/`ResvErr`/`ResvTear`, leaving local LSP/admission/FIB tables unchanged and sending zero error replies, while the local-destination control is processed normally | Cleanly decoded `MsgTypeResvConf` stays on `handleResvConf` to preserve FRR bypass translation and Router Alert; synthetic unit packets with zero `Packet.Dst` still dispatch locally; non-local packets with `TTL <= 1`, unroutable destination, local `NextHop`, `samePeer(Src, NextHop)`, or `outIfIndex == Packet.IfIndex` are dropped to prevent loops |
| AC-2 | `RFC2205-3-6`: a transit `PATH` arrives at `handlePacket` with `Packet.IfIndex > 0` on an interface different from routing's expected incoming interface (`ResolveRoute` toward `SenderAddr` for implicit PATH, toward `RSVP_HOP` for explicit ERO PATH), alongside a positive control arriving on the matching interface (in unit, Linux namespace, and freeRouter interop scenario `transit-wrong-incoming-interface-refused`) | The matching-interface PATH is accepted, installs transit state, and relays downstream, whereas the mismatched-interface (or unroutable-source) PATH logs a refusal, relays zero downstream PATHs, and installs zero transit state; `transit-wrong-incoming-interface-refused` passes against freeRouter and goes red when the incoming-interface guard is reverted and the image rebuilt | Egress PATHs (`isLocalAddress(TunnelEndpoint)`) and FRR merge-point backup PATHs (`mergeBackupPath`) do not forward PATH and remain accepted; explicit-ERO multi-link paths (`TestRSVPNativeProducer`) validate against `RSVP_HOP` rather than sender router-id; unit packets with `Packet.IfIndex == 0` bypass the interface-index check |
| AC-3 | `RFC2205-3-21`: a state-modifying `PATH`, `PathTear`, `RESV`, `ResvTear`, `PathErr`, `ResvErr`, or `ResvConf` arrives on `(Packet.IfIndex, incomingPeer)` on a multi-interface or merged-branch node, alongside a positive control with a distinct downstream/upstream interface | Immediate forwarding delivers the state modification to every distinct non-arriving upstream/downstream interface and skips every route returning to `samePeer(incomingPeer, nextHop)`, every implicit-route hairpin with `outIfIndex == Packet.IfIndex`, and every merged branch with `InIfIndex == Packet.IfIndex`, leaving unrelated sessions untouched | Distinct unicast peers sharing a single broadcast LAN under an explicit ERO or distinct PHOP (`interop-rsvpte` single-`eth0` scenarios) remain permitted because `samePeer` is false; egress RESV replies to PATH, `PathErr`/`ResvErr` error replies, local `ResvConf` (`confirmHere`), and the RFC 3209 §4.4.3 transit RRO-withdrawal RESV update (`mpls-rsvp-te.md`) remain permitted as opposite-direction replies |
| AC-4 | `RFC2205-3-23`: a transit `PATH` carries a received RRO already containing a local address, or an implicit PATH resolves to a local `NextHop`, or `Send_TTL <= 1`, alongside a loop-free 3-node LSP positive control | The loop-free LSP establishes end-to-end, whereas a local-nexthop implicit route returns PathErr Routing Problem, an RRO self-loop refuses forwarding (PathErr Routing Problem / no downstream relay), and `Send_TTL <= 1` drops cleanly with zero forwarded packets | Multicast Wildcard-Filter `SCOPE`-object loop avoidance remains out of scope (`plan/post-release/features/spec-rsvp-multicast-wildcard.md`), because Ze implements unicast FF and SE LSP tunnels only |
| AC-5 | `RFC2205-3-33` (unicast): a multi-homed router receives a `PATH` on link 0 (`IfIndex = A`) from a PHOP reachable on both link 0 and link 1 (with default route preferring link 1) | Path state records `InIfIndex = A` on `pathStateBlock` (and per branch on `mergedPath`), and `sendResv` calls `Transport.SendOnInterface(dst, A, raw)` so the Linux kernel transmits the unicast RESV out link 0 (observed by AF_PACKET capture on link 0 and zero frames on link 1), with fallback to `RouteGet` when `InIfIndex == 0` or link 0 loses its route | General multicast/local-application interface selection (`Iapp` vs `Isp`, `Local_only` path state) remains owned by `plan/post-release/features/spec-rsvp-multicast-interface-selection.md`; fallback when `InIfIndex == 0` or the recorded link has no route stays permitted for synthetic tests and asymmetric non-RSVP return paths |
| AC-6 | `RFC2205-2-11`: two RSVP-capable routers (`H`, `E`) are separated by a non-RSVP IPv4 forwarder (`N`, no RSVP socket) and signal an implicit or loose-hop LSP, alongside a negative control from an unrelated third host (in unit, 3-namespace Linux lab, and freeRouter interop scenario `ingress-non-rsvp-cloud`) | `H`'s PATH traverses `N` (IP TTL decremented, Sender/Session/Router Alert/`RSVP_HOP` intact), `E` sends RESV unicast back to `H`'s `RSVP_HOP` through `N`, `H` accepts the TTL-decremented gateway-routed RESV (retaining `N` as adjacent forwarding gateway) and brings the LSP Up, and `PathTear` tears both sides down cleanly, while the unrelated host's RESV is ignored; `ingress-non-rsvp-cloud` passes against freeRouter and goes red when the non-RSVP-cloud RESV admission is reverted and the image rebuilt | Strict-hop EROs and FRR merge-point alternate-source checks keep strict `samePeer` verification (`mpls-rsvp-te-fast-reroute.md`); TTL-based `NonRSVP` traffic-control reporting (`RFC2205-3-30`) stays in `plan/release/fixes/spec-rsvpte-non-rsvp-neighbors.md`, and IP-tunnel/neighbor suppression (`RFC2205-3-31`, `RFC3209-4.2.5-1`) stays post-release |
| AC-7 | `RFC2205-3-40` (closing owned journal row) and `RFC2205-3-38`: (a) a diverted non-local PATH arrives on `rawTransport` with IP TTL `36` and RSVP `Send_TTL` `37`; (b) a namespace has active veth and virtual (dummy and VLAN) links plus one downed link | (a) `receivedIPv4Packet` delivers `Packet.TTL == 36` (`!= payload[4]`), `Packet.Src`, and `Packet.IfIndex`, a recorded break moving `data[8]` to `data[ihl+4]` turns the test red, and independent re-judgment restores `RFC2205-3-40` to `enforced`; (b) `rawTransport.LocalAddresses` returns every active veth, dummy, and VLAN IPv4 address with `Up == true`, index, and MTU, and marks the downed link `Up == false`, with a recorded break | Non-Linux platforms continue returning `errTransportUnsupported` via `transport_other.go` (`ai/rules/platform-linux.md`) |
| AC-8 | `RFC2205-3-37`, `3-39`, `3-42` and regressions: native Linux transport tests, transit diversion check, `TestRSVPNativeProducer`, and `./le test integration interop-rsvpte` | All existing native carriage tests pass, transit diversion proves a diverted non-local PATH reaches `rawTransport` and emits zero frames on the router's onward link without engine relay, `TestRSVPNativeProducer` passes, and all 12 `interop-rsvpte` scenarios (10 existing + 2 new) pass | Linux `ip_forward` Router Alert interception behavior remains kernel-side (`rfc/short/rfc2205.md` `RFC2205-3-39` single-polarity annotation); Ze's obligation is keeping the wildcard `IP_ROUTER_ALERT` socket and verifying non-forwarding until engine relay |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | A non-local `Resv`, `PathErr`, `ResvErr`, `ResvTear`, or fallback `ResvConf` arrives at a Ze router | `rawTransport.receive` -> `engine.handlePacket` (`DecodeHeader` + `!isLocalAddress`) -> `forwardNonLocal` -> `rawTransport.Forward` (24-byte Router Alert for `ResvConf`; 20-byte plain IPv4 for `Resv`/`PathErr`/`ResvErr`/`ResvTear`; unmodified payload) -> remote peer | `TestIntegrationRSVPNonLocalForwarding` |
| 2 | A transit `PATH` arrives on a wrong incoming interface or loops back | `rawTransport.receive` (`IP_PKTINFO`) -> `engine.handlePacket` -> `expectedIncomingRoute` / RRO loop check / arrival exclusion -> dropped without relay | `TestIntegrationRSVPIncomingInterfaceAndLoopGuards` and `transit-wrong-incoming-interface-refused` |
| 3 | A multi-homed Ze router receives a `PATH` on link 0 and sends `RESV` upstream | `rawTransport.receive` (`IfIndex`) -> `pathStateBlock.InIfIndex` -> `sendResv` -> `rawTransport.SendOnInterface` (`OifIndex`) -> link 0 capture | `TestIntegrationRSVPResvPinsPathInterface` |
| 4 | Two RSVP routers signal an LSP across a non-RSVP IPv4 forwarding router | `H` `SendPath` -> `N` (`ip_forward`, TTL-1) -> `E` `receivedIPv4Packet` (`Packet.TTL != Send_TTL`) -> `E` `SendOnInterface` -> `N` -> `H` `acceptReservation` -> LSP Up -> `PathTear` | `TestIntegrationRSVPNonRSVPCloud` and `ingress-non-rsvp-cloud` |
| 5 | Operator runs RSVP-TE over veth, dummy, and VLAN interfaces | `rawTransport.LocalAddresses` (netlink `AddrList` + `LinkList`) -> `engine.refreshLocalAddresses` -> `isLocalAddress` / ERO consumption | `TestIntegrationRSVPLocalAddressesRealAndVirtual` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC2205NonLocalMessagesForwardedUnmodified` | `internal/plugins/rsvpte/rfc2205_routing_interface_test.go` | AC-1 (`RFC2205-2-12` positive + negative: non-local `Resv`/`PathErr`/`ResvErr`/`ResvTear` and unknown-object `Resv` forwarded byte-for-byte with `routerAlert=false`; non-local fallback `ResvConf` forwarded byte-for-byte with `routerAlert=true`; local-destination control processed; TTL `<= 1` and same-interface dropped; FRR `ResvConf` preserved) | |
| `TestRFC2205PathWrongIncomingInterfaceNotForwarded` | `internal/plugins/rsvpte/rfc2205_routing_interface_test.go` | AC-2 (`RFC2205-3-6` positive + negative: implicit PATH checks `SenderAddr` route, explicit ERO PATH checks `RSVP_HOP` route; matching interface relays, mismatched or unroutable drops; egress and merge-point backup preserved) | |
| `TestRFC2205ImmediateForwardExcludesArrivalInterface` | `internal/plugins/rsvpte/rfc2205_routing_interface_test.go` | AC-3 (`RFC2205-3-21` positive + negative: implicit PATH/PathTear and non-local drop on `outIfIndex == pkt.IfIndex`; explicit PATH/PathTear drop on `samePeer`; immediate RESV/ResvTear/PathErr/ResvErr skip `samePeer` and merged branch with `InIfIndex == pkt.IfIndex`; distinct-peer shared-LAN explicit transit and distinct-interface relay pass) | |
| `TestRFC2205ForwardingAvoidsLoops` | `internal/plugins/rsvpte/rfc2205_routing_interface_test.go` | AC-4 (`RFC2205-3-23` positive + negative: implicit local `NextHop`, RRO containing local address, `Send_TTL <= 1`, and loop-free 3-node fabric control) | |
| `TestRFC2205UnicastResvUsesPathIncomingInterface` | `internal/plugins/rsvpte/rfc2205_routing_interface_test.go` | AC-5 (unicast `RFC2205-3-33` positive + negative: `pathStateBlock.InIfIndex` and `mergedPath.InIfIndex` recorded and handed to `SendOnInterface`; updated when PATH switches interface) | |
| `TestRFC2205NonRSVPCloudSignaling` | `internal/plugins/rsvpte/rfc2205_routing_interface_test.go` | AC-6 (`RFC2205-2-11` positive + negative: TTL-decremented RESV from downstream RSVP peer routed via gateway accepted with adjacent gateway retained; forged non-gateway RESV and strict-hop/merge-point alias rejected) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Received IP TTL / Send_TTL for forwarding (`forwardNonLocal`, `handlePathTransit`) | 2-255 forwarded (decremented by 1) | 2 (forwarded as 1) | 0, 1 (dropped, not forwarded) | N/A (`uint8` max 255) |
| Interface index (`Packet.IfIndex`, `InIfIndex`, `OifIndex`) | 1..max int (0 = unset in unit tests) | 1 | 0 (rejected by `receivedIPv4Packet`; bypasses pinning in unit fixtures), negative (rejected) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestIntegrationRSVPPathCarriage` (extended) | `internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go` | AC-7a, AC-8 (`RFC2205-3-40` diverted PATH with `IP TTL != Send_TTL`, plus `3-37`, `3-41`, `3-42`) | |
| `TestIntegrationRSVPPathAvoidsDataFEC` (extended) | `internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go` | AC-8 (`RFC2205-3-37`, `3-39` transit diversion confirmed not kernel-forwarded onward) | |
| `TestIntegrationRSVPLocalAddressesRealAndVirtual` | `internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go` | AC-7b (`RFC2205-3-38` real veth + virtual dummy/VLAN active and down interface discovery) | |
| `TestIntegrationRSVPNonLocalForwarding` | `internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go` | AC-1 (`RFC2205-2-12` transparent forwarding across 3 namespaces: 20-byte plain IPv4 for `Resv`/`PathErr`, 24-byte Router Alert for fallback `ResvConf`) | |
| `TestIntegrationRSVPIncomingInterfaceAndLoopGuards` | `internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go` | AC-2, AC-3, AC-4 (`RFC2205-3-6`, `3-21`, `3-23` multi-interface Linux lab) | |
| `TestIntegrationRSVPResvPinsPathInterface` | `internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go` | AC-5 (unicast `RFC2205-3-33` 2-link namespace lab pinning RESV to PATH arrival link against default route) | |
| `TestIntegrationRSVPNonRSVPCloud` | `internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go` | AC-6 (`RFC2205-2-11` 3-namespace `H -- N(non-RSVP) -- E` PATH/RESV/PathTear establishment and teardown) | |
| `TestRSVPNativeProducer` | `internal/plugins/rsvpte/producer_integration_linux_test.go` | AC-8 (regression: 3-daemon native OSPF + FRR + strict/loose + bypass + confirmation + shutdown) | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `transit-wrong-incoming-interface-refused` | `test/interop-rsvpte/scenarios/transit-wrong-incoming-interface-refused/` | freeRouter | AC-2 (`RFC2205-3-6`): multi-link VLAN 100 (`link0`) + `eth0` topology where Ze transit refuses a freeRouter PATH arriving on `link0` when routing expects `eth0` (zero PATH at freeRouter egress, zero swap on Ze), while accepting the matching-interface positive control on `eth0`; proved red with the `RFC2205-3-6` check reverted and image rebuilt | |
| `ingress-non-rsvp-cloud` | `test/interop-rsvpte/scenarios/ingress-non-rsvp-cloud/` | freeRouter | AC-6 (`RFC2205-2-11`): Ze ingress (VLAN 100) signals a loose-hop LSP through a non-RSVP freeRouter IPv4 forwarder (`relay`, no `mpls rsvp4`, decrementing TTL) to a freeRouter RSVP egress (VLAN 200), accepts the TTL-decremented gateway-routed RESV, brings the LSP Up, and tears it down cleanly; proved red with the `RFC2205-2-11` RESV admission reverted and image rebuilt | |
| All 10 existing RSVP-TE scenarios (`./le test integration interop-rsvpte`) | `test/interop-rsvpte/scenarios/` | freeRouter | AC-8 regression: single-LAN explicit-ERO transit (`strict`, `loose`, `ResvErr`, `ResvTear`, `InPlace`, `Implicit NULL`, `FRR backup`) continues to pass without false loop/incoming-interface rejection | |

## Files to Modify

- `internal/plugins/rsvpte/engine.go` - header-first non-local transparent forwarding (`RFC2205-2-12`, setting `routerAlert = hdr.MsgType == MsgTypeResvConf`), transit PATH incoming-interface check (`RFC2205-3-6`), RRO self-loop check (`RFC2205-3-23`), arrival-interface/peer exclusion on immediate relays (`RFC2205-3-21`), and PSB `InIfIndex` propagation into `sendResv` (`RFC2205-3-33` unicast).
- `internal/plugins/rsvpte/fsm.go` - add `InIfIndex int` to `pathStateBlock` and `mergedPath`.
- `internal/plugins/rsvpte/routing.go` - add local-nexthop check to `resolveImplicitPath` (`RFC2205-3-23`) and `expectedIncomingRoute` helper (`RFC2205-3-6`).
- `internal/plugins/rsvpte/reservation.go` - non-RSVP-cloud RESV admission on implicit/loose paths with TTL decrement and gateway match while keeping adjacent forwarding gateway (`RFC2205-2-11`), and arrival-interface/peer exclusion on immediate RESV/ResvTear/ResvErr relays (`RFC2205-3-21`).
- `internal/plugins/rsvpte/confirmation.go` - arrival-interface/peer loop exclusion in `handleResvConf` while preserving FRR bypass confirmation carriage (`RFC2205-2-12`, `3-21`, `3-23`).
- `internal/plugins/rsvpte/frr.go` - record `InIfIndex` on `mergedPath` in `mergeBackupPath` (`RFC2205-3-33` unicast, `3-21`).
- `internal/plugins/rsvpte/transport.go` - add `Forward` (`routerAlert bool`) and `SendOnInterface` methods to `Transport`.
- `internal/plugins/rsvpte/transport_linux.go` - implement `rawTransport.Forward` (24-byte `0x46` Router Alert header when `routerAlert` is true for `MsgTypeResvConf`, 20-byte `0x45` plain IPv4 header otherwise, unmodified payload) and `rawTransport.SendOnInterface` (`OifIndex` route lookup with `RouteGet` fallback).
- `internal/plugins/rsvpte/rfc3209_engine_test.go` - implement `Forward` and `SendOnInterface` on `fakeTransport` and support per-packet `IfIndex`/`TTL`.
- `internal/plugins/rsvpte/interop_test.go` - implement `Forward` and `SendOnInterface` on `fabricPort`.
- `internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go` - fix `RFC2205-3-40` distinct IP-TTL proof (closing owned journal row) and add native Linux integration proofs for `RFC2205-3-38`, `3-39`, `2-11`, `2-12`, `3-6`, `3-21`, `3-23`, and unicast `3-33`.
- `internal/le/interoplab/rsvpte/rsvpte.go` - declare `labLinks` VLAN links and register `transit-wrong-incoming-interface-refused` and `ingress-non-rsvp-cloud` in `scenarioCheckerMap`.
- `internal/le/interoplab/rsvpte/checkers.go` - implement typed Go checkers for `transit-wrong-incoming-interface-refused` and `ingress-non-rsvp-cloud`.
- `docs/architecture/rsvpte/mpls-rsvp-te.md` - document non-local transparent forwarding (including Router Alert preservation on fallback `ResvConf`), transit incoming-interface validation, loop/reflection exclusion, PSB `InIfIndex` pinning, and non-RSVP-cloud RESV acceptance under `Dataplane`.
- `docs/architecture/testing/interop.md` - document `transit-wrong-incoming-interface-refused` and `ingress-non-rsvp-cloud` in `The freeRouter RSVP-TE suite` table.
- `rfc/short/rfc2205.md` - update compliance checklist rows for `RFC2205-2-11`, `2-12`, `3-6`, `3-21`, `3-23`, `3-33` (unicast proven; multicast still cites `spec-rsvp-multicast-interface-selection.md`), and `3-38`, plus `rfc/discrimination/` and `rfc/audit/` records for new/changed tags and `RFC2205-3-40` re-judgment.
- `plan/journal/green-that-could-not-have-been-red.md` - mark the owned 2026-10-30 `TestIntegrationRSVPPathCarriage` / `RFC2205-3-40` row `fixed:`.

## Files to Create

- `internal/plugins/rsvpte/rfc2205_routing_interface_test.go` - unit and fabric conformance tests tagged for `RFC2205-2-11`, `2-12`, `3-6`, `3-21`, `3-23`, and unicast `3-33`.
- `test/interop-rsvpte/scenarios/transit-wrong-incoming-interface-refused/ingress-hw.txt`, `ingress-sw.txt`, `transit.conf`, `transit-net.txt`, `egress-hw.txt`, `egress-sw.txt` - declarative freeRouter/Ze configs for `RFC2205-3-6` interop scenario.
- `test/interop-rsvpte/scenarios/ingress-non-rsvp-cloud/ingress.conf`, `ingress-net.txt`, `relay-hw.txt`, `relay-sw.txt`, `egress-hw.txt`, `egress-sw.txt` - declarative Ze/freeRouter configs for `RFC2205-2-11` non-RSVP-cloud interop scenario.

## Work Not Done

The release-scope and spec-split rules in `ai/META-DECISIONS.md` approve this scope separation, not completion or additional product implementation. Original dependencies, design decisions, and evidence requirements remain binding within each slice.

| Original obligation | Implementation owner | Boundary |
|---------------------|----------------------|----------|
| `RFC2205-2-11`/`2-12`, `3-6`/`3-21`/`3-23`, unicast `3-33`, and `3-37`/`3-38`/`3-39`/`3-40`/`3-42` | `plan/release/fixes/spec-rsvp-routing-and-interface-integration.md` | Shipped routing/interface repairs, owned `3-40` journal row, and native Linux proofs |
| `RFC2205-3-33`: general multicast/interface-selection extension (`Iapp` vs `Isp`, `Local_only`) | `plan/post-release/features/spec-rsvp-multicast-interface-selection.md` | Absent multicast/host-application extension |
| `RFC2205-3-30`: TTL vs `Send_TTL` non-RSVP-hop detection and `NonRSVP` traffic-control flag (`RFC2205-3-31` and `RFC3209-4.2.5-1` split to post-release) | `plan/release/fixes/spec-rsvpte-non-rsvp-neighbors.md` | Non-RSVP hop detection and traffic-control flag reporting |
| Multicast Wildcard-Filter `SCOPE`-object loop avoidance | `plan/post-release/features/spec-rsvp-multicast-wildcard.md` | Absent multicast WF reservation style |

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new config leaves or RPCs; behavior operates on existing `rsvp-te` runtime transport and engine |
| YANG validation constraints | N-A | No YANG changes |
| YANG custom validators | N-A | No YANG changes |
| CLI commands/flags | No | No CLI surface changes |
| CLI grammar (keyword before value) | N-A | No CLI changes |
| Editor autocomplete | N-A | No YANG or CLI changes |
| Functional test for new RPC/API | N-A | No new RPC/API; protocol carriage is proven by Linux namespace integration tests and `interop-rsvpte` |
| Pipe completeness | N-A | No CLI output changes |
| Env var registration | No | No new environment variables |
| Doctor check for runtime dependencies | No | Uses existing raw protocol-46 socket and netlink handle already probed by `internal/plugins/rsvpte/doctor.go` |
| Doctor code per problem class | No | Incoming-interface drop and loop drop are per-packet peer/routing events logged at `Warn` on existing transport; no new daemon readiness problem class |
| Prometheus counters/metrics | No | Existing `ze_rsvpte_*` counters in `register.go` stay unchanged |
| BGP family surface (new SAFI / capability / attribute) | N-A | RSVP-TE protocol plugin, not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | `docs/features.md` MPLS / LDP / RSVP-TE row stays accurate; RFC compliance detail lives in `rfc/short/rfc2205.md` |
| 2 | Config syntax changed? | No | No YANG or config syntax changes |
| 3 | CLI command added/changed? | No | No CLI changes |
| 4 | API/RPC added/changed? | No | No API/RPC changes |
| 5 | Plugin added/changed? | No | Existing `rsvp-te` plugin registration unchanged |
| 6 | Has a user guide page? | No | `docs/guide/rsvp-te.md` (unaffected: operator config, `show` commands, and carrier commands do not change) |
| 7 | Wire format changed? | No | No wire format changes (see Changed-Wire Census) |
| 8 | Plugin SDK/protocol changed? | No | Internal `rsvpte` package only |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2205.md` (rows `2-11`, `2-12`, `3-6`, `3-21`, `3-23`, `3-33`, `3-38`, `3-40`) and `docs/features/rfc-status.md` if regenerated |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (add `transit-wrong-incoming-interface-refused` and `ingress-non-rsvp-cloud` to `The freeRouter RSVP-TE suite` table) |
| 11 | Affects daemon comparison? | No | No comparison-table change |
| 12 | Internal architecture changed? | Yes | `docs/architecture/rsvpte/mpls-rsvp-te.md` (`Dataplane` section) |
| 13 | Route metadata keys added/changed? | No | No route metadata changes |
| 14 | Prometheus counters added/changed? | No | No metric changes |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration or inventory changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived via `./le spec citation anchors`: `docs/architecture/rsvpte/mpls-rsvp-te.md` (updated -- `// Design:` of `engine.go`, `fsm.go`, `routing.go`, `reservation.go`, `transport.go`, `transport_linux.go`); `docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md` (unaffected -- `// Design:` of `frr.go`, `confirmation.go`, anchored to `engine.go`, `fsm.go`, `reservation.go`, `transport_linux.go`; FRR bypass carriage and strict merge-point `samePeer` stay unchanged); `docs/architecture/testing/interop.md` (updated -- `// Design:` and anchored to `internal/le/interoplab/rsvpte/rsvpte.go`, `checkers.go`); `docs/guide/rsvp-te.md` (unaffected -- anchored to `engine.go`, `transport_linux.go`, `reservation.go`, `frr.go`, `checkers.go`; guide config and carrier steps stay valid); `docs/features.md` (unaffected -- anchored to `engine.go`, `frr.go`, `checkers.go`; feature summary stays accurate) |
| 17 | Existing docs show config/CLI/API examples for this area? | No | Examples in `docs/guide/rsvp-te.md` remain valid against `yang/ze-rsvp-te-conf.yang` |

## Design presentation

### What changes

This spec finishes the remaining RFC 2205 routing and interface integration defects in Ze's shipped RSVP-TE engine and closes the open `RFC2205-3-40` weak-audit journal row. Non-local RSVP messages other than `Path` and `PathTear` are now forwarded transparently as unmodified IPv4 protocol-46 datagrams (preserving the 24-byte IPv4 Router Alert header when forwarding fallback `MsgTypeResvConf` and using a 20-byte plain IPv4 header without Router Alert for `Resv`/`PathErr`/`ResvErr`/`ResvTear`, with zero `RSVP_HOP` rewrite) before local object decoding. Transit `PATH` processing validates the packet's Linux arrival interface against routing (`SenderAddr` for implicit routes, `RSVP_HOP` for explicit EROs), detects RRO self-loops and local-nexthop implicit routes, and excludes the triggering arrival interface/peer from immediate state-change forwarding while preserving single-LAN explicit-route transit (`interop-rsvpte`). Path state retains the `PATH` arrival interface index so unicast `RESV` messages pin that interface on multi-homed routers, and implicit/loose LSP signaling works across a non-RSVP IPv4 forwarding router while preserving strict `samePeer` verification for adjacent and FRR merge-point peers. Native Linux namespace integration tests and two new named freeRouter interop scenarios (`transit-wrong-incoming-interface-refused` and `ingress-non-rsvp-cloud`, each proved red with its fix reverted and image rebuilt) prove the wire-visible behavior, including distinct IP TTL vs `Send_TTL` (`RFC2205-3-40`) and real plus virtual interface discovery (`RFC2205-3-38`).

### Before and after

1. **Non-local `Resv` / `PathErr` / `ResvErr` / `ResvTear` / fallback `ResvConf` (`RFC2205-2-12`)**:
   - Before: a `Resv` addressed to a non-local router `203.0.113.50` arrives at `handlePacket`, runs `DecodeMessage` and `handleResv` locally, fails lookup with `ResvErr` (No Path Information), and is never forwarded; a non-local `ResvConf` with an unknown `0bbbbbbb` object is dropped locally instead of forwarded.
   - After: `handlePacket` reads the common header, sees `Packet.Dst = 203.0.113.50` is non-local and not `Path`/`PathTear`, and calls `Transport.Forward` to send the exact received bytes with decremented IP TTL, original `Src`/`Dst`, a 20-byte plain IPv4 header without Router Alert for `Resv`/`PathErr`/`ResvErr`/`ResvTear`, and a 24-byte IPv4 Router Alert header (`0x46`, `0x94, 4, 0, 0`) when forwarding fallback `ResvConf`.
2. **Transit `PATH` arriving on the wrong interface (`RFC2205-3-6`)**:
   - Before: a transit `PATH` arriving on `eth1` when routing reaches its expected upstream source/PHOP on `eth0` is accepted, installs a PSB, and relays downstream.
   - After: `handlePath` checks `expectedIncomingRoute`, observes `pkt.IfIndex (eth1) != expected.IfIndex (eth0)`, logs a warning, installs no state, and emits zero downstream `PATH` packets.
3. **Immediate state-change forwarding loop/reflection (`RFC2205-3-21`, `RFC2205-3-23`)**:
   - Before: immediate relay paths ignore `pkt.IfIndex` and `incomingPeer`; an implicit route pointing back out the arrival link, a `NextHop` equal to `peer`, a local implicit `NextHop`, or a received RRO containing this router's own address can forward or loop.
   - After: immediate forwarding skips arrival-interface implicit hairpins, same-peer bounces, merged branches matching `pkt.IfIndex`, local implicit next hops, and RRO self-loops while continuing to relay normally to distinct downstream/upstream interfaces and distinct shared-LAN explicit-ERO peers.
4. **Unicast `RESV` interface selection on multi-homed routers (`RFC2205-3-33` unicast)**:
   - Before: `pathStateBlock` discards `Packet.IfIndex`; `rawTransport.Send` always calls `RouteGet(prevHop)`, which can transmit `RESV` out link 1 even though the `PATH` arrived on link 0.
   - After: `pathStateBlock.InIfIndex` (and `mergedPath.InIfIndex`) stores link 0's index, and `sendResv` calls `Transport.SendOnInterface(prevHop, link0, raw)` so the `RESV` exits link 0 (falling back to `RouteGet` only if link 0 is unset or no longer routes `prevHop`).
5. **Signaling across a non-RSVP IPv4 cloud (`RFC2205-2-11`) and `RFC2205-3-40` proof**:
   - Before: when `H` and `E` are separated by non-RSVP router `N`, `H` sets `lsp.NextHop = N` and `acceptReservation` silently drops `E`'s returning `RESV` because `!samePeer(N, E)`. Meanwhile `TestIntegrationRSVPPathCarriage` tests a 1-hop link where `IP TTL == Send_TTL`, leaving `RFC2205-3-40` audit `weak`.
   - After: `H` accepts `E`'s TTL-decremented, gateway-routed `RESV` on implicit/loose paths (keeping `N` as adjacent forwarding gateway), bringing the LSP Up across `N`, and the Linux integration test observes `Packet.TTL == 36 != Send_TTL (37)` on the diverted `PATH`, closing the 2026-10-30 journal row.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Reject every packet where `outIfIndex == pkt.IfIndex` unconditionally on all paths | Breaks all 9 single-LAN explicit-ERO transit scenarios in `./le test integration interop-rsvpte`, where `ingress`, `transit`, and `egress` legitimately share one Ethernet segment (`eth0`) and forward unicast between distinct peers. |
| Validate `RFC2205-3-6` incoming interface against `SenderTemplate.SenderAddr` even when `HasERO` is true | Breaks multi-link explicit bypasses in `TestRSVPNativeProducer`, where `bypass-a` arrives on `bypass-a` while `H`'s router-id (`SenderAddr`) routes over `primary`; explicit EROs must validate incoming interface against `RSVP_HOP`. |
| Reuse `SendPath` for `RFC2205-2-12` non-local forwarding | `SendPath` inserts the IPv4 Router Alert option (RFC 2205 §3.3 restricts Router Alert to `Path`, `PathTear`, `ResvConf`) and rewrites `RSVP_HOP` via `outgoingMessage`, violating §2.9 "forwarded without further processing". |
| Relax `samePeer(lsp.NextHop, src)` unconditionally for `RFC2205-2-11` | Violates `docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md` ("An alternate merge-point source address is accepted only when the live native OSPF or IS-IS database attributes it to the same node") and weakens spoof protection (`resv_control_source_test.go`). |
| Move `RFC2205-3-40` weak-verdict proof only to `spec-rfc-weak-verdict-proof-gaps.md` | This spec already owns the 2026-10-30 journal row and `transport_linux.go`, and the 3-namespace non-RSVP lab for `RFC2205-2-11` naturally produces a diverted `PATH` with `IP TTL != Send_TTL`. |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase 1: Wiring (MANDATORY FIRST)** -- extend `Transport` (`Forward`, `SendOnInterface`), `pathStateBlock`/`mergedPath` (`InIfIndex`), test doubles (`fakeTransport`, `fabricPort`), and write failing unit wiring tests for AC-1..AC-6.
   - Tests: `TestRFC2205NonLocalMessagesForwardedUnmodified`, `TestRFC2205PathWrongIncomingInterfaceNotForwarded`, `TestRFC2205ImmediateForwardExcludesArrivalInterface`, `TestRFC2205ForwardingAvoidsLoops`, `TestRFC2205UnicastResvUsesPathIncomingInterface`, `TestRFC2205NonRSVPCloudSignaling`
   - Files: `internal/plugins/rsvpte/transport.go`, `transport_linux.go`, `fsm.go`, `rfc3209_engine_test.go`, `interop_test.go`, `rfc2205_routing_interface_test.go`
   - Verify: package compiles; new wiring tests fail against stubbed/pre-fix behavior.
2. **Phase 2: Non-local forwarding, incoming-interface validation, loop avoidance, and arrival exclusion (AC-1..AC-4)** -- implement header-first `forwardNonLocal`, `expectedIncomingRoute` transit check, `resolveImplicitPath` local-nexthop check, RRO self-loop detection, and `(pkt.IfIndex, incomingPeer)` exclusion across `engine.go`, `routing.go`, `reservation.go`, `confirmation.go`, and `rawTransport.Forward`.
   - Tests: AC-1..AC-4 unit tests pass; existing `internal/plugins/rsvpte/` unit suite stays green.
   - Files: `internal/plugins/rsvpte/engine.go`, `routing.go`, `reservation.go`, `confirmation.go`, `transport_linux.go`
   - Verify: targeted unit tests go from red to green.
3. **Phase 3: Unicast RESV interface pinning and non-RSVP-cloud RESV admission (AC-5, AC-6)** -- record `InIfIndex` in `handlePathEgress`, `handlePathTransit`, and `mergeBackupPath`; implement `rawTransport.SendOnInterface` with `OifIndex` pinning and `RouteGet` fallback; implement non-RSVP-cloud RESV admission in `reservation.go` preserving adjacent gateway and strict `samePeer` on adjacent/FRR paths.
   - Tests: AC-5, AC-6 unit tests pass; `resv_control_source_test.go` and FRR tests stay green.
   - Files: `internal/plugins/rsvpte/engine.go`, `frr.go`, `reservation.go`, `transport_linux.go`
   - Verify: targeted unit tests pass under race detector.
4. **Phase 4: Native Linux integration proofs, freeRouter interop scenarios, RFC records, and docs (AC-7, AC-8)** -- extend `rfc2205_transport_integration_linux_test.go` with distinct IP-TTL `RFC2205-3-40` proof, `RFC2205-3-38` real+virtual discovery, transit non-forwarding observation (`3-39`), non-local forwarding (`2-12`, both 20-byte plain IPv4 and 24-byte Router Alert fallback `ResvConf`), incoming-interface/loop/RESV-pinning (`3-6`, `3-21`, `3-23`, `3-33`), and 3-namespace non-RSVP cloud (`2-11`); add `transit-wrong-incoming-interface-refused` and `ingress-non-rsvp-cloud` in `test/interop-rsvpte/scenarios/` and `internal/le/interoplab/rsvpte/` (`rsvpte.go`, `checkers.go`) and prove each red with its fix reverted and image rebuilt; record `./le rfc record` discrimination breaks and independent `RFC2205-3-40` re-judgment; update `docs/architecture/rsvpte/mpls-rsvp-te.md`, `docs/architecture/testing/interop.md`, `rfc/short/rfc2205.md`, and close the owned row in `plan/journal/green-that-could-not-have-been-red.md`; run `TestRSVPNativeProducer` and `./le test integration interop-rsvpte`.
   - Tests: all Linux integration tests in `rfc2205_transport_integration_linux_test.go`, `TestRSVPNativeProducer`, `./le test integration interop-rsvpte` (all 12 scenarios including `transit-wrong-incoming-interface-refused` and `ingress-non-rsvp-cloud`), and `./le rfc check`.
   - Files: `internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go`, `internal/le/interoplab/rsvpte/rsvpte.go`, `internal/le/interoplab/rsvpte/checkers.go`, `test/interop-rsvpte/scenarios/transit-wrong-incoming-interface-refused/`, `test/interop-rsvpte/scenarios/ingress-non-rsvp-cloud/`, `docs/architecture/rsvpte/mpls-rsvp-te.md`, `docs/architecture/testing/interop.md`, `rfc/short/rfc2205.md`, `plan/journal/green-that-could-not-have-been-red.md`, `rfc/discrimination/`, `rfc/audit/`
   - Verify: all native proofs pass, every discrimination break turns red, both new freeRouter scenarios fail with their fix reverted and image rebuilt and pass with the fix applied, `./le rfc check` is clean, and all 12 `interop-rsvpte` scenarios pass.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-8 has an implementation at `file:line` and a matching tagged test |
| Feature completeness | Every user story 1..5 has a working native Linux test path |
| Non-local transparency | `handlePacket` forwards non-local `Resv`/`PathErr`/`ResvErr`/`ResvTear`/fallback `ResvConf` BEFORE `DecodeMessage` and `rejectUnknownObject`, using 24-byte Router Alert (`0x46`, `0x94, 4, 0, 0`) when `hdr.MsgType == MsgTypeResvConf`, 20-byte plain IPv4 (`0x45`, no Router Alert) otherwise, and zero `RSVP_HOP` rewrite |
| Shared-LAN interop safety | Single-LAN explicit-ERO transit between distinct peers (`interop-rsvpte`) and multi-link FRR bypasses (`TestRSVPNativeProducer`) pass without false drops |
| Merge-point security | Strict `samePeer` verification stays enforced on adjacent peers, strict EROs, and FRR merge points; non-RSVP-cloud admission requires TTL decrement and route-via-gateway match |
| Netlink read-only boundary | `transport_linux.go` never writes kernel routes or labels (`ai/doors/routing-protocols.md`) |
| RFC quotes & records | Every enforcing branch quotes RFC 2205 section text and every new/changed tag carries a discrimination record (`./le rfc check` clean) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Non-local forwarding, incoming-interface check, loop/reflection guards, unicast RESV pinning, non-RSVP-cloud RESV admission | `./le test unit pkg path ./internal/plugins/rsvpte/` passes (including `rfc2205_routing_interface_test.go`) |
| Native Linux forwarding proofs (`2-11`, `2-12`, `3-6`, `3-21`, `3-23`, `3-33`, `3-37`, `3-38`, `3-39`, `3-40`, `3-42`) | Linux namespace run of `rfc2205_transport_integration_linux_test.go` and `TestRSVPNativeProducer` |
| FreeRouter interop proof and non-regression | `./le test integration interop-rsvpte` passes all 12 scenarios (`transit-wrong-incoming-interface-refused` and `ingress-non-rsvp-cloud` each proved red when reverted and rebuilt, plus all 10 existing scenarios) |
| RFC discrimination records and `RFC2205-3-40` re-judgment | `./le rfc check` clean for `rfc2205` and `RFC2205-3-40` verdict `enforced` |
| Docs and journal updated | `docs/architecture/rsvpte/mpls-rsvp-te.md`, `docs/architecture/testing/interop.md`, `rfc/short/rfc2205.md`, and 2026-10-30 row in `plan/journal/green-that-could-not-have-been-red.md` updated |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Packet spoofing / reflection amplification | Non-local forwarding drops `TTL <= 1`, local `NextHop`, `samePeer(Src, NextHop)`, and `outIfIndex == pkt.IfIndex`; malformed/short headers fail closed in `DecodeHeader` before forwarding |
| Non-RSVP-cloud RESV spoofing | Non-RSVP-cloud admission requires `!repaired`, non-strict path, `pkt.TTL > 0 && pkt.TTL < msg.Header.TTL`, `msg.Hop.NextHop == pkt.Src`, and `ResolveRoute(pkt.Src)` routing through `lsp.NextHop`; random third-party hosts cannot hijack an LSP |
| Buffer / descriptor safety | `rawTransport.Forward` validates IPv4 addresses, `rsvpHdrLen <= len(payload) <= maxRSVPPacket`, locks `sendMu`, resets `SO_MARK = 0`, and checks `t.closed` before using `pathFD` |
| Lock ordering | All `ResolveRoute` / `LocalAddresses` / transport calls run outside `lsp.mu` (under `e.control` only), preserving `lsp.locked` panic-release safety |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read source in Current Behavior and RFC 2205 full text. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Interop or native Linux test fails | Check AC and topology assumption (A-1..A-5): wrong AC → DESIGN, code defect → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- `interop-rsvpte` places all containers on one Docker bridge (`eth0`, `172.29.81.0/24`), so explicit-routed unicast transit legitimately receives from one peer on `eth0` and forwards to a distinct peer on `eth0`; loop exclusion must therefore separate implicit-route hairpins and same-peer bounces from distinct-peer explicit-ERO hops.
- `TestRSVPNativeProducer` connects `H` and `M` over three parallel links while routing `H`'s router-id over only one link, so explicit-routed PATH incoming-interface validation must query `RSVP_HOP` rather than `SenderAddr`.

## Owned journal rows
<!-- Every open journal row this spec will fix, one line each; `./le commit create`
     refuses an open row no spec lists (plan/journal/README.md, "Owned rows"). -->
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `green-that-could-not-have-been-red.md` | 2026-10-30 | `TestIntegrationRSVPPathCarriage` cannot tell the IP TTL from Send_TTL (RFC2205-3-40) |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Fix `RFC2205-2-12`, `3-6`, and `3-21` as shipped defects in this spec (owner decision, 2026-10-08) | Defer to post-release or separate gap specs | Unmet MUSTs in implemented RSVP are defects under `ai/rules/rfc-compliance.md`. |
| Decode only the 8-byte common header before checking non-local `Packet.Dst` for `RFC2205-2-12` | Run full `DecodeMessage` before forwarding | RFC 2205 §2.9 requires forwarding non-local non-Path/PathTear messages without further processing; full decoding would reject unknown `0bbbbbbb` classes or unsupported styles meant for another router. |
| Add `Transport.Forward` on `pathFD` with `routerAlert bool` (24-byte `0x46` Router Alert header when `hdr.MsgType == MsgTypeResvConf`, 20-byte `0x45` plain IPv4 header otherwise, unmodified payload) | Reuse `SendPath` or `Send`, or omit Router Alert on fallback `ResvConf` | `SendPath` always inserts Router Alert (prohibited on `Resv`/`PathErr`/`ResvErr`/`ResvTear` by §3.3) and rewrites `RSVP_HOP`, while omitting Router Alert on fallback `ResvConf` would violate §3.3 and `docs/architecture/rsvpte/mpls-rsvp-te.md` (`Dataplane`). |
| Resolve `RFC2205-3-6` expected incoming interface against `SenderAddr` when `!HasERO` and `RSVP_HOP` when `HasERO` | Always query `SenderAddr` | Preserves multi-link explicit-routed tunnels and FRR bypasses (`TestRSVPNativeProducer`) while still blocking wrong-interface arrivals on both implicit and explicit paths. |
| Distinguish implicit-route hairpins and same-peer bounces from distinct-peer shared-LAN explicit-ERO transit for `RFC2205-3-21` | Unconditionally drop `outIfIndex == pkt.IfIndex` | Preserves all 9 single-LAN `interop-rsvpte` freeRouter scenarios while blocking broadcast-LAN reflections, same-peer bounces, implicit hairpins, and merged-branch arrival-interface refreshes. |
| Gate non-RSVP-cloud RESV acceptance (`RFC2205-2-11`) on `!repaired`, non-strict path, `Packet.TTL < Send_TTL`, and `ResolveRoute(src)` traversing `lsp.NextHop`, keeping adjacent gateway for FIB | Relax `samePeer` unconditionally | Preserves strict OSPF/IS-IS merge-point source verification (`mpls-rsvp-te-fast-reroute.md`) and prevents spoofed RESV hijacking while enabling genuine non-RSVP-cloud operation. |

## Known Limitations

- General multicast / local-application interface selection for `RFC2205-3-33` (`Iapp` vs `Isp`, `Local_only` path state) is owned by `plan/post-release/features/spec-rsvp-multicast-interface-selection.md`.
- Non-RSVP-hop detection and traffic-control `NonRSVP` flag reporting (`RFC2205-3-30`) is owned by `plan/release/fixes/spec-rsvpte-non-rsvp-neighbors.md`, and IP-tunnel/non-RSVP-neighbor suppression (`RFC2205-3-31`, `RFC3209-4.2.5-1`) is split to post-release.
- Multicast Wildcard-Filter `SCOPE`-object loop avoidance is owned by `plan/post-release/features/spec-rsvp-multicast-wildcard.md`.
- Raw IP protocol-46 socket transport is Linux-only (`transport_other.go` returns `errTransportUnsupported` on non-Linux hosts per `ai/rules/platform-linux.md`).

## RFC Documentation (Scope: protocol)

Every enforcing branch carries `// RFC 2205 Section X.Y: "<quoted requirement>"` above the code. Default `rsvp-te` configuration is compliant; explicit ERO hops and FRR bypass configurations remain preserved and tested alongside default implicit/unicast behavior.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 2205 §2.9 (`RFC2205-2-11`) | Default `rsvp-te` signals PATH/RESV/PathTear across a non-RSVP IPv4 forwarder using `RSVP_HOP` and TTL-decremented gateway-routed RESV admission | Strict ERO hops and FRR merge-point strict `samePeer` verification (`fast-reroute`, `bypass`) | `TestRFC2205NonRSVPCloudSignaling`, `TestIntegrationRSVPNonRSVPCloud`, `ingress-non-rsvp-cloud` | `resv_control_source_test.go`, `TestRSVPNativeProducer` |
| RFC 2205 §2.9 (`RFC2205-2-12`) | Default `rsvp-te` forwards non-local non-Path/PathTear messages byte-for-byte unmodified (with Router Alert for fallback `ResvConf`, without Router Alert otherwise) and without local processing | FRR `ResvConf` bypass translation via `handleResvConf` (`fast-reroute`) | `TestRFC2205NonLocalMessagesForwardedUnmodified`, `TestIntegrationRSVPNonLocalForwarding` | `TestMergePointRestoresConfirmationIdentity`, `TestRSVPNativeProducer` |
| RFC 2205 §3.1.3 (`RFC2205-3-6`) | Default transit PATH is not forwarded if arrival `IfIndex` disagrees with routing's expected incoming interface | Explicit multi-link EROs and FRR bypasses (`explicit-route`, `bypass`) validated against `RSVP_HOP` | `TestRFC2205PathWrongIncomingInterfaceNotForwarded`, `TestIntegrationRSVPIncomingInterfaceAndLoopGuards`, `transit-wrong-incoming-interface-refused` | `TestRSVPNativeProducer`, `./le test integration interop-rsvpte` |
| RFC 2205 §3.3, §3.4 (`RFC2205-3-21`, `RFC2205-3-23`) | Default immediate forwarding excludes arrival interface/peer and rejects local implicit next hops, RRO self-loops, and `Send_TTL <= 1` | Single-LAN explicit-ERO transit between distinct peers (`interop-rsvpte`), RFC 3209 §4.4.3 RRO-withdrawal RESV update | `TestRFC2205ImmediateForwardExcludesArrivalInterface`, `TestRFC2205ForwardingAvoidsLoops`, `TestIntegrationRSVPIncomingInterfaceAndLoopGuards` | `./le test integration interop-rsvpte`, `refresh_rro_test.go` |
| RFC 2205 §3.9 (`RFC2205-3-33` unicast) | Default unicast RESV uses the PATH's recorded arrival interface (`InIfIndex`) via `OifIndex` pinning | Fallback to `RouteGet` on asymmetric return paths or unit fixtures with `InIfIndex == 0` | `TestRFC2205UnicastResvUsesPathIncomingInterface`, `TestIntegrationRSVPResvPinsPathInterface` | Existing `rfc2205_engine_test.go`, `interop_test.go` |
| RFC 2205 §3.11.4, §3.11.5 (`RFC2205-3-37`, `3-38`, `3-39`, `3-40`, `3-42`) | Native netlink route queries, active real+virtual interface discovery, wildcard `IP_ROUTER_ALERT` diversion, arrived IP TTL distinct from `Send_TTL`, and `IP_HDRINCL` source/`Send_TTL` carriage | Bypass mark-scoped `SO_MARK` forwarding (`bypass`) | `TestIntegrationRSVPPathCarriage`, `TestIntegrationRSVPPathAvoidsDataFEC`, `TestIntegrationRSVPLocalAddressesRealAndVirtual` | `TestIntegrationRSVPSelectedBypass`, `TestRSVPNativeProducer` |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le test unit pkg path ./internal/plugins/rsvpte/`, the Linux namespace integration tests in `rfc2205_transport_integration_linux_test.go`, `TestRSVPNativeProducer`, `./le test integration interop-rsvpte`, and `./le rfc check`; GitHub CI verifies the pushed commit
- [ ] Feature code integrated (`internal/plugins/rsvpte/`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named in `## Work Not Done`, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (`TTL`, `IfIndex`)
- [ ] Native Linux integration tests for end-to-end forwarding behavior
- [ ] Interop tests (`./le test integration interop-rsvpte`) pass

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] `./le rfc check` clean with all discrimination records and `RFC2205-3-40` re-judgment landed
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + closed journal row + RFC records
- [ ] **Commit B:** `remove plan/release/fixes/spec-rsvp-routing-and-interface-integration.md` only, in the same `./le commit create` script
