// Overview: peer_initial_sync.go — the plugin-route rail these tests drive

package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// TestPluginRouteAddPathWritesPathID checks the NLRI field the plugin-route rail
// sends with and without ADD-PATH send negotiated for the route's family.
//
// Method: build a peer whose plugin routes are one ungrouped ipv4/sr-policy
// route and two grouped ipv4/mvpn routes, store a send context with ADD-PATH on
// or off, run sendPluginRoutesVia and read MP_REACH_NLRI out of each UPDATE.
//
// VALIDATES: RFC 7911 Section 3: every NLRI carries its 4-octet Path Identifier
// when ADD-PATH send is negotiated, including each NLRI of a grouped UPDATE; the
// identifier is the route's configured path-information, 0 when none is set.
// PREVENTS: a Ze receiver with ADD-PATH receive reading NLRI octets as the Path
// Identifier and resetting the session on every UPDATE.
func TestPluginRouteAddPathWritesPathID(t *testing.T) {
	srPolicy, ok := family.LookupFamily("ipv4/sr-policy")
	require.True(t, ok, "ipv4/sr-policy must be registered")
	mvpn, ok := family.LookupFamily("ipv4/mvpn")
	require.True(t, ok, "ipv4/mvpn must be registered")

	srNLRI := mustHex(t, "6000000000000000640a000001")
	mvpnA := []byte{0x01, 0x05, 0xAA, 0xAA, 0xAA, 0xAA, 0x01}
	mvpnB := []byte{0x01, 0x05, 0xBB, 0xBB, 0xBB, 0xBB, 0x02}

	tests := []struct {
		name    string
		addPath bool
		wantSR  []byte
		wantMV  []byte
	}{
		{"without add-path", false, srNLRI, concatPluginNLRIs(mvpnA, mvpnB)},
		{"with add-path", true,
			concatPluginNLRIs(mustHex(t, "00000007"), srNLRI),
			concatPluginNLRIs(mustHex(t, "00000000"), mvpnA, mustHex(t, "00000000"), mvpnB)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &PeerSettings{
				Connection: ConnectionBoth,
				Address:    netip.MustParseAddr("10.0.0.2"),
				LocalAS:    65000,
				PeerAS:     65001,
				RouterID:   0x01020301,
				PluginRoutes: []PluginRoute{
					{Family: "ipv4/sr-policy", NLRI: srNLRI, NextHop: netip.MustParseAddr("10.0.0.1"), PathID: 7},
					{Family: "ipv4/mvpn", NLRI: mvpnA, NextHop: netip.MustParseAddr("10.0.0.1"), Group: true},
					{Family: "ipv4/mvpn", NLRI: mvpnB, NextHop: netip.MustParseAddr("10.0.0.1"), Group: true},
				},
			}
			peer := NewPeer(settings)
			peer.state.Store(int32(PeerStateEstablished))
			peer.negotiated.Store(&NegotiatedCapabilities{
				families: map[family.Family]bool{srPolicy: true, mvpn: true},
			})
			peer.sendCtx.Store(bgpctx.EncodingContextWithAddPath(true,
				map[family.Family]bool{srPolicy: tt.addPath, mvpn: tt.addPath}))

			got := map[uint8][]byte{}
			peer.sendPluginRoutesVia(nil, func(u *message.Update) error {
				safi, nlri := pluginMPReachNLRI(t, u.PathAttributes)
				got[safi] = append([]byte(nil), nlri...)
				return nil
			})
			if !bytes.Equal(got[uint8(srPolicy.SAFI)], tt.wantSR) {
				t.Errorf("sr-policy NLRI = %x, want %x", got[uint8(srPolicy.SAFI)], tt.wantSR)
			}
			if !bytes.Equal(got[uint8(mvpn.SAFI)], tt.wantMV) {
				t.Errorf("mvpn NLRI = %x, want %x", got[uint8(mvpn.SAFI)], tt.wantMV)
			}
		})
	}
}

// pluginMPReachNLRI returns the SAFI and NLRI field of the MP_REACH_NLRI
// attribute in packed path attributes.
func pluginMPReachNLRI(t *testing.T, attrs []byte) (uint8, []byte) {
	t.Helper()
	for len(attrs) >= 3 {
		flags, code := attrs[0], attrs[1]
		hdr, valueOctets := 3, int(attrs[2])
		if flags&0x10 != 0 {
			hdr, valueOctets = 4, int(attrs[2])<<8|int(attrs[3])
		}
		value := attrs[hdr : hdr+valueOctets]
		attrs = attrs[hdr+valueOctets:]
		if code != 14 {
			continue
		}
		nextHopOctets := int(value[3])
		return value[2], value[4+nextHopOctets+1:]
	}
	t.Fatalf("no MP_REACH_NLRI in %x", attrs)
	return 0, nil
}

func concatPluginNLRIs(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
