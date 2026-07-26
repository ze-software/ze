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
- Supports neighbor qualifiers for multi-session matching:
  - `neighbor <IP> local-as <ASN> announce route ...`
  - `neighbor <IP> peer-as <ASN> announce route ...`
  - `neighbor <IP> local-ip <IP> announce route ...`
  - `neighbor <IP> router-id <IP> announce route ...`
- Commands only apply to sessions matching ALL specified qualifiers
- Enables targeting specific sessions when multiple sessions exist to same peer

**ZeBGP behavior:**
- Uses `peer` keyword: `peer <IP> update text ...`
- Does NOT support multi-session qualifier syntax
- Commands apply to all sessions matching the peer IP

**RFC compliance:**
- N/A - This is API syntax, not BGP protocol

**Impact:**
- API tests using `local-as`, `peer-as`, `local-ip`, `router-id` qualifiers will NOT work
- Test scripts must be simplified to use basic `neighbor <IP>` syntax
- Tests requiring multi-session discrimination are NOT SUPPORTED

**Tests affected:**
- `announcement.run` - Uses all qualifier types
- Any test requiring multi-session targeting

**Decision rationale:**
1. Multi-session to same peer is a rare use case
2. Simpler API implementation
3. Most use cases only need single session per peer
4. Can be added later if needed

**Date:** 2025-12-23

---

## Template for Future Differences

### Feature Name

**ExaBGP behavior:** [Description]

**ZeBGP behavior:** [Description]

**RFC compliance:** [Analysis]

**Impact:** [Testing/compatibility notes]

**Files affected:** [List]

**Decision rationale:** [Why ZeBGP differs]
