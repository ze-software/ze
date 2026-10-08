// Design: docs/architecture/update-building.md -- family-specific next-hop admission.
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestFamilyNextHopOriginationAdmission drives actual API, queued and commit
// producers and their final export writer. RFC 8950 Section 3 incorporates the
// RFC 2545 Section 3 address roles; RFC 9830 Section 2.1 selects roles by wire
// width, not policy AFI. Literal NLRI and next-hop bytes are the recipient oracle.
//
// RFC requirement: RFC2545-3-1 positive -- actual batch, queued and commit
// producers and their export overrides retain genuine IPv6 global fields.
// Native IPv4, mapped labeled and capability-77 cases are separate controls.
// RFC requirement: RFC2545-3-1 negative -- the same producer and final-writer
// paths refuse unusable IPv6 global-address roles without advertising routes.
// RFC requirement: RFC2545-3-2 positive -- global-only announcements retain
// the exact sixteen-octet IPv6 next-hop field.
// RFC requirement: RFC2545-3-2 negative -- native IPv6 multicast and labeled
// announcements cannot substitute a four-octet next-hop field.
// RFC requirement: RFC8950-4-1 positive -- IPv4 unicast, multicast and labeled
// routes advertise IPv6 next hops under their exact negotiated capability pair.
// RFC requirement: RFC8950-4-1 negative -- absent or wrong-pair capability
// prevents IPv6 next-hop advertisement, including export replacements and
// capability 77 without the required capability 5.
func TestFamilyNextHopOriginationAdmission(t *testing.T) {
	for _, fc := range []struct {
		name     string
		fam      family.Family
		raw      string
		extended bool
		native4  bool
		mapped   bool
	}{
		{"ipv6-multicast", family.Family{AFI: 2, SAFI: 2}, "4020010db800070000", false, false, true},
		{"ipv6-labeled", family.Family{AFI: 2, SAFI: 4}, "5800064120010db800070000", false, false, true},
		{"ipv4-unicast", family.IPv4Unicast, "18cb0071", true, true, false},
		{"ipv4-multicast", family.Family{AFI: 1, SAFI: 2}, "18cb0071", true, true, false},
		{"ipv4-labeled", family.Family{AFI: 1, SAFI: 4}, "30000641cb0071", true, true, false},
		{"ipv4-policy", family.Family{AFI: 1, SAFI: 73}, "60000000070000002ac0000209", false, true, false},
		{"ipv6-policy", family.Family{AFI: 2, SAFI: 73}, "c0000000070000002a20010db8000000000000000000000009", false, true, false},
	} {
		for _, rail := range []string{"batch", "queued", "commit"} {
			for _, postPolicy := range []bool{false, true} {
				stage := "producer"
				if postPolicy {
					stage = "export"
				}
				for _, tc := range []struct {
					name, hop                       string
					valid, cap77, noCap5, wrongCap5 bool
				}{
					{name: "global", hop: "2001:db8:1::1", valid: true},
					{name: "loopback", hop: "::1"},
					{name: "unspecified", hop: "::"},
					{name: "multicast", hop: "ff0e::1"},
					{name: "mapped", hop: "::ffff:192.0.2.9", valid: fc.mapped},
					{name: "native-ipv4", hop: "192.0.2.9", valid: fc.native4},
					{name: "link-local-cap77", hop: "fe80::9", valid: true, cap77: true},
					{name: "global-no-cap5", hop: "2001:db8:1::1", valid: !fc.extended, noCap5: true},
					{name: "global-wrong-cap5", hop: "2001:db8:1::1", valid: !fc.extended, wrongCap5: true},
					{name: "link-local-cap77-no-cap5", hop: "fe80::9", valid: !fc.extended, cap77: true, noCap5: true},
				} {
					t.Run(fc.name+"/"+rail+"/"+stage+"/"+tc.name, func(t *testing.T) {
						peer, conn := newInitialSyncPeer(t, true, fc.fam)
						peer.settings.Address = netip.MustParseAddr("2001:db8:2::2")
						caps := []capability.Capability{&capability.ASN4{ASN: 65000}, &capability.Multiprotocol{AFI: fc.fam.AFI, SAFI: fc.fam.SAFI}}
						if tc.cap77 {
							caps = append(caps, &capability.LinkLocalNextHop{})
						}
						if fc.extended && !tc.noCap5 {
							safi := capability.SAFI(fc.fam.SAFI)
							if tc.wrongCap5 {
								safi = capability.SAFIVPN
							}
							caps = append(caps, &capability.ExtendedNextHop{Families: []capability.ExtendedNextHopFamily{{NLRIAFI: capability.AFIIPv4, NLRISAFI: safi, NextHopAFI: capability.AFIIPv6}}})
						}
						neg := capability.Negotiate(caps, caps, capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001})
						peer.negotiated.Store(NewNegotiatedCapabilities(neg))
						peer.session.negotiated = neg
						peer.sendCtx.Store(bgpctx.NewEncodingContext(&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001}, neg.Encoding, bgpctx.DirectionSend))
						peer.state.Store(int32(PeerStateEstablished))
						peer.sendingInitialRoutes.Store(0)
						api := groupUpdatesReactor([]*Peer{peer}, false)
						raw := mustHex(t, fc.raw)
						routeNLRI, err := nlri.NewWireNLRI(fc.fam, raw, false)
						if err != nil {
							t.Fatal(err)
						}
						hop := netip.MustParseAddr(tc.hop)
						inputHop := hop
						if postPolicy {
							inputHop = netip.MustParseAddr("2001:db8:1::1")
							if fc.native4 {
								inputHop = netip.MustParseAddr("192.0.2.1")
							}
							mp := append([]byte{0, byte(fc.fam.AFI), byte(fc.fam.SAFI), byte(len(hop.AsSlice()))}, hop.AsSlice()...)
							mp = append(mp, 0)
							mp = append(mp, raw...)
							body := buildUpdatePayload(mixedAttrs(append([]byte{0x80, 14, byte(len(mp))}, mp...)), nil)
							peer.session.egressRouteFilter = func([]byte) (bool, []byte) { return false, body }
						}
						batch := bgptypes.NLRIBatch{Family: fc.fam, NLRIs: []nlri.NLRI{routeNLRI}, NextHop: bgptypes.NewNextHopExplicit(inputHop)}
						if rail == "queued" {
							peer.sendingInitialRoutes.Store(1)
							peer.initialSyncEOROwed.Store(true)
						}
						if rail == "commit" {
							result, commitErr := api.SendRoutes(selector.All(), []*rib.Route{rib.NewRouteWithASPath(routeNLRI, inputHop, nil, nil)}, nil, true, plugin.OperatorSender())
							if commitErr != nil {
								t.Fatal(commitErr)
							}
							want := 0
							if tc.valid {
								want = 1
							}
							if result.RoutesAnnounced != want {
								t.Errorf("announced=%d want=%d", result.RoutesAnnounced, want)
							}
						} else {
							err = api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender())
							if tc.valid && err != nil {
								t.Errorf("valid announcement refused: %v", err)
							}
							if rail == "queued" {
								peer.sendInitialRoutes()
							}
						}
						announcements := 0
						for _, update := range recoveryWrittenUpdates(t, conn.written()) {
							if update.IsEndOfRIBAnyFamily() {
								continue
							}
							announcements++
							_, _, mp, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
							if fc.fam == family.IPv4Unicast && hop.Is4() && !postPolicy {
								_, _, nh, present := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
								if !present || !bytes.Equal(nh, hop.AsSlice()) || !bytes.Equal(update.NLRI, raw) {
									t.Errorf("legacy next-hop=%x NLRI=%x", nh, update.NLRI)
								}
							} else {
								want := append([]byte{0, byte(fc.fam.AFI), byte(fc.fam.SAFI), byte(len(hop.AsSlice()))}, hop.AsSlice()...)
								want = append(want, 0)
								want = append(want, raw...)
								if !found || !bytes.Equal(mp, want) {
									t.Errorf("MP_REACH=%x want=%x", mp, want)
								}
							}
						}
						want := 0
						if tc.valid {
							want = 1
						}
						if announcements != want {
							t.Errorf("delivered announcements=%d want=%d", announcements, want)
						}
					})
				}
			}
		}
	}
}

