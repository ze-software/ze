package vpn

import (
	"testing"
)

// FuzzParseVPN tests VPN NLRI binary parsing robustness.
//
// VALIDATES: ParseVPN handles arbitrary bytes without crashing.
// PREVENTS: Panic on truncated data, malformed labels, bad RD, prefix overflow.
// SECURITY: VPN bytes come from untrusted BGP UPDATE NLRI.
func FuzzParseVPN(f *testing.F) {
	// VPNv4: RD type 0 (0:1:1), label 100, prefix 10.0.0.0/24
	// 0x70=112 bits (24 label + 64 RD + 24 prefix), 000641=label 100 BOS,
	// 00000001 00000001=RD type 0: ASN=1 value=1, 0A0000=10.0.0.0/24
	f.Add([]byte{0x70, 0x00, 0x06, 0x41, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x0A, 0x00, 0x00})
	// VPNv4: RD type 1 (1:192.0.2.1:100), label 100, prefix 10.0.1.0/24
	f.Add([]byte{0x70, 0x00, 0x06, 0x41, 0x00, 0x01, 0xC0, 0x00, 0x02, 0x01, 0x00, 0x64, 0x0A, 0x00, 0x01})
	// VPNv4: RD type 2 (2:65537:100), label 200, prefix 10.0.2.0/24
	f.Add([]byte{0x70, 0x00, 0x0C, 0x81, 0x00, 0x02, 0x00, 0x01, 0x00, 0x01, 0x00, 0x64, 0x0A, 0x00, 0x02})
	// VPNv4: /32 prefix (more prefix bytes)
	f.Add([]byte{0x78, 0x00, 0x06, 0x41, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x0A, 0x00, 0x00, 0x01})
	// VPNv4: /0 prefix (no prefix bytes)
	f.Add([]byte{0x58, 0x00, 0x06, 0x41, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01})
	f.Add([]byte{})                                               // Empty
	f.Add([]byte{0x70})                                           // Prefix-len only
	f.Add([]byte{0x70, 0x00})                                     // Truncated label
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}) // Max values

	f.Fuzz(func(t *testing.T, data []byte) {
		vpn, _, err := ParseVPN(AFIIPv4, SAFIVPN, data, false)
		if err != nil {
			return
		}
		// Exercise methods — MUST NOT panic
		_ = vpn.String()
		_ = vpn.Bytes()
		_ = vpn.Len()
		buf := make([]byte, vpn.Len()+10)
		_ = vpn.WriteTo(buf, 0)
		_ = vpn.PathID()
		_ = vpn.HasPathID()
	})
}

// FuzzParseVPNAddPath tests VPN NLRI parsing with ADD-PATH enabled.
//
// VALIDATES: ParseVPN with addpath=true handles arbitrary bytes without crashing.
// PREVENTS: Panic on truncated path-ID, interaction between path-ID and label/RD parsing.
// SECURITY: ADD-PATH bytes come from untrusted BGP UPDATE NLRI.
func FuzzParseVPNAddPath(f *testing.F) {
	// ADD-PATH: 4-byte path-ID + VPNv4 NLRI
	f.Add([]byte{
		0x00, 0x00, 0x00, 0x2A, // path-ID = 42
		0x70, 0x00, 0x06, 0x41,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x0A, 0x00, 0x00,
	})
	f.Add([]byte{})                                   // Empty
	f.Add([]byte{0x00, 0x00})                         // Truncated path-ID
	f.Add([]byte{0x00, 0x00, 0x00, 0x01})             // Path-ID only, no NLRI
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}) // Max values

	f.Fuzz(func(t *testing.T, data []byte) {
		vpn, _, err := ParseVPN(AFIIPv4, SAFIVPN, data, true)
		if err != nil {
			return
		}
		_ = vpn.String()
		_ = vpn.Bytes()
		_ = vpn.Len()
		_ = vpn.PathID()
	})
}
