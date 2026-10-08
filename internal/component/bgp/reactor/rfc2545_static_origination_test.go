package reactor

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// TestStaticOriginateIPv6NextHopAdmission checks actual static-route delivery and
// automatic default origination against exact UPDATE and End-of-RIB bytes.
//
// RFC requirement: RFC2545-3-1 negative -- static IPv6 routes with unusable
// global next hops are not advertised through the shared unicast builder.
// RFC requirement: RFC2545-3-1 positive -- static and automatically addressed
// default routes retain the exact supplied or connected global IPv6 next hop.
// MUTATION: bypass BuildUnicast's IPv6 admission; unusable static next hops must
// still leave only End-of-RIB on the socket.
func TestStaticOriginateIPv6NextHopAdmission(t *testing.T) {
	// The oracle constructs only framing around literal expected attributes;
	// it never asks a production route builder what the expected bytes are.
	wire := func(t *testing.T, fam family.Family, attrs, nlri string) []byte {
		t.Helper()
		want := eorWire(fam)
		if attrs == "" {
			return want
		}
		attributes, err := hex.DecodeString("4001010040020602010000fde8" + attrs)
		if err != nil {
			t.Fatal(err)
		}
		prefix, err := hex.DecodeString(nlri)
		if err != nil {
			t.Fatal(err)
		}
		update := bytes.Repeat([]byte{0xff}, 16)
		update = append(update, 0, 0, 2, 0, 0, 0, 0)
		binary.BigEndian.PutUint16(update[16:18], uint16(23+len(attributes)+len(prefix)))
		binary.BigEndian.PutUint16(update[21:23], uint16(len(attributes)))
		update = append(update, attributes...)
		update = append(update, prefix...)
		return append(update, want...)
	}

	const global = "20010db8000100000000000000000001"
	const linkLocal = "fe800000000000000000000000000001"
	cases := []struct {
		name     string
		prefix   string
		nextHop  netip.Addr
		extended bool
		cap77    bool
		attrs    string
		nlri     string
	}{
		{name: "loopback", prefix: "2001:db8:7::/64", nextHop: netip.MustParseAddr("::1")},
		{name: "unset", prefix: "2001:db8:7::/64"},
		{name: "unspecified", prefix: "2001:db8:7::/64", nextHop: netip.MustParseAddr("::")},
		{name: "multicast", prefix: "2001:db8:7::/64", nextHop: netip.MustParseAddr("ff0e::1")},
		{name: "mapped", prefix: "2001:db8:7::/64", nextHop: netip.MustParseAddr("::ffff:192.0.2.1"), attrs: "800e1e0002011000000000000000000000ffffc0000201004020010db800070000"},
		{name: "link-local-unnegotiated", prefix: "2001:db8:7::/64", nextHop: netip.MustParseAddr("fe80::1")},
		{name: "global", prefix: "2001:db8:7::/64", nextHop: netip.MustParseAddr("2001:db8:1::1"), attrs: "800e1e00020110" + global + "004020010db800070000"},
		{name: "cap77", prefix: "2001:db8:7::/64", nextHop: netip.MustParseAddr("fe80::1"), cap77: true, attrs: "800e1e00020110" + linkLocal + "004020010db800070000"},
		{name: "extended-global", prefix: "192.0.2.0/24", nextHop: netip.MustParseAddr("2001:db8:1::1"), extended: true, attrs: "800e1900010110" + global + "0018c00002"},
		{name: "extended-loopback", prefix: "192.0.2.0/24", nextHop: netip.MustParseAddr("::1"), extended: true},
		{name: "extended-unnegotiated", prefix: "192.0.2.0/24", nextHop: netip.MustParseAddr("2001:db8:1::1")},
		{name: "extended-cap77", prefix: "192.0.2.0/24", nextHop: netip.MustParseAddr("fe80::1"), extended: true, cap77: true, attrs: "800e1900010110" + linkLocal + "0018c00002"},
		{name: "extended-without-cap77", prefix: "192.0.2.0/24", nextHop: netip.MustParseAddr("fe80::1"), extended: true},
		{name: "native-ipv4", prefix: "192.0.2.0/24", nextHop: netip.MustParseAddr("192.0.2.1"), attrs: "400304c0000201", nlri: "18c00002"},
		// RFC 8950 Section 4 licenses IPv4 NLRI with IPv6 next hops only.
		{name: "ipv6-with-ipv4-extended", prefix: "2001:db8:7::/64", nextHop: netip.MustParseAddr("192.0.2.1"), extended: true},
		{name: "ipv6-with-ipv4", prefix: "2001:db8:7::/64", nextHop: netip.MustParseAddr("192.0.2.1")},
	}
	for _, grouped := range []bool{false, true} {
		mode := "ungrouped"
		if grouped {
			mode = "grouped"
		}
		t.Run(mode, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					prefix := netip.MustParsePrefix(tc.prefix)
					fam := family.IPv6Unicast
					if prefix.Addr().Is4() {
						fam = family.IPv4Unicast
					}
					peer, conn := newInitialSyncPeer(t, true, fam)
					peer.settings.Address = netip.MustParseAddr("2001:db8:2::2")
					peer.settings.GroupUpdates = grouped
					peer.settings.StaticRoutes = []StaticRoute{{Prefix: prefix, NextHop: bgptypes.NewNextHopExplicit(tc.nextHop)}}
					caps := []capability.Capability{
						&capability.ASN4{ASN: 65000},
						&capability.Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI},
					}
					if tc.cap77 {
						caps = append(caps, &capability.LinkLocalNextHop{})
					}
					if tc.extended {
						nhAFI := capability.AFIIPv6
						if tc.nextHop.Is4() {
							nhAFI = capability.AFIIPv4
						}
						caps = append(caps, &capability.ExtendedNextHop{
							Families: []capability.ExtendedNextHopFamily{{
								NLRIAFI: capability.AFI(fam.AFI), NLRISAFI: capability.SAFI(fam.SAFI), NextHopAFI: nhAFI,
							}},
						})
					}
					neg := capability.Negotiate(caps, caps, capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001})
					peer.session.negotiated = neg
					peer.negotiated.Store(NewNegotiatedCapabilities(neg))
					peer.sendCtx.Store(bgpctx.NewEncodingContext(neg.Identity, neg.Encoding, bgpctx.DirectionSend))
					peer.refreshLinkScopeFrom(nil)

					// RFC 2545 Section 3; RFC 8950 Section 3; capability 77
					// retains its negotiated standalone link-local form.
					peer.sendInitialRoutes()

					want := wire(t, fam, tc.attrs, tc.nlri)
					if got := conn.written(); !bytes.Equal(got, want) {
						t.Fatalf("static next hop %v: wire = %x, want %x", tc.nextHop, got, want)
					}
				})
			}
		})
	}

	t.Run("default-auto-global", func(t *testing.T) {
		peer, conn := newDefaultOriginatePeer(t, "2001:db8:2::2", "2001:db8:1::1", "fe80::1")
		peer.settings.LocalAddress = netip.Addr{}
		peer.currentSession().transport.Store(&sessionTransport{local: netip.MustParseAddr("2001:db8:1::1")})
		peer.refreshLinkScopeFrom(nil)

		// RFC 2545 Section 3: automatic local addressing must use the actual
		// session endpoint, never invent ::1 from absent configuration.
		peer.sendInitialRoutes()

		want := wire(t, family.IPv6Unicast, "800e1600020110"+global+"0000", "")
		if got := conn.written(); !bytes.Equal(got, want) {
			t.Fatalf("automatic default next hop: wire = %x, want %x", got, want)
		}
	})
}

