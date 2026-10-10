package reactor

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configtx "github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/internal/core/bgp/configop"
	"github.com/ze-software/ze/pkg/plugin/rpc"

	// The BGP operation decomposer registers itself here, and this test drives
	// the one a reload runs rather than a copy of what it emits.
	_ "github.com/ze-software/ze/internal/component/bgp/plugin"
)

// TestCandidateDeclaringACreatedPeersAddressUnderAnotherName holds the edge of
// AC-9 of spec-yang-rpc-declarations-with-no-handler that the reload reaches
// when the operator declares a created peer in their own words.
//
// GOAL: a candidate that declares the address of a peer `create bgp peer`
// built, under a name of the operator's choosing, takes the peer over: after
// the reload exactly one peer runs at that address, under the operator's name,
// the configuration owns it, and a later candidate that drops it removes it.
// This is the path whose failure before the spec was "remove-peer peer
// peer-X is not running", which refused the commit.
// METHOD: create a peer through the API adapter, then run every reactor step a
// reload of such a candidate takes, in the order the reload takes them:
// ReloadRunning, the registered BGP operation decomposer over that running
// tree and the candidate, each operation through applyConfigOperation with the
// destroy before the create (the transaction solver's verb order),
// ApplyConfigDiff and SetConfigTree. Then do the same with a candidate that no
// longer declares it.
//
// VALIDATES: the decomposer plans the removal of the created peer's entry and
// the creation of the operator's; afterwards one peer runs at the address,
// named after the operator's entry; no createdPeers mark is left; the running
// configuration holds the operator's entry and not the created one; the next
// candidate without it removes the peer and its entry.
// PREVENTS: a takeover that leaves two entries for one address, a created mark
// that makes the peer unremovable by configuration, or a reload refused because
// the created entry is not where the decomposer looks for it.
func TestCandidateDeclaringACreatedPeersAddressUnderAnotherName(t *testing.T) {
	r, api, file := newTakeoverReactor(t)

	addr := netip.MustParseAddr("192.0.2.7")
	require.NoError(t, api.AddDynamicPeer(addr, map[string]any{
		"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
	}))

	edge := map[string]any{
		"connection": map[string]any{
			"remote": map[string]any{"ip": "192.0.2.7"},
			"local":  map[string]any{"ip": "auto"},
		},
		"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
	}
	declaringBGP := takeoverGlobals()
	declaringBGP["peer"] = map[string]any{"edge": edge}
	declaring := map[string]any{"bgp": declaringBGP}

	*file = declaring
	ops := reloadThroughOperations(t, api, declaring)
	require.Len(t, ops, 2, "the takeover is one removal and one creation: %v", ops)
	assert.Equal(t, configop.RemovePeer, ops[0].Type)
	assert.Equal(t, "peer-192.0.2.7", ops[0].Params.Peer, "the created entry is the one removed")
	assert.Equal(t, configop.AddPeer, ops[1].Type)
	assert.Equal(t, "edge", ops[1].Params.Peer, "the operator's entry is the one created")

	r.mu.RLock()
	running := r.peersAt(addr)
	_, marked := r.createdPeers[addr]
	r.mu.RUnlock()
	require.Len(t, running, 1, "exactly one peer runs at the address")
	assert.Equal(t, "edge", running[0], "the peer carries the operator's name")
	assert.False(t, marked, "the configuration owns the peer, so it is no longer marked created")
	peers := runningPeerList(t, api)
	assert.Contains(t, peers, "edge", "the running configuration holds the operator's entry")
	assert.NotContains(t, peers, "peer-192.0.2.7", "and not the created one")

	*file = map[string]any{"bgp": takeoverGlobals()}
	ops = reloadThroughOperations(t, api, *file)
	require.Len(t, ops, 1, "dropping the declaration is one removal: %v", ops)
	assert.Equal(t, configop.RemovePeer, ops[0].Type)
	assert.Equal(t, "edge", ops[0].Params.Peer)

	r.mu.RLock()
	running = r.peersAt(addr)
	r.mu.RUnlock()
	assert.Empty(t, running, "a candidate that drops the taken-over peer removes it")
	bgp, _ := api.GetConfigTree()["bgp"].(map[string]any)
	assert.NotContains(t, bgp, "peer", "and its entry leaves the running configuration")
}

// newTakeoverReactor answers a reactor running a file that declares no peer,
// whose reloads read that file through the config loader the way the daemon's
// do, and the file itself for the test to replace before each reload.
func newTakeoverReactor(t *testing.T) (*Reactor, *reactorAPIAdapter, *map[string]any) {
	t.Helper()
	r := newDynamicPeerReactor(t, 65001, 0x0A000001)
	// The file the daemon runs: the globals, and no peer yet. A reload reads
	// the file through the config loader, which names each peer after its
	// list key (PeersFromTree), and the candidate is what the file holds.
	file := map[string]any{"bgp": takeoverGlobals()}
	r.configTree = file
	r.config.ConfigPath = "ze.conf"
	r.SetReloadFunc(func(string) ([]*PeerSettings, Globals, error) {
		bgp, _ := file["bgp"].(map[string]any)
		peers, err := PeersFromTree(bgp)
		if err != nil {
			return nil, Globals{}, err
		}
		globals, err := GlobalsFromTree(bgp)
		return peers, globals, err
	})
	api := &reactorAPIAdapter{r: r}
	return r, api, &file
}

