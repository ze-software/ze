# Routing AC-C2 record sweep, 2026-10-02

Scope: every verdict in the routing child's owned stems (rfc1195, rfc2205, rfc2966, rfc3032,
rfc3209, rfc3786, rfc3787, rfc4090, rfc5036, rfc5303, rfc5305, rfc5308, rfc5310, rfc905,
rfc5301, rfc3031) that is `enforced` now and was not `enforced` at `aa71dd19e3~1` (the tree
before the child's first commit, `aa71dd19e3`, "rfc: routing rsvpte verdict fixes").
97 ids: rfc1195 18, rfc2205 14, rfc2966 3, rfc3032 1, rfc3209 8, rfc3786 1, rfc3787 4,
rfc4090 13, rfc5036 6, rfc5303 6, rfc5305 5, rfc5308 5, rfc5310 7, rfc905 6. rfc5301 has no
newly enforced id; rfc3031 has no audit file. For each stem `./le rfc discriminate stem <stem> '|' json`
was read for `unproven` and `stale`, filtered to the 97 ids.

- 79 unproven tag lines, which are 76 units once a unit's several tags for one id and
  polarity are counted once (one record seals all of them: `claimSHA` hashes every tag of the
  cover in line order).
- 1 stale record in scope: RFC4090-4.4-7 negative
  `rsvpte/frr_rfc4090_test.go::TestRFC4090NodeBitClearWithoutNodeBypass` on `frr.go::selectBypass`
  (producer changed). 77 units owed in all.
- Orphans: none. The only stale record over all sixteen stems is the one above, and its unit
  still carries its tag.
- No overlap with the BGP or OSPF judges' stems or files (the isis BGP-LS export test carries
  no routing-stem tag in scope).

Scripts (copied from the ike sweep and adapted): `tmp/session/2026-10-02-e08980d7-b739-4095-ab66-53539559487d/scratch/c2rs/scope.sh`
(scope), `plan.sh` (plan), `sweep.sh` (revert route, per-stem flock in
`tmp/session/2026-09-28-869df689-cc8f-4d78-9161-1d7c87434c8e/scratch/children/ledger-<stem>.lock`,
first OBSERVED red stops). Results: `c2rs/routing/sweep.tsv`, one log per attempt under
`c2rs/routing/logs/`.

