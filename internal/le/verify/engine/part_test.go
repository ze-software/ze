// VALIDATES: a cut verification runs each stage in exactly one piece, refuses a
// piece that names nothing, and writes a certificate no reader can take for a
// pass over the tree.
// PREVENTS: the failure that makes cutting a run worth doing at all becoming the
// failure it introduces -- a piece of a verification crediting the stages it
// never started.
package verifyengine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/job"
)

// TestEveryStageIsDealtIntoExactlyOnePiece holds the property the whole cut
// rests on: the pieces together are the population, so nothing is judged twice
// and nothing is skipped.
func TestEveryStageIsDealtIntoExactlyOnePiece(t *testing.T) {
	for _, mode := range []string{Mode, FastMode} {
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
	report := RunPart(context.Background(), t.TempDir(), "abc", Mode, Part{Index: 2, Of: 3}, Plan{}, runner, Slot{})
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
		report := RunPart(context.Background(), t.TempDir(), "abc", Mode, part, Plan{}, runner, Slot{})
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

// TestACutRunKeepsTheDealtOrder proves a cut run ignores the history: its
// piece runs in the dealt declared order even when the history would reverse it.
func TestACutRunKeepsTheDealtOrder(t *testing.T) {
	part := Part{Index: 1, Of: 2}
	dealt, err := part.deal(StagesForMode(Mode))
	if err != nil {
		t.Fatal(err)
	}
	checkout := t.TempDir()
	lines := make([]string, 0, len(dealt))
	for position, stage := range dealt {
		lines = append(lines, historyLine(stage.Identity.Name, strconv.Itoa(len(dealt)-position)))
	}
	writeHistory(t, checkout, lines...)
	started := make([]string, 0, len(dealt))
	RunPart(context.Background(), t.TempDir(), "abc", Mode, part, Plan{History: checkout}, recordingRunner("", 0, &started), Slot{})
	for position, stage := range dealt {
		if position >= len(started) || started[position] != stage.Identity.Name {
			t.Fatalf("cut run started %q, want the dealt order", started)
		}
	}
}

// TestAnIndexNamesEveryFullStageItDidNotRun proves a piece of a cut run, and a
// fast run, publish an index that names every stage of the uncut full
// population they never started as not reached. An index that lists only the
// stages its own run held let a piece replace an earlier full run's lint red,
// so the index read clean with lint never run.
//
// Method: run part 2 of 3 and the fast population with a passing runner, read
// each index back, and require every full-population stage to appear once:
// reached when the run started it, reached false otherwise. `go lint/run` is
// dealt to part 1, and `verify deps/evidence-vet` is a stage the fast
// population drops, so each run has a stage it never ran.
func TestAnIndexNamesEveryFullStageItDidNotRun(t *testing.T) {
	cases := []struct {
		name  string
		mode  string
		part  Part
		unrun string
	}{
		{name: "piece", mode: Mode, part: Part{Index: 2, Of: 3}, unrun: "go lint/run"},
		{name: "fast", mode: FastMode, part: Uncut(), unrun: "verify deps/evidence-vet"},
	}
	full := StagesForMode(Mode)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			started := make([]string, 0)
			report := RunPart(context.Background(), root, "abc", tc.mode, tc.part, Plan{}, recordingRunner("", 0, &started), Slot{})
			if report.Code != 0 {
				t.Fatalf("run = %#v", report.Failure)
			}
			index := readIndex(t, root)
			reached := map[string]bool{}
			for _, stage := range index.Stages {
				if _, twice := reached[stage.Stage]; twice {
					t.Fatalf("index names %s twice", stage.Stage)
				}
				reached[stage.Stage] = stage.Reached
			}
			for _, stage := range full {
				name := stage.Identity.Name
				was, listed := reached[name]
				if !listed {
					t.Fatalf("index does not name %s, a full-population stage this run never started", name)
				}
				if was != slices.Contains(started, name) {
					t.Fatalf("index says %s reached=%v, and the run started %v", name, was, started)
				}
			}
			if reached[tc.unrun] {
				t.Fatalf("%s reads as reached in a run that never started it", tc.unrun)
			}
		})
	}
}

// readIndex answers the failure index a run wrote under root.
func readIndex(t *testing.T, root string) artifactIndex {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(FailuresJSONPath)))
	if err != nil {
		t.Fatal(err)
	}
	var index artifactIndex
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatal(err)
	}
	return index
}
