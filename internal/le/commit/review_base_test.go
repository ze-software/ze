package commit

// VALIDATES: prospectiveTree applies earlier blocks in order and refuses
// unreadable scripts instead of selecting another source. PREVENTS: an appended
// closure judged against a tree its earlier blocks do not produce.

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

// TestScriptBaseReadsEarlierBlocks proves last-wins content, actual removal and
// re-addition through the prospective tree built from rendered script blocks.
func TestScriptBaseReadsEarlierBlocks(t *testing.T) {
	root := newCommitRepository(t)
	class := "plan/journal/native.md"
	spec := "plan/release/fixes/spec-a.md"
	gone := "plan/release/fixes/spec-gone.md"
	commitFixture(t, root, "baseline removal", map[string]string{gone: "gone\n"}, nil)
	head := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
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

	content, err := prospectiveTree(root, head, "tmp/commit-x.sh", script, nil, nil)
	if err != nil {
		t.Fatalf("prospectiveTree: %v", err)
	}
	if got := runCommitGitOutput(t, root, "show", content.before+":"+class); got != "second\n" {
		t.Fatalf("class content = %q, want the last block's bytes", got)
	}
	if got := runCommitGitOutput(t, root, "show", content.before+":"+spec); got != "back\n" {
		t.Fatalf("spec re-addition content = %q", got)
	}
	if status, _ := gitExit(root, "cat-file", "-e", content.before+":"+gone); status == 0 {
		t.Fatal("the earlier block's removal retained the baseline path")
	}
	if got := runCommitGitOutput(t, root, "show", content.before+":internal/x.go"); got != "package x\n" {
		t.Fatalf("prospective tree lost exact non-plan content: %q", got)
	}
}

// TestScriptBaseRefusesWhatItCannotRead proves each unreadable shape is an
// error naming the script and the line, never a base that falls back to HEAD.
func TestScriptBaseRefusesWhatItCannotRead(t *testing.T) {
	root := newCommitRepository(t)
	entry := snapshotEntry(t, root, "plan/journal/native.md", "row\n")
	head := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
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
			_, err := prospectiveTree(root, head, "tmp/commit-x.sh", test.script, nil, nil)
			if err == nil {
				t.Fatal("prospectiveTree read an unreadable script")
			}
			for _, want := range []string{"tmp/commit-x.sh", test.line, "./le commit create replace"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("prospectiveTree error = %q, want it to contain %q", err, want)
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
	path, earlier, err := appendScript(root, "x", &Options{Replace: true, Script: script})
	if err != nil {
		t.Fatalf("appendScript(replace): %v", err)
	}
	if path != "" || earlier != "" {
		t.Fatalf("appendScript(replace) = %q, %q; want no earlier overlay", path, earlier)
	}
	path, earlier, err = appendScript(root, "x", &Options{Append: true, Script: script})
	if err != nil {
		t.Fatalf("appendScript(append) masked the parser input: %v", err)
	}
	head := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
	if _, err := prospectiveTree(root, head, path, earlier, nil, nil); err == nil {
		t.Fatal("prospectiveTree accepted the malformed earlier script")
	} else if !strings.Contains(err.Error(), "snapshot line with no path") {
		t.Fatalf("prospectiveTree refused outside the empty snapshot-entry branch: %v", err)
	}
}
