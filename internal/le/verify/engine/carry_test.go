// VALIDATES: a run publishes, for every full-population stage it did not run,
// the verdict the checkout's previous index recorded for that stage, so a red
// stays red until a run reaches the stage again, and a stage nobody judged
// stays unjudged rather than being called red.
// PREVENTS: a piece of a cut run, or a fast run, erasing a known structural red
// by replacing the index that recorded it; and the opposite over-refusal, where
// every stage a run did not hold was charged as red.
package verifyengine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/job"
)

// carriedStage is the index entry as a reader outside this package sees it.
type carriedStage struct {
	Stage       string `json:"stage"`
	Reached     bool   `json:"reached"`
	ExitCode    int    `json:"exit-code"`
	OutsideRun  bool   `json:"outside-run"`
	CarriedFrom *struct {
		Commit string `json:"commit"`
		RunDir string `json:"run-dir"`
	} `json:"carried-from"`
}

type carriedIndex struct {
	Commit    string         `json:"commit"`
	CarryNote string         `json:"carry-note"`
	Stages    []carriedStage `json:"stages"`
}

func readCarried(t *testing.T, root string) carriedIndex {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(FailuresJSONPath)))
	if err != nil {
		t.Fatal(err)
	}
	var index carriedIndex
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatal(err)
	}
	return index
}

func carriedEntry(t *testing.T, index carriedIndex, name string) carriedStage {
	t.Helper()
	for _, stage := range index.Stages {
		if stage.Stage == name {
			return stage
		}
	}
	t.Fatalf("index does not name %s", name)
	return carriedStage{}
}

func readIndexText(t *testing.T, root string) string {
	t.Helper()
	text, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(FailuresLogPath)))
	if err != nil {
		t.Fatal(err)
	}
	return string(text)
}

// TestAPieceCarriesAnEarlierRedForAStageItDidNotRun proves a lint red a full
// run recorded survives a later piece that never ran lint, with the commit and
// run that judged it, and that a piece which does run lint green clears it.
//
// Method: run the full population red at `go lint/run` as commit aaa, then
// part 2 of 3 (lint is dealt to part 1) green as bbb, and require the lint
// entry to be the carried red from aaa. Then run part 1 green as ccc and
// require lint reached, green, and not carried.
func TestAPieceCarriesAnEarlierRedForAStageItDidNotRun(t *testing.T) {
	root := t.TempDir()
	const lint = "go lint/run"
	full := RunMode(context.Background(), root, "aaa", Mode, Plan{}, recordingRunner(lint, 1, new([]string)), Slot{})
	if full.Code == 0 {
		t.Fatal("the red full run exited 0")
	}
	piece := RunPart(context.Background(), root, "bbb", Mode, Part{Index: 2, Of: 3}, Plan{},
		recordingRunner("", 0, new([]string)), Slot{})
	if piece.Code != 0 {
		t.Fatalf("piece run = %#v", piece.Failure)
	}
	entry := carriedEntry(t, readCarried(t, root), lint)
	if !entry.Reached || entry.ExitCode != 1 || !entry.OutsideRun || entry.CarriedFrom == nil ||
		entry.CarriedFrom.Commit != "aaa" {
		t.Fatalf("lint after a piece that did not run it = %#v, want the red carried from aaa", entry)
	}
	if text := readIndexText(t, root); !strings.Contains(text, "## Stage: "+lint+" (carried: red at aaa in ") {
		t.Fatalf("the text index does not name the carried red's origin:\n%s", text)
	}
	reaching := RunPart(context.Background(), root, "ccc", Mode, Part{Index: 1, Of: 3}, Plan{},
		recordingRunner("", 0, new([]string)), Slot{})
	if reaching.Code != 0 {
		t.Fatalf("part 1 run = %#v", reaching.Failure)
	}
	entry = carriedEntry(t, readCarried(t, root), lint)
	if !entry.Reached || entry.ExitCode != 0 || entry.OutsideRun || entry.CarriedFrom != nil {
		t.Fatalf("lint after a piece that ran it green = %#v, want reached, green, not carried", entry)
	}
}

// TestAStageNeverJudgedStaysUnjudged proves a stage no run has judged is
// listed as outside the run and not reached, never as a red, and that the
// index says no earlier index existed to carry from.
//
// Method: run the fast population green over an empty checkout and require
// `verify deps/evidence-vet` outside the run, not reached, not carried.
func TestAStageNeverJudgedStaysUnjudged(t *testing.T) {
	root := t.TempDir()
	fast := RunMode(context.Background(), root, "aaa", FastMode, Plan{}, recordingRunner("", 0, new([]string)), Slot{})
	if fast.Code != 0 {
		t.Fatalf("fast run = %#v", fast.Failure)
	}
	index := readCarried(t, root)
	entry := carriedEntry(t, index, "verify deps/evidence-vet")
	if entry.Reached || !entry.OutsideRun || entry.CarriedFrom != nil {
		t.Fatalf("evidence-vet after a fast run over no earlier index = %#v, want unjudged", entry)
	}
	if !strings.Contains(index.CarryNote, "no earlier failure index") {
		t.Fatalf("carry note = %q, want it to say no earlier index existed", index.CarryNote)
	}
}

