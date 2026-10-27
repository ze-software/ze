// Design: docs/architecture/meta/filter-community.md -- scrub before the route server reads
// Related: reactor_notify.go -- notifyMessageReceiver, the ingress PeerFilterInfo
// Related: forward_rs.go -- reactorForwardRSSection, rsLocalAS from GlobalLocalAS

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ingressSourceSeen runs one received UPDATE from a session with a per-peer
// local-as override (LocalAS 65010, GlobalLocalAS 65000) through one
// in-process ingress step, and returns the PeerFilterInfo the step was given.
func ingressSourceSeen(t *testing.T, rsClient bool) filterapi.PeerFilterInfo {
	t.Helper()
	addr := netip.MustParseAddr("203.0.113.81")
	source := NewPeer(&PeerSettings{
		Connection:    ConnectionBoth,
		Address:       addr,
		LocalAS:       65010,
		GlobalLocalAS: 65000,
		PeerAS:        65001,
		RouterID:      0xcb007151,
		RSClient:      rsClient,
	})
	source.state.Store(int32(PeerStateEstablished))
	source.session = NewSession(source.Settings())

	var seen []filterapi.PeerFilterInfo
	step := func(src filterapi.PeerFilterInfo, _ []byte, _ map[string]any) (bool, []byte) {
		seen = append(seen, src)
		return true, nil
	}
	r := &Reactor{
		clock:               source.clock,
		config:              &Config{LocalAS: 65000},
		peers:               map[netip.AddrPort]*Peer{source.Settings().PeerKey(): source},
		recentUpdates:       newRecentUpdateCache(100),
		attrModHandlers:     attrModHandlersWithDefaults(),
		messageReceiver:     &testDeliveryReceiver{consumerCount: 1, onReceived: func(_ plugin.PeerInfo, _ bgptypes.RawMessage) {}},
		orderedIngressSteps: []orderedIngressStep{{name: "capture", inproc: step}},
	}

	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN igp
		0x40, 0x02, 0x06, 0x02, 0x01, 0, 0, 0xFD, 0xE9, // AS_PATH [65001]
		0x40, 0x03, 0x04, 203, 0, 113, 81, // NEXT_HOP
	}
	body := buildUpdatePayload(attrs, []byte{24, 192, 0, 2})
	wu := wireu.NewWireUpdate(body, 0)
	wu.SetSourceID(source.SourceID())

	r.notifyMessageReceiver(addr, msgtype.TypeUPDATE, body, wu, 0, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0)

	require.Len(t, seen, 1, "the ingress step runs once")
	return seen[0]
}

// TestIngressRouteServerASIsTheGlobalLocalAS verifies the ingress pass hands a
// route-server client's filters the ASN the route-server rails act on, which is
// the global local-as and not the session's local-as override, and hands every
// other session zero.
//
// The community scrub keeps a client's route-server control communities by
// matching them against this field. The forward rails read and strip them by
// GlobalLocalAS (reactorForwardRSSection's rsLocalAS), so a field filled from
// LocalAS would keep override-AS values the route server neither acts on nor
// strips, and they would reach every client.
//
// VALIDATES: AC-17, A-5 (spec-bcp194-1-communities), with a local-as override.
// PREVENTS: RouteServerAS read from the overridden LocalAS, or left zero on a client.
func TestIngressRouteServerASIsTheGlobalLocalAS(t *testing.T) {
	client := ingressSourceSeen(t, true)
	require.True(t, client.RSClient)
	require.Equal(t, uint32(65010), client.LocalAS, "the session's own AS is the override")
	require.Equal(t, uint32(65000), client.RouteServerAS, "the route server acts on the global local-as")

	other := ingressSourceSeen(t, false)
	require.False(t, other.RSClient)
	require.Zero(t, other.RouteServerAS, "a session that is not a route-server client carries no route-server ASN")
}
