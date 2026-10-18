// Design: docs/architecture/api/update-syntax.md -- SRv6 Prefix-SID origination
// Related: update_text.go -- ParseUpdateText, where the attribute meets its family
// RFC 9252 Section 7 -- see rfc/short/rfc9252.md.

package update

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// TestUpdateTextSRv6TranspositionNeedsALabelField drives `update text` with an
// SRv6 SID Structure that declares a transposition, once per section family.
//
// VALIDATES: the structure is refused with an IPv4 unicast `nlri` section and an
// IPv6 unicast `mp` section, naming the family and the corrected structure, and
// accepted with an IPv6 VPN section, whose NLRI carries the label field. The same
// command with a zero transposition is accepted on IPv6 unicast.
// PREVENTS: the encoder's family-free checks passing a transposition onto a
// global IPv6 service route.
//
// RFC requirement: RFC9252-3.2.1-2 positive -- through update text, a SID Structure with transposition offset and length 0 is accepted with an IPv6 unicast route, and a non-zero transposition is accepted with an IPv6 VPN route
// RFC requirement: RFC9252-3.2.1-2 negative -- through update text, a SID Structure with a non-zero transposition is refused with an IPv4 unicast nlri section and an IPv6 unicast mp section.
func TestUpdateTextSRv6TranspositionNeedsALabelField(t *testing.T) {
	sid := func(structure string) []string {
		return []string{"bgp-prefix-sid-srv6", "(", "l3-service", "2001:db8::", "0x13", structure, ")"}
	}
	transposed := "[64,24,16,0,16,64]"
	vpn6 := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}.String()

	for name, section := range map[string][]string{
		"ipv4 unicast": {"next-hop", "192.0.2.1", "nlri", "add", "10.0.1.0/24"},
		"ipv6 unicast": {"next-hop", "2001:db8::1", "mp", family.IPv6Unicast.String(), "add", "2001:db8:1::/48"},
	} {
		_, err := ParseUpdateText(append(sid(transposed), section...))
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), "[64,24,16,0,0,0]", name)
		assert.Contains(t, err.Error(), "RFC 9252 Section 7", name)
	}

	result, err := ParseUpdateText(append(sid("[64,24,16,0,0,0]"), "next-hop", "2001:db8::1", "mp", family.IPv6Unicast.String(), "add", "2001:db8:1::/48"))
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	result, err = ParseUpdateText(append(sid(transposed), "next-hop", "2001:db8::1", "mp", vpn6, "rd", "65000:1", "label", "100", "add", "2001:db8:1::/48"))
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
}
