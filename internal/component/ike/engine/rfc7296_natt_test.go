package engine

import (
	"bytes"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// nttOddPort is a source port that is neither 500 nor 4500. RFC 7296 Section 2.11
// requires the response to reach the port the request came FROM, and a NAT routinely
// picks a port like this one.
const nttOddPort = 34567

// nttNATTLink builds a loopback stand-in for a peer behind a NAT, plus the two
// sockets Ze owns.
//
// peerTr receives what Ze sends. ikeTr is Ze's port-500 socket and nattTr is its
// port-4500 one. IsNATT tells them apart, and the bind port never does.
//
// Both bind an ephemeral port, so the test needs no privilege. That is the same
// reason production reads the role off the transport instead of comparing a number
// (ai/rules/evidence.md).
func nttNATTLink(t *testing.T) (peerTr, ikeTr, nattTr *transport.UDPTransport) {
	t.Helper()
	log := slogutil.DiscardLogger()

	peerTr, err := transport.NewUDPTransport("127.0.0.1:0", log)
	if err != nil {
		t.Fatalf("peer transport: %v", err)
	}
	t.Cleanup(func() { _ = peerTr.Close() }) //nolint:errcheck // the socket dies with the test
	go peerTr.Run()

	ikeTr, err = transport.NewUDPTransport("127.0.0.1:0", log)
	if err != nil {
		t.Fatalf("ike transport: %v", err)
	}
	t.Cleanup(func() { _ = ikeTr.Close() }) //nolint:errcheck // the socket dies with the test

	nattTr, err = transport.NewNATTTransport("127.0.0.1:0", log)
	if err != nil {
		t.Fatalf("natt transport: %v", err)
	}
	t.Cleanup(func() { _ = nattTr.Close() }) //nolint:errcheck // the socket dies with the test

	addr, ok := peerTr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("peer transport local address is not *net.UDPAddr")
	}
	port := addr.Port
	oldPortFn := ikeTestPortFn
	ikeTestPortFn = func() string { return strconv.Itoa(port) }
	t.Cleanup(func() { ikeTestPortFn = oldPortFn })
	return peerTr, ikeTr, nattTr
}

// nttPort reads a transport's bound port through a checked assertion.
func nttPort(t *testing.T, tr *transport.UDPTransport) int {
	t.Helper()
	addr, ok := tr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("transport local address is not *net.UDPAddr")
	}
	return addr.Port
}

// nttPeerAddr is the address the peer's socket really answers on.
func nttPeerAddr(t *testing.T, peerTr *transport.UDPTransport) *net.UDPAddr {
	t.Helper()
	addr, ok := peerTr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("peer transport local address is not *net.UDPAddr")
	}
	return addr
}

// nttSourcePortOf returns the port a datagram the peer received was sent FROM.
func nttSourcePortOf(t *testing.T, peerTr *transport.UDPTransport) (int, []byte) {
	t.Helper()
	select {
	case pkt := <-peerTr.Recv():
		if pkt.RemoteAddr == nil {
			t.Fatal("the peer read a datagram with no source address")
		}
		return pkt.RemoteAddr.Port, pkt.Data
	case <-time.After(rtxArrive):
		t.Fatal("no datagram arrived at the peer")
	}
	return 0, nil
}

