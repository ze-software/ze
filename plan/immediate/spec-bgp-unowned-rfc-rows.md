# Spec: bgp-unowned-rfc-rows -- the BGP RFC defects no spec owned

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-12 |

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
- [ ] `docs/architecture/plugin/plugin-system.md` - "The process boundary"
  → Constraint: a plain exported Go function called across packages reaches process-local state and "does nothing useful when the plugin runs external ... There is no error, no panic and no log line". The `locatororigin` resolver is that class, so it is guarded by `.IsInternal()` and listed in `dangerousCalls` (Design resolutions, R2-I1)
- [ ] `pkg/plugin/rpc/bridge.go` - `DirectBridge`, the in-process transport of an internal plugin (`ai/rules/plugins.md` asks for it before new core-to-plugin plumbing)
  → Decision: not used for the originator lookup: it serves only an internal plugin, so it adds nothing the guarded registry lacks, and a request/response per DSD NLRI would put an RPC on the receive path (R2-I1)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/draft-ietf-bess-mup-safi.md` - rows 3.3.6-4 (line 422), 3.3.6-5 (line 423)
  → Constraint: both `{gap}` annotations name `internal/component/bgp/plugins/nlri/mup/rfc7606.go::RecognizeNLRI` as reading the architecture and route type alone
- [ ] `rfc/short/draft-ietf-idr-linklocal-capability.md` - rows 1-2 (line 109), 6-1 (line 129) and the `Support coverage` gap sentence
  → Constraint: 6-1's annotation separates a wrong Next Hop length (session reset under RFC 7606 §7.11, already met by `validateMPReachNextHop`) from a wrong address shape inside a 32-octet field (this row); the fix must not turn the first into the second
- [ ] `rfc/short/rfc2545.md` - row 3-6 (line 207) and the `Support remaining` partial text
  → Constraint: the gap covers only rails where Ze writes the next hop; a received complete 32-octet pair relayed unchanged is not in it (main-thread ruling R58(b), `plan/handover/rfc-verdict-test-fix-pass/RULINGS.md`; corrected 2026-10-11 from R53(b), which is RFC4271-6.3-2 logging)
- [ ] `rfc/short/rfc9830.md` - row 2.4.2-12 (line 312)
  → Decision: owner OUT. The ledger form is `{feature-declined: "<quote>"; <reason naming internal/component/bgp/plugins/nlri/srpolicy/config.go::buildSRv6BindingSIDSubTLV>}`. It satisfies both facts `checkFeatureDeclined` checks (`docs/contributing/rfc-conformance-gates.md`, "The feature-declined annotation"): the opening double-quoted span is the RECOMMENDED sentence plus the "retained primarily for backward compatibility" sentence, verbatim from `rfc/full/rfc9830.txt` §2.4.2, and the producer is a non-test function in `config.go` that `gopls symbols` lists. The row carries no `RFC requirement:` tag anywhere in `internal/` or `test/` (grep, 2026-10-11), so the R50 refusal of `{feature-declined}` beside existing tags does not arise. The obligation is conditional ("when a SRv6 BSID is present" in type 13) and the RFC recommends the other form, which are the kind's rules 1 and 2
  → Constraint: the declined feature stays disclosed on the summary's `Support status` and `Support remaining` rows as an implementation gap. `rfc/extraction/rfc9830.json` lists the id under section 2.4.2 `unsourced-ids`; whether the extraction register also records it as `feature-out-of-scope` is read from the gate at implementation, and `./le rfc check` exiting clean for the stem is the proof the form is accepted
- [ ] `rfc/short/rfc7606.md` - row 5.1-1 (line 338), and the `docs/features/rfc-status.md` RFC 7606 row that discloses it publicly
  → Constraint: the public disclosure is a published claim; when the fix lands, the row and the generated status page change in the same commit
- [ ] `rfc/full/rfc9352.txt` §7.1 (lines 453-462) and `rfc/full/rfc7794.txt` §2.1 (X-Flag lines 210-214, R-Flag lines 231-233) and §2.2 (IPv6 Source Router ID, type 12, lines 247-278)
  → Constraint: "The Prefix Attribute Flags sub-TLV MUST be included in the Locator TLV when it is leaked upwards in the hierarchy or originated as a result of the redistribution from another protocol or another IS-IS instance." (RFC 9352 §7.1). Only flags present with R and X clear prove an originator; a Source Router ID equal to the next hop is a match (Design resolutions, B1)
- [ ] `rfc/full/rfc9352.txt` §6 (lines 324-372), `rfc/full/rfc7794.txt` §2.1 X-Flag in TLVs 236 and 237 (lines 218-221), `rfc/full/rfc5308.txt` §2 external bit (lines 107-108)
  → Constraint: "If at least one of them sets the A-flag in its advertisement, the prefix/SRv6 Locator SHOULD be considered as anycast." and "In case of any inconsistency between the Prefix Attribute Flags advertised in the Locator TLV and in the Prefix Reachability TLV, the ones advertised in the Prefix Reachability TLV MUST be preferred." (RFC 9352 §6). The A-flag (bit 4) on any covering advertisement is the anycast not-known; a same-system TLV 236 entry for the prefix decides X by its external bit whether or not it carries a Prefix Attribute Flags sub-TLV, because the bit is in TLV 236's fixed header and RFC 9352 §6 says "When included in the Locator TLV, the state of the X-flag in the Prefix Attributes Flags sub-TLV MUST match the setting of the embedded "X-bit" in any advertisement for the same prefix in TLVs 236 [RFC5308] and 237 [RFC5120]." (lines 365-368), while RFC 7794 §2.1 says of the sub-TLV X-Flag "In TLVs 236 and 237, this flag SHOULD always be sent as 0 and MUST be ignored on receipt"; that sub-TLV, when present, decides R and A (Design resolutions, B1, round-4 I1, round-5 I1)

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

### Design resolutions from the independent review (2026-10-11)

| Finding | Design |
|---------|--------|
| B1, row 1 originator (revised by round-2 I2) | The originator of a locator exists only in the IS-IS LSDB. A new registry in `internal/core/` (`locatororigin`: `Register(name, Resolver)`, `Resolve(sid netip.Addr)` answering the originator's address set, whether it is known with the reason when it is not, and an error) is filled by the IS-IS plugin at `init()`. The DSD Prefix-SID carries a SID, which "consists of the PE locator followed by a function" (draft §3.3.4), not a locator prefix, so `Resolve` takes the SID address and longest-matches it against the SRv6 Locator TLVs (27) the LSDB holds; the locator length comes from the advertised TLV 27, never from the BGP side. The originator is a router and a router has many IPv6 addresses, so the answer is the SET the single originating LSP advertises: TLV 140 (IPv6 TE Router ID), every TLV 232 (IPv6 Interface Address) and every host /128 in TLV 236. Which address BGP compares follows the MP_REACH Next Hop length (round-3 I3): 16 octets, that address; 32 octets, the global first half (the Link-Local second half is never an address the LSP advertises); 4 octets, an IPv4 address, which is in no originator set because the set holds IPv6 addresses only and the draft requires one ("a PE MUST use the IPv6 address of the PE as the nexthop address in the MP_REACH_NLRI attribute", §3.3.4), so against a known originator it is a proven mismatch and against a not-known one the route is kept like any other; any other length is not-known, "next hop length N not comparable". A member of the set is stored, and only a next hop in none of them, against a known originator, is a proven mismatch and withdraws. Which system is the originator (round-3 B1): Ze holds one LSDB per level (`internal/plugins/isis/lsdb/lsdb.go`, "held separately for Level 1 and Level 2"), and in a multi-level domain the Level 2 copy of a Level 1 PE's locator is the leaking L1/L2 router's, whose addresses do not hold the PE's next hop. So the index spans both levels and keys advertisers by system ID: every fragment and both levels of one router are one originator, and its address set is the union of those LSPs' TLV 140, TLV 232 and host /128 TLV 236 addresses. The resolver walks the sub-TLVs of each covering TLV 27 (new code: `bgpls_export.go` stores TLV 27 raw today). An IPv6 Source Router ID sub-TLV (RFC 7794 §2.2, type 12) on a covering advertisement whose address equals the next hop is a known match, stored. Advertisements whose Prefix Attribute Flags sub-TLV (RFC 7794 §2.1, type 4) has R ("Set when the prefix has been leaked from one level to another (upwards or downwards)") or X ("Set if the prefix has been redistributed from another protocol") set are not the originator. An advertisement with no Prefix Attribute Flags sub-TLV proves no originator either: RFC 9352 §7.1 makes the sub-TLV a MUST only on a leaked or redistributed locator and says that without it "receivers will be unable to determine the correct source of the advertisement", so a receiver cannot tell it from a leaked one. Anycast and flag precedence (round-4 I1): RFC 9352 §6: "The same prefix/SRv6 Locator can be advertised by multiple routers. If at least one of them sets the A-flag in its advertisement, the prefix/SRv6 Locator SHOULD be considered as anycast." So the A-flag (bit 4) set on any covering advertisement, at either level and from any system, leaked or not, is the anycast not-known whatever the R/X-clear count, because two PEs sharing an anycast locator in two Level 1 areas leave an L1L2 Ze one R-clear system and only R-set copies of the other ("A-flag MUST be preserved when the advertisement is leaked between levels", same section). RFC 9352 §6 also sets precedence: "In case of any inconsistency between the Prefix Attribute Flags advertised in the Locator TLV and in the Prefix Reachability TLV, the ones advertised in the Prefix Reachability TLV MUST be preferred." So where the same system advertises the locator prefix in a TLV 236 entry, that entry's own external bit decides X whether or not the entry carries a Prefix Attribute Flags sub-TLV (round-5 I1): the bit sits in TLV 236's fixed header, so every entry carries it (`packet.DecodeIPv6ReachabilityTLV` decodes it as `External`), RFC 5308 §2 sets it ("If the prefix was distributed into IS-IS from another routing protocol, the external bit SHALL be set to 1"), and RFC 9352 §6 binds the locator's X to it: "When included in the Locator TLV, the state of the X-flag in the Prefix Attributes Flags sub-TLV MUST match the setting of the embedded "X-bit" in any advertisement for the same prefix in TLVs 236 [RFC5308] and 237 [RFC5120]." (`rfc/full/rfc9352.txt` lines 365-368). The sub-TLV's own X-Flag is never read in TLV 236, because RFC 7794 §2.1 says of it "In TLVs 236 and 237, this flag SHOULD always be sent as 0 and MUST be ignored on receipt". When the TLV 236 entry carries a Prefix Attribute Flags sub-TLV (`packet.decodeSubTLVs` keeps TLV 236 sub-TLVs as opaque spans today), that sub-TLV decides R and A; with no sub-TLV, R and A stay from TLV 27. Ze already applies this X rule once, in BGP-LS export: `prefixAttributes` in `internal/plugins/isis/bgpls_export.go` takes the IPv6 X from the TLV 236 header and clears the sub-TLV's (its comment cites RFC 9085 §2.3.2). The resolver reuses that rule, or mirrors it with a comment naming `prefixAttributes`, so the tree holds one statement of it rather than two that can drift (round-5 N1). TLV 237 is decoded nowhere, which is already a not-known below. Exactly one system whose covering locator carries the flags (after that precedence) with R and X clear, and no covering advertisement with A set, is the originator: RFC 9352 §6, "A prefix/SRv6 Locator that is advertised by a single node and without an A-flag MUST be considered node specific." None is not-known, "locator re-advertised by <system-id>; originator not visible in this LSDB", or "locator from <system-id> carries no Prefix Attribute Flags sub-TLV; RFC 9352 §7.1: its source cannot be determined"; two or more such systems, or the A-flag on any covering advertisement, is the anycast not-known. Not known, each a named reason the keep-and-Warn arm (R-3) prints, never a zero address or an empty set read as a mismatch: no resolver registered; IS-IS engine not running in the daemon process (not configured, stopped, or run external, R2-I1); no advertised locator covers the SID; two or more systems advertise the longest covering locator with Prefix Attribute Flags present and R and X clear (anycast); any covering advertisement sets the A-flag (anycast, RFC 9352 §6); the covering locator is re-advertised (R or X set) with no Source Router ID equal to the next hop; the covering locator carries no Prefix Attribute Flags sub-TLV; a Next Hop length other than 4, 16 or 32 octets; or the next hop is in none of the decoded TLVs while the LSP carries a TLV 237 (MT IPv6 reachability), which Ze's IS-IS decodes nowhere (`internal/plugins/isis/packet/tlv.go` names 232 and 236 and no 237; grep 2026-10-11), so its host routes cannot rule the next hop out |
| I1, A-1 edit path (revised by round-2 I3 and round-3 I1) | `rewriteMPNLRISections` in `session_validation_nlritype.go` removes the rejected DSD NLRIs from MP_REACH, as it removes the §5.4 discards today, and changes nothing else, so the kept UPDATE never gains an attribute and never holds a second MP_UNREACH (RFC 7606 §3(g): "If the MP_REACH_NLRI attribute or the MP_UNREACH_NLRI [RFC4760] attribute appears more than once in the UPDATE message, then a NOTIFICATION message MUST be sent"; §5.1 says an implementation "MUST still be prepared to receive these fields in any position or combination", so another family's MP_UNREACH may already be there, and it stays byte-identical). The rejected NLRIs leave, in every case, as their own withdraw-only UPDATE body: no Withdrawn Routes, no NLRI, and one MP_UNREACH_NLRI of the MUP AFI/SAFI. A new exported `message.SynthesizeWithdrawNLRI(afi, safi, nlri)` in `rfc7606_withdraw.go`, beside `SynthesizeWithdrawFamilies`, builds it as `buildWithdrawBody(nil, nil, appendMPUnreach(...))`, the two helpers `SynthesizeWithdrawFamilies` already uses, so the Extended Length rule stays declared once, in `message.appendMPUnreach`, and the reactor declares no function of that name. `applyTypedNLRIDiscard` returns the body beside the rewritten UPDATE, `applyRFC7606` passes it out, and `session_read.go` dispatches it through the loop that already carries `SynthesizeWithdrawFamilies`' further bodies (from line 283: `wireu.NewWireUpdate` with the source context and source ID, `onMessageReceived` with the non-pool `BufHandle{ID: noPoolBufID}`, so the forward cache holds it), on the normal arm as well as the treat-as-withdraw arm. When the rewrite removed every route of the UPDATE (`typedNLRIEmptied`), the withdraw body is still dispatched: the "nothing left to withdraw" return in `session_read.go` drops the emptied UPDATE, never the withdraw body. Each dispatched body holds at most one MP_UNREACH, the RIB's first-match read sees it, and this edit creates no UPDATE that holds MP_REACH beside MP_UNREACH |
| I3, row 5 neighbour states | REACHABLE, STALE, DELAY, PROBE and PERMANENT are valid bindings; INCOMPLETE and FAILED are not. The Link-Local is re-read on every neighbour add, change or delete for an address on the shared interface, not only at establishment and reload. The netlink monitor (`internal/plugins/iface/netlink/monitor_linux.go`, `handleNeighUpdate`) already subscribes to neighbour updates but emits only router discovered and lost events for Link-Local routers, so it gains a neighbour-changed event the reactor's cache subscribes to (round-2 I4: emitted BEFORE the `IsLinkLocalUnicast` filter at the top of `handleNeighUpdate`, which today returns for every non-Link-Local address and would hide the third party's global entry A-5 starts from; and emitted only on add, delete, a MAC change, or a crossing between the valid class (REACHABLE, STALE, DELAY, PROBE, PERMANENT) and the invalid class (INCOMPLETE, FAILED), never on a flip inside one class, so the shared bus does not carry one event per NUD transition of every IPv6 neighbour; the monitor keeps the last MAC and class per neighbour key to decide, as it already keeps `knownRouters`); a change of the derived Link-Local re-advertises the affected prefixes through the per-peer re-advertise `plan/immediate/spec-bgp-link-local-only-next-hop.md` designs (its Key Design Decisions "Re-advertise" and AC-17, which `plan/immediate/spec-rfc2545-readvertise-on-condition-flip.md` also reuses; whichever spec lands first builds it, the others reuse it, round-8 I3). That path re-runs the egress next-hop decision, never a replay of a recorded next hop: a forwarded route, the case this row's derived Link-Local governs, is re-sent through the forward rail over adj-rib-in's stored copy (`request bgp adj-rib-in replay` scoped to one peer and family, relayed by `RelayStoredRoute`, which applies `applyFactsNextHop` and the pair trimming per destination, with no End-of-RIB), under a per-peer re-send hold on live forwards and a cut adj-rib-in must have ingested before it snapshots, in ascending message-ID order, because adj-rib-in trails the live forward rail and an unguarded relay can land a replaced or withdrawn route after its live replacement (round-10 B1, I1 on that spec); nothing in a pass is suppressed as unchanged, so it re-sends every stored route of the family (that spec's Known Limitations); an API route through the RIB's `outboundResend` restricted to `LocalOrigin` entries that are not `ConfigStatic`, replaying `next-hop self` as `self` (round-10 I2); a config route through the `staticMu` sibling of `deliverStaticRouteDelta` (round-9 B1 on that spec). One test per state class and one for the refresh. Open since 2026-10-12: the re-send shape is an owner question in that spec (Owner decisions, "Round 12: re-send shape"); if the owner picks ending the session with a Cease, subcode 6, a change of the derived Link-Local ends the recipient's session instead, and this row's re-send text is replaced, not patched |
| I4, row 3 send side (revised 2026-10-12, Owner decisions: "match bird and frr behaviour") | Commit checks only that `session link-local` is a `fe80::/10` address, because a config validator sees config and not the kernel. The check is its own function in `llnh.go`, wired into `OnConfigVerify` and `OnConfigure` beside `refuseLinkLocalCapabilityWithoutAddress`, never inside it: `plan/immediate/spec-bgp-link-local-only-next-hop.md` AC-15 deletes that function and its `...Sections` wrapper (capability 77 no longer requires `session link-local`) and keeps this `fe80::/10` check, which then also returns `llnhEnabledFor`'s error for a `link-local-nexthop` value outside the enumeration, the last other job of the deleted function; `TestLinkLocalSessionAddressMustBeLinkLocal` owns that case (round-8 N5). At runtime, when the session's derived interface does not hold that address (at establishment or after an interface change), Ze withholds its own Link-Local, sends the 16-octet global form, and logs one Warn. This spec builds the one Warn producer both specs share (the link-local spec's AC-13): once per peer and cause, re-armed when the link-scope snapshot changes, the consequence a parameter. Here its consequence is "carry the global next hop without a link-local" and its text is the link-local spec's AC-13 (d): "peer <name>: IPv6 routes to this peer carry the global next hop without a link-local", then interface <name> and that it does not hold the leaf address, then the command on its own line: the peer's `session link-local <addr>` path listing the interface's usable link-locals, built from `configjson.PeerOrigin` (`internal/component/bgp/configjson/traverse.go`), so a standalone peer is told `set bgp peer <p> session link-local <addr>`, a grouped peer `set bgp group <g> peer <p> session link-local <addr>`, and a leaf set on the group the group's `session` path, never a hard-coded `set bgp peer <name> ...` (round-8 N6, the link-local spec's AC-13). The link-local spec adds its other causes to the same producer, and the second fix (the same path with `delete`, to use the interface's own link-local), which exists only once that spec derives the link-local from the interface |
| I5, row 5 with no backend | `iface.ListNeighbors` returns `errIfaceNoBackendLoaded` with no backend and unsupported on non-Linux (`netlink/backend_other.go`). Then Ze sends 16 octets and logs one Warn per peer naming the cause and the fix (load the iface backend); a `ze doctor` row reports the same |
| I2, ownership | Rows 3 (`-1-2`) and 4 (`-6-1`) are owned HERE: the owner placed them in this immediate spec on 2026-10-11. `plan/immediate/spec-bgp-link-local-only-next-hop.md` (skeleton, updated 2026-09-01) claims the whole draft; its AC-7 says `-6-1` "is still met", which row 4 shows false, and its AC-9 promises no violation names the draft. That skeleton was narrowed on 2026-10-11: it excludes `-1-2` and `-6-1` and points at this spec, its AC-7 no longer lists `-6-1`, and its AC-6 is read (`-4-13` unusable for forwarding) as the no-interface case, which is row 3's withdraw arm. Row 5 (`RFC2545-3-6`) is owned here; `spec-rfc2545-readvertise-on-condition-flip.md` owns the address-event re-advertise this spec reuses, and `spec-rfc2545-next-hop-truncated-to-sixteen-bytes.md` owns relay truncation of a received 32-octet pair, which R58(b) keeps out of row 5 |
| R2-I1, process boundary | `locatororigin.Resolve` reaches IS-IS package state through a plain exported call, the class `docs/architecture/plugin/plugin-system.md` "The process boundary" names; IS-IS can run external (`reg.CLIHandler` runs `cli.RunPlugin`, `internal/plugins/isis/register.go`). Mechanism: the registry guarded by `.IsInternal()`, not a DirectBridge request/response. A DirectBridge serves only an internal plugin (`pkg/plugin/rpc/bridge.go`), so it gives an external IS-IS nothing the guarded registry does not, and a request/response per DSD NLRI is an RPC on the receive path, which the Security checklist forbids. The resolver reads an engine handle that `runISISEngine` publishes only when `p.IsInternal()` (`sdk.Plugin.IsInternal` in `pkg/plugin/sdk/sdk.go`) and clears when the engine stops; the resolver is registered at `init()` whether or not IS-IS is configured, so with no published handle it answers not-known "IS-IS engine not running in the daemon process". When IS-IS runs external, `runISISEngine` calls a package `warnIfExternal` (the `internal/plugins/static/register.go` shape: warn, never refuse, because IS-IS routing still works external) that logs once at start: "isis: running as an external plugin process, so the BGP MUP Direct Segment Discovery next-hop check cannot read this LSDB and keeps every DSD route unverified; run isis internal to enable the check". The daemon's per-route keep-Warn for the not-running state names both causes and both fixes (configure IS-IS, or run it internal), because the daemon process cannot tell "not configured" from "external" without asking the plugin manager, which this spec does not couple to. `locatororigin.Register` joins `dangerousCalls` in `internal/le/plugin/boundary/pluginboundary.go`, so `./le plugin boundary check` refuses a future registrant whose package carries no guard. The resolver takes the LSDB read lock (`LSDB.mu`, `internal/plugins/isis/lsdb/lsdb.go`) and reads a locator-to-LSP index the engine rebuilds on LSP install, purge and expiry, never a walk per NLRI (review N2) |

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to RIB plugin | received UPDATE with per-NLRI treat-as-withdraw already decided | No |
| RIB plugin to sysrib | best-change event; today carries no interface for a next hop | No |
| sysrib to FIB | route with `nextHopInterface` | No |
| BGP (MUP check) to IS-IS (locator originator) | `internal/core/locatororigin` registry: IS-IS registers a resolver at `init()`, the MUP check calls `Resolve`; neither imports the other. Same process only: the resolver answers from an engine handle published under `.IsInternal()`, and not-known otherwise (R2-I1) | No |
| Reactor to iface component (row 5) | `iface.ListNeighbors` read, plus the neighbour-changed event from the netlink monitor | No |

### Integration Points
- A sibling registry beside `nlritype` per family (MUP) - rows 1 and 2 need the UPDATE's Prefix-SID attribute, which the recognizer does not see (A-1).
- The `locatororigin` registry in `internal/core/` - row 1's originator, filled by the IS-IS plugin (Design resolutions, B1).
- The withdraw-only body for rejected DSD NLRIs, `message.SynthesizeWithdrawNLRI`, built from the helpers `SynthesizeWithdrawFamilies` uses and dispatched by the `session_read.go` multi-body loop - rows 1 and 2 withdraw rather than discard (Design resolutions, I1).
- Session interface knowledge in the reactor (`link_scope.go`) - rows 3 and 5.
- `OrderAttributes` and every writer A-3 lists - row 10.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be shown at implementation |
| No unintended coupling (components stay isolated) | No | row 1's originator lookup goes through a registered locator-originator resolver the IS-IS plugin provides, never an import of `internal/plugins/isis` from BGP (owner, 2026-10-11: "Yes, IS-IS resolver"). sysrib cannot answer it: `protocolRoute` in `sysrib.go` holds next hop, interface, protocol, metric, labels and SRv6 SID, and no originating router. Row 5 adds a dependency of the reactor on `internal/component/iface` (`iface.ListNeighbors` in `dispatch.go`), which the reactor does not import today (only `plugins/healthcheck/ip.go` does); it is a component-to-infrastructure call, not a plugin import |
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
- A-1 → Constraint: the draft says "A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message" (§3.3.6), so the verdict is per NLRI, and it is WITHDRAW, not the §5.4 discard `typedNLRIEdit` does today: a DSD route stored earlier under the same key must leave the Adj-RIB-In. The rejected NLRIs therefore leave as their own withdraw-only body, one MP_UNREACH_NLRI of the MUP family built by `message.SynthesizeWithdrawNLRI` and dispatched by the `session_read.go` multi-body loop (Design resolutions, I1), never into the dropped count
- A-1 → Constraint: the RFC 8669 §4 EBGP Prefix-SID discard (`session_validation_receive.go`) runs first. A DSD route from an EBGP peer not configured to accept the Prefix-SID therefore arrives at the MUP check with no Prefix-SID and is treated as withdrawn. AC-2 states it, and a functional test covers it
- A-1 → Constraint: the ISD rows `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1/-3/-4` carry the same `{gap}` and the same mechanism would serve them; they are not this spec's rows (`plan/pre-release/spec-rfc-verdict-fix-bgp.md` lists them). The check is written so an ISD rule is a second registration, not a second mechanism
- A-2 → Decision: `linkScope` holds prefixes only and `network.InterfacePrefixes` reads `net.InterfaceAddrs`, which returns no interface names; sessions run over global addresses (`reactor_peers.go` refuses a Link-Local `local-address`). The link-scope snapshot gains the NAME of the interface whose connected prefix contains the peer, read at the same cadence as today (session establishment and reload, never per UPDATE, from `net.Interfaces` with each interface's addresses). A multihop peer, or one no single interface reaches, has no interface, and its Link-Local next hops are treated as withdrawn with a Warn (the FRR behavior `bgp_attr.c` shows). This field is the ONE-HOP interface and the receive side reads only it: `plan/immediate/spec-bgp-link-local-only-next-hop.md` adds a separate source-interface field (the interface holding the session's local address, for a one-hop and a multihop peer alike, FRR's choice; that spec's Transformation Path step 1 uses the same words, round-8 N7), read only to choose Ze's own next hop, never by this receive side (that spec's round-7 I2, owner, 2026-10-12)
- A-2 → Constraint: the RIB plugin, not the reactor, publishes the best change to sysrib, so the interface crosses the engine-to-plugin boundary with the received route (an event field), and sysrib already accepts it (`protocolRoute.nextHopInterface` from the candidate's `Interface`). No new sysrib field is needed
- A-3 → Decision: holds. Each send path emits at most one MP attribute per UPDATE: the withdrawal writers (`forward_build.go::writeWithdrawal`, `reactor_wire.go`, `reactor_api_relay.go`) emit MP_UNREACH alone; `update_split.go::splitUpdateWithMP` emits MP_UNREACH chunks and MP_REACH chunks as separate UPDATEs; the forward loop splits any received UPDATE that `MixesNLRIFields` reports before it goes out (`forward_body.go`, both rails). So "very first" has one candidate in every UPDATE Ze sends, and no second §5.1 defect or journal row arises. `OrderAttributes` still accepts a list holding both, so AC-7's unit test feeds it one
- A-3 → Constraint: row 10 reaches more than the "three builders" the original design named: `OrderAttributes` and `WriteAttributesOrdered`; `packAttributesOrderedInto`, which the VPN, EVPN, labeled, flowspec, grouped and plugin builders call (their comments restate the old order and change with it); `update_split.go::lowRunLen` and `emitMPChunk`; the forward merge-insert in `forward_build.go`; `peer_rib_routes.go::buildRIBRouteUpdate`; `reactor_api_batch.go::buildWireModeUpdate`; `internal/perf/sender.go`. A pure forward stays byte-identical today, so a received UPDATE whose MP attribute is NOT first (an older speaker, which §5.1 tells receivers to accept) would go out still violating §5.1: the forward path moves the MP attribute first for that source only, and the zero-copy identity survives for every source that already complies (FRR and BIRD both do)
- A-4 → Decision: GoBGP implements the MUP SAFI from v3.6.0, and the interop image pins v3.31.0 (`test/interop/Dockerfile.gobgp`). Its CLI DSD form (`cmd/gobgp/global.go::parseMUPDirectSegmentDiscoveryRouteArgs`) REQUIRES `prefix`, `locator-node-length`, `function-length` and `behavior` and always attaches an SRv6 L3 Service Prefix-SID, so GoBGP can send the stored case and a next hop of the operator's choosing, but never a DSD route without a Prefix-SID. The interop scenario proves the positive case and the keep-and-warn case (owner decision on row 1) against GoBGP; the without-Prefix-SID case (AC-2) and the proven-mismatch case (AC-1) are proven by ze-peer functional tests, and the interop scenario's README says so. The README says the GoBGP CLI cannot send them (review N3): GoBGP's gRPC API takes arbitrary path attributes and could, so the limit is the CLI the scenario drives, not GoBGP
- A-5 → Decision: FRR (`bgpd/bgp_updgrp_packet.c`) keeps a third-party global, keeps a received Link-Local in a 32-octet field, and substitutes its OWN Link-Local for `::`; it never derives the third party's Link-Local. BIRD sends 16 octets. Neither does what the owner chose for row 5 ("Fix it": read the third party's Link-Local from the Linux neighbour table), so Ze has no reference implementation to copy and the AC is written from RFC 2545 §3 alone
- A-5 → Constraint: the kernel neighbour table maps an address to a MAC and a device, not a global address to a Link-Local. The lookup is: the third-party global's entry on the shared interface gives its MAC; a `fe80::/10` entry on the same device with the same MAC is that router's Link-Local. An entry in REACHABLE, STALE, DELAY, PROBE or PERMANENT is a valid binding (Linux holds a working neighbour in STALE most of the time); no entry, an INCOMPLETE or FAILED entry, or two candidate Link-Locals for one MAC means no Link-Local is known: the field goes out as 16 octets, as today, with one rate-limited Warn naming the next hop and the reason. The read goes through `iface.ListNeighbors` (the iface component, already consumed by the BGP healthcheck plugin), is cached per distinct third-party next hop, and is refreshed on a neighbour change for a cached address (Design resolutions, I3) as well as at establishment and reload, never per UPDATE; a changed Link-Local re-advertises the affected prefixes. A received complete pair relayed unchanged keeps its Link-Local (R58(b)); the lookup applies only where Ze writes the next hop
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
| Who else touches this path? | `plan/immediate/spec-srv6-bestpath-resolvability.md` (same RIB candidate filter as row 1's Prefix-SID read), `plan/immediate/spec-bgp-prefix-sid-rfc-defects.md` (Prefix-SID parsing), `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (ledger rows). Row 10 changes the byte order of every BGP-LS UPDATE `plan/spec-bgp-epe-peer-segments-and-srv6-peernode.md` originates: whichever lands second writes its expectations against the other. `plan/immediate/spec-bgp-link-local-only-next-hop.md` (skeleton) claims the whole linklocal draft; rows 3 (`-1-2`) and 4 (`-6-1`) are owned here and that skeleton is narrowed to exclude them, and its AC-6 (`-4-13`) shares row 3's receive path. `plan/immediate/spec-rfc2545-readvertise-on-condition-flip.md` owns the address-event re-advertise row 5 reuses for a changed Link-Local (whichever lands first builds it). `plan/immediate/spec-rfc2545-next-hop-truncated-to-sixteen-bytes.md` owns relay truncation of a received 32-octet pair, which R58(b) keeps out of row 5; both touch `link_scope.go` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE with DSD route, no Prefix-SID, from a peer | → | MUP check treat-as-withdraw | `test/plugin/mup-dsd-without-prefix-sid-withdraw.ci` |
| UPDATE with DSD route whose next hop differs from the locator originator | → | MUP check, `locatororigin.Resolve` answered by the IS-IS resolver | `test/plugin/mup-dsd-nexthop-locator-mismatch-withdraw.ci` |
| UPDATE with 16-octet Link-Local next hop on a capability-77 session | → | RIB best change carries the session interface to sysrib and FIB | `test/plugin/linklocal-capability-nexthop-interface.ci` |
| UPDATE with a malformed 32-octet next hop on a capability-77 session | → | treat-as-withdraw | `test/plugin/linklocal-capability-malformed-nexthop-withdraw.ci` |
| Static route with a third-party global next hop on the common subnet | → | `link_scope.go` third-party Link-Local | `test/plugin/rfc2545-third-party-linklocal.ci` |
| Any MP announcement from config | → | `OrderAttributes` and every builder | `test/plugin/rfc7606-mp-reach-first.ci` |
| DSD route whose locator originator the IGP table cannot resolve | → | MUP check keeps the route and logs a Warn (owner decision, row 1) | `test/plugin/mup-dsd-locator-unresolved-kept.ci` |
| Capability-77 session whose interface does not hold the configured `session link-local` | → | `link_scope.go` withholds the own Link-Local and logs the Warn (AC-3 send side) | `test/plugin/linklocal-session-address-off-interface-warn.ci` |
| DSD route from an EBGP peer not accepting the Prefix-SID | → | RFC 8669 §4 discard, then MUP treat-as-withdraw | `test/plugin/mup-dsd-ebgp-prefix-sid-discarded-withdraw.ci` |
| Row 9 | → | none: OUT, ledger only | the row's `{feature-declined}` and `./le rfc check` clean for `rfc9830` |

