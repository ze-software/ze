package commit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scriptMessagePath reads the one message path a prepared script commits from.
// A script that names none, or names more than one, is not the artifact this
// test is about, so it fails rather than picking one.
func scriptMessagePath(t *testing.T, root, script string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(script))) //nolint:gosec // the fixture repository is a t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for line := range strings.SplitSeq(string(content), "\n") {
		_, path, isCommit := strings.Cut(strings.TrimSpace(line), "git commit -F ")
		if !isCommit {
			continue
		}
		found = append(found, strings.Trim(strings.Fields(path)[0], "'\""))
	}
	if len(found) != 1 {
		t.Fatalf("script %s names %d message paths, want 1: %q", script, len(found), found)
	}
	return found[0]
}

// TestTwoCreatesUnderOneTagKeepTheirOwnMessages drives the defect
// plan/journal/pointer-shared-across-the-names-it-indexes.md records four times.
//
// The script path carries a random suffix and the message path did not, so a
// second create under the same tag wrote a second script while OVERWRITING the
// first script's message. Both scripts stayed runnable, so running the first
// made a commit carrying the second's subject, with nothing printed to say so.
//
// It is driven from Create rather than from nextTag, because the whole defect is
// that the two paths were derived differently: a test over one path helper
// cannot see a disagreement between two.
//
// VALIDATES: two prepared commits under one tag in one session name two
// different message files, and the first one's content survives the second
// create.
// PREVENTS: a prepared commit being made under another commit's subject and
// body, which has reached main once.
func TestTwoCreatesUnderOneTagKeepTheirOwnMessages(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "commit-msg-collision")
	writeCommitFixture(t, root, "first.txt", "first\n")
	writeCommitFixture(t, root, "second.txt", "second\n")

	first, err := Create(root, &Options{
		Subject: "first prepared commit", Files: []string{"first.txt"},
		Tag: "shared",
	})
	if err != nil {
		t.Fatal(err)
	}
	firstText, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(first.Message))) //nolint:gosec // the fixture repository is a t.TempDir
	if err != nil {
		t.Fatal(err)
	}

	second, err := Create(root, &Options{
		Subject: "second prepared commit", Files: []string{"second.txt"},
		Tag: "shared",
	})
	if err != nil {
		t.Fatal(err)
	}

	if first.Script == second.Script {
		t.Fatalf("both creates answered one script path: %s", first.Script)
	}
	if first.Message == second.Message {
		t.Fatalf("both creates answered one message path: %s", first.Message)
	}

	// The script is what actually gets run, so the paths it names are what
	// decides the subject each commit lands with.
	if named := scriptMessagePath(t, root, first.Script); named != first.Message {
		t.Fatalf("first script commits from %s, want its own message %s", named, first.Message)
	}
	if named := scriptMessagePath(t, root, second.Script); named != second.Message {
		t.Fatalf("second script commits from %s, want its own message %s", named, second.Message)
	}

	afterText, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(first.Message))) //nolint:gosec // the fixture repository is a t.TempDir
	if err != nil {
		t.Fatalf("the second create destroyed the first commit's message: %v", err)
	}
	if !bytes.Equal(afterText, firstText) {
		t.Fatalf("the second create rewrote the first commit's message:\nbefore:\n%s\nafter:\n%s", firstText, afterText)
	}
	if !strings.Contains(string(afterText), "first prepared commit") {
		t.Fatalf("the first message no longer carries its own subject:\n%s", afterText)
	}
}

// TestAppendUnderOneTagGivesEachBlockItsOwnMessage covers the same defect on the
// route it was first met on: a two-commit closure prepared with `append` put two
// blocks in one script, and both named the same message file.
//
// VALIDATES: each block of an appended script commits from its own message.
// PREVENTS: a closure's first commit landing under the closing commit's message.
func TestAppendUnderOneTagGivesEachBlockItsOwnMessage(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "commit-msg-append")
	writeCommitFixture(t, root, "code.txt", "code\n")
	writeCommitFixture(t, root, "spec.txt", "spec\n")

	first, err := Create(root, &Options{
		Subject: "carry the code", Files: []string{"code.txt"},
		Tag: "closure",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Create(root, &Options{
		Subject: "carry the spec", Files: []string{"spec.txt"},
		Tag: "closure", Append: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Script != second.Script {
		t.Fatalf("append did not extend the first script: %s then %s", first.Script, second.Script)
	}
	if first.Message == second.Message {
		t.Fatalf("both blocks of one script name one message: %s", first.Message)
	}

	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(first.Script))) //nolint:gosec // the fixture repository is a t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range []string{first.Message, second.Message} {
		if !strings.Contains(string(content), message) {
			t.Fatalf("script does not commit from %s:\n%s", message, content)
		}
	}

	firstText, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(first.Message))) //nolint:gosec // the fixture repository is a t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(firstText), "carry the code") {
		t.Fatalf("the first block's message was overwritten by the second:\n%s", firstText)
	}
}
