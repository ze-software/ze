# ZeBGP vs ExaBGP Behavioral Differences

This file documents intentional differences between ZeBGP and ExaBGP behavior.
These are not bugs - they are design decisions where ZeBGP diverges from ExaBGP.

**Impact on testing:** When `.ci` files from ExaBGP tests don't match ZeBGP output
due to these differences, update the `.ci` files to match ZeBGP's behavior.

---

## Attribute Ordering in UPDATE Messages -- RESOLVED, no longer a difference

This entry described Ze emitting attributes in RFC 4271 Section 5 description
order (MP_REACH_NLRI after LARGE_COMMUNITY) while ExaBGP sorted by type code.
That is no longer true, and the entry is kept only so the change is traceable.

Ze now keeps attributes in ascending type-code order in every builder, which is
what ExaBGP does and what RFC 4271 Appendix F.3 describes:

- `internal/component/bgp/message/update_build.go` sorts explicitly by `Code()`.
- `internal/component/bgp/reactor/reactor_api_batch.go` appends MP_REACH_NLRI
  (14) after the lower-coded attributes it emits, and AS4_PATH (17) last.
- `internal/component/bgp/reactor/peer_rib_routes.go` writes MP_REACH_NLRI
  between the lower-coded optional attributes (ATOMIC_AGGREGATE 6, AGGREGATOR 7,
  COMMUNITIES 8, ORIGINATOR_ID 9, CLUSTER_LIST 10) and the higher-coded ones
  (EXT_COMMUNITIES 16, IPV6_EXT_COMMUNITIES 25, LARGE_COMMUNITIES 32).

The third builder was the last one out of order. Which of the three runs is
decided by `Peer.ShouldQueue()`, that is by timing, so one route encoded to two
different byte strings depending on whether it drained through the initial-sync
queue or the post-establishment batch builder. Aligning it removed both the
ExaBGP difference and the non-determinism.
<!-- source: internal/component/bgp/message/update_build.go -- sort.Slice by Code(), "per RFC 4271 Appendix F.3" -->
<!-- source: internal/component/bgp/reactor/peer_rib_routes.go -- MP_REACH written between lower- and higher-coded attributes -->

---

## Neighbor Qualifier Syntax (Multi-Session)

**ExaBGP behavior:**
- Supports six qualifiers after a neighbor address, each taking one value:
  `local-ip`, `local-address`, `local-as`, `peer-as`, `router-id`,
  `family-allowed`.
  - `neighbor <IP> local-as <ASN> announce route ...`
  - `neighbor <IP> family-allowed <families> announce route ...`
- A qualifier NARROWS the set of sessions the command reaches. ExaBGP matches
  each qualifier as a substring of the session's own name, so a command whose
  qualifier does not appear in that name reaches no session and sends nothing.
- `family-allowed` is where `capability { multi-session; }` shows: the name
  carries `family-allowed in-open` on an ordinary session, and the session's
  family list on a multi-session one.
Upstream (exa-networks/exabgp): `src/exabgp/reactor/api/command/limit.py`,
`SELECTOR_KEYS` and `match_neighbor`; `src/exabgp/bgp/neighbor/neighbor.py`,
`name()` and its `in-open` / family-list branch.

**ZeBGP behavior:**
- Uses the send verb and the protocol keyword: `send bgp <IP> update text ...`
- The ExaBGP bridge PARSES all six qualifiers and then DISCARDS them. The
  command reaches the session the address names.
- A Ze selector names a peer by address, name, ASN or glob. It cannot conjoin a
  predicate onto an address, so there is nothing to translate a qualifier into.
<!-- source: internal/exabgp/bridge/bridge_selector.go -- bridgeSelectorKeys, splitNeighborSelector -->

**RFC compliance:**
- N/A - This is API syntax, not BGP protocol

**Impact:**
- A qualified command is accepted and reaches the peer the address names. The
  qualifier changes nothing.
- The cost is bounded in one direction. An address resolves to at most one
  session in Ze, so a qualifier could only ever have REJECTED that session. Ze
  therefore sends where ExaBGP would have stayed silent, and never the reverse.
- A test that needs a qualifier to EXCLUDE a session is not supported.

**Tests affected:**
- `test/exabgp-compat/etc/run/api-multisession.run` sends
  `neighbor 127.0.0.1 local-as 1 family-allowed in-open announce route 9.9.9.9/24`,
  which ExaBGP drops because the session is named `family-allowed ipv4-unicast`
  under `multi-session`. `test/exabgp-compat/api/api-multisession.ci` expects
  four UPDATE frames from five commands for that reason. Ze would send all five.
- `test/exabgp-compat/etc/run/api-announcement.run` uses the other qualifier
  types, none of which excludes a session there.

**Decision rationale:**
1. Multi-session to same peer is a rare use case
2. Simpler API implementation
3. Most use cases only need single session per peer
4. A conjunctive selector is a change to the Ze command grammar, which is not
   the bridge's to make

**Date:** 2025-12-23. Corrected 2026-09-06: the bridge parses the qualifiers
since 2026-09-05, so a qualified command no longer fails to translate. The
earlier text said such commands "will NOT work".

---

## Template for Future Differences

### Feature Name

**ExaBGP behavior:** [Description]

**ZeBGP behavior:** [Description]

**RFC compliance:** [Analysis]

**Impact:** [Testing/compatibility notes]

**Files affected:** [List]

**Decision rationale:** [Why ZeBGP differs]