// takeoverGlobals answers the bgp block of the file the test daemon runs,
// without its peers: the local AS and router ID newDynamicPeerReactor gives the
// reactor.
func takeoverGlobals() map[string]any {
	return map[string]any{
		"session":   map[string]any{"asn": map[string]any{"local": "65001"}},
		"router-id": "10.0.0.1",
	}
}

// reloadThroughOperations runs the reactor half of a reload of candidate, the
// way reloadConfig (../../plugin/server/reload.go) and the transaction
// orchestrator drive it, and answers the operations the decomposer planned.
func reloadThroughOperations(t *testing.T, api *reactorAPIAdapter, candidate map[string]any) []configtx.ConfigOperation {
	t.Helper()
	decompose, registered := configtx.OperationDecomposerFor("bgp")
	require.True(t, registered, "the bgp root has an operation decomposer")

	base := api.ReloadRunning(candidate)
	active, err := json.Marshal(map[string]any{"bgp": base["bgp"]})
	require.NoError(t, err)
	next, err := json.Marshal(map[string]any{"bgp": candidate["bgp"]})
	require.NoError(t, err)

	ops, err := decompose(context.Background(), configtx.DecomposeRequest{
		TransactionID: "tx-takeover",
		Root:          "bgp",
		ActiveRoot:    string(active),
		CandidateRoot: string(next),
		// Only whether the diff reaches the peer list is read here; the
		// operations come from the two roots.
		Diff: configtx.DiffSection{Root: "bgp", Changed: `{"bgp/peer":{}}`},
	})
	require.NoError(t, err)

	// The solver runs every destroy before any create (operationPhase,
	// ../../config/transaction/solver.go).
	for _, verb := range []rpc.OperationVerb{configtx.VerbDestroy, configtx.VerbCreate} {
		for i := range ops {
			if ops[i].Verb != verb {
				continue
			}
			_, err := api.applyConfigOperation(&ops[i], &testJournal{})
			require.NoError(t, err, "operation %s %s", ops[i].Type, ops[i].Params.Peer)
		}
	}
	require.NoError(t, api.ApplyConfigDiff(candidate))
	api.SetConfigTree(candidate)
	return ops
}

// peersAt answers the names of the peers running at addr. The caller MUST hold
// r.mu.
func (r *Reactor) peersAt(addr netip.Addr) []string {
	var names []string
	for key, peer := range r.peers {
		if key.Addr() == addr {
			names = append(names, peer.Settings().Name)
		}
	}
	return names
}

// TestCompensatedTakeoverMarksTheCreatedPeerAgain holds AC-9 across a reload
// the hub rejects after the reactor applied it (reload_compensation.go,
// ../../plugin/server).
//
// GOAL: a commit that declares a created peer, then is rejected and undone,
// leaves the peer as it was before the commit: created, unsaved, and kept by
// the next reload whose candidate does not declare it. SetConfigTree drops the
// mark of a peer the new tree declares, and setting the old tree back does not
// put the mark back, because the old tree holds the created entry too.
// METHOD: create a peer and capture CreatedPeers, as the compensation does
// when it records the tree it would restore. Reload a candidate that declares
// the peer by name (`update bgp config`, then a commit), then undo it the way
// restoreReload does with the file back as it was: RestoreCreatedPeers,
// ApplyConfigDiff and SetConfigTree of the recorded tree, RestoreCreatedPeers
// again, each with the recorded tree. Then reload the file, which does not
// declare it.
//
// VALIDATES: the takeover drops the mark; the restore puts it back; the next
// reload plans nothing for the peer and leaves it running and in the running
// configuration; an address no peer runs at, which the restored tree holds no
// entry for, is neither rebuilt nor marked by a restore.
// PREVENTS: a created peer silently turned configured by a commit that never
// took effect, which the next unrelated commit then deletes.
func TestCompensatedTakeoverMarksTheCreatedPeerAgain(t *testing.T) {
	r, api, file := newTakeoverReactor(t)

	addr := netip.MustParseAddr("192.0.2.7")
	require.NoError(t, api.AddDynamicPeer(addr, map[string]any{
		"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
	}))
	created := api.CreatedPeers()
	require.Equal(t, map[netip.Addr]string{addr: "peer-192.0.2.7"}, created)

	declaringBGP := takeoverGlobals()
	declaringBGP["peer"] = map[string]any{"peer-192.0.2.7": runningPeerList(t, api)["peer-192.0.2.7"]}
	declaringBGP["router-id"] = "10.0.0.2"
	declaring := map[string]any{"bgp": declaringBGP}
	before := api.ReloadRunning(declaring)
	*file = declaring
	reloadThroughOperations(t, api, declaring)
	require.Empty(t, api.CreatedPeers(), "the commit declaring the peer takes it over")

	*file = map[string]any{"bgp": takeoverGlobals()}
	require.NoError(t, api.RestoreCreatedPeers(before, created))
	require.NoError(t, api.ApplyConfigDiff(before))
	_, held := r.findPeerByAddr(addr)
	require.True(t, held, "the reconcile against the restored file keeps the peer marked again")
	api.SetConfigTree(before)
	require.NoError(t, api.RestoreCreatedPeers(before, created))
	assert.Equal(t, created, api.CreatedPeers(), "undoing the commit marks the peer created again")

	ops := reloadThroughOperations(t, api, *file)
	assert.Empty(t, ops, "the next reload plans nothing for the created peer")
	_, held = r.findPeerByAddr(addr)
	assert.True(t, held, "and leaves it running")
	assert.Contains(t, runningPeerList(t, api), "peer-192.0.2.7", "and in the running configuration")

	require.NoError(t, api.RestoreCreatedPeers(before, map[netip.Addr]string{netip.MustParseAddr("198.51.100.1"): "peer-198.51.100.1"}))
	assert.NotContains(t, api.CreatedPeers(), netip.MustParseAddr("198.51.100.1"),
		"a restore marks no address no peer runs at")
}

