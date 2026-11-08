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
			if w.passes() != epoch {
				t.Fatalf("completion %d passed IPv4 before the interface applied it: %v", epoch, w.modes)
			}
			got := applyPending(t, s)
			want := iface.PPPoEResult{LocalIP: netip.MustParseAddr("198.51.100.2"),
				PeerIP: netip.MustParseAddr("198.51.100.1"), NegMTU: 1400, Seq: uint64(epoch + 1)}
			if got != want {
				t.Fatalf("complete result = %+v, want %+v", got, want)
			}
			if w.passes() != epoch+1 {
				t.Fatalf("completion %d did not pass IPv4 once applied: %v", epoch, w.modes)
			}
		}
	})
}

// TestConnectedChangesNewestWins leaves a published result unread while a
// newer one is published. Method: an IPCP Configure-Request while IPCP is
// Opened publishes the IPv4 withdrawal (Seq 1), which nobody waits on, then
// IPCP reopens with new addresses. The one-slot queue must then hold only the
// newer full result (Seq 2), and IPv4 must stay DROP until that Seq is
// acknowledged, then pass exactly once.
func TestConnectedChangesNewestWins(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		s, _ := w.session(t)
		stop, done := make(chan struct{}), make(chan struct{})
		defer close(stop)
		go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		synctest.Wait()
		local := []byte{3, 6, 203, 0, 113, 2}
		peer := []byte{3, 6, 203, 0, 113, 1}
		// RFC 1661 Section 4.1, Opened: RCR takes tld, scr, sca.
		w.send(ppp.ProtoIPCP, ppp.LCPConfigureRequest, 95, peer)
		if len(w.modes) != 1 || w.modes[0] != ppp.NPModeDrop {
			t.Fatalf("IPv4 modes %v, want one drop when IPCP left Opened", w.modes)
		}
		if len(s.changes) != 1 {
			t.Fatalf("IPCP leaving Opened published %d results, want the withdrawal", len(s.changes))
		}
		request := w.take(t, ppp.ProtoIPCP, ppp.LCPConfigureRequest)
		ack := w.take(t, ppp.ProtoIPCP, ppp.LCPConfigureAck)
		if !bytes.Equal(ack, exchangeFrame(ppp.ProtoIPCP, ppp.LCPConfigureAck, 95, peer)) {
			t.Fatalf("peer IPCP Ack = %x", ack)
		}
		w.send(ppp.ProtoIPCP, ppp.LCPConfigureNak, request[3], local)
		request = w.take(t, ppp.ProtoIPCP, ppp.LCPConfigureRequest)
		if !bytes.Equal(request[6:], ipcpRequestFor(local)) {
			t.Fatalf("new local address not adopted: %x", request)
		}
		w.send(ppp.ProtoIPCP, ppp.LCPConfigureAck, request[3], request[6:])
		w.quiet(t)
		if cap(s.changes) != 1 {
			t.Fatalf("queue bound = %d", cap(s.changes))
		}
		if len(s.changes) != 1 {
			t.Fatalf("pending results = %d", len(s.changes))
		}
		got := <-s.changes
		want := iface.PPPoEResult{LocalIP: netip.MustParseAddr("203.0.113.2"),
			PeerIP: netip.MustParseAddr("203.0.113.1"), NegMTU: 1492, Seq: 2}
		if got != want {
			t.Fatalf("newest complete result = %+v, want %+v: the unread withdrawal was not replaced", got, want)
		}
		if w.passes() != 0 {
			t.Fatalf("IPv4 passed before the interface applied the result: %v", w.modes)
		}
		s.acknowledge(got.Seq)
		synctest.Wait()
		if len(w.modes) != 2 || w.modes[1] != ppp.NPModePass {
			t.Fatalf("IPv4 modes %v, want drop then exactly one pass once applied", w.modes)
		}
		if len(s.changes) != 0 {
			t.Fatal("applying the result published another")
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
		if got := applyPending(t, s); got.Seq != 1 {
			t.Fatalf("final completion Seq = %d, want 1: the interrupted exchange published", got.Seq)
		}
		if w.passes() != 1 {
			t.Fatalf("applied completion did not pass IPv4: %v", w.modes)
		}
	})
}

// TestConnectedChangesFailureSilent proves a failed forwarding restoration
// notifies iface exactly once: the completed result is published and PASS is
// attempted only after the consumer applied it. Method: fail PASS, check the
// one pending result (Seq 1) while the worker waits with no PASS attempted,
// acknowledge it, then check the worker ends and publishes nothing more.
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
		synctest.Wait()
		if len(s.changes) != 1 {
			t.Fatalf("completed restart published %d results, want one", len(s.changes))
		}
		if got := <-s.changes; got.Seq != 1 {
			t.Fatalf("pending result Seq = %d, want 1", got.Seq)
		}
		select {
		case <-done:
			t.Fatal("the worker ended before the interface applied the result")
		default:
		}
		if w.passes() != 0 {
			t.Fatalf("PASS attempted before the interface applied the result: %v", w.modes)
		}
		s.acknowledge(1)
		synctest.Wait()
		select {
		case <-done:
		default:
			t.Fatal("failed PASS did not end the worker")
		}
		if len(s.changes) != 0 {
			t.Fatal("failed PASS published a second result")
		}
	})
}
