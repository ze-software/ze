package redistevents

import (
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/core/events"

	"github.com/stretchr/testify/assert"
)

// TestReplayBatchJSONTagsStable pins the external JSON contract on the replay
// vocabulary: the replay-request payload marshals its correlation token under
// "replay-id" (the tag external, forked producers decode), and a
// RouteChangeBatch derives IsReplay() from its ReplayID (the single marker
// source). Holds before and after the type moves to the internal/core/replay
// leaf via alias (spec-unify-replay, A-4/AC-7).
func TestReplayBatchJSONTagsStable(t *testing.T) {
	data, err := json.Marshal(ReplayRequest{ReplayID: 42})
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"replay-id":42`,
		"replay-request must marshal the token under \"replay-id\"")

	assert.True(t, (&RouteChangeBatch{ReplayID: 7}).IsReplay(), "nonzero ReplayID is a replay")
	assert.False(t, (&RouteChangeBatch{}).IsReplay(), "zero ReplayID is not a replay")
}

// VALIDATES: the ReplayRequest event is registered as a payload-carrying typed
// event (not a payload-less signal like ribevents) so the orchestrator can
// carry the opaque ReplayID correlation token to producers. The handle is
// bound to the shared (redistribute, replay-request) pair.
// PREVENTS: registering it as a signal, which would lose the token and force a
// peer-in-the-batch fallback that breaks the peer-agnostic producer contract.
func TestReplayRequestEventRegistered(t *testing.T) {
	assert.Equal(t, ReplayNamespace, ReplayRequestEvent.Namespace())
	assert.Equal(t, ReplayRequestEventType, ReplayRequestEvent.EventType())

	// Registered as a payload-carrying event, NOT a signal.
	_, isSignal := events.PayloadInfo(ReplayNamespace, ReplayRequestEventType)
	assert.False(t, isSignal, "ReplayRequest must be payload-carrying, not a signal")

	// Zero value of the payload token is 0.
	var r ReplayRequest
	assert.Equal(t, uint64(0), r.ReplayID)
}

// VALIDATES: existing producers' batches default ReplayID=0 and IsReplay()
// reports false; a nonzero ReplayID flips IsReplay() to true. The pool clears
// ReplayID on Acquire and Release so a recycled batch never leaks a stale
// token into the next producer's emit.
// PREVENTS: a wire/behavior change for producers that never set ReplayID
// (AC-8, R-4) and a pooled batch carrying a stale replay token.
func TestRouteChangeBatch_ReplayIDBackCompat(t *testing.T) {
	var b RouteChangeBatch
	assert.Equal(t, uint64(0), b.ReplayID, "zero value must be 0 (incremental)")
	assert.False(t, b.IsReplay(), "ReplayID=0 is not a replay")

	b.ReplayID = 42
	assert.True(t, b.IsReplay(), "nonzero ReplayID is a replay")

	// Pool contract: Acquire returns ReplayID=0, and Release clears it.
	pb := AcquireBatch()
	assert.Equal(t, uint64(0), pb.ReplayID, "AcquireBatch must return ReplayID=0")
	pb.ReplayID = 7
	ReleaseBatch(pb)

	pb2 := AcquireBatch()
	defer ReleaseBatch(pb2)
	assert.Equal(t, uint64(0), pb2.ReplayID, "ReleaseBatch must clear ReplayID")
}
