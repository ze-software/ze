# Spec: bgp-unowned-rfc-rows -- the BGP RFC defects no spec owned

| Field | Value |
|-------|-------|
| Status | blocked |
| Priority | P1 |
| Release reason | Remaining wire and routing obligations concern implemented BGP families, not optional feature rows already split out. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Blocker

Handed back on 2026-11-14 by the agent that split this spec and prepared its
closure. The split is done and committed; the closure is not, and nothing below
is a product defect.

| What blocks the two closure commits | Producer | What clears it |
|-------------------------------------|----------|----------------|
| Five owed runs are not in: the interop scenarios `bgp-rfc2545-third-party-linklocal-frr` and `bgp-sr-policy-gobgp` (Linux Docker lab), `./le rfc check`, and the unit packages `./internal/plugins/iface/netlink/` (Linux) and `./internal/component/bgp/config/`. Every other owed run is green and recorded under Pre-Commit Verification | the main thread schedules them | Each green, its result written into the Goal Validation and Pre-Commit Verification cells that say "not yet run" |
| Two specs still cite this file by path and commit B removes it: `plan/post-release/features/spec-bgp-epe-peer-segments-and-srv6-peernode.md` (lines 20, 171, 364) and `plan/post-release/features/spec-bgp-fsm-delayopen-and-automatic-start.md` (lines 20, 461). No other tracked file cites the path (`grep -rn`, 2026-11-14); the two `.ci` tests and the sibling specs cite the bare stem. The spec edit gate refused both repoints on 2026-11-14 for reasons the repoint does not cause: the EPE spec is `ready` with no `## Existing Mechanisms` section (`writeSpecMechanisms`, `internal/le/hookruntime/specmechanisms.go`) and cites a producer `../../peer/address` that resolves outside the checkout; the FSM spec cites `fsm/fsm.go`, which matches two tracked files | `./le spec citation`; the `pretool-writeedit` spec gates | Each spec's own defect repaired by its owner (the missing section, the two producer paths), then lines 20 and 364 (EPE) and 20 and 461 (FSM) restated to the bare stem, and EPE line 171 repointed to `plan/release/validation/spec-bgp-mp-attribute-first-wire-proof.md`, which owns row 10's wire proof, all in commit A |

Cleared on 2026-11-14: the Review Gate artifact is recorded and checks clean
(Review Gate), and the news fragment is written as
`website/changes/fragments/bgp-unowned-rfc-rows.md`, the name
`newsFragmentPath` (`internal/le/commit/newsfragment.go`) expects: the stem
without `spec-`.

The main thread sets Status back to `in-progress` when it runs the owed commands.

## Task

The BGP child of the rfc-verdict pass (spec-rfc-verdict-fix-bgp; its
"Gap accounting" section survives in git history) left eleven requirement rows that are
canonically `unimplemented` and that no spec owned. Owner decision, 2026-10-11:
"10 give it a spec". This spec took them and two `partial` RFC 9514 rows; the
owner then answered its open questions and split it: "Split into three"
(2026-10-11). Three ISD rows joined on 2026-10-21.

On 2026-11-14 the owner split it again ("split bgp-unowned-rfc-rows"), so the
rows that are implemented and proven close here and every other row stays owed
work in the spec that receives it, with its acceptance criteria, tests and
design text. Nothing was dropped or declined.

### Where every row lives

Row numbers are the numbers the rows had before the first split.

| # | Row id | Lives in | Why |
|---|--------|----------|-----|
| 1 | `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-4` | `plan/release/fixes/spec-bgp-mup-segment-discovery-originator-checks.md` | no code |
| 2 | `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-5` | same | implemented, tagged and recorded, but its interop scenario `bgp-mup-dsd-gobgp` and its EBGP-discard `.ci` do not exist; it moved with those owed proofs |
| 3 | `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-1-2` | `plan/release/fixes/spec-bgp-link-local-only-next-hop.md` (its AC-20, AC-21) | no code; that spec depended on it |
| 4 | `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1` | here | implemented, tagged, recorded |
| 5 | `RFC2545-3-6` | here for the tested half; `plan/release/fixes/spec-rfc2545-readvertise-on-condition-flip.md` (its AC-4) for the gap half | the row is `{partial}`: the include at advertisement is built and proven, the re-advertise when the neighbor table later names the Link-Local is not |
| 6, 7 | `RFC4271-8.2.2-13`, `RFC4271-8.2.2-20` | `plan/post-release/features/spec-bgp-fsm-delayopen-and-automatic-start.md` | first split, 2026-10-11 |
| 8, 11, 12 | `RFC9086-7-2` with 7-1's gap half, `RFC9514-7.2-5`, `RFC9514-7.2-6` | `plan/post-release/features/spec-bgp-epe-peer-segments-and-srv6-peernode.md` | first split, 2026-10-11 |
| 9 | `RFC9830-2.4.2-12` | here | implemented, tagged, recorded, interop scenario exists |
| 10 | `RFC7606-5.1-1` | `plan/release/validation/spec-bgp-mp-attribute-first-wire-proof.md` | implemented, tagged and recorded, but its interop scenarios `bgp-mp-reach-first-frr` and `-bird` and its `.ci` do not exist; it moved with those owed proofs |
| 13, 14, 15 | `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1`, `-3`, `-4` | `plan/release/fixes/spec-bgp-mup-segment-discovery-originator-checks.md` | no code |

`RFC9252-5-2` was never this spec's: `plan/release/fixes/spec-srv6-bestpath-resolvability.md` owns it.

### The rows this spec closes on

| # | Row id | Document, section | What Ze did before | What Ze does now |
|---|--------|-------------------|--------------------|------------------|
| 4 | `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1` | draft-ietf-idr-linklocal-capability §6 | On a capability-77 session, admitted a 32-octet Next Hop field without checking that it is a Global followed by a Link-Local | Treats the UPDATE as withdrawn unless the field is one of the three shapes §6 names (`validateLinkLocalNextHopPair`, `internal/component/bgp/message/rfc7606_linklocal.go`, called from the attribute walk in `rfc7606.go` on a session that negotiated capability 77) |
| 5 | `RFC2545-3-6`, tested half | RFC 2545 §3 | Sent a third-party global next hop on a common subnet in 16 octets, without the Link-Local of that next hop | Writes that router's Link-Local after its global, read from the kernel neighbor table (`linkScope.thirdPartyLinkLocal`, `internal/component/bgp/reactor/link_scope.go`); with no usable entry it sends the global alone and warns |
| 9 | `RFC9830-2.4.2-12` | RFC 9830 §2.4.2 | Never signalled an SRv6 BSID in the type-13 Binding SID sub-TLV and decoded no type 13 with Length 18 | `binding-sid srv6 <ipv6-address>` writes type 13 at Length 18 (`buildBindingSIDSRv6SubTLV`, `internal/component/bgp/plugins/nlri/srpolicy/config.go`) and a received one is decoded by its length (`bindingSIDToJSON`, `attr_json.go`); type 20 stays the default |

### The RFC text, quoted from the source

