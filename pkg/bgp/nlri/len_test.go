package nlri

import (
	"net/netip"
	"testing"
)

// TestLenWithContext_MatchesPack verifies that LenWithContext returns the same
// length as len(Pack(ctx)) for all NLRI types and context combinations.
//
// VALIDATES: LenWithContext is consistent with Pack for buffer pre-allocation.
// PREVENTS: Buffer overflow or garbage bytes when using WriteTo after LenWithContext.
func TestLenWithContext_MatchesPack(t *testing.T) {
	// Create test NLRIs of different types
	testCases := []struct {
		name string
		nlri NLRI
	}{
		// INET (IPv4/IPv6 unicast)
		{
			name: "INET_IPv4_noPath",
			nlri: mustParseINET(t, "10.0.0.0/24", false, 0),
		},
		{
			name: "INET_IPv4_withPath",
			nlri: mustParseINET(t, "10.0.0.0/24", true, 42),
		},
		{
			name: "INET_IPv6_noPath",
			nlri: mustParseINET(t, "2001:db8::/32", false, 0),
		},
		{
			name: "INET_IPv6_withPath",
			nlri: mustParseINET(t, "2001:db8::/32", true, 100),
		},
		// IPVPN (VPNv4/VPNv6)
		{
			name: "IPVPN_noPath",
			nlri: mustParseIPVPN(t, "10.0.0.0/24", false, 0),
		},
		{
			name: "IPVPN_withPath",
			nlri: mustParseIPVPN(t, "10.0.0.0/24", true, 55),
		},
		// LabeledUnicast (RFC 8277)
		{
			name: "LabeledUnicast_noPath",
			nlri: mustParseLabeledUnicast(t, "10.0.0.0/24", false, 0),
		},
		{
			name: "LabeledUnicast_withPath",
			nlri: mustParseLabeledUnicast(t, "10.0.0.0/24", true, 77),
		},
		// FlowSpec (no ADD-PATH support)
		{
			name: "FlowSpec_IPv4",
			nlri: mustParseFlowSpec(t, true),
		},
		{
			name: "FlowSpec_IPv6",
			nlri: mustParseFlowSpec(t, false),
		},
	}

	// Test all context combinations
	contexts := []struct {
		name string
		ctx  *PackContext
	}{
		{"nil", nil},
		{"AddPath=false", &PackContext{AddPath: false}},
		{"AddPath=true", &PackContext{AddPath: true}},
	}

	for _, tc := range testCases {
		for _, ctxCase := range contexts {
			name := tc.name + "_" + ctxCase.name
			t.Run(name, func(t *testing.T) {
				nlri := tc.nlri
				ctx := ctxCase.ctx

				// Get length via LenWithContext
				lenFromFunc := LenWithContext(nlri, ctx)

				// Get length via Pack
				packed := nlri.Pack(ctx)
				lenFromPack := len(packed)

				if lenFromFunc != lenFromPack {
					t.Errorf("LenWithContext=%d but len(Pack)=%d for %s",
						lenFromFunc, lenFromPack, name)
				}
			})
		}
	}
}

