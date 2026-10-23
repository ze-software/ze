package verifyengine

// VALIDATES: a stage an interrupt cut short is recorded as interrupted by the
// run and written to the failure index with no verdict, and the next run
// never carries such an entry as a red.
// PREVENTS: an interrupted stage's exit status (130, or whatever its process
// died with) reaching the index as a reached red with a generic group, which
// the commit gate then charged against every commit carrying Go.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestPublishWritesTheCurrentFormatOverALegacySource holds the worktree
// publish path: the index it writes into the checkout carries the current
// format whatever format the worktree's index had, a legacy exit-130 entry of
// the source is written as interrupted and never judged, and a genuine
// exit-130 red the checkout's current-format index recorded stays a red.
// Method: publish a format-less source holding one stage at exit 130 over a
// checkout whose current-format index holds another stage's exit-130 red, then
// read both stages back.
func TestPublishWritesTheCurrentFormatOverALegacySource(t *testing.T) {
	stages := fullStages()
	cutName, redName := stages[0].Identity.Name, stages[1].Identity.Name
	source, target := t.TempDir(), t.TempDir()
	writeIndexFixture(t, source, `{"commit":"new","run-dir":"tmp/verify","stages":[
		{"stage":"`+cutName+`","reached":true,"exit-code":130,
		"groups":[{"stage":"`+cutName+`","group-id":"stage:x","kind":"generic","related":[],
		"summary":"context canceled","rerun":"le cut rerun"}]}]}`)
	writeIndexFixture(t, target, `{"format":`+strconv.Itoa(FailureIndexFormat)+
		`,"commit":"old","run-dir":"tmp/old","stages":[
		{"stage":"`+redName+`","reached":true,"exit-code":130}]}`)

	if err := PublishFailureIndex(source, target, ""); err != nil {
		t.Fatalf("publish: %v", err)
	}
	index := readIndexFixture(t, target)
	if index.Format != FailureIndexFormat {
		t.Fatalf("published index format = %d, want %d: its carried exit-130 red would read as an interrupt",
			index.Format, FailureIndexFormat)
	}
	for _, stage := range index.Stages {
		switch stage.Stage {
		case cutName:
			if !stage.Interrupted || stage.Reached || stage.ExitCode != 0 || len(stage.Groups) != 0 ||
				stage.Rerun != "le cut rerun" {
				t.Errorf("legacy exit-130 entry = %#v, want interrupted, never judged, rerun le cut rerun", stage)
			}
		case redName:
			if stage.Interrupted || stage.ExitCode != Interrupted {
				t.Errorf("carried current-format red = %#v, want exit %d kept", stage, Interrupted)
			}
		}
	}
}

// writeIndexFixture writes body as the failure index under root.
func writeIndexFixture(t *testing.T, root, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(FailuresJSONPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// readIndexFixture reads back the failure index under root.
func readIndexFixture(t *testing.T, root string) artifactIndex {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(FailuresJSONPath)))
	if err != nil {
		t.Fatalf("read the failure index: %v", err)
	}
	var index artifactIndex
	if err := json.Unmarshal(body, &index); err != nil {
		t.Fatalf("parse the failure index: %v", err)
	}
	return index
}

// TestAnInterruptedStageOutsideTheFullPopulationIsKept holds that carryForward
// never drops an interrupted stage the full population does not name: it is
// still written once, as interrupted. Method: carry one such stage over an
// empty earlier index.
func TestAnInterruptedStageOutsideTheFullPopulationIsKept(t *testing.T) {
	cut := artifactStage{Stage: "not in any population", OutsideRun: true, Interrupted: true, Rerun: "le x"}
	found := 0
	for _, stage := range carryForward([]artifactStage{cut}, artifactIndex{}) {
		if stage.Stage == cut.Stage {
			found++
			if !stage.Interrupted || stage.Reached || stage.ExitCode != 0 {
				t.Fatalf("stage = %#v, want it written as interrupted", stage)
			}
		}
	}
	if found != 1 {
		t.Fatalf("the interrupted stage appears %d times, want once", found)
	}
}

// TestAStageCutShortByAnInterruptIsMarkedInterrupted holds that the run marks
// a stage interrupted only when the stage's own result says the interrupt cut
// it short, and never a stage that answered: a red that returned before the
// run's context was canceled stays a red. Method: a runner that answers a red
// exit 1 and then cancels, one that refuses as interrupted, and one that
// answers a plain red.
func TestAStageCutShortByAnInterruptIsMarkedInterrupted(t *testing.T) {
	for name, tc := range map[string]struct {
		cancel bool
		result func(Identity) ActionResult
		want   bool
	}{
		// The stage answered before the interrupt landed, so its red is a
		// verdict: only the stage's own result can say it was cut short.
		"canceled after the stage answered": {cancel: true, want: false, result: func(identity Identity) ActionResult {
			return ActionResult{Identity: identity, Registered: true, Completed: true, Code: 1}
		}},
		"runner refused": {want: true, result: func(identity Identity) ActionResult {
			return ActionResult{Identity: identity, Registered: true, Code: Interrupted,
				Failure: &Failure{Kind: "interrupted", Stage: identity.Name, Message: "interrupted"}}
		}},
		"plain red": {want: false, result: func(identity Identity) ActionResult {
			return ActionResult{Identity: identity, Registered: true, Completed: true, Code: 1}
		}},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			runner := func(_ context.Context, _ string, identity Identity) ActionResult {
				if tc.cancel {
					cancel()
				}
				return tc.result(identity)
			}
			report := RunMode(ctx, t.TempDir(), "abc", Mode, Plan{}, runner, Slot{})
			if len(report.Stages) == 0 {
				t.Fatalf("no stage ran: %#v", report)
			}
			if got := report.Stages[0].Interrupted; got != tc.want {
				t.Fatalf("first stage Interrupted = %v, want %v (%#v)", got, tc.want, report.Stages[0])
			}
			if tc.cancel && report.Stages[0].Code != 1 {
				t.Fatalf("the red the stage answered was lost: %#v", report.Stages[0])
			}
			for _, later := range report.Stages[1:] {
				if later.Interrupted {
					t.Fatalf("a stage after the first was marked interrupted: %#v", later)
				}
			}
		})
	}
}

