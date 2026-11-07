// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- IPv6CP in the network phase
// Related: ipv6cp_client.go -- the automaton and its session glue under test

package pppoeclient

import (
	"net/netip"
	"testing"

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
// result keeps the IPCP addresses.
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
