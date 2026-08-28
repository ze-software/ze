// Design: docs/architecture/core-design.md -- exact subprocess contracts for component-group unit tests
package testunit

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
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
		{"bgp", "./internal/component/bgp/...", "the BGP component group: reactor, fsm, wire, message, attribute (~1:30)"},
		{"core", "./internal/core/...", "the core leaf libraries every tier above depends on (~30s)"},
		{"plugins", "./internal/plugins/...", "the system plugins: DHCP, NTP, static, firewall, the CLI verb providers (~40s)"},
		{"config", "./internal/component/config/...", "the YANG-modeled config pipeline: file, tree, resolve (~20s)"},
		{"cli", "./internal/component/cli/...", "the CLI: modes, completion, diff, commit, dashboard (~10s)"},
	}
	if got := Table(); !reflect.DeepEqual(got, want) {
		t.Fatalf("unit group table differs:\n got: %#v\nwant: %#v", got, want)
	}

	list := Actions()
	if list.Area != Area || len(list.Actions) != len(want) {
		t.Fatalf("action population is area %q with %d rows, want %q with %d", list.Area, len(list.Actions), Area, len(want))
	}
	for index, row := range list.Actions {
		if row.Verb != want[index].Verb || row.Why != want[index].Why {
			t.Errorf("action %d differs: %#v", index, row)
		}
		if row.Writes {
			t.Errorf("action %s claims to write", row.Verb)
		}
	}
}

// TestBareSweepDerivesEveryDefaultFromTheActionTable protects the action
// population and order.
// VALIDATES: bare `le test-unit` runs every table action exactly once.
// PREVENTS: omitting an action or appending one twice.
func TestBareSweepDerivesEveryDefaultFromTheActionTable(t *testing.T) {
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

	answer, code := sweep(nil, command)
	if code != 6 {
		t.Fatalf("bare sweep answered %d, want 6", code)
	}
	if !reflect.DeepEqual(called, []string{"one", "two"}) {
		t.Fatalf("bare sweep calls differ: got %q, want [one two]", called)
	}
	report, ok := answer.(leaction.Sweep)
	if !ok {
		t.Fatalf("bare sweep returned %T, want leaction.Sweep", answer)
	}
	if len(report.Ran) != 2 || report.Ran[0].Verb != "one" || report.Ran[1].Verb != "two" {
		t.Errorf("structured sweep differs: %#v", report)
	}
}

// TestNamedSweepPreservesSelectionOrder proves that an explicit selection is
// neither expanded to the default nor deduplicated.
// VALIDATES: selected test-unit actions retain their order and repetitions.
// PREVENTS: applying the bare-command expansion or deduplication to named actions.
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
// with the full feature set, race detector, cgo, and concurrency cap.
func TestEachActionPinsArgvAndEnvironment(t *testing.T) {
	tc := fixtureToolchain()
	wantEnvironment := []string{
		"GOCACHE=" + filepath.Join(tc.Root, "cache", "go-cache"),
		"GOLANGCI_LINT_CACHE=" + filepath.Join(tc.Root, "tmp", "golangci-lint-cache"),
		"CGO_ENABLED=1",
		"GOTOOLCHAIN=go1.26.6",
		"GOMAXPROCS=8",
	}

	for _, group := range Table() {
		t.Run(group.Verb, func(t *testing.T) {
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
		nil,
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
