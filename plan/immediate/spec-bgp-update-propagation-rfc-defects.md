# Spec: bgp-update-propagation-rfc-defects

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Six places where Ze accepts or re-advertises a BGP route in a way its RFC or
draft forbids, found by the strict re-read of
`spec-rfc-requirement-quote-hand-backfill`. An operator meets
each as a wire behaviour towards a peer: bits that must be zero are sent,
duplicates that must not be sent are sent, a malformed MUP route is kept, a
received link-local next hop reaches a peer off the link, and
two propagation rules whose reading the owner must settle. Each row was read
at its producer in HEAD on 2026-09-27.

## Defects

| ID | RFC section | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|-------------|----------------|----------|--------------------|------------------------|
| D1 | RFC 4271 Section 4.3 (RFC4271-4.3-3, 4.3-4) | "The lower-order four bits of the Attribute Flags octet are unused. They MUST be zero when sent and MUST be ignored when received." | `internal/core/bgp/attribute/opaque.go::NewOpaqueAttribute` (flags kept verbatim), `internal/core/bgp/attribute/attribute.go::WriteHeaderTo` (writes flags unmasked) | An unrecognised attribute received with low-order flag bits set is re-advertised with those bits set | unverified by a wire test |
| D2 | RFC 8092 Section 3 (RFC8092-3-1) | "Duplicate BGP Large Community values MUST NOT be transmitted. A receiving speaker MUST silently remove redundant BGP Large Community values from a BGP Large Community attribute." | `internal/component/bgp/message/rfc7606.go::validateLargeCommunityAttr` (length only), `internal/component/bgp/plugins/rib/ribout_entry.go::parseLargeCommunityWire` | Received duplicates are neither removed on receipt nor filtered on the forward path that reuses the received bytes; only the `LargeCommunities` encoder dedups | weak; lead not yet traced to the wire |
| D3 | draft-ietf-bess-mup-safi Section 3.1.3.1 (rows 3.1.3.1-6, -7, -10) | "the mandatory fields exceed the declared Length; the NLRI is malformed; MUST be treated as Treat-as-withdraw." and "any TLV parsing error MUST result in Treat-as-withdraw" | `internal/core/bgp/nlri/nlrisplit/mup.go::SplitMUP` with `RecognizeNLRI` at ingress | Ingress frames the NLRI on its length and stores it; `ParseMUP`'s truncation and TLV errors are reached only by the decoder, so a malformed ST1 route is kept instead of withdrawn | weak, code gap with no `{gap}` marker |
| D4 | RFC 9234 Section 3.1 (RFC9234-3.1-1) | "Customer: MAY propagate any route learned from a Customer, or that is locally originated, to a Provider. All other routes MUST NOT be propagated." | `internal/component/bgp/plugins/role/otc.go::OTCEgressFilter` | Suppresses sources whose local role is customer, peer or rs-client; a route learned from an RS-Client (local role rs) is propagated to a Provider, Peer or RS. `TestOTCEgressFilter` subtest `src_role_rs_to_provider_accept` pins the same reading | weak; OWNER DECISION |
| D5 | RFC 7999 Section 3.1 (RFC7999-3.1-2) | "In a bilateral peering relationship, use of the BLACKHOLE community MUST be agreed upon by the two networks before advertising it." | `internal/component/bgp/plugins/cmd/announce/blackhole_agreement.go::agreedSelector`; `internal/component/bgp/plugins/filter_community/config.go` `blackholeGuardToken` (default none) | The agreement gate covers origination by command only; a received BLACKHOLE route is re-advertised to a peer that never agreed, since the Section 3.2 propagation guard is off by default | weak; OWNER DECISION |
| D6 | RFC 2545 Section 3; draft-ietf-idr-linklocal-capability-06 Section 4 item 1 (DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2) when capability 77 is negotiated | RFC 2545: "The link-local address shall be included in the Next Hop field if and only if the BGP speaker shares a common subnet with the entity identified by the global IPv6 address carried in the Network Address of Next Hop field and the peer the route is being advertised to." Draft: "If the internal peer is more than one IP hop away, the BGP speaker MUST NOT include a Link-Local IPv6 next hop." | `internal/component/bgp/reactor/forward_next_hop.go::egressNextHopGlobalHalf`, `egressNextHopWithheld`; `reactor_api_forward.go::forwardUpdateSection`; `forward_rs.go::reactorForwardRSSection`; `forward_build.go::buildWithdrawalPayload` | Initial defect: Link-Local-only relays crossed off-link under unchanged/auto, and withhold gates failed to withdraw the previous generation. Fixed, including mixed post-policy output and section-specific withdrawals | D6 implementation and independent round 3 review complete; scoped closure evidence below. D1–D5 and the whole-spec status remain open |

D6 also owns the missing-next-hop outcomes for
`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1`, `-4-4` and `-4-9`
(2026-10-02). With capability 77 negotiated, Section 4 says: "If, after
completing these procedures, there are no IPv6 next hop addresses included in
the next hop, the BGP route MUST not be advertised to its peer. Instead,
treat-as-withdraw (Section 2 of [RFC7606]) is used." Its internal-peer case
says: "If, after evaluating the above procedures, there are no IPv6 next hops
included with the route, the route MUST NOT be announced to the remote BGP
speaker. (Treat-as-withdraw.)" Its multihop external-peer case says: "If a
Global IPv6 next hop is not included, the route MUST NOT be advertised to the
external peer (treat-as-withdraw)."

`TestDraftLinkLocalOnlyRouteCannotCrossMultihopEgress` in
`internal/component/bgp/reactor/rfc_draft_linklocal_missing_global_test.go`
reproduces a link-local-only next hop leaking under unchanged/auto forwarding
to both internal and external multihop peers; its global-address controls
pass. The probe is untagged: no discrimination record or enforced verdict is
claimed. D6 must suppress the unusable announcement and withdraw any
previously advertised generation, while preserving the usable-global control.
The distinct single-hop external condition in `-4-7` was then reproduced by
`TestDraftLinkLocalOneHopLostNextHopWithdraws` in
`internal/component/bgp/reactor/rfc_draft_linklocal_onehop_withdraw_test.go`.
An already advertised route remains at the external, directly attached peer
after the speaker loses its usable next-hop-self addresses. Both forwarding
rails originally used `wireu.WithdrawalsOnly` on the replacement announcement,
which contains no withdrawal, and sent nothing. Section 4's one-hop default procedure
says: "If no next hops are included, the route MUST NOT be announced
(treat-as-withdraw)." D6 owns this missing withdrawal too; its probe remains
untagged, with no enforced verdict claimed.

## Re-verification at HEAD (2026-10-13, design phase)

Each row re-read at its producer at HEAD `3e2770062b`. Verdicts read from
`rfc/audit/<stem>.json`. `./le rfc check` (rc 0) names none of these IDs as a
residual: the weak verdicts live in the audit JSON and in rows 305-312 of
`plan/pre-release/spec-rfc-verdict-fix-bgp.md`, which point here. No other
session holds this spec: no state file under `tmp/session/*/state/` names it, and
its last commit is `d09deeecf3` (2026-10-04, D6 closure).

| ID | Verdict at HEAD | Producer at HEAD | Still open? |
|----|-----------------|------------------|-------------|
| D1 | RFC4271-4.3-3 weak, RFC4271-4.3-4 weak | `reactor/forward_body.go::forwardOpaquePayload` edits only the drop and Partial cases; `forwardWire` returns the received bytes unchanged when `preserveOpaque` (route-server clients); `reactor/session_write.go::writeUpdateBody` copies every body into the session's private buffer and edits only the Prefix-SID Label-Index (`session_prefix_sid.go::clearTransmittedLabelIndex`). A grep for a low-nibble mask over `attribute/`, `reactor/`, `wireu/`, `message/` finds none | Yes |
| D2 | RFC8092-3-1 weak | Receive half FIXED in `586887a063` (2026-09-30): `reactor/rfc8092_large_community.go::removeRedundantLargeCommunities`, called from `publishBase` (`session_validation.go`), removes repeats at ingest. That is RFC8092-3-2's sentence. The transmit half is open at a different producer from the one the Defects row names: `filter_community/handler.go::genericCommunityHandler` appends every `AttrModAdd` value without asking whether the attribute already holds it, so an export policy adding a value the route carries, or adding one value twice, transmits it twice. The row's `parseLargeCommunityWire` lead is closed: `reconstructRoute` replays through text commands whose encoder (`LargeCommunities.WriteTo`) removes repeats, and `packEventAttrs` builds Adj-RIB-Out storage, not wire | Yes, narrowed to the transmit half |
| D3 | 3.1.3.1-6, -7, -10 weak | `d3e5b4a770` changed only `SplitMUP`'s callback signature. `nlri/mup/rfc7606.go::RecognizeNLRI` reads the architecture and route type; `nlritype.Recognizer` returns a bool and documents that it never validates the body. `ParseMUP` (`nlri/mup/types.go`) holds every check the draft names, on the decode path only | Yes |
| D4 | RFC9234-3.1-1 weak | `role/otc.go::OTCEgressFilter` unchanged since 2026-09-27: source roles suppressed towards Provider/Peer/RS are `customer`, `peer`, `rs-client`; `rs` passes | Yes, owner decision |
| D5 | RFC7999-3.1-2 weak | `cmd/announce/blackhole_agreement.go::agreedSelector` gates origination by command only; `filter_community` leaf `blackhole-propagation` defaults `none` | Yes, owner decision |
| D6 | linklocal 4-1, 4-2, 4-4, 4-7, 4-9 enforced | closed in `d09deeecf3`, `2987e2c517`, `9ac8bd76fb` (sections below) | No |

