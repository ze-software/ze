package interoplab

import (
	"bytes"
	"context"
	"errors"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"
)

// phaseLabRoot is the scenario root, relative to phaseLabCheckout, that the
// test catalog declares, so Suite.rerunCommand finds the catalog's action.
const (
	phaseLabCheckout = "/checkout"
	phaseLabRoot     = "test/phase-lab"
	phaseLabSuite    = "phase-test-lab"
	phaseLabAction   = "test integration/interop-phase-test"
)

// phaseRunner answers the Docker kernel check, answers every docker exec
// with ready on stdout, so a readiness probe that wants "ready" passes and
// one that wants anything else waits until its deadline, and answers a
// container's consumed image with phaseLabImage.
func phaseRunner() *recordingRunner {
	return &recordingRunner{run: kernelAnswering(func(command ProcessCommand) (ProcessResult, error) {
		if slices.Contains(command.Arguments, dockerExecVerb) {
			return ProcessResult{Stdout: "ready\n"}, nil
		}
		if slices.Contains(command.Arguments, "{{.Image}}") {
			return ProcessResult{Stdout: phaseLabImage + "\n"}, nil
		}
		return ProcessResult{}, nil
	})}
}

// phaseLabImage is the image ID the scripted runner answers for every
// container's `docker inspect --format {{.Image}}`, the call Lab.PeerImage makes.
const phaseLabImage = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// phasePlan is one scenario under the test catalog's root whose only peer
// starts with no readiness probe.
func phasePlan(name string, checker Checker) ScenarioPlan {
	return ScenarioPlan{
		Source: ScenarioSource{Name: name, Directory: phaseLabCheckout + "/" + phaseLabRoot + "/" + name,
			Checker: checker},
		Network: NetworkSpec{Name: "lab-net", Candidates: []Subnet{{IPv4: netip.MustParsePrefix("172.29.0.0/24")}}},
		Peers:   []PeerConfig{{Name: "ze", Container: "lab-ze", Image: "ze-image", Host: 2}},
	}
}

// readyProbe waits for want on the probe's stdout, briefly, so a probe whose
// want the runner never prints fails in milliseconds.
func readyProbe(want string) *ReadyProbe {
	return &ReadyProbe{Command: []string{"cat", "/ready"}, Contains: []string{want},
		Timeout: 50 * time.Millisecond, Interval: 10 * time.Millisecond}
}

// failingIn makes plan fail in phase: setup through its Prepare, readiness
// through a probe the runner never satisfies, checker through its checker.
func failingIn(t *testing.T, plan ScenarioPlan, phase ScenarioPhase, cause string) ScenarioPlan {
	t.Helper()
	switch phase {
	case PhaseSetup:
		peers := plan.Peers
		plan.Peers = nil
		plan.Containers = []string{"lab-ze"}
		plan.Prepare = func(context.Context, PrepareContext) (PreparedScenario, error) {
			return PreparedScenario{Peers: peers}, errors.New(cause)
		}
	case PhaseReadiness:
		plan.Peers[0].Ready = readyProbe("never printed")
	case PhaseChecker:
		plan.Source.Checker = func(context.Context, *CheckContext) error { return errors.New(cause) }
	}
	return plan
}

func passing(context.Context, *CheckContext) error { return nil }

// phaseSuite is a suite over plans whose progress goes to progress.
func phaseSuite(progress *bytes.Buffer, plans ...ScenarioPlan) Suite {
	return Suite{
		Docker:    newDocker(phaseRunner()),
		StagedZe:  kernelCheckZe,
		NoBuild:   true,
		Progress:  progress,
		Checkout:  phaseLabCheckout,
		Scenarios: plans,
	}
}

