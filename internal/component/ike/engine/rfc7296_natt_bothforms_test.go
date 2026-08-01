// VALIDATES: which ESP form Ze programs for a Child SA. It also validates that BOTH forms
// are reachable on the device. The PORT the IKE SA runs on selects the encapsulated form,
// which is reached with no NAT detected. An SA that never floated gets the bare form.
//
// PREVENTS: a return to encapsulation derived from Ze's own NAT verdict. Before WP-8 a
// peer that chose port 4500 with no NAT present had its ESP dropped. Ze installed a bare
// inbound state and held port 4500 with a plain socket.
package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// bfmInstall runs createFirstChildSA for one (NAT detected, floated) combination. It
// returns the SAs that call asked the dataplane to install.
func bfmInstall(t *testing.T, natDetected, floated bool) []dataplane.SAParams {
	t.Helper()
	sa := testSA()
	sa.NATDetected = natDetected
	if floated {
		sa.floatToNATTPort()
	}
	dp := &mockDP{}
	if _, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("createFirstChildSA(nat=%v, floated=%v): %v", natDetected, floated, err)
	}
	if len(dp.sas) != 2 {
		t.Fatalf("nat=%v floated=%v: installed %d SAs, want an inbound and an outbound", natDetected, floated, len(dp.sas))
	}
	return dp.sas
}

// RFC requirement: RFC7296-2.23-11 positive -- "Implementations MUST process received
// UDP-encapsulated ESP packets even when no NAT was detected" (RFC 7296 Section 2.23,
// rfc/full/rfc7296.txt:3624-3625).
//
// Ze programs the encapsulated receive path from the PORT the IKE SA runs on. It never
// uses its own NAT verdict alone (createFirstChildSA, child.go: `sa.NATDetected ||
// sa.localPort == transport.NATTPort`).
//
// A peer that chooses UDP encapsulation without a NAT runs its IKE on port 4500.
// adoptAuthenticatedEndpoint floats the SA on that authenticated observation. The inbound
// XFRM state is then built with an ESP-in-UDP template. The antecedent "no NAT was
// detected" is therefore exactly the case asserted below.
//
// The kernel half is real too. transport.EnableESPInUDP sets UDP_ENCAP_ESPINUDP on the
// port-4500 socket. The kernel therefore decapsulates the datagram. Before WP-8 it died
// in user space.
//
// WHAT WOULD FALSIFY THIS: UDPEncap derived from sa.NATDetected alone. An inbound state
// installed without the template, while the SA runs on port 4500, does the same.
func TestBfmEncapsulatedESPAcceptedWithoutNAT(t *testing.T) {
	for _, p := range bfmInstall(t, false, true) {
		if !p.UDPEncap {
			t.Errorf("spi %d: no UDP encapsulation on an SA running on port %d with no NAT detected", p.SPI, transport.NATTPort)
			continue
		}
		if p.UDPEncapSPort != transport.NATTPort || p.UDPEncapDPort != transport.NATTPort {
			t.Errorf("spi %d: encapsulation ports %d/%d, want %d/%d",
				p.SPI, p.UDPEncapSPort, p.UDPEncapDPort, transport.NATTPort, transport.NATTPort)
		}
	}
}

// RFC requirement: RFC7296-2.23-11 negative -- the discriminator. It rests on a property
// the code HAS and not on a guard that is absent. An SA that never floated is programmed
// WITHOUT the template. The encapsulation above is therefore a decision taken from the
// port, and not a constant Ze always sets.
//
// This matters beyond test hygiene. An unconditional template would break every no-NAT
// tunnel's receive path. A state that carries an ESP-in-UDP template refuses bare ESP
// outright. That is measured against a real kernel in QEMU by
// TestEncapKernelBindsOneESPFormPerState (dataplane/encap_integration_linux_test.go).
func TestBfmBareESPKeptForUnfloatedSA(t *testing.T) {
	for _, p := range bfmInstall(t, false, false) {
		if p.UDPEncap {
			t.Errorf("spi %d: UDP encapsulation requested for an SA that never left port %d", p.SPI, transport.IKEPort)
		}
	}
}

