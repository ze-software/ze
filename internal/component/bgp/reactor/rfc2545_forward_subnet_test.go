// Design: docs/architecture/bgp/structural-forwarding.md -- received next-hop scope.
package reactor

import (
	"bytes"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC2545ReceivedPairSubnetConditions checks the final writer with actual
// IPv6 destination addresses: IPv6 unicast takes the joint common-prefix
// predicate, VPN-IPv6 the recipient-only one.
// RFC 2545 Section 3: "The link-local address shall be included in the Next Hop
// field if and only if the BGP speaker shares a common subnet with the entity
// identified by the global IPv6 address carried in the Network Address of Next
// Hop field and the peer the route is being advertised to."
// RFC 4659 Section 3.2.1.1: "The link-local address shall be included in the
// Next Hop field if and only if the advertising BGP speaker shares a common
// subnet with the peer the route is being advertised to [BGP-IPv6]." So the
// vpn/global-off-link case keeps its pair (owner decision 2026-10-19).
// RFC requirement: RFC2545-3-3 positive -- a received global/link-local pair remains intact when the recipient and effective Global share one connected prefix.
// RFC requirement: RFC2545-3-3 negative -- on IPv6 unicast either off-link entity, and on VPN-IPv6 an off-link recipient, removes only the link-local half, preserving the global address and native NLRI.
// MUTATION: bypass the received-global subnet check (the unicast cases go red)
// or the destination subnet check in applyEgressNextHopScope; alternatively
// strip every pair.
func TestRFC2545ReceivedPairSubnetConditions(t *testing.T) {
	for _, native := range []struct {
		name string
		fam  family.Family
		nlri string
	}{
		{"unicast", family.IPv6Unicast, "4020010db800070000"},
		{"vpn", family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}, "980001010000ffff0001000020010db800070000"},
	} {
		for _, scope := range []struct {
			name        string
			peerOnLink  bool
			global      string
			keepUnicast bool
			keepVPN     bool
		}{
			{"both-on-link", true, "2001:db8:1::9", true, true},
			{"global-off-link", true, "2001:db8:2::9", false, true},
			{"peer-off-link", false, "2001:db8:1::9", false, false},
			{"both-off-link", false, "2001:db8:2::9", false, false},
		} {
			t.Run(native.name+"/"+scope.name, func(t *testing.T) {
				f := newAIGPReplayFixture(t, nil)
				for _, peer := range []*Peer{f.source, f.destination} {
					negotiateForwardTestFamilies(peer, native.fam)
				}
				f.destination.settings.NextHopMode = NextHopUnchanged
				f.destination.settings.PeerAS = 65002
				rfc2545ForwardScope(f, scope.peerOnLink, true)
				global := netip.MustParseAddr(scope.global).AsSlice()
				linkLocal := netip.MustParseAddr("fe80::9").AsSlice()
				var pair []byte
				if native.fam.SAFI == family.SAFIVPN {
					pair = append(pair, make([]byte, 8)...)
				}
				pair = append(pair, global...)
				globalOctets := len(pair)
				if native.fam.SAFI == family.SAFIVPN {
					pair = append(pair, make([]byte, 8)...)
				}
				pair = append(pair, linkLocal...)
				raw := mustHex(t, native.nlri)
				body := buildUpdatePayload(mixedAttrs(mixedReach(byte(native.fam.SAFI), pair, raw)), nil)
				original := bytes.Clone(body)
				id := f.receive(t, body)
				if got := destOnLink(f.destination); got != scope.peerOnLink {
					t.Fatalf("fixture destination scope=%v, want %v before dispatch", got, scope.peerOnLink)
				}
				f.forward(t, id)
				forwardSocketBarrier(t, f.r)
				bodies := aigpSocketBodies(t, f.conn)
				if len(bodies) != 1 {
					t.Fatalf("recipient UPDATE count=%d, want one native announcement", len(bodies))
				}
				update, err := message.UnpackUpdate(bodies[0])
				if err != nil {
					t.Fatal(err)
				}
				keepPair := scope.keepUnicast
				if native.fam.SAFI == family.SAFIVPN {
					keepPair = scope.keepVPN
				}
				wantHop := pair[:globalOctets]
				if keepPair {
					wantHop = pair
				}
				want := []byte{0, 2, byte(native.fam.SAFI), byte(len(wantHop))}
				want = append(want, wantHop...)
				want = append(want, 0)
				want = append(want, raw...)
				_, _, mp, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
				if !found || !bytes.Equal(mp, want) {
					t.Errorf("recipient MP_REACH=%x present=%v, want %x", mp, found, want)
				}
				if len(update.WithdrawnRoutes) != 0 || len(update.NLRI) != 0 {
					t.Errorf("unexpected legacy NLRI: %+v", update)
				}
				if _, _, _, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI); found {
					t.Error("next-hop trimming withdrew the native route")
				}
				if !bytes.Equal(body, original) {
					t.Error("forwarding changed the received payload")
				}
			})
		}
	}
}

