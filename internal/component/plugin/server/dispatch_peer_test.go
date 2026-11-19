// Design: docs/architecture/api/process-protocol.md -- shared physical command evaluator.
package server

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"testing"
	"time"
)

// The actual registered engine method on native and framed transports must
// preserve its authenticated process and physical constraint to the command
// handler, refuse unsupported commands before mutation, then remain useful.
func TestDispatchPhysicalPeerRegisteredEngineParity(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	s := &Server{ctx: ctx, dispatcher: NewDispatcher(), subscriptions: newSubscriptionManager()}
	calls := 0
	require.NoError(t, s.dispatcher.RegisterWithOptions("physical command", func(commandCtx *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		calls++
		require.Equal(t, uint64(901), commandCtx.PeerOwner)
		require.Equal(t, uint64(902), commandCtx.ConnectionID)
		require.Equal(t, plugin.ProcessSender(commandCtx.Process.Name()), commandCtx.Sender)
		require.NoError(t, commandCtx.Context().Err())
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "physical command", RegisterOptions{AcceptsPeerConnection: true}))
	require.NoError(t, s.dispatcher.Register("unsupported command", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
		t.Error("unsupported physical constraint reached a handler")
		return nil, nil
	}, "unsupported"))
	proc := nativeOwnerProcess(t, s)
	proc.Bridge().SetReady()
	conn, remote := newDriverPipe(t)
	for _, name := range []string{"physical command", "unsupported command", "physical command"} {
		input := rpc.DispatchCommandInput{Command: name, PeerOwner: 901, ConnectionID: 902, DeadlineMS: time.Now().Add(time.Second).UnixMilli()}
		value, handled, nativeErr := proc.Bridge().DispatchNative(ctx, rpc.MethodDispatchCommand, input)
		require.True(t, handled)
		served := make(chan struct{})
		go func() {
			defer close(served)
			request, err := conn.ReadRequest(ctx)
			if err == nil {
				s.dispatchPluginRPC(proc, conn, request)
			}
		}()
		wire, wireErr := remote.CallRPC(ctx, rpc.MethodDispatchCommand, input)
		select {
		case <-served:
		case <-ctx.Done():
			t.Fatal("physical command server did not finish")
		}
		if name == "unsupported command" {
			require.Error(t, nativeErr)
			require.Error(t, wireErr)
			continue
		}
		require.NoError(t, nativeErr)
		require.NoError(t, wireErr)
		output, ok := value.(*rpc.DispatchCommandOutput)
		require.True(t, ok)
		require.NoError(t, output.ValidatePeerAnswer(input))
		var framed rpc.DispatchCommandOutput
		require.NoError(t, json.Unmarshal(wire, &framed))
		require.NoError(t, framed.ValidatePeerAnswer(input))
	}
	require.Equal(t, 4, calls)
}
