// VALIDATES: `le arch compound-guard check` judges the unpushed range from its
// entry point: a guard on a committed but unpushed line is flagged, a guard at
// the merge base is not, a detached worktree holding no tmp/ still flags it, a
// clean pushed checkout judges nothing, and a checkout with no base exits 2.
// PREVENTS: the gate passing in silence inside `le verify worktree`, whose
// detached worktree holds no verify status file to read a base from.
package archcompoundguard

import (
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// pushedSource holds the guard already at the merge base, on line 4.
const pushedSource = `package p

func old(a, b bool) {
	if a || b {
		return
	}
}
`

// unpushedSource adds a second guard on line 10, in an unpushed commit.
const unpushedSource = pushedSource + `
func added(c, d bool) {
	if c || d {
		return
	}
}
`

// TestCheckJudgesTheUnpushedRange drives runCheck over a checkout whose
// origin/main sits two commits behind HEAD, then over a detached worktree of
// the tip, then over the same checkout once origin/main reaches the tip.
func TestCheckJudgesTheUnpushedRange(t *testing.T) {
	root := unpushedFixture(t)
	want := Findings{{File: "internal/p/p.go", Line: 10, Fn: "added"}}

	report, code := checkAt(t, root)
	if !slices.Equal(report.Findings, want) || code != 1 {
		t.Fatalf("checkout: findings = %v exit %d, want %v exit 1", report.Findings, code, want)
	}

	// The shape `le verify worktree` judges: detached at the tip, no tmp/.
	detached := filepath.Join(t.TempDir(), "verify")
	gitIn(t, root, "worktree", "add", "-q", "--detach", detached, "HEAD")
	report, code = checkAt(t, detached)
	if !slices.Equal(report.Findings, want) || code != 1 {
		t.Fatalf("detached worktree: findings = %v exit %d, want %v exit 1", report.Findings, code, want)
	}

	gitIn(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	report, code = checkAt(t, root)
	if len(report.Findings) != 0 || report.Files != 0 || code != 0 {
		t.Fatalf("pushed checkout: %d finding(s) over %d file(s) exit %d, want none, 0, exit 0",
			len(report.Findings), report.Files, code)
	}
}

// TestCheckWithNoBaseExitsTwo proves a checkout with no upstream and no
// origin/main is a run that judged nothing, never a pass.
func TestCheckWithNoBaseExitsTwo(t *testing.T) {
	root := unpushedFixture(t)
	gitIn(t, root, "update-ref", "-d", "refs/remotes/origin/main")
	useRoot(t, root)
	payload, code := runCheck()
	if code != 2 {
		t.Fatalf("exit = %d (payload %v), want 2", code, payload)
	}
}

// unpushedFixture builds a checkout whose origin/main holds pushedSource and
// whose two local commits add the second guard and an unrelated file.
func unpushedFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}
	root := t.TempDir()
	writeSource(t, root, "internal/p/p.go", pushedSource)
	gitIn(t, root, "init", "-q", ".")
	commitAll(t, root, "pushed")
	gitIn(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	writeSource(t, root, "internal/p/p.go", unpushedSource)
	commitAll(t, root, "unpushed guard")
	writeSource(t, root, "internal/q/q.go", "package q\n")
	commitAll(t, root, "unpushed neighbor")
	return root
}

// checkAt runs the check action over root and answers its report.
func checkAt(t *testing.T, root string) (CheckReport, int) {
	t.Helper()
	useRoot(t, root)
	payload, code := runCheck()
	report, ok := payload.(CheckReport)
	if !ok {
		t.Fatalf("payload = %T (exit %d), want CheckReport", payload, code)
	}
	return report, code
}

// useRoot points lepath.Root at root. env.Get answers from a cache, so the
// cache is reset on both sides of the change.
func useRoot(t *testing.T, root string) {
	t.Helper()
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

func commitAll(t *testing.T, root, message string) {
	t.Helper()
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "-c", "user.email=t@ze", "-c", "user.name=t", "commit", "-qm", message)
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...) //nolint:gosec,noctx // this test's own fixture
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