// TestCompensatedRenameTakeoverRebuildsTheCreatedPeer holds AC-9 across a
// rejected reload whose candidate took a created peer's address over under
// another name (reload_compensation.go, ../../plugin/server).
//
// GOAL: a commit that declares a created peer's address under the operator's
// name, then is rejected and undone, leaves the peer as it was before the
// commit: running under its created name, marked created, and kept by the next
// reload whose candidate does not declare it.
// METHOD: create a peer and capture CreatedPeers and the running tree, as the
// compensation does. Reload a candidate declaring the address as `edge`, then
// undo it the way restoreReload does with the file back as it was:
// RestoreCreatedPeers, ApplyConfigDiff and SetConfigTree of the recorded tree,
// RestoreCreatedPeers again. Then reload the file.
//
// VALIDATES: the takeover renames the peer and drops the mark; the first
// restore neither marks nor replaces `edge`; the reconcile removes `edge`; the
// second restore rebuilds `peer-<addr>` from the restored tree and marks it;
// the next reload plans nothing and leaves it running and in the running
// configuration.
// PREVENTS: a rejected rename takeover that loses the created peer, or leaves
// the operator's peer of a commit that never took effect running.
func TestCompensatedRenameTakeoverRebuildsTheCreatedPeer(t *testing.T) {
	r, api, file := newTakeoverReactor(t)

	addr := netip.MustParseAddr("192.0.2.7")
	require.NoError(t, api.AddDynamicPeer(addr, map[string]any{
		"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
	}))
	created := api.CreatedPeers()
	require.Equal(t, map[netip.Addr]string{addr: "peer-192.0.2.7"}, created)

	edge := map[string]any{
		"connection": map[string]any{
			"remote": map[string]any{"ip": "192.0.2.7"},
			"local":  map[string]any{"ip": "auto"},
		},
		"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
	}
	declaringBGP := takeoverGlobals()
	declaringBGP["peer"] = map[string]any{"edge": edge}
	declaring := map[string]any{"bgp": declaringBGP}
	before := api.ReloadRunning(declaring)
	*file = declaring
	reloadThroughOperations(t, api, declaring)
	require.Empty(t, api.CreatedPeers(), "the commit declaring the address takes the peer over")
	r.mu.RLock()
	running := r.peersAt(addr)
	r.mu.RUnlock()
	require.Equal(t, []string{"edge"}, running, "under the operator's name")

	*file = map[string]any{"bgp": takeoverGlobals()}
	require.NoError(t, api.RestoreCreatedPeers(before, created))
	require.Empty(t, api.CreatedPeers(), "the peer at the address is not the one the mark named")
	require.NoError(t, api.ApplyConfigDiff(before))
	api.SetConfigTree(before)
	require.NoError(t, api.RestoreCreatedPeers(before, created))

	r.mu.RLock()
	running = r.peersAt(addr)
	r.mu.RUnlock()
	assert.Equal(t, []string{"peer-192.0.2.7"}, running, "undoing the commit rebuilds the created peer")
	assert.Equal(t, created, api.CreatedPeers(), "and marks it created again")
	peers := runningPeerList(t, api)
	assert.Contains(t, peers, "peer-192.0.2.7", "the running configuration holds the created entry")
	assert.NotContains(t, peers, "edge", "and not the operator's")

	ops := reloadThroughOperations(t, api, *file)
	assert.Empty(t, ops, "the next reload plans nothing for the created peer")
	r.mu.RLock()
	running = r.peersAt(addr)
	r.mu.RUnlock()
	assert.Equal(t, []string{"peer-192.0.2.7"}, running, "and leaves it running")
}
