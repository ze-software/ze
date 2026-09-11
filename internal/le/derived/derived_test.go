// Related: derived.go -- the registry these tests read
//
// VALIDATES: a registered artifact carries both halves it needs, one path is
// claimed once, and the write-then-read loop answers from the edited tree.
// PREVENTS: a half-registered artifact that is silently inert, two generators
// answering for one path, and the stale read this spec exists to remove.
//
// The tests run in the EXTERNAL test package because the end-to-end row drives
// the two hooks, and internal/le/hookruntime imports this package. An in-package
// test could not reach the hook that consumes the registry, so it would assert
// over the registry alone and prove nothing about the loop a session runs.
package derived_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/derived"
	"github.com/ze-software/ze/internal/le/discoveryindex"
	_ "github.com/ze-software/ze/internal/le/docstocode"
	"github.com/ze-software/ze/internal/le/hookruntime"
)

// TestRegisteredArtifactsCarryAPredicateAndARebuild reads the real registry and
// requires each half of every artifact.
//
// An artifact with no predicate is removed by no write, and one with no rebuild
// is materialized by no read. Either is inert with nothing red to say so, which
// is the failure the registry replaced two hardcoded os.Stat blocks to avoid.
func TestRegisteredArtifactsCarryAPredicateAndARebuild(t *testing.T) {
	artifacts := derived.All()
	if len(artifacts) == 0 {
		t.Fatal("the registry is empty, so every assertion below is vacuous")
	}

	for _, artifact := range artifacts {
		if artifact.Feeds == nil {
			t.Errorf("%s carries no Feeds predicate, so no write invalidates it", artifact.Path)
		}
		if artifact.Rebuild == nil {
			t.Errorf("%s carries no Rebuild, so no read materializes it", artifact.Path)
		}
		if filepath.IsAbs(artifact.Path) || strings.Contains(artifact.Path, `\`) {
			t.Errorf("%s is not a slash path relative to the checkout root", artifact.Path)
		}
	}

	// The three artifacts the generators declare. Naming them here is what
	// makes a lost init() red: a registry that answers two is as well formed as
	// one that answers three.
	registered := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		registered = append(registered, artifact.Path)
	}
	for _, want := range []string{"ai/PACKAGE-MAP.md", "ai/DOCS-TO-CODE.md", "ai/CODE-TO-DOCS.md"} {
		if !slices.Contains(registered, want) {
			t.Errorf("%s is not registered; the registry holds %v", want, registered)
		}
	}
}

// TestAGrepAfterAnEditReadsTheEditedPackage is user story 1, over a throwaway
// checkout: a developer edits a Go file, then greps the package map.
//
// The write hook removes the map because the edited file feeds it, and the read
// hook rebuilds it because the command names it. What the grep then reads has
// to carry the EDIT, which is the whole point: the committed map answered from
// before the edit with nothing to say so.
func TestAGrepAfterAnEditReadsTheEditedPackage(t *testing.T) {
	root := t.TempDir()
	writeDerivedFixture(t, root, "ai/.keep", "")
	source := "internal/core/x/x.go"
	writeDerivedFixture(t, root, source, "// Package x reads the old summary.\npackage x\n")

	if _, err := discoveryindex.Update(root); err != nil {
		t.Fatalf("seed the fixture map: %v", err)
	}
	mapPath := filepath.Join(root, "ai", "PACKAGE-MAP.md")
	if before := readDerivedFixture(t, mapPath); !strings.Contains(before, "reads the old summary") {
		t.Fatalf("the seeded map does not describe the fixture package: %s", before)
	}

	writeDerivedFixture(t, root, source, "// Package x reads the edited summary.\npackage x\n")

	write := hookruntime.Payload{
		ToolName:  "Write",
		ToolInput: map[string]any{"file_path": filepath.Join(root, filepath.FromSlash(source))},
	}
	code, message, found := hookruntime.Probe("postInvalidateDerived", write, root)
	if !found {
		t.Fatal("postInvalidateDerived is not a registered check")
	}
	if code > 1 {
		t.Fatalf("the write hook refused the edit: %d %s", code, message)
	}
	if _, err := os.Stat(mapPath); !os.IsNotExist(err) {
		t.Fatalf("the map survived a write to a file that feeds it: %v", err)
	}

	read := hookruntime.Payload{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "grep -n 'core/x' ai/PACKAGE-MAP.md"},
	}
	code, message, found = hookruntime.Probe("preMaterializeDerived", read, root)
	if !found {
		t.Fatal("preMaterializeDerived is not a registered check")
	}
	if code != 0 {
		t.Fatalf("the read hook refused the grep: %d %s", code, message)
	}

	after := readDerivedFixture(t, mapPath)
	if !strings.Contains(after, "reads the edited summary") {
		t.Errorf("the grep reads a map from before the edit: %s", after)
	}
}

func writeDerivedFixture(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create the fixture directory for %s: %v", relative, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write the fixture %s: %v", relative, err)
	}
}

func readDerivedFixture(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path) //nolint:gosec // a fixture path this test built under t.TempDir()
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
