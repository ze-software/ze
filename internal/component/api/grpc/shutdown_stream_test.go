// Design: docs/architecture/api/architecture.md -- shutdown cancels an open gRPC stream before joining requests.
package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	zepb "github.com/ze-software/ze/api/proto"
	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestGRPCStopCancelsOpenStream reaches the actual server-streaming RPC with
// an open provider. Stop must cancel that RPC before the client deadline;
// waiting for the client to cancel would leave the hub stuck in GracefulStop.
func TestGRPCStopCancelsOpenStream(t *testing.T) {
	engine := api.NewAPIEngine(
		func(context.Context, api.CallerIdentity, string) (*plugin.Response, error) {
			return plugin.NewResponse(api.StatusDone, nil), nil
		},
		func() ([]api.CommandMeta, error) {
			return []api.CommandMeta{{Name: "bgp monitor", ReadOnly: true}}, nil
		}, nil,
		func(context.Context, api.CallerIdentity, string) (<-chan string, func(), error) {
			events := make(chan string, 1)
			events <- "started"
			return events, func() { close(events) }, nil
		},
	)
	srv, err := NewGRPCServer(GRPCConfig{ListenAddrs: []string{"127.0.0.1:0"}}, engine, nil)
	require.NoError(t, err)
	serveErrors, err := srv.Start(t.Context())
	require.NoError(t, err)
	t.Cleanup(srv.Stop)
	connection, err := grpc.NewClient(srv.Addresses()[0], grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	t.Cleanup(cancel)
	stream, err := zepb.NewZeServiceClient(connection).Stream(ctx, &zepb.CommandRequest{Command: "bgp monitor"})
	require.NoError(t, err)
	first, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, "started", string(first.GetData()))
	finished := make(chan error, 1)
	go func() {
		_, recvErr := stream.Recv()
		finished <- recvErr
	}()
	srv.Stop()
	require.Equal(t, codes.Canceled, status.Code(<-finished), "Stop must cancel its RPC before the client deadline")
	require.NoError(t, <-serveErrors)
}
