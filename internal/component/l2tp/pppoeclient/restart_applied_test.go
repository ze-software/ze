// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- forwarding resumes only once iface applied the result.
// Related: connected_session.go -- restart, awaitApplied; ipcp_independent.go -- renegotiateIPCP.
// Related: network_phase.go -- keepaliveLoop's reenter takes IPv6CP down.

package pppoeclient

import (
	"bytes"
	"io"
	"log/slog"
	"net/netip"
	"slices"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestRestartForwardingWaitsForApplied proves an LCP restart keeps IPv4 at
// DROP after IPCP opens until the interface consumer reports the published
// result applied: a stale report does not release it, the matching one does.
// Break: in connectedSession.restart, set NPModePass before awaitApplied (or
// drop the awaitApplied call); PASS then precedes the report and the first
// passes check goes red.
func TestRestartForwardingWaitsForApplied(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		s, initial := w.session(t)
		stop, done := make(chan struct{}), make(chan struct{})
		defer close(stop)
		go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		w.openLCP(t, 0, initial.magic)
		w.openIPCP(t, initial.ipcpIdentifier, 0)
		synctest.Wait()
		if w.passes() != 0 {
			t.Fatalf("IPv4 passed before the interface applied the result: %v", w.modes)
		}
		if len(s.changes) != 1 {
			t.Fatalf("pending results = %d, want the restart's result", len(s.changes))
		}
		got := <-s.changes
		if got.Seq == 0 {
			t.Fatal("published result carries no Seq")
		}
		if got.LocalIP != netip.MustParseAddr("198.51.100.2") {
			t.Fatalf("published local address = %v", got.LocalIP)
		}
		s.acknowledge(got.Seq - 1)
		synctest.Wait()
		if w.passes() != 0 {
			t.Fatalf("a report for an older result passed IPv4: %v", w.modes)
		}
		s.acknowledge(got.Seq)
		synctest.Wait()
		if w.passes() != 1 {
			t.Fatalf("IPv4 still discarded after the result was applied: %v", w.modes)
		}
		select {
		case <-done:
			t.Fatal("the applied restart ended the worker")
		default:
		}
	})
}

// TestIPCPRenegotiationWaitsForApplied proves the IPCP-only renegotiation
// (renegotiateIPCP) holds IPv4 at DROP until its new addresses are applied.
// Break: in renegotiateIPCP, set NPModePass before awaitApplied; PASS then
// precedes the report.
func TestIPCPRenegotiationWaitsForApplied(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		s, initial := w.session(t)
		stop, done := make(chan struct{}), make(chan struct{})
		defer close(stop)
		go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		synctest.Wait()
		// RCR in Opened: tld (DROP, withdrawal published), then scr and sca.
		peer := []byte{3, 6, 198, 51, 100, 1}
		w.send(ppp.ProtoIPCP, ppp.LCPConfigureRequest, 96, peer)
		request := w.take(t, ppp.ProtoIPCP, ppp.LCPConfigureRequest)
		if request[3] != initial.ipcpIdentifier+1 {
			t.Fatalf("IPCP Identifier = %d, want %d", request[3], initial.ipcpIdentifier+1)
		}
		if ack := w.take(t, ppp.ProtoIPCP, ppp.LCPConfigureAck); !bytes.Equal(ack, exchangeFrame(ppp.ProtoIPCP, 2, 96, peer)) {
			t.Fatalf("peer IPCP Ack = %x", ack)
		}
		w.send(ppp.ProtoIPCP, ppp.LCPConfigureNak, request[3], []byte{3, 6, 198, 51, 100, 2})
		request = w.take(t, ppp.ProtoIPCP, ppp.LCPConfigureRequest)
		w.send(ppp.ProtoIPCP, ppp.LCPConfigureAck, request[3], request[6:])
		synctest.Wait()
		if w.passes() != 0 {
			t.Fatalf("IPv4 passed before the interface applied the reopened IPCP: %v", w.modes)
		}
		got := applyPending(t, s)
		if got.LocalIP != netip.MustParseAddr("198.51.100.2") {
			t.Fatalf("published local address = %v, want the reopened one", got.LocalIP)
		}
		if w.passes() != 1 {
			t.Fatalf("IPv4 still discarded after the reopened IPCP was applied: %v", w.modes)
		}
	})
}

