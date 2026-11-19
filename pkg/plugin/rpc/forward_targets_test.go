// Design: docs/architecture/api/process-protocol.md -- complete captured target sets.
package rpc

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestForwardCachedTargetSetRefusesPartialMixedAndRepeatedTargets(t *testing.T) {
	target := ForwardTarget{Destination: "192.0.2.8:179", DestinationOwner: ^uint64(0), DestinationSession: ^uint64(0) - 1}
	input := ForwardCachedInput{IDs: []uint64{1, 2}, Targets: []ForwardTarget{target}, DeadlineMS: time.Now().Add(time.Second).UnixMilli()}
	require.NoError(t, input.Validate())
	bytes, err := json.Marshal(input)
	require.NoError(t, err)
	var decoded ForwardCachedInput
	require.NoError(t, json.Unmarshal(bytes, &decoded))
	require.Equal(t, input.IDs, decoded.IDs)
	require.Equal(t, input.Targets, decoded.Targets)
	require.Equal(t, input.DeadlineMS, decoded.DeadlineMS)
	require.Contains(t, string(bytes), `"destination-owner":"18446744073709551615"`)
	for _, mutate := range []func(*ForwardCachedInput){
		func(i *ForwardCachedInput) { i.Targets[0].DestinationOwner = 0 },
		func(i *ForwardCachedInput) { i.Targets[0].DestinationSession = 0 },
		func(i *ForwardCachedInput) { i.Targets[0].Destination = "" },
		func(i *ForwardCachedInput) { i.Destinations = []string{"192.0.2.8"} },
		func(i *ForwardCachedInput) { i.Targets = append(i.Targets, i.Targets[0]) },
		func(i *ForwardCachedInput) { i.Targets = nil },
	} {
		candidate := input
		candidate.Targets = append([]ForwardTarget(nil), input.Targets...)
		mutate(&candidate)
		candidate.IDs = nil // Empty work must not hide malformed destination authority.
		require.Error(t, candidate.Validate())
	}
	for _, raw := range []string{`{"ids":[],"targets":null}`, `{"ids":[],"targets":[]}`} {
		var malformed ForwardCachedInput
		require.NoError(t, json.Unmarshal([]byte(raw), &malformed))
		require.Error(t, malformed.Validate())
	}
	var oldScalar ForwardCachedInput
	require.Error(t, json.Unmarshal([]byte(`{"destination-owner":7,"destination-session":8,"destinations":["192.0.2.8"]}`), &oldScalar))
	sibling := target
	sibling.DestinationOwner--
	input.Targets = append(input.Targets, sibling)
	require.NoError(t, input.Validate(), "same address with a different original owner stays distinct")
}
