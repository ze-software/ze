// Design: docs/architecture/core-design.md -- explicit terminal policy disposition.
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPolicyFilterChainFinalDisposition exercises exhaustion and raw terminal
// outcomes without allowing a filter's accept or modification to grant acceptance.
func TestPolicyFilterChainFinalDisposition(t *testing.T) {
	for _, final := range []filterapi.FinalDisposition{filterapi.FinalAccept, filterapi.FinalReject} {
		for _, action := range []PolicyAction{PolicyAccept, PolicyReject, PolicyModify} {
			for _, raw := range []bool{false, true} {
				policy, err := filterapi.NewPolicy([]filterapi.FilterRef{{Name: "test:first"}, {Name: "test:last"}}, final)
				if err != nil {
					t.Fatal(err)
				}
				calls := 0
				result := PolicyFilterChain(policy, directionExport, "192.0.2.1", 65001, "origin igp", func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
					calls++
					if raw && action == PolicyModify {
						return PolicyResponse{Action: action, Raw: []byte{0, 0, 0, 0}}
					}
					return PolicyResponse{Action: action, Delta: "local-preference 200"}
				})
				wantReject := final == filterapi.FinalReject || action == PolicyReject
				if (result.Action == PolicyReject) != wantReject || result.Failed || result.Teardown {
					t.Fatalf("final=%v action=%v raw=%v: result=%+v", final, action, raw, result)
				}
				wantCalls := 2
				if action == PolicyReject || (raw && action == PolicyModify) {
					wantCalls = 1
				}
				if calls != wantCalls {
					t.Fatalf("calls=%d want=%d", calls, wantCalls)
				}
			}
		}
	}
}

// TestPolicyFinalWithoutActiveFilters proves nil transport is irrelevant to a
// terminal decision, including a direction whose only reference is inactive.
func TestPolicyFinalWithoutActiveFilters(t *testing.T) {
	for _, refs := range [][]filterapi.FilterRef{nil, {{Name: "test:inactive", Inactive: true}}} {
		for _, final := range []filterapi.FinalDisposition{filterapi.FinalAccept, filterapi.FinalReject} {
			policy, err := filterapi.NewPolicy(refs, final)
			if err != nil {
				t.Fatal(err)
			}
			result := PolicyFilterChain(policy, directionExport, "192.0.2.1", 65001, "origin igp", nil)
			if (result.Action != PolicyReject) != policy.AcceptsAtEnd() || result.Failed {
				t.Fatalf("final=%v result=%+v", final, result)
			}
		}
	}
}

// TestPolicyFinalNilAPIRails applies empty and inactive policies through ingress,
// forwarded export and originated export; no filter transport is installed.
func TestPolicyFinalNilAPIRails(t *testing.T) {
	body := policyChainAdvertisingBody()
	for _, refs := range [][]filterapi.FilterRef{nil, {{Name: "test:off", Inactive: true}}} {
		for _, final := range []filterapi.FinalDisposition{filterapi.FinalAccept, filterapi.FinalReject} {
			policy := mustPolicy(t, refs, final)
			peer := peerWithExportPolicy(t, "192.0.2.1", policy)
			peer.settings.ImportPolicy = policy
			r := &Reactor{}
			want := final == filterapi.FinalAccept
			in := r.runIngressPolicyChain(peer, peer.Settings().Address, 65001, testWireUpdate(body), body)
			if in.accept != want {
				t.Fatalf("ingress final=%v accept=%v", final, in.accept)
			}
			out := r.runEgressPolicyChain(policy, "192.0.2.1", 65001, 65000, false, testWireUpdate(body))
			if out.accept != want || out.failed {
				t.Fatalf("forward final=%v result=%+v", final, out)
			}
			suppress, _ := r.exportFilterForBody(peer, body)
			if suppress == want {
				t.Fatalf("originated final=%v suppress=%v", final, suppress)
			}
		}
	}
}

