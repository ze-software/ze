package netlab

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const fixtureConfig = "router-id: 192.0.2.1\n"

type processFixture struct {
	t              *testing.T
	render         map[string]string
	create         CommandResult
	validation     CommandResult
	commands       []Command
	beforeCreate   func(Command)
	beforeValidate func(Command)
}

func (p *processFixture) execute(command Command) CommandResult {
	p.t.Helper()
	copyOfCommand := command
	copyOfCommand.Argv = append([]string(nil), command.Argv...)
	copyOfCommand.Env = append([]string(nil), command.Env...)
	p.commands = append(p.commands, copyOfCommand)

	if len(command.Argv) == 2 && command.Argv[1] == "create" {
		if p.beforeCreate != nil {
			p.beforeCreate(command)
		}
		if p.create.Code != 0 || p.create.Err != nil || strings.Contains(p.create.Stdout, "Errors encountered") {
			return p.create
		}
		for node, body := range p.render {
			path := filepath.Join(command.Dir, "node_files", node, "ze")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				p.t.Fatalf("create rendered node directory: %v", err)
			}
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				p.t.Fatalf("create rendered configuration: %v", err)
			}
		}
		return p.create
	}

	if len(command.Argv) == 4 && command.Argv[1] == "config" && command.Argv[2] == "validate" {
		if p.beforeValidate != nil {
			p.beforeValidate(command)
		}
		return p.validation
	}
	p.t.Fatalf("unexpected process invocation: %#v", command)
	return CommandResult{Code: -1, Err: errors.New("unexpected process invocation")}
}

type checkerFixture struct {
	checker     *Checker
	process     *processFixture
	root        string
	netlab      string
	ze          string
	temporary   string
	removeCalls int
}

func newCheckerFixture(t *testing.T, rendered string) *checkerFixture {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/repository")); err != nil {
		t.Fatalf("copy repository fixture: %v", err)
	}

	netlab := filepath.Join(root, "tools", "netlab")
	ze := filepath.Join(root, "bin", "ze")
	writeFixtureFile(t, netlab, "netlab fixture\n", 0o755)
	writeFixtureFile(t, ze, "ze fixture\n", 0o644)

	process := &processFixture{t: t, render: map[string]string{"r1": rendered}}
	checker := newChecker(root)
	checker.Env = []string{"PATH=/fixture/path", "FIXTURE=value", "NETLAB=" + netlab, "ZE_BIN=" + ze}
	checker.TempParent = t.TempDir()
	checker.Execute = process.execute

	fixture := &checkerFixture{checker: checker, process: process, root: root, netlab: netlab, ze: ze}
	makeTemporary := checker.FS.MkdirTemp
	checker.FS.MkdirTemp = func(parent, pattern string) (string, error) {
		path, err := makeTemporary(parent, pattern)
		fixture.temporary = path
		return path, err
	}
	removeAll := checker.FS.RemoveAll
	checker.FS.RemoveAll = func(path string) error {
		fixture.removeCalls++
		return removeAll(path)
	}
	return fixture
}

func writeFixtureFile(t *testing.T, path, body string, mode fs.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}

func (f *checkerFixture) assertCleaned(t *testing.T) {
	t.Helper()
	if f.temporary == "" {
		t.Fatal("checker did not create a temporary lab")
	}
	if f.removeCalls != 1 {
		t.Errorf("temporary lab removed %d times, want exactly once", f.removeCalls)
	}
	if _, err := os.Stat(f.temporary); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("temporary lab still exists after the check: %v", err)
	}
}

// VALIDATES: NETLAB overrides must name an executable file, and absence fails
// before rendering rather than becoming a skipped check.
func TestMissingNetlabExecutableFailsLoudly(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	if err := os.Chmod(fixture.netlab, 0o644); err != nil {
		t.Fatalf("make netlab fixture non-executable: %v", err)
	}

	report, code := fixture.checker.Run(false)
	if code != 1 {
		t.Errorf("missing executable answered %d, want 1", code)
	}
	want := fmt.Sprintf("error: NETLAB=%s is not an executable file\n", fixture.netlab)
	if report.errorText() != want {
		t.Errorf("diagnostic mismatch:\ngot  %q\nwant %q", report.errorText(), want)
	}
	if len(fixture.process.commands) != 0 {
		t.Errorf("missing executable still started processes: %#v", fixture.process.commands)
	}
	if fixture.temporary != "" || fixture.removeCalls != 0 {
		t.Errorf("missing executable created or cleaned a lab: temporary=%q removals=%d", fixture.temporary, fixture.removeCalls)
	}
}

