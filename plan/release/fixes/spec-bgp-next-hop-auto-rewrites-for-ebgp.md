# Spec: bgp-next-hop-auto-rewrites-for-ebgp

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P1 |
| Release reason | The default next-hop policy must implement its promised eBGP and iBGP behavior through the single export filter chain surface. |
| Scope | protocol |
| Depends | - |
| Phase | 5/5 |
| Handoff | - |
| Updated | 2026-11-25 |

Bucket: `plan/release/fixes/`, because the default next-hop behavior is a prerequisite of ordinary best-route advertisement.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The original defect was a disagreement between the documented `next-hop auto`
default and `peer_forward_facts.go::precomputeNextHop`: Auto preserved a
received next hop toward ordinary eBGP. Commit `ae01917413` implemented the
approved default by selecting the existing Self branch, and subsequent commits
landed AC-1..AC-10 core/daemon/FRR proof, AC-12 shared `NextHopProfile` and
LS71/LS72 unit coverage (`ff7032975a`, `52e4ae832a`, `771ba0d96d`), AC-13
FlowSpec scope preservation (`63afaf0c95`), AC-14 listener ownership code
(`9b6f3c8708`, pending 6th review round), and AC-11 raw/scoped peer-own units
and daemon promotion (`5854d06bff`, `f2569d1c08`).

On 2026-11-23 Thomas ruled (D-0..D-6, recorded in
`docs/architecture/core-design.md`) to unify all BGP egress next-hop rewriting
into the export filter chain and remove `session { next-hop }` before closing
this spec, because having both `session { next-hop }` and `policy { modify {
set { next-hop } } }` creates two egress rewrite surfaces where `applyFactsNextHop`
silently overwrote export-policy next-hop sets on eBGP. This design update adds
AC-15 through AC-20: removing `session { next-hop }`, extending `policy { modify }`
and `policychain.Sets.NextHop` to `self | unchanged | auto | <ipv4/ipv6>`,
adding built-in zero-RPC `next-hop:self`, `next-hop:unchanged`, and
`next-hop:auto` chain refs, auto-appending `next-hop:auto` at the tail of every
export chain as a fill-if-unset default for forwarded routes (with MED-style
provenance checking for raw overrides), publishing all auto-inserted chain
members through one queryable implied-filter seam for `| implied` (aligned with
`plan/release/fixes/spec-bus-2-implied-display-and-grant-refusals.md`), and
migrating all repository configs, tests, netlab templates and docs off
`session { next-hop }`. Remaining AC-11 (raw FRR acceptance/redwalk +
`RFC4271-5.1.3-1` rejudgment), AC-12 (`TestMUPMappedNextHopPreserved`, LS-VPN
daemon/ExaBGP tests, `RFC7752-3.4-1` / `RFC9552-5.5-1`), and AC-14 (owner-pending
6th review round) stay inside this spec through closure.

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

Owner decision (Thomas, 2026-11-23, voice-transcribed): unify egress next-hop
rewriting into the export filter chain and drop the separate `session { next-hop }`
configuration surface so there is only one place that modifies outbound packets.
Ze automatically appends `next-hop auto` at the end of the export chain when no
earlier filter has already selected a next-hop outcome (`self`, `unchanged`, or
an explicit address), preserving the RFC 4271 Section 5.1.3 default behavior
(rewrite to self on ordinary eBGP, preserve on iBGP and RS clients). Recorded in
`docs/architecture/core-design.md` under `## Design decisions`: "I would think
that all the rewrite should be in the prefix export and not under configuration.
I believe we should automatically add to the export prefix list the auto at the
end of the list if not defined with something else. So if no one, for example,
used a self or unchanged, we should automatically change it on eBGP. Because
otherwise we have two ways to modify packets on the way out to the peer." —
"Unify into the export chain it is the way I want it."

Owner decision (Thomas, 2026-11-23, D-1): extend `policy { modify <name> { set {
next-hop self | unchanged | auto | <ip>; } } }` for conditional rules and
provide built-in chain references `next-hop:self`, `next-hop:unchanged`, and
`next-hop:auto` for peer-wide policy without a `modify` wrapper block, both
feeding the same `policychain.Sets` next-hop intent (recorded in
`docs/architecture/core-design.md` under `## Design decisions`).

Owner decision (Thomas, 2026-11-23, D-2): `next-hop:auto`, `next-hop:self`, and
`next-hop:unchanged` are built-in BGP engine steps compiled into
`policychain.Chain` (parallel to `loop-detection`, zero RPC, no
`pkg/plugin/rpc.FilterInsertion` tail extension). Config loading appends
`next-hop:auto` at the tail of every export chain unless an unconditional
`next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present,
leaving `Policy.Configured()` false for unconfigured chains; operators override
via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused (recorded in
`docs/architecture/core-design.md` under `## Design decisions`).

Owner decision (Thomas, 2026-11-23, D-3): explicit next-hop sets in an export
chain use last-write-wins folding while auto-appended trailing `next-hop:auto`
fills only when next-hop is still unset. For whole-UPDATE raw overrides
(`wireOverride`), Ze checks next-hop byte provenance against the received
UPDATE (matching `forward_med.go`): trailing `auto` is skipped only when the
raw override changed the next-hop bytes or an earlier member already selected
`unchanged`/`self`/explicit (recorded in `docs/architecture/core-design.md`
under `## Design decisions`).

Owner decision (Thomas, 2026-11-23, D-4): explicit next-hop addresses in
`policy { modify <name> { set { next-hop <ip>; } } }` and
`policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH
(through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and
import chains. Egress-only modes (`self`, `unchanged`, `auto`) — both built-in
`next-hop:*` refs and inside `policy { modify }` — are refused on import chains
at config load (recorded in `docs/architecture/core-design.md` under
`## Design decisions`).

