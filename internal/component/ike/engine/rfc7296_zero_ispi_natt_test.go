// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- inbound dispatch
// Related: rfc7296_zero_ispi_test.go -- the same drop on the port-500 dispatcher
// VALIDATES: dispatchNATTInbound drops a datagram whose Initiator's SPI is zero, before any
// SA lookup (RFC 7296 Section 3.1).
// PREVENTS: a zero Initiator's SPI arriving on the NAT-T socket that reaches the SA table.

package engine

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestRFC7296ZeroInitiatorSPIIsDroppedOnTheNATTSocket drives the real NAT-T dispatcher over
// a UDP socket with a non-ESP-marked IKE datagram whose Initiator's SPI is zero.
//
// Goal: dispatchNATTInbound holds its own zero check, separate from the port-500
// dispatcher's. Method: as TestRFC7296ZeroInitiatorSPIIsDroppedOnReceipt. The table holds
// an SA whose Initiator's SPI is zero, so a dispatcher without the check WOULD deliver the
// zero-SPI datagram to that SA's owner. The zero-SPI datagram goes first, then an otherwise
// identical one for an SA with a non-zero Initiator's SPI. The dispatcher is one goroutine
// and loopback UDP keeps the order, so the second arriving while the first never does
// proves the first was dropped, not delayed.
//
// RFC 7296 Section 3.1: "Initiator's SPI (8 octets) - A value chosen by the initiator to
// identify a unique IKE Security Association. This value MUST NOT be zero."
//
// RFC requirement: RFC7296-3.1-3 negative -- a non-ESP-marked datagram on the NAT-T socket whose Initiator's SPI is zero is dropped by dispatchNATTInbound and reaches no session, although the SA table holds an SA it would match; the same datagram with a non-zero Initiator's SPI is delivered.
func TestRFC7296ZeroInitiatorSPIIsDroppedOnTheNATTSocket(t *testing.T) {
	log := slogutil.DiscardLogger()
	tr, err := transport.NewUDPTransport("127.0.0.1:0", log)
	if err != nil {
		t.Fatalf("NewUDPTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })
	go tr.Run()

	responderSPI := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	table := NewSATable()
	zeroSA := testSA()
	zeroSA.InitiatorSPI = [8]byte{}
	zeroSA.ResponderSPI = responderSPI
	zeroSA.PeerName = "natt-zero-ispi"
	table.Insert(zeroSA)
	liveSA := testSA()
	liveSA.InitiatorSPI = [8]byte{9, 9, 9, 9, 9, 9, 9, 9}
	liveSA.ResponderSPI = responderSPI
	liveSA.PeerName = "natt-live-ispi"
	table.Insert(liveSA)

	zeroPS := &PeerSession{peerName: zeroSA.PeerName, inbound: make(chan transport.Packet, 4)}
	zeroPS.ownedSA.Store(zeroSA)
	livePS := &PeerSession{peerName: liveSA.PeerName, inbound: make(chan transport.Packet, 4)}
	livePS.ownedSA.Store(liveSA)
	SetActivePeersForTest(map[string]*PeerSession{zeroSA.PeerName: zeroPS, liveSA.PeerName: livePS})
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

	// RFC 3948 Section 2.2: IKE on port 4500 follows a four-octet zero non-ESP marker.
	datagram := func(initiatorSPI [8]byte) []byte {
		msg := wire.Message{Header: wire.Header{
			InitiatorSPI: initiatorSPI,
			ResponderSPI: responderSPI,
			MajorVersion: 2,
			ExchangeType: wire.ExchangeInformational,
			Flags:        wire.FlagInitiator,
			MessageID:    7,
		}}
		buf := make([]byte, 512)
		n := msg.WriteTo(buf, 4)
		return buf[:4+n]
	}
	if _, err := sender.Write(datagram(zeroSA.InitiatorSPI)); err != nil {
		t.Fatalf("Write zero SPI: %v", err)
	}
	if _, err := sender.Write(datagram(liveSA.InitiatorSPI)); err != nil {
		t.Fatalf("Write non-zero SPI: %v", err)
	}

	select {
	case pkt := <-livePS.inbound:
		if len(pkt.Data) < 8 || [8]byte(pkt.Data[0:8]) != liveSA.InitiatorSPI {
			t.Fatalf("the delivered datagram does not start with the IKE header (marker not stripped): % x", pkt.Data)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the non-zero SPI datagram was never delivered, so the drop cannot be judged")
	}
	select {
	case <-zeroPS.inbound:
		t.Fatal("the NAT-T datagram with a zero Initiator's SPI was delivered to the SA it matched")
	default:
	}
}
