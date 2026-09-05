package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
)

// TestTransportSelectorsMatchTheIKESAAddressesBehindANAT drives the two client-side MUSTs
// of RFC 7296 Section 2.23.1 over a path that really translates addresses.
//
// The fixture is the section's own figure: the client sits behind NAT A at IP1 and dials
// IPN2, the outer address of NAT B, behind which the server sits at IP2. A conforming
// server answers in ITS address space, IPN1 and IP2, and neither of those is an address
// the client's own IKE SA runs on.
//
// VALIDATES: after the client adopts that answer, the selectors it holds carry exactly one
// IP address each, the TSi address is the source address of the client's IKE SA, and the
// TSr address is its destination address.
// PREVENTS: the two rows being satisfied only on a NAT-free path, where the addresses the
// server answers with happen to equal the client's own pair. On a translated path the
// answer differs from the requirement, and the substitution is what makes the rows true.
func TestTransportSelectorsMatchTheIKESAAddressesBehindANAT(t *testing.T) {
	sa := natInitiatorSA(true, true)

	// The server answers with the pair ITS stack sees: the client at IPN1 and itself at
	// IP2. Both differ from the client's own IKE SA addresses.
	answerTSi := tsPayload(t, wire.PayloadTypeTSi, natClientPublic+"/32")
	answerTSr := tsPayload(t, wire.PayloadTypeTSr, natServerReal+"/32")
	if err := recordInitiatorSelectors(sa, answerTSi, answerTSr, nil); err != nil {
		t.Fatalf("the client refused a conforming answer: %v", err)
	}
	if len(sa.NegotiatedPairs) != 1 {
		t.Fatalf("the client holds %d selector pairs, want 1; the assertions below would check nothing",
			len(sa.NegotiatedPairs))
	}

	// RFC requirement: RFC7296-2.23.1-2 positive -- "The TSi entries MUST have exactly one
	// IP address, and that MUST match the source address of the IKE SA" (RFC 7296 S2.23.1,
	// rfc/full/rfc7296.txt:3819-3820). The IKE SA's source address on this node is its
	// local-address, IP1. The selectors the client holds after adoption carry that address,
	// as a single-address /32.
	if got := sa.NegotiatedTSi.String(); got != natClientReal+"/32" {
		t.Errorf("adopted TSi = %s, want the IKE SA source %s/32", got, natClientReal)
	}
	if !singleAddress(sa.NegotiatedTSi) {
		t.Errorf("adopted TSi %v spans more than one IP address", sa.NegotiatedTSi)
	}

	// RFC requirement: RFC7296-2.23.1-3 positive -- "The TSr entries MUST have exactly one
	// IP address, and that MUST match the destination address of the IKE SA" (RFC 7296
	// S2.23.1, rfc/full/rfc7296.txt:3822-3823). The IKE SA's destination address on this
	// node is the address it dials, IPN2.
	if got := sa.NegotiatedTSr.String(); got != natServerPublic+"/32" {
		t.Errorf("adopted TSr = %s, want the IKE SA destination %s/32", got, natServerPublic)
	}
	if !singleAddress(sa.NegotiatedTSr) {
		t.Errorf("adopted TSr %v spans more than one IP address", sa.NegotiatedTSr)
	}

	// RFC requirement: RFC7296-2.23.1-2 negative -- the discriminator. The address the
	// SERVER put in TSi, IPN1, is not the source address of this node's IKE SA, and it does
	// not survive adoption. Without this assertion the positive above would also pass on an
	// implementation that echoed whatever the peer answered, on any fixture where the two
	// happened to agree.
	if sa.NegotiatedTSi.IP.String() == natClientPublic {
		t.Errorf("adopted TSi kept the server's %s; S2.23.1 requires the IKE SA source address", natClientPublic)
	}

	// RFC requirement: RFC7296-2.23.1-3 negative -- the same discriminator for TSr. The
	// server answered with its own IP2, which is not the destination address of this node's
	// IKE SA, and it does not survive adoption.
	if sa.NegotiatedTSr.IP.String() == natServerReal {
		t.Errorf("adopted TSr kept the server's %s; S2.23.1 requires the IKE SA destination address", natServerReal)
	}
}

// TestResponderAnswersTheIKESAAddressesBehindANAT is the responder half of the same
// figure. Section 2.23.1 states the outcome directly: "it will thus send back Traffic
// Selectors having IPN1 and IP2 as their IP addresses" and "The SAD entry created for the
// Child SA will have the addresses as seen by the server, namely IPN1 and IP2."
//
// VALIDATES: the responder answers a client behind a NAT with the address pair its own
// stack sees, one IP address each, rather than with the pre-NAT pair the client proposed.
// PREVENTS: the responder answering the client's IP1, which its own kernel never sees on
// the wire, so the SAD entry would match no packet and the tunnel would carry nothing.
func TestResponderAnswersTheIKESAAddressesBehindANAT(t *testing.T) {
	sa := natResponderSA(true, true)
	if err := narrowChildSelectors(sa,
		tsPayload(t, wire.PayloadTypeTSi, natClientReal+"/32"),
		tsPayload(t, wire.PayloadTypeTSr, natServerPublic+"/32"), nil); err != nil {
		t.Fatalf("the responder refused a conforming transport-mode proposal: %v", err)
	}

	answerTSi, answerTSr := pairsToWire(sa.NegotiatedPairs)
	if answerTSi == nil || answerTSr == nil {
		t.Fatal("the responder built no TS payloads")
	}
	if len(answerTSi.TrafficSelectors) != 1 || len(answerTSr.TrafficSelectors) != 1 {
		t.Fatalf("the answer carried %d TSi and %d TSr selectors, want 1 each",
			len(answerTSi.TrafficSelectors), len(answerTSr.TrafficSelectors))
	}
	gotI := answerTSi.TrafficSelectors[0]
	gotR := answerTSr.TrafficSelectors[0]

	// RFC requirement: RFC7296-2.23.1-1 positive -- "For transport mode, it MUST use
	// exactly one IP address in the TSi and TSr payloads" (RFC 7296 S2.23.1,
	// rfc/full/rfc7296.txt:3712-3714). Each answered entry spans one address.
	if !bytesEqual(gotI.StartAddress, gotI.EndAddress) {
		t.Errorf("answered TSi spans %v-%v, want exactly one IP address", gotI.StartAddress, gotI.EndAddress)
	}
	if !bytesEqual(gotR.StartAddress, gotR.EndAddress) {
		t.Errorf("answered TSr spans %v-%v, want exactly one IP address", gotR.StartAddress, gotR.EndAddress)
	}

	// RFC requirement: RFC7296-2.23.1-1 negative -- the discriminator. The single address is
	// not the one the client proposed. The client sent IP1 and IPN2, and both are replaced,
	// so the answer states the responder's own view rather than echoing the proposal.
	if net.IP(gotI.StartAddress).String() == natClientReal {
		t.Errorf("answered TSi echoed the client's pre-NAT %s, which this node's kernel never sees", natClientReal)
	}
	if net.IP(gotR.StartAddress).String() == natServerPublic {
		t.Errorf("answered TSr echoed the NAT's outer %s rather than this node's own address", natServerPublic)
	}
	if got := net.IP(gotI.StartAddress).String(); got != natClientPublic {
		t.Errorf("answered TSi = %s, want the observed remote %s", got, natClientPublic)
	}
	if got := net.IP(gotR.StartAddress).String(); got != natServerReal {
		t.Errorf("answered TSr = %s, want the observed local %s", got, natServerReal)
	}
}
