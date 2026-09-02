// VALIDATES: spec-le-is-a-ze-binary AC-8 and AC-11. The three chaos gates keep
// their exact command lines, environments, order, output payloads, and exit codes.
// PREVENTS: a reduced normal test surface, a race run without cgo, a linter that
// ignores its ceilings, or a sweep that replaces the first tool failure with 1.

package testchaos

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
)

func TestMain(m *testing.M) {
	if os.Getenv("TESTCHAOS_GO_FIXTURE") == "1" {
		_, _ = fmt.Fprintln(os.Stdout, "tool stdout")
		fmt.Fprintln(os.Stderr, "tool stderr")
		os.Exit(23)
	}
	os.Exit(m.Run())
}

func chaosFixtureToolchain(t *testing.T) gotoolchain.Toolchain {
	t.Helper()
	return gotoolchain.Toolchain{
		Root:         t.TempDir(),
		Features:     []string{"ze_bgp", "ze_web"},
		GoToolchain:  "go1.26.6",
		Procs:        8,
		Timeout:      "7m",
		LintMemLimit: "9GiB",
		ExtraTags:    []string{"ze_extra"},
	}
}

// TestChaosActionsKeepExactCommandsAndEnvironments covers the complete boundary
// to Go and golangci-lint.
func TestChaosActionsKeepExactCommandsAndEnvironments(t *testing.T) {
	tc := chaosFixtureToolchain(t)
	cache := filepath.Join(tc.Root, "cache", "go-cache")
	lintCache := filepath.Join(tc.Root, "tmp", "golangci-lint-cache")

	tests := []struct {
		name      string
		verb      string
		argv      []string
		overrides []string
	}{
		{
			name: "lint uses argv parallelism and only the memory ceiling",
			verb: "lint",
			argv: []string{"golangci-lint", "run", "-j", "8", "./internal/chaos/..."},
			overrides: []string{
				"GOCACHE=" + cache,
				"GOLANGCI_LINT_CACHE=" + lintCache,
				"CGO_ENABLED=0",
				"GOTOOLCHAIN=go1.26.6",
				"GOMEMLIMIT=9GiB",
			},
		},
		{
			name: "unit uses every feature tag and the race detector",
			verb: "unit",
			argv: []string{
				"go", "test", "-timeout", "7m", "-tags",
				"ze_core ze_bgp ze_web ze_extra", "-race", "./internal/chaos/...",
			},
			overrides: []string{
				"GOCACHE=" + cache,
				"GOLANGCI_LINT_CACHE=" + lintCache,
				"CGO_ENABLED=1",
				"GOTOOLCHAIN=go1.26.6",
				"GOMAXPROCS=8",
			},
		},
		{
			name: "CLI uses the required reduced tags without race",
			verb: "cli-unit",
			argv: []string{
				"go", "test", "-timeout", "7m", "-tags",
				"ze_core ze_bgp ze_chaos", "./cmd/ze",
			},
			overrides: []string{
				"GOCACHE=" + cache,
				"GOLANGCI_LINT_CACHE=" + lintCache,
				"CGO_ENABLED=0",
				"GOTOOLCHAIN=go1.26.6",
				"GOMAXPROCS=8",
			},
		},
	}

	declared := Table()
	if len(declared) != len(tests) {
		t.Fatalf("Table has %d actions, want %d", len(declared), len(tests))
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := declared[index]
			if action.Verb != test.verb {
				t.Errorf("action %d is %q, want %q", index, action.Verb, test.verb)
			}
			if !slices.Equal(action.Argv(tc), test.argv) {
				t.Errorf("argv = %q, want %q", action.Argv(tc), test.argv)
			}
			if !slices.Equal(action.Overrides(tc), test.overrides) {
				t.Errorf("environment overrides = %q, want %q", action.Overrides(tc), test.overrides)
			}
			if action.Writes {
				t.Error("chaos test action says that it writes the checkout")
			}
		})
	}
}

// TestAggregateRunUsesTableOrderAndFirstFailureCode proves what `all` runs.
// All three tools run, and the first failing tool owns the result.
func TestAggregateRunUsesTableOrderAndFirstFailureCode(t *testing.T) {
	tc := chaosFixtureToolchain(t)
	codes := map[string]int{"lint": 3, "unit": 0, "cli-unit": 9}
	var ran []string
	run := func(action string, argv []string, _ string, _ []string) (gaterun.ActionReport, int) {
		ran = append(ran, action)
		code := codes[action]
		return gaterun.ActionReport{Action: action, Command: slices.Clone(argv), Code: code}, code
	}

	answer, code := answerWith(tc, run, []string{allVerb})
	if code != 3 {
		t.Errorf("aggregate code = %d, want first failing code 3", code)
	}
	wantOrder := []string{"lint", "unit", "cli-unit"}
	if !slices.Equal(ran, wantOrder) {
		t.Errorf("run order = %q, want %q", ran, wantOrder)
	}
	sweep, ok := answer.(leaction.Sweep)
	if !ok {
		t.Fatalf("answer type = %T, want leaction.Sweep", answer)
	}
	wantFailed := []string{"lint", "cli-unit"}
	if !slices.Equal(sweep.Failed, wantFailed) {
		t.Errorf("failed = %q, want %q", sweep.Failed, wantFailed)
	}
}

