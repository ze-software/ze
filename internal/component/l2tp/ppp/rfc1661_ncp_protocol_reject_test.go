// Design: docs/research/l2tpv2-ze-integration.md -- independent NCP lifetimes
// Related: session_run.go -- receiveProtocolReject and the LCP-Opened gate in handleLCPPacket
// Related: ncp.go -- stopRejectedNCP, ncpSettled; ncp_restart.go -- ncpPending
// Related: ncp_helpers_test.go -- the pipe driver; ncp_lifetime_test.go -- awaitDualStackSessionUp
// Related: rfc/short/rfc1661.md -- RFC1661-5.7-2 and RFC1661-5.7-5
//
// Goal: prove, through the session's real run loop, that a Protocol-Reject
// naming IPCP or IPv6CP received in LCP Opened stops that NCP alone, so the
// session comes up, or stays up, with the other family; and that one received
// while LCP is not Opened is discarded before the LCP automaton sees it.
//
// Method: the pipe driver (newNCPTestDriverUnit) runs the session against a
// peer that plays pppd with one NCP disabled: pppd v2.5.5 main.c get_input
// finds no enabled protent for the protocol and answers with lcp_sprotrej.
// Everything is read where it crosses the session boundary: frames on the
// channel pipe, events on the driver's channel, address removals on the fake
// backend's locked recorder. No session field is read from the test
// goroutine. The LCP-gate test runs on the synctest lifecycle fixture
// (runLifecycleSession), where the LCP state is observed on the wire.

package ppp

import (
	"encoding/binary"
	"errors"
	"net"
	"os"
	"testing"
	"testing/synctest"
	"time"
)

// The Restart time is short so a window of a few Restart times, and of the
// ncp timeout, fits in a unit test; Max-Configure 10 keeps a Configure-Request
// that is never answered retransmitting past that ncp timeout.
const (
	protocolRejectRestart   = 100 * time.Millisecond
	protocolRejectIPTimeout = time.Second
)

// newProtocolRejectDriver starts the pipe driver with both NCPs enabled, the
// short Restart time and the short ncp timeout. unit is nil for a test that
// never writes the unit descriptor.
func newProtocolRejectDriver(t *testing.T, unit net.Conn) *ncpTestDriver {
	t.Helper()
	config, err := NewLCPConfig(protocolRejectRestart, 2, 10, 5)
	if err != nil {
		t.Fatal(err)
	}
	start := &StartSession{IPTimeout: protocolRejectIPTimeout, LCP: config}
	// A nil net.Conn converts to a nil io.ReadCloser, which keeps the unit
	// unread as newNCPTestDriverUnit documents.
	return newNCPTestDriverUnit(t, start, autoAcceptIP, discardLogger(), unit)
}

// protocolRejectOf builds the Data of an LCP Protocol-Reject that rejects pkt
// of protocol proto. RFC 1661 Section 5.7: octets 0-1 of Data are the
// Rejected-Protocol, then the Rejected-Information, "a copy of the packet
// which is being rejected", here pkt's Code, Identifier, Length and Data.
func protocolRejectOf(proto uint16, pkt LCPPacket) []byte {
	data := binary.BigEndian.AppendUint16(nil, proto)
	data = append(data, pkt.Code, pkt.Identifier)
	data = binary.BigEndian.AppendUint16(data, uint16(lcpHeaderLen+len(pkt.Data)))
	return append(data, pkt.Data...)
}

// peerNCPRequest is the option list of the peer's own Configure-Request for
// family: the address autoAcceptIP assigns, or the peer's interface identifier.
func peerNCPRequest(family AddressFamily) []byte {
	if family == AddressFamilyIPv4 {
		peer := ipcpTestPeer.As4()
		return append([]byte{IPCPOptIPAddress, 6}, peer[:]...)
	}
	return append([]byte{1, 10}, ipv6cpTestPeerID[:]...)
}

