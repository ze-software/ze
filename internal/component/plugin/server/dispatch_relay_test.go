package server

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: a relay-stored-route RPC reaches the reactor's ReactorRelayCoordinator
// with the destination parsed and every stored route delivered intact.
// PREVENTS: the primitive being declared but never reaching the forward rail --
// the exact shape of the bug spec-fixit-bgp-egress-rail-divergence exists to fix,
// where a replayed route took a second, divergent egress path.
func TestRelayStoredRouteReachesCoordinator(t *testing.T) {
	t.Parallel()

	reactor := &mockReactor{}
	s := &Server{reactor: reactor}

	routes := []rpc.StoredRoute{
		{
			SourcePeer: "10.0.0.1",
			Family:     "ipv4/unicast",
			AttrHex:    "4001010040020602010000fe0a",
			NextHopHex: "0a000001",
			NLRIHex:    "180a0000",
		},
		{
			SourcePeer: "10.0.0.2",
			Family:     "ipv4/unicast",
			AttrHex:    "4001010040020602010000fe0b",
			NextHopHex: "0a000002",
			NLRIHex:    "180a0001",
		},
	}

	err := s.relayStoredRoute("192.0.2.7", routes)
	require.NoError(t, err)

	require.Len(t, reactor.relayCalls, 1, "the RPC must dispatch exactly one relay call")
	call := reactor.relayCalls[0]
	assert.Equal(t, "192.0.2.7", call.destination.String(), "destination must be parsed at the boundary")
	require.Len(t, call.routes, 2, "every stored route must survive the boundary")

	// The source peer is the field the old "update hex ... add" replay dropped,
	// which is what let the replay and forward rails diverge. Assert it survives.
	assert.Equal(t, "10.0.0.1", call.routes[0].SourcePeer)
	assert.Equal(t, "10.0.0.2", call.routes[1].SourcePeer)
	assert.Equal(t, "180a0000", call.routes[0].NLRIHex)
}

// VALIDATES: an unparseable destination fails closed instead of relaying nothing.
// PREVENTS: a silent no-op replay that is indistinguishable from a successful one
// (ai/rules/evidence.md).
func TestRelayStoredRouteRejectsBadDestination(t *testing.T) {
	t.Parallel()

	reactor := &mockReactor{}
	s := &Server{reactor: reactor}

	routes := []rpc.StoredRoute{{SourcePeer: "10.0.0.1", Family: "ipv4/unicast", NLRIHex: "180a0000"}}

	err := s.relayStoredRoute("not-an-address", routes)

	require.Error(t, err, "a destination that does not parse must be an error, not a dropped entry")
	assert.Contains(t, err.Error(), "not-an-address", "the error must name the offending value")
	assert.Empty(t, reactor.relayCalls, "nothing may be dispatched when the destination is invalid")
}

// VALIDATES: an empty route list is a success no-op that dispatches nothing.
// PREVENTS: a peer-up with no stored routes being reported as a relay failure.
func TestRelayStoredRouteEmptyIsNoOp(t *testing.T) {
	t.Parallel()

	reactor := &mockReactor{}
	s := &Server{reactor: reactor}

	require.NoError(t, s.relayStoredRoute("192.0.2.7", nil))
	assert.Empty(t, reactor.relayCalls)
}

// VALIDATES: the JSON transport decodes RelayStoredRouteInput and drives the same
// handler as the typed DirectBridge path.
// PREVENTS: the two transports drifting, which the engineOp registry exists to stop.
func TestRelayStoredRouteJSONTransport(t *testing.T) {
	t.Parallel()

	reactor := &mockReactor{}
	s := &Server{reactor: reactor}

	params, err := json.Marshal(rpc.RelayStoredRouteInput{
		Destination: "192.0.2.7",
		Routes: []rpc.StoredRoute{
			{SourcePeer: "10.0.0.1", Family: "ipv4/unicast", NLRIHex: "180a0000"},
		},
	})
	require.NoError(t, err)

	result, opErr := s.opRelayStoredRoute(nil, params)
	require.NoError(t, opErr)
	assert.Nil(t, result, "relay-stored-route carries no result payload")

	require.Len(t, reactor.relayCalls, 1)
	assert.Equal(t, "10.0.0.1", reactor.relayCalls[0].routes[0].SourcePeer)
}

// VALIDATES: malformed relay-stored-route params are rejected as an RPC error.
// PREVENTS: a truncated or mistyped payload being treated as an empty replay.
func TestRelayStoredRouteRejectsBadParams(t *testing.T) {
	t.Parallel()

	s := &Server{reactor: &mockReactor{}}

	_, err := s.opRelayStoredRoute(nil, json.RawMessage(`{"destination":`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "relay-stored-route")
}