// TestScenarioFailureNamesItsPhase drives one failing scenario per phase
// through Suite.Run, the entry point every interop lab reaches, with a
// scripted Docker runner.
// VALIDATES: AC-1. A failure before the checker reads `FAIL (<elapsed>) in
// setup` or `in readiness` on its progress line, in its result's phase and in
// the report text; a checker failure reads `in checker` and keeps its
// `scenario <name> assertion N:` text whole; a container run after a passed
// readiness wait is setup again, not readiness; a pass line is unchanged.
// PREVENTS: a lab whose helper could not start reading as the feature's red,
// as four scenarios did on 2026-11-09 (c77a8dd2f3).
func TestScenarioFailureNamesItsPhase(t *testing.T) {
	afterReadiness := phasePlan("run-after-ready", passing)
	afterReadiness.Peers = []PeerConfig{
		{Name: "speaker", Container: "lab-speaker", Image: "ze-image", Host: 2, Ready: readyProbe("ready")},
		{Name: "ze", Container: "lab-ze", Host: 3},
	}
	cases := []struct {
		name  string
		plan  ScenarioPlan
		phase ScenarioPhase
		cause string
	}{
		{"setup", failingIn(t, phasePlan("prepare-fails", passing), PhaseSetup, "the relay config did not render"),
			PhaseSetup, "the relay config did not render"},
		{"readiness", failingIn(t, phasePlan("never-ready", passing), PhaseReadiness, ""),
			PhaseReadiness, "ze"},
		{"checker", failingIn(t, phasePlan("checker-red", passing), PhaseChecker,
			"scenario checker-red assertion 2: route 10.0.0.0/24 missing"),
			PhaseChecker, "scenario checker-red assertion 2: route 10.0.0.0/24 missing"},
		{"setup after readiness", afterReadiness, PhaseSetup, "peer image is empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var progress bytes.Buffer
			report := phaseSuite(&progress, phasePlan("first", passing), tc.plan).Run(t.Context())
			if report.Passed != 1 || report.Failed != 1 || len(report.Scenarios) != 2 {
				t.Fatalf("report passed %d failed %d over %d scenarios, want 1, 1 and 2: %#v",
					report.Passed, report.Failed, len(report.Scenarios), report)
			}
			if report.Scenarios[0].Phase != "" {
				t.Errorf("the passing scenario carries phase %q, want none", report.Scenarios[0].Phase)
			}
			if !strings.Contains(progress.String(), "interop: [1/2] first: pass (") {
				t.Errorf("the pass line changed:\n%s", progress.String())
			}
			result := report.Scenarios[1]
			if result.Phase != tc.phase {
				t.Errorf("result phase %q, want %q (error %q)", result.Phase, tc.phase, result.Error)
			}
			if !strings.Contains(result.Error, tc.cause) {
				t.Errorf("result error %q lacks %q", result.Error, tc.cause)
			}
			marker := "interop: [2/2] " + result.Name + ": FAIL ("
			want := ") in " + string(tc.phase) + ": " + result.Error + "\n"
			if !strings.Contains(progress.String(), marker) || !strings.Contains(progress.String(), want) {
				t.Errorf("progress lacks %q ... %q:\n%s", marker, want, progress.String())
			}
			text := report.Text()
			if !strings.Contains(text, "interop: "+result.Name+": FAIL in "+string(tc.phase)+": "+result.Error+"\n") {
				t.Errorf("report text does not name the phase:\n%s", text)
			}
			if report.Stopped != "" || len(report.NotRun) != 0 {
				t.Errorf("one failure stopped the run: %q, not run %v", report.Stopped, report.NotRun)
			}
		})
	}
}

