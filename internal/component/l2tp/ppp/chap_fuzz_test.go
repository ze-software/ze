package ppp

import (
	"testing"
)

// FuzzParseCHAPResponse exercises ParseCHAPResponse against arbitrary
// input to catch panics and out-of-bounds reads. Required by
// .claude/rules/tdd.md "Fuzz (MANDATORY for wire format)".
func FuzzParseCHAPResponse(f *testing.F) {
	seeds := [][]byte{
		nil,
		// valid minimal response: 1-byte Value, empty Name.
		{0x02, 0x00, 0x00, 0x06, 0x01, 0xFF},
		// valid MD5-sized response with name "bob".
		append(append([]byte{
			0x02, 0x42, 0x00, 0x18,
			0x10, // Value-Size = 16
		},
			0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
			0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
		), 'b', 'o', 'b'),
		// length below Value-Size octet (header but no Value-Size).
		{0x02, 0x00, 0x00, 0x04},
		// length far beyond buffer.
		{0x02, 0x00, 0xFF, 0xFF, 0x01},
		// wrong code (Challenge).
		{0x01, 0x00, 0x00, 0x06, 0x01, 0xFF},
		// Value-Size zero (invalid per RFC 1994 Section 4.1).
		{0x02, 0x00, 0x00, 0x05, 0x00},
		// Value-Size overflows Length.
		{0x02, 0x00, 0x00, 0x08, 0xFF, 'a', 'b', 'c'},
		// max-frame-sized buffer of zeros (MaxFrameLen - 2).
		make([]byte, MaxFrameLen-2),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		resp, err := ParseCHAPResponse(data)
		if err != nil {
			return
		}
		// Invariant 1: declared Length is reachable and covers the
		// body the parser consumed.
		if len(data) < chapHeaderLen {
			t.Errorf("parse succeeded on buf too short for header: len=%d", len(data))
			return
		}
		length := int(data[2])<<8 | int(data[3])
		if length < chapHeaderLen+1 {
			t.Errorf("parse succeeded with Length %d below minimum %d",
				length, chapHeaderLen+1)
			return
		}
		bodyLen := 1 + len(resp.Value) + len(resp.Name)
		if chapHeaderLen+bodyLen != length {
			t.Errorf("parsed fields (%d body bytes) do not cover Length %d (diff %d)",
				bodyLen, length, length-chapHeaderLen-bodyLen)
			return
		}
		// Invariant 2: parsed Value bytes match the input Value region
		// byte-for-byte. Defends against off-by-one bugs that produce
		// self-consistent sizes but read the wrong region.
		valueOff := chapHeaderLen + 1
		valueEnd := valueOff + len(resp.Value)
		if valueEnd > length {
			t.Errorf("Value end %d exceeds Length %d", valueEnd, length)
			return
		}
		for i, b := range resp.Value {
			if data[valueOff+i] != b {
				t.Errorf("Value[%d] = %02x, input[%d] = %02x", i, b,
					valueOff+i, data[valueOff+i])
				return
			}
		}
		// Invariant 3: parsed Name equals the bytes at the Name offset.
		nameOff := valueEnd
		nameEnd := nameOff + len(resp.Name)
		if nameEnd > length {
			t.Errorf("Name end %d exceeds Length %d", nameEnd, length)
			return
		}
		if string(data[nameOff:nameEnd]) != resp.Name {
			t.Errorf("Name %q does not match input bytes %q",
				resp.Name, string(data[nameOff:nameEnd]))
		}
	})
}
