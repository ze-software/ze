// Design: docs/architecture/api/ipc_protocol.md -- EventKind enum wire strings and boundaries
// Related: enums.go -- EventKindUpdateFiltered and EventKindCount
package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEventKindUpdateFilteredRoundTrips verifies the update-filtered event kind
// wire string, text marshal/unmarshal round-trip, and enum ordinal boundaries.
//
// VALIDATES: spec-rfc7947-adj-rib-in-accepts-filtered-updates EventKindUpdateFiltered
// wire encoding and boundary checks (0 and EventKindCount refuse to marshal).
func TestEventKindUpdateFilteredRoundTrips(t *testing.T) {
	assert.Less(t, EventKindUpdateFiltered, EventKindCount)
	assert.Equal(t, "update-filtered", EventKindUpdateFiltered.String())

	raw, err := EventKindUpdateFiltered.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "update-filtered", string(raw))

	var got EventKind
	require.NoError(t, got.UnmarshalText(raw))
	assert.Equal(t, EventKindUpdateFiltered, got)

	_, err = EventKindUnspecified.MarshalText()
	assert.Error(t, err, "unspecified ordinal 0 must refuse to marshal")

	_, err = EventKindCount.MarshalText()
	assert.Error(t, err, "count sentinel ordinal must refuse to marshal")
}
