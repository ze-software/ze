# Spec: bgp-link-local-only-next-hop

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | plan/immediate/spec-bgp-unowned-rfc-rows.md (row 3: the link-scope snapshot gains the name of the session's interface, its A-2 decision, which AC-9 to AC-19 read; its I4 runtime check of `session link-local`, which AC-14 reuses; and its scenario `bgp-linklocal-capability-interface-frr`, which proves AC-8's receive direction); plan/immediate/spec-rfc2545-readvertise-on-condition-flip.md (the per-peer re-advertise on an interface address event, which AC-17 triggers when Ze's own next hop changes; unowned-rows I3 reuses the same path, and whichever spec lands first builds it) |
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
| AC-9 to AC-19 | Build, only for a session whose interface is configured under the `interface` root (owner, 2026-10-12, "Require an `interface` section"; without it Ze sends exactly what it sends today and logs one Warn naming the fix, AC-18), for every IPv6 family whose next hop has an IPv6 global form except 6PE (AFI 2 SAFI 4), which keeps the IPv4-mapped field RFC 4798 Section 2 requires: under next-hop self, an IPv6 route over an IPv4 session is sent with the session interface's global IPv6 address (plus its link-local for a directly attached peer, whether or not capability 77 is negotiated), or with the interface link-local alone when the peer is directly attached, the interface holds no global and capability 77 is negotiated, instead of being withheld; a multihop session whose source interface holds no global is withheld; addresses come from the interface with no config; `session link-local` stays as an explicit override, and capability 77 no longer requires it (owner, 2026-10-11, "match bird and frr behaviour"; owner, 2026-10-12, the two cases where FRR and BIRD differ, Owner decisions) |

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
FRR and BIRD"). AC-9 to AC-19 are that build, numbered after AC-8. The
round-7 review (B1, I1, I2, I3) then bounded it: only a session interface
configured under `interface`, every IPv6 family but 6PE, a separate source
interface field, and a re-advertise when the answer changes (Owner decisions).

### Where Ze stands today on an IPv6 next hop of its own

| Session transport | Next-hop self resolves to | What reaches the wire today |
|-------------------|---------------------------|-----------------------------|
| Link-local IPv6 (the TCP local endpoint is `fe80::`, for example a session a peer opened to Ze's link-local address) | that endpoint (`connectedLocalAddress`, `session_connection.go`) | with capability 77 negotiated, the 16-octet Link-Local-only Next Hop: `canUseNextHopFor` admits it for IPv6 unicast and `linkLocalOnlyNextHopRefused` (`peer.go`) lets it through; without 77 the route is withheld. `TestNextHopSelfLinkLocalEndpointNeedsTheCapabilityOnTheForwardRails` (`rfc_draft_linklocal_nhself_test.go`) asserts both |
| IPv4, IPv6 NLRI | the IPv4 endpoint | announce rail: `canUseNextHopFor` refuses an IPv4 next hop for every AFI 2 family but SR Policy unless an Extended Next Hop pair admits it, so `resolveNextHop` returns `ErrNextHopIncompatible` and the route is withheld. Forward rails: `precomputeNextHop` sets `nhModeSelf4` and `applyFactsNextHop` writes the IPv4-mapped `nhMapped` (`::ffff:<IPv4 endpoint>`) into MP_REACH. `MPNextHopProfile` (`internal/core/bgp/attribute/mpnlri.go`) gives AFI 2 SAFI 1, 2 and 4 (and SAFI 128, with a zero RD) `MappedIPv4: true`, so `egressNextHopWithheld` computes `mapped` for that field and skips `withholdUnusableGlobal`: the forward rails SEND the mapped field for 2/1, 2/2, 2/4 and 2/128 today, while the announce rail withholds the same route (round-7 I1). The comment above that test (`forward_next_hop.go`, "They do not license a mapped global in ordinary plain IPv6") says the opposite of the code, and no test covers 2/1. Whatever the interface holds, Ze never sends its own IPv6 address on this transport: this is the missing case |
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
  → Decision: the refusal is removed (AC-15) and `refuseLinkLocalCapabilityWithoutAddress` and its `...Sections` wrapper are deleted, not reduced (no layering, round-7 N6). Its last other job, returning `llnhEnabledFor`'s error for a value outside the enumeration, moves to the unowned-rows spec's `fe80::/10` check function, which walks the same peers; the YANG enumeration already refuses such a value at parse. The comment on `extractLLNHCapabilities` ("Caller MUST run refuseLinkLocalCapabilityWithoutAddress over the same section first") is rewritten to name that function. The `-4-3` tags at lines 75 and 104 and their discrimination records are deleted and re-observed on `TestOwnIPv6NextHopInternalDirectlyConnected` (AC-11), because the refusal no longer produces `-4-3`; this changes RFC-tagged tests, so it carries its own Owner decisions row and needs the owner's approval when implementation starts (round-7 N9).
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
    when `sharesNextHopSubnet` (`link_scope.go`) finds one connected prefix
    holding both the peer and the global, which an IPv4 peer and an IPv6 global
    never satisfy (round-7 N10).
  → Decision: for an IPv4 peer the RFC 2545 Section 3 condition becomes "the
    peer is in a connected IPv4 prefix of the interface that holds the global",
    with the Section 3 quote above the statement, because "common subnet" is
    then read across families: the two addresses share the link, not a prefix.
  → Constraint: the comment at `forward_next_hop.go` lines 725-727 contradicts
    the `mapped` test below it for 2/1 and 2/2; step 1 rewrites it to state
    what the code does and why (RFC 8950 Section 1: RFC 4798 "defines how the
    IPv4-mapped IPv6 address format ... can be used for that purpose when the
    <AFI/SAFI> is <2/1>, <2/2>, or <2/4>").
  → Decision: one new function answers "Ze's own IPv6 next hop for this peer, or
    the cause it has none" from the snapshot and the leaf. The announce rail
    calls it from `resolveNextHop`'s NextHopSelf arm when `canUseNextHopFor`
    refuses the endpoint for the family; the forward rails call it from
    `buildForwardFacts` and store the answer in a new facts field, which
    `applyFactsNextHop` writes as the MP_REACH next hop under `nhModeSelf4` in
    place of `nhMapped`, for an arm family only. The arm families are derived,
    never listed by hand: every AFI 2 family whose `MPNextHopProfile` carries
    an IPv6 global form and for which `canUseNextHopFor` refuses the IPv4
    endpoint, minus AFI 2 SAFI 4 (RFC 4798 Section 2, Key Design Decisions).
    Today that set is 2/1, 2/2 and 2/128; step 1 prints it from the profile
    table. The second address `linkLocalNextHop` appends takes its link-local
    from the same answer.
  → Constraint: the new behaviour applies only when the session interface is
    configured under `interface` and the iface backend answers for it
    (round-7 B1, owner decision). A lookup cause (no backend, not configured,
    bound twice, read failed) makes `ownIPv6NextHop` answer "not in use", and
    both rails keep today's answer for that route (announce rail withheld,
    forward rails the mapped field), so the not-configured case withholds no
    route Ze sends today. With the interface configured, every rail applies
    the same answer, including its withhold causes (AC-13), so the rails
    cannot disagree.
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
  → Constraint: A-2's interface is the ONE-HOP interface, and its row-3 receive
    side reads it: "A multihop peer, or one no single interface reaches, has no
    interface, and its Link-Local next hops are treated as withdrawn". So the
    interface a multihop session leaves from is a SEPARATE snapshot field, the
    source interface, read only by `ownIPv6NextHop` to choose Ze's own address
    and never by the receive side (round-7 I2). The unowned-rows spec states
    the same in its A-2 row.
  → Constraint: its I4 runtime check ("sent only when held") makes the reactor
    `-4-3` tests that feed the leaf `fe80::1` (`draft_ietf_idr_linklocal_capability_announce_test.go`
    lines 85 and 107, `draft_ietf_idr_linklocal_capability_test.go` lines 135,
    166 and 193) need a snapshot whose interface holds the leaf. That spec lands
    first (R-3) and updates them, with fresh discrimination records for each
    changed tagged unit; this spec re-reads them in AC-7 (round-7 N12).
- [ ] `internal/component/iface/iface.go` (`AddrInfo`: `LinkLocal`, `Tentative`, `Origin`), `internal/component/iface/resolve.go` (`Addresses`, `osDeviceFor`, `recordBinding`), `internal/component/iface/dispatch.go` (`errIfaceNoBackendLoaded`, `ResolveDevice`), `internal/component/iface/register.go` (`ConfigRoots`)
  → Constraint: `net.Interfaces` returns no address flags, so it cannot tell a
    tentative or temporary address. `iface.Addresses` returns each address with
    `Tentative` (kernel `IFA_F_TENTATIVE`, which a DAD-failed address keeps) and
    `Origin` ("temporary" for `IFA_F_TEMPORARY`). Its readers today are the
    OSPFv3 transport (through `resolveIfaceAddresses`), IS-IS, VRRP, LDP, IKE
    and the image server; the BGP healthcheck calls only `AddAddress` and
    `RemoveAddress` (round-7 N5).
  → Constraint: every call fails with `errIfaceNoBackendLoaded` until the iface
    plugin has loaded a backend, and the plugin auto-loads only when the config
    holds the `interface` root (`ConfigRoots`; `plugin/registry/registry.go`, "A
    present root auto-loads the plugin"). A BGP-only config has no backend
    (round-7 B1).
  → Constraint: `Addresses` takes a LOGICAL name and `osDeviceFor` maps it
    through `osNames` and `permMACs`, both keyed by logical name, falling back
    to the logical name as the device. The snapshot holds a KERNEL name
    (unowned-rows A-2), so passing it to `Addresses` resolves the wrong device
    when it equals the logical name of another interface that carries an
    os-name or MAC selector (round-7 B1).
  → Constraint: `docs/architecture/iface/logical-name-resolution.md`,
    "Resolution path": the config-apply path publishes to the resolver "a
    reverse map from OS name to logical names, so a kernel-name monitor event
    reaches every logical name bound to it" (`setResolverConfig`), and
    `osNameMap` "emits real overrides only. It skips the identity mapping". So
    the reverse map alone misses a logical name equal to its device with no
    selector, and a MAC-matched name is known only once bound.
  → Decision: a new iface function beside `ResolveDevice` answers by kernel
    device: the configured logical interface bound to exactly that device
    (the reverse map, the resolver's recorded MAC bindings, and a configured
    logical name equal to the device that carries no os-name or MAC selector;
    the page's "Resolution path" section gains a paragraph for it), with
    its addresses and flags, or a named cause: no backend loaded, no configured
    interface bound to the device, two bound to it, or the address read failed.
    It never falls back to treating the kernel name as a logical name (owner,
    2026-10-12, "resolve exactly; never by selector fallback").
- [ ] `internal/plugins/iface/netlink/show_linux.go` (`addrList`, called from `ListInterfaces` and `GetInterface`)
  → Constraint: `addrList` logs a Warn on a `netlink.AddrList` error and
    returns nil, so a failed read arrives as "no addresses" (round-7 B1).
  → Decision: `addrList` returns the error; `GetInterface`, the path this
    spec's read takes, returns it, so the read failure reaches the reactor as
    cause (f). An interrupted dump (`netlink.ErrDumpInterrupted`) is listed
    again, as `transient-failure-treated-as-fatal.md` records for the GTSM
    test. `ListInterfaces` fails one interface's read inside a listing of all;
    that path is a journal row (`plan/journal/zero-value-as-valid-answer.md`),
    not this spec's.
- [ ] `rfc/full/rfc4798.txt` Section 2 (line 241)
  → Constraint: "The IPv4 address of the egress 6PE router MUST be encoded as
    an IPv4-mapped IPv6 address in the BGP Next Hop field." 6PE (2/4) keeps
    the mapped field on every rail, interface or not.
- [ ] `rfc/full/rfc4659.txt` Section 3.2.1 (lines 313-415)
  → Constraint: Section 3.2.1: "The encoding of the BGP Next Hop depends on
    whether the policy of the BGP speaker is to request that IPv6 VPN traffic
    be transported to that BGP Next Hop using IPv6 tunneling ... or using IPv4
    tunneling". Section 3.2.1.1 (IPv6 transport): "the BGP speaker SHALL
    advertise a Next Hop Network Address field containing a VPN-IPv6 address
    ... whose 16-octet IPv6 address is set to the global IPv6 address of the
    advertising BGP speaker", and "The link-local address shall be included in
    the Next Hop field if and only if the advertising BGP speaker shares a
    common subnet with the peer the route is being advertised to". Section
    3.2.1.2 (IPv4 transport): the IPv4-mapped form with a zero RD. Both are
    permitted; the operator's tunnel policy chooses.
- [ ] `rfc/full/rfc4271.txt` Sections 9.1.3 and 9.2
  → Constraint: Section 9.2: "All newly installed routes and all newly
    unfeasible routes for which there is no replacement route SHALL be
    advertised to its peers by means of an UPDATE message." Section 9.1.3: "A
    route SHALL NOT be installed in the Adj-Rib-Out unless the destination,
    and NEXT_HOP described by this route, may be forwarded appropriately".
    A route whose next hop Ze rewrites to a new own address is a new route in
    that peer's Adj-RIB-Out, and a route Ze could not send before and can now
    is newly installed: both owe an UPDATE (AC-17).
- FRR `bgpd/bgp_zebra.c` `bgp_interface_address_add` and `bgp_interface_address_delete` (lines 318-430), FRR `bgpd/bgp_updgrp_packet.c` lines 455-560, BIRD `proto/bgp/bgp.c` `bgp_ifa_notify`, `bgp_neigh_notify` (lines 2122-2206) and `bgp_start_channel` (lines 2679-2730), BIRD `proto/bgp/packets.c` lines 1382-1392 and 1466-1478
  → Decision: FRR re-announces every family to a peer when a global IPv6
    address it uses as `v6_global` is deleted ("When we are using the v6
    global as part of the peering nexthops and we are removing it, then we need
    to clear the peer data saved for that nexthop and cause a re-announcement
    of the route. Since we do not want the peering to bounce."), and when a
    global appears on an interface peer whose `v6_global` was a mapped or
    link-local address. BIRD fixes the next hop at `bgp_start_channel` and
    re-reads it only when the session restarts (`bgp_neigh_notify` stops the
    session when the neighbor is lost). Both re-advertise through session
    re-establishment, which is what the draft says for a link-local change
    over a link-local transport ("routes are re-advertised through the normal
    session re-establishment procedures", §1). Over an IPv4 session the change
    does not reset the session, so RFC 4271 Section 9.2 decides: re-advertise
    the affected routes per peer, FRR's shape, without bouncing the session
    (Key Design Decisions, "Re-advertise").
  → Decision: 6VPE over an IPv4 session. FRR runs the same next-hop rewrite
    for the VPNv6 lengths (`BGP_ATTR_NHLEN_VPNV6_GLOBAL`, `..._AND_LL`) as for
    unicast: the interface global `peer->nexthop.v6_global`, and the mapped
    form only when that is unspecified (`ipv4_to_ipv4_mapped_ipv6`), which on
    an IPv4 session happens only when the interface holds no IPv6 address at
    all. FRR has no 6PE special case (no match for "6pe" in `bgpd/*.c`). BIRD
    takes `iface->addr6` for every IPv6 channel, VPNv6 included, and encodes
    an IPv4 next hop as the mapped form with a zero RD only when the channel
    sets `extended next hop` (packets.c: "IPv4 address is directly encoded with
    VPNv4 NLRI, but as IPv4-mapped IPv6 address with VPNv6 NLRI"). So both send
    the interface global for 6VPE by default; Ze matches them.
- [ ] `plan/immediate/spec-rfc2545-readvertise-on-condition-flip.md` (skeleton)
  → Constraint: it designs the per-peer forced re-announce on an interface
    address event, and records that "Ze has no Adj-RIB-Out refeed primitive
    today, and building one is the bulk of the work". AC-17 adds a trigger to
    that primitive and builds no second one (R-7).
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
Ze's only link-local of its own is that leaf. Over an IPv4 transport an IPv6
route under next-hop self is withheld on the announce rail and sent with the
IPv4-mapped field `::ffff:<IPv4 endpoint>` on the forward rails (2/1, 2/2, 2/4,
2/128), whatever the session interface holds, so Ze never sends an IPv6
address of its own there and `-4-3`, `-4-12` and `-4-14` are not met there. Over a global-IPv6 transport
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

1. Session establishment, reload, and an interface address change: the link-scope snapshot (unowned-rows A-2) records the ONE-HOP interface by kernel name (the interface whose connected prefix holds the peer; the receive side reads only this field), and, new here, a separate SOURCE interface field: the one-hop interface when there is one, else the interface holding the session's local endpoint address. Only `ownIPv6NextHop` reads the source interface (round-7 I2). For it the snapshot asks the iface component by kernel device (the new exact lookup, Required Reading) and keeps either the usable IPv6 addresses in kernel order, dropping a `Tentative` address and an address whose `Origin` is "temporary", or the cause the lookup returned (no backend loaded, interface not configured under `interface`, bound twice, read failed).
2. `ownIPv6NextHop` (new, `link_scope.go`) answers from the snapshot and the `session link-local` leaf: the global (the first remaining address that is not link-local), the link-local (the leaf when set and held, else the first remaining `fe80::/10` address), and the one-hop flag; or a cause when it cannot (Key Design Decisions, "The answer").
3. Announce rail: `resolveNextHop`'s NextHopSelf arm, when `canUseNextHopFor` refuses the endpoint for this family and the family is an arm family, takes the global from `ownIPv6NextHop`, or the link-local alone when one hop and no global (2/1 and 2/2 only: the draft's 16-octet form has no VPN-IPv6 counterpart here). `linkLocalOnlyNextHopRefused` then stays the one capability test, and `linkLocalNextHopFor` appends the link-local after a global for a one-hop peer. On "not in use" the arm does nothing and the route is withheld, as today.
4. Forward rails: `buildForwardFacts` stores the same answer in a new facts field; `applyFactsNextHop` under `nhModeSelf4` writes it as the MP_REACH next hop for an arm family (16 or 32 octets; 24 or 48 with a zero RD for 2/128), and keeps `nhLegacy` and `nhMapped` for every other family, 2/4 included, and for an arm family whose answer is "not in use", so the forward rails send the mapped field there as today. A withhold cause (AC-13 a to d) withholds on the forward rails too, the way the announce rail does. `egressNextHopLinkLocalOnlyRefused` stays the forward rails' capability test.
5. Global-IPv6 transport: `linkLocalNextHop` takes its link-local from `ownIPv6NextHop` instead of the leaf alone, so a session with no leaf gains the interface link-local as the RFC 2545 second address. On "not in use" it keeps today's answer, the leaf alone.
6. `buildMPReach` writes the 16-octet or 32-octet form.
7. A rebuilt snapshot whose `ownIPv6NextHop` answer differs from the previous one triggers the per-peer re-advertise of `plan/immediate/spec-rfc2545-readvertise-on-condition-flip.md` for that peer's routes in the arm families and the global-IPv6 transport's IPv6 families (AC-17).
8. Receive (AC-8 relay and the FRR side): MP_REACH_NLRI with a 16-octet link-local, carried with the session interface to the FIB, which the unowned-rows spec row 3 builds and its scenario `bgp-linklocal-capability-interface-frr` proves.

### Boundaries Crossed

The host interface table and the iface component's address flags (read into the
snapshot, never per UPDATE, through a component-to-infrastructure call the
unowned-rows spec's row 5 also makes), the iface netlink backend's address
read, the config verifier in the llnh plugin, the re-advertise path, and the
BGP wire against FRR and BIRD.

### Integration Points

| Point | What |
|-------|------|
| `linkScope` (`link_scope.go`) | the A-2 one-hop interface, the separate source interface, the source interface's usable IPv6 addresses or lookup cause, and `ownIPv6NextHop` |
| `resolveNextHop` (`peer.go`) | the announce rail's call, per family |
| `buildForwardFacts`, `applyFactsNextHop` (`peer_forward_facts.go`) | the forward rails' call and the new facts field |
| `refreshPeerLinkScopes` (`reactor_iface.go`) | rebuilds on any change of the per-peer inputs, and triggers the re-advertise when the answer changes |
| iface exact lookup by kernel device (`internal/component/iface/resolve.go`) | the configured interface bound to the device, its addresses and flags, or a named cause |
| `addrList`, `GetInterface` (`internal/plugins/iface/netlink/show_linux.go`) | a read error is returned, never an empty list |
| re-advertise path (`spec-rfc2545-readvertise-on-condition-flip`) | the per-peer re-announce AC-17 triggers |
| `ze doctor` | the unowned-rows AC-5 row for "no iface backend" also names the BGP peers whose own IPv6 next hop it disables (AC-18) |
| `refuseLinkLocalCapabilityWithoutAddress` (`llnh.go`) | the capability-77 refusal removed; the `fe80::/10` leaf check kept |
| `interoplab` checkers | `bgp-linklocal-only-next-hop-frr` (AC-8), `bgp-ipv4-session-ipv6-global-next-hop-frr` and `-bird` (AC-9), `bgp-linklocal-own-next-hop-frr` (AC-10, AC-11) |

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | FRR implements the draft's capability 77 and the 16-octet form | FRR source: `bgpd/bgp_attr.c` (`PEER_HAS_LINK_LOCAL_CAPABILITY`), `bgp_updgrp_packet.c` `ll_nexthop_only`; the existing `bgp-linklocal-only-multihop-withdraw-frr` scenario negotiates 77 between FRR 10.4.1 and Ze | AC-8, AC-10 and AC-11 have no peer, which `ai/rules/interop-and-goal-validation.md` refuses; the owner is asked which implementation to use | the existing scenario's negotiation | holds |
| A-2 | The existing tagged tests for `-5-1`, `-4-2`, `-4-8` drive the form Ze now emits | tags and discrimination records exist (grep, 2026-10-11) | a test that passes because the form is unreachable is vacuous; AC-7 replaces it | AC-7 | unvalidated |
| A-3 | FRR keeps an IPv4 single-hop session up with IPv6 unicast and capability 77 on a link with no global IPv6 address | FRR source (round-6 N1): `bgp_zebra_nexthop_set` copies the interface link-local into `v6_global` when no global exists, and `bgp_open_receive` Ceases only when `v6_global` is unspecified; `ipv6Sysctls` gives FRR's eth a link-local; the multihop Cease came from update-source `lo` | the no-global scenarios cannot run; the owner is told with the FRR log | FRR source; the bring-up in step 2 confirms | holds |
| A-4 | The lab can put a global IPv6 address on the shared link for Ze, FRR and BIRD while the sessions stay on IPv4 | `needsIPv6` (`internal/le/interoplab/bgp/prepare.go` line 137) makes the network dual-stack, with Docker-assigned globals, when any scenario file contains `baseIPv6Prefix` (round-7 N2). So the AC-9 scenarios contain that text, and the AC-8, AC-10 and AC-11 scenarios MUST NOT, or their link gains a global | AC-9's scenarios need a lab change first; it is made in this spec, not deferred | source | holds |
| A-5 | BIRD (the lab's image) installs an IPv6 route received over an IPv4 session with a 32-octet global plus link-local next hop | BIRD source 02d082a7 (round-7 N1): `bgp_decode_next_hop_ip` (`proto/bgp/packets.c` line 1412) accepts 32 octets, and under the default gateway mode `bgp_apply_next_hop` (line 1099) resolves the link-local on `p->neigh->iface`; with `next hop prefer global` it calls `neigh_find(gw)`, which needs BIRD to hold an address in the global's prefix, so the BIRD scenario keeps the default | AC-9's BIRD scenario cannot pass; the owner is told with BIRD's log | source; the step-2 bring-up confirms | holds |
| A-6 | Configuring the lab container's `eth0` (and a `dummy` interface) under `interface` in `ze.conf` leaves the Docker-assigned addresses and the kernel link-local in place, so the iface backend loads without changing the link the scenarios assert | the iface plugin programs the addresses its config names; whether it removes unnamed ones is not read yet | the AC-9 to AC-11 scenarios would assert addresses Ze removed; the scenario configures the addresses explicitly instead, and the checker reads them from the container either way | step 2: bring up `ze.conf` with `interface ethernet eth0` and compare `ip -6 addr show eth0` before and after | unvalidated |

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | The `-4-2` re-read finds that a received link-local forwarded under next-hop auto or unchanged reaches a peer more than one hop away again, a regression of D6 | it is a defect in an implemented capability and is fixed in this spec (`ai/rules/completion.md`): the producer changes, the tagged tests go red on the regression and green on the fix, and the row stays met; the ledger is never moved to `{gap}` to absorb it |
| R-2 | AC-8's receive direction rests on row 3 of the unowned-rows spec | the receive direction is proven by that spec's scenario `bgp-linklocal-capability-interface-frr`, and this spec's scenarios assert the send direction only |
| R-3 | AC-9 to AC-19 read the interface name the unowned-rows spec adds to `linkScope`, and AC-15 changes a function that spec's I4 text names | Depends names it; if that spec has not landed, this one stops before AC-9 and reports, rather than building a second interface read. The unowned-rows spec's I4, AC-3, YANG-validator and llnh rows were corrected on 2026-10-12 to match this design: the `fe80::/10` check is its own function, and the I4 Warn is the producer AC-13 extends |
| R-4 | Routes sent today change their next hop, all only when the session interface is configured under `interface`: on an IPv4 session the forward rails send the interface global (plus link-local one hop) for 2/1, 2/2 and 2/128 in place of the mapped field they send today, and the announce rail sends the same where it withholds today; a global-IPv6 session with no leaf gains the interface link-local as the second address (AC-16); a peer that negotiated 77 without the leaf, refused today, now comes up. For 6VPE the mapped field requests IPv4 tunneling (RFC 4659 Section 3.2.1.2) and the global requests IPv6 tunneling, so an IPv4-only MPLS core would lose 2/128 forwarding | the owner's decisions ("addresses come from the interface automatically"; I1, "Apply to all IPv6 families", which is FRR's and BIRD's default for 6VPE, Required Reading). 6PE keeps the mapped field (RFC 4798 Section 2). Without the `interface` entry no route sent today is withheld (AC-18). With it, the forward rails also withhold where the announce rail withholds (AC-13 a to d), in place of the mapped field they send today: the owner's "Withhold, like BIRD" and §4's "MUST not be advertised" applied on every rail, opted into by configuring the interface. AC-16 lists every case that must not change, and the docs name the 6VPE change (Known Limitations) |
| R-5 | The iface backend is absent (no `interface` root), the session interface is not configured under it, or the address read fails | owner, 2026-10-12, "Require an `interface` section": each is a named cause from the exact lookup; Ze keeps today's answer on every rail and logs the AC-13 Warn with the fix (AC-18). The lookup resolves by kernel device exactly and never falls back to the kernel name as a logical name; `addrList` returns its error, so a read failure is cause (f), never "no address" (AC-19) |
| R-7 | AC-17's re-advertise needs the per-peer re-announce primitive `spec-rfc2545-readvertise-on-condition-flip` designs, which is a skeleton and records that no Adj-RIB-Out refeed primitive exists | Depends names it; the unowned-rows spec's I3 reuses the same path ("whichever lands first builds it, the other reuses it"). If neither has landed when step 5 reaches AC-17, this spec builds it there to that spec's design and that spec reuses it; it never builds a second, per-trigger copy |
| R-6 | Deleting the llnh refusal deletes tagged tests | the behaviour they assert is reversed by the owner's decision; each tag moves to the test that proves the new producer (AC-11, AC-15), and the records are re-observed, never copied |

## Blast Radius

| Area | Change |
|------|--------|
| `link_scope.go` | the snapshot keeps the source interface beside the one-hop interface, its usable IPv6 addresses or the lookup cause; `ownIPv6NextHop`; `linkLocalNextHop` takes its link-local from it; `sharesNextHopSubnet` gains the IPv4-peer condition |
| `internal/component/iface/resolve.go` | the exact lookup by kernel device |
| `internal/plugins/iface/netlink/show_linux.go` | `addrList` returns its error; `GetInterface` returns it |
| re-advertise path | AC-17's trigger on an answer change |
| `forward_next_hop.go` comment | lines 725-727 state what the `mapped` test does |
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
| A BGP-only config (no `interface` root) announces and forwards IPv6 routes with next-hop self over an IPv4 session | → | the exact lookup's no-backend cause, the rails' unchanged answers, the Warn | `TestOwnIPv6NextHopWithoutInterfaceConfig`; `bgp-linklocal-only-multihop-withdraw-frr` (no `interface` root) stays green |
| The session interface's link-local finishes DAD after the session is up | → | iface monitor address event, `refreshPeerLinkScopes`, the re-advertise trigger | `TestRefreshPeerLinkScopesOwnAddressChange`; `bgp-linklocal-own-next-hop-frr` (Ze's route reaches FRR whether DAD finished before or after establishment) |

AC-7 has no row here: it adds no entry point, it re-reads tests over existing
producers (round-3 N3).

## Acceptance Criteria

| ID | When | Then |
|----|------|------|
| AC-7 | `-5-1`, `-4-2` and `-4-8` are re-read against the producers the summary names | each row's tagged test is shown, by its discrimination record and by reading its body, to drive the form Ze now emits rather than rest on the form being unreachable; a test that does not is rewritten and re-recorded. For `-4-2` the re-read covers the D6 forms: a received link-local forwarded under next-hop auto and under unchanged, to a peer on the shared segment and to one more than one hop away. If a form regressed, the producer is fixed in this spec and the tagged test proves it (R-1); the row stays met. `./le rfc check` exits clean for the stem. `-6-1` and `-1-2` are not in this list |
| AC-8 | The interop scenario `bgp-linklocal-only-next-hop-frr` runs: the injector (AS 65004, the `bgp-linklocal-only-multihop-withdraw-frr` pattern), Ze (AS 65001) and FRR (AS 65002) on one shared link, so both sessions are EBGP and §4 item 2 is the arm exercised (round-5 N1), each session over IPv4 addresses on that link (single hop), the IPv6 unicast family negotiated with capability 77 on both sessions, no global IPv6 address on the link, Ze relaying with `next-hop unchanged`; the injector announces a prefix whose MP_REACH Next Hop is 16 octets holding only the injector's own link-local on the shared interface | Ze relays the route to FRR with that 16-octet link-local unchanged: the checker asserts the captured Next Hop field of Ze's UPDATE to FRR equals the injector's link-local, and FRR installs the route via that link-local through the shared interface, read from FRR's RIB and kernel. The receive direction is AC-3 of `plan/immediate/spec-bgp-unowned-rfc-rows.md` and its scenario `bgp-linklocal-capability-interface-frr`; this scenario does not repeat it. The revert, rebuild, red, restore, green walk is recorded |
| AC-9 | Ze (AS 65001) announces an IPv6 unicast route with next-hop self to an external peer one hop away over an IPv4 session, on the announce rail and on both forward rails; Ze's interface on the link is configured under `interface` and holds a global IPv6 address and a link-local; no `session link-local`; with and without capability 77. Unit level, the same for IPv6 multicast (2/2) and VPNv6 (2/128) | the Next Hop field is 32 octets: the interface global, then the interface link-local (§4 item 2, "The Global IPv6 address, if one is present, SHOULD be included", and `-4-12`; RFC 2545 Section 3). With capability 77 negotiated it is still global plus link-local, never the link-local alone: owner, 2026-10-12, "Global plus link-local (Recommended)", BIRD's form, where FRR sends the link-local alone. Scenarios `bgp-ipv4-session-ipv6-global-next-hop-frr` (FRR, AS 65002, 77 negotiated) and `bgp-ipv4-session-ipv6-global-next-hop-bird` (BIRD, no 77): each `ze.conf` configures `eth0` under `interface`; each checker reads Ze's addresses from the container, asserts the captured Next Hop field equals them, and reads the peer's RIB and kernel. For 2/2 the field is the same; for 2/128 it is 48 octets, each address after a zero RD (RFC 4659 Section 3.2.1.1). Red with the arm removed: on the announce rail the route is withheld; on the forward rails the field is the 16-octet `::ffff:<IPv4 endpoint>` (step 1's pinned answer), not the interface addresses |
| AC-10 | The same, but the interface holds no global IPv6 address, capability 77 negotiated, no `session link-local` | the Next Hop field is 16 octets holding the interface link-local (§4 item 2, "MAY be the only next hop address"), and Ze logs one Info line per peer naming the peer, the interface and that the link-local goes alone because the interface holds no global. FRR installs it via that link-local (`bgp-linklocal-own-next-hop-frr`: the checker reads Ze's eth0 link-local from the container). Negative polarity, unit level: without 77 the announce rail returns `ErrNextHopLinkLocalOnly` and the forward rails withhold with `withholdLinkLocalOnlyRefused`, no NOTIFICATION, and the AC-13 Warn names the cause |
| AC-11 | The same link with no global, an internal peer (`peerFRRSink`, AS 65001, IBGP, 77 negotiated), and a route directly connected to Ze (an IPv6 prefix on another Ze interface, announced with next-hop self). The lab gives Ze only `eth0`, so the scenario's `ze.conf` configures `interface dummy dummy0` with an IPv6 prefix (for example `2001:db8:11::1/64`) beside `interface ethernet eth0`, and announces `2001:db8:11::/64` (round-7 N11) | the Next Hop is Ze's own interface link-local, alone because the interface holds no global (`-4-3`, §4 item 1). `TestOwnIPv6NextHopInternalDirectlyConnected` carries the `-4-3` positive tag (red with `ownIPv6NextHop` returning no link-local: the route is withheld) and a named negative: an internal peer more than one hop away (in no connected prefix of the source interface) gets no link-local in the field (red with the one-hop test removed: the link-local appears). Each tag's record comes from `./le rfc discriminate-record`, re-observed, never copied from the llnh records (Owner decisions, the `-4-3` tag move) |
| AC-12 | Multihop: the peer is in no connected prefix of any interface; the session's local address is on an interface | external peer: the Next Hop is the global of the interface holding the local address, alone (`-4-14`; `-4-8`, no link-local); with no global there the route is withheld (`-4-9`), never sent with a link-local in the global field: owner, 2026-10-12, "Withhold, like BIRD (Recommended)", where FRR sends the link-local, which §4 item 3 forbids ("Link-Local IPv6 next hops MUST NOT be included"). Internal peer: the same global alone (`-4-2`), withheld with none. A loopback source with no global is the withheld case. "Withheld" holds on every rail: the forward rails no longer send the mapped field they send today here, and never put a link-local in the global field, which is what the owner's answer rejected. Without the `interface` entry the rails keep today's answers (AC-18), and AC-7's re-read of `-4-8` and `-4-9` decides from the draft text whether the mapped field the forward rails send a multihop external peer under capability 77 is "a Global IPv6 next hop" in §4 item 3's sense ("If a Global IPv6 next hop is not included, the route MUST NOT be advertised to the external peer"); if it is not, that is a defect in an implemented capability, fixed at its producer in this spec (R-1's route), never a ledger move. One unit case each; `bgp-linklocal-only-multihop-withdraw-frr` stays green |
| AC-13 | Ze cannot give a next hop the rules above accept: (a) one hop, no global, no usable link-local (IPv6 disabled, or every link-local tentative); (b) one hop, no global, link-local present, 77 not negotiated; (c) multihop, no global on the source interface; (d) `session link-local` set and not held, or held but tentative, by the interface; (e) one hop, a global present and no usable link-local; (f) the interface address read failed; (g) no iface backend loaded (the config has no `interface` root); (h) the session interface is not configured under `interface`, or two configured interfaces bind its device | (a), (b), (c), and (d) when no global exists: the route is not sent to that peer on any rail (§4: "If, after completing these procedures, there are no IPv6 next hop addresses included in the next hop, the BGP route MUST not be advertised to its peer"), the session stays up; on the forward rails this replaces the mapped field sent today (R-4). (d) with a global and (e): the global goes alone. (f), (g), (h): today's answer on every rail (AC-18), never read as "no address". One Warn producer, once per peer and cause, re-armed when the snapshot changes (`-4-11`), with the consequence as a parameter. Each text opens with the problem ("peer <name>: IPv6 routes to this peer are withheld", "... carry the global next hop without a link-local", or "... do not use this router's own IPv6 address"), then the reason (interface <name> and what it lacks), then the exact command on its own line. Every `bgp` path is built from `configjson.PeerOrigin`, so a grouped peer is told `set bgp group <g> peer <p> ...` (or the group's `session` path where the setting is the group's), never `set bgp peer <name> ...` (round-7 N7). Where the interface is configured under `interface`, the fix is a Ze command, never a raw `sysctl -w`. (a) the iface YANG's path that enables IPv6 on the interface, or the one that adds an IPv6 address (the exact path read from the iface YANG, sibling of `set interface ethernet <name> unit 0 ipv4 address`); (b) the peer's `session capability link-local-nexthop enable` path and the same on the peer's side, or the address command; (c) the address command on the source interface; (d) the peer's `session link-local <addr>` path listing the interface's usable link-locals, or its `delete ... session link-local` to use the interface's own; (e) as (a); (f) the read error and `show interface <name>`; (g) `set interface ethernet <kernel name>` (the iface YANG list for the device's type), which loads the iface backend; (h) the same for the unconfigured device, or the two logical names bound to it. The unowned-rows spec builds this producer for its I4 case (the leaf not held, a global present: consequence "carry the global next hop without a link-local", the `set` fix only); this spec adds the other causes and the `delete` fix to the same producer, never a second one. `TestOwnIPv6NextHopRefusals`, one case per cause, asserts the decision, the text and that a second UPDATE logs nothing |
| AC-14 | Address choice on an interface with several IPv6 addresses | the global is the first address in kernel order that is not link-local, not `Tentative` and not "temporary" (BIRD `if_recalc_preferred`, FRR `if_get_ipv6_global`); the link-local is the `session link-local` leaf when the interface holds it and it is not tentative, else the first usable `fe80::/10` address (BIRD `llv6`, FRR `if_get_ipv6_local`). A tentative link-local is never sent as Ze's own (`-1-1`). `TestLinkScopeOwnAddresses`: several globals, a temporary first, a tentative first, the leaf second among several link-locals |
| AC-15 | A config enables capability 77 on a peer or group with no `session link-local` | `ze` boots and the session comes up: `refuseLinkLocalCapabilityWithoutAddress` and its `...Sections` wrapper are deleted, with the stale "Caller MUST run refuseLinkLocalCapabilityWithoutAddress" comment on `extractLLNHCapabilities` rewritten; a non-`fe80::/10` leaf is still refused, and a `link-local-nexthop` value outside the enumeration is still refused by the YANG enumeration and by the `fe80::/10` check function, which returns `llnhEnabledFor`'s error (a unit case each). With no `interface` root the config is accepted too, and the AC-13 (g) Warn names the fix. `draft_ietf_idr_linklocal_capability_llnh_refusal_test.go` and `llnh_test.go` assert acceptance; `test/plugin/llnh-refuses-without-link-local.ci` is replaced by `test/plugin/llnh-accepts-without-link-local.ci`; the `-4-3` tags leave the llnh tests (AC-11) and their records are removed from `rfc/discrimination/draft-ietf-idr-linklocal-capability.json` |
| AC-16 | Each case the arm must not change, and the one it changes on purpose | unchanged, one test case each: a link-local transport session (`TestNextHopSelfLinkLocalEndpointNeedsTheCapabilityOnTheForwardRails` stays green); an IPv4 NLRI (IPv4 next hop self); an IPv4 NLRI with an Extended Next Hop pair keeps the IPv4 endpoint, so §5's 16-octet form is not originated (round-6 N8); an IPv6 NLRI whose endpoint `canUseNextHopFor` admits (the arm's trigger, round-6 N4); an explicit `fe80::` next hop stays refused at the route-level entry points; a global-IPv6 session with the leaf set and held; 6PE (2/4) over an IPv4 session keeps the 16-octet `::ffff:<IPv4 endpoint>` on both forward rails and stays unchanged on the announce rail, with the interface configured and holding a global (RFC 4798 Section 2); SR Policy (2/73) keeps the IPv4 endpoint `canUseNextHopFor` admits (RFC 9830 Section 2.1). Changed on purpose, each a case: a global-IPv6 session with no leaf now sends the interface link-local after the global when RFC 2545 Section 3's condition holds; 2/1, 2/2 and 2/128 on the forward rails send the interface addresses in place of the mapped field (AC-9) |
| AC-17 | An interface change that keeps the masked prefix table equal: a link-local moved from one interface to another, a global replaced within its prefix, a link-local finishing DAD | `refreshPeerLinkScopes` rebuilds the affected peers' scope and forwarding facts. The shortcut compares the per-peer inputs (both interface names and the source interface's usable addresses with flags, unmasked), not the masked prefix list. When a peer's `ownIPv6NextHop` answer changes, Ze re-advertises that peer's affected routes through the re-advertise path (R-7) without resetting the session, as RFC 4271 Section 9.2 requires ("All newly installed routes and all newly unfeasible routes for which there is no replacement route SHALL be advertised"). `TestRefreshPeerLinkScopesOwnAddressChange`, red on today's comparison, asserts on routes advertised BEFORE the change: (1) a route withheld at establishment because the link-local was tentative is sent with the link-local once DAD completes, with no new route from the RIB; (2) a route sent with a link-local that then moves to another interface is re-sent with the new answer, or withdrawn when no answer remains (the AC-13 cause), never left on the wire with the old address; (3) a route sent with a global that is replaced within its prefix is re-sent with the new global; (4) a change that leaves the answer equal (an unrelated address, a temporary address) sends nothing. `bgp-linklocal-own-next-hop-frr` asserts (1) end to end: FRR holds Ze's route whether Ze's link-local finished DAD before or after the session came up |
| AC-18 | A BGP-only config (no `interface` root, so no iface backend), or a session interface not configured under `interface`, or one whose address read fails, carrying IPv6 routes with next-hop self over IPv4 and over global-IPv6 sessions | every rail sends exactly what it sends today: the announce rail withholds over IPv4, the forward rails send `::ffff:<IPv4 endpoint>` for 2/1, 2/2, 2/4 and 2/128, and a global-IPv6 session sends its global plus the leaf (unowned-rows I4 rules) or the global alone; one Warn per peer names the cause (AC-13 f, g, h) and the fix, and the unowned-rows AC-5 `ze doctor` row for "no iface backend" names these peers too. `TestOwnIPv6NextHopWithoutInterfaceConfig`, one case per cause and per rail, red if a lookup cause is read as "no address" (the route would be withheld on the forward rails). Step 1's `TestForwardRailsIPv4SessionMappedNextHopToday` pins today's 2/1 forward answer (16 octets, `::ffff:<IPv4 endpoint>`) before any change, and stays green after it for this case |
| AC-19 | The netlink address dump for the session interface fails | `addrList` returns the error, `GetInterface` returns it, and the exact lookup returns cause (f); an interrupted dump is listed again. `TestAddrListReturnsReadError` (netlink backend, Linux) and the cause (f) case of `TestOwnIPv6NextHopWithoutInterfaceConfig`; red on today's `addrList`, which returns nil |

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
| `TestRefreshPeerLinkScopesOwnAddressChange` | `internal/component/bgp/reactor/reactor_iface_test.go` | AC-17, the four before-and-after cases | |
| `TestForwardRailsIPv4SessionMappedNextHopToday` | `internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_own_test.go` | step 1: today's 2/1 forward answer on both forward rails, pinned before the arm; AC-18 keeps it green for the not-configured case | |
| `TestOwnIPv6NextHopWithoutInterfaceConfig` | same file | AC-18 and AC-19: each lookup cause on each rail keeps today's answer, one Warn | |
| `TestOwnIPv6NextHopArmFamilies` | same file | the arm family set derived from `MPNextHopProfile` and `canUseNextHopFor` is 2/1, 2/2 and 2/128, and excludes 2/4 and 2/73 | |
| `TestDeviceLookupExact` | `internal/component/iface/resolve_test.go` | the kernel-device lookup: an os-name binding, a MAC binding, a device whose name is another interface's logical name with a selector (answers the bound interface, never the namesake), unbound, bound twice, no backend | |
| `TestAddrListReturnsReadError` | `internal/plugins/iface/netlink/show_linux_test.go` | AC-19 | |
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
| `bgp-linklocal-own-next-hop-frr` | `test/interop/scenarios/bgp-linklocal-own-next-hop-frr` | FRR | AC-10, AC-11 and AC-17 (1): Ze's interface link-local alone, to an EBGP FRR (AS 65002) and an IBGP `peerFRRSink` (AS 65001), on a link with no global, no `session link-local` in `ze.conf`, which configures `interface ethernet eth0` and `interface dummy dummy0`; no file contains `baseIPv6Prefix` | |
| `bgp-linklocal-only-multihop-withdraw-frr` (existing, unchanged) | `test/interop/scenarios/bgp-linklocal-only-multihop-withdraw-frr` | FRR | AC-18: its `ze.conf` has no `interface` root, and it stays green | |
| `bgp-linklocal-capability-interface-frr` (created by the unowned-rows spec) | `test/interop/scenarios/bgp-linklocal-capability-interface-frr` | FRR | AC-8 receive direction, by reference | |

## Files to Modify

- `internal/component/bgp/reactor/link_scope.go` - the snapshot's source interface field beside the A-2 one-hop interface, its usable IPv6 addresses from the iface exact lookup or the lookup cause; `ownIPv6NextHop` with the §4 item 1, item 2 and item 3 quotes above the statements that pick each address and the §1 quote above the tentative filter; `linkLocalNextHop` and `linkLocalNextHopFor` take the link-local from it; `sharesNextHopSubnet` gains the IPv4-peer condition (the peer in a connected IPv4 prefix of the interface holding the global) with the RFC 2545 Section 3 quote above it
- `internal/component/iface/resolve.go` (and `dispatch.go` if the export lives there) - the exact lookup by kernel device, beside `ResolveDevice`
- `docs/architecture/iface/logical-name-resolution.md` - "Resolution path": the lookup by kernel device and its causes, beside the reverse map
- `docs/features/interfaces.md` - the address read: a failed dump is an error, never an empty address list; BGP's own IPv6 next hop reads the configured interface's addresses
- `internal/plugins/iface/netlink/show_linux.go` - `addrList` returns its error, `GetInterface` returns it, an interrupted dump is listed again; `ListInterfaces` keeps its behaviour and gets the journal row
- `plan/journal/zero-value-as-valid-answer.md` - one row: `ListInterfaces` reads a failed address dump as an interface with no addresses
- `internal/component/bgp/reactor/forward_next_hop.go` - the comment at lines 725-727 rewritten to state what the `mapped` test admits (step 1)
- the re-advertise path of `spec-rfc2545-readvertise-on-condition-flip` - the trigger on an `ownIPv6NextHop` answer change (AC-17)
- `internal/component/bgp/reactor/peer.go` (`resolveNextHop`) - the per-family arm with the RFC reference at the call
- `internal/component/bgp/reactor/peer_forward_facts.go` (the facts struct, `buildForwardFacts`, `applyFactsNextHop`) - the stored answer and its write for an IPv6-AFI `mpFamily` under `nhModeSelf4`
- `internal/component/bgp/reactor/reactor_iface.go` (`refreshPeerLinkScopes`) - compare the per-peer inputs, unmasked, with the interface name
- `docs/architecture/bgp/interface-event-reactions.md` - the interface-change paragraph declared by `refreshPeerLinkScopes`, which describes masked prefixes for the RFC 2545 scope: the per-peer comparison and the own-address inputs
- `internal/component/bgp/plugins/llnh/llnh.go`, `register.go` - delete `refuseLinkLocalCapabilityWithoutAddress` and `refuseLinkLocalCapabilityWithoutAddressSections` and their wiring in `OnConfigVerify` and `OnConfigure`, keep the leaf form check (which now also returns `llnhEnabledFor`'s enumeration error), rewrite the `extractLLNHCapabilities` and `register.go` comments
- `internal/component/bgp/plugins/llnh/yang/ze-link-local-nexthop.yang` - the description no longer says the leaf is required
- `internal/component/bgp/plugins/llnh/llnh_test.go`, `draft_ietf_idr_linklocal_capability_llnh_refusal_test.go` - AC-15
- `rfc/discrimination/draft-ietf-idr-linklocal-capability.json` - the moved `-4-3` records, new records for AC-11
- `rfc/short/draft-ietf-idr-linklocal-capability.md` - `-4-3` producer (the interface-derived link-local, AC-11) and its Enrolment reason; `-4-12` and `-4-14` re-read as met by the arm; `-1-1` is NOT met by this spec: the tentative filter covers the first-party half only, so the row names that producer and stays a counted `{gap}` for the other half, worded as that half ("a third-party Next Hop re-advertised from another peer" in the tentative state is not detected) (round-7 N8); `-4-8` and `-4-9` name the multihop arm; `-4-11` names the Warn; `-2-1` ("SHOULD advertise the Link-Local Next Hop Capability to its peers only when: 1. It is capable of sending IPv6 Link-Local-only next hops for a route") re-read against the arm; the Support coverage line drops `refuseLinkLocalCapabilityWithoutAddress`; AC-7's corrections; no support-remaining line claims Ze never sends its own link-local alone
- `rfc/short/rfc2545.md` - the Section 3 second-address row: the link-local now comes from the interface when no leaf is set (AC-16)
- `docs/features/rfc-status.md` - where AC-7, AC-11 or the RFC 2545 row changes a claim
- `docs/guide/configuration.md` - `link-local-nexthop` no longer needs `session link-local`; Ze's own IPv6 next hop needs the session's interface under `interface`, and what Ze sends without it; what Ze sends over an IPv4 session per family (6PE keeps the mapped field; 6VPE now sends the interface global, which requests IPv6 tunneling); the Warns
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
| Doctor check for runtime dependencies | Yes | the unowned-rows AC-5 row for "no iface backend" is shared, never duplicated: it also names the BGP peers whose own IPv6 next hop needs the session interface under `interface` (AC-18). A missing address stays a per-peer Warn at runtime, because it changes with the interface |
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
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/testing/interop.md`, declared by `check_special.go`; `docs/architecture/core-design.md`, declared by `link_scope.go` and `forward_next_hop.go`; `docs/architecture/wire/capabilities.md`, declared by `llnh.go`; `docs/architecture/bgp/interface-event-reactions.md`, declared by `reactor_iface.go`; `docs/architecture/iface/logical-name-resolution.md` and `docs/features/interfaces.md`, declared by `resolve.go` and the netlink backend |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/configuration.md` and `docs/guide/route-injection.md` mention `link-local-nexthop`; re-read both |

## Implementation Steps

1. AC-7 re-read of `-5-1`, `-4-2`, `-4-8` (and `-4-9` for the mapped field, AC-12); correct any vacuous test, and fix a `-4-2` regression at its producer if the re-read finds one. Pin today's 2/1 forward answer with `TestForwardRailsIPv4SessionMappedNextHopToday` (green on today's code), print the arm family set from `MPNextHopProfile`, and rewrite the contradicting comment at `forward_next_hop.go` lines 725-727. Own commit.
2. Validate A-6, and the AC-19 read error and the exact device lookup (tests first, red, then the iface and netlink changes; own commit). Bring up FRR over IPv4 with IPv6 unicast and capability 77 on a link with no global (A-3 confirmation), and BIRD over IPv4 with a global (A-5 confirmation). Stop and report on any failure.
3. AC-8 scenario `bgp-linklocal-only-next-hop-frr` (relay), with the recorded red. Own commit.
4. AC-15 (llnh), tests first, red; the `.ci`; docs §9c and route-injection; the tags leave the llnh tests in the same commit that adds them on the reactor (step 5), so no commit carries `-4-3` with no tag.
5. AC-9 to AC-14, AC-16 to AC-18, after the unowned-rows spec's interface name has landed (R-3): tests first, red; the snapshot fields; `ownIPv6NextHop`; the two rail calls; the refresh comparison; the Warn producer; the re-advertise trigger, on the path R-7 names; green; the `-4-3` records, after the owner has approved the tag move (Owner decisions); docs and summaries. One commit with step 4.
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
  address flags come from the iface component for the one source interface,
  looked up by kernel device exactly, because `net.Interfaces` cannot tell a
  tentative or temporary address and both FRR and BIRD drop those. A failed
  read is a cause, never "no address" (R-5, AC-19).
- Require an `interface` section (owner, 2026-10-12, round-7 B1). The iface
  backend, and with it the address flags and the address events AC-17 needs,
  exists only when the config holds the `interface` root. Without it, or with
  the session interface left out of it, Ze keeps today's behaviour exactly on
  every rail and says so once per peer with the fix (AC-18). The new
  behaviour is opted into per interface, so no BGP-only config changes what
  it sends.
- Which families (owner, 2026-10-12, "Apply to all IPv6 families"), bounded by
  RFC 4798 Section 2: "The IPv4 address of the egress 6PE router MUST be
  encoded as an IPv4-mapped IPv6 address in the BGP Next Hop field." An RFC
  MUST outranks a preference (`ai/rules/rule-precedence.md`, rung 2), so 6PE
  (2/4) keeps the mapped field. Every other AFI 2 family with an IPv6 global
  form whose IPv4 endpoint `canUseNextHopFor` refuses takes the arm: today
  2/1, 2/2 and 2/128, derived from `MPNextHopProfile`, never listed. 6VPE is
  in: RFC 4659 Section 3.2.1 leaves the choice to the operator's tunnel
  policy, and FRR and BIRD both send the interface global for VPNv6 over an
  IPv4 session by default (Required Reading). The 16-octet link-local-only
  form stays 2/1 and 2/2.
- Re-advertise when Ze's own next hop changes. RFC 4271 Section 9.2 owes an
  UPDATE for every newly installed route, and a route whose next hop Ze
  rewrites is a new route in that peer's Adj-RIB-Out. FRR re-announces a
  peer's families when the global it uses is deleted or a global replaces a
  mapped or link-local `v6_global` (`bgp_interface_address_add`,
  `bgp_interface_address_delete`); BIRD re-reads its next hop only on session
  restart. Both re-advertise through re-establishment, and over an IPv4
  session the change resets nothing, so Ze takes FRR's per-peer re-announce
  without a session reset, on the path `spec-rfc2545-readvertise-on-condition-flip`
  designs (R-7). This is decided from the RFC and both sources, not put to
  the owner.
- Two interface fields (round-7 I2). The A-2 one-hop interface keeps its
  contract, and the receive side reads only it; the source interface is a
  separate field read only to choose Ze's own address. A multihop peer has a
  source interface and no one-hop interface, so a link-local received from it
  stays withdrawn.
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
| 2026-10-12 (b): a multihop session whose source interface has no global IPv6 address: withhold (BIRD), or send the link-local (FRR)? | "Withhold, like BIRD (Recommended)" | AC-12 and AC-13 (c): the route is not sent to that peer and the session stays up, with the AC-13 Warn. §4 item 3: "Link-Local IPv6 next hops MUST NOT be included." and "If a Global IPv6 next hop is not included, the route MUST NOT be advertised to the external peer (treat-as-withdraw)." Differs from FRR, which writes the link-local as the global. Round 7 found that the forward rails send the mapped field today where the spec had said they withhold; with the interface configured the withhold applies on every rail and replaces that field, and without it nothing changes (R-4, AC-18) |
| Round-7 B1: the address source has no answer in a BGP-only config, because the iface backend loads only with the `interface` root | "Require an `interface` section" | The new behaviour applies only to a session interface configured under `interface`. Without the backend, or with the interface not configured, Ze keeps today's behaviour exactly, so no route Ze sends today is withheld, and logs one Warn naming the fix (AC-13 g, h; AC-18). The device lookup is exact, never a selector fallback; `addrList` returns its read error (AC-19), and its `ListInterfaces` path is a journal row |
| Round-7 I1: limit the arm to IPv6 unicast and multicast, leaving 6PE and 6VPE on the mapped field? | "Apply to all IPv6 families" | Every AFI 2 family with an IPv6 global form takes the arm, 6VPE included (FRR and BIRD send the interface global for it), except 6PE: RFC 4798 Section 2, "The IPv4 address of the egress 6PE router MUST be encoded as an IPv4-mapped IPv6 address in the BGP Next Hop field." An RFC MUST outranks the preference (`ai/rules/rule-precedence.md`, rung 2), so 2/4 keeps the mapped field (AC-16). Today's 2/1 forward answer is pinned in step 1, and Current Behavior, R-4 and AC-9's red claim are corrected |
| Round-7 I2: a multihop peer's "session interface" collides with unowned-rows A-2 | Owner, 2026-10-12: keep A-2, add a separate source-interface field | Two fields; the receive side reads only the one-hop interface. Stated in both specs (consistency edit to `plan/immediate/spec-bgp-unowned-rfc-rows.md` A-2, its status unchanged) |
| Round-7 I3: nothing re-advertises when Ze's own next hop changes | Owner, 2026-10-12: not an owner question; decide from the RFC and FRR and BIRD | RFC 4271 Section 9.2 and FRR's per-peer re-announce decide it: AC-17 re-advertises the affected routes on the readvertise spec's path, which Depends names (Key Design Decisions, "Re-advertise") |
| Round-7 N9: the two llnh `-4-3` tags (`draft_ietf_idr_linklocal_capability_llnh_refusal_test.go` lines 75 and 104) and their records are deleted and re-observed on `TestOwnIPv6NextHopInternalDirectlyConnected` | PENDING: needs the owner's approval at implementation, because it moves RFC-tagged tests (`ai/rules/rule-precedence.md`) | AC-11: the positive (the internal peer's next hop is Ze's interface link-local) and the named negative (an internal peer more than one hop away gets none), each recorded by `./le rfc discriminate-record`. Step 5 stops before the move until the owner approves |

## Known Limitations

- The draft is revision -06 and not yet an RFC. A later revision can change the
  wire format, and the summary records the revision the walk read.
- A global-IPv6 session keeps its global endpoint as the next hop; when the
  peer is reached through no connected prefix (an unnumbered link with routed
  `/128`s, or IPv4 unnumbered) Ze cannot establish one hop from the snapshot
  and treats the peer as multihop (round-6 N7). Closing it means taking the
  egress interface from the kernel route lookup or the socket.
- A BGP-only config (no `interface` root) keeps today's next hops: no own IPv6
  address over an IPv4 session, the mapped field on the forward rails. The
  Warn and the doctor row name the fix (AC-18).
- 6VPE over an IPv4 session now requests IPv6 tunneling (the interface
  global) where it requested IPv4 tunneling (the mapped field) when the
  session interface is configured under `interface`; an operator whose core
  carries only IPv4 LSPs has no per-family switch for it, only leaving that
  interface out of `interface` (R-4).
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
- [ ] AC-7 to AC-19 demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket
