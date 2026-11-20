// Design: docs/architecture/plugin/rib-storage-design.md -- a best row carries no `age`
// Related: rib_pipeline_best.go -- bestSource.Next, which takes no reading of the route clock
// Related: rib_pipeline_age_test.go -- the received row, which does carry the age

package rib

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bestRowsUnder answers the rows `show bgp rib best` writes with args, read
// from the envelope member key.
func bestRowsUnder(t *testing.T, r *RIBManager, args []string, key string) []map[string]any {
	t.Helper()
	var answer map[string][]map[string]any
	require.NoError(t, json.Unmarshal(mustMarshal(t, r.bestPipeline("*", ribPipe(args...))), &answer))
	rows, ok := answer[key]
	require.True(t, ok, "expected rows under %q, got %v", key, answer)
	require.Len(t, rows, 1, "one route was stored, so one best row is expected")
	return rows
}

// seedAgedBestRoute stores one route, lets thirty seconds pass on the route
// clock, and proves the received row of that route answers the age. A best row
// with no `age` is then the best source's decision, and not a route that holds
// no time.
func seedAgedBestRoute(t *testing.T) *RIBManager {
	t.Helper()
	r := newTestRIBManager(t)
	hand := startRouteAgeClock(r)
	injectRouteAgeRoute(t, r)
	hand.advance(30 * time.Second)
	require.Equal(t, float64(30), receivedRouteRow(t, r, "10.0.0.0/24")[rowKeyAge],
		"the stored route holds a time, so its received row carries the age")
	return r
}

// TestBestWalkRowCarriesNoAge reads the row the walk of `show bgp rib best`
// writes for a route whose received row carries an age.
//
// VALIDATES: the streamed best row has no `age` member, at the top level or
// among its attributes.
// PREVENTS: the walk answering the age of the winning route's attributes, which
// a reader of a best row takes for the time the path has been best.
func TestBestWalkRowCarriesNoAge(t *testing.T) {
	r := seedAgedBestRoute(t)

	row := bestRowsUnder(t, r, nil, bestPathEnvelopeKey)[0]
	require.Equal(t, "10.0.0.0/24", row[rowKeyPrefix])
	assert.NotContains(t, row, rowKeyAge)
	attributes, ok := row["attributes"].(map[string]any)
	require.True(t, ok, "the best row carries the winning route's attributes, got %v", row)
	assert.NotContains(t, attributes, rowKeyAge)
}