| # | Quote | Read at |
|---|-------|---------|
| 4 | "If the Next Hop field is malformed, the implementation MUST handle the malformed UPDATE message using the approach of "treat-as- withdraw", as described in section 7.3 of [RFC7606]." (the draft breaks the line after "treat-as-"; the quote keeps its bytes, with the line break read as one space) Section 6 opens: "These procedures apply only when the Link-Local Next Hop Capability has been negotiated for a BGP session." | `rfc/drafts/draft-ietf-idr-linklocal-capability.txt` §6 |
| 5 | "The link-local address shall be included in the Next Hop field if and only if the BGP speaker shares a common subnet with the entity identified by the global IPv6 address carried in the Network Address of Next Hop field and the peer the route is being advertised to." | `rfc/full/rfc2545.txt` §3 |
| 9 | "The value MUST be 18 when a SRv6 BSID is present, 6 when an SR-MPLS BSID is present, or 2 when no BSID is present." | `rfc/full/rfc9830.txt` §2.4.2 |
| 9 (full) | "It is RECOMMENDED that the SRv6 Binding SID sub-TLV, as defined in Section 2.4.3, be used when signaling an SRv6 BSID for an SR Policy CP. The support for the use of this Binding SID sub-TLV for the signaling of an SRv6 BSID is retained primarily for backward compatibility with implementations that followed early draft versions of this document that had not defined the SRv6 Binding SID sub-TLV." | `rfc/full/rfc9830.txt` §2.4.2 (lines 514-522) |

### How FRR and BIRD handle it

| # | FRR | BIRD |
|---|-----|------|
| 4 | `bgpd/bgp_attr.c`, MP_REACH next hop parse, around line 2687: a 32-octet field whose second address is not Link-Local is logged at debug and the Link-Local half dropped | `proto/bgp/packets.c` around line 1432: a second address that is not Link-Local is dropped and the field treated as 16 octets |
| 5 | `bgpd/bgp_updgrp_packet.c` (checkout `cd39d029`, lines 466-560): a third-party global is kept (`bgp_multiaccess_check_v6`); a 32-octet field keeps the Link-Local it received, and a `::` Link-Local is replaced by FRR's OWN `peer->nexthop.v6_local`. No neighbour-table lookup: FRR never derives the third party's Link-Local (A-5) | `bgp_update_next_hop_ip` (`packets.c` around line 1280): a third-party gateway is sent in 16 octets, no Link-Local. BIRD has the same omission |
| 9 | SR Policy lives in `pathd`, not compared | None |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` - scenario naming and the revert-and-red walk
  → Constraint: every new scenario is named (no numeric prefix) and owes a recorded red with the fix reverted and the artifact rebuilt
- [ ] `docs/architecture/bgp/interface-event-reactions.md` - declared by `reactor_iface.go`: BGP's reactions to interface and neighbor changes
  → Constraint: the neighbor table watch is the reactor's own, started from `StartWithContext`, and needs no `interface` block (the two journal rows this spec owns)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/draft-ietf-idr-linklocal-capability.md` - row 6-1
  → Constraint: a wrong Next Hop length stays a session reset (RFC 7606 §7.11, `validateMPReachNextHop`); only a wrong address shape inside a 32-octet field is this row
- [ ] `rfc/short/rfc2545.md` - row 3-6
  → Constraint: the row is `{partial}` and its annotation names `link_scope.go::thirdPartyLinkLocal` as the boundary: tested is the include at advertisement, gap is the re-advertise when the table later names the Link-Local. A received complete 32-octet pair relayed unchanged is not in it (main-thread ruling R58(b), `plan/handover/rfc-verdict-test-fix-pass/RULINGS.md`)