// VALIDATES: an unset override searches PATH and absence carries the install
// guidance instead of silently skipping the render.
func TestNetlabAbsentFromPATHCarriesInstallGuidance(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	fixture.checker.Env = []string{"PATH=" + filepath.Join(fixture.root, "missing")}

	report, code := fixture.checker.Run(false)
	if code != 1 {
		t.Errorf("absent PATH executable answered %d, want 1", code)
	}
	for _, want := range []string{
		"error: netlab not found on PATH\n",
		"Install networklab from https://netlab.tools/install/",
		"NETLAB=/path/to/netlab",
	} {
		if !strings.Contains(report.errorText(), want) {
			t.Errorf("absence diagnostic does not contain %q:\n%s", want, report.errorText())
		}
	}
	if len(fixture.process.commands) != 0 {
		t.Errorf("absent executable still started processes: %#v", fixture.process.commands)
	}
}

// VALIDATES: the render process receives the exact executable, argv, inherited
// environment and scratch cwd, and its failure is returned after one cleanup.
func TestRenderFailurePreservesProcessAndCleanupSemantics(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	fixture.process.create = CommandResult{Stdout: "render stdout\n", Stderr: "render stderr\n", Code: 23}

	report, code := fixture.checker.Run(false)
	if code != 1 {
		t.Errorf("render failure answered %d, want 1", code)
	}
	if len(fixture.process.commands) != 1 {
		t.Fatalf("render failure started %d processes, want 1", len(fixture.process.commands))
	}
	command := fixture.process.commands[0]
	if !reflect.DeepEqual(command.Argv, []string{fixture.netlab, "create"}) {
		t.Errorf("render argv = %#v", command.Argv)
	}
	if !reflect.DeepEqual(command.Env, fixture.checker.Env) {
		t.Errorf("render environment = %#v, want %#v", command.Env, fixture.checker.Env)
	}
	if command.Dir != filepath.Join(fixture.temporary, "lab") {
		t.Errorf("render cwd = %q, want temporary lab", command.Dir)
	}
	wantError := "render stdout\n\nrender stderr\n\n" +
		"error: `netlab create` failed on contrib/netlab/topology.yml\n" +
		"  Every module the topology declares needs a daemon_config key in\n" +
		"  contrib/netlab/ze.yml and a template in contrib/netlab/ze/.\n"
	if report.errorText() != wantError {
		t.Errorf("render diagnostic mismatch:\ngot:\n%s\nwant:\n%s", report.errorText(), wantError)
	}
	fixture.assertCleaned(t)
}

// VALIDATES: difflib keeps each source line's own terminator. In particular,
// it does not invent a newline for a rendered last line that has none.
func TestUnifiedDiffPreservesMissingFinalNewline(t *testing.T) {
	got := unifiedDiffText(linesKeepingEnds("old\nlast"), linesKeepingEnds("old\nnew"),
		"golden/r1.conf", "rendered/r1")
	want := "--- golden/r1.conf\n+++ rendered/r1\n@@ -1,2 +1,2 @@\n old\n-last+new"
	if got != want {
		t.Errorf("diff mismatch:\ngot  %q\nwant %q", got, want)
	}
}

// VALIDATES: drift reports Python-compatible unified-diff bytes, remains a
// failing verdict, and does not suppress validation of the golden evidence.
func TestGoldenDriftFailsAndStillValidatesTheGolden(t *testing.T) {
	fixture := newCheckerFixture(t, "router-id: 198.51.100.8\n")

	report, code := fixture.checker.Run(false)
	if code != 1 || report.Problems != 1 || report.Clean {
		t.Errorf("drift answered code=%d problems=%d clean=%v", code, report.Problems, report.Clean)
	}
	for _, want := range []string{
		"FAIL: contrib/netlab/golden/r1.conf does not match the render\n",
		"--- golden/r1.conf\n+++ rendered/r1\n@@ -1 +1 @@\n-router-id: 192.0.2.1\n+router-id: 198.51.100.8\n",
		"./le netlab render-check FAILED (1 problem(s))",
		"run `./le netlab render-update` and review the diff",
	} {
		if !strings.Contains(report.errorText(), want) {
			t.Errorf("drift report does not contain %q:\n%s", want, report.errorText())
		}
	}
	if len(fixture.process.commands) != 2 {
		t.Fatalf("drift started %d processes, want render and validation", len(fixture.process.commands))
	}
	wantValidate := []string{fixture.ze, "config", "validate", filepath.Join(fixture.root, "contrib", "netlab", "golden", "r1.conf")}
	if !reflect.DeepEqual(fixture.process.commands[1].Argv, wantValidate) {
		t.Errorf("validation argv = %#v, want %#v", fixture.process.commands[1].Argv, wantValidate)
	}
	fixture.assertCleaned(t)
}

