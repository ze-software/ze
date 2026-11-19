// Design: docs/architecture/api/process-protocol.md -- structured SENT destination provenance.
package server

import (
	"testing"

	"github.com/stretchr/testify/require"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The actual structured event constructor preserves SENT provenance and keeps
// an ordinary received UPDATE unqualified. Neither projection grants output.
// MUTATION: restrict DestinationSession projection to ROUTEREFRESH only.
func TestStructuredSentKeepsCapturedDestinationSession(t *testing.T) {
	peer := plugin.PeerInfo{PeerOwner: 81, DestinationSession: 91}
	sent := getStructuredEvent(&peer, &bgptypes.RawMessage{Type: msgtype.TypeUPDATE, Direction: rpc.DirectionSent})
	require.Equal(t, uint64(91), sent.DestinationSession)
	require.False(t, sent.SelectedDestination)
	rpc.PutStructuredEvent(sent)
	received := getStructuredEvent(&peer, &bgptypes.RawMessage{Type: msgtype.TypeUPDATE, Direction: rpc.DirectionReceived})
	require.Zero(t, received.DestinationSession, "ingress is not output ownership")
	rpc.PutStructuredEvent(received)
}
