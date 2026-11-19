// Design: docs/guide/redistribution.md -- original accepted contribution removal
// Related: held.go -- the original consumer episode must survive replacement

package redistribute

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// TestWithdrawalUsesOriginalAcceptingConsumer drives actual configured Add and
// Remove dispatch across registry replacement. All three contributions share a
// prefix, so source instances and the accepting episode decide what may leave.
// The unchanged-registry case proves ordinary useful dispatch still works.
// MUTATION: resolving the consumer by current registry name redirects removal.
func TestWithdrawalUsesOriginalAcceptingConsumer(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "same episode", true: "replacement episode"}[replace], func(t *testing.T) {
			resetState(t)
			protocol := redistevents.RegisterProtocol("withdrawal-episode-source")
			configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{{
				Source: "withdrawal-episode-source", Destination: "bgp", Final: "accept",
				Chain: familyChain{family.IPv4Unicast},
			}}))
			original := &withdrawalEpisodeConsumer{live: make(map[uint32]configredist.RouteEntry)}
			require.NoError(t, configredist.RegisterConsumer(original))
			stopWatching := watchConsumers(newTestBus(), newReplayCoordinator())
			t.Cleanup(stopWatching)
			publish := func(action redistevents.RouteAction, instance uint32) {
				batch := addBatch(protocol, afiIPv4, "192.0.2.0/24", "")
				batch.Entries[0].Action = action
				batch.Entries[0].SourceInstance = instance
				handleBatch(t.Context(), nil, batch)
			}
			publish(redistevents.ActionAdd, 11)
			publish(redistevents.ActionAdd, 22)
			require.Len(t, original.live, 2, "both original contributions must first be useful")
			current := original
			if replace {
				current = &withdrawalEpisodeConsumer{live: make(map[uint32]configredist.RouteEntry)}
				require.True(t, configredist.ReregisterConsumer(current))
			}
			publish(redistevents.ActionAdd, 33)
			require.Contains(t, current.live, uint32(33), "the current episode must also accept useful work")
			publish(redistevents.ActionRemove, 11)
			require.Len(t, original.removed, 1, "the original contribution was redirected to a replacement consumer")
			require.Equal(t, uint32(11), original.removed[0].SourceInstance)
			require.Equal(t, protocol, original.removed[0].Protocol)
			require.Equal(t, "192.0.2.0/24", original.removed[0].Prefix)
			require.NotContains(t, original.live, uint32(11))
			require.Contains(t, original.live, uint32(22), "same-prefix original neighbor must survive")
			require.Contains(t, current.live, uint32(33), "same-prefix current contribution must survive")
			if replace {
				require.Empty(t, current.removed, "old removal must not call the replacement episode")
			}
			publish(redistevents.ActionRemove, 33)
			require.NotContains(t, current.live, uint32(33), "the useful current contribution must remain removable")
			require.Contains(t, original.live, uint32(22))
		})
	}
}

// This serialized dispatch fixture records actual consumer invocations. Its
// local bookkeeping supplies no provider settlement or native cleanup receipt.
type withdrawalEpisodeConsumer struct {
	live    map[uint32]configredist.RouteEntry
	removed []configredist.RouteRemoval
}

func (*withdrawalEpisodeConsumer) Name() string { return "bgp" }

func (consumer *withdrawalEpisodeConsumer) InjectRoute(_ context.Context, _ family.Family, entry configredist.RouteEntry) {
	consumer.live[entry.SourceInstance] = entry
}

func (consumer *withdrawalEpisodeConsumer) WithdrawRoute(_ context.Context, _ family.Family, removal configredist.RouteRemoval) {
	consumer.removed = append(consumer.removed, removal)
	delete(consumer.live, removal.SourceInstance)
}