// TestRFC2545ReceivedUnusableGlobalRefused reads exact withdrawal bytes from the
// writer for unusable global addresses, with and without the link-local extension.
// RFC 2545 Section 3: "A BGP speaker shall advertise to its peer in the Network
// Address of Next Hop field the global IPv6 address of the next hop, potentially
// followed by the link-local IPv6 address of the next hop."
// RFC requirement: RFC2545-3-1 negative -- unspecified and multicast global addresses never reach the recipient as announcements, in either single or paired form.
// RFC requirement: RFC2545-3-1 positive -- a usable global remains advertised; negotiated link-local-only extension handling is independently preserved.
// MUTATION: bypass the unusable-global egress gate, or reject every IPv6 next hop.
func TestRFC2545ReceivedUnusableGlobalRefused(t *testing.T) {
	for _, negotiated := range []bool{false, true} {
		for _, tc := range []struct {
			name  string
			hop   string
			pair  bool
			valid bool
		}{
			{"unspecified", "::", false, false},
			{"unspecified-pair", "::", true, false},
			{"multicast-link", "ff02::1", false, false},
			{"multicast-link-pair", "ff02::1", true, false},
			{"multicast-global", "ff0e::1", false, false},
			{"multicast-global-pair", "ff0e::1", true, false},
			{"global", "2001:db8:1::9", false, true},
			{"global-pair", "2001:db8:1::9", true, true},
			{"link-local-extension", "fe80::9", false, negotiated},
		} {
			capabilityName := "without-capability"
			if negotiated {
				capabilityName = "with-capability"
			}
			t.Run(capabilityName+"/"+tc.name, func(t *testing.T) {
				f := newAIGPReplayFixture(t, nil)
				for _, peer := range []*Peer{f.source, f.destination} {
					caps := []capability.Capability{
						&capability.ASN4{ASN: peer.settings.LocalAS},
						&capability.Multiprotocol{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast},
					}
					if negotiated {
						caps = append(caps, &capability.LinkLocalNextHop{})
					}
					neg := capability.Negotiate(caps, caps, capability.PeerIdentity{
						LocalASN: peer.settings.LocalAS, PeerASN: peer.settings.PeerAS,
						Internal: peer.settings.IsIBGP(),
					})
					peer.session.negotiated = neg
					peer.negotiated.Store(NewNegotiatedCapabilities(neg))
				}
				f.destination.settings.NextHopMode = NextHopUnchanged
				f.destination.settings.PeerAS = 65002
				rfc2545ForwardScope(f, true, true)
				hop := netip.MustParseAddr(tc.hop).AsSlice()
				if tc.pair {
					hop = append(hop, netip.MustParseAddr("fe80::9").AsSlice()...)
				}
				raw := mustHex(t, "4020010db800070000")
				body := buildUpdatePayload(mixedAttrs(mixedReach(1, hop, raw)), nil)
				f.advertiseFirst(t, body)
				id := f.receive(t, body)
				if !destOnLink(f.destination) {
					t.Fatal("fixture destination lost its on-link scope before dispatch")
				}
				f.forward(t, id)
				forwardSocketBarrier(t, f.r)
				bodies := aigpSocketBodies(t, f.conn)
				if len(bodies) != 1 {
					t.Fatalf("recipient UPDATE count=%d, want exactly one announcement or withdrawal", len(bodies))
				}
				update, err := message.UnpackUpdate(bodies[0])
				if err != nil {
					t.Fatal(err)
				}
				_, _, reach, hasReach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
				_, _, unreach, hasUnreach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
				if tc.valid {
					want := []byte{0, 2, 1, byte(len(hop))}
					want = append(want, hop...)
					want = append(want, 0)
					want = append(want, raw...)
					if !hasReach || hasUnreach || !bytes.Equal(reach, want) {
						t.Fatalf("valid next hop: reach=%x present=%v unreach=%x present=%v, want %x", reach, hasReach, unreach, hasUnreach, want)
					}
				} else {
					want := mixedUnreachValue(2, 1, raw)
					if hasReach || !hasUnreach || !bytes.Equal(unreach, want) {
						t.Fatalf("unusable next hop: reach=%x present=%v unreach=%x present=%v, want withdrawal %x", reach, hasReach, unreach, hasUnreach, want)
					}
				}
				if len(update.NLRI) != 0 || len(update.WithdrawnRoutes) != 0 {
					t.Error("unexpected legacy NLRI alongside native route")
				}
			})
		}
	}
}

// TestRFC2545ReceivedPairSecondAddressValidated crosses Session.processMessage
// before either forwarding rail and inspects the recipient's native UPDATE.
// The scope cases exercise configured prefix-membership predicates only, not
// next-hop-entity identity or whole RFC2545-3-3 adjacency.
// RFC 2545 Section 3: "A BGP speaker shall advertise to its peer in the Network
// Address of Next Hop field the global IPv6 address of the next hop, potentially
// followed by the link-local IPv6 address of the next hop."
// draft-ietf-idr-linklocal-capability Section 6: "If the Next Hop field is
// malformed, the implementation MUST handle the malformed UPDATE message using
// the approach of "treat-as-withdraw", as described in section 7.3 of [RFC7606]."
// RFC requirement: RFC2545-3-1 negative -- on a receiving session without capability 77, a usable first global never licenses a global, unspecified or multicast second address: the recipient is sent a withdrawal, whether the pair would otherwise be preserved or trimmed, whichever side of the sending session negotiated capability 77.
// RFC requirement: RFC2545-3-1 positive -- on a receiving session without capability 77, a global/link-local pair is announced, whole or trimmed to its global half by the configured prefix predicates, whichever side of the sending session negotiated capability 77.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1 negative -- on a receiving session that negotiated capability 77, a 32-octet Next Hop whose second address is global, unspecified or multicast is malformed and treated as withdraw at receipt: the Session publishes the withdrawal and the recipient is sent no announcement.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1 positive -- on a receiving session that negotiated capability 77, the well-formed Next Hops are not withdrawn at receipt: a global/link-local pair is announced, whole or trimmed to its global half, and a single link-local is announced when the sending session negotiated capability 77 on both sides and the peer is on link, and the recipient is sent a withdrawal otherwise.
// MUTATION: validate only the first address, or trim before checking the second;
// alternatively reject all pairs or let capability 77 license malformed pairs.
func TestRFC2545ReceivedPairSecondAddressValidated(t *testing.T) {
	for _, rail := range []string{"cached", "rs"} {
		for _, caps := range []struct {
			name          string
			local, remote bool
		}{
			{"cap77-neither", false, false},
			{"cap77-local-only", true, false},
			{"cap77-remote-only", false, true},
			{"cap77-both", true, true},
		} {
			for _, scope := range []struct {
				name                     string
				peerOnLink, globalOnLink bool
			}{
				{"both-on-link", true, true},
				{"peer-off-link", false, true},
				{"global-off-link", true, false},
			} {
				for _, tc := range []struct {
					name, second  string
					valid, single bool
					// source77 negotiates capability 77 on the receiving session.
					// withdrawn expects draft-ietf-idr-linklocal-capability Section 6
					// treat-as-withdraw there: a second address that is not Link-Local.
					source77, withdrawn bool
				}{
					{"second-global", "2001:db8:1::a", false, false, false, false},
					{"second-unspecified", "::", false, false, false, false},
					{"second-multicast", "ff02::1", false, false, false, false},
					{"second-link-local", "fe80::9", true, false, false, false},
					{"link-local-only-control", "", true, true, true, false},
					{"cap77-source-second-global", "2001:db8:1::a", false, false, true, true},
					{"cap77-source-second-unspecified", "::", false, false, true, true},
					{"cap77-source-second-multicast", "ff02::1", false, false, true, true},
					{"cap77-source-second-link-local", "fe80::9", true, false, true, false},
				} {
					t.Run(rail+"/"+caps.name+"/"+scope.name+"/"+tc.name, func(t *testing.T) {
						f := newAIGPReplayFixture(t, nil)
						f.destination.settings.NextHopMode = NextHopUnchanged
						f.destination.settings.PeerAS = 65002
						for _, peer := range []*Peer{f.source, f.destination} {
							localLL, remoteLL := tc.source77, tc.source77
							if peer == f.destination {
								localLL, remoteLL = caps.local, caps.remote
							}
							localCaps := []capability.Capability{
								&capability.ASN4{ASN: peer.settings.LocalAS},
								&capability.Multiprotocol{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast},
							}
							remoteCaps := []capability.Capability{
								&capability.ASN4{ASN: peer.settings.PeerAS},
								&capability.Multiprotocol{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast},
							}
							if localLL {
								localCaps = append(localCaps, &capability.LinkLocalNextHop{})
							}
							if remoteLL {
								remoteCaps = append(remoteCaps, &capability.LinkLocalNextHop{})
							}
							neg := capability.Negotiate(localCaps, remoteCaps, capability.PeerIdentity{
								LocalASN: peer.settings.LocalAS, PeerASN: peer.settings.PeerAS,
							})
							peer.session.negotiated = neg
							peer.negotiated.Store(&NegotiatedCapabilities{
								ASN4: neg.ASN4, LinkLocalNextHop: neg.LinkLocalNextHop,
								families: map[family.Family]bool{family.IPv6Unicast: true},
							})
							t.Cleanup(peer.session.timers.StopAll)
						}
						// Receive negotiates capability 77 only for the single-address
						// control, which needs it to reach egress, and for the cases
						// that prove its Section 6 treat-as-withdraw on receipt.
						f.source.session.recvCtxID = f.ctxID
						f.source.session.SetSourceID(f.source.SourceID())
						rfc2545ForwardScope(f, scope.peerOnLink, scope.globalOnLink)
						if destOnLink(f.destination) != scope.peerOnLink {
							t.Fatal("fixture destination scope differs from the selected predicate")
						}
						hop := netip.MustParseAddr("2001:db8:1::9").AsSlice()
						if tc.single {
							hop = netip.MustParseAddr("fe80::9").AsSlice()
						} else {
							hop = append(hop, netip.MustParseAddr(tc.second).AsSlice()...)
						}
						raw := mustHex(t, "4020010db800070000")
						body := buildUpdatePayload(mixedAttrs(mixedReach(1, hop, raw)), nil)
						updates, published := rfc2545ReceiveForwardPublished(t, f, rail, body, tc.withdrawn)
						if tc.withdrawn {
							rfc2545AssertWithdrawnAtReceipt(t, updates, published, raw, nil)
							return
						}
						if len(updates) != 1 {
							t.Fatalf("recipient UPDATE count=%d, want one native announcement or withdrawal", len(updates))
						}
						update := updates[0]
						_, _, reach, hasReach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
						_, _, unreach, hasUnreach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
						valid := tc.valid
						wantHop := hop
						if tc.single {
							valid = caps.local && caps.remote && scope.peerOnLink
						} else if !scope.peerOnLink || !scope.globalOnLink {
							wantHop = hop[:16]
						}
						if valid {
							want := append([]byte{0, 2, 1, byte(len(wantHop))}, wantHop...)
							want = append(want, 0)
							want = append(want, raw...)
							if !hasReach || hasUnreach || !bytes.Equal(reach, want) {
								t.Errorf("valid next hop: reach=%x present=%v unreach=%x present=%v, want %x", reach, hasReach, unreach, hasUnreach, want)
							}
						} else {
							want := mixedUnreachValue(2, 1, raw)
							if hasReach || !hasUnreach || !bytes.Equal(unreach, want) {
								t.Errorf("forbidden next hop: reach=%x present=%v unreach=%x present=%v, want withdrawal %x", reach, hasReach, unreach, hasUnreach, want)
							}
						}
						if len(update.NLRI) != 0 || len(update.WithdrawnRoutes) != 0 {
							t.Error("unexpected legacy NLRI alongside native route")
						}
					})
				}
			}
		}
	}
}

