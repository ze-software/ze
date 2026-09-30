# reactor author A handoff (continuation A of reactor-author.md, commit 55a5633c1e)

Child spec plan/pre-release/spec-rfc-verdict-fix-bgp.md, package internal/component/bgp/reactor.
Scope: rfc4271 unresolved verdicts, RFC4271-6.3-6 LOCAL_PREF, 6.3-15 re-record, RFC8950-4-1 / RFC5549-4-1 / 4-4 explicit next hop, RFC9830-2.1-3 withdrawals.
No verdict stamped. Nothing committed. No existing test file edited (so nothing to tell B). Every test is in a NEW reactor_a_*_test.go file.

## Verdicts

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|------------------------------------|-----------------|------------------|-------|
| RFC4271-6.3-6 | tests | NEW `TestRFC4271LocalPrefLengthFromInternalPeer` (reactor_a_rfc4271_local_pref_length_test.go): iBGP session over the real receive path; + LOCAL_PREF length 3 -> treat-as-withdraw payload exactly, session Established (RFC 7606 7.5); - length 4 before and after -> prefix kept, LOCAL_PREF byte-identical | SEE RECORDS STATUS (producer `message/rfc7606.go::validateLocalPrefAttr`) | enforced | closes the audit's LOCAL_PREF / iBGP gap; green under `-tags ze_core,ze_bgp` |
| RFC4271-6.3-15 | re-record | unchanged unit `TestRFC4271UpdateMalformedAttributeList` | SEE RECORDS STATUS (producer `message/rfc7606.go::ValidateUpdateRFC7606AddPath`) | unchanged | producer-changed records, known red |
| RFC8950-4-1 | defect (D-8) fixed + tests | NEW `TestRFC8950ExplicitIPv6NextHopFollowsTheNegotiatedPair` (reactor_a_rfc8950_explicit_nexthop_test.go) over `AnnounceNLRIBatch` with a live pipe session: + <1/1,IPv6> -> IPv4 unicast announce with explicit 2001:db8::1 accepted, UPDATE sent; <1/128,IPv6> -> explicit IPv6 next hop for VPN-IPv4 resolved (resolveNextHop); - no pair / other pair -> ErrNextHopIncompatible returned by the entry point, no UPDATE on the wire, VPN-IPv4 refused with no address. Test was RED before the fix | SEE RECORDS STATUS (producer `reactor/peer.go::canUseNextHopFor`) | enforced for the refusal; see defect 2 for the encoding | fix below |
| RFC5549-4-1 | same as RFC8950-4-1 | same unit | same | enforced | superseded row |
| RFC5549-4-4 | same as RFC8950-4-1 | same unit | same | enforced | superseded row |
| RFC9830-2.1-3 | tests (R4 verified: no defect on the API withdraw rail) | NEW `TestRFC9830SRPolicyWithdrawalCarriesMandatoryAttributes` (reactor_a_rfc9830_withdraw_attrs_test.go) over `buildBatchWithdrawUpdate`: + SR Policy (1/73) withdrawal with an operator block (MED) carries MP_UNREACH 1/73 + ORIGIN + AS_PATH, eBGP and iBGP (LOCAL_PREF iBGP only); - batch forced to name no attribute (R1b) still carries ORIGIN and AS_PATH | SEE RECORDS STATUS (producer `reactor/reactor_api_batch.go::planBatchAttrs`) | enforced for the withdrawal half | the existing srpolicy-package tags are unchanged; the judge must read both units |
| RFC4271-10-1, 5-7, 5.1.2-2, 5.1.2-3, 5.1.3-3, 5.1.4-3, 5.1.5-1, 6.3-1, 6.7-1, 6.7-4, 6.8-2, 8.2.2-18, 8.2.2-2, 8.2.2-3, 8.2.2-4, 8.2.2-5, 9.1.1-2, 9.2-6, 9.2-9, Security-1 | unresolved, not reached | - | - | weak | tool-call budget spent on the two defects and the build/disk failures below; the audit notes (rfc/audit/rfc4271.json) name each gap |

### D-8 fix 1: explicit IPv6 next hop for IPv4 NLRI sent without the Extended Next Hop pair

