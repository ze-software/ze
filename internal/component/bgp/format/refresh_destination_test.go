// Design: docs/architecture/api/process-protocol.md -- refresh destination facts.
package format

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRefreshDestinationTransports proves parsed/full JSON and text carry the
// same exact receiving Session, independently of initial-update readiness.
func TestRefreshDestinationTransports(t *testing.T) {
	const token = uint64(18446744073709551615)
	for _, selected := range []bool{false, true} {
		peer := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: rpc.RemoteASOf(65001),
			DestinationSession: token, SelectedDestination: selected}
		message := bgptypes.RawMessage{Type: msgtype.TypeROUTEREFRESH, RawBytes: []byte{0, 1, 0, 1},
			Direction: rpc.DirectionReceived, MessageID: 42}
		for _, form := range []string{plugin.FormatParsed, plugin.FormatFull} {
			encoded := NewJSONEncoder("").Message(&peer, message, form)
			event, err := bgp.ParseEvent([]byte(encoded))
			require.NoError(t, err)
			require.Equal(t, token, event.DestinationSession)
			require.Equal(t, selected, event.SelectedDestination)
			require.Zero(t, event.InitialReplay)
		}
		text := string(AppendRouteRefresh(nil, &peer, DecodeRouteRefresh(message.RawBytes), rpc.DirectionReceived, 42))
		require.Contains(t, text, "destination-session 18446744073709551615")
		if selected {
			require.Contains(t, text, "selected-destination true")
		} else {
			require.NotContains(t, text, "selected-destination true")
		}
	}
}
