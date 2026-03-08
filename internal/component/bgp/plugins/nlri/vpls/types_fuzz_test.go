package vpls

import (
	"testing"
)

// FuzzParseVPLS tests VPLS NLRI binary parsing robustness.
//
// VALIDATES: ParseVPLS handles arbitrary bytes without crashing.
// PREVENTS: Panic on truncated data, malformed length, label encoding.
// SECURITY: VPLS bytes come from untrusted BGP UPDATE NLRI.
func FuzzParseVPLS(f *testing.F) {
	// Valid VPLS: length=17, RD type 0 (65001:100), VE-ID=5, offset=100, size=200, label=16000
	f.Add([]byte{
		0x00, 0x11, // length = 17
		0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x64, // RD type 0: 65001:100
		0x00, 0x05, // VE-ID = 5
		0x00, 0x64, // VE block offset = 100
		0x00, 0xC8, // VE block size = 200
		0x03, 0xE8, 0x01, // label base 16000 (3 bytes)
	})
	// Minimal VPLS with RD type 1
	f.Add([]byte{
		0x00, 0x11,
		0x00, 0x01, 0x0A, 0x00, 0x00, 0x01, 0x00, 0xC8, // RD type 1: 10.0.0.1:200
		0x00, 0x0A, // VE-ID = 10
		0x00, 0x00, // offset = 0
		0x00, 0x00, // size = 0
		0x00, 0x01, 0xF5, // label 500
	})
	f.Add([]byte{})                       // Empty
	f.Add([]byte{0x00})                   // Truncated length
	f.Add([]byte{0x00, 0x02})             // Short length
	f.Add([]byte{0x00, 0x11, 0, 0, 0, 0}) // Too short for declared length
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF}) // Max values
	f.Add([]byte{0x00, 0x00})             // Zero length

	f.Fuzz(func(t *testing.T, data []byte) {
		vpls, _, err := ParseVPLS(data)
		if err != nil {
			return
		}
		// Exercise methods — MUST NOT panic
		_ = vpls.String()
		_ = vpls.Bytes()
		_ = vpls.Len()
		buf := make([]byte, vpls.Len()+10)
		_ = vpls.WriteTo(buf, 0)
		_ = vpls.RD()
		_ = vpls.VEID()
		_ = vpls.VEBlockOffset()
		_ = vpls.VEBlockSize()
		_ = vpls.LabelBase()
	})
}