// runProtocolRejectingPeer plays a peer that negotiates the NCP of keep and
// answers the first packet of the NCP of reject with one LCP Protocol-Reject
// naming it. It returns once Ze acknowledged the peer's keep Configure-Request
// and the rejection is sent, reports through t.Errorf any packet of reject's
// NCP that Ze sends after the rejection and before that point, and closes
// done when it returns.
func runProtocolRejectingPeer(t *testing.T, conn net.Conn, keep, reject AddressFamily, done chan<- struct{}) {
	defer close(done)
	keepAcked, keepRequested, rejected := false, false, false
	buf := make([]byte, MaxFrameLen)
	deadline := time.Now().Add(3 * time.Second)
	for !keepAcked || !rejected {
		if err := conn.SetReadDeadline(deadline); err != nil {
			t.Errorf("peer SetReadDeadline: %v", err)
			return
		}
		n, err := conn.Read(buf)
		if err != nil {
			t.Errorf("peer read (keep acked %v, rejected %v): %v", keepAcked, rejected, err)
			return
		}
		proto, payload, _, err := ParseFrame(buf[:n])
		if err != nil {
			t.Errorf("peer ParseFrame: %v", err)
			return
		}
		pkt, err := ParseLCPPacket(payload)
		if err != nil {
			t.Errorf("peer ParseLCPPacket: %v", err)
			return
		}
		switch proto {
		case ncpProto(keep):
			switch pkt.Code {
			case LCPConfigureRequest:
				writePeerNCPFrame(t, conn, proto, LCPConfigureAck, pkt.Identifier, pkt.Data)
				if !keepRequested {
					writePeerNCPFrame(t, conn, proto, LCPConfigureRequest, 0x40, peerNCPRequest(keep))
					keepRequested = true
				}
			case LCPConfigureAck:
				keepAcked = true
			}
		case ncpProto(reject):
			if rejected {
				t.Errorf("Ze sent %s Code %d after the peer Protocol-Rejected it", reject, pkt.Code)
				return
			}
			writePeerNCPFrame(t, conn, ProtoLCP, LCPProtocolReject, 0x61, protocolRejectOf(proto, pkt))
			rejected = true
		}
	}
}

// awaitSessionUpFamilies acknowledges each assignment as the transport would
// and returns the families assigned once EventSessionUp arrives, failing on
// EventSessionDown, which is how the ncp timeout ends a session whose NCP
// phase never settles.
func awaitSessionUpFamilies(t *testing.T, td *ncpTestDriver) map[AddressFamily]EventSessionIPAssigned {
	t.Helper()
	assigned := make(map[AddressFamily]EventSessionIPAssigned, 2)
	deadline := time.After(3 * time.Second)
	for {
		select {
		case event := <-td.driver.EventsOut():
			switch ev := event.(type) {
			case EventSessionIPAssigned:
				ev.Acknowledge()
				assigned[ev.Family] = ev
			case EventSessionUp:
				return assigned
			case EventSessionDown:
				t.Fatalf("session ended before coming up: %s", ev.Reason)
			}
		case <-deadline:
			t.Fatal("no EventSessionUp")
		}
	}
}

// expectNoProtocolOnChannel reads the channel for window and fails on any
// frame of proto. Frames of other protocols are let pass.
func expectNoProtocolOnChannel(t *testing.T, peer net.Conn, proto uint16, window time.Duration) {
	t.Helper()
	end := time.Now().Add(window)
	buf := make([]byte, MaxFrameBufLen)
	for {
		if err := peer.SetReadDeadline(end); err != nil {
			t.Fatal(err)
		}
		n, err := peer.Read(buf)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return
		}
		if err != nil {
			t.Fatalf("peer read: %v", err)
		}
		got, payload, _, err := ParseFrame(buf[:n])
		if err != nil {
			t.Fatalf("Ze sent a malformed frame: %v", err)
		}
		if got == proto {
			t.Fatalf("Ze sent protocol %#04x (% x) after the peer Protocol-Rejected it", proto, payload)
		}
	}
}

// expectNoSessionDown fails if the driver has published EventSessionDown, and
// fails on any assignment of family. Other events are let pass.
func expectNoSessionDown(t *testing.T, td *ncpTestDriver, family AddressFamily) {
	t.Helper()
	for {
		select {
		case event := <-td.driver.EventsOut():
			switch ev := event.(type) {
			case EventSessionDown:
				t.Fatalf("session ended after the Protocol-Reject: %s", ev.Reason)
			case EventSessionIPAssigned:
				ev.Acknowledge()
				if ev.Family == family {
					t.Fatalf("Ze assigned the Protocol-Rejected %s family", family)
				}
			}
		default:
			return
		}
	}
}

