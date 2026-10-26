package testfunctional

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

type exaBGPRecorder struct {
	commands      []exaBGPCommand
	failures      map[string]exaBGPExecution
	skipArtifacts bool
	contextErr    error
}

func (r *exaBGPRecorder) Run(ctx context.Context, command exaBGPCommand) exaBGPExecution {
	r.commands = append(r.commands, command)
	r.contextErr = ctx.Err()
	if result, exists := r.failures[command.Stage]; exists {
		return result
	}
	if command.Artifact != "" {
		if r.skipArtifacts {
			return exaBGPExecution{}
		}
		if err := os.WriteFile(command.Artifact, []byte("artifact"), 0o700); err != nil {
			return exaBGPExecution{Error: err.Error(), Code: 1}
		}
		return exaBGPExecution{Stdout: command.Stage + " complete\n"}
	}
	// A version probe answers the version it resolved, as the real one does.
	if strings.HasPrefix(command.Stage, "version-") {
		return exaBGPExecution{Stdout: command.Pin + "\n"}
	}
	return exaBGPExecution{Stdout: "42/42 ExaBGP compatibility tests passed\n"}
}

// VALIDATES: the native adapter preserves the Make producer's artifact tags,
// full-suite argv, timeout, toolchain limits, artifact resolution, and cleanup.
// PREVENTS: verify-worktree substituting a narrower suite or a stale binary.
func TestRunExaBGPMatchesMakeProducer(t *testing.T) {
	root := exaBGPFixture(t)
	recorder := &exaBGPRecorder{}

	report, code := runExaBGP(t.Context(), root, recorder)
	if code != 0 || report.Code != 0 {
		t.Fatalf("exit = %d report code = %d error = %q", code, report.Code, report.Error)
	}
	// Two builds, then for each pin its version probe and one subject per
	// predecessor population, then the populations copied from ExaBGP's 4.2
	// and 5.0 branches once. Each subject names its suite, because
	// `le test exabgp` alone runs encoding and nothing else.
	wantCommands := 2 + len(exaBGPPins)*(1+len(exaBGPSuites)) + len(exaBGPOldSuites)
	if len(recorder.commands) != wantCommands {
		t.Fatalf("commands = %d, want %d", len(recorder.commands), wantCommands)
	}

	ze := recorder.commands[0]
	if ze.Stage != "build-ze" {
		t.Fatalf("first stage = %q, want build-ze", ze.Stage)
	}
	if got := argumentAfter(ze.Arguments, "-tags"); got != "ze_core ze_distro ze_setup zetest ze_exabgp" {
		t.Fatalf("ze tags = %q", got)
	}
	if got := argumentAfter(ze.Arguments, "-o"); got != filepath.Join(report.Cleanup.Path, "bin", "ze") {
		t.Fatalf("ze artifact = %q, cleanup path = %q", got, report.Cleanup.Path)
	}

	zeTest := recorder.commands[1]
	if zeTest.Stage != "build-le" {
		t.Fatalf("second stage = %q, want build-le", zeTest.Stage)
	}
	if got := argumentAfter(zeTest.Arguments, "-tags"); got != "ze_le ze_exabgp" {
		t.Fatalf("le tags = %q", got)
	}

	index := 2
	for _, pin := range exaBGPPins {
		probe := recorder.commands[index]
		wantProbe := []string{
			"uv", "run", "--no-project", "--python", exaBGPPython, "--with", "paramiko",
			"--with", pin.Requirement, "python", "-c", exaBGPVersionProbe,
		}
		if probe.Stage != "version-"+pin.Name || !reflect.DeepEqual(probe.Arguments, wantProbe) {
			t.Fatalf("probe %s = %q %#v, want %#v", pin.Name, probe.Stage, probe.Arguments, wantProbe)
		}
		index++
		for _, suite := range exaBGPSuites {
			subject := recorder.commands[index]
			index++
			wantSubject := []string{
				"uv", "run", "--no-project", "--python", exaBGPPython, "--with", "paramiko",
				"--with", pin.Requirement, zeTest.Artifact, "test", "exabgp", suite, "--all",
				"--timeout", "180s",
			}
			if subject.Stage != "exabgp-"+pin.Name+"-"+suite {
				t.Fatalf("subject stage = %q, want exabgp-%s-%s", subject.Stage, pin.Name, suite)
			}
			if !reflect.DeepEqual(subject.Arguments, wantSubject) {
				t.Fatalf("subject %s = %#v, want %#v", subject.Stage, subject.Arguments, wantSubject)
			}
			if subject.Directory != root {
				t.Fatalf("subject %s directory = %q, want %q", subject.Stage, subject.Directory, root)
			}
		}
	}
	// The environment is the same for both subjects, so it is read from the
	// first one rather than asserted twice.
	environment := exaBGPEffectiveEnvironment(recorder.commands[2].Environment)
	if environment["CGO_ENABLED"] != "0" {
		t.Fatalf("CGO_ENABLED = %q, want 0", environment["CGO_ENABLED"])
	}
	// The toolchain is the process's (stdpatch.Activate): the run pins none.
	if environment["GOTOOLCHAIN"] == "go1.26.6" {
		t.Fatalf("GOTOOLCHAIN = %q, pinned; the toolchain is the process's", environment["GOTOOLCHAIN"])
	}
	if environment["LE_TEST_NO_BUILD"] != "1" {
		t.Fatalf("LE_TEST_NO_BUILD = %q, want 1", environment["LE_TEST_NO_BUILD"])
	}
	if environment["ZE_BIN"] != ze.Artifact {
		t.Fatalf("ZE_BIN = %q, want %q", environment["ZE_BIN"], ze.Artifact)
	}
	// The subject runs the set's own le (asserted above), so no harness
	// variable names another file.
	if value, present := environment["LE_TEST_BIN"]; present {
		t.Fatalf("LE_TEST_BIN = %q, want it unset: the harness is the set's le", value)
	}
	if len(report.Artifacts) != 2 {
		t.Fatalf("artifacts = %#v, want ze and le", report.Artifacts)
	}
	if got := report.Children[3].Stdout; got != "42/42 ExaBGP compatibility tests passed\n" {
		t.Fatalf("reported subject stdout = %q", got)
	}
	if !report.Cleanup.Removed || report.Cleanup.Kept || report.Cleanup.Error != "" {
		t.Fatalf("cleanup = %#v, want removed", report.Cleanup)
	}
	if _, err := os.Stat(report.Cleanup.Path); !os.IsNotExist(err) {
		t.Fatalf("isolated binary root remains after cleanup: %v", err)
	}
}

