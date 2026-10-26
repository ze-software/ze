package rollback

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// VALIDATES: the handler a build reaches without the launcher refuses with
// exit 2, says why, and names the exact command that does the work.
// PREVENTS: a build run by its path answering 0 for a rollback it never made.
func TestRefusalNamesTheLauncherCommand(t *testing.T) {
	var stderr bytes.Buffer
	if code := refuse(&stderr); code != 2 {
		t.Errorf("refuse answered %d, want 2", code)
	}
	got := stderr.String()
	for _, want := range []string{"build selection", "--name", "bin/builds/", "./le rollback"} {
		if !strings.Contains(got, want) {
			t.Errorf("the refusal does not say %q:\n%s", want, got)
		}
	}
}

// VALIDATES: the bare line, the explicit verb and a stray value all answer 2
// with no payload, and a help word answers the listing with 0.
// PREVENTS: a bare `le rollback` from a build answering the action listing
// with 0, which a caller reads as a rollback that happened.
func TestEveryLineRefusesExceptHelp(t *testing.T) {
	// `newest` is the retired verb, a stray value to the verbless table now.
	for _, line := range [][]string{nil, {"newest"}, {"stray"}, {"newest", "stray"}} {
		payload, code := Answer(line)
		if code != 2 {
			t.Errorf("le rollback %v answered %d, want 2", line, code)
		}
		if payload != nil {
			t.Errorf("le rollback %v answered a payload %v, want none", line, payload)
		}
	}
	payload, code := Answer([]string{"help"})
	if code != 0 {
		t.Errorf("le rollback help answered %d, want 0", code)
	}
	list, ok := payload.(leaction.List)
	if !ok {
		t.Fatalf("le rollback help answered %T, want the action listing", payload)
	}
	if len(list.Actions) != 1 || !list.Verbless() || list.Actions[0].Does != "rollback" {
		t.Errorf("le rollback help listed %+v, want the one verbless action that does rollback", list.Actions)
	}
}

// VALIDATES: `le help` names rollback, read from the registry the dispatcher
// renders, not from a list this test holds.
// PREVENTS: an action only the le script knows, which help never names.
func TestRootHelpListsRollback(t *testing.T) {
	code := 1
	out := captureStdout(t, func() { code = leroot.Dispatch("le", []string{"help"}) })
	if code != 0 {
		t.Fatalf("le help answered %d, want 0", code)
	}
	if !strings.Contains(out, area) {
		t.Errorf("le help does not name %q:\n%s", area, out)
	}
}

// VALIDATES: completion offers rollback for a partial word, and its one action
// after it.
// PREVENTS: Tab completing nothing for a command the launcher accepts.
func TestCompletionOffersRollback(t *testing.T) {
	for _, tc := range []struct {
		words []string
		want  string
	}{
		{[]string{"roll"}, area},
		{[]string{area}, area},
	} {
		candidates, err := leroot.Complete(tc.words)
		if err != nil {
			t.Fatalf("complete %q: %v", tc.words, err)
		}
		values := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			values = append(values, candidate.Value)
		}
		if !slices.Contains(values, tc.want) {
			t.Errorf("complete %q offered %v, want %q among them", tc.words, values, tc.want)
		}
	}
}

// captureStdout answers what fn wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stdout
	os.Stdout = writer
	done := make(chan string, 1)
	go drain(reader, done)
	fn()
	_ = writer.Close()
	os.Stdout = saved
	out := <-done
	_ = reader.Close()
	return out
}

// drain reads reader to its end and sends what it read.
func drain(reader *os.File, done chan<- string) {
	var buffer bytes.Buffer
	_, _ = buffer.ReadFrom(reader)
	done <- buffer.String()
}
