// Design: docs/architecture/api/commands.md -- edit authorization for private read answers.
package server

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type privateAnswerPolicy struct{}

func (privateAnswerPolicy) Authorize(_, _, _ string, readOnly bool) bool { return readOnly }

// TestBuiltinPrivateReadRequiresEditAuthorization exercises the actual alias
// loader with a show command holding private bytes, a reader and an edit owner.
func TestBuiltinPrivateReadRequiresEditAuthorization(t *testing.T) {
	dispatcher := NewDispatcher()
	called := 0
	registration := RPCRegistration{WireMethod: "test:private", NotReadOnly: true,
		Handler: func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
			called++
			return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"private": "bytes"}}, nil
		}}
	model, err := yang.DefaultLoader()
	require.NoError(t, err)
	paths := yang.WireMethodToPaths(model)
	paths[registration.WireMethod] = []string{"show test private", "show test alternate"}
	registrations := append(AllBuiltinRPCs(), registration)
	require.NoError(t, loadBuiltinsWithAliases(dispatcher, registrations,
		paths, yang.PathToDescription(model), yang.PathToHelp(model), yang.PathToArgDefs(model), yang.BuildCommandTree(model)))
	for _, line := range []string{"show test private", "show test alternate"} {
		response, err := dispatcher.Dispatch(&CommandContext{Username: "reader", Authorizer: privateAnswerPolicy{}, Sender: plugin.OperatorSender()}, line)
		require.ErrorIs(t, err, ErrUnauthorized)
		require.Equal(t, plugin.StatusError, response.Status)
	}
	require.Zero(t, called)
	response, err := dispatcher.Dispatch(&CommandContext{Username: "owner", Sender: plugin.OperatorSender()}, "show test private")
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status)
	require.Equal(t, 1, called)
}

func privateVersionRegistration(t *testing.T) string {
	t.Helper()
	saved := registeredRPCs
	t.Cleanup(func() { registeredRPCs = saved })
	registeredRPCs = append([]RPCRegistration(nil), saved...)
	const method = "ze-plugin:system-version-software"
	found := false
	for index := range registeredRPCs {
		if registeredRPCs[index].WireMethod == method {
			registeredRPCs[index].NotReadOnly = true
			found = true
		}
	}
	require.True(t, found, "the fixture needs the real server-owned version registration")
	return method
}

// TestLoadBuiltinsPrivateReadRequiresEditAuthorization exercises the public
// native loader using a real modeled registration with the private-read flag.
func TestLoadBuiltinsPrivateReadRequiresEditAuthorization(t *testing.T) {
	method := privateVersionRegistration(t)
	model, err := yang.DefaultLoader()
	require.NoError(t, err)
	paths := yang.WireMethodToPath(model)
	dispatcher := NewDispatcher()
	require.NoError(t, LoadBuiltins(dispatcher, paths, yang.PathToDescription(model), yang.PathToHelp(model), yang.PathToArgDefs(model), yang.BuildCommandTree(model)))
	response, err := dispatcher.Dispatch(&CommandContext{Username: "reader", Authorizer: privateAnswerPolicy{}, Sender: plugin.OperatorSender()}, paths[method])
	require.ErrorIs(t, err, ErrUnauthorized)
	require.Equal(t, plugin.StatusError, response.Status)
	response, err = dispatcher.Dispatch(&CommandContext{Username: "owner", Sender: plugin.OperatorSender()}, paths[method])
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status)
}

// TestWireBuiltinPrivateReadRequiresEditAuthorization reaches NewServer's wire
// binding with the same real modeled registration and observes its refusal.
func TestWireBuiltinPrivateReadRequiresEditAuthorization(t *testing.T) {
	method := privateVersionRegistration(t)
	server, err := NewServer(&ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	server.dispatcher.SetAuthorizer(privateAnswerPolicy{})
	answer := server.rpcDispatcher.Dispatch(&rpc.Request{ID: 1, Method: method})
	refusal, ok := answer.(*ipc.DispatchError)
	require.True(t, ok, "a private wire answer must refuse the read policy: %#v", answer)
	require.Equal(t, "unauthorized", refusal.Error)
	server.dispatcher.SetAuthorizer(nil)
	answer = server.rpcDispatcher.Dispatch(&rpc.Request{ID: 2, Method: method})
	require.IsType(t, &ipc.DispatchResult{}, answer)
}
