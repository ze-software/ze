// Design: docs/architecture/config/syntax.md — route attribute parsing
// RFC: rfc/short/rfc8956.md — the IPv6 route-target redirect (Section 6.1)

package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseExtendedCommunityRedirectIPv6 checks that a config
// `extended-community [redirect:[<IPv6>]:<value>]` writes the 20-octet RFC 8956
// Section 6.1 community into IPv6Bytes (attribute 25), not the 8-octet list,
// and that a value too wide for the 2-octet local administrator is refused.
func TestParseExtendedCommunityRedirectIPv6(t *testing.T) {
	t.Parallel()

	ec, err := ParseExtendedCommunity("[ redirect:[2001:db8::1]:100 redirect:65000:7 ]")
	require.NoError(t, err)
	assert.Equal(t, []byte{0x80, 0x08, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x07}, ec.Bytes)
	assert.Equal(t, []byte{
		0x00, 0x0d,
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00, 0x64,
	}, ec.IPv6Bytes)

	_, err = ParseExtendedCommunity("[ redirect:[2001:db8::1]:70000 ]")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "0-65535")
}
