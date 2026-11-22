// Design: docs/architecture/route-selection.md -- recording AIGP original-session inputs.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// aigpFixturePublish uses the actual original-Session receipt producer and
// callback. Capture happens after the question configured source policy/role.
// The caller MUST keep the same recording fixture until publication completes.
func aigpFixturePublish(t *testing.T, f *aigpReplayFixture, peer *Peer, session *Session, body []byte, update *wireu.WireUpdate) {
	t.Helper()
	if session.sourceReceiptID() == 0 {
		if peer.forwardGeneration.Load() == 0 {
			peer.advanceForwardGeneration()
		}
		session.setRecvCtxID(f.ctxID)
		require.NoError(t, session.captureSourceReceipt(peer, &sourceReceipts))
		// MUST close the same receipt after this fixture's receive work ends.
		t.Cleanup(session.closeSourceReceipt)
	}
	callback := session.sourceMessageCallback(func(captured *Session, address netip.Addr, kind msgtype.MessageType, raw []byte, wire *wireu.WireUpdate, context bgpctx.ContextID, direction rpc.MessageDirection, buffer BufHandle, meta map[string]any, source string, id uint64) bool {
		return f.r.notifyMessageReceiverFrom(peer, captured, address, kind, raw, wire, context, direction, buffer, meta, source, id)
	})
	require.True(t, callback(session, peer.Settings().Address, msgtype.TypeUPDATE, body, update, f.ctxID, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0))
}

func aigpFixtureReceive(t *testing.T, f *aigpReplayFixture, peer *Peer, session *Session, body []byte) uint64 {
	t.Helper()
	update := wireu.NewWireUpdate(body, f.ctxID)
	update.SetSourceID(peer.SourceID())
	aigpFixturePublish(t, f, peer, session, body, update)
	f.ids = append(f.ids, update.MessageID())
	return update.MessageID()
}
