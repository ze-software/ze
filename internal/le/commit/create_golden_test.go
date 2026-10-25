// Design: docs/contributing/committing.md -- the message file and the script create writes
// Related: actions.go -- the create grammar this golden reads through Answer

package commit

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// goldenCreatePath is the message and script a representative create line
// produced through the hand-rolled keyword parser `commit` had before its
// grammar moved onto the action table. It was written once, from that code, and
// is never regenerated from the table, because then it would prove the table
// agrees with itself.
const goldenCreatePath = "testdata/create-golden.txt"

// goldenNonce matches the random suffix of a file name create chooses.
var goldenNonce = regexp.MustCompile(`-golden-[0-9a-f]{6}\.`)

// goldenCreateOutput runs one create line that names every kind of keyword a
// dry run carries (a repeated body with a paragraph break, file, file-list,
// hunks, remove, the admission reasons, tag, replace and dry-run) through
// Answer, the entry point an operator reaches, and answers the message text and
// the script text with the fixture's checkout path replaced, so the bytes
// compare across runs.
func goldenCreateOutput(t *testing.T) string {
	t.Helper()
	// The fixture commits name HEAD in the script, so their dates are fixed:
	// otherwise HEAD is a new SHA on every run.
	t.Setenv("GIT_AUTHOR_DATE", "2026-01-01T00:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2026-01-01T00:00:00Z")
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "create-golden-fixture")
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	configureCommitAuthor(t, root)
	path := "pkg/shared.txt"
	commitFixture(t, root, "base", map[string]string{path: hunkBase, "gone.txt": "gone\n"}, nil)
	patch := writeHunkPatch(t, root, path, hunkMine, hunkBoth)
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	writeCommitFixture(t, root, "docs/listed.md", "# Listed\n")
	list := filepath.Join(root, "tmp", "golden-paths")
	writeCommitFixture(t, root, "tmp/golden-paths", "# the listed paths\n\ndocs/listed.md\n")

	payload, code := Answer([]string{"create",
		"session", "0a1b2c3d", "tag", "golden", "subject", "commit: one golden create line",
		"body", "The first chunk of the body, long enough that the message wraps it at seventy-two columns.",
		"body", "", "body", "A second paragraph.",
		"file", "mine.txt", "file-list", list, "hunks", path, patch, "remove", "gone.txt",
		"no-test", "the golden fixture carries no test of its own",
		"structural-red-ok", "no structural red is charged here",
		"review-override", "no closure in this fixture",
		"rfc-change-ok", "no RFC-tagged unit in this fixture",
		"replace", "dry-run",
	})
	if code != 0 {
		t.Fatalf("create answered %d with %#v", code, payload)
	}
	prepared, ok := payload.(*Prepared)
	if !ok {
		t.Fatalf("create answered %T, want *Prepared", payload)
	}
	text := prepared.MessageText + "--- script ---\n" + prepared.ScriptText
	text = strings.ReplaceAll(text, root, "<root>")
	// Each script and message file name ends in a random nonce, so two runs
	// never share a path; the nonce is the only part of the bytes a run chooses.
	return goldenNonce.ReplaceAllString(text, "-golden-<nonce>.")
}

// TestCreateWritesTheBytesTheHandRolledParserWrote proves the move of create's
// grammar onto the action table changed no byte of what create writes: the
// message and the script for the same line equal the ones the deleted parser
// produced (goldenCreatePath).
//
// MUTATION: read body with One instead of Values, or drop the hunks pair, and
// the bytes differ.
func TestCreateWritesTheBytesTheHandRolledParserWrote(t *testing.T) {
	want, err := os.ReadFile(goldenCreatePath)
	if err != nil {
		t.Fatal(err)
	}
	got := goldenCreateOutput(t)
	if got == string(want) {
		return
	}
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")
	for index := range min(len(gotLines), len(wantLines)) {
		if gotLines[index] != wantLines[index] {
			t.Fatalf("create wrote line %d as\n%s\nwant the hand-rolled parser's\n%s", index+1, gotLines[index], wantLines[index])
		}
	}
	t.Fatalf("create wrote %d lines, want the hand-rolled parser's %d", len(gotLines), len(wantLines))
}
