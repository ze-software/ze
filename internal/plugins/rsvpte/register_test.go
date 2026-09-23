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
// loop and the refresh loop. Concurrent signaling, refreshes, and reconciliation
// exercise the LSP-table, admission, and per-LSP locking. The FIB is nil so the
// deliberately-unsynchronized fakeFIB stays out of the race.
// PREVENTS: a data race between config-reload tunnel reconciliation and signaling.
func TestReconcileTunnelsConcurrentWithSignaling(t *testing.T) {
	log := slogutil.DiscardLogger()
	routerID := netip.MustParseAddr("10.0.0.1")
	cfgBase := rsvpteConfig{RouterID: routerID, RefreshPeriod: DefaultRefreshPeriod}
	transport := newFakeTransport()
	transport.localAddresses = []netip.Addr{routerID}
	e := newEngine(transport, newLSPTable(), newAdmissionController(), nil, cfgBase, log)

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

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				refreshPaths(log, e.table, e)
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

// configuredTunnelResv returns the downstream response to an originated PATH.
func configuredTunnelResv(t *testing.T, e *engine, key lspKey, label uint32) Packet {
	t.Helper()
	lsp, ok := e.table.Get(key)
	require.True(t, ok)
	lsp.mu.Lock()
	psb, nextHop := lsp.PSB, lsp.NextHop
	lsp.mu.Unlock()
	require.NotNil(t, psb)
	rsb := &resvStateBlock{
		Session: psb.Session, FlowSpec: psb.SenderTSpec,
		Label: labelObject{Label: label}, Style: StyleSharedExplicit,
	}
	return Packet{Src: nextHop, Payload: buildResv(rsb, psb.SenderTemplate, DefaultRefreshPeriod, nextHop)}
}

// Withdrawal must remove the established generation and every pending attempt,
// including after the configured generation has already retired.
func TestReconcileTunnelsWithdrawsReoptimizedGenerations(t *testing.T) {
	for _, complete := range []bool{false, true} {
		name := "replacement-pending"
		if complete {
			name = "replacement-established"
		}
		t.Run(name, func(t *testing.T) {
			e, ft, fib := testEngine(t, "10.0.0.1", nil)
			cfg := e.cfg()
			cfg.Tunnels = []tunnelConfig{
				{Name: "removed", Destination: netip.MustParseAddr("10.0.0.9"), TunnelID: 1},
				{Name: "kept", Destination: netip.MustParseAddr("10.0.0.8"), TunnelID: 1},
			}
			cfg.Bypasses = []bypassConfig{{Name: "kept-bypass", MergePoint: cfg.Tunnels[0].Destination}}
			prev := reconcileTunnels(e.log, e.table, cfg, e, nil)
			original := tunnelKey(cfg.Tunnels[0], cfg.RouterID)
			kept := tunnelKey(cfg.Tunnels[1], cfg.RouterID)
			bypass := bypassKey(cfg.Bypasses[0], cfg.RouterID)
			for _, key := range []lspKey{original, kept, bypass} {
				e.handlePacket(configuredTunnelResv(t, e, key, 17000))
				require.Equal(t, LSPStateUp, mustLSP(t, e, key).State)
			}

			lsp := mustLSP(t, e, original)
			notify := buildPathErr(lsp.PSB.Session, lsp.PSB.SenderTemplate, lsp.PSB.SenderTSpec,
				errorSpec{ErrorNode: lsp.NextHop, ErrorCode: ErrCodeNotify, ErrorValue: ErrValueTunnelLocallyRepaired}, lsp.NextHop)
			e.handlePacket(Packet{Src: lsp.NextHop, Payload: notify})
			replacement := original
			replacement.LSPID++
			reply := configuredTunnelResv(t, e, replacement, 18000)
			if complete {
				e.handlePacket(reply)
				require.Equal(t, LSPStateUp, mustLSP(t, e, replacement).State)
				_, exists := e.table.Get(original)
				require.False(t, exists, "MBB retired the configured generation before withdrawal")
			}

			mark := len(ft.sent)
			cfg.Tunnels = cfg.Tunnels[1:]
			reconcileTunnels(e.log, e.table, cfg, e, prev)
			for _, key := range []lspKey{original, replacement} {
				_, exists := e.table.Get(key)
				assert.False(t, exists, "withdrawal left generation %d", key.LSPID)
			}
			assert.Equal(t, LSPStateUp, mustLSP(t, e, kept).State, "same tunnel ID at another endpoint survives")
			assert.Equal(t, LSPStateUp, mustLSP(t, e, bypass).State, "same-endpoint bypass survives")
			assert.Contains(t, fib.removed, netip.PrefixFrom(original.TunnelEndpoint, 32))
			assert.NotContains(t, fib.removeTables, bypassTableID(bypass))
			assert.NotContains(t, fib.removed, netip.PrefixFrom(kept.TunnelEndpoint, 32))
			tears := make(map[lspKey]bool)
			for _, sent := range ft.sent[mark:] {
				message, err := DecodeMessage(sent.payload)
				require.NoError(t, err)
				if message.Header.MsgType == MsgTypePathTear {
					tears[keyFromMessage(message)] = true
				}
			}
			expected := map[lspKey]bool{replacement: true}
			if !complete {
				expected[original] = true
			}
			assert.Equal(t, expected, tears, "every withdrawn generation signals its own PathTear")

			pushes := len(fib.pushed)
			e.handlePacket(reply)
			_, exists := e.table.Get(replacement)
			assert.False(t, exists, "a late RESV cannot resurrect a withdrawn replacement")
			assert.Len(t, fib.pushed, pushes, "late RESV cannot restore the withdrawn FEC")
		})
	}
}

// Reload follows the current generation and can supersede a pending route change
// without abandoning the established predecessor or accepting a stale RESV.
func TestReconcileTunnelsUpdatesCurrentGeneration(t *testing.T) {
	e, ft, fib := testEngine(t, "10.0.0.1", nil)
	cfg := e.cfg()
	cfg.Tunnels = []tunnelConfig{{
		Name: "moving", Destination: netip.MustParseAddr("10.0.0.9"), TunnelID: 1,
		ERO: []eroHop{{Address: netip.MustParsePrefix("10.0.0.5/32")}},
	}}
	prev := reconcileTunnels(e.log, e.table, cfg, e, nil)
	first := tunnelKey(cfg.Tunnels[0], cfg.RouterID)
	e.handlePacket(configuredTunnelResv(t, e, first, 17000))
	require.Equal(t, LSPStateUp, mustLSP(t, e, first).State)
	cfg.Tunnels[0].ERO = []eroHop{{Address: netip.MustParsePrefix("10.0.0.6/32")}}
	prev = reconcileTunnels(e.log, e.table, cfg, e, prev)
	current := first
	current.LSPID = 2
	e.handlePacket(configuredTunnelResv(t, e, current, 18000))
	require.Equal(t, LSPStateUp, mustLSP(t, e, current).State)
	mark := len(ft.sent)
	prev = reconcileTunnels(e.log, e.table, cfg, e, prev)
	assert.Len(t, ft.sent, mark, "unchanged reload must not recreate the retired generation")
	_, exists := e.table.Get(first)
	assert.False(t, exists)

	cfg.Tunnels[0].ERO = []eroHop{{Address: netip.MustParsePrefix("10.0.0.7/32")}}
	prev = reconcileTunnels(e.log, e.table, cfg, e, prev)
	pending := current
	pending.LSPID = 3
	staleReply := configuredTunnelResv(t, e, pending, 19000)
	mark = len(ft.sent)
	prev = reconcileTunnels(e.log, e.table, cfg, e, prev)
	assert.Len(t, ft.sent, mark, "unchanged reload leaves the pending attempt alone")

	cfg.Tunnels[0].ERO = []eroHop{{Address: netip.MustParsePrefix("10.0.0.8/32")}}
	reconcileTunnels(e.log, e.table, cfg, e, prev)
	latest := current
	latest.LSPID = 4
	path, nextHop, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok)
	assert.Equal(t, latest, keyFromMessage(path), "new ERO belongs to a fresh generation")
	assert.Equal(t, netip.MustParseAddr("10.0.0.8"), nextHop)
	assert.Equal(t, LSPStateUp, mustLSP(t, e, current).State, "established predecessor still carries traffic")
	assert.Empty(t, fib.removed, "superseding a pending attempt preserves the active FEC")
	_, exists = e.table.Get(pending)
	assert.False(t, exists, "superseded pending attempt is torn down")

	// Another repair notification for the established predecessor must find
	// generation 4, even though its immediately following generation is gone.
	lsp := mustLSP(t, e, current)
	notify := buildPathErr(lsp.PSB.Session, lsp.PSB.SenderTemplate, lsp.PSB.SenderTSpec,
		errorSpec{ErrorNode: lsp.NextHop, ErrorCode: ErrCodeNotify, ErrorValue: ErrValueTunnelLocallyRepaired}, lsp.NextHop)
	mark = ft.countByType(MsgTypePath)
	e.handlePacket(Packet{Src: lsp.NextHop, Payload: notify})
	assert.Equal(t, mark, ft.countByType(MsgTypePath), "repeated Notify cannot restart the abandoned generation")
	pushes := len(fib.pushed)
	e.handlePacket(staleReply)
	assert.Len(t, fib.pushed, pushes, "abandoned attempt cannot take over the FEC")
	e.handlePacket(configuredTunnelResv(t, e, latest, 20000))
	require.Equal(t, LSPStateUp, mustLSP(t, e, latest).State)
	assert.Equal(t, []uint32{20000}, fib.pushLabels[len(fib.pushLabels)-1])
	for _, key := range []lspKey{first, current, pending} {
		_, exists = e.table.Get(key)
		assert.False(t, exists, "latest RESV must retire predecessor %d", key.LSPID)
	}
	assert.Empty(t, fib.removed, "MBB completion keeps the newly accepted FEC")
}

