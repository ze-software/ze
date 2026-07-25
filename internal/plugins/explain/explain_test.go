// Design: docs/features/ai-first.md — explain command tests

package explain

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

func TestMain(m *testing.M) {
	diagnostic.RegisterBuiltinCodes()
	os.Exit(m.Run())
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	old := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = old
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(data)
}

func TestExplainKnownDiagnostic(t *testing.T) {
	out := captureStdout(t, func() {
		code := Run([]string{"config-parse"})
		assert.Equal(t, 0, code)
	})
	assert.Contains(t, out, "config-parse")
	assert.Contains(t, out, "Config syntax error")
}

func TestExplainKnownDiagnosticJSON(t *testing.T) {
	out := captureStdout(t, func() {
		code := Run([]string{"--json", "config-parse"})
		assert.Equal(t, 0, code)
	})

	var result diagnostic.ExplainResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "config-parse", result.Code)
	assert.Equal(t, "Config syntax error", result.Title)
	assert.NotEmpty(t, result.Description)
}

func TestExplainUnknownDiagnostic(t *testing.T) {
	code := Run([]string{"nonexistent-code"})
	assert.Equal(t, 1, code)
}

func TestExplainNoArgs(t *testing.T) {
	code := Run([]string{})
	assert.Equal(t, 1, code)
}

func TestExplainHelp(t *testing.T) {
	code := Run([]string{"--help"})
	assert.Equal(t, 0, code)
}
