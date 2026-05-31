package sdk

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

func TestOnExecuteCommandAnyMap(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return "done", map[string]any{"running": true, "peers": 5}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:execute-command",
		rpc.ExecuteCommandInput{Command: "test"})
	require.NoError(t, err)

	var out rpc.ExecuteCommandOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, "done", out.Status)

	var data map[string]any
	require.NoError(t, json.Unmarshal(out.Data, &data))
	assert.Equal(t, true, data["running"])
	assert.Equal(t, float64(5), data["peers"])

	cancel()
	<-errCh
}

func TestOnExecuteCommandAnyStruct(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	type entry struct {
		Prefix  string `json:"prefix"`
		NextHop string `json:"next-hop"`
	}

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return "done", []entry{{Prefix: "10.0.0.0/24", NextHop: "10.0.0.1"}}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:execute-command",
		rpc.ExecuteCommandInput{Command: "show"})
	require.NoError(t, err)

	var out rpc.ExecuteCommandOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, "done", out.Status)

	var entries []entry
	require.NoError(t, json.Unmarshal(out.Data, &entries))
	require.Len(t, entries, 1)
	assert.Equal(t, "10.0.0.0/24", entries[0].Prefix)

	cancel()
	<-errCh
}

func TestOnExecuteCommandAnyNil(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return "done", nil, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:execute-command",
		rpc.ExecuteCommandInput{Command: "noop"})
	require.NoError(t, err)

	var out rpc.ExecuteCommandOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, "done", out.Status)
	assert.Empty(t, out.Data)

	cancel()
	<-errCh
}

func TestOnExecuteCommandAnySlice(t *testing.T) {
	t.Parallel()
	p, engine := newTestPair(t)

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return "done", []string{"cache", "route", "peer"}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx, Registration{}) }()
	completeStartup(t, ctx, engine)

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:execute-command",
		rpc.ExecuteCommandInput{Command: "events"})
	require.NoError(t, err)

	var out rpc.ExecuteCommandOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, "done", out.Status)

	var events []string
	require.NoError(t, json.Unmarshal(out.Data, &events))
	assert.Equal(t, []string{"cache", "route", "peer"}, events)

	cancel()
	<-errCh
}
