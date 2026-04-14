package l2tp

import (
	"bytes"
	"testing"
)

// TestMessageRoundTrip assembles a realistic SCCRQ message using all of the
// writers, parses the result via the header parser and AVP iterator, and
// verifies that every field survived the round-trip.
//
// VALIDATES: the public encoding and decoding APIs are consistent.
// PREVENTS: silent regressions that would let one side drift from the other.
func TestMessageRoundTrip(t *testing.T) {
	buf := make([]byte, 512)
	// Reserve 12 bytes for the control header; write AVPs starting at off=12.
	off := ControlHeaderLen
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgSCCRQ))
	off += WriteAVPBytes(buf, off, true, 0, AVPProtocolVersion, ProtocolVersionValue[:])
	off += WriteAVPUint32(buf, off, true, AVPFramingCapabilities, 0x00000003)
	off += WriteAVPString(buf, off, true, AVPHostName, "lac-01")
	off += WriteAVPUint16(buf, off, true, AVPAssignedTunnelID, 0xBEEF)
	off += WriteAVPUint16(buf, off, true, AVPReceiveWindowSize, 16)
	off += WriteAVPUint64(buf, off, false, AVPTieBreaker, 0xCAFEBABE12345678)
	total := off

	// Now that length is known, write the header.
	WriteControlHeader(buf, 0, uint16(total) /*tid*/, 0 /*sid*/, 0 /*ns*/, 0 /*nr*/, 0)

	wire := buf[:total]

	// Parse back.
	h, err := ParseMessageHeader(wire)
	if err != nil {
		t.Fatalf("ParseMessageHeader: %v", err)
	}
	if !h.IsControl || h.Length != uint16(total) || h.PayloadOff != ControlHeaderLen {
		t.Fatalf("header mismatch: %+v", h)
	}
	it := NewAVPIterator(wire[h.PayloadOff:h.Length])

	type seen struct {
		attr AVPType
		val  []byte
		m    bool
	}
	var got []seen
	for {
		_, at, flags, val, ok := it.Next()
		if !ok {
			break
		}
		vc := make([]byte, len(val))
		copy(vc, val)
		got = append(got, seen{attr: at, val: vc, m: flags&FlagMandatory != 0})
	}
	if err := it.Err(); err != nil {
		t.Fatalf("iter err: %v", err)
	}
	if len(got) != 7 {
		t.Fatalf("AVP count: %d want 7", len(got))
	}
	checks := []struct {
		attr AVPType
		m    bool
	}{
		{AVPMessageType, true},
		{AVPProtocolVersion, true},
		{AVPFramingCapabilities, true},
		{AVPHostName, true},
		{AVPAssignedTunnelID, true},
		{AVPReceiveWindowSize, true},
		{AVPTieBreaker, false},
	}
	for i, c := range checks {
		if got[i].attr != c.attr {
			t.Fatalf("AVP[%d] attr: got %d want %d", i, got[i].attr, c.attr)
		}
		if got[i].m != c.m {
			t.Fatalf("AVP[%d] M: got %v want %v", i, got[i].m, c.m)
		}
	}

	// Spot-check a few parsed values.
	if mt, err := ReadAVPUint16(got[0].val); err != nil || MessageType(mt) != MsgSCCRQ {
		t.Fatalf("message type: %d (%v)", mt, err)
	}
	if !bytes.Equal(got[1].val, ProtocolVersionValue[:]) {
		t.Fatalf("protocol version: %x", got[1].val)
	}
	if fc, err := ReadAVPUint32(got[2].val); err != nil || fc != 0x03 {
		t.Fatalf("framing cap: %x (%v)", fc, err)
	}
	if string(got[3].val) != "lac-01" {
		t.Fatalf("host name: %q", got[3].val)
	}
	if tid, err := ReadAVPUint16(got[4].val); err != nil || tid != 0xBEEF {
		t.Fatalf("assigned TID: %x", tid)
	}
	if rws, err := ReadAVPUint16(got[5].val); err != nil || rws != 16 {
		t.Fatalf("recv window: %d", rws)
	}
	if tb, err := ReadAVPUint64(got[6].val); err != nil || tb != 0xCAFEBABE12345678 {
		t.Fatalf("tie breaker: %x", tb)
	}
}
