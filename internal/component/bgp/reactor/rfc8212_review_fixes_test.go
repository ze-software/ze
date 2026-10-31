// Design: docs/architecture/behavior/peer-lifecycle.md -- in-place import invalidation
// RFC naming: untagged -- tests machinery RFC 8212 default reject relies on; rows RFC8212-3-1/3-2 are {gap}, which refuse a proof tag until the spec closes them (AC-11).
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection

package reactor

import (
	"context"
	"net"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRFC8212RejectionNotUndoneKeepsDenial proves a rejection receipt for an
// invalidation the journal never undid follows the published import policy.
// Goal: while the published policy still rejects, the receipt keeps the
// denial (the owner finalizes, its path stays ineligible) and reports the
// peer, the transaction and the consequence; once an accepting policy is
// published, the receipt aborts and the path is restored. Method: the swap
// applies and the journal is never rolled back, so the receipt meets an
// applied invalidation; in the second case the accepting policy is put back
// outside the journal first.
// PREVENTS: a late rejection restamping and resending the source's routes
// under a policy that rejects them, against RFC 8212 Section 3.
func TestRFC8212RejectionNotUndoneKeepsDenial(t *testing.T) {
	copyImport := func(dst, src *PeerSettings) { dst.ImportPolicy = src.ImportPolicy }
	t.Run("published-policy-rejects", func(t *testing.T) {
		fx := newRFC8212ImportFixture(t, false)
		j := &rfc8212TxJournal{Journal: sdk.NewJournal(), txID: "tx-late-reject", deadline: time.Now().Add(30 * time.Second)}
		require.NoError(t, fx.swap(j))
		err := fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, false)
		require.ErrorContains(t, err, "peer 192.0.2.1: transaction tx-late-reject was rejected but its rollback did not restore")
		require.ErrorContains(t, err, "they stay withdrawn")
		j.Discard()
		ops, _, eligible, _ := fx.owner.seen()
		require.Equal(t, []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyApply, rpc.SourcePolicyFinalize}, ops)
		require.False(t, eligible, "eligibility restored under a rejecting import policy")
		fx.settled(t, false)
	})
	t.Run("published-policy-accepts", func(t *testing.T) {
		fx := newRFC8212ImportFixture(t, false)
		previous := fx.peer.settingsSnapshot()
		j := &rfc8212TxJournal{Journal: sdk.NewJournal(), txID: "tx-late-accept", deadline: time.Now().Add(30 * time.Second)}
		require.NoError(t, fx.swap(j))
		require.NoError(t, fx.peer.restoreHotSwappableSettings(previous, copyImport))
		err := fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, false)
		require.ErrorContains(t, err, "rejected before its import invalidation was undone")
		j.Discard()
		ops, _, eligible, _ := fx.owner.seen()
		require.Equal(t, []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyApply, rpc.SourcePolicyAbort}, ops)
		require.True(t, eligible, "an accepting policy is published, so the abort restores the path")
		fx.settled(t, true)
	})
}

// rfc8212RetainingRIB models bgp-rib's purge-stale for one peer: it holds a
// path its previous session left stale under Graceful Restart, and purges it
// on the command. Safe for concurrent use.
type rfc8212RetainingRIB struct {
	mu       sync.Mutex
	retained map[string]bool
	accepts  []bool
	peer     *Peer
}

func (rib *rfc8212RetainingRIB) purge(args []string) {
	rib.mu.Lock()
	defer rib.mu.Unlock()
	s := rib.peer.settingsSnapshot()
	rib.accepts = append(rib.accepts, s.ImportPolicy.AcceptsAtEnd(s.IsEBGP()))
	for address := range rib.retained {
		if slices.Contains(args, address) {
			delete(rib.retained, address)
		}
	}
}

// TestRFC8212ImportSwapPurgesRetainedPaths proves a swap that removes the last
// accepting import policy of a peer that is not Established purges the paths
// Graceful Restart retained from its previous session. Goal: the stale path
// bgp-rib keeps eligible for the restarting peer is purged, once, after the
// rejecting policy is published. Method: a plugin answering the RIB's
// purge-stale command holds one retained path for the peer; the peer is not
// Established; the swap runs through the journaled path.
// PREVENTS: the previous session's paths staying eligible and advertised under
// the rejecting policy until the restart timer or End-of-RIB sweeps them.
func TestRFC8212ImportSwapPurgesRetainedPaths(t *testing.T) {
	const name = "rfc8212-retaining-rib"
	r := New(&Config{ListenAddr: "127.0.0.1:0", LocalAS: 65001})
	accept, err := filterapi.NewPolicy(nil, routepolicy.FinalAccept)
	require.NoError(t, err)
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.ImportPolicy = accept
	peer := NewPeer(settings)
	peer.reactor = r
	r.peers[settings.PeerKey()] = peer
	rib := &rfc8212RetainingRIB{retained: map[string]bool{"192.0.2.1": true}, peer: peer}

	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	require.NoError(t, registry.Register(registry.Registration{
		Name: name, Description: "RFC 8212 retained-path RIB fixture",
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			p.OnExecuteCommand(func(_, _ string, args []string, _ string) (string, any, error) {
				rib.purge(args)
				return rpc.StatusDone, map[string]any{"purged": 1}, nil
			})
			if err := p.Run(context.Background(), &sdk.Registration{
				Commands: []sdk.CommandDecl{{Name: retainedPurgeCommand}},
			}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))
	srv := policyFinalPluginServer(t, r, name)
	process := srv.ProcessManager().GetProcess(name)
	require.NotNil(t, process)
	require.NotNil(t, process.Bridge())
	lowEventually(t, process.Bridge().HasExecuteCommand, "purge-stale callback published")
	require.NotEqual(t, PeerStateEstablished, peer.State())

	next := peer.settingsSnapshot()
	next.ImportPolicy = filterapi.Policy{}
	copyImport := func(dst, src *PeerSettings) { dst.ImportPolicy = src.ImportPolicy }
	j := &rfc8212TxJournal{Journal: sdk.NewJournal(), txID: "tx-gr", deadline: time.Now().Add(30 * time.Second)}
	require.NoError(t, (&reactorAPIAdapter{r: r}).swapPeerSettingsJournaled([]peerSettingsSwap{{
		key: settings.PeerKey(), next: next, apply: copyImport}}, j))
	j.Discard()

	rib.mu.Lock()
	defer rib.mu.Unlock()
	require.Empty(t, rib.retained, "the retained path stays eligible under the rejecting import policy")
	require.Equal(t, []bool{false}, rib.accepts, "purge ran before the rejecting policy was published, or more than once")
}
