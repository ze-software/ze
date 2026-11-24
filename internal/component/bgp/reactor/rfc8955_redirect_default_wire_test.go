// Design: docs/architecture/update-building.md -- RFC 8955 Section 4 default-path enforcement for 0x0800 FlowSpec routes.
// RFC: rfc/short/rfc8955.md -- Section 4, zero-length Next-Hop Network Address on default configuration path.
// Related: flowspec_redirect_nexthop_wire_test.go -- opted-in draft-simpson next-hop tests.
// Related: flowspec_redirect_forward_wire_test.go -- opted-in draft-simpson forwarding tests.
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// flowSpecRedirectPeerTree builds a minimal resolved BGP tree for one peer with
// an optional `session flowspec-redirect-next-hop` leaf.
func flowSpecRedirectPeerTree(optIn *bool) map[string]any {
	session := map[string]any{
		"asn": map[string]any{
			"local":  "65000",
			"remote": "65000",
		},
		"router-id": "192.0.2.1",
	}
	if optIn != nil {
		if *optIn {
			session["flowspec-redirect-next-hop"] = "true"
		} else {
			session["flowspec-redirect-next-hop"] = "false"
		}
	}
	return map[string]any{
		"router-id": "192.0.2.1",
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"peer": map[string]any{
			"p1": map[string]any{
				"connection": map[string]any{
					"remote": map[string]any{"ip": "192.0.2.2"},
					"local":  map[string]any{"ip": "auto"},
				},
				"session": session,
			},
		},
	}
}

type flowSpecRedirectWireCase struct {
	name      string
	fam       family.Family
	nlri      string
	hop       netip.Addr
	optIn     bool
	action    string
	wantFrame string
}

func runFlowSpecRedirectOriginCases(t *testing.T, cases []flowSpecRedirectWireCase) {
	t.Helper()
	for _, tc := range cases {
		for _, rail := range []string{"config", "batch", "queued-initial", "queued-final"} {
			t.Run("origin/"+tc.name+"/"+rail, func(t *testing.T) {
				peer, _ := newInitialSyncPeer(t, false, tc.fam)
				peer.settings.PeerAS = peer.settings.LocalAS
				peer.settings.ManualEOR = true
				peer.settings.FlowSpecRedirectNextHop = tc.optIn
				session, client := newConnectedLocalSession(t, peer.settings, "127.0.0.1")
				require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
				require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))
				peer.session = session
				peer.setTestInitialSync(1, true)

				rawNLRI := mustHex(t, tc.nlri)
				action := mustHex(t, tc.action)
				if rail == "config" {
					peer.settings.PluginRoutes = []PluginRoute{{
						Family: tc.fam.String(), IsIPv6: tc.fam.AFI == family.AFIIPv6,
						NLRI: rawNLRI, NextHop: tc.hop,
						RawAttrs: [][]byte{append(mustHex(t, "c01008"), action...)},
					}}
					peer.runInitialSyncNow()
				} else {
					api := &reactorAPIAdapter{r: &Reactor{
						config:          &Config{LocalAS: peer.settings.LocalAS},
						attrModHandlers: attrModHandlersWithDefaults(),
						peers:           map[netip.AddrPort]*Peer{peer.settings.PeerKey(): peer},
					}}
					n, err := nlri.NewWireNLRI(tc.fam, rawNLRI, false)
					require.NoError(t, err)
					var ec attribute.ExtendedCommunity
					copy(ec[:], action)
					attrs := attribute.NewBuilder()
					attrs.AddExtendedCommunity(ec)
					batch := bgptypes.NLRIBatch{
						Family: tc.fam, NLRIs: []nlri.NLRI{n},
						NextHop: bgptypes.NewNextHopExplicit(tc.hop),
						Wire:    attribute.NewAttributesWire(attrs.Build(), bgpctx.APIContextID),
					}
					if rail == "batch" {
						peer.testInitialSync().sendingInitialRoutes.Store(0)
						peer.testInitialSync().initialSyncEOROwed.Store(false)
					}
					require.NoError(t, api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()))
					if rail != "batch" {
						require.Len(t, peer.opQueue, 1)
						if rail == "queued-initial" {
							peer.runInitialSyncNow()
						} else {
							peer.drainAndCloseQueueGate(peer.testInitialSync(), peer.addrString, message.MaxMsgLen)
						}
						require.Empty(t, peer.opQueue)
					}
				}
				got, err := core4271ReadMessage(client)
				require.NoError(t, err)
				want := mustHex(t, tc.wantFrame)
				if !bytes.Equal(got, want) {
					t.Fatalf("got frame  %x\nwant frame %x", got, want)
				}
			})
		}
	}
}