// TestFamilyConfiguredBuilderAdmission exercises configured multicast and labeled
// builders, without relying on a final Session guard to cover their own bypass.
//
// RFC requirement: RFC2545-3-1 positive -- configured IPv6 multicast and
// labeled builders retain the literal global next-hop bytes.
// RFC requirement: RFC2545-3-1 negative -- those builders return no UPDATE
// for loopback, unspecified or multicast global-address roles.
func TestFamilyConfiguredBuilderAdmission(t *testing.T) {
	for _, safi := range []attribute.SAFI{attribute.SAFIMulticast, attribute.SAFIMPLSLabel} {
		for _, hop := range []string{"::1", "::", "ff0e::1", "2001:db8:1::1"} {
			t.Run((family.Family{AFI: 2, SAFI: family.SAFI(safi)}).String()+"/"+hop, func(t *testing.T) {
				builder := message.NewUpdateBuilder(65000, false, true, false)
				var update *message.Update
				if safi == attribute.SAFIMulticast {
					update, _ = builder.BuildUnicast(&message.UnicastParams{Prefix: netip.MustParsePrefix("2001:db8:7::/64"), SAFI: safi, NextHop: netip.MustParseAddr(hop)})
				} else {
					update = builder.BuildLabeledUnicast(&message.LabeledUnicastParams{Prefix: netip.MustParsePrefix("2001:db8:7::/64"), Labels: []uint32{100}, NextHop: netip.MustParseAddr(hop)})
				}
				if hop == "2001:db8:1::1" {
					if update == nil {
						t.Fatal("valid configured route refused")
					}
					_, _, mp, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
					if !found || len(mp) < 20 || !bytes.Equal(mp[4:20], netip.MustParseAddr(hop).AsSlice()) {
						t.Fatalf("next-hop wire=%x", mp)
					}
				} else if update != nil {
					t.Errorf("unusable configured next-hop %s encoded", hop)
				}
			})
		}
	}
}