// protocolRejectKeepsOtherFamily is the body of AC-2 and AC-3: the peer
// Protocol-Rejects the NCP of reject during the NCP phase and negotiates the
// NCP of keep. The session comes up with keep alone before the ncp timeout,
// stays up past it, and Ze sends no packet of reject's NCP, through several
// Restart times, after the rejection.
func protocolRejectKeepsOtherFamily(t *testing.T, keep, reject AddressFamily) {
	t.Helper()
	started := time.Now()
	td := newProtocolRejectDriver(t, nil)
	peerDone := make(chan struct{})
	go runProtocolRejectingPeer(t, td.peer, keep, reject, peerDone)

	assigned := awaitSessionUpFamilies(t, td)
	if elapsed := time.Since(started); elapsed >= protocolRejectIPTimeout {
		t.Fatalf("session came up %s after the start, not before the ncp timeout %s", elapsed, protocolRejectIPTimeout)
	}
	<-peerDone
	if t.Failed() {
		return
	}
	if _, ok := assigned[keep]; !ok {
		t.Fatalf("session came up with no %s assignment", keep)
	}
	if _, ok := assigned[reject]; ok {
		t.Fatalf("session came up with the Protocol-Rejected %s family assigned", reject)
	}

	// The window spans the ncp timeout and many Restart times: a rejected
	// NCP left negotiating would retransmit within it, and a phase bound left
	// running would end the session within it.
	expectNoProtocolOnChannel(t, td.peer, ncpProto(reject), protocolRejectIPTimeout+5*protocolRejectRestart)
	expectNoSessionDown(t, td, reject)
}

// TestProtocolRejectIPv6CPKeepsIPv4Session proves AC-2: an IPv4-only peer
// that Protocol-Rejects Ze's IPv6CP Configure-Request, as pppd 2.5.5 run with
// noipv6 does, gets an IPv4 session as soon as IPCP opens, and Ze sends no
// further IPv6CP packet. Red before receiveProtocolReject existed: IPv6CP
// stayed in Req-Sent, retransmitting every Restart time, so the NCP phase
// never settled and the ncp timeout ended the session.
//
// RFC requirement: RFC1661-5.7-2 positive -- after the peer's LCP
// Protocol-Reject naming IPv6CP (0x8057) in LCP Opened, Ze sends no IPv6CP
// frame on the channel for the ncp timeout plus five Restart times, and the
// session comes up with IPv4 alone before the ncp timeout.
//
// RFC requirement: RFC1661-5.7-5 negative -- a Protocol-Reject received in
// LCP Opened is not discarded: it stops the NCP it names, so the NCP phase
// settles with IPCP Opened and the session comes up before the ncp timeout.
func TestProtocolRejectIPv6CPKeepsIPv4Session(t *testing.T) {
	protocolRejectKeepsOtherFamily(t, AddressFamilyIPv4, AddressFamilyIPv6)
}

// TestProtocolRejectIPCPKeepsIPv6Session proves AC-3: the mirror of AC-2, a
// peer that Protocol-Rejects IPCP and negotiates IPv6CP gets an IPv6 session.
// Red before receiveProtocolReject existed, for the same reason as AC-2.
//
// RFC requirement: RFC1661-5.7-2 negative -- the stop is tied to the protocol
// the Protocol-Reject indicates: with IPCP (0x8021) rejected, IPv6CP, which
// was not indicated, keeps negotiating and opens, the session comes up with
// its IPv6 assignment, and Ze sends no IPCP frame after the rejection.
func TestProtocolRejectIPCPKeepsIPv6Session(t *testing.T) {
	protocolRejectKeepsOtherFamily(t, AddressFamilyIPv6, AddressFamilyIPv4)
}

// TestProtocolRejectIgnoredOutsideLCPOpened proves AC-10: a Protocol-Reject
// naming IPCP received while LCP is in Ack-Rcvd changes neither LCP nor IPCP.
//
// Method: the synctest lifecycle fixture plays the peer. It acknowledges
// Ze's LCP Configure-Request (Ack-Rcvd), sends the Protocol-Reject, then its
// own Configure-Request. RFC 1661 Section 4.1 takes Ack-Rcvd on RCR+ to
// Opened (sca, tlu), so authentication starts at once and IPCP then
// negotiates and opens. Had the Protocol-Reject reached the automaton as RXJ+,
// Ack-Rcvd would have gone to Req-Sent and the peer's Configure-Request to
// Ack-Sent, with no authentication request until Ze's next Configure-Request
// was acknowledged. Red before the gate existed: the authentication request
// never arrives.
//
// RFC requirement: RFC1661-5.7-5 positive -- a Protocol-Reject naming IPCP
// received in LCP Ack-Rcvd is discarded: the peer's next Configure-Request
// takes LCP to Opened (authentication requested), and IPCP then negotiates
// and the session comes up.
func TestProtocolRejectIgnoredOutsideLCPOpened(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		session, _, events := newRFC1661Session(LCPStateInitial)
		session.ipTimeout = 30 * time.Second
		authEvents, ips := prepareIPCPLifecycle(session)
		peer := runLifecycleSession(t, session)

		request := readLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest)
		writeLifecyclePacket(t, peer, ProtoLCP, LCPConfigureAck, request.Identifier, request.Data)
		rejected := LCPPacket{Code: LCPConfigureRequest, Identifier: 1, Data: peerNCPRequest(AddressFamilyIPv4)}
		writeLifecyclePacket(t, peer, ProtoLCP, LCPProtocolReject, 0x61, protocolRejectOf(ProtoIPCP, rejected))
		writeLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest, 0x31, nil)
		if ack := readLifecyclePacket(t, peer, ProtoLCP, LCPConfigureAck); ack.Identifier != 0x31 {
			t.Fatalf("LCP Configure-Ack Identifier = %#02x, want the request's 0x31", ack.Identifier)
		}

		answerAuthRequestForTest(t, awaitAuthRequest(t, authEvents), authResponseMsg{accept: true})
		select {
		case event := <-ips:
			address, ok := event.(EventIPRequest)
			if !ok {
				t.Fatalf("address event %T, want EventIPRequest", event)
			}
			answerIPRequestForTest(t, address, ipResponseMsg{
				accept: true, family: AddressFamilyIPv4,
				local: restartLifetimeLocal, peer: restartLifetimePeer,
			})
		case <-time.After(5 * time.Second):
			t.Fatal("missing IPv4 address request")
		}
		ipcpRequest := readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest)
		openLifecycleIPCP(t, peer, ipcpRequest, events)
	})
}

