// This file proves the migration of the perf NUDGE. scripts/dev/perf-suggest.py
// and `le perf-bench` answer the same result for the same checkout. The record
// verb also writes the same bytes.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 for `perf-suggest.py`. In fixture
// repositories with real commits, both halves name the same uncovered files.
// They use the same order, print the same message, and answer the same exit
// code. They also leave the same marker on disk.
// PREVENTS: a nudge that agrees about a fixture but uses a different filter
// list. HOT_PATH_PREFIXES is a shared constant. A missing prefix is invisible
// when the fixture has no file under that prefix. Thus, the test also compares
// the two lists BY VALUE.
//
// This file is deliberately HERE instead of beside internal/le/perfbench. It is a
// migration artifact, so the commit that deletes the script also deletes its
// proof.
//
// It also pins the fail-open defect that the port FIXED but the script still
// has. A git that cannot answer leaves the script silent. This case asserts that
// the SCRIPT still fails open. When somebody repairs the script, the case fails
// and must be deleted with the script.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/perfbench"
)

const (
	perfScript  = "perf-suggest.py"
	perfCommand = "perf-bench"
	perfGate    = "ze-perf-suggestion-report"
)

// perfHotFile is a path under a measured prefix, and perfColdFile is not. Both
// are Go files, so the only thing telling them apart is the prefix list this
// comparison exists to pin.
const (
	perfHotFile  = "internal/component/bgp/reactor/peer.go"
	perfColdFile = "internal/component/cli/model.go"
)

// perfRepo builds a fixture checkout with one commit, so HEAD exists and a
// committed range can be diffed.
//
// It carries feature-gates.txt because lepath.Root looks for it, and the
// command must resolve the fixture rather than this checkout.
func perfRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	runGit(t, root, "config", "commit.gpgsign", "false")
	runGit(t, root, "config", "core.hooksPath", filepath.Join(root, ".nohooks"))
	writeFixture(t, root, "feature-gates.txt", "ze_x pkg/x\n")
	writeFixture(t, root, ".gitignore", "tmp/\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", "fixture")
	return root
}

// perfBothHalves runs the script and the command over ONE tree and answers what
// each said, with the tree they judged.
//
// The two halves use one tree because two copies caused a flake in this file.
// The message names the baseline COMMIT. Fixture repositories built one second
// apart have different SHAs, so their comparison failed even though neither
// half was wrong. Two copies built in one second have the SAME SHA. Thus, the
// case usually passed but failed at a second boundary.
//
// The record verb writes into the tree, so its side effect is read back BETWEEN
// the two runs rather than from two trees.
func perfBothHalves(t *testing.T, build func(*testing.T) string) (devPyResult, devPyResult, string) {
	t.Helper()

	tree := build(t)
	script := devPyRunScript(t, perfScript, nil, tree)

	devPyPointAt(t, tree)
	command := devPyRunCommand(t, perfCommand, perfbench.Answer, []string{perfGate})

	return script, command, tree
}

// perfAgree compares the two halves.
//
// The script prints its nudge to stderr and nothing to stdout. The command
// answers a PAYLOAD, which leroot renders on stdout. Thus, the streams differ by
// design, and the test compares the TEXT. Each ported evidence tool made the
// same decision for its log.
func perfAgree(t *testing.T, what string, script, command devPyResult) {
	t.Helper()
	devPyAgree(t, what, script, command, script.Stderr, command.Stdout)
}

// TestPerfNudgeBothHalvesNameTheSameWorkingTreeChange is the ordinary case: a
// hot file edited and never committed, with no marker and no upstream.
func TestPerfNudgeBothHalvesNameTheSameWorkingTreeChange(t *testing.T) {
	build := func(t *testing.T) string {
		t.Helper()
		root := perfRepo(t)
		writeFixture(t, root, perfHotFile, "package reactor\n")
		writeFixture(t, root, perfColdFile, "package cli\n")
		return root
	}

	script, command, _ := perfBothHalves(t, build)
	perfAgree(t, "a working-tree change", script, command)

	// A comparison of two silences proves nothing, so the case asserts what it
	// found rather than only that both halves found the same thing.
	if !strings.Contains(command.Stdout, perfHotFile) {
		t.Errorf("the nudge did not name the hot file it was given:\n%s", command.Stdout)
	}
	if strings.Contains(command.Stdout, perfColdFile) {
		t.Errorf("the nudge named a file under no measured prefix:\n%s", command.Stdout)
	}
	if !strings.Contains(command.Stdout, "working tree (perf never recorded here)") {
		t.Errorf("the nudge did not say it has no trusted baseline:\n%s", command.Stdout)
	}
}

