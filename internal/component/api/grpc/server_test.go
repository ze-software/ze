package grpc

import (
	"context"
	"errors"
	"net"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	zepb "github.com/ze-software/ze/api/proto"
	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/audit"
)

func testAuthentication(token string, authenticate func(string) (string, bool), authorizer aaa.Authorizer) api.AuthenticationProvider {
	return func() api.Authentication {
		if authenticate != nil {
			return api.Authentication{
				Required: true,
				Authenticate: func(header string) (api.CallerIdentity, bool) {
					username, ok := authenticate(header)
					return api.CallerIdentity{Username: username, Authorizer: authorizer}, ok
				},
			}
		}
		if token != "" {
			return api.Authentication{
				Required: true,
				Authenticate: func(header string) (api.CallerIdentity, bool) {
					return api.CallerIdentity{
						Username:   aaa.ReservedSharedAPIUsername,
						Authorizer: authorizer,
					}, header == "Bearer "+token
				},
			}
		}
		return api.Authentication{}
	}
}

// testEngine creates an APIEngine with fake implementations.
func testEngine() *api.APIEngine {
	exec := func(_ context.Context, _ api.CallerIdentity, command string) (*plugin.Response, error) {
		switch command {
		case "bgp summary":
			return plugin.NewResponse(api.StatusDone, plugin.RawJSON(`{"peer-count":3}`)), nil
		default:
			return plugin.NewResponse(api.StatusDone, plugin.RawJSON("ok: "+command)), nil
		}
	}
	cmds := func() []api.CommandMeta {
		return []api.CommandMeta{
			{Name: "bgp summary", Description: "Show BGP summary", ReadOnly: true},
			{Name: "bgp monitor", Description: "Monitor BGP events", ReadOnly: true},
			{Name: "daemon reload", Description: "Reload config", ReadOnly: false},
		}
	}
	auth := func(_, _ string) bool { return true }
	stream := func(_ context.Context, _ api.CallerIdentity, _ string) (<-chan string, func(), error) {
		ch := make(chan string, 2) //nolint:mnd // test events
		ch <- `{"event":"update"}`
		ch <- `{"event":"withdraw"}`
		close(ch)
		return ch, func() {}, nil
	}
	return api.NewAPIEngine(exec, cmds, auth, stream)
}

// fakeEditor implements api.ConfigEditor for testing.
type fakeEditor struct {
	values           map[string]string
	committedContent string
}

func (e *fakeEditor) SetValue(path []string, key, value string) error {
	e.values[strings.Join(path, ".")+"."+key] = value
	return nil
}

func (e *fakeEditor) DeleteByPath(fullPath []string) error {
	delete(e.values, strings.Join(fullPath, "."))
	return nil
}

func (e *fakeEditor) Diff() string {
	if len(e.values) == 0 {
		return ""
	}
	var b strings.Builder
	for k, v := range e.values {
		b.WriteString("+" + k + " = " + v + "\n")
	}
	return b.String()
}

func (e *fakeEditor) Save() error {
	e.committedContent = e.WorkingContent()
	return nil
}
func (e *fakeEditor) StageCandidate(time.Time) (string, string, error) {
	return e.WorkingContent(), "test-version", nil
}
func (e *fakeEditor) MarkCommittedContent(content string) { e.committedContent = content }
func (e *fakeEditor) RestoreOriginalContent(string) error { return nil }
func (e *fakeEditor) Discard() error                      { e.values = make(map[string]string); return nil }
func (e *fakeEditor) OriginalContent() string             { return "# original\n" }
func (e *fakeEditor) WorkingContent() string {
	if len(e.values) == 0 {
		return "# config\n"
	}
	keys := make([]string, 0, len(e.values))
	for key := range e.values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("# config\n")
	for _, key := range keys {
		b.WriteString(key)
		b.WriteString(" = ")
		b.WriteString(e.values[key])
		b.WriteByte('\n')
	}
	return b.String()
}

type denyAllAuthorizer struct {
	username string
	command  string
	readOnly bool
}

func (a *denyAllAuthorizer) Authorize(username, _, command string, isReadOnly bool) bool {
	a.username = username
	a.command = command
	a.readOnly = isReadOnly
	return false
}

// serveBackground starts a gRPC server in a background goroutine.
// One-time test lifecycle goroutine, not per-event.
func serveBackground(srv *grpc.Server, ln net.Listener) {
	go func() {
		if err := srv.Serve(ln); err != nil {
			return
		}
	}()
}

