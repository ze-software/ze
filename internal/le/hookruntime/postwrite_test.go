// Related: postwrite.go -- the posttool-writeedit checks
//
// VALIDATES: a write to a file that feeds a derived artifact removes that
// artifact, and a write to a file that feeds a DIFFERENT one leaves it alone.
// PREVENTS: the stale read this spec exists to remove, and its opposite, an
// invalidation wide enough to delete every artifact on every edit.
//
// The generators are imported for their init(), which is what puts the three
// artifacts in the registry. Without them derived.All() is empty here and every
// assertion below passes over nothing.
package hookruntime

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/discoveryindex"
	"github.com/ze-software/ze/internal/le/docstocode"
)

// derivedFixture builds a throwaway checkout holding one Go package and one
// documentation page, then renders all three derived artifacts over it.
func derivedFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeHookFixture(t, root, "ai/.keep", "")
	writeHookFixture(t, root, "internal/core/x/x.go",
		"// Design: docs/architecture/core-design.md -- x\n// Package x does x.\npackage x\n")
	writeHookFixture(t, root, "docs/architecture/core-design.md",
		"# Core design\n\n<!-- source: internal/core/x/x.go -- x -->\n")

	if _, err := discoveryindex.Update(root); err != nil {
		t.Fatalf("seed ai/PACKAGE-MAP.md: %v", err)
	}
	if _, err := docstocode.Update(root); err != nil {
		t.Fatalf("seed ai/DOCS-TO-CODE.md: %v", err)
	}
	if _, err := docstocode.UpdateCodeIndex(root); err != nil {
		t.Fatalf("seed ai/CODE-TO-DOCS.md: %v", err)
	}
	return root
}

// artifactStamp answers the modification time of one artifact, and fails the
// test when the file is absent.
func artifactStamp(t *testing.T, root, relative string) time.Time {
	t.Helper()
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatalf("%s is absent: %v", relative, err)
	}
	return info.ModTime()
}

// TestEditingAnIndexFeedingSourceRemovesThePackageMap drives the hook that
// invalidates, over the entry point a session reaches: a Write payload.
//
// The edited file carries the `// Package` header the map takes its row text
// from, so the map can no longer describe the tree. It is REMOVED rather than
// rewritten: a rewrite costs half a second on an edit most sessions never read
// the map after, and a stamped-but-kept file is still read by the next grep.
func TestEditingAnIndexFeedingSourceRemovesThePackageMap(t *testing.T) {
	root := derivedFixture(t)
	writeHookFixture(t, root, "internal/core/x/x.go",
		"// Design: docs/architecture/core-design.md -- x\n// Package x does something else.\npackage x\n")

	payload := Payload{
		ToolName:  toolWrite,
		ToolInput: map[string]any{"file_path": filepath.Join(root, "internal", "core", "x", "x.go")},
	}
	code, message, found := Probe("postInvalidateDerived", payload, root)
	if !found {
		t.Fatal("postInvalidateDerived is not registered in nativeHookActions")
	}
	if code > 1 {
		t.Fatalf("the hook refused the write: %d %s", code, message)
	}

	if _, err := os.Stat(filepath.Join(root, "ai", "PACKAGE-MAP.md")); !os.IsNotExist(err) {
		t.Errorf("ai/PACKAGE-MAP.md survived an edit to the file it reads its row from: %v", err)
	}
	// The same write feeds ai/DOCS-TO-CODE.md, whose input is every `.go` file
	// the walk visits. Both go, and that is one predicate each answering for
	// itself rather than one blanket removal.
	if _, err := os.Stat(filepath.Join(root, "ai", "DOCS-TO-CODE.md")); !os.IsNotExist(err) {
		t.Errorf("ai/DOCS-TO-CODE.md survived an edit to a Go file: %v", err)
	}
}

// TestEditingANonSourceLeavesEveryArtifactInPlace is the scoping half.
//
// A page under docs/ feeds ai/CODE-TO-DOCS.md and nothing else, so the other
// two artifacts must be untouched, down to their modification time. A hook that
// removed all three on every write would pass the test above and would make
// every read in a session pay a rebuild.
func TestEditingANonSourceLeavesEveryArtifactInPlace(t *testing.T) {
	root := derivedFixture(t)
	packageMap := artifactStamp(t, root, "ai/PACKAGE-MAP.md")
	docsToCode := artifactStamp(t, root, "ai/DOCS-TO-CODE.md")

	writeHookFixture(t, root, "docs/architecture/core-design.md",
		"# Core design\n\n<!-- source: internal/core/x/x.go -- x does x -->\n")
	payload := Payload{
		ToolName:  toolWrite,
		ToolInput: map[string]any{"file_path": filepath.Join(root, "docs", "architecture", "core-design.md")},
	}
	code, message, found := Probe("postInvalidateDerived", payload, root)
	if !found {
		t.Fatal("postInvalidateDerived is not registered in nativeHookActions")
	}
	if code > 1 {
		t.Fatalf("the hook refused the write: %d %s", code, message)
	}

	if now := artifactStamp(t, root, "ai/PACKAGE-MAP.md"); !now.Equal(packageMap) {
		t.Errorf("ai/PACKAGE-MAP.md was rewritten by a docs edit: %v then %v", packageMap, now)
	}
	if now := artifactStamp(t, root, "ai/DOCS-TO-CODE.md"); !now.Equal(docsToCode) {
		t.Errorf("ai/DOCS-TO-CODE.md was rewritten by a docs edit: %v then %v", docsToCode, now)
	}
	if _, err := os.Stat(filepath.Join(root, "ai", "CODE-TO-DOCS.md")); !os.IsNotExist(err) {
		t.Errorf("ai/CODE-TO-DOCS.md survived an edit to the page its anchors come from: %v", err)
	}
}

