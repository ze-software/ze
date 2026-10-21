# Spec: bgp-update-propagation-rfc-defects

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-10-14 |

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
residual: the weak verdicts live in the audit JSON and in the "Blocked by" table of
the BGP child spec-rfc-verdict-fix-bgp, whose rows pointed here. No other
session holds this spec: no state file under `tmp/session/*/state/` names it, and
its last commit is `d09deeecf3` (2026-10-04, D6 closure).

| ID | Verdict at HEAD | Producer at HEAD | Still open? |
|----|-----------------|------------------|-------------|
| D1 | RFC4271-4.3-3 weak, RFC4271-4.3-4 weak | `reactor/forward_body.go::forwardOpaquePayload` edits only the drop and Partial cases; `forwardWire` returns the received bytes unchanged when `preserveOpaque` (route-server clients); `reactor/session_write.go::writeUpdateBody` copies every body into the session's private buffer and edits only the Prefix-SID Label-Index (`session_prefix_sid.go::clearTransmittedLabelIndex`). A grep for a low-nibble mask over `attribute/`, `reactor/`, `wireu/`, `message/` finds none | Yes |
| D2 | RFC8092-3-1 weak | Receive half FIXED in `586887a063` (2026-09-30): `reactor/rfc8092_large_community.go::removeRedundantLargeCommunities`, called from `publishBase` (`session_validation.go`), removes repeats at ingest. That is RFC8092-3-2's sentence. The transmit half is open at a different producer from the one the Defects row names: `filter_community/handler.go::genericCommunityHandler` appends every `AttrModAdd` value without asking whether the attribute already holds it, so an export policy adding a value the route carries, or adding one value twice, transmits it twice. A second producer appends the same way: `filter_community/filter.go::ingressTagCommunities`, reached from the ingress tag loop, the RFC 9234 relation tag and the blackhole tag in `blackhole.go`. The same defect is recorded in `plan/journal/non-idempotent-append-duplicates-the-record.md` (row 2026-10-13, from `spec-daemon-drain-before-stop`), whose Fix cell planned a skip at the handler; the D2 egress walk replaces that handler fix (D2 decision under Current Behavior), and the row now points here. The row's `parseLargeCommunityWire` lead is closed: `reconstructRoute` replays through text commands whose encoder (`LargeCommunities.WriteTo`) removes repeats, and `packEventAttrs` builds Adj-RIB-Out storage, not wire | Yes, narrowed to the transmit half |
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

Q-D4 is D4's question and Q-D5 is D5's. Q-5 is answered by the owner on 2026-10-13. Every other row is answered by the owner on 2026-10-14, "Do what you think is right", which accepts each recommendation: Q-D4 (a), Q-D5 (a1), Q-1 (a), Q-2 (a), Q-3 (a), Q-4 (a). No question is open.