Producer: `Peer.resolveNextHop` returned `NextHopExplicit` unchecked ("validated by the wire builder"); no builder knows the negotiated pairs, so an explicit IPv6 next hop for IPv4 unicast or VPN-IPv4 went out to a peer lacking the pair.
Fix: `internal/component/bgp/reactor/peer.go` resolveNextHop explicit case refuses `fam.AFI == IPv4 && addr.Is6() && !canUseNextHopFor` with ErrNextHopIncompatible (RFC 8950 Section 4 quote above it). Invalid (zero) explicit addresses still pass to the builder guard (TestResolveNextHop_ExplicitInvalid unchanged). Other AFIs (EVPN, IPv6 with IPv4-mapped) are untouched on purpose.
Queued routes: `reactor_api_batch.go` no longer resolves an explicit next hop at queue time (no negotiated caps then); `peer_initial_sync.go` both drain loops resolve every queued route through `queuedNextHopPolicy(op)` against the draining session. The announce now reports `ErrNextHopIncompatible` instead of `ErrNoPeersAcceptedFamily` when no peer took it (lastErr set, added to the acceptedCount==0 switch).
Docs: `docs/architecture/wire/capabilities.md` section 3 paragraph. Stale comment fixed in planBatchAttrs.
Scoped run (`-tags ze_core,ze_bgp`, private GOCACHE): every `NextHop|Queue|TestAnnounce|TestCanUse|TestResolveNextHop|TestRFC8950|TestRFC9830|TestRFC4271LocalPref` test green except the defect-2 test below.

### Defect 2 (NOT fixed, STOP, needs a design choice): batch rail encodes IPv4 unicast with an IPv6 next hop as NEXT_HOP, not MP_REACH_NLRI

Failing untagged test: `TestRFC8950BatchRailIPv4UnicastIPv6NextHopUsesMPReach` (reactor_a_rfc8950_explicit_nexthop_test.go), RED now: with <1/1,IPv6> negotiated the UPDATE carries no MP_REACH_NLRI. Producer: `buildBatchAnnounceUpdate` / `planBatchAttrs` treat every `family.IPv4Unicast` batch as inline NLRI + `plan.nextHopFor(facts.nextHop)`, whatever the next hop's family (RFC 8950 Section 3 requires MP_REACH_NLRI AFI 1/SAFI 1). The IPv4Unicast branches also sit in the withdraw rail and the MP_REACH branch.
Recommendation: in buildBatchAnnounceUpdate and planBatchAttrs, route an IPv4 unicast batch whose resolved next hop is IPv6 through the MP_REACH branch (no inline NLRI, no NEXT_HOP), the same split `message.UpdateBuilder` already makes (`update_build.go`, case `UseExtendedNextHop && Is4 prefix && Is6 next hop`). Fix 1 makes the no-pair case unreachable, so only the licensed case remains.
Related unverified: `message/update_build.go` still emits neither NEXT_HOP nor MP_REACH for IPv4 unicast + IPv6 next hop with `UseExtendedNextHop == false`; after fix 1 no resolveNextHop caller reaches it (peer_static_routes sets the flag from sendCtx), not otherwise verified.

### Not done (b): forwarded route with an unchanged IPv6 next hop toward a peer lacking the pair

Forward rail `precomputeNextHop` (peer_forward_facts.go) and the `NextHopUnchanged` / `NextHopExplicit` arms in reactor_api_forward.go write the source MP_REACH next hop (or an explicit IPv6 one) with no Extended Next Hop check. Reading only, no test written (budget). Needs a design choice: skip the route to that peer, or require next-hop self. Recommendation: skip it (RFC 8950 Section 4 "MUST only advertise"), checked once per destination from `peerForwardFacts` using the destination's sendCtx.

## Records status

All discrimination runs for this continuation FAILED to build, not red/green: author B's in-flight files `reactor_b_rfc9687_test.go` and `rfc9687_test.go` do not compile (`p.client undefined`), and before that unrelated packages (vrrp, l2tp authradius) did not build under the full tag set. Log: children/bgp/A-disc.log. Second attempt (children/bgp/A-disc2.log) refused every run before any build: another session's tag in `internal/component/l2tp/reactor_sccrq_zero_tid_test.go:75` ("RFC2661-10-2negative", no ` -- ` separator) makes the corpus parse fail. The command to re-run once the package compiles is at the end of this file. Nothing was written to rfc/discrimination by this continuation (see update below if any).

## Environment incidents

- Disk full on /home/thomas/.cache (11 MB free); `./le scratch cache-clean` was run (another session ran it at the same time), then the shared go-build cache was being emptied under running builds. Scoped runs used a private `GOCACHE=<scratch>/A-gocache` (delete when done).

## Gates owed (main thread)

- `./le go lint run` (peer.go, peer_initial_sync.go, reactor_api_batch.go, 3 new test files).
- scoped `go test -race ./internal/component/bgp/reactor/` once B's files compile (expect defect-2 test red).
- `./le rfc check` for rfc4271, rfc5549, rfc8950, rfc9830 after the records land.
- The 12 discrimination records below.

## Re-run command (records)

