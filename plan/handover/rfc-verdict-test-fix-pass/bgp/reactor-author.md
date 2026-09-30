# reactor author handoff (child spec-rfc-verdict-fix-bgp, package internal/component/bgp/reactor)

Derived listing restricted to reactor: 75 verdicts. Blocked (dropped per "Blocked by"): DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2, RFC6286-2.1-1. Worked list: 73.
Resolved: 9 (8 tests, 1 defect fixed with tests). Unresolved, not reached: 64 (tool-call budget).
No verdict stamped. Nothing committed. No existing tagged unit edited, so no `./le rfc approve unit` was needed.

## Resolved

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|------------------------------------|-----------------|------------------|-------|
| RFC4271-6.3-5 | tests | NEW `TestRFC4271RecognizedAttributeErrorsBeyondTheFirst` (rfc4271_recognized_attribute_errors_test.go): + MED, AGGREGATOR, ATOMIC_AGGREGATE with conflicting Optional/Transitive flags -> exact treat-as-withdraw payload, session Established (RFC 7606 3(c)); - same attributes with specified flags -> prefix kept, value byte-identical | positive + negative, revert `message/rfc7606.go::validateAttributeFlags` | enforced | row is revised by RFC 7606 (as the audit already accepted); the old ORIGIN-flags case stays on TestSessionRFC4271RevisedAttributeErrors |
| RFC4271-6.3-6 | tests | same NEW unit: + ORIGIN length 2 -> withdraw (7606 7.1); ATOMIC_AGGREGATE length 1 and AGGREGATOR length 5 -> attribute discarded, prefix kept (7606 7.6, 7.7); - expected lengths -> attribute present, value equal | positive + negative, revert `message/rfc7606.go::validateAggregatorAttr` | enforced | |
| RFC4271-6.3-14 | tests | same NEW unit: + COMMUNITIES length 3 and EXTENDED COMMUNITIES length 7 -> withdraw; AGGREGATOR bad value -> discard ("MUST be discarded" clause); - valid values kept byte-identical | positive + negative, revert `message/rfc7606.go::validateCommunityAttr` | enforced | |
| RFC8950-4-1 | tests | NEW `TestRFC8950IPv6NextHopFollowsTheNegotiatedPair` (rfc8950_nexthop_pair_test.go), drives `Peer.resolveNextHop` (next-hop self, the rail resolver) with an IPv6 session: + ExtNH for 1/1 only licenses IPv4/unicast, for 1/128 only licenses VPN-IPv4; - the other pair, and both with none, refused with ErrNextHopIncompatible and no address | positive + negative, revert `reactor/peer.go::canUseNextHopFor` | enforced | covers the VPN-IPv4 and per-pair clauses the audit named |
| RFC5549-4-1 | tests | same NEW unit as RFC8950-4-1 | positive + negative, same producer | enforced | superseded row restating 8950 |
| RFC5549-4-4 | tests | same NEW unit as RFC8950-4-1 | positive + negative, same producer | enforced | old tag prose on `TestCanUseNextHopFor_ExtendedNH` still says "the MUST NOT" (audit note); not edited |
| RFC2918-4-2 | defect (D-8) fixed | NEW `TestRFC2918RouteRefreshForAnUnadvertisedFamilyIsIgnored` (rfc2918_unadvertised_family_test.go) over `ReadAndProcess`: + IPv6/unicast refresh (not advertised) reaches no onMessageReceived consumer, no NOTIFICATION, Established; - IPv4/unicast refresh delivered once, body unchanged. Test was RED before the fix (the unadvertised refresh was delivered to onMessageReceived) | positive + negative, revert `reactor/session_handlers.go::routeRefreshFamilyUnadvertised` (see batch 3 status below) | enforced | fix below |
| RFC4456-8-4 | tests | NEW `TestRFC4456OriginatorIDIsTheOriginatorsBGPIdentifier` (rfc4456_originator_id_test.go): source address 10.0.0.1 with BGP Identifier 10.11.12.13; + created ORIGINATOR_ID = 10.11.12.13, existing 9.9.9.9 kept; - not the address, not the RR id, neighbor's Identifier does not replace an existing originator | positive + negative, revert `reactor/forward_rs.go::reactorForwardRS` | enforced | helper `rrForward` now delegates to new `rrForwardFromRouterID` (forward_rr_test.go, no tagged unit edited). Old negative tag prose on TestReactorForwardRRInjects (describes 8-1) left as is |
| RFC8277-3.2.1-1 | tests | NEW `TestLabeledVPNPropagationUnchangedNextHopKeepsLabels` (rfc8277_vpn_propagation_test.go): + SAFI 128 MP_REACH with unchanged next hop keeps Next Hop, label entry, RD, prefix byte for byte | positive, revert `reactor/filter_delta_handlers.go::mpReachNextHopHandler` | enforced | row carries `{single-polarity: positive}` already |

### D-8 fix (RFC2918-4-2)

Producer: `internal/component/bgp/reactor/session_read.go` delivered every length-valid ROUTE-REFRESH to `onMessageReceived` (plugins) before `handleRouteRefresh` checked the family. New `Session.screenRouteRefresh` (length rule + `routeRefreshFamilyUnadvertised`, RFC 2918 Section 4 quote above the check) is the one gate for both call sites; the old family check in `handleRouteRefresh` was deleted (no layering). No discrimination record names the changed producers (checked `rfc/discrimination/*.json` for handleRouteRefresh, validateRouteRefreshLength, processMessage). Docs updated in the same change: `docs/architecture/behavior/fsm-established.md`, `docs/architecture/wire/messages.md`.

