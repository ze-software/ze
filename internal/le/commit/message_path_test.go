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
// create (AC-1 of plan/spec-commit-message-file-carries-its-own-suffix.md).
// Each script's own `git commit -F` line names its message, while session and
// tag are identical for both, so the script is the one source the message path
// is derived from (AC-2).
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

// messageArtifacts lists the message files the fixture's tmp/ holds, the names
// a later create would have to step around.
func messageArtifacts(t *testing.T, root string) []string {
	t.Helper()
	found, err := filepath.Glob(filepath.Join(root, "tmp", "commit-msg-*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// TestACreateThatWritesNoScriptLeavesNoMessageHoldingAName drives the two
// routes on which nextTag has reserved a message and no script follows: a dry
// run, and a create refused after the reservation.
//
// The method is to count the message files in tmp/ before and after each
// route. nextTag allocates with O_EXCL, so the reservation is a real file from
// that moment, and only Create's deferred cleanup removes it.
//
// VALIDATES: AC-4 of plan/spec-commit-message-file-carries-its-own-suffix.md, a
// failed or dry-run create leaves no message artifact behind.
// PREVENTS: an empty reservation that a later automatic tag walk reads as a
// taken letter, and that nothing ever runs or cleans.
// MUTATION: delete the os.Remove in Create's keepReservation defer and both
// halves go red.
func TestACreateThatWritesNoScriptLeavesNoMessageHoldingAName(t *testing.T) {
	t.Run("dry run", func(t *testing.T) {
		root := newCommitRepository(t)
		t.Setenv("CLAUDE_CODE_SESSION_ID", "commit-msg-dry-run")
		writeCommitFixture(t, root, "mine.txt", "mine\n")

		dry, err := Create(root, &Options{
			Subject: "a dry run", Files: []string{"mine.txt"}, DryRun: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if dry.MessageText == "" {
			t.Fatal("the dry run answered no message text, so it is not the route under test")
		}
		if left := messageArtifacts(t, root); len(left) != 0 {
			t.Fatalf("the dry run left message files behind: %q", left)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(dry.Script))); err == nil {
			t.Fatalf("the dry run wrote its script %s", dry.Script)
		}
	})
	t.Run("refused after the reservation", func(t *testing.T) {
		root := newCommitRepository(t)
		t.Setenv("CLAUDE_CODE_SESSION_ID", "commit-msg-refused")
		writeCommitFixture(t, root, "mine.txt", "mine\n")

		// Append with no prepared script is refused by targetScript, which runs
		// after nextTag has reserved the message.
		_, err := Create(root, &Options{
			Subject: "an append with nothing to append to", Files: []string{"mine.txt"}, Append: true,
		})
		if err == nil || !strings.Contains(err.Error(), "no prepared script") {
			t.Fatalf("the append was not refused after the reservation: %v", err)
		}
		if left := messageArtifacts(t, root); len(left) != 0 {
			t.Fatalf("the refused create left message files behind: %q", left)
		}
	})
}

// TestTheAutomaticTagWalkStepsOverATakenLetter drives nextTag's automatic walk
// through Create, then exhausts it.
//
// The method is two creates with no tag in one session, then a third session
// whose tmp/ already holds a message under every letter. The letter is the
// only part of the name the walk chooses, so a walk that ignored the suffixed
// files on disk would hand both creates the letter a.
//
// VALIDATES: AC-3 of plan/spec-commit-message-file-carries-its-own-suffix.md,
// and the a..z boundary: z is the last letter allocated, and past it create
// refuses rather than reusing one.
// PREVENTS: two prepared commits of one session reading as one tag in tmp/,
// and an exhausted walk falling back to a letter another commit holds.
// MUTATION: drop the `continue` on a non-empty glob in nextTag and the second
// create answers the letter a again.
func TestTheAutomaticTagWalkStepsOverATakenLetter(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "commit-msg-auto-tag")
	writeCommitFixture(t, root, "one.txt", "one\n")
	writeCommitFixture(t, root, "two.txt", "two\n")

	first, err := Create(root, &Options{Subject: "first automatic tag", Files: []string{"one.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Create(root, &Options{Subject: "second automatic tag", Files: []string{"two.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	firstTag := "-" + first.Session + "-a-"
	secondTag := "-" + second.Session + "-b-"
	if !strings.Contains(first.Message, firstTag) || !strings.Contains(first.Script, firstTag) {
		t.Fatalf("the first create did not take the letter a: message %s, script %s", first.Message, first.Script)
	}
	if !strings.Contains(second.Message, secondTag) || !strings.Contains(second.Script, secondTag) {
		t.Fatalf("the second create did not step over the taken a: message %s, script %s", second.Message, second.Script)
	}

	t.Setenv("CLAUDE_CODE_SESSION_ID", "commit-msg-auto-tag-full")
	full, err := Create(root, &Options{Subject: "learn the session", Files: []string{"one.txt"}, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	for code := byte('a'); code <= byte('y'); code++ {
		writeCommitFixture(t, root, "tmp/commit-msg-"+full.Session+"-"+string(code)+"-000000.txt", "held\n")
	}
	last, err := Create(root, &Options{Subject: "the last letter", Files: []string{"one.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(last.Message, "-"+full.Session+"-z-") {
		t.Fatalf("with a..y taken the walk answered %s, want the letter z", last.Message)
	}
	_, err = Create(root, &Options{Subject: "past the last letter", Files: []string{"two.txt"}})
	if err == nil || !strings.Contains(err.Error(), "no free message tag") {
		t.Fatalf("with every letter taken create answered %v, want the exhaustion refusal", err)
	}
}
