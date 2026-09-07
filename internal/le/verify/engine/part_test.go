// VALIDATES: a cut verification runs each stage in exactly one piece, refuses a
// piece that names nothing, and writes a certificate no reader can take for a
// pass over the tree.
// PREVENTS: the failure that makes cutting a run worth doing at all becoming the
// failure it introduces -- a piece of a verification crediting the stages it
// never started.
package verifyengine

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/job"
)

// TestEveryStageIsDealtIntoExactlyOnePiece holds the property the whole cut
// rests on: the pieces together are the population, so nothing is judged twice
// and nothing is skipped.
func TestEveryStageIsDealtIntoExactlyOnePiece(t *testing.T) {
	for _, mode := range []string{Mode, ChangedMode} {
		population := StagesForMode(mode)
		for count := 1; count <= 8; count++ {
			dealt := make([]string, 0, len(population))
			for index := 1; index <= count; index++ {
				piece, err := (Part{Index: index, Of: count}).deal(population)
				if err != nil {
					t.Fatalf("%s cut into %d, part %d: %v", mode, count, index, err)
				}
				for _, stage := range piece {
					dealt = append(dealt, stage.Identity.Name)
				}
			}
			want := make([]string, 0, len(population))
			for _, stage := range population {
				want = append(want, stage.Identity.Name)
			}
			slices.Sort(dealt)
			slices.Sort(want)
			if !slices.Equal(dealt, want) {
				t.Fatalf("%s cut into %d pieces judged %d stages, want the %d the population declares",
					mode, count, len(dealt), len(want))
			}
		}
	}
}

// TestACutRunStartsOnlyItsOwnPieceOfTheStages proves the RUN honors the deal:
// the stages a piece never starts are the ones the operator still owes.
func TestACutRunStartsOnlyItsOwnPieceOfTheStages(t *testing.T) {
	started := make([]string, 0)
	runner := func(_ context.Context, _ string, identity Identity) ActionResult {
		started = append(started, identity.Name)
		return ActionResult{Identity: identity, Registered: true, Completed: true}
	}
	report := RunPart(context.Background(), t.TempDir(), "abc", Mode, Part{Index: 2, Of: 3}, runner, Slot{})
	if report.Code != 0 {
		t.Fatalf("a green piece exited %d: %#v", report.Code, report.Failure)
	}
	want, err := (Part{Index: 2, Of: 3}).deal(StagesForMode(Mode))
	if err != nil {
		t.Fatal(err)
	}
	if len(started) != len(want) {
		t.Fatalf("the piece started %d stages, want the %d it was dealt", len(started), len(want))
	}
	for index, stage := range want {
		if started[index] != stage.Identity.Name {
			t.Fatalf("stage %d was %q, want %q", index, started[index], stage.Identity.Name)
		}
	}
	if len(started) == len(StagesForMode(Mode)) {
		t.Fatal("the piece started the whole population, so nothing was cut")
	}
}

// TestAPieceThatNamesNothingIsRefused holds the fail-closed rule: a run that
// cannot say which piece it judges judges none, rather than guessing at the
// whole population or at an empty one.
func TestAPieceThatNamesNothingIsRefused(t *testing.T) {
	for name, part := range map[string]Part{
		"the zero value":        {},
		"an index past the cut": {Index: 7, Of: 6},
		"a zero index":          {Index: 0, Of: 6},
		"a negative count":      {Index: 1, Of: -1},
	} {
		started := 0
		runner := func(_ context.Context, _ string, identity Identity) ActionResult {
			started++
			return ActionResult{Identity: identity, Registered: true, Completed: true}
		}
		report := RunPart(context.Background(), t.TempDir(), "abc", Mode, part, runner, Slot{})
		if report.Code != 2 || report.Failure == nil || report.Failure.Kind != "unknown-part" {
			t.Errorf("%s answered code %d failure %#v, want 2 and unknown-part", name, report.Code, report.Failure)
		}
		if started != 0 {
			t.Errorf("%s started %d stages, want none", name, started)
		}
	}
}

// TestACutCertificateIsNeverFresh is the guard between the two halves of this
// change: a piece exiting 0 records progress for the caller that cut the run,
// and it MUST NOT become the tree's verification certificate for anybody else.
func TestACutCertificateIsNeverFresh(t *testing.T) {
	root := statusFixture(t)
	start := job.SnapshotTree(root)
	at := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)

	whole := WriteRequest{Exit: 0, Mode: Mode, GitSHA: job.Head(root), Start: start, At: at}
	if _, err := WriteCertificate(root, whole); err != nil {
		t.Fatal(err)
	}
	if got := CheckCertificate(root, nil); !got.Fresh {
		t.Fatalf("the whole population reads %#v, so this test would pass for the wrong reason", got)
	}

	piece := whole
	piece.Mode = (Part{Index: 2, Of: 6}).Name(Mode)
	if _, err := WriteCertificate(root, piece); err != nil {
		t.Fatal(err)
	}
	if got := CheckCertificate(root, nil); got.Fresh {
		t.Errorf("a certificate written by part 2 of 6 reads fresh: %#v", got)
	}
	if got := CheckCertificate(root, []string{"changed.txt"}); got.Fresh {
		t.Errorf("a certificate written by part 2 of 6 reads fresh for a scoped path: %#v", got)
	}
}
