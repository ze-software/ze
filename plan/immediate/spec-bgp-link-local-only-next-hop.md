# Spec: bgp-link-local-only-next-hop

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | plan/immediate/spec-bgp-unowned-rfc-rows.md (row 3: the link-scope snapshot gains the name of the session's interface, its A-2 decision, which AC-9 to AC-12 read; and its scenario `bgp-linklocal-capability-interface-frr`, which proves AC-8's receive direction) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Prove what is left of `draft-ietf-idr-linklocal-capability` once the code that
performs it exists, and build the one origination case Ze lacks:

| Part | What |
|------|------|
| AC-7 | Re-read three rows that used to rest on an unreachable form |
| AC-8 | Interop: Ze relays a received link-local-only next hop unchanged to FRR |
| AC-9 to AC-12 | Build: Ze originates its OWN link-local as the only next hop on an IPv4 session to a directly attached peer over a link whose interface holds no global IPv6 address, when capability 77 is negotiated; refuse with a named reason when it cannot (owner, 2026-10-11, "Build the missing case") |

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
dropped as already met by other work. The round-5 design review (I1) then found
that the round-4 premise, "Ze has no path that writes its own link-local as the
only next hop", was false for one transport, and the owner chose to build the
case that is really missing. That case is AC-9 to AC-12, numbered after AC-8.

### Where Ze stands today on its own link-local as the only next hop

