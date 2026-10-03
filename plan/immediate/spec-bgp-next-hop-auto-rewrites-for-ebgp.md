# Spec: bgp-next-hop-auto-rewrites-for-ebgp

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-03 |

Bucket: `plan/immediate/`, because an operator relaying to eBGP with `next-hop auto` meets wrong next hops.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The YANG text and the code disagree about `next-hop auto`, which is the
default. `ze-bgp-conf.yang` (leaf `next-hop`, about lines 858 to 879) says
"auto rewrites the next hop for an eBGP peer, and preserves it for an iBGP
peer", and its `auto` enum help says "RFC 4271 default: rewrite for eBGP,
preserve for iBGP". `peer_settings.go` says the same at the `NextHopAuto`
constant (line 25) and in the `NextHopMode` field comment (line 576). The code
does not do it: `precomputeNextHop` maps `NextHopAuto` and `NextHopUnchanged`
to the same no-op mode, so a route Ze relays to an eBGP peer under `auto`
keeps the next hop it was received with.

Owner decision (Thomas, 2026-10-03): change the CODE so `auto` rewrites the
next hop to this speaker's own address for an eBGP peer, single hop and
multihop alike, as the RFC 4271 Section 5.1.3 default. The YANG text stays as
written and becomes true. The work lives in this spec, separate from the
link-local spec `plan/immediate/spec-bgp-update-propagation-rfc-defects.md`
(defect D6), which owns the `unchanged` towards multihop case.

### Owner-approved behaviour table

Recorded from the link-local research, approved by Thomas on 2026-10-03.

| Configured mode | Destination | Behaviour | Owner |
|-----------------|-------------|-----------|-------|
| `unchanged` | multihop peer | withhold the route | link-local spec, `spec-bgp-update-propagation-rfc-defects.md` D6 |
| `auto` | eBGP, single hop or multihop | rewrite to self; withdraw only if self has no usable address | this spec |
| `auto` | multihop iBGP, route not locally originated | draft Section 4 item 1 gives nothing without next-hop-self, so withhold unless `self` is configured | this spec (see A-3 for the reading) |
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
  → Constraint: RFC 7947 Section 2.2.1: "As the route server does not participate in the actual routing of traffic, the NEXT_HOP attribute MUST be passed unmodified to the route server clients". `auto` towards an RS client MUST keep the received next hop

**Key insights:**
- The rewrite machinery already exists: `auto` towards eBGP needs the same wire form `self` builds, including the withheld outcome when the session has no local address.
- `auto` and `unchanged` are one case in `precomputeNextHop` today; this spec splits them by the destination's eBGP flag and RS-client flag.
- `applyNextHopMod` (`reactor_api_forward.go`) has no production caller and its `auto` comment claims the eBGP AS-path step "already sets next-hop", which is false.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - leaf `next-hop` under `session`: a union of an IP address and the enum `self`, `unchanged`, `auto`; **default `"auto"`**; description and help both say auto rewrites for eBGP
- [ ] `internal/component/bgp/reactor/peer_settings.go` - constants `NextHopAuto` (0), `NextHopSelf`, `NextHopUnchanged`, `NextHopExplicit`; the `NextHopAuto` comment and the `NextHopMode` field comment both say "rewrite for eBGP, preserve for iBGP"
- [ ] `internal/component/bgp/reactor/config_nexthop_form.go` - `applyNextHopMode` parses the leaf into `PeerSettings.NextHopMode`
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - `precomputeNextHop`: the `NextHopAuto, NextHopUnchanged` case sets the no-op mode; `NextHopSelf` reads the connected local endpoint, then `LocalAddress`, and sets the withheld flag when neither exists; `applyFactsNextHop` records no operation for the no-op mode and records attribute 3 and 14 sets otherwise
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - the forward loop calls `applyFactsNextHop`, then `egressNextHopGlobalHalf`, then withholds on the self-withheld flag; the eBGP branch (`facts.isEBGP`, near line 1049) builds only an AS_PATH prepend intent and touches no next hop; `applyNextHopMod` is dead code with a false `auto` comment
- [ ] `internal/component/bgp/reactor/forward_rs.go` - the route-server rail calls the same `applyFactsNextHop` and relies on it recording nothing for an RS client; its comment cites RFC 7947 Section 2.2.2 for the next hop, which is Section 2.2.1
- [ ] `internal/component/bgp/reactor/link_scope.go` - `applyLinkLocalNextHop` raises the self and explicit IPv6 modes to the two-address form under the RFC 2545 Section 3 condition
- [ ] `internal/component/bgp/reactor/forward_next_hop.go` - `egressNextHopGlobalHalf` strips a received link-local half under `auto` and `unchanged` for a multihop peer

