// Package nlri tests for ADD-PATH encoding simplification.
package nlri

import (
	"net/netip"
	"testing"
)

// TestBaseLen_INET verifies BaseLen returns payload length without path ID.
//
// VALIDATES: BaseLen equals Len() after Phase 3 simplification.
// PREVENTS: Size mismatch when building wire format with ADD-PATH.
func TestBaseLen_INET(t *testing.T) {
	tests := []struct {
		name     string
		prefix   netip.Prefix
		pathID   uint32
		wantBase int // BaseLen (no path ID)
	}{
		{
			name:     "IPv4/24 no path",
			prefix:   netip.MustParsePrefix("10.0.0.0/24"),
			pathID:   0,
			wantBase: 1 + 3, // length byte + 3 prefix bytes
		},
		{
			name:     "IPv4/24 with path",
			prefix:   netip.MustParsePrefix("10.0.0.0/24"),
			pathID:   1,
			wantBase: 1 + 3, // Same! Len() no longer includes path ID
		},
		{
			name:     "IPv4/32 no path",
			prefix:   netip.MustParsePrefix("192.168.1.1/32"),
			pathID:   0,
			wantBase: 1 + 4,
		},
		{
			name:     "IPv4/0 no path",
			prefix:   netip.MustParsePrefix("0.0.0.0/0"),
			pathID:   0,
			wantBase: 1 + 0, // /0 = 0 prefix bytes
		},
		{
			name:     "IPv6/64 with path",
			prefix:   netip.MustParsePrefix("2001:db8::/64"),
			pathID:   100,
			wantBase: 1 + 8, // length byte + 8 prefix bytes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			family := IPv4Unicast
			if tt.prefix.Addr().Is6() {
				family = IPv6Unicast
			}

			inet := NewINET(family, tt.prefix, tt.pathID)

			got := inet.BaseLen()
			if got != tt.wantBase {
				t.Errorf("BaseLen() = %d, want %d", got, tt.wantBase)
			}

			// Phase 3: Len() = BaseLen() always (no path ID included)
			if inet.Len() != got {
				t.Errorf("Len() = %d, want BaseLen() = %d", inet.Len(), got)
			}
		})
	}
}

// TestBaseLen_IPVPN verifies BaseLen returns payload length without path ID.
//
// VALIDATES: BaseLen equals Len() after Phase 3 simplification.
// PREVENTS: Size mismatch in VPN route encoding.
func TestBaseLen_IPVPN(t *testing.T) {
	rd, _ := ParseRDString("65000:100")

	tests := []struct {
		name     string
		prefix   netip.Prefix
		labels   []uint32
		pathID   uint32
		wantBase int
	}{
		{
			name:     "VPNv4/24 one label no path",
			prefix:   netip.MustParsePrefix("10.0.0.0/24"),
			labels:   []uint32{16000},
			pathID:   0,
			wantBase: 1 + 3 + 8 + 3, // length + labels + RD + prefix
		},
		{
			name:     "VPNv4/24 one label with path",
			prefix:   netip.MustParsePrefix("10.0.0.0/24"),
			labels:   []uint32{16000},
			pathID:   1,
			wantBase: 1 + 3 + 8 + 3, // Same! Len() no longer includes path ID
		},
		{
			name:     "VPNv4/32 two labels",
			prefix:   netip.MustParsePrefix("192.168.1.1/32"),
			labels:   []uint32{16000, 17000},
			pathID:   0,
			wantBase: 1 + 6 + 8 + 4, // length + 2*3 labels + RD + prefix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			family := IPv4VPN
			if tt.prefix.Addr().Is6() {
				family = IPv6VPN
			}

			vpn := NewIPVPN(family, rd, tt.labels, tt.prefix, tt.pathID)

			got := vpn.BaseLen()
			if got != tt.wantBase {
				t.Errorf("BaseLen() = %d, want %d", got, tt.wantBase)
			}

			// Phase 3: Len() = BaseLen() always
			if vpn.Len() != got {
				t.Errorf("Len() = %d, want BaseLen() = %d", vpn.Len(), got)
			}
		})
	}
}

// TestBaseLen_LabeledUnicast verifies BaseLen returns payload length without path ID.
//
// VALIDATES: BaseLen equals Len() after Phase 3 simplification.
// PREVENTS: Size mismatch in MPLS-labeled route encoding.
func TestBaseLen_LabeledUnicast(t *testing.T) {
	tests := []struct {
		name     string
		prefix   netip.Prefix
		labels   []uint32
		pathID   uint32
		wantBase int
	}{
		{
			name:     "IPv4/24 one label no path",
			prefix:   netip.MustParsePrefix("10.0.0.0/24"),
			labels:   []uint32{16000},
			pathID:   0,
			wantBase: 1 + 3 + 3, // length + labels + prefix
		},
		{
			name:     "IPv4/24 one label with path",
			prefix:   netip.MustParsePrefix("10.0.0.0/24"),
			labels:   []uint32{16000},
			pathID:   1,
			wantBase: 1 + 3 + 3, // Same! Len() no longer includes path ID
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			family := IPv4LabeledUnicast
			if tt.prefix.Addr().Is6() {
				family = IPv6LabeledUnicast
			}

			lu := NewLabeledUnicast(family, tt.prefix, tt.labels, tt.pathID)

			got := lu.BaseLen()
			if got != tt.wantBase {
				t.Errorf("BaseLen() = %d, want %d", got, tt.wantBase)
			}

			// Phase 3: Len() = BaseLen() always
			if lu.Len() != got {
				t.Errorf("Len() = %d, want BaseLen() = %d", lu.Len(), got)
			}
		})
	}
}

