// Design: docs/features/ai-first.md — explain command tests

package explain

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func TestMain(m *testing.M) {
	diagnostic.RegisterBuiltinCodes()
	os.Exit(m.Run())
}

// captureOutput runs fn with os.Stdout and os.Stderr redirected to one pipe
// and answers everything written to either.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = w, w
	fn()
	os.Stdout, os.Stderr = oldOut, oldErr
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(data)
}

// runQuiet runs the handler and fails the test when it writes anything: the
// answer, and any refusal, is a value the route prints.
func runQuiet(t *testing.T, args ...string) command.LocalAnswer {
	t.Helper()
	var result command.LocalAnswer
	written := captureOutput(t, func() { result = Run(commandtest.Args(args...)) })
	assert.Empty(t, written, "handler wrote to stdout or stderr")
	return result
}

// TestExplainKnownDiagnostic checks the answer is the explanation as data, and
// that the local route draws it as the text the command printed before.
// Method: the expected text is built here from the catalog entry in the old
// layout, then compared byte for byte with what render prints.
func TestExplainKnownDiagnostic(t *testing.T) {
	result := runQuiet(t, "config-parse")
	require.Equal(t, 0, result.Code)
	explained, ok := result.Payload.(answer)
	require.True(t, ok, "payload %T, want answer", result.Payload)
	assert.Equal(t, "config-parse", explained.Diagnostic.Code)
	assert.Equal(t, "Config syntax error", explained.Diagnostic.Title)

	meta := diagnostic.Lookup("config-parse")
	require.NotNil(t, meta)
	var want textbuf.Buffer
	want.Str(meta.Code).Str(": ").Str(meta.Title).Str("\n\n").Str(meta.Description).Byte('\n')
	if len(meta.Examples) > 0 {
		want.Str("\nExamples:\n")
		for _, ex := range meta.Examples {
			want.Str("  ").Str(ex).Byte('\n')
		}
	}
	if len(meta.RelatedCodes) > 0 {
		want.Str("\nRelated: ").Join(meta.RelatedCodes, ", ").Byte('\n')
	}

	var code int
	out := captureOutput(t, func() { code = render("show diagnostic", result.Payload) })
	assert.Equal(t, 0, code)
	assert.Equal(t, want.String(), out)
}

// TestExplainKnownDiagnosticJSON checks `--json` stays a local rendering flag:
// the local route prints the indented explanation, and the data carries no
// trace of the flag.
func TestExplainKnownDiagnosticJSON(t *testing.T) {
	result := runQuiet(t, "--json", "config-parse")
	require.Equal(t, 0, result.Code)

	out := captureOutput(t, func() { assert.Equal(t, 0, render("show diagnostic", result.Payload)) })
	var printed diagnostic.ExplainResult
	require.NoError(t, json.Unmarshal([]byte(out), &printed))
	assert.Equal(t, "config-parse", printed.Code)
	assert.Equal(t, "Config syntax error", printed.Title)
	assert.NotEmpty(t, printed.Description)

	encoded, err := json.Marshal(result.Payload)
	require.NoError(t, err)
	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &envelope))
	assert.Len(t, envelope, 1)
	assert.Contains(t, envelope, keyDiagnostic)
}

// TestExplainUnknownDiagnostic checks an unknown code is refused with exit
// code 1, as a returned diagnostic naming the codes that exist.
func TestExplainUnknownDiagnostic(t *testing.T) {
	unknown := runQuiet(t, "nonexistent-code")
	assert.Equal(t, 1, unknown.Code)
	assert.Nil(t, unknown.Payload)
	assert.Contains(t, unknown.Diagnostic, "error: unknown diagnostic code: nonexistent-code\n")
	assert.Contains(t, unknown.Diagnostic, "valid codes: ")
}

// TestExplainTwoCodes checks a second code is refused rather than ignored.
func TestExplainTwoCodes(t *testing.T) {
	two := runQuiet(t, "config-parse", "config-yang-missing")
	assert.Equal(t, 1, two.Code)
	assert.Nil(t, two.Payload)
	assert.Equal(t, "error: expected one diagnostic code, got 2\n", two.Diagnostic)
}

// TestExplainNoArgs checks no code is refused with exit code 1 and the usage
// page as the returned diagnostic.
func TestExplainNoArgs(t *testing.T) {
	none := runQuiet(t)
	assert.Equal(t, 1, none.Code)
	assert.Nil(t, none.Payload)
	assert.Contains(t, none.Diagnostic, "ze show diagnostic [--json] <diagnostic-code>")
}

// TestExplainHelp checks --help answers the usage page with exit code 0.
func TestExplainHelp(t *testing.T) {
	help := runQuiet(t, "--help")
	assert.Equal(t, 0, help.Code)
	assert.Nil(t, help.Payload)
	assert.Contains(t, help.Diagnostic, "ze show diagnostic [--json] <diagnostic-code>")
}
