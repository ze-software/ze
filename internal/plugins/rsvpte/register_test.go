// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- tunnel reconciliation on config reload
package rsvpte

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: mpls-3 reload -- reconcileTunnels sets up every configured tunnel as an
// ingress LSP and tears down the LSP of a tunnel removed from config, originating a
// PathTear. This is the OnConfigApply reload path; without it a removed tunnel would
// leak its LSP and forwarding state (the gap before this change).
// PREVENTS: tunnel removal on reload leaving a stranded LSP, and an added tunnel not
// signaling.
func TestReconcileTunnelsSetupAndTeardown(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.1", nil)
	log := slogutil.DiscardLogger()
	cfg := rsvpteConfig{RouterID: netip.MustParseAddr("10.0.0.1"), RefreshPeriod: DefaultRefreshPeriod}
	cfg.Tunnels = []tunnelConfig{
		{Name: "t1", Destination: netip.MustParseAddr("10.0.0.9"), TunnelID: 1, Bandwidth: 1e8},
		{Name: "t2", Destination: netip.MustParseAddr("10.0.0.8"), TunnelID: 2, Bandwidth: 1e8},
	}
	t1key := tunnelKey(cfg.Tunnels[0], cfg.RouterID)
	t2key := tunnelKey(cfg.Tunnels[1], cfg.RouterID)

	// Initial reconcile (prev empty): both tunnels become ingress LSPs, each sends
	// a PATH; nothing is torn down.
	prev := reconcileTunnels(log, e.table, cfg, e, nil)
	require.Len(t, prev, 2, "two tunnels configured")
	require.Contains(t, prev, t1key)
	require.Contains(t, prev, t2key)
	assert.Len(t, e.table.All(), 2, "two ingress LSPs created")
	if _, _, ok := ft.lastByType(MsgTypePath); !ok {
		t.Fatal("setup originated a PATH")
	}
	if _, _, ok := ft.lastByType(MsgTypePathTear); ok {
		t.Fatal("nothing should be torn down on the first reconcile")
	}

	// Remove t2 from config and reconcile: t2's LSP is torn down, t1 survives.
	cfg.Tunnels = cfg.Tunnels[:1]
	next := reconcileTunnels(log, e.table, cfg, e, prev)
	require.Len(t, next, 1, "one tunnel remains configured")
	require.Contains(t, next, t1key)
	assert.NotContains(t, next, t2key)

	_, ok := e.table.Get(t1key)
	assert.True(t, ok, "t1 LSP survives the reconcile")
	_, ok = e.table.Get(t2key)
	assert.False(t, ok, "t2 LSP removed when its tunnel left config")

	// The teardown originated a PathTear toward t2's egress (RFC 2205 Section 3.1.5).
	tear, dst, ok := ft.lastByType(MsgTypePathTear)
	require.True(t, ok, "removing t2 originated a PathTear")
	assert.Equal(t, netip.MustParseAddr("10.0.0.8"), dst, "PathTear sent toward t2's destination")
	assert.Equal(t, uint16(2), tear.Session.TunnelID, "PathTear carries t2's session")
}

// VALIDATES: mpls-3 reload -- reconcileTunnels is safe to run concurrently with live
// signaling. OnConfigApply reconciles on a different goroutine than the engine's run
// loop (as the link-down handler already does), so two goroutines hammer one engine:
// one feeds egress PATHs through handlePacket, the other reconciles the tunnel set up
// and down. -race exercises the LSP-table, admission and per-LSP locking. fib is nil
// so the deliberately-unsynchronized fakeFIB stays out of the race -- the engine's
// own state is what must be safe.
// PREVENTS: a data race between config-reload tunnel reconciliation and signaling.
func TestReconcileTunnelsConcurrentWithSignaling(t *testing.T) {
	log := slogutil.DiscardLogger()
	routerID := netip.MustParseAddr("10.0.0.1")
	cfgBase := rsvpteConfig{RouterID: routerID, RefreshPeriod: DefaultRefreshPeriod}
	e := newEngine(newFakeTransport(), newLSPTable(), newAdmissionController(), nil, cfgBase, log)

	cfg := cfgBase
	cfg.Tunnels = []tunnelConfig{
		{Name: "t1", Destination: netip.MustParseAddr("10.0.0.9"), TunnelID: 1, Bandwidth: 1e8},
		{Name: "t2", Destination: netip.MustParseAddr("10.0.0.8"), TunnelID: 2, Bandwidth: 1e8},
	}

	// An egress PATH this node terminates (SESSION endpoint == its router-id).
	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: routerID, TunnelID: 9, ExtTunnelID: 1},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.2"), LSPID: 1},
		SenderTSpec:    FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8},
		LabelRequest:   labelRequest{L3PID: 0x0800},
	}
	pathBytes := buildPath(psb, netip.MustParseAddr("10.0.0.2"), 64)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Signaling: repeatedly process an egress PATH (creates then refreshes the LSP).
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.2"), Payload: pathBytes})
			}
		}
	})

	// Reconcile: repeatedly bring the tunnel set up and tear it all down.
	wg.Go(func() {
		var prev map[lspKey]bool
		for range 300 {
			prev = reconcileTunnels(log, e.table, cfg, e, prev)
			prev = reconcileTunnels(log, e.table, cfgBase, e, prev)
		}
		close(stop)
	})

	wg.Wait()

	// The -race result (no report) is the point. As a correctness check, the final
	// reconcile (with no tunnels) left both ingress tunnel LSPs torn down; the egress
	// LSP the signaling loop created is not a tunnel, so it legitimately remains.
	_, ok1 := e.table.Get(tunnelKey(cfg.Tunnels[0], routerID))
	_, ok2 := e.table.Get(tunnelKey(cfg.Tunnels[1], routerID))
	assert.False(t, ok1, "t1 ingress LSP torn down by the final reconcile")
	assert.False(t, ok2, "t2 ingress LSP torn down by the final reconcile")
}

// TestReconcileTunnelsNoRouterIDNoPanic: reconcileTunnels must not panic when the
// config has tunnels/bypasses but no router-id (tunnelKey/bypassKey derive a
// tunnel-id from the router-id, whose As4() panics on the zero Addr). OnConfigApply
// reaches this path on reload without the OnStarted guard.
func TestReconcileTunnelsNoRouterIDNoPanic(t *testing.T) {
	cfg := rsvpteConfig{
		// no RouterID
		Tunnels:  []tunnelConfig{{Name: "t1", Destination: netip.MustParseAddr("10.0.0.9"), TunnelID: 1}},
		Bypasses: []bypassConfig{{Name: "bp", MergePoint: netip.MustParseAddr("10.0.0.3")}},
	}
	got := reconcileTunnels(slogutil.DiscardLogger(), newLSPTable(), cfg, nil, nil) // must not panic
	assert.Empty(t, got, "nothing reconciled without a router-id")
}
