// Design: docs/architecture/api/process-protocol.md -- stable original Peer identity.
// Related: source_session.go -- exact Session publication and original receipt.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestReceivedUpdateCarriesExactStableSourceOwner drives the real Session
// callback and notifier for two configured peers at one IP. Reconnect changes
// the receipt but retains the original Peer identity; recreation changes it.
func TestReceivedUpdateCarriesExactStableSourceOwner(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	address := netip.MustParseAddr("192.0.2.87")
	settings := []*PeerSettings{
		{Address: address, Port: 1179, LocalAS: 65000, PeerAS: 65001, Name: "first"},
		{Address: address, Port: 2179, LocalAS: 65000, PeerAS: 65002, Name: "sibling"},
	}
	for _, value := range settings {
		acceptingPolicies(t, value)
		require.NoError(t, r.AddPeer(value))
	}
	first, sibling := r.peers[settings[0].PeerKey()], r.peers[settings[1].PeerKey()]
	require.NotZero(t, first.sourceOwner)
	require.NotEqual(t, first.sourceOwner, sibling.sourceOwner)
	var observed bgptypes.RawMessage
	var observedName string
	r.setMessageReceiver(&testDeliveryReceiver{consumerCount: 1,
		onReceived: func(peer plugin.PeerInfo, message bgptypes.RawMessage) {
			observed, observedName = message, peer.Name
		}})
	var receipts sourceReceiptStore
	var originalReceipt uint64
	for _, peer := range []*Peer{first, sibling, first} {
		session := sourceSessionFixture(t, peer)
		require.NoError(t, session.captureSourceReceipt(peer, &receipts))
		t.Cleanup(session.closeSourceReceipt)
		body := []byte{0, 0, 0, 0}
		update := wireu.NewWireUpdate(body, session.recvCtxID)
		require.True(t, session.onMessageReceived(address, msgtype.TypeUPDATE, body, update,
			session.recvCtxID, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0))
		require.Equal(t, peer.settings.Name, observedName)
		require.Equal(t, peer.sourceOwner, observed.SourceOwner)
		require.Equal(t, session.sourceReceiptID(), observed.SourceReceipt)
		if originalReceipt == 0 {
			originalReceipt = observed.SourceReceipt
		} else {
			require.NotEqual(t, originalReceipt, observed.SourceReceipt)
		}
		r.recentUpdates.abortPublication(observed.MessageID)
	}
	recreated := NewPeer(settings[0])
	require.NotEqual(t, first.sourceOwner, recreated.sourceOwner)
}

// TestSourceReceiptFactsRequireStablePeerOwner distinguishes unresolved Peer
// identity from a valid captured source. An address and generation grant none.
func TestSourceReceiptFactsRequireStablePeerOwner(t *testing.T) {
	peer, facts := sourceReceiptFixture(t)
	require.NotZero(t, peer.sourceOwner)
	_, err := newSourceReceiptFacts(facts.source)
	require.NoError(t, err)
	peer.sourceOwner = 0 // Deliberately corrupt this isolated, transport-free fixture.
	_, err = newSourceReceiptFacts(facts.source)
	require.Error(t, err)
}
