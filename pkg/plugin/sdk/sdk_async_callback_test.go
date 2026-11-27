// Design: docs/architecture/api/process-protocol.md -- SDK async callback answer delivery
// Related: sdk_async_callback.go -- Plugin.answerAsyncCallback under test

package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type sdkWarnBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *sdkWarnBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *sdkWarnBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *sdkWarnBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

type sdkWriteErrorConn struct {
	net.Conn
	writeErr error
}

func (c *sdkWriteErrorConn) Write([]byte) (int, error) {
	return 0, c.writeErr
}

// TestAnswerAsyncCallbackLiveSendFailureLogsWarn exercises
// Plugin.answerAsyncCallback against a live connection whose write fails,
// a stopped event loop, and a closed connection.
//
// VALIDATES: AC-3 for pkg/plugin/sdk/sdk_async_callback.go: a live send failure
// logs at slog.Warn naming the plugin, request ID, why, and fix, while a stopped
// event loop or closed connection emits zero WARN lines.
// PREVENTS: a live async callback receipt failure dropping below WARN while the
// engine waits for the callback answer, or shutdown emitting spurious WARN logs.
func TestAnswerAsyncCallbackLiveSendFailureLogsWarn(t *testing.T) {
	const (
		pluginName = "test-async-plugin"
		writeCause = "write deadline exceeded"
	)

	logs := &sdkWarnBuffer{}
	orig := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(orig) })

	t.Run("live send failure logs WARN naming plugin, request ID, why and fix", func(t *testing.T) {
		logs.Reset()

		engineEnd, pluginEnd := net.Pipe()
		t.Cleanup(func() {
			_ = engineEnd.Close()
			_ = pluginEnd.Close()
		})
		wrapped := &sdkWriteErrorConn{Conn: pluginEnd, writeErr: errors.New(writeCause)}
		mux := rpc.NewMuxConn(rpc.NewConn(wrapped, wrapped))
		t.Cleanup(func() { _ = mux.Close() })

		p := &Plugin{name: pluginName, engineMux: mux}
		p.answerAsyncCallback(t.Context(), 42, time.Now().Add(time.Second), json.RawMessage(`{"ok":true}`), nil)

		gotLog := logs.String()
		require.Contains(t, gotLog, "level=WARN", "live async callback send failure must log at WARN")
		require.Contains(t, gotLog, pluginName, "WARN log must name the plugin")
		require.Contains(t, gotLog, "42", "WARN log must name the request ID")
		require.Contains(t, gotLog, writeCause, "WARN log must explain why the send failed")
		require.Contains(t, gotLog, "fix=", "WARN log must include a fix attribute")
	})

	t.Run("stopped loop stays silent", func(t *testing.T) {
		logs.Reset()

		engineEnd, pluginEnd := net.Pipe()
		t.Cleanup(func() {
			_ = engineEnd.Close()
			_ = pluginEnd.Close()
		})
		wrapped := &sdkWriteErrorConn{Conn: pluginEnd, writeErr: errors.New(writeCause)}
		mux := rpc.NewMuxConn(rpc.NewConn(wrapped, wrapped))
		t.Cleanup(func() { _ = mux.Close() })

		loopCtx, cancel := context.WithCancel(t.Context())
		cancel()

		p := &Plugin{name: pluginName, engineMux: mux}
		p.answerAsyncCallback(loopCtx, 43, time.Now().Add(time.Second), json.RawMessage(`{"ok":true}`), nil)

		require.Empty(t, logs.String(), "stopped loop must not emit a WARN log")
	})

	t.Run("closed connection stays silent", func(t *testing.T) {
		logs.Reset()

		engineEnd, pluginEnd := net.Pipe()
		mux := rpc.NewMuxConn(rpc.NewConn(pluginEnd, pluginEnd))
		_ = engineEnd.Close()
		_ = mux.Close()
		select {
		case <-mux.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("closed MuxConn did not finish")
		}

		p := &Plugin{name: pluginName, engineMux: mux}
		p.answerAsyncCallback(t.Context(), 44, time.Now().Add(time.Second), json.RawMessage(`{"ok":true}`), nil)

		require.Empty(t, logs.String(), "closed connection must not emit a WARN log")
	})
}
