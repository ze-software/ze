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
	"testing"
)

// TestAStageCutShortByAnInterruptIsMarkedInterrupted holds that the run marks
// the stage it was running when the interrupt landed, whether the run's
// context saw it or only the runner did, and marks no stage that judged the
// tree. Method: a runner that answers a red exit 1 after canceling, one that
// refuses as interrupted, and one that answers a plain red.
func TestAStageCutShortByAnInterruptIsMarkedInterrupted(t *testing.T) {
	for name, tc := range map[string]struct {
		cancel bool
		result func(Identity) ActionResult
		want   bool
	}{
		"context canceled": {cancel: true, want: true, result: func(identity Identity) ActionResult {
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
			for _, later := range report.Stages[1:] {
				if later.Interrupted {
					t.Fatalf("a stage after the first was marked interrupted: %#v", later)
				}
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
