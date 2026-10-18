package server

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
)

// TestDispatcherRefusesACommandPrefix proves a prefix of registered commands
// never runs and is answered with every whole form below it.
//
// VALIDATES: ai/rules/cli.md, acting on everything needs an explicit all:
// "clear dns cache" stops before its selector, so it is refused and the
// refusal names "clear dns cache all" and the narrower forms. A handler
// registered on the whole form is not called.
// PREVENTS: a typo, an early Enter or a tab completion that stops at the
// prefix reaching the all-effect, or the operator being told only "unknown
// command" with no way to find the explicit form.
func TestDispatcherRefusesACommandPrefix(t *testing.T) {
	d := NewDispatcher()
	called := false
	handler := func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		called = true
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	for _, name := range []string{"clear dns cache all", "clear dns cache stats", "clear dns cache record"} {
		require.NoError(t, d.Register(name, handler, "test"))
	}

	resp, err := d.Dispatch(nil, "clear dns cache")
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.False(t, called, "a bare prefix ran a handler")

	var incomplete *IncompleteCommandError
	require.True(t, errors.As(err, &incomplete), "want IncompleteCommandError, got %T: %v", err, err)
	assert.Equal(t, "clear dns cache", incomplete.Prefix)
	assert.Equal(t, []string{"clear dns cache all", "clear dns cache record", "clear dns cache stats"}, incomplete.Forms)
	assert.True(t, strings.Contains(err.Error(), "\n  clear dns cache all"), "the refusal must name the explicit all form: %q", err.Error())

	_, err = d.Dispatch(nil, "clear dns cache all")
	require.NoError(t, err)
	assert.True(t, called, "the explicit all form must run")
}

// TestDispatcherUnknownInputIsNotAPrefix proves input that prefixes no
// registered command stays an unknown command.
//
// VALIDATES: the prefix refusal answers only for a strict prefix of a
// registered command, including a prefix that runs past a token boundary.
// PREVENTS: a typo being reported as an incomplete command with an empty list.
func TestDispatcherUnknownInputIsNotAPrefix(t *testing.T) {
	d := NewDispatcher()
	handler := func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	require.NoError(t, d.Register("clear dns cache all", handler, "test"))

	for _, input := range []string{"clear dns cach", "clear dns cache everything", "show nothing"} {
		_, err := d.Dispatch(nil, input)
		require.Error(t, err, input)
		assert.True(t, errors.Is(err, ErrUnknownCommand), "%q: want ErrUnknownCommand, got %v", input, err)
	}
}

// TestDispatcherRefusesAPluginCommandPrefix proves the refusal reads the
// plugin registry as well as the builtins, and leaves hidden commands out.
//
// VALIDATES: a prefix of plugin commands is refused with the visible plugin
// forms, merged and sorted with the builtin forms below the same prefix; a
// hidden plugin command is never listed, and a prefix whose only completions
// are hidden stays an unknown command.
// PREVENTS: the refusal naming a command that completion and help keep hidden,
// or a plugin-only prefix falling through to "unknown command".
func TestDispatcherRefusesAPluginCommandPrefix(t *testing.T) {
	d := NewDispatcher()
	handler := func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	require.NoError(t, d.Register("show widget builtin", handler, "test"))
	proc := process.NewProcess(plugin.PluginConfig{Name: "widget-proc"})
	results := d.Registry().Register(proc, []CommandDef{
		{Name: "show widget plugin", ShortHelp: "visible plugin form"},
		{Name: "show widget secret", ShortHelp: "hidden plugin form", Hidden: true},
		{Name: "debug vault dump", ShortHelp: "hidden only", Hidden: true},
	})
	for _, result := range results {
		require.True(t, result.OK, "%s: %s", result.Name, result.Error)
	}

	_, err := d.Dispatch(nil, "show widget")
	var incomplete *IncompleteCommandError
	require.True(t, errors.As(err, &incomplete), "want IncompleteCommandError, got %T: %v", err, err)
	assert.Equal(t, []string{"show widget builtin", "show widget plugin"}, incomplete.Forms)
	assert.NotContains(t, err.Error(), "secret", "the refusal named a hidden command")

	for _, input := range []string{"debug vault", "debug"} {
		_, err = d.Dispatch(nil, input)
		require.Error(t, err, input)
		assert.True(t, errors.Is(err, ErrUnknownCommand), "%q: a prefix of hidden commands only must stay unknown, got %v", input, err)
		assert.False(t, errors.As(err, &incomplete), "%q: the refusal revealed a hidden command: %v", input, err)
	}
}