## Acceptance Criteria

AC numbers changed in the split: rows 9 and 10 were AC-9 and AC-10 before it and
are AC-6 and AC-7 here; the ledger AC was AC-11 over eight stems and is AC-8 here
over this spec's five. AC-6, AC-7, AC-8, AC-12 and AC-13 of the unsplit spec went to
the FSM and EPE specs.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 (row 1) | A DSD route arrives whose MP_REACH global next hop is none of the addresses the originating LSP advertises, as the IS-IS locator-originator resolver answers it (the one system, across both IS-IS levels, whose SRv6 Locator TLV 27 longest-matches the Prefix-SID's SID with Prefix Attribute Flags present and R and X clear; its TLV 140, TLV 232 and host /128 TLV 236 addresses; the next hop compared per its length: 16 octets whole, 32 octets by the global half, 4 octets never in the set; Design resolutions, B1) | The NLRI is treated as withdrawn (removed from the Adj-RIB-In if stored, carried as an MP_UNREACH NLRI, never merely discarded), the other NLRIs of the UPDATE are processed, and a Warn log names the peer, the route, the next hop and the originator's system ID. A next hop equal to the TLV 140 address, to a TLV 232 address, to a host /128 in TLV 236, or to an IPv6 Source Router ID sub-TLV on a covering locator is stored (positive polarity, one case each). When the originator is not known, the route is KEPT and a rate-limited Warn names the peer, the route, the SID and the reason with its fix: no resolver registered; IS-IS engine not running in the daemon process (configure IS-IS, or run it internal); no advertised locator covers the SID; two or more systems advertise the covering locator with Prefix Attribute Flags present and R and X clear, a leaker not counted (anycast); any covering advertisement sets the A-flag (anycast, RFC 9352 §6), with a same-system TLV 236 Prefix Attribute Flags sub-TLV preferred over TLV 27 for R and A, and a same-system TLV 236 entry's external bit deciding X whether or not that entry carries the sub-TLV (round-5 I1); the covering locator re-advertised by another system (R or X set) with no Source Router ID equal to the next hop; the covering locator carrying no Prefix Attribute Flags sub-TLV (RFC 9352 §7.1); a Next Hop length other than 4, 16 or 32 octets; next hop unmatched while the LSP carries an undecoded TLV 237 (owner: "Keep it, log a warning"): only a proven mismatch withdraws |
| AC-2 (row 2) | A DSD route arrives in an UPDATE with no Prefix-SID attribute, including one whose Prefix-SID the RFC 8669 §4 EBGP discard removed | The NLRI is treated as withdrawn as in AC-1 and the other NLRIs of the UPDATE are processed; the same route with a Prefix-SID is stored |
| AC-3 (row 3) | A Link-Local next hop (16 octets, or the Link-Local half of 32) arrives on a session over a known interface | The installed kernel route names that interface as its device; on a session with no known interface the route is treated as withdrawn with a Warn log naming the reason (FRR behavior). On send, the advertised own Link-Local is an address of the session's interface. Commit refuses a `session link-local` that is not a `fe80::/10` address (the validator sees config, not the kernel). When the session's derived interface does not hold the configured address, at establishment or after an interface change, Ze withholds its own Link-Local, sends the 16-octet global form, and logs one Warn through the producer `plan/immediate/spec-bgp-link-local-only-next-hop.md` AC-13 shares, with consequence "carry the global next hop without a link-local", naming the peer, the leaf, the interface and the fix, the peer's `session link-local <addr>` path built from `configjson.PeerOrigin` (a grouped peer is told its `set bgp group <g> peer <p> ...` path), with the interface's usable link-locals; a second UPDATE logs nothing; when the interface gains the address the 32-octet form resumes (Design resolutions, I4) |
| AC-4 (row 4) | On a capability-77 session, a 32-octet Next Hop field that is not a Global (or `::`) followed by a Link-Local, and not the two-Link-Local form §6 tells receivers to accept | The UPDATE is handled as treat-as-withdraw (RFC 7606 §7.3); a well-formed pair and the two-Link-Local form are accepted; a wrong length still resets the session |
| AC-5 (row 5) | Ze writes a next hop whose global address is another router on the subnet Ze shares with the recipient (owner: "Fix it") | The Next Hop field is 32 octets and its second address is that router's Link-Local, read from the Linux neighbour table on the send path (A-5: same device, same MAC as the global's entry, entry in REACHABLE, STALE, DELAY, PROBE or PERMANENT). With no usable neighbour entry (none, INCOMPLETE, FAILED, or two candidate Link-Locals) the field is 16 octets, as today, and a rate-limited Warn names the next hop and the reason. With no iface backend loaded, or on a platform without one, the field is 16 octets, one Warn per peer names the cause and the fix (load the iface backend), and `ze doctor` reports the same. A neighbour change for a cached next hop refreshes the cache, and a changed Link-Local re-advertises the affected prefixes. A received complete pair relayed unchanged keeps its Link-Local. The neighbour read is cached per distinct next hop and never runs per UPDATE |
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
| `TestMUPDSDNextHopLocatorMismatch` | same | AC-1 both polarities, and the unresolvable case; per Next Hop length: 16 octets compared whole; 32 octets compared by the global half (a Link-Local half equal to a set member does not match); 4 octets a proven mismatch against a known originator and kept with the Warn against a not-known one; any other length not-known | |
| `TestLinkLocalNextHopCarriesSessionInterface` | `internal/component/bgp/plugins/rib/` | AC-3 receive | |
| `TestLinkLocalSessionAddressMustBeLinkLocal` | `internal/component/bgp/plugins/llnh/` | AC-3 send side, commit: a `session link-local` outside `fe80::/10` is refused naming the leaf and the fix; an `fe80::/10` address is accepted; a `link-local-nexthop` value outside the enumeration returns `llnhEnabledFor`'s error, the job this function takes over from the deleted `refuseLinkLocalCapabilityWithoutAddress` (round-8 N5) | |
| `TestLinkLocalOwnAddressWithheldOffInterface` | `internal/component/bgp/reactor/link_scope_test.go` | AC-3 send side, runtime: the session interface holds the address (32 octets), does not (16 octets and one Warn naming leaf, interface and fix), gains it after an interface change (32 octets again) | |
| `TestLinkLocalNextHopNoInterfaceWithdrawn` | `internal/component/bgp/plugins/rib/` | AC-3 receive: a Link-Local next hop on a session with no known interface is treated as withdrawn with the Warn | |
| `TestLinkLocalCapabilityMalformedThirtyTwoOctet` | `internal/component/bgp/message/` | AC-4, including the two-Link-Local accepted form and the wrong-length reset | |
| `TestRFC2545ThirdPartyLinkLocal` | `internal/component/bgp/reactor/link_scope_test.go` | AC-5: neighbour entry found (32 octets), absent or ambiguous (16 octets and Warn), relayed pair unchanged | |
| `TestRFC2545NeighbourStateClasses` | same | AC-5: REACHABLE, STALE, DELAY, PROBE and PERMANENT each yield 32 octets; INCOMPLETE and FAILED each yield 16 octets and the Warn | |
| `TestRFC2545NeighbourChangeRefreshesAndReadvertises` | same | AC-5: a neighbour-changed event for a cached next hop refreshes the cache, and a changed Link-Local re-advertises the affected prefixes; an event for an uncached address does nothing | |
| `TestRFC2545NoIfaceBackendSixteenOctets` | same | AC-5: `iface.ListNeighbors` answering no backend loaded, or unsupported, yields 16 octets and one Warn per peer naming the cause and the fix | |
| `TestNeighbourChangedEventEmitted` | `internal/plugins/iface/netlink/` | AC-5: the monitor emits the neighbour-changed event for a GLOBAL IPv6 neighbour as well as a Link-Local one (the event precedes the Link-Local filter), on add, delete, MAC change, and a valid-to-invalid or invalid-to-valid state crossing; a REACHABLE to STALE flip emits nothing; the existing router events are unchanged | |
| `TestDoctorNeighbourTableUnreadable` | the doctor check's package, placed at implementation | AC-5: an unreadable neighbour table is reported with the fix | |
| `TestRFC7606Section51MPAttributeEncodedFirst` (existing, flipped) | `internal/component/bgp/message/rfc7606_mp_first_test.go` | AC-7: today it demonstrates the `{gap}` through `rfcgap.Demonstrate` and carries a `gap` polarity `RFC requirement: RFC7606-5.1-1` tag. Flipped, it asserts MP_REACH first directly, its tag takes positive polarity, and its comment block stops saying Ze does not meet §5.1. It is RFC-tagged, so the flip needs the owner's approval at implementation (`ai/rules/rule-precedence.md`: a fix that changes an RFC-tagged test is asked first); the implementer asks before editing it and records the answer in Owner decisions | |
| `TestOrderAttributesBothMPAttributes` | `internal/core/bgp/attribute/origin_test.go` | AC-7: `OrderAttributes` given a list holding both MP attributes puts one MP attribute first, and given MP_REACH alone puts it first | |
| `TestLocatorOriginResolveArms` | `internal/core/locatororigin/` | AC-1: no resolver registered answers not-known with its reason (never a zero address or an empty set), a registered resolver's known and not-known answers pass through with their reasons, a resolver error is returned as an error | |
| `TestISISLocatorOriginator` | `internal/plugins/isis/` | AC-1: the SID longest-matches the advertised TLV 27 locators (a /48 and a /64 both covering it pick the /64's LSP); the answer set holds that LSP's TLV 140, TLV 232 and host /128 TLV 236 addresses and no others; no covering locator, two systems on the covering locator (anycast), and an unmatched next hop with TLV 237 present each answer not-known with their reason; multi-level (round-3 B1): an L1L2 router advertising the locator in both its Level 1 and Level 2 LSPs, and across two fragments, is one originator, not anycast; a Level 2 copy with R set beside the Level 1 original resolves to the Level 1 system; a leaked locator alone, with R set and again with X set, answers not-known "re-advertised" naming the advertising system ID; the same leaked locator with an IPv6 Source Router ID sub-TLV equal to the next hop answers a known match; a covering locator with no Prefix Attribute Flags sub-TLV answers not-known with its reason; anycast by A-flag (round-4 I1): PE1 with R clear in Level 1 and only an R-set leaked copy of PE2 carrying the A-flag answers not-known anycast, not PE1; flag precedence (round-4 I1): a system whose TLV 27 flags have R clear but whose TLV 236 entry for the same prefix carries a Prefix Attribute Flags sub-TLV with R set is not the originator, and the TLV 236 sub-TLV's X-Flag is ignored (TLV 236's external bit decides X); external bit with no sub-TLV (round-5 I1): TLV 27 flags present with X clear, and a same-system TLV 236 entry for the same prefix with the external bit set and no Prefix Attribute Flags sub-TLV, answers not-known "re-advertised" naming that system, and the route is kept; with no published engine handle (engine not running, or external) it answers not-known "IS-IS engine not running in the daemon process" | |
| `TestISISWarnIfExternal` | `internal/plugins/isis/` | R2-I1: started external, the engine logs the one start Warn naming the DSD check and the fix and publishes no handle; started internal, it logs nothing and publishes the handle | |
| `TestSynthesizeWithdrawNLRI` | `internal/component/bgp/message/rfc7606_withdraw_test.go` | AC-1, AC-2: the body holds no Withdrawn Routes, no NLRI and exactly one MP_UNREACH_NLRI carrying the given AFI/SAFI and NLRI, flags Optional, Extended Length set exactly when the value exceeds 255 octets (value 255 and 256) | |
| `TestRejectedDSDWithdrawBodyDispatched` | `internal/component/bgp/reactor/session_validation_nlritype_test.go` | AC-1, AC-2: an UPDATE holding kept and rejected DSD NLRIs dispatches the rewritten UPDATE (rejected NLRIs gone from MP_REACH, an MP_UNREACH of another family byte-identical) and one withdraw-only body of the MUP family with a non-pool BufHandle; the same on the treat-as-withdraw arm; when every DSD NLRI is rejected the withdraw body is still dispatched; no dispatched body holds two MP_UNREACH | |
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
| `linklocal-session-address-off-interface-warn` | `test/plugin/` | AC-3 send side, runtime withhold | |

