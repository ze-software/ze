// VALIDATES: a stage that could not judge its subject keeps that outcome
// through the run, a stage that judged the tree and found it wrong outranks it,
// and a write failure separates a full device from a run that broke.
// PREVENTS: a run that cleared nothing answering with the exit code a red
// answers, which is the false verdict plan/journal/full-disk-false-red.md
// records seven times.
package verifyengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// twoStageRunner answers first for stage 0, second for stage 1, and zero for
// every stage after them.
func twoStageRunner(first, second int) ActionRunner {
	call := 0
	return func(_ context.Context, _ string, identity Identity) ActionResult {
		result := ActionResult{Identity: identity, Registered: true, Completed: true}
		switch call {
		case 0:
			result.Code = first
		case 1:
			result.Code = second
		}
		call++
		return result
	}
}

func TestAStageThatCouldNotJudgeIsNotFlattenedToFailed(t *testing.T) {
	for _, test := range []struct {
		name  string
		stage int
		want  int
	}{
		{name: "every stage green", stage: 0, want: 0},
		{name: "a stage could not judge", stage: stageUnjudged, want: Unjudged},
		{name: "a stage judged and failed", stage: 1, want: 1},
		{name: "any other stage status", stage: 37, want: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			report := run(context.Background(), t.TempDir(), "abc",
				twoStageRunner(0, test.stage), time.Now)
			if report.Code != test.want {
				t.Fatalf("run status = %d, want %d for a stage that exited %d",
					report.Code, test.want, test.stage)
			}
			if report.Stages[1].Code != test.stage {
				t.Fatalf("stage status = %d, want the %d the action answered",
					report.Stages[1].Code, test.stage)
			}
		})
	}
}

// TestAStageThatJudgedTheTreeOutranksOneThatCouldNot drives BOTH orders. The
// red-first order is what holds the precedence guard in runCode: without it a
// later stage that could not judge would demote a red the run had already
// reached, which is unjudged outranking a verdict.
func TestAStageThatJudgedTheTreeOutranksOneThatCouldNot(t *testing.T) {
	for _, test := range []struct {
		name          string
		first, second int
	}{
		{name: "could not judge, then judged and failed", first: stageUnjudged, second: 9},
		{name: "judged and failed, then could not judge", first: 9, second: stageUnjudged},
	} {
		t.Run(test.name, func(t *testing.T) {
			report := run(context.Background(), t.TempDir(), "abc",
				twoStageRunner(test.first, test.second), time.Now)
			if report.Code != 1 {
				t.Fatalf("run status = %d, want 1: a stage that found the tree wrong judged it", report.Code)
			}
		})
	}
}

func TestAFullDeviceIsUnjudgedAndAnyOtherWriteFailureIsABrokenRun(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "a full device", err: syscall.ENOSPC, want: Unjudged},
		{name: "a full device under a wrapper", err: fmt.Errorf("copy stage logs: %w", syscall.ENOSPC), want: Unjudged},
		{name: "a permission refusal", err: syscall.EACCES, want: 2},
		{name: "a short write", err: io.ErrShortWrite, want: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := brokenCode(test.err); got != test.want {
				t.Fatalf("brokenCode(%v) = %d, want %d", test.err, got, test.want)
			}
			if Defeated(test.err) != (test.want == Unjudged) {
				t.Fatalf("Defeated(%v) = %v", test.err, Defeated(test.err))
			}
		})
	}
	if Defeated(nil) || Defeated(errors.New("no space left on device")) {
		t.Fatal("a full device is recognized by its typed error, never by its text")
	}
}

// TestEveryStageLogCarriesItsTimingBeforeItsResult proves each stage records
// when it started and how long it ran, on its own line before an unchanged
// result line, in the stage log, the report and the failure index.
//
// Method: a clock that advances 1.5s on every read drives a whole run of the
// full population, so every stage's duration is exactly 1.5s. The result line
// must stay the LAST line, spelled as before, because stageResult and three
// fixtures read it; the timing line therefore goes before it.
func TestEveryStageLogCarriesItsTimingBeforeItsResult(t *testing.T) {
	root := t.TempDir()
	base := time.Date(2026, 10, 21, 3, 0, 0, 0, time.UTC)
	reads := 0
	clock := func() time.Time {
		at := base.Add(time.Duration(reads) * 1500 * time.Millisecond)
		reads++
		return at
	}
	report := runMode(context.Background(), root, "abc", Mode, Part{Index: 1, Of: 1}, twoStageRunner(0, 0), Slot{}, clock)
	if len(report.Stages) != len(StagesForMode(Mode)) {
		t.Fatalf("ran %d stages, want the whole population %d", len(report.Stages), len(StagesForMode(Mode)))
	}
	for _, stage := range report.Stages {
		if stage.Seconds != 1.5 || stage.Start.IsZero() {
			t.Fatalf("stage %s start=%v seconds=%v, want a start and 1.5s", stage.Identity.Name, stage.Start, stage.Seconds)
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(stage.Log)))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
		timing := "### Stage timing: " + stage.Identity.Name + " start=" +
			stage.Start.UTC().Format(time.RFC3339) + " seconds=1.500"
		result := "### Stage result: " + stage.Identity.Name + " exit=0"
		if len(lines) < 2 || lines[len(lines)-2] != timing || lines[len(lines)-1] != result {
			t.Fatalf("stage log ends %q, want %q then %q", lines[max(0, len(lines)-2):], timing, result)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(FailuresJSONPath)))
	if err != nil {
		t.Fatal(err)
	}
	var index artifactIndex
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatal(err)
	}
	for _, stage := range index.Stages {
		if stage.Seconds != 1.5 || stage.Start == "" {
			t.Fatalf("index stage %s start=%q seconds=%v", stage.Stage, stage.Start, stage.Seconds)
		}
	}
}
