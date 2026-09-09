package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// VALIDATES: Plugin subcommand dispatch (Run), help flags, unknown plugin handling.
// PREVENTS: Broken dispatch routing, panic on empty args.

// TestRun_NoArgs verifies exit 1 when no arguments provided.
func TestRun_NoArgs(t *testing.T) {
	code := Run(nil)
	assert.Equal(t, 1, code)
}

// TestRun_EmptyArgs verifies exit 1 for empty slice.
func TestRun_EmptyArgs(t *testing.T) {
	code := Run([]string{})
	assert.Equal(t, 1, code)
}

// TestRun_HelpFlag verifies help returns exit 0.
func TestRun_HelpFlag(t *testing.T) {
	tests := []struct {
		name string
		arg  string
	}{
		{"help", "help"},
		{"dash_h", "-h"},
		{"double_dash_help", "--help"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := Run([]string{tt.arg})
			assert.Equal(t, 0, code)
		})
	}
}

// TestRun_UnknownPlugin verifies exit 1 for unregistered plugin name.
func TestRun_UnknownPlugin(t *testing.T) {
	code := Run([]string{"nonexistent-plugin-xyz"})
	assert.Equal(t, 1, code)
}

// TestMapKeys verifies key extraction from map.
func TestMapKeys(t *testing.T) {
	m := map[string]any{
		"alpha": 1,
		"beta":  2,
	}
	keys := mapKeys(m)
	assert.Len(t, keys, 2)
	assert.Contains(t, keys, "alpha")
	assert.Contains(t, keys, "beta")
}

// TestMapKeys_Empty verifies empty map returns empty slice.
func TestMapKeys_Empty(t *testing.T) {
	keys := mapKeys(map[string]any{})
	assert.Empty(t, keys)
}

// captureQuery runs the plugin subcommand with stdout and stderr captured, so
// a test can read the framed answer query mode writes to stdout.
func captureQuery(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()

	return captureStdio(t, func() int { return Run(args) })
}

// captureStdio runs one entry point with stdout and stderr redirected to pipes,
// and returns its exit code beside what each stream received. Query mode writes
// its answer to os.Stdout, so a test reads the line by taking that file over
// for the duration of the call.
func captureStdio(t *testing.T, run func() int) (code int, stdout, stderr string) {
	t.Helper()

	origOut, origErr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	require.NoError(t, err)
	rErr, wErr, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout, os.Stderr = wOut, wErr

	code = run()

	os.Stdout, os.Stderr = origOut, origErr
	require.NoError(t, wOut.Close())
	require.NoError(t, wErr.Close())

	var bufOut, bufErr bytes.Buffer
	_, err = io.Copy(&bufOut, rOut)
	require.NoError(t, err)
	_, err = io.Copy(&bufErr, rErr)
	require.NoError(t, err)
	require.NoError(t, rOut.Close())
	require.NoError(t, rErr.Close())

	return code, bufOut.String(), bufErr.String()
}

// registerQueryFake registers a plugin whose CLI handler records that it ran,
// and returns the flag that records it. The registry is restored afterwards.
func registerQueryFake(t *testing.T, name string, commands []rpc.CommandDecl, pipes []rpc.PipeDecl) *bool {
	t.Helper()

	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })

	handlerRan := false
	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "fake plugin for query mode tests",
		Commands:    commands,
		Pipes:       pipes,
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler: func([]string) int {
			handlerRan = true
			return 0
		},
	}))
	return &handlerRan
}

// requestQueryMode puts the process in query mode for the duration of the test.
func requestQueryMode(t *testing.T) {
	t.Helper()

	require.NoError(t, env.Set(sdk.EnvPluginMode, sdk.ModeDeclare))
	t.Cleanup(func() { _ = env.Set(sdk.EnvPluginMode, "") })
}

// parseDeclaration reads the one framed line query mode writes, through the
// protocol's own parser, and returns the Stage 1 declaration it carries.
func parseDeclaration(t *testing.T, stdout string) rpc.DeclareRegistrationInput {
	t.Helper()

	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	require.Len(t, lines, 1, "query mode writes one line: %q", stdout)

	id, method, payload, err := rpc.ParseLine([]byte(lines[0]))
	require.NoError(t, err)
	assert.Equal(t, uint64(1), id)
	assert.Equal(t, rpc.MethodDeclareRegistration, method)

	var decl rpc.DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(payload, &decl))
	return decl
}

