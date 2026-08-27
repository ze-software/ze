// The migration's proof for the testing-health gates: the script and the
// command answer the same thing.
//
// VALIDATES: spec-le-is-a-ze-binary AC-8 and AC-11. The fixture checkout has
// every input used by the nine collectors. testing_health.py writes the same
// bytes as each `le test-health <verb>` command. Each verb also answers the
// exit code from its gate.
// PREVENTS: a port that renders the same page from different numbers. A
// staleness gate compares three of these COMMITTED artifacts byte for byte. One
// changed digit or reordered key fails `ze-test-health-check` for every session
// until regeneration. The two halves run together until the migration swaps
// them.
//
// The WHOLE-CHECKOUT comparison is also here and deliberately accepts the
// current tree state. Several sessions share this checkout. The page need not
// be current, and the RFC ledger need not partition. Both halves must give the
// SAME answer for that state. Thus, the case compares only the exit code and
// text.
//
// It also pins the three fail-open defects that the port FIXED but the script
// still has. Each case asserts that the SCRIPT still fails open. When somebody
// repairs the script, the case fails and must be deleted with its script.
//
// Helpers carry a healthPy prefix. Several steps are porting into this same
// package, and two helpers of one name cannot both exist.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/testhealth"
)

// healthScript is the tool this file compares against.
const healthScript = "testing_health.py"

// healthDetector is the sensitivity detector the SCRIPT half starts as a
// subprocess, at the path it starts it from.
const healthDetector = "scripts/checks/inert_tests.go"

// healthArtifacts are the files that both halves must leave identical. Readers
// and the website use the first two. The last two are ratchet floors. Floors
// that move differently can make a ratchet fire for only one half.
var healthArtifacts = [...]string{
	testhealth.Page,
	testhealth.Latest,
	testhealth.Baseline,
	testhealth.QualityBaseline,
}

// healthPyTree writes a fixture checkout carrying every input the collectors
// read, commits it, and answers its root.
//
// The fixture is COMMITTED instead of only staged because one collector groups
// each package by its first commit date. Without a commit, there is no history
// or group. Both halves then refuse before the comparison.
func healthPyTree(t *testing.T, extra map[string]string) string {
	t.Helper()

	files := healthFixture()
	maps.Copy(files, extra)

	root := t.TempDir()
	for rel, body := range files {
		healthWrite(t, root, rel, body)
	}

	// The detector the script starts as a subprocess. The command calls
	// internal/le/testsensitivity instead, so this copy is what the SCRIPT half
	// needs and the two halves must agree over it. A case that supplies its own
	// keeps it: one of them stands a stub in for the detector.
	if _, supplied := files[healthDetector]; !supplied {
		source, err := os.ReadFile(filepath.Join(devPyRoot(t), "scripts", "checks", "inert_tests.go")) // #nosec G304 -- a tracked script path
		if err != nil {
			t.Fatalf("reading inert_tests.go: %v", err)
		}
		healthWrite(t, root, healthDetector, string(source))
	}

	healthGit(t, root, "init", "--quiet")
	healthGit(t, root, "add", "--all")
	healthGit(t, root, "-c", "user.email=test@example.invalid", "-c", "user.name=test",
		"commit", "--quiet", "-m", "fixture")
	return root
}

// healthWrite puts one file into a fixture tree.
func healthWrite(t *testing.T, root, rel, body string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("fixture file %s: %v", rel, err)
	}
}

// healthGit runs one git command in the fixture.
func healthGit(t *testing.T, root string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// healthRead answers one file of a fixture tree.
func healthRead(t *testing.T, root, rel string) string {
	t.Helper()

	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) // #nosec G304 -- a fixture path
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(body)
}

// healthScriptRun runs THIS checkout's copy of the script against the tree
// given. The script takes its tree from --root, so one copy judges any tree.
func healthScriptRun(t *testing.T, tree string, args ...string) devPyResult {
	t.Helper()
	return healthScriptRunWithPath(t, tree, "", args...)
}