// rfc2545ReceiveFixture negotiates capability 77 on the receiving session
// always, so a received standalone link-local next hop is accepted, and on the
// sending session only when cap77 is set.
func rfc2545ReceiveFixture(t *testing.T, cap77, peerOnLink, globalOnLink bool) *aigpReplayFixture {
	t.Helper()
	return rfc2545ReceiveFixtureCaps(t, true, cap77, peerOnLink, globalOnLink)
}

// rfc2545ReceiveFixtureCaps negotiates both native families so a mixed legacy/MP
// input crosses the same Session boundary as a standalone IPv6 announcement.
// sourceCap77 decides capability 77 on the receiving session, where
// draft-ietf-idr-linklocal-capability Section 6 turns a malformed 32-octet next
// hop into treat-as-withdraw; destinationCap77 decides it on the sending session.
func rfc2545ReceiveFixtureCaps(t *testing.T, sourceCap77, destinationCap77, peerOnLink, globalOnLink bool) *aigpReplayFixture {
	t.Helper()
	f := newAIGPReplayFixture(t, nil)
	f.destination.settings.NextHopMode = NextHopUnchanged
	f.destination.settings.PeerAS = 65002
	for _, peer := range []*Peer{f.source, f.destination} {
		caps := []capability.Capability{
			&capability.ASN4{ASN: peer.settings.LocalAS},
			&capability.Multiprotocol{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast},
			&capability.Multiprotocol{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast},
		}
		cap77 := destinationCap77
		if peer == f.source {
			cap77 = sourceCap77
		}
		if cap77 {
			caps = append(caps, &capability.LinkLocalNextHop{})
		}
		neg := capability.Negotiate(caps, caps, capability.PeerIdentity{
			LocalASN: peer.settings.LocalAS, PeerASN: peer.settings.PeerAS,
		})
		peer.session.negotiated = neg
		peer.negotiated.Store(&NegotiatedCapabilities{
			ASN4: neg.ASN4, LinkLocalNextHop: neg.LinkLocalNextHop,
			families: map[family.Family]bool{family.IPv4Unicast: true, family.IPv6Unicast: true},
		})
		t.Cleanup(peer.session.timers.StopAll)
	}
	f.source.session.recvCtxID = f.ctxID
	f.source.session.SetSourceID(f.source.SourceID())
	f.source.session.nextHopScope.Store(&receiveNextHopScope{
		addresses: []netip.Prefix{netip.MustParsePrefix("198.18.231.254/24")},
		local:     netip.MustParseAddr("198.18.231.254"),
		remote:    f.source.Settings().Address,
		direct:    true,
	})
	rfc2545ForwardScope(f, peerOnLink, globalOnLink)
	if destOnLink(f.destination) != peerOnLink {
		t.Fatal("fixture destination scope differs from selected predicate")
	}
	return f
}