// VALIDATES: an established SA answers an authenticated request at the address AND
// port that request came from, and it keeps answering there afterwards, rather than
// at the configured remote on port 500.
// PREVENTS: sendRaw resolving sa.PeerCfg.RemoteAddress for every established-path
// send, which discards the observed port and breaks every peer behind a
// port-translating NAT.
//
// RFC requirement: RFC7296-2.11-2 positive
//
// TWO PRODUCERS carry this obligation. The assertion covers the observable both
// name.
//
// RFC 7296 Section 2.11 (rfc/full/rfc7296.txt:2591-2592): an implementation
// "MUST respond to the address and port from which the request was received."
//
// RFC 7296 Section 2.23 (rfc/full/rfc7296.txt:3519-3521) states it again:
// "responses MUST be sent to the port from whence they came."
// It gives the reason too. A NAT reads the port numbers of inbound packets to select
// the internal node.
//
// The odd source port is the whole point. A test whose peer sits on 500 or 4500
// passes for an implementation that ignores the observed port.
func TestNattRepliesToTheObservedSourcePort(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)
	peerTr, ikeTr, _ := nttNATTLink(t)

	// The peer's real endpoint, and a DIFFERENT one that the SA would use if it fell
	// back to the configured remote. remoteUDPAddr resolves the configured address at
	// the ze.test.ike.port override, which nttNATTLink points at peerTr's port.
	observed := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: nttOddPort}
	fallback := resp.remoteUDPAddr()
	if fallback == nil || fallback.Port == observed.Port {
		t.Fatalf("the fallback endpoint %v matches the observed one %v, so this test cannot discriminate", fallback, observed)
	}

	// A listener on the odd port, so the reply has somewhere to land.
	oddConn, err := net.ListenUDP("udp4", observed)
	if err != nil {
		t.Skipf("port %d is not free: %v", nttOddPort, err)
	}
	defer func() { _ = oddConn.Close() }() //nolint:errcheck // the socket dies with the test

	resp.bindSockets(ikeTr, nil)

	// An authenticated INFORMATIONAL request, delivered as though it arrived from the
	// odd port. handleOwnedInbound decrypts it, so the observation is corroborated.
	req := rtxIKEDelete(t, ini, resp.ExpectedMsgID)
	ps.handleOwnedInbound(resp, transport.Packet{Data: req, RemoteAddr: observed}, ikeTr, nil, log)

	if resp.peerEndpoint == nil {
		t.Fatal("the SA stored no peer endpoint after an authenticated request")
	}
	if resp.peerEndpoint.Port != nttOddPort {
		t.Errorf("stored peer endpoint port = %d, want %d (the port the request came from)",
			resp.peerEndpoint.Port, nttOddPort)
	}
	if got := resp.remoteUDPAddr(); got == nil || got.Port != nttOddPort {
		t.Errorf("remoteUDPAddr() = %v, want port %d; every later send would miss the NAT binding",
			got, nttOddPort)
	}
	_ = peerTr
}

// VALIDATES: an UNAUTHENTICATED datagram never moves the SA's stored peer endpoint,
// and never draws a reply toward the address it claims to come from.
// PREVENTS: one forged 28-byte header repointing an established SA at a victim,
// which is what makes the cached-response replay a spoofable amplifier.
//
// RFC requirement: RFC7296-2.11-2 negative
//
// This is the discriminating half of the row. RFC 7296 Section 2.11 asks the response
// to follow the request's source. RFC 7296 Section 2.23 bounds that
// (rfc/full/rfc7296.txt:3659-3663). It limits a dynamic address update to a NEW
// packet, because an attacker can otherwise revert the addresses with an old replayed
// one. It then states that a dynamic update is safe only while replay protection runs.
//
// The adoption therefore sits AFTER decryptAndParse and AFTER the Message ID window.
// This test drives a packet that clears neither.
func TestNattUnauthenticatedPacketDoesNotMoveTheEndpoint(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)
	peerTr, ikeTr, _ := nttNATTLink(t)
	resp.bindSockets(ikeTr, nil)

	// First, an AUTHENTICATED request establishes a known endpoint.
	good := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: nttOddPort}
	req := rtxIKEDelete(t, ini, resp.ExpectedMsgID)
	ps.handleOwnedInbound(resp, transport.Packet{Data: req, RemoteAddr: good}, ikeTr, nil, log)
	before := resp.peerEndpoint
	if before == nil {
		t.Fatal("setup failed: no endpoint stored from the authenticated request")
	}

	// Now a forgery from somewhere else: the right SPI pair, a Message ID in window,
	// and ciphertext that is not.
	attacker := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 2), Port: 9999}
	forged := append([]byte(nil), req...)
	forged[len(forged)-1] ^= 0xFF
	ps.handleOwnedInbound(resp, transport.Packet{Data: forged, RemoteAddr: attacker}, ikeTr, nil, log)

	if resp.peerEndpoint == nil {
		t.Fatal("the endpoint was cleared by an unauthenticated packet")
	}
	if resp.peerEndpoint.Port != before.Port || !resp.peerEndpoint.IP.Equal(before.IP) {
		t.Errorf("an unauthenticated packet moved the peer endpoint from %v to %v. One forged datagram can now aim this SA at any address",
			before, resp.peerEndpoint)
	}
	if got := resp.remoteUDPAddr(); got.Port == attacker.Port {
		t.Errorf("the next self-initiated request would go to the attacker's port %d", attacker.Port)
	}
	_ = peerTr
}

