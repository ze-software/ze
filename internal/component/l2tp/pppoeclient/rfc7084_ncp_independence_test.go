// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- IPv6CP in the network phase
// Related: ipv6cp_client.go -- the automaton and its session glue under test
// Related: ipcp_independent.go -- connected IPCP, which closes and reopens on its own

package pppoeclient

import (
	"bytes"
	"net/netip"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// independenceSession is a connected session whose mode controls record each
// call, so a test sees which protocol an IPv6CP outcome touched.
func independenceSession(t *testing.T, c *clientIPv6CP, v4, v6 *[]ppp.NPMode) *connectedSession {
	t.Helper()
	return &connectedSession{
		transport: discardRWC{},
		current: negotiatedSession{
			localIP: netip.MustParseAddr("100.64.0.2"), peerIP: netip.MustParseAddr("100.64.0.1"), negMTU: 1492,
		},
		setIPv4Mode: func(mode ppp.NPMode) error { *v4 = append(*v4, mode); return nil },
		setIPv6Mode: func(mode ppp.NPMode) error { *v6 = append(*v6, mode); return nil },
		ipv6:        c,
		changes:     make(chan iface.PPPoEResult, 1),
	}
}

// openedIPv6CP drives an automaton to Opened through a full exchange.
func openedIPv6CP(t *testing.T) (*clientIPv6CP, *ipv6cpWire, []byte) {
	t.Helper()
	c, w, buf := startedIPv6CP(t)
	logger := slogutil.DiscardLogger()
	ours := lastIPv6CP(t, w)
	peer := ppp.LCPPacket{Code: ppp.LCPConfigureRequest, Identifier: 1, Data: ipv6cpOption(ppp.IPv6CPInterfaceID{0, 0, 0, 0, 0, 0, 0, 1})}
	if _, err := c.handle(w, buf, peer, logger); err != nil {
		t.Fatalf("handle request: %v", err)
	}
	ack := ppp.LCPPacket{Code: ppp.LCPConfigureAck, Identifier: ours.Identifier, Data: ours.Data}
	if event, err := c.handle(w, buf, ack, logger); err != nil || event != ipv6cpOpened {
		t.Fatalf("event %d err %v, want opened", event, err)
	}
	return c, w, buf
}

// TestRFC7084WLL3NCPIndependence checks AC-3 and AC-4: an access
// concentrator that Protocol-Rejects IPv6CP, Configure-Rejects its
// identifier, ignores it, or closes it while IPCP is Opened never touches
// IPv4. Method: drive each outcome through the automaton and the session
// glue, and assert IPv4's mode control is never called while the published
// result keeps the IPCP addresses. This is the IPv6CP half of RFC 7084 WLL-3
// (RFC7084-4.2-10); TestRFC7084WLL3IPCPIndependence proves the IPCP half.
//
// RFC requirement: RFC7084-4.2-10 positive -- an IPv6CP that is
// Protocol-Rejected or terminated while Opened sets only IPv6 to discard and
// keeps the published IPv4 address, and one whose identifier is rejected or
// that is ignored stops after its own requests, writing only IPv6CP.
func TestRFC7084WLL3NCPIndependence(t *testing.T) {
	logger := slogutil.DiscardLogger()

	t.Run("protocol-reject-while-opened", func(t *testing.T) {
		c, _, _ := openedIPv6CP(t)
		var v4, v6 []ppp.NPMode
		s := independenceSession(t, c, &v4, &v6)
		if err := s.applyIPv6CP(ipv6cpOpened, logger); err != nil {
			t.Fatalf("apply opened: %v", err)
		}
		<-s.changes
		if err := s.applyIPv6CP(c.protocolRejected(), logger); err != nil {
			t.Fatalf("apply rejected: %v", err)
		}
		if len(v4) != 0 {
			t.Fatalf("IPv4 mode changed %v by an IPv6CP outcome", v4)
		}
		if len(v6) != 2 || v6[1] != ppp.NPModeDrop {
			t.Fatalf("IPv6 modes %v, want pass then drop", v6)
		}
		result := <-s.changes
		if result.LocalIP != s.current.localIP {
			t.Fatalf("published IPv4 %v, want %v kept", result.LocalIP, s.current.localIP)
		}
		if result.LinkLocal.IsValid() {
			t.Fatalf("link-local %v still published after IPv6CP closed", result.LinkLocal)
		}
	})

	t.Run("identifier-rejected", func(t *testing.T) {
		c, w, buf := startedIPv6CP(t)
		ours := lastIPv6CP(t, w)
		before := len(w.frames)
		reject := ppp.LCPPacket{Code: ppp.LCPConfigureReject, Identifier: ours.Identifier, Data: ours.Data}
		event, err := c.handle(w, buf, reject, logger)
		if err != nil {
			t.Fatalf("handle Reject: %v", err)
		}
		if event != ipv6cpUnchanged {
			t.Fatalf("event %d, want unchanged", event)
		}
		if c.state != ppp.LCPStateStopped {
			t.Fatalf("state %d, want Stopped", c.state)
		}
		if len(w.frames) != before {
			t.Fatal("the client wrote a frame after its identifier was rejected")
		}
		if err := c.tick(w, buf, logger); err != nil {
			t.Fatalf("tick: %v", err)
		}
		if len(w.frames) != before {
			t.Fatal("a stopped IPv6CP retransmitted its request")
		}
	})

	t.Run("ignored", func(t *testing.T) {
		c, w, buf := startedIPv6CP(t)
		for range 3 * ipv6cpConfigureMax {
			if err := c.tick(w, buf, logger); err != nil {
				t.Fatalf("tick: %v", err)
			}
		}
		if len(w.frames) != ipv6cpConfigureMax {
			t.Fatalf("%d Configure-Requests sent to a silent peer, want Max-Configure %d", len(w.frames), ipv6cpConfigureMax)
		}
		if c.state != ppp.LCPStateStopped {
			t.Fatalf("state %d, want Stopped", c.state)
		}
		for i := range w.frames {
			proto, _, _, err := ppp.ParseFrame(w.frames[i])
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if proto != ppp.ProtoIPv6CP {
				t.Fatalf("frame %d protocol 0x%04x: only IPv6CP may be written", i, proto)
			}
		}
	})

	t.Run("terminate-while-opened", func(t *testing.T) {
		c, w, buf := openedIPv6CP(t)
		event, err := c.handle(w, buf, ppp.LCPPacket{Code: ppp.LCPTerminateRequest, Identifier: 7}, logger)
		if err != nil {
			t.Fatalf("handle Terminate-Request: %v", err)
		}
		if event != ipv6cpClosed {
			t.Fatalf("event %d, want closed", event)
		}
		reply := lastIPv6CP(t, w)
		if reply.Code != ppp.LCPTerminateAck {
			t.Fatalf("code %d, want Terminate-Ack", reply.Code)
		}
		var v4, v6 []ppp.NPMode
		s := independenceSession(t, c, &v4, &v6)
		if err := s.applyIPv6CP(event, logger); err != nil {
			t.Fatalf("apply: %v", err)
		}
		if len(v4) != 0 {
			t.Fatalf("IPv4 mode changed %v when IPv6CP closed", v4)
		}
	})
}

// ipcpIndependence builds a connected exchange whose IPv6CP is Opened when
// withIPv6 is set, recording the IPv6 modes in v6.
func ipcpIndependence(t *testing.T, withIPv6 bool, v6 *[]ppp.NPMode) (*connectedExchange, *connectedSession, chan struct{}) {
	t.Helper()
	w := newConnectedExchange()
	s, _ := w.session(t)
	if withIPv6 {
		c, _, _ := openedIPv6CP(t)
		record := func(mode ppp.NPMode) error { *v6 = append(*v6, mode); return nil }
		if err := s.enableIPv6CP(c, record, slogutil.DiscardLogger()); err != nil {
			t.Fatalf("enable IPv6CP: %v", err)
		}
		<-s.changes
	}
	return w, s, make(chan struct{})
}

// pendingResult takes the one published result without blocking: a blocked
// receive would let synctest advance the echo ticker.
func pendingResult(t *testing.T, s *connectedSession) iface.PPPoEResult {
	t.Helper()
	select {
	case result := <-s.changes:
		return result
	default:
		t.Fatal("no result published")
	}
	return iface.PPPoEResult{}
}

// workerEnded reports whether keepaliveLoop has returned.
func workerEnded(done chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

// TestRFC7084WLL3IPCPIndependence checks the IPCP half of RFC 7084 WLL-3:
// IPCP terminates and starts again on its own while LCP and IPv6CP stay up,
// and the session ends only when no NCP is left. Method: drive the real
// keepaliveLoop with scripted IPCP and IPv6CP frames under synctest, and
// assert each mode change, the frames written, the published result and
// whether the worker still runs.
//
// RFC requirement: RFC7084-4.2-10 positive -- with IPv6CP Opened, an IPCP
// Terminate-Request is answered with a Terminate-Ack, IPv4 alone is set to
// discard and withdrawn from the published result while the link-local
// address stays, the session goes on, and a later IPCP Configure-Request
// opens IPCP again and passes IPv4 with no LCP frame and no IPv6 mode change.
// RFC requirement: RFC7084-4.2-10 negative -- an IPCP Terminate-Request on a
// session with no IPv6CP, or an IPv6CP Terminate-Request while IPCP is
// Stopped, ends the session, because no NCP is left on the link.
func TestRFC7084WLL3IPCPIndependence(t *testing.T) {
	t.Run("ipcp-terminates-and-reopens-alone", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var v6 []ppp.NPMode
			w, s, done := ipcpIndependence(t, true, &v6)
			stop := make(chan struct{})
			defer close(stop)
			go keepaliveLoop(s, done, stop, slogutil.DiscardLogger())
			synctest.Wait()
			linkLocal := s.linkLocal

			w.send(ppp.ProtoIPCP, ppp.LCPTerminateRequest, 21, nil)
			ack := w.take(t, ppp.ProtoIPCP, ppp.LCPTerminateAck)
			if ack[3] != 21 {
				t.Fatalf("Terminate-Ack Identifier %d, want 21", ack[3])
			}
			w.quiet(t)
			if workerEnded(done) {
				t.Fatal("an IPCP Terminate-Request ended the session while IPv6CP is Opened")
			}
			if len(w.modes) != 1 || w.modes[0] != ppp.NPModeDrop {
				t.Fatalf("IPv4 modes %v, want one drop", w.modes)
			}
			if len(v6) != 1 || v6[0] != ppp.NPModePass {
				t.Fatalf("IPv6 modes %v, want the one pass IPv6CP opened with", v6)
			}
			down := pendingResult(t, s)
			if down.LocalIP.IsValid() || down.PeerIP.IsValid() {
				t.Fatalf("IPv4 %v/%v still published after IPCP terminated", down.LocalIP, down.PeerIP)
			}
			if down.PrimaryDNS.IsValid() || down.SecondaryDNS.IsValid() {
				t.Fatal("IPCP DNS servers still published after IPCP terminated")
			}
			if !linkLocal.IsValid() || down.LinkLocal != linkLocal {
				t.Fatalf("link-local %v, want %v kept", down.LinkLocal, linkLocal)
			}

			// RFC 1661 Section 4.1, Stopped: RCR takes irc, scr, sca.
			peer := []byte{3, 6, 198, 51, 100, 1}
			w.send(ppp.ProtoIPCP, ppp.LCPConfigureRequest, 95, peer)
			request := w.take(t, ppp.ProtoIPCP, ppp.LCPConfigureRequest)
			if !bytes.Equal(request[6:], ipcpRequestFor([]byte{3, 6, 0, 0, 0, 0})) {
				t.Fatalf("reopening IPCP request %x", request)
			}
			if got := w.take(t, ppp.ProtoIPCP, ppp.LCPConfigureAck); !bytes.Equal(got, exchangeFrame(ppp.ProtoIPCP, ppp.LCPConfigureAck, 95, peer)) {
				t.Fatalf("peer IPCP Ack %x", got)
			}
			w.send(ppp.ProtoIPCP, ppp.LCPConfigureNak, request[3], []byte{3, 6, 198, 51, 100, 2})
			request = w.take(t, ppp.ProtoIPCP, ppp.LCPConfigureRequest)
			w.send(ppp.ProtoIPCP, ppp.LCPConfigureAck, request[3], request[6:])
			w.quiet(t)
			if workerEnded(done) {
				t.Fatal("reopening IPCP ended the session")
			}
			if len(w.modes) != 2 || w.modes[1] != ppp.NPModePass {
				t.Fatalf("IPv4 modes %v, want drop then pass", w.modes)
			}
			if len(v6) != 1 {
				t.Fatalf("IPv6 modes %v changed while IPCP reopened", v6)
			}
			up := pendingResult(t, s)
			if up.LocalIP != netip.MustParseAddr("198.51.100.2") {
				t.Fatalf("reopened local address %v", up.LocalIP)
			}
			if up.PeerIP != netip.MustParseAddr("198.51.100.1") {
				t.Fatalf("reopened peer address %v", up.PeerIP)
			}
			if up.LinkLocal != linkLocal {
				t.Fatalf("link-local %v, want %v kept", up.LinkLocal, linkLocal)
			}
		})
	})

	t.Run("no-ipv6cp-ends-the-session", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			w, s, done := ipcpIndependence(t, false, nil)
			stop := make(chan struct{})
			defer close(stop)
			go keepaliveLoop(s, done, stop, slogutil.DiscardLogger())
			synctest.Wait()
			w.send(ppp.ProtoIPCP, ppp.LCPTerminateRequest, 21, nil)
			w.take(t, ppp.ProtoIPCP, ppp.LCPTerminateAck)
			synctest.Wait()
			if !workerEnded(done) {
				t.Fatal("the session went on with no NCP left")
			}
		})
	})

	t.Run("ipv6cp-closing-while-ipcp-stopped-ends-the-session", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var v6 []ppp.NPMode
			w, s, done := ipcpIndependence(t, true, &v6)
			stop := make(chan struct{})
			defer close(stop)
			go keepaliveLoop(s, done, stop, slogutil.DiscardLogger())
			synctest.Wait()
			w.send(ppp.ProtoIPCP, ppp.LCPTerminateRequest, 21, nil)
			w.take(t, ppp.ProtoIPCP, ppp.LCPTerminateAck)
			synctest.Wait()
			if workerEnded(done) {
				t.Fatal("an IPCP Terminate-Request ended the session while IPv6CP is Opened")
			}
			w.send(ppp.ProtoIPv6CP, ppp.LCPTerminateRequest, 33, nil)
			w.take(t, ppp.ProtoIPv6CP, ppp.LCPTerminateAck)
			synctest.Wait()
			if !workerEnded(done) {
				t.Fatal("the session went on with IPCP Stopped and IPv6CP closed")
			}
			if len(v6) != 2 || v6[1] != ppp.NPModeDrop {
				t.Fatalf("IPv6 modes %v, want pass then drop", v6)
			}
		})
	})
}
