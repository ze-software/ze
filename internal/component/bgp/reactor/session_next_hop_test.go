// Design: docs/architecture/route-selection.md -- received NEXT_HOP semantics
package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// RFC requirement: RFC4271-6.3-11 positive -- an invalid legacy NEXT_HOP is withheld while explicit withdrawals and independently valid MP_REACH routes survive.
// RFC requirement: RFC4271-6.3-11 negative -- after an interface-address snapshot removes the conflicting local address, the same next hop becomes usable without restarting the session.
func TestSessionRFC4271NextHopMixedUpdateAndAddressChange(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Capabilities = []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
	}
	s, client := firstASSession(t, settings, settings.Capabilities, nil)
	oldAddresses := []netip.Prefix{netip.MustParsePrefix("192.0.2.2/24"), netip.MustParsePrefix("198.51.100.1/24")}
	s.nextHopScope.Store(newReceiveNextHopScope(oldAddresses, client, settings))
	mpNextHop := netip.MustParseAddr("2001:db8::1").As16()
	mpValue := append([]byte{0, 2, 1, 16}, mpNextHop[:]...)
	mpValue = append(mpValue, 0, 64, 0x20, 0x01, 0x0d, 0xb8, 0, 1, 0, 0)
	attrs := firstASAttrs(2, 65002)
	copy(attrs[len(attrs)-4:], []byte{198, 51, 100, 1})
	attrs = append(attrs, collapseAttr(0x80, byte(attribute.AttrMPReachNLRI), mpValue)...)
	withdrawn := []byte{24, 198, 18, 0}
	announced := []byte{24, 203, 0, 113}
	body := makeUpdateBody(withdrawn, attrs, announced)
	for _, invalid := range []bool{true, false} {
		if !invalid {
			// The same subnet stays attached, but this address is no longer ours.
			s.nextHopScope.Store(newReceiveNextHopScope([]netip.Prefix{
				netip.MustParsePrefix("192.0.2.2/24"), netip.MustParsePrefix("198.51.100.2/24"),
			}, client, settings))
		}
		wu := firstASReceive(t, s, client, body)
		nlri, err := wu.NLRI()
		require.NoError(t, err)
		wantWithdrawn := withdrawn
		if invalid {
			require.Empty(t, nlri)
			wantWithdrawn = append(append([]byte{}, withdrawn...), announced...)
		} else {
			require.Equal(t, announced, nlri)
		}
		length := int(binary.BigEndian.Uint16(wu.Payload()[:2]))
		require.Equal(t, wantWithdrawn, wu.Payload()[2:2+length])
		gotAttrs, err := wu.Attrs()
		require.NoError(t, err)
		mp, err := gotAttrs.GetRaw(attribute.AttrMPReachNLRI)
		require.NoError(t, err)
		require.Equal(t, mpValue, mp)
	}
}

// RFC requirement: RFC4271-6.3-11 positive -- iBGP still rejects a next hop belonging to the receiver.
// RFC requirement: RFC4271-6.3-11 negative -- iBGP permits an off-link next hop.
func TestSessionRFC4271IBGPNextHop(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65001, 0x01020301)
	s, client := firstASSession(t, settings, nil, nil)
	s.nextHopScope.Store(newReceiveNextHopScope([]netip.Prefix{netip.MustParsePrefix("192.0.2.2/24")}, client, settings))
	prefix := []byte{24, 203, 0, 113}
	attrs := firstASAttrs(2, 65001)
	copy(attrs[len(attrs)-4:], []byte{198, 51, 100, 1})
	good := append(append([]byte{}, attrs...), collapseAttr(0x40, byte(attribute.AttrLocalPref), []byte{0, 0, 0, 100})...)
	accepted := firstASReceive(t, s, client, makeUpdateBody(nil, good, prefix))
	nlri, err := accepted.NLRI()
	require.NoError(t, err)
	require.Equal(t, prefix, nlri, "iBGP permits an off-link next hop")
	copy(good[len(attrs)-4:len(attrs)], []byte{192, 0, 2, 2})
	withdrawal := firstASReceive(t, s, client, makeUpdateBody(nil, good, prefix))
	nlri, err = withdrawal.NLRI()
	require.NoError(t, err)
	require.Empty(t, nlri)
	length := int(binary.BigEndian.Uint16(withdrawal.Payload()[:2]))
	require.Equal(t, prefix, withdrawal.Payload()[2:2+length])
}

// A sender on the receiving host is zero IP hops away, so RFC 4271 Section 6.3's
// one-hop common-subnet condition does not bind it, while the receiving
// speaker's own address stays invalid. A one-hop sender on a connected subnet
// keeps the condition: an off-link next hop from it is still withdrawn.
func TestSessionNextHopSameHostSender(t *testing.T) {
	prefix := []byte{24, 203, 0, 113}
	for _, tc := range []struct {
		name    string
		remote  string
		nextHop string
		invalid bool
	}{
		{"loopback sender, off-link next hop", "127.0.0.1", "1.1.1.1", false},
		{"loopback sender, receiver address", "127.0.0.1", "127.0.0.1", true},
		{"sender at own address, off-link next hop", "192.0.2.2", "1.1.1.1", false},
		{"one-hop sender, off-link next hop", "192.0.2.1", "1.1.1.1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr(tc.remote), 65001, 65002, 0x01020301)
			s, client := firstASSession(t, settings, nil, nil)
			s.nextHopScope.Store(newReceiveNextHopScope([]netip.Prefix{
				netip.MustParsePrefix("127.0.0.1/8"), netip.MustParsePrefix("192.0.2.2/24"),
			}, client, settings))
			attrs := append([]byte{}, firstASAttrs(2, 65002)...)
			addr := netip.MustParseAddr(tc.nextHop).As4()
			copy(attrs[len(attrs)-4:], addr[:])
			wu := firstASReceive(t, s, client, makeUpdateBody(nil, attrs, prefix))
			nlri, err := wu.NLRI()
			require.NoError(t, err)
			if tc.invalid {
				require.Empty(t, nlri)
				require.Equal(t, prefix, wu.Payload()[2:6])
				return
			}
			require.Equal(t, prefix, nlri)
		})
	}
}