// healthScriptRunWithPath runs the script with a PATH prefix.
// A case uses this prefix to make one git query fail but leaves the other queries intact.
func healthScriptRunWithPath(t *testing.T, tree, pathPrefix string, args ...string) devPyResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), devPyTimeout)
	defer cancel()

	argv := append([]string{
		filepath.Join(devPyRoot(t), "scripts", "dev", healthScript), "--root", tree,
	}, args...)
	cmd := exec.CommandContext(ctx, "python3", argv...) // #nosec G204 -- a tracked script path and a test's own arguments
	cmd.Dir = devPyRoot(t)
	if pathPrefix != "" {
		cmd.Env = append(os.Environ(), healthPathEnv(pathPrefix))
	}
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()

	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("running %s: %v: %s", healthScript, err, errOut.String())
	}
	return devPyResult{Stdout: out.String(), Stderr: errOut.String(), Code: cmd.ProcessState.ExitCode()}
}

// healthPathEnv builds the PATH the shimmed run uses.
func healthPathEnv(prefix string) string {
	var tb strings.Builder
	tb.WriteString("PATH=")
	tb.WriteString(prefix)
	tb.WriteString(string(os.PathListSeparator))
	tb.WriteString(os.Getenv("PATH"))
	return tb.String()
}

// healthCommand runs one `le test-health` action through the binary path.
// leroot.Run splits the pipe chain, calls the tool, and renders the payload.
// This code does not implement that rendering again.
func healthCommand(t *testing.T, tree string, args ...string) devPyResult {
	t.Helper()

	devPyPointAt(t, tree)
	return devPyRunCommand(t, "test-health", testhealth.Answer, args)
}

func TestTestHealthBothHalvesWriteTheSameFourArtifacts(t *testing.T) {
	script, command := healthPyTree(t, nil), healthPyTree(t, nil)

	scriptRun := healthScriptRun(t, script, "--write")
	commandRun := healthCommand(t, command, "update")

	devPyAgree(t, "test-health update", scriptRun, commandRun, scriptRun.Stdout, commandRun.Stdout)
	for _, rel := range healthArtifacts {
		if left, right := healthRead(t, script, rel), healthRead(t, command, rel); left != right {
			t.Errorf("%s differs between the two halves\nscript:\n%s\ncommand:\n%s", rel, left, right)
		}
	}

	// Two halves that both write nothing would satisfy the comparison. Thus, the
	// test checks that the artifacts contain the fixture's expected numbers. The
	// proof density follows the stated RFC arithmetic: five gated requirements,
	// with one requirement proven by a pair.
	page := healthRead(t, script, testhealth.Page)
	for _, want := range [...]string{"# Testing Health", "## Needs attention", "## Trends"} {
		if !strings.Contains(page, want) {
			t.Errorf("the generated page does not carry %q:\n%s", want, page)
		}
	}
	for _, half := range [...]string{script, command} {
		if got := healthMetric(t, half, "rfc-proof-density")["value"]; got != "1 / 5" {
			t.Errorf("the proof density reads %v, want the fixture's 1 / 5", got)
		}
		if got := healthMetric(t, half, "known-failures")["value"]; got != "1" {
			t.Errorf("the live known-failure count reads %v, want the fixture's 1", got)
		}
	}
}

// healthMetric answers one metric of a written snapshot.
func healthMetric(t *testing.T, root, key string) map[string]any {
	t.Helper()

	var document struct {
		Metrics []map[string]any `json:"metrics"`
	}
	if err := json.Unmarshal([]byte(healthRead(t, root, testhealth.Latest)), &document); err != nil {
		t.Fatalf("reading the snapshot: %v", err)
	}
	for _, metric := range document.Metrics {
		if metric["key"] == key {
			return metric
		}
	}
	t.Fatalf("the snapshot carries no %q metric", key)
	return nil
}

