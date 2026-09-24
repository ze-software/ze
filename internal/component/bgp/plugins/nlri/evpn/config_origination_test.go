// Design: docs/architecture/wire/nlri-evpn.md -- explicit local EVPN origination

package evpn

import (
	"bytes"
	"encoding/hex"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestEVPNConfigOriginatingAddressAndCommunities exercises the registered native
// parser with next-hop self, whose transport address is resolved only at send.
func TestEVPNConfigOriginatingAddressAndCommunities(t *testing.T) {
	parser := registry.ConfigRouteParserByFamily("l2vpn/evpn")
	require.NotNil(t, parser)
	request := registry.ConfigRouteRequest{
		Content: strings.Fields("multicast rd 65000:7 ip 192.0.2.1"), NextHop: "self",
		ExtCommunity: []byte{0, 2, 0xfd, 0xe8, 0, 0, 0, 100},
	}
	route, err := parser(request)
	require.NoError(t, err)
	decoded, rest, err := ParseEVPN(route.NLRI, false)
	require.NoError(t, err)
	require.Empty(t, rest)
	type3, ok := decoded.(*EVPNType3)
	require.True(t, ok, "decoded route is %T", decoded)
	require.Equal(t, netip.MustParseAddr("192.0.2.1"), type3.OriginatorIP())
	request.Content = strings.Fields("multicast rd 65000:7")
	_, err = parser(request)
	require.Error(t, err, "next-hop self does not provide an originating router address at parse time")
	request.Content = strings.Fields("multicast rd 65000:7 ip 192.0.2.1")
	request.ExtCommunity = nil
	_, err = parser(request)
	require.Error(t, err, "native IMET origination requires its explicit Route Target")
}

// TestEVPNEncodersRejectIncompleteTypedRoutes checks fields whose absence or
// wrong address family would otherwise panic while writing the route bytes.
func TestEVPNEncodersRejectIncompleteTypedRoutes(t *testing.T) {
	for _, command := range []string{
		"type3 rd 65000:7",
		"type4 rd 65000:7",
		"type5 rd 65000:7",
		"type5 rd 65000:7 prefix 192.0.2.0/24 gateway 2001:db8::1",
		"type5 rd 65000:7 prefix 192.0.2.0/24 label 1 label 2",
		"type2 rd 65000:7 mac 00:11:22:33:44:55 label 1048576",
	} {
		t.Run(command, func(t *testing.T) {
			encoded, err := EncodeNLRIHex("l2vpn/evpn", strings.Fields(command))
			require.Error(t, err)
			require.Empty(t, encoded)
		})
	}
	_, _, err := EncodeRoute("ip-prefix rd 65000:7 prefix 192.0.2.0/24 gateway 2001:db8::1 next-hop 192.0.2.1", "l2vpn/evpn", 65000, true, true, false)
	require.Error(t, err)
}

// TestEVPNRequiredZeroLabelFields checks that omitted label values retain the
// mandatory field and produce a route the public decoder can consume whole.
func TestEVPNRequiredZeroLabelFields(t *testing.T) {
	for _, command := range []string{
		"mac-ip rd 65000:7 mac 00:11:22:33:44:55 next-hop 192.0.2.1",
		"ip-prefix rd 65000:7 prefix 192.0.2.129/24 next-hop 192.0.2.1",
	} {
		t.Run(command, func(t *testing.T) {
			_, encoded, err := EncodeRoute(command, "l2vpn/evpn", 65000, true, true, false)
			require.NoError(t, err)
			decoded, rest, err := ParseEVPN(encoded, false)
			require.NoError(t, err)
			require.Empty(t, rest)
			switch route := decoded.(type) {
			case *EVPNType2:
				require.Equal(t, []uint32{0}, route.Labels())
			case *EVPNType5:
				require.Equal(t, []uint32{0}, route.Labels())
				require.Equal(t, netip.MustParsePrefix("192.0.2.0/24"), route.Prefix())
			default:
				t.Fatalf("unexpected route type %T", decoded)
			}
		})
	}
}

// TestEVPNPrefixOmittedGatewayClearsScratch checks caller-owned wire buffers:
// a previous route's bytes cannot become the omitted gateway of a new route.
func TestEVPNPrefixOmittedGatewayClearsScratch(t *testing.T) {
	for _, prefix := range []string{"192.0.2.1/24", "2001:db8::1/64"} {
		t.Run(prefix, func(t *testing.T) {
			encoded, err := EncodeNLRIHex("l2vpn/evpn", strings.Fields("type5 rd 65000:7 prefix "+prefix))
			require.NoError(t, err)
			wire, err := hex.DecodeString(encoded)
			require.NoError(t, err)
			decoded, rest, err := ParseEVPN(wire, false)
			require.NoError(t, err)
			require.Empty(t, rest)
			route, ok := decoded.(*EVPNType5)
			require.True(t, ok, "decoded route is %T", decoded)
			route.gateway = netip.Addr{}
			scratch := bytes.Repeat([]byte{0xff}, route.Len())
			require.Equal(t, len(scratch), route.WriteTo(scratch, 0))
			require.Equal(t, wire, scratch)
		})
	}
}