- [ ] `rfc/short/rfc9830.md` - row 2.4.2-12
  → Decision: the 2026-10-11 decline was reversed by the owner on 2026-10-18 (`plan/records/rfc-gap-difficulty-2026-10-18.md`, "Owner decisions", #7), so the row carries no annotation and is tagged

**Key insights:**
- Three rows close here: 4, 9, and the tested half of 5.
- Rows 3 and 5 shared the "Link-Local needs an interface or a neighbor" question; row 5 answered the neighbor half, row 3 (the interface) is the link-local spec's.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/message/rfc7606_linklocal.go` - `validateLinkLocalNextHopPair` returns treat-as-withdraw for a 32-octet field that `linkLocalNextHopPairWellFormed` refuses; it judges no length
- [ ] `internal/component/bgp/reactor/link_scope.go` - `linkScope` holds the connected prefixes and a neighbor view; `linkLocalNextHop` answers the speaker's own Link-Local for the speaker's global and `thirdPartyLinkLocal` for another router's; `thirdPartyLinkLocal` warns once per peer for an unreadable table and rate-limited for a table that names no router
- [ ] `internal/component/bgp/reactor/reactor_iface.go` - `startNeighborWatch` and `neighborWatchWorker` keep every live peer's neighbor view current through `network.WatchNeighbors`
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/config.go` - `buildBindingSIDSRv6SubTLV` writes type 13 with Length 18; `parseBindingSIDSRv6` refuses an IPv4, IPv4-mapped or zoned address
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/attr_json.go` - `bindingSIDToJSON` reads the Binding SID by its length and marks any other length malformed

**Behavior to preserve:**
- A wrong Next Hop LENGTH stays a session reset (`validateMPReachNextHop`, RFC 7606 §7.11).
- A received complete Global + Link-Local pair relayed unchanged.
- The RECOMMENDED type-20 SRv6 BSID encoding stays the default.

**Behavior to change:** rows 4, 5 (tested half) and 9, as the ACs state. Done.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Receive: UPDATE wire bytes on a capability-77 session (row 4).
- Send: route origination, reflection or relay with a third-party next hop (row 5).
- Config: `binding-sid srv6` on an SR Policy route; `ze bgp decode` for a received one (row 9).

### Transformation Path
1. Row 4: the RFC 7606 attribute walk in `internal/component/bgp/message/` calls `validateLinkLocalNextHopPair` on a session that negotiated capability 77.
2. Row 5: `applyLinkLocalNextHop` settles the next-hop form per peer from the link-scope snapshot; the neighbor watch refreshes the snapshot.
3. Row 9: the SR Policy config route parser builds the sub-TLV; the attribute JSON decoder reads it.

### Design resolutions carried for the rows kept

| Finding | Design, and what landed |
|---------|-------------------------|
| I3, row 5 neighbour states | REACHABLE, STALE, DELAY, PROBE and PERMANENT are valid bindings; INCOMPLETE and FAILED are not (`TestRFC2545NeighbourStateClasses`). The Link-Local is re-read on a neighbour change, not only at establishment and reload (`TestRFC2545NeighbourChangeRefreshes`). The design named a neighbour-changed bus event from the netlink monitor; what landed is a neighbor table watch the netlink backend registers in `internal/core/network` and the reactor starts itself, because the monitor runs only with an `interface` block (the first owned journal row). The re-advertise of affected prefixes on a changed Link-Local is not built and is owned by `plan/release/fixes/spec-rfc2545-readvertise-on-condition-flip.md` |
| I5, row 5 with no backend | With no readable neighbor table Ze sends 16 octets and warns once per peer naming the cause (`warnNeighborTableUnread`), and a `ze doctor` check reports the same (`TestDoctorNeighbourTableUnreadable`, `internal/component/bgp/config/neighbor_table_doctor_test.go`) |
| I2, ownership | Row 4 (`-6-1`) is owned here. `plan/release/fixes/spec-rfc2545-next-hop-truncated-to-sixteen-bytes.md` owns relay truncation of a received 32-octet pair, which R58(b) keeps out of row 5 |

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor to the kernel neighbor table (row 5) | through the `internal/core/network` reader and watch the interface component's backend fills, never an import of `internal/component/iface` from the reactor | Yes: `link_scope.go` and `reactor_iface.go` import `internal/core/network` only |
| Engine to RIB plugin (row 4) | received UPDATE with treat-as-withdraw already decided | Yes: `validateLinkLocalNextHopPair` returns an `RFC7606ValidationResult` with `RFC7606ActionTreatAsWithdraw` to the attribute walk (`rfc7606.go` line 548), the same result every other RFC 7606 check returns; `test/plugin/linklocal-capability-malformed-nexthop-withdraw.ci` observes the withdrawal on a second connection |

### Integration Points
- The RFC 7606 attribute walk - row 4.
- `linkScope` and the neighbor watch - row 5.
- The SR Policy config parser and attribute decoder - row 9.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | row 4 is called from the attribute walk (`rfc7606.go` line 548); row 5 from `linkScope.linkLocalNextHop`, which `applyLinkLocalNextHop` and `Peer.linkLocalNextHopFor` call on the per-peer facts; row 9 from `buildTunnelEncap` and `subTLVToJSON` |
| No unintended coupling (components stay isolated) | Yes | the reactor reads the neighbor table through `internal/core/network` (imports of `link_scope.go`, `reactor_iface.go`), so it gained no dependency on `internal/component/iface`, which the design had accepted |
| No duplicated functionality (extends existing, does not recreate) | Yes | row 5 extends `linkScope.linkLocalNextHop` with the `nextHopRouterThirdParty` arm; row 9 adds a Length-18 arm to the existing type-13 builder and decoder |
| Zero-copy preserved where applicable (refs, not copies) | Yes | row 4 reads the two addresses from `data[4:36]` in place and allocates only on the malformed result |
| Registration over hardcoding, outbound | Yes | the interface component registers the neighbor reader and watch (`network.RegisterNeighborTable`, `internal/component/iface/register.go` line 167); the reactor calls `network.WatchNeighbors` and names no backend |
| Registration over hardcoding, inbound | Yes | row 9 is reached through the SR Policy plugin's registered config route parser and its registered attribute-23 renderer (`TunnelEncapToJSON`); no engine code names the form |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-5 | FRR's behavior for row 5 was not read in this pass | budget | The FRR comparison for row 5 is missing from the design | Read `bgpd/bgp_updgrp_packet.c` next-hop rewrite before design approval | confirmed (2026-10-11): read, recorded in the FRR/BIRD table |

A-1 and A-4 moved with the MUP rows, A-2 with row 3, A-3 and A-6 with row 10.

- A-5 → Decision: FRR keeps a third-party global, keeps a received Link-Local in a 32-octet field, and substitutes its OWN Link-Local for `::`; it never derives the third party's Link-Local. BIRD sends 16 octets. Neither does what the owner chose for row 5 ("Fix it"), so Ze has no reference implementation to copy and the AC is written from RFC 2545 §3 alone
- A-5 → Constraint: the kernel neighbour table maps an address to a MAC and a device, not a global address to a Link-Local. The lookup is: the third-party global's entry on the shared interface gives its MAC; a `fe80::/10` entry on the same device with the same MAC is that router's Link-Local. No entry, an INCOMPLETE or FAILED entry, or two candidate Link-Locals for one MAC means no Link-Local is known. A received complete pair relayed unchanged keeps its Link-Local (R58(b)); the lookup applies only where Ze writes the next hop

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-5 | Row 4 withdraws routes from a peer sending two Link-Locals, a form the draft says is "observed in deployment" (BIRD) | interop with BIRD loses routes | The draft says receivers SHOULD use the second address for that form, so it is not malformed; `linkLocalNextHopPairWellFormed` accepts it |
| R-6 | Row 9's ledger form is refused by `./le rfc check` | gate exit 2 | Moot since 2026-10-18: the row is implemented and tagged, with no annotation |

R-1 and R-2 moved with row 10, R-3 with row 1, R-4 with row 3, R-7 and R-8 with the EPE rows.

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Row 4: routes withdrawn that should have been kept. Row 5: a third-party next hop sent with a wrong Link-Local. Row 9: an SR Policy BSID a peer cannot read |
| How is it reverted? | Each row landed as its own commits, revertible alone |
| Who else touches this path? | `plan/release/fixes/spec-bgp-link-local-only-next-hop.md` and `plan/release/fixes/spec-rfc2545-readvertise-on-condition-flip.md` both extend `link_scope.go`; `plan/release/fixes/spec-rfc2545-next-hop-truncated-to-sixteen-bytes.md` owns relay truncation of a received pair |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE with a malformed 32-octet next hop on a capability-77 session | → | treat-as-withdraw | `test/plugin/linklocal-capability-malformed-nexthop-withdraw.ci` |
| Static route with a third-party global next hop on the common subnet | → | `link_scope.go` third-party Link-Local | `test/plugin/rfc2545-third-party-linklocal.ci` |
| `binding-sid srv6` in an SR Policy route | → | `buildBindingSIDSRv6SubTLV`, `bindingSIDToJSON` | `test/encode/bgp-srpolicy-bsid-srv6-encode.ci`, `test/decode/bgp-srpolicy-bsid-srv6-decode.ci`, `test/parse/srpolicy-binding-sid-srv6.ci` |

## Acceptance Criteria

AC numbers are unchanged. AC-1, AC-2 and AC-9 moved with the MUP rows, AC-3
with row 3 and AC-7 with row 10.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-4 (row 4) | On a capability-77 session, a 32-octet Next Hop field that is not a Global (or `::`) followed by a Link-Local, and not the two-Link-Local form §6 tells receivers to accept | The UPDATE is handled as treat-as-withdraw (RFC 7606 §7.3); a well-formed pair and the two-Link-Local form are accepted; a wrong length still resets the session |
| AC-5 (row 5) | Ze writes a next hop whose global address is another router on the subnet Ze shares with the recipient (owner: "Fix it") | The Next Hop field is 32 octets and its second address is that router's Link-Local, read from the Linux neighbour table on the send path (A-5: same device, same MAC as the global's entry, entry in REACHABLE, STALE, DELAY, PROBE or PERMANENT). With no usable neighbour entry (none, INCOMPLETE, FAILED, or two candidate Link-Locals) the field is 16 octets, as before, and a rate-limited Warn names the next hop and the reason. With no readable neighbour table the field is 16 octets, one Warn per peer names the cause and the fix, and `ze doctor` reports the same. A neighbour change for a cached next hop refreshes the cache. A received complete pair relayed unchanged keeps its Link-Local. The neighbour read never runs per UPDATE. The one clause not met here, "a changed Link-Local re-advertises the affected prefixes", is AC-4 of `plan/release/fixes/spec-rfc2545-readvertise-on-condition-flip.md` (Work Not Done) |
| AC-6 (row 9) | The type-13 SRv6 BSID form (owner: OUT 2026-10-11, IN 2026-10-18) | `binding-sid srv6 <ipv6-address>` writes the type-13 Binding SID sub-TLV at Length 18 (`config.go::buildBindingSIDSRv6SubTLV`), `srv6-binding-sid` (type 20) stays the default, `ze bgp decode` reads a received type 13 by its length (`attr_json.go::bindingSIDToJSON`); the row carries no annotation and is tagged by `rfc9830_bsid_srv6_test.go`, with `test/encode/bgp-srpolicy-bsid-srv6-encode.ci`, `test/decode/bgp-srpolicy-bsid-srv6-decode.ci`, `test/parse/srpolicy-binding-sid-srv6.ci` and the `bgp-sr-policy-gobgp` interop scenario |
| AC-8 (ledger) | Each row once its AC is met | Rows 4 and 9 carry no annotation, tags in both polarities and a discrimination record. Row 5 is `{partial}` with its tested half tagged and recorded and its gap half owned by the readvertise spec. `./le rfc check` exits clean for the three stems `draft-ietf-idr-linklocal-capability`, `rfc2545`, `rfc9830` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Advertises a route through a third router on the segment it shares with FRR | config -> next-hop facts -> wire with the third router's Link-Local | `bgp-rfc2545-third-party-linklocal-frr` |
| 2 | Sends an SR Policy with a type-13 SRv6 Binding SID to GoBGP | config -> sub-TLV -> wire | `bgp-sr-policy-gobgp` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLinkLocalCapabilityMalformedThirtyTwoOctet`, `TestLinkLocalCapabilityWellFormedThirtyTwoOctet` | `internal/component/bgp/message/draft_ietf_idr_linklocal_capability_malformed_test.go` | AC-4, including the two-Link-Local accepted form | landed, recorded |
| `TestLinkLocalCapabilityWrongLengthStillResets`, `TestLinkLocalCapabilityNotNegotiatedJudgesLengthOnly` | same | AC-4: a wrong Next Hop length on a capability-77 session stays a session reset, and without the capability only the length is judged | landed |
| `TestRFC2545ReceivedPairFirstAddressValidated`, `TestRFC2545ReceivedPairSecondAddressValidated`, `TestRFC2545EffectivePairPolicyAndMixedSibling` | `internal/component/bgp/reactor/rfc2545_forward_subnet_test.go` | AC-4 on the receiving session | landed, recorded |
| `TestRFC2545ThirdPartyLinkLocalAnnounce`, `TestRFC2545ThirdPartyLinkLocalForward` (the plan named `TestRFC2545ThirdPartyLinkLocal`) | `internal/component/bgp/reactor/rfc2545_third_party_link_local_test.go` | AC-5: neighbour entry found (32 octets) on the announce and forwarding rails | landed, recorded |
| `TestRFC2545NeighbourStateClasses` | same | AC-5: the valid and invalid neighbour state classes | landed |
| `TestRFC2545ThirdPartyLinkLocalMisses`, `TestRFC2545ThirdPartyLinkLocalScope` (the plan named `TestRFC2545NoIfaceBackendSixteenOctets`) | same | AC-5: an absent, ambiguous or unreadable table yields 16 octets (the unreadable table is the `no-reader` case of `TestRFC2545ThirdPartyLinkLocalMisses`). These tests assert no log line | landed |
| `TestThirdPartyMissWarningTellsTheTruth` | `internal/component/bgp/reactor/neighbor_watch_test.go` | AC-5: the rate-limited Warn names the next hop and the reason, and tells a watched table with no entry from a table nobody watches | landed |
| `TestNoNeighbourReaderWarnsOncePerPeer` | `internal/component/bgp/reactor/neighbor_table_unread_test.go` | AC-5: an unreadable table warns once per peer | landed |
| `TestRFC2545NeighbourChangeRefreshes` (the plan named `TestRFC2545NeighbourChangeRefreshesAndReadvertises`) | same | AC-5: a neighbour change refreshes the cache. The re-advertise half moved (Work Not Done) | landed |
| `TestNeighbourWatchNotesPairingChanges` and the other `TestNeighbourWatch*` (the plan named `TestNeighbourChangedEventEmitted`) | `internal/plugins/iface/netlink/neighbor_watch_linux_test.go` | AC-5: the watch signals on a pairing change; a watch replaced the planned bus event (Deviations) | landed |
| `TestDoctorNeighbourTableUnreadable`, `TestDoctorNeighbourTableCheckRegistered` | `internal/component/bgp/config/neighbor_table_doctor_test.go` | AC-5: an unreadable neighbour table is reported with the fix | landed |
| `TestRFC9830BindingSIDSRv6Encode`, `TestRFC9830BindingSIDSRv6Decode` | `internal/component/bgp/plugins/nlri/srpolicy/rfc9830_bsid_srv6_test.go` | AC-6 both polarities | landed, recorded |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Next Hop length on a capability-77 session | 16 or 32 | 32 | 15 (session reset, unchanged) | 33 (session reset, unchanged) |
| Binding SID sub-TLV Length | 2, 6 or 18 | 18 | none | 20 (shown malformed, `TestRFC9830BindingSIDSRv6Decode`) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `linklocal-capability-malformed-nexthop-withdraw` | `test/plugin/` | AC-4 | landed |
| `rfc2545-third-party-linklocal` | `test/plugin/` | AC-5 | landed |
| `bgp-srpolicy-bsid-srv6-encode`, `bgp-srpolicy-bsid-srv6-decode`, `srpolicy-binding-sid-srv6` | `test/encode/`, `test/decode/`, `test/parse/` | AC-6 | landed |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-rfc2545-third-party-linklocal-frr` (the plan extended `bgp-rfc2545-linklocal-nexthop-frr`; a scenario of its own landed) | `test/interop/scenarios/` | FRR | Ze advertises a route whose global next hop is a third router on the shared segment; FRR's received next hop carries that router's Link-Local (`checkRFC2545ThirdPartyLinkLocal`, tagged and recorded) | landed |
| `bgp-sr-policy-gobgp` | same | GoBGP | GoBGP prints the type-13 Binding SID Ze sent with `binding-sid srv6` | landed |