**Behavior to preserve:**
- `auto` towards an iBGP peer keeps the received next hop (RFC 4271 Section 5.1.3 item 1), except where the link-local spec withholds.
- `auto` towards an RS client keeps the received next hop (RFC 7947 Section 2.2.1), on both rails.
- `unchanged` never rewrites; `self` and an explicit address behave as today.
- The self-withheld outcome: no local address means the announcement is withheld, the withdrawals in the same UPDATE still go, and the warning names the peer.
- Route reflection attribute handling, AS_PATH prepend, and the link-local strip for multihop peers.

**Behavior to change:**
- `auto` towards a non-RS-client eBGP peer, single hop or multihop, rewrites the next hop to this speaker's address on that session, exactly as `self` would.
- `auto` towards a multihop iBGP peer for a route not locally originated withholds where draft Section 4 item 1 leaves no next hop (A-3).
- The dead `applyNextHopMod` and its false comment are removed or corrected (implementation decides, under `ai/rules/no-layering.md`).
- The RFC 7947 section cite in `forward_rs.go` is corrected to Section 2.2.1.

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
| Config → reactor | `PeerSettings.NextHopMode` | No |
| Reactor → wire | `ModAccumulator` operations on attributes 3 and 14 | No |
| Ze → peer daemon | NEXT_HOP and MP_REACH_NLRI next-hop field | No |

