package sdk

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestExecuteCommandHandlerReceivesPipeFromTheWire proves the SDK hands an
// OnExecuteCommand handler the declared list a forked plugin reads off the
// socket, beside its arguments and never inside them.
//
// Method: executeCommandAnswer, the one decoder of the callback both the
// callback map and the socket loop run, over a request holding a list, and
// over one holding none.
//
// VALIDATES: spec-pipe-chain-honored-on-every-command-surface AC-14 (the SDK
// delivers the list), AC-2/AC-9 carrier half.
// PREVENTS: the "pipe" key decoded and dropped, or merged into args.
func TestExecuteCommandHandlerReceivesPipeFromTheWire(t *testing.T) {
	t.Parallel()

	var gotArgs []string
	var gotPipe []PipeOperation
	handler := func(_, _ string, args []string, _ string, pipe []PipeOperation) (string, any, error) {
		gotArgs = args
		gotPipe = pipe
		return rpc.StatusDone, map[string]any{"ok": true}, nil
	}

	params := json.RawMessage(`{"serial":"1","command":"show x","args":["alpha"],` +
		`"pipe":[{"name":"top-talkers","args":["5"]},{"name":"histogram"}]}`)
	_, err := executeCommandAnswer(handler, params, nil, 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha"}, gotArgs, "no operation word reaches the arguments")
	assert.Equal(t, []PipeOperation{{Name: "top-talkers", Args: []string{"5"}}, {Name: "histogram"}}, gotPipe)

	_, err = executeCommandAnswer(handler, json.RawMessage(`{"serial":"2","command":"show x"}`), nil, 0)
	require.NoError(t, err)
	assert.Nil(t, gotPipe, "a line that declared none hands the handler nil")
}

// TestExecuteCommandHandlerReceivesPipeOverTheBridge proves a plugin running
// in the daemon receives the same list through DirectBridge.
//
// Method: a bridged test pair, the startup completed, then
// bridge.ExecuteCommand with a list.
//
// VALIDATES: AC-14 on the in-process transport.
// PREVENTS: the list reaching a forked plugin only.
func TestExecuteCommandHandlerReceivesPipeOverTheBridge(t *testing.T) {
	t.Parallel()

	p, engine, bridge := newBridgedTestPair(t)
	received := make(chan []PipeOperation, 1)
	p.OnExecuteCommand(func(_, _ string, _ []string, _ string, pipe []PipeOperation) (string, any, error) {
		received <- pipe
		return rpc.StatusDone, map[string]any{}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, &Registration{})
	}()

	completeStartup(t, ctx, engine)
	require.Eventually(t, func() bool { return bridge.HasExecuteCommand() }, 2*time.Second, 10*time.Millisecond,
		"execute-command bridge handler should be ready after startup")

	want := []PipeOperation{{Name: "peer", Args: []string{"10.0.0.1"}}, {Name: "count"}}
	out, err := bridge.ExecuteCommand(ctx, "serial-1", "show x", nil, "", want)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, want, <-received)

	byeParams, err := json.Marshal(struct {
		Reason string `json:"reason"`
	}{Reason: "done"})
	require.NoError(t, err)
	_, err = bridge.SendCallback(ctx, callbackBye, byeParams)
	require.NoError(t, err)
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}
