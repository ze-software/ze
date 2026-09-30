// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS fragment reassembly guards
// RFC: rfc/short/rfc5216.md -- Section 2.1.5 fragmentation and reassembly, Section 3 flags
//
// VALIDATES: the receive side refuses a fragmented TLS message whose first
// fragment lacks the L bit, and refuses a reassembly whose octet total exceeds
// or falls short of the TLS Message Length the first fragment declared. Each
// refusal is observed at the EAP peer's request handler, where acceptance would
// show as a fragment ACK or as data fed to the TLS engine.
// PREVENTS: a malformed fragment train being buffered without a declared
// length, and a reassembled buffer that disagrees with its declared length
// reaching crypto/tls.

package eap

import (
	"strings"
	"testing"
)

// feedPeerTLSRequest hands one EAP-TLS Request carrying typeData to a peer's
// EAP-TLS handler and returns what the handler answered.
func feedPeerTLSRequest(ps *PeerSession, id uint8, typeData []byte) PeerResult {
	return ps.handleTLSRequest(&Packet{Code: CodeRequest, Identifier: id, Type: TypeTLS, TypeData: typeData})
}

// isFragmentACK reports whether res is the empty EAP-TLS Response that
// acknowledges a fragment (RFC 5216 Section 2.1.5).
func isFragmentACK(res PeerResult) bool {
	if res.Response == nil {
		return false
	}
	if len(res.Response.TypeData) != 1 {
		return false
	}
	return res.Response.TypeData[0] == 0
}

// TestRFC5216FirstFragmentWithoutLengthIsRefused feeds the peer the first
// fragment of a fragmented TLS message (M set) with the L bit clear, and
// asserts it is refused as malformed: an error, no fragment ACK, and nothing
// buffered. The control feeds the same fragment with L set and asserts the ACK.
func TestRFC5216FirstFragmentWithoutLengthIsRefused(t *testing.T) {
	payload := make([]byte, 1024)

	// Control: a conformant first fragment (L and M set) is acknowledged.
	good := &PeerSession{tlsTransport: newEAPTLSTransport()}
	res := feedPeerTLSRequest(good, 1, lFragment(3000, true, payload))
	if res.Err != nil {
		t.Fatalf("conformant first fragment refused: %v", res.Err)
	}
	if !isFragmentACK(res) {
		t.Fatalf("conformant first fragment was not acknowledged: %+v", res.Response)
	}

	// RFC requirement: RFC5216-3-1 negative -- a first fragment of a fragmented
	// TLS message (M set) whose L bit is clear is refused as malformed: the peer
	// returns an error, sends no fragment ACK, and buffers none of its octets.
	bad := &PeerSession{tlsTransport: newEAPTLSTransport()}
	res = feedPeerTLSRequest(bad, 1, plainFragment(true, payload))
	if res.Err == nil {
		t.Fatal("first fragment without the L bit was accepted, want refusal")
	}
	if !strings.Contains(res.Err.Error(), "L bit") {
		t.Fatalf("error = %q, want it to name the missing L bit", res.Err)
	}
	if isFragmentACK(res) {
		t.Fatal("first fragment without the L bit was acknowledged")
	}
	if len(bad.inBuf) != 0 {
		t.Fatalf("buffered %d octets of a malformed first fragment, want 0", len(bad.inBuf))
	}
}

// TestRFC5216ReassemblyLongerThanDeclaredIsRefused declares a 30-octet TLS
// message on the first fragment, then sends 40 octets across two fragments,
// and asserts the fragment that overruns the declared length is refused.
func TestRFC5216ReassemblyLongerThanDeclaredIsRefused(t *testing.T) {
	part := make([]byte, 20)
	ps := &PeerSession{tlsTransport: newEAPTLSTransport()}

	res := feedPeerTLSRequest(ps, 1, lFragment(30, true, part))
	if res.Err != nil {
		t.Fatalf("first fragment refused: %v", res.Err)
	}
	if !isFragmentACK(res) {
		t.Fatal("first fragment was not acknowledged")
	}

	// RFC requirement: RFC5216-2.1.5-1 negative -- reassembly refuses a fragment
	// that carries the total past the TLS Message Length the first fragment
	// declared: an error, no ACK, and the buffer keeps only the declared prefix.
	res = feedPeerTLSRequest(ps, 2, plainFragment(false, part))
	if res.Err == nil {
		t.Fatal("reassembly past the declared TLS Message Length was accepted, want refusal")
	}
	if !strings.Contains(res.Err.Error(), "exceeds declared length") {
		t.Fatalf("error = %q, want it to name the declared length overrun", res.Err)
	}
	if res.Response != nil {
		t.Fatalf("overrunning fragment was answered: %+v", res.Response)
	}
	if len(ps.inBuf) != len(part) {
		t.Fatalf("buffer holds %d octets, want the %d accepted before the overrun", len(ps.inBuf), len(part))
	}
}

// TestRFC5216ReassemblyShorterThanDeclaredIsRefused declares a 3000-octet TLS
// message, then ends it (M clear) after 1524 octets, and asserts the short
// message is refused instead of being fed to the TLS engine.
func TestRFC5216ReassemblyShorterThanDeclaredIsRefused(t *testing.T) {
	ps := &PeerSession{tlsTransport: newEAPTLSTransport()}

	res := feedPeerTLSRequest(ps, 1, lFragment(3000, true, make([]byte, 1024)))
	if res.Err != nil {
		t.Fatalf("first fragment refused: %v", res.Err)
	}

	// RFC requirement: RFC5216-2.1.5-1 negative -- a fragment train that ends
	// before the TLS Message Length the first fragment declared is refused: the
	// peer returns an error naming both counts, sends nothing, and the short
	// buffer is not drained into the TLS engine.
	res = feedPeerTLSRequest(ps, 2, plainFragment(false, make([]byte, 500)))
	if res.Err == nil {
		t.Fatal("reassembly short of the declared TLS Message Length was accepted, want refusal")
	}
	if !strings.Contains(res.Err.Error(), "1524 of 3000") {
		t.Fatalf("error = %q, want it to name 1524 of 3000 declared octets", res.Err)
	}
	if res.Response != nil {
		t.Fatalf("short message was answered: %+v", res.Response)
	}
	if len(ps.inBuf) != 1524 {
		t.Fatalf("buffer holds %d octets, want the 1524 undrained", len(ps.inBuf))
	}
}
