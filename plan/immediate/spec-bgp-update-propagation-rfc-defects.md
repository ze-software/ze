# Spec: bgp-update-propagation-rfc-defects

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-27 |

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
| D6 | RFC 2545 Section 3; draft-ietf-idr-linklocal-capability-06 Section 4 item 1 (DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2) when capability 77 is negotiated | RFC 2545: "The link-local address shall be included in the Next Hop field if and only if the BGP speaker shares a common subnet with the entity identified by the global IPv6 address carried in the Network Address of Next Hop field and the peer the route is being advertised to." Draft: "If the internal peer is more than one IP hop away, the BGP speaker MUST NOT include a Link-Local IPv6 next hop." | `internal/component/bgp/reactor/forward_next_hop.go::egressNextHopGlobalHalf` (the 32-octet cut, landed in 20fec75f48 on both rails); the withhold gates of `reactor_api_forward.go::forwardUpdateCore` and `forward_rs.go::reactorForwardRS`; `forward_build.go::buildWithdrawalPayload`. `applyNextHopMod` has no production caller and decides nothing | The 32-octet pair is cut to its Global towards a peer off the link, but a received Link-Local-only next hop still crossed to a multihop peer under `unchanged`/`auto`, and every withhold gate sent nothing (or only the source's own withdrawals) where the destination may hold the previous generation | 4-2 had no relay test; 4-1, -4-4, -4-7, -4-9 reproduced red (below) |

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
rails use `wireu.WithdrawalsOnly` on the replacement announcement, which
contains no withdrawal, and send nothing. Section 4's one-hop default procedure
says: "If no next hops are included, the route MUST NOT be announced
(treat-as-withdraw)." D6 owns this missing withdrawal too; its probe remains
untagged, with no enforced verdict claimed.

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | One test per row, red against HEAD, over the forward path (not the codec alone) |
| Both polarities | D1: set bits cleared on send, clear bits unchanged. D2: duplicates removed, distinct values kept in order. D3: each malformed shape withdrawn, a well-formed route with an unknown TLV kept and propagated unchanged |
| Discrimination | `./le rfc discriminate-record` for every tagged unit |
| Real entry point | `.ci` tests: a peer sends the UPDATE, a second peer receives the re-advertisement |
| D6 cases | a received Global plus Link-Local next hop forwarded under next-hop auto to a multihop internal peer carries the global address only (red against HEAD); to an internal peer on the same link it may keep the link-local. The multihop EBGP twin (4-8, "Link-Local IPv6 next hops MUST NOT be included.") goes through the eBGP next-hop rewrite, which was not traced: the same test covers it |
| Interop | D1, D2 against FRR or BIRD as the receiving peer; D4 against a Role-capable FRR; D6 with FRR as a multihop eBGP receiver (`bgp-linklocal-only-multihop-withdraw-frr`, FRR AS 65002, Ze AS 65001, 834f7d26e2). The multihop internal case (4-1, 4-2) is proven by unit tests only |

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
different AFI/SAFI the result needs two messages, and the function refuses that
shape rather than send half of it.
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

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md`, `rfc/short/rfc8092.md`, `rfc/short/rfc9234.md`, `rfc/short/rfc7999.md`, `rfc/drafts/draft-ietf-bess-mup-safi.txt` Section 3.1.3.1

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/bgp/attribute/opaque.go` - flags preserved
- [ ] `internal/core/bgp/attribute/attribute.go` - `WriteHeaderTo`
- [ ] `internal/component/bgp/message/rfc7606.go` - `validateLargeCommunityAttr`
- [ ] `internal/core/bgp/nlri/nlrisplit/mup.go` - `SplitMUP`
- [ ] `internal/component/bgp/plugins/role/otc.go` - `OTCEgressFilter`
- [ ] `internal/component/bgp/plugins/cmd/announce/blackhole_agreement.go` - `agreedSelector`

**Behavior to change:** D1 to D3; D4 and D5 after the owner rules.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- a BGP UPDATE received from a peer

### Transformation Path
1. RFC 7606 validation and NLRI split
2. RIB insert and best path
3. egress filters (OTC, community guard) and the forward path to other peers

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ filter plugins | egress filter calls with wire payload | No |

### Integration Points
- the zero-copy forward path and the per-peer egress filter chain

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE from peer A, re-advertised to peer B | → | forward path and egress filters | [to fill in design: `.ci`] |
| D6: Link-Local-only UPDATE from an on-link peer, relayed to a multihop peer | → | `egressNextHopWithheld`, `egressNextHopLinkLocalOnlyOffLink`, `buildWithdrawalPayload` (RS rail) | `test/plugin/linklocal-only-multihop-withdraw.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | unknown transitive attribute with flag bits 0x0F set | forwarded with those bits zero |
| AC-2 | LARGE_COMMUNITY with a repeated value | forwarded once per value |
| AC-3 | MUP ST1 route with truncated mandatory fields or a bad TLV | treat-as-withdraw |
| AC-4 | D4, D5 | as the owner rules |
| AC-5 | the `weak` verdicts of RFC4271-4.3-3, RFC4271-4.3-4, RFC8092-3-1, DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6, DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-7, DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-10, RFC9234-3.1-1, RFC7999-3.1-2 and DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1, -4-2, -4-4, -4-7, -4-9, after this spec's producer fix | each verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. D6 covers missing-global internal and external multihop cases, loss of usable next hops at a directly attached external peer, withdrawal of an already advertised generation, and usable-global controls. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (parent P-3; original transfer 2026-09-28, missing-next-hop extension 2026-10-02) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| one failing-first test per row | beside each producer | D1 to D5 | [to fill in design] |

### Functional Tests
- [to fill in design]

## Files to Modify

- the producers in the defect table and their tests

## Implementation Steps

1. Ask the owner D4 and D5; 2. failing tests; 3. fixes; 4. discrimination and verdicts.

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
| cap 77 | a received Link-Local-only next hop crossed unchanged to a peer without capability 77 | implemented, not committed: `egressNextHopLinkLocalOnlyRefused` asks the emitted next hop, written or received. Fixtures `llnhClient` and `llnhExternalPeer` negotiate capability 77. Waits for owner approval of the tagged units in `draft_ietf_idr_linklocal_capability_reflect_test.go` and `draft_ietf_idr_linklocal_capability_multihop_test.go` | `TestReceivedLinkLocalOnlyWithdrawnFromPeerWithoutCapability` red without the gate change on both rails |