// rfc2545ForwardScope gives the recipient an IPv6 identity on the Global's
// prefix only in the common-subnet positive. The IPv4 source stays distinct
// because mixed-message cases also assert its valid legacy NEXT_HOP unchanged.
func rfc2545ForwardScope(f *aigpReplayFixture, peerOnLink, globalOnLink bool) {
	delete(f.r.peers, f.destination.Settings().PeerKey())
	recipient := "2001:db8:ffff::2"
	if peerOnLink {
		recipient = "2001:db8:2::2"
		if globalOnLink {
			recipient = "2001:db8:1::2"
		}
	}
	f.destination.settings.Address = netip.MustParseAddr(recipient)
	f.r.peers[f.destination.Settings().PeerKey()] = f.destination
	f.r.fwdPool.registerOutgoingPool(fwdKey{peerAddr: f.destination.Settings().PeerKey()}, 4096)
	connected := []netip.Prefix{netip.PrefixFrom(f.source.Settings().Address, 32)}
	if peerOnLink {
		connected = append(connected, netip.PrefixFrom(f.destination.Settings().Address, 64))
	}
	if globalOnLink {
		// fe80::/64 exposes the invalid pair-first slot separately from
		// capability 77's valid standalone link-local form.
		for _, prefix := range []string{"2001:db8:1::/64", "fe80::/64", "::1/128", "::ffff:192.0.2.0/120"} {
			connected = append(connected, netip.MustParsePrefix(prefix))
		}
	}
	f.destination.refreshLinkScopeFrom(connected)
	f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
}

// rfc2545ReceiveForward MUST retain the published cache entry until the writer
// assertions finish; its cleanup MUST release that retain before fixture teardown.
// The Session MUST publish the received UPDATE unchanged.
func rfc2545ReceiveForward(t *testing.T, f *aigpReplayFixture, rail string, body []byte) []*message.Update {
	t.Helper()
	recipient, _ := rfc2545ReceiveForwardPublished(t, f, rail, body, false)
	return recipient
}

// rfc2545ReceiveForwardPublished is rfc2545ReceiveForward with the receive
// verdict selectable. withdrawnAtReceipt expects RFC 7606 treat-as-withdraw: every
// UPDATE the Session publishes announces nothing, and each is forwarded. It
// returns the recipient's UPDATEs, then the UPDATEs the Session published.
func rfc2545ReceiveForwardPublished(t *testing.T, f *aigpReplayFixture, rail string, body []byte,
	withdrawnAtReceipt bool) (recipient, published []*message.Update) {
	t.Helper()
	original := bytes.Clone(body)
	if !withdrawnAtReceipt {
		// A gate's withdrawal reaches only a recipient that was advertised
		// the route, so the recipient first holds the source's earlier
		// generation of it.
		f.advertiseFirst(t, body)
	}
	t.Cleanup(func() {
		if !bytes.Equal(body, original) {
			t.Error("receive or forwarding changed the original UPDATE")
		}
	})
	var ids []uint64
	f.source.session.onMessageReceived = func(addr netip.Addr, typ msgtype.MessageType, received []byte,
		wu *wireu.WireUpdate, ctxID bgpctx.ContextID, direction rpc.MessageDirection,
		buf BufHandle, meta map[string]any, sourcePeer string, sourceMessageID uint64) bool {
		if typ != msgtype.TypeUPDATE || wu == nil {
			t.Fatal("Session did not publish a native UPDATE")
		}
		update, err := message.UnpackUpdate(wu.Payload())
		if err != nil {
			t.Fatal(err)
		}
		published = append(published, update)
		if withdrawnAtReceipt {
			if rfc2545Announces(update) {
				t.Fatalf("Session published an announcement %x, want treat-as-withdraw", wu.Payload())
			}
		} else if !bytes.Equal(wu.Payload(), original) {
			t.Fatal("Session did not publish the intact native announcement")
		}
		kept := f.r.notifyMessageReceiver(addr, typ, received, wu, ctxID, direction, buf, meta, sourcePeer, sourceMessageID)
		id := wu.MessageID()
		ids = append(ids, id)
		f.ids = append(f.ids, id)
		if !f.r.recentUpdates.Retain(id) {
			t.Fatalf("published update %d missing before fixture retain", id)
		}
		t.Cleanup(func() { f.r.recentUpdates.Release(id) })
		return kept
	}
	header := message.Header{Type: msgtype.TypeUPDATE, Length: uint16(message.HeaderLen + len(body))}
	err, kept := f.source.session.processMessage(&header, body, BufHandle{ID: noPoolBufID, Buf: body})
	if err != nil || !kept {
		t.Fatalf("Session receive: err=%v kept=%v, want the UPDATE kept and the session up", err, kept)
	}
	if len(ids) == 0 {
		t.Fatal("Session published nothing")
	}
	if !withdrawnAtReceipt && len(ids) != 1 {
		t.Fatalf("Session publications=%d, want one cached announcement", len(ids))
	}
	for _, id := range ids {
		rfc2545DispatchReceived(t, f, rail, id)
	}
	forwardSocketBarrier(t, f.r)
	for _, body := range aigpSocketBodies(t, f.conn) {
		update, err := message.UnpackUpdate(body)
		if err != nil {
			t.Fatal(err)
		}
		recipient = append(recipient, update)
	}
	return recipient, published
}

// rfc2545AssertWithdrawnAtReceipt checks draft-ietf-idr-linklocal-capability
// Section 6 treat-as-withdraw end to end: the Session published a withdrawal of
// the native route (and of legacy, when given), and the recipient was sent no
// announcement of either.
func rfc2545AssertWithdrawnAtReceipt(t *testing.T, recipient, published []*message.Update, raw, legacy []byte) {
	t.Helper()
	nativeWithdrawn, legacyWithdrawn := false, legacy == nil
	for _, update := range published {
		_, _, unreach, hasUnreach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
		if hasUnreach && bytes.Equal(unreach, mixedUnreachValue(2, 1, raw)) {
			nativeWithdrawn = true
		}
		if legacy != nil && bytes.Equal(update.WithdrawnRoutes, legacy) {
			legacyWithdrawn = true
		}
	}
	if !nativeWithdrawn || !legacyWithdrawn {
		t.Errorf("Session publications withdraw native=%v legacy=%v, want both: treat-as-withdraw", nativeWithdrawn, legacyWithdrawn)
	}
	for _, update := range recipient {
		if rfc2545Announces(update) {
			t.Errorf("recipient was sent an announcement NLRI=%x attrs=%x after treat-as-withdraw", update.NLRI, update.PathAttributes)
		}
	}
}

