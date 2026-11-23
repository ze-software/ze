// Design: docs/architecture/api/process-protocol.md -- original actual SENT destination.
package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The real Session callback must publish its own actual SENT session after a
// successor becomes current. The useful successor has a distinct receipt, and
// a SENT observation alone never creates selected ownership.
// MUTATION: omit the SENT UPDATE captured initialReplay projection.
func TestSentDestinationCaptureKeepsOriginalSession(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	p := f.peer
	p.messageCallback = f.api.r.messageReceiverForPeer(p)
	original := p.newSession()
	original.initialReplay.Store(71)
	p.mu.Lock()
	p.session = original
	p.mu.Unlock()
	var observed plugin.PeerInfo
	f.api.r.setMessageReceiver(&testDeliveryReceiver{onSent: func(peer plugin.PeerInfo, _ bgptypes.RawMessage) { observed = peer }})
	body := testUpdatePayload()
	deliver := func(session *Session) {
		session.onMessageReceived(session, p.settings.Address, msgtype.TypeUPDATE, body, nil, 0,
			rpc.DirectionSent, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0)
	}
	deliver(original)
	require.Equal(t, uint64(71), observed.DestinationSession)
	require.Empty(t, observed.SelectedOwner)
	replacement := p.newSession()
	replacement.initialReplay.Store(72)
	p.mu.Lock()
	p.session = replacement
	p.mu.Unlock()
	deliver(original)
	require.Equal(t, uint64(71), observed.DestinationSession, "old actual writer cannot borrow successor metadata")
	deliver(replacement)
	require.Equal(t, uint64(72), observed.DestinationSession, "useful new writer retains its own exact output identity")
	require.Empty(t, observed.SelectedOwner)
}