// TestDispatcherRunsAWholeCommandThatIsAlsoAPrefix proves a registered
// command still runs when longer commands extend it.
//
// VALIDATES: "show peer" runs its own handler although "show peer detail" is
// registered too; only input that is no whole command is refused.
// PREVENTS: the prefix refusal shadowing a whole command because longer forms
// exist below it.
func TestDispatcherRunsAWholeCommandThatIsAlsoAPrefix(t *testing.T) {
	d := NewDispatcher()
	var ran []string
	register := func(name string) {
		handler := func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
			ran = append(ran, name)
			return &plugin.Response{Status: plugin.StatusDone}, nil
		}
		require.NoError(t, d.Register(name, handler, "test"))
	}
	register("show peer")
	register("show peer detail")

	resp, err := d.Dispatch(nil, "show peer")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, []string{"show peer"}, ran)
}

// TestSelectorRefusalPlacesTheSelectorWhereDispatchReadsIt proves the guard's
// refusal of a command whose selector is anchored to an interior key token
// names the line with `<selector>` after that token, not after the whole path.
// Method: register `request peer teardown` with its selector anchored to
// `peer`, the shape ze-peer-cmd.yang declares, and dispatch it bare and with
// two unkeyed values; then dispatch the documented form the refusal names and
// see the handler run with that selector.
//
// VALIDATES: the refusal reads `request peer <selector> teardown`, both on the
// bare form and in the keyword example of the two-value form, and the line it
// names is one Dispatch accepts.
// PREVENTS: a refusal whose fix, `request peer teardown <selector>`, is a
// line the dispatcher does not read the selector from.
func TestSelectorRefusalPlacesTheSelectorWhereDispatchReadsIt(t *testing.T) {
	const help = "* for every peer, or an address, a peer name or an AS pattern"
	defs := []command.ArgDef{
		commandtest.Must(command.NewStringArg(selectorLeaf, nil, nil,
			command.ArgOptions{Mandatory: true, Anchor: "peer", ShortHelp: help})),
		commandtest.Must(command.NewStringArg("reason", nil, nil, command.ArgOptions{})),
	}
	d := NewDispatcher()
	var gotPeer string
	handler := func(ctx *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		gotPeer = ctx.Peer
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}
	require.NoError(t, d.RegisterWithOptions("request peer teardown", handler, "Tear a session down",
		RegisterOptions{RequiresSelector: true, ArgDefs: defs}))

	_, err := d.Dispatch(&CommandContext{}, "request peer teardown")
	require.Error(t, err)
	assert.Equal(t, "request peer teardown requires a selector: request peer <selector> teardown ("+help+")", err.Error())

	_, err = d.Dispatch(&CommandContext{}, "request peer teardown 192.0.2.1 maintenance")
	require.Error(t, err)
	assert.Equal(t, "request peer teardown requires a selector: 192.0.2.1 maintenance are 2 values typed without a keyword,"+
		" so none is taken as the selector. Type the selector first and the other value after its keyword:"+
		" request peer 192.0.2.1 teardown reason maintenance", err.Error())

	resp, err := d.Dispatch(&CommandContext{}, "request peer 192.0.2.1 teardown reason maintenance")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "192.0.2.1", gotPeer, "the line the refusal names must bind the selector")
}