// TestPolicyFinalIngressPreservesWithdrawals observes the RIB's delivery, not
// just a filter return: final reject strips additions but keeps legacy and MP
// withdrawals, including an MP_REACH of a different family in a mixed UPDATE.
func TestPolicyFinalIngressPreservesWithdrawals(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"legacy-pure", "000418c000020000", "000418c000020000"},
		{"legacy-mixed", "000418c00002000018c00003", "000418c000020000"},
		{"mp-pure", "0000000d800f0a0002013020010db80001", "0000000d800f0a0002013020010db80001"},
		{"mp-and-legacy", "0000000d800f0a0002013020010db8000118c00003", "0000000d800f0a0002013020010db80001"},
		{"two-mp-families", "0000001d800f0a0002013020010db80001800e0d00010104010101010018c00003", "0000000d800f0a0002013020010db80001"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addr := netip.MustParseAddr("203.0.113.9")
			source := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
				ImportPolicy: mustPolicy(t, nil, filterapi.FinalReject)})
			source.state.Store(int32(PeerStateEstablished))
			var delivered [][]byte
			r := &Reactor{clock: source.clock, config: &Config{LocalAS: 65000},
				peers:               map[netip.AddrPort]*Peer{source.Settings().PeerKey(): source},
				recentUpdates:       newRecentUpdateCache(100),
				orderedIngressSteps: []orderedIngressStep{{name: policyChainStepName, policyChain: true}},
				messageReceiver: &testDeliveryReceiver{consumerCount: 1, onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
					delivered = append(delivered, bytes.Clone(msg.RawBytes))
				}}}
			body := mustHex(t, tc.body)
			wu := wireu.NewWireUpdate(body, 0)
			wu.SetSourceID(source.SourceID())
			r.notifyMessageReceiver(addr, msgtype.TypeUPDATE, body, wu, 0, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0)
			if len(delivered) != 1 || !bytes.Equal(delivered[0], mustHex(t, tc.want)) {
				t.Fatalf("consumer received %x, want %s", delivered, tc.want)
			}
			destination := peerWithExportPolicy(t, "203.0.113.10", mustPolicy(t, nil, filterapi.FinalReject))
			suppress, withdrawals := r.exportFilterForBody(destination, body)
			if suppress || !bytes.Equal(withdrawals, mustHex(t, tc.want)) {
				t.Fatalf("API withdrawal suppressed=%v body=%x want=%s", suppress, withdrawals, tc.want)
			}
		})
	}
}

// TestPolicyFinalReloadRetainedRoutes uses live sessions and the registered
// receive store. The source sends once; final transitions and rollback must
// withdraw and restore its route without another source UPDATE or a reconnect.
func TestPolicyFinalReloadRetainedRoutes(t *testing.T) {
	settings := []*PeerSettings{lowLiveSettings("192.0.2.1", 65000, 65002), lowLiveSettings("192.0.2.2", 65000, 65003)}
	for _, setting := range settings {
		setting.RSClient = true
	}
	settings[1].ExportPolicy = mustPolicy(t, nil, filterapi.FinalAccept)
	peers := rsLiveRouter(t, settings...)
	source, target := peers[0], peers[1]
	session := target.peer.currentSession()
	prefix := netip.MustParsePrefix("10.17.0.0/24")
	attrs := firstASAttrs(4, 65002)
	ownershipLiveSend(t, source, nil, attrs, prefix)
	ownershipLiveThrough(t, target, prefix, attrs)
	previous := target.peer.hotSwappableSnapshot(hotSwappableSettings)
	next := target.peer.settingsSnapshot()
	next.ExportPolicy = mustPolicy(t, nil, filterapi.FinalReject)
	mustApplyHotSwappableSettings(t, target.peer, next, hotSwappableSettings)
	lowEventually(t, func() bool { _, held := ownershipLiveRead(t, target).routes[prefix]; return !held }, "final reject withdraws prior advertisement")
	absent := target.peer.settingsSnapshot()
	absent.ExportPolicy = filterapi.Policy{}
	mustApplyHotSwappableSettings(t, target.peer, absent, hotSwappableSettings)
	ownershipLiveThrough(t, target, prefix, attrs)
	mustApplyHotSwappableSettings(t, target.peer, next, hotSwappableSettings)
	lowEventually(t, func() bool { _, held := ownershipLiveRead(t, target).routes[prefix]; return !held }, "reject after removal withdraws again")
	mustApplyHotSwappableSettings(t, target.peer, previous, hotSwappableSettings)
	ownershipLiveThrough(t, target, prefix, attrs)
	if target.peer.currentSession() != session {
		t.Fatal("final-only reload replaced the session")
	}
	ownershipLiveEstablished(t, peers)
}

