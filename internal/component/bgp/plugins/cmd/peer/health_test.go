package peer

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestBgpHealthRowNamesKnownASOnly proves that a `show bgp health` row carries
// the peer's AS as a number when it is known, and no `as` at all for a dynamic
// peer whose OPEN has not arrived. Method: encode the rows as JSON, the bytes an
// operator reads, so a RemoteAS written into the row as a struct (which encodes
// as `{}`) cannot pass.
func TestBgpHealthRowNamesKnownASOnly(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: rpc.RemoteASOf(65001), State: plugin.PeerStateEstablished},
			{Address: netip.MustParseAddr("192.0.2.2"), State: plugin.PeerStateActive},
		},
	}

	resp, err := handleShowBGPHealth(newTestContext(reactor), commandtest.Args())
	require.NoError(t, err)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	rows, ok := data[fieldPeers].([]map[string]any)
	require.True(t, ok)
	require.Len(t, rows, 2)

	known, err := json.Marshal(rows[0]["as"])
	require.NoError(t, err)
	assert.Equal(t, "65001", string(known), "a known AS is written as its number")

	_, named := rows[1]["as"]
	assert.False(t, named, "a peer whose AS is not yet known carries no as, never AS 0 or {}")
}