// TestFamilyNextHopForwardAdmission inspects real cached and route-server output
// after policy width changes. A later repair and an independent legacy sibling
// must survive; RFC 4798's mapped labeled field is a separate positive control.
//
// RFC requirement: RFC2545-3-1 positive -- cached and route-server output
// retains an effective global next hop after policy, including the global
// half of a valid pair and a later valid replacement.
// RFC requirement: RFC2545-3-1 negative -- effective loopback next hops are
// withheld while the independently valid legacy sibling survives.
// RFC requirement: RFC2545-3-2 positive -- valid plain policy fields leave
// with the exact sixteen-byte global half; this does not prove pair retention.
// RFC requirement: RFC2545-3-2 negative -- VPN-shaped 24/48-byte policy fields
// cannot substitute for plain fields; a later valid repair still takes effect.
func TestFamilyNextHopForwardAdmission(t *testing.T) {
	global := netip.MustParseAddr("2001:db8:1::9").AsSlice()
	linkLocal := netip.MustParseAddr("fe80::a").AsSlice()
	pair := append(bytes.Clone(global), linkLocal...)
	for _, fc := range []struct {
		name            string
		fam             family.Family
		raw             string
		native4, mapped bool
	}{
		{"ipv6-multicast", family.Family{AFI: 2, SAFI: 2}, "4020010db800070000", false, true},
		{"ipv6-labeled", family.Family{AFI: 2, SAFI: 4}, "5800064120010db800070000", false, true},
		{"ipv4-labeled", family.Family{AFI: 1, SAFI: 4}, "30000641cb0071", true, false},
		{"ipv4-policy", family.Family{AFI: 1, SAFI: 73}, "60000000070000002ac0000209", true, false},
		{"ipv6-policy", family.Family{AFI: 2, SAFI: 73}, "c0000000070000002a20010db8000000000000000000000009", true, false},
	} {
		for _, rail := range []string{"cached", "rs"} {
			for _, tc := range []struct {
				name                 string
				replacement, earlier []byte
				valid                bool
			}{
				{"global", global, nil, true},
				{"pair", pair, nil, true},
				{"loopback", netip.IPv6Loopback().AsSlice(), nil, false},
				{"native-ipv4", netip.MustParseAddr("192.0.2.9").AsSlice(), nil, fc.native4},
				{"mapped", netip.MustParseAddr("::ffff:192.0.2.9").AsSlice(), nil, fc.mapped},
				{"vpn24", append(make([]byte, 8), global...), nil, false},
				{"vpn48", append(append(append(make([]byte, 8), global...), make([]byte, 8)...), linkLocal...), nil, false},
				{"later-repair", pair, append(make([]byte, 8), global...), true},
			} {
				t.Run(fc.name+"/"+rail+"/"+tc.name, func(t *testing.T) {
					f := rfc2545ReceiveFixture(t, true, false, true)
					for _, peer := range []*Peer{f.source, f.destination} {
						caps := []capability.Capability{
							&capability.ASN4{ASN: peer.settings.LocalAS},
							&capability.Multiprotocol{AFI: fc.fam.AFI, SAFI: fc.fam.SAFI},
							&capability.Multiprotocol{AFI: 1, SAFI: 1},
							&capability.ExtendedNextHop{Families: []capability.ExtendedNextHopFamily{{NLRIAFI: 1, NLRISAFI: capability.SAFI(fc.fam.SAFI), NextHopAFI: 2}}},
						}
						neg := capability.Negotiate(caps, caps, capability.PeerIdentity{LocalASN: peer.settings.LocalAS, PeerASN: peer.settings.PeerAS})
						peer.session.negotiated = neg
						peer.negotiated.Store(NewNegotiatedCapabilities(neg))
						peer.sendCtx.Store(bgpctx.NewEncodingContext(neg.Identity, neg.Encoding, bgpctx.DirectionSend))
					}
					f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
					rewrite := func(_, _ filterapi.PeerFilterInfo, payload []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
						if payloadNextHop(payload).mpFamily == fc.fam {
							if tc.earlier != nil {
								mods.Op(14, filterapi.AttrModSet, tc.earlier)
							}
							mods.Op(14, filterapi.AttrModSet, tc.replacement)
						}
						return true
					}
					f.r.orderedEgressSteps = orderedEgressStepsFromFuncs(rewrite)
					f.r.egressFilters = []filterapi.EgressFilterFunc{rewrite}
					raw := mustHex(t, fc.raw)
					mp := append([]byte{0, byte(fc.fam.AFI), byte(fc.fam.SAFI), 16}, global...)
					mp = append(mp, 0)
					mp = append(mp, raw...)
					attrs := mixedAttrs(append([]byte{0x80, 14, byte(len(mp))}, mp...))
					if fc.fam.SAFI == 73 {
						tunnel := teSRPolicyValue(0, false)
						attrs = append(attrs, 0xd0, byte(attribute.AttrTunnelEncap), byte(len(tunnel)>>8), byte(len(tunnel)))
						attrs = append(attrs, tunnel...)
						attrs = append(attrs, 0xc0, byte(attribute.AttrExtCommunity), 8, 1, 2, 192, 0, 2, 2, 0, 0)
					}
					attrs = append(attrs, 0x40, 3, 4)
					attrs = append(attrs, f.source.Settings().Address.AsSlice()...)
					legacy := mustHex(t, "180a0900")
					updates := rfc2545ReceiveForward(t, f, rail, buildUpdatePayload(attrs, legacy))
					nativeCount, legacyCount := 0, 0
					for _, update := range updates {
						if len(update.NLRI) != 0 {
							legacyCount++
							if !bytes.Equal(update.NLRI, legacy) {
								t.Errorf("sibling NLRI=%x want=%x", update.NLRI, legacy)
							}
							continue
						}
						nativeCount++
						_, _, reach, hasReach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
						_, _, unreach, hasUnreach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
						if tc.valid {
							wantHop := tc.replacement
							if len(wantHop) == 32 {
								wantHop = wantHop[:16]
							}
							want := append([]byte{0, byte(fc.fam.AFI), byte(fc.fam.SAFI), byte(len(wantHop))}, wantHop...)
							want = append(want, 0)
							want = append(want, raw...)
							if !hasReach || hasUnreach || !bytes.Equal(reach, want) {
								t.Errorf("reach=%x withdrawal=%x want reach=%x", reach, unreach, want)
							}
						} else {
							want := append([]byte{0, byte(fc.fam.AFI), byte(fc.fam.SAFI)}, raw...)
							if hasReach || !hasUnreach || !bytes.Equal(unreach, want) {
								t.Errorf("reach=%x withdrawal=%x want withdrawal=%x", reach, unreach, want)
							}
						}
					}
					if nativeCount != 1 || legacyCount != 1 {
						t.Errorf("native/legacy decisions=%d/%d want 1/1", nativeCount, legacyCount)
					}
				})
			}
		}
	}
}