// TestClientNetworkPhaseRenegotiationRecovers drives the network-phase request
// of TestRFC1661ClientRenegotiatesLCPInNetworkPhase (Identifier 0x73, Magic
// 0x55667788) to a complete recovery: the peer revises its request with CHAP,
// the client authenticates again, IPCP reopens, and forwarding is restored
// once the result is applied.
// Break: in keepaliveLoop, replace the LCPConfigureRequest case body with
// continue; no Configure-Request follows and the first take goes red.
func TestClientNetworkPhaseRenegotiationRecovers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		s, initial := w.session(t)
		stop, done := make(chan struct{}), make(chan struct{})
		defer close(stop)
		go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		synctest.Wait()
		w.frames <- clientRenegotiationRequest()
		request := w.take(t, ppp.ProtoLCP, ppp.LCPConfigureRequest)
		if w.events[0] != "drop" {
			t.Fatalf("restart did not DROP before its first write: %v", w.events)
		}
		ack := w.take(t, ppp.ProtoLCP, ppp.LCPConfigureAck)
		if !bytes.Equal(ack, []byte{0xc0, 0x21, 2, 0x73, 0, 10, 5, 6, 0x55, 0x66, 0x77, 0x88}) {
			t.Fatalf("Ack of the renegotiation request = %x", ack)
		}
		// RFC 1661 Section 4.1, Ack-Sent/RCR+: the peer revises its request
		// to require CHAP, and the client acknowledges it in turn.
		options := exchangePeerOptions(ppp.ProtoCHAP)
		w.send(ppp.ProtoLCP, ppp.LCPConfigureRequest, 0x74, options)
		if ack := w.take(t, ppp.ProtoLCP, ppp.LCPConfigureAck); !bytes.Equal(ack, exchangeFrame(ppp.ProtoLCP, 2, 0x74, options)) {
			t.Fatalf("Ack of the revised request = %x", ack)
		}
		w.send(ppp.ProtoLCP, ppp.LCPConfigureAck, request[3], request[6:])
		w.authenticate(t, ppp.ProtoCHAP, true)
		w.openIPCP(t, initial.ipcpIdentifier, 0)
		if w.passes() != 0 {
			t.Fatalf("IPv4 passed before the interface applied the result: %v", w.modes)
		}
		got := applyPending(t, s)
		if got.LocalIP != netip.MustParseAddr("198.51.100.2") {
			t.Fatalf("recovered local address = %v", got.LocalIP)
		}
		if w.passes() != 1 {
			t.Fatalf("forwarding not restored after recovery: %v", w.modes)
		}
		if s.current.chap == nil {
			t.Fatal("re-authentication did not leave CHAP answering in the network phase")
		}
		select {
		case <-done:
			t.Fatal("the recovered session ended")
		default:
		}
	})
}

// TestDualStackLCPRestartTakesIPv6Down proves an LCP restart takes IPv6CP down
// before the client's first LCP packet, both when IPv6CP had opened (IPv6 back
// to DROP, automaton no longer Opened) and while it was still negotiating
// (automaton no longer running).
// Break: delete the session.ipv6 lowerDown block in keepaliveLoop's reenter;
// the opened case keeps IPv6 passing during LCP and the negotiating case keeps
// IPv6CP running, both red.
func TestDualStackLCPRestartTakesIPv6Down(t *testing.T) {
	for _, opened := range []bool{true, false} {
		name := "negotiating"
		if opened {
			name = "opened"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w := newConnectedExchange()
				s, _ := w.session(t)
				var c *clientIPv6CP
				if opened {
					c, _, _ = openedIPv6CP(t)
				} else {
					c, _, _ = startedIPv6CP(t)
				}
				setIPv6 := func(mode ppp.NPMode) error {
					if mode == ppp.NPModeDrop {
						w.events = append(w.events, "drop6")
					} else {
						w.events = append(w.events, "pass6")
					}
					return nil
				}
				if err := s.enableIPv6CP(c, setIPv6, slogutil.DiscardLogger()); err != nil {
					t.Fatal(err)
				}
				for len(s.changes) != 0 {
					<-s.changes
				}
				stop, done := make(chan struct{}), make(chan struct{})
				defer close(stop)
				go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
				synctest.Wait()
				before := len(w.events)
				w.send(ppp.ProtoLCP, ppp.LCPConfigureRequest, 91, exchangePeerOptions(0))
				w.take(t, ppp.ProtoLCP, ppp.LCPConfigureRequest)
				events := w.events[before:]
				write := slices.Index(events, "write")
				if write < 0 {
					t.Fatalf("no LCP write recorded: %v", events)
				}
				if opened {
					if !slices.Contains(events[:write], "drop6") {
						t.Fatalf("IPv6 not discarded before the LCP restart wrote: %v", events)
					}
					if c.opened() {
						t.Fatal("IPv6CP still Opened during LCP")
					}
				}
				if c.running() {
					t.Fatal("IPv6CP still running during LCP")
				}
			})
		})
	}
}
