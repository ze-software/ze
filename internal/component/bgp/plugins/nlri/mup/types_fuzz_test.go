package mup

import (
	"testing"
)

// FuzzParseMUP tests MUP NLRI binary parsing robustness.
//
// VALIDATES: ParseMUP handles arbitrary bytes without crashing.
// PREVENTS: Panic on truncated data, unknown route/arch types, malformed RD.
// SECURITY: MUP bytes come from untrusted BGP UPDATE NLRI.
func FuzzParseMUP(f *testing.F) {
	// ISD (type 1) with arch 3GPP-5G, RD type 0 (65001:100), data
	f.Add([]byte{
		0x01,       // route type: ISD
		0x00, 0x01, // arch type: 3GPP-5G
		0x10,                                           // length = 16
		0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x64, // RD type 0: 65001:100
		0x0A, 0x00, 0x00, 0x01, // data: 10.0.0.1
	})
	// T1ST (type 3) with RD
	f.Add([]byte{
		0x03,
		0x00, 0x01,
		0x10,
		0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x64,
		0x0A, 0x00, 0x00, 0x01,
	})
	// DSD (type 2)
	f.Add([]byte{
		0x02,
		0x00, 0x01,
		0x08,
		0x00, 0x01, 0x0A, 0x00, 0x00, 0x01, 0x00, 0xC8,
	})
	f.Add([]byte{})                       // Empty
	f.Add([]byte{0x01})                   // Truncated header
	f.Add([]byte{0x01, 0x00, 0x01, 0x10}) // Truncated body
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF}) // Max values
	f.Add([]byte{0x00, 0x00, 0x00, 0x00}) // Invalid type 0

	f.Fuzz(func(t *testing.T, data []byte) {
		mup, _, err := ParseMUP(AFIIPv4, data)
		if err != nil {
			return
		}
		// Exercise methods — MUST NOT panic
		_ = mup.String()
		_ = mup.Bytes()
		_ = mup.Len()
		buf := make([]byte, mup.Len()+10)
		_ = mup.WriteTo(buf, 0)
		_ = mup.RouteType()
		_ = mup.ArchType()
		_ = mup.RD()
	})
}
