package replay

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestReplayTokenDisjointCases validates that the three token cases -- an
// incremental change (0), a broadcast replay (Broadcast), and a targeted replay
// (any other nonzero token an orchestrator allocates) -- are three disjoint
// cases, and that IsReplay() distinguishes them exactly (spec-unify-replay
// AC-6, A-3, R-2). If Broadcast collided with 0 or with a targeted token,
// normal incremental batches would be mis-routed as replays.
func TestReplayTokenDisjointCases(t *testing.T) {
	// The three cases are pairwise distinct.
	const targeted uint64 = 1 // first token the redistribute orchestrator allocates
	assert.NotEqual(t, uint64(0), Broadcast, "broadcast must not collide with incremental (0)")
	assert.NotEqual(t, targeted, Broadcast, "broadcast must not collide with a targeted token")
	assert.NotEqual(t, uint64(0), targeted, "a targeted token is never 0")

	// IsReplay classifies each case correctly.
	assert.False(t, IsReplay(0), "token 0 is a normal incremental change, not a replay")
	assert.True(t, IsReplay(Broadcast), "the broadcast sentinel is a replay")
	assert.True(t, IsReplay(targeted), "a targeted token is a replay")

	// The broadcast sentinel is the top of the range, unreachable by the
	// orchestrator's monotonic-from-1 allocator in any realistic run.
	assert.Equal(t, uint64(0xFFFFFFFFFFFFFFFF), Broadcast)
}

// TestRequestJSONTag pins the "replay-id" wire tag on the shared request
// payload; forked plugin producers decode it, so it must never move
// (spec-unify-replay A-4).
func TestRequestJSONTag(t *testing.T) {
	data, err := json.Marshal(Request{ReplayID: 42})
	assert.NoError(t, err)
	assert.JSONEq(t, `{"replay-id":42}`, string(data))

	var r Request
	assert.NoError(t, json.Unmarshal([]byte(`{"replay-id":7}`), &r))
	assert.Equal(t, uint64(7), r.ReplayID)
}
