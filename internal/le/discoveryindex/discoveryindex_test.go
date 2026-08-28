// VALIDATES: spec-le-is-a-ze-binary AC-11 -- the generated package map is
// derived from the tree, and a tree the generator cannot read is refused
// rather than described.
// PREVENTS: an unreadable file counted as a package that declares nothing,
// whose write half then commits that omission into ai/PACKAGE-MAP.md.

package discoveryindex

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// tree writes a fixture checkout and answers its root.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture file: %v", err)
		}
	}
	return root
}

func TestBuildReadsThePackageDoc(t *testing.T) {
	root := tree(t, map[string]string{
		"internal/core/thing/thing.go": "// Package thing does a thing. And more.\npackage thing\n",
	})

	packages, err := Build(root)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(packages) != 1 {
		t.Fatalf("Build found %d packages, want 1: %+v", len(packages), packages)
	}
	if packages[0].Path != "internal/core/thing" {
		t.Errorf("path is %q", packages[0].Path)
	}
	if packages[0].Responsibility != "does a thing" {
		t.Errorf("responsibility is %q, want the first sentence", packages[0].Responsibility)
	}
}

func TestBuildRefusesAFileItCannotRead(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file, so the case cannot be built")
	}
	root := tree(t, map[string]string{
		"internal/core/thing/thing.go": "// Package thing does a thing.\npackage thing\n",
	})
	unreadable := filepath.Join(root, "internal", "core", "thing", "thing.go")
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	_, err := Build(root)
	if err == nil {
		t.Fatal("Build described a tree holding a file it could not read")
	}
	if !strings.Contains(err.Error(), "thing.go") {
		t.Errorf("the error does not name the file: %v", err)
	}
}

func TestBuildRefusesADirectoryItCannotEnter(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root enters a mode-000 directory, so the case cannot be built")
	}
	root := tree(t, map[string]string{
		"internal/core/thing/thing.go": "// Package thing does a thing.\npackage thing\n",
		"internal/core/closed/shut.go": "// Package shut is unreachable.\npackage shut\n",
	})
	closed := filepath.Join(root, "internal", "core", "closed")
	if err := os.Chmod(closed, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(closed, 0o750) })

	if _, err := Build(root); err == nil {
		t.Fatal("Build described a tree holding a directory it could not enter")
	}
}