Row 4's malformed form cannot be produced by a conforming peer; its proof is the ze-peer functional test.

## Files to Modify
- `internal/component/bgp/message/rfc7606_linklocal.go` - row 4
- `internal/component/bgp/reactor/link_scope.go`, `internal/component/bgp/reactor/reactor_iface.go` - row 5
- `internal/component/bgp/plugins/nlri/srpolicy/config.go`, `internal/component/bgp/plugins/nlri/srpolicy/attr_json.go` - row 9
- `rfc/short/draft-ietf-idr-linklocal-capability.md`, `rfc/short/rfc2545.md`, `rfc/short/rfc9830.md` - AC-8

These are the producers this closure pass read. The reviewer derives the whole
landed change set from the history of these files and of the tests above.

## Files to Create
- `test/plugin/linklocal-capability-malformed-nexthop-withdraw.ci`, `test/plugin/rfc2545-third-party-linklocal.ci` - landed
- `test/interop/scenarios/bgp-rfc2545-third-party-linklocal-frr/` and `internal/le/interoplab/bgp/check_rfc2545_third_party.go` - landed

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | row 9: the `binding-sid srv6` form of the SR Policy route; `test/parse/srpolicy-binding-sid-srv6.ci` |
| YANG validation constraints | N-A | none new |
| YANG custom validators | N-A | the `session link-local` check moved with row 3 |
| CLI commands/flags | N-A | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | N-A | no new RPC; behavior `.ci` tests are listed above |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | row 5 needs a readable neighbour table; the check is proven by `TestDoctorNeighbourTableUnreadable` and `TestDoctorNeighbourTableCheckRegistered` |
| Prometheus counters/metrics | No | treat-as-withdraw counting already exists in the RFC 7606 path |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/bgp-protocol.md`, whose page is `docs/features/bgp-protocol.md`: the SR Policy keyword table (lines 44 to 58) and the next-hop wire form row (line 315) already carry the three behaviors. No scope or level moved |
| 2 | Config syntax changed? | Yes | `binding-sid srv6 <ipv6-address>`: `docs/features/bgp-protocol.md` lines 45 and 49 to 53 (the form, when to use it, the refused addresses), read against `parseBindingSIDSRv6`; the keyword lists in `docs/architecture/wire/nlri.md` line 712 and `docs/architecture/api/update-syntax.md` line 851 name `binding-sid` |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | no plugin gained a surface |
| 6 | Has a user guide page? | Yes | the third-party Link-Local is on `docs/features/bgp-protocol.md` line 315 (rewritten in review round 1) and `docs/architecture/update-building.md` lines 304 to 319, which also name the `bgp-neighbor-table` doctor check |
| 7 | Wire format changed? | Yes | type 13 at Length 18: `docs/features/bgp-protocol.md` lines 45 and 58 (written and decoded forms); the capability-77 receive check: `docs/architecture/wire/capabilities.md` lines 432 to 438, anchored to `validateLinkLocalNextHopPair` |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | the three `rfc/short/` files above and the generated `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` has no row for a third-party Link-Local or for the Binding SID forms (`grep -n -i "link-local\|sr policy"`, 2026-11-14: only the "SR Policy" row at line 49, already Yes for Ze) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/bgp/interface-event-reactions.md` (the neighbor watch) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | the neighbor table reader and watch registered in `internal/core/network`, and the neighbour-table doctor check. The registration is on `docs/architecture/bgp/interface-event-reactions.md` (lines 84 to 97, anchored to `RegisterNeighborTable` and `WatchNeighbors`) and `docs/features/interfaces.md` (lines 1268 to 1271); the doctor check is named with its code on `docs/architecture/update-building.md` line 319. `docs/architecture/doctor-and-health-checks.md` describes the mechanism and lists no single check by name |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | re-derive with `./le spec citation anchors spec plan/release/fixes/spec-bgp-unowned-rfc-rows.md`. Declared by the files above: `docs/architecture/core-design.md` (`link_scope.go`: unaffected, no change to the core and registration design), `docs/architecture/wire/capabilities.md` (`rfc7606_linklocal.go`: lines 432 to 438 state the check and carry its anchor), `docs/architecture/bgp/interface-event-reactions.md` (`reactor_iface.go`: affected, item 12, done in review round 1), `docs/architecture/config/syntax.md` (declared by the SR Policy route parser: it holds no `binding-sid` text, `grep -rn -i "binding-sid" docs`, so nothing there went stale; the form is documented on the feature page, item 2). `./le doc check all`, 2026-11-14: the source-anchor stage passes; the run as a whole is red on the documentation-drift stage for the `delete data key` command (live registry against the wiki), which another session's uncommitted work on the data commands produces and this spec does not touch |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the SR Policy keyword table of `docs/features/bgp-protocol.md` (lines 44 to 47) lists `binding-sid srv6`, `binding-sid mpls` and `binding-sid null` beside `srv6-binding-sid` |

