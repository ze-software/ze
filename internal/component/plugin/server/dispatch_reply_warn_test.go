// Design: docs/architecture/api/process-protocol.md -- plugin RPC reply delivery and runtime request abort
// Related: dispatch.go -- dispatchPluginRPCContext and handleCodecRPC reply sends
// Related: request_runtime.go -- serveRuntimeRequests collision abort and rejection replies

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type lockedWarnBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedWarnBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedWarnBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *lockedWarnBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

func captureServerWarnLogs(t *testing.T) *lockedWarnBuffer {
	t.Helper()
	buf := &lockedWarnBuffer{}
	orig := logger
	captured := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	logger = func() *slog.Logger { return captured }
	t.Cleanup(func() { logger = orig })
	return buf
}

// failingWriteConn keeps Read open while failing every Write with writeErr,
// simulating a live connection whose reply cannot be delivered (for example,
// a write deadline or framing failure) without closing the peer reader.
type failingWriteConn struct {
	net.Conn
	writeErr error
}

func (c *failingWriteConn) Write([]byte) (int, error) {
	return 0, c.writeErr
}

// closeErrorConn closes its underlying connection and returns closeErr,
// triggering onClose once so blocked test handlers can release cleanly.
type closeErrorConn struct {
	net.Conn
	closeErr  error
	closeOnce sync.Once
	onClose   func()
}

func (c *closeErrorConn) Close() error {
	_ = c.Conn.Close()
	c.closeOnce.Do(func() {
		if c.onClose != nil {
			c.onClose()
		}
	})
	return c.closeErr
}

func reportIssuesByCode(code string) []report.Issue {
	var issues []report.Issue
	for _, issue := range report.Errors(0) {
		if issue.Code == code {
			issues = append(issues, issue)
		}
	}
	return issues
}

// TestDispatchReplyFailureOnLivePluginLogsWarnWithDoctorCode verifies that
// SendError and SendResult failures on a live plugin connection log at WARN
// with CodeDoctorPluginAnswerUndelivered and raise a rate-limited report-bus
// event, while ended plugins and stopping servers stay silent.
//
// VALIDATES: AC-2 and AC-3 for dispatch.go reply sends and rate-limited
// CodeDoctorPluginAnswerUndelivered reporting.
// PREVENTS: a live plugin waiting indefinitely after a reply send failure with
// no WARN log or doctor event, or normal shutdown emitting spurious WARN noise.
func TestDispatchReplyFailureOnLivePluginLogsWarnWithDoctorCode(t *testing.T) {
	const (
		livePluginName    = "live-reply-plugin"
		endedPluginName   = "ended-reply-plugin"
		stoppedPluginName = "stopping-reply-plugin"
		codecResultMethod = "test-codec-result"
		codecErrorMethod  = "test-codec-error"
		writeCause        = "write deadline exceeded"
	)

	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	logs := captureServerWarnLogs(t)

	newLivePair := func(t *testing.T, name string) (*process.Process, *plugipc.PluginConn) {
		t.Helper()
		pluginEnd, engineEnd := net.Pipe()
		t.Cleanup(func() {
			_ = pluginEnd.Close()
			_ = engineEnd.Close()
		})
		wrapped := &failingWriteConn{Conn: engineEnd, writeErr: errors.New(writeCause)}
		engineConn := plugipc.NewPluginConn(wrapped, wrapped)
		proc := process.NewProcess(plugin.PluginConfig{Name: name})
		proc.SetConn(engineConn)
		return proc, engineConn
	}

	t.Run("live SendError and SendResult failures warn and rate-limit report events", func(t *testing.T) {
		report.ResetForTest()
		logs.Reset()

		s := &Server{
			ctx: t.Context(),
			rpcHandlers: map[string]func(json.RawMessage) (any, error){
				codecResultMethod: func(json.RawMessage) (any, error) {
					return map[string]string{"status": "ok"}, nil
				},
				codecErrorMethod: func(json.RawMessage) (any, error) {
					return nil, errors.New("codec rejected payload")
				},
			},
		}
		proc, engineConn := newLivePair(t, livePluginName)

		// Unknown-method SendError (dispatch.go:133).
		s.dispatchPluginRPCContext(s.Context(), proc, engineConn, &rpc.Request{
			ID:     101,
			Method: "ze-plugin-engine:no-such-method",
		})
		// Codec SendError (dispatch.go:678).
		s.dispatchPluginRPCContext(s.Context(), proc, engineConn, &rpc.Request{
			ID:     102,
			Method: codecErrorMethod,
		})
		// Codec SendResult (dispatch.go:684).
		s.dispatchPluginRPCContext(s.Context(), proc, engineConn, &rpc.Request{
			ID:     103,
			Method: codecResultMethod,
		})

		gotLog := logs.String()
		require.Equal(t, 3, strings.Count(gotLog, "level=WARN"),
			"each live reply send failure must log at WARN:\n%s", gotLog)
		require.Contains(t, gotLog, "code="+CodeDoctorPluginAnswerUndelivered)
		require.Contains(t, gotLog, livePluginName)
		require.Contains(t, gotLog, writeCause)

		events := reportIssuesByCode(CodeDoctorPluginAnswerUndelivered)
		require.Len(t, events, 1, "report-bus events for one plugin must be rate-limited within the window")
		require.Equal(t, livePluginName, events[0].Subject)
		require.Contains(t, events[0].Message, livePluginName)
		require.NotEmpty(t, events[0].Fix)
	})

	t.Run("ended plugin stays silent", func(t *testing.T) {
		report.ResetForTest()
		logs.Reset()

		s := &Server{ctx: t.Context()}
		proc, engineConn := newLivePair(t, endedPluginName)
		// Clear proc's connection via Stop so proc.PluginEnded() reports true.
		proc.Stop()

		s.dispatchPluginRPCContext(s.Context(), proc, engineConn, &rpc.Request{
			ID:     104,
			Method: "ze-plugin-engine:no-such-method",
		})

		require.Empty(t, logs.String(), "ended plugin reply drop must not emit a WARN log")
		require.Empty(t, reportIssuesByCode(CodeDoctorPluginAnswerUndelivered),
			"ended plugin reply drop must not raise a report event")
	})

	t.Run("stopping server stays silent", func(t *testing.T) {
		report.ResetForTest()
		logs.Reset()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		s := &Server{ctx: ctx}
		proc, engineConn := newLivePair(t, stoppedPluginName)

		s.dispatchPluginRPCContext(s.Context(), proc, engineConn, &rpc.Request{
			ID:     105,
			Method: "ze-plugin-engine:no-such-method",
		})

		require.Empty(t, logs.String(), "stopping server reply drop must not emit a WARN log")
		require.Empty(t, reportIssuesByCode(CodeDoctorPluginAnswerUndelivered),
			"stopping server reply drop must not raise a report event")
	})
}

