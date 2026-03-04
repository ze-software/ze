package wireu

import (
	"testing"
)

// FuzzParseIPv4Prefixes tests IPv4 prefix wire parsing robustness.
//
// VALIDATES: ParseIPv4Prefixes handles arbitrary bytes without crashing.
// PREVENTS: Buffer overrun on invalid prefix lengths, out-of-bounds on truncated data.
// SECURITY: Prefix bytes come directly from untrusted UPDATE NLRI sections.
func FuzzParseIPv4Prefixes(f *testing.F) {
	f.Add([]byte{24, 10, 0, 0})                 // 10.0.0.0/24
	f.Add([]byte{32, 10, 0, 0, 1})              // 10.0.0.1/32
	f.Add([]byte{0})                            // 0.0.0.0/0 (default route)
	f.Add([]byte{16, 172, 16, 8, 192})          // Two prefixes: 172.16.0.0/16, 192.0.0.0/8
	f.Add([]byte{})                             // Empty
	f.Add([]byte{33})                           // Invalid: prefix length > 32
	f.Add([]byte{24})                           // Truncated: claims 3 bytes, none follow
	f.Add([]byte{24, 10})                       // Truncated: claims 3 bytes, only 1
	f.Add([]byte{0xFF, 0xDE, 0xAD, 0xBE, 0xEF}) // Length 255 with some data

	f.Fuzz(func(t *testing.T, data []byte) {
		_ = ParseIPv4Prefixes(data) // MUST NOT panic
	})
}

// FuzzParseIPv6Prefixes tests IPv6 prefix wire parsing robustness.
//
// VALIDATES: ParseIPv6Prefixes handles arbitrary bytes without crashing.
// PREVENTS: Buffer overrun on invalid prefix lengths (up to 128), truncated data.
// SECURITY: Prefix bytes come directly from untrusted UPDATE NLRI sections.
func FuzzParseIPv6Prefixes(f *testing.F) {
	f.Add([]byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}) // 2001:db8:1::/48
	f.Add([]byte{128,
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}) // 2001:db8::1/128
	f.Add([]byte{0})              // ::/0
	f.Add([]byte{})               // Empty
	f.Add([]byte{129})            // Invalid: > 128
	f.Add([]byte{64, 0x20, 0x01}) // Truncated
	f.Add([]byte{0xFF})           // Length 255

	f.Fuzz(func(t *testing.T, data []byte) {
		_ = ParseIPv6Prefixes(data) // MUST NOT panic
	})
}

// FuzzParsePrefixes tests generic prefix parsing with arbitrary address sizes.
//
// VALIDATES: ParsePrefixes handles arbitrary bytes and address sizes without crashing.
// PREVENTS: Panic on edge-case address sizes combined with malformed data.
// SECURITY: Address size determined by AFI from untrusted OPEN negotiation.
func FuzzParsePrefixes(f *testing.F) {
	f.Add([]byte{24, 10, 0, 0}, 4)                            // IPv4
	f.Add([]byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}, 16) // IPv6
	f.Add([]byte{}, 4)                                        // Empty IPv4
	f.Add([]byte{}, 16)                                       // Empty IPv6
	f.Add([]byte{0xFF}, 4)                                    // Bad prefix len, IPv4

	f.Fuzz(func(t *testing.T, data []byte, addrSize int) {
		// Constrain to valid address family sizes
		if addrSize != 4 && addrSize != 16 {
			return
		}
		_ = ParsePrefixes(data, addrSize) // MUST NOT panic
	})
}