Units tagged in BOTH polarities for one id (DUAL) were read before their records were
accepted: each record's producer is reached by the half its polarity names, not only by the
other half (the ike judge's rejection). Where it was not, the record was replaced, see below.

## Results

Counts: 77 units owed, 76 OBSERVED reds written (one record per unit, all `route revert`,
all written by `./le rfc discriminate-record`), 0 green findings, 1 orphan, 0 not run.
Net records added over HEAD: rfc1195 +10, rfc2205 +7, rfc2966 +4, rfc3209 +7, rfc3786 +2,
rfc4090 +13, rfc5303 +8, rfc5305 +5, rfc5308 +14, rfc5310 +3, rfc905 +3 = 76.
After the sweep `scope.sh routing-after` (same base, same stems) finds 0 unproven units in
scope and 1 stale record, the orphan below.

Linux-guest units (run with `kernel ~/.cache/ze/runtime-kernel/7.2-runtime-arm64-runtime-8e843adc-cd0e7a40/vmlinuz`,
both files carry the `_linux_test.go` constraint): RFC1195-4.4-1 both polarities on
`fib/kernel/onlink_integration_linux_test.go`, RFC2205-x-1 positive on
`rsvpte/transport_integration_linux_test.go`. All three observed red in the guest.

A sweep-script defect cost one round, recorded so the TSV reads right: in round 1 an empty
flag column collapsed under `IFS=tab`, so a single-polarity unit lost its first candidate
producer (and with one candidate, ran none: the `NO-RED` rows with no attempt before them).
Rounds 2 and 3 re-ran every such unit with its full list; nothing was recorded on a
producer that was not tried in order.

DUAL review (both polarities of one id in one unit; the record is honest only when the
half its polarity names reaches the producer):
- Shared call, both halves read its result, accepted: RFC2966-2-1 (`LeakPrefixes` once,
  `leakInto` per direction), RFC5303-3.2-1/-2/-10 (each half calls `p2pThreeWayState`),
  RFC5303-3.2-5 (each half builds a hello through `threeWayTLV`), RFC5308-2-2 (one
  `BuildRoutesV6`), RFC5308-3-2 and 5308-4-1 dual-stack (one `Originate` per case),
  RFC5308-5-1 `TestISISLeakUpDownBit` (every case ranks through `preferenceRank`),
  RFC4090-4.4-2/-3 (every half calls `rroProtectionFlags`), RFC2205-3.10-1
  `TestDecodeUnknownObjectClass` (every case decodes through `DecodeMessage`, the 0bbbbbbb,
  10bbbbbb and 11bbbbbb cases classify through `classifyUnknownClass`), RFC3786-5-1 and
  RFC5305-4.1-1 engine tests (one engine, both halves read its LSDB/RIB), RFC1195-4.4-1
  (both halves drive `processEvent`).
- Replaced: RFC5305-4-1 positive `spf/spf_test.go::TestISISMetricWidth`. Round 1 recorded it
  on `route.go::BuildRoutes`, which the test calls only AFTER the positive's assertion
  (`res2.Nodes[b]` reachable) and whose result only the negative reads, so that red came from
  the negative half. Re-recorded on `spf/spf.go::Compute` (the positive's `res2`), which
  replaced the old record. The negative stays on `BuildRoutes`.
- Plan producers that only another package or only the engine reaches were refused R-10 and
  replaced by the producer the unit calls: RFC2205-3.10-1 wire unit (`classifyUnknownClass`,
  not `engine.go::rejectUnknownObject`), RFC4090-4.4-2 (`rroProtectionFlags`, not
  `tryLocalRepair`), RFC5303-3.2-10 (`p2pThreeWayState`, not `adjacency/fsm.go::threeWayTableAction`).

## Orphan (owner decision; not deleted, JSON not edited)

| id | polarity | unit | producer | result |
|----|----------|------|----------|--------|
| RFC4090-4.4-7 | negative | internal/plugins/rsvpte/frr_rfc4090_test.go::TestRFC4090NodeBitClearWithoutNodeBypass | internal/plugins/rsvpte/frr.go::selectBypass | orphan, and stale (producer changed). The unit no longer carries a 4.4-7 negative tag; the tool names the unit that does: `rsvpte/plr_flags_rfc4090_test.go::TestRFC4090LinkProtectionLeavesNodeBitClear`, which carries a current record (not unproven). Both re-record attempts refused ("no tag for RFC4090-4.4-7 negative resolves to the unit") |

Out of scope, noted: 61 ids in these stems hold unproven units but are outside AC-C2 (they
were already `enforced` at `aa71dd19e3~1`, or are not `enforced` now), list in
`c2rs/routing/allunproven` minus `newids`. rfc5301 holds 7 of them (3-4 to 3-10).

## Records written (paths relative to `internal/plugins/`)

| id | polarity | unit | producer |
|----|----------|------|----------|
| RFC1195-3.2-2 | positive | `isis/rfc1195_spf_test.go::TestRFC1195SpecificLeakCollapsesDuplicatePrefix` | `isis/spf/leak.go::leakInto` |
| RFC1195-3.9-1 | negative | `isis/packet/auth_verify_test.go::TestISISAuthConstantTimeCompare` | `isis/packet/auth_verify.go::verifyKey` |
| RFC1195-3.9-1 | positive | `isis/packet/auth_verify_test.go::TestISISAuthSignVerifyHMACMD5` | `isis/packet/auth_verify.go::VerifyPDU` |
| RFC1195-4.4-1 | negative | `fib/kernel/onlink_integration_linux_test.go::TestFIBOnLinkAdjacencyAndUnsupportedPrefix` | `fib/kernel/fibkernel.go::processEvent` |
| RFC1195-4.4-1 | positive | `fib/kernel/onlink_integration_linux_test.go::TestFIBOnLinkAdjacencyAndUnsupportedPrefix` | `fib/kernel/fibkernel.go::processEvent` |
| RFC1195-5.2-1 | positive | `isis/lsdb/origination_test.go::TestISISOriginateOnAdjacencyUp` | `isis/lsdb/origination.go::Originate` |
| RFC1195-5.2-2 | positive | `isis/lsdb/origination_test.go::TestISISOriginateOnAdjacencyUp` | `isis/lsdb/origination.go::Originate` |
| RFC1195-5.2-2 | positive | `isis/packet/tlv_ipv4_test.go::TestISISTLVIPv4InterfaceAddr` | `isis/packet/tlv_ipv4.go::DecodeIPv4InterfaceAddrTLV` |
| RFC1195-7-2 | positive | `isis/rfc1195_spf_test.go::TestRFC1195PeriodicSPFRecoversMissedUpdate` | `isis/lsdb_wiring.go::refreshOwnLSPs` |
| RFC1195-7-4 | positive | `isis/rfc1195_spf_test.go::TestRFC1195InternalMetricOutranksExternal` | `isis/spf/route.go::better` |
| RFC2205-3.1-1 | negative | `rsvpte/wire_test.go::TestRSVPDecodeHeaderBadVersion` | `rsvpte/wire.go::DecodeHeader` |
| RFC2205-3.1-1 | positive | `rsvpte/wire_test.go::TestRSVPHeaderRoundTrip` | `rsvpte/wire.go::encodeHeader` |
| RFC2205-3.1-2 | positive | `rsvpte/wire_test.go::TestRSVPReservedByteZeroOnSend` | `rsvpte/wire.go::encodeHeader` |
| RFC2205-3.10-1 | negative | `rsvpte/frr_test.go::TestEnginePathWithIgnorableObjectAccepted` | `rsvpte/wire.go::classifyUnknownClass` |
| RFC2205-3.10-1 | negative | `rsvpte/wire_test.go::TestDecodeUnknownObjectClass` | `rsvpte/wire.go::classifyUnknownClass` |
| RFC2205-3.10-1 | positive | `rsvpte/wire_test.go::TestDecodeUnknownObjectClass` | `rsvpte/wire.go::classifyUnknownClass` |
| RFC2205-x-1 | positive | `rsvpte/transport_integration_linux_test.go::TestIntegrationRSVPPathCarriage` | `rsvpte/transport_linux.go::SendPath` |
| RFC2966-2-1 | negative | `isis/spf/leak_test.go::TestISISLeakOriginationL1L2` | `isis/spf/leak.go::leakInto` |
| RFC2966-2-1 | positive | `isis/spf/leak_test.go::TestISISLeakOriginationL1L2` | `isis/spf/leak.go::leakInto` |
| RFC2966-3.2-1 | negative | `isis/rfc1195_spf_test.go::TestRFC2966ExternalL1DoesNotOverrideDownInternal` | `isis/spf/route.go::better` |
| RFC2966-3.2-1 | positive | `isis/rfc1195_spf_test.go::TestRFC2966SixPreferenceClasses` | `isis/spf/route.go::preferenceRank` |
| RFC3209-4.2-1 | positive | `rsvpte/build_test.go::TestBuildPathRoundTrip` | `rsvpte/wire.go::encodeLabelRequest` |
| RFC3209-4.3.4-1 | negative | `rsvpte/engine_test.go::TestEngineTransitNoUsableERONextHop` | `rsvpte/routing.go::resolveExplicitPath` |
| RFC3209-4.3.4-1 | positive | `rsvpte/engine_test.go::TestEngineTransitForwarding` | `rsvpte/routing.go::resolveExplicitPath` |
| RFC3209-4.6.1-1 | positive | `rsvpte/wire_test.go::TestRSVPSessionObjectEncoding` | `rsvpte/wire.go::encodeObjectHeader` |
| RFC3209-6-1 | negative | `rsvpte/admission_se_test.go::TestSEAdmissionDistinctSessionsDoNotShare` | `rsvpte/admission.go::reserve` |
| RFC3209-6-1 | positive | `rsvpte/reroute_test.go::TestEngineMakeBeforeBreak` | `rsvpte/reroute.go::reroute` |
| RFC3209-6-2 | positive | `rsvpte/build_test.go::TestBuildResvRoundTrip` | `rsvpte/wire.go::encodeStyle` |
| RFC3786-5-1 | negative | `isis/rfc1195_spf_test.go::TestRFC1195StandardFragmentsRequireFragmentZero` | `isis/spf_wiring.go::Records` |
| RFC3786-5-1 | positive | `isis/rfc1195_spf_test.go::TestRFC1195StandardFragmentsRequireFragmentZero` | `isis/spf_wiring.go::Records` |
| RFC4090-4.1-1 | negative | `rsvpte/frr_test.go::TestFastRerouteShortBody` | `rsvpte/frr.go::decodeFastReroute` |
| RFC4090-4.1-1 | positive | `rsvpte/frr_test.go::TestEncodeDecodeFastReroute` | `rsvpte/frr.go::encodeFastReroute` |
| RFC4090-4.1-2 | negative | `rsvpte/frr_test.go::TestFastRerouteOneToOneMethodFlag` | `rsvpte/frr.go::fastReroute` |
| RFC4090-4.1-2 | positive | `rsvpte/frr_test.go::TestBuildPathIncludesFastReroute` | `rsvpte/frr.go::fastReroute` |
| RFC4090-4.3-2 | negative | `rsvpte/frr_test.go::TestSessionAttributeEmptyName` | `rsvpte/frr.go::decodeSessionAttr` |
| RFC4090-4.3-2 | positive | `rsvpte/frr_test.go::TestBuildPathIncludesFastReroute` | `rsvpte/frr.go::sessionAttr` |
| RFC4090-4.3-2 | positive | `rsvpte/frr_test.go::TestSessionAttributeProtectionFlags` | `rsvpte/frr.go::sessionAttr` |
| RFC4090-4.4-2 | negative | `rsvpte/frr_test.go::TestRROProtectionFlagsReflectState` | `rsvpte/frr.go::rroProtectionFlags` |
| RFC4090-4.4-2 | positive | `rsvpte/frr_test.go::TestRROProtectionFlagsReflectState` | `rsvpte/frr.go::rroProtectionFlags` |
| RFC4090-4.4-3 | negative | `rsvpte/frr_test.go::TestRROProtectionFlagsReflectState` | `rsvpte/frr.go::rroProtectionFlags` |
| RFC4090-4.4-3 | positive | `rsvpte/frr_test.go::TestRROProtectionFlagsReflectState` | `rsvpte/frr.go::rroProtectionFlags` |
| RFC4090-6.5-1 | negative | `rsvpte/frr_test.go::TestLocalRepairFallsBackToTeardown` | `rsvpte/frr.go::tryLocalRepair` |
| RFC4090-6.5-1 | positive | `rsvpte/frr_test.go::TestLocalRepairSendsNotify` | `rsvpte/frr.go::tryLocalRepair` |
| RFC5303-3.2-1 | negative | `isis/circuit/rfc5303_threeway_test.go::TestISISThreeWayReportsCurrentState` | `isis/circuit/runtime.go::p2pThreeWayState` |
| RFC5303-3.2-1 | positive | `isis/circuit/rfc5303_threeway_test.go::TestISISThreeWayReportsCurrentState` | `isis/circuit/runtime.go::p2pThreeWayState` |
| RFC5303-3.2-10 | negative | `isis/circuit/rfc5303_threeway_test.go::TestISISThreeWayNewAdjacencyStateDown` | `isis/circuit/runtime.go::p2pThreeWayState` |
| RFC5303-3.2-10 | positive | `isis/circuit/rfc5303_threeway_test.go::TestISISThreeWayNewAdjacencyStateDown` | `isis/circuit/runtime.go::p2pThreeWayState` |
| RFC5303-3.2-2 | negative | `isis/circuit/rfc5303_threeway_test.go::TestISISThreeWayDownWhenNoAdjacency` | `isis/circuit/runtime.go::p2pThreeWayState` |
| RFC5303-3.2-2 | positive | `isis/circuit/rfc5303_threeway_test.go::TestISISThreeWayDownWhenNoAdjacency` | `isis/circuit/runtime.go::p2pThreeWayState` |
| RFC5303-3.2-5 | negative | `isis/circuit/rfc5303_threeway_test.go::TestISISThreeWayReportsNeighborSystemID` | `isis/circuit/hello.go::threeWayTLV` |
| RFC5303-3.2-5 | positive | `isis/circuit/rfc5303_threeway_test.go::TestISISThreeWayReportsNeighborSystemID` | `isis/circuit/hello.go::threeWayTLV` |
| RFC5305-4-1 | negative | `isis/spf/spf_test.go::TestISISMetricWidth` | `isis/spf/route.go::BuildRoutes` |
| RFC5305-4-1 | positive | `isis/spf/spf_test.go::TestISISMetricWidth` | `isis/spf/spf.go::Compute` |
| RFC5305-4.1-1 | negative | `isis/lsdb_wiring_test.go::TestISISEngineLeakOrigination` | `isis/lsdb_wiring.go::leakedToPrefixInfos` |
| RFC5305-4.1-1 | negative | `isis/redistribute/consumer_test.go::TestISISRedistConsumerConnected` | `isis/redistribute/consumer.go::InjectRoute` |
| RFC5305-4.1-1 | positive | `isis/lsdb_wiring_test.go::TestISISEngineLeakOrigination` | `isis/lsdb_wiring.go::leakedToPrefixInfos` |
| RFC5308-2-2 | negative | `isis/spf/ipv6_test.go::TestISISIPv6MetricAboveMaxIgnored` | `isis/spf/ipv6.go::BuildRoutesV6` |
| RFC5308-2-2 | positive | `isis/spf/ipv6_test.go::TestISISIPv6MetricAboveMaxIgnored` | `isis/spf/ipv6.go::BuildRoutesV6` |
| RFC5308-3-1 | negative | `isis/circuit/hello_ipv6_test.go::TestISISIIHTLV232OmittedNoLinkLocal` | `isis/circuit/hello.go::ipv6InterfaceAddrTLV` |
| RFC5308-3-1 | negative | `isis/circuit/hello_ipv6_test.go::TestISISIIHTLV232RejectsNonLinkLocal` | `isis/circuit/hello.go::ipv6InterfaceAddrTLV` |
| RFC5308-3-1 | positive | `isis/circuit/hello_ipv6_test.go::TestISISIIHTLV232LinkLocal` | `isis/circuit/hello.go::ipv6InterfaceAddrTLV` |
| RFC5308-3-2 | negative | `isis/lsdb/origination_ipv6_test.go::TestISISOriginateTLV232Scope` | `isis/lsdb/origination.go::Originate` |
| RFC5308-3-2 | positive | `isis/lsdb/origination_ipv6_test.go::TestISISOriginateTLV232Scope` | `isis/lsdb/origination.go::Originate` |
| RFC5308-4-1 | negative | `isis/circuit/hello_ipv6_test.go::TestISISIIHNoTLV232WhenIPv4Only` | `isis/circuit/hello.go::protocolsSupportedTLV` |
| RFC5308-4-1 | negative | `isis/lsdb/origination_ipv6_test.go::TestISISProtocolsSupportedDualStack` | `isis/lsdb/origination.go::Originate` |
| RFC5308-4-1 | positive | `isis/circuit/hello_ipv6_test.go::TestISISIIHTLV232LinkLocal` | `isis/circuit/hello.go::protocolsSupportedTLV` |
| RFC5308-4-1 | positive | `isis/lsdb/origination_ipv6_test.go::TestISISProtocolsSupportedDualStack` | `isis/lsdb/origination.go::Originate` |
| RFC5308-5-1 | negative | `isis/spf/route_test.go::TestISISLeakUpDownBit` | `isis/spf/route.go::preferenceRank` |
| RFC5308-5-1 | positive | `isis/spf/ipv6_test.go::TestISISIPv6LevelArbitration` | `isis/spf/route.go::preferenceRank` |
| RFC5308-5-1 | positive | `isis/spf/route_test.go::TestISISLeakUpDownBit` | `isis/spf/route.go::preferenceRank` |
| RFC5310-3.2-5 | positive | `isis/circuit/runtime_test.go::TestISISHelloSignedOverPaddedPDU` | `isis/circuit/hello.go::padHello` |
| RFC5310-3.4-2 | positive | `isis/circuit/runtime_test.go::TestISISHelloSignedOverPaddedPDU` | `isis/circuit/hello.go::padHello` |
| RFC5310-4-1 | positive | `isis/packet/auth_verify_test.go::TestISISAuthLSPChecksumAfterSign` | `isis/packet/auth_sign.go::lspAuthLayout` |
| RFC905-x-1 | positive | `isis/packet/checksum_test.go::TestISISChecksumFixedVector` | `isis/packet/checksum.go::VerifyChecksum` |
| RFC905-x-1 | positive | `isis/packet/checksum_test.go::TestISISChecksumModulus` | `isis/packet/checksum.go::VerifyChecksum` |
| RFC905-x-2 | positive | `isis/packet/checksum_test.go::TestISISChecksumVectors` | `isis/packet/checksum.go::VerifyChecksum` |

Files changed by this sweep: `rfc/discrimination/rfc1195.json`, `rfc2205.json`,
`rfc2966.json`, `rfc3209.json`, `rfc3786.json`, `rfc4090.json`, `rfc5303.json`,
`rfc5305.json`, `rfc5308.json`, `rfc5310.json`, `rfc905.json` (records only, all written by
`./le rfc discriminate-record`), and this file. No test, code, audit or row was edited,
nothing stamped or committed. Gates owed by the main thread: `./le rfc check` over these stems.
`bin/le` printed "older than committed sources" on every run; it was not refreshed here.

## Judge (independent, 2026-10-02)

Scope re-derived with `scope.sh judge-routing aa71dd19e3~1` over the sixteen stems: 97 ids
newly `enforced` (rfc3031 has no audit file), 0 unproven units in scope, 1 stale record (the
orphan). Net records over HEAD match the sweep's +76 per stem, and the diff removes no record
except the ones the judge replaced below.

Re-recorded (each observed red, `route revert`, replacing the old record):
- RFC905-x-1 positive `TestISISChecksumFixedVector` and `TestISISChecksumModulus`, RFC905-x-2
  positive `TestISISChecksumVectors`: from `checksum.go::VerifyChecksum` to
  `checksum.go::Checksum`. Each claim names the generation arithmetic (`Checksum`,
  checksum.go:67-76 and 74-75), and each body calls `Checksum` first, so the VerifyChecksum
  panic proved reach of the verifier only.
- RFC3209-4.6.1-1 positive `wire_test.go::TestRSVPSessionObjectEncoding`: from
  `wire.go::encodeObjectHeader` to `wire.go::encodeSessionIPv4`, which writes the two reserved
  octets (`buf[8]`, `buf[9]`). The id's other unit,
  `literals_rfc_test.go::TestRFC3209SessionReservedZeroOverDirtyBuffer`, carried the same
  reach-only record from HEAD and was moved to the same producer.
- RFC3209-4.2-1 positive `build_test.go::TestBuildPathRoundTrip`: from
  `wire.go::encodeLabelRequest` to `build.go::buildPath`, the inclusion the claim names.

Producer spot-checks, accepted:
- RFC5305-4-1 positive on `spf.go::Compute`: the positive half reads `res2.Nodes[b]`, which
  `Compute` produces; the old `BuildRoutes` red came from the negative half.
- RFC1195-3.9-1 negative on `auth_verify.go::verifyKey` (the `hmac.Equal` comparison under
  `VerifyPDU`); RFC1195-7-2 on `lsdb_wiring.go::refreshOwnLSPs` (`ageOnce` -> refresh ->
  `originate`, the periodic full pass); RFC1350/RSVP-TE/IS-IS DUAL acceptances re-read.

Findings (no verdict change, no re-stamp; each verdict rests on another recorded unit):
- RFC5305-4-1 positive `TestISISMetricWidth`: the claim says a below-ceiling "node/prefix
  remains reachable and installable", but the body asserts only node B reachable. The verdict
  rests on `rfc5305_max_metric_order_test.go::TestRFC5305PrefixAboveMaxPathMetricNotConsidered`
  (both polarities on `BuildRoutes`). OWED (author): narrow the claim to node reachability,
  or assert the below-ceiling prefix is installed.
- RFC1195-5.2-2 positive `packet/tlv_ipv4_test.go::TestISISTLVIPv4InterfaceAddr` proves the
  TLV 132 decoder, not inclusion in an LSP, and cannot reach `Originate` (R-10). The verdict
  rests on `lsdb/origination_test.go::TestISISOriginateOnAdjacencyUp` (asserts TLV 132 in
  fragment 0, recorded on `Originate`). OWED (author): drop the tag (D-15) or narrow its claim.
- RFC3209-4.3.4-1 positive `TestEngineTransitForwarding`: the claim names
  `nextHopFromERO, engine.go:407-416`, which no longer exists. The recorded
  `routing.go::resolveExplicitPath` is the current producer (strips local leading subobjects,
  routing.go:48-50). OWED (author): update the claim prose.
- Orphan RFC4090-4.4-7: untouched, for the owner.

`./le rfc check` (2026-10-02, after the re-records): 37 violations, none in this child's
stems (rfc2328 8, rfc9552 18, rfc9086 2 stale/shifted verdicts; producer-changed records in
rfc4271 4, rfc4456 2, rfc8277 1, rfc9190 2). The orphan is not reported.

AC-C2 for this child: met for all 76 swept units plus the judge's 6 re-records. No verdict
changed and no audit file was stamped.
