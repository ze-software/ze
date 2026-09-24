package rib

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func flowValidationPath(kind byte, asns ...uint32) []byte {
	if len(asns) == 0 {
		return nil
	}
	path := []byte{kind, byte(len(asns))}
	for _, asn := range asns {
		path = binary.BigEndian.AppendUint32(path, asn)
	}
	return path
}

func flowValidationAttrs(path []byte, originator netip.Addr, med uint32, communities []byte) []byte {
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, byte(len(path))}
	attrs = append(attrs, path...)
	attrs = append(attrs, 0x80, 4, 4)
	attrs = binary.BigEndian.AppendUint32(attrs, med)
	if originator.IsValid() {
		attrs = append(attrs, 0x80, 9, 4)
		id := originator.As4()
		attrs = append(attrs, id[:]...)
	}
	if communities != nil {
		attrs = append(attrs, 0xc0, 16, byte(len(communities)))
		attrs = append(attrs, communities...)
	}
	return attrs
}

// flowValidationReceive drives the structured UPDATE entrypoint with the same
// generation and peer metadata the reactor supplies. No helper writes RIB state.
func flowValidationReceive(t *testing.T, r *RIBManager, peer netip.Addr, peerAS uint32, id uint64, fam family.Family, raw, attrs []byte, withdraw bool) *rpc.StructuredEvent {
	const localAS uint32 = 65000
	t.Helper()
	attrs = bytes.Clone(attrs)
	mp := []byte{byte(fam.AFI >> 8), byte(fam.AFI), byte(fam.SAFI)}
	code := byte(15)
	if !withdraw {
		code = 14
		var nextHop []byte
		if !ribevents.IsFlowSpec(fam) {
			nextHop = []byte{192, 0, 2, 254}
			if fam.AFI == family.AFIIPv6 {
				addr := netip.MustParseAddr("2001:db8::ffff").As16()
				nextHop = addr[:]
			}
			if fam.SAFI == family.SAFIVPN {
				nextHop = append(make([]byte, 8), nextHop...)
			}
		}
		mp = append(mp, byte(len(nextHop)))
		mp = append(mp, nextHop...)
		mp = append(mp, 0)
	}
	mp = append(mp, raw...)
	attrs = append(attrs, 0x80, code, byte(len(mp)))
	attrs = append(attrs, mp...)
	body := []byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}
	body = append(body, attrs...)
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	if err != nil {
		t.Fatal(err)
	}
	wu := wireu.NewWireUpdate(body, ctxID)
	aw, err := wu.Attrs()
	if err != nil {
		t.Fatal(err)
	}
	event := &rpc.StructuredEvent{
		EventType: rpc.EventKindUpdate, PeerAddress: peer.String(), PeerAS: peerAS, LocalAS: localAS,
		// A shared BGP Identifier must never substitute for source transport IP.
		RemoteRouterID: 0x01010101,
		RawMessage:     &bgptypes.RawMessage{Type: msgtype.TypeUPDATE, MessageID: id, RawBytes: body, WireUpdate: wu, AttrsWire: aw},
	}
	r.handleReceivedStructured(event)
	return event
}

func flowValidationFixture(t *testing.T) (*RIBManager, *testEventBus) {
	t.Helper()
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	ribevents.RegisterFlowSpecLookup(r.flowSpecEligible, r.flowSpecPresent, r.flowSpecPath, r.flowSpecRoutes)
	t.Cleanup(func() {
		ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil)
		for _, peer := range r.bgpPeers {
			peer.Release()
		}
	})
	ribevents.ValidationChange.Subscribe(bus, r.validationChanged)
	ribevents.ReplayRequest.Subscribe(bus, r.replayBestPaths)
	return r, bus
}

func flowValidationEvents(bus *testEventBus) []*ribevents.FlowSpecChange {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	var result []*ribevents.FlowSpecChange
	for _, event := range bus.events {
		if change, ok := event.Payload.(*ribevents.FlowSpecChange); ok {
			result = append(result, change)
		}
	}
	return result
}

