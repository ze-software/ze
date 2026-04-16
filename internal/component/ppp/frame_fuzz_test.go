package ppp

import (
	"testing"
)

// FuzzParseFrame verifies that ParseFrame never panics or reads
// out of bounds on arbitrary input. Required by .claude/rules/
// testing.md "Fuzz (MANDATORY for wire format)".
func FuzzParseFrame(f *testing.F) {
	// Seed with known-good frames and adversarial shapes.
	seeds := [][]byte{
		nil,
		{0x00},
		{0xC0, 0x21},                         // LCP, no payload
		{0xC0, 0x21, 0x01, 0x42, 0x00, 0x04}, // LCP with embedded LCP packet
		{0xFF, 0xFF},                         // invalid protocol
		{0x21},                               // one-byte form (now rejected)
		make([]byte, MaxFrameLen),
		make([]byte, MaxFrameLen+1),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		proto, payload, hlen, err := ParseFrame(data)
		if err != nil {
			return
		}
		// Successful parse invariants.
		if hlen != 2 {
			t.Errorf("hlen = %d, want 2", hlen)
		}
		if len(payload) != len(data)-hlen {
			t.Errorf("payload len = %d, want %d", len(payload), len(data)-hlen)
		}
		_ = proto
	})
}
