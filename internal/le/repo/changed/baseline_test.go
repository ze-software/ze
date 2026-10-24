// Design: docs/architecture/testing/verify-freshness-scope.md -- race selection freshness.
package repochanged

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// VALIDATES: the race groups start the committed term at the git anchor, so a
// committed Go edit behind a docs-only commit is raced and a checkout with no
// anchor is refused rather than raced whole.
// PREVENTS: the loop where a missing or red verify certificate made every
// scoped race pass race ./....

// The two anchor queries the recorder answers, and the commit names it answers
// them with.
const (
	anchorUpstreamQuery  = "git rev-parse --verify -q @{u}^{commit}"
	anchorMergeBaseQuery = "git merge-base HEAD upstream"
	anchorCommittedQuery = "git diff --name-only anchor HEAD -- *.go"
)

// anchoredAnswers adds the anchor's two answers to a recorder table: @{u} is
// "upstream", and its merge base with HEAD is "anchor".
func anchoredAnswers(answers map[string]string) map[string]string {
	if answers == nil {
		answers = map[string]string{}
	}
	answers[anchorUpstreamQuery] = "upstream\n"
	answers[anchorMergeBaseQuery] = "anchor\n"
	return answers
}

// TestRaceSelectionIncludesGoBeforeADocsOnlyClosure uses an empty working tree
// and an earlier committed Go edit to prove HEAD alone is not the boundary.
func TestRaceSelectionIncludesGoBeforeADocsOnlyClosure(t *testing.T) {
	isolateSelection(t)
	rec := &recorder{answers: anchoredAnswers(map[string]string{
		anchorCommittedQuery: "internal/component/bgp/wire/update.go\n",
	})}
	selection, err := (Selector{Root: t.TempDir(), Run: rec.run}).Select()
	if err != nil {
		t.Fatal(err)
	}
	want := []Group{{Name: "bgp", Pattern: "./internal/component/bgp/..."}}
	if !slices.Equal(selection.Groups, want) {
		t.Fatalf("closure selected %+v, want %v", selection, want)
	}
	if len(selection.Rest) != 0 {
		t.Fatalf("an anchored selection widened to %v", selection.Rest)
	}
}

// TestRaceSelectionAtAnchorDocsOnlyChangeIsEmpty proves the empty answer is
// earned: every query ran, and no Go edit since the anchor leaves nothing to
// race.
func TestRaceSelectionAtAnchorDocsOnlyChangeIsEmpty(t *testing.T) {
	isolateSelection(t)
	rec := &recorder{answers: anchoredAnswers(nil)}
	selection, err := (Selector{Root: t.TempDir(), Run: rec.run}).Select()
	if err != nil {
		t.Fatal(err)
	}
	if !selection.Empty() {
		t.Fatalf("docs-only tree selected %+v", selection)
	}
	if len(rec.calls) != 6 {
		t.Fatalf("queried %v, want the three working-tree queries, the two anchor queries and the committed query", rec.calls)
	}
}

// TestRaceSelectionRefusesWithNoAnchor proves a checkout whose upstream and
// origin/HEAD both fail to resolve is an error, never ./....
func TestRaceSelectionRefusesWithNoAnchor(t *testing.T) {
	isolateSelection(t)
	rec := &recorder{}
	selection, err := (Selector{Root: t.TempDir(), Run: rec.run}).Select()
	if !errors.Is(err, ErrNoUpstream) {
		t.Fatalf("Select err = %v (selection %+v), want ErrNoUpstream", err, selection)
	}
}

// TestRaceSelectionHonorsPublishedFullScope proves that a run's shared widening
// survives an otherwise empty Go-only diff.
func TestRaceSelectionHonorsPublishedFullScope(t *testing.T) {
	root := t.TempDir()
	isolateSelection(t)
	file := filepath.Join(t.TempDir(), "scope-packages.txt")
	if err := WriteScopePackages(file, root, []string{"./..."}); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ScopeFileKey, file)
	// The verifier can supply either spelling before this process starts.
	// Keep both equal so their inherited environment order cannot hide the file.
	t.Setenv("ZE_VERIFY_SCOPE_PACKAGES", file)
	env.ResetCache()
	rec := &recorder{answers: anchoredAnswers(nil)}
	selection, err := (Selector{Root: root, Run: rec.run}).Select()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(selection.Rest, []string{"./..."}) {
		t.Fatalf("published widening selected %+v", selection)
	}
}

// TestRaceSelectionRealCommittedClosure drives real Git over an isolated
// repository: an unpushed Go commit behind a docs-only one is raced, and once
// both are on the upstream nothing is.
func TestRaceSelectionRealCommittedClosure(t *testing.T) {
	root := gitFixture(t)
	isolateSelection(t)
	gitIn(t, root, "add", "internal")
	gitIn(t, root, "-c", "user.email=t@ze", "-c", "user.name=t", "commit", "-qm", "code baseline")
	trackPublished(t, root)
	writeFile(t, root, "internal/core/env/env.go", "package env\n\nconst Changed = true\n")
	gitIn(t, root, "add", "internal/core/env/env.go")
	gitIn(t, root, "-c", "user.email=t@ze", "-c", "user.name=t", "commit", "-qm", "unpushed Go")
	commitReadme(t, root, "docs-only closure\n")

	selection, err := (Selector{Root: root}).Select()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(selection.Groups, []Group{{Name: "core", Pattern: "./internal/core/..."}}) {
		t.Fatalf("docs-only closure lost the earlier Go edit: %+v", selection)
	}
	if len(selection.Rest) != 0 {
		t.Fatalf("an anchored selection widened to %v", selection.Rest)
	}

	gitIn(t, root, "branch", "-f", "published", "HEAD")
	selection, err = (Selector{Root: root}).Select()
	if err != nil {
		t.Fatal(err)
	}
	if !selection.Empty() {
		t.Fatalf("a pushed tree selected %+v", selection)
	}
}

// isolateSelection clears the scope files a verify run publishes, so a test
// selects from its own fixture rather than from an answer the environment
// carries.
func isolateSelection(t *testing.T) {
	t.Helper()
	t.Setenv("ZE_VERIFY_SCOPE_PACKAGES", "")
	t.Setenv(ScopeFileKey, "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}
