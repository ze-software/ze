// Design: plan/spec-isis-2-wire.md -- IIH round-trip tests
package packet

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// areaTLV builds an opaque TLV 1 carrying one area (for PDU body tests that
// just need a representative TLV in the stream).
func areaTLV(t *testing.T, area []byte) TLV {
	t.Helper()
	a := AreaAddressesTLV{Areas: []types.AreaID{mustArea(t, area)}}
	buf := make([]byte, 64)
	n := writeAreaAddressesTLV(buf, 0, a)
	it := NewTLVIterator(buf[:n])
	_, value, _ := it.Next()
	return TLV{Type: TLVAreaAddresses, Value: value}.CopyValue()
}

// VALIDATES: AC-2 -- LAN L1 and LAN L2 IIH bodies round-trip every field
// (circuit type, system ID, holding time, priority, LAN ID) and the TLV list.
func TestISISLANIIHRoundTrip(t *testing.T) {
	sys := types.SystemID{0, 1, 0, 2, 0, 3}
	lan := types.NewSourceID(types.SystemID{0xaa, 0, 0, 0, 0, 1}, 1)
	for _, pt := range []PDUType{PDUTypeL1LANHello, PDUTypeL2LANHello} {
		t.Run(pt.String(), func(t *testing.T) {
			in := &LANHello{
				PDUType:     pt,
				CircuitType: CircuitL1L2,
				SystemID:    sys,
				HoldingTime: 30,
				Priority:    64,
				LANID:       lan,
				TLVs: []TLV{
					areaTLV(t, []byte{0x49, 0x00, 0x01}),
				},
			}
			buf := make([]byte, in.EncodedLen())
			n := in.WriteTo(buf, 0)
			if n != in.EncodedLen() {
				t.Fatalf("WriteTo returned %d, want EncodedLen %d", n, in.EncodedLen())
			}
			// Decode via the top-level dispatcher (exercises header + body).
			pdu, err := DecodePDU(buf)
			if err != nil {
				t.Fatalf("DecodePDU: %v", err)
			}
			if pdu.LANHello == nil {
				t.Fatalf("expected LANHello, header type %v", pdu.Header.PDUType)
			}
			out := pdu.LANHello
			if out.PDUType != pt || out.CircuitType != CircuitL1L2 || out.SystemID != sys ||
				out.HoldingTime != 30 || out.Priority != 64 || out.LANID != lan {
				t.Errorf("field mismatch: %+v", out)
			}
			if len(out.TLVs) != 1 || out.TLVs[0].Type != TLVAreaAddresses {
				t.Errorf("TLVs = %+v", out.TLVs)
			}
		})
	}
}

// VALIDATES: priority high bit is masked to the 7-bit DIS-priority range.
func TestISISLANIIHPriorityMask(t *testing.T) {
	in := &LANHello{PDUType: PDUTypeL1LANHello, Priority: 0xFF} // > 127
	buf := make([]byte, in.EncodedLen())
	in.WriteTo(buf, 0)
	pdu, err := DecodePDU(buf)
	if err != nil {
		t.Fatalf("DecodePDU: %v", err)
	}
	if pdu.LANHello.Priority != MaxDISPriority {
		t.Errorf("priority = %d, want %d (masked)", pdu.LANHello.Priority, MaxDISPriority)
	}
}

// VALIDATES: AC-2 -- P2P IIH body round-trips (circuit type, system ID, holding
// time, local circuit ID) plus a TLV 240 carried in the stream.
func TestISISP2PIIHRoundTrip(t *testing.T) {
	sys := types.SystemID{9, 8, 7, 6, 5, 4}
	// Build a TLV 240 (full form) as an opaque TLV in the stream.
	tw := P2PThreeWayTLV{State: AdjThreeWayUp, HasCircuitID: true, LocalCircuitID: 7,
		HasNeighbor: true, NeighborID: types.SystemID{1, 1, 1, 1, 1, 1}, NeighborCircuit: 9}
	twBuf := make([]byte, 32)
	twN := writeP2PThreeWayTLV(twBuf, 0, tw)
	it := NewTLVIterator(twBuf[:twN])
	_, twVal, _ := it.Next()

	in := &P2PHello{
		CircuitType:    CircuitL2,
		SystemID:       sys,
		HoldingTime:    27,
		LocalCircuitID: 5,
		TLVs:           []TLV{{Type: TLVP2PThreeWay, Value: twVal}},
	}
	buf := make([]byte, in.EncodedLen())
	n := in.WriteTo(buf, 0)
	if n != in.EncodedLen() {
		t.Fatalf("WriteTo returned %d, want %d", n, in.EncodedLen())
	}
	pdu, err := DecodePDU(buf)
	if err != nil {
		t.Fatalf("DecodePDU: %v", err)
	}
	if pdu.P2PHello == nil {
		t.Fatalf("expected P2PHello, header type %v", pdu.Header.PDUType)
	}
	out := pdu.P2PHello
	if out.CircuitType != CircuitL2 || out.SystemID != sys || out.HoldingTime != 27 || out.LocalCircuitID != 5 {
		t.Errorf("field mismatch: %+v", out)
	}
	if len(out.TLVs) != 1 || out.TLVs[0].Type != TLVP2PThreeWay {
		t.Fatalf("TLVs = %+v", out.TLVs)
	}
	// The carried TLV 240 must itself decode to the original three-way state.
	got, err := DecodeP2PThreeWayTLV(out.TLVs[0].Value)
	if err != nil {
		t.Fatalf("DecodeP2PThreeWayTLV: %v", err)
	}
	if got != tw {
		t.Errorf("TLV 240 = %+v, want %+v", got, tw)
	}
}

// VALIDATES: encoding at a non-zero buffer offset is correct (buffer-first
// WriteTo must honor off, not assume 0).
func TestISISP2PIIHNonZeroOffset(t *testing.T) {
	in := &P2PHello{CircuitType: CircuitL1, SystemID: types.SystemID{1, 2, 3, 4, 5, 6}, HoldingTime: 9, LocalCircuitID: 1}
	const pad = 5
	buf := make([]byte, pad+in.EncodedLen())
	end := in.WriteTo(buf, pad)
	if end != pad+in.EncodedLen() {
		t.Fatalf("WriteTo end = %d, want %d", end, pad+in.EncodedLen())
	}
	pdu, err := DecodePDU(buf[pad:end])
	if err != nil {
		t.Fatalf("DecodePDU at offset: %v", err)
	}
	if pdu.P2PHello == nil || pdu.P2PHello.SystemID != in.SystemID {
		t.Errorf("offset round-trip mismatch")
	}
}
