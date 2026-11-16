// Design: docs/architecture/cli/command-completion.md -- registered operational input.

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
)

// TestRecognizesCommandUsesDispatchRegistrations asks the actual dispatch
// matcher without authorization, execution, or accounting side effects.
func TestRecognizesCommandUsesDispatchRegistrations(t *testing.T) {
	dispatcher := NewDispatcher()
	runs := 0
	require.NoError(t, dispatcher.Register("delete agent skill", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
		runs++
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{}), nil
	}, "Remove the agent skill"))
	for _, line := range []string{"delete agent skill", "delete agent skill agent all", "DELETE agent skill agent all", `delete agent skill agent "literal|value"`, "delete agent skill | json"} {
		assert.True(t, dispatcher.RecognizesCommand(line), line)
	}
	for _, line := range []string{"", "delete bgp", "delete agent skillful", "delete agent", `delete agent skill "unfinished`} {
		assert.False(t, dispatcher.RecognizesCommand(line), line)
	}
	assert.Zero(t, runs)
}

// TestRecognizesCommandTracksHiddenPluginsAndSubsystems checks recognition
// follows dispatch registrations, including hidden commands and retirement.
func TestRecognizesCommandTracksHiddenPluginsAndSubsystems(t *testing.T) {
	dispatcher := NewDispatcher()
	proc := process.NewProcess(plugin.PluginConfig{Name: "recognition-probe"})
	const path = "delete recognition item"
	results := dispatcher.Registry().Register(proc, []CommandDef{{Name: path, Hidden: true}})
	require.Len(t, results, 1)
	require.True(t, results[0].OK, results[0].Error)
	dispatcher.Registry().Freeze()
	assert.True(t, dispatcher.RecognizesCommand(path+" name alpha"))
	assert.False(t, dispatcher.RecognizesCommand("delete recognition itemized"))
	dispatcher.Registry().Unregister(proc, []string{path})
	assert.False(t, dispatcher.RecognizesCommand(path+" name alpha"))
	dispatcher.Subsystems().Register(SubsystemConfig{Name: "recognition-probe", Commands: []string{"request recognition task"}})
	assert.True(t, dispatcher.RecognizesCommand("request recognition task name alpha"))
	assert.False(t, dispatcher.RecognizesCommand("request recognition taskful"))
}
