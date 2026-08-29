// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RSVP-TE config-reload completeness
//
// A commit that changes the rsvp-te config must reach the running engine. These tests
// cover the two places it did not: the refresh and cleanup loops, which ran on the
// configuration they were started with, and the admission controller, which grew an
// entry for every interface ever configured and dropped none.
package rsvpte

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// ingressTestLSP puts one up ingress LSP in the engine's table, signaled with period.
func ingressTestLSP(t *testing.T, e *engine, period time.Duration) *LSP {
	t.Helper()
	key := lspKey{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1,
		SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1,
	}
	lsp, _ := e.table.GetOrCreate(key)
	lsp.Role = RoleIngress
	lsp.PSB = &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: key.TunnelEndpoint, TunnelID: 1},
		SenderTemplate: senderTemplateIPv4{SenderAddr: key.SenderAddr, LSPID: 1},
		SenderTSpec:    FlowSpec{TokenRate: 1e8},
		LabelRequest:   labelRequest{L3PID: 0x0800},
		RefreshPeriod:  period,
		LastRefresh:    time.Now(),
	}
	lsp.setState(LSPStateUp)
	return lsp
}

// VALIDATES: AC-1 -- after a commit changes refresh-period, the period ze ADVERTISES
// in TIME_VALUES and the cadence it actually refreshes at are the same value. The two
// used to diverge: the PATH carried the period the LSP was signaled with while the
// ticker kept the period the loop was started with, so a neighbor deriving its
// cleanup timeout from the advertised period deleted a reservation ze believed it was
// still refreshing (RFC 2205 Section 3.7).
// PREVENTS: a refresh-period commit costing the operator the tunnel it was meant to
// tune.
func TestRefreshTickAdoptsReloadedPeriod(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.1", func(c *rsvpteConfig) { c.RefreshPeriod = 30 * time.Second })
	startup := e.cfg()
	ingressTestLSP(t, e, startup.RefreshPeriod)

	// The operator commits refresh-period 10. OnConfigApply pushes it to the engine.
	reloaded := startup
	reloaded.RefreshPeriod = 10 * time.Second
	e.setConfig(reloaded)

	cadence, changed := refreshTick(slogutil.DiscardLogger(), e.table, startup, e, startup.RefreshPeriod)
	require.True(t, changed, "the tick adopts the committed period")
	assert.Equal(t, 10*time.Second, cadence, "the next refresh is one committed period away")

	path, _, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok, "the tick refreshed the ingress LSP")
	require.True(t, path.HasTimeValues)
	advertised := time.Duration(path.TimeValues.RefreshPeriod) * time.Millisecond
	assert.Equal(t, cadence, advertised,
		"the advertised refresh period and the actual refresh cadence must agree")
}

// VALIDATES: AC-3 -- a commit that leaves refresh-period alone re-periods nothing.
// time.Ticker.Reset restarts the interval, so a tick that reset unconditionally would
// push the next refresh a further full period out on every commit.
// PREVENTS: an unrelated commit delaying a refresh past the neighbor's cleanup
// timeout.
func TestRefreshTickIdempotentOnUnchangedPeriod(t *testing.T) {
	e, _, _ := testEngine(t, "10.0.0.1", func(c *rsvpteConfig) { c.RefreshPeriod = 30 * time.Second })
	cfg := e.cfg()
	ingressTestLSP(t, e, cfg.RefreshPeriod)

	// A commit that changes something else entirely: the period is untouched.
	reloaded := cfg
	reloaded.Interfaces = []ifaceConfig{{Name: "eth0", MaxBW: 1e9, MaxReservableBW: 1e9}}
	e.setConfig(reloaded)

	cadence, changed := refreshTick(slogutil.DiscardLogger(), e.table, cfg, e, cfg.RefreshPeriod)
	assert.False(t, changed, "an unchanged period must not re-period the ticker")
	assert.Equal(t, 30*time.Second, cadence)
}

