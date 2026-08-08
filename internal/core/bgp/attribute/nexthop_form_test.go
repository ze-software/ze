package attribute

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateGlobalNextHop covers the address forms that can and cannot occupy
// the Network Address of Next Hop field of an MP_REACH_NLRI attribute.
//
// RFC 2545 Section 3: "A BGP speaker shall advertise to its peer in the Network
// Address of Next Hop field the global IPv6 address of the next hop, potentially
// followed by the link-local IPv6 address of the next hop." Section 2 splits IPv6
// unicast addresses into "link-local" and "global"/"non-link-local", and states
// that a link-local address is "not ... well suited to be used as next hop
// attributes in BGP-4".
//
// VALIDATES: an IPv6 link-local unicast address is refused; every non-link-local
// address, and every IPv4 address, is accepted.
func TestValidateGlobalNextHop(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{"ipv6 global", "2001:db8::1", false},
		{"ipv6 unique local", "fd00::1", false},
		{"ipv6 loopback", "::1", false},
		{"ipv6 link-local", "fe80::cafe", true},
		{"ipv6 link-local low", "fe80::", true},
		{"ipv6 link-local high", "febf:ffff:ffff:ffff:ffff:ffff:ffff:ffff", true},
		{"ipv6 just above link-local range", "fec0::1", false},
		{"ipv4", "192.0.2.1", false},
		{"ipv4 link-local", "169.254.0.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGlobalNextHop(netip.MustParseAddr(tt.addr))
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrLinkLocalNextHop)
				assert.Contains(t, err.Error(), tt.addr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

// TestValidateGlobalNextHopInvalidAddress checks the zero value.
//
// VALIDATES: an unset address is not reported as a link-local violation; the
// caller's own parse step owns that failure.
func TestValidateGlobalNextHopInvalidAddress(t *testing.T) {
	assert.NoError(t, ValidateGlobalNextHop(netip.Addr{}))
}
