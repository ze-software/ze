// Design: docs/architecture/plugin/rib-storage-design.md -- selecting a session by its name
// Related: rib_peer_name.go -- recordPeerName and selectsPeer

package rib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRIBShowSelectsASessionByItsName proves `show bgp rib received peer
// <name>` and `show bgp rib best peer <name>` answer the routes of the session
// configured under that name.
//
// Method: two sessions, each holding one route, report their names on the
// state event the engine sends when they come up; the show pipeline is then
// asked for each by name, and for a name neither carries.
// VALIDATES: a name selects exactly its own session's routes, a negated name
// every other session's, and a name no session carries none.
// PREVENTS: a name selector reaching bgp-rib, whose sessions are keyed by
// address, and matching nothing, so `show bgp rib received peer injector`
// answered 0 routes for a session holding 1000.
func TestRIBShowSelectsASessionByItsName(t *testing.T) {
	r := newTestRIBManager(t)
	attrs := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001)
	sessions := []struct {
		address string
		name    string
		nlri    []byte
	}{
		{"192.0.2.1", "injector", []byte{24, 10, 0, 1}},
		{"192.0.2.2", "transit", []byte{24, 10, 0, 2}},
	}
	for _, session := range sessions {
		peerRIB := storage.NewPeerRIB(session.address)
		peerRIB.Insert(family.IPv4Unicast, attrs, session.nlri)
		r.bgpPeers[netip.MustParseAddr(session.address)] = peerRIB
		// An attached up records the session with no replay to answer.
		r.handleStructuredState(&rpc.StructuredEvent{
			PeerAddress: session.address,
			PeerName:    session.name,
			State:       rpc.SessionStateUp,
			Reason:      rpc.ReasonAttached,
		})
	}

	tests := []struct {
		args []string
		want int
	}{
		{[]string{"received", "peer", "injector", "count"}, 1},
		{[]string{"received", "peer", "transit", "count"}, 1},
		{[]string{"received", "peer", "!injector", "count"}, 1},
		{[]string{"received", "peer", "nosuch", "count"}, 0},
		{[]string{"received", "peer", "192.0.2.1", "count"}, 1},
	}
	for _, tt := range tests {
		got := r.showPipeline("*", tt.args)
		answer, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("%v: answer = %#v, want a count", tt.args, got)
		}
		if count, _ := answer["count"].(int); count != tt.want {
			t.Errorf("%v: answer = %v, want count %d", tt.args, answer, tt.want)
		}
	}

	best := r.bestPipeline("*", []string{"peer", "injector", "count"})
	answer, ok := best.(map[string]any)
	if !ok {
		t.Fatalf("best peer injector: answer = %#v, want a count", best)
	}
	if count, _ := answer["count"].(int); count != 1 {
		t.Errorf("best peer injector: answer = %v, want count 1", answer)
	}
}