// VALIDATES: AC-2 -- the cleanup tick judges the expiry deadline by the LIVE refresh
// multiplier. Started with a multiplier of 10 the two-minute-old PSB below is inside
// its five-minute lifetime; after a commit lowering the multiplier to 1 the lifetime
// is 30 seconds, the same PSB is past it, and the tick tears it down.
// PREVENTS: a refresh-multiplier commit that only takes effect on a daemon restart.
func TestCleanupTickUsesReloadedMultiplier(t *testing.T) {
	log := slogutil.DiscardLogger()
	e, _, _ := testEngine(t, "10.0.0.1", func(c *rsvpteConfig) {
		c.RefreshPeriod = 30 * time.Second
		c.RefreshMultiplier = 10
	})
	startup := e.cfg()
	lsp := ingressTestLSP(t, e, startup.RefreshPeriod)
	lsp.mu.Lock()
	lsp.PSB.LastRefresh = time.Now().Add(-2 * time.Minute)
	lsp.mu.Unlock()

	// 2 minutes old, 30s period, multiplier 10: the lifetime is 5 minutes, not passed.
	_, changed := cleanupTick(log, e.table, startup, e, time.Now(), startup.RefreshPeriod)
	assert.False(t, changed, "the period did not change")
	_, alive := e.table.Get(lsp.Key)
	require.True(t, alive, "with the startup multiplier the LSP is still within its lifetime")

	// The operator commits refresh-multiplier 1: the deadline is now 30 seconds.
	reloaded := startup
	reloaded.RefreshMultiplier = 1
	e.setConfig(reloaded)

	cleanupTick(log, e.table, startup, e, time.Now(), startup.RefreshPeriod)
	_, alive = e.table.Get(lsp.Key)
	assert.False(t, alive, "the committed multiplier expires the stale LSP")
}

// VALIDATES: the loops keep running on their launch config when no engine exists.
// Without a transport nothing is signaled, so there is no engine to read a reloaded
// config from and no neighbor whose cleanup timeout depends on the cadence.
func TestLiveConfigWithoutEngine(t *testing.T) {
	cfg := rsvpteConfig{RefreshPeriod: 30 * time.Second, RefreshMultiplier: 3}
	assert.Equal(t, cfg, liveConfig(cfg, nil))
}

// VALIDATES: adoptedRefreshPeriod refuses a period time.Ticker.Reset would reject or
// that the `refresh-period` YANG range (1..65535 seconds) excludes, keeping the
// running one instead of panicking the daemon.
func TestAdoptedRefreshPeriodBounds(t *testing.T) {
	cases := []struct {
		name       string
		current    time.Duration
		configured time.Duration
		want       time.Duration
		wantChange bool
	}{
		{"changed", 30 * time.Second, 10 * time.Second, 10 * time.Second, true},
		{"unchanged", 30 * time.Second, 30 * time.Second, 30 * time.Second, false},
		{"zero keeps the running period", 30 * time.Second, 0, 30 * time.Second, false},
		{"negative keeps the running period", 30 * time.Second, -time.Second, 30 * time.Second, false},
		{"at the ceiling", 30 * time.Second, maxRefreshPeriod, maxRefreshPeriod, true},
		{"above the ceiling", 30 * time.Second, maxRefreshPeriod + time.Second, 30 * time.Second, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := adoptedRefreshPeriod(tc.current, tc.configured)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantChange, changed)
		})
	}
}