// rfc2545DispatchReceived forwards one published UPDATE on the selected rail.
func rfc2545DispatchReceived(t *testing.T, f *aigpReplayFixture, rail string, id uint64) {
	t.Helper()
	if rail != "rs" && rail != "rs-export" {
		f.forward(t, id)
		return
	}
	received, found := f.r.recentUpdates.Get(id)
	if !found {
		t.Fatal("Session-delivered update missing before RS dispatch")
	}
	skipped, sent := reactorForwardRS(f.r, received, id, f.source.Settings().Address, f.source)
	if rail == "rs-export" {
		if sent != 0 || len(skipped) != 1 || skipped[0] != f.destination.Settings().PeerKey() {
			t.Fatalf("RS export fallback: forwarded=%d skipped=%v, want this destination skipped", sent, skipped)
		}
		f.forward(t, id)
	} else if len(skipped) != 0 || sent != 1 {
		t.Fatalf("RS forwarded=%d skipped=%v, want one direct-rail decision", sent, skipped)
	}
	f.source.session.flushFwdDirty()
}

// rfc2545Announces reports whether an UPDATE carries any reachability:
// MP_REACH_NLRI or legacy NLRI.
func rfc2545Announces(update *message.Update) bool {
	if len(update.NLRI) != 0 {
		return true
	}
	_, _, _, hasReach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
	return hasReach
}

// rfc2545AssertNative checks complete native reachability and forbids a legacy
// sibling on this particular UPDATE; mixed input must leave as separate sections.
func rfc2545AssertNative(t *testing.T, update *message.Update, hop, raw []byte, valid bool) {
	t.Helper()
	_, _, reach, hasReach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
	_, _, unreach, hasUnreach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
	if valid {
		want := append([]byte{0, 2, 1, byte(len(hop))}, hop...)
		want = append(want, 0)
		want = append(want, raw...)
		if !hasReach || hasUnreach || !bytes.Equal(reach, want) {
			t.Errorf("valid next hop: reach=%x present=%v unreach=%x present=%v, want %x", reach, hasReach, unreach, hasUnreach, want)
		}
	} else {
		want := mixedUnreachValue(2, 1, raw)
		if hasReach || !hasUnreach || !bytes.Equal(unreach, want) {
			t.Errorf("forbidden next hop: reach=%x present=%v unreach=%x present=%v, want withdrawal %x", reach, hasReach, unreach, hasUnreach, want)
		}
	}
	if len(update.NLRI) != 0 || len(update.WithdrawnRoutes) != 0 {
		t.Error("unexpected legacy NLRI alongside native route")
	}
}

// RFC 2545 Section 3: "A BGP speaker shall advertise to its peer in the Network
// Address of Next Hop field the global IPv6 address of the next hop, potentially
// followed by the link-local IPv6 address of the next hop."
// draft-ietf-idr-linklocal-capability Section 6: "If the Next Hop field is
// malformed, the implementation MUST handle the malformed UPDATE message using
// the approach of "treat-as-withdraw", as described in section 7.3 of [RFC7606]."
// RFC requirement: RFC2545-3-1 negative -- a valid second link-local cannot license an unusable first address, even when trimming would hide the original pair: a link-local or unspecified first address, and on a receiving session without capability 77 also a loopback, multicast or IPv4-mapped one, draws a withdrawal to the recipient.
// RFC requirement: RFC2545-3-1 positive -- a usable global first address retains the valid pair or only its global half according to configured prefix predicates, whichever session negotiated capability 77.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1 negative -- on a receiving session that negotiated capability 77, a loopback, multicast or IPv4-mapped first address is malformed and treated as withdraw at receipt: the Session publishes the withdrawal and the recipient is sent no announcement.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1 positive -- on a receiving session that negotiated capability 77, a global, unspecified or link-local first address is not withdrawn at receipt: the recipient is sent one native decision: the global pair announced, whole or trimmed to its global half, and the others withdrawn.
// MUTATION: check only unspecified/multicast first addresses, or permit a
// paired link-local first address under capability 77's single-address exception.
func TestRFC2545ReceivedPairFirstAddressValidated(t *testing.T) {
	for _, rail := range []string{"cached", "rs"} {
		for _, caps := range []struct {
			name                string
			source, destination bool
		}{
			{"cap77-neither", false, false},
			{"cap77-destination", false, true},
			{"cap77-source", true, false},
			{"cap77-both", true, true},
		} {
			for _, scope := range []struct {
				name                     string
				peerOnLink, globalOnLink bool
			}{
				{"both-on-link", true, true},
				{"peer-off-link", false, true},
				{"global-off-link", true, false},
			} {
				for _, tc := range []struct {
					name, first string
					valid       bool
					// malformed77 marks a first address that is neither Global, ::,
					// nor Link-Local: draft-ietf-idr-linklocal-capability Section 6
					// malformed, so a capability-77 receiver withdraws it on receipt.
					malformed77 bool
				}{
					{"link-local", "fe80::9", false, false},
					{"ipv4-mapped", "::ffff:192.0.2.9", false, true},
					{"loopback", "::1", false, true},
					{"unspecified", "::", false, false},
					{"multicast", "ff02::1", false, true},
					{"global-control", "2001:db8:1::9", true, false},
				} {
					t.Run(rail+"/"+caps.name+"/"+scope.name+"/"+tc.name, func(t *testing.T) {
						f := rfc2545ReceiveFixtureCaps(t, caps.source, caps.destination, scope.peerOnLink, scope.globalOnLink)
						first := netip.MustParseAddr(tc.first).As16()
						hop := append(first[:], netip.MustParseAddr("fe80::a").AsSlice()...)
						raw := mustHex(t, "4020010db800070000")
						body := buildUpdatePayload(mixedAttrs(mixedReach(1, hop, raw)), nil)
						updates, published := rfc2545ReceiveForwardPublished(t, f, rail, body, caps.source && tc.malformed77)
						if caps.source && tc.malformed77 {
							rfc2545AssertWithdrawnAtReceipt(t, updates, published, raw, nil)
							return
						}
						if len(updates) != 1 {
							t.Fatalf("recipient UPDATE count=%d, want one native decision", len(updates))
						}
						wantHop := hop
						if !scope.peerOnLink || !scope.globalOnLink {
							wantHop = hop[:16]
						}
						rfc2545AssertNative(t, updates[0], wantHop, raw, tc.valid)
					})
				}
			}
		}
	}
}