// Observe authorization at the selected-rule event and candidate entrypoint.
// RFC requirement: RFC8955-6-1 positive -- received rules with a matching covering unicast originator, or the RFC 9117 local-controller alternative, become selected installs.
// RFC requirement: RFC8955-6-1 negative -- missing destinations, missing unicast, foreign originators and foreign more-specific routes remain retained but cannot become candidates or installs.
// RFC requirement: RFC8955-6-2 positive -- under RFC 9117 Section 4, a route-server peer ASN may differ when the rule and covering unicast route have the same leftmost AS_SEQUENCE ASN.
// RFC requirement: RFC8955-6-2 negative -- external rules with another leftmost ASN, or only an AS_SET, cannot be authorized by the covering unicast route.
// Stored but infeasible UPDATEs remain available for later recovery.
func TestFlowSpecAuthorizationFromReceivedUpdates(t *testing.T) {
	unicastPeer := netip.MustParseAddr("192.0.2.1")
	otherPeer := netip.MustParseAddr("192.0.2.2")
	originator := netip.MustParseAddr("203.0.113.1")
	sequence := flowValidationPath(2, 65001)
	drop := []byte{0x80, 6, 0, 0, 0, 0, 0, 0}
	for _, tc := range []struct {
		name        string
		peer        netip.Addr
		peerAS      uint32
		path        []byte
		uniOrigin   netip.Addr
		flowOrigin  netip.Addr
		destination bool
		unicast     bool
		specificAS  uint32
		want        bool
	}{
		{name: "external authorized through route server", peer: unicastPeer, peerAS: 65100, path: sequence, destination: true, unicast: true, want: true},
		{name: "destination required", peer: unicastPeer, peerAS: 65001, path: sequence, unicast: true},
		{name: "covering unicast required", peer: unicastPeer, peerAS: 65001, path: sequence, destination: true},
		{name: "same router ID different transport", peer: otherPeer, peerAS: 65001, path: sequence, destination: true, unicast: true},
		{name: "matching originator IDs", peer: otherPeer, peerAS: 65001, path: sequence, uniOrigin: originator, flowOrigin: originator, destination: true, unicast: true, want: true},
		{name: "originator ID differs from transport", peer: unicastPeer, peerAS: 65001, path: sequence, flowOrigin: originator, destination: true, unicast: true},
		{name: "local controller empty path", peer: otherPeer, peerAS: 65000, destination: true, unicast: true, want: true},
		{name: "confederation controller", peer: otherPeer, peerAS: 65002, path: flowValidationPath(3, 65002), destination: true, unicast: true, want: true},
		{name: "confederation SET controller", peer: otherPeer, peerAS: 65002, path: flowValidationPath(4, 65002), destination: true, unicast: true, want: true},
		{name: "external first AS mismatch", peer: unicastPeer, peerAS: 65100, path: flowValidationPath(2, 65002), destination: true, unicast: true},
		{name: "AS SET is not leftmost sequence", peer: unicastPeer, peerAS: 65001, path: flowValidationPath(1, 65001), destination: true, unicast: true},
		{name: "more specific foreign AS", peer: unicastPeer, peerAS: 65001, path: sequence, destination: true, unicast: true, specificAS: 65002},
		{name: "more specific same AS", peer: unicastPeer, peerAS: 65001, path: sequence, destination: true, unicast: true, specificAS: 65001, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, bus := flowValidationFixture(t)
			if tc.unicast {
				flowValidationReceive(t, r, unicastPeer, 65001, 1, family.IPv4Unicast, []byte{8, 10}, flowValidationAttrs(sequence, tc.uniOrigin, 0, nil), false)
			}
			if tc.specificAS != 0 {
				flowValidationReceive(t, r, otherPeer, tc.specificAS, 2, family.IPv4Unicast, []byte{24, 10, 1, 1}, flowValidationAttrs(flowValidationPath(2, tc.specificAS), netip.Addr{}, 0, nil), false)
			}
			raw := []byte{3, 3, 0x81, 6}
			if tc.destination {
				raw = flowspecNLRI(16, 10, 1)
			}
			flowValidationReceive(t, r, tc.peer, tc.peerAS, 3, flowspecFamily, raw, flowValidationAttrs(tc.path, tc.flowOrigin, 100, drop), false)
			key := ribevents.ValidationRoute{Peer: tc.peer, Family: flowspecFamily, NLRI: string(raw)}
			if got := ribevents.RouteEligible(key, 3); got != tc.want {
				t.Fatalf("eligibility = %v, want %v", got, tc.want)
			}
			if !ribevents.RoutePresent(key) {
				t.Fatal("authorization discarded the received route")
			}
			if got := len(r.gatherCandidates(flowspecFamily, raw)) != 0; got != tc.want {
				t.Fatalf("candidate availability = %v, want %v", got, tc.want)
			}
			events := flowValidationEvents(bus)
			if !tc.want {
				if len(events) != 0 {
					t.Fatalf("infeasible rule installed: %+v", events)
				}
				return
			}
			if len(events) != 1 || events[0].Withdraw || !bytes.Equal(events[0].NLRI, raw) || !bytes.Equal(events[0].ExtendedCommunities, drop) {
				t.Fatalf("selected event = %+v", events)
			}
			if ribevents.RouteEligible(key, 2) {
				t.Fatal("older generation authorized over current UPDATE")
			}
		})
	}
}