New finding while reading D3's producer: `nlri/mup/encode.go::writeT1STData`
writes no TEID when `teidBits` is 0 and no Endpoint Address Length when the
endpoint is unset, so a T1ST that Ze originates without those values is one its
own `ParseMUP` refuses. Once D3 lands, a Ze receiver treats such a route as
withdrawn. This is the send side of the `{gap}` on DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-1.
Owner question Q-2.

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | One test per row, red against HEAD, over the forward path (not the codec alone) |
| Both polarities | D1: set bits cleared on send, clear bits unchanged. D2: duplicates removed, distinct values kept in order. D3: each malformed shape withdrawn, a well-formed route with an unknown TLV kept and propagated unchanged |
| Discrimination | `./le rfc discriminate-record` for every tagged unit |
| Real entry point | `.ci` tests: a peer sends the UPDATE, a second peer receives the re-advertisement |
| D6 cases | a received Global plus Link-Local next hop forwarded under next-hop auto to a multihop internal peer carries the global address only (red against HEAD); to an internal peer on the same link it may keep the link-local. The multihop EBGP twin (4-8, "Link-Local IPv6 next hops MUST NOT be included.") goes through the eBGP next-hop rewrite, which was not traced: the same test covers it |
| Interop | D1, D2 against FRR or BIRD as the receiving peer; D4 against a Role-capable FRR; D6 with FRR as an off-connected-subnet eBGP receiver (`bgp-linklocal-only-multihop-withdraw-frr`, FRR AS 65002, Ze AS 65001, 834f7d26e2). FRR's adjacent-interface/loopback topology has no transit router; the `.ci` supplies the separate genuine routed-hop proof. The multihop internal case (4-1, 4-2) is proven by unit tests only |

## D6 design (owner decisions 2026-10-03)

Owner authorised the fix without claiming the spec. This section covers D6 and
the owner-widened withdrawal fix only; D1 to D5 stay open.