func TestUnitActionsRunSimulatorThenCLI(t *testing.T) {
	tc := chaosFixtureToolchain(t)
	var ran []string
	run := func(action string, argv []string, _ string, _ []string) (gaterun.ActionReport, int) {
		ran = append(ran, action)
		return gaterun.ActionReport{Action: action, Command: slices.Clone(argv)}, 0
	}
	_, code := answerWith(tc, run, []string{"unit", "cli-unit"})
	if code != 0 {
		t.Fatalf("unit pair code = %d, want 0", code)
	}
	want := []string{"unit", "cli-unit"}
	if !slices.Equal(ran, want) {
		t.Errorf("unit pair order = %q, want %q", ran, want)
	}
}

func TestNamedToolFailureKeepsItsReportAndCode(t *testing.T) {
	tc := chaosFixtureToolchain(t)
	const failureCode = 17
	run := func(action string, argv []string, dir string, environ []string) (gaterun.ActionReport, int) {
		report := gaterun.ActionReport{Action: action, Command: slices.Clone(argv), Code: failureCode}
		if dir != tc.Root {
			t.Errorf("run directory = %q, want %q", dir, tc.Root)
		}
		wantEnv := Table()[2].Overrides(tc)
		if len(environ) < len(wantEnv) {
			t.Errorf("environment has %d entries, fewer than %d required overrides", len(environ), len(wantEnv))
			return report, failureCode
		}
		gotEnv := environ[len(environ)-len(wantEnv):]
		if !slices.Equal(gotEnv, wantEnv) {
			t.Errorf("environment suffix = %q, want %q", gotEnv, wantEnv)
		}
		return report, failureCode
	}

	answer, code := answerWith(tc, run, []string{"cli-unit"})
	if code != failureCode {
		t.Fatalf("answer code = %d, want tool code %d", code, failureCode)
	}
	sweep, ok := answer.(leaction.Sweep)
	if !ok || len(sweep.Ran) != 1 {
		t.Fatalf("answer = %#v, want one-action sweep", answer)
	}
	report, ok := sweep.Ran[0].Answer.(gaterun.ActionReport)
	if !ok {
		t.Fatalf("action answer type = %T, want gaterun.ActionReport", sweep.Ran[0].Answer)
	}
	wantCommand := []string{
		"go", "test", "-timeout", "7m", "-tags", "ze_core ze_bgp ze_chaos", "./cmd/ze",
	}
	if report.Action != "cli-unit" || report.Code != failureCode {
		t.Errorf("report = %#v", report)
	}
	if !slices.Equal(report.Command, wantCommand) {
		t.Errorf("report command = %q, want %q", report.Command, wantCommand)
	}
}

// TestAreaMetadataDeclaresThreeToolsAndTheRunThatSweepsThem pins the public
// command surface: the three tools of the table, then the word that runs them.
func TestAreaMetadataDeclaresThreeToolsAndTheRunThatSweepsThem(t *testing.T) {
	wantVerbs := []string{"lint", "unit", "cli-unit", allVerb}
	listing := Actions()
	if listing.Area != Area {
		t.Errorf("area = %q, want %q", listing.Area, Area)
	}
	if len(listing.Actions) != len(wantVerbs) {
		t.Fatalf("listing has %d actions, want %d", len(listing.Actions), len(wantVerbs))
	}
	for index, row := range listing.Actions {
		if row.Verb != wantVerbs[index] {
			t.Errorf("action %d verb = %q, want %q", index, row.Verb, wantVerbs[index])
		}
		if row.Writes {
			t.Errorf("action %q says that it writes", row.Verb)
		}
	}
}

