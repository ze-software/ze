package store

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// TestNLRIKey_IPv4 validates AC-2: IPv4 prefix round-trips through NLRIKey.
//
// VALIDATES: NLRIKey correctly encodes IPv4 prefix-len + prefix bytes.
// PREVENTS: Trailing zeros corrupting NLRI bytes on round-trip.
func TestNLRIKey_IPv4(t *testing.T) {
	// /24 prefix: [prefix-len=24][10][0][0]
	nlri := []byte{24, 10, 0, 0}
	key := newNLRIKey(nlri)

	assert.Equal(t, 4, key.Len())
	assert.Equal(t, nlri, key.Bytes())
}

// TestNLRIKey_IPv6 validates AC-3: IPv6 prefix round-trips through NLRIKey.
//
// VALIDATES: NLRIKey correctly encodes IPv6 prefix bytes.
// PREVENTS: Longer NLRI bytes being truncated or padded incorrectly.
func TestNLRIKey_IPv6(t *testing.T) {
	// /48 prefix: [prefix-len=48][2001:0db8:0001] = 7 bytes
	nlri := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}
	key := newNLRIKey(nlri)

	assert.Equal(t, 7, key.Len())
	assert.Equal(t, nlri, key.Bytes())
}

// TestNLRIKey_AddPath validates AC-4: ADD-PATH prefix with path-id round-trips.
//
// VALIDATES: NLRIKey includes 4-byte path-id prefix.
// PREVENTS: Path-id being lost or corrupted in fixed-size key.
func TestNLRIKey_AddPath(t *testing.T) {
	// ADD-PATH: [path-id:4][prefix-len=24][10][0][0]
	nlri := []byte{0, 0, 0, 42, 24, 10, 0, 0}
	key := newNLRIKey(nlri)

	assert.Equal(t, 8, key.Len())
	assert.Equal(t, nlri, key.Bytes())
}

// TestNLRIKey_MaxLength validates AC-4: max NLRI length (21 bytes) fits.
//
// VALIDATES: ADD-PATH IPv6 /128 (21 bytes) fits in [24]byte.
// PREVENTS: Max-length NLRI being truncated.
func TestNLRIKey_MaxLength(t *testing.T) {
	// ADD-PATH IPv6 /128: [path-id:4][prefix-len=128][16 bytes addr] = 21 bytes
	nlri := make([]byte, 21)
	nlri[3] = 1    // path-id = 1
	nlri[4] = 128  // prefix-len
	nlri[5] = 0x20 // 2001:...
	nlri[6] = 0x01
	key := newNLRIKey(nlri)

	assert.Equal(t, 21, key.Len())
	assert.Equal(t, nlri, key.Bytes())
}

// TestNLRIKey_Equality validates AC-2: same input produces equal keys.
//
// VALIDATES: NLRIKey is deterministic and comparable.
// PREVENTS: Map lookups failing due to non-deterministic key encoding.
func TestNLRIKey_Equality(t *testing.T) {
	nlri := []byte{24, 10, 0, 0}
	k1 := newNLRIKey(nlri)
	k2 := newNLRIKey(nlri)

	assert.Equal(t, k1, k2, "same input must produce equal keys")

	different := []byte{24, 10, 0, 1}
	k3 := newNLRIKey(different)
	assert.NotEqual(t, k1, k3, "different input must produce unequal keys")
}

// TestNLRIKey_Empty validates edge case: zero-length NLRI.
//
// VALIDATES: Empty NLRI produces a valid key with Len()==0.
// PREVENTS: Panic on empty input.
func TestNLRIKey_Empty(t *testing.T) {
	key := newNLRIKey(nil)
	assert.Equal(t, 0, key.Len())
	assert.Equal(t, []byte{}, key.Bytes())

	key2 := newNLRIKey([]byte{})
	assert.Equal(t, 0, key2.Len())
	assert.Equal(t, key, key2)
}

// TestNLRIKey_Oversized validates safety: input > 24 bytes is truncated.
//
// VALIDATES: No panic on oversized input.
// PREVENTS: Index out of bounds.
func TestNLRIKey_Oversized(t *testing.T) {
	nlri := make([]byte, 30)
	for i := range nlri {
		nlri[i] = byte(i)
	}
	key := newNLRIKey(nlri)

	assert.Equal(t, 24, key.Len())
	assert.Equal(t, nlri[:24], key.Bytes())
}

// TestNLRIToPrefix_IPv4 verifies IPv4 NLRI wire bytes convert to correct prefix.
//
// VALIDATES: NLRIToPrefix decodes [prefix-len][prefix-bytes] for IPv4.
// PREVENTS: Wrong prefix length, wrong address, wrong byte count calculation.
func TestNLRIToPrefix_IPv4(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}
	tests := []struct {
		name string
		nlri []byte
		want netip.Prefix
		ok   bool
	}{
		{"default", []byte{0}, netip.MustParsePrefix("0.0.0.0/0"), true},
		{"/8", []byte{8, 10}, netip.MustParsePrefix("10.0.0.0/8"), true},
		{"/24", []byte{24, 10, 0, 0}, netip.MustParsePrefix("10.0.0.0/24"), true},
		{"/32", []byte{32, 192, 168, 1, 1}, netip.MustParsePrefix("192.168.1.1/32"), true},
		{"/25", []byte{25, 10, 0, 0, 128}, netip.MustParsePrefix("10.0.0.128/25"), true},
	}
	for _, tt := range tests {
		pfx, ok := NLRIToPrefix(fam, tt.nlri)
		require.Equal(t, tt.ok, ok, tt.name)
		assert.Equal(t, tt.want, pfx, tt.name)
	}
}

// TestNLRIToPrefix_IPv6 verifies IPv6 NLRI wire bytes convert to correct prefix.
//
// VALIDATES: NLRIToPrefix decodes IPv6 prefixes correctly.
// PREVENTS: IPv6 address bytes misaligned or truncated.
func TestNLRIToPrefix_IPv6(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast}
	nlri := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}
	pfx, ok := NLRIToPrefix(fam, nlri)
	require.True(t, ok)
	assert.Equal(t, netip.MustParsePrefix("2001:db8:1::/48"), pfx)
}

// TestNLRIToPrefix_Reject verifies malformed or unsupported NLRI is rejected.
//
// VALIDATES: NLRIToPrefix returns ok=false for empty, truncated, oversized, or non-IP families.
// PREVENTS: Panic on bad input, accepting malformed wire data.
func TestNLRIToPrefix_Reject(t *testing.T) {
	ipv4 := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}
	ipv6 := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast}
	l2vpn := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}

	tests := []struct {
		name string
		fam  family.Family
		nlri []byte
	}{
		{"empty", ipv4, nil},
		{"empty-slice", ipv4, []byte{}},
		{"ipv4-prefix-too-long", ipv4, []byte{33, 10, 0, 0, 0, 0}},
		{"ipv6-prefix-too-long", ipv6, append([]byte{129}, make([]byte, 17)...)},
		{"ipv4-truncated", ipv4, []byte{24, 10, 0}},
		{"non-ip-family", l2vpn, []byte{24, 10, 0, 0}},
	}
	for _, tt := range tests {
		_, ok := NLRIToPrefix(tt.fam, tt.nlri)
		assert.False(t, ok, tt.name)
	}
}

// TestPrefixToNLRI_RoundTrip verifies PrefixToNLRI output round-trips through NLRIToPrefix.
//
// VALIDATES: Encode then decode produces the original prefix.
// PREVENTS: Encode/decode asymmetry, wrong byte count math.
func TestPrefixToNLRI_RoundTrip(t *testing.T) {
	prefixes := []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/0"),
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParsePrefix("10.0.0.1/32"),
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("2001:db8:1::/48"),
		netip.MustParsePrefix("::1/128"),
	}

	for _, pfx := range prefixes {
		nlri := PrefixToNLRI(pfx)
		require.NotNil(t, nlri, "PrefixToNLRI(%v)", pfx)

		fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}
		if pfx.Addr().Is6() {
			fam.AFI = family.AFIIPv6
		}
		got, ok := NLRIToPrefix(fam, nlri)
		require.True(t, ok, "NLRIToPrefix round-trip failed for %v", pfx)
		assert.Equal(t, pfx, got, "round-trip mismatch")
	}
}

// TestPrefixToNLRIInto_BufferTooSmall verifies nil return on undersized buffer.
//
// VALIDATES: PrefixToNLRIInto returns nil when buffer is too small.
// PREVENTS: Buffer overrun on small buffers.
func TestPrefixToNLRIInto_BufferTooSmall(t *testing.T) {
	pfx := netip.MustParsePrefix("10.0.0.0/24")
	buf := make([]byte, 2)
	got := PrefixToNLRIInto(pfx, buf)
	assert.Nil(t, got)
}

// TestPrefixToNLRIInto_InvalidPrefix verifies nil return for zero-value prefix.
//
// VALIDATES: PrefixToNLRIInto rejects invalid prefix (Bits() == -1).
// PREVENTS: Encoding garbage for zero-value prefixes.
func TestPrefixToNLRIInto_InvalidPrefix(t *testing.T) {
	var pfx netip.Prefix
	buf := make([]byte, 17)
	got := PrefixToNLRIInto(pfx, buf)
	assert.Nil(t, got)
}

// TestPrefixToNLRIInto_ExactBuffer verifies encoding with minimum-sized buffer.
//
// VALIDATES: PrefixToNLRIInto works with exactly-sized buffer.
// PREVENTS: Off-by-one in needed-size calculation.
func TestPrefixToNLRIInto_ExactBuffer(t *testing.T) {
	pfx := netip.MustParsePrefix("10.0.0.0/24")
	buf := make([]byte, 4)
	got := PrefixToNLRIInto(pfx, buf)
	require.NotNil(t, got)
	assert.Equal(t, byte(24), got[0])
	assert.Equal(t, 4, len(got))
}

// TestPrefixToNLRI_IPv4_ByteValues verifies exact wire bytes for IPv4 prefixes.
//
// VALIDATES: Prefix length byte and address bytes match wire format.
// PREVENTS: Wrong byte order, wrong prefix-length encoding.
func TestPrefixToNLRI_IPv4_ByteValues(t *testing.T) {
	nlri := PrefixToNLRI(netip.MustParsePrefix("192.168.1.0/24"))
	require.NotNil(t, nlri)
	assert.Equal(t, []byte{24, 192, 168, 1}, nlri)
}

// TestPrefixToNLRI_IPv6_ByteValues verifies exact wire bytes for IPv6 prefixes.
//
// VALIDATES: IPv6 address bytes packed correctly in wire format.
// PREVENTS: IPv4/IPv6 branch confusion in encoding.
func TestPrefixToNLRI_IPv6_ByteValues(t *testing.T) {
	nlri := PrefixToNLRI(netip.MustParsePrefix("2001:db8::/32"))
	require.NotNil(t, nlri)
	assert.Equal(t, []byte{32, 0x20, 0x01, 0x0d, 0xb8}, nlri)
}
