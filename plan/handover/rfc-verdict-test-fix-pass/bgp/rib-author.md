# rib author (BGP child), packages rib, rib/pool, rib/storage -- first pass, 2026-09-30

Listing: 33 rows whose first package is rib (22), rib/pool (6), rib/storage (5).
Blocked by (spec Blocked-by table), skipped: RFC9252-3.2.1-3, RFC9252-5-1, RFC9252-7-1 (spec-bgp-prefix-sid-rfc-defects),
RFC9494-4.2-6, RFC9494-4.3-1 (spec-bgp-graceful-restart-rfc-defects). 28 left to work; all re-checked still weak/wrong in rfc/audit on 2026-09-30.

Records: all route revert, OBSERVED red, per-stem lock (log scratch/children/bgp/rib-rec1.log for rfc4271).

| id | resolution | what now proves each clause (+/-) | records | expected verdict | notes |
|----|-----------|-----------------------------------|---------|------------------|-------|
| RFC4271-4.3-7 | tests | + the three HEAD positives (rfc4271_rib_mixed_update_test.go); - TestRFC4271MixedUpdateStillAppliesItsOtherWithdrawals (duplicated prefix kept, the other withdrawn prefix still removed; red on "discard WITHDRAWN whole") | - peerrib.go::Remove | enforced | new unit, receive path |
| RFC4271-9-1 | tests | + TestRFC4271ReceivedWithdrawalRemovesTheRouteFromAdjRIBIn; - TestRFC4271ReceivedWithdrawalRemovesOnlyTheNamedRoutes (both through handleReceivedStructured) | +/- peerrib.go::Remove | enforced | storage-level HEAD tags kept |
| RFC4271-9-2 | tests | + TestRFC4271ReceivedRouteWithIdenticalNLRIReplacesTheOlder; - TestRFC4271ReplacementWithdrawsTheOlderRouteEvenWhenItWasBetter (R1-b: newer worse route must be the one in service; keep-both/keep-better goes red) | +/- peerrib.go::InsertEntry | enforced | |
| RFC4271-9-3 | tests | + TestRFC4271AdjRIBInUpdateRunsTheDecisionProcess (no checkBestPathChange call in the test); - TestRFC4271AdjRIBInUpdateThatDisplacesTheBestReselects (missed run leaves stale best) | +/- rib_bestchange.go::checkBestPathChange | enforced | |
| RFC4271-9.1.2-2 | tests | + TestRFC4271SelectedRouteReplacesTheLocRIBRoute; - TestRFC4271LocRIBHoldsOnlyTheReplacingRoute (Loc-RIB group holds exactly one BGP path) | +/- rib_remote.go::insertLocRIB | enforced | |
| RFC4271-9.2-4 | tests (wrong -> fixed) | + TestRFC4271OverlappingReceivedRoutesAreBothInstalled; - TestRFC4271OverlappingRouteArrivalDisplacesNeither (both orders, no withdraw). Old storage tags (NLRI keying / 9-2) REMOVED (approved D-15) | +/- checkBestPathChange | enforced, or weak on "configured acceptance policy" | the rib applies no import policy (filters run in reactor ingress); tag prose says default accept-all only. Judge decides whether that clause needs a filter-plugin unit |
| RFC4271-9.2-5 | tests (wrong -> fixed) | + TestRFC4271OverlappingReceivedRoutesAreBothInstalled (same NEXT_HOP, both in Loc-RIB); - TestRFC4271OverlappingRouteArrivalDisplacesNeither. Old storage tags removed (D-15) | +/- insertLocRIB | enforced | |
| RFC7911-2-1 | tests + row (wrong -> fixed) | + reactor TestForwardPathIDStableAcrossUpdates (same path, same ze id across UPDATEs); - reactor TestForwardPathIDsDifferForCollidingSources (R1-b: sources forced to collide on id 1, still distinct). storage TestPeerRIB_AddPath tag removed (receive-side keying, neighbour). Row's {single-polarity} marker removed from rfc/short/rfc7911.md | +/- forward_path_id.go::generatePath | enforced | approvals D-15 for the three units |
| RFC4271-9.1.2.2-3 | DEFECT fixed (D-8) + tests | + TestRFC4271IBGPLocallyOriginatedRoutesCompareMED (was RED at HEAD: two iBGP routes with empty AS_PATH skipped MED, IGP cost decided); - TestRFC4271IBGPMEDIsNotSkippedWhenLaterStepsDisagree (R1-b: every later step favors the higher-MED iBGP route). Fix: bestpath.go neighborAS/sameNeighborAS (RFC 9.1.2.2 (c) quote above), used by comparePair, comparePairWithReason, multipathEqual. Doc: docs/architecture/route-selection.md row 13 | +/- bestpath.go::sameNeighborAS | enforced | NOT fixed: clause (b) iBGP aggregate whose AS_PATH BEGINS WITH AS_SET should be local AS; firstASInPath returns the set's first ASN (Candidate carries no segment type). HEAD tags on TestBestPath_MED_SameNeighborAS left in place. comparePair/multipathEqual changed: records naming them as producer go producer-changed (see rfc check) |
| RFC4271-9.1.1-1 | tests | + TestRFC4271DegreeOfPreferenceIgnoresOtherRoutes REWRITTEN (approved D-15): vacuous withNoise assertion removed; now SelectBest winner is invariant over all 120 permutations of {a,b}+3 unrelated routes (one ties b on LOCAL_PREF) and beats each member in ComparePair; - HEAD TestRFC4271DegreeOfPreferenceFollowsOwnAttributes unchanged | + bestpath.go::SelectBest (re-recorded) | enforced or weak | a mutant (position-dependent tie break) would be stronger than the revert; judge's call |
| RFC4271-9.1.2.2-1 | tests | + NEW TestRFC4271AdjacentCriteriaApplyInTheOrderSpecified: six adjacent pairs a)..f) (LOCAL_PREF/AS_PATH len, AS_PATH len/ORIGIN, ORIGIN/MED, MED/EBGP, EBGP/IGP cost, IGP cost/BGP Identifier), earlier decides in both argument orders; - HEAD negatives unchanged | + bestpath.go::comparePair | enforced | closes "reordering among a) to e) passes every unit" |
| RFC8955-6-2 | unresolved (main thread) | -- | -- | wrong | Row quotes RFC 8955 §6 leftmost-AS rule, which RFC 9117 §4.2 replaces; the tags prove the 9117 rule. rfc9117 is not enrolled (no rfc/full, no rfc/short). Route needs a decision: enrol rfc9117 and move the tags to its §4.2 row, then retire 8955-6-2 as updated by RFC 9117 (D-10) |
| RFC4271-9.1.2.1-1 | unresolved (recommendation) | -- | -- | wrong | Producer is not in rib: sysrib nhresolver.go Resolve does recursive resolution and sysrib publishes DirectNH to the FIB (TestRecursiveCrossFamilyGroupReachesFIB); an unresolvable gateway is programmed as-is (TestEveryDoorProgramsAnUnresolvableGatewayTheSameWay) and the kernel refuses a non-link gateway. Route: move the two rib tags off (they prove Loc-RIB NH = NEXT_HOP attribute), add + in sysrib (BGP route over recursive NH reaches FIB with the direct NH) and - as a netns fib-kernel test (non-connected gateway never installed). Cross-package, not started |
| RFC9252-7-2 | blocked | -- | -- | weak | L2 half of D5 in plan/immediate/spec-bgp-prefix-sid-rfc-defects.md (ExtractSRv6SIDFull falls through to the 2nd instance). Add 7-2 to that spec's AC-5/AC-6 (P-3) and to the BGP child's Blocked-by |
| RFC8669-3.1-2, RFC8669-3.2-4 | blocked (recommend) | -- | -- | weak | Genuine negative lives in bgp/message rfc7606.go validatePrefixSIDAttr (Label-Index/SRGB length refusal on any family), which D1 of spec-bgp-prefix-sid-rfc-defects rewrites; whether a malformed Label-Index on a non-LU route is "ignored" (§3.1) or "malformed" (§6) is a design call inside D1. Recommend P-3: add both to that spec |
| RFC8669-6-3 | unresolved | -- | -- | weak | Discard-from-forwarded-bytes clause is the same shape as the RFC8669-6-2 gap; candidate R3 split. Not started |

