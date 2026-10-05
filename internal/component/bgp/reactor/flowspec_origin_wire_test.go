// Design: docs/architecture/update-building.md -- local FlowSpec origination rails.
// RFC 8955 Section 4 -- see rfc/short/rfc8955.md.
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

// TestFlowSpecOriginationOmitsConfiguredNextHop reads complete UPDATEs from a
// real TCP socket after config-route initial sync or API batch admission. The
// API cases exercise both queue drains as well as the already-established rail.
// The literal frames pin the NLRI, RD, rate-limit action and empty next-hop;
// absent-hop controls prevent a producer that drops every rule from passing.
// RFC 8955 Section 4: "When advertising Flow Specifications, the Length of the
// Next-Hop Network Address MUST be set to 0."
// RFC 8956 Section 1: "It only defines the delta changes required to support
// IPv6, while all other definitions and operation mechanisms of
// \"Dissemination of Flow Specification Rules\" will remain in the main
// specification and will not be repeated here."
// RFC requirement: RFC8955-4-3 positive -- config initial sync and API batch/queued origination deliver exact IPv4/IPv6 flow and flow-vpn UPDATEs with an empty MP_REACH next-hop and intact NLRI/actions over TCP.
// RFC requirement: RFC8955-4-3 negative -- explicit IPv4 and IPv6 next hops cannot enter MP_REACH on any originating rail, without dropping the advertised rule or its action.
// RFC requirement: RFC5575-4-8 positive -- daemon origination delivers exact FlowSpec UPDATEs with zero next-hop length over TCP; receipt is covered separately by the RIB carrier.
// RFC requirement: RFC5575-4-8 negative -- configured forwarding addresses do not displace the FlowSpec NLRI or action on config, batch or either queued writer.
// MUTATION: remove the NeedsNextHop guard in buildMPReachPlugin,
// buildBatchAnnounceUpdate or buildRIBRouteUpdate so the configured address is
// copied into MP_REACH. The corresponding socket case must fail.
func TestFlowSpecOriginationOmitsConfiguredNextHop(t *testing.T) {
	family.RegisterTestFamilies()
	for _, tc := range []struct {
		name   string
		family family.Family
		nlri   string
		packet string
	}{
		{
			name: "ipv4-flow", family: family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec},
			nlri:   "050118c00002",
			packet: "ffffffffffffffffffffffffffffffff003e02000000274001010040020040050400000064800e0b0001850000050118c00002c010088006000046160000",
		},
		{
			name: "ipv4-flow-vpn", family: family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpecVPN},
			nlri:   "0d0000ffff000100000118c00002",
			packet: "ffffffffffffffffffffffffffffffff0046020000002f4001010040020040050400000064800e1300018600000d0000ffff000100000118c00002c010088006000046160000",
		},
		{
			name: "ipv6-flow", family: family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec},
			nlri:   "0701200020010db8",
			packet: "ffffffffffffffffffffffffffffffff004002000000294001010040020040050400000064800e0d00028500000701200020010db8c010088006000046160000",
		},
		{
			name: "ipv6-flow-vpn", family: family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpecVPN},
			nlri:   "0f0000ffff0001000001200020010db8",
			packet: "ffffffffffffffffffffffffffffffff004802000000314001010040020040050400000064800e1500028600000f0000ffff0001000001200020010db8c010088006000046160000",
		},
	} {
		for _, hop := range []struct {
			name string
			addr netip.Addr
		}{
			{name: "absent"},
			{name: "ipv4", addr: netip.MustParseAddr("192.0.2.9")},
			{name: "ipv6", addr: netip.MustParseAddr("2001:db8::9")},
		} {
			for _, rail := range []string{"config", "batch", "queued-initial", "queued-final"} {
				t.Run(tc.name+"/"+hop.name+"/"+rail, func(t *testing.T) {
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
					if rail == "config" {
						peer.settings.PluginRoutes = []PluginRoute{{
							Family: tc.family.String(), IsIPv6: tc.family.AFI == family.AFIIPv6,
							NLRI: rawNLRI, NextHop: hop.addr,
							RawAttrs: [][]byte{mustHex(t, "c010088006000046160000")},
						}}
						// RFC 8955 Section 4: production initial sync calls BuildPlugin.
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
						attrs.AddExtendedCommunity(attribute.ExtendedCommunity{0x80, 6, 0, 0, 0x46, 0x16, 0, 0})
						batch := bgptypes.NLRIBatch{
							Family: tc.family, NLRIs: []nlri.NLRI{n},
							NextHop: bgptypes.NewNextHopExplicit(hop.addr),
							Wire:    attribute.NewAttributesWire(attrs.Build(), bgpctx.APIContextID),
						}
						if rail == "batch" {
							peer.sendingInitialRoutes.Store(0)
							peer.initialSyncEOROwed.Store(false)
						}
						// RFC 8955 Section 4: admit via the real API, never a builder.
						if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
							t.Fatal(err)
						}
						if rail != "batch" {
							if len(peer.opQueue) != 1 {
								t.Fatalf("queued %d operations, want one announcement", len(peer.opQueue))
							}
							// RFC 8955 Section 4: both production drains encode MP_REACH.
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
						t.Fatalf("FlowSpec UPDATE = %x, want %x (zero next-hop, exact NLRI and action)", packet, want)
					}
				})
			}
		}
	}
}