// VALIDATES: each artifact or subject failure is the stage's exact first code.
// PREVENTS: collapsing a useful compiler or compatibility verdict into exit 1.
func TestRunExaBGPPreservesFirstFailureCodeAndCleanup(t *testing.T) {
	tests := []struct {
		name      string
		stage     string
		code      int
		wantCalls int
	}{
		{name: "ze build", stage: "build-ze", code: 37, wantCalls: 1},
		{name: "le build", stage: "build-le", code: 38, wantCalls: 2},
		// The encoding stage failing stops the run before the api stage, which is
		// the point of the first-failure contract: the builds, the first pin's
		// probe and its encoding subject, and nothing after.
		{name: "compatibility subject", stage: "exabgp-5.0.15-encoding", code: 42, wantCalls: 4},
		{name: "pin install", stage: "version-main", code: 2, wantCalls: 6},
	}
	for _, one := range tests {
		t.Run(one.name, func(t *testing.T) {
			root := exaBGPFixture(t)
			recorder := &exaBGPRecorder{failures: map[string]exaBGPExecution{
				one.stage: {Stderr: "subject failed\n", Error: "exit status", Code: one.code},
			}}

			report, code := runExaBGP(t.Context(), root, recorder)
			if code != one.code || report.Code != one.code {
				t.Fatalf("exit = %d report code = %d, want %d", code, report.Code, one.code)
			}
			if len(recorder.commands) != one.wantCalls {
				t.Fatalf("commands = %d, want %d", len(recorder.commands), one.wantCalls)
			}
			last := report.Children[len(report.Children)-1]
			if last.Stage != one.stage || last.Code != one.code {
				t.Fatalf("last child = %#v", last)
			}
			if !report.Cleanup.Removed {
				t.Fatalf("failed run cleanup = %#v", report.Cleanup)
			}
		})
	}
}

