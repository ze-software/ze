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

// bfmLocalAddr is the local tunnel endpoint bfmInstall builds its Child SA with. The
// INBOUND state is the one the peer sends to, so it is the state whose Dst is this
// address. Identifying the two directions by address rather than by install order keeps
// the assertions honest if that order ever changes.
const bfmLocalAddr = "10.0.0.1"

// bfmDirections splits one Child SA's installed states into the inbound state (what Ze
// RECEIVES on) and the outbound state (what Ze SENDS with).
func bfmDirections(t *testing.T, natDetected, floated bool) (inbound, outbound dataplane.SAParams) {
	t.Helper()
	var haveIn, haveOut bool
	sas := bfmInstall(t, natDetected, floated)
	for i := range sas {
		if sas[i].Dst.String() == bfmLocalAddr {
			inbound, haveIn = sas[i], true
			continue
		}
		outbound, haveOut = sas[i], true
	}
	if !haveIn || !haveOut {
		t.Fatalf("nat=%v floated=%v: inbound found %v, outbound found %v; the split is wrong and every assertion below would be vacuous",
			natDetected, floated, haveIn, haveOut)
	}
	return inbound, outbound
}

// RFC requirement: RFC7296-2.23-11 positive -- "Implementations MUST process received
// UDP-encapsulated ESP packets even when no NAT was detected" (RFC 7296 Section 2.23,
// rfc/full/rfc7296.txt:3624-3625).
//
// It used to assert that EVERY state of a floated SA carried the encapsulation template,
// inbound and outbound alike. That conflated two decisions. RECEPTION is what this MUST
// governs, and the paragraph's last sentence governs TRANSMISSION separately: "if a NAT
// is detected, both devices MUST use UDP encapsulation for ESP"
// (rfc/full/rfc7296.txt:3550-3551). With no NAT the choice is explicitly free
// (:3548-3550), so requiring Ze to SEND the encapsulated form there was never the RFC's
// ask, and it broke interop against a strongSwan that floats for MOBIKE and sends bare.
//
// The assertions are now strictly stronger. The inbound state must still carry the
// template, which is the encapsulated form being RECEIVED with no NAT detected. It must
// ALSO accept the bare form, which the old assertion never checked. And the outbound
// state must NOT encapsulate, which the old assertion had backwards.
//
// WHAT WOULD FALSIFY THIS: an inbound state built without the template while the SA runs
// on port 4500, which loses the encapsulated receive path. An inbound state that does not
// accept both forms does the same for the bare one. An outbound state that encapsulates
// with no NAT detected sends a form the peer never asked for.
func TestBfmEncapsulatedESPAcceptedWithoutNAT(t *testing.T) {
	inbound, outbound := bfmDirections(t, false, true)

	// RECEIVE: the encapsulated form, with no NAT detected. This is the MUST itself.
	if !inbound.UDPEncap {
		t.Errorf("inbound spi %d: no UDP encapsulation template on an SA running on port %d with no NAT detected; encapsulated ESP is not received",
			inbound.SPI, transport.NATTPort)
	} else if inbound.UDPEncapSPort != transport.NATTPort || inbound.UDPEncapDPort != transport.NATTPort {
		t.Errorf("inbound spi %d: encapsulation ports %d/%d, want %d/%d",
			inbound.SPI, inbound.UDPEncapSPort, inbound.UDPEncapDPort, transport.NATTPort, transport.NATTPort)
	}

	// RECEIVE: the bare form too, on the SAME state. RFC 7296 Section 2.23 asks for both
	// "at any time", and one XFRM state binds one form, so the second form is served
	// beside the kernel through this flag.
	if !inbound.AcceptBothESPForms {
		t.Errorf("inbound spi %d: the state accepts one ESP form only; bare ESP on this SA is dropped",
			inbound.SPI)
	}

	// SEND: bare, because no NAT was detected. RFC 7296 requires encapsulation only when
	// a NAT is present.
	if outbound.UDPEncap {
		t.Errorf("outbound spi %d: UDP encapsulation with no NAT detected; the send form must follow the NAT verdict, not the port the IKE SA floated to",
			outbound.SPI)
	}
}

