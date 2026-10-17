package bridge

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBridgeWritesTheSectionKeyword pins the UPDATE section keyword the bridge
// writes for the two route forms whose section is not the family's default.
//
// VALIDATES: IPv4 unicast announced with an IPv6 next hop goes out as
// "mp ipv4/unicast add" (RFC 8950: MP_REACH_NLRI carries it), while its
// withdrawal stays "nlri del" (RFC 4271 Section 4.3: Withdrawn Routes); an SR
// Policy announce and withdraw go out as "mp <afi>/sr-policy add|del".
// PREVENTS: an announce ze refuses ("nlri add" with an IPv6 next hop, or the
// removed "nlri <family>" form), which reaches the script as a failed command
// and the peer as no route.
func TestBridgeWritesTheSectionKeyword(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			"ipv4_unicast_ipv6_next_hop_announce",
			"neighbor 10.0.0.1 announce route 10.0.0.0/24 next-hop 2001:db8::1",
			"send bgp 10.0.0.1 update text nhop 2001:db8::1 mp ipv4/unicast add 10.0.0.0/24",
		},
		{
			"ipv4_unicast_ipv6_next_hop_withdraw",
			"neighbor 10.0.0.1 withdraw route 10.0.0.0/24 next-hop 2001:db8::1",
			"send bgp 10.0.0.1 update text nhop 2001:db8::1 nlri del 10.0.0.0/24",
		},
		{
			"sr_policy_announce",
			"neighbor 10.0.0.1 announce ipv4 sr-policy distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 1.2.3.4 preference 100",
			"send bgp 10.0.0.1 update text nhop 1.2.3.4 mp ipv4/sr-policy add distinguisher 0 color 100 endpoint 10.0.0.1 preference 100",
		},
		{
			"sr_policy_withdraw",
			"neighbor 10.0.0.1 withdraw ipv4 sr-policy distinguisher 0 color 100 endpoint 10.0.0.1",
			"send bgp 10.0.0.1 update text mp ipv4/sr-policy del distinguisher 0 color 100 endpoint 10.0.0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, translatedCommand(t, tt.line))
		})
	}
}
