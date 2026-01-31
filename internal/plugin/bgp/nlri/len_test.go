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
		// Note: VPN tests moved to internal/plugin/vpn/
		// LabeledUnicast (RFC 8277)
		{
			name: "LabeledUnicast_noPath",
			nlri: mustParseLabeledUnicast(t, "10.0.0.0/24", false, 0),
		},
		{
			name: "LabeledUnicast_withPath",
			nlri: mustParseLabeledUnicast(t, "10.0.0.0/24", true, 77),
		},
	}

	// Test all context combinations
	addPathValues := []struct {
		name    string
		addPath bool
	}{
		{"AddPath=false", false},
		{"AddPath=true", true},
	}

	for _, tc := range testCases {
		for _, ap := range addPathValues {
			name := tc.name + "_" + ap.name
			t.Run(name, func(t *testing.T) {
				nlri := tc.nlri

				// Get length via LenWithContext
				lenFromFunc := LenWithContext(nlri, ap.addPath)

				// Get length via WriteNLRI
				buf := make([]byte, 100)
				written := WriteNLRI(nlri, buf, 0, ap.addPath)

				if lenFromFunc != written {
					t.Errorf("LenWithContext=%d but WriteNLRI wrote %d for %s",
						lenFromFunc, written, name)
				}
			})
		}
	}
}

// TestLenWithContext_MatchesWriteNLRI_AllTypes verifies that LenWithContext returns the same
// length as WriteNLRI actually writes for all NLRI types.
//
// Phase 3: WriteTo writes payload only. Use WriteNLRI for ADD-PATH encoding.
//
// VALIDATES: Buffer size from LenWithContext is exactly what WriteNLRI needs.
// PREVENTS: Buffer overflow when WriteNLRI writes more than LenWithContext predicted.
func TestLenWithContext_MatchesWriteNLRI_AllTypes(t *testing.T) {
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
		// VPN - supports ADD-PATH (tests moved to internal/plugin/vpn)
		// LabeledUnicast - supports ADD-PATH
		{"LabeledUnicast_noPath", mustParseLabeledUnicast(t, "172.16.0.0/16", false, 0), true},
		{"LabeledUnicast_withPath", mustParseLabeledUnicast(t, "172.16.0.0/16", true, 4), true},
		// Note: EVPN tests moved to internal/plugin/evpn/types_test.go
	}

	addPathValues := []bool{false, true}

	for _, tc := range testCases {
		for _, addPath := range addPathValues {
			// Skip ADD-PATH tests for NLRI types that don't support it
			if addPath && !tc.supportsAddPath {
				continue
			}

			ctxName := "AddPath=false"
			if addPath {
				ctxName = "AddPath=true"
			}
			name := tc.name + "_" + ctxName

			t.Run(name, func(t *testing.T) {
				nlri := tc.nlri

				// Get predicted length
				predictedLen := LenWithContext(nlri, addPath)

				// Allocate buffer and write using WriteNLRI (not WriteTo)
				buf := make([]byte, predictedLen+10) // Extra space to detect overflow
				written := WriteNLRI(nlri, buf, 0, addPath)

				if written != predictedLen {
					t.Errorf("LenWithContext=%d but WriteNLRI wrote %d bytes",
						predictedLen, written)
				}
			})
		}
	}
}

// mustParseINET creates INET NLRI for testing.
// hasPath parameter is kept for API compatibility but ignored - pathID!=0 implies path exists.
func mustParseINET(t *testing.T, prefix string, _ bool, pathID uint32) *INET {
	t.Helper()
	p := netip.MustParsePrefix(prefix)
	return &INET{
		PrefixNLRI: PrefixNLRI{
			prefix: p,
			pathID: pathID,
		},
	}
}

// mustParseLabeledUnicast creates LabeledUnicast NLRI for testing.
// hasPath parameter is kept for API compatibility but ignored - pathID!=0 implies path exists.
func mustParseLabeledUnicast(t *testing.T, prefix string, _ bool, pathID uint32) *LabeledUnicast {
	t.Helper()
	p := netip.MustParsePrefix(prefix)
	family := IPv4Unicast
	if p.Addr().Is6() {
		family = IPv6Unicast
	}
	return &LabeledUnicast{
		PrefixNLRI: PrefixNLRI{
			family: family,
			prefix: p,
			pathID: pathID,
		},
		labels: []uint32{16000}, // Single label
	}
}
