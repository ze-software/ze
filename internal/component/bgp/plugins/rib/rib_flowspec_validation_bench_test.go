package rib

import (
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// BenchmarkRIBFlowSpecValidation measures the RIB's FlowSpec validation
// decode: flowSpecDestination parses a received FlowSpec NLRI through the
// FlowSpec codec and extracts the destination prefix it is validated against
// (RFC 8955 Section 6). The cases cover the IPv4 and IPv6 plain families and
// the IPv4 VPN family, so every codec entry point the RIB calls is timed.
// The NLRI bytes are written out here rather than built by the codec, so the
// benchmark text does not change when the codec's package does, and an
// allocation count taken before such a move compares with one taken after.
func BenchmarkRIBFlowSpecValidation(b *testing.B) {
	rd := []byte{0, 0, 0xfd, 0xe9, 0, 0, 0, 1}
	// Destination 10.0.0.0/24, protocol == 6, destination port == 443.
	v4 := []byte{1, 24, 10, 0, 0, 3, 0x81, 6, 5, 0x91, 0x01, 0xbb}
	// Destination 2001:db8:1::/64 at offset 0, next header == 6.
	v6 := []byte{1, 64, 0, 0x20, 0x01, 0x0d, 0xb8, 0, 1, 0, 0, 3, 0x81, 6}
	cases := []struct {
		name string
		fam  family.Family
		raw  []byte
	}{
		{"ipv4", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}, append([]byte{byte(len(v4))}, v4...)},
		{"ipv6", family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec}, append([]byte{byte(len(v6))}, v6...)},
		{"ipv4-vpn", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpecVPN}, append(append([]byte{byte(len(rd) + len(v4))}, rd...), v4...)},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			_, prefix, _ := flowSpecDestination(tc.fam, tc.raw)
			if !prefix.IsValid() {
				b.Fatalf("flowSpecDestination(%s) found no destination prefix in % x", tc.fam, tc.raw)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				flowSpecDestination(tc.fam, tc.raw)
			}
		})
	}
}