// RFC requirement: RFC8955-6-3 positive -- received unicast changes and validation recovery reinstall a retained rule without another FlowSpec UPDATE.
// RFC requirement: RFC8955-6-3 negative -- a foreign more-specific route, loss of the covering route, unicast ineligibility or session removal withdraws the installed rule.
func TestFlowSpecUnicastLifecycleRevalidatesRetainedRule(t *testing.T) {
	r, bus := flowValidationFixture(t)
	peer := netip.MustParseAddr("192.0.2.1")
	foreign := netip.MustParseAddr("192.0.2.2")
	path := flowValidationPath(2, 65001)
	attrs := flowValidationAttrs(path, netip.Addr{}, 100, nil)
	raw := flowspecNLRI(16, 10, 1)
	flowValidationReceive(t, r, peer, 65001, 1, flowspecFamily, raw, attrs, false)
	if len(flowValidationEvents(bus)) != 0 {
		t.Fatal("rule installed before its unicast route")
	}
	assertLast := func(count int, withdraw bool) {
		t.Helper()
		events := flowValidationEvents(bus)
		if len(events) != count || events[count-1].Withdraw != withdraw || !bytes.Equal(events[count-1].NLRI, raw) {
			t.Fatalf("events = %+v, want count %d and withdraw %v", events, count, withdraw)
		}
	}
	flowValidationReceive(t, r, peer, 65001, 2, family.IPv4Unicast, []byte{8, 10}, attrs, false)
	assertLast(1, false)
	flowValidationReceive(t, r, foreign, 65002, 3, family.IPv4Unicast, []byte{24, 10, 1, 1}, flowValidationAttrs(flowValidationPath(2, 65002), netip.Addr{}, 0, nil), false)
	assertLast(2, true)
	flowValidationReceive(t, r, foreign, 65002, 4, family.IPv4Unicast, []byte{24, 10, 1, 1}, nil, true)
	assertLast(3, false)
	flowValidationReceive(t, r, peer, 65001, 5, family.IPv4Unicast, []byte{8, 10}, nil, true)
	assertLast(4, true)
	flowValidationReceive(t, r, peer, 65001, 6, family.IPv4Unicast, []byte{8, 10}, attrs, false)
	assertLast(5, false)
	eligible := false
	ribevents.RegisterValidationLookup(func(key ribevents.ValidationRoute, _ uint64) bool {
		return key.Family != family.IPv4Unicast || eligible
	}, nil)
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	key := ribevents.ValidationRoute{Peer: peer, Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("10.0.0.0/8")}
	_, _ = ribevents.ValidationChange.Emit(bus, []ribevents.ValidationRoute{key})
	assertLast(6, true)
	eligible = true
	_, _ = ribevents.ValidationChange.Emit(bus, []ribevents.ValidationRoute{key})
	assertLast(7, false)
	r.dispatchHook = func(string) {}
	r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: peer.String(), State: rpc.SessionStateUp})
	r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: peer.String(), State: rpc.SessionStateDown})
	assertLast(8, true)
}

