package functional

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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
	if len(recorder.commands) != 3 {
		t.Fatalf("commands = %d, want two builds and one subject", len(recorder.commands))
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
	if zeTest.Stage != "build-ze-test" {
		t.Fatalf("second stage = %q, want build-ze-test", zeTest.Stage)
	}
	if got := argumentAfter(zeTest.Arguments, "-tags"); got != "ze_test ze_exabgp" {
		t.Fatalf("ze-test tags = %q", got)
	}

	subject := recorder.commands[2]
	wantSubject := []string{
		"uv", "run", "--with", "paramiko", zeTest.Artifact,
		"exabgp", "--all", "--timeout", "180s",
	}
	if !reflect.DeepEqual(subject.Arguments, wantSubject) {
		t.Fatalf("subject = %#v, want %#v", subject.Arguments, wantSubject)
	}
	if subject.Directory != root {
		t.Fatalf("subject directory = %q, want %q", subject.Directory, root)
	}
	environment := exaBGPEffectiveEnvironment(subject.Environment)
	if environment["CGO_ENABLED"] != "0" {
		t.Fatalf("CGO_ENABLED = %q, want 0", environment["CGO_ENABLED"])
	}
	if environment["GOTOOLCHAIN"] != "go1.26.6" {
		t.Fatalf("GOTOOLCHAIN = %q, want go1.26.6", environment["GOTOOLCHAIN"])
	}
	if environment["ZE_TEST_NO_BUILD"] != "1" {
		t.Fatalf("ZE_TEST_NO_BUILD = %q, want 1", environment["ZE_TEST_NO_BUILD"])
	}
	if environment["ZE_BIN"] != ze.Artifact {
		t.Fatalf("ZE_BIN = %q, want %q", environment["ZE_BIN"], ze.Artifact)
	}
	if environment["ZE_TEST_BIN"] != zeTest.Artifact {
		t.Fatalf("ZE_TEST_BIN = %q, want %q", environment["ZE_TEST_BIN"], zeTest.Artifact)
	}
	if len(report.Artifacts) != 2 {
		t.Fatalf("artifacts = %#v, want ze and ze-test", report.Artifacts)
	}
	if got := report.Children[2].Stdout; got != "42/42 ExaBGP compatibility tests passed\n" {
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
		{name: "ze-test build", stage: "build-ze-test", code: 38, wantCalls: 2},
		{name: "compatibility subject", stage: "exabgp", code: 42, wantCalls: 3},
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
			"exabgp": {},
		}}
		report, code := runExaBGP(t.Context(), root, recorder)
		if code != 1 || report.Code != 1 {
			t.Fatalf("exit = %d report code = %d, want 1", code, report.Code)
		}
		last := report.Children[len(report.Children)-1]
		if last.Error != "ExaBGP suite produced no output" {
			t.Fatalf("suite error = %q", last.Error)
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
	if got := recorder.commands[2].Arguments[len(recorder.commands[2].Arguments)-1]; got != "27s" {
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
// PREVENTS: compiler, uv, or ze-test processes outliving worktree verification.
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
		"exabgp": {
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
		"functional/exabgp-test: stage exabgp exited 127",
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
// says how many children ran rather than printing nothing at all.
func TestExaBGPReportTextSaysSomethingWhenEveryChildPassed(t *testing.T) {
	root := exaBGPFixture(t)

	report, code := runExaBGP(t.Context(), root, &exaBGPRecorder{})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if want := "functional/exabgp-test: 3 children passed\n"; report.Text() != want {
		t.Errorf("text = %q, want %q", report.Text(), want)
	}
}
