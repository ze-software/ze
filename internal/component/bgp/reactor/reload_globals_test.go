package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// routerIDTree builds a BGP tree with a global router-id, one peer that
// inherits it and one peer that overrides it with its own session router-id.
func routerIDTree(global string) map[string]any {
	tree := makeBGPTree(map[string]testPeer{
		"inherits":  {remoteIP: "10.0.0.1", remoteAS: "65001", localAS: "65000"},
		"overrides": {remoteIP: "10.0.0.2", remoteAS: "65002", localAS: "65000"},
	})
	overrides := tree["peer"].(map[string]any)["overrides"].(map[string]any)
	overrides["session"].(map[string]any)["router-id"] = "9.9.9.9"
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
	require.NoError(t, adapter.ApplyConfigDiff(routerIDTree("1.2.3.4")))
	inheritsBefore := peerByAddr(t, r, "10.0.0.1")
	overridesBefore := peerByAddr(t, r, "10.0.0.2")

	require.NoError(t, adapter.ApplyConfigDiff(routerIDTree("2.2.2.2")))

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