func TestFlowSpecSelectedCommunitiesReplacementAndReplay(t *testing.T) {
	r, bus := flowValidationFixture(t)
	peerA := netip.MustParseAddr("192.0.2.1")
	peerB := netip.MustParseAddr("192.0.2.2")
	origin := netip.MustParseAddr("203.0.113.1")
	path := flowValidationPath(2, 65001)
	drop := []byte{0x80, 6, 0, 0, 0, 0, 0, 0}
	mark := []byte{0x80, 9, 0, 0, 0, 0, 0, 46}
	raw := flowspecNLRI(24, 10, 0, 0)
	flowValidationReceive(t, r, peerA, 65001, 1, family.IPv4Unicast, []byte{8, 10}, flowValidationAttrs(path, origin, 0, nil), false)
	flowValidationReceive(t, r, peerA, 65001, 2, flowspecFamily, raw, flowValidationAttrs(path, origin, 100, drop), false)
	flowValidationReceive(t, r, peerB, 65001, 3, flowspecFamily, raw, flowValidationAttrs(path, origin, 200, mark), false)
	if events := flowValidationEvents(bus); len(events) != 1 || !bytes.Equal(events[0].ExtendedCommunities, drop) {
		t.Fatalf("losing source replaced selected actions: %+v", events)
	}
	flowValidationReceive(t, r, peerA, 65001, 4, flowspecFamily, raw, flowValidationAttrs(path, origin, 100, mark), false)
	if events := flowValidationEvents(bus); len(events) != 2 || !bytes.Equal(events[1].ExtendedCommunities, mark) || !bytes.Equal(events[0].ExtendedCommunities, drop) {
		t.Fatalf("community-only update lost or changed earlier owned event: %+v", events)
	}
	_, err := ribevents.ReplayRequest.Emit(bus, &replay.Request{ReplayID: replay.Broadcast})
	if err != nil {
		t.Fatal(err)
	}
	events := flowValidationEvents(bus)
	if len(events) != 3 || events[2].Withdraw || !bytes.Equal(events[2].ExtendedCommunities, mark) || !bytes.Equal(events[2].NLRI, raw) {
		t.Fatalf("replay did not carry selected actions: %+v", events)
	}
	flowValidationReceive(t, r, peerA, 65001, 5, flowspecFamily, raw, nil, true)
	if events = flowValidationEvents(bus); len(events) != 4 || events[3].Withdraw || !bytes.Equal(events[3].ExtendedCommunities, mark) {
		t.Fatalf("withdrawal failed to select surviving eligible source: %+v", events)
	}
}

// RFC requirement: RFC8956-5-1 positive -- a received IPv6 destination with offset zero is authorized by same-RD VPN unicast.
// RFC requirement: RFC8956-5-1 negative -- a legally encoded /64 destination with offset 32 remains infeasible despite a same-RD covering default route.
func TestFlowSpecVPNValidationSeparatesAFIAndRD(t *testing.T) {
	for _, afi := range []family.AFI{family.AFIIPv4, family.AFIIPv6} {
		t.Run(afi.String(), func(t *testing.T) {
			r, bus := flowValidationFixture(t)
			peer := netip.MustParseAddr("192.0.2.1")
			flowFamily := family.Family{AFI: afi, SAFI: family.SAFIFlowSpecVPN}
			vpnFamily := family.Family{AFI: afi, SAFI: family.SAFIVPN}
			rd := []byte{0, 0, 0xfd, 0xe9, 0, 0, 0, 1}
			prefix := []byte{24, 10, 0, 0}
			component := []byte{1, 24, 10, 0, 0}
			if afi == family.AFIIPv6 {
				prefix = []byte{64, 0x20, 1, 0x0d, 0xb8, 0, 1, 0, 0}
				component = append([]byte{1, 64, 0}, prefix[1:]...)
			}
			raw := append([]byte{byte(len(rd) + len(component))}, rd...)
			raw = append(raw, component...)
			vpn := append([]byte{prefix[0] + 88, 0, 1, 1}, rd...)
			vpn = append(vpn, prefix[1:]...)
			wrongRD := bytes.Clone(vpn)
			wrongRD[11] = 2
			attrs := flowValidationAttrs(flowValidationPath(2, 65001), netip.Addr{}, 0, nil)
			flowValidationReceive(t, r, peer, 65001, 1, flowFamily, raw, attrs, false)
			flowValidationReceive(t, r, peer, 65001, 2, family.Family{AFI: afi, SAFI: family.SAFIUnicast}, prefix, attrs, false)
			flowValidationReceive(t, r, peer, 65001, 3, vpnFamily, wrongRD, attrs, false)
			if len(flowValidationEvents(bus)) != 0 {
				t.Fatal("global unicast or another RD authorized VPN FlowSpec")
			}
			flowValidationReceive(t, r, peer, 65001, 4, vpnFamily, vpn, attrs, false)
			events := flowValidationEvents(bus)
			if len(events) != 1 || events[0].Family != flowFamily || !bytes.Equal(events[0].NLRI, raw) {
				t.Fatalf("same-domain VPN route did not authorize: %+v", events)
			}
			if afi == family.AFIIPv6 {
				// A default route makes offset, not missing unicast coverage, the
				// reason the otherwise legal shortened pattern is infeasible.
				vpnDefault := append([]byte{88, 0, 1, 1}, rd...)
				flowValidationReceive(t, r, peer, 65001, 5, vpnFamily, vpnDefault, attrs, false)
				offsetComponent := []byte{1, 64, 32, 0, 1, 0, 0}
				offsetRaw := append([]byte{byte(len(rd) + len(offsetComponent))}, rd...)
				offsetRaw = append(offsetRaw, offsetComponent...)
				flowValidationReceive(t, r, peer, 65001, 6, flowFamily, offsetRaw, attrs, false)
				key := ribevents.ValidationRoute{Peer: peer, Family: flowFamily, NLRI: string(offsetRaw)}
				if !ribevents.RoutePresent(key) {
					t.Fatal("legal four-byte offset pattern was rejected instead of retained")
				}
				if ribevents.RouteEligible(key, 6) || len(r.gatherCandidates(flowFamily, offsetRaw)) != 0 {
					t.Fatal("nonzero destination offset passed IPv6 validation")
				}
				events = flowValidationEvents(bus)
				if len(events) != 1 || !bytes.Equal(events[0].NLRI, raw) {
					t.Fatalf("offset rule changed the authorized zero-offset selection: %+v", events)
				}
			}
		})
	}
}

