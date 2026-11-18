// Design: docs/architecture/api/commands.md -- schema visibility across command lists.
package server

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestHiddenSchemaCommandsKeepDispatchButHideDiscovery exercises the alias
// loader, actual command-list row encoder and live plugin registry together.
func TestHiddenSchemaCommandsKeepDispatchButHideDiscovery(t *testing.T) {
	loader := yang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.AddModuleFromText("hidden-projection-cmd.yang", `
module hidden-projection-cmd {
 namespace "urn:test:hidden-projection"; prefix hp;
 import ze-extensions { prefix ze; }
 container show { config false;
  container visibility { config false;
   container public { config false; ze:command "fixture:visible";
    ze:help "Public answer"; description "The public answer.";
   }
   container secret { config false; ze:hidden true; ze:command "fixture:hidden";
    ze:help "Hidden answer"; description "The hidden answer.";
   }
   container plugin { config false; ze:hidden true;
    ze:help "Hidden plugin"; description "The hidden plugin.";
   }
  }
 }
}`))
	schema, err := loader.Resolve()
	require.NoError(t, err)
	dispatcher := NewDispatcher()
	calls := 0
	handler := func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		calls++
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"answer": "present"}}, nil
	}
	registrations := []RPCRegistration{
		{WireMethod: "fixture:visible", Handler: handler},
		{WireMethod: "fixture:hidden", Handler: handler},
	}
	require.NoError(t, loadBuiltinsWithAliases(dispatcher, registrations, yang.WireMethodToPaths(schema),
		yang.PathToDescription(schema), yang.PathToHelp(schema), yang.PathToArgDefs(schema), yang.BuildCommandTree(schema)))
	proc := process.NewProcess(plugin.PluginConfig{Name: "hidden-projection"})
	results := dispatcher.Registry().Register(proc, []CommandDef{
		{Name: "show visibility plugin", ShortHelp: "Hidden by schema"},
		{Name: "show visibility dynamic", ShortHelp: "Visible dynamic command"},
		{Name: "show visibility sdkhidden", ShortHelp: "Hidden by SDK", Hidden: true},
	})
	for _, result := range results {
		require.True(t, result.OK, result.Error)
	}
	t.Run("rows", func(t *testing.T) {
		rows := make(map[string]Completion)
		for record := range rpc.HeldRecords(commandRows(dispatcher, false)) {
			require.Nil(t, record.Fault)
			var row Completion
			require.NoError(t, json.Unmarshal(record.Item, &row))
			rows[row.Value] = row
		}
		require.True(t, rows["show visibility secret"].Hidden)
		require.True(t, rows["show visibility plugin"].Hidden)
		require.True(t, rows["show visibility sdkhidden"].Hidden)
		require.False(t, rows["show visibility public"].Hidden)
		require.False(t, rows["show visibility dynamic"].Hidden)
	})
	t.Run("plugin-discovery", func(t *testing.T) {
		entries := dispatcher.Registry().VisibleCommandEntries()
		require.Equal(t, []command.CommandEntry{{Name: "show visibility dynamic", ShortHelp: "Visible dynamic command"}}, entries)
		require.NotNil(t, dispatcher.Registry().Lookup("show visibility plugin"))
	})
	t.Run("typed-dispatch", func(t *testing.T) {
		response, err := dispatcher.Dispatch(&CommandContext{Sender: plugin.OperatorSender()}, "show visibility secret")
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, response.Status)
		require.Equal(t, 1, calls)
	})
}
