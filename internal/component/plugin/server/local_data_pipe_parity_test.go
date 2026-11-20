// Design: docs/architecture/api/commands.md -- a local-data command's declared operations on both of its routes.

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
)

// TestLocalDataDeclaredOperationsSameOnBothRoutes asks AC-24's question: does
// a local-data command that declares an operation receive the same declared
// list when `ze cli -c` answers it in the caller's process (ServeLocal) and
// when the daemon answers it for every other surface (Dispatch, through the
// handler loadLocalData registers), and do the two routes answer alike?
//
// Method: the command declares `only`, which its handler applies to its
// peers; the line types it before a generic `| count | json`. Both routes run
// the one handler, which records what it was handed.
//
// MUTATION: drop WithPipeOperations in ServeLocal or in Dispatch; that
// route's list is empty and its count is three, not two.
func TestLocalDataDeclaredOperationsSameOnBothRoutes(t *testing.T) {
	resetPipeOperationRegistries(t)
	const path = "show test local data pipe parity"
	command.RegisterPipeFilters([]string{path},
		command.PipeFilter{Name: "only", Description: "Keep the peers of one state", Argument: command.ArgText})
	replaced := command.ReplaceArgDefSourceForTest(func(string) ([]command.ArgDef, error) { return nil, nil })
	t.Cleanup(func() { command.ReplaceArgDefSourceForTest(replaced) })

	var handed [][]command.PipeOperation
	handler := func(args command.ValidatedArgs) command.LocalAnswer {
		operations := args.PipeOperations()
		handed = append(handed, operations)
		peers := []any{}
		for _, state := range []string{"up", "down", "up"} {
			if !keepsState(operations, state) {
				continue
			}
			peers = append(peers, map[string]any{"state": state})
		}
		return command.LocalAnswer{Payload: map[string]any{"peers": peers}}
	}
	t.Cleanup(command.AddLocalDataForTest(path, handler))
	d := NewDispatcher()
	require.NoError(t, d.RegisterWithOptions(path, localDataHandler(path, handler),
		"Answer the parity peers", RegisterOptions{ReadOnly: true}))

	const line = path + " | only up | count | json"
	local, code, served := command.ServeLocal(line, "")
	require.True(t, served, "the local-data command is served in this process")
	require.Zero(t, code, "ServeLocal answered %q", local)

	resp, err := d.Dispatch(&CommandContext{Username: "alice"}, line)
	require.NoError(t, err)
	require.NotNil(t, resp)

	want := []command.PipeOperation{{Name: "only", Args: []string{"up"}}}
	require.Len(t, handed, 2, "each route ran the handler once")
	assert.Equal(t, want, handed[0], "ServeLocal hands the declared list")
	assert.Equal(t, want, handed[1], "Dispatch hands the same declared list")
	assert.Contains(t, local, `"count": 2`, "the handler applied its own operation before the count")
	assert.JSONEq(t, local, dataOf(t, resp), "the two routes answer alike")
}

// keepsState reports whether a peer in state passes the declared operations:
// `only <state>` keeps the peers in that state, and no operation keeps all.
func keepsState(operations []command.PipeOperation, state string) bool {
	for _, operation := range operations {
		if operation.Name != "only" {
			continue
		}
		if len(operation.Args) != 1 {
			return false
		}
		return operation.Args[0] == state
	}
	return true
}
