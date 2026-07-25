// Design: docs/features/ai-first.md — skills command tests

package skills

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

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

func TestSkillsListAll(t *testing.T) {
	out := captureStdout(t, func() {
		code := Run([]string{"list"})
		assert.Equal(t, 0, code)
	})
	assert.Contains(t, out, "ze")
	assert.Contains(t, out, "ze-diagnostics")
	assert.Contains(t, out, "ze-config")
	assert.Contains(t, out, "ze-commands")
	assert.Contains(t, out, "ze-agent")
}

func TestSkillsListJSON(t *testing.T) {
	out := captureStdout(t, func() {
		code := Run([]string{"list", "--json"})
		assert.Equal(t, 0, code)
	})
	var entries []diagnostic.SkillEntry
	require.NoError(t, json.Unmarshal([]byte(out), &entries))
	assert.Len(t, entries, len(inventory))
	assert.Equal(t, "ze", entries[0].Name)
}

func TestSkillsGetCompact(t *testing.T) {
	out := captureStdout(t, func() {
		code := Run([]string{"get", "ze"})
		assert.Equal(t, 0, code)
	})
	assert.Contains(t, out, "name: ze")
	assert.Contains(t, out, "Agent Entry Points")
}

func TestSkillsGetFull(t *testing.T) {
	compact := captureStdout(t, func() {
		Run([]string{"get", "ze"})
	})
	full := captureStdout(t, func() {
		code := Run([]string{"get", "ze", "--full"})
		assert.Equal(t, 0, code)
	})
	assert.Contains(t, full, "Diagnostic Loop")
	assert.True(t, len(full) > len(compact), "full should be longer than compact")
}

func TestSkillsGetInnerSkill(t *testing.T) {
	out := captureStdout(t, func() {
		code := Run([]string{"get", "ze-diagnostics"})
		assert.Equal(t, 0, code)
	})
	assert.Contains(t, out, "ze-diagnostics")
	assert.Contains(t, out, "Diagnostic Shape")
}

func TestSkillsGetJSON(t *testing.T) {
	out := captureStdout(t, func() {
		code := Run([]string{"get", "ze", "--json"})
		assert.Equal(t, 0, code)
	})
	var result struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Content     string `json:"content"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "ze", result.Name)
	assert.NotEmpty(t, result.Content)
}

func TestSkillsGetUnknown(t *testing.T) {
	code := Run([]string{"get", "nonexistent"})
	assert.Equal(t, 1, code)
}

func TestSkillsNoArgs(t *testing.T) {
	code := Run([]string{})
	assert.Equal(t, 1, code)
}
