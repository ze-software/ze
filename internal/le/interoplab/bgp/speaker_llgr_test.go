// Design: docs/architecture/testing/interop.md -- transport-only LLGR source loss.
// Related: speaker_llgr.go -- oracle-local signal registration and source lifetime.
package bgp

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// TestLLGRSourceControlledTransportLoss drives both source variants over owned
// TCP connections. EOF proves no NOTIFICATION or later KEEPALIVE was sent; the
// context-wait barrier proves the source remains in its bounded lifetime after
// closing the socket. No signal is sent to the shared test process.
func TestLLGRSourceControlledTransportLoss(t *testing.T) {
	for _, asn := range []uint{65004, 65005} {
		name := "omitted-family"
		if asn == 65005 {
			name = "zero-LLST"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			lifetime := &llgrSourceWaitContext{Context: ctx, waiting: make(chan struct{})}
			peer, control, result := startLLGRSourceTest(t, lifetime, cancel, asn)
			establishLLGRSourceTest(t, peer, asn)
			control <- syscall.SIGUSR1
			var octet [1]byte
			count, err := peer.Read(octet[:])
			if count != 0 || !errors.Is(err, io.EOF) {
				t.Fatalf("controlled loss = %d bytes, %v; want bare EOF without NOTIFICATION", count, err)
			}
			select {
			case <-lifetime.waiting:
			case err := <-result:
				t.Fatalf("source returned instead of retaining its lifetime: %v", err)
			case <-ctx.Done():
				t.Fatal("source never entered its post-loss lifetime wait")
			}
			select {
			case err := <-result:
				t.Fatalf("source ended before lifetime cancellation: %v", err)
			default:
			}
			cancel()
			select {
			case err := <-result:
				if err != nil {
					t.Fatalf("controlled source cleanup: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("controlled source did not stop after cancellation")
			}
		})
	}
}

// TestLLGRSourceUnexpectedPeerLoss rejects EOF and NOTIFICATION on the same
// established TCP lifecycle used by the controlled-loss proof.
func TestLLGRSourceUnexpectedPeerLoss(t *testing.T) {
	for _, notification := range []bool{false, true} {
		name := "EOF"
		if notification {
			name = "NOTIFICATION"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			peer, _, result := startLLGRSourceTest(t, ctx, cancel, 65004)
			establishLLGRSourceTest(t, peer, 65004)
			want := "lost its fenced connection"
			if notification {
				want = "received NOTIFICATION"
				if _, err := peer.Write(speakerMessage(bgpNotification, []byte{6, 0})); err != nil {
					t.Fatal(err)
				}
			} else if err := peer.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-result:
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("unexpected %s result = %v, want %q", name, err, want)
				}
			case <-ctx.Done():
				t.Fatalf("source ignored unexpected %s", name)
			}
		})
	}
}

// TestLLGRSourceExpiryBeforeControl proves the original bounded lifetime cannot
// silently succeed without the checker's fault, even while its peer stays open.
func TestLLGRSourceExpiryBeforeControl(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	peer, _, result := startLLGRSourceTest(t, ctx, cancel, 65005)
	readLLGRSourceTestMessage(t, peer, bgpOpen)
	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), "before transport-loss control") {
			t.Fatalf("uncontrolled expiry = %v, want lifetime failure", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("source did not stop at its lifetime deadline")
	}
}

// llgrSourceWaitContext exposes entry into the real post-loss context wait,
// without replacing the TCP transport, its close operation, or its deadline.
// Only the source goroutine calls Done; the test owns cancellation and MUST
// join that goroutine through the cleanup installed by startLLGRSourceTest.
type llgrSourceWaitContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (c *llgrSourceWaitContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}

// startLLGRSourceTest transfers its accepted TCP connection to the source and
// MUST join the source on cleanup, including when a wire assertion fails.
func startLLGRSourceTest(t *testing.T, ctx context.Context, cancel context.CancelFunc, asn uint) (net.Conn, chan<- os.Signal, <-chan error) {
	t.Helper()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("source test requires a bounded context")
	}
	// Socket setup must not consume the source's post-loss Done barrier.
	setup, stop := context.WithDeadline(t.Context(), deadline)
	defer stop()
	var config net.ListenConfig
	listener, err := config.Listen(setup, "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() }) //nolint:errcheck // The listener may already be closed.
	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		t.Fatalf("listener = %T, want TCP listener", listener)
	}
	if err := tcpListener.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	var dialer net.Dialer
	peer, err := dialer.DialContext(setup, "tcp4", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { peer.Close() }) //nolint:errcheck // EOF tests deliberately close the peer early.
	if err := peer.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	connection, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	if err := listener.Close(); err != nil {
		connection.Close() //nolint:errcheck // Release the accepted connection before failing setup.
		t.Fatal(err)
	}
	control := make(chan os.Signal, 1)
	result := make(chan error, 1)
	stopped := make(chan struct{})
	options := speakerOptions{asn: asn, routerID: "192.0.2.4", sourceNextHopV6: "2001:db8::4"}
	// This single session goroutine MUST close stopped; cleanup MUST cancel,
	// close its transport to interrupt reads, and join it before the test ends.
	go func() {
		defer close(stopped)
		result <- runLLGRSourceConnection(ctx, options, connection, control)
	}()
	t.Cleanup(func() {
		cancel()
		connection.Close() //nolint:errcheck // Control or source teardown may have already closed it.
		select {
		case <-stopped:
		case <-time.After(3 * time.Second):
			t.Error("LLGR source goroutine did not stop during cleanup")
		}
	})
	return peer, control, result
}

func establishLLGRSourceTest(t *testing.T, peer net.Conn, asn uint) {
	t.Helper()
	readLLGRSourceTestMessage(t, peer, bgpOpen)
	// Literal peer OPEN: BGP 4, ASN 65001, hold 90, ID 192.0.2.1, no options.
	if _, err := peer.Write(speakerMessage(bgpOpen, []byte{4, 0xfd, 0xe9, 0, 90, 192, 0, 2, 1, 0})); err != nil {
		t.Fatal(err)
	}
	readLLGRSourceTestMessage(t, peer, bgpKeepalive)
	if _, err := peer.Write(speakerKeepalive()); err != nil {
		t.Fatal(err)
	}
	frames := 4
	if asn == 65005 {
		frames = 2
	}
	for range frames {
		readLLGRSourceTestMessage(t, peer, bgpUpdate)
	}
}

func readLLGRSourceTestMessage(t *testing.T, peer net.Conn, want byte) {
	t.Helper()
	// Raw framing avoids the production reader's idle result becoming a test
	// synchronization shortcut. The peer deadline bounds both exact reads.
	var header [19]byte
	if _, err := io.ReadFull(peer, header[:]); err != nil {
		t.Fatal(err)
	}
	if header[18] != want {
		t.Fatalf("message type = %d, want %d", header[18], want)
	}
	length := int(header[16])<<8 | int(header[17])
	if length < len(header) {
		t.Fatalf("message length = %d, below header length", length)
	}
	if _, err := io.CopyN(io.Discard, peer, int64(length-len(header))); err != nil {
		t.Fatal(err)
	}
}
