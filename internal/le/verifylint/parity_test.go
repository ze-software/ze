// Related: verifylint.go -- exact argv, environment, output order, and first-failure behavior
// Related: matrix.go -- every build flavor the former producer selected
//
// VALIDATES: verify-lint run directly executes every non-empty golangci-lint pass with
// the current config, derived package scope, and resource ceilings.
// PREVENTS: a native verifier silently dropping a tagged build, linting an empty
// scope, inheriting the ambient toolchain, or replacing a child's status with 1.
package verifylint

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const fixtureConfig = "version: \"2\"\nrun:\n  timeout: 10m\n  build-tags:\n    - ze_core\n    - ze_a\n    - ze_b\nlinters:\n  enable:\n    - errcheck\n"

func fixtureChain(root string) gotoolchain.Toolchain {
	return gotoolchain.Toolchain{
		Root: root, GoToolchain: "go1.26.6", Procs: 8, LintMemLimit: "9GiB",
	}
}

func lintFixture(t *testing.T) (string, []string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, configName), []byte(fixtureConfig), 0o600); err != nil {
		t.Fatalf("write lint config: %v", err)
	}
	files := make([]string, 0, len(basePasses())+len(flavorMatrix(nil))+2)
	for index := range len(basePasses()) + len(flavorMatrix(nil)) {
		path := filepath.ToSlash(filepath.Join("pkg", "p"+twoDigits(index), "file.go"))
		files = append(files, path)
		writeLintFile(t, root, path, "package fixture\n")
	}
	files = append(files, "examples/plugin/go/main.go", "tools.go")
	writeLintFile(t, root, "examples/plugin/go/main.go", "package main\n")
	writeLintFile(t, root, "tools.go", "//go:build tools\n\npackage tools\n")
	return root, files
}

func writeLintFile(t *testing.T, root, relative, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", relative, err)
	}
}

func twoDigits(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}

type captureCall struct {
	argv        []string
	environment []string
}

func fixtureOps(root string, tracked []string) (runnerOps, *[]captureCall) {
	goListIndex := 0
	calls := make([]captureCall, 0)
	capture := func(_ context.Context, argv []string, _ string, environment []string) commandResult {
		calls = append(calls, captureCall{
			argv:        slices.Clone(argv),
			environment: slices.Clone(environment),
		})
		switch argv[0] {
		case listProgram:
			path := filepath.Join(root, "pkg", "p"+twoDigits(goListIndex), "file.go")
			goListIndex++
			return commandResult{stdout: []byte(path + "\n")}
		case trackedProgram:
			return commandResult{stdout: append([]byte(strings.Join(tracked, "\x00")), 0)}
		default:
			return commandResult{code: 127, err: errors.New("unexpected capture command")}
		}
	}
	return runnerOps{
		lookPath: func(name string) (string, error) { return "/bin/" + name, nil },
		capture:  capture,
		stream:   func(context.Context, []string, string, []string) (int, error) { return 0, nil },
	}, &calls
}

func TestVerifyLintRunActionIsNativeAndGateless(t *testing.T) {
	listing := Actions()
	if listing.Area != "verify-lint" || len(listing.Actions) != 1 {
		t.Fatalf("verify-lint actions = %#v, want one verify-lint action", listing)
	}
	row := listing.Actions[0]
	if row.Verb != "run" || row.Writes {
		t.Fatalf("verify-lint run is not a native gateless action: %#v", row)
	}
}

func TestVerifyLintRunActionGrammarRefusesMalformedScope(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing package value",
			args: []string{"run", "scope"},
			want: "error: argument keyword \"scope\" requires <packages>\n",
		},
		{
			name: "repeated scope",
			args: []string{"run", "scope", "./cmd/ze", "scope", "./internal/le"},
			want: "error: argument keyword \"scope\" was provided more than once\n",
		},
		{
			name: "unquoted package list",
			args: []string{"run", "scope", "./cmd/ze", "./internal/le"},
			want: "error: unknown argument keyword \"./internal/le\"; use one of: scope\n",
		},
		{
			name: "undeclared positional",
			args: []string{"run", "./cmd/ze"},
			want: "error: unknown argument keyword \"./cmd/ze\"; use one of: scope\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var answer any
			var code int
			stdout, stderr := captureLintOutput(t, func() {
				answer, code = Answer(test.args)
			})
			if answer != nil || code != 2 {
				t.Fatalf("Answer(%q) = (%#v, %d), want (nil, 2)", test.args, answer, code)
			}
			if stdout != "" || stderr != test.want {
				t.Fatalf("Answer(%q) output = stdout %q stderr %q, want stdout empty stderr %q", test.args, stdout, stderr, test.want)
			}
		})
	}
}