// TestFamilyIndependentNextHopFields preserves contracts that are not plain
// IPv6-global roles: MVPN IPv4 under AFI 2, ignored FlowSpec, and mapped MUP.
// These are real builder bytes, not values returned by the admission predicate.
func TestFamilyIndependentNextHopFields(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params message.PluginParams
		want   string
	}{
		{"mvpn-ipv4", message.PluginParams{AFI: 2, SAFI: 5, IsIPv6: true, NLRI: []byte{1}, NextHop: netip.MustParseAddr("192.0.2.9")}, "c0000209"},
		{"flowspec-zero", message.PluginParams{AFI: 2, SAFI: 133, IsIPv6: true, NLRI: []byte{1}, NextHop: netip.IPv6Loopback()}, ""},
		{"mup-mapped", message.PluginParams{AFI: 2, SAFI: 85, IsIPv6: true, NLRI: []byte{1}, NextHop: netip.MustParseAddr("192.0.2.9"), MapV4NextHop: true}, "00000000000000000000ffffc0000209"},
		{"unknown-profile", message.PluginParams{AFI: 16388, SAFI: 71, NLRI: []byte{1}, NextHop: netip.IPv6Loopback()}, "00000000000000000000000000000001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builder := message.NewUpdateBuilder(65000, false, true, false)
			update := builder.BuildPlugin(tc.params)
			if update == nil {
				t.Fatal("independent family field refused")
			}
			_, _, mp, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
			want := mustHex(t, tc.want)
			if !found || len(mp) != 6+len(want) || int(mp[3]) != len(want) || !bytes.Equal(mp[4:4+len(want)], want) {
				t.Errorf("MP_REACH=%x want next-hop=%x", mp, want)
			}
		})
	}
}