// RFC requirement: RFC7296-2.23-10 positive -- "If Network Address Translation Traversal
// (NAT-T) is supported, all devices MUST be able to receive and process both
// UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time" (RFC 7296
// Section 2.23, rfc/full/rfc7296.txt:3544-3548).
//
// BOTH forms are reachable on the device. This asserts that both are actually produced,
// and that neither is unreachable. Ze holds port 500 for bare ESP. It holds port 4500
// with UDP_ENCAP_ESPINUDP set (transport.EnableESPInUDP) for the encapsulated form.
// installChildSA programs whichever template the SA's port calls for.
//
// PLATFORM LIMIT, stated plainly. It bounds the words "at any time". On Linux XFRM one
// inbound state accepts exactly ONE of the two forms. A state that carries an ESP-in-UDP
// template refuses bare ESP with XfrmInStateMismatch. A state without one refuses
// encapsulated ESP the same way.
//
// Two states on one SPI do not help. The state lookup is keyed on destination, SPI,
// protocol and family, and it is not encapsulation-aware. It returns the first match, and
// the encapsulation check then drops the packet.
//
// That is measured and not inferred. TestEncapKernelBindsOneESPFormPerState
// (dataplane/encap_integration_linux_test.go) drives a real kernel in QEMU and records
// the truth table. Ze therefore receives both forms across its SAs at any time. It cannot
// accept a form CHANGE on one established SA. The negative below pins that boundary.
//
// WHAT WOULD FALSIFY THIS: a device that CAN only ever program one of the two forms. A
// port-4500 socket without UDP_ENCAP set does the same, because the encapsulated form is
// then unreachable however the state was built.
func TestBfmBothESPFormsAreReachable(t *testing.T) {
	sawEncapsulated, sawBare := false, false
	for _, c := range []struct{ nat, floated bool }{
		{false, false}, // no NAT, IKE on port 500: bare ESP
		{false, true},  // no NAT, peer chose port 4500: encapsulated ESP
		{true, true},   // NAT discovered: encapsulated ESP
	} {
		for _, p := range bfmInstall(t, c.nat, c.floated) {
			if p.UDPEncap {
				sawEncapsulated = true
			} else {
				sawBare = true
			}
		}
	}
	if !sawEncapsulated {
		t.Error("no combination programmed UDP-encapsulated ESP; the device cannot receive that form")
	}
	if !sawBare {
		t.Error("no combination programmed bare ESP; the device cannot receive that form")
	}
}

// RFC requirement: RFC7296-2.23-10 negative -- the boundary of the positive. It records
// the platform limit as a property the code HAS, and not as an absent guard.
//
// One Child SA carries ONE encapsulation decision. child.UDPEncap is a single boolean,
// and installChildSA applies it to the inbound and the outbound state together. A given
// SA is therefore programmed for exactly one of the two ESP forms. "At any time" holds
// across Ze's SAs, and not within one of them.
//
// A peer that alternates forms on one SA is not handled. That is the Linux XFRM state
// model measured by TestEncapKernelBindsOneESPFormPerState. It is not a check Ze omitted.
//
// This argument does not expire when a guard arrives, because it asserts what the code
// DOES. If the two directions ever disagreed, the SA would be programmed for two forms at
// once, and the assertion would go red. Lifting the limit needs an ESP receive path off
// XFRM, which is a dataplane change of a different size.
func TestBfmOneFormPerChildSA(t *testing.T) {
	for _, c := range []struct{ nat, floated bool }{{false, false}, {false, true}, {true, true}} {
		sas := bfmInstall(t, c.nat, c.floated)
		if sas[0].UDPEncap != sas[1].UDPEncap {
			t.Errorf("nat=%v floated=%v: inbound encapsulation %v, outbound %v; one SA was programmed for both ESP forms",
				c.nat, c.floated, sas[0].UDPEncap, sas[1].UDPEncap)
		}
	}
}
