// Design: docs/architecture/rib-transition.md -- scoped origin arbitration.
// Related: policy_scoped_origin_reject_test.go -- scoped origin socket fixture.
package reactor

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/route"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestSelectedOriginResultRetainsLoserAcrossTransports proves that an API
// caller can distinguish a retained loser from a failure and that the same
// admitted origin reaches its recipient when the learned winner disappears.
func TestSelectedOriginResultRetainsLoserAcrossTransports(t *testing.T) {
	for _, transport := range []string{"typed", "framed"} {
		t.Run(transport, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			const learnedHop, originHop = "198.18.249.1", "198.18.249.2"
			const producer = "selected-origin-result-test"
			peers, caller := startSelectedOriginResultPeers(t, ctx, producer, transport == "typed", learnedHop, originHop)
			source, a, b, apiOnly := peers[0], peers[1], peers[2], peers[3]
			selected := selector.ParseDefault(a.peer.addrString)
			key := netip.MustParsePrefix("198.19.193.0/24")
			marker := netip.MustParsePrefix("198.19.194.0/24")
			ownershipLiveSend(t, source, nil, synthesizedNextHopAttrs(netip.MustParseAddr(learnedHop), 65010), key, marker)
			ordinarySelectedStored(t, source, key)
			ordinarySelectedBest(t, key, learnedHop)
			learnedA := ordinarySelectedWire(t, a, key, learnedHop).routes[key]
			learnedB := ordinarySelectedWire(t, b, key, learnedHop).routes[key]
			markerA := ordinarySelectedWire(t, a, marker, learnedHop).routes[marker]
			markerB := ordinarySelectedWire(t, b, marker, learnedHop).routes[marker]

			for _, prefix := range []netip.Prefix{key, marker} {
				out, err := caller.UpdateRouteSelResult(ctx, selected, "update text nhop "+originHop+" nlri add "+prefix.String())
				require.NoError(t, err)
				require.NotNil(t, out)
				require.Zero(t, out.Announced)
				require.Empty(t, out.Warnings)
				require.Empty(t, out.Failure)
				require.Len(t, out.Admissions, 1)
				admission := out.Admissions[0]
				address := prefix.Addr().As4()
				require.Equal(t, []byte{24, address[0], address[1], address[2]}, admission.KeyNLRI)
				require.Equal(t, family.IPv4Unicast.String(), admission.Family)
				require.Equal(t, rpc.RouteAdmissionRetained, admission.Outcome)
				require.Equal(t, rpc.RouteAdmissionSuppressedSelection, admission.Disposition)
				require.Equal(t, a.peer.addrString, admission.Destination)
				require.Equal(t, a.peer.sourceOwner, admission.PeerOwner)
				require.NotZero(t, admission.Generation)
			}
			out, err := caller.UpdateRouteSelResult(ctx, selected, "update text nlri del "+marker.String())
			require.NoError(t, err)
			require.NotNil(t, out)
			require.Equal(t, uint32(1), out.Withdrawn, "removing a retained loser keeps the learned winner")
			out, err = caller.UpdateRouteSelResult(ctx, selector.ParseDefault(apiOnly.peer.addrString), "update text nlri del "+marker.String())
			require.NoError(t, err)
			require.NotNil(t, out)
			require.Equal(t, uint32(1), out.Withdrawn, "a completed legacy withheld withdrawal keeps its count")
			require.Len(t, out.Warnings, 1)
			require.Contains(t, out.Warnings[0], route.ErrWithdrawWithheld.Error())
			require.Contains(t, out.Warnings[0], apiOnly.peer.addrString)
			out, err = caller.UpdateRouteSelResult(ctx, selected, "update text nhop 2001:db8::1 mp ipv6/unicast add 2001:db8:193::/48")
			require.ErrorContains(t, err, route.ErrNoPeerConfiguredFamily.Error()+": ipv6/unicast")
			require.NotNil(t, out)
			require.ErrorContains(t, out.Err(), route.ErrNoPeerConfiguredFamily.Error())
			require.Empty(t, out.Admissions)
			require.Zero(t, out.Announced)

			ordinaryBoundaryDrain(t, source)
			ordinaryBoundaryCapture(t, a)
			before := ownershipLiveRead(t, a)
			ownershipLiveRoute(t, before, key, learnedA)
			ownershipLiveAnnouncements(t, before, key, learnedA)
			ownershipLiveWithdrawals(t, before, key, 0)
			ownershipLiveAnnouncements(t, before, marker, markerA)
			ownershipLiveWithdrawals(t, before, marker, 0)

			ownershipLiveSend(t, source, []netip.Prefix{key}, nil)
			local := ordinarySelectedWire(t, a, key, originHop).routes[key]
			ordinaryBoundaryDrain(t, source)
			ordinaryBoundaryCapture(t, a)
			ordinaryBoundaryCapture(t, b)
			ordinaryBoundaryCapture(t, apiOnly)
			afterA, afterB := ownershipLiveRead(t, a), ownershipLiveRead(t, b)
			ownershipLiveAnnouncements(t, afterA, key, learnedA, local)
			ownershipLiveRoute(t, afterA, marker, markerA)
			ownershipLiveAnnouncements(t, afterA, marker, markerA)
			ownershipLiveWithdrawals(t, afterA, marker, 0)
			ownershipLiveAbsent(t, afterB, key)
			ownershipLiveAnnouncements(t, afterB, key, learnedB)
			ownershipLiveWithdrawals(t, afterB, key, 1)
			ownershipLiveRoute(t, afterB, marker, markerB)
			ownershipLiveAnnouncements(t, afterB, marker, markerB)
			ownershipLiveWithdrawals(t, afterB, marker, 0)
			ownershipLiveWithdrawals(t, ownershipLiveRead(t, apiOnly), marker, 0)
			ownershipLiveEstablished(t, peers)

			out, err = caller.UpdateRouteSelResult(ctx, selected, "update text nlri del "+key.String())
			require.NoError(t, err)
			require.NotNil(t, out)
			require.Equal(t, uint32(1), out.Withdrawn)
			ordinaryBoundaryDrain(t, source)
			ordinaryBoundaryCapture(t, a)
			ownershipLiveAbsent(t, ownershipLiveRead(t, a), key)
		})
	}
}

