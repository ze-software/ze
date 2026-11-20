package server

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPluginPipeDeclWithArgumentIsAnOperation proves a Stage 1 Pipes entry
// stating an argument is recorded into the pipe filter registry under the
// plugin, with its kind and its own description, while an entry stating an
// expansion beside it stays an alias exactly as before.
//
// VALIDATES: spec-pipe-chain-honored-on-every-command-surface AC-14.
// PREVENTS: an operation recorded as an alias, or an alias read as an
// operation, because one Pipes list carries both kinds.
func TestPluginPipeDeclWithArgumentIsAnOperation(t *testing.T) {
	const owner = "pipe-operation-recorded"
	const path = "show pipe operation recorded"
	t.Cleanup(func() {
		command.UnregisterPluginPipeFilters(owner)
		command.UnregisterPluginAliases(owner)
	})

	commands := []rpc.CommandDecl{{Name: path}}
	pipes := []rpc.PipeDecl{
		{Command: path, Name: "top-talkers", Description: "Rank the rows by bytes.", Argument: "count"},
		{Command: path, Name: "totals", Description: "The counters alone", Expansion: "display kind"},
	}
	require.NoError(t, validatePipeDecls(pipes, commands))
	require.NoError(t, registerPluginPipes(owner, pipes, commands))

	assert.Equal(t,
		[]command.PipeFilter{{Name: "top-talkers", Description: "Rank the rows by bytes.", Argument: command.ArgCount}},
		command.PipeFiltersForCommand(path), "the argument entry is an operation of the command")
	aliases := command.AliasesForCommand(path)
	require.Len(t, aliases, 1, "the expansion entry is an alias, and the operation is not one")
	assert.Equal(t, "totals", aliases[0].Name)
}

// TestPluginPipeDeclBothOrNeitherRefused proves an entry stating both an
// expansion and an argument, one stating neither, and one stating an argument
// outside the vocabulary are refused at Stage 1, by name.
//
// VALIDATES: AC-14, the refusal half.
// PREVENTS: an entry that states nothing read as one kind by default, the
// guard "expands to nothing" lost with the new field.
func TestPluginPipeDeclBothOrNeitherRefused(t *testing.T) {
	commands := []rpc.CommandDecl{{Name: "show x"}}
	rows := []struct {
		name string
		decl rpc.PipeDecl
		want string
	}{
		{"both", rpc.PipeDecl{Command: "show x", Name: "two-kinds", Expansion: "table", Argument: "none"}, "both an expansion and an argument"},
		{"neither", rpc.PipeDecl{Command: "show x", Name: "no-kind"}, "expands to nothing"},
		{"unknown argument", rpc.PipeDecl{Command: "show x", Name: "odd-kind", Argument: "number"}, `argument "number"`},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			err := validatePipeDecls([]rpc.PipeDecl{row.decl}, commands)
			require.Error(t, err)
			assert.Contains(t, err.Error(), row.decl.Name)
			assert.Contains(t, err.Error(), row.want)
		})
	}
	require.NoError(t, validatePipeDecls([]rpc.PipeDecl{{Command: "show x", Name: "one-kind", Argument: "none"}}, commands),
		"an entry stating an argument alone is an operation")
}

// TestPluginPipeDeclRefusedAliasWithdrawsOperations proves a Pipes list whose
// alias the alias registry refuses leaves none of the list's operations
// behind: the alias here takes the name of the operation beside it.
//
// VALIDATES: AC-30, "nothing of the batch is recorded", across the two kinds.
// PREVENTS: a refused Stage 1 that still answers its operations.
func TestPluginPipeDeclRefusedAliasWithdrawsOperations(t *testing.T) {
	const owner = "pipe-operation-alias-clash"
	const path = "show pipe operation clash"
	t.Cleanup(func() {
		command.UnregisterPluginPipeFilters(owner)
		command.UnregisterPluginAliases(owner)
	})

	commands := []rpc.CommandDecl{{Name: path}}
	pipes := []rpc.PipeDecl{
		{Command: path, Name: "top", Argument: "none"},
		{Command: path, Name: "top", Expansion: "table"},
	}
	err := registerPluginPipes(owner, pipes, commands)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "top")
	assert.Nil(t, command.PipeFiltersForCommand(path), "the operation left with the refused alias")
	assert.Empty(t, command.AliasesForCommand(path))
}

