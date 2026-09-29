// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- inbound dispatch
// Related: rfc7296_header_test.go -- TestHigherMajorVersionDropped, the same UDP fixture
// VALIDATES: dispatchInbound drops a datagram whose Initiator's SPI is zero, before any SA
// lookup (RFC 7296 Section 3.1).
// PREVENTS: a zero Initiator's SPI that reaches the SA table and is matched there.

package engine

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestRFC7296ZeroInitiatorSPIIsDroppedOnReceipt drives the real inbound dispatcher over a
// UDP socket with a datagram whose Initiator's SPI is zero.
//
// Goal: the zero check in dispatchInbound is the only thing between a zero Initiator's SPI
// and the SA table, and TestSPIZeroRules never runs the dispatcher. Method: the table
// holds an SA whose Initiator's SPI is zero, so a dispatcher without the check WOULD find
// an owner for the zero-SPI datagram and deliver it. The zero-SPI datagram is sent first,
// then an otherwise identical datagram for an SA with a non-zero Initiator's SPI.
// Loopback UDP keeps the order and dispatchInbound is one goroutine, so the second
// datagram arriving while the first never does proves the first was dropped rather than
// delayed.
//
// RFC 7296 Section 3.1: "Initiator's SPI (8 octets) - A value chosen by the initiator to
// identify a unique IKE Security Association. This value MUST NOT be zero."
//
// RFC requirement: RFC7296-3.1-3 negative -- a datagram whose Initiator's SPI is zero is dropped by dispatchInbound and reaches no session, although the SA table holds an SA it would match; the same datagram with a non-zero Initiator's SPI is delivered.
func TestRFC7296ZeroInitiatorSPIIsDroppedOnReceipt(t *testing.T) {
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
	zeroSA.PeerName = "zero-ispi"
	table.Insert(zeroSA)
	liveSA := testSA()
	liveSA.InitiatorSPI = [8]byte{9, 9, 9, 9, 9, 9, 9, 9}
	liveSA.ResponderSPI = responderSPI
	liveSA.PeerName = "live-ispi"
	table.Insert(liveSA)

	zeroPS := &PeerSession{peerName: zeroSA.PeerName, inbound: make(chan transport.Packet, 4)}
	zeroPS.ownedSA.Store(zeroSA)
	livePS := &PeerSession{peerName: liveSA.PeerName, inbound: make(chan transport.Packet, 4)}
	livePS.ownedSA.Store(liveSA)
	SetActivePeersForTest(map[string]*PeerSession{zeroSA.PeerName: zeroPS, liveSA.PeerName: livePS})
	t.Cleanup(func() { SetActivePeersForTest(nil) })

	go dispatchInbound(tr, table, log)

	local, ok := tr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("transport LocalAddr is not *net.UDPAddr")
	}
	sender, err := net.DialUDP("udp4", nil, local)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	t.Cleanup(func() { _ = sender.Close() })

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
		n := msg.WriteTo(buf, 0)
		return buf[:n]
	}
	if _, err := sender.Write(datagram(zeroSA.InitiatorSPI)); err != nil {
		t.Fatalf("Write zero SPI: %v", err)
	}
	if _, err := sender.Write(datagram(liveSA.InitiatorSPI)); err != nil {
		t.Fatalf("Write non-zero SPI: %v", err)
	}

	select {
	case <-livePS.inbound:
	case <-time.After(5 * time.Second):
		t.Fatal("the non-zero SPI datagram was never delivered, so the drop cannot be judged")
	}
	select {
	case <-zeroPS.inbound:
		t.Fatal("the datagram with a zero Initiator's SPI was delivered to the SA it matched")
	default:
	}
}
