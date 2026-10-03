package reactor

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// RFC 7752 Section 3.2 assigns VPN link-state information AFI 16388 and SAFI 72.
// RFC requirement: RFC7752-3.2-6 positive -- a VPN Node NLRI reaches a negotiated peer in MP_REACH for AFI 16388/SAFI 72 with its RD intact.
// RFC requirement: RFC7752-3.2-6 negative -- VPN and non-VPN Node NLRIs in the same send matrix are never interchanged between SAFI 72 and SAFI 71.
func TestRFC7752VPNFamilyReachesMPReach(t *testing.T) {
	for _, safi := range []family.SAFI{71, 72} {
		fam := family.Family{AFI: family.AFIBGPLS, SAFI: safi}
		wire := lsNodeNLRI(65001)
		if safi == 72 {
			vpn := append([]byte{}, wire[:4]...)
			vpn = append(vpn, 0, 0, 0xfd, 0xe9, 0, 0, 0, 7)
			vpn = append(vpn, wire[4:]...)
			binary.BigEndian.PutUint16(vpn[2:4], uint16(len(vpn)-4))
			wire = vpn
		}
		route, err := nlri.NewWireNLRI(fam, wire, false)
		if err != nil {
			t.Fatal(err)
		}
		peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
		peer.negotiated.Store(&NegotiatedCapabilities{families: map[family.Family]bool{fam: true}})
		api := groupUpdatesReactor([]*Peer{peer}, false)
		batch := bgptypes.NLRIBatch{Family: fam, NLRIs: []nlri.NLRI{route}, NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1"))}
		if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
			t.Fatal(err)
		}
		bodies := updateBodies(t, conn.written())
		if len(bodies) != 1 {
			t.Fatalf("SAFI %d: sent %d updates, want one", safi, len(bodies))
		}
		_, attrs, _ := updateSections(t, bodies[0])
		_, mp, found := findPathAttr(attrs, byte(attribute.AttrMPReachNLRI))
		if !found || len(mp) < 5 || !bytes.Equal(mp[:3], []byte{0x40, 0x04, byte(safi)}) {
			t.Fatalf("SAFI %d: MP_REACH = %x", safi, mp)
		}
		off := 5 + int(mp[3])
		if off > len(mp) || !bytes.Equal(mp[off:], wire) {
			t.Fatalf("SAFI %d: VPN/non-VPN NLRI changed: %x", safi, mp)
		}
	}
}
