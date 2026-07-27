// Design: docs/architecture/core-design.md -- route-server peer-up replay convergence
// Related: server_handlers.go -- replayForPeer's catch-up phase under test

package rs

import (
	"encoding/json"
	"testing"
)

// TestReplayProgressCoversCut pins the convergence predicate that decides
// whether an empty replay answer may be trusted as "done".
//
// VALIDATES: a replay bounded at `cut` is only able to carry the routes the
// live rail suppressed once adj-rib-in has ingested through that cut.
//
// PREVENTS: the route loss in test/plugin/forward-overflow-two-tier.ci. bgp-rs
// suppresses every UPDATE at or below its cut on the live rail
// (server_forward.go) on the promise that this replay carries them. bgp-rs and
// bgp-adj-rib-in consume events on independent goroutines (one eventChan per
// Process, plugin/process/delivery.go), so adj-rib-in routinely trails. When it
// had not yet ingested the UPDATE, the full replay returned empty, the delta
// loop broke immediately on `lastIndex == 0` -- which means "zero routes
// replayed", not "caught up" -- and neither rail delivered the route.
func TestReplayProgressCoversCut(t *testing.T) {
	pos := func(v uint64) *uint64 { return &v }

	for _, tt := range []struct {
		name string
		prog replayProgress
		cut  uint64
		want bool
	}{
		{
			// The losing case: ingested nothing, cut well ahead. An empty answer
			// here means "not there yet", and trusting it drops routes.
			name: "ingested zero with a live cut is NOT covered",
			prog: replayProgress{ingested: pos(0)},
			cut:  8,
			want: false,
		},
		{
			name: "behind the cut is not covered",
			prog: replayProgress{ingested: pos(7)},
			cut:  8,
			want: false,
		},
		{
			name: "level with the cut is covered",
			prog: replayProgress{ingested: pos(8)},
			cut:  8,
			want: true,
		},
		{
			name: "past the cut is covered",
			prog: replayProgress{ingested: pos(9)},
			cut:  8,
			want: true,
		},
		{
			// A responder that tracks no position stores no MessageIDs, so
			// replayCut never excludes any of its routes and its replay is
			// already unbounded. Nothing to wait for -- and waiting would tax
			// every peer-up. This is why the field is a POINTER: absent must not
			// read as 0, which is the losing case above.
			name: "no signal reads as covered, and is not the same as zero",
			prog: replayProgress{ingested: nil},
			cut:  8,
			want: true,
		},
		{
			name: "cut of zero is covered by an ingest of zero",
			prog: replayProgress{ingested: pos(0)},
			cut:  0,
			want: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.prog.coversCut(tt.cut); got != tt.want {
				t.Fatalf("coversCut(%d) with ingested=%v = %v, want %v",
					tt.cut, tt.prog.ingested, got, tt.want)
			}
		})
	}
}

// TestParseReplayProgressDistinguishesAbsentFromZero is the wire half of the
// same invariant: the JSON decode must not turn an omitted ingest position into
// a present zero.
//
// VALIDATES: `ingested-msg-id` round-trips as presence + value.
//
// PREVENTS: silently reintroducing the zero-value trap at the parse boundary --
// a plain uint64 field would decode both "omitted" and "0" to 0, and the two
// demand opposite behavior (proceed vs wait).
func TestParseReplayProgressDistinguishesAbsentFromZero(t *testing.T) {
	t.Run("omitted stays nil", func(t *testing.T) {
		p := parseReplayProgress(json.RawMessage(`{"last-index":3,"replayed":2}`))
		if p.ingested != nil {
			t.Fatalf("absent ingested-msg-id decoded to %v, want nil", *p.ingested)
		}
		if p.cursor != 3 || p.replayed != 2 {
			t.Fatalf("cursor/replayed = %d/%d, want 3/2", p.cursor, p.replayed)
		}
	})

	t.Run("explicit zero is present and zero", func(t *testing.T) {
		p := parseReplayProgress(json.RawMessage(`{"last-index":0,"replayed":0,"ingested-msg-id":0}`))
		if p.ingested == nil {
			t.Fatal("explicit ingested-msg-id:0 decoded to nil; absent and zero must differ")
		}
		if *p.ingested != 0 {
			t.Fatalf("ingested = %d, want 0", *p.ingested)
		}
		if p.coversCut(8) {
			t.Fatal("ingested 0 must not cover cut 8: this is the route-loss case")
		}
	})

	t.Run("malformed payload yields no signal rather than a false position", func(t *testing.T) {
		p := parseReplayProgress(json.RawMessage(`not json`))
		if p.ingested != nil {
			t.Fatal("malformed payload must not fabricate an ingest position")
		}
	})
}