func TestFlowSpecAddPathCandidatesShareOneNativeRule(t *testing.T) {
	r, bus := flowValidationFixture(t)
	peer := netip.MustParseAddr("192.0.2.1")
	flowValidationReceive(t, r, peer, 65001, 1, family.IPv4Unicast, []byte{8, 10},
		flowValidationAttrs(flowValidationPath(2, 65001), netip.MustParseAddr("1.1.1.1"), 0, nil), false)
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{flowspecFamily: true}))
	if err != nil {
		t.Fatal(err)
	}
	raw := flowspecNLRI(24, 10, 0, 0)
	first := append(binary.BigEndian.AppendUint32(nil, 42), raw...)
	second := append(binary.BigEndian.AppendUint32(nil, 43), raw...)
	feedReceived(r, peer, ctxID, flowspecAnnounceBody(100, first))
	feedReceived(r, peer, ctxID, flowspecAnnounceBody(200, second))
	events := flowValidationEvents(bus)
	if len(events) != 1 || events[0].Withdraw || !bytes.Equal(events[0].NLRI, raw) {
		t.Fatalf("received Path Identifiers split the installed rule: %+v", events)
	}
	feedReceived(r, peer, ctxID, flowspecWithdrawBody(first))
	events = flowValidationEvents(bus)
	if len(events) != 2 || events[1].Withdraw || !bytes.Equal(events[1].NLRI, raw) {
		t.Fatalf("losing the first path failed to select the second: %+v", events)
	}
	feedReceived(r, peer, ctxID, flowspecWithdrawBody(second))
	events = flowValidationEvents(bus)
	if len(events) != 3 || !events[2].Withdraw || !bytes.Equal(events[2].NLRI, raw) {
		t.Fatalf("last-path withdrawal changed the native key: %+v", events)
	}
}

func TestFlowSpecReentrantWithdrawalOrdersReplayAfterChange(t *testing.T) {
	r, bus := flowValidationFixture(t)
	peer := netip.MustParseAddr("192.0.2.1")
	raw := flowspecNLRI(24, 10, 0, 0)
	attrs := flowValidationAttrs(flowValidationPath(2, 65001), netip.Addr{}, 0, nil)
	flowValidationReceive(t, r, peer, 65001, 1, flowspecFamily, raw, attrs, false)
	ribevents.FlowSpecChanged.Subscribe(bus, func(change *ribevents.FlowSpecChange) {
		if change.Withdraw {
			return
		}
		// A subscriber may request startup replay while another producer changes
		// the unicast topology. Neither callback may run under a RIB lock.
		_, _ = ribevents.ReplayRequest.Emit(bus, &replay.Request{ReplayID: replay.Broadcast})
		flowValidationReceive(t, r, peer, 65001, 3, family.IPv4Unicast, []byte{8, 10}, nil, true)
	})
	flowValidationReceive(t, r, peer, 65001, 2, family.IPv4Unicast, []byte{8, 10}, attrs, false)
	events := flowValidationEvents(bus)
	if len(events) != 2 || events[0].Withdraw || !events[1].Withdraw ||
		!bytes.Equal(events[0].NLRI, raw) || !bytes.Equal(events[1].NLRI, raw) {
		t.Fatalf("reentrant replay resurrected a withdrawn rule: %+v", events)
	}
}

