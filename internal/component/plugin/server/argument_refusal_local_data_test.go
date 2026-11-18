// Design: docs/architecture/api/commands.md -- local-data commands keep their local refusal text.
// Related: command.go -- Dispatch's registration-scoped argument hint.

package server

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestDispatchArgumentHelpUsesTheExactLocalDataRegistration proves a local
// route's hint follows only the exact registered command. Method: refuse the
// same missing leaf on a local-data path and a separate builtin child whose
// longest local-data lookup finds that parent with words left over.
func TestDispatchArgumentHelpUsesTheExactLocalDataRegistration(t *testing.T) {
	const path = "show test local argument refusal"
	t.Cleanup(command.AddLocalDataForTest(path, func(command.ValidatedArgs) command.LocalAnswer {
		t.Fatal("a refused command must not reach its local-data handler")
		return command.LocalAnswer{}
	}))
	defs := []command.ArgDef{
		commandtest.Must(command.NewStringArg("content", nil, nil, command.ArgOptions{Mandatory: true})),
	}
	d := NewDispatcher()
	for _, name := range []string{path, path + " detail"} {
		require.NoError(t, d.RegisterWithOptions(name, func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
			t.Fatal("a refused command must not reach its daemon handler")
			return nil, assert.AnError
		}, "Inspect the registration boundary", RegisterOptions{ReadOnly: true, ArgDefs: defs}))
	}
	for _, tc := range []struct {
		name string
		want string
	}{
		{path, "required argument missing: content\n" +
			"The command definition makes content mandatory and supplies no further explanation\n" +
			"fix: " + path + " content <content>\nhint: run '" + path + " help' to see the arguments it takes"},
		{path + " detail", "required argument missing: content\n" +
			"The command definition makes content mandatory and supplies no further explanation\n" +
			"fix: " + path + " detail content <content>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := d.Dispatch(&CommandContext{}, tc.name)
			require.Error(t, err)
			require.NotNil(t, response)
			assert.Equal(t, tc.want, err.Error())
			assert.Equal(t, tc.want, response.Error)
			assert.Equal(t, plugin.StatusError, response.Status)
			missing, found := errors.AsType[*command.MissingArgumentError](err)
			require.True(t, found, "both routes retain the typed validation cause")
			assert.Equal(t, "content", missing.Leaf)
		})
	}
}
