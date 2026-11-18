// Design: docs/architecture/api/commands.md -- every route validates.
// Related: arg_faults.go -- dispatcher-specific whole-command correction.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestLoneMissingArgumentDaemonRoutes enters every daemon validation route
// with exactly one missing mandatory leaf. The whole-command fix survives
// each entry point before its handler or process lookup. In-process errors
// retain the typed cause; the RPC boundary retains its coded wire error.
// Positive complete inputs prove that the same entry points still proceed.
func TestLoneMissingArgumentDaemonRoutes(t *testing.T) {
	withRouteFaultsModel(t)
	args := []string{"asn", "65001", "router-id", "192.0.2.1"}
	const line = "show test route faults asn 65001 router-id 192.0.2.1"
	complete := append(append([]string{}, args...), "hold-time", "30")
	t.Run("dispatcher", func(t *testing.T) {
		called := 0
		d := NewDispatcher()
		require.NoError(t, d.RegisterWithOptions(routeFaultsPath, func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
			called++
			return &plugin.Response{Status: plugin.StatusDone}, nil
		}, "Show argument correction", RegisterOptions{ReadOnly: true, ArgDefs: []command.ArgDef{
			commandtest.Must(command.NewUintArg("asn", 32, nil, command.ArgOptions{Mandatory: true})),
			commandtest.Must(command.NewUintArg("hold-time", 16, nil, command.ArgOptions{Mandatory: true})),
			commandtest.Must(command.NewStringArg("router-id", nil, nil, command.ArgOptions{})),
		}}))
		response, err := d.Dispatch(&CommandContext{}, line)
		requireDaemonLoneMissingArgument(t, err)
		require.NotNil(t, response)
		assert.Equal(t, plugin.StatusError, response.Status)
		assert.Equal(t, err.Error(), response.Error)
		assert.Zero(t, called)
		response, err = d.Dispatch(&CommandContext{}, line+" hold-time 30")
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, plugin.StatusDone, response.Status)
		assert.Equal(t, 1, called)
	})
	t.Run("plugin-process", func(t *testing.T) {
		d := NewDispatcher()
		registered := &RegisteredCommand{Name: routeFaultsPath, LowerName: routeFaultsPath}
		response, err := d.routeToProcess(nil, registered, args, "*")
		requireDaemonLoneMissingArgument(t, err)
		require.NotNil(t, response)
		assert.Equal(t, err.Error(), response.Error)
		_, err = d.routeToProcess(nil, registered, complete, "*")
		assert.ErrorIs(t, err, ErrPluginProcessNotRunning)
	})
	t.Run("subsystem", func(t *testing.T) {
		d := NewDispatcher()
		handler := NewSubsystemHandler(SubsystemConfig{Name: "test"})
		handler.commands = []string{routeFaultsPath}
		_, err := d.dispatchSubsystem(&CommandContext{}, handler, line)
		requireDaemonLoneMissingArgument(t, err)
		_, err = d.dispatchSubsystem(&CommandContext{}, handler, line+" hold-time 30")
		assert.ErrorIs(t, err, ErrSubsystemNotRunning)
	})
	t.Run("RPC-wrapper", func(t *testing.T) {
		called := 0
		s := &Server{dispatcher: NewDispatcher()}
		handler := s.wrapHandler(func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
			called++
			return &plugin.Response{Status: plugin.StatusDone}, nil
		}, routeFaultsPath, true)
		encoded, err := json.Marshal(rpcParams{Args: args})
		require.NoError(t, err)
		_, err = handler("m", encoded)
		require.Error(t, err)
		coded, found := errors.AsType[*rpc.CodedError](err)
		require.True(t, found, "the RPC boundary preserves its documented coded wire error")
		assert.Equal(t, "invalid-params", coded.Code)
		assert.Equal(t, "required argument missing: hold-time\n"+
			"The command definition makes hold-time mandatory and supplies no further explanation\n"+
			"fix: "+line+" hold-time <hold-time>", coded.Error())
		assert.Zero(t, called)
		encoded, err = json.Marshal(rpcParams{Args: complete})
		require.NoError(t, err)
		_, err = handler("m", encoded)
		require.NoError(t, err)
		assert.Equal(t, 1, called)
	})
	t.Run("SSH-streaming", func(t *testing.T) {
		streamingHandlersMu.Lock()
		saved := streamingHandlers
		streamingHandlers = make(map[string]streamingRegistration)
		streamingHandlersMu.Unlock()
		t.Cleanup(func() {
			streamingHandlersMu.Lock()
			streamingHandlers = saved
			streamingHandlersMu.Unlock()
		})
		called := 0
		RegisterStreamingHandler(routeFaultsPath, OneShotSample, func(context.Context, *Server, io.Writer, string, command.ValidatedArgs) error {
			called++
			return nil
		})
		handler, _, err := GetStreamingHandlerForCommand(line)
		requireDaemonLoneMissingArgument(t, err)
		assert.Nil(t, handler)
		assert.Zero(t, called)
		handler, validated, err := GetStreamingHandlerForCommand(line + " hold-time 30")
		require.NoError(t, err)
		require.NotNil(t, handler)
		assert.Equal(t, complete, validated.Tokens())
		require.NoError(t, handler(context.Background(), &Server{}, io.Discard, "alice", validated))
		assert.Equal(t, 1, called)
	})
}

// requireDaemonLoneMissingArgument checks the one common refusal reached by
// the real daemon route, including the operator error and original leaf cause.
func requireDaemonLoneMissingArgument(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	missing, found := errors.AsType[*command.MissingArgumentError](err)
	require.True(t, found)
	assert.Equal(t, "hold-time", missing.Leaf)
	faults, found := errors.AsType[*command.ArgumentFaults](err)
	require.True(t, found, "a lone missing argument must use the shared correction route")
	assert.Len(t, faults.Faults(), 1)
	refusal, found := errors.AsType[*opererr.Error](err)
	require.True(t, found)
	assert.Contains(t, refusal.Problem(), "hold-time")
	assert.Contains(t, refusal.Why(), "hold-time")
	assert.Equal(t, "show test route faults asn 65001 router-id 192.0.2.1 hold-time <hold-time>", refusal.Fix())
}
