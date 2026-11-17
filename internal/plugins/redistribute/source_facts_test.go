// Design: docs/architecture/core-design.md -- producer facts at consumer dispatch
// Related: redistribute.go -- applySets preserves source preference metadata

package redistribute

import (
	"context"
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/redistevents"

	"github.com/stretchr/testify/require"
)

// TestRedistributionPreservesProducerFacts captures actual incremental dispatch
// with an import metric transform. Source identity and present-zero distance
// survive independently of that transform and its explicit MED-presence flag.
func TestRedistributionPreservesProducerFacts(t *testing.T) {
	id, consumer, _ := orderRig(t)
	installRules(bgpRules(configredist.ImportRule{Final: "accept", Chain: metricChain(55)}))
	batch := orderBatch(id, redistevents.ActionAdd, 0)
	batch.Entries[0].SourceInstance = 73
	batch.Entries[0].Table = 19
	batch.Entries[0].Metric = 8
	batch.Entries[0].HasDistanceOverride = true
	handleBatch(context.Background(), skipIDs(), batch)
	injected := consumer.snapshotInjected()
	require.Len(t, injected, 1)
	entry := injected[0].entry
	require.Equal(t, id, entry.Protocol)
	require.Equal(t, uint32(73), entry.SourceInstance)
	require.Equal(t, uint32(19), entry.Table)
	require.True(t, entry.HasDistanceOverride)
	require.Zero(t, entry.DistanceOverride)
	require.Equal(t, uint32(55), entry.Metric)
	require.True(t, entry.HasMetric)
}

// TestRedistributionReplayNoticesDistanceChange changes only the source's own
// distance between two actual consumer replays. The second must dispatch the
// new preference while leaving MED absent without an import metric transform.
func TestRedistributionReplayNoticesDistanceChange(t *testing.T) {
	id, consumer, coordinator := orderRig(t)
	first, fired := coordinator.onConsumerRegistered(newTestBus(), "bgp")
	require.True(t, fired)
	batch := orderBatch(id, redistevents.ActionAdd, first)
	batch.Entries[0].HasDistanceOverride = true
	batch.Entries[0].DistanceOverride = 7
	handleBatch(context.Background(), skipIDs(), batch)
	next, fired := coordinator.onConsumerRegistered(newTestBus(), "bgp")
	require.True(t, fired)
	batch.ReplayID = next
	batch.Entries[0].DistanceOverride = 0
	handleBatch(context.Background(), skipIDs(), batch)
	injected := consumer.snapshotInjected()
	require.Len(t, injected, 2)
	require.Equal(t, uint8(7), injected[0].entry.DistanceOverride)
	require.True(t, injected[1].entry.HasDistanceOverride)
	require.Zero(t, injected[1].entry.DistanceOverride)
	require.False(t, injected[1].entry.HasMetric)
}

// TestRedistributionReplayNoticesDistancePresenceChange removes only the source
// override's presence bit while keeping its value zero. The replay must carry
// the now-absent preference rather than retain a present-zero override.
func TestRedistributionReplayNoticesDistancePresenceChange(t *testing.T) {
	id, consumer, coordinator := orderRig(t)
	first, fired := coordinator.onConsumerRegistered(newTestBus(), "bgp")
	require.True(t, fired)
	batch := orderBatch(id, redistevents.ActionAdd, first)
	batch.Entries[0].HasDistanceOverride = true
	handleBatch(context.Background(), skipIDs(), batch)
	next, fired := coordinator.onConsumerRegistered(newTestBus(), "bgp")
	require.True(t, fired)
	batch.ReplayID = next
	batch.Entries[0].HasDistanceOverride = false
	handleBatch(context.Background(), skipIDs(), batch)
	injected := consumer.snapshotInjected()
	require.Len(t, injected, 2)
	require.True(t, injected[0].entry.HasDistanceOverride)
	require.False(t, injected[1].entry.HasDistanceOverride)
	require.Zero(t, injected[0].entry.DistanceOverride)
	require.Zero(t, injected[1].entry.DistanceOverride)
}
