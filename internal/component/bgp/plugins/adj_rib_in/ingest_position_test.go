// Design: docs/architecture/core-design.md -- Adj-RIB-In ingest position
// Related: rib.go -- noteIngested / ingestPosition under test

package adj_rib_in

import "testing"

// TestNoteIngestedIsAMonotonicMax pins the advance rule for the ingest position.
//
// VALIDATES: the position only ever moves forward, and reflects the newest
// MessageID this plugin has taken delivery of.
//
// PREVENTS: a late, low-numbered UPDATE pulling the position back below a cut a
// peer is already waiting on. MessageIDs are allocated per receive across ALL
// peers, so this plugin genuinely observes them out of numeric order; a plain
// store would let bgp-rs's replay conclude "covered" and then un-conclude it.
func TestNoteIngestedIsAMonotonicMax(t *testing.T) {
	r := &AdjRIBInManager{ingestTracked: true}

	for _, step := range []struct {
		note uint64
		want uint64
	}{
		{note: 5, want: 5},
		{note: 8, want: 8},
		{note: 3, want: 8}, // out-of-order arrival must not rewind
		{note: 8, want: 8}, // repeat is a no-op
		{note: 9, want: 9},
	} {
		r.noteIngested(step.note)
		got, tracked := r.ingestPosition()
		if !tracked {
			t.Fatal("position must be tracked on the in-process path")
		}
		if got != step.want {
			t.Fatalf("after noteIngested(%d): position = %d, want %d", step.note, got, step.want)
		}
	}
}

// TestIngestPositionTrackingIsPresenceNotValue pins the distinction the whole
// signal rests on.
//
// VALIDATES: "not tracked" is carried separately from the position's value, so
// a position of 0 is reportable as a real answer.
//
// PREVENTS: collapsing the two. Zero is the position of a plugin that has
// ingested nothing yet -- precisely the state in which bgp-rs must WAIT -- while
// "not tracked" means the responder stores no MessageIDs at all, so its replay
// is unbounded and bgp-rs must NOT wait. Same presence-vs-value lesson as
// replayCut (replay_cut.go).
func TestIngestPositionTrackingIsPresenceNotValue(t *testing.T) {
	t.Run("tracked and zero is a real answer", func(t *testing.T) {
		r := &AdjRIBInManager{ingestTracked: true}
		got, tracked := r.ingestPosition()
		if !tracked || got != 0 {
			t.Fatalf("ingestPosition() = (%d, %v), want (0, true)", got, tracked)
		}
	})

	t.Run("untracked reports no signal even after notes", func(t *testing.T) {
		r := &AdjRIBInManager{ingestTracked: false}
		r.noteIngested(42)
		_, tracked := r.ingestPosition()
		if tracked {
			t.Fatal("a plugin that stores no MessageIDs must report no ingest signal")
		}
	})
}