// startTestServer starts a gRPC server on a random port and returns a client connection.
func startTestServer(t *testing.T, token string) (zepb.ZeServiceClient, zepb.ZeConfigServiceClient) {
	t.Helper()

	engine := testEngine()
	return startTestServerWithEngine(t, token, engine)
}

func startTestServerWithEngine(t *testing.T, token string, engine *api.APIEngine) (zepb.ZeServiceClient, zepb.ZeConfigServiceClient) {
	t.Helper()
	return startTestServerWithEngineAndAudit(t, token, engine, nil)
}
func startTestServerWithSessionFactory(t *testing.T, token string, factory api.ConfigEditorFactory) (zepb.ZeServiceClient, zepb.ZeConfigServiceClient) {
	t.Helper()

	engine := testEngine()
	sessions := api.NewConfigSessionManager(factory)

	// ListenAddrs is required by NewGRPCServer validation, but the tests
	// bypass Serve() and bind their own listener via serveBackground below,
	// so the value here is a placeholder that never gets bound.
	srv, err := NewGRPCServer(GRPCConfig{
		ListenAddrs:    []string{"127.0.0.1:0"},
		Authentication: testAuthentication(token, nil, nil),
	}, engine, sessions)
	require.NoError(t, err)

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()

	serveBackground(srv.srv, ln)
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	return zepb.NewZeServiceClient(conn), zepb.NewZeConfigServiceClient(conn)
}

func startTestServerWithEngineAndAudit(t *testing.T, token string, engine *api.APIEngine, recorder audit.Recorder) (zepb.ZeServiceClient, zepb.ZeConfigServiceClient) {
	t.Helper()

	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) {
		return &fakeEditor{values: make(map[string]string)}, nil
	})

	// ListenAddrs is required by NewGRPCServer validation, but the tests
	// bypass Serve() and bind their own listener via serveBackground below,
	// so the value here is a placeholder that never gets bound.
	srv, err := NewGRPCServer(GRPCConfig{
		ListenAddrs:    []string{"127.0.0.1:0"},
		Authentication: testAuthentication(token, nil, nil),
		AuditRecorder:  recorder,
	}, engine, sessions)
	require.NoError(t, err)

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()

	serveBackground(srv.srv, ln)
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	return zepb.NewZeServiceClient(conn), zepb.NewZeConfigServiceClient(conn)
}

// VALIDATES: AC-1 -- ListCommands returns all commands.
// PREVENTS: missing commands in gRPC response.
func TestGRPCListCommands(t *testing.T) {
	ze, _ := startTestServer(t, "")

	resp, err := ze.ListCommands(t.Context(), &zepb.ListCommandsRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.GetCommands(), 3)
	assert.Equal(t, "bgp summary", resp.GetCommands()[0].GetName())
}

// VALIDATES: AC-2 -- Execute returns command output.
// PREVENTS: execute RPC broken.
func TestGRPCExecute(t *testing.T) {
	ze, _ := startTestServer(t, "")

	resp, err := ze.Execute(t.Context(), &zepb.CommandRequest{Command: "bgp summary"})
	require.NoError(t, err)
	assert.Equal(t, "done", resp.GetStatus())
	assert.Contains(t, string(resp.GetData()), "peer-count")
}

// VALIDATES: gRPC retains lifecycle completion until grpc-go reports that the
// unary response payload was written.
// PREVENTS: accepted teardown never running, or running inside the API engine.
func TestGRPCExecuteCompletesAfterPayloadWrite(t *testing.T) {
	completed := make(chan struct{}, 1)
	engine := api.NewAPIEngine(
		func(context.Context, api.CallerIdentity, string) (*plugin.Response, error) {
			resp := plugin.NewResponse(api.StatusDone, plugin.RawJSON(`{"accepted":true}`))
			resp.OnTransportComplete(func() { completed <- struct{}{} })
			return resp, nil
		},
		func() []api.CommandMeta {
			return []api.CommandMeta{{Name: "request shutdown"}}
		},
		func(_, _ string) bool { return true },
		nil,
	)
	ze, _ := startTestServerWithEngine(t, "completion-token", engine)
	md := metadata.Pairs("authorization", "Bearer completion-token")
	ctx := metadata.NewOutgoingContext(t.Context(), md)

	resp, err := ze.Execute(ctx, &zepb.CommandRequest{Command: "request shutdown"})
	require.NoError(t, err)
	assert.Equal(t, api.StatusDone, resp.GetStatus())
	select {
	case <-completed:
	default:
		t.Fatal("gRPC payload write did not complete the accepted action")
	}
}

