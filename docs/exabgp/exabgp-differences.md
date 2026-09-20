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
- The ExaBGP bridge PARSES all six qualifiers and DISCARDS five of them. The
  command reaches the session the address names.
- A Ze selector names a peer by address, name, ASN or glob. It cannot conjoin a
  predicate onto an address, so there is nothing to translate a qualifier into.
- The exception is `family-allowed in-open`, which no Ze session can satisfy.
  ExaBGP's `in-open` names a session that negotiates the families the OPEN
  carried; Ze STATES the families its OPEN offers and has no mode that defers
  the choice. The bridge therefore treats the address it qualifies as excluded,
  dispatches nothing, and still answers `done`, which is what ExaBGP does with a
  command whose selector matches no session.
<!-- source: internal/exabgp/bridge/bridge_selector.go -- bridgeSelectorKeys, splitNeighborSelector, selectorExcludes -->

**RFC compliance:**
- N/A - This is API syntax, not BGP protocol

**Impact:**
- A qualified command is accepted and reaches the peer the address names,
  unless the qualifier is `family-allowed in-open`.
- The cost is bounded in one direction. An address resolves to at most one
  session in Ze, so a qualifier could only ever have REJECTED that session. Ze
  therefore sends where ExaBGP would have stayed silent, and never the reverse.
- One qualifier VALUE does exclude, because it can never match: see
  `family-allowed in-open` above. Any other value could name Ze's single
  session, so it is discarded rather than evaluated.

**Tests affected:**
- `test/exabgp-compat/etc/run/api-multisession.run` sends
  `neighbor 127.0.0.1 local-as 1 family-allowed in-open announce route 9.9.9.9/24`,
  which ExaBGP drops because the session is named `family-allowed ipv4-unicast`
  under `multi-session`. `test/exabgp-compat/api/api-multisession.ci` expects
  four UPDATE frames from five commands for that reason, and Ze now sends four
  and acknowledges five.
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
earlier text said such commands "will NOT work". Corrected again the same day:
`family-allowed in-open` now excludes, so the claim that no qualifier ever does
is gone.

---

## JSON member spellings the bridge folds

Five members carry the same fact under different words. Ze keeps its own word,
because each is load-bearing on Ze's side, and the bridge writes ExaBGP's word
so a script reads one vocabulary.

| Fact | Ze writes | ExaBGP writes | Why Ze keeps its word |
|------|-----------|---------------|-----------------------|
| Route distinguisher | `0:65000:1` | `65000:1` | Type 0 and Type 2 are indistinguishable for an AS number of 65535 or less, and `ParseRDString` has to read `String()`'s own output back |
| RFC 7911 Path Identifier | `"path-id": 16909060` | `"path-information": "1.2.3.4"` | RFC 7911 Section 3 calls it a 4-octet value, and Ze's CLI and RIB read the number |
| FlowSpec DSCP on IPv6 | `dscp` | `traffic-class` | RFC 8956 Section 8 registers component type 11 as `DSCP` under both its IPv4 and its IPv6 name |
| FlowSpec traffic-marking and traffic-action | `mark:10`, `traffic-action:sample` | `mark 10`, `action sample` | the FlowSpec firewall lowering keys on the colon form |
| Interface set transitivity | `interface-set:non-transitive:input:1234:10` | `interface-set:input:1234:10` plus `"transitive": false` | Ze carries every extended community in one flat list, and ExaBGP has a `scope` block to state it beside |

Nothing is lost either way, so each is a spelling rather than a conversion. The
RFC governs the wire and says nothing about a JSON member, which is what leaves
the choice open.

Where an RFC or Ze's own grammar DOES decide, Ze changed instead of folding: a
two-octet AS Route Origin renders its 4-octet local administrator as a dotted
quad in both projects, and so do `l2info` and the FlowSpec redirect-to-IP
communities, because Ze's config parser already read those spellings and its
renderer wrote something the parser refuses.
<!-- source: internal/exabgp/bridge/bridge_event.go -- exabgpRD, exabgpPathInformation, exabgpFlowComponentName -->
<!-- source: internal/exabgp/bridge/bridge_wire_json.go -- exabgpCommunitySpellings, exabgpInterfaceSet -->

---

## The live bridge writes less than a fixture checks

`WireUpdateToExabgpJSON` renders an UPDATE from its WIRE BYTES, so it can state
three members Ze's own JSON does not carry: the AS_PATH's segments and their
types, the numeric value beside each extended community, and the flag octet in
an unknown attribute's `attribute-0xCC-0xFF` name.

The community value is read for both widths, RFC 4360's 8 octets and RFC 5701's
20, and it is built exactly and rounded once. RFC 5701's community is 160 bits
wide and JSON has one number type, so accumulating the octets in a float rounds
at every step: doing that put the member two thousand off an expectation whose
own decimal literal rounds cleanly.

One community gains a second member here. ExaBGP writes `"transitive"` beside
the interface set and beside nothing else, because that community
(draft-ietf-idr-flowspec-interfaceset Section 5) is defined twice, transitive
and non-transitive, differing by one bit of the type octet. Ze states that bit
in the text instead, since it carries every extended community in one flat list
and has no `scope` block to put it in.

The live bridge cannot. `translateAndForward` receives Ze's JSON event as text
and has no wire to read, so an ExaBGP process attached to a running Ze gets Ze's
flattened AS_PATH and no community values. The ExaBGP compatibility fixtures go
through the wire path, so they check more than a live process receives.

Closing it means the bridge subscribing at `FormatFull`, which carries
`raw.update` beside the parsed body, and rendering from those bytes. Until then
this page is the only statement of what the fixtures do not prove.
<!-- source: internal/exabgp/bridge/bridge_wire_json.go -- WireUpdateToExabgpJSON, overlayASPath, overlayExtendedCommunities, overlayUnknownAttributes -->
<!-- source: internal/exabgp/bridge/bridge.go -- translateAndForward -->

---

## Template for Future Differences

### Feature Name

**ExaBGP behavior:** [Description]

**ZeBGP behavior:** [Description]

**RFC compliance:** [Analysis]

**Impact:** [Testing/compatibility notes]

**Files affected:** [List]

**Decision rationale:** [Why ZeBGP differs]