// TestPluginOperationWithdrawnOnStop proves a plugin's declared operations
// leave the registry on the paths a plugin leaves by: a Stage 1 that fails
// after the pipe write (the rollback arms of onRegistration), and
// releasePluginRegistrations, which a failed later stage, a removal from the
// config (startup_removal.go) and a restart (reload_tx.go) all run.
//
// VALIDATES: A-6, AC-14 (withdrawn on a failed startup, a removal and a
// restart).
// PREVENTS: an operation outliving its plugin, which would refuse that plugin
// its own declaration when it starts again ("already holds the pipe
// operations").
func TestPluginOperationWithdrawnOnStop(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	t.Run("a Stage 1 failing after the pipe write", func(t *testing.T) {
		const pluginName = "pipe-operation-rollback"
		const path = "show pipe operation rollback"
		t.Cleanup(func() { command.UnregisterPluginPipeFilters(pluginName) })

		s, _ := newLifecycleStartupServer(t)
		proc := process.NewProcess(plugin.PluginConfig{Name: pluginName, Internal: true})
		sink := &engineStartupSink{s: s, proc: proc}
		err := sink.onRegistration(&rpc.DeclareRegistrationInput{
			ProtocolVersion: rpc.ProtocolVersion,
			Commands:        []rpc.CommandDecl{{Name: path}},
			Pipes:           []rpc.PipeDecl{{Command: path, Name: "top", Argument: "none"}},
			// The conflict: the AFI number is IPv4's and the name is not.
			Families: []rpc.FamilyDecl{
				{Name: "not-ipv4/pipe-operations", Mode: "both", AFI: uint16(family.AFIIPv4), SAFI: 204},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "family")
		assert.Nil(t, command.PipeFiltersForCommand(path), "a Stage 1 that failed left its operation behind")
	})

	t.Run("released when the plugin leaves", func(t *testing.T) {
		const pluginName = "pipe-operation-release"
		const path = "show pipe operation release"
		t.Cleanup(func() { command.UnregisterPluginPipeFilters(pluginName) })

		s, _ := newLifecycleStartupServer(t)
		proc := process.NewProcess(plugin.PluginConfig{Name: pluginName, Internal: true})
		commands := []rpc.CommandDecl{{Name: path}}
		pipes := []rpc.PipeDecl{{Command: path, Name: "top", Argument: "none"}}
		require.NoError(t, registerPluginPipes(pluginName, pipes, commands))
		require.Len(t, command.PipeFiltersForCommand(path), 1)

		s.releasePluginRegistrations(proc)
		assert.Nil(t, command.PipeFiltersForCommand(path), "the operation left with its plugin")
		require.NoError(t, registerPluginPipes(pluginName, pipes, commands),
			"the same plugin declares its operation again when it starts again")
	})
}

// TestRouteToProcessCarriesPipe proves the request routeToProcess sends holds
// the line's declared operations in its pipe list, with their argument
// words, and keeps them out of the arguments, while the generic operation
// after them runs over the plugin's answer.
//
// Method: a plugin command whose path holds a plugin-declared operation `top`
// (argument count); Dispatch `request target pipe | top 3 | count`; the fake
// plugin captures the decoded request and answers two rows.
//
// VALIDATES: AC-14 (its command receives the operation through
// execute-command), AC-2 carrier half.
// PREVENTS: the 3b refusal standing ("cannot be sent to"), the list dropped,
// or its words folded into args.
func TestRouteToProcessCarriesPipe(t *testing.T) {
	const owner = "route-to-process-pipe"
	const path = "request target pipe"
	require.NoError(t, command.RegisterPluginPipeFilters(owner, []string{path},
		[]command.PluginPipeFilter{{Command: path, Filter: command.PipeFilter{Name: "top", Argument: command.ArgCount}}}))
	t.Cleanup(func() { command.UnregisterPluginPipeFilters(owner) })

	d := NewDispatcher()
	var requested rpc.ExecuteCommandInput
	done := registerExecuteCommandTarget(t, d, path, 1, func(_ int, input *rpc.ExecuteCommandInput) (*rpc.ExecuteCommandOutput, error) {
		requested = *input
		return &rpc.ExecuteCommandOutput{Status: plugin.StatusDone, Data: json.RawMessage(`{"peers":[{"name":"a"},{"name":"b"}]}`)}, nil
	})

	resp, err := d.Dispatch(&CommandContext{Username: "alice"}, path+" | top 3 | count")
	require.NoError(t, err)
	require.NoError(t, <-done)
	assert.Equal(t, path, requested.Command)
	assert.Empty(t, requested.Args, "no operation word reaches the arguments")
	assert.Equal(t, []rpc.PipeOperation{{Name: "top", Args: []string{"3"}}}, requested.Pipe)
	assert.Equal(t, float64(2), chainCountOf(t, resp), "the generic count runs over the plugin's answer")
}