## Implementation Steps

1. **Phase: row 4** -- the malformed 32-octet check. Landed.
2. **Phase: row 5** -- the third-party Link-Local from the neighbor table, the watch, the Warns and the doctor check. Landed, except the re-advertise (Work Not Done).
3. **Phase: row 9** -- `binding-sid srv6` and the type-13 decode. Landed.
4. **Phase: ledger** -- AC-8, discrimination records, `./le rfc check` clean for the three stems.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | Row 4 does not reclassify a wrong length; the two-Link-Local form is accepted |
| Correctness | Row 5 never substitutes the speaker's own Link-Local for a third party's |
| Rule: rfc-compliance | Each implementing function quotes its RFC sentence; each caller names the section |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No `{gap}` left on rows 4 and 9; row 5 `{partial}` with its gap owned | `grep -n` each id in its `rfc/short/` file |
| Both polarities tagged | `grep -rn "RFC requirement: <id>" internal/ test/` |
| Ledger clean | `./le rfc check` exit status for the three stems |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Row 4 parses peer-supplied bytes; bounds checked before reading the second address |
| Resource exhaustion | Row 5's neighbour read is cached in the snapshot, never per UPDATE |
| Error leakage | Warn logs name the peer and next hop, rate-limited |

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

## Owner decisions