### Integration Points
- `precomputeNextHop` - gains the `auto` towards eBGP case; it already holds the self branch to reuse
- `applyFactsNextHop` - unchanged; it already writes every self form
- The withheld gate on both rails - reused for `auto` with no usable local address

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be filled at design |
| No unintended coupling (components stay isolated) | No | to be filled at design |
| No duplicated functionality (extends existing, does not recreate) | No | to be filled at design: `auto` reuses the self branch rather than a second builder |
| Zero-copy preserved where applicable (refs, not copies) | No | to be filled at design |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | to be filled at design |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | to be filled at design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `auto` is the YANG default, so every eBGP session with no `next-hop` leaf changes behaviour | `ze-bgp-conf.yang` leaf `next-hop`, `default "auto"` | the blast radius is smaller than stated | read of the YANG leaf, 2026-10-03 | confirmed |
| A-2 | An RS client is recognised by `PeerSettings.RSClient` and must keep the received next hop under `auto` | `peer_forward_facts.go` sets `rsClient` from `RSClient`; RFC 7947 Section 2.2.1 | the change rewrites next hops at an IXP route server, a MUST violation | unit test over both rails with an RS client under `auto` | unvalidated |
| A-3 | The owner's row "`auto` to multihop iBGP, route not locally originated: withhold unless `self`" applies where the received next hop leaves no usable address for that peer (a link-local-only next hop), not to every such route | draft Section 4 item 1 and its closing sentence; RFC 4271 Section 5.1.3 item 1 says SHOULD NOT modify | withholding every relayed route to a multihop iBGP peer would break iBGP relay outright | owner confirmation at design | unvalidated |
| A-4 | "Self" under `auto` is the same address `self` uses: the connected local endpoint, then the configured local address | `precomputeNextHop` self branch; `connectedLocalAddress` | announce rail and forward rails disagree | wiring test over both rails | unvalidated |
| A-5 | Whether "eBGP" includes a confederation-external peer is decided by the existing `isEBGP` flag | `peer_forward_facts.go` | wrong treatment inside a confederation | read the producer of `isEBGP` at design | unvalidated |

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
| `next-hop` leaf absent (default `auto`), route relayed to an eBGP peer | → | `precomputeNextHop` then `applyFactsNextHop` on the forward rail | `test/plugin/nexthop-auto-ebgp-rewrite.ci` (to be written) |
| `next-hop auto`, route relayed to an RS client | → | `forward_rs.go` keeps the received next hop | `test/plugin/nexthop-auto-rs-client-unchanged.ci` (to be written) |

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
| AC-8 | `unchanged`, `self`, explicit address | behaviour unchanged from HEAD |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator relays routes from an upstream to an eBGP customer with no `next-hop` leaf | wire → forward rail → `precomputeNextHop` → accumulator → wire | `test/plugin/nexthop-auto-ebgp-rewrite.ci` (to be written) |
| 2 | IXP operator runs Ze as route server with default `auto` | wire → RS rail → wire, next hop untouched | `test/plugin/nexthop-auto-rs-client-unchanged.ci` (to be written) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| to be named at design | `internal/component/bgp/reactor/` | AC-1 to AC-8 per rail | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | the feature takes no numeric input | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `nexthop-auto-ebgp-rewrite` | `test/plugin/` | eBGP relay under default `auto` carries Ze's address | |
| `nexthop-auto-rs-client-unchanged` | `test/plugin/` | RS client keeps the received next hop | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-nexthop-auto-ebgp-frr` (planned name) | `test/interop/scenarios/` | FRR | FRR receives Ze's address as next hop for a relayed route under default `auto`; red when the change is reverted | |

## Files to Modify
- `internal/component/bgp/reactor/peer_forward_facts.go` - `precomputeNextHop` splits `auto` by eBGP and RS-client flags; the no-op mode comment
- `internal/component/bgp/reactor/reactor_api_forward.go` - `applyNextHopMod` dead code and its false `auto` comment
- `internal/component/bgp/reactor/forward_rs.go` - RFC 7947 section cite corrected
- `internal/component/bgp/reactor/peer_settings.go` - comments checked against the new behaviour
- `internal/component/bgp/yang/ze-bgp-conf.yang` - description gains the RS-client exception and the withheld outcome
- Tests listed under Blast Radius, where the behaviour table says the old assertion was wrong
- `docs/architecture/bgp/structural-forwarding.md` - next-hop sections cover `auto`

## Files to Create
- `test/plugin/nexthop-auto-ebgp-rewrite.ci` - functional test, AC-1
- `test/plugin/nexthop-auto-rs-client-unchanged.ci` - functional test, AC-6
- `test/interop/scenarios/bgp-nexthop-auto-ebgp-frr/` - interop scenario

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
| 6 | Has a user guide page? | to check | `docs/guide/configuration.md`, `docs/guide/bgp-policy.md` |
| 7 | Wire format changed? | No | wire format unchanged; the value sent changes |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4271.md` Section 5.1.3 default rows; `rfc/short/draft-ietf-idr-linklocal-capability.md`; `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | to check | `docs/comparison.md` next-hop row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/bgp/structural-forwarding.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none planned |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | to derive | `./le spec citation anchors spec plan/immediate/spec-bgp-next-hop-auto-rewrites-for-ebgp.md` at design |
| 17 | Existing docs show config/CLI/API examples for this area? | to check | grep `docs/` for `next-hop` examples at design |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- write the two functional tests and confirm they are red against HEAD
   - Tests: `nexthop-auto-ebgp-rewrite`, `nexthop-auto-rs-client-unchanged`
   - Files: `test/plugin/`
   - Verify: the eBGP test fails because the next hop is the received one; the RS test passes (it guards the preserve case)
2. **Phase: auto towards eBGP** -- split the mode in `precomputeNextHop`, reuse the self branch and its withheld outcome
   - Tests: unit tests for AC-1 to AC-6, AC-8
   - Files: `peer_forward_facts.go`, `reactor_api_forward.go`, `forward_rs.go`
   - Verify: tests fail, implement, tests pass
3. **Phase: multihop iBGP withhold (AC-7)** -- after the owner confirms A-3, coordinated with the link-local spec
4. **Phase: test review** -- walk the Blast Radius list, correct assertions the behaviour table proves wrong
5. **Phase: interop and docs** -- FRR scenario with a recorded red, YANG description, architecture page, RFC summaries

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

- To be written at design.

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
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/immediate/spec-bgp-next-hop-auto-rewrites-for-ebgp.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
