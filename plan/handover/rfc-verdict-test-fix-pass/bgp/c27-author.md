# BGP continuation c27 (2026-10-02), test author

No commit, no stamp, no reseal. Logs: tmp/session/2026-10-02-e08980d7-b739-4095-ab66-53539559487d/scratch/c27-*.
Records under flock tmp/session/2026-09-28-869df689-cc8f-4d78-9161-1d7c87434c8e/scratch/children/ledger-<stem>.lock (script c27-rec.sh).

| id | resolution | + / - units | records | expected verdict | notes |
|----|-----------|-------------|---------|------------------|-------|
| RFC9494-5-1 (R59, D-8) | DEFECT fixed, failing test first (c26 red, c26-51-red.log), + tests | - gr/rfc9494_helper_default_red_test.go::TestRFC9494HelperProceduresOffWithoutLocalConfig (now tagged; real sent and received OPEN events on the JSON and structured paths; Ze's OPEN with no code 71, and with code 71 for IPv6 only; peer's code 71 for IPv4; TCP failure: no ffff0006, no ffff0007). + NEW ::TestRFC9494HelperProceduresOnPerConfiguredFamily (Ze declares IPv4, peer IPv4+IPv6: delete-with-community ffff0007 and attach-community ffff0006 for ipv4/unicast, no command naming ipv6/unicast, both paths). HEAD units rfc9494_test.go::TestRFC9494_LLGRNotEnabledByDefault (+) and ::TestRFC9494_LLGREnabledByExplicitConfig (-) kept, first records | 4 new, all observed red: - and + on gr_llgr_exchange.go::exchangedLLGRLocked (c27-rec-a3, a4; a1/a2 on gr.go::handleStructuredState were replaced by these); + and - on gr_llgr.go::parseLLGRCapValue for the two HEAD units (a5, a6) | weak -> enforced for the helper side; judge to weigh "per AFI/SAFI" on the config side (see notes) | Producer: a received code 71 was stored and honored whatever Ze advertised. Fix: Ze records the families of the LLGR Capability in the OPEN it SENT (new subscription "open direction sent", both event paths, gr_llgr_exchange.go), and exchangedLLGRLocked hands the session-down and session-up paths only the received families Ze declared; RFC 9494 Section 5 quote above the filter, section reference at the 4 call sites in gr.go. Chosen over a config index keyed by peer address because the sent OPEN is what this session exchanged (a reload does not change it) and it needs no name/address/group mapping (dynamic peers included). Egress filter (gr_egress.go) still reads the peer's own advertisement, which is what RFC 9494 Section 4.6 conditions on ("stale routes MAY be advertised to neighbors that have not advertised the Long-Lived Graceful Restart Capability under the following conditions"). OPEN QUESTION for the main thread: Ze's config cannot enable LLGR for one family of a multi-family peer: extractLLGRCapabilities declares code 71 for every session family with the one long-lived-stale-time. The procedures now run only for configured families, but the configuration unit is the peer, not the AFI/SAFI. A per-family knob is a YANG design choice, not made here. Recommendation: a `long-lived-stale-time` per `graceful-restart family` entry, or a sibling LLGR family list. Docs: docs/guide/graceful-restart.md LLGR Configuration (false "per-family configurable" and "only active when both peers negotiate it" corrected) |
| RFC9494-4.2-1, 4.2-2, 4.2-4, 4.2-5 (c26 units) | setup edit (D-15) | rfc9494_llgr_entry_test.go: the three units now call declareLocalLLGR(gp, IPv4 unicast) so Ze's side of the exchange exists; assertions unchanged | 7 re-recorded, all observed red (c27-rec-r1..r7) | unchanged (enforced, 4.2-5 weak per c26 judge) | approvals: gr.TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod, gr.TestRFC9494BothTimesZeroRetainsNothing, gr.TestRFC9494RestartTimeThenLongLivedStaleTime |
| RFC4724-4.2-4 (records only) | re-record | rfc4724_consecutive_test.go units unchanged; producer gr.go::handleStateEvent changed by the R59 fix | 2 re-recorded, observed red (c27-rec-s1, s2) | unchanged | |

