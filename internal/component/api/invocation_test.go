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
