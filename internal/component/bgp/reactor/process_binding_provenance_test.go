// Design: docs/architecture/api/architecture.md -- explicit peer bus filter provenance.
// Related: config.go -- parsePeerBusFromTree; delivery_graph.go -- deliveryPeersFromSettings.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestProcessBindingExplicitProvenance proves parsed peer bus blocks preserve
// written origin, source location, and canonical bus path into the delivery
// graph input.
//
// VALIDATES: parsePeerBusFromTree records BusOriginWritten, canonical BusPath,
// and BusSource, and deliveryPeersFromSettings preserves them on DeliveryPeer.
// PREVENTS: losing written-vs-inherited bus origin or canonical peer path when
// constructing delivery graph inputs.
func TestProcessBindingExplicitProvenance(t *testing.T) {
	address := netip.MustParseAddr("192.0.2.41")
	settings := &PeerSettings{Name: "edge", Address: address}
	require.NoError(t, parsePeerBusFromTree("edge", map[string]any{
		"bus": map[string]any{
			"send": map[string]any{
				"allow": []string{"bgp:update", "bgp:state"},
				"to":    []string{"/bus/endpoint/operator"},
			},
			"receive": map[string]any{
				"allow": []string{"bgp:update"},
				"from":  []string{"/bus/endpoint/operator"},
			},
		},
	}, settings))
	require.Equal(t, pluginserver.BusOriginWritten, settings.BusOrigin)
	require.Equal(t, "/bgp/peer/edge", settings.BusPath)
	require.Equal(t, "/bgp/peer/edge > bus", settings.BusSource)

	graph := deliveryPeersFromSettings([]*PeerSettings{settings})
	require.Len(t, graph, 1)
	require.Equal(t, "/bgp/peer/edge", graph[0].Path)
	require.Equal(t, []string{"/bgp/ip/192.0.2.41"}, graph[0].SecondaryPaths)
	require.Equal(t, pluginserver.BusOriginWritten, graph[0].BusOrigin)
	require.Equal(t, "/bgp/peer/edge > bus", graph[0].BusSource)
	require.True(t, plugin.BusBlockEqual(settings.Bus, graph[0].Bus))
}