| RFC4724-4-4 (OWNER RULING 7) | DESIGN NEEDED, failing test written (untagged), STOPPED | NEW gr/rfc4724_rib_families_red_test.go::TestRFC4724GRCapabilityListsOnlyFamiliesARIBHolds: session ipv4/unicast + ipv4/flow (no plugin linked into the test binary registers ipv4/flow; precondition asserted): payload must be "007800010100"; with `graceful-restart family name [ ipv4/flow ]` it must be "0078" (capability sent, no tuple). RED now (c27-44-red.log): "00780001850000010100" and "007800018500". Warn assertion not written (needs the seam below). TestRFC4724GRCapabilityListsTheFamiliesOfTheSession NOT changed yet: it stays true under the ruling while ipv4/ipv6 unicast are RIB-held | none | weak (unchanged) | See "RULING 7 design question" below |

Scoped `go test -race` on plugins/gr green after the R59 fix (c27-gr2.log), run before rfc4724_rib_families_red_test.go existed; that file is the package's only red, on purpose.

### FRR, how it picks the GR capability families (bgpd/bgp_open.c, master, fetched 2026-10-02)

`bgp_peer_send_gr_capability`:

```
	if (CHECK_FLAG(peer->flags, PEER_FLAG_GRACEFUL_RESTART)) {
		FOREACH_AFI_SAFI (afi, safi) {
			bool f_bit = false;

			if (!peer->afc[afi][safi])
				continue;
			if (!bgp_gr_supported_for_afi_safi(afi, safi))
				continue;
```

and bgpd/bgpd.h:

```
static inline bool bgp_gr_supported_for_afi_safi(afi_t afi, safi_t safi)
{
	/*
	 * GR restarter behavior is supported only for IPv4-unicast,
	 * IPv6-unicast, L2vpn EVPN, and IPv4/IPv6 unreachability
	 */
	if ((afi == AFI_IP && safi == SAFI_UNICAST) || (afi == AFI_IP6 && safi == SAFI_UNICAST) ||
	    (afi == AFI_L2VPN && safi == SAFI_EVPN) || (afi == AFI_IP && safi == SAFI_UNREACH) ||
	    (afi == AFI_IP6 && safi == SAFI_UNREACH))
		return true;
	return false;
}
```

So FRR lists a family when the peer has it activated AND it is in a hard-coded list of families its restarter supports; the F bit comes from `bgp_gr_is_forwarding_preserved_for_safi`. Ze's ruling replaces FRR's hand list with a registry derivation.

### RULING 7 design question (why STOPPED)

1. No registry fact says "this plugin holds family X's routes and can re-send them". `Registration.Families` means "address families handled" for NLRI decode; bgp-rib registers no families and stores any family it is handed (no family gate in plugins/rib); bgp-gr already Depends on bgp-rib, so "a RIB is loaded" is always true wherever the capability is built.
2. "Loaded" is not knowable inside the gr plugin when the capability is declared: capabilities are set in OnConfigure (Stage 2/3); the SDK's only registry delivery, OnShareRegistry, is Stage 4 and carries commands, not plugins or families; `registry.AllFamilies()` lists what is compiled in, not what the config loaded.
3. Options: (a) a new `Registration` field (for example `RetainsFamilies`, or a per-family "re-sendable" mark) declared by bgp-rib and the NLRI plugins, with the ENGINE intersecting bgp-gr's code-64 tuples with the loaded set before the OPEN is built (reactor side, where loaded plugins are known), Warn per dropped family there; (b) the engine hands each plugin the loaded families at Stage 2 (SDK contract change, plugin API); (c) bgp-gr intersects with `registry.AllFamilies()` (compiled-in, not loaded: does not meet the ruling's "depends on the plugin loaded"). Recommendation: (a), because it keeps the fact with the plugin that owns it and needs no plugin API change. Needs the main thread or owner to pick, and it touches internal/component/plugin/registry and the reactor's capability assembly.
4. The F bit rule is untouched in every option.

