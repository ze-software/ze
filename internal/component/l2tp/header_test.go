package l2tp

import (
	"bytes"
	"errors"
	"testing"
)

// TestParseControlHeader validates AC-1:
// VALIDATES: A well-formed 12-byte control header parses with exact field values.
// PREVENTS: bit-layout regressions on the canonical SCCRQ/SCCRP/SCCCN shape.
func TestParseControlHeader(t *testing.T) {
	// C8 02 00 14 00 0A 00 03 00 05 00 04
	// flags=0xC802, length=20, tid=10, sid=3, ns=5, nr=4
	b := []byte{
		0xC8, 0x02,
		0x00, 0x14,
		0x00, 0x0A,
		0x00, 0x03,
		0x00, 0x05,
		0x00, 0x04,
		// 8 bytes of "payload" to satisfy Length=20
		0, 0, 0, 0, 0, 0, 0, 0,
	}
	h, err := ParseMessageHeader(b)
	if err != nil {
		t.Fatalf("ParseMessageHeader: %v", err)
	}
	if !h.IsControl || !h.HasLength || !h.HasSequence || h.HasOffset || h.Priority {
		t.Fatalf("flag bits wrong: %+v", h)
	}
	if h.Version != 2 {
		t.Fatalf("Version: got %d want 2", h.Version)
	}
	if h.Length != 20 {
		t.Fatalf("Length: got %d want 20", h.Length)
	}
	if h.TunnelID != 10 || h.SessionID != 3 {
		t.Fatalf("TID/SID: got %d/%d want 10/3", h.TunnelID, h.SessionID)
	}
	if h.Ns != 5 || h.Nr != 4 {
		t.Fatalf("Ns/Nr: got %d/%d want 5/4", h.Ns, h.Nr)
	}
	if h.PayloadOff != 12 {
		t.Fatalf("PayloadOff: got %d want 12", h.PayloadOff)
	}
}

// TestParseDataHeaderMinimal validates AC-2:
// VALIDATES: A T=0,L=0,S=0,O=0 data header has PayloadOff=6 and no optional fields set.
// PREVENTS: incorrectly requiring Length/Ns for data messages.
func TestParseDataHeaderMinimal(t *testing.T) {
	// 0x0002 (T=0,L=0,S=0,O=0,P=0,Ver=2), TID, SID
	b := []byte{
		0x00, 0x02,
		0x00, 0x11,
		0x00, 0x22,
	}
	h, err := ParseMessageHeader(b)
	if err != nil {
		t.Fatalf("ParseMessageHeader: %v", err)
	}
	if h.IsControl || h.HasLength || h.HasSequence || h.HasOffset {
		t.Fatalf("unexpected flags: %+v", h)
	}
	if h.TunnelID != 0x11 || h.SessionID != 0x22 || h.PayloadOff != 6 {
		t.Fatalf("fields: %+v", h)
	}
}

// TestParseDataHeaderAllOptional validates AC-3:
// VALIDATES: L=1, S=1, O=1 data header parses all optional fields; PayloadOff includes the offset pad.
// PREVENTS: forgetting to advance PayloadOff past OffsetSize pad bytes.
func TestParseDataHeaderAllOptional(t *testing.T) {
	// flags: T=0, L=1, S=1, O=1, Ver=2 => 0x4A02 (L=0x4000 | S=0x0800 | O=0x0200 | Ver=2)
	b := []byte{
		0x4A, 0x02,
		0x00, 0x10, // length 16
		0x00, 0x11, // TID
		0x00, 0x22, // SID
		0x00, 0x33, // Ns
		0x00, 0x44, // Nr
		0x00, 0x02, // OffsetSize = 2
		0x00, 0x00, // OffsetPad
	}
	h, err := ParseMessageHeader(b)
	if err != nil {
		t.Fatalf("ParseMessageHeader: %v", err)
	}
	if !h.HasLength || !h.HasSequence || !h.HasOffset {
		t.Fatalf("flags: %+v", h)
	}
	if h.Length != 16 || h.Ns != 0x33 || h.Nr != 0x44 || h.OffsetSize != 2 {
		t.Fatalf("fields: %+v", h)
	}
	if h.PayloadOff != 16 { // 2 flags + 2 length + 2 tid + 2 sid + 2 ns + 2 nr + 2 offsize + 2 pad
		t.Fatalf("PayloadOff: got %d want 16", h.PayloadOff)
	}
}

// TestParseHeaderUnsupportedVersion validates AC-4.
func TestParseHeaderUnsupportedVersion(t *testing.T) {
	// Ver=3 (L2TPv3)
	b := []byte{0xC8, 0x03, 0x00, 0x0C, 0, 1, 0, 0, 0, 0, 0, 0}
	_, err := ParseMessageHeader(b)
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("expected ErrUnsupportedVersion, got %v", err)
	}
}

