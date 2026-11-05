// Design: docs/architecture/testing/ci-format.md -- remote_close peer option

package peer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// remoteCloseTestConn supplies only the two operations waitRemoteClose owns.
// Each test initializes read; the embedded connection is never used.
type remoteCloseTestConn struct {
	net.Conn
	read func([]byte) (int, error)
}

func (c *remoteCloseTestConn) Read(p []byte) (int, error)      { return c.read(p) }
func (c *remoteCloseTestConn) SetReadDeadline(time.Time) error { return nil }

func TestRemoteCloseRequiresTransportEnd(t *testing.T) {
	for _, tc := range []struct {
		name            string
		err             error
		data            bool
		cancel          bool
		cancelAfterRead bool
		want            bool
	}{
		{name: "EOF", err: io.EOF, want: true},
		{name: "reset", err: syscall.ECONNRESET, want: true},
		{name: "other error", err: io.ErrClosedPipe},
		{name: "trailing data", data: true, err: io.EOF},
		{name: "teardown", cancel: true},
		{name: "never closes before timeout", err: os.ErrDeadlineExceeded, cancelAfterRead: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			p, err := New(&Config{RemoteClose: true, Output: &output})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			conn := &remoteCloseTestConn{read: func(buf []byte) (int, error) {
				if tc.cancelAfterRead {
					cancel()
				}
				if tc.data {
					buf[0] = 1
					return 1, tc.err
				}
				return 0, tc.err
			}}
			result := p.completed(ctx, conn)
			if result.Success != tc.want {
				t.Fatalf("success=%v error=%v, want %v", result.Success, result.Error, tc.want)
			}
			if !tc.want {
				if result.Error == nil {
					t.Fatal("failure lost its reason")
				}
				if strings.Contains(output.String(), "successful") {
					t.Fatal("announced success before remote close")
				}
			}
			if tc.cancel {
				if !errors.Is(result.Error, context.Canceled) {
					t.Fatalf("teardown error=%v", result.Error)
				}
			}
		})
	}
}

func TestRemoteCloseOption(t *testing.T) {
	for _, config := range []Config{
		{RemoteClose: true, Linger: true},
		{RemoteClose: true, ConnMap: connMapRouterID},
		{RemoteClose: true, Mode: ModeSink},
	} {
		if _, err := New(&config); err == nil {
			t.Fatalf("accepted incompatible config %+v", config)
		}
	}
	for _, value := range []string{"", "yes", "garbage"} {
		if err := parsePeerOptionRemoteClose(&Config{}, map[string]string{keyValue: value}); err == nil {
			t.Fatalf("accepted remote_close=%q", value)
		}
	}
	var config Config
	if err := parsePeerOptionRemoteClose(&config, map[string]string{keyValue: "true"}); err != nil {
		t.Fatal(err)
	}
	if !config.RemoteClose {
		t.Fatal("remote_close was not set")
	}
}

// runRemoteClosePeer is one lifecycle worker, joined by each test's result read.
func runRemoteClosePeer(ctx context.Context, p *Peer, done chan<- Result) {
	done <- p.Run(ctx)
}

func TestRemoteCloseRunRejectsCancellationWithoutConnection(t *testing.T) {
	p, err := New(&Config{RemoteClose: true, Output: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	done := make(chan Result, 1)
	go runRemoteClosePeer(ctx, p, done)
	select {
	case <-p.Ready():
	case result := <-done:
		t.Fatalf("peer failed before readiness: %v", result.Error)
	}
	cancel()
	result := <-done
	if result.Success {
		t.Fatal("empty checker accepted cancellation without observing remote close")
	}
	if result.Error == nil {
		t.Fatal("missing failure reason")
	}
}

func TestRemoteCloseRunRejectsLocalClose(t *testing.T) {
	port := reservePort(t)
	p, err := New(&Config{
		Port: port, RemoteClose: true, Output: io.Discard,
		Expect: []string{
			"expect=bgp:conn=1:seq=1:hex=" + unmetEOR,
			"action=close:conn=1:seq=2",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	done := make(chan Result, 1)
	go runRemoteClosePeer(ctx, p, done)
	select {
	case <-p.Ready():
	case result := <-done:
		t.Fatalf("peer failed before readiness: %v", result.Error)
	}
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() //nolint:errcheck // Test socket cleanup, not the asserted remote close.
	if err := serveOneConnection(conn); err != nil {
		t.Fatal(err)
	}
	result := <-done
	if result.Success {
		t.Fatal("local action=close supplied remote-close evidence")
	}
	if result.Error == nil {
		t.Fatal("missing failure reason")
	}
}
