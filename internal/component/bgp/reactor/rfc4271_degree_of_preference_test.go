// Design: docs/architecture/bgp/structural-forwarding.md -- what a forwarded route carries
// RFC: rfc/short/rfc4271.md
// Related: reactor_notify.go -- notifyMessageReceiver, the ingress loop and the RIB dispatch
// Related: filter_ordered.go -- runIngressPolicyChain, where an import policy computes the preference
// Related: forward_local_pref.go -- applyFactsLocalPref, the LOCAL_PREF a forwarded route leaves with
// Related: rfc4271_local_pref_readvertise_test.go -- the Section 9.1.1 half of the same composition
//
// RFC 4271 Section 5.1.5: "A BGP speaker SHALL calculate the degree of
// preference for each external route based on the locally-configured policy,
// and include the degree of preference when advertising a route to its internal
// peers." Ze calculates it in the external peer's import policy chain, before the
// UPDATE is dispatched to the RIB plugin, and the dispatched payload is the one
// the forward rails then send. These tests receive one external route through
// notifyMessageReceiver and follow the payload the RIB was handed to the wire of
// an internal and an external peer.
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

// receiveExternalRoute receives a2InlinePayload (no LOCAL_PREF, as an external
// route arrives) from the external peer 203.0.113.9 (AS 65001) through
// notifyMessageReceiver and answers the one UPDATE body dispatched to the RIB
// plugin. With policy, the peer's import policy answers "local-preference 200"
// (modifyingFilter); without, the peer has no import filter.
func receiveExternalRoute(t *testing.T, policy bool) []byte {
	t.Helper()
	addr := netip.MustParseAddr("203.0.113.9")
	settings := &PeerSettings{
		Connection:    ConnectionBoth,
		Address:       addr,
		LocalAS:       65000,
		GlobalLocalAS: 65000,
		PeerAS:        65001,
		RouterID:      0xcb007109,
	}
	if policy {
		settings.ImportFilters = []filterapi.FilterRef{{Name: "set-local-pref"}}
	}
	source := NewPeer(settings)
	source.state.Store(int32(PeerStateEstablished))

	var dispatched [][]byte
	r := &Reactor{
		clock:           source.clock,
		config:          &Config{LocalAS: 65000},
		peers:           map[netip.AddrPort]*Peer{source.Settings().PeerKey(): source},
		recentUpdates:   newRecentUpdateCache(100),
		attrModHandlers: attrModHandlersWithDefaults(),
		messageReceiver: &testDeliveryReceiver{consumerCount: 1, onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
			dispatched = append(dispatched, append([]byte(nil), msg.RawBytes...))
		}},
		api:                 &pluginserver.Server{}, // non-nil: past the fail-closed r.api guard
		policyFilterSeam:    modifyingFilter(),
		orderedIngressSteps: []orderedIngressStep{{name: policyChainStepName, policyChain: true}},
	}

	body := a2InlinePayload()
	_, received := bodyPathAttr(t, body, 5)
	require.False(t, received, "the external route arrives with no LOCAL_PREF")
	wu := wireu.NewWireUpdate(body, 0)
	wu.SetSourceID(source.SourceID())
	r.notifyMessageReceiver(addr, msgtype.TypeUPDATE, body, wu, 0, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0)

	require.Len(t, dispatched, 1, "the external route is dispatched to the RIB plugin")
	return dispatched[0]
}

// TestRFC4271ExternalRoutePreferenceFromPolicyReachesRIBAndInternalPeers drives
// both clauses of the Section 5.1.5 sentence with a configured policy.
//
// VALIDATES: the UPDATE the RIB plugin is handed for the external route carries
// LOCAL_PREF 200, the degree of preference the locally configured import policy
// calculated, and forwarding that same payload sends LOCAL_PREF 200 to an
// internal peer and no LOCAL_PREF to an external peer.
// PREVENTS: an external route ranked or readvertised on a preference other than
// the one local policy calculated.
//
// RFC requirement: RFC4271-5.1.5-5 positive -- an external route (received with no LOCAL_PREF) whose import policy calculates the degree of preference 200 is dispatched to the RIB plugin carrying LOCAL_PREF 200, and the dispatched payload forwarded on the general rail reaches an internal peer with LOCAL_PREF 200.
func TestRFC4271ExternalRoutePreferenceFromPolicyReachesRIBAndInternalPeers(t *testing.T) {
	payload := receiveExternalRoute(t, true)
	pref, ok := bodyPathAttr(t, payload, 5)
	require.True(t, ok, "the RIB is handed the calculated degree of preference")
	require.Equal(t, []byte{0, 0, 0, 200}, pref, "the RIB is handed the policy's degree of preference")

	internalAddr := netip.MustParseAddr("192.0.2.71")
	externalAddr := netip.MustParseAddr("192.0.2.72")
	got := a2Forward(t, false, payload,
		a2Dest(t, internalAddr.String(), 65000, netip.Addr{}, false),
		a2Dest(t, externalAddr.String(), 65002, netip.Addr{}, false))

	toInternal, ok := got[internalAddr]
	require.True(t, ok, "the internal peer is owed the route")
	require.Equal(t, []byte{0, 0, 0, 200}, toInternal.localPref, "the degree of preference is included toward the internal peer")
	toExternal, ok := got[externalAddr]
	require.True(t, ok, "the external peer is owed the route")
	require.Nil(t, toExternal.localPref, "no LOCAL_PREF toward an external peer")
}

// TestRFC4271ExternalRouteWithoutPolicyStillAdvertisesAPreferenceInternally
// forces the input toward the violation: no configured policy and no received
// LOCAL_PREF, so nothing on the route names a degree of preference.
//
// VALIDATES: the RIB is handed the route with no LOCAL_PREF (the policy
// calculated none, so the RIB applies its configured default), and the internal
// peer is still sent a degree of preference, the default 100, never the 200 the
// policy would have calculated.
// PREVENTS: an internal advertisement that omits the degree of preference when
// local policy set none, and a constant 200 standing in for the policy.
//
// RFC requirement: RFC4271-5.1.5-5 negative -- an external route received with no LOCAL_PREF and no import policy is dispatched to the RIB plugin with no LOCAL_PREF (never 200), and is still advertised to an internal peer with a LOCAL_PREF, the default 100: the internal advertisement never omits the degree of preference.
func TestRFC4271ExternalRouteWithoutPolicyStillAdvertisesAPreferenceInternally(t *testing.T) {
	payload := receiveExternalRoute(t, false)
	_, ok := bodyPathAttr(t, payload, 5)
	require.False(t, ok, "no policy calculated a preference, so the RIB is handed none")

	internalAddr := netip.MustParseAddr("192.0.2.71")
	got := a2Forward(t, false, payload, a2Dest(t, internalAddr.String(), 65000, netip.Addr{}, false))

	toInternal, ok := got[internalAddr]
	require.True(t, ok, "the internal peer is owed the route")
	require.Equal(t, []byte{0, 0, 0, 100}, toInternal.localPref, "the internal advertisement carries the default degree of preference")
}
