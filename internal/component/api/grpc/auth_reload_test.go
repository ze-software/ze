package grpc

import (
	"net"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	zepb "github.com/ze-software/ze/api/proto"
	"github.com/ze-software/ze/internal/component/api"
)

// startAuthReloadServer starts a real gRPC server on a bound loopback listener
// and returns it alongside a client, so a test can flip the server's
// authentication and prove the change reaches a request in flight.
func startAuthReloadServer(t *testing.T, cfg GRPCConfig) (*GRPCServer, zepb.ZeServiceClient) {
	t.Helper()

	if len(cfg.ListenAddrs) == 0 {
		cfg.ListenAddrs = []string{"127.0.0.1:0"}
	}
	srv, err := NewGRPCServer(cfg, testEngine(), nil)
	require.NoError(t, err)

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	serveBackground(srv.srv, ln)
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	return srv, zepb.NewZeServiceClient(conn)
}

// listCommandsCode issues a real RPC and reports its gRPC status code.
func listCommandsCode(t *testing.T, client zepb.ZeServiceClient, token string) codes.Code {
	t.Helper()
	ctx := t.Context()
	if token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}
	_, err := client.ListCommands(ctx, &zepb.ListCommandsRequest{})
	return status.Code(err)
}

func TestGRPCAuthenticationProviderPublishesModesAtomically(t *testing.T) {
	var live atomic.Pointer[api.Authentication]
	publish := func(authentication api.Authentication) { live.Store(&authentication) }
	provider := func() api.Authentication { return *live.Load() }
	publish(testAuthentication("old", nil, nil)())
	srv, client := startAuthReloadServer(t, GRPCConfig{Authentication: provider})

	assert.True(t, srv.Authenticated())
	assert.Equal(t, codes.OK, listCommandsCode(t, client, "old"))
	assert.Equal(t, codes.Unauthenticated, listCommandsCode(t, client, "new"))

	publish(api.Authentication{Required: true})
	assert.True(t, srv.Authenticated(), "staging is gated for exposure checks")
	assert.Equal(t, codes.Unauthenticated, listCommandsCode(t, client, "old"))
	assert.Equal(t, codes.Unauthenticated, listCommandsCode(t, client, "new"))

	publish(testAuthentication("new", nil, nil)())
	assert.Equal(t, codes.Unauthenticated, listCommandsCode(t, client, "old"))
	assert.Equal(t, codes.OK, listCommandsCode(t, client, "new"))

	publish(api.Authentication{})
	assert.False(t, srv.Authenticated())
	assert.Equal(t, codes.OK, listCommandsCode(t, client, ""))
}

func TestGRPCRejectedCandidateNeverAuthenticates(t *testing.T) {
	var live atomic.Pointer[api.Authentication]
	accepted := testAuthentication("accepted", nil, nil)()
	live.Store(&accepted)
	_, client := startAuthReloadServer(t, GRPCConfig{Authentication: func() api.Authentication {
		return *live.Load()
	}})

	staging := api.Authentication{Required: true}
	live.Store(&staging)
	assert.Equal(t, codes.Unauthenticated, listCommandsCode(t, client, "candidate"))

	live.Store(&accepted)
	assert.Equal(t, codes.Unauthenticated, listCommandsCode(t, client, "candidate"))
	assert.Equal(t, codes.OK, listCommandsCode(t, client, "accepted"))
}

func TestGRPCReconfigureAppliesPublishedExposureModeAndTLS(t *testing.T) {
	var live atomic.Pointer[api.Authentication]
	authenticated := testAuthentication("secret", nil, nil)()
	live.Store(&authenticated)
	srv, err := NewGRPCServer(GRPCConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		Authentication: func() api.Authentication {
			return *live.Load()
		},
	}, testEngine(), nil)
	require.NoError(t, err)

	err = srv.Reconfigure(t.Context(), []string{"0.0.0.0:50051"})
	require.ErrorContains(t, err, "TLS")

	unauthenticated := api.Authentication{}
	live.Store(&unauthenticated)
	err = srv.Reconfigure(t.Context(), []string{"0.0.0.0:50051"})
	require.ErrorContains(t, err, "requires authentication")
	require.NoError(t, srv.Reconfigure(t.Context(), []string{"127.0.0.1:0"}))
}
