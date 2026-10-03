// Design: docs/features/interfaces.md -- DHCPv4 client lifecycle
// Related: dhcp_shortframe_integration_linux_test.go -- the veth lab and frame helpers
//
// VALIDATES: RFC 2131 Section 2 -- "The remaining bits of the flags field are
// reserved for future use. They MUST be set to zero by clients".
// PREVENTS: a client that copies an OFFER's flags word into its REQUEST. The
// vendored dhcpv4.NewRequestFromOffer fills the REQUEST's flags from the OFFER
// (WithReply), so a server that sets a reserved bit would have ze's REQUEST
// carry it. The renewal has the same shape: dhcpv4.NewRenewFromAck fills the
// renewing REQUEST's flags from the ACK.

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

// renewalLeaseSeconds is the lease the ACK grants, so T1 (half of it) comes
// one second after the lease is bound and the renewal runs inside the test.
const renewalLeaseSeconds = 2

// renewalReadsMax bounds the REQUESTs read while looking for the renewal: a
// retransmitted acquisition REQUEST may arrive first.
const renewalReadsMax = 5

// TestRFC2131ClientRenewalSetsReservedFlagsBitsToZero drives the real client
// through acquisition and into its T1 renewal over a veth pair. Method: the
// OFFER is clean, the ACK that binds the lease sets every flags bit and grants
// a two-second lease, and the renewing REQUEST ze builds from that ACK is read.
// The renewal is the second producer path: renewV4 passes clearReservedFlags
// to client.Renew, and dropping that argument sends the ACK's reserved bits
// back.
func TestRFC2131ClientRenewalSetsReservedFlagsBitsToZero(t *testing.T) {
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
	offer, err := dhcpv4.NewReplyFromRequest(discover,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
		dhcpv4.WithServerIP(sfServerIP),
		dhcpv4.WithYourIP(sfClientIP),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(sfServerIP)),
		dhcpv4.WithOption(dhcpv4.OptSubnetMask(net.CIDRMask(24, 32))),
		dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(renewalLeaseSeconds*time.Second)),
	)
	if err != nil {
		t.Fatalf("build offer: %v", err)
	}
	if err := sfWrite(peerConn, sfUDPFrame(offer.ToBytes())); err != nil {
		t.Fatalf("write offer: %v", err)
	}

	request := sfReadDHCP(t, peerConn, dhcpv4.MessageTypeRequest, 10*time.Second)
	ack, err := dhcpv4.NewReplyFromRequest(request,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
		dhcpv4.WithServerIP(sfServerIP),
		dhcpv4.WithYourIP(sfClientIP),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(sfServerIP)),
		dhcpv4.WithOption(dhcpv4.OptSubnetMask(net.CIDRMask(24, 32))),
		dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(renewalLeaseSeconds*time.Second)),
	)
	if err != nil {
		t.Fatalf("build ack: %v", err)
	}
	ack.Flags = 0xffff
	if err := sfWrite(peerConn, sfUDPFrame(ack.ToBytes())); err != nil {
		t.Fatalf("write ack: %v", err)
	}

	// The renewing REQUEST is told apart by ciaddr: the acquisition REQUEST
	// carries 0.0.0.0, the renewal the bound address (RFC 2131 Section 4.3.2).
	// The xid cannot tell them apart, because NewRenewFromAck copies the ACK's.
	var renewal *dhcpv4.DHCPv4
	for range renewalReadsMax {
		next := sfReadDHCP(t, peerConn, dhcpv4.MessageTypeRequest, 10*time.Second)
		if !next.ClientIPAddr.IsUnspecified() {
			renewal = next
			break
		}
	}
	if renewal == nil {
		t.Fatalf("no renewing REQUEST (ciaddr set) within %d REQUESTs", renewalReadsMax)
	}
	if !renewal.ClientIPAddr.Equal(sfClientIP) {
		t.Fatalf("REQUEST ciaddr = %v, want the bound %v: not a renewal", renewal.ClientIPAddr, sfClientIP)
	}
	// RFC requirement: RFC2131-2-3 negative -- answered by an ACK whose flags word has every reserved bit set, the client renews with a DHCPREQUEST whose flags bits 1 to 15 are zero.
	if got := renewal.Flags & flagsReserved; got != 0 {
		t.Errorf("renewing REQUEST flags = %#04x, reserved bits %#04x copied from the ACK", renewal.Flags, got)
	}
}