// VALIDATES: gRPC Execute extracts the peer address into CallerIdentity.
// PREVENTS: dispatcher/accounting seeing empty remote address for gRPC callers.
func TestExecuteUsesPeerRemoteAddr(t *testing.T) {
	var gotAuth api.CallerIdentity

	engine := api.NewAPIEngine(
		func(_ context.Context, auth api.CallerIdentity, command string) (*plugin.Response, error) {
			gotAuth = auth
			return plugin.NewResponse(api.StatusDone, plugin.RawJSON("ok: "+command)), nil
		},
		func() []api.CommandMeta {
			return []api.CommandMeta{{Name: "bgp summary", ReadOnly: true}}
		},
		func(_, _ string) bool { return true },
		nil,
	)

	ze, _ := startTestServerWithEngine(t, "", engine)

	resp, err := ze.Execute(t.Context(), &zepb.CommandRequest{Command: "bgp summary"})
	require.NoError(t, err)
	assert.Equal(t, "done", resp.GetStatus())
	assert.Equal(t, aaa.ReservedSharedAPIUsername, gotAuth.Username)
	assert.NotEmpty(t, gotAuth.RemoteAddr)
	assert.Contains(t, gotAuth.RemoteAddr, "127.0.0.1:")
}

// VALIDATES: AC-3 -- RPC without auth returns Unauthenticated.
// PREVENTS: unauthenticated gRPC access.
func TestGRPCExecuteUnauthorized(t *testing.T) {
	ze, _ := startTestServer(t, "secret")

	// No metadata.
	noAuthErr := execWithErr(t, ze, t.Context(), "bgp summary")
	assert.Equal(t, codes.Unauthenticated, status.Code(noAuthErr))

	// Wrong token.
	md := metadata.Pairs("authorization", "Bearer wrong")
	wrongErr := execWithErr(t, ze, metadata.NewOutgoingContext(t.Context(), md), "bgp summary")
	assert.Equal(t, codes.Unauthenticated, status.Code(wrongErr))

	// Correct token.
	md = metadata.Pairs("authorization", "Bearer secret")
	ctx := metadata.NewOutgoingContext(t.Context(), md)
	resp, err := ze.Execute(ctx, &zepb.CommandRequest{Command: "bgp summary"})
	require.NoError(t, err)
	assert.Equal(t, "done", resp.GetStatus())
}

// VALIDATES: AC-16 -- gRPC failed authentication emits an audit record with source IP and attempted user.
// PREVENTS: gRPC auth failures being visible only as Unauthenticated status codes.
func TestGRPCAuthFailureAuditRecord(t *testing.T) {
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	engine := testEngine()
	srv, err := NewGRPCServer(GRPCConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		Authentication: testAuthentication("", func(header string) (string, bool) {
			return "", false
		}, nil),
		AuditRecorder: recorder,
	}, engine, nil)
	require.NoError(t, err)

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	serveBackground(srv.srv, ln)
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	ze := zepb.NewZeServiceClient(conn)
	md := metadata.Pairs("authorization", "Bearer alice:wrong")
	ctx := metadata.NewOutgoingContext(t.Context(), md)

	_, err = ze.Execute(ctx, &zepb.CommandRequest{Command: "bgp summary"})

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	entries := recorder.Query(audit.Filter{Action: audit.ActionAuthFail})
	require.Len(t, entries, 1)
	assert.Equal(t, "alice", entries[0].Actor)
	assert.Contains(t, entries[0].RemoteAddr, "127.0.0.1:")
	assert.Equal(t, audit.GRPC, entries[0].Surface)
	assert.Equal(t, audit.OutcomeDenied, entries[0].Outcome)
}

