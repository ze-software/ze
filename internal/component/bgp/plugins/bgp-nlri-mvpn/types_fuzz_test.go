package bgp_nlri_mvpn

import (
	"testing"
)

// FuzzParseMVPN tests MVPN NLRI binary parsing robustness.
//
// VALIDATES: ParseMVPN handles arbitrary bytes without crashing.
// PREVENTS: Panic on truncated data, unknown route types, malformed RD.
// SECURITY: MVPN bytes come from untrusted BGP UPDATE NLRI.
func FuzzParseMVPN(f *testing.F) {
	// Intra-AS I-PMSI A-D (type 1) with RD type 0 (65001:100) + originator
	f.Add([]byte{
		0x01,                                           // route type: Intra-AS I-PMSI A-D
		0x0C,                                           // length = 12
		0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x64, // RD type 0: 65001:100
		0x0A, 0x00, 0x00, 0x01, // originator 10.0.0.1
	})
	// Source Tree Join (type 7) with RD type 0
	f.Add([]byte{
		0x07, // route type: Source Tree Join
		0x0C,
		0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x64,
		0x0A, 0x00, 0x00, 0x01,
	})
	// S-PMSI A-D (type 3) with RD type 1
	f.Add([]byte{
		0x03,
		0x0C,
		0x00, 0x01, 0x0A, 0x00, 0x00, 0x01, 0x00, 0xC8,
		0x00, 0x00, 0x00, 0x00,
	})
	f.Add([]byte{})                       // Empty
	f.Add([]byte{0x01})                   // Truncated header
	f.Add([]byte{0x01, 0x10})             // Truncated body
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF}) // Unknown type, max values
	f.Add([]byte{0x00, 0x00})             // Invalid type 0

	f.Fuzz(func(t *testing.T, data []byte) {
		mvpn, _, err := ParseMVPN(AFIIPv4, data)
		if err != nil {
			return
		}
		// Exercise methods — MUST NOT panic
		_ = mvpn.String()
		_ = mvpn.Bytes()
		_ = mvpn.Len()
		buf := make([]byte, mvpn.Len()+10)
		_ = mvpn.WriteTo(buf, 0)
		_ = mvpn.RouteType()
		_ = mvpn.RD()
	})
}
