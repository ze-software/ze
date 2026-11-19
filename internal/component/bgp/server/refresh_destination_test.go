// Design: docs/architecture/api/process-protocol.md -- per-recipient refresh facts.
package server

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRefreshDestinationDeliveryTransports drives actual framed JSON and typed
// delivery. Only the resolved owner gains eligibility; both retain the captured
// Session, and rendering one recipient must not contaminate the other's cache.
func TestRefreshDestinationDeliveryTransports(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "wire", true: "bridge"}[typed], func(t *testing.T) {
			snapshot := registry.Snapshot()
			t.Cleanup(func() { registry.Restore(snapshot) })
			s := newTestServer(t)
			const owner, observer = "selected-refresh-owner", "selected-refresh-observer"
			observations := make(chan selectedStateObservation, 2)
			var processes []*process.Process
			for _, name := range []string{owner, observer} {
				if typed {
					processes = append(processes, selectedStateBridgeProcess(t, name, observations))
					continue
				}
				processes = append(processes, selectedStateWireProcess(t, name, observations))
			}
			for _, proc := range processes {
				s.Subscriptions().Add(proc, &pluginserver.Subscription{Namespace: events.LookupNamespaceID("bgp"),
					EventType: events.LookupEventTypeID("refresh"), Direction: events.DirReceived})
			}
			const token = uint64(18446744073709551615)
			peer := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.9"), PeerAS: rpc.RemoteASOf(65001),
				DestinationSession: token, SelectedOwner: owner}
			attachForDelivery(t, s, &peer, owner, observer)
			message := bgptypes.RawMessage{Type: msgtype.TypeROUTEREFRESH, RawBytes: []byte{0, 1, 0, 1}, Direction: rpc.DirectionReceived}
			onMessageReceived(s, format.NewJSONEncoder(""), &peer, message)
			for range processes {
				select {
				case observation := <-observations:
					require.Equal(t, token, observation.session)
					require.Equal(t, observation.process == owner, observation.selected)
					require.Zero(t, observation.replay)
				case <-time.After(5 * time.Second):
					t.Fatal("refresh did not reach both configured recipients")
				}
			}
			require.False(t, peer.SelectedDestination)
		})
	}
}