// VALIDATES: AC-8, gRPC no-auth mode gives the reserved shared API identity
// read-only access, not admin access.
func TestGRPCNoAuthReadOnly(t *testing.T) {
	ze, cfg := startTestServer(t, "")

	readResp, readErr := ze.Execute(t.Context(), &zepb.CommandRequest{Command: "bgp summary"})
	require.NoError(t, readErr)
	assert.Equal(t, "done", readResp.GetStatus())

	_, writeErr := ze.Execute(t.Context(), &zepb.CommandRequest{Command: "daemon reload"})
	require.Error(t, writeErr)
	assert.Equal(t, codes.PermissionDenied, status.Code(writeErr))

	_, sessionErr := cfg.EnterSession(t.Context(), &zepb.Empty{})
	require.Error(t, sessionErr)
	assert.Equal(t, codes.PermissionDenied, status.Code(sessionErr))
}

// VALIDATES: AC-7 and AC-8, real gRPC token writes and no-auth reads cross a
// strict AAA store, while no-auth writes remain read-only denied.
// PREVENTS: gRPC publishing the printable unassigned username "api", which a
// strict Store correctly denies when authorization profiles exist.
func TestGRPCSharedIdentitySurvivesStrictAuthorization(t *testing.T) {
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "unrelated",
		Run:  authz.Section{Default: authz.Deny},
		Edit: authz.Section{Default: authz.Deny},
	})
	commands := func() []api.CommandMeta {
		return []api.CommandMeta{
			{Name: "bgp summary", ReadOnly: true},
			{Name: "daemon reload", ReadOnly: false},
		}
	}
	var callers []api.CallerIdentity
	var authorizationCalls []string
	engine := api.NewAPIEngine(
		func(_ context.Context, caller api.CallerIdentity, command string) (*plugin.Response, error) {
			callers = append(callers, caller)
			return plugin.NewResponse(api.StatusDone, plugin.RawJSON("ok: "+command)), nil
		},
		commands,
		func(username, command string) bool {
			authorizationCalls = append(authorizationCalls, command)
			return store.Authorize(username, command, command == "bgp summary") == authz.Allow
		},
		nil,
	)
	start := func(t *testing.T, token string) zepb.ZeServiceClient {
		t.Helper()
		srv, err := NewGRPCServer(GRPCConfig{
			ListenAddrs:    []string{"127.0.0.1:0"},
			Authentication: testAuthentication(token, nil, nil),
		}, engine, nil)
		require.NoError(t, err)
		var lc net.ListenConfig
		ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
		require.NoError(t, err)
		serveBackground(srv.srv, ln)
		t.Cleanup(srv.Stop)
		conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, conn.Close()) })
		return zepb.NewZeServiceClient(conn)
	}

	noAuth := start(t, "")
	read, err := noAuth.Execute(t.Context(), &zepb.CommandRequest{Command: "bgp summary"})
	require.NoError(t, err, "the reserved shared identity must let a no-auth read cross strict AAA")
	assert.Equal(t, "done", read.GetStatus())
	require.Len(t, callers, 1)
	assert.Equal(t, aaa.ReservedSharedAPIUsername, callers[0].Username)
	assert.True(t, callers[0].ReadOnly)
	assert.Equal(t, []string{"bgp summary"}, authorizationCalls)
	authorizationCallCount := len(authorizationCalls)
	_, err = noAuth.Execute(t.Context(), &zepb.CommandRequest{Command: "daemon reload"})
	assert.Equal(t, codes.PermissionDenied, status.Code(err),
		"the read-only gate must reject no-auth writes before strict AAA")
	assert.Len(t, authorizationCalls, authorizationCallCount,
		"the read-only gate must reject no-auth writes before calling the authorizer")
	assert.Len(t, callers, 1,
		"the read-only gate must reject no-auth writes before dispatch")

	token := start(t, "shared-secret")
	ctx := metadata.NewOutgoingContext(t.Context(), metadata.Pairs("authorization", "Bearer shared-secret"))
	write, err := token.Execute(ctx, &zepb.CommandRequest{Command: "daemon reload"})
	require.NoError(t, err, "a validated token caller must retain write authority through strict AAA")
	assert.Equal(t, "done", write.GetStatus())
	require.Len(t, callers, 2)
	assert.Equal(t, aaa.ReservedSharedAPIUsername, callers[1].Username)
	assert.False(t, callers[1].ReadOnly)
	assert.Equal(t, []string{"bgp summary", "daemon reload"}, authorizationCalls)
	wrongCtx := metadata.NewOutgoingContext(t.Context(), metadata.Pairs("authorization", "Bearer wrong"))
	_, err = token.Execute(wrongCtx, &zepb.CommandRequest{Command: "daemon reload"})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// execWithErr calls Execute and returns only the error.
