// Design: plan/spec-mpls-3-rsvp-te.md -- tunnel reconciliation on config reload
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
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
