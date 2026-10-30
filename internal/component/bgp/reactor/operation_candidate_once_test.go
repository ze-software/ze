// Design: docs/architecture/config/apply-ordering.md -- BGP peer operation handlers

package reactor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/configop"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestOperationCandidateResolvedOncePerTransaction proves a reload transaction
// resolves its candidate config once, however many modify-peer operations it
// carries. Goal: a bgp-level import change on N peers costs one full config
// load across verify and apply, not 2N; every operation still gets the settings
// a per-peer resolution gives, each in its own copy; the shared candidate goes
// when the transaction completes. Method: a counting reload function as the
// seam; N modify-peers verified then applied under one transaction ID.
func TestOperationCandidateResolvedOncePerTransaction(t *testing.T) {
	const peers = 3
	r := New(&Config{ConfigPath: "ze.conf"})
	var candidates []*PeerSettings
	var ops []*rpc.ConfigOperation
	for i := range peers {
		address := mustParseAddr("203.0.113.1")
		for range i {
			address = address.Next()
		}
		settings := NewPeerSettings(address, 65000, uint32(65001+i), 0)
		settings.Name = "edge" + string(rune('0'+i))
		settings.ImportPolicy = mustPolicy(t, []routepolicy.Ref{{Name: "bgp-filter-prefix:CUSTOMERS"}}, routepolicy.FinalAccept)
		require.NoError(t, r.AddPeer(settings))
		candidate := *settings
		candidate.ImportPolicy = mustPolicy(t, []routepolicy.Ref{{Name: "bgp-filter-community:UPSTREAM"}}, routepolicy.FinalAccept)
		candidates = append(candidates, &candidate)
		ops = append(ops, &rpc.ConfigOperation{ID: "bgp-modify-peer-" + settings.Name, Root: "bgp", Owner: "bgp",
			Type: configop.ModifyPeer, Target: rpc.ResourceRef{Kind: rpc.ResourcePeer, Peer: settings.Name},
			Params: rpc.ConfigOperationParams{Peer: settings.Name}})
	}
	var loads atomic.Int32
	r.SetReloadFunc(func(string) ([]*PeerSettings, Globals, []*DynamicGroupConfig, error) {
		loads.Add(1)
		return candidates, Globals{}, nil, nil
	})

	// Identical to per-peer resolution, each handout its own copy.
	adapter := &reactorAPIAdapter{r: r}
	for _, op := range ops {
		alone, _, err := adapter.resolveOperationCandidate("", op)
		require.NoError(t, err)
		first, _, err := adapter.resolveOperationCandidate("tx-compare", op)
		require.NoError(t, err)
		second, _, err := adapter.resolveOperationCandidate("tx-compare", op)
		require.NoError(t, err)
		require.Equal(t, *alone, *first, "shared resolution differs from per-peer resolution")
		require.NotSame(t, first, second, "two operations share one settings value")
		for _, candidate := range candidates {
			require.NotSame(t, candidate, first, "the shared candidate itself was handed out")
		}
	}
	require.Equal(t, int32(peers+1), loads.Load(), "per-peer resolution loads each time, the transaction once")
	require.NoError(t, r.CompleteSourcePolicyTx(context.Background(), "tx-compare", true))

	loads.Store(0)
	for _, op := range ops {
		require.NoError(t, r.VerifyConfigOperation("tx1", op))
	}
	j := &rfc8212TxJournal{Journal: sdk.NewJournal(), txID: "tx1", deadline: time.Now().Add(time.Minute)}
	for _, op := range ops {
		_, err := r.ApplyConfigOperation(op, j)
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), loads.Load(), "N modify-peers in one transaction must load the candidate once")
	for _, peer := range r.Peers() {
		require.Equal(t, []routepolicy.Ref{{Name: "bgp-filter-community:UPSTREAM"}},
			peer.settingsSnapshot().ImportPolicy.Filters(), "peer %s did not take its candidate", peer.Settings().Name)
	}

	// The candidate goes with the transaction: its completion drops it, and
	// another transaction resolves its own.
	require.NoError(t, r.CompleteSourcePolicyTx(context.Background(), "tx1", true))
	require.NoError(t, r.VerifyConfigOperation("tx1", ops[0]))
	require.Equal(t, int32(2), loads.Load(), "a completed transaction's candidate was kept")
	require.NoError(t, r.VerifyConfigOperation("tx2", ops[0]))
	require.Equal(t, int32(3), loads.Load(), "another transaction reused a foreign candidate")
	j.Discard()
}
