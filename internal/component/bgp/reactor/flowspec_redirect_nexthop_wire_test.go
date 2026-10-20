// Design: docs/architecture/update-building.md -- local FlowSpec origination rails.
// Related: flowspec_origin_wire_test.go -- the zero next-hop length every other FlowSpec action keeps.
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestFlowSpecRedirectToNextHopCarriesTheNextHop proves that a FlowSpec route
// whose action is the redirect/mirror-to-IP-next-hop community (type 0x0800)
// goes out with its next hop in MP_REACH_NLRI, on every originating rail.
//
// Method: the same socket harness as TestFlowSpecOriginationOmitsConfiguredNextHop
// (config initial sync through BuildPlugin, the API batch, and both queue
// drains through buildRIBRouteUpdate), reading the complete UPDATE as literal
// bytes. The absent-hop rows keep the zero length, so a producer that copies
// a zero address in, or refuses the route, fails too.
//
// draft-simpson-idr-flowspec-redirect-02 Section 3: packets go "towards the
// IPv4 or IPv6 address specified in the 'Network Address of Next-Hop' field of
// the associated MP_REACH_NLRI", and with "the length of the next-hop is 0,
// then the redirect-to-IP extended community, if present, should be ignored."
// ExaBGP qa/old conf-flow (4.2 and 5.0) expects `redirect 1.2.3.4` as
// `0x0800` plus next hop `04 01020304`.
func TestFlowSpecRedirectToNextHopCarriesTheNextHop(t *testing.T) {
	family.RegisterTestFamilies()
	ipv4Flow := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	ipv4FlowVPN := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpecVPN}
	ipv6Flow := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec}
	const (
		redirect = "0800000000000000"
		mirror   = "0800000000000001"
		ipv4NLRI = "050118c00002"
		vpnNLRI  = "0d0000ffff000100000118c00002"
		ipv6NLRI = "0701200020010db8"
		marker   = "ffffffffffffffffffffffffffffffff"
		wellKnow = "4001010040020040050400000064"
	)
	for _, tc := range []struct {
		name   string
		family family.Family
		nlri   string
		hop    netip.Addr
		action string
		packet string
	}{
		{
			name: "ipv4-flow/redirect", family: ipv4Flow, nlri: ipv4NLRI,
			hop: netip.MustParseAddr("1.2.3.4"), action: redirect,
			packet: marker + "0042020000002b" + "800e0f000185040102030400" + ipv4NLRI + wellKnow + "c01008" + redirect,
		},
		{
			name: "ipv4-flow/mirror", family: ipv4Flow, nlri: ipv4NLRI,
			hop: netip.MustParseAddr("1.2.3.4"), action: mirror,
			packet: marker + "0042020000002b" + "800e0f000185040102030400" + ipv4NLRI + wellKnow + "c01008" + mirror,
		},
		{
			name: "ipv4-flow-vpn/redirect", family: ipv4FlowVPN, nlri: vpnNLRI,
			hop: netip.MustParseAddr("5.6.7.8"), action: redirect,
			packet: marker + "004a0200000033" + "800e17000186040506070800" + vpnNLRI + wellKnow + "c01008" + redirect,
		},
		{
			name: "ipv6-flow/redirect", family: ipv6Flow, nlri: ipv6NLRI,
			hop: netip.MustParseAddr("2001:db8::9"), action: redirect,
			packet: marker + "00500200000039" + "800e1d00028510" + "20010db8000000000000000000000009" + "00" + ipv6NLRI + wellKnow + "c01008" + redirect,
		},
		{
			name: "ipv4-flow/redirect-absent-hop", family: ipv4Flow, nlri: ipv4NLRI,
			action: redirect,
			packet: marker + "003e0200000027" + "800e0b0001850000" + ipv4NLRI + wellKnow + "c01008" + redirect,
		},
	} {
		for _, rail := range []string{"config", "batch", "queued-initial", "queued-final"} {
			t.Run(tc.name+"/"+rail, func(t *testing.T) {
				peer, _ := newInitialSyncPeer(t, false, tc.family)
				peer.settings.PeerAS = peer.settings.LocalAS
				peer.settings.ManualEOR = true
				session, client := newConnectedLocalSession(t, peer.settings, "127.0.0.1")
				if err := session.fsm.Event(fsm.EventBGPOpen); err != nil {
					t.Fatal(err)
				}
				if err := session.fsm.Event(fsm.EventKeepaliveMsg); err != nil {
					t.Fatal(err)
				}
				peer.session = session
				rawNLRI := mustHex(t, tc.nlri)
				action := mustHex(t, tc.action)
				if rail == "config" {
					peer.settings.PluginRoutes = []PluginRoute{{
						Family: tc.family.String(), IsIPv6: tc.family.AFI == family.AFIIPv6,
						NLRI: rawNLRI, NextHop: tc.hop,
						RawAttrs: [][]byte{append(mustHex(t, "c01008"), action...)},
					}}
					peer.sendInitialRoutes()
				} else {
					api := &reactorAPIAdapter{r: &Reactor{
						config:          &Config{LocalAS: peer.settings.LocalAS},
						attrModHandlers: attrModHandlersWithDefaults(),
						peers:           map[netip.AddrPort]*Peer{peer.settings.PeerKey(): peer},
					}}
					n, err := nlri.NewWireNLRI(tc.family, rawNLRI, false)
					if err != nil {
						t.Fatal(err)
					}
					attrs := attribute.NewBuilder()
					attrs.AddExtendedCommunity(attribute.ExtendedCommunity(action))
					batch := bgptypes.NLRIBatch{
						Family: tc.family, NLRIs: []nlri.NLRI{n},
						NextHop: bgptypes.NewNextHopExplicit(tc.hop),
						Wire:    attribute.NewAttributesWire(attrs.Build(), bgpctx.APIContextID),
					}
					if rail == "batch" {
						peer.sendingInitialRoutes.Store(0)
						peer.initialSyncEOROwed.Store(false)
					}
					if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
						t.Fatal(err)
					}
					if rail != "batch" {
						if len(peer.opQueue) != 1 {
							t.Fatalf("queued %d operations, want one announcement", len(peer.opQueue))
						}
						if rail == "queued-initial" {
							peer.sendInitialRoutes()
						} else {
							peer.drainAndCloseQueueGate(peer.addrString, message.MaxMsgLen)
						}
						if len(peer.opQueue) != 0 {
							t.Fatal("announcement remained queued instead of reaching the socket")
						}
					}
				}
				packet, err := core4271ReadMessage(client)
				if err != nil {
					t.Fatal(err)
				}
				if want := mustHex(t, tc.packet); !bytes.Equal(packet, want) {
					t.Fatalf("FlowSpec redirect UPDATE = %x, want %x (next hop carried for the 0x0800 action)", packet, want)
				}
			})
		}
	}
}
