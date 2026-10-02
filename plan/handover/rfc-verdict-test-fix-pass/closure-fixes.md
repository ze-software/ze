# Closure fixes, 2026-10-02 (author; routing, ike-eap, bfd)

Judge findings from routing/ac-c2-sweep.md, ike-eap/ac-c2-sweep.md and the BFD owner ruling
(RULINGS.md, OWNER RULING 2026-09-30). D-15 approvals recorded with `./le rfc approve` in
`tmp/commit-rfc-approved-6912077e.md` for the five edited units. Nothing stamped or committed.

| # | id | change | records | expected verdict |
|---|----|--------|---------|------------------|
| 1 | RFC5305-4-1 | `isis/spf/spf_test.go::TestISISMetricWidth` positive claim narrowed to what the body asserts: node B (edge 10, below MAX_PATH_METRIC) remains reachable in the SPF result; "installable" dropped | positive re-recorded on `isis/spf/spf.go::Compute`, revert route, OBSERVED red (panic in Compute under TestISISMetricWidth); the negative record on `route.go::BuildRoutes` still verifies (unit-sha unchanged) | enforced, unchanged (rests on `TestRFC5305PrefixAboveMaxPathMetricNotConsidered`, both polarities) |
| 2 | RFC1195-5.2-2 | tag removed from `isis/packet/tlv_ipv4_test.go::TestISISTLVIPv4InterfaceAddr` (decode-only, cannot reach `Originate`); the test stays as the decoder's coverage | none owed. Its old record (`tlv_ipv4.go::DecodeIPv4InterfaceAddrTLV`) is now an ORPHAN in `rfc/discrimination/rfc1195.json`; not deleted (judge/owner) | enforced, unchanged (positives `TestISISOriginateOnAdjacencyUp` on `Originate`, `TestRFC1195SameInterfaceAddressesBothLevels` on `levelState`; row is `{single-polarity: positive}`) |
| 3 | RFC3209-4.3.4-1 | `rsvpte/engine_test.go::TestEngineTransitForwarding` positive claim: the stale `nextHopFromERO` citation replaced by `resolveExplicitPath` in routing.go stripping the leading local subobjects | positive re-recorded on `rsvpte/routing.go::resolveExplicitPath`, revert route, OBSERVED red (panic via `handlePacket` in TestEngineTransitForwarding) | enforced, unchanged |
| 4 | RFC3948-4-1 | tag removed from `ike/transport/keepalive_test.go::TestKeepaliveDefaultInterval` (asserts only a constant, can never carry a record); comment now states it pins the constant and names `TestRFC3948UnsetKeepaliveIntervalFallsBackToM` for the behavior | none owed; the unit never had a record. Coverage held: 4 positives and 3 negatives recorded remain | enforced, unchanged |
| 5 | RFC7296-3.1-3 | negative tag removed from `ike/engine/rfc7296_header_test.go::TestSPIZeroRules` (never calls `dispatchInbound`) | none owed; the unit never had a negative record. Negatives held by `TestRFC7296ZeroInitiatorSPIIsDroppedOnReceipt` (dispatchInbound) and `...OnTheNATTSocket` (dispatchNATTInbound); the unit's 3.1-3 positive and 3.1-4 records still verify | enforced, unchanged |
| 6 | RFC5880-6.1-3, RFC5880-6.8.6-15 | NOT DONE: blocked by the coverage ratchet, see below | none | unchanged |

## Item 6, blocked

The neighbouring rows the two HEAD negatives prove already carry the same tag on the same unit:
- `bfd/session/rfc5880_test.go::TestRFC5880PassiveRoleSilentUntilReception`: the 6.1-3 negative
  (Passive request yields RolePassive, no TX deadline) is what the unit's `RFC5880-6.1-1 negative`
  tag already claims.
