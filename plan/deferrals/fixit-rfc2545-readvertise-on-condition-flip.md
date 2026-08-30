# Deferrals: fixit-rfc2545-readvertise-on-condition-flip

One issue, recorded not fixed. The aggregate live backlog is folded on read from
`plan/deferrals/` by `/ze-status`. Nothing stores it (`ai/rules/planning.md`).

**Issue:** when RFC 2545 Section 3's condition flips, the peer keeps the next hop
the old condition produced.

**Owner ruling, 2026-08-08:** "re-advertise the affected prefixes". Given after
BIRD and FRR were read at source, so it is a deliberate choice to exceed both.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-08 | Thomas's ruling on B-16, after the closing review of the RFC 2545 work in `ec3ad9c76` | An interface address event that flips RFC 2545 Section 3's shared-subnet condition must re-advertise the prefixes that condition governs. `(*Reactor).refreshPeerLinkScopes` (`internal/component/bgp/reactor/reactor_iface.go`) re-settles the snapshot and deliberately does not re-advertise, so when the condition goes FALSE a peer keeps a link-local next hop it can no longer resolve on its own link, until the next update for that prefix. The false-to-true direction is harmless and neither reference corrects it | The reading ze took is correct and is NOT overturned: Section 3 binds the act of advertising, and `rfc/short/rfc2545.md` puts usability on the receiver. What the ruling adds is that a flip is itself an event owing the peer a new advertisement, taking the RFC 4271 Section 9.2 Update-Send reading. **This exceeds both references, knowingly.** BIRD never re-advertises on an interface event and does not evaluate the condition dynamically at all; FRR re-announces on an address change but guards both call sites `!IN6_IS_ADDR_LINKLOCAL` and freezes `shared_network` at establishment, likely because it is an update-group key. The shape to copy is FRR's per-peer forced re-announce scoped to that peer's channels, triggered the way BIRD triggers `channel_request_feeding` from `channel_roa_out_changed`, which is its one refeed trigger that is neither a config nor a protocol event. **CORRECTED 2026-08-30: this is NOT an RFC violation.** RFC 2545 Section 3 binds the act of advertising, not re-advertising, so what the ruling asks for is an addition Thomas wants rather than conformance ze owes. It stays live because he ruled for it, not because the wire is wrong. Cost note: ze has no Adj-RIB-Out refeed primitive today, and building one is the bulk of the work | `plan/future/` (needs its own spec) | deferred |
