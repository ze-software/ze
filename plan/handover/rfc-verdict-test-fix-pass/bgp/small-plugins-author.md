# small-plugins author handoff (spec-rfc-verdict-fix-bgp)

Packages: role, server (rfc9234); filter_remove_private_as (rfc6996); filter_community, filter_modify,
cmd/announce (rfc7999); filter_prefix (rfc8955); test/plugin (rfc6793/7999/9234 tags).
Blocked, not touched: RFC9234-3.1-1, RFC7999-3.1-2 (spec-bgp-update-propagation-rfc-defects).
Author: subagent, 2026-09-29, budget stop at ~85 calls. No verdict stamped, nothing committed.
No existing RFC-tagged unit was edited (the two edited units carried no tag), so no `./le rfc approve` was needed.

Environment during the run: other sessions had the tree mid-edit. `internal/component/bgp/reactor`
(peer_initial_sync.go: queuedNextHopPolicy undefined), `internal/component/l2tp/pppoe` and
`internal/plugins/rsvpte` failed to build at different moments, and the go-build cache lost entries
under a concurrent clean. Every record marked PENDING below was refused for that reason, not by a green run.

| id | resolution | what now proves each clause (+ / -) | records written | expected verdict | notes |
|----|-----------|--------------------------------------|-----------------|------------------|-------|
| RFC6996-4-1 | tests | + TestRFC6996RemovesPrivateUseASNsFromBothPathAttributes (plugin handleFilterUpdate: AS_PATH with 64512/65534/4200000000/4294967294 rewritten to [64496 64497]; AS4_PATH [4294967294] draws the directive); + TestRFC6996StripRemovesPrivateUseASNsFromAS4Path (reactor ExtractRemovePrivateASOps: AS4_PATH [64496 4200000000 64512 4294967294 64497] -> [64496 64497]); - TestRFC6996KeepsASNsOutsideThePrivateUseRanges (64511/65535/4199999999/4294967295 in AS_PATH and AS4_PATH: accept); - TestRFC6996StripKeepsAS4PathASNsOutsideThePrivateUseRanges (reactor: no op) | plugin pos+neg (revert isPrivateASN). Reactor pos+neg PENDING: `./le rfc discriminate-record id RFC6996-4-1 polarity <p> unit internal/component/bgp/reactor/rfc6996_remove_private_test.go::<Func> route revert producer internal/component/bgp/reactor/filter_delta.go::isRFC6996PrivateASN` | enforced once reactor records land | reactor tests passed once before the reactor build broke |
| RFC7999-3.2-1 | tests | + TestRFC7999ReceivedBlackholeRouteGainsThePropagationCommunity (config leaf parsed -> applyIngressFilter; both tokens; other community kept); - TestRFC7999RouteWithoutBlackholeGainsNoPropagationCommunity (no COMMUNITY, 65535:665, 64496:1 unchanged) | pos, neg (revert blackholePropagationGuard) | enforced | old tagged units kept |
| RFC7999-3.2-2 | tests | + TestRFC7999PropagationCommunityFollowsTheOperatorLeaf; - TestRFC7999PropagationCommunityIsNeverForced (the other community absent; leaf none or unset -> unchanged) | pos, neg (revert blackholeGuardCommunity) | enforced | |
| RFC7999-3.3-2 | tests | + TestRFC7999BlackholeHonoredForTheEqualAuthorizedPrefix (rib: /24 authorized, /24 announced -> Blackhole on both rails); - TestRFC7999BlackholeNotHonoredOutsideTheAuthorizedPrefix (agreed session; outside prefix and SHORTER /16 -> ordinary route); condition 2 held by existing rib units | pos, neg (revert coveredByAuthorized) | enforced for the rib path; RISK below | MAIN THREAD: the filter_modify units tagged 3.3-2 honor BLACKHOLE (next-hop rewrite to discard) with no coverage check, so on that route condition 1 rests on an operator prefix filter in the chain. A judge can read this as a condition-1 defect of the filter_modify route or as the tags over-claiming. Decide: move/retag those two units (approval D-15) or journal the defect |
| RFC8955-3-1 | tests | + TestRFC8955CommunityPolicyAcceptsAPermittedFlowSpecRoute (filter_community_match: ipv4/flow and ipv4/flow-vpn with 65001:100 accepted); - TestRFC8955CommunityPolicyRejectsADeniedFlowSpecRoute (denied 65001:666 first match, and no listed community: reject); prefix half held by filter_prefix unit | pos, neg (revert evaluateCommunities) | enforced | new file in filter_community_match (package had no RFC8955 tag, no other holder) |
| RFC9234-4.1-1 | tests | + test/plugin/dynamic-peer-gets-group-role-capability.ci tagged: the whole OPEN on the wire carries 09 01 01; - existing unit negative | PENDING: `... id RFC9234-4.1-1 polarity positive unit test/plugin/dynamic-peer-gets-group-role-capability.ci route revert producer internal/component/bgp/plugins/role/config.go::extractRoleCapabilities` (refused: rsvpte build) | enforced once recorded | |
| RFC9234-4.1-2 | tests | + same .ci tagged (whole OPEN octet for octet: exactly one Role capability) | PENDING: same command, id RFC9234-4.1-2 | enforced (single-polarity kept) once recorded | |
| RFC9234-4.2-2 | tests (partial) | + NEW test/plugin/role-mismatch-notification.ci (Provider vs Provider -> seq 1 NOTIFICATION 03020B, no EoR; passed); - test/plugin/peer-open-role-complement.ci tagged (Provider/Customer establishes, reject= 03020B) | neg PENDING (build). pos UNRESOLVED: revert of validateOpenRolePair does NOT discriminate (see finding) | weak until pos record exists | needs the mutant route: a gomu report killing the `!isValidRolePair` branch, then `discriminate-record ... route mutant` |
| RFC9234-4.2-3 | tests (partial) | + NEW test/plugin/role-multiple-roles-notification.ci (Customer+Peer -> 03020B, no EoR; passed); - NEW test/plugin/role-multiple-same-roles.ci (Customer twice -> EoR, reject= 03020B; not yet run green: cache loss during build) | revert on pos refused as GREEN (finding); neg PENDING | weak until both land | pos needs mutant on the `v != first` loop |
| RFC9234-4.2-5 | tests | + test/plugin/role-strict-enforcement.ci tagged (wire 03020B in strict mode); - TestValidateOpenRolePair_NoPeerRole_NoStrict tagged (non-strict accepts missing role) | neg (revert validateOpenRolePair); pos .ci PENDING (revert likely non-discriminating, same finding: strict and crash both give 2/11; use mutant on `if cfg.strict`) | enforced once pos lands | |
| RFC9234-5-12 | tests | + TestRFC9234OTCOfAnyLengthButFourIsTreatedAsWithdraw (lengths 0, 5, 8 -> NLRI to withdrawn, attrs cleared, kept); - TestRFC9234OTCOfLengthFourIsNotTreatedAsWithdraw | pos, neg (revert findOTC) | enforced | |
| RFC9234-5-4 | unresolved | not started | none | weak | owes: a tagged Peer-destination stamp assertion, and a scope negative whose payload carries NLRI (toward a Provider) so the destination gate is what refuses |
| RFC9234-5-11 | unresolved | not started | none | weak | owes: ingress procedures and the egress stamp surviving an operator setting (export policy / role option), per the audit note |
| RFC6793-4.1-6 | unresolved | not started | none | weak | owes: AS4_AGGREGATOR absent between NEW speakers, with an input that carries AS4_AGGREGATOR/AGGREGATOR (the current .ci input has none). First tagged unit is in internal/component/bgp/message; check the reactor stream does not hold it |