// RFC requirement: RFC7296-2.23-11 positive -- the NAT half of the same paragraph. "if a
// NAT is detected, both devices MUST use UDP encapsulation for ESP" (RFC 7296
// Section 2.23, rfc/full/rfc7296.txt:3550-3551).
//
// The test above proves Ze sends bare when no NAT is detected. Without this one, a build
// that NEVER encapsulated on transmission would satisfy it, and that build would violate
// the MUST above and break every NAT-traversing tunnel. The pair is what makes the send
// decision a decision rather than a constant.
func TestBfmEncapsulatedESPSentWhenNATDetected(t *testing.T) {
	inbound, outbound := bfmDirections(t, true, true)

	if !outbound.UDPEncap {
		t.Errorf("outbound spi %d: no UDP encapsulation with a NAT detected; RFC 7296 Section 2.23 makes it mandatory there",
			outbound.SPI)
	}
	if outbound.UDPEncapSPort != transport.NATTPort || outbound.UDPEncapDPort != transport.NATTPort {
		t.Errorf("outbound spi %d: encapsulation ports %d/%d, want %d/%d",
			outbound.SPI, outbound.UDPEncapSPort, outbound.UDPEncapDPort, transport.NATTPort, transport.NATTPort)
	}
	if !inbound.AcceptBothESPForms {
		t.Errorf("inbound spi %d: a NAT-traversing SA accepts one ESP form only", inbound.SPI)
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
// MEASURED. TestEncapKernelBindsOneESPFormPerState
// (dataplane/encap_integration_linux_test.go) drives a real kernel in QEMU and records
// that truth table. It installs its two states on two DISTINCT SPIs.
//
// REASONED, and not measured: two states on ONE SPI do not help either. The state lookup
// is keyed on destination, SPI, protocol and family, so it returns the first match and
// the mismatch check then drops the packet. No test installs two states on one SPI.
// plan/spec-ipsec-esp-dual-form-receive.md carries that as an assumption to validate, and
// it owns the work of lifting the constraint.
//
// Ze therefore receives both forms across its SAs at any time. It cannot accept a form
// CHANGE on one established SA. The negative below pins that boundary.
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
// the dual-form property as something the code HAS, and not as an absent guard.
//
// It used to assert `inbound.UDPEncap == outbound.UDPEncap`, which IS the limit this work
// removes: one boolean applied to both directions, so one Child SA served exactly one ESP
// form. That is no longer true and must no longer be asserted.
//
// What replaces it is stronger in two ways. EVERY Child SA, in every combination of NAT
// verdict and port, must accept both ESP forms inbound, which is "at any time" holding
// WITHIN one SA rather than only across Ze's SAs. And the two directions must be
// genuinely independent decisions: at least one combination sends a different form from
// the one whose template it receives on. A build that re-merged them into one boolean
// would satisfy the first assertion and fail the second.
//
// The kernel half is measured, not argued. TestEncapOneStateAcceptsBothForms drives a
// real kernel in QEMU: ONE state, ONE SPI, the encapsulated form on the kernel fast path
// and the bare form read off a raw socket and re-presented, both reaching the crypto
// check. TestEncapTwoStatesOneSPI measures why a second state is not the answer: the
// kernel refuses to install one.
//
// WHAT WOULD FALSIFY THIS: an inbound state programmed for one form only, which drops the
// other. A build that derives the send form from the receive form, which is the merged
// boolean returning under another name.
func TestBfmBothESPFormsReceivedOnOneChildSA(t *testing.T) {
	sawIndependent := false
	for _, c := range []struct{ nat, floated bool }{{false, false}, {false, true}, {true, true}} {
		inbound, outbound := bfmDirections(t, c.nat, c.floated)

		if !inbound.AcceptBothESPForms {
			t.Errorf("nat=%v floated=%v: inbound spi %d accepts one ESP form only; a peer that changes form on this SA is not served",
				c.nat, c.floated, inbound.SPI)
		}
		if inbound.UDPEncap != outbound.UDPEncap {
			sawIndependent = true
		}
	}

	// The discriminator. Without it, a build that copied the receive decision onto the
	// send side would pass every assertion above.
	if !sawIndependent {
		t.Error("no combination sent a different ESP form from the one its inbound template serves; the two directions are still one decision")
	}
}