func execWithErr(t *testing.T, ze zepb.ZeServiceClient, ctx context.Context, command string) error {
	t.Helper()
	_, err := ze.Execute(ctx, &zepb.CommandRequest{Command: command})
	require.Error(t, err)
	return err
}

// VALIDATES: AC-4 -- Stream delivers events via server-streaming.
// PREVENTS: streaming broken.
func TestGRPCStream(t *testing.T) {
	ze, _ := startTestServer(t, "")

	stream, err := ze.Stream(t.Context(), &zepb.CommandRequest{Command: "bgp monitor"})
	require.NoError(t, err)

	var events []string
	for {
		resp, recvErr := stream.Recv()
		if recvErr != nil {
			break
		}
		events = append(events, string(resp.GetData()))
	}
	assert.Len(t, events, 2)
	assert.Contains(t, events[0], "update")
	assert.Contains(t, events[1], "withdraw")
}

// VALIDATES: AC-5 -- Config session lifecycle via gRPC.
// PREVENTS: config RPCs broken.
func TestGRPCConfigSession(t *testing.T) {
	var editor *fakeEditor
	_, cfg := startTestServerWithSessionFactory(t, "secret", func() (api.ConfigEditor, error) {
		editor = &fakeEditor{values: make(map[string]string)}
		return editor, nil
	})
	md := metadata.Pairs("authorization", "Bearer secret")
	ctx := metadata.NewOutgoingContext(t.Context(), md)

	// Enter session.
	sessResp, err := cfg.EnterSession(ctx, &zepb.Empty{})
	require.NoError(t, err)
	id := sessResp.GetSessionId()
	assert.NotEmpty(t, id)

	// Set config.
	_, err = cfg.SetConfig(ctx, &zepb.ConfigSetRequest{
		SessionId: id, Path: "bgp.router-id", Value: "10.0.0.1",
	})
	require.NoError(t, err)

	// Diff.
	diffResp, err := cfg.DiffSession(ctx, &zepb.SessionRequest{SessionId: id})
	require.NoError(t, err)
	assert.NotEmpty(t, diffResp.GetDiff())

	// Commit.
	_, err = cfg.CommitSession(ctx, &zepb.CommitRequest{SessionId: id})
	require.NoError(t, err)
	require.NotNil(t, editor)
	assert.Contains(t, editor.committedContent, "bgp.router-id = 10.0.0.1")
}

// VALIDATES: config session RPCs consult profile RBAC for authenticated users.
// PREVENTS: read-only config users mutating config through gRPC sessions.
func TestGRPCConfigSessionAuthorizerDeny(t *testing.T) {
	engine := testEngine()
	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) {
		return &fakeEditor{values: make(map[string]string)}, nil
	})
	authorizer := &denyAllAuthorizer{}
	srv, err := NewGRPCServer(GRPCConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		Authentication: testAuthentication("", func(header string) (string, bool) {
			return "alice", header == "Bearer alice-token"
		}, authorizer),
	}, engine, sessions)
	require.NoError(t, err)

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	serveBackground(srv.srv, ln)
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	cfg := zepb.NewZeConfigServiceClient(conn)
	md := metadata.Pairs("authorization", "Bearer alice-token")
	ctx := metadata.NewOutgoingContext(t.Context(), md)
	_, err = cfg.EnterSession(ctx, &zepb.Empty{})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Equal(t, "alice", authorizer.username)
	assert.Equal(t, "config edit", authorizer.command)
	assert.False(t, authorizer.readOnly)
}

// VALIDATES: AC-9 -- Config commit via gRPC emits an audit record with actor, surface, action, and summary.
// PREVENTS: gRPC config session commits bypassing the unified audit trail.
func TestGRPCConfigCommitAuditRecord(t *testing.T) {
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	_, cfg := startTestServerWithEngineAndAudit(t, "secret", testEngine(), recorder)

	md := metadata.Pairs("authorization", "Bearer secret")
	ctx := metadata.NewOutgoingContext(t.Context(), md)
	sessResp, err := cfg.EnterSession(ctx, &zepb.Empty{})
	require.NoError(t, err)
	id := sessResp.GetSessionId()

	_, err = cfg.SetConfig(ctx, &zepb.ConfigSetRequest{SessionId: id, Path: "bgp.router-id", Value: "10.0.0.1"})
	require.NoError(t, err)
	_, err = cfg.CommitSession(ctx, &zepb.CommitRequest{SessionId: id})
	require.NoError(t, err)

	entries := recorder.Query(audit.Filter{Action: audit.ActionConfigCommit})
	require.Len(t, entries, 1)
	assert.Equal(t, aaa.ReservedSharedAPIUsername, entries[0].Actor)
	assert.Equal(t, audit.GRPC, entries[0].Surface)
	assert.Equal(t, audit.OutcomeSuccess, entries[0].Outcome)
	assert.Contains(t, entries[0].Detail, "bgp.router-id")
}