## Finding (not a defect)
When the role plugin panics, the OPEN validation fails closed and ze still sends NOTIFICATION 2/11
(consistent with TestBroadcastValidateOpenRefusesAnUnansweredPerPeerPolicy). So a `route revert` of any
role producer cannot turn a "ze refuses with 2/11" .ci red: the positives of 4.2-2, 4.2-3 and 4.2-5 need
the mutant route. The negatives (session establishes) do discriminate under revert.

## Gates owed (main thread)
`./le rfc check`, `./le go lint run`, `./le test bgp plugin` for the four new/edited role .ci, the reactor
package test, independent re-judge (`mode rejudge`) of RFC6996-4-1, RFC7999-3.2-1, 3.2-2, 3.3-2,
RFC8955-3-1, RFC9234-5-12 now, and of 4.1-1, 4.1-2, 4.2-2, 4.2-3, 4.2-5 after their records.

## Files changed
- internal/component/bgp/plugins/filter_remove_private_as/private_as_rfc6996_test.go (new)
- internal/component/bgp/reactor/rfc6996_remove_private_test.go (new)
- internal/component/bgp/plugins/filter_community/blackhole_rfc7999_test.go (new)
- internal/component/bgp/plugins/rib/rib_blackhole_rfc7999_cover_test.go (new)
- internal/component/bgp/plugins/filter_community_match/flowspec_rfc8955_test.go (new)
- internal/component/bgp/plugins/role/otc_malformed_rfc9234_test.go (new)
- internal/component/bgp/plugins/role/validate_test.go (tag added to TestValidateOpenRolePair_NoPeerRole_NoStrict)
- test/plugin/role-mismatch-notification.ci (new)
- test/plugin/role-multiple-roles-notification.ci (new)
- test/plugin/role-multiple-same-roles.ci (new)
- test/plugin/dynamic-peer-gets-group-role-capability.ci (tags added)
- test/plugin/peer-open-role-complement.ci (tag added)
- test/plugin/role-strict-enforcement.ci (tag added)
- rfc/discrimination/rfc6996.json, rfc/discrimination/rfc7999.json, rfc/discrimination/rfc8955.json,
  rfc/discrimination/rfc9234.json (records added by the recorder)

