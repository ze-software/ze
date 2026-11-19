// Design: docs/architecture/plugin/rib-storage-design.md -- original connection on received and sent rails.
package rib

import (
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	pluginapi "github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"net/netip"
	"testing"
)

// MUTATION: omit the received event ConnectionID at either acquire callsite.
func TestRIBReceivedConnectionProvenanceBothRails(t *testing.T) {
	for _, native := range []bool{false, true} {
		for _, jsonRail := range []bool{false, true} {
			for _, labeled := range []bool{false, true} {
				t.Run(map[bool]string{false: "wire-", true: "native-"}[native]+testReceiptRailName(jsonRail, labeled), func(t *testing.T) {
					r, _ := sourceLeaseTestRIB(t, native)
					address := netip.MustParseAddr("192.0.2.81")
					fam, raw := sourceLeaseReceive(t, r, address, 81, 701, jsonRail, labeled)
					entry, found := r.bgpPeers[uint64(81)].LookupRetained(fam, raw)
					require.True(t, found)
					defer entry.Release()
					require.Equal(t, uint64(41), entry.SourceConnectionID(), "original received event physical connection is retained")
					require.Equal(t, uint64(81), entry.SourcePeerOwner())
					require.Equal(t, uint64(701), entry.SourceReceipt)
				})
			}
		}
	}
}

// The real formatter and both RIB sent constructors retain the ORIGINAL
// source connection, distinct from the event's destination connection.
// MUTATION: omit source-connection-id in full JSON or either sent row constructor.
func TestRIBSentOriginalConnectionSurvivesBothTransports(t *testing.T) {
	const sourceConnection uint64 = 1<<53 + 71
	const destinationOwner uint64 = 91
	destination := netip.MustParseAddr("192.0.2.91")
	fam := family.IPv4Unicast
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	raw := []byte{24, 192, 0, 2}
	for _, rail := range []string{"structured", "full-json", "family-ops"} {
		t.Run(rail, func(t *testing.T) {
			r := newTestRIBManager(t)
			attrs := unicastAttrs([4]byte{192, 0, 2, 81}, 30, 100)
			body := append([]byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}, attrs...)
			body = append(body, raw...)
			context, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
			require.NoError(t, err)
			update := wireu.NewWireUpdate(body, context)
			aw, err := update.Attrs()
			require.NoError(t, err)
			message := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, Direction: rpc.DirectionSent, MessageID: 99,
				SourceOwner: 81, SourceConnectionID: sourceConnection, SourceReceipt: 701, SourceMessageID: 87,
				RawBytes: body, WireUpdate: update, AttrsWire: aw, Meta: map[string]any{bgptypes.SourceConnectionMeta: "999"}}
			if rail == "structured" {
				r.handleSentStructured(&rpc.StructuredEvent{PeerOwner: destinationOwner, ConnectionID: 812,
					PeerAddress: destination.String(), PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000, MessageID: 99, RawMessage: &message})
			} else {
				peer := pluginapi.PeerInfo{PeerOwner: destinationOwner, ConnectionID: 812, Address: destination, PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000}
				data := format.AppendSentMessage(nil, &peer, message, bgptypes.ContentConfig{Encoding: pluginapi.EncodingJSON, Format: pluginapi.FormatFull})
				event, err := parseEvent(data)
				require.NoError(t, err)
				require.Equal(t, uint64(812), event.ConnectionID)
				require.Equal(t, "9007199254741063", event.RouteMeta[bgptypes.SourceConnectionMeta], "route metadata cannot replace original transport provenance")
				if rail == "family-ops" {
					event.RawNLRI = nil
					event.FamilyOps[fam][0].NLRIs = []any{prefix.String()}
				}
				r.handleSent(event)
			}
			entry, found := r.ribOut[destinationOwner][fam][ribOutKey{Prefix: prefix}]
			require.True(t, found, "useful sent route remains in the destination inventory")
			require.Equal(t, sourceConnection, entry.SourceConnectionID)
			require.Equal(t, uint64(81), entry.SourceOwner)
			require.NotEqual(t, uint64(812), entry.SourceConnectionID)
			message.SourceOwner = 0
			message.SourceConnectionID = 0
			message.SourceReceipt = 0
			message.SourceLocal = true
			if rail == "structured" {
				r.handleSentStructured(&rpc.StructuredEvent{PeerOwner: destinationOwner, ConnectionID: 812, PeerAddress: destination.String(), PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000, MessageID: 99, RawMessage: &message})
			} else {
				peer := pluginapi.PeerInfo{PeerOwner: destinationOwner, ConnectionID: 812, Address: destination, PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000}
				event, err := parseEvent(format.AppendSentMessage(nil, &peer, message, bgptypes.ContentConfig{Encoding: pluginapi.EncodingJSON, Format: pluginapi.FormatFull}))
				require.NoError(t, err)
				if rail == "family-ops" {
					event.RawNLRI = nil
					event.FamilyOps[fam][0].NLRIs = []any{prefix.String()}
				}
				r.handleSent(event)
			}
			local, found := r.ribOut[destinationOwner][fam][ribOutKey{Prefix: prefix}]
			require.True(t, found, "explicit local origin remains useful without a received connection")
			require.True(t, local.LocalOrigin)
			require.Zero(t, local.SourceConnectionID, "local replacement cannot inherit the former learned source connection")
			for _, families := range r.ribOut {
				for _, routes := range families {
					for _, entry := range routes {
						entry.release()
					}
				}
			}
			r.ribOut = make(map[uint64]map[family.Family]map[ribOutKey]ribOutEntry)
		})
	}
	for _, invalid := range []any{float64(sourceConnection), "-1", "1.5", "18446744073709551616", nil} {
		require.Zero(t, sentSourceConnection(map[string]any{bgptypes.SourceConnectionMeta: invalid}))
	}
}
