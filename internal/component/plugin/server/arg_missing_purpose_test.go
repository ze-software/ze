// Design: docs/architecture/api/commands.md -- required arguments explain their purpose.
package server

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestLoneMissingArgumentPurposeReachesRegisteredDispatcher loads the real
// restart command from YANG, then requires the name leaf's declared meaning
// in the actual dispatcher refusal and its original typed missing cause.
func TestLoneMissingArgumentPurposeReachesRegisteredDispatcher(t *testing.T) {
	d := restartLineDispatcher(t)
	const path = "request plugin restart"
	registered := d.commands[path]
	require.NotNil(t, registered)
	purpose := ""
	for _, def := range registered.ArgDefs {
		if def.Name() != "name" {
			continue
		}
		purpose = def.Description()
		if purpose == "" {
			purpose = def.ShortHelp()
		}
	}
	require.NotEmpty(t, purpose, "the registered YANG name leaf must declare its purpose")
	response, err := d.Dispatch(&CommandContext{}, path)
	require.Error(t, err)
	require.NotNil(t, response)
	assert.Equal(t, plugin.StatusError, response.Status)
	assert.Equal(t, err.Error(), response.Error)
	missing, found := errors.AsType[*command.MissingArgumentError](err)
	require.True(t, found)
	assert.Equal(t, "name", missing.Leaf)
	assert.Contains(t, missing.Error(), purpose)
	refusal, found := errors.AsType[*opererr.Error](err)
	require.True(t, found)
	assert.Contains(t, refusal.Why(), purpose)
	assert.Equal(t, path+" name <name>", refusal.Fix())
}

// TestLoneMissingArgumentSelectorReachesSharedRefusal refuses a registered
// mandatory selector through its actual guard, retaining the shared typed
// missing cause, declared purpose and correct interior or terminal placement.
// A complete selector must still reach the original handler exactly once.
func TestLoneMissingArgumentSelectorReachesSharedRefusal(t *testing.T) {
	const purpose = "Remote peer whose session this command stops"
	for _, tc := range []struct {
		path     string
		anchor   string
		tail     string
		fix      string
		complete string
	}{
		{path: "request purpose peer teardown", anchor: "peer", fix: "request purpose peer <selector> teardown", complete: "request purpose peer 192.0.2.7 teardown"},
		{path: "delete purpose peer", fix: "delete purpose peer <selector>", complete: "delete purpose peer 192.0.2.7"},
		{path: "request purpose peer teardown", anchor: "peer", tail: " family ipv4/unicast", fix: "request purpose peer <selector> teardown family ipv4/unicast", complete: "request purpose peer 192.0.2.7 teardown family ipv4/unicast"},
		{path: "delete purpose peer", tail: " family ipv4/unicast", fix: "delete purpose peer <selector> family ipv4/unicast", complete: "delete purpose peer 192.0.2.7 family ipv4/unicast"},
	} {
		t.Run(tc.path+tc.tail, func(t *testing.T) {
			d := NewDispatcher()
			called := 0
			require.NoError(t, d.RegisterWithOptions(tc.path, func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
				called++
				return &plugin.Response{Status: plugin.StatusDone}, nil
			}, "Stop one peer", RegisterOptions{RequiresSelector: true, ArgDefs: []command.ArgDef{
				commandtest.Must(command.NewStringArg("selector", nil, nil, command.ArgOptions{Mandatory: true, Anchor: tc.anchor, ShortHelp: purpose})),
				commandtest.Must(command.NewEnumArg("family", []string{"ipv4/unicast", "ipv6/unicast"}, command.ArgOptions{})),
			}}))
			response, err := d.Dispatch(&CommandContext{}, tc.path+tc.tail)
			require.Error(t, err)
			assert.Zero(t, called, "a missing selector must not reach the handler")
			missing, found := errors.AsType[*command.MissingArgumentError](err)
			require.True(t, found, "the selector guard must retain its typed missing cause")
			assert.Equal(t, "selector", missing.Leaf)
			assert.Contains(t, missing.Error(), purpose)
			faults, found := errors.AsType[*command.ArgumentFaults](err)
			require.True(t, found, "the selector guard must use the shared correction route")
			assert.Len(t, faults.Faults(), 1)
			refusal, found := errors.AsType[*opererr.Error](err)
			require.True(t, found)
			assert.Contains(t, refusal.Why(), purpose)
			assert.Equal(t, tc.fix, refusal.Fix())
			require.NotNil(t, response)
			assert.Equal(t, plugin.StatusError, response.Status)
			assert.Equal(t, err.Error(), response.Error)
			response, err = d.Dispatch(&CommandContext{}, tc.complete)
			require.NoError(t, err)
			require.NotNil(t, response)
			assert.Equal(t, plugin.StatusDone, response.Status)
			assert.Equal(t, 1, called)
		})
	}
}

// TestLoneMissingArgumentGuardNamesOnlyUnboundValues keeps the ambiguous
// positional diagnostic honest when a valid named option is also present.
// The guard must count and name the actual positional values, never that pair.
func TestLoneMissingArgumentGuardNamesOnlyUnboundValues(t *testing.T) {
	d := NewDispatcher()
	called := 0
	const path = "delete purpose peer"
	require.NoError(t, d.RegisterWithOptions(path, func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
		called++
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "Stop one peer", RegisterOptions{RequiresSelector: true, ArgDefs: []command.ArgDef{
		commandtest.Must(command.NewStringArg("selector", nil, nil, command.ArgOptions{Mandatory: true})),
		commandtest.Must(command.NewEnumArg("family", []string{"ipv4/unicast"}, command.ArgOptions{})),
	}}))
	_, err := d.Dispatch(&CommandContext{}, path+" family ipv4/unicast first second third")
	require.Error(t, err)
	assert.Zero(t, called)
	assert.Contains(t, err.Error(), "first second third are 3 values typed without a keyword")
	assert.NotContains(t, err.Error(), "family ipv4/unicast first second third are")
	assert.NotContains(t, err.Error(), "are 5 values")
}