// VALIDATES: a daemon rejection is counted independently of render drift and
// includes both captured process streams in the diagnostic.
func TestGoldenValidationFailureFailsTheCheck(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	fixture.process.validation = CommandResult{Stdout: "validator stdout\n", Stderr: "validator stderr\n", Code: 2}

	report, code := fixture.checker.Run(false)
	if code != 1 || report.Problems != 1 || report.Clean {
		t.Errorf("validation failure answered code=%d problems=%d clean=%v", code, report.Problems, report.Clean)
	}
	for _, want := range []string{
		"is not valid ze configuration",
		"validator stdout\nvalidator stderr\n",
		"./le netlab render-check FAILED (1 problem(s))",
	} {
		if !strings.Contains(report.errorText(), want) {
			t.Errorf("validation report does not contain %q:\n%s", want, report.errorText())
		}
	}
	fixture.assertCleaned(t)
}

// VALIDATES: the complete native workflow prepares the isolated topology,
// removes it before validation, reports both successful checks and exits zero.
func TestAllCleanRunsTheCompleteNativeWorkflow(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	fixture.checker.Env = []string{"PATH=" + filepath.Dir(fixture.netlab), "FIXTURE=value"}
	fixture.process.beforeCreate = func(command Command) {
		defaults, err := os.ReadFile(filepath.Join(command.Dir, "topology-defaults.yml"))
		if err != nil {
			t.Fatalf("read native daemon defaults: %v", err)
		}
		if !strings.Contains(string(defaults), "daemons:") || !strings.Contains(string(defaults), "ze:") {
			t.Errorf("daemon defaults are not wrapped under daemons.ze:\n%s", defaults)
		}
		for _, path := range []string{
			filepath.Join(command.Dir, "topology.yml"),
			filepath.Join(command.Dir, "templates", "ze", "ze.j2"),
		} {
			if _, err := os.Stat(path); err != nil {
				t.Errorf("scratch lab omitted %s: %v", path, err)
			}
		}
	}
	fixture.process.beforeValidate = func(Command) {
		if _, err := os.Stat(fixture.temporary); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("validation began before scratch cleanup: %v", err)
		}
	}

	report, code := fixture.checker.Run(false)
	if code != 0 || !report.Clean || report.Problems != 0 {
		t.Errorf("clean workflow answered code=%d clean=%v problems=%d", code, report.Clean, report.Problems)
	}
	if !reflect.DeepEqual(report.Nodes, []string{"r1"}) {
		t.Errorf("nodes = %#v, want r1", report.Nodes)
	}
	wantText := fmt.Sprintf("netlab: %s\n", fixture.netlab) +
		"ok: contrib/netlab/golden/r1.conf matches the render\n" +
		"ok: contrib/netlab/golden/r1.conf validates\n" +
		"\n./le netlab render-check OK (1 node(s))\n"
	if report.Text() != wantText {
		t.Errorf("clean output mismatch:\ngot:\n%s\nwant:\n%s", report.Text(), wantText)
	}
	if report.errorText() != "" {
		t.Errorf("clean workflow wrote diagnostics: %q", report.errorText())
	}
	if len(fixture.process.commands) != 2 {
		t.Fatalf("clean workflow started %d processes, want 2", len(fixture.process.commands))
	}
	validation := fixture.process.commands[1]
	if validation.Dir != "" {
		t.Errorf("validation cwd = %q, want caller cwd", validation.Dir)
	}
	if !reflect.DeepEqual(validation.Env, fixture.checker.Env) {
		t.Errorf("validation environment = %#v, want %#v", validation.Env, fixture.checker.Env)
	}
	fixture.assertCleaned(t)
}

