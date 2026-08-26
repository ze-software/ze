package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ze-software/ze/letools/aisync"
)

// VALIDATES: letools/aisync writes the same tree scripts/dev/skill_sync.sh
// writes, file for file and byte for byte -- and refuses the two inputs the
// script answers success for.
// PREVENTS: a swap (step 14) that repoints `make ze-ai-skills-sync` and
// .claude/hooks/session-start.sh at a command generating a different mirror.
// Every target is gitignored, so a difference here shows up as an agent reading
// stale instructions and never as a diff.
//
// The test checks SIDE EFFECTS instead of stdout because the written tree is the
// tool's complete output. It compares both halves as path-to-content-hash maps.
// It also asserts the absolute file count, so two halves cannot pass after both
// stop early.

// syncSources are the canonical inputs both halves generate from. They exercise
// the three substitutions: a verbatim copy, a .claude/ path the .agents mirror
// repoints, and the {{TOOL}} token.
var syncSources = map[string]string{
	"ai/skills/ze-close.md": "---\nname: ze-close\n---\nread .claude/rules/planning.md\n",
	"ai/skills/ze-spec.md":  "---\nname: ze-spec\n---\nbody\n",
	"ai/agents/ze-work.md":  "---\nname: ze-work\n---\nagent definition\n",
	"ai/INSTRUCTIONS.md":    "# {{TOOL}} instructions\n\n{{TOOL}} reads this.\n",
	"feature-gates.txt":     "ze_bgp on\n",
	"go.mod":                "module parity\n\ngo 1.25\n",
}

// syncFixture builds a git checkout holding the canonical sources.
//
// It is a git repository because the shell half starts with
// `cd "$(git rev-parse --show-toplevel)"`. Without a repository, the command
// would enter this checkout and generate over the developer's tree.
func syncFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}

	root := t.TempDir()
	paths := make([]string, 0, len(syncSources))
	for rel := range syncSources {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(syncSources[rel]), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	git(t, root, "init", "-q", ".")
	return root
}

// runSkillSync runs the shell half inside a fixture and answers its exit code
// and its stdout.
func runSkillSync(t *testing.T, root string, args ...string) (string, int) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash on PATH")
	}
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("repository root: %v", err)
	}

	argv := append([]string{filepath.Join(repo, "scripts", "dev", "skill_sync.sh")}, args...)
	cmd := exec.Command("bash", argv...) //nolint:gosec,noctx // the script under comparison
	cmd.Dir = root
	out, runErr := cmd.Output()
	code := 0
	if runErr != nil {
		var exit *exec.ExitError
		if !asExit(runErr, &exit) {
			t.Fatalf("skill_sync.sh: %v", runErr)
		}
		code = exit.ExitCode()
	}
	return string(out), code
}

// generatedTree answers every generated path under root, mapped to the hash of
// its contents. The canonical sources are excluded: they are the INPUT, and a
// comparison that included them would pass on the fixture alone.
func generatedTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if _, isSource := syncSources[rel]; isSource || strings.HasPrefix(rel, ".git/") {
			return nil
		}
		body, readErr := os.ReadFile(path) //nolint:gosec // a path from a walk of this test's own fixture
		if readErr != nil {
			return readErr
		}
		sum := sha256.Sum256(body)
		tree[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return tree
}

func treePaths(tree map[string]string) []string {
	paths := make([]string, 0, len(tree))
	for path := range tree {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// Both halves, over identical sources, write identical trees.
//
// The count is stated ABSOLUTELY: two skills give three mirrors each, one agent
// gives one, and the two instruction files make nine. A half that wrote four of
// them and a half that wrote the same four would compare equal.
func TestBothHalvesWriteTheSameGeneratedTree(t *testing.T) {
	const generatedFiles = 9

	shellRoot := syncFixture(t)
	if _, code := runSkillSync(t, shellRoot); code != 0 {
		t.Fatalf("skill_sync.sh exited %d", code)
	}
	shellTree := generatedTree(t, shellRoot)

	portRoot := syncFixture(t)
	if _, err := (aisync.Mirror{Root: portRoot}).Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	portTree := generatedTree(t, portRoot)

	if len(shellTree) != generatedFiles {
		t.Fatalf("the shell wrote %d files, want exactly %d: %v",
			len(shellTree), generatedFiles, treePaths(shellTree))
	}
	if len(portTree) != generatedFiles {
		t.Fatalf("the port wrote %d files, want exactly %d: %v",
			len(portTree), generatedFiles, treePaths(portTree))
	}
	equal(t, "the generated paths", treePaths(portTree), treePaths(shellTree))
	for path, sum := range shellTree {
		if portTree[path] != sum {
			t.Errorf("%s differs: the shell wrote %s, the port wrote %s", path, sum, portTree[path])
		}
	}
}

// The check agrees on a tree the OTHER half generated, which is what makes the
// two interchangeable across the swap.
func TestEachHalfChecksTheOtherHalvesTreeClean(t *testing.T) {
	shellRoot := syncFixture(t)
	if _, code := runSkillSync(t, shellRoot); code != 0 {
		t.Fatalf("skill_sync.sh exited %d", code)
	}
	report, err := aisync.Mirror{Root: shellRoot}.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Fresh() {
		t.Errorf("the port calls the shell's own tree stale: %v", report.Stale)
	}

	portRoot := syncFixture(t)
	if _, err := (aisync.Mirror{Root: portRoot}).Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if out, code := runSkillSync(t, portRoot, "--check"); code != 0 {
		t.Errorf("the shell calls the port's own tree stale (exit %d):\n%s", code, out)
	}
}

// An argument neither half declares.
//
// The test deliberately asserts that the SHELL WRITES. Its `case` has no
// default branch, so the typo enters the sync. This row fails when somebody
// repairs the script. That failure shows that the port's refusal is no longer
// the only guard (ai/rules/testing.md).
func TestTheShellSyncsOnAnUnknownFlagAndThePortRefusesIt(t *testing.T) {
	shellRoot := syncFixture(t)
	out, code := runSkillSync(t, shellRoot, "--chekc")
	if code != 0 {
		t.Fatalf("skill_sync.sh --chekc exited %d", code)
	}
	if !strings.HasPrefix(out, "synced ") {
		t.Fatalf("skill_sync.sh --chekc printed %q; this case pins its fall-through to the"+
			" sync branch, so repair the script and delete this case", out)
	}
	if wrote := generatedTree(t, shellRoot); len(wrote) == 0 {
		t.Fatalf("skill_sync.sh --chekc wrote nothing; this case pins that it WRITES," +
			" so repair the script and delete this case")
	}

	payload, exit := aisync.Answer([]string{"chekc"})
	if exit == 0 {
		t.Errorf("the port accepted an unknown verb, answering %v", payload)
	}
}

// A checkout holding no skill at all.
//
// The shell answers "synced 0 skill(s) + 0 agent(s) + CLAUDE.md + AGENTS.md"
// and exits 0, naming two files it did not write. Pinned for the same reason as
// the case above.
func TestTheShellReportsSuccessOverNoSourceAndThePortRefuses(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "-q", ".")

	out, code := runSkillSync(t, root)
	if code != 0 || !strings.Contains(out, "synced 0 skill(s)") {
		t.Fatalf("skill_sync.sh answered %q with code %d; this case pins its fail-open,"+
			" so repair the script and delete this case", out, code)
	}
	if !strings.Contains(out, "CLAUDE.md") {
		t.Errorf("the shell no longer names the files it did not write: %q", out)
	}

	if _, err := (aisync.Mirror{Root: root}).Sync(); err == nil {
		t.Error("the port synced a checkout holding no canonical source")
	}
}