// TestPerfNudgeBothHalvesMeasureAgainstTheRecordedRun is the marker case. It
// requires a hot file COMMITTED after the marker to remain named. This is the
// purpose of the baseline.
func TestPerfNudgeBothHalvesMeasureAgainstTheRecordedRun(t *testing.T) {
	build := func(t *testing.T) string {
		t.Helper()
		root := perfRepo(t)
		base := runGit(t, root, "rev-parse", "HEAD")
		writeFixture(t, root, "tmp/.ze-perf-lastrun", base+"\n")
		writeFixture(t, root, perfHotFile, "package reactor\n")
		runGit(t, root, "add", "-A")
		runGit(t, root, "commit", "-q", "-m", "a hot change since perf ran")
		return root
	}

	script, command, _ := perfBothHalves(t, build)
	perfAgree(t, "a commit made since the recorded run", script, command)

	if !strings.Contains(command.Stdout, "last perf run ") {
		t.Errorf("the nudge did not measure against the marker:\n%s", command.Stdout)
	}
	if !strings.Contains(command.Stdout, perfHotFile) {
		t.Errorf("a hot file committed after the marker went unnamed:\n%s", command.Stdout)
	}
}

// TestPerfNudgeBothHalvesGoQuietWhenTheRecordedRunCoversTheWork is the case the
// nudge exists to reach: perf ran at this commit, so there is nothing to say.
func TestPerfNudgeBothHalvesGoQuietWhenTheRecordedRunCoversTheWork(t *testing.T) {
	build := func(t *testing.T) string {
		t.Helper()
		root := perfRepo(t)
		writeFixture(t, root, perfHotFile, "package reactor\n")
		runGit(t, root, "add", "-A")
		runGit(t, root, "commit", "-q", "-m", "a hot change")
		writeFixture(t, root, "tmp/.ze-perf-lastrun", runGit(t, root, "rev-parse", "HEAD")+"\n")
		return root
	}

	script, command, _ := perfBothHalves(t, build)
	perfAgree(t, "work the recorded run covers", script, command)

	if script.Stderr != "" || command.Stdout != "" {
		t.Errorf("the nudge spoke about covered work:\nscript:\n%s\ncommand:\n%s", script.Stderr, command.Stdout)
	}
}

// TestPerfNudgeBothHalvesCountTheFilesTheyDoNotName is the boundary of the
// message: twelve are named and the rest are counted.
func TestPerfNudgeBothHalvesCountTheFilesTheyDoNotName(t *testing.T) {
	const extra = 3
	build := func(t *testing.T) string {
		t.Helper()
		root := perfRepo(t)
		for i := range 12 + extra {
			writeFixture(t, root, perfHotPath(i), "package reactor\n")
		}
		return root
	}

	script, command, _ := perfBothHalves(t, build)
	perfAgree(t, "more files than the message names", script, command)

	if !strings.Contains(command.Stdout, "... and 3 more") {
		t.Errorf("15 uncovered files did not produce a count of the rest:\n%s", command.Stdout)
	}
	if got := strings.Count(command.Stdout, "reactor/hot"); got != 12 {
		t.Errorf("the message names %d files, want 12", got)
	}
}

// TestPerfRecordBothHalvesWriteTheSameMarker tests the side effect. The verdict
// is one word, but the marker contains the bytes that the next run reads.
//
// The test reads the marker between the two runs over one tree. Thus, it
// compares two writes of the same HEAD instead of two repositories that happen
// to agree.
func TestPerfRecordBothHalvesWriteTheSameMarker(t *testing.T) {
	tree := perfRepo(t)
	marker := filepath.Join(tree, "tmp", ".ze-perf-lastrun")

	script := devPyRunScript(t, perfScript, []string{"--record"}, tree)
	fromScript := devPyRead(t, marker)
	if err := os.Remove(marker); err != nil {
		t.Fatalf("clearing the marker between the two halves: %v", err)
	}

	devPyPointAt(t, tree)
	command := devPyRunCommand(t, perfCommand, perfbench.Answer, []string{"record"})
	fromCommand := devPyRead(t, marker)

	devPyAgree(t, "recording a run", script, command, "", "")
	if fromScript != fromCommand {
		t.Errorf("the two halves recorded different markers: %q and %q", fromScript, fromCommand)
	}
	if want := runGit(t, tree, "rev-parse", "HEAD"); strings.TrimSpace(fromCommand) != want {
		t.Errorf("the marker holds %q rather than HEAD %q", fromCommand, want)
	}
	// A comparison of two empty files would pass and prove nothing.
	if len(strings.TrimSpace(fromCommand)) != 40 {
		t.Errorf("the marker holds %q rather than a commit SHA", fromCommand)
	}
}

