// Design: docs/architecture/cli/plugin-modes.md -- query mode
// Related: sdk_query.go -- the entry point under test
package sdk

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// queryDeclaration is the declaration both tests hand the entry point. It
// carries one command and one pipe, so an answer that drops a field is visible.
func queryDeclaration() Registration {
	return Registration{
		Commands: []rpc.CommandDecl{{Name: "show acme state", Description: "Show acme state"}},
		Pipes: []rpc.PipeDecl{{
			Command:     "show acme state",
			Name:        "brief",
			Description: "One line for each entry",
			Expansion:   "columns name,state",
		}},
	}
}

// requestQueryMode puts the process in query mode for the duration of the test.
func requestQueryMode(t *testing.T) {
	t.Helper()

	require.NoError(t, env.Set(EnvPluginMode, ModeDeclare))
	t.Cleanup(func() { _ = env.Set(EnvPluginMode, "") })
}

// parseDeclaration reads the one framed line query mode writes, through the
// protocol's own parser, and returns the Stage 1 declaration it carries.
func parseDeclaration(t *testing.T, stdout string) rpc.DeclareRegistrationInput {
	t.Helper()

	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	require.Len(t, lines, 1, "query mode writes one line: %q", stdout)

	id, method, payload, err := rpc.ParseLine([]byte(lines[0]))
	require.NoError(t, err)
	assert.Equal(t, uint64(1), id, "the query answer carries the Stage 1 id")
	assert.Equal(t, rpc.MethodDeclareRegistration, method)

	var declaration rpc.DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(payload, &declaration))
	return declaration
}

// VALIDATES: AC-1 and AC-6 of spec-plugin-query-mode -- the SDK entry point
// answers a declaration query and leaves the activation function unreached.
// PREVENTS: a third-party plugin initializing data, connecting, binding or
// starting a timer while it is only being interrogated.
//
// The goal is the guarantee a third-party plugin buys by adopting the entry
// point: side-effecting code lives inside the activation function, and under
// the mode that function is never called. The method sets the mode variable,
// hands the entry point a function that records its own call, and reads the
// answer back through the protocol's own parser.
func TestQueryModeSkipsActivation(t *testing.T) {
	requestQueryMode(t)

	activated := false
	var stdout bytes.Buffer

	code := runOrDeclare(&stdout, queryDeclaration(), func() int {
		activated = true
		return 7
	})

	assert.False(t, activated, "the activation function ran under query mode")
	assert.Equal(t, 0, code)

	declaration := parseDeclaration(t, stdout.String())
	require.Len(t, declaration.Commands, 1)
	assert.Equal(t, "show acme state", declaration.Commands[0].Name)
	require.Len(t, declaration.Pipes, 1)
	assert.Equal(t, "brief", declaration.Pipes[0].Name)
}

// VALIDATES: a start with no mode variable is the start every plugin had
// before query mode existed.
// PREVENTS: the entry point swallowing a live start, or writing a declaration
// line into a running plugin's stdout.
//
// The goal is that a live start is unchanged. The method clears the mode
// variable and asserts the activation function ran, that its exit code is the
// one returned, and that stdout stayed empty.
func TestQueryModeActivatesWithoutTheMode(t *testing.T) {
	require.NoError(t, env.Set(EnvPluginMode, ""))

	activated := false
	var stdout bytes.Buffer

	code := runOrDeclare(&stdout, queryDeclaration(), func() int {
		activated = true
		return 7
	})

	assert.True(t, activated, "a start with no mode never reached the activation function")
	assert.Equal(t, 7, code, "the exit code is the activation function's own")
	assert.Empty(t, stdout.String(), "a live start writes no declaration to stdout")
}