Owner decision (Thomas, 2026-11-23, D-5): auto-appended trailing
`next-hop:auto` applies only to forwarded routes and preserves the
originator-chosen next hop on locally originated routes (static config routes,
ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions
(`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both
forwarded and locally originated routes (recorded in
`docs/architecture/core-design.md` under `## Design decisions`).

Owner decision (Thomas, 2026-11-23, D-6): every automatic filter-chain member
Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail,
`loop-detection` at the import head) and plugin-declared `FilterInsertions` —
must be published through one queryable implied-filter seam (direction,
position, reference, `implied-by` reason) so `| implied` displays every extra
chain member alongside the operator's written filter list (recorded in
`docs/architecture/core-design.md` under `## Design decisions`, and aligned with
`plan/release/fixes/spec-bus-2-implied-display-and-grant-refusals.md`). In
Thomas's words: "Can you make sure that | implied show the extra chain members
when it displays the filter ?".

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
| Destination next-hop wire precomputation | `internal/component/bgp/reactor/peer_forward_facts.go`, `precomputeNextHop`, `precomputeRedirectNextHop`, `applyFactsNextHop` | Precomputes session self/RD/link-local wire buffers and FlowSpec `0x0800` redirect-to-IP next-hop decisions from `PeerSettings.NextHopMode` and applies them unconditionally after export policy | Extends: keep the zero-allocation self/RD/link-local wire buffer precomputation, but replace `PeerSettings.NextHopMode` dispatch in both `precomputeNextHop` and `precomputeRedirectNextHop` with the export chain's resolved `policychain.Sets` next-hop intent (fast-pathing static peer-wide `auto`/`self`/`unchanged` chains) |
| Policy chain attribute sets, intents & dry-run tracing | `internal/component/bgp/policychain/sets_attributes.go`, `intents.go`, `external_attributes.go`, `chain.go`, `apply.go`, `trace.go`; `internal/component/bgp/reactor/policy_dryrun.go`, `PolicyDryRun` | `Sets.NextHop` accepts only one IPv4 address (attr 3); `intents.go` carries reactor-resolved intents (`PrependASPath`, `RemovePrivateAS`); `mergeAttributes` applies last-write-wins; `trace.go` / `PolicyDryRun` trace member steps for `show policy test` | Extends: widen `Sets.NextHop` to carry mode (`unset`, `auto`, `self`, `unchanged`, `explicit`) + IPv4/IPv6 `netip.Addr`; add built-in `next-hop:self`, `next-hop:unchanged`, `next-hop:auto` evaluators with fill-if-unset semantics for auto-appended trailing `auto`; trace those steps and widened IPv4/IPv6 next-hop sets in `show policy test` |
| Route modifier plugin | `internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang`, `config.go`, `filter_modify.go`, `modify.go` | Declares `policy { modify <name> { set { next-hop <ipv4>; } } }` and refuses IPv6 addresses at config load | Extends: widen `set { next-hop }` to union of `zt:ip-address` (IPv4 or global IPv6 via `ValidatePeerGlobalNextHop`) and enum `self`, `unchanged`, `auto`; refuse egress-only modes on import chains at config load |
| Built-in default filter insertion & chain compilation | `internal/component/bgp/config/peers.go`, `prependDefaultFilters`, `validateChainNames`; `internal/component/bgp/filterapi/policy.go`; `internal/component/bgp/config/filter_insertion.go`, `library_chains.go` (`compileLibraryChains`), `templated_chain.go` (`templatedChain`) | `prependDefaultFilters` prepends `loop-detection` at import head while keeping `Policy.Configured()` false; `insertDeclaredFilters` prepends plugin `FilterInsertions`; `validateChainNames` queues unknown `<prefix>:<name>` refs as external plugin refs; `compileLibraryChains` and `templatedChain` compile static and dynamic-group chains | Extends: add `appendDefaultExportFilters` beside `prependDefaultFilters` to append `next-hop:auto` at export tail unless an unconditional `next-hop:self`/`unchanged`/`auto` is present (refusing `inactive:next-hop:auto`), recognize built-in `next-hop:*` in `validateChainNames` before external-plugin fallback, compile built-in `next-hop:*` steps across static and dynamic-group chains, and publish all auto-inserted import/export members through one queryable implied-filter seam for `\| implied` (`spec-bus-2`) |
| Policy-filter reference completion & commit validation | `internal/component/policy/reference_source.go`, `filterReference`, `MustRegisterChainLeaf` (`ze:reference "policy-filter"`) | Populates Tab completion `Names` for every `filters` leaf-list solely from `reg.Names()` (named instances defined under `policy { ... }`) and checks each member through the attachment owner's `ChainMemberCheck` | Extends: allow attachment owners to register built-in reference names (`next-hop:self`, `next-hop:unchanged`, `next-hop:auto` on BGP export) alongside `reg.Names()` so editor autocomplete offers them and commit validates direction and `inactive:` refusal |
| BGP filter reference doctor check | `internal/component/bgp/config/filter_reference_doctor.go`, `checkFilterReferences`, `undefinedFilterRefs` (`bgp-filter-references`) | Checks every `bgp ... filter { import\|export { filters [...] } }` entry solely against `policyRegistryOf(tree, schema)` and emits `doctor-config-reference` for any name not under `policy { ... }` | Extends: recognize built-in `next-hop:self`/`unchanged`/`auto` on export chains so `ze doctor` accepts them without false `doctor-config-reference` errors while keeping undefined-reference reporting for unknown names |
| Origination default next-hop selection | `internal/component/bgp/reactor/peer.go`, `defaultOriginationNextHop`, `Peer.resolveNextHop`; `internal/component/bgp/reactor/peer_initial_sync.go`, `Peer.resolveQueuedNextHop` | Switches on `PeerSettings.NextHopMode` (`Self`, `Explicit`, `Unchanged`, `Auto`) and `PeerSettings.NextHopAddress` to choose the next hop for `next-hop default <gateway>` originated routes | Replaces `PeerSettings.NextHopMode` switch: apply the RFC 4271 §5.1.3 default (`self` on eBGP, `candidate` on iBGP unless link-local or peer-own), preserving `candidate` when peer-wide export policy selects `next-hop:unchanged`, while explicit export-chain next-hop actions (`next-hop:self`, `modify`) run uniformly at `egress_inject_filter.go` |
| Raw override attribute provenance | `internal/component/bgp/reactor/forward_med.go`, `payloadMED`, `sameAs` | Compares emitted wire bytes of a raw full-UPDATE override against the received UPDATE to distinguish deliberate filter writes from untouched received bytes | Extends: apply the same per-field wire byte comparison to `NEXT_HOP` and `MP_REACH_NLRI` next-hop so trailing `next-hop:auto` skips only when a raw override changed next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit |
| Effective IPv6 next-hop scope & refusal gates | `internal/component/bgp/reactor/link_scope.go`, `applyLinkLocalNextHop`; `forward_next_hop.go`, `applyEgressNextHopScope`, `egressNextHopWithheld` | Appends/trims RFC 2545 link-local pairs and withholds unusable or peer-own next hops after export rewriting | Preserves: continues to run after the export chain and next-hop intent materialization so policy cannot grant an RFC-forbidden next hop |
| Two forwarding paths & originated egress filter | `internal/component/bgp/reactor/reactor_api_forward.go`, `forwardUpdateSection`; `forward_rs.go`, `reactorForwardRSSection`; `egress_inject_filter.go` | Both forwarding rails run export policy then `applyFactsNextHop`; originated routes run export policy through `egress_inject_filter.go` | Extends: read the export chain's next-hop intent on both forwarding rails and originated egress; apply auto-appended `next-hop:auto` to forwarded routes only while applying explicit export next-hop actions to both forwarded and originated routes |
| Peer inspection & config syntax | `internal/component/bgp/yang/ze-bgp-conf.yang`; `internal/component/plugin/types_bgp.go`; `internal/component/bgp/plugins/cmd/peer/peer.go` | Declares `session { next-hop }` in YANG, carries `NextHopMode`/`NextHopAddress` on `PeerSettings` and `plugin.PeerInfo`, and prints `next-hop` on `show bgp peer` | Replaces: delete `session { next-hop }`, `PeerSettings.NextHopMode`/`NextHopAddress`, `applyNextHopMode`, and the `PeerInfo` fields; `show bgp peer` displays the effective export filter chain including `next-hop:*` |
| Shared MP next-hop profile | `internal/core/bgp/attribute/mpnexthop.go`, `RegisterMPNextHopProfile`; `internal/component/bgp/plugins/nlri/ls/register.go` | One registered field contract for widths, RD-per-address framing, mapping, writing and decoding across codecs, wire readers, CLI and forwarding | Preserves and extends: explicit IPv4/IPv6 export and import `Sets.NextHop` rewrites use this same profile for MP_REACH encoding and validation |
| Functional and interop harnesses | `test/plugin/`; `internal/le/interoplab/bgp/`; `contrib/netlab/ze/ze.j2` | Run the real daemon, FRR/ExaBGP interop and netlab rendering | Extends & migrates: convert all `session { next-hop }` configs to export-chain syntax (`next-hop:self`, `next-hop:unchanged`, `modify`) and add wiring/daemon tests for conditional export-chain next-hop, import refusal, raw override provenance, originated preservation and implied-filter seam |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | the door every change reads | "**A feature registers from `init()` in its own `register.go` and is discovered.** It never needs an edit to a switch, a factory, a field list, a help string or a completion table elsewhere (`ai/rules/principles.md`, `ai/patterns/registration.md`)." | agrees | BGP-LS profiles register via `attribute.RegisterMPNextHopProfile` and auto-inserted filter members publish through one shared implied-filter seam so `\| implied` discovers them without hardcoding BGP names |
| `ai/doors/bgp-engine.md` | Boundaries | door covering `internal/component/bgp/*` | "No compiled-in filter registry exists: the reactor's own steps are loop detection and the per-peer chain (`docs/architecture/core-design.md`, "All ingress filtering runs in one stage-ordered pass"), and a procedure a plugin's config implies is a declared filter insertion." | departs | Departs by adding built-in BGP engine export steps (`next-hop:self`, `next-hop:unchanged`, `next-hop:auto`) compiled into `policychain.Chain` parallel to `loop-detection` and auto-appended at config load; this door page is updated in the same work |
| `ai/doors/bgp-plugins.md` | Where it sits | door covering `internal/component/bgp/plugins/*` | "**Every one can run in another process.** A plugin runs internal (a goroutine) or external (a forked subprocess over TLS RPC), and the SDK hides which (`docs/architecture/plugin/plugin-system.md`, "The process boundary")." | agrees | Widening `bgp-filter-modify`'s `set { next-hop }` keeps answering structured `FilterSet` values over `filter-update`, so internal and forked executions behave identically |
| `ai/doors/plugin-infra.md` | Boundaries | door covering `internal/component/plugin/types_bgp.go` | "`DirectBridge` is an in-process shortcut of the same protocol, and a forked plugin always takes the RPC path. A per-route callback gets a typed bridge slot, as `filter-update` does (`rpc.DirectBridge.SetFilterSlot`, installed by `sdk.OnFilter`): one evaluator, called through the slot internal and over the batched message forked (`docs/plugin-development/handlers.md`, "Route filters")." | agrees | `policy { modify { set { next-hop } } }` reuses the existing `filter-update` `FilterSet` path on both transports while built-in `next-hop:*` stays inside the BGP engine with no `pkg/plugin/rpc.FilterInsertion` tail extension |
| `ai/doors/le-tooling.md` | Where it sits | door covering `internal/le/` and `test/` | - | silent | Covers `le`'s dispatcher, hooks, job admission and test runner without addressing BGP next-hop rewriting, export filter chains or `\| implied`; this spec alters no `le` tool or runner contract |
| `docs/architecture/core-design.md` | Policy Filter Chain | `// Design:` of `peer_forward_facts.go`, `forward_next_hop.go`, `policychain/*`, `filter_modify/*`, `peers.go`, and modified by spec | "The egress pass in `forwardUpdateCore` holds the export policy chain alone: the engine owns no egress step, and every egress procedure is a member of that chain." | departs | Departs by compiling built-in `next-hop:self\|unchanged\|auto` as BGP engine members of `policychain.Chain` with trailing auto-appended `next-hop:auto` and publishing all automatic members through one implied-filter seam, aligning the section with Thomas's 2026-11-23 Design decisions rows |
| `docs/architecture/bgp/structural-forwarding.md` | Forwarded routes and next-hop self | `// Design:` of `forward_rs.go`, anchored to `peer_forward_facts.go`, `forward_next_hop.go`, `reactor_api_forward.go`, and modified by spec | "Both relay paths read this decision from the destination's precomputed forwarding facts." | departs | Departs by removing `session { next-hop }` / `PeerSettings.NextHopMode` and driving egress next-hop rewriting from the export filter chain's resolved `policychain.Sets` intent (reusing `peerForwardFacts` self buffers and static fast path), while preserving post-policy RFC 2545/4271 scope and refusal gates |
| `docs/architecture/bgp/egress-attribute-rules.md` | How provenance is established | anchored to `reactor_api_forward.go`, `forward_rs.go`, `policychain/*`, `filter_modify/*`, and modified by spec | "The override carries no provenance, so the base is compared against the source: a metric that differs from the one received was written here." | agrees | Trailing `next-hop:auto` reuses this exact raw-override provenance comparison per next-hop field (matching `forward_med.go`), and this spec updates the page to document it alongside MED |
| `docs/architecture/bgp/fanout-dedup.md` | The decisions | anchored to `reactor_api_forward.go` | "**The identity carries the BASE as well as the digest.** Two destinations in one policy group can hold identical edit sets over DIFFERENT bases." | agrees | Export-chain next-hop sets and raw overrides continue to flow into each destination's edit set and base identity in `fwdDedupTable`, preserving per-destination wire isolation |
| `docs/architecture/bgp/filter-path-asn.md` | Constraints | anchored to `ze-bgp-conf.yang` | - | silent | Covers only the `bgp-filter-path-asn` (`reject-asn`) AS-path filter plugin and says nothing about next-hop rewriting, `session { next-hop }`, `policy modify` or `\| implied` |
| `docs/architecture/bgp/on-demand-origination.md` | The decisions | anchored to `egress_inject_filter.go` | "Export policy still applies: `egressGate` runs the destination peer's export chain at the session write gate for every originated route." | agrees | Originated routes continue to pass through `egressGate` under the export chain where explicit export next-hop actions apply while trailing `next-hop:auto` preserves originator-chosen next hops |
| `docs/architecture/wire/attributes.md` | 40. BGP_PREFIX_SID (Code 40) | `// Design:` of `rfc4271_attr_flags_egress_test.go`, anchored to `peers.go`, `filter_library.go` | "An MP carrier remains present when raw export policy temporarily leaves its next-hop field empty or undecodable. If a configured rewrite repairs that field before admission, the repaired address governs Service-TLV propagation; an unrelated legacy next hop or an unusable intermediate value cannot replace it." | departs | Departs from assuming a post-policy session-configured rewrite repairs a raw export-policy override, because `session { next-hop }` is removed and a raw override that alters next-hop bytes is not rewritten by trailing `next-hop:auto`; this page is updated in the same work |
| `docs/architecture/wire/capabilities.md` | 3. Extended Next Hop Encoding (Code 5) | anchored to `forward_next_hop.go`, `config.go`, `// Design:` of `extended_next_hop_family_scope_test.go` | "The forward rails (general and route server) apply the same check to a received route. The next hop checked is the one about to be written: the received one when it passes along unchanged, or the one a next-hop mode or a filter sets." | departs | Preserves the post-policy RFC 8950 check on the effective next hop, but departs from the two-mechanism split ("a next-hop mode or a filter") because `session { next-hop }` is removed and unified into the export filter chain; this page is updated in the same work |
| `docs/architecture/wire/nlri.md` | MP next-hop address readers | `// Design:` of `rfc8277_withdrawal_compatibility_test.go` | "`MPReachWire.NextHop` and the full-UPDATE CLI decoder read the first address through `attribute.MPNextHopProfile` with the field's actual AFI and SAFI. The LS plugin declares plain LS71 and per-address RD-framed LS72 separately, so an RD is skipped rather than read as part of the address." | agrees | Export-chain MP next-hop decoding, raw-override provenance comparison and AC-12 BGP-LS/LS-VPN framing continue to rely on `attribute.MPNextHopProfile` without normalizing received RDs |
| `docs/architecture/wire/nlri-bgpls.md` | MP_REACH next hop | anchored to `reactor_api.go`, `// Design:` of `reactor_api_local_address_test.go` | "SAFI 71 carries native IPv4 (4 octets), IPv6 (16), or IPv6 followed by link-local (32). SAFI 72 prefixes **each** address with an eight-octet zero RD, giving 12, 24, or 48 octets." | agrees | Export-chain next-hop rewriting retains AC-12's BGP-LS 4/16/32 and LS-VPN 12/24/48 per-address zero-RD framing through `MPNextHopProfile` and preserves originated `bgp-ls-export` next hops under trailing `next-hop:auto` |
| `docs/architecture/api/architecture.md` | Next-Hop Resolution | anchored to `reactor_api_forward.go`, `peer_settings.go`, `peers.go`, `config.go`, `egress_inject_filter.go`, `types_bgp.go` | "Normalization does not repair invalid address roles or field widths: those bytes remain subject to the ordinary writer's final admission. Standalone link-local and IPv4-mapped forms, capability 77, RFC 8950 negotiation and peer-own next-hop refusal retain their existing rules." | agrees | Post-policy scope normalization, final writer admission and peer-own next-hop refusal stay intact after export-chain rewriting, and D-6's implied-filter seam aligns with "Owner ruling: implied and shown, or written" |
| `docs/architecture/api/capability-contract.md` | BGP Cache Control | anchored to `peer_settings.go`, `peers.go` | - | silent | Governs API-dependent capabilities, cache lifetime commands and opaque-attribute preservation and says nothing about egress next-hop rewriting, `session { next-hop }`, `FilterSet`, `PeerInfo` or implied filters |
| `docs/architecture/api/commands.md` | Route Commands (update cursor) | `// Design:` of `cmd/peer/peer.go`, anchored to `peer_settings.go`, `trace.go`, `config.go`, `filter_library.go`, `reactor_api.go` | "The candidate can be an IGP IPv6 link-local gateway; default selection then uses the session's local address unless a configured next-hop setting overrides it." | departs | Departs by removing `session { next-hop }` (`PeerSettings.NextHopMode`), eliminating peer-level session next-hop overrides on `defaultOriginationNextHop` and removing the `session` `next-hop` field from `show bgp peer` in favor of export-chain steps; this page is updated in the same work |
| `docs/architecture/api/ipc_protocol.md` | Command-Dispatch Namespaces | anchored to `peer.go` | - | silent | Covers correlated IPC framing, answer protocol, command-dispatch namespaces and `plugin session ready` replay receipts without addressing BGP egress next-hop rewriting, `defaultOriginationNextHop` or export filter chains |
| `docs/architecture/api/json-format.md` | UPDATE Events | anchored to `peer.go` | "The MP next-hop address reader uses the actual AFI/SAFI's shared profile. Plain BGP-LS (16388/71) and RD-framed BGP-LS VPN (16388/72) therefore display their first address in the same form; the latter skips the RD before each address." | agrees | Preserves AC-12's shared `MPNextHopProfile` reader contract for BGP-LS (16388/71) and RD-framed BGP-LS VPN (16388/72) and leaves JSON UPDATE/state event envelopes unchanged under AC-15..AC-20 |
| `docs/architecture/api/process-protocol.md` | Filter Insertion | `// Design:` of `types_bgp.go`, anchored to `filter_insertion.go`, `policy.go`, `chain.go`, `filter_library.go`, `reactor_api.go`, and modified by spec | "A raw full-UPDATE rewrite stops later filters but still receives the final." | departs | Departs under D-2/D-3 because auto-appended trailing `next-hop:auto` still runs after a raw full-UPDATE override whose emitted next-hop bytes match the received UPDATE (and no earlier member set `unchanged`/`self`/explicit), while `PeerInfo` drops `NextHopMode`/`NextHopAddress` and keeps `ImportPolicy`/`ExportPolicy`; this page is updated in the same work |
| `docs/architecture/api/text-format.md` | Attribute Formats | anchored to `reactor_api.go` | - | silent | Documents text IPC event encoding for engine-to-plugin delivery, which is unchanged by export-chain next-hop unification or AC-11..14 |
| `docs/architecture/api/update-syntax.md` | Path Attributes | `// Design:` of `rfc4271_raw_originated_next_hop_test.go`, `raw_originated_error_test.go`, `register_raw_originated_next_hop.go`, anchored to `session_aigp.go`, `forward_next_hop.go`, `ze-bgp-conf.yang`, and modified by spec | "The peer's configured `self`, `unchanged` or explicit next-hop setting overrides this default. The selected address must still meet the wire-format and negotiated-capability checks." | departs | Departs by removing peer-level `session { next-hop }` overrides on `next-hop default <gateway>` in favor of export-chain next-hop actions (while `Raw Passthrough Commands` already agrees with AC-11 raw peer-own refusal); this page is updated in the same work |
| `docs/plugin-development/protocol.md` | The Sets of a filter-update answer | `// Design:` of `policychain/external_attributes.go` | "one IPv4 address" | departs | Departs by widening `FilterUpdateOutput.Sets` `next-hop` from IPv4-only to accept `"self"`, `"unchanged"`, `"auto"`, IPv4 and global IPv6 addresses (refusing egress-only modes on import); this page is updated in the same work |
| `docs/architecture/buffer-architecture.md` | When a copy is deliberate | `// Design:` of `reactor_api_forward.go`, anchored to `session_aigp.go`, and modified by spec | "A copy on the wire path MUST match one of the triggers below. A copy that fits none of them is a defect until this table names the new trigger." | agrees | Export-chain next-hop rewriting reuses `buildModifiedPayload` copy-on-modify with precomputed zero-allocation wire buffers, and AC-11 retains the recorded `writeOriginatedPeerOwnUpdate` read-pool copy trigger |
| `docs/architecture/edge-cases/extended-message.md` | Ze implementation | anchored to `session_aigp.go`, `egress_inject_filter.go`, and modified by spec | "When next-hop admission partitions a mixed originated UPDATE, it retains at most one extended-message body. It chooses this scratch pool from the input length, then sizes each permitted output chunk to the destination's negotiated send limit." | agrees | AC-11 retains this bounded extended-message scratch pool and per-chunk send-limit sizing for partitioned mixed peer-own refusals, and export-chain next-hop unification keeps final-writer splitting intact |
| `docs/architecture/edge-cases/as4.md` | Where it runs | `// Design:` of `as4_ingest_collapse_test.go` | - | silent | Covers RFC 6793 4-byte ASN negotiation, AS_PATH/AS4_PATH reconciliation and receive-side collapse, none of which this spec alters |
| `docs/architecture/update-building.md` | Path 1: Build Path (Local Origination) | `// Design:` of `rfc4271_scoped_next_hop_test.go`, anchored to `forward_next_hop.go`, `peer_forward_facts.go`, `reactor_api_forward.go`, `peer_settings.go`, `egress_inject_filter.go` | "When the destination peer sets `flowspec-redirect-next-hop true`, and the route's EXTENDED_COMMUNITIES carry 0x0800 with a 4- or 16-octet MP_REACH next hop, each destination's decision is computed once per facts snapshot:" | departs | Departs because `precomputeRedirectNextHop` switched on removed `PeerSettings.NextHopMode`; redirect-to-IP next-hop rewriting now follows the export chain's resolved next-hop intent (preserving AC-12 LS-VPN framing and AC-13 FlowSpec scope); this page is updated in the same work |
| `docs/architecture/encoding-context.md` | EncodingContext | anchored to `reactor_api_forward.go` | - | silent | Defines `NegotiatedCapabilities`, `EncodingContext`, `ContextID` and `ContextRegistry` for capability-dependent wire encoding, which this spec reads without modifying |
| `docs/architecture/forward-congestion-pool.md` | Buffer Ownership: Zero-Copy with Copy-on-Modify | anchored to `reactor_api_forward.go` | "When a destination peer requires modification (AS-PATH prepend, attribute rewrite), the egress filter path copies the payload into a buffer from the destination peer's Outgoing Peer Pool and modifies it in place. This is copy-on-modify: only peers that need changes pay for a copy; unmodified peers use the shared source buffer." | agrees | Trailing `next-hop:auto` stays a zero-copy no-op on iBGP and RS clients and acquires an Outgoing Peer Pool slot only when an export next-hop rewrite modifies the payload |
| `docs/architecture/memory/lifetime-contracts.md` | A. WireUpdate — eager copy | anchored to `reactor_api_forward.go`, `forward_rs.go` | "Both rails MUST finish body construction before publishing a materialization to the per-fan-out dedup table. A failed body MUST abandon its candidate before returning the outgoing slot; otherwise later destinations can copy a different UPDATE that has reused that slot." | agrees | Both forwarding rails keep completing export-chain next-hop body construction (or abandoning withheld/failed candidates) before publishing dedup materializations or returning outgoing slots |
| `docs/architecture/meta/role.md` | Ordering | anchored to `reactor_api_forward.go`, `config.go` | "`bgp-role:otc-ingress` is inserted at the head of each role peer's import list. While any peer or dynamic group sets `role import`, `bgp-role:otc-egress` is inserted at the head of every peer's export list (`otcInsertions`, priority 100, required, generic `any-peer` scope)." | agrees | Appending built-in `next-hop:auto` at the export tail and publishing all automatic members through the D-6 implied-filter seam preserves `bgp-role:otc-ingress` and `bgp-role:otc-egress` head ordering |
| `docs/architecture/config/syntax.md` | Filter Block | `// Design:` of `peers.go`, `config_nexthop_form.go`, anchored to `ze-bgp-conf.yang`, `policy.go`, `config.go`, `reactor_api.go`, and modified by spec | "One filter is injected rather than written: `prependDefaultFilters` (`internal/component/bgp/config/peers.go`) puts each `loop-detection` entry defined under the top-level `policy` block at the head of every peer's import chain." | departs | Departs by also auto-appending built-in `next-hop:auto` at the export tail (refusing `inactive:next-hop:auto`) and removing `session { next-hop }` / `applyNextHopMode`; this page is updated in the same work |
| `docs/architecture/config/apply-ordering.md` | Phases 2 and 5, for BGP | anchored to `peers.go` | "Neither path rewrites configured peer keys. Local listener declarations likewise retain both names until actual bound sockets can establish whether they are aliases." | agrees | AC-14 retains distinct stored listener declarations without rewriting peer keys and relies on observed bound sockets and verified claimant sets for reuse and removal |
| `docs/architecture/config/environment-block.md` | Sections | anchored to `ze-bgp-conf.yang` | - | silent | Covers only process-level `environment { }` configuration plumbing and says nothing about BGP egress next-hop rewriting, export filter chains or listener ownership |
| `docs/architecture/config/environment.md` | Overview | anchored to `ze-bgp-conf.yang` | - | silent | Documents registered environment variables and lookup precedence only, without covering BGP next-hop rewriting, export filter chains or listener ownership |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `reactor_api.go` | - | silent | Governs the stream-based config transaction lifecycle and operation ordering and is silent on BGP next-hop rewriting, export filter chain semantics and listener ownership |
| `docs/architecture/config/yang-config-design.md` | Layer 4: The BGP peer pipeline | anchored to `ze-bgp-conf.yang`, `peers.go` | "`bgp` is deliberately absent from `validatedSections`, because the BGP tree has a deeper walk of its own: `PeersFromConfigTree` resolves the group and peer layers, builds each peer's settings and filter chains, and validates the result." | agrees | Trailing `next-hop:auto` insertion, `inactive:next-hop:auto` refusal and import-chain rejection of egress-only next-hop modes live inside `PeersFromConfigTree` across all config entry points |
| `docs/architecture/plugin/plugin-system.md` | Registration fields | anchored to `peer.go`, `peer_initial_sync.go` | "The route-server plugin owns the selection for its clients through an inserted export step (`bgp-rs:transparency`, placed by `session rs-client`), served on both transports; the body builder and caches consume only the resulting treatment." | agrees | Preserves plugin registrations, `bgp-filter-modify`'s `ConfigReads`/`WantsConfig`, `bgp-rs:transparency` inserted export step, and `sendInitialRoutes` replay-fence contracts while surfacing all automatic members through the D-6 implied-filter seam |
| `docs/architecture/plugin/rib-storage-design.md` | Mandatory FlowSpec authorization | anchored to `forward_next_hop.go`, `reactor_api_forward.go` | "The advertised MP_REACH next-hop bytes are ignored for FlowSpec, including self-address and non-address values; they do not drive next-hop eligibility or interior-distance selection." | agrees | AC-13 preserves FlowSpec next-hop normalization (keeping an empty field or permitted single redirect target without letting invalid received bytes restore cleared fields) |
| `docs/architecture/rib-transition.md` | Owner ruling: a learned route needs the recipient's export policy | anchored to `ze-bgp-conf.yang`, `policy.go` | "The export final now takes an engine-derived route kind. On an internal session, absent policy rejects ordinary learned output and permits explicit local or role output; external sessions reject both." | agrees | Auto-appending trailing `next-hop:auto` leaves `Policy.Configured()` false on unconfigured export chains, preserving absent-policy rejection for ordinary learned internal output and external sessions |
| `docs/architecture/route-selection.md` | AIGP session policy | `// Design:` of `session_aigp.go`, anchored to `forward_rs.go` | "Forwarding with an unchanged next hop preserves the received attribute; next-hop-self adds the resolved interior distance or the configured source-link metric, changing only the first metric TLV." | agrees | Unifying next-hop rewriting into the export filter chain preserves post-policy AIGP behavior where unchanged next hops retain received AIGP and self rewrites add the interior or source-link metric |
| `docs/architecture/behavior/fsm.md` | States | anchored to `reactor_api.go`, `cmd/peer/peer.go` | - | silent | Documents BGP FSM states, events, timers and session notifications only, and does not cover egress next-hop rewriting, export filter chains or listener ownership |
| `docs/architecture/behavior/fsm-open-sent.md` | Events handled in OpenSent | anchored to `peer.go` | - | silent | Documents OpenSent validation, capability negotiation, BFD strict mode and hold-timer expiry without addressing BGP egress next-hop rewriting, `defaultOriginationNextHop` or export filter chains |
| `docs/architecture/behavior/peer-lifecycle.md` | Inbound connection handling for passive peers | anchored to `reactor_api.go` | "A known static peer is attributed by remote address on a shared socket. Dynamic members also qualify the actual receiving local address and listen port before longest-prefix group selection, creation or reuse." | agrees | AC-14 preserves remote-address attribution on shared sockets and actual receiving-endpoint qualification for dynamic group admission |
| `docs/architecture/cli/command-verbs.md` | Where a read verb changes the system | anchored to `policy_dryrun.go` | "`show policy test peer` runs the peer's filter chain over an operator-supplied UPDATE and installs nothing." | agrees | Extending `PolicyDryRun` to trace built-in `next-hop:*` steps and widened IPv4/IPv6 `Sets.NextHop` keeps `show policy test peer` strictly read-only with no state change |
| `docs/architecture/rfc-may-decisions.md` | RFC 4760 Section 7 - Non-Negotiated AFI/SAFI Handling | anchored to `reactor/config.go` | - | silent | Records only RFC 4760 Section 7 and RFC 7606 Section 5.1 MAY decisions and is silent on RFC 4271 Section 5.1.3 next-hop rewriting, export filter chains and listener ownership |
| `docs/architecture/doctor-and-health-checks.md` | BGP policy absences in the supplied configuration | `// Design:` of `register_originated_next_hop.go` | "The `bgp-policy-absence` post-config check uses the same peer walk and resolved same-process grant graph as config validation. It reports an ordinary internal recipient with no export policy under `doctor-bgp-learned-export-no-policy`: Ze sends it no routes learned from other peers, while local announcements remain permitted." | agrees | Auto-appending trailing `next-hop:auto` leaves `Policy.Configured()` false so unconfigured export directions continue to trigger `bgp-policy-absence`, and AC-14 agrees with `bgp-transport-scopes` failing closed on interface read errors |
| `docs/architecture/testing/ci-format.md` | Syntax Overview | anchored to `peer.go` | - | silent | Documents the `.ci` and `.et` functional test format, port leasing and runner expectations without governing BGP egress next-hop rewriting, `defaultOriginationNextHop` or export filter chains |
| `docs/architecture/testing/interop.md` | Scenario Inventory | `// Design:` of `check_rfc.go`, `check_raw_originated_next_hop.go`, `check_raw_originated_next_hop_test.go` | "`bgp-nexthop-self-local-auto-frr` retains both original self-next-hop checks and adds AIGP on the general forwarding path. The received metric is 100; source-link cost 7 and destination-link cost 43 must produce 107 and Ze's next hop at FRR." | agrees | Migrating `bgp-nexthop-self-local-auto-frr` and related scenario configs from `session { next-hop }` to export-chain syntax preserves all self-next-hop, unchanged-next-hop and AIGP wire assertions while AC-12 adds ExaBGP LS-VPN coverage |
| `docs/guide/bgp-policy.md` | Policy model | `// Design:` of `policy_reconcile_test.go` | "The effective references are cumulative: automatic safety filters run first, followed by global, group, and peer references." | departs | Departs by also auto-appending built-in `next-hop:auto` at the tail of every export chain after operator references; this guide is updated in the same work |
| `docs/guide/route-policy.md` | What Ze refuses | `// Design:` of `reference_source.go`, anchored to `peers.go`, `filter_insertion.go`, `policy.go` | "Tab on `filters` offers the filter instances this chain accepts, and only those: every entry of a filter type list under `policy` whose fields fit the attachment" | departs | Departs by extending `policy/reference_source.go` so an attachment owner (BGP export) can also offer built-in references (`next-hop:self`, `next-hop:unchanged`, `next-hop:auto`) in `filters` Tab completion alongside `policy` instances; this guide is updated in the same work |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This BGP next-hop spec changes no data-plane compatibility verifier or VPP/kernel feature refusal |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This BGP next-hop spec preserves the unified data-plane compatibility verdict across start, commit, reload and validate |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "nft"` annotation." | agrees | This BGP next-hop spec does not touch firewall YANG annotations or VPP firewall verification |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This BGP next-hop spec introduces no data-plane backend selector or implementation name |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` (D-0) | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | AC-15 through AC-20 directly implement this decision by removing `session { next-hop }` and appending `next-hop:auto` at the export chain tail |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` (D-1) | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | AC-16 implements both authoring forms over one `policychain.Sets` next-hop intent |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` (D-2) | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | AC-17 compiles `next-hop:*` as built-in BGP engine steps appended by `appendDefaultExportFilters`, leaves `Policy.Configured()` false and refuses `inactive:next-hop:auto` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` (D-3) | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | AC-18 enforces last-write-wins explicit folding, fill-if-unset trailing `auto` and per-field wire byte provenance for raw overrides |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` (D-4) | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | AC-16 widens explicit `next-hop` to IPv4 and global IPv6 on both directions via `NextHopProfile` and refuses `self`/`unchanged`/`auto` on import at config load |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` (D-5) | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | AC-19 scopes trailing `next-hop:auto` to forwarded routes while applying explicit export-chain next-hop actions uniformly to forwarded and originated routes |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` (D-6) | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | AC-20 exposes the unified implied-filter seam for both built-in engine steps and plugin `FilterInsertions`, aligned with `spec-bus-2` |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 on `update-building.md` | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | AC-15 preserves `session { flowspec-redirect-next-hop }` and AC-13 preserves FlowSpec's RFC 8955 zero-length default and opt-in redirect target |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 on `testing/interop.md` | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | BGP interop scenarios (`bgp-nexthop-auto-ebgp-frr`, `bgp-self-nexthop-withheld-frr`) declare no unrelated kernel capabilities |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 on `api/commands.md` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | D-6 surfaces implied filter chain members via the `\| implied` pipe operation rather than a subcommand or flag |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 on `api/commands.md` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | This spec overrides no generic pipe operator and preserves existing `show bgp peer` and `show policy test` answer formats |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 on `api/commands.md` | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | D-6's implied-filter seam lets `\| implied` extend displayed filter answers with automatic chain members without altering command execution |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | D-6's implied-filter seam publishes each auto-inserted member's exact ref and `implied-by` reason so `\| implied` (`spec-bus-2`) can render the same-line trailing comment naming what was added |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: peer/group/global `filter { export { filters [ ... ]; final accept|reject; } }` naming built-in `next-hop:self`, `next-hop:unchanged`, `next-hop:auto`, or a `policy { modify <name> { set { next-hop self | unchanged | auto | <ipv4/ipv6>; } } }` instance; or absent (Ze appends built-in `next-hop:auto` at the export tail while leaving `Policy.Configured()` false). `session { next-hop }` is removed.
- Wire: an UPDATE received from one peer and relayed to another, or a locally originated UPDATE (`static`, ExaBGP `announce`, API) traversing export policy.

