# Spec: bgp-link-local-only-next-hop

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | plan/immediate/spec-bgp-unowned-rfc-rows.md (row 3: the link-scope snapshot gains the name of the session's interface, its A-2 decision, which AC-9 to AC-17 read; its I4 runtime check of `session link-local`, which AC-14 reuses; and its scenario `bgp-linklocal-capability-interface-frr`, which proves AC-8's receive direction) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Prove what is left of `draft-ietf-idr-linklocal-capability` once the code that
performs it exists, and give Ze an IPv6 next hop of its own on an IPv4 session,
taken from the session's interface the way FRR and BIRD take it:

| Part | What |
|------|------|
| AC-7 | Re-read three rows that used to rest on an unreachable form |
| AC-8 | Interop: Ze relays a received link-local-only next hop unchanged to FRR |
| AC-9 to AC-17 | Build: under next-hop self, an IPv6 route over an IPv4 session is sent with the session interface's global IPv6 address (plus its link-local for a directly attached peer, whether or not capability 77 is negotiated), or with the interface link-local alone when the peer is directly attached, the interface holds no global and capability 77 is negotiated, instead of being withheld; a multihop session whose source interface holds no global is withheld; addresses come from the interface with no config; `session link-local` stays as an explicit override, and capability 77 no longer requires it (owner, 2026-10-11, "match bird and frr behaviour"; owner, 2026-10-12, the two cases where FRR and BIRD differ, Owner decisions) |

Owner decision, 2026-10-11, on the round-2 design review's I5 ("the narrowed
skeleton owns no open row"): "Rewrite to AC-7/AC-8". This spec was written on
2026-09-01, when Ze advertised capability 77 and performed none of the draft.
That is no longer true, and the earlier text claimed rows the ledger shows met:

| Earlier claim | What the source and the ledger say now |
|---------------|----------------------------------------|
| `parseCapability` holds no case for 77 and no negotiated state is recorded | `internal/core/bgp/capability/capability.go` declares `CodeLinkLocalNextHop` (77) and parses it as a typed `LinkLocalNextHop`; `Negotiate` in `internal/core/bgp/capability/negotiated.go` records the negotiated `LinkLocalNextHop` |
| Seven MUST-level rows (`-3-1`, `-4-1`, `-4-3`, `-4-4`, `-4-7`, `-4-9`, `-4-5`, `-4-6`) have no producer | `rfc/short/draft-ietf-idr-linklocal-capability.md` "Enrolment reason" records each as met and names its producer: `Peer.linkLocalOnlyNextHopPermitted` and `buildMPReach` for `-3-1` and `-5-1`; `egressNextHopIsLinkLocalOnly` and `sameLinkLayerSegment` for `-4-5`/`-4-6`; a route whose next hop has no wire form refused for `-4-1`/`-4-4`/`-4-7`/`-4-9` |
| The draft's open rows are this spec's | The summary carries `{gap}` on two rows only, `-1-2` and `-6-1`, and both are owned by `plan/immediate/spec-bgp-unowned-rfc-rows.md` (its rows 3 and 4) |

