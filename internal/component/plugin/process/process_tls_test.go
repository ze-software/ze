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

// TestProcessSingleConnInitConns verifies that rawConn + InitConns creates
// a working MuxPluginConn for bidirectional RPC on a single connection.
//
// VALIDATES: InitConns produces a working Conn backed by MuxConn.
// PREVENTS: Nil Conn or wrong wiring after InitConns.
func TestProcessSingleConnInitConns(t *testing.T) {
	t.Parallel()

	engineEnd, pluginEnd := net.Pipe()
	defer engineEnd.Close() //nolint:errcheck // test cleanup
	defer pluginEnd.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	proc := NewProcess(plugin.PluginConfig{Name: "test-single-conn"})
	proc.rawConn = engineEnd
	require.NoError(t, proc.InitConns())

	conn := proc.Conn()
	require.NotNil(t, conn, "Conn should not be nil after InitConns")

	// Plugin side: send a request (simulating plugin->engine RPC).
	pluginConn := rpc.NewConn(pluginEnd, pluginEnd)
	go func() {
		line := rpc.FormatRequest(1, "ze-plugin-engine:declare-registration", json.RawMessage(`{"families":[]}`))
		_ = pluginConn.WriteRawFrame(append(line, '\n'))
	}()

	// Engine side: read from ConnA (should receive the plugin's request via MuxConn).
	req, err := conn.ReadRequest(ctx)
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
	require.NoError(t, conn.SendOK(ctx, req.ID))

	// Plugin side: verify the response arrived.
	select {
	case resp := <-respCh:
		assert.Equal(t, uint64(1), resp.ID)
	case <-ctx.Done():
		t.Fatal("timed out waiting for response")
	}
}

// TestProcessSingleConnBidirectional verifies that the engine can call the plugin
// via the single MuxConn connection.
//
// VALIDATES: Engine can send callbacks while reading requests on the same connection.
// PREVENTS: Deadlock or data corruption from bidirectional traffic on single conn.
func TestProcessSingleConnBidirectional(t *testing.T) {
	t.Parallel()

	engineEnd, pluginEnd := net.Pipe()
	defer engineEnd.Close() //nolint:errcheck // test cleanup
	defer pluginEnd.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	proc := NewProcess(plugin.PluginConfig{Name: "test-bidir"})
	proc.rawConn = engineEnd
	require.NoError(t, proc.InitConns())

	conn := proc.Conn()
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
	result, err := conn.CallRPC(ctx, "ze-plugin-callback:configure", map[string]any{"sections": []any{}})
	require.NoError(t, err)

	var resp struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(result, &resp))
	assert.Equal(t, "configured", resp.Status)
}
