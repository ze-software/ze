// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- complete restart notifications.
// Related: connected_exchange_test.go -- scripted channel and unit peer.

package pppoeclient

import (
	"bytes"
	"io"
	"log/slog"
	"net/netip"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestConnectedChangesEveryCompletion proves unchanged completions still notify
// through the real network worker, and incomplete exchanges publish nothing.
func TestConnectedChangesEveryCompletion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		s, initial := w.session(t)
		stop, done := make(chan struct{}), make(chan struct{})
		defer close(stop)
		go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		magic, identifier := initial.magic, initial.ipcpIdentifier
		for epoch := range 2 {
			magic = w.openLCP(t, 0, magic)
			if len(s.changes) != 0 {
				t.Fatal("partial LCP exchange published a result")
			}
			identifier = w.openIPCP(t, identifier, epoch)
			if len(s.changes) != 1 {
				t.Fatalf("completion %d produced %d records, want one", epoch, len(s.changes))
			}
			got := <-s.changes
			want := iface.PPPoEResult{LocalIP: netip.MustParseAddr("198.51.100.2"),
				PeerIP: netip.MustParseAddr("198.51.100.1"), NegMTU: 1400}
			if got != want {
				t.Fatalf("complete result = %+v, want %+v", got, want)
			}
		}
	})
}

// TestConnectedChangesNewestWins leaves the first result unread while a second
// exchange finishes. The bounded queue must contain only the second full result.
func TestConnectedChangesNewestWins(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		s, initial := w.session(t)
		stop, done := make(chan struct{}), make(chan struct{})
		defer close(stop)
		go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		magic := w.openLCP(t, 0, initial.magic)
		identifier := w.openIPCP(t, initial.ipcpIdentifier, 0)
		w.openLCP(t, 0, magic)
		request := w.take(t, ppp.ProtoIPCP, 1)
		if request[3] != identifier+1 {
			t.Fatal("second exchange reused IPCP Identifier")
		}
		local := []byte{3, 6, 203, 0, 113, 2}
		peer := []byte{3, 6, 203, 0, 113, 1}
		w.send(ppp.ProtoIPCP, 3, request[3], local)
		request = w.take(t, ppp.ProtoIPCP, 1)
		if !bytes.Equal(request[6:], ipcpRequestFor(local)) {
			t.Fatalf("second local address not adopted: %x", request)
		}
		w.send(ppp.ProtoIPCP, 1, 95, peer)
		ack := w.take(t, ppp.ProtoIPCP, 2)
		if !bytes.Equal(ack, exchangeFrame(ppp.ProtoIPCP, 2, 95, peer)) {
			t.Fatalf("second peer Ack = %x", ack)
		}
		w.send(ppp.ProtoIPCP, 2, request[3], request[6:])
		if cap(s.changes) != 1 {
			t.Fatalf("queue bound = %d", cap(s.changes))
		}
		if len(s.changes) != 1 {
			t.Fatalf("pending results = %d", len(s.changes))
		}
		got := <-s.changes
		want := iface.PPPoEResult{LocalIP: netip.MustParseAddr("203.0.113.2"),
			PeerIP: netip.MustParseAddr("203.0.113.1"), NegMTU: 1400}
		if got != want {
			t.Fatalf("newest complete result = %+v, want %+v", got, want)
		}
		if w.passes() != 2 {
			t.Fatal("second completion blocked behind pending notification")
		}
	})
}

// TestConnectedChangesInterruptedExchange proves an overtaken IPCP exchange
// sends nothing, and the next complete exchange sends exactly one result.
func TestConnectedChangesInterruptedExchange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		s, initial := w.session(t)
		stop, done := make(chan struct{}), make(chan struct{})
		defer close(stop)
		go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		magic := w.openLCP(t, 0, initial.magic)
		request := w.take(t, ppp.ProtoIPCP, 1)
		w.openLCP(t, ppp.ProtoPAP, magic)
		w.authenticate(t, ppp.ProtoPAP, true)
		if len(s.changes) != 0 {
			t.Fatal("interrupted IPCP published a result")
		}
		w.openIPCP(t, request[3], 0)
		if len(s.changes) != 1 {
			t.Fatalf("final completion produced %d records", len(s.changes))
		}
	})
}

// TestConnectedChangesFailureSilent proves failed forwarding restoration cannot
// notify iface even though both directions of IPCP have completed.
func TestConnectedChangesFailureSilent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		w.modeFailure = ppp.NPModePass
		s, initial := w.session(t)
		stop, done := make(chan struct{}), make(chan struct{})
		defer close(stop)
		go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		w.openLCP(t, 0, initial.magic)
		w.openIPCP(t, initial.ipcpIdentifier, 0)
		select {
		case <-done:
		default:
			t.Fatal("failed PASS did not end the worker")
		}
		if len(s.changes) != 0 {
			t.Fatal("failed restart published a result")
		}
	})
}