So the ACs that built the feature (the former AC-1 to AC-6 and AC-9) are
dropped as already met by other work. The round-5 design review (I1) found that
Ze already sends its own link-local alone over a link-local transport, and the
owner chose to build the case that is really missing ("Build the missing
case"). The round-6 review (B1, I1) then showed that case was unreachable as
designed: capability 77 is refused at config without `session link-local`, and
on an IPv4 session an interface global gives no next hop at all. The owner
answered both by asking Ze to match FRR and BIRD (Key Design Decisions, "Match
FRR and BIRD"). AC-9 to AC-17 are that build, numbered after AC-8.

### Where Ze stands today on an IPv6 next hop of its own

| Session transport | Next-hop self resolves to | What reaches the wire today |
|-------------------|---------------------------|-----------------------------|
| Link-local IPv6 (the TCP local endpoint is `fe80::`, for example a session a peer opened to Ze's link-local address) | that endpoint (`connectedLocalAddress`, `session_connection.go`) | with capability 77 negotiated, the 16-octet Link-Local-only Next Hop: `canUseNextHopFor` admits it for IPv6 unicast and `linkLocalOnlyNextHopRefused` (`peer.go`) lets it through; without 77 the route is withheld. `TestNextHopSelfLinkLocalEndpointNeedsTheCapabilityOnTheForwardRails` (`rfc_draft_linklocal_nhself_test.go`) asserts both |
| IPv4, IPv6 NLRI | the IPv4 endpoint | announce rail: `canUseNextHopFor` refuses an IPv4 next hop for IPv6 unicast unless an Extended Next Hop pair admits it, so `resolveNextHop` returns `ErrNextHopIncompatible`. Forward rails: `precomputeNextHop` sets `nhModeSelf4`, `applyFactsNextHop` writes the IPv4-mapped `nhMapped` into MP_REACH, and `egressNextHopWithheld` withholds it as `withholdUnusableGlobal`. Whatever the interface holds, global or link-local, the route is withheld: this is the missing case |
| Global IPv6 | the global endpoint | the global, plus the `session link-local` leaf as the second address when RFC 2545 Section 3's shared-subnet condition holds (`linkScope.linkLocalNextHop`); never the link-local alone, and never a second address when the leaf is unset |

`reactor_peers.go` refuses a link-local `local-address` ("link-local addresses
not supported for local-address"); it does not refuse a session whose local
endpoint turns out to be link-local, which is the first row.

`refuseLinkLocalCapabilityWithoutAddress` (`internal/component/bgp/plugins/llnh/llnh.go`)
refuses, at commit and at startup, every peer that enables capability 77 with no
`session link-local` on the peer or its group. Its comment records the design
this spec reverses: "Deriving the address from the interface instead would make
the next hop depend on runtime state the configuration does not show."

## Required Reading

- `rfc/short/draft-ietf-idr-linklocal-capability.md` and
  `rfc/drafts/draft-ietf-idr-linklocal-capability.txt` (revision -06)
  → Constraint: the draft text is the authority; the summary is derived. Rows
    `-1-2` and `-6-1` are not this spec's.
- `rfc/drafts/draft-ietf-idr-linklocal-capability.txt` §1 (lines 144-153), §4 (lines 250-398) and §5 (lines 400-415)
  → Constraint: §1: "BGP speakers SHOULD NOT advertise a route whose Next Hop is a Link-Local address that is in the tentative state (Section 5.4 of [RFC4862]); this applies both to a first-party Next Hop (the speaker's own Link-Local address) and to a third-party Next Hop re-advertised from another peer." (`-1-1`)
  → Constraint: §4 opening: "The next hop MAY consist of only a Link-Local IPv6 next hop." and "If, after completing these procedures, there are no IPv6 next hop addresses included in the next hop, the BGP route MUST not be advertised to its peer."
  → Constraint: §4 item 1 (internal peer): "When announcing a locally-originated route to an internal peer, or the BGP speaker has been explicitly configured to announce its own IP address as the next hop, the BGP speaker SHOULD use the Global IPv6 address of the interface of the router through which the announced network is reachable for the speaker as the next hop, if present. If the route is directly connected to the speaker, or if the interface address of the router through which the announced network is reachable for the speaker is the internal peer's address, the next hop MUST include its own Link-Local IPv6 address." (`-4-3`; the draft prints "Link- Local" across a line break, and code comments quote it as printed, round-6 N5)
  → Constraint: §4 item 2, default arm (external peer one IP hop away): "By default (if none of the above conditions apply), the BGP speaker SHOULD use the IP address of the interface that the speaker uses to establish the BGP connection to peer X in the next hop attribute. The Global IPv6 address, if one is present, SHOULD be included. If the BGP speaker is one IP hop away, the Link-Local IPv6 address SHOULD be included, and MAY be the only next hop address in the next hop." (`-4-12`)
  → Constraint: §4 item 3 (multihop external peer): "Link-Local IPv6 next hops MUST NOT be included." (`-4-8`), "By default, the BGP speaker SHOULD use the Global IPv6 address of the interface that the speaker uses in the next hop to establish the BGP connection to peer X." (`-4-14`), and "If a Global IPv6 next hop is not included, the route MUST NOT be advertised to the external peer (treat-as-withdraw)." (`-4-9`)
  → Constraint: §5: "When both the Link-Local Next Hop Capability defined in this document and the Extended Next Hop Encoding capability ([RFC8950]) have been negotiated between two peers, a 16-octet Link-Local-only Next Hop is permitted for IPv4 NLRI carried with an IPv6 Next Hop, provided the receiving peer is directly attached."
- `rfc/full/rfc2545.txt` Section 3
  → Constraint: "A BGP speaker shall advertise to its peer in the Network Address of Next Hop field the global IPv6 address of the next hop, potentially followed by the link-local IPv6 address of the next hop." Without capability 77 a link-local never goes alone, so Ze does not copy FRR's "link-local in both halves" form (Key Design Decisions).
- FRR `bgpd/bgp_zebra.c` (checkout cd39d029, `/Users/thomas/Code/github.com/FRRouting/frr`): `if_get_ipv6_global` (line 689), `if_get_ipv6_local` (line 706), `bgp_zebra_nexthop_set` (lines 740-845); `bgpd/bgp_route.c` lines 2516-2534; `bgpd/bgp_updgrp_packet.c` lines 449-452 and 549-558; `zebra/if_netlink.c` lines 1154-1163
  → Decision: on an IPv4 session FRR finds the interface by `if_lookup_by_ipv4_exact` of the local address (or `update-source`), whatever the hop count. With a global on it, `v6_global` is the FIRST non-link-local IPv6 address in the interface's connected list and `v6_local` the first link-local. With none, the link-local fills both ("the expectation for interop is that the network admin would use a route-map to specify the global IPv6 nexthop"). `peer->shared_network` is set when the peer's IPv4 address is on a connected interface. Zebra drops an address while it is `IFA_F_TENTATIVE` or `IFA_F_DADFAILED` ("Only consider valid addresses").
  → Constraint: on a shared network with capability 77 negotiated, FRR sends the 16-octet field and overwrites it with `v6_local` (`ll_nexthop_only`), so FRR sends the link-local alone EVEN WHEN a global exists. Without 77 it sends global plus link-local (32 octets). On a multihop IPv4 session with no global FRR writes the link-local as the global, which §4 item 3 forbids.
- BIRD `proto/bgp/bgp.c` (checkout 02d082a7, `/Users/thomas/Code/gitlab.nic.cz/labs/bird`): `bgp_start_neighbor` (lines 2058-2070), `bgp_start_channel` (lines 2683-2730); `proto/bgp/packets.c` line 1303; `nest/iface.c` `if_recalc_preferred` (lines 497-540); `sysdep/linux/netlink.c` lines 1347-1352
  → Decision: on an IPv4 session for an IPv6 channel the next hop is `iface->addr6`, the interface's preferred global: the first non-secondary non-link-local IPv6 address in the list unless one is configured as preferred, kept stable once chosen. The interface is the neighbor's interface for a single-hop session and, for multihop, `neigh_find` of the session's SOURCE address, which is the interface holding it. The link-local (`iface->llv6`, chosen the same way) is added only for a single-hop session (`p->neigh`). With no global BIRD logs "Missing next hop address" and sends nothing; with a global and no link-local it logs "Missing link-local address" and sends the global alone. Netlink drops tentative addresses ("Ignore tentative addresses silently") and marks `IFA_F_SECONDARY` (on IPv6, the temporary-address flag) so it is never preferred. BIRD does not implement capability 77.
- `internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_send_test.go`,
  `draft_ietf_idr_linklocal_capability_test.go`,
  `draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go`,
  `draft_ietf_idr_linklocal_capability_multihop_test.go`
  → Constraint: these already carry `RFC requirement:` tags for `-5-1`, `-4-2`
    and `-4-8`, and `rfc/discrimination/draft-ietf-idr-linklocal-capability.json`
    holds records for all three (grep, 2026-10-11). `-4-2` carries positive and
    negative tags (`draft_ietf_idr_linklocal_capability_test.go` lines 73 and 84;
    `draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go` lines 155 and
    158). AC-7 re-reads them; it writes new tests only where a re-read finds one
    that does not drive the reachable form.
- `internal/component/bgp/plugins/llnh/llnh.go` (`refuseLinkLocalCapabilityWithoutAddress`, `refuseLinkLocalCapabilityWithoutAddressSections`, wired in `OnConfigVerify` and `OnConfigure`), `register.go` (its comment at line 33), `llnh_test.go` (line 315), `draft_ietf_idr_linklocal_capability_llnh_refusal_test.go` (`-4-3` positive and negative tags at lines 75 and 104), `test/plugin/llnh-refuses-without-link-local.ci`
  → Decision: the refusal is removed (AC-15); its `-4-3` tags and discrimination records move to the reactor test that proves the interface-derived link-local (AC-11), because the refusal no longer produces `-4-3`.
- `plan/immediate/spec-bgp-update-propagation-rfc-defects.md`, D6
  → Constraint: D6 is fixed. Its row reads "Fixed, including mixed post-policy
    output and section-specific withdrawals ... D6 implementation and independent
    round 3 review complete" (round-3 I1, read 2026-10-11). The 2026-09-27 record
    of `-4-2` as unmet is superseded, so AC-7 re-reads `-4-2` like the other two
    rows.
- `docs/contributing/rfc-conformance-gates.md`
  → Constraint: a `{gap}` row with a positive or negative tag is refused as a
    stale annotation, and an author-written `{gap}` is not an escape from the
    coverage ratchet, so a met row with tags is never moved to `{gap}` to clear a
    finding.
- `docs/architecture/testing/interop.md`
  → Constraint: each scenario is named, owes a recorded red with the fix reverted
    and the artifact rebuilt.
- `test/interop/scenarios/bgp-linklocal-only-multihop-withdraw-frr/`
  → Decision: AC-8's and AC-10's sessions run over IPv4, as this scenario's does
    (`frr.conf` `neighbor 172.30.0.2 capability link-local`). Ze refuses a
    link-local `local-address` (`reactor_peers.go`), and the missing case is the
    IPv4 transport.
  → Decision: AC-8 reuses this scenario's injector (`inject.msg`, `inject-args`,
    a `peer injector` Ze dials with `accept false`), placed on the SAME link as
    Ze and FRR, so the relayed link-local is a third-party next hop FRR can use
    (round-4 I1).
  → Constraint: its `frr.conf` notes "FRR also needs a Global IPv6 address on the
    interface the session leaves from, or it has no IPv6 next hop of its own and
    ends the session with a Cease." That was a multihop session whose
    update-source was `lo`, which has no link-local (round-6 N1).
- `internal/le/interoplab/bgp/prepare.go`
  → Constraint: `ipv6Sysctls` (line 465) enables IPv6 on the lab containers, so
    FRR's eth has a link-local; `peerFRRSink` starts `frr-sink.conf` at host
    `extendedSinkHost` (line 410), the second FRR AC-11 needs on the same link
    (round-6 N2); `@ZE_LINK_LOCAL@` (line 562) renders and assigns the leaf
    address on Ze's eth0 (round-6 N3).
- [ ] `internal/core/bgp/attribute/nexthop_form.go` (`ValidateGlobalNextHop`),
  `internal/component/bgp/reactor/session_connection.go` (`connectedLocalAddress`),
  `internal/component/bgp/reactor/peer.go` (`resolveNextHop`, `canUseNextHopFor`,
  `linkLocalOnlyNextHopPermitted`, `linkLocalOnlyNextHopRefused`),
  `internal/component/bgp/reactor/peer_forward_facts.go` (`precomputeNextHop`, `buildForwardFacts`, `applyFactsNextHop`, called from `forward_rs.go` and `reactor_api_forward.go`),
  `internal/component/bgp/reactor/forward_next_hop.go` (`egressNextHopWithheld`, `egressNextHopLinkLocalOnlyRefused`, `withholdLinkLocalOnlyRefused`, `withholdUnusableGlobal`),
  `internal/component/bgp/reactor/link_scope.go` (`linkScope`, `linkLocalNextHop`, `applyLinkLocalNextHop`, `linkLocalNextHopFor`)
  → Constraint: `connectedLocalAddress` takes no family, and `precomputeNextHop`
    settles one per-peer mode with no family, so no helper both rails share can
    hold a per-family arm (round-6 I2). `linkLocalNextHop` adds the leaf only
    when one connected prefix holds both the peer and the global, which an IPv4
    peer and an IPv6 global never satisfy.
  → Decision: one new function answers "Ze's own IPv6 next hop for this peer, or
    the cause it has none" from the snapshot and the leaf. The announce rail
    calls it from `resolveNextHop`'s NextHopSelf arm when `canUseNextHopFor`
    refuses the endpoint for the family; the forward rails call it from
    `buildForwardFacts` and store the answer in a new facts field, which
    `applyFactsNextHop` writes as the MP_REACH next hop for an IPv6-AFI
    `mpFamily` under `nhModeSelf4` in place of `nhMapped`. The second address
    `linkLocalNextHop` appends takes its link-local from the same answer.
- [ ] `internal/component/bgp/reactor/reactor_iface.go` (`refreshPeerLinkScopes`)
  → Constraint: it skips a peer when `slices.Equal(scope.connected, connected)`,
    and `connected` holds MASKED prefixes with no interface name; every
    link-local masks to `fe80::/64`, so a link-local moved between interfaces
    or a global swapped within a prefix leaves the table equal (round-6 I3).
- [ ] `plan/immediate/spec-bgp-unowned-rfc-rows.md` A-2 decision and I4
  → Constraint: the link-scope snapshot gains "the NAME of the interface whose
    connected prefix contains the peer, read at the same cadence as today
    (session establishment and reload, never per UPDATE, from `net.Interfaces`
    with each interface's addresses)"; a peer no single interface reaches has no
    interface. I4: the configured `session link-local` must be an address of
    that interface, re-checked after an interface change, or Ze withholds it
    with a Warn. As corrected on 2026-10-12, its I4 and YANG-validator rows make
    the `fe80::/10` commit check its own function beside
    `refuseLinkLocalCapabilityWithoutAddress`; AC-15 removes that function's
    capability-77 refusal and keeps the `fe80::/10` check. Its I4 Warn is built
    there as the one producer AC-13 extends, with consequence "carry the global
    next hop without a link-local".
- [ ] `internal/component/iface/iface.go` (`AddrInfo`: `LinkLocal`, `Tentative`, `Origin`) and `internal/component/iface/resolve.go` (`Addresses`)
  → Constraint: `net.Interfaces` returns no address flags, so it cannot tell a
    tentative or temporary address. `iface.Addresses` returns each address with
    `Tentative` (kernel `IFA_F_TENTATIVE`, which a DAD-failed address keeps) and
    `Origin` ("temporary" for `IFA_F_TEMPORARY`), and is already read by
    `internal/plugins/ospf/v3/transport/backend_linux.go` and
    `internal/component/bgp/plugins/healthcheck/ip.go`. It translates a logical
    name through the os-name selector and errors when no backend is loaded (R-5).
  → Constraint: the iface monitor (`internal/plugins/iface/netlink/monitor_linux.go`,
    `handleAddrUpdate`) drops an IPv6 address event while the address is
    tentative, so the address-added event, and with it `refreshPeerLinkScopes`,
    fires when DAD completes.

## Current Behavior (MANDATORY)

Source files read:

- [ ] `internal/core/bgp/capability/capability.go` - `CodeLinkLocalNextHop`, the typed `LinkLocalNextHop` case
- [ ] `internal/core/bgp/capability/negotiated.go` - `Negotiate`, the `LinkLocalNextHop` field
- [ ] `internal/component/bgp/reactor/peer.go` - `resolveNextHop` (next-hop self: `connectedLocalAddress`, then `canUseNextHopFor`, then `linkLocalOnlyNextHopRefused`), `linkLocalOnlyNextHopPermitted`, `canUseNextHopFor` (natural match, then the Extended Next Hop pairs)
- [ ] `internal/component/bgp/reactor/session_connection.go` - `connectedLocalAddress`: the TCP local endpoint, zero for unspecified or multicast
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - `precomputeNextHop` (IPv4 endpoint to `nhModeSelf4` with `nhLegacy` and `nhMapped`), `applyFactsNextHop` (writes `nhMapped` into MP_REACH for a family with no 4-octet form)
- [ ] `internal/component/bgp/reactor/forward_next_hop.go` - `egressNextHopWithheld` (`withholdUnusableGlobal` for the mapped field, `withholdLinkLocalOnlyRefused` for a link-local-only field without 77), `egressNextHopIsLinkLocalOnly`
- [ ] `internal/component/bgp/reactor/link_scope.go` - `linkScope` (connected prefixes and `peerOnLink`), `linkLocalNextHop` (the leaf as the second address only when the speaker is the next hop's router and the global shares a connected prefix with the peer), `applyLinkLocalNextHop` (`nhModeSelfV6` to `nhModeSelfV6LL`)
- [ ] `internal/component/bgp/reactor/reactor_iface.go` - `refreshPeerLinkScopes` and its masked-table shortcut
- [ ] `internal/component/bgp/message/update_build.go` - `buildMPReach`, the 16-octet and 32-octet forms
- [ ] `internal/component/bgp/reactor/reactor_peers.go` - the refusal of a link-local `local-address`
- [ ] `internal/component/bgp/plugins/llnh/llnh.go` - `refuseLinkLocalCapabilityWithoutAddress`
- [ ] `internal/component/bgp/reactor/rfc_draft_linklocal_nhself_test.go` - `TestNextHopSelfLinkLocalEndpointNeedsTheCapabilityOnTheForwardRails`

Capability 77 is advertised, parsed, negotiated and acted on for the send side
and the reflection path, and is refused at config without `session link-local`.
Ze's only link-local of its own is that leaf. Over an IPv4 transport every IPv6
route under next-hop self is withheld, whatever the session interface holds, so
`-4-3`, `-4-12` and `-4-14` are not met there. Over a global-IPv6 transport
without the leaf the global goes alone, where RFC 2545 Section 3 and `-4-12`
want the link-local after it. `test/interop/scenarios/bgp-linklocal-only-multihop-withdraw-frr/`
covers the multihop withdraw. No scenario shows another implementation
installing an IPv6 next hop Ze takes from its interface on an IPv4 session, or a
16-octet link-local Ze sends on a shared link, relayed or its own.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

An operator peers over IPv4 and carries IPv6 unicast with next-hop self: config
`update` blocks or the API (announce rail), or a route received from another
peer and forwarded (forward rails). Capability 77 (`link-local-nexthop`) is
optional and only decides the no-global case. For AC-8 Ze relays with
`next-hop unchanged`.

### Transformation Path

1. Session establishment, reload, and an interface address change: the link-scope snapshot (unowned-rows A-2) records the session interface by name, and, new here, which interface that is when the peer is in no connected prefix: the one holding the session's local endpoint address. For that interface it keeps the usable IPv6 addresses in kernel order from `iface.Addresses`, dropping a `Tentative` address and an address whose `Origin` is "temporary", and a one-hop flag (the peer is in a connected prefix of that interface).
2. `ownIPv6NextHop` (new, `link_scope.go`) answers from the snapshot and the `session link-local` leaf: the global (the first remaining address that is not link-local), the link-local (the leaf when set and held, else the first remaining `fe80::/10` address), and the one-hop flag; or a cause when it cannot (Key Design Decisions, "The answer").
3. Announce rail: `resolveNextHop`'s NextHopSelf arm, when `canUseNextHopFor` refuses the endpoint for this family and the family's AFI is IPv6, takes the global from `ownIPv6NextHop`, or the link-local alone when one hop and no global. `linkLocalOnlyNextHopRefused` then stays the one capability test, and `linkLocalNextHopFor` appends the link-local after a global for a one-hop peer.
4. Forward rails: `buildForwardFacts` stores the same answer in a new facts field; `applyFactsNextHop` under `nhModeSelf4` writes it as the MP_REACH next hop (16 or 32 octets) for an IPv6-AFI `mpFamily`, and keeps `nhLegacy` and `nhMapped` for every other family. `egressNextHopLinkLocalOnlyRefused` stays the forward rails' capability test.
5. Global-IPv6 transport: `linkLocalNextHop` takes its link-local from `ownIPv6NextHop` instead of the leaf alone, so a session with no leaf gains the interface link-local as the RFC 2545 second address.
6. `buildMPReach` writes the 16-octet or 32-octet form.
7. Receive (AC-8 relay and the FRR side): MP_REACH_NLRI with a 16-octet link-local, carried with the session interface to the FIB, which the unowned-rows spec row 3 builds and its scenario `bgp-linklocal-capability-interface-frr` proves.

### Boundaries Crossed

The host interface table and the iface component's address flags (read into the
snapshot, never per UPDATE), the config verifier in the llnh plugin, and the BGP
wire against FRR and BIRD.

### Integration Points

| Point | What |
|-------|------|
| `linkScope` (`link_scope.go`) | the session interface (one-hop or source), its usable IPv6 addresses, the one-hop flag, and `ownIPv6NextHop` |
| `resolveNextHop` (`peer.go`) | the announce rail's call, per family |
| `buildForwardFacts`, `applyFactsNextHop` (`peer_forward_facts.go`) | the forward rails' call and the new facts field |
| `refreshPeerLinkScopes` (`reactor_iface.go`) | rebuilds on any change of the per-peer inputs |
| `refuseLinkLocalCapabilityWithoutAddress` (`llnh.go`) | the capability-77 refusal removed; the `fe80::/10` leaf check kept |
| `interoplab` checkers | `bgp-linklocal-only-next-hop-frr` (AC-8), `bgp-ipv4-session-ipv6-global-next-hop-frr` and `-bird` (AC-9), `bgp-linklocal-own-next-hop-frr` (AC-10, AC-11) |

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | FRR implements the draft's capability 77 and the 16-octet form | FRR source: `bgpd/bgp_attr.c` (`PEER_HAS_LINK_LOCAL_CAPABILITY`), `bgp_updgrp_packet.c` `ll_nexthop_only`; the existing `bgp-linklocal-only-multihop-withdraw-frr` scenario negotiates 77 between FRR 10.4.1 and Ze | AC-8, AC-10 and AC-11 have no peer, which `ai/rules/interop-and-goal-validation.md` refuses; the owner is asked which implementation to use | the existing scenario's negotiation | holds |
| A-2 | The existing tagged tests for `-5-1`, `-4-2`, `-4-8` drive the form Ze now emits | tags and discrimination records exist (grep, 2026-10-11) | a test that passes because the form is unreachable is vacuous; AC-7 replaces it | AC-7 | unvalidated |
| A-3 | FRR keeps an IPv4 single-hop session up with IPv6 unicast and capability 77 on a link with no global IPv6 address | FRR source (round-6 N1): `bgp_zebra_nexthop_set` copies the interface link-local into `v6_global` when no global exists, and `bgp_open_receive` Ceases only when `v6_global` is unspecified; `ipv6Sysctls` gives FRR's eth a link-local; the multihop Cease came from update-source `lo` | the no-global scenarios cannot run; the owner is told with the FRR log | FRR source; the bring-up in step 2 confirms | holds |
| A-4 | The lab can put a global IPv6 address on the shared link for Ze, FRR and BIRD while the sessions stay on IPv4 | `bgp-ipv6-ebgp-bird` already runs an IPv6 network in the lab | AC-9's scenarios need a lab change first; it is made in this spec, not deferred | step 2 | unvalidated |
| A-5 | BIRD (the lab's image) installs an IPv6 route received over an IPv4 session with a 32-octet global plus link-local next hop | BIRD's own sender emits that form (`packets.c` line 1303) and its receiver decodes it | AC-9's BIRD scenario cannot pass; the owner is told with BIRD's log | step 2 | unvalidated |

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | The `-4-2` re-read finds that a received link-local forwarded under next-hop auto or unchanged reaches a peer more than one hop away again, a regression of D6 | it is a defect in an implemented capability and is fixed in this spec (`ai/rules/completion.md`): the producer changes, the tagged tests go red on the regression and green on the fix, and the row stays met; the ledger is never moved to `{gap}` to absorb it |
| R-2 | AC-8's receive direction rests on row 3 of the unowned-rows spec | the receive direction is proven by that spec's scenario `bgp-linklocal-capability-interface-frr`, and this spec's scenarios assert the send direction only |
| R-3 | AC-9 to AC-17 read the interface name the unowned-rows spec adds to `linkScope`, and AC-15 changes a function that spec's I4 text names | Depends names it; if that spec has not landed, this one stops before AC-9 and reports, rather than building a second interface read. The unowned-rows spec's I4, AC-3, YANG-validator and llnh rows were corrected on 2026-10-12 to match this design: the `fe80::/10` check is its own function, and the I4 Warn is the producer AC-13 extends |
| R-4 | Routes sent today change their next hop: a global-IPv6 session with no leaf gains the interface link-local as the second address (AC-16), and a peer that negotiated 77 without the leaf, refused today, now comes up | that is the owner's decision ("addresses come from the interface automatically"); the IPv4-transport arm still fires only where `canUseNextHopFor` refuses the endpoint, so on that transport only withheld routes change. AC-16 lists every case that must not change |
| R-5 | `iface.Addresses` has no backend in a build that runs the reactor, or takes a logical name the snapshot (which holds the kernel name) cannot give it | the first implementation step confirms both: the backend loads in every build tag that carries the reactor, and the resolver accepts the kernel name or has an entry that does. If either fails, stop and report; a read error is never treated as "no address" (AC-13, cause "address read failed") |
| R-6 | Deleting the llnh refusal deletes tagged tests | the behaviour they assert is reversed by the owner's decision; each tag moves to the test that proves the new producer (AC-11, AC-15), and the records are re-observed, never copied |

## Blast Radius

| Area | Change |
|------|--------|
| `link_scope.go` | the snapshot keeps the session interface (one-hop or source), its usable IPv6 addresses and the one-hop flag; `ownIPv6NextHop`; `linkLocalNextHop` takes its link-local from it |
| `peer.go` `resolveNextHop` | the per-family arm |
| `peer_forward_facts.go` | a facts field, `buildForwardFacts`, `applyFactsNextHop` |
| `reactor_iface.go` | the refresh shortcut compares the per-peer inputs |
| `llnh.go`, `register.go`, YANG description | the capability-77 refusal removed |
| tests and the ledger | AC-7 re-read; new unit tests; four interop scenarios; the llnh refusal tests and `.ci` replaced; summary rows and the Support coverage line |
| `forward_next_hop.go` | only if the `-4-2` re-read finds a regression (R-1) |

No wire-format change. Config: no new leaf; capability 77 stops requiring
`session link-local`.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An operator sets `link-local-nexthop` on a peer, the peer advertises capability 77, Ze relays with `next-hop unchanged` | → | `Peer.linkLocalOnlyNextHopPermitted`, `buildMPReach` | `bgp-linklocal-only-next-hop-frr` interop scenario |
| A config `update` block announces an IPv6 route with next-hop self over an IPv4 session whose interface holds a global IPv6 address | → | `resolveNextHop` arm, `ownIPv6NextHop`, `linkLocalNextHopFor` | `bgp-ipv4-session-ipv6-global-next-hop-frr`, `bgp-ipv4-session-ipv6-global-next-hop-bird`, `TestOwnIPv6NextHopGlobalOverIPv4Session` |
| The same over a link with no global IPv6 address, capability 77 negotiated, no `session link-local` | → | the same arm, `linkLocalOnlyNextHopRefused` | `bgp-linklocal-own-next-hop-frr`, `TestOwnIPv6NextHopLinkLocalOnlyOverIPv4Session` |
| The same to an internal peer for a directly connected route | → | the same arm (`-4-3`) | `bgp-linklocal-own-next-hop-frr` (IBGP session to `peerFRRSink`), `TestOwnIPv6NextHopInternalDirectlyConnected` |
| A route received from another peer, forwarded with next-hop self over an IPv4 session | → | `buildForwardFacts`, `applyFactsNextHop` | the forward-rail cases of `TestOwnIPv6NextHopGlobalOverIPv4Session` and `TestOwnIPv6NextHopLinkLocalOnlyOverIPv4Session` |
| `ze` boots a config enabling capability 77 with no `session link-local` | → | `llnh.go` verifier | `test/plugin/llnh-accepts-without-link-local.ci` |

AC-7 has no row here: it adds no entry point, it re-reads tests over existing
producers (round-3 N3).

## Acceptance Criteria

| ID | When | Then |
|----|------|------|
| AC-7 | `-5-1`, `-4-2` and `-4-8` are re-read against the producers the summary names | each row's tagged test is shown, by its discrimination record and by reading its body, to drive the form Ze now emits rather than rest on the form being unreachable; a test that does not is rewritten and re-recorded. For `-4-2` the re-read covers the D6 forms: a received link-local forwarded under next-hop auto and under unchanged, to a peer on the shared segment and to one more than one hop away. If a form regressed, the producer is fixed in this spec and the tagged test proves it (R-1); the row stays met. `./le rfc check` exits clean for the stem. `-6-1` and `-1-2` are not in this list |
| AC-8 | The interop scenario `bgp-linklocal-only-next-hop-frr` runs: the injector (AS 65004, the `bgp-linklocal-only-multihop-withdraw-frr` pattern), Ze (AS 65001) and FRR (AS 65002) on one shared link, so both sessions are EBGP and §4 item 2 is the arm exercised (round-5 N1), each session over IPv4 addresses on that link (single hop), the IPv6 unicast family negotiated with capability 77 on both sessions, no global IPv6 address on the link, Ze relaying with `next-hop unchanged`; the injector announces a prefix whose MP_REACH Next Hop is 16 octets holding only the injector's own link-local on the shared interface | Ze relays the route to FRR with that 16-octet link-local unchanged: the checker asserts the captured Next Hop field of Ze's UPDATE to FRR equals the injector's link-local, and FRR installs the route via that link-local through the shared interface, read from FRR's RIB and kernel. The receive direction is AC-3 of `plan/immediate/spec-bgp-unowned-rfc-rows.md` and its scenario `bgp-linklocal-capability-interface-frr`; this scenario does not repeat it. The revert, rebuild, red, restore, green walk is recorded |
| AC-9 | Ze (AS 65001) announces an IPv6 unicast route with next-hop self to an external peer one hop away over an IPv4 session, on the announce rail and on both forward rails; Ze's interface on the link holds a global IPv6 address and a link-local; no `session link-local`; with and without capability 77 | the Next Hop field is 32 octets: the interface global, then the interface link-local (§4 item 2, "The Global IPv6 address, if one is present, SHOULD be included", and `-4-12`; RFC 2545 Section 3). With capability 77 negotiated it is still global plus link-local, never the link-local alone: owner, 2026-10-12, "Global plus link-local (Recommended)", BIRD's form, where FRR sends the link-local alone. Scenarios `bgp-ipv4-session-ipv6-global-next-hop-frr` (FRR, AS 65002, 77 negotiated) and `bgp-ipv4-session-ipv6-global-next-hop-bird` (BIRD, no 77): each checker reads Ze's addresses from the container, asserts the captured Next Hop field equals them, and reads the peer's RIB and kernel. Red with the arm removed: the route is withheld |
| AC-10 | The same, but the interface holds no global IPv6 address, capability 77 negotiated, no `session link-local` | the Next Hop field is 16 octets holding the interface link-local (§4 item 2, "MAY be the only next hop address"), and Ze logs one Info line per peer naming the peer, the interface and that the link-local goes alone because the interface holds no global. FRR installs it via that link-local (`bgp-linklocal-own-next-hop-frr`: the checker reads Ze's eth0 link-local from the container). Negative polarity, unit level: without 77 the announce rail returns `ErrNextHopLinkLocalOnly` and the forward rails withhold with `withholdLinkLocalOnlyRefused`, no NOTIFICATION, and the AC-13 Warn names the cause |
| AC-11 | The same link with no global, an internal peer (`peerFRRSink`, AS 65001, IBGP, 77 negotiated), and a route directly connected to Ze (an IPv6 prefix on another Ze interface, announced with next-hop self) | the Next Hop is Ze's own interface link-local, alone because the interface holds no global (`-4-3`, §4 item 1). `TestOwnIPv6NextHopInternalDirectlyConnected` carries the `-4-3` positive and negative tags moved from `draft_ietf_idr_linklocal_capability_llnh_refusal_test.go`, each with a record from `./le rfc discriminate-record` (red with `ownIPv6NextHop` returning no link-local: the route is withheld) |
| AC-12 | Multihop: the peer is in no connected prefix of any interface; the session's local address is on an interface | external peer: the Next Hop is the global of the interface holding the local address, alone (`-4-14`; `-4-8`, no link-local); with no global there the route is withheld (`-4-9`), never sent with a link-local in the global field: owner, 2026-10-12, "Withhold, like BIRD (Recommended)", where FRR sends the link-local, which §4 item 3 forbids ("Link-Local IPv6 next hops MUST NOT be included"). Internal peer: the same global alone (`-4-2`), withheld with none. A loopback source with no global is the withheld case. One unit case each; `bgp-linklocal-only-multihop-withdraw-frr` stays green |
| AC-13 | Ze cannot give a next hop the rules above accept: (a) one hop, no global, no usable link-local (IPv6 disabled, or every link-local tentative); (b) one hop, no global, link-local present, 77 not negotiated; (c) multihop, no global on the source interface; (d) `session link-local` set and not held, or held but tentative, by the interface; (e) one hop, a global present and no usable link-local; (f) the interface address read failed | (a), (b), (c), and (d) when no global exists: the route is not sent to that peer (§4: "If, after completing these procedures, there are no IPv6 next hop addresses included in the next hop, the BGP route MUST not be advertised to its peer"), the session stays up. (d) with a global and (e): the global goes alone. (f): withheld, never read as "no address". One Warn producer, once per peer and cause, re-armed when the snapshot changes (`-4-11`), with the consequence as a parameter. Each text opens with the problem ("peer <name>: IPv6 routes to this peer are withheld" or "... carry the global next hop without a link-local"), then the reason (interface <name> and what it lacks), then the exact command on its own line: (a) `sysctl -w net.ipv6.conf.<interface>.disable_ipv6=0`, or the Ze command that adds an IPv6 address to the interface (the exact path read from the iface YANG, sibling of `set interface ethernet <name> unit 0 ipv4 address`); (b) `set bgp peer <name> session capability link-local-nexthop enable` and the same on the peer's side, or the address command; (c) the address command on the source interface; (d) `set bgp peer <name> session link-local <addr>` listing the interface's usable link-locals, or `delete bgp peer <name> session link-local` to use the interface's own; (e) as (a); (f) the read error and the command that shows the interface. The unowned-rows spec builds this producer for its I4 case (the leaf not held, a global present: consequence "carry the global next hop without a link-local", the `set` fix only); this spec adds the other causes and the `delete` fix to the same producer, never a second one. `TestOwnIPv6NextHopRefusals`, one case per cause, asserts the decision, the text and that a second UPDATE logs nothing |
| AC-14 | Address choice on an interface with several IPv6 addresses | the global is the first address in kernel order that is not link-local, not `Tentative` and not "temporary" (BIRD `if_recalc_preferred`, FRR `if_get_ipv6_global`); the link-local is the `session link-local` leaf when the interface holds it and it is not tentative, else the first usable `fe80::/10` address (BIRD `llv6`, FRR `if_get_ipv6_local`). A tentative link-local is never sent as Ze's own (`-1-1`). `TestLinkScopeOwnAddresses`: several globals, a temporary first, a tentative first, the leaf second among several link-locals |
| AC-15 | A config enables capability 77 on a peer or group with no `session link-local` | `ze` boots and the session comes up: `refuseLinkLocalCapabilityWithoutAddress` no longer refuses it; a non-`fe80::/10` leaf is still refused. `draft_ietf_idr_linklocal_capability_llnh_refusal_test.go` and `llnh_test.go` assert acceptance; `test/plugin/llnh-refuses-without-link-local.ci` is replaced by `test/plugin/llnh-accepts-without-link-local.ci`; the `-4-3` tags leave the llnh tests (AC-11) and their records are removed from `rfc/discrimination/draft-ietf-idr-linklocal-capability.json` |
| AC-16 | Each case the arm must not change, and the one it changes on purpose | unchanged, one test case each: a link-local transport session (`TestNextHopSelfLinkLocalEndpointNeedsTheCapabilityOnTheForwardRails` stays green); an IPv4 NLRI (IPv4 next hop self); an IPv4 NLRI with an Extended Next Hop pair keeps the IPv4 endpoint, so §5's 16-octet form is not originated (round-6 N8); an IPv6 NLRI whose endpoint `canUseNextHopFor` admits (the arm's trigger, round-6 N4); an explicit `fe80::` next hop stays refused at the route-level entry points; a global-IPv6 session with the leaf set and held. Changed on purpose: a global-IPv6 session with no leaf now sends the interface link-local after the global when RFC 2545 Section 3's condition holds |
| AC-17 | An interface change that keeps the masked prefix table equal: a link-local moved from one interface to another, a global replaced within its prefix, a link-local finishing DAD | `refreshPeerLinkScopes` rebuilds the affected peers' scope and forwarding facts, so the next UPDATE carries the new address or the AC-13 refusal. The shortcut compares the per-peer inputs (interface name and its usable addresses with flags, unmasked), not the masked prefix list. `TestRefreshPeerLinkScopesOwnAddressChange`, red on today's comparison |

## End-to-End User Stories

| Story | Path |
|-------|------|
| An operator runs Ze as a route server or transit on a shared link with IPv4 sessions and carries IPv6 unicast routes; a speaker on that link announces a route with only its link-local as next hop | every speaker advertises 77; Ze relays the route with that 16-octet link-local unchanged, and FRR installs it through the link (`bgp-linklocal-only-next-hop-frr`) |
| The same operator has a global IPv6 address on the link and announces Ze's own IPv6 prefixes over the IPv4 sessions | Ze sends its interface global plus its link-local, and FRR and BIRD install the routes (`bgp-ipv4-session-ipv6-global-next-hop-frr`, `-bird`) |
| The same operator has no global on the link and enables `link-local-nexthop`, configuring no address | Ze sends its interface link-local alone and says so once in the log; FRR installs the routes (`bgp-linklocal-own-next-hop-frr`); if Ze cannot, its Warn names the peer, the interface and the command |
| FRR's own 16-octet link-local next hop toward Ze | installed by Ze through the link (`bgp-linklocal-capability-interface-frr`, unowned-rows spec) |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| the existing `-5-1`, `-4-2`, `-4-8` tagged tests, re-read | `internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_*_test.go` | AC-7 | |
| `TestOwnIPv6NextHopGlobalOverIPv4Session` | `internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_own_test.go` (new) | AC-9 through a real `Peer` with an injected snapshot, on the announce rail and both forward rails (general and route-server), the pattern of `TestNextHopSelfLinkLocalEndpointNeedsTheCapabilityOnTheForwardRails`: the Next Hop field is the 32-octet global plus link-local, with and without 77 | |
| `TestOwnIPv6NextHopLinkLocalOnlyOverIPv4Session` | same file | AC-10 on the three rails: with 77 the 16-octet link-local and one Info line; without 77 `ErrNextHopLinkLocalOnly` and `withholdLinkLocalOnlyRefused` | |
| `TestOwnIPv6NextHopInternalDirectlyConnected` | same file | AC-11, `-4-3` positive and negative tags | |
| `TestOwnIPv6NextHopMultihop` | same file | AC-12, external and internal, with and without a global on the source interface | |
| `TestOwnIPv6NextHopRefusals` | same file | AC-13, one case per cause: decision, Warn text, once per peer and cause | |
| `TestOwnIPv6NextHopUnchanged` | same file | AC-16, one case per row, and the global-IPv6 session gaining the interface link-local | |
| `TestLinkScopeOwnAddresses` | `internal/component/bgp/reactor/link_scope_test.go` | AC-14, and the source interface for a multihop peer | |
| `TestRefreshPeerLinkScopesOwnAddressChange` | `internal/component/bgp/reactor/reactor_iface_test.go` | AC-17 | |
| `TestLinkLocalCapabilityAcceptedWithoutLinkLocalAddress` | `internal/component/bgp/plugins/llnh/draft_ietf_idr_linklocal_capability_llnh_refusal_test.go` (renamed to match) and `llnh_test.go` | AC-15: accepted standalone, through the group, through `LoadConfig` and `ToPluginMap`; a non-`fe80::/10` leaf still refused | |

### Functional Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `llnh-accepts-without-link-local` | `test/plugin/llnh-accepts-without-link-local.ci` (replaces `llnh-refuses-without-link-local.ci`) | AC-15 through `ze` boot | |

### Interop

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-linklocal-only-next-hop-frr` | `test/interop/scenarios/bgp-linklocal-only-next-hop-frr` | FRR | AC-8: the 16-octet link-local Ze relays unchanged from a speaker on the same link is installed by FRR | |
| `bgp-ipv4-session-ipv6-global-next-hop-frr` | `test/interop/scenarios/bgp-ipv4-session-ipv6-global-next-hop-frr` | FRR | AC-9: over an IPv4 session, Ze's interface global plus link-local, installed by FRR | |
| `bgp-ipv4-session-ipv6-global-next-hop-bird` | `test/interop/scenarios/bgp-ipv4-session-ipv6-global-next-hop-bird` | BIRD | AC-9: the same, installed by BIRD, without capability 77 | |
| `bgp-linklocal-own-next-hop-frr` | `test/interop/scenarios/bgp-linklocal-own-next-hop-frr` | FRR | AC-10 and AC-11: Ze's interface link-local alone, to an EBGP FRR (AS 65002) and an IBGP `peerFRRSink` (AS 65001), on a link with no global, no `session link-local` in `ze.conf` | |
| `bgp-linklocal-capability-interface-frr` (created by the unowned-rows spec) | `test/interop/scenarios/bgp-linklocal-capability-interface-frr` | FRR | AC-8 receive direction, by reference | |

## Files to Modify

- `internal/component/bgp/reactor/link_scope.go` - the snapshot's session interface (one-hop or source), its usable IPv6 addresses from `iface.Addresses` and the one-hop flag; `ownIPv6NextHop` with the §4 item 1, item 2 and item 3 quotes above the statements that pick each address and the §1 quote above the tentative filter; `linkLocalNextHop` and `linkLocalNextHopFor` take the link-local from it; the one-hop-on-the-same-interface case for an IPv4 peer
- `internal/component/bgp/reactor/peer.go` (`resolveNextHop`) - the per-family arm with the RFC reference at the call
- `internal/component/bgp/reactor/peer_forward_facts.go` (the facts struct, `buildForwardFacts`, `applyFactsNextHop`) - the stored answer and its write for an IPv6-AFI `mpFamily` under `nhModeSelf4`
- `internal/component/bgp/reactor/reactor_iface.go` (`refreshPeerLinkScopes`) - compare the per-peer inputs, unmasked, with the interface name
- `docs/architecture/bgp/interface-event-reactions.md` - the interface-change paragraph declared by `refreshPeerLinkScopes`, which describes masked prefixes for the RFC 2545 scope: the per-peer comparison and the own-address inputs
- `internal/component/bgp/plugins/llnh/llnh.go`, `register.go` - remove the capability-77 refusal, keep the leaf form check, rewrite both comments
- `internal/component/bgp/plugins/llnh/yang/ze-link-local-nexthop.yang` - the description no longer says the leaf is required
- `internal/component/bgp/plugins/llnh/llnh_test.go`, `draft_ietf_idr_linklocal_capability_llnh_refusal_test.go` - AC-15
- `rfc/discrimination/draft-ietf-idr-linklocal-capability.json` - the moved `-4-3` records, new records for AC-11
- `rfc/short/draft-ietf-idr-linklocal-capability.md` - `-4-3` producer (the interface-derived link-local, AC-11) and its Enrolment reason; `-4-12` and `-4-14` re-read as met by the arm; `-1-1` names the tentative filter for a first-party next hop; `-4-8` and `-4-9` name the multihop arm; `-4-11` names the Warn; `-2-1` ("SHOULD advertise the Link-Local Next Hop Capability to its peers only when: 1. It is capable of sending IPv6 Link-Local-only next hops for a route") re-read against the arm; the Support coverage line drops `refuseLinkLocalCapabilityWithoutAddress`; AC-7's corrections; no support-remaining line claims Ze never sends its own link-local alone
- `rfc/short/rfc2545.md` - the Section 3 second-address row: the link-local now comes from the interface when no leaf is set (AC-16)
- `docs/features/rfc-status.md` - where AC-7, AC-11 or the RFC 2545 row changes a claim
- `docs/guide/configuration.md` - `link-local-nexthop` no longer needs `session link-local`; what Ze sends over an IPv4 session; the Warns
- `docs/guide/route-injection.md` - its "ze refuses" sentence about `session link-local`
- `docs/architecture/wire/capabilities.md` - §9c: the refusal paragraph
- `docs/architecture/core-design.md` - the next-hop self paragraph the reactor's next-hop code declares, and the link-scope snapshot
- `internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_*_test.go` - AC-7, only a test the re-read finds vacuous; any test asserting that an IPv6 route over an IPv4 session is withheld where the interface now gives an address, updated to the new answer with the owner decision cited
- `internal/component/bgp/reactor/forward_next_hop.go` - AC-7, only if the `-4-2` re-read finds a regression (R-1)
- `internal/le/interoplab/bgp/check_special.go` - map entries for the four scenarios
- `internal/le/interoplab/bgp/prepare.go` - a global IPv6 address on the shared link for the AC-9 scenarios, if A-4 finds none
- `docs/architecture/testing/interop.md` - a paragraph for each scenario beside the existing `bgp-linklocal-only-multihop-withdraw-frr` one

## Files to Create

- `test/interop/scenarios/bgp-linklocal-only-next-hop-frr/`
- `test/interop/scenarios/bgp-ipv4-session-ipv6-global-next-hop-frr/`
- `test/interop/scenarios/bgp-ipv4-session-ipv6-global-next-hop-bird/`
- `test/interop/scenarios/bgp-linklocal-own-next-hop-frr/`
- `internal/le/interoplab/bgp/check_own_next_hop.go` and its `_test.go` - the four scenarios' checkers
- `internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_own_test.go`
- `test/plugin/llnh-accepts-without-link-local.ci` (and `test/plugin/llnh-refuses-without-link-local.ci` passed to the commit's `remove`)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | no new leaf; `ze-link-local-nexthop.yang` description changes (AC-15) |
| YANG validation constraints | N-A | none new |
| YANG custom validators | Yes | `llnh.go`: the capability-77 refusal removed, the `fe80::/10` check kept (AC-15) |
| CLI commands/flags | N-A | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | Yes | `test/plugin/llnh-accepts-without-link-local.ci` |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | the missing address is a per-peer Warn at runtime, because it changes with the interface |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | capability 77 and IPv6 unicast exist |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `docs/guide/configuration.md` and the summary's interop evidence |
| 2 | Config syntax changed? | Yes | `link-local-nexthop` no longer requires `session link-local`: `docs/guide/configuration.md`, `docs/guide/route-injection.md`, the YANG description |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | Yes | llnh: `docs/architecture/wire/capabilities.md` §9c |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` |
| 7 | Wire format changed? | No | the 16-octet and 32-octet forms exist; Ze emits them in more cases |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/draft-ietf-idr-linklocal-capability.md`, `rfc/short/rfc2545.md`, `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md`: four scenario paragraphs |
| 11 | Affects daemon comparison? | Yes | the FRR and BIRD comparison is recorded in Key Design Decisions; any comparison page the docs index names for next-hop self is updated |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`: next-hop self and the link-scope snapshot |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/testing/interop.md`, declared by `check_special.go`; `docs/architecture/core-design.md`, declared by `link_scope.go` and `forward_next_hop.go`; `docs/architecture/wire/capabilities.md`, declared by `llnh.go`; `docs/architecture/bgp/interface-event-reactions.md`, declared by `reactor_iface.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/configuration.md` and `docs/guide/route-injection.md` mention `link-local-nexthop`; re-read both |

## Implementation Steps

1. AC-7 re-read of `-5-1`, `-4-2`, `-4-8`; correct any vacuous test, and fix a `-4-2` regression at its producer if the re-read finds one. Own commit.
2. Validate R-5, A-4 and A-5; bring up FRR over IPv4 with IPv6 unicast and capability 77 on a link with no global (A-3 confirmation). Stop and report on any failure.
3. AC-8 scenario `bgp-linklocal-only-next-hop-frr` (relay), with the recorded red. Own commit.
4. AC-15 (llnh), tests first, red; the `.ci`; docs §9c and route-injection; the tags leave the llnh tests in the same commit that adds them on the reactor (step 5), so no commit carries `-4-3` with no tag.
5. AC-9 to AC-14, AC-16, AC-17, after the unowned-rows spec's interface name has landed (R-3): tests first, red; the snapshot fields; `ownIPv6NextHop`; the two rail calls; the refresh comparison; the Warn producer; green; the `-4-3` records; docs and summaries. One commit with step 4.
6. The three own-next-hop scenarios, each with its recorded red (arm removed, image rebuilt). Own commit.

## Design Insights

- The feature was built by other work after this spec was written; what remains
  is proof plus Ze's own IPv6 next hop on an IPv4 session, and a proof that
  rests on an unreachable form proves nothing.
- Two implementations already answer "which IPv6 address does a speaker send
  over an IPv4 session"; reading both turned a design question into a
  comparison, and the only real choice left was where they differ.

## Key Design Decisions

- Match FRR and BIRD (owner, 2026-10-11, "match bird and frr behaviour"). The
  comparison, from the sources in Required Reading:

| Case | FRR | BIRD | Ze |
|------|-----|------|----|
| Which interface, one hop | the one holding the local IPv4 address | the neighbor's interface | the interface whose connected prefix holds the peer (unowned-rows A-2); the two agree on a numbered link |
| Which interface, multihop | the one holding the local IPv4 address | `neigh_find` of the source address: the one holding it | the one holding the local endpoint address |
| Which global | first non-link-local in the connected list | preferred: first non-secondary non-link-local, stable | first in kernel order not link-local, not tentative, not temporary |
| Which link-local | first link-local | `llv6`, chosen the same way | the leaf when held, else the first usable link-local |
| Tentative addresses | dropped (also DAD-failed) | dropped | dropped (`-1-1`) |
| Global present, one hop, no 77 | global plus link-local | global plus link-local | global plus link-local (AC-9) |
| Global present, one hop, 77 negotiated | link-local alone | no capability 77 | global plus link-local (AC-9), BIRD's form: owner, 2026-10-12, "Global plus link-local (Recommended)"; §4 item 2, "The Global IPv6 address, if one is present, SHOULD be included" |
| No global, one hop | link-local in both halves; alone in 16 octets with 77 | "Missing next hop address", nothing sent | with 77: link-local alone, logged once (AC-10, FRR's choice, owner); without 77: withheld with a Warn, because RFC 2545 Section 3 puts a global first and FRR's both-halves form is the one its own comment calls an interop expedient |
| Global present, no link-local | global alone | global alone, "Missing link-local address" | global alone, Warn (AC-13 e) |
| Multihop | the source interface's global; with none, the link-local as global | the source interface's global, no link-local; with none, nothing | BIRD's: the global alone, withheld with none (AC-12): owner, 2026-10-12, "Withhold, like BIRD (Recommended)"; §4 item 3, `-4-8`, `-4-9` |

- Addresses come from the interface with no config (owner). `session link-local`
  stays as an explicit override, and capability 77 no longer requires it: this
  reverses the llnh rule and its comment ("Deriving the address from the
  interface instead would make the next hop depend on runtime state the
  configuration does not show"). FRR and BIRD both depend on that runtime state;
  Ze makes it visible instead, through the Info line and the Warns, and the
  leaf remains for an operator who wants the address in config.
- A leaf that the interface does not hold, or holds tentative, is not replaced
  by the interface's own link-local. The operator named an address; sending
  another one silently would hide the config error (`ai/rules/principles.md`,
  first statement). The Warn offers both fixes.
- The arm's trigger is "`canUseNextHopFor` refuses the endpoint for this
  family" and the family's AFI is IPv6, never "the endpoint is not an IPv6
  address" (round-6 N4). On the IPv4 transport it therefore fires only where
  today's answer is withheld, including under any Extended Next Hop tuple that
  admits an IPv4 next hop for AFI 2.
- The answer: one function, `ownIPv6NextHop`, returns Ze's own global,
  link-local and one-hop flag, or the cause it has none, and every rail reads
  it (round-6 I2). The announce rail calls it from `resolveNextHop`; the forward
  rails from `buildForwardFacts`, written by `applyFactsNextHop`. It does not
  test capability 77: `linkLocalOnlyNextHopRefused` (announce) and
  `egressNextHopLinkLocalOnlyRefused` (forward) stay the only tests of the
  form, so the rails cannot disagree.
- The source is the link-scope snapshot the unowned-rows spec extends, at its
  cadence (establishment, reload, interface change, never per UPDATE). The
  address flags come from `iface.Addresses` for the one session interface,
  because `net.Interfaces` cannot tell a tentative or temporary address and
  both FRR and BIRD drop those. A failed read is a cause, never "no address"
  (R-5).
- The refresh compares what the answer is built from (round-6 I3): the
  interface name and its usable addresses, unmasked, with flags. A masked
  table cannot see a link-local move.
- One hop is the snapshot's interface: a peer inside a connected prefix of the
  interface. A peer reached through no connected prefix is multihop, and §4
  item 3 forbids the link-local ("Link-Local IPv6 next hops MUST NOT be
  included").
- For an internal peer the global is the session interface's, as FRR and BIRD
  do, though §4 item 1 names "the interface of the router through which the
  announced network is reachable". For a directly connected route on another
  interface that address is on a link the peer is not attached to; the session
  interface's address is the one the peer can reach. The MUST in the same item
  (`-4-3`, "its own Link-Local IPv6 address") is met by the session interface
  link-local.
- RFC 8950: an IPv4 NLRI keeps its IPv4 next hop self. §5 permits a 16-octet
  link-local for IPv4 NLRI when both capabilities are negotiated, but an IPv4
  next hop is present and natural there, so Ze does not originate that form;
  `linkLocalOnlyNextHopPermitted` keeps deciding whether a received one may be
  relayed (round-6 N8).
- Interop is the evidence for AC-8 to AC-12. A capability-gated wire format
  proven only against Ze proves the encoder agrees with itself
  (`ai/rules/interop-and-goal-validation.md`). BIRD has no capability 77, so it
  proves the global case only.
- The receive direction is proven once, by the unowned-rows spec's
  `bgp-linklocal-capability-interface-frr` (round-3 I2).
- AC-8 proves a relay, and stays (owner, 2026-10-11). The draft permits that
  form as a third-party next hop for an external peer: "Similarly, the speaker
  can use the received Link-Local IPv6 address, provided that peer X is
  directly attached." (§4, item 2).
- `-4-3` moves its producer from the config refusal to the interface-derived
  link-local and is re-recorded met by AC-11. A MUST is never relabeled: a
  further case where `-4-3` binds and Ze sends nothing (AC-13 e for an internal
  peer and a directly connected route) is a counted conformance gap reported to
  the owner, not an exclusion.
- A `-4-2` regression is a defect fixed here, never a ledger move to `{gap}`
  (round-3 I1).

## Owner decisions (2026-10-11 and 2026-10-12)

| Question | Owner's words | What it means here |
|----------|---------------|--------------------|
| Round-2 I5: the narrowed skeleton owns no open row; close it or rewrite it? | "Rewrite to AC-7/AC-8" | This spec keeps AC-7 and AC-8; the false claims about capability 77 and the seven rows are removed |
| Round-4 I1: relay a received link-local, or build origination? | "Relay only, for now (Recommended)" | Superseded below: the question rested on "no origination path exists", which round-5 I1 found false for a link-local transport |
| Round-5 I1: Ze sends its own link-local alone over a link-local transport, and not over an IPv4 or global-IPv6 transport on a link with no global IPv6 address; build that case? | "Build the missing case" | Built for the IPv4 transport. The global-IPv6 transport already sends a global, so it keeps it and now gains the interface link-local (Known Limitations, round-6 N6) |
| Round-6 B1 and I1: the interface-derived link-local is unreachable behind the llnh refusal, and a global on the interface gives no next hop on an IPv4 session; which way? The main thread read FRR `bgp_zebra_nexthop_set` and BIRD `bgp_start_channel` | "match bird and frr behaviour" | Where they agree Ze matches both: over an IPv4 session, next-hop self is the session interface's global (plus its link-local for a directly attached peer), from the interface with no config. Where they differ (no global on the link) Ze follows FRR: the link-local alone, single hop only, logged once. `session link-local` stays as an override and the llnh refusal is reversed. Found in the reading and put to the owner: with capability 77 FRR sends the link-local alone even when a global exists, and on a multihop session with no global FRR sends the link-local in the global field. Both answered on 2026-10-12, in the two rows below |
| 2026-10-12 (a): a directly connected peer, the link HAS a global IPv6 address, capability 77 negotiated: global plus link-local (BIRD's form), or the link-local alone (FRR's)? | "Global plus link-local (Recommended)" | AC-9: the 32-octet field, the interface global then the interface link-local, with and without capability 77; Ze sends the link-local alone only when the interface holds no global (AC-10). §4 item 2: "The Global IPv6 address, if one is present, SHOULD be included." Differs from FRR's `ll_nexthop_only` |
| 2026-10-12 (b): a multihop session whose source interface has no global IPv6 address: withhold (BIRD), or send the link-local (FRR)? | "Withhold, like BIRD (Recommended)" | AC-12 and AC-13 (c): the route is not sent to that peer and the session stays up, with the AC-13 Warn. §4 item 3: "Link-Local IPv6 next hops MUST NOT be included." and "If a Global IPv6 next hop is not included, the route MUST NOT be advertised to the external peer (treat-as-withdraw)." Differs from FRR, which writes the link-local as the global |

## Known Limitations

- The draft is revision -06 and not yet an RFC. A later revision can change the
  wire format, and the summary records the revision the walk read.
- A global-IPv6 session keeps its global endpoint as the next hop; when the
  peer is reached through no connected prefix (an unnumbered link with routed
  `/128`s, or IPv4 unnumbered) Ze cannot establish one hop from the snapshot
  and treats the peer as multihop (round-6 N7). Closing it means taking the
  egress interface from the kernel route lookup or the socket.
- An address the operator marked preferred is not honoured: BIRD has a
  configured preference, FRR does not, and Ze takes the first usable address.
  The leaf covers the link-local half.
- The link-local transport path is proven by a unit test
  (`TestNextHopSelfLinkLocalEndpointNeedsTheCapabilityOnTheForwardRails`), not
  by interop.

## RFC Documentation (Scope: protocol)

`rfc/short/draft-ietf-idr-linklocal-capability.md` carries the gated rows.
`rfc/drafts/draft-ietf-idr-linklocal-capability.txt` is the authority; the
summary is derived and is never the evidence. `ownIPv6NextHop` carries the §1,
§4 item 1, item 2 and item 3 quotes above the statements that implement them,
and each call names the section (`ai/rules/rfc-compliance.md`, functions added
from 2026-09-24).

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

### Goal Gates (MUST pass)
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] AC-7 to AC-17 demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket
