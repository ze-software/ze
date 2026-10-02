package flowspecfirewall

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC8956ZeroLengthZeroOffsetMatchesEveryAddress receives IPv6 FlowSpec
// prefix components whose length and offset are both zero, and follows them
// into the firewall match Ze installs.
//
// VALIDATES: RFC 8956 Section 3.1 -- "If length = 0 and offset = 0, this
// component matches every address". The received component is accepted, and the
// firewall match it becomes is ::/0, which holds the first, the last and an
// ordinary IPv6 address.
// PREVENTS: a zero-length component refused as malformed, or translated into a
// match narrower than every address.
//
// RFC requirement: RFC8956-3.1-3 positive -- a received IPv6 Destination Prefix (wire 03 01 00 00) and Source Prefix (wire 03 02 00 00) with length 0 and offset 0 parse without error and translate to the firewall matches MatchDestinationAddress{::/0} and MatchSourceAddress{::/0}, which contain ::, 2001:db8::1 and ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff.
func TestRFC8956ZeroLengthZeroOffsetMatchesEveryAddress(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec}
	everyAddress := netip.MustParsePrefix("::/0")
	probes := []netip.Addr{
		netip.MustParseAddr("::"),
		netip.MustParseAddr("2001:db8::1"),
		netip.MustParseAddr("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff"),
	}

	for _, tc := range []struct {
		name string
		wire []byte
		want firewall.Match
	}{
		{"destination", []byte{0x03, 0x01, 0x00, 0x00}, firewall.MatchDestinationAddress{Prefix: everyAddress}},
		{"source", []byte{0x03, 0x02, 0x00, 0x00}, firewall.MatchSourceAddress{Prefix: everyAddress}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs, err := flowspec.ParseFlowSpec(flowspec.IPv6FlowSpec, tc.wire)
			require.NoError(t, err, "length 0 offset 0 is well formed")
			require.Len(t, fs.Components(), 1)

			matches, err := componentToMatch(fs.Components()[0], fam)
			require.NoError(t, err)
			require.Equal(t, []firewall.Match{tc.want}, matches)

			for _, addr := range probes {
				assert.True(t, everyAddress.Contains(addr), "%s is matched", addr)
			}
		})
	}
}