// TestPerfNudgeSharedConstantsAgreeByValue uses technique 3. A prefix list is
// invisible to an output comparison when its fixture has no file under a
// missing prefix. Thus, the test reads both lists from the two halves and
// compares each element.
func TestPerfNudgeSharedConstantsAgreeByValue(t *testing.T) {
	fromPython := perfPythonConstants(t)

	if !equalStrings(fromPython.Prefixes, perfbench.HotPathPrefixes) {
		t.Errorf("the measured-prefix lists differ:\npython: %v\ngo:     %v",
			fromPython.Prefixes, perfbench.HotPathPrefixes)
	}
	if fromPython.Marker != perfbench.MarkerPath {
		t.Errorf("the marker paths differ: python %q, go %q", fromPython.Marker, perfbench.MarkerPath)
	}
	// A comparison of two empty lists would pass and prove nothing.
	if len(fromPython.Prefixes) < 10 {
		t.Errorf("the script declared %d prefixes, so this comparison is vacuous", len(fromPython.Prefixes))
	}
}

// TestPerfNudgeScriptStillGoesSilentOnAGitItCannotRun pins the fail-open the
// PORT fixed and the script still carries.
//
// Outside a repository, every git query fails. The script interprets a failed
// command as "nothing changed", prints nothing, and exits 0. Its documentation
// identifies under-coverage as the important failure, but the script cannot
// report this cause. The command reports it instead.
//
// This asserts that the SCRIPT still fails open. When somebody repairs it, this
// case fails and must be deleted with the script.
func TestPerfNudgeScriptStillGoesSilentOnAGitItCannotRun(t *testing.T) {
	noRepo := func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		writeFixture(t, root, "feature-gates.txt", "ze_x pkg/x\n")
		writeFixture(t, root, perfHotFile, "package reactor\n")
		return root
	}

	script, command, _ := perfBothHalves(t, noRepo)

	if script.Code != 0 || script.Stderr != "" {
		t.Errorf("the script no longer fails open: exit %d, stderr %q."+
			" Delete this case with the script's fail-open path", script.Code, script.Stderr)
	}
	if command.Code != 0 {
		t.Errorf("the command exited %d; the nudge is advisory and may never block a build", command.Code)
	}
	if !strings.Contains(command.Stdout, "could not read") {
		t.Errorf("the command did not say the checkout was unreadable:\n%s", command.Stdout)
	}
}

// perfHotPath answers the nth distinct hot-path file name.
func perfHotPath(n int) string {
	var name bytes.Buffer
	name.WriteString("internal/component/bgp/reactor/hot")
	name.WriteByte(byte('a' + n))
	name.WriteString(".go")
	return name.String()
}

// perfConstants is what the script declares, read out of the module itself.
type perfConstants struct {
	Prefixes []string `json:"prefixes"`
	Marker   string   `json:"marker"`
}

// perfPythonConstants imports the script as a module and answers the two
// constants both halves must agree about.
//
// Imported rather than parsed: a regular expression over the source would agree
// with a list the module never uses, and the module is what runs.
func perfPythonConstants(t *testing.T) perfConstants {
	t.Helper()

	const reader = `
import importlib.util, json, sys
spec = importlib.util.spec_from_file_location("perf_suggest", sys.argv[1])
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)
print(json.dumps({"prefixes": list(mod.HOT_PATH_PREFIXES), "marker": str(mod.MARKER)}))
`
	ctx, cancel := context.WithTimeout(t.Context(), devPyTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "-c", reader,
		filepath.Join(devPyRoot(t), "scripts", "dev", perfScript))
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("reading the script's constants: %v: %s", err, errOut.String())
	}

	var declared perfConstants
	if err := json.Unmarshal(out, &declared); err != nil {
		t.Fatalf("decoding the script's constants: %v: %s", err, out)
	}
	return declared
}

// equalStrings compares two lists element for element.
func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