// TestATrackedArtifactIsNeverRemoved is the never-destroy-work half.
//
// Several sessions share one checkout, and git reads a removed tracked file as
// a deletion to stage, so another session's commit script would carry it. An
// artifact that is registered as derived and still tracked is a migration half
// done: the hook says so and removes nothing, rather than deleting a file the
// index holds (ai/rules/never-destroy-work.md).
func TestATrackedArtifactIsNeverRemoved(t *testing.T) {
	root := derivedFixture(t)
	gitInitFixture(t, root)

	stamp := artifactStamp(t, root, "ai/PACKAGE-MAP.md")
	writeHookFixture(t, root, "internal/core/x/x.go",
		"// Design: docs/architecture/core-design.md -- x\n// Package x moved on.\npackage x\n")

	payload := Payload{
		ToolName:  toolWrite,
		ToolInput: map[string]any{"file_path": filepath.Join(root, "internal", "core", "x", "x.go")},
	}
	code, message, found := Probe("postInvalidateDerived", payload, root)
	if !found {
		t.Fatal("postInvalidateDerived is not registered in nativeHookActions")
	}
	if code == 0 {
		t.Errorf("the hook was silent about a tracked artifact it is registered to remove: %s", message)
	}

	after, err := os.Stat(filepath.Join(root, "ai", "PACKAGE-MAP.md"))
	if err != nil {
		t.Fatalf("ai/PACKAGE-MAP.md was removed while git tracked it: %v", err)
	}
	if !after.ModTime().Equal(stamp) {
		t.Errorf("ai/PACKAGE-MAP.md was rewritten while git tracked it")
	}

	// The other half: a tracked artifact must not stop the pass. The same Go
	// edit feeds ai/DOCS-TO-CODE.md, which git does not track here, so it has
	// to go. A refusal that RETURNED would leave every artifact behind the
	// tracked one present and stale, which is the wrong answer this check
	// exists to prevent.
	if _, err := os.Stat(filepath.Join(root, "ai", "DOCS-TO-CODE.md")); !os.IsNotExist(err) {
		t.Errorf("an untracked artifact behind the tracked one survived the same write: %v", err)
	}
}

// TestRemovingAPackageHeaderStillInvalidatesTheMap covers the edit that DELETES
// the summary the map quotes.
//
// The map takes each row's text from a `// Package` comment, so removing that
// comment changes the row as surely as rewriting it: `recordPackage` falls back
// to the register.go description or to TODO. The invalidation predicate reads
// the file AFTER the write, where the deleted line is already gone, so a
// predicate that asks for a package header answers false exactly when the map
// has most obviously drifted.
func TestRemovingAPackageHeaderStillInvalidatesTheMap(t *testing.T) {
	root := derivedFixture(t)
	writeHookFixture(t, root, "internal/core/x/x.go",
		"// Design: docs/architecture/core-design.md -- x\npackage x\n")

	payload := Payload{
		ToolName:  toolWrite,
		ToolInput: map[string]any{"file_path": filepath.Join(root, "internal", "core", "x", "x.go")},
	}
	code, message, found := Probe("postInvalidateDerived", payload, root)
	if !found {
		t.Fatal("postInvalidateDerived is not registered in nativeHookActions")
	}
	if code > 1 {
		t.Fatalf("the hook refused the write: %d %s", code, message)
	}
	if _, err := os.Stat(filepath.Join(root, "ai", "PACKAGE-MAP.md")); !os.IsNotExist(err) {
		t.Errorf("ai/PACKAGE-MAP.md survived the deletion of the comment it quotes: %v", err)
	}
}

// TestTrackedFailsClosedWhenTheCheckoutCannotBeRead drives the guard's own
// error path.
//
// tracked answers a question whose false arm AUTHORIZES A DELETION, so an
// unreadable checkout must read as tracked. Answering false for any stat error
// means a checkout this process cannot look into licenses the removal of a file
// git may well be holding.
func TestTrackedFailsClosedWhenTheCheckoutCannotBeRead(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the directory permission this test removes")
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o750) })

	if !tracked(root, "ai/PACKAGE-MAP.md") {
		t.Error("a checkout that cannot be read answered 'not tracked', which authorizes a deletion")
	}
}

// gitInitFixture makes the fixture tree a git checkout holding one artifact in
// its index. The index is what `tracked` reads, so a fixture that only creates
// .git would answer the opposite of what this test is about.
func gitInitFixture(t *testing.T, root string) {
	t.Helper()
	for _, arguments := range [][]string{
		{"init", "--quiet"},
		{"add", "ai/PACKAGE-MAP.md"},
	} {
		command := exec.CommandContext(t.Context(), "git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %s in the fixture: %v\n%s", arguments[0], err, output)
		}
	}
}