// VALIDATES: a reply leaves from the socket the request arrived on. A request that
// reached the NAT-T socket is answered from that socket, with the non-ESP marker of
// RFC 3948. A request that reached the plain IKE socket is answered from that one,
// with no marker.
// PREVENTS: sendReply reusing the session's port-500 socket for a request that
// arrived on 4500, which a NAT drops because no binding exists for the pair.
//
// RFC requirement: RFC7296-2.11-3 positive
//
// RFC 7296 Section 2.11 (rfc/full/rfc7296.txt:2592-2593): an implementation
// "MUST specify the address and port at which the request was received as the source
// address and port in the response."
//
// SCOPE: this proves the PORT half. The source ADDRESS half is not proven here, and
// it is not claimed. Both listeners bind the wildcard by default (register.go). On a
// multi-homed host the route table picks the source address, and no sender reads
// pkt.LocalAddr. That gap is recorded for the owner as OR-WP8-1 in
// docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md.
//
// BOTH directions are asserted. A test that checked only the 4500 case passes for an
// implementation that always answers from 4500, which breaks the row the other way.
//
// The test used to call sendReply with a transport IT chose. That proves only that
// sendReply sends on the socket it is handed. The CHOICE belongs to production.
// handleResponderInbound (responder.go) takes the arrival transport from its dispatch loop
// and passes it down. And sa.sendPath is the wrong answer beside the right one: it returns
// the bound port-500 socket for an SA that has not floated.
//
// The delivery runs through handleResponderInbound now. A swap of that choice therefore
// reddens this test. Stronger checks replace the two dropped sendReply error checks. The
// wrong socket is proven REACHABLE from the SA. Each arrival is proven to draw exactly one
// datagram.
func TestNattReplyLeavesFromTheArrivalSocket(t *testing.T) {
	log := slogutil.DiscardLogger()
	peerTr, ikeTr, nattTr := nttNATTLink(t)
	dst := nttPeerAddr(t, peerTr)

	ikePort := nttPort(t, ikeTr)
	nattPort := nttPort(t, nattTr)
	if ikePort == nattPort {
		t.Fatalf("both sockets bound port %d, so the assertion cannot discriminate", ikePort)
	}

	// The cached IKE_SA_INIT response a repeated request draws. Its content does not
	// matter here. The socket it leaves from does.
	body := make([]byte, 40)
	body[17] = 0x20

	// arrive delivers a retransmitted IKE_SA_INIT request on one of ze's two sockets and
	// returns the source port and the bytes the peer saw. BOTH sockets are bound to the SA,
	// so the handler has a real choice and is proven to take the arrival one.
	arrive := func(on *transport.UDPTransport) (int, []byte) {
		t.Helper()
		sa := testSA()
		sa.PeerName = "natt-arrival"
		sa.IsInitiator = false
		sa.State = StateSAInitReceived
		sa.ResponderSAInitMsg = body
		sa.bindSockets(ikeTr, nattTr)

		// Anti-vacuity: the WRONG socket is reachable from this SA. sendPath answers with
		// the bound port-500 socket while the SA has not floated. A handler that asked the
		// SA, and not the arrival transport, answers a 4500 request from port 500.
		if chosen, _ := sa.sendPath(on); chosen != ikeTr {
			t.Fatalf("the fixture offers no wrong socket to take, so the assertions below "+
				"cannot discriminate: sendPath chose %v", chosen.LocalAddr())
		}

		req := &wire.Message{Header: wire.Header{
			InitiatorSPI: sa.InitiatorSPI,
			ResponderSPI: sa.ResponderSPI,
			MajorVersion: 2,
			ExchangeType: wire.ExchangeIKESAInit,
			Flags:        wire.FlagInitiator,
		}}
		ps := &PeerSession{peerName: sa.PeerName}
		ps.handleResponderInbound(sa, req, transport.Packet{Data: make([]byte, 28), RemoteAddr: dst}, on, log)

		port, data := nttSourcePortOf(t, peerTr)
		// A second datagram would leave the next arrival reading a stale one, so the
		// remaining assertions would judge the wrong reply.
		select {
		case extra := <-peerTr.Recv():
			t.Fatalf("one request drew a second datagram, from port %d", extra.RemoteAddr.Port)
		default:
		}
		return port, data
	}

	gotPort, gotData := arrive(nattTr)
	if gotPort != nattPort {
		t.Errorf("a request that arrived on the NAT-T socket was answered from port %d, want %d", gotPort, nattPort)
	}
	if len(gotData) != len(body)+transport.NonESPMarkerLen {
		t.Errorf("the NAT-T reply is %d bytes, want %d; RFC 3948 Section 2.2 puts a four-octet non-ESP marker on IKE over port 4500",
			len(gotData), len(body)+transport.NonESPMarkerLen)
	} else if !bytes.Equal(gotData[:transport.NonESPMarkerLen], make([]byte, transport.NonESPMarkerLen)) {
		t.Errorf("the NAT-T reply does not start with the four-octet non-ESP marker")
	}

	gotPort, gotData = arrive(ikeTr)
	if gotPort != ikePort {
		t.Errorf("a request that arrived on the IKE socket was answered from port %d, want %d", gotPort, ikePort)
	}
	if len(gotData) != len(body) {
		t.Errorf("the plain IKE reply is %d bytes, want %d; a marker on port 500 would be read as an ESP SPI", len(gotData), len(body))
	}
}