| ID | Question | Options | Recommendation and reason | Blocks |
|----|----------|---------|---------------------------|--------|
| Q-D4 | Is a route learned from an RS-Client (Ze's local role `rs`) propagated to a Provider, Peer or RS? | (a) No: follow RFC 9234 Section 3.1, add `rs` to the suppressed source roles. (b) Yes: keep HEAD, and re-scope RFC9234-3.1-1's claim to exclude the RS-learned case with the reason recorded | **Owner, 2026-10-14: accept (recommendation): (a).** Section 3.1, quoted below, makes "All other routes MUST NOT be propagated" for the Customer, RS-Client and Peer roles, and an RS-Client is not a Customer. Section 5's OTC text describes what OTC detects, not what Section 3.1 permits, and Section 5 ingress rule 3 adds no OTC to a route received from an RS-Client, so without (a) nothing stops the route | AC-4, AC-11; flips the tagged subtest named in the TDD plan |
| Q-D5 | Which reading of RFC 7999 Section 3.1 applies to a RECEIVED BLACKHOLE route, and on which sessions? | (a1) "advertising it" covers re-advertisement on a BILATERAL eBGP session only: withhold (and so withdraw, or replace with an eligible survivor, A-6) a BLACKHOLE-tagged route from a destination that is eBGP (the new `PeerFilterInfo.IsIBGP` is false), is not a route-server client (`RSClient` false), and whose `blackhole` container does not agree to the carried community (`blackholecfg.Rule.Agreed`). `IsIBGP` is filled from `PeerSettings.IsIBGP` (`peer_settings.go`), whose one rule is `PeerSettings.isIBGPWith` (`session_as_migration.go`), so an RFC 7705 migration session is iBGP here as everywhere else; the gate MUST NOT compare `PeerAS` with `LocalAS`, the open-coded copy `plan/journal/helper-bypassed-by-an-open-coded-copy.md` records (row 2026-10-13). iBGP and RR sessions keep receiving the route, so a customer's RTBH signal still spreads inside the AS, which RFC 7999 Section 3.2 assumes ("to prevent propagation of the prefix outside the local AS"). Route-server-client destinations are exempt, because Section 3.1's next sentence hands the multilateral case to "the operator's routing policy". Ze has no confederation config, so no peer is a confederation member AS: every non-iBGP peer is external, the same split the NO_EXPORT gate `wireu.WellKnown.AllowsEgressTo` makes. (a2) as (a1) on EVERY bilateral session, iBGP included, route-server-client destinations still exempt: an iBGP or RR session without a `blackhole` container listing the community stops receiving the route, which breaks RTBH distribution inside the AS unless every internal session is configured. (b) Section 3.1 binds origination only; the propagation question is Section 3.2's SHOULD, and `blackhole-propagation` default changes from `none` to `no-export`: every operator's received BLACKHOLE routes gain NO_EXPORT, which stops them at every eBGP peer, agreed or not, and passes iBGP (RFC 1997 NO_EXPORT, `wireu.WellKnown.AllowsEgressTo`). (c) Section 3.1 binds origination only and the default stays `none`; RFC7999-3.1-2's claim is narrowed to origination with the reading recorded: no behaviour change. Under (a1) and (a2) `filter_community` gains a config consumer it lacks today: its `filterConfig` holds only `blackholePropagation`, and `blackholecfg` is imported only by rib, rpki and cmd/announce. The map `blackholecfg.Parse` returns is cached beside `peerConfigs` and not inside `filterConfig`. (Design review round 3, I-3.) The gate runs once per route per destination on the egress hot path, so its checks are ordered cheapest first and the lookup allocates nothing: (1) a destination with `IsIBGP` or `RSClient` true passes at once, with no scan and no lookup; (2) `blackholecfg.Carries` over the route's COMMUNITIES value, against the BLACKHOLE community (`attribute.CommunityBlackhole`) passes every untagged route with no lookup; (3) only then the destination's rule is looked up, address first, then name, then group, the order `configjson.LookupPeerConfig` uses, and `Rule.Agreed` judges the carried value. `PeerFilterInfo.Address` is a `netip.Addr` while `Parse`'s keys are strings (`PeerConfigKey.ID`), and calling `LookupPeerConfig` with `dest.Address.String()` would allocate per route per destination (`performance.md`). So the cache is re-keyed once, when it is built at config time, split on `PeerConfigKey.Template` and never on whether the ID parses as an address (a group may be named `10.0.0.1`, which `naming.ValidateNodeName` accepts): non-template entries go into a `netip.Addr`-keyed map (every non-template entry is an address by `blackholecfg.Parse` construction, which keys a peer by its remote IP and refuses one without it, so the `dest.Name` lookup always misses, which reads as not agreed), and the name and group entries stay in a `PeerConfigKey`-keyed map, read with the `dest.Name` and `dest.GroupName` strings `PeerFilterInfo` already carries (`configjson.GroupKey` builds a struct, no allocation). A miss means not agreed, as in `agreedSelector`. The cache does not pass through `configureCommunityFilter`'s `hasAnyRule` drop, so a peer whose only statement is a `blackhole` container keeps its entry, and the gate runs before `egressFilter`'s early return for a destination with no `filterConfig`. The cache follows reload: an `OnConfigVerify` parses the candidate, an `OnConfigApply` swaps the staged map in, and an `OnConfigRollback` restores the previous one (AC-12a). The egress check reads the cache and never parses config per UPDATE, unlike `agreedSelector` | **Owner, 2026-10-14: accept (recommendation): (a1).** Section 3.1 requires the use to "be agreed upon by the two networks" in "a bilateral peering relationship": an iBGP session is one network, and a route-server session is the multilateral case the same paragraph excludes, so the MUST reaches exactly the bilateral eBGP session. It states the agreement in terms of the community, not of who created the route, so it reaches a re-advertisement. (a2) over-applies it inside the AS, (b) changes a default for every operator, and (c) leaves the MUST unmet on re-advertisement | AC-4, AC-12, AC-12a |
| Q-1 | D3 scope: does the ingress MUP check run all of `ParseMUP`, or only the three owned shapes? | (a) all of `ParseMUP`, which also enforces the `{gap}` rows 3.1.3.1-1, -2, -3, -4, 3.1.1-1, 3.1.1-2, 3.1.2-1, 3.1.3-1, 3.1.4-1, 3.1.4.1-1, -2 at ingress. (b) only Length overrun, one leftover octet and TLV overrun. Under either answer an NLRI whose key `keyMUP` cannot form is discarded (AC-10a), so (b) still drops an IPv6 T1ST with Prefix Length 129 or an ISD with a trailing octet; what (b) keeps, installs and relays is every other shape `ParseMUP` refuses whose key forms | **Owner, 2026-10-14: accept (recommendation): (a).** One validator already exists; (b) needs a second, partial parser beside it, which is the duplicate declaration `principles.md` forbids. It does widen this spec's scope, so it needs the owner | AC-3 |
| Q-2 | Ze-originated T1ST without a TEID or endpoint: fix here or journal? | (a) refuse the route at the `update`/config entry point with an error naming the missing field, in this spec. (b) journal row, out of scope: under either Q-1 answer, two Ze speakers withdraw each other's TEID-less or endpoint-less T1ST routes once D3 lands, and the MUP functional tests that originate such routes go red (R-5) | **Owner, 2026-10-14: accept (recommendation): (a).** A T1ST originated with no `teid` and no `endpoint` carries only QFI after the prefix: `encode.go::writeT1STData` writes no TEID when `teidBits` is 0 (`parseTEIDWithBits("")` returns 0 bits) and no Endpoint Address Length when the endpoint is unset. `types.go::parseBodyT1ST` then needs 6 octets after the prefix (TEID, QFI, Endpoint Address Length) and returns `ErrMUPTruncated`; a missing endpoint alone leaves 5 and fails the same way. That is the draft's "< 0: the mandatory fields exceed the declared Length" shape, 3.1.3.1-6, which Q-1 (b) keeps in scope too | AC-3 interop |
| Q-3 | Do the D1 and D2 egress normalizations apply to the explicit raw diagnostic write (`writeUpdateBody` with `raw` true)? | (a) yes, as `clearTransmittedLabelIndex` already does. (b) no: raw stays byte-exact for fault injection | **Owner, 2026-10-14: accept (recommendation): (a)**, for uniformity with the existing Prefix-SID normalization at the same spot. If an operator needs non-conformant bytes for a lab, that is a separate, named feature | AC-1, AC-2 |
| Q-4 | D2: remove repeats at egress for LARGE_COMMUNITY only, or also for COMMUNITIES and EXTENDED COMMUNITIES? | (a) LARGE_COMMUNITY only. (b) all three | **Owner, 2026-10-14: accept (recommendation): (a).** Only RFC 8092 Section 3 forbids transmitting a repeat; RFC 1997 and RFC 4360 do not, and changing their bytes changes behaviour no RFC asks for | AC-2 |
| Q-5 | Approval for the RFC-tagged units this spec changes (list in the TDD plan) | approve, or rule otherwise | **Answered by the owner, 2026-10-13: "approve all RFC-tagged test changes".** Q-D4 (a) and Q-D5 (a1) were answered separately on 2026-10-14. The approval is blanket, so it covers every unit in "RFC-tagged tests this spec changes", the mechanical verdict-type edits of D3 included; each unit still needs its own `./le rfc approve unit` command. The `TestOTCEgressFilter` change applies Q-D4 (a), the `TestRFC7999PropagationCommunityIsNeverForced` change under Q-D5 (b); under the other answers that unit is not edited and its row goes unused. Commands under "Owner approval commands" in the TDD plan | AC-5 |

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md`, `rfc/short/rfc8092.md`, `rfc/short/rfc9234.md`, `rfc/short/rfc7999.md`, `rfc/drafts/draft-ietf-bess-mup-safi.txt` Section 3.1.3.1

### RFC text read for this design (whole sentences)

- [ ] `rfc/full/rfc4271.txt` Section 4.3, the Attribute Flags description: "The lower-order four bits of the Attribute Flags octet are unused. They MUST be zero when sent and MUST be ignored when received."
  -> Constraint: the four bits are 0x0F of the flags octet. Extended Length (0x10), Partial (0x20), Transitive (0x40) and Optional (0x80) MUST NOT be touched by the fix. The sentence binds every attribute, recognized or not.
- [ ] `rfc/full/rfc8092.txt` Section 3: "Duplicate BGP Large Community values MUST NOT be transmitted. A receiving speaker MUST silently remove redundant BGP Large Community values from a BGP Large Community attribute." Earlier in the same section: "There is no significance to the order in which twelve-octet Large Community Attribute values are encoded in a Large Communities attribute, A BGP speaker can transmit them in any order."
  -> Constraint: the first sentence (RFC8092-3-1) is about what goes on the wire, whatever produced it; the second (RFC8092-3-2) is met at ingest already. Keep-first order is a choice, not an obligation; the ingest helper already keeps first, and the egress fix keeps the same rule so one value set never has two orders.
- [ ] `rfc/drafts/draft-ietf-bess-mup-safi.txt` Section 3.1.3.1, the remaining-octets procedure: "To determine their presence, compute the number of remaining octets by subtracting, from the Length field in the MUP NLRI common header, the offset of the end of the Source Address measured from the start of the Route Type specific field." Then "< 0: the mandatory fields exceed the declared Length; the NLRI is malformed; MUST be treated as Treat-as-withdraw." "1: encoding is invalid (a valid TLV requires at minimum a Type byte and a Length byte); MUST be treated as Treat-as-withdraw." ">= 2: one or more TLVs are present. Parse TLVs one by one per Section 3.1.5; unknown TLV types MUST be ignored for local processing and MUST be propagated unchanged when re-advertising the route to other BGP peers; any TLV parsing error MUST result in Treat-as-withdraw; TLVs are not part of the NLRI key for route processing." The sibling rules of the same section add: "A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message."
  -> Decision: treat-as-withdraw is per NLRI. The malformed NLRI leaves MP_REACH_NLRI and is withdrawn; well-formed siblings in the same UPDATE are processed normally.
  -> Constraint: TLVs are not part of the key. `nlrisplit/mup_key.go::keyMUP` already keys a T1ST on RD, prefix length and prefix, so a withdrawal built from the malformed NLRI's bytes removes the earlier well-formed generation of the same route.
  -> Decision: (AC-10 (iv), design review rounds 1 and 2, I-2) ANY MUP NLRI whose key cannot be formed (`keyMUP` returns `errPrefixKey`: the RD or prefix overruns the NLRI, an ISD or T1ST Prefix Length above 128, an ISD with octets past its prefix, a DSD that is neither 12 nor 24 octets, or the native Length disagrees with the carved size) is DISCARDED, whatever the body checks would say and under either Q-1 answer. The MUP recognizer asks `nlrisplit.GetPrefixKey` first (after stripping an RFC 7911 identifier) and answers withdraw for an unkeyable NLRI, because an unkeyable NLRI is malformed by construction. -> Constraint: (design review round 3, I-2) `keyMUP` also returns `errPrefixKey` when `len(scratch) < 3+keyLen`, and the largest MUP key is 32 octets (a T2ST: 3 octets of architecture type and 2-octet route type (`keyMUP` copies `raw[:3]`) plus RD 8, Endpoint Length 1 and 20 octets for the 160-bit maximum), so a short scratch would discard a valid route as unkeyable, the silent wrong answer `principles.md` forbids. The recognizer therefore receives a caller-owned scratch of `nlrisplit.PrefixKeyScratchSize` (64) octets, the constant every other `GetPrefixKey` caller already sizes its scratch with: a fixed-size array field of `Session`, used only by `typedNLRIEdit` on the session's receive path (`processValidatedMessage` through `applyRFC7606`), passed as a slice through `nlritype.Retain` into the `Recognizer` call. A local array passed through the func value would escape to the heap once per MUP NLRI, which R-8 rules out; `typedNLRIEdit` turns a withdraw verdict whose key cannot form into the counted discard. So the discard is key-driven and nothing central names MUP. Such an NLRI leaves MP_REACH_NLRI, is counted and logged, and no withdrawal of it is relayed. Governing text: RFC 7606 Section 2 defines treat-as-withdraw as the UPDATE being "treated as though all contained routes had been withdrawn just as if they had been listed in the WITHDRAWN ROUTES field (or in the MP_UNREACH_NLRI attribute if appropriate) of the UPDATE message, thus causing them to be removed from the Adj-RIB-In according to the procedures of [RFC4271]." The obligation is the Adj-RIB-In removal; a withdrawal reaches another peer only through the Decision Process, when a route Ze advertised loses its best path. The draft adds "A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message", and neither text asks for the malformed bytes to be relayed. With no key, no Adj-RIB-In entry can match, because the key-driven discard installs no unkeyable MUP NLRI under either Q-1 answer, so the removal is empty and there is nothing to withdraw downstream. Relaying it would also fail at the writer: `adjOutWrite.key` calls `fwdPathKeyFor`, which returns the key error; `allow` returns it, `ownership.filter` fails inside `writeUpdateBody`, and the forward worker (`forward_pool.go`, the `writeUpdateGated` loop) logs "forward batch write failed", records the error on the batch's recovery items and returns before `flushWrites`, so every later item of that batch to that destination is abandoned. Malformed NLRIs whose key forms (3.1.3.1-6, -7, -10 with an intact RD and prefix) are withdrawn as in AC-3a. RFC 7606 Section 2 is still met: the UPDATE is processed and Ze holds no route for the malformed NLRI.
- [ ] `rfc/full/rfc9234.txt` Section 3.1, the role list: "Customer: MAY propagate any route learned from a Customer, or that is locally originated, to a Provider. All other routes MUST NOT be propagated." Same shape for "Route Server Client (RS-Client)" towards an RS and "Peer" towards a Peer. "If the local AS has one of the above Roles (in the order shown), then the corresponding peering relationship with the remote AS is Provider-to-Customer, Customer-to-Provider, RS-to-RS-Client, RS-Client-to-RS, or Peer-to-Peer". Section 5 ingress rule 3: "If a route is received from a Provider, a Peer, or an RS and the OTC Attribute is not present, then it MUST be added with a value equal to the AS number of the remote AS."
  -> Constraint: the role named in Section 3.1 is the LOCAL role towards the destination. "learned from a Customer" is a source session where Ze's local role is `provider`. A source where Ze's local role is `rs` is an RS-Client, not a Customer.
  -> Constraint: no OTC is added to a route received from an RS-Client (ingress rule 3 lists Provider, Peer, RS only), so the wire-bytes OTC check cannot stop it; only the source-role check can.
- [ ] `rfc/full/rfc7999.txt` Section 3.1: "In a bilateral peering relationship, use of the BLACKHOLE community MUST be agreed upon by the two networks before advertising it. In a multilateral peering relationship, the decision to honor or ignore the BLACKHOLE community is to be made according to the operator's routing policy." Section 3.2: "A BGP speaker receiving an announcement tagged with the BLACKHOLE community SHOULD add the NO_ADVERTISE or NO_EXPORT community as defined in [RFC1997], or a similar community, to prevent propagation of the prefix outside the local AS."
  -> Constraint: RFC 7999 is Informational; the sentence still carries a MUST and is enrolled. Q-D5 (a1), owner 2026-10-14: the agreement reaches re-advertisement on a bilateral eBGP session.

### Architecture pages
- [ ] `docs/architecture/bgp/structural-forwarding.md` "Four costs" and following: both rails key body reuse on opaque-attribute treatment; route-server clients get `PreserveOpaqueAttributes`.
  -> Constraint: a fix in `forwardOpaquePayload` alone misses route-server clients and every non-forward writer (origination, replay, plugin-injected bodies), so D1 belongs at the per-session final writer.
- [ ] `docs/architecture/wire/attributes.md` "Attribute Flags" and "32. LARGE_COMMUNITY".
  -> Decision: both sections gain the egress rule with a source anchor to the new function (Documentation row 7).
- [ ] `docs/guide/bgp-role.md` "On Send" table, rule 2 (the Gao-Rexford check).
  -> Constraint: Q-D4 (a) the row's role list gains `rs`; the page is edited in the same change.
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
  -> Constraint: (design review round 2, cross-spec) `writeUpdateBody` calls `clearTransmittedLabelIndex` on every outbound body with no family gate, which contradicts the owner's 2026-10-03 ruling that Label-Index is recognized only on labeled unicast. `spec-bgp-prefix-sid-rfc-defects` (closed 2026-10-20) AC-10 gates that clear to IPv4/IPv6 labeled unicast and names this spec as dependent. So the joined walk lands after that spec, or it carries the labeled-unicast gate on its Label-Index step. In the second case the fold changes behaviour (the clear stops running on other families) and is tested as that spec's AC-10 prescribes.
- [ ] `internal/component/bgp/reactor/rfc8092_large_community.go` (169L) - `removeRedundantLargeCommunities` (ingest), `largeCommunityRedundant` (pair scan up to 16 values, stack bitset above), `largeCommunityAttrsDeduped` (allocating copy)
  -> Decision: reuse `largeCommunityRedundant` for detection; add an in-place compaction for the egress side (no allocation; the private buffer is owned).
- [ ] `internal/component/bgp/plugins/filter_community/handler.go` - `genericCommunityHandler`: Keep for retained values, `p.Op(i)` for every Add with no membership check; header written Extended Length 0xC0
  -> Constraint: the transmit-side repeat is produced here today, and by `filter.go::ingressTagCommunities` (ingress tag, relation tag, blackhole tag); the egress fix covers both without editing either (Q-4 keeps COMMUNITIES and EXTENDED COMMUNITIES as they are).
  -> Decision: the egress walk in `writeUpdateBody` REPLACES the handler fix that `plan/journal/non-idempotent-append-duplicates-the-record.md` (row 2026-10-13) planned ("Skip a code-32 Add already kept or added, at the handler"); no handler skip is written, so the rule is declared once. Reason: RFC 8092 Section 3's sentence binds what is transmitted, whatever produced it, and a handler skip covers only that handler: it misses `ingressTagCommunities`, a plugin's raw body, and a policy Set that writes a repeat. The row's other half stands: no dedup for codes 8 and 16 (Q-4 (a), `TestTagExistingCommunityNoDeDup` pins code 8 and stays green). The row's Fix cell points at this spec; at landing it is closed with the commit.
- [ ] `internal/component/bgp/plugins/rib/ribout_entry.go` - `reconstructRoute` (stored bytes to `Route`, replayed as a text command), `packEventAttrs` (Adj-RIB-Out storage from a sent event)
  -> Constraint: neither writes a peer's wire directly; no change.
- [ ] `internal/component/bgp/message/rfc7606.go` - `validateAttributeFlags` judges Optional/Transitive only via `AttributeCode.FlagsConflict`; `validateLargeCommunityAttr` checks the length only
  -> Constraint: receive already ignores the low nibble for the three attributes the existing tagged test covers; AC-7 extends the proof to the forward rail.
- [ ] `internal/component/bgp/reactor/session_validation.go` - `applyRFC7606`: keep-first strip, tunnel-encap, then `applyTypedNLRIDiscard`; `publishBase` runs `removeRedundantLargeCommunities`
- [ ] `internal/component/bgp/reactor/session_validation_nlritype.go` - `applyTypedNLRIDiscard`, `typedNLRIEdit`: per-family `nlritype.Retain` drops unrecognized types (RFC 7606 Section 5.4) and malformed BGP-LS NLRI (RFC 9552 Section 8.2.2); rebuilds the body once; `typedNLRIEmptied` stops a body with no routes
  -> Decision: D3 adds a third per-NLRI outcome, malformed-withdraw, in the same pass and the same rewrite. A withdraw verdict whose key `nlrisplit.GetPrefixKey` cannot form becomes a counted discard (AC-10a), decided in this pass. The MUP recognizer answers withdraw for every unkeyable NLRI before its body checks, so no unkeyable MUP NLRI reaches the RIB or a writer, whatever Q-1 rules.
- [ ] `internal/component/bgp/reactor/session_read.go` - `processValidatedMessage`, its treat-as-withdraw arm: `SynthesizeWithdrawFamilies` bodies; extra bodies enter `onMessageReceived` with a `noPoolBufID` handle so the forward cache finds them
  -> Decision: the D3 withdrawal of malformed MUP NLRIs rides an extra body through this same mechanism when the UPDATE already carries an MP_UNREACH_NLRI (RFC 7606 Section 3(g) permits one); otherwise it is an MP_UNREACH_NLRI added to the rewritten body.
- [ ] `internal/core/bgp/nlri/nlritype/nlritype.go` - `Recognizer func(nlriBytes []byte, addPath bool) bool`, `Register`, `Get`, `Retain`
  -> Decision: (design review round 1, N-3) the `Recognizer`'s answer becomes a three-way verdict, keep / discard (unimplemented type, RFC 7606 Section 5.4) / withdraw (malformed, treat-as-withdraw), so each family keeps ONE registered function and nothing central names MUP. The EVPN and MVPN recognizers (`nlri/evpn/register.go`, `nlri/mvpn/register.go`, the only other registrants) answer keep or discard as today; the MUP recognizer runs the non-allocating body checks (R-8). The `Recognizer` and `Retain` also take the caller-owned key scratch (`nlrisplit.PrefixKeyScratchSize` octets, a `Session` field; AC-10a's Constraint), which the EVPN and MVPN recognizers ignore. `Retain` reports withdrawn NLRIs separately from discarded ones. The type comment's "It never validates the body" changes with it. Not taken: a second per-family function kind beside `Recognizer`, which is two registrations and two lookups per family for one per-NLRI question. BGP-LS's malformed discard stays on its own route (`message.RetainWellFormedNLRI`, `NLRISyntaxRuled`, a central enumeration); moving it is not this spec's scope.
- [ ] `internal/component/bgp/plugins/nlri/mup/rfc7606.go` - `Implemented`, `RecognizeNLRI`
- [ ] `internal/component/bgp/plugins/nlri/mup/types.go` - `ParseMUP`, `parseBodyT1ST` (tolerates an absent Source Address Length octet, the ExaBGP shape), `validateTLVs` (returns `ErrMUPTLV` for one leftover octet and for a Length overrun), `ErrMUPTruncated`, `ErrMUPTEIDZero`, `ErrMUPEndpointLength`, `ErrMUPSourceLength`
  -> Constraint: `ParseMUP` takes an AFI and a single NLRI without the path identifier; the recognizer strips the 4-octet RFC 7911 identifier first when ADD-PATH is negotiated.
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
| D3 | a T1ST whose mandatory fields overrun its Length, or whose TLV area is one octet, or whose TLV overruns, is installed and relayed | that NLRI is treated as withdrawn: removed from MP_REACH_NLRI, withdrawn from Adj-RIB-In and, when an earlier generation of its key was advertised, relayed as a withdrawal; an NLRI with no formable key is discarded and counted, never relayed (AC-10a); siblings kept; no session reset |
| D4 | RS-Client-learned unicast route sent to Provider, Peer, RS | Q-D4 (a): suppressed, and so withdrawn, to those three |
| D5 | received BLACKHOLE-tagged route re-advertised to any peer | Q-D5 (a1): withheld, and so withdrawn or replaced, towards a bilateral eBGP destination that did not agree to the community; iBGP and route-server-client destinations still receive it |

**Preserved:** ingest repeat removal (RFC8092-3-2); RFC 4271 Section 5 drop/Partial treatment; route-server opaque preservation of attribute VALUES; MUP unknown-route-type discard (3.1-1) and unknown-TLV propagation (3.1.3.1-9); OTC wire check and stamping; blackhole origination gate; every D6 withhold and withdrawal behaviour.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- a BGP UPDATE received from a peer

### Transformation Path
1. RFC 7606 validation (`applyRFC7606`): keep-first strip, tunnel-encap, typed NLRI pass (`applyTypedNLRIDiscard`). D3 lands here: malformed MUP NLRI leave MP_REACH and become a withdrawal.
2. `publishBase`: ingest repeat removal for LARGE_COMMUNITY (unchanged).
3. RIB insert and best path; forward rails (`forwardUpdateCore`, `reactorForwardRS`) run egress filters per destination. D4 (`OTCEgressFilter`) and D5 (filter_community egress) land here; a genuine egress reject already withdraws (D6 owner decision).
4. Policy edits (`genericCommunityHandler` and siblings) build the destination's attributes.
5. Final writer `Session.writeUpdateBody`: private-buffer copy, then one attribute walk that clears the low nibble (D1), removes LARGE_COMMUNITY repeats (D2) and clears the Label-Index fields (existing; on labeled unicast only once the Prefix-SID spec's AC-10 gate is in), then ownership filter, PATHS-LIMIT, TCP.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ filter plugins | egress filter calls with wire payload and `ModAccumulator` (role, filter_community) | Yes, `OTCEgressFilter` signature read 2026-10-13 |
| NLRI plugin → core registry | `nlritype.Register` from `nlri/mup` `init()`; core never imports the plugin | Yes, registry read 2026-10-13 |
| Receive goroutine → forward cache | extra withdraw-only bodies through `onMessageReceived` with `noPoolBufID` | Yes, `session_read.go` read 2026-10-13 |

### Integration Points
- `Session.writeUpdateBody` (one call site, beside `clearTransmittedLabelIndex`)
- `nlritype` registry (three-way `Recognizer` verdict) and `typedNLRIEdit`
- `OTCEgressFilter` source-role set
- filter_community egress filter, config load and reload (D5 under Q-D5 (a1): the `blackholecfg.Parse` map cached beside `peerConfigs`, rebuilt by `OnConfigApply`)
- `filterapi.PeerFilterInfo` gains `IsIBGP`, filled from `PeerSettings.IsIBGP` (D5)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE from peer A with an unknown transitive attribute flagged 0xCF and ORIGIN flagged 0x4F, relayed to peer B (normal client) and peer C (route-server client) | → | `writeUpdateBody` egress walk | `test/plugin/rfc4271-attr-flags-low-nibble-relay.ci` |
| UPDATE from peer A with LARGE_COMMUNITY X; export policy on peer B adds X and adds Y twice | → | `genericCommunityHandler` then `writeUpdateBody` egress walk | `test/plugin/rfc8092-large-community-no-repeat-egress.ci` |
| ipv4/mup UPDATE from peer A: well-formed T1ST K, then an UPDATE with malformed T1ST of key K plus well-formed T1ST L | → | `typedNLRIEdit` with the MUP recognizer's withdraw verdict, extra withdraw body, forward | `test/plugin/mup-t1st-malformed-treat-as-withdraw.ci` |
| unicast route from a session where Ze's local role is `rs`, destinations with remote roles provider, peer, rs, customer, rs-client | → | `OTCEgressFilter` | `test/plugin/rfc9234-rs-client-learned-not-upward.ci` (Q-D4 (a)) |
| BLACKHOLE-tagged route from an agreed peer A, destinations B (eBGP, agreed), C (eBGP, not agreed), D (iBGP, no `blackhole` container), E (route-server client, not agreed) | → | filter_community egress gate reading the cached per-peer rule | `test/plugin/rfc7999-blackhole-readvertise-agreement.ci` (under Q-D5 (a1)) |
| config reload that removes B's agreement while B holds a BLACKHOLE-tagged route | → | filter_community `OnConfigVerify` / `OnConfigApply` rebuilding the cache, then the egress gate | `test/reload/rfc7999-blackhole-agreement-removed-by-reload.ci` (under Q-D5 (a1)) |
| D6: Link-Local-only UPDATE from an on-link peer, relayed to a multihop peer | → | `egressNextHopWithheld`, `egressNextHopLinkLocalOnlyOffLink`, `buildWithdrawalPayload` (RS rail) | `test/plugin/linklocal-only-multihop-withdraw.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | unknown transitive attribute with flag bits 0x0F set | forwarded with those bits zero |
| AC-2 | LARGE_COMMUNITY with a repeated value | forwarded once per value |
| AC-3 | MUP ST1 route with truncated mandatory fields or a bad TLV | treat-as-withdraw |
| AC-4 | D4, D5 | as the owner rules |
| AC-5 | the `weak` verdicts of RFC4271-4.3-3, RFC4271-4.3-4, RFC8092-3-1, DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6, DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-7, DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-10, RFC9234-3.1-1, RFC7999-3.1-2 and DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1, -4-2, -4-4, -4-7, -4-9, after this spec's producer fix | each verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. D6 covers missing-global internal and external multihop cases, loss of usable next hops at a directly attached external peer, withdrawal of an already advertised generation, and usable-global controls. Moved here from the "Blocked by" table of the BGP child spec-rfc-verdict-fix-bgp (parent `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` P-3; original transfer 2026-09-28, missing-next-hop extension 2026-10-02). |

### Detailed acceptance criteria (design, 2026-10-13)

AC-1 to AC-5 above keep their IDs: the parent `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` and the BGP child spec-rfc-verdict-fix-bgp cite AC-5. The rows below make AC-1 to AC-4 testable and add the proof each owes.

| AC ID | Refines | Input / Condition | Expected Behavior | Proof |
|-------|---------|-------------------|-------------------|-------|
| AC-1a | AC-1 | peer A sends an UPDATE with ORIGIN flagged 0x4F and an unrecognized optional transitive attribute (code 241) flagged 0xCF; relayed to normal peer B and route-server client C | on B's wire ORIGIN is 0x40 and code 241 is 0xE0 (Partial stamped by the existing RFC 4271 Section 5 rule, nibble cleared); on C's wire code 241 is 0xE0 as well and its value bytes equal the received ones. The route-server rail stamps Partial too, because RFC 4271 Section 5 reads: "If a path with an unrecognized transitive optional attribute is accepted and passed to other BGP peers, then the unrecognized transitive optional attribute of that path MUST be passed, along with the path, to other BGP peers with the Partial bit in the Attribute Flags octet set to 1." (main-thread decision 2026-10-14; the design text said 0xC0) | unit over `writeUpdateBody` (both `raw` values, per Q-3) and the `.ci` in the Wiring table, hex assertions on both sessions |
| AC-6 | AC-1 | same relay, every flags octet received with 0x0F clear, Extended Length set on one attribute | bytes sent equal bytes the pre-fix code sends: Extended Length kept, no other bit touched | negative-polarity unit, byte-for-byte |
| AC-7 | AC-1 (RFC4271-4.3-4 receive) | ORIGIN, AS_PATH, NEXT_HOP and an unknown transitive attribute each received with 0x0F set | no RFC 7606 action, route installed and relayed (with the bits cleared) | unit over the receive-to-forward path, extending what `TestRFC4271AttrFlagsLowNibbleIgnoredOnReceive` proves at the validator |
| AC-2a | AC-2 | route with LARGE_COMMUNITY [X, Z]; export policy on B adds X, then Y, then Y | B receives [X, Z, Y], each once, first-occurrence order; attribute length 36; total path attribute and message lengths consistent | `.ci` with hex, plus unit over `writeUpdateBody` |
| AC-8 | AC-2 | a body reaching `writeUpdateBody` with LARGE_COMMUNITY [X, X, Y] from any producer (hand-built, as a plugin raw send would); and a body with 20 distinct values | first: sent as [X, Y]; second: sent unchanged (exercises the bitset path, above 16 values) | unit, both polarities; allocation count 0 on the no-repeat path AND on the repeat (in-place compaction) path (`testing.AllocsPerRun`), above and below 16 values |
| AC-3a | AC-3 (3.1.3.1-6) | ipv4/mup UPDATE, MP_REACH with T1ST L (well-formed) and T1ST K whose Length ends inside the Endpoint Address | K is withdrawn (Adj-RIB-In entry for K's key removed if present; where that earlier generation was advertised, a withdrawal of K relayed), L installed and relayed, session stays Established, no NOTIFICATION | unit over `applyRFC7606` and the `.ci` in the Wiring table |
| AC-3b | AC-3 (3.1.3.1-7) | as AC-3a, K's TLV area is exactly one octet | as AC-3a | unit |
| AC-3c | AC-3 (3.1.3.1-10) | as AC-3a, K carries a TLV whose Length overruns the NLRI | as AC-3a | unit |
| AC-9 | AC-3 polarity | T1ST with a well-formed unknown TLV 0xF0 beside a malformed sibling | the well-formed route is kept and relayed with its TLV octets unchanged | unit; the existing 3.1.3.1-9 tests stay green with their assertions unchanged (only their helper `retainMUP` changes) |
| AC-10 | AC-3 edges | (i) the UPDATE already carries an MP_UNREACH_NLRI of another family; (ii) K was the only route; (iii) ADD-PATH negotiated for ipv4/mup; (iv) K's RD/prefix overruns so no key forms | (i) the K withdrawal rides a separate withdraw-only UPDATE and both families' withdrawals survive; (ii) the body becomes withdraw-only, never a forged End-of-RIB; (iii) the path identifier is kept on the withdrawal; (iv) see AC-10a. A framing overrun of the MP attribute itself stays RFC 7606 Section 5.3 session reset (unchanged) | unit table, one case each for (i) to (iii) |
| AC-10a | AC-3 edge (iv) | any MUP NLRI K whose key cannot form, whatever the body verdict: (1) K's RD or prefix overruns its own Length; (2) an IPv6 T1ST with Prefix Length 129 and enough octets that nothing overruns; (3) an ISD with one octet past its prefix. In each, `keyMUP` returns `errPrefixKey`; well-formed sibling L in the same UPDATE; a second destination already holds L | K is discarded: removed from MP_REACH_NLRI, counted in the malformed-MUP counter and logged with the NLRI bytes, no withdrawal of K is relayed to any destination, no panic, no session reset; L is installed and relayed; the forward batch to every destination completes (no "forward batch write failed") | `TestMUPUnkeyableMalformedT1STDiscardedNotRelayed`, ingress plus forward rail, one subtest per shape (1) to (3) |
| AC-11 | AC-4 (D4, Q-D4 (a)) | unicast route learned on a session where Ze's local role is `rs`; destinations with remote roles provider, peer, rs, customer, rs-client; plus a locally originated route | not sent to provider, peer, rs; where an earlier generation was sent, it is replaced by an eligible survivor, else withdrawn (A-6); sent to customer and rs-client with OTC stamped per Section 5 egress rule 1; the local route still reaches the provider | flipped subtest of `TestOTCEgressFilter` plus the `.ci`, which holds two fixtures: one with no surviving candidate (withdrawal asserted) and one with an eligible survivor (the survivor asserted) |
| AC-12 | AC-4 (D5) | BLACKHOLE-tagged route received from A; eBGP B lists 65535:666 in its `blackhole` container, eBGP C does not, iBGP D has no `blackhole` container, route-server client E does not list it (Q-D5 (a1), owner 2026-10-14) | B, D and E receive it, C does not; an untagged control route reaches C; an RFC 7705 migration iBGP destination (`PeerSettings.IsIBGP` true through `MigrationAS`) with no `blackhole` container receives it. Where C held an earlier generation, it is replaced by an eligible survivor, else withdrawn (A-6) | `.ci` plus unit; the `.ci` holds one fixture with no surviving candidate (withdrawal asserted) and one with an eligible untagged survivor (the survivor asserted); a unit proves the rule is read from the cache (no `blackholecfg.Parse` per UPDATE), and a unit proves a peer whose only statement is a `blackhole` container is found agreed |
| AC-12a | AC-4 (D5, reload) | B agreed at startup holds the BLACKHOLE-tagged route; a reload removes B's agreement; a second reload adds an agreement for C | after the first reload, A's next UPDATE of the BLACKHOLE-tagged route is not re-advertised to B, so B's earlier generation is withdrawn or replaced by an eligible survivor as in AC-12; after the second, A's next UPDATE reaches C; a reload whose verify fails, or that rolls back, leaves the previous cache in force. Whether a reload alone, with no new UPDATE from A, re-judges what B already holds is not claimed (unverified; `plan/journal/policy-change-never-re-evaluates-what-it-admitted.md` records that class for redistribute) | `test/reload/rfc7999-blackhole-agreement-removed-by-reload.ci` plus a unit over the verify, apply and rollback handlers |
| AC-13 | goals | the scenarios in "Interop and goal validation" | each passes against the named daemon and was shown red with the fix reverted and the image rebuilt (`interop-and-goal-validation.md`) | recorded red and green logs |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | an operator relays routes from a peer that sets unused flag bits, to a normal peer and a route-server client | wire -> `applyRFC7606` -> RIB -> `forwardUpdateCore` / `reactorForwardRS` -> `writeUpdateBody` egress walk -> TCP | `test/plugin/rfc4271-attr-flags-low-nibble-relay.ci` |
| 2 | an operator's export policy tags a large community the route already carries | RIB -> egress policy (`genericCommunityHandler`) -> `writeUpdateBody` egress walk -> TCP | `test/plugin/rfc8092-large-community-no-repeat-egress.ci` |
| 3 | a peer sends a malformed T1ST replacing a route Ze already relayed | wire -> `applyRFC7606` (`typedNLRIEdit`, MUP recognizer withdraw verdict) -> withdraw body -> RIB removal -> forward withdrawal | `test/plugin/mup-t1st-malformed-treat-as-withdraw.ci` |
| 4 | an RS operator's RS-Client-learned route meets a provider session (Q-D4 (a)) | RIB -> `OTCEgressFilter` -> withheld and withdrawn | `test/plugin/rfc9234-rs-client-learned-not-upward.ci` |
| 5 | a customer's BLACKHOLE route meets an eBGP peer with no agreement (under Q-D5 (a1)) | RIB -> filter_community egress gate (cached `blackholecfg.Rule`) -> withheld and withdrawn | `test/plugin/rfc7999-blackhole-readvertise-agreement.ci` |

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
| `TestMUPMalformedT1STWithdrawEdges` (new, untagged table) | same | AC-10 (i) to (iii) | red at HEAD |
| `TestMUPUnkeyableMalformedT1STDiscardedNotRelayed` (new, untagged) | same | AC-10a: no withdrawal of K relayed, sibling L delivered, every destination's batch completes | red at HEAD (K is installed and relayed) |
| `TestMUPRecognizerVerdicts` (new) | `internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_ingress_test.go` (existing file) | the ipv4/mup and ipv6/mup recognizer answers keep for a well-formed T1ST (with and without an unknown TLV), discard for an unimplemented route type, withdraw for each malformed shape and for each unkeyable shape of AC-10a; it runs the same body checks `ParseMUP` runs; a well-formed T2ST with a 160-bit Endpoint Length (the 32-octet key) is keyed with a `nlrisplit.PrefixKeyScratchSize` scratch and never answered as unkeyable; `testing.AllocsPerRun` is 0 for a keep, a withdraw and a discard verdict (R-8, the scratch arrives from the caller) | red at HEAD |
| `TestRetainReportsWithdrawnSeparately` (new) | `internal/core/bgp/nlri/nlritype/nlritype_test.go` | registry: discarded and withdrawn NLRIs counted apart; the EVPN and MVPN recognizers never answer withdraw, so their families are unchanged | red at HEAD |
| `BenchmarkWriteUpdateBodyEgressWalk` (new) | `internal/component/bgp/reactor/rfc4271_attr_flags_egress_test.go` | R-1 budget: cases no LARGE_COMMUNITY, 4 distinct, 20 distinct, 20 with one repeat; 0 allocs/op in every case. It drives `Session.writeUpdateBody`, so the same file compiles and runs at HEAD for the baseline. Agents prepare it; the owner runs it on the Mac | new |
| `TestOTCEgressFilter` subtest `src_role_rs_to_provider_accept` renamed `src_role_rs_to_provider_suppress`, plus new subtests for rs to peer and rs to rs, and rs to customer and rs-client accept | `internal/component/bgp/plugins/role/rfc9234_otc_test.go` | AC-11 | red at HEAD; CHANGES A TAGGED UNIT (below) |
| D5 test per ruling | `internal/component/bgp/plugins/filter_community/rfc7999_blackhole_readvertise_test.go` (new, under Q-D5 (a1)) | AC-12: agreed, not agreed, iBGP, migration iBGP, route-server client, blackhole-only peer, cache read with no per-UPDATE parse; check order (an iBGP or RS-client destination and an untagged route never reach the lookup); `testing.AllocsPerRun` 0 for the egress gate on an agreed eBGP destination found by address, one found by name, a not-agreed miss, and an untagged route | red at HEAD under Q-D5 (a1) |
| D5 reload units | same file | AC-12a: verify stages, apply swaps, rollback restores, a failed verify leaves the cache | red at HEAD under Q-D5 (a1) |

### RFC-tagged tests this spec changes (each needs owner approval, Q-5)

| Tagged unit | Tag | Change | Condition |
|-------------|-----|--------|-----------|
| `internal/component/bgp/plugins/role/rfc9234_otc_test.go::TestOTCEgressFilter` (both recorded entries) | RFC9234-3.1-1 | the untagged subtest `src_role_rs_to_provider_accept` inside the tagged unit flips its assertion from accept to suppress; three subtests added | Q-D4 (a) |
| `reactor/rfc7606_session_validation_nlritype_test.go`: `TestRFC7606Section54DiscardsUnrecognizedEVPNType`, `TestRFC7606Section54PropagatesUnknownBGPLSType`, `TestRFC7606Section54DropsMPUnreachOnlyUpdateRatherThanForgeEOR`, `TestRFC7606Section54SessionResetsUnparseableTypedNLRI` | RFC7606-5.4-1 | the file's helper `registerEVPNRecognizer` registers a closure that returns the new verdict instead of a bool; assertions unchanged. The helper is its own top-level function, so these test bodies are not edited | D3, only if the commit gate names the unit |
| `reactor/rfc7606_session_validation_nlritype_typed_test.go`: `TestRFC7606Section54DiscardsUnrecognizedMVPNType`, `TestRFC7606Section54DiscardsUnrecognizedMUPType`, `TestRFC7606Section54FiltersTreatAsWithdrawSynthesis` | RFC7606-5.4-1 | the file's helpers `registerTypeLenRecognizer` and `registerMUPRecognizer` return the verdict; assertions unchanged. The helpers are top-level functions, so these test bodies are not edited | D3, only if the commit gate names the unit |
| `reactor/session_validation_nlritype_bypass_test.go`: `TestRFC7606Section54FiltersWhenTheAttributeWalkIsAbandoned`, `TestRFC7606Section54ReadsTypedNLRIUnderAddPath`; `reactor/rfc7432_session_validation_evpn_test.go`: `TestEVPNIngressPreservesPerESZeroLabel`; `reactor/rfc7606_session_validation_dupmp_test.go`: `TestRFC7606Section3gDuplicateMPBeatsAnAbandonedWalk`; `reactor/rfc7606_receive_boundary_test.go`: `TestRFC7606DiagnosticWireAndMPNLRI` | RFC7606-5.4-1, RFC7911-5-5, RFC7432-8.2.1-9, RFC7606-3.g-1, RFC7606-6-1 | call site unedited; their recognizer is a changed helper above, so their behaviour runs through the change (the Review Gate I7 precedent counted a helper change). Approved here so the commit is not refused if the gate names them | D3, only if the commit gate names the unit |
| `nlri/mvpn/rfc7606_test.go::TestRecognizeNLRIRejectsUnimplementedTypes`, `nlri/mup/rfc7606_test.go::TestRecognizeNLRIRejectsUnimplementedTypes` | RFC7606-5.4-1 | `RecognizeNLRI` is asserted as the discard verdict instead of `false`; same inputs. The EVPN twin carries no tag. These two test bodies change, so the gate names them | always (D3) |
| `nlri/mup/draft_ietf_bess_mup_safi_ingress_test.go`: `TestMUPOtherRouteTypesSilentlyIgnoredAtIngress`, `TestMUPDefinedRouteTypesSurviveIngress`, `TestMUPUnknownTLVPropagatedUnchangedAtIngress`, `TestMUPUnknownTLVNotStrippedAtIngress` | DRAFT-IETF-BESS-MUP-SAFI-3.1-1, DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-9 | the file's helper `retainMUP` reads `nlritype.Retain`'s new results (withdrawn reported apart from discarded). Since `32ff59085b` (spec-rfc-verdict-fix-bgp) `TestMUPOtherRouteTypesSilentlyIgnoredAtIngress` calls `retainMUPCounts` and asserts discarded 4 and withdrawn 0 apart, so its body and assertions are already changed; the other three keep their assertions. The same file's T2ST encoder units (3.1.4.1-3) do not call the helper and are not edited | D3, only if the commit gate names the unit |
| `reactor/rfc8669_label_index_transmit_test.go::TestRFC8669LabelIndexTransmissionClearsReservedAndFlags` | RFC8669-3.1-3, RFC8669-3.1-5 | its fixture (`rfc8669RelayBody`, `rfc8669Relay`) is an IPv4 unicast body, so once the joined walk carries the labeled-unicast gate the dirty case is no longer cleared and the unit goes red. It gets its own IPv4 labeled-unicast body and relay helper (MP_REACH AFI 1, SAFI 4, relayed on both rails) in the same file; assertions unchanged; the shared IPv4 unicast helpers in `rfc8669_nexthop_change_defect_test.go` are not edited. This is the Prefix-SID spec's Q5 row 9, and only the spec that lands the gate makes the edit | only on R-9's land-first branch (this spec lands before `spec-bgp-prefix-sid-rfc-defects` (closed 2026-10-20) and carries its labeled-unicast gate) |

#### Independent audit of these changes (2026-10-13)

Source: `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/rfc-tagged-test-compliance-review.md`, section 6.
Re-checked against the test bodies at HEAD, `rfc/full/rfc9234.txt` Section 3.1 and `rfc/full/rfc7999.txt` Section 3.2; agreed on both rows.

| Tagged unit | Audit verdict | What the change owes |
|-------------|---------------|----------------------|
| `TestOTCEgressFilter`, subtest `src_role_rs_to_provider_accept` flipped to `src_role_rs_to_provider_suppress` | COMPLIANT Q-D4 (a) | RFC 9234 Section 3.1: "Route Server Client (RS-Client):  MAY propagate any route learned from a Customer, or that is locally originated, to an RS.  All other routes MUST NOT be propagated." The Customer and Peer roles carry the same "All other routes MUST NOT be propagated" sentence, and a route learned from an RS-Client is not "learned from a Customer", so it MUST NOT go to a Provider, Peer or RS. Today the subtest has no tag line of its own (the unit's two tags sit on `TestOTCEgressFilter`'s other subtests, `src_role_customer_to_provider_suppress` and `src_role_provider_to_provider_accept`). The flipped subtest carries the D4 fix, so it gets its own `// RFC requirement: RFC9234-3.1-1 positive -- ...` line stating what its body checks (src-role `rs`, destination provider, suppressed), with its own `./le rfc discriminate-record` record. The three added subtests (rs to peer, rs to rs suppress; rs to customer and rs-client accept) each carry a tag line too |
| `TestRFC7999PropagationCommunityIsNeverForced` (RFC7999-3.2-2 negative) | COMPLIANT and unchanged under Q-D5 (a1) (owner, 2026-10-14) | RFC 7999 Section 3.2: "The community to prevent propagation SHOULD be chosen according to the operator's routing policy." Under (a1) the `blackhole-propagation` default stays `none`, so the claim "with the leaf set to none or left unset a BLACKHOLE route passes unchanged, so no fixed default is applied" and the body stay true; listed only so a reader sees it was judged |

The audit also confirms unchanged and still correct: the RFC4271-4.3-3/4 and RFC8092
units, and the MUP decode-path units (their claims are about `ParseMUP`).

The D3 rows above are mechanical: the `Recognizer` verdict type and `Retain`'s results change, and no claim, input or asserted outcome does. The set was derived by grep over every test file that calls `nlritype.Register`, `nlritype.Get`, `nlritype.Retain`, `RecognizeNLRI` or one of the four helpers, keeping the units that carry an `RFC requirement:` tag; `nlritype_snapshot_test.go` and `nlrisplit/typelen_test.go` carry none. At implementation, re-run that grep and add a row for any unit it finds that this table lacks. The decode-path units `TestMUPT1STMandatoryFieldsWithinLength` and `TestMUPT1STTLVFraming` (tagged 3.1.3.1-6, -7, -10) stay as they are: they prove `ParseMUP` refuses the shapes, which the MUP recognizer's withdraw verdict relies on, and the new ingress tests carry the treat-as-withdraw claim. `TestRFC4271AttributeFlagsLowNibbleZeroOnSend`, `TestRFC4271AttrFlagsLowNibbleIgnoredOnReceive`, `TestLargeCommunities` and `TestLargeCommunitiesWriteToNoDuplicates` are untouched. Every new tag gets a `./le rfc discriminate-record` record, and every changed verdict a `./le rfc audit-stamp ... mode rejudge` by an agent that did not write the test (AC-5).

#### Owner approval commands

Approved by the owner on 2026-10-13 (Q-5). The gate reads approvals from the committing session's own file (`tmp/commit-rfc-approved-<session>.md`), and a row is used up by the commit that carries its `RFC-approved:` trailer. So the owner runs these at the prompt of the session that implements and commits the edits, and runs a unit's command again if a later commit changes that unit a second time. A unit is `<package directory>.<top-level test function>`: the flipped and added subtests of `TestOTCEgressFilter` are named by that enclosing function. The first two names were checked to exist at HEAD `051876c3f4`, the D3 names at HEAD `68826acc76` (2026-10-14). The first applies Q-D4 (a). The Q-D5 (b) row for `TestRFC7999PropagationCommunityIsNeverForced` was removed on 2026-10-14: the owner chose (a1), under which that unit is unchanged. The D3 rows apply, one command per unit; the package directory disambiguates the two `TestRecognizeNLRIRejectsUnimplementedTypes`. A row a commit does not use stays in the approval file and would approve a later change to that unit, so a conditional command is run only for a unit the commit gate names when it refuses the commit. The gate (`internal/le/commit/rfcchange.go::changedRFCUnits`) compares, file by file, the text of each tagged top-level function (`rfc.FunctionUnits`) before and after the commit, so a unit whose own body is unedited is never named, whatever a helper it calls does. Every tag in the D3 files sits inside a function, and `registerEVPNRecognizer`, `registerTypeLenRecognizer`, `registerMUPRecognizer` and `retainMUP` are top-level functions of their own, so 16 of the 18 D3 units keep their bodies and are conditional; only the two `TestRecognizeNLRIRejectsUnimplementedTypes` units, whose bodies change, run unconditionally, plus any unit whose body the implementer does edit. The `TestRFC8669LabelIndexTransmissionClearsReservedAndFlags` row runs only on R-9's land-first branch.

| Applies under | Command |
|---------------|---------|
| Q-D4 (a) | `./le rfc approve unit role.TestOTCEgressFilter reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects, Q-D4 (a): src_role_rs_to_provider flips to suppress, three tagged subtests added)'` || D3, only if the gate names it | `./le rfc approve unit reactor.TestRFC7606Section54DiscardsUnrecognizedEVPNType reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects D3: Recognizer verdict type, assertions unchanged)'` |
| D3, only if the gate names it | `./le rfc approve unit reactor.TestRFC7606Section54PropagatesUnknownBGPLSType reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects D3: Recognizer verdict type, assertions unchanged)'` |
| D3, only if the gate names it | `./le rfc approve unit reactor.TestRFC7606Section54DropsMPUnreachOnlyUpdateRatherThanForgeEOR reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects D3: Recognizer verdict type, assertions unchanged)'` |
| D3, only if the gate names it | `./le rfc approve unit reactor.TestRFC7606Section54SessionResetsUnparseableTypedNLRI reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects D3: Recognizer verdict type, assertions unchanged)'` |
| D3, only if the gate names it | `./le rfc approve unit reactor.TestRFC7606Section54DiscardsUnrecognizedMVPNType reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects D3: Recognizer verdict type, assertions unchanged)'` |
| D3, only if the gate names it | `./le rfc approve unit reactor.TestRFC7606Section54DiscardsUnrecognizedMUPType reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects D3: Recognizer verdict type, assertions unchanged)'` |
| D3, only if the gate names it | `./le rfc approve unit reactor.TestRFC7606Section54FiltersTreatAsWithdrawSynthesis reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects D3: Recognizer verdict type, assertions unchanged)'` |
| D3, only if the gate names it | `./le rfc approve unit reactor.TestRFC7606Section54FiltersWhenTheAttributeWalkIsAbandoned reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects D3: Recognizer verdict type through a changed helper, assertions unchanged)'` |
| D3, only if the gate names it | `./le rfc approve unit reactor.TestRFC7606Section54ReadsTypedNLRIUnderAddPath reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects D3: Recognizer verdict type through a changed helper, assertions unchanged)'` |
| D3, only if the gate names it | `./le rfc approve unit reactor.TestEVPNIngressPreservesPerESZeroLabel reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects D3: Recognizer verdict type through a changed helper, assertions unchanged)'` |
| D3, only if the gate names it | `./le rfc approve unit reactor.TestRFC7606Section3gDuplicateMPBeatsAnAbandonedWalk reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects D3: Recognizer verdict type through a changed helper, assertions unchanged)'` |
| D3, only if the gate names it | `./le rfc approve unit reactor.TestRFC7606DiagnosticWireAndMPNLRI reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects D3: Recognizer verdict type through a changed helper, assertions unchanged)'` |
| D3, always | `./le rfc approve unit mvpn.TestRecognizeNLRIRejectsUnimplementedTypes reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects D3: discard verdict asserted instead of false, same inputs)'` |
| D3, always | `./le rfc approve unit mup.TestRecognizeNLRIRejectsUnimplementedTypes reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects D3: discard verdict asserted instead of false, same inputs)'` |
| D3, only if the gate names it | `./le rfc approve unit mup.TestMUPOtherRouteTypesSilentlyIgnoredAtIngress reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects D3: retainMUP reads the new Retain results, assertions unchanged)'` |
| D3, only if the gate names it | `./le rfc approve unit mup.TestMUPDefinedRouteTypesSurviveIngress reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects D3: retainMUP reads the new Retain results, assertions unchanged)'` |
| D3, only if the gate names it | `./le rfc approve unit mup.TestMUPUnknownTLVPropagatedUnchangedAtIngress reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects D3: retainMUP reads the new Retain results, assertions unchanged)'` |
| D3, only if the gate names it | `./le rfc approve unit mup.TestMUPUnknownTLVNotStrippedAtIngress reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects D3: retainMUP reads the new Retain results, assertions unchanged)'` |
| R-9 land-first branch only | `./le rfc approve unit reactor.TestRFC8669LabelIndexTransmissionClearsReservedAndFlags reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-update-propagation-rfc-defects R-9 land-first: fixture moves to its own IPv4 labeled-unicast helper, assertions unchanged; Prefix-SID spec Q5 row 9)'` |

### Functional Tests
| Test | Proves |
|------|--------|
| `test/plugin/rfc4271-attr-flags-low-nibble-relay.ci` | AC-1a end to end on a normal and a route-server destination |
| `test/plugin/rfc8092-large-community-no-repeat-egress.ci` | AC-2a end to end |
| `test/plugin/mup-t1st-malformed-treat-as-withdraw.ci` | AC-3a end to end: earlier generation of K withdrawn at the receiving peer, L delivered |
| `test/plugin/rfc9234-rs-client-learned-not-upward.ci` | AC-11 (Q-D4 (a)) |
| `test/plugin/rfc7999-blackhole-readvertise-agreement.ci` | AC-12 (Q-D5 (a1)) |
| `test/reload/rfc7999-blackhole-agreement-removed-by-reload.ci` | AC-12a (Q-D5 (a1)): a SIGHUP reload removes B's agreement, and A's next BLACKHOLE-tagged UPDATE is not re-advertised to B |

Each `.ci` is shown red against HEAD before the fix.

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Attribute Flags low nibble on send (D1) | 0x0-0xF received | 0x0 sent for every received value; 0x1 and 0xF both cleared, 0x10 (Extended Length) kept | N/A | N/A (the high nibble is never touched) |
| LARGE_COMMUNITY value count (D2) | 1 to the attribute's 255-or-extended capacity | 16 (pair scan) and 17 (stack bitset) both dedup a repeat and keep 16/17 distinct values unchanged | N/A | N/A |
| T1ST remaining octets after the Source Address (D3, draft "< 0", "1", ">= 2") | -1, 0, 1, 2 | 0 (no TLV) and 2 (one empty TLV) kept | -1: withdrawn (3.1.3.1-6) | 1: withdrawn (3.1.3.1-7) |
| T1ST TLV Length (D3) | 0 to the octets left | a TLV ending exactly at the NLRI end kept | N/A | one octet past the NLRI end: withdrawn (3.1.3.1-10) |
| IPv6 T1ST Prefix Length (D3, AC-10a) | 0 to 128 | 128 keyed and judged by the body checks | N/A | 129, with enough octets that nothing overruns: discarded, not relayed |
| ISD body length past the prefix (D3, AC-10a) | exactly RD, Prefix Length and prefix | no trailing octet: keyed and kept | N/A | one trailing octet: discarded, not relayed |

### Interop and goal validation

Per `docs/architecture/testing/interop.md`: scenario directories under `test/interop/scenarios/`, named, no numeric prefix, each with a typed checker in `internal/le/interoplab/bgp`. Malformed or unusual input is sent by the raw injector sidecar (172.30.0.9, `inject.msg`).

| Scenario (new) | Defect | Peer daemon | Assertion | Discrimination |
|----------------|--------|-------------|-----------|----------------|
| `bgp-attr-flags-low-nibble-gobgp` | D1 | GoBGP 3.31.0 (receiver) | the injector sends ORIGIN 0x4F and attribute 241 flagged 0xCF to Ze; GoBGP's JSON for the relayed path shows attribute 241 with flags 0xE0 | red with the egress clear reverted; A-2 |
| `bgp-large-community-no-repeat-gobgp` | D2 | GoBGP 3.31.0 (receiver) | Ze's export policy adds a value the route already carries; GoBGP's JSON lists each large community once | red with the egress removal reverted; A-3 |
| `bgp-mup-malformed-t1st-withdraw-gobgp` | D3 | GoBGP 3.31.0 (receiver, ipv4-mup) | GoBGP holds K after the well-formed generation, then loses K and holds L after the malformed UPDATE; the session stays up | red with the MUP recognizer's withdraw verdict turned back to keep; A-4 |
| `bgp-role-rs-client-learned-not-upward-frr` | D4 | FRR 10.4.1 (Ze's provider, `local-role provider`) | a route the injector sends on a session where Ze's role is `rs` never appears in FRR; a Ze-originated control route does | red with `rs` removed from the suppressed set; Q-D4 (a) |
| `bgp-rfc7999-blackhole-readvertise-frr` | D5 | FRR 10.4.1 (eBGP, no agreement) and BIRD (sender, agreed) | FRR never holds the BLACKHOLE-tagged prefix; it holds the untagged control prefix | red with the gate reverted; Q-D5 (a1) |

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
| A-1 | every UPDATE Ze writes to a BGP peer passes `Session.writeUpdateBody` | callers read 2026-10-13 and 2026-10-14: `writeUpdateGated`, `writeOrdinaryUpdateBody` chunks, `writeRawUpdateBody`, `writeOriginatedRawUpdate`, `writeUpdateWithoutAIGP` (`session_aigp.go`), and the message path of the generic writer. Every `bufWriter.Write` in `session_write.go` outside `writeUpdateBody` is reached only for a non-UPDATE type: `writeMessageWithin` (an UPDATE diverts to `writeOriginatedRawUpdate` first), and `SendRawMessage` in both modes (full-packet mode tests the type octet of the header, body mode the `msgType` argument, and each diverts an UPDATE to `writeOriginatedRawUpdate` before its `bufWriter.Write`) | a writer bypasses D1 and D2 | re-run the `bufWriter.Write` grep at implementation; a unit that fails if any UPDATE path skips the walk |
| A-2 | GoBGP's JSON shows an unknown attribute's flags octet | GoBGP models unknown attributes with their flags; not checked on 3.31.0 | the D1 scenario cannot discriminate | run the scenario with the fix reverted first; if GoBGP hides flags, use BIRD's `show route all` attribute dump or the compiled strict speaker's raw capture, and record the substitution |
| A-3 | GoBGP keeps repeated large communities on receipt | not checked; FRR removes them on receipt (`lcommunity_uniq_sort`, unverified) and so cannot discriminate | the D2 scenario cannot discriminate | same as A-2: red run first |
| A-4 | GoBGP 3.31.0 negotiates and shows ipv4-mup | GoBGP 3.x lists MUP families; not checked in Ze's image | no MUP interop peer among FRR, BIRD, GoBGP | check `gobgp global rib -a ipv4-mup` in the lab image first; if absent, record the gap and ask the owner which peer stands in |
| A-5 | an absent Source Address Length octet means "no source address" | `parseBodyT1ST` reads it so, matching ExaBGP; draft: "Source Address Length is 0 bytes then the Source address is not carried within the NLRI" | Ze accepts a shape the draft calls malformed | unchanged by this spec; recorded here so the recognizer does not silently tighten it |
| A-6 | a genuine egress filter reject withdraws the destination's earlier generation, or replaces it with an eligible survivor | D6 owner decision (2026-10-03) and its landing: HEAD writes that withdrawal unconditionally on both rails. `plan/immediate/spec-bgp-rs-replacement-on-withdrawal.md` has decided the policy-reject case (Behavior: "A withdrawal from the destination's current owner, sent to a non-ADD-PATH destination, becomes the RIB-elected surviving route (an UPDATE carrying it) when one exists, and stays a withdrawal only when none exists. This covers received withdrawals and the owner's synthesized withdrawals (policy-denied re-announcement, treat-as-withdraw), on both rails, for bgp-rs and bgp-rr."; its AC-14). So once it lands, a D4 or D5 suppression at a non-ADD-PATH destination sends the elected survivor when one exists and a withdrawal only when none does, on both rails. AC-11 and AC-12 therefore assert "replaced by an eligible survivor, else withdrawn", with one fixture each way; before that spec lands, the survivor fixture shows a withdrawal, and the fixture is written against whichever spec lands second. That election's egress check MUST step past a survivor that the D4 source-role gate or the D5 agreement gate refuses for the destination, or the survivor re-leaks what the gate withheld (recorded under Blast Radius; the RS spec is not edited here) | D4 and D5 suppression leaves earlier generations at the destination, or the survivor bypasses the gate | AC-11 and AC-12, both fixtures |
| A-7 | route-server transparency (RFC 7947) does not require the low nibble to be kept | RFC 4271 Section 4.3 binds every speaker on send; the nibble is unused and carries no route information | none known | the owner chose Q-3 (a) on 2026-10-14 |

### Risks
| ID | Failure mode | Early signal | Mitigation |
|----|--------------|--------------|------------|
| R-1 | a per-write attribute walk costs throughput on the hot path | `BenchmarkFanout*` and `./le perf` regress | one walk shared with the existing Label-Index walk, on the private copy `writeUpdateBody` already makes, so no new copy. The walk replaces `clearTransmittedLabelIndex`'s `AttrFind`, which already walks every attribute header when Prefix-SID is absent. Cost that remains: a flags mask per attribute, and, only when a LARGE_COMMUNITY holds more than 16 values, `largeCommunityRedundant`'s 8 KiB stack bitset clear per write, now paid per destination. Budget, measured before and after on the owner's Mac (agents prepare the benchmark and the commands; no perf run happens on this machine) with `-count 6` and benchstat: (1) `BenchmarkWriteUpdateBodyEgressWalk` 0 allocs/op in every case, the repeat case included; (2) its no-LARGE_COMMUNITY and 4-value cases within 5% ns/op of the same benchmark run at HEAD, where `writeUpdateBody` holds only `clearTransmittedLabelIndex` (the benchmark drives `writeUpdateBody`, so it compiles at HEAD); (3) `BenchmarkFanoutFloor` and `BenchmarkFanoutDedup` at n=100 within 3% ns/op and unchanged allocs/op; (4) the 20-value case reported with its number, no budget. A miss on (1) to (3) is a defect of the walk, fixed before landing. Numbers go in the closing commit |
| R-2 | shrinking the body after the ownership filter would make duplicate-advertisement comparison see stale bytes | an ownership test sees a mismatch | run the walk at the existing `clearTransmittedLabelIndex` site, before the filter |
| R-3 | in-place compaction miscomputes a length (attribute, total path attribute, message) | peer NOTIFICATION, UPDATE Message Error | unit asserts all three lengths; the extended-length header width is kept |
| R-4 | the malformed-MUP withdrawal adds a second MP_UNREACH_NLRI | RFC 7606 Section 3(g) refusal at the peer | AC-10 (i): a separate withdraw-only body through the existing extra-body dispatch |
| R-5 | Ze-originated T1ST without TEID is withdrawn by every Ze receiver once D3 lands | MUP `.ci` tests that originate T1ST go red | Q-2 (a), owner 2026-10-14: the entry point refuses such a route, so the tests that originate one are rewritten with a TEID and endpoint; run `test/plugin/mup-*` and `test/encode/srv6-mup*.ci` early |
| R-6 | operators relying on RS-Client routes reaching providers lose them | operator report | Q-D4 (a), owner 2026-10-14; `docs/guide/bgp-role.md` row edited in the same change |
| R-7 | a test pinning byte-identical relay of a low-nibble attribute goes red | red unit in reactor or rs | read it: a test asserting the bits survive asserts the defect and is corrected under `rule-precedence.md` (wrong test), named in the commit |
| R-8 | the D3 recognizer runs `ParseMUP` per received MUP NLRI and allocates (`*MUP` per call) | allocation profile of MUP ingress | a non-allocating validate entry in `nlri/mup` that runs the same body checks without building `*MUP`; `ParseMUP` calls it, so one implementation holds the rules. The key check before it uses the caller-owned scratch (AC-10a's Constraint), so the whole verdict allocates nothing; `TestMUPRecognizerVerdicts` asserts 0 allocations |
| R-9 | the joined walk folds `clearTransmittedLabelIndex` while `spec-bgp-prefix-sid-rfc-defects` (closed 2026-10-20) gates that clear to labeled unicast, and one change undoes the other | a Prefix-SID AC-10 test, or a D1/D2 writer unit, goes red after the second spec rebases | land after the Prefix-SID spec, or carry its labeled-unicast gate in the joined walk (Current Behavior, `session_prefix_sid.go` constraint). On the land-first branch this spec also owns that spec's Q5 row 9: tagged `TestRFC8669LabelIndexTransmissionClearsReservedAndFlags` goes red on its IPv4 unicast fixture, moves to its own labeled-unicast helper, and is listed in the tagged-change table with its approval command; and `docs/features/srv6.md` is narrowed (Documentation row 16) |
| R-10 | the RS-replacement election sends a survivor that the D4 or D5 gate refuses for the destination | AC-11 or AC-12 survivor fixture receives a refused route | the election's egress check runs the same egress filters, D4 and D5 included (A-6); the survivor fixtures prove it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | D1/D2: every UPDATE Ze writes passes the new walk, so a length miscomputed by the in-place compaction (R-3) makes every peer that receives such an UPDATE send an UPDATE Message Error NOTIFICATION and drop the session; a wrong flags mask mis-encodes every attribute on every session. D3: a recognizer verdict wrong in the keep-to-withdraw direction withdraws good MUP routes at every Ze receiver. D4/D5: routes withheld from, or leaked to, upward or un-agreed peers |
| How is it reverted? | each defect lands as its own commit (D1 and D2 share one walk and so one commit), and each is a single-commit revert with no config migration; no YANG default changes (Q-D5 (a1)) |
| Who else touches this path? | `Session.writeUpdateBody` (`session_write.go`) is edited by two other open specs. `plan/immediate/spec-bgp-rs-replacement-on-withdrawal.md` stages owed-set effects and resets them between the two `ownership.filter` passes; `plan/immediate/spec-daemon-drain-before-stop.md` adds the `drain-unjudged` meta merge at the sent-event call. This spec inserts the D1/D2 walk at the `clearTransmittedLabelIndex` site, before the first `ownership.filter` pass, so it changes neither; whichever lands second rebases onto the other's edit of the same function. The RS spec also turns a policy-reject withdrawal at a non-ADD-PATH destination into the elected survivor on both rails, for bgp-rs and bgp-rr (A-6), which AC-11 and AC-12 depend on; its election's egress check MUST include the D4 role gate and the D5 `filter_community` gate (R-10). `spec-bgp-prefix-sid-rfc-defects` (closed 2026-10-20) gates `clearTransmittedLabelIndex` to labeled unicast (its AC-10), so the joined walk lands after it or carries that gate (R-9). The drain step and the D5 gate both add per-destination egress work on the same rails; R-1's fanout budget is measured with both in the tree when both have landed. `filter_community` (`handler.go`, `filter.go`) is the D2 producer and, under Q-D5 (a1), gains the D5 gate and its first reload handlers. `filterapi.PeerFilterInfo` gains `IsIBGP`, which every egress filter receives |

## Files to Modify

- `internal/component/bgp/reactor/session_write.go` - call the joined egress walk at the existing Label-Index site
- `internal/component/bgp/reactor/session_prefix_sid.go` - Label-Index clear becomes one step of the joined walk. Its outcome depends on the R-9 branch. Landing after `spec-bgp-prefix-sid-rfc-defects` (closed 2026-10-20), the fold changes no behaviour: the gate is already in, and the step keeps it. Landing first, the step carries that spec's labeled-unicast gate (its AC-10), a behaviour change on every other family: the clear stops running there, tagged `TestRFC8669LabelIndexTransmissionClearsReservedAndFlags` moves to a labeled-unicast fixture (tagged-change table), and `docs/features/srv6.md` is edited (Documentation row 16)
- `internal/component/bgp/reactor/rfc8092_large_community.go` - in-place compaction for the egress side; reuse `largeCommunityRedundant`
- `internal/component/bgp/reactor/session_validation_nlritype.go` - malformed-withdraw outcome, withdraw body; passes the session's key scratch to `nlritype.Retain`
- `internal/component/bgp/reactor/session.go` (`type Session struct`) - a `[nlrisplit.PrefixKeyScratchSize]byte` field for the typed NLRI pass's key scratch (AC-10a)
- `internal/component/bgp/reactor/session_read.go` - dispatch the extra withdraw body when needed (reuse the treat-as-withdraw extra-body loop)
- `internal/core/bgp/nlri/nlritype/nlritype.go` - three-way `Recognizer` verdict (type comment rewritten), the caller-owned key scratch argument on `Recognizer` and `Retain`, and the withdrawn count from `Retain`
- `internal/component/bgp/plugins/nlri/evpn/register.go` and the EVPN `RecognizeNLRI`, `internal/component/bgp/plugins/nlri/mvpn/register.go` and the MVPN `RecognizeNLRI` - return keep or discard in the new verdict type (no behaviour change)
- `internal/component/bgp/plugins/nlri/mup/rfc7606.go`, `register.go`, `types.go` - `RecognizeNLRI` returns withdraw for a malformed NLRI; non-allocating body check shared with `ParseMUP`
- `internal/component/bgp/plugins/nlri/mup/encode.go` or the MUP command parser - Q-2 (a): refuse a T1ST with no TEID or no endpoint, naming the missing field
- `internal/component/bgp/plugins/role/otc.go` - suppressed source roles (Q-D4 (a))
- `internal/component/bgp/plugins/filter_community/filter_community.go` - under Q-D5 (a1): cache the rules `blackholecfg.Parse` returns beside `peerConfigs`, re-keyed at config time into a `netip.Addr`-keyed map for non-template address entries and a `PeerConfigKey`-keyed map for name and group entries (split on `PeerConfigKey.Template`, with a unit case for a group named like an address), swapped under the package `mu` that already guards `peerConfigs`, looked up in `LookupPeerConfig`'s order (address, name, group) with no `.String()` per route (Q-D5 row, I-3), outside `configureCommunityFilter`'s `hasAnyRule` drop; register `OnConfigVerify` (parse and stage), `OnConfigApply` (swap) and `OnConfigRollback` (restore) for that cache only; in `egressFilter`, run the gate before the `!hasCfg` early return, and update its "Community filter never suppresses routes" comment. A new consumer of `blackholecfg`, which today only rib, rpki and cmd/announce import
- `internal/component/bgp/plugins/filter_community/blackhole.go` - agreement gate reading the cache and `PeerFilterInfo.IsIBGP`, `RSClient` (Q-D5 (a1)), in the order the Q-D5 row fixes: `IsIBGP`/`RSClient`, then `blackholecfg.Carries`, then the allocation-free lookup
- `internal/component/bgp/filterapi/filterapi.go` - `PeerFilterInfo` gains `IsIBGP` (Q-D5 (a1)); the `LocalAS` comment "for iBGP detection" is corrected. The `EgressFilterFunc` doc comment is edited too (design review round 3, I-4; `stale-comments.md`): its sentence "filter_community's egress filter never suppresses at all" becomes false and is replaced by one saying it suppresses a BLACKHOLE-tagged route towards a bilateral destination that did not agree to it, and that this suppression is a decision. The paragraph "NO REGISTERED FILTER IS IN THAT STATE TODAY, re-derived from all three" is re-derived with its date: it stays true, because the gate has no state in which it cannot decide. A cache miss, including the empty cache before the first config apply, means "not agreed", the same answer `agreedSelector` gives, so a refusal is the decision written for a peer known not to have agreed, the shape the paragraph already accepts for `LLGREgressFilter`
- `internal/component/bgp/reactor/peer_forward_facts.go` (`Peer.buildForwardFacts`) and `reactor_api_batch.go` (`reactorAPIAdapter.sendStaleReadvertiseUnit`) - the two destination fills set `IsIBGP` from `PeerSettings.IsIBGP`; every other destination fill does too, so no destination fill leaves a silent `false`. (Design review round 3, N-1.) `sendStaleReadvertiseUnit`'s fill also omits `RSClient` today. That is harmless at HEAD, because only `gr` registers with `Readvertise` true (`plugins/gr/register.go`, read through `filterapi.ReadvertiseEgressFuncs`) and the D5 filter does not opt in, so the D5 gate never runs on that rail; it is filled anyway, from `peer.Settings().RSClient` as `buildForwardFacts` fills it from `PeerSettings.RSClient`, so no destination fill reads a silent `false` the day a filter that reads `RSClient` opts into readvertise. This provides the field the 2026-10-13 row of `plan/journal/helper-bypassed-by-an-open-coded-copy.md` plans; switching the relation-tag step to it, and the `forward_rs.go` `srcFilter` that carries only `Address`, stay with that row
- `internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_ingress_test.go` - add `TestMUPRecognizerVerdicts`; `retainMUP` reads the new `Retain` results
- the RFC-tagged test files of D3 listed under "RFC-tagged tests this spec changes", and `nlri/evpn/rfc7606_test.go` (untagged) - mechanical verdict-type edits
- `plan/journal/non-idempotent-append-duplicates-the-record.md` - row 2026-10-13 closed at landing with the D2 commit
- `rfc/short/rfc4271.md`, `rfc8092.md`, `draft-ietf-bess-mup-safi.md`, `rfc9234.md`, `rfc7999.md` - requirement-row annotations for the new proof; no new enrolment (RFC scope frozen 2026-10-08)
- docs per the Documentation checklist

## Files to Create

| File | Purpose |
|------|---------|
| `internal/component/bgp/reactor/rfc4271_attr_flags_egress_test.go` | D1 writer units and `BenchmarkWriteUpdateBodyEgressWalk` |
| `internal/component/bgp/reactor/rfc4271_attr_flags_relay_test.go` | D1 through both forward rails |
| `internal/component/bgp/reactor/rfc8092_large_community_egress_test.go` | D2 writer units |
| `internal/component/bgp/reactor/rfc8092_large_community_policy_test.go` | D2 through a policy add |
| `internal/component/bgp/reactor/draft_mup_t1st_withdraw_test.go` | D3 ingress, edges and AC-10a |
| `internal/component/bgp/plugins/filter_community/rfc7999_blackhole_readvertise_test.go` | D5, under Q-D5 (a1) |
| `test/plugin/rfc4271-attr-flags-low-nibble-relay.ci`, `rfc8092-large-community-no-repeat-egress.ci`, `mup-t1st-malformed-treat-as-withdraw.ci`, `rfc9234-rs-client-learned-not-upward.ci`, `rfc7999-blackhole-readvertise-agreement.ci`; `test/reload/rfc7999-blackhole-agreement-removed-by-reload.ci` | functional tests (the last three per ruling) |
| `test/interop/scenarios/bgp-attr-flags-low-nibble-gobgp/`, `bgp-large-community-no-repeat-gobgp/`, `bgp-mup-malformed-t1st-withdraw-gobgp/`, `bgp-role-rs-client-learned-not-upward-frr/`, `bgp-rfc7999-blackhole-readvertise-frr/` | interop scenarios (the last two per ruling) |
| a typed checker per scenario in `internal/le/interoplab/bgp` | scenario assertions, both polarities in `TestBespokeCheckerBranches` when bespoke |

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new leaf and no default change (Q-D5 (a1)) |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | No | none; under Q-2 (a) the MUP `update` parser gains a refusal, not a flag |
| CLI grammar (keyword before value) | N-A | no grammar change |
| Editor autocomplete | N-A | no new value |
| Functional test for new RPC/API | Yes | the six `.ci` files in the TDD plan |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no file, socket, port, module or binary |
| Prometheus counters/metrics | Yes | a counter of malformed MUP NLRI treated as withdrawn per peer, beside the RFC 7606 counters; D4 reuses `recordDrop(dropSourceRole, ...)`; names fixed at implementation and listed in Documentation row 14 |
| BGP family surface (new SAFI / capability / attribute) | N-A | no new family, capability or attribute; MUP is existing |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the BGP role and MUP feature records under `features/` (evidence level changes); found with `./le feature report` |
| 2 | Config syntax changed? | No | no default changes (Q-D5 (a1)) |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | Yes | `docs/architecture/meta/role.md` (D4), `docs/architecture/meta/filter-community.md` (D5) |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-role.md` rule 2 row (D4); `docs/guide/bgp-policy.md` (D5) |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/attributes.md` "Attribute Flags" and "32. LARGE_COMMUNITY" (egress rule, anchors to the new walk) |
| 8 | Plugin SDK/protocol changed? | No | the `nlritype` registry is internal, not SDK |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | the five `rfc/short/` files; `docs/features/rfc-status.md` regenerates from them |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` Scenario Inventory gains the new scenarios |
| 11 | Affects daemon comparison? | No | no comparison row covers these behaviours |
| 12 | Internal architecture changed? | Yes | `docs/architecture/bgp/structural-forwarding.md` (final-writer normalization), `docs/features/bgp-protocol.md` (nlritype three-way verdict) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derive with `./le spec citation anchors spec plan/immediate/spec-bgp-update-propagation-rfc-defects.md` before the first code edit; `session_write.go` alone is anchored by 16 pages (`ai/CODE-TO-DOCS.md`). Pages DECLARED by the files this spec changes: `docs/architecture/core-design.md` (RFC 7606 validation section gains the per-NLRI malformed-withdraw outcome; its community-handler text is unaffected because the handler is not edited), `docs/architecture/wire/nlri.md` (MUP section: ingress validation now withdraws a malformed T1ST), `docs/features/srv6.md`, by R-9 branch: landing after `spec-bgp-prefix-sid-rfc-defects` (closed 2026-10-20), the content is unaffected, because that spec has already narrowed the page to labeled unicast and the clear moves into the joined walk with that behaviour, so only the source anchor on `clearTransmittedLabelIndex` moves to the joined function; landing first, this spec carries the labeled-unicast gate and so edits the content too: the Propagation row's "cleared on every transmission, including relay" and the "The final session writers clear the Label-Index Reserved octet" paragraph are narrowed to IPv4 and IPv6 labeled unicast, in the same change as the gate, and the anchor moves, `rfc/short/rfc7606.md` (Section 5.4 and treat-as-withdraw notes gain the MUP per-NLRI withdraw beside the BGP-LS discard), `docs/architecture/api/architecture.md` (declared by `filterapi.go`; unaffected in content, since it does not list `PeerFilterInfo`'s fields; re-read when `IsIBGP` is added) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/bgp-role.md` examples; verify against the role YANG after D4 |

### Discovery (`ai/rules/repo-maintenance.md`)

| Question | Answer |
|----------|--------|
| Where an agent looks first | `ai/INDEX.md` rows for BGP role, RFC 7606, wire attributes; `ai/CODE-TO-DOCS.md` for `session_write.go` |
| Rule that prevents regression | the new tagged tests under `./le rfc check` discrimination |
| Registry that prevents drift | `nlritype` registry (the three-way recognizer registered by the owning NLRI plugin) |
| Verification that proves it | the `.ci` files and interop scenarios above |

### Deliverables Checklist

Added at closure (2026-10-21): the design phase left this table and the next one out.

| Deliverable | Verification method |
|-------------|---------------------|
| D1 low-nibble clear on every write | `grep -n 'attrFlagsUnusedMask' internal/component/bgp/reactor/session_egress_normalize.go`; `TestRFC4271WriteClearsLowNibbleEveryAttribute`, `TestRFC4271RelayedLowNibbleClearedOnBothRails` |
| D2 LARGE_COMMUNITY repeats removed on write | `compactLargeCommunities` in `session_egress_normalize.go`; `TestRFC8092WriteRemovesRepeatedLargeCommunity`, `TestRFC8092PolicyAddOfPresentValueSentOnce` |
| D3 malformed MUP NLRI withdrawn, unkeyable discarded | `mup/rfc7606.go::recognizerFor`, `nlritype.RetainUpdate`; `TestMUPMalformedT1STTreatAsWithdraw`, `TestMUPUnkeyableMalformedT1STDiscardedNotRelayed` |
| Q-2 T1ST origination refusal | `mup/encode.go` `errMupT1stRequiresTEID`, `errMupT1stRequiresEndpoint`; `mup/encode_test.go` |
| D4 `rs` source role suppressed upward | `role/otc.go::OTCEgressFilter`; `TestOTCEgressFilter` |
| D5 BLACKHOLE withheld from unagreed eBGP, reload | `filter_community/blackhole.go::blackholeReadvertiseRefused`, `configTx`; `TestRFC7999ReadvertiseWithheldWithoutAgreement`, `TestRFC7999AgreementReloadTransaction` |
| Six `.ci` and five interop scenarios | `ls` of the Wiring and Files to Create rows; `INTEROP_SCENARIO=<name>` runs (AC-13) |
| AC-5 verdicts enforced | `rfc/audit/<stem>.json` verdict per ID |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Peer-reachable panic | no `panic` on a received UPDATE in the egress walk, the MUP recognizer or `nlritype.RetainUpdate` (the scratch-length panic is a caller defect, sized by `Session.typedNLRIKeyScratch`) |
| Bounds on peer input | the egress walk stops on an attribute that overruns the section; `ValidateNLRI` bounds every field against the NLRI length |
| Resource exhaustion | the compaction allocates nothing (fixed 8 KiB stack filter); a withdraw body is allocated only for malformed input, bounded by the message size |
| Fail open | a D5 cache miss reads as "not agreed" (withheld); a parse failure at verify refuses the reload |
| Data race | the D5 cache pointer is swapped under `mu` and read under `mu.RLock` in `egressFilter` |

## Key Insights

- D1 and D2 are send-side sentences; the only point every send passes is `Session.writeUpdateBody`, which already normalizes the Label-Index on its private buffer. Fix there, one walk.
- D2's receive half is already fixed (`586887a063`); the live transmit producers are the community policy handler's Add and `ingressTagCommunities`, both covered by the egress walk, which replaces the handler fix the 2026-10-13 journal row planned.
- D3 needs a per-NLRI withdraw outcome in the existing typed NLRI pass, carried by a three-way recognizer verdict; `ParseMUP` already knows every malformed shape; keyMUP keeps TLVs out of the key, so the withdrawal hits the earlier generation. An NLRI with no formable key is discarded, never relayed (AC-10a).
- D4 and D5 are owner rulings; the code change under the recommended reading is small (one role in a set; one egress gate reading the `blackholecfg.Parse` map cached beside `peerConfigs` and rebuilt on reload, scoped to bilateral eBGP by `PeerSettings.IsIBGP` under Q-D5 (a1)).
- Ze's own T1ST encoder can emit a route its ingress will now withdraw (Q-2).

### Design Insights (implementation)

- -> Decision (D3, implementation agent 3, 2026-10-14; deviates from the AC-10a Constraint and the N-3 Decision): the key scratch goes to `nlritype.Retain` only, never to the `Recognizer`. The `Recognizer` stays `func([]byte, bool) Verdict`; the key check runs once, inside `Retain` (`judge`), for every family's Withdraw verdict, using the caller's `nlrisplit.PrefixKeyScratchSize` scratch (the `Session.typedNLRIKeyScratch` field). Reason: with Q-1 (a) the MUP recognizer's `ValidateNLRI` refuses every unkeyable implemented-type NLRI, so `Retain` turns each into the counted discard, and a scratch argument no recognizer reads would be dead. The design's properties hold: nothing central names MUP, the discard is key-driven, and the verdict plus its key check allocate nothing (`TestMUPRecognizerVerdicts`).
- -> Decision (rfc/short, 2026-10-14): DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2 keeps its `{gap}`, like 3.1.3.1-4. Both rows read "The NLRI architecture field MUST be encoded as shown above ... Otherwise the NLRI is considered as a malformed", and the recognizer discards a non-3gpp-5g architecture as an unimplemented type rather than withdrawing it, so an earlier route of the same key survives. The nine other rows Q-1 (a) made false lost their gap markers, each with a tagged positive and negative ingress test and a discrimination record.
- -> Decision (interop, implementation agent 6, 2026-10-14): the five scenarios are catalogue entries in `internal/le/interoplab/bgp/checkers.go` (`scenarioOperations`), not bespoke checkers. Each is an ordered list of existing operations, so `TestEveryScenarioOperationUsesRecorder` and `TestEveryScenarioOperationRejectsContradictoryRecorder` drive both polarities, and no `TestBespokeCheckerBranches` subtest is owed. Every negative carries positive proof: `opRequireAbsent` and `opWaitAbsent` name a proof token, and each `opFRRRouteAbsent` follows a control route at FRR and Ze's `show bgp rib status` routes-in count.
- -> Decision (D5 interop, deviates from the scenario table's "BIRD (sender, agreed)"): BIRD's session carries no `blackhole` block. An agreement there makes Ze honor the route and add the NO_EXPORT guard (`blackholePropagationGuard`, `filter_community/blackhole.go`), and NO_EXPORT alone keeps the route off FRR, so the scenario could not go red with `blackholeReadvertiseRefused` reverted. The `.ci` source peer A has no agreement for the same reason.
- -> Constraint (interop, 2026-10-14): NOT RUN. The Docker host kernel check refuses on this machine, so no scenario has a green or a red run, and AC-13 is unmet. The GoBGP output shapes the assertions read are unverified until the first run: `"flags":224` and `"type":241` in `gobgp global rib -j`, `{LargeCommunity: a, b, c}` in the text table, and the T1ST prefix under `-a ipv4-mup`. A-2, A-3 and A-4 stay unvalidated with them.
- -> Constraint (interop, first runs, 2026-10-21, macOS QEMU docker-lab over a `git archive` of ce28145f2f, logs in `plan/handover/15-update-propagation-interop.md`): D1 `bgp-attr-flags-low-nibble-gobgp` PASS green (A-2 confirmed: GoBGP prints `"flags":224`). D2 FAIL on the assertion only: Ze sent each value once, GoBGP prints `{LargeCommunity: [ 65000:1:1, 65000:3:3, 65000:2:2]}` with brackets; the checker string is corrected (A-3 confirmed). D3 FAIL: GoBGP 3.31.0 panics (`MUPType1SessionTransformedRoute.DecodeFromBytes`, index out of range) decoding Ze's relayed MP_UNREACH_NLRI, because the treat-as-withdraw withdrawal carries the malformed K' bytes as received, not the bytes of the generation Ze advertised. D4 FAIL: Ze's `show bgp rib status` reports `routes-in 0` after the injector's UPDATE arrives. D5 FAIL: FRR never gets the control route; Ze logs "peer-up replay gave up waiting for adj-rib-in to reach the cut". D4 and D5 both log that bgp-rib and bgp-adj-rib-in are attached to no peer. No red run yet.

## Implementation Steps

1. Owner answers recorded: Q-5 on 2026-10-13; Q-D4 (a), Q-D5 (a1), Q-1 (a), Q-2 (a), Q-3 (a), Q-4 (a) on 2026-10-14 ("Do what you think is right"), in the Owner decisions rows.
2. Read `docs/contributing/ze-go-style.md`; run `./le spec citation anchors` (Documentation row 16).
3. D1 and D2: failing units, then the joined egress walk (after the Prefix-SID spec, or carrying its labeled-unicast gate, R-9); page edits for row 7 in the same step; prepare the benchmark, and the owner runs it on the Mac (R-1).
4. D3: registry and recognizer units red, then the three-way verdict, the MUP recognizer's withdraw verdict, the withdraw and unkeyable-discard outcomes and dispatch; `.ci`; R-5 run of the MUP functional tests.
5. D4 and D5 per ruling, with the tagged-unit change carrying `RFC-approved:` trailers; under Q-D5 (a1) the `IsIBGP` field, the cache and its reload handlers (AC-12a). The D3 commit carries the D3 `RFC-approved:` trailers.
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

## Design Review Log

| Date | Round | Outcome |
|------|-------|---------|
| 2026-10-14 | design review round 1 (0 BLOCKER, 5 ISSUE, 8 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-update-propagation-design-r1.md`) | Applied by a separate editing agent, Edit tool only, each finding re-read at the producer. I-1 Q-2 recommends (a) under either Q-1 answer (`writeT1STData`, `parseTEIDWithBits`, `parseBodyT1ST` `ErrMUPTruncated`). I-2 AC-10 (iv) became AC-10a: an unkeyable malformed MUP NLRI is discarded and counted, never relayed; RFC 7606 Section 2 and the draft's "skip such NLRIs" quoted; the writer path traced (`adjOutWrite.key`, `fwdPathKeyFor`, `allow`, `ownership.filter` in `writeUpdateBody`, the forward worker abandoning the batch); new test `TestMUPUnkeyableMalformedT1STDiscardedNotRelayed`. I-3 Q-D5 (a) split into (a1) bilateral eBGP only and (a2) every bilateral session, the RS rail excluded from both; each option states its iBGP, RS and confederation consequence; the recommendation became (a1); the new `filter_community` config consumer and its config-time cache named in Files to Modify; AC-12, Wiring, Functional and Interop rows follow. I-4 the 2026-10-13 row of `plan/journal/non-idempotent-append-duplicates-the-record.md` cited, the egress walk replaces its handler fix, `ingressTagCommunities` named, the row's Fix cell points here. I-5 A-1 restated by symbol (`writeMessageWithin`, `SendRawMessage` both modes); every hand-typed line number replaced by a symbol or section name. N-1 Blast Radius added (RS-replacement and drain specs, A-6 dependency). N-2 R-1 budget and `BenchmarkWriteUpdateBodyEgressWalk`; AC-8 pins 0 allocs on the repeat path. N-3 three-way `Recognizer` verdict adopted, the second function kind named as not taken. N-4 the duplicate owner table dropped. N-5 headings re-levelled. N-6 Blast Radius, End-to-End User Stories, Files to Create, Boundary Tests added. N-7 Q-D5 (b)'s consequence reworded (NO_EXPORT passes iBGP and confederation-internal eBGP). N-8 approval commands moved into a table. Q-D4, Q-D5 and Q-1 to Q-4 stay OPEN. Round 2 owed, narrow: the Q-2 and Q-D5 rows, AC-10a and its Decision, the three-way verdict (Current Behavior, Files, TDD rows), A-1, A-6, R-1 budget, Blast Radius |
| 2026-10-14 | design review round 2 (0 BLOCKER, 7 ISSUE, 5 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-update-propagation-design-r2.md`) | Applied by a separate editing agent, Edit tool only, each finding re-read at the producer at HEAD `68826acc76`. I-1 every RFC-tagged unit the D3 verdict type reaches listed (grep over `nlritype` and recognizer helper callers), each with its own `./le rfc approve unit` row; the false "No other existing tagged unit changes" removed. I-2 the unkeyable discard made key-driven under either Q-1 answer (the MUP recognizer asks `GetPrefixKey` first); AC-10a restated with three shapes; two boundary rows. I-3 the D5 gate reads a new `PeerFilterInfo.IsIBGP` filled from `PeerSettings.IsIBGP`; the open-coded `PeerAS != LocalAS` dropped; the journal row in `helper-bypassed-by-an-open-coded-copy.md` cited. I-4 the confederation clause dropped from Q-D5 (b) and its reason; (a1)'s note rests on Ze having no confederation config and `wireu.WellKnown.AllowsEgressTo`. I-5 the `blackholecfg.Parse` map cached beside `peerConfigs`, looked up by `LookupPeerConfig`, outside the `hasAnyRule` drop, gate before `egressFilter`'s `!hasCfg` return. I-6 verify, apply and rollback handlers for the cache, AC-12a, `test/reload/rfc7999-blackhole-agreement-removed-by-reload.ci`; the other rules' reload gap journaled in `plan/journal/unwired-feature.md` (2026-10-14). I-7 A-6 restated from the RS-replacement spec's decision; AC-11 and AC-12 assert "replaced by an eligible survivor, else withdrawn", one fixture each way; R-10. Cross-spec: the Prefix-SID spec's labeled-unicast gate on `clearTransmittedLabelIndex` recorded (R-9, Files, Blast Radius); "no behaviour change" made conditional. N-1 to N-5 applied. Q-D4, Q-D5 and Q-1 to Q-4 stay OPEN. Round 3 owed, narrow: the Q-D5 row, AC-10a, AC-11, AC-12, AC-12a, A-6, R-9, R-10, the tagged-unit and approval tables, Files to Modify |
| 2026-10-14 | design review round 3 (0 BLOCKER, 5 ISSUE, 2 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-update-propagation-design-r3.md`) | Applied by a separate editing agent, Edit tool only, each finding re-read at the producer. I-1 `commit/rfcchange.go::changedRFCUnits` compares tagged top-level function text (`rfc.FunctionUnits`), and the four D3 helpers are top-level functions, so the "only if the gate names it" rule now covers all 16 helper-only units; only the two `TestRecognizeNLRIRejectsUnimplementedTypes` units stay unconditional; the gate explained in the approval paragraph. I-2 `keyMUP` refuses a scratch shorter than `3+keyLen` (largest MUP key 32 octets, T2ST); the recognizer gets a caller-owned `nlrisplit.PrefixKeyScratchSize` scratch, a `Session` field passed through `Retain`; `TestMUPRecognizerVerdicts` asserts 0 allocations; R-8 restated. I-3 D5 gate order: `IsIBGP`/`RSClient`, then `blackholecfg.Carries`, then the lookup, from a cache re-keyed at config time into a `netip.Addr` map plus a `PeerConfigKey` map (no `.String()` per route); `AllocsPerRun` cases in the D5 unit row. I-4 the `EgressFilterFunc` doc edit planned in the `filterapi.go` row and its non-decision paragraph re-derived (a cache miss is the "not agreed" decision). I-5 `TestRFC8669LabelIndexTransmissionClearsReservedAndFlags` added to the tagged-change and approval tables on R-9's land-first branch (Prefix-SID spec Q5 row 9). N-1 `sendStaleReadvertiseUnit` fills `RSClient` too, with why it is harmless today. N-2 "every other destination fill". Prefix-SID round 2 N-5: the `session_prefix_sid.go` Files row and Documentation row 16 (`docs/features/srv6.md`) now state both R-9 branches. Q-D4, Q-D5 and Q-1 to Q-4 stay OPEN. Round 4 owed, narrow: the tagged-unit and approval tables, AC-10a's scratch Constraint and R-8, the Q-D5 row's gate order and cache keys, the `filterapi.go`, `session_prefix_sid.go` and destination-fill Files rows, Documentation row 16, R-9 |
| 2026-10-14 | design review round 4 (0 BLOCKER, 1 ISSUE, 2 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-update-propagation-design-r4.md`) | Applied by the main thread. I-1: the blackhole cache re-key splits on `PeerConfigKey.Template`, not on whether the ID parses, because a group may be named like an address; unit case added. N-1: the swap happens under the package `mu` that guards `peerConfigs`. N-2: the MUP key prefix is the architecture type and the 2-octet route type. Round 5 owed, narrow, over these three edits |
| 2026-10-14 | design review round 5, narrow (0 BLOCKER, 0 ISSUE, 1 NIT; `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-update-propagation-design-r5.md`) | NIT applied by the main thread: every non-template blackhole entry is an address by `Parse` construction. Design converged; Q-D4, Q-D5 and Q-1..Q-4 stay with the owner |
| 2026-10-14 | owner answers recorded | Owner, 2026-10-14: "Do what you think is right", accepting every recommendation: Q-D4 (a), Q-D5 (a1), Q-1 (a), Q-2 (a), Q-3 (a), Q-4 (a). Removed the unchosen branches: the Q-D5 (a2), (b) and (c) outcomes in AC-12 and AC-12a, the (b)-only tagged-test row and its approval command for `TestRFC7999PropagationCommunityIsNeverForced` (unchanged under (a1)), the YANG-default text in Files, Documentation, Blast Radius and the checklist, the "under either Q-1 answer" hedges, and the "only under" conditionals on D4, Q-2 and the D5 wiring, user-story, `.ci`, interop and Files rows. A-7, R-5, R-6, the D4/D5 expectation rows, Required Reading and Implementation step 1 now name the answers. No answer needs new design: each was the reading the design already proceeded under |

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