func TestReconcileTunnelsSkipsOccupiedLSPID(t *testing.T) {
	e, ft, fib := testEngine(t, "10.0.0.1", nil)
	cfg := e.cfg()
	cfg.Tunnels = []tunnelConfig{{
		Name: "wrapped", Destination: netip.MustParseAddr("10.0.0.9"), TunnelID: 1,
		ERO: []eroHop{{Address: netip.MustParsePrefix("10.0.0.5/32")}},
	}}
	prev := reconcileTunnels(e.log, e.table, cfg, e, nil)
	first := tunnelKey(cfg.Tunnels[0], cfg.RouterID)
	e.handlePacket(configuredTunnelResv(t, e, first, 17000))
	established := mustLSP(t, e, first)
	require.Equal(t, LSPStateUp, established.State)

	// Seed a legal pending generation near wrap instead of issuing 65534 updates.
	pendingKey := first
	pendingKey.LSPID = 65535
	pending, existed := e.table.GetOrCreate(pendingKey)
	require.False(t, existed)
	pending.Role = RoleIngress
	pending.Replaces = &first
	pending.PSB = &pathStateBlock{
		Session: established.PSB.Session,
		SenderTemplate: senderTemplateIPv4{
			SenderAddr: pendingKey.SenderAddr, LSPID: pendingKey.LSPID,
		},
		SenderTSpec:   established.PSB.SenderTSpec,
		LabelRequest:  established.PSB.LabelRequest,
		ERO:           []eroHop{{Address: netip.MustParsePrefix("10.0.0.6/32")}},
		RefreshPeriod: cfg.RefreshPeriod,
	}
	pending.setState(LSPStatePathSent)
	require.NoError(t, e.sendPath(pending))

	cfg.Tunnels[0].ERO = []eroHop{{Address: netip.MustParsePrefix("10.0.0.7/32")}}
	prev = reconcileTunnels(e.log, e.table, cfg, e, prev)
	wrapped := first
	wrapped.LSPID = 0
	path, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok)
	require.Equal(t, wrapped, keyFromMessage(path))

	cfg.Tunnels[0].ERO = []eroHop{{Address: netip.MustParsePrefix("10.0.0.8/32")}}
	reconcileTunnels(e.log, e.table, cfg, e, prev)
	latest := first
	latest.LSPID = 2
	path, nextHop, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok)
	assert.Equal(t, latest, keyFromMessage(path), "skip the established predecessor's occupied ID")
	assert.Equal(t, netip.MustParseAddr("10.0.0.8"), nextHop)
	assert.Equal(t, LSPStateUp, mustLSP(t, e, first).State)
	assert.Empty(t, fib.removed, "the established predecessor still carries traffic")
	e.handlePacket(configuredTunnelResv(t, e, latest, 19000))
	require.Equal(t, LSPStateUp, mustLSP(t, e, latest).State)
	assert.Equal(t, []uint32{19000}, fib.pushLabels[len(fib.pushLabels)-1])
	_, exists := e.table.Get(first)
	assert.False(t, exists, "the accepted replacement retires the retained predecessor")
}
