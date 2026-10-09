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
// IPv6 destination addresses and a joint common-prefix predicate.
// RFC 2545 Section 3: "The link-local address shall be included in the Next Hop
// field if and only if the BGP speaker shares a common subnet with the entity
// identified by the global IPv6 address carried in the Network Address of Next
// Hop field and the peer the route is being advertised to."
// RFC requirement: RFC2545-3-3 positive -- a received global/link-local pair remains intact when the recipient and effective Global share one connected prefix.
// RFC requirement: RFC2545-3-3 negative -- either off-link entity removes only the link-local half, preserving the global address and native NLRI.
// MUTATION: bypass either the received-global subnet check or the destination
// subnet check in applyEgressNextHopScope; alternatively strip every pair.
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
			name       string
			peerOnLink bool
			global     string
			keepPair   bool
		}{
			{"both-on-link", true, "2001:db8:1::9", true},
			{"global-off-link", true, "2001:db8:2::9", false},
			{"peer-off-link", false, "2001:db8:1::9", false},
			{"both-off-link", false, "2001:db8:2::9", false},
		} {
			t.Run(native.name+"/"+scope.name, func(t *testing.T) {
				f := newAIGPReplayFixture(t, nil)
				for _, peer := range []*Peer{f.source, f.destination} {
					peer.negotiated.Store(&NegotiatedCapabilities{ASN4: true, families: map[family.Family]bool{native.fam: true}})
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
				wantHop := pair[:globalOctets]
				if scope.keepPair {
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
			capability := "without-capability"
			if negotiated {
				capability = "with-capability"
			}
			t.Run(capability+"/"+tc.name, func(t *testing.T) {
				f := newAIGPReplayFixture(t, nil)
				for _, peer := range []*Peer{f.source, f.destination} {
					peer.negotiated.Store(&NegotiatedCapabilities{
						ASN4: true, LinkLocalNextHop: negotiated,
						families: map[family.Family]bool{family.IPv6Unicast: true},
					})
				}
				f.destination.settings.NextHopMode = NextHopUnchanged
				f.destination.settings.PeerAS = 65002
				rfc2545ForwardScope(f, true, true)
				hop := netip.MustParseAddr(tc.hop).AsSlice()
				if tc.pair {
					hop = append(hop, netip.MustParseAddr("fe80::9").AsSlice()...)
				}
				raw := mustHex(t, "4020010db800070000")
				id := f.receive(t, buildUpdatePayload(mixedAttrs(mixedReach(1, hop, raw)), nil))
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
// RFC requirement: RFC2545-3-1 negative -- a usable first global never licenses a non-link-local second address, whether the pair would otherwise be preserved or trimmed.
// RFC requirement: RFC2545-3-1 positive -- a global/link-local pair remains usable, and capability 77 retains its separate negotiated single-address exception.
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
				}{
					{"second-global", "2001:db8:1::a", false, false},
					{"second-unspecified", "::", false, false},
					{"second-multicast", "ff02::1", false, false},
					{"second-link-local", "fe80::9", true, false},
					{"link-local-only-control", "", true, true},
				} {
					t.Run(rail+"/"+caps.name+"/"+scope.name+"/"+tc.name, func(t *testing.T) {
						f := newAIGPReplayFixture(t, nil)
						f.destination.settings.NextHopMode = NextHopUnchanged
						f.destination.settings.PeerAS = 65002
						for _, peer := range []*Peer{f.source, f.destination} {
							localLL, remoteLL := true, true
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
						// Receive always negotiates capability 77 so the single-address
						// control reaches egress. Only destination advertisements vary.
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
						original := bytes.Clone(body)
						defer func() {
							if !bytes.Equal(body, original) {
								t.Error("receive or forwarding changed the original UPDATE")
							}
						}()
						var id uint64
						calls := 0
						f.source.session.onMessageReceived = func(addr netip.Addr, typ msgtype.MessageType, received []byte,
							wu *wireu.WireUpdate, ctxID bgpctx.ContextID, direction rpc.MessageDirection,
							buf BufHandle, meta map[string]any, sourcePeer string, sourceMessageID uint64) bool {
							calls++
							if typ != msgtype.TypeUPDATE || wu == nil || !bytes.Equal(wu.Payload(), original) {
								t.Fatal("Session did not publish the intact native announcement")
							}
							kept := f.r.notifyMessageReceiver(addr, typ, received, wu, ctxID, direction, buf, meta, sourcePeer, sourceMessageID)
							// Publication assigns the received ID. The fixture consumer
							// keeps it alive until this explicit assertion-lifetime retain.
							id = wu.MessageID()
							f.ids = append(f.ids, id)
							if !f.r.recentUpdates.Retain(id) {
								t.Fatalf("published update %d missing before fixture retain", id)
							}
							t.Cleanup(func() { f.r.recentUpdates.Release(id) })
							return kept
						}
						header := message.Header{Type: msgtype.TypeUPDATE, Length: uint16(message.HeaderLen + len(body))}
						err, kept := f.source.session.processMessage(&header, body, BufHandle{ID: noPoolBufID, Buf: body})
						if err != nil || !kept || calls != 1 {
							t.Fatalf("Session receive: err=%v kept=%v publications=%d, want one cached announcement", err, kept, calls)
						}
						if rail == "rs" {
							received, found := f.r.recentUpdates.Get(id)
							if !found {
								t.Fatal("Session-delivered update missing before RS dispatch")
							}
							skipped, sent := reactorForwardRS(f.r, received, id, f.source.Settings().Address, f.source)
							if len(skipped) != 0 || sent != 1 {
								t.Fatalf("RS forwarded=%d skipped=%v, want one direct-rail decision", sent, skipped)
							}
							f.source.session.flushFwdDirty()
						} else {
							f.forward(t, id)
						}
						forwardSocketBarrier(t, f.r)
						bodies := aigpSocketBodies(t, f.conn)
						if len(bodies) != 1 {
							t.Fatalf("recipient UPDATE count=%d, want one native announcement or withdrawal", len(bodies))
						}
						update, err := message.UnpackUpdate(bodies[0])
						if err != nil {
							t.Fatal(err)
						}
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

// rfc2545ReceiveFixture negotiates both native families so a mixed legacy/MP
// input crosses the same Session boundary as a standalone IPv6 announcement.
func rfc2545ReceiveFixture(t *testing.T, cap77, peerOnLink, globalOnLink bool) *aigpReplayFixture {
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
		if peer == f.source || cap77 {
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
func rfc2545ReceiveForward(t *testing.T, f *aigpReplayFixture, rail string, body []byte) []*message.Update {
	t.Helper()
	original := bytes.Clone(body)
	t.Cleanup(func() {
		if !bytes.Equal(body, original) {
			t.Error("receive or forwarding changed the original UPDATE")
		}
	})
	var id uint64
	calls := 0
	f.source.session.onMessageReceived = func(addr netip.Addr, typ msgtype.MessageType, received []byte,
		wu *wireu.WireUpdate, ctxID bgpctx.ContextID, direction rpc.MessageDirection,
		buf BufHandle, meta map[string]any, sourcePeer string, sourceMessageID uint64) bool {
		calls++
		if typ != msgtype.TypeUPDATE || wu == nil || !bytes.Equal(wu.Payload(), original) {
			t.Fatal("Session did not publish the intact native announcement")
		}
		kept := f.r.notifyMessageReceiver(addr, typ, received, wu, ctxID, direction, buf, meta, sourcePeer, sourceMessageID)
		id = wu.MessageID()
		f.ids = append(f.ids, id)
		if !f.r.recentUpdates.Retain(id) {
			t.Fatalf("published update %d missing before fixture retain", id)
		}
		t.Cleanup(func() { f.r.recentUpdates.Release(id) })
		return kept
	}
	header := message.Header{Type: msgtype.TypeUPDATE, Length: uint16(message.HeaderLen + len(body))}
	err, kept := f.source.session.processMessage(&header, body, BufHandle{ID: noPoolBufID, Buf: body})
	if err != nil || !kept || calls != 1 {
		t.Fatalf("Session receive: err=%v kept=%v publications=%d, want one cached announcement", err, kept, calls)
	}
	if rail == "rs" || rail == "rs-export" {
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
	} else {
		f.forward(t, id)
	}
	forwardSocketBarrier(t, f.r)
	var updates []*message.Update
	for _, body := range aigpSocketBodies(t, f.conn) {
		update, err := message.UnpackUpdate(body)
		if err != nil {
			t.Fatal(err)
		}
		updates = append(updates, update)
	}
	return updates
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
// RFC requirement: RFC2545-3-1 negative -- a valid second link-local cannot license an unusable first address, even when trimming would hide the original pair.
// RFC requirement: RFC2545-3-1 positive -- a usable global first address retains the valid pair or only its global half according to configured prefix predicates.
// MUTATION: check only unspecified/multicast first addresses, or permit a
// paired link-local first address under capability 77's single-address exception.
func TestRFC2545ReceivedPairFirstAddressValidated(t *testing.T) {
	for _, rail := range []string{"cached", "rs"} {
		for _, cap77 := range []bool{false, true} {
			capName := "cap77-neither"
			if cap77 {
				capName = "cap77-both"
			}
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
				}{
					{"link-local", "fe80::9", false},
					{"ipv4-mapped", "::ffff:192.0.2.9", false},
					{"loopback", "::1", false},
					{"unspecified", "::", false},
					{"multicast", "ff02::1", false},
					{"global-control", "2001:db8:1::9", true},
				} {
					t.Run(rail+"/"+capName+"/"+scope.name+"/"+tc.name, func(t *testing.T) {
						f := rfc2545ReceiveFixture(t, cap77, scope.peerOnLink, scope.globalOnLink)
						first := netip.MustParseAddr(tc.first).As16()
						hop := append(first[:], netip.MustParseAddr("fe80::a").AsSlice()...)
						raw := mustHex(t, "4020010db800070000")
						body := buildUpdatePayload(mixedAttrs(mixedReach(1, hop, raw)), nil)
						updates := rfc2545ReceiveForward(t, f, rail, body)
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
// RFC requirement: RFC2545-3-1 negative -- an effective invalid pair is withdrawn after policy, without withdrawing an independently valid legacy sibling.
// RFC requirement: RFC2545-3-1 positive -- a valid policy replacement of an obsolete malformed received pair is announced with exact native bytes.
// RFC requirement: RFC2545-3-2 negative -- effective native IPv4 and VPN-shaped next-hop fields cannot leave under IPv6-unicast AFI/SAFI, including policy pairs whose off-link trim would hide their original width.
// RFC requirement: RFC2545-3-2 positive -- a later legal sixteen- or thirty-two-octet policy replacement supersedes an obsolete wrong-width operation, with valid legacy siblings unchanged.
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
				} {
					if tc.offLinkOnly && onLink {
						// Direct untrimmed 48-byte AttrModSet is unsupported by
						// the handler; this case targets its real 48-to-24 trim.
						continue
					}
					t.Run(rail+"/"+scope+"/"+layout+"/"+tc.name, func(t *testing.T) {
						f := rfc2545ReceiveFixture(t, !tc.withoutCap77, onLink, true)
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
							f.destination.settings.ExportFilters = []filterapi.FilterRef{{Name: "ipv6:raw-next-hop"}}
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
						updates := rfc2545ReceiveForward(t, f, dispatchRail, body)
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