func TestTestHealthBothHalvesPassAFreshSnapshot(t *testing.T) {
	script, command := healthPyTree(t, nil), healthPyTree(t, nil)
	healthScriptRun(t, script, "--write")
	healthCommand(t, command, "update")

	scriptRun := healthScriptRun(t, script, "--check")
	commandRun := healthCommand(t, command, "check")

	devPyAgree(t, "test-health check over a fresh snapshot", scriptRun, commandRun,
		scriptRun.Stdout, commandRun.Stdout)
	if scriptRun.Code != 0 {
		t.Errorf("the gate refused a snapshot it had just written: %d\n%s%s",
			scriptRun.Code, scriptRun.Stdout, scriptRun.Stderr)
	}
}

func TestTestHealthBothHalvesRefuseAStaleSnapshotAndNameTheSameFacts(t *testing.T) {
	script, command := healthPyTree(t, nil), healthPyTree(t, nil)
	healthScriptRun(t, script, "--write")
	healthCommand(t, command, "update")
	healthStale(t, script)
	healthStale(t, command)

	scriptRun := healthScriptRun(t, script, "--check")
	commandRun := healthCommand(t, command, "check")

	if scriptRun.Code != 1 || commandRun.Code != 1 {
		t.Errorf("a stale snapshot answered %d (script) and %d (command), want 1 and 1\n%s%s\n%s%s",
			scriptRun.Code, commandRun.Code,
			scriptRun.Stdout, scriptRun.Stderr, commandRun.Stdout, commandRun.Stderr)
	}

	// The script writes its diagnosis to stderr and the command answers it as a
	// payload, so the STREAM differs by design (ai/rules/cli.md: the payload is
	// what `| json` renders). What must not differ is which facts moved.
	scriptText := scriptRun.Stderr
	commandText := commandRun.Stdout
	for _, fact := range [...]string{"statuses", "tag-orphans", "rfc-unproven"} {
		if !strings.Contains(scriptText, fact) || !strings.Contains(commandText, fact) {
			t.Errorf("the fact %q is named by only one half\nscript:\n%s\ncommand:\n%s",
				fact, scriptText, commandText)
		}
	}
	for _, moved := range [...]string{"rfc1001", "internal/alpha/orphan_test.go", "inventory"} {
		if !strings.Contains(scriptText, moved) || !strings.Contains(commandText, moved) {
			t.Errorf("the entry %q is named by only one half\nscript:\n%s\ncommand:\n%s",
				moved, scriptText, commandText)
		}
	}
}

// healthStale changes a written snapshot so every gated fact moves. It keeps
// each list consistent with its counter because the cross-check refuses a list
// with a different count. This case tests the COMPARISON, not that guard.
func healthStale(t *testing.T, root string) {
	t.Helper()

	var document struct {
		Metrics []map[string]any `json:"metrics"`
	}
	if err := json.Unmarshal([]byte(healthRead(t, root, testhealth.Latest)), &document); err != nil {
		t.Fatalf("reading the snapshot: %v", err)
	}
	for _, metric := range document.Metrics {
		switch metric["key"] {
		case "rfc-unproven":
			metric["unproven_rfcs"] = []any{}
			unproven, _ := metric["unproven"].(map[string]any)
			unproven["numerator"] = 0
		case "tag-orphan":
			metric["orphans"] = []any{}
			metric["orphan_count"] = 0
		case "inventory":
			metric["status"] = "warn"
		}
	}
	body, err := json.MarshalIndent(map[string]any{"metrics": document.Metrics}, "", "  ")
	if err != nil {
		t.Fatalf("writing the snapshot: %v", err)
	}
	healthWrite(t, root, testhealth.Latest, string(body)+"\n")
}