The row 3 and row 5 kernel tests (`linklocal-capability-nexthop-interface`, `linklocal-session-address-off-interface-warn`, `rfc2545-third-party-linklocal`) need the kernel: each carries `option=needs-linux:caps=net-admin`, creates a dummy link, and for row 5 seeds a neighbour entry, the shape `test/plugin/fib-mpls-kernel.ci` already uses.

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
- `internal/component/bgp/reactor/session_validation_nlritype.go` - rows 1, 2 (consult the sibling registry beside `nlritype`; `rewriteMPNLRISections` removes the rejected DSD NLRIs and `applyTypedNLRIDiscard` returns their withdraw-only body from `message.SynthesizeWithdrawNLRI`, A-1 and Design resolutions I1)
- `internal/component/bgp/message/rfc7606_withdraw.go` - rows 1, 2: `SynthesizeWithdrawNLRI`, built on `buildWithdrawBody` and `appendMPUnreach` (Design resolutions, I1); `rfc7606_withdraw_test.go` gains `TestSynthesizeWithdrawNLRI`
- `internal/plugins/isis/register.go` and a new resolver file in `internal/plugins/isis/` - row 1: register the locator-originator resolver at `init()`; `runISISEngine` publishes the engine handle only under `p.IsInternal()` and calls `warnIfExternal` otherwise (R2-I1); the resolver takes the LSDB read lock, reads a locator index rebuilt on LSP change that spans Level 1 and Level 2 and keys advertisers by system ID, longest-matches the SID against TLV 27, walks the covering locators' sub-TLVs for Prefix Attribute Flags (type 4, including the A-flag, bit 4) and IPv6 Source Router ID (type 12), prefers a same-system TLV 236 entry's Prefix Attribute Flags sub-TLV, when present, for R and A, and takes X from that entry's external bit whether or not the sub-TLV is present, reusing or mirroring the X rule of `prefixAttributes` in `internal/plugins/isis/bgpls_export.go` (round-4 I1, round-5 I1, N1), and answers the originating system's TLV 140, TLV 232 and host /128 TLV 236 addresses (B1, round-3 B1)
- `internal/le/plugin/boundary/pluginboundary.go` - row 1: `locatororigin.Register` added to `dangerousCalls`, so the guard is checked (R2-I1)
- `internal/component/bgp/reactor/session_validation.go` and `session_read.go` - rows 1, 2: `applyRFC7606` returns the withdraw-only body, and the `session_read.go` multi-body loop dispatches it on both arms, including when the rewrite emptied the UPDATE (Design resolutions, I1)
- `internal/plugins/iface/netlink/monitor_linux.go` (`handleNeighUpdate`) and the iface event registration in `internal/component/iface/register.go` - row 5: emit a neighbour-changed event before the Link-Local filter, for any IPv6 neighbour, on add, delete, MAC change and valid/invalid class crossing only (round-2 I4), beside the existing router discovered and lost events
- the `ze doctor` check registration for the neighbour table - row 5 (Integration Checklist)
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - row 3 (next hop carries an interface)
- `internal/component/sysrib/sysrib.go` - row 3: the candidate `Interface` field exists (A-2) and the BGP producer fills it; the implementer records at file:line whether this file needed an edit
- `internal/component/bgp/message/rfc7606.go` - row 4
- `internal/component/bgp/plugins/llnh/llnh.go` - row 3 send side: commit refuses a `session link-local` outside `fe80::/10`, in its own function beside `refuseLinkLocalCapabilityWithoutAddress`, which the link-local spec's AC-15 then deletes, with its `...Sections` wrapper (the runtime interface check and the shared Warn producer live in `link_scope.go`, Design resolutions I4)
- `internal/component/bgp/reactor/link_scope.go`, `internal/core/network/connected.go` - rows 3, 5 (interface name in the snapshot; third-party Link-Local via `iface.ListNeighbors`)
- `internal/core/bgp/attribute/origin.go`, `internal/component/bgp/message/update_build.go`, `update_build_vpn.go`, `update_build_evpn.go`, `update_build_labeled.go`, `update_build_plugin.go`, `update_split.go`, `internal/component/bgp/reactor/peer_rib_routes.go`, `reactor_api_batch.go`, `forward_build.go`, `internal/perf/sender.go` - row 10 (A-3)
- `internal/core/bgp/attribute/origin.go` comment above `OrderAttributes` - row 10 stale comment: it calls the order "a house convention rather than a requirement", which RFC 7606 §5.1 makes false; rewritten with the code change (`ai/rules/stale-comments.md`)
- `internal/component/bgp/message/rfc7606_mp_first_test.go` - row 10: `TestRFC7606Section51MPAttributeEncodedFirst` flipped from gap demonstration to positive assertion, after the owner approves the RFC-tagged test change
- `docs/architecture/wire/mp-nlri-ordering.md`, `docs/architecture/wire/update-packing.md` (the order diagram and "Why This Order", which states the `{gap}` on RFC7606-5.1-1 and names the demonstrating test) - row 10
- `internal/le/interoplab/bgp/check_special.go` - map entries binding `bgp-mp-reach-first-frr`, `bgp-mp-reach-first-bird`, `bgp-linklocal-capability-interface-frr` and `bgp-mup-dsd-gobgp` to their checkers
- `internal/le/interoplab/bgp/check_rfc.go` (`checkRFC2545NextHops`) - row 5: extend the existing `bgp-rfc2545-linklocal-nexthop-frr` checker with the third-party Link-Local assertion
- `rfc/short/draft-ietf-bess-mup-safi.md`, `rfc/short/draft-ietf-idr-linklocal-capability.md`, `rfc/short/rfc2545.md`, `rfc/short/rfc9830.md`, `rfc/short/rfc7606.md` - AC-8
- `test/plugin/*.ci`, `test/encode/*.ci`, `test/exabgp-compat/**/*.ci`, `test/draft/plugin/*.ci` and Go `_test.go` files carrying MP attribute hex - row 10 expectations (A-6)

