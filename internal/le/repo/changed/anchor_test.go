// Design: docs/architecture/testing/verify-freshness-scope.md -- one change-set selection
package repochanged

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// VALIDATES: the change set starts at merge-base(HEAD, @{u}), falls back to
// refs/remotes/origin/HEAD, and refuses with the fixing command when neither
// resolves; no verify certificate takes part.
// PREVENTS: a red or missing certificate turning every scoped selection into
// ./... and every feature tag.

// anchoredScopeFixture is writeScopeFixture committed as a git repository whose
// branch tracks a local branch, published, at that first commit: the upstream
// holds the fixture and nothing else. The verify certificate is pointed at a
// red one outside the checkout, so a selector that still read it would widen.
func anchoredScopeFixture(t *testing.T) string {
	t.Helper()
	root := writeScopeFixture(t)
	isolateSelection(t)
	red := filepath.Join(t.TempDir(), "ze-verify.status")
	writeFile(t, filepath.Dir(red), "ze-verify.status", "exit=130\nmode=full\ngit_sha=unknown\n")
	t.Setenv("ZE_VERIFY_STATUS_FILE", red)
	env.ResetCache()
	commitAll(t, root, "fixture")
	trackPublished(t, root)
	return root
}

// commitAll commits every path of root, initializing the repository first when
// it is not one yet.
func commitAll(t *testing.T, root, message string) {
	t.Helper()
	if gitOutAllowFail(root, "rev-parse", "--git-dir") == "" {
		gitIn(t, root, "init", "-q", ".")
	}
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "-c", "user.email=t@ze", "-c", "user.name=t", "commit", "-qm", message)
}

// trackPublished makes the current branch track a local branch named published
// at HEAD, so @{u} resolves with no remote and no network.
func trackPublished(t *testing.T, root string) {
	t.Helper()
	branch := gitOut(t, root, "symbolic-ref", "--short", "HEAD")
	gitIn(t, root, "branch", "-f", "published", "HEAD")
	gitIn(t, root, "config", "branch."+branch+".remote", ".")
	gitIn(t, root, "config", "branch."+branch+".merge", "refs/heads/published")
}