// RFC 2545 Section 3: "A BGP speaker shall advertise to its peer in the Network
// Address of Next Hop field the global IPv6 address of the next hop, potentially
// followed by the link-local IPv6 address of the next hop."
// draft-ietf-idr-linklocal-capability Section 6: "If the Next Hop field is
// malformed, the implementation MUST handle the malformed UPDATE message using
// the approach of "treat-as-withdraw", as described in section 7.3 of [RFC7606]."
// RFC requirement: RFC2545-3-1 negative -- on a receiving session without capability 77, an effective invalid pair is withdrawn after policy, without withdrawing an independently valid legacy sibling.
// RFC requirement: RFC2545-3-1 positive -- on a receiving session without capability 77, a valid policy replacement of an obsolete malformed received pair is announced with exact native bytes.
// RFC requirement: RFC2545-3-2 negative -- effective native IPv4 and VPN-shaped next-hop fields cannot leave under IPv6-unicast AFI/SAFI, including policy pairs whose off-link trim would hide their original width.
// RFC requirement: RFC2545-3-2 positive -- a later legal sixteen- or thirty-two-octet policy replacement supersedes an obsolete wrong-width operation, with valid legacy siblings unchanged.
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1 negative -- on a receiving session that negotiated capability 77, a received global/global pair is malformed and treated as withdraw at receipt: the Session publishes the withdrawal of every route of the UPDATE, legacy sibling included, and the recipient is sent no announcement.
// MUTATION: validate the obsolete received pair instead of its replacement,
// trust every policy-written pair, or withdraw the whole mixed UPDATE.
func TestRFC2545EffectivePairPolicyAndMixedSibling(t *testing.T) {
	pair := func(first, second string) []byte {
		a := netip.MustParseAddr(first).As16()
		return append(a[:], netip.MustParseAddr(second).AsSlice()...)
	}
	validPair := pair("2001:db8:1::9", "fe80::a")
	invalidFirst := pair("fe80::9", "fe80::a")
	invalidSecond := pair("2001:db8:1::9", "2001:db8:1::a")
	nativeIPv4 := netip.MustParseAddr("192.0.2.9").AsSlice()
	vpnSingle := append(make([]byte, 8), validPair[:16]...)
	vpnPair := append(bytes.Clone(vpnSingle), make([]byte, 8)...)
	vpnPair = append(vpnPair, validPair[16:]...)
	for _, rail := range []string{"cached", "rs"} {
		for _, onLink := range []bool{true, false} {
			scope := "both-on-link"
			if !onLink {
				scope = "peer-off-link"
			}
			for _, mixed := range []bool{false, true} {
				layout := "native"
				if mixed {
					layout = "mixed"
				}
				for _, tc := range []struct {
					name                  string
					received, replacement []byte
					earlier               []byte
					valid                 bool
					rawPolicy             bool
					offLinkOnly           bool
					afterRaw              []byte
					withoutCap77          bool
					onLinkOnly            bool
					// source77 negotiates capability 77 on the receiving session,
					// where withdrawn expects draft-ietf-idr-linklocal-capability
					// Section 6 treat-as-withdraw at receipt.
					source77, withdrawn bool
				}{
					{name: "invalid-first-unchanged", received: invalidFirst},
					{name: "invalid-second-unchanged", received: invalidSecond},
					{name: "invalid-first-to-valid-pair", received: invalidFirst, replacement: validPair, valid: true},
					{name: "invalid-first-to-valid-single", received: invalidFirst, replacement: validPair[:16], valid: true},
					{name: "invalid-second-to-valid-pair", received: invalidSecond, replacement: validPair, valid: true},
					{name: "valid-to-invalid-first", received: validPair, replacement: invalidFirst},
					{name: "valid-to-invalid-second", received: validPair, replacement: invalidSecond},
					{name: "valid-to-native-ipv4", received: validPair, replacement: nativeIPv4},
					{name: "valid-to-vpn24", received: validPair, replacement: vpnSingle},
					{name: "valid-to-vpn48-before-trim", received: validPair, replacement: vpnPair, offLinkOnly: true},
					{name: "superseded-native-ipv4-to-valid-single", received: validPair, earlier: nativeIPv4, replacement: validPair[:16], valid: true},
					{name: "superseded-vpn24-to-valid-pair", received: validPair, earlier: vpnSingle, replacement: validPair, valid: true},
					{name: "superseded-vpn48-to-valid-pair", received: validPair, earlier: vpnPair, replacement: validPair, valid: true},
					{name: "raw-policy-native-ipv4", received: validPair, replacement: nativeIPv4, rawPolicy: true},
					{name: "raw-policy-vpn24", received: validPair, replacement: vpnSingle, rawPolicy: true},
					{name: "raw-policy-vpn48", received: validPair, replacement: vpnPair, rawPolicy: true},
					{name: "raw-policy-valid-pair-control", received: invalidFirst, replacement: validPair, rawPolicy: true, valid: true},
					{name: "raw-policy-empty", received: validPair, replacement: []byte{}, rawPolicy: true},
					{name: "raw-policy-eight-octets", received: validPair, replacement: make([]byte, 8), rawPolicy: true},
					{name: "raw-empty-repaired-global", received: validPair, replacement: []byte{}, rawPolicy: true, afterRaw: validPair[:16], valid: true},
					{name: "raw-eight-repaired-pair", received: validPair, replacement: make([]byte, 8), rawPolicy: true, afterRaw: validPair, valid: true},
					{name: "raw-eight-replaced-loopback", received: validPair, replacement: make([]byte, 8), rawPolicy: true, afterRaw: netip.IPv6Loopback().AsSlice()},
					{name: "raw-eight-replaced-invalid-second", received: validPair, replacement: make([]byte, 8), rawPolicy: true, afterRaw: invalidSecond},
					{name: "raw-empty-replaced-link-local-unnegotiated", received: validPair, replacement: []byte{}, rawPolicy: true,
						afterRaw: netip.MustParseAddr("fe80::9").AsSlice(), withoutCap77: true},
					{name: "raw-empty-replaced-link-local-cap77", received: validPair, replacement: []byte{}, rawPolicy: true,
						afterRaw: netip.MustParseAddr("fe80::9").AsSlice(), valid: true, onLinkOnly: true},
					{name: "cap77-source-invalid-second-withdrawn", received: invalidSecond, source77: true, withdrawn: true},
				} {
					if tc.offLinkOnly && onLink {
						// Direct untrimmed 48-byte AttrModSet is unsupported by
						// the handler; this case targets its real 48-to-24 trim.
						continue
					}
					t.Run(rail+"/"+scope+"/"+layout+"/"+tc.name, func(t *testing.T) {
						f := rfc2545ReceiveFixtureCaps(t, tc.source77, !tc.withoutCap77, onLink, true)
						policyCalls, afterRawCalls := 0, 0
						if tc.replacement != nil && !tc.rawPolicy {
							original := bytes.Clone(tc.replacement)
							t.Cleanup(func() {
								if !bytes.Equal(tc.replacement, original) {
									t.Error("forwarding changed the policy replacement")
								}
							})
							rewrite := func(_, _ filterapi.PeerFilterInfo, payload []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
								if len(payloadMPNextHopField(payload)) != 0 {
									policyCalls++
									if tc.earlier != nil {
										mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, tc.earlier)
									}
									mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, tc.replacement)
								}
								return true
							}
							f.r.orderedEgressSteps = orderedEgressStepsFromFuncs(rewrite)
							f.r.egressFilters = []filterapi.EgressFilterFunc{rewrite}
						}
						raw := mustHex(t, "4020010db800070000")
						attrs := mixedAttrs(mixedReach(1, tc.received, raw))
						var legacy []byte
						if mixed {
							attrs = append(attrs, 0x40, 3, 4)
							attrs = append(attrs, f.source.Settings().Address.AsSlice()...)
							legacy = mustHex(t, "180a0900")
						}
						body := buildUpdatePayload(attrs, legacy)
						dispatchRail := rail
						if tc.rawPolicy {
							// A raw export response replaces the whole offered
							// body, so retain its independent legacy section.
							overrideAttrs := mixedAttrs(mixedReach(1, tc.replacement, raw))
							if mixed {
								overrideAttrs = append(overrideAttrs, 0x40, 3, 4)
								overrideAttrs = append(overrideAttrs, f.source.Settings().Address.AsSlice()...)
							}
							override := buildUpdatePayload(overrideAttrs, legacy)
							f.r.api = &pluginserver.Server{}
							f.r.policyFilterSeam = func(_, _, _, _ string, _ uint32, text string) PolicyResponse {
								if strings.Contains(text, "ipv6/unicast") {
									policyCalls++
									return PolicyResponse{Action: PolicyModify, Raw: override}
								}
								return PolicyResponse{Action: PolicyAccept}
							}
							f.destination.settings.ExportPolicy = mustPolicy(t, []filterapi.FilterRef{{Name: "ipv6:raw-next-hop"}}, filterapi.FinalAccept)
							// Publish the policy without replacing this fixture's
							// explicit connected-prefix scope with host interfaces.
							f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
							f.r.orderedEgressSteps = []orderedEgressStep{{name: policyChainStepName, policyChain: true}}
							if tc.afterRaw != nil {
								// Operations are applied to the final raw base, even
								// though each in-process step observes received bytes.
								rewrite := func(_, _ filterapi.PeerFilterInfo, payload []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
									if payloadNextHop(payload).mpFamily == family.IPv6Unicast {
										afterRawCalls++
										mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, tc.afterRaw)
									}
									return true
								}
								f.r.orderedEgressSteps = append(f.r.orderedEgressSteps, orderedEgressStepsFromFuncs(rewrite)...)
							}
							if rail == "rs" {
								dispatchRail = "rs-export"
							}
						}
						updates, published := rfc2545ReceiveForwardPublished(t, f, dispatchRail, body, tc.withdrawn)
						if tc.withdrawn {
							// RFC 7606 Section 2 treat-as-withdraw takes every route in
							// the UPDATE, the legacy sibling included.
							rfc2545AssertWithdrawnAtReceipt(t, updates, published, raw, legacy)
							return
						}
						if tc.replacement != nil && policyCalls == 0 {
							t.Fatal("effective next-hop policy never ran")
						}
						if tc.afterRaw != nil && afterRawCalls == 0 {
							t.Fatal("replacement operation after raw export never ran")
						}
						wantCount := 1
						if mixed {
							wantCount = 2
						}
						if len(updates) != wantCount {
							t.Fatalf("recipient UPDATE count=%d, want %d independent sections", len(updates), wantCount)
						}
						wantHop := tc.received
						if tc.replacement != nil {
							wantHop = tc.replacement
						}
						if tc.afterRaw != nil {
							wantHop = tc.afterRaw
						}
						if !onLink && len(wantHop) == 32 {
							wantHop = wantHop[:16]
						}
						nativeCount, legacyCount := 0, 0
						for _, update := range updates {
							if len(update.NLRI) == 0 {
								nativeCount++
								rfc2545AssertNative(t, update, wantHop, raw, tc.valid && (!tc.onLinkOnly || onLink))
								continue
							}
							legacyCount++
							_, _, nh, hasNextHop := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
							if !mixed || !bytes.Equal(update.NLRI, legacy) || len(update.WithdrawnRoutes) != 0 ||
								!hasNextHop || !bytes.Equal(nh, f.source.Settings().Address.AsSlice()) {
								t.Errorf("legacy sibling changed: NLRI=%x withdrawals=%x next-hop=%x present=%v", update.NLRI, update.WithdrawnRoutes, nh, hasNextHop)
							}
							for _, code := range []attribute.AttributeCode{attribute.AttrMPReachNLRI, attribute.AttrMPUnreachNLRI} {
								if _, _, _, found := attribute.AttrFind(update.PathAttributes, code); found {
									t.Error("native reachability leaked into the legacy sibling")
								}
							}
						}
						wantLegacy := 0
						if mixed {
							wantLegacy = 1
						}
						if nativeCount != 1 || legacyCount != wantLegacy {
							t.Errorf("native sections=%d legacy sections=%d, want 1/%d", nativeCount, legacyCount, wantLegacy)
						}
					})
				}
			}
		}
	}
}

