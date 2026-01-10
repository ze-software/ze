package attribute

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuilderOrigin verifies ORIGIN attribute encoding.
//
// VALIDATES: Builder produces correct ORIGIN wire format.
// PREVENTS: Incorrect origin values in announcements.
func TestBuilderOrigin(t *testing.T) {
	b := NewBuilder()
	b.SetOrigin(0) // IGP

	wire := b.Build()

	// Should have ORIGIN: flags=0x40, code=1, len=1, value=0
	require.Len(t, wire, 4)
	assert.Equal(t, byte(0x40), wire[0]) // Transitive
	assert.Equal(t, byte(1), wire[1])    // ORIGIN
	assert.Equal(t, byte(1), wire[2])    // Length
	assert.Equal(t, byte(0), wire[3])    // IGP
}

// TestBuilderLocalPref verifies LOCAL_PREF attribute encoding.
//
// VALIDATES: Builder produces correct LOCAL_PREF wire format.
// PREVENTS: Incorrect local preference in iBGP announcements.
func TestBuilderLocalPref(t *testing.T) {
	b := NewBuilder()
	b.SetLocalPref(200)

	wire := b.Build()

	// Should have ORIGIN (4 bytes) + LOCAL_PREF (7 bytes)
	require.Len(t, wire, 11)

	// Check LOCAL_PREF at offset 4
	assert.Equal(t, byte(0x40), wire[4]) // Transitive
	assert.Equal(t, byte(5), wire[5])    // LOCAL_PREF
	assert.Equal(t, byte(4), wire[6])    // Length
	// Value: 200 = 0x000000C8
	assert.Equal(t, []byte{0, 0, 0, 200}, wire[7:11])
}

// TestBuilderMED verifies MED attribute encoding.
//
// VALIDATES: Builder produces correct MED wire format.
// PREVENTS: Incorrect MED values affecting route selection.
func TestBuilderMED(t *testing.T) {
	b := NewBuilder()
	b.SetMED(100)

	wire := b.Build()

	// Should have ORIGIN (4 bytes) + MED (7 bytes)
	require.Len(t, wire, 11)

	// Check MED at offset 4
	assert.Equal(t, byte(0x80), wire[4]) // Optional
	assert.Equal(t, byte(4), wire[5])    // MED
	assert.Equal(t, byte(4), wire[6])    // Length
	assert.Equal(t, []byte{0, 0, 0, 100}, wire[7:11])
}

// TestBuilderASPath verifies AS_PATH attribute encoding.
//
// VALIDATES: Builder produces correct AS_PATH wire format.
// PREVENTS: Loop detection failures from malformed AS_PATH.
func TestBuilderASPath(t *testing.T) {
	b := NewBuilder()
	b.SetASPath([]uint32{65001, 65002})

	wire := b.Build()

	// ORIGIN (4) + AS_PATH header (3) + segment header (2) + 2 ASNs (8) = 17
	require.Len(t, wire, 17)

	// Check AS_PATH starts at offset 4
	assert.Equal(t, byte(0x40), wire[4])                   // Transitive
	assert.Equal(t, byte(2), wire[5])                      // AS_PATH
	assert.Equal(t, byte(10), wire[6])                     // Length: 2 + 4*2 = 10
	assert.Equal(t, byte(ASSequence), wire[7])             // Segment type
	assert.Equal(t, byte(2), wire[8])                      // Segment count
	assert.Equal(t, []byte{0, 0, 0xFD, 0xE9}, wire[9:13])  // 65001
	assert.Equal(t, []byte{0, 0, 0xFD, 0xEA}, wire[13:17]) // 65002
}