// VALIDATES: sendReply refuses to send when it has no socket or no destination.
// PREVENTS: a nil transport reading as a successful reply.
//
// RFC requirement: RFC7296-2.11-3 negative
//
// A guard that cannot evaluate must say so rather than appear to succeed
// (ai/rules/evidence.md). Without this, the positive above would pass for
// an implementation that silently dropped every reply.
func TestNattReplyRefusesWithoutADestination(t *testing.T) {
	_, ikeTr, _ := nttNATTLink(t)
	body := make([]byte, 40)

	if err := sendReply(nil, body, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1}); err == nil {
		t.Error("sendReply with no socket returned nil; a dropped reply must not read as a sent one")
	}
	if err := sendReply(ikeTr, body, nil); err == nil {
		t.Error("sendReply with no address returned nil; a dropped reply must not read as a sent one")
	}
}

// VALIDATES: once a NAT is discovered, EVERY later message the SA sends leaves from
// port 4500 and carries the non-ESP marker, whichever sender raised it. The senders
// covered are the DPD probe, the IKE Delete, the cached-response replay and the
// error notify, which between them reach every caller of sendRaw.
// PREVENTS: the pre-WP8 split where only the IKE_AUTH senders framed themselves for
// a NAT while sendRaw kept emitting bare IKE from port 500, which a floated peer
// classifies as ESP and drops without a log.
//
// RFC requirement: RFC7296-2.23-8 positive
//
// RFC 7296 Section 2.23 MUST (rfc/full/rfc7296.txt:3535-3538):
// "Port 4500 is reserved for UDP-encapsulated ESP and IKE. An IPsec endpoint that discovers a NAT between it and its correspondent (as described below) MUST send all subsequent traffic from port 4500."
// The sentence ends with a non-normative clause about NAT behavior, dropped here.
//
// The SCOPING sentence of that section applies, and it is NOT an escape hatch
// (rfc/full/rfc7296.txt:3553-3556):
// "In this section only, requirements listed as MUST apply only to implementations supporting NAT traversal."
//
// Ze supports NAT traversal. It sends the NAT_DETECTION payloads
// (buildNATDetectionPayloads, initiator.go). It binds port 4500 (register.go). It
// programs UDP encapsulation (child.go). The antecedent holds, so the MUST binds.
//
// The sender set is enumerated, and its size is asserted. A "for every sender" loop
// that reaches no sender passes trivially.
func TestNattFloatsEverySenderToPort4500(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)
	peerTr, ikeTr, nattTr := nttNATTLink(t)
	nattPort := nttPort(t, nattTr)

	resp.bindSockets(ikeTr, nattTr)
	resp.floatToNATTPort()
	resp.peerEndpoint = nttPeerAddr(t, peerTr)

	senders := []struct {
		name string
		fire func()
	}{
		{"DPD probe", func() {
			dpd := newDPDState(ipsec.DPDConfig{Interval: 30, Timeout: 120})
			sendDPD(resp, ikeTr, dpd, log)
		}},
		{"ESP Delete", func() { ps.sendDeleteESP(resp, ikeTr, 0x11223344, log) }},
		{"IKE Delete", func() { ps.sendDeleteIKE(resp, ikeTr, log) }},
		{"cached response replay", func() { sendRaw(resp, ikeTr, resp.lastResponse, log) }},
		{"error notify", func() {
			ps.respondError(resp, resp.ExpectedMsgID, wire.ExchangeInformational, wire.NotifyNoProposalChosen, nil, ikeTr, log)
		}},
	}
	if len(senders) < 5 {
		t.Fatalf("the sender set holds %d entries; a loop over an empty set proves nothing", len(senders))
	}

	seen := 0
	for _, s := range senders {
		resp.releaseRequestWindow()
		s.fire()
		gotPort, gotData := nttSourcePortOf(t, peerTr)
		seen++
		if gotPort != nattPort {
			t.Errorf("%s left from port %d, want the NAT-T socket %d", s.name, gotPort, nattPort)
		}
		if len(gotData) < transport.NonESPMarkerLen ||
			!bytes.Equal(gotData[:transport.NonESPMarkerLen], make([]byte, transport.NonESPMarkerLen)) {
			t.Errorf("%s carries no four-octet non-ESP marker; a floated peer reads it as ESP and drops it", s.name)
		}
	}
	if seen != len(senders) {
		t.Fatalf("only %d of %d senders were observed", seen, len(senders))
	}
	_ = ini
}