func TestTestHealthBothHalvesRefuseAMissingBaseline(t *testing.T) {
	script, command := healthPyTree(t, nil), healthPyTree(t, nil)
	for _, root := range [...]string{script, command} {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(testhealth.Baseline))); err != nil {
			t.Fatalf("removing the baseline: %v", err)
		}
	}

	scriptRun := healthScriptRun(t, script, "--write")
	commandRun := healthCommand(t, command, "update")

	if scriptRun.Code != 2 || commandRun.Code != 2 {
		t.Errorf("a deleted baseline answered %d (script) and %d (command), want 2 and 2",
			scriptRun.Code, commandRun.Code)
	}
	if !strings.Contains(scriptRun.Stderr, "would launder any regression") {
		t.Errorf("the script's refusal does not say why a missing baseline is refused: %q",
			scriptRun.Stderr)
	}
	// The test compares the command diagnosis through the ERROR from the tool,
	// not through the run. leaction.ReportError writes to process stderr instead
	// of the writer given to leroot.Run. Thus, every ported tool has an empty
	// captured stream.
	if _, err := testhealth.Write(command, false); err == nil ||
		!strings.Contains(err.Error(), "would launder any regression") {
		t.Errorf("the command's refusal does not say why a missing baseline is refused: %v", err)
	}
	if healthExists(filepath.Join(command, filepath.FromSlash(testhealth.Baseline))) {
		t.Errorf("the command minted %s from today's counts", testhealth.Baseline)
	}
}

// healthExists reports whether a path is there.
func healthExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestTestHealthBothHalvesAppendTheSameSample(t *testing.T) {
	script, command := healthPyTree(t, nil), healthPyTree(t, nil)

	scriptRun := healthScriptRun(t, script, "--record")
	commandRun := healthCommand(t, command, "record")

	devPyAgree(t, "test-health record", scriptRun, commandRun, scriptRun.Stdout, commandRun.Stdout)

	// The timestamp uses wall-clock time, and the SHA is the fixture commit. The
	// two rows must agree on every other field. Otherwise, they are not the same
	// KPI measurement.
	left := healthSample(t, script)
	right := healthSample(t, command)
	delete(left, "ts")
	delete(right, "ts")
	delete(left, "sha")
	delete(right, "sha")
	if !healthSameRow(left, right) {
		t.Errorf("the two halves recorded different samples\nscript:  %v\ncommand: %v", left, right)
	}
	if len(left) < 8 {
		t.Errorf("the sample carries %d field(s), so it records almost nothing: %v", len(left), left)
	}
	for _, rel := range healthArtifacts {
		if a, b := healthRead(t, script, rel), healthRead(t, command, rel); a != b {
			t.Errorf("%s differs after recording\nscript:\n%s\ncommand:\n%s", rel, a, b)
		}
	}
}

func TestTestHealthBothHalvesSkipASampleIdenticalToTheLast(t *testing.T) {
	script, command := healthPyTree(t, nil), healthPyTree(t, nil)
	healthScriptRun(t, script, "--record")
	healthCommand(t, command, "record")

	scriptRun := healthScriptRun(t, script, "--record")
	commandRun := healthCommand(t, command, "record")

	devPyAgree(t, "a second identical sample", scriptRun, commandRun,
		scriptRun.Stdout, commandRun.Stdout)
	if !strings.Contains(scriptRun.Stdout, "nothing appended") {
		t.Errorf("the script appended a duplicate sample: %q", scriptRun.Stdout)
	}
	for _, root := range [...]string{script, command} {
		rows := strings.Count(strings.TrimSpace(healthRead(t, root, testhealth.History)), "\n") + 1
		if rows != 1 {
			t.Errorf("%s holds %d sample(s) after two identical runs, want 1", testhealth.History, rows)
		}
	}
}

// healthSample answers the last row of a fixture's history.
func healthSample(t *testing.T, root string) map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(healthRead(t, root, testhealth.History)), "\n")
	var row map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &row); err != nil {
		t.Fatalf("reading the sample: %v", err)
	}
	return row
}

// healthSameRow compares two decoded samples field by field.
func healthSameRow(left, right map[string]any) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		other, held := right[key]
		if !held || value != other {
			return false
		}
	}
	return true
}

