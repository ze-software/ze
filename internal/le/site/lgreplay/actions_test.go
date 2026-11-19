package sitelgreplay

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestEachFailedStageNamesItsOwnFix proves a run that fails names the part
// that failed and a fix for that part, and that no part answers the fix of
// another: naming another ze does not free a port or bring a session up.
//
// Method: stageFailure for every stage a run can fail at, with one cause.
func TestEachFailedStageNamesItsOwnFix(t *testing.T) {
	cause := errors.New("the cause")
	cases := []struct {
		stage   recordStage
		problem string
		fix     string
	}{
		{recordStageWorkDirectory, "cannot create its work directory", "TMPDIR=<writable-directory> " + RecorderCommand},
		{recordStagePorts, "free ports from port 28470", RecorderCommand + " port <first-free-port>"},
		{recordStageNetwork, "did not pass its checks against /opt/ze", "./le test fixture plugin/lg-simulated-network 28470 28471"},
		{recordStageSubjects, "no route the tour can show", RecorderCommand},
		{recordStageTour, "did not answer the whole tour", RecorderCommand},
	}
	for _, tc := range cases {
		failure := stageFailure(tc.stage, cause, "/opt/ze", 28470)
		if !strings.Contains(failure.Problem(), tc.problem) {
			t.Errorf("stage %d: problem %q does not say %q", tc.stage, failure.Problem(), tc.problem)
		}
		if failure.Fix() != tc.fix {
			t.Errorf("stage %d: fix %q, want %q", tc.stage, failure.Fix(), tc.fix)
		}
		if strings.Contains(failure.Fix(), keywordBinary) {
			t.Errorf("stage %d: the fix names another ze, which does not fix this failure: %q", tc.stage, failure.Fix())
		}
		if !errors.Is(failure, cause) {
			t.Errorf("stage %d: the failure lost its cause", tc.stage)
		}
		rendered := failure.Error()
		if !strings.HasSuffix(rendered, "fix: "+tc.fix) {
			t.Errorf("stage %d: the fix is not the last line:\n%s", tc.stage, rendered)
		}
	}
}

// TestAStoppedRunSaysItWasStopped proves a run whose context ended answers the
// stop, and a run whose context is alive answers nothing here, so the stage's
// own failure is reported.
func TestAStoppedRunSaysItWasStopped(t *testing.T) {
	if ended := endedRun(context.Background()); ended != nil {
		t.Fatalf("a live run was reported as stopped: %v", ended)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ended := endedRun(ctx)
	if ended == nil {
		t.Fatal("a stopped run was not reported")
	}
	if !strings.Contains(ended.Problem(), "stopped before it finished") {
		t.Fatalf("problem: %q", ended.Problem())
	}
	if !errors.Is(ended, context.Canceled) {
		t.Fatalf("the stop lost its cause: %v", ended)
	}
}

// TestUncommittedChangesAreRefusedUnlessTrial proves the recorder refuses a
// checkout with uncommitted changes, names the changes and the trial keyword,
// records a clean checkout, and records a dirty one when the line says trial.
func TestUncommittedChangesAreRefusedUnlessTrial(t *testing.T) {
	clean := checkout{provenance: Provenance{Commit: "0123456789abcdef0123"}}
	if err := refuseUncommitted(clean, false); err != nil {
		t.Fatalf("a clean checkout was refused: %v", err)
	}

	dirty := checkout{
		provenance:  Provenance{Commit: "0123456789abcdef0123", TreeDirty: true},
		uncommitted: []string{"M a.go", "M b.go", "M c.go", "M d.go", "M e.go", "?? f.go", "?? g.go"},
	}
	if err := refuseUncommitted(dirty, true); err != nil {
		t.Fatalf("a trial of a dirty checkout was refused: %v", err)
	}
	err := refuseUncommitted(dirty, false)
	if err == nil {
		t.Fatal("a checkout with uncommitted changes was recorded without the trial keyword")
	}
	var refusal *opererr.Error
	if !errors.As(err, &refusal) {
		t.Fatalf("the refusal is no operator error: %v", err)
	}
	for _, want := range []string{"7 uncommitted changes", "M a.go; M b.go; M c.go; M d.go; M e.go, and 2 more"} {
		if !strings.Contains(refusal.Problem(), want) {
			t.Errorf("the refusal does not say %q: %s", want, refusal.Problem())
		}
	}
	if strings.Contains(refusal.Problem(), "f.go") {
		t.Errorf("the refusal names more than %d paths: %s", uncommittedShown, refusal.Problem())
	}
	if !strings.Contains(refusal.Why(), "commit 0123456789 ") {
		t.Errorf("the refusal does not name the commit: %s", refusal.Why())
	}
	// The refusal says what the trial it offers is good for, and where it
	// stops: the site check refuses to publish it.
	for _, want := range []string{"look at the replay locally", "./le site check refuses to publish it"} {
		if !strings.Contains(refusal.Why(), want) {
			t.Errorf("the refusal does not say %q: %s", want, refusal.Why())
		}
	}
	if refusal.Fix() != RecorderCommand+" trial" {
		t.Errorf("fix %q, want the trial command", refusal.Fix())
	}
}

// TestRecordDeclaresTheTrialKeyword proves the action declares trial as a
// parameter, which is what puts it in the action's help and completion, that
// the help says what a trial is for and that the site check refuses it, and
// that the parameter's help line fits the launcher's 60 characters.
func TestRecordDeclaresTheTrialKeyword(t *testing.T) {
	for _, action := range Actions().Actions {
		if action.Verb != "record" {
			continue
		}
		for _, parameter := range action.Parameters {
			if parameter.Keyword == keywordTrial {
				for _, want := range []string{"trial keyword", "look at the replay locally", "./le site check refuses"} {
					if !strings.Contains(action.Why, want) {
						t.Fatalf("the action's help does not say %q:\n%s", want, action.Why)
					}
				}
				if !strings.Contains(parameter.Help, "site check refuses") {
					t.Errorf("the trial keyword's help line does not say the site check refuses it: %s", parameter.Help)
				}
				if len(parameter.Help) > 60 {
					t.Errorf("the trial keyword's help line holds %d characters, and the launcher takes 60: %s", len(parameter.Help), parameter.Help)
				}
				return
			}
		}
		t.Fatal("the record action declares no trial keyword")
	}
	t.Fatal("the area holds no record action")
}
