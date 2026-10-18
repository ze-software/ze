// Design: docs/architecture/wire/nlri-evpn.md -- explicit EVPN route origination
// RFC: rfc/short/rfc7432.md

package evpn

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

const perESRedundancyRoute = "ethernet-ad rd 192.0.2.1:7 esi 00:01:02:03:04:05:06:07:08:09 etag 4294967295 next-hop 192.0.2.1 extended-community target:65000:100 "

// VALIDATES: the route command's redundancy-mode keyword reaches the UPDATE:
// a per-ES route whose ESI Label agrees with the mode is sent with that ESI
// Label byte for byte.
// PREVENTS: the keyword being parsed and then dropped before the check.
func TestOriginPerESRedundancyModeAccepted(t *testing.T) {
	cases := []struct {
		args     string
		esiLabel []byte
	}{
		// RFC requirement: RFC7432-8.2.1-4 positive -- redundancy-mode all-active originates an ESI Label with flag 0 and ESI label 256 (06 01 00 0000 001000).
		{"redundancy-mode all-active extended-community 0x0601000000001000", []byte{0x06, 0x01, 0x00, 0, 0, 0x00, 0x10, 0x00}},
		// RFC requirement: RFC7432-8.2.1-7 positive -- redundancy-mode single-active originates an ESI Label with flag 1 (06 01 01 0000 001000).
		{"redundancy-mode single-active extended-community 0x0601010000001000", []byte{0x06, 0x01, 0x01, 0, 0, 0x00, 0x10, 0x00}},
	}
	for _, tc := range cases {
		frame, _, err := EncodeRoute(perESRedundancyRoute+tc.args, "l2vpn/evpn", 65000, true, true, false)
		require.NoError(t, err, tc.args)
		update, err := message.UnpackUpdate(frame[message.HeaderLen:])
		require.NoError(t, err)
		_, _, communities, found := attribute.AttrFind(update.PathAttributes, attribute.AttrExtCommunity)
		require.True(t, found)
		require.Equal(t, append([]byte{0x00, 0x02, 0xfd, 0xe8, 0, 0, 0, 100}, tc.esiLabel...), communities, tc.args)
	}
}

// VALIDATES: the route command refuses an ESI Label that contradicts the
// requested redundancy mode, an All-Active ESI label from the reserved range,
// and a redundancy-mode on a route that carries no per-ES ESI Label.
// PREVENTS: an originated per-ES route signaling a mode the operator did not ask for.
func TestOriginPerESRedundancyModeRefused(t *testing.T) {
	cases := []struct {
		route   string
		message string
	}{
		// RFC requirement: RFC7432-8.2.1-4 negative -- redundancy-mode all-active refuses flag 1 and a reserved ESI label 0.
		{perESRedundancyRoute + "redundancy-mode all-active extended-community 0x0601010000001000", "redundancy-mode all-active needs the ESI Label Single-Active flag at 0"},
		{perESRedundancyRoute + "redundancy-mode all-active extended-community 0x0601000000000000", "carries label 0, a reserved value"},
		// RFC requirement: RFC7432-8.2.1-7 negative -- redundancy-mode single-active refuses an ESI Label with flag 0.
		{perESRedundancyRoute + "redundancy-mode single-active extended-community 0x0601000000001000", "redundancy-mode single-active needs the ESI Label Single-Active flag at 1"},
		{perESRedundancyRoute + "redundancy-mode active-standby extended-community 0x0601000000001000", `invalid redundancy-mode "active-standby": use all-active or single-active`},
		{"ethernet-ad rd 65000:7 etag 100 label 100 next-hop 192.0.2.1 redundancy-mode all-active", "redundancy-mode applies only to an Ethernet A-D per ES route, whose Ethernet Tag is MAX-ET"},
		{"multicast rd 65000:7 next-hop 192.0.2.1 extended-community target:65000:100 redundancy-mode all-active", "redundancy-mode applies only to an Ethernet A-D per ES route;"},
	}
	for _, tc := range cases {
		frame, _, err := EncodeRoute(tc.route, "l2vpn/evpn", 65000, true, true, false)
		require.ErrorContains(t, err, tc.message, tc.route)
		require.Nil(t, frame)
	}
}

// VALIDATES: the native config path hands the configured redundancy-mode to the
// mode check, so an `update` block whose ESI Label contradicts its
// redundancy-mode is refused before any route is built.
// PREVENTS: parseConfigRoute dropping the configured mode and letting the ESI
// Label flag alone decide, which would accept both contradictions below.
func TestConfiguredEVPNRefusesPerESRedundancyMismatch(t *testing.T) {
	parser := registry.ConfigRouteParserByFamily("l2vpn/evpn")
	require.NotNil(t, parser)
	const perESRoute = "ethernet-ad rd 192.0.2.1:7 esi 00:01:02:03:04:05:06:07:08:09 etag 4294967295 redundancy-mode "
	target := []byte{0x00, 0x02, 0xfd, 0xe8, 0, 0, 0, 100}
	cases := []struct {
		mode     string
		esiLabel []byte
		message  string
	}{
		// RFC requirement: RFC7432-8.2.1-4 negative -- a configured redundancy-mode all-active refuses an ESI Label with flag 1.
		{"all-active", []byte{0x06, 0x01, 0x01, 0, 0, 0x00, 0x10, 0x00}, "redundancy-mode all-active needs the ESI Label Single-Active flag at 0"},
		// RFC requirement: RFC7432-8.2.1-7 negative -- a configured redundancy-mode single-active refuses an ESI Label with flag 0.
		{"single-active", []byte{0x06, 0x01, 0x00, 0, 0, 0x00, 0x10, 0x00}, "redundancy-mode single-active needs the ESI Label Single-Active flag at 1"},
	}
	for _, tc := range cases {
		request := registry.ConfigRouteRequest{
			Content:      strings.Fields(perESRoute + tc.mode),
			NextHop:      "192.0.2.1",
			ExtCommunity: append(append([]byte{}, target...), tc.esiLabel...),
		}
		route, err := parser(request)
		require.ErrorContains(t, err, tc.message, tc.mode)
		require.Empty(t, route.NLRI, tc.mode)
	}
}
