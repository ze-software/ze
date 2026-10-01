// Design: docs/architecture/core-design.md -- the ingress filter pass before RIB dispatch
// RFC: rfc/short/rfc4271.md
// Related: reactor_notify.go -- notifyMessageReceiver, the ingress loop and the dispatch
// Related: filter_ordered.go -- runIngressPolicyChain, the configured acceptance policy
// Overview: ../plugins/rib/rfc4271_receive_decision_test.go -- the RIB half: both accepted routes installed
//
// RFC 4271 Section 9.1.4: "If a BGP speaker receives overlapping routes, the
// Decision Process MUST consider both routes based on the configured acceptance
// policy." Ze applies the configured acceptance policy (the peer's import filter
// chain) in the reactor, before the UPDATE is dispatched to the RIB plugin that
// runs the Decision Process. These tests receive two overlapping routes from one
// peer and read what the RIB plugin is handed, so the acceptance policy, and not
// the overlap, is what decides which routes the Decision Process considers.
package reactor

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// overlapLessSpecific and overlapMoreSpecific overlap: the /24 is inside the /8.
var (
	overlapLessSpecific = []byte{8, 10}
	overlapMoreSpecific = []byte{24, 10, 1, 0}
)

// receiveOverlappingRoutes receives the /8 and then the /24 from the external
// peer 203.0.113.9 through notifyMessageReceiver, with an import policy that
// answers decide(updateText) for each UPDATE, and returns the NLRI of every
// UPDATE the reactor dispatched to the RIB plugin, in order.
func receiveOverlappingRoutes(t *testing.T, decide func(updateText string) PolicyResponse) [][]byte {
	t.Helper()
	addr := netip.MustParseAddr("203.0.113.9")
	source := NewPeer(&PeerSettings{
		Connection:    ConnectionBoth,
		Address:       addr,
		LocalAS:       65000,
		GlobalLocalAS: 65000,
		PeerAS:        65001,
		RouterID:      0xcb007109,
		ImportFilters: []filterapi.FilterRef{{Name: "acceptance"}},
	})
	source.state.Store(int32(PeerStateEstablished))

	var dispatched [][]byte
	r := &Reactor{
		clock:           source.clock,
		config:          &Config{LocalAS: 65000},
		peers:           map[netip.AddrPort]*Peer{source.Settings().PeerKey(): source},
		recentUpdates:   newRecentUpdateCache(100),
		attrModHandlers: attrModHandlersWithDefaults(),
		messageReceiver: &testDeliveryReceiver{consumerCount: 1, onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
			require.NotNil(t, msg.WireUpdate)
			nlriBytes, err := msg.WireUpdate.NLRI()
			require.NoError(t, err)
			dispatched = append(dispatched, append([]byte(nil), nlriBytes...))
		}},
		api: &pluginserver.Server{}, // non-nil: past the fail-closed r.api guard
		policyFilterSeam: func(_, _, _, _ string, _ uint32, updateText string) PolicyResponse {
			return decide(updateText)
		},
		orderedIngressSteps: []orderedIngressStep{{name: policyChainStepName, policyChain: true}},
	}

	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN igp
		0x40, 0x02, 0x06, 0x02, 0x01, 0, 0, 0xFD, 0xE9, // AS_PATH [65001]
		0x40, 0x03, 0x04, 203, 0, 113, 9, // NEXT_HOP
	}
	for _, prefix := range [][]byte{overlapLessSpecific, overlapMoreSpecific} {
		body := buildUpdatePayload(attrs, prefix)
		wu := wireu.NewWireUpdate(body, 0)
		wu.SetSourceID(source.SourceID())
		r.notifyMessageReceiver(addr, msgtype.TypeUPDATE, body, wu, 0, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0)
	}
	return dispatched
}

// TestRFC4271OverlappingRoutesAcceptedByPolicyBothReachTheDecisionProcess drives
// two overlapping routes through an acceptance policy that accepts both.
//
// VALIDATES: the /8 and the /24 are each dispatched to the RIB plugin, so the
// overlap removes neither from the Decision Process's input.
// PREVENTS: an overlap check ahead of the RIB that drops the less or the more
// specific route the configured policy accepted.
//
// RFC requirement: RFC4271-9.2-4 positive -- two overlapping routes (10.0.0.0/8 and 10.1.0.0/24) received from one peer and both accepted by its configured import policy are each dispatched to the RIB plugin that runs the Decision Process, in arrival order.
func TestRFC4271OverlappingRoutesAcceptedByPolicyBothReachTheDecisionProcess(t *testing.T) {
	var seen []string
	dispatched := receiveOverlappingRoutes(t, func(updateText string) PolicyResponse {
		seen = append(seen, updateText)
		return PolicyResponse{Action: PolicyAccept}
	})

	require.Len(t, seen, 2, "the acceptance policy is asked about each overlapping route")
	assert.Equal(t, [][]byte{overlapLessSpecific, overlapMoreSpecific}, dispatched,
		"both overlapping routes the policy accepted reach the Decision Process")
}

// TestRFC4271OverlappingRouteRejectedByPolicyIsNotConsidered is the other
// polarity: the policy accepts the /8 and rejects the /24.
//
// VALIDATES: only the /8 is dispatched to the RIB plugin; the /24 the policy
// rejected never reaches the Decision Process, and the /8 is still considered
// although a more specific route overlapping it arrived.
// PREVENTS: a Decision Process fed by the overlap rather than by the configured
// acceptance policy (the rejected route considered anyway, or the accepted one
// displaced by it).
//
// RFC requirement: RFC4271-9.2-4 negative -- when the configured import policy rejects the more specific of two overlapping routes (10.1.0.0/24) and accepts the less specific (10.0.0.0/8), only 10.0.0.0/8 is dispatched to the RIB plugin: the rejected route is not considered by the Decision Process.
func TestRFC4271OverlappingRouteRejectedByPolicyIsNotConsidered(t *testing.T) {
	dispatched := receiveOverlappingRoutes(t, func(updateText string) PolicyResponse {
		if strings.Contains(updateText, "10.1.0.0/24") {
			return PolicyResponse{Action: PolicyReject}
		}
		return PolicyResponse{Action: PolicyAccept}
	})

	assert.Equal(t, [][]byte{overlapLessSpecific}, dispatched,
		"only the route the acceptance policy accepted reaches the Decision Process")
}
