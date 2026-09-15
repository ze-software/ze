package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestCommandHelpAndCompleteDeclareTheirArgument verifies that the four
// command-introspection builtins declare the value an operator types after
// them on their `-cmd` container, which is the one declaration the command
// tree, completion, `ze help ai --json` and the web admin form read
// (extractArgDefs, internal/component/config/yang/command.go).
//
// PREVENTS: the leaf living on the rpc input of the `-api` module alone, so
// the web admin page renders no input, its Execute posts the bare command, and
// the daemon refuses it with the usage line
// (plan/journal/command-takes-an-untyped-positional-value.md, 2026-09-15).
func TestCommandHelpAndCompleteDeclareTheirArgument(t *testing.T) {
	loader, err := yang.DefaultLoader()
	require.NoError(t, err, "load YANG")
	argDefs := yang.PathToArgDefs(loader)

	cases := map[string]string{
		"system command help":     "name",
		"system command complete": "partial",
		"plugin command help":     "name",
		"plugin command complete": "partial",
	}
	for path, leaf := range cases {
		t.Run(path, func(t *testing.T) {
			defs := argDefs[path]
			require.Len(t, defs, 1, "the command declares exactly one leaf")
			assert.Equal(t, leaf, defs[0].Name)
			assert.True(t, defs[0].Mandatory, "the value is required")
			assert.NotEmpty(t, defs[0].ShortHelp, "the leaf carries its summary")
			assert.NotEmpty(t, defs[0].Description, "the leaf carries its explanation")
		})
	}
}

// TestCommandHelpAnswersTheKeywordForm drives the real dispatcher, loaded from
// the real YANG tree and the registered handlers, with the form the web admin
// form posts (keyword before value, ai/rules/cli.md) and with the positional
// form an operator types. Both must answer the help of the named command.
//
// PREVENTS: the dispatcher handing `name` to the handler as the command to
// look up, which answers "unknown command: name", or consuming the keyword
// pair and leaving the handler with no argument at all, which answers the
// usage line.
func TestCommandHelpAnswersTheKeywordForm(t *testing.T) {
	loader, err := yang.DefaultLoader()
	require.NoError(t, err, "load YANG")

	d := NewDispatcher()
	loadBuiltinsWithAliases(d, yang.WireMethodToPaths(loader), yang.PathToDescription(loader),
		yang.PathToHelp(loader), yang.PathToArgDefs(loader), yang.BuildCommandTree(loader))
	ctx := &CommandContext{Server: &Server{dispatcher: d}}

	t.Run("help keyword", func(t *testing.T) {
		resp, err := d.Dispatch(ctx, `system command help name "system command list"`)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status, "answer: %s", resp.Error)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok, "data: %#v", resp.Data)
		assert.Equal(t, "system command list", data[fieldCommand])
		assert.Equal(t, sourceBuiltin, data[fieldSource])
	})

	t.Run("help positional", func(t *testing.T) {
		resp, err := d.Dispatch(ctx, `system command help "system command list"`)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status, "answer: %s", resp.Error)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok, "data: %#v", resp.Data)
		assert.Equal(t, "system command list", data[fieldCommand])
	})

	t.Run("help missing", func(t *testing.T) {
		resp, err := d.Dispatch(ctx, `system command help`)
		require.Error(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "name", "the refusal names the missing leaf")
	})

	t.Run("complete keyword", func(t *testing.T) {
		resp, err := d.Dispatch(ctx, `system command complete partial "system command he"`)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status, "answer: %s", resp.Error)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok, "data: %#v", resp.Data)
		completions, ok := data[fieldCompletions].([]Completion)
		require.True(t, ok, "completions: %#v", data[fieldCompletions])
		require.Len(t, completions, 1)
		assert.Equal(t, "system command help", completions[0].Value)
	})

	t.Run("complete positional", func(t *testing.T) {
		resp, err := d.Dispatch(ctx, `system command complete "system command he"`)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status, "answer: %s", resp.Error)
	})

	t.Run("complete args form", func(t *testing.T) {
		// The args form completes one command's ARGUMENT and carries the
		// command name after the keyword; the partial leaf must not swallow it.
		resp, err := d.Dispatch(ctx, `system command complete args "system command list" x`)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status, "answer: %s", resp.Error)
	})

	t.Run("plugin help keyword", func(t *testing.T) {
		resp, err := d.Dispatch(ctx, `plugin command help name nothing`)
		require.Error(t, err)
		assert.Equal(t, "unknown plugin command: nothing", resp.Error,
			"the value after the keyword is the command looked up")
	})

	t.Run("plugin complete keyword", func(t *testing.T) {
		resp, err := d.Dispatch(ctx, `plugin command complete partial no`)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status, "answer: %s", resp.Error)
	})
}
