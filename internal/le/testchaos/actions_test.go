// VALIDATES: spec-le-is-a-ze-binary AC-8 and AC-11. The three chaos gates keep
// their exact command lines, environments, order, output payloads, and exit codes.
// PREVENTS: a reduced normal test surface, a race run without cgo, a linter that
// ignores its ceilings, or a sweep that replaces the first tool failure with 1.

package testchaos

import (
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

// TestChaosGatesKeepExactCommandsAndEnvironments covers the complete boundary
// to go and golangci-lint. The expected slices include ordering and absence.
func TestChaosGatesKeepExactCommandsAndEnvironments(t *testing.T) {
	tc := chaosFixtureToolchain(t)
	cache := filepath.Join(tc.Root, "cache", "go-cache")
	lintCache := filepath.Join(tc.Root, "tmp", "golangci-lint-cache")

	tests := []struct {
		name      string
		gate      string
		argv      []string
		overrides []string
	}{
		{
			name: "lint uses argv parallelism and only the memory ceiling",
			gate: "ze-chaos-lint",
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
			gate: "ze-chaos-unit-test",
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
			gate: "ze-chaos-cli-unit-test",
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

	gates := Table()
	if len(gates) != len(tests) {
		t.Fatalf("Table has %d gates, want %d", len(gates), len(tests))
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gate := gates[index]
			if gate.Name != test.gate {
				t.Errorf("gate %d is %q, want %q", index, gate.Name, test.gate)
			}
			if !slices.Equal(gate.Argv(tc), test.argv) {
				t.Errorf("argv = %q, want %q", gate.Argv(tc), test.argv)
			}
			if !slices.Equal(gate.Overrides(tc), test.overrides) {
				t.Errorf("environment overrides = %q, want %q", gate.Overrides(tc), test.overrides)
			}
			if gate.Writes {
				t.Error("chaos test gate says that it writes the checkout")
			}
		})
	}
}

// TestAggregateRunUsesTableOrderAndFirstFailureCode proves the old bare-area
// behavior. All three tools run, and the first failing tool owns the result.
func TestAggregateRunUsesTableOrderAndFirstFailureCode(t *testing.T) {
	tc := chaosFixtureToolchain(t)
	codes := map[string]int{
		"ze-chaos-lint":          3,
		"ze-chaos-unit-test":     0,
		"ze-chaos-cli-unit-test": 9,
	}
	var ran []string
	run := func(gate string, argv []string, dir string, environ []string) (gaterun.GateReport, int) {
		ran = append(ran, gate)
		code := codes[gate]
		return gaterun.GateReport{Gate: gate, Command: slices.Clone(argv), Code: code}, code
	}

	answer, code := answerWith(tc, run, nil)
	if code != 3 {
		t.Errorf("aggregate code = %d, want first failing code 3", code)
	}
	wantOrder := []string{"ze-chaos-lint", "ze-chaos-unit-test", "ze-chaos-cli-unit-test"}
	if !slices.Equal(ran, wantOrder) {
		t.Errorf("run order = %q, want %q", ran, wantOrder)
	}

	sweep, ok := answer.(leaction.Sweep)
	if !ok {
		t.Fatalf("answer type = %T, want leaction.Sweep", answer)
	}
	wantFailed := []string{"ze-chaos-lint", "ze-chaos-cli-unit-test"}
	if !slices.Equal(sweep.Failed, wantFailed) {
		t.Errorf("failed = %q, want %q", sweep.Failed, wantFailed)
	}
}

// TestUnitTargetRunsSimulatorThenCLI pins mk/test-chaos.mk's two-gate recipe.
// The CLI run must stay second because it extends the simulator's unit surface.
func TestUnitTargetRunsSimulatorThenCLI(t *testing.T) {
	tc := chaosFixtureToolchain(t)
	var ran []string
	run := func(gate string, argv []string, dir string, environ []string) (gaterun.GateReport, int) {
		ran = append(ran, gate)
		return gaterun.GateReport{Gate: gate, Command: slices.Clone(argv)}, 0
	}

	_, code := answerWith(tc, run, []string{
		"ze-chaos-unit-test",
		"ze-chaos-cli-unit-test",
	})
	if code != 0 {
		t.Fatalf("unit pair code = %d, want 0", code)
	}
	want := []string{"ze-chaos-unit-test", "ze-chaos-cli-unit-test"}
	if !slices.Equal(ran, want) {
		t.Errorf("unit pair order = %q, want %q", ran, want)
	}
}

// TestNamedToolFailureKeepsItsReportAndCode checks the structured output from
// one action. The child command and its nonstandard code survive unchanged.
func TestNamedToolFailureKeepsItsReportAndCode(t *testing.T) {
	tc := chaosFixtureToolchain(t)
	const failureCode = 17
	run := func(gate string, argv []string, dir string, environ []string) (gaterun.GateReport, int) {
		report := gaterun.GateReport{
			Gate: gate, Command: slices.Clone(argv), Code: failureCode,
		}
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

	answer, code := answerWith(tc, run, []string{"ze-chaos-cli-unit-test"})
	if code != failureCode {
		t.Fatalf("answer code = %d, want tool code %d", code, failureCode)
	}
	sweep, ok := answer.(leaction.Sweep)
	if !ok {
		t.Fatalf("answer type = %T, want leaction.Sweep", answer)
	}
	if len(sweep.Ran) != 1 {
		t.Fatalf("ran %d actions, want 1", len(sweep.Ran))
	}
	report, ok := sweep.Ran[0].Answer.(gaterun.GateReport)
	if !ok {
		t.Fatalf("action answer type = %T, want gaterun.GateReport", sweep.Ran[0].Answer)
	}
	wantCommand := []string{
		"go", "test", "-timeout", "7m", "-tags", "ze_core ze_bgp ze_chaos", "./cmd/ze",
	}
	if report.Gate != "ze-chaos-cli-unit-test" {
		t.Errorf("report gate = %q", report.Gate)
	}
	if report.Code != failureCode {
		t.Errorf("report code = %d, want %d", report.Code, failureCode)
	}
	if !slices.Equal(report.Command, wantCommand) {
		t.Errorf("report command = %q, want %q", report.Command, wantCommand)
	}
}

// TestAreaMetadataClaimsExactlyThreeChecks pins the public command surface and
// the write flags that the listing exposes before composition imports it.
func TestAreaMetadataClaimsExactlyThreeChecks(t *testing.T) {
	wantGates := []string{"ze-chaos-lint", "ze-chaos-unit-test", "ze-chaos-cli-unit-test"}
	if got := Gates(); !slices.Equal(got, wantGates) {
		t.Errorf("Gates = %q, want %q", got, wantGates)
	}
	listing := Actions()
	if listing.Area != Area {
		t.Errorf("area = %q, want %q", listing.Area, Area)
	}
	if len(listing.Actions) != len(wantGates) {
		t.Fatalf("listing has %d actions, want %d", len(listing.Actions), len(wantGates))
	}
	for index, row := range listing.Actions {
		if row.Gate != wantGates[index] {
			t.Errorf("action %d gate = %q, want %q", index, row.Gate, wantGates[index])
		}
		if row.Writes {
			t.Errorf("action %q says that it writes", row.Gate)
		}
		if len(row.Forks) != 0 {
			t.Errorf("action %q reports an unported script %q", row.Gate, row.Forks)
		}
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

	if answer, code := Answer([]string{"ze-chaos-unit-test"}); code == 0 {
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
	fakeGo := filepath.Join(bin, "go")
	script := "#!/usr/bin/env python3\nimport sys\nprint('tool stdout')\nprint('tool stderr', file=sys.stderr)\nsys.exit(23)\n"
	if err := os.WriteFile(fakeGo, []byte(script), 0o700); err != nil {
		t.Fatalf("write go stand-in: %v", err)
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
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	_, code := Answer([]string{"ze-chaos-cli-unit-test"})

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
	if !strings.Contains(string(gotStderr), "==> ze-chaos-cli-unit-test\n") {
		t.Errorf("stderr lacks gate announcement: %q", gotStderr)
	}
	if !strings.Contains(string(gotStderr), "tool stderr\n") {
		t.Errorf("stderr lacks external tool output: %q", gotStderr)
	}
}
