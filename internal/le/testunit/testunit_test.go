// Design: docs/architecture/core-design.md -- exact subprocess contracts for component-group unit tests
package testunit

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
)

const fixtureTags = "ze_core ze_anomaly ze_as112 ze_bfd ze_bgp ze_bmp ze_copp ze_cos ze_ddos ze_dhcpserver ze_exabgp ze_flowexport ze_geodns ze_gnmi ze_grpc ze_ike ze_isis ze_l2tp ze_ldp ze_lg ze_mcp ze_mpls ze_mrt ze_ntp ze_ospf ze_policyroute ze_pxe ze_radius ze_rest ze_rsvpte ze_ssh ze_tacacs ze_telemetry ze_trafficusage ze_vpp ze_vrrp ze_web"

func fixtureToolchain() gotoolchain.Toolchain {
	return gotoolchain.Toolchain{
		Root:        "/checkout",
		Features:    strings.Fields(strings.TrimPrefix(fixtureTags, "ze_core ")),
		GoToolchain: "go1.26.6",
		Procs:       8,
		Timeout:     "20m",
	}
}

// TestTablePinsEveryUnitGroup protects the producer's population, order,
// package patterns, reasons, and non-writing contract.
func TestTablePinsEveryUnitGroup(t *testing.T) {
	want := []Group{
		{Verb: "bgp", Pattern: "./internal/component/bgp/...", Race: true,
			Why: "the BGP component group: reactor, fsm, wire, message, attribute (~1:30)"},
		{Verb: "core", Pattern: "./internal/core/...", Race: true,
			Why: "the core leaf libraries every tier above depends on (~30s)"},
		{Verb: "plugins", Pattern: "./internal/plugins/...", Race: true,
			Why: "the system plugins: DHCP, NTP, static, firewall, the CLI verb providers (~40s)"},
		{Verb: "config", Pattern: "./internal/component/config/...", Race: true,
			Why: "the YANG-modeled config pipeline: file, tree, resolve (~20s)"},
		{Verb: "cli", Pattern: "./internal/component/cli/...", Race: true,
			Why: "the CLI: modes, completion, diff, commit, dashboard (~10s)"},
		{Verb: "installer", Pattern: "./internal/install/...", Tags: []string{"ze_installer"}, GOOS: "linux",
			Why: "the installer initrd's own logic behind the ze_installer tag: bootstrap, console, fault, rescue, initrd (~10s)"},
	}
	if got := Table(); !reflect.DeepEqual(got, want) {
		t.Fatalf("unit group table differs:\n got: %#v\nwant: %#v", got, want)
	}

	// The listing carries one row for each group, then `all`, which runs them
	// and is therefore not one of them (actions.go, Actions).
	list := Actions()
	if list.Area != Area || len(list.Actions) != len(want)+1 {
		t.Fatalf("action population is area %q with %d rows, want %q with %d", list.Area, len(list.Actions), Area, len(want)+1)
	}
	for index, row := range list.Actions {
		if index < len(want) && (row.Verb != want[index].Verb || row.Why != want[index].Why) {
			t.Errorf("action %d differs: %#v", index, row)
		}
		if row.Writes {
			t.Errorf("action %s claims to write", row.Verb)
		}
	}
	last := list.Actions[len(list.Actions)-1]
	if last.Verb != allVerb || last.Why == "" {
		t.Errorf("the listing ends with %#v, want %q and a reason", last, allVerb)
	}
}

