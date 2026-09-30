// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- inbound dispatch
// Related: rfc7296_spi_pair_identifier_test.go -- the same property over SATable.lookupInbound
// Related: rfc7296_zero_ispi_test.go -- the UDP fixture this file follows
// VALIDATES: RFC 7296 Section 2.6 through both receive loops: a datagram whose SPI pair
// names no IKE SA is delivered to no session, although its initiator SPI is an
// established SA's, on port 500 (dispatchInbound) and on the NAT-T socket
// (dispatchNATTInbound).
// PREVENTS: a fallback in a receive loop, such as a lookup by initiator SPI alone, that
// routes a packet to an IKE SA its pair does not name.

package engine

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// spiPairDatagram answers one INFORMATIONAL request header for the SPI pair, prefixed with
// the four-octet non-ESP marker when natt is set (RFC 3948 Section 2.2).
func spiPairDatagram(initiatorSPI, responderSPI [8]byte, natt bool) []byte {
	msg := wire.Message{Header: wire.Header{
		InitiatorSPI: initiatorSPI,
		ResponderSPI: responderSPI,
		MajorVersion: 2,
		ExchangeType: wire.ExchangeInformational,
		Flags:        wire.FlagInitiator,
		MessageID:    7,
	}}
	buf := make([]byte, 512)
	off := 0
	if natt {
		off = 4
	}
	n := msg.WriteTo(buf, off)
	return buf[:off+n]
}

// RFC requirement: RFC7296-2.6-1 negative -- through the real receive loops, a datagram
// whose initiator SPI is an established IKE SA's but whose non-zero responder SPI is not
// is delivered to no session, on port 500 (dispatchInbound) and on the NAT-T socket
// (dispatchNATTInbound). The datagram with the SA's own pair, sent after it on the same
// socket, is delivered to that SA's session.
// RFC requirement: RFC7296-2.6-1 positive -- the datagram carrying the established SA's
// full SPI pair reaches that SA's session through each receive loop.
//
// Goal: TestRFC7296UnknownIKESPIPairMapsToNoSA calls SATable.lookupInbound directly, so a
// fallback added to either loop would leave it green. Method: loopback UDP keeps the order
// and each loop is one goroutine, so the matching datagram arriving while the mismatched
// one never does proves the mismatched one was not routed rather than delayed.
func TestRFC7296MismatchedResponderSPIReachesNoSAThroughTheReceiveLoops(t *testing.T) {
	for _, natt := range []bool{false, true} {
		log := slogutil.DiscardLogger()
		tr, err := transport.NewUDPTransport("127.0.0.1:0", log)
		if err != nil {
			t.Fatalf("NewUDPTransport: %v", err)
		}
		go tr.Run()

		sa := testSA()
		sa.InitiatorSPI = [8]byte{0x11, 1, 1, 1, 1, 1, 1, 1}
		sa.ResponderSPI = [8]byte{0x22, 2, 2, 2, 2, 2, 2, 2}
		sa.PeerName = "established"
		table := NewSATable()
		if !table.Insert(sa) {
			t.Fatal("inserting the established SA: duplicate SPI pair")
		}
		ps := &PeerSession{peerName: sa.PeerName, inbound: make(chan transport.Packet, 4)}
		ps.ownedSA.Store(sa)
		SetActivePeersForTest(map[string]*PeerSession{sa.PeerName: ps})

		if natt {
			go dispatchNATTInbound(tr, table, log)
		} else {
			go dispatchInbound(tr, table, log)
		}

		local, ok := tr.LocalAddr().(*net.UDPAddr)
		if !ok {
			t.Fatal("transport LocalAddr is not *net.UDPAddr")
		}
		sender, err := net.DialUDP("udp4", nil, local)
		if err != nil {
			t.Fatalf("DialUDP: %v", err)
		}

		mismatched := [8]byte{0x33, 3, 3, 3, 3, 3, 3, 3}
		if _, err := sender.Write(spiPairDatagram(sa.InitiatorSPI, mismatched, natt)); err != nil {
			t.Fatalf("natt=%v: Write mismatched pair: %v", natt, err)
		}
		if _, err := sender.Write(spiPairDatagram(sa.InitiatorSPI, sa.ResponderSPI, natt)); err != nil {
			t.Fatalf("natt=%v: Write matching pair: %v", natt, err)
		}

		select {
		case pkt := <-ps.inbound:
			if len(pkt.Data) < 16 || [8]byte(pkt.Data[8:16]) != sa.ResponderSPI {
				t.Errorf("natt=%v: the first datagram delivered to the SA carries responder "+
					"SPI % x, want the SA's own % x", natt, pkt.Data[8:16], sa.ResponderSPI)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("natt=%v: the datagram with the SA's own SPI pair was never delivered", natt)
		}
		select {
		case pkt := <-ps.inbound:
			t.Errorf("natt=%v: a second datagram reached the SA; responder SPI % x",
				natt, pkt.Data[8:16])
		default:
		}

		_ = sender.Close()
		_ = tr.Close()
		SetActivePeersForTest(nil)
	}
}
