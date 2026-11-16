// Design: docs/architecture/config/syntax.md -- BGP TCP endpoints and next-hop selection
// Related: config_nexthop_form.go -- transport parsing is independent of the emitted next-hop form

package reactor

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/family"
)

// TestConfiguredLinkLocalTransportRetainsScope drives the config-file-to-peer
// entry point with a link-local TCP source. The endpoint remains usable under
// every configured next-hop mode, and the parser retains both interface zones.
func TestConfiguredLinkLocalTransportRetainsScope(t *testing.T) {
	for _, localIP := range []string{"fe80::1", "fe80::1%if0"} {
		for _, mode := range []struct {
			value string
			want  uint8
		}{
			{"unchanged", NextHopUnchanged},
			{"self", NextHopSelf},
			{"auto", NextHopAuto},
			{"2001:db8::9", NextHopExplicit},
		} {
			t.Run(localIP+"/"+mode.value, func(t *testing.T) {
				settings := configuredLinkLocalTransport(t, localIP, mode.value)
				assert.Equal(t, netip.MustParseAddr(localIP), settings.LocalAddress)
				assert.Equal(t, netip.MustParseAddr("fe80::2%if0"), settings.Address)
				assert.Equal(t, mode.want, settings.NextHopMode)
				assert.Equal(t, netip.MustParseAddrPort("[fe80::2%if0]:179"), settings.PeerKey())
				session := NewSession(settings)
				assert.Same(t, settings, session.settings)
				assert.Equal(t, netip.MustParseAddr(localIP), session.settings.LocalAddress)
			})
		}
	}
}

// TestConfiguredLinkLocalTransportSelfNeedsCapability keeps admission at the
// point that resolves a route's actual next hop. Accepting the transport does
// not permit a link-local-only IPv6 next hop without negotiated capability 77.
func TestConfiguredLinkLocalTransportSelfNeedsCapability(t *testing.T) {
	for _, negotiated := range []bool{false, true} {
		t.Run(fmt.Sprintf("capability77=%v", negotiated), func(t *testing.T) {
			settings := configuredLinkLocalTransport(t, "fe80::1%if0", "self")
			peer := NewPeer(settings)
			peer.negotiated.Store(&NegotiatedCapabilities{LinkLocalNextHop: negotiated})
			session := NewSession(settings)
			session.transport.Store(&sessionTransport{local: settings.LocalAddress})
			addr, err := peer.resolveNextHop(session, bgptypes.NewNextHopSelf(), family.IPv6Unicast)
			if negotiated {
				require.NoError(t, err)
				assert.Equal(t, netip.MustParseAddr("fe80::1%if0"), addr)
				return
			}
			require.ErrorIs(t, err, ErrNextHopLinkLocalOnly)
			assert.False(t, addr.IsValid(), "a refused next hop cannot be emitted")
		})
	}
}

// TestParsePeerFromTreeRefusesMalformedLocalTransport preserves syntax
// validation when the local TCP endpoint stops using the global-next-hop guard.
func TestParsePeerFromTreeRefusesMalformedLocalTransport(t *testing.T) {
	_, err := parsePeerFromTree("scoped", peerTreeWith("not-an-ip", "unchanged"), 65000, 0x0a000001)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid local ip")
}

func configuredLinkLocalTransport(t *testing.T, localIP, nextHop string) *PeerSettings {
	t.Helper()
	tree, err := config.ParseTreeForValidation(fmt.Sprintf(`bgp {
    router-id 1.2.3.4;
    peer scoped {
        connection {
            local { ip %s; }
            remote { ip fe80::2%%if0; }
        }
        session {
            asn { local 65000; remote 65001; }
            next-hop %s;
        }
    }
}`, localIP, nextHop))
	require.NoError(t, err)
	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp)
	peers, err := PeersFromTree(bgp.ToMap())
	require.NoError(t, err)
	require.Len(t, peers, 1)
	return peers[0]
}