## Update after the table (final calls)
- RFC6996-4-1 reactor POSITIVE recorded (revert isRFC6996PrivateASN). Reactor NEGATIVE still PENDING: the run hit `reactor [build failed]` again (another session's edits), not a green.
- RFC9234-4.1-1 and RFC9234-4.1-2 positive RECORDED on dynamic-peer-gets-group-role-capability.ci. A .ci record needs `citation "<directive line>"`, e.g. `citation "expect=bgp:conn=1:seq=1:hex=..."`.
- RFC9234-4.2-2 negative on peer-open-role-complement.ci: refused as "already failing before any break". That .ci passed at 20s earlier this session; only a comment was added since. Re-run it once when the tree builds; if it fails, read why before recording.
- Still PENDING records: RFC6996-4-1 reactor negative; RFC9234-4.2-2 neg (complement .ci), RFC9234-4.2-3 neg (role-multiple-same-roles.ci, never run green yet). Mutant route owed: RFC9234-4.2-2 pos, 4.2-3 pos, 4.2-5 pos.

## Continuation 2 (2026-09-29, second author)
- RFC6996-4-1 reactor NEGATIVE recorded (revert isRFC6996PrivateASN, observed red). RFC6996-4-1 now has all four records.
- All 19 role .ci ran green once (`./le test bgp plugin --pattern role -t 40s`), including role-multiple-same-roles.ci and peer-open-role-complement.ci; the earlier 'already failing' was environment.
- RFC9234-4.2-2 NEGATIVE recorded on peer-open-role-complement.ci; RFC9234-4.2-3 NEGATIVE recorded on role-multiple-same-roles.ci (revert validateOpenRolePair; citation = the EoR expect line).
- RFC7999-3.3-2 ruling: filter_modify TestBlackholeCommunityRewritesNextHopWhenAgreed / ...LeavesNextHopAloneWithoutAgreement retagged to RFC7999-4-1 (negative / positive), approvals D-15 recorded, both records written (revert (matchCond).matches). 3.3-2 keeps rib units both polarities + interop. 4-1 (was weak: two positives, no negative) now has a negative: the directive-present counter-case. Judge to confirm that reading of 'negative' for a SHOULD NOT row.

### Continuation 2 table (supersedes the rows above for these ids)

| id | resolution | what now proves each clause (+ / -) | records written | expected verdict | notes |
|----|-----------|--------------------------------------|-----------------|------------------|-------|
| RFC6996-4-1 | tests | as table above | reactor neg by revert isRFC6996PrivateASN (all 4 records now exist) | enforced | |
| RFC9234-4.2-2 | tests | + role-mismatch-notification.ci; - peer-open-role-complement.ci | pos: MUTANT validate.go:102:5#1 (`!` removed from `!isValidRolePair`), citation the 03020B expect line; neg: revert validateOpenRolePair, citation the EoR expect line | enforced | gomu report: scratch/gomu-role-report.json (real run, `gomu run ./internal/component/bgp/plugins/role`) |
| RFC9234-4.2-3 | tests | + role-multiple-roles-notification.ci (Customer+Peer); - role-multiple-same-roles.ci (Customer twice) | pos: MUTANT validate.go:68:5#2 (`len(peerRoles) > 1` to `< 1`); neg: revert | enforced | same-roles .ci ran green |
| RFC9234-4.2-5 | tests | + role-strict-enforcement.ci; - TestValidateOpenRolePair_NoPeerRole_NoStrict | pos: MUTANT validate.go:86:3#2 (`cfg.strict` to `false`) | enforced | clean run flaked once ("No messages received", 10s budget); journal row added |
| RFC9234-5-4 | tests | + NEW TestRFC9234OTCStampedToEveryDownstreamDestination (Customer, Peer, RS-Client each get one op 35 = 65000); - NEW TestRFC9234OTCNotStampedOutsideTheDownstreamScope (NLRI-bearing route to Provider and RS: no op; OTC already present to Customer: no op); - TestOTCEgressNoStampProvider REPAIRED (payload now carries NLRI, approval D-15) | pos + both negs by revert OTCEgressFilter. Also checked by overlay: widening the destination gate (`true ||`) turns the new negative red (provider and rs subtests) | enforced | a mutant on otc.go:746 is refused (the line carries `==` three times) |
| RFC9234-5-11 | tests (single-polarity kept) | + NEW TestRFC9234OTCProceduresSurviveOperatorSettings: strict on and an export set naming every role; ingress still stamps OTC 64500 from a Provider, still refuses an OTC route from a Customer; egress still withholds an OTC route from a Provider and still stamps 65000 toward a Customer | pos by revert checkOTCIngress | enforced if the judge accepts the marker (R1: no refusal path; the marker was at HEAD) | the row's marker cites stale lines (otc.go:164, :384; now checkOTCIngress 260, OTCEgressFilter 608). Not edited: it is row text under the requirement hash |
| RFC6793-4.1-6 | tests | + NEW test/plugin/rfc6793-no-as4aggregator-from-new-speaker.ci (AGGREGATOR 4200000123 + AS4_AGGREGATOR 4200000000 in; conn=2 exact hex with AGGREGATOR unchanged, no type 18; ran green); + NEW TestOriginatedAggregatorOmitsAS4AggregatorTowardNewSpeaker (message builders, non-mappable aggregating AS toward NEW: no AS4_AGGREGATOR, AGGREGATOR carries the real AS); - rfc6793-narrow-to-old-speaker.ci (HEAD) | .ci pos by revert wireu CollapseAS4Family, citation the conn=2 expect line; unit pos by revert attribute AS4AggregatorFor | enforced | the .ci reuses fixture `plugin/rfc6793-no-as4path-from-new-speaker` (generic shutdown-after-up observer) rather than editing the shared plugin_fixture_03.go |
| RFC7999-3.3-2 / 4-1 | tests (retag) | 3.3-2: rib units + interop, both polarities. 4-1: + TestBlackholeCommunityLeavesNextHopAloneWithoutAgreement; - TestBlackholeCommunityRewritesNextHopWhenAgreed (directive present: discard) | 4-1 pos + neg by revert matches | 3.3-2 enforced; 4-1 enforced if the judge accepts the counter-case as the negative | |

Nothing left unresolved in this package's list.

### Gates owed (main thread)
`./le rfc check`; `./le go lint run`; `./le test bgp plugin` for role-*.ci and rfc6793-*.ci; `go test -race` for reactor, rib, filter_community, filter_community_match, message (role and filter_modify ran green here); independent re-judge of every id in the table above.

### Files changed in continuation 2
- internal/component/bgp/plugins/filter_modify/match_test.go (two tags moved 3.3-2 to 4-1)
- internal/component/bgp/plugins/role/otc_procedures_rfc9234_test.go (new)
- internal/component/bgp/plugins/role/otc_test.go (TestOTCEgressNoStampProvider payload carries NLRI)
- internal/component/bgp/message/rfc6793_originate_as4agg_new_test.go (new)
- test/plugin/rfc6793-no-as4aggregator-from-new-speaker.ci (new)
- rfc/discrimination/rfc6996.json, rfc7999.json, rfc9234.json, rfc6793.json (recorder)
- tmp/commit-rfc-approved-01a40e57.md (three D-15 approvals)
- plan/journal/gate-verdict-depends-on-the-machine.md (one row)
