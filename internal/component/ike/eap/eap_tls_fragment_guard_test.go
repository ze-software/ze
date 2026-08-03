// Design: plan/learned/805-ipsec-11-interop-eap.md -- EAP-TLS fragment reassembly guards
// RFC: rfc/short/rfc5216.md -- Section 2.1.5 fragmentation, Section 3 flags
//
// VALIDATES: the EAP-TLS reassembly buffer is bounded whatever flags a peer
// sends, a repeated L (length) flag on a middle fragment does not discard what
// has already arrived, and a message shorter than its declared length is
// refused instead of handed to crypto/tls. Also validates that the
// authenticator sends the TLS engine's final flight rather than dropping it on
// the round that completes the handshake.
// PREVENTS: unbounded memory growth driven by an unauthenticated peer, silent
// truncation that surfaces as "tls: error decoding message", data loss when a
// peer sets L on every fragment, and an authenticator that completes without
// ever sending its own Finished.

package eap

import (
	"encoding/binary"
	"strings"
	"testing"
)

// lFragment builds an EAP-TLS TypeData carrying the L flag with declaredTotal,
// optionally the M flag, and payload bytes.
func lFragment(declaredTotal int, more bool, payload []byte) []byte {
	td := make([]byte, 5, 5+len(payload))
	td[0] = eapTLSFlagL
	if more {
		td[0] |= eapTLSFlagM
	}
	binary.BigEndian.PutUint32(td[1:5], uint32(declaredTotal))
	return append(td, payload...)
}

// plainFragment builds an EAP-TLS TypeData with no length field.
func plainFragment(more bool, payload []byte) []byte {
	td := make([]byte, 1, 1+len(payload))
	if more {
		td[0] = eapTLSFlagM
	}
	return append(td, payload...)
}

// TestReassembleBoundsBufferWithoutLengthFlag proves the reassembly buffer is
// bounded even when the peer never sets the L flag.
//
// The eapTLSMaxReassembly bound used to sit inside the L-flag branch, so a peer
// that simply never sent L left inExpected at 0, skipped the ceiling entirely,
// and could grow inBuf without limit. EAP-TLS runs before the peer is
// authenticated, so that is remote memory growth driven by an unauthenticated
// party (ai/rules/evidence.md).
func TestReassembleBoundsBufferWithoutLengthFlag(t *testing.T) {
	var f tlsFragmenter
	chunk := make([]byte, 4096)

	var err error
	for range (eapTLSMaxReassembly / len(chunk)) + 8 {
		if err = f.reassemble(plainFragment(true, chunk)); err != nil {
			break
		}
	}

	if err == nil {
		t.Fatalf("reassemble accepted %d bytes with no L flag, want refusal above %d",
			len(f.inBuf), eapTLSMaxReassembly)
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("error = %q, want it to name the size limit", err)
	}
	if len(f.inBuf) > eapTLSMaxReassembly {
		t.Fatalf("buffer grew to %d bytes, past the %d ceiling", len(f.inBuf), eapTLSMaxReassembly)
	}
}

// TestReassembleKeepsDataWhenLengthFlagRepeats proves a repeated L flag does not
// discard the fragments already received.
//
// RFC 5216 Section 2.1.5 makes the L flag REQUIRED on the first fragment and
// OPTIONAL on middle fragments, so a conformant peer may set it on every one.
// Resetting the buffer on each L kept only the final fragment and reported no
// error, which reaches crypto/tls as a truncated record.
func TestReassembleKeepsDataWhenLengthFlagRepeats(t *testing.T) {
	var f tlsFragmenter
	part := []byte("0123456789")
	total := len(part) * 3

	for i := range 3 {
		last := i == 2
		if err := f.reassemble(lFragment(total, !last, part)); err != nil {
			t.Fatalf("fragment %d: %v", i, err)
		}
	}

	got := f.drainReassembled()
	if len(got) != total {
		t.Fatalf("reassembled %d bytes, want %d (a repeated L flag discarded data)", len(got), total)
	}
	if string(got) != strings.Repeat(string(part), 3) {
		t.Fatalf("reassembled %q, want three copies of %q", got, part)
	}
}

