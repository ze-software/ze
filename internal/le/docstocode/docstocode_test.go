// VALIDATES: spec-le-is-a-ze-binary AC-11 -- the reverse index from `// Design:`
// headers is derived from the tree, and a tree the generator cannot read is
// refused rather than described.
// PREVENTS: an unreadable file counted as a file that cites no design doc,
// whose write half then commits that omission into ai/DOCS-TO-CODE.md.

package docstocode

import (
	"encoding/json"
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

func TestBuildReadsTheDesignHeader(t *testing.T) {
	root := tree(t, map[string]string{
		"internal/a/a.go": "// Design: docs/architecture/x.md -- the topic\npackage a\n",
	})

	refs, err := Build(root)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("Build found %d references, want 1: %+v", len(refs), refs)
	}
	if refs[0].Doc != "docs/architecture/x.md" || refs[0].File != "internal/a/a.go" {
		t.Errorf("reference is %+v", refs[0])
	}
	if refs[0].Topic != "the topic" {
		t.Errorf("topic is %q, want the separator removed", refs[0].Topic)
	}
}

func TestBuildRefusesAFileItCannotRead(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file, so the case cannot be built")
	}
	root := tree(t, map[string]string{
		"internal/a/a.go": "// Design: docs/architecture/x.md -- the topic\npackage a\n",
	})
	if err := os.Chmod(filepath.Join(root, "internal", "a", "a.go"), 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	_, err := Build(root)
	if err == nil {
		t.Fatal("Build described a tree holding a file it could not read")
	}
	if !strings.Contains(err.Error(), "a.go") {
		t.Errorf("the error does not name the file: %v", err)
	}
}