### c27 files changed
- internal/component/bgp/plugins/gr/gr_llgr_exchange.go (NEW: exchangedLLGRLocked, recordSentLLGR, sent-OPEN readers for both event paths)
- internal/component/bgp/plugins/gr/gr.go (sentLLGRFamilies field; "open direction sent" subscription; sent-OPEN dispatch on both paths; 4 session-down/up sites read exchangedLLGRLocked; releaseRoutes forgets the record)
- internal/component/bgp/plugins/gr/gr_removal.go (onPeerRemoved forgets the record)
- internal/component/bgp/plugins/gr/rfc9494_helper_default_red_test.go (rewritten: two tagged units, RFC9494-5-1 - and +; it is green now, the "_red" in the name is c26's and could be renamed by the committer)
- internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go (declareLocalLLGR helper; three tagged units' setup, D-15 approvals)
- internal/component/bgp/plugins/gr/rfc4724_rib_families_red_test.go (NEW, untagged, RED on purpose: RULING 7)
- docs/guide/graceful-restart.md (LLGR intro and Configuration)
- rfc/discrimination/rfc9494.json (4 new 5-1 records, 7 re-recorded 4.2-x), rfc/discrimination/rfc4724.json (2 re-recorded 4.2-4)
- the rfc approval ledger (3 D-15 approvals above)
- plan/handover/rfc-verdict-test-fix-pass/bgp/c27-author.md (this file)

### Owed by the main thread or a fresh agent
- Scoped golangci-lint on plugins/gr; `./le rfc check` on rfc9494 and rfc4724 (I checked `./le rfc discriminate id` for 5-1, 4.2-1/2/4/5 and 4.2-4: no stale record left).
- `go test -race` on plugins/gr skipping TestRFC4724GRCapabilityListsOnlyFamiliesARIBHolds.
- Not started: rfc8277 (7 ids), RFC9252-5-3, RFC8669-3.1-3, 3.1-5, 6-3 (budget).

### Judge (BGP c27 judge, 2026-10-02, R59 part only)

| id | old -> new | judgement |
|----|-----------|-----------|
| RFC9494-5-1 | weak -> enforced | Producer verified: "open direction sent" reaches the plugin through onMessageSent -> proc.Deliver, the same per-process FIFO as the state event, and the OPEN is sent before Established, so the sent record is in place at session up (no race). Both event paths dispatch on direction; egress (Section 4.6) still reads the peer's own advertisement; GR-only unchanged. Independent break via `go test -overlay` (exchangedLLGRLocked returning every received family): the negative goes red in all four subcases. DEFECT (test, reported not fixed): the positive's "no command names ipv6/unicast" assertion stays green under that break, because rfc9494Opens builds a GR capability for IPv4 only, so IPv6 never enters GR; the per-family exclusion is unproven. It belongs to RFC9494-5-2, which stays {gap} (per-peer config granularity, owner YANG decision). |
| RFC9494-4.2-1, 4.2-2, 4.2-4, 4.2-5 | unchanged (enforced, enforced, enforced, weak) | D-15 setup edit only (declareLocalLLGR), assertions unchanged, 7 records re-observed red. Re-stamped mode rejudge. |
| RFC4724-4.2-4 | unchanged | 2 records re-recorded; audit verdict not stale. |

Gates: golangci-lint ./internal/component/bgp/plugins/gr/... 0 issues; `go test -race` on gr (skip TestRFC4724GRCapabilityListsOnlyFamiliesARIBHolds) ok; `./le rfc check` showed only the 5 rfc9494 stale verdicts, re-stamped here; rfc4724 clean. Excluded from the commit: gr/rfc4724_rib_families_red_test.go (OWNER RULING 7, red on purpose).
