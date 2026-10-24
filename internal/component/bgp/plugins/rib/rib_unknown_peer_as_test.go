// Design: docs/architecture/plugin/rib-storage-design.md -- the two event rails
// Related: rib.go -- handleReceived, the JSON rail's refusal
// Related: rib_structured.go -- handleReceivedStructured, the structured rail's refusal
package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	pluginapi "github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// unknownASAnnounce is a valid UPDATE announcing 10.0.0.0/8: ORIGIN IGP, an
// empty AS_PATH and NEXT_HOP 10.0.0.1.
var unknownASAnnounce = []byte{
	0x00, 0x00, // Withdrawn Routes length 0
	0x00, 0x0e, // Total Path Attribute Length 14
	0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
	0x40, 0x02, 0x00, // AS_PATH = empty
	0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
	0x08, 0x0a, // NLRI 10.0.0.0/8
}

// feedReceivedJSONWithAS drives the JSON rail from the shipped producer
// (format.AppendMessage) through the rib's own parser and dispatcher, with the
// peer's AS as the event states it.
func feedReceivedJSONWithAS(t *testing.T, r *RIBManager, peer netip.Addr, peerAS rpc.RemoteAS, ctxID bgpctx.ContextID) {
	t.Helper()
	wu := wireu.NewWireUpdate(unknownASAnnounce, ctxID)
	attrs, err := wu.Attrs()
	require.NoError(t, err, "the test UPDATE body must parse")
	peerInfo := pluginapi.PeerInfo{Address: peer, AddressStr: peer.String(), PeerAS: peerAS, LocalAS: 65000}
	msg := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, RawBytes: unknownASAnnounce, WireUpdate: wu, AttrsWire: attrs}
	line := format.AppendMessage(nil, &peerInfo, msg, bgptypes.ContentConfig{
		Encoding: pluginapi.EncodingJSON,
		Format:   pluginapi.FormatFull,
	})
	event, err := parseEvent(line)
	require.NoError(t, err, "the produced event must parse: %s", line)
	r.dispatch(event)
}

// feedReceivedStructuredWithAS drives the structured rail with the peer's AS
// as the event states it.
func feedReceivedStructuredWithAS(r *RIBManager, peer netip.Addr, peerAS rpc.RemoteAS, ctxID bgpctx.ContextID) {
	wu := wireu.NewWireUpdate(unknownASAnnounce, ctxID)
	attrs, _ := wu.Attrs()
	r.handleReceivedStructured(&rpc.StructuredEvent{
		EventType:   rpc.EventKindUpdate,
		PeerAddress: peer.String(),
		PeerAS:      peerAS,
		LocalAS:     65000,
		RawMessage: &bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			RawBytes:   unknownASAnnounce,
			WireUpdate: wu,
			AttrsWire:  attrs,
		},
	})
}

// VALIDATES: both event rails refuse a received UPDATE whose peer AS is not
// known, and both store one whose AS is known. Method: the same UPDATE is fed
// through each rail twice, once with rpc.RemoteAS{} and once with AS 65001,
// and the test reads the Adj-RIB-In and the peer metadata the rail wrote.
// PREVENTS: the rails disagreeing, which rib.go forbids (updatePeerMetadata):
// the JSON rail once skipped only the metadata and still stored the routes,
// so a peer's routes reached best path with no iBGP or eBGP answer on the
// rail an external plugin reads, while the structured rail dropped them.
func TestReceivedUpdateWithUnknownPeerASIsRefusedOnBothRails(t *testing.T) {
	rails := []struct {
		name string
		feed func(t *testing.T, r *RIBManager, peer netip.Addr, peerAS rpc.RemoteAS, ctxID bgpctx.ContextID)
	}{
		{"json", feedReceivedJSONWithAS},
		{"structured", func(_ *testing.T, r *RIBManager, peer netip.Addr, peerAS rpc.RemoteAS, ctxID bgpctx.ContextID) {
			feedReceivedStructuredWithAS(r, peer, peerAS, ctxID)
		}},
	}
	for _, rail := range rails {
		t.Run(rail.name, func(t *testing.T) {
			ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
			r := newTestRIBManager(t)
			unknown := netip.MustParseAddr("192.0.2.1")
			known := netip.MustParseAddr("192.0.2.2")

			rail.feed(t, r, unknown, rpc.RemoteAS{}, ctxID)
			rail.feed(t, r, known, rpc.RemoteASOf(65001), ctxID)

			r.peerMu.RLock()
			defer r.peerMu.RUnlock()
			assert.Nil(t, r.bgpPeers[unknown], "an UPDATE with no known peer AS must store no route")
			assert.Nil(t, r.peerMeta[unknown], "an UPDATE with no known peer AS must record no peer metadata")
			require.NotNil(t, r.bgpPeers[known], "an UPDATE with a known peer AS must be stored")
			require.NotNil(t, r.peerMeta[known], "an UPDATE with a known peer AS must record peer metadata")
			assert.Equal(t, uint32(65001), r.peerMeta[known].PeerASN)
		})
	}
}