// TestQueryModeAnswersBeforeTheHandler verifies AC-1 and AC-3: a plugin started
// in query mode writes its Stage 1 declaration to stdout and its own CLI
// handler never runs, because cli.Run answers from the looked-up registration.
//
// VALIDATES: AC-1, AC-3.
// PREVENTS: A query reaching plugin code, which is where every measured start
// side effect lives.
func TestQueryModeAnswersBeforeTheHandler(t *testing.T) {
	handlerRan := registerQueryFake(t, "query-fake",
		[]rpc.CommandDecl{{Name: "show fake state", Description: "fake command"}},
		[]rpc.PipeDecl{{Command: "show fake state", Name: "brief", Expansion: "fields name"}},
	)
	requestQueryMode(t)

	code, stdout, stderr := captureQuery(t, "query-fake")

	assert.Equal(t, 0, code, "stderr: %s", stderr)
	assert.False(t, *handlerRan, "the plugin's CLI handler ran under query mode")

	decl := parseDeclaration(t, stdout)
	require.Len(t, decl.Commands, 1)
	assert.Equal(t, "show fake state", decl.Commands[0].Name)
	require.Len(t, decl.Pipes, 1)
	assert.Equal(t, "brief", decl.Pipes[0].Name)
}

// TestQueryModeEmptyDeclarationIsNotSilence verifies AC-2: a plugin that
// declares no command and no pipe still writes the framed line, so a reader can
// tell "declared nothing" from "sent nothing".
//
// VALIDATES: AC-2.
// PREVENTS: An empty declaration reading as a plugin that does not answer.
func TestQueryModeEmptyDeclarationIsNotSilence(t *testing.T) {
	handlerRan := registerQueryFake(t, "query-fake-silent", nil, nil)
	requestQueryMode(t)

	code, stdout, stderr := captureQuery(t, "query-fake-silent")

	assert.Equal(t, 0, code, "stderr: %s", stderr)
	assert.False(t, *handlerRan, "the plugin's CLI handler ran under query mode")

	decl := parseDeclaration(t, stdout)
	assert.Empty(t, decl.Commands)
	assert.Empty(t, decl.Pipes)
}

// TestQueryAnswerIsTheSameLineFromBothWriters verifies that the ze binary and
// the SDK write one line for one declaration, byte for byte.
//
// VALIDATES: AC-1 and AC-7 -- the query answer is the Stage 1 message on a
// second carrier, whichever binary writes it.
// PREVENTS: the request id, the method or the framing drifting between the two
// routes. One reader parses both lines, so a drift the compiler cannot see
// makes half the plugin population unreadable.
//
// The goal is that the answer has one definition. The method hands the same
// commands and pipes to `ze plugin <name>` and to sdk.RunOrDeclare, captures
// what each writes to stdout, and compares the bytes.
func TestQueryAnswerIsTheSameLineFromBothWriters(t *testing.T) {
	commands := []rpc.CommandDecl{{Name: "show fake state", Description: "fake command"}}
	pipes := []rpc.PipeDecl{{Command: "show fake state", Name: "brief", Expansion: "fields name"}}
	registerQueryFake(t, "query-fake-shared", commands, pipes)
	requestQueryMode(t)

	zeCode, zeLine, zeStderr := captureQuery(t, "query-fake-shared")
	require.Equal(t, 0, zeCode, "stderr: %s", zeStderr)

	sdkCode, sdkLine, sdkStderr := captureStdio(t, func() int {
		return sdk.RunOrDeclare(sdk.Registration{Commands: commands, Pipes: pipes}, func() int { return 1 })
	})
	require.Equal(t, 0, sdkCode, "stderr: %s", sdkStderr)

	assert.Equal(t, zeLine, sdkLine,
		"the ze binary and the SDK answer one declaration with two different lines")
}
