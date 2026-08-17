package hub

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/plugin"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// VALIDATES: AC-6, successful API local authentication publishes the zefs
// recovery profile before strict command authorization runs.
// PREVENTS: buildUserAuthenticator authenticating a power user but discarding
// AuthResult.Profiles, which leaves authz.Store unable to recognize recovery.
func TestBuildUserAuthenticatorRecordsRecoveryProfiles(t *testing.T) {
	const username = "api-recovery-user"
	aaa.ForgetLoginProfilesForTest(username)
	t.Cleanup(func() { aaa.ForgetLoginProfilesForTest(username) })

	hash, err := bcrypt.GenerateFromPassword([]byte("recovery-pass"), bcrypt.MinCost)
	require.NoError(t, err)
	users := []authz.UserConfig{{
		Name:     username,
		Hash:     string(hash),
		Profiles: []string{aaa.ReservedRecoveryProfile},
	}}
	authenticator := buildUserAuthenticator(users, func() ([]authz.UserConfig, error) {
		return users, nil
	})
	require.NotNil(t, authenticator)

	gotUser, ok := authenticator("Bearer " + username + ":recovery-pass")
	require.True(t, ok, "the credential must authenticate before profile publication is assessed")
	assert.Equal(t, username, gotUser)

	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "unrelated",
		Run:  authz.Section{Default: authz.Deny},
		Edit: authz.Section{Default: authz.Deny},
	})
	assert.Equal(t, authz.Allow, store.Authorize(username, "show version", true),
		"the recorded recovery profile must cross a strict authorization store")
}

// VALIDATES: the shared API engine translates a denial from the real command
// dispatcher into api.ErrUnauthorized for REST and gRPC status mapping.
// PREVENTS: dispatcher RBAC returning a canonical denial response with no Go
// error while the transport renders that response with HTTP/RPC success.
func TestBuildAPIEngineTranslatesDispatcherAuthorizationDenial(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	const command = "test api denied"
	ran := false
	server.Dispatcher().Register(command, func(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
		ran = true
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON("should not run")), nil
	}, command)
	authorizer := &apiStreamTestAuthorizer{allow: false}
	server.Dispatcher().SetAuthorizer(authorizer)

	engine := buildAPIEngine(server)
	result, err := engine.Execute(t.Context(), &api.ExecuteRequest{
		Caller: api.CallerIdentity{
			Username:   "alice",
			RemoteAddr: "198.51.100.10:4444",
		},
		Command: command,
	})

	require.ErrorIs(t, err, api.ErrUnauthorized)
	require.NotNil(t, result)
	assert.Equal(t, api.StatusError, result.Status)
	assert.False(t, ran, "authorization denial must stop dispatch")
	assert.Equal(t, "alice", authorizer.username)
	assert.Equal(t, "198.51.100.10:4444", authorizer.remoteAddr)
	assert.Equal(t, command, authorizer.command)
	assert.False(t, authorizer.readOnly)
}

// VALIDATES: a canonical dispatcher denial response with a nil Go error maps
// to api.ErrUnauthorized at the shared API boundary.
// PREVENTS: nested dispatcher paths returning HTTP/RPC success for denied
// commands after preserving only the response envelope.
func TestAPIDispatchErrorTranslatesNilErrorDenialResponse(t *testing.T) {
	const command = "show config dump"
	denied := &plugin.Response{
		Status: plugin.StatusError,
		Error:  plugin.UnauthorizedMessage + ": " + command,
	}

	assert.ErrorIs(t, apiDispatchError(denied, nil, command), api.ErrUnauthorized)
	assert.NoError(t, apiDispatchError(&plugin.Response{
		Status: plugin.StatusDone,
		Error:  denied.Error,
	}, nil, command), "a successful response must not be reclassified")
	assert.NoError(t, apiDispatchError(denied, nil, "show version"),
		"a denial for a different command must not be reclassified")
	assert.NoError(t, apiDispatchError(&plugin.Response{
		Status: plugin.StatusError,
		Error:  "unrelated failure",
	}, nil, command), "an ordinary error response must not be reclassified")
}