func TestTestHealthBothHalvesAgreeOverAnExportOfThisCheckout(t *testing.T) {
	// This case adds SCALE to the fixture coverage. The fixture has five gated
	// requirements and seven test functions. This checkout has thousands. An
	// ordering or rounding difference hidden by the small tree appears here as
	// different bytes in a committed file.
	//
	// The export is a SNAPSHOT instead of the live tree because several sessions
	// share this checkout. Two working-tree reads forty seconds apart can see
	// different trees. The comparison would then attribute another session's
	// edit to a difference between the halves.
	script, command := healthExport(t), healthExport(t)

	scriptRun := healthScriptRun(t, script, "--write")
	commandRun := healthCommand(t, command, "update")

	if scriptRun.Code != commandRun.Code {
		t.Fatalf("over an export of HEAD the script exited %d and the command exited %d\nscript:\n%s%s\ncommand:\n%s%s",
			scriptRun.Code, commandRun.Code,
			scriptRun.Stdout, scriptRun.Stderr, commandRun.Stdout, commandRun.Stderr)
	}

	// A collector refusal describes the TREE, not the port.
	// The RFC ledger must partition, but a session mid-extraction leaves it short.
	// Both halves must still refuse for the same reason.
	if scriptRun.Code != 0 {
		if scriptRun.Code != 2 {
			t.Errorf("both halves answered %d, which is not a verdict this tool states", scriptRun.Code)
		}
		diagnosis := healthDiagnosis(t, command)
		if !strings.Contains(scriptRun.Stderr, diagnosis) {
			t.Errorf("the two halves refused for different reasons\nscript:\n%s\ncommand:\n%s",
				scriptRun.Stderr, diagnosis)
		}
		return
	}

	devPyAgree(t, "test-health update over an export of HEAD", scriptRun, commandRun,
		scriptRun.Stdout, commandRun.Stdout)
	for _, rel := range healthArtifacts {
		if left, right := healthRead(t, script, rel), healthRead(t, command, rel); left != right {
			t.Errorf("%s differs between the two halves over the whole checkout", rel)
		}
	}
	// The comparison would be satisfied by two halves that both measured a
	// nearly empty tree, so the scale it exists to reach is asserted.
	if got := healthCount(t, command, "assert-nothing", "inert", "denominator"); got < 1000 {
		t.Errorf("the export scanned %v test function(s), which is not this repository", got)
	}
}

