package commit

// VALIDATES: scriptBase reads back what the earlier blocks of a prepared script
// commit under plan/, last block winning, and refuses a script it cannot read
// rather than answering HEAD. PREVENTS: an appended closure judged against a
// tree its earlier blocks do not produce.

import (
	"path/filepath"
	"strings"
	"testing"
)

// snapshotEntry writes content as a blob in root's object store and answers the
// `git ls-files -s` line a block's snapshot holds for path.
func snapshotEntry(t *testing.T, root, path, content string) string {
	t.Helper()
	scratch := filepath.Join("tmp", "blob-"+strings.ReplaceAll(path, "/", "-"))
	writeCommitFixture(t, root, scratch, content)
	object := strings.TrimSpace(runCommitGitOutput(t, root, "hash-object", "-w", "--", scratch))
	return "100644 " + object + " 0\t" + path
}

// TestScriptBaseReadsEarlierBlocks proves the reading over scripts rendered by
// renderPrivateIndex itself. Method: render two blocks naming one plan/ path,
// a removal, and a path re-added after its removal, then read the base.
func TestScriptBaseReadsEarlierBlocks(t *testing.T) {
	root := newCommitRepository(t)
	class := "plan/journal/native.md"
	spec := "plan/release/fixes/spec-a.md"
	gone := "plan/release/fixes/spec-gone.md"
	script := renderPrivateIndex(commitBlock{IndexEntries: []string{
		snapshotEntry(t, root, class, "first\n"),
		snapshotEntry(t, root, spec, "kept\n"),
		snapshotEntry(t, root, "internal/x.go", "package x\n"),
	}}) + "\n" +
		renderPrivateIndex(commitBlock{Removed: []string{spec, gone}}) + "\n" +
		renderPrivateIndex(commitBlock{IndexEntries: []string{
			snapshotEntry(t, root, class, "second\n"),
			snapshotEntry(t, root, spec, "back\n"),
		}}) + "\n"

	base, err := scriptBase(root, "tmp/commit-x.sh", script)
	if err != nil {
		t.Fatalf("scriptBase: %v", err)
	}
	if got := base.Files[class]; got != "second\n" {
		t.Fatalf("Files[%s] = %q, want the last block's %q", class, got, "second\n")
	}
	if got := base.Files[spec]; got != "back\n" || base.Removed[spec] {
		t.Fatalf("spec re-added after its removal: Files %q, Removed %v; want Files %q, not removed", got, base.Removed[spec], "back\n")
	}
	if !base.Removed[gone] {
		t.Fatalf("Removed = %v, want %s removed", base.Removed, gone)
	}
	if _, laid := base.Files["internal/x.go"]; laid {
		t.Fatal("a path outside plan/ was laid over the base")
	}
}

// TestScriptBaseRefusesWhatItCannotRead proves each unreadable shape is an
// error naming the script and the line, never a base that falls back to HEAD.
func TestScriptBaseRefusesWhatItCannotRead(t *testing.T) {
	root := newCommitRepository(t)
	entry := snapshotEntry(t, root, "plan/journal/native.md", "row\n")
	tests := []struct {
		name   string
		script string
		line   string
	}{
		// The script ends on the entry: a trailing empty line would be refused
		// first, as a snapshot line with no path.
		{name: "snapshot heredoc never closed", script: "set -e\n" + indexInfoOpen + "\n" + entry, line: "line 2"},
		{name: "unknown private update-index line", script: "set -e\n" + privateIndexUpdate + " --add foo\n", line: "line 2"},
		{name: "malformed snapshot line outside plan/", script: indexInfoOpen + "\n100644 deadbeef\tinternal/x.go\n" + indexInfoDelimiter + "\n", line: "line 2"},
		{name: "snapshot line with no path", script: indexInfoOpen + "\nnot an entry\n" + indexInfoDelimiter + "\n", line: "line 2"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := scriptBase(root, "tmp/commit-x.sh", test.script)
			if err == nil {
				t.Fatal("scriptBase read an unreadable script")
			}
			for _, want := range []string{"tmp/commit-x.sh", test.line, "./le commit create replace"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("scriptBase error = %q, want it to contain %q", err, want)
				}
			}
		})
	}
}

// TestAppendBaseJudgesReplaceOnHead proves only an appended block reads the
// script: replace drops the earlier blocks, so it lands on HEAD.
func TestAppendBaseJudgesReplaceOnHead(t *testing.T) {
	root := newCommitRepository(t)
	script := "tmp/commit-x.sh"
	writeCommitFixture(t, root, script, scriptMarker+"\n"+indexInfoOpen+"\n")
	base, err := appendBase(root, "x", &Options{Replace: true, Script: script})
	if err != nil {
		t.Fatalf("appendBase(replace): %v", err)
	}
	if base.Files != nil || base.Removed != nil {
		t.Fatalf("appendBase(replace) = %+v, want the zero Base (HEAD)", base)
	}
	if _, err := appendBase(root, "x", &Options{Append: true, Script: script}); err == nil {
		t.Fatal("appendBase(append) read a script whose snapshot heredoc never closes")
	}
}