// TestAMalformedEarlierIndexIsTreatedAsEmptyAndSaidSo proves an earlier index
// another session left torn or malformed does not fail the run and is named.
func TestAMalformedEarlierIndexIsTreatedAsEmptyAndSaidSo(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(FailuresJSONPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{\"stages\": [torn"), 0o600); err != nil {
		t.Fatal(err)
	}
	piece := RunPart(context.Background(), root, "bbb", Mode, Part{Index: 2, Of: 3}, Plan{},
		recordingRunner("", 0, new([]string)), Slot{})
	if piece.Code != 0 {
		t.Fatalf("piece run over a malformed index = %#v", piece.Failure)
	}
	index := readCarried(t, root)
	if !strings.Contains(index.CarryNote, "malformed") {
		t.Fatalf("carry note = %q, want it to name the malformed earlier index", index.CarryNote)
	}
	if entry := carriedEntry(t, index, "go lint/run"); entry.Reached || entry.CarriedFrom != nil {
		t.Fatalf("lint carried from a malformed index: %#v", entry)
	}
}

// TestAFastStopCountsOnlyItsOwnPopulationAsNotReached proves the stop list in
// the text index is the fast population's unreached stages, the same count
// the progress line prints, and that the full-only stages a fast run never
// holds are listed apart as not run by this run.
//
// Method: stop a fast first-red run at its first stage, find the stop line
// and the never-judged line in the text index, and compare.
func TestAFastStopCountsOnlyItsOwnPopulationAsNotReached(t *testing.T) {
	root := t.TempDir()
	first := StagesForMode(FastMode)[0].Identity.Name
	report := RunMode(context.Background(), root, "aaa", FastMode, Plan{FirstRed: true},
		recordingRunner(first, 1, new([]string)), Slot{})
	if report.StoppedAt != first || len(report.Unreached) == 0 {
		t.Fatalf("the fast run did not stop at %s: %q", first, report.StoppedAt)
	}
	text := readIndexText(t, root)
	var stopLine, outsideLine string
	for line := range strings.SplitSeq(text, "\n") {
		if rest, found := strings.CutPrefix(line, "Not reached, the run stopped at its first red: "); found {
			stopLine = rest
		}
		if rest, found := strings.CutPrefix(line, "Not run by this run and never judged: "); found {
			outsideLine = rest
		}
	}
	if stopLine == "" {
		t.Fatalf("no stop line in the text index:\n%s", text)
	}
	if got := len(strings.Split(stopLine, ", ")); got != len(report.Unreached) {
		t.Fatalf("stop line names %d stages for %d unreached:\n%s", got, len(report.Unreached), text)
	}
	if strings.Contains(stopLine, "verify deps/evidence-vet") {
		t.Fatalf("evidence-vet, outside the fast population, is listed as stopped before:\n%s", text)
	}
	if !strings.Contains(outsideLine, "verify deps/evidence-vet") {
		t.Fatalf("evidence-vet is not listed as not run by this run:\n%s", text)
	}
}

// TestAFirstRedRunWhoseLastStageIsRedWritesAStaleCertificate proves the
// certificate a first-red run writes when its red is the last stage is stale:
// the run is an ordinary finished red run, and a red run certifies nothing.
//
// Method: write a green control and check it reads fresh, then run first-red
// with the red on the last stage over the same checkout and require stale.
func TestAFirstRedRunWhoseLastStageIsRedWritesAStaleCertificate(t *testing.T) {
	root := statusFixture(t)
	head := job.Head(root)
	green := RunMode(context.Background(), root, head, Mode, Plan{}, recordingRunner("", 0, new([]string)), Slot{})
	if green.Code != 0 {
		t.Fatalf("control run exited %d", green.Code)
	}
	if got := CheckCertificate(root, nil); !got.Fresh {
		t.Fatalf("the green control reads %#v, so this test would pass for the wrong reason", got)
	}
	population := StagesForMode(Mode)
	last := population[len(population)-1].Identity.Name
	red := RunMode(context.Background(), root, head, Mode, Plan{FirstRed: true},
		recordingRunner(last, 1, new([]string)), Slot{})
	if red.Code == 0 || red.StoppedAt != "" {
		t.Fatalf("first-red red at the last stage: code %d, stopped at %q", red.Code, red.StoppedAt)
	}
	if got := CheckCertificate(root, nil); got.Fresh {
		t.Fatalf("a first-red run red at its last stage reads fresh: %#v", got)
	}
}
