// VALIDATES: AC-15 of spec-path-mtu-diagnostic at the payload: the child-sa object of
// show vpn ipsec sa carries the negotiated transform and the three installed facts
// that size an ESP packet
// PREVENTS: an operator reading the first configured proposal as the running one
package cmd

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/engine"
)

// TestSAToMapChildCarriesInstalledFacts proves the payload renders PeerInfo's
// installed fields under child-sa, with the mode as its name, the encapsulation as a
// bool and the endpoint as text, and that the transform keys carry whatever Info
// reported (the negotiated proposal, proven in engine.TestPeerInfoReportsNegotiatedTransform).
func TestSAToMapChildCarriesInstalledFacts(t *testing.T) {
	peers := map[string]engine.PeerInfo{
		"peer-alpha": {
			PeerName:        "peer-alpha",
			HasChild:        true,
			ChildInSPI:      100,
			ChildOutSPI:     200,
			ChildRemoteAddr: netip.MustParseAddr("198.51.100.7"),
			ChildMode:       dataplane.ModeTransport,
			ChildUDPEncap:   true,
			ESPEncryption:   "aes128gcm",
			ESPIntegrity:    "none",
		},
	}

	row := saToMap(&engine.SA{PeerName: "peer-alpha", State: engine.StateEstablished},
		time.Now(), peers, sadCounters{})

	child, ok := row["child-sa"].(map[string]any)
	require.True(t, ok, "no child-sa object")
	require.Equal(t, "transport", child["mode"])
	require.Equal(t, true, child["udp-encapsulation"])
	require.Equal(t, "198.51.100.7", child["remote-address"])
	require.Equal(t, "aes128gcm", child["esp-encryption"])
	require.Equal(t, "none", child["esp-integrity"])
}

// TestSAToMapChildWithoutEndpointAnswersNull proves an invalid installed address
// renders null rather than the text "invalid IP".
func TestSAToMapChildWithoutEndpointAnswersNull(t *testing.T) {
	peers := map[string]engine.PeerInfo{
		"peer-alpha": {PeerName: "peer-alpha", HasChild: true, ChildMode: dataplane.ModeTunnel},
	}
	row := saToMap(&engine.SA{PeerName: "peer-alpha"}, time.Now(), peers, sadCounters{})
	child, ok := row["child-sa"].(map[string]any)
	require.True(t, ok, "no child-sa object")
	require.Nil(t, child["remote-address"])
	require.Equal(t, "tunnel", child["mode"])
	require.Equal(t, false, child["udp-encapsulation"])
}
