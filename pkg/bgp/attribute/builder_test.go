package attribute

import (
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
