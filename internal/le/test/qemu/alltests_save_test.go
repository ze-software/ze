// Design: docs/architecture/testing/qemu-integration.md -- full selected-CI output.

package testqemu

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSelectedCISaveActionGrammar(t *testing.T) {
	args, err := actions.Parse("all", []string{"test", "test/draft/plugin/selected.ci", "save", "tmp/selected-output"})
	if err != nil {
		t.Fatalf("registered selected-CI output grammar refused: %v", err)
	}
	if got := args.One("save"); got != "tmp/selected-output" {
		t.Fatalf("save directory %q, want the caller's exact path", got)
	}
}

func TestSelectedCISaveReachesExistingRunner(t *testing.T) {
	run := vmFixture(t)
	run.Test = "test/draft/plugin/selected-save.ci"
	run.SaveDir = filepath.Join(run.Workspace, "output with spaces")
	writeFile(t, run.Workspace, run.Test, "option=netns-link:name=zesrproof\ncmd=foreground:seq=1:exec=ze doctor\n")
	rec := &recorder{}
	run.Run = rec.run
	if _, code := run.Execute(); code != 0 {
		t.Fatalf("selected CI failed: exit %d", code)
	}
	if len(rec.calls) != 3 {
		t.Fatalf("selected CI emitted %d commands, want namespace, conntrack and one runner", len(rec.calls))
	}
	argv := rec.calls[2]
	saveAt := slices.Index(argv, "--save")
	if saveAt < 0 {
		t.Fatalf("runner %q lost the requested complete output directory", argv)
	}
	if saveAt+1 >= len(argv) {
		t.Fatalf("runner %q has no --save value", argv)
	}
	if argv[saveAt+1] != run.SaveDir {
		t.Errorf("runner output directory %q, want exact %q", argv[saveAt+1], run.SaveDir)
	}
	if argv[len(argv)-1] != "selected-save" {
		t.Errorf("runner selection %q, want the same single CI last", argv)
	}
	if !slices.Contains(argv, "--draft") {
		t.Errorf("runner %q lost draft discovery", argv)
	}
}

func TestSelectedCISaveRefusesOtherPopulations(t *testing.T) {
	for _, population := range []string{"whole suite", "stress"} {
		t.Run(population, func(t *testing.T) {
			run := vmFixture(t)
			run.SaveDir = "tmp/selected-output"
			if population == "stress" {
				run.Test = "test/plugin/selected.ci"
				run.Stress = &selectedStress{iterations: 2, parallel: 1}
			}
			err := run.verify()
			if err == nil {
				t.Fatalf("selected-CI save accepted %s execution", population)
			}
			if !strings.Contains(err.Error(), "save") {
				t.Fatalf("save refusal does not explain the requested output: %v", err)
			}
		})
	}
}
