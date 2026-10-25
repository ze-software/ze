package human

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// VALIDATES: a whole-tree verb is admitted only when stdin and stdout are both
// terminals; every other answer, an unanswerable question included, refuses
// with the stream named, the reason, the given command line and the scoped
// command, each command on its own line.
// PREVENTS: an agent's tool call, whose streams are pipes, starting a full
// verification on its own.

// answers is a Question that answers from a table keyed by stream, so a test
// can be a terminal it is not.
func answers(stdin, stdout *os.File, stdinAnswer, stdoutAnswer bool, failing *os.File) Question {
	return func(stream *os.File) (bool, error) {
		if stream == failing {
			return false, errors.New("bad file descriptor")
		}
		if stream == stdin {
			return stdinAnswer, nil
		}
		if stream == stdout {
			return stdoutAnswer, nil
		}
		return false, errors.New("an unknown stream was asked about")
	}
}

// twoStreams answers two distinct open files to stand for stdin and stdout.
func twoStreams(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	stdin, stdout, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()  //nolint:errcheck // test cleanup of a pipe end
		_ = stdout.Close() //nolint:errcheck // test cleanup of a pipe end
	})
	return stdin, stdout
}

const givenLine = "./le verify worktree commit HEAD"

// assertRefusal checks every part AC-8 names: the condition, the reason, and
// the two commands on lines of their own.
func assertRefusal(t *testing.T, err error, condition string) {
	t.Helper()
	var refused *Refusal
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v, want a *Refusal", err)
	}
	text := refused.Error()
	for _, want := range []string{
		"a release check only Thomas starts",
		condition,
		"loads the whole shared machine",
		"\n    " + givenLine + "\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("refusal lacks %q:\n%s", want, text)
		}
	}
	if !strings.HasSuffix(text, "\n    "+ScopedCommand) {
		t.Errorf("refusal does not end with %q on its own line:\n%s", ScopedCommand, text)
	}
}

// TestGateRefusesEachNonTerminalStream is AC-8 for the three non-terminal
// combinations.
func TestGateRefusesEachNonTerminalStream(t *testing.T) {
	stdin, stdout := twoStreams(t)
	cases := []struct {
		name          string
		stdinTerminal bool
		outTerminal   bool
		condition     string
	}{
		{name: "stdin only", stdinTerminal: false, outTerminal: true, condition: "stdin is not a terminal"},
		{name: "stdout only", stdinTerminal: true, outTerminal: false, condition: "stdout is not a terminal"},
		{name: "both", stdinTerminal: false, outTerminal: false, condition: "neither stdin nor stdout is a terminal"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			question := answers(stdin, stdout, testCase.stdinTerminal, testCase.outTerminal, nil)
			assertRefusal(t, Check(question, stdin, stdout, givenLine), testCase.condition)
		})
	}
}

// TestGateRefusesWhenQuestionErrors is AC-9: a question that cannot be answered
// refuses, naming the stream and the error, and is never read as a terminal.
func TestGateRefusesWhenQuestionErrors(t *testing.T) {
	stdin, stdout := twoStreams(t)
	for name, failing := range map[string]*os.File{"stdin": stdin, "stdout": stdout} {
		t.Run(name, func(t *testing.T) {
			question := answers(stdin, stdout, true, true, failing)
			assertRefusal(t, Check(question, stdin, stdout, givenLine),
				"whether "+name+" is a terminal could not be answered (bad file descriptor)")
		})
	}
	if _, err := IsTerminal(nil); err == nil {
		t.Error("IsTerminal(nil) answered, want an error")
	}
	closed, _ := twoStreams(t)
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := IsTerminal(closed); err == nil {
		t.Error("IsTerminal of a closed file answered, want an error")
	}
}

// TestGateAdmitsTwoTerminals is AC-10: two terminals admit, with no keyword or
// environment variable involved. A pipe is not a terminal, so the real question
// refuses it, which is the agent's case.
func TestGateAdmitsTwoTerminals(t *testing.T) {
	stdin, stdout := twoStreams(t)
	if err := Check(answers(stdin, stdout, true, true, nil), stdin, stdout, givenLine); err != nil {
		t.Fatalf("two terminals refused: %v", err)
	}
	assertRefusal(t, Check(IsTerminal, stdin, stdout, givenLine), "neither stdin nor stdout is a terminal")
}

// TestHumanGateIsNotAnsweredByAPrompt proves the whole-tree gate takes its
// answer only from the process streams, so le's prompting for a missing value
// (internal/le/le/root prompt.go, over internal/component/cli/menu) can never
// answer it: no file of this package imports the prompt, the menu or the
// dispatcher that drives them, and RequireTerminal asks os.Stdin and os.Stdout
// themselves, exactly as before prompting existed (AC-47). Method: parse the
// package's own source and read its imports and RequireTerminal's body.
func TestHumanGateIsNotAnsweredByAPrompt(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{"internal/component/cli/menu", "internal/le/le/root", "bubbletea"}
	sawGate := false
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, readErr := os.ReadFile(name)
		if readErr != nil {
			t.Fatal(readErr)
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, source, parser.ImportsOnly)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		for _, spec := range file.Imports {
			path, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil {
				t.Fatal(unquoteErr)
			}
			for _, banned := range forbidden {
				if strings.Contains(path, banned) {
					t.Errorf("%s imports %s: the gate must not be reachable from a prompt", name, path)
				}
			}
		}
		if strings.Contains(string(source), "return Check(IsTerminal, os.Stdin, os.Stdout, commandLine)") {
			sawGate = true
		}
	}
	if !sawGate {
		t.Error("RequireTerminal no longer asks os.Stdin and os.Stdout itself")
	}
}
