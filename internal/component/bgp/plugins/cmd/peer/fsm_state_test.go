// Design: docs/architecture/exabgp-bridge.md -- the ExaBGP bridge's `status`
// lists every peer with its RFC 4271 Section 8.2.2 state, read from
// `show bgp peer * detail`.
//
// VALIDATES: peer detail carries the session's FSM state as fsm-state.
// PREVENTS:  a session in OPENSENT reported only as "connecting", which is all
//            the lifecycle state says, so a reader guesses the FSM state.

package peer

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPeerDetailCarriesTheFSMState proves the detail row names the FSM state
// the reactor reported, beside the lifecycle state, and that the two differ
// while the open exchange runs.
func TestPeerDetailCarriesTheFSMState(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{{
			Address:  netip.MustParseAddr("192.0.2.1"),
			PeerAS:   rpc.RemoteASOf(65001),
			LocalAS:  65000,
			State:    plugin.PeerStateConnecting,
			FSMState: "OPENSENT",
		}},
	}

	payload := marshalPeerDetail(t, reactor)
	require.True(t, contains(payload, `"fsm-state":"OPENSENT"`), "peer detail = %s, want fsm-state OPENSENT", payload)
	require.True(t, contains(payload, `"state":"connecting"`), "peer detail = %s, want state connecting", payload)
}
