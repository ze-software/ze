package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// routerIDTree builds a BGP tree with a global router-id, one peer that
// inherits it and one peer that overrides it with its own session router-id.
func routerIDTree(t *testing.T, global string) map[string]any {
	t.Helper()
	tree := makeBGPTree(map[string]testPeer{
		"inherits":  {remoteIP: "10.0.0.1", remoteAS: "65001", localAS: "65000"},
		"overrides": {remoteIP: "10.0.0.2", remoteAS: "65002", localAS: "65000"},
	})
	peers, ok := tree["peer"].(map[string]any)
	require.True(t, ok, "makeBGPTree must build a peer map")
	overrides, ok := peers["overrides"].(map[string]any)
	require.True(t, ok, "makeBGPTree must build the overrides peer")
	session, ok := overrides["session"].(map[string]any)
	require.True(t, ok, "makeBGPTree must build a session container")
	session["router-id"] = "9.9.9.9"
	tree["router-id"] = global
	return tree
}

// peerByAddr returns the running peer at addr, or fails the test.
func peerByAddr(t *testing.T, r *Reactor, addr string) *Peer {
	t.Helper()
	for _, p := range r.Peers() {
		if p.Settings().Address.String() == addr {
			return p
		}
	}
	t.Fatalf("no peer %s", addr)
	return nil
}

// TestReloadAppliesGlobalRouterID verifies that a reload changing the global
// router-id applies it to the reactor and to every peer that inherits it.
//
// VALIDATES: after ApplyConfigDiff with router-id 1.2.3.4 -> 2.2.2.2, Stats
// (what `show bgp` reports) answers 2.2.2.2, the inheriting peer is restarted
// with Identifier 2.2.2.2 (a new OPEN carries it, RFC 4271 Section 4.2), and
// the peer that overrides the router-id keeps its session and 9.9.9.9.
// PREVENTS: the daemon reporting the startup Identifier after its config
// changed it, the defect in plan/journal/reload-rolls-back-instead-of-applying.md.
func TestReloadAppliesGlobalRouterID(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true, RouterID: 0x01020304, LocalAS: 65000})
	require.NoError(t, r.Start())
	defer r.Stop()

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.ApplyConfigDiff(configRoot(routerIDTree(t, "1.2.3.4"))))
	inheritsBefore := peerByAddr(t, r, "10.0.0.1")
	overridesBefore := peerByAddr(t, r, "10.0.0.2")

	require.NoError(t, adapter.ApplyConfigDiff(configRoot(routerIDTree(t, "2.2.2.2"))))

	assert.Equal(t, uint32(0x02020202), r.Stats().RouterID, "show bgp must report the reloaded router-id")

	inherits := peerByAddr(t, r, "10.0.0.1")
	assert.Equal(t, uint32(0x02020202), inherits.Settings().RouterID)
	assert.NotSame(t, inheritsBefore, inherits, "a peer whose Identifier changed must be re-established")

	overrides := peerByAddr(t, r, "10.0.0.2")
	assert.Equal(t, uint32(0x09090909), overrides.Settings().RouterID)
	assert.Same(t, overridesBefore, overrides, "a peer whose Identifier did not change must keep its session")
}

// TestReloadGlobalsRollback verifies that a rolled back reconcile restores the
// global router-id and local AS.
//
// VALIDATES: Rollback of the reconcile journal puts back 1.2.3.4 and AS 65000.
// PREVENTS: a refused reload leaving the reactor on globals it never applied.
func TestReloadGlobalsRollback(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true, RouterID: 0x01020304, LocalAS: 65000})
	adapter := &reactorAPIAdapter{r: r}

	j := &internalJournal{}
	require.NoError(t, adapter.reconcilePeersJournaled(nil, Globals{RouterID: 0x02020202, LocalAS: 65100}, "test", j))
	assert.Equal(t, Globals{RouterID: 0x02020202, LocalAS: 65100}, r.globals())

	assert.Empty(t, j.Rollback())
	assert.Equal(t, Globals{RouterID: 0x01020304, LocalAS: 65000}, r.globals())
}

// TestReloadMovesInfoGauge verifies that a reload which changes the global
// router-id and local AS moves the ze_info series with them, and that a rolled
// back reload moves it back.
//
// VALIDATES: after the reconcile, ze_info{router_id="2.2.2.2",local_as="65100"}
// is 1 and the startup series is gone; after Rollback the startup series is
// back and the reloaded one is gone.
// PREVENTS: a scrape reporting the startup identity after a reload applied a
// new one, or two identities for one instance.
func TestReloadMovesInfoGauge(t *testing.T) {
	reg := newSpyRegistry()
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true, RouterID: 0x01020304, LocalAS: 65000})
	r.rmetrics = initReactorMetrics(reg, "test", "1.2.3.4", "65000")
	adapter := &reactorAPIAdapter{r: r}
	info := reg.gaugeVec("ze_info")
	require.NotNil(t, info, "ze_info must be registered")

	j := &internalJournal{}
	require.NoError(t, adapter.reconcilePeersJournaled(nil, Globals{RouterID: 0x02020202, LocalAS: 65100}, "test", j))

	reloaded := info.get("test", "2.2.2.2", "65100")
	require.NotNil(t, reloaded, "ze_info must carry the reloaded router-id and local AS")
	assert.InDelta(t, 1.0, reloaded.Value(), 0)
	assert.Nil(t, info.get("test", "1.2.3.4", "65000"), "the startup series must be deleted")

	assert.Empty(t, j.Rollback())
	restored := info.get("test", "1.2.3.4", "65000")
	require.NotNil(t, restored, "a rolled back reload must restore the startup series")
	assert.InDelta(t, 1.0, restored.Value(), 0)
	assert.Nil(t, info.get("test", "2.2.2.2", "65100"), "the rolled back series must be deleted")
}
