package hub

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/api"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// VALIDATES: API execute wiring preserves request context and remote address into dispatcher context.
// PREVENTS: REST/gRPC metadata reaching APIEngine but being dropped before Dispatcher.Dispatch().
func TestAPIExecutorPropagatesRequestContextAndRemoteAddr(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	type ctxKey struct{}

	var seen *pluginserver.CommandContext
	server.Dispatcher().Register("test api", func(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
		seen = ctx
		return &plugin.Response{Status: plugin.StatusDone, Data: "ok"}, nil
	}, "test api")

	exec := apiExecutor(server)
	requestCtx := context.WithValue(context.Background(), ctxKey{}, "trace-id")

	output, err := exec(requestCtx, api.CallerIdentity{
		Username:   "alice",
		RemoteAddr: "198.51.100.10:4444",
	}, "test api")
	require.NoError(t, err)
	assert.Equal(t, "ok", output)

	require.NotNil(t, seen)
	assert.Equal(t, "alice", seen.Username)
	assert.Equal(t, "198.51.100.10:4444", seen.RemoteAddr)
	assert.Same(t, requestCtx, seen.Context())
	assert.Equal(t, "trace-id", seen.Context().Value(ctxKey{}))
}

// VALIDATES: API streaming uses pluginserver streaming handlers with caller metadata and accounting.
// PREVENTS: REST/gRPC Stream staying disconnected from the production monitor path.
func TestAPIStreamSourceRunsStreamingHandler(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	const command = "test api stream source lines"
	t.Cleanup(func() { pluginserver.UnregisterStreamingHandler(command) })
	var seen struct {
		server   *pluginserver.Server
		username string
		args     []string
	}
	pluginserver.RegisterStreamingHandler(command, func(_ context.Context, srv *pluginserver.Server, w io.Writer, username string, args []string) error {
		seen.server = srv
		seen.username = username
		seen.args = append([]string(nil), args...)
		if _, writeErr := fmt.Fprintln(w, "first"); writeErr != nil {
			return writeErr
		}
		_, writeErr := fmt.Fprintln(w, "second")
		return writeErr
	})

	acct := &apiStreamTestAccountant{}
	server.Dispatcher().SetAccountingHook(acct)

	stream := apiStreamSource(server)
	ch, cancel, err := stream(context.Background(), api.CallerIdentity{
		Username:   "alice",
		RemoteAddr: "198.51.100.10:4444",
	}, command+" arg")
	require.NoError(t, err)
	defer cancel()

	var lines []string
	for line := range ch {
		lines = append(lines, line)
	}
	assert.Equal(t, []string{"first", "second"}, lines)
	assert.Same(t, server, seen.server)
	assert.Equal(t, "alice", seen.username)
	assert.Equal(t, []string{"arg"}, seen.args)
	assert.Equal(t, []string{command + " arg"}, acct.starts)
	assert.Equal(t, []string{command + " arg"}, acct.stops)
}

// VALIDATES: API streaming returns handler startup errors before opening the stream.
// PREVENTS: malformed monitor requests becoming silent empty streams.
func TestAPIStreamSourceReturnsHandlerStartupError(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	const command = "test api stream source error"
	t.Cleanup(func() { pluginserver.UnregisterStreamingHandler(command) })
	pluginserver.RegisterStreamingHandler(command, func(context.Context, *pluginserver.Server, io.Writer, string, []string) error {
		return fmt.Errorf("bad stream arguments")
	})

	stream := apiStreamSource(server)
	_, _, err = stream(context.Background(), api.CallerIdentity{Username: "alice"}, command)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad stream arguments")
}

// VALIDATES: API streaming uses dispatcher authorization with read-only semantics and caller origin.
// PREVENTS: API stream endpoints bypassing command authorization.
func TestAPIStreamSourceAuthorizesReadOnly(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	const command = "test api stream source auth"
	t.Cleanup(func() { pluginserver.UnregisterStreamingHandler(command) })
	ran := false
	pluginserver.RegisterStreamingHandler(command, func(context.Context, *pluginserver.Server, io.Writer, string, []string) error {
		ran = true
		return nil
	})

	auth := &apiStreamTestAuthorizer{allow: false}
	server.Dispatcher().SetAuthorizer(auth)

	stream := apiStreamSource(server)
	_, _, err = stream(context.Background(), api.CallerIdentity{
		Username:   "alice",
		RemoteAddr: "198.51.100.10:4444",
	}, command)
	assert.ErrorIs(t, err, api.ErrUnauthorized)
	assert.False(t, ran)
	assert.Equal(t, "alice", auth.username)
	assert.Equal(t, "198.51.100.10:4444", auth.remoteAddr)
	assert.Equal(t, command, auth.command)
	assert.True(t, auth.readOnly)
}