// VALIDATES: update mode rewrites the golden from the isolated render and then
// validates that updated path, preserving the legacy maintenance workflow.
func TestUpdateRewritesThenValidatesTheGolden(t *testing.T) {
	const updated = "router-id: 203.0.113.9\n"
	fixture := newCheckerFixture(t, updated)

	report, code := fixture.checker.Run(true)
	if code != 0 || !report.Clean || !report.Updated {
		t.Errorf("update answered code=%d clean=%v updated=%v", code, report.Clean, report.Updated)
	}
	path := filepath.Join(fixture.root, "contrib", "netlab", "golden", "r1.conf")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read updated golden: %v", err)
	}
	if string(body) != updated {
		t.Errorf("updated golden = %q, want %q", body, updated)
	}
	if !strings.Contains(report.Text(), "updated contrib/netlab/golden/r1.conf\n") {
		t.Errorf("update output omitted rewritten path:\n%s", report.Text())
	}
	fixture.assertCleaned(t)
}

// VALIDATES: an FS cleanup failure is visible, exits one, is attempted exactly
// once and stops before a golden can be compared or validated.
func TestCleanupFailureIsNotRetriedOrHidden(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	fixture.checker.FS.RemoveAll = func(string) error {
		fixture.removeCalls++
		return errors.New("fixture cleanup refused")
	}

	report, code := fixture.checker.Run(false)
	if code != 1 {
		t.Errorf("cleanup failure answered %d, want 1", code)
	}
	if fixture.removeCalls != 1 {
		t.Errorf("cleanup attempted %d times, want exactly once", fixture.removeCalls)
	}
	if !strings.Contains(report.errorText(), "fixture cleanup refused") {
		t.Errorf("cleanup diagnostic omitted cause: %s", report.errorText())
	}
	if len(fixture.process.commands) != 1 {
		t.Errorf("cleanup failure started %d processes, want only render", len(fixture.process.commands))
	}
}

// VALIDATES: the native check and golden rewrite are zero-argument leactions.
// The listing distinguishes the read-only proof from the write.
func TestRenderActionsAreNativeAndZeroArgument(t *testing.T) {
	checkCalls, updateCalls := 0, 0
	command := actionTable(
		func() (any, int) {
			checkCalls++
			return &Report{Clean: true}, 7
		},
		func() (any, int) {
			updateCalls++
			return &Report{Clean: true, Updated: true}, 8
		},
	)
	list := command.Actions()
	if len(list.Actions) != 2 {
		t.Fatalf("action table has %d rows, want 2", len(list.Actions))
	}
	check, update := list.Actions[0], list.Actions[1]
	if check.Verb != "render-check" || check.Writes {
		t.Errorf("native check action row = %#v", check)
	}
	if update.Verb != "render-update" || !update.Writes {
		t.Errorf("native update action row = %#v", update)
	}
	if command.Subs() != "render-check | render-update (writes)" {
		t.Errorf("action grammar hint = %q", command.Subs())
	}

	payload, code := command.Answer([]string{"render-check"})
	if code != 7 || checkCalls != 1 || updateCalls != 0 || payload == nil {
		t.Errorf("check answered code=%d check=%d update=%d payload=%#v",
			code, checkCalls, updateCalls, payload)
	}
	payload, code = command.Answer([]string{"render-update"})
	report, ok := payload.(*Report)
	if code != 8 || checkCalls != 1 || updateCalls != 1 || !ok || !report.Updated {
		t.Errorf("update answered code=%d check=%d update=%d payload=%#v",
			code, checkCalls, updateCalls, payload)
	}
	payload, code = command.Answer([]string{"render-update", "extra"})
	if code != 2 || payload != nil || updateCalls != 1 {
		t.Errorf("update argument refusal answered code=%d calls=%d payload=%#v",
			code, updateCalls, payload)
	}
}

// VALIDATES: the answer remains structured for pipe operators while its default
// renderer preserves the producer's stdout separately from diagnostics.
func TestReportIsStructuredAndPreservesStreams(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	report, code := fixture.checker.Run(false)
	if code != 0 {
		t.Fatalf("fixture workflow answered %d: %s", code, report.errorText())
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	for _, key := range []string{`"netlab"`, `"nodes"`, `"problems"`, `"updated"`, `"clean"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("structured report omitted %s: %s", key, raw)
		}
	}
	if strings.Contains(string(raw), "./le netlab render-check OK") {
		t.Errorf("structured report embedded rendered prose: %s", raw)
	}
}
