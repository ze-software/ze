package bgpconfig

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/evpn"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

func configuredEVPN(t *testing.T, route, communities string) (*UpdateBlockRoutes, error) {
	t.Helper()
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	input := `bgp {
		session { asn { local 65000; } }
		peer mypeer {
			connection { remote { ip 192.0.2.1; } }
			session { asn { remote 65001; } }
			update {
				attribute { next-hop 192.0.2.2; ` + communities + ` }
				nlri { l2vpn/evpn add ` + route + `; }
			}
		}
	}`
	tree, err := config.NewParser(schema).Parse(input)
	require.NoError(t, err)
	return extractRoutesFromTree(tree.GetContainer("bgp").GetList("peer")["mypeer"])
}

// RFC requirement: RFC7432-8.2.1-9 positive -- native configuration produces the required zero-valued three-octet label field for per-ES Ethernet A-D.
// RFC requirement: RFC7432-8.2.1-10 positive -- native configuration retains the IPv4-address-specific RD on per-ES Ethernet A-D.
func TestConfiguredEVPNPerESRoute(t *testing.T) {
	routes, err := configuredEVPN(t,
		"ethernet-ad rd 192.0.2.1:7 esi 00:01:02:03:04:05:06:07:08:09 etag 4294967295",
		"extended-community [ target:65000:100 0x0601000000001000 ];")
	require.NoError(t, err)
	require.Len(t, routes.PluginRoutes, 1)
	route := routes.PluginRoutes[0]
	require.Equal(t, []byte{1, 25, 0, 1, 192, 0, 2, 1, 0, 7, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 255, 255, 255, 255, 0, 0, 0}, route.NLRI)
	for _, attr := range route.Attrs {
		if attr.Code == uint8(attribute.AttrExtCommunity) {
			require.Equal(t, []byte{0, 2, 0xfd, 0xe8, 0, 0, 0, 100, 6, 1, 0, 0, 0, 0, 0x10, 0}, attr.Value)
			return
		}
	}
	t.Fatal("configured per-ES route lost its Route Target and ESI Label")
}

// RFC requirement: RFC7432-8.2.1-9 negative -- native configuration cannot originate a nonzero per-ES NLRI label.
// RFC requirement: RFC7432-8.2.1-10 negative -- native configuration cannot originate per-ES Ethernet A-D with an AS-specific RD.
func TestConfiguredEVPNRefusesInvalidPerESRoute(t *testing.T) {
	for _, route := range []string{
		"ethernet-ad rd 192.0.2.1:7 etag 4294967295 label 100",
		"ethernet-ad rd 65000:7 etag 4294967295",
	} {
		_, err := configuredEVPN(t, route, "extended-community [ target:65000:100 0x0601000000001000 ];")
		require.Error(t, err)
	}
}

// RFC requirement: RFC7432-11.1-3 positive -- native configuration originates IMET with the configured Route Target, not the Route Origin subtype.
func TestConfiguredEVPNIMETOriginatorIsNotNextHop(t *testing.T) {
	routes, err := configuredEVPN(t, "multicast rd 65000:7 ip 198.51.100.1", "extended-community target:65000:100;")
	require.NoError(t, err)
	require.Len(t, routes.PluginRoutes, 1)
	route := routes.PluginRoutes[0]
	require.Equal(t, "192.0.2.2", route.NextHop)
	require.Equal(t, uint32(0xc6336401), binary.BigEndian.Uint32(route.NLRI[15:19]))
	for _, attr := range route.Attrs {
		if attr.Code == uint8(attribute.AttrExtCommunity) {
			require.Equal(t, []byte{0, 2, 0xfd, 0xe8, 0, 0, 0, 100}, attr.Value)
			return
		}
	}
	t.Fatal("configured IMET route lost its Route Target")
}

// RFC requirement: RFC7432-11.1-3 negative -- native configuration rejects IMET without a Route Target.
func TestConfiguredEVPNIMETRefusesMissingRouteTarget(t *testing.T) {
	for _, communities := range []string{"", "extended-community origin:65000:100;"} {
		_, err := configuredEVPN(t, "multicast rd 65000:7", communities)
		require.Error(t, err)
	}
}

// Ethernet Segment discovery needs its dedicated ES-Import community; neither
// an ordinary service RT nor an ESI Label can substitute for it.
func TestConfiguredEVPNEthernetSegmentRequirements(t *testing.T) {
	for _, input := range []struct {
		rd        string
		community string
	}{
		{"65000:7", "0x0602000102030405"},
		{"192.0.2.1:7", ""},
		{"192.0.2.1:7", "target:65000:100"},
		{"192.0.2.1:7", "0x0601000000001000"},
	} {
		communities := ""
		if input.community != "" {
			communities = "extended-community " + input.community + ";"
		}
		_, err := configuredEVPN(t, "ethernet-segment rd "+input.rd+" ip 198.51.100.1", communities)
		require.Error(t, err)
	}
	routes, err := configuredEVPN(t, "ethernet-segment rd 192.0.2.1:7 ip 198.51.100.1",
		"extended-community 0x0602000102030405;")
	require.NoError(t, err)
	require.Len(t, routes.PluginRoutes, 1)
	route, err := convertPluginRoute(routes.PluginRoutes[0])
	require.NoError(t, err)
	require.Equal(t, []byte{4, 23, 0, 1, 192, 0, 2, 1, 0, 7, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 32, 198, 51, 100, 1}, route.NLRI)
	require.Contains(t, route.RawAttrs, []byte{0xc0, 16, 8, 6, 2, 0, 1, 2, 3, 4, 5})
}
