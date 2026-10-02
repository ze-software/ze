// Design: docs/architecture/config/syntax.md -- the peer update block that originates routes
// RFC: rfc/short/rfc4659.md -- RFC4659-3.2-1, an advertised IPv6 VPN route carries its label
// Related: peers.go -- patchStaticRoutes, the label check under test

package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/vpn"
	"github.com/ze-software/ze/internal/component/config"
)

// rfc4659PeerWithVPNv6Route is a peer whose update block originates one IPv6
// VPN route, with the nlri line given by the caller.
func rfc4659PeerWithVPNv6Route(nlriLine string) string {
	return `
bgp {
    router-id 1.2.3.4;
    session {
        asn {
            local 65000
        }
    }
    peer mypeer {
        connection {
            remote {
                ip 2001:db8::2
            }
            local {
                ip auto
            }
        }
        session {
            asn {
                remote 65001
            }
        }
        update {
            attribute {
                origin igp;
                next-hop 2001:db8::1;
            }
            nlri {
                ` + nlriLine + `
            }
        }
    }
}
`
}

// rfc4659PeersFromConfig parses input with the YANG schema and converts the
// tree into peer settings, the path a configured route takes to the reactor.
func rfc4659PeersFromConfig(t *testing.T, input string) error {
	t.Helper()
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree, err := config.NewParser(schema).Parse(input)
	require.NoError(t, err, "the configuration must parse before the routes are converted")
	_, err = PeersFromConfigTree(tree)
	return err
}

// TestRFC4659ConfiguredVPNv6RouteKeepsItsLabel originates an IPv6 VPN route
// from the configuration with label 1000 and reads the static route the
// reactor will advertise.
//
// RFC 4659 Section 3.2: "When distributing IPv6 VPN routes, the advertising PE
// router MUST assign and distribute MPLS labels with the IPv6 VPN routes."
//
// RFC requirement: RFC4659-3.2-1 positive -- a configured ipv6/mpls-vpn route with rd
// 65000:100 and label 1000 becomes a static route for 2001:db8:1::/48 carrying exactly the
// label stack [1000].
func TestRFC4659ConfiguredVPNv6RouteKeepsItsLabel(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree, err := config.NewParser(schema).Parse(
		rfc4659PeerWithVPNv6Route("ipv6/mpls-vpn add rd 65000:100 label 1000 2001:db8:1::/48;"))
	require.NoError(t, err)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)
	require.Len(t, peers[0].StaticRoutes, 1)

	r := peers[0].StaticRoutes[0]
	assert.Equal(t, "2001:db8:1::/48", r.Prefix.String())
	assert.True(t, r.IsVPN(), "the route carries an RD, so it is advertised as a VPN route")
	assert.Equal(t, []uint32{1000}, r.Labels, "the configured label is the one advertised")
}

// TestRFC4659ConfiguredVPNv6RouteWithoutLabelRefused originates the same IPv6
// VPN route with no label, which the reactor would advertise with none.
//
// RFC requirement: RFC4659-3.2-1 negative -- a configured ipv6/mpls-vpn route carrying an
// RD and no label is refused at config load with "requires at least one label", so no
// labelless IPv6 VPN route reaches the reactor.
func TestRFC4659ConfiguredVPNv6RouteWithoutLabelRefused(t *testing.T) {
	err := rfc4659PeersFromConfig(t,
		rfc4659PeerWithVPNv6Route("ipv6/mpls-vpn add rd 65000:100 2001:db8:1::/48;"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "VPN route 2001:db8:1::/48 requires at least one label")
}
