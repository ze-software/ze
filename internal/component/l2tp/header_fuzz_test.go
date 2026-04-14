package l2tp

import "testing"

// FuzzParseMessageHeader ensures ParseMessageHeader never panics, reads out of
// bounds, or loops on arbitrary input. Bounds-checking is BLOCKING: any
// out-of-slice access would be detected by the race/bound checks.
func FuzzParseMessageHeader(f *testing.F) {
	seed := [][]byte{
		{0xC8, 0x02, 0x00, 0x0C, 0, 1, 0, 0, 0, 0, 0, 0},
		{0x00, 0x02, 0x00, 0x11, 0x00, 0x22},
		{0x4A, 0x02, 0x00, 0x10, 0, 0x11, 0, 0x22, 0, 0x33, 0, 0x44, 0, 2, 0, 0},
		{0xC8, 0x03, 0x00, 0x0C, 0, 0, 0, 0, 0, 0, 0, 0},
		{0x80, 0x02, 0x00, 0x0A, 0, 1, 0, 0, 0, 0},
		{},
		{0xFF},
	}
	for _, s := range seed {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		h, err := ParseMessageHeader(in)
		if err != nil {
			return
		}
		// When parse succeeds, PayloadOff must be within the input bounds.
		if h.PayloadOff < 0 || h.PayloadOff > len(in) {
			t.Fatalf("PayloadOff out of range: %d (len=%d)", h.PayloadOff, len(in))
		}
		// If HasLength, Length must be >= PayloadOff and <= len(in).
		if h.HasLength && (int(h.Length) < h.PayloadOff || int(h.Length) > len(in)) {
			t.Fatalf("Length inconsistent: %d, PayloadOff=%d, len=%d", h.Length, h.PayloadOff, len(in))
		}
	})
}
