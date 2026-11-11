// Design: docs/contributing/committing.md -- the keywords
// Related: input.go -- bodyOf, the reader these tests drive

package commit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeBodyFile writes text into a body file under a fresh directory and
// answers its path.
func writeBodyFile(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "body.txt")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestCreateBodyFileIsWrappedAndGatedLikeBody pins what body-file buys: a
// body whose prose names an environment-changing word or a heavy command,
// which the Bash hook would judge if it were typed inline, read from a file
// and turned into the message body exactly as the body keyword would.
//
// VALIDATES: a create line with body-file yields the same message as the same
// text given through body chunks, its paragraph break included, wrapped at
// 72; and a file line that starts with a declared trailer key is refused as
// a body line is.
// PREVENTS: a second body route that wraps differently or skips the
// hand-written-trailer gate.
func TestCreateBodyFileIsWrappedAndGatedLikeBody(t *testing.T) {
	first := "The hook read export FOO=1 and go build ./... in an inline body, so the commit could not be prepared at all."
	second := "  An indented second paragraph."
	path := writeBodyFile(t, first+"\n\n"+second+"\n")
	fromFile, err := parseCreate([]string{"subject", "s", "file", "a", "body-file", path})
	if err != nil {
		t.Fatalf("body-file refused: %v", err)
	}
	fromBody, err := parseCreate([]string{"subject", "s", "file", "a", "body", first, "body", "", "body", second})
	if err != nil {
		t.Fatalf("body refused: %v", err)
	}
	got, err := Message("s", fromFile.Body, nil)
	if err != nil {
		t.Fatalf("the body file's message was refused: %v", err)
	}
	want, err := Message("s", fromBody.Body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("body-file message:\n%s\nwant the body message:\n%s", got, want)
	}
	for line := range strings.SplitSeq(got, "\n") {
		if len(line) > messageWidth {
			t.Errorf("line over %d columns: %q", messageWidth, line)
		}
	}

	trailer := writeBodyFile(t, "Prose.\n"+trailerSpecWork+"some-spec\n")
	options, err := parseCreate([]string{"subject", "s", "file", "a", "body-file", trailer})
	if err != nil {
		t.Fatalf("body-file refused at parse: %v", err)
	}
	if _, err := Message("s", options.Body, nil); err == nil || !strings.Contains(err.Error(), "Spec-work:") {
		t.Errorf("a hand-written trailer line in the body file: err %v, want the trailer refusal", err)
	}
}

// TestCreateRefusesBodyBesideBodyFile pins the one refusal the two routes
// owe: a line that gives both has no single body.
//
// VALIDATES: body with body-file is refused, through the parser and through
// the create action's preparation (exit 2), naming both keywords and the fix.
// PREVENTS: one of the two bodies being dropped in silence.
func TestCreateRefusesBodyBesideBodyFile(t *testing.T) {
	path := writeBodyFile(t, "From the file.\n")
	line := []string{"subject", "s", "file", "a", "body", "inline", "body-file", path}
	_, err := parseCreate(line)
	if err == nil {
		t.Fatal("body beside body-file was accepted")
	}
	for _, want := range []string{"body", "body-file", "drop"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal %q lacks %q", err, want)
		}
	}
	parsed, err := table().Parse("create", line)
	if err != nil {
		t.Fatalf("the table refused the line itself: %v", err)
	}
	if _, code := prepareCreate(t.TempDir(), parsed); code != 2 {
		t.Errorf("prepareCreate answered %d, want 2", code)
	}
}

// TestCreateBodyFileRefusesAFileWithNoText is the survivor: a body file that
// cannot give a body refuses rather than commit an empty one.
//
// VALIDATES: an empty or blank file, and a missing one, are refused naming
// the file.
// PREVENTS: a silent empty body from a file the author meant to fill.
func TestCreateBodyFileRefusesAFileWithNoText(t *testing.T) {
	blank := writeBodyFile(t, " \n\n")
	missing := filepath.Join(t.TempDir(), "missing.txt")
	for name, path := range map[string]string{"blank": blank, "missing": missing} {
		t.Run(name, func(t *testing.T) {
			_, err := parseCreate([]string{"subject", "s", "file", "a", "body-file", path})
			if err == nil {
				t.Fatalf("a %s body file was accepted", name)
			}
			if !strings.Contains(err.Error(), path) {
				t.Errorf("the refusal %q does not name %s", err, path)
			}
		})
	}
}
