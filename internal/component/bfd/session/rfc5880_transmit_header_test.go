package session

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// transmittedHeaders drives one machine through every state it transmits in
// (Down, Init, Up, a Final answering a Poll, AdminDown) and returns the wire
// bytes of each packet Build or BuildFinal produced, encoded by WriteTo.
func transmittedHeaders(t *testing.T) map[string][]byte {
	t.Helper()
	m, _ := newMachine(t, newFakeClock())
	out := make(map[string][]byte)
	encode := func(name string, c packet.Control) {
		buf := make([]byte, packet.MandatoryLen)
		out[name] = buf[:c.WriteTo(buf, 0)]
	}
	encode("Down", m.Build())
	if err := m.Receive(recv(packet.StateDown, 0)); err != nil {
		t.Fatalf("peer Down: %v", err)
	}
	encode("Init", m.Build())
	if err := m.Receive(recv(packet.StateUp, peerLearnedDiscr)); err != nil {
		t.Fatalf("peer Up: %v", err)
	}
	if m.State() != packet.StateUp {
		t.Fatalf("machine in %v after peer Up, want Up", m.State())
	}
	encode("Up", m.Build())
	encode("Final", m.BuildFinal())
	m.AdminDown(packet.DiagAdminDown)
	encode("AdminDown", m.Build())
	return out
}

// RFC requirement: RFC5880-4.1-1 positive -- every Control packet the session
// transmits (Build in Down, Init, Up and AdminDown, and BuildFinal) leaves with
// the Version field, the top three bits of the first octet, set to 1.
//
// VALIDATES: the RFC 5880 Section 6.8.7 transmit rule on the session's own
// packets rather than a hand-built fixture.
func TestRFC5880TransmittedControlCarriesVersionOne(t *testing.T) {
	for name, wire := range transmittedHeaders(t) {
		if got := wire[0] >> 5; got != 1 {
			t.Errorf("%s packet: Version %d, want 1 (octet 0 = %#02x)", name, got, wire[0])
		}
	}
}

// RFC requirement: RFC5880-4.1-1 negative -- a transmitted packet whose
// Diagnostic is pushed past its five bits (0xFF) still leaves with Version 1:
// the encoder never lets another field write into the version bits, and the
// packet parses as a version 1 packet.
//
// VALIDATES: the version bits of a session packet are the version constant
// alone, whatever the other fields of the first octet hold.
func TestRFC5880TransmittedVersionNeverOverwrittenByDiag(t *testing.T) {
	m, _ := newMachine(t, newFakeClock())
	c := m.Build()
	c.Diag = packet.Diag(0xFF)
	buf := make([]byte, packet.MandatoryLen)
	wire := buf[:c.WriteTo(buf, 0)]
	if got := wire[0] >> 5; got != 1 {
		t.Fatalf("Version %d after a Diag of 0xFF, want 1 (octet 0 = %#02x)", got, wire[0])
	}
	if _, _, err := packet.ParseControl(wire); err != nil {
		t.Fatalf("the packet does not parse as version 1: %v", err)
	}
}

// RFC requirement: RFC5880-4.1-2 positive -- the Multipoint bit, the lowest
// bit of the second octet, is zero on every Control packet the session
// transmits (Build in Down, Init, Up and AdminDown, and BuildFinal).
//
// VALIDATES: the transmit half of "zero on both transmit and receipt" on the
// session's own packets; the receipt half is TestRFC5880MultipointSetDiscarded.
func TestRFC5880TransmittedControlMultipointZero(t *testing.T) {
	for name, wire := range transmittedHeaders(t) {
		if wire[1]&packet.FlagMultipoint != 0 {
			t.Errorf("%s packet: Multipoint bit set (octet 1 = %#02x)", name, wire[1])
		}
	}
}
