// Design: docs/architecture/api/process-protocol.md -- original Peer lease binding.
package rpc

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSourceLeasePeerOwnerBoundary exercises raw inputs and decoded answers;
// acquire/retain bind nonzero owners, while terminal release may echo zero.
func TestSourceLeasePeerOwnerBoundary(t *testing.T) {
	input := SourceLeaseInput{SourceReceipt: 7, DeadlineMS: time.Now().Add(time.Second).UnixMilli()}
	for _, action := range []SourceLeaseAction{SourceLeaseAcquire, SourceLeaseRetain} {
		input.Action = action
		input.PeerOwner = 0
		require.Error(t, input.Validate(time.Now()))
		input.PeerOwner = ^uint64(0)
		require.NoError(t, input.Validate(time.Now()))
		output := &SourceLeaseOutput{Action: action, SourceReceipt: input.SourceReceipt, PeerOwner: input.PeerOwner}
		require.NoError(t, output.Validate(input))
		output.PeerOwner = 701
		require.Error(t, output.Validate(input))
	}
	input.Action, input.PeerOwner = SourceLeaseRelease, 0
	require.NoError(t, input.Validate(time.Now()))
	require.NoError(t, (&SourceLeaseOutput{Action: SourceLeaseRelease, SourceReceipt: 7}).Validate(input))
	for _, raw := range []string{`{"action":3,"source-receipt":7}`, `{"action":3,"source-receipt":7,"peer-owner":null}`, `{"action":3,"source-receipt":7,"peer-owner":-1}`, `{"action":3,"source-receipt":7,"peer-owner":"7"}`} {
		var output SourceLeaseOutput
		require.Error(t, json.Unmarshal([]byte(raw), &output))
	}
	var output SourceLeaseOutput
	require.NoError(t, json.Unmarshal([]byte(`{"action":3,"source-receipt":7,"peer-owner":18446744073709551615}`), &output))
	require.Equal(t, ^uint64(0), output.PeerOwner)
}
