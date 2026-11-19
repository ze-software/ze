// Design: docs/architecture/api/process-protocol.md -- physical Session provenance on structured events.
package server

import (
	"github.com/stretchr/testify/require"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"testing"
)

// MUTATION: omit ConnectionID in either structured event constructor.
func TestStructuredPhysicalConnectionDoesNotGrantSelectedOutput(t *testing.T) {
	peer := plugin.PeerInfo{PeerOwner: 81, ConnectionID: ^uint64(0)}
	event := getStructuredEvent(&peer, &bgptypes.RawMessage{})
	require.Equal(t, peer.ConnectionID, event.ConnectionID)
	require.False(t, event.SelectedDestination)
	rpc.PutStructuredEvent(event)
	for _, state := range []rpc.SessionState{rpc.SessionStateUp, rpc.SessionStateDown} {
		event = getStructuredStateEvent(&peer, state, rpc.ReasonTCPFailure, nil)
		require.Equal(t, peer.ConnectionID, event.ConnectionID)
		require.False(t, event.SelectedDestination)
		require.Zero(t, event.DestinationSession)
		rpc.PutStructuredEvent(event)
	}
	reused := rpc.GetStructuredEvent()
	require.Zero(t, reused.ConnectionID, "pooled event resets physical provenance")
	rpc.PutStructuredEvent(reused)
}