```
R=internal/component/bgp/reactor
./le rfc discriminate-record id RFC4271-6.3-15 polarity {positive,negative} unit $R/session_update_error_rfc4271_test.go::TestRFC4271UpdateMalformedAttributeList route revert producer internal/component/bgp/message/rfc7606.go::ValidateUpdateRFC7606AddPath
./le rfc discriminate-record id RFC4271-6.3-6 polarity {positive,negative} unit $R/reactor_a_rfc4271_local_pref_length_test.go::TestRFC4271LocalPrefLengthFromInternalPeer route revert producer internal/component/bgp/message/rfc7606.go::validateLocalPrefAttr
./le rfc discriminate-record id {RFC8950-4-1,RFC5549-4-1,RFC5549-4-4} polarity {positive,negative} unit $R/reactor_a_rfc8950_explicit_nexthop_test.go::TestRFC8950ExplicitIPv6NextHopFollowsTheNegotiatedPair route revert producer $R/peer.go::canUseNextHopFor
./le rfc discriminate-record id RFC9830-2.1-3 polarity {positive,negative} unit $R/reactor_a_rfc9830_withdraw_attrs_test.go::TestRFC9830SRPolicyWithdrawalCarriesMandatoryAttributes route revert producer $R/reactor_api_batch.go::planBatchAttrs
```

## Files changed

- internal/component/bgp/reactor/peer.go
- internal/component/bgp/reactor/peer_initial_sync.go
- internal/component/bgp/reactor/reactor_api_batch.go
- internal/component/bgp/reactor/reactor_a_rfc4271_local_pref_length_test.go (new)
- internal/component/bgp/reactor/reactor_a_rfc8950_explicit_nexthop_test.go (new)
- internal/component/bgp/reactor/reactor_a_rfc9830_withdraw_attrs_test.go (new)
- docs/architecture/wire/capabilities.md
- `./le rfc approve unit` entries for reactor.TestRFC8950ExplicitIPv6NextHopFollowsTheNegotiatedPair and reactor.TestRFC9830SRPolicyWithdrawalCarriesMandatoryAttributes (D-15, own new units)

## Continuation A2 (relayed by main thread from the author's final report, 2026-09-29)
Fixed: RFC8950-4-1 defect 2 (reactor_api_batch.go inlineIPv4Unicast; buildBatchAnnounceUpdate uses MP_REACH for IPv4 unicast with IPv6 NH; planBatchAttrs drops base NEXT_HOP; RFC 8950 §3 / RFC 4760 §3 quotes). Forward rail: forward_next_hop.go nextHopValue.mpFamily + egressNextHopLacksExtendedNextHop (python-edited: lint it); reactor_api_forward.go forwardUpdateCore and forward_rs.go reactorForwardRS withhold the route from a peer lacking the pair (withdrawals still sent, warning cites RFC 8950 §4). NEW defect RFC4271-5.1.5-1 fixed: forward_local_pref.go applyFactsLocalPref adds LOCAL_PREF 100 toward an internal peer when none set (RFC 4271 §5.1.5 quote).
Tests: reactor_a_rfc8950_explicit_nexthop_test.go (defect-2 test tagged, rfc8950UpdateSections helper), NEW reactor_a2_rfc8950_forward_test.go, NEW reactor_a2_rfc4271_forward_test.go (5.1.5-1 +/-, 5.1.2-2 +/-), forward_rr_test.go (rrForward negotiates <1/1,IPv6>), forward_update_test.go fixture repair (origAttrs LOCAL_PREF 100 in four tests; fwdTestHandlersWith).
Records written (observed red): 6.3-15 +/-, 6.3-6 +/-, RFC8950-4-1/RFC5549-4-1/4-4 +/- on the explicit-NH test, RFC9830-2.1-3 +/-, RFC8950-4-1 + batch test, RFC8950-4-1 +/- forward test, RFC4456-8-4 +. Logs a2-records*.log.
OWED: records RFC4456-8-4 negative (TestRFC4456OriginatorIDIsTheOriginatorsBGPIdentifier, producer forward_rs.go::reactorForwardRS), RFC4271-5.1.5-1 +/- (TestRFC4271ForwardAddsLocalPrefTowardInternalPeer, forward_local_pref.go::applyFactsLocalPref), RFC4271-5.1.2-2 +/- (TestRFC4271ForwardASPathUnmodifiedTowardInternalPeer, reactor_api_forward.go::forwardUpdateCore); re-record any stale records whose producer is applyFactsLocalPref / forwardUpdateCore / payloadNextHop / rrForward (RFC8950-5-1, RFC5549-5-1). Docs page for forward-rail LOCAL_PREF (ai/CODE-TO-DOCS.md for forward_local_pref.go). Full package test, -race, lint, BGP functional tests (LOCAL_PREF now added toward iBGP; IPv6-NH routes withheld from peers without ExtNH — .ci expectations may change).
Unresolved rfc4271 (18): 10-1, 5-7, 5.1.2-3, 5.1.3-3, 5.1.4-3, 6.3-1, 6.7-1, 6.7-4, 6.8-2, 8.2.2-18, 8.2.2-2..-5, 9.1.1-2, 9.2-6, 9.2-9, Security-1.

