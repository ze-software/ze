// Design: docs/architecture/api/process-protocol.md -- receiving refresh identity.
package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRefreshDestinationCaptureKeepsReceivingSession uses the real Session
// factory and notifier, then replaces the current connection. A delayed old
// callback must name its original Session and cannot borrow the new owner.
func TestRefreshDestinationCaptureKeepsReceivingSession(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	p := f.peer
	p.messageCallback = f.api.r.messageReceiverForPeer(p)
	original := p.newSession()
	original.initialReplay.Store(71)
	p.mu.Lock()
	p.session = original
	p.mu.Unlock()
	p.refreshForwardFacts()
	var observed plugin.PeerInfo
	f.api.r.setMessageReceiver(&testDeliveryReceiver{onReceived: func(peer plugin.PeerInfo, _ bgptypes.RawMessage) {
		observed = peer
	}})
	body := []byte{0, 1, 0, 1}
	deliver := func(session *Session) {
		session.onMessageReceived(session, p.settings.Address, msgtype.TypeROUTEREFRESH, body, nil, 0,
			rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0)
	}
	deliver(original)
	require.Equal(t, uint64(71), observed.DestinationSession)
	require.Equal(t, selectedWriterProcess, observed.SelectedOwner)

	replacement := p.newSession()
	replacement.initialReplay.Store(72)
	p.mu.Lock()
	p.session = replacement
	p.mu.Unlock()
	p.refreshForwardFacts()
	deliver(original)
	require.Equal(t, uint64(71), observed.DestinationSession, "a delayed callback cannot name the replacement")
	require.Empty(t, observed.SelectedOwner, "an ended Session cannot borrow current output eligibility")
	deliver(replacement)
	require.Equal(t, uint64(72), observed.DestinationSession)
	require.Equal(t, selectedWriterProcess, observed.SelectedOwner)
}
