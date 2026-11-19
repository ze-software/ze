// Design: docs/architecture/plugin/rib-storage-design.md -- pending membership publication.
package adj_rib_in

import (
	"net/netip"
	"testing"

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
)

// A first pending arrival changes known membership even though it is still
// ineligible. Both receive producers MUST publish that original path after
// releasing the store lock. An install before the gate has ever been enabled
// stays useful and emits nothing, preserving the live-forward contract.
func TestPendingAdmissionPublishesOriginalMembershipBothRails(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		for _, gated := range []bool{false, true} {
			rail := map[bool]string{false: "typed", true: "json"}[jsonRail]
			phase := map[bool]string{false: "no-gate", true: "pending"}[gated]
			t.Run(rail+"/"+phase, func(t *testing.T) {
				t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
				r := newTestManager(t)
				r.ingestTracked = true
				peer := netip.MustParseAddr("192.0.2.188")
				const owner, connection, receipt, epoch, message uint64 = 1701, 41, 91, 5, 71
				key := ribevents.ValidationRoute{
					Peer: peer, PeerOwner: owner, ConnectionID: connection,
					Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("10.77.0.0/24"),
				}
				var changes []ribevents.ValidationRoute
				r.emitValidation = func(batch []ribevents.ValidationRoute) error {
					if !r.mu.TryLock() {
						t.Fatal("pending membership was published while the store lock was held")
					}
					r.mu.Unlock()
					for _, change := range batch {
						require.Equal(t, key, change, "the original pending path owns its notification")
						require.True(t, ribevents.RoutePresent(change), "published membership must already be queryable")
						require.False(t, ribevents.RouteEligible(change, message), "publication must not accept pending input")
					}
					changes = append(changes, batch...)
					return nil
				}
				if gated {
					_, _, err := r.handleCommand("request bgp adj-rib-in enable-validation", []string{"timeout", "65535"}, "")
					require.NoError(t, err)
				}
				require.False(t, r.routePresent(key), "the original path is absent before its first UPDATE")
				ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
				require.NoError(t, err)
				body := []byte{0, 0, 0, 14, 0x40, 1, 1, 0, 0x40, 2, 0,
					0x40, 3, 4, 192, 0, 2, 9, 24, 10, 77, 0}
				wire := wireu.NewWireUpdate(body, ctxID)
				wire.SetSourceOwner(owner)
				wire.SetSourceReceipt(receipt)
				attrs, err := wire.Attrs()
				require.NoError(t, err)
				msg := bgptypes.RawMessage{
					Type: msgtype.TypeUPDATE, MessageID: message, SourceOwner: owner,
					SourceReceipt: receipt, SourcePolicyEpoch: epoch,
					RawBytes: body, WireUpdate: wire, AttrsWire: attrs,
				}
				if jsonRail {
					info := pluginapi.PeerInfo{
						Address: peer, AddressStr: peer.String(), PeerOwner: owner, ConnectionID: connection,
						PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000,
					}
					line := format.AppendMessage(nil, &info, msg, bgptypes.ContentConfig{Encoding: pluginapi.EncodingJSON, Format: pluginapi.FormatFull})
					event, err := bgp.ParseEvent(line)
					require.NoError(t, err)
					r.handleReceived(event)
				} else {
					r.handleReceivedStructured(&rpc.StructuredEvent{
						EventType: rpc.EventKindUpdate, PeerAddress: peer.String(), PeerOwner: owner,
						ConnectionID: connection, SourceReceipt: receipt, SourcePolicyEpoch: epoch,
						PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000, RawMessage: &msg,
					})
				}
				require.True(t, r.routePresent(key), "the same original route remains retained")
				if gated {
					require.Equal(t, []ribevents.ValidationRoute{key}, changes, "first pending membership must reach the subscriber")
					require.False(t, r.routeEligible(key, message))
					require.Len(t, r.pending, 1)
				} else {
					require.Empty(t, changes, "a gate that never enabled must not duplicate the live forward")
					require.True(t, r.routeEligible(key, message), "the ungated control must remain useful")
					require.Empty(t, r.pending)
				}
				for _, wrong := range []ribevents.ValidationRoute{
					{Peer: peer, PeerOwner: owner + 1, ConnectionID: connection, Family: key.Family, Prefix: key.Prefix},
					{Peer: peer, PeerOwner: owner, ConnectionID: connection + 1, Family: key.Family, Prefix: key.Prefix},
				} {
					require.False(t, r.routePresent(wrong), "another original source cannot borrow membership")
					require.False(t, r.routeEligible(wrong, message))
				}
				r.mu.RLock()
				stored := r.validationStoredRouteLocked(receivedPeerOf(peer, owner, connection), compactRouteKey{Fam: key.Family, Prefix: key.Prefix})
				if stored == nil {
					r.mu.RUnlock()
					t.Fatal("admission lost the actual stored route")
				}
				gotOwner, gotConnection := stored.SourceOwner, stored.ConnectionID
				gotReceipt, gotEpoch, gotMessage := stored.SourceReceipt, stored.SourcePolicyEpoch, stored.MsgID
				r.mu.RUnlock()
				require.Equal(t, owner, gotOwner)
				require.Equal(t, connection, gotConnection)
				require.Equal(t, receipt, gotReceipt)
				require.Equal(t, epoch, gotEpoch)
				require.Equal(t, message, gotMessage)
			})
		}
	}
}
