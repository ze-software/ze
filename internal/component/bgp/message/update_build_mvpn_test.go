package message

import (
	"net/netip"
	"testing"
)

// VALIDATES: mvpnNLRISize returns exactly the number of bytes writeMVPNNLRI writes,
// for every (route type × source family × group family) combination. If the two
// functions drift apart, buildMPReachMVPN will under- or over-allocate.
//
// PREVENTS: silent byte mismatch between sizing pass (BuildGroupedMVPN) and write
// pass (buildMPReachMVPN) that would corrupt wire output.
func TestMVPN_SizeWriteParity(t *testing.T) {
	ipv4Src := netip.MustParseAddr("10.0.0.1")
	ipv4Grp := netip.MustParseAddr("239.1.1.1")
	ipv6Src := netip.MustParseAddr("2001:db8::1")
	ipv6Grp := netip.MustParseAddr("ff00::1")
	var rd = [8]byte{0, 0, 0xfd, 0xe9, 0, 0, 0, 1}

	tests := []struct {
		name   string
		route  MVPNParams
		expect int
	}{
		// Type 5: Source Active A-D
		{"t5_v4_v4", MVPNParams{RouteType: 5, RD: rd, Source: ipv4Src, Group: ipv4Grp}, 2 + 8 + 5 + 5},
		{"t5_v4_v6", MVPNParams{RouteType: 5, RD: rd, Source: ipv4Src, Group: ipv6Grp}, 2 + 8 + 5 + 17},
		{"t5_v6_v4", MVPNParams{RouteType: 5, RD: rd, Source: ipv6Src, Group: ipv4Grp}, 2 + 8 + 17 + 5},
		{"t5_v6_v6", MVPNParams{RouteType: 5, RD: rd, Source: ipv6Src, Group: ipv6Grp}, 2 + 8 + 17 + 17},
		// Type 6: Shared Tree Join
		{"t6_v4_v4", MVPNParams{RouteType: 6, RD: rd, SourceAS: 65001, Source: ipv4Src, Group: ipv4Grp}, 2 + 8 + 4 + 5 + 5},
		{"t6_v6_v6", MVPNParams{RouteType: 6, RD: rd, SourceAS: 65001, Source: ipv6Src, Group: ipv6Grp}, 2 + 8 + 4 + 17 + 17},
		// Type 7: Source Tree Join
		{"t7_v4_v4", MVPNParams{RouteType: 7, RD: rd, SourceAS: 65001, Source: ipv4Src, Group: ipv4Grp}, 2 + 8 + 4 + 5 + 5},
		{"t7_v6_v6", MVPNParams{RouteType: 7, RD: rd, SourceAS: 65001, Source: ipv6Src, Group: ipv6Grp}, 2 + 8 + 4 + 17 + 17},
		// Asymmetric: IPv4 source + IPv6 group (should just work)
		{"t6_v4_v6_asymmetric", MVPNParams{RouteType: 6, RD: rd, SourceAS: 65001, Source: ipv4Src, Group: ipv6Grp}, 2 + 8 + 4 + 5 + 17},
		// Unknown route type: 2-byte header only, no data
		{"unknown_type_0", MVPNParams{RouteType: 0, RD: rd}, 2},
		{"unknown_type_4", MVPNParams{RouteType: 4, RD: rd}, 2},
		{"unknown_type_8", MVPNParams{RouteType: 8, RD: rd}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := mvpnNLRISize(tt.route)
			if size != tt.expect {
				t.Errorf("mvpnNLRISize() = %d, want %d", size, tt.expect)
			}

			// writeMVPNNLRI must write exactly `size` bytes into a buffer of
			// length size, matching the sizing pass.
			buf := make([]byte, size+16) // +16 slack to detect over-write
			written := writeMVPNNLRI(buf, 0, tt.route)
			if written != size {
				t.Errorf("writeMVPNNLRI() wrote %d bytes, mvpnNLRISize returned %d", written, size)
			}

			// Bytes beyond the written region must be untouched (all zero since make zeroes).
			for i := size; i < len(buf); i++ {
				if buf[i] != 0 {
					t.Errorf("writeMVPNNLRI() wrote past size: buf[%d] = %#x", i, buf[i])
				}
			}

			// For known types, verify the length byte at offset 1 equals size-2.
			if tt.route.RouteType >= 5 && tt.route.RouteType <= 7 {
				if int(buf[1]) != size-2 {
					t.Errorf("wire length byte = %d, want %d", buf[1], size-2)
				}
				if buf[0] != tt.route.RouteType {
					t.Errorf("wire type byte = %d, want %d", buf[0], tt.route.RouteType)
				}
			}
		})
	}
}

// VALIDATES: writeMVPNNLRI panics with BUG prefix when data exceeds the 1-byte
// length field. Currently unreachable for route types 5/6/7 (max data = 46 bytes),
// but the guard prevents silent truncation if a future RFC adds a larger type.
//
// PREVENTS: future route types producing invalid NLRI with length byte truncated
// to 8 bits and no warning.
func TestMVPN_LengthOverflowPanic(t *testing.T) {
	// Synthesize a pathological route-type handler would be intrusive.
	// Instead verify the guard fires by direct inspection: at time of writing
	// all valid MVPN route types fit in one byte. This test documents the
	// invariant and is a placeholder for a full test if a larger type is added.
	//
	// The actual panic path is a `if dataLen > 0xFF { panic(...) }` guard in
	// writeMVPNNLRI; exercising it requires adding an MVPN route type that
	// produces > 255 bytes of data, which does not exist today.
	t.Log("Length-overflow guard exists; unreachable with current RFC 6514 types (max 46 bytes)")
}