// TestReassembleRejectsConflictingDeclaredLength proves a middle fragment that
// re-declares a DIFFERENT total is refused rather than silently adopted.
func TestReassembleRejectsConflictingDeclaredLength(t *testing.T) {
	var f tlsFragmenter
	part := []byte("0123456789")

	if err := f.reassemble(lFragment(30, true, part)); err != nil {
		t.Fatalf("first fragment: %v", err)
	}
	err := f.reassemble(lFragment(999, true, part))
	if err == nil {
		t.Fatal("reassemble accepted a second L flag declaring a different total, want refusal")
	}
	if !strings.Contains(err.Error(), "999") || !strings.Contains(err.Error(), "30") {
		t.Fatalf("error = %q, want it to name both the new and the original length", err)
	}
}

// TestReassemblyCompleteRejectsShortMessage proves a message that ends before
// its declared length is reported incomplete.
//
// Completeness used to be judged from the M flag alone. A peer whose last
// fragment carried fewer octets than the L field declared therefore had its
// short buffer handed straight to crypto/tls, which answers with the opaque
// "local error: tls: error decoding message" (ai/rules/evidence.md).
func TestReassemblyCompleteRejectsShortMessage(t *testing.T) {
	var f tlsFragmenter
	if err := f.reassemble(lFragment(3000, true, make([]byte, 1024))); err != nil {
		t.Fatalf("first fragment: %v", err)
	}
	if err := f.reassemble(plainFragment(false, make([]byte, 500))); err != nil {
		t.Fatalf("last fragment: %v", err)
	}

	if f.reassemblyComplete() {
		t.Fatalf("reassemblyComplete() = true with %d of %d declared octets, want false",
			len(f.inBuf), f.inExpected)
	}

	// A message that arrives whole is complete.
	var whole tlsFragmenter
	if err := whole.reassemble(lFragment(10, false, make([]byte, 10))); err != nil {
		t.Fatalf("whole message: %v", err)
	}
	if !whole.reassemblyComplete() {
		t.Fatal("reassemblyComplete() = false for a message that arrived whole, want true")
	}

	// A message that declared no length has nothing to check against.
	var undeclared tlsFragmenter
	if err := undeclared.reassemble(plainFragment(false, make([]byte, 10))); err != nil {
		t.Fatalf("undeclared message: %v", err)
	}
	if !undeclared.reassemblyComplete() {
		t.Fatal("reassemblyComplete() = false when no length was declared, want true")
	}
}

// TestAuthenticatorSendsFinalFlightBeforeCompleting proves the authenticator
// transmits the TLS engine's last output instead of discarding it on the round
// that completes the handshake.
//
// Go's TLS 1.2 server writes ChangeCipherSpec and Finished and only THEN returns
// from HandshakeContext, so those 51 octets are produced in the very round that
// sets handshaked. Returning Done there dropped the server Finished: the peer
// waited for it forever and its MSK stayed zero, so the IKEv2 AUTH payload
// (RFC 7296 Section 2.16) was computed over keys the two ends did not share.
// The peer side already documents this ordering; the authenticator did not.
func TestAuthenticatorSendsFinalFlightBeforeCompleting(t *testing.T) {
	m := &tlsMethod{state: tlsStateHandshake, transport: newEAPTLSTransport()}

	// The engine finished and left its closing flight in the buffer.
	final := []byte("change-cipher-spec-and-finished")
	m.transport.serverBuf = append(m.transport.serverBuf, final...)
	m.transport.finished = true
	m.handshaked.Store(true)

	res := m.Process(&Packet{Code: CodeResponse, Type: TypeTLS, TypeData: []byte{0}})
	if res.Err != nil {
		t.Fatalf("Process: %v", res.Err)
	}
	if res.Done {
		t.Fatal("Process reported Done while the final TLS flight was still unsent")
	}
	if res.Response == nil {
		t.Fatal("Process returned no response, so the final TLS flight was dropped")
	}
	// TypeData is the L-prefixed first fragment: 5 header octets then the flight.
	if len(res.Response.TypeData) < 5 {
		t.Fatalf("TypeData is %d octets, too short to carry the flight", len(res.Response.TypeData))
	}
	if got := string(res.Response.TypeData[5:]); got != string(final) {
		t.Fatalf("sent %q, want the engine's final flight %q", got, final)
	}
}
