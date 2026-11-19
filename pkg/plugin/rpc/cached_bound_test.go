// Design: docs/architecture/api/process-protocol.md -- exact bounded completion shape.
package rpc

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
	"time"
)

// TestCachedWriteBoundaryRefusesPartialTupleAndMissingCompletion proves full
// uint64 constraints survive JSON and only a same-deadline final receipt passes.
func TestCachedWriteBoundaryRefusesPartialTupleAndMissingCompletion(t *testing.T) {
	input := RelayStoredRouteInput{Destination: "192.0.2.7", DestinationOwner: math.MaxUint64, DestinationSession: math.MaxUint64 - 1, DeadlineMS: time.Now().Add(time.Second).UnixMilli()}
	raw, err := json.Marshal(input)
	require.NoError(t, err)
	var decoded RelayStoredRouteInput
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Equal(t, input, decoded)
	require.NoError(t, ValidateCachedWrite(input.DestinationOwner, input.DestinationSession, input.DeadlineMS))
	require.NoError(t, ValidateCachedWrite(0, 0, 0), "explicit unbound operators keep their contract")
	for _, tuple := range [][3]int64{{0, 1, 1}, {1, 0, 1}, {1, 1, 0}, {1, 1, -1}, {0, 0, 1}} {
		require.Error(t, ValidateCachedWrite(uint64(tuple[0]), uint64(tuple[1]), tuple[2]))
	}
	for _, output := range []*CachedWriteOutput{nil, {}, {Completed: false, DeadlineMS: input.DeadlineMS}, {Completed: true, DeadlineMS: input.DeadlineMS + 1}} {
		require.Error(t, output.Validate(input.DeadlineMS))
	}
	require.NoError(t, (&CachedWriteOutput{Completed: true, DeadlineMS: input.DeadlineMS}).Validate(input.DeadlineMS))
}

// TestCachedWriteContextKeepsEarlierCallerDeadline checks normalization uses
// the original request's earlier bound, while malformed partial input refuses.
func TestCachedWriteContextKeepsEarlierCallerDeadline(t *testing.T) {
	parent, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	deadline := time.Now().Add(time.Minute).UnixMilli()
	work, stop, err := CachedWriteContext(parent, 1, 2, &deadline)
	require.NoError(t, err)
	defer stop()
	limit, _ := parent.Deadline()
	got, _ := work.Deadline()
	require.Equal(t, limit, got)
	require.Equal(t, limit.UnixMilli(), deadline)
	zero := int64(0)
	_, _, err = CachedWriteContext(t.Context(), 0, 2, &zero)
	require.Error(t, err)
}
