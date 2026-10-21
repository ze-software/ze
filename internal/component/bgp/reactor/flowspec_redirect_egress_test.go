// Design: docs/architecture/update-building.md -- the FlowSpec redirect-to-IP next hop on the forward rails.
// Related: flowspec_redirect_forward_wire_test.go -- the per-destination keep, reset and strip decisions.
package reactor

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var flowRedirectIPv4 = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}

// flowRedirectForward forwards one received FlowSpec UPDATE through
// forwardUpdateCore to an IBGP destination under next hop auto, after setup
// adjusts the fixture, and returns every UPDATE body the destination received.
func flowRedirectForward(t *testing.T, fam family.Family, raw, payload []byte, setup func(f *aigpReplayFixture)) [][]byte {
	t.Helper()
	f := newAIGPReplayFixture(t, nil)
	for _, peer := range []*Peer{f.source, f.destination} {
		peer.negotiated.Store(&NegotiatedCapabilities{ASN4: true, families: map[family.Family]bool{fam: true}})
	}
	f.destination.settings.PeerAS = 65000
	f.destination.settings.NextHopMode = NextHopAuto
	if setup != nil {
		setup(f)
	}
	f.destination.refreshForwardFacts()
	key := ribevents.ValidationRoute{Peer: f.source.Settings().Address, Family: fam, NLRI: string(raw)}
	ribevents.RegisterFlowSpecLookup(func(got ribevents.ValidationRoute, _ uint64) bool { return got == key }, nil, nil, nil)
	t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })
	id := f.receive(t, payload)
	f.forward(t, id)
	forwardSocketBarrier(t, f.r)
	return aigpSocketBodies(t, f.conn)
}

// flowRedirectPayloadWith is flowRedirectPayload with the extended communities
// given in hex, so a case can carry more than the one redirect action.
func flowRedirectPayloadWith(t *testing.T, fam family.Family, raw, hop []byte, communities string) []byte {
	t.Helper()
	ext := mustHex(t, communities)
	attrs := mustHex(t, "4001010040020602010000fdea")
	attrs = append(attrs, 0xc0, 16, byte(len(ext)))
	attrs = append(attrs, ext...)
	mp := []byte{byte(fam.AFI >> 8), byte(fam.AFI), byte(fam.SAFI), byte(len(hop))}
	mp = append(mp, hop...)
	mp = append(mp, 0)
	mp = append(mp, raw...)
	attrs = append(attrs, 0x80, 14, byte(len(mp)))
	attrs = append(attrs, mp...)
	body := binary.BigEndian.AppendUint16([]byte{0, 0}, uint16(len(attrs)))
	return append(body, attrs...)
}

// flowRedirectNextHop returns the next-hop field of the one announcement in
// bodies, failing unless exactly one MP_REACH_NLRI was sent.
func flowRedirectNextHop(t *testing.T, bodies [][]byte) []byte {
	t.Helper()
	var hop []byte
	reaches := 0
	for _, body := range bodies {
		update, err := message.UnpackUpdate(body)
		if err != nil {
			t.Fatal(err)
		}
		_, _, mp, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
		if !found {
			continue
		}
		reaches++
		if len(mp) < 4 || 4+int(mp[3]) > len(mp) {
			t.Fatalf("malformed MP_REACH %x", mp)
		}
		hop = mp[4 : 4+int(mp[3])]
	}
	if reaches != 1 {
		t.Fatalf("recipient announcements=%d in %d UPDATEs, want one", reaches, len(bodies))
	}
	return hop
}

// TestFlowSpecRedirectEgressStripVoidsNextHop proves a destination whose egress
// filter strips the redirect-to-IP community also loses the next hop.
//
// RFC 8955 Section 4: "When advertising Flow Specifications, the Length of the
// Next-Hop Network Address MUST be set to 0." The redirect address is only the
// exception while the 0x0800 action leaves with the route.
//
// Method: an IBGP destination under next hop auto (which keeps the address)
// has an egress step recording the community filter's Remove of
// 0800000000000000. The rule leaves with length 0. A Remove of a value the
// route does not carry, and a Remove that leaves a second (C=1) redirect
// community behind, both keep 192.0.2.77.
//
// MUTATION: drop the redirectActionLeaves test from applyRedirectNextHop's keep
// case (the stripped route leaves with c000024d).
func TestFlowSpecRedirectEgressStripVoidsNextHop(t *testing.T) {
	raw := mustHex(t, "050118c00002")
	hop := netip.MustParseAddr("192.0.2.77").AsSlice()
	for _, tc := range []struct {
		name        string
		communities string
		remove      string
		want        []byte
	}{
		{"strip-voids", flowRedirectAction, flowRedirectAction, nil},
		{"other-value-keeps", flowRedirectAction, "0800000000000001", hop},
		{"second-action-keeps", flowRedirectAction + "0800000000000001", flowRedirectAction, hop},
	} {
		t.Run(tc.name, func(t *testing.T) {
			remove := mustHex(t, tc.remove)
			bodies := flowRedirectForward(t, flowRedirectIPv4, raw, flowRedirectPayloadWith(t, flowRedirectIPv4, raw, hop, tc.communities), func(f *aigpReplayFixture) {
				f.r.orderedEgressSteps = orderedEgressStepsFromFuncs(func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
					mods.Op(uint8(attribute.AttrExtCommunity), filterapi.AttrModRemove, remove)
					return true
				})
			})
			if got := flowRedirectNextHop(t, bodies); !bytes.Equal(got, tc.want) {
				t.Fatalf("recipient next hop=%x, want %x", got, tc.want)
			}
		})
	}
}

