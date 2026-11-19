# Spec: bgp-next-hop-auto-rewrites-for-ebgp

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P1 |
| Release reason | The default next-hop policy must implement its promised eBGP and iBGP behavior. |
| Scope | protocol |
| Depends | - |
| Phase | 2/3 |
| Handoff | - |
| Updated | 2026-11-16 |

Bucket: `plan/release/fixes/`, because the default next-hop behavior is a prerequisite of ordinary best-route advertisement.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The original defect was a disagreement between the documented `next-hop auto`
default and `peer_forward_facts.go::precomputeNextHop`: Auto preserved a
received next hop toward ordinary eBGP. Commit `ae01917413` implemented the
approved default by selecting the existing Self branch. This continuation
completes its boundary, daemon and interoperability proof, removes the unused
helper, and reconciles affected tests and documentation. Live interoperability
and its semantic redwalk passed; final scoped checks, RFC freshness and closure
remain open.

The common-subnet scope pass must preserve the FlowSpec family normalizer's
empty field or permitted single redirect target. Invalid received next-hop
bytes must not restore a field that family processing cleared. Ordinary and
unknown forwarding families retain their existing scope rules (AC-13).

Listener lookup, ownership and dynamic admission also remain in this spec
(AC-14). An interface-read failure must not grant exclusive socket ownership
or prove that no claimant remains. The stored declaration and observed bound
endpoint determine socket reuse across mapped wildcard spellings. Removal
must keep the socket until its claimant set is known to be empty.

Removing the unused `applyNextHopMod` also updates the producer provenance in
`rfc/short/rfc8277.md` for `RFC8277-3.2.1-1`, whose unchanged-next-hop tests now
use live forwarding facts. Package 2 changes only that annotation's producer
references: its stable ID, mandatory level, normative quote, positive-only
classification and reason that there is no formal inverse stay unchanged.

Package 2 also closes the draft Section 4 item 3 test-binding gap found by
independent audit: a received sixteen-octet Link-Local-only next hop is a
distinct input from the pair and configured-Self cases. Dedicated tests cover
borrowed and export-policy-produced Link-Local output toward multihop eBGP, with directly
attached capability-77 survivors and usable Global/Self controls. This supplies
proof for the existing prohibition, without changing protocol policy.

The continuation also repairs the test fixtures that block its reactor-package
validation. Their build contexts must carry the configured AS identity and
their final Session writers must negotiate the AS_PATH width their builders
use, while explicit OLD-speaker and raw two-octet subjects keep their existing
negotiation. The queued replacement test must parse checked UPDATE
sections and prove the replacement announces the exact route and next hop.
These are fixture repairs; the production AS-zero refusal remains unchanged.

That same bounded fixture work includes the eight tagged next-hop units whose
permitted output failed in `job-unit-pkg-5c0fa16d.log`, plus the two adjacent
single-announce callers found by the independent RFC population review.
Local ASN4 negotiation must match their existing four-octet AS_PATH builders
and policy literals, while shared fixtures keep their two-octet and pre-OPEN
callers. Every original refusal, permitted survivor and exact assertion stays.

The RFC4271-5.1.3-1 source audit also exposed a related raw-origination gap:
recognized body and full-packet UPDATEs reached `writeOriginatedRawUpdate`
without the peer-own next-hop prohibition that ordinary origination applied.
This continuation owns its regression, final raw admission and real
daemon/FRR proof. RFC 4271 Section 5.1.3 states: "A route originated by a BGP
speaker SHALL NOT be advertised to a peer using an address of that peer as
NEXT_HOP." Raw's diagnostic purpose supplies no exception. The main thread
observed the forbidden wire output, applied the admission repair, and recorded
semantic breaks for the raw unit and promoted daemon test. The scoped-recipient
repair also passed its forbidden and permitted wire cases with native semantic
records. Raw FRR acceptance and its redwalk remain unrun; the whole-row verdict
stays weak and closure remains open. Existing AC-1 through AC-10 and the
explicit AS_SET raw exception stay.

### BGP-LS next-hop family contract

The shared reactor run also exposed a production defect in BGP-LS forwarding.
For an ordinary eBGP destination with usable local IPv4 address 192.0.2.254,
`applyFactsNextHop` recorded a sixteen-octet mapped IPv6 replacement because
it treated the absent BGP-LS length table as a ban on native IPv4. The
effective-address gate then withheld the announcement as an unusable IPv6
global next hop.
`TestRFC7752UnknownTLVsReachForwardDestinations` and
`TestRFC9552UnknownTypesSurviveARewrittenUpdate` consequently lost MP_REACH;
the internal-peer propagation control passed. Their original fixtures and
assertions remain the required subjects of this repair.

The main thread observed the same semantic red in
`job-next-hop-bgpls-native4-red-dfb078ed.log`: all twelve new Auto/Self/explicit
cases and all four recovery seeds failed, all twelve preserving controls
passed, and the two original external-peer propagation roots failed while
the internal-peer root passed. The producer now uses native IPv4 for an empty
length table, matching `familyAdmitsNextHopOctets` and the typed MP writer's
unrestricted address form. Known profiles and SAFI 128 retain their existing
encoding decisions. Main observed all six selected roots pass in
`job-next-hop-bgpls-native4-green-dfb078ed.log`, including all 28 new cases,
and all seven known-profile controls pass in
`job-next-hop-bgpls-known-profiles-85aefd44.log`. Independent source review
found zero issues against the unchanged post-format source hashes. The new
tests observe in-process Session writes to a `recordingConn` sink. External
TCP peer acceptance and daemon coverage remain separate obligations, together
with scoped lint and documentation checks.

RFC 9552 Section 5.5 states: "If the next-hop length is 4, then the next hop
is an IPv4 address; if the next-hop length is 16, then it is a global IPv6
address; and if the next-hop length is 32, then there is one global IPv6
address followed by an IPv6 link-local address." RFC 7752 Section 3.4 defines
the same plain-family forms. Auto, Self and explicit IPv4 rewrites must select
the native four-octet form for AFI 16388, SAFI 71. Preserving destinations keep
the received IPv4 next hop; peer-own refusal and subsequent usable recovery
still apply on both relays.

The same spec owns the adjacent SAFI 72 framing defect. RFC 9552 Section 5.5
states: "For VPN Subsequent Address Family Identifier (SAFI), as per custom,
an 8-byte Route Distinguisher set to all zero is prepended to the next hop."
The current producer and typed MP encoder recognize RD framing only for SAFI
128. BGP-LS VPN therefore needs its own verified framing contract, including
the single-address 12- and 24-octet forms. The dual-address form remains a
design question: a single RD before the complete 32-octet field gives 40
octets, while VPN-IPv6's per-address RDs give 48. Research must resolve the
BGP-LS contract before implementation or a coverage claim; copying the
VPN-IPv6 exception or its subnet condition does not establish that contract.
AC-12 and this spec remain open through that work.

Owner decision (Thomas, 2026-10-03): change the CODE so `auto` rewrites the
next hop to this speaker's own address for an eBGP peer, single hop and
multihop alike, as the RFC 4271 Section 5.1.3 default. The YANG text stays as
written and becomes true. The work lives in this spec, separate from the
link-local spec `spec-bgp-update-propagation-rfc-defects` (closed 2026-10-21)
(defect D6), which owns the `unchanged` towards multihop case.

Owner decision (Thomas, 2026-11-13): run this spec. Asked by the
deployment-user-journeys session "Run `spec-bgp-next-hop-auto-rewrites-for-ebgp`
(skeleton, P1)? Forwarded routes keep their next hop toward an external peer
under the default, against the YANG text and RFC 4271 Section 5.1.3. The
netlab render works around it explicitly.", he answered "yes". The evidence was
a live netlab lab: r3 refused r2's loopback as the next hop of a forwarded
route ("route ignored: semantically incorrect NEXT_HOP"), commit `ef1a3cf33d`
and `git show a64267bb2f:plan/handover/deployment-user-journeys-2026-11-13/README.md`,
"Progress after the migration".

