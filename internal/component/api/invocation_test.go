// Design: docs/architecture/api/commands.md -- structured parameters are written before the pipe chain.
package api

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// TestBuildInvocationWritesParamsBeforePipe asks whether a REST or gRPC
// command that holds a chain gets its structured parameters written into the
// command part, before the first pipe, so they reach the handler as arguments
// and the chain runs on the answer (AC-32).
//
// Method: prepareCommand's own steps over one line: ScanExecution answers the
// words and the pipe offset, and buildInvocation writes one parameter for a
// command the metadata declares. The built line is scanned again and judged by
// its parts: the words before the pipe, and the chain after it.
//
// Red today: a line holding a pipe gets its parameters appended after the whole
// input, so they land in the chain as words of `| count`.
// MUTATION: append the parameters after the whole input again; the command
// part loses `name a` and the chain gains it.
func TestBuildInvocationWritesParamsBeforePipe(t *testing.T) {
	const path = "show fixture pipe rows"
	const input = path + " | count"
	meta := &CommandMeta{Name: path, ReadOnly: true, Params: []ParamMeta{{Name: "name", Type: "string"}}}
	words, pipeAt, err := command.ScanExecution(input)
	if err != nil {
		t.Fatalf("scan %q: %v", input, err)
	}

	built, err := buildInvocation(input, map[string]string{"name": "a"}, words, pipeAt, meta)
	if err != nil {
		t.Fatalf("build %q: %v", input, err)
	}

	builtWords, builtPipeAt, err := command.ScanExecution(built)
	if err != nil {
		t.Fatalf("scan built line %q: %v", built, err)
	}
	if want := []string{"show", "fixture", "pipe", "rows", "name", "a"}; !slices.Equal(builtWords, want) {
		t.Errorf("command part of %q = %q, want %q", built, builtWords, want)
	}
	if builtPipeAt < 0 {
		t.Fatalf("built line %q lost its chain", built)
	}
	if chain := strings.TrimSpace(built[builtPipeAt+1:]); chain != "count" {
		t.Errorf("chain of %q = %q, want %q: a parameter landed in the chain", built, chain, "count")
	}
}

// TestInvocationKeepsPipeAndQuoteInValue asks whether a REST or gRPC
// parameter value holding a pipe or a double quote stays one argument and
// opens no operation, on a line with no chain and on a line with one (AC-11).
//
// Method: buildInvocation over the declared command, then the built line is
// scanned again with ScanExecution: the decoded command words must end in the
// parameter name and its whole value, and the chain must be exactly what the
// caller typed.
//
// MUTATION: write the value bare (drop WriteInvocationValue's quoting); the
// value's pipe then cuts the command part and the chain gains an operation.
func TestInvocationKeepsPipeAndQuoteInValue(t *testing.T) {
	const path = "show fixture pipe rows"
	const value = `a|b"c`
	meta := &CommandMeta{Name: path, ReadOnly: true, Params: []ParamMeta{{Name: "name", Type: "string"}}}
	for _, test := range []struct {
		input string
		chain string
	}{
		{input: path},
		{input: path + " | count", chain: "count"},
	} {
		t.Run(test.input, func(t *testing.T) {
			words, pipeAt, err := command.ScanExecution(test.input)
			if err != nil {
				t.Fatalf("scan %q: %v", test.input, err)
			}
			built, err := buildInvocation(test.input, map[string]string{"name": value}, words, pipeAt, meta)
			if err != nil {
				t.Fatalf("build %q: %v", test.input, err)
			}
			builtWords, builtPipeAt, err := command.ScanExecution(built)
			if err != nil {
				t.Fatalf("scan built line %q: %v", built, err)
			}
			if want := []string{"show", "fixture", "pipe", "rows", "name", value}; !slices.Equal(builtWords, want) {
				t.Errorf("command part of %q = %q, want %q", built, builtWords, want)
			}
			chain := ""
			if builtPipeAt >= 0 {
				chain = strings.TrimSpace(built[builtPipeAt+1:])
			}
			if chain != test.chain {
				t.Errorf("chain of %q = %q, want %q: the value opened an operation", built, chain, test.chain)
			}
		})
	}
}