// TestMappedVPNNextHopForwardAdmission preserves RFC 4659 Section 3.2.1.2's
// zero-RD mapped field through actual receive and both forwarding consumers.
// It asserts only control-plane field retention, not IPv4 tunnel installation.
func TestMappedVPNNextHopForwardAdmission(t *testing.T) {
	fam := family.Family{AFI: 2, SAFI: 128}
	for _, rail := range []string{"cached", "rs"} {
		t.Run(rail, func(t *testing.T) {
			f := rfc2545ReceiveFixture(t, false, true, true)
			for _, peer := range []*Peer{f.source, f.destination} {
				caps := []capability.Capability{
					&capability.ASN4{ASN: peer.settings.LocalAS},
					&capability.Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI},
				}
				neg := capability.Negotiate(caps, caps, capability.PeerIdentity{LocalASN: peer.settings.LocalAS, PeerASN: peer.settings.PeerAS})
				peer.session.negotiated = neg
				peer.negotiated.Store(NewNegotiatedCapabilities(neg))
			}
			f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
			want := mustHex(t, "00028018000000000000000000000000000000000000ffffc00002090088003e810000fde80000006420010db80001")
			attrs := mixedAttrs(append([]byte{0x80, 14, byte(len(want))}, want...))
			updates := rfc2545ReceiveForward(t, f, rail, buildUpdatePayload(attrs, nil))
			if len(updates) != 1 {
				t.Fatalf("UPDATE count=%d want=1", len(updates))
			}
			_, _, reach, hasReach := attribute.AttrFind(updates[0].PathAttributes, attribute.AttrMPReachNLRI)
			_, _, _, hasUnreach := attribute.AttrFind(updates[0].PathAttributes, attribute.AttrMPUnreachNLRI)
			if !hasReach || hasUnreach || !bytes.Equal(reach, want) {
				t.Errorf("mapped VPN reach=%x want=%x", reach, want)
			}
		})
	}
}