// TestPolicyFinalReloadStaticRoutes observes the normal originated writer on
// one connection, including restoration of a configured route after denial.
func TestPolicyFinalReloadStaticRoutes(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	peer.settings.LocalAddress = netip.MustParseAddr("10.9.9.9")
	peer.settings.StaticRoutes = []StaticRoute{staticRouteAt("10.17.1.0/24", "10.9.9.9")}
	peer.settings.DefaultOriginate = map[string]bool{"ipv4/unicast": true}
	peer.settings.ExportPolicy = mustPolicy(t, nil, filterapi.FinalAccept)
	peer.reactor = &Reactor{clock: clock.RealClock{},
		peers: map[netip.AddrPort]*Peer{peer.Settings().PeerKey(): peer}}
	session := peer.currentSession()
	session.adjOut = &peer.adjOut
	session.egressFinalAccepts = func() bool { return peer.ExportPolicy().AcceptsAtEnd() }
	peer.sendInitialRoutes()
	assertHeld := func(want bool) {
		t.Helper()
		held := make(map[netip.Prefix]bool)
		for _, update := range recoveryWrittenUpdates(t, conn.written()) {
			for _, prefix := range ownershipLivePrefixes(t, update.WithdrawnRoutes) {
				delete(held, prefix)
			}
			for _, prefix := range ownershipLivePrefixes(t, update.NLRI) {
				held[prefix] = true
			}
		}
		for _, prefix := range []netip.Prefix{netip.MustParsePrefix("10.17.1.0/24"), netip.MustParsePrefix("0.0.0.0/0")} {
			if held[prefix] != want {
				t.Fatalf("wire holds %s=%v, want %v: %x", prefix, held[prefix], want, conn.written())
			}
		}
	}
	assertHeld(true)
	next := peer.settingsSnapshot()
	next.ExportPolicy = mustPolicy(t, nil, filterapi.FinalReject)
	mustApplyHotSwappableSettings(t, peer, next, hotSwappableSettings)
	assertHeld(false)
	next = peer.settingsSnapshot()
	next.ExportPolicy = filterapi.Policy{}
	mustApplyHotSwappableSettings(t, peer, next, hotSwappableSettings)
	assertHeld(true)
	if peer.currentSession() != session {
		t.Fatal("static final reload replaced the session")
	}
}

// TestPolicyFinalRejectLeavesNativeRSForPolicy requires the fast path to hand
// empty and inactive rejecting policies to the ordered policy-aware rail.
func TestPolicyFinalRejectLeavesNativeRSForPolicy(t *testing.T) {
	for _, refs := range [][]filterapi.FilterRef{nil, {{Name: "test:off", Inactive: true}}} {
		ctx := bgpctx.EncodingContextForASN4(true)
		ctxID, _ := bgpctx.Registry.Register(ctx)
		source := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
		destination := makeRSPeer(t, "10.0.0.2", 65002, ctx, ctxID)
		destination.settings.ExportPolicy = mustPolicy(t, refs, filterapi.FinalReject)
		destination.refreshForwardFacts()
		update := &ReceivedUpdate{WireUpdate: wireu.NewWireUpdate(policyChainAdvertisingBody(), ctxID), SourcePeerIP: source.Settings().Address}
		r := &Reactor{peers: map[netip.AddrPort]*Peer{source.Settings().PeerKey(): source, destination.Settings().PeerKey(): destination}}
		skipped, dispatched := reactorForwardRS(r, update, 51, source.Settings().Address, source)
		if dispatched != 0 || len(skipped) != 1 || skipped[0] != destination.Settings().PeerKey() {
			t.Fatalf("final reject skipped=%v dispatched=%d", skipped, dispatched)
		}
	}
}

