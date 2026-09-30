# BGP child, R34 + attribute leftovers -- author handoff (2026-09-30)

Scope: RFC8092-3-2 (R34, D-8), RFC8950-3-2, RFC6396-4.3.4-1. Nothing stamped, nothing committed.
Records taken under `flock scratch/children/ledger-<stem>.lock`.

| id | resolution | what now proves each clause (+/-) | records | expected verdict | notes |
|----|-----------|------------------------------------|---------|------------------|-------|
| RFC8092-3-2 | DEFECT fixed (D-8, R34) | + reactor TestRFC8092RedundantLargeCommunityRemovedBeforePropagation (was the untagged red, now tagged): 2 copies (1-octet length) and 23 values/3 repeats (ext length, >16 so the hash-filter path): no error, ActionNone ("silently"), published attr and attr on a peer's wire hold each value once, first-occurrence order. - reactor TestRFC8092DistinctLargeCommunitiesKeptWhole: 3 and 22 distinct values (same GA, differ in local data): published body octet-equal to received (common path zero-copy), every value forwarded. Old attribute-package tags (TestLargeCommunitiesDeduplication/Parse) kept (ratchet) | +/- revert route, producer rfc8092_large_community.go::removeRedundantLargeCommunities, both OBSERVED | enforced | fix: publishBase calls removeRedundantLargeCommunities before discardNonVPNAcceptOwn; scan first (pairwise <=16, seeded maphash 64k-bit stack filter above, exact confirm), rebuild only on a hit (map + copy, via message.RebuildUpdateBody -- called, not edited). RIB parseLargeCommunityWire now receives deduped bytes. RFC8092-6-3 unit/record/audit from predecessor untouched and kept. Approvals recorded (D-15) for the two new units |
| RFC8950-3-2 | tests (+ producer extracted) | + attribute TestRFC8950VPNIPv6NextHopCarriesZeroRD (new file rfc8950_vpn_nexthop_test.go): AFI 1 SAFI 128, IPv6 NH; Length 24 = zero RD + global; Length 48 = zero RD + global + zero RD + link-local; buffer pre-filled 0xFF so an unwritten RD reads non-zero (R1(b) pressure); parse returns the same addresses. Row keeps {single-polarity: positive} (no negative at HEAD; sender-side encoding, no refusal path; encoder takes no RD input); marker reason's stale line numbers replaced by function names. HEAD tags kept (TestMPReachNLRI_RoundTrip_VPN prose claims "RD always written as zero" but asserts only NH_Len=12 -- judge may want it reworded; not touched) | + revert, producer mpnlri.go::writeNextHops, OBSERVED | enforced | revert of WriteTo was refused (two WriteTo in mpnlri.go), so the next-hop field write moved out of MPReachNLRI.WriteTo into writeNextHops (behaviour-neutral; clear() for the RD; RFC 4364 4.3.4 + RFC 8950 3 quotes above it) |
| RFC6396-4.3.4-1 | tests | + rib TestRFC6396TwoByteSessionRouteDumpsFourByteASPath (new rfc6396_mrt_aspath_test.go): 2-byte-session UPDATE (AS_SEQ 65001 65002, 2-octet) -> wireu.CollapseAS4Family (the session read path's ingest step, srcASN4=false) -> RIB handleReceivedStructured (ctx stays ASN4=false) -> dumpRIBForMRT -> mrt.WriteRIBHeader/WriteRIBEntries -> DecodeRIBRecord: entry AS_PATH is 4-octet. - rib TestRFC6396FourByteSessionRouteDumpsASPathUnchanged: 4-byte session, AS_SEQ 65001 200000 -> written entry octet-equal (no double widening, no narrowing). HEAD tags kept (attribute 2 units, mrt 1 unit); mrt TestDumpV2RIBEntryASPathIs4Byte tag prose reworded (named a removed rib/storage canonicalizeASPath producer), approval D-15, re-recorded | + revert wireu/aspath_collapse.go::CollapseAS4Family; - revert rib/rib_mrt.go::reconstructWireAttrs; mrt unit + revert plugins/mrt/dump.go::writeTableDumpV2; all OBSERVED | enforced | the RIB record is composed with the same internal/mrt writers the MRT plugin's writeTableDumpV2 calls (the plugin itself is a separate component; its verbatim copy is the mrt unit) |

## Results
- go test -race: attribute, plugins/mrt, wireu ok (job r34-race-attr). reactor + rib: see job r34-race-big (appended below when done).
- ./le rfc check (log children/bgp/r34-rfccheck.log), my stems: STALE verdict lines for RFC8092-3-2, RFC8950-3-2, RFC6396-4.3.4-1 (expected, judge re-stamps); SHIFTED for RFC6396-4.2-2 and 4.3.1-3 (dump_test.go lines moved by the prose edit; judge runs ./le rfc reseal). No record refusal, no other line.
- Owed by main thread: ./le doc index write (new file rfc8092_large_community.go carries a Design: header and docs <!-- source --> anchors); golangci-lint result see job r34-lint.

## Files changed (RFC6396-4.3.4-1)
- internal/component/bgp/plugins/rib/rfc6396_mrt_aspath_test.go (new)
- internal/plugins/mrt/dump_test.go (TestDumpV2RIBEntryASPathIs4Byte tag prose only)
- rfc/discrimination/rfc6396.json (three records)

## Files changed (RFC8950-3-2)
- internal/core/bgp/attribute/mpnlri.go (writeNextHops extracted from MPReachNLRI.WriteTo)
- internal/core/bgp/attribute/rfc8950_vpn_nexthop_test.go (new)
- rfc/short/rfc8950.md (RFC8950-3-2 single-polarity reason text only; quote untouched)
- rfc/discrimination/rfc8950.json (one record)

## Files changed (RFC8092-3-2)
- internal/component/bgp/reactor/rfc8092_large_community.go (new: removeRedundantLargeCommunities, largeCommunityRedundant, largeCommunityAttrsDeduped)
- internal/component/bgp/reactor/session_validation.go (publishBase: one call + comment)
- internal/component/bgp/reactor/rfc8092_duplicate_not_malformed_test.go (red test tagged + table; new negative unit; helpers; header)
- rfc/discrimination/rfc8092.json (two RFC8092-3-2 records)
- docs/features/bgp-protocol.md (ingest rules table: new row)
- docs/architecture/wire/attributes.md (LARGE_COMMUNITY section paragraph; index-freezes paragraph)
- predecessor's rfc/audit/rfc8092.json + RFC8092-6-3 record: kept, land with this work