// TestFlowSpecRedirectIgnoredCases proves the received next hop never leaves
// where the redirect-to-IP action is void, and leaves where it is not.
//
// draft-simpson-idr-flowspec-redirect-02 Section 3: "If an MP_REACH_NLRI
// containing one or more flow-spec NLRI does not have a valid IPv4 or IPv6
// address in its next-hop field, or the length of the next-hop is 0, then the
// redirect-to-IP extended community, if present, should be ignored", and "the
// redirect-to-IP extended community is valid with any other set of flow-spec
// extended communities except if that set includes a redirect-to-VRF extended
// community (type 0x8008) and in that case the redirect-to-IP extended
// community should be ignored."
//
// Method: an IBGP destination under next hop auto, which keeps a valid
// redirect address. A 12-octet (RD + IPv4) and a 32-octet (IPv6 pair) field
// are not one address, and a 0x8008 beside the 0x0800 voids the action: each
// leaves with length 0. A flow-vpn rule (SAFI 134) with a 4-octet address
// keeps it, the draft naming VPNv4 and VPNv6 flow-spec routes as carriers.
//
// MUTATION: return true from FlowSpecRedirectsToNextHop on 0x8008 (the 8008
// case keeps c000024d); accept a 12- or 32-octet field in
// flowSpecRedirectNextHop (those cases keep the field).
func TestFlowSpecRedirectIgnoredCases(t *testing.T) {
	vpn := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpecVPN}
	v4 := netip.MustParseAddr("192.0.2.77").AsSlice()
	rd12 := append(make([]byte, 8), v4...)
	pair32 := append(netip.MustParseAddr("2001:db8::77").AsSlice(), netip.MustParseAddr("fe80::77").AsSlice()...)
	for _, tc := range []struct {
		name        string
		fam         family.Family
		nlri        string
		hop         []byte
		communities string
		want        []byte
	}{
		{"rd-ipv4-12-octets", flowRedirectIPv4, "050118c00002", rd12, flowRedirectAction, nil},
		{"ipv6-pair-32-octets", flowRedirectIPv4, "050118c00002", pair32, flowRedirectAction, nil},
		{"redirect-to-vrf-voids", flowRedirectIPv4, "050118c00002", v4, flowRedirectAction + "8008fdea00000001", nil},
		{"flow-vpn-keeps", vpn, "0d0000fdea000000010118c00002", v4, flowRedirectAction, v4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := mustHex(t, tc.nlri)
			bodies := flowRedirectForward(t, tc.fam, raw, flowRedirectPayloadWith(t, tc.fam, raw, tc.hop, tc.communities), nil)
			if got := flowRedirectNextHop(t, bodies); !bytes.Equal(got, tc.want) {
				t.Fatalf("recipient next hop=%x, want %x", got, tc.want)
			}
		})
	}
}

// TestFlowSpecRedirectPeerOwnWithheld proves a kept redirect address that is
// the destination's own address is never sent to it.
//
// RFC 4271 Section 5.1.3: "A route originated by a BGP speaker SHALL NOT be
// advertised to a peer using an address of that peer as NEXT_HOP." A redirect
// target equal to the destination sends the matching packets back into it.
//
// Method: an IBGP destination under next hop auto keeps the received address,
// which is the destination's own: no announcement reaches it. The same route
// with another address is announced, so the fixture can deliver.
//
// MUTATION: drop the redirectNextHopIsPeerOwn test from egressNextHopWithheld
// (the destination receives its own address as the redirect target).
func TestFlowSpecRedirectPeerOwnWithheld(t *testing.T) {
	raw := mustHex(t, "050118c00002")
	own := netip.MustParseAddr("198.18.232.2").AsSlice() // newAIGPReplayFixture's destination
	t.Run("own-address-withheld", func(t *testing.T) {
		for _, body := range flowRedirectForwardPayload(t, raw, own) {
			update, err := message.UnpackUpdate(body)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, mp, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI); found {
				t.Fatalf("destination was sent its own address as the redirect target: %x", mp)
			}
		}
	})
	t.Run("other-address-sent", func(t *testing.T) {
		hop := netip.MustParseAddr("192.0.2.77").AsSlice()
		if got := flowRedirectNextHop(t, flowRedirectForwardPayload(t, raw, hop)); !bytes.Equal(got, hop) {
			t.Fatalf("recipient next hop=%x, want %x", got, hop)
		}
	})
}

