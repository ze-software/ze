// VALIDATES: every PEER-driven session teardown emits (l2tp, session-down)
// with the RFC 2866 Section 5.10 cause that is true of it. The subscriber is
// the party that hangs up in the ordinary case, so these paths carry the bulk
// of a production LNS's Accounting-Stop records.
// PREVENTS: the third and worst instance of a teardown that tells only the
// route observer. A CDN the peer sent, a StopCCN the peer sent, and the
// dead-peer keepalive each removed the session from the tunnel map in silence,
// so RADIUS sent no Accounting-Stop, the pool released no address and the
// shaper dropped no rules for a subscriber who simply hung up. The assertions
// read the EVENT BUS rather than the route observer, because the route
// observer already fired on all three paths and that is what hid the defect
// through two earlier rounds.

package l2tp

import (
	"log/slog"
	"net/netip"
	"testing"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// newReactorForPeerTeardown builds a reactor that can run handle and
// handleTick: both push a deadline onto updateCh and select on stop, so a
// reactor with nil channels would block forever.
func newReactorForPeerTeardown(t *testing.T, bus *causeBus) *l2tpReactor {
	t.Helper()
	r := newReactorForSnapshot(t)
	r.listener = newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), slog.Default())
	r.eventBus = bus
	r.updateCh = make(chan heapUpdate, 16)
	r.stop = make(chan struct{})
	t.Cleanup(func() { close(r.stop) })
	return r
}

// controlDatagram frames one L2TPv2 control message: the 12-byte header of
// RFC 2661 Section 3.1 followed by the AVP body the caller built.
func controlDatagram(tunnelID, sessionID, ns, nr uint16, body []byte) []byte {
	pkt := make([]byte, ControlHeaderLen+len(body))
	copy(pkt[ControlHeaderLen:], body)
	WriteControlHeader(pkt, 0, uint16(len(pkt)), tunnelID, sessionID, ns, nr)
	return pkt
}

// TestPeerCDNEmitsSessionDownWithCause drives a real CDN datagram through the
// reactor's inbound path and reads the cause off the event bus.
//
// The mapping under test is RFC 2661 Section 4.4.2's CDN Result Codes onto RFC
// 2866 Section 5.10's Acct-Terminate-Cause values. Only two of the twelve CDN
// codes state something Section 5.10 also states; the rest describe a call
// setup the LAC could not complete, and ze reports the general value for them
// rather than inventing a specific one.
func TestPeerCDNEmitsSessionDownWithCause(t *testing.T) {
	cases := []struct {
		name       string
		resultCode uint16
		want       l2tpevents.TerminateCause
	}{
		{"loss-of-carrier", 1, l2tpevents.TerminateCauseLostCarrier},
		{"administrative", 3, l2tpevents.TerminateCauseAdminReset},
		{"error-code", 2, l2tpevents.TerminateCauseNASError},
		{"no-facilities", 4, l2tpevents.TerminateCauseNASError},
		{"reserved", 0, l2tpevents.TerminateCauseNASError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bus := newCauseBus()
			r := newReactorForPeerTeardown(t, bus)
			peer := netip.MustParseAddrPort("10.0.0.9:1701")
			insertEstablishedTunnel(t, r, 11, 22, peer,
				&L2TPSession{localSID: 7, remoteSID: 70, state: L2TPSessionEstablished, username: "grace"})

			r.handle(rxPacket{
				from:    peer,
				bytes:   controlDatagram(11, 7, 0, 0, buildCDN(tc.resultCode, 70)),
				release: func() {},
			})

			events := bus.downEvents()
			if len(events) != 1 {
				t.Fatalf("session-down events = %d, want 1", len(events))
			}
			if events[0].SessionID != 7 || events[0].TunnelID != 11 {
				t.Errorf("event = tunnel %d session %d, want 11/7",
					events[0].TunnelID, events[0].SessionID)
			}
			if events[0].Username != "grace" {
				t.Errorf("username = %q, want %q", events[0].Username, "grace")
			}
			if events[0].Cause != tc.want {
				t.Errorf("cause = %d, want %d", events[0].Cause, tc.want)
			}
		})
	}
}

