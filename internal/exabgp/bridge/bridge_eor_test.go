package bridge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertRouteBareEORIsIPv4Unicast pins the family a bare `announce eor`
// names. ExaBGP's api_eor (src/exabgp/reactor/api/__init__.py) answers
// `Family(AFI.ipv4, SAFI.unicast)` when the line carries no family tokens, and
// refuses any token count other than none or two.
//
// VALIDATES: a bare `announce eor` is one IPv4 unicast End-of-RIB whatever
// families the bridge declared; `announce eor <afi> <safi>` is that family;
// one family token is refused.
// PREVENTS: the bridge expanding a bare EOR to every declared family, which
// sent markers ExaBGP never sends (api-eor sequence 3).
func TestConvertRouteBareEORIsIPv4Unicast(t *testing.T) {
	tr := Translator{Families: []string{"ipv4/unicast", "ipv6/unicast", "ipv4/flow"}}

	commands, matched, err := tr.convertRoute("peer 10.0.0.2", "announce eor")
	require.True(t, matched)
	require.NoError(t, err)
	assert.Equal(t, []Command{eorCommand("peer 10.0.0.2", "ipv4/unicast")}, commands)

	commands, matched, err = tr.convertRoute("peer 10.0.0.2", "announce eor ipv6 unicast")
	require.True(t, matched)
	require.NoError(t, err)
	assert.Equal(t, []Command{eorCommand("peer 10.0.0.2", "ipv6/unicast")}, commands)

	_, matched, err = tr.convertRoute("peer 10.0.0.2", "announce eor ipv6")
	require.True(t, matched)
	require.ErrorIs(t, err, errEORNeedsFamily)
}