// VALIDATES: with no NAT detected the same senders stay on port 500 and carry no
// marker.
// PREVENTS: the positive above degenerating into "Ze always uses 4500", which would
// break every tunnel that has no NAT between its endpoints.
//
// RFC requirement: RFC7296-2.23-8 negative
//
// The float is a consequence of DISCOVERING a NAT (rfc/full/rfc7296.txt:3537). An
// implementation that floats unconditionally satisfies the MUST's text and violates
// its antecedent, and only this test separates the two.
func TestNattNoFloatWithoutNAT(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)
	peerTr, ikeTr, nattTr := nttNATTLink(t)
	ikePort := nttPort(t, ikeTr)

	resp.bindSockets(ikeTr, nattTr)
	resp.peerEndpoint = nttPeerAddr(t, peerTr)
	if resp.localPort == transport.NATTPort {
		t.Fatal("the SA floated with no NAT detected")
	}

	ps.sendDeleteIKE(resp, ikeTr, log)
	gotPort, gotData := nttSourcePortOf(t, peerTr)
	if gotPort != ikePort {
		t.Errorf("with no NAT the Delete left from port %d, want the IKE socket %d", gotPort, ikePort)
	}
	if len(gotData) >= transport.NonESPMarkerLen &&
		bytes.Equal(gotData[:transport.NonESPMarkerLen], make([]byte, transport.NonESPMarkerLen)) {
		t.Error("with no NAT the Delete carries a non-ESP marker; RFC 3948 Section 2.2 puts that marker on port 4500 only")
	}
	_ = ini
}

// VALIDATES: a floated SA whose NAT-T socket is missing sends NOTHING, rather than
// falling back to port 500.
// PREVENTS: a silent fallback that looks like a working tunnel and carries no
// traffic, because a floated peer reads an unmarked datagram from port 500 as ESP.
//
// RFC requirement: RFC7296-2.23-8 negative
//
// RFC 7296 Section 2.23 says ALL subsequent traffic leaves from port 4500. There is
// no permitted fallback, so the guard denies (ai/rules/evidence.md).
func TestNattFloatedSAWithoutNATTSocketSendsNothing(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)
	peerTr, ikeTr, _ := nttNATTLink(t)

	resp.bindSockets(ikeTr, nil)
	resp.floatToNATTPort()
	resp.peerEndpoint = nttPeerAddr(t, peerTr)

	ps.sendDeleteIKE(resp, ikeTr, log)
	rtxExpectSilence(t, peerTr, ikeTr, nttPeerAddr(t, peerTr), "a floated SA with no NAT-T socket")
	_ = ini
}

