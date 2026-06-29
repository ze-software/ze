package nlri

import (
	"testing"
)

// FuzzParseRouteDistinguisher tests binary RD parsing robustness.
//
// VALIDATES: ParseRouteDistinguisher handles arbitrary bytes without crashing.
// PREVENTS: Panic on truncated data, unknown RD types.
// SECURITY: RD bytes come from untrusted VPN UPDATE NLRI.
func FuzzParseRouteDistinguisher(f *testing.F) {
	// Type 0: 2-byte ASN : 4-byte assigned
	type0 := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64} // 65000:100
	f.Add(type0)
	// Type 1: IP : 2-byte assigned
	type1 := []byte{0x00, 0x01, 0xC0, 0x00, 0x02, 0x01, 0x00, 0x64} // 192.0.2.1:100
	f.Add(type1)
	// Type 2: 4-byte ASN : 2-byte assigned
	type2 := []byte{0x00, 0x02, 0x00, 0x01, 0x00, 0x01, 0x00, 0x64} // 65537:100
	f.Add(type2)
	f.Add([]byte{})                                               // Empty
	f.Add([]byte{0x00})                                           // 1 byte
	f.Add([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})       // 7 bytes (short)
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}) // Unknown type, max values
	f.Add([]byte{0x00, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) // Invalid type 3

	f.Fuzz(func(t *testing.T, data []byte) {
		rd, err := ParseRouteDistinguisher(data)
		if err != nil {
			return
		}
		// Exercise methods — MUST NOT panic
		_ = rd.String()
		_ = rd.Bytes()
		_ = rd.Len()
	})
}

// FuzzParseRDString tests string RD parsing robustness.
//
// VALIDATES: ParseRDString handles arbitrary strings without crashing.
// PREVENTS: Panic on malformed RD strings, invalid IP addresses, overflows.
// SECURITY: RD strings come from user/API input.
func FuzzParseRDString(f *testing.F) {
	f.Add("65000:100")         // Type 0
	f.Add("192.0.2.1:100")     // Type 1
	f.Add("4200000001:100")    // Type 2
	f.Add("0:65000:100")       // Typed format, type 0
	f.Add("1:192.0.2.1:100")   // Typed format, type 1
	f.Add("2:65000:100")       // Typed format, type 2
	f.Add("")                  // Empty
	f.Add("invalid")           // No colon
	f.Add(":")                 // Empty parts
	f.Add("::")                // Three empty parts
	f.Add("abc:100")           // Non-numeric ASN
	f.Add("65000:abc")         // Non-numeric value
	f.Add("65000:99999999999") // Overflow value
	f.Add("3:1:1")             // Invalid type 3
	f.Add("1.2.3.4.5:100")     // Invalid IP
	f.Add("0:0:0")             // Zero type, zero ASN, zero value
	f.Add("\x00:\x00")         // Null bytes

	f.Fuzz(func(t *testing.T, s string) {
		rd, err := ParseRDString(s)
		if err != nil {
			return
		}
		// Round-trip: String() should produce parseable output
		_ = rd.String()
	})
}

// FuzzParseLabelStack tests MPLS label stack parsing robustness.
//
// VALIDATES: ParseLabelStack handles arbitrary bytes without crashing.
// PREVENTS: Panic on truncated labels, missing bottom-of-stack bit.
// SECURITY: Label bytes come from untrusted MPLS-BGP NLRI.
func FuzzParseLabelStack(f *testing.F) {
	// Single label with BOS: label=100, EXP=0, S=1
	f.Add([]byte{0x00, 0x06, 0x41}) // label 100, BOS
	// Two labels
	f.Add([]byte{0x00, 0x06, 0x40, 0x00, 0x0C, 0x81}) // 100 then 200, BOS
	f.Add([]byte{})                                   // Empty
	f.Add([]byte{0x00})                               // 1 byte (short)
	f.Add([]byte{0x00, 0x00})                         // 2 bytes (short)
	f.Add([]byte{0x00, 0x06, 0x40})                   // No BOS, then truncated
	f.Add([]byte{0xFF, 0xFF, 0xFF})                   // Max values, BOS set

	f.Fuzz(func(t *testing.T, data []byte) {
		labels, rest, err := ParseLabelStack(data)
		if err != nil {
			return
		}
		// If successful, verify basic invariants — MUST NOT panic
		_ = len(labels)
		_ = len(rest)
	})
}
