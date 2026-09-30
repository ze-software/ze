// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- NAT traversal, reply to the observed endpoint
// Related: rfc7296_natt_test.go -- the NAT link fixture and the stored-endpoint assertions

package engine

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// rpoReply drives one authenticated INFORMATIONAL request into an established responder as
// though it arrived from 127.0.0.1:nttOddPort, a port that is neither 500, 4500 nor the
// configured remote's port. It returns the initiator SA (to decrypt the reply), the request's
// Message ID, the socket bound on the odd port, and the transport that stands for the
// configured remote.
func rpoReply(t *testing.T) (*SA, uint32, *net.UDPConn, *transport.UDPTransport) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)
	peerTr, ikeTr, _ := nttNATTLink(t, resp)

	observed := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: nttOddPort}
	if configured := resp.remoteUDPAddr(); configured == nil || configured.Port == observed.Port {
		t.Fatalf("the configured endpoint %v matches the observed one %v, so nothing discriminates",
			configured, observed)
	}
	oddConn, err := net.ListenUDP("udp4", observed)
	if err != nil {
		t.Skipf("port %d is not free: %v", nttOddPort, err)
	}
	t.Cleanup(func() { _ = oddConn.Close() }) //nolint:errcheck // the socket dies with the test

	resp.bindSockets(ikeTr, nil)
	msgID := resp.ExpectedMsgID
	req := rtxIKEDelete(t, ini, msgID)
	ps.handleOwnedInbound(resp, transport.Packet{Data: req, RemoteAddr: observed}, ikeTr, nil, log)
	return ini, msgID, oddConn, peerTr
}

// VALIDATES: the response to a request that arrived from an unusual source port is a real
// datagram delivered to that address and port, it answers the request's Message ID, and it
// authenticates under the IKE SA.
// PREVENTS: a responder that records the observed endpoint but still sends from a path that
// reads the configured remote, which strands a peer behind a NAT.
//
// RFC requirement: RFC7296-2.11-2 positive -- RFC 7296 Section 2.11: an implementation "MUST
// respond to the address and port from which the request was received." The datagram read on
// the socket bound to 127.0.0.1:34567, the request's source, is a response carrying the
// request's Message ID that decrypts under the initiator's IKE SA.
func TestRFC7296ReplyLandsOnTheRequestSourcePort(t *testing.T) {
	ini, msgID, oddConn, _ := rpoReply(t)

	if err := oddConn.SetReadDeadline(time.Now().Add(rtxArrive)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	buf := make([]byte, 65535)
	n, _, err := oddConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("no reply reached the address and port the request came from: %v", err)
	}
	raw := buf[:n]
	msg := parseMsg(t, raw)
	if msg.Header.Flags&wire.FlagResponse == 0 {
		t.Errorf("the datagram on the request's source port is not a response (flags %#x)", msg.Header.Flags)
	}
	if msg.Header.MessageID != msgID {
		t.Errorf("the reply carries Message ID %d, want the request's %d", msg.Header.MessageID, msgID)
	}
	lcyDecrypt(t, ini, raw)
}

// RFC requirement: RFC7296-2.11-2 negative -- the non-compliant destination, the configured
// remote endpoint the SA would fall back to, receives no datagram when the request came from a
// different port: the response goes to the request's source and not to the configured peer.
func TestRFC7296ReplyDoesNotGoToTheConfiguredEndpoint(t *testing.T) {
	_, _, _, configured := rpoReply(t)

	if raw := rtxRecv(t, configured); raw != nil {
		t.Errorf("a %d-octet datagram went to the configured remote instead of the request's source",
			len(raw))
	}
}
