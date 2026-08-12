// Related: peer_settings_apply.go -- applyHotSwappableSettings, the reload-side writer
// Related: filter_ordered.go -- runIngressPolicyChain, the session-side reader
package reactor

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// TestReloadWhileReceivingNoRace runs config reloads that swap a peer's filter
// chains WHILE the ingress datapath evaluates received UPDATEs against that same
// chain, and requires the race detector to stay silent.
//
// VALIDATES: AC-7 / R-1. The swap this spec added writes PeerSettings from the
// RELOAD goroutine, and PeerSettings is read per received UPDATE from the peer's
// session read goroutine. Those two goroutines meet on one struct.
//
// PREVENTS: the fix for the silent discard being paid for with a data race. Before
// the swap existed, p.settings was written once in the Peer literal and the
// unlocked read on the hot path was safe by construction (peer.go, Settings). The
// swap falsified that, so the delivery has to be a guarded write and every hot-path
// read has to go through the p.mu accessor.
//
// DISCRIMINATION: the reader is runIngressPolicyChain (filter_ordered.go), the real
// producer of the ingress read, not the accessor called directly -- the accessor
// alone would still pass if the datapath stopped using it. It is evidence ONLY
// under -race: `make ze-race-reactor`. Verified by mutation on 2026-08-12 --
// replacing the accessor call at filter_ordered.go with a direct peer.settings
// read makes the detector report a read there against the write in
// hotSwappableSettings from applyHotSwappableSettings.
func TestReloadWhileReceivingNoRace(t *testing.T) {
	chainA := []filterapi.FilterRef{{Name: "policy:chain-a"}}
	chainB := []filterapi.FilterRef{{Name: "policy:chain-b"}}

	initial := swapTestPeerSettings()
	initial.ImportFilters = chainA
	initial.ExportFilters = chainA

	// Every reload alternates the chains, so each one is a real swap rather than a
	// no-change reload the reconcile would skip.
	var reloads atomic.Uint64
	nextSettings := func() *PeerSettings {
		ps := swapTestPeerSettings()
		if reloads.Add(1)%2 == 0 {
			ps.ImportFilters, ps.ExportFilters = chainA, chainA
		} else {
			ps.ImportFilters, ps.ExportFilters = chainB, chainB
		}
		return ps
	}

	r, peer := newSwapTestReactor(t, initial, initial)
	// Replace the helper's fixed one-shot reload with the alternating one.
	r.SetReloadFunc(func(string) ([]*PeerSettings, error) {
		return []*PeerSettings{nextSettings()}, nil
	})
	peer.state.Store(int32(PeerStateEstablished))
	peer.refreshForwardFacts()

	adapter := &reactorAPIAdapter{r: r}
	const rounds = 40

	var wg sync.WaitGroup
	wg.Add(2)

	// The reload failure is carried out of the goroutine and checked after Wait.
	// The fail-now helpers are only valid on the test's own goroutine, so the same
	// check has to run there rather than inside the loop.
	var reloadErr error
	go func() {
		defer wg.Done()
		for range rounds {
			if err := adapter.Reload(); err != nil {
				reloadErr = err
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		addr := initial.Address
		for range rounds {
			// r.api is nil in this fixture, so the step reads the chain and then
			// takes the fail-closed branch without touching the wire arguments.
			// The read at filter_ordered.go is what this test is here for.
			r.runIngressPolicyChain(peer, addr, initial.PeerAS, nil, nil)
			// The egress side reads its chain from the facts snapshot the swap
			// rebuilds, which is the other cross-goroutine handoff.
			_ = peer.forwardFacts()
			_ = peer.ExportFilters()
		}
	}()

	wg.Wait()
	require.NoError(t, reloadErr, "every reload in the loop must succeed")

	// The peer survived every swap: a restart would have replaced the object and
	// left this test racing a peer nothing reads.
	r.mu.RLock()
	after := r.peers[initial.PeerKey()]
	r.mu.RUnlock()
	require.True(t, after == peer, "every reload must swap in place, not restart the peer")
}
