// Design: docs/architecture/testing/verify-freshness-scope.md -- race selection freshness.
package repochanged

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// TestRaceSelectionWithoutGreenBaselineCoversTheTree classifies a clean closure
// tree with absent, red, incomplete, and unresolvable certificates as unproven.
func TestRaceSelectionWithoutGreenBaselineCoversTheTree(t *testing.T) {
	for name, certificate := range map[string]string{
		"cold":       "",
		"red":        "exit=1\ngit_sha=verified\n",
		"missing":    "exit=0\n",
		"unknown":    "exit=0\ngit_sha=unknown\n",
		"unresolved": "exit=0\ngit_sha=unresolved\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			selectionBaseline(t, root)
			writeFile(t, root, "tmp/ze-verify.status", certificate)
			rec := &recorder{}
			selection, err := (Selector{Root: root, Run: rec.run}).Select()
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(selection.Rest, []string{"./..."}) {
				t.Fatalf("unproven tree selected %+v, want the whole tree", selection)
			}
		})
	}
}

// TestRaceSelectionIncludesGoBeforeADocsOnlyClosure uses an empty working tree
// and an earlier committed Go edit to prove HEAD alone is not the boundary.
func TestRaceSelectionIncludesGoBeforeADocsOnlyClosure(t *testing.T) {
	root := t.TempDir()
	selectionBaseline(t, root)
	rec := &recorder{answers: map[string]string{
		"git cat-file -t verified":                   "commit\n",
		"git diff --name-only verified HEAD -- *.go": "internal/component/bgp/wire/update.go\n",
	}}
	selection, err := (Selector{Root: root, Run: rec.run}).Select()
	if err != nil {
		t.Fatal(err)
	}
	want := []Group{{Name: "bgp", Pattern: "./internal/component/bgp/..."}}
	if !slices.Equal(selection.Groups, want) {
		t.Fatalf("closure selected %+v, want %v", selection, want)
	}
	if len(selection.Rest) != 0 {
		t.Fatalf("proven baseline widened to %v", selection.Rest)
	}
}

// TestRaceSelectionWithGreenDocsOnlyChangeIsEmpty contrasts a proven baseline
// with the cold case: no intervening Go edit really does leave nothing to race.
func TestRaceSelectionWithGreenDocsOnlyChangeIsEmpty(t *testing.T) {
	root := t.TempDir()
	selectionBaseline(t, root)
	rec := &recorder{answers: map[string]string{"git cat-file -t verified": "commit\n"}}
	selection, err := (Selector{Root: root, Run: rec.run}).Select()
	if err != nil {
		t.Fatal(err)
	}
	if !selection.Empty() {
		t.Fatalf("covered docs-only tree selected %+v", selection)
	}
	if len(rec.calls) != 5 {
		t.Fatalf("queried %v, want baseline validation and all four change queries", rec.calls)
	}
}

// TestRaceSelectionHonorsPublishedFullScope proves that a run's shared widening
// survives a green certificate and an otherwise empty Go-only diff.
func TestRaceSelectionHonorsPublishedFullScope(t *testing.T) {
	root := t.TempDir()
	selectionBaseline(t, root)
	file := filepath.Join(t.TempDir(), "scope-packages.txt")
	if err := WriteScopePackages(file, root, []string{"./..."}); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ScopeFileKey, file)
	env.ResetCache()
	rec := &recorder{answers: map[string]string{"git cat-file -t verified": "commit\n"}}
	selection, err := (Selector{Root: root, Run: rec.run}).Select()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(selection.Rest, []string{"./..."}) {
		t.Fatalf("published widening selected %+v", selection)
	}
}

// TestRaceSelectionRealCommittedClosure drives real Git over an isolated
// repository: unverified Go, a docs-only closure, then verified docs-only work.
func TestRaceSelectionRealCommittedClosure(t *testing.T) {
	root := gitFixture(t)
	gitIn(t, root, "add", "internal")
	gitIn(t, root, "-c", "user.email=t@ze", "-c", "user.name=t", "commit", "-qm", "code baseline")
	selectionBaseline(t, root)
	writeFile(t, root, "tmp/ze-verify.status", "exit=0\ngit_sha="+gitOut(t, root, "rev-parse", "HEAD")+"\n")
	writeFile(t, root, "internal/core/env/env.go", "package env\n\nconst Changed = true\n")
	gitIn(t, root, "add", "internal/core/env/env.go")
	gitIn(t, root, "-c", "user.email=t@ze", "-c", "user.name=t", "commit", "-qm", "unverified Go")
	commitReadme(t, root, "docs-only closure\n")

	selection, err := (Selector{Root: root}).Select()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(selection.Groups, []Group{{Name: "core", Pattern: "./internal/core/..."}}) {
		t.Fatalf("docs-only closure lost the earlier Go edit: %+v", selection)
	}
	if len(selection.Rest) != 0 {
		t.Fatalf("verified baseline widened to %v", selection.Rest)
	}

	writeFile(t, root, "tmp/ze-verify.status", "exit=0\ngit_sha="+gitOut(t, root, "rev-parse", "HEAD")+"\n")
	commitReadme(t, root, "verified code, newer docs\n")
	selection, err = (Selector{Root: root}).Select()
	if err != nil {
		t.Fatal(err)
	}
	if !selection.Empty() {
		t.Fatalf("verified docs-only change selected %+v", selection)
	}

	if err := os.Remove(filepath.Join(root, "tmp", "ze-verify.status")); err != nil {
		t.Fatal(err)
	}
	selection, err = (Selector{Root: root}).Select()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(selection.Rest, []string{"./..."}) {
		t.Fatalf("cold docs-only closure selected %+v", selection)
	}
}

func selectionBaseline(t *testing.T, root string) {
	t.Helper()
	t.Setenv("ZE_VERIFY_STATUS_FILE", filepath.Join(root, "tmp", "ze-verify.status"))
	t.Setenv("ZE_VERIFY_SCOPE_PACKAGES", "")
	t.Setenv(ScopeFileKey, "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	writeFile(t, root, "tmp/ze-verify.status", "exit=0\ngit_sha=verified\n")
}
