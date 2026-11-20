package command

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

// TestServeLocalRefusesInvalidChainBeforeCallingProducer keeps validation in
// front of both source work and payload encoding. The handler deliberately
// returns a value encoding/json cannot marshal, so either operation happening
// before validation would replace the named pipe refusal.
func TestServeLocalRefusesInvalidChainBeforeCallingProducer(t *testing.T) {
	called := 0
	const path = "show test local validation order"
	if err := RegisterLocalData(path, func(ValidatedArgs) LocalAnswer {
		called++
		return LocalAnswer{Payload: func() {}}
	}, registry.Meta{}, func(string, any) int { return 0 }); err != nil {
		t.Fatalf("register local data handler: %v", err)
	}

	answer, code, served := ServeLocal(path+" | raw | json", "")
	if !served {
		t.Fatal("registered local data command was not served")
	}
	if code != 1 {
		t.Errorf("refused chain exit code = %d, want 1", code)
	}
	if called != 0 {
		t.Errorf("refused chain called its producer %d times, want none", called)
	}
	if !IsPipeError(answer) || !strings.Contains(answer, "raw") || !strings.Contains(answer, "json") {
		t.Errorf("refusal = %q, want a named raw/json pipe error", answer)
	}
}

// withArgDefSource installs source for the duration of a test and restores the
// one registered before. Not safe for parallel tests.
func withArgDefSource(t *testing.T, source ArgDefSource) {
	t.Helper()
	saved := argDefSource
	argDefSource = source
	t.Cleanup(func() { argDefSource = saved })
}

// TestServeLocalJudgesArgumentsAgainstTheirDefinitions proves the local route
// calls the argument validator before the handler.
//
// VALIDATES: a value one past its declared length is refused and never reaches
// the handler; a value at the bound does reach it.
// PREVENTS: the local route skipping argument validation, which let a
// 129-character `show env get` name past `length "1..128"`.
func TestServeLocalJudgesArgumentsAgainstTheirDefinitions(t *testing.T) {
	const path = "show test local argument length"
	var got []string
	if err := RegisterLocalData(path, func(args ValidatedArgs) LocalAnswer {
		got = args.Tokens()
		return LocalAnswer{Payload: map[string]any{"ok": true}}
	}, registry.Meta{}, func(string, any) int { return 0 }); err != nil {
		t.Fatalf("register local data handler: %v", err)
	}
	withArgDefSource(t, func(p string) ([]ArgDef, error) {
		if p != path {
			return nil, nil
		}
		return []ArgDef{mustArgDef(NewStringArg("name", []UintRange{{Min: 1, Max: 4}}, nil, ArgOptions{Mandatory: true}))}, nil
	})

	_, code, served := ServeLocal(path+" abcde", "")
	if !served {
		t.Fatal("registered local data command was not served")
	}
	if code != 1 {
		t.Errorf("over-long argument exit code = %d, want 1", code)
	}
	if got != nil {
		t.Errorf("over-long argument reached the handler as %q", got)
	}

	_, code, _ = ServeLocal(path+" abcd", "")
	if code != 0 {
		t.Errorf("argument at the bound exit code = %d, want 0", code)
	}
	if len(got) != 1 || got[0] != "abcd" {
		t.Errorf("handler args = %q, want [abcd]", got)
	}
}

// TestServeLocalRefusesWithoutArgDefSource proves a process that cannot read
// the definitions refuses instead of skipping the check.
//
// VALIDATES: no registered source means exit 1 and no handler call.
// PREVENTS: a missing source reading as "no constraints", the silent pass.
func TestServeLocalRefusesWithoutArgDefSource(t *testing.T) {
	const path = "show test local no source"
	called := 0
	if err := RegisterLocalData(path, func(ValidatedArgs) LocalAnswer {
		called++
		return LocalAnswer{Payload: map[string]any{}}
	}, registry.Meta{}, func(string, any) int { return 0 }); err != nil {
		t.Fatalf("register local data handler: %v", err)
	}
	withArgDefSource(t, nil)

	_, code, _ := ServeLocal(path, "")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if called != 0 {
		t.Errorf("handler called %d times, want none", called)
	}
}

// TestServeLocalHandsDeclaredOperations asks whether a local-data command
// answered in the caller's own process receives the operation it declares,
// from the chain typed after it, in its declared list (AC-24).
//
// Method: the command declares one operation, `only`, taking an argument. The
// line carries one argument before the pipe and that operation after it, and
// the handler records the arguments and the declared list it was handed.
//
// Red before the partition: ServeLocal folds the operation into a command line
// it then discards, so the handler is handed no declared list and the
// operation the operator typed is silently dropped.
// MUTATION: drop the declared list ServeLocal attaches; the operations
// comparison fails.
func TestServeLocalHandsDeclaredOperations(t *testing.T) {
	const path = "show test local declared operations"
	var tokens []string
	var operations []PipeOperation
	called := 0
	if err := RegisterLocalData(path, func(args ValidatedArgs) LocalAnswer {
		called++
		tokens = args.Tokens()
		operations = args.PipeOperations()
		return LocalAnswer{Payload: map[string]any{"ok": true}}
	}, registry.Meta{}, func(string, any) int { return 0 }); err != nil {
		t.Fatalf("register local data handler: %v", err)
	}
	RegisterPipeFilters([]string{path}, PipeFilter{Name: "only", Description: "Keep the rows of one state", Argument: ArgText})
	withArgDefSource(t, func(string) ([]ArgDef, error) { return nil, nil })

	answer, code, served := ServeLocal(path+" alpha | only up", "")
	if !served {
		t.Fatal("registered local data command was not served")
	}
	if code != 0 || called != 1 {
		t.Fatalf("exit code %d after %d handler calls, want 0 after 1 (answer %q)", code, called, answer)
	}
	if !slices.Equal(tokens, []string{"alpha"}) {
		t.Errorf("handler args = %q, want [alpha]: a chain word reached the handler as an argument", tokens)
	}
	want := []PipeOperation{{Name: "only", Args: []string{"up"}}}
	if !slices.EqualFunc(want, operations, sameLocalPipeOperation) {
		t.Errorf("declared list = %+v, want %+v", operations, want)
	}
}

// sameLocalPipeOperation compares two declared operations by name and argument
// words.
func sameLocalPipeOperation(a, b PipeOperation) bool {
	return a.Name == b.Name && slices.Equal(a.Args, b.Args)
}