// TestAllExpandsToEveryActionOfTheTable protects the action population and
// order behind the one word that runs them.
// VALIDATES: `le test-unit all` runs every table action exactly once.
// PREVENTS: omitting an action or appending one twice.
func TestAllExpandsToEveryActionOfTheTable(t *testing.T) {
	called := make([]string, 0, 2)
	command := leaction.New("fixture",
		leaction.Action{
			Verb: "one",
			Why:  "first fixture action",
			Answer: func() (any, int) {
				called = append(called, "one")
				return "one report", 0
			},
		},
		leaction.Action{
			Verb: "two",
			Why:  "second fixture action",
			Answer: func() (any, int) {
				called = append(called, "two")
				return "two report", 6
			},
		},
	)

	answer, code := sweep([]string{allVerb}, command)
	if code != 6 {
		t.Fatalf("all sweep answered %d, want 6", code)
	}
	if !reflect.DeepEqual(called, []string{"one", "two"}) {
		t.Fatalf("all sweep calls differ: got %q, want [one two]", called)
	}
	report, ok := answer.(leaction.Sweep)
	if !ok {
		t.Fatalf("all sweep returned %T, want leaction.Sweep", answer)
	}
	if len(report.Ran) != 2 || report.Ran[0].Verb != "one" || report.Ran[1].Verb != "two" {
		t.Errorf("structured sweep differs: %#v", report)
	}
}

// TestBareCommandListsAndRunsNothing proves that typing the area name reads the
// surface instead of starting a test run.
// VALIDATES: `le test-unit` answers the listing, reaches neither the checkout
// nor the toolchain nor the process runner, and names `all` as the run.
// PREVENTS: a developer opening the help and waiting on six race-instrumented
// suites (owner directive, 2026-09-02).
func TestBareCommandListsAndRunsNothing(t *testing.T) {
	resolveRoot := func() (string, error) {
		t.Error("the bare command read the checkout")
		return "/checkout", nil
	}
	loadToolchain := func(string) (gotoolchain.Toolchain, error) {
		t.Error("the bare command resolved the toolchain")
		return fixtureToolchain(), nil
	}
	run := func(string, []string, string, []string) (gaterun.ActionReport, int) {
		t.Error("the bare command started a process")
		return gaterun.ActionReport{}, 0
	}

	result, code := answer(nil, resolveRoot, loadToolchain, run)
	if code != 0 {
		t.Fatalf("the bare command answered %d, want 0", code)
	}
	listing, ok := result.(leaction.List)
	if !ok {
		t.Fatalf("the bare command returned %T, want leaction.List", result)
	}
	verbs := make([]string, 0, len(listing.Actions))
	for _, row := range listing.Actions {
		verbs = append(verbs, row.Verb)
	}
	want := append(groupVerbs(), allVerb)
	if !reflect.DeepEqual(verbs, want) {
		t.Fatalf("the listing names %q, want %q", verbs, want)
	}
	for _, row := range listing.Actions {
		if row.Why == "" {
			t.Errorf("action %q states no reason, so the listing renders it blank", row.Verb)
		}
	}

	// The listing and the help hint are two surfaces on one command, and the
	// reader who typed `--help` never sees the listing, so both name the run.
	if !strings.HasSuffix(Subs(), "| "+allVerb) {
		t.Errorf("the help hint is %q, and it must end by naming %q", Subs(), allVerb)
	}
}

// TestNamedSweepPreservesSelectionOrder proves that an explicit selection is
// neither expanded to the default nor deduplicated.
// VALIDATES: selected test-unit actions retain their order and repetitions.
// PREVENTS: applying the `all` expansion or deduplication to named actions.
func TestNamedSweepPreservesSelectionOrder(t *testing.T) {
	tc := fixtureToolchain()
	rows := table(tc, func(name string, argv []string, _ string, _ []string) (gaterun.ActionReport, int) {
		return gaterun.ActionReport{Action: name, Command: argv}, 0
	}).Actions().Actions
	selected := []string{rows[2].Verb, rows[0].Verb, rows[2].Verb}
	called := make([]string, 0, len(selected))
	command := table(tc, func(name string, argv []string, _ string, _ []string) (gaterun.ActionReport, int) {
		called = append(called, name)
		return gaterun.ActionReport{Action: name, Command: argv}, 0
	})

	if _, code := sweep(selected, command); code != 0 {
		t.Fatalf("named sweep answered %d", code)
	}
	want := []string{Table()[2].Verb, Table()[0].Verb, Table()[2].Verb}
	if !reflect.DeepEqual(called, want) {
		t.Errorf("named sweep calls differ: got %q, want %q", called, want)
	}
}

