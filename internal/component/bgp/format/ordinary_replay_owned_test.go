// Design: docs/architecture/api/process-protocol.md -- advisory replay ownership.
package format

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestStateCarriesOrdinaryReplayAdvisory(t *testing.T) {
	for _, owned := range []bool{false, true} {
		peer := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.48"), OrdinaryReplayOwned: owned}
		text := string(AppendStateChange(nil, &peer, rpc.SessionStateUp, "", nil, plugin.EncodingText))
		require.Equal(t, owned, strings.Contains(text, " ordinary-replay-owned true"))
		require.NotContains(t, text, " selected-destination true")
		data := AppendStateChange(nil, &peer, rpc.SessionStateUp, "", nil, plugin.EncodingJSON)
		var event struct {
			BGP struct {
				OrdinaryReplayOwned bool `json:"ordinary-replay-owned"`
				SelectedDestination bool `json:"selected-destination"`
			} `json:"bgp"`
		}
		require.NoError(t, json.Unmarshal(data, &event))
		require.Equal(t, owned, event.BGP.OrdinaryReplayOwned)
		require.False(t, event.BGP.SelectedDestination)
	}
}
