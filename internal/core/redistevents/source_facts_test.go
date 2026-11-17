// Design: docs/architecture/core-design.md -- redistribution source value contract
// Related: pool.go -- released entries lose every original producer fact

package redistevents

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/memguard"

	"github.com/stretchr/testify/require"
)

// TestRouteChangeSourceFactsJSON round-trips independent instance and distance
// presence, including the valid present-zero value, through the public payload.
func TestRouteChangeSourceFactsJSON(t *testing.T) {
	for _, present := range []bool{false, true} {
		entry := RouteChangeEntry{
			Action:              ActionAdd,
			Prefix:              netip.MustParsePrefix("198.51.100.0/24"),
			SourceInstance:      37,
			DistanceOverride:    0,
			HasDistanceOverride: present,
		}
		data, err := json.Marshal(entry)
		require.NoError(t, err)
		var values map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &values))
		require.JSONEq(t, "37", string(values["source-instance"]))
		require.JSONEq(t, "0", string(values["distance-override"]))
		var presence bool
		require.NoError(t, json.Unmarshal(values["has-distance-override"], &presence))
		require.Equal(t, present, presence)
		var restored RouteChangeEntry
		require.NoError(t, json.Unmarshal(data, &restored))
		require.Equal(t, entry, restored)
	}
}

// TestRouteChangePoolDropsSourceFacts inspects the original backing slot after
// ReleaseBatch, so sync.Pool allocation choices cannot turn reset into a guess.
func TestRouteChangePoolDropsSourceFacts(t *testing.T) {
	batch := AcquireBatch()
	batch.Entries = append(batch.Entries, RouteChangeEntry{
		Action: ActionAdd, SourceInstance: 37,
		DistanceOverride: 7, HasDistanceOverride: true,
	})
	retained := batch.Entries[:1]
	ReleaseBatch(batch)
	if memguard.Enabled {
		require.Equal(t, uint32(0xDEADBEEF), retained[0].SourceInstance)
		require.Equal(t, uint8(0xEE), retained[0].DistanceOverride)
	} else {
		require.Zero(t, retained[0].SourceInstance)
		require.Zero(t, retained[0].DistanceOverride)
	}
	require.False(t, retained[0].HasDistanceOverride)
}