// TestSuiteStopsAfterRepeatedSetupFailures runs a suite whose scenarios fail
// before their checker three times in a row, with a checker failure between
// two earlier setup failures, through Suite.Run.
// VALIDATES: D1 (b). The run stops after setupFailuresBeforeStop scenarios in
// a row fail in setup or readiness; a checker failure resets the count; the
// scenarios after the stop are listed as not run, counted neither passed nor
// failed, and their checkers never run; the stop names the failures and the
// fix with the lab's registered action.
// PREVENTS: a broken lab helper failing every one of 234 scenarios one setup
// timeout at a time, and a skipped scenario read as passed.
func TestSuiteStopsAfterRepeatedSetupFailures(t *testing.T) {
	if _, held := CatalogNamed(phaseLabSuite); !held {
		RegisterCatalog(Catalog{
			Suite:            phaseLabSuite,
			Scenarios:        func(string) ([]ScenarioSource, error) { return nil, nil },
			RunScenario:      func(context.Context, string, string) SuiteReport { return SuiteReport{} },
			Package:          "internal/le/interoplab/phasetest/",
			ScenarioRoot:     phaseLabRoot,
			SelectorVariable: "ZE_PHASE_TEST_SCENARIO",
			Action:           phaseLabAction,
		})
	}
	var reached []string
	reaching := func(name string) Checker {
		return func(context.Context, *CheckContext) error {
			reached = append(reached, name)
			return nil
		}
	}
	var progress bytes.Buffer
	suite := phaseSuite(&progress,
		failingIn(t, phasePlan("one", reaching("one")), PhaseSetup, "network busy"),
		failingIn(t, phasePlan("two", reaching("two")), PhaseChecker, "scenario two assertion 1: no route"),
		failingIn(t, phasePlan("three", reaching("three")), PhaseSetup, "speaker panicked"),
		failingIn(t, phasePlan("four", reaching("four")), PhaseReadiness, ""),
		failingIn(t, phasePlan("five", reaching("five")), PhaseSetup, "speaker panicked"),
		phasePlan("six", reaching("six")),
		phasePlan("seven", reaching("seven")),
	)

	report := suite.Run(t.Context())
	if report.Passed != 0 || report.Failed != 5 {
		t.Fatalf("report passed %d failed %d, want 0 and 5: %#v", report.Passed, report.Failed, report)
	}
	if !slices.Equal(report.NotRun, []string{"six", "seven"}) {
		t.Errorf("not run %v, want [six seven]", report.NotRun)
	}
	if len(report.Scenarios) != 5 {
		t.Errorf("%d scenario results, want 5: a scenario not run has no result", len(report.Scenarios))
	}
	if len(reached) != 0 {
		t.Errorf("checkers ran after the stop or in a failed setup: %v", reached)
	}
	if report.Code != 1 {
		t.Errorf("code %d, want 1", report.Code)
	}
	for _, want := range []string{
		"3 scenarios in a row failed before their checker ran, in setup or readiness: three four five.",
		"2 scenario(s) were not run and none of them passed: six seven.",
		"\nFix: correct the setup cause these failures name, then run the lab again:\n./le test integration interop-phase-test",
	} {
		if !strings.Contains(report.Stopped, want) {
			t.Errorf("stop reason lacks %q:\n%s", want, report.Stopped)
		}
	}
	if !strings.Contains(progress.String(), "interop: stopped: "+report.Stopped+"\n") {
		t.Errorf("progress does not narrate the stop:\n%s", progress.String())
	}
	for _, name := range []string{"six", "seven"} {
		if strings.Contains(progress.String(), "] "+name+":") {
			t.Errorf("progress narrates %s, which never ran:\n%s", name, progress.String())
		}
	}
	text := report.Text()
	for _, want := range []string{"interop: stopped: " + report.Stopped + "\n", "interop: 0 passed, 5 failed, 2 not run: one two three four five\n"} {
		if !strings.Contains(text, want) {
			t.Errorf("report text lacks %q:\n%s", want, text)
		}
	}
}

// TestSuiteNeverStopsOnCheckerFailures runs failures that do not stop a run:
// checker failures in a row, and setup failures in a row with no scenario
// left after them.
// VALIDATES: D1 (b) bounds. Only failures before the checker count, and a
// stop needs a scenario left to skip, so every scenario runs.
// PREVENTS: a run of feature reds cut short, which would hide which other
// scenarios the change broke.
func TestSuiteNeverStopsOnCheckerFailures(t *testing.T) {
	checkerReds := make([]ScenarioPlan, 0, setupFailuresBeforeStop+1)
	setupReds := make([]ScenarioPlan, 0, setupFailuresBeforeStop)
	for _, name := range []string{"a", "b", "c", "d"} {
		checkerReds = append(checkerReds, failingIn(t, phasePlan(name, passing), PhaseChecker, "scenario "+name+" assertion 1: red"))
	}
	for _, name := range []string{"x", "y", "z"} {
		setupReds = append(setupReds, failingIn(t, phasePlan(name, passing), PhaseSetup, "speaker panicked"))
	}
	for _, plans := range [][]ScenarioPlan{checkerReds, setupReds} {
		var progress bytes.Buffer
		report := phaseSuite(&progress, plans...).Run(t.Context())
		if report.Failed != len(plans) {
			t.Errorf("failed %d of %d scenarios, want every one to run and fail: %#v", report.Failed, len(plans), report)
		}
		if report.Stopped != "" || len(report.NotRun) != 0 {
			t.Errorf("the run stopped: %q, not run %v", report.Stopped, report.NotRun)
		}
	}
}