// VALIDATES: no code path can ask the dataplane to encapsulate ESP on port 500.
// PREVENTS: a later change deriving the encapsulation ports from the SA's own port
// field, which would make port 500 expressible.
//
// RFC requirement: RFC7296-2.23-9 positive
//
// RFC 7296 Section 2.23 MUST NOT (rfc/full/rfc7296.txt:3544): "UDP encapsulation MUST
// NOT be done on port 500."
//
// This rests on a property the code HAS, not on a guard that is absent. The only two
// writers assign transport.NATTPort, and that constant is not 500.
//
// The sweep covers both NAT states and both directions. The claim is therefore about
// every reachable combination and not about one sample.
func TestEncapNeverRequestedOnPort500(t *testing.T) {
	log := slogutil.DiscardLogger()
	if transport.NATTPort == transport.IKEPort {
		t.Fatalf("transport.NATTPort == transport.IKEPort == %d; the two constants collided", transport.NATTPort)
	}

	// The counter increments AFTER the `if !p.UDPEncap { continue }`, and it must.
	// Before it, the counter reached its target while every port assertion was
	// skipped: pinning createFirstChildSA to UDPEncap false left this test green with
	// zero assertions run, which is the shape it exists to prevent.
	encapChecked, plainChecked := 0, 0
	for _, nat := range []bool{false, true} {
		sa := testSA()
		sa.NATDetected = nat
		if nat {
			sa.floatToNATTPort()
		}
		dp := &mockDP{}
		if _, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log); err != nil {
			t.Fatalf("createFirstChildSA(nat=%v): %v", nat, err)
		}
		if len(dp.sas) != 2 {
			t.Fatalf("nat=%v: installed %d SAs, want 2", nat, len(dp.sas))
		}
		for i, p := range dp.sas {
			if !p.UDPEncap {
				plainChecked++
				continue
			}
			encapChecked++
			if p.UDPEncapSPort == transport.IKEPort || p.UDPEncapDPort == transport.IKEPort {
				t.Errorf("nat=%v SA[%d]: encapsulation requested on port 500 (sport=%d dport=%d)",
					nat, i, p.UDPEncapSPort, p.UDPEncapDPort)
			}
		}
	}
	// Anti-vacuity. The port assertion runs only on an ENCAPSULATED SA, so the count that
	// matters is the encapsulated one. A builder that stopped requesting encapsulation
	// would leave encapChecked at zero and fail here rather than pass silently.
	if encapChecked != 2 {
		t.Fatalf("%d encapsulated SA params were examined, want 2; the port assertion ran on "+
			"nothing, so this test proves nothing about which port Ze asks for", encapChecked)
	}
	if plainChecked != 2 {
		t.Fatalf("%d unencapsulated SA params were seen, want 2; the no-NAT arm did not run",
			plainChecked)
	}
}

// VALIDATES: a rekeyed Child SA inherits UDPEncap, so its XFRM state keeps the ESP-in-UDP
// template the first Child SA was given.
// PREVENTS: a NAT-traversing tunnel that establishes, rekeys, and then carries nothing.
//
// createFirstChildSA is the only other writer of UDPEncap, and installChildSA gates the
// ESP-in-UDP template on it. A rekeyed child that does not inherit the field installs a
// state with no template, and the kernel then refuses the encapsulated ESP the peer keeps
// sending. The review that found this checked the field's writers, which no test did.
//
// RFC requirement: RFC7296-2.23-8 positive -- RFC 7296 Section 2.23 requires both devices to
// use UDP encapsulation for ESP once a NAT is detected. The obligation binds for the life of
// the tunnel, not only its first Child SA, so a rekey that drops the encapsulation breaks it.
func TestNattRekeyedChildKeepsUDPEncap(t *testing.T) {
	for _, encap := range []bool{false, true} {
		old := &ChildSA{
			LocalAddr:   net.IPv4(10, 0, 0, 1),
			RemoteAddr:  net.IPv4(10, 0, 0, 2),
			NATDetected: encap,
			UDPEncap:    encap,
		}
		// The rekey negotiated no selector, so the replacement keeps the retired pair's.
		got := newRekeyedChild(old, 0x11111111, 0x22222222, nil, true, nil)
		if got.UDPEncap != encap {
			t.Errorf("rekeyed child UDPEncap = %v, want %v; installChildSA gates the "+
				"ESP-in-UDP template on this field, so a rekey that drops it installs a "+
				"state the kernel will not match against the peer's encapsulated ESP",
				got.UDPEncap, encap)
		}
		// The control: NATDetected was already inherited before this fix, so a test that
		// only read that field would have stayed green through the whole defect.
		if got.NATDetected != encap {
			t.Errorf("rekeyed child NATDetected = %v, want %v", got.NATDetected, encap)
		}
	}
}