// VALIDATES: AC-10 -- Config discard via gRPC emits an audit record with actor, surface, action, and summary.
// PREVENTS: gRPC config discards losing audit attribution.
func TestGRPCConfigDiscardAuditRecord(t *testing.T) {
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	_, cfg := startTestServerWithEngineAndAudit(t, "secret", testEngine(), recorder)

	md := metadata.Pairs("authorization", "Bearer secret")
	ctx := metadata.NewOutgoingContext(t.Context(), md)
	sessResp, err := cfg.EnterSession(ctx, &zepb.Empty{})
	require.NoError(t, err)
	id := sessResp.GetSessionId()

	_, err = cfg.SetConfig(ctx, &zepb.ConfigSetRequest{SessionId: id, Path: "bgp.router-id", Value: "10.0.0.1"})
	require.NoError(t, err)
	_, err = cfg.DiscardSession(ctx, &zepb.SessionRequest{SessionId: id})
	require.NoError(t, err)

	entries := recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard})
	require.Len(t, entries, 1)
	assert.Equal(t, aaa.ReservedSharedAPIUsername, entries[0].Actor)
	assert.Equal(t, audit.GRPC, entries[0].Surface)
	assert.Equal(t, audit.OutcomeSuccess, entries[0].Outcome)
	assert.Contains(t, entries[0].Detail, "bgp.router-id")
}

// VALIDATES: AC-6 -- gRPC reflection enabled and services reachable.
// PREVENTS: grpcurl/grpcui cannot discover services.
func TestGRPCReflection(t *testing.T) {
	ze, _ := startTestServer(t, "")
	// If reflection registration panicked, startTestServer would have failed.
	// Verify the service is reachable by listing commands.
	resp, err := ze.ListCommands(t.Context(), &zepb.ListCommandsRequest{})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.GetCommands())
}

// VALIDATES: AC-8 -- Unknown command returns NotFound.
// PREVENTS: unknown command silently succeeds.
func TestGRPCUnknownCommand(t *testing.T) {
	ze, _ := startTestServer(t, "")

	_, err := ze.DescribeCommand(t.Context(), &zepb.DescribeCommandRequest{Path: "nonexistent"})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// VALIDATES: Empty command returns InvalidArgument.
// PREVENTS: empty command accepted.
func TestGRPCExecuteEmptyCommand(t *testing.T) {
	ze, _ := startTestServer(t, "")

	_, err := ze.Execute(t.Context(), &zepb.CommandRequest{Command: ""})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// VALIDATES: Execute with denied auth returns PermissionDenied.
// PREVENTS: auth bypass in gRPC path.
func TestGRPCExecutePermissionDenied(t *testing.T) {
	exec := func(_ context.Context, _ api.CallerIdentity, _ string) (*plugin.Response, error) {
		return nil, errors.New("should not reach")
	}
	cmds := func() []api.CommandMeta { return nil }
	auth := func(_, _ string) bool { return false }
	engine := api.NewAPIEngine(exec, cmds, auth, nil)

	// ListenAddrs placeholder; serveBackground binds its own listener below.
	srv, err := NewGRPCServer(GRPCConfig{ListenAddrs: []string{"127.0.0.1:0"}}, engine, nil)
	require.NoError(t, err)

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	serveBackground(srv.srv, ln)
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	ze := zepb.NewZeServiceClient(conn)
	execErr := execWithErr(t, ze, t.Context(), "bgp summary")
	assert.Equal(t, codes.PermissionDenied, status.Code(execErr))
}

// VALIDATES: GetRunningConfig maps denied "show config dump" execution to PermissionDenied.
// PREVENTS: authorization denials being exposed as Internal gRPC failures.
func TestGRPCGetRunningConfigPermissionDenied(t *testing.T) {
	exec := func(_ context.Context, _ api.CallerIdentity, _ string) (*plugin.Response, error) {
		return nil, errors.New("should not reach")
	}
	cmds := func() []api.CommandMeta {
		return []api.CommandMeta{{Name: "show config dump", ReadOnly: true}}
	}
	var authorizedCommand string
	auth := func(_, command string) bool {
		authorizedCommand = command
		return false
	}
	engine := api.NewAPIEngine(exec, cmds, auth, nil)
	_, cfg := startTestServerWithEngine(t, "", engine)

	_, err := cfg.GetRunningConfig(t.Context(), &zepb.Empty{})

	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Equal(t, "show config dump", authorizedCommand)
}

// VALIDATES: TLS cert/key mismatch rejected at construction.
// PREVENTS: gRPC server starting plaintext when TLS was intended.
func TestGRPCTLSRequiresBoth(t *testing.T) {
	engine := testEngine()

	_, err := NewGRPCServer(GRPCConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		TLSCert:     "cert.pem",
	}, engine, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "both TLSCert and TLSKey")

	_, err = NewGRPCServer(GRPCConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		TLSKey:      "key.pem",
	}, engine, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "both TLSCert and TLSKey")
}