// TestWritePayloadTo_INET verifies WritePayloadTo writes payload without path ID.
//
// VALIDATES: WritePayloadTo produces bytes identical to Bytes().
// PREVENTS: Wire format corruption when ADD-PATH is handled externally.
func TestWritePayloadTo_INET(t *testing.T) {
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
			family := IPv4Unicast
			if tt.prefix.Addr().Is6() {
				family = IPv6Unicast
			}

			inet := NewINET(family, tt.prefix, tt.pathID)

			// Write to buffer
			buf := make([]byte, 100)
			n := inet.WritePayloadTo(buf, 0)

			// Verify length matches BaseLen
			if n != inet.BaseLen() {
				t.Errorf("WritePayloadTo returned %d, want BaseLen() = %d", n, inet.BaseLen())
			}

			// Phase 3: Bytes() = payload only, should match WritePayloadTo
			expected := inet.Bytes()
			got := buf[:n]

			if len(got) != len(expected) {
				t.Errorf("WritePayloadTo wrote %d bytes, want %d", len(got), len(expected))
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
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := NewINET(IPv4Unicast, prefix, 0) // No stored path ID

	t.Run("AddPath enabled", func(t *testing.T) {
		ctx := &PackContext{AddPath: true}
		buf := make([]byte, 100)
		n := WriteNLRI(inet, buf, 0, ctx)

		// Should be 4 (path ID) + BaseLen
		wantLen := 4 + inet.BaseLen()
		if n != wantLen {
			t.Errorf("WriteNLRI returned %d, want %d", n, wantLen)
		}

		// First 4 bytes should be zero (NOPATH)
		if buf[0] != 0 || buf[1] != 0 || buf[2] != 0 || buf[3] != 0 {
			t.Errorf("path ID = %02x%02x%02x%02x, want 00000000", buf[0], buf[1], buf[2], buf[3])
		}
	})

	t.Run("AddPath disabled", func(t *testing.T) {
		ctx := &PackContext{AddPath: false}
		buf := make([]byte, 100)
		n := WriteNLRI(inet, buf, 0, ctx)

		// Should be just BaseLen
		if n != inet.BaseLen() {
			t.Errorf("WriteNLRI returned %d, want %d", n, inet.BaseLen())
		}
	})

	t.Run("nil context", func(t *testing.T) {
		buf := make([]byte, 100)
		n := WriteNLRI(inet, buf, 0, nil)

		// Phase 3: nil context = payload only (no path ID)
		if n != inet.BaseLen() {
			t.Errorf("WriteNLRI returned %d, want %d", n, inet.BaseLen())
		}
	})
}

// TestWriteNLRI_WithStoredPathID verifies WriteNLRI uses stored path ID.
//
// VALIDATES: WriteNLRI uses stored path ID when ctx.AddPath=true.
// PREVENTS: Path ID being lost or zeroed when forwarding routes.
func TestWriteNLRI_WithStoredPathID(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := NewINET(IPv4Unicast, prefix, 42) // Stored path ID = 42

	ctx := &PackContext{AddPath: true}
	buf := make([]byte, 100)
	n := WriteNLRI(inet, buf, 0, ctx)

	// Path ID should be 42 (big-endian: 0x0000002a)
	if buf[0] != 0 || buf[1] != 0 || buf[2] != 0 || buf[3] != 42 {
		t.Errorf("path ID = %02x%02x%02x%02x, want 0000002a", buf[0], buf[1], buf[2], buf[3])
	}

	// Total length should be 4 + BaseLen
	wantLen := 4 + inet.BaseLen()
	if n != wantLen {
		t.Errorf("WriteNLRI returned %d, want %d", n, wantLen)
	}
}

// TestLenWithContext_MatchesWriteNLRI verifies predicted length equals actual.
//
// VALIDATES: LenWithContext returns exact bytes WriteNLRI will write.
// PREVENTS: Buffer overflow/underflow when pre-allocating.
func TestLenWithContext_MatchesWriteNLRI(t *testing.T) {
	nlris := []NLRI{
		NewINET(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0),
		NewINET(IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 1),
		NewINET(IPv6Unicast, netip.MustParsePrefix("2001:db8::/64"), 0),
		NewINET(IPv6Unicast, netip.MustParsePrefix("2001:db8::/64"), 100),
	}

	contexts := []*PackContext{
		nil,
		{AddPath: false},
		{AddPath: true},
	}

	for _, n := range nlris {
		for _, ctx := range contexts {
			ctxName := "nil"
			if ctx != nil {
				if ctx.AddPath {
					ctxName = "AddPath=true"
				} else {
					ctxName = "AddPath=false"
				}
			}

			t.Run(n.String()+"_"+ctxName, func(t *testing.T) {
				predicted := LenWithContext(n, ctx)

				buf := make([]byte, 100)
				actual := WriteNLRI(n, buf, 0, ctx)

				if predicted != actual {
					t.Errorf("LenWithContext = %d, WriteNLRI returned %d", predicted, actual)
				}
			})
		}
	}
}