// TestEachActionPinsArgvAndEnvironment proves that every package pattern runs
// with the full feature set, race detector, cgo, and concurrency cap, and that
// the cross-targeted group carries its own tag, platform and command instead.
func TestEachActionPinsArgvAndEnvironment(t *testing.T) {
	tc := fixtureToolchain()

	for _, group := range Table() {
		t.Run(group.Verb, func(t *testing.T) {
			wantEnvironment := []string{
				"GOCACHE=" + filepath.Join(tc.Root, "cache", "go-cache"),
				"GOLANGCI_LINT_CACHE=" + filepath.Join(tc.Root, "tmp", "golangci-lint-cache"),
				"CGO_ENABLED=1",
				"GOTOOLCHAIN=go1.26.6",
				"GOMAXPROCS=8",
			}
			if group.GOOS != "" {
				// The detector needs cgo, which a cross build cannot have, so
				// the platform group runs without either and pins its GOOS.
				wantEnvironment = []string{
					"GOCACHE=" + filepath.Join(tc.Root, "cache", "go-cache"),
					"GOLANGCI_LINT_CACHE=" + filepath.Join(tc.Root, "tmp", "golangci-lint-cache"),
					"CGO_ENABLED=0",
					"GOTOOLCHAIN=go1.26.6",
					"GOMAXPROCS=8",
					"GOOS=" + group.GOOS,
				}
			}
			var gotArgv, gotEnvironment []string
			run := func(name string, argv []string, root string, environment []string) (gaterun.ActionReport, int) {
				if name != group.Verb {
					t.Errorf("runner received action %q, want %q", name, group.Verb)
				}
				if root != tc.Root {
					t.Errorf("runner root is %q, want %q", root, tc.Root)
				}
				gotArgv = slices.Clone(argv)
				gotEnvironment = slices.Clone(environment)
				return gaterun.ActionReport{Action: name, Command: argv, Code: 0}, 0
			}

			answer, code := table(tc, run).Answer([]string{group.Verb})
			if code != 0 {
				t.Fatalf("action answered %d, want 0", code)
			}
			report, ok := answer.(gaterun.ActionReport)
			if !ok || report.Action != group.Verb || report.Code != 0 {
				t.Fatalf("action report differs: %#v", answer)
			}

			wantArgv := []string{"go", "test", "-timeout", "20m", "-tags", fixtureTags, "-race", group.Pattern}
			if group.GOOS != "" {
				// A group whose platform is not the host is type-checked, and
				// a group naming its own tags takes the bare core set.
				wantArgv = []string{"go", "test", "-timeout", "20m", "-tags", "ze_core ze_installer", group.Pattern}
				if group.GOOS != runtime.GOOS {
					wantArgv = []string{"go", "vet", "-tags", "ze_core ze_installer", group.Pattern}
				}
			}
			if !reflect.DeepEqual(gotArgv, wantArgv) {
				t.Errorf("argv differs:\n got: %q\nwant: %q", gotArgv, wantArgv)
			}
			if len(gotEnvironment) < len(wantEnvironment) {
				t.Fatalf("environment has %d entries, want at least %d", len(gotEnvironment), len(wantEnvironment))
			}
			gotOverrides := gotEnvironment[len(gotEnvironment)-len(wantEnvironment):]
			if !reflect.DeepEqual(gotOverrides, wantEnvironment) {
				t.Errorf("environment overrides differ:\n got: %q\nwant: %q", gotOverrides, wantEnvironment)
			}
		})
	}
}