// TestBuilderCommunities verifies COMMUNITY attribute encoding.
//
// VALIDATES: Builder produces correct community wire format.
// PREVENTS: Policy failures from malformed communities.
func TestBuilderCommunities(t *testing.T) {
	b := NewBuilder()
	b.AddCommunity(65000, 100)
	b.AddCommunity(65000, 200)

	wire := b.Build()

	// ORIGIN (4) + COMMUNITY header (3) + 2 communities (8) = 15
	require.Len(t, wire, 15)

	// Check COMMUNITY starts at offset 4
	assert.Equal(t, byte(0xC0), wire[4]) // Optional + Transitive
	assert.Equal(t, byte(8), wire[5])    // COMMUNITY
	assert.Equal(t, byte(8), wire[6])    // Length: 2 * 4 = 8
	// First community: 65000:100 = 0xFDE80064
	assert.Equal(t, []byte{0xFD, 0xE8, 0, 100}, wire[7:11])
	// Second community: 65000:200 = 0xFDE800C8
	assert.Equal(t, []byte{0xFD, 0xE8, 0, 200}, wire[11:15])
}

// TestBuilderLargeCommunities verifies LARGE_COMMUNITY encoding.
//
// VALIDATES: Builder produces correct large community wire format.
// PREVENTS: Policy failures from malformed large communities.
func TestBuilderLargeCommunities(t *testing.T) {
	b := NewBuilder()
	b.AddLargeCommunity(65000, 1, 2)

	wire := b.Build()

	// ORIGIN (4) + LARGE_COMMUNITY header (3) + 1 large community (12) = 19
	require.Len(t, wire, 19)

	// Check LARGE_COMMUNITY starts at offset 4
	assert.Equal(t, byte(0xC0), wire[4]) // Optional + Transitive
	assert.Equal(t, byte(32), wire[5])   // LARGE_COMMUNITY
	assert.Equal(t, byte(12), wire[6])   // Length
}

// TestBuilderChaining verifies method chaining.
//
// VALIDATES: Builder methods return self for chaining.
// PREVENTS: Verbose code when building multiple attributes.
func TestBuilderChaining(t *testing.T) {
	wire := NewBuilder().
		SetOrigin(0).
		SetLocalPref(100).
		SetMED(50).
		SetASPath([]uint32{65001}).
		AddCommunity(65000, 100).
		Build()

	// Should have all attributes
	assert.True(t, len(wire) > 20)
}

// TestBuilderEmpty verifies empty builder behavior.
//
// VALIDATES: Empty builder still produces ORIGIN.
// PREVENTS: Missing mandatory attributes.
func TestBuilderEmpty(t *testing.T) {
	b := NewBuilder()
	wire := b.Build()

	// Should have ORIGIN with default IGP
	require.Len(t, wire, 4)
	assert.Equal(t, byte(0), wire[3]) // IGP
}

// TestBuilderWirePassthrough verifies wire passthrough.
//
// VALIDATES: SetWire returns bytes directly without rebuilding.
// PREVENTS: Unnecessary re-encoding for forwarded routes.
func TestBuilderWirePassthrough(t *testing.T) {
	original := []byte{0x40, 0x01, 0x01, 0x00}
	b := NewBuilder()
	b.SetWire(original)

	wire := b.Build()
	assert.Equal(t, original, wire)
}

// TestBuilderReset verifies reset clears all state.
//
// VALIDATES: Reset allows builder reuse.
// PREVENTS: State leakage between builds.
func TestBuilderReset(t *testing.T) {
	b := NewBuilder()
	b.SetOrigin(1)
	b.SetLocalPref(100)
	b.SetMED(50)

	b.Reset()

	assert.True(t, b.IsEmpty())
	wire := b.Build()
	// Should just have default ORIGIN
	assert.Len(t, wire, 4)
}

// TestBuilderNextHop verifies NEXT_HOP attribute encoding.
//
// VALIDATES: Builder produces correct NEXT_HOP wire format.
// PREVENTS: Routing failures from malformed next-hop.
func TestBuilderNextHop(t *testing.T) {
	b := NewBuilder()
	b.SetNextHop([4]byte{192, 168, 1, 1})

	wire := b.Build()

	// ORIGIN (4) + NEXT_HOP (7) = 11
	require.Len(t, wire, 11)

	// Check NEXT_HOP at offset 4
	assert.Equal(t, byte(0x40), wire[4])              // Transitive
	assert.Equal(t, byte(3), wire[5])                 // NEXT_HOP
	assert.Equal(t, byte(4), wire[6])                 // Length
	assert.Equal(t, []byte{192, 168, 1, 1}, wire[7:]) // IP address
}