func startSelectedOriginResultPeers(t *testing.T, ctx context.Context, producer string, typed bool, hops ...string) ([]*lowLivePeer, *sdk.Plugin) {
	t.Helper()
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	type readyOriginCaller struct {
		client *sdk.Plugin
		bridge *rpc.DirectBridge
	}
	started := make(chan readyOriginCaller, 1)
	require.NoError(t, registry.Register(registry.Registration{
		Name: producer, CLIHandler: func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			if !typed {
				conn = struct{ net.Conn }{conn}
			}
			bridger, bridged := conn.(rpc.Bridger)
			if bridged != typed {
				t.Errorf("SDK connection bridge presence = %t, want %t", bridged, typed)
				return 1
			}
			var bridge *rpc.DirectBridge
			if bridged {
				bridge = bridger.Bridge()
			}
			caller := sdk.NewWithConn(producer, conn)
			caller.OnStarted(func(context.Context) error {
				started <- readyOriginCaller{client: caller, bridge: bridge}
				return nil
			})
			if err := caller.Run(t.Context(), &sdk.Registration{}); err != nil && t.Context().Err() == nil {
				t.Errorf("origin caller: %v", err)
				return 1
			}
			return 0
		},
	}))
	settings := []*PeerSettings{
		acceptingLiveSettings(t, "192.0.2.193", 65000, 65010),
		acceptingLiveSettings(t, "192.0.2.194", 65000, 65000),
		acceptingLiveSettings(t, "192.0.2.195", 65000, 65000),
	}
	prepareScopedOriginOwners(t, settings)
	settings = append(settings, acceptingLiveSettings(t, "192.0.2.196", 65000, 65000))
	lowLiveAttachProcess(t, settings[1], producer, "")
	lowLiveAttachProcess(t, settings[3], producer, "")
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	for i, peer := range settings {
		if i < 3 {
			lowLiveAttachProcess(t, peer, "bgp-adj-rib-in", "update-received state")
		}
		require.NoError(t, r.AddPeer(peer))
	}
	srv := newBorrowedPluginServerOver(t, &reactorAPIAdapter{r: r}, "bgp-rib", "bgp-adj-rib-in", producer)
	r.SetPluginServer(srv)
	require.NoError(t, r.StartWithContext(context.Background()))
	t.Cleanup(func() { stopAndWait(t, r) })
	startBorrowedPeers(t, r, srv)
	connection := srv.ProcessManager().GetProcess("bgp-rib").Conn()
	sections := []rpc.ConfigSection{{Root: "bgp", Data: `{"bgp":{}}`}, {Root: "rib", Data: `{"rib":{"distance":{"static":"250"}}}`}}
	verified, err := connection.SendConfigVerify(ctx, sections)
	require.NoError(t, err)
	require.Equal(t, rpc.StatusOK, verified.Status)
	applied, err := connection.SendConfigApply(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, rpc.StatusOK, applied.Status)

	loc := locrib.Default()
	require.NotNil(t, loc)
	protocol := redistevents.RegisterProtocol(producer)
	for _, hop := range hops {
		host := netip.PrefixFrom(netip.MustParseAddr(hop), 32)
		loc.Insert(family.IPv4Unicast, host, locrib.Path{Source: protocol, Metric: 7})
		t.Cleanup(func() { loc.Remove(family.IPv4Unicast, host, protocol, 0) })
	}
	peers := make([]*lowLivePeer, 0, len(settings))
	for i, peer := range settings {
		peers = append(peers, lowLiveConnect(t, r.peers[peer.PeerKey()], 0x0a00f301+uint32(i)))
	}
	select {
	case caller := <-started:
		if typed {
			require.NotNil(t, caller.bridge)
			require.True(t, caller.bridge.HasUpdateRouteSelResult(), "typed call must use the dedicated result slot")
		}
		return peers, caller.client
	case <-ctx.Done():
		t.Fatal("origin caller did not complete SDK startup")
		return nil, nil
	}
}
