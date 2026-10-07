// Design: docs/architecture/bgp/replay-cursor.md -- config owns peer-up, RIB owns refresh.
package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC2918ConfigStaticRetainedForRefresh drives sent-event ingestion and a
// structured refresh, then checks origin survives the resulting sent event.
// RFC requirement: RFC2918-4-3 positive -- a sent config-static IPv4 route is re-advertised on an IPv4 refresh, and excluded only from peer-up replay.
func TestRFC2918ConfigStaticRetainedForRefresh(t *testing.T) {
	r := newTestRIBManager(t)
	peer := netip.MustParseAddr("192.0.2.2")
	r.peerUp[peer] = true
	body := []byte{0, 0, 0, 14, 0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 1, 24, 198, 51, 100}
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, err := wu.Attrs()
	if err != nil {
		t.Fatal(err)
	}
	sent := rpc.StructuredEvent{
		PeerAddress: peer.String(),
		Meta:        map[string]any{"config-static": true},
		RawMessage:  &bgptypes.RawMessage{WireUpdate: wu, AttrsWire: attrs},
	}
	r.handleSentStructured(&sent)
	var commands []string
	r.dispatchHook = func(string) {}
	r.updateHook = func(command string, meta map[string]any) {
		if meta["replay"] != true {
			t.Error("refresh did not request re-advertisement")
		}
		commands = append(commands, command)
	}
	r.handleRefreshStructured(&rpc.StructuredEvent{
		PeerAddress: peer.String(),
		RawMessage:  &bgptypes.RawMessage{RawBytes: []byte{0, 1, 0, 1}},
	})
	var routes []refreshRouteIdentity
	for _, command := range commands {
		groups := consumeRefreshCommand(t, command)
		routes = append(routes, consumedRefreshRoutes(t, groups)...)
		for _, group := range groups {
			require.Equal(t, attrs.Packed(), group.Wire.Packed(),
				"the refresh consumer must retain the configured path attributes")
		}
	}
	require.Equal(t, []refreshRouteIdentity{
		{family: family.IPv4Unicast, prefix: netip.MustParsePrefix("198.51.100.0/24")},
	}, routes, "the real UPDATE parser must receive the configured route")
	if groups := r.collectPeerUpReplay(peer, true); len(groups) != 0 {
		t.Fatalf("config-static replayed on peer-up: %v", groups)
	}
	// The wire feedback from refresh lacks the config marker; origin must survive.
	sent.Meta = map[string]any{"replay": true}
	r.handleSentStructured(&sent)
	if groups := r.collectPeerUpReplay(peer, true); len(groups) != 0 {
		t.Fatalf("refresh changed origin: %v", groups)
	}
	if groups := r.collectGroupedRibOutRoutesForFamily(peer, family.IPv4Unicast); len(groups) != 1 {
		t.Fatalf("refresh lost route: %v", groups)
	}
	// A new plugin advertisement is not a refresh and takes ownership.
	sent.Meta = nil
	sent.RawMessage.(*bgptypes.RawMessage).SourceLocal = true
	r.handleSentStructured(&sent)
	if groups := r.collectPeerUpReplay(peer, true); len(groups) != 1 {
		t.Fatalf("replacement plugin route not replayed: %v", groups)
	}
}

// TestRFC2918RefreshDoesNotSendToDownPeer observes both update and marker rails.
// RFC requirement: RFC2918-4-3 negative -- a refresh from a peer with no established session emits neither routes nor markers, even when Adj-RIB-Out contains a route.
func TestRFC2918RefreshDoesNotSendToDownPeer(t *testing.T) {
	r := newTestRIBManager(t)
	peer := netip.MustParseAddr("192.0.2.2")
	r.ribOut[peer] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv4Unicast: {"198.51.100.0/24": {Prefix: "198.51.100.0/24", NextHop: "192.0.2.1"}},
	})
	r.dispatchHook = func(command string) { t.Errorf("marker sent to down peer: %s", command) }
	r.updateHook = func(command string, _ map[string]any) { t.Errorf("route sent to down peer: %s", command) }
	r.handleRefreshStructured(&rpc.StructuredEvent{
		PeerAddress: peer.String(),
		RawMessage:  &bgptypes.RawMessage{RawBytes: []byte{0, 1, 0, 1}},
	})
}