// healthExport writes a snapshot of HEAD into a temporary tree and makes it a
// repository of its own.
//
// Without the second step, the export is a directory INSIDE this checkout.
// Both halves would ask the parent repository what it tracks.
// They would answer with the current edits from other sessions.
func healthExport(t *testing.T) string {
	t.Helper()

	// os.MkdirTemp rather than t.TempDir: the export becomes a repository, and
	// removing a .git tree can race git's own housekeeping. A cleanup that
	// cannot finish must not fail a comparison that already ran.
	root, err := os.MkdirTemp("", "health-export")
	if err != nil {
		t.Fatalf("export directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })

	ctx, cancel := context.WithTimeout(t.Context(), devPyTimeout)
	defer cancel()

	archive := exec.CommandContext(ctx, "git", "-C", devPyRoot(t), "archive", "HEAD") // #nosec G204 -- this checkout's own path
	extract := exec.CommandContext(ctx, "tar", "-x", "-C", root)                      // #nosec G204 -- a temporary directory
	pipe, err := archive.StdoutPipe()
	if err != nil { //nolint:govet // the shadow is the pipe error, reported here
		t.Fatalf("exporting HEAD: %v", err)
	}
	extract.Stdin = pipe
	var failure bytes.Buffer
	archive.Stderr = &failure
	if err := extract.Start(); err != nil {
		t.Fatalf("extracting the export: %v", err)
	}
	if err := archive.Run(); err != nil {
		t.Fatalf("exporting HEAD: %v: %s", err, failure.String())
	}
	if err := extract.Wait(); err != nil {
		t.Fatalf("extracting the export: %v", err)
	}

	healthGit(t, root, "init", "--quiet")
	healthGit(t, root, "add", "--all")
	healthGit(t, root, "-c", "user.email=test@example.invalid", "-c", "user.name=test",
		"commit", "--quiet", "-m", "export of HEAD")
	return root
}

// healthDiagnosis answers the message the command's own collectors refuse with.
//
// The run's stderr does not contain that message.
// leaction.ReportError writes to the process stderr instead of the writer that leroot.Run received.
// Thus, the captured stream is empty for every ported tool.
func healthDiagnosis(t *testing.T, tree string) string {
	t.Helper()

	_, err := testhealth.Write(tree, false)
	if err == nil {
		t.Fatalf("the command refused through its action and not through its function")
	}
	message := err.Error()
	if index := strings.Index(message, "\n"); index >= 0 {
		message = message[index+1:]
	}
	return message
}

// ─── The fail-open defects the port FIXED and the script still carries ──────

func TestScriptTestHealthStillRecordsAnUnknownShaWhenGitCannotName(t *testing.T) {
	script, command := healthPyTree(t, nil), healthPyTree(t, nil)
	shim := healthGitShim(t)

	scriptRun := healthScriptRunWithPath(t, script, shim, "--record")
	if scriptRun.Code != 0 {
		t.Fatalf("the script refused a HEAD git could not name, so this row is fixed: %d\n%s%s",
			scriptRun.Code, scriptRun.Stdout, scriptRun.Stderr)
	}
	if sha := healthSample(t, script)["sha"]; sha != "unknown" {
		t.Errorf("the script recorded the sha %v, so this row is fixed and belongs with the script", sha)
	}

	// The port refuses instead: a sample nobody can attribute to a commit
	// lands in an append-only file, and the file is the only record.
	t.Setenv("PATH", strings.TrimPrefix(healthPathEnv(shim), "PATH="))
	commandRun := healthCommand(t, command, "record")
	if commandRun.Code == 0 {
		t.Errorf("the command recorded a sample against a HEAD git could not name: %q", commandRun.Stdout)
	}
	if healthExists(filepath.Join(command, filepath.FromSlash(testhealth.History))) {
		t.Errorf("the command appended to %s before refusing", testhealth.History)
	}
}

// healthGitShim answers a directory holding a `git` that fails for
// `rev-parse --short HEAD` and passes everything else through.
//
// Only a git stand-in CAN fail one query while the nine collectors continue to use git.
// The defect is visible only in that state.
func healthGitShim(t *testing.T) string {
	t.Helper()

	real, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("no git on PATH: %v", err)
	}
	dir := t.TempDir()
	var tb strings.Builder
	tb.WriteString("#!/bin/sh\n")
	tb.WriteString("for arg in \"$@\"; do\n")
	tb.WriteString("  if [ \"$arg\" = \"--short\" ]; then exit 128; fi\n")
	tb.WriteString("done\n")
	tb.WriteString("exec ")
	tb.WriteString(real)
	tb.WriteString(" \"$@\"\n")
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(tb.String()), 0o700); err != nil { // #nosec G302 -- the shim must be executable
		t.Fatalf("writing the git shim: %v", err)
	}
	return dir
}

func TestScriptTestHealthStillAcceptsAQualityFloorThatIsNotANumber(t *testing.T) {
	const floors = "test/health/quality-baseline.json"
	silenced := healthPyTree(t, map[string]string{floors: "{\n  \"mutation\": false\n}\n"})
	honest := healthPyTree(t, map[string]string{floors: "{\n  \"mutation\": 90.0\n}\n"})
	command := healthPyTree(t, map[string]string{floors: "{\n  \"mutation\": false\n}\n"})

	// The fixture kills 15 of 20 mutants, so a floor of 90 is a regression the
	// page must flag. That run is what makes the boolean run meaningful: the
	// signal exists, and the boolean is what removes it.
	if run := healthScriptRun(t, honest, "--write"); run.Code != 0 {
		t.Fatalf("the script refused a numeric floor: %d\n%s", run.Code, run.Stderr)
	}
	if got := healthMetric(t, honest, "mutation")["status"]; got != "warn" {
		t.Fatalf("a floor of 90 over a 75%% kill rate read %v, so this case proves nothing", got)
	}

	run := healthScriptRun(t, silenced, "--write")
	if run.Code != 0 {
		t.Fatalf("the script refused a boolean quality floor, so this row is fixed: %d\n%s",
			run.Code, run.Stderr)
	}
	// `false` compares as 0, so every percentage clears it: replacing the number
	// with a boolean silences the regression signal and says nothing.
	if got := healthMetric(t, silenced, "mutation")["status"]; got != "ok" {
		t.Errorf("the boolean floor read %v rather than silencing the metric, so this row is fixed", got)
	}

	commandRun := healthCommand(t, command, "update")
	if commandRun.Code == 0 {
		t.Errorf("the command accepted a quality floor that is not a number: %q", commandRun.Stdout)
	}
	if _, err := testhealth.Write(command, false); err == nil ||
		!strings.Contains(err.Error(), "a number is required") {
		t.Errorf("the refusal does not say what is wrong with the floor: %v", err)
	}
}

