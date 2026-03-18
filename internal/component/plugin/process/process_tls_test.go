package process

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// TestProcessSingleConnInitConns verifies that SetSingleConn + InitConns creates
// working MuxPluginConns that handle bidirectional RPC on a single connection.
//
// VALIDATES: Single-conn mode produces ConnA and ConnB backed by MuxConn.
// PREVENTS: Nil ConnA/ConnB or wrong wiring in single-conn mode.
func TestProcessSingleConnInitConns(t *testing.T) {
	t.Parallel()

	engineEnd, pluginEnd := net.Pipe()
	defer engineEnd.Close() //nolint:errcheck // test cleanup
	defer pluginEnd.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	proc := NewProcess(plugin.PluginConfig{Name: "test-single-conn"})
	proc.SetSingleConn(engineEnd)
	require.NoError(t, proc.InitConns())

	connA := proc.ConnA()
	connB := proc.ConnB()
	require.NotNil(t, connA, "ConnA should not be nil in single-conn mode")
	require.NotNil(t, connB, "ConnB should not be nil in single-conn mode")

	// Plugin side: send a request (simulating plugin->engine RPC).
	pluginConn := rpc.NewConn(pluginEnd, pluginEnd)
	go func() {
		line := rpc.FormatRequest(1, "ze-plugin-engine:declare-registration", json.RawMessage(`{"families":[]}`))
		_ = pluginConn.WriteRawFrame(append(line, '\n'))
	}()

	// Engine side: read from ConnA (should receive the plugin's request via MuxConn).
	req, err := connA.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)
	assert.Equal(t, uint64(1), req.ID)

	// Start plugin-side reader BEFORE engine writes response.
	// net.Pipe() is synchronous: writes block until the reader is ready.
	respCh := make(chan *rpc.Request, 1)
	go func() {
		resp, readErr := pluginConn.ReadRequest(ctx)
		if readErr != nil {
			return
		}
		respCh <- resp
	}()

	// Engine side: respond via ConnA.
	require.NoError(t, connA.SendOK(ctx, req.ID))

	// Plugin side: verify the response arrived.
	select {
	case resp := <-respCh:
		assert.Equal(t, uint64(1), resp.ID)
	case <-ctx.Done():
		t.Fatal("timed out waiting for response")
	}
}

// TestProcessSingleConnBidirectional verifies that ConnB can call the plugin
// while ConnA reads plugin requests, all on the same underlying connection.
//
// VALIDATES: Engine can send callbacks via ConnB while reading requests via ConnA.
// PREVENTS: Deadlock or data corruption from bidirectional traffic on single conn.
func TestProcessSingleConnBidirectional(t *testing.T) {
	t.Parallel()

	engineEnd, pluginEnd := net.Pipe()
	defer engineEnd.Close() //nolint:errcheck // test cleanup
	defer pluginEnd.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	proc := NewProcess(plugin.PluginConfig{Name: "test-bidir"})
	proc.SetSingleConn(engineEnd)
	require.NoError(t, proc.InitConns())

	connB := proc.ConnB()
	pluginConn := rpc.NewConn(pluginEnd, pluginEnd)
	pluginMux := rpc.NewMuxConn(pluginConn)
	defer pluginMux.Close() //nolint:errcheck // test cleanup

	// Plugin side: handle incoming callback from engine.
	go func() {
		req, ok := <-pluginMux.Requests()
		if !ok {
			return
		}
		_ = pluginMux.SendResult(ctx, req.ID, map[string]string{"status": "configured"})
	}()

	// Engine side: send callback via ConnB (e.g., configure).
	result, err := connB.CallRPC(ctx, "ze-plugin-callback:configure", map[string]any{"sections": []any{}})
	require.NoError(t, err)

	var resp struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(result, &resp))
	assert.Equal(t, "configured", resp.Status)
}