// VALIDATES: a successful child must produce its artifact and suite output.
// PREVENTS: a missing build or empty test population being reported as green.
func TestRunExaBGPFailsClosedOnMissingArtifactAndOutput(t *testing.T) {
	t.Run("artifact", func(t *testing.T) {
		root := exaBGPFixture(t)
		recorder := &exaBGPRecorder{skipArtifacts: true}
		report, code := runExaBGP(t.Context(), root, recorder)
		if code != 1 || report.Code != 1 {
			t.Fatalf("exit = %d report code = %d, want 1", code, report.Code)
		}
		if len(recorder.commands) != 1 {
			t.Fatalf("commands = %d, want one failed artifact build", len(recorder.commands))
		}
		if !strings.Contains(report.Children[0].Error, "did not produce") {
			t.Fatalf("artifact error = %q", report.Children[0].Error)
		}
	})

	t.Run("suite output", func(t *testing.T) {
		root := exaBGPFixture(t)
		recorder := &exaBGPRecorder{failures: map[string]exaBGPExecution{
			"exabgp-5.0.15-encoding": {},
		}}
		report, code := runExaBGP(t.Context(), root, recorder)
		if code != 1 || report.Code != 1 {
			t.Fatalf("exit = %d report code = %d, want 1", code, report.Code)
		}
		// The silent suite is the one that must carry the error, and it is not
		// necessarily the last child: this stage runs two suites, and a guard
		// that only read the last one would pass a silent encoding run on the
		// strength of the api run's output.
		var named string
		for _, child := range report.Children {
			if child.Error == "ExaBGP suite produced no output" {
				named = child.Stage
			}
		}
		if named != "exabgp-5.0.15-encoding" {
			t.Fatalf("no child names the silent suite: %#v", report.Children)
		}
	})

	// A silent version probe is named as the probe, with its command, and
	// not as a suite: no suite has started when the probe runs.
	t.Run("version output", func(t *testing.T) {
		root := exaBGPFixture(t)
		recorder := &exaBGPRecorder{failures: map[string]exaBGPExecution{
			"version-5.0.15": {},
		}}
		report, code := runExaBGP(t.Context(), root, recorder)
		if code != 1 || report.Code != 1 {
			t.Fatalf("exit = %d report code = %d, want 1", code, report.Code)
		}
		var probe *ExaBGPChildResult
		for index := range report.Children {
			if report.Children[index].Stage == "version-5.0.15" {
				probe = &report.Children[index]
			}
		}
		if probe == nil {
			t.Fatalf("no version-5.0.15 child: %#v", report.Children)
		}
		for _, want := range []string{"version probe version-5.0.15 printed nothing", "Run the probe by hand", exaBGPVersionProbe} {
			if !strings.Contains(probe.Error, want) {
				t.Errorf("probe error %q does not carry %q", probe.Error, want)
			}
		}
	})
}

// VALIDATES: the legacy timeout alias and named-artifact retention remain usable.
// PREVENTS: the native adapter discarding the Make target's operator controls.
func TestRunExaBGPUsesTimeoutAliasAndRetainsNamedArtifacts(t *testing.T) {
	root := exaBGPFixture(t)
	t.Setenv(exaBGPTimeoutAlias, "27")
	t.Setenv("ZE_SUFFIX", "parity")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	recorder := &exaBGPRecorder{}

	report, code := runExaBGP(t.Context(), root, recorder)
	if code != 0 {
		t.Fatalf("exit = %d error = %q", code, report.Error)
	}
	if report.Timeout != "27s" {
		t.Fatalf("timeout = %q, want 27s", report.Timeout)
	}
	if got := recorder.commands[3].Arguments[len(recorder.commands[3].Arguments)-1]; got != "27s" {
		t.Fatalf("subject timeout = %q, want 27s", got)
	}
	if !report.Cleanup.Kept || report.Cleanup.Removed {
		t.Fatalf("named artifact cleanup = %#v", report.Cleanup)
	}
	for _, artifact := range report.Artifacts {
		if _, err := os.Stat(artifact); err != nil {
			t.Fatalf("named artifact %s is not retained: %v", artifact, err)
		}
	}
}

// VALIDATES: unreadable manifests stop the adapter before any child can run.
// PREVENTS: an empty feature population silently building a reduced Ze binary.
func TestRunExaBGPFailsBeforeChildrenWhenPopulationCannotBeDerived(t *testing.T) {
	root := t.TempDir()
	recorder := &exaBGPRecorder{}

	report, code := runExaBGP(t.Context(), root, recorder)
	if code != 1 || report.Code != 1 {
		t.Fatalf("exit = %d report code = %d, want 1", code, report.Code)
	}
	if len(recorder.commands) != 0 || len(report.Children) != 0 {
		t.Fatalf("children ran without manifests: recorder=%d report=%d", len(recorder.commands), len(report.Children))
	}
	if report.Error == "" {
		t.Fatal("missing population produced no setup error")
	}
}

