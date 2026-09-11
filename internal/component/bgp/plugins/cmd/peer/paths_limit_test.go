// Design: the operator-facing half of PATHS-LIMIT, capability 76
// (draft-abraitis-idr-addpath-paths-limit). The number that silently caps what
// Ze announces to a peer must be readable from `show bgp peer detail` and
// `show bgp peer capabilities`, in both directions, from the negotiated state.
//
// VALIDATES: addPathsLimitFields writes capabilities.paths-limit for both
//            directions, and writes nothing when there is no limit.
// PREVENTS:  a family that negotiated ADD-PATH and no limit rendering as a
//            limit of 0, which reads as "announce nothing" and is the opposite
//            of what an absent limit means.

package peer

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/family"
)

// pathsLimitReactor answers one established peer whose negotiation completed
// with the ADD-PATH modes and PATHS-LIMIT maps a case needs. The same peer
// feeds both inspection commands, so the two surfaces are read against one
// negotiated state.
func pathsLimitReactor(addPath map[string]string, send, receive map[string]uint16) *mockReactor {
	return &mockReactor{
		peers: []plugin.PeerInfo{{
			Address:                     netip.MustParseAddr("192.0.2.1"),
			PeerAS:                      65001,
			LocalAS:                     65000,
			State:                       plugin.PeerStateEstablished,
			NegotiationComplete:         true,
			NegotiatedFamilies:          []family.Family{family.IPv4Unicast},
			NegotiatedAddPath:           addPath,
			NegotiatedPathsLimitSend:    send,
			NegotiatedPathsLimitReceive: receive,
		}},
		peerCaps: &plugin.PeerCapabilitiesInfo{
			Families:          []string{"ipv4/unicast"},
			AddPath:           addPath,
			PathsLimitSend:    send,
			PathsLimitReceive: receive,
		},
	}
}

// TestPeerDetailShowsBothPathsLimitDirections proves an operator reads both
// limits and can tell them apart. `send` is the peer's advertised limit, which
// the draft's Section 3 makes the cap on what Ze announces to it: "A sender
// advertising multiple paths for the same prefix SHOULD send only the specified
// maximum number of paths indicated in the PATHS-LIMIT capability." `receive`
// is Ze's own advertised limit, the request the peer is asked to respect.
// The two carry different values here, so a swapped direction fails the test.
func TestPeerDetailShowsBothPathsLimitDirections(t *testing.T) {
	reactor := pathsLimitReactor(
		map[string]string{"ipv4/unicast": "both"},
		map[string]uint16{"ipv4/unicast": 4},
		map[string]uint16{"ipv4/unicast": 8},
	)

	payload := marshalPeerDetail(t, reactor)
	want := `"paths-limit":{"receive":{"ipv4/unicast":8},"send":{"ipv4/unicast":4}}`
	require.True(t, contains(payload, want),
		"peer detail = %s, want %s: the peer's limit under send, ours under receive", payload, want)
}

// TestPeerDetailOmitsPathsLimitWhenAddPathCarriesNoLimit is the case the draft
// makes normal: ADD-PATH negotiated, PATHS-LIMIT absent, which means no limit.
// Writing a zero here would publish "send no paths" for a session that has no
// limit at all (ai/rules/principles.md).
func TestPeerDetailOmitsPathsLimitWhenAddPathCarriesNoLimit(t *testing.T) {
	reactor := pathsLimitReactor(map[string]string{"ipv4/unicast": "both"}, nil, nil)

	payload := marshalPeerDetail(t, reactor)
	require.True(t, contains(payload, `"add-path":{"ipv4/unicast":"both"}`),
		"peer detail = %s, want the negotiated ADD-PATH mode", payload)
	require.False(t, contains(payload, "paths-limit"),
		"peer detail = %s, want no paths-limit key: no limit is not a limit of zero", payload)
}

// TestPeerDetailOmitsPathsLimitWithoutAddPath covers the peer that negotiated
// neither. The key is absent, so the answer for a session that never discussed
// paths reads the same as it did before capability 76 existed.
func TestPeerDetailOmitsPathsLimitWithoutAddPath(t *testing.T) {
	reactor := pathsLimitReactor(nil, nil, nil)

	payload := marshalPeerDetail(t, reactor)
	require.False(t, contains(payload, "paths-limit"),
		"peer detail = %s, want no paths-limit key", payload)
}

// TestPeerCapabilitiesShowsBothPathsLimitDirections reads the second surface.
// An operator asking what was negotiated goes to `show bgp peer capabilities`,
// so the limit is owed there as well as on the detail row.
func TestPeerCapabilitiesShowsBothPathsLimitDirections(t *testing.T) {
	reactor := pathsLimitReactor(
		map[string]string{"ipv4/unicast": "both"},
		map[string]uint16{"ipv4/unicast": 4},
		map[string]uint16{"ipv4/unicast": 8},
	)
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	response, err := handleBgpPeerCapabilities(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status)

	negotiated, isMap := firstPeerRow(t, response)["negotiated"].(map[string]any)
	require.True(t, isMap, "the negotiated block is not a map")
	limits, hasLimits := negotiated["paths-limit"].(map[string]any)
	require.True(t, hasLimits, "the negotiated block holds no paths-limit: %#v", negotiated)
	require.Equal(t, map[string]uint16{"ipv4/unicast": 4}, limits["send"],
		"send carries the peer's advertised limit, which caps what Ze announces to it")
	require.Equal(t, map[string]uint16{"ipv4/unicast": 8}, limits["receive"],
		"receive carries Ze's own advertised limit, the request the peer is asked to respect")
}

// TestPeerCapabilitiesOmitsPathsLimitWhenAddPathCarriesNoLimit is the same
// distinction on the capabilities surface: a negotiated ADD-PATH with no limit
// carries no key rather than a zero.
func TestPeerCapabilitiesOmitsPathsLimitWhenAddPathCarriesNoLimit(t *testing.T) {
	reactor := pathsLimitReactor(map[string]string{"ipv4/unicast": "both"}, nil, nil)
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	response, err := handleBgpPeerCapabilities(ctx, nil)
	require.NoError(t, err)

	negotiated, isMap := firstPeerRow(t, response)["negotiated"].(map[string]any)
	require.True(t, isMap, "the negotiated block is not a map")
	_, hasLimits := negotiated["paths-limit"]
	require.False(t, hasLimits, "the negotiated block holds a paths-limit key: %#v", negotiated)
}
