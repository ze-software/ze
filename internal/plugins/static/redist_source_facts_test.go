// Design: docs/guide/redistribution.md -- original producer facts survive redistribution
// Related: inject.go -- emitRouteChangeID, shared by incremental and replay output

package static

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/redistevents"

	"github.com/stretchr/testify/require"
)

// TestStaticRedistributionPreservesPresentZeroDistance captures the actual
// producer's add, replay and remove, then decodes their public value payload.
// It proves that configured distance zero remains present on all three paths.
func TestStaticRedistributionPreservesPresentZeroDistance(t *testing.T) {
	bus := &staticRecordingBus{}
	setEventBus(bus)
	t.Cleanup(func() { eventBusPtr.Store(nil) })
	manager := newTestRouteManager(&mockStaticBackend{})
	route := staticRoute{
		Prefix:      netip.MustParsePrefix("192.0.2.0/24"),
		Action:      actionBlackhole,
		Distance:    0,
		HasDistance: true,
	}
	require.NoError(t, manager.applyRoutes([]staticRoute{route}))
	manager.reemitAll(91)
	require.NoError(t, manager.applyRoutes(nil))
	batches := bus.events()
	require.Len(t, batches, 3)
	for i, batch := range batches {
		require.Len(t, batch.Entries, 1)
		data, err := json.Marshal(batch.Entries[0])
		require.NoError(t, err)
		var value map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &value))
		require.JSONEq(t, "0", string(value["source-instance"]), "event %d aggregate instance", i)
		require.JSONEq(t, "0", string(value["distance-override"]), "event %d distance", i)
		require.JSONEq(t, "true", string(value["has-distance-override"]), "event %d distance presence", i)
	}
	require.Equal(t, redistevents.ActionAdd, batches[0].Entries[0].Action)
	require.Equal(t, uint64(91), batches[1].ReplayID)
	require.Equal(t, redistevents.ActionRemove, batches[2].Entries[0].Action)
}