// VALIDATES: the caller's context reaches the first and every subsequent child.
// PREVENTS: compiler, uv, or le test processes outliving worktree verification.
func TestRunExaBGPPassesCallerContextToEveryChild(t *testing.T) {
	root := exaBGPFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	recorder := &exaBGPRecorder{failures: map[string]exaBGPExecution{
		"build-ze": {Error: "context canceled", Code: 130},
	}}

	report, code := runExaBGP(ctx, root, recorder)
	if code != 130 || report.Code != 130 {
		t.Fatalf("exit = %d report code = %d, want 130", code, report.Code)
	}
	if !errors.Is(recorder.contextErr, context.Canceled) {
		t.Fatalf("runner context error = %v, want context canceled", recorder.contextErr)
	}
}

func exaBGPFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(
		"module example.invalid/exabgp-parity\n\ngo 1.26\ntoolchain go1.26.6\n",
	), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "feature-gates.txt"), []byte(
		"ze_exabgp internal/plugins/exabgp\n",
	), 0o600); err != nil {
		t.Fatalf("write feature-gates.txt: %v", err)
	}
	t.Setenv("ZE_SCRATCH_DIR", "tmp")
	t.Setenv("ZE_TEST_CANONICAL", "")
	t.Setenv("ZE_SUFFIX", "")
	t.Setenv(exaBGPTimeoutAlias, "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	return root
}

func argumentAfter(arguments []string, key string) string {
	for index := range len(arguments) - 1 {
		if arguments[index] == key {
			return arguments[index+1]
		}
	}
	return ""
}

func exaBGPEffectiveEnvironment(entries []string) map[string]string {
	result := make(map[string]string, len(entries))
	for _, entry := range entries {
		key, value, exists := strings.Cut(entry, "=")
		if exists {
			result[key] = value
		}
	}
	return result
}

// TestExaBGPReportTextNamesTheChildThatCouldNotStart drives the stage with a
// runner that answers what a missing executable answers: exit 127 and the
// error the exec package produces. The method is runExaBGP plus the report's
// own Text, which is the only string the verify sweep prints for this action.
//
// It exists because the stage answered 127 with an empty terminal on a runner
// that has no uv: the cause was recorded in the report and nothing read it.
func TestExaBGPReportTextNamesTheChildThatCouldNotStart(t *testing.T) {
	root := exaBGPFixture(t)
	recorder := &exaBGPRecorder{failures: map[string]exaBGPExecution{
		// The first uv child is the first pin's version probe.
		"version-5.0.15": {
			Error: `exec: "uv": executable file not found in $PATH`,
			Code:  127,
		},
	}}

	report, code := runExaBGP(t.Context(), root, recorder)
	if code != 127 {
		t.Fatalf("exit = %d, want 127", code)
	}
	text := report.Text()
	for _, want := range []string{
		"functional/exabgp: stage version-5.0.15 exited 127",
		`exec: "uv": executable file not found in $PATH`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text does not name %q:\n%s", want, text)
		}
	}
	if !strings.HasSuffix(text, "\n") {
		t.Errorf("text does not end in a newline: %q", text)
	}
}

// TestExaBGPReportTextSaysSomethingWhenEveryChildPassed pins the other half.
// The Prose contract is that Text renders the whole answer, so a passing stage
// names the ExaBGP each pin ran and how many children ran, rather than
// printing nothing at all.
func TestExaBGPReportTextSaysSomethingWhenEveryChildPassed(t *testing.T) {
	root := exaBGPFixture(t)

	report, code := runExaBGP(t.Context(), root, &exaBGPRecorder{})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	want := "functional/exabgp: pin 5.0.15 ran ExaBGP 5.0.15\n" +
		"functional/exabgp: pin main ran ExaBGP main\n" +
		"functional/exabgp: 10 children passed\n"
	if report.Text() != want {
		t.Errorf("text = %q, want %q", report.Text(), want)
	}
}

