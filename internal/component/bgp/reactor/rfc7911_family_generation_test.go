package reactor

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// RFC 7911 Section 5 requires the extended encoding for the negotiated AFI/SAFI
// and generation based on the combination of prefix and Path Identifier.
// RFC requirement: RFC7911-5-3 positive -- generated UPDATEs use Path Identifiers only for the specific negotiated AFI/SAFI.
// RFC requirement: RFC7911-5-3 negative -- negotiating ADD-PATH for another AFI or SAFI leaves this family's NLRI in its ordinary RFC 4271 encoding.
// RFC requirement: RFC7911-5-4 positive -- two paths for one prefix are both generated with their distinct four-octet identifiers in the negotiated family.
// RFC requirement: RFC7911-5-4 negative -- prefix equality does not collapse the two negotiated paths; an unrelated family does not inherit their extended encoding.
func TestRFC7911GeneratedPathsAreScopedToTheNegotiatedFamily(t *testing.T) {
	families := []family.Family{family.IPv4Unicast, family.IPv6Unicast, {AFI: family.AFIIPv4, SAFI: family.SAFIMulticast}}
	for _, apFamily := range families {
		local := append(peerOffering(families...), &capability.AddPath{Families: []capability.AddPathFamily{{AFI: apFamily.AFI, SAFI: apFamily.SAFI, Mode: capability.AddPathSend}}})
		remote := append(peerOffering(families...), &capability.AddPath{Families: []capability.AddPathFamily{{AFI: apFamily.AFI, SAFI: apFamily.SAFI, Mode: capability.AddPathReceive}}})
		neg := capability.Negotiate(local, remote, testPeerIdentity)
		ctx := bgpctx.FromNegotiatedSend(neg)
		ctxID, err := bgpctx.Registry.Register(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, fam := range families {
			prefix, nextHop := netip.MustParsePrefix("198.51.100.0/24"), netip.MustParseAddr("192.0.2.1")
			if fam.AFI == family.AFIIPv6 {
				prefix, nextHop = netip.MustParsePrefix("2001:db8:7::/64"), netip.MustParseAddr("2001:db8::1")
			}
			peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
			peer.negotiated.Store(NewNegotiatedCapabilities(neg))
			peer.sendCtx.Store(ctx)
			peer.sendCtxID, peer.session.sendCtxID = ctxID, ctxID
			api := groupUpdatesReactor([]*Peer{peer}, false)
			ids := []uint32{7}
			if fam == apFamily {
				ids = append(ids, 9)
			}
			var routes []nlri.NLRI
			var want []byte
			for _, id := range ids {
				routes = append(routes, nlri.NewINET(fam, prefix, id))
				if fam == apFamily {
					want = binary.BigEndian.AppendUint32(want, id)
				}
				want = append(want, byte(prefix.Bits()))
				want = append(want, prefix.Addr().AsSlice()[:(prefix.Bits()+7)/8]...)
			}
			batch := bgptypes.NLRIBatch{Family: fam, NLRIs: routes, NextHop: bgptypes.NewNextHopExplicit(nextHop)}
			if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
				t.Fatal(err)
			}
			var got []byte
			for _, body := range updateBodies(t, conn.written()) {
				_, attrs, plain := updateSections(t, body)
				if fam == family.IPv4Unicast {
					got = append(got, plain...)
					continue
				}
				_, mp, found := findPathAttr(attrs, byte(attribute.AttrMPReachNLRI))
				if !found || len(mp) < 5 || 5+int(mp[3]) > len(mp) {
					t.Fatalf("%s missing framed MP_REACH: %x", fam, mp)
				}
				got = append(got, mp[5+int(mp[3]):]...)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("ADD-PATH %s, sent family %s: NLRI=%x, want %x", apFamily, fam, got, want)
			}
		}
	}
}