-> Decision: (owner, 2026-10-03, "the analysis lgtm") a route whose next hop
about to be written is Link-Local-only is WITHHELD, as treat-as-withdraw, from a
destination that is not on-link, on both rails. Research behind it: draft
Section 4 forbids a Link-Local next hop towards a peer more than one hop away
("If the internal peer is more than one IP hop away, the BGP speaker MUST NOT
include a Link-Local IPv6 next hop.") and forbids announcing a route left with
no next hop ("If, after completing these procedures, there are no IPv6 next hop
addresses included in the next hop, the BGP route MUST not be advertised to its
peer. Instead, treat-as-withdraw (Section 2 of [RFC7606]) is used."). Its
default under the other modes is rewriting to self, which FRR and BIRD do; under
`next-hop unchanged` the operator configured propagation, so withholding is the
conformant act. By mode: unchanged and multihop, withhold; unchanged and
on-link, sent unchanged; auto and eBGP, rewrite to self, which is
`plan/immediate/spec-bgp-next-hop-auto-rewrites-for-ebgp.md` and NOT this fix;
self and RR next-hop-self, rewritten (existing).
-> Decision: the gate is mode-independent: "destination not on-link" (the hop
test egressNextHopGlobalHalf uses; a nil link scope proves no shared subnet) and
"the next hop about to be written is Link-Local-only" (egressNextHopIsLinkLocalOnly
over mods then base; the 24-octet RD plus Link-Local VPN field counts, because
nextHopAddr leaves its second address invalid). Under `auto` it withholds until
the auto-rewrite spec records a rewrite in mods, and then stops firing with no
edit. Placed after egressNextHopGlobalHalf and before RFC 8950.
-> Decision: the RR reflected-only gate is NOT merged into it: it asks a
different question (the client against the ORIGINAL ADVERTISER's segment), so
an on-link client off the advertiser's segment is refused by it and passed by
the new gate. The route-server rail gains the RR gate as well, because it
injects the RFC 4456 reflection attributes for the same pairs and the two rails
must answer alike.
-> Decision: (owner) every withhold gate that suppresses a route the destination
may already hold WITHDRAWS it: next-hop-self withheld, the new Link-Local-only
gate, peer-own NEXT_HOP, RR Link-Local-only, RFC 8950, Link-Local-only refused,
RFC 1997 well-known communities, RFC 7947 control communities, and a genuine
egress policy reject (a step that could not run stays a drop, unchanged). Reuse:
mods.SetWithdraw() and buildWithdrawalPayload, the RFC 9494 LLGR conversion;
wireu.WithdrawalsOnly lost its forward-rail callers and was deleted with its
test (owner approval, Thomas, 2026-10-03: "Delete both"). The reactor rail holds no
per-peer Adj-RIB-Out, so the withdrawal is unconditional: RFC 7606 Section 2
treat-as-withdraw. BIRD does not do this: `rt_notify_basic` (`nest/rt-table.c`)
withdraws only a route its `export_map` says was exported to the channel, so it
never sends the withdrawal of a route the peer was not sent. Withdrawing only
what was sent is `plan/immediate/spec-bgp-withdraw-only-exported-routes.md`
(skeleton, c5219d54b0). Until it lands, a withdrawal of a route the destination
never held is a no-op for it, and its family is one the announcement would have
been sent in.
-> Decision: buildWithdrawalPayload merges the source UPDATE's own Withdrawn
Routes and MP_UNREACH_NLRI with the converted announcement. HEAD dropped them,
a latent LLGR defect in the same function (an LLGR conversion of a mixed UPDATE
lost its withdrawals), fixed here. One MP_UNREACH_NLRI is emitted (RFC 7606
Section 3(g) refuses two); when the source's MP_UNREACH and MP_REACH name
different AFI/SAFI the converter refuses that shape. The rails partition it into
separate single-field messages instead, preserving both families.
-> Constraint: the next-hop withhold sequence is one function both rails call
(egressNextHopWithheld, forward_next_hop.go), so the rails cannot diverge.

### D6 tests

| Test | Proves |
|------|--------|
| TestDraftLinkLocalOnlyRouteCannotCrossMultihopEgress (+ RS twin) | 4-1/4-4/4-9: LL-only to a multihop internal or external peer is withdrawn, global control sent |
| TestDraftLinkLocalOneHopLostNextHopWithdraws (+ RS twin) | 4-7: lost next-hop-self addresses withdraw the advertised generation |
| relay 4-2 test | 32-octet pair to a multihop internal peer carries the Global alone (red by reverting egressNextHopGlobalHalf) |
| per-gate withdraw tests | each gate class sends a withdrawal of the announced NLRI |
| buildWithdrawalPayload merge tests | source withdrawals kept beside the converted announcement |
| `test/plugin/linklocal-only-multihop-withdraw.ci` (QEMU, `needs-linux:caps=net-admin`) | 4-4/4-9 end to end on the RS rail over a real routed IPv6 hop: an on-link sender relays a Global then a Link-Local-only generation through Ze (`next-hop unchanged`, capability 77 on both sessions) to an external peer one router away, which receives the Global, then a withdrawal, then a control route, never fe80::9. Fixture: `plugin/clamped-path` grew `ipv6`, `peer` and `peer-after` (owner, 2026-10-03: "Extend the netns fixture") |

## Owner decisions

| Row | Question |
|-----|----------|
| D4 | Does an RS-Client-learned route count as "learned from a Customer" for propagation to a Provider, Peer or RS? The RFC sentence says it does not; code and test say it does |
| D5 | Does "advertising it" in Section 3.1 cover re-advertising a received BLACKHOLE route, and should the Section 3.2 guard default on? |

None of these is answered. No row below records an approval.

| ID | Question | Options | Recommendation and reason | Blocks |
|----|----------|---------|---------------------------|--------|
| Q-D4 | Is a route learned from an RS-Client (Ze's local role `rs`) propagated to a Provider, Peer or RS? | (a) No: follow RFC 9234 Section 3.1, add `rs` to the suppressed source roles. (b) Yes: keep HEAD, and re-scope RFC9234-3.1-1's claim to exclude the RS-learned case with the reason recorded | (a). Section 3.1, quoted below, makes "All other routes MUST NOT be propagated" for the Customer, RS-Client and Peer roles, and an RS-Client is not a Customer. Section 5's OTC text describes what OTC detects, not what Section 3.1 permits, and Section 5 ingress rule 3 adds no OTC to a route received from an RS-Client, so without (a) nothing stops the route | AC-4, AC-11; flips the tagged subtest named in the TDD plan |
| Q-D5 | Which reading of RFC 7999 Section 3.1 applies to a RECEIVED BLACKHOLE route? | (a) "advertising it" covers re-advertisement: withhold (and so withdraw) a BLACKHOLE-tagged route from a destination whose `blackhole` container does not list the community, on both forward rails. (b) Section 3.1 binds origination only; the propagation question is Section 3.2's SHOULD, and `blackhole-propagation` default changes from `none` to `no-export`. (c) Section 3.1 binds origination only and the default stays `none`; RFC7999-3.1-2's claim is narrowed to origination with the reading recorded | (a). Section 3.1 states the agreement in terms of the community and two networks, not of who created the route, and the agreement data Ze already holds per session (`blackholecfg.Rule.Communities`) answers it with no new configuration. (b) changes the propagation default for every operator and still lets the route reach an un-agreed eBGP peer inside a confederation or over iBGP | AC-4, AC-12 |
| Q-1 | D3 scope: does the ingress MUP check run all of `ParseMUP`, or only the three owned shapes? | (a) all of `ParseMUP`, which also enforces the `{gap}` rows 3.1.3.1-1, -2, -3, -4, 3.1.1-1, 3.1.1-2, 3.1.2-1, 3.1.3-1, 3.1.4-1, 3.1.4.1-1, -2 at ingress. (b) only Length overrun, one leftover octet and TLV overrun | (a). One validator already exists; (b) needs a second, partial parser beside it, which is the duplicate declaration `principles.md` forbids. It does widen this spec's scope, so it needs the owner | AC-3 |
| Q-2 | Ze-originated T1ST without a TEID or endpoint: fix here or journal? | (a) refuse the route at the `update`/config entry point with an error naming the missing field, in this spec. (b) journal row, out of scope | (a) if Q-1 is (a): the D3 fix otherwise makes two Ze speakers withdraw each other's routes. (b) if Q-1 is (b) | AC-3 interop |
| Q-3 | Do the D1 and D2 egress normalizations apply to the explicit raw diagnostic write (`writeUpdateBody` with `raw` true)? | (a) yes, as `clearTransmittedLabelIndex` already does. (b) no: raw stays byte-exact for fault injection | (a), for uniformity with the existing Prefix-SID normalization at the same spot. If an operator needs non-conformant bytes for a lab, that is a separate, named feature | AC-1, AC-2 |
| Q-4 | D2: remove repeats at egress for LARGE_COMMUNITY only, or also for COMMUNITIES and EXTENDED COMMUNITIES? | (a) LARGE_COMMUNITY only. (b) all three | (a). Only RFC 8092 Section 3 forbids transmitting a repeat; RFC 1997 and RFC 4360 do not, and changing their bytes changes behaviour no RFC asks for | AC-2 |
| Q-5 | Approval for the RFC-tagged units this spec changes (list in the TDD plan) | approve, or rule otherwise | Needed before implementation of D4 (and D5 under Q-D5 (b)) | AC-5 |

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md`, `rfc/short/rfc8092.md`, `rfc/short/rfc9234.md`, `rfc/short/rfc7999.md`, `rfc/drafts/draft-ietf-bess-mup-safi.txt` Section 3.1.3.1

### RFC text read for this design (whole sentences)

- [ ] `rfc/full/rfc4271.txt` Section 4.3 (line 922): "The lower-order four bits of the Attribute Flags octet are unused. They MUST be zero when sent and MUST be ignored when received."
  -> Constraint: the four bits are 0x0F of the flags octet. Extended Length (0x10), Partial (0x20), Transitive (0x40) and Optional (0x80) MUST NOT be touched by the fix. The sentence binds every attribute, recognized or not.
- [ ] `rfc/full/rfc8092.txt` Section 3 (line 204): "Duplicate BGP Large Community values MUST NOT be transmitted. A receiving speaker MUST silently remove redundant BGP Large Community values from a BGP Large Community attribute." Line 200: "There is no significance to the order in which twelve-octet Large Community Attribute values are encoded in a Large Communities attribute, A BGP speaker can transmit them in any order."
  -> Constraint: the first sentence (RFC8092-3-1) is about what goes on the wire, whatever produced it; the second (RFC8092-3-2) is met at ingest already. Keep-first order is a choice, not an obligation; the ingest helper already keeps first, and the egress fix keeps the same rule so one value set never has two orders.
- [ ] `rfc/drafts/draft-ietf-bess-mup-safi.txt` Section 3.1.3.1 (lines 427-457): "To determine their presence, compute the number of remaining octets by subtracting, from the Length field in the MUP NLRI common header, the offset of the end of the Source Address measured from the start of the Route Type specific field." Then "< 0: the mandatory fields exceed the declared Length; the NLRI is malformed; MUST be treated as Treat-as-withdraw." "1: encoding is invalid (a valid TLV requires at minimum a Type byte and a Length byte); MUST be treated as Treat-as-withdraw." ">= 2: one or more TLVs are present. Parse TLVs one by one per Section 3.1.5; unknown TLV types MUST be ignored for local processing and MUST be propagated unchanged when re-advertising the route to other BGP peers; any TLV parsing error MUST result in Treat-as-withdraw; TLVs are not part of the NLRI key for route processing." The sibling rules of the same section add: "A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message."
  -> Decision: treat-as-withdraw is per NLRI. The malformed NLRI leaves MP_REACH_NLRI and is withdrawn; well-formed siblings in the same UPDATE are processed normally.
  -> Constraint: TLVs are not part of the key. `nlrisplit/mup_key.go::keyMUP` already keys a T1ST on RD, prefix length and prefix, so a withdrawal built from the malformed NLRI's bytes removes the earlier well-formed generation of the same route.
- [ ] `rfc/full/rfc9234.txt` Section 3.1 (lines 163-182): "Customer: MAY propagate any route learned from a Customer, or that is locally originated, to a Provider. All other routes MUST NOT be propagated." Same shape for "Route Server Client (RS-Client)" towards an RS and "Peer" towards a Peer. "If the local AS has one of the above Roles (in the order shown), then the corresponding peering relationship with the remote AS is Provider-to-Customer, Customer-to-Provider, RS-to-RS-Client, RS-Client-to-RS, or Peer-to-Peer". Section 5 ingress rule 3: "If a route is received from a Provider, a Peer, or an RS and the OTC Attribute is not present, then it MUST be added with a value equal to the AS number of the remote AS."
  -> Constraint: the role named in Section 3.1 is the LOCAL role towards the destination. "learned from a Customer" is a source session where Ze's local role is `provider`. A source where Ze's local role is `rs` is an RS-Client, not a Customer.
  -> Constraint: no OTC is added to a route received from an RS-Client (ingress rule 3 lists Provider, Peer, RS only), so the wire-bytes OTC check cannot stop it; only the source-role check can.
- [ ] `rfc/full/rfc7999.txt` Section 3.1: "In a bilateral peering relationship, use of the BLACKHOLE community MUST be agreed upon by the two networks before advertising it. In a multilateral peering relationship, the decision to honor or ignore the BLACKHOLE community is to be made according to the operator's routing policy." Section 3.2: "A BGP speaker receiving an announcement tagged with the BLACKHOLE community SHOULD add the NO_ADVERTISE or NO_EXPORT community as defined in [RFC1997], or a similar community, to prevent propagation of the prefix outside the local AS."
  -> Constraint: RFC 7999 is Informational; the sentence still carries a MUST and is enrolled. Which reading applies to re-advertisement is Q-D5.

### Architecture pages
- [ ] `docs/architecture/bgp/structural-forwarding.md` "Four costs" and following: both rails key body reuse on opaque-attribute treatment; route-server clients get `PreserveOpaqueAttributes`.
  -> Constraint: a fix in `forwardOpaquePayload` alone misses route-server clients and every non-forward writer (origination, replay, plugin-injected bodies), so D1 belongs at the per-session final writer.
- [ ] `docs/architecture/wire/attributes.md` "Attribute Flags" and "32. LARGE_COMMUNITY".
  -> Decision: both sections gain the egress rule with a source anchor to the new function (Documentation row 7).
- [ ] `docs/guide/bgp-role.md` table at line 95 (rule 2, the Gao-Rexford check).
  -> Constraint: under Q-D4 (a) the row's role list gains `rs`; the page is edited in the same change.
- [ ] `docs/architecture/testing/interop.md` "Scenario Structure", "Typed checker operations", the raw injector sidecar (172.30.0.9, `inject.msg`).
  -> Decision: D1, D2 and D3 scenarios drive malformed or unusual input through the raw injector and assert at GoBGP; each new directory has a typed checker in `internal/le/interoplab/bgp` with both polarities in `TestBespokeCheckerBranches` when bespoke.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/bgp/attribute/opaque.go` (74L) - `OpaqueAttribute` keeps received flags verbatim; used by the decode/rebuild path
  -> Constraint: not the only producer; the forward rails copy raw attribute bytes and never build an `OpaqueAttribute`.
- [ ] `internal/core/bgp/attribute/attribute.go` - `WriteHeaderTo` writes the flags argument as given
  -> Constraint: local encoders pass constant flags (covered by `TestRFC4271AttributeFlagsLowNibbleZeroOnSend`); leave it.
- [ ] `internal/component/bgp/reactor/forward_body.go` - `forwardWire`, `forwardOpaquePayload`: RFC 4271 Section 5 drop/Partial treatment on a body-cache miss; preserved destinations get the received bytes
- [ ] `internal/component/bgp/reactor/session_write.go` - `writeUpdateBody`: every UPDATE writer funnels here (`writeUpdateGated`, `writeOrdinaryUpdateBody` split chunks, `writeRawUpdateBody`, `writeOriginatedRawUpdate`, `writeUpdateWithoutAIGP`, the message path of `writeMessage`); copies the body into `s.writeBuf` then calls `clearTransmittedLabelIndex`, then the ownership filter and PATHS-LIMIT
  -> Decision: D1 and D2 run at the `clearTransmittedLabelIndex` call site, on the private buffer, before the ownership filter, so duplicate-advertisement comparison sees final bytes.
  -> Constraint: received bytes are shared across destinations and the forward cache; they MUST NOT be edited. The private buffer is the only legal place.
- [ ] `internal/component/bgp/reactor/session_prefix_sid.go` - `clearTransmittedLabelIndex`: `wire.ParseUpdateSections` then `attribute.AttrFind`, in-place edit, silent return on malformed framing
  -> Decision: the D1 and D2 work joins this into one attribute walk per written UPDATE, not three.
- [ ] `internal/component/bgp/reactor/rfc8092_large_community.go` (169L) - `removeRedundantLargeCommunities` (ingest), `largeCommunityRedundant` (pair scan up to 16 values, stack bitset above), `largeCommunityAttrsDeduped` (allocating copy)
  -> Decision: reuse `largeCommunityRedundant` for detection; add an in-place compaction for the egress side (no allocation; the private buffer is owned).
- [ ] `internal/component/bgp/plugins/filter_community/handler.go` - `genericCommunityHandler`: Keep for retained values, `p.Op(i)` for every Add with no membership check; header written Extended Length 0xC0
  -> Constraint: the transmit-side repeat is produced here today; the egress fix covers it without editing the handler (Q-4 keeps COMMUNITIES and EXTENDED COMMUNITIES as they are).
- [ ] `internal/component/bgp/plugins/rib/ribout_entry.go` - `reconstructRoute` (stored bytes to `Route`, replayed as a text command), `packEventAttrs` (Adj-RIB-Out storage from a sent event)
  -> Constraint: neither writes a peer's wire directly; no change.
- [ ] `internal/component/bgp/message/rfc7606.go` - `validateAttributeFlags` judges Optional/Transitive only via `AttributeCode.FlagsConflict`; `validateLargeCommunityAttr` checks the length only
  -> Constraint: receive already ignores the low nibble for the three attributes the existing tagged test covers; AC-7 extends the proof to the forward rail.
- [ ] `internal/component/bgp/reactor/session_validation.go` - `applyRFC7606`: keep-first strip, tunnel-encap, then `applyTypedNLRIDiscard`; `publishBase` runs `removeRedundantLargeCommunities`
- [ ] `internal/component/bgp/reactor/session_validation_nlritype.go` - `applyTypedNLRIDiscard`, `typedNLRIEdit`: per-family `nlritype.Retain` drops unrecognized types (RFC 7606 Section 5.4) and malformed BGP-LS NLRI (RFC 9552 Section 8.2.2); rebuilds the body once; `typedNLRIEmptied` stops a body with no routes
  -> Decision: D3 adds a third per-NLRI outcome, malformed-withdraw, in the same pass and the same rewrite.
- [ ] `internal/component/bgp/reactor/session_read.go` (lines 271-318) - treat-as-withdraw dispatch: `SynthesizeWithdrawFamilies` bodies; extra bodies enter `onMessageReceived` with a `noPoolBufID` handle so the forward cache finds them
  -> Decision: the D3 withdrawal of malformed MUP NLRIs rides an extra body through this same mechanism when the UPDATE already carries an MP_UNREACH_NLRI (RFC 7606 Section 3(g) permits one); otherwise it is an MP_UNREACH_NLRI added to the rewritten body.
- [ ] `internal/core/bgp/nlri/nlritype/nlritype.go` - `Recognizer func(nlriBytes []byte, addPath bool) bool`, `Register`, `Get`, `Retain`
  -> Decision: the registry gains a per-family body validator registered by the owning plugin, so nothing central names MUP; `Retain` reports withdrawn NLRIs separately from discarded ones.
- [ ] `internal/component/bgp/plugins/nlri/mup/rfc7606.go` - `Implemented`, `RecognizeNLRI`
- [ ] `internal/component/bgp/plugins/nlri/mup/types.go` - `ParseMUP`, `parseBodyT1ST` (tolerates an absent Source Address Length octet, the ExaBGP shape), `validateTLVs` (returns `ErrMUPTLV` for one leftover octet and for a Length overrun), `ErrMUPTruncated`, `ErrMUPTEIDZero`, `ErrMUPEndpointLength`, `ErrMUPSourceLength`
  -> Constraint: `ParseMUP` takes an AFI and a single NLRI without the path identifier; the validator strips the 4-octet RFC 7911 identifier first when ADD-PATH is negotiated.
- [ ] `internal/component/bgp/plugins/nlri/mup/encode.go` - `writeT1STData` omits TEID when `teidBits` is 0 and the endpoint length when the endpoint is unset (Q-2)
- [ ] `internal/core/bgp/nlri/nlrisplit/mup_key.go` - `keyMUP`: T1ST key is RD, prefix length, prefix; a non-key tail is not parsed; a key that cannot be formed falls back to the whole wire NLRI in `RouteKey`
- [ ] `internal/component/bgp/plugins/role/otc.go` - `OTCEgressFilter`: unicast-only; wire OTC check; then source-role check (customer, peer, rs-client) when the destination's remote role is provider, peer or rs; then export role set; then OTC stamping
- [ ] `internal/component/bgp/plugins/cmd/announce/blackhole_agreement.go` - `agreedSelector`: origination by command only
- [ ] `internal/component/bgp/blackholecfg/blackholecfg.go` - `Rule.Communities` is the per-session agreement
- [ ] `internal/component/bgp/plugins/filter_community/config.go`, `blackhole.go`, YANG leaf `blackhole-propagation` (default `none`)

**Behavior to change:** D1 to D3; D4 and D5 after the owner rules.

| ID | Current | Proposed |
|----|---------|----------|
| D1 | an attribute received with any of 0x0F set is re-sent with those bits on every rail (route-server clients always; others whenever the attribute is not an unknown non-transitive) | every UPDATE written to any peer has 0x0F clear in every attribute's flags octet; no other bit changes |
| D2 | a LARGE_COMMUNITY repeat reaches the wire when export policy adds a value already present or adds one twice | no UPDATE written to a peer carries a LARGE_COMMUNITY repeat; first occurrence kept; attribute, path-attribute and message lengths reduced to match |
| D3 | a T1ST whose mandatory fields overrun its Length, or whose TLV area is one octet, or whose TLV overruns, is installed and relayed | that NLRI is treated as withdrawn: removed from MP_REACH_NLRI, withdrawn from Adj-RIB-In and relayed as a withdrawal; siblings kept; no session reset |
| D4 | RS-Client-learned unicast route sent to Provider, Peer, RS | under Q-D4 (a): suppressed, and so withdrawn, to those three |
| D5 | received BLACKHOLE-tagged route re-advertised to any peer | per Q-D5 |

**Preserved:** ingest repeat removal (RFC8092-3-2); RFC 4271 Section 5 drop/Partial treatment; route-server opaque preservation of attribute VALUES; MUP unknown-route-type discard (3.1-1) and unknown-TLV propagation (3.1.3.1-9); OTC wire check and stamping; blackhole origination gate; every D6 withhold and withdrawal behaviour.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- a BGP UPDATE received from a peer

### Transformation Path
1. RFC 7606 validation (`applyRFC7606`): keep-first strip, tunnel-encap, typed NLRI pass (`applyTypedNLRIDiscard`). D3 lands here: malformed MUP NLRI leave MP_REACH and become a withdrawal.
2. `publishBase`: ingest repeat removal for LARGE_COMMUNITY (unchanged).
3. RIB insert and best path; forward rails (`forwardUpdateCore`, `reactorForwardRS`) run egress filters per destination. D4 (`OTCEgressFilter`) and D5 (filter_community egress) land here; a genuine egress reject already withdraws (D6 owner decision).
4. Policy edits (`genericCommunityHandler` and siblings) build the destination's attributes.
5. Final writer `Session.writeUpdateBody`: private-buffer copy, then one attribute walk that clears the low nibble (D1), removes LARGE_COMMUNITY repeats (D2) and clears the Label-Index fields (existing), then ownership filter, PATHS-LIMIT, TCP.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ filter plugins | egress filter calls with wire payload and `ModAccumulator` (role, filter_community) | Yes, `OTCEgressFilter` signature read 2026-10-13 |
| NLRI plugin → core registry | `nlritype.Register` from `nlri/mup` `init()`; core never imports the plugin | Yes, registry read 2026-10-13 |
| Receive goroutine → forward cache | extra withdraw-only bodies through `onMessageReceived` with `noPoolBufID` | Yes, `session_read.go` read 2026-10-13 |

### Integration Points
- `Session.writeUpdateBody` (one call site, beside `clearTransmittedLabelIndex`)
- `nlritype` registry (new validator kind) and `typedNLRIEdit`
- `OTCEgressFilter` source-role set
- filter_community egress filter (D5 under Q-D5 (a))

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE from peer A with an unknown transitive attribute flagged 0xCF and ORIGIN flagged 0x4F, relayed to peer B (normal client) and peer C (route-server client) | → | `writeUpdateBody` egress walk | `test/plugin/rfc4271-attr-flags-low-nibble-relay.ci` |
| UPDATE from peer A with LARGE_COMMUNITY X; export policy on peer B adds X and adds Y twice | → | `genericCommunityHandler` then `writeUpdateBody` egress walk | `test/plugin/rfc8092-large-community-no-repeat-egress.ci` |
| ipv4/mup UPDATE from peer A: well-formed T1ST K, then an UPDATE with malformed T1ST of key K plus well-formed T1ST L | → | `typedNLRIEdit` MUP validator, extra withdraw body, forward | `test/plugin/mup-t1st-malformed-treat-as-withdraw.ci` |
| unicast route from a session where Ze's local role is `rs`, destinations with remote roles provider, peer, rs, customer, rs-client | → | `OTCEgressFilter` | `test/plugin/rfc9234-rs-client-learned-not-upward.ci` (under Q-D4 (a)) |
| BLACKHOLE-tagged route from an agreed peer A, destinations B (agreed) and C (not agreed) | → | filter_community egress gate | `test/plugin/rfc7999-blackhole-readvertise-agreement.ci` (under Q-D5 (a)) |
| D6: Link-Local-only UPDATE from an on-link peer, relayed to a multihop peer | → | `egressNextHopWithheld`, `egressNextHopLinkLocalOnlyOffLink`, `buildWithdrawalPayload` (RS rail) | `test/plugin/linklocal-only-multihop-withdraw.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | unknown transitive attribute with flag bits 0x0F set | forwarded with those bits zero |
| AC-2 | LARGE_COMMUNITY with a repeated value | forwarded once per value |
| AC-3 | MUP ST1 route with truncated mandatory fields or a bad TLV | treat-as-withdraw |
| AC-4 | D4, D5 | as the owner rules |
| AC-5 | the `weak` verdicts of RFC4271-4.3-3, RFC4271-4.3-4, RFC8092-3-1, DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6, DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-7, DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-10, RFC9234-3.1-1, RFC7999-3.1-2 and DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1, -4-2, -4-4, -4-7, -4-9, after this spec's producer fix | each verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. D6 covers missing-global internal and external multihop cases, loss of usable next hops at a directly attached external peer, withdrawal of an already advertised generation, and usable-global controls. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (parent P-3; original transfer 2026-09-28, missing-next-hop extension 2026-10-02) |

### Detailed acceptance criteria (design, 2026-10-13)

AC-1 to AC-5 above keep their IDs: `plan/pre-release/spec-rfc-verdict-fix-bgp.md` cites AC-5. The rows below make AC-1 to AC-4 testable and add the proof each owes.

| AC ID | Refines | Input / Condition | Expected Behavior | Proof |
|-------|---------|-------------------|-------------------|-------|
| AC-1a | AC-1 | peer A sends an UPDATE with ORIGIN flagged 0x4F and an unrecognized optional transitive attribute (code 241) flagged 0xCF; relayed to normal peer B and route-server client C | on B's wire ORIGIN is 0x40 and code 241 is 0xE0 (Partial stamped by the existing RFC 4271 Section 5 rule, nibble cleared); on C's wire code 241 is 0xC0 and its value bytes equal the received ones | unit over `writeUpdateBody` (both `raw` values, per Q-3) and the `.ci` in the Wiring table, hex assertions on both sessions |
| AC-6 | AC-1 | same relay, every flags octet received with 0x0F clear, Extended Length set on one attribute | bytes sent equal bytes the pre-fix code sends: Extended Length kept, no other bit touched | negative-polarity unit, byte-for-byte |
| AC-7 | AC-1 (RFC4271-4.3-4 receive) | ORIGIN, AS_PATH, NEXT_HOP and an unknown transitive attribute each received with 0x0F set | no RFC 7606 action, route installed and relayed (with the bits cleared) | unit over the receive-to-forward path, extending what `TestRFC4271AttrFlagsLowNibbleIgnoredOnReceive` proves at the validator |
| AC-2a | AC-2 | route with LARGE_COMMUNITY [X, Z]; export policy on B adds X, then Y, then Y | B receives [X, Z, Y], each once, first-occurrence order; attribute length 36; total path attribute and message lengths consistent | `.ci` with hex, plus unit over `writeUpdateBody` |
| AC-8 | AC-2 | a body reaching `writeUpdateBody` with LARGE_COMMUNITY [X, X, Y] from any producer (hand-built, as a plugin raw send would); and a body with 20 distinct values | first: sent as [X, Y]; second: sent unchanged (exercises the bitset path, above 16 values) | unit, both polarities; allocation count 0 on the no-repeat path (`testing.AllocsPerRun`) |
| AC-3a | AC-3 (3.1.3.1-6) | ipv4/mup UPDATE, MP_REACH with T1ST L (well-formed) and T1ST K whose Length ends inside the Endpoint Address | K is withdrawn (Adj-RIB-In entry for K's key removed if present; a withdrawal of K relayed), L installed and relayed, session stays Established, no NOTIFICATION | unit over `applyRFC7606` and the `.ci` in the Wiring table |
| AC-3b | AC-3 (3.1.3.1-7) | as AC-3a, K's TLV area is exactly one octet | as AC-3a | unit |
| AC-3c | AC-3 (3.1.3.1-10) | as AC-3a, K carries a TLV whose Length overruns the NLRI | as AC-3a | unit |
| AC-9 | AC-3 polarity | T1ST with a well-formed unknown TLV 0xF0 beside a malformed sibling | the well-formed route is kept and relayed with its TLV octets unchanged | unit; the existing 3.1.3.1-9 tests stay green unmodified |
| AC-10 | AC-3 edges | (i) the UPDATE already carries an MP_UNREACH_NLRI of another family; (ii) K was the only route; (iii) ADD-PATH negotiated for ipv4/mup; (iv) K's RD/prefix overruns so no key forms | (i) the K withdrawal rides a separate withdraw-only UPDATE and both families' withdrawals survive; (ii) the body becomes withdraw-only, never a forged End-of-RIB; (iii) the path identifier is kept on the withdrawal; (iv) the withdrawal carries the received bytes, no panic, no session reset. A framing overrun of the MP attribute itself stays RFC 7606 Section 5.3 session reset (unchanged) | unit table, one case each |
| AC-11 | AC-4 (D4, under Q-D4 (a)) | unicast route learned on a session where Ze's local role is `rs`; destinations with remote roles provider, peer, rs, customer, rs-client; plus a locally originated route | not sent (and withdrawn if earlier sent) to provider, peer, rs; sent to customer and rs-client with OTC stamped per Section 5 egress rule 1; the local route still reaches the provider | flipped subtest of `TestOTCEgressFilter` plus the `.ci` |
| AC-12 | AC-4 (D5) | under Q-D5 (a): BLACKHOLE-tagged route received from A, B lists 65535:666 in its `blackhole` container, C does not. Under (b): received route gains NO_EXPORT by default. Under (c): no behaviour change, claim narrowed | (a) B receives it, C receives nothing (withdrawn if earlier sent); an untagged control route reaches C. (b) and (c) as stated | per ruling: `.ci` plus unit |
| AC-13 | goals | the scenarios in "Interop and goal validation" | each passes against the named daemon and was shown red with the fix reverted and the image rebuilt (`interop-and-goal-validation.md`) | recorded red and green logs |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC4271WriteClearsLowNibbleEveryAttribute` (new, tagged RFC4271-4.3-3 positive) | `internal/component/bgp/reactor/rfc4271_attr_flags_egress_test.go` | AC-1a on `writeUpdateBody`, raw and non-raw | red at HEAD |
| `TestRFC4271WriteLeavesCleanFlagsUnchanged` (new, tagged RFC4271-4.3-3 negative) | same | AC-6 | green at HEAD (polarity guard) |
| `TestRFC4271RelayedLowNibbleClearedOnBothRails` (new, tagged RFC4271-4.3-3 positive, RFC4271-4.3-4 positive) | `internal/component/bgp/reactor/rfc4271_attr_flags_relay_test.go` | AC-1a, AC-7 through `forwardUpdateCore` and `reactorForwardRS` | red at HEAD |
| `TestRFC8092WriteRemovesRepeatedLargeCommunity` (new, tagged RFC8092-3-1 positive) | `internal/component/bgp/reactor/rfc8092_large_community_egress_test.go` | AC-8 repeat case, both scan paths | red at HEAD |
| `TestRFC8092WriteKeepsDistinctLargeCommunities` (new, tagged RFC8092-3-1 negative) | same | AC-8 distinct case, zero allocations | green at HEAD |
| `TestRFC8092PolicyAddOfPresentValueSentOnce` (new, tagged RFC8092-3-1 positive) | `internal/component/bgp/reactor/rfc8092_large_community_policy_test.go` | AC-2a through the forward rail with a policy add | red at HEAD |
| `TestMUPMalformedT1STTreatedAsWithdrawnAtIngress` (new, tagged 3.1.3.1-6, -7, -10 positive, one subtest per shape) | `internal/component/bgp/reactor/draft_mup_t1st_withdraw_test.go` | AC-3a, AC-3b, AC-3c over `applyRFC7606` | red at HEAD |
| `TestMUPWellFormedSiblingSurvivesMalformedT1ST` (new, tagged 3.1.3.1-6, -7, -10 negative) | same | AC-9 | green at HEAD |
| `TestMUPMalformedT1STWithdrawEdges` (new, untagged table) | same | AC-10 (i) to (iv) | red at HEAD for (i) to (iii) |
| `TestMUPValidatorRegistered` (new) | `internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_ingress_test.go` | the validator is registered for ipv4/mup and ipv6/mup and calls `ParseMUP` | red at HEAD |
| `TestRetainReportsWithdrawnSeparately` (new) | `internal/core/bgp/nlri/nlritype/nlritype_test.go` | registry: discarded and withdrawn NLRIs counted apart; a family with no validator unchanged | red at HEAD |
| `TestOTCEgressFilter` subtest `src_role_rs_to_provider_accept` renamed `src_role_rs_to_provider_suppress`, plus new subtests for rs to peer and rs to rs, and rs to customer and rs-client accept | `internal/component/bgp/plugins/role/rfc9234_otc_test.go` | AC-11 | red at HEAD; CHANGES A TAGGED UNIT (below) |
| D5 test per ruling | `internal/component/bgp/plugins/filter_community/rfc7999_blackhole_readvertise_test.go` (new, under (a)) | AC-12 | red at HEAD under (a) |

### RFC-tagged tests this spec changes (each needs owner approval, Q-5)

| Tagged unit | Tag | Change | Condition |
|-------------|-----|--------|-----------|
| `internal/component/bgp/plugins/role/rfc9234_otc_test.go::TestOTCEgressFilter` (both recorded entries) | RFC9234-3.1-1 | the untagged subtest `src_role_rs_to_provider_accept` inside the tagged unit flips its assertion from accept to suppress; three subtests added | only under Q-D4 (a) |
| `internal/component/bgp/plugins/filter_community/rfc7999_blackhole_propagation_test.go` (units tagged RFC7999-3.2-2 negative, "no propagation community is forced") | RFC7999-3.2-2 | the default it pins changes from `none` | only under Q-D5 (b) |

#### Independent audit of these changes (2026-10-13)

Source: `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/rfc-tagged-test-compliance-review.md`, section 6.
Re-checked against the test bodies at HEAD, `rfc/full/rfc9234.txt` Section 3.1 and `rfc/full/rfc7999.txt` Section 3.2; agreed on both rows.

| Tagged unit | Audit verdict | What the change owes |
|-------------|---------------|----------------------|
| `TestOTCEgressFilter`, subtest `src_role_rs_to_provider_accept` flipped to `src_role_rs_to_provider_suppress` | COMPLIANT under Q-D4 (a) | RFC 9234 Section 3.1: "Route Server Client (RS-Client):  MAY propagate any route learned from a Customer, or that is locally originated, to an RS.  All other routes MUST NOT be propagated." The Customer and Peer roles carry the same "All other routes MUST NOT be propagated" sentence, and a route learned from an RS-Client is not "learned from a Customer", so it MUST NOT go to a Provider, Peer or RS. Today the subtest has no tag line of its own (the unit's two tags sit on `TestOTCEgressFilter`'s other subtests, lines 661 and 671). The flipped subtest carries the D4 fix, so it gets its own `// RFC requirement: RFC9234-3.1-1 positive -- ...` line stating what its body checks (src-role `rs`, destination provider, suppressed), with its own `./le rfc discriminate-record` record. The three added subtests (rs to peer, rs to rs suppress; rs to customer and rs-client accept) each carry a tag line too |
| `TestRFC7999PropagationCommunityIsNeverForced` (RFC7999-3.2-2 negative) | COMPLIANT under Q-D5 (b) only if rewritten; unchanged and correct under (a) or (c) | RFC 7999 Section 3.2: "The community to prevent propagation SHOULD be chosen according to the operator's routing policy." Under (b) the default becomes `no-export`, so the prose "with the leaf set to none or left unset a BLACKHOLE route passes unchanged, so no fixed default is applied" becomes false, and so does the body's last assertion (`rfc7999Ingress(t, nil, payload)` returns nil). Under (b): remove "or left unset" and "so no fixed default is applied" from the claim, and move the unset-leaf case out of this negative (it becomes a positive of the new default). The explicit `none` case stays: an operator's `none` is still the operator's choice, so the negative still proves the SHOULD |

The audit also confirms unchanged and still correct: the RFC4271-4.3-3/4 and RFC8092
units, and the MUP decode-path units (their claims are about `ParseMUP`).

No other existing tagged unit changes. The decode-path units `TestMUPT1STMandatoryFieldsWithinLength` and `TestMUPT1STTLVFraming` (tagged 3.1.3.1-6, -7, -10) stay as they are: they prove `ParseMUP` refuses the shapes, which the new ingress validator relies on, and the new ingress tests carry the treat-as-withdraw claim. `TestRFC4271AttributeFlagsLowNibbleZeroOnSend`, `TestRFC4271AttrFlagsLowNibbleIgnoredOnReceive`, `TestLargeCommunities`, `TestLargeCommunitiesWriteToNoDuplicates` and the 3.1.3.1-9 ingress tests are untouched. Every new tag gets a `./le rfc discriminate-record` record, and every changed verdict a `./le rfc audit-stamp ... mode rejudge` by an agent that did not write the test (AC-5).

### Functional Tests
| Test | Proves |
|------|--------|
| `test/plugin/rfc4271-attr-flags-low-nibble-relay.ci` | AC-1a end to end on a normal and a route-server destination |
| `test/plugin/rfc8092-large-community-no-repeat-egress.ci` | AC-2a end to end |
| `test/plugin/mup-t1st-malformed-treat-as-withdraw.ci` | AC-3a end to end: earlier generation of K withdrawn at the receiving peer, L delivered |
| `test/plugin/rfc9234-rs-client-learned-not-upward.ci` | AC-11 (Q-D4 (a)) |
| `test/plugin/rfc7999-blackhole-readvertise-agreement.ci` | AC-12 (Q-D5 (a)) |

Each `.ci` is shown red against HEAD before the fix.

### Interop and goal validation

Per `docs/architecture/testing/interop.md`: scenario directories under `test/interop/scenarios/`, named, no numeric prefix, each with a typed checker in `internal/le/interoplab/bgp`. Malformed or unusual input is sent by the raw injector sidecar (172.30.0.9, `inject.msg`).

| Scenario (new) | Defect | Peer daemon | Assertion | Discrimination |
|----------------|--------|-------------|-----------|----------------|
| `bgp-attr-flags-low-nibble-gobgp` | D1 | GoBGP 3.31.0 (receiver) | the injector sends ORIGIN 0x4F and attribute 241 flagged 0xCF to Ze; GoBGP's JSON for the relayed path shows attribute 241 with flags 0xE0 | red with the egress clear reverted; A-2 |
| `bgp-large-community-no-repeat-gobgp` | D2 | GoBGP 3.31.0 (receiver) | Ze's export policy adds a value the route already carries; GoBGP's JSON lists each large community once | red with the egress removal reverted; A-3 |
| `bgp-mup-malformed-t1st-withdraw-gobgp` | D3 | GoBGP 3.31.0 (receiver, ipv4-mup) | GoBGP holds K after the well-formed generation, then loses K and holds L after the malformed UPDATE; the session stays up | red with the ingress validator unregistered; A-4 |
| `bgp-role-rs-client-learned-not-upward-frr` | D4 | FRR 10.4.1 (Ze's provider, `local-role provider`) | a route the injector sends on a session where Ze's role is `rs` never appears in FRR; a Ze-originated control route does | red with `rs` removed from the suppressed set; Q-D4 (a) |
| `bgp-rfc7999-blackhole-readvertise-frr` | D5 | FRR 10.4.1 (no agreement) and BIRD (sender, agreed) | FRR never holds the BLACKHOLE-tagged prefix; it holds the untagged control prefix | red with the gate reverted; Q-D5 (a) |

| Goal (Task section) | Evidence that proves it |
|---------------------|-------------------------|
| bits that must be zero are not sent | `bgp-attr-flags-low-nibble-gobgp` pass plus recorded red; AC-1a hex on both rails |
| duplicates that must not be sent are not sent | `bgp-large-community-no-repeat-gobgp` pass plus red; AC-8 producer-independent unit |
| a malformed MUP route is withdrawn, not kept | `bgp-mup-malformed-t1st-withdraw-gobgp` pass plus red; `.ci` showing the earlier generation withdrawn |
| RS-Client-learned routes stay off upward sessions (if ruled) | `bgp-role-rs-client-learned-not-upward-frr` pass plus red |
| BLACKHOLE reaches only agreed peers (if ruled) | `bgp-rfc7999-blackhole-readvertise-frr` pass plus red |
| verdicts reach `enforced` | AC-5: `rfc/audit/*.json` rows rejudged by an independent agent |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validation |
|----|------------|-------|----------|------------|
| A-1 | every UPDATE Ze writes to a BGP peer passes `Session.writeUpdateBody` | callers read 2026-10-13: `writeUpdateGated`, `writeOrdinaryUpdateBody` chunks, `writeRawUpdateBody`, `writeOriginatedRawUpdate`, `writeUpdateWithoutAIGP`, and the message path of the generic writer; `bufWriter.Write` appears only at line 183 (non-UPDATE) and 715 (inside `writeUpdateBody`) | a writer bypasses D1 and D2 | re-run the `bufWriter.Write` grep at implementation; a unit that fails if any UPDATE path skips the walk |
| A-2 | GoBGP's JSON shows an unknown attribute's flags octet | GoBGP models unknown attributes with their flags; not checked on 3.31.0 | the D1 scenario cannot discriminate | run the scenario with the fix reverted first; if GoBGP hides flags, use BIRD's `show route all` attribute dump or the compiled strict speaker's raw capture, and record the substitution |
| A-3 | GoBGP keeps repeated large communities on receipt | not checked; FRR removes them on receipt (`lcommunity_uniq_sort`, unverified) and so cannot discriminate | the D2 scenario cannot discriminate | same as A-2: red run first |
| A-4 | GoBGP 3.31.0 negotiates and shows ipv4-mup | GoBGP 3.x lists MUP families; not checked in Ze's image | no MUP interop peer among FRR, BIRD, GoBGP | check `gobgp global rib -a ipv4-mup` in the lab image first; if absent, record the gap and ask the owner which peer stands in |
| A-5 | an absent Source Address Length octet means "no source address" | `parseBodyT1ST` reads it so, matching ExaBGP; draft: "Source Address Length is 0 bytes then the Source address is not carried within the NLRI" | Ze accepts a shape the draft calls malformed | unchanged by this spec; recorded here so the validator does not silently tighten it |
| A-6 | a genuine egress filter reject withdraws on both rails | D6 owner decision (2026-10-03) and its landing | D4 and D5 suppression leaves earlier generations at the destination | AC-11 and AC-12 assert the withdrawal |
| A-7 | route-server transparency (RFC 7947) does not require the low nibble to be kept | RFC 4271 Section 4.3 binds every speaker on send; the nibble is unused and carries no route information | none known | owner may overrule under Q-3 |

### Risks
| ID | Failure mode | Early signal | Mitigation |
|----|--------------|--------------|------------|
| R-1 | a per-write attribute walk costs throughput on the hot path | `BenchmarkFanout*` and `./le perf` regress | one walk shared with the existing Label-Index walk; no allocation; benchmark before and after, numbers in the closing commit |
| R-2 | shrinking the body after the ownership filter would make duplicate-advertisement comparison see stale bytes | an ownership test sees a mismatch | run the walk at the existing `clearTransmittedLabelIndex` site, before the filter |
| R-3 | in-place compaction miscomputes a length (attribute, total path attribute, message) | peer NOTIFICATION, UPDATE Message Error | unit asserts all three lengths; the extended-length header width is kept |
| R-4 | the malformed-MUP withdrawal adds a second MP_UNREACH_NLRI | RFC 7606 Section 3(g) refusal at the peer | AC-10 (i): a separate withdraw-only body through the existing extra-body dispatch |
| R-5 | Ze-originated T1ST without TEID is withdrawn by every Ze receiver once D3 lands | MUP `.ci` tests that originate T1ST go red | Q-2; run `test/plugin/mup-*` and `test/encode/srv6-mup*.ci` early |
| R-6 | operators relying on RS-Client routes reaching providers lose them | operator report | Q-D4 decides; `docs/guide/bgp-role.md` row edited in the same change |
| R-7 | a test pinning byte-identical relay of a low-nibble attribute goes red | red unit in reactor or rs | read it: a test asserting the bits survive asserts the defect and is corrected under `rule-precedence.md` (wrong test), named in the commit |
| R-8 | the D3 validator runs `ParseMUP` per received MUP NLRI and allocates (`*MUP` per call) | allocation profile of MUP ingress | a non-allocating validate entry in `nlri/mup` that runs the same body checks without building `*MUP`; `ParseMUP` calls it, so one implementation holds the rules |

## Files to Modify

- `internal/component/bgp/reactor/session_write.go` - call the joined egress walk at the existing Label-Index site
- `internal/component/bgp/reactor/session_prefix_sid.go` - Label-Index clear becomes one step of the joined walk (no behaviour change)
- `internal/component/bgp/reactor/rfc8092_large_community.go` - in-place compaction for the egress side; reuse `largeCommunityRedundant`
- `internal/component/bgp/reactor/session_validation_nlritype.go` - malformed-withdraw outcome, withdraw body
- `internal/component/bgp/reactor/session_read.go` - dispatch the extra withdraw body when needed (reuse the treat-as-withdraw extra-body loop)
- `internal/core/bgp/nlri/nlritype/nlritype.go` - validator registration and the withdrawn count from `Retain`
- `internal/component/bgp/plugins/nlri/mup/rfc7606.go`, `register.go`, `types.go` - register the validator; non-allocating validate shared with `ParseMUP`
- `internal/component/bgp/plugins/nlri/mup/encode.go` or the MUP command parser - only under Q-2 (a)
- `internal/component/bgp/plugins/role/otc.go` - suppressed source roles (Q-D4 (a))
- `internal/component/bgp/plugins/filter_community/blackhole.go`, `egress.go` - agreement gate (Q-D5 (a)) or `yang/` default (Q-D5 (b))
- `rfc/short/rfc4271.md`, `rfc8092.md`, `draft-ietf-bess-mup-safi.md`, `rfc9234.md`, `rfc7999.md` - requirement-row annotations for the new proof; no new enrolment (RFC scope frozen 2026-10-08)
- docs per the Documentation checklist; new tests and `.ci` files as listed; interop scenario directories and checkers

## Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new leaf; under Q-D5 (b) only the default of `blackhole-propagation` changes in `filter_community/yang/` |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | No | none; under Q-2 (a) the MUP `update` parser gains a refusal, not a flag |
| CLI grammar (keyword before value) | N-A | no grammar change |
| Editor autocomplete | N-A | no new value |
| Functional test for new RPC/API | Yes | the five `.ci` files in the TDD plan |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no file, socket, port, module or binary |
| Prometheus counters/metrics | Yes | a counter of malformed MUP NLRI treated as withdrawn per peer, beside the RFC 7606 counters; D4 reuses `recordDrop(dropSourceRole, ...)`; names fixed at implementation and listed in Documentation row 14 |
| BGP family surface (new SAFI / capability / attribute) | N-A | no new family, capability or attribute; MUP is existing |

## Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the BGP role and MUP feature records under `features/` (evidence level changes); found with `./le feature report` |
| 2 | Config syntax changed? | No | only a default under Q-D5 (b): then `docs/guide/bgp-policy.md` |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | Yes | `docs/architecture/meta/role.md` (D4), `docs/architecture/meta/filter-community.md` (D5) |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-role.md` rule 2 row (D4); `docs/guide/bgp-policy.md` (D5) |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/attributes.md` "Attribute Flags" and "32. LARGE_COMMUNITY" (egress rule, anchors to the new walk) |
| 8 | Plugin SDK/protocol changed? | No | the `nlritype` registry is internal, not SDK |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | the five `rfc/short/` files; `docs/features/rfc-status.md` regenerates from them |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` Scenario Inventory gains the new scenarios |
| 11 | Affects daemon comparison? | No | no comparison row covers these behaviours |
| 12 | Internal architecture changed? | Yes | `docs/architecture/bgp/structural-forwarding.md` (final-writer normalization), `docs/features/bgp-protocol.md` (nlritype validator) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derive with `./le spec citation anchors spec plan/immediate/spec-bgp-update-propagation-rfc-defects.md` before the first code edit; `session_write.go` alone is anchored by 16 pages (`ai/CODE-TO-DOCS.md`). Pages DECLARED by the files this spec changes: `docs/architecture/core-design.md` (RFC 7606 validation section gains the per-NLRI malformed-withdraw outcome; its community-handler text is unaffected because the handler is not edited), `docs/architecture/wire/nlri.md` (MUP section: ingress validation now withdraws a malformed T1ST), `docs/features/srv6.md` (unaffected in content: the Label-Index clear moves into the joined walk with the same behaviour; its source anchor is updated to the joined function), `rfc/short/rfc7606.md` (Section 5.4 and treat-as-withdraw notes gain the MUP per-NLRI withdraw beside the BGP-LS discard) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/bgp-role.md` examples; verify against the role YANG after D4 |

### Discovery (`ai/rules/repo-maintenance.md`)

| Question | Answer |
|----------|--------|
| Where an agent looks first | `ai/INDEX.md` rows for BGP role, RFC 7606, wire attributes; `ai/CODE-TO-DOCS.md` for `session_write.go` |
| Rule that prevents regression | the new tagged tests under `./le rfc check` discrimination |
| Registry that prevents drift | `nlritype` registry (validator registered by the owning NLRI plugin) |
| Verification that proves it | the `.ci` files and interop scenarios above |

## Key Insights

- D1 and D2 are send-side sentences; the only point every send passes is `Session.writeUpdateBody`, which already normalizes the Label-Index on its private buffer. Fix there, one walk.
- D2's receive half is already fixed (`586887a063`); the live transmit producer is the community policy handler's Add.
- D3 needs a per-NLRI withdraw outcome in the existing typed NLRI pass; `ParseMUP` already knows every malformed shape; keyMUP keeps TLVs out of the key, so the withdrawal hits the earlier generation.
- D4 and D5 are owner rulings; the code change under the recommended reading is small (one role in a set; one egress gate reading `blackholecfg`).
- Ze's own T1ST encoder can emit a route its ingress will now withdraw (Q-2).

## Implementation Steps

1. Owner answers Q-D4, Q-D5, Q-1 to Q-5; record them here as `-> Decision:` with date and quote.
2. Read `docs/contributing/ze-go-style.md`; run `./le spec citation anchors` (Documentation row 16).
3. D1 and D2: failing units, then the joined egress walk; page edits for row 7 in the same step; benchmark.
4. D3: registry and validator units red, then the registry change, the MUP validator, the withdraw outcome and dispatch; `.ci`; R-5 run of the MUP functional tests.
5. D4 and D5 per ruling, with the tagged-unit change carrying `RFC-approved:` trailers.
6. Interop scenarios: red with the fix reverted and the image rebuilt, then green.
7. `./le rfc discriminate-record` for every new tag; independent `audit-stamp ... mode rejudge` for the eight verdicts (AC-5).
8. Gates owed to the main thread: `./le go lint run`, `./le test unit all`, the `test/plugin` functional suite, `./le verify worktree`, `/ze-review`.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`

## Review Gate

### Round 1 (link-local and withdrawal, 2026-10-03 and 2026-10-04)

| Item | Finding | Disposition | Evidence |
|------|---------|-------------|----------|
| B1, N1 | a refused destination of a mixed UPDATE lost the other fields' routes | fixed: per-section redo (`withdrawalBySection`, `forwardBySection`, `reactorForwardRSSection`), chosen over a blanket split, which broke about 14 single-batch harness tests | 28ab406700; the three new tests red with the fix off |
| I1 | `:: then fe80::x` cut to its Global reached a multihop peer as `::` | fixed: `nextHopValue.globalUnusable` | 28ab406700 |
| I2 | the spec said BIRD also withdraws unconditionally | corrected in "D6 design": BIRD withdraws only exported routes; homed in `spec-bgp-withdraw-only-exported-routes.md` | this spec |
| I3 | the withdrawal was converted per destination | fixed: `fwdWithdrawal` builds once per base, `buildWithdrawalPayload(payload, buf) int` | `BenchmarkFanoutWithdraw`, count 3: n=100/g=100 515 to 208 allocs/op and 148.7 KB to 118.8 KB per op; n=1/g=1 8 allocs either way, 2.5 KB to 6.9 KB per op (unverified: the 4 KB read-pool buffer the bench never returns). ns/op is not comparable: load average 14 during the after run |
| I4 | the RS reflection test passed with `reflected` forced false | fixed: the speaker is attached to both segments, so only the reflection gate refuses the client | 94c9a3d3c6; red against mutant I |
| I5 | the VPN Link-Local-only test was not shown red | shown red against mutant B (the off-link gate skipping SAFI 128) | 94c9a3d3c6 body |
| I6 | no test proved an AS_PATH resolve failure costs a withheld destination nothing | fixed: `TestASPathResolveFailureCostsWithheldDestinationNothing`, both rails | 94c9a3d3c6; red against mutants D and E |
| I7 | owner approvals for tagged units changed in 28ab406700 not recorded | 28ab406700 carries no `RFC-approved:` trailer, and a landed commit is not amended. Thomas approved on 2026-10-04 ("Approve") the RFC 1997 well-known community units changed through the `wkForwardParts` helper: `reactor.TestForwardNoExportStillWithdrawsFromExternalPeer`, `reactor.TestForwardNoExportWithdrawsAnAnnouncementOnlyUpdate`, `reactor.TestForwardRSHonorsWellKnownCommunities`, `reactor.TestForwardRSWithdrawsFromRefusedClient`. `./le commit audit` reports the file WEAKENED for good, because it never reads trailers (journaled in `plan/journal/check-cannot-see-the-change-it-looks-for.md`, 2026-10-03) | this record |
| N2 | `peerOnLink` is any connected subnet; the draft's Section 4 MAY for an internal peer needs the source on the same interface | journaled in `plan/journal/escape-hatch-scoped-wider-than-its-justification.md`, 2026-10-04 | journal row |
| N4 | the withhold warning named neither next hop nor family | fixed | 28ab406700 |
| N5 | the Interop row called the FRR scenario iBGP | corrected: eBGP; the internal case is unit-only | this spec |
| cap 77 | a received Link-Local-only next hop crossed unchanged to a peer without capability 77 | landed in 2987e2c517: `egressNextHopLinkLocalOnlyRefused` asks the emitted next hop, written or received. Fixtures `llnhClient` and `llnhExternalPeer` negotiate capability 77; owner approvals and refreshed discrimination records accompanied the landing. Audit rejudgment followed in 9ac8bd76fb | `TestReceivedLinkLocalOnlyWithdrawnFromPeerWithoutCapability` red without the gate change on both rails |

### Round 2 findings and round 3 closure (D6 only, 2026-10-04)

| Severity | Finding | Source repair | Evidence |
|----------|---------|---------------|----------|
| BLOCKER | Mixed-field retry reran raw policy on the original source, losing the judged replacement and preceding edits; a policy that recreated mixed output could repeat indefinitely | `reactor_api_forward.go::forwardUpdateSection` materializes the actual post-policy output once and continues over its bounded field partition; `forwardBySection` was removed | `TestRawExportMixedSectionsPreservePolicy`: rewritten legacy prefix and LOCAL_PREF survive, MP route withdrawn, one raw callback; received-AIGP case retains the original metric baseline |
| ISSUE | AS_PATH materialization before partitioning could drop withdrawals together with an unencodable sibling announcement | `forwardUpdateSection` asks gates and resolves announcement AS_PATH per section, after preserving policy output | `TestRawExportMixedBadASPathStillWithdraws`: malformed AS_SET rejects the announcement while source IPv4 and converted IPv6 withdrawals both survive |
| ISSUE | RS retry used a linear membership search for every current peer | `forward_rs.go::reactorForwardRSSection` scans current peers once, then visits selected current identities with live facts and no active export filter | `TestReactorForwardRSRetrySelection` pins removed/replaced/inactive/filtered exclusions, source reflection facts and emitted NLRI; `BenchmarkReactorForwardRSMixedRetry` proves retry reachability before timing |
| ISSUE | FRR proof was described as a physically multihop topology | Checker/configuration comments and `docs/architecture/testing/interop.md` now say off-connected-subnet, no transit router | `checkLinkLocalOnlyMultihopWithdraw` still proves FRR receives withdrawal; `clampedPath.wireIPv6` supplies the distinct transit-router topology |

Round 3 reviewed only those repairs and affected siblings, frozen in
`linklocal-round3.diff`. Independent reviews by `ReviewPolicyContinuation` and
`ReviewRetryProof` report **0 BLOCKER / 0 ISSUE**, including protocol/dataflow,
guards/security/performance, proof/removed-behavior and Go-style passes.
The D6 closure owner read the producing functions and assertions independently;
no additional product finding arose. Closure prose does not add a review round.
The native hash-pinned artifact is
`tmp/review/bgp-update-propagation-rfc-defects-01a10694-3795-71f2-8250-25e3a877f4cf.md`,
recorded CLEAN at round 3 over the 21-file closure population.

## D6 scoped closure evidence

This closes the D6 implementation and owner-authorized withdrawal behavior,
not this six-defect skeleton. D1–D5, AC-1–AC-4, and the non-D6 portion of AC-5
remain open. The generic unchecked tables above still describe that unfinished
whole-spec work; they are not evidence against or claims about the D6 slice.

Evidence roots:
- `S` = `tmp/session/2026-10-04-01a10694-3795-71f2-8250-25e3a877f4cf/scratch`.
- `P` = `tmp/session/2026-10-03-ff3776cb-ce05-4491-9d8f-ba31bcc36641/scratch`.

### D6 deliverables and goal validation

| Deliverable / goal | Producer and proof | Result |
|--------------------|--------------------|--------|
| Off-link LL-only refusal, usable-Global controls, lost self next hop, every owner-authorized withhold gate | `egressNextHopWithheld`, `buildWithdrawalPayload`, both forward rails; tagged draft 4-1/4-2/4-4/4-7/4-9 tests and per-gate withdrawal tests | Implemented; draft rejudged in 9ac8bd76fb. Cap77 fix landed in 2987e2c517 |
| Mixed-family isolation and source withdrawals | `withdrawalBySection`, `forwardUpdateSection`, `reactorForwardRS`; mixed-family, per-field, unspecified-Global, VPN and resolver-failure tests | Exact withdrawal bytes and unrelated announcement controls preserved; historical reds in `P/job-mixred-8402b24a.log` and `P/job-i6-{D2-83e7e3ac,E2-38d9d216,green-78a71cb5}.log` |
| RR client on another advertiser segment, not merely off-link | `egressNextHopWithheld`; `TestRouteServerReflectedLinkLocalOnlyWithdrawnFromClientOffTheSegment` uses two connected segments and requires `destOnLink` | Reflection gate isolated; `P/job-mut-I3-1d29f80c.log` observed red |
| Post-policy partition, retained edits and withdrawals despite malformed AS_PATH | `forwardUpdateSection`; both `TestRawExportMixed*` tests in `forward_policy_section_test.go` | Reds in `S/job-linklocal-policy-red-fixed-proof-54107ea0.log` and `S/job-linklocal-policy-as-set-red-d651f068.log`; full Linux reactor race run passes both |
| Raw policy through a live daemon and external filter RPC | `forwardUpdateSection` reached by single-field ingress whose raw export result is mixed; smoke uses explicit `manual-eor true` | Actual pre-fix rebuilt overlay of 7e6a3cefe2: 0 passed / 1 failed, eight callbacks then bounded IPC failure and no route outputs. Final fixed binary: 1 passed / 0 failed in 1.1s, exact rewritten legacy NLRI and MP withdrawal, forbidden LL and original prefix absent. `S/linklocal-runtime-observations.json`, jobs bg_83/bg_86 |
| Genuine routed IPv6 hop and replacement of an earlier generation | `test/plugin/linklocal-only-multihop-withdraw.ci`, fixture `clampedPath.wireIPv6` / `runWithPeers`; Global generation, withdrawal, control route, permanent LL rejection | Historical discriminating red `P/job-llci-red3-d732bee5.log`; restored green `P/job-llci-green2-d732bee5.log` and fixture regression `P/job-fnll-regress2-8b1b0523.log` |
| Real FRR receiving withdrawal, not merely rejecting a leaked LL announcement | `checkLinkLocalOnlyMultihopWithdraw`: subject announcement count, explicit withdrawal, LL absence, table removal, session stays up | Post-fix 1 passed / 0 failed in 120.63s against FRR 10.4.1; Ze image `sha256:5759abff43a6e5d7136e6e46b7f564374bf64cb3bc4e4be029bd47e4697ef15a`, `S/linklocal-runtime-observations.json`. Historical distinct red/green images and logs in `P/llint/` |
| Bounded retry work without a membership-map allocation | `reactorForwardRSSection`, `BenchmarkReactorForwardRSMixedRetry` | n=2000 median 5.646ms to 3.448ms; 18,081 allocs/op unchanged. `S/job-linklocal-rs-baseline-coherent-21a0303f.log` and `S/job-linklocal-rs-after-21a0303f.log`; not a claim of allocation-free forwarding |

### D6 security, ownership and documentation

| Concern | Source-aware disposition |
|---------|--------------------------|
| Raw output causes recursion or policy bypass | The general rail runs policy once for the decision and partitions its actual output; continuation is bounded by NLRI-bearing fields and cannot call policy. Each announcement section meets next-hop gates and AS_PATH resolution |
| Buffer reuse or losing source identity | `SplitWireUpdate` owns multi-field sections before intermediate pool return; its single-field alias keeps the buffer until rebuild/dispatch. `forwardUpdateSection` preserves source identity on policy replacement and rebuild; section items use the existing pending-item retention and dispatch path |
| Stale peer pointer or filter bypass on retry | `reactorForwardRSSection` checks the current peer-map identity, live forwarding facts and active export filters while preserving source classification and skipped-policy reporting |
| Documentation | `docs/architecture/bgp/structural-forwarding.md` describes post-policy section partition, AIGP baseline and O(N+K) selection with producer anchors. `docs/architecture/testing/interop.md` separates off-subnet FRR proof from routed-hop `.ci`. No new product configuration, command, RPC or runtime dependency is introduced by the round 3 repairs |

### D6 RFC records and verification

| Gate / record | Observed result and scope |
|---------------|---------------------------|
| Linux reactor package with race detector | PASS, 151.435s: `S/job-linklocal-linux-reactor-cgo-26ecb2d1.log`, including both new raw-policy tests and RS selection guard |
| Owned audit freshness | No owned STALE unit needed new judgment. Eleven SHIFTED verdicts mechanically re-sealed using exact native payloads after source-byte comparison: draft 4-5/4-6/4-8, RFC1997 Well-1/2/3/4, RFC4271 5.1.3-1, RFC4456 8-1/8-2, RFC8950 4-1. `S/linklocal-reseal-stage/native-reseal.log` |
| Changed-producer discrimination | Eight native observed-red records renewed: RFC1997 Well-1 negative (`buildWithdrawalPayload`); RFC4271 5.1.2-2 export negative and unmodified-path positive/negative, 9.2-6 negative; RFC9552 5.1-3 positive (`forwardUpdateSection`); RFC4456 8-4 positive/negative (`reactorForwardRS`). `S/linklocal-discrimination-full.log` |
| Integrated RFC check | 92 residuals versus 111 before the scoped refresh; no owned D6 residual. `S/linklocal-rfc-full.log`. RFC7705's separately changed unit and unrelated RFC7947 rows were deliberately not refreshed |
| Repository and commit audit | Parent observed the same two previously journaled number-parsing repository issues; current commit audit reports no changed existing tests. New tests were independently reviewed; no new tagged-unit change requiring owner approval |
| Documentation checks | Parent regenerated the two ignored derived indexes; documentation check retains 29 existing published-catalog drifts already recorded during ADD-PATH closure, not a clean global documentation result |
| Native review artifact | `tmp/review/bgp-update-propagation-rfc-defects-01a10694-3795-71f2-8250-25e3a877f4cf.md`: CLEAN, round 3, 21 files; checked by the native review gate before commit |
| Final integrated Linux verification | Still owed after D6 and startup-race closure, in the handover's order; not run early and not represented by the scoped reactor PASS |

### D6 scope boundaries and remaining integration

The separately owned next-hop-auto rewrite remains in
`plan/immediate/spec-bgp-next-hop-auto-rewrites-for-ebgp.md`; tracking exported
routes before withdrawal remains in
`plan/immediate/spec-bgp-withdraw-only-exported-routes.md`. Neither is silently
included or claimed complete here. The internal non-RR same-interface condition
recorded in round 1 remains outside this multihop/withdrawal closure.

The raw smoke's earlier extra-EOR counter discrepancy remains an unverified
observation, not a fixed product defect or successful proof; the parent recorded
it in `plan/journal/false-synchronization-claim.md`. The final discriminating
smoke uses the explicit manual-EOR configuration above. The seven disposable
smoke draft and overlay source files were removed after red/green proof;
permanent regression tests and runtime evidence remain. No D1–D5 work,
whole-spec closure or skeleton deletion is authorized by this D6 record.
