// Design: docs/architecture/api/architecture.md -- the derived half of a delivery edge
// Related: filter_state_binding.go -- grantFilterOwnersState

package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// inserterProcess is the plugin registerInserter registers, whose derived
// binding every case below inspects.
const inserterProcess = "fake-inserter"

// stateBinding answers the binding the peer holds for inserterProcess, and
// whether it grants the peer's state events, marked implicit, and no send type.
func stateBinding(ps *reactor.PeerSettings) (found, stateOnly bool) {
	b := processBinding(ps)
	if b == nil {
		return false, false
	}
	_, state := b.Receive[bgpevents.EventState]
	implicit := b.Implicit[bgpevents.EventState] && len(b.Implicit) == 1
	return true, state && implicit && len(b.Receive) == 1 && !b.ReceiveAll && !b.SendAll && len(b.Send) == 0
}

// processBinding answers the peer's binding for inserterProcess, or nil.
func processBinding(ps *reactor.PeerSettings) *reactor.ProcessBinding {
	for i := range ps.ProcessBindings {
		if ps.ProcessBindings[i].PluginName == inserterProcess {
			return &ps.ProcessBindings[i]
		}
	}
	return nil
}

// TestFilterOwnerReceivesPeerState checks spec-filter-wire-6-stage-filters
// phase 2's implicit `state` delivery: the plugin whose member a peer's
// chain carries, inserted or named by the operator, is fed that peer's
// state events with no `attach process` block, and nothing else, so the
// peer's removal reaches the per-peer state it keeps on either transport.
// Method: an insertion places fake-inserter:late on A's import list; then,
// with no insertion, the operator names fake-inserter:early on A's export
// list; last, the operator attaches fake-inserter to A for updates only, and
// state joins that block, marked implicit. B carries no member in any case.
func TestFilterOwnerReceivesPeerState(t *testing.T) {
	t.Run("an inserted member", func(t *testing.T) {
		registerInserter(t, rpc.FilterInsertion{Filter: "fake-inserter:late", Direction: rpc.FilterImport,
			Priority: 5, Config: rrClient, Values: []string{"on"}})
		peers, err := insertionPeers(t, "")
		require.NoError(t, err)
		found, stateOnly := stateBinding(peers["A"])
		require.True(t, found, "peer A carries fake-inserter's member, so fake-inserter is fed A's state")
		require.True(t, stateOnly, "the derived binding grants state and nothing else")
		found, _ = stateBinding(peers["B"])
		require.False(t, found, "peer B carries no member of fake-inserter, so it feeds it nothing")
	})
	t.Run("an operator-named member", func(t *testing.T) {
		registerInserter(t)
		peers, err := insertionPeers(t, "filter { export { filters [ fake-inserter:early ]; final accept; } }")
		require.NoError(t, err)
		found, stateOnly := stateBinding(peers["A"])
		require.True(t, found, "peer A's export list names fake-inserter:early")
		require.True(t, stateOnly, "the derived binding grants state and nothing else")
		found, _ = stateBinding(peers["B"])
		require.False(t, found, "peer B names no member of fake-inserter")
	})
	t.Run("an operator attach block without state", func(t *testing.T) {
		registerInserter(t)
		peers, err := insertionPeers(t, "filter { export { filters [ fake-inserter:early ]; final accept; } } "+
			"attach process fake-inserter { receive [ update-received ]; send [ update ]; }")
		require.NoError(t, err)
		b := processBinding(peers["A"])
		require.NotNil(t, b, "the operator's block is kept")
		require.Contains(t, b.Receive, bgpevents.EventState, "the plugin a chain names receives the peer's state regardless")
		require.True(t, b.Implicit[bgpevents.EventState], "the added state grant is marked implicit")
		require.Contains(t, b.Receive, "update", "the operator's grant is kept")
		require.False(t, b.Implicit["update"], "the operator's grant is not implicit")
		require.True(t, b.Send["update"], "the operator's send grant is kept")
		owned := 0
		for i := range peers["A"].ProcessBindings {
			if peers["A"].ProcessBindings[i].PluginName == "fake-inserter" {
				owned++
			}
		}
		require.Equal(t, 1, owned, "no second binding is made for one process")
	})
}