func runFlowSpecRedirectForwardCase(t *testing.T, fam family.Family, nlriHex string, recvHop netip.Addr, optIn bool, wantHop []byte) {
	t.Helper()
	f := newAIGPReplayFixture(t, nil)
	for _, peer := range []*Peer{f.source, f.destination} {
		negotiateForwardTestFamilies(peer, fam)
	}
	f.destination.settings.PeerAS = 65000
	f.destination.settings.ExportPolicy = setExportNextHopAuto(f.destination.settings.ExportPolicy)
	f.destination.settings.FlowSpecRedirectNextHop = optIn
	f.destination.refreshForwardFacts()

	raw := mustHex(t, nlriHex)
	hop := recvHop.AsSlice()
	originalSession := f.source.currentSession()
	require.NotNil(t, originalSession)
	key := ribevents.ValidationRoute{
		Peer: f.source.Settings().Address, PeerOwner: f.source.sourceOwner,
		ConnectionID: connectionIDOf(originalSession), Family: fam, NLRI: string(raw),
	}
	ribevents.RegisterFlowSpecLookup(func(got ribevents.ValidationRoute, _ uint64) bool { return got == key }, nil, nil, nil)
	t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })

	id := f.receive(t, flowRedirectPayload(t, fam, raw, hop))
	f.forwardViaRIB(t, id)
	forwardSocketBarrier(t, f.r)
	bodies := aigpSocketBodies(t, f.conn)
	require.Len(t, bodies, 1)
	assertFlowRedirectWire(t, bodies[0], fam, raw, wantHop, true)
}

// TestFlowSpecDefaultPathOmitsRedirectNextHop proves that on the default
// configuration path (session flowspec-redirect-next-hop omitted or false),
// Ze enforces RFC 8955 Section 4 unconditionally: FlowSpec routes, including
// routes carrying the 0x0800 redirect-to-IP extended community and an explicit
// or received IPv4/IPv6 next hop, emit MP_REACH_NLRI next-hop length 0 on
// every origination rail (config initial sync, API batch, both queued drains)
// and on the forwarding rail for both IPv4 and IPv6.
//
// VALIDATES: PeersFromTree defaults FlowSpecRedirectNextHop to false; default
// origination and forwarding emit exact zero-length MP_REACH next hops for
// IPv4 and IPv6 FlowSpec routes even when 0x0800 and a next hop are present.
// PREVENTS: carrying a non-zero FlowSpec next hop on the default configuration
// path, violating RFC 8955 Section 4.
//
// RFC 8955 Section 4: "When advertising Flow Specifications, the Length of the
// Next-Hop Network Address MUST be set to 0."
// RFC requirement: RFC8955-4-3 positive -- on the default config path parsed by PeersFromTree, FlowSpec routes (including 0x0800 redirect-to-IP) emit an exact zero-length MP_REACH next hop across config, API batch, queued and forwarding rails for IPv4 and IPv6 (§4).
// RFC requirement: RFC8955-4-3 negative -- a configured or received IPv4 or IPv6 next hop on a 0x0800 redirect-to-IP FlowSpec route never leaks into the wire MP_REACH next hop on the default config path (§4).
func TestFlowSpecDefaultPathOmitsRedirectNextHop(t *testing.T) {
	family.RegisterTestFamilies()

	defaultPeers, err := PeersFromTree(flowSpecRedirectPeerTree(nil))
	require.NoError(t, err)
	require.Len(t, defaultPeers, 1)
	require.False(t, defaultPeers[0].FlowSpecRedirectNextHop, "default session must keep RFC 8955 zero-length FlowSpec next hop")

	optInFalse := false
	optOutPeers, err := PeersFromTree(flowSpecRedirectPeerTree(&optInFalse))
	require.NoError(t, err)
	require.Len(t, optOutPeers, 1)
	require.False(t, optOutPeers[0].FlowSpecRedirectNextHop, "explicit flowspec-redirect-next-hop false must enforce RFC 8955")

	ipv4Flow := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	ipv6Flow := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec}
	const (
		redirect = "0800000000000000"
		rateLim  = "8006000046160000"
		ipv4NLRI = "050118c00002"
		ipv6NLRI = "0701200020010db8"
		marker   = "ffffffffffffffffffffffffffffffff"
		wellKnow = "4001010040020040050400000064"
	)

	runFlowSpecRedirectOriginCases(t, []flowSpecRedirectWireCase{
		{
			name: "default-ipv4-ratelimit", fam: ipv4Flow, nlri: ipv4NLRI,
			hop: netip.MustParseAddr("1.2.3.4"), optIn: false, action: rateLim,
			wantFrame: marker + "003e0200000027" + "800e0b0001850000" + ipv4NLRI + wellKnow + "c01008" + rateLim,
		},
		{
			name: "default-ipv4-0x0800", fam: ipv4Flow, nlri: ipv4NLRI,
			hop: netip.MustParseAddr("1.2.3.4"), optIn: false, action: redirect,
			wantFrame: marker + "003e0200000027" + "800e0b0001850000" + ipv4NLRI + wellKnow + "c01008" + redirect,
		},
		{
			name: "default-ipv6-0x0800", fam: ipv6Flow, nlri: ipv6NLRI,
			hop: netip.MustParseAddr("2001:db8::9"), optIn: false, action: redirect,
			wantFrame: marker + "00400200000029" + "800e0d0002850000" + ipv6NLRI + wellKnow + "c01008" + redirect,
		},
	})

	t.Run("forward/default-ipv4-strips", func(t *testing.T) {
		runFlowSpecRedirectForwardCase(t, ipv4Flow, ipv4NLRI, netip.MustParseAddr("192.0.2.77"), false, nil)
	})
	t.Run("forward/default-ipv6-strips", func(t *testing.T) {
		runFlowSpecRedirectForwardCase(t, ipv6Flow, ipv6NLRI, netip.MustParseAddr("2001:db8::77"), false, nil)
	})
}

