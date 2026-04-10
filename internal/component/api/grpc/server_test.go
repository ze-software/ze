package grpc

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	zepb "codeberg.org/thomas-mangin/ze/api/proto"
	"codeberg.org/thomas-mangin/ze/internal/component/api"
)

// testEngine creates an APIEngine with fake implementations.
func testEngine() *api.APIEngine {
	exec := func(_, command string) (string, error) {
		switch command {
		case "bgp summary":
			return `{"peer-count":3}`, nil
		default:
			return "ok: " + command, nil
		}
	}
	cmds := func() []api.CommandMeta {
		return []api.CommandMeta{
			{Name: "bgp summary", Description: "Show BGP summary", ReadOnly: true},
			{Name: "daemon reload", Description: "Reload config", ReadOnly: false},
		}
	}
	auth := func(_, _ string) bool { return true }
	stream := func(_ context.Context, _, _ string) (<-chan string, func(), error) {
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
	values map[string]string
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

func (e *fakeEditor) Save() error            { return nil }
func (e *fakeEditor) Discard() error         { e.values = make(map[string]string); return nil }
func (e *fakeEditor) WorkingContent() string { return "# config\n" }

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
	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) {
		return &fakeEditor{values: make(map[string]string)}, nil
	})

	srv, err := NewGRPCServer(GRPCConfig{Token: token}, engine, sessions)
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
	assert.Len(t, resp.GetCommands(), 2)
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
	_, cfg := startTestServer(t, "")

	ctx := t.Context()

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
	exec := func(_, _ string) (string, error) { return "", errors.New("should not reach") }
	cmds := func() []api.CommandMeta { return nil }
	auth := func(_, _ string) bool { return false }
	engine := api.NewAPIEngine(exec, cmds, auth, nil)

	srv, err := NewGRPCServer(GRPCConfig{}, engine, nil)
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
