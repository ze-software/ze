// Related: completion.go -- the callback these tests drive

package completion_test

import (
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	_ "github.com/ze-software/ze/internal/le" // the le root handler and every area
	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

var registerProbe sync.Once

// probeArea registers one area whose action carries every shape the callback
// must apply from the table: a positional, a switch, an enum, a repeating
// keyword, a pair and a retired keyword.
func probeArea() {
	registerProbe.Do(func() {
		area := leaction.New("completionprobe", leaction.Action{
			Verb: "run", Why: "run the probe",
			Parameters: []leaction.Parameter{
				{Kind: leaction.KindEnum, Value: "target", Values: []string{"alpha", "beta"}, Requirement: leaction.Optional, Help: "the target"},
				{Keyword: "dry", Kind: leaction.KindSwitch, Help: "change nothing"},
				{Keyword: "step", Kind: leaction.KindEnum, Value: "step", Values: []string{"build", "test"}, Requirement: leaction.Optional, Help: "the step"},
				{Keyword: "file", Kind: leaction.KindPath, Value: "path", Requirement: leaction.Optional, Repeat: true, Help: "a file"},
				{Keyword: "hunks", Kind: leaction.KindPath, Value: "path", Pair: "patch", Requirement: leaction.Optional, Help: "a patch"},
				{Keyword: "old", Retired: "file"},
			},
			AnswerArgs: func(leaction.Arguments) (any, int) { return nil, 0 },
		})
		leroot.Register(area.Name(), leroot.GroupReport, area.Answer, registry.Meta{
			ShortHelp: "a probe for the completion callback", Mode: "offline", Section: registry.SectionTest,
		})
		leroot.RegisterActions(area.Name(), area.Actions)
	})
}

// values answers the candidate words of one completion, and nothing for a
// completion le refused (unknownRefused asserts the refusal itself).
func values(words ...string) []string {
	candidates, err := leroot.Complete(words)
	if err != nil {
		return nil
	}
	var out []string
	for _, candidate := range candidates {
		out = append(out, candidate.Value)
	}
	return out
}

// VALIDATES: AC-33 -- the callback offers what the table allows next: a
// keyword once unless it repeats, never a retired keyword, an enum's values
// only after its keyword, the positional's values only in the first slot, and
// nothing for the second value of a pair.
// PREVENTS: completion offering a line the parser refuses, which the tree's
// own TreeCompleter does for an enum before its keyword.
func TestCompletionCallbackHonoursRepeatAndRetired(t *testing.T) {
	probeArea()
	first := values("completionprobe", "run", "")
	for _, want := range []string{"alpha", "beta", "dry", "step", "file", "hunks"} {
		if !slices.Contains(first, want) {
			t.Errorf("first slot: %q not offered in %q", want, first)
		}
	}
	for _, refused := range []string{"old", "build", "test"} {
		if slices.Contains(first, refused) {
			t.Errorf("first slot: %q offered in %q", refused, first)
		}
	}
	later := values("completionprobe", "run", "dry", "file", "a", "")
	if slices.Contains(later, "dry") {
		t.Errorf("a switch already given is offered again: %q", later)
	}
	if !slices.Contains(later, "file") {
		t.Errorf("a repeating keyword is not offered again: %q", later)
	}
	if slices.Contains(later, "alpha") {
		t.Errorf("the positional is offered after the first slot: %q", later)
	}
	if got := values("completionprobe", "run", "step", ""); !slices.Equal(got, []string{"build", "test"}) {
		t.Errorf("after step: got %q, want its two values", got)
	}
	if got := values("completionprobe", "run", "step", "b"); !slices.Equal(got, []string{"build"}) {
		t.Errorf("after step b: got %q, want build", got)
	}
	if got := values("completionprobe", "run", "hunks", "x", ""); len(got) != 0 {
		t.Errorf("the second value of a pair: got %q, want nothing", got)
	}
	if got := values("completionprobe", "r"); !slices.Equal(got, []string{"run"}) {
		t.Errorf("a verb prefix: got %q, want run", got)
	}
	if got, err := leroot.Complete([]string{"nosuch", ""}); !errors.Is(err, leroot.ErrUnknownWord) || len(got) != 0 {
		t.Errorf("after an unknown word: got %v, %v, want nothing and ErrUnknownWord", got, err)
	}
	if got, err := leroot.Complete([]string{"spec", "nosuch", ""}); !errors.Is(err, leroot.ErrUnknownWord) || len(got) != 0 {
		t.Errorf("after an unknown verb: got %v, %v, want nothing and ErrUnknownWord", got, err)
	}
	// The real commit table: subject is not offered once given, and none of
	// the three retired create keywords is ever offered.
	create := values("commit", "create", "subject", "x", "")
	for _, refused := range []string{"subject", "push", "unverified", "missing-full-verify-ok"} {
		if slices.Contains(create, refused) {
			t.Errorf("commit create: %q offered in %q", refused, create)
		}
	}
	if !slices.Contains(create, "body") {
		t.Errorf("commit create: body not offered in %q", create)
	}
	if got := values("spec", "cl"); !slices.Equal(got, []string{"claim"}) {
		t.Errorf("spec cl: got %q, want claim", got)
	}
}

// VALIDATES: AC-36 -- `le completion words` run through the le root handler
// runs no before-dispatch hook and prints only candidate records on stdout.
// PREVENTS: a keystroke starting a hook's work, or a repair line corrupting
// the shell's candidate list.
func TestCompletionCallbackRunsNoBeforeDispatchHook(t *testing.T) {
	var ran bool
	leroot.RegisterBeforeDispatch("completionprobe-hook", func(string) { ran = true })
	handler := registry.LookupRoot("le")
	if handler == nil {
		t.Fatal("no le root handler registered")
	}
	out := captureStdout(t, func() {
		if code := handler(nil, []string{"completion", "words", "spec", "cl"}); code != 0 {
			t.Errorf("exit %d, want 0", code)
		}
	})
	if ran {
		t.Error("a before-dispatch hook ran for the completion callback")
	}
	for line := range strings.SplitSeq(strings.TrimSuffix(out, "\n"), "\n") {
		if strings.Count(line, "\t") != 1 {
			t.Errorf("stdout line %q is not one candidate record", line)
		}
	}
	if !strings.HasPrefix(out, "claim\t") {
		t.Errorf("stdout %q does not offer claim", out)
	}
}

// captureStdout runs fn with os.Stdout redirected and answers what it wrote.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = writer
	done := make(chan string, 1)
	go drain(reader, done)
	fn()
	os.Stdout = saved
	writer.Close() //nolint:errcheck // test pipe
	return <-done
}

// drain reads the pipe to its end and hands the text over.
func drain(reader io.Reader, done chan<- string) {
	data, err := io.ReadAll(reader)
	if err != nil {
		done <- "read error: " + err.Error()
		return
	}
	done <- string(data)
}