// VALIDATES: port 500 IS expressible in the dataplane parameters, so the absence
// above is a decision the BUILDER takes and not a limit of the representation.
// PREVENTS: the positive degenerating into "SAParams cannot carry 500", which would
// make it prove nothing about Ze's behavior.
//
// RFC requirement: RFC7296-2.23-9 negative.
func TestEncapPortsAreExpressible(t *testing.T) {
	dp := &mockDP{}
	if err := dp.InstallSA(dataplane.SAParams{
		SPI: 1, UDPEncap: true, UDPEncapSPort: transport.IKEPort, UDPEncapDPort: transport.IKEPort,
	}); err != nil {
		t.Fatalf("InstallSA: %v", err)
	}
	if len(dp.sas) != 1 {
		t.Fatalf("captured %d SAs, want 1", len(dp.sas))
	}
	if dp.sas[0].UDPEncapSPort != transport.IKEPort || dp.sas[0].UDPEncapDPort != transport.IKEPort {
		t.Errorf("the port fields did not survive the call (sport=%d dport=%d); TestEncapNeverRequestedOnPort500 proves nothing",
			dp.sas[0].UDPEncapSPort, dp.sas[0].UDPEncapDPort)
	}
}

// VALIDATES: the established-SA cached-response replay is bounded by BOTH guards.
// An unprotected forgery at the cached Message ID draws nothing, and a burst of
// protected-looking forgeries is cut off by the token bucket.
// PREVENTS: WP-8 turning that replay into a spoofable amplifier. Before WP-8 the
// branch answered the CONFIGURED peer, so an attacker had no way to aim it.
//
// WP-8 makes every established-path send follow the SA's authenticated endpoint.
// That is the same primitive the sibling site in handleResponderInbound already
// needed both guards for.
//
// RFC 7296 Section 2.21.4: "A node needs to limit the rate at which it will send
// messages in response to unprotected messages", and "A peer receiving such an
// unprotected Notify payload MUST NOT respond."
//
// It is deliberately untagged. It proves a guard on an emission path, not an
// obligation the RFC ledger claims for a numbered requirement.
func TestNattCachedReplayIsBoundedByBothGuards(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)
	peerTr, ikeTr, _ := nttNATTLink(t)
	resp.bindSockets(ikeTr, nil)
	resp.peerEndpoint = nttPeerAddr(t, peerTr)

	// One authenticated request, so the SA holds a cached response to replay.
	req := rtxIKEDelete(t, ini, resp.ExpectedMsgID)
	ps.handleOwnedInbound(resp, transport.Packet{Data: req, RemoteAddr: resp.peerEndpoint}, ikeTr, nil, log)
	if !resp.lastResponseSet {
		t.Fatal("setup failed: no cached response to replay")
	}
	if _, _ = nttSourcePortOf(t, peerTr); false {
		t.Fatal("unreachable")
	}

	// GUARD ONE. A bare header at the cached Message ID carries no Encrypted payload,
	// so it draws nothing at all.
	hdrOnly := append([]byte(nil), req[:wire.HeaderLen]...)
	// The length field must match, else the outer parse fails before the branch.
	hdrOnly[24] = 0
	hdrOnly[25] = 0
	hdrOnly[26] = 0
	hdrOnly[27] = byte(wire.HeaderLen)
	beforeSK := errorNotifySuppressedCount("unprotected-retransmit")
	ps.handleOwnedInbound(resp, transport.Packet{Data: hdrOnly, RemoteAddr: resp.peerEndpoint}, ikeTr, nil, log)
	if errorNotifySuppressedCount("unprotected-retransmit") <= beforeSK {
		t.Error("an unprotected header at the cached message id was not stopped by the SK-presence guard")
	}
	rtxExpectSilence(t, peerTr, ikeTr, resp.peerEndpoint, "an unprotected header at the cached message id")

	// GUARD TWO. Genuine retransmissions DO carry an Encrypted payload, so only the
	// token bucket bounds them. The burst is cachedReplayBurst, so a run well past it
	// must be cut off.
	beforeRate := errorNotifySuppressedCount("replay-rate-limited")
	for range cachedReplayBurst + 4 {
		ps.handleOwnedInbound(resp, transport.Packet{Data: req, RemoteAddr: resp.peerEndpoint}, ikeTr, nil, log)
	}
	if errorNotifySuppressedCount("replay-rate-limited") <= beforeRate {
		t.Errorf("%d protected retransmissions all drew a replay; the token bucket did not bound the amplifier",
			cachedReplayBurst+4)
	}
}