// pointOriginHead records HEAD as origin/main and makes refs/remotes/origin/HEAD
// name it, as a clone leaves them.
func pointOriginHead(t *testing.T, root string) {
	t.Helper()
	gitIn(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	gitIn(t, root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
}

// gitOutAllowFail answers one git query's output, or "" when git fails.
func gitOutAllowFail(dir string, args ...string) string {
	out, err := runGit(dir, args...)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// TestScopeAnchoredOnUpstreamWithNoCertificate is AC-1 and the wiring row for
// `le repo changed scope`: with a red certificate and an upstream, one Go edit
// selects its package, its gated importer and its feature, never ./....
func TestScopeAnchoredOnUpstreamWithNoCertificate(t *testing.T) {
	goToolchain(t)
	root := anchoredScopeFixture(t)
	writeFile(t, root, "ssh/ssh.go", "package ssh\n\n// Listen is the gated feature.\nfunc Listen() int { return 9 }\n")

	stdout, stderr, code := runScopeSelector(t, "", root, "--print=both")
	if code != 0 {
		t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
	}
	packages, tags, err := splitScopeSections(stdout)
	if err != nil {
		t.Fatalf("%v\nstdout:\n%s", err, stdout)
	}
	if !slices.Equal(packages, []string{"./hub", "./ssh"}) {
		t.Errorf("packages = %v, want [./hub ./ssh]\nstderr:\n%s", packages, stderr)
	}
	if !slices.Equal(tags, []string{"ze_ssh"}) {
		t.Errorf("tags = %v, want [ze_ssh]", tags)
	}
	if strings.Contains(stderr, "green") {
		t.Errorf("the selector still names a green-commit widening:\n%s", stderr)
	}
}

// TestScopeHoldsCommittedAheadOfUpstream is AC-2: on a clean tree, a commit the
// upstream lacks is in the change set, with its importers.
func TestScopeHoldsCommittedAheadOfUpstream(t *testing.T) {
	goToolchain(t)
	root := anchoredScopeFixture(t)
	writeFile(t, root, "core/core.go", "package core\n\n// Value is the leaf of the fixture chain.\nfunc Value() int { return 7 }\n")
	commitAll(t, root, "unpushed core edit")

	stdout, stderr, code := runScopeSelector(t, "", root, "--print=packages")
	if code != 0 {
		t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
	}
	if got := scopeLines(stdout); !slices.Equal(got, []string{"./core", "./high", "./mid"}) {
		t.Errorf("packages = %v, want the committed package and two levels of importers\nstderr:\n%s", got, stderr)
	}
}

// TestScopeEmptyAtUpstream is AC-3: a clean tree whose HEAD is its upstream has
// an empty change set, says so, and does not widen.
func TestScopeEmptyAtUpstream(t *testing.T) {
	goToolchain(t)
	root := anchoredScopeFixture(t)

	stdout, stderr, code := runScopeSelector(t, "", root, "--print=packages")
	if code != 0 {
		t.Fatalf("selector exited %d\nstderr:\n%s", code, stderr)
	}
	if got := scopeLines(stdout); len(got) != 0 {
		t.Errorf("packages = %v, want none", got)
	}
	if !strings.Contains(stderr, "the change set holds nothing") {
		t.Errorf("the empty change set is not said:\n%s", stderr)
	}
}

// TestAnchorFallsBackToOriginHead is AC-4: a branch with no upstream anchors on
// refs/remotes/origin/HEAD.
func TestAnchorFallsBackToOriginHead(t *testing.T) {
	root := gitFixture(t)
	pushed := gitOut(t, root, "rev-parse", "HEAD")
	pointOriginHead(t, root)
	commitReadme(t, root, "base\nunpushed\n")

	anchor, err := ResolveAnchor(root, runGit)
	if err != nil {
		t.Fatalf("ResolveAnchor: %v", err)
	}
	if anchor.Commit != pushed {
		t.Errorf("anchor = %q, want origin/HEAD's commit %q", anchor.Commit, pushed)
	}
	if anchor.Upstream != "refs/remotes/origin/HEAD" {
		t.Errorf("upstream = %q, want refs/remotes/origin/HEAD", anchor.Upstream)
	}
	if anchor.UpstreamCommit != pushed {
		t.Errorf("upstream commit = %q, want %q", anchor.UpstreamCommit, pushed)
	}
}

// TestAnchorRefusesWithNoUpstream is AC-5: with neither an upstream nor
// origin/HEAD the selector exits 2, prints no package, and gives the command
// that sets the upstream on its own line.
func TestAnchorRefusesWithNoUpstream(t *testing.T) {
	goToolchain(t)
	root := writeScopeFixture(t)
	isolateSelection(t)
	commitAll(t, root, "fixture")
	branch := gitOut(t, root, "symbolic-ref", "--short", "HEAD")

	if _, err := ResolveAnchor(root, runGit); !errors.Is(err, ErrNoUpstream) {
		t.Fatalf("ResolveAnchor err = %v, want ErrNoUpstream", err)
	}

	stdout, stderr, code := runScopeSelector(t, "", root, "--print=packages")
	if code != 2 {
		t.Fatalf("selector exited %d, want 2\nstderr:\n%s", code, stderr)
	}
	if got := scopeLines(stdout); len(got) != 0 {
		t.Errorf("a refused selection printed %v", got)
	}
	want := "\n    git branch --set-upstream-to=origin/" + branch + "\n"
	if !strings.Contains(stderr, want) {
		t.Errorf("the refusal does not give %q on its own line:\n%s", strings.TrimSpace(want), stderr)
	}
}