func TestBuildReportsADanglingSymbolicLink(t *testing.T) {
	// A link whose target is gone is a directory entry that is STILL THERE and
	// cannot be read. Skipping it drops every citation the file made, and the
	// write half then commits that omission.
	root := tree(t, map[string]string{
		"internal/a/a.go": "// Design: docs/architecture/x.md -- topic\npackage a\n",
	})
	if err := os.Symlink(filepath.Join(root, "gone.go"), filepath.Join(root, "internal", "a", "b.go")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if _, err := Build(root); err == nil {
		t.Fatal("Build described a tree holding a link that resolves to nothing")
	}
}

func TestDesignLineReadsEachSeparator(t *testing.T) {
	for _, tc := range []struct{ line, doc, topic string }{
		{"// Design: docs/a/b.md -- dashes", "docs/a/b.md", "dashes"},
		{"// Design: docs/a/b.md - dash", "docs/a/b.md", "dash"},
		{"// Design: docs/a/b.md — em dash", "docs/a/b.md", "em dash"},
		{"// Design: docs/a/b.md : colon", "docs/a/b.md", "colon"},
		{"// Design: docs/a/b.md plain", "docs/a/b.md", "plain"},
		{"//Design: docs/a/b.md", "docs/a/b.md", ""},
	} {
		doc, topic, ok := designLine(tc.line)
		if !ok || doc != tc.doc || topic != tc.topic {
			t.Errorf("designLine(%q) = (%q, %q, %v), want (%q, %q, true)",
				tc.line, doc, topic, ok, tc.doc, tc.topic)
		}
	}
	if _, _, ok := designLine("// something else"); ok {
		t.Error("a line that is not a citation was read as one")
	}
}

func TestOnlyAMarkdownPathInADirectoryIsADocument(t *testing.T) {
	for _, tc := range []struct {
		token string
		want  bool
	}{
		{"docs/architecture/one.md", true},
		{"one.md", false},
		{"docs/architecture/one", false},
		{"(none", false},
	} {
		if got := isDocPath(tc.token); got != tc.want {
			t.Errorf("isDocPath(%q) = %v, want %v", tc.token, got, tc.want)
		}
	}
}

func TestARunOfCitationsCrossesFromBulletsIntoATable(t *testing.T) {
	// InlineRefs is the boundary, so the case one BELOW it must still be
	// bullets and the case one above must be a table.
	var refs []Reference
	for i := range InlineRefs {
		refs = append(refs, Reference{Doc: "docs/a.md", File: string(rune('a' + i)), Topic: "t"})
	}
	if page := Render(refs); strings.Contains(page, "| File | Topic |") {
		t.Errorf("a list at the bound became a table:\n%s", page)
	}

	refs = append(refs, Reference{Doc: "docs/a.md", File: "z", Topic: "t"})
	if page := Render(refs); !strings.Contains(page, "| File | Topic |") {
		t.Errorf("a list past the bound stayed bullets:\n%s", page)
	}
}

func TestTheWholeIndexIsRenderedByteForByte(t *testing.T) {
	// The Python half joins a line list whose final element is empty. Thus, the
	// blank line between two sections also stops the file. The file has one
	// newline after its final reference and a blank line before each heading.
	// A VERDICT comparison cannot see either fact, but both affect the committed
	// index bytes.
	const want = "# Documentation to Code Index\n" +
		"\n" +
		"<!-- GENERATED by ./le docs-to-code update -- do not edit -->\n" +
		"<!-- Regenerate: ./le docs-to-code update -->\n" +
		"\n" +
		"Given a design doc, the `.go` files that cite it in their `// Design:`\n" +
		"header. The inverse of `ai/CODE-TO-DOCS.md` (which is built from doc-side\n" +
		"`<!-- source: -->` anchors). See `ai/rules/go-standards.md`.\n" +
		"\n" +
		"## `docs/a.md`\n" +
		"\n" +
		"- `a.go` -- one\n" +
		"\n" +
		"## `docs/b.md`\n" +
		"\n" +
		"- `b.go`\n"

	got := Render([]Reference{
		{Doc: "docs/a.md", File: "a.go", Topic: "one"},
		{Doc: "docs/b.md", File: "b.go"},
	})
	if got != want {
		t.Errorf("the index is rendered differently:\nwant:\n%q\ngot:\n%q", want, got)
	}
}

func TestFourCitationsOfOneDocumentAreATable(t *testing.T) {
	// Both halves must share the size that changes a list into a table. A test
	// that uses InlineRefs cannot detect a moved constant. This test names the
	// literal size from the Python half.
	var refs []Reference
	for _, file := range []string{"a.go", "b.go", "c.go"} {
		refs = append(refs, Reference{Doc: "docs/a.md", File: file, Topic: "t"})
	}
	if page := Render(refs); strings.Contains(page, "| File | Topic |") {
		t.Errorf("three citations became a table:\n%s", page)
	}

	refs = append(refs, Reference{Doc: "docs/a.md", File: "d.go", Topic: "t"})
	if page := Render(refs); !strings.Contains(page, "| File | Topic |") {
		t.Errorf("four citations stayed bullets:\n%s", page)
	}
}

func TestCheckWritesAnIndexThatWasNeverThere(t *testing.T) {
	// The index is gitignored, so a clean checkout does not contain it. A
	// missing index is not stale. Reporting it as stale failed the doc gate on
	// fresh clones but passed after a user ran the generator.
	root := tree(t, map[string]string{
		"ai/.keep":        "",
		"internal/a/a.go": "// Design: docs/architecture/x.md -- topic\npackage a\n",
	})

	report, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Generated || report.Stale {
		t.Errorf("a missing index was reported as %+v", report)
	}

	fresh, err := Check(root)
	if err != nil {
		t.Fatalf("Check after Check: %v", err)
	}
	if fresh.Generated || fresh.Stale {
		t.Errorf("the index the first run wrote was reported as %+v", fresh)
	}
}

func TestReportIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	raw, err := json.Marshal(Report{
		File:       OutputRel,
		Docs:       1,
		References: []Reference{{Doc: "docs/a.md", File: "a.go", Topic: "t"}},
	})
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	for _, want := range []string{`"file"`, `"docs"`, `"references"`, `"doc"`, `"topic"`, `"stale"`, `"written"`, `"generated"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the payload has no %s key: %s", want, raw)
		}
	}
	if strings.Contains(string(raw), "_") {
		t.Errorf("a JSON key is snake_case: %s", raw)
	}
}

func TestOnlyTheUpdateActionWrites(t *testing.T) {
	list := Actions()

	// Four: the design-doc index and its MIRROR, the source-anchor reverse
	// index, each with one check and one update.
	if len(list.Actions) != 4 {
		t.Fatalf("the area holds %d actions, want four", len(list.Actions))
	}
	for _, row := range list.Actions {
		switch row.Verb {
		case "check":
			if row.Writes {
				t.Errorf("check is %+v", row)
			}
		case "update":
			if !row.Writes {
				t.Errorf("update is %+v", row)
			}
		case "index-check":
			if row.Writes {
				t.Errorf("the reverse index's check is %+v, and it writes nothing", row)
			}
		case "index-update":
			if !row.Writes {
				t.Errorf("the reverse index's update is %+v, and it rewrites the index", row)
			}
		default:
			t.Errorf("an unexpected action: %+v", row)
		}
	}
}

func TestAStaleIndexAnswersTheCodeACallerReadsApartFromOne(t *testing.T) {
	root := tree(t, map[string]string{
		"ai/DOCS-TO-CODE.md": "stale\n",
		"internal/a/a.go":    "// Design: docs/architecture/x.md -- topic\npackage a\n",
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

func TestTheModuleCacheContributesNoCitation(t *testing.T) {
	// The gokrazy module cache holds extracted external modules AND a self-copy
	// of ze, regenerated by appliance builds. Indexing it duplicates every
	// citation the working tree already makes, under paths no reader can open.
	// It is gitignored, so no comparison over the committed tree can see this.
	root := tree(t, map[string]string{
		"internal/a/a.go":                        "// Design: docs/architecture/x.md -- kept\npackage a\n",
		"gokrazy/modcache/ze@v1/internal/a/a.go": "// Design: docs/architecture/x.md -- a copy\npackage a\n",
	})

	refs, err := Build(root)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, ref := range refs {
		if strings.HasPrefix(ref.File, ModCache) {
			t.Errorf("the module cache earned a citation: %+v", ref)
		}
	}
	if len(refs) != 1 {
		t.Errorf("Build answered %d references, want the one outside the cache: %+v", len(refs), refs)
	}
}