// RFC 2545 Section 3: "A BGP speaker shall advertise to its peer in the Network
// Address of Next Hop field the global IPv6 address of the next hop, potentially
// followed by the link-local IPv6 address of the next hop."
// RFC requirement: RFC2545-3-1 negative -- an unpaired IPv6-family next hop cannot use loopback as its global.
// RFC requirement: RFC2545-3-1 positive -- usable IPv6 globals, negotiated single link-local next hops, and native IPv4-family MP next hops retain their separate valid forms.
// RFC 8950 Section 1's separate single mapped field remains a valid control.
// MUTATION: validate only pairs, or apply IPv6-global restrictions to native IPv4
// next hops or the negotiated standalone link-local extension.
func TestRFC2545ReceivedSingleGlobalAddressValidated(t *testing.T) {
	for _, rail := range []string{"cached", "rs"} {
		for _, tc := range []struct {
			name, address, nlri string
			afi                 byte
			valid               bool
		}{
			{"loopback", "::1", "4020010db800070000", 2, false},
			{"ipv4-mapped", "::ffff:192.0.2.9", "4020010db800070000", 2, true},
			{"global-control", "2001:db8:1::9", "4020010db800070000", 2, true},
			{"cap77-link-local-control", "fe80::9", "4020010db800070000", 2, true},
			{"native-ipv4-control", "198.18.231.1", "180a0900", 1, true},
		} {
			t.Run(rail+"/"+tc.name, func(t *testing.T) {
				f := rfc2545ReceiveFixture(t, true, true, true)
				hop := netip.MustParseAddr(tc.address).AsSlice()
				raw := mustHex(t, tc.nlri)
				reach := append([]byte{0, tc.afi, 1, byte(len(hop))}, hop...)
				reach = append(reach, 0)
				reach = append(reach, raw...)
				body := buildUpdatePayload(mixedAttrs(mixedAttr(14, reach)), nil)
				updates := rfc2545ReceiveForward(t, f, rail, body)
				if len(updates) != 1 {
					t.Fatalf("recipient UPDATE count=%d, want one native decision", len(updates))
				}
				update := updates[0]
				_, _, gotReach, hasReach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
				_, _, unreach, hasUnreach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
				if tc.valid {
					if !hasReach || hasUnreach || !bytes.Equal(gotReach, reach) {
						t.Errorf("valid single next hop: reach=%x present=%v unreach=%x present=%v, want %x", gotReach, hasReach, unreach, hasUnreach, reach)
					}
				} else {
					want := mixedUnreachValue(uint16(tc.afi), 1, raw)
					if hasReach || !hasUnreach || !bytes.Equal(unreach, want) {
						t.Errorf("invalid single global: reach=%x present=%v unreach=%x present=%v, want withdrawal %x", gotReach, hasReach, unreach, hasUnreach, want)
					}
				}
				if len(update.NLRI) != 0 || len(update.WithdrawnRoutes) != 0 {
					t.Error("unexpected legacy NLRI alongside native route")
				}
			})
		}
	}
}

