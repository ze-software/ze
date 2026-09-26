// VALIDATES: WorkingTreeLines answers the working-tree lines changed against
// HEAD, LinesSinceUpstream the lines changed since the last pushed commit,
// both in working-tree coordinates, and the parser refuses a diff it cannot
// read.
// PREVENTS: a line-scoped gate judging the wrong lines, or none.
package repochanged

import (
	"errors"
	"math"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// TestParseZeroContextDiffReadsNewSideSpans proves the parser over a hand-made
// diff: an addition, a modification, a deletion inside a file, a deletion at
// the head of a file, a deleted file, an unquoted path holding a space (which
// git ends with a TAB), and a C-quoted path each land where the new side holds
// them. Two added lines whose text renders as a `+++` header, `+++ /dev/null`
// and `+++ b/other.go`, stay body lines: the hunks after each still count
// for plus.go, and no other.go key appears.
func TestParseZeroContextDiffReadsNewSideSpans(t *testing.T) {
	diff := `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -3,0 +4,2 @@ func f() {
+	if a || b {
+		return
@@ -10 +12 @@ func g() {
-	old
+	new
@@ -20,2 +21,0 @@ func h() {
-	gone
-	gone
@@ -1 +0,0 @@
-// head
diff --git a/dead.go b/dead.go
--- a/dead.go
+++ /dev/null
@@ -1,3 +0,0 @@
-package dead
diff --git a/sp ace.go b/sp ace.go
--- a/sp ace.go	
+++ b/sp ace.go	
@@ -1 +1 @@
-x
+y
diff --git a/plus.go b/plus.go
--- a/plus.go
+++ b/plus.go
@@ -2,0 +3 @@ package plus
+++ /dev/null
@@ -5,0 +7 @@ func f() {
+++ b/other.go
@@ -9,0 +12 @@ func g() {
+	x
diff --git "a/tab\there.go" "b/tab\there.go"
--- "a/tab\there.go"
+++ "b/tab\there.go"
@@ -1 +1 @@
-x
+y
`
	got, err := parseZeroContextDiff(diff)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := ChangedLines{
		"a.go":         {{From: 4, To: 5}, {From: 12, To: 12}, {From: 21, To: 21}},
		"sp ace.go":    {{From: 1, To: 1}},
		"plus.go":      {{From: 3, To: 3}, {From: 7, To: 7}, {From: 12, To: 12}},
		"tab\there.go": {{From: 1, To: 1}},
	}
	if len(got) != len(want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	for path, spans := range want {
		if !slices.Equal(got[path], spans) {
			t.Errorf("%s: spans = %v, want %v", path, got[path], spans)
		}
	}
}

// TestParseZeroContextDiffRefusesAnUnreadableShape proves a diff the parser
// cannot place is an error, never an empty change set that passes a gate.
func TestParseZeroContextDiffRefusesAnUnreadableShape(t *testing.T) {
	for name, diff := range map[string]string{
		"hunk before any target": "@@ -1 +1 @@\n",
		"target without prefix":  "+++ a.go\n@@ -1 +1 @@\n",
	} {
		if _, err := parseZeroContextDiff(diff); !errors.Is(err, errDiffShape) {
			t.Errorf("%s: err = %v, want errDiffShape", name, err)
		}
	}
}

// TestChangedLinesTouches proves the overlap test at both edges of a span.
func TestChangedLinesTouches(t *testing.T) {
	changed := ChangedLines{"a.go": {{From: 10, To: 12}}}
	for _, row := range []struct {
		first, last int
		want        bool
	}{
		{1, 9, false}, {1, 10, true}, {12, 20, true}, {13, 20, false}, {11, 11, true},
	} {
		if got := changed.Touches("a.go", row.first, row.last); got != row.want {
			t.Errorf("Touches(%d, %d) = %v, want %v", row.first, row.last, got, row.want)
		}
	}
	if changed.Touches("b.go", 1, math.MaxInt) {
		t.Error("an unchanged path is touched")
	}
}

// TestWorkingTreeLinesAgainstARealCheckout drives the git route: a committed
// file edited after commit answers only the edited lines, a staged-then-edited
// file answers working-tree coordinates, and an untracked file answers whole.
func TestWorkingTreeLinesAgainstARealCheckout(t *testing.T) {
	root := gitFixture(t)
	// The owner's machine sets this globally. Set here, it makes the test fail
	// without --inter-hunk-context=0 on every machine: lines 1 and 3 would then
	// merge into one hunk covering the unchanged line 2.
	gitIn(t, root, "config", "diff.interHunkContext", "10")
	writeFile(t, root, "README.md", "base\nadded\n")
	gitIn(t, root, "add", "README.md")
	writeFile(t, root, "README.md", "top\nbase\nadded\n")
	writeFile(t, root, "new.go", "package x\n")

	got, err := WorkingTreeLines(root)
	if err != nil {
		t.Fatalf("WorkingTreeLines: %v", err)
	}
	// Line 3 is the staged addition, counted where the working tree now holds
	// it; the index numbers it 2.
	if !slices.Equal(got["README.md"], []LineSpan{{From: 1, To: 1}, {From: 3, To: 3}}) {
		t.Errorf("README.md spans = %v, want lines 1 and 3", got["README.md"])
	}
	if !got.Touches("new.go", 1, 1) {
		t.Errorf("untracked new.go is not wholly changed: %v", got["new.go"])
	}
	if got.Touches("go.mod", 1, math.MaxInt) {
		t.Errorf("unchanged go.mod answered as changed: %v", got["go.mod"])
	}
}

// TestLinesSinceUpstreamJudgesTheUnpushedRange proves the base: the merge base
// with origin/main when the branch tracks nothing, the merge base with the
// upstream when it tracks one, and errNoUpstreamBase when neither ref resolves
// or the ref shares no history with HEAD.
func TestLinesSinceUpstreamJudgesTheUnpushedRange(t *testing.T) {
	root := gitFixture(t)
	pushed := gitOut(t, root, "rev-parse", "HEAD")
	gitIn(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	commitReadme(t, root, "base\nfirst\n")
	first := gitOut(t, root, "rev-parse", "HEAD")
	commitReadme(t, root, "base\nfirst\nsecond\n")

	changed, base, err := LinesSinceUpstream(root)
	if err != nil {
		t.Fatalf("LinesSinceUpstream: %v", err)
	}
	if base.Commit != pushed {
		t.Errorf("base = %q, want origin/main %q", base.Commit, pushed)
	}
	if !slices.Equal(changed["README.md"], []LineSpan{{From: 2, To: 3}}) {
		t.Errorf("README.md spans = %v, want both unpushed lines 2-3", changed["README.md"])
	}

	// A tracked upstream wins over origin/main.
	branch := gitOut(t, root, "symbolic-ref", "--short", "HEAD")
	gitIn(t, root, "branch", "published", first)
	gitIn(t, root, "config", "branch."+branch+".remote", ".")
	gitIn(t, root, "config", "branch."+branch+".merge", "refs/heads/published")
	changed, base, err = LinesSinceUpstream(root)
	if err != nil {
		t.Fatalf("LinesSinceUpstream with an upstream: %v", err)
	}
	if base.Commit != first {
		t.Errorf("base = %q, want the upstream %q", base.Commit, first)
	}
	if !slices.Equal(changed["README.md"], []LineSpan{{From: 3, To: 3}}) {
		t.Errorf("README.md spans = %v, want line 3 alone", changed["README.md"])
	}

	// origin/main on a commit with no shared history, as a shallow clone cut
	// above the fork point leaves it.
	gitIn(t, root, "config", "--unset", "branch."+branch+".remote")
	gitIn(t, root, "config", "--unset", "branch."+branch+".merge")
	orphan := gitOut(t, root, "-c", "user.email=t@ze", "-c", "user.name=t",
		"commit-tree", gitOut(t, root, "write-tree"), "-m", "unrelated")
	gitIn(t, root, "update-ref", "refs/remotes/origin/main", orphan)
	if _, _, err := LinesSinceUpstream(root); !errors.Is(err, errNoUpstreamBase) {
		t.Errorf("unrelated origin/main: err = %v, want errNoUpstreamBase", err)
	}

	bare := gitFixture(t)
	if _, _, err := LinesSinceUpstream(bare); !errors.Is(err, errNoUpstreamBase) {
		t.Errorf("no upstream and no origin/main: err = %v, want errNoUpstreamBase", err)
	}
}

// commitReadme commits README.md holding body.
func commitReadme(t *testing.T, root, body string) {
	t.Helper()
	writeFile(t, root, "README.md", body)
	gitIn(t, root, "add", "README.md")
	gitIn(t, root, "-c", "user.email=t@ze", "-c", "user.name=t", "commit", "-qm", "unpushed")
}

// gitOut answers one git query's output, trimmed.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...) //nolint:gosec,noctx // this test's own fixture
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
}