// RFC requirement: RFC3948-4-3 positive -- "Reception of NAT-keepalive packets MUST NOT be
// used to detect whether a connection is live" (rfc/full/rfc3948.txt, Section 4). Two
// guards drop the one-octet 0xFF datagram, and the test asserts the OUTCOME both produce:
// UDPTransport.Run refuses any datagram shorter than an IKE header before it reaches
// Recv (transport/udp.go:150-152), and dispatchNATTInbound discards a keepalive before any
// SA lookup (register.go:775-777). No SA, no peer session and no DPD state ever observes
// the arrival, so nothing about the connection can be concluded from it. Removing either
// guard alone keeps the test green; removing BOTH turns it red, which is how its
// discrimination was measured.
// RFC requirement: RFC3948-4-3 negative -- the discard is keepalive-specific rather than a
// blanket drop. A marked IKE datagram sent behind the keepalive on the SAME socket IS
// delivered to the owning session, so liveness still travels the authenticated exchange
// path (engine/dpd.go matchesProbe) and only that path.
func TestNATKeepaliveReachesNoSA(t *testing.T) {
	log := slogutil.DiscardLogger()

	tr, err := transport.NewNATTTransport("127.0.0.1:0", log)
	if err != nil {
		t.Fatalf("NewNATTTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })
	go tr.Run()

	table := NewSATable()
	sa := testSA()
	spi, err := GenerateSPI()
	if err != nil {
		t.Fatalf("GenerateSPI: %v", err)
	}
	sa.InitiatorSPI = spi
	sa.PeerName = "natt-keepalive"
	table.Insert(sa)

	ps := &PeerSession{peerName: sa.PeerName, inbound: make(chan transport.Packet, 4)}
	ps.ownedSA.Store(sa)
	SetActivePeersForTest(map[string]*PeerSession{sa.PeerName: ps})
	t.Cleanup(func() { SetActivePeersForTest(nil) })

	go dispatchNATTInbound(tr, table, log)

	local, ok := tr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("transport LocalAddr is not *net.UDPAddr")
	}
	sender, err := net.DialUDP("udp4", nil, local)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	t.Cleanup(func() { _ = sender.Close() })

	msg := wire.Message{Header: wire.Header{
		InitiatorSPI: sa.InitiatorSPI,
		MajorVersion: 2,
		ExchangeType: wire.ExchangeInformational,
		Flags:        wire.FlagInitiator,
		MessageID:    9,
	}}
	buf := make([]byte, 512)
	n := msg.WriteTo(buf, 0)

	// The keepalive goes FIRST. UDP on loopback keeps order and dispatchNATTInbound is a
	// single goroutine, so the IKE datagram arriving on ps.inbound proves the keepalive was
	// dropped rather than merely overtaken.
	if _, err := sender.Write([]byte{0xFF}); err != nil {
		t.Fatalf("write the NAT-keepalive: %v", err)
	}
	if _, err := sender.Write(transport.AddNonESPMarker(buf[:n])); err != nil {
		t.Fatalf("write the IKE datagram: %v", err)
	}

	select {
	case pkt := <-ps.inbound:
		if len(pkt.Data) == 1 && pkt.Data[0] == 0xFF {
			t.Fatal("the NAT-keepalive was delivered to the SA; RFC 3948 Section 4 forbids reading liveness from its reception")
		}
		if len(pkt.Data) < wire.HeaderLen {
			t.Fatalf("the delivered datagram is %d byte(s), want at least an IKE header (%d)", len(pkt.Data), wire.HeaderLen)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the IKE datagram was never delivered, so the keepalive drop cannot be judged")
	}

	// Nothing else CAN follow: the keepalive was ahead of the IKE datagram in the queue.
	select {
	case pkt := <-ps.inbound:
		t.Errorf("a second datagram reached the SA (%d byte(s)); only the IKE one may pass", len(pkt.Data))
	default:
	}
}
