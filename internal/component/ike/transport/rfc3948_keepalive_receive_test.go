// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- NAT keepalive receive tests

package transport

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// natTReceiver starts a NAT-T transport on loopback and a sender socket aimed at it.
func natTReceiver(t *testing.T) (*UDPTransport, *net.UDPConn) {
	t.Helper()
	tr, err := NewNATTTransport("127.0.0.1:0", slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("NewNATTTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })
	go tr.Run()

	local, ok := tr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("LocalAddr is not *net.UDPAddr")
	}
	sender, err := net.DialUDP("udp4", nil, local)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	t.Cleanup(func() { _ = sender.Close() })
	return tr, sender
}

// natTIKEDatagram is an IKE message as it arrives on port 4500: the four-octet
// non-ESP marker, then a 28-octet IKE header whose first octet is a marker the
// test looks for.
func natTIKEDatagram(first byte) []byte {
	datagram := make([]byte, NonESPMarkerLen+28)
	datagram[NonESPMarkerLen] = first
	return datagram
}

// RFC requirement: RFC3948-2.3-1 positive -- "The receiver SHOULD ignore a received
// NAT-keepalive packet." (rfc/full/rfc3948.txt, Section 2.3). A 0xFF keepalive sent
// ahead of an IKE message is ignored and costs the receive path nothing: the first packet
// Recv delivers is the IKE message, byte for byte, and no keepalive octet is delivered.
//
// Method: one keepalive, then one IKE datagram, on the same NAT-T socket.
func TestRFC3948KeepaliveAheadOfIKEIsIgnored(t *testing.T) {
	tr, sender := natTReceiver(t)

	if _, err := sender.Write([]byte{keepaliveByte}); err != nil {
		t.Fatalf("write keepalive: %v", err)
	}
	ike := natTIKEDatagram(0xA5)
	if _, err := sender.Write(ike); err != nil {
		t.Fatalf("write IKE datagram: %v", err)
	}

	select {
	case pkt := <-tr.Recv():
		if !bytes.Equal(pkt.Data, ike) {
			t.Fatalf("first delivered packet = %x; want the IKE datagram %x, the keepalive ignored", pkt.Data, ike)
		}
	case <-time.After(time.Second):
		t.Fatal("the IKE datagram after a keepalive was never delivered")
	}
}

// RFC requirement: RFC3948-2.3-1 negative -- "The receiver SHOULD ignore a received
// NAT-keepalive packet." (rfc/full/rfc3948.txt, Section 2.3). The non-compliant receiver
// hands the keepalive on as a packet; Ze's transport refuses to: a lone 0xFF datagram
// never reaches Recv (udp.go Run, the 28-octet IKE header floor).
//
// Method: a keepalive alone, then a bounded wait on Recv.
func TestRFC3948KeepaliveIsNotHandedOn(t *testing.T) {
	tr, sender := natTReceiver(t)

	if _, err := sender.Write([]byte{keepaliveByte}); err != nil {
		t.Fatalf("write keepalive: %v", err)
	}

	select {
	case pkt := <-tr.Recv():
		t.Fatalf("a NAT-keepalive was handed on as a %d-octet packet %x", len(pkt.Data), pkt.Data)
	case <-time.After(200 * time.Millisecond):
	}
}