// VALIDATES: API execute wiring preserves request context and remote address into dispatcher context.
// PREVENTS: REST/gRPC metadata reaching APIEngine but being dropped before Dispatcher.Dispatch().
func TestAPIExecutorPropagatesRequestContextAndRemoteAddr(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	type ctxKey struct{}

	var seen *pluginserver.CommandContext
	server.Dispatcher().Register("test api", func(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
		seen = ctx
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"result": "ok"}}, nil
	}, "test api")

	exec := serverDispatcher(server, "")
	requestCtx := context.WithValue(context.Background(), ctxKey{}, "trace-id")

	// exec.JSON dispatches then flattens the typed response to the JSON string
	// text surfaces render -- the same path the two old hub adapters produced.
	output, err := exec.JSON(requestCtx, api.CallerIdentity{
		Username:   "alice",
		RemoteAddr: "198.51.100.10:4444",
	}, "test api")
	require.NoError(t, err)
	assert.Equal(t, `{"result":"ok"}`, output)

	require.NotNil(t, seen)
	assert.Equal(t, "alice", seen.Username)
	assert.Equal(t, "198.51.100.10:4444", seen.RemoteAddr)
	assert.Same(t, requestCtx, seen.Context())
	assert.Equal(t, "trace-id", seen.Context().Value(ctxKey{}))
}

// VALIDATES: serverDispatcher does NOT pin a text surface's dispatch to the
// never-canceled context.Background(); it leaves RequestContext nil so
// CommandContext.Context() falls back to the server context (which cancels on
// daemon shutdown), while a genuine per-request context (REST/gRPC) IS threaded.
// PREVENTS: in-flight web/mcp/lg/ssh/cli commands surviving daemon shutdown
// because Background never cancels (regression from the envelope unification).
func TestServerDispatcherContextThreading(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	var seen *pluginserver.CommandContext
	server.Dispatcher().Register("test ctx", func(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
		seen = ctx
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "test ctx")

	d := serverDispatcher(server, "web")

	// Text surface: passes Background -> RequestContext must stay nil so the
	// dispatch inherits the server context (cancels on shutdown).
	_, err = d(context.Background(), plugin.CallerIdentity{}, "test ctx")
	require.NoError(t, err)
	require.NotNil(t, seen)
	assert.Nil(t, seen.RequestContext, "context.Background() must not be threaded as the request context")

	// context.TODO() is the other never-canceling placeholder and must be
	// treated identically (server-context fallback, not a pinned request ctx).
	seen = nil
	_, err = d(context.TODO(), plugin.CallerIdentity{}, "test ctx")
	require.NoError(t, err)
	require.NotNil(t, seen)
	assert.Nil(t, seen.RequestContext, "context.TODO() must not be threaded as the request context")

	// API surface: a real per-request context is threaded through unchanged.
	seen = nil
	type ctxKey struct{}
	reqCtx := context.WithValue(context.Background(), ctxKey{}, "trace")
	_, err = d(reqCtx, plugin.CallerIdentity{}, "test ctx")
	require.NoError(t, err)
	require.NotNil(t, seen)
	assert.Same(t, reqCtx, seen.RequestContext, "a genuine per-request context must be threaded")
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
		if _, writeErr := fmt.Fprintln(w, "first"); writeErr != nil { //nolint:errcheck // output
			return writeErr
		}
		_, writeErr := fmt.Fprintln(w, "second") //nolint:errcheck // output
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
		if _, writeErr := fmt.Fprintln(w, "started"); writeErr != nil { //nolint:errcheck // output
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
		if _, writeErr := fmt.Fprintln(w, "line one"); writeErr != nil { //nolint:errcheck // output
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
// The fixture uses an always-on config root (interface), not bgp: BGP is
// compile-out-able (//go:build ze_bgp) and this test also runs in the bare
// ze_core pass, where a bgp{} block is correctly rejected as an unknown keyword
// -- which would make the test pass for the wrong reason, proving only that
// parsing failed rather than that validation ran.
func TestConfigValidationHookRunsFullValidation(t *testing.T) {
	hook := configValidationHook("test.conf")
	const good = `interface { ethernet eth0 { unit 0 { ipv4 { address [ 192.0.2.1/24 ]; } } } }`
	const bad = `interface { ethernet eth0 { unit 0 { ipv4 { address [ not-an-address ]; } } } }`
	err := hook(good, bad)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config validation failed")
	assert.Contains(t, err.Error(), "address")
}