// TestPeerCDNMalformedEmitsNASError proves the branch where ze has no Result
// Code to read. handleCDN destroys the session whatever the body says, and a
// destroyed session still owes its subscribers an event.
//
// The body carries a well-formed Message Type AVP naming CDN, so dispatch
// reaches handleCDN, followed by an AVP header whose length runs past the
// payload. RFC 2661 Section 4.1 makes that malformed, the iterator stops, and
// parseCDN returns its zero value.
func TestPeerCDNMalformedEmitsNASError(t *testing.T) {
	bus := newCauseBus()
	r := newReactorForPeerTeardown(t, bus)
	peer := netip.MustParseAddrPort("10.0.0.9:1701")
	insertEstablishedTunnel(t, r, 11, 22, peer,
		&L2TPSession{localSID: 7, remoteSID: 70, state: L2TPSessionEstablished, username: "grace"})

	var body [16]byte
	off := WriteAVPUint16(body[:], 0, true, AVPMessageType, uint16(MsgCDN))
	// An AVP header claiming 40 octets with 4 present.
	body[off], body[off+1] = 0x80, 0x28
	body[off+2], body[off+3] = 0x00, 0x00
	off += 4

	r.handle(rxPacket{
		from:    peer,
		bytes:   controlDatagram(11, 7, 0, 0, body[:off]),
		release: func() {},
	})

	events := bus.downEvents()
	if len(events) != 1 {
		t.Fatalf("session-down events = %d, want 1", len(events))
	}
	if events[0].Cause != l2tpevents.TerminateCauseNASError {
		t.Errorf("cause = %d, want %d", events[0].Cause, l2tpevents.TerminateCauseNASError)
	}
}

// TestPeerStopCCNEmitsSessionDownPerSession proves the tunnel-scoped peer
// teardown reaches every subscriber the tunnel carried.
//
// RFC 2661 Section 6.4 clears all sessions implicitly on a StopCCN, so the
// only record of who was on the tunnel is the session map the clear is about
// to drop. The cause is Lost Service: the transport under every one of those
// sessions is gone whatever Result Code the peer stated.
func TestPeerStopCCNEmitsSessionDownPerSession(t *testing.T) {
	bus := newCauseBus()
	r := newReactorForPeerTeardown(t, bus)
	peer := netip.MustParseAddrPort("10.0.0.9:1701")
	insertEstablishedTunnel(t, r, 11, 22, peer,
		&L2TPSession{localSID: 7, remoteSID: 70, state: L2TPSessionEstablished, username: "grace"},
		&L2TPSession{localSID: 8, remoteSID: 80, state: L2TPSessionEstablished, username: "ada"})

	r.handle(rxPacket{
		from:    peer,
		bytes:   buildStopCCN(t, 11, 0, 0, 22, 1),
		release: func() {},
	})

	events := bus.downEvents()
	if len(events) != 2 {
		t.Fatalf("session-down events = %d, want 2", len(events))
	}
	byUsername := map[uint16]string{}
	for _, e := range events {
		if e.Cause != l2tpevents.TerminateCauseLostService {
			t.Errorf("cause = %d, want %d", e.Cause, l2tpevents.TerminateCauseLostService)
		}
		byUsername[e.SessionID] = e.Username
	}
	if byUsername[7] != "grace" || byUsername[8] != "ada" {
		t.Errorf("usernames = %v, want 7:grace 8:ada", byUsername)
	}
}

// TestDeadPeerKeepaliveEmitsSessionDown proves the keepalive timeout tells the
// subscribers. A LAC that dies without sending StopCCN is the failure this
// detector exists for, so its sessions would otherwise stay billed until an
// operator noticed.
//
// The cause is Lost Carrier, which is what ze already reports when LCP echo
// probes stop being answered one layer up: an unanswered keepalive is how a
// virtual link says its carrier is gone.
func TestDeadPeerKeepaliveEmitsSessionDown(t *testing.T) {
	bus := newCauseBus()
	r := newReactorForPeerTeardown(t, bus)
	r.params.HelloInterval = time.Second
	r.params.HelloRetries = 3
	peer := netip.MustParseAddrPort("10.0.0.9:1701")
	insertEstablishedTunnel(t, r, 11, 22, peer,
		&L2TPSession{localSID: 7, remoteSID: 70, state: L2TPSessionEstablished, username: "grace"})

	// Liveness last proven well beyond HelloRetries * HelloInterval.
	r.tunnelsMu.Lock()
	tun := r.tunnelsByLocalID[11]
	tun.lastLiveness = r.params.Clock().Add(-time.Minute)
	tun.lastActivity = r.params.Clock().Add(-time.Minute)
	r.tunnelsMu.Unlock()

	r.handleTick(tickReq{tunnelID: 11})

	events := bus.downEvents()
	if len(events) != 1 {
		t.Fatalf("session-down events = %d, want 1", len(events))
	}
	if events[0].Cause != l2tpevents.TerminateCauseLostCarrier {
		t.Errorf("cause = %d, want %d", events[0].Cause, l2tpevents.TerminateCauseLostCarrier)
	}
	if events[0].Username != "grace" {
		t.Errorf("username = %q, want %q", events[0].Username, "grace")
	}
}