// VALIDATES: API streaming propagates context cancellation to the handler.
// PREVENTS: leaked streaming goroutines when the client disconnects.
func TestAPIStreamSourceCancelStopsHandler(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	const command = "test api stream source cancel"
	t.Cleanup(func() { pluginserver.UnregisterStreamingHandler(command) })
	handlerDone := make(chan struct{})
	pluginserver.RegisterStreamingHandler(command, func(ctx context.Context, _ *pluginserver.Server, w io.Writer, _ string, _ []string) error {
		defer close(handlerDone)
		if _, writeErr := fmt.Fprintln(w, "started"); writeErr != nil {
			return writeErr
		}
		<-ctx.Done()
		return ctx.Err()
	})

	stream := apiStreamSource(server)
	ch, cancel, err := stream(context.Background(), api.CallerIdentity{Username: "alice"}, command)
	require.NoError(t, err)

	line := <-ch
	assert.Equal(t, "started", line)

	cancel()
	<-handlerDone

	remaining := 0
	for range ch {
		remaining++
	}
	_ = remaining
}

// VALIDATES: Line writer buffers partial writes and emits complete lines.
// PREVENTS: split writes (e.g. from small bufio flushes) producing truncated SSE events.
func TestAPIStreamLineWriterPartialWrites(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	const command = "test api stream source partial"
	t.Cleanup(func() { pluginserver.UnregisterStreamingHandler(command) })
	pluginserver.RegisterStreamingHandler(command, func(_ context.Context, _ *pluginserver.Server, w io.Writer, _ string, _ []string) error {
		// Write "hello world\n" across three Write calls.
		if _, writeErr := w.Write([]byte("hel")); writeErr != nil {
			return writeErr
		}
		if _, writeErr := w.Write([]byte("lo wor")); writeErr != nil {
			return writeErr
		}
		_, writeErr := w.Write([]byte("ld\n"))
		return writeErr
	})

	stream := apiStreamSource(server)
	ch, cancel, err := stream(context.Background(), api.CallerIdentity{Username: "alice"}, command)
	require.NoError(t, err)
	defer cancel()

	var lines []string
	for line := range ch {
		lines = append(lines, line)
	}
	assert.Equal(t, []string{"hello world"}, lines)
}

// VALIDATES: Line writer flushes buffered content without trailing newline on close.
// PREVENTS: last line of handler output silently dropped.
func TestAPIStreamLineWriterFlushesOnClose(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	const command = "test api stream source notail"
	t.Cleanup(func() { pluginserver.UnregisterStreamingHandler(command) })
	pluginserver.RegisterStreamingHandler(command, func(_ context.Context, _ *pluginserver.Server, w io.Writer, _ string, _ []string) error {
		if _, writeErr := fmt.Fprintln(w, "line one"); writeErr != nil {
			return writeErr
		}
		_, writeErr := w.Write([]byte("no newline"))
		return writeErr
	})

	stream := apiStreamSource(server)
	ch, cancel, err := stream(context.Background(), api.CallerIdentity{Username: "alice"}, command)
	require.NoError(t, err)
	defer cancel()

	var lines []string
	for line := range ch {
		lines = append(lines, line)
	}
	assert.Equal(t, []string{"line one", "no newline"}, lines)
}

// VALIDATES: Panic in streaming handler is recovered and reported as an error.
// PREVENTS: process crash from a misbehaving streaming handler.
func TestAPIStreamSourceRecoversPanic(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	const command = "test api stream source panic"
	t.Cleanup(func() { pluginserver.UnregisterStreamingHandler(command) })
	pluginserver.RegisterStreamingHandler(command, func(context.Context, *pluginserver.Server, io.Writer, string, []string) error {
		panic("handler exploded")
	})

	stream := apiStreamSource(server)
	_, _, err = stream(context.Background(), api.CallerIdentity{Username: "alice"}, command)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "handler exploded")
}

type apiStreamTestAuthorizer struct {
	allow      bool
	username   string
	remoteAddr string
	command    string
	readOnly   bool
}

func (a *apiStreamTestAuthorizer) Authorize(username, remoteAddr, command string, isReadOnly bool) bool {
	a.username = username
	a.remoteAddr = remoteAddr
	a.command = command
	a.readOnly = isReadOnly
	return a.allow
}

type apiStreamTestAccountant struct {
	starts []string
	stops  []string
}

func (a *apiStreamTestAccountant) CommandStart(_, _, command string) string {
	a.starts = append(a.starts, command)
	return "task-1"
}

func (a *apiStreamTestAccountant) CommandStop(_, _, _, command string) {
	a.stops = append(a.stops, command)
}

// TestConfigValidationHookRunsFullValidation verifies API commits reject normal
// config validation errors before saving, not only plugin verifier errors.
//
// VALIDATES: API pre-save validation uses ze config validation semantics.
// PREVENTS: invalid non-plugin config being persisted before reload fails.
func TestConfigValidationHookRunsFullValidation(t *testing.T) {
	hook := configValidationHook("test.conf")
	err := hook(`bgp { router-id 1.2.3.4; }`, `bgp { router-id invalid; }`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config validation failed")
	assert.Contains(t, err.Error(), "router-id")
}