// flowRedirectForwardPayload forwards an IPv4 redirect rule with hop.
func flowRedirectForwardPayload(t *testing.T, raw, hop []byte) [][]byte {
	t.Helper()
	return flowRedirectForward(t, flowRedirectIPv4, raw, flowRedirectPayload(t, flowRedirectIPv4, raw, hop), nil)
}

// TestPrecomputeRedirectNextHopAddressFamily proves the EBGP reset writes only
// a usable address of the rule's own family.
//
// draft-simpson-idr-flowspec-redirect-02 Section 3: "if an IPv4 [or IPv6]
// packet with destination address D that is normally forwarded to a next-hop A
// matches a filter entry of the type described above it MUST instead be
// redirected (C=0) or mirrored (C=1) to next-hop B, where B is found by FIB
// lookup of the IPv4 [or IPv6] address contained in the MP_REACH_NLRI next-hop
// field".
//
// Method: precomputeRedirectNextHop over an EBGP destination under next hop
// auto. An IPv6 session address serves IPv6 rules and leaves IPv4 rules with
// no address (zero length); an IPv4 one the reverse; a link-local-only session
// has no address to reset to, so the decision is the zero length rather than
// fe80::, which the link-local gates would refuse the whole rule for.
//
// MUTATION: return the 16-octet address for an IPv4 rule from hop; accept a
// link-local address in add.
func TestPrecomputeRedirectNextHopAddressFamily(t *testing.T) {
	for _, tc := range []struct {
		name   string
		local  string
		action redirectAction
		want4  int
		want6  int
	}{
		{"ipv6-session", "2001:db8::1", redirectRewrite, 0, 16},
		{"ipv4-session", "10.0.0.1", redirectRewrite, 4, 0},
		{"link-local-only", "fe80::1", redirectStrip, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &peerForwardFacts{isEBGP: true, connectedLocal: netip.MustParseAddr(tc.local)}
			precomputeRedirectNextHop(&PeerSettings{NextHopMode: NextHopAuto}, f)
			if f.redirect.action != tc.action {
				t.Fatalf("action=%d, want %d", f.redirect.action, tc.action)
			}
			if got := len(f.redirect.hop(family.AFIIPv4)); got != tc.want4 {
				t.Errorf("IPv4 rule next hop=%d octets, want %d", got, tc.want4)
			}
			if got := len(f.redirect.hop(family.AFIIPv6)); got != tc.want6 {
				t.Errorf("IPv6 rule next hop=%d octets, want %d", got, tc.want6)
			}
		})
	}
}

// TestRelayFlowSpecRedirectNextHop proves the selecting-RIB relay puts a
// redirect-to-IP route's received next hop back, byte for byte.
//
// draft-simpson-idr-flowspec-redirect-02 Section 3 sends matching packets
// "towards the IPv4 or IPv6 address specified in the 'Network Address of
// Next-Hop' field of the associated MP_REACH_NLRI". A FlowSpec StoredRoute
// carries no NextHopHex, so the field comes from its stored MP_REACH_NLRI
// (relayFlowSpecNextHop).
//
// Method: RelayStoredRoute replays an authorized redirect rule to a
// route-server client, which keeps the next hop (RFC 7947 Section 2.2.1), and
// the replayed MP_REACH_NLRI is compared byte for byte.
//
// MUTATION: drop the relayFlowSpecNextHop branch of buildRelayUpdate (the
// replay carries length 0).
func TestRelayFlowSpecRedirectNextHop(t *testing.T) {
	api, src, dst, capture := validationForwardFixture(t, false)
	for _, peer := range []*Peer{src, dst} {
		peer.negotiated.Store(&NegotiatedCapabilities{families: map[family.Family]bool{flowRedirectIPv4: true}})
		peer.refreshForwardFacts()
	}
	raw := mustHex(t, "050118c00002")
	hop := netip.MustParseAddr("192.0.2.77").AsSlice()
	payload := flowRedirectPayload(t, flowRedirectIPv4, raw, hop)
	attrs := payload[4:]
	update, id := newLeakTestUpdate(t, api.r.recentUpdates, payload, src.recvCtxID)
	update.WireUpdate.SetSourceID(src.SourceID())
	key := ribevents.ValidationRoute{Peer: src.Settings().Address, Family: flowRedirectIPv4, NLRI: string(raw)}
	ribevents.RegisterFlowSpecLookup(func(got ribevents.ValidationRoute, _ uint64) bool { return got == key },
		func(got ribevents.ValidationRoute) bool { return got == key }, nil, nil)
	t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })
	route := rpc.StoredRoute{SourcePeer: src.Settings().Address.String(), Family: flowRedirectIPv4.String(),
		NLRIHex: hex.EncodeToString(raw), AttrHex: hex.EncodeToString(attrs), MsgID: id, NLRIFraming: rpc.NLRIFramingPrefixOnly}
	if err := api.RelayStoredRoute(dst.Settings().Address, []rpc.StoredRoute{route}, plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
	if got := flowRedirectNextHop(t, validationBodies(t, api, capture)); !bytes.Equal(got, hop) {
		t.Fatalf("replayed next hop=%x, want %x", got, hop)
	}
}
