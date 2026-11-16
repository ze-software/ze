// Design: docs/architecture/cli/command-completion.md -- attached mode recognition.

package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	unicli "github.com/ze-software/ze/internal/component/cli"
)

// TestAttachedModelUsesDaemonCommandRecognition proves attached construction
// carries the registry matcher into Enter rather than entering the editor.
func TestAttachedModelUsesDaemonCommandRecognition(t *testing.T) {
	const line = "delete agent skill agent all"
	called := false
	m, err := newAttachedModel(attachedTestDispatch, unicli.CommandExecutor(attachedTestDispatch), attachedTestEditor(t), func(input string) bool {
		called = true
		return input == line
	})
	require.NoError(t, err)
	updated := pressEnter(t, &m, line)
	t.Cleanup(func() { require.NoError(t, updated.Close()) })
	assert.True(t, called)
	assert.Equal(t, unicli.ModeOperational, updated.Mode())
}