// TestServeRuntimeRequestsCollisionAbortErrorLogsWarnWithDoctorCode drives a
// duplicate in-flight request ID into Server.serveRuntimeRequests so it calls
// conn.AbortRuntime, testing both a non-closed close error and net.ErrClosed.
//
// VALIDATES: AC-2 and AC-3 for request_runtime.go collision abort errors: a
// non-closed AbortRuntime error logs at WARN with CodeDoctorPluginRequestRefused
// and raises a report-bus event, while net.ErrClosed stays silent.
// PREVENTS: a failed MuxConn close (not retried due to sync.Once) dropping below
// WARN after a request-ID collision.
func TestServeRuntimeRequestsCollisionAbortErrorLogsWarnWithDoctorCode(t *testing.T) {
	const (
		pluginName  = "collision-abort-plugin"
		blockMethod = "test-block-collision"
		closeCause  = "close collided socket failed: EIO"
	)

	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	logs := captureServerWarnLogs(t)

	runCollision := func(t *testing.T, closeErr error) {
		t.Helper()
		entered := make(chan struct{})
		release := make(chan struct{})
		var enterOnce sync.Once
		var releaseOnce sync.Once
		unlock := func() { releaseOnce.Do(func() { close(release) }) }
		t.Cleanup(unlock)

		s := &Server{
			ctx: t.Context(),
			rpcHandlers: map[string]func(json.RawMessage) (any, error){
				blockMethod: func(json.RawMessage) (any, error) {
					enterOnce.Do(func() { close(entered) })
					<-release
					return map[string]string{"status": "ok"}, nil
				},
			},
		}

		pluginEnd, engineEnd := net.Pipe()
		t.Cleanup(func() {
			_ = pluginEnd.Close()
			_ = engineEnd.Close()
		})
		wrapped := &closeErrorConn{Conn: engineEnd, closeErr: closeErr, onClose: unlock}
		mux := rpc.NewMuxConn(rpc.NewConn(wrapped, wrapped))
		engineConn := plugipc.NewMuxPluginConn(mux)
		proc := process.NewProcess(plugin.PluginConfig{Name: pluginName})
		proc.SetConn(engineConn)

		done := make(chan struct{})
		go func() {
			defer close(done)
			s.serveRuntimeRequests(proc, engineConn)
		}()

		_, err := pluginEnd.Write([]byte("#42 " + blockMethod + " {}\n"))
		require.NoError(t, err)
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("first request did not enter handler")
		}

		// Duplicate ID 42 while request 42 is still active triggers RequestIDCollision.
		_, err = pluginEnd.Write([]byte("#42 " + blockMethod + " {}\n"))
		require.NoError(t, err)

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("serveRuntimeRequests did not exit after request-ID collision")
		}
	}

	t.Run("non-closed AbortRuntime error warns and raises doctor code", func(t *testing.T) {
		report.ResetForTest()
		logs.Reset()

		runCollision(t, errors.New(closeCause))

		gotLog := logs.String()
		require.Contains(t, gotLog, "level=WARN", "non-closed AbortRuntime error must log at WARN")
		require.Contains(t, gotLog, "code="+diagnostic.CodeDoctorPluginRequestRefused)
		require.Contains(t, gotLog, pluginName)
		require.Contains(t, gotLog, closeCause)

		events := reportIssuesByCode(diagnostic.CodeDoctorPluginRequestRefused)
		require.Len(t, events, 1, "non-closed AbortRuntime error must raise one report-bus event")
		require.Equal(t, pluginName, events[0].Subject)
		require.Contains(t, events[0].Message, pluginName)
		require.Contains(t, events[0].Message, closeCause)
		require.NotEmpty(t, events[0].Fix)
	})

	t.Run("net.ErrClosed AbortRuntime stays silent", func(t *testing.T) {
		report.ResetForTest()
		logs.Reset()

		runCollision(t, net.ErrClosed)

		require.Empty(t, logs.String(), "net.ErrClosed on collision abort must not emit a WARN log")
		require.Empty(t, reportIssuesByCode(diagnostic.CodeDoctorPluginRequestRefused),
			"net.ErrClosed on collision abort must not raise a report event")
	})
}