// TestLenWithContext_MatchesWriteTo verifies that LenWithContext returns the same
// length as WriteTo actually writes.
//
// VALIDATES: Buffer size from LenWithContext is exactly what WriteTo needs.
// PREVENTS: Buffer overflow when WriteTo writes more than LenWithContext predicted.
func TestLenWithContext_MatchesWriteTo(t *testing.T) {
	testCases := []struct {
		name            string
		nlri            NLRI
		supportsAddPath bool // Whether this NLRI type supports ADD-PATH
	}{
		// INET - supports ADD-PATH
		{"INET_IPv4_noPath", mustParseINET(t, "192.168.1.0/24", false, 0), true},
		{"INET_IPv4_withPath", mustParseINET(t, "192.168.1.0/24", true, 1), true},
		{"INET_IPv6_noPath", mustParseINET(t, "fe80::/10", false, 0), true},
		{"INET_IPv6_withPath", mustParseINET(t, "fe80::/10", true, 2), true},
		// IPVPN - supports ADD-PATH
		{"IPVPN_noPath", mustParseIPVPN(t, "10.0.0.0/8", false, 0), true},
		{"IPVPN_withPath", mustParseIPVPN(t, "10.0.0.0/8", true, 3), true},
		// LabeledUnicast - supports ADD-PATH
		{"LabeledUnicast_noPath", mustParseLabeledUnicast(t, "172.16.0.0/16", false, 0), true},
		{"LabeledUnicast_withPath", mustParseLabeledUnicast(t, "172.16.0.0/16", true, 4), true},
		// FlowSpec - does NOT support ADD-PATH
		{"FlowSpec_IPv4", mustParseFlowSpec(t, true), false},
	}

	contexts := []*PackContext{
		nil,
		{AddPath: false},
		{AddPath: true},
	}

	for _, tc := range testCases {
		for _, ctx := range contexts {
			// Skip ADD-PATH tests for NLRI types that don't support it
			if ctx != nil && ctx.AddPath && !tc.supportsAddPath {
				continue
			}

			ctxName := "nil"
			if ctx != nil {
				if ctx.AddPath {
					ctxName = "AddPath=true"
				} else {
					ctxName = "AddPath=false"
				}
			}
			name := tc.name + "_" + ctxName

			t.Run(name, func(t *testing.T) {
				nlri := tc.nlri

				// Get predicted length
				predictedLen := LenWithContext(nlri, ctx)

				// Allocate buffer and write
				buf := make([]byte, predictedLen+10) // Extra space to detect overflow
				written := nlri.WriteTo(buf, 0, ctx)

				if written != predictedLen {
					t.Errorf("LenWithContext=%d but WriteTo wrote %d bytes",
						predictedLen, written)
				}
			})
		}
	}
}

// mustParseINET creates INET NLRI for testing.
func mustParseINET(t *testing.T, prefix string, hasPath bool, pathID uint32) *INET {
	t.Helper()
	p := netip.MustParsePrefix(prefix)
	inet := &INET{
		prefix:  p,
		hasPath: hasPath,
		pathID:  pathID,
	}
	return inet
}

// mustParseIPVPN creates IPVPN NLRI for testing.
func mustParseIPVPN(t *testing.T, prefix string, hasPath bool, pathID uint32) *IPVPN {
	t.Helper()
	p := netip.MustParsePrefix(prefix)
	return &IPVPN{
		prefix:  p,
		hasPath: hasPath,
		pathID:  pathID,
		labels:  []uint32{100}, // Single label
		rd:      RouteDistinguisher{Type: 0, Value: [6]byte{0, 0, 0, 0, 0, 1}},
	}
}

// mustParseLabeledUnicast creates LabeledUnicast NLRI for testing.
func mustParseLabeledUnicast(t *testing.T, prefix string, hasPath bool, pathID uint32) *LabeledUnicast {
	t.Helper()
	p := netip.MustParsePrefix(prefix)
	family := IPv4Unicast
	if p.Addr().Is6() {
		family = IPv6Unicast
	}
	return &LabeledUnicast{
		family:  family,
		prefix:  p,
		labels:  []uint32{16000}, // Single label
		hasPath: hasPath,
		pathID:  pathID,
	}
}

// mustParseFlowSpec creates FlowSpec NLRI for testing.
// FlowSpec doesn't support ADD-PATH, so hasPath is always false.
func mustParseFlowSpec(t *testing.T, isIPv4 bool) *FlowSpec {
	t.Helper()
	family := Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}
	if !isIPv4 {
		family = Family{AFI: AFIIPv6, SAFI: SAFIFlowSpec}
	}
	// Create FlowSpec with pre-cached bytes (simulating parsed FlowSpec)
	// This avoids needing to construct components
	fs := &FlowSpec{
		family:     family,
		components: nil,
		cached:     []byte{0x03, 0x01, 0x18, 0x0a}, // Simple dest prefix 10.0.0.0/24
	}
	return fs
}
