// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- NAT traversal, dynamic address update
// Related: rfc7296_natt_test.go -- the NAT link fixture and the unauthenticated-packet negative

package engine

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// dynDeliver drives one authenticated, empty INFORMATIONAL request (a liveness check)
// into the established responder as though it arrived from source. The request sits at
// the responder's expected Message ID, so it clears both the integrity check and the
// window, which are what make it a "validated packet" in RFC 7296 Section 2.23.
func dynDeliver(t *testing.T, ini, resp *SA, ps *PeerSession, ikeTr *transport.UDPTransport, source *net.UDPAddr) {
	t.Helper()
	raw, err := buildEncryptedMessageEx(ini, nil, resp.ExpectedMsgID, wire.ExchangeInformational, initiatorFlag(ini))
	if err != nil {
		t.Fatalf("build INFORMATIONAL request: %v", err)
	}
	pkt := transport.Packet{Data: raw, RemoteAddr: source}
	ps.handleOwnedInbound(resp, pkt, ikeTr, nil, slogutil.DiscardLogger())
}

// dynEstablish returns an established initiator/responder pair whose responder owns a
// bound port-500 socket, ready for dynDeliver.
func dynEstablish(t *testing.T) (ini, resp *SA, ps *PeerSession, ikeTr *transport.UDPTransport) {
	t.Helper()
	ini, resp, ps = establishPSK(t)
	_, ikeTr, _ = nttNATTLink(t, resp)
	resp.bindSockets(ikeTr, nil)
	return ini, resp, ps, ikeTr
}

// dynListen binds a loopback socket on an ephemeral port, standing for the peer's new
// endpoint, and returns it with its address.
func dynListen(t *testing.T) (*net.UDPConn, *net.UDPAddr) {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() }) //nolint:errcheck // the socket dies with the test
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("local address %v is not UDP", conn.LocalAddr())
	}
	return conn, addr
}

// VALIDATES: once the SA holds an endpoint, a validated packet from a DIFFERENT address and
// port moves it. The new pair is stored on the SA, every later send resolves to it, and the
// datagram answering that packet is read on the new endpoint's socket.
// PREVENTS: an SA that keeps sending to the endpoint a NAT mapping dropped, so the tunnel
// dies although the peer is alive and authenticated.
//
// RFC requirement: RFC7296-2.23-13 positive -- RFC 7296 Section 2.23: a host "that is not
// behind a NAT, SHOULD send all packets (including retransmission packets) to the IP address
// and port in the validated packet, and SHOULD store this as the new address and port
// combination for the SA". The responder is not behind a NAT and runs no MOBIKE; after a
// validated request from 127.0.0.1:34567 and a second from a new loopback port, the stored
// endpoint and remoteUDPAddr (the address every send and retransmission reads) are the new
// pair, and the response is received on it.
func TestRFC7296ValidatedPacketFromANewEndpointMovesTheSA(t *testing.T) {
	ini, resp, ps, ikeTr := dynEstablish(t)
	if resp.BehindNAT {
		t.Fatal("setup: the responder believes it is behind a NAT, so the row does not apply")
	}
	if resp.mobike.enabled {
		t.Fatal("setup: MOBIKE is enabled, so the row does not apply")
	}

	first := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: nttOddPort}
	dynDeliver(t, ini, resp, ps, ikeTr, first)
	if resp.peerEndpoint == nil || !sameUDPEndpoint(resp.peerEndpoint, first) {
		t.Fatalf("setup: stored endpoint %v after the first validated packet, want %v", resp.peerEndpoint, first)
	}

	conn, moved := dynListen(t)
	dynDeliver(t, ini, resp, ps, ikeTr, moved)

	if resp.peerEndpoint == nil || !sameUDPEndpoint(resp.peerEndpoint, moved) {
		t.Errorf("stored endpoint %v after a validated packet from %v, want the new pair", resp.peerEndpoint, moved)
	}
	if got := resp.remoteUDPAddr(); got == nil || !sameUDPEndpoint(got, moved) {
		t.Errorf("remoteUDPAddr() = %v, want %v: later sends and retransmissions would miss the peer", got, moved)
	}
	if err := conn.SetReadDeadline(time.Now().Add(rtxArrive)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	buf := make([]byte, 65535)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("no datagram reached the new endpoint: %v", err)
	}
	if msg := parseMsg(t, buf[:n]); msg.Header.Flags&wire.FlagResponse == 0 {
		t.Errorf("the datagram on the new endpoint is not a response (flags %#x)", msg.Header.Flags)
	}
}

// RFC requirement: RFC7296-2.23-13 negative -- the row binds a host "that does not support
// other methods of recovery such as IKEv2 Mobility and Multihoming (MOBIKE) [MOBIKE], and that
// is not behind a NAT". A responder behind a NAT, and one running MOBIKE, each receive the
// same validated packet from a new endpoint and keep the stored one: the dynamic update is
// not applied outside the hosts the sentence names.
func TestRFC7296ValidatedPacketDoesNotMoveTheSAOutsideTheRow(t *testing.T) {
	for _, tc := range []struct {
		name  string
		apply func(*SA)
	}{
		{"behind a NAT", func(sa *SA) { sa.BehindNAT = true }},
		{"MOBIKE", func(sa *SA) { sa.mobike.enabled = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ini, resp, ps, ikeTr := dynEstablish(t)
			first := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: nttOddPort}
			dynDeliver(t, ini, resp, ps, ikeTr, first)
			if resp.peerEndpoint == nil || !sameUDPEndpoint(resp.peerEndpoint, first) {
				t.Fatalf("setup: stored endpoint %v, want %v", resp.peerEndpoint, first)
			}

			tc.apply(resp)
			_, moved := dynListen(t)
			dynDeliver(t, ini, resp, ps, ikeTr, moved)

			if resp.peerEndpoint == nil || !sameUDPEndpoint(resp.peerEndpoint, first) {
				t.Errorf("stored endpoint %v after a validated packet from %v, want it kept at %v",
					resp.peerEndpoint, moved, first)
			}
		})
	}
}