// TestPolicyFinalOnlyDecisionDoesNotAllocate protects the no-RPC/no-text path.
func TestPolicyFinalOnlyDecisionDoesNotAllocate(t *testing.T) {
	body := policyChainAdvertisingBody()
	update := testWireUpdate(body)
	policy := mustPolicy(t, nil, filterapi.FinalReject)
	peer := peerWithExportPolicy(t, "192.0.2.1", policy)
	peer.settings.ImportPolicy = policy
	r := &Reactor{}
	allocations := testing.AllocsPerRun(100, func() {
		in := r.runIngressPolicyChain(peer, peer.Settings().Address, 65001, update, body)
		out := r.runEgressPolicyChain(policy, "192.0.2.1", 65001, 65000, false, update)
		suppressed, _ := r.exportFilterForBody(peer, body)
		if in.accept || out.accept || out.failed || !suppressed {
			panic("final reject failed")
		}
	})
	if allocations != 0 {
		t.Fatalf("final-only decisions allocated %g times", allocations)
	}
}

// TestPolicyFinalWriterRejectsOldPrefilteredAnnouncement closes the race where
// an already-evaluated announcement reaches the writer after a policy swap.
func TestPolicyFinalWriterRejectsOldPrefilteredAnnouncement(t *testing.T) {
	r, _, destination, conn, _ := newSyncOrderRail(t)
	r.clock = clock.RealClock{}
	destination.sendingInitialRoutes.Store(0)
	session := destination.currentSession()
	session.egressFinalAccepts = func() bool { return destination.ExportPolicy().AcceptsAtEnd() }
	update, err := message.UnpackUpdate(syncOrderAnnounceBody)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.writeUpdatePreFiltered(update); err != nil {
		t.Fatal(err)
	}
	if err := session.flushWrites(); err != nil {
		t.Fatal(err)
	}
	before := len(conn.written())
	destination.settings.ExportPolicy = mustPolicy(t, nil, filterapi.FinalReject)
	if err := session.writeUpdatePreFiltered(update); err != nil {
		t.Fatal(err)
	}
	if len(conn.written()) != before {
		t.Fatal("stale prefiltered announcement escaped final reject")
	}
	destination.refreshForwardFacts()
	batch := recoveryDepartureBatch(netip.Addr{}, "192.0.2.0/24")
	api := &reactorAPIAdapter{r: r}
	if err := api.WithdrawNLRIBatch(t.Context(), selector.ParseDefault(destination.Settings().Address.String()), batch, plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
	written := recoveryWrittenUpdates(t, conn.written()[before:])
	if len(written) != 1 || !bytes.Equal(written[0].WithdrawnRoutes, syncOrderPrefixWire) {
		t.Fatalf("writer swallowed API withdrawal: %+v", written)
	}
}

// TestPolicyFinalRecoveryWithdrawsAfterDeniedSurvivor observes the actual
// recovery writer: final rejection is a candidate rejection, not a failed guard
// that leaves the old source's advertisement in place.
func TestPolicyFinalRecoveryWithdrawsAfterDeniedSurvivor(t *testing.T) {
	for _, refs := range [][]filterapi.FilterRef{nil, {{Name: "test:off", Inactive: true}}} {
		r, source, destination, conn, ctxID := newSyncOrderRail(t)
		r.clock = clock.RealClock{}
		source.recvCtxID = ctxID
		destination.sendingInitialRoutes.Store(0)
		destination.session.setSendCtxID(ctxID)
		destination.session.onMessageReceived = r.notifyMessageReceiver
		destination.settings.NextHopMode = NextHopUnchanged
		destination.refreshForwardFacts()
		failed := recoveryAdvertiseFailed(t, r, destination, syncOrderAnnounceBody)
		before := len(conn.written())
		recoveryOwnerStart(t, r, destination)
		destination.settings.ExportPolicy = mustPolicy(t, refs, filterapi.FinalReject)
		destination.refreshForwardFacts()
		r.orderedEgressSteps = []orderedEgressStep{{name: policyChainStepName, policyChain: true}}
		stored := storedIPv4Route(source.Settings().Address.String())
		original := mustHex(t, stored.AttrHex)
		owner := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
			return recoverySurvivorAnswer(request, syncOrderPrefixWire, ribevents.RecoveryRoute{Source: source.Settings().Address,
				NLRI: syncOrderPrefixWire, Attributes: original, NextHop: []byte{1, 1, 1, 1}, MessageID: 81}), nil
		})
		t.Cleanup(owner.Close)
		if err := recoveryDepart(t, r, destination, failed.Settings().Address, "192.0.2.0/24"); err != nil {
			t.Fatal(err)
		}
		written := recoveryWrittenUpdates(t, conn.written()[before:])
		if len(written) != 1 || len(written[0].NLRI) != 0 || !bytes.Equal(written[0].WithdrawnRoutes, syncOrderPrefixWire) {
			t.Fatalf("final reject failed to retire old advertisement: %+v", written)
		}
		owner.Close()
		r.fwdPool.Stop()
	}
}