// RFC 9830 Section 2.1: "The next-hop network address field in SR Policy SAFI
// (73) updates may be either a 4-octet IPv4 address or a 16-octet IPv6 address,
// independent of the SR Policy AFI. The Length field of the next-hop address
// specifies the next-hop address family."
// This carrier deliberately fixes the NLRI at AFI 2 and changes the actual
// next-hop encoding, including policy replacement in both directions.
// MUTATION: infer next-hop encoding from NLRI AFI, erase it while unmapping,
// or keep the received encoding after an effective policy replacement.
func TestSRPolicyNextHopWireFamily(t *testing.T) {
	for _, rail := range []string{"cached", "rs"} {
		for _, tc := range []struct {
			name, received, replacement string
			valid                       bool
		}{
			{"received-ipv4", "192.0.2.9", "", true},
			{"received-ipv6", "2001:db8:1::9", "", true},
			{"received-mapped", "::ffff:192.0.2.9", "", false},
			{"rewrite-ipv4-to-ipv6", "192.0.2.9", "2001:db8:1::9", true},
			{"rewrite-ipv6-to-ipv4", "2001:db8:1::9", "192.0.2.9", true},
			{"rewrite-ipv4-to-mapped", "192.0.2.9", "::ffff:192.0.2.9", false},
			{"rewrite-mapped-to-ipv4", "::ffff:192.0.2.9", "192.0.2.9", true},
		} {
			t.Run(rail+"/"+tc.name, func(t *testing.T) {
				f := rfc2545ReceiveFixture(t, false, true, true)
				for _, peer := range []*Peer{f.source, f.destination} {
					caps := []capability.Capability{
						&capability.ASN4{ASN: peer.settings.LocalAS},
						&capability.Multiprotocol{AFI: family.AFIIPv6, SAFI: family.SAFISRPolicy},
					}
					neg := capability.Negotiate(caps, caps, capability.PeerIdentity{
						LocalASN: peer.settings.LocalAS, PeerASN: peer.settings.PeerAS,
					})
					peer.session.negotiated = neg
					peer.negotiated.Store(NewNegotiatedCapabilities(neg))
				}
				f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
				hop := netip.MustParseAddr(tc.received).AsSlice()
				wantHop := hop
				policyCalls := 0
				if tc.replacement != "" {
					wantHop = netip.MustParseAddr(tc.replacement).AsSlice()
					original := bytes.Clone(wantHop)
					t.Cleanup(func() {
						if !bytes.Equal(wantHop, original) {
							t.Error("forwarding changed the replacement next hop")
						}
					})
					rewrite := func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
						policyCalls++
						mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, wantHop)
						return true
					}
					f.r.orderedEgressSteps = orderedEgressStepsFromFuncs(rewrite)
					f.r.egressFilters = []filterapi.EgressFilterFunc{rewrite}
				}
				// RFC 9830 Sections 2.1 and 4.2.1: 192-bit policy key,
				// IPv4-format Route Target, and SR Policy Tunnel Type 15.
				raw := mustHex(t, "c0000000070000002a20010db8000000000000000000000001")
				tunnel := teSRPolicyValue(0, false)
				tunnelAttr := append([]byte{0xd0, byte(attribute.AttrTunnelEncap), byte(len(tunnel) >> 8), byte(len(tunnel))}, tunnel...)
				rt := []byte{0xc0, byte(attribute.AttrExtCommunity), 8, 1, 2, 192, 0, 2, 2, 0, 0}
				body := buildUpdatePayload(mixedAttrs(mixedReach(73, hop, raw), rt, tunnelAttr), nil)
				updates := rfc2545ReceiveForward(t, f, rail, body)
				if len(updates) != 1 {
					t.Fatalf("recipient UPDATE count=%d, want one native SR Policy decision", len(updates))
				}
				if tc.replacement != "" && policyCalls == 0 {
					t.Fatal("next-hop replacement policy never ran")
				}
				update := updates[0]
				_, _, reach, hasReach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
				_, _, unreach, hasUnreach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
				if tc.valid {
					want := append([]byte{0, 2, 73, byte(len(wantHop))}, wantHop...)
					want = append(want, 0)
					want = append(want, raw...)
					if !hasReach || hasUnreach || !bytes.Equal(reach, want) {
						t.Errorf("valid SR Policy next hop: reach=%x present=%v unreach=%x present=%v, want %x", reach, hasReach, unreach, hasUnreach, want)
					}
					_, _, sentTunnel, hasTunnel := attribute.AttrFind(update.PathAttributes, attribute.AttrTunnelEncap)
					if !hasTunnel || !bytes.Equal(sentTunnel, tunnel) {
						t.Errorf("SR Policy tunnel=%x present=%v, want %x", sentTunnel, hasTunnel, tunnel)
					}
				} else {
					want := mixedUnreachValue(2, 73, raw)
					if hasReach || !hasUnreach || !bytes.Equal(unreach, want) {
						t.Errorf("mapped IPv6 wire form: reach=%x present=%v unreach=%x present=%v, want withdrawal %x", reach, hasReach, unreach, hasUnreach, want)
					}
				}
				if len(update.NLRI) != 0 || len(update.WithdrawnRoutes) != 0 {
					t.Error("unexpected legacy NLRI alongside SR Policy")
				}
			})
		}
	}
}
