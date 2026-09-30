// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Child SA install
// Related: child.go -- installChildSA, the producer of the inbound SA
// Related: rfc4303_inbound_key_linux_test.go -- the namespace probe of the same key on real XFRM
// RFC: rfc/short/rfc4303.md -- the inbound SA lookup key an IKE Child SA sets (Section 2.1)
package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// inKeySPI is the inbound SPI Ze advertised in its SA payload for the Child SA.
const inKeySPI = 0x0c0ffee1

// TestRFC4303InboundChildSAKeyedOnNegotiatedSPIAndLocalAddress proves that the IKE
// negotiation sets how inbound ESP maps to the Child SA's SA. Method: createFirstChildSA
// with a recorded inbound SPI; the one inbound SA it installs must carry that SPI,
// protocol 50, and the negotiated local address as its destination, so the SA is keyed on
// the destination and the SPI Ze negotiated.
func TestRFC4303InboundChildSAKeyedOnNegotiatedSPIAndLocalAddress(t *testing.T) {
	// RFC requirement: RFC4303-2.1-2 positive -- the inbound SA Ze installs for an IKE Child SA carries the negotiated inbound SPI, protocol 50 and the negotiated local address as its destination.
	const local, remote = "10.0.0.1", "10.0.0.2"
	sa := testSA()
	sa.ChildInboundSPI = inKeySPI
	dp := &mockDP{}

	child, err := createFirstChildSA(sa, testESPGroup(), local, remote, 42, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	defer child.Clear()

	var inbound []dataplane.SAParams
	for _, s := range dp.sas {
		if s.Dir == dataplane.SADirIn {
			inbound = append(inbound, s)
		}
	}
	if len(inbound) != 1 {
		t.Fatalf("inbound SAs installed = %d, want 1", len(inbound))
	}
	in := inbound[0]
	if in.SPI != inKeySPI {
		t.Errorf("inbound SA SPI = %#x, want the negotiated %#x", in.SPI, inKeySPI)
	}
	if in.Proto != 50 {
		t.Errorf("inbound SA protocol = %d, want ESP (50)", in.Proto)
	}
	if !in.Dst.Equal(net.ParseIP(local)) {
		t.Errorf("inbound SA destination = %v, want the negotiated local address %s", in.Dst, local)
	}
}