func TestFlavorMatrixPinsEveryCurrentBuildInOrder(t *testing.T) {
	want := []Flavor{
		{Name: "darwin", GOOS: "darwin", Why: "every !linux and darwin file that both Linux base passes miss"},
		{Name: "freebsd", GOOS: "freebsd", Why: "the FreeBSD TCP-MD5 socket option and non-Linux fallback files"},
		{Name: "openbsd", GOOS: "openbsd", Why: "the generic non-Linux TCP-MD5 fallback selected on OpenBSD"},
		{Name: "dragonfly", GOOS: "dragonfly", GOARCH: "amd64", Why: "the generic Unix fallback outside the explicitly supported BSD targets; amd64 is pinned because it is the only architecture DragonFly builds for, so an arm64 host would select no files and lint nothing"},
		{Name: "wasip1", GOOS: "wasip1", GOARCH: "wasm", Why: "the !unix fallbacks through a target whose whole import graph type-checks"},
		{Name: "linux-amd64", GOOS: "linux", GOARCH: "amd64", Why: "the amd64 filename-selected netlink implementations, which the base Linux pass covers only on an amd64 host"},
		{Name: "linux-arm64", GOOS: "linux", GOARCH: "arm64", Why: "the arm64 filename-selected netlink implementations shipped by the appliance"},
		{Name: "linux-other-arch", GOOS: "linux", GOARCH: "riscv64", Why: "the linux && !amd64 && !arm64 netlink fallback"},
		{Name: "capability", GOOS: "linux", Tags: []string{"debug", "race", "live", "stress", "maprib", "fleetperf", "zetest", "gokrazy", "ze_test", "ze_perf", "ze_analyze", "ze_chaos", "ze_le", "integration", "ze_docvalid_fixture"}, Why: "every additive capability tag that is not a mutually exclusive personality"},
		{Name: "distro", GOOS: "linux", Tags: []string{"ze_distro"}, Why: "the distro daemon build"},
		{Name: "appliance", GOOS: "linux", Tags: []string{"ze_appliance"}, Why: "the daemon packed into the appliance image"},
		{Name: "setup", GOOS: "linux", Tags: []string{"ze_setup"}, Why: "the appliance setup build driver"},
		{Name: "personalities", GOOS: "linux", Tags: []string{"ze_distro", "ze_appliance", "ze_setup"}, Why: "files that assert the behavior of combined personality tags"},
		{Name: "installer", GOOS: "linux", Tags: []string{"ze_installer", "ze_installer_fault"}, Why: "the installer initrd and its fault-injection files"},
		{Name: "installer-nofault", GOOS: "linux", Tags: []string{"ze_installer"}, Why: "the installer files selected when fault injection is off"},
		{Name: "tinygo", GOOS: "linux", Tags: []string{"tinygo"}, Why: "the TinyGo pprof stub"},
		{Name: "setup-standalone", GOOS: "linux", Tags: []string{"ze_setup"}, Without: []string{"ze_core"}, Why: "the standalone ze_setup && !ze_core program"},
		{Name: "compile-out", GOOS: "linux", Without: []string{"ze_a", "ze_b"}, Why: "every !ze_<feature> stub selected with ze_core and no feature gate"},
	}
	if got := flavorMatrix([]string{"ze_a", "ze_b"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("flavor matrix differs:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestPlanPinsEveryArgvEnvironmentScopeAndOrder(t *testing.T) {
	root, tracked := lintFixture(t)
	ops, captureCalls := fixtureOps(root, tracked)
	runner, err := newRunner(t.Context(), root, fixtureChain(root), ops)
	if err != nil {
		t.Fatalf("newRunner: %v", err)
	}
	plan, err := runner.Plan()
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	wantNames := []string{
		"host", "linux-integration", "darwin", "freebsd", "openbsd", "dragonfly", "wasip1",
		"linux-amd64", "linux-arm64", "linux-other-arch", "capability", "distro", "appliance", "setup",
		"personalities", "installer", "installer-nofault", "tinygo", "setup-standalone", "compile-out",
	}
	if len(plan.Passes) != len(wantNames) {
		t.Fatalf("Plan produced %d passes, want %d", len(plan.Passes), len(wantNames))
	}
	for index := range plan.Passes {
		pass := &plan.Passes[index]
		if pass.Name != wantNames[index] || pass.Skipped {
			t.Errorf("pass %d = name %q skipped=%v, want %q runnable", index, pass.Name, pass.Skipped, wantNames[index])
		}
	}

	wantCommands := [][]string{
		{"golangci-lint", "run", "-j", "8", "./..."},
		{"golangci-lint", "run", "-j", "8", "--build-tags", "integration", "./..."},
		{"golangci-lint", "run", "-j", "8", "./pkg/p02"},
		{"golangci-lint", "run", "-j", "8", "./pkg/p03"},
		{"golangci-lint", "run", "-j", "8", "./pkg/p04"},
		{"golangci-lint", "run", "-j", "8", "./pkg/p05"},
		{"golangci-lint", "run", "-j", "8", "./pkg/p06"},
		{"golangci-lint", "run", "-j", "8", "./pkg/p07"},
		{"golangci-lint", "run", "-j", "8", "./pkg/p08"},
		{"golangci-lint", "run", "-j", "8", "./pkg/p09"},
		{"golangci-lint", "run", "-j", "8", "--build-tags", "debug,race,live,stress,maprib,fleetperf,zetest,gokrazy,ze_test,ze_perf,ze_analyze,ze_chaos,ze_le,integration,ze_docvalid_fixture", "./pkg/p10"},
		{"golangci-lint", "run", "-j", "8", "--build-tags", "ze_distro", "./pkg/p11"},
		{"golangci-lint", "run", "-j", "8", "--build-tags", "ze_appliance", "./pkg/p12"},
		{"golangci-lint", "run", "-j", "8", "--build-tags", "ze_setup", "./pkg/p13"},
		{"golangci-lint", "run", "-j", "8", "--build-tags", "ze_distro,ze_appliance,ze_setup", "./pkg/p14"},
		{"golangci-lint", "run", "-j", "8", "--build-tags", "ze_installer,ze_installer_fault", "./pkg/p15"},
		{"golangci-lint", "run", "-j", "8", "--build-tags", "ze_installer", "./pkg/p16"},
		{"golangci-lint", "run", "-j", "8", "--build-tags", "tinygo", "./pkg/p17"},
		{"golangci-lint", "run", "-j", "8", "-c", plan.TaglessConfig, "--build-tags", "ze_a,ze_b,ze_setup", "./pkg/p18"},
		{"golangci-lint", "run", "-j", "8", "-c", plan.TaglessConfig, "--build-tags", "ze_core", "./pkg/p19"},
	}
	for index := range plan.Passes {
		pass := &plan.Passes[index]
		if !reflect.DeepEqual(pass.Command, wantCommands[index]) {
			t.Errorf("%s argv differs:\n got: %q\nwant: %q", pass.Name, pass.Command, wantCommands[index])
		}
		wantOverrides := []string{
			"GOCACHE=" + filepath.Join(root, "cache", "go-cache"),
			"GOLANGCI_LINT_CACHE=" + filepath.Join(root, "tmp", "golangci-lint-cache"),
			"CGO_ENABLED=0", "GOTOOLCHAIN=go1.26.6", "GOMEMLIMIT=9GiB",
		}
		if pass.GOOS != "" {
			wantOverrides = append(wantOverrides, "GOOS="+pass.GOOS)
		}
		if pass.GOARCH != "" {
			wantOverrides = append(wantOverrides, "GOARCH="+pass.GOARCH)
		}
		gotOverrides := pass.Environment[len(pass.Environment)-len(wantOverrides):]
		if !reflect.DeepEqual(gotOverrides, wantOverrides) {
			t.Errorf("%s environment overrides differ:\n got: %q\nwant: %q", pass.Name, gotOverrides, wantOverrides)
		}
	}
	if len(*captureCalls) != len(wantNames)+1 {
		t.Fatalf("planning ran %d capture commands, want 20 go list plus git", len(*captureCalls))
	}
	wantListCalls := map[int][]string{
		0:  {"go", "list", "-e", "-tags", "ze_core ze_a ze_b", "-f", listTemplate, "./..."},
		1:  {"go", "list", "-e", "-tags", "ze_core ze_a ze_b integration", "-f", listTemplate, "./..."},
		19: {"go", "list", "-e", "-tags", "ze_core", "-f", listTemplate, "./..."},
	}
	for index, want := range wantListCalls {
		call := (*captureCalls)[index]
		if !reflect.DeepEqual(call.argv, want) {
			t.Errorf("go-list call %d differs:\n got: %q\nwant: %q", index, call.argv, want)
		}
		wantOverrides := []string{
			"GOCACHE=" + filepath.Join(root, "cache", "go-cache"),
			"GOLANGCI_LINT_CACHE=" + filepath.Join(root, "tmp", "golangci-lint-cache"),
			"CGO_ENABLED=0", "GOTOOLCHAIN=go1.26.6", "GOMEMLIMIT=9GiB",
		}
		if index == 1 || index == 19 {
			wantOverrides = append(wantOverrides, "GOOS=linux")
		}
		gotOverrides := call.environment[len(call.environment)-len(wantOverrides):]
		if !reflect.DeepEqual(gotOverrides, wantOverrides) {
			t.Errorf("go-list call %d environment differs:\n got: %q\nwant: %q", index, gotOverrides, wantOverrides)
		}
	}
	trackedCall := (*captureCalls)[len(*captureCalls)-1]
	if !reflect.DeepEqual(trackedCall.argv, []string{"git", "ls-files", "-z", "--", "*.go"}) {
		t.Errorf("tracked population argv = %q", trackedCall.argv)
	}
	wantTrackedOverrides := []string{
		"GOCACHE=" + filepath.Join(root, "cache", "go-cache"),
		"GOLANGCI_LINT_CACHE=" + filepath.Join(root, "tmp", "golangci-lint-cache"),
		"CGO_ENABLED=0", "GOTOOLCHAIN=go1.26.6", "GOMEMLIMIT=9GiB",
	}
	gotTrackedOverrides := trackedCall.environment[len(trackedCall.environment)-len(wantTrackedOverrides):]
	if !reflect.DeepEqual(gotTrackedOverrides, wantTrackedOverrides) {
		t.Errorf("tracked population environment differs:\n got: %q\nwant: %q", gotTrackedOverrides, wantTrackedOverrides)
	}
	if plan.Coverage.Code != 0 || plan.Coverage.Population != 22 || plan.Coverage.Selected != 20 || len(plan.Coverage.Blind) != 2 {
		t.Fatalf("coverage differs: %#v", plan.Coverage)
	}
}

func TestScopedRunParsesPackagesAndNeverBroadensToTheTree(t *testing.T) {
	root, tracked := lintFixture(t)
	ops, captureCalls := fixtureOps(root, tracked)
	var streamed [][]string
	ops.stream = func(_ context.Context, argv []string, _ string, _ []string) (int, error) {
		streamed = append(streamed, slices.Clone(argv))
		return 0, nil
	}
	runner, err := newRunner(t.Context(), root, fixtureChain(root), ops)
	if err != nil {
		t.Fatalf("newRunner: %v", err)
	}

	var answer any
	var code int
	stdout, stderr := captureLintOutput(t, func() {
		answer, code = runRunner(runner, leaction.Arguments{
			"scope": " \t./pkg/alpha\n./pkg/beta  ",
		})
	})
	if code != 0 {
		t.Fatalf("scoped run code = %d, report = %#v, stderr = %q", code, answer, stderr)
	}
	report, ok := answer.(Report)
	if !ok {
		t.Fatalf("scoped run answer type = %T, want Report", answer)
	}
	if !reflect.DeepEqual(report.Coverage, Coverage{}) {
		t.Fatalf("scoped run reported full-tree coverage: %#v", report.Coverage)
	}
	if strings.Contains(stdout, "every tracked Go file is linted") || strings.Contains(stderr, "tracked Go file") {
		t.Fatalf("scoped run emitted a full-tree coverage verdict: stdout=%q stderr=%q", stdout, stderr)
	}

	patterns := []string{"./pkg/alpha", "./pkg/beta"}
	if len(*captureCalls) != len(basePasses())+len(flavorMatrix(nil)) {
		t.Fatalf("scoped planning ran %d capture commands, want only the 19 go-list calls", len(*captureCalls))
	}
	for index, call := range *captureCalls {
		if call.argv[0] != listProgram || !reflect.DeepEqual(call.argv[len(call.argv)-len(patterns):], patterns) {
			t.Errorf("scoped go-list call %d = %q, want exact package suffix %q", index, call.argv, patterns)
		}
	}
	if len(streamed) != len(basePasses())+len(flavorMatrix(nil)) {
		t.Fatalf("scoped run started %d children, want the complete 19-pass matrix", len(streamed))
	}
	for index, command := range streamed[:len(basePasses())] {
		if !reflect.DeepEqual(command[len(command)-len(patterns):], patterns) {
			t.Errorf("base lint command %d = %q, want exact scoped suffix %q", index, command, patterns)
		}
		if slices.Contains(command, packageRoot) {
			t.Errorf("base lint command %d broadened the scope to %q: %q", index, packageRoot, command)
		}
	}
	for _, pass := range report.Passes {
		if pass.Skipped {
			continue
		}
		if len(pass.Command) < 4 || !reflect.DeepEqual(pass.Command[:4], []string{lintProgram, "run", "-j", "8"}) {
			t.Errorf("%s lost the native concurrency argv: %q", pass.Name, pass.Command)
		}
		if !slices.Contains(pass.Environment, "GOMEMLIMIT=9GiB") ||
			!slices.Contains(pass.Environment, "GOTOOLCHAIN=go1.26.6") ||
			!slices.Contains(pass.Environment, "CGO_ENABLED=0") {
			t.Errorf("%s lost the scoped lint environment: %q", pass.Name, pass.Environment)
		}
	}
}

func TestExplicitFullTreeScopeKeepsTheCoverageProof(t *testing.T) {
	root, tracked := lintFixture(t)
	ops, captureCalls := fixtureOps(root, tracked)
	runner, err := newRunner(t.Context(), root, fixtureChain(root), ops)
	if err != nil {
		t.Fatalf("newRunner: %v", err)
	}
	plan, err := runner.planScope([]string{packageRoot})
	if err != nil {
		t.Fatalf("PlanScope: %v", err)
	}
	if !plan.reportCoverage || plan.Coverage.Population != len(tracked) {
		t.Fatalf("explicit ./... scope lost full-tree coverage: %#v", plan.Coverage)
	}
	last := (*captureCalls)[len(*captureCalls)-1]
	if !reflect.DeepEqual(last.argv, []string{"git", "ls-files", "-z", "--", "*.go"}) {
		t.Fatalf("explicit ./... scope ended with %q, want the tracked-file population query", last.argv)
	}
}

// TestPopulationSkipsTestdataFixtures proves that a tracked Go file under a
// testdata directory never enters the coverage population. The method builds
// the fixture tree, adds one such file, and reads the population count, because
// go list cannot reach a testdata directory and no flavor could ever select it.
// A fixture that is deliberately unparseable is included, because that is the
// shape the old population counted and no pass could have loaded.
func TestPopulationSkipsTestdataFixtures(t *testing.T) {
	root, tracked := lintFixture(t)
	buried := []string{
		"internal/le/mig/testdata/schema/success/model.go",
		"internal/le/mig/testdata/schema/malformed-go/broken.go",
	}
	writeLintFile(t, root, buried[0], "package model\n")
	writeLintFile(t, root, buried[1], "package broken\n\nfunc broken( {\n")
	ops, _ := fixtureOps(root, append(append([]string{}, tracked...), buried...))
	runner, err := newRunner(t.Context(), root, fixtureChain(root), ops)
	if err != nil {
		t.Fatalf("newRunner: %v", err)
	}
	population, err := runner.population()
	if err != nil {
		t.Fatalf("population: %v", err)
	}
	for _, path := range buried {
		if population[path] {
			t.Errorf("testdata fixture %s entered the coverage population", path)
		}
	}
	if len(population) != len(tracked) {
		t.Fatalf("population = %d, want %d: only the testdata files should be skipped",
			len(population), len(tracked))
	}
}

func TestEmptyScopeRunsNoCommandsAndPrintsNothing(t *testing.T) {
	root, tracked := lintFixture(t)
	ops, captureCalls := fixtureOps(root, tracked)
	streamed := 0
	ops.stream = func(context.Context, []string, string, []string) (int, error) {
		streamed++
		return 0, nil
	}
	runner, err := newRunner(t.Context(), root, fixtureChain(root), ops)
	if err != nil {
		t.Fatalf("newRunner: %v", err)
	}

	var answer any
	var code int
	stdout, stderr := captureLintOutput(t, func() {
		answer, code = runRunner(runner, leaction.Arguments{"scope": ""})
	})
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("empty scope = report %#v code %d stdout %q stderr %q, want a silent success", answer, code, stdout, stderr)
	}
	report, ok := answer.(Report)
	if !ok || len(report.Passes) != len(basePasses())+len(flavorMatrix(nil)) {
		t.Fatalf("empty scope report = %#v, want every matrix row recorded", answer)
	}
	for _, pass := range report.Passes {
		if !pass.Skipped {
			t.Errorf("empty scope left %s runnable: %#v", pass.Name, pass)
		}
	}
	if len(*captureCalls) != 0 || streamed != 0 {
		t.Fatalf("empty scope ran %d planning and %d lint commands", len(*captureCalls), streamed)
	}
}

func TestMalformedScopeStopsBeforeAnyLintCommand(t *testing.T) {
	root, tracked := lintFixture(t)
	ops, _ := fixtureOps(root, tracked)
	var planned [][]string
	ops.capture = func(_ context.Context, argv []string, _ string, _ []string) commandResult {
		planned = append(planned, slices.Clone(argv))
		return commandResult{stderr: []byte("malformed package pattern\n"), code: 1}
	}
	streamed := 0
	ops.stream = func(context.Context, []string, string, []string) (int, error) {
		streamed++
		return 0, nil
	}
	runner, err := newRunner(t.Context(), root, fixtureChain(root), ops)
	if err != nil {
		t.Fatalf("newRunner: %v", err)
	}

	var report Report
	var code int
	stdout, stderr := captureLintOutput(t, func() {
		report, code = runner.runScope([]string{"./bad["})
	})
	wantArgv := []string{
		"go", "list", "-e", "-tags", "ze_core ze_a ze_b", "-f", listTemplate, "./bad[",
	}
	if len(planned) != 1 || !reflect.DeepEqual(planned[0], wantArgv) {
		t.Fatalf("malformed scope planning argv = %q, want exactly %q", planned, wantArgv)
	}
	if code != cannotPlan || report.Code != cannotPlan ||
		report.Error != "go list for lint flavor host returned code 1 with malformed package pattern" {
		t.Fatalf("malformed scope report = %#v code %d", report, code)
	}
	wantStderr := "lint: go list for lint flavor host returned code 1 with malformed package pattern\n"
	if stdout != "" || stderr != wantStderr || streamed != 0 {
		t.Fatalf("malformed scope output = stdout %q stderr %q streamed %d, want stdout empty stderr %q and no lint", stdout, stderr, streamed, wantStderr)
	}
}

func TestExecuteRunsAllChildrenAndReturnsTheFirstFailureCode(t *testing.T) {
	root, _ := lintFixture(t)
	var ran []string
	ops := runnerOps{
		lookPath: func(name string) (string, error) { return name, nil },
		capture:  func(context.Context, []string, string, []string) commandResult { return commandResult{} },
		stream: func(_ context.Context, argv []string, _ string, _ []string) (int, error) {
			ran = append(ran, argv[len(argv)-1])
			switch argv[len(argv)-1] {
			case "second":
				return 7, nil
			case "fourth":
				return 3, nil
			default:
				return 0, nil
			}
		},
	}
	runner, err := newRunner(t.Context(), root, fixtureChain(root), ops)
	if err != nil {
		t.Fatalf("newRunner: %v", err)
	}
	plan := LintPlan{
		Passes: []PassPlan{
			{Name: "host", Command: []string{lintProgram, "run", "first"}, Packages: []string{"first"}},
			{Name: "linux-integration", Command: []string{lintProgram, "run", "second"}, Packages: []string{"second"}},
			{Name: "empty-on-host", Skipped: true},
			{Name: "darwin", Command: []string{lintProgram, "run", "fourth"}, Packages: []string{"fourth"}},
		},
		Coverage: Coverage{Population: 3, Selected: 3},
	}
	report, code := runner.execute(plan)
	if code != 7 || report.Code != 7 {
		t.Fatalf("execute code=%d report.code=%d, want first failure 7", code, report.Code)
	}
	if want := []string{"first", "second", "fourth"}; !reflect.DeepEqual(ran, want) {
		t.Fatalf("children ran in order %q, want %q", ran, want)
	}
	if len(report.Passes) != 4 || !report.Passes[2].Skipped || report.Passes[1].Code != 7 || report.Passes[3].Code != 3 {
		t.Fatalf("structured child results differ: %#v", report.Passes)
	}
}

func TestRunnerRefusesMissingToolAndEmptyPopulationOutput(t *testing.T) {
	root, tracked := lintFixture(t)
	chain := fixtureChain(root)
	ops, _ := fixtureOps(root, tracked)
	ops.lookPath = func(name string) (string, error) {
		if name == lintProgram {
			return "", errors.New("not found")
		}
		return name, nil
	}
	if _, err := newRunner(t.Context(), root, chain, ops); err == nil || !strings.Contains(err.Error(), lintProgram) {
		t.Fatalf("missing linter error = %v", err)
	}

	ops, _ = fixtureOps(root, tracked)
	ops.capture = func(_ context.Context, argv []string, _ string, _ []string) commandResult {
		if argv[0] == listProgram {
			return commandResult{}
		}
		return commandResult{stdout: []byte(strings.Join(tracked, "\x00") + "\x00")}
	}
	runner, err := newRunner(t.Context(), root, chain, ops)
	if err != nil {
		t.Fatalf("newRunner: %v", err)
	}
	if _, err := runner.Plan(); err == nil || !strings.Contains(err.Error(), "no output") {
		t.Fatalf("empty go-list output error = %v", err)
	}

	ops, _ = fixtureOps(root, nil)
	runner, err = newRunner(t.Context(), root, chain, ops)
	if err != nil {
		t.Fatalf("newRunner for empty tracked population: %v", err)
	}
	if _, err := runner.Plan(); err == nil || !strings.Contains(err.Error(), "tracked Go population is empty") {
		t.Fatalf("empty tracked population error = %v", err)
	}
}

func TestTaglessConfigurationKeepsCurrentLintersAndDropsOnlyBuildTags(t *testing.T) {
	derived, err := deriveTaglessConfig([]byte(fixtureConfig))
	if err != nil {
		t.Fatalf("deriveTaglessConfig: %v", err)
	}
	text := string(derived)
	if strings.Contains(text, "build-tags:") || strings.Contains(text, "- ze_core") {
		t.Fatalf("tagless config retained build tags:\n%s", text)
	}
	for _, preserved := range []string{"version: \"2\"", "timeout: 10m", "relative-path-mode: gitroot", "linters:", "- errcheck"} {
		if !strings.Contains(text, preserved) {
			t.Errorf("tagless config dropped %q:\n%s", preserved, text)
		}
	}
}

func TestProducerHeadingsAndCoverageOutputArePinned(t *testing.T) {
	var outputErr error
	stdout, stderr := captureLintOutput(t, func() {
		host := PassPlan{Name: "host"}
		outputErr = errors.Join(outputErr, announcePass(&host))
		integration := PassPlan{Name: "linux-integration"}
		outputErr = errors.Join(outputErr, announcePass(&integration))
		installer := PassPlan{
			Name: "installer", GOOS: "linux", TagsAdded: []string{"ze_installer"},
			Packages: []string{"./cmd/ze-installer", "./internal/install/disk"},
		}
		outputErr = errors.Join(outputErr, announcePass(&installer))
		outputErr = errors.Join(outputErr, renderCoverage(Coverage{Population: 42, Selected: 42}))
	})
	if outputErr != nil {
		t.Fatalf("write producer output: %v", outputErr)
	}
	wantStdout := "" +
		"Running ze linter...\n" +
		"Running ze linter (GOOS=linux, integration tag)...\n" +
		"Running ze linter (installer: GOOS=linux, tags add ze_installer drop none, 2 packages)...\n" +
		"lint_flavors: every tracked Go file is linted, except the 0 stated above.\n"
	if stdout != wantStdout || stderr != "" {
		t.Fatalf("producer output differs:\nstdout got: %q\nstdout want: %q\nstderr: %q", stdout, wantStdout, stderr)
	}
}

func TestExecuteRemovesDerivedTaglessConfiguration(t *testing.T) {
	root, _ := lintFixture(t)
	path := filepath.Join(root, taglessDir, "notags-test.yml")
	seen := false
	ops := runnerOps{
		lookPath: func(name string) (string, error) { return name, nil },
		capture:  func(context.Context, []string, string, []string) commandResult { return commandResult{} },
		stream: func(_ context.Context, _ []string, _ string, _ []string) (int, error) {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read derived config during lint: %v", err)
			}
			seen = !strings.Contains(string(content), "build-tags:")
			return 0, nil
		},
	}
	runner, err := newRunner(t.Context(), root, fixtureChain(root), ops)
	if err != nil {
		t.Fatalf("newRunner: %v", err)
	}
	plan := LintPlan{
		Passes: []PassPlan{{
			Name: "compile-out", Packages: []string{"./pkg/p00"},
			Command: []string{lintProgram, "run", "-c", path, "./pkg/p00"},
		}},
		Coverage:       Coverage{Population: 1, Selected: 1},
		TaglessConfig:  path,
		NeedsTagless:   true,
		configContents: []byte(fixtureConfig),
	}
	report, code := runner.execute(plan)
	if code != 0 || report.Code != 0 || !seen {
		t.Fatalf("tagless execution report=%#v code=%d seen=%v", report, code, seen)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("derived config remains after lint: %v", err)
	}
}

func captureLintOutput(t *testing.T, action func()) (string, string) {
	t.Helper()
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("open stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("open stderr pipe: %v", err)
	}
	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	action()
	os.Stdout, os.Stderr = oldStdout, oldStderr
	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	stdout, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	stderr, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if err := stdoutReader.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	if err := stderrReader.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}
	return string(stdout), string(stderr)
}

func TestRepositoryConfigurationDefinesCompileOutDrops(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}
	config, err := os.ReadFile(filepath.Join(root, configName))
	if err != nil {
		t.Fatalf("read repository lint config: %v", err)
	}
	tags, err := parseConfigTags(config)
	if err != nil {
		t.Fatalf("parse repository lint config: %v", err)
	}
	features := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag != "ze_core" {
			features = append(features, tag)
		}
	}
	matrix := flavorMatrix(features)
	compileOut := matrix[len(matrix)-1]
	if compileOut.Name != "compile-out" || !reflect.DeepEqual(compileOut.Without, features) {
		t.Fatalf("compile-out drops %q, want current config features %q", compileOut.Without, features)
	}
}