### Transformation Path
1. Config parse, completion & validation: `ze-bgp-conf.yang` no longer accepts `session { next-hop }`; `ze-filter-modify.yang` accepts `self`, `unchanged`, `auto`, or an IPv4/global-IPv6 address (validated via `ValidatePeerGlobalNextHop`). `policy/reference_source.go` (`filterReference` / `MustRegisterChainLeaf`) exposes BGP export's built-in `next-hop:self|unchanged|auto` refs for editor Tab completion and commit validation; `bgp/config/peers.go` (`validateChainNames`) and `bgp/config/filter_reference_doctor.go` (`undefinedFilterRefs`) recognize built-in `next-hop:*` on export without falling through to external-plugin queues or `doctor-config-reference` errors. Import chains refuse `next-hop:self|unchanged|auto` and `modify` instances setting egress-only modes at config load; `inactive:next-hop:auto` is refused.
2. Default filter insertion & implied-filter seam: after `insertDeclaredFilters` and `prependDefaultFilters` (`peers.go`), `appendDefaultExportFilters` appends `next-hop:auto` at the export tail unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` unchanged. All auto-inserted chain members (built-in `loop-detection` on import, trailing `next-hop:auto` on export, and plugin `FilterInsertions`) are published through one queryable implied-filter seam (direction, position, ref, `implied-by` reason) for `| implied` (`spec-bus-2`) and `show bgp peer`.
3. Chain compilation & session facts: `compileLibraryChains` and `templatedChain` compile built-in `next-hop:*` as zero-RPC in-process members of `policychain.Chain` (for both static peers and dynamic-group templates, and traced by `PolicyDryRun` / `trace.go` in `show policy test`). At session establishment, `peerForwardFacts` (`precomputeNextHop`, `precomputeRedirectNextHop`) precomputes the session's zero-allocation self/RD/link-local wire buffers (from connected local endpoint, then `LocalAddress`, else withheld) and FlowSpec `0x0800` redirect-to-IP next-hop fast path from static peer-wide next-hop chains without per-route chain overhead.
4. Egress policy evaluation & intent materialization: for forwarded routes on both rails (`reactor_api_forward.go`, `forward_rs.go`), explicit next-hop sets use last-write-wins folding in `policychain.Sets`, while auto-appended trailing `next-hop:auto` fills only when next-hop is still unset (and for a raw `wireOverride`, compares emitted vs received next-hop bytes per field like `forward_med.go`, skipping `auto` only if the override changed next-hop bytes or an earlier member set `unchanged`/`self`/explicit). For locally originated routes (`peer.go` `defaultOriginationNextHop`, `peer_initial_sync.go` `resolveQueuedNextHop`, and `egress_inject_filter.go`), `next-hop default <gateway>` applies the RFC 4271 §5.1.3 default (preserving `candidate` when peer-wide export policy selects `next-hop:unchanged`), auto-appended trailing `next-hop:auto` is a no-op, and explicit export next-hop actions apply uniformly. `applyFactsNextHop` / `NextHopProfile` (and the FlowSpec redirect helper) materialize the resolved intent onto attr 3 and MP_REACH_NLRI.
5. Post-policy RFC gates: `applyEgressNextHopScope` applies RFC 2545 / draft-linklocal common-subnet pair normalization, and `egressNextHopWithheld` enforces missing-self, peer-own, and off-link link-local refusals before socket write.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → `policychain` / `reactor` | `ExportPolicy` / `ImportPolicy` compiled `policychain.Chain` + `ImpliedFilter` seam; `session { next-hop }` removed | Planned unit + `.ci` wiring tests (AC-15..AC-20) |
| Engine ↔ `bgp-filter-modify` plugin | Batched `filter-update` RPC `Sets` field `next-hop` carrying `"self"`, `"unchanged"`, `"auto"`, or IPv4/IPv6 address string over DirectBridge and TLS RPC | Planned internal + forked filter conformance tests (AC-16, AC-18) |
| Reactor → wire | `ModAccumulator` operations on attributes 3 and 14 via `NextHopProfile` and precomputed self buffers | IPv4, IPv6, VPN, BGP-LS and FlowSpec socket tests on cached and native relays |
| Ze → peer daemon | NEXT_HOP and MP_REACH_NLRI next-hop field | Portable daemon wiring + FRR `bgp-nexthop-auto-ebgp-frr` + `bgp-self-nexthop-withheld-frr` + ExaBGP LS-VPN single |

### Changed-Wire Census

**Search evidence:** Searched `internal/`, `cmd/`, `pkg/`, `test/`, `contrib/`, and `docs/` with `git grep` for `next-hop self|next-hop unchanged|next-hop auto`, `NextHopMode|NextHopAddress|applyNextHopMode`, `setKindNextHop`, `validateNextHop`, and `leaf next-hop` (332 matching config/doc/test occurrences across Go, `.ci`, `.conf`, `.j2`, and Markdown; positive controls: `internal/component/bgp/yang/ze-bgp-conf.yang:995`, `internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang:146`, `internal/component/bgp/reactor/config_nexthop_form.go:92`, `internal/component/bgp/policychain/external_attributes.go:99`, `internal/component/plugin/types_bgp.go:271`, `contrib/netlab/ze/ze.j2:226`).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| YANG config (`session/next-hop`, `policy/modify/set/next-hop`) | reader (schema + config parser) | `internal/component/bgp/yang/ze-bgp-conf.yang` (`session/next-hop`), `internal/component/bgp/reactor/config_nexthop_form.go` (`applyNextHopMode`), `internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang` (`set/next-hop`), `internal/component/bgp/plugins/filter_modify/config.go` (`validateNextHop`), `internal/component/bgp/config/peers.go` (`appendDefaultExportFilters`) | `session/next-hop` removed; `filter_modify` `set/next-hop` widened from IPv4-only to union of IPv4/IPv6 + `self\|unchanged\|auto`; `peers.go` appends trailing `next-hop:auto` | Change: delete `session/next-hop` and `applyNextHopMode`; regenerate YANG glue and `internal/le/site/testdata/published-yang-config-tree.json`; route origination `static/route/next-hop`, `aggregate/next-hop`, `session/link-local`, and `session/flowspec-redirect-next-hop` stay unchanged |
| YANG config (`session/next-hop`, `policy/modify/set/next-hop`) | writer (non-Go templates, functional tests, interop scenarios, docs) | `contrib/netlab/ze/ze.j2` + `contrib/netlab/golden/r1..r3.conf`; `test/plugin/*.ci` (~34 files); `test/interop/scenarios/*/ze.conf` (~24 files); `test/parse/*.ci` (4); `test/policy/policy-next-hop.ci`; `test/reload/*.ci` (3); `test/encode/*.ci` (3); `docs/guide/*.md`, `docs/architecture/*.md` | `git grep` census found `session { next-hop self\|unchanged\|auto\|<ip>; }` across all listed trees; `test/exabgp-compat/*` uses route-origination `announce route ... next-hop self` only | Change: migrate every `session { next-hop ... }` to `filter { export { filters [ next-hop:self\|unchanged ]; final accept; } }` or `policy { modify }` (or omit where default `auto` suffices); preserve `test/exabgp-compat/*` and `internal/exabgp/migration/*` route-origination syntax |
| `filter-update` RPC `FilterSet` (`field: "next-hop"`) | writer (`bgp-filter-modify`, third-party filter plugins) / reader (`policychain/external_attributes.go::setNextHop`) | `internal/component/bgp/plugins/filter_modify/modify.go` (`setText`), `internal/component/bgp/policychain/external_attributes.go` (`setNextHop`), `internal/component/bgp/policychain/sets_attributes.go` (`Sets.NextHop`), external plugins speaking `pkg/plugin/rpc` | `FilterSet` already carries JSON string value for `field: "next-hop"`; `setNextHop` currently rejects non-IPv4 strings | Extend: keep existing JSON string wire shape; accept `"self"`, `"unchanged"`, `"auto"`, IPv4, and global IPv6 strings in `setNextHop` and `Sets.NextHop` (refusing egress-only keywords on import); backward-compatible for third-party plugins sending IPv4 strings |
| Internal `plugin.PeerInfo` / `show bgp peer` answer | writer (`reactor_api.go::ListPeers`) / reader (`cmd/peer/peer.go`) | `internal/component/plugin/types_bgp.go` (`PeerInfo.NextHopMode`, `NextHopAddress`), `internal/component/bgp/reactor/reactor_api.go:212-213`, `internal/component/bgp/plugins/cmd/peer/peer.go:504-510`, `peer_shape_test.go` | `PeerInfo` copied `PeerSettings.NextHopMode` and `cmd/peer` rendered a `next-hop` row key beside `export-policy` | Change: remove `NextHopMode`/`NextHopAddress` from `PeerSettings`, `PeerInfo`, and `cmd/peer`; effective `export-policy` already renders the chain including `next-hop:auto`/`self`/`unchanged` |
| BGP wire `UPDATE` (`NEXT_HOP` attr 3, `MP_REACH_NLRI` attr 14) | writer (`reactor_api_forward.go`, `forward_rs.go`, `egress_inject_filter.go`, `peer_forward_facts.go`, `attribute/mpnexthop.go`) / reader (remote BGP peers: FRR, BIRD, GoBGP, ExaBGP, `wireu/mpwire.go`, `cli/decode_mp.go`) | `applyFactsNextHop`, `applyEgressNextHopScope`, `egressNextHopWithheld`, `writeOriginatedRawUpdate`, `NextHopProfile` | Wire encoding unchanged for RFC 4271/2545/4760/7752/9552; only the decision source moves from `session { next-hop }` to the export chain's resolved `Sets` next-hop intent (preserving LS71 4/16/32, LS72 12/24/48, FlowSpec 0/4/16, and raw peer-own refusal) | Preserve wire format and prove with unit, `.ci`, FRR (`bgp-nexthop-auto-ebgp-frr`, `bgp-self-nexthop-withheld-frr`) and ExaBGP (`bgpls-vpn-exabgp-single.ci`) peers |
| Implied-filter seam (`\| implied` consumer) | writer (`bgp/config` implied-filter provider) / reader (`spec-bus-2` `\| implied` renderer, `show bgp peer`) | `internal/component/bgp/config/peers.go`, `filter_insertion.go`, `filterapi/policy.go`; consumed by `plan/release/fixes/spec-bus-2-implied-display-and-grant-refusals.md` | Currently no unified query seam exposes built-in `loop-detection` and trailing `next-hop:auto` alongside `FilterInsertions` | Add: expose one pure-value implied-filter seam (direction, position, ref, `implied-by` reason) read by `show bgp peer` now and by `\| implied` when `spec-bus-2` lands |

### Integration Points
- `internal/component/bgp/config/peers.go` (`appendDefaultExportFilters`) + `filter_insertion.go` + `filterapi/policy.go` — append trailing `next-hop:auto` without setting `Configured()`, refuse `inactive:next-hop:auto` and import-side `next-hop:self|unchanged|auto`, and publish the unified implied-filter seam
- `internal/component/bgp/policychain/` (`sets_attributes.go`, `external_attributes.go`, `chain.go`, `chain_overlay_attributes.go`, `apply.go`, `trace.go`) — widen `Sets.NextHop` to mode+IPv4/IPv6, compile built-in `next-hop:self|unchanged|auto` zero-RPC evaluators, and trace them in `show policy test`
- `internal/component/bgp/plugins/filter_modify/` (`ze-filter-modify.yang`, `config.go`, `filter_modify.go`, `modify.go`, `register.go`) — accept `self | unchanged | auto | <ipv4/ipv6>` and reject egress-only modes on import chains at config load
- `internal/component/bgp/reactor/` (`peer_forward_facts.go`, `filter_library.go`, `reactor_api_forward.go`, `forward_rs.go`, `forward_next_hop.go`, `egress_inject_filter.go`, `config_nexthop_form.go`, `peer_settings.go`, `reactor_api.go`) — remove `PeerSettings.NextHopMode`/`NextHopAddress`, retain precomputed self/RD/link-local wire buffers + fast-path for static next-hop chains, resolve `Sets` next-hop intent (with raw-override provenance check matching `forward_med.go` and originated-route preservation for auto-appended `auto`), and keep `applyEgressNextHopScope` + `egressNextHopWithheld` post-policy
- `internal/component/plugin/types_bgp.go` and `internal/component/bgp/plugins/cmd/peer/peer.go` — remove `NextHopMode`/`NextHopAddress` fields; show effective export chain including `next-hop:*`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Every egress next-hop rewrite enters through the export filter chain (`policychain.Chain` + `Sets`), materializes through `applyFactsNextHop` / `NextHopProfile`, and passes `applyEgressNextHopScope` and `egressNextHopWithheld` before socket write |
| No unintended coupling (components stay isolated) | Yes | Built-in `next-hop:*` stays inside `internal/component/bgp` (zero `pkg/plugin/rpc` tail-insertion change); `bgp-filter-modify` communicates over standard `filter-update` `FilterSet` on both internal and forked transports; `\| implied` reads a generic seam without naming BGP filters |
| No duplicated functionality (extends existing, does not recreate) | Yes | Deletes `session { next-hop }` (`ai/rules/no-layering.md`); extends existing `policy { modify }`, `policychain.Sets`, `peerForwardFacts` self buffers, `NextHopProfile`, and `forward_med.go` provenance comparison |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Built-in `next-hop:*` steps run in-process with zero RPC and reuse `peerForwardFacts`'s precomputed immutable self/RD/link-local byte slices; static-only next-hop chains retain zero-allocation RS fast-path execution |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | BGP-LS profiles register via `attribute.RegisterMPNextHopProfile`; `bgp-nexthop-auto-ebgp-frr` registers in `specialCheckers`; implied filter members publish through a registered seam rather than being hardcoded in `\| implied` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `internal/component/bgp/config`, `policychain`, `filter_modify`, `cmd/peer`, and `interoplab`: `modify` leaf enum/union derives validation and completion from `ze-filter-modify.yang`; `next-hop:*` built-in refs register beside `loop:` in BGP chain resolution and publish into the implied-filter seam |

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
| R-11 | Migrating ~332 `session { next-hop ... }` config occurrences (tests, interop, netlab, docs) misses a file or breaks an RFC-tagged `.ci` / unit test | parse failure (`unknown leaf next-hop`) or unintended RFC-tagged test edit | grep-driven census (`task-258`), run scoped parse/plugin/unit tests after migration, and route any RFC-tagged test fixture change through independent `ze-test-judge` approval |
| R-12 | Auto-appended `next-hop:auto` accidentally marks an unconfigured export policy as `Configured() == true` and breaks RFC 8212 default-deny | eBGP peer with no explicit export policy accepts routes | preserve `Policy.Configured() == false` via `Append` (mirroring `Prepend` in `filterapi/policy.go`) and test RFC 8212 default-deny with trailing `next-hop:auto` present |
| R-13 | Adding a built-in export member slows down the zero-policy RS fast path (`forward_rs.go`) | per-route allocation or chain-run overhead on RS clients without operator export filters | recognize static-only next-hop chains (`next-hop:auto`/`self`/`unchanged` alone) in `peerForwardFacts` so zero-policy RS/eBGP/iBGP peers stay on the precomputed fast path with zero chain calls |
| R-14 | Trailing `next-hop:auto` overwrites an originated route's explicit `next-hop` (`static`, ExaBGP `announce`, API) on eBGP | ExaBGP compat or `originated-nexthop-peer-own.ci` fails because third-party NEXT_HOP is replaced with self | D-5 distinguishes auto-appended `next-hop:auto` (forwarded routes only) from explicit export `next-hop:self` / `modify` (forwarded and originated) |
| R-15 | Dynamic group template has a `$remote_as` variable or conditional `FilterInsertion` beside trailing `next-hop:auto` | `CompileResolved` drops trailing `next-hop:auto` or duplicates it after resolving peer variables | apply `appendDefaultExportFilters` deterministically on templates and verify dynamic-group parity in unit tests |

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
| Export policy absent (trailing auto-appended `next-hop:auto`), route relayed to an eBGP peer | → | `appendDefaultExportFilters` → `peerForwardFacts` / `policychain` → `applyFactsNextHop` on the forward rail | `test/plugin/nexthop-auto-ebgp-rewrite.ci` |
| Trailing auto-appended `next-hop:auto`, route relayed to an RS client | → | `forward_rs.go` keeps the received next hop | `test/plugin/nexthop-auto-rs-client-unchanged.ci` |
| Conditional `policy { modify { match ... set { next-hop <ip\|unchanged>; } } }` in export chain on eBGP | → | `policychain.Chain` runs `modify` then fill-if-unset trailing `next-hop:auto` → `applyFactsNextHop` | `test/plugin/export-chain-nexthop-precedence.ci` (planned) |
| `filter { export { filters [ next-hop:self ]; final accept; } }` on iBGP peer (and `session { next-hop }` refused at parse) | → | `ze-bgp-conf.yang` removal + built-in `next-hop:self` step → `applyFactsNextHop` | `test/plugin/nexthop-self.ci`, `test/parse/session-nexthop-removed.ci` (planned) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | IPv4 route received from peer A, relayed under `auto` to a single-hop eBGP peer B | B receives NEXT_HOP equal to Ze's address on the B session | Peers explicitly configuring `next-hop:unchanged` or a `modify` next-hop override keep their chosen next hop because `auto` only fills when unset |
| AC-2 | IPv6 route, same as AC-1 | B receives an MP_REACH next hop whose global is Ze's address on the B session; a link-local is added only under the RFC 2545 Section 3 condition | Direct-link global+link-local pairs stay permitted under RFC 2545 Section 3 when both speakers share a common subnet |
| AC-3 | Route relayed under `auto` to a multihop eBGP peer | B receives Ze's address on that session, global only | Global-only self rewrite stays permitted; link-local half is stripped because multihop peers do not share a link |
| AC-4 | `auto` towards eBGP, session has no usable local address | announcement withheld, withdrawals in the same UPDATE still sent, warning names the peer | Explicit withdrawals in the same UPDATE survive because RFC 4271 withdrawals carry no next hop |
| AC-5 | Route relayed under `auto` to an iBGP peer | received next hop kept, except the A-3 case | Received usable global next hops survive unchanged on iBGP per RFC 4271 Section 5.1.3 item 1 |
| AC-6 | Route relayed under `auto` to an RS client, either rail | received next hop kept | RS-client received next hops survive unchanged on both rails per RFC 7947 Section 2.2.1 |
| AC-7 | `auto` towards a multihop iBGP peer, route not locally originated, received next hop leaves nothing usable (A-3) | route withheld unless `self` is configured | Usable global received next hops and routes explicitly rewritten via `next-hop:self` survive because they provide a reachable multihop next hop |
| AC-8 | `unchanged`, `self`, explicit address | Preserve each mode's configured meaning: Unchanged retains the received next hop, Self uses the session's local address, and explicit uses its configured address, with the family's valid wire encoding and existing admission rules | All valid family encodings and RFC-compliant explicit/self/unchanged rewrites survive subject to post-policy admission |
| AC-9 | Received or export-policy-produced sixteen-octet Link-Local-only next hop toward multihop eBGP, with capability 77 negotiated | No Link-Local is advertised; a previously advertised prefix is withdrawn | Usable Global and explicit-Self recovery remain advertisable, and a directly attached capability-77 recipient may receive the Link-Local-only form per draft Section 4 |
| AC-10 | Existing reactor fixtures whose builders emit four-octet AS_PATH, including both queued connection generations, and adjacent writer controls with MP traffic | Their build contexts carry the configured AS identity and their final Session writers have matching real ASN4 negotiation; the original wire, split, refusal, selection and warning assertions pass across the derived caller population; checked queued output announces the exact NLRI with the replacement connection's next hop and no test-side slice panic | Explicit OLD-speaker and raw two-octet controls retain their subjects, and other shared-helper callers keep their existing family negotiation |
| AC-11 | Valid locally originated raw UPDATE, body and full-packet APIs, with NEXT_HOP equal to the recipient's address; scoped IPv6 recipients on raw and typed writers | The forbidden announcement never reaches the wire; scoped IPv6 wire-address identity ignores the interface zone while configuration, peer key and socket scope retain it; an MP32 pair naming that recipient is withheld on both raw forms and typed SendUpdate; real daemon and FRR decode evidence cover both raw forms before whole-row rejudgment | Original explicit withdrawals, pure withdrawal, EOR, valid third-party next hops, unrelated permitted family siblings, and MP32 pairs whose link-local slot is changed to a non-peer address all survive because none violates RFC 4271 Section 5.1.3 |
| AC-12 | BGP-LS received with a native IPv4 next hop, relayed on cached and native paths to directly attached or multihop destinations; Auto, Self, explicit and preserving roles; adjacent LS-VPN next-hop framing | Plain BGP-LS Auto/Self/explicit IPv4 replacements reach the socket as exactly four native octets with the exact announced NLRI; explicit recipient-owned next hop withdraws that NLRI and corrected Auto reannounces it; LS-VPN single-address output carries the required zero RD and exact 12- or 24-octet field (plus MUP mapped16 preservation and ExaBGP LS-VPN interop) | Internal and RS-client Auto and explicit Unchanged retain the received four octets; original RFC7752/RFC9552 unknown-type propagation tests retain all assertions |
| AC-13 | FlowSpec or FlowSpecVPN family normalization followed by common-subnet scope processing, with ignored invalid next-hop bytes or a permitted single redirect target | An empty next-hop field stays empty; a permitted four- or sixteen-octet redirect target retains its selected form; the exact NLRI and borrowed input remain unchanged | Ordinary IPv6 and unknown forwarding families retain their useful scope transformations because RFC 8955 zero-length scoping applies only to FlowSpec families |
| AC-14 | Interface lookup fails while static peers or dynamic groups share a listener, including retained mapped IPv4 wildcard declarations and mixed wildcard bindings | An unresolved alias never grants dedicated attribution or a known empty claimant count; stored declaration and observed endpoint identify socket reuse, removal eligibility and dynamic admission; known co-owners retain the socket and the last known claimant releases it | Remote-address attribution remains available when ownership is unknown, and other ports and global listeners retain their existing behavior |
| AC-15 | `session { next-hop ... }` removal (D-0) | `session { next-hop }` is removed from `ze-bgp-conf.yang`, `PeerSettings`, `applyNextHopMode`, `plugin.PeerInfo`, and `show bgp peer`; parsing any config with `session { next-hop ... }` fails at validate/commit/start with an error pointing to export policy (`next-hop:self`, `next-hop:unchanged`, or `policy { modify }`) | `session { link-local }`, `session { flowspec-redirect-next-hop }`, `static { route ... { next-hop ... } }`, and `aggregate { next-hop ... }` remain intact because they supply link facts, FlowSpec opt-in, and route origination rather than egress rewriting |
| AC-16 | Export-chain next-hop syntax & IPv6/MP_REACH + import refusal (D-1, D-4) | Built-in `next-hop:self`, `next-hop:unchanged`, `next-hop:auto` and `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ipv4/ipv6>; } } }` compile to one `policychain.Sets` next-hop intent (built-in steps execute in-process with zero RPC; `modify` works both internal and forked); explicit IPv4 rewrites attr 3 and global IPv6 (validated by `ValidatePeerGlobalNextHop`) rewrites MP_REACH via `NextHopProfile` on both export and import chains; egress-only modes (`self`, `unchanged`, `auto`), whether built-in refs or inside `policy { modify }`, are refused on import chains at config load | Explicit IPv4 and global IPv6 `modify { set { next-hop <ip>; } }` remain valid on import chains for inbound steering and RTBH |
| AC-17 | Auto-appended trailing `next-hop:auto` & RFC 8212 preservation (D-2) | Config loading (`appendDefaultExportFilters`) appends `next-hop:auto` at the tail of every static peer and dynamic group export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present; `Policy.Configured()` remains false when no operator export policy was written, so RFC 8212 default-deny on eBGP and ordinary iBGP learned-route gating remain intact; `inactive:next-hop:auto` is refused at config load naming `next-hop:unchanged` as the explicit override | Peers with explicit `next-hop:unchanged` or `next-hop:self` keep their declared mode with no duplicate `next-hop:auto` appended |
| AC-18 | Chain precedence, raw override provenance & post-policy RFC gates (D-3) | Explicit export-chain next-hop actions use last-write-wins folding; auto-appended trailing `next-hop:auto` acts as fill-if-unset so a conditional `modify` setting `<ip>`, `unchanged`, or `self` wins for matching routes while unmatched routes on the same eBGP peer still rewrite to self; for a whole-UPDATE raw override (`wireOverride`), Ze compares emitted vs received next-hop bytes per field (attr 3 and MP_REACH, matching `forward_med.go`): trailing `auto` is skipped when the raw override changed the next-hop bytes or an earlier member set `unchanged`/`self`/explicit, and still applies when the raw override left next-hop bytes identical to the received UPDATE; post-policy `applyEgressNextHopScope` and `egressNextHopWithheld` still enforce RFC 2545 pair rules, missing-self, peer-own, and off-link link-local refusals | Unaffected siblings, non-matching routes, and deliberate raw next-hop overrides survive with their resolved next hop |
| AC-19 | Originated routes vs. forwarded routes (D-5) | Auto-appended trailing `next-hop:auto` applies only to forwarded routes, preserving originator-chosen next hops on static config routes, ExaBGP `announce route ... next-hop <ip>`, and API announcements toward eBGP and iBGP peers; explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes (subject to post-policy RFC admission); `defaultOriginationNextHop` uses RFC 4271 Section 5.1.3 default (respecting peer-wide `next-hop:unchanged` for `next-hop default <gateway>`) | ExaBGP compat tests and static/API routes with explicit next hops keep their advertised next hop under default export chains |
| AC-20 | Implied-filter seam & repo-wide migration (D-6) | Every auto-inserted chain member (built-in `loop-detection` at import head, trailing `next-hop:auto` at export tail, and plugin `FilterInsertions`) is published through one queryable implied-filter seam carrying direction, position, member ref, and `implied-by` reason for `\| implied` (`spec-bus-2`) and shown in `show bgp peer` and `show policy test`; every repository config, `.ci` test, interop `ze.conf`, `contrib/netlab/ze/ze.j2` + golden config, reactor unit test, and documentation page migrates off `session { next-hop }` cleanly with no remaining `session { next-hop }` syntax | Existing `bgp-rs:transparency`, `bgp-role:otc-ingress`/`otc-egress`, and `loop-detection` automatic filter behaviors survive unchanged and become visible through the shared seam |

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
| `TestAppendDefaultExportNextHopAuto`, `TestImpliedFilterSeamIncludesEngineAndPluginInsertions`, `TestImportRefusesEgressNextHopModes`, `TestInactiveNextHopAutoRefused` | `internal/component/bgp/config/export_nexthop_default_test.go` | AC-15, AC-16, AC-17, AC-20: trailing `next-hop:auto` append without setting `Configured()`, dynamic-group parity, `inactive:next-hop:auto` refusal, import-direction refusal of `self`/`unchanged`/`auto`, and unified implied-filter seam | planned |
| `TestPolicyChainNextHopIntentAndPrecedence`, `TestPolicyChainNextHopIPv6MPReach` | `internal/component/bgp/policychain/nexthop_intent_test.go` | AC-16, AC-18: built-in `next-hop:self\|unchanged\|auto` zero-RPC evaluation, last-write-wins explicit folding, fill-if-unset trailing `auto`, IPv4+IPv6 `Sets.NextHop` via `NextHopProfile`, and `show policy test` trace output | planned |
| `TestModifyNextHopUnionAndImportRefusal` | `internal/component/bgp/plugins/filter_modify/config_test.go` | AC-16: `modify { set { next-hop self \| unchanged \| auto \| <ipv4> \| <ipv6>; } }` config parsing, `ValidatePeerGlobalNextHop` refusal of link-local/unspecified, and internal+forked `filter-update` delivery | planned |
| `TestExportChainNextHopRawOverrideProvenance`, `TestExportChainNextHopOriginatedVsForwarded`, `TestStaticNextHopFastPathNoPerRouteChain` | `internal/component/bgp/reactor/export_nexthop_chain_test.go` | AC-18, AC-19: conditional `modify` wins over trailing `auto` on eBGP while unmatched routes rewrite to self; raw `wireOverride` byte-provenance check (like `forward_med.go`); originated routes preserved under trailing `auto` and rewritten under explicit `next-hop:self`/`modify`; static fast path preserves RS throughput | planned |
| `TestMUPMappedNextHopPreserved` | `internal/component/bgp/plugins/nlri/mup/next_hop_mapped_test.go` | AC-12 step 3a: MUP 2/85 mapped16 preservation through `config.go`, `encode.go`, `peer_static_routes.go` and `BuildPlugin` | planned |

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
| `originated-nexthop-peer-own` oracle repair | `test/plugin/` | Preserve the typed cases, assert each useful announcement and its exact next hop in the same UPDATE, and permit lawful withdrawals of the refused subject | Promoted with native positive/negative records in `5854d06bff` |
| `export-chain-nexthop-precedence` | `test/plugin/` | Conditional `modify { set { next-hop <ip\|unchanged>; } }` beats trailing `next-hop:auto` for matched routes while unmatched routes on eBGP rewrite to self, plus explicit `next-hop:self` on iBGP and originated-route preservation | planned (draft under `test/draft/plugin/` first) |
| `session-nexthop-removed` | `test/parse/` | Config using removed `session { next-hop ... }` fails validation cleanly; `next-hop:self` and `next-hop:unchanged` in export chain parse and load; `inactive:next-hop:auto` and import `next-hop:self` are refused | planned |
| `bgpls-vpn-next-hop-framing` | `test/plugin/` | AC-12 step 7: real daemon LS71/LS72 Auto relay and plugin-origin singles | planned (draft under `test/draft/plugin/` first) |
| `bgpls-vpn-exabgp-single` | `test/plugin/` | AC-12 step 8: pinned ExaBGP `3790c532eb` receives LS72 IPv4/IPv6 singles and withdrawal from Ze | planned (draft under `test/draft/plugin/` first) |

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
- `internal/component/bgp/yang/ze-bgp-conf.yang` - remove `session { next-hop }` leaf (keep `session { link-local }` and `session { flowspec-redirect-next-hop }`); regenerate YANG glue
- `internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang` - widen `set { next-hop }` to union of `zt:ip-address` (IPv4 or global IPv6) and enum `self`, `unchanged`, `auto`; regenerate YANG glue
- `internal/component/bgp/plugins/filter_modify/config.go`, `filter_modify.go`, `modify.go`, `register.go` - accept IPv4/global-IPv6 and `self`/`unchanged`/`auto`, and refuse egress-only modes on import chains at config load
- `internal/component/bgp/config/peers.go`, `filter_insertion.go`, `filter_reference_doctor.go`, `library_chains.go`, `templated_chain.go` - add `appendDefaultExportFilters` (trailing `next-hop:auto`, preserving `Policy.Configured()`, refusing `inactive:next-hop:auto` and import `next-hop:*`), recognize built-in `next-hop:*` in `validateChainNames` and `bgp-filter-references` (`undefinedFilterRefs`), compile built-in `next-hop:*` chain members across static peers and dynamic-group templates, and publish the unified implied-filter seam (built-in `loop-detection`, trailing `next-hop:auto`, and plugin `FilterInsertions`)
- `internal/component/policy/reference_source.go` - allow attachment owners (`MustRegisterChainLeaf`) to declare built-in references (`next-hop:self`, `next-hop:unchanged`, `next-hop:auto` on BGP export) for `ze:reference "policy-filter"` editor autocomplete and commit checks
- `internal/component/bgp/filterapi/policy.go` - add `Append` preserving `Configured()` and carry implied-filter provenance metadata
- `internal/component/bgp/policychain/sets.go`, `sets_attributes.go`, `external_attributes.go`, `chain.go`, `chain_overlay_attributes.go`, `apply.go`, `trace.go` - widen `Sets.NextHop` to mode (`unset|auto|self|unchanged|explicit`) + IPv4/IPv6 `netip.Addr`, register built-in zero-RPC `next-hop:self|unchanged|auto` evaluators with fill-if-unset semantics for auto-appended `auto`, support IPv4/IPv6 MP_REACH application and `show policy test` tracing
- `internal/component/bgp/reactor/config.go`, `config_nexthop_form.go`, `peer_settings.go`, `peer.go`, `peer_initial_sync.go`, `peer_forward_facts.go`, `filter_library.go`, `policy_dryrun.go`, `reactor_api_forward.go`, `forward_rs.go`, `forward_next_hop.go`, `egress_inject_filter.go`, `reactor_api.go` - remove `PeerSettings.NextHopMode`/`NextHopAddress` and `applyNextHopMode`; replace `PeerSettings.NextHopMode` switches in `defaultOriginationNextHop`, `resolveQueuedNextHop`, `precomputeNextHop`, and `precomputeRedirectNextHop`; trace built-in `next-hop:*` in `PolicyDryRun`; keep precomputed self/RD/link-local wire buffers + static fast path; apply resolved `Sets` next-hop intent with `forward_med.go`-style raw-override byte provenance and originated-route preservation under trailing `auto`; keep post-policy `applyEgressNextHopScope` and `egressNextHopWithheld`
- `internal/component/plugin/types_bgp.go` - remove `NextHopMode` and `NextHopAddress` from `PeerInfo`
- `internal/component/bgp/plugins/cmd/peer/peer.go` - remove `session.next-hop` display field; effective `export-policy` displays `next-hop:*`
- `contrib/netlab/ze/ze.j2` - migrate explicit `session { next-hop self; }` blocks to export policy (`filter { export { filters [ next-hop:self ]; final accept; } }` for iBGP, omit for eBGP default `auto`) and refresh `contrib/netlab/golden/r1.conf`, `r2.conf`, `r3.conf`
- `docs/architecture/core-design.md`, `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/bgp/egress-attribute-rules.md`, `docs/architecture/config/syntax.md`, `docs/architecture/wire/attributes.md`, `docs/architecture/wire/capabilities.md`, `docs/architecture/api/commands.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/update-syntax.md`, `docs/architecture/update-building.md`, `docs/plugin-development/protocol.md`, `ai/doors/bgp-engine.md`, `docs/guide/bgp-policy.md`, `docs/guide/route-policy.md`, `docs/guide/configuration.md` - update next-hop policy architecture, door boundaries, wire/protocol docs, config syntax, examples and source anchors for export-chain unification
- `test/plugin/*.ci`, `test/interop/scenarios/*/ze.conf`, `test/parse/*.ci`, `test/policy/policy-next-hop.ci`, `test/reload/*.ci`, `test/encode/*.ci`, and reactor unit tests (`internal/component/bgp/reactor/*_test.go`) - migrate all `session { next-hop }` configs and `PeerSettings.NextHopMode` fixtures to export-chain next-hop syntax (independent `ze-test-judge` approval for any RFC-tagged test fixture)
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
- `internal/component/bgp/config/export_nexthop_default_test.go` - AC-15, AC-16, AC-17, AC-20 unit tests for trailing `next-hop:auto`, `Policy.Configured()`, `inactive:next-hop:auto` refusal, import refusal, and the implied-filter seam
- `internal/component/bgp/policychain/nexthop_intent_test.go` - AC-16, AC-18 unit tests for built-in `next-hop:*` evaluators, last-write-wins vs fill-if-unset folding, IPv4+IPv6/MP_REACH sets, and tracing
- `internal/component/bgp/reactor/export_nexthop_chain_test.go` - AC-18, AC-19 unit tests for conditional `modify` precedence, raw `wireOverride` byte provenance, originated vs forwarded routes, and static fast path
- `internal/component/bgp/plugins/nlri/mup/next_hop_mapped_test.go` - AC-12 step 3a MUP 2/85 mapped16 preservation test
- `test/plugin/export-chain-nexthop-precedence.ci` - AC-16, AC-18, AC-19 daemon wiring test
- `test/parse/session-nexthop-removed.ci` - AC-15, AC-16, AC-17 config parse/validation test
- `test/plugin/bgpls-vpn-next-hop-framing.ci` - AC-12 step 7 LS-VPN daemon framing test
- `test/plugin/bgpls-vpn-exabgp-single.ci` - AC-12 step 8 real ExaBGP LS-VPN single interop test

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | Remove `session { next-hop }` in `internal/component/bgp/yang/ze-bgp-conf.yang`; widen `set { next-hop }` union in `internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang`; run `./le yang glue update` |
| YANG validation constraints | Yes | `ze-filter-modify.yang` native union (`zt:ip-address` + `enumeration { self; unchanged; auto; }`); `config.go` enforces `ValidatePeerGlobalNextHop` and import-direction refusal |
| YANG custom validators | No | Standard chain validation in `internal/component/bgp/config/peers.go` refuses `inactive:next-hop:auto` and import-side `next-hop:self\|unchanged\|auto` |
| CLI commands/flags | Yes | `internal/component/bgp/plugins/cmd/peer/peer.go` drops `session.next-hop`; effective `export-policy` shows `next-hop:*`; `show policy test` traces built-in `next-hop:*` |
| CLI grammar (keyword before value) | N-A | No new CLI verb or keyword added |
| Editor autocomplete | Yes | Automatic from `ze-filter-modify.yang` union; filter-chain completion includes built-in `next-hop:self`, `next-hop:unchanged`, `next-hop:auto` |
| Functional test for new RPC/API | Yes | `test/plugin/export-chain-nexthop-precedence.ci`, `test/parse/session-nexthop-removed.ci`, `test/plugin/nexthop-auto-ebgp-rewrite.ci`, `test/plugin/nexthop-auto-rs-client-unchanged.ci` |
| Pipe completeness | Yes | `show bgp peer` and `show policy test` continue through standard `ApplyPipes`; implied-filter seam feeds `\| implied` in `spec-bus-2` |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Doctor code per problem class | Yes | Existing `doctor-bgp-originated-next-hop-peer-own` (`register_originated_next_hop.go`) retained; config refusals return actionable parse/load errors naming `next-hop:unchanged` or export policy |
| Prometheus counters/metrics | No | Existing next-hop withholding warnings and counters retained; no new metric needed |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new family; BGP-LS SAFI 71/72 profiles registered via `attribute.RegisterMPNextHopProfile` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `docs/features/bgp-protocol.md` (replace `session { next-hop }` mentions with export-chain next-hop) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md`, `internal/component/bgp/yang/ze-bgp-conf.yang`, `internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang` |
| 3 | CLI command added/changed? | Yes | `show bgp peer` drops `session.next-hop` (effective export policy shows `next-hop:*`); verify `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md` / `docs/plugin-development/protocol.md` (`FilterSet` `next-hop` now accepts `"self"`, `"unchanged"`, `"auto"`, IPv4, IPv6) |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` (`bgp-filter-modify` next-hop union and IPv6 support) |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-policy.md`, `docs/guide/configuration.md`, `docs/guide/netlab.md`, `docs/guide/quickstart.md`, `docs/guide/as112.md`, `docs/guide/flowspec-route-reflector.md`, `docs/guide/healthcheck.md` |
| 7 | Wire format changed? | No | Wire format unchanged; BGP-LS/LS-VPN framing docs (`attributes.md`, `nlri-bgpls.md`) already updated |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/plugin-development/protocol.md` (widened `FilterSet` `next-hop` values; `pkg/plugin/rpc.FilterInsertion` unchanged per D-2) |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4271.md` Section 5.1.3 default rows; `rfc/short/draft-ietf-idr-linklocal-capability.md`; `rfc/short/rfc7752.md`, `rfc/short/rfc9552.md`, `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/interop.md` (ExaBGP LS-VPN single fixture and migrated test examples) |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` has no stale `session { next-hop }` claim |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/bgp/egress-attribute-rules.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none planned |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Run `./le spec citation anchors spec plan/release/fixes/spec-bgp-next-hop-auto-rewrites-for-ebgp.md` and update every anchored page affected by removing `session { next-hop }` (`structural-forwarding.md`, `egress-attribute-rules.md`, `core-design.md`, `syntax.md`, `update-building.md`, `api/commands.md`, `api/update-syntax.md`) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Convert all `session { next-hop ... }` examples in `docs/`, `contrib/netlab/ze/ze.j2`, and `contrib/netlab/golden/` to export-chain syntax |

## Design presentation

### What changes

All BGP egress next-hop rewriting moves into the export filter chain, and `session { next-hop }` is removed so there is only one place that modifies outbound routes. Operators write `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` directly in an export filter list for peer-wide behavior, or `policy { modify <name> { set { next-hop self | unchanged | auto | <ipv4/ipv6>; } } }` for conditional/per-prefix rules. When no unconditional `next-hop:*` member is present, Ze automatically appends built-in `next-hop:auto` at the tail of every export chain (zero RPC, keeping `Policy.Configured()` false so RFC 8212 default-deny is unchanged). Trailing `next-hop:auto` fills in only when no earlier filter set a next hop (and for raw full-UPDATE overrides, only when the override left next-hop bytes unchanged), applies only to forwarded routes (leaving static, ExaBGP, and API originated next hops intact), and is published through a shared implied-filter seam so `| implied` (and `show bgp peer`) displays every extra chain member alongside the written filter list. Post-policy RFC 2545 scope normalization and RFC 4271 §5.1.3 / draft-linklocal refusal gates stay in the engine after policy.

### Before and after

1. **Peer-wide iBGP `next-hop self` (and netlab iBGP mesh):**
   - **Before:** configured under `session { next-hop self; }` beside a separate `filter { export { ... } }` block; `show bgp peer` printed a separate `next-hop: self` field.
   - **After:** configured once in `filter { export { filters [ next-hop:self ]; final accept; } }`; `session { next-hop self; }` is rejected at parse/validate with a message pointing to export policy, and `show bgp peer` displays `next-hop:self` inside the export filter list.
2. **Per-prefix next-hop override on an eBGP peer (e.g. RTBH or traffic engineering):**
   - **Before:** writing `policy { modify bh { match { community [ 65000:666 ]; } set { next-hop 192.0.2.66; } } }` in an eBGP export chain was silently overwritten by `session { next-hop auto; }` running after the export chain (and IPv6 next hops in `modify` were refused at config load).
   - **After:** matching routes receive `192.0.2.66` (or an IPv6 global next hop on MP_REACH) and skip the trailing fill-if-unset `next-hop:auto`, while non-matching routes on the same eBGP peer still reach trailing `next-hop:auto` and rewrite to Ze's session address.
3. **Preserving third-party next hop on eBGP:**
   - **Before:** `session { next-hop unchanged; }`.
   - **After:** `filter { export { filters [ next-hop:unchanged ]; final accept; } }` (attempting `inactive:next-hop:auto` is refused and points to `next-hop:unchanged`).
4. **Viewing implied chain members (`| implied` and `show bgp peer`):**
   - **Before:** `session { next-hop auto; }` was invisible in the export filter chain, and built-in auto-inserted members (`loop-detection`) were not exposed through a shared implied-filter seam.
   - **After:** `show bgp peer` shows `next-hop:auto` at the export tail, and the shared implied-filter seam publishes both built-in steps (`loop-detection` at import head, `next-hop:auto` at export tail) and plugin `FilterInsertions` with their `implied-by` reasons for `| implied` (`spec-bus-2`).
5. **Import-chain next-hop rules:**
   - **Before:** `policy { modify { set { next-hop <ipv4>; } } }` worked on import for IPv4 only; IPv6 was refused.
   - **After:** explicit IPv4 and global IPv6 next hops work on both import and export chains, while egress-only modes (`self`, `unchanged`, `auto`, built-in or in `modify`) are refused on import chains at config load.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Keep `session { next-hop }` beside export policy and only change precedence | Leaves two egress rewrite surfaces to reason about and configure; Thomas ruled on 2026-11-23 (`a7f7eb14d2`) to unify into the export chain and drop `session { next-hop }` |
| Require a `policy { modify }` wrapper block for peer-wide `self`/`unchanged` (no built-in `next-hop:*` refs) | Adds boilerplate on every iBGP `next-hop self` and third-party eBGP peer and couples basic peer-wide next-hop to loading `bgp-filter-modify`; Thomas chose Option A (D-1) |
| Create a separate `policy { next-hop <name> }` filter type | Duplicates `set { next-hop }` already present in `policy { modify }` (`ai/rules/no-layering.md`); rejected under D-1 |
| Host trailing `next-hop:auto` as a plugin `rpc.FilterInsertion` with a new `tail` position | Widens the public `pkg/plugin/rpc` external-plugin contract and puts mandatory RFC 4271 §5.1.3 default next-hop behind a plugin boundary that can be forked or unloaded; rejected under D-2 |
| Skip trailing `next-hop:auto` on any raw `wireOverride` | Any unrelated raw export filter (e.g., editing communities) would silently leak received third-party next hops on eBGP; rejected under D-3 in favor of `forward_med.go`-style provenance |
| Apply trailing `next-hop:auto` to locally originated routes too | Would silently overwrite explicit `next-hop` on static config routes and ExaBGP `announce route ... next-hop <ip>` on eBGP; rejected under D-5 |

### Documents departed from

- `ai/doors/bgp-engine.md` (section `Boundaries`): currently states that the reactor's own steps are loop detection and the per-peer chain. After this work, it also names built-in BGP engine export steps (`next-hop:self`, `next-hop:unchanged`, `next-hop:auto`) compiled into `policychain.Chain` parallel to `loop-detection`.
- `docs/architecture/core-design.md` (section `Policy Filter Chain`): currently states that the engine owns no egress step. After this work, it documents built-in `next-hop:*` steps compiled into `policychain.Chain`, trailing auto-appended `next-hop:auto` (keeping `Policy.Configured()` false), widened `Sets.NextHop`, and the shared implied-filter seam, matching the 2026-11-23 Design decisions rows.
- `docs/architecture/bgp/structural-forwarding.md` (section `Forwarded routes and next-hop self`): currently describes `session { next-hop self|auto|unchanged|<ip> }` driving `PeerSettings.NextHopMode` and `precomputeNextHop` directly after export policy. After this work, `session { next-hop }` is removed, egress next-hop selection lives in the export filter chain (`next-hop:self|unchanged|auto` and `policy { modify }`) with trailing auto-appended `next-hop:auto`, and `peerForwardFacts` supplies the precomputed self/RD/link-local wire buffers and static fast path driven by the chain's resolved intent.
- `docs/architecture/wire/attributes.md` (section `40. BGP_PREFIX_SID (Code 40)`): currently states that a configured rewrite after raw export policy can repair an empty or undecodable MP next-hop field before admission. After this work, `session { next-hop }` is removed and a raw override that alters next-hop bytes is not rewritten by trailing `next-hop:auto`; `applyEgressPrefixSIDNextHop` continues to compare final vs received next-hop entities.
- `docs/architecture/wire/capabilities.md` (section `3. Extended Next Hop Encoding (Code 5)`): currently names two separate sources ("a next-hop mode or a filter"). After this work, next-hop modes live inside the export filter chain; the post-policy RFC 8950 check on the effective outgoing next hop stays unchanged.
- `docs/architecture/api/commands.md` (section `Route Commands (update cursor)`): currently states that a peer-level configured next-hop setting overrides `defaultOriginationNextHop` and `show bgp peer` reports a `session` `next-hop` field. After this work, peer-level overrides move to export-chain next-hop actions and `show bgp peer` renders `next-hop:*` inside `export-policy`.
- `docs/architecture/api/process-protocol.md` (section `Filter Insertion`): currently states that a raw full-UPDATE rewrite stops later filters unconditionally, and `PeerInfo` carries `NextHopMode`/`NextHopAddress`. After this work, auto-appended trailing `next-hop:auto` still runs after a raw full-UPDATE override whose emitted next-hop bytes match the received UPDATE (when no earlier member set `unchanged`/`self`/explicit), and `PeerInfo` drops `NextHopMode`/`NextHopAddress` while keeping `ImportPolicy`/`ExportPolicy`.
- `docs/architecture/api/update-syntax.md` (section `Path Attributes`): currently states that a peer's configured `self`, `unchanged` or explicit session next-hop overrides `next-hop default <gateway>`. After this work, that override comes from explicit export-chain next-hop actions instead of `session { next-hop }`.
- `docs/plugin-development/protocol.md` (section `The Sets of a filter-update answer`): currently states that `FilterUpdateOutput.Sets` `next-hop` accepts `one IPv4 address`. After this work, it accepts `"self"`, `"unchanged"`, `"auto"`, IPv4 and global IPv6 addresses (refusing egress-only modes on import).
- `docs/architecture/update-building.md` (section `Path 1: Build Path (Local Origination)`): currently describes `precomputeRedirectNextHop` computing a destination's redirect decision once per facts snapshot from `PeerSettings.NextHopMode`. After this work, FlowSpec redirect-to-IP next-hop rewriting follows the export chain's resolved next-hop intent (preserving static fast path when the chain has no conditional filters).
- `docs/architecture/config/syntax.md` (section `Filter Block`): currently states that only `loop-detection` is injected (`prependDefaultFilters`) and documents `session { next-hop }` under session transport. After this work, `session { next-hop }` is removed and `appendDefaultExportFilters` appends trailing `next-hop:auto` on export (refusing `inactive:next-hop:auto`).
- `docs/guide/bgp-policy.md` (section `Policy model`): currently states only that automatic safety filters run first before operator references. After this work, it documents built-in `next-hop:*` refs, widened `policy { modify { set { next-hop } } }`, trailing auto-appended `next-hop:auto`, and import-chain refusal of egress-only modes.
- `docs/guide/route-policy.md` (section `What Ze refuses`): currently states that Tab on `filters` offers only entries of filter type lists under `policy`. After this work, an attachment owner can also declare built-in references (BGP export declares `next-hop:self`, `next-hop:unchanged`, `next-hop:auto`) that Tab offers and commit validates alongside `policy` instances.

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| D-0 | Unify egress next-hop into export chain and drop `session { next-hop }` | (a) Unify into export chain (recommended); (b) Keep `session { next-hop }` | (a) Unify into export chain, pivoting this spec now (`a7f7eb14d2`) |
| D-1 | Operator syntax | (a) Extend `policy { modify }` (`self\|unchanged\|auto\|<ip>`) + built-in `next-hop:self\|unchanged\|auto` (recommended); (b) `modify` only; (c) Separate `policy { next-hop }` | (a) Selected (`390086adb8`) |
| D-2 | Trailing `next-hop:auto` ownership | (a) Built-in BGP engine step appended at config load, no `inactive:next-hop:auto` (recommended); (b) Plugin tail `rpc.FilterInsertion` | (a) Selected (`390086adb8`) |
| D-3 | Precedence & raw overrides | (a) Last-write-wins explicit, fill-if-unset trailing `auto`, MED-style wire provenance on raw `wireOverride` (recommended); (b) Skip trailing `auto` on any raw override | (a) Selected (`390086adb8`) |
| D-4 | IPv6/MP_REACH & import direction | (a) IPv4 + IPv6/MP_REACH explicit on export and import; refuse `self`/`unchanged`/`auto` on import at config load (recommended); (b) IPv6 on export only | (a) Selected (`390086adb8`) |
| D-5 | Originated vs. forwarded routes | (a) Trailing `next-hop:auto` applies to forwarded routes only; explicit export next-hop applies to both (recommended); (b) Trailing `auto` also rewrites originated routes | (a) Selected (`390086adb8`) |
| D-6 | `\| implied` display of extra chain members | Publish built-in (`loop-detection`, trailing `next-hop:auto`) and plugin `FilterInsertions` via one queryable implied-filter seam for `\| implied` | Requested and recorded (`ee8d115eb6`) |
| AC-14 | Listener ownership review round 6 (`9b6f3c8708` hit 5-round review cap) | (a) Authorize 6th review round (`--owner-authorised`) to close AC-14 inside this spec; (b) Split AC-14 into its own spec so it does not hold this closure | (a) Authorized by Thomas on 2026-11-25: run 6th independent review round on `9b6f3c8708`, fix findings, and close inside this spec |

## Implementation Steps

1. **Phase: Completed baseline (already landed)** -- AC-1..AC-10 core/daemon/FRR proof (`ae01917413`, `3a0bc5ef46`, `c562cdde0b`), AC-12 shared `NextHopProfile` and LS71/LS72 unit coverage (`ff7032975a`, `52e4ae832a`, `771ba0d96d`), AC-13 FlowSpec scope (`63afaf0c95`), AC-14 listener ownership code (`9b6f3c8708`, pending 6th review round), and AC-11 raw/scoped peer-own units and promoted `.ci` tests (`5854d06bff`, `f2569d1c08`).
2. **Phase: Export-chain unification — schema, `policychain`, and `filter_modify` (AC-15, AC-16, AC-17, AC-20)** -- remove `session { next-hop }` from `ze-bgp-conf.yang`, widen `ze-filter-modify.yang` `set { next-hop }` to `self | unchanged | auto | <ip>`, regenerate YANG glue, widen `policychain.Sets.NextHop` + `external_attributes.go` + `trace.go`, add built-in zero-RPC `next-hop:self|unchanged|auto` evaluators, add `appendDefaultExportFilters` and the unified implied-filter seam in `bgp/config` + `filterapi`, and write unit tests (`export_nexthop_default_test.go`, `nexthop_intent_test.go`, `filter_modify/config_test.go`).
3. **Phase: Export-chain unification — reactor rails, `cmd/peer`, and repo-wide config/test migration (AC-15..AC-20)** -- remove `PeerSettings.NextHopMode`/`NextHopAddress`, `applyNextHopMode`, and `PeerInfo` fields; wire export-chain next-hop intent into `peer_forward_facts.go` (preserving precomputed self buffers and static fast path), `reactor_api_forward.go`, `forward_rs.go`, `egress_inject_filter.go` (with `forward_med.go`-style raw-override provenance and originated-route preservation); update `cmd/peer`; migrate all `session { next-hop }` configs, tests, interop scenarios, `contrib/netlab/ze/ze.j2` + golden configs, and docs (with `ze-test-judge` approvals for any RFC-tagged test fixture); add `export_nexthop_chain_test.go`, `test/plugin/export-chain-nexthop-precedence.ci`, and `test/parse/session-nexthop-removed.ci`.
4. **Phase: Remaining AC-11 & AC-12 proofs** -- run `check_raw_originated_next_hop_test.go`, live FRR `bgp-self-nexthop-withheld-frr` acceptance + redwalk, clear `RFC-approval-owed` (`3573004e78`) for `bgp.checkSelfNextHopWithheld`, re-judge `RFC4271-5.1.3-1`; add `TestMUPMappedNextHopPreserved`, draft/stress/promote `bgpls-vpn-next-hop-framing.ci` and `bgpls-vpn-exabgp-single.ci`, and clear `RFC7752-3.4-1` / `RFC9552-5.5-1`.
5. **Phase: AC-14 resolution, scoped verification, Review Gate and closure** -- resolve AC-14 per Thomas's choice on the 6th review round, run scoped unit/functional/lint/`./le rfc check`/`./le doc check` gates, pass `/ze-review` gate, and close in Commits A + B.

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


---

## Implementation Summary

### What Was Implemented
- **Default eBGP Auto next-hop & boundary semantics (AC-1..AC-10):** `ae01917413`, `3a0bc5ef46`, `c562cdde0b` implemented RFC 4271 Section 5.1.3 default next-hop-self on ordinary eBGP while preserving received next hops on iBGP and RS clients, added RFC 2545 / draft-linklocal boundary and bare-link-local proofs, removed dead `applyNextHopMod`, corrected the RFC 7947 Section 2.2.1 cite, and repaired ASN4/identity test fixtures.
- **AC-11 raw & scoped peer-own next-hop refusal:** `5854d06bff`, `f2569d1c08`, `5ef6ed17f5` enforced RFC 4271 Section 5.1.3 peer-own next-hop refusal on recognized raw body/full-packet origination (`session_aigp.go::writeOriginatedRawUpdate`), compared scoped IPv6 wire identity zone-free (`forward_next_hop.go::nextHopValue.has`), registered `doctor-bgp-originated-next-hop-peer-own`, added daemon/unit proofs, ran live FRR 10.7.1 `bgp-self-nexthop-withheld-frr` acceptance + semantic redwalk, and re-judged `RFC4271-5.1.3-1` (`enforced`) and `RFC4271-5.1.3-3` (`enforced`).
- **AC-12 BGP-LS & BGP-LS VPN framing + MUP preservation:** `ff7032975a`, `52e4ae832a`, `771ba0d96d`, `81f6bd905c`, `5ef6ed17f5` added `attribute.RegisterMPNextHopProfile` and `NextHopProfile` framing (`RDPerAddress`, `EncodeIPv4Mapped`, `DecodeUnprefixedIPv6`), registered LS71 (4/16/32) and LS72 (12/24/48 per-address zero-RD) in `plugins/nlri/ls/register.go`, proved MUP 2/85 mapped16 preservation (`TestMUPMappedNextHopPreserved`), added daemon (`bgpls-vpn-next-hop-framing.ci`) and real ExaBGP (`bgpls-vpn-exabgp-single.ci`) proofs, and updated `RFC7752-3.4-1` / `RFC9552-5.5-1` (`partial`, paired non-Ze peer acceptance honestly unclaimed).
- **AC-13 FlowSpec scope preservation:** `63afaf0c95` preserved FlowSpec / FlowSpecVPN family-cleared empty next-hop fields and permitted single redirect targets across common-subnet scope normalization without letting invalid received bytes restore cleared fields.
- **AC-14 listener ownership & interface-read fail-closed:** `9b6f3c8708` plus Commit A fixes in `reactor_connection.go`, `reactor_dynamic.go`, `transport_scope.go`, `transport_scope_read_failure_test.go`, and `docs/architecture/bgp/interface-event-reactions.md` enforced reactor-local `r.sameTransportAddress` fail-closed checks on static and dynamic admission, rejected invalid zero `netip.Addr{}` inputs in `sameTransportAddressUsing` and `listenerBinding.matches`, and deduplicated repeated address checks.
- **AC-15..AC-20 export-chain next-hop unification (D-0..D-6):** `6f79198a40`, `cabc8c511e`, `c2d7117d05` removed `session { next-hop }`, `PeerSettings.NextHopMode`/`NextHopAddress`, `applyNextHopMode`, and `plugin.PeerInfo` next-hop fields; widened `ze-filter-modify.yang` and `policychain.Sets.NextHop` to `self | unchanged | auto | <ipv4/ipv6>`; compiled zero-RPC built-in `next-hop:self|unchanged|auto` steps into `policychain.Chain`; added `appendDefaultExportFilters` (leaving `Policy.Configured()` false, refusing `inactive:next-hop:auto` and import-direction egress modes); implemented MED-style wire byte provenance for raw overrides and forwarded-only scope for trailing `next-hop:auto`; exposed the unified implied-filter seam for `| implied` and `show bgp peer`; and migrated all repository tests, interop configs, netlab templates/goldens, and docs off `session { next-hop }`.

### Bugs Found/Fixed
- **BGP-LS native IPv4 mapped-IPv6 defect:** `applyFactsNextHop` emitted 16-octet mapped IPv6 for usable IPv4 replacements on BGP-LS; fixed via `NextHopProfile` and covered by `next_hop_auto_bgpls_test.go` + LS71/72 tests.
- **Raw origination peer-own bypass:** recognized raw body/full-packet UPDATEs bypassed RFC 4271 §5.1.3 peer-own refusal; fixed in `session_aigp.go::writeOriginatedRawUpdate` and covered by `rfc4271_raw_originated_next_hop_test.go`, `raw-originated-nexthop-peer-own.ci`, and FRR `bgp-self-nexthop-withheld-frr`.
- **AC-14 Round 6 defects (Commit A):** `reactor_connection.go:76` and `reactor_dynamic.go:519` bypassed `r.readInterfaces` via package-level `sameTransportAddress`, and `transport_scope.go` (`sameTransportAddressUsing` / `listenerBinding.matches`) matched invalid zero `netip.Addr{}` values; fixed and covered by `TestScopedTransportReadFailureCannotGrantDedicatedOwnership` (`unreadable=true` dynamic refusal) and `TestInvalidAddressNeverMatchesTransportBinding`.

### Documentation Updates
- Commit `c2d7117d05` updated all 13 departed design pages (`ai/doors/bgp-engine.md`, `docs/architecture/core-design.md`, `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/wire/attributes.md`, `docs/architecture/wire/capabilities.md`, `docs/architecture/api/commands.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/update-syntax.md`, `docs/plugin-development/protocol.md`, `docs/architecture/update-building.md`, `docs/architecture/config/syntax.md`, `docs/guide/bgp-policy.md`, `docs/guide/route-policy.md`) plus `docs/architecture/bgp/egress-attribute-rules.md`, `docs/features/bgp-protocol.md`, `docs/guide/configuration.md`, `docs/guide/plugins.md`, `docs/guide/netlab.md`, `docs/guide/quickstart.md`, `docs/guide/as112.md`, `docs/guide/flowspec-route-reflector.md`, `docs/guide/healthcheck.md`, `docs/functional-tests.md`, `docs/architecture/testing/interop.md`, and `contrib/netlab/ze/ze.j2` + golden configs.
- Commit `5ef6ed17f5` updated `rfc/short/rfc4271.md`, `rfc/short/rfc7752.md`, `rfc/short/rfc9552.md`, and regenerated `docs/features/rfc-status.md`.
- Commit A updates `docs/architecture/bgp/interface-event-reactions.md` (lines 89-93, hunks only: ownership fail-closed wording matching `transport_scope.go`), repoints `docs/architecture/core-design.md` lines 4266-4272 `## Design decisions` `Source` cells from this closing spec path to landed decision commits `a7f7eb14d2`, `390086adb8`, and `ee8d115eb6` (hunks only), and adds operator news fragment `website/changes/fragments/bgp-next-hop-auto-rewrites-for-ebgp.md`.
- `./bin/le-nh-unify/le doc check all` and `./bin/le-nh-unify/le doc consistency` ran with zero errors on any file owned by this spec (only unrelated concurrent-session drift: `docs/DESIGN.md` missing `fib-dummy`, and pre-existing VPP/interoplab JSON tag warnings).

### Deviations from Plan
- **D-0..D-6 mid-spec unification (owner-approved 2026-11-23/24):** Thomas ruled to unify all egress next-hop rewriting into the export filter chain and remove `session { next-hop }` inside this spec before closure (`a7f7eb14d2`, `390086adb8`, `ee8d115eb6`, approved digest `9e2823b94235`).
- **`docs/architecture/core-design.md` `## Design decisions` `Source` repoint:** `internal/le/doc/consistency/decisions.go::sourceProblem` refuses bare stems and requires a committed SHA, existing file, or `ai/META-DECISIONS.md` section, so lines 4266-4272 repoint to landed decision commits `a7f7eb14d2`, `390086adb8`, and `ee8d115eb6` (matching closed-spec precedent `6619eb83e0`), requiring Thomas's terminal `decision-change` on Commit A.
- **AC-14 Round 6 owner authorization:** Thomas explicitly authorized Option (a) on 2026-11-25 to run the 6th independent review round on `9b6f3c8708`, fix the two product defects found, and close AC-14 inside this spec.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | Existing Self encoding was treated as complete for every implemented MP family | Absent BGP-LS length contract caused `applyFactsNextHop` to emit mapped IPv6 for IPv4; SAFI 72 also lacked per-address zero-RD framing | Shared reactor propagation reds and source trace | Resolved in AC-12 via `attribute.RegisterMPNextHopProfile` and LS71/72 profiles |
| assumption | Admission-fixture repair assumed Session ASN4 width alone sufficed for every original announcement | `newCommitFixture` used identity-free `EncodingContextForASN4` causing AS 0 prepend on eBGP | Focused unit run `job-unit-pkg-8a3d09f3.log` | Resolved in AC-10 with approved identity-bearing fixture context |
| approach | AC-14 listener ownership left package-level `sameTransportAddress` at two reactor call sites (`reactor_connection.go:76`, `reactor_dynamic.go:519`) and allowed zero `netip.Addr{}` equality in `sameTransportAddressUsing`/`listenerBinding.matches` | Both call sites bypassed `r.readInterfaces`, and two zero `netip.Addr{}` values compared equal | Round 6 independent review (`c44c93e1`) | Fixed in Commit A and proved via `TestScopedTransportReadFailureCannotGrantDedicatedOwnership` (`unreadable=true`) and `TestInvalidAddressNeverMatchesTransportBinding` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Default eBGP `auto` rewrites to self; iBGP and RS clients preserve received next hop | Done | `internal/component/bgp/config/peers.go::appendDefaultExportFilters`, `internal/component/bgp/reactor/peer_forward_facts.go::precomputeNextHop`, `internal/component/bgp/reactor/forward_rs.go` | Verified by unit, `.ci`, and FRR interop |
| Remove `session { next-hop }` and unify into export filter chain (D-0..D-6) | Done | `internal/component/bgp/yang/ze-bgp-conf.yang`, `internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang`, `internal/component/bgp/policychain/`, `internal/component/bgp/config/`, `internal/component/bgp/reactor/`, `internal/component/bgp/plugins/cmd/peer/peer.go` | Landed in `6f79198a40`, `cabc8c511e`, `c2d7117d05` |
| Remove dead `applyNextHopMod` and fix RFC 7947 Section 2.2.1 cite | Done | `internal/component/bgp/reactor/reactor_api_forward.go`, `internal/component/bgp/reactor/forward_rs.go`, `rfc/short/rfc8277.md` | Verified absent by grep |
| AC-11 raw & scoped peer-own NEXT_HOP refusal + FRR proof + RFC4271 rejudgment | Done | `internal/component/bgp/reactor/session_aigp.go::writeOriginatedRawUpdate`, `internal/component/bgp/reactor/forward_next_hop.go::nextHopValue.has`, `internal/le/interoplab/bgp/check_raw_originated_next_hop.go`, `rfc/short/rfc4271.md` | Live FRR acceptance + redwalk PASS; `RFC4271-5.1.3-1` and `-3` enforced (`5ef6ed17f5`) |
| AC-12 BGP-LS native IPv4 + LS-VPN 12/24/48 RD framing + MUP + ExaBGP proof | Done | `internal/core/bgp/attribute/mpnexthop.go::RegisterMPNextHopProfile`, `internal/component/bgp/plugins/nlri/ls/register.go`, `internal/component/bgp/plugins/nlri/mup/next_hop_mapped_test.go`, `test/plugin/bgpls-vpn-next-hop-framing.ci`, `test/plugin/bgpls-vpn-exabgp-single.ci` | `81f6bd905c`, `5ef6ed17f5`; `RFC7752-3.4-1` and `RFC9552-5.5-1` partial |
| AC-13 FlowSpec scope preservation | Done | `internal/component/bgp/reactor/forward_next_hop.go` | Landed in `63afaf0c95` |
| AC-14 listener ownership fail-closed on interface-read errors & zero-address guard | Done | `internal/component/bgp/reactor/transport_scope.go::sameTransportAddressUsing`, `listenerBinding.matches`, `reactor_connection.go::handleConnectionWithContext`, `reactor_dynamic.go::tryCreateDynamicPeerAt` | `9b6f3c8708` + Commit A Round 6 fixes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestNextHopAutoMatchesSelfForEBGP`, `test/plugin/nexthop-auto-ebgp-rewrite.ci`, `bgp-nexthop-auto-ebgp-frr` | IPv4 single-hop eBGP rewrite to self |
| AC-2 | Done | `TestNextHopAutoIPv6PairFollowsConnectedSubnet`, `TestNextHopAutoMatchesSelfForEBGP` | IPv6 global + conditional RFC 2545 link-local pair |
| AC-3 | Done | `TestNextHopAutoMatchesSelfForEBGP` | Multihop eBGP global-only rewrite |
| AC-4 | Done | `TestNextHopAutoWithoutOwnAddressPreservesMixedWithdrawals`, `TestNextHopAutoMissingOwnAddressWarnsForThePeer` | Withheld announcement, preserved withdrawals, peer warning |
| AC-5 | Done | `TestNextHopAutoPreservesRSClientAndIBGP`, `bgp-nexthop-auto-ebgp-frr` | iBGP preserves received next hop |
| AC-6 | Done | `TestNextHopAutoPreservesRSClientAndIBGP`, `test/plugin/nexthop-auto-rs-client-unchanged.ci` | RS client preserves received next hop on both rails |
| AC-7 | Done | `TestNextHopAutoInternalMultihopUsesOnlyUsableNextHop` | Multihop iBGP withholds unusable link-local-only; preserves global/Self |
| AC-8 | Done | `TestNextHopAutoChangePreservesExplicitModes`, `test/plugin/nexthop-self.ci` | `unchanged`, `self`, explicit modes preserved under export chain |
| AC-9 | Done | `draft_ietf_idr_linklocal_capability_single_next_hop_test.go` (6 units) | 16-octet bare Link-Local withheld multihop; permitted on-link cap 77 |
| AC-10 | Done | 30 admission callers (`job-unit-pkg-2da61af7.log`), 10 tagged callers (`job-unit-pkg-f3909007.log`), 6 Auto callers (`job-unit-pkg-bf94097f.log`) | ASN4/identity fixture repairs green |
| AC-11 | Done | `rfc4271_raw_originated_next_hop_test.go`, `rfc4271_scoped_next_hop_test.go`, `raw_originated_error_test.go`, `test/plugin/raw-originated-nexthop-peer-own.ci`, FRR `bgp-self-nexthop-withheld-frr` + redwalk | `RFC4271-5.1.3-1` enforced (`5ef6ed17f5`) |
| AC-12 | Done | `next_hop_auto_bgpls_test.go`, `plugins/nlri/ls/next_hop_wire_test.go`, `TestMUPMappedNextHopPreserved`, `bgpls-vpn-next-hop-framing.ci`, `bgpls-vpn-exabgp-single.ci` | LS71 4/16/32, LS72 12/24/48, ExaBGP singles PASS |
| AC-13 | Done | `63afaf0c95` FlowSpec scope tests in `internal/component/bgp/reactor/` | Empty & permitted redirect targets preserved |
| AC-14 | Done | `TestScopedTransportReadFailureCannotGrantDedicatedOwnership`, `TestInvalidAddressNeverMatchesTransportBinding` (PASS 0.592s) | `9b6f3c8708` + Commit A Round 6 fixes CLEAN |
| AC-15 | Done | `test/parse/session-nexthop-removed.ci`, `TestDynamicGroupRejectsRemovedSessionNextHop` | `session { next-hop }` removed across YANG, PeerSettings, PeerInfo, CLI |
| AC-16 | Done | `policychain/nexthop_intent_test.go`, `filter_modify/config_test.go`, `test/plugin/export-chain-nexthop-precedence.ci`, `test/parse/session-nexthop-removed.ci` | Built-in `next-hop:*`, IPv4+IPv6 `modify`, import refusal |
| AC-17 | Done | `TestAppendDefaultExportNextHopAuto`, `test/parse/session-nexthop-removed.ci` | Trailing `next-hop:auto`, `Policy.Configured()` false, `inactive:next-hop:auto` refused |
| AC-18 | Done | `reactor/export_nexthop_chain_test.go`, `test/plugin/export-chain-nexthop-precedence.ci` | Last-write-wins, fill-if-unset trailing `auto`, MED-style raw override provenance |
| AC-19 | Done | `reactor/export_nexthop_chain_test.go`, `test/plugin/originated-nexthop-peer-own.ci`, `test/plugin/raw-originated-nexthop-peer-own.ci` | Forwarded-only trailing `auto`; explicit export applies uniformly |
| AC-20 | Done | `TestImpliedFilterSeamIncludesEngineAndPluginInsertions`, `TestBuiltInNextHopDoctorAndCompletion`, `c2d7117d05` repo migration | Unified implied-filter seam; zero remaining `session { next-hop }` configs |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Unit tests (AC-1..AC-10, AC-11, AC-12, AC-13, AC-14, AC-15..AC-20) | Done | `internal/component/bgp/{reactor,config,policychain,plugins/filter_modify,plugins/nlri/mup}/*_test.go` | All passing in scoped main-thread runs |
| Functional `.ci` tests | Done | `test/plugin/{nexthop-auto-ebgp-rewrite,nexthop-auto-rs-client-unchanged,raw-originated-nexthop-peer-own,originated-nexthop-peer-own,export-chain-nexthop-precedence,bgpls-vpn-next-hop-framing,bgpls-vpn-exabgp-single,nexthop-self}.ci`, `test/parse/session-nexthop-removed.ci` | All passing (`./bin/le-nh-unify/le test bgp`) |
| Interop scenarios | Done | `test/interop/scenarios/bgp-nexthop-auto-ebgp-frr/`, `test/interop/scenarios/bgp-self-nexthop-withheld-frr/`, `test/plugin/bgpls-vpn-exabgp-single.ci` | Live FRR 10.7.1 acceptance + redwalk PASS; real ExaBGP PASS |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| All 21 Files to Create + `test/plugin/nexthop-self.ci` | Done | Verified present via `ls -la` in Pre-Commit Verification |
| All Files to Modify (YANG, config, policy, policychain, filter_modify, reactor, plugin, cmd/peer, interoplab, netlab, tests, docs, RFCs) | Done | Landed across `63afaf0c95`, `9b6f3c8708`, `6f79198a40`, `81f6bd905c`, `cabc8c511e`, `c2d7117d05`, `5ef6ed17f5`, and Commit A |

### Audit Summary
- **Total items:** 20 ACs, 21 created files, 13 departed design pages, 3 interop/ExaBGP suites
- **Done:** All 20 ACs, all 21 created files, all 13 departed pages, all interop/ExaBGP suites
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (recorded in Deviations from Plan)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Default eBGP `auto` rewrites forwarded next hop to Ze's session address while iBGP and RS clients preserve received next hops | Functional `.ci` + Live FRR Interop + Redwalk | `test/plugin/nexthop-auto-ebgp-rewrite.ci` (PASS, redwalk clean PASS/broken FAIL), `test/plugin/nexthop-auto-rs-client-unchanged.ci` (PASS, redwalk clean PASS/broken FAIL), FRR `bgp-nexthop-auto-ebgp-frr` (PASS + semantic redwalk PASS) |
| Unify all egress next-hop rewriting into the export filter chain, remove `session { next-hop }`, and surface implied members (D-0..D-6) | Unit + Functional `.ci` | `TestAppendDefaultExportNextHopAuto`, `TestImpliedFilterSeamIncludesEngineAndPluginInsertions`, `TestBuiltInNextHopDoctorAndCompletion`, `TestDynamicGroupRejectsRemovedSessionNextHop` (PASS); `test/parse/session-nexthop-removed.ci` (PASS 1/1); `test/plugin/export-chain-nexthop-precedence.ci` (PASS) |
| Raw and scoped locally originated UPDATEs enforce RFC 4271 §5.1.3 peer-own NEXT_HOP refusal | Unit + Functional `.ci` + Live FRR 10.7.1 Interop + Redwalk | `TestRawOriginatedAnnouncementDecoded`, `TestFRRDecodedAnnouncementForPeerOwnRelay` (PASS); `test/plugin/raw-originated-nexthop-peer-own.ci` & `originated-nexthop-peer-own.ci` (PASS 3/3); FRR `bgp-self-nexthop-withheld-frr` PASS (2m37s) + redwalk PASS (clean 2m6s, broken FAIL 2m59s assertion 7); `RFC4271-5.1.3-1` enforced (`5ef6ed17f5`) |
| BGP-LS uses native 4/16/32 next hops, LS-VPN uses 12/24/48 per-address zero-RD framing, and MUP retains mapped16 | Unit + Functional `.ci` + Real ExaBGP Interop | `TestNextHopAutoBGPLSNativeIPv4`, `TestLSVPNNextHopTypedWire`, `TestMUPMappedNextHopPreserved` (PASS); `test/plugin/bgpls-vpn-next-hop-framing.ci` (PASS); `test/plugin/bgpls-vpn-exabgp-single.ci` (PASS) |
| FlowSpec scope processing preserves empty and permitted redirect targets (AC-13) | Unit | `63afaf0c95` reactor FlowSpec scope tests (PASS) |
| Shared scoped listeners fail closed on interface-table read errors and invalid zero addresses (AC-14) | Unit | `go test -count=1 -v -run "TestScopedTransport\|TestListener\|TestInvalidAddressNeverMatchesTransportBinding" ./internal/component/bgp/reactor/` -> PASS (0.592s) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | All AC-1..AC-20, interop proofs, RFC updates, and AC-14 Round 6 fixes are complete inside this spec | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-next-hop-auto-rewrites-for-ebgp-8af317a1-5223-4974-8a3a-a381f5ad64aa.md` (recorded from Thomas's terminal via owed `le spec review record` with `rounds 6` and `owner-authorised`) |
| `./le spec review check` | Owed after Thomas records the terminal-only `rounds 6` artifact |
| Rounds | 6 — Rounds 1-5 covered AC-1..AC-13, export-chain unification (`/ze-review` `104794df`, `/ze-review-spec` `85daefd4`, `ze-test-judge` `8384730a` CLEAN), and AC-14 (`9b6f3c8708`); Round 6 (authorized by Thomas on 2026-11-25, `c44c93e1`) found two AC-14 product defects (`reactor_connection.go:76` and `reactor_dynamic.go:519` bypassed `r.readInterfaces`; `transport_scope.go` matched invalid zero `netip.Addr{}`), fixed in Commit A and re-verified CLEAN (0 BLOCKER, 0 ISSUE, 0 NOTE) |
| Reviewer lenses used | logic+wiring, security+edge-cases, Ze Go style pass (7 questions, 0 peer-reachable panics), design-documents lens (all 13 departures corrected in `c2d7117d05`), spec completeness (`/ze-review-spec`), test-weakening (`le test weakened check`: 0 findings on owned files) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | `handleConnectionWithContext` and `tryCreateDynamicPeerAt` called package-level `sameTransportAddress` instead of `r.sameTransportAddress`, bypassing `r.readInterfaces` and failing open/ignoring test reader injection | `internal/component/bgp/reactor/reactor_connection.go:76`, `internal/component/bgp/reactor/reactor_dynamic.go:519` | Switched both call sites to `r.sameTransportAddress(...) != transportAddressSame`; added `unreadable=true` refusal assertion in `transport_scope_read_failure_test.go` (Commit A) |
| 2 | ISSUE | `sameTransportAddressUsing` and `listenerBinding.matches` allowed invalid zero `netip.Addr{}` values to match each other (`a == b`) and redundantly re-checked equal `declaration`/`bound` addresses | `internal/component/bgp/reactor/transport_scope.go:48-55,319-358` | Guarded `!a.IsValid()`, `!b.IsValid()`, `!candidate.IsValid()` returning `transportAddressDifferent`, deduplicated equal fields, and added `TestInvalidAddressNeverMatchesTransportBinding` (Commit A) |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/next_hop_auto_bgpls_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 6271 Nov 24 02:16` |
| `internal/component/bgp/reactor/rfc4271_scoped_next_hop_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 10552 Nov 24 02:08` |
| `internal/component/bgp/reactor/rfc4271_raw_originated_next_hop_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 17063 Nov 20 18:00` |
| `internal/component/bgp/reactor/raw_originated_error_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 10686 Nov 20 18:00` |
| `internal/component/bgp/reactor/register_originated_next_hop.go` | Yes | `-rw-r--r-- 1 thomas thomas 1386 Nov 20 18:00` |
| `internal/test/fixture/register_raw_originated_next_hop.go` | Yes | `-rw-r--r-- 1 thomas thomas 3706 Nov 20 18:00` |
| `test/plugin/raw-originated-nexthop-peer-own.ci` | Yes | `-rw-r--r-- 1 thomas thomas 2190 Nov 24 19:37` |
| `internal/le/interoplab/bgp/check_raw_originated_next_hop.go` | Yes | `-rw-r--r-- 1 thomas thomas 6100 Nov 20 18:00` |
| `internal/le/interoplab/bgp/check_raw_originated_next_hop_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 2273 Nov 20 18:00` |
| `test/plugin/nexthop-auto-ebgp-rewrite.ci` | Yes | `-rw-r--r-- 1 thomas thomas 3316 Nov 24 20:00` |
| `test/plugin/nexthop-auto-rs-client-unchanged.ci` | Yes | `-rw-r--r-- 1 thomas thomas 3753 Nov 25 01:20` |
| `test/interop/scenarios/bgp-nexthop-auto-ebgp-frr/` | Yes | `frr.conf`, `frr-sink.conf`, `gobgp.toml`, `inject-args`, `inject.msg`, `ze.conf` (Nov 25 03:45) |
| `internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_single_next_hop_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 10158 Nov 24 02:03` |
| `internal/component/bgp/config/export_nexthop_default_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 24030 Nov 25 11:28` |
| `internal/component/bgp/policychain/nexthop_intent_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 30716 Nov 24 10:26` |
| `internal/component/bgp/reactor/export_nexthop_chain_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 31200 Nov 24 12:44` |
| `internal/component/bgp/plugins/nlri/mup/next_hop_mapped_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 14676 Nov 24 02:31` |
| `test/plugin/export-chain-nexthop-precedence.ci` | Yes | `-rw-r--r-- 1 thomas thomas 4583 Nov 25 11:32` |
| `test/parse/session-nexthop-removed.ci` | Yes | `-rw-r--r-- 1 thomas thomas 3899 Nov 24 10:25` |
| `test/plugin/bgpls-vpn-next-hop-framing.ci` | Yes | `-rw-r--r-- 1 thomas thomas 6936 Nov 25 11:32` |
| `test/plugin/bgpls-vpn-exabgp-single.ci` | Yes | `-rw-r--r-- 1 thomas thomas 12057 Nov 25 11:32` |
| `website/changes/fragments/bgp-next-hop-auto-rewrites-for-ebgp.md` | Yes | Created in Commit A (front matter `spec: bgp-next-hop-auto-rewrites-for-ebgp`, `topic: BGP`, 6 required headings) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-3 | eBGP single-hop/multihop IPv4/IPv6 `auto` rewrites to self (conditional RFC 2545 link-local) | `./bin/le-nh-unify/le test bgp` `plugin/nexthop-auto-ebgp-rewrite` PASS (4/4 batch, 2.7s); `bgp-nexthop-auto-ebgp-frr` + redwalk PASS |
| AC-4 | Missing local address withholds announcement, preserves withdrawals, warns peer | `next_hop_auto_relay_test.go::TestNextHopAutoWithoutOwnAddressPreservesMixedWithdrawals`, `next_hop_auto_boundaries_test.go::TestNextHopAutoMissingOwnAddressWarnsForThePeer` PASS |
| AC-5..AC-7 | iBGP & RS clients preserve received next hop; multihop iBGP withholds unusable link-local-only | `./bin/le-nh-unify/le test bgp` `plugin/nexthop-auto-rs-client-unchanged` PASS (4/4 batch, 2.7s); `TestNextHopAutoInternalMultihopUsesOnlyUsableNextHop` PASS |
| AC-8..AC-10 | Explicit modes preserved; 16-octet bare Link-Local refused multihop; ASN4/identity fixtures repaired | `test/plugin/nexthop-self.ci`, `draft_ietf_idr_linklocal_capability_single_next_hop_test.go`, `cabc8c511e` 93/93 fresh Go RFC records |
| AC-11 | Raw & scoped peer-own NEXT_HOP refusal on unit, daemon, and live FRR 10.7.1 | `go test -count=1 -run "TestRawOriginatedAnnouncementDecoded\|TestFRRDecodedAnnouncementForPeerOwnRelay" ./internal/le/interoplab/bgp/` PASS; `plugin/raw-originated-nexthop-peer-own` + `plugin/originated-nexthop-peer-own` PASS (3/3, 3.9s); FRR `bgp-self-nexthop-withheld-frr` PASS (2m37s) + redwalk PASS (broken FAIL assertion 7); `RFC4271-5.1.3-1` enforced (`5ef6ed17f5`) |
| AC-12 | BGP-LS native IPv4 4/16/32, LS-VPN 12/24/48 RD framing, MUP mapped16, real ExaBGP singles | `plugin/bgpls-vpn-next-hop-framing` PASS (4/4 batch, 2.7s); `plugin/bgpls-vpn-exabgp-single` PASS (3/3 batch, 3.9s); `81f6bd905c` + `5ef6ed17f5` |
| AC-13 | FlowSpec scope preserves empty and permitted single redirect target | `63afaf0c95` landed & verified |
| AC-14 | Scoped listener ownership fails closed on unreadable interfaces and rejects zero `netip.Addr{}` | `go test -count=1 -v -run "TestScopedTransport\|TestListener\|TestInvalidAddressNeverMatchesTransportBinding" ./internal/component/bgp/reactor/` PASS (0.592s); `./bin/le-nh-unify/le go style check` OK on all 4 reactor files |
| AC-15..AC-20 | Export-chain next-hop unification, `session { next-hop }` removal, raw override provenance, originated preservation, implied-filter seam | `go test -count=1 -run "TestAppendDefaultExportNextHopAuto\|TestImpliedFilterSeamIncludesEngineAndPluginInsertions\|TestBuiltInNextHopDoctorAndCompletion\|TestDynamicGroupRejectsRemovedSessionNextHop\|TestImplied" ./internal/component/bgp/config/` PASS; `parse/session-nexthop-removed` PASS (1/1, 2.9s); `plugin/export-chain-nexthop-precedence` PASS (4/4 batch, 2.7s) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Absent export `next-hop:*` (trailing `next-hop:auto`) relaying to eBGP peer | `test/plugin/nexthop-auto-ebgp-rewrite.ci` | Yes — read lines 1-103: sends `NEXT_HOP 192.0.2.9` from `source` (`65001`) and asserts exact `NEXT_HOP 127.0.0.4` (`7F000004`) at eBGP `destination` (`65002`) with no explicit next-hop config; PASS |
| Trailing `next-hop:auto` relaying to RS client vs ordinary eBGP control | `test/plugin/nexthop-auto-rs-client-unchanged.ci` | Yes — read lines 1-120: asserts byte-identical `NEXT_HOP 192.0.2.9` at `rs-client true` `client` and rewritten `127.0.0.6` (`7F000006`) at ordinary eBGP `ordinary`; PASS |
| Conditional `policy { modify bh }` beating trailing `next-hop:auto` on eBGP | `test/plugin/export-chain-nexthop-precedence.ci` | Yes — read lines 1-134: Route 1 (`65000:666`) carries `NEXT_HOP 192.0.2.66` (`C0000242`) while unmatched Route 2 falls through to trailing `next-hop:auto` and carries `127.0.0.4` (`7F000004`); PASS |
| Peer-wide `filter { export { filters [ next-hop:self ]; } }` on iBGP | `test/plugin/nexthop-self.ci` | Yes — read lines 1-87: configures `filter { export { filters [ next-hop:self ]; final accept; } }` on iBGP `peer1` and verifies peer detail + RIB via `le test fixture plugin/nexthop-self`; PASS |
| Removed `session { next-hop }`, `inactive:next-hop:auto`, and import egress-mode refusal | `test/parse/session-nexthop-removed.ci` | Yes — read lines 1-150: validates `next-hop:self|unchanged|auto` + IPv6 `modify` on export/import while refusing `session { next-hop self; }`, `inactive:next-hop:auto`, `import { next-hop:self }`, and `import { modify:nh-self }`; PASS (1/1, 2.9s) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `ze-bgp-conf.yang` had `default "auto"`, now unified as trailing `next-hop:auto` appended by `appendDefaultExportFilters` |
| A-2 | confirmed | `TestNextHopAutoPreservesRSClientAndIBGP` + `test/plugin/nexthop-auto-rs-client-unchanged.ci` PASS; RS clients keep received next hop on both rails |
| A-3 | confirmed | `TestNextHopAutoInternalMultihopUsesOnlyUsableNextHop` proves multihop iBGP withholds unusable link-local-only while preserving usable Global and explicit Self |
| A-4 | confirmed | `TestNextHopAutoMatchesSelfForEBGP` confirms connected local endpoint then `LocalAddress` across both relays |
| A-5 | confirmed | `buildForwardFacts` uses `PeerSettings.IsEBGP` (`!IsIBGP`), consistent across reactor |
| A-6 | broken | Broken: BGP-LS emitted mapped IPv6 and LS-VPN lacked RD framing; resolved in AC-12 via `attribute.RegisterMPNextHopProfile` and LS71/72 profiles (Mistake Log row 1) |
| A-7 | confirmed | Pinned Wireshark v4.4.0 LS72 decoder (12/24/48 per-address zero-RD) matches `plugins/nlri/ls/register.go`; paired non-Ze peer acceptance honestly unclaimed (`RFC9552-5.5-1` partial) |
| A-8 | confirmed | Real pinned ExaBGP exercised in `test/plugin/bgpls-vpn-exabgp-single.ci` (PASS 3/3 batch) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Feature list & user guides (Checklist #1, #2, #3, #5, #6, #17) | `c2d7117d05` updated `docs/features/bgp-protocol.md`, `docs/guide/{configuration,bgp-policy,route-policy,plugins,netlab,quickstart,as112,flowspec-route-reflector,healthcheck}.md`, `contrib/netlab/ze/ze.j2` + golden configs | Yes |
| API/RPC, Plugin SDK, wire & architecture (Checklist #4, #7, #8, #12, #16) | `c2d7117d05` corrected all 13 departed design pages (verified via `git log --grep="Spec-work: bgp-next-hop-auto-rewrites-for-ebgp"` — all 13 present); Commit A updates `docs/architecture/bgp/interface-event-reactions.md` (hunk lines 89-93) and repoints `docs/architecture/core-design.md` (hunk lines 4266-4272) `Source` to `a7f7eb14d2`/`390086adb8`/`ee8d115eb6` | Yes |
| RFC compliance (Checklist #9) | `5ef6ed17f5` updated `rfc/short/rfc4271.md` (`RFC4271-5.1.3-1`, `-3` enforced), `rfc/short/rfc7752.md`, `rfc/short/rfc9552.md` (`partial`), and regenerated `docs/features/rfc-status.md`; `5956b5b518` resealed 63 shifted and re-judged 39 stale audit verdicts across 20 stems (`./bin/le-nh-unify/le rfc check`: 0 remaining violations on any file touched by this spec) | Yes |
| Test infrastructure & comparison (Checklist #10, #11, #13, #14, #15) | `c2d7117d05` updated `docs/functional-tests.md` and `docs/architecture/testing/interop.md`; `docs/comparison.md`, route metadata, metrics, and runtime inventories required no change (`le doc check all` & `le doc consistency`: 0 errors on owned files) | Yes |
| Spec citation check (post-closure citers) | `git grep -n "plan/release/fixes/spec-bgp-next-hop-auto-rewrites-for-ebgp.md"` returns zero hits outside this spec itself; `./bin/le-nh-unify/le spec citation` reports zero errors for this spec | Yes |

## Core Insight

Egress next-hop rewriting belongs on a single ordered surface — the export filter chain with a fill-if-unset trailing `next-hop:auto` — so conditional policy rules and the RFC 4271 Section 5.1.3 default compose deterministically without a second session-level writer silently overwriting policy decisions, while post-policy scope and peer-own admission remain non-negotiable engine gates.
