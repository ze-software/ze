// Design: docs/architecture/bgp/structural-forwarding.md -- request-owned writes
package reactor

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// The signal makes cancellation happen after the request reaches a cancellable
// wait, rather than racing its initial ctx.Err check.
type observedWriteContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (c *observedWriteContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}

func requestWriteFixture(t *testing.T) (*reactorAPIAdapter, *Peer, *recordingConn, bgptypes.NLRIBatch) {
	t.Helper()
	peer, conn := newAnnouncePeer(t, "192.0.2.2")
	peer.negotiated.Store(&NegotiatedCapabilities{families: map[family.Family]bool{family.IPv4Unicast: true}})
	r := &Reactor{
		config:          &Config{LocalAS: 65000},
		attrModHandlers: attrModHandlersWithDefaults(),
		peers:           map[netip.AddrPort]*Peer{peer.settings.PeerKey(): peer},
	}
	batch := bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.20.0.0/24"), 0)},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")),
	}
	return &reactorAPIAdapter{r: r}, peer, conn, batch
}

// VALIDATES: request cancellation bounds the wait for another session writer.
// PREVENTS: a timed-out announce or withdrawal remaining queued on writeMu and
// being emitted after the caller has already received its cancellation error.
func TestBatchRequestCancelsWriterWait(t *testing.T) {
	for _, withdraw := range []bool{false, true} {
		name := "announce"
		if withdraw {
			name = "withdraw"
		}
		t.Run(name, func(t *testing.T) {
			api, peer, conn, batch := requestWriteFixture(t)
			send := api.AnnounceNLRIBatch
			if withdraw {
				require.NoError(t, api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()))
				require.Len(t, framesOnTheWire(t, conn.written()), 1, "the withdrawal fixture must first advertise its route")
				send = api.WithdrawNLRIBatch
			}
			before := conn.written()
			base, cancel := context.WithCancel(t.Context())
			defer cancel()
			ctx := &observedWriteContext{Context: base, waiting: make(chan struct{})}
			peer.session.writeMu.Lock()
			locked := true
			defer func() {
				if locked {
					peer.session.writeMu.Unlock()
				}
			}()
			done := make(chan error, 1)
			go func() { done <- send(ctx, selector.All(), batch, plugin.OperatorSender()) }()
			select {
			case <-ctx.waiting:
			case <-time.After(5 * time.Second):
				t.Fatal("request never reached its cancellable writer wait")
			}
			cancel()
			select {
			case err := <-done:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(5 * time.Second):
				t.Fatal("canceled request is still waiting for the session writer")
			}
			require.Equal(t, before, conn.written(), "the canceled request must add no wire output")
			peer.session.writeMu.Unlock()
			locked = false
			require.NoError(t, send(t.Context(), selector.All(), batch, plugin.OperatorSender()))
			frames := framesOnTheWire(t, conn.written()[len(before):])
			require.Len(t, frames, 1, "only the succeeding request may reach the socket")
		})
	}
}

type observedWriteConn struct {
	net.Conn
	writing chan struct{}
	once    sync.Once
}

func (c *observedWriteConn) Write(p []byte) (int, error) {
	c.once.Do(func() { close(c.writing) })
	return c.Conn.Write(p)
}

// VALIDATES: canceling a request interrupts its actual blocked socket flush.
// PREVENTS: canceling only the RPC wait while the BGP writer lives on and holds
// the session's write gate indefinitely.
func TestBatchRequestCancelsBlockedFlush(t *testing.T) {
	api, peer, _, batch := requestWriteFixture(t)
	writer, remote := net.Pipe()
	t.Cleanup(func() { _ = writer.Close(); _ = remote.Close() })
	conn := &observedWriteConn{Conn: writer, writing: make(chan struct{})}
	peer.session.conn = conn
	peer.session.bufWriter = bufio.NewWriterSize(conn, 4096)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- api.AnnounceNLRIBatch(ctx, selector.All(), batch, plugin.OperatorSender()) }()
	select {
	case <-conn.writing:
	case <-time.After(5 * time.Second):
		t.Fatal("request never reached the socket flush")
	}
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("canceled request left a socket writer running")
	}
	// Another request must return an error for the unusable connection rather
	// than wait behind a canceled writer that survived its original request.
	go func() { done <- api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()) }()
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("a later request is stuck behind the canceled writer")
	}
	// Interrupted BGP frames cannot be resumed by a later request.
	var header [message.HeaderLen]byte
	n, err := remote.Read(header[:])
	require.Zero(t, n)
	require.ErrorIs(t, err, io.EOF)
}
