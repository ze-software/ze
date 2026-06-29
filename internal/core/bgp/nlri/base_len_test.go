// Package nlri tests for ADD-PATH encoding (Phase 4 cleanup).
package nlri

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// TestLen_INET verifies Len returns payload length without path ID.
//
// VALIDATES: Len() returns payload-only length after Phase 3 simplification.
// PREVENTS: Size mismatch when building wire format with ADD-PATH.
func TestLen_INET(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		prefix  netip.Prefix
		pathID  uint32
		wantLen int
	}{
		{
			name:    "IPv4/24 no path",
			prefix:  netip.MustParsePrefix("10.0.0.0/24"),
			pathID:  0,
			wantLen: 1 + 3, // length byte + 3 prefix bytes
		},
		{
			name:    "IPv4/24 with path",
			prefix:  netip.MustParsePrefix("10.0.0.0/24"),
			pathID:  1,
			wantLen: 1 + 3, // Same! Len() excludes path ID
		},
		{
			name:    "IPv4/32 no path",
			prefix:  netip.MustParsePrefix("192.168.1.1/32"),
			pathID:  0,
			wantLen: 1 + 4,
		},
		{
			name:    "IPv4/0 no path",
			prefix:  netip.MustParsePrefix("0.0.0.0/0"),
			pathID:  0,
			wantLen: 1 + 0, // /0 = 0 prefix bytes
		},
		{
			name:    "IPv6/64 with path",
			prefix:  netip.MustParsePrefix("2001:db8::/64"),
			pathID:  100,
			wantLen: 1 + 8, // length byte + 8 prefix bytes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fam := family.IPv4Unicast
			if tt.prefix.Addr().Is6() {
				fam = family.IPv6Unicast
			}

			inet := NewINET(fam, tt.prefix, tt.pathID)

			got := inet.Len()
			if got != tt.wantLen {
				t.Errorf("Len() = %d, want %d", got, tt.wantLen)
			}
		})
	}
}

// Note: TestLen_IPVPN moved to internal/plugin/vpn/vpn_test.go
// Note: TestLen_LabeledUnicast moved to internal/plugins/bgp-nlri-labeled/

// TestWriteTo_INET verifies WriteTo writes payload without path ID.
//
// VALIDATES: WriteTo produces bytes identical to Bytes().
// PREVENTS: Wire format corruption when ADD-PATH is handled externally.
func TestWriteTo_INET(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		prefix netip.Prefix
		pathID uint32
	}{
		{
			name:   "IPv4/24 no path",
			prefix: netip.MustParsePrefix("10.0.0.0/24"),
			pathID: 0,
		},
		{
			name:   "IPv4/24 with path",
			prefix: netip.MustParsePrefix("10.0.0.0/24"),
			pathID: 1,
		},
		{
			name:   "IPv6/64 with path",
			prefix: netip.MustParsePrefix("2001:db8::/64"),
			pathID: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fam := family.IPv4Unicast
			if tt.prefix.Addr().Is6() {
				fam = family.IPv6Unicast
			}

			inet := NewINET(fam, tt.prefix, tt.pathID)

			// Write to buffer using WriteTo
			buf := make([]byte, 100)
			n := inet.WriteTo(buf, 0)

			// Verify length matches Len()
			if n != inet.Len() {
				t.Errorf("WriteTo returned %d, want Len() = %d", n, inet.Len())
			}

			// Bytes() = payload only, should match WriteTo
			expected := inet.Bytes()
			got := buf[:n]

			if len(got) != len(expected) {
				t.Errorf("WriteTo wrote %d bytes, want %d", len(got), len(expected))
			}
			for i := range got {
				if got[i] != expected[i] {
					t.Errorf("byte[%d] = %02x, want %02x", i, got[i], expected[i])
				}
			}
		})
	}
}

// TestWriteNLRI_AddPath verifies WriteNLRI handles ADD-PATH correctly.
//
// VALIDATES: WriteNLRI prepends path ID when ctx.AddPath=true.
// PREVENTS: Missing path ID in ADD-PATH enabled sessions.
func TestWriteNLRI_AddPath(t *testing.T) {
	t.Parallel()
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := NewINET(family.IPv4Unicast, prefix, 0) // No stored path ID

	t.Run("AddPath enabled", func(t *testing.T) {
		t.Parallel()
		buf := make([]byte, 100)
		n := WriteNLRI(inet, buf, 0, true)

		// Should be 4 (path ID) + Len()
		wantLen := 4 + inet.Len()
		if n != wantLen {
			t.Errorf("WriteNLRI returned %d, want %d", n, wantLen)
		}

		// First 4 bytes should be zero (NOPATH)
		if buf[0] != 0 || buf[1] != 0 || buf[2] != 0 || buf[3] != 0 {
			t.Errorf("path ID = %02x%02x%02x%02x, want 00000000", buf[0], buf[1], buf[2], buf[3])
		}
	})

	t.Run("AddPath disabled", func(t *testing.T) {
		t.Parallel()
		buf := make([]byte, 100)
		n := WriteNLRI(inet, buf, 0, false)

		// Should be just Len()
		if n != inet.Len() {
			t.Errorf("WriteNLRI returned %d, want %d", n, inet.Len())
		}
	})
}

// TestWriteNLRI_WithStoredPathID verifies WriteNLRI uses stored path ID.
//
// VALIDATES: WriteNLRI uses stored path ID when addPath=true.
// PREVENTS: Path ID being lost or zeroed when forwarding routes.
func TestWriteNLRI_WithStoredPathID(t *testing.T) {
	t.Parallel()
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := NewINET(family.IPv4Unicast, prefix, 42) // Stored path ID = 42

	buf := make([]byte, 100)
	n := WriteNLRI(inet, buf, 0, true)

	// Path ID should be 42 (big-endian: 0x0000002a)
	if buf[0] != 0 || buf[1] != 0 || buf[2] != 0 || buf[3] != 42 {
		t.Errorf("path ID = %02x%02x%02x%02x, want 0000002a", buf[0], buf[1], buf[2], buf[3])
	}

	// Total length should be 4 + Len()
	wantLen := 4 + inet.Len()
	if n != wantLen {
		t.Errorf("WriteNLRI returned %d, want %d", n, wantLen)
	}
}

// TestLenWithContext_MatchesWriteNLRI verifies predicted length equals actual.
//
// VALIDATES: LenWithContext returns exact bytes WriteNLRI will write.
// PREVENTS: Buffer overflow/underflow when pre-allocating.
func TestLenWithContext_MatchesWriteNLRI(t *testing.T) {
	t.Parallel()
	nlris := []NLRI{
		NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0),
		NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 1),
		NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8::/64"), 0),
		NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8::/64"), 100),
	}

	addPathValues := []bool{false, true}

	for _, n := range nlris {
		for _, addPath := range addPathValues {
			name := "AddPath=false"
			if addPath {
				name = "AddPath=true"
			}

			t.Run(n.String()+"_"+name, func(t *testing.T) {
				t.Parallel()
				predicted := LenWithContext(n, addPath)

				buf := make([]byte, 100)
				actual := WriteNLRI(n, buf, 0, addPath)

				if predicted != actual {
					t.Errorf("LenWithContext = %d, WriteNLRI returned %d", predicted, actual)
				}
			})
		}
	}
}
