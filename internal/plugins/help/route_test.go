package help_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/plugins/help"
)

// answerSilently runs a help handler with os.Stdout and os.Stderr pointed at
// files, and answers the handler's answer and whatever it wrote to either.
func answerSilently(t *testing.T, path string, handler func(command.ValidatedArgs) command.LocalAnswer, args []string) (command.LocalAnswer, string) {
	t.Helper()
	validated, err := command.ValidateModelArgs(path, args, nil)
	require.NoError(t, err, "the model refuses %q %v", path, args)

	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "out"))
	require.NoError(t, err)
	stdout, stderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = out, out
	answer := handler(validated)
	os.Stdout, os.Stderr = stdout, stderr
	require.NoError(t, out.Close())

	written, err := os.ReadFile(filepath.Join(dir, "out"))
	require.NoError(t, err)
	return answer, string(written)
}

// VALIDATES: help command answers its rows as DATA and help ai its Markdown
// page as a command.Blob (or the reference as data under --json); neither
// handler writes, and a bare word keeps its local meaning (A-6): `json` is a
// filter word for help command and names no section for help ai.
// PREVENTS: a handler that prints on the caller's terminal, which on the
// daemon route would print on the daemon's stdout and answer nothing, and a
// model leaf that turns a bare filter word into a rendering option.
func TestHelpHandlersAnswerDataAndWriteNothing(t *testing.T) {
	all, err := help.CollectCommands()
	require.NoError(t, err)

	for _, args := range [][]string{nil, {"json"}, {"peer"}, {help.FlagJSON}} {
		answer, written := answerSilently(t, "help command", help.AnswerHelpCommand, args)
		assert.Empty(t, written, "help command %v wrote on stdout or stderr", args)
		assert.Equal(t, 0, answer.Code, "help command %v", args)
		catalog, isCatalog := answer.Payload.(help.CatalogAnswer)
		require.True(t, isCatalog, "help command %v answered %T", args, answer.Payload)
		want := all
		if filter := help.ExtractCommandFilter(args); filter != "" {
			want = help.FilterCommands(all, filter)
		}
		assert.Equal(t, want, catalog.Commands, "help command %v", args)
	}

	summary, written := answerSilently(t, "help ai", help.AnswerAIHelp, nil)
	assert.Empty(t, written)
	summaryBlob, isBlob := summary.Payload.(command.Blob)
	require.True(t, isBlob, "help ai answered %T", summary.Payload)
	for _, args := range [][]string{{"json"}, {"cli", "api"}} {
		answer, written := answerSilently(t, "help ai", help.AnswerAIHelp, args)
		assert.Empty(t, written, "help ai %v wrote on stdout or stderr", args)
		blob, isBlob := answer.Payload.(command.Blob)
		require.True(t, isBlob, "help ai %v answered %T", args, answer.Payload)
		assert.Equal(t, string(c(renderReference(args))), string(blob), "help ai %v", args)
	}
	jsonWord, _ := answerSilently(t, "help ai", help.AnswerAIHelp, []string{"json"})
	assert.Equal(t, summaryBlob, jsonWord.Payload, "a bare json must print the summary")

	reference, written := answerSilently(t, "help ai", help.AnswerAIHelp, []string{help.FlagJSON})
	assert.Empty(t, written)
	_, isReference := reference.Payload.(help.ReferenceAnswer)
	assert.True(t, isReference, "help ai --json answered %T", reference.Payload)
}

func c(b []byte) []byte { return b }

func renderReference(args []string) []byte {
	var out bytes.Buffer
	help.RenderAIHelp(&out, args)
	return out.Bytes()
}

// VALIDATES: the renderer both commands register prints every answer and
// exits 1 when the write fails.
// PREVENTS: `ze help ai | head` closing the pipe and the command exiting 0 on a
// truncated page, which command.RenderLocalAnswer would do: it drops the
// write error.
func TestHelpRouteExitsNonZeroOnABrokenPipe(t *testing.T) {
	cases := []struct {
		path    string
		handler func(command.ValidatedArgs) command.LocalAnswer
		args    []string
	}{
		{"help command", help.AnswerHelpCommand, nil},
		{"help command", help.AnswerHelpCommand, []string{"--verbose"}},
		{"help command", help.AnswerHelpCommand, []string{help.FlagJSON}},
		{"help ai", help.AnswerAIHelp, nil},
		{"help ai", help.AnswerAIHelp, []string{help.FlagJSON}},
	}
	for _, tc := range cases {
		answer, _ := answerSilently(t, tc.path, tc.handler, tc.args)
		var out bytes.Buffer
		assert.Equal(t, 0, help.RenderTo(&out, io.Discard, tc.path, answer.Payload), "%s %v: healthy writer", tc.path, tc.args)
		assert.NotEmpty(t, out.Bytes(), "%s %v", tc.path, tc.args)
		assert.Equal(t, 1, help.RenderTo(brokenPipeWriter{}, io.Discard, tc.path, answer.Payload), "%s %v: broken pipe -> exit 1", tc.path, tc.args)
	}
}

// VALIDATES: the daemon's text route draws the catalog table the local route
// prints, from the answer's JSON envelope.
// PREVENTS: `ze cli -c "help command"` printing a generic table of nested
// fields where `ze help command` prints the catalog.
func TestCatalogDrawsTheLocalTableOnTheTextRoute(t *testing.T) {
	answer, _ := answerSilently(t, "help command", help.AnswerHelpCommand, []string{"peer"})
	encoded, err := json.Marshal(answer.Payload)
	require.NoError(t, err)
	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &envelope))
	require.Len(t, envelope, 1, "the answer is one envelope key: %s", encoded)

	drawn, err := help.DrawCatalog(envelope["command-catalog"])
	require.NoError(t, err)
	var local bytes.Buffer
	help.RenderHelpCommand(&local, []string{"peer"})
	assert.Equal(t, local.String(), drawn)
}
