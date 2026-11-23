// Design: docs/architecture/api/process-protocol.md -- permanent destination retirement.
package reactor

import (
	"bufio"
	"bytes"
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// TestSentObservationRemovalJoinsOriginalSession covers original-Peer
// publication before receipt capture and after physical Session DOWN. No
// received-source or retention authority is created for these observations.
func TestSentObservationRemovalJoinsOriginalSession(t *testing.T) {
	peer, _ := sourceReceiptFixture(t)
	publication, supported := any(peer).(interface {
		BeginSentObservation() bool
		EndSentObservation()
		joinSentObservations(context.Context) error
	})
	require.True(t, supported, "original Peer lacks SENT publication lifetime")
	peer.state.Store(int32(PeerStateActive))
	require.Nil(t, peer.currentSession(), "pre-UP control must have no receipt-producing Session")
	require.True(t, publication.BeginSentObservation(), "SENT before UP or receipt capture must remain observable")
	held := true
	defer func() {
		if held {
			publication.EndSentObservation()
		}
	}()
	// A sibling at the same address has a separate removal lifetime.
	sibling, _ := sourceReceiptFixture(t)
	require.Equal(t, peer.addrString, sibling.addrString)
	siblingPublication, ok := any(sibling).(interface {
		BeginSentObservation() bool
		EndSentObservation()
		joinSentObservations(context.Context) error
	})
	require.True(t, ok)
	sibling.stopping.Store(true)
	require.NoError(t, siblingPublication.joinSentObservations(t.Context()))
	// Physical DOWN/reconnect state alone does not seal the configured owner.
	peer.state.Store(int32(PeerStateStopped))
	require.True(t, publication.BeginSentObservation())
	publication.EndSentObservation()
	peer.stopping.Store(true)
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, publication.joinSentObservations(canceled), context.Canceled,
		"permanent removal must not acknowledge a held original SENT publication")
	require.False(t, publication.BeginSentObservation(), "sealed Peer must refuse late callbacks")
	publication.EndSentObservation()
	held = false
	require.NoError(t, publication.joinSentObservations(t.Context()))
}

// TestRawSentBeforeSourceReceiptCaptureReachesRouting enters the SAME
// production window as peer_run: enterEstablished has published the Session,
// captureSourceReceipt and announceEstablished/UP have not run. The authorized
// raw API writes a complete UPDATE and the real notifier publishes its SENT.
func TestRawSentBeforeSourceReceiptCaptureReachesRouting(t *testing.T) {
	peer, conn := rawOwnershipPeer(t)
	session := peer.currentSession()
	r := New(&Config{LocalAS: 65000})
	r.peers[peer.settings.PeerKey()] = peer
	peer.reactor = r
	session.onMessageReceived = session.sourceMessageCallback(r.messageReceiverForPeer(peer))
	var routingBodies [][]byte
	r.setMessageReceiver(&testMessageReceiver{onSent: func(info plugin.PeerInfo, message bgptypes.RawMessage) {
		if message.Type != msgtype.TypeUPDATE {
			return
		}
		field := reflect.ValueOf(info).FieldByName("SentObservation")
		if field.IsValid() && !field.IsNil() {
			observation, ok := reflect.TypeAssert[interface {
				BeginSentObservation() bool
				EndSentObservation()
			}](field)
			require.True(t, ok, "SentObservation lacks BeginSentObservation/EndSentObservation")
			if !observation.BeginSentObservation() {
				return
			}
			defer observation.EndSentObservation()
		}
		routingBodies = append(routingBodies, bytes.Clone(message.RawBytes))
	}})
	var packets [][]byte
	session.onWireMessage = func(packet []byte, _ bgpctx.ContextID, sent bool, _ *sessionTransport) {
		require.True(t, sent)
		packets = append(packets, bytes.Clone(packet))
	}
	writer := &observedBGPWriter{session: session, writer: conn}
	session.wireWriter = writer
	session.bufWriter = bufio.NewWriterSize(writer, 4096)
	peer.state.Store(int32(PeerStateActive))
	peer.enterEstablished(session)
	require.Equal(t, PeerStateEstablished, peer.State())
	require.True(t, peer.shouldQueue(), "ordinary replay gate must still be closed")
	require.Nil(t, session.sourceReceipt.Load(), "test must exercise the real pre-capture window")
	body := rawOwnershipAnnounceBody()
	api := &reactorAPIAdapter{r: r}
	require.NoError(t, api.SendRawMessage(peer.settings.Address, uint8(msgtype.TypeUPDATE), body, plugin.ProcessSender(railGranted)))
	require.Equal(t, [][]byte{body}, routingBodies, "legitimate pre-UP raw SENT was lost")
	require.Equal(t, [][]byte{rawOwnershipPacket(body, msgtype.TypeUPDATE)}, packets)
	// The wire recorder's boundary is independent of routing publication.
	peer.stopping.Store(true)
	require.NoError(t, api.SendRawMessage(peer.settings.Address, uint8(msgtype.TypeUPDATE), body, plugin.ProcessSender(railGranted)))
	require.Len(t, routingBodies, 1, "sealed original Peer admitted another routing observation")
	require.Len(t, packets, 2, "refused routing publication hid actual outbound wire bytes")
	require.Equal(t, packets[0], packets[1])
}