// TestExaBGPMainPinCommands holds the stage to running every population under
// both ExaBGP pins, 5.0.15 and a commit of ExaBGP main, each on the named
// interpreter, and to the report naming the version each pin resolved. The
// method is runExaBGP over a recorder that answers the probe with a version.
// It exists because the stage named one pin, 5.0.15, so ExaBGP 6.0's API was
// driven by nothing here.
func TestExaBGPMainPinCommands(t *testing.T) {
	root := exaBGPFixture(t)
	recorder := &exaBGPRecorder{failures: map[string]exaBGPExecution{
		"version-5.0.15": {Stdout: "5.0.15\n"},
		"version-main":   {Stdout: "6.0.0-20261018+uncontrolled\n"},
	}}

	report, code := runExaBGP(t.Context(), root, recorder)
	if code != 0 {
		t.Fatalf("exit = %d error = %q", code, report.Error)
	}
	ran := make(map[string]bool)
	for _, command := range recorder.commands {
		if command.Pin == "" {
			continue
		}
		if got := argumentAfter(command.Arguments, "--python"); got != exaBGPPython {
			t.Fatalf("%s --python = %q, want %q", command.Stage, got, exaBGPPython)
		}
		if !slices.Contains(command.Arguments, "--no-project") {
			t.Fatalf("%s argv %#v carries no --no-project", command.Stage, command.Arguments)
		}
		ran[command.Stage] = true
	}
	for _, stage := range []string{
		"version-5.0.15", "exabgp-5.0.15-encoding", "exabgp-5.0.15-api",
		"version-main", "exabgp-main-encoding", "exabgp-main-api",
	} {
		if !ran[stage] {
			t.Fatalf("stage %s did not run; ran %v", stage, ran)
		}
	}
	// The main pin is a commit, never a branch or a tag: forty hex digits.
	main := exaBGPPins[1].Requirement
	_, commit, found := strings.Cut(main, "@ git+https://github.com/Exa-Networks/exabgp@")
	if !found || len(commit) != 40 || strings.Trim(commit, "0123456789abcdef") != "" {
		t.Fatalf("main pin %q is not a commit of ExaBGP main", main)
	}
	text := report.Text()
	for _, want := range []string{
		"pin 5.0.15 ran ExaBGP 5.0.15\n",
		"pin main ran ExaBGP 6.0.0-20261018+uncontrolled\n",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("report text %q does not carry %q", text, want)
		}
	}
}

// TestExaBGPStageRunsTheOldPopulationsOnce checks old-4.2 and old-5.0 run
// once per stage, under the main pin's uv environment with the named Python,
// rather than once per pin: their helpers import no ExaBGP, so a second run
// under 5.0.15 would drive the same code twice.
func TestExaBGPStageRunsTheOldPopulationsOnce(t *testing.T) {
	root := exaBGPFixture(t)
	recorder := &exaBGPRecorder{}
	report, code := runExaBGP(t.Context(), root, recorder)
	if code != 0 {
		t.Fatalf("exit = %d error = %q", code, report.Error)
	}
	for _, suite := range []string{"old-4.2", "old-5.0"} {
		var stages []string
		for _, command := range recorder.commands {
			if argumentAfter(command.Arguments, "exabgp") != suite {
				continue
			}
			stages = append(stages, command.Stage)
			if command.Stage != "exabgp-main-"+suite {
				t.Fatalf("%s ran as stage %q, want exabgp-main-%s", suite, command.Stage, suite)
			}
			if got := argumentAfter(command.Arguments, "--python"); got != exaBGPPython {
				t.Fatalf("%s --python = %q, want %q", command.Stage, got, exaBGPPython)
			}
		}
		if len(stages) != 1 {
			t.Fatalf("%s ran as stages %v, want exactly one", suite, stages)
		}
	}
}

// TestExaBGPStageClearsPythonPath sets PYTHONPATH in the stage's own process
// and asserts that every uv child gets it empty, both in what runs and in what
// the report records. A shell PYTHONPATH pointing at an ExaBGP checkout made
// the stage drive that checkout instead of the pin
// (plan/journal/gate-verdict-depends-on-the-machine.md, row 2026-09-13).
func TestExaBGPStageClearsPythonPath(t *testing.T) {
	root := exaBGPFixture(t)
	t.Setenv("PYTHONPATH", "/somewhere/exabgp/src")
	env.ResetCache()
	recorder := &exaBGPRecorder{}

	if _, code := runExaBGP(t.Context(), root, recorder); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	children := 0
	for _, command := range recorder.commands {
		if command.Pin == "" {
			continue
		}
		children++
		value, present := exaBGPEffectiveEnvironment(command.Environment)["PYTHONPATH"]
		if !present || value != "" {
			t.Fatalf("%s runs with PYTHONPATH = %q (present %v), want it set empty", command.Stage, value, present)
		}
		reported, present := exaBGPEffectiveEnvironment(command.ReportEnvironment)["PYTHONPATH"]
		if !present || reported != "" {
			t.Fatalf("%s reports PYTHONPATH = %q (present %v), want it named empty", command.Stage, reported, present)
		}
	}
	if children == 0 {
		t.Fatal("no uv child ran, so nothing was checked")
	}
}
