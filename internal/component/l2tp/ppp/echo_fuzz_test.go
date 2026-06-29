package ppp

import (
	"testing"
)

// FuzzParseLCPEchoMagic exercises ParseLCPEchoMagic against arbitrary
// payload lengths to catch panics on short or truncated buffers.
// Required by .claude/rules/testing.md "Fuzz (MANDATORY for wire
// format)".
func FuzzParseLCPEchoMagic(f *testing.F) {
	seeds := [][]byte{
		nil,
		{},
		{0x01},
		{0x01, 0x02},
		{0x01, 0x02, 0x03},
		{0x01, 0x02, 0x03, 0x04},
		{0xDE, 0xAD, 0xBE, 0xEF, 'h', 'i'},
		make([]byte, 1500),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := ParseLCPEchoMagic(data)
		if err == nil && len(data) < 4 {
			t.Errorf("len=%d returned no error", len(data))
		}
	})
}
