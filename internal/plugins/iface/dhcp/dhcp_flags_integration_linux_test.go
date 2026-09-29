// Design: docs/features/interfaces.md -- DHCPv4 client lifecycle
// Related: dhcp_shortframe_integration_linux_test.go -- the veth lab and frame helpers
//
// VALIDATES: RFC 2131 Section 2 -- "The remaining bits of the flags field are
// reserved for future use. They MUST be set to zero by clients".
// PREVENTS: a client that copies an OFFER's flags word into its REQUEST. The
// vendored dhcpv4.NewRequestFromOffer fills the REQUEST's flags from the OFFER
// (WithReply), so a server that sets a reserved bit would have ze's REQUEST
// carry it.

//go:build integration && linux

package ifacedhcp

import (
	"net"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// flagsReserved masks bits 1 to 15 of the flags field. Bit 0, the most
// significant, is BROADCAST.
const flagsReserved = 0x7fff

// TestRFC2131ClientSetsReservedFlagsBitsToZero drives the real client over a
// veth pair. Method: the DISCOVER ze sends is read first; the OFFER that
// answers it sets every flags bit, and the REQUEST ze builds from that OFFER
// is read next.
func TestRFC2131ClientSetsReservedFlagsBitsToZero(t *testing.T) {
	zeSide, peerConn := sfSetupLab(t)

	bus := &recordingBus{}
	client, err := newDHCPClient(zeSide, "0", bus, true, false, dHCPConfig{})
	if err != nil {
		t.Fatalf("newDHCPClient(%s): %v", zeSide, err)
	}
	if err := client.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(client.Stop)

	discover := sfReadDHCP(t, peerConn, dhcpv4.MessageTypeDiscover, 10*time.Second)
	// RFC requirement: RFC2131-2-3 positive -- the DHCPDISCOVER the client sends has flags bits 1 to 15 set to zero.
	if got := discover.Flags & flagsReserved; got != 0 {
		t.Errorf("DISCOVER flags = %#04x, reserved bits %#04x set", discover.Flags, got)
	}

	offer, err := dhcpv4.NewReplyFromRequest(discover,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
		dhcpv4.WithServerIP(sfServerIP),
		dhcpv4.WithYourIP(sfClientIP),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(sfServerIP)),
		dhcpv4.WithOption(dhcpv4.OptSubnetMask(net.CIDRMask(24, 32))),
		dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(10*time.Minute)),
	)
	if err != nil {
		t.Fatalf("build offer: %v", err)
	}
	offer.Flags = 0xffff
	if err := sfWrite(peerConn, sfUDPFrame(offer.ToBytes())); err != nil {
		t.Fatalf("write offer: %v", err)
	}

	request := sfReadDHCP(t, peerConn, dhcpv4.MessageTypeRequest, 10*time.Second)
	if request.TransactionID != discover.TransactionID {
		t.Fatalf("REQUEST xid = %v, want the DISCOVER's %v", request.TransactionID, discover.TransactionID)
	}
	// RFC requirement: RFC2131-2-3 negative -- answered by an OFFER whose flags word has every reserved bit set, the client still sends its DHCPREQUEST with flags bits 1 to 15 set to zero.
	if got := request.Flags & flagsReserved; got != 0 {
		t.Errorf("REQUEST flags = %#04x, reserved bits %#04x copied from the OFFER", request.Flags, got)
	}
}
