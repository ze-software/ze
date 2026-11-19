// Design: docs/architecture/plugin/rib-storage-design.md -- original receive validation identity.
package adj_rib_in

import (
	"github.com/stretchr/testify/require"
	bgp "github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	pluginapi "github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"net/netip"
	"testing"
)

// Typed and genuine full JSON ingest retain the issuing connection on stored
// bytes. A newer connection can add another useful prefix without relabeling
// the old one, then replace only the prefix it actually updates. This is the
// producer/query boundary, not the later owner-key store migration proof.
func TestReceiveValidationKeepsOriginalConnectionBothRails(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(map[bool]string{false: "typed", true: "json"}[jsonRail], func(t *testing.T) {
			r := newTestManager(t)
			peer := netip.MustParseAddr("192.0.2.188")
			var changes []ribevents.ValidationRoute
			r.validationGate = true
			r.emitValidation = func(routes []ribevents.ValidationRoute) error { changes = append(changes, routes...); return nil }
			ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
			require.NoError(t, err)
			receive := func(connection, msgID uint64, octet byte) {
				attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 1}
				body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
				body = append(body, 24, 10, 77, octet)
				wire := wireu.NewWireUpdate(body, ctxID)
				wire.SetSourceOwner(1701)
				packed, err := wire.Attrs()
				require.NoError(t, err)
				msg := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, MessageID: msgID, SourceOwner: 1701, RawBytes: body, WireUpdate: wire, AttrsWire: packed}
				if jsonRail {
					info := pluginapi.PeerInfo{Address: peer, AddressStr: peer.String(), PeerOwner: 1701, ConnectionID: connection, PeerAS: rpc.RemoteASOf(65000), LocalAS: 65000}
					line := format.AppendMessage(nil, &info, msg, bgptypes.ContentConfig{Encoding: pluginapi.EncodingJSON, Format: pluginapi.FormatFull})
					event, err := bgp.ParseEvent(line)
					require.NoError(t, err)
					r.handleReceived(event)
					return
				}
				r.handleReceivedStructured(&rpc.StructuredEvent{EventType: rpc.EventKindUpdate, PeerAddress: peer.String(), PeerOwner: 1701, ConnectionID: connection, PeerAS: rpc.RemoteASOf(65000), LocalAS: 65000, RawMessage: &msg})
			}
			first := ribevents.ValidationRoute{Peer: peer, PeerOwner: 1701, ConnectionID: 41, Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("10.77.0.0/24")}
			receive(41, 71, 0)
			require.True(t, r.routeEligible(first, 71))
			require.True(t, r.routePresent(first))
			require.NotEmpty(t, changes)
			require.Equal(t, first, changes[len(changes)-1])
			wrong := first
			wrong.ConnectionID = 42
			require.False(t, r.routeEligible(wrong, 71))
			require.False(t, r.routePresent(wrong))
			receive(42, 72, 1)
			useful := wrong
			useful.Prefix = netip.MustParsePrefix("10.77.1.0/24")
			require.True(t, r.routeEligible(useful, 72))
			require.True(t, r.routeEligible(first, 71), "another original prefix retains its original connection")
			receive(42, 73, 0)
			require.False(t, r.routeEligible(first, 0), "old identity cannot borrow replacement bytes")
			require.True(t, r.routeEligible(wrong, 73))
			require.True(t, r.routeEligible(useful, 72), "unrelated useful output survives exact replacement")
		})
	}
}

// Pending validation is not eligible, but its original connection alone owns
// presence. An alias must not retain that connection's ADD-PATH obligations.
func TestPendingValidationPresenceKeepsOriginalConnection(t *testing.T) {
	r := newTestManager(t)
	peer := netip.MustParseAddr("192.0.2.188")
	prefix := netip.MustParsePrefix("10.77.0.0/24")
	key := compactRouteKey{Fam: family.IPv4Unicast, Prefix: prefix}
	r.pending[pendingKey(receivedPeer{address: peer, owner: 1701}, key)] = &pendingRoute{route: &RawRoute{Family: family.IPv4Unicast, SourceOwner: 1701, ConnectionID: 41, MsgID: 71}}
	query := ribevents.ValidationRoute{Peer: peer, PeerOwner: 1701, ConnectionID: 41, Family: family.IPv4Unicast, Prefix: prefix}
	require.True(t, r.routePresent(query))
	require.False(t, r.routeEligible(query, 71))
	query.ConnectionID = 42
	require.False(t, r.routePresent(query))
	query.ConnectionID = 0
	require.False(t, r.routePresent(query))
}