func TestFlowSpecForeignMoreSpecificLosingPathInvalidatesRule(t *testing.T) {
	r, bus := flowValidationFixture(t)
	peer, foreign := netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.2")
	attrs := flowValidationAttrs(flowValidationPath(2, 65001), netip.Addr{}, 100, nil)
	flowValidationReceive(t, r, peer, 65001, 1, family.IPv4Unicast, []byte{8, 10}, attrs, false)
	raw := flowspecNLRI(16, 10, 1)
	flowValidationReceive(t, r, peer, 65001, 2, flowspecFamily, raw, attrs, false)
	specific := []byte{24, 10, 1, 1}
	flowValidationReceive(t, r, peer, 65001, 3, family.IPv4Unicast, specific, attrs, false)
	// A longer AS_PATH loses unicast selection regardless of the peer tie-break.
	flowValidationReceive(t, r, foreign, 65002, 4, family.IPv4Unicast, specific,
		flowValidationAttrs(flowValidationPath(2, 65002, 65003), netip.Addr{}, 200, nil), false)
	events := flowValidationEvents(bus)
	if len(events) != 2 || events[0].Withdraw || !events[1].Withdraw {
		t.Fatalf("losing foreign more-specific was ignored: %+v", events)
	}
	flowValidationReceive(t, r, foreign, 65002, 5, family.IPv4Unicast, specific, nil, true)
	events = flowValidationEvents(bus)
	if len(events) != 3 || events[2].Withdraw || !bytes.Equal(events[2].NLRI, raw) {
		t.Fatalf("removing the losing path did not restore authorization: %+v", events)
	}
}

func TestFlowSpecAuthorizationSurvivesRealRPKIDisableAndRefresh(t *testing.T) {
	r, bus := flowValidationFixture(t)
	bridge, ctx := startValidationReceiver(t, bus)
	peer := netip.MustParseAddr("192.0.2.1")
	attrs := flowValidationAttrs(flowValidationPath(2, 65001), netip.Addr{}, 0,
		[]byte{0x80, 6, 0, 0, 0, 0, 0, 0})
	deliver := func(id uint64, fam family.Family, raw []byte) {
		t.Helper()
		event := flowValidationReceive(t, r, peer, 65001, id, fam, raw, attrs, false)
		if err := bridge.DeliverStructured([]any{event}); err != nil {
			t.Fatal(err)
		}
	}
	deliver(10, family.IPv4Unicast, []byte{24, 203, 0, 113})
	validationDecision(t, bridge, ctx, false, "a", 10)
	good, bad := flowspecNLRI(25, 203, 0, 113, 0), flowspecNLRI(25, 203, 0, 114, 0)
	deliver(11, flowspecFamily, good)
	deliver(12, flowspecFamily, bad)
	goodKey := ribevents.ValidationRoute{Peer: peer, Family: flowspecFamily, NLRI: string(good)}
	badKey := ribevents.ValidationRoute{Peer: peer, Family: flowspecFamily, NLRI: string(bad)}
	check := func(eligible bool, count int) {
		t.Helper()
		if ribevents.RouteEligible(goodKey, 11) != eligible || ribevents.RouteEligible(badKey, 12) {
			t.Fatal("optional validation lifecycle bypassed or replaced mandatory FlowSpec export authorization")
		}
		if (len(r.gatherCandidates(flowspecFamily, good)) != 0) != eligible || len(r.gatherCandidates(flowspecFamily, bad)) != 0 {
			t.Fatal("candidate selection diverged from mandatory FlowSpec authorization")
		}
		for key, id := range map[ribevents.ValidationRoute]uint64{goodKey: 11, badKey: 12} {
			path, present := ribevents.LookupFlowSpecPath(key)
			if !present || path.MsgID != id {
				t.Fatalf("validation lifecycle lost retained generation %d: %+v/%v", id, path, present)
			}
		}
		events := flowValidationEvents(bus)
		if len(events) != count {
			t.Fatalf("events = %d, want %d", len(events), count)
		}
		for i, event := range events {
			if !bytes.Equal(event.NLRI, good) || event.Withdraw != (i%2 == 1) {
				t.Fatalf("event %d crossed the authorization boundary: %+v", i, event)
			}
		}
	}
	check(true, 1)
	validationDecision(t, bridge, ctx, false, "i", 10)
	check(false, 2)
	if _, err := bridge.ExecuteCommand(ctx, "disable", "request bgp adj-rib-in disable-validation", nil, ""); err != nil {
		t.Fatal(err)
	}
	check(true, 3)
	if _, err := bridge.ExecuteCommand(ctx, "refresh", "request bgp adj-rib-in enable-validation", []string{"refresh"}, ""); err != nil {
		t.Fatal(err)
	}
	check(false, 4)
	validationDecision(t, bridge, ctx, false, "a", 10)
	check(true, 5)
}