// TestPolicyFinalDoesNotReplaceTerminalFailure preserves transport and teardown
// distinctions even though a final reject is itself a genuine policy decision.
func TestPolicyFinalDoesNotReplaceTerminalFailure(t *testing.T) {
	for _, final := range []filterapi.FinalDisposition{filterapi.FinalAccept, filterapi.FinalReject} {
		for _, terminal := range []PolicyResponse{
			{Action: PolicyReject, Failed: true},
			{Action: PolicyReject, Teardown: true, NotifyCode: 6, NotifySubcode: 2},
		} {
			policy := mustPolicy(t, []filterapi.FilterRef{{Name: "test:first"}, {Name: "test:unreachable"}}, final)
			calls := 0
			result := PolicyFilterChain(policy, directionImport, "192.0.2.1", 65001, "origin igp", func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
				calls++
				return terminal
			})
			if calls != 1 || result.Action != PolicyReject || result.Failed != terminal.Failed ||
				result.Teardown != terminal.Teardown || result.NotifyCode != terminal.NotifyCode || result.NotifySubcode != terminal.NotifySubcode {
				t.Fatalf("terminal result changed by final=%v: %+v", final, result)
			}
		}
	}
}

// TestPolicyFinalReloadStoreSelfReplay preserves the receive store's existing
// per-peer fallback when no exclusive replay claimant is attached to that peer.
func TestPolicyFinalReloadStoreSelfReplay(t *testing.T) {
	settings := []*PeerSettings{lowLiveSettings("192.0.2.1", 65000, 65002), lowLiveSettings("192.0.2.2", 65000, 65003)}
	for _, setting := range settings {
		setting.RSClient = true
	}
	settings[1].ExportPolicy = mustPolicy(t, nil, filterapi.FinalAccept)
	peers := rsLiveRouter(t, settings...)
	source, destination := peers[0], peers[1]
	prefix := netip.MustParsePrefix("10.17.2.0/24")
	attrs := firstASAttrs(4, 65002)
	ownershipLiveSend(t, source, nil, attrs, prefix)
	ownershipLiveThrough(t, destination, prefix, attrs)
	next := destination.peer.settingsSnapshot()
	next.ExportPolicy = mustPolicy(t, nil, filterapi.FinalReject)
	mustApplyHotSwappableSettings(t, destination.peer, next, hotSwappableSettings)
	lowEventually(t, func() bool { _, held := ownershipLiveRead(t, destination).routes[prefix]; return !held }, "self-replay setup withdrawal")
	next = destination.peer.settingsSnapshot()
	next.ExportPolicy = filterapi.Policy{}
	var bindings []ProcessBinding
	for _, binding := range next.ProcessBindings {
		if binding.PluginName != "bgp-rs" {
			bindings = append(bindings, binding)
		}
	}
	next.ProcessBindings = bindings
	// A binding with a send grant is not hot-swappable. Install the intended
	// receive-store-only graph explicitly before exercising policy restoration.
	destination.peer.writeBindings(destination.peer.reactor, bindings)
	mustApplyHotSwappableSettings(t, destination.peer, next, hotSwappableSettings)
	ownershipLiveThrough(t, destination, prefix, attrs)
	if owner := destination.peer.policyReplayOwner(); owner != plugin.ProcessSender("bgp-adj-rib-in") {
		t.Fatalf("self-replay used the wrong owner: %+v", owner)
	}
}
