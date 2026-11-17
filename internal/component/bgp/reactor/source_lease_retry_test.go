// Design: docs/architecture/api/process-protocol.md -- terminal source release.
// Related: source_receipt.go -- revoked writes own memory until completion.
package reactor

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// sourceLeaseRetryAdapter reaches the real public evaluator and, on JSON,
// the cancellation admission that must accept the same terminal retry.
func sourceLeaseRetryAdapter(server *pluginserver.Server, ctx context.Context, proc *process.Process, native bool, input rpc.SourceLeaseInput) (*rpc.SourceLeaseOutput, error) {
	if native {
		value, err := sourceLeaseNative(server, ctx, proc, input)
		if err != nil {
			return nil, err
		}
		output, ok := value.(*rpc.SourceLeaseOutput)
		if !ok {
			panic("BUG: source lease native result is not *rpc.SourceLeaseOutput")
		}
		return output, nil
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	if _, err := sourceLeaseCancellation(server, proc, encoded); err != nil {
		return nil, err
	}
	value, err := sourceLeaseJSON(server, ctx, proc, encoded)
	if err != nil {
		return nil, err
	}
	output, ok := value.(*rpc.SourceLeaseOutput)
	if !ok {
		panic("BUG: source lease JSON result is not *rpc.SourceLeaseOutput")
	}
	return output, nil
}

// TestSourceLeaseReleaseRetryAfterCanceledJoin holds a real final write until
// its revoked lease's first caller has gone, then retries after that write
// removes the lease. The other owner's retained source must stay useful.
func TestSourceLeaseReleaseRetryAfterCanceledJoin(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "json", true: "native"}[native], func(t *testing.T) {
			work, stop := context.WithTimeout(t.Context(), 5*time.Second)
			defer stop()
			makeProcess := sourceLeaseProcessFactory(t)
			first, other := makeProcess("rib-one"), makeProcess("rib-other")
			peer, facts := sourceReceiptFixture(t)
			peer.settings.ProcessBindings = []ProcessBinding{sourceLeaseGrant(first.Name()), sourceLeaseGrant(other.Name())}
			live, err := sourceReceipts.issue(facts)
			require.NoError(t, err)
			t.Cleanup(live.close)
			server := &pluginserver.Server{}
			for _, proc := range []*process.Process{first, other} {
				for _, action := range []rpc.SourceLeaseAction{rpc.SourceLeaseAcquire, rpc.SourceLeaseRetain} {
					_, err = sourceLeaseRetryAdapter(server, work, proc, native, sourceLeaseRequest(action, live.ID(), peer.sourceOwner))
					require.NoError(t, err)
				}
			}
			owner, err := first.SelectedOwner()
			require.NoError(t, err)
			otherOwner, err := other.SelectedOwner()
			require.NoError(t, err)
			peer.state.Store(int32(PeerStateStopped))
			live.close()
			write, err := sourceReceipts.admitWrite(live.ID(), owner, peer.sourcePolicyEpoch.Load())
			require.NoError(t, err)
			defer write.complete()
			request, cancel := context.WithCancel(work)
			done := make(chan error, 1)
			go func() {
				_, releaseErr := sourceLeaseRetryAdapter(server, request, first, native, sourceLeaseRequest(rpc.SourceLeaseRelease, live.ID(), peer.sourceOwner))
				done <- releaseErr
			}()
			select {
			case <-write.done():
			case <-work.Done():
				t.Fatal("release did not revoke its held final write")
			}
			cancel()
			select {
			case err := <-done:
				require.ErrorIs(t, err, context.Canceled)
			case <-work.Done():
				t.Fatal("the canceled release did not return its unsettled result")
			}
			write.complete()
			for range 2 {
				input := sourceLeaseRequest(rpc.SourceLeaseRelease, live.ID(), peer.sourceOwner)
				output, err := sourceLeaseRetryAdapter(server, work, first, native, input)
				require.NoError(t, err, "a completed late join must settle its retry without lease history")
				require.NoError(t, output.Validate(input))
			}
			for _, action := range []rpc.SourceLeaseAction{rpc.SourceLeaseAcquire, rpc.SourceLeaseRetain} {
				_, err = sourceLeaseRetryAdapter(server, work, first, native, sourceLeaseRequest(action, live.ID(), peer.sourceOwner))
				require.Error(t, err, "terminal release must not recreate original authority")
			}
			useful, err := sourceReceipts.admitWrite(live.ID(), otherOwner, peer.sourcePolicyEpoch.Load())
			require.NoError(t, err, "another owner's retained lease remains useful")
			require.NoError(t, useful.current())
			useful.complete()
			_, err = sourceLeaseRetryAdapter(server, work, other, native, sourceLeaseRequest(rpc.SourceLeaseRelease, live.ID(), peer.sourceOwner))
			require.NoError(t, err)
			sourceReceipts.mu.Lock()
			_, retained := sourceReceipts.entries[live.ID()]
			sourceReceipts.mu.Unlock()
			require.False(t, retained, "settled release keeps no original-source history")
		})
	}
}

// TestSourceLeaseReleaseAbsentOwnerIsTerminal proves a live authenticated
// caller can dispose of no authority without acquiring or changing a sibling.
func TestSourceLeaseReleaseAbsentOwnerIsTerminal(t *testing.T) {
	makeProcess := sourceLeaseProcessFactory(t)
	first, other := makeProcess("rib-one"), makeProcess("rib-other")
	peer, facts := sourceReceiptFixture(t)
	peer.settings.ProcessBindings = []ProcessBinding{sourceLeaseGrant(other.Name())}
	live, err := sourceReceipts.issue(facts)
	require.NoError(t, err)
	t.Cleanup(live.close)
	server := &pluginserver.Server{}
	_, err = sourceLeaseApply(server, t.Context(), other, sourceLeaseRequest(rpc.SourceLeaseAcquire, live.ID(), peer.sourceOwner))
	require.NoError(t, err)
	for _, receipt := range []uint64{live.ID(), ^uint64(0)} {
		input := sourceLeaseRequest(rpc.SourceLeaseRelease, receipt, peer.sourceOwner)
		output, err := sourceLeaseApply(server, t.Context(), first, input)
		require.NoError(t, err)
		require.NoError(t, output.Validate(input))
		for _, action := range []rpc.SourceLeaseAction{rpc.SourceLeaseAcquire, rpc.SourceLeaseRetain} {
			_, err = sourceLeaseApply(server, t.Context(), first, sourceLeaseRequest(action, receipt, peer.sourceOwner))
			require.Error(t, err, "disposal gives no source grant")
		}
	}
	owner, err := other.SelectedOwner()
	require.NoError(t, err)
	useful, err := sourceReceipts.admitWrite(live.ID(), owner, peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	require.NoError(t, useful.current())
	useful.complete()
	_, err = sourceLeaseApply(server, t.Context(), first, sourceLeaseRequest(rpc.SourceLeaseRelease, 0, peer.sourceOwner))
	require.Error(t, err, "zero remains malformed input")
}
