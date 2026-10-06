// Design: docs/architecture/encoding-context.md -- send context publication and callback lifetime.
package reactor

import (
	"bufio"
	"net/netip"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSendContextPublicationWaitsForControlWrite holds a control write between
// its transport flush and semantic callback, then starts capability publication.
// The publication must wait for that write and must not hold the Peer's mutex:
// the callback reads the Peer just as Reactor.notifyMessageReceiver does.
func TestSendContextPublicationWaitsForControlWrite(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
		peer := NewPeer(settings)
		session := NewSession(settings)
		peer.session = session
		conn := &recordingConn{}
		session.wireWriter = &observedBGPWriter{writer: conn, session: session}
		session.bufWriter = bufio.NewWriter(session.wireWriter)
		session.setSendCtxID(0)

		flushed := make(chan struct{})
		release := make(chan struct{})
		session.onWrite = func() {
			close(flushed)
			<-release
		}
		seen := make(chan bgpctx.ContextID, 2)
		session.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte,
			_ *wireu.WireUpdate, id bgpctx.ContextID, _ rpc.MessageDirection,
			_ BufHandle, _ map[string]any, _ string, _ uint64,
		) bool {
			peer.mu.RLock()
			defer peer.mu.RUnlock()
			seen <- id
			return false
		}
		written := make(chan error, 1)
		go func() {
			// RFC 4271 Section 4.4: exercise the real control-message writer.
			written <- session.writeMessageWithin(conn, message.NewKeepalive(), time.Second)
		}()
		<-flushed

		published := make(chan struct{})
		go func() {
			// RFC 8654 Sections 4 and 6: publish the negotiated direction contexts.
			peer.setEncodingContexts(capability.Negotiate(nil, nil, capability.PeerIdentity{
				LocalASN: 65001, PeerASN: 65002,
			}))
			close(published)
		}()
		synctest.Wait()
		select {
		case <-published:
			t.Error("send context changed before the in-flight control callback completed")
		default:
		}
		if got := session.wireWriter.context.Load(); got != 0 {
			t.Errorf("wire context changed during the write: got %d, want 0", got)
		}
		close(release)
		if err := <-written; err != nil {
			t.Fatal(err)
		}
		if got := <-seen; got != 0 {
			t.Errorf("control callback context = %d, want its original context 0", got)
		}
		<-published

		session.onWrite = nil
		// RFC 4271 Section 4.4: the next control message uses the published context.
		if err := session.writeMessageWithin(conn, message.NewKeepalive(), time.Second); err != nil {
			t.Fatal(err)
		}
		want := peer.sendContextID()
		if want == 0 {
			t.Fatal("negotiation did not register a send context")
		}
		if got := <-seen; got != want {
			t.Errorf("next control callback context = %d, want %d", got, want)
		}
		if got := session.wireWriter.context.Load(); got != uint32(want) {
			t.Errorf("wire context = %d, want %d", got, want)
		}
	})
}
