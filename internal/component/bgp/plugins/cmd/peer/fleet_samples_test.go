// Design: docs/architecture/fleet-config.md -- BGP peer fleet sample producer tests
// Related: fleet_samples.go -- hidden read-only BGP peer sample handler

package peer

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	peeryang "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/peer/yang"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/fleet"
)

func decodePeerSourceSnapshot(t *testing.T, resp *plugin.Response) fleet.SourceSnapshot {
	t.Helper()
	require.NotNil(t, resp)
	require.Equal(t, plugin.StatusDone, resp.Status)
	raw, err := json.Marshal(resp.Data)
	require.NoError(t, err)
	var snap fleet.SourceSnapshot
	require.NoError(t, json.Unmarshal(raw, &snap))
	require.NoError(t, fleet.ValidateSourceSnapshot(fleet.SampleOwnerBGPPeer, &snap))
	return snap
}

// VALIDATES: show fleet source bgp-peer returns empty readings when no BGP
// reactor is running (nil context, nil reactor, or bare coordinator without
// BGP), so non-BGP devices omit BGP figures instead of reporting zero.
// PREVENTS: a coordinator without an attached BGP reactor fabricating zero
// BGP peer counts on non-BGP devices.
func TestFleetSourceBGPPeerAbsentWithoutReactor(t *testing.T) {
	t.Run("nil-context", func(t *testing.T) {
		resp, err := handleFleetSourceBGPPeer(nil, commandtest.Args())
		require.NoError(t, err)
		snap := decodePeerSourceSnapshot(t, resp)
		require.Equal(t, fleet.SampleOwnerBGPPeer, snap.Owner)
		require.Empty(t, snap.Readings)
	})

	t.Run("nil-reactor", func(t *testing.T) {
		resp, err := handleFleetSourceBGPPeer(newTestContext(nil), commandtest.Args())
		require.NoError(t, err)
		snap := decodePeerSourceSnapshot(t, resp)
		require.Equal(t, fleet.SampleOwnerBGPPeer, snap.Owner)
		require.Empty(t, snap.Readings)
	})

	t.Run("coordinator-without-bgp", func(t *testing.T) {
		coord := plugin.NewCoordinator(nil)
		resp, err := handleFleetSourceBGPPeer(newTestContext(coord), commandtest.Args())
		require.NoError(t, err)
		snap := decodePeerSourceSnapshot(t, resp)
		require.Equal(t, fleet.SampleOwnerBGPPeer, snap.Owner)
		require.Empty(t, snap.Readings)
	})
}

// VALIDATES: show fleet source bgp-peer reports real zeros when BGP is live
// with zero peers and accurate established/configured counts across peer states.
// PREVENTS: established peer counting from confusing real zeros with absence or
// miscounting non-established peers as established.
func TestFleetSourceBGPPeerCounts(t *testing.T) {
	t.Run("live-zero-peers", func(t *testing.T) {
		resp, err := handleFleetSourceBGPPeer(newTestContext(&mockReactor{}), commandtest.Args())
		require.NoError(t, err)
		snap := decodePeerSourceSnapshot(t, resp)
		require.Equal(t, []fleet.SourceReading{
			{Figure: fleet.FigureBGPPeersEstablished, Unit: fleet.SampleUnitCount, Value: 0},
			{Figure: fleet.FigureBGPPeersConfigured, Unit: fleet.SampleUnitCount, Value: 0},
		}, snap.Readings)
	})

	t.Run("mixed-peer-states", func(t *testing.T) {
		reactor := &mockReactor{
			peers: []plugin.PeerInfo{
				{Address: netip.MustParseAddr("192.0.2.1"), State: plugin.PeerStateEstablished},
				{Address: netip.MustParseAddr("192.0.2.2"), State: plugin.PeerStateStopped},
				{Address: netip.MustParseAddr("192.0.2.3"), State: plugin.PeerStateEstablished},
				{Address: netip.MustParseAddr("192.0.2.4"), State: plugin.PeerStateActive},
				{Address: netip.MustParseAddr("192.0.2.5"), State: plugin.PeerStateIdleHold},
			},
		}
		resp, err := handleFleetSourceBGPPeer(newTestContext(reactor), commandtest.Args())
		require.NoError(t, err)
		snap := decodePeerSourceSnapshot(t, resp)
		require.Equal(t, []fleet.SourceReading{
			{Figure: fleet.FigureBGPPeersEstablished, Unit: fleet.SampleUnitCount, Value: 2},
			{Figure: fleet.FigureBGPPeersConfigured, Unit: fleet.SampleUnitCount, Value: 5},
		}, snap.Readings)
	})
}

// VALIDATES: ze-peer-cmd.yang declares show fleet source bgp-peer as hidden and
// wires it to wireFleetSourceBGPPeer, and the RPC registration reaches it.
// PREVENTS: hidden fleet source commands leaking into operator completion or
// drifting from their registered wire method.
func TestFleetSourceBGPPeerSchemaAndRegistration(t *testing.T) {
	loader := configyang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.AddModuleFromText("ze-peer-cmd.yang", peeryang.ZePeerCmdYANG))
	schema, err := loader.Resolve()
	require.NoError(t, err)

	tree := configyang.BuildCommandTree(schema)
	node := command.FindNode(tree, strings.Fields(cmdFleetSourceBGPPeer))
	require.NotNil(t, node)
	require.True(t, node.Hidden)
	require.Equal(t, wireFleetSourceBGPPeer, node.WireMethod)
	require.True(t, command.HiddenAtPath(tree, strings.Fields(cmdFleetSourceBGPPeer)))
	require.Empty(t, command.NewTreeCompleter(tree).Complete("show fleet source "))

	found := false
	for _, reg := range pluginserver.AllBuiltinRPCs() {
		if reg.WireMethod == wireFleetSourceBGPPeer {
			found = true
			require.NotNil(t, reg.Handler)
			break
		}
	}
	require.True(t, found, "wire method %s must be registered", wireFleetSourceBGPPeer)
}