// TestSweepRunsEveryGroupAndReturnsTheFirstFailure pins the sweep contract: all
// selected groups run, while the first non-zero status wins.
func TestSweepRunsEveryGroupAndReturnsTheFirstFailure(t *testing.T) {
	tc := fixtureToolchain()
	codes := map[string]int{
		"bgp":     0,
		"core":    7,
		"plugins": 3,
		"config":  0,
		"cli":     9,
	}
	called := make([]string, 0, len(codes))
	run := func(name string, argv []string, _ string, _ []string) (gaterun.ActionReport, int) {
		called = append(called, name)
		code := codes[name]
		return gaterun.ActionReport{Action: name, Command: argv, Code: code}, code
	}

	result, code := answer(
		[]string{allVerb},
		func() (string, error) { return tc.Root, nil },
		func(string) (gotoolchain.Toolchain, error) { return tc, nil },
		run,
	)
	if code != 7 {
		t.Fatalf("sweep answered %d, want first failure 7", code)
	}
	wantOrder := groupVerbs()
	if !reflect.DeepEqual(called, wantOrder) {
		t.Fatalf("sweep call order differs: got %q, want %q", called, wantOrder)
	}
	report, ok := result.(leaction.Sweep)
	if !ok {
		t.Fatalf("sweep returned %T, want leaction.Sweep", result)
	}
	wantFailed := []string{"core", "plugins", "cli"}
	if !reflect.DeepEqual(report.Failed, wantFailed) {
		t.Errorf("failed groups differ: got %q, want %q", report.Failed, wantFailed)
	}
	if len(report.Ran) != len(wantOrder) {
		t.Fatalf("sweep reports %d runs, want %d", len(report.Ran), len(wantOrder))
	}
	for _, row := range report.Ran {
		actionReport, ok := row.Answer.(gaterun.ActionReport)
		if !ok || row.Code != codes[row.Verb] || actionReport.Code != row.Code {
			t.Errorf("structured report lost status for %s: %#v", row.Verb, row)
		}
	}
}

func groupVerbs() []string {
	groups := Table()
	verbs := make([]string, len(groups))
	for index, group := range groups {
		verbs[index] = group.Verb
	}
	return verbs
}

// TestAnswerFailsClosedBeforeStartingGo proves that an unreadable checkout or
// toolchain manifest cannot turn into a reduced test surface.
func TestAnswerFailsClosedBeforeStartingGo(t *testing.T) {
	runnerCalled := false
	run := func(string, []string, string, []string) (gaterun.ActionReport, int) {
		runnerCalled = true
		return gaterun.ActionReport{}, 0
	}

	result, code := answer(
		[]string{"core"},
		func() (string, error) { return "", errors.New("root unavailable") },
		func(string) (gotoolchain.Toolchain, error) { return fixtureToolchain(), nil },
		run,
	)
	if result != nil || code != 1 || runnerCalled {
		t.Fatalf("root failure returned answer=%#v code=%d runnerCalled=%v", result, code, runnerCalled)
	}

	result, code = answer(
		[]string{"core"},
		func() (string, error) { return "/checkout", nil },
		func(string) (gotoolchain.Toolchain, error) {
			return gotoolchain.Toolchain{}, errors.New("feature manifest unavailable")
		},
		run,
	)
	if result != nil || code != 1 || runnerCalled {
		t.Fatalf("toolchain failure returned answer=%#v code=%d runnerCalled=%v", result, code, runnerCalled)
	}
}

