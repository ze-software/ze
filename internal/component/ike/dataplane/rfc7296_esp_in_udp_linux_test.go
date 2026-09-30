// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- UDP-encapsulated ESP on port 4500
// Related: xfrm_linux.go -- xfrmStateFromParams, the state these tests read.

//go:build linux

package dataplane

import (
	"testing"

	"github.com/vishvananda/netlink"
)

// espInUDPSA is an outbound tunnel SA to a peer behind a NAT: UDP-encapsulated on
// port 4500 at both ends, as the engine installs it when NAT detection found a NAT.
func espInUDPSA(spi uint32) SAParams {
	p := boundarySA(spi)
	p.UDPEncap = true
	p.UDPEncapSPort = 4500
	p.UDPEncapDPort = 4500
	return p
}

// RFC requirement: RFC7296-2.23-12 positive -- "To tunnel ESP packets over UDP port 4500,
// the ESP header immediately follows the UDP header." (rfc/full/rfc7296.txt, Section
// 2.23). Linux performs the encapsulation, so the proof is the state Ze installs: an
// encapsulated SA carries the XFRM_ENCAP_ESPINUDP template on ports 4500, the kernel's
// RFC 3948 form, in which the ESP header (its SPI first) immediately follows the UDP
// header. The other template, XFRM_ENCAP_ESPINUDP_NON_IKE, puts eight zero octets
// between them and is never installed.
//
// VALIDATES: xfrmStateFromParams asks the kernel for the RFC 3948 ESP-in-UDP form.
// PREVENTS: a state with the draft NON_IKE encapsulation, whose packets a conforming
// peer reads as an IKE message behind a non-ESP marker.
func TestRFC7296ESPInUDPStateHasESPRightAfterUDP(t *testing.T) {
	state, err := xfrmStateFromParams(espInUDPSA(0x2300))
	if err != nil {
		t.Fatalf("encapsulated SA refused: %v", err)
	}
	if state.Encap == nil {
		t.Fatal("encapsulated SA installed with no UDP encapsulation template")
	}
	if state.Encap.Type != netlink.XFRM_ENCAP_ESPINUDP {
		t.Fatalf("encapsulation type = %v, want XFRM_ENCAP_ESPINUDP (ESP header right after UDP)", state.Encap.Type)
	}
	if state.Encap.SrcPort != 4500 || state.Encap.DstPort != 4500 {
		t.Fatalf("encapsulation ports = %d -> %d, want 4500 -> 4500", state.Encap.SrcPort, state.Encap.DstPort)
	}
	if state.Spi != 0x2300 {
		t.Fatalf("state SPI = %#x, want %#x", state.Spi, 0x2300)
	}
}

// RFC requirement: RFC7296-2.23-12 negative -- "Since the first four octets of the ESP
// header contain the SPI, and the SPI cannot validly be zero, it is always possible to
// distinguish ESP and IKE messages." (rfc/full/rfc7296.txt, Section 2.23). An
// encapsulated SA with SPI zero would put four zero octets right after the UDP header,
// which is the non-ESP marker of an IKE message, so the peer could not tell its ESP
// from IKE. The input is forced toward that violation and no state is built.
//
// VALIDATES: an encapsulated state never carries SPI zero.
// PREVENTS: ESP on port 4500 that the receiver takes for IKE.
func TestRFC7296ESPInUDPStateRefusesSPIZero(t *testing.T) {
	state, err := xfrmStateFromParams(espInUDPSA(0))
	if err == nil {
		t.Fatalf("encapsulated SA with SPI 0 built state %+v, want a refusal", state)
	}
	if state != nil {
		t.Fatalf("refused SA still returned state %+v", state)
	}
}
