// Design: docs/architecture/cli/command-completion.md -- shared mode selection.

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegisteredOperationalInputPrecedesConfigConvenience checks the one mode
// predicate consumed by the model and browser, retaining unregistered edits.
func TestRegisteredOperationalInputPrecedesConfigConvenience(t *testing.T) {
	registered := func(input string) bool { return input == "delete agent skill agent all" }
	assert.False(t, ShouldEnterConfigMode("delete agent skill agent all", registered))
	assert.True(t, ShouldEnterConfigMode("delete bgp peer old", registered))
	assert.True(t, ShouldEnterConfigMode("set bgp router-id 192.0.2.44", registered))
	assert.False(t, ShouldEnterConfigMode("show bgp", registered))
	assert.False(t, ShouldEnterConfigMode("unknown input", registered))
	assert.True(t, ShouldEnterConfigMode("delete bgp peer old", nil))
}

// TestModelRegisteredOperationalDeleteStaysOperational drives Enter through the
// real model, proving the shared predicate is wired beyond its standalone test.
func TestModelRegisteredOperationalDeleteStaysOperational(t *testing.T) {
	m := newTestModel(t)
	t.Cleanup(func() { require.NoError(t, m.Close()) })
	const line = "delete agent skill agent all"
	m.SetRegisteredOperational(func(input string) bool { return input == line })
	seen := ""
	m.SetCommandExecutor(func(input string) (CommandOutput, error) {
		seen = input
		return CommandOutput{Text: "daemon answer"}, nil
	})
	m.SetMode(ModeOperational)
	m.SetInput(line)
	next, execute := m.handleEnter()
	updated, ok := next.(Model)
	require.True(t, ok)
	assert.Equal(t, ModeOperational, updated.Mode())
	require.NotNil(t, execute)
	answer, ok := execute().(commandResultMsg)
	require.True(t, ok)
	require.NoError(t, answer.err)
	assert.Equal(t, line, seen)
	assert.Contains(t, answer.result.output, "daemon answer")
	assert.False(t, updated.hasPendingChanges())
}
