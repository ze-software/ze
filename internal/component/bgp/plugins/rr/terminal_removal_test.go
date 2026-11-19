// Design: docs/architecture/plugin/rib-storage-design.md -- original source terminal removal.
// Related: rr.go -- typed and native text state events share the actor handler.
package rr

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRRConnectionlessTerminalRemoval proves that both state rails accept the
// reactor's owner-terminal DOWN before a physical session exists. An ordinary
// DOWN or any UP still needs a physical connection, and another owner at the
// same address survives a valid terminal removal.
func TestRRConnectionlessTerminalRemoval(t *testing.T) {
	for _, transport := range []struct {
		name string
		feed func(*routeReflector, plugin.PeerInfo, rpc.SessionState, string) error
	}{
		{"structured", func(rr *routeReflector, peer plugin.PeerInfo, state rpc.SessionState, reason string) error {
			return rr.handleStructuredState(&rpc.StructuredEvent{
				EventType: rpc.EventKindState, PeerAddress: peer.Address.String(),
				PeerOwner: peer.PeerOwner, ConnectionID: peer.ConnectionID,
				DestinationSession: peer.DestinationSession, SessionGeneration: peer.SessionGeneration,
				State: state, Reason: reason,
			})
		}},
		{"native text", func(rr *routeReflector, peer plugin.PeerInfo, state rpc.SessionState, reason string) error {
			text := format.AppendStateChange(nil, &peer, state, reason, nil, plugin.EncodingText)
			return rr.dispatchText(string(text))
		}},
	} {
		t.Run(transport.name, func(t *testing.T) {
			rr := &routeReflector{}
			t.Cleanup(rr.stopDepartures)
			original := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: rpc.RemoteASOf(65001),
				PeerOwner: 701, ConnectionID: 41, DestinationSession: 141, SessionGeneration: 1041}
			sibling := original
			sibling.PeerOwner = 702
			sibling.ConnectionID = 42
			sibling.DestinationSession = 142
			sibling.SessionGeneration = 1042
			for _, peer := range []plugin.PeerInfo{original, sibling} {
				require.NoError(t, transport.feed(rr, peer, rpc.SessionStateUp, rpc.ReasonAttached))
			}
			originalActor, err := newPeerIdentity(original.Address.String(), original.PeerOwner, original.ConnectionID)
			require.NoError(t, err)
			siblingActor, err := newPeerIdentity(sibling.Address.String(), sibling.PeerOwner, sibling.ConnectionID)
			require.NoError(t, err)

			removed := original
			removed.ConnectionID = 0
			removed.DestinationSession = 0
			require.ErrorContains(t, transport.feed(rr, removed, rpc.SessionStateDown, rpc.ReasonTCPFailure), "no original physical connection")
			require.ErrorContains(t, transport.feed(rr, removed, rpc.SessionStateUp, rpc.ReasonPeerRemoved), "no original physical connection")
			require.True(t, rr.peers[originalActor].Up, "invalid state events cannot end an owner")

			require.NoError(t, transport.feed(rr, removed, rpc.SessionStateDown, rpc.ReasonPeerRemoved))
			settled, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			require.NoError(t, rr.awaitDepartures(settled, original.PeerOwner))
			require.NotContains(t, rr.peers, originalActor)
			require.True(t, rr.peers[siblingActor].Up, "terminal removal cannot close another owner at the same address")
		})
	}
}