func TestBuildReportsADanglingSymbolicLink(t *testing.T) {
	// A link with a missing target is a directory entry that is STILL THERE but
	// unreadable. It differs from a file that another session removed. This
	// tree cannot be described, so the two cases must have different answers.
	root := tree(t, map[string]string{
		"internal/core/thing/thing.go": "// Package thing does a thing.\npackage thing\n",
	})
	if err := os.Symlink(filepath.Join(root, "gone.go"),
		filepath.Join(root, "internal", "core", "thing", "aaa.go")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if _, err := Build(root); err == nil {
		t.Fatal("Build described a tree holding a link that resolves to nothing")
	}
}

func TestVanishedTellsAGoneFileFromAnEntryThatIsStillThere(t *testing.T) {
	// Another session can remove a file between the listing and read in this
	// shared checkout. That file was never in the described tree. Refusing the
	// run would report another session's timing as a defect.
	root := t.TempDir()

	gone := filepath.Join(root, "gone.go")
	lines, err := head(gone, HeaderLines)
	if err != nil {
		t.Errorf("a file that is no longer there was refused: %v", err)
	}
	if lines != nil {
		t.Errorf("a file that is no longer there answered %v", lines)
	}

	link := filepath.Join(root, "link.go")
	if err := os.Symlink(gone, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if _, err := head(link, HeaderLines); err == nil {
		t.Error("a link that resolves to nothing was read as a file that vanished")
	}
}

func TestFirstSentenceCutsAtCharactersNotBytes(t *testing.T) {
	// Python measures this bound with len(), which counts code points. A
	// byte-counting port cuts a summary carrying one non-ASCII character at a
	// different place, and the two halves then write different files.
	const accent = "é"
	text := strings.Repeat(accent+" ", MaxSummary)

	got := firstSentence(text)
	runes := []rune(got)
	if len(runes) > MaxSummary {
		t.Errorf("the summary is %d characters, above the bound of %d", len(runes), MaxSummary)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("a truncated summary does not say so: %q", got)
	}
	// One below the bound is untouched, which is what makes the bound a bound
	// rather than a habit.
	short := strings.TrimSpace(strings.Repeat("a ", (MaxSummary-1)/2))
	if firstSentence(short) != short {
		t.Errorf("a summary inside the bound was cut: %q", firstSentence(short))
	}

	// The case that tells the two measures apart: 150 accented characters are
	// 300 BYTES and 150 characters. A byte-counting bound truncates it and a
	// character-counting one does not, and the Python half does not.
	wide := strings.Repeat(accent, MaxSummary-50)
	if got := firstSentence(wide); got != wide {
		t.Errorf("a summary of %d characters and %d bytes was cut at the byte bound: %q",
			len([]rune(wide)), len(wide), got)
	}
}

func TestARowEscapesABarInItsResponsibility(t *testing.T) {
	// A bar ends a table cell, so an unescaped one moves every column after it
	// and the map stops parsing as a table.
	page := Render([]Package{{Path: "internal/core/x", Responsibility: "reads a | b"}})

	if !strings.Contains(page, `reads a \| b`) {
		t.Errorf("the bar is not escaped:\n%s", page)
	}
}

func TestRegistrationReadsANameHeldInAConstant(t *testing.T) {
	root := tree(t, map[string]string{
		"internal/plugins/p/p.go": "package p\n",
		"internal/plugins/p/register.go": "package p\n\nconst Name = \"the-plugin\"\n\n" +
			"var _ = R{Name: Name, Description: \"what it does\"}\n",
	})

	packages, err := Build(root)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(packages) != 1 {
		t.Fatalf("Build found %d packages: %+v", len(packages), packages)
	}
	if packages[0].Registered != "the-plugin" {
		t.Errorf("the registered name is %q, want the constant's value", packages[0].Registered)
	}
	if packages[0].Responsibility != "what it does" {
		t.Errorf("the responsibility is %q, want the registry Description", packages[0].Responsibility)
	}
}

func TestAnEmbedOnlyDirectoryEarnsNoRow(t *testing.T) {
	root := tree(t, map[string]string{
		"internal/plugins/p/p.go":          "// Package p does p.\npackage p\n",
		"internal/plugins/p/yang/embed.go": "package yang\n",
	})

	packages, err := Build(root)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, pkg := range packages {
		if strings.HasSuffix(pkg.Path, "/yang") {
			t.Errorf("an embed-only directory earned a row: %+v", pkg)
		}
	}
}

func TestCheckAndUpdateAgreeAboutOneTree(t *testing.T) {
	root := tree(t, map[string]string{
		"ai/.keep":                     "",
		"internal/core/thing/thing.go": "// Package thing does a thing.\npackage thing\n",
	})

	stale, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !stale.Stale {
		t.Error("an index that is not there reads as current")
	}

	written, err := Update(root)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !written.Written {
		t.Error("Update did not report writing")
	}

	fresh, err := Check(root)
	if err != nil {
		t.Fatalf("Check after Update: %v", err)
	}
	if fresh.Stale {
		t.Error("the index Update just wrote reads as stale")
	}
}

func TestAnswerRefusesATreeWithNoAIDirectory(t *testing.T) {
	root := tree(t, map[string]string{
		"internal/core/thing/thing.go": "// Package thing does a thing.\npackage thing\n",
	})

	if _, err := Check(root); !errors.Is(err, ErrNoAIDir) {
		t.Errorf("Check answered %v over a tree with nowhere to put the index", err)
	}
}

func TestFeedsNamesTheIndexEachSourceDrifts(t *testing.T) {
	for _, tc := range []struct {
		path, header string
		want         bool
	}{
		{OutputRel, "", true},
		{"internal/le/discoveryindex/discoveryindex.go", "", true},
		{"internal/plugins/p/register.go", "", true},
		{"internal/core/x/x.go", "// Package x does x.", true},
		{"internal/core/x/x.go", "package x", false},
		{"internal/core/x/x_test.go", "// Package x does x.", false},
		{"docs/architecture/one.md", "", false},
	} {
		if got := IsSource(tc.path, tc.header); got != tc.want {
			t.Errorf("IsSource(%q, header=%q) = %v, want %v", tc.path, tc.header, got, tc.want)
		}
	}
}

func TestReportIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	raw, err := json.Marshal(Report{
		File:     OutputRel,
		Packages: []Package{{Path: "internal/core/x", Responsibility: "does x", Registered: "x"}},
	})
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	for _, want := range []string{`"file"`, `"packages"`, `"path"`, `"responsibility"`, `"registered"`, `"todo"`, `"stale"`, `"written"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the payload has no %s key: %s", want, raw)
		}
	}
	if strings.Contains(string(raw), "_") {
		t.Errorf("a JSON key is snake_case: %s", raw)
	}
}

func TestTheAreaHoldsBothNativeActionsAndOnlyUpdateWrites(t *testing.T) {
	list := Actions()

	if len(list.Actions) != 2 {
		t.Fatalf("the area holds %d actions, want two", len(list.Actions))
	}
	for _, row := range list.Actions {
		switch row.Verb {
		case "check":
			if row.Writes {
				t.Error("check is marked as writing")
			}
		case "update":
			if !row.Writes {
				t.Error("update is not marked as writing")
			}
		default:
			t.Errorf("an unexpected action: %+v", row)
		}
	}
	if !strings.Contains(Subs(), "update (writes)") {
		t.Errorf("help does not say which action writes: %q", Subs())
	}
}

func TestAStaleIndexAnswersTheCodeTheCommitGateReads(t *testing.T) {
	// 3 rather than 1, because the commit gate BLOCKS on drift and stays
	// warn-only on a generator that failed. A flattened 1 turns a blocking gate
	// into an advisory one.
	root := tree(t, map[string]string{
		"ai/PACKAGE-MAP.md":            "stale\n",
		"internal/core/thing/thing.go": "// Package thing does a thing.\npackage thing\n",
	})
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	if _, code := Answer([]string{"check"}); code != StaleExit {
		t.Errorf("a stale index answered %d, want %d", code, StaleExit)
	}
	if _, code := Answer([]string{"update"}); code != 0 {
		t.Errorf("update answered %d over a stale index", code)
	}
}
