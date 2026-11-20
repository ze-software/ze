package rpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExecuteCommandInputCarriesPipe pins the wire form of the declared list
// on execute-command, which a plugin in any language reads.
//
// Method: marshal an input with two operations, one with argument words and
// one without, and one input with none; compare the exact JSON, then decode it
// back.
//
// VALIDATES: spec-pipe-chain-honored-on-every-command-surface AC-14 (the
// carrier), the "pipe" key, each element's "name" and "args", nil for none.
// PREVENTS: the list folded into "args", or an empty list published as
// "args":[] where nil means the operation took no argument.
func TestExecuteCommandInputCarriesPipe(t *testing.T) {
	t.Parallel()

	input := ExecuteCommandInput{
		Serial:  "1",
		Command: "show fixture pipe-op",
		Args:    []string{"alpha"},
		Pipe:    []PipeOperation{{Name: "top-talkers", Args: []string{"5"}}, {Name: "histogram"}},
	}
	encoded, err := json.Marshal(input)
	require.NoError(t, err)
	assert.Equal(t,
		`{"serial":"1","command":"show fixture pipe-op","args":["alpha"],"pipe":[{"name":"top-talkers","args":["5"]},{"name":"histogram"}]}`,
		string(encoded))

	var decoded ExecuteCommandInput
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, input, decoded, "the list round-trips, and an operation without words keeps nil args")

	none, err := json.Marshal(ExecuteCommandInput{Command: "show x"})
	require.NoError(t, err)
	assert.Equal(t, `{"serial":"","command":"show x"}`, string(none), "no declared operation: no pipe key")
}

// TestPipeDeclArgumentWire pins the two kinds of a Stage 1 pipe entry: an
// alias states "expansion", an operation states "argument", and neither key
// appears when it is not stated.
//
// VALIDATES: AC-14, the explicit argument field.
// PREVENTS: an operation published with "expansion":"", which a reader in
// another language would take for an alias that expands to nothing.
func TestPipeDeclArgumentWire(t *testing.T) {
	t.Parallel()

	operation, err := json.Marshal(PipeDecl{Command: "show x", Name: "top-talkers", Argument: "count"})
	require.NoError(t, err)
	assert.Equal(t, `{"command":"show x","name":"top-talkers","argument":"count"}`, string(operation))

	alias, err := json.Marshal(PipeDecl{Command: "show x", Name: "brief", Expansion: "table"})
	require.NoError(t, err)
	assert.Equal(t, `{"command":"show x","name":"brief","expansion":"table"}`, string(alias))
}

// TestDirectBridgeExecuteCommandCarriesPipe proves the in-process transport
// hands the handler the same list the socket carries.
//
// Method: register a typed handler, call ExecuteCommand with a list, drain the
// request as the SDK event loop does, and read what the handler received.
//
// VALIDATES: AC-14 for a plugin running in the daemon.
// PREVENTS: the list reaching only a forked plugin.
func TestDirectBridgeExecuteCommandCarriesPipe(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()
	var received []PipeOperation
	bridge.SetExecuteCommand(func(_, _ string, _ []string, _ string, pipe []PipeOperation) (*ExecuteCommandOutput, error) {
		received = pipe
		return &ExecuteCommandOutput{Status: StatusDone}, nil
	})
	bridge.SetReady()

	go func() {
		req := <-bridge.ExecuteCommandRequests()
		out, err := bridge.RunExecuteCommand(req)
		req.Result <- ExecuteCommandResult{Output: out, Err: err}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	want := []PipeOperation{{Name: "peer", Args: []string{"10.0.0.1"}}, {Name: "count"}}
	out, err := bridge.ExecuteCommand(ctx, "s", "show x", nil, "", want)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, want, received)
}