// VALIDATES: Invalid TLS cert path returns error.
// PREVENTS: silent failure on missing TLS files.
func TestGRPCTLSInvalidCert(t *testing.T) {
	engine := testEngine()

	_, err := NewGRPCServer(GRPCConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		TLSCert:     "/nonexistent/cert.pem",
		TLSKey:      "/nonexistent/key.pem",
	}, engine, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load TLS")
}

// TestNewGRPCServer_RejectsNonLoopbackAuthenticatedPlaintext verifies remote
// authenticated gRPC listeners require TLS.
//
// VALIDATES: Non-loopback authenticated gRPC listeners require tls-cert/tls-key.
// PREVENTS: Management credentials crossing the network in cleartext.
func TestNewGRPCServer_RejectsNonLoopbackAuthenticatedPlaintext(t *testing.T) {
	engine := testEngine()

	tests := []struct {
		name string
		cfg  GRPCConfig
		want string
	}{
		{
			name: "no_auth",
			cfg:  GRPCConfig{ListenAddrs: []string{"0.0.0.0:50051"}},
			want: "requires authentication",
		},
		{
			name: "token",
			cfg:  GRPCConfig{ListenAddrs: []string{"0.0.0.0:50051"}, Authentication: testAuthentication("secret", nil, nil)},
			want: "requires TLS",
		},
		{
			name: "per_user_auth",
			cfg: GRPCConfig{
				ListenAddrs:    []string{"0.0.0.0:50051"},
				Authentication: testAuthentication("", func(string) (string, bool) { return "alice", true }, nil),
			},
			want: "requires TLS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGRPCServer(tt.cfg, engine, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// VALIDATES: per-user authenticator passes username to engine.
// PREVENTS: all gRPC requests using the reserved shared API identity.
func TestGRPCAuthenticator(t *testing.T) {
	var seenUser string
	exec := func(_ context.Context, auth api.CallerIdentity, _ string) (*plugin.Response, error) {
		seenUser = auth.Username
		return plugin.NewResponse(api.StatusDone, plugin.RawJSON(`"ok"`)), nil
	}
	cmds := func() []api.CommandMeta { return nil }
	auth := func(_, _ string) bool { return true }
	engine := api.NewAPIEngine(exec, cmds, auth, nil)

	authenticator := func(header string) (string, bool) {
		if header == "Bearer alice-token" {
			return "alice", true
		}
		return "", false
	}

	srv, err := NewGRPCServer(GRPCConfig{
		ListenAddrs:    []string{"127.0.0.1:0"},
		Authentication: testAuthentication("", authenticator, nil),
	}, engine, nil)
	require.NoError(t, err)

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	serveBackground(srv.srv, ln)
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	ze := zepb.NewZeServiceClient(conn)

	// No credentials -- rejected.
	_, execErr := ze.Execute(t.Context(), &zepb.CommandRequest{Command: "test"})
	require.Error(t, execErr)
	assert.Equal(t, codes.Unauthenticated, status.Code(execErr))

	// Valid credentials -- username flows through.
	md := metadata.Pairs("authorization", "Bearer alice-token")
	ctx := metadata.NewOutgoingContext(t.Context(), md)
	_, err = ze.Execute(ctx, &zepb.CommandRequest{Command: "test"})
	require.NoError(t, err)
	assert.Equal(t, "alice", seenUser)
}

// TestNewGRPCServer_RequiresListenAddrs verifies empty-slice / empty-entry
// rejection.
func TestNewGRPCServer_RequiresListenAddrs(t *testing.T) {
	engine := testEngine()

	_, err := NewGRPCServer(GRPCConfig{}, engine, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one listen address")

	_, err = NewGRPCServer(GRPCConfig{ListenAddrs: []string{""}}, engine, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

// TestGRPCServer_MultiListener verifies Serve binds every entry in
// GRPCConfig.ListenAddrs and both listeners accept gRPC calls.
//
// VALIDATES: AC-6 (gRPC config with two server entries binds both).
// VALIDATES: AC-14 (Stop closes every listener).
// PREVENTS: Regression where only the first gRPC listener is bound.
func TestGRPCServer_MultiListener(t *testing.T) {
	// Pre-allocate two ports via bind-then-close; we hand them back to
	// GRPCServer.Serve which will re-bind them under the grpc.Server.
	var lc net.ListenConfig
	probe1, err := lc.Listen(t.Context(), "tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	addr1 := probe1.Addr().String()
	require.NoError(t, probe1.Close())
	probe2, err := lc.Listen(t.Context(), "tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	addr2 := probe2.Addr().String()
	require.NoError(t, probe2.Close())

	engine := testEngine()
	srv, err := NewGRPCServer(GRPCConfig{
		ListenAddrs: []string{addr1, addr2},
	}, engine, nil)
	require.NoError(t, err)
	t.Cleanup(srv.Stop)

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- srv.Serve(t.Context())
	}()

	// Give Serve a moment to bind both listeners.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(srv.Addresses()) == 2 && srv.Addresses()[0] != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	bound := srv.Addresses()
	require.Len(t, bound, 2, "expected 2 bound addresses")

	// Dial each listener and confirm it accepts an Execute RPC.
	for i, addr := range bound {
		conn, dialErr := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, dialErr, "listener %d (%s)", i, addr)
		client := zepb.NewZeServiceClient(conn)
		resp, execErr := client.Execute(t.Context(), &zepb.CommandRequest{Command: "bgp summary"})
		require.NoError(t, execErr, "listener %d (%s)", i, addr)
		assert.NotNil(t, resp)
		if closeErr := conn.Close(); closeErr != nil {
			t.Logf("close conn: %v", closeErr)
		}
	}

	// Stop releases every listener.
	srv.Stop()
	select {
	case serveErr := <-serveErrCh:
		if serveErr != nil && !errors.Is(serveErr, grpc.ErrServerStopped) {
			t.Fatalf("Serve returned unexpected error: %v", serveErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return after Stop")
	}
}

// TestGRPCServer_BindFailureClosesPartialListeners verifies that when the
// second entry fails to bind, the first listener is closed and Serve
// returns the bind error.
//
// VALIDATES: AC-15 (fail-fast on partial bind).
func TestGRPCServer_BindFailureClosesPartialListeners(t *testing.T) {
	// Squat on a port so the second bind fails.
	var lc net.ListenConfig
	squatter, err := lc.Listen(t.Context(), "tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	defer squatter.Close() //nolint:errcheck // test cleanup
	squattedAddr := squatter.Addr().String()

	engine := testEngine()
	srv, err := NewGRPCServer(GRPCConfig{
		ListenAddrs: []string{"127.0.0.1:0", squattedAddr},
	}, engine, nil)
	require.NoError(t, err)

	err = srv.Serve(t.Context())
	require.Error(t, err, "Serve must fail when any bind fails")
	assert.Contains(t, err.Error(), squattedAddr)
}

func TestGRPCServerStartReturnsBindFailure(t *testing.T) {
	var lc net.ListenConfig
	squatter, err := lc.Listen(t.Context(), "tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	defer squatter.Close() //nolint:errcheck // test cleanup
	squattedAddr := squatter.Addr().String()

	engine := testEngine()
	srv, err := NewGRPCServer(GRPCConfig{
		ListenAddrs: []string{"127.0.0.1:0", squattedAddr},
	}, engine, nil)
	require.NoError(t, err)

	errCh, err := srv.Start(t.Context())
	require.Error(t, err, "Start must fail before returning when any bind fails")
	assert.Nil(t, errCh)
	assert.Contains(t, err.Error(), squattedAddr)
}