| Session transport | Next-hop self resolves to | What reaches the wire today |
|-------------------|---------------------------|-----------------------------|
| Link-local IPv6 (the TCP local endpoint is `fe80::`, for example a session a peer opened to Ze's link-local address) | that endpoint (`connectedLocalAddress`, `session_connection.go`) | with capability 77 negotiated, the 16-octet Link-Local-only Next Hop: `canUseNextHopFor` admits it for IPv6 unicast and `linkLocalOnlyNextHopRefused` (`peer.go`) lets it through; without 77 the route is withheld. `TestNextHopSelfLinkLocalEndpointNeedsTheCapabilityOnTheForwardRails` (`rfc_draft_linklocal_nhself_test.go`) asserts both |
| IPv4, IPv6 NLRI | the IPv4 endpoint | `canUseNextHopFor` refuses an IPv4 next hop for IPv6 unicast unless an Extended Next Hop pair is negotiated, so the route is withheld (`ErrNextHopIncompatible`). Ze's own link-local is never sent: this is the missing case |
| Global IPv6 | the global endpoint | the global, plus Ze's link-local as the second address when RFC 2545 Section 3's shared-subnet condition holds (`linkScope.linkLocalNextHop`); never the link-local alone |

`reactor_peers.go` refuses a link-local `local-address` ("link-local addresses
not supported for local-address"); it does not refuse a session whose local
endpoint turns out to be link-local, which is the first row.

## Required Reading

- `rfc/short/draft-ietf-idr-linklocal-capability.md` and
  `rfc/drafts/draft-ietf-idr-linklocal-capability.txt` (revision -06)
  → Constraint: the draft text is the authority; the summary is derived. Rows
    `-1-2` and `-6-1` are not this spec's.
- `rfc/drafts/draft-ietf-idr-linklocal-capability.txt` §4 (lines 250-395) and §5 (lines 400-415)
  → Constraint: §4 opening: "The next hop MAY consist of only a Link-Local IPv6 next hop." and "If, after completing these procedures, there are no IPv6 next hop addresses included in the next hop, the BGP route MUST not be advertised to its peer."
  → Constraint: §4 item 1 (internal peer): "When announcing a locally-originated route to an internal peer, or the BGP speaker has been explicitly configured to announce its own IP address as the next hop, the BGP speaker SHOULD use the Global IPv6 address of the interface of the router through which the announced network is reachable for the speaker as the next hop, if present. If the route is directly connected to the speaker, or if the interface address of the router through which the announced network is reachable for the speaker is the internal peer's address, the next hop MUST include its own Link-Local IPv6 address." (`-4-3`)
  → Constraint: §4 item 2, default arm (external peer one IP hop away): "By default (if none of the above conditions apply), the BGP speaker SHOULD use the IP address of the interface that the speaker uses to establish the BGP connection to peer X in the next hop attribute. The Global IPv6 address, if one is present, SHOULD be included. If the BGP speaker is one IP hop away, the Link-Local IPv6 address SHOULD be included, and MAY be the only next hop address in the next hop."
  → Constraint: §4 item 3 (multihop external peer): "Link-Local IPv6 next hops MUST NOT be included."
  → Constraint: §5: "When both the Link-Local Next Hop Capability defined in this document and the Extended Next Hop Encoding capability ([RFC8950]) have been negotiated between two peers, a 16-octet Link-Local-only Next Hop is permitted for IPv4 NLRI carried with an IPv6 Next Hop, provided the receiving peer is directly attached."
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
  → Decision: AC-8's and AC-9's sessions run over IPv4, as this scenario's does
    (`frr.conf` `neighbor 172.30.0.2 capability link-local`, `ze.conf` `link-local
    fe80::be:ef:2`). Ze refuses a link-local `local-address`
    (`reactor_peers.go`), and the missing case is the IPv4 transport.
  → Decision: AC-8 reuses this scenario's injector (`inject.msg`, `inject-args`,
    a `peer injector` Ze dials with `accept false`), placed on the SAME link as
    Ze and FRR, so the relayed link-local is a third-party next hop FRR can use
    (round-4 I1).
  → Constraint: its `frr.conf` notes "FRR also needs a Global IPv6 address on the
    interface the session leaves from, or it has no IPv6 next hop of its own and
    ends the session with a Cease." That was a multihop session without a
    link-local next hop of FRR's own; A-3 validates it on the single-hop link
    before either new scenario is built.
- [ ] `internal/core/bgp/attribute/nexthop_form.go` (`ValidateGlobalNextHop`),
  `internal/component/bgp/reactor/session_connection.go` (`connectedLocalAddress`),
  `internal/component/bgp/reactor/peer.go` (`resolveNextHop`, `canUseNextHopFor`,
  `linkLocalOnlyNextHopPermitted`, `linkLocalOnlyNextHopRefused`),
  `internal/component/bgp/reactor/peer_forward_facts.go` (`precomputeNextHop`),
  `internal/component/bgp/reactor/link_scope.go` (`linkScope`, `linkLocalNextHop`, `applyLinkLocalNextHop`)
  → Constraint: next-hop self resolves to the TCP local endpoint on every rail
    (announce: `resolveNextHop`; forward: `precomputeNextHop`), so the rails
    send one address. Over a link-local transport that endpoint is link-local
    and is sent alone when capability 77 is negotiated (round-5 I1). Over an
    IPv4 transport it is IPv4 and an IPv6 NLRI is withheld. An explicit
    `fe80::` next hop is refused at every route-level entry point ("RFC 2545
    Section 3"); the `session link-local` leaf fills only the second address
    after a global today.
  → Decision: the new arm sits in the one helper both rails already share for
    next-hop self, never in each rail, so the announce and forward rails cannot
    disagree (the comment above `linkLocalOnlyNextHopRefused` names that failure).
- [ ] `plan/immediate/spec-bgp-unowned-rfc-rows.md` A-2 decision and I4
  → Constraint: the link-scope snapshot gains "the NAME of the interface whose
    connected prefix contains the peer, read at the same cadence as today
    (session establishment and reload, never per UPDATE, from `net.Interfaces`
    with each interface's addresses)"; a peer no single interface reaches has no
    interface. I4: the configured `session link-local` must be an address of
    that interface, re-checked after an interface change, or Ze withholds it
    with a Warn naming the fix.
  → Decision: this spec reads Ze's own link-local from the SAME snapshot read,
    not from a second source: the interface's link-local addresses and whether
    it holds any global IPv6 address are kept beside its name.
- [ ] `internal/component/iface/iface.go` (`AddrInfo.LinkLocal`)
  → Constraint: the iface component already classifies a `fe80::/10` address as
    `LinkLocal` for IS-IS. The reactor's snapshot classifies with the same test
    (`netip.Addr.IsLinkLocalUnicast`); it does not call the iface component per
    session, because the unowned-rows A-2 read already returns each interface's
    addresses.

## Current Behavior (MANDATORY)

Source files read:

- [ ] `internal/core/bgp/capability/capability.go` - `CodeLinkLocalNextHop`, the typed `LinkLocalNextHop` case
- [ ] `internal/core/bgp/capability/negotiated.go` - `Negotiate`, the `LinkLocalNextHop` field
- [ ] `internal/component/bgp/reactor/peer.go` - `resolveNextHop` (next-hop self: `connectedLocalAddress`, then `canUseNextHopFor`, then `linkLocalOnlyNextHopRefused`), `linkLocalOnlyNextHopPermitted`, `canUseNextHopFor` (natural match, then the Extended Next Hop pairs)
- [ ] `internal/component/bgp/reactor/session_connection.go` - `connectedLocalAddress`: the TCP local endpoint, zero for unspecified or multicast
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - `precomputeNextHop`, next-hop self on the forward rails
- [ ] `internal/component/bgp/reactor/link_scope.go` - `linkScope` (connected prefixes and `peerOnLink`), `linkLocalNextHop` (the configured link-local as the second address only when the speaker is the next hop's router and the global shares a subnet with the peer)
- [ ] `internal/component/bgp/message/update_build.go` - `buildMPReach`, the 16-octet and 32-octet forms
- [ ] `internal/component/bgp/reactor/forward_next_hop.go` - `egressNextHopIsLinkLocalOnly`
- [ ] `internal/component/bgp/reactor/reactor_peers.go` - the refusal of a link-local `local-address`
- [ ] `internal/component/bgp/reactor/rfc_draft_linklocal_nhself_test.go` - `TestNextHopSelfLinkLocalEndpointNeedsTheCapabilityOnTheForwardRails`

Capability 77 is advertised, parsed, negotiated and acted on for the send side
and the reflection path. Ze sends its own link-local as the only next hop only
over a link-local transport (Task, table). Over an IPv4 transport on a link
whose interface holds no global IPv6 address, an IPv6 route under next-hop self
is withheld, so `-4-3` is not met there (round-5 review, "-4-3 under
relay-only"). `test/interop/scenarios/bgp-linklocal-only-multihop-withdraw-frr/`
exists and covers the multihop withdraw. No scenario shows another
implementation installing a 16-octet link-local next hop Ze sends on a shared
link, relayed or its own.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

An operator sets `link-local-nexthop` (capability 77) on a peer facing a
speaker that also advertises capability 77, over an IPv4 session on a shared
link. For AC-8 Ze relays with `next-hop unchanged`; for AC-9 to AC-12 Ze
announces (config `update` blocks, the API) or forwards with next-hop self.

### Transformation Path

1. Session establishment, reload, and an interface change: the link-scope snapshot (unowned-rows A-2) records the session's interface by name, and, new here, that interface's link-local addresses and whether it holds a global IPv6 address.
2. Next-hop self, one helper for the announce rail (`resolveNextHop`) and the forward rails (`precomputeNextHop`): the TCP local endpoint, as today, unless all of these hold: the NLRI's next hop is an IPv6 one (IPv6 NLRI; IPv4 NLRI keeps its IPv4 next hop), the endpoint is not an IPv6 address, capability 77 is negotiated, the snapshot names an interface, and that interface holds no global IPv6 address. Then the next hop is Ze's own link-local on that interface: the `session link-local` leaf when set and held by the interface, else the interface's single link-local address.
3. `canUseNextHopFor` admits the link-local for an IPv6 NLRI (natural match), and `linkLocalOnlyNextHopRefused` stays the one capability test for the form.
4. `buildMPReach` writes the 16-octet Link-Local-only Next Hop.
5. Receive (AC-8 relay and the FRR side): MP_REACH_NLRI with a 16-octet link-local, carried with the session interface to the FIB, which the unowned-rows spec row 3 builds and its scenario `bgp-linklocal-capability-interface-frr` proves.

### Boundaries Crossed

The host interface table (read into the snapshot, never per UPDATE), and the BGP
wire against a peer daemon that implements the draft.

### Integration Points

| Point | What |
|-------|------|
| `linkScope` (`link_scope.go`) | keeps the session interface's link-local set and global-presence flag beside the name the unowned-rows spec adds |
| next-hop self helper (`peer.go`, `peer_forward_facts.go`) | the new arm, shared by every rail |
| `interoplab` checkers | `bgp-linklocal-only-next-hop-frr` (AC-8) and `bgp-linklocal-own-next-hop-frr` (AC-9, AC-10) |

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | FRR implements the draft's capability 77 and the 16-octet form | FRR source: `bgpd/bgp_attr.c` (`PEER_HAS_LINK_LOCAL_CAPABILITY`), also `bgp_nht.c` and `bgp_updgrp_packet.c` (round-3 review N2); the existing `bgp-linklocal-only-multihop-withdraw-frr` scenario already negotiates capability 77 between FRR 10.4.1 and Ze | AC-8 and AC-9 have no peer, which `ai/rules/interop-and-goal-validation.md` refuses; the owner is asked which implementation to use | the existing scenario's negotiation | holds |
| A-2 | The existing tagged tests for `-5-1`, `-4-2`, `-4-8` drive the form Ze now emits | tags and discrimination records exist (grep, 2026-10-11) | a test that passes because the form is unreachable is vacuous; AC-7 replaces it | AC-7 | unvalidated |
| A-3 | FRR keeps an IPv4 single-hop session up with IPv6 unicast and capability 77 on a link with no global IPv6 address | the multihop scenario's `frr.conf` records a Cease when FRR has no IPv6 next hop of its own; on a directly attached link FRR has its own link-local, which capability 77 lets it send alone | the scenarios cannot run on a link with no global IPv6 address; the owner is told with the FRR log, and asked which peer to use instead | first step of AC-8 and AC-9: bring the session up and read FRR's log before writing a checker | unvalidated |
| A-4 | A Linux interface with IPv6 enabled holds exactly one link-local address in the common case | the kernel assigns one `fe80::` per interface from the interface identifier; an operator can add more | AC-11's several-addresses refusal fires more often than expected; the Warn names the fix (set `session link-local`) | AC-11 | holds |

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | The `-4-2` re-read finds that a received link-local forwarded under next-hop auto or unchanged reaches a peer more than one hop away again, a regression of D6 | it is a defect in an implemented capability and is fixed in this spec (`ai/rules/completion.md`): the producer changes, the tagged tests go red on the regression and green on the fix, and the row stays met; the ledger is never moved to `{gap}` to absorb it |
| R-2 | AC-8's receive direction rests on row 3 of the unowned-rows spec | the receive direction is proven by that spec's scenario `bgp-linklocal-capability-interface-frr`, and this spec's scenarios assert the send direction only |
| R-3 | AC-9 to AC-12 read the interface name the unowned-rows spec adds to `linkScope` | Depends names it; if that spec has not landed, this one stops before AC-9 and reports, rather than building a second interface read |
| R-4 | The new arm changes the next hop on a rail where today the route is withheld, so a peer that negotiated 77 starts receiving routes it did not before | that is the purpose; the arm fires only where today's answer is "withheld" (IPv4 endpoint, IPv6 NLRI, no global on the interface), so no route that is sent today changes its next hop. AC-12 proves the unchanged cases |

## Blast Radius

| Area | Change |
|------|--------|
| `link_scope.go` | the snapshot keeps the session interface's link-local addresses and a global-IPv6 flag |
| next-hop self (`peer.go` `resolveNextHop`, `peer_forward_facts.go` `precomputeNextHop`) | one new arm in the helper both use |
| tests and the ledger | AC-7 re-read; new unit tests; two interop scenarios; summary rows `-4-3`, `-2-1` and the Enrolment reason |
| `forward_next_hop.go` | only if the `-4-2` re-read finds a regression (R-1) |

No config, YANG, CLI or wire-format change.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An operator sets `link-local-nexthop` on a peer, the peer advertises capability 77, Ze relays with `next-hop unchanged` | → | `Peer.linkLocalOnlyNextHopPermitted`, `buildMPReach` | `bgp-linklocal-only-next-hop-frr` interop scenario |
| A config `update` block announces an IPv6 route with next-hop self to an external peer over an IPv4 session on a link with no global IPv6 address | → | the next-hop self helper's new arm, `linkScope` | `bgp-linklocal-own-next-hop-frr` interop scenario, and `TestLinkLocalOwnNextHopOverIPv4Session` |
| The same to an internal peer for a directly connected route | → | the same arm (`-4-3`) | `bgp-linklocal-own-next-hop-frr` (IBGP session), `TestLinkLocalOwnNextHopInternalDirectlyConnected` |

AC-7 has no row here: it adds no entry point, it re-reads tests over existing
producers (round-3 N3).

## Acceptance Criteria

| ID | When | Then |
|----|------|------|
| AC-7 | `-5-1`, `-4-2` and `-4-8` are re-read against the producers the summary names | each row's tagged test is shown, by its discrimination record and by reading its body, to drive the form Ze now emits rather than rest on the form being unreachable; a test that does not is rewritten and re-recorded. For `-4-2` the re-read covers the D6 forms: a received link-local forwarded under next-hop auto and under unchanged, to a peer on the shared segment and to one more than one hop away. If a form regressed, the producer is fixed in this spec and the tagged test proves it (R-1); the row stays met. `./le rfc check` exits clean for the stem. `-6-1` and `-1-2` are not in this list |
| AC-8 | The interop scenario `bgp-linklocal-only-next-hop-frr` runs: the injector (AS 65004, the `bgp-linklocal-only-multihop-withdraw-frr` pattern), Ze (AS 65001) and FRR (AS 65002) on one shared link, so both sessions are EBGP and §4 item 2 is the arm exercised (round-5 N1), each session over IPv4 addresses on that link (single hop), the IPv6 unicast family negotiated with capability 77 on both sessions, no global IPv6 address on the link, Ze relaying with `next-hop unchanged`; the injector announces a prefix whose MP_REACH Next Hop is 16 octets holding only the injector's own link-local on the shared interface | Ze relays the route to FRR with that 16-octet link-local unchanged: the checker asserts the captured Next Hop field of Ze's UPDATE to FRR equals the injector's link-local, and FRR installs the route via that link-local through the shared interface, read from FRR's RIB and kernel. The receive direction, FRR's 16-octet link-local next hop installed by Ze with the session interface, is AC-3 of `plan/immediate/spec-bgp-unowned-rfc-rows.md` and its scenario `bgp-linklocal-capability-interface-frr`; this scenario does not repeat it. The revert, rebuild, red, restore, green walk is recorded |
| AC-9 | Ze (AS 65001) announces an IPv6 unicast route with next-hop self to an external peer (FRR, AS 65002) over an IPv4 session on a shared link whose interface on Ze holds a link-local address and no global IPv6 address, capability 77 negotiated; on the announce rail (config `update` block) and on both forward rails (a route received from another peer, forwarded with next-hop self) | Ze sends the route with a 16-octet Next Hop holding its own link-local on the session's interface (§4 item 2, default arm, "MAY be the only next hop address"). FRR installs it via that link-local through the shared interface (`bgp-linklocal-own-next-hop-frr`: the checker reads Ze's interface link-local from the container, asserts the captured Next Hop field equals it, and reads FRR's RIB and kernel). Negative polarity: with capability 77 not negotiated on the same session the route is withheld as today, no NOTIFICATION, and the existing skip log names the reason |
| AC-10 | The same link, an internal peer (FRR, AS 65001, IBGP), and a route directly connected to Ze (an IPv6 prefix on another Ze interface, announced with next-hop self) | the Next Hop includes Ze's own link-local, here alone because the interface holds no global IPv6 address to put before it (`-4-3`, §4 item 1). `-4-3` is re-recorded met for this case with a tag on `TestLinkLocalOwnNextHopInternalDirectlyConnected` and a discrimination record from `./le rfc discriminate-record` (red with the arm removed: the route is withheld) |
| AC-11 | The arm's conditions hold except that Ze cannot name one link-local: the session's interface holds none (IPv6 disabled on it), or holds several and `session link-local` is not set, or `session link-local` is set and the interface does not hold it | the route is not announced to that peer ("If, after completing these procedures, there are no IPv6 next hop addresses included in the next hop, the BGP route MUST not be advertised to its peer", §4), the session stays up, and one rate-limited Warn per peer and cause names the peer, the interface, the reason and the fix: "interface <name> has no IPv6 link-local address; enable IPv6 on it, or give it a global IPv6 address", "interface <name> has N link-local addresses; set `session link-local` to one of <list>", "`session link-local` <addr> is not on interface <name>; set it to an address on <name>" (the last is the unowned-rows I4 Warn, reused) |
| AC-12 | Each case the arm must not touch | unchanged from today, one test case each: the interface holds a global IPv6 address (the global path, RFC 2545 Section 3, stays); the peer is in no connected prefix, so the snapshot names no interface (not one hop, §4 item 3 "Link-Local IPv6 next hops MUST NOT be included": withheld as today); a global-IPv6 session (the endpoint is the next hop); a link-local transport session (the endpoint is already the link-local, `TestNextHopSelfLinkLocalEndpointNeedsTheCapabilityOnTheForwardRails` stays green); an IPv4 NLRI (IPv4 next hop self); an IPv4 NLRI with an Extended Next Hop pair negotiated keeps the IPv4 endpoint, because an IPv4 next hop is present, so §5's 16-octet form is not originated; an explicit `fe80::` next hop stays refused at the route-level entry points |

## End-to-End User Stories

| Story | Path |
|-------|------|
| An operator runs Ze as a route server or transit on a shared link with IPv4 sessions, carries IPv6 unicast routes, and configures no global IPv6 address on the link; a speaker on that link announces a route with only its link-local as next hop | every speaker advertises 77; Ze relays the route to the other speaker with that 16-octet link-local unchanged, and FRR installs a usable route via the announcing speaker through the link (`bgp-linklocal-only-next-hop-frr`) |
| The same operator announces Ze's own IPv6 prefixes over the IPv4 sessions | Ze sends its own link-local on the shared interface as the only next hop, and FRR installs the routes through the link (`bgp-linklocal-own-next-hop-frr`); if the interface has no link-local, Ze withholds the routes and its Warn names the fix |
| FRR's own 16-octet link-local next hop toward Ze | installed by Ze through the link (`bgp-linklocal-capability-interface-frr`, unowned-rows spec) |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| the existing `-5-1`, `-4-2`, `-4-8` tagged tests, re-read | `internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_*_test.go` | AC-7 | |
| `TestLinkLocalOwnNextHopOverIPv4Session` | `internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_own_test.go` (new) | AC-9 through a real `Peer` with an injected link-scope snapshot, on the announce rail and both forward rails (general and route-server), the pattern of `TestNextHopSelfLinkLocalEndpointNeedsTheCapabilityOnTheForwardRails`: with 77 the Next Hop field is the 16-octet own link-local; without 77 nothing is sent | |
| `TestLinkLocalOwnNextHopInternalDirectlyConnected` | same file | AC-10, `-4-3` tag, both polarities | |
| `TestLinkLocalOwnNextHopRefusals` | same file | AC-11: no link-local, several without the leaf, leaf not on the interface; each withholds and logs its reason once; the leaf held by the interface is chosen among several | |
| `TestLinkLocalOwnNextHopUnchanged` | same file | AC-12, one case per row | |
| `TestLinkScopeInterfaceLinkLocals` | `internal/component/bgp/reactor/link_scope_test.go` | the snapshot records the session interface's link-local set and global flag from one interface read, and refreshes them on an interface change | |

### Interop

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-linklocal-only-next-hop-frr` | `test/interop/scenarios/bgp-linklocal-only-next-hop-frr` | FRR | AC-8: the 16-octet link-local Ze relays unchanged from a speaker on the same link is accepted and installed by FRR | |
| `bgp-linklocal-own-next-hop-frr` | `test/interop/scenarios/bgp-linklocal-own-next-hop-frr` | FRR | AC-9 and AC-10: Ze's own link-local as the only next hop, sent to an EBGP FRR (AS 65002) and an IBGP FRR (AS 65001) over IPv4 sessions on a link with no global IPv6 address, installed by both via the shared interface | |
| `bgp-linklocal-capability-interface-frr` (created by the unowned-rows spec) | `test/interop/scenarios/bgp-linklocal-capability-interface-frr` | FRR | AC-8 receive direction, by reference | |

## Files to Modify

- `internal/component/bgp/reactor/link_scope.go` - the snapshot keeps the session interface's link-local addresses and global-IPv6 flag, read with the interface name the unowned-rows spec adds
- `internal/component/bgp/reactor/peer.go` (`resolveNextHop`) and `internal/component/bgp/reactor/peer_forward_facts.go` (`precomputeNextHop`) - the shared next-hop self helper's new arm, with the §4 item 1 and item 2 quotes above the statement that picks the link-local, and the RFC reference at each call
- `rfc/short/draft-ietf-idr-linklocal-capability.md` - `-4-3` re-recorded with AC-10's tag and record; the Enrolment reason names the new producer; `-2-1` ("SHOULD advertise the Link-Local Next Hop Capability to its peers only when: 1. It is capable of sending IPv6 Link-Local-only next hops for a route") re-read against the new arm; AC-7's corrections; no support-remaining line claims Ze never sends its own link-local alone (round-5 I1)
- `docs/features/rfc-status.md` - where AC-7 or AC-10 changes a claim
- `docs/guide/configuration.md` - the `link-local-nexthop` text gains what Ze sends as its own next hop on a link with no global IPv6 address, and the three Warns
- `docs/architecture/core-design.md` - the next-hop self paragraph the reactor's next-hop code declares: the new arm
- `internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_*_test.go` - AC-7, only a test the re-read finds vacuous
- `internal/component/bgp/reactor/forward_next_hop.go` - AC-7, only if the `-4-2` re-read finds a regression (R-1)
- `internal/le/interoplab/bgp/check_special.go` - map entries for both scenarios
- `docs/architecture/testing/interop.md` - a paragraph for each scenario beside the existing `bgp-linklocal-only-multihop-withdraw-frr` one

## Files to Create

- `test/interop/scenarios/bgp-linklocal-only-next-hop-frr/`
- `test/interop/scenarios/bgp-linklocal-own-next-hop-frr/`
- `internal/le/interoplab/bgp/check_linklocal_only.go` and its `_test.go` - both scenarios' checkers
- `internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_own_test.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no new leaf: automatic when 77 is negotiated (Key Design Decisions); `link-local-nexthop` and `session link-local` exist |
| YANG validation constraints | N-A | none new; the `session link-local` commit check is the unowned-rows spec's |
| YANG custom validators | N-A | none new |
| CLI commands/flags | N-A | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no new runtime dependency; the missing link-local is a per-peer Warn at runtime, because it changes with the interface |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | capability 77 and IPv6 unicast exist |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `docs/guide/configuration.md` (`link-local-nexthop`: Ze's own link-local alone on a link with no global IPv6 address) and the summary's interop evidence |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` |
| 7 | Wire format changed? | No | the 16-octet form exists; Ze now emits it in one more case |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/draft-ietf-idr-linklocal-capability.md` (`-4-3`, `-2-1`, the Enrolment reason, the interop evidence, any claim AC-7 corrects) and `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md`: both scenarios' paragraphs |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`: next-hop self and the link-scope snapshot |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/testing/interop.md`, declared by `check_special.go`; `docs/architecture/core-design.md`, declared by `link_scope.go` and `forward_next_hop.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/configuration.md` and `docs/guide/route-injection.md` mention `link-local-nexthop`; re-read both |

## Implementation Steps

1. AC-7 re-read of `-5-1`, `-4-2`, `-4-8`; correct any vacuous test, and fix a `-4-2` regression at its producer if the re-read finds one. Own commit.
2. A-3: bring up FRR over IPv4 with IPv6 unicast and capability 77 on a link with no global IPv6 address; read its log. If it Ceases, stop and report (A-3 "If wrong").
3. AC-8 scenario `bgp-linklocal-only-next-hop-frr` (relay), with the recorded red. Own commit.
4. AC-9 to AC-12, after the unowned-rows spec's link-scope interface name has landed (R-3): tests first, red; the snapshot fields; the helper arm; green; the `-4-3` discrimination record; docs and summary. Own commit.
5. `bgp-linklocal-own-next-hop-frr`, with the recorded red (arm removed, image rebuilt: FRR receives nothing). Own commit.

## Design Insights

- The feature was built by other work after this spec was written; what remains
  is proof plus one origination case, and a proof that rests on an unreachable
  form proves nothing.
- The missing case is defined by today's answer: the arm fires only where Ze
  withholds the route today, so it adds routes and moves no next hop that is
  already sent.

## Key Design Decisions

- Interop is the evidence for AC-8 to AC-10. A capability-gated wire format
  proven only against Ze proves the encoder agrees with itself
  (`ai/rules/interop-and-goal-validation.md`).
- The sessions run over IPv4 on the shared link. "No global next hop" means no
  global IPv6 address on the session's interface (round-3 I2).
- The receive direction is proven once, by the unowned-rows spec's
  `bgp-linklocal-capability-interface-frr`; a second scenario asserting the same
  install would be a second copy of one proof (round-3 I2).
- AC-8 proves a relay, and stays (owner, 2026-10-11). The draft permits that
  form as a third-party next hop for an external peer: "Similarly, the speaker
  can use the received Link-Local IPv6 address, provided that peer X is
  directly attached." (§4, item 2). Distinct ASNs make both sessions EBGP so
  this is the arm exercised; under IBGP it would be §4 item 1's "MAY include the
  received Link-Local IPv6 nexthop" instead (round-5 N1).
- Ze already sends its own link-local alone over a link-local transport
  (round-5 I1; Task, table). The earlier "Ze never makes its own link-local the
  only next hop" was false and is removed everywhere, including the planned
  support-remaining line.
- Origination over an IPv4 transport is automatic, with no new config leaf.
  The operator opts in by enabling capability 77 (`link-local-nexthop`), and the
  draft makes the form a property of that negotiation: "This section defines
  the behaviors for setting IPv6 next hops when the Link-Local Next Hop
  Capability has been negotiated between two peers. The next hop MAY consist of
  only a Link-Local IPv6 next hop." (§4). A second leaf would let an operator
  negotiate 77 and still withhold every IPv6 route on the link, which helps no
  one, and the alternative today is no route at all.
- Which link-local: the `session link-local` leaf when set and held by the
  interface, because the operator named it; else the interface's one link-local.
  With several and no leaf Ze refuses rather than picks, because a pick the
  operator cannot predict is a silently wrong next hop on the peer
  (`ai/rules/principles.md`, first statement).
- The source is the link-scope snapshot the unowned-rows spec extends (its A-2:
  `net.Interfaces` with each interface's addresses, at establishment, reload and
  interface change, never per UPDATE), not a second read through the iface
  component. One read keeps the interface name and its addresses from the same
  instant; the iface component's `AddrInfo.LinkLocal` uses the same
  `fe80::/10` test.
- The arm is in the one next-hop self helper all rails share, before
  `canUseNextHopFor` and `linkLocalOnlyNextHopRefused`, so the capability test
  stays single (`linkLocalOnlyNextHopRefused`) and the rails cannot disagree.
- "Interface holds no global IPv6 address" is the trigger, from §4 item 1's
  "the Global IPv6 address of the interface ... if present" and item 2's "The
  Global IPv6 address, if one is present, SHOULD be included": when one is
  present, today's global path (with RFC 2545 Section 3's second address) is
  what the draft asks for, so it stays.
- One hop is the snapshot's interface: a peer inside a connected prefix of the
  interface. A peer reached through no connected prefix is treated as more than
  one hop, and §4 item 3 forbids the link-local ("Link-Local IPv6 next hops MUST
  NOT be included"), so the arm does not fire.
- RFC 8950: an IPv4 NLRI keeps its IPv4 next hop self. §5 permits a 16-octet
  link-local for IPv4 NLRI when both capabilities are negotiated, but an IPv4
  next hop is present and natural there, so Ze does not originate that form;
  `linkLocalOnlyNextHopPermitted` keeps deciding whether a received one may be
  relayed.
- `-4-3` is resolved for this case by AC-10 and recorded met with its
  discrimination record. A MUST is never relabeled: if the walk finds a further
  case where `-4-3` binds and Ze sends nothing, it is a counted conformance gap
  reported to the owner, not an exclusion.
- A `-4-2` regression is a defect fixed here, never a ledger move to `{gap}`:
  the gates refuse a tagged `{gap}`, and a met row that broke is a defect in an
  implemented capability (round-3 I1).

## Owner decisions (2026-10-11)

| Question | Owner's words | What it means here |
|----------|---------------|--------------------|
| Round-2 I5: the narrowed skeleton owns no open row; close it or rewrite it? | "Rewrite to AC-7/AC-8" | This spec keeps AC-7 and AC-8; the false claims about capability 77 and the seven rows are removed |
| Round-4 I1: relay a received link-local, or build origination? | "Relay only, for now (Recommended)" | Superseded below: the question rested on "no origination path exists", which round-5 I1 found false for a link-local transport |
| Round-5 I1: Ze sends its own link-local alone over a link-local transport, and not over an IPv4 or global-IPv6 transport on a link with no global IPv6 address; build that case? (Thomas, 2026-10-11) | "Build the missing case" | AC-9 to AC-12 and `bgp-linklocal-own-next-hop-frr`; AC-8's relay stays |

## Known Limitations

- The draft is revision -06 and not yet an RFC. A later revision can change the
  wire format, and the summary records the revision the walk read.
- A global-IPv6 session keeps its global endpoint as the next hop. When the
  peer's global is in a connected prefix, the interface holds a global IPv6
  address and the draft's "if present" keeps it. When the peer is reached
  through no connected prefix (an unnumbered link with routed `/128`s), Ze
  cannot establish one hop from the snapshot and §4 item 3 forbids the
  link-local, so the global goes alone as today.
- The link-local transport path is proven by a unit test
  (`TestNextHopSelfLinkLocalEndpointNeedsTheCapabilityOnTheForwardRails`), not
  by interop.

## RFC Documentation (Scope: protocol)

`rfc/short/draft-ietf-idr-linklocal-capability.md` carries the gated rows.
`rfc/drafts/draft-ietf-idr-linklocal-capability.txt` is the authority; the
summary is derived and is never the evidence. The new arm carries the §4 item 1
and item 2 quotes above the statement that picks the link-local, and each call
names the section (`ai/rules/rfc-compliance.md`, functions added from
2026-09-24).

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
- [ ] AC-7 to AC-12 demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket
