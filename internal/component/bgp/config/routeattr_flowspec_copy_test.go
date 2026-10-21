package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseExtendedCommunityCopyToNextHopAddress verifies the two meanings of
// `copy-to-nexthop`: followed by an address it is the IETF redirect-to-IP
// community with the copy bit set, and alone it stays the one-word 0x0800
// action with the copy bit.
//
// VALIDATES: `copy-to-nexthop 1.2.3.4` encodes 01 0C 01020304 0001 (the bytes
// ExaBGP 6.0 sends for `copy 1.2.3.4`), `copy-to-nexthop <IPv6>` encodes the
// 20-octet attribute 25 form with its last octet 01, and bare `copy-to-nexthop`
// still encodes 0800 000000000001, including when a further word follows it.
//
// PREVENTS: the copy flag of draft-ietf-idr-flowspec-redirect-ip-16 Section 2
// being unreachable from config, and the address form swallowing the word after
// a bare copy-to-nexthop.
func TestParseExtendedCommunityCopyToNextHopAddress(t *testing.T) {
	t.Parallel()

	ec, err := ParseExtendedCommunity("[ copy-to-nexthop 1.2.3.4 ]")
	require.NoError(t, err)
	assert.Equal(t, []byte{0x01, 0x0c, 0x01, 0x02, 0x03, 0x04, 0x00, 0x01}, ec.Bytes)
	assert.Empty(t, ec.IPv6Bytes)

	ec, err = ParseExtendedCommunity("[ copy-to-nexthop 2001:db8::1 ]")
	require.NoError(t, err)
	assert.Empty(t, ec.Bytes)
	assert.Equal(t, []byte{
		0x00, 0x0c,
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00, 0x01,
	}, ec.IPv6Bytes)

	ec, err = ParseExtendedCommunity("[ copy-to-nexthop ]")
	require.NoError(t, err)
	assert.Equal(t, []byte{0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}, ec.Bytes)

	ec, err = ParseExtendedCommunity("[ copy-to-nexthop target:65000:1 ]")
	require.NoError(t, err)
	assert.Len(t, ec.Bytes, 16, "bare copy-to-nexthop then a route target: two communities")
	assert.Contains(t, string(ec.Bytes), string([]byte{0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}))
}
