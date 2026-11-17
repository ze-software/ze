// Design: docs/contributing/testing.md -- real plugin transport fixtures.
package pluginengine

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestSourceLeaseFixtureRequiresIssuedSession(t *testing.T) {
	e := &Engine{}
	input := rpc.SourceLeaseInput{Action: rpc.SourceLeaseAcquire, SourceReceipt: 97, PeerOwner: 701, DeadlineMS: time.Now().Add(time.Second).UnixMilli()}
	encoded, err := json.Marshal(input)
	require.NoError(t, err)
	_, err = e.answer(rpc.MethodSourceLease, encoded)
	require.ErrorContains(t, err, "not issued")
	first := e.IssueSource("192.0.2.1", 701)
	second := e.IssueSource("192.0.2.1", 701)
	require.NotEqual(t, first, second, "a successor Session gets its own receipt")
	input.SourceReceipt = first
	for _, action := range []rpc.SourceLeaseAction{rpc.SourceLeaseRetain} {
		input.Action = action
		encoded, err = json.Marshal(input)
		require.NoError(t, err)
		_, err = e.answer(rpc.MethodSourceLease, encoded)
		require.ErrorContains(t, err, "not acquired")
	}
	for _, action := range []rpc.SourceLeaseAction{rpc.SourceLeaseAcquire, rpc.SourceLeaseRetain, rpc.SourceLeaseRelease} {
		input.Action = action
		encoded, err = json.Marshal(input)
		require.NoError(t, err)
		answer, err := e.answer(rpc.MethodSourceLease, encoded)
		require.NoError(t, err)
		output, ok := answer.(*rpc.SourceLeaseOutput)
		require.True(t, ok)
		require.NoError(t, output.Validate(input))
	}
	require.False(t, e.sources[first].acquired)
	require.False(t, e.sources[first].retained)
	require.False(t, e.sources[second].acquired, "another Session's lease is untouched")
}

// TestSourceLeaseFixtureReleaseIsTerminal keeps the fake engine on the same
// no-history disposal contract as the real authenticated source operation.
func TestSourceLeaseFixtureReleaseIsTerminal(t *testing.T) {
	e := &Engine{}
	first, sibling := e.IssueSource("192.0.2.1", 701), e.IssueSource("192.0.2.2", 702)
	invoke := func(action rpc.SourceLeaseAction, receipt, peerOwner uint64) (*rpc.SourceLeaseOutput, error) {
		input := rpc.SourceLeaseInput{Action: action, SourceReceipt: receipt, PeerOwner: peerOwner, DeadlineMS: time.Now().Add(time.Second).UnixMilli()}
		encoded, err := json.Marshal(input)
		require.NoError(t, err)
		answer, err := e.answer(rpc.MethodSourceLease, encoded)
		if err != nil {
			return nil, err
		}
		output, ok := answer.(*rpc.SourceLeaseOutput)
		require.True(t, ok)
		require.NoError(t, output.Validate(input))
		return output, nil
	}
	_, err := invoke(rpc.SourceLeaseAcquire, sibling, 702)
	require.NoError(t, err)
	for _, receipt := range []uint64{first, first, ^uint64(0)} {
		_, err = invoke(rpc.SourceLeaseRelease, receipt, 701)
		require.NoError(t, err)
	}
	require.Len(t, e.sources, 2, "unknown disposal must not allocate source history")
	require.False(t, e.sources[first].acquired)
	require.True(t, e.sources[sibling].acquired)
	_, err = invoke(rpc.SourceLeaseRetain, sibling, 702)
	require.NoError(t, err, "a useful sibling lease survives terminal disposal")
	_, err = invoke(rpc.SourceLeaseRetain, first, 701)
	require.ErrorContains(t, err, "not acquired", "disposal does not grant retention")
	_, err = invoke(rpc.SourceLeaseAcquire, ^uint64(0), 701)
	require.ErrorContains(t, err, "not issued")
}
