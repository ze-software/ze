# Spec: bgp-unowned-rfc-rows -- the BGP RFC defects no spec owned

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The BGP child of the rfc-verdict pass (`plan/pre-release/spec-rfc-verdict-fix-bgp.md`,
"Gap accounting", around line 975) closed with eleven requirement rows that are
canonically `unimplemented` and that no spec owned. Owner decision, 2026-10-11:
"10 give it a spec". This spec took them and two `partial` RFC 9514 rows; the
owner then answered its open questions and split it: "Split into three"
(2026-10-11). This spec keeps the defects and the one ledger exclusion, under its
original file name. The two features went to `plan/`:

| Rows | Owner spec |
|------|------------|
| 6, 7 (`RFC4271-8.2.2-13`, `RFC4271-8.2.2-20`): DelayOpen, Event 3 | `plan/spec-bgp-fsm-delayopen-and-automatic-start.md` |
| 8, 11, 12 (`RFC9086-7-2` with 7-1's gap half, `RFC9514-7.2-5`, `RFC9514-7.2-6`): EPE PeerAdj/PeerSet, SRv6 PeerNode | `plan/spec-bgp-epe-peer-segments-and-srv6-peernode.md` |

`RFC9252-5-2` (SRv6 Service SID resolvability before best path) already had an
owner the child did not name: `plan/immediate/spec-srv6-bestpath-resolvability.md`
(its lines 27, 63 and 453-454 carry the row and its tags). It stays there.

Row numbers below are the numbers the rows had before the split, kept so the
quote and comparison rows stay traceable across the three specs.

| # | Row id | Document, section | Class | What Ze does today |
|---|--------|-------------------|-------|--------------------|
| 1 | `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-4` | draft-ietf-bess-mup-safi §3.3.6 | defect | Stores a received Direct Segment Discovery (DSD) route without comparing its MP_REACH next hop to the originator of the Prefix-SID locator |
| 2 | `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-5` | draft-ietf-bess-mup-safi §3.3.6 | defect | Stores a received DSD route that carries no Prefix-SID attribute |
| 3 | `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-1-2` | draft-ietf-idr-linklocal-capability §1 | defect | Accepts a Link-Local next hop on a capability-77 session and publishes it to the RIB, sysrib and FIB as a bare `fe80::/10` address with no interface; the sent `session link-local` address is not tied to the session's interface |
| 4 | `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1` | draft-ietf-idr-linklocal-capability §6 | defect | On a capability-77 session, admits a 32-octet Next Hop field without checking that it is a Global followed by a Link-Local |
| 5 | `RFC2545-3-6` | RFC 2545 §3 | defect (fix needs a source Ze lacks) | Sends a third-party global next hop on a common subnet in 16 octets, without the Link-Local of that next hop |
| 9 | `RFC9830-2.4.2-12` | RFC 9830 §2.4.2 | gap: feature absent, owner OUT | Never signals an SRv6 BSID in the type-13 Binding SID sub-TLV (uses the RECOMMENDED type 20) and decodes no type 13 on receipt |
| 10 | `RFC7606-5.1-1` | RFC 7606 §5.1 | defect | Emits MP_REACH_NLRI at its type-code position, after ORIGIN and AS_PATH, in every UPDATE builder, the splitter, and a forward that adds an attribute |

The class column follows `ai/rules/rfc-compliance.md`: "Behavior that violates an
applicable RFC requirement in a capability Ze implements is a defect; a capability Ze
does not provide is an implementation gap". The six defects are fixed here. The
type-13 SRv6 BSID (row 9) is OUT (owner, 2026-10-11): its AC is a ledger correction
(`{feature-declined}` with the RFC's own sentence), never counted conformant.

### Bucket

`plan/immediate/`. Every code row is a wire or routing correctness defect an
operator on the first release meets: a MUP DSD route that should be withdrawn is
installed (rows 1, 2), a Link-Local route is installed with no outgoing interface or
a malformed next hop is accepted (rows 3, 4), a third-party next hop loses its
Link-Local (row 5), and every MP UPDATE Ze sends violates an RFC 7606 SHALL that
FRR and BIRD both meet (row 10). `plan/README.md` puts wire correctness and routing
correctness in `immediate/`. Row 9 changes the ledger only and rides with the
defects because it needs no code.

### The RFC text, quoted from the source

| # | Quote | Read at |
|---|-------|---------|
| 1 | "However, the receiving BGP speaker MUST ensure that the received nexthop value in the MP_REACH_NLRI attribute is identical to the originator of the locator value in the prefix SID attribute. The originator of the locator value can be resolved from the IPv6 IGP table. If the result of the match is not identical then the receiving BGP speaker MUST consider it as a malformed NLRI and the "Treat-as-withdraw procedure of [RFC7606] is applied." | `rfc/drafts/draft-ietf-bess-mup-safi.txt` §3.3.6 |
| 2 | "When a BGP speaker receives a MP_REACH_NLRI attribute update message with a Direct Segment Discovery route without a prefix SID attribute, than it MUST be treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is applied." | same, §3.3.6 |
| 1, 2 (skip) | "A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message." (follows both DSD sentences) | same, §3.3.6 |
| 3 | "Since IPv6 Link-Local addresses are not required to be globally unique, implementations must ensure that they are strictly associated with a specific interface." | `rfc/drafts/draft-ietf-idr-linklocal-capability.txt` §1 |
| 4 | "If the Next Hop field is malformed, the implementation MUST handle the malformed UPDATE message using the approach of "treat-as-withdraw", as described in section 7.3 of [RFC7606]." Section 6 opens: "These procedures apply only when the Link-Local Next Hop Capability has been negotiated for a BGP session." | same, §6 |
| 5 | "The link-local address shall be included in the Next Hop field if and only if the BGP speaker shares a common subnet with the entity identified by the global IPv6 address carried in the Network Address of Next Hop field and the peer the route is being advertised to." | `rfc/full/rfc2545.txt` §3 |
| 9 | "The value MUST be 18 when a SRv6 BSID is present, 6 when an SR-MPLS BSID is present, or 2 when no BSID is present." | `rfc/full/rfc9830.txt` §2.4.2 |
| 9 (full) | "It is RECOMMENDED that the SRv6 Binding SID sub-TLV, as defined in Section 2.4.3, be used when signaling an SRv6 BSID for an SR Policy CP. The support for the use of this Binding SID sub-TLV for the signaling of an SRv6 BSID is retained primarily for backward compatibility with implementations that followed early draft versions of this document that had not defined the SRv6 Binding SID sub-TLV." | `rfc/full/rfc9830.txt` §2.4.2 (lines 514-522) |
| 10 | "The MP_REACH_NLRI or MP_UNREACH_NLRI attribute (if present) SHALL be encoded as the very first path attribute in an UPDATE message." | `rfc/full/rfc7606.txt` §5.1 |

### How FRR and BIRD handle it

| # | FRR (`/Users/thomas/Code/github.com/FRRouting/frr`) | BIRD (`/Users/thomas/Code/gitlab.nic.cz/labs/bird`) |
|---|------|------|
| 1, 2 | No MUP SAFI in `bgpd/` | No MUP SAFI in `proto/bgp/` |
| 3 | `bgpd/bgp_attr.c` (MP_REACH next hop parse, around line 2644): a Link-Local global next hop takes `peer->nexthop.ifp->ifindex` as `nh_ifindex`; with no peer interface the route is withdrawn (`BGP_ATTR_PARSE_WITHDRAW`) | `proto/bgp/packets.c` around line 1113: the Link-Local is resolved with `neigh_find(..., p->neigh->iface, ...)`, the session's interface; not directly reachable is a withdraw |
| 4 | Same function, around line 2687: a 32-octet field whose second address is not Link-Local is logged at debug and the Link-Local half dropped | `proto/bgp/packets.c` around line 1432: a second address that is not Link-Local is dropped and the field treated as 16 octets |
| 5 | `bgpd/bgp_updgrp_packet.c` (checkout `cd39d029`, lines 466-560): a third-party global is kept (`bgp_multiaccess_check_v6`); a 32-octet field keeps the Link-Local it received, and a `::` Link-Local is replaced by FRR's OWN `peer->nexthop.v6_local`. No neighbour-table lookup: FRR never derives the third party's Link-Local (A-5) | `bgp_update_next_hop_ip` (packets.c around line 1280): a third-party gateway is sent in 16 octets, no Link-Local. BIRD has the same omission |
| 9 | SR Policy lives in `pathd`, not compared | None |
| 10 | `bgp_packet_attribute` (`bgpd/bgp_attr.c` line 4950) writes MP_REACH (`bgp_packet_mpattr_start`, line 4976) before ORIGIN (line 4988) | `bgp_create_mp_reach` (packets.c line 2895) writes MP_REACH first, then copies the other attributes after it |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/mp-nlri-ordering.md` - the page that states Ze's MP attribute order
  → Constraint: this page states the current order (MP_UNREACH first, everything else by type code, MP_REACH included) and argues it; row 10's fix makes the page wrong, so the page edit lands in the same change as the builder edit
  → Decision: the page's "three builders disagreed" history is the reason the fix goes into `attribute.OrderAttributes` and every builder at once, never one builder
- [ ] `docs/architecture/testing/interop.md` - scenario naming and the revert-and-red walk
  → Constraint: every new scenario is named (no numeric prefix) and owes a recorded red with the fix reverted and the artifact rebuilt

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/draft-ietf-bess-mup-safi.md` - rows 3.3.6-4 (line 422), 3.3.6-5 (line 423)
  → Constraint: both `{gap}` annotations name `internal/component/bgp/plugins/nlri/mup/rfc7606.go::RecognizeNLRI` as reading the architecture and route type alone
- [ ] `rfc/short/draft-ietf-idr-linklocal-capability.md` - rows 1-2 (line 109), 6-1 (line 129) and the `Support coverage` gap sentence
  → Constraint: 6-1's annotation separates a wrong Next Hop length (session reset under RFC 7606 §7.11, already met by `validateMPReachNextHop`) from a wrong address shape inside a 32-octet field (this row); the fix must not turn the first into the second
- [ ] `rfc/short/rfc2545.md` - row 3-6 (line 207) and the `Support remaining` partial text
  → Constraint: the gap covers only rails where Ze writes the next hop; a received complete 32-octet pair relayed unchanged is not in it (main-thread ruling R53(b), `plan/handover/rfc-verdict-test-fix-pass/RULINGS.md`)
- [ ] `rfc/short/rfc9830.md` - row 2.4.2-12 (line 312)
  → Decision: owner OUT. The ledger form is `{feature-declined: "<quote>"; <reason naming internal/component/bgp/plugins/nlri/srpolicy/config.go::buildSRv6BindingSIDSubTLV>}`. It satisfies both facts `checkFeatureDeclined` checks (`docs/contributing/rfc-conformance-gates.md`, "The feature-declined annotation"): the opening double-quoted span is the RECOMMENDED sentence plus the "retained primarily for backward compatibility" sentence, verbatim from `rfc/full/rfc9830.txt` §2.4.2, and the producer is a non-test function in `config.go` that `gopls symbols` lists. The row carries no `RFC requirement:` tag anywhere in `internal/` or `test/` (grep, 2026-10-11), so the R50 refusal of `{feature-declined}` beside existing tags does not arise. The obligation is conditional ("when a SRv6 BSID is present" in type 13) and the RFC recommends the other form, which are the kind's rules 1 and 2
  → Constraint: the declined feature stays disclosed on the summary's `Support status` and `Support remaining` rows as an implementation gap. `rfc/extraction/rfc9830.json` lists the id under section 2.4.2 `unsourced-ids`; whether the extraction register also records it as `feature-out-of-scope` is read from the gate at implementation, and `./le rfc check` exiting clean for the stem is the proof the form is accepted
- [ ] `rfc/short/rfc7606.md` - row 5.1-1 (line 338), and the `docs/features/rfc-status.md` RFC 7606 row that discloses it publicly
  → Constraint: the public disclosure is a published claim; when the fix lands, the row and the generated status page change in the same commit

**Key insights:**
- Seven rows: six defects and one owner-declined form. RFC9252-5-2 is owned elsewhere; rows 6-8 and 11-12 moved to the two `plan/` specs named in the Task.
- Row 10 touches every MP UPDATE Ze sends; it is the widest change and the only one with no design freedom.
- Rows 3 and 4 share the receive next-hop path; rows 3 and 5 share the "Link-Local needs an interface or a neighbor" question.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/nlri/mup/rfc7606.go` - `RecognizeNLRI` (line 53) and `MUPRouteType.Implemented` (line 31) decide per-NLRI recognition from architecture and route type only; DSD is an implemented type
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - `mpNextHopAddr` (line 1491) takes the first 16 octets of a 32-octet field and parses it with no zone; a 16-octet Link-Local becomes a bare address. `isSRv6Ineligible` (line 1318) reads Prefix-SID type 40 from the stored other-attributes
- [ ] `internal/component/sysrib/sysrib.go` - `nextHopInterface` (line 149) is filled only from a producer that names one; the BGP best-change event names none
- [ ] `internal/plugins/fib/kernel/mplsentry_linux.go` - `resolveMPLSNextHop` (line 152) refuses a Link-Local next hop with no zone ("link-local next hop requires an interface zone"); the MPLS path already knows the need
- [ ] `internal/core/bgp/attribute/origin.go` - `OrderAttributes` (line 220) and `WriteAttributesOrdered` (line 269); the comment above line 220 records why MP_REACH moved to its type-code position and names the other two builders (`reactor/peer_rib_routes.go::buildRIBRouteUpdate`, `reactor/reactor_api_batch.go::buildWireModeUpdate`)
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/config.go` - `buildBindingSIDSubTLV` / `buildBindingSIDNullSubTLV` emit type 13 with 6 or 2 octets only; `buildSRv6BindingSIDSubTLV` emits the RECOMMENDED type-20 form for every configured SRv6 BSID; no file in `internal/component/bgp` decodes type 13
- [ ] `internal/component/bgp/reactor/link_scope.go` - `linkLocalNextHop`, `sameLinkLayerSegment`; the send-side Link-Local selection rows 3 and 5 extend. `linkScope` holds connected PREFIXES only (`network.ConnectedPrefixes`, built on `net.InterfaceAddrs`), no interface name
- [ ] `internal/core/bgp/nlri/nlritype/nlritype.go`, `internal/component/bgp/reactor/session_validation_nlritype.go` - `Recognizer` is `func(nlriBytes []byte, addPath bool) bool` and its contract is "answers about the TYPE only"; `(*Session).typedNLRIEdit` holds `pathAttrs` and splits kept from dropped NLRIs (`nlritype.Retain`), and the dropped ones are DISCARDED (RFC 7606 §5.4), not withdrawn
- [ ] `internal/component/bgp/reactor/session_validation_receive.go` - an EBGP Prefix-SID is discarded under RFC 8669 §4 unless the session accepts it, before the typed-NLRI pass
- [ ] `internal/component/iface/dispatch.go` - `RouteLookup` (outgoing interface for an address) and `ListNeighbors` (kernel neighbour table: address, MAC, device, state) through the active backend; `internal/component/mtu/cmd/run.go::routeInterfaceOf` and `internal/component/bgp/plugins/healthcheck` already consume the iface component
- [ ] `internal/component/bgp/message/update_build.go::packAttributesOrderedInto`, `internal/component/bgp/message/update_split.go::lowRunLen` / `emitMPChunk`, `internal/component/bgp/reactor/forward_build.go` (merge-insert at type-code position), `internal/perf/sender.go` - every writer that places MP_REACH at its type-code position besides `OrderAttributes`
- [ ] `internal/component/bgp/reactor/forward_body.go`, `internal/component/bgp/reactor/forward_aigp.go::forwardUpdateSelected` - a received UPDATE mixing NLRI-bearing fields is split per destination before it goes out (`MixesNLRIFields`); a single-field UPDATE that gains no attribute is forwarded byte-identical

**Behavior to preserve:**
- A wrong Next Hop LENGTH stays a session reset (`validateMPReachNextHop`, RFC 7606 §7.11).
- MP_UNREACH_NLRI-only UPDATEs keep their current byte layout apart from position rules row 10 imposes.
- A received complete Global + Link-Local pair relayed unchanged.
- The RECOMMENDED type-20 SRv6 BSID encoding (row 9 changes no code).

**Behavior to change:** rows 1-5 and 10, as the ACs state; row 9 changes the ledger only (owner: OUT).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Receive: UPDATE wire bytes on a peer session (rows 1-4).
- Send: route origination, reflection or relay through the UPDATE builders (rows 5, 10).
- Config: `session link-local` refusal at commit (row 3 send side). Row 9 has no config change (OUT).

### Transformation Path
1. Wire parse and RFC 7606 classification in `internal/component/bgp/message/` (per-NLRI recognition calls into the family plugin's `RecognizeNLRI`).
2. Admission to the RIB plugin, best path in `rib_bestchange.go`, best-change event to sysrib.
3. sysrib selection, FIB programming in `internal/plugins/fib/kernel/` or `fib/vpp/`.
4. Egress: `reactor/link_scope.go` next-hop scope, attribute assembly, `attribute.OrderAttributes` / `WriteAttributesOrdered`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to RIB plugin | received UPDATE with per-NLRI treat-as-withdraw already decided | No |
| RIB plugin to sysrib | best-change event; today carries no interface for a next hop | No |
| sysrib to FIB | route with `nextHopInterface` | No |

### Integration Points
- A sibling registry beside `nlritype` per family (MUP) - rows 1 and 2 need the UPDATE's Prefix-SID attribute, which the recognizer does not see (A-1).
- Session interface knowledge in the reactor (`link_scope.go`) - rows 3 and 5.
- `OrderAttributes` and every writer A-3 lists - row 10.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be shown at implementation |
| No unintended coupling (components stay isolated) | No | row 1's IGP lookup must go through sysrib, never import an IGP plugin |
| No duplicated functionality (extends existing, does not recreate) | No | row 10 changes the one ordering function and the writers that reimplement it |
| Zero-copy preserved where applicable (refs, not copies) | No | row 10 reorders in place in the build buffer; no extra copy |
| Registration over hardcoding, outbound | No | MUP checks live in the MUP plugin's registered check, not in the engine |
| Registration over hardcoding, inbound | No | to be shown at implementation |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The MUP per-NLRI recognizer can be given the UPDATE's Prefix-SID attribute without a central switch | `RecognizeNLRI` signature takes NLRI bytes only | Rows 1-2 need an engine-side hook that learns about MUP, which `ai/rules/principles.md` refuses; a registered per-family attribute check is the alternative | Read the engine call site of `RecognizeNLRI` | broken (2026-10-11): the recognizer cannot carry it; a sibling registry does |
| A-2 | The session's interface is known in the reactor for a directly connected IPv6 peer | `link_scope.go::sameLinkLayerSegment` answers segment questions from connected prefixes | Row 3 needs an interface source; without it the route must be withdrawn, the FRR behavior | `gopls` on `link_scope.go`, read the connected-prefix source | broken (2026-10-11): no interface name is held anywhere in the reactor |
| A-3 | Ze never emits MP_REACH and MP_UNREACH in one UPDATE | RFC 7606 §5.1 second bullet forbids it | If it does, row 10's "very first" has two candidates and a second RFC 7606 §5.1 defect exists, which gets a journal row | grep the builders, a unit test over a mixed batch | holds by source reading (2026-10-11); the mixed-batch unit test in AC-7 is the proof owed |
| A-4 | GoBGP implements the MUP SAFI and can send a DSD route with and without a Prefix-SID | GoBGP release notes (unchecked here) | Rows 1-2 interop falls back to a ze-peer raw-hex functional test, and the interop gap is disclosed | Build the GoBGP container, send a DSD route | half holds (2026-10-11): with a Prefix-SID only |
| A-5 | FRR's behavior for row 5 was not read in this pass | budget | The FRR comparison for row 5 is missing from the design | Read `bgpd/bgp_updgrp_packet.c` next-hop rewrite before design approval | settled (2026-10-11): read, recorded in the FRR/BIRD table |
| A-6 | About 118 `.ci` files carry MP attribute hex (`800e`/`900e`) | `grep -rli` over `test/` | Row 10's expectation rewrite is larger or smaller than planned | rerun the count at implementation | holds (2026-10-11) as an upper bound, with more surfaces than `.ci` |

- A-1 → Decision: `nlritype.Recognizer` is `func(nlriBytes []byte, addPath bool) bool` and its package contract says it "answers about the TYPE only", so giving it attributes would break its contract for every family. The engine call site, `(*Session).typedNLRIEdit`, already holds `pathAttrs` and already carves each NLRI. Rows 1-2 add a SIBLING registry beside `nlritype` (per family, registered from the MUP plugin's `init()` next to its `nlritype.Register` call): a check that takes one carved NLRI plus the UPDATE's path attributes and MP next hop and answers keep or treat-as-withdraw. The engine names no family; a family with no check is unaffected
- A-1 → Constraint: the draft says "A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message" (§3.3.6), so the verdict is per NLRI, and it is WITHDRAW, not the §5.4 discard `typedNLRIEdit` does today: a DSD route stored earlier under the same key must leave the Adj-RIB-In. The rejected NLRIs therefore move into an MP_UNREACH_NLRI section of the rewritten UPDATE (the `rewriteMPNLRISections` edit path), never into the dropped count
- A-1 → Constraint: the RFC 8669 §4 EBGP Prefix-SID discard (`session_validation_receive.go`) runs first. A DSD route from an EBGP peer not configured to accept the Prefix-SID therefore arrives at the MUP check with no Prefix-SID and is treated as withdrawn. AC-2 states it, and a functional test covers it
- A-1 → Constraint: the ISD rows `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1/-3/-4` carry the same `{gap}` and the same mechanism would serve them; they are not this spec's rows (`plan/pre-release/spec-rfc-verdict-fix-bgp.md` lists them). The check is written so an ISD rule is a second registration, not a second mechanism
- A-2 → Decision: `linkScope` holds prefixes only and `network.InterfacePrefixes` reads `net.InterfaceAddrs`, which returns no interface names; sessions run over global addresses (`reactor_peers.go` refuses a Link-Local `local-address`). The link-scope snapshot gains the NAME of the interface whose connected prefix contains the peer, read at the same cadence as today (session establishment and reload, never per UPDATE, from `net.Interfaces` with each interface's addresses). A multihop peer, or one no single interface reaches, has no interface, and its Link-Local next hops are treated as withdrawn with a Warn (the FRR behavior `bgp_attr.c` shows)
- A-2 → Constraint: the RIB plugin, not the reactor, publishes the best change to sysrib, so the interface crosses the engine-to-plugin boundary with the received route (an event field), and sysrib already accepts it (`protocolRoute.nextHopInterface` from the candidate's `Interface`). No new sysrib field is needed
- A-3 → Decision: holds. Each send path emits at most one MP attribute per UPDATE: the withdrawal writers (`forward_build.go::writeWithdrawal`, `reactor_wire.go`, `reactor_api_relay.go`) emit MP_UNREACH alone; `update_split.go::splitUpdateWithMP` emits MP_UNREACH chunks and MP_REACH chunks as separate UPDATEs; the forward loop splits any received UPDATE that `MixesNLRIFields` reports before it goes out (`forward_body.go`, both rails). So "very first" has one candidate in every UPDATE Ze sends, and no second §5.1 defect or journal row arises. `OrderAttributes` still accepts a list holding both, so AC-7's unit test feeds it one
- A-3 → Constraint: row 10 reaches more than the "three builders" the original design named: `OrderAttributes` and `WriteAttributesOrdered`; `packAttributesOrderedInto`, which the VPN, EVPN, labeled, flowspec, grouped and plugin builders call (their comments restate the old order and change with it); `update_split.go::lowRunLen` and `emitMPChunk`; the forward merge-insert in `forward_build.go`; `peer_rib_routes.go::buildRIBRouteUpdate`; `reactor_api_batch.go::buildWireModeUpdate`; `internal/perf/sender.go`. A pure forward stays byte-identical today, so a received UPDATE whose MP attribute is NOT first (an older speaker, which §5.1 tells receivers to accept) would go out still violating §5.1: the forward path moves the MP attribute first for that source only, and the zero-copy identity survives for every source that already complies (FRR and BIRD both do)
- A-4 → Decision: GoBGP implements the MUP SAFI from v3.6.0, and the interop image pins v3.31.0 (`test/interop/Dockerfile.gobgp`). Its CLI DSD form (`cmd/gobgp/global.go::parseMUPDirectSegmentDiscoveryRouteArgs`) REQUIRES `prefix`, `locator-node-length`, `function-length` and `behavior` and always attaches an SRv6 L3 Service Prefix-SID, so GoBGP can send the stored case and a next hop of the operator's choosing, but never a DSD route without a Prefix-SID. The interop scenario proves the positive case and the keep-and-warn case (owner decision on row 1) against GoBGP; the without-Prefix-SID case (AC-2) and the proven-mismatch case (AC-1) are proven by ze-peer functional tests, and the interop scenario's README says so
- A-5 → Decision: FRR (`bgpd/bgp_updgrp_packet.c`) keeps a third-party global, keeps a received Link-Local in a 32-octet field, and substitutes its OWN Link-Local for `::`; it never derives the third party's Link-Local. BIRD sends 16 octets. Neither does what the owner chose for row 5 ("Fix it": read the third party's Link-Local from the Linux neighbour table), so Ze has no reference implementation to copy and the AC is written from RFC 2545 §3 alone
- A-5 → Constraint: the kernel neighbour table maps an address to a MAC and a device, not a global address to a Link-Local. The lookup is: the third-party global's entry on the shared interface gives its MAC; a `fe80::/10` entry on the same device with the same MAC is that router's Link-Local. No entry, a stale or failed entry, or two candidate Link-Locals for one MAC means no Link-Local is known: the field goes out as 16 octets, as today, with one rate-limited Warn naming the next hop and the reason. The read goes through `iface.ListNeighbors` (the iface component, already consumed by the BGP healthcheck plugin), is cached per distinct third-party next hop, and is refreshed with the link-scope snapshot, never per UPDATE. A received complete pair relayed unchanged keeps its Link-Local (R53(b)); the lookup applies only where Ze writes the next hop
- A-6 → Decision: holds as an upper bound. 118 `.ci` files under `test/` match `800e|900e` (42 `test/plugin`, 22 `test/encode`, 21 `test/exabgp-compat/encoding`, 13 `test/decode`, 11 `test/exabgp-compat/api`, 7 `test/draft/plugin`); decode files hold RECEIVED bytes and mostly stay. Two further surfaces were not in the estimate: 49 Go `_test.go` files under `internal/` carry MP attribute hex or bytes, and interop `inject.msg` files carry it (received bytes, unaffected)
- A-6 → Constraint: `test/exabgp-compat/encoding` expectations are ExaBGP's byte order (the VPN builder's comment says so). `ai/rules/rfc-compliance.md` ranks the RFC above ExaBGP compatibility, so those expectations change with row 10 and the commit body says why

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Row 10 changes every MP UPDATE byte string: exact-hex `.ci`, golden files and interop captures go red together | the first `./le test functional plugin` run after the change | Regenerate expectations mechanically from the new builder, review a sample by hand, and land code and expectations in one commit |
| R-2 | Row 10 desynchronises the builders again | two builders emitting different bytes for one route | One test feeds the same route through every builder and compares bytes (exists for type-code order; extend it) |
| R-3 | Row 1 needs an IPv6 IGP lookup; with no IGP route for the locator Ze would withdraw every DSD route | DSD routes vanish in a lab without IS-IS | Owner decision on row 1 ("Keep it, log a warning"): an unresolvable originator keeps the route and logs a Warn; only a proven mismatch withdraws |
| R-4 | Row 3 withdraws Link-Local routes Ze used to install (badly) | operators see fewer routes after upgrade | Log the withdraw at Warn with the reason and the fix; the route was not forwardable before |
| R-5 | Row 4 withdraws routes from a peer sending two Link-Locals, a form the draft says is "observed in deployment" (BIRD) | interop with BIRD loses routes | The draft says receivers SHOULD use the second address for that form, so it is not malformed; the AC excludes it explicitly |
| R-6 | Row 9's `{feature-declined}` is refused by `./le rfc check` (the R50-R53 history) | gate exit 2 | The row has no tags, so the R50 refusal does not apply; a refusal names its cause, which the ledger phase fixes in the row text (quote span or producer symbol), never by dropping the annotation back to `{gap}` silently |

R-7 and R-8 moved with the EPE rows to `plan/spec-bgp-epe-peer-segments-and-srv6-peernode.md`.

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Row 10: every MP UPDATE Ze sends. Rows 1-4: routes withdrawn that should have been kept. Row 5: a third-party next hop sent with a wrong Link-Local. Row 9: the RFC 9830 ledger only |
| How is it reverted? | Each row lands as its own commit, revertible alone |
| Who else touches this path? | `plan/immediate/spec-srv6-bestpath-resolvability.md` (same RIB candidate filter as row 1's Prefix-SID read), `plan/immediate/spec-bgp-prefix-sid-rfc-defects.md` (Prefix-SID parsing), `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (ledger rows). Row 10 changes the byte order of every BGP-LS UPDATE `plan/spec-bgp-epe-peer-segments-and-srv6-peernode.md` originates: whichever lands second writes its expectations against the other |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE with DSD route, no Prefix-SID, from a peer | → | MUP check treat-as-withdraw | `test/plugin/mup-dsd-without-prefix-sid-withdraw.ci` |
| UPDATE with DSD route whose next hop differs from the locator originator | → | MUP check, sysrib lookup | `test/plugin/mup-dsd-nexthop-locator-mismatch-withdraw.ci` |
| UPDATE with 16-octet Link-Local next hop on a capability-77 session | → | RIB best change carries the session interface to sysrib and FIB | `test/plugin/linklocal-capability-nexthop-interface.ci` |
| UPDATE with a malformed 32-octet next hop on a capability-77 session | → | treat-as-withdraw | `test/plugin/linklocal-capability-malformed-nexthop-withdraw.ci` |
| Static route with a third-party global next hop on the common subnet | → | `link_scope.go` third-party Link-Local | `test/plugin/rfc2545-third-party-linklocal.ci` |
| Any MP announcement from config | → | `OrderAttributes` and every builder | `test/plugin/rfc7606-mp-reach-first.ci` |
| DSD route whose locator originator the IGP table cannot resolve | → | MUP check keeps the route and logs a Warn (owner decision, row 1) | `test/plugin/mup-dsd-locator-unresolved-kept.ci` |
| DSD route from an EBGP peer not accepting the Prefix-SID | → | RFC 8669 §4 discard, then MUP treat-as-withdraw | `test/plugin/mup-dsd-ebgp-prefix-sid-discarded-withdraw.ci` |
| Row 9 | → | none: OUT, ledger only | the row's `{feature-declined}` and `./le rfc check` clean for `rfc9830` |

## Acceptance Criteria

AC numbers changed in the split: rows 9 and 10 were AC-9 and AC-10 before it and
are AC-6 and AC-7 here; the ledger AC was AC-11 over eight stems and is AC-8 here
over this spec's five. AC-6, AC-7, AC-8, AC-12 and AC-13 of the unsplit spec went to
the FSM and EPE specs.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 (row 1) | A DSD route arrives whose MP_REACH next hop is not the originator of the Prefix-SID locator, as the IPv6 IGP table resolves it | The NLRI is treated as withdrawn (removed from the Adj-RIB-In if stored, carried as an MP_UNREACH NLRI, never merely discarded), the other NLRIs of the UPDATE are processed, and a Warn log names the peer, the route, the next hop and the resolved originator. A matching next hop is stored (positive polarity). When the originator cannot be resolved from the IGP table (no IGP, or no route for the locator), the route is KEPT and a rate-limited Warn names the peer, the route, the locator and "originator not resolvable from the IGP table" (owner: "Keep it, log a warning"): only a proven mismatch withdraws |
| AC-2 (row 2) | A DSD route arrives in an UPDATE with no Prefix-SID attribute, including one whose Prefix-SID the RFC 8669 §4 EBGP discard removed | The NLRI is treated as withdrawn as in AC-1 and the other NLRIs of the UPDATE are processed; the same route with a Prefix-SID is stored |
| AC-3 (row 3) | A Link-Local next hop (16 octets, or the Link-Local half of 32) arrives on a session over a known interface | The installed kernel route names that interface as its device; on a session with no known interface the route is treated as withdrawn with a Warn log naming the reason (FRR behavior). On send, the advertised own Link-Local is an address of the session's interface, and a configured `session link-local` that is not on that interface is refused at commit with an error naming the leaf, the interface and the fix |
| AC-4 (row 4) | On a capability-77 session, a 32-octet Next Hop field that is not a Global (or `::`) followed by a Link-Local, and not the two-Link-Local form §6 tells receivers to accept | The UPDATE is handled as treat-as-withdraw (RFC 7606 §7.3); a well-formed pair and the two-Link-Local form are accepted; a wrong length still resets the session |
| AC-5 (row 5) | Ze writes a next hop whose global address is another router on the subnet Ze shares with the recipient (owner: "Fix it") | The Next Hop field is 32 octets and its second address is that router's Link-Local, read from the Linux neighbour table on the send path (A-5: same device, same MAC as the global's entry). With no usable neighbour entry the field is 16 octets, as today, and a rate-limited Warn names the next hop and the reason. A received complete pair relayed unchanged keeps its Link-Local. The neighbour read is cached per distinct next hop and never runs per UPDATE |
| AC-6 (row 9) | The type-13 SRv6 BSID form (owner: OUT) | No code changes. `rfc/short/rfc9830.md` row `RFC9830-2.4.2-12` changes from `{gap}` to `{feature-declined}` with the RFC 9830 §2.4.2 RECOMMENDED and backward-compatibility sentences as its quote and `config.go::buildSRv6BindingSIDSubTLV` as its producer; the summary's `Support status` / `Support remaining` name the type-13 SRv6 form as an implementation gap; the row is not counted conformant; `./le rfc check` exits clean for `rfc9830` |
| AC-7 (row 10) | Ze sends any UPDATE carrying MP_REACH_NLRI or MP_UNREACH_NLRI, from any writer A-3 lists: `OrderAttributes`, `packAttributesOrderedInto` and the family builders on it, `buildRIBRouteUpdate`, `buildWireModeUpdate`, the splitter, the forward merge-insert, and a forward of a received UPDATE whose MP attribute was not first | That attribute is the first path attribute on the wire; the same route through every builder yields identical bytes; `OrderAttributes` given a list holding both MP attributes still puts one first; a forward of a source that already complies stays byte-identical; the `.ci` and Go test expectations, `docs/architecture/wire/mp-nlri-ordering.md` and the public RFC 7606 status row change in the same commit |
| AC-8 (ledger) | Each row once its AC is met | The row in `rfc/short/` loses `{gap}` (row 9: takes `{feature-declined}`), carries tags in both polarities where Ze can observe the behavior, and carries a discrimination record (`./le rfc discriminate-record`); `./le rfc check` exits clean for the five stems `draft-ietf-bess-mup-safi`, `draft-ietf-idr-linklocal-capability`, `rfc2545`, `rfc9830`, `rfc7606` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Peers with a MUP speaker that sends a malformed DSD route | wire -> MUP check -> treat-as-withdraw -> no RIB entry | `test/plugin/mup-dsd-without-prefix-sid-withdraw.ci` |
| 2 | Peers over an interface with capability 77 and receives Link-Local-only next hops | wire -> RIB -> sysrib -> kernel route with device | `bgp-linklocal-capability-interface-frr` |
| 3 | Sends IPv6 routes to FRR | config -> builder -> wire with MP_REACH first | `bgp-mp-reach-first-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMUPDSDWithoutPrefixSIDTreatAsWithdraw` | `internal/component/bgp/plugins/nlri/mup/rfc7606_test.go` | AC-2 both polarities | |
| `TestMUPDSDNextHopLocatorMismatch` | same | AC-1 both polarities, and the unresolvable case | |
| `TestLinkLocalNextHopCarriesSessionInterface` | `internal/component/bgp/plugins/rib/` | AC-3 receive | |
| `TestLinkLocalSessionAddressMustBeOnInterface` | `internal/component/bgp/plugins/llnh/` | AC-3 send-side refusal | |
| `TestLinkLocalCapabilityMalformedThirtyTwoOctet` | `internal/component/bgp/message/` | AC-4, including the two-Link-Local accepted form and the wrong-length reset | |
| `TestRFC2545ThirdPartyLinkLocal` | `internal/component/bgp/reactor/link_scope_test.go` | AC-5: neighbour entry found (32 octets), absent or ambiguous (16 octets and Warn), relayed pair unchanged | |
| `TestOrderAttributesMPFirst` | `internal/core/bgp/attribute/origin_test.go` | AC-7, including a list holding both MP attributes | |
| `TestEveryBuilderSameBytesMPFirst` | `internal/component/bgp/reactor/` | AC-7, R-2, over every writer A-3 lists | |
| `TestSplitterEmitsMPFirst` | `internal/component/bgp/message/update_split_test.go` | AC-7 splitter | |
| `TestForwardMovesLateMPFirst` | `internal/component/bgp/reactor/` | AC-7: a source with MP late is moved; a compliant source stays byte-identical | |
| `TestMUPDSDUnresolvedOriginatorKept` | `internal/component/bgp/plugins/nlri/mup/rfc7606_test.go` | AC-1 keep-and-warn | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Next Hop length on a capability-77 session | 16 or 32 | 32 | 15 (session reset, unchanged) | 33 (session reset, unchanged) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mup-dsd-without-prefix-sid-withdraw` | `test/plugin/` | AC-2 | |
| `mup-dsd-nexthop-locator-mismatch-withdraw` | `test/plugin/` | AC-1 | |
| `linklocal-capability-nexthop-interface` | `test/plugin/` | AC-3 | |
| `linklocal-capability-malformed-nexthop-withdraw` | `test/plugin/` | AC-4 | |
| `rfc2545-third-party-linklocal` | `test/plugin/` | AC-5 | |
| `rfc7606-mp-reach-first` | `test/plugin/` | AC-7 | |
| `mup-dsd-locator-unresolved-kept` | `test/plugin/` | AC-1 keep-and-warn | |
| `mup-dsd-ebgp-prefix-sid-discarded-withdraw` | `test/plugin/` | AC-2 EBGP discard | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-mp-reach-first-frr` | `test/interop/scenarios/` | FRR | FRR receives Ze's IPv6 UPDATE with MP_REACH as the first attribute (capture asserted), routes accepted | |
| `bgp-mp-reach-first-bird` | same | BIRD | the same against BIRD | |
| `bgp-linklocal-capability-interface-frr` | same | FRR (capability 77, `PEER_HAS_LINK_LOCAL_CAPABILITY`) | FRR sends Link-Local-only next hops; Ze's kernel route names the session interface | |
| `bgp-mup-dsd-gobgp` | same | GoBGP v3.31.0 (MUP since v3.6.0) | GoBGP sends DSD routes, always with a Prefix-SID (A-4): Ze stores them, and with no IGP route for the locator keeps them and logs the Warn. The absent-Prefix-SID and proven-mismatch cases cannot be produced by GoBGP's CLI and are proven by the ze-peer functional tests; the scenario README says so | |
| `bgp-rfc2545-linklocal-nexthop-frr` (existing, extended) | same | FRR | Ze advertises a route whose global next hop is a third router on the shared segment; FRR's received next hop carries that router's Link-Local, and the FRR route resolves through it | |

Row 4's malformed form cannot be produced by a conforming peer; its proof is the ze-peer functional test. Row 9 is OUT and has no scenario. Every scenario owes the revert, rebuild, red, restore, green walk with the red recorded.

## Files to Modify
- `internal/component/bgp/plugins/nlri/mup/rfc7606.go`, `internal/component/bgp/plugins/nlri/mup/register.go` - rows 1, 2 (the MUP check and its registration)
- `internal/component/bgp/reactor/session_validation_nlritype.go` - rows 1, 2 (consult the sibling registry beside `nlritype`; rejected NLRIs become MP_UNREACH, A-1)
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - row 3 (next hop carries an interface)
- `internal/component/sysrib/sysrib.go` - row 3: the candidate `Interface` field exists (A-2) and the BGP producer fills it; the implementer records at file:line whether this file needed an edit
- `internal/component/bgp/message/rfc7606.go` - row 4
- `internal/component/bgp/plugins/llnh/llnh.go` - row 3 send-side refusal
- `internal/component/bgp/reactor/link_scope.go`, `internal/core/network/connected.go` - rows 3, 5 (interface name in the snapshot; third-party Link-Local via `iface.ListNeighbors`)
- `internal/core/bgp/attribute/origin.go`, `internal/component/bgp/message/update_build.go`, `update_build_vpn.go`, `update_build_evpn.go`, `update_build_labeled.go`, `update_build_plugin.go`, `update_split.go`, `internal/component/bgp/reactor/peer_rib_routes.go`, `reactor_api_batch.go`, `forward_build.go`, `internal/perf/sender.go` - row 10 (A-3)
- `docs/architecture/wire/mp-nlri-ordering.md` - row 10
- `rfc/short/draft-ietf-bess-mup-safi.md`, `rfc/short/draft-ietf-idr-linklocal-capability.md`, `rfc/short/rfc2545.md`, `rfc/short/rfc9830.md`, `rfc/short/rfc7606.md` - AC-8
- `test/plugin/*.ci`, `test/encode/*.ci`, `test/exabgp-compat/**/*.ci`, `test/draft/plugin/*.ci` and Go `_test.go` files carrying MP attribute hex - row 10 expectations (A-6)

## Files to Create
- the eight `test/plugin/*.ci` files and the interop scenarios named above
- the MUP check registry beside `internal/core/bgp/nlri/nlritype/` (A-1), placed by `ai/rules/architecture.md` at design review

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no new leaf; the YANG work moved with rows 6 and 8 to the two `plan/` specs |
| YANG validation constraints | N-A | none new |
| YANG custom validators | Yes | row 3 send side: `session link-local` must be on the session's interface, through the existing `refuseLinkLocalCapabilityWithoutAddress` path in `llnh.go` |
| CLI commands/flags | N-A | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | N-A | no new RPC; behavior `.ci` tests are listed above |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | row 5 needs a readable neighbour table; a `ze doctor` check names an unreadable one with the fix |
| Prometheus counters/metrics | No | treat-as-withdraw counting already exists in the RFC 7606 path; confirm at implementation |
| BGP family surface (new SAFI / capability / attribute) | N-A | no new family, capability or attribute; MUP and capability 77 exist |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/bgp-protocol.md` if its evidence names these rows (check `./le feature report`) |
| 2 | Config syntax changed? | No | the `session link-local` refusal adds no syntax; its guide page is checklist item 6 |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, checked against the MUP check and the llnh refusal this spec adds |
| 6 | Has a user guide page? | Yes | the IPv6 next-hop guide page that covers `session link-local`, found through `ai/INDEX.md` |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/mp-nlri-ordering.md` |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | the five `rfc/short/` files above and `docs/features/rfc-status.md` (generated by `./le rfc index-update`) |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it lists MP attribute order |
| 12 | Internal architecture changed? | Yes | `docs/architecture/wire/mp-nlri-ordering.md` (row 10); the RIB-to-sysrib next-hop interface (row 3) in `docs/architecture/plugin/rib-storage-design.md`. `docs/architecture/behavior/fsm.md` moved with rows 6-7 to the FSM spec |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared by the files this spec changes (`// Design:` headers): `docs/architecture/wire/messages.md` and `docs/architecture/wire/attributes.md` (row 10 attribute order: affected, update with the builder change); `docs/architecture/plugin/rib-storage-design.md` (row 3 next hop with interface: affected); `docs/architecture/config/syntax.md` (row 3 send-side refusal: the implementer reads the page and edits it where the refusal text belongs there); `docs/architecture/core-design.md` (unaffected: no change to the core/registration design). `docs/architecture/wire/nlri-bgpls.md` moved with rows 8, 11, 12 to the EPE spec. Re-derive at implementation with `./le spec citation anchors spec plan/immediate/spec-bgp-unowned-rfc-rows.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | check `session link-local` examples against the new refusal |

## Implementation Steps

Defects land first, each in its own commit, so no defect waits on another.

1. **Phase: Wiring** -- the eight wiring `.ci` tests written and red.
2. **Phase: row 10** -- `OrderAttributes` and every writer A-3 lists put MP_REACH and MP_UNREACH first; the forward moves a late MP attribute first for non-compliant sources only; expectations regenerated; page and RFC 7606 row edited. Own commit.
3. **Phase: rows 2 then 1** -- the MUP check registry (A-1) reads Prefix-SID presence, then the locator originator with keep-and-warn when unresolvable. Own commit.
4. **Phase: rows 3 and 4** -- the receive next hop carries the session interface through sysrib to the FIB; malformed 32-octet check; send-side refusal. Own commit.
5. **Phase: row 5** -- third-party Link-Local from the neighbour table, cached, with the doctor check. Own commit.
6. **Phase: row 9** -- the `{feature-declined}` ledger correction, no code. Own commit.
7. **Phase: ledger** -- AC-8 for every row, discrimination records, `./le rfc check` clean for the five stems.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation or an owner-recorded ledger form at file:line |
| Correctness | Row 4 does not reclassify a wrong length; the two-Link-Local form is accepted |
| Correctness | Row 10 holds for every builder, and for MP_UNREACH-only UPDATEs |
| Data flow | MUP checks stay in the MUP plugin; no MUP name in the engine |
| Rule: rfc-compliance | Each new implementing function quotes its RFC sentence; each caller names the section |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No `{gap}` left on this spec's rows | `grep -n` each id in its `rfc/short/` file |
| Both polarities tagged | `grep -rn "RFC requirement: <id>" internal/ test/` |
| Ledger clean | `./le rfc check` exit status for the five stems |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Rows 1, 2, 4 parse peer-supplied bytes; bounds checked before reading the Prefix-SID or the second address |
| Resource exhaustion | Row 1's IGP lookup per DSD route must not block the receive path; row 5's neighbour read is cached, never per UPDATE |
| Error leakage | Warn logs name the peer and route, rate-limited through the existing per-route limiter |

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

## Owner decisions (2026-10-11)

| Question | Owner's words | What it means here |
|----------|---------------|--------------------|
| Row 1: when the locator's originator cannot be resolved from the IPv6 IGP table, is the DSD route withdrawn or kept? | "Keep it, log a warning" | AC-1: an unresolvable originator keeps the route with a rate-limited Warn; only a proven mismatch withdraws (R-3) |
| Row 5: fix the third-party Link-Local, or record a deviation? | "Fix it" | AC-5: read the third party's Link-Local from the Linux neighbour table on the send path (A-5) |
| Row 9: support the type-13 SRv6 BSID form, or decline it? | OUT | AC-6: `{feature-declined}` quoting the RFC 9830 §2.4.2 RECOMMENDED sentence; disclosed as an implementation gap, never counted |
| DelayOpen (row 6), Event 3 (row 7), EPE PeerAdj/PeerSet (row 8) | IN | Implemented, in the two `plan/` specs below |
| RFC9514-7.2-5/-6: does this work own them? | "Only if EPE is in" | EPE is in, so they are owned, by the EPE spec |
| One spec or several? | "Split into three" | This spec keeps the defects and row 9; `plan/spec-bgp-fsm-delayopen-and-automatic-start.md` takes rows 6-7; `plan/spec-bgp-epe-peer-segments-and-srv6-peernode.md` takes rows 8, 11, 12 |

## Open Questions (owner)

None open.

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Defects stay in `immediate/`; DelayOpen/Event 3 and EPE move to two `plan/` specs | One owning spec for every row | Owner: "Split into three". The defects decide this bucket; the features pass the `plan/` test |
| RFC9252-5-2 left to `spec-srv6-bestpath-resolvability` | Take it here | It already has a design-status spec with ACs and tags planned; two owners is the defect the pass was cleaning |
| Rows 1-2 use a sibling per-family check registry beside `nlritype` | Widen `nlritype.Recognizer` to take attributes; an engine-side MUP hook | The recognizer's contract is "answers about the TYPE only" for every family; an engine hook names MUP centrally (A-1) |
| Row 3 withdraws when no interface is known | Keep the bare address | FRR and BIRD both withdraw; a zone-less `fe80::` route is not forwardable |
| The link-scope snapshot carries the interface name, read at establishment and reload | A per-UPDATE route lookup | The snapshot cadence already exists and no interface name is held today (A-2) |
| Row 5 matches the neighbour table by device and MAC | Copy FRR (own Link-Local) or BIRD (16 octets) | Owner: "Fix it"; neither reference derives the third party's Link-Local, and the kernel maps address to MAC, not global to Link-Local (A-5) |
| Row 10 moves the MP attribute first in a forward only for a non-compliant source | Rebuild every forwarded UPDATE | Keeps the zero-copy identity for every compliant source, FRR and BIRD included (A-3) |
| Row 9 takes `{feature-declined}` | `{gap}` left counted; implement type 13 | Owner: OUT; the RFC recommends type 20 and keeps type 13 for backward compatibility only |

## Known Limitations
- Row 9: the type-13 SRv6 BSID form is declined by the owner and stays disclosed as an implementation gap, never counted conformant.
- The ISD rows `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1/-3/-4` share row 1's mechanism but are owned by `plan/pre-release/spec-rfc-verdict-fix-bgp.md`.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code, using
the quotes in the Task section. Drafts are cited by name and section.

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
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
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
- [ ] **Commit B:** `remove plan/immediate/spec-bgp-unowned-rfc-rows.md` only, in the same `./le commit create` script