// TestProtocolRejectAfterSessionUpWithdrawsFamily proves AC-5: after a
// dual-stack session came up, a Protocol-Reject naming the open IPv6CP takes
// IPv6CP through the Down event from Opened: the IPv6 family is withdrawn and
// released, IPv4 stays, and IPv6CP answers nothing more.
//
// Method: the release is read on the driver's events. A peer IPv6CP
// Configure-Request on the unit then probes Starting, which takes no receive
// event (RFC 1661 Section 4.1), and an IPCP packet of an unused Code orders
// it: its Code-Reject must be the first frame Ze sends, so no IPv6CP answer
// came before it. Red before receiveProtocolReject existed: RXJ+ left IPv6CP
// Opened and no release was published.
//
// RFC requirement: RFC1661-5.7-2 positive -- after the peer's Protocol-Reject
// naming the Opened IPv6CP, Ze publishes the IPv6 release, answers a later
// IPv6CP Configure-Request with nothing, and sends no IPv6CP frame for five
// Restart times, while the IPv4 address stays installed.
func TestProtocolRejectAfterSessionUpWithdrawsFamily(t *testing.T) {
	driverUnit, peerUnit := net.Pipe()
	t.Cleanup(func() { _ = peerUnit.Close() }) //nolint:errcheck // test cleanup

	td := newProtocolRejectDriver(t, driverUnit)
	peerDone := make(chan struct{})
	go runParallelNCPPeer(t, td.peer, true, true, peerDone)
	assigned := awaitDualStackSessionUp(t, td)
	<-peerDone
	removalsBefore := len(td.backend.AddrRemoveCalls())

	opened := LCPPacket{Code: LCPConfigureRequest, Identifier: 0x50, Data: peerNCPRequest(AddressFamilyIPv6)}
	td.writePeerNCPPacket(t, ProtoLCP, LCPProtocolReject, 0x62, protocolRejectOf(ProtoIPv6CP, opened))

	released := awaitFamilyRelease(t, td)
	if released.Family != AddressFamilyIPv6 {
		t.Fatalf("released family = %s, want ipv6", released.Family)
	}
	if released.NetworkLifetime != assigned[AddressFamilyIPv6].NetworkLifetime {
		t.Fatalf("release network lifetime = %+v, want the assignment's %+v",
			released.NetworkLifetime, assigned[AddressFamilyIPv6].NetworkLifetime)
	}

	writeUnitNCPPacket(t, peerUnit, ProtoIPv6CP, LCPConfigureRequest, 0x63, peerNCPRequest(AddressFamilyIPv6))
	// RFC 1332 Section 2: an unknown Code draws a Code-Reject, and its answer
	// orders every earlier frame.
	writeUnitNCPPacket(t, peerUnit, ProtoIPCP, unusedNCPCode, 0x64, []byte{1, 2, 3, 4})
	if reject := td.readPeerNCPPacket(t, ProtoIPCP); reject.Code != LCPCodeReject {
		t.Fatalf("IPCP answer has Code %d, want Code-Reject", reject.Code)
	}
	expectNoProtocolOnChannel(t, td.peer, ProtoIPv6CP, 5*protocolRejectRestart)
	expectNoSessionDown(t, td, AddressFamilyIPv6)

	for _, removal := range td.backend.AddrRemoveCalls()[removalsBefore:] {
		if removal.cidr == "10.0.0.1/32" {
			t.Fatalf("the IPv6 Protocol-Reject removed the IPv4 address: %+v", removal)
		}
	}
}