// TestRunnerStreamsTheGroupOutputAndPreservesStatus executes the real gaterun
// path against this test binary acting as Go. The fixture refuses a missing
// race flag, wrong package group, cgo setting, or concurrency cap.
func TestRunnerStreamsTheGroupOutputAndPreservesStatus(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test executable: %v", err)
	}
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatalf("create fixture bin: %v", err)
	}
	if err := os.Symlink(executable, filepath.Join(bin, "go")); err != nil {
		t.Fatalf("link fixture go: %v", err)
	}

	group := Table()[3]
	tc := gotoolchain.Toolchain{
		Root: root, Features: []string{"ze_fixture"}, GoToolchain: "go1.fixture", Procs: 3, Timeout: "41s",
	}
	t.Setenv("PATH", bin)
	t.Setenv("TESTUNIT_GO_FIXTURE", "1")
	t.Setenv("TESTUNIT_FIXTURE_PATTERN", group.Pattern)
	t.Setenv("TESTUNIT_FIXTURE_TIMEOUT", tc.Timeout)
	t.Setenv("TESTUNIT_FIXTURE_TAGS", tc.TestTags())
	t.Setenv("TESTUNIT_FIXTURE_GOCACHE", filepath.Join(root, "cache", "go-cache"))
	t.Setenv("TESTUNIT_FIXTURE_PROCS", "3")
	t.Setenv("TESTUNIT_FIXTURE_CODE", "23")

	var answer any
	var code int
	stdout, stderr := captureOutput(t, func() {
		answer, code = table(tc, gaterun.Run).Answer([]string{group.Verb})
	})
	if code != 23 {
		t.Fatalf("runner answered %d, want 23", code)
	}
	report, ok := answer.(gaterun.ActionReport)
	if !ok || report.Code != 23 || report.Action != group.Verb {
		t.Fatalf("runner report differs: %#v", answer)
	}
	if want := "unit fixture stdout: " + group.Pattern + "\n"; stdout != want {
		t.Errorf("stdout differs:\n got: %q\nwant: %q", stdout, want)
	}
	wantStderr := "==> " + group.Verb + "\nunit fixture stderr: " + group.Pattern + "\n"
	if stderr != wantStderr {
		t.Errorf("stderr differs:\n got: %q\nwant: %q", stderr, wantStderr)
	}
}

func captureOutput(t *testing.T, action func()) (string, string) {
	t.Helper()
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("open stdout capture: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("open stderr capture: %v", err)
	}
	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	action()
	os.Stdout, os.Stderr = oldStdout, oldStderr
	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("close stdout capture: %v", err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatalf("close stderr capture: %v", err)
	}
	stdout, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("read stdout capture: %v", err)
	}
	stderr, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatalf("read stderr capture: %v", err)
	}
	return string(stdout), string(stderr)
}

func TestMain(m *testing.M) {
	if os.Getenv("TESTUNIT_GO_FIXTURE") == "1" {
		os.Exit(runGoFixture())
	}
	os.Exit(m.Run())
}

func runGoFixture() int {
	pattern := os.Getenv("TESTUNIT_FIXTURE_PATTERN")
	wantArgv := []string{
		"test", "-timeout", os.Getenv("TESTUNIT_FIXTURE_TIMEOUT"),
		"-tags", os.Getenv("TESTUNIT_FIXTURE_TAGS"), "-race", pattern,
	}
	if !slices.Equal(os.Args[1:], wantArgv) {
		_, _ = fmt.Fprintf(os.Stderr, "fixture argv differs: got %q, want %q\n", os.Args[1:], wantArgv)
		return 96
	}
	if got := os.Getenv("CGO_ENABLED"); got != "1" {
		_, _ = fmt.Fprintf(os.Stderr, "fixture CGO_ENABLED=%q, want 1\n", got)
		return 96
	}
	if got := os.Getenv("GOMAXPROCS"); got != os.Getenv("TESTUNIT_FIXTURE_PROCS") {
		_, _ = fmt.Fprintf(os.Stderr, "fixture GOMAXPROCS=%q, want %q\n", got, os.Getenv("TESTUNIT_FIXTURE_PROCS"))
		return 96
	}
	if got := os.Getenv("GOCACHE"); got != os.Getenv("TESTUNIT_FIXTURE_GOCACHE") {
		_, _ = fmt.Fprintf(os.Stderr, "fixture GOCACHE=%q, want %q\n", got, os.Getenv("TESTUNIT_FIXTURE_GOCACHE"))
		return 96
	}

	_, _ = fmt.Fprintf(os.Stdout, "unit fixture stdout: %s\n", pattern)
	_, _ = fmt.Fprintf(os.Stderr, "unit fixture stderr: %s\n", pattern)
	code, err := strconv.Atoi(os.Getenv("TESTUNIT_FIXTURE_CODE"))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "fixture code: %v\n", err)
		return 96
	}
	return code
}