## Files to Create
- the nine `test/plugin/*.ci` files the Functional Tests table names, and the four new interop scenarios named above
- the MUP check registry beside `internal/core/bgp/nlri/nlritype/` (A-1), placed by `ai/rules/architecture.md` at design review
- `internal/core/locatororigin/` - row 1: the registry (`Register(name, Resolver)`, `Resolve(sid netip.Addr)` answering the originator's address set, whether it is known with the reason when it is not, and an error) and its test
- `internal/component/bgp/reactor/session_validation_nlritype_test.go` - `TestRejectedDSDWithdrawBodyDispatched`
- one `internal/le/interoplab/bgp/check_*.go` checker file (and its `_test.go`) per new scenario: `bgp-mp-reach-first-frr` and `bgp-mp-reach-first-bird` (capture asserts the first path attribute is type 14 or 15), `bgp-linklocal-capability-interface-frr` (kernel route device), `bgp-mup-dsd-gobgp` (route stored, keep-and-Warn log line)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no new leaf; the YANG work moved with rows 6 and 8 to the two `plan/` specs |
| YANG validation constraints | N-A | none new |
| YANG custom validators | Yes | row 3 send side: `session link-local` must be a `fe80::/10` address, a new check in `llnh.go` wired beside `refuseLinkLocalCapabilityWithoutAddress`, never inside it, because `plan/immediate/spec-bgp-link-local-only-next-hop.md` AC-15 deletes that function and its `...Sections` wrapper and keeps this check; whether it is on the session's interface is a runtime check (Design resolutions, I4), because the validator sees config and not the kernel |
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
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, checked against the MUP check and the llnh refusal this spec adds; `docs/architecture/doctor-and-health-checks.md` for the new neighbour-table doctor check |
| 6 | Has a user guide page? | Yes | the IPv6 next-hop guide page that covers `session link-local`, found through `ai/INDEX.md` |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/mp-nlri-ordering.md`, `docs/architecture/wire/update-packing.md` (order diagram and the "Not RFC 7606 §5.1" reason, which becomes false) |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | the five `rfc/short/` files above and `docs/features/rfc-status.md` (generated by `./le rfc index-update`) |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it lists MP attribute order |
| 12 | Internal architecture changed? | Yes | `docs/architecture/wire/mp-nlri-ordering.md` (row 10); the RIB-to-sysrib next-hop interface (row 3) in `docs/architecture/plugin/rib-storage-design.md`. `docs/architecture/behavior/fsm.md` moved with rows 6-7 to the FSM spec |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | the iface neighbour-changed event type, registered in the iface namespace (`internal/component/iface/register.go`): `docs/architecture/iface/management.md`, which documents the router discovered and lost events; the `locatororigin` registry and the IS-IS resolver registration: `docs/architecture/isis/isis-4-component-config.md`; the MUP check registry beside `nlritype`: the page `ai/CODE-TO-DOCS.md` names for `nlritype`; the neighbour-table doctor check: `docs/architecture/doctor-and-health-checks.md`; the `dangerousCalls` entry: `docs/architecture/plugin/plugin-system.md` "The process boundary" where it lists watched calls |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared by the files this spec changes (`// Design:` headers): `docs/architecture/wire/messages.md` and `docs/architecture/wire/attributes.md` (row 10 attribute order: affected, update with the builder change); `docs/architecture/plugin/rib-storage-design.md` (row 3 next hop with interface: affected); `docs/architecture/config/syntax.md` (row 3 send-side refusal: the implementer reads the page and edits it where the refusal text belongs there); `docs/architecture/core-design.md` (unaffected: no change to the core/registration design); `docs/architecture/bgp/structural-forwarding.md` (declared by `forward_body.go`: affected, because row 10 moves a late MP attribute first on a forward from a non-compliant source and rows 1-2 rely on the mixed-field split; the page states when a forward stays byte-identical); `docs/architecture/isis/isis-4-component-config.md` (declared by the IS-IS `register.go`: affected, the plugin registers the locator-originator resolver at `init()`); `docs/features/interfaces.md` (declared by the netlink monitor: affected, the monitor emits the new neighbour-changed event). `docs/architecture/wire/nlri-bgpls.md` moved with rows 8, 11, 12 to the EPE spec. Re-derive at implementation with `./le spec citation anchors spec plan/immediate/spec-bgp-unowned-rfc-rows.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | check `session link-local` examples against the new refusal |

## Implementation Steps

Defects land first, each in its own commit, so no defect waits on another.

1. **Phase: Wiring** -- the nine wiring `.ci` tests written and red.
2. **Phase: row 10** -- `OrderAttributes` and every writer A-3 lists put MP_REACH and MP_UNREACH first; the forward moves a late MP attribute first for non-compliant sources only; expectations regenerated; page and RFC 7606 row edited. Own commit.
3. **Phase: rows 2 then 1** -- the MUP check registry (A-1) and the withdraw-only body (`SynthesizeWithdrawNLRI`) with its dispatch test; Prefix-SID presence; then the `locatororigin` registry, the IS-IS resolver, and the originator comparison with keep-and-warn when not known. Own commit.
4. **Phase: rows 3 and 4** -- the receive next hop carries the session interface through sysrib to the FIB; malformed 32-octet check; commit check on `session link-local` and the runtime withhold. Own commit.
5. **Phase: row 5** -- the netlink neighbour-changed event, then the third-party Link-Local from the neighbour table, cached, refreshed and re-advertised on change, with the no-backend Warn and the doctor check. Own commit.
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
| Process boundary guarded | `./le plugin boundary check` exits clean with `locatororigin` in `dangerousCalls` (R2-I1); the check is presence-based, so `TestISISWarnIfExternal` is the proof of the guard |

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
| Row 1: where does the locator's originator come from, given sysrib holds no originating router (review B1)? | "Yes, IS-IS resolver" | A `locatororigin` registry in `internal/core/`, filled by the IS-IS plugin from the LSDB (TLV 27 locator, TLV 140 address); BGP never imports IS-IS (Design resolutions, B1) |
| Rows 3 and 4: owned here or by `plan/immediate/spec-bgp-link-local-only-next-hop.md`? | placed in this spec | Owned here; that skeleton excludes `-1-2` and `-6-1` and points at this spec (Design resolutions, I2) |
| Round-2 I5: the narrowed linklocal skeleton owns no open row; close it or rewrite it? | "Rewrite to AC-7/AC-8" | `plan/immediate/spec-bgp-link-local-only-next-hop.md` is rewritten to its AC-7 re-reads and AC-8 interop only, in its own commit; rows `-1-2` and `-6-1` stay here |
| Approve the design after the round-5 design review (0 BLOCKER, I1 folded in 4fd32decbc)? (Thomas, 2026-10-11) | "Approve" | Status moves from `design` to `ready`, so `/ze-implement` may claim it |
| Link-local spec round-6 B1 and I1: where does Ze's own IPv6 next hop come from, and does capability 77 still require `session link-local`? (Thomas, 2026-10-11, recorded in `plan/immediate/spec-bgp-link-local-only-next-hop.md`) | "match bird and frr behaviour" | A consistency edit here, 2026-10-12, no scope change: the link-local spec removes the capability-77 refusal from `refuseLinkLocalCapabilityWithoutAddress` (its AC-15), so this spec's `fe80::/10` commit check is its own function beside that one, not a path through it; the I4 runtime Warn is the one producer both specs share (the link-local spec's AC-13), built here with consequence "carry the global next hop without a link-local". This spec's ACs, behavior and status are unchanged (Design resolutions, I4) |
| Link-local spec round-7 I2 and N12: a multihop peer's own-address interface collides with A-2, and this spec's I4 changes the reactor `-4-3` tests that feed the leaf (Thomas, 2026-10-12, recorded in `plan/immediate/spec-bgp-link-local-only-next-hop.md`) | "keep the unowned-rows A-2 contract (multihop peer has no session interface); use a separate source-interface field" | A consistency edit here, 2026-10-12, no scope change: A-2's interface is the one-hop interface and the receive side reads only it; the link-local spec adds the source-interface field. Because I4 sends the leaf only when the interface holds it, this spec, which lands first, gives the reactor `-4-3` tests that feed `fe80::1` (`draft_ietf_idr_linklocal_capability_announce_test.go` lines 85 and 107, `draft_ietf_idr_linklocal_capability_test.go` lines 135, 166 and 193) a snapshot whose interface holds it, with a fresh discrimination record for each changed tagged unit. Status unchanged |
| One spec or several? | "Split into three" | This spec keeps the defects and row 9; `plan/spec-bgp-fsm-delayopen-and-automatic-start.md` takes rows 6-7; `plan/spec-bgp-epe-peer-segments-and-srv6-peernode.md` takes rows 8, 11, 12 |

## Open Questions (owner)

- The re-send shape row 5 (I3) reuses: end the recipient's session with a
  Cease, subcode 6, or re-send in place. Asked and evidenced in
  `plan/immediate/spec-bgp-link-local-only-next-hop.md`, Owner decisions,
  "Round 12: re-send shape"; the answer there decides row 5's re-advertise.

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
| Row 4 treats a malformed 32-octet field on a capability-77 session as withdrawn (review N4) | Drop the Link-Local half and keep the route, as FRR (`bgp_attr.c`, MP_REACH next-hop parse) and BIRD (`packets.c`, the 32-octet handling) both do | The draft says "the implementation MUST handle the malformed UPDATE message using the approach of "treat-as-withdraw"" (§6); both references diverge from it, so Ze follows the draft, and the `linklocal-capability-malformed-nexthop-withdraw` test comment records the divergence |
| Row 1 locator originator from a registered IS-IS resolver | Look it up through sysrib; import IS-IS from BGP | sysrib's `protocolRoute` holds no originating router; an import couples a BGP component to a plugin. Owner: "Yes, IS-IS resolver" |
| Rows 1 and 2 withdraw by dispatching the rejected NLRIs as their own withdraw-only body (round-3 I1) | Extend `rewriteMPNLRISections` to add an MP_UNREACH; a three-arm reactor edit that writes or extends an MP_UNREACH in the kept UPDATE | The first only shortens NLRI inside existing MP attributes; the second restated the Extended Length rule `message.appendMPUnreach` holds, reused that function's name, and needed a separate carrier when another family's MP_UNREACH was present. `SynthesizeWithdrawFamilies`' helpers and the `session_read.go` multi-body loop already do both jobs, so the rejected NLRIs take that path in every case (`ai/rules/principles.md`, declare once) |
| The IS-IS resolver is a registry guarded by `.IsInternal()`, not a DirectBridge request/response (round-2 I1) | DirectBridge request/response to IS-IS; an event-fed originator index in the daemon | DirectBridge serves internal plugins only, so it adds nothing over the guarded registry and costs an RPC per DSD NLRI; an event-fed index copies the LSDB's locator and address sets onto the bus for one check. The guard makes the external case a not-known with a start Warn and a fix, and the boundary check enforces it |
| Row 1 compares the next hop with every address the originating LSP advertises (round-2 I2) | TLV 140 only | A PE's BGP next hop is often a loopback other than its TE Router ID; comparing TLV 140 alone withdraws a correct route, the opposite of the owner's keep-unless-proven decision |
| The originator is a system across both IS-IS levels, and only a covering locator whose Prefix Attribute Flags are present with R and X clear proves it (round-3 B1) | One LSDB level; the single LSP holding the covering locator | In a multi-level domain the Level 2 copy of a Level 1 PE's locator is the leaker's, and reading the leaker as the originator withdraws a correct route. RFC 7794 §2.1: "R-Flag: Re-advertisement Flag (Bit 1) Set when the prefix has been leaked from one level to another (upwards or downwards)." RFC 9352 §7.1 makes the sub-TLV a MUST only on a leaked or redistributed locator, and without it "receivers will be unable to determine the correct source of the advertisement", so an absent sub-TLV proves nothing. Consequence: a PE that omits the sub-TLV (RFC 9352 §7.1: it "SHOULD be included in the Locator TLV") never yields a proven mismatch, which the owner's keep-unless-proven decision accepts. A single R/X-clear advertiser is the originator because RFC 9352 §6 says "A prefix/SRv6 Locator that is advertised by a single node and without an A-flag MUST be considered node specific"; the A-flag on any covering advertisement, and a same-system TLV 236 Prefix Attribute Flags sub-TLV preferred over TLV 27, follow the same section (round-4 I1). X set with R clear makes the redistributing router the IS-IS advertiser; treating it as not-known is the conservative reading and keeps the route, as the owner decided |
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