func TestScriptTestHealthStillReadsAnEmptyInertListWhenTheDetectorRenamesItsKeys(t *testing.T) {
	stub := map[string]string{healthDetector: healthDetectorStub}
	script, command := healthPyTree(t, stub), healthPyTree(t, stub)

	scriptRun := healthScriptRun(t, script, "--write")
	if scriptRun.Code != 0 {
		t.Fatalf("the script refused a detector that renamed its keys, so this row is fixed: %d\n%s",
			scriptRun.Code, scriptRun.Stderr)
	}
	// Zero inert tests and zero stranded files is the GOAL STATE for both
	// counts, and one of them is a fact `check` gates. The script reads them by
	// string key with an empty fallback, so a renamed key publishes the goal
	// state having measured nothing.
	if got := healthCount(t, script, "assert-nothing", "inert", "numerator"); got != 0 {
		t.Errorf("the script reported %v inert test(s), so this row is fixed", got)
	}
	if got := healthMetric(t, script, "tag-orphan")["orphan_count"]; got != float64(0) {
		t.Errorf("the script reported %v stranded file(s), so this row is fixed", got)
	}

	// The port calls the detector as a FUNCTION, so no key CAN be renamed.
	// It has no fallback and reports what the tree holds.
	commandRun := healthCommand(t, command, "update")
	if commandRun.Code != 0 {
		t.Fatalf("the command refused: %d\n%s", commandRun.Code, commandRun.Stderr)
	}
	if got := healthCount(t, command, "assert-nothing", "inert", "numerator"); got != 1 {
		t.Errorf("the command reported %v inert test(s), want the one the fixture carries", got)
	}
	if got := healthMetric(t, command, "tag-orphan")["orphan_count"]; got != float64(1) {
		t.Errorf("the command reported %v stranded file(s), want the one the fixture carries", got)
	}
}

// healthCount answers a nested counter of one metric.
func healthCount(t *testing.T, root, key, part, field string) float64 {
	t.Helper()

	nested, ok := healthMetric(t, root, key)[part].(map[string]any)
	if !ok {
		t.Fatalf("the %q metric carries no %q", key, part)
	}
	value, ok := nested[field].(float64)
	if !ok {
		t.Fatalf("the %q metric's %q carries no numeric %q", key, part, field)
	}
	return value
}

// healthDetectorStub is a detector that answers the same document under
// different key names, which is the shape a rename would leave behind.
const healthDetectorStub = `//go:build ignore

// A stand-in for scripts/checks/inert_tests.go that renames its two finding
// lists. Used by one parity case to show what the script does when the keys it
// reads by name are not the keys the detector writes.
package main

import "fmt"

func main() {
	fmt.Println(` + "`" + `{"assertNothing": [{"file": "internal/alpha/inert_test.go", "test": "TestInert", "line": 1}],
 "tagOrphan": [{"file": "internal/alpha/orphan_test.go", "detail": "ze_missing"}],
 "files-scanned": 5, "tests-scanned": 7, "valid": true}` + "`" + `)
}
`