// TestStaticOriginateContinuesAfterUnusablePolicyNextHop drives the configured
// static route loop, including the grouped IPv4 callback, with policy refusing
// the first group and a usable IPv6 group ordered strictly after it.
//
// RFC requirement: RFC2545-3-1 negative -- a policy-written loopback next hop is
// withheld without discarding an independent usable static route on the session.
// RFC requirement: RFC2545-3-1 positive -- unchanged global next hops and later
// usable groups retain their exact wire bytes and successful-send accounting.
// MUTATION: treat the final next-hop refusal as a connection failure in any
// static send loop; the later group and its recorded static ownership must remain.
func TestStaticOriginateContinuesAfterUnusablePolicyNextHop(t *testing.T) {
	for _, mode := range []string{"ungrouped", "grouped", "grouped-ipv4-batch"} {
		for _, phase := range []string{"global-control", "refused-first", "withdraw-control", "refused-first-withdrawal"} {
			refuse := phase == "refused-first"
			t.Run(mode+"/"+phase, func(t *testing.T) {
				initial, conn := newInitialSyncPeer(t, true, family.IPv4Unicast, family.IPv6Unicast)
				initial.settings.Address = netip.MustParseAddr("2001:db8:2::2")
				initial.settings.GroupUpdates = mode != "ungrouped"
				firstHop := netip.MustParseAddr("2001:db8:1::1")
				routes := []StaticRoute{{
					Prefix: netip.MustParsePrefix("2001:db8:7::/64"), NextHop: bgptypes.NewNextHopExplicit(firstHop),
				}}
				firstWire := mustHex(t, "ffffffffffffffffffffffffffffffff0045020000002e4001010040020602010000fde8800e1e0002011020010db8000100000000000000000001004020010db800070000")
				if mode == "grouped-ipv4-batch" {
					firstHop = netip.MustParseAddr("192.0.2.1")
					routes = []StaticRoute{
						{Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: bgptypes.NewNextHopExplicit(firstHop)},
						{Prefix: netip.MustParsePrefix("10.0.1.0/24"), NextHop: bgptypes.NewNextHopExplicit(firstHop)},
					}
					firstWire = mustHex(t, "ffffffffffffffffffffffffffffffff003302000000144001010040020602010000fde8400304c0000201180a0000180a0001")
				}
				good := StaticRoute{
					Prefix:  netip.MustParsePrefix("2001:db8:8::/64"),
					NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8:9::1")),
				}
				routes = append(routes, good)
				initial.settings.StaticRoutes = routes
				initial.settings.ExportFilters = []filterapi.FilterRef{{Name: "static:replace-first-next-hop"}}
				r := New(&Config{ListenAddr: "127.0.0.1:0", LocalAS: 65000})
				if err := r.AddPeer(initial.settings); err != nil {
					t.Fatal(err)
				}
				r.mu.RLock()
				peer, found := r.findPeerByAddr(initial.settings.Address)
				r.mu.RUnlock()
				if !found {
					t.Fatal("configured static recipient was not registered")
				}
				peer.state.Store(int32(PeerStateEstablished))
				peer.negotiated.Store(initial.negotiated.Load())
				peer.session = initial.session
				peer.session.SetMessageCallback(peer.messageCallback)
				peer.sendingInitialRoutes.Store(1)
				peer.initialSyncEOROwed.Store(true)
				peer.refreshLinkScopeFrom(nil)
				peer.refreshForwardFacts()
				r.api = &pluginserver.Server{}
				invalid := mustHex(t, "0000002e4001010040020602010000fde8800e1e0002011000000000000000000000000000000001004020010db800070000")
				var policyHop netip.Addr
				var policyHops []netip.Addr
				withdrawing, withdrawalCalls := false, 0
				r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
					if withdrawing {
						withdrawalCalls++
						if phase == "refused-first-withdrawal" && withdrawalCalls == 1 {
							return PolicyResponse{Action: PolicyModify, Raw: invalid}
						}
						return PolicyResponse{Action: PolicyAccept}
					}
					policyHops = append(policyHops, policyHop)
					if refuse && policyHop == firstHop {
						return PolicyResponse{Action: PolicyModify, Raw: invalid}
					}
					return PolicyResponse{Action: PolicyAccept}
				}
				peer.session.egressRouteFilter = func(body []byte) (bool, []byte) {
					hop := payloadNextHop(body)
					policyHop = hop.mp
					if !policyHop.IsValid() {
						policyHop = hop.legacy
					}
					return r.exportFilterForBody(peer, body)
				}
				peer.sendInitialRoutes()

				var want []byte
				wantRoutes, wantUpdates := len(routes), uint32(4)
				if refuse {
					wantRoutes, wantUpdates = 1, 3
				} else {
					want = append(want, firstWire...)
				}
				want = append(want, mustHex(t, "ffffffffffffffffffffffffffffffff0045020000002e4001010040020602010000fde8800e1e0002011020010db8000900000000000000000001004020010db800080000")...)
				want = append(want, eorWire(family.IPv4Unicast)...)
				want = append(want, eorWire(family.IPv6Unicast)...)
				if got := conn.written(); !bytes.Equal(got, want) {
					t.Errorf("static wire = %x, want %x", got, want)
				}
				if len(policyHops) != 2 || policyHops[0] != firstHop || policyHops[1] != good.NextHop.Addr {
					t.Errorf("policy group order = %v, want [%s %s]", policyHops, firstHop, good.NextHop.Addr)
				}
				if stats := peer.Stats(); stats.UpdatesSent != wantUpdates || stats.EORSent != 2 {
					t.Errorf("sent updates/EOR = %d/%d, want %d/2", stats.UpdatesSent, stats.EORSent, wantUpdates)
				}
				if len(peer.staticWire.routes) != wantRoutes {
					t.Errorf("recorded static routes = %d, want %d", len(peer.staticWire.routes), wantRoutes)
				} else if peer.staticWire.routes[wantRoutes-1].Prefix != good.Prefix {
					t.Error("later usable static route was not recorded on this session")
				}
				if phase != "withdraw-control" && phase != "refused-first-withdrawal" {
					return
				}
				withdrawing = true
				peer.deliverStaticRouteDelta(nil)
				refuseWithdrawal := phase == "refused-first-withdrawal"
				if mode == "grouped-ipv4-batch" {
					if !refuseWithdrawal {
						want = append(want, mustHex(t, "ffffffffffffffffffffffffffffffff001b020004180a00000000")...)
					}
					want = append(want, mustHex(t, "ffffffffffffffffffffffffffffffff001b020004180a00010000")...)
				} else if !refuseWithdrawal {
					want = append(want, mustHex(t, "ffffffffffffffffffffffffffffffff0026020000000f800f0c0002014020010db800070000")...)
				}
				want = append(want, mustHex(t, "ffffffffffffffffffffffffffffffff0026020000000f800f0c0002014020010db800080000")...)
				wantUpdates += uint32(len(routes))
				if refuseWithdrawal {
					wantUpdates--
				}
				if got := conn.written(); !bytes.Equal(got, want) {
					t.Errorf("static withdrawal wire = %x, want %x", got, want)
				}
				if withdrawalCalls != len(routes) {
					t.Errorf("withdrawal policy calls = %d, want %d", withdrawalCalls, len(routes))
				}
				if stats := peer.Stats(); stats.UpdatesSent != wantUpdates || stats.EORSent != 2 {
					t.Errorf("after withdrawal updates/EOR = %d/%d, want %d/2", stats.UpdatesSent, stats.EORSent, wantUpdates)
				}
				if peer.session.tearingDown.Load() || peer.session.writeFailed != nil {
					t.Fatal("route-scoped withdrawal refusal retired the healthy session")
				}
			})
		}
	}
}