// TestBareCommandListsAndRunsNothing proves that typing the area name reads the
// surface instead of starting the chaos suites.
// VALIDATES: `le test-chaos` answers the listing, resolves no toolchain, and
// names `all` as the run.
// PREVENTS: a developer opening the help and waiting on a race-instrumented
// simulator run (owner directive, 2026-09-02).
func TestBareCommandListsAndRunsNothing(t *testing.T) {
	// A checkout with no feature manifest is what gotoolchain.New refuses, so a
	// bare command that answered 0 here reached no toolchain at all.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/chaos\n"), 0o600); err != nil {
		t.Fatalf("write fixture go.mod: %v", err)
	}
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	result, code := Answer(nil)
	if code != 0 {
		t.Fatalf("the bare command answered %d, want 0", code)
	}
	listing, ok := result.(leaction.List)
	if !ok {
		t.Fatalf("the bare command returned %T, want leaction.List", result)
	}
	want := []string{"lint", "unit", "cli-unit", allVerb}
	verbs := make([]string, 0, len(listing.Actions))
	for _, row := range listing.Actions {
		verbs = append(verbs, row.Verb)
		if row.Why == "" {
			t.Errorf("action %q states no reason, so the listing renders it blank", row.Verb)
		}
	}
	if !slices.Equal(verbs, want) {
		t.Fatalf("the listing names %q, want %q", verbs, want)
	}

	// The listing and the help hint are two surfaces on one command, and the
	// reader who typed `--help` never sees the listing, so both name the run.
	if !strings.HasSuffix(Subs(), "| "+allVerb) {
		t.Errorf("the help hint is %q, and it must end by naming %q", Subs(), allVerb)
	}
}

// TestUnreadableToolchainStopsBeforeAChildRuns proves setup errors fail closed.
// A missing feature manifest must not become a ze_core-only test run.
func TestUnreadableToolchainStopsBeforeAChildRuns(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/chaos\n"), 0o600); err != nil {
		t.Fatalf("write fixture go.mod: %v", err)
	}
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	if answer, code := Answer([]string{"unit"}); code == 0 {
		t.Fatalf("Answer = %#v, code 0 for a checkout with no feature manifest", answer)
	}
}

// TestExternalToolOutputAndExitCodePassThrough drives the real gaterun boundary.
// A stand-in named go writes both streams and exits 23, as the external tool can.
func TestExternalToolOutputAndExitCodePassThrough(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"),
		[]byte("module example.test/chaos\n\ngo 1.26\ntoolchain go1.26.6\n"), 0o600); err != nil {
		t.Fatalf("write fixture go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "feature-gates.txt"),
		[]byte("ze_bgp internal/component/bgp\n"), 0o600); err != nil {
		t.Fatalf("write fixture feature manifest: %v", err)
	}
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatalf("make fixture bin: %v", err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test executable: %v", err)
	}
	fakeGo := filepath.Join(bin, "go")
	if err := os.Symlink(executable, fakeGo); err != nil {
		t.Fatalf("link go stand-in: %v", err)
	}

	stdoutPath := filepath.Join(root, "stdout")
	stderrPath := filepath.Join(root, "stderr")
	stdout, err := os.Create(stdoutPath)
	if err != nil {
		t.Fatalf("create stdout capture: %v", err)
	}
	stderr, err := os.Create(stderrPath)
	if err != nil {
		if closeErr := stdout.Close(); closeErr != nil {
			t.Errorf("close stdout capture after stderr setup failure: %v", closeErr)
		}
		t.Fatalf("create stderr capture: %v", err)
	}
	savedStdout, savedStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdout, stderr
	defer func() {
		os.Stdout, os.Stderr = savedStdout, savedStderr
	}()

	t.Setenv("ZE_REPO_ROOT", root)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TESTCHAOS_GO_FIXTURE", "1")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	_, code := Answer([]string{"cli-unit"})

	os.Stdout, os.Stderr = savedStdout, savedStderr
	if err := stdout.Close(); err != nil {
		t.Fatalf("close stdout capture: %v", err)
	}
	if err := stderr.Close(); err != nil {
		t.Fatalf("close stderr capture: %v", err)
	}
	if code != 23 {
		t.Errorf("Answer code = %d, want external tool code 23", code)
	}
	gotStdout, err := os.ReadFile(stdoutPath)
	if err != nil {
		t.Fatalf("read stdout capture: %v", err)
	}
	if string(gotStdout) != "tool stdout\n" {
		t.Errorf("stdout = %q, want external tool output", gotStdout)
	}
	gotStderr, err := os.ReadFile(stderrPath)
	if err != nil {
		t.Fatalf("read stderr capture: %v", err)
	}
	if !strings.Contains(string(gotStderr), "==> cli-unit\n") {
		t.Errorf("stderr lacks action announcement: %q", gotStderr)
	}
	if !strings.Contains(string(gotStderr), "tool stderr\n") {
		t.Errorf("stderr lacks external tool output: %q", gotStderr)
	}
}
