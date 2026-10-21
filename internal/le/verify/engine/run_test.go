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
	report := runMode(context.Background(), root, "abc", Mode, Part{Index: 1, Of: 1}, Plan{}, twoStageRunner(0, 0), Slot{}, clock)
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

// recordingRunner answers code for the stage named red and zero for every other
// stage, and records the order the engine started them in.
func recordingRunner(red string, code int, started *[]string) ActionRunner {
	return func(_ context.Context, _ string, identity Identity) ActionResult {
		*started = append(*started, identity.Name)
		result := ActionResult{Identity: identity, Registered: true, Completed: true}
		if identity.Name == red {
			result.Code = code
		}
		return result
	}
}

// writeHistory writes lines as the checkout's stage-time history.
func writeHistory(t *testing.T, checkout string, lines ...string) {
	t.Helper()
	path := filepath.Join(checkout, filepath.FromSlash(StageTimesPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func historyLine(stage, seconds string) string {
	return stage + "\t" + seconds + "\t0\tfull\tabc\t2026-10-21T03:00:00Z"
}

// TestAnUncutRunRunsCheapestRecordedStageFirst proves the cost order of AC-9:
// every stage with no recorded time runs first in declared order, then the
// recorded stages by ascending latest seconds, a tie keeping declared order.
//
// Method: a history records the last three declared stages (the slowest, then
// two tied, one of them recorded twice so only its latest line counts) and a
// name the population does not hold. The run must start the unrecorded stages
// in declared order and then the three in cost order, and must not start the
// stranger.
func TestAnUncutRunRunsCheapestRecordedStageFirst(t *testing.T) {
	population := StagesForMode(Mode)
	count := len(population)
	last, beforeLast, third := population[count-1].Identity.Name,
		population[count-2].Identity.Name, population[count-3].Identity.Name
	checkout := t.TempDir()
	writeHistory(t, checkout,
		historyLine(third, "1.000"),
		historyLine(last, "900.000"),
		historyLine(third, "5.000"),
		historyLine(beforeLast, "5.000"),
		historyLine("no such stage", "0.001"),
	)
	started := make([]string, 0, count)
	report := RunMode(context.Background(), t.TempDir(), "abc", Mode,
		Plan{History: checkout}, recordingRunner("", 0, &started), Slot{})
	if report.Code != 0 {
		t.Fatalf("a green run exited %d: %#v", report.Code, report.Failure)
	}
	want := make([]string, 0, count)
	for _, stage := range population[:count-3] {
		want = append(want, stage.Identity.Name)
	}
	want = append(want, third, beforeLast, last)
	if strings.Join(started, "|") != strings.Join(want, "|") {
		t.Fatalf("started %q\nwant    %q", started, want)
	}
}

// TestFirstRedStopsAndRecordsEveryUnreachedStage proves AC-10: with first-red
// no stage starts after the first red, the run answers that stage's code, is
// not complete, and the report and the failure index name the stop and every
// stage the run never reached.
//
// Method: the third declared stage answers Unjudged-earning code 3 in a run
// with no history (declared order), so exactly three stages start and the rest
// of the population is unreached.
func TestFirstRedStopsAndRecordsEveryUnreachedStage(t *testing.T) {
	population := StagesForMode(Mode)
	red := population[2].Identity.Name
	root := t.TempDir()
	started := make([]string, 0)
	report := RunMode(context.Background(), root, "abc", Mode,
		Plan{FirstRed: true}, recordingRunner(red, stageUnjudged, &started), Slot{})
	if len(started) != 3 {
		t.Fatalf("started %d stages %q, want 3: no stage after the red may start", len(started), started)
	}
	if report.Code != Unjudged {
		t.Fatalf("a stop at an unjudged stage exited %d, want %d", report.Code, Unjudged)
	}
	if report.Completed {
		t.Fatal("a stopped run reads as completed")
	}
	if report.StoppedAt != red {
		t.Fatalf("stopped-at %q, want %q", report.StoppedAt, red)
	}
	if len(report.Unreached) != len(population)-3 || report.Unreached[0].Name != population[3].Identity.Name {
		t.Fatalf("unreached %d stages starting %v, want %d starting %s",
			len(report.Unreached), report.Unreached[:min(1, len(report.Unreached))], len(population)-3, population[3].Identity.Name)
	}
	if !strings.Contains(report.Console, StoppedMarker+red+", "+fmt.Sprint(len(population)-3)+" stages not run") {
		t.Fatalf("combined log carries no stop line:\n%s", report.Console)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(FailuresJSONPath)))
	if err != nil {
		t.Fatal(err)
	}
	var index artifactIndex
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatal(err)
	}
	if index.StoppedAt != red || len(index.Stages) != len(population) {
		t.Fatalf("index stopped-at %q with %d stages, want %q with the whole population %d",
			index.StoppedAt, len(index.Stages), red, len(population))
	}
	for position, stage := range index.Stages {
		if stage.Reached != (position < 3) {
			t.Fatalf("index stage %d %s reached=%v", position, stage.Stage, stage.Reached)
		}
	}
	if !strings.Contains(string(data), `"reached": false`) {
		t.Fatal("the index spells no unreached stage as reached: false")
	}
}

// TestARunWithoutFirstRedStillRunsPastARed keeps the documented contract for
// a run that did not opt in: a red stage does not hide later reds.
func TestARunWithoutFirstRedStillRunsPastARed(t *testing.T) {
	population := StagesForMode(Mode)
	started := make([]string, 0)
	report := RunMode(context.Background(), t.TempDir(), "abc", Mode,
		Plan{}, recordingRunner(population[0].Identity.Name, 1, &started), Slot{})
	if len(started) != len(population) || report.StoppedAt != "" || len(report.Unreached) != 0 {
		t.Fatalf("a plain run started %d of %d stages, stopped-at %q", len(started), len(population), report.StoppedAt)
	}
}

// TestAFirstRedAtTheLastStageIsNoStop proves a first-red run whose red is its
// last stage reports an ordinary finished run. Nothing was left to stop
// before, so a stop with no unreached stage made every surface say pending
// stages "will never report" or "runs them all" about stages that do not exist.
//
// Method: run the full population with first-red and the red on the last
// stage, and require no StoppedAt, no unreached stage, a completed run, the
// red's exit, and no stop line in the combined log.
func TestAFirstRedAtTheLastStageIsNoStop(t *testing.T) {
	population := StagesForMode(Mode)
	last := population[len(population)-1].Identity.Name
	started := make([]string, 0)
	report := RunMode(context.Background(), t.TempDir(), "abc", Mode,
		Plan{FirstRed: true}, recordingRunner(last, 1, &started), Slot{})
	if len(started) != len(population) || report.Code != 1 || !report.Completed {
		t.Fatalf("started %d of %d, code %d, completed %v", len(started), len(population), report.Code, report.Completed)
	}
	if report.StoppedAt != "" || len(report.Unreached) != 0 {
		t.Fatalf("a red at the last stage reads as a stop at %q with %d unreached", report.StoppedAt, len(report.Unreached))
	}
	if strings.Contains(report.Console, StoppedMarker) {
		t.Fatalf("the combined log carries a stop line:\n%s", report.Console)
	}
}
