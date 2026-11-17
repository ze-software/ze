// Design: docs/architecture/api/process-protocol.md -- retained source lease contract.
package rpc

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSourceLeaseInputBoundary keeps every action explicit and refuses missing
// authority or an expired/unbounded request before either transport dispatches.
func TestSourceLeaseInputBoundary(t *testing.T) {
	now := time.Now()
	input := SourceLeaseInput{Action: SourceLeaseAcquire, SourceReceipt: 7, PeerOwner: 701, DeadlineMS: now.Add(time.Second).UnixMilli()}
	for _, action := range []SourceLeaseAction{SourceLeaseAcquire, SourceLeaseRetain, SourceLeaseRelease} {
		input.Action = action
		require.NoError(t, input.Validate(now))
	}
	for _, action := range []SourceLeaseAction{SourceLeaseUnspecified, SourceLeaseAction(99)} {
		input.Action = action
		require.Error(t, input.Validate(now))
	}
	input.Action = SourceLeaseAcquire
	input.SourceReceipt = 0
	require.Error(t, input.Validate(now))
	input.SourceReceipt = 7
	input.DeadlineMS = now.UnixMilli()
	require.Error(t, input.Validate(now))
	input.DeadlineMS = now.Add(CancellableRequestTimeout + time.Second).UnixMilli()
	require.Error(t, input.Validate(now))
}

// TestSourceLeaseNumericJSON keeps the full issuer identity width and rejects
// negative, overflowing and string-encoded identities at the JSON boundary.
func TestSourceLeaseNumericJSON(t *testing.T) {
	input := SourceLeaseInput{Action: SourceLeaseAcquire, SourceReceipt: math.MaxUint64, PeerOwner: 701,
		DeadlineMS: time.Now().Add(time.Second).UnixMilli()}
	raw, err := json.Marshal(input)
	require.NoError(t, err)
	var decoded SourceLeaseInput
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Equal(t, input, decoded)
	require.NoError(t, decoded.Validate(time.Now()))
	for _, raw := range []string{
		`{"action":1,"source-receipt":-1,"deadline-ms":1}`,
		`{"action":1,"source-receipt":18446744073709551616,"deadline-ms":1}`,
		`{"action":1,"source-receipt":"7","deadline-ms":1}`,
		`{"action":256,"source-receipt":7,"deadline-ms":1}`,
	} {
		require.Error(t, json.Unmarshal([]byte(raw), &decoded))
	}
}

// TestSourceLeaseOutputRequiresExactSettlement rejects a missing or rebound
// receipt and an answer which does not settle the requested lifecycle action.
func TestSourceLeaseOutputRequiresExactSettlement(t *testing.T) {
	input := SourceLeaseInput{Action: SourceLeaseRelease, SourceReceipt: 7, PeerOwner: 701}
	output := &SourceLeaseOutput{Action: SourceLeaseRelease, SourceReceipt: 7, PeerOwner: 701}
	require.NoError(t, output.Validate(input))
	output.SourceReceipt = 8
	require.Error(t, output.Validate(input))
	output.SourceReceipt = 7
	output.Action = SourceLeaseRetain
	require.Error(t, output.Validate(input))
	var missing *SourceLeaseOutput
	require.Error(t, missing.Validate(input))
	for _, raw := range []string{`{}`, `{"action":3}`, `{"action":null,"source-receipt":7}`, `{"action":3,"source-receipt":null}`} {
		var decoded SourceLeaseOutput
		require.Error(t, json.Unmarshal([]byte(raw), &decoded))
	}
}
