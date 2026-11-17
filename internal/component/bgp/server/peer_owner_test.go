// Design: docs/architecture/api/process-protocol.md -- typed Peer incarnation.
package server

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestStructuredEventsKeepEventPeerOwnerSeparate reaches typed event creation
// and pool return with a destination owner distinct from the original source.
func TestStructuredEventsKeepEventPeerOwnerSeparate(t *testing.T) {
	peer := plugin.PeerInfo{PeerOwner: 701, Address: netip.MustParseAddr("192.0.2.1")}
	message := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, Direction: rpc.DirectionSent, SourceOwner: 702}
	event := getStructuredEvent(&peer, &message)
	require.Equal(t, peer.PeerOwner, event.PeerOwner)
	require.Equal(t, uint64(702), event.RawMessage.(*bgptypes.RawMessage).SourceOwner)
	rpc.PutStructuredEvent(event)
	event = getStructuredStateEvent(&peer, rpc.SessionStateUp, "", nil)
	require.Equal(t, peer.PeerOwner, event.PeerOwner)
	rpc.PutStructuredEvent(event)
	event = rpc.GetStructuredEvent()
	require.Zero(t, event.PeerOwner, "a pooled recipient must not retain the previous Peer incarnation")
	rpc.PutStructuredEvent(event)
}