// TestBuilderNextHopAddr verifies NEXT_HOP from netip.Addr.
//
// VALIDATES: SetNextHopAddr correctly converts netip.Addr.
// PREVENTS: Address conversion errors.
func TestBuilderNextHopAddr(t *testing.T) {
	b := NewBuilder()
	addr := netip.MustParseAddr("10.0.0.1")
	b.SetNextHopAddr(addr)

	wire := b.Build()

	// Check NEXT_HOP value
	assert.Equal(t, []byte{10, 0, 0, 1}, wire[7:11])
}

// TestBuilderNextHopAddrIPv6Ignored verifies IPv6 is ignored for NEXT_HOP.
//
// VALIDATES: IPv6 addresses don't set NEXT_HOP attribute.
// PREVENTS: Invalid NEXT_HOP encoding for IPv6 routes.
func TestBuilderNextHopAddrIPv6Ignored(t *testing.T) {
	b := NewBuilder()
	addr := netip.MustParseAddr("2001:db8::1")
	b.SetNextHopAddr(addr)

	wire := b.Build()

	// Should only have ORIGIN (no NEXT_HOP for IPv6)
	assert.Len(t, wire, 4)
}

// TestBuilderLen verifies Len() returns correct size.
//
// VALIDATES: Len() matches Build() output length.
// PREVENTS: Buffer size mismatches in zero-allocation encoding.
func TestBuilderLen(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Builder)
	}{
		{"empty", func(b *Builder) {}},
		{"origin_only", func(b *Builder) { b.SetOrigin(1) }},
		{"all_attrs", func(b *Builder) {
			b.SetOrigin(0).SetLocalPref(100).SetMED(50)
			b.SetASPath([]uint32{65001, 65002})
			b.AddCommunity(65000, 100)
			b.AddLargeCommunity(65000, 1, 2)
			b.AddExtendedCommunity(ExtendedCommunity{0x00, 0x02, 0xFD, 0xE8, 0, 0, 0, 100})
		}},
		{"wire_passthrough", func(b *Builder) {
			b.SetWire([]byte{0x40, 0x01, 0x01, 0x00})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder()
			tt.setup(b)

			expectedLen := len(b.Build())
			actualLen := b.Len()

			assert.Equal(t, expectedLen, actualLen)
		})
	}
}

// TestBuilderWriteTo verifies WriteTo produces same output as Build.
//
// VALIDATES: WriteTo produces identical wire format as Build.
// PREVENTS: Inconsistency between Build and WriteTo.
func TestBuilderWriteTo(t *testing.T) {
	b := NewBuilder()
	b.SetOrigin(0).SetLocalPref(200).SetMED(100)
	b.SetASPath([]uint32{65001, 65002, 65003})
	b.AddCommunity(65000, 100)
	b.AddCommunity(65000, 200)
	b.AddLargeCommunity(65000, 1, 2)

	// Get expected output from Build
	expected := b.Build()

	// Use WriteTo with pre-allocated buffer
	buf := make([]byte, b.Len())
	written := b.WriteTo(buf)

	assert.Equal(t, len(expected), written)
	assert.Equal(t, expected, buf[:written])
}

// TestBuilderWriteToWire verifies WriteTo with wire passthrough.
//
// VALIDATES: WriteTo correctly handles pre-built wire bytes.
// PREVENTS: Wire passthrough failing with WriteTo.
func TestBuilderWriteToWire(t *testing.T) {
	wire := []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64}
	b := NewBuilder()
	b.SetWire(wire)

	buf := make([]byte, b.Len())
	written := b.WriteTo(buf)

	assert.Equal(t, len(wire), written)
	assert.Equal(t, wire, buf[:written])
}
