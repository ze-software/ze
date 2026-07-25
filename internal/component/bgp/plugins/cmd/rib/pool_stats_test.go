package rib

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestHandlePoolStats verifies the pool-stats handler returns a well-formed
// response whose pool rows match the reported count and carry the expected
// per-pool keys.
//
// VALIDATES: pool-stats (ze-bgp:pool-stats) lives in the RIB command owner and
// returns per-attribute-pool occupancy plus aggregate totals.
// PREVENTS: pool-stats regressing to the central metrics verb, or returning a
// row count inconsistent with the "count" field.
func TestHandlePoolStats(t *testing.T) {
	ctx := &pluginserver.CommandContext{}
	resp, err := handlePoolStats(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "expected plugin.Map data")

	rows, ok := data["pools"].([]map[string]any)
	require.True(t, ok, "expected []map[string]any in pools field")
	assert.Equal(t, len(rows), data["count"], "count must match pool row length")

	// Aggregate totals are always present, even with zero pools.
	for _, key := range []string{
		"total-live-slots", "total-dead-slots",
		"total-live-bytes", "total-dead-bytes",
		"total-intern", "total-hits",
	} {
		_, present := data[key]
		assert.True(t, present, "missing aggregate key %q", key)
	}

	// Each row names its pool and reports a formatted dedup rate.
	for _, row := range rows {
		assert.Contains(t, row, "name")
		rate, ok := row["dedup-rate"].(string)
		require.True(t, ok, "dedup-rate must be a formatted string")
		assert.Contains(t, rate, "%")
	}
}