- `bfd/engine/rfc5880_test.go::TestRFC5880NoPeriodicTransmitWhileAdminDown`: the 6.8.6-15
  negative (AdminDown session silent after the window) is what the unit's `RFC5880-6.8.16-3
  negative` tag already claims.

So "move" means deleting the 6.1-3 and 6.8.6-15 negative tags. `checkCoverageRatchet`
(`internal/le/rfc/check_ratchets.go`) then reports both ids "no longer proven -- the negative
test(s) that covered it at HEAD are gone ... An annotation does not substitute": it compares tag
polarities to HEAD only and reads no annotation and no `RFC-approved` row, so
`{single-polarity: positive}` does not quiet it. Landing the ruling as written reds `./le rfc
check` for every session. Which way: (a) land it with the two ratchet lines named as
owner-ruled in the commit body, or (b) give the ratchet an escape for an owner-ruled move first.

## Package tests

`go test -count=1` under `./le job run`: isis/spf, isis/packet, rsvpte, ike/transport ok.
ike/engine FAIL in only TestMobikeParallelAuthKeepsPromotedPolicyEndpoints and
TestMobikeParallelAuthRetiresChildRekey ("listen udp4 127.0.0.2:0: bind: can't assign requested
address"). That is the host: macOS has no 127.0.0.2 loopback alias. It is not caused by this
change, whose edit in that package removes comment lines only. gofmt is clean.

## Files changed

- internal/plugins/isis/spf/spf_test.go
- internal/plugins/isis/packet/tlv_ipv4_test.go
- internal/plugins/rsvpte/engine_test.go
- internal/component/ike/transport/keepalive_test.go
- internal/component/ike/engine/rfc7296_header_test.go
- rfc/discrimination/rfc5305.json (re-record, tool-written)
- rfc/discrimination/rfc3209.json (re-record, tool-written)
- plan/handover/rfc-verdict-test-fix-pass/closure-fixes.md

## Judge, 2026-10-02 (independent; items 1-5 only, item 6 untouched pending the owner)

| # | id | claim vs body | polarity still held | verdict |
|---|----|---------------|---------------------|---------|
| 1 | RFC5305-4-1 | match: the body asserts only `res2.Nodes[b]` present, and the narrowed claim says exactly that | + `TestRFC5305PrefixAboveMaxPathMetricNotConsidered` (BuildRoutes) and `TestISISMetricWidth` (re-recorded on `spf.go::Compute`); - both units on BuildRoutes | enforced (rejudged) |
| 2 | RFC1195-5.2-2 | tag removed; the unit stays as decoder coverage | + `TestISISOriginateOnAdjacencyUp` (Originate), `TestRFC1195SameInterfaceAddressesBothLevels` (levelState); the row is single-polarity positive | enforced (rejudged). ORPHAN left in `rfc/discrimination/rfc1195.json`: the positive record on `tlv_ipv4.go::DecodeIPv4InterfaceAddrTLV` for `packet/tlv_ipv4_test.go::TestISISTLVIPv4InterfaceAddr` (`./le rfc discriminate id` lists it as stale) |
| 3 | RFC3209-4.3.4-1 | match: `resolveExplicitPath` in routing.go skips the leading local subobjects and returns `ERO: rest` when the next hop is in the second abstract node; the body asserts the one-hop ERO at the egress | + re-recorded on resolveExplicitPath; - `TestRFC3209TransitNotAdjacentToSecondSubobject`, `TestEngineTransitNoUsableERONextHop` | enforced (rejudged) |
| 4 | RFC3948-4-1 | tag removed; the new comment is accurate and the test it names exists in rfc3948_keepalive_window_test.go | 4 positive and 3 negative recorded units remain | enforced (rejudged) |
| 5 | RFC7296-3.1-3 | negative tag removed; the unit's remaining 3.1-3 positive and 3.1-4 tags match its body | - `TestRFC7296ZeroInitiatorSPIIsDroppedOnReceipt` (dispatchInbound), `...OnTheNATTSocket` (dispatchNATTInbound) | enforced (rejudged) |

Also rejudged because these edits staled them, with their assertions unchanged: RFC7296-3.1-4
(TestSPIZeroRules lost only the 3.1-3 lines) and RFC5308-5-2 (TestISISMetricWidth changed only in
the RFC5305-4-1 prose line). `./le rfc check` before stamping showed no coverage-ratchet line for
any of the five ids; the only violations in these stems were the seven stale verdicts, now
stamped with `mode rejudge`. The remaining violations belong to other packages (RFC2328-13-6
stale; producer-changed records in rfc4271, rfc4456, rfc8277, rfc9190). The audit files for
rfc3948, rfc5305 and rfc7296 also carry the BGP judge's reseal of tests in these same files
(SHIFTED `tests` shas of TestNATKeepalive, TestISISTLVIPv4RoundTrip,
TestISISSPFMaxLinkMetricExcluded, TestBuiltMessagesClearXAndVBits, TestXBitsIgnoredOnReceipt,
TestResponseBitMatchesDirection): line shifts these edits caused, carried here.