// VALIDATES: AC-4 and AC-5 -- reconcileInterfaces drops the admission state of an
// interface the operator removed, and leaves a kept interface's live reservation
// alone (the mpls-4 read-modify-write invariant).
// PREVENTS: `show rsvp-te interface` reporting a link that is no longer configured,
// and its reserved bandwidth counting against a link ze no longer accounts for.
func TestReconcileInterfacesRemovesDropped(t *testing.T) {
	log := slogutil.DiscardLogger()
	adm := newAdmissionController()
	cfg := rsvpteConfig{Interfaces: []ifaceConfig{
		{Name: "eth0", MaxBW: 1e9, MaxReservableBW: 1e9},
		{Name: "eth1", MaxBW: 1e9, MaxReservableBW: 1e9},
	}}

	prev := reconcileInterfaces(log, adm, cfg, nil)
	require.Len(t, prev, 2)

	// Both links carry a live reservation before the reload.
	sess := sessionID{endpoint: netip.MustParseAddr("10.0.0.9"), tunnelID: 1}
	require.NoError(t, adm.reserveSession("eth0", sess, 2e8))
	require.NoError(t, adm.reserveSession("eth1", sess, 3e8))

	// The operator removes eth1 and commits.
	cfg.Interfaces = cfg.Interfaces[:1]
	next := reconcileInterfaces(log, adm, cfg, prev)

	assert.Equal(t, map[string]bool{"eth0": true}, next, "eth1 left the configured set")
	_, listed := adm.GetInterface("eth1")
	assert.False(t, listed, "a removed interface is no longer serviced")
	assert.NotContains(t, adm.allInterfaces(), "eth1", "show rsvp-te interface drops it")

	kept, ok := adm.GetInterface("eth0")
	require.True(t, ok, "the kept interface survives the reload")
	assert.Equal(t, 2e8, kept.ReservedBandwidth, "its live reservation is preserved")
}

// VALIDATES: the lifetime of state a neighbor creates here follows the period THAT
// NEIGHBOR advertises, not this node's configured one (RFC 2205 Section 3.7). Reading
// the local period made a refresh-period commit shorten the lifetime of state ze does
// not refresh itself, expiring a reservation the neighbor was still refreshing.
// PREVENTS: a local refresh-period commit tearing down a peer's live LSP.
func TestEgressStateLifetimeFollowsSenderPeriod(t *testing.T) {
	// The local period is 1 second; the sender advertises 300.
	e, _, _ := testEngine(t, "10.0.0.9", func(c *rsvpteConfig) { c.RefreshPeriod = time.Second })
	psb := egressTestPSB()
	psb.RefreshPeriod = 300 * time.Second
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.1"), Payload: buildPath(psb, netip.MustParseAddr("10.0.0.1"), 64)})

	key := lspKey{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1, ExtTunnelID: 0x0a000001,
		SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1,
	}
	lsp, ok := e.table.Get(key)
	require.True(t, ok)
	lsp.mu.Lock()
	stored := lsp.PSB.RefreshPeriod
	lsp.mu.Unlock()
	assert.Equal(t, 300*time.Second, stored, "the lifetime derives from the sender's TIME_VALUES")

	// Three local periods on, the state is nowhere near its lifetime.
	assert.NotContains(t, e.table.expiredPSBs(time.Now().Add(3*time.Second), 3), key,
		"a short local refresh-period must not expire state the sender refreshes slowly")
}

// VALIDATES: receivedRefreshPeriod bounds what a neighbor can ask for. A PATH with no
// TIME_VALUES falls back to the RFC 2205 suggested default rather than to the local
// period, and an advertised period past the ceiling is clamped so a peer cannot keep
// ze's state alive for years.
func TestReceivedRefreshPeriodBounds(t *testing.T) {
	cases := []struct {
		name string
		msg  ParsedMessage
		want time.Duration
	}{
		{"absent", ParsedMessage{}, DefaultRefreshPeriod},
		{"zero", ParsedMessage{HasTimeValues: true}, DefaultRefreshPeriod},
		{"advertised", ParsedMessage{HasTimeValues: true, TimeValues: timeValues{RefreshPeriod: 45000}}, 45 * time.Second},
		{"above the ceiling", ParsedMessage{HasTimeValues: true, TimeValues: timeValues{RefreshPeriod: ^uint32(0)}}, maxRefreshPeriod},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, receivedRefreshPeriod(&tc.msg))
		})
	}
}
