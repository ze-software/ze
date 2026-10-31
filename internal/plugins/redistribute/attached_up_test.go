package redistribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestOrchestratorSkipsReplayOnAttachedUp proves that an up carrying reason
// attached records the peer up and fires no replay, on both event paths.
//
// VALIDATES: onStateUp with rpc.ReasonAttached fires no ReplayRequest even
// with an import feeding bgp; the peer is recorded up, so a second up is not
// an edge, and a down then up fires again. The text path parses "reason
// attached".
// PREVENTS: every redistribute producer re-emitting its route set into a
// session a reload only attached this plugin to.
//
// Method: TestOrchestratorFiresReplayOnPeerUp's evaluator, where a normal up
// fires one replay.
func TestOrchestratorSkipsReplayOnAttachedUp(t *testing.T) {
	const peer = "10.0.0.1"
	for _, event := range []string{"structured", "text"} {
		t.Run(event, func(t *testing.T) {
			resetState(t)
			require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "fakeredist", Protocol: "fakeredist"}))
			configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
				{Source: "fakeredist", Destination: "bgp", Final: "accept", Chain: familyChain{family.IPv4Unicast}},
			}))
			bus := newTestBus()
			var gotIDs []uint64
			redistevents.ReplayRequestEvent.Subscribe(bus, func(r *redistevents.ReplayRequest) {
				gotIDs = append(gotIDs, r.ReplayID)
			})
			coord := newReplayCoordinator()

			reason := rpc.ReasonAttached
			if event == "text" {
				addr, state, parsed := parseStateEvent("peer " + peer + " remote as 65001 state up reason " + rpc.ReasonAttached + "\n")
				require.Equal(t, peer, addr)
				require.Equal(t, "up", state)
				reason = parsed
			}

			fired := coord.onStateUp(bus, peer, reason)
			assert.False(t, fired, "an attached up fires no replay")
			assert.Empty(t, gotIDs, "an attached up emits no ReplayRequest")

			again := coord.onStateUp(bus, peer, "")
			assert.False(t, again, "the attached up recorded the peer up, so this up is no edge")

			coord.onPeerDown(peer)
			edge := coord.onStateUp(bus, peer, "")
			assert.True(t, edge, "a down then a normal up is an edge again")
			assert.Len(t, gotIDs, 1)
		})
	}
}