## Ids NOT started (continuation picks these up, smallest first)
RFC4271-6.3-2 (error SHOULD be logged + criterion b) EBGP one-hop subnet), RFC4271-5.1.5-5 (import local-preference policy unit), RFC2918-4-3 (genuine negative), RFC4724-4.2-5 (HEAD negative is RFC 9494 behaviour; needs genuine negative or R1 OWNER-GATE), RFC8277-2.5-2 (label side-data keyed per path), RFC8277-3.1-1, RFC9494-4.2-4, RFC9494-4.2-5 (LLGR entry dispatch half lives in the gr plugin), RFC4271-9.2-7 (advertise-to-peers half is reactor/adj-rib-out). Plus the three unresolved rows above (9.1.2.1-1, 8955-6-2, 8669-6-3).
Nothing is half-edited: every id above either has its full edit + records or is untouched.

## Verification (this pass)
- go test -race -count=1 rib, rib/storage, rib/pool under ./le job run: all ok (scratch/job-rib-race-4d2e52dd.log).
- golangci-lint run ./internal/component/bgp/plugins/rib/... under ./le job run: exit 0 (scratch/job-rib-lint-a30dae5e.log). Reactor lint not run (forward_path_id_test.go change is comments only).
- Failing-first for 9.1.2.2-3 observed red before the fix (scratch/job-rib-t2-*.log).
- ./le rfc check (scratch/children/bgp/rib-rfccheck.log): for rfc4271/rfc7911 only STALE/SHIFTED audit verdicts on the rows touched here (expected: judges re-judge 4.3-7, 9-1, 9-2, 9-3, 9.1.1-1, 9.1.2-2, 9.1.2.2-1, 9.1.2.2-3, 9.2-4, 9.2-5, RFC7911-2-1, and RFC7911-2-2 whose units' doc comments gained the 2-1 tags; 5.1.5-5 and 9.1.2.1-1 SHIFTED only -> reseal). RFC4271-5.1.5-3 and 9.2-10 STALE come from other sessions' reactor files. No other finding names these stems.
- Gates owed by the main thread: ./le verify worktree (not run here per budget rule).

Files changed so far:
- internal/component/bgp/plugins/rib/rfc4271_receive_decision_test.go (new)
- internal/component/bgp/plugins/rib/storage/rfc4271_test.go (9.2-4/9.2-5 tags removed, comment)
- internal/component/bgp/plugins/rib/storage/peerrib_test.go (RFC7911-2-1 tag removed, comment)
- internal/component/bgp/reactor/forward_path_id_test.go (RFC7911-2-1 +/- tags added)
- rfc/short/rfc7911.md (2-1 marker removed)
- internal/component/bgp/plugins/rib/rfc4271_ibgp_med_test.go (new)
- internal/component/bgp/plugins/rib/rfc4271_criteria_order_test.go (new)
- internal/component/bgp/plugins/rib/rfc4271_test.go (9.1.1-1 positive rewritten, peerOrder helper)
- internal/component/bgp/plugins/rib/bestpath.go (neighborAS, sameNeighborAS; three MED call sites)
- docs/architecture/route-selection.md (row 13, neighbor AS for the MED step)
- rfc/discrimination/rfc4271.json, rfc/discrimination/rfc7911.json (records)
- rfc/approvals (via ./le rfc approve unit)