// TestAStageThatExited130OnTheInterruptIsMarkedInterrupted holds the window
// before the cancel: Ctrl-C reaches the whole foreground process group, so a
// stage that handles SIGINT itself exits 130 before exec cancels it, and the
// runner reads a completed exit 130. With the run's context done by then, that
// 130 is the interrupt status, not a verdict, so the stage is marked
// interrupted. A 130 with no interrupt stays the stage's own red. Method: a
// runner that answers a completed 130, once canceling the run and once not.
func TestAStageThatExited130OnTheInterruptIsMarkedInterrupted(t *testing.T) {
	for name, tc := range map[string]struct {
		cancel bool
		want   bool
	}{
		"interrupt landed": {cancel: true, want: true},
		"no interrupt":     {cancel: false, want: false},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			runner := func(_ context.Context, _ string, identity Identity) ActionResult {
				if tc.cancel {
					cancel()
				}
				return ActionResult{Identity: identity, Registered: true, Completed: true, Code: Interrupted}
			}
			report := RunMode(ctx, t.TempDir(), "abc", Mode, Plan{}, runner, Slot{})
			if len(report.Stages) == 0 {
				t.Fatalf("no stage ran: %#v", report)
			}
			if got := report.Stages[0].Interrupted; got != tc.want {
				t.Fatalf("first stage Interrupted = %v, want %v (%#v)", got, tc.want, report.Stages[0])
			}
			if tc.want && report.Code != Interrupted {
				t.Fatalf("run code = %d, want %d", report.Code, Interrupted)
			}
		})
	}
}

// TestAnInterruptedStageIsWrittenAsNeverJudged holds the writer: the stage an
// interrupt cut short is written not reached, with no exit status, no group,
// the interrupted marker and its rerun command, so no reader can take it for
// a verdict. An earlier verdict for the stage carries as for any stage the
// run did not finish, and an earlier entry that is itself an interrupted
// stage, in either spelling, is never carried as a red. Method: write one
// interrupted stage over no index, over an earlier red, and over an earlier
// exit-130 entry, and read the stage back.
func TestAnInterruptedStageIsWrittenAsNeverJudged(t *testing.T) {
	identity := fullStages()[0].Identity
	name := identity.Name
	for label, tc := range map[string]struct {
		earlier  string
		wantExit int
		wantMark bool
	}{
		"no earlier index": {wantMark: true},
		"earlier red": {wantExit: 1, earlier: `{"commit":"old","run-dir":"tmp/old","stages":[
			{"stage":"` + name + `","reached":true,"exit-code":1}]}`},
		"earlier exit 130": {wantMark: true, earlier: `{"commit":"old","run-dir":"tmp/old","stages":[
			{"stage":"` + name + `","reached":true,"exit-code":130}]}`},
		// The current writer never gives an interrupted stage an exit, so 130
		// in an index it wrote is the stage's own red, and it carries.
		"earlier current-format exit 130": {wantExit: Interrupted, earlier: `{"format":` +
			strconv.Itoa(FailureIndexFormat) + `,"commit":"old","run-dir":"tmp/old","stages":[
			{"stage":"` + name + `","reached":true,"exit-code":130}]}`},
		"earlier marked": {wantMark: true, earlier: `{"commit":"old","run-dir":"tmp/old","stages":[
			{"stage":"` + name + `","reached":false,"interrupted":true,"outside-run":true}]}`},
	} {
		t.Run(label, func(t *testing.T) {
			root := t.TempDir()
			if tc.earlier != "" {
				path := filepath.Join(root, filepath.FromSlash(FailuresJSONPath))
				if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(tc.earlier), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			report := Report{
				Mode: Mode, Code: Interrupted, LogDir: filepath.ToSlash(filepath.Join("tmp", "verify")),
				Stages: []StageReport{{
					Identity: identity, Code: Interrupted, Log: "tmp/verify/01.log", Interrupted: true,
					Failure: &Failure{Kind: "interrupted", Stage: name, Message: "context canceled"},
				}},
			}
			if err := writeRunArtifacts(root, report, testArtifactTime()); err != nil {
				t.Fatalf("write the run artifacts: %v", err)
			}
			body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(FailuresJSONPath)))
			if err != nil {
				t.Fatalf("read the failure index: %v", err)
			}
			var index artifactIndex
			if err := json.Unmarshal(body, &index); err != nil {
				t.Fatalf("parse the failure index: %v", err)
			}
			if index.Format != FailureIndexFormat {
				t.Fatalf("index format = %d, want %d", index.Format, FailureIndexFormat)
			}
			found := 0
			for _, stage := range index.Stages {
				if stage.Stage != name {
					continue
				}
				found++
				if stage.ExitCode != tc.wantExit || stage.Interrupted != tc.wantMark {
					t.Fatalf("stage = %#v, want exit %d interrupted %v", stage, tc.wantExit, tc.wantMark)
				}
				if !tc.wantMark {
					continue
				}
				if stage.Reached || len(stage.Groups) != 0 || stage.Rerun != "le "+invocation(identity) {
					t.Fatalf("an interrupted stage carries a verdict or no rerun: %#v", stage)
				}
			}
			if found != 1 {
				t.Fatalf("stage %s appears %d times in the index:\n%s", name, found, body)
			}
		})
	}
}
