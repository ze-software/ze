// Design: docs/architecture/core-design.md -- the ingress filter pass before RIB dispatch
// Related: reactor_notify.go -- notifyMessageReceiver, the ingress loop and the dispatch
// Related: filter_ordered.go -- runIngressPolicyChain, the import policy step

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// medTen is the MULTI_EXIT_DISC value the import policy writes.
var medTen = []byte{0, 0, 0, 10}

// TestRFC4271ReceivedMEDAlteredBeforeTheDecisionProcess drives RFC 4271 Section
// 5.1.4: "If a BGP speaker is configured to alter the value of the
// MULTI_EXIT_DISC attribute received over EBGP, then altering the value MUST be
// done prior to determining the degree of preference of the route and prior to
// performing route selection (Decision Process phases 1 and 2)."
//
// Method: an UPDATE carrying MULTI_EXIT_DISC 100 is received from the external
// peer 203.0.113.9 through the reactor's receive entry (notifyMessageReceiver).
// The peer's import policy, the configured alteration, answers "med 10". The
// message receiver stands in for the RIB plugin, which runs phases 1 and 2 on
// what it is handed.
//
// VALIDATES: the UPDATE handed to the RIB plugin carries MULTI_EXIT_DISC 10, in
// its raw bytes and in its wire view, so the decision process never sees the
// received 100.
// PREVENTS: the alteration applied after dispatch, or only to a copy the RIB
// never reads, so routes were preferred on the received MED.
//
// RFC requirement: RFC4271-5.1.4-3 positive -- an import policy that alters a MULTI_EXIT_DISC received over EBGP (100 to 10) has its altered value in the UPDATE the reactor dispatches to the RIB plugin, raw bytes and wire view alike, so the decision process is handed 10 and never 100.
func TestRFC4271ReceivedMEDAlteredBeforeTheDecisionProcess(t *testing.T) {
	addr := netip.MustParseAddr("203.0.113.9")
	source := NewPeer(&PeerSettings{
		Connection:    ConnectionBoth,
		Address:       addr,
		LocalAS:       65000,
		GlobalLocalAS: 65000,
		PeerAS:        65001,
		RouterID:      0xcb007109,
		ImportFilters: []filterapi.FilterRef{{Name: "alter-med"}},
	})
	source.state.Store(int32(PeerStateEstablished))

	var dispatched []bgptypes.RawMessage
	r := &Reactor{
		clock:           source.clock,
		config:          &Config{LocalAS: 65000},
		peers:           map[netip.AddrPort]*Peer{source.Settings().PeerKey(): source},
		recentUpdates:   newRecentUpdateCache(100),
		attrModHandlers: attrModHandlersWithDefaults(),
		messageReceiver: &testDeliveryReceiver{consumerCount: 1, onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
			dispatched = append(dispatched, msg)
		}},
		api: &pluginserver.Server{}, // non-nil: past the fail-closed r.api guard
		policyFilterSeam: func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
			return PolicyResponse{Action: PolicyModify, Delta: "med 10"}
		},
		orderedIngressSteps: []orderedIngressStep{{name: policyChainStepName, policyChain: true}},
	}

	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN igp
		0x40, 0x02, 0x06, 0x02, 0x01, 0, 0, 0xFD, 0xE9, // AS_PATH [65001]
		0x40, 0x03, 0x04, 203, 0, 113, 9, // NEXT_HOP
		0x80, 0x04, 0x04, 0, 0, 0, 100, // MULTI_EXIT_DISC 100
	}
	body := buildUpdatePayload(attrs, []byte{24, 192, 0, 2})
	wu := wireu.NewWireUpdate(body, 0)
	wu.SetSourceID(source.SourceID())

	r.notifyMessageReceiver(addr, msgtype.TypeUPDATE, body, wu, 0, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0)

	require.Len(t, dispatched, 1, "the UPDATE is dispatched to the RIB plugin")
	msg := dispatched[0]
	raw := payloadMED(msg.RawBytes)
	require.True(t, raw.present, "the raw UPDATE the RIB is handed carries a MED")
	require.Equal(t, medTen, raw.raw, "the raw UPDATE the RIB is handed carries the altered MED")
	require.NotNil(t, msg.WireUpdate)
	view := payloadMED(msg.WireUpdate.Payload())
	require.True(t, view.present, "the wire view the RIB is handed carries a MED")
	require.Equal(t, medTen, view.raw, "the wire view the RIB is handed carries the altered MED")
}