// TestFlowSpecRedirectNextHopOptIn verifies the explicitly selected
// draft-simpson-idr-flowspec-redirect-02 Section 3 behavior when an operator
// configures `session flowspec-redirect-next-hop true`. This is an untagged
// compatibility control and is never counted as RFC 8955 Section 4 conformance.
//
// VALIDATES: PeersFromTree parses flowspec-redirect-next-hop true; opted-in
// sessions emit the 4/16-octet MP_REACH next hop for 0x0800 routes across all
// origination and forwarding rails while keeping length 0 for non-0x0800
// FlowSpec routes.
// PREVENTS: breaking ExaBGP / draft-simpson redirect-to-IP compatibility on
// opted-in sessions or leaking next hops onto ordinary FlowSpec routes.
func TestFlowSpecRedirectNextHopOptIn(t *testing.T) {
	family.RegisterTestFamilies()

	optInTrue := true
	optInPeers, err := PeersFromTree(flowSpecRedirectPeerTree(&optInTrue))
	require.NoError(t, err)
	require.Len(t, optInPeers, 1)
	require.True(t, optInPeers[0].FlowSpecRedirectNextHop, "explicit flowspec-redirect-next-hop true must opt into draft-simpson")

	ipv4Flow := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	ipv6Flow := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec}
	const (
		redirect = "0800000000000000"
		rateLim  = "8006000046160000"
		ipv4NLRI = "050118c00002"
		ipv6NLRI = "0701200020010db8"
		marker   = "ffffffffffffffffffffffffffffffff"
		wellKnow = "4001010040020040050400000064"
	)

	runFlowSpecRedirectOriginCases(t, []flowSpecRedirectWireCase{
		{
			name: "optin-ipv4-0x0800", fam: ipv4Flow, nlri: ipv4NLRI,
			hop: netip.MustParseAddr("1.2.3.4"), optIn: true, action: redirect,
			wantFrame: marker + "0042020000002b" + "800e0f000185040102030400" + ipv4NLRI + wellKnow + "c01008" + redirect,
		},
		{
			name: "optin-ipv6-0x0800", fam: ipv6Flow, nlri: ipv6NLRI,
			hop: netip.MustParseAddr("2001:db8::9"), optIn: true, action: redirect,
			wantFrame: marker + "00500200000039" + "800e1d0002851020010db800000000000000000000000900" + ipv6NLRI + wellKnow + "c01008" + redirect,
		},
		{
			name: "optin-ipv4-ratelimit-still-zero", fam: ipv4Flow, nlri: ipv4NLRI,
			hop: netip.MustParseAddr("1.2.3.4"), optIn: true, action: rateLim,
			wantFrame: marker + "003e0200000027" + "800e0b0001850000" + ipv4NLRI + wellKnow + "c01008" + rateLim,
		},
	})

	t.Run("forward/optin-ipv4-keeps", func(t *testing.T) {
		hop := netip.MustParseAddr("192.0.2.77")
		runFlowSpecRedirectForwardCase(t, ipv4Flow, ipv4NLRI, hop, true, hop.AsSlice())
	})
	t.Run("forward/optin-ipv6-keeps", func(t *testing.T) {
		hop := netip.MustParseAddr("2001:db8::77")
		runFlowSpecRedirectForwardCase(t, ipv6Flow, ipv6NLRI, hop, true, hop.AsSlice())
	})
}