// TestParseHeaderVersionBeforeLength validates that ParseMessageHeader
// inspects the Ver field BEFORE enforcing the 6-byte minimum. A short
// L2TPv3 or L2F frame (Ver != 2) must return ErrUnsupportedVersion so
// phase 3 can distinguish L2TPv3 (StopCCN Result Code 5) from L2F
// (silently discard) even when the frame is truncated.
// VALIDATES: short-input Ver detection.
// PREVENTS: regression that would report ErrShortBuffer for a 2-byte
// L2TPv3 frame, hiding the version signal.
func TestParseHeaderVersionBeforeLength(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
	}{
		{"L2TPv3 two bytes only", []byte{0xC8, 0x03}},
		{"L2F control two bytes only", []byte{0xC8, 0x01}},
		{"L2TPv3 five bytes", []byte{0xC8, 0x03, 0x00, 0x0C, 0x00}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseMessageHeader(tc.in)
			if !errors.Is(err, ErrUnsupportedVersion) {
				t.Fatalf("got %v, want ErrUnsupportedVersion", err)
			}
		})
	}
}

// TestParseHeaderEmptyPayload validates the boundary where Length exactly
// equals PayloadOff (control message with zero AVPs after the header).
// VALIDATES: the Length >= PayloadOff guard at end-of-parse accepts the
// minimum valid value.
// PREVENTS: an off-by-one regression that would reject a syntactically
// legal empty-AVP control message.
func TestParseHeaderEmptyPayload(t *testing.T) {
	// Ver=2, control, Length=12, TID=1, SID=0, Ns=0, Nr=0.
	b := []byte{0xC8, 0x02, 0x00, 0x0C, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	h, err := ParseMessageHeader(b)
	if err != nil {
		t.Fatalf("ParseMessageHeader: %v", err)
	}
	if h.Length != 12 || h.PayloadOff != 12 || h.TunnelID != 1 {
		t.Fatalf("unexpected %+v", h)
	}
	// Payload slice is empty.
	if r := int(h.Length) - h.PayloadOff; r != 0 {
		t.Fatalf("payload length: got %d want 0", r)
	}
}

// TestParseHeaderInvariants directly asserts the invariants that
// FuzzParseMessageHeader checks, so the class of bug (Length <
// PayloadOff) stays covered even if the fuzz invariant is later changed.
// VALIDATES: for every parse that succeeds, PayloadOff is in-range and
// Length (if present) is >= PayloadOff and <= len(b).
// PREVENTS: regressing the final Length-vs-PayloadOff check that the
// fuzz surfaced during initial development.
func TestParseHeaderInvariants(t *testing.T) {
	inputs := [][]byte{
		{0xC8, 0x02, 0x00, 0x0C, 0, 1, 0, 0, 0, 0, 0, 0},
		{0x00, 0x02, 0x00, 0x11, 0x00, 0x22},
		{0x4A, 0x02, 0x00, 0x10, 0, 0x11, 0, 0x22, 0, 0x33, 0, 0x44, 0, 2, 0, 0},
	}
	for i, in := range inputs {
		h, err := ParseMessageHeader(in)
		if err != nil {
			t.Fatalf("input %d: %v", i, err)
		}
		if h.PayloadOff < 0 || h.PayloadOff > len(in) {
			t.Fatalf("input %d: PayloadOff %d out of [0,%d]", i, h.PayloadOff, len(in))
		}
		if h.HasLength && (int(h.Length) < h.PayloadOff || int(h.Length) > len(in)) {
			t.Fatalf("input %d: Length %d out of [%d,%d]", i, h.Length, h.PayloadOff, len(in))
		}
	}
}

// TestParseHeaderMalformedControl validates AC-5.
func TestParseHeaderMalformedControl(t *testing.T) {
	// T=1, L=0, Ver=2 -- control without length
	b := []byte{0x80, 0x02, 0x00, 0x0A, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00}
	_, err := ParseMessageHeader(b)
	if !errors.Is(err, ErrMalformedControl) {
		t.Fatalf("expected ErrMalformedControl, got %v", err)
	}
}

// TestParseHeaderShortBuffer validates AC-6.
func TestParseHeaderShortBuffer(t *testing.T) {
	t.Run("too short for flags", func(t *testing.T) {
		_, err := ParseMessageHeader([]byte{0xC8})
		if !errors.Is(err, ErrShortBuffer) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("length exceeds buffer", func(t *testing.T) {
		// claims length=100 but only 12 bytes provided
		b := []byte{0xC8, 0x02, 0x00, 0x64, 0, 0, 0, 0, 0, 0, 0, 0}
		_, err := ParseMessageHeader(b)
		if !errors.Is(err, ErrShortBuffer) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("truncated before TID", func(t *testing.T) {
		b := []byte{0xC8, 0x02, 0x00, 0x0C}
		_, err := ParseMessageHeader(b)
		if !errors.Is(err, ErrShortBuffer) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("truncated offset pad", func(t *testing.T) {
		// L=1, S=1, O=1, OffsetSize=4 but only 2 pad bytes provided
		b := []byte{0x4A, 0x02, 0x00, 0x14, 0, 1, 0, 2, 0, 0, 0, 0, 0x00, 0x04, 0, 0}
		_, err := ParseMessageHeader(b)
		if !errors.Is(err, ErrShortBuffer) {
			t.Fatalf("got %v", err)
		}
	})
}

// TestWriteControlHeader validates that the serialized header round-trips
// byte-for-byte through ParseMessageHeader.
// VALIDATES: exact byte layout; Length backfill by re-calling WriteControlHeader.
// PREVENTS: endian/offset regressions in the header writer.
func TestWriteControlHeader(t *testing.T) {
	buf := make([]byte, 12)
	n := WriteControlHeader(buf, 0, 12, 0xABCD, 0x1234, 7, 8)
	if n != 12 {
		t.Fatalf("n: got %d want 12", n)
	}
	want := []byte{0xC8, 0x02, 0x00, 0x0C, 0xAB, 0xCD, 0x12, 0x34, 0x00, 0x07, 0x00, 0x08}
	if !bytes.Equal(buf, want) {
		t.Fatalf("bytes:\n got  %x\n want %x", buf, want)
	}
	h, err := ParseMessageHeader(buf)
	if err != nil {
		t.Fatalf("ParseMessageHeader: %v", err)
	}
	if h.TunnelID != 0xABCD || h.SessionID != 0x1234 || h.Ns != 7 || h.Nr != 8 || h.Length != 12 {
		t.Fatalf("%+v", h)
	}
}

// TestWriteDataHeaderRoundTrip validates data-message header round-trip.
// VALIDATES: WriteDataHeader + ParseMessageHeader preserve all fields and
// flag combinations.
func TestWriteDataHeaderRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   MessageHeader
		size int // expected bytes written
	}{
		{"minimal", MessageHeader{TunnelID: 5, SessionID: 9}, 6},
		// Length field, when present on the wire, is the total message length.
		// The test uses `size` bytes for the whole message, so Length must equal size.
		{"with length", MessageHeader{HasLength: true, Length: 8, TunnelID: 5, SessionID: 9}, 8},
		{
			"all flags", MessageHeader{
				HasLength: true, HasSequence: true, HasOffset: true, Priority: true,
				Length: 18, TunnelID: 5, SessionID: 9, Ns: 3, Nr: 4, OffsetSize: 4,
			},
			18, // 2+2+2+2+2+2+2+4
		},
		// HasOffset=true with OffsetSize=0 is legal: the O bit signals that
		// an OffsetSize field is present, and its value happens to be zero.
		// PayloadOff advances past the 2-byte OffsetSize field but reserves
		// no pad bytes.
		{
			"offset size zero", MessageHeader{
				HasOffset: true,
				TunnelID:  5, SessionID: 9, OffsetSize: 0,
			},
			8, // 2 flags + 2 TID + 2 SID + 2 OffsetSize
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, 40)
			n := WriteDataHeader(buf, 0, tc.in)
			if n != tc.size {
				t.Fatalf("WriteDataHeader n=%d want %d", n, tc.size)
			}
			out, err := ParseMessageHeader(buf[:n])
			if err != nil {
				t.Fatalf("ParseMessageHeader: %v", err)
			}
			// Do not compare PayloadOff directly (WriteDataHeader wrote n bytes).
			if out.TunnelID != tc.in.TunnelID || out.SessionID != tc.in.SessionID {
				t.Fatalf("TID/SID: %+v vs %+v", out, tc.in)
			}
			if out.HasLength && out.Length != tc.in.Length {
				t.Fatalf("Length mismatch: %d vs %d", out.Length, tc.in.Length)
			}
			if out.HasSequence && (out.Ns != tc.in.Ns || out.Nr != tc.in.Nr) {
				t.Fatalf("Ns/Nr: %d/%d vs %d/%d", out.Ns, out.Nr, tc.in.Ns, tc.in.Nr)
			}
			if out.HasOffset && out.OffsetSize != tc.in.OffsetSize {
				t.Fatalf("OffsetSize: %d vs %d", out.OffsetSize, tc.in.OffsetSize)
			}
			if out.Priority != tc.in.Priority {
				t.Fatalf("Priority: %v vs %v", out.Priority, tc.in.Priority)
			}
		})
	}
}