## Continuation A3 (2026-09-29): gates closed, no new verdict work

| gate | result | log (children/bgp/) |
|------|--------|---------------------|
| forward_next_hop.go re-opened with Edit (comment reworded) | hooks passed | - |
| reactor pkg `-tags ze_core,ze_bgp` | ok 76.6s | a3-pkg.log |
| reactor pkg `-race` same tags (./le job run) | ok 127.8s, no race | a3-race.log |
| `./le go lint run scope ./internal/component/bgp/reactor/` | 0 issues (host + linux integration) | a3-lint.log |
| `./le test bgp plugin -a` | 768/779; 4 reds were tests asserting the old non-compliant behaviour, corrected (below) and re-run green; 7 remaining reds unrelated (below) | a3-plugin.log, a3-plugin-fix.log, a3-plugin-766.log |
| `./le test bgp reload -a` | 53/53 (18 skipped) | a3-reload.log |
| `./le test bgp encode -a` | 63/63 | a3-reload.log's sibling a3-encode.log |

.ci expectations corrected (tests wrong about what they asserted; none RFC-tagged except 766):
- test/plugin/asn4-transcode-pooled-buffer.ci, test/plugin/bgp-local-as-inbound-untouched.ci, test/plugin/prefixsid-ebgp-discard-single-walk.ci: the iBGP receiver's expected frame now carries LOCAL_PREF 100 (40050400000064) in type-code order, lengths +7; comments quote RFC 4271 §5.1.5. Received frames differed from the old expectation by exactly that insertion.
- test/plugin/srv6-service-export-control.ci (RFC9252-3.2.1-5): announces VPN-IPv4 with an IPv6 next hop to peers that never negotiated RFC 8950 (RFC 9252 §5.1 requires the RFC 8950 encoding); failed with "next-hop incompatible with family". Both peers gain `capability { nexthop ipv4/mpls-vpn { nhafi ipv6; } }` (ze-peer mirrors the capability set). No assertion changed, tag claims unchanged. `./le rfc approve unit plugin.srv6-service-export-control` recorded (D-15, P-1) in tmp/commit-rfc-approved-01a40e57.md. The tag had no discrimination record before or after.

Remaining plugin reds, unrelated: 523 path-asn-filter-export-reject, 640 redistribute-export-modify (known wait-file driver change); 2/3/4 aaa-radius-*, 79/80 authz-* fail "ZE-OBSERVER-FAIL: SSH server did not start" (journal class plan/journal/gate-verdict-depends-on-the-machine.md already holds it).

Records written this continuation (all observed red, revert route; a3-records.log, a3-records2.log):
- RFC4456-8-4 negative, TestRFC4456OriginatorIDIsTheOriginatorsBGPIdentifier, producer forward_rs.go::reactorForwardRS (was stale).
- RFC4271-5.1.5-1 +/-, TestRFC4271ForwardAddsLocalPrefTowardInternalPeer, producer forward_local_pref.go::applyFactsLocalPref.
- RFC4271-5.1.2-2 +/-, TestRFC4271ForwardASPathUnmodifiedTowardInternalPeer, producer reactor_api_forward.go::forwardUpdateSection (forwardUpdateCore delegates to it; first attempt refused by another session's transient l2tp/ppp build break, retried green).
- RFC4271-5.1.3-3 negative, rfc4271_test.go::TestRFC4271ThirdPartyNextHopDisableFailsClosed, producer peer.go::resolveNextHop (listed stale by `./le rfc discriminate stem rfc4271`).
After these, `./le rfc discriminate stem` lists no stale record for rfc4271, rfc8950, rfc5549, rfc4456.

Docs: docs/architecture/bgp/egress-attribute-rules.md (LOCAL_PREF toward internal peers paragraph); docs/features/bgp-protocol.md (two rows: LOCAL_PREF on an internal session; IPv4 NLRI behind an IPv6 next hop needs Extended Next Hop).

Owed to the main thread / judge: `./le rfc check` for rfc4271, rfc4456, rfc5549, rfc8950, rfc9830, rfc9252; the full-tree gates at commit. Files changed in A3: internal/component/bgp/reactor/forward_next_hop.go, the four .ci above, the two docs pages, rfc/discrimination/rfc4271.json, rfc/discrimination/rfc4456.json.
