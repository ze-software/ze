//go:build ze_bgp

package bridge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/evpn" // blank import: registers l2vpn/evpn
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"   // blank import: registers bgp-ls/bgp-ls
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/vpls" // blank import: registers l2vpn/vpls
)

// TestControlLineFamilyFromRegistry checks that `announce eor` and
// `announce route-refresh` take every family ze carries.
//
// VALIDATES: an ExaBGP 4.2 or 5.0 script's `announce eor l2vpn vpls` and
// `announce route-refresh l2vpn evpn` / `bgp-ls bgp-ls` (qa/old/commands-5.0.json)
// translate to the ze command for that family, and a pair ze does not carry is
// refused naming the families it does.
// PREVENTS: the family check drifting from the registry again; until
// 2026-10-22 a hand-written AFI set refused bgp-ls, and the route SAFI map
// refused evpn and vpls.
func TestControlLineFamilyFromRegistry(t *testing.T) {
	translated := []struct{ line, want string }{
		{"announce eor l2vpn vpls", "send bgp * update text mp l2vpn/vpls eor"},
		{"announce eor l2vpn evpn", "send bgp * update text mp l2vpn/evpn eor"},
		{"announce route-refresh l2vpn evpn", "request peer * refresh l2vpn/evpn"},
		{"announce route-refresh bgp-ls bgp-ls", "request peer * refresh bgp-ls/bgp-ls"},
		{"announce route-refresh ipv4 nlri-mpls", "request peer * refresh ipv4/mpls-label"},
	}
	for _, tc := range translated {
		t.Run(tc.line, func(t *testing.T) {
			translation, err := TranslateLine(tc.line)
			require.NoError(t, err)
			assert.Equal(t, tc.want, onlyCommand(translation))
		})
	}

	_, err := TranslateLine("announce route-refresh l2vpn nonsense")
	require.ErrorIs(t, err, errRouteRefreshNeedsFamily)
	assert.Contains(t, err.Error(), `"nonsense" names no subsequent address family ze carries for l2vpn`)
	assert.Contains(t, err.Error(), "l2vpn evpn")

	_, err = TranslateLine("announce eor ipx unicast")
	require.ErrorIs(t, err, errEORNeedsFamily)
	assert.Contains(t, err.Error(), `"ipx" names no address family ze carries`)
	assert.Contains(t, err.Error(), "bgp-ls")
}
