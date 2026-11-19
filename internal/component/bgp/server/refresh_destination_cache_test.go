// Design: docs/architecture/api/process-protocol.md -- recipient refresh renderings.
package server

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRefreshDestinationSurvivesFormatCacheOverflow drives real framed delivery
// with four observer formats and an ordinary owner sharing one of those formats.
// Every recipient must receive its exact projection even when the fixed cache
// cannot retain all five owner/format classes.
func TestRefreshDestinationSurvivesFormatCacheOverflow(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	s := newTestServer(t)
	const owner = "selected-refresh-cache-owner"
	formats := []string{plugin.FormatParsed, plugin.FormatFull, plugin.FormatHex, plugin.FormatBase64}
	observations := make(chan selectedStateObservation, len(formats)+1)
	names := make([]string, 0, len(formats)+1)
	for i, mode := range formats {
		name := fmt.Sprintf("selected-refresh-cache-observer-%d", i)
		proc := selectedStateWireProcess(t, name, observations)
		proc.SetFormat(mode)
		s.Subscriptions().Add(proc, &pluginserver.Subscription{Namespace: events.LookupNamespaceID("bgp"),
			EventType: events.LookupEventTypeID("refresh"), Direction: events.DirReceived})
		names = append(names, name)
	}
	proc := selectedStateWireProcess(t, owner, observations)
	proc.SetFormat(plugin.FormatParsed)
	s.Subscriptions().Add(proc, &pluginserver.Subscription{Namespace: events.LookupNamespaceID("bgp"),
		EventType: events.LookupEventTypeID("refresh"), Direction: events.DirReceived})
	names = append(names, owner)
	const token = uint64(18446744073709551615)
	peer := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.9"), PeerAS: rpc.RemoteASOf(65001),
		DestinationSession: token, SelectedOwner: owner}
	attachForDelivery(t, s, &peer, names...)
	message := bgptypes.RawMessage{Type: msgtype.TypeROUTEREFRESH, RawBytes: []byte{0, 1, 0, 1}, Direction: rpc.DirectionReceived}
	onMessageReceived(s, format.NewJSONEncoder(""), &peer, message)
	seen := make([]string, 0, len(names))
	for range names {
		select {
		case observation := <-observations:
			require.Equal(t, token, observation.session)
			require.Equal(t, observation.process == owner, observation.selected)
			require.Zero(t, observation.replay)
			seen = append(seen, observation.process)
		case <-time.After(5 * time.Second):
			t.Fatal("refresh did not reach every format and recipient class")
		}
	}
	require.ElementsMatch(t, names, seen)
	require.False(t, peer.SelectedDestination)
}