That commit set `next-hop self` explicitly toward every external peer in the
netlab render (`contrib/netlab/ze/ze.j2`, the `n.type == 'ebgp'` block). It is
a workaround this spec's implementation should make unnecessary, and the
implementation must not break it: under the behaviour table below, `self`
toward eBGP and `auto` toward eBGP send the same address, so the render stays
correct with or without the explicit leaf. The same day Thomas also ruled that
Ze passes elected best routes between peers by default. The
[ordinary selected-route path](../../../docs/architecture/rib-transition.md#ordinary-selected-route-advertisement)
relies on the eBGP next-hop behavior specified here.

### Owner-approved behaviour table

Recorded from the link-local research, approved by Thomas on 2026-10-03.

| Configured mode | Destination | Behaviour | Owner |
|-----------------|-------------|-----------|-------|
| `unchanged` | multihop peer | withhold the route | link-local spec, `spec-bgp-update-propagation-rfc-defects.md` D6 |
| `auto` | eBGP, single hop or multihop | rewrite to self; withdraw only if self has no usable address | this spec |
| `auto` | multihop iBGP, route not locally originated, received next hop has no usable address | withhold unless `self` supplies a usable address; preserve a received usable Global | this spec (see A-3 for the reading) |
| `self`, or route reflector next-hop-self | any | rewrite | unchanged by this spec; already implemented |

### Research facts carried from the owner's brief

| Source | What it does | Where |
|--------|--------------|-------|
| FRR | resets a received link-local next hop and fills its own global address | `bgp_route.c` `subgroup_announce_check`, `bgp_updgrp_packet.c` `bpacket_reformat_for_peer`; commit 003c1ba05ab9 (2015) |
| BIRD | `bgp_use_next_hop` keeps the received next hop for iBGP only when the global is non-zero, for eBGP only on the same interface, otherwise uses self | BIRD `bgp_use_next_hop` (source file not yet read in this checkout) |

These are the owner's research summary. The implementation phase reads the
deciding function in each before the design is presented
(`compare-other-implementations-before-a-protocol-design`), and quotes it here.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/bgp/structural-forwarding.md` - sections "Configured native next-hop self" and "Forwarded routes and next-hop self" describe the next-hop self rails this spec reuses for `auto`
  → Decision: next-hop self is built from the address the peer reaches Ze on (the connected local endpoint), so the announce rail and both forward rails send one address
  → Constraint: when no local address exists, the route is withheld and the withdrawals in the same UPDATE still go; a third-party next hop is never sent in its place
- [ ] `docs/guide/configuration.md` and `docs/guide/bgp-policy.md` - named for `peer_forward_facts.go` in `ai/CODE-TO-DOCS.md`; check each for a statement about `auto`
  → Constraint: a page made wrong by this change is edited in the same work (`ai/rules/documentation.md`)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - Section 5.1.3 rows RFC4271-5.1.3-1, -2, -3 exist; the SHOULD-level defaults this spec implements have no row yet
  → Constraint: RFC 4271 Section 5.1.3, from `rfc/full/rfc4271.txt`, iBGP: "When sending a message to an internal peer, if the route is not locally originated, the BGP speaker SHOULD NOT modify the NEXT_HOP attribute unless it has been explicitly configured to announce its own IP address as the NEXT_HOP."
  → Constraint: Section 5.1.3 item 2, single-hop eBGP, last bullet: "By default (if none of the above conditions apply), the BGP speaker SHOULD use the IP address of the interface that the speaker uses to establish the BGP connection to peer X in the NEXT_HOP attribute."
  → Constraint: Section 5.1.3 item 3, multihop eBGP: "The speaker MAY be configured to propagate the NEXT_HOP attribute." and "By default, the BGP speaker SHOULD use the IP address of the interface that the speaker uses in the NEXT_HOP attribute to establish the BGP connection to peer X."
  → Decision: the single-hop third-party bullets of item 2 ("can use ... provided that peer X shares a common subnet") are permissions, not the default. The owner chose the default for `auto`; `unchanged` remains the configured propagation item 3 permits
- [ ] `rfc/short/draft-ietf-idr-linklocal-capability.md` - Section 4 items 1 to 3, read from `rfc/drafts/draft-ietf-idr-linklocal-capability.txt`
  → Constraint: Section 4 item 2, last bullet: "By default (if none of the above conditions apply), the BGP speaker SHOULD use the IP address of the interface that the speaker uses to establish the BGP connection to peer X in the next hop attribute."
  → Constraint: Section 4 item 3, third bullet: "By default, the BGP speaker SHOULD use the Global IPv6 address of the interface that the speaker uses in the next hop to establish the BGP connection to peer X."
  → Constraint: Section 4 item 1: "If the internal peer is more than one IP hop away, the BGP speaker MUST NOT include a Link-Local IPv6 next hop." and Section 4: "If, after completing these procedures, there are no IPv6 next hop addresses included in the next hop, the BGP route MUST not be advertised to its peer."
- [ ] `rfc/short/rfc7947.md` - route server clients
  → Constraint: RFC 7947 Section 2.2.1: "As the route server does not participate in the actual routing of traffic, the NEXT_HOP attribute MUST be passed unmodified to the route server clients, similar to the "third-party" next-hop feature described in Section 5.1.3. of [RFC4271]." `auto` towards an RS client MUST keep the received next hop

**Key insights:**
- The rewrite machinery already exists: `auto` towards eBGP needs the same wire form `self` builds, including the withheld outcome when the session has no local address.
- `precomputeNextHop` now selects Self for Auto only when the destination is ordinary eBGP. Auto toward internal peers and RS clients shares Unchanged's no-op mode.
- The unused `applyNextHopMod` in `reactor_api_forward.go` was removed in package 2. Its test-only callers now use the live facts and policy producers.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - leaf `next-hop` under `session`: a union of an IP address and the enum `self`, `unchanged`, `auto`; **default `"auto"`**; description and help both say auto rewrites for eBGP
- [ ] `internal/component/bgp/reactor/peer_settings.go` - constants `NextHopAuto` (0), `NextHopSelf`, `NextHopUnchanged`, `NextHopExplicit`; the `NextHopAuto` comment and the `NextHopMode` field comment both say "rewrite for eBGP, preserve for iBGP"
- [ ] `internal/component/bgp/reactor/config_nexthop_form.go` - `applyNextHopMode` parses the leaf into `PeerSettings.NextHopMode`
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - `precomputeNextHop` selects Self for ordinary eBGP Auto and preserves internal/RS-client Auto. Self reads the connected local endpoint, then `LocalAddress`, and sets the withheld flag when neither exists. `applyFactsNextHop` records the family-specific rewrite.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - the forward loop applies facts, normalizes effective scope, and calls `egressNextHopWithheld` before output. The unused `applyNextHopMod` has been removed.
- [ ] `internal/component/bgp/reactor/forward_rs.go` - the native relay uses the same facts and admission rules. The next-hop transparency comment cites RFC 7947 Section 2.2.1.
- [ ] `internal/component/bgp/reactor/link_scope.go` - `applyLinkLocalNextHop` raises the self and explicit IPv6 modes to the two-address form under the RFC 2545 Section 3 condition
- [ ] `internal/component/bgp/reactor/forward_next_hop.go` - `applyEgressNextHopScope` trims a pair when its common-subnet condition does not hold. `egressNextHopLinkLocalOnlyOffLink` and `egressNextHopWithheld` withhold an effective Link-Local-only address from an off-link peer.

**Behavior to preserve:**
- `auto` towards an iBGP peer keeps the received next hop (RFC 4271 Section 5.1.3 item 1), except where the link-local spec withholds.
- `auto` towards an RS client keeps the received next hop (RFC 7947 Section 2.2.1), on both rails.
- `unchanged` retains the received next hop; `self` and an explicit address select their configured address with the family's valid encoding and existing admission rules.
- The self-withheld outcome: no local address means the announcement is withheld, the withdrawals in the same UPDATE still go, and the warning names the peer.
- Route reflection attribute handling, AS_PATH prepend, and the link-local strip for multihop peers.

**Behavior to change:**
- `auto` towards a non-RS-client eBGP peer, single hop or multihop, rewrites the next hop to this speaker's address on that session, exactly as `self` would.
- `auto` towards a multihop iBGP peer for a route not locally originated withholds where draft Section 4 item 1 leaves no next hop (A-3).
- The dead `applyNextHopMod` and its false comment are removed, under `ai/rules/no-layering.md`.
- The RFC 7947 section cite in `forward_rs.go` is corrected to Section 2.2.1.

## Existing Mechanisms

| Mechanism | Location | Current behavior | Use in this spec |
|-----------|----------|------------------|------------------|
| Destination next-hop selection | `peer_forward_facts.go`, `precomputeNextHop` | Auto selects the existing Self branch for ordinary eBGP; iBGP and RS clients preserve the received next hop | Keep the committed producer; complete its wiring and preservation proof |
| Effective IPv6 next-hop scope | `link_scope.go`, `applyLinkLocalNextHop`; `forward_next_hop.go`, `egressNextHopWithheld` | Adds a permitted local link-local half and withholds an unusable off-link link-local next hop | Extend tests, without adding another scope decision |
| Two forwarding paths | `reactor_api_forward.go`, `forwardUpdateSection`; `forward_rs.go`, `reactorForwardRSSection` | Both use the same precomputed next-hop facts | Exercise both paths independently |
| Socket-output fixtures | `next_hop_auto_relay_test.go`; `relay_proof_helpers_test.go` | Decode actual output for ordinary, RS-client, internal and explicit-mode controls | Reuse for the missing conditional IPv6 and multihop iBGP cases |
| Functional and interop harnesses | `test/plugin/`; `internal/le/interoplab/bgp/` | Run the real daemon and inspect peer-visible output | Add the two planned daemon tests and the FRR scenario through their existing registries |
| Fixture capability negotiation and checked UPDATE sections | `forward_order_regression_test.go`, `negotiateForwardTestFamilies`; `internal/core/bgp/wire/update_sections.go`, `ParseUpdateSections` | Publish real family/ASN4 negotiation and validate section bounds before reading wire fields | Reuse for the affected fixture writers and queued replacement output; preserve explicit OLD-speaker and raw subjects |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `session { next-hop auto; }`, or the leaf absent, since `auto` is the default.
- Wire: an UPDATE received from one peer and relayed to another.

### Transformation Path
1. Config parse: `applyNextHopMode` stores the mode in `PeerSettings`.
2. Session establishment: forward facts are built per destination; `precomputeNextHop` fixes the next-hop wire form from the mode, the eBGP flag and the connected local endpoint; `applyLinkLocalNextHop` may raise it to the two-address form.
3. Relay: the forward loop (`reactor_api_forward.go`) or the RS rail (`forward_rs.go`) records the next-hop operations through `applyFactsNextHop`, then the link-local strip, then the withheld gate.
4. Egress: the modification accumulator rewrites attribute 3 and attribute 14 in the outgoing UPDATE.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → reactor | `PeerSettings.NextHopMode` | Both promoted daemon tests exercise the omitted Auto default or explicit Auto |
| Reactor → wire | `ModAccumulator` operations on attributes 3 and 14 | IPv4 and IPv6 socket tests cover cached and native relays |
| Ze → peer daemon | NEXT_HOP and MP_REACH_NLRI next-hop field | Portable daemon wiring and both semantic redwalks passed; legal-topology FRR acceptance and its semantic redwalk passed |

### Integration Points
- `precomputeNextHop` - gains the `auto` towards eBGP case; it already holds the self branch to reuse
- `applyFactsNextHop` - the AC-12 repair retains native IPv4 when the length table is empty; main observed semantic RED before the repair and targeted GREEN afterward, while scoped lint, documentation checks and daemon coverage remain pending
- The withheld gate on both rails - reused for `auto` with no usable local address

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Config selects `PeerSettings.NextHopMode`; both relays use `applyFactsNextHop` and `egressNextHopWithheld` before the writer |
| No unintended coupling (components stay isolated) | Yes | Auto selection remains inside the reactor's existing per-destination facts |
| No duplicated functionality (extends existing, does not recreate) | Yes | `precomputeNextHop` selects the existing Self branch; the unused helper is removed |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Auto adds no new encoder or buffer owner; it uses Self's fixed facts and accumulator operations |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new product surface; `register_nexthop_auto_ebgp.go` registers the checker in `specialCheckers` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Scenario discovery reads the scenario directory and `specialCheckers`; existing config enums and functional suite discovery remain unchanged |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `auto` is the YANG default, so every eBGP session with no `next-hop` leaf changes behaviour | `ze-bgp-conf.yang` leaf `next-hop`, `default "auto"` | the blast radius is smaller than stated | read of the YANG leaf, 2026-10-03 | confirmed |
| A-2 | An RS client is recognized by `PeerSettings.RSClient` and must keep the received next hop under `auto` | `peer_forward_facts.go` sets `rsClient` from `RSClient`; RFC 7947 Section 2.2.1 | the change rewrites next hops at an IXP route server, a MUST violation | `TestNextHopAutoPreservesRSClientAndIBGP`, both paths; promoted RS-client daemon test | confirmed; native RS role/AS-path/session interop passed; exact next-hop preservation is proved separately by socket/daemon tests |
| A-3 | The owner's row "`auto` to multihop iBGP, route not locally originated: withhold unless `self`" applies where the received next hop leaves no usable address for that peer (a link-local-only next hop), not to every such route | draft Section 4 item 1 and its closing sentence; RFC 4271 Section 5.1.3 item 1 says SHOULD NOT modify | withholding every relayed route to a multihop iBGP peer would break iBGP relay outright | `egressNextHopLinkLocalOnlyOffLink`; `TestNextHopAutoInternalMultihopUsesOnlyUsableNextHop` preserves Global and Self recovery | confirmed by the RFC condition and socket proof; no new owner decision needed |
| A-4 | "Self" under `auto` is the same address `self` uses: the connected local endpoint, then the configured local address | `precomputeNextHop` self branch; `connectedLocalAddress` | announce rail and forward rails disagree | `TestNextHopAutoMatchesSelfForEBGP`, both paths and both address sources | confirmed |
| A-5 | Auto uses the existing external-session classification without adding a confederation rule | `buildForwardFacts` reads `PeerSettings.IsEBGP`, the inverse of `IsIBGP`; `isIBGPWith` uses the existing local/migration-AS predicate | a separate Auto classification would disagree with other eBGP behavior | source review of `peer_settings.go` and `session_as_migration.go` | confirmed within the existing session model; no new confederation capability claimed |
| A-6 | The existing Self producer selects a usable wire form for every implemented MP family | `applyFactsNextHop` interprets absence of four octets from `ValidNextHopLens` as a reason to emit mapped IPv6 | ordinary eBGP Auto withholds valid BGP-LS routes | source trace through `attribute.MPNextHopProfile`, `applyFactsNextHop` and `egressNextHopWithheld`; the two existing BGP-LS propagation reds in the shared reactor run | broken; AC-12 owns native IPv4 selection and the related LS-VPN framing repair |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Blast radius: every eBGP relay on the default changes its wire next hop, and tests asserting the received next hop go red | the tests listed under Blast Radius fail on the next-hop field | review each red against the behaviour table; correct the assertion only where the table says the old value was wrong; never weaken a test to clear a red |
| R-2 | Interaction with the link-local spec: both edit `precomputeNextHop`, `egressNextHopGlobalHalf` and the withheld gate | merge conflict, or a D6 test going red after this lands | land in a defined order (Depends set at design); share one predicate for "no usable next hop" rather than two |
| R-3 | RS client regression | an RS client receives Ze's address | A-2 test on both rails, interop `bgp-route-server-frr` |
| R-4 | A session with no usable local address now withholds routes it used to relay | withheld-route warnings on eBGP peers with `local ip auto` before connect | same withheld outcome `self` has today; documented in the YANG description |
| R-5 | An IPv4 route to an IPv6-transport eBGP peer, or the reverse, has no address of the right family to put in the field | a red for mixed-family sessions | decide at design: the self branch's mapped form, or withhold; owner sees the choice |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes relayed to eBGP peers carry a wrong next hop or are withheld; at an IXP, RS clients would receive Ze's address (traffic black-holed through a box that does not forward) |
| How is it reverted? | Single commit revert; an operator can set `next-hop unchanged` to get today's behaviour |
| Who else touches this path? | `spec-bgp-update-propagation-rfc-defects.md` (D6, same functions); `spec-bgp-link-local-only-next-hop.md`; sessions working `internal/component/bgp/reactor/**` |

Tests likely affected, found by grep on 2026-10-03 (candidates to review, not
confirmed reds). Unit tests that run `NextHopAuto` towards an external peer:
- `internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_multihop_test.go`: `TestLinkLocalReceivedPairStrippedForMultihopExternalPeer` asserts the received Global is kept under `auto` towards a multihop eBGP peer; `TestLinkLocalRouteServerStripsReceivedPairForMultihopClient` runs `auto` on the RS rail (must stay unchanged if the client is an RS client)
- `internal/component/bgp/reactor/config_test.go`, `config_nexthop_form_test.go`, `rfc9252_peer_forward_facts_test.go`: reference `NextHopAuto`; check each for a next-hop assertion

Functional `.ci` tests with two or more connections, at least one eBGP session
and no `next-hop` leaf (heuristic: connection count, ASN pairs, leaf absence):
`test/plugin/` `asn4-transcode-pooled-buffer`, `attach-process-reload`,
`attach-process-runtime-subscribe`, `attach-process-send-permission`,
`bgp-local-as-options`, `bgp-rs-asn4-transcode`,
`bgp-rs-community-strip-multi-fastpath`, `bgp-rs-community-strip-multi`,
`bgp-rs-control-community-withdraw-egress`, `bgp-rs-fastpath-ebgp-shared`,
`bgp-rs-reactor-fastpath-fallback`, `bgp-rs-reactor-fastpath`,
`bgp-rs-relay-aspath-transparency`, `control-community-withdraw-egress`,
`med-not-propagated-across-as`, `med-removal-configured`,
`med-removal-export-refused`, `modify-increment-med-from-route-value`,
`prefixsid-ebgp-discard-single-walk`, `prefixsid-ebgp-egress-boundary`,
`remove-private-as-export`, `remove-private-as-replace-peer`,
`rfc4271-partial-unknown-transitive`, `rfc6793-ingest-collapse`,
`rfc6793-narrow-to-old-speaker`, `rfc6793-no-as4aggregator-from-new-speaker`,
`rfc6793-no-as4path-from-new-speaker`, `rfc7606-relay-one-field`,
`wellknown-no-export-withdraw-egress`, `as112-shared-watchdog-group`,
`rfc7606-54-bgpls-override-propagates`,
`rfc7606-54-discard-unrecognized-mup-nlri`,
`rfc7606-54-discard-unrecognized-nlri`, `prefixsid-announce-rail-boundary`.
The `bgp-rs-*` ones change only where a destination is not an RS client.

Interop scenarios (`test/interop/scenarios/`) with two or more Ze peers, at
least one eBGP and no `next-hop` leaf: `as-path-mixed-width-relay-frr`,
`as112-origin-as-frr`, `bgp-addpath-rail-agreement-speaker`,
`bgp-addpath-readvertise-collision-frr`, `bgp-aggregator-as4-downgrade-bird`,
`bgp-attribute-default-localpref-gobgp`, `bgp-local-pref-strip-gobgp`,
`bgp-med-across-as-gobgp`, `bgp-med-increment-gobgp`,
`bgp-med-remove-configured-gobgp`, `bgp-relay-withdraw-shape-frr`,
`bgp-remove-private-as-as4path-frr`, `bgp-remove-private-as-frr`,
`bgp-rfc7606-relay-shape-frr`, `bgp-rfc7606-speaker-dup-attr`,
`bgp-rfc7606-typed-nlri-discard`, `bgp-rfc7999-blackhole-frr`,
`bgp-role-otc-withdraw-frr`, `bgp-route-server-frr`,
`bgp-send-community-suppress-frr`, `bgp-speaker-two-instance`,
`bgp-triangle`, `bgp-wellknown-noexport-frr`, `frr-software-version`. Ten of
these configure an RS client and should not change; the rest are reviewed at
implementation for a next-hop assertion.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `next-hop` leaf absent (default `auto`), route relayed to an eBGP peer | → | `precomputeNextHop` then `applyFactsNextHop` on the forward rail | `test/plugin/nexthop-auto-ebgp-rewrite.ci` |
| `next-hop auto`, route relayed to an RS client | → | `forward_rs.go` keeps the received next hop | `test/plugin/nexthop-auto-rs-client-unchanged.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IPv4 route received from peer A, relayed under `auto` to a single-hop eBGP peer B | B receives NEXT_HOP equal to Ze's address on the B session |
| AC-2 | IPv6 route, same as AC-1 | B receives an MP_REACH next hop whose global is Ze's address on the B session; a link-local is added only under the RFC 2545 Section 3 condition |
| AC-3 | Route relayed under `auto` to a multihop eBGP peer | B receives Ze's address on that session, global only |
| AC-4 | `auto` towards eBGP, session has no usable local address | announcement withheld, withdrawals in the same UPDATE still sent, warning names the peer |
| AC-5 | Route relayed under `auto` to an iBGP peer | received next hop kept, except the A-3 case |
| AC-6 | Route relayed under `auto` to an RS client, either rail | received next hop kept |
| AC-7 | `auto` towards a multihop iBGP peer, route not locally originated, received next hop leaves nothing usable (A-3) | route withheld unless `self` is configured |
| AC-8 | `unchanged`, `self`, explicit address | Preserve each mode's configured meaning: Unchanged retains the received next hop, Self uses the session's local address, and explicit uses its configured address, with the family's valid wire encoding and existing admission rules |
| AC-9 | Received or export-policy-produced sixteen-octet Link-Local-only next hop toward multihop eBGP, with capability 77 negotiated | No Link-Local is advertised; a previously advertised prefix is withdrawn. Usable Global and explicit-Self recovery remain advertisable, and a directly attached capability-77 recipient may receive the Link-Local-only form |
| AC-10 | Existing reactor fixtures whose builders emit four-octet AS_PATH, including both queued connection generations, and adjacent writer controls with MP traffic | Their build contexts carry the configured AS identity and their final Session writers have matching real ASN4 negotiation; the original wire, split, refusal, selection and warning assertions pass across the derived caller population. Explicit OLD-speaker and raw two-octet controls retain their subjects. Checked queued output has no withdrawn routes and announces the exact NLRI with the replacement connection's next hop; malformed or withdrawal-only output fails without a test-side slice panic. The VPNv4 withdrawal writer control explicitly negotiates its actual family while retaining all four rails, exact Compatibility bytes and borrowed-source checks; other shared-helper callers keep their existing family negotiation |
| AC-11 | Valid locally originated raw UPDATE, body and full-packet APIs, with NEXT_HOP equal to the recipient's address; scoped IPv6 recipients on raw and typed writers | The forbidden announcement never reaches the wire. Original explicit withdrawals survive; pure withdrawal and EOR remain usable. An otherwise identical valid third-party next hop still advertises the exact prefix and address. A refused family does not suppress an unrelated permitted family. For a configured scoped IPv6 recipient, wire-address identity ignores the interface zone while configuration, peer key and socket scope retain it. A valid MP32 global-plus-link-local pair naming that recipient is withheld on both raw forms and typed SendUpdate; changing only its link-local slot to a different address preserves the complete MP32 field and exact IPv6 prefix. Ordinary typed cases remain covered; real daemon and FRR decode evidence cover both raw forms before whole-row rejudgment |
| AC-12 | BGP-LS received with a native IPv4 next hop, relayed on cached and native paths to directly attached or multihop destinations; Auto, Self, explicit and preserving roles; adjacent LS-VPN next-hop framing | Plain BGP-LS Auto/Self/explicit IPv4 replacements reach the socket as exactly four native octets with the exact announced NLRI. Internal and RS-client Auto and explicit Unchanged retain the received four octets. After a useful announcement, an explicit recipient-owned next hop withdraws that exact NLRI, and corrected Auto reannounces it. The original RFC7752/RFC9552 unknown-type propagation tests retain all assertions and pass. LS-VPN single-address output carries the required zero RD and exact 12- or 24-octet field; its dual-address layout and scope require a verified family contract before implementation and remain owed by this criterion |
| AC-13 | FlowSpec or FlowSpecVPN family normalization followed by common-subnet scope processing, with ignored invalid next-hop bytes or a permitted single redirect target | An empty next-hop field stays empty. A permitted four- or sixteen-octet redirect target retains its selected form. The exact NLRI and borrowed input remain unchanged. Ordinary IPv6 and unknown forwarding families retain their useful scope transformations |
| AC-14 | Interface lookup fails while static peers or dynamic groups share a listener, including retained mapped IPv4 wildcard declarations and mixed wildcard bindings | An unresolved alias never grants dedicated attribution or a known empty claimant count. The stored declaration and observed endpoint identify socket reuse, removal eligibility and dynamic admission. Known co-owners retain the socket, and the last known claimant releases it. Remote-address attribution remains available when ownership is unknown. Other ports and global listeners retain their existing behavior |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator relays routes from an upstream to an eBGP customer with no `next-hop` leaf | wire → forward rail → `precomputeNextHop` → accumulator → wire | `test/plugin/nexthop-auto-ebgp-rewrite.ci` |
| 2 | IXP operator runs Ze as route server with default `auto` | wire → RS rail → wire, next hop untouched | `test/plugin/nexthop-auto-rs-client-unchanged.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNextHopAutoBGPLSNativeIPv4`, `TestNextHopAutoBGPLSPreservesReceivedIPv4`, `TestNextHopAutoBGPLSPeerOwnRecovery` | `next_hop_auto_bgpls_test.go` | AC-12 native four-octet replacements, preserving roles and seeded refusal/recovery on both relays and both directness conditions, observed through in-process Session writes to `recordingConn` | Semantic RED preceded the repair. All 28 cases and all three original propagation roots pass in `job-next-hop-bgpls-native4-green-dfb078ed.log`; seven known-profile roots pass in `job-next-hop-bgpls-known-profiles-85aefd44.log`. Independent source review found zero issues. Scoped lint, documentation checks and daemon coverage remain pending. Original tagged tests remain unchanged; no new RFC credit |
| `TestScopedNextHopPeerSettingsRetainsZone`, `TestScopedNextHopComparisonIgnoresTransportZone`, `TestRFC4271ScopedNextHopWithholdsPeerOwn`, `TestRFC4271ScopedNextHopPreservesThirdParty` | `rfc4271_scoped_next_hop_test.go` | AC-11 static settings retain transport scope; valid MP32 on both raw forms and typed SendUpdate refuses the scoped recipient's address while the one-address permitted twin retains exact next-hop and prefix bytes | Observed wire red preceded the shared comparison repair. All four units and all six writer cases PASS in `job-next-hop-scoped-recipient-green-9e1534c8.log`; positive and negative semantic records are observed in `job-next-hop-scoped-positive-proof-e0332b30.log` and `job-next-hop-scoped-negative-proof-e9e8f7ed.log` |
| `TestRawOriginatedUpdateWithholdsPeerOwnNextHop` and dedicated withdrawal/EOR controls | `rfc4271_raw_originated_next_hop_test.go` | AC-11, both real raw API forms with valid negotiated AS_PATH and exact final wire assertions; preserve original withdrawals and permitted output | Wire red preceded the admission repair; the raw primary/error population passes and the tagged unit has native positive/negative semantic records. The later 22-root preservation run also passes in `job-next-hop-scoped-existing-controls-f81cecaa.log` |
| Five raw split-error and duplicate-MP controls | `raw_originated_error_test.go` | AC-11 preservation on partial failure: actual prior ownership, exact withdrawal flush, useful later output, both split/transport error causes, borrowed generic input, bounded duplicate-MP refusal and opaque flush before retirement | All five units/nine cases PASS in `job-next-hop-raw-error-controls-32872e88.log`; independent missing-regression finding closed in the owning session state. This is error-path evidence, not whole-row RFC credit |
| `TestNextHopAutoMatchesSelfForEBGP`, `TestNextHopAutoPreservesRSClientAndIBGP`, `TestNextHopAutoChangePreservesExplicitModes`, `TestNextHopAutoWithoutOwnAddressPreservesMixedWithdrawals` | `next_hop_auto_relay_test.go` | AC-1, AC-3 through AC-6, AC-8, both paths | focused run passed; repaired affected caller populations now pass; fresh full reactor-package run remains owed |
| `TestNextHopAutoInternalMultihopUsesOnlyUsableNextHop`, `TestNextHopAutoIPv6PairFollowsConnectedSubnet`, `TestNextHopAutoMissingOwnAddressWarnsForThePeer` | `next_hop_auto_boundaries_test.go` | AC-2, AC-4 warning, AC-7, both paths | baseline and three semantic redwalks passed |
| `TestDraftBareLinkLocalReceivedMultihopCached`, `TestDraftBareLinkLocalReceivedMultihopNative` | `draft_ietf_idr_linklocal_capability_single_next_hop_test.go` | AC-9 received Link-Local replacement and Global/Self survivors, one independently proven unit per forwarding path | main-thread baselines, semantic redwalks and native records completed |
| `TestDraftBareLinkLocalPolicyMultihopCached`, `TestDraftBareLinkLocalPolicyMultihopNative` | same dedicated file | AC-9 reachable raw-policy Link-Local output with valid Global policy output before and after | main-thread baselines, semantic redwalks and native records completed |
| `TestDraftBareLinkLocalAttachedCached`, `TestDraftBareLinkLocalAttachedNative` | same dedicated file | AC-9 directly attached capability-77 survivor, received and policy-produced Link-Local | main-thread baselines, semantic redwalks and native records completed |
| Four AS4 forwarding units through `collapseDestPeer`, `TestRFC8950WithheldRouteWithdrawnFromPeerLackingThePair`, `TestRFC9830IPv4SRPolicyIPv6NextHopIsForwardedWithoutThePair` | `as4_ingest_collapse_test.go`, `forward_withhold_withdraw_test.go`, `extended_next_hop_family_scope_test.go` | AC-3, AC-6, AC-8 and the AC-10 affected-fixture review: supply the established Auto recipient's usable local address; explicitly preserve the received IPv6 field when it is the test's subject. Retain every original assertion and the RS-client control | Six independent native fixture approvals recorded; exact reviewed patch applied. All six focused callers PASS in `job-unit-pkg-bf94097f.log` |
| Existing 21 failing admission-fixture units and their complete derived population of 30 tests plus one RS benchmark | The 13 fixture paths below; exact names, producer contracts and commands are in `tmp/session/2026-11-15-153337/state/session-state-bgp-next-hop-auto-rewrites-for-ebgp-b1119c80-3eb1-42ef-a799-d4dc4a95a38e.md`, "Admission fixture proposal" | AC-10 matching ASN4 negotiation, preserved OLD-speaker/raw controls and checked queued replacement output; unchanged assertions must remain discriminating | Approved repair and separately judged context-identity correction applied. All 30 caller tests PASS in `job-unit-pkg-2da61af7.log`; benchmark delivery control PASS for 100/1000/2000 destinations in `job-next-hop-auto-benchmark-control-3009c456.log`. Required RFC refreshes and final scoped checks remain pending |
| Eight tagged next-hop admission/policy/static reds and the adjacent off-link and third-party single-announce callers | `rfc2545_announce_nexthop_guard_test.go`, `rfc2545_peer_send_test.go`, `rfc2545_api_origination_test.go`, `rfc2545_static_origination_test.go`, `rfc2545_third_party_link_local_test.go`; exact ten-unit and two-helper census in the existing session state's "Tagged fixture proposal" | AC-10 matching real ASN4 negotiation; preserve mapped/global/capability-77 controls, policy replacements, healthy siblings, common-subnet polarity and every exact assertion. Shared helpers and existing capability overrides stay unchanged | Exact reviewed patch applied after native approvals; all ten focused callers PASS in `job-unit-pkg-f3909007.log`. Required RFC freshness/proof remains owed |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | the feature takes no numeric input | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `nexthop-auto-ebgp-rewrite` | `test/plugin/` | eBGP relay under default `auto` carries Ze's address | promoted baseline passed after bounded draft stress; native clean-PASS/broken-FAIL semantic redwalk observed |
| `nexthop-auto-rs-client-unchanged` | `test/plugin/` | RS client keeps the received next hop | promoted baseline passed after bounded draft stress; native clean-PASS/broken-FAIL semantic redwalk observed |
| `raw-originated-nexthop-peer-own` | `test/plugin/` | Raw body/full-frame peer-own refusal with exact permitted controls and explicit withdrawal preservation | Promoted after 80 counted host runs; native positive/negative semantic records are written. After fixture preallocation, focused live acceptance PASS 1/1 in 2.3s in `job-next-hop-raw-daemon-final-fixture-f168e4ca.log`; lint remains owed |
| `originated-nexthop-peer-own` oracle repair | `test/draft/plugin/` | Preserve the typed cases, assert each useful announcement and its exact next hop in the same UPDATE, and permit lawful withdrawals of the refused subject | Independently approved draft; acceptance, counted stress, promotion and native semantic records remain owed. The live CI is unchanged |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-nexthop-auto-ebgp-frr` | `test/interop/scenarios/` | FRR | FRR selects Ze's accessible address for a relayed route under default `auto`; Unchanged and iBGP preserve the source hop | native legal-topology acceptance PASS; native semantic redwalk clean PASS/broken FAIL |
| `bgp-self-nexthop-withheld-frr` extension | `test/interop/scenarios/` | FRR | Preserve existing relay cases; inspect FRR's UPDATE decode for raw body/full-frame forbidden prefixes and exact useful controls on the same surviving session | Raw extension and independently approved announcement-oracle repair are written. Oracle-unit execution, native acceptance and semantic redwalk remain unrun |

## Files to Modify
- `internal/component/bgp/reactor/forward_next_hop.go` - AC-11 after the observed scoped-recipient red, compare zone-free wire-address identity in `nextHopValue.has` while retaining configured transport scope
- `internal/component/bgp/reactor/rfc4271_attr_flags_egress_test.go` - AC-10 adjacent writer fixture: allow explicitly requested additional negotiated families while retaining the existing IPv4/ASN4 default and every shared writer assertion
- `internal/component/bgp/reactor/rfc8277_withdrawal_compatibility_test.go` - AC-10 adjacent VPNv4 withdrawal control: explicitly negotiate its actual MP family, preserving the input, literal whole-body Compatibility oracle and all four writer rails
- `internal/component/bgp/reactor/session_aigp.go` - after the observed AC-11 red, apply peer-own next-hop admission to recognized raw origination while preserving raw ownership, explicit withdrawals and permitted family siblings
- `docs/architecture/api/update-syntax.md` - document that recognized raw UPDATE origination also owes RFC 4271 Section 5.1.3 admission
- `docs/architecture/buffer-architecture.md` - document the retained pooled input required when a refused raw family is partitioned without losing permitted siblings
- `docs/architecture/edge-cases/extended-message.md` - document the retained mixed-input bound and each output chunk's negotiated send limit
- `internal/le/interoplab/bgp/check_rfc.go` and the registered `bgp-self-nexthop-withheld-frr` scenario's source inputs - extend actual FRR proof with both raw carriers while preserving all existing relay assertions; retain the separate typed daemon test unchanged
- `internal/component/bgp/reactor/peer_forward_facts.go` - `precomputeNextHop` splits `auto` by eBGP and RS-client flags; the no-op mode comment
- `internal/component/bgp/reactor/reactor_api_forward.go` - `applyNextHopMod` dead code and its false `auto` comment
- `internal/component/bgp/reactor/forward_rs.go` - RFC 7947 section cite corrected
- `internal/component/bgp/reactor/peer_settings.go` - comments checked against the new behaviour
- `contrib/netlab/ze/ze.j2` - reconcile the external-peer comment with the landed Auto default; preserve the explicit Self setting and all rendered behavior
- `internal/component/bgp/yang/ze-bgp-conf.yang` - description gains the RS-client exception and the withheld outcome
- Tests listed under Blast Radius, where the behaviour table says the old assertion was wrong
- `docs/architecture/bgp/structural-forwarding.md` - next-hop sections cover `auto`
- `rfc/short/rfc8277.md` - package 2 repoints the unchanged-next-hop annotation's producer provenance from the deleted helper to live facts and the MP_REACH handler, without changing its normative scope or polarity
- `internal/component/bgp/reactor/as4_ingest_collapse_test.go` - complete the established ordinary eBGP Auto fixture with a usable local address for its four derived AS4 forwarding units; preserve the RS-client role and every AS-width assertion
- `internal/component/bgp/reactor/forward_withhold_withdraw_test.go` - select Unchanged only for the RFC8950 paired/unpaired recipient test so its received IPv6 capability subject and exact withdrawal assertions remain intact
- `internal/component/bgp/reactor/extended_next_hop_family_scope_test.go` - select Unchanged only for the RFC9830 IPv6-next-hop SR Policy test, retaining its exact wire and sibling-withdrawal assertions on both forwarding paths
- `internal/component/bgp/reactor/announce_facts_partition_test.go` - give the recording Session the exact already-negotiated capabilities, including its OLD-speaker case
- `internal/component/bgp/reactor/forward_rs_selection_test.go` - match the ASN4 recording writer to its existing family set and refuse an unsupported ASN2 fixture before creating the Session
- `internal/component/bgp/reactor/neighbor_table_unread_test.go` - negotiate ASN4 only for the affected warning unit, retaining its exact wire and per-peer warning assertions
- `internal/component/bgp/reactor/rfc2545_ordinary_joint_subnet_test.go` - advertise the ASN4 capability already declared by the fixture's send context
- `internal/component/bgp/reactor/ordinary_growth_test.go` - retain ASN4 negotiation in the 65535-octet override alongside Extended Message
- `internal/component/bgp/reactor/peer_initial_sync_test.go` - match the default-originate helper's Session negotiation to its four-octet builder; leave the shared initial-sync helper unchanged
- `internal/component/bgp/reactor/reactor_api_batch_report_test.go` - match established commit-fixture Sessions to their ASN4 send context and existing families; carry the configured local/peer AS identity into the commit builder's context
- `internal/component/bgp/reactor/peer_static_group_next_hop_test.go` - negotiate the grouped static unit's actual ASN4/family inputs while retaining its capability-5 overrides and exact output assertions
- `internal/component/bgp/reactor/peer_static_restore_test.go` - negotiate ASN4 narrowly in the two configured-route restoration units
- `internal/component/bgp/reactor/peer_static_wire_test.go` - negotiate ASN4 narrowly in the running-session static-delta unit
- `internal/component/bgp/reactor/policy_final_test.go` - negotiate ASN4 narrowly in the policy-final static-reload unit
- `internal/component/bgp/reactor/policy_reconcile_test.go` - negotiate ASN4 narrowly in the accepting-policy removal unit
- `internal/component/bgp/reactor/reactor_api_local_address_test.go` - negotiate both queued connection generations and parse checked UPDATE sections while retaining exact replacement NEXT_HOP/NLRI assertions
- `internal/component/bgp/reactor/rfc2545_announce_nexthop_guard_test.go` - locally negotiate ASN4 for the IPv6 refusal matrix's mapped survivor and the accepted global/link-local pair; retain the shared announce helper and all assertions
- `internal/component/bgp/reactor/rfc2545_peer_send_test.go` - locally negotiate ASN4 in the on-link pair and off-link global-only units, retaining their exact connected-subnet fixtures
- `internal/component/bgp/reactor/rfc2545_api_origination_test.go` - append ASN4 to the existing API capability slice and match the queued-forward Session locally, preserving the capability-77 cases, explicit commit context, policy literals, healthy siblings and accounting
- `internal/component/bgp/reactor/rfc2545_static_origination_test.go` - locally negotiate ASN4 in the policy-refusal continuation unit across its original grouping and withdrawal phases, leaving the shared initial-sync fixture unchanged
- `internal/component/bgp/reactor/rfc2545_third_party_link_local_test.go` - locally negotiate ASN4 in both announce arms, preserving neighbor identity, exact third-party link-local and off-link global-only assertions

## Files to Create
- `internal/component/bgp/reactor/next_hop_auto_bgpls_test.go` - AC-12 standalone native IPv4 BGP-LS forwarding, preservation and exact refusal/recovery controls, using existing receive-to-socket helpers
- `internal/component/bgp/reactor/rfc4271_scoped_next_hop_test.go` - AC-11 static scoped settings, shared comparison and exact MP32 peer-own/permitted wire controls on raw body, raw packet and typed SendUpdate
- `internal/component/bgp/reactor/rfc4271_raw_originated_next_hop_test.go` - AC-11 actual-wire raw origination regression and exact permitted, withdrawal and EOR controls
- `internal/component/bgp/reactor/raw_originated_error_test.go` - AC-11 partial split-error, joined flush-error, borrowed generic-input, duplicate-MP termination and actual opaque-output retirement controls
- `internal/component/bgp/reactor/register_originated_next_hop.go` - register the specific diagnostic for a raw originated route refused because its next hop is the recipient's own address
- `internal/test/fixture/register_raw_originated_next_hop.go` - exercise both actual operator raw command forms with permitted before/after routes and original withdrawals
- `test/plugin/raw-originated-nexthop-peer-own.ci` - AC-11 daemon acceptance, developed under test/draft/plugin before native promotion
- `internal/le/interoplab/bgp/check_raw_originated_next_hop.go` and `check_raw_originated_next_hop_test.go` - actual raw CLI transport, exact installed FRR controls, announcement-versus-withdrawal decode and unchanged-session acceptance
- `test/plugin/nexthop-auto-ebgp-rewrite.ci` - functional test, AC-1
- `test/plugin/nexthop-auto-rs-client-unchanged.ci` - functional test, AC-6
- `test/interop/scenarios/bgp-nexthop-auto-ebgp-frr/` - interop scenario
- `internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_single_next_hop_test.go` - dedicated draft4-8 positive and negative bindings for the distinct sixteen-octet Link-Local-only input and reachable raw-policy replacement, with independently proven units for each forwarding path

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new leaf; description text only |
| YANG validation constraints | N-A | no leaf added or retyped |
| YANG custom validators | N-A | none needed |
| CLI commands/flags | N-A | no CLI change |
| CLI grammar (keyword before value) | N-A | no CLI change |
| Editor autocomplete | N-A | enum unchanged |
| Functional test for new RPC/API | N-A | no RPC; functional tests listed above |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | No | to confirm at design whether a withheld counter exists for self |
| BGP family surface (new SAFI / capability / attribute) | N-A | no new family |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | behaviour fix of an existing default |
| 2 | Config syntax changed? | No | syntax unchanged; `ze-bgp-conf.yang` description edited |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No change needed | `docs/guide/configuration.md` and `docs/guide/bgp-policy.md` contain no stale Auto default claim |
| 7 | Wire format changed? | No | wire format unchanged; the value sent changes |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4271.md` Section 5.1.3 default rows; `rfc/short/draft-ietf-idr-linklocal-capability.md`; `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` has no Auto-default claim to change |
| 12 | Internal architecture changed? | Yes | `docs/architecture/bgp/structural-forwarding.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none planned |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `structural-forwarding.md` anchors live facts, effective scope and the new receiver checker; executed document check found no owned next-hop anchor failure, but the whole gate remains open on unrelated IRR anchors |
| 17 | Existing docs show config/CLI/API examples for this area? | No syntax change | Existing explicit Self remains valid; `contrib/netlab/ze/ze.j2` retains it and corrects its obsolete default comment |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- write the two functional tests and confirm they are red against HEAD
   - Tests: `nexthop-auto-ebgp-rewrite`, `nexthop-auto-rs-client-unchanged`
   - Files: `test/plugin/`
   - Verify: the eBGP test fails because the next hop is the received one; the RS test passes (it guards the preserve case)
2. **Phase: auto towards eBGP** -- split the mode in `precomputeNextHop`, reuse the self branch and its withheld outcome
   - Tests: unit tests for AC-1 to AC-6, AC-8
   - Files: `peer_forward_facts.go`, `reactor_api_forward.go`, `forward_rs.go`
   - Verify: tests fail, implement, tests pass
3. **Phase: multihop iBGP withhold (AC-7)** -- the RFC's scope condition determines A-3; preserve useful Global output and explicit-Self recovery
4. **Phase: test review** -- walk the Blast Radius list, correct assertions the behaviour table proves wrong
5. **Phase: interop and docs** -- FRR scenario with a recorded red, YANG description, architecture page, RFC summaries
6. **Phase: BGP-LS family contract (AC-12)** -- standalone untagged regressions and the original two propagation roots have an observed semantic red. The bounded plain-family producer repair has targeted GREEN and a zero-issue independent source review; scoped lint, documentation checks and daemon coverage remain owed. The LS-VPN framing contract remains a separate owed production phase within this spec

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | RS clients keep the received next hop on both rails; `auto` and `self` produce the same address on the same session |
| Naming | no new names expected |
| Data flow | the decision is made once in `precomputeNextHop`; the rails read it |
| Rule: `ai/rules/no-layering.md` | `applyNextHopMod` is deleted, not left beside the live rail |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `auto` rewrites towards eBGP | `nexthop-auto-ebgp-rewrite.ci` passes, red when reverted |
| RS client untouched | `nexthop-auto-rs-client-unchanged.ci` passes |
| Interop | `bgp-nexthop-auto-ebgp-frr` passes, red recorded |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | none new; a withheld route must not leak the received next hop on any rail |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Auto selects an existing per-destination Self representation. The old helper supplied no production behavior, so its tests now reach live facts.
- Receiver acceptance is a separate obligation from exact socket output. The legal-topology FRR run and its semantic redwalk passed independently of the portable daemon tests.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Change the code so `auto` rewrites for eBGP | change the YANG text to say `auto` preserves | Owner decision 2026-10-03: RFC 4271 Section 5.1.3 items 2 and 3 name rewrite-to-self as the default, and the draft Section 4 items 2 and 3 say the same for IPv6 |
| `auto` towards eBGP uses the self address and the self withheld outcome | a separate builder for `auto` | one address per session on every rail; no second copy of the self logic |

## Known Limitations

- The single-hop third-party next hop of RFC 4271 Section 5.1.3 item 2 (keep the received next hop when the peer shares its subnet, as BIRD does on the same interface) is not offered under `auto`. An operator who wants it sets `unchanged`.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Scoped main-checkout checks for the changed behavior pass or carry source-attributed unresolved failures. Agents run no `./le verify` command and test no copy or worktree.
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

## Implementation continuation, 2026-11-15

The owner instructed this session to implement the necessary specs in dependency
order toward 100% RFC 4271 mandatory proof. This spec is the prerequisite of
ordinary selected-route advertisement. Its earlier Current Behavior section
records the original defect; commit `ae01917413` has since implemented the Auto
selection in `precomputeNextHop`. Do not implement that change again.

AC-1 through AC-9 now have concrete socket or daemon test inputs. Package 1 added
the conditional pair, warning and internal-multihop proofs. Package 2 migrated
the dead helper's tests, corrected the affected ordinary external Auto
expectations after independent judgment, and added the six bare-Link-Local
units. Package 3 added the FRR scenario and updated the YANG and architecture
text. AC-10 owns the necessary fixture repairs exposed by the full reactor run;
its approved repairs, identity binding, complete 30-test caller run and benchmark
control passed. Both daemon redwalks, legal FRR acceptance/redwalk and RS role
interop are observed. Fresh lint, full reactor-package/group/plugin runs,
fourteen RFC bindings, final audits/RFC checks and closure remain open.

### Design constraints for the continuation

| Decision | Basis | Preservation requirement |
|----------|-------|--------------------------|
| Extend the landed producer and existing tests | Owner-approved behavior table and `precomputeNextHop` | Do not introduce a second Auto builder or change explicit modes |
| AC-7 withholds only when the received next hop is unusable for that destination | RFC 4271 Section 5.1.3 item 1 and draft Section 4 item 1; `egressNextHopLinkLocalOnlyOffLink` | A usable global next hop toward multihop iBGP remains advertisable; explicit Self with a usable global remains advertisable |
| Auto uses exactly the existing Self family representation | Owner-approved reuse of Self; `applyFactsNextHop` | Preserve existing IPv4, IPv6, VPN and mixed-family admission rather than adding a new fallback |
| Correct an old test only after independent judgment | `ai/META-DECISIONS.md`, Test changes | The Unchanged expectation remains; Auto must prove the approved self address, with a usable local endpoint |
| Reuse existing withdrawal ownership | `egressNextHopWithheld` and the final writer | A new refused route sends nothing; an advertised route is withdrawn; valid siblings continue |

The alternative of a separate Auto encoder is rejected: it would duplicate
Self, address ownership and family admission. No new public wire format, config
leaf, RPC field or command is introduced. The changed-wire census is the
existing legacy NEXT_HOP and MP_REACH next-hop values on cached forwarding,
native forwarding and daemon/FRR reception. Local origination and explicit
modes are preservation surfaces, not new writers.

### Remaining implementation packages

| Package | Work | Evidence |
|---------|------|----------|
| 1: wiring and missing boundary cases | Add Auto internal-multihop, conditional IPv6-pair and missing-address warning tests; draft the two functional tests outside `test/` through `ze-test` | Scoped socket tests; actual daemon output; retain successful global, explicit-Self, RS-client and sibling controls |
| 2: affected tests and dead helper | Independently judge affected RFC-tagged expectations; remove the unused helper and replace its test-only coverage with live producer coverage | Focused affected tests, required judge approvals and re-recorded RFC proofs |
| 3: interop and documentation | Add the planned FRR scenario; reconcile YANG, architecture and guides with the landed behavior | Scenario output, scoped functional/package checks, documentation check and independent Review Gate |

The three packages retain every earlier acceptance criterion and deliverable.
Any newly exposed defect blocking these criteria is fixed in its owning
producer. The earlier whole-tree verification checkbox is obsolete: scoped
main-checkout tests are the owed evidence, under `ai/rules/precommit-verify.md`.

### Package 1 evidence

The main thread ran the three new boundary tests in
`next_hop_auto_boundaries_test.go` with the named session build. The focused
run passed in `tmp/session/2026-11-15-153337/scratch/job-unit-pkg-79060f46.log`:
`TestNextHopAutoInternalMultihopUsesOnlyUsableNextHop` covers AC-7 and its
usable-global and explicit-Self survivors on both relays;
`TestNextHopAutoIPv6PairFollowsConnectedSubnet` covers AC-2's conditional pair;
`TestNextHopAutoMissingOwnAddressWarnsForThePeer` covers AC-4's warning at Warn
with the destination identity and an actual prior announcement.

The two functional files were promoted to `test/plugin/` after their draft
baselines and bounded stress run passed: `nexthop-auto-ebgp-rewrite.ci` and
`nexthop-auto-rs-client-unchanged.ci`. Main reported both promoted tests passed.
They use real daemon sessions and exact wire expectations with state-based
readiness and shutdown fences. They are untagged wiring evidence: the portable
loopback harness's synthetic 127/8 NEXT_HOP does not prove a valid production
NEXT_HOP or another router's acceptance. Package 3's separate legal-topology FRR
proof passed. Main also observed clean-PASS/broken-FAIL semantic redwalks for
both functional tests; their exact failures are retained in the owning state.

Three temporary `.diff` inputs under this session's scratch name the semantic
breaks for the new unit tests. They are overlay inputs for `le test redwalk`,
not changes to product source. Their paths and exact owed runs are in this
spec's owning per-session state file. Remove them after the redwalk results
are retained there. No new RFC tag or conformance credit is claimed by this
package.

## Current test-campaign evidence

The bounded eBGP Auto producer change selects the existing Self branch for
ordinary eBGP destinations. `job-bgp-auto-as0-green-55235c3d.log` in
`tmp/session/2026-11-14-210034/scratch/` passes all four new units in
`next_hop_auto_relay_test.go`: IPv4/IPv6 single-hop and multihop rewrite,
connected-endpoint preference and configured fallback, iBGP/RS preservation,
explicit-mode preservation, and exact mixed withdrawals when no self address
exists, across cached and native relays.

These four tests observe in-process Session output using controlled transport
metadata. The separate boundary test now asserts the warning, and AC-7/A-3
has its own conditional output proof. Functional baselines, both daemon redwalks,
legal FRR acceptance/redwalk and RS role interop passed. The old full
reactor-package run exposed the fixture defects recorded below; focused repaired
populations now pass, while a fresh full package run and spec closure stay open.
The existing owner decisions are unchanged; the untagged tests add no RFC
conformance credit. Retain the smallest useful green evidence through the
source commit and remaining evidence review.

The full package baseline is
`tmp/session/2026-11-15-153337/scratch/job-unit-pkg-a983a42a.log`: 26 top-level
failures and a test-side slice panic. Twenty-one failing units have the
initial AC-10 fixture correction; five Auto fixtures are separately owned
within this continuation. The exact patch and 30-test/one-benchmark census live
under the existing session state's "Admission fixture proposal" section.
The exact approved patch is applied after all 36 native approval actions; all
13 full before/after images matched the frozen manifest. Main's subsequent
21-unit run in `job-unit-pkg-8a3d09f3.log` passed twenty and still failed
`TestSendRoutesKeepsThePartialUpdatesOfARefusedCommit`: its identity-free
encoding context makes the commit builder prepend actual AS zero. The separate
single-helper identity correction is now applied after all six fresh native
approvals; its full after-image matches the reviewed manifest and all five
caller bodies remain byte-identical. Final native helper approval binding is
complete for SHA256 `6334d0da71f824075c3b3928f8cbbaa1d3016ca27b1b63c1922f18c0a9d95b8c`.
The previously failing single unit passes in `job-unit-pkg-1ed01873.log`, all
five identity callers pass in `job-unit-pkg-70c90c90.log`, and all thirty
admission caller tests pass in `job-unit-pkg-2da61af7.log`. The one-iteration
RS benchmark delivery controls for 100, 1000 and 2000 destinations pass in
`job-next-hop-auto-benchmark-control-3009c456.log`; this is delivery validation,
not a throughput comparison. All six Auto fixture callers and ten tagged
fixture callers pass in `job-unit-pkg-bf94097f.log` and
`job-unit-pkg-f3909007.log`. Fourteen RFC bindings, final audits/RFC check and
fresh lint/full reactor-package/group/plugin runs remain owed. The draft4-8
header-only audit reseal completed with one Shifted row and zero refusals;
it supplies no new semantic credit.

### Goal Validation

| Goal | Evidence and current limit |
|------|----------------------------|
| BGP-LS uses its native next-hop family and retains required propagation | Plain-family targeted GREEN after observed semantic RED: all 28 new cases and the three original propagation roots pass in `job-next-hop-bgpls-native4-green-dfb078ed.log`; seven existing profile roots pass in `job-next-hop-bgpls-known-profiles-85aefd44.log`. Independent source review found zero issues against unchanged post-format hashes. These fixtures observe Session writes to `recordingConn`, with no external TCP peer or real daemon in the new unit tests. Scoped lint, documentation checks and daemon coverage remain owed. LS-VPN framing research and implementation remain an open production phase in this spec; AC-12 stays open |
| Recognized raw origination obeys the peer-own NEXT_HOP prohibition | All ten raw primary/error units and the later 22-root preservation run passed. The promoted raw daemon test also passed after fixture preallocation. All four scoped units pass, including the three forbidden and three permitted MP32 writer cases, and both scoped semantic breaks have native records. The raw unit, raw daemon, scoped pair and direct SendAnnounce pair supply eight new semantic records in `rfc/discrimination/rfc4271/RFC4271-5.1.3-1.json`, which currently holds 15 entries; that record count is not a coverage percentage. The five approved Go oracle/claim repairs are applied, but their post-application unit runs and changed-claim proofs remain owed, and the typed CI repair remains a draft. Raw FRR acceptance/redwalk, lint and independent whole-row rejudgment/RFC check remain owed. AC-11 stays open and the canonical row remains weak. |
| Ordinary eBGP Auto produces a usable Self next hop | `TestNextHopAutoMatchesSelfForEBGP` observes exact IPv4/IPv6 output on both paths; promoted daemon test has an observed semantic red. Native legal FRR selection and its own semantic red passed. |
| Internal and RS-client Auto retain useful received next hops | `TestNextHopAutoPreservesRSClientAndIBGP` covers both paths; promoted RS-client daemon test retains exact received bytes with an ordinary eBGP control and an observed semantic red. Native RS role/AS-path/session interoperability passed; that existing checker does not itself assert exact next-hop bytes. |
| Unusable next hops cause withdrawal without suppressing permitted output | Boundary and six bare-Link-Local units assert exact withdrawal, Global/Self recovery and directly attached permission. The new tagged units and both changed pair units have native records; draft4-8 and RFC8277 rejudge verdicts are stamped. `job-next-hop-withdrawal-family-proof-refresh-39b8a037.log` observes all four clean EveryWriter cases pass and refreshes its native producer-reach record; its producer-halt red does not add a semantic-proof claim. |
| Complete integration and accurate documentation | YANG, structural-forwarding and netlab comments match live selection. Repaired caller populations and live labs passed; fresh lint/full reactor-package/group/plugin runs, fourteen RFC bindings, final audits/RFC check, documentation gate and closure remain open. The documentation run reports unrelated IRR anchors; no owned next-hop anchor fails. No closure or RFC4271 percentage increase is claimed. |

### Observed functional discrimination

The main thread observed clean-PASS/broken-FAIL results for both promoted daemon
tests after their baselines and stress run. The exact selectors were:

- `./le --name session test redwalk check break tmp/session/2026-11-15-153337/scratch/next-hop-auto-ebgp-interop.diff ci plugin/nexthop-auto-ebgp-rewrite` — clean passes; mapping ordinary Auto to Unchanged fails this test's exact destination next hop.
- `./le --name session test redwalk check break tmp/session/2026-11-15-153337/scratch/next-hop-auto-rs-preservation.diff ci plugin/nexthop-auto-rs-client-unchanged` — clean passes; removing only the RS-client Auto exception fails its exact preserved frame while ordinary Auto still rewrites.

`internal/le/test/redwalk/redwalk.go::ciTarget` takes `ci <suite>/<name>`
without `test/` or `.ci`. Both patches are overlay inputs, never applied
product edits. Their owner and removal conditions are in the existing
per-spec state record. The separate legal FRR scenario passed in
`job-next-hop-auto-frr-9f3d5b57.log`; its own semantic redwalk passed in
`job-next-hop-auto-frr-proof-2d788de7.log`, detecting received next hop
172.30.0.9 instead of 172.30.0.2 while the route remained valid, selected and
accessible. RS daemon redwalk evidence is in
`job-next-hop-auto-rs-daemon-proof-43ab1ebc.log`; ordinary daemon observations
are retained in the owning state. Native RS role interop passed in
`job-next-hop-auto-rs-interop-e0867a44.log`, with its narrower checker limits
preserved above. The later complete documentation run
`job-next-hop-auto-docs-ff96521f.log` executed and reported four unrelated IRR
source-anchor errors and a stale discovery index; native index update left no
tracked diff. No owned next-hop anchor fails, but the whole documentation gate
is not claimed passed.

## AC-12 LS framing continuation: design and TDD baseline, 2026-11-16

This appendix resolves the historical 40-versus-48 research question above and
defines the remaining AC-12 work. The spec remains in-progress and AC-12 remains
open. The small native-IPv4 repair stands independently; its execution evidence
is owned by the main thread.

Independent scoped re-review returned APPROVE for the two corrected design
issues: address-selection intent and the explicit LS71 pair forwarding proof
matrix. The reviewer verified MUP's producing inputs and their config carrier
and found no remaining ISSUE in those corrections or reached siblings. This
is a scoped design verdict. Main subsequently approved the corrected design
and authorized its test-first implementation. In
`job-next-hop-lsvpn-framing-red-cfd4d9d6.log`, main observed eight semantic
LS72 failures across the typed encode/decode and plugin-builder cases, while
all eight LS71 control leaves passed. The two roots are
`TestLSVPNNextHopTypedWire` and `TestLSVPNPluginBuilderNextHopWire` in
`plugins/nlri/ls/next_hop_wire_test.go`. The run compiled before any profile API
or framing repair. LS72 GREEN, daemon acceptance and live peer acceptance
remain owed.

### Primary evidence and wire decision

| Source | Deciding producer or consumer | Constraint |
|--------|-------------------------------|------------|
| RFC 9552 Section 5.5 and RFC 7752 Section 3.4 | Next-hop information, including the VPN sentence already quoted above | Plain LS carries IPv4, IPv6 global, or global plus link-local. VPN uses the customary zero RD prefix; NH RD is separate from the NLRI RD |
| [Wireshark v4.4.0 packet-bgp.c](https://github.com/wireshark/wireshark/blob/v4.4.0/epan/dissectors/packet-bgp.c) | `decode_mp_next_hop` selects the VPN helpers for `SAFNUM_BGP_LS_VPN`; `decode_mp_next_hop_vpn_ipv6` accepts one or two RD-plus-address units | The pinned LS72 decoder accepts 12/24/48, with zero RD before each address, and rejects 40. This is decoder evidence, not a live router acceptance claim |
| [ExaBGP commit 3790c532eb1d799b51db6e12ca2662f747e85042, family.py](https://github.com/Exa-Networks/exabgp/blob/3790c532eb1d799b51db6e12ca2662f747e85042/src/exabgp/protocol/family.py), `attribute/mprnlri.py`, `nlri/collection.py` | `Family.size`, `next_hop_lengths`, `MPRNLRI.unpack_attribute`, `MPNLRICollection._encode_nexthop` | LS72 declares 12/24 with RD8. Its actual writer emits those singles; its receiver rejects both 40 and 48. No ExaBGP pair acceptance claim is permitted |
| [freeRtr commit c5f8450fc2c1c92f64501408c5cebcc72cb8a0f7, rtrBgp.java](https://github.com/mc36/freeRtr/blob/c5f8450fc2c1c92f64501408c5cebcc72cb8a0f7/src/org/freertr/rtr/rtrBgp.java), `rtrBgpUtil.java`, `rtrBgpAttr.java` | The constructor's `bgp4idxes` and `bgp6idxes`, `safi2idx`, `rtrBgpAttrReachable.readAttrib` | LS72 constants exist but neither live family table enrolls them. The receiver returns before reading its NH. This does not establish LS72 parity |

→ Decision: LS71 has widths 4/16/32. LS72 has widths 12/24/48 and a
zero eight-octet RD before each address. The 48-octet pair follows the pinned
LS72 decoder's reading of the customary VPN encoding. There is no 40-octet
layout, compatibility acceptance or new once-per-field prefix abstraction.
Paired acceptance by another routing daemon remains unverified.

→ Constraint: LS follows RFC 2545's joint-subnet link-local condition. Merely
sharing RD framing does not import RFC 4659's VPN-IPv6 peer-only condition,
unspecified-global exception or IPv4-mapped origination rule. LS neither
requires nor advertises RFC 8950 negotiation for its native transport address.

### Existing mechanisms and alternatives

| Existing mechanism | Proposed use |
|--------------------|--------------|
| `attribute.NextHopProfile`, `MPNextHopProfile`, `ValidNextHopLens` | Extend this one field contract with framing and the missing plugin-owned facts. Existing built-in cases remain declared once; a registration cannot shadow one |
| `MPReachNLRI` sizing, writing and parsing | Make the shared profile interpret the field for both directions. Keep buffer-first writing and inline address storage |
| `peerForwardFacts` precomputed address and RD buffers | Keep snapshot-owned immutable buffers and reuse the existing per-address RD forms. Choose framing from the profile, not SAFI128 |
| `familyAdmitsNextHopOctets`, `ValidateMPNextHop`, final writer gates | Share widths and decoded roles with the producers. Preserve capability and family-specific scope decisions separately |
| LS plugin `register.go` | Register LS71 and LS72 pure-value profiles from its existing `init()`. Attribute and reactor packages import no LS plugin |
| Functional runner, existing `le test peer` and registered fixtures | Reuse process ownership, real sockets, state fences and cleanup for daemon propagation and native ExaBGP receipt |

| Approach | Trade-off | Disposition |
|----------|-----------|-------------|
| Add LS facts through an optional registration extension to the existing profile and use that profile in the current codecs | Adds one small leaf seam and migrates the actual framing consumers; keeps built-in facts in their present owner | Approved for test-first implementation after independent scoped re-review |
| Convert every existing built-in profile to init-time registration in the same patch | Also yields one authority, but changes registration lifecycle for unrelated families and expands the proof surface | Rejected for this correction; the optional extension adds missing facts without relocating all existing declarations |

### Minimum API and ownership proposal

| Surface | Contract |
|---------|----------|
| `RegisterMPNextHopProfile` in the attribute leaf | Init-time registration keyed by AFI/SAFI; error for duplicate registration, a built-in key, unsupported framing or an internally inconsistent width declaration. Registration copies the caller's length storage once. No session or UPDATE registers or mutates a profile |
| `NextHopProfile.RDPerAddress` | Generic framing fact: prepend/skip `RDSize` for each address. Built-in VPN profiles declare it once; LS72 declares it in the LS plugin. Zero retains plain framing |
| `NextHopProfile.EncodeIPv4Mapped` | Default typed-address conversion required by the family, distinct from permission to admit an already mapped input. Only the existing built-in2/128 profile sets it; plain2/1,2/2,2/4 permit mapped input but do not convert a direct attribute's native IPv4 input today. Registration refuses this flag unless `MappedIPv4` is true and the single mapped field's framed width is admitted. LS71/72 set neither mapping flag |
| `NextHopProfile.DecodeUnprefixedIPv6` | Preserve the existing codec-only legacy 16/32 acceptance of VPN128 as an explicit built-in fact. Strict session/outbound lengths remain unchanged. LS72 does not set it; no SAFI128 fallback remains inside framing code |
| Shared profile field-length, buffer-write and address-decode operations | Fixed address values and caller-owned buffers; no returned allocated slice, callback, config lookup or per-route registration. Derive primary/secondary offsets and wire IPv6 identity from the same profile. Method names may follow existing local naming, but these three operations have one implementation |
| `MPNextHopProfile` lookup | Read the optional init-time additions, then the existing built-in declaration. There are not two declarations of one family's facts. Length slices returned today remain shared read-only data: no allocating clone per query and no claim that Go can prevent caller mutation |

### Address-form selection before framing

`MappedIPv4` answers whether a mapped address is admitted; it does not request
conversion. The separate default-encode fact remains necessary to preserve a
real existing distinction: `MPReachNLRI.wireNextHop` maps a native IPv4 input
only for2/128, while `MPNextHopProfile` permits mapped input for2/1,2/2,2/4
as well. Replacing the former predicate with the latter would silently change
the unchecked direct attribute writer. Converting every IPv4 input whenever
its native width is absent would make the same change. This continuation
preserves that direct writer's behavior and leaves admission at its existing
checked callers; it does not legitimize the unchecked invalid plain-family
field.

One shared, allocation-free address-selection operation takes the profile,
the supplied address and a caller's explicit mapping request. An already
mapped or native IPv6 address retains its representation. A native IPv4
address becomes mapped only when the profile's default-encode fact or the
caller's explicit request says so; otherwise it stays native. Field length,
writing and admission consume that selected value, without selecting again.
The RD writer never decides mapping, and a received or raw-policy field is
decoded in its existing form, not passed through this conversion.

| Producer intent | Mapping request and profile facts | Required output and boundary |
|-----------------|-----------------------------------|------------------------------|
| Direct `MPReachNLRI` address input | No caller mapping request;2/128 alone has `EncodeIPv4Mapped` | Native IPv4 becomes RD8+mappedIPv6,24, for2/128. For2/1,2/2,2/4 the unchecked codec retains native4 as it does now; checked originators must refuse that form or deliberately select an admitted mapped form |
| Configured Auto/Self/explicit IPv4 forward replacement | Obtain the native candidate width through profile framing:4 for plain,12 for RD. A known table excluding that width requests mapping; nil table or an admitted native candidate does not. Profile default conversion still applies | Existing2/1,2/2,2/4 replacements remain mapped16;2/128 remains RD+mapped24;1/128 andLS72 choose native RD+IPv4,12; LS71 stays native4. A selected form still owes width/role/capability admission; a mapping request is not an exemption |
| `BuildPlugin` / config-owned plugin input | Preserve the existing request `PluginParams.MapV4NextHop` together with `IsIPv6` and native IPv4 input; combine it with the profile default in the same selection operation | MUP config and encode routes set the request true and retain mapped16 under2/85 despite its presently unknown profile. SR Policy leaves it false and retains native4 under either AFI. Other plugin inputs retain their supplied forms unless their own family has a default conversion |
| Effective raw-policy replacement / received bytes | No address conversion request is synthesized from mode or another family's facts | Validate and decode the exact emitted field. Do not repair a mapped-plus-link-local pair, erase a recipient-owned slot, or turn preserving traffic into a different representation |

For `BuildPlugin`, compute the selected address once before
`ValidateFamilyNextHop` and pass that same value to `buildMPReachPlugin`;
remove its duplicate mapping decision. For `MPReachNLRI`, the same selection
operation supplies both its size and write. Forwarding may continue selecting
the corresponding precomputed immutable byte view after applying the same
rule; it need not serialize into fresh per-route storage.

Registration validation treats an encode/admission contradiction as a
programming error: `EncodeIPv4Mapped` with `MappedIPv4` false is refused, as
is an admitted-mapped claim with no single IPv6 width after adding its RD
framing. A forced mapping profile without that same admitted framed width is
also refused. The independent synthetic tests cover each refusal, then prove
a valid plain mapped-permission profile does not convert direct native input,
a valid forced-mapping RD profile does, and an explicit mapping request still
works for a zero/unknown profile. This tests behavior rather than flag storage.

The deciding source was read: MUP's `config.go` returns
`registry.PluginRoute.MapV4NextHop` true; its `encode.go` passes the same true
value to `PluginParams` and `BuildPlugin`; `peer_static_routes.go` carries that
flag through config origination. `BuildPlugin` presently maps separately
before admission and again in `buildMPReachPlugin`. These are declared producer
inputs to preserve, not new MUP facts copied into the core profile table.

The registration-input ownership test mutates the caller's original slice after
registration and proves lookup is unaffected. It does not mutate a query's
documented read-only slice or claim stronger immutability. Generic tests use
synthetic AFI/SAFI keys and restore their registry snapshot without parallel
global mutation; LS tests link the real owning plugin and never re-register it.
No generated composition-root change is needed for a new declaration inside
the already-linked LS plugin. This mechanism registers compile-time codec facts;
it adds no new dynamic external-plugin protocol promise.

Unknown profiles retain the current nil-table permissiveness and generic
length-decoding behavior in the shared helper. Known profiles determine their
own framing; they never acquire a legacy unprefixed decode from another family.
The RFC5549-tagged legacy VPN codec test stays intact. No new rejection of
nonzero received RDs is inferred merely from moving the decoder: emitted RDs
are zero, while an additional receive error policy would need its own source
and reviewed test claim.

### Changed-wire census and edits owed

The census searches covered Go producers/readers under `internal/core/bgp`,
`internal/component/bgp`, bridge/format readers, and the shell/Python/CI
carriers in the existing test trees. `MPReachNLRI.WriteTo`,
`buildMPReachPlugin`, `nextHopAddr` and `mpReachNextHopHandler` were positive
controls. The sites below are derived from those actual carriers, not merely
from the proposed file list.

| Current site | Owed change or preservation |
|--------------|----------------------------|
| `attribute/mpnlri.go`: `nextHopLen`, `nextHopOctets`, `wireNextHop`, `writeNextHops`, `parseNextHops`, `parseVPNNextHops`, profiles | Use the shared profile for RD count, mapping, offsets and legacy decoding; remove the old SAFI128 framing branch and duplicated layout interpretation |
| `plugins/nlri/ls/register.go` | Declare the two LS profiles; neither declaration appears in a central feature switch |
| `message/update_build_plugin.go`: `BuildPlugin`, `buildMPReachPlugin` | Select the actual address once using profile default plus the existing explicit plugin mapping request, validate that selected value, then pass it to the shared profile writer. Preserve MUP's mapped16 and SR Policy's native4 while LS72 gains RD12/24 |
| `message/update_build.go`: `ValidateFamilyNextHop`, `ValidateMPNextHop` | Count actual framed wire width, then judge decoded address roles. A 12-octet IPv4 LS72 field is not an IPv6 global, and a 48-octet LS72 pair is not plain48 |
| `message/rfc7606.go`, `rfc7606_linklocal.go` | Width admission reads the registered profile; pair-role decoding uses its framing without bypassing existing RFC7606 error handling or broadening capability77 |
| `message/update_build_vpn.go` | Keep its VPN NLRI/label building; obtain NH sizing/writing from the same profile rather than retaining an independent SAFI128 framing authority |
| `wireu/mpwire.go`: `MPReachWire.NextHop` | Read family and use shared address decoding; `NextHopBytes`, splitting and NLRI slicing retain exact whole-field bytes |
| `reactor/peer_forward_facts.go`: `applyFactsNextHop`, RD buffer selection | Select native or mapped address and RD framing from the profile; retain precomputed borrowed buffers and independently decided link-local inclusion |
| `reactor/forward_next_hop.go`: `nextHopAddr`, `payloadNextHop`, `modsNextHop`, scope/admission consumers | Pass the effective MP family into profile decoding; remove the independent IPv6-width enumeration. Policy buffers remain complete fields. Preserve raw received form for lossless forwarding |
| `reactor/filter_delta_handlers.go`: `mpReachNextHopHandler`, `familyAdmitsNextHopOctets` | Check the selected profile, with the existing generic-width behavior for unknown families. Keep final length/header accounting, reserved byte and exact NLRI preservation |
| `reactor/egress_inject_filter.go`: ordinary/RD normalization | Obtain offsets, field growth and pair/single forms from the profile. LS uses joint-subnet scope; the existing VPN-specific predicate still owns its narrower exception |
| `reactor/session_write.go`, batch and queued announce callers | Final admission reads the same contract after policy. Batch/queued callers already pass the actual family; verify both without routing them through the context-free checked attribute writer |
| `reactor/aigp_readvertise.go`: `replayAIGP` | Decode retained NH with `ad.route.key.nlri.family`; preserve its retained whole-field bytes and metric/source ownership |
| `reactor/peer_static_routes.go`: `toPluginParams` and static callers; MUP `config.go`/`encode.go` | Keep config-to-`BuildPlugin` reachability and existing explicit mapping request. Add a MUP actual-output preservation test through these producers; do not rewrite plugin-owned mapping intent into a central family enumeration |
| `reactor/peer.go`: transport/capability admission | Preserve LS's existing independent IPv4/IPv6 permission; do not add another LS special case to repair framing |
| `cli/decode_mp.go`: `buildMPReachZe`, `parseNextHop`; format and bridge readers | Pass SAFI as well as AFI to shared address decoding. Existing `MPReachWire.NextHop` consumers inherit it; retain full-field storage and NLRI RD independently |
| Existing raw body/packet API, stored-route reconstruction and wire splitting | No framing rewrite merely to normalize received bytes. Assert exact NH/NLRI round trip and mixed-family survival through the normal final guards |

### TDD order and useful outcomes

| Order and proposed test | Existing entry point and literal oracle | Required first observation |
|-------------------------|-----------------------------------------|----------------------------|
| 1. `TestLSVPNNextHopTypedWire` in `plugins/nlri/ls/next_hop_wire_test.go`, external test package | Existing `NewMPReachNLRI`, `Len`, `WriteTo`, `ParseMPReachNLRI`; IPv4/global/pair input with a nonzero NLRI RD. Assert literal NH lengths12/24/48, zero RD before every address, exact address bytes, reserved octet, unchanged NLRI and decoded addresses. Include LS71 4/16/32 positive controls | Semantic RED on the existing naked4/16/32 output before adding any API. No new symbols or registration calls in this first test; the owning package links LS registration naturally |
| 2. `TestLSVPNPluginBuilderNextHopWire` alongside it | Existing `UpdateBuilder.BuildPlugin` with actual LS72 `PluginParams`, IPv4 and IPv6 singles. Inspect the returned MP_REACH and exact NLRI, with LS71 survivors | Semantic RED on naked4/16 output before the seam; do not invent a `PluginParams` pair field merely to test the typed attribute's pair |
| 3. Core `TestMPNextHopProfileRegistration` and `TestMPNextHopProfileInputOwnership` after the seam exists | Synthetic keys test duplicate/built-in-key/invalid-profile refusal, including encode-without-admission and missing framed mapped width; copied input ownership and unchanged unknown lookup. Decode/write actual bytes, rather than assert a map exists | Main runs these only after the API exists; a missing-symbol compile failure is not protocol RED |
| 3a. `TestMPNextHopMappingIntent` and `TestMUPMappedNextHopPreserved` | Existing direct attribute, configured replacement and `BuildPlugin` entry points;2/128 automatic mapped24 versus2/1,2/2,2/4 direct native4/checked refusal and forward mapped16; MUP config and encoder2/85 mapped16; SR Policy2/73 native4; LS71native4 andLS72native12; zero-profile explicit mapping and supplied mapped-address preservation | Record literal before bytes at the current producers before changing their shared implementation. Existing correct producer distinctions remain GREEN; no test expects a plain invalid field to pass checked admission |
| 4. `TestLSVPNNextHopSharedConsumers` | Existing wire reader, ordinary/batch/plugin writers and public decoder; known12/24/48, zero RD offsets, LS71 controls, malformed widths including40, and existing128 mapping/legacy controls | Establish each old omission at its real entry point before its corresponding producer/consumer edit |
| 5. `TestNextHopAutoLSVPNFraming` and `TestNextHopLSVPNPeerOwnRecovery` in a separate reactor test file | Reuse receive-to-socket cached/native helpers, real family/ASN4 negotiation and existing scope fixtures. Auto/Self/explicit send RD12/24 or on-link48; iBGP/RS/Unchanged keep exact received fields; a seeded peer-own route withdraws the exact NLRI and corrected Auto reannounces it | Actual ANN/NH/NLRI assertions fail before repair; preserving twins survive. Cover recipient identity in both slots and policy-created fields, not only precomputed Self |
| 6. LS scope/policy continuation tests | Compare LS joint-subnet on/off-link output with existing VPN128 peer-only and unspecified-global controls. Policy returns a complete framed field; final admission sees its width and both address roles | Exact48-to24 trimming and24-to48 growth, independent scope distinctions, useful surviving route and exact withdrawal for an unusable replacement |
| 6a. `TestLS71PairForwardScope`, `TestLS71PairPeerOwnRecovery` and the scoped, superseded-source and invalid-pair roots in `ls71_pair_recovery_test.go` | The explicit receive-to-recording-writer matrix below, cached/native and actual LS71 MP_REACH with exact full NLRI. Export policy supplies a base before configured mode selection. Preserving modes expose received or policy final fields; Auto/Self/explicit expose their selected field | Exact32-to16 trimming,16-to32 growth, on-link32 survival and useful off-link16 survivors. First show the old missing growth or other incorrect branch semantically RED; branches already correct remain preservation GREEN. Codec controls alone do not discharge this row |
| 7. `test/draft/plugin/bgpls-vpn-next-hop-framing.ci`, then native promotion | Existing real daemon, `le test peer`, registered fixture and state-based fences; negotiate LS71/72, exercise actual default Auto relay and a config/plugin-origin single. Assert exact NH and full NLRI, with preserving recipient control | Daemon useful output first; draft load proof and approved semantic discrimination before promotion under `ze-test` |
| 8. `test/draft/plugin/bgpls-vpn-exabgp-single.ci`, then native promotion | Reuse the runner's owned background processes and registered fixture mechanism to connect Ze to actual ExaBGP at immutable3790c532eb1d799b51db6e12ca2662f747e85042. Its native decoded UPDATE/API reports must identify exact LS72 NLRI and IPv4/IPv6 NH after Ze Auto forwarding, followed by exact withdrawal; preserve healthy session and separate control route | Actual decoder receipt of both singles. The existing compiled Go ExaBGP personality and static captured bytes are not this acceptance oracle; pairs remain excluded from the ExaBGP claim because its source rejects them |

The existing `startExaBGPDaemon` resolves the real ExaBGP executable on PATH
and the predecessor suite supports `--dut exabgp`; reuse that managed pinned
environment and process conventions, not a second download/runner framework.
The new dual-daemon functional fixture must run the actual executable and
read its native decoded report. Installing/resolving that exact revision is
a prerequisite to execution; absence is a setup failure, not a passing or
skipped interop result. No current ExaBGP process or installation was inspected
or created during this design phase.

### Explicit LS71 paired forwarding proof matrix

Every row starts from an actually received, well-framed LS71 UPDATE and ends
at the recording Session writer. Run both cached/native relays. The mode
axis is ordinary eBGP Auto, Self and explicit speaker-owned IPv6, plus
Unchanged, iBGP Auto and RS-client Auto preserving twins. Use actual
negotiation and the existing connected-prefix/local/neighbor scope fixtures;
the supplied addresses and expected bytes are literal, including both slots.
No stand-alone codec or direct call to a scope helper substitutes for this path.

The existing order is explicit in `reactor_api_forward.go`
`forwardUpdateSection` and `forward_rs.go` `reactorForwardRSSectionResult`:
the export chain selects the base wire, then `applyFactsNextHop` applies the
configured mode, then scope normalization and final admission inspect the
effective field. Raw export policy does not override Auto/Self/explicit after
that selection. This matrix preserves that order.

| Received/effective field and condition | Exact useful socket result | Counter-case that keeps the proof conditional |
|----------------------------------------|----------------------------|-----------------------------------------------|
| Received global+LL32; selected effective global and recipient share one connected subnet | Effective global+its correct LL32; preserving modes retain the exact valid received32, and rewriting modes carry their chosen global and that entity's LL | Keep the same input and mode but put the recipient outside that joint subnet: output is the same effective global alone16, exact NLRI still announced |
| Received global16; selected effective global and recipient share one connected subnet, and the effective entity's LL is known | Effective global+that entity's LL32, proving16-to32 growth after actual mode/policy selection | Remove the common-prefix fact or the effective entity's LL: useful global16 survives, never a fabricated or unrelated LL |
| Speaker and recipient share a prefix, but the effective third-party global belongs to another prefix | Effective third-party global16 with the exact announced NLRI; a received LL is trimmed32-to16 | Move that effective global into the one shared prefix and provide its own LL:32 is now required. This distinguishes LS's joint condition from VPN128's peer-only condition |
| Export policy supplies global+LL32 or global16 as the base | Preserving modes normalize that policy field. Auto/Self/explicit normalize their configured replacement instead. Each follows the same32-to16/16-to32 scope conditions | Preserve a separate valid sibling NLRI/field in a mixed UPDATE; normalization or refusal of this field must not remove that sibling |
| Effective global is the recipient's own global address, after a useful accepted seed | All six modes withdraw the exact seeded LS71 NLRI and retain the allowed sibling. Corrected selection or preserving input reannounces the exact NLRI with its literal usable NH | Preserving modes obtain the harmful final global from received or policy bytes. Auto/Self/explicit obtain it from their actual selected endpoint/address; a harmful source base legitimately replaced by usable self remains permitted |
| Effective second slot is the scoped recipient's own link-local wire address, after a useful accepted seed | Unchanged, iBGP Auto and RS Auto withdraw the exact seeded LS71 NLRI before trimming could conceal the peer-owned LL. Corrected received or policy field reannounces the exact NLRI | Keep recipient `fe80::2%if0` scoped and compare zone-free bytes. An otherwise identical pair with another router's valid LL trims to useful global16 and survives, with the recipient LL-prefix present or absent |
| Auto/Self/explicit replace a received or policy base naming the scoped recipient in slot two | The legitimate configured replacement announces useful global16 with the exact LS71 NLRI and valid sibling | A scoped LL recipient cannot share an ordinary connected global prefix with the selected global. Even a configured speaker LL equal to the recipient's wire LL cannot create a selected32 in this topology |

Both identity-refusal sequences retain the precise seed/withdraw/reannounce
order and exact NLRI bytes, not merely an absence or warning. Both relays cover
all six modes for harmful final global selection and the three preserving modes
for reachable harmful final second-slot fields. Rewriting modes keep their
permitted stale-source twins; no case invents a policy stage or an impossible
joint subnet. Final admission must still inspect both address slots before
scope trimming can hide a malformed or recipient-owned slot. The invalid-pair
root checks received and policy fields under the preserving modes on/off-link,
with exact withdrawal, recovery and useful sibling output. The pre-existing
typed LS71 4/16/32 controls remain additional codec evidence.

Existing RFC-tagged tests are not edited by this test-first tranche. New
tests initially carry no broader credit. Any later tag/body/claim change gets
independent test judgment, actual positive and negative semantic records and
same-commit scope/audit reconciliation. The partial RFC9552-5.5-1 and
RFC7752-3.4-1 annotations remain honest until the relevant observations exist.

### Failure modes, documentation and review gate

| Risk or assumption | Early signal | Required mitigation/validation |
|--------------------|--------------|--------------------------------|
| R-6: profile registration duplicates a built-in fact or aliases caller data | Duplicate accepted or mutation changes subsequent wire output | Reject occupied keys and clone input once; synthetic behavioral tests; no tests mutate linked LS globals |
| R-7: moving RD framing changes existing VPN128 mapping, scope, unspecified-global or legacy codec behavior | Existing VPN controls change their literal bytes, capability verdict or decoded address | Declare existing facts once in the built-in profile; retain tagged legacy test and strict session width checks; run scoped VPN controls |
| R-8: a raw policy operation is decoded using its predecessor's width/family | Peer-own identity is missed or an invalid field is silently retained | Pass effective source family and inspect the final complete operation; seeded announcement/withdrawal/recovery and exact useful survivors |
| R-9: registration or helper use allocates per route | Allocation assertion changes on profile lookup or fixed-buffer write/decode | Only registration copies; shared read-only lookup and caller-buffer operations. Main uses bounded allocation tests for the new helpers |
| R-10: default family conversion, configured width selection and explicit plugin mapping are conflated | MUP2/85 loses mapped16, direct2/1 unexpectedly converts,2/128 loses mapped24, or a policy pair is normalized before identity admission | Use the one selected-address operation with explicit producer intent and profile default; reject contradictory registered facts; run the literal mapping-intent preservation matrix before/after |
| A-7: customary per-address VPN framing is the compatible LS pair interpretation | New authoritative LS72 peer source disagrees with pinned Wireshark | Keep primary pin and limited claim; 40 is not accepted speculatively; actual paired-router acceptance stays unclaimed |
| A-8: real ExaBGP receipt can be exercised through existing managed process facilities | No exact executable or native decoded report is available | Resolve the pinned dependency and implement its normal process subscriber; retain the interop obligation, do not substitute the Go mock |

Documentation changes are part of each implementing chunk, before its source
edit: `docs/architecture/wire/attributes.md` and
`docs/architecture/wire/nlri-bgpls.md` own profile/RD layout;
`docs/architecture/bgp/structural-forwarding.md` owns effective identity,
normalization and distinct scope rules; `docs/architecture/update-building.md`
owns plugin/config encoding; `docs/architecture/testing/interop.md` owns the
actual ExaBGP fixture and its limited claim. Amend `ai/patterns/bgp-family.md`
to direct new family NH facts to owning-plugin registration rather than a
new central case. Existing RFC summaries keep their partial classifications
until tests and required native records support a larger claim.

Triple challenge: this extends the existing profile and codec rather than
introducing a second authority; it uses existing init-time registration and
buffer-first encoding patterns; initialization alone allocates for copied
facts and hot readers/writers remain allocation-free. The strongest concern
for independent review is preserving decoder-only VPN128 compatibility while
every live framing consumer moves to one contract, and distinguishing actual
ExaBGP receipt from the existing mock. The independent scoped design re-review
and main authorization above permit the test-first continuation. Implementation
review, scoped GREEN, daemon and interop observations remain owed. Main alone
executes the commands finalized by each implementation tranche.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | Existing Self encoding was treated as complete for every implemented MP family | The absent BGP-LS length contract makes `applyFactsNextHop` emit mapped IPv6 for a usable IPv4 replacement; SAFI 72 also lacks its required RD framing | Shared reactor propagation reds and a trace through the profile, producer and effective-address guard | AC-12 owns standalone regression tests, reviewed producer repair and the LS-VPN contract; original tagged propagation tests remain unchanged |
| assumption | The admission-fixture author assumed matching Session ASN4 width was sufficient for every original announcement to become valid | `newCommitFixture` also used an identity-free `EncodingContextForASN4`; `EncodingContext.LocalASN` returned zero and `CommitService.buildASPathFromExplicit` correctly prepended it for EBGP. Final admission correctly withdrew actual AS zero | Main's focused run `job-unit-pkg-8a3d09f3.log` passed twenty units but the partial-commit unit still reported zero announced routes instead of two; source trace identified the missing context identity | Applied the exact independently approved context carrying the same PeerSettings local/peer AS and role after six native replacement judgments and completed the final helper binding. The single unit, five identity callers and complete thirty-test population now pass; both original groups, one-flush budget and every assertion remain |

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-next-hop-auto-rewrites-for-ebgp.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