### Records status

Batch 1+2 (RFC4271-6.3-5/6/14, RFC8950-4-1, RFC5549-4-1, RFC5549-4-4): 12 records, every run rc=0, OBSERVED red (log `children/bgp/disc-4271-63.log`, `children/bgp/disc-2.log`). Three first attempts (6.3-6 negative, 6.3-14 +/-) failed rc=2 because a sibling test file briefly did not compile; they were re-run successfully in batch 2.
Batch 3 (RFC2918-4-2 +/-, RFC4456-8-4 +/-, RFC8277-3.2.1-1 +): 5 records, every run rc=0, OBSERVED red (`children/bgp/disc-3.log`). All 17 records for the 9 resolved verdicts are written.

## Unresolved: not reached (64)

DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7, DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-2, DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-4-1, DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-6-1, DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-2, -4-1, -4-3, -4-4, -4-7, -4-8, -4-9, RFC2385-2.0-4, RFC2385-2.0-5, RFC2918-2-1, RFC2918-3-4, RFC4271-10-1, RFC4271-5-7, RFC4271-5.1.2-2, RFC4271-5.1.2-3, RFC4271-5.1.3-3, RFC4271-5.1.4-3, RFC4271-5.1.5-1, RFC4271-6.3-1, RFC4271-6.7-1, RFC4271-6.7-4, RFC4271-6.8-2, RFC4271-8.2.2-18, RFC4271-8.2.2-2, RFC4271-8.2.2-3, RFC4271-8.2.2-4, RFC4271-8.2.2-5, RFC4271-9.1.1-2, RFC4271-9.2-6, RFC4271-9.2-9, RFC4271-Security-1, RFC4659-3.4-1, RFC4724-4-1, RFC4724-4.2-9, RFC4760-7-1, RFC5492-3-1, RFC5492-5-1, RFC6286-2.1-2, RFC7311-3.4.3-2, RFC7311-3.4.3-6, RFC7311-3.4.3-7, RFC7313-4-3, RFC7313-5-3, RFC7432-8.2.1-9, RFC7611-2.2-1, RFC7705-3.3-2, RFC7705-4.2-2, RFC7705-4.2-4, RFC7911-5-3, RFC7911-5-4, RFC7911-5-5, RFC7947-2.2-1, RFC7947-x-4, RFC9252-3.3-1, RFC9552-5.1-5, RFC9552-5.2.1.4-1, RFC9552-8.2.2-9, RFC9687-4.3-10, RFC9687-4.3-3, RFC9687-4.3-8.

Hints for the continuation: RFC4271-8.2.2-3/-4/-5 and RFC9687-4.3-3 turn on "releases all BGP resources" and the ConnectRetryCounter clause, which the parent's narrowing audit also touches (8.2.2-8/-13/-15 rows); RFC9552-5.2.1.4-1 overlaps the deferred RFC9552-5.1-2 518 encoding defect in `nlri/ls/types_descriptor.go` (another package).

## Unverified observation (not journaled: reachability not verified)

`message/update_build.go` (UpdateBuilder, NEXT_HOP and MP_REACH branches): an IPv4 unicast prefix with an IPv6 next hop and `UseExtendedNextHop == false` takes the inline-NLRI branch and gets no NEXT_HOP attribute at all. `resolveNextHop` does not gate `NextHopExplicit` ("validated by the wire builder"), so an explicit IPv6 next hop for IPv4 NLRI on a session without ExtNH may produce an UPDATE missing NEXT_HOP. Not checked whether config or the API refuses that input first.

## Gates owed (main thread or a fresh agent)

- `./le go lint run` (reactor package touched: session_handlers.go, session_read.go, 6 test files).
- `./le rfc check` for rfc4271, rfc5549, rfc8950, rfc2918, rfc4456, rfc8277.
- Scoped run done here: `go test -race -count=1 ./internal/component/bgp/reactor/` under `./le job run` green after the fix (before rfc4456/rfc8277 tests were added; those two plus the RR/2918/8950 tests ran green afterwards).

## Files changed

- internal/component/bgp/reactor/session_handlers.go
- internal/component/bgp/reactor/session_read.go
- internal/component/bgp/reactor/forward_rr_test.go (helper only)
- internal/component/bgp/reactor/rfc4271_recognized_attribute_errors_test.go (new)
- internal/component/bgp/reactor/rfc8950_nexthop_pair_test.go (new)
- internal/component/bgp/reactor/rfc2918_unadvertised_family_test.go (new)
- internal/component/bgp/reactor/rfc4456_originator_id_test.go (new)
- internal/component/bgp/reactor/rfc8277_vpn_propagation_test.go (new)
- docs/architecture/behavior/fsm-established.md
- docs/architecture/wire/messages.md
- rfc/discrimination/rfc4271.json, rfc/discrimination/rfc5549.json, rfc/discrimination/rfc8950.json, rfc/discrimination/rfc2918.json, rfc/discrimination/rfc4456.json, rfc/discrimination/rfc8277.json (written by `./le rfc discriminate-record`)