// RFC requirement: RFC8955-4-4 positive -- the real received RIB producer selects identical authorized FlowSpec rules and actions regardless of the advertised MP_REACH next-hop bytes.
// RFC requirement: RFC8955-4-4 negative -- a self next hop or non-address next-hop bytes cannot exclude the authorized FlowSpec candidate or replace its selected actions.
func TestRFC8955NextHopIgnoredForFlowSpec(t *testing.T) {
	r, bus := flowValidationFixture(t)
	peer := netip.MustParseAddr("192.0.2.1")
	originAttrs := flowValidationAttrs(flowValidationPath(2, 65001), netip.Addr{}, 0, nil)
	flowValidationReceive(t, r, peer, 65001, 1, family.IPv4Unicast, []byte{8, 10}, originAttrs, false)
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	if err != nil {
		t.Fatal(err)
	}
	raw := flowspecNLRI(24, 10, 0, 0)
	actions := []byte{0x80, 6, 0, 0, 0, 0, 0, 0}
	for i, nextHop := range [][]byte{nil, {192, 0, 2, 100}, {0xde, 0xad, 0xbe}} {
		attrs := flowValidationAttrs(flowValidationPath(2, 65001), netip.Addr{}, 100, actions)
		// The legacy attribute belongs to other routes in a mixed UPDATE.
		// It must not make this FlowSpec candidate self-referential.
		attrs = append(attrs, 0x40, 3, 4, 192, 0, 2, 100)
		mp := append([]byte{0, 1, byte(family.SAFIFlowSpec), byte(len(nextHop))}, nextHop...)
		mp = append(mp, 0)
		mp = append(mp, raw...)
		attrs = append(attrs, 0x80, 14, byte(len(mp)))
		attrs = append(attrs, mp...)
		body := binary.BigEndian.AppendUint16([]byte{0, 0}, uint16(len(attrs)))
		body = append(body, attrs...)
		wu := wireu.NewWireUpdate(body, ctxID)
		aw, err := wu.Attrs()
		if err != nil {
			t.Fatal(err)
		}
		msgID := uint64(i + 10)
		r.handleReceivedStructured(&rpc.StructuredEvent{
			EventType: rpc.EventKindUpdate, PeerAddress: peer.String(), PeerAS: 65001, LocalAS: 65000,
			LocalAddress: "192.0.2.100", RemoteRouterID: 0x01010101,
			RawMessage: &bgptypes.RawMessage{Type: msgtype.TypeUPDATE, MessageID: msgID, RawBytes: body, WireUpdate: wu, AttrsWire: aw},
		})
		key := ribevents.ValidationRoute{Peer: peer, Family: flowspecFamily, NLRI: string(raw)}
		if !ribevents.RouteEligible(key, msgID) || len(r.gatherCandidates(flowspecFamily, raw)) != 1 {
			t.Fatalf("ignored next hop %x excluded the authorized candidate", nextHop)
		}
		events := flowValidationEvents(bus)
		if len(events) != 1 || events[0].Withdraw || !bytes.Equal(events[0].NLRI, raw) || !bytes.Equal(events[0].ExtendedCommunities, actions) {
			t.Fatalf("ignored next hop %x changed the selected rule/actions: %+v", nextHop, events)
		}
	}
}

