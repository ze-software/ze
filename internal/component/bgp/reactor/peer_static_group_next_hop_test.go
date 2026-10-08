package reactor

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// TestStaticGroupedResolvedNextHopWire sends two configured routes with identical
// attributes through initial synchronization and compares exact next-hop/NLRI
// fields, frame counts and recorded ownership. The unrelated legacy NEXT_HOP
// compatibility attribute on MP-only routes is outside this dispatch proof.
//
// RFC requirement: RFC2545-3-1 positive -- two same-attribute IPv4 routes with
// explicit or session-resolved IPv6 next hops retain the exact global address,
// both NLRIs, frame/EOR counts and successful route ownership.
// RFC requirement: RFC2545-3-2 positive -- both resolved IPv6 cases encode
// the exact sixteen-octet MP_REACH next-hop field rather than legacy NLRI.
// RFC requirement: RFC8950-4-1 positive -- these grouped IPv4 routes reach
// the recipient under the actual session's negotiated extended-next-hop pair.
func TestStaticGroupedResolvedNextHopWire(t *testing.T) {
	const global = "20010db8000100000000000000000001"
	cases := []struct {
		name    string
		fam     family.Family
		nextHop string
		self    bool
		labels  []uint32
		rd      string
		attrs   []string
		legacy  string
	}{
		{name: "extended-explicit", fam: family.IPv4Unicast, nextHop: "2001:db8:1::1", attrs: []string{
			"800e1900010110" + global + "0018c00002",
			"800e1900010110" + global + "0018c63364",
		}},
		{name: "extended-self", fam: family.IPv4Unicast, nextHop: "2001:db8:1::1", self: true, attrs: []string{
			"800e1900010110" + global + "0018c00002",
			"800e1900010110" + global + "0018c63364",
		}},
		{name: "native-ipv4", fam: family.IPv4Unicast, nextHop: "192.0.2.1", attrs: []string{"400304c0000201"}, legacy: "18c0000218c63364"},
		{name: "labeled-ipv4", fam: family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}, nextHop: "192.0.2.1", labels: []uint32{16000}, attrs: []string{
			"800e1000010404c0000201003003e801c00002",
			"800e1000010404c0000201003003e801c63364",
		}},
		{name: "vpn-ipv4", fam: family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}, nextHop: "192.0.2.1", labels: []uint32{16000}, rd: "65000:7", attrs: []string{
			"800e200001800c0000000000000000c0000201007003e8010000fde800000007c00002",
			"800e200001800c0000000000000000c0000201007003e8010000fde800000007c63364",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peer, conn := newInitialSyncPeer(t, true, tc.fam)
			peer.settings.GroupUpdates = true
			nextHop := netip.MustParseAddr(tc.nextHop)
			policy := bgptypes.NewNextHopExplicit(nextHop)
			if tc.self {
				policy = bgptypes.NewNextHopSelf()
				peer.currentSession().transport.Store(&sessionTransport{local: nextHop})
			}
			if nextHop.Is6() {
				peer.settings.Address = netip.MustParseAddr("2001:db8:2::2")
				caps := []capability.Capability{
					&capability.ASN4{ASN: peer.settings.LocalAS},
					&capability.Multiprotocol{AFI: tc.fam.AFI, SAFI: tc.fam.SAFI},
					&capability.ExtendedNextHop{Families: []capability.ExtendedNextHopFamily{{
						NLRIAFI: capability.AFIIPv4, NLRISAFI: capability.SAFI(tc.fam.SAFI), NextHopAFI: capability.AFIIPv6,
					}}},
				}
				neg := capability.Negotiate(caps, caps, capability.PeerIdentity{
					LocalASN: peer.settings.LocalAS, PeerASN: peer.settings.PeerAS,
				})
				peer.session.negotiated = neg
				peer.negotiated.Store(NewNegotiatedCapabilities(neg))
				peer.sendCtx.Store(bgpctx.NewEncodingContext(neg.Identity, neg.Encoding, bgpctx.DirectionSend))
			}
			peer.refreshLinkScopeFrom(nil)
			for _, prefix := range []string{"192.0.2.0/24", "198.51.100.0/24"} {
				route := StaticRoute{Prefix: netip.MustParsePrefix(prefix), NextHop: policy, Labels: tc.labels, RD: tc.rd}
				if tc.rd != "" {
					route.RDBytes = [8]byte{0, 0, 0xfd, 0xe8, 0, 0, 0, 7}
				}
				peer.settings.StaticRoutes = append(peer.settings.StaticRoutes, route)
			}

			// RFC 8950 Section 3: IPv4 NLRI with IPv6 next hops uses MP_REACH;
			// RFC 8277 Section 2 and RFC 4364 Section 4.3.4 retain labeled NLRI.
			peer.sendInitialRoutes()

			remaining := conn.written()
			for _, attrs := range tc.attrs {
				if len(remaining) < message.HeaderLen {
					t.Fatalf("missing announcement: remaining wire = %x", remaining)
				}
				length := int(binary.BigEndian.Uint16(remaining[16:18]))
				if length < message.HeaderLen || length > len(remaining) {
					t.Fatalf("invalid announcement length %d: wire = %x", length, remaining)
				}
				staticGroupCheckUpdate(t, remaining[:length], attrs, tc.legacy)
				remaining = remaining[length:]
			}
			if !bytes.Equal(remaining, eorWire(tc.fam)) {
				t.Errorf("remaining wire = %x, want exactly one EOR = %x", remaining, eorWire(tc.fam))
			}
			if !reflect.DeepEqual(peer.staticWire.routes, peer.settings.StaticRoutes) {
				t.Errorf("recorded routes = %+v, want %+v", peer.staticWire.routes, peer.settings.StaticRoutes)
			}
			if peer.staticWire.session != peer.currentSession() {
				t.Error("successful routes not owned by the receiving session")
			}
		})
	}
}

// staticGroupCheckUpdate compares literal route-bearing fields independently of
// the builders, without pinning unrelated compatibility attributes.
func staticGroupCheckUpdate(t *testing.T, packet []byte, attrs, nlri string) {
	t.Helper()
	if packet[18] != 2 {
		t.Fatalf("message type = %d, want UPDATE", packet[18])
	}
	wantAttr, err := hex.DecodeString(attrs)
	if err != nil {
		t.Fatal(err)
	}
	wantNLRI, err := hex.DecodeString(nlri)
	if err != nil {
		t.Fatal(err)
	}
	body := packet[message.HeaderLen:]
	// RFC 4271 Section 4.3 separates withdrawals, attributes and legacy NLRI.
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		t.Fatal(err)
	}
	if sections.WithdrawnLen() != 0 || !bytes.Equal(sections.NLRI(body), wantNLRI) {
		t.Errorf("withdrawals = %x, NLRI = %x; want no withdrawals and NLRI = %x",
			sections.Withdrawn(body), sections.NLRI(body), wantNLRI)
	}
	code := attribute.AttributeCode(wantAttr[1])
	got, present := rfc4271FindAttr(sections.Attrs(body), code)
	if !present || !bytes.Equal(got, wantAttr[3:]) {
		t.Errorf("attribute %d = %x (present=%v), want %x", code, got, present, wantAttr[3:])
	}
	if _, present := rfc4271FindAttr(sections.Attrs(body), attribute.AttrMPUnreachNLRI); present {
		t.Error("announcement also carries an MP withdrawal")
	}
	if code == attribute.AttrNextHop {
		if _, present := rfc4271FindAttr(sections.Attrs(body), attribute.AttrMPReachNLRI); present {
			t.Error("legacy group also carries MP reachability")
		}
	}
}
