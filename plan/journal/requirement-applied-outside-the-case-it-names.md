# A requirement is applied outside the case it names

A rule that one document, protocol or variant states is applied to a neighbour
it does not govern: a sequence rule RFC 7474 defines for AuType 3 applied to
AuType 2, a BGP hold-time habit applied to LDP's KeepAlive Time. The code is
consistent with the rule it borrowed, and every test derived from that rule
agrees with it, so only the text that governs the case shows the error.

The repair quotes, at the code that implements the case, the sentence of the
document that governs it (`ai/rules/rfc-compliance.md`). A borrowed rule then
shows as a quote that does not say what the code does.

Split out of `paraphrase-becomes-the-authority` on 2026-10-14
(spec-journal-class-lifecycle, Phase 4): neither row is a paraphrase of a
requirement; each is a requirement of another case.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-20 | - (rfc8362 test triage, walked into) | `v6PrefixSIDsFromTLV` via `engine.v6ReceivedPrefixSIDs` (`internal/plugins/ospf/sr_reception_v6.go`) | Prefix-SIDs are read from an Inter-Area-Prefix (3), External-Prefix (5) or Intra-Area-Prefix (6) TLV in any of the four E-prefix LSA types, because the TLV case never sees the LS type. RFC 8362 Sections 3.4, 3.6 and 3.7 make each TLV only applicable to its own LSAs: "Inclusion in other Extended LSAs MUST be ignored." Read at the producer | open: pass the LS type in, accept each TLV only from its LSAs, negative test per pair |
| 2026-10-18 | - (RFC9494-4.2-8 test backfill) | `onSessionReestablished` (`bgp/plugins/gr/gr_state.go`) | Read, not run: on an LLGR reconnect the GR F bits seed `newForwarding` and LLGR F bits are only ORed in, so a family with GR F=1 keeps its stale routes when the new LLGR capability clears its F bit or omits it; RFC 9494 Section 4.2 says remove them | fixed: `reconnectKeepsFamily` keeps a family only when new GR sets its F bit AND, in its LLGR period, new LLGR lists it with F set (owner Q-1) |
| 2026-09-27 | spec-rfc-requirement-quote-hand-backfill | ospf auth replay | RFC 7474 §2's strictly-greater sequence rule, defined for its AuType 3, was applied to AuType 2, where RFC 2328 D.4.3 accepts an equal sequence number | fixed: fixed (DF-OSPF-B): replayedSequence branches on AuType |
| 2026-10-13 | ldp-keepalive-expiry-notification (design) | `handleInit` and `NewSession` (`internal/plugins/ldp/session.go`), sizing `holdTime`; `docs/guide/mpls.md` "closes the session when no PDU arrives inside three times that value" | KeepAlive expiry fires at 3x the negotiated KeepAlive Time ("hold time is typically 3x keepalive", a BGP habit); RFC 5036 3.5.3 makes KeepAlive Time itself the maximum gap between PDUs. Guide and RFC5036-3.5.3-1 test pin the 3x | fixed: Fixed 2026-10-14 (spec phase 4): `handleInit` arms the negotiated time |
| 2026-10-20 | spec-bgp-prefix-sid-rfc-defects | FRR 10.3.1 bgpd Prefix-SID receive, interop `bgp-prefix-sid-malformed-frr` | FRR answers a Label-Index TLV whose length is not 7 with NOTIFICATION 3/5 on every family, VPN included, where RFC 8669 3.1 says "It MUST be ignored when received for other BGP AFI/SAFI combinations." ze relays such an attribute unchanged off labeled unicast (AC-1a), so an FRR neighbour of ze resets | open: FRR's defect, not ze's; the scenario's VPN control no longer carries the TLV, and case (c) is proved by `prefixsid-malformed-receive.ci` |
