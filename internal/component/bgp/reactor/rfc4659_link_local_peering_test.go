// Design: docs/architecture/bgp/structural-forwarding.md -- VPN link-local peering.
package reactor

import (
	"bufio"
	"bytes"
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// rfc4659PeeringConn supplies actual session endpoints to connectedTransport while
// retaining the existing final-writer byte recorder.
type rfc4659PeeringConn struct {
	*recordingConn
	local, remote netip.Addr
}

func (c *rfc4659PeeringConn) LocalAddr() net.Addr {
	return net.TCPAddrFromAddrPort(netip.AddrPortFrom(c.local, 179))
}

func (c *rfc4659PeeringConn) RemoteAddr() net.Addr {
	return net.TCPAddrFromAddrPort(netip.AddrPortFrom(c.remote, 179))
}

// TestRFC4659LinkLocalPeeringPreservesUnspecifiedGlobalPair checks actual Session
// writer output on both forwarding rails, using captured connection endpoints
// rather than configured local addresses to establish the peering condition.
// RFC 4659 Section 3.2.1.1: "If the BGP speakers peer using only their link-local
// IPv6 address (for example, in the case where an IPv6 CE peers with an IPv6 PE,
// where the CE does not have any IPv6 global address, and where eBGP peering is
// achieved over the link-local addresses), the "unspecified address" ([V6ADDR])
// is used by the advertising BGP speaker to indicate the absence of the global
// IPv6 address in the Next Hop Network Address field."
// RFC requirement: RFC4659-3.2.1.1-1 positive -- link-local-only VPN peering retains the 48-octet zero-RD unspecified-global plus zero-RD link-local next-hop form at the recipient.
// RFC requirement: RFC2545-3-1 negative -- the VPN exception does not permit an unspecified global in IPv6 unicast.
// MUTATION: reject every unspecified global, trim this legal VPN pair to 24
// octets, or allow the exception with global/config-only endpoints or bad framing.
func TestRFC4659LinkLocalPeeringPreservesUnspecifiedGlobalPair(t *testing.T) {
	for _, rail := range []string{"cached", "rs"} {
		for _, tc := range []struct {
			name, local, remote, global, second string
			unicast, single, nonzeroRD, valid   bool
		}{
			{"link-local-only", "fe80::1", "fe80::2", "::", "fe80::1", false, false, false, true},
			{"global-local", "2001:db8:1::1", "fe80::2", "::", "fe80::1", false, false, false, false},
			{"global-peer", "fe80::1", "2001:db8:1::2", "::", "fe80::1", false, false, false, false},
			{"global-peering", "2001:db8:1::1", "2001:db8:1::2", "::", "fe80::1", false, false, false, false},
			{"single-unspecified", "fe80::1", "fe80::2", "::", "", false, true, false, false},
			{"multicast-global", "fe80::1", "fe80::2", "ff0e::1", "fe80::1", false, false, false, false},
			{"wrong-second", "fe80::1", "fe80::2", "::", "2001:db8:1::9", false, false, false, false},
			{"nonzero-rd", "fe80::1", "fe80::2", "::", "fe80::1", false, false, true, false},
			{"unicast", "fe80::1", "fe80::2", "::", "fe80::1", true, false, false, false},
		} {
			t.Run(rail+"/"+tc.name, func(t *testing.T) {
				f := newAIGPReplayFixture(t, nil)
				fam := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}
				raw := mustHex(t, "980001010000ffff0001000020010db800070000")
				if tc.unicast {
					fam = family.IPv6Unicast
					raw = mustHex(t, "4020010db800070000")
				}
				for _, peer := range []*Peer{f.source, f.destination} {
					peer.negotiated.Store(&NegotiatedCapabilities{ASN4: true, families: map[family.Family]bool{fam: true}})
				}
				delete(f.r.peers, f.destination.Settings().PeerKey())
				f.destination.settings.Address = netip.MustParseAddr(tc.remote)
				f.destination.settings.PeerAS = 65002
				f.destination.settings.NextHopMode = NextHopUnchanged
				// Configuration deliberately disagrees with the connection. Only
				// the captured established-session endpoint can license the form.
				f.destination.settings.LocalAddress = netip.MustParseAddr("fe80::ffff")
				if tc.valid {
					f.destination.settings.LocalAddress = netip.MustParseAddr("2001:db8:ffff::1")
				}
				conn := &rfc4659PeeringConn{recordingConn: f.conn, local: netip.MustParseAddr(tc.local), remote: netip.MustParseAddr(tc.remote)}
				f.destination.session.conn = conn
				f.destination.session.bufWriter = bufio.NewWriterSize(conn, 4096)
				f.destination.session.transport.Store(connectedTransport(conn))
				f.destination.refreshLinkScopeFrom([]netip.Prefix{
					netip.MustParsePrefix("fe80::/64"),
					netip.MustParsePrefix("2001:db8:1::/64"),
				})
				f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
				f.r.peers[f.destination.Settings().PeerKey()] = f.destination
				f.r.fwdPool.registerOutgoingPool(fwdKey{peerAddr: f.destination.Settings().PeerKey()}, 4096)
				var hop []byte
				if !tc.unicast {
					hop = append(hop, make([]byte, 8)...)
				}
				hop = append(hop, netip.MustParseAddr(tc.global).AsSlice()...)
				if !tc.single {
					if !tc.unicast {
						hop = append(hop, make([]byte, 8)...)
					}
					hop = append(hop, netip.MustParseAddr(tc.second).AsSlice()...)
				}
				if tc.nonzeroRD {
					hop[0] = 1
				}
				id := f.receive(t, buildUpdatePayload(mixedAttrs(mixedReach(byte(fam.SAFI), hop, raw)), nil))
				if rail == "rs" {
					update, found := f.r.recentUpdates.Get(id)
					if !found {
						t.Fatal("received update missing before RS dispatch")
					}
					skipped, sent := reactorForwardRS(f.r, update, id, f.source.Settings().Address, f.source)
					if len(skipped) != 0 || sent != 1 {
						t.Fatalf("RS forwarded=%d skipped=%v", sent, skipped)
					}
					f.source.session.flushFwdDirty()
				} else {
					f.forward(t, id)
				}
				forwardSocketBarrier(t, f.r)
				bodies := aigpSocketBodies(t, f.conn)
				if len(bodies) != 1 {
					t.Fatalf("recipient UPDATE count=%d, want exactly one", len(bodies))
				}
				update, err := message.UnpackUpdate(bodies[0])
				if err != nil {
					t.Fatal(err)
				}
				_, _, reach, hasReach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
				_, _, unreach, hasUnreach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
				if tc.valid {
					want := append([]byte{0, 2, byte(fam.SAFI), byte(len(hop))}, hop...)
					want = append(want, 0)
					want = append(want, raw...)
					if !hasReach || hasUnreach || !bytes.Equal(reach, want) {
						t.Errorf("legal VPN pair: reach=%x present=%v unreach=%x present=%v, want %x", reach, hasReach, unreach, hasUnreach, want)
					}
				} else {
					want := mixedUnreachValue(2, byte(fam.SAFI), raw)
					if hasReach || !hasUnreach || !bytes.Equal(unreach, want) {
						t.Errorf("forbidden next hop: reach=%x present=%v unreach=%x present=%v, want %x", reach, hasReach, unreach, hasUnreach, want)
					}
				}
				if len(update.NLRI) != 0 || len(update.WithdrawnRoutes) != 0 {
					t.Error("unexpected legacy NLRI alongside native route")
				}
			})
		}
	}
}