// Short and extended length fields frame the same rule. A replacement changes
// its actions once, and a withdrawal using either form removes that winner.
func TestFlowSpecLengthEncodingSharesRetainedGeneration(t *testing.T) {
	r, bus := flowValidationFixture(t)
	peer := netip.MustParseAddr("192.0.2.1")
	path := flowValidationPath(2, 65001)
	flowValidationReceive(t, r, peer, 65001, 1, family.IPv4Unicast, []byte{8, 10},
		flowValidationAttrs(path, netip.Addr{}, 0, nil), false)
	short := flowspecNLRI(24, 10, 0, 0)
	extended := append([]byte{0xf0}, short...)
	drop := []byte{0x80, 6, 0, 0, 0, 0, 0, 0}
	mark := []byte{0x80, 9, 0, 0, 0, 0, 0, 46}
	flowValidationReceive(t, r, peer, 65001, 2, flowspecFamily, short,
		flowValidationAttrs(path, netip.Addr{}, 100, drop), false)
	flowValidationReceive(t, r, peer, 65001, 3, flowspecFamily, extended,
		flowValidationAttrs(path, netip.Addr{}, 100, mark), false)
	events := flowValidationEvents(bus)
	if len(events) != 2 || events[1].Withdraw || !bytes.Equal(events[1].NLRI, short) ||
		!bytes.Equal(events[1].ExtendedCommunities, mark) {
		t.Fatalf("equivalent framing split the selected rule: %+v", events)
	}
	key := ribevents.ValidationRoute{Peer: peer, Family: flowspecFamily, NLRI: string(short)}
	if !ribevents.RouteEligible(key, 3) || ribevents.RouteEligible(key, 2) {
		t.Fatal("equivalent framing split generation eligibility")
	}
	current, present := ribevents.LookupFlowSpecPath(key)
	if !present || current.MsgID != 3 {
		t.Fatalf("canonical replay lost the current received path: %+v/%v", current, present)
	}
	flowValidationReceive(t, r, peer, 65001, 4, flowspecFamily, short, nil, true)
	events = flowValidationEvents(bus)
	if len(events) != 3 || !events[2].Withdraw || !bytes.Equal(events[2].NLRI, short) {
		t.Fatalf("short-form withdrawal left the extended-form rule installed: %+v", events)
	}
	if ribevents.RoutePresent(key) {
		t.Fatal("withdrawal retained an equivalent rule identity")
	}
}

func TestFlowSpecIPv6ActionReplacementAndReplay(t *testing.T) {
	r, bus := flowValidationFixture(t)
	peer := netip.MustParseAddr("192.0.2.1")
	raw := flowspecNLRI(24, 10, 0, 0)
	path := flowValidationPath(2, 65001)
	flowValidationReceive(t, r, peer, 65001, 1, family.IPv4Unicast, []byte{8, 10},
		flowValidationAttrs(path, netip.Addr{}, 0, nil), false)
	redirect, err := attribute.FlowSpecRedirectToIPv6(netip.MustParseAddr("2001:db8::1"))
	if err != nil {
		t.Fatal(err)
	}
	attrs := flowValidationAttrs(path, netip.Addr{}, 100, nil)
	attrs = append(attrs, 0xc0, byte(attribute.AttrIPv6ExtCommunity), byte(len(redirect)))
	attrs = append(attrs, redirect[:]...)
	flowValidationReceive(t, r, peer, 65001, 2, flowspecFamily, raw, attrs, false)
	events := flowValidationEvents(bus)
	if len(events) != 1 || events[0].Withdraw || !bytes.Equal(events[0].IPv6ExtendedCommunities, redirect[:]) {
		t.Fatalf("selected IPv6 redirect was lost: %+v", events)
	}

	// Consumers own event bytes; changing them must not corrupt replay.
	events[0].IPv6ExtendedCommunities[2] ^= 1
	if _, err := ribevents.ReplayRequest.Emit(bus, &replay.Request{ReplayID: replay.Broadcast}); err != nil {
		t.Fatal(err)
	}
	events = flowValidationEvents(bus)
	if len(events) != 2 || !bytes.Equal(events[1].IPv6ExtendedCommunities, redirect[:]) {
		t.Fatalf("replay lost the retained IPv6 action: %+v", events)
	}

	replacement := redirect
	replacement[17] = 2
	copy(attrs[len(attrs)-len(replacement):], replacement[:])
	flowValidationReceive(t, r, peer, 65001, 3, flowspecFamily, raw, attrs, false)
	events = flowValidationEvents(bus)
	if len(events) != 3 || events[2].Withdraw || !bytes.Equal(events[2].IPv6ExtendedCommunities, replacement[:]) {
		t.Fatalf("IPv6-action-only replacement did not publish: %+v", events)
	}
	flowValidationReceive(t, r, peer, 65001, 4, flowspecFamily, raw, nil, true)
	events = flowValidationEvents(bus)
	if len(events) != 4 || !events[3].Withdraw || !bytes.Equal(events[3].NLRI, raw) {
		t.Fatalf("IPv6-action rule was not withdrawn: %+v", events)
	}
}