| Question | Owner's words | What it means here |
|----------|---------------|--------------------|
| Row 5: fix the third-party Link-Local, or record a deviation? (2026-10-11) | "Fix it" | AC-5: read the third party's Link-Local from the Linux neighbour table on the send path (A-5) |
| Row 9: support the type-13 SRv6 BSID form, or decline it? | OUT on 2026-10-11; reversed to IN on 2026-10-18 (`plan/records/rfc-gap-difficulty-2026-10-18.md`, "Owner decisions", #7) | AC-6: implemented as `binding-sid srv6`, explicit operator choice, type 20 stays the default |
| Rows 3 and 4: owned here or by `plan/release/fixes/spec-bgp-link-local-only-next-hop.md`? (2026-10-11) | placed in this spec | Row 4 stayed and landed; row 3 moved to that spec in the 2026-11-14 split |
| Approve the design after the round-5 design review? (2026-10-11) | "Approve" | Status moved from `design` to `ready` |
| One spec or several? (2026-10-11) | "Split into three" | DelayOpen and Event 3, and the EPE rows, went to their two specs |
| Close the proven rows and move the rest? (2026-11-14) | "split bgp-unowned-rfc-rows" | The table "Where every row lives" |

The owner decisions on rows 1 and 13 to 15 are carried in the MUP spec, and
those on row 3's send side in the link-local spec.

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Row 5 matches the neighbour table by device and MAC | Copy FRR (own Link-Local) or BIRD (16 octets) | Owner: "Fix it"; neither reference derives the third party's Link-Local, and the kernel maps address to MAC, not global to Link-Local (A-5) |
| Row 4 treats a malformed 32-octet field on a capability-77 session as withdrawn (review N4) | Drop the Link-Local half and keep the route, as FRR and BIRD both do | The draft says "the implementation MUST handle the malformed UPDATE message using the approach of "treat-as- withdraw"" (§6); both references diverge from it, so Ze follows the draft, and the `linklocal-capability-malformed-nexthop-withdraw` test comment records the divergence |
| Row 9 is implemented as an explicit operator form | Keep the 2026-10-11 decline | Owner, 2026-10-18: "implement the type-13 SRv6 form"; type 20 stays the default because the RFC recommends it |
| Rows 2 and 10 moved although they are recorded | Close them here on unit proof | A wire-visible change owes an interop test (`ai/rules/interop-and-goal-validation.md`) and the scenarios this spec named for them were never built. On the ledger both rows read as proven: `RFC7606-5.1-1` and `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-5` carry no annotation in `rfc/short/`, and their unit tags and discrimination records stand. What is owed is the interop and `.ci` proof, in `plan/release/validation/spec-bgp-mp-attribute-first-wire-proof.md` (row 10) and `plan/release/fixes/spec-bgp-mup-segment-discovery-originator-checks.md` (row 2). `docs/contributing/rfc-conformance-gates.md` has no annotation for "unit-proven, interop owed": `{gap}` and `{partial}` both say the requirement is unmet, which is false here, so the rows stay as they are |

## Known Limitations
- Row 5: a route already sent in 16 octets is not re-advertised when the neighbor table later names the Link-Local. Owned by `plan/release/fixes/spec-rfc2545-readvertise-on-condition-flip.md`.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `subscriber-waits-on-an-event-another-feature-produces.md` | 2026-10-30 | AC-5 neighbor refresh needs the netlink monitor, which runs only with an `interface` block |
| `subscriber-waits-on-an-event-another-feature-produces.md` | 2026-10-30 | AC-5 connected subnets and late listeners need address events, which the monitor emits only with an `interface` block |

Both rows read `fixed:` in the journal.

## RFC Documentation (Scope: protocol)

`// RFC NNNN Section X.Y: "<quoted requirement>"` stands above the enforcing
code of each row: `validateLinkLocalNextHopPair`, `linkScope.linkLocalNextHop`
and `buildBindingSIDSRv6SubTLV` each carry their sentence.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-4, AC-5 (but for the moved clause), AC-6 and AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le test unit pkg path` for the packages under Pre-Commit Verification, `./le test functional plugin` for the two `.ci` tests, and `./le test integration interop` for the two scenarios
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
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-unowned-rfc-rows.md` only, in the same `./le commit create` script

---

## Implementation Summary

### What Was Implemented
- Row 4: `validateLinkLocalNextHopPair` and `linkLocalNextHopPairWellFormed` (`internal/component/bgp/message/rfc7606_linklocal.go`).
- Row 5, tested half: `linkScope.thirdPartyLinkLocal` and the third-party arm of `linkScope.linkLocalNextHop` (`internal/component/bgp/reactor/link_scope.go`), the neighbor watch (`startNeighborWatch`, `neighborWatchWorker`, `internal/component/bgp/reactor/reactor_iface.go`), the neighbor-table doctor check, the scenario `bgp-rfc2545-third-party-linklocal-frr`.
- Row 9: `buildBindingSIDSRv6SubTLV`, `parseBindingSIDSRv6` (`config.go`) and the Length-18 arm of `bindingSIDToJSON` (`attr_json.go`).

### Bugs Found/Fixed
- With no `interface` block no netlink monitor ran, so BGP never re-read the neighbor table or the connected subnets (the two owned journal rows). Fixed by the reactor's own watches; `TestRFC2545NeighbourChangeRefreshes` and the `TestNeighbourWatch*` tests cover the neighbor half.
- `srpolicy.DecodeNLRIHex` read the wire form as the body (`plan/journal/declared-format-contradicts-payload.md`, 2026-10-18, walked into from this spec; its row reads `fixed:`).

### Documentation Updates
- `docs/features/bgp-protocol.md`: the next-hop wire form row says a third-party next hop carries its router's Link-Local from the neighbor table (review round 1).
- `docs/architecture/bgp/interface-event-reactions.md`: the neighbor table watch (review round 1, checklist item 12).
- Closure pass, 2026-11-14: every Yes of the Documentation Update Checklist was checked against its page and the checklist rows now name the page and lines. No page was found stale beyond the two that review round 1 repaired. `./le doc check all` ran: its source-anchor stage passes, and its one red stage (documentation drift on the `delete data key` command) belongs to another session's uncommitted work.

### Deviations from Plan
- Row 5 reads the neighbor table through a reader and a watch registered in `internal/core/network`, not through `iface.ListNeighbors` and a neighbour-changed bus event, because the monitor that would emit the event runs only with an `interface` block. The reactor gained no import of the interface component.
- Row 5's interop proof is a scenario of its own, `bgp-rfc2545-third-party-linklocal-frr`, not an extension of `bgp-rfc2545-linklocal-nexthop-frr`.
- Row 9 was implemented, where the first design declined it (owner, 2026-10-18).
- Several unit tests landed under names other than the planned ones; the TDD table maps each.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The spec stayed `in-progress` for two weeks holding five rows with no code beside four proven ones, with no closure sections | The proven rows could close and the rest are separate mechanisms (MUP originator, link-local interface, re-advertise) | Read-only audit, 2026-11-14 | Split on the owner's order; each row has one owning spec |
| approach | Rows 2 and 10 were recorded in the ledger while the interop scenarios and `.ci` tests the spec named for them were never written | Unit tags and records existed; the named daemon and interop proofs did not | `ls test/plugin`, `ls test/interop/scenarios`, 2026-11-14 | The rows moved with their owed proofs to the MUP spec and to `plan/release/validation/spec-bgp-mp-attribute-first-wire-proof.md` |

## Implementation Audit

Evidence is from source reading on 2026-11-14. The test runs the main thread
observed that day are under Pre-Commit Verification, with the runs still owed.

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Row 4 fixed | Done | `internal/component/bgp/message/rfc7606_linklocal.go`, `validateLinkLocalNextHopPair`, called at `rfc7606.go` line 548 | |
| Row 5 fixed | Partial | `internal/component/bgp/reactor/link_scope.go`, `thirdPartyLinkLocal` | the re-advertise is Work Not Done, on the owner's split |
| Row 9 implemented | Done | `internal/component/bgp/plugins/nlri/srpolicy/config.go`, `buildBindingSIDSRv6SubTLV`; `attr_json.go`, `bindingSIDToJSON` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-4 | Done | `TestLinkLocalCapabilityMalformedThirtyTwoOctet`, `TestLinkLocalCapabilityWellFormedThirtyTwoOctet`, `test/plugin/linklocal-capability-malformed-nexthop-withdraw.ci` | record `rfc/discrimination/draft-ietf-idr-linklocal-capability/DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1.json` |
| AC-5 | Partial | `TestRFC2545ThirdPartyLinkLocalAnnounce`, `TestRFC2545ThirdPartyLinkLocalForward`, `test/plugin/rfc2545-third-party-linklocal.ci`, `bgp-rfc2545-third-party-linklocal-frr` | record `rfc/discrimination/rfc2545/RFC2545-3-6.json`; the re-advertise clause moved |
| AC-6 | Done | `TestRFC9830BindingSIDSRv6Encode`, `TestRFC9830BindingSIDSRv6Decode`, the three `.ci` tests, `bgp-sr-policy-gobgp` | record `rfc/discrimination/rfc9830/RFC9830-2.4.2-12.json` |
| AC-8 | owed run | `./le rfc check` | rows read on 2026-11-14: `-6-1` and `RFC9830-2.4.2-12` carry no annotation, `RFC2545-3-6` is `{partial}` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| every row of the Unit, Functional and Interop tables above | Done | as listed there | each was found by name on 2026-11-14; the planned names that differ are mapped in the tables |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/message/rfc7606_linklocal.go` | Done | the plan named `rfc7606.go`; the check has a file of its own |
| `internal/component/bgp/reactor/link_scope.go` | Done | |
| `internal/component/bgp/reactor/reactor_iface.go` | Done | the plan named the netlink monitor and the iface event registration (Deviations) |
| `internal/le/interoplab/bgp/check_rfc2545_third_party.go` | Done | the plan extended `check_rfc.go` (Deviations) |

### Audit Summary
- **Total items:** 3 requirements, 4 criteria
- **Done:** rows 4 and 9, AC-4, AC-6
- **Partial:** row 5 and AC-5, by the owner's split of 2026-11-14 ("split bgp-unowned-rfc-rows")
- **Skipped:** none
- **Changed:** the four Deviations above

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A malformed 32-octet next hop on a capability-77 session is withdrawn, not accepted | functional | `./le test bgp plugin linklocal-capability-malformed-nexthop-withdraw`: pass 1/1 (main thread, 2026-11-14). The test sends the malformed pair on one connection and expects the withdrawal of `2001:db8:5701::/48` on the second (`expect=bgp:conn=2:seq=2`), and rejects the malformed pair being relayed. No conforming peer can send the form, so no interop scenario exists; the recorded break is `rfc/discrimination/draft-ietf-idr-linklocal-capability/DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1.json` |
| A third-party next hop on the common subnet carries its router's Link-Local | functional, in Ze's QEMU guest | `./le test qemu ci test test/plugin/rfc2545-third-party-linklocal.ci`: pass 1/1 (main thread, 2026-11-14). The peer expects the 32-octet next hop `2001:db8:2545::9` then `fe80::9` for the route through the neighbor the table names, and the 16-octet form for the router it does not name |
| The same, against another implementation | interop | `bgp-rfc2545-third-party-linklocal-frr`, judged by `checkRFC2545ThirdPartyLinkLocal` (FRR decodes the third router's global then its Link-Local, and installs the route through it), with its recorded break in `rfc/discrimination/rfc2545/RFC2545-3-6.json`. Not yet run in this closure: owed on the Linux Docker lab |
| An SRv6 BSID can be signalled in the type-13 sub-TLV at Length 18 and read back | functional | `./le test bgp encode bgp-srpolicy-bsid-srv6-encode`, `./le test bgp decode bgp-srpolicy-bsid-srv6-decode`, `./le test bgp parse srpolicy-binding-sid-srv6`: each pass 1/1 (main thread, 2026-11-14). The encode test asserts the UPDATE hex, the decode test the JSON with `srv6-sid`, the parse test the refusal of an IPv4 address |
| The same, against another implementation | interop | `bgp-sr-policy-gobgp`: GoBGP prints the type-13 Binding SID. Not yet run in this closure: owed on the Linux Docker lab |

The two interop rows stay open until their runs are in; the closure commits
wait on them.

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Rows 1, 13, 14, 15: the MUP DSD and ISD originator and Prefix-SID checks (AC-1, AC-9) | no code; a mechanism of its own (the IS-IS locator-originator resolver) | `plan/release/fixes/spec-bgp-mup-segment-discovery-originator-checks.md` |
| Row 2's owed proofs: `bgp-mup-dsd-gobgp` and `mup-dsd-ebgp-prefix-sid-discarded-withdraw.ci` | never written | `plan/release/fixes/spec-bgp-mup-segment-discovery-originator-checks.md` |
| Row 3: a Link-Local next hop tied to the session's interface (AC-3) | no code; the link-local spec builds on it | `plan/release/fixes/spec-bgp-link-local-only-next-hop.md` |
| Row 5, gap half: re-advertise when the neighbor table later names the Link-Local | needs the per-peer re-advertise that spec owns | `plan/release/fixes/spec-rfc2545-readvertise-on-condition-flip.md` |
| Row 10's owed proofs: `bgp-mp-reach-first-frr`, `bgp-mp-reach-first-bird`, `rfc7606-mp-reach-first.ci`, the every-builder byte comparison | never written | `plan/release/validation/spec-bgp-mp-attribute-first-wire-proof.md` |

## Review Gate

Two rounds ran, by reviewers independent of the agent that narrowed this spec.
Round 1's findings are fixed below, in commits 0af5860697 (the two pages) and
d7aa6f2f93 (the `.ci` comment). Round 2 was clean and the main thread recorded
the artifact.

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-unowned-rfc-rows-5c1d37ea-a4ea-4b06-b7b3-2b3b45cd9c24.md`, recorded by the main thread at 2026-11-14T15:12:09Z: verdict clean, rounds 2, reviewers `ze-review` and `ze-review-spec`, 11 files pinned by hash |
| `./le spec review check` | run on 2026-11-14 over the same 11 files: `review_gate: OK (11 code files, clean, hashes match ...)` |
| Rounds | 2 |
| Round 1 scope | the whole diff of the kept rows (4, 5 tested half, 9) |
| Reviewer lenses used | protocol correctness against the RFC text, test proves claim, spec match |
| Round 1 verdict | 0 BLOCKER, 2 ISSUE, 4 NOTE |
| Round 2 verdict | 0 BLOCKER, 0 ISSUE: clean |

The 11 files the artifact pins: `rfc7606_linklocal.go` and
`draft_ietf_idr_linklocal_capability_malformed_test.go` (message);
`link_scope.go`, `link_scope_neighbors.go` and
`rfc2545_third_party_link_local_test.go` (reactor); `config.go`, `attr_json.go`
and `rfc9830_bsid_srv6_test.go` (srpolicy); `check_rfc2545_third_party.go`
(interop lab); and the two `test/plugin/` `.ci` tests of rows 4 and 5.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The page said a third-party global next hop goes alone because Ze holds no source for that router's Link-Local; the code reads it from the neighbor table | `docs/features/bgp-protocol.md`, the "next-hop wire form matches its length octet" row | The cell rewritten from `linkScope.linkLocalNextHop` and `thirdPartyLinkLocal`: the neighbor table is the source, the global goes alone on a miss with a warning, and the re-advertisement is named as not built; a source anchor added |
| 2 | ISSUE | The `// Design:` page of `reactor_iface.go` never named the neighbor watch | `docs/architecture/bgp/interface-event-reactions.md` | Four decisions added from the code: the watch and its route through `internal/core/network`, the whole-table re-read per signal, the watch state each snapshot carries, and no re-advertisement |
| 3 | NOTE | The `.ci` cited the dead path `plan/immediate/spec-bgp-unowned-rfc-rows.md` | `test/plugin/linklocal-capability-malformed-nexthop-withdraw.ci` | Comment-only edit to the bare stem, the form the other `.ci` files use for a closed spec; listed in the Blocker table |
| 4 | NOTE | Key Design Decisions said a row is not recorded as covered while its named proof is absent, yet rows 2 and 10 read as proven on the ledger | this spec, Key Design Decisions | Reworded to the truth: ledger-proven by unit record, interop and `.ci` proof owed in the two receiving specs. No `rfc/short/` annotation changed, because the gates page has none for that state |
| 5 | NOTE | The row 4 quote dropped the draft's "treat-as- withdraw" spacing; the Warn proof was credited to the wrong tests; `TestLinkLocalCapabilityWrongLengthStillResets` was unlisted | this spec, the RFC text table and the Unit Tests table | The quote made byte-exact in both places; the Warn rows name `TestThirdPartyMissWarningTellsTheTruth` and `TestNoNeighbourReaderWarnsOncePerPeer`; the wrong-length and not-negotiated tests listed |
| 6 | NOTE | Three Warn lines in `link_scope_neighbors.go` carry no `code` attribute | `internal/component/bgp/reactor/link_scope_neighbors.go` | Not changed here. The lines landed on 2026-10-31, before the directive (2026-11-10), whose gate refuses only a site a change adds. They are site C17 of `plan/release/validation/spec-doctor-coverage-6-bgp-engine.md` (its AC-28), which keeps `doctor-bgp-neighbor-table` for the unreadable table and adds `doctor-bgp-neighbor-watch-stopped` for the watch. Only the unreadable-table line has a declared code today, and that code is a private constant of `internal/component/bgp/config`, which imports the reactor |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/linklocal-capability-malformed-nexthop-withdraw.ci` | Yes | `ls`, 2026-11-14: 5.7K, Oct 30 |
| `test/plugin/rfc2545-third-party-linklocal.ci` | Yes | `ls`, 2026-11-14: 3.5K, Oct 30 |
| `test/encode/bgp-srpolicy-bsid-srv6-encode.ci`, `test/decode/bgp-srpolicy-bsid-srv6-decode.ci`, `test/parse/srpolicy-binding-sid-srv6.ci` | Yes | `ls`, 2026-11-14: Oct 20 |
| `test/interop/scenarios/bgp-rfc2545-third-party-linklocal-frr/`, `test/interop/scenarios/bgp-sr-policy-gobgp/` | Yes | `ls test/interop/scenarios`, 2026-11-14 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-4 | both polarities tagged and recorded | `grep -rn "RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1"`: `draft_ietf_idr_linklocal_capability_malformed_test.go` lines 47 and 89, `rfc2545_forward_subnet_test.go` lines 231, 232, 627, 628, 701; the record lists five units. Run: `./le test unit pkg path ./internal/component/bgp/message/ path ./internal/component/bgp/plugins/nlri/srpolicy/`: ok; `./le test bgp plugin linklocal-capability-malformed-nexthop-withdraw`: pass 1/1 (main thread, 2026-11-14) |
| AC-5 | tested half tagged and recorded | the record lists `TestRFC2545ThirdPartyLinkLocalAnnounce`, `TestRFC2545ThirdPartyLinkLocalForward` and `checkRFC2545ThirdPartyLinkLocal`. Run: `./le test unit pkg path ./internal/component/bgp/reactor/ run 'RFC2545\|ThirdParty\|LinkScope\|NeighborWatch\|NeighbourReader\|LinkLocal'`: ok (8.586s); the whole reactor package was not run, because another session is editing it. `./le test qemu ci test test/plugin/rfc2545-third-party-linklocal.ci`: pass 1/1 in Ze's guest (main thread, 2026-11-14). Not yet run: the interop scenario `bgp-rfc2545-third-party-linklocal-frr`; `./le test unit pkg path ./internal/plugins/iface/netlink/` (Linux), which holds the `TestNeighbourWatch*` tests; `./le test unit pkg path ./internal/component/bgp/config/`, which holds the two doctor tests |
| AC-6 | both polarities tagged and recorded | `rfc9830_bsid_srv6_test.go` lines 26, 29, 80, 93; the record lists the two units. Run: the srpolicy package in the unit run above: ok; `./le test bgp encode bgp-srpolicy-bsid-srv6-encode`, `./le test bgp decode bgp-srpolicy-bsid-srv6-decode`, `./le test bgp parse srpolicy-binding-sid-srv6`: each pass 1/1 (main thread, 2026-11-14). Not yet run: the interop scenario `bgp-sr-policy-gobgp` |
| AC-8 | ledger clean for the three stems | not yet run: `./le rfc check`. Read on 2026-11-14: rows `-6-1` and `RFC9830-2.4.2-12` carry no annotation, `RFC2545-3-6` is `{partial}`, and the three discrimination records exist |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| malformed 32-octet next hop | `test/plugin/linklocal-capability-malformed-nexthop-withdraw.ci` | pass 1/1 (2026-11-14). It runs `ze --plugin ze.bgp-llnh` against two `le test peer` connections and asserts the bytes the second one receives |
| third-party next hop from config | `test/plugin/rfc2545-third-party-linklocal.ci` | pass 1/1 in Ze's QEMU guest (2026-11-14; `option=needs-linux:caps=net-admin`). It starts `ze start ze.conf` in a network namespace and asserts the MP_REACH next-hop bytes the peer receives |
| `binding-sid srv6` | the three SR Policy `.ci` files | each pass 1/1 (2026-11-14). Encode drives the config route and `update text` and asserts the UPDATE hex; decode runs `ze bgp decode --json`; parse runs `ze config validate` and asserts the refusal text |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-5 | confirmed | the FRR and BIRD functions are named in the comparison table |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| RFC status rows | `rfc/short/` rows read on 2026-11-14: `draft-ietf-idr-linklocal-capability.md` line 128 and `rfc9830.md` line 313 carry no annotation, `rfc2545.md` line 206 is `{partial}` | Yes for the rows. This closure changed no `rfc/short/` row, so the generated `docs/features/rfc-status.md` owes no regeneration; `./le rfc check` (not yet run) judges the records |
| The Documentation Update Checklist's Yes rows | each row names its page and lines, read on 2026-11-14 | Yes; `./le doc check all` source-anchor stage passes |
| News fragment | `website/changes/fragments/bgp-unowned-rfc-rows.md`, topic `BGP` (a tag of `website/data/topics.json`), six headings in order | written; `./le commit create` validates it when commit A is prepared |
